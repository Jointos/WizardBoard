package match

import (
	"errors"
	"testing"

	"wizardboardgame/internal/game"
)

func TestPlayAndUndo(t *testing.T) {
	svc := NewService(nil)
	snap := svc.Create(Config{})
	if len(snap.Legal) == 0 {
		t.Fatal("no legal actions at start")
	}
	if snap.CanUndo {
		t.Fatal("fresh match must not be undoable")
	}
	first := snap.Legal[0]
	played, err := svc.Play(snap.ID, 0)
	if err != nil {
		t.Fatalf("play: %v", err)
	}
	if len(played.Log) != 1 {
		t.Fatalf("log length = %d, want 1", len(played.Log))
	}
	if played.State.Hash() == snap.State.Hash() {
		t.Fatal("state did not change")
	}
	if !played.CanUndo {
		t.Fatal("match should be undoable after a move")
	}
	undone, err := svc.Undo(snap.ID)
	if err != nil {
		t.Fatalf("undo: %v", err)
	}
	if undone.State.Hash() != snap.State.Hash() {
		t.Fatal("undo did not restore the previous position")
	}
	if len(undone.Log) != 0 {
		t.Fatalf("undo should leave an empty log, got %d entries", len(undone.Log))
	}
	if first.Kind != snap.Legal[0].Kind {
		t.Fatal("sanity check failed")
	}
}

func TestPlayRejectsOutOfRange(t *testing.T) {
	svc := NewService(nil)
	snap := svc.Create(Config{})
	if _, err := svc.Play(snap.ID, 9999); err != ErrBadAction {
		t.Fatalf("err = %v, want ErrBadAction", err)
	}
	if _, err := svc.Play("nope", 0); err != ErrNotFound {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestUndoOnEmptyMatch(t *testing.T) {
	svc := NewService(nil)
	snap := svc.Create(Config{})
	if _, err := svc.Undo(snap.ID); err != ErrNothingToUndo {
		t.Fatalf("err = %v, want ErrNothingToUndo", err)
	}
}

func TestRandomPlaysDoNotStall(t *testing.T) {
	svc := NewService(nil)
	snap := svc.Create(Config{})
	for i := 0; i < 500 && !snap.State.Over; i++ {
		next, err := svc.PlayRandom(snap.ID)
		if err != nil {
			t.Fatalf("ply %d: %v", i, err)
		}
		snap = next
	}
	if !snap.State.Over && snap.State.Ply < snap.State.Rules.MaxPly {
		t.Fatalf("game stalled at ply %d", snap.State.Ply)
	}
}

func TestReplayFromLog(t *testing.T) {
	svc := NewService(nil)
	snap := svc.Create(Config{})
	for i := 0; i < 120 && !snap.State.Over; i++ {
		next, err := svc.PlayRandom(snap.ID)
		if err != nil {
			t.Fatalf("ply %d: %v", i, err)
		}
		snap = next
	}
	replay := game.New(nil)
	for i, e := range snap.Log {
		ns, err := replay.Apply(e.Action)
		if err != nil {
			t.Fatalf("replay ply %d: %v", i, err)
		}
		replay = ns
	}
	if replay.Hash() != snap.State.Hash() {
		t.Fatal("replay diverged from the live state")
	}
}

func TestAIFlow(t *testing.T) {
	svc := NewService(nil)
	snap := svc.Create(Config{AI: [2]bool{false, true}, EngineID: "alphabeta-r2-v1", Difficulty: "fast"})
	if snap.PendingAI {
		t.Fatal("A starts, so the AI should not be pending")
	}
	if _, err := svc.PlayAI(snap.ID); err != nil {
		t.Fatalf("idempotent ai call: %v", err)
	}
	if _, err := svc.Play(snap.ID, 0); err != nil {
		t.Fatalf("human move: %v", err)
	}
	got, err := svc.Get(snap.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if !got.PendingAI {
		t.Fatal("after A's move the AI should be pending")
	}
	if _, err := svc.Play(snap.ID, 0); !errors.Is(err, ErrAITurn) {
		t.Fatalf("human move on AI turn: err = %v, want ErrAITurn", err)
	}
	after, err := svc.PlayAI(snap.ID)
	if err != nil {
		t.Fatalf("ai move: %v", err)
	}
	if after.PendingAI {
		t.Fatal("after the AI reply it should be the human's turn")
	}
	if len(after.Log) != 2 {
		t.Fatalf("log length = %d, want 2", len(after.Log))
	}
	if after.State.Turn != game.PlayerA {
		t.Fatalf("turn = %v, want A", after.State.Turn)
	}
}

func TestUndoWhileAIIsPending(t *testing.T) {
	svc := NewService(nil)
	snap := svc.Create(Config{AI: [2]bool{false, true}, EngineID: "alphabeta-r2-v1", Difficulty: "fast"})
	if _, err := svc.Play(snap.ID, 0); err != nil {
		t.Fatalf("human move: %v", err)
	}
	pending, _ := svc.Get(snap.ID)
	if !pending.PendingAI {
		t.Fatal("precondition: AI should be pending")
	}
	u, err := svc.Undo(snap.ID)
	if err != nil {
		t.Fatalf("undo: %v", err)
	}
	if len(u.Log) != 0 {
		t.Fatalf("log length = %d, want 0", len(u.Log))
	}
	if u.State.Turn != game.PlayerA {
		t.Fatalf("turn = %v, want A", u.State.Turn)
	}
	if u.PendingAI {
		t.Fatal("no AI should be pending after undo")
	}
}

func TestUndoAfterAIReplyRemovesBoth(t *testing.T) {
	svc := NewService(nil)
	snap := svc.Create(Config{AI: [2]bool{false, true}, EngineID: "alphabeta-r2-v1", Difficulty: "fast"})
	if _, err := svc.Play(snap.ID, 0); err != nil {
		t.Fatalf("human move: %v", err)
	}
	afterAI, err := svc.PlayAI(snap.ID)
	if err != nil {
		t.Fatalf("ai move: %v", err)
	}
	if len(afterAI.Log) != 2 {
		t.Fatalf("log length = %d, want 2", len(afterAI.Log))
	}
	u, err := svc.Undo(snap.ID)
	if err != nil {
		t.Fatalf("undo: %v", err)
	}
	if len(u.Log) != 0 {
		t.Fatalf("log length = %d, want 0 after undoing both plies", len(u.Log))
	}
	if u.State.Turn != game.PlayerA {
		t.Fatalf("turn = %v, want A", u.State.Turn)
	}
}

func TestUndoWithoutHumanMoves(t *testing.T) {
	svc := NewService(nil)
	snap := svc.Create(Config{AI: [2]bool{true, false}, EngineID: "alphabeta-r2-v1", Difficulty: "fast"})
	if snap.CanUndo {
		t.Fatal("nothing to undo before any human move")
	}
	moved, err := svc.PlayAI(snap.ID)
	if err != nil {
		t.Fatalf("ai move: %v", err)
	}
	if moved.CanUndo {
		t.Fatal("an AI-only move must not become undoable")
	}
	if _, err := svc.Undo(snap.ID); !errors.Is(err, ErrNothingToUndo) {
		t.Fatalf("err = %v, want ErrNothingToUndo", err)
	}
}

func TestExportImportRoundTrip(t *testing.T) {
	svc := NewService(nil)
	snap := svc.Create(Config{
		AI: [2]bool{false, true}, EngineID: "alphabeta-r2-v1", Difficulty: "fast", Seed: 5,
	})
	if _, err := svc.Play(snap.ID, 0); err != nil {
		t.Fatalf("human move: %v", err)
	}
	after, err := svc.PlayAI(snap.ID)
	if err != nil {
		t.Fatalf("ai move: %v", err)
	}
	exp, err := svc.Export(snap.ID)
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if exp.StateHash != after.State.Hash() {
		t.Fatal("export hash mismatch")
	}

	svc2 := NewService(nil)
	got, err := svc2.Import(exp)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if got.State.Hash() != after.State.Hash() {
		t.Fatalf("imported hash %d != %d", got.State.Hash(), after.State.Hash())
	}
	if len(got.Log) != len(after.Log) {
		t.Fatalf("log length %d != %d", len(got.Log), len(after.Log))
	}
	if got.State.Turn != after.State.Turn || got.CanUndo != after.CanUndo {
		t.Fatalf("turn/canUndo mismatch: %v/%v vs %v/%v", got.State.Turn, got.CanUndo, after.State.Turn, after.CanUndo)
	}
	if got.AI != after.AI || got.EngineID != after.EngineID || got.Difficulty != after.Difficulty {
		t.Fatal("config mismatch after import")
	}
	// The imported match is playable.
	if _, err := svc2.Play(got.ID, 0); err != nil {
		t.Fatalf("play imported: %v", err)
	}
	// And undoable back to the start of the human's turn.
	u, err := svc2.Undo(got.ID)
	if err != nil {
		t.Fatalf("undo imported: %v", err)
	}
	if u.State.Hash() != got.State.Hash() {
		t.Fatal("undo after import did not restore the imported position")
	}
}

func TestImportRejectsHashMismatch(t *testing.T) {
	svc := NewService(nil)
	snap := svc.Create(Config{})
	if _, err := svc.Play(snap.ID, 0); err != nil {
		t.Fatalf("play: %v", err)
	}
	exp, err := svc.Export(snap.ID)
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	exp.StateHash++
	if _, err := NewService(nil).Import(exp); !errors.Is(err, ErrBadSave) {
		t.Fatalf("err = %v, want ErrBadSave", err)
	}
}

func TestUpdateConfigChangesMode(t *testing.T) {
	svc := NewService(nil)
	snap := svc.Create(Config{}) // human vs human
	moved, err := svc.Play(snap.ID, 0)
	if err != nil {
		t.Fatalf("human move: %v", err)
	}
	if moved.PendingAI {
		t.Fatal("no AI in a human vs human match")
	}

	upd, err := svc.UpdateConfig(moved.ID, Config{
		AI: [2]bool{false, true}, EngineID: "alphabeta-r2-v1", Difficulty: "fast",
	})
	if err != nil {
		t.Fatalf("update config: %v", err)
	}
	if !upd.AI[1] || !upd.PendingAI {
		t.Fatalf("B should now be AI and pending: ai=%v pending=%v", upd.AI, upd.PendingAI)
	}
	if upd.EngineID != "alphabeta-r2-v1" || upd.Difficulty != "fast" {
		t.Fatalf("engine/difficulty not updated: %s/%s", upd.EngineID, upd.Difficulty)
	}

	afterAI, err := svc.PlayAI(upd.ID)
	if err != nil {
		t.Fatalf("ai move: %v", err)
	}
	if afterAI.State.Turn != game.PlayerA {
		t.Fatalf("turn = %v, want A", afterAI.State.Turn)
	}

	// Switch back to human vs human; the log is now two human moves.
	back, err := svc.UpdateConfig(upd.ID, Config{AI: [2]bool{false, false}})
	if err != nil {
		t.Fatalf("update back: %v", err)
	}
	if back.PendingAI || back.AI[0] || back.AI[1] {
		t.Fatalf("expected human vs human, got ai=%v pending=%v", back.AI, back.PendingAI)
	}
	u, err := svc.Undo(back.ID)
	if err != nil {
		t.Fatalf("undo: %v", err)
	}
	if len(u.Log) != 1 || u.State.Turn != game.PlayerB {
		t.Fatalf("undo after mode change: log=%d turn=%v", len(u.Log), u.State.Turn)
	}
}

func TestUpdateConfigRejectsUnknownEngine(t *testing.T) {
	svc := NewService(nil)
	snap := svc.Create(Config{})
	if _, err := svc.UpdateConfig(snap.ID, Config{AI: [2]bool{false, true}, EngineID: "nope"}); !errors.Is(err, ErrBadConfig) {
		t.Fatalf("err = %v, want ErrBadConfig", err)
	}
}
