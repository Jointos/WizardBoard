package ml_r2_v3

import (
	"math/rand"
	"time"

	"wizardboardgame/internal/ai"
	"wizardboardgame/internal/game"
)

const (
	ID       = "ml-r2-v3"
	winScore = 1_000_000
	inf      = 1 << 30
	maxPly   = 24
)

func init() {
	ai.Register(ai.Info{
		ID:    ID,
		Name:  "ML r2 v3",
		Notes: "Alpha-beta + richer r2 features + 64 hidden MLP (fast float32 inference).",
	}, New)
}

type engine struct {
	cfg ai.Config
	rng *rand.Rand
}

// New builds the engine. With no limits configured it defaults to a 1.5s live
// budget, which is what the server uses in "strong" mode.
func New(cfg ai.Config) ai.Engine {
	if cfg.MaxTime <= 0 && cfg.MaxNodes <= 0 && cfg.MaxDepth <= 0 {
		cfg.MaxTime = 1500 * time.Millisecond
	}
	if cfg.MaxDepth <= 0 {
		cfg.MaxDepth = 8
	}
	return &engine{cfg: cfg, rng: rand.New(rand.NewSource(cfg.Seed))}
}

func (e *engine) Name() string { return ID }

func (e *engine) Select(s game.State, legal []game.Action) game.Action {
	a, _ := e.SelectScore(s, legal)
	return a
}

func (e *engine) SelectScore(s game.State, legal []game.Action) (game.Action, int) {
	if len(legal) == 0 {
		return game.Action{}, 0
	}
	if len(legal) == 1 || s.Over {
		return legal[0], 0
	}

	cands := legal
	if !e.cfg.DisableFilter {
		cands = filterCandidates(&s, legal)
	}
	if len(cands) == 0 {
		cands = legal
	}
	// Copy so shuffling/rotating never touches the caller's slice.
	cands = append([]game.Action(nil), cands...)
	e.rng.Shuffle(len(cands), func(i, j int) { cands[i], cands[j] = cands[j], cands[i] })

	se := &search{
		maxNodes: e.cfg.MaxNodes,
		tt:       make(map[uint64]entry, 1<<14),
	}
	if e.cfg.MaxTime > 0 {
		se.deadline = time.Now().Add(e.cfg.MaxTime)
	}
	for i := range se.bufs {
		se.bufs[i] = make([]game.Action, 0, 384)
		se.ord[i] = make([]game.Action, 0, 384)
	}

	best := cands[0]
	bestScore := 0
	for depth := 1; depth <= e.cfg.MaxDepth; depth++ {
		se.stopped = false
		b, sc := se.searchRoot(&s, depth, cands)
		if se.stopped {
			break
		}
		best = b
		bestScore = sc
		// Keep the best first for the next, deeper iteration.
		for i := range cands {
			if cands[i] == best {
				cands[0], cands[i] = cands[i], cands[0]
				break
			}
		}
	}
	return best, bestScore
}
