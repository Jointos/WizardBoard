package arena

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"wizardboardgame/internal/ai"
)

// DefaultPath is where the ranking history is stored.
const DefaultPath = "data/rankings.json"

// StartElo is the rating every new engine starts with.
const StartElo = 1200

// RankConfig records the settings a ranking was produced with.
type RankConfig struct {
	MaxTimeMs int64 `json:"maxTimeMs,omitempty"`
	MaxNodes  int   `json:"maxNodes"`
	MaxDepth  int   `json:"maxDepth,omitempty"`
	NoFilter  bool  `json:"noFilter,omitempty"`
}

// EngineRank is the persisted record of one engine.
type EngineRank struct {
	Name   string  `json:"name"`
	Elo    float64 `json:"elo"`
	Games  int     `json:"games"`
	Wins   int     `json:"wins"`
	Draws  int     `json:"draws"`
	Losses int     `json:"losses"`
}

// PairStat is the head-to-head record. AWin is always the alphabetically first
// engine of the pair key.
type PairStat struct {
	Games int `json:"games"`
	AWin  int `json:"aWin"`
	BWin  int `json:"bWin"`
	Draw  int `json:"draw"`
	Error int `json:"error"`
}

// HistoryEntry is one completed match batch.
type HistoryEntry struct {
	Date   time.Time `json:"date"`
	A      string    `json:"a"`
	B      string    `json:"b"`
	Games  int       `json:"games"`
	ScoreA float64   `json:"scoreA"`
	EloA   float64   `json:"eloA"`
	EloB   float64   `json:"eloB"`
}

// Rankings is the persisted ranking table.
type Rankings struct {
	Updated  time.Time              `json:"updated"`
	Config   RankConfig             `json:"config"`
	Engines  map[string]*EngineRank `json:"engines"`
	Pairings map[string]*PairStat   `json:"pairings"`
	History  []HistoryEntry         `json:"history"`
}

// NewRankings creates an empty table populated with every registered engine.
func NewRankings() *Rankings {
	r := &Rankings{
		Engines:  make(map[string]*EngineRank),
		Pairings: make(map[string]*PairStat),
	}
	r.ensure()
	return r
}

func (r *Rankings) ensure() {
	if r.Engines == nil {
		r.Engines = make(map[string]*EngineRank)
	}
	if r.Pairings == nil {
		r.Pairings = make(map[string]*PairStat)
	}
	for _, info := range ai.List() {
		if _, ok := r.Engines[info.ID]; !ok {
			r.Engines[info.ID] = &EngineRank{Name: info.Name, Elo: StartElo}
		}
	}
}

// Load reads the ranking file, creating an empty table when it does not exist.
func Load(path string) (*Rankings, error) {
	if path == "" {
		path = DefaultPath
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return NewRankings(), nil
		}
		return nil, err
	}
	var r Rankings
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	r.ensure()
	return &r, nil
}

// Save writes the table atomically.
func (r *Rankings) Save(path string) error {
	if path == "" {
		path = DefaultPath
	}
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// PairKey returns a stable key for a pair of engines.
func PairKey(a, b string) string {
	if b < a {
		a, b = b, a
	}
	return a + "|" + b
}

// ApplyPair updates ratings and statistics from one match result.
func (r *Rankings) ApplyPair(aID, bID string, res PairResult, cfg RankConfig) (eloA, eloB float64) {
	r.ensure()
	ea := r.Engines[aID]
	eb := r.Engines[bID]
	scored := res.Games - res.Error
	ea.Elo, eb.Elo = Update(ea.Elo, eb.Elo, res.ScoreA, scored, KFactor)

	ea.Games += scored
	ea.Wins += res.AWin
	ea.Draws += res.Draw
	ea.Losses += res.BWin
	eb.Games += scored
	eb.Wins += res.BWin
	eb.Draws += res.Draw
	eb.Losses += res.AWin

	key := PairKey(aID, bID)
	ps := r.Pairings[key]
	if ps == nil {
		ps = &PairStat{}
		r.Pairings[key] = ps
	}
	ps.Games += res.Games
	ps.Error += res.Error
	if key == aID+"|"+bID {
		ps.AWin += res.AWin
		ps.BWin += res.BWin
	} else {
		ps.AWin += res.BWin
		ps.BWin += res.AWin
	}
	ps.Draw += res.Draw

	r.Config = cfg
	r.Updated = time.Now().UTC()
	r.History = append(r.History, HistoryEntry{
		Date:   r.Updated,
		A:      aID,
		B:      bID,
		Games:  res.Games,
		ScoreA: res.ScoreA,
		EloA:   ea.Elo,
		EloB:   eb.Elo,
	})
	return ea.Elo, eb.Elo
}

// Sorted returns the engine IDs ordered by Elo (highest first).
func (r *Rankings) Sorted() []string {
	ids := make([]string, 0, len(r.Engines))
	for id := range r.Engines {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		if r.Engines[ids[i]].Elo == r.Engines[ids[j]].Elo {
			return ids[i] < ids[j]
		}
		return r.Engines[ids[i]].Elo > r.Engines[ids[j]].Elo
	})
	return ids
}
