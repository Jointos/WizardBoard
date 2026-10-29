// Command selfplay runs automated games between registered engines. It is the
// main harness for validating and tuning the AI: every engine lives in its own
// versioned package under internal/ai.
package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"wizardboardgame/internal/ai"
	_ "wizardboardgame/internal/ai/catalog"
	"wizardboardgame/internal/game"
)

func main() {
	games := flag.Int("games", 100, "number of games to play")
	idA := flag.String("a", "alphabeta-r2-v1", "engine ID for player A")
	idB := flag.String("b", "ml-r2-v1", "engine ID for player B")
	maxPlies := flag.Int("maxplies", 5000, "safety cap per game")
	nodes := flag.Int("nodes", 0, "node budget per move (deterministic; 0 = use time)")
	depth := flag.Int("depth", 0, "max search depth (0 = engine default)")
	perMove := flag.Duration("time", 0, "time budget per move (0 = engine default)")
	seed := flag.Int64("seed", 1, "base seed; game i uses seed+i")
	flag.Parse()

	if _, _, ok := ai.Get(*idA); !ok {
		fmt.Fprintf(os.Stderr, "unknown engine %q\n\nregistered engines:\n", *idA)
		listEngines()
		os.Exit(2)
	}
	if _, _, ok := ai.Get(*idB); !ok {
		fmt.Fprintf(os.Stderr, "unknown engine %q\n\nregistered engines:\n", *idB)
		listEngines()
		os.Exit(2)
	}

	aWins, bWins, draws, errors := 0, 0, 0, 0
	totalPlies := 0
	start := time.Now()

	for i := 0; i < *games; i++ {
		cfg := ai.Config{
			MaxNodes: *nodes,
			MaxDepth: *depth,
			MaxTime:  *perMove,
			Seed:     *seed + int64(i)*2,
		}
		pa, _ := ai.New(*idA, cfg)
		cfg.Seed++
		pb, _ := ai.New(*idB, cfg)

		s := game.New(nil)
		plies := 0
		for !s.Over && plies < *maxPlies {
			var eng ai.Engine = pa
			if s.Turn == game.PlayerB {
				eng = pb
			}
			legal := s.LegalActions()
			if len(legal) == 0 {
				break
			}
			next, err := s.Apply(eng.Select(s, legal))
			if err != nil {
				fmt.Fprintf(os.Stderr, "game %d ply %d: %v\n", i, plies, err)
				errors++
				break
			}
			s = next
			plies++
		}
		totalPlies += plies
		switch {
		case !s.Over, s.Winner == game.NoPlayer:
			draws++
		case s.Winner == game.PlayerA:
			aWins++
		default:
			bWins++
		}
	}

	elapsed := time.Since(start)
	fmt.Printf("A=%s B=%s  games: %d  A wins: %d  B wins: %d  draws: %d  errors: %d\n",
		*idA, *idB, *games, aWins, bWins, draws, errors)
	if *games > 0 {
		fmt.Printf("avg plies: %.1f  elapsed: %s  games/s: %.1f\n",
			float64(totalPlies)/float64(*games), elapsed.Round(time.Millisecond),
			float64(*games)/elapsed.Seconds())
	}
}

func listEngines() {
	for _, info := range ai.List() {
		fmt.Fprintf(os.Stderr, "  %-16s %s\n", info.ID, info.Name)
	}
}
