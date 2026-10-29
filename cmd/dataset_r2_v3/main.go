// Command dataset_r2_v3 generates supervised training data for the r2 ML v2
// evaluation. It plays alphabeta-r2-v1 self-play games (no random, greedy or
// old-rules engine), records the 50-feature vector for every position, and
// labels each position with the teacher's search score (dense labels) plus the
// eventual game result.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"wizardboardgame/internal/ai"
	_ "wizardboardgame/internal/ai/catalog"
	"wizardboardgame/internal/ai/ml_r2_v3"
	"wizardboardgame/internal/game"
)

const teacherID = "alphabeta-r2-v1"

type row struct {
	Game     int       `json:"game"`
	Ply      int       `json:"ply"`
	X        []float32 `json:"x"`
	Hand     int       `json:"hand"`
	Score    int       `json:"score,omitempty"`
	HasScore bool      `json:"hasScore,omitempty"`
	Outcome  int       `json:"outcome"`
}

type position struct {
	x        []float32
	hand     int
	ply      int
	mover    game.Player
	score    int
	hasScore bool
}

func main() {
	games := flag.Int("games", 500, "number of self-play games")
	perMove := flag.Duration("time", 500*time.Millisecond, "time budget per move")
	depth := flag.Int("depth", 6, "max search depth")
	workers := flag.Int("workers", runtime.NumCPU(), "parallel games")
	seed := flag.Int64("seed", 21, "base seed")
	out := flag.String("out", "data/datasets/r2_ml_v3.jsonl", "output JSONL")
	maxPlies := flag.Int("maxplies", 5000, "safety cap per game")
	flag.Parse()

	if err := os.MkdirAll(filepath.Dir(*out), 0o755); err != nil {
		fatal(err)
	}
	f, err := os.Create(*out)
	if err != nil {
		fatal(err)
	}
	defer f.Close()
	w := bufio.NewWriterSize(f, 1<<20)
	var writeMu sync.Mutex

	var done atomic.Int64
	var rows atomic.Int64
	start := time.Now()
	var wg sync.WaitGroup
	jobs := make(chan int)
	for k := 0; k < *workers; k++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				rs := playGame(i, *perMove, *depth, *seed, *maxPlies)
				if len(rs) == 0 {
					continue
				}
				writeMu.Lock()
				for _, r := range rs {
					b, _ := json.Marshal(r)
					w.Write(b)
					w.WriteByte('\n')
				}
				writeMu.Unlock()
				rows.Add(int64(len(rs)))
				n := done.Add(1)
				if n%25 == 0 || int(n) == *games {
					writeMu.Lock()
					w.Flush()
					writeMu.Unlock()
					fmt.Printf("%d/%d games, %d positions, %s\n", n, *games, rows.Load(), time.Since(start).Round(time.Second))
				}
			}
		}()
	}
	for i := 0; i < *games; i++ {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	if err := w.Flush(); err != nil {
		fatal(err)
	}
	fmt.Printf("done: %d games, %d positions, %s -> %s\n", done.Load(), rows.Load(), time.Since(start).Round(time.Second), *out)
}

func playGame(i int, perMove time.Duration, depth int, seed int64, maxPlies int) []row {
	aIsA := i%2 == 0
	gameSeed := seed + int64(i)*7919
	cfgA := ai.Config{MaxTime: perMove, MaxDepth: depth, Seed: gameSeed}
	cfgB := ai.Config{MaxTime: perMove, MaxDepth: depth, Seed: gameSeed + 104729}
	ea, ok := ai.New(teacherID, cfgA)
	if !ok {
		return nil
	}
	eb, ok := ai.New(teacherID, cfgB)
	if !ok {
		return nil
	}
	a, b := ea, eb
	if !aIsA {
		a, b = eb, ea
	}

	s := game.New(nil)
	var positions []position
	for ply := 0; !s.Over && ply < maxPlies; ply++ {
		legal := s.LegalActions()
		if len(legal) == 0 {
			break
		}
		eng := a
		if s.Turn == game.PlayerB {
			eng = b
		}
		var action game.Action
		var score int
		var hasScore bool
		if se, ok := eng.(ai.ScoredEngine); ok && len(legal) > 1 {
			action, score = se.SelectScore(s, legal)
			hasScore = true
		} else {
			action = eng.Select(s, legal)
		}
		parts := ml_r2_v3.HandPartsOf(&s)
		feat := ml_r2_v3.FeaturesWithParts(&s, parts)
		positions = append(positions, position{
			x:        feat[:],
			hand:     parts.Total(s.Turn),
			ply:      ply,
			mover:    s.Turn,
			score:    score,
			hasScore: hasScore,
		})
		next, err := s.Apply(action)
		if err != nil {
			break
		}
		s = next
	}

	out := make([]row, 0, len(positions))
	for _, pos := range positions {
		outcome := 0
		if s.Over && s.Winner != game.NoPlayer {
			if s.Winner == pos.mover {
				outcome = 1
			} else {
				outcome = -1
			}
		}
		out = append(out, row{
			Game: i, Ply: pos.ply, X: pos.x, Hand: pos.hand,
			Score: pos.score, HasScore: pos.hasScore, Outcome: outcome,
		})
	}
	return out
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "dataset_r2_v3:", err)
	os.Exit(1)
}
