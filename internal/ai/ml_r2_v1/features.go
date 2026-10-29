package ml_r2_v1

import "wizardboardgame/internal/game"

// FeatureCount is the length of the compact, side-relative feature vector.
const FeatureCount = 24

// Features returns the compact feature vector used by the learned evaluation.
func Features(s *game.State) []float32 {
	return FeaturesWithHand(s, HandEval(s))
}

// FeaturesWithHand avoids recomputing the hand evaluation when the caller
// already has it (the engine's evaluate does).
func FeaturesWithHand(s *game.State, hand int) []float32 {
	f := make([]float32, FeatureCount)
	me, opp := s.Turn, s.Turn.Opponent()

	var mat, buffs, nerfs, aim [game.UnitTypeCount][2]int
	var pawnAdv, mobility, pieces [2]int
	var buffed, nerfed [game.MaxPieces]bool

	for i := 0; i < game.MaxPieces; i++ {
		p := &s.Pieces[i]
		if !p.Alive {
			continue
		}
		side := 0
		if p.Owner == opp {
			side = 1
		}
		mat[p.Type][side]++
		pieces[side]++
		if p.Type == game.UnitPawn {
			if p.Owner == game.PlayerA {
				pawnAdv[side] += game.RankOf(p.Sq) / 2
			} else {
				pawnAdv[side] += (7 - game.RankOf(p.Sq)) / 2
			}
		}
		if t := s.Buffs[i]; t != game.NoPiece {
			buffed[t] = true
		}
		if t := s.Nerfs[i]; t != game.NoPiece {
			nerfed[t] = true
		}
	}

	for i := 0; i < game.MaxPieces; i++ {
		p := &s.Pieces[i]
		if !p.Alive {
			continue
		}
		side := 0
		if p.Owner == opp {
			side = 1
		}
		if buffed[i] {
			buffs[p.Type][side]++
		}
		if nerfed[i] {
			nerfs[p.Type][side]++
		}
	}

	for i := 0; i < game.MaxPieces; i++ {
		p := &s.Pieces[i]
		if !p.Alive || p.Type != game.UnitCannon {
			continue
		}
		side := 0
		if p.Owner == opp {
			side = 1
		}
		mobility[side] += cannonMobility(s, p.Sq)
		if t, ok := aimTarget(s, game.PieceID(i), buffed); ok {
			aim[t][side]++
		}
	}

	idx := 0
	for t := 0; t < game.UnitTypeCount; t++ {
		f[idx] = float32(mat[t][0] - mat[t][1])
		idx++
	}
	for t := 0; t < game.UnitTypeCount; t++ {
		f[idx] = float32(buffs[t][0] - buffs[t][1])
		idx++
	}
	for t := 0; t < game.UnitTypeCount; t++ {
		f[idx] = float32(nerfs[t][0] - nerfs[t][1])
		idx++
	}
	f[idx] = float32(int(s.Token[me]) - int(s.Token[opp]))
	idx++
	phase := int(s.PhaseIndex(me))
	for p := 0; p < game.UnitTypeCount; p++ {
		if p == phase {
			f[idx] = 1
		}
		idx++
	}
	f[idx] = float32(pawnAdv[0] - pawnAdv[1])
	idx++
	aimTotal := 0
	for t := 0; t < game.UnitTypeCount; t++ {
		aimTotal += aim[t][0] - aim[t][1]
	}
	f[idx] = float32(aimTotal)
	idx++
	f[idx] = float32(mobility[0] - mobility[1])
	idx++
	f[idx] = float32(wizardDanger(s, opp, buffed) - wizardDanger(s, me, buffed))
	idx++
	f[idx] = float32(pieces[0] - pieces[1])
	idx++
	f[idx] = float32(hand) / 100
	idx++
	if s.Rules.MaxPly > 0 {
		f[idx] = float32(s.Ply) / float32(s.Rules.MaxPly)
	}
	idx++
	if s.Pending != game.NoPiece {
		f[idx] = 1
	}
	idx++
	for t := 0; t < game.UnitTypeCount; t++ {
		f[idx] = float32(aim[t][0] - aim[t][1])
		idx++
	}
	return f
}

// aimTarget reports the type of the enemy the cannon is aiming at, if any.
func aimTarget(s *game.State, id game.PieceID, buffed [game.MaxPieces]bool) (game.UnitType, bool) {
	u := &s.Pieces[id]
	if !u.Alive || u.Type != game.UnitCannon {
		return 0, false
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
			return s.Pieces[occ].Type, true
		}
	}
	return 0, false
}
