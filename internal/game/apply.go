package game

import "errors"

var (
	// ErrGameOver is returned when an action is applied after the game ended.
	ErrGameOver = errors.New("game is over")
	// ErrIllegal is returned for actions that do not follow the rules.
	ErrIllegal = errors.New("illegal action")
)

// Apply returns the state that results from playing a. s itself is never
// modified: Apply is a pure transition function, which is what makes the
// engine usable for search, simulation and machine learning.
func (s *State) Apply(a Action) (State, error) {
	ns := *s
	if err := ns.apply(a); err != nil {
		return *s, err
	}
	return ns, nil
}

// ApplyInto is the allocation-free variant of Apply: it copies s into dst and
// then applies a in place on dst. src is left untouched.
func (s *State) ApplyInto(dst *State, a Action) error {
	*dst = *s
	return dst.apply(a)
}

func (s *State) apply(a Action) error {
	if s.Over {
		return ErrGameOver
	}
	if a.Kind == ActPass {
		s.endTurn()
		return nil
	}
	if s.Pending != NoPiece {
		if a.Kind != ActFreeMove || a.Unit != s.Pending {
			return ErrIllegal
		}
		return s.applyFreeMove(a)
	}
	if a.Unit >= MaxPieces {
		return ErrIllegal
	}
	u := &s.Pieces[a.Unit]
	if !u.Alive || u.Owner != s.Turn || u.Type != s.Phase(s.Turn) {
		return ErrIllegal
	}
	switch a.Kind {
	case ActMove:
		return s.applyMove(a)
	case ActCapture:
		return s.applyCapture(a)
	case ActShoot:
		return s.applyShoot(a)
	case ActTurn:
		return s.applyTurn(a)
	case ActMoveTurn:
		return s.applyMoveTurn(a)
	case ActCast:
		return s.applyCast(a)
	case ActSwap:
		return s.applySwap(a)
	case ActSkip:
		s.gainToken(u.Owner)
		s.endTurn()
		return nil
	case ActTeleport:
		return s.applyTeleport(a)
	default:
		return ErrIllegal
	}
}

func (s *State) applyMove(a Action) error {
	u := &s.Pieces[a.Unit]
	if a.To == NoSquare || a.To >= NumSquares || s.Board[a.To] != NoPiece {
		return ErrIllegal
	}
	dx := FileOf(a.To) - FileOf(u.Sq)
	dy := RankOf(a.To) - RankOf(u.Sq)
	if absInt(dx) > 1 || absInt(dy) > 1 || (dx == 0 && dy == 0) {
		return ErrIllegal
	}
	if u.Type == UnitPawn && s.IsNerfed(a.Unit) && s.hasBackwardStep(u) {
		// A nerfed pawn must retreat when it can.
		sign := 1
		if u.Owner == PlayerB {
			sign = -1
		}
		if dy*sign >= 0 {
			return ErrIllegal
		}
	}
	s.movePiece(a.Unit, a.To)
	u.Acts++
	s.endTurn()
	return nil
}

func (s *State) applyCapture(a Action) error {
	u := &s.Pieces[a.Unit]
	if u.Type != UnitPawn {
		return ErrIllegal
	}
	if a.To == NoSquare || a.To >= NumSquares {
		return ErrIllegal
	}
	target := s.Board[a.To]
	if target == NoPiece || s.Pieces[target].Owner == u.Owner {
		return ErrIllegal
	}
	if !s.pawnCanCapture(a.Unit, a.To) {
		return ErrIllegal
	}
	s.kill(target)
	s.movePiece(a.Unit, a.To)
	u.Acts++
	s.endTurn()
	return nil
}

func (s *State) pawnCanCapture(id PieceID, to Square) bool {
	u := &s.Pieces[id]
	fwdSign := 1
	if u.Owner == PlayerB {
		fwdSign = -1
	}
	fwd := (RankOf(to) - RankOf(u.Sq)) * fwdSign
	switch {
	case fwd < 0:
		return true
	case fwd > 0:
		if s.Rules.PawnForwardCaptureEvery == 0 {
			return false
		}
		return (u.Acts+1)%s.Rules.PawnForwardCaptureEvery == 0
	default:
		return s.Rules.PawnBuffSideCapture && s.IsBuffed(id)
	}
}

func (s *State) applyShoot(a Action) error {
	u := &s.Pieces[a.Unit]
	if u.Type != UnitCannon {
		return ErrIllegal
	}
	if a.To == NoSquare || a.To >= NumSquares {
		return ErrIllegal
	}
	target := s.Board[a.To]
	if target == NoPiece || s.Pieces[target].Owner == u.Owner {
		return ErrIllegal
	}
	dist := -1
	for r := 1; r <= 8; r++ {
		sq, ok := stepFrom(u.Sq, u.Facing, r)
		if !ok {
			break
		}
		if sq == a.To {
			dist = r
			break
		}
	}
	if dist < 0 {
		return ErrIllegal
	}
	if !s.cannonCanShootAt(a.Unit, dist) {
		return ErrIllegal
	}
	s.kill(target)
	s.endTurn()
	return nil
}

func (s *State) cannonCanShootAt(id PieceID, dist int) bool {
	if dist == s.Rules.CannonRange {
		return true
	}
	return s.IsBuffed(id) && dist == s.Rules.CannonBuffRange
}

func (s *State) applyTurn(a Action) error {
	u := &s.Pieces[a.Unit]
	if u.Type != UnitCannon || s.IsNerfed(a.Unit) {
		return ErrIllegal
	}
	u.Facing = a.Facing % DirCount
	s.endTurn()
	return nil
}

func (s *State) applyMoveTurn(a Action) error {
	u := &s.Pieces[a.Unit]
	if u.Type != UnitCannon || s.IsNerfed(a.Unit) {
		return ErrIllegal
	}
	if a.To == NoSquare || a.To >= NumSquares || s.Board[a.To] != NoPiece {
		return ErrIllegal
	}
	dx := FileOf(a.To) - FileOf(u.Sq)
	dy := RankOf(a.To) - RankOf(u.Sq)
	if absInt(dx) > 1 || absInt(dy) > 1 || (dx == 0 && dy == 0) {
		return ErrIllegal
	}
	s.movePiece(a.Unit, a.To)
	u.Facing = a.Facing % DirCount
	u.Acts++
	s.endTurn()
	return nil
}

func (s *State) applyCast(a Action) error {
	w := &s.Pieces[a.Unit]
	if w.Type != UnitWizard {
		return ErrIllegal
	}
	if a.Target >= MaxPieces || a.Target == a.Unit {
		return ErrIllegal
	}
	t := &s.Pieces[a.Target]
	if !t.Alive {
		return ErrIllegal
	}
	if s.Rules.WizardStepTowardTarget {
		stepSq, ok := s.castStep(w.Sq, t.Sq)
		if !ok || stepSq != a.To {
			return ErrIllegal
		}
		s.movePiece(a.Unit, a.To)
	} else if a.To != NoSquare {
		if a.To >= NumSquares || s.Board[a.To] != NoPiece {
			return ErrIllegal
		}
		s.movePiece(a.Unit, a.To)
	}

	switch a.Spell {
	case SpellBuff:
		if t.Owner != w.Owner {
			return ErrIllegal
		}
		s.clearBuffOn(a.Target)
		s.Buffs[a.Unit] = a.Target
	case SpellNerf:
		if t.Owner == w.Owner {
			return ErrIllegal
		}
		if t.Type == UnitWizard && !s.Rules.WizardNerf {
			return ErrIllegal
		}
		s.clearNerfOn(a.Target)
		s.Nerfs[a.Unit] = a.Target
	case SpellCleanse:
		if !s.cleanseVisible(w.Sq, t.Sq) {
			return ErrIllegal
		}
		if t.Owner == w.Owner {
			if s.NerfCaster(a.Target) == NoPiece {
				return ErrIllegal
			}
			s.clearNerfOn(a.Target)
		} else {
			if s.BuffCaster(a.Target) == NoPiece {
				return ErrIllegal
			}
			s.clearBuffOn(a.Target)
		}
	default:
		return ErrIllegal
	}

	// A buffed wizard may move anywhere after casting. That move is resolved
	// through a follow-up ActFreeMove action before the turn ends.
	if s.IsBuffed(a.Unit) {
		s.Pending = a.Unit
		return nil
	}
	s.endTurn()
	return nil
}

// applySwap exchanges the acting wizard with an adjacent friendly unit. Both
// pieces keep their identity, so buffs, nerfs and the pawn action counter stay
// attached to the piece rather than the square.
func (s *State) applySwap(a Action) error {
	if !s.Rules.WizardSwap {
		return ErrIllegal
	}
	u := &s.Pieces[a.Unit]
	if u.Type != UnitWizard {
		return ErrIllegal
	}
	if a.Target >= MaxPieces || a.Target == a.Unit {
		return ErrIllegal
	}
	t := &s.Pieces[a.Target]
	if !t.Alive || t.Owner != u.Owner {
		return ErrIllegal
	}
	dx := FileOf(t.Sq) - FileOf(u.Sq)
	dy := RankOf(t.Sq) - RankOf(u.Sq)
	if absInt(dx) > 1 || absInt(dy) > 1 || (dx == 0 && dy == 0) {
		return ErrIllegal
	}
	if a.To != NoSquare && a.To != t.Sq {
		return ErrIllegal
	}
	from, to := u.Sq, t.Sq
	s.Board[from] = a.Target
	s.Board[to] = a.Unit
	u.Sq = to
	t.Sq = from
	s.endTurn()
	return nil
}

func (s *State) applyTeleport(a Action) error {
	u := &s.Pieces[a.Unit]
	if u.Type != UnitWizard {
		return ErrIllegal
	}
	cost := s.Rules.TeleportCost
	if cost == 0 || s.Token[u.Owner] < cost {
		return ErrIllegal
	}
	if a.To == NoSquare || a.To >= NumSquares || s.Board[a.To] != NoPiece {
		return ErrIllegal
	}
	s.movePiece(a.Unit, a.To)
	s.Token[u.Owner] -= cost
	s.endTurn()
	return nil
}

func (s *State) applyFreeMove(a Action) error {
	if a.Unit >= MaxPieces || a.Unit != s.Pending {
		return ErrIllegal
	}
	if a.To != NoSquare {
		if a.To >= NumSquares || s.Board[a.To] != NoPiece {
			return ErrIllegal
		}
		s.movePiece(a.Unit, a.To)
	}
	s.Pending = NoPiece
	s.endTurn()
	return nil
}

func (s *State) movePiece(id PieceID, to Square) {
	p := &s.Pieces[id]
	s.Board[p.Sq] = NoPiece
	p.Sq = to
	s.Board[to] = id
}

func (s *State) gainToken(p Player) {
	if s.Token[p] < s.Rules.MaxTokens {
		s.Token[p]++
	}
}

func (s *State) clearBuffOn(target PieceID) {
	for i := 0; i < MaxPieces; i++ {
		if s.Buffs[i] == target {
			s.Buffs[i] = NoPiece
		}
	}
}

func (s *State) clearNerfOn(target PieceID) {
	for i := 0; i < MaxPieces; i++ {
		if s.Nerfs[i] == target {
			s.Nerfs[i] = NoPiece
		}
	}
}

// kill removes a piece and every effect it cast or received.
func (s *State) kill(id PieceID) {
	p := &s.Pieces[id]
	if !p.Alive {
		return
	}
	s.Board[p.Sq] = NoPiece
	p.Alive = false
	s.Buffs[id] = NoPiece
	s.Nerfs[id] = NoPiece
	for i := 0; i < MaxPieces; i++ {
		if s.Buffs[i] == id {
			s.Buffs[i] = NoPiece
		}
		if s.Nerfs[i] == id {
			s.Nerfs[i] = NoPiece
		}
	}
	if s.Pending == id {
		s.Pending = NoPiece
	}
}

func (s *State) endTurn() {
	p := s.Turn
	s.Ply++
	cur := s.PhaseIndex(p)
	s.Next[p] = s.nextAvailable(p, (cur+1)%UnitTypeCount)
	s.Turn = p.Opponent()
	s.checkOver()
}

func (s *State) checkOver() {
	if s.Over {
		return
	}
	if !s.HasWizards(PlayerA) {
		s.Over = true
		s.Winner = PlayerB
		return
	}
	if !s.HasWizards(PlayerB) {
		s.Over = true
		s.Winner = PlayerA
		return
	}
	if s.Rules.MaxPly > 0 && s.Ply >= s.Rules.MaxPly {
		s.Over = true
		s.Winner = NoPlayer
	}
}
