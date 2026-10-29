package ml_r2_v4

import (
	"time"

	"wizardboardgame/internal/game"
)

type ttFlag uint8

const (
	flagExact ttFlag = iota
	flagLower
	flagUpper
)

type entry struct {
	depth int
	score int
	flag  ttFlag
	best  game.Action
}

type search struct {
	deadline time.Time
	maxNodes int
	nodes    int
	stopped  bool
	// trim enables the candidate filter inside the tree as well, not just at
	// the root. The analysis uses it to reach a greater depth; live play keeps
	// the full move set.
	trim bool
	tt   map[uint64]entry
	bufs [maxPly][]game.Action
	ord  [maxPly][]game.Action
}

func (se *search) abort() bool {
	if se.stopped {
		return true
	}
	se.nodes++
	if se.maxNodes > 0 && se.nodes >= se.maxNodes {
		se.stopped = true
		return true
	}
	if !se.deadline.IsZero() && se.nodes&255 == 0 && time.Now().After(se.deadline) {
		se.stopped = true
		return true
	}
	return false
}

// searchRoot tries every root candidate and returns the best action and score.
// Scores are from the root side's perspective.
func (se *search) searchRoot(s *game.State, depth int, cands []game.Action) (game.Action, int) {
	best := cands[0]
	bestScore := -inf
	alpha, beta := -inf, inf
	for _, a := range cands {
		ns, err := s.Apply(a)
		if err != nil {
			continue
		}
		var v int
		if ns.Turn == s.Turn {
			// Pending free move: the same player continues, so no negation.
			v = se.negamax(&ns, depth-1, 1, alpha, beta)
		} else {
			v = -se.negamax(&ns, depth-1, 1, -beta, -alpha)
		}
		if se.stopped {
			return best, bestScore
		}
		if v > bestScore {
			bestScore = v
			best = a
		}
		if v > alpha {
			alpha = v
		}
	}
	return best, bestScore
}

func (se *search) negamax(s *game.State, depth, ply int, alpha, beta int) int {
	if se.abort() {
		return 0
	}
	if s.Over {
		switch s.Score() {
		case game.ResultWin:
			return winScore - ply
		case game.ResultLoss:
			return -(winScore - ply)
		default:
			return 0
		}
	}
	if depth <= 0 || ply >= maxPly-1 {
		return se.quiesce(s, alpha, beta, ply, 0)
	}

	key := s.Hash()
	if e, ok := se.tt[key]; ok && e.depth >= depth {
		switch e.flag {
		case flagExact:
			return e.score
		case flagLower:
			if e.score > alpha {
				alpha = e.score
			}
		case flagUpper:
			if e.score < beta {
				beta = e.score
			}
		}
		if alpha >= beta {
			return e.score
		}
	}

	acts := s.Actions(se.bufs[ply][:0])
	se.bufs[ply] = acts
	if se.trim {
		acts = trimCandidates(s, acts)
	}
	se.orderActions(s, ply, acts, key)

	best := -inf
	alpha0 := alpha
	var bestAction game.Action
	for _, a := range acts {
		ns, err := s.Apply(a)
		if err != nil {
			continue
		}
		var v int
		if ns.Turn == s.Turn {
			v = se.negamax(&ns, depth-1, ply+1, alpha, beta)
		} else {
			v = -se.negamax(&ns, depth-1, ply+1, -beta, -alpha)
		}
		if se.stopped {
			return 0
		}
		if v > best {
			best = v
			bestAction = a
		}
		if v > alpha {
			alpha = v
		}
		if alpha >= beta {
			break
		}
	}
	if best == -inf {
		return evaluate(s)
	}

	flag := flagExact
	switch {
	case best <= alpha0:
		flag = flagUpper
	case best >= beta:
		flag = flagLower
	}
	if len(se.tt) < 1<<20 {
		se.tt[key] = entry{depth: depth, score: best, flag: flag, best: bestAction}
	}
	return best
}

// maxQuiesce caps the extra tactical depth below the main search horizon.
const maxQuiesce = 4

// quiesce extends the search past the main horizon with captures and shots
// only. It uses stand-pat: the side to move may always keep the static
// evaluation, so a bad capture is never forced. Without this, the engine
// happily walks into a recapture just beyond its nominal depth.
func (se *search) quiesce(s *game.State, alpha, beta, ply, qply int) int {
	if se.abort() {
		return 0
	}
	if s.Over {
		switch s.Score() {
		case game.ResultWin:
			return winScore - ply
		case game.ResultLoss:
			return -(winScore - ply)
		default:
			return 0
		}
	}

	stand := evaluate(s)
	if qply >= maxQuiesce || ply >= maxPly-1 {
		return stand
	}
	if stand >= beta {
		return stand
	}
	if stand > alpha {
		alpha = stand
	}

	acts := s.Actions(se.bufs[ply][:0])
	se.bufs[ply] = acts
	n := 0
	for _, a := range acts {
		if a.Kind == game.ActCapture || a.Kind == game.ActShoot {
			acts[n] = a
			n++
		}
	}
	if n == 0 {
		return stand
	}
	acts = acts[:n]
	// Wizard victims first: they decide games, so they are the most
	// important continuation to refute or confirm.
	if n > 1 {
		w := 0
		for i := 0; i < n; i++ {
			if wizardVictim(s, acts[i]) {
				acts[w], acts[i] = acts[i], acts[w]
				w++
			}
		}
	}

	best := stand
	for _, a := range acts {
		ns, err := s.Apply(a)
		if err != nil {
			continue
		}
		v := -se.quiesce(&ns, -beta, -alpha, ply+1, qply+1)
		if se.stopped {
			return 0
		}
		if v > best {
			best = v
		}
		if v > alpha {
			alpha = v
		}
		if alpha >= beta {
			break
		}
	}
	return best
}

func wizardVictim(s *game.State, a game.Action) bool {
	return a.Target < game.MaxPieces && s.Pieces[a.Target].Alive && s.Pieces[a.Target].Type == game.UnitWizard
}
