package game

import (
	"math/rand"
	"testing"
)

// ---------- helpers ----------

type testPos struct{ s State }

func newTestPos(t *testing.T) *testPos {
	t.Helper()
	s := New(DefaultRules())
	for i := range s.Board {
		s.Board[i] = NoPiece
	}
	for i := range s.Pieces {
		s.Pieces[i] = Piece{}
	}
	for i := range s.Buffs {
		s.Buffs[i] = NoPiece
		s.Nerfs[i] = NoPiece
	}
	s.Turn = PlayerA
	s.Next = [2]uint8{}
	s.Token = [2]uint8{}
	s.Pending = NoPiece
	s.Over = false
	s.Winner = NoPlayer
	s.Ply = 0
	return &testPos{s: s}
}

func (p *testPos) add(id PieceID, owner Player, typ UnitType, sq Square) *testPos {
	p.s.Pieces[id] = Piece{Alive: true, Owner: owner, Type: typ, Sq: sq}
	p.s.Board[sq] = id
	return p
}

func (p *testPos) face(id PieceID, d Dir) *testPos {
	pc := p.s.Pieces[id]
	pc.Facing = d
	p.s.Pieces[id] = pc
	return p
}

func (p *testPos) turn(pl Player, phase uint8) *testPos {
	p.s.Turn = pl
	p.s.Next[pl] = phase
	return p
}

func (p *testPos) buff(caster, target PieceID) *testPos {
	p.s.Buffs[caster] = target
	return p
}

func (p *testPos) nerf(caster, target PieceID) *testPos {
	p.s.Nerfs[caster] = target
	return p
}

func (p *testPos) actions() []Action { return p.s.LegalActions() }

func (p *testPos) find(kind ActionKind, to Square) (Action, bool) {
	for _, a := range p.s.LegalActions() {
		if a.Kind == kind && a.To == to {
			return a, true
		}
	}
	return Action{}, false
}

func (p *testPos) findCast(target PieceID, spell Spell) (Action, bool) {
	for _, a := range p.s.LegalActions() {
		if a.Kind == ActCast && a.Target == target && a.Spell == spell {
			return a, true
		}
	}
	return Action{}, false
}

func (p *testPos) findCastFrom(unit, target PieceID, spell Spell) (Action, bool) {
	for _, a := range p.s.LegalActions() {
		if a.Kind == ActCast && a.Unit == unit && a.Target == target && a.Spell == spell {
			return a, true
		}
	}
	return Action{}, false
}

func mustPlay(t *testing.T, s State, a Action) State {
	t.Helper()
	ns, err := s.Apply(a)
	if err != nil {
		t.Fatalf("apply %s: %v", s.Describe(a), err)
	}
	return ns
}

func countType(s *State, p Player, typ UnitType) int {
	n := 0
	for id := 0; id < MaxPieces; id++ {
		if s.Pieces[id].Alive && s.Pieces[id].Owner == p && s.Pieces[id].Type == typ {
			n++
		}
	}
	return n
}

// ---------- setup / rotation ----------

func TestSetup(t *testing.T) {
	s := New(nil)
	if got := countType(&s, PlayerA, UnitPawn); got != 6 {
		t.Errorf("A pawns = %d, want 6", got)
	}
	if got := countType(&s, PlayerA, UnitCannon); got != 4 {
		t.Errorf("A cannons = %d, want 4", got)
	}
	if got := countType(&s, PlayerA, UnitWizard); got != 2 {
		t.Errorf("A wizards = %d, want 2", got)
	}
	if got := countType(&s, PlayerB, UnitPawn); got != 6 {
		t.Errorf("B pawns = %d, want 6", got)
	}
	if s.Turn != PlayerA {
		t.Errorf("turn = %v, want A", s.Turn)
	}
	if s.Phase(PlayerA) != UnitPawn {
		t.Errorf("phase = %v, want pawn", s.Phase(PlayerA))
	}
	for _, tc := range []struct {
		sq   string
		want UnitType
		own  Player
	}{
		{"b3", UnitPawn, PlayerA},
		{"g3", UnitPawn, PlayerA},
		{"c2", UnitCannon, PlayerA},
		{"f2", UnitCannon, PlayerA},
		{"d1", UnitWizard, PlayerA},
		{"e1", UnitWizard, PlayerA},
		{"b6", UnitPawn, PlayerB},
		{"g6", UnitPawn, PlayerB},
		{"c7", UnitCannon, PlayerB},
		{"d8", UnitWizard, PlayerB},
	} {
		sq, ok := ParseSquare(tc.sq)
		if !ok {
			t.Fatalf("bad square %s", tc.sq)
		}
		p, ok := s.PieceAt(sq)
		if !ok {
			t.Fatalf("expected piece on %s", tc.sq)
		}
		if p.Type != tc.want || p.Owner != tc.own {
			t.Errorf("%s = %v %v, want %v %v", tc.sq, p.Owner, p.Type, tc.own, tc.want)
		}
	}
}

func TestRoundRobinOrder(t *testing.T) {
	s := New(nil)
	want := []struct {
		p     Player
		phase UnitType
	}{
		{PlayerA, UnitPawn}, {PlayerB, UnitPawn},
		{PlayerA, UnitCannon}, {PlayerB, UnitCannon},
		{PlayerA, UnitWizard}, {PlayerB, UnitWizard},
		{PlayerA, UnitPawn}, {PlayerB, UnitPawn},
	}
	for i, w := range want {
		if s.Turn != w.p {
			t.Fatalf("step %d: turn = %v, want %v", i, s.Turn, w.p)
		}
		if got := s.Phase(s.Turn); got != w.phase {
			t.Fatalf("step %d: phase = %v, want %v", i, got, w.phase)
		}
		acts := s.LegalActions()
		if len(acts) == 0 {
			t.Fatalf("step %d: no legal actions", i)
		}
		var next State
		var err error
		next, err = s.Apply(acts[0])
		if err != nil {
			t.Fatalf("step %d: %v", i, err)
		}
		s = next
	}
}

func TestRotationSkipsExhaustedType(t *testing.T) {
	p := newTestPos(t)
	p.add(10, PlayerA, UnitWizard, sq(t, "d1"))
	p.add(6, PlayerA, UnitCannon, sq(t, "c2"))
	p.add(22, PlayerB, UnitWizard, sq(t, "d8"))
	p.add(12, PlayerB, UnitPawn, sq(t, "b6"))
	p.turn(PlayerA, 0)

	// A owns no pawns, so the cursor must resolve to the cannon.
	if got := p.s.Phase(PlayerA); got != UnitCannon {
		t.Fatalf("A phase = %v, want cannon", got)
	}
	acts := p.actions()
	if len(acts) == 0 || acts[0].Kind == ActPass {
		t.Fatalf("expected cannon actions, got %v", acts)
	}
	s1 := mustPlay(t, p.s, acts[0])
	if s1.Turn != PlayerB {
		t.Fatalf("turn = %v, want B", s1.Turn)
	}
	if got := s1.Phase(PlayerB); got != UnitPawn {
		t.Fatalf("B phase = %v, want pawn", got)
	}
	var pawnMove Action
	for _, a := range s1.LegalActions() {
		if a.Kind == ActMove {
			pawnMove = a
			break
		}
	}
	if pawnMove.Kind != ActMove {
		t.Fatal("expected a pawn move for B")
	}
	s2 := mustPlay(t, s1, pawnMove)
	if s2.Turn != PlayerA {
		t.Fatalf("turn = %v, want A", s2.Turn)
	}
	// A used cannon, so the next phase is the wizard.
	if got := s2.Phase(PlayerA); got != UnitWizard {
		t.Fatalf("A phase = %v, want wizard", got)
	}
}

func sq(t *testing.T, name string) Square {
	t.Helper()
	v, ok := ParseSquare(name)
	if !ok {
		t.Fatalf("bad square %q", name)
	}
	return v
}

// ---------- pawn ----------

func TestPawnMovesLikeKingButCapturesRestricted(t *testing.T) {
	p := newTestPos(t)
	p.add(0, PlayerA, UnitPawn, sq(t, "d4"))
	p.add(10, PlayerA, UnitWizard, sq(t, "a1"))
	p.add(22, PlayerB, UnitWizard, sq(t, "h8"))
	p.add(12, PlayerB, UnitPawn, sq(t, "d5")) // directly in front
	p.add(13, PlayerB, UnitPawn, sq(t, "e4")) // sideways
	p.add(14, PlayerB, UnitPawn, sq(t, "d3")) // behind
	p.turn(PlayerA, 0)

	acts := p.actions()
	var moves, captures int
	for _, a := range acts {
		switch a.Kind {
		case ActMove:
			moves++
		case ActCapture:
			captures++
			if a.To != sq(t, "d3") {
				t.Errorf("unexpected capture at %s (only backward should be legal on action 1)", a.To)
			}
		}
	}
	if moves != 5 { // 8 neighbours minus 3 occupied
		t.Errorf("moves = %d, want 5", moves)
	}
	if captures != 1 {
		t.Errorf("captures = %d, want 1 (backward)", captures)
	}
}

func TestPawnForwardCaptureEveryThirdAction(t *testing.T) {
	p := newTestPos(t)
	p.add(0, PlayerA, UnitPawn, sq(t, "d4"))
	p.add(10, PlayerA, UnitWizard, sq(t, "a1"))
	p.add(22, PlayerB, UnitWizard, sq(t, "h8"))
	p.add(12, PlayerB, UnitPawn, sq(t, "d5"))
	p.turn(PlayerA, 0)

	for i := 0; i < 4; i++ {
		p.s.Pieces[0].Acts = uint8(i)
		_, hasFwd := p.find(ActCapture, sq(t, "d5"))
		want := i == 2 || i == 5
		if hasFwd != want {
			t.Fatalf("after %d actions: forward capture = %v, want %v", i, hasFwd, want)
		}
	}
}

func TestPawnSidewaysCaptureNeedsBuff(t *testing.T) {
	p := newTestPos(t)
	p.add(0, PlayerA, UnitPawn, sq(t, "d4"))
	p.add(10, PlayerA, UnitWizard, sq(t, "a1"))
	p.add(22, PlayerB, UnitWizard, sq(t, "h8"))
	p.add(13, PlayerB, UnitPawn, sq(t, "e4"))
	p.turn(PlayerA, 0)

	if _, ok := p.find(ActCapture, sq(t, "e4")); ok {
		t.Fatal("sideways capture must not be legal without a buff")
	}
	p.buff(10, 0)
	if _, ok := p.find(ActCapture, sq(t, "e4")); !ok {
		t.Fatal("sideways capture must be legal with a buff")
	}
}

func TestNerfedPawnMustRetreat(t *testing.T) {
	p := newTestPos(t)
	p.add(0, PlayerA, UnitPawn, sq(t, "d4"))
	p.add(10, PlayerA, UnitWizard, sq(t, "a1"))
	p.add(22, PlayerB, UnitWizard, sq(t, "h8"))
	p.nerf(22, 0)
	p.turn(PlayerA, 0)

	for _, a := range p.actions() {
		if a.Kind != ActMove {
			continue
		}
		if a.To != sq(t, "c3") && a.To != sq(t, "d3") && a.To != sq(t, "e3") {
			t.Fatalf("nerfed pawn produced non-retreat move to %s", a.To)
		}
	}
}

// ---------- cannon ----------

func TestCannonShootsExactlyTwoSquaresOverPieces(t *testing.T) {
	p := newTestPos(t)
	p.add(6, PlayerA, UnitCannon, sq(t, "c2"))
	p.face(6, North)
	p.add(10, PlayerA, UnitWizard, sq(t, "a1"))
	p.add(22, PlayerB, UnitWizard, sq(t, "h8"))
	p.add(12, PlayerB, UnitPawn, sq(t, "c4"))
	p.add(13, PlayerB, UnitPawn, sq(t, "c3")) // blocker, must be ignored
	p.add(14, PlayerB, UnitPawn, sq(t, "c5"))
	p.turn(PlayerA, 1)

	if _, ok := p.find(ActShoot, sq(t, "c4")); !ok {
		t.Fatal("expected shot at c4")
	}
	if _, ok := p.find(ActShoot, sq(t, "c5")); ok {
		t.Fatal("range 3 must not be legal without a buff")
	}
	p.buff(10, 6)
	if _, ok := p.find(ActShoot, sq(t, "c5")); !ok {
		t.Fatal("range 3 must be legal with a buff")
	}
}

func TestNerfedCannonCannotTurn(t *testing.T) {
	p := newTestPos(t)
	p.add(6, PlayerA, UnitCannon, sq(t, "c2"))
	p.face(6, North)
	p.add(10, PlayerA, UnitWizard, sq(t, "a1"))
	p.add(22, PlayerB, UnitWizard, sq(t, "h8"))
	p.nerf(22, 6)
	p.turn(PlayerA, 1)

	for _, a := range p.actions() {
		if a.Kind == ActTurn || a.Kind == ActMoveTurn {
			t.Fatalf("nerfed cannon produced a turning action: %v", a)
		}
	}
}

// ---------- wizard ----------

func TestWizardStepsTowardTarget(t *testing.T) {
	p := newTestPos(t)
	p.add(10, PlayerA, UnitWizard, sq(t, "d1"))
	p.add(0, PlayerA, UnitPawn, sq(t, "d4"))
	p.add(22, PlayerB, UnitWizard, sq(t, "h8"))
	p.turn(PlayerA, 2)

	a, ok := p.findCast(0, SpellBuff)
	if !ok {
		t.Fatal("expected buff action")
	}
	if a.To != sq(t, "d2") {
		t.Fatalf("step = %s, want d2", a.To)
	}
	ns := mustPlay(t, p.s, a)
	if ns.Pieces[10].Sq != sq(t, "d2") {
		t.Fatalf("wizard at %s, want d2", ns.Pieces[10].Sq)
	}
	if !ns.IsBuffed(0) {
		t.Fatal("pawn should be buffed")
	}
}

func TestWizardCannotCastOnAdjacentTarget(t *testing.T) {
	p := newTestPos(t)
	p.add(10, PlayerA, UnitWizard, sq(t, "d1"))
	p.add(0, PlayerA, UnitPawn, sq(t, "e2"))
	p.add(22, PlayerB, UnitWizard, sq(t, "h8"))
	p.turn(PlayerA, 2)

	if _, ok := p.findCast(0, SpellBuff); ok {
		t.Fatal("casting needs an empty square to step onto; adjacent targets are not castable")
	}
}

func TestWizardCannotCastThroughBlockedStep(t *testing.T) {
	p := newTestPos(t)
	p.add(10, PlayerA, UnitWizard, sq(t, "d1"))
	p.add(0, PlayerA, UnitPawn, sq(t, "d4"))
	p.add(1, PlayerA, UnitPawn, sq(t, "d2")) // blocks the ray
	p.add(22, PlayerB, UnitWizard, sq(t, "h8"))
	p.turn(PlayerA, 2)

	if _, ok := p.findCast(0, SpellBuff); ok {
		t.Fatal("cast must be illegal when the step square is blocked")
	}
}

func TestBuffNeedsNoLOSButCleanseDoes(t *testing.T) {
	p := newTestPos(t)
	p.add(10, PlayerA, UnitWizard, sq(t, "d1"))
	p.add(0, PlayerA, UnitPawn, sq(t, "d4"))
	p.add(12, PlayerB, UnitPawn, sq(t, "d3")) // blocks the d2 -> d4 line
	p.add(13, PlayerB, UnitPawn, sq(t, "e3")) // blocks the e2 -> d4 line
	p.add(22, PlayerB, UnitWizard, sq(t, "h8"))
	p.nerf(22, 0)
	p.turn(PlayerA, 2)

	if _, ok := p.findCast(0, SpellBuff); !ok {
		t.Fatal("buff must not require line of sight")
	}
	if _, ok := p.findCast(0, SpellCleanse); ok {
		t.Fatal("cleanse must require line of sight")
	}
}

func TestWizardCannotTargetItself(t *testing.T) {
	p := newTestPos(t)
	p.add(10, PlayerA, UnitWizard, sq(t, "d1"))
	p.add(22, PlayerB, UnitWizard, sq(t, "h8"))
	p.turn(PlayerA, 2)
	for _, a := range p.actions() {
		if a.Kind == ActCast && a.Target == 10 {
			t.Fatal("wizard must not be able to cast on itself")
		}
	}
}

func TestBuffRemovedWhenWizardDies(t *testing.T) {
	p := newTestPos(t)
	p.add(10, PlayerA, UnitWizard, sq(t, "d1"))
	p.add(0, PlayerA, UnitPawn, sq(t, "c3"))
	p.add(22, PlayerB, UnitWizard, sq(t, "h8"))
	p.buff(10, 0)
	if !p.s.IsBuffed(0) {
		t.Fatal("precondition: pawn buffed")
	}
	p.s.kill(10)
	if p.s.IsBuffed(0) {
		t.Fatal("buff must be removed when the caster dies")
	}
}

func TestCleanseRemovesNerfFromAlly(t *testing.T) {
	p := newTestPos(t)
	p.add(10, PlayerA, UnitWizard, sq(t, "a1"))
	p.add(0, PlayerA, UnitPawn, sq(t, "a3"))
	p.add(22, PlayerB, UnitWizard, sq(t, "h8"))
	p.nerf(22, 0)
	p.turn(PlayerA, 2)

	a, ok := p.findCast(0, SpellCleanse)
	if !ok {
		t.Fatal("expected cleanse on the nerfed ally")
	}
	if a.To != sq(t, "a2") {
		t.Fatalf("step = %s, want a2", a.To)
	}
	ns := mustPlay(t, p.s, a)
	if ns.IsNerfed(0) {
		t.Fatal("nerf should be gone")
	}
}

func TestCleanseRemovesBuffFromEnemy(t *testing.T) {
	p := newTestPos(t)
	p.add(10, PlayerA, UnitWizard, sq(t, "a1"))
	p.add(22, PlayerB, UnitWizard, sq(t, "h8"))
	p.add(12, PlayerB, UnitPawn, sq(t, "a5"))
	p.buff(22, 12)
	p.turn(PlayerA, 2)

	a, ok := p.findCast(12, SpellCleanse)
	if !ok {
		t.Fatal("expected cleanse on the buffed enemy")
	}
	if a.To != sq(t, "a2") {
		t.Fatalf("step = %s, want a2", a.To)
	}
	ns := mustPlay(t, p.s, a)
	if ns.IsBuffed(12) {
		t.Fatal("enemy buff should be gone")
	}
}

func TestBuffedWizardGetsFreeMove(t *testing.T) {
	p := newTestPos(t)
	p.add(10, PlayerA, UnitWizard, sq(t, "d1"))
	p.add(11, PlayerA, UnitWizard, sq(t, "b1"))
	p.add(0, PlayerA, UnitPawn, sq(t, "b3"))
	p.add(22, PlayerB, UnitWizard, sq(t, "h8"))
	p.buff(10, 11) // wizard 10 buffs wizard 11
	p.turn(PlayerA, 2)

	var cast Action
	found := false
	for _, act := range p.actions() {
		if act.Kind == ActCast && act.Unit == 11 {
			cast, found = act, true
			break
		}
	}
	if !found {
		t.Fatal("expected wizard 11 to be able to cast")
	}
	ns := mustPlay(t, p.s, cast)
	if ns.Pending != 11 {
		t.Fatalf("pending = %v, want 11", ns.Pending)
	}
	acts := ns.LegalActions()
	if len(acts) == 0 {
		t.Fatal("expected free move actions")
	}
	stay := acts[0]
	if stay.Kind != ActFreeMove || stay.To != NoSquare {
		t.Fatalf("first action = %v, want stay", stay)
	}
	ns2 := mustPlay(t, ns, stay)
	if ns2.Pending != NoPiece || ns2.Turn != PlayerB {
		t.Fatalf("free move did not end the turn: pending=%v turn=%v", ns2.Pending, ns2.Turn)
	}
}

// ---------- game over / purity ----------

func TestOneWizardDeathDoesNotEndGame(t *testing.T) {
	p := newTestPos(t)
	p.add(6, PlayerB, UnitCannon, sq(t, "d6"))
	p.face(6, South)
	p.add(10, PlayerA, UnitWizard, sq(t, "d4"))
	p.add(11, PlayerA, UnitWizard, sq(t, "e1"))
	p.add(22, PlayerB, UnitWizard, sq(t, "h8"))
	p.turn(PlayerB, 1)

	shot, ok := p.find(ActShoot, sq(t, "d4"))
	if !ok {
		t.Fatal("expected shot at d4")
	}
	s1 := mustPlay(t, p.s, shot)
	if s1.Pieces[10].Alive {
		t.Fatal("wizard 10 should be dead")
	}
	if s1.Over {
		t.Fatal("game must not end while A still has a wizard")
	}
}

func TestGameEndsWhenBothWizardsDie(t *testing.T) {
	p := newTestPos(t)
	p.add(6, PlayerB, UnitCannon, sq(t, "d6"))
	p.face(6, South)
	p.add(10, PlayerA, UnitWizard, sq(t, "d4"))
	p.add(22, PlayerB, UnitWizard, sq(t, "h8"))
	p.turn(PlayerB, 1)

	shot, ok := p.find(ActShoot, sq(t, "d4"))
	if !ok {
		t.Fatal("expected shot at d4")
	}
	s1 := mustPlay(t, p.s, shot)
	if !s1.Over {
		t.Fatal("game should be over after A's last wizard died")
	}
	if s1.Winner != PlayerB {
		t.Fatalf("winner = %v, want B", s1.Winner)
	}
	if len(s1.LegalActions()) != 0 {
		t.Fatal("no actions may be legal after the game ended")
	}
}

func TestApplyDoesNotMutateOriginal(t *testing.T) {
	s := New(nil)
	before := s
	h := s.Hash()
	acts := s.LegalActions()
	for i := 0; i < 50 && len(acts) > 0; i++ {
		if _, err := s.Apply(acts[0]); err != nil {
			t.Fatalf("apply: %v", err)
		}
		acts = s.LegalActions()
	}
	if s.Hash() != h {
		t.Fatal("Apply mutated the receiver")
	}
	if !s.Equal(&before) {
		t.Fatal("Apply mutated the receiver")
	}
}

func TestReplayIsDeterministic(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	var states []State
	s := New(nil)
	states = append(states, s)
	for i := 0; i < 400 && !s.Over; i++ {
		acts := s.LegalActions()
		if len(acts) == 0 {
			break
		}
		a := acts[rng.Intn(len(acts))]
		var err error
		s, err = s.Apply(a)
		if err != nil {
			t.Fatalf("apply: %v", err)
		}
		states = append(states, s)
	}
	final := s.Hash()
	replay := New(nil)
	for i := 1; i < len(states); i++ {
		acts := replay.LegalActions()
		matched := false
		for _, a := range acts {
			ns, err := replay.Apply(a)
			if err == nil && ns.Hash() == states[i].Hash() {
				replay = ns
				matched = true
				break
			}
		}
		if !matched {
			t.Fatalf("could not replay ply %d", i)
		}
	}
	if replay.Hash() != final {
		t.Fatal("replay diverged")
	}
}

func TestRandomGamesKeepInvariants(t *testing.T) {
	for seed := int64(0); seed < 100; seed++ {
		rng := rand.New(rand.NewSource(seed))
		s := New(nil)
		for ply := 0; ply < 1200 && !s.Over; ply++ {
			acts := s.LegalActions()
			if len(acts) == 0 {
				t.Fatalf("seed %d ply %d: no legal actions but game not over", seed, ply)
			}
			a := acts[rng.Intn(len(acts))]
			ns, err := s.Apply(a)
			if err != nil {
				t.Fatalf("seed %d ply %d: apply %v: %v", seed, ply, a, err)
			}
			s = ns
			checkInvariants(t, &s, seed, ply)
		}
		if s.Ply > 0 && !s.Over && s.Ply < s.Rules.MaxPly {
			t.Fatalf("seed %d: game stalled at ply %d", seed, s.Ply)
		}
	}
}

func checkInvariants(t *testing.T, s *State, seed int64, ply int) {
	t.Helper()
	seen := map[Square]PieceID{}
	for id := 0; id < MaxPieces; id++ {
		p := &s.Pieces[id]
		if !p.Alive {
			continue
		}
		if prev, ok := seen[p.Sq]; ok {
			t.Fatalf("seed %d ply %d: pieces %d and %d share square %s", seed, ply, prev, id, p.Sq)
		}
		seen[p.Sq] = PieceID(id)
		if s.Board[p.Sq] != PieceID(id) {
			t.Fatalf("seed %d ply %d: board mismatch on %s", seed, ply, p.Sq)
		}
	}
	for sq := Square(0); sq < NumSquares; sq++ {
		id := s.Board[sq]
		if id == NoPiece {
			continue
		}
		if id >= MaxPieces || !s.Pieces[id].Alive {
			t.Fatalf("seed %d ply %d: board holds dead piece %d on %s", seed, ply, id, sq)
		}
	}
	for i := 0; i < MaxPieces; i++ {
		if s.Buffs[i] == NoPiece {
			continue
		}
		caster := &s.Pieces[i]
		target := s.Buffs[i]
		if !caster.Alive || caster.Type != UnitWizard {
			t.Fatalf("seed %d ply %d: buff cast by non-wizard", seed, ply)
		}
		if target >= MaxPieces || !s.Pieces[target].Alive || s.Pieces[target].Owner != caster.Owner {
			t.Fatalf("seed %d ply %d: invalid buff target", seed, ply)
		}
	}
}

func TestTeleportCostsFourTokens(t *testing.T) {
	p := newTestPos(t)
	p.add(10, PlayerA, UnitWizard, sq(t, "d1"))
	p.add(22, PlayerB, UnitWizard, sq(t, "h8"))
	p.turn(PlayerA, 2)

	p.s.Token[PlayerA] = 3
	for _, a := range p.actions() {
		if a.Kind == ActTeleport {
			t.Fatal("3 tokens must not be enough for a teleport")
		}
	}

	p.s.Token[PlayerA] = 4
	var tp Action
	found := false
	for _, a := range p.actions() {
		if a.Kind == ActTeleport {
			tp, found = a, true
			break
		}
	}
	if !found {
		t.Fatal("4 tokens must allow a teleport")
	}
	ns := mustPlay(t, p.s, tp)
	if ns.Token[PlayerA] != 0 {
		t.Fatalf("tokens = %d, want 0 after teleport", ns.Token[PlayerA])
	}
	if ns.Pieces[10].Sq != tp.To {
		t.Fatalf("wizard at %s, want %s", ns.Pieces[10].Sq, tp.To)
	}
}

func TestWizardSwapIsLegalAndSwapsPieces(t *testing.T) {
	p := newTestPos(t)
	p.add(10, PlayerA, UnitWizard, sq(t, "d1"))
	p.add(0, PlayerA, UnitPawn, sq(t, "e2"))
	p.add(22, PlayerB, UnitWizard, sq(t, "h8"))
	p.turn(PlayerA, 2)

	sw, ok := p.find(ActSwap, sq(t, "e2"))
	if !ok {
		t.Fatal("adjacent friendly pawn must allow a swap")
	}
	if sw.Unit != 10 || sw.Target != 0 || sw.From != sq(t, "d1") {
		t.Fatalf("swap = %+v, want wizard d1 with pawn e2", sw)
	}

	ns := mustPlay(t, p.s, sw)
	if ns.Pieces[10].Sq != sq(t, "e2") || ns.Pieces[0].Sq != sq(t, "d1") {
		t.Fatalf("after swap wizard=%s pawn=%s, want e2/d1", ns.Pieces[10].Sq, ns.Pieces[0].Sq)
	}
	if ns.Board[sq(t, "e2")] != 10 || ns.Board[sq(t, "d1")] != 0 {
		t.Fatal("board must follow the swapped pieces")
	}
	if ns.Turn != PlayerB || ns.Ply != 1 {
		t.Fatalf("turn=%v ply=%d, want PlayerB ply 1", ns.Turn, ns.Ply)
	}
}

func TestWizardSwapRequiresAdjacentFriendly(t *testing.T) {
	p := newTestPos(t)
	p.add(10, PlayerA, UnitWizard, sq(t, "d1"))
	p.add(5, PlayerB, UnitCannon, sq(t, "e2")) // adjacent but enemy
	p.add(1, PlayerA, UnitPawn, sq(t, "d4"))   // friendly but two squares away
	p.add(22, PlayerB, UnitWizard, sq(t, "h8"))
	p.turn(PlayerA, 2)

	for _, a := range p.actions() {
		if a.Kind == ActSwap {
			t.Fatalf("unexpected swap %+v: only adjacent friendly units may swap", a)
		}
	}
}

func TestWizardSwapCanTargetOtherWizard(t *testing.T) {
	p := newTestPos(t)
	p.add(10, PlayerA, UnitWizard, sq(t, "d1"))
	p.add(11, PlayerA, UnitWizard, sq(t, "e2"))
	p.add(22, PlayerB, UnitWizard, sq(t, "h8"))
	p.turn(PlayerA, 2)

	sw, ok := p.find(ActSwap, sq(t, "e2"))
	if !ok || sw.Target != 11 {
		t.Fatalf("wizard-to-wizard swap missing (action=%+v ok=%v)", sw, ok)
	}
	ns := mustPlay(t, p.s, sw)
	if ns.Pieces[10].Sq != sq(t, "e2") || ns.Pieces[11].Sq != sq(t, "d1") {
		t.Fatal("the two wizards must exchange squares")
	}
}

func TestWizardSwapKeepsPieceEffects(t *testing.T) {
	p := newTestPos(t)
	p.add(10, PlayerA, UnitWizard, sq(t, "d1"))
	p.add(0, PlayerA, UnitPawn, sq(t, "e2"))
	p.add(11, PlayerA, UnitWizard, sq(t, "h1"))
	p.add(22, PlayerB, UnitWizard, sq(t, "h8"))
	p.buff(11, 0) // the second wizard buffs the pawn
	p.turn(PlayerA, 2)

	sw, ok := p.find(ActSwap, sq(t, "e2"))
	if !ok {
		t.Fatal("swap not generated")
	}
	ns := mustPlay(t, p.s, sw)
	if !ns.IsBuffed(0) {
		t.Fatal("the buff must travel with the pawn, not stay on the square")
	}
	if ns.BuffCaster(0) != 11 {
		t.Fatalf("buff caster = %d, want 11", ns.BuffCaster(0))
	}
}

func TestWizardSwapDisabledByRule(t *testing.T) {
	r := DefaultRules()
	r.WizardSwap = false
	p := &testPos{s: New(r)}
	for i := range p.s.Board {
		p.s.Board[i] = NoPiece
	}
	for i := range p.s.Pieces {
		p.s.Pieces[i] = Piece{}
	}
	p.add(10, PlayerA, UnitWizard, sq(t, "d1"))
	p.add(0, PlayerA, UnitPawn, sq(t, "e2"))
	p.add(22, PlayerB, UnitWizard, sq(t, "h8"))
	p.turn(PlayerA, 2)

	for _, a := range p.actions() {
		if a.Kind == ActSwap {
			t.Fatal("WizardSwap=false must not generate swaps")
		}
	}
	_, err := p.s.Apply(Action{Kind: ActSwap, Unit: 10, From: sq(t, "d1"), To: sq(t, "e2"), Target: 0})
	if err == nil {
		t.Fatal("WizardSwap=false must reject a swap")
	}
}

func TestActionsCarryNoStaleFields(t *testing.T) {
	rng := rand.New(rand.NewSource(11))
	s := New(nil)
	for ply := 0; ply < 300 && !s.Over; ply++ {
		acts := s.LegalActions()
		for _, a := range acts {
			switch a.Kind {
			case ActTurn, ActSkip, ActPass:
				if a.To != NoSquare || a.Target != NoPiece {
					t.Fatalf("ply %d: %v action carries to=%v target=%v", ply, a.Kind, a.To, a.Target)
				}
			case ActMove, ActMoveTurn, ActTeleport:
				if a.To == NoSquare || a.Target != NoPiece {
					t.Fatalf("ply %d: %v action carries to=%v target=%v", ply, a.Kind, a.To, a.Target)
				}
				if s.Board[a.To] != NoPiece {
					t.Fatalf("ply %d: %v should target an empty square", ply, a.Kind)
				}
			case ActFreeMove:
				if a.Target != NoPiece {
					t.Fatalf("ply %d: freemove carries target=%v", ply, a.Target)
				}
				if a.To != NoSquare && s.Board[a.To] != NoPiece {
					t.Fatalf("ply %d: freemove should target an empty square", ply)
				}
			case ActSwap:
				if a.Target == NoPiece || a.Target >= MaxPieces {
					t.Fatalf("ply %d: swap has no target", ply)
				}
				if !s.Pieces[a.Target].Alive || s.Pieces[a.Target].Owner != s.Pieces[a.Unit].Owner {
					t.Fatalf("ply %d: swap must target a living friendly piece", ply)
				}
				if a.To != s.Pieces[a.Target].Sq || a.From != s.Pieces[a.Unit].Sq {
					t.Fatalf("ply %d: swap squares do not match the pieces", ply)
				}
			case ActCapture, ActShoot, ActCast:
				if a.Target == NoPiece || a.Target >= MaxPieces {
					t.Fatalf("ply %d: %v action has no target", ply, a.Kind)
				}
				if !s.Pieces[a.Target].Alive {
					t.Fatalf("ply %d: %v action targets a dead piece", ply, a.Kind)
				}
			}
		}
		s, _ = s.Apply(acts[rng.Intn(len(acts))])
	}
}

func perft(s *State, depth int) int {
	if depth == 0 || s.Over {
		return 1
	}
	acts := s.LegalActions()
	total := 0
	for _, a := range acts {
		ns, err := s.Apply(a)
		if err != nil {
			panic(err)
		}
		total += perft(&ns, depth-1)
	}
	return total
}

func TestPerft(t *testing.T) {
	s := New(nil)
	want := map[int]int{1: 26, 2: 676, 3: 70928}
	for depth := 1; depth <= 3; depth++ {
		got := perft(&s, depth)
		t.Logf("perft(%d) = %d", depth, got)
		if want[depth] != 0 && got != want[depth] {
			t.Errorf("perft(%d) = %d, want %d", depth, got, want[depth])
		}
	}
}

func BenchmarkActions(b *testing.B) {
	s := New(nil)
	buf := make([]Action, 0, 256)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		buf = s.Actions(buf[:0])
	}
}

func BenchmarkApply(b *testing.B) {
	s := New(nil)
	buf := make([]Action, 0, 256)
	buf = s.Actions(buf[:0])
	a := buf[0]
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		var ns State
		if err := s.ApplyInto(&ns, a); err != nil {
			b.Fatal(err)
		}
	}
}

func TestEncodeShape(t *testing.T) {
	s := New(nil)
	out := s.Encode(nil)
	if len(out) != EncodePlanes*NumSquares+EncodeScalars {
		t.Fatalf("encode len = %d", len(out))
	}
	nonZero := 0
	for _, v := range out {
		if v != 0 {
			nonZero++
		}
	}
	if nonZero == 0 {
		t.Fatal("encoding is empty")
	}
}
