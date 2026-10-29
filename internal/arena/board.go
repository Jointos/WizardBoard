package arena

import (
	"fmt"
	"html/template"
	"math"
	"strings"

	"wizardboardgame/internal/game"
)

const (
	boardCell   = 54.0
	boardMargin = 24.0
	boardSize   = boardCell * 8
	boardTotal  = boardSize + 2*boardMargin
)

// BoardSVG draws one game state as a frontend-style SVG board. When action is
// non-nil the move is highlighted (source, destination, arrow, cast/shoot line
// and capture marker). It is used by the HTML stats report.
func BoardSVG(s *game.State, action *game.Action) template.HTML {
	if s == nil {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, `<svg class="board-svg" viewBox="0 0 %.0f %.0f" role="img" aria-label="Game board">`, boardTotal, boardTotal)
	fmt.Fprintf(&b, `<rect x="0" y="0" width="%.0f" height="%.0f" rx="10" fill="#0e1219"/>`, boardTotal, boardTotal)

	// Squares.
	for f := 0; f < 8; f++ {
		for r := 0; r < 8; r++ {
			x := boardMargin + float64(f)*boardCell
			y := boardMargin + float64(7-r)*boardCell
			fill := "#232c3c"
			if (f+r)%2 == 0 {
				fill = "#2c3648"
			}
			fmt.Fprintf(&b, `<rect x="%.1f" y="%.1f" width="%.1f" height="%.1f" fill="%s"/>`, x, y, boardCell, boardCell, fill)
		}
	}
	// Coordinates.
	for f := 0; f < 8; f++ {
		x := boardMargin + float64(f)*boardCell + boardCell/2
		fmt.Fprintf(&b, `<text x="%.1f" y="%.1f" text-anchor="middle" font-size="10" font-family="ui-monospace,monospace" fill="#8593aa">%c</text>`,
			x, boardMargin+boardSize+16, 'a'+f)
	}
	for r := 0; r < 8; r++ {
		y := boardMargin + float64(7-r)*boardCell + boardCell/2 + 3
		fmt.Fprintf(&b, `<text x="%.1f" y="%.1f" text-anchor="middle" font-size="10" font-family="ui-monospace,monospace" fill="#8593aa">%d</text>`,
			boardMargin-12, y, r+1)
	}

	// Magical links (buff green, nerf red) behind the pieces.
	for i, p := range s.Pieces {
		if !p.Alive {
			continue
		}
		if c := s.Buffs[i]; c != game.NoPiece && int(c) < len(s.Pieces) && s.Pieces[c].Alive {
			link(&b, s.Pieces[c].Sq, p.Sq, "#57d98a")
		}
		if c := s.Nerfs[i]; c != game.NoPiece && int(c) < len(s.Pieces) && s.Pieces[c].Alive {
			link(&b, s.Pieces[c].Sq, p.Sq, "#f06b6b")
		}
	}

	// Move highlight below the pieces.
	if action != nil {
		drawMoveHighlight(&b, s, *action)
	}

	// Pieces.
	for i, p := range s.Pieces {
		if !p.Alive {
			continue
		}
		x, y := sqTopLeft(p.Sq)
		color := "#7aa2ff"
		ink := "#0b1428"
		if p.Owner == game.PlayerB {
			color = "#ffb066"
			ink = "#2a1403"
		}
		pad := 3.0
		fmt.Fprintf(&b, `<use href="#pc-%s" x="%.1f" y="%.1f" width="%.1f" height="%.1f" style="color:%s"/>`,
			p.Type.String(), x+pad, y+pad, boardCell-2*pad, boardCell-2*pad, color)
		cx, cy := sqCenter(p.Sq)
		if p.Type == game.UnitCannon {
			dx, dy := game.DirDelta(p.Facing)
			fmt.Fprintf(&b, `<line x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" stroke="%s" stroke-width="3" stroke-linecap="round" opacity=".85"/>`,
				cx, cy, cx+float64(dx)*boardCell*0.3, cy-float64(dy)*boardCell*0.3, ink)
		}
		if p.Type == game.UnitPawn && s.Rules != nil && s.Rules.PawnForwardCaptureEvery > 0 {
			if (p.Acts+1)%s.Rules.PawnForwardCaptureEvery == 0 {
				fmt.Fprintf(&b, `<circle cx="%.1f" cy="%.1f" r="3" fill="#f0d45e"/>`, x+boardCell-9, y+9)
			}
		}
		if s.Buffs[i] != game.NoPiece {
			fmt.Fprintf(&b, `<circle cx="%.1f" cy="%.1f" r="3.2" fill="#57d98a" stroke="#0b0e14" stroke-width="1"/>`, x+9, y+9)
		} else if s.Nerfs[i] != game.NoPiece {
			fmt.Fprintf(&b, `<circle cx="%.1f" cy="%.1f" r="3.2" fill="#f06b6b" stroke="#0b0e14" stroke-width="1"/>`, x+9, y+9)
		}
	}

	b.WriteString(`</svg>`)
	return template.HTML(b.String())
}

func drawMoveHighlight(b *strings.Builder, s *game.State, a game.Action) {
	from := a.From
	if !validSq(from) {
		return
	}
	// Source.
	rectAt(b, from, "none", "#f0d45e", 3)
	srcX, srcY := sqCenter(from)

	to := a.To
	targetSq := game.NoSquare
	cast := a.Kind == game.ActCast
	if cast && a.Target != game.NoPiece && int(a.Target) < len(s.Pieces) && s.Pieces[a.Target].Alive {
		targetSq = s.Pieces[a.Target].Sq
	}
	if !cast && a.Kind == game.ActCapture && a.Target != game.NoPiece && int(a.Target) < len(s.Pieces) && s.Pieces[a.Target].Alive {
		targetSq = s.Pieces[a.Target].Sq
	}

	switch a.Kind {
	case game.ActSkip:
		fmt.Fprintf(b, `<text x="%.1f" y="%.1f" text-anchor="middle" font-size="10" font-weight="700" fill="#f0d45e">SKIP</text>`, srcX, srcY+boardCell*0.33)
		return
	case game.ActTurn:
		dx, dy := game.DirDelta(a.Facing)
		drawArrow(b, srcX, srcY, srcX+float64(dx)*boardCell*0.55, srcY-float64(dy)*boardCell*0.55, "#f0d45e", false)
		return
	case game.ActCast:
		if validSq(targetSq) {
			rectAt(b, targetSq, "rgba(87,217,138,.10)", "#57d98a", 3)
			tx, ty := sqCenter(targetSq)
			color := "#57d98a"
			if a.Spell == game.SpellNerf {
				color = "#f06b6b"
				rectAt(b, targetSq, "rgba(240,107,107,.10)", "#f06b6b", 3)
			}
			drawArrow(b, srcX, srcY, tx, ty, color, false)
		}
		if validSq(to) {
			x, y := sqCenter(to)
			fmt.Fprintf(b, `<circle cx="%.1f" cy="%.1f" r="6" fill="none" stroke="#f0d45e" stroke-width="2" stroke-dasharray="2 3"/>`, x, y)
			rectAt(b, to, "none", "#f0d45e", 2)
		}
		return
	}

	if !validSq(to) {
		return
	}
	rectAt(b, to, "rgba(87,217,138,.14)", "#57d98a", 3)
	dstX, dstY := sqCenter(to)
	color := "#f0d45e"
	dashed := false
	switch a.Kind {
	case game.ActTeleport:
		dashed = true
	case game.ActShoot:
		color = "#f06b6b"
		if validSq(targetSq) {
			dstX, dstY = sqCenter(targetSq)
			rectAt(b, targetSq, "rgba(240,107,107,.12)", "#f06b6b", 3)
		}
	case game.ActCapture:
		color = "#f06b6b"
		if validSq(targetSq) {
			dstX, dstY = sqCenter(targetSq)
		}
	}
	drawArrow(b, srcX, srcY, dstX, dstY, color, dashed)
	if a.Kind == game.ActCapture || a.Kind == game.ActShoot {
		drawX(b, dstX, dstY, 10, "#f06b6b")
	}
	if a.Kind == game.ActMoveTurn {
		dx, dy := game.DirDelta(a.Facing)
		drawArrow(b, dstX, dstY, dstX+float64(dx)*boardCell*0.3, dstY-float64(dy)*boardCell*0.3, "#f0d45e", false)
	}
}

func validSq(sq game.Square) bool {
	return sq != game.NoSquare && int(sq) >= 0 && int(sq) < game.NumSquares
}

func sqTopLeft(sq game.Square) (x, y float64) {
	f := game.FileOf(sq)
	r := game.RankOf(sq)
	return boardMargin + float64(f)*boardCell, boardMargin + float64(7-r)*boardCell
}

func sqCenter(sq game.Square) (x, y float64) {
	x, y = sqTopLeft(sq)
	return x + boardCell/2, y + boardCell/2
}

func rectAt(b *strings.Builder, sq game.Square, fill, stroke string, width float64) {
	x, y := sqTopLeft(sq)
	fmt.Fprintf(b, `<rect x="%.1f" y="%.1f" width="%.1f" height="%.1f" rx="5" fill="%s" stroke="%s" stroke-width="%.0f"/>`,
		x+1.5, y+1.5, boardCell-3, boardCell-3, fill, stroke, width)
}

func link(b *strings.Builder, from, to game.Square, color string) {
	x1, y1 := sqCenter(from)
	x2, y2 := sqCenter(to)
	fmt.Fprintf(b, `<line x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" stroke="%s" stroke-width="2" stroke-dasharray="2 5" opacity=".6"/>`,
		x1, y1, x2, y2, color)
}

func drawArrow(b *strings.Builder, x1, y1, x2, y2 float64, color string, dashed bool) {
	dx, dy := x2-x1, y2-y1
	length := math.Hypot(dx, dy)
	if length < 1 {
		return
	}
	ux, uy := dx/length, dy/length
	// Leave a gap at both ends so the arrow does not cover the pieces.
	start := 20.0
	end := length - 22
	if end <= start {
		start, end = 0, length
	}
	ax1, ay1 := x1+ux*start, y1+uy*start
	ax2, ay2 := x1+ux*end, y1+uy*end
	dash := ""
	if dashed {
		dash = ` stroke-dasharray="7 6"`
	}
	fmt.Fprintf(b, `<line x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" stroke="%s" stroke-width="3.2" stroke-linecap="round" opacity=".9"%s/>`,
		ax1, ay1, ax2, ay2, color, dash)
	// Arrowhead.
	h := 9.0
	w := 5.5
	bx, by := ax2-ux*h, ay2-uy*h
	px, py := -uy*w, ux*w
	fmt.Fprintf(b, `<polygon points="%.1f,%.1f %.1f,%.1f %.1f,%.1f" fill="%s" opacity=".95"/>`,
		ax2, ay2, bx+px, by+py, bx-px, by-py, color)
}

func drawX(b *strings.Builder, x, y, r float64, color string) {
	fmt.Fprintf(b, `<line x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" stroke="%s" stroke-width="3" stroke-linecap="round"/>`,
		x-r, y-r, x+r, y+r, color)
	fmt.Fprintf(b, `<line x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" stroke="%s" stroke-width="3" stroke-linecap="round"/>`,
		x+r, y-r, x-r, y+r, color)
}
