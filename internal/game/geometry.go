package game

import "math"

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func signInt(v int) int {
	switch {
	case v > 0:
		return 1
	case v < 0:
		return -1
	default:
		return 0
	}
}

// stepFrom walks n squares from sq in direction d.
func stepFrom(sq Square, d Dir, n int) (Square, bool) {
	f := FileOf(sq) + dirDX[d]*n
	r := RankOf(sq) + dirDY[d]*n
	if !OnBoard(f, r) {
		return NoSquare, false
	}
	return SqAt(f, r), true
}

// castStep returns the empty square the wizard must step onto to cast at the
// target. A target is only castable when it lies exactly on one of the 8 rays
// (straight or diagonal); the first square along that ray must be empty and is
// where the wizard steps. Adjacent targets leave no square to step onto, so
// they cannot be cast on.
func (s *State) castStep(from, to Square) (Square, bool) {
	dx := FileOf(to) - FileOf(from)
	dy := RankOf(to) - RankOf(from)
	if dx == 0 && dy == 0 {
		return NoSquare, false
	}
	// Not on a straight or diagonal ray.
	if dx != 0 && dy != 0 && absInt(dx) != absInt(dy) {
		return NoSquare, false
	}
	// Adjacent: the only square in that direction is the target itself.
	if absInt(dx) <= 1 && absInt(dy) <= 1 {
		return NoSquare, false
	}
	f := FileOf(from) + signInt(dx)
	r := RankOf(from) + signInt(dy)
	if !OnBoard(f, r) {
		return NoSquare, false
	}
	sq := SqAt(f, r)
	if s.Board[sq] != NoPiece {
		return NoSquare, false
	}
	return sq, true
}

// cleanseVisible reports whether a cleanse may be cast from `from` at `to`.
// Only cleanse requires line of sight; buff and nerf ignore pieces in between.
func (s *State) cleanseVisible(from, to Square) bool {
	if !s.Rules.WizardNeedsLOS {
		return true
	}
	return s.los(from, to)
}

// los reports whether the straight line between from and to is free of pieces.
// Endpoints are ignored. A 2x oversampled DDA keeps the test conservative for
// off-axis lines.
func (s *State) los(from, to Square) bool {
	if from == to {
		return true
	}
	fx, fy := float64(FileOf(from)), float64(RankOf(from))
	tx, ty := float64(FileOf(to)), float64(RankOf(to))
	dx, dy := tx-fx, ty-fy
	steps := absInt(FileOf(to) - FileOf(from))
	if v := absInt(RankOf(to) - RankOf(from)); v > steps {
		steps = v
	}
	steps *= 2
	if steps == 0 {
		return true
	}
	for i := 1; i < steps; i++ {
		t := float64(i) / float64(steps)
		f := int(math.Round(fx + dx*t))
		r := int(math.Round(fy + dy*t))
		if !OnBoard(f, r) {
			return false
		}
		sq := SqAt(f, r)
		if sq == from || sq == to {
			continue
		}
		if s.Board[sq] != NoPiece {
			return false
		}
	}
	return true
}
