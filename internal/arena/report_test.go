package arena

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"wizardboardgame/internal/game"
)

func TestRenderHTML(t *testing.T) {
	sf := NewStatsFile()
	sf.Runs = 3
	sf.Config = StatsConfig{GamesPerPair: 10, MaxTimeMs: 1500, MaxDepth: 6, NoFilter: true, Paired: true, Mode: "pairs"}
	sf.GeneratedAt = time.Now().UTC()
	sf.Sides = SideStats{Games: 100, AWin: 55, BWin: 40, Draw: 5}

	overall := newEventStats()
	overall.Games, overall.Plies = 100, 3000
	overall.Actions["move"] = 900
	overall.Actions["capture"] = 50
	overall.Kills.ByVictim["wizard"] = 100
	overall.Kills.ByAttacker["cannon"]["wizard"] = 60
	sf.Overall = overall
	sf.BySide = newSideEventMap()
	sf.BySide["A"].Games = 100
	sf.BySide["A"].Actions["move"] = 500
	sf.BySide["B"].Games = 100
	sf.BySide["B"].Actions["move"] = 450

	sf.Endgame = EndStats{Games: 100, Plies: 3000, Stalemates: 5, FirstKillA: 50, FirstKillB: 40, NoFirstKill: 10, FirstKillWins: 70, FirstKillDecided: 90}
	sf.Endgame.ensure()
	sf.Endgame.ASurvivors["wizard"] = 150
	sf.Endgame.BSurvivors["wizard"] = 100
	sf.Endgame.PlyBuckets[1] = 100
	sf.Endgame.AWinByBucket[1] = 55
	sf.Endgame.BWinByBucket[1] = 40
	sf.Endgame.DrawByBucket[1] = 5

	sf.Ranking.Engines = []RankingEngine{
		{ID: "ml-r2-v1", Name: "<ML v2>", Elo: 1330, Games: 100, Wins: 55, Draws: 5, Losses: 40},
	}
	ps := &PairStats{
		Games:    10,
		AWin:     6,
		BWin:     3,
		Draw:     1,
		Plies:    300,
		Sides:    SideStats{Games: 10, AWin: 6, BWin: 3, Draw: 1},
		BySide:   newSideEventMap(),
		ByEngine: map[string]*PairEngineStats{"ml-r2-v1": {Games: 10, Wins: 6, Draws: 1, Losses: 3, Events: newEventStats()}},
		Events:   newEventStats(),
	}
	ps.Endgame = EndStats{Games: 10, Plies: 300, FirstKillA: 6, FirstKillB: 3, NoFirstKill: 1, FirstKillWins: 7, FirstKillDecided: 9}
	ps.Endgame.ensure()
	sf.AIVsAI.ByPair["ml-r2-v1|alphabeta-r2-v1"] = ps
	sf.RecentRuns = []RunEntry{{
		At: time.Now(), A: "ml-r2-v1", B: "alphabeta-r2-v1", Games: 10, AWin: 6, BWin: 3, Draw: 1,
		Sides: SideStats{Games: 10, AWin: 6, BWin: 3, Draw: 1}, ScoreA: 0.65, TookMs: 60000, StatsOnly: true,
		Config: sf.Config,
	}}

	// A sample game-changing move so the board section is exercised.
	before := game.New(nil)
	var move game.Action
	for _, a := range before.LegalActions() {
		if a.Kind == game.ActMove {
			move = a
			break
		}
	}
	after, err := before.Apply(move)
	if err != nil {
		t.Fatal(err)
	}
	sf.BestMoves = []BestMove{{
		Rank: 1, Game: 1, Pair: "ml-r2-v1|ml-r2-v1", A: "ml-r2-v1", B: "ml-r2-v1", Ply: 1,
		Mover: game.PlayerA, Engine: "ml-r2-v1", Winner: game.PlayerA,
		Action: move, Label: before.Describe(move), Before: before, After: after,
		EvalBefore: -10, EvalAfter: 6, Swing: 16, Uniqueness: 5, SecondBest: 11,
		Phase: "pawn", TokensA: 1, TokensB: 2,
	}}
	sf.BestMoveConfig = &BestMoveConfig{Engine: "ml-r2-v1", Depth: 4, Games: 2, Candidates: 100, Top: 1}

	data, err := RenderHTML("data/balance/test.json", sf)
	if err != nil {
		t.Fatal(err)
	}
	html := string(data)
	lo, hi := Wilson(55, 100)
	wantCI := fmt.Sprintf("%.1f%% [%.1f–%.1f]", 55.0, 100*lo, 100*hi)
	for _, want := range []string{
		"Arena Balance Report",
		"Player A",
		"Player B",
		"55.0%",
		wantCI,
		"ml-r2-v1|alphabeta-r2-v1",
		"alphabeta-r2-v1",
		"pc-wizard",
		"stats only",
		"First blood",
		"Game-changing move",
		`class="board-svg"`,
		"#1 game-changing move",
		`href="#pc-pawn"`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("report missing %q", want)
		}
	}
	if !strings.Contains(html, "&lt;ML v2&gt;") {
		t.Fatal("engine name was not HTML-escaped")
	}
	if strings.Contains(html, "<ML v2>") {
		t.Fatal("un-escaped engine name leaked into the report")
	}
}
