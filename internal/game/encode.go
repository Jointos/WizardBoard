package game

import "fmt"

// EncodePlanes is the number of 8x8 planes produced by Encode.
const EncodePlanes = 21

// EncodeScalars is the number of trailing scalar features.
const EncodeScalars = 6

// Encode writes a neural-network friendly representation of the state into dst
// as a flat []float32. Layout:
//
//	planes 0..5    piece planes, index = owner*3 + type
//	planes 6..8    pawn action counter (Acts % 3 == plane-6)
//	plane  9       buffed pieces
//	plane  10      nerfed pieces
//	planes 11..18  cannon facing (Dir == plane-11)
//	plane  19      side to move (1 = A)
//	plane  20      current phase (PhaseIndex / 2)
//	scalars        tokenA/4, tokenB/4, pending, ply/MaxPly, over, winner
func (s *State) Encode(dst []float32) []float32 {
	need := EncodePlanes*NumSquares + EncodeScalars
	if cap(dst) < need {
		dst = make([]float32, need)
	}
	dst = dst[:need]
	for i := range dst {
		dst[i] = 0
	}

	at := func(plane int, sq Square) int { return plane*NumSquares + int(sq) }

	for id := 0; id < MaxPieces; id++ {
		p := &s.Pieces[id]
		if !p.Alive {
			continue
		}
		plane := int(p.Owner)*3 + int(p.Type)
		dst[at(plane, p.Sq)] = 1
		if p.Type == UnitPawn {
			dst[at(6+int(p.Acts%3), p.Sq)] = 1
		}
		if s.IsBuffed(PieceID(id)) {
			dst[at(9, p.Sq)] = 1
		}
		if s.IsNerfed(PieceID(id)) {
			dst[at(10, p.Sq)] = 1
		}
		if p.Type == UnitCannon {
			dst[at(11+int(p.Facing), p.Sq)] = 1
		}
	}

	turnPlane := 19 * NumSquares
	aCount := 0
	if s.Turn == PlayerA {
		aCount = NumSquares
	}
	for i := 0; i < NumSquares; i++ {
		dst[turnPlane+i] = float32(aCount) / NumSquares
		dst[20*NumSquares+i] = float32(s.PhaseIndex(s.Turn)) / 2
	}

	base := EncodePlanes * NumSquares
	dst[base+0] = float32(s.Token[PlayerA]) / float32(s.Rules.MaxTokens)
	dst[base+1] = float32(s.Token[PlayerB]) / float32(s.Rules.MaxTokens)
	if s.Pending != NoPiece {
		dst[base+2] = 1
	}
	if s.Rules.MaxPly > 0 {
		dst[base+3] = float32(s.Ply) / float32(s.Rules.MaxPly)
	}
	if s.Over {
		dst[base+4] = 1
	}
	switch s.Winner {
	case PlayerA:
		dst[base+5] = 1
	case PlayerB:
		dst[base+5] = -1
	}
	return dst
}

// PieceName renders a piece as "Pawn c3" (or "-" for NoPiece).
func (s *State) PieceName(id PieceID) string {
	if id >= MaxPieces || !s.Pieces[id].Alive {
		return "-"
	}
	p := &s.Pieces[id]
	return fmt.Sprintf("%s %s", titleCase(p.Type.String()), p.Sq)
}

// Describe renders an action as a short English sentence. Used for logs and
// the web UI.
func (s *State) Describe(a Action) string {
	unit := s.PieceName(a.Unit)
	switch a.Kind {
	case ActMove:
		return fmt.Sprintf("%s moves to %s", unit, a.To)
	case ActCapture:
		return fmt.Sprintf("%s captures %s", unit, s.PieceName(a.Target))
	case ActShoot:
		return fmt.Sprintf("%s shoots %s", unit, s.PieceName(a.Target))
	case ActTurn:
		return fmt.Sprintf("%s turns to %s", unit, a.Facing)
	case ActMoveTurn:
		return fmt.Sprintf("%s moves to %s and turns to %s", unit, a.To, a.Facing)
	case ActCast:
		if a.To != NoSquare {
			return fmt.Sprintf("%s steps to %s and %ss %s", unit, a.To, a.Spell, s.PieceName(a.Target))
		}
		return fmt.Sprintf("%s %ss %s", unit, a.Spell, s.PieceName(a.Target))
	case ActSkip:
		return fmt.Sprintf("%s skips (+1 token)", unit)
	case ActTeleport:
		return fmt.Sprintf("%s teleports to %s", unit, a.To)
	case ActSwap:
		return fmt.Sprintf("%s swaps with %s", unit, s.PieceName(a.Target))
	case ActFreeMove:
		if a.To == NoSquare {
			return fmt.Sprintf("%s stays in place", unit)
		}
		return fmt.Sprintf("%s moves to %s", unit, a.To)
	case ActPass:
		return "Pass"
	default:
		return "Unknown action"
	}
}

func titleCase(s string) string {
	if s == "" {
		return s
	}
	b := []byte(s)
	if b[0] >= 'a' && b[0] <= 'z' {
		b[0] -= 'a' - 'A'
	}
	return string(b)
}
