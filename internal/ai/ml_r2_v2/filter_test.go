package ml_r2_v2

import (
	"math"
	"testing"

	"wizardboardgame/internal/ai"
	"wizardboardgame/internal/game"
)

func TestFilterRemovesSomeCannonMoves(t *testing.T) {
	s := game.New(nil)
	// A pawn, B pawn -> A cannon phase.
	for i := 0; i < 2; i++ {
		legal := s.LegalActions()
		if len(legal) == 0 {
			t.Fatal("no legal actions")
		}
		s, _ = s.Apply(legal[0])
	}
	legal := s.LegalActions()
	filtered := filterCandidates(&s, legal)
	if len(filtered) >= len(legal) {
		t.Fatalf("filter did not reduce candidates: %d vs %d", len(filtered), len(legal))
	}
}

func TestFeaturesShape(t *testing.T) {
	s := game.New(nil)
	x := Features(&s)
	if len(x) != FeatureCount {
		t.Fatalf("features = %d, want %d", len(x), FeatureCount)
	}
	for i, v := range x {
		if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
			t.Fatalf("feature %d is not finite: %v", i, v)
		}
	}
}

func TestSelectWithAndWithoutFilter(t *testing.T) {
	s := game.New(nil)
	legal := s.LegalActions()
	for _, disable := range []bool{false, true} {
		eng := New(ai.Config{MaxDepth: 2, Seed: 1, DisableFilter: disable})
		a := eng.Select(s, legal)
		if _, err := s.Apply(a); err != nil {
			t.Fatalf("illegal move (disable=%v): %v", disable, err)
		}
	}
}
