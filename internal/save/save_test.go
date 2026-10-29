package save

import (
	"encoding/json"
	"errors"
	"testing"

	"wizardboardgame/internal/game"
	"wizardboardgame/internal/match"
)

func mustSq(t *testing.T, s string) game.Square {
	t.Helper()
	sq, ok := game.ParseSquare(s)
	if !ok {
		t.Fatalf("bad square %q", s)
	}
	return sq
}

func sampleExport(t *testing.T) match.Export {
	t.Helper()
	return match.Export{
		ID:         "abc123",
		Engine:     "alphabeta-r2-v1",
		Difficulty: "strong",
		AI:         [2]bool{false, true},
		Seed:       42,
		Rules:      game.DefaultRules(),
		Log: []match.Entry{
			{Action: game.Action{Kind: game.ActMove, Unit: 0, From: mustSq(t, "b3"), To: mustSq(t, "b4")}},
			{Action: game.Action{Kind: game.ActTurn, Unit: 6, From: mustSq(t, "c2"), Facing: game.NorthWest}},
			{Action: game.Action{Kind: game.ActCast, Unit: 10, From: mustSq(t, "d1"), To: mustSq(t, "c2"), Target: 0, Spell: game.SpellBuff}},
			{Action: game.Action{Kind: game.ActMoveTurn, Unit: 18, From: mustSq(t, "d7"), To: mustSq(t, "e7"), Facing: game.SouthEast}},
			{Action: game.Action{Kind: game.ActPass, Unit: game.NoPiece, From: game.NoSquare, To: game.NoSquare}},
		},
		StateHash: 987654321,
	}
}

func TestEncodeDecodeRoundTrip(t *testing.T) {
	exp := sampleExport(t)
	f := Encode(exp)
	data, err := json.Marshal(f)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got, err := Decode(data)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	back, err := got.ToExport()
	if err != nil {
		t.Fatalf("to export: %v", err)
	}
	if back.ID != exp.ID || back.Engine != exp.Engine || back.AI != exp.AI || back.Seed != exp.Seed {
		t.Fatalf("metadata mismatch: %+v", back)
	}
	if back.StateHash != exp.StateHash {
		t.Fatalf("hash mismatch: %d != %d", back.StateHash, exp.StateHash)
	}
	if len(back.Log) != len(exp.Log) {
		t.Fatalf("log length %d != %d", len(back.Log), len(exp.Log))
	}
	for i := range exp.Log {
		if back.Log[i].Action != exp.Log[i].Action {
			t.Fatalf("action %d mismatch: %+v != %+v", i, back.Log[i].Action, exp.Log[i].Action)
		}
	}
	if len(got.Match.History) != len(exp.Log) {
		t.Fatalf("history length %d != %d", len(got.Match.History), len(exp.Log))
	}
}

func TestDecodeRejectsBadFiles(t *testing.T) {
	if _, err := Decode([]byte(`{`)); !errors.Is(err, match.ErrBadSave) {
		t.Fatalf("bad json: err = %v", err)
	}

	f := Encode(sampleExport(t))
	f.Format = "something-else"
	if _, err := f.ToExport(); !errors.Is(err, match.ErrBadSave) {
		t.Fatalf("bad format: err = %v", err)
	}

	f = Encode(sampleExport(t))
	f.Version = 99
	if _, err := f.ToExport(); !errors.Is(err, match.ErrBadSave) {
		t.Fatalf("bad version: err = %v", err)
	}

	f = Encode(sampleExport(t))
	f.Match.Actions[0].Kind = "teleport-ish"
	if _, err := f.ToExport(); !errors.Is(err, match.ErrBadSave) {
		t.Fatalf("bad action: err = %v", err)
	}
}

func TestStoreWritesFile(t *testing.T) {
	dir := t.TempDir()
	st := NewStore(dir)
	name, err := st.Save(Encode(sampleExport(t)))
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if name == "" || name[len(name)-5:] != ".json" {
		t.Fatalf("unexpected file name %q", name)
	}
	data, err := readFile(dir, name)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if _, err := Decode(data); err != nil {
		t.Fatalf("decode saved file: %v", err)
	}
}
