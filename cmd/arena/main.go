// Command arena plays batches of games between registered engines and maintains
// the Elo ranking in data/rankings.json.
package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"sort"
	"time"

	"wizardboardgame/internal/ai"
	_ "wizardboardgame/internal/ai/catalog"
	"wizardboardgame/internal/arena"
)

func main() {
	list := flag.Bool("list", false, "list registered engines and their ratings")
	showRanks := flag.Bool("rankings", false, "print the ranking table")
	tournament := flag.Bool("tournament", false, "play every pair of engines")
	idA := flag.String("a", "alphabeta-r2-v1", "first engine ID")
	idB := flag.String("b", "ml-r2-v1", "second engine ID")
	games := flag.Int("games", 10, "games per pair")
	nodes := flag.Int("nodes", 0, "node budget per move (debug only; 0 = no node cap)")
	timeBudget := flag.Duration("time", 1500*time.Millisecond, "time budget per move (live strong default)")
	depth := flag.Int("depth", 6, "max search depth (same as live play and assist)")
	filter := flag.Bool("filter", false, "enable the heuristic candidate filter (default off; affects the r2 engines)")
	workers := flag.Int("workers", runtime.NumCPU(), "parallel games")
	seed := flag.Int64("seed", 1, "base seed")
	path := flag.String("path", arena.DefaultPath, "ranking file")
	flag.Parse()

	ranks, err := arena.Load(*path)
	if err != nil {
		fatal(err)
	}

	if *list || *showRanks {
		printRanks(ranks)
		return
	}

	cfg := arena.GameConfig{
		MaxTime:       *timeBudget,
		MaxNodes:      *nodes,
		MaxDepth:      *depth,
		DisableFilter: !*filter,
		Workers:       *workers,
		Seed:          *seed,
	}

	if *tournament {
		ids := make([]string, 0)
		for _, info := range ai.List() {
			ids = append(ids, info.ID)
		}
		sort.Strings(ids)
		for i := 0; i < len(ids); i++ {
			for j := i + 1; j < len(ids); j++ {
				if err := runAndRecord(ranks, ids[i], ids[j], *games, cfg, *path); err != nil {
					fatal(err)
				}
			}
		}
		printRanks(ranks)
		return
	}

	if err := runAndRecord(ranks, *idA, *idB, *games, cfg, *path); err != nil {
		fatal(err)
	}
	printRanks(ranks)
}

func runAndRecord(ranks *arena.Rankings, aID, bID string, games int, cfg arena.GameConfig, path string) error {
	if _, _, ok := ai.Get(aID); !ok {
		return fmt.Errorf("unknown engine %q (use -list)", aID)
	}
	if _, _, ok := ai.Get(bID); !ok {
		return fmt.Errorf("unknown engine %q (use -list)", bID)
	}
	fmt.Printf("%s vs %s: %d games, depth %d, %s/move, %d workers...\n", aID, bID, games, cfg.MaxDepth, cfg.MaxTime, cfg.Workers)
	res, err := arena.RunPair(aID, bID, games, cfg)
	if err != nil {
		return err
	}
	eloA, eloB := ranks.ApplyPair(aID, bID, res, arena.RankConfig{
		MaxTimeMs: cfg.MaxTime.Milliseconds(),
		MaxNodes:  cfg.MaxNodes,
		MaxDepth:  cfg.MaxDepth,
		NoFilter:  cfg.DisableFilter,
	})
	fmt.Printf("  A wins %d, B wins %d, draws %d, errors %d  |  score(A) %.3f\n",
		res.AWin, res.BWin, res.Draw, res.Error, res.ScoreA)
	fmt.Printf("  %s %.0f, %s %.0f\n", aID, eloA, bID, eloB)
	return ranks.Save(path)
}

func printRanks(ranks *arena.Rankings) {
	ids := ranks.Sorted()
	fmt.Printf("%-4s %-18s %6s %6s %5s %4s %4s\n", "#", "engine", "elo", "games", "wins", "draw", "loss")
	for i, id := range ids {
		e := ranks.Engines[id]
		fmt.Printf("%-4d %-18s %6.0f %6d %5d %4d %4d\n", i+1, id, e.Elo, e.Games, e.Wins, e.Draws, e.Losses)
	}
	if !ranks.Updated.IsZero() {
		fmt.Printf("updated: %s\n", ranks.Updated.Format("2006-01-02 15:04"))
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "arena:", err)
	os.Exit(1)
}
