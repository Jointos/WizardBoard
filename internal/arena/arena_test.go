package arena

import (
	"math"
	"path/filepath"
	"testing"
)

func TestUpdateKeepsPoolAverage(t *testing.T) {
	ra, rb := 1200.0, 1200.0
	a, b := Update(ra, rb, 1.0, 100, KFactor)
	if a <= ra {
		t.Fatalf("winner rating did not grow: %.1f -> %.1f", ra, a)
	}
	if b >= rb {
		t.Fatalf("loser rating did not drop: %.1f -> %.1f", rb, b)
	}
	if math.Abs((a+b)-(ra+rb)) > 1e-9 {
		t.Fatalf("pool average changed: %.3f vs %.3f", a+b, ra+rb)
	}
}

func TestUpdateEvenScoreDoesNothing(t *testing.T) {
	a, b := Update(1300, 1300, 0.5, 100, KFactor)
	if a != 1300 || b != 1300 {
		t.Fatalf("ratings changed on an even score: %.1f / %.1f", a, b)
	}
}

func TestExpected(t *testing.T) {
	if got := Expected(1200, 1200); math.Abs(got-0.5) > 1e-9 {
		t.Fatalf("expected 0.5, got %f", got)
	}
	if Expected(1400, 1200) <= 0.5 {
		t.Fatal("higher rated must be favoured")
	}
}

func TestStoreRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rankings.json")

	r, err := Load(path)
	if err != nil {
		t.Fatalf("load (missing): %v", err)
	}
	if len(r.Engines) == 0 {
		t.Fatal("expected the registered engines in a fresh table")
	}
	r.ApplyPair("alphabeta-r2-v1", "ml-r2-v1", PairResult{
		Games: 10, AWin: 9, BWin: 1, ScoreA: 0.9,
	}, RankConfig{MaxNodes: 5000})
	if err := r.Save(path); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	er := got.Engines["alphabeta-r2-v1"]
	if er == nil || er.Games != 10 || er.Wins != 9 || er.Losses != 1 {
		t.Fatalf("unexpected stats: %+v", er)
	}
	if len(got.History) != 1 {
		t.Fatalf("history length = %d, want 1", len(got.History))
	}
	if ps := got.Pairings[PairKey("alphabeta-r2-v1", "ml-r2-v1")]; ps == nil || ps.Games != 10 {
		t.Fatalf("pairing not recorded: %+v", ps)
	}
}

func TestRunPairAlternatesSides(t *testing.T) {
	res, err := RunPair("alphabeta-r2-v1", "alphabeta-r2-v1", 6, GameConfig{MaxNodes: 200, Workers: 3, Seed: 11})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if res.Games != 6 || res.Error != 0 {
		t.Fatalf("games=%d errors=%d", res.Games, res.Error)
	}
	if res.AWin+res.BWin+res.Draw != 6 {
		t.Fatalf("outcomes do not add up: %+v", res)
	}
	if res.ScoreA < 0 || res.ScoreA > 1 {
		t.Fatalf("score out of range: %f", res.ScoreA)
	}
}
