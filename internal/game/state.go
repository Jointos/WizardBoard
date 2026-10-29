package game

// Piece is the mutable-in-a-copy runtime state of a single piece.
type Piece struct {
	Alive  bool
	Owner  Player
	Type   UnitType
	Sq     Square
	Facing Dir
	// Acts counts how many actions this piece has performed (pawns use it for
	// the "every 3rd action" forward capture).
	Acts uint8
}

// State is the complete game state. It contains no pointers, slices or maps
// (except the immutable *Rules), so copying it by value produces an
// independent state. That property is what makes the engine safe to use from
// parallel search / machine-learning code.
type State struct {
	Board  [NumSquares]PieceID
	Pieces [MaxPieces]Piece

	Turn  Player
	Next  [2]uint8 // per-player cursor into rotOrder
	Token [2]uint8

	// Buffs/Nerfs are indexed by the caster wizard's PieceID and hold the
	// target PieceID, or NoPiece. A wizard can hold at most one of each.
	Buffs [MaxPieces]PieceID
	Nerfs [MaxPieces]PieceID

	// Pending is set when a buffed wizard has cast and still owes its free
	// move. While pending, the only legal actions are ActFreeMove.
	Pending PieceID

	Ply    uint16
	Over   bool
	Winner Player

	Rules *Rules
}

// New builds the standard starting position.
func New(r *Rules) State {
	if r == nil {
		r = DefaultRules()
	}
	s := State{Turn: PlayerA, Pending: NoPiece, Winner: NoPlayer, Rules: r}
	for i := 0; i < NumSquares; i++ {
		s.Board[i] = NoPiece
	}
	for i := 0; i < MaxPieces; i++ {
		s.Buffs[i] = NoPiece
		s.Nerfs[i] = NoPiece
	}
	s.setup()
	return s
}

func (s *State) setup() {
	// Pawns sit on the row closest to the middle, cannons behind them,
	// wizards on the back row. Files are centred.
	for i := 0; i < 6; i++ {
		s.place(PlayerA, UnitPawn, i, SqAt(1+i, 2))
		s.place(PlayerB, UnitPawn, i, SqAt(1+i, 5))
	}
	for i := 0; i < 4; i++ {
		s.place(PlayerA, UnitCannon, i, SqAt(2+i, 1))
		s.place(PlayerB, UnitCannon, i, SqAt(2+i, 6))
	}
	s.place(PlayerA, UnitWizard, 0, SqAt(3, 0))
	s.place(PlayerA, UnitWizard, 1, SqAt(4, 0))
	s.place(PlayerB, UnitWizard, 0, SqAt(3, 7))
	s.place(PlayerB, UnitWizard, 1, SqAt(4, 7))

	for id := 0; id < MaxPieces; id++ {
		p := &s.Pieces[id]
		if p.Alive && p.Type == UnitCannon {
			if p.Owner == PlayerA {
				p.Facing = North
			} else {
				p.Facing = South
			}
		}
	}
}

func pieceID(owner Player, t UnitType, idx int) PieceID {
	base := 0
	if owner == PlayerB {
		base = 12
	}
	switch t {
	case UnitPawn:
		return PieceID(base + idx)
	case UnitCannon:
		return PieceID(base + 6 + idx)
	case UnitWizard:
		return PieceID(base + 10 + idx)
	}
	return NoPiece
}

func (s *State) place(owner Player, t UnitType, idx int, sq Square) {
	id := pieceID(owner, t, idx)
	s.Pieces[id] = Piece{Alive: true, Owner: owner, Type: t, Sq: sq}
	s.Board[sq] = id
}

// At returns the piece on a square (NoPiece when empty).
func (s *State) At(sq Square) PieceID { return s.Board[sq] }

// PieceAt returns a copy of the piece on a square and whether it is occupied.
func (s *State) PieceAt(sq Square) (Piece, bool) {
	id := s.Board[sq]
	if id == NoPiece {
		return Piece{}, false
	}
	return s.Pieces[id], true
}

// Unit returns a copy of a piece.
func (s *State) Unit(id PieceID) (Piece, bool) {
	if id >= MaxPieces {
		return Piece{}, false
	}
	p := s.Pieces[id]
	return p, p.Alive
}

// HasAliveType reports whether the player still has a living unit of type t.
func (s *State) HasAliveType(p Player, t UnitType) bool {
	for id := 0; id < MaxPieces; id++ {
		q := &s.Pieces[id]
		if q.Alive && q.Owner == p && q.Type == t {
			return true
		}
	}
	return false
}

// HasWizards reports whether the player still has a living wizard.
func (s *State) HasWizards(p Player) bool { return s.HasAliveType(p, UnitWizard) }

// Phase is the unit type that the player must act with on their turn. Unit
// types the player no longer owns are skipped automatically.
func (s *State) Phase(p Player) UnitType { return rotOrder[s.PhaseIndex(p)] }

// PhaseIndex is the cursor position (0..2) inside the rotation.
func (s *State) PhaseIndex(p Player) uint8 {
	return s.nextAvailable(p, s.Next[p]%UnitTypeCount)
}

func (s *State) nextAvailable(p Player, from uint8) uint8 {
	for i := 0; i < UnitTypeCount; i++ {
		idx := (from + uint8(i)) % UnitTypeCount
		if s.HasAliveType(p, rotOrder[idx]) {
			return idx
		}
	}
	return from % UnitTypeCount
}

// IsBuffed reports whether any living wizard currently buffs the piece.
func (s *State) IsBuffed(id PieceID) bool {
	for i := 0; i < MaxPieces; i++ {
		if s.Buffs[i] == id && s.Pieces[i].Alive {
			return true
		}
	}
	return false
}

// IsNerfed reports whether any living wizard currently nerfs the piece.
func (s *State) IsNerfed(id PieceID) bool {
	for i := 0; i < MaxPieces; i++ {
		if s.Nerfs[i] == id && s.Pieces[i].Alive {
			return true
		}
	}
	return false
}

// BuffCaster returns the wizard that buffs id, or NoPiece.
func (s *State) BuffCaster(id PieceID) PieceID {
	for i := 0; i < MaxPieces; i++ {
		if s.Buffs[i] == id && s.Pieces[i].Alive {
			return PieceID(i)
		}
	}
	return NoPiece
}

// NerfCaster returns the wizard that nerfs id, or NoPiece.
func (s *State) NerfCaster(id PieceID) PieceID {
	for i := 0; i < MaxPieces; i++ {
		if s.Nerfs[i] == id && s.Pieces[i].Alive {
			return PieceID(i)
		}
	}
	return NoPiece
}

// AlivePieces returns the ids of all living pieces in stable order.
func (s *State) AlivePieces() []PieceID {
	out := make([]PieceID, 0, MaxPieces)
	for id := 0; id < MaxPieces; id++ {
		if s.Pieces[id].Alive {
			out = append(out, PieceID(id))
		}
	}
	return out
}

// Clone returns an independent copy of the state.
func (s *State) Clone() State { return *s }

// Done reports whether the game is finished.
func (s *State) Done() bool { return s.Over }

// Score returns the outcome from the side-to-move's perspective, suitable for
// negamax: +1 win, -1 loss, 0 draw or ongoing.
func (s *State) Score() Result {
	if !s.Over {
		return ResultOngoing
	}
	if s.Winner == NoPlayer {
		return ResultDraw
	}
	if s.Winner == s.Turn {
		return ResultWin
	}
	return ResultLoss
}
