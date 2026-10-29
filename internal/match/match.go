package match

import (
	crand "crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"wizardboardgame/internal/ai"
	_ "wizardboardgame/internal/ai/catalog"
	"wizardboardgame/internal/game"
)

var (
	ErrNotFound      = errors.New("match not found")
	ErrBadAction     = errors.New("action index out of range")
	ErrNothingToUndo = errors.New("nothing to undo")
	ErrAITurn        = errors.New("it is the AI's turn")
	ErrStale         = errors.New("match changed while the AI was thinking")
	ErrBadSave       = errors.New("invalid save")
	ErrBadConfig     = errors.New("invalid match config")
)

// Config describes who plays which side and which engine plays the AI seats.
type Config struct {
	AI         [2]bool
	EngineID   string
	Difficulty string
	Seed       int64
}

func (c Config) normalized() Config {
	if c.EngineID == "" {
		c.EngineID = ai.DefaultEngine
	}
	switch c.Difficulty {
	case "fast", "strong", "max":
	default:
		c.Difficulty = "strong"
	}
	if c.Seed == 0 {
		c.Seed = time.Now().UnixNano()
	}
	return c
}

// Entry is one played action plus its human-readable label, captured at the
// moment it was played.
type Entry struct {
	Action game.Action
	Label  string
}

// Match is one running game. It owns the authoritative state and the action
// log, so any position can be rebuilt (replay, undo, debugging, ML training).
type Match struct {
	ID      string
	Rules   *game.Rules
	Cfg     Config
	Base    game.State
	State   game.State
	Log     []Entry
	Created time.Time
	// HumanMoves counts actions taken by a human-controlled seat, so Undo can
	// tell whether there is anything meaningful to roll back.
	HumanMoves int
	// CfgSeq changes whenever the AI configuration is edited; an in-flight AI
	// move from before the edit must not be applied.
	CfgSeq uint32
}

// Snapshot is an immutable copy handed to the API layer.
type Snapshot struct {
	ID         string
	State      game.State
	Legal      []game.Action
	Log        []Entry
	CanUndo    bool
	AI         [2]bool
	EngineID   string
	Difficulty string
	PendingAI  bool
}

// Service keeps all matches in memory. It is safe for concurrent use.
type Service struct {
	mu      sync.Mutex
	matches map[string]*Match
	rules   *game.Rules
	rng     *rand.Rand
}

func NewService(rules *game.Rules) *Service {
	if rules == nil {
		rules = game.DefaultRules()
	}
	return &Service{
		matches: make(map[string]*Match),
		rules:   rules.Clone(),
		rng:     rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func newID() string {
	var b [6]byte
	if _, err := crand.Read(b[:]); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b[:])
}

// Create starts a new local match.
func (s *Service) Create(cfg Config) Snapshot {
	cfg = cfg.normalized()
	s.mu.Lock()
	defer s.mu.Unlock()
	st := game.New(s.rules)
	m := &Match{
		ID:      newID(),
		Rules:   s.rules,
		Cfg:     cfg,
		Base:    st,
		State:   st,
		Created: time.Now(),
	}
	s.matches[m.ID] = m
	return s.snapshotLocked(m)
}

// Get returns the current snapshot of a match.
func (s *Service) Get(id string) (Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, ok := s.matches[id]
	if !ok {
		return Snapshot{}, ErrNotFound
	}
	return s.snapshotLocked(m), nil
}

// Play applies the action at index in the current legal action list. It rejects
// moves for AI-controlled seats.
func (s *Service) Play(id string, index int) (Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, ok := s.matches[id]
	if !ok {
		return Snapshot{}, ErrNotFound
	}
	if m.Cfg.AI[m.State.Turn] {
		return Snapshot{}, ErrAITurn
	}
	legal := m.State.LegalActions()
	if index < 0 || index >= len(legal) {
		return Snapshot{}, ErrBadAction
	}
	return s.applyLocked(m, legal[index])
}

// PlayAI lets the configured engine take the current turn (and its pending free
// move, if a buffed wizard cast). It is idempotent: if it is not an AI turn the
// current snapshot is returned unchanged.
func (s *Service) PlayAI(id string) (Snapshot, error) {
	s.mu.Lock()
	m, ok := s.matches[id]
	if !ok {
		s.mu.Unlock()
		return Snapshot{}, ErrNotFound
	}
	if m.State.Over || !m.Cfg.AI[m.State.Turn] {
		snap := s.snapshotLocked(m)
		s.mu.Unlock()
		return snap, nil
	}
	st := m.State
	cfg := m.Cfg
	ply := st.Ply
	cfgSeq := m.CfgSeq
	entries := make([]Entry, 0, 2)
	s.mu.Unlock()

	// Think outside the lock. A buffed wizard's free move keeps the same player
	// on turn, so loop until the turn actually changes.
	for steps := 0; steps < 4 && !st.Over && cfg.AI[st.Turn]; steps++ {
		legal := st.LegalActions()
		if len(legal) == 0 {
			break
		}
		eng, ok := ai.New(cfg.EngineID, ai.LiveConfig(cfg.Difficulty, cfg.Seed+int64(st.Ply)*7919))
		if !ok {
			eng, _ = ai.New(ai.DefaultEngine, ai.LiveConfig(cfg.Difficulty, cfg.Seed+int64(st.Ply)*7919))
		}
		if eng == nil {
			break
		}
		a := eng.Select(st, legal)
		label := st.Describe(a)
		ns, err := st.Apply(a)
		if err != nil {
			break
		}
		entries = append(entries, Entry{Action: a, Label: label})
		st = ns
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if m.State.Ply != ply || m.CfgSeq != cfgSeq {
		return s.snapshotLocked(m), ErrStale
	}
	if len(entries) > 0 {
		m.State = st
		m.Log = append(m.Log, entries...)
	}
	return s.snapshotLocked(m), nil
}

// UpdateConfig changes the AI seats, engine and difficulty of a running match
// without touching the position or the history. This is how a loaded game can
// continue in a different mode (for example a human-vs-human save continued
// against the AI).
func (s *Service) UpdateConfig(id string, cfg Config) (Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, ok := s.matches[id]
	if !ok {
		return Snapshot{}, ErrNotFound
	}
	seed := cfg.Seed
	cfg = cfg.normalized()
	if seed == 0 {
		cfg.Seed = m.Cfg.Seed
	}
	if _, _, ok := ai.Get(cfg.EngineID); !ok {
		return Snapshot{}, fmt.Errorf("%w: unknown engine %q", ErrBadConfig, cfg.EngineID)
	}
	m.Cfg.AI = cfg.AI
	m.Cfg.EngineID = cfg.EngineID
	m.Cfg.Difficulty = cfg.Difficulty
	m.Cfg.Seed = cfg.Seed
	m.CfgSeq++
	m.HumanMoves = recomputeHumanMoves(m)
	return s.snapshotLocked(m), nil
}

// recomputeHumanMoves replays the log with the current AI seats, so undo keeps
// working after a mode change.
func recomputeHumanMoves(m *Match) int {
	st := m.Base
	n := 0
	for _, e := range m.Log {
		if !m.Cfg.AI[st.Turn] {
			n++
		}
		ns, err := st.Apply(e.Action)
		if err != nil {
			break
		}
		st = ns
	}
	return n
}

// Undo rolls back to the state just before the last human action. Everything
// after it (AI replies, a pending AI turn) is discarded, so in a Human vs AI
// game one click always returns the turn to the human.
func (s *Service) Undo(id string) (Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, ok := s.matches[id]
	if !ok {
		return Snapshot{}, ErrNotFound
	}
	if m.HumanMoves == 0 {
		return Snapshot{}, ErrNothingToUndo
	}

	st := m.Base
	lastHuman := -1
	humanMoves := 0
	for i, e := range m.Log {
		if !m.Cfg.AI[st.Turn] {
			lastHuman = i
			humanMoves++
		}
		ns, err := st.Apply(e.Action)
		if err != nil {
			break
		}
		st = ns
	}
	if lastHuman < 0 {
		return Snapshot{}, ErrNothingToUndo
	}

	st = m.Base
	for _, e := range m.Log[:lastHuman] {
		ns, err := st.Apply(e.Action)
		if err != nil {
			return Snapshot{}, err
		}
		st = ns
	}
	m.Log = append([]Entry(nil), m.Log[:lastHuman]...)
	m.State = st
	m.HumanMoves = humanMoves - 1
	return s.snapshotLocked(m), nil
}

// PlayRandom is a development helper: it plays one uniformly random legal
// action. Useful for smoke-testing the UI.
func (s *Service) PlayRandom(id string) (Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, ok := s.matches[id]
	if !ok {
		return Snapshot{}, ErrNotFound
	}
	legal := m.State.LegalActions()
	if len(legal) == 0 {
		return Snapshot{}, ErrBadAction
	}
	return s.applyLocked(m, legal[s.rng.Intn(len(legal))])
}

func (s *Service) applyLocked(m *Match, a game.Action) (Snapshot, error) {
	label := m.State.Describe(a)
	if !m.Cfg.AI[m.State.Turn] {
		m.HumanMoves++
	}
	ns, err := m.State.Apply(a)
	if err != nil {
		return Snapshot{}, err
	}
	m.State = ns
	m.Log = append(m.Log, Entry{Action: a, Label: label})
	return s.snapshotLocked(m), nil
}

func (s *Service) snapshotLocked(m *Match) Snapshot {
	log := make([]Entry, len(m.Log))
	copy(log, m.Log)
	return Snapshot{
		ID:         m.ID,
		State:      m.State,
		Legal:      m.State.LegalActions(),
		Log:        log,
		CanUndo:    m.HumanMoves > 0,
		AI:         m.Cfg.AI,
		EngineID:   m.Cfg.EngineID,
		Difficulty: m.Cfg.Difficulty,
		PendingAI:  !m.State.Over && m.Cfg.AI[m.State.Turn],
	}
}

// Export is a complete, replayable description of a match: config, rules and
// the action log. It is the basis of the debug save files.
type Export struct {
	ID         string
	Engine     string
	Difficulty string
	AI         [2]bool
	Seed       int64
	Rules      *game.Rules
	Log        []Entry
	StateHash  uint64
}

// Export snapshots a match for saving.
func (s *Service) Export(id string) (Export, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, ok := s.matches[id]
	if !ok {
		return Export{}, ErrNotFound
	}
	log := make([]Entry, len(m.Log))
	copy(log, m.Log)
	return Export{
		ID:         m.ID,
		Engine:     m.Cfg.EngineID,
		Difficulty: m.Cfg.Difficulty,
		AI:         m.Cfg.AI,
		Seed:       m.Cfg.Seed,
		Rules:      m.Rules.Clone(),
		Log:        log,
		StateHash:  m.State.Hash(),
	}, nil
}

// Import validates and replays an exported match, registering it as a new
// match. The action log is authoritative; labels are regenerated and the final
// state hash is verified when present.
func (s *Service) Import(exp Export) (Snapshot, error) {
	rules := exp.Rules.Clone()
	base := game.New(rules)
	st := base
	log := make([]Entry, 0, len(exp.Log))
	humanMoves := 0
	for i, e := range exp.Log {
		if !exp.AI[st.Turn] {
			humanMoves++
		}
		label := st.Describe(e.Action)
		ns, err := st.Apply(e.Action)
		if err != nil {
			return Snapshot{}, fmt.Errorf("%w: action %d: %v", ErrBadSave, i, err)
		}
		log = append(log, Entry{Action: e.Action, Label: label})
		st = ns
	}
	if exp.StateHash != 0 && st.Hash() != exp.StateHash {
		return Snapshot{}, fmt.Errorf("%w: state hash mismatch", ErrBadSave)
	}

	cfg := Config{
		AI:         exp.AI,
		EngineID:   exp.Engine,
		Difficulty: exp.Difficulty,
		Seed:       exp.Seed,
	}.normalized()

	m := &Match{
		ID:         newID(),
		Rules:      rules,
		Cfg:        cfg,
		Base:       base,
		State:      st,
		Log:        log,
		Created:    time.Now(),
		HumanMoves: humanMoves,
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.matches[m.ID] = m
	return s.snapshotLocked(m), nil
}
