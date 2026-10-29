package arena

import (
	"strings"
	"testing"

	"wizardboardgame/internal/game"
)

func TestBoardSVG(t *testing.T) {
	s := game.New(nil)
	var move game.Action
	found := false
	for _, a := range s.LegalActions() {
		if a.Kind == game.ActMove {
			move = a
			found = true
			break
		}
	}
	if !found {
		t.Fatal("no move action")
	}

	svg := string(BoardSVG(&s, &move))
	for _, want := range []string{
		`class="board-svg"`,
		`viewBox="0 0 480 480"`,
		`href="#pc-pawn"`,
		`href="#pc-wizard"`,
		`#f0d45e`, // source / arrow
		`#57d98a`, // destination
	} {
		if !strings.Contains(svg, want) {
			t.Fatalf("board missing %q", want)
		}
	}

	ns, err := s.Apply(move)
	if err != nil {
		t.Fatal(err)
	}
	after := string(BoardSVG(&ns, nil))
	if !strings.Contains(after, `href="#pc-wizard"`) {
		t.Fatal("after board has no pieces")
	}
	if strings.Contains(after, `stroke="#f0d45e"`) {
		t.Fatal("after board should not contain a move overlay")
	}
}
