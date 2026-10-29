package ml_r2_v2

import "wizardboardgame/internal/game"

// unitValue is the raw material value of a piece.
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

// aimValue is the target-aware aim bonus: pawn +3, cannon +8, wizard 0.
func aimValue(t game.UnitType) int {
	switch t {
	case game.UnitPawn:
		return 3
	case game.UnitCannon:
		return 8
	}
	return 0
}

// threatValue weights the wizard threats by the attacking piece type.
func threatValue(t [game.UnitTypeCount]int) int {
	return 10*t[0] + 5*t[1] + 2*t[2]
}

// sideParts is one side's contribution to the hand evaluation. Every component
// is kept separately so the ML features can reuse them without recomputation.
type sideParts struct {
	Count    [game.UnitTypeCount]int
	Buffed   [game.UnitTypeCount]int
	Nerfed   [game.UnitTypeCount]int
	Material int
	Buff     int // +4 per buffed piece
	Nerf     int // -4 per nerfed piece
	Pawn     int
	Aim      [game.UnitTypeCount]int // aimed cannons by target type
	AimValue int
	AimCount int
	Mobility int
	Threats  [game.UnitTypeCount]int // threats to this side's wizards by enemy type
	Danger   int
	Tokens   int

	ForwardReady  int
	TeleportReady bool
	FacingEnemy   int
	BuffedAim     int
	WizMinDist    [game.UnitTypeCount]int // min Chebyshev distance to an enemy piece type
}

// HandParts is the full hand-evaluation breakdown for both sides.
type HandParts struct {
	A, B sideParts
}

// Total returns the hand-evaluation score from turn's perspective.
func (p HandParts) Total(turn game.Player) int {
	a, b := p.A, p.B
	if turn == game.PlayerB {
		a, b = b, a
	}
	own := a.Material + a.Buff + a.Nerf + a.Pawn + a.AimValue + a.Mobility + a.Tokens*3
	opp := b.Material + b.Buff + b.Nerf + b.Pawn + b.AimValue + b.Mobility + b.Tokens*3
	return own - opp + b.Danger - a.Danger
}

// HandPartsOf computes every hand-evaluation component exactly once.
func HandPartsOf(s *game.State) HandParts {
	var p HandParts
	var buffed, nerfed [game.MaxPieces]bool

	for i := 0; i < game.MaxPieces; i++ {
		q := &s.Pieces[i]
		if !q.Alive {
			continue
		}
		side := &p.A
		if q.Owner == game.PlayerB {
			side = &p.B
		}
		side.Count[q.Type]++
		side.Material += unitValue(q.Type)
		if t := s.Buffs[i]; t != game.NoPiece {
			buffed[t] = true
		}
		if t := s.Nerfs[i]; t != game.NoPiece {
			nerfed[t] = true
		}
	}

	for i := 0; i < game.MaxPieces; i++ {
		q := &s.Pieces[i]
		if !q.Alive {
			continue
		}
		side := &p.A
		if q.Owner == game.PlayerB {
			side = &p.B
		}
		if buffed[i] {
			side.Buffed[q.Type]++
			side.Buff += 4
		}
		if nerfed[i] {
			side.Nerfed[q.Type]++
			side.Nerf -= 4
		}
		if q.Type == game.UnitPawn {
			if q.Owner == game.PlayerA {
				side.Pawn += game.RankOf(q.Sq) / 2
			} else {
				side.Pawn += (7 - game.RankOf(q.Sq)) / 2
			}
			if s.Rules.PawnForwardCaptureEvery > 0 && (q.Acts+1)%s.Rules.PawnForwardCaptureEvery == 0 {
				side.ForwardReady++
			}
		}
	}

	p.A.Tokens = int(s.Token[game.PlayerA])
	p.B.Tokens = int(s.Token[game.PlayerB])
	if s.Rules.TeleportCost > 0 {
		p.A.TeleportReady = s.Token[game.PlayerA] >= s.Rules.TeleportCost
		p.B.TeleportReady = s.Token[game.PlayerB] >= s.Rules.TeleportCost
	}

	for i := 0; i < game.MaxPieces; i++ {
		q := &s.Pieces[i]
		if !q.Alive || q.Type != game.UnitCannon {
			continue
		}
		side := &p.A
		if q.Owner == game.PlayerB {
			side = &p.B
		}
		side.Mobility += cannonMobility(s, q.Sq)
		if t, ok := aimTarget(s, game.PieceID(i), buffed); ok {
			side.Aim[t]++
			side.AimCount++
			side.AimValue += aimValue(t)
			if buffed[i] {
				side.BuffedAim++
			}
		}
		_, dy := game.DirDelta(q.Facing)
		sign := 1
		if q.Owner == game.PlayerB {
			sign = -1
		}
		if dy*sign > 0 {
			side.FacingEnemy++
		}
	}

	p.A.Threats = wizardThreats(s, game.PlayerA, buffed)
	p.B.Threats = wizardThreats(s, game.PlayerB, buffed)
	p.A.Danger = threatValue(p.A.Threats)
	p.B.Danger = threatValue(p.B.Threats)
	p.A.WizMinDist = wizardMinDist(s, game.PlayerA)
	p.B.WizMinDist = wizardMinDist(s, game.PlayerB)
	return p
}

// HandEval is the pure hand evaluation (terminal states included).
func HandEval(s *game.State) int {
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
	return HandPartsOf(s).Total(s.Turn)
}

// wizardThreats counts the enemy pieces threatening the owner's wizards.
func wizardThreats(s *game.State, owner game.Player, buffed [game.MaxPieces]bool) [game.UnitTypeCount]int {
	var out [game.UnitTypeCount]int
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
					out[game.UnitCannon]++
				}
			case game.UnitPawn:
				if pawnThreatens(s, game.PieceID(eid), w.Sq) {
					out[game.UnitPawn]++
				}
			case game.UnitWizard:
				if stepTowardEmpty(s, e.Sq, w.Sq) {
					out[game.UnitWizard]++
				}
			}
		}
	}
	return out
}

// wizardMinDist returns, per enemy piece type, the minimum Chebyshev distance
// from this side's wizards to an enemy piece (8 when there is none).
func wizardMinDist(s *game.State, owner game.Player) [game.UnitTypeCount]int {
	out := [game.UnitTypeCount]int{8, 8, 8}
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
			d := chebyshev(w.Sq, e.Sq)
			if d < out[e.Type] {
				out[e.Type] = d
			}
		}
	}
	return out
}

func chebyshev(a, b game.Square) int {
	dx := abs(game.FileOf(a) - game.FileOf(b))
	dy := abs(game.RankOf(a) - game.RankOf(b))
	if dx > dy {
		return dx
	}
	return dy
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

// cannonMobility rewards cannons that are not boxed in.
func cannonMobility(s *game.State, sq game.Square) int {
	n := 0
	for d := game.Dir(0); d < game.DirCount; d++ {
		to, ok := squareAhead(sq, d, 1)
		if ok && s.Board[to] == game.NoPiece {
			n++
		}
	}
	if n > 4 {
		n = 4
	}
	return n
}

func shotRanges(s *game.State, id game.PieceID, buffed [game.MaxPieces]bool) [2]int {
	ranges := [2]int{s.Rules.CannonRange, 0}
	if buffed[id] && s.Rules.CannonBuffRange > 0 && s.Rules.CannonBuffRange != s.Rules.CannonRange {
		ranges[1] = s.Rules.CannonBuffRange
	}
	return ranges
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

// pawnThreatens reports whether the pawn could capture the wizard next.
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
