package ml_r2_v3

import (
	"testing"
	"time"

	"wizardboardgame/internal/ai"
	"wizardboardgame/internal/game"
)

func testState(t *testing.T) game.State {
	t.Helper()
	s := game.New(nil)
	for i := range s.Board {
		s.Board[i] = game.NoPiece
	}
	for i := range s.Pieces {
		s.Pieces[i] = game.Piece{}
	}
	for i := range s.Buffs {
		s.Buffs[i] = game.NoPiece
		s.Nerfs[i] = game.NoPiece
	}
	s.Turn = game.PlayerA
	s.Next = [2]uint8{}
	s.Token = [2]uint8{}
	s.Pending = game.NoPiece
	s.Over = false
	s.Winner = game.NoPlayer
	s.Ply = 0
	return s
}

func place(t *testing.T, s *game.State, id game.PieceID, owner game.Player, typ game.UnitType, sq string) {
	t.Helper()
	sqVal, ok := game.ParseSquare(sq)
	if !ok {
		t.Fatalf("bad square %q", sq)
	}
	s.Pieces[id] = game.Piece{Alive: true, Owner: owner, Type: typ, Sq: sqVal}
	s.Board[sqVal] = id
}

func face(t *testing.T, s *game.State, id game.PieceID, d game.Dir) {
	t.Helper()
	s.Pieces[id].Facing = d
}

func sqOf(t *testing.T, name string) game.Square {
	t.Helper()
	sq, ok := game.ParseSquare(name)
	if !ok {
		t.Fatalf("bad square %q", name)
	}
	return sq
}

func selectMove(t *testing.T, s *game.State, cfg ai.Config) game.Action {
	t.Helper()
	legal := s.LegalActions()
	if len(legal) == 0 {
		t.Fatal("no legal actions")
	}
	eng, ok := ai.New(ID, cfg)
	if !ok {
		t.Fatal("engine not registered")
	}
	a := eng.Select(*s, legal)
	if err := checkLegal(s, a); err != nil {
		t.Fatalf("engine returned an illegal action: %v", err)
	}
	return a
}

func checkLegal(s *game.State, a game.Action) error {
	ns, err := s.Apply(a)
	_ = ns
	return err
}

func TestFindsWizardShot(t *testing.T) {
	s := testState(t)
	place(t, &s, 0, game.PlayerA, game.UnitCannon, "d6")
	face(t, &s, 0, game.South)
	place(t, &s, 10, game.PlayerA, game.UnitWizard, "a1")
	place(t, &s, 20, game.PlayerB, game.UnitWizard, "d4") // in the cannon's line
	place(t, &s, 22, game.PlayerB, game.UnitWizard, "h8")
	s.Turn = game.PlayerA
	s.Next[game.PlayerA] = 1 // cannon phase

	a := selectMove(t, &s, ai.Config{MaxNodes: 5000, Seed: 1})
	if a.Kind != game.ActShoot || a.To != sqOf(t, "d4") {
		t.Fatalf("got %v (%s), want shoot at d4", a, s.Describe(a))
	}
}

func TestRespectsTimeBudget(t *testing.T) {
	s := game.New(nil) // standard start, pawn phase
	budget := 120 * time.Millisecond
	start := time.Now()
	selectMove(t, &s, ai.Config{MaxTime: budget, Seed: 2})
	elapsed := time.Since(start)
	if elapsed > budget*4 {
		t.Fatalf("search took %s, budget was %s", elapsed, budget)
	}
}

func TestRespectsNodeBudget(t *testing.T) {
	s := game.New(nil)
	legal := s.LegalActions()
	eng, _ := ai.New(ID, ai.Config{MaxNodes: 400, Seed: 3})
	eng.Select(s, legal) // must terminate quickly; a hang fails the test timeout
}

func TestHandlesPendingFreeMove(t *testing.T) {
	s := testState(t)
	place(t, &s, 10, game.PlayerA, game.UnitWizard, "d4")
	place(t, &s, 22, game.PlayerB, game.UnitWizard, "h8")
	s.Turn = game.PlayerA
	s.Next[game.PlayerA] = 2
	s.Pending = 10 // buffed wizard owes its free move

	legal := s.LegalActions()
	for _, a := range legal {
		if a.Kind != game.ActFreeMove {
			t.Fatalf("unexpected legal action %v", a)
		}
	}
	eng, _ := ai.New(ID, ai.Config{MaxNodes: 500, Seed: 4})
	a := eng.Select(s, legal)
	if a.Kind != game.ActFreeMove || a.Unit != 10 {
		t.Fatalf("got %v, want a free move of wizard 10", a)
	}
	if err := checkLegal(&s, a); err != nil {
		t.Fatalf("free move illegal: %v", err)
	}
}

func TestPlaysLegalFullGame(t *testing.T) {
	s := game.New(nil)
	cfg := ai.Config{MaxNodes: 800, Seed: 5}
	cfgB := cfg
	cfgB.Seed = 77
	engA, _ := ai.New(ID, cfg)
	engB, _ := ai.New(ID, cfgB)
	for ply := 0; ply < 400 && !s.Over; ply++ {
		legal := s.LegalActions()
		if len(legal) == 0 {
			t.Fatalf("no legal actions at ply %d", ply)
		}
		eng := engA
		if s.Turn == game.PlayerB {
			eng = engB
		}
		next, err := s.Apply(eng.Select(s, legal))
		if err != nil {
			t.Fatalf("ply %d: %v", ply, err)
		}
		s = next
	}
	if !s.Over {
		t.Fatalf("game did not finish, ply %d", s.Ply)
	}
}

func BenchmarkSelect(b *testing.B) {
	s := game.New(nil)
	legal := s.LegalActions()
	eng, _ := ai.New(ID, ai.Config{MaxNodes: 20000, Seed: 9})
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		eng.Select(s, legal)
	}
}

func TestAnalyzeSortedAndDeterministic(t *testing.T) {
	s := game.New(nil)
	legal := s.LegalActions()
	before := s.Hash()

	eng, ok := ai.New(ID, ai.Config{MaxNodes: 50000, MaxDepth: 3, Seed: 1})
	if !ok {
		t.Fatal("engine not registered")
	}
	an, ok := eng.(ai.Analyzer)
	if !ok {
		t.Fatal("engine does not implement Analyzer")
	}
	res := an.Analyze(s, legal)
	if res.Depth < 1 {
		t.Fatalf("depth = %d, want >= 1", res.Depth)
	}
	if res.ListDepth > res.Depth {
		t.Fatalf("list depth %d exceeds engine depth %d", res.ListDepth, res.Depth)
	}
	if _, err := s.Apply(res.Engine.Action); err != nil {
		t.Fatalf("illegal engine move %v: %v", res.Engine.Action, err)
	}
	if len(res.Moves) == 0 {
		t.Fatal("no analysed moves")
	}
	for i := 1; i < len(res.Moves); i++ {
		if res.Moves[i].Score > res.Moves[i-1].Score {
			t.Fatalf("not sorted at %d: %d > %d", i, res.Moves[i].Score, res.Moves[i-1].Score)
		}
	}
	for _, m := range res.Moves {
		if _, err := s.Apply(m.Action); err != nil {
			t.Fatalf("illegal analysed move %v: %v", m.Action, err)
		}
	}
	if s.Hash() != before {
		t.Fatal("Analyze mutated the state")
	}

	// Deterministic with the same node budget.
	res2 := an.Analyze(s, legal)
	if len(res2.Moves) != len(res.Moves) || res2.Depth != res.Depth {
		t.Fatalf("second analysis shape differs: %d/%d vs %d/%d", len(res2.Moves), res2.Depth, len(res.Moves), res.Depth)
	}
	for i := range res.Moves {
		if res.Moves[i].Action != res2.Moves[i].Action || res.Moves[i].Score != res2.Moves[i].Score {
			t.Fatalf("analysis not deterministic at %d", i)
		}
	}
}

// TestAnalyzeScoresAlternativesWithinTimeBudget guards the phase split: the
// top-move search must not consume the whole time budget, otherwise the assist
// returns the engine move with no ranked alternatives.
func TestAnalyzeScoresAlternativesWithinTimeBudget(t *testing.T) {
	s := game.New(nil)
	legal := s.LegalActions()

	eng, ok := ai.New(ID, ai.Config{MaxTime: time.Second, MaxDepth: 6, Seed: 1})
	if !ok {
		t.Fatal("engine not registered")
	}
	an := eng.(ai.Analyzer)
	res := an.Analyze(s, legal)
	if res.Depth < 1 {
		t.Fatalf("depth = %d, want >= 1", res.Depth)
	}
	if len(res.Moves) == 0 {
		t.Fatal("no analysed alternatives within the time budget")
	}
	for _, m := range res.Moves {
		if _, err := s.Apply(m.Action); err != nil {
			t.Fatalf("illegal analysed move %v: %v", m.Action, err)
		}
	}
}
