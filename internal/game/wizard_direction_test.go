package game

import "testing"

// Only targets exactly on one of the 8 rays (straight or diagonal) are
// castable, and the first square along that ray must be empty. Adjacent
// targets have no square to step onto, so they are not castable either.
func TestWizardCastOnlyAlongRays(t *testing.T) {
	center := SqAt(4, 3) // e4
	for f := 0; f < 8; f++ {
		for r := 0; r < 8; r++ {
			target := SqAt(f, r)
			if target == center {
				continue
			}
			dx, dy := f-4, r-3
			aligned := dx == 0 || dy == 0 || absInt(dx) == absInt(dy)
			adjacent := absInt(dx) <= 1 && absInt(dy) <= 1

			p := newTestPos(t)
			p.add(10, PlayerA, UnitWizard, center)
			p.add(12, PlayerB, UnitPawn, target)
			p.turn(PlayerA, 2)

			a, ok := p.findCast(12, SpellNerf)
			want := aligned && !adjacent
			if ok != want {
				t.Errorf("target %s (dx=%d dy=%d): castable=%v, want %v", target, dx, dy, ok, want)
				continue
			}
			if ok {
				wantStep := SqAt(4+signInt(dx), 3+signInt(dy))
				if a.To != wantStep {
					t.Errorf("target %s: step = %s, want %s", target, a.To, wantStep)
				}
			}
		}
	}
}

// Explicit regression: diagonal distance 2 with the in-between square empty.
func TestWizardCastDiagonalTwo(t *testing.T) {
	p := newTestPos(t)
	p.add(10, PlayerA, UnitWizard, sq(t, "d1"))
	p.add(12, PlayerB, UnitPawn, sq(t, "f3"))
	p.turn(PlayerA, 2)

	a, ok := p.findCast(12, SpellNerf)
	if !ok {
		t.Fatal("diagonal distance 2 with an empty e2 must be castable")
	}
	if a.To != sq(t, "e2") {
		t.Fatalf("step = %s, want e2", a.To)
	}
}

// The square directly in the target's direction must be empty.
func TestWizardStepMustBeEmpty(t *testing.T) {
	p := newTestPos(t)
	p.add(10, PlayerA, UnitWizard, sq(t, "d1"))
	p.add(12, PlayerB, UnitPawn, sq(t, "d4"))
	p.add(1, PlayerA, UnitPawn, sq(t, "d2")) // blocks the only step
	p.turn(PlayerA, 2)

	if _, ok := p.findCast(12, SpellNerf); ok {
		t.Fatal("cast must be illegal when the ray square is occupied")
	}
}

// Wizards are not nerfable in the current ruleset.
func TestCannotNerfWizardByDefault(t *testing.T) {
	p := newTestPos(t)
	p.add(10, PlayerA, UnitWizard, sq(t, "a1"))
	p.add(22, PlayerB, UnitWizard, sq(t, "a5"))
	p.turn(PlayerA, 2)

	for _, a := range p.actions() {
		if a.Kind == ActCast && a.Spell == SpellNerf && a.Target == 22 {
			t.Fatal("wizard nerf must not be generated")
		}
	}

	// Even a hand-built action must be rejected by Apply.
	act := Action{Kind: ActCast, Unit: 10, Target: 22, Spell: SpellNerf, To: sq(t, "a2")}
	if _, err := p.s.Apply(act); err == nil {
		t.Fatal("applying a wizard nerf should be illegal")
	}
}

func TestWizardNerfVariant(t *testing.T) {
	p := newTestPos(t)
	p.s.Rules.WizardNerf = true
	p.add(10, PlayerA, UnitWizard, sq(t, "a1"))
	p.add(22, PlayerB, UnitWizard, sq(t, "a5"))
	p.turn(PlayerA, 2)

	if _, ok := p.findCast(22, SpellNerf); !ok {
		t.Fatal("with WizardNerf enabled the nerf should be generated")
	}
}

// Both wizards can cleanse the same ally; the one whose line is blocked still
// has its buff, and cleanse is not limited in any way.
func TestCleanseAvailableFromTheOtherWizard(t *testing.T) {
	p := newTestPos(t)
	p.add(10, PlayerA, UnitWizard, sq(t, "a1"))
	p.add(11, PlayerA, UnitWizard, sq(t, "d1"))
	p.add(0, PlayerA, UnitPawn, sq(t, "a4"))
	p.add(12, PlayerB, UnitPawn, sq(t, "a3")) // blocks wizard 10's line only
	p.add(22, PlayerB, UnitWizard, sq(t, "h8"))
	p.nerf(22, 0)
	p.turn(PlayerA, 2)

	// Wizard a1: step a2, a3 blocks the cleanse line, but buff needs no LOS.
	if _, ok := p.findCastFrom(10, 0, SpellCleanse); ok {
		t.Fatal("wizard a1 must not see through a3")
	}
	if _, ok := p.findCastFrom(10, 0, SpellBuff); !ok {
		t.Fatal("wizard a1 should still be able to buff")
	}

	// Wizard d1: diagonal, clear line through b3.
	if _, ok := p.findCastFrom(11, 0, SpellCleanse); !ok {
		t.Fatal("wizard d1 should be able to cleanse the ally")
	}

	// Cleanse is unlimited: cast it, then nerf the same pawn again and cleanse
	// again with the same wizard on a later turn.
	first, _ := p.findCastFrom(11, 0, SpellCleanse)
	ns := mustPlay(t, p.s, first)
	if ns.IsNerfed(0) {
		t.Fatal("nerf should be removed")
	}
	ns.Nerfs[22] = 0
	ns.Turn = PlayerA
	ns.Next[PlayerA] = 2
	p2 := &testPos{s: ns}
	if _, ok := p2.findCastFrom(11, 0, SpellCleanse); !ok {
		t.Fatal("cleanse must be repeatable")
	}
}
