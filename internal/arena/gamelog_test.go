package arena

import (
	"testing"

	"wizardboardgame/internal/game"
)

func TestRunPairLogsGames(t *testing.T) {
	res, err := RunPair("alphabeta-r2-v1", "ml-r2-v1", 2, GameConfig{MaxNodes: 100, Workers: 1, Seed: 3, LogGames: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Logs) != 2 {
		t.Fatalf("logs = %d, want 2", len(res.Logs))
	}
	for _, log := range res.Logs {
		if log.Plies != len(log.Actions) {
			t.Fatalf("plies = %d, actions = %d", log.Plies, len(log.Actions))
		}
		s := game.New(&log.Rules)
		for i, a := range log.Actions {
			next, err := s.Apply(a)
			if err != nil {
				t.Fatalf("replay ply %d: %v", i, err)
			}
			s = next
		}
		if !s.Over {
			t.Fatalf("replay did not finish, ply %d", s.Ply)
		}
		if s.Winner != log.Winner {
			t.Fatalf("replayed winner %v, logged %v", s.Winner, log.Winner)
		}
	}
}

func TestRunPairDefaultsToNoLogs(t *testing.T) {
	res, err := RunPair("alphabeta-r2-v1", "ml-r2-v1", 1, GameConfig{MaxNodes: 100, Workers: 1, Seed: 4})
	if err != nil {
		t.Fatal(err)
	}
	if res.Logs != nil {
		t.Fatalf("logs should be nil by default, got %d", len(res.Logs))
	}
}

func TestFindBestMoves(t *testing.T) {
	res, err := RunPair("alphabeta-r2-v1", "ml-r2-v1", 2, GameConfig{MaxNodes: 100, Workers: 1, Seed: 7, LogGames: true})
	if err != nil {
		t.Fatal(err)
	}
	moves, cfg, err := FindBestMoves(res.Logs, BestMoveOptions{Engine: "ml-r2-v1", Depth: 1, Top: 3})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Engine != "ml-r2-v1" || cfg.Depth != 1 {
		t.Fatalf("config = %+v", cfg)
	}
	if cfg.Games == 0 {
		t.Skip("no decisive games in this sample")
	}
	if len(moves) == 0 {
		t.Fatal("no best moves found")
	}
	if len(moves) > 3 {
		t.Fatalf("got %d moves, want <= 3", len(moves))
	}
	for i, m := range moves {
		if m.Rank != i+1 {
			t.Fatalf("rank %d = %d", i, m.Rank)
		}
		if m.Label == "" || m.Engine == "" || m.Pair == "" {
			t.Fatalf("incomplete best move: %+v", m)
		}
		if m.Before.Pieces == [game.MaxPieces]game.Piece{} {
			t.Fatal("before state is empty")
		}
	}
}
