package ml_r2_v2

import "wizardboardgame/internal/game"

// FeatureCount is the length of the compact, side-relative feature vector.
const FeatureCount = 43

// Features returns the feature vector for the learned evaluation.
func Features(s *game.State) []float32 {
	return FeaturesWithParts(s, HandPartsOf(s))
}

// FeaturesWithParts avoids recomputing the hand breakdown when the caller
// already has it.
func FeaturesWithParts(s *game.State, p HandParts) []float32 {
	me := s.Turn
	opp := me.Opponent()
	a, b := p.A, p.B
	if me == game.PlayerB {
		a, b = b, a
	}
	f := make([]float32, FeatureCount)
	idx := 0

	// Material counts per type.
	for t := 0; t < game.UnitTypeCount; t++ {
		f[idx] = float32(a.Count[t] - b.Count[t])
		idx++
	}
	// Buffed / nerfed counts per type.
	for t := 0; t < game.UnitTypeCount; t++ {
		f[idx] = float32(a.Buffed[t] - b.Buffed[t])
		idx++
	}
	for t := 0; t < game.UnitTypeCount; t++ {
		f[idx] = float32(a.Nerfed[t] - b.Nerfed[t])
		idx++
	}
	f[idx] = float32(a.Tokens - b.Tokens)
	idx++

	phase := int(s.PhaseIndex(me))
	for t := 0; t < game.UnitTypeCount; t++ {
		if t == phase {
			f[idx] = 1
		}
		idx++
	}

	f[idx] = float32(a.Pawn - b.Pawn)
	idx++
	f[idx] = float32(a.AimCount - b.AimCount)
	idx++
	f[idx] = float32(a.Mobility - b.Mobility)
	idx++
	f[idx] = float32(b.Danger - a.Danger)
	idx++

	piecesA := a.Count[0] + a.Count[1] + a.Count[2]
	piecesB := b.Count[0] + b.Count[1] + b.Count[2]
	f[idx] = float32(piecesA - piecesB)
	idx++
	f[idx] = float32(p.Total(me)) / 100
	idx++
	if s.Rules.MaxPly > 0 {
		f[idx] = float32(s.Ply) / float32(s.Rules.MaxPly)
	}
	idx++
	if s.Pending != game.NoPiece {
		f[idx] = 1
	}
	idx++

	// Aim target types.
	for t := 0; t < game.UnitTypeCount; t++ {
		f[idx] = float32(a.Aim[t] - b.Aim[t])
		idx++
	}
	// Wizard threats by attacking type (more threats on the opponent is good).
	for t := 0; t < game.UnitTypeCount; t++ {
		f[idx] = float32(b.Threats[t] - a.Threats[t])
		idx++
	}
	f[idx] = float32(a.ForwardReady - b.ForwardReady)
	idx++
	f[idx] = boolDiff(a.TeleportReady, b.TeleportReady)
	idx++

	// Wizard distance to enemy piece types (our wizards farther is good).
	for t := 0; t < game.UnitTypeCount; t++ {
		f[idx] = float32(b.WizMinDist[t] - a.WizMinDist[t])
		idx++
	}
	f[idx] = float32(a.FacingEnemy - b.FacingEnemy)
	idx++
	f[idx] = float32(game.NumSquares - piecesA - piecesB)
	idx++
	f[idx] = float32(a.BuffedAim - b.BuffedAim)
	idx++

	// Hand-evaluation components (net, normalized).
	f[idx] = float32(a.Material-b.Material) / 100
	idx++
	f[idx] = float32(a.Buff-b.Buff) / 100
	idx++
	f[idx] = float32(a.Nerf-b.Nerf) / 100
	idx++
	f[idx] = float32(a.Pawn-b.Pawn) / 100
	idx++
	f[idx] = float32(a.AimValue-b.AimValue) / 100
	idx++
	f[idx] = float32(a.Mobility-b.Mobility) / 100
	idx++

	// r2 swap options: how many wizards can swap, and how many friendly
	// neighbours they have in total (capped per wizard so material cannot
	// dominate the feature).
	sa, sb := swapSummary(s, me), swapSummary(s, opp)
	f[idx] = float32(sa.ready - sb.ready)
	idx++
	f[idx] = float32(sa.targets - sb.targets)
	idx++

	return f
}

// swapStats summarises the wizard swap options of one side.
type swapStats struct {
	ready   int // wizards with at least one adjacent friendly piece
	targets int // adjacent friendly pieces, capped at 4 per wizard
}

func swapSummary(s *game.State, owner game.Player) swapStats {
	var out swapStats
	for wid := 0; wid < game.MaxPieces; wid++ {
		w := &s.Pieces[wid]
		if !w.Alive || w.Owner != owner || w.Type != game.UnitWizard {
			continue
		}
		n := 0
		for tid := 0; tid < game.MaxPieces; tid++ {
			if tid == wid {
				continue
			}
			t := &s.Pieces[tid]
			if !t.Alive || t.Owner != owner {
				continue
			}
			dx := game.FileOf(t.Sq) - game.FileOf(w.Sq)
			dy := game.RankOf(t.Sq) - game.RankOf(w.Sq)
			if dx >= -1 && dx <= 1 && dy >= -1 && dy <= 1 {
				n++
			}
		}
		if n > 0 {
			out.ready++
		}
		if n > 4 {
			n = 4
		}
		out.targets += n
	}
	return out
}

func boolDiff(a, b bool) float32 {
	switch {
	case a && !b:
		return 1
	case !a && b:
		return -1
	default:
		return 0
	}
}
