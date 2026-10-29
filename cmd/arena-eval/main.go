// Command arena-eval re-evaluates engine ratings and writes detailed event
// statistics (buffs, nerfs, cleanses, kills, side balance).
//
// By default it evaluates every engine that has no games yet against every
// existing engine. Use -all for a full round-robin and -id for one engine.
//
// The arena plays at a fixed depth (6 by default), the same depth live play and
// the AI assist use. The -nodes flag exists for debugging only; do not use a
// node budget for rating games.
//
// Use -no-elo for a statistics-only balance run: the games are merged into the
// stats file but data/rankings.json is not touched.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"wizardboardgame/internal/ai"
	_ "wizardboardgame/internal/ai/catalog"
	"wizardboardgame/internal/arena"
)

const defaultStatsPath = "data/arena_stats.json"

func main() {
	idFlag := flag.String("id", "", "evaluate this engine against every other engine")
	all := flag.Bool("all", false, "evaluate every pair (full round-robin)")
	pairsSpec := flag.String("pairs", "", "explicit pairs, e.g. 'a|b,c|d' (useful to finish a partial run)")
	reset := flag.Bool("reset", false, "reset every rating to 1200 and start with empty stats")
	filter := flag.Bool("filter", false, "enable the heuristic candidate filter (default off; affects the r2 engines)")
	games := flag.Int("games", 10, "games per pair")
	depth := flag.Int("depth", 6, "max search depth (same as live play and assist)")
	timeBudget := flag.Duration("time", 1500*time.Millisecond, "time budget per move (live strong default)")
	nodes := flag.Int("nodes", 0, "node budget per move (debug only; 0 = no node cap)")
	workers := flag.Int("workers", runtime.NumCPU(), "parallel games")
	seed := flag.Int64("seed", 1, "base seed")
	paired := flag.Bool("paired", false, "pair games with the same seed and swapped sides (lower side-comparison noise)")
	noElo := flag.Bool("no-elo", false, "record stats only; do not update data/rankings.json")
	stamp := flag.Bool("stamp", false, "append a _YYYY-MM-DD_HH-MM-SS timestamp to -out")
	bestMove := flag.Bool("best-move", true, "log games and store the top game-changing moves in the stats file")
	bestMoveDepth := flag.Int("best-move-depth", 4, "reference search depth for the best-move analysis")
	bestMoveTop := flag.Int("best-move-top", 10, "number of game-changing moves to keep")
	bestMoveEngine := flag.String("best-move-engine", "ml-r2-v2", "reference engine for the best-move analysis")
	ranksPath := flag.String("path", arena.DefaultPath, "ranking file")
	outPath := flag.String("out", defaultStatsPath, "stats JSON output")
	flag.Parse()

	if *paired && *games%2 != 0 {
		fatal(fmt.Errorf("-paired needs an even -games value (got %d); otherwise one engine plays Player A one more time", *games))
	}

	out := stampPath(*outPath, *stamp)

	ranks, err := arena.Load(*ranksPath)
	if err != nil {
		fatal(err)
	}
	if *reset {
		ranks = arena.NewRankings()
	}

	mode := "new"
	var pairs [][2]string
	switch {
	case *pairsSpec != "":
		mode = "pairs"
		for _, spec := range strings.Split(*pairsSpec, ",") {
			spec = strings.TrimSpace(spec)
			if spec == "" {
				continue
			}
			parts := strings.Split(spec, "|")
			if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
				fatal(fmt.Errorf("invalid -pairs entry %q (want a|b)", spec))
			}
			if parts[0] == parts[1] && !*noElo {
				fatal(fmt.Errorf("self-play pair %q needs -no-elo (it is a balance run, not a rating game)", spec))
			}
			pairs = append(pairs, [2]string{parts[0], parts[1]})
		}
	case *all:
		mode = "all"
		ids := registeredIDs()
		for i := 0; i < len(ids); i++ {
			for j := i + 1; j < len(ids); j++ {
				pairs = append(pairs, [2]string{ids[i], ids[j]})
			}
		}
	case *idFlag != "":
		mode = "id"
		if _, _, ok := ai.Get(*idFlag); !ok {
			fatal(fmt.Errorf("unknown engine %q", *idFlag))
		}
		for _, other := range registeredIDs() {
			if other != *idFlag {
				pairs = append(pairs, [2]string{*idFlag, other})
			}
		}
	default:
		var fresh []string
		for _, info := range ai.List() {
			if e := ranks.Engines[info.ID]; e != nil && e.Games == 0 {
				fresh = append(fresh, info.ID)
			}
		}
		if len(fresh) == 0 {
			fmt.Println("no new engines (all registered engines already have games); use -all or -id")
			return
		}
		seen := make(map[string]bool)
		for _, nid := range fresh {
			for _, info := range ai.List() {
				if info.ID == nid {
					continue
				}
				key := arena.PairKey(nid, info.ID)
				if seen[key] {
					continue
				}
				seen[key] = true
				pairs = append(pairs, [2]string{nid, info.ID})
			}
		}
	}

	if len(pairs) == 0 {
		fmt.Println("no pairs to evaluate")
		return
	}

	var sf *arena.StatsFile
	if *reset {
		sf = arena.NewStatsFile()
	} else {
		sf, err = arena.LoadStats(out)
		if err != nil {
			fatal(err)
		}
	}

	cfg := arena.GameConfig{
		MaxTime:       *timeBudget,
		MaxNodes:      *nodes,
		MaxDepth:      *depth,
		DisableFilter: !*filter,
		Workers:       *workers,
		Seed:          *seed,
		Paired:        *paired,
		LogGames:      *bestMove,
	}
	runCfg := arena.StatsConfig{
		GamesPerPair: *games,
		MaxTimeMs:    timeBudget.Milliseconds(),
		MaxDepth:     *depth,
		MaxNodes:     *nodes,
		NoFilter:     !*filter,
		Paired:       *paired,
		StatsOnly:    *noElo,
		BestMove:     *bestMove,
		Seed:         *seed,
		Mode:         mode,
	}
	var allLogs []arena.GameLog

	for _, p := range pairs {
		aID, bID := p[0], p[1]
		fmt.Printf("%s vs %s: %d games, depth %d, %s/move...\n", aID, bID, *games, *depth, *timeBudget)
		started := time.Now()
		res, err := arena.RunPair(aID, bID, *games, cfg)
		took := time.Since(started).Round(time.Second)
		if err != nil {
			fatal(err)
		}
		if !*noElo {
			ranks.ApplyPair(aID, bID, res, arena.RankConfig{
				MaxTimeMs: timeBudget.Milliseconds(),
				MaxNodes:  *nodes,
				MaxDepth:  *depth,
				NoFilter:  !*filter,
			})
			if err := ranks.Save(*ranksPath); err != nil {
				fatal(err)
			}
		}
		arena.MergeRun(sf, ranks, aID, bID, res, runCfg, took, *noElo)
		if err := arena.SaveStats(out, sf); err != nil {
			fatal(err)
		}
		allLogs = append(allLogs, res.Logs...)
		fmt.Printf("  engines: %s %d, %s %d, draws %d, errors %d  |  score(%s) %.3f  |  took %s\n",
			aID, res.AWin, bID, res.BWin, res.Draw, res.Error, aID, res.ScoreA, took)
		printSides("  sides  ", res.Sides)
		printEndgame("  endgame", res.Endgame)
	}

	if *bestMove && len(allLogs) > 0 {
		fmt.Printf("best moves: analysing %d games (reference %s, depth %d)...\n", len(allLogs), *bestMoveEngine, *bestMoveDepth)
		moves, bmc, err := arena.FindBestMoves(allLogs, arena.BestMoveOptions{
			Engine: *bestMoveEngine,
			Depth:  *bestMoveDepth,
			Top:    *bestMoveTop,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "best moves: %v\n", err)
		} else {
			sf.BestMoves = moves
			sf.BestMoveConfig = &bmc
			if len(moves) > 0 {
				m := moves[0]
				fmt.Printf("  #1 %s  |  swing %d  |  game %d, ply %d  |  %s\n", m.Label, m.Swing, m.Game, m.Ply, m.Engine)
			} else {
				fmt.Println("  no decisive games to analyse")
			}
			if err := arena.SaveStats(out, sf); err != nil {
				fatal(err)
			}
		}
	}

	printRanks(ranks)
	printSides("overall sides", sf.Sides)
	fmt.Printf("stats: %s\n", out)
	fmt.Printf("html:  go run ./cmd/arena-report -in %s -out <path>/report.html\n", out)
}

// stampPath appends a local timestamp before the extension.
func stampPath(path string, stamp bool) string {
	if !stamp || path == "" {
		return path
	}
	ext := filepath.Ext(path)
	base := strings.TrimSuffix(path, ext)
	return base + "_" + time.Now().Format("2006-01-02_15-04-05") + ext
}

func printSides(prefix string, s arena.SideStats) {
	if s.Games == 0 {
		fmt.Printf("%s: no finished games\n", prefix)
		return
	}
	al, ah := arena.Wilson(s.AWin, s.Games)
	bl, bh := arena.Wilson(s.BWin, s.Games)
	fmt.Printf("%s: Player A %d (%.1f%% [%.1f-%.1f])  Player B %d (%.1f%% [%.1f-%.1f])  draws %d  |  n=%d\n",
		prefix,
		s.AWin, 100*float64(s.AWin)/float64(s.Games), 100*al, 100*ah,
		s.BWin, 100*float64(s.BWin)/float64(s.Games), 100*bl, 100*bh,
		s.Draw, s.Games)
}

func printEndgame(prefix string, e arena.EndStats) {
	if e.Games == 0 {
		return
	}
	first := 0
	if e.FirstKillDecided > 0 {
		first = 100 * e.FirstKillWins / e.FirstKillDecided
	}
	firstWiz := 0
	if e.FirstWizardDecided > 0 {
		firstWiz = 100 * e.FirstWizardWins / e.FirstWizardDecided
	}
	fmt.Printf("%s: first blood A %d / B %d / none %d (%d%% won after first kill)  |  first wizard A %d / B %d (%d%% won)  |  stalemates %d  |  avg plies %.1f\n",
		prefix, e.FirstKillA, e.FirstKillB, e.NoFirstKill, first,
		e.FirstWizardKillA, e.FirstWizardKillB, firstWiz, e.Stalemates,
		float64(e.Plies)/float64(e.Games))
}

func registeredIDs() []string {
	ids := make([]string, 0)
	for _, info := range ai.List() {
		ids = append(ids, info.ID)
	}
	sort.Strings(ids)
	return ids
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
	fmt.Fprintln(os.Stderr, "arena-eval:", err)
	os.Exit(1)
}
