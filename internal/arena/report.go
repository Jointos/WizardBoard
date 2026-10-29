package arena

import (
	"bytes"
	_ "embed"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"sort"
	"time"

	"wizardboardgame/internal/ai"
	"wizardboardgame/internal/game"
)

//go:embed report.html
var reportHTML string

// reportView is the data the HTML template renders.
type reportView struct {
	Source         string
	Generated      string
	Config         StatsConfig
	Runs           int
	StatsOnlyGames int
	OverallGames   int
	Balance        SideStats
	Verdict        string
	VerdictClass   string
	SampleNote     string
	Pairs          []pairView
	Engines        []engineView
	Overall        EventStats
	BySide         map[string]*EventStats
	Endgame        EndStats
	Recent         []RunEntry
	PieceTypes     []string
	PlyBuckets     []plyBucketView
	MaxBucketGames int
	BestMoves      []bestMoveView
	BestMoveConfig *BestMoveConfig
}

type pairView struct {
	Key   string
	A     string
	B     string
	Stats *PairStats
	Color string // "even", "a", "b" for the summary hint
}

type engineView struct {
	ID     string
	Name   string
	Elo    float64
	Games  int
	Wins   int
	Draws  int
	Losses int
	Events EventStats
}

type plyBucketView struct {
	Label string
	Games int
	AWin  int
	BWin  int
	Draw  int
}

type bestMoveView struct {
	Rank       int
	Game       int
	Pair       string
	Ply        int
	Mover      string
	Engine     string
	Winner     string
	Action     game.Action
	Label      string
	Before     game.State
	After      game.State
	EvalBefore int
	EvalAfter  int
	Swing      int
	Uniqueness int
	SecondBest int
	Turnaround bool
	Phase      string
	TokensA    int
	TokensB    int
}

var pieceTypes = []string{"pawn", "cannon", "wizard"}

func buildReport(source string, sf *StatsFile) reportView {
	sf.ensure()
	v := reportView{
		Source:         source,
		Generated:      time.Now().Format("2006-01-02 15:04"),
		Config:         sf.Config,
		Runs:           sf.Runs,
		StatsOnlyGames: sf.StatsOnlyGames,
		OverallGames:   sf.Overall.Games,
		Balance:        sf.Sides,
		Overall:        sf.Overall,
		BySide:         sf.BySide,
		Endgame:        sf.Endgame,
		Recent:         sf.RecentRuns,
		PieceTypes:     pieceTypes,
	}
	v.Verdict, v.VerdictClass = sideVerdict(sf.Sides)
	v.SampleNote = sampleNote(sf)

	// Matchups, sorted by key for a stable report.
	keys := make([]string, 0, len(sf.AIVsAI.ByPair))
	for key := range sf.AIVsAI.ByPair {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		parts := splitPairKey(key)
		v.Pairs = append(v.Pairs, pairView{Key: key, A: parts[0], B: parts[1], Stats: sf.AIVsAI.ByPair[key]})
	}

	// Per-engine event totals, aggregated from the pair-local records (this
	// includes stats-only runs, unlike the ranking-mirrored byEngine block).
	events := make(map[string]*EventStats)
	for _, ps := range sf.AIVsAI.ByPair {
		for id, pe := range ps.ByEngine {
			if events[id] == nil {
				empty := newEventStats()
				events[id] = &empty
			}
			events[id].Add(&pe.Events)
		}
	}
	seen := make(map[string]bool)
	for _, e := range sf.Ranking.Engines {
		ev := events[e.ID]
		if ev == nil {
			empty := newEventStats()
			ev = &empty
		}
		v.Engines = append(v.Engines, engineView{
			ID: e.ID, Name: e.Name, Elo: e.Elo,
			Games: e.Games, Wins: e.Wins, Draws: e.Draws, Losses: e.Losses,
			Events: *ev,
		})
		seen[e.ID] = true
	}
	for id, ev := range events {
		if seen[id] {
			continue
		}
		name := id
		if info, _, ok := ai.Get(id); ok {
			name = info.Name
		}
		v.Engines = append(v.Engines, engineView{ID: id, Name: name, Events: *ev})
	}
	sort.SliceStable(v.Engines, func(i, j int) bool {
		if v.Engines[i].Elo == v.Engines[j].Elo {
			return v.Engines[i].ID < v.Engines[j].ID
		}
		return v.Engines[i].Elo > v.Engines[j].Elo
	})

	labels := []string{"<50", "50–99", "100–149", "150–199", "200–249", "250+"}
	for i, label := range labels {
		games := sf.Endgame.PlyBuckets[i]
		if games > v.MaxBucketGames {
			v.MaxBucketGames = games
		}
		v.PlyBuckets = append(v.PlyBuckets, plyBucketView{
			Label: label,
			Games: games,
			AWin:  sf.Endgame.AWinByBucket[i],
			BWin:  sf.Endgame.BWinByBucket[i],
			Draw:  sf.Endgame.DrawByBucket[i],
		})
	}
	for _, m := range sf.BestMoves {
		v.BestMoves = append(v.BestMoves, bestMoveView{
			Rank:       m.Rank,
			Game:       m.Game,
			Pair:       m.Pair,
			Ply:        m.Ply,
			Mover:      m.Mover.String(),
			Engine:     m.Engine,
			Winner:     m.Winner.String(),
			Action:     m.Action,
			Label:      m.Label,
			Before:     m.Before,
			After:      m.After,
			EvalBefore: m.EvalBefore,
			EvalAfter:  m.EvalAfter,
			Swing:      m.Swing,
			Uniqueness: m.Uniqueness,
			SecondBest: m.SecondBest,
			Turnaround: m.Turnaround,
			Phase:      m.Phase,
			TokensA:    m.TokensA,
			TokensB:    m.TokensB,
		})
	}
	v.BestMoveConfig = sf.BestMoveConfig
	return v
}

func splitPairKey(key string) [2]string {
	for i := 0; i < len(key); i++ {
		if key[i] == '|' {
			return [2]string{key[:i], key[i+1:]}
		}
	}
	return [2]string{key, ""}
}

func sideVerdict(s SideStats) (string, string) {
	if s.Games == 0 {
		return "no side data yet", "muted"
	}
	if lo, _ := Wilson(s.AWin, s.Games); lo > 0.5 {
		return "Player A edge (95% CI)", "warn"
	}
	if lo, _ := Wilson(s.BWin, s.Games); lo > 0.5 {
		return "Player B edge (95% CI)", "warn"
	}
	return "balanced (95% CI includes 50%)", "ok"
}

func sampleNote(sf *StatsFile) string {
	if sf.Sides.Games == 0 {
		return "Side stats start from the first run after the upgrade; older games have no side information."
	}
	total := sf.Overall.Games
	if total < sf.Sides.Games {
		total = sf.Sides.Games
	}
	note := fmt.Sprintf("Side sample: %d of %d games have a recorded winner side.", sf.Sides.Games, total)
	if sf.Sides.Games < 100 {
		note += " Small sample: the 95% interval is wide, so treat a few percent as noise."
	}
	if sf.StatsOnlyGames > 0 {
		note += fmt.Sprintf(" %d games are stats-only (not in Elo).", sf.StatsOnlyGames)
	}
	return note
}

// RenderHTML renders the self-contained report.
func RenderHTML(source string, sf *StatsFile) ([]byte, error) {
	tmpl, err := template.New("report").Funcs(template.FuncMap{
		"pct":       pct,
		"ci":        ci,
		"bar":       bar,
		"avg":       avg,
		"sumMap":    sumMap,
		"maxTotals": maxTotals,
		"maxMatrix": maxMatrix,
		"heat":      heat,
		"add":       func(a, b int) int { return a + b },
		"sub":       func(a, b int) int { return a - b },
		"dur":       dur,
		"date":      date,
		"score":     scorePct,
		"avgDur":    avgDur,
		"eval":      evalText,
		"swingText": swingText,
		"board": func(s game.State, a game.Action) template.HTML {
			return BoardSVG(&s, &a)
		},
		"boardAfter": func(s game.State) template.HTML {
			return BoardSVG(&s, nil)
		},
		"lower": func(s string) string { return s },
		"verdict": func(s SideStats) string {
			text, _ := sideVerdict(s)
			return text
		},
		"verdictClass": func(s SideStats) string {
			_, class := sideVerdict(s)
			return class
		},
	}).Parse(reportHTML)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, buildReport(source, sf)); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// WriteHTML renders the report and writes it to path.
func WriteHTML(path, source string, sf *StatsFile) error {
	data, err := RenderHTML(source, sf)
	if err != nil {
		return err
	}
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return os.WriteFile(path, data, 0o644)
}

func pct(n, total int) string {
	if total <= 0 {
		return "–"
	}
	return fmt.Sprintf("%.1f%%", 100*float64(n)/float64(total))
}

func ci(wins, games int) string {
	if games <= 0 {
		return "–"
	}
	lo, hi := Wilson(wins, games)
	return fmt.Sprintf("%.1f%% [%.1f–%.1f]", 100*float64(wins)/float64(games), 100*lo, 100*hi)
}

func bar(n, total int) string {
	if total <= 0 {
		return "0"
	}
	return fmt.Sprintf("%.2f", 100*float64(n)/float64(total))
}

func avg(total, n int) string {
	if n == 0 {
		return "–"
	}
	return fmt.Sprintf("%.1f", float64(total)/float64(n))
}

func sumMap(m map[string]int) int {
	total := 0
	for _, v := range m {
		total += v
	}
	return total
}

func maxTotals(m UnitTotals) int {
	max := 0
	for _, v := range m {
		if v > max {
			max = v
		}
	}
	return max
}

func maxMatrix(m map[string]UnitTotals) int {
	max := 0
	for _, row := range m {
		for _, v := range row {
			if v > max {
				max = v
			}
		}
	}
	return max
}

func heat(v, max int) string {
	if max <= 0 {
		return "0.06"
	}
	return fmt.Sprintf("%.3f", 0.06+0.5*float64(v)/float64(max))
}

func dur(ms int64) string {
	if ms <= 0 {
		return "–"
	}
	d := time.Duration(ms) * time.Millisecond
	if d < time.Minute {
		return d.Round(time.Second).String()
	}
	return d.Round(time.Second).String()
}

// scorePct is the average match score (draw = half a win) as a percentage.
func scorePct(wins, draws, games int) string {
	if games <= 0 {
		return "–"
	}
	return fmt.Sprintf("%.1f%%", 100*(float64(wins)+0.5*float64(draws))/float64(games))
}

// avgDur is the average time per game.
func avgDur(ms int64, games int) string {
	if games <= 0 {
		return "–"
	}
	return dur(ms / int64(games))
}

// evalText renders a search eval, collapsing forced results into +win/-win.
func evalText(v int) string {
	switch {
	case v >= 900000:
		return "+win"
	case v <= -900000:
		return "-win"
	case v == 0:
		return "0"
	default:
		return fmt.Sprintf("%+d", v)
	}
}

// swingText describes the impact of a move; a forced win is clearer than a
// six-digit centipawn number.
func swingText(after, swing int) string {
	if after >= 900000 {
		return "forced win"
	}
	if after <= -900000 {
		return "forced loss"
	}
	if swing == 0 {
		return "0"
	}
	return fmt.Sprintf("%+d", swing)
}

func date(t time.Time) string {
	if t.IsZero() {
		return "–"
	}
	return t.Local().Format("2006-01-02 15:04")
}
