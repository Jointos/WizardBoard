package ml_r2_v1

import (
	"testing"

	"wizardboardgame/internal/ai"
	"wizardboardgame/internal/game"
)

func benchPosition() game.State {
	s := game.New(nil)
	return s
}

func BenchmarkAnalyzePawn(b *testing.B) {
	s := benchPosition()
	legal := s.LegalActions()
	eng, _ := ai.New(ID, ai.Config{MaxNodes: 2_000_000, MaxDepth: 8, Seed: 1})
	an := eng.(ai.Analyzer)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		res := an.Analyze(s, legal)
		if i == 0 {
			b.Logf("depth=%d moves=%d", res.Depth, len(res.Moves))
		}
	}
}
