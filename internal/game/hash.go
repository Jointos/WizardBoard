package game

// Hash returns a deterministic 64-bit hash of the state. It is intended for
// transposition tables, caching and fast equality checks in search / ML code.
func (s *State) Hash() uint64 {
	const offset64 = 1469598103934665603
	const prime64 = 1099511628211
	h := uint64(offset64)
	mix := func(b byte) {
		h ^= uint64(b)
		h *= prime64
	}

	for id := 0; id < MaxPieces; id++ {
		p := &s.Pieces[id]
		if !p.Alive {
			mix(0)
			continue
		}
		mix(1)
		mix(byte(p.Owner))
		mix(byte(p.Type))
		mix(byte(p.Sq))
		mix(byte(p.Facing))
		mix(p.Acts)
	}
	mix(byte(s.Turn))
	mix(s.Next[0])
	mix(s.Next[1])
	mix(s.Token[0])
	mix(s.Token[1])
	mix(byte(s.Pending))
	mix(byte(s.Ply >> 8))
	mix(byte(s.Ply))
	if s.Over {
		mix(1)
	} else {
		mix(0)
	}
	mix(byte(s.Winner))
	for id := 0; id < MaxPieces; id++ {
		mix(byte(s.Buffs[id]))
		mix(byte(s.Nerfs[id]))
	}
	return h
}

// Equal reports whether two states are identical. Rules pointers are compared
// by value so that independently constructed but equal rule sets still match.
func (s *State) Equal(o *State) bool {
	if s.Turn != o.Turn || s.Ply != o.Ply || s.Over != o.Over || s.Winner != o.Winner {
		return false
	}
	if s.Next != o.Next || s.Token != o.Token || s.Pending != o.Pending {
		return false
	}
	if s.Board != o.Board || s.Buffs != o.Buffs || s.Nerfs != o.Nerfs {
		return false
	}
	for id := 0; id < MaxPieces; id++ {
		if s.Pieces[id] != o.Pieces[id] {
			return false
		}
	}
	return true
}
