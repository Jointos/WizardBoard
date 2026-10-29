package alphabeta_r2_v1

import "wizardboardgame/internal/game"

// Material values. Wizards are evaluated separately because losing the first
// one is much less dangerous than losing the last one.
func unitValue(t game.UnitType) int {
	switch t {
	case game.UnitWizard:
		return 100
	case game.UnitCannon:
		return 12
	case game.UnitPawn:
		return 4
	}
	return 0
}

// wizardValue makes the first wizard cheap and the last one precious:
//
//	2 alive -> 55 each (110 total)
//	1 alive -> 95      (95 total)
//
// Losing the first wizard costs only 15 points; the last wizard is still worth
// nearly the full 100. The terminal score still ends the game when both die.
func wizardValue(count int) int {
	switch {
	case count >= 2:
		return 55
	case count == 1:
		return 95
	}
	return 0
}

// evaluate scores the position from the perspective of the side to move.
func evaluate(s *game.State) int {
	if s.Over {
		switch s.Score() {
		case game.ResultWin:
			return winScore
		case game.ResultLoss:
			return -winScore
		default:
			return 0
		}
	}

	// Resolve effects once; IsBuffed/IsNerfed would rescan all pieces per query.
	var buffed, nerfed [game.MaxPieces]bool
	wizA, wizB := 0, 0
	for i := 0; i < game.MaxPieces; i++ {
		p := &s.Pieces[i]
		if !p.Alive {
			continue
		}
		if p.Type == game.UnitWizard {
			if p.Owner == game.PlayerA {
				wizA++
			} else {
				wizB++
			}
		}
		if t := s.Buffs[i]; t != game.NoPiece {
			buffed[t] = true
		}
		if t := s.Nerfs[i]; t != game.NoPiece {
			nerfed[t] = true
		}
	}

	score := 0
	for id := 0; id < game.MaxPieces; id++ {
		p := &s.Pieces[id]
		if !p.Alive {
			continue
		}
		var v int
		if p.Type == game.UnitWizard {
			if p.Owner == game.PlayerA {
				v = wizardValue(wizA)
			} else {
				v = wizardValue(wizB)
			}
		} else {
			v = unitValue(p.Type)
		}
		if buffed[id] {
			v += 3
		}
		if nerfed[id] {
			v -= 3
		}
		switch p.Type {
		case game.UnitPawn:
			// Reward advancing toward the enemy back rank (where wizards live).
			if p.Owner == game.PlayerA {
				v += game.RankOf(p.Sq) / 2
			} else {
				v += (7 - game.RankOf(p.Sq)) / 2
			}
		case game.UnitCannon:
			if cannonAimed(s, game.PieceID(id), buffed) {
				v += 7
			}
		}
		if p.Owner == s.Turn {
			score += v
		} else {
			score -= v
		}
	}

	// Wizard danger: being under threat is bad for its owner.
	score -= wizardDanger(s, s.Turn, buffed)
	score += wizardDanger(s, s.Turn.Opponent(), buffed)

	score += int(s.Token[s.Turn])*2 - int(s.Token[s.Turn.Opponent()])*2
	return score
}

// cannonAimed reports whether the cannon has an enemy at shooting range along
// its facing. Pieces in between do not matter (cannons shoot over them).
func cannonAimed(s *game.State, id game.PieceID, buffed [game.MaxPieces]bool) bool {
	u := &s.Pieces[id]
	if !u.Alive || u.Type != game.UnitCannon {
		return false
	}
	for _, r := range shotRanges(s, id, buffed) {
		if r <= 0 {
			continue
		}
		sq, ok := squareAhead(u.Sq, u.Facing, r)
		if !ok {
			continue
		}
		occ := s.Board[sq]
		if occ != game.NoPiece && s.Pieces[occ].Owner != u.Owner {
			return true
		}
	}
	return false
}

func shotRanges(s *game.State, id game.PieceID, buffed [game.MaxPieces]bool) [2]int {
	ranges := [2]int{s.Rules.CannonRange, 0}
	if buffed[id] && s.Rules.CannonBuffRange > 0 && s.Rules.CannonBuffRange != s.Rules.CannonRange {
		ranges[1] = s.Rules.CannonBuffRange
	}
	return ranges
}

// wizardDanger counts how many enemy pieces currently threaten each of the
// owner's wizards (cannon line or an adjacent pawn capture).
func wizardDanger(s *game.State, owner game.Player, buffed [game.MaxPieces]bool) int {
	threat := 0
	for wid := 0; wid < game.MaxPieces; wid++ {
		w := &s.Pieces[wid]
		if !w.Alive || w.Owner != owner || w.Type != game.UnitWizard {
			continue
		}
		for eid := 0; eid < game.MaxPieces; eid++ {
			e := &s.Pieces[eid]
			if !e.Alive || e.Owner == owner {
				continue
			}
			switch e.Type {
			case game.UnitCannon:
				if cannonCovers(s, game.PieceID(eid), w.Sq, buffed) {
					threat += 9
				}
			case game.UnitPawn:
				if pawnThreatens(s, game.PieceID(eid), w.Sq) {
					threat += 5
				}
			case game.UnitWizard:
				if stepTowardEmpty(s, e.Sq, w.Sq) {
					threat += 2
				}
			}
		}
	}
	return threat
}

// cannonCovers reports whether the cannon's facing points at sq within range.
func cannonCovers(s *game.State, id game.PieceID, sq game.Square, buffed [game.MaxPieces]bool) bool {
	u := &s.Pieces[id]
	if !u.Alive || u.Type != game.UnitCannon {
		return false
	}
	dx := game.FileOf(sq) - game.FileOf(u.Sq)
	dy := game.RankOf(sq) - game.RankOf(u.Sq)
	ddx, ddy := game.DirDelta(u.Facing)

	var dist int
	switch {
	case ddx == 0:
		if dx != 0 || dy*ddy <= 0 {
			return false
		}
		dist = abs(dy)
	case ddy == 0:
		if dy != 0 || dx*ddx <= 0 {
			return false
		}
		dist = abs(dx)
	default:
		if abs(dx) != abs(dy) || dx*ddx <= 0 || dy*ddy <= 0 {
			return false
		}
		dist = abs(dx)
	}
	if dist == s.Rules.CannonRange {
		return true
	}
	return buffed[id] && s.Rules.CannonBuffRange > 0 && dist == s.Rules.CannonBuffRange
}

// pawnThreatens reports whether the pawn could capture the wizard on its next
// action (forward only every third action, backward always, sideways if buffed).
func pawnThreatens(s *game.State, id game.PieceID, sq game.Square) bool {
	p := &s.Pieces[id]
	if !p.Alive || p.Type != game.UnitPawn {
		return false
	}
	dx := game.FileOf(sq) - game.FileOf(p.Sq)
	dy := game.RankOf(sq) - game.RankOf(p.Sq)
	if abs(dx) > 1 || abs(dy) > 1 || (dx == 0 && dy == 0) {
		return false
	}
	sign := 1
	if p.Owner == game.PlayerB {
		sign = -1
	}
	fwd := dy * sign
	switch {
	case fwd < 0:
		return true
	case fwd > 0:
		return s.Rules.PawnForwardCaptureEvery > 0 &&
			(p.Acts+1)%s.Rules.PawnForwardCaptureEvery == 0
	default:
		return s.Rules.PawnBuffSideCapture && s.IsBuffed(id)
	}
}

// stepTowardEmpty reports whether the wizard could step one square toward the
// target (the precondition for casting).
func stepTowardEmpty(s *game.State, from, to game.Square) bool {
	dx := game.FileOf(to) - game.FileOf(from)
	dy := game.RankOf(to) - game.RankOf(from)
	if dx == 0 && dy == 0 {
		return false
	}
	if dx != 0 && dy != 0 && abs(dx) != abs(dy) {
		return false
	}
	if abs(dx) <= 1 && abs(dy) <= 1 {
		return false
	}
	f := game.FileOf(from) + sign(dx)
	r := game.RankOf(from) + sign(dy)
	if !game.OnBoard(f, r) {
		return false
	}
	return s.Board[game.SqAt(f, r)] == game.NoPiece
}

func squareAhead(from game.Square, d game.Dir, n int) (game.Square, bool) {
	dx, dy := game.DirDelta(d)
	f := game.FileOf(from) + dx*n
	r := game.RankOf(from) + dy*n
	if !game.OnBoard(f, r) {
		return game.NoSquare, false
	}
	return game.SqAt(f, r), true
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func sign(v int) int {
	switch {
	case v > 0:
		return 1
	case v < 0:
		return -1
	default:
		return 0
	}
}
