package ml_r2_v4

import (
	"wizardboardgame/internal/game"
)

// trimCandidates applies the root candidate filter in place inside the search
// tree. It keeps every action except cannon move+turn combinations that do not
// aim anywhere useful, which is the main source of the branching explosion.
func trimCandidates(s *game.State, acts []game.Action) []game.Action {
	n := 0
	for _, a := range acts {
		if a.Kind == game.ActMoveTurn && !usefulMoveTurn(s, a) {
			continue
		}
		acts[n] = a
		n++
	}
	if n == 0 {
		return acts
	}
	return acts[:n]
}

// filterCandidates trims the AI's move list without changing the rules. Cannon
// move+turn actions explode the branching factor (8 destinations x 8 facings),
// so only the ones that aim somewhere useful are kept.
func filterCandidates(s *game.State, legal []game.Action) []game.Action {
	out := make([]game.Action, 0, len(legal))
	for _, a := range legal {
		if a.Kind == game.ActMoveTurn && !usefulMoveTurn(s, a) {
			continue
		}
		out = append(out, a)
	}
	if len(out) == 0 {
		return legal
	}
	return out
}

// usefulMoveTurn keeps a move+turn if the new facing can shoot an enemy, or if
// it points toward the enemy side of the board.
func usefulMoveTurn(s *game.State, a game.Action) bool {
	u := &s.Pieces[a.Unit]
	if !u.Alive || u.Type != game.UnitCannon {
		return true
	}
	ranges := [2]int{s.Rules.CannonRange, 0}
	if s.IsBuffed(a.Unit) && s.Rules.CannonBuffRange > 0 && s.Rules.CannonBuffRange != s.Rules.CannonRange {
		ranges[1] = s.Rules.CannonBuffRange
	}
	for _, r := range ranges {
		if r <= 0 {
			continue
		}
		sq, ok := squareAhead(a.To, a.Facing, r)
		if !ok {
			continue
		}
		occ := s.Board[sq]
		if occ != game.NoPiece && s.Pieces[occ].Owner != u.Owner {
			return true
		}
	}
	_, dy := game.DirDelta(a.Facing)
	sign := 1
	if u.Owner == game.PlayerB {
		sign = -1
	}
	return dy*sign > 0
}

// actionBucket groups actions for ordering. Higher is tried first.
func actionBucket(s *game.State, a game.Action) int {
	switch a.Kind {
	case game.ActCapture, game.ActShoot:
		if a.Target < game.MaxPieces && s.Pieces[a.Target].Alive {
			if s.Pieces[a.Target].Type == game.UnitWizard {
				return 4
			}
			return 3
		}
		return 3
	case game.ActCast, game.ActTeleport, game.ActSwap:
		return 2
	case game.ActMove, game.ActMoveTurn, game.ActTurn:
		return 1
	case game.ActSkip:
		return 0
	}
	return 1
}

// orderActions sorts acts into se.ord[ply] by tactical value, then moves the
// transposition-table hint to the front. It uses a counting sort: two passes
// over the actions instead of one pass per bucket.
func (se *search) orderActions(s *game.State, ply int, acts []game.Action, key uint64) {
	if len(acts) > 1 {
		var counts [5]int
		for _, a := range acts {
			counts[actionBucket(s, a)]++
		}
		var pos [5]int
		off := 0
		for bucket := 4; bucket >= 0; bucket-- {
			pos[bucket] = off
			off += counts[bucket]
		}
		scratch := se.ord[ply]
		if cap(scratch) < off {
			scratch = make([]game.Action, off)
		} else {
			scratch = scratch[:off]
		}
		for _, a := range acts {
			b := actionBucket(s, a)
			scratch[pos[b]] = a
			pos[b]++
		}
		copy(acts, scratch)
		se.ord[ply] = scratch
	}

	if e, ok := se.tt[key]; ok {
		for i := range acts {
			if acts[i] == e.best {
				acts[0], acts[i] = acts[i], acts[0]
				break
			}
		}
	}
}
