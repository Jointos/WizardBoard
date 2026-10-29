package arena

import (
	"fmt"
	"sort"

	"wizardboardgame/internal/ai"
	"wizardboardgame/internal/ai/ml_r2_v2"
	"wizardboardgame/internal/game"
)

// GameLog is the replayable action list of one arena game. It is only filled
// when GameConfig.LogGames is set (the stats generator), so live play and
// training never pay for it.
type GameLog struct {
	Game    int           `json:"game"`
	Pair    string        `json:"pair"`
	A       string        `json:"a"`
	B       string        `json:"b"`
	AIsA    bool          `json:"aIsA"`
	Winner  game.Player   `json:"winner"`
	Plies   int           `json:"plies"`
	Rules   game.Rules    `json:"rules"`
	Actions []game.Action `json:"actions"`
}

// BestMoveOptions configures the offline game-changing-move analysis.
type BestMoveOptions struct {
	// Engine is the reference search engine (default ml-r2-v2).
	Engine string
	// Depth is the stronger depth used for the final ranking.
	Depth int
	// Top is how many moves to keep.
	Top int
}

// BestMoveConfig records how the stored best moves were produced.
type BestMoveConfig struct {
	Engine     string `json:"engine"`
	Depth      int    `json:"depth"`
	Games      int    `json:"games"`
	Candidates int    `json:"candidates"`
	Top        int    `json:"top"`
}

// BestMove is one highlighted game-changing move, including the before/after
// states so the report can draw the board.
type BestMove struct {
	Rank       int         `json:"rank"`
	Game       int         `json:"game"`
	Pair       string      `json:"pair"`
	A          string      `json:"a"`
	B          string      `json:"b"`
	Ply        int         `json:"ply"`
	Mover      game.Player `json:"mover"`
	Engine     string      `json:"engine"`
	Winner     game.Player `json:"winner"`
	Action     game.Action `json:"action"`
	Label      string      `json:"label"`
	Before     game.State  `json:"before"`
	After      game.State  `json:"after"`
	EvalBefore int         `json:"evalBefore"`
	EvalAfter  int         `json:"evalAfter"`
	Swing      int         `json:"swing"`
	Uniqueness int         `json:"uniqueness"`
	SecondBest int         `json:"secondBest,omitempty"`
	Turnaround bool        `json:"turnaround,omitempty"`
	Phase      string      `json:"phase,omitempty"`
	TokensA    int         `json:"tokensA,omitempty"`
	TokensB    int         `json:"tokensB,omitempty"`
}

const bestMoveWinScore = 1_000_000

// FindBestMoves ranks the moves played by the eventual winner in decisive
// games by how much they improved the winner's evaluated position. It is
// deliberately offline: the arena only keeps the action lists.
func FindBestMoves(logs []GameLog, opt BestMoveOptions) ([]BestMove, BestMoveConfig, error) {
	if opt.Engine == "" {
		opt.Engine = "ml-r2-v2"
	}
	if opt.Depth <= 0 {
		opt.Depth = 4
	}
	if opt.Top <= 0 {
		opt.Top = 10
	}
	cfg := BestMoveConfig{
		Engine: opt.Engine,
		Depth:  opt.Depth,
	}

	type candidate struct {
		logIdx int
		ply    int // 1-based
		action game.Action
		mover  game.Player
		before int
		after  int
		swing  int
	}

	candidates := make([]candidate, 0, len(logs)*64)
	for li, log := range logs {
		if log.Winner == game.NoPlayer || len(log.Actions) == 0 {
			continue
		}
		cfg.Games++
		s := game.New(rulesFor(log))
		for i, a := range log.Actions {
			mover := s.Turn
			isWinner := mover == log.Winner
			before := 0
			if isWinner {
				before = ml_r2_v2.HandEval(&s)
				if s.Turn != log.Winner {
					before = -before
				}
			}
			ns, err := s.Apply(a)
			if err != nil {
				break
			}
			if isWinner && !ns.Over && a.Kind != game.ActSkip && a.Kind != game.ActPass {
				after := ml_r2_v2.HandEval(&ns)
				if ns.Turn != log.Winner {
					after = -after
				}
				candidates = append(candidates, candidate{
					logIdx: li,
					ply:    i + 1,
					action: a,
					mover:  mover,
					before: before,
					after:  after,
					swing:  after - before,
				})
			}
			s = ns
		}
	}

	if len(candidates) == 0 {
		cfg.Top = 0
		return nil, cfg, nil
	}
	cfg.Candidates = len(candidates)
	sort.SliceStable(candidates, func(i, j int) bool {
		return candidates[i].swing > candidates[j].swing
	})

	stage2 := opt.Top * 5
	if stage2 < 50 {
		stage2 = 50
	}
	if stage2 > len(candidates) {
		stage2 = len(candidates)
	}
	ref2, err := newRefEval(opt.Engine, opt.Depth)
	if err != nil {
		return nil, cfg, err
	}

	out := make([]BestMove, 0, opt.Top)
	for _, c := range candidates[:stage2] {
		log := logs[c.logIdx]
		beforeState, err := replayTo(log, c.ply-1)
		if err != nil {
			continue
		}
		afterState, err := beforeState.Apply(c.action)
		if err != nil || afterState.Over {
			continue
		}
		before := ref2.score(&beforeState, log.Winner)
		after := ref2.score(&afterState, log.Winner)
		uniq, second := ref2.uniqueness(&beforeState)

		engine := log.A
		if (c.mover == game.PlayerA) != log.AIsA {
			engine = log.B
		}
		out = append(out, BestMove{
			Game:       log.Game,
			Pair:       log.Pair,
			A:          log.A,
			B:          log.B,
			Ply:        c.ply,
			Mover:      c.mover,
			Engine:     engine,
			Winner:     log.Winner,
			Action:     c.action,
			Label:      beforeState.Describe(c.action),
			Before:     beforeState,
			After:      afterState,
			EvalBefore: before,
			EvalAfter:  after,
			Swing:      after - before,
			Uniqueness: uniq,
			SecondBest: second,
			Turnaround: before < 0 && after > 0,
			Phase:      game.UnitType(beforeState.PhaseIndex(beforeState.Turn)).String(),
			TokensA:    int(beforeState.Token[game.PlayerA]),
			TokensB:    int(beforeState.Token[game.PlayerB]),
		})
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Swing == out[j].Swing {
			return out[i].Uniqueness > out[j].Uniqueness
		}
		return out[i].Swing > out[j].Swing
	})
	if len(out) > opt.Top {
		out = out[:opt.Top]
	}
	for i := range out {
		out[i].Rank = i + 1
	}
	cfg.Top = len(out)
	return out, cfg, nil
}

func rulesFor(log GameLog) *game.Rules {
	if log.Rules == (game.Rules{}) {
		return nil
	}
	r := log.Rules
	return &r
}

func replayTo(log GameLog, n int) (game.State, error) {
	s := game.New(rulesFor(log))
	if n > len(log.Actions) {
		n = len(log.Actions)
	}
	for i := 0; i < n; i++ {
		ns, err := s.Apply(log.Actions[i])
		if err != nil {
			return game.State{}, err
		}
		s = ns
	}
	return s, nil
}

// refEval wraps a scored engine used to evaluate positions from a fixed side.
type refEval struct {
	eng ai.ScoredEngine
	an  ai.Analyzer
}

func newRefEval(id string, depth int) (*refEval, error) {
	e, ok := ai.New(id, ai.Config{MaxDepth: depth, DisableFilter: true, Seed: 1})
	if !ok {
		return nil, fmt.Errorf("arena: unknown reference engine %q", id)
	}
	se, ok := e.(ai.ScoredEngine)
	if !ok {
		return nil, fmt.Errorf("arena: reference engine %q does not expose a move score", id)
	}
	an, _ := e.(ai.Analyzer)
	return &refEval{eng: se, an: an}, nil
}

// score returns the position score from side's perspective.
func (r *refEval) score(s *game.State, side game.Player) int {
	if s.Over {
		switch s.Winner {
		case side:
			return bestMoveWinScore
		case game.NoPlayer:
			return 0
		default:
			return -bestMoveWinScore
		}
	}
	legal := s.LegalActions()
	if len(legal) == 0 {
		return 0
	}
	var sc int
	if len(legal) == 1 && r.an != nil {
		res := r.an.Analyze(*s, legal)
		sc = res.Engine.Score
	} else {
		_, sc = r.eng.SelectScore(*s, legal)
	}
	if s.Turn == side {
		return sc
	}
	return -sc
}

// uniqueness returns how much better the best move is than the second best,
// plus the second-best score. It is a "there was only one good move" signal.
func (r *refEval) uniqueness(s *game.State) (gap, second int) {
	if r.an == nil {
		return 0, 0
	}
	legal := s.LegalActions()
	if len(legal) < 2 {
		return 0, 0
	}
	res := r.an.Analyze(*s, legal)
	if res.ListDepth == 0 || len(res.Moves) == 0 {
		return 0, 0
	}
	second = res.Moves[0].Score
	gap = res.Engine.Score - second
	if gap < 0 {
		gap = 0
	}
	return gap, second
}
