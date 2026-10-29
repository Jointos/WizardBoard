package ml_r2_v4

import (
	"sort"
	"time"

	"wizardboardgame/internal/ai"
	"wizardboardgame/internal/game"
)

// analysisBand is the score window behind the best move that the analyser
// scores exactly. Moves further behind get an upper bound, which is enough to
// know they are clearly worse.
const analysisBand = 15

// topDepthMax caps the engine-move search. "strong" live play reaches depth 6
// in about 1.5s.
const topDepthMax = 6

// analysisMoves is how many alternatives are scored (in addition to the
// engine move itself).
const analysisMoves = 10

// Budget split, in percent of the whole analysis budget. Phase 1 must never
// consume everything: a complex position can spend the entire budget on the
// top move alone, which used to leave no time to score any alternative and the
// assist panel came back empty.
const (
	topBudgetShare = 50 // engine move (iterative deepening)
	selBudgetShare = 65 // + the cheap ranking pass
)

// Analyze ranks the candidate moves. All suggestions (the engine move and the
// scored alternatives) are evaluated at the same depth.
//
// It runs in three phases:
//
//  1. The engine's own move is found with a dedicated share of the budget
//     (iterative deepening, the same search `Select` uses).
//  2. A cheap pass ranks all candidates a couple of plies shallower, just to
//     pick which alternatives are worth scoring at full depth.
//  3. The picked alternatives are scored exactly at the engine depth.
//
// Each phase gets its own slice of the time/node budget, so a hard position
// still yields a list of alternatives. If a phase runs out of budget, the
// scores computed so far are returned instead of an empty result.
func (e *engine) Analyze(s game.State, legal []game.Action) ai.Analysis {
	cands := legal
	if !e.cfg.DisableFilter {
		cands = filterCandidates(&s, legal)
	}
	if len(cands) == 0 {
		cands = legal
	}
	if len(cands) == 0 {
		return ai.Analysis{}
	}

	se := &search{
		tt: make(map[uint64]entry, 1<<14),
	}
	for i := range se.bufs {
		se.bufs[i] = make([]game.Action, 0, 384)
		se.ord[i] = make([]game.Action, 0, 384)
	}

	start := time.Now()
	totalNodes := e.cfg.MaxNodes
	// Phase 1 may use at most topBudgetShare of the budget.
	if totalNodes > 0 {
		se.maxNodes = totalNodes * topBudgetShare / 100
	}
	if e.cfg.MaxTime > 0 {
		se.deadline = start.Add(e.cfg.MaxTime * topBudgetShare / 100)
	}

	maxDepth := e.cfg.MaxDepth
	if maxDepth <= 0 {
		maxDepth = 8
	}

	order := make([]int, len(cands))
	for i := range order {
		order[i] = i
	}

	// Phase 1: find the engine's move within its share of the budget.
	bestIdxAt := make([]int, maxDepth+2)
	bestScoreAt := make([]int, maxDepth+2)
	topDepth := 0
	for d := 1; d <= maxDepth && d <= topDepthMax; d++ {
		se.stopped = false
		idx, best := se.searchBest(&s, d, cands, order)
		if se.stopped || idx < 0 {
			break
		}
		bestIdxAt[d], bestScoreAt[d] = idx, best
		topDepth = d
		for i := range order {
			if order[i] == idx {
				order[0], order[i] = order[i], order[0]
				break
			}
		}
	}
	if topDepth == 0 {
		return ai.Analysis{Depth: 0, ListDepth: 0, Engine: ai.ScoredAction{Action: cands[0]}}
	}
	engineIdx := bestIdxAt[topDepth]
	engineScore := bestScoreAt[topDepth]

	engineOnly := ai.Analysis{
		Depth:  topDepth,
		Engine: ai.ScoredAction{Action: cands[engineIdx], Score: engineScore},
	}

	// From here on the phases share the rest of the budget.
	if totalNodes > 0 {
		se.maxNodes = totalNodes
	}
	if e.cfg.MaxTime > 0 {
		se.deadline = start.Add(e.cfg.MaxTime * selBudgetShare / 100)
	}

	// Phase 2: cheap ranking at a shallower depth to pick the alternatives.
	selDepth := topDepth - 2
	if selDepth < 1 {
		selDepth = 1
	}
	se.stopped = false
	rankScores, _, _ := se.scoreAt(&s, selDepth, cands, order, bestScoreAt[selDepth], bestIdxAt[selDepth])
	sort.SliceStable(order, func(a, b int) bool { return rankScores[order[a]] > rankScores[order[b]] })

	picked := make([]int, 0, analysisMoves+1)
	picked = append(picked, engineIdx)
	for _, i := range order {
		// rankScores[i] == -inf means the ranking pass did not reach this move.
		if i == engineIdx || rankScores[i] == -inf {
			continue
		}
		picked = append(picked, i)
		if len(picked) > analysisMoves {
			break
		}
	}
	if len(picked) == 1 {
		return engineOnly
	}

	// Phase 3: exact scores at the engine depth. The deadline covers the whole
	// budget; whatever was scored before it ran out is returned.
	if e.cfg.MaxTime > 0 {
		se.deadline = start.Add(e.cfg.MaxTime)
	}
	se.stopped = false
	scores, bounds, _ := se.scoreAt(&s, topDepth, cands, picked, engineScore, engineIdx)

	moves := make([]ai.ScoredAction, 0, len(picked)-1)
	for _, i := range picked[1:] {
		// -inf means the budget ran out before this move was searched.
		if scores[i] == -inf {
			continue
		}
		moves = append(moves, ai.ScoredAction{Action: cands[i], Score: scores[i], Bound: bounds[i]})
	}
	if len(moves) == 0 {
		return engineOnly
	}
	sort.SliceStable(moves, func(i, j int) bool { return moves[i].Score > moves[j].Score })

	return ai.Analysis{
		Depth:     topDepth,
		ListDepth: topDepth,
		Engine:    ai.ScoredAction{Action: cands[engineIdx], Score: engineScore},
		Moves:     moves,
	}
}

// scoreAt searches cands at depth d and returns their scores from the root
// side's perspective. idxs selects which candidates to search; the best move
// is filled in without searching it again.
//
// When the budget runs out the partially filled slices are returned together
// with ok=false; entries that were not searched keep the -inf sentinel.
func (se *search) scoreAt(s *game.State, d int, cands []game.Action, idxs []int, best, bestIdx int) ([]int, []bool, bool) {
	scores := make([]int, len(cands))
	bounds := make([]bool, len(cands))
	for i := range scores {
		scores[i] = -inf
	}
	scores[bestIdx] = best

	lo := best - analysisBand - 1
	for _, i := range idxs {
		if i == bestIdx {
			continue
		}
		v := se.searchMove(s, d, cands[i], lo, best)
		if se.stopped {
			return scores, bounds, false
		}
		if v > best {
			// A tie for the best move; clamp to the known maximum.
			v = best
		}
		scores[i] = v
		bounds[i] = v <= lo
	}
	return scores, bounds, true
}

// searchBest scans the root moves with a shared alpha (beta is infinite) and
// returns the index of the best move together with its exact score.
func (se *search) searchBest(s *game.State, depth int, cands []game.Action, order []int) (int, int) {
	bestIdx := -1
	best := -inf
	alpha := -inf
	for _, i := range order {
		v := se.searchMove(s, depth, cands[i], alpha, inf)
		if se.stopped {
			return bestIdx, best
		}
		if v > best {
			best = v
			bestIdx = i
		}
		if v > alpha {
			alpha = v
		}
	}
	return bestIdx, best
}

// searchMove applies a root move and searches the resulting position from the
// root side's perspective.
func (se *search) searchMove(s *game.State, depth int, a game.Action, alpha, beta int) int {
	ns, err := s.Apply(a)
	if err != nil {
		return -inf
	}
	if ns.Turn == s.Turn {
		// Pending free move: the same player continues, so no negation.
		return se.negamax(&ns, depth-1, 1, alpha, beta)
	}
	return -se.negamax(&ns, depth-1, 1, -beta, -alpha)
}
