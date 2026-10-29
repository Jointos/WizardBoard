package arena

import (
	"math"

	"wizardboardgame/internal/game"
)

// UnitTotals counts events by piece type. The JSON keys are the engine's
// unit names: "pawn", "cannon", "wizard".
type UnitTotals map[string]int

// KillStats breaks kills down two ways: by the piece that died and by the
// attacker type -> victim type matrix.
type KillStats struct {
	ByVictim   UnitTotals            `json:"byVictim"`
	ByAttacker map[string]UnitTotals `json:"byAttacker"`
}

// EventStats is the in-game event tally for one engine, one pair or the whole
// run. It only counts things that actually happened (captures, shots, spells),
// so it is safe to merge across batches.
type EventStats struct {
	Games    int            `json:"games"`
	Plies    int            `json:"plies"`
	Actions  map[string]int `json:"actions"`
	Buffs    UnitTotals     `json:"buffs"`
	Nerfs    UnitTotals     `json:"nerfs"`
	Cleanses UnitTotals     `json:"cleanses"`
	Kills    KillStats      `json:"kills"`
}

// SideStats counts finished games by the side that won. "A" is game.PlayerA
// (the side that moves first), independent of which engine sat there. This is
// the balance metric: it answers "does Player A or Player B win more often?".
type SideStats struct {
	Games int `json:"games"`
	AWin  int `json:"aWin"`
	BWin  int `json:"bWin"`
	Draw  int `json:"draw"`
}

// Record adds one finished game.
func (s *SideStats) Record(w game.Player) {
	s.Games++
	switch w {
	case game.PlayerA:
		s.AWin++
	case game.PlayerB:
		s.BWin++
	default:
		s.Draw++
	}
}

// Add merges o into s.
func (s *SideStats) Add(o SideStats) {
	s.Games += o.Games
	s.AWin += o.AWin
	s.BWin += o.BWin
	s.Draw += o.Draw
}

// WinRateA returns Player A's win share in [0,1] (draws excluded in spirit but
// kept in the denominator, matching the Elo score convention).
func (s SideStats) WinRateA() float64 {
	if s.Games == 0 {
		return 0
	}
	return float64(s.AWin) / float64(s.Games)
}

// WinRateB returns Player B's win share in [0,1].
func (s SideStats) WinRateB() float64 {
	if s.Games == 0 {
		return 0
	}
	return float64(s.BWin) / float64(s.Games)
}

// Wilson returns the 95% Wilson score interval for successes/n. It behaves
// well for small samples and extreme rates, where the normal approximation
// would produce intervals outside [0,1].
func Wilson(successes, n int) (lo, hi float64) {
	if n <= 0 {
		return 0, 1
	}
	p := float64(successes) / float64(n)
	const z = 1.96
	z2 := z * z
	den := 1 + z2/float64(n)
	center := (p + z2/(2*float64(n))) / den
	half := z / den * math.Sqrt(p*(1-p)/float64(n)+z2/(4*float64(n)*float64(n)))
	lo, hi = center-half, center+half
	if lo < 0 {
		lo = 0
	}
	if hi > 1 {
		hi = 1
	}
	return lo, hi
}

// EndStats is the end-of-game / tempo tally. Wins only happen by eliminating
// the opponent's wizards, so "survivors" is the closest thing to a win margin.
type EndStats struct {
	Games      int `json:"games"`
	Plies      int `json:"plies"`
	Stalemates int `json:"stalemates"` // draw by the rules' MaxPly
	SafetyCaps int `json:"safetyCaps"` // arena safety cap, also a draw

	ASurvivors UnitTotals `json:"aSurvivors"`
	BSurvivors UnitTotals `json:"bSurvivors"`
	ATokens    int        `json:"aTokens"`
	BTokens    int        `json:"bTokens"`

	FirstKillA         int `json:"firstKillA"`
	FirstKillB         int `json:"firstKillB"`
	NoFirstKill        int `json:"noFirstKill"`
	FirstKillPlies     int `json:"firstKillPlies"`
	FirstWizardKillA   int `json:"firstWizardKillA"`
	FirstWizardKillB   int `json:"firstWizardKillB"`
	NoFirstWizardKill  int `json:"noFirstWizardKill"`
	FirstWizardPlies   int `json:"firstWizardPlies"`
	FirstWizardWins    int `json:"firstWizardWins"`    // first-wizard-kill side also won
	FirstWizardDecided int `json:"firstWizardDecided"` // games with a first wizard kill and a winner
	FirstKillWins      int `json:"firstKillWins"`      // first-kill side also won
	FirstKillDecided   int `json:"firstKillDecided"`   // games with a first kill and a winner

	// Plies buckets: <50, 50-99, 100-149, 150-199, 200-249, 250+.
	PlyBuckets   [6]int `json:"plyBuckets"`
	AWinByBucket [6]int `json:"aWinByBucket"`
	BWinByBucket [6]int `json:"bWinByBucket"`
	DrawByBucket [6]int `json:"drawByBucket"`
}

// PairEventStats is the event tally of one head-to-head match.
type PairEventStats struct {
	Overall  EventStats             `json:"overall"`
	ByEngine map[string]*EventStats `json:"byEngine"`
	BySide   map[string]*EventStats `json:"bySide"`
}

func newUnitTotals() UnitTotals {
	u := make(UnitTotals, game.UnitTypeCount)
	for t := game.UnitType(0); t < game.UnitTypeCount; t++ {
		u[t.String()] = 0
	}
	return u
}

func newEventStats() EventStats {
	byAttacker := make(map[string]UnitTotals, game.UnitTypeCount)
	for t := game.UnitType(0); t < game.UnitTypeCount; t++ {
		byAttacker[t.String()] = newUnitTotals()
	}
	return EventStats{
		Actions:  make(map[string]int),
		Buffs:    newUnitTotals(),
		Nerfs:    newUnitTotals(),
		Cleanses: newUnitTotals(),
		Kills: KillStats{
			ByVictim:   newUnitTotals(),
			ByAttacker: byAttacker,
		},
	}
}

func newPairEventStats() *PairEventStats {
	return &PairEventStats{
		Overall:  newEventStats(),
		ByEngine: make(map[string]*EventStats),
		BySide:   newSideEventMap(),
	}
}

func newSideEventMap() map[string]*EventStats {
	out := make(map[string]*EventStats, 2)
	for _, p := range []game.Player{game.PlayerA, game.PlayerB} {
		empty := newEventStats()
		out[p.String()] = &empty
	}
	return out
}

// ensure fills the maps of a stats value that came from JSON (older files or
// hand-edited entries may be missing keys).
func (e *EventStats) ensure() {
	if e.Actions == nil {
		e.Actions = make(map[string]int)
	}
	if e.Buffs == nil {
		e.Buffs = newUnitTotals()
	}
	if e.Nerfs == nil {
		e.Nerfs = newUnitTotals()
	}
	if e.Cleanses == nil {
		e.Cleanses = newUnitTotals()
	}
	if e.Kills.ByVictim == nil {
		e.Kills.ByVictim = newUnitTotals()
	}
	if e.Kills.ByAttacker == nil {
		e.Kills.ByAttacker = make(map[string]UnitTotals, game.UnitTypeCount)
	}
	for t := game.UnitType(0); t < game.UnitTypeCount; t++ {
		key := t.String()
		if _, ok := e.Buffs[key]; !ok {
			e.Buffs[key] = 0
		}
		if _, ok := e.Nerfs[key]; !ok {
			e.Nerfs[key] = 0
		}
		if _, ok := e.Cleanses[key]; !ok {
			e.Cleanses[key] = 0
		}
		if _, ok := e.Kills.ByVictim[key]; !ok {
			e.Kills.ByVictim[key] = 0
		}
		if e.Kills.ByAttacker[key] == nil {
			e.Kills.ByAttacker[key] = newUnitTotals()
		}
		for v := game.UnitType(0); v < game.UnitTypeCount; v++ {
			vk := v.String()
			if _, ok := e.Kills.ByAttacker[key][vk]; !ok {
				e.Kills.ByAttacker[key][vk] = 0
			}
		}
	}
}

// Add merges o into e.
func (e *EventStats) Add(o *EventStats) {
	if o == nil {
		return
	}
	e.ensure()
	o.ensure()
	e.Games += o.Games
	e.Plies += o.Plies
	for k, v := range o.Actions {
		e.Actions[k] += v
	}
	addTotals(e.Buffs, o.Buffs)
	addTotals(e.Nerfs, o.Nerfs)
	addTotals(e.Cleanses, o.Cleanses)
	addTotals(e.Kills.ByVictim, o.Kills.ByVictim)
	for atk, victims := range o.Kills.ByAttacker {
		if e.Kills.ByAttacker[atk] == nil {
			e.Kills.ByAttacker[atk] = newUnitTotals()
		}
		addTotals(e.Kills.ByAttacker[atk], victims)
	}
}

func addTotals(dst, src UnitTotals) {
	for k, v := range src {
		dst[k] += v
	}
}

// ensure fills the maps of an EndStats value loaded from JSON.
func (e *EndStats) ensure() {
	if e.ASurvivors == nil {
		e.ASurvivors = newUnitTotals()
	}
	if e.BSurvivors == nil {
		e.BSurvivors = newUnitTotals()
	}
	for t := game.UnitType(0); t < game.UnitTypeCount; t++ {
		key := t.String()
		if _, ok := e.ASurvivors[key]; !ok {
			e.ASurvivors[key] = 0
		}
		if _, ok := e.BSurvivors[key]; !ok {
			e.BSurvivors[key] = 0
		}
	}
}

// Add merges o into e.
func (e *EndStats) Add(o *EndStats) {
	if o == nil {
		return
	}
	e.ensure()
	o.ensure()
	e.Games += o.Games
	e.Plies += o.Plies
	e.Stalemates += o.Stalemates
	e.SafetyCaps += o.SafetyCaps
	addTotals(e.ASurvivors, o.ASurvivors)
	addTotals(e.BSurvivors, o.BSurvivors)
	e.ATokens += o.ATokens
	e.BTokens += o.BTokens
	e.FirstKillA += o.FirstKillA
	e.FirstKillB += o.FirstKillB
	e.NoFirstKill += o.NoFirstKill
	e.FirstKillPlies += o.FirstKillPlies
	e.FirstWizardKillA += o.FirstWizardKillA
	e.FirstWizardKillB += o.FirstWizardKillB
	e.NoFirstWizardKill += o.NoFirstWizardKill
	e.FirstWizardPlies += o.FirstWizardPlies
	e.FirstWizardWins += o.FirstWizardWins
	e.FirstWizardDecided += o.FirstWizardDecided
	e.FirstKillWins += o.FirstKillWins
	e.FirstKillDecided += o.FirstKillDecided
	for i := range e.PlyBuckets {
		e.PlyBuckets[i] += o.PlyBuckets[i]
		e.AWinByBucket[i] += o.AWinByBucket[i]
		e.BWinByBucket[i] += o.BWinByBucket[i]
		e.DrawByBucket[i] += o.DrawByBucket[i]
	}
}

// Add merges o into p.
func (p *PairEventStats) Add(o *PairEventStats) {
	if o == nil {
		return
	}
	p.Overall.Add(&o.Overall)
	for id, st := range o.ByEngine {
		if p.ByEngine[id] == nil {
			empty := newEventStats()
			p.ByEngine[id] = &empty
		}
		p.ByEngine[id].Add(st)
	}
	if p.BySide == nil {
		p.BySide = newSideEventMap()
	}
	for side, st := range o.BySide {
		if p.BySide[side] == nil {
			empty := newEventStats()
			p.BySide[side] = &empty
		}
		p.BySide[side].Add(st)
	}
}

// killInfo resolves the attacker and victim types of a capture or shot. It is
// shared by the event tally and the first-blood tracking so the two never
// disagree about what counts as a kill.
func killInfo(s *game.State, a game.Action) (attacker, victim game.UnitType, ok bool) {
	if a.Kind != game.ActCapture && a.Kind != game.ActShoot {
		return 0, 0, false
	}
	victimID := a.Target
	if victimID == game.NoPiece || victimID >= game.MaxPieces || !s.Pieces[victimID].Alive {
		victimID = s.Board[a.To]
	}
	if victimID == game.NoPiece || victimID >= game.MaxPieces {
		return 0, 0, false
	}
	v := s.Pieces[victimID]
	atk := s.Pieces[a.Unit]
	if !v.Alive || !atk.Alive {
		return 0, 0, false
	}
	return atk.Type, v.Type, true
}

// collectEvent records the effects of one action. The action has already been
// generated for s, but not applied yet, so s still holds the target pieces.
func collectEvent(s *game.State, a game.Action, st *EventStats) {
	if st == nil {
		return
	}
	st.ensure()
	st.Plies++
	st.Actions[a.Kind.String()]++

	switch a.Kind {
	case game.ActCast:
		if a.Target >= game.MaxPieces {
			return
		}
		t := s.Pieces[a.Target]
		if !t.Alive {
			return
		}
		switch a.Spell {
		case game.SpellBuff:
			st.Buffs[t.Type.String()]++
		case game.SpellNerf:
			st.Nerfs[t.Type.String()]++
		case game.SpellCleanse:
			st.Cleanses[t.Type.String()]++
		}
	case game.ActCapture, game.ActShoot:
		atk, victim, ok := killInfo(s, a)
		if !ok {
			return
		}
		st.Kills.ByVictim[victim.String()]++
		atkKey := atk.String()
		if st.Kills.ByAttacker[atkKey] == nil {
			st.Kills.ByAttacker[atkKey] = newUnitTotals()
		}
		st.Kills.ByAttacker[atkKey][victim.String()]++
	}
}
