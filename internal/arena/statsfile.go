package arena

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"wizardboardgame/internal/ai"
)

// Side event maps use these keys. They are the same as game.Player.String().
const (
	SideKeyA = "A"
	SideKeyB = "B"

	// MaxRecentRuns caps the per-run log in the stats file.
	MaxRecentRuns = 100
)

// StatsConfig records the settings a stats file was produced with.
type StatsConfig struct {
	GamesPerPair int    `json:"gamesPerPair"`
	MaxTimeMs    int64  `json:"maxTimeMs"`
	MaxDepth     int    `json:"maxDepth"`
	MaxNodes     int    `json:"maxNodes"`
	NoFilter     bool   `json:"noFilter,omitempty"`
	Paired       bool   `json:"paired,omitempty"`
	StatsOnly    bool   `json:"statsOnly,omitempty"`
	BestMove     bool   `json:"bestMove,omitempty"`
	Seed         int64  `json:"seed"`
	Mode         string `json:"mode"`
}

// RankingEngine is one row of the Elo snapshot embedded in the stats file.
type RankingEngine struct {
	ID     string  `json:"id"`
	Name   string  `json:"name"`
	Elo    float64 `json:"elo"`
	Games  int     `json:"games"`
	Wins   int     `json:"wins"`
	Draws  int     `json:"draws"`
	Losses int     `json:"losses"`
}

// RankingSnapshot is the Elo table at the time the stats were last updated.
type RankingSnapshot struct {
	UpdatedAt time.Time       `json:"updatedAt"`
	Engines   []RankingEngine `json:"engines"`
}

// EngineStats is the global per-engine record. Elo and W/D/L mirror the
// ranking table; Events are the accumulated in-game events.
type EngineStats struct {
	Name   string     `json:"name"`
	Elo    float64    `json:"elo"`
	Games  int        `json:"games"`
	Wins   int        `json:"wins"`
	Draws  int        `json:"draws"`
	Losses int        `json:"losses"`
	Events EventStats `json:"events"`
}

// PairEngineStats is one engine's record inside a head-to-head pair.
type PairEngineStats struct {
	Games  int        `json:"games"`
	Wins   int        `json:"wins"`
	Draws  int        `json:"draws"`
	Losses int        `json:"losses"`
	Events EventStats `json:"events"`
}

// PairStats is the cumulative head-to-head record of one pair.
type PairStats struct {
	Games    int                         `json:"games"`
	AWin     int                         `json:"aWin"`
	BWin     int                         `json:"bWin"`
	Draw     int                         `json:"draw"`
	Error    int                         `json:"error"`
	Plies    int                         `json:"plies"`
	Sides    SideStats                   `json:"sides"`
	Endgame  EndStats                    `json:"endgame"`
	BySide   map[string]*EventStats      `json:"bySide,omitempty"`
	Events   EventStats                  `json:"events"`
	ByEngine map[string]*PairEngineStats `json:"byEngine"`
}

// AIVsAI is the engine-level part of the stats file.
type AIVsAI struct {
	ByEngine map[string]*EngineStats `json:"byEngine"`
	ByPair   map[string]*PairStats   `json:"byPair"`
}

// RunEntry is one recorded batch (one pair, one invocation).
type RunEntry struct {
	At        time.Time   `json:"at"`
	A         string      `json:"a"`
	B         string      `json:"b"`
	Games     int         `json:"games"`
	AWin      int         `json:"aWin"`
	BWin      int         `json:"bWin"`
	Draw      int         `json:"draw"`
	Error     int         `json:"error"`
	Sides     SideStats   `json:"sides"`
	ScoreA    float64     `json:"scoreA"`
	TookMs    int64       `json:"tookMs"`
	StatsOnly bool        `json:"statsOnly,omitempty"`
	Config    StatsConfig `json:"config"`
}

// StatsFile is the balance/statistics document written by the arena.
type StatsFile struct {
	GeneratedAt    time.Time              `json:"generatedAt"`
	Config         StatsConfig            `json:"config"`
	Runs           int                    `json:"runs"`
	Sides          SideStats              `json:"sides"`
	BySide         map[string]*EventStats `json:"bySide,omitempty"`
	Endgame        EndStats               `json:"endgame"`
	StatsOnlyGames int                    `json:"statsOnlyGames,omitempty"`
	BestMoves      []BestMove             `json:"bestMoves,omitempty"`
	BestMoveConfig *BestMoveConfig        `json:"bestMoveConfig,omitempty"`
	Ranking        RankingSnapshot        `json:"ranking"`
	Overall        EventStats             `json:"overall"`
	AIVsAI         AIVsAI                 `json:"aiVsAi"`
	RecentRuns     []RunEntry             `json:"recentRuns,omitempty"`
}

// NewStatsFile returns an empty stats document.
func NewStatsFile() *StatsFile {
	return &StatsFile{
		BySide: newSideEventMap(),
		AIVsAI: AIVsAI{
			ByEngine: make(map[string]*EngineStats),
			ByPair:   make(map[string]*PairStats),
		},
	}
}

func (sf *StatsFile) ensure() {
	if sf.AIVsAI.ByEngine == nil {
		sf.AIVsAI.ByEngine = make(map[string]*EngineStats)
	}
	if sf.AIVsAI.ByPair == nil {
		sf.AIVsAI.ByPair = make(map[string]*PairStats)
	}
	if sf.BySide == nil {
		sf.BySide = newSideEventMap()
	}
	for _, side := range []string{SideKeyA, SideKeyB} {
		if sf.BySide[side] == nil {
			empty := newEventStats()
			sf.BySide[side] = &empty
		}
		sf.BySide[side].ensure()
	}
	sf.Overall.ensure()
	sf.Endgame.ensure()
	for _, ps := range sf.AIVsAI.ByPair {
		if ps.ByEngine == nil {
			ps.ByEngine = make(map[string]*PairEngineStats)
		}
		if ps.BySide == nil {
			ps.BySide = newSideEventMap()
		}
		for _, side := range []string{SideKeyA, SideKeyB} {
			if ps.BySide[side] == nil {
				empty := newEventStats()
				ps.BySide[side] = &empty
			}
			ps.BySide[side].ensure()
		}
		ps.Events.ensure()
		ps.Endgame.ensure()
	}
	for _, es := range sf.AIVsAI.ByEngine {
		es.Events.ensure()
	}
	if len(sf.RecentRuns) > MaxRecentRuns {
		sf.RecentRuns = sf.RecentRuns[len(sf.RecentRuns)-MaxRecentRuns:]
	}
}

// LoadStats reads a stats file, creating an empty one when it does not exist.
func LoadStats(path string) (*StatsFile, error) {
	sf := NewStatsFile()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return sf, nil
		}
		return nil, err
	}
	if err := json.Unmarshal(data, sf); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	sf.ensure()
	return sf, nil
}

// SaveStats writes the stats file atomically.
func SaveStats(path string, sf *StatsFile) error {
	sf.ensure()
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	data, err := json.MarshalIndent(sf, "", "  ")
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

// RefreshRanking copies the latest Elo table into the stats file and updates
// the per-engine records (games/wins/draws/losses) to their current values.
func RefreshRanking(sf *StatsFile, ranks *Rankings) {
	snap := RankingSnapshot{
		UpdatedAt: ranks.Updated,
		Engines:   make([]RankingEngine, 0, len(ranks.Engines)),
	}
	for _, id := range ranks.Sorted() {
		e := ranks.Engines[id]
		snap.Engines = append(snap.Engines, RankingEngine{
			ID: id, Name: e.Name, Elo: e.Elo,
			Games: e.Games, Wins: e.Wins, Draws: e.Draws, Losses: e.Losses,
		})
		if eng := sf.AIVsAI.ByEngine[id]; eng != nil {
			eng.Elo = e.Elo
			eng.Games = e.Games
			eng.Wins = e.Wins
			eng.Draws = e.Draws
			eng.Losses = e.Losses
			if eng.Name == "" {
				eng.Name = e.Name
			}
		}
	}
	sf.Ranking = snap
}

// MergeRun adds one pair result to the stats file. With statsOnly the run is
// recorded in the balance data but the global per-engine event table (which
// mirrors the ranking) is left alone.
func MergeRun(sf *StatsFile, ranks *Rankings, aID, bID string, res PairResult, cfg StatsConfig, took time.Duration, statsOnly bool) {
	sf.ensure()
	sf.Runs++
	sf.Config = cfg
	sf.GeneratedAt = time.Now().UTC()

	if res.Stats != nil {
		sf.Overall.Add(&res.Stats.Overall)
		mergeSideEvents(sf.BySide, res.Stats.BySide)
	}
	sf.Sides.Add(res.Sides)
	sf.Endgame.Add(&res.Endgame)
	if statsOnly {
		sf.StatsOnlyGames += res.Games - res.Error
	}

	key := PairKey(aID, bID)
	ps := sf.AIVsAI.ByPair[key]
	if ps == nil {
		ps = &PairStats{ByEngine: make(map[string]*PairEngineStats), BySide: newSideEventMap()}
		sf.AIVsAI.ByPair[key] = ps
	}
	if ps.ByEngine == nil {
		ps.ByEngine = make(map[string]*PairEngineStats)
	}
	if ps.BySide == nil {
		ps.BySide = newSideEventMap()
	}
	ps.Games += res.Games
	ps.Plies += res.Plies
	ps.Error += res.Error
	ps.Draw += res.Draw
	ps.Sides.Add(res.Sides)
	ps.Endgame.Add(&res.Endgame)
	if res.Stats != nil {
		ps.Events.Add(&res.Stats.Overall)
		mergeSideEvents(ps.BySide, res.Stats.BySide)
	}
	if key == aID+"|"+bID {
		ps.AWin += res.AWin
		ps.BWin += res.BWin
	} else {
		ps.AWin += res.BWin
		ps.BWin += res.AWin
	}

	scored := res.Games - res.Error
	if res.Stats != nil {
		for id, es := range res.Stats.ByEngine {
			// Pair-local record always includes the run.
			pe := ps.ByEngine[id]
			if pe == nil {
				pe = &PairEngineStats{}
				ps.ByEngine[id] = pe
			}
			pe.Games += scored
			pe.Events.Add(es)
			switch id {
			case aID:
				pe.Wins += res.AWin
				pe.Losses += res.BWin
				pe.Draws += res.Draw
			case bID:
				pe.Wins += res.BWin
				pe.Losses += res.AWin
				pe.Draws += res.Draw
			}
			if statsOnly {
				continue
			}
			eng := sf.AIVsAI.ByEngine[id]
			if eng == nil {
				info, _, _ := ai.Get(id)
				eng = &EngineStats{Name: info.Name}
				sf.AIVsAI.ByEngine[id] = eng
			}
			eng.Events.Add(es)
		}
	}

	sf.RecentRuns = append(sf.RecentRuns, RunEntry{
		At:        sf.GeneratedAt,
		A:         aID,
		B:         bID,
		Games:     res.Games,
		AWin:      res.AWin,
		BWin:      res.BWin,
		Draw:      res.Draw,
		Error:     res.Error,
		Sides:     res.Sides,
		ScoreA:    res.ScoreA,
		TookMs:    took.Milliseconds(),
		StatsOnly: statsOnly,
		Config:    cfg,
	})
	if len(sf.RecentRuns) > MaxRecentRuns {
		sf.RecentRuns = sf.RecentRuns[len(sf.RecentRuns)-MaxRecentRuns:]
	}

	RefreshRanking(sf, ranks)
}

func mergeSideEvents(dst, src map[string]*EventStats) {
	if dst == nil {
		dst = newSideEventMap()
	}
	for side, st := range src {
		if dst[side] == nil {
			empty := newEventStats()
			dst[side] = &empty
		}
		dst[side].Add(st)
	}
}
