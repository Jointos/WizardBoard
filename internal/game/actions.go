package game

// Actions appends every legal action for the side to move to dst and returns
// the extended slice. The engine never allocates when dst has spare capacity.
//
// If no action is possible the single returned action is ActPass.
func (s *State) Actions(dst []Action) []Action {
	if s.Over {
		return dst
	}
	start := len(dst)
	if s.Pending != NoPiece {
		dst = s.freeMoveActions(dst)
	} else {
		p := s.Turn
		switch s.Phase(p) {
		case UnitPawn:
			dst = s.pawnActions(p, dst)
		case UnitCannon:
			dst = s.cannonActions(p, dst)
		case UnitWizard:
			dst = s.wizardActions(p, dst)
		}
		if len(dst) == start {
			dst = append(dst, Action{Kind: ActPass, Unit: NoPiece, From: NoSquare, To: NoSquare, Target: NoPiece})
		}
	}
	for i := start; i < len(dst); i++ {
		dst[i] = normalizeAction(dst[i])
	}
	return dst
}

// normalizeAction fills in the "not used" fields so that actions never carry
// zero-value garbage (Square 0 is a real square, PieceID 0 is a real piece).
func normalizeAction(a Action) Action {
	switch a.Kind {
	case ActMove, ActMoveTurn, ActTeleport, ActFreeMove:
		a.Target = NoPiece
	case ActTurn, ActSkip, ActPass:
		a.To = NoSquare
		a.Target = NoPiece
		a.Spell = SpellNone
	}
	return a
}

// LegalActions is a convenience wrapper around Actions.
func (s *State) LegalActions() []Action {
	return s.Actions(make([]Action, 0, 64))
}

func (s *State) freeMoveActions(dst []Action) []Action {
	w := s.Pending
	wiz := &s.Pieces[w]
	dst = append(dst, Action{Kind: ActFreeMove, Unit: w, From: wiz.Sq, To: NoSquare})
	for sq := Square(0); sq < NumSquares; sq++ {
		if s.Board[sq] == NoPiece && sq != wiz.Sq {
			dst = append(dst, Action{Kind: ActFreeMove, Unit: w, From: wiz.Sq, To: sq})
		}
	}
	return dst
}

func (s *State) pawnActions(p Player, dst []Action) []Action {
	fwdSign := 1
	if p == PlayerB {
		fwdSign = -1
	}
	for id := 0; id < MaxPieces; id++ {
		u := &s.Pieces[id]
		if !u.Alive || u.Owner != p || u.Type != UnitPawn {
			continue
		}
		pid := PieceID(id)
		nerfed := s.IsNerfed(pid)
		mustRetreat := nerfed && s.hasBackwardStep(u)
		sideOK := s.Rules.PawnBuffSideCapture && s.IsBuffed(pid)
		forwardOK := false
		if s.Rules.PawnForwardCaptureEvery > 0 {
			forwardOK = (u.Acts+1)%s.Rules.PawnForwardCaptureEvery == 0
		}
		for d := Dir(0); d < DirCount; d++ {
			to, ok := stepFrom(u.Sq, d, 1)
			if !ok {
				continue
			}
			fwd := dirDY[d] * fwdSign
			if mustRetreat && fwd >= 0 {
				continue
			}
			occ := s.Board[to]
			if occ == NoPiece {
				dst = append(dst, Action{Kind: ActMove, Unit: pid, From: u.Sq, To: to})
				continue
			}
			if s.Pieces[occ].Owner == p {
				continue
			}
			allowed := false
			switch {
			case fwd < 0:
				allowed = true
			case fwd > 0:
				allowed = forwardOK
			default:
				allowed = sideOK
			}
			if allowed {
				dst = append(dst, Action{Kind: ActCapture, Unit: pid, From: u.Sq, To: to, Target: occ})
			}
		}
	}
	return dst
}

func (s *State) hasBackwardStep(u *Piece) bool {
	sign := 1
	if u.Owner == PlayerB {
		sign = -1
	}
	for d := Dir(0); d < DirCount; d++ {
		if dirDY[d]*sign >= 0 {
			continue
		}
		to, ok := stepFrom(u.Sq, d, 1)
		if ok && s.Board[to] == NoPiece {
			return true
		}
	}
	return false
}

func (s *State) cannonActions(p Player, dst []Action) []Action {
	for id := 0; id < MaxPieces; id++ {
		u := &s.Pieces[id]
		if !u.Alive || u.Owner != p || u.Type != UnitCannon {
			continue
		}
		pid := PieceID(id)
		nerfed := s.IsNerfed(pid)

		// Shoot exactly CannonRange ahead (or CannonBuffRange when buffed).
		ranges := [2]int{s.Rules.CannonRange, 0}
		n := 1
		if s.IsBuffed(pid) && s.Rules.CannonBuffRange > 0 && s.Rules.CannonBuffRange != s.Rules.CannonRange {
			ranges[1] = s.Rules.CannonBuffRange
			n = 2
		}
		for i := 0; i < n; i++ {
			r := ranges[i]
			if r <= 0 {
				continue
			}
			to, ok := stepFrom(u.Sq, u.Facing, r)
			if !ok {
				continue
			}
			occ := s.Board[to]
			if occ != NoPiece && s.Pieces[occ].Owner != p {
				dst = append(dst, Action{Kind: ActShoot, Unit: pid, From: u.Sq, To: to, Target: occ})
			}
		}

		// Rotate in place.
		if !nerfed {
			for d := Dir(0); d < DirCount; d++ {
				if d == u.Facing {
					continue
				}
				dst = append(dst, Action{Kind: ActTurn, Unit: pid, From: u.Sq, Facing: d})
			}
		}

		// Move one square (empty only), optionally turning afterwards.
		for d := Dir(0); d < DirCount; d++ {
			to, ok := stepFrom(u.Sq, d, 1)
			if !ok || s.Board[to] != NoPiece {
				continue
			}
			dst = append(dst, Action{Kind: ActMove, Unit: pid, From: u.Sq, To: to})
			if !nerfed {
				for f := Dir(0); f < DirCount; f++ {
					if f == u.Facing {
						continue
					}
					dst = append(dst, Action{Kind: ActMoveTurn, Unit: pid, From: u.Sq, To: to, Facing: f})
				}
			}
		}
	}
	return dst
}

func (s *State) wizardActions(p Player, dst []Action) []Action {
	for id := 0; id < MaxPieces; id++ {
		w := &s.Pieces[id]
		if !w.Alive || w.Owner != p || w.Type != UnitWizard {
			continue
		}
		wid := PieceID(id)

		// Skip: always available, grants a token (capped).
		dst = append(dst, Action{Kind: ActSkip, Unit: wid, From: w.Sq})

		// Teleport: costs the full token bank, lands on any empty square.
		if cost := s.Rules.TeleportCost; cost > 0 && s.Token[p] >= cost {
			for sq := Square(0); sq < NumSquares; sq++ {
				if s.Board[sq] == NoPiece && sq != w.Sq {
					dst = append(dst, Action{Kind: ActTeleport, Unit: wid, From: w.Sq, To: sq})
				}
			}
		}

		// Swap: exchange places with an adjacent friendly unit. This is the
		// wizard's whole action; it costs nothing and does not move a token.
		if s.Rules.WizardSwap {
			for tid := 0; tid < MaxPieces; tid++ {
				t := &s.Pieces[tid]
				if !t.Alive || t.Owner != p || tid == id {
					continue
				}
				dx := FileOf(t.Sq) - FileOf(w.Sq)
				dy := RankOf(t.Sq) - RankOf(w.Sq)
				if absInt(dx) > 1 || absInt(dy) > 1 {
					continue
				}
				dst = append(dst, Action{Kind: ActSwap, Unit: wid, From: w.Sq, To: t.Sq, Target: PieceID(tid)})
			}
		}

		// Spells: only targets on one of the wizard's 8 rays, and the first
		// square in that direction must be empty (the wizard steps there).
		for tid := 0; tid < MaxPieces; tid++ {
			t := &s.Pieces[tid]
			if !t.Alive || tid == id {
				continue
			}
			stepSq, ok := s.castStep(w.Sq, t.Sq)
			if !ok {
				continue
			}
			tidP := PieceID(tid)
			if t.Owner == p {
				// Buff allies (no line of sight required).
				if s.Buffs[wid] != tidP {
					dst = append(dst, Action{Kind: ActCast, Unit: wid, From: w.Sq, To: stepSq, Target: tidP, Spell: SpellBuff})
				}
				// Cleanse a nerf from an ally (needs line of sight).
				if s.NerfCaster(tidP) != NoPiece && s.cleanseVisible(stepSq, t.Sq) {
					dst = append(dst, Action{Kind: ActCast, Unit: wid, From: w.Sq, To: stepSq, Target: tidP, Spell: SpellCleanse})
				}
			} else {
				// Nerf enemies (no line of sight required). Wizards are not
				// nerfable unless the variant explicitly enables it.
				if (t.Type != UnitWizard || s.Rules.WizardNerf) && s.Nerfs[wid] != tidP {
					dst = append(dst, Action{Kind: ActCast, Unit: wid, From: w.Sq, To: stepSq, Target: tidP, Spell: SpellNerf})
				}
				// Cleanse a buff from an enemy (needs line of sight).
				if s.BuffCaster(tidP) != NoPiece && s.cleanseVisible(stepSq, t.Sq) {
					dst = append(dst, Action{Kind: ActCast, Unit: wid, From: w.Sq, To: stepSq, Target: tidP, Spell: SpellCleanse})
				}
			}
		}
	}
	return dst
}
