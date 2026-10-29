package game

import "fmt"

// Player identifies one of the two sides.
type Player uint8

const (
	PlayerA  Player = 0
	PlayerB  Player = 1
	NoPlayer Player = 255
)

func (p Player) String() string {
	switch p {
	case PlayerA:
		return "A"
	case PlayerB:
		return "B"
	default:
		return "-"
	}
}

// Opponent returns the other player.
func (p Player) Opponent() Player {
	if p == PlayerA {
		return PlayerB
	}
	return PlayerA
}

// UnitType is the kind of a piece.
type UnitType uint8

const (
	UnitPawn UnitType = iota
	UnitCannon
	UnitWizard
	UnitTypeCount = 3
)

func (u UnitType) String() string {
	switch u {
	case UnitPawn:
		return "pawn"
	case UnitCannon:
		return "cannon"
	case UnitWizard:
		return "wizard"
	default:
		return "?"
	}
}

// rotOrder is the fixed order in which a player cycles through unit types.
var rotOrder = [UnitTypeCount]UnitType{UnitPawn, UnitCannon, UnitWizard}

// PieceID is a stable identifier for a piece (0..MaxPieces-1).
type PieceID uint8

const (
	NoPiece   PieceID = 255
	MaxPieces         = 24
)

// Square is a board index: file (0..7) + rank (0..7)*8.
type Square uint8

const (
	NoSquare   Square = 255
	NumSquares        = 64
)

func FileOf(s Square) int { return int(s) & 7 }
func RankOf(s Square) int { return int(s) >> 3 }

// SqAt builds a square from file/rank. Callers must check OnBoard first.
func SqAt(file, rank int) Square { return Square(rank*8 + file) }

// OnBoard reports whether file/rank is a valid square.
func OnBoard(file, rank int) bool {
	return file >= 0 && file < 8 && rank >= 0 && rank < 8
}

func (s Square) String() string {
	if s == NoSquare {
		return "-"
	}
	return fmt.Sprintf("%c%d", 'a'+FileOf(s), RankOf(s)+1)
}

// ParseSquare parses algebraic coordinates like "e4".
func ParseSquare(str string) (Square, bool) {
	if len(str) < 2 {
		return NoSquare, false
	}
	file := int(str[0] - 'a')
	rank := int(str[1] - '1')
	if !OnBoard(file, rank) {
		return NoSquare, false
	}
	return SqAt(file, rank), true
}

// Dir is one of the 8 compass directions.
type Dir uint8

const (
	North Dir = iota
	NorthEast
	East
	SouthEast
	South
	SouthWest
	West
	NorthWest
	DirCount = 8
)

var dirDX = [DirCount]int{0, 1, 1, 1, 0, -1, -1, -1}
var dirDY = [DirCount]int{1, 1, 0, -1, -1, -1, 0, 1}

// DirDelta returns the file/rank delta of a direction.
func DirDelta(d Dir) (dx, dy int) { return dirDX[d], dirDY[d] }

func (d Dir) String() string {
	switch d {
	case North:
		return "N"
	case NorthEast:
		return "NE"
	case East:
		return "E"
	case SouthEast:
		return "SE"
	case South:
		return "S"
	case SouthWest:
		return "SW"
	case West:
		return "W"
	case NorthWest:
		return "NW"
	default:
		return "?"
	}
}

// DirFromDelta returns the direction matching a delta, if the delta is one of
// the 8 unit steps.
func DirFromDelta(dx, dy int) (Dir, bool) {
	for d := Dir(0); d < DirCount; d++ {
		if dirDX[d] == dx && dirDY[d] == dy {
			return d, true
		}
	}
	return North, false
}

// ActionKind enumerates everything a player can do in a turn.
type ActionKind uint8

const (
	ActMove     ActionKind = iota // move to an empty square
	ActCapture                    // pawn captures an adjacent enemy
	ActShoot                      // cannon shoots a piece
	ActTurn                       // cannon rotates in place
	ActMoveTurn                   // cannon moves then faces a direction
	ActCast                       // wizard casts buff / nerf / cleanse
	ActSkip                       // wizard passes and gains a token
	ActTeleport                   // wizard spends a token to teleport
	ActFreeMove                   // buffed wizard's optional move after a cast
	ActPass                       // forced pass when no action is legal
	ActSwap                       // wizard swaps places with an adjacent friendly unit
)

func (k ActionKind) String() string {
	switch k {
	case ActMove:
		return "move"
	case ActCapture:
		return "capture"
	case ActShoot:
		return "shoot"
	case ActTurn:
		return "turn"
	case ActMoveTurn:
		return "moveturn"
	case ActCast:
		return "cast"
	case ActSkip:
		return "skip"
	case ActTeleport:
		return "teleport"
	case ActFreeMove:
		return "freemove"
	case ActPass:
		return "pass"
	case ActSwap:
		return "swap"
	default:
		return "?"
	}
}

// Spell is the kind of magic a wizard can cast.
type Spell uint8

const (
	SpellNone Spell = iota
	SpellBuff
	SpellNerf
	SpellCleanse
)

func (s Spell) String() string {
	switch s {
	case SpellBuff:
		return "buff"
	case SpellNerf:
		return "nerf"
	case SpellCleanse:
		return "cleanse"
	default:
		return "none"
	}
}

// Action is a single, fully described move. It is a value type so it can be
// hashed, compared and serialized cheaply.
type Action struct {
	Kind   ActionKind
	Unit   PieceID // the acting piece
	From   Square  // origin (filled in by the generator, useful for UI/replay)
	To     Square  // destination or shot target; NoSquare for none
	Target PieceID // captured / buffed / nerfed / cleansed piece
	Spell  Spell
	Facing Dir // for ActTurn / ActMoveTurn
}

// Result is an outcome from the perspective of the side to move.
type Result int8

const (
	ResultOngoing Result = 0
	ResultWin     Result = 1
	ResultLoss    Result = -1
	ResultDraw    Result = 0
)
