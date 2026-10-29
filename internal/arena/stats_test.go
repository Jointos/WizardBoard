package arena

import (
	"encoding/json"
	"testing"

	"wizardboardgame/internal/game"
)

func TestCollectEvent(t *testing.T) {
	s := game.New(nil)
	st := newEventStats()

	// A wizard 10 buffs A pawn 0, nerfs B pawn 12, cleanses B cannon 18.
	collectEvent(&s, game.Action{Kind: game.ActCast, Unit: 10, Target: 0, Spell: game.SpellBuff}, &st)
	collectEvent(&s, game.Action{Kind: game.ActCast, Unit: 10, Target: 12, Spell: game.SpellNerf}, &st)
	collectEvent(&s, game.Action{Kind: game.ActCast, Unit: 10, Target: 18, Spell: game.SpellCleanse}, &st)
	// A pawn 0 captures B pawn 12.
	collectEvent(&s, game.Action{Kind: game.ActCapture, Unit: 0, Target: 12}, &st)
	// A cannon 6 shoots B cannon 18 and B wizard 22.
	collectEvent(&s, game.Action{Kind: game.ActShoot, Unit: 6, Target: 18}, &st)
	collectEvent(&s, game.Action{Kind: game.ActShoot, Unit: 6, Target: 22}, &st)

	if st.Buffs["pawn"] != 1 || st.Nerfs["pawn"] != 1 || st.Cleanses["cannon"] != 1 {
		t.Fatalf("spell counts: buffs=%v nerfs=%v cleanses=%v", st.Buffs, st.Nerfs, st.Cleanses)
	}
	if st.Kills.ByVictim["pawn"] != 1 || st.Kills.ByVictim["cannon"] != 1 || st.Kills.ByVictim["wizard"] != 1 {
		t.Fatalf("kills by victim: %v", st.Kills.ByVictim)
	}
	if st.Kills.ByAttacker["pawn"]["pawn"] != 1 {
		t.Fatalf("pawn kills: %v", st.Kills.ByAttacker["pawn"])
	}
	if st.Kills.ByAttacker["cannon"]["cannon"] != 1 || st.Kills.ByAttacker["cannon"]["wizard"] != 1 {
		t.Fatalf("cannon kills: %v", st.Kills.ByAttacker["cannon"])
	}
	if st.Actions["cast"] != 3 || st.Actions["capture"] != 1 || st.Actions["shoot"] != 2 {
		t.Fatalf("actions: %v", st.Actions)
	}
	if st.Plies != 6 {
		t.Fatalf("plies = %d, want 6", st.Plies)
	}

	// JSON uses readable unit keys.
	raw, err := json.Marshal(st)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{`"byVictim"`, `"byAttacker"`, `"pawn"`, `"cannon"`, `"wizard"`} {
		if !contains(string(raw), key) {
			t.Fatalf("JSON missing %s: %s", key, raw)
		}
	}
}

func TestEventStatsAdd(t *testing.T) {
	a := newEventStats()
	b := newEventStats()
	a.Games, a.Plies = 1, 10
	b.Games, b.Plies = 2, 20
	a.Buffs["pawn"] = 1
	b.Buffs["pawn"] = 2
	b.Kills.ByVictim["wizard"] = 1
	b.Kills.ByAttacker["cannon"]["wizard"] = 1

	a.Add(&b)
	if a.Games != 3 || a.Plies != 30 {
		t.Fatalf("games/plies: %d/%d", a.Games, a.Plies)
	}
	if a.Buffs["pawn"] != 3 {
		t.Fatalf("buffs: %v", a.Buffs)
	}
	if a.Kills.ByVictim["wizard"] != 1 || a.Kills.ByAttacker["cannon"]["wizard"] != 1 {
		t.Fatalf("kills: %v / %v", a.Kills.ByVictim, a.Kills.ByAttacker)
	}
}

func TestRunPairCollectsStats(t *testing.T) {
	res, err := RunPair("alphabeta-r2-v1", "ml-r2-v1", 2, GameConfig{MaxNodes: 200, Workers: 2, Seed: 5})
	if err != nil {
		t.Fatal(err)
	}
	if res.Stats == nil {
		t.Fatal("no stats")
	}
	if res.Stats.Overall.Games != 2 {
		t.Fatalf("overall games = %d, want 2", res.Stats.Overall.Games)
	}
	if res.Stats.Overall.Plies != res.Plies {
		t.Fatalf("overall plies = %d, result plies = %d", res.Stats.Overall.Plies, res.Plies)
	}
	for _, id := range []string{"alphabeta-r2-v1", "ml-r2-v1"} {
		es := res.Stats.ByEngine[id]
		if es == nil {
			t.Fatalf("missing stats for %s", id)
		}
		if es.Games != 2 {
			t.Fatalf("%s games = %d, want 2", id, es.Games)
		}
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
