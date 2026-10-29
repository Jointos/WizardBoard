// Package ai contains pluggable engines. Every engine lives in its own
// versioned package (e.g. alphabeta_v1) and registers itself here with a stable
// ID. Old versions are never modified, so their behaviour and their ranking
// history stay comparable when a new version is added.
package ai

import (
	"sort"
	"sync"
	"time"

	"wizardboardgame/internal/game"
)

// DefaultEngine is the engine used when a match does not specify one.
const DefaultEngine = "ml-r2-v2"

// Config configures one engine instance. MaxTime is used for live play; the
// arena uses the same time budget with MaxDepth as a cap.
type Config struct {
	MaxTime  time.Duration
	MaxNodes int
	MaxDepth int
	Seed     int64
	// DisableFilter turns off the heuristic cannon move+turn filter. Engines
	// that support it (the r2 engines) expose this for ablations; older versions ignore it.
	DisableFilter bool
}

// Info is the public metadata of an engine.
type Info struct {
	ID    string
	Name  string
	Notes string
}

// Engine chooses one of the legal actions for the side to move.
type Engine interface {
	Name() string
	Select(s game.State, legal []game.Action) game.Action
}

// ScoredAction is one analysed move with its engine score.
//
// Bound reports that Score is only an upper bound (the move is clearly worse
// than the analysed band), not an exact value. Bound moves are always outside
// the "good moves" counts, so their exact score is irrelevant for balance work.
type ScoredAction struct {
	Action game.Action
	Score  int
	Bound  bool
}

// Analysis is the ranked result of an engine analysis.
type Analysis struct {
	// Depth is the depth the Engine move was chosen at.
	Depth int
	// ListDepth is the depth the Scores in Moves come from. The scoring pass is
	// more expensive than the best-move search, so it can be one or two plies
	// shallower.
	ListDepth int
	// Engine is the move the engine itself would play (same search as live
	// play, at Depth).
	Engine ScoredAction
	// Moves are all candidates with their scores at ListDepth, sorted best
	// first. Bound means the score is only an upper bound.
	Moves []ScoredAction
}

// ScoredEngine can also return the score of its chosen move. The dataset
// generator uses it to create dense value labels.
type ScoredEngine interface {
	Engine
	SelectScore(s game.State, legal []game.Action) (game.Action, int)
}

// Analyzer is implemented by engines that can rank moves (balance analysis).
type Analyzer interface {
	Analyze(s game.State, legal []game.Action) Analysis
}

// Factory builds an engine instance from a config.
type Factory func(Config) Engine

type registered struct {
	info    Info
	factory Factory
}

var (
	mu       sync.RWMutex
	registry = make(map[string]registered)
)

// Register adds an engine. It is meant to be called from init().
func Register(info Info, f Factory) {
	if info.ID == "" {
		panic("ai: empty engine ID")
	}
	if f == nil {
		panic("ai: nil factory for " + info.ID)
	}
	mu.Lock()
	defer mu.Unlock()
	if _, exists := registry[info.ID]; exists {
		panic("ai: duplicate engine ID " + info.ID)
	}
	registry[info.ID] = registered{info: info, factory: f}
}

// Get returns the metadata and factory of an engine.
func Get(id string) (Info, Factory, bool) {
	mu.RLock()
	defer mu.RUnlock()
	r, ok := registry[id]
	return r.info, r.factory, ok
}

// New builds an engine by ID.
func New(id string, cfg Config) (Engine, bool) {
	_, f, ok := Get(id)
	if !ok {
		return nil, false
	}
	return f(cfg), true
}

// List returns every registered engine, sorted by ID.
func List() []Info {
	mu.RLock()
	defer mu.RUnlock()
	out := make([]Info, 0, len(registry))
	for _, r := range registry {
		out = append(out, r.info)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// LiveConfig maps a difficulty name to a live-play time budget.
func LiveConfig(difficulty string, seed int64) Config {
	switch difficulty {
	case "fast":
		return Config{MaxTime: 400 * time.Millisecond, DisableFilter: true, Seed: seed}
	case "max":
		return Config{MaxTime: 2500 * time.Millisecond, DisableFilter: true, Seed: seed}
	default: // "strong"
		return Config{MaxTime: 1500 * time.Millisecond, DisableFilter: true, Seed: seed}
	}
}
