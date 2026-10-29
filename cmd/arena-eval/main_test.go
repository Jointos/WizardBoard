package main

import (
	"path/filepath"
	"testing"
	"time"

	"wizardboardgame/internal/ai"
	"wizardboardgame/internal/arena"
	"wizardboardgame/internal/game"
)

func TestStatsMergeRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "stats.json")

	res, err := arena.RunPair("alphabeta-r2-v1", "ml-r2-v1", 1, arena.GameConfig{MaxNodes: 100, Workers: 1, Seed: 3})
	if err != nil {
		t.Fatal(err)
	}
	ranks := arena.NewRankings()
	ranks.ApplyPair("alphabeta-r2-v1", "ml-r2-v1", res, arena.RankConfig{MaxDepth: 1})

	cfg := arena.StatsConfig{GamesPerPair: 1, MaxDepth: 1, Mode: "all"}
	sf := arena.NewStatsFile()
	arena.MergeRun(sf, ranks, "alphabeta-r2-v1", "ml-r2-v1", res, cfg, time.Second, false)

	before := game.New(nil)
	var move game.Action
	for _, a := range before.LegalActions() {
		if a.Kind == game.ActMove {
			move = a
			break
		}
	}
	after, err := before.Apply(move)
	if err != nil {
		t.Fatal(err)
	}
	sf.BestMoves = []arena.BestMove{{
		Rank: 1, Game: 1, Pair: "alphabeta-r2-v1|ml-r2-v1", Ply: 1,
		Mover: game.PlayerA, Engine: "alphabeta-r2-v1", Winner: game.PlayerA,
		Action: move, Label: before.Describe(move), Before: before, After: after,
		EvalBefore: -5, EvalAfter: 11, Swing: 16, Uniqueness: 2,
	}}
	sf.BestMoveConfig = &arena.BestMoveConfig{Engine: "ml-r2-v1", Depth: 4, Games: 1, Candidates: 10, Top: 1}

	if err := arena.SaveStats(path, sf); err != nil {
		t.Fatal(err)
	}

	got, err := arena.LoadStats(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Runs != 1 {
		t.Fatalf("runs = %d, want 1", got.Runs)
	}
	if got.Overall.Plies != res.Plies {
		t.Fatalf("overall plies = %d, want %d", got.Overall.Plies, res.Plies)
	}
	if got.Sides.Games != 1 || got.Sides.AWin+got.Sides.BWin+got.Sides.Draw != 1 {
		t.Fatalf("overall sides: %+v", got.Sides)
	}
	if got.Endgame.Games != 1 {
		t.Fatalf("endgame games = %d, want 1", got.Endgame.Games)
	}
	if len(got.BestMoves) != 1 || got.BestMoves[0].Label == "" {
		t.Fatalf("best moves round-trip: %+v", got.BestMoves)
	}
	if got.BestMoveConfig == nil || got.BestMoveConfig.Top != 1 {
		t.Fatalf("best move config round-trip: %+v", got.BestMoveConfig)
	}
	if len(got.RecentRuns) != 1 || got.RecentRuns[0].TookMs != 1000 {
		t.Fatalf("recent runs: %+v", got.RecentRuns)
	}
	key := arena.PairKey("alphabeta-r2-v1", "ml-r2-v1")
	ps := got.AIVsAI.ByPair[key]
	if ps == nil || ps.Games != 1 {
		t.Fatalf("pair stats: %+v", ps)
	}
	if ps.Sides.Games != 1 || ps.Sides.AWin+ps.Sides.BWin+ps.Sides.Draw != 1 {
		t.Fatalf("pair sides: %+v", ps.Sides)
	}
	if len(ps.ByEngine) != 2 {
		t.Fatalf("pair engine stats: %d", len(ps.ByEngine))
	}
	if got.AIVsAI.ByEngine["alphabeta-r2-v1"] == nil || got.AIVsAI.ByEngine["ml-r2-v1"] == nil {
		t.Fatal("missing per-engine stats")
	}
	if len(got.Ranking.Engines) != len(ai.List()) {
		t.Fatalf("ranking snapshot engines = %d, want %d", len(got.Ranking.Engines), len(ai.List()))
	}

	// A second merge must accumulate, not replace.
	arena.MergeRun(got, ranks, "alphabeta-r2-v1", "ml-r2-v1", res, cfg, time.Second, false)
	if got.Runs != 2 {
		t.Fatalf("runs after merge = %d, want 2", got.Runs)
	}
	if got.Overall.Plies != 2*res.Plies {
		t.Fatalf("overall plies after merge = %d, want %d", got.Overall.Plies, 2*res.Plies)
	}
	if got.Sides.Games != 2 {
		t.Fatalf("overall side games after merge = %d, want 2", got.Sides.Games)
	}
	if got.AIVsAI.ByPair[key].Games != 2 {
		t.Fatalf("pair games after merge = %d, want 2", got.AIVsAI.ByPair[key].Games)
	}
	if len(got.RecentRuns) != 2 {
		t.Fatalf("recent runs after merge = %d, want 2", len(got.RecentRuns))
	}
}

func TestStatsOnlyDoesNotTouchGlobalEngineTable(t *testing.T) {
	res, err := arena.RunPair("alphabeta-r2-v1", "ml-r2-v1", 2, arena.GameConfig{MaxNodes: 100, Workers: 2, Seed: 7})
	if err != nil {
		t.Fatal(err)
	}
	ranks := arena.NewRankings()
	sf := arena.NewStatsFile()
	cfg := arena.StatsConfig{GamesPerPair: 2, MaxDepth: 1, StatsOnly: true, Mode: "pairs"}

	arena.MergeRun(sf, ranks, "alphabeta-r2-v1", "ml-r2-v1", res, cfg, time.Second, true)
	if len(sf.AIVsAI.ByEngine) != 0 {
		t.Fatalf("stats-only run updated the global engine table: %+v", sf.AIVsAI.ByEngine)
	}
	if sf.StatsOnlyGames != 2 {
		t.Fatalf("statsOnlyGames = %d, want 2", sf.StatsOnlyGames)
	}
	if sf.Sides.Games != 2 {
		t.Fatalf("side games = %d, want 2", sf.Sides.Games)
	}
	if sf.AIVsAI.ByPair[arena.PairKey("alphabeta-r2-v1", "ml-r2-v1")].Games != 2 {
		t.Fatal("pair stats were not recorded in stats-only mode")
	}
}
