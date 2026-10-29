package arena

import (
	"math"
	"testing"

	"wizardboardgame/internal/game"
)

func TestSideStatsRecordAndAdd(t *testing.T) {
	var s SideStats
	s.Record(game.PlayerA)
	s.Record(game.PlayerA)
	s.Record(game.PlayerB)
	s.Record(game.NoPlayer)
	if s.Games != 4 || s.AWin != 2 || s.BWin != 1 || s.Draw != 1 {
		t.Fatalf("record: %+v", s)
	}

	var o SideStats
	o.Record(game.PlayerA)
	s.Add(o)
	if s.Games != 5 || s.AWin != 3 {
		t.Fatalf("add: %+v", s)
	}

	if math.Abs(s.WinRateA()-0.6) > 1e-9 {
		t.Fatalf("win rate A = %f, want 0.6", s.WinRateA())
	}
}

func TestWilson(t *testing.T) {
	lo, hi := Wilson(52, 100)
	if math.Abs(lo-0.424) > 0.006 || math.Abs(hi-0.614) > 0.008 {
		t.Fatalf("Wilson(52,100) = [%f, %f]", lo, hi)
	}
	lo, hi = Wilson(0, 10)
	if lo != 0 || math.Abs(hi-0.278) > 0.01 {
		t.Fatalf("Wilson(0,10) = [%f, %f]", lo, hi)
	}
	lo, hi = Wilson(0, 0)
	if lo != 0 || hi != 1 {
		t.Fatalf("Wilson(0,0) = [%f, %f]", lo, hi)
	}
}

func TestRunPairSideAndEndStats(t *testing.T) {
	res, err := RunPair("alphabeta-r2-v1", "ml-r2-v1", 4, GameConfig{MaxNodes: 200, Workers: 2, Seed: 11, Paired: true})
	if err != nil {
		t.Fatal(err)
	}
	if res.Sides.Games != 4 {
		t.Fatalf("side games = %d, want 4", res.Sides.Games)
	}
	if res.Sides.AWin+res.Sides.BWin+res.Sides.Draw != 4 {
		t.Fatalf("side outcomes do not add up: %+v", res.Sides)
	}
	if res.Endgame.Games != 4 {
		t.Fatalf("endgame games = %d, want 4", res.Endgame.Games)
	}
	if res.Endgame.Plies != res.Plies {
		t.Fatalf("endgame plies = %d, result plies = %d", res.Endgame.Plies, res.Plies)
	}
	buckets := 0
	for _, n := range res.Endgame.PlyBuckets {
		buckets += n
	}
	if buckets != 4 {
		t.Fatalf("ply buckets sum = %d, want 4", buckets)
	}
	if res.Stats.BySide["A"] == nil || res.Stats.BySide["B"] == nil {
		t.Fatal("missing per-side event stats")
	}
	if res.Stats.BySide["A"].Games != 4 || res.Stats.BySide["B"].Games != 4 {
		t.Fatalf("per-side games: A=%d B=%d", res.Stats.BySide["A"].Games, res.Stats.BySide["B"].Games)
	}
	if got, want := sumMap(res.Stats.BySide["A"].Actions)+sumMap(res.Stats.BySide["B"].Actions), sumMap(res.Stats.Overall.Actions); got != want {
		t.Fatalf("per-side actions = %d, overall = %d", got, want)
	}
}

func TestRunPairSelfPlay(t *testing.T) {
	res, err := RunPair("alphabeta-r2-v1", "alphabeta-r2-v1", 4, GameConfig{MaxNodes: 200, Workers: 2, Seed: 13})
	if err != nil {
		t.Fatal(err)
	}
	if res.Sides.Games != 4 {
		t.Fatalf("side games = %d, want 4", res.Sides.Games)
	}
	if len(res.Stats.ByEngine) != 1 {
		t.Fatalf("self-play byEngine entries = %d, want 1", len(res.Stats.ByEngine))
	}
	es := res.Stats.ByEngine["alphabeta-r2-v1"]
	if es == nil || es.Games != 4 {
		t.Fatalf("self-play engine stats: %+v", es)
	}
}
