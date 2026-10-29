// Package arena runs batches of games between registered engines and turns the
// outcomes into an Elo ranking. It is the standard way to compare AI versions.
package arena

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"wizardboardgame/internal/ai"
	_ "wizardboardgame/internal/ai/catalog"
	"wizardboardgame/internal/game"
)

// GameConfig configures one arena game. Use the same budget as live play
// (MaxTime + MaxDepth); node budgets are for debugging only.
type GameConfig struct {
	MaxTime       time.Duration
	MaxNodes      int
	MaxDepth      int
	DisableFilter bool
	MaxPlies      int
	Workers       int
	Seed          int64
	// Paired plays games 2k and 2k+1 with the same seed and swapped sides.
	// That halves the noise of the side (Player A vs Player B) comparison.
	Paired bool
	// LogGames keeps the action list of every game in PairResult.Logs. It is
	// only enabled by the stats generator; live play and training never set it.
	LogGames bool
}

// PairResult is the outcome of a match between two engines.
type PairResult struct {
	Games int
	AWin  int // wins for the first engine (aID)
	BWin  int // wins for the second engine (bID)
	Draw  int
	Error int
	Plies int

	// ScoreA is the first engine's score in [0,1] (draws count 0.5).
	ScoreA float64

	// Sides is the balance tally: who won by game side, independent of engines.
	Sides SideStats
	// Endgame holds tempo and end-state aggregates.
	Endgame EndStats

	// Stats holds the in-game event tally (buffs, nerfs, cleanses, kills).
	Stats *PairEventStats

	// Logs holds the replayable action list of every game when GameConfig
	// .LogGames is set. It is used by the best-move analysis.
	Logs []GameLog
}

type outcome struct {
	winner  game.Player // NoPlayer means draw
	aIsA    bool
	plies   int
	err     error
	stats   *PairEventStats
	end     EndStats
	actions []game.Action
}

// RunPair plays `games` games between aID and bID, alternating sides so that
// first-move advantage does not favour either engine.
func RunPair(aID, bID string, games int, cfg GameConfig) (PairResult, error) {
	if games <= 0 {
		return PairResult{}, fmt.Errorf("arena: games must be positive")
	}
	if _, _, ok := ai.Get(aID); !ok {
		return PairResult{}, fmt.Errorf("arena: unknown engine %q", aID)
	}
	if _, _, ok := ai.Get(bID); !ok {
		return PairResult{}, fmt.Errorf("arena: unknown engine %q", bID)
	}
	if cfg.Workers <= 0 {
		cfg.Workers = 1
	}
	if cfg.MaxPlies <= 0 {
		cfg.MaxPlies = 5000
	}

	results := make([]outcome, games)
	jobs := make(chan int)
	var wg sync.WaitGroup
	var done atomic.Int64

	for w := 0; w < cfg.Workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				results[i] = playGame(aID, bID, i, cfg)
				done.Add(1)
			}
		}()
	}
	for i := 0; i < games; i++ {
		jobs <- i
	}
	close(jobs)
	wg.Wait()

	var res PairResult
	res.Games = games
	res.Stats = newPairEventStats()
	res.Endgame.ensure()
	for _, o := range results {
		res.Plies += o.plies
		if o.stats != nil {
			res.Stats.Add(o.stats)
		}
		if o.err != nil {
			res.Error++
			continue
		}
		res.Endgame.Add(&o.end)
		switch {
		case o.winner == game.NoPlayer:
			res.Draw++
		case (o.winner == game.PlayerA) == o.aIsA:
			res.AWin++
		default:
			res.BWin++
		}
		// Side balance is game-side only, so it never depends on the pair
		// orientation (and must not be flipped when the pair key is sorted).
		switch o.winner {
		case game.PlayerA:
			res.Sides.AWin++
		case game.PlayerB:
			res.Sides.BWin++
		default:
			res.Sides.Draw++
		}
	}
	res.Sides.Games = res.Games - res.Error
	scored := res.Games - res.Error
	if scored > 0 {
		res.ScoreA = (float64(res.AWin) + 0.5*float64(res.Draw)) / float64(scored)
	}
	if cfg.LogGames {
		res.Logs = make([]GameLog, 0, games)
		for i, o := range results {
			if o.err != nil {
				continue
			}
			res.Logs = append(res.Logs, GameLog{
				Game:    i + 1,
				Pair:    aID + "|" + bID,
				A:       aID,
				B:       bID,
				AIsA:    o.aIsA,
				Winner:  o.winner,
				Plies:   o.plies,
				Rules:   *game.DefaultRules(),
				Actions: o.actions,
			})
		}
	}
	return res, nil
}

func playGame(aID, bID string, index int, cfg GameConfig) outcome {
	// Alternate sides: even games aID plays A, odd games aID plays B.
	aIsA := index%2 == 0
	seed := cfg.Seed + int64(index)*7919
	if cfg.Paired {
		// Games 2k and 2k+1 share a seed and swap sides.
		seed = cfg.Seed + int64(index/2)*7919
	}

	cfgA := ai.Config{MaxTime: cfg.MaxTime, MaxNodes: cfg.MaxNodes, MaxDepth: cfg.MaxDepth, DisableFilter: cfg.DisableFilter, Seed: seed}
	cfgB := ai.Config{MaxTime: cfg.MaxTime, MaxNodes: cfg.MaxNodes, MaxDepth: cfg.MaxDepth, DisableFilter: cfg.DisableFilter, Seed: seed + 104729}
	ea, _ := ai.New(aID, cfgA)
	eb, _ := ai.New(bID, cfgB)

	white, black := ea, eb // engine playing PlayerA / PlayerB
	if !aIsA {
		white, black = eb, ea
	}

	st := newPairEventStats()
	emptyA := newEventStats()
	st.ByEngine[aID] = &emptyA
	if bID != aID {
		// Self-play would otherwise overwrite the first engine's tally.
		emptyB := newEventStats()
		st.ByEngine[bID] = &emptyB
	}

	firstKill := game.NoPlayer
	firstKillPly := 0
	firstWizardKill := game.NoPlayer
	firstWizardPly := 0

	s := game.New(nil)
	plies := 0
	var actions []game.Action
	if cfg.LogGames {
		actions = make([]game.Action, 0, 128)
	}
	for !s.Over && plies < cfg.MaxPlies {
		var eng ai.Engine = white
		if s.Turn == game.PlayerB {
			eng = black
		}
		legal := s.LegalActions()
		if len(legal) == 0 {
			return outcome{winner: game.NoPlayer, aIsA: aIsA, plies: plies,
				err: fmt.Errorf("no legal actions at ply %d", plies), stats: st, actions: actions}
		}
		action := eng.Select(s, legal)
		if cfg.LogGames {
			actions = append(actions, action)
		}
		mover := aID
		if (s.Turn == game.PlayerA) != aIsA {
			mover = bID
		}
		side := s.Turn
		_, victim, killed := killInfo(&s, action)
		collectEvent(&s, action, st.ByEngine[mover])
		collectEvent(&s, action, &st.Overall)
		collectEvent(&s, action, st.BySide[side.String()])
		if killed {
			if firstKill == game.NoPlayer {
				firstKill = side
				firstKillPly = plies
			}
			if victim == game.UnitWizard && firstWizardKill == game.NoPlayer {
				firstWizardKill = side
				firstWizardPly = plies
			}
		}
		next, err := s.Apply(action)
		if err != nil {
			return outcome{aIsA: aIsA, plies: plies, err: err, stats: st, actions: actions}
		}
		s = next
		plies++
	}
	// A game counts as played even when the safety cap turns it into a draw.
	st.Overall.Games = 1
	for _, es := range st.ByEngine {
		es.Games = 1
	}
	for _, es := range st.BySide {
		es.Games = 1
	}

	end := endStatsOf(&s, plies, firstKill, firstKillPly, firstWizardKill, firstWizardPly)
	if !s.Over {
		// Hit the safety cap: count as a draw.
		return outcome{winner: game.NoPlayer, aIsA: aIsA, plies: plies, stats: st, end: end, actions: actions}
	}
	return outcome{winner: s.Winner, aIsA: aIsA, plies: plies, stats: st, end: end, actions: actions}
}

// endStatsOf snapshots the final position and the first-blood timing.
func endStatsOf(s *game.State, plies int, firstKill game.Player, firstKillPly int, firstWizard game.Player, firstWizardPly int) EndStats {
	end := EndStats{Games: 1, Plies: plies}
	end.ensure()
	for i := range s.Pieces {
		p := &s.Pieces[i]
		if !p.Alive {
			continue
		}
		if p.Owner == game.PlayerA {
			end.ASurvivors[p.Type.String()]++
		} else {
			end.BSurvivors[p.Type.String()]++
		}
	}
	end.ATokens = int(s.Token[game.PlayerA])
	end.BTokens = int(s.Token[game.PlayerB])
	if s.Over && s.Winner == game.NoPlayer {
		end.Stalemates = 1
	}
	if !s.Over {
		end.SafetyCaps = 1
	}

	b := plyBucket(plies)
	end.PlyBuckets[b] = 1
	switch s.Winner {
	case game.PlayerA:
		end.AWinByBucket[b] = 1
	case game.PlayerB:
		end.BWinByBucket[b] = 1
	default:
		end.DrawByBucket[b] = 1
	}

	if firstKill == game.NoPlayer {
		end.NoFirstKill = 1
	} else {
		if firstKill == game.PlayerA {
			end.FirstKillA = 1
		} else {
			end.FirstKillB = 1
		}
		end.FirstKillPlies = firstKillPly
		if s.Over && s.Winner != game.NoPlayer {
			end.FirstKillDecided = 1
			if s.Winner == firstKill {
				end.FirstKillWins = 1
			}
		}
	}
	if firstWizard == game.NoPlayer {
		end.NoFirstWizardKill = 1
	} else {
		if firstWizard == game.PlayerA {
			end.FirstWizardKillA = 1
		} else {
			end.FirstWizardKillB = 1
		}
		end.FirstWizardPlies = firstWizardPly
		if s.Over && s.Winner != game.NoPlayer {
			end.FirstWizardDecided = 1
			if s.Winner == firstWizard {
				end.FirstWizardWins = 1
			}
		}
	}
	return end
}

// plyBucket maps a game length to its histogram bucket.
func plyBucket(plies int) int {
	switch {
	case plies < 50:
		return 0
	case plies < 100:
		return 1
	case plies < 150:
		return 2
	case plies < 200:
		return 3
	case plies < 250:
		return 4
	default:
		return 5
	}
}
