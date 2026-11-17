package main

import (
	"fmt"
	"testing"

	"github.com/egoist/mygo/ui"
)

// Numbers on the duel canvas are the classic board's deep inks, and stay
// readable on both sides' territory, on both papers.
func TestDuelNumbersReadOnEveryTerritory(t *testing.T) {
	for _, dark := range []bool{false, true} {
		pal, inks := paperFor(dark), duelInksFor(dark)
		for owner := DuelNobody; owner <= DuelAI; owner++ {
			under := duelCellPaper(pal, owner)
			for n := 1; n <= 8; n++ {
				ink := inks[owner][n]
				if got := contrast(ink, under); got < minInkContrast {
					t.Errorf("dark=%v owner=%d number %d ink %s on %s: contrast %.2f < %.1f",
						dark, owner, n, hexOf(ink), hexOf(under), got, minInkContrast)
				}
				if owner == DuelNobody && ink != numberInk(n, dark) {
					t.Errorf("dark=%v number %d on bare paper is %s, want the classic ink %s",
						dark, n, hexOf(ink), hexOf(numberInk(n, dark)))
				}
			}
		}
	}
}

// The duel canvas is the notebook page in both themes: unopened cells are
// paper slips, and each side's opened ground is the page under its wash.
func TestDuelCanvasPaintsPaperDayAndNight(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(fmt.Sprintf("dark=%v", dark), func(t *testing.T) {
			app := stageDuelScene(dark)
			g := app.game
			g.duel.NextAI = 1e9
			tt := ui.NewTester(app.view, 1280, 900)
			tt.SetScale(1)
			tt.Frame()
			app.board.center(g, duelStageRow, duelStageCol)
			tt.Frame()
			board, ok := tt.Find("百万格决斗战场")
			if !ok {
				t.Fatal("duel board not found")
			}
			pal := paperFor(dark)
			want := map[string]ui.Color{
				"slip":   pal.Slip,
				"player": duelCellPaper(pal, DuelPlayer),
				"ai":     duelCellPaper(pal, DuelAI),
			}
			v := app.board
			// Sample only cells in open view: not under the minimap or the
			// caption along the bottom edge.
			center := func(row, col int) (float32, float32, bool) {
				x := (float32(col)+0.5)*v.Cell - v.X
				y := (float32(row)+0.5)*v.Cell - v.Y
				open := x > 0 && y > 0 && x < board.W && y < board.H-48 && !v.minimapFor(g, x, y)
				return board.X + x, board.Y + y, open
			}
			found := map[string][2]int{}
			for row := duelStageRow - 6; row <= duelStageRow+6; row++ {
				for col := duelStageCol - 8; col <= duelStageCol+12; col++ {
					c := g.Cell(row, col)
					kind := ""
					switch {
					case !c.Revealed && !c.Flagged:
						kind = "slip"
					case c.Revealed && !c.IsMine && c.Adjacent == 0 && c.Owner == DuelPlayer:
						kind = "player"
					case c.Revealed && !c.IsMine && c.Adjacent == 0 && c.Owner == DuelAI:
						kind = "ai"
					}
					_, _, open := center(row, col)
					if _, seen := found[kind]; kind != "" && !seen && open && g.duel.LastAI != row*g.Cols+col {
						found[kind] = [2]int{row, col}
					}
				}
			}
			img := tt.Image()
			for kind, color := range want {
				at, ok := found[kind]
				if !ok {
					t.Fatalf("staged duel has no %s cell in open view", kind)
				}
				x, y, _ := center(at[0], at[1])
				got := img.RGBAAt(int(x), int(y))
				if !near(got.R, color.R) || !near(got.G, color.G) || !near(got.B, color.B) {
					t.Errorf("%s cell (%d,%d) painted #%02X%02X%02X, want %s", kind, at[0], at[1], got.R, got.G, got.B, hexOf(color))
				}
			}
		})
	}
}

// Zoomed out past drawing knots, a mine that went off still shows as a dot
// of the hit knot's ink, apart from the territory wash around it.
func TestDuelHitMineReadsZoomedOut(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(fmt.Sprintf("dark=%v", dark), func(t *testing.T) {
			app := stageDuelScene(dark)
			g := app.game
			g.duel.NextAI = 1e9
			var hit *Cell
			for row := duelStageRow - 6; row <= duelStageRow+6 && hit == nil; row++ {
				for col := duelStageCol + 4; col <= duelStageCol+12; col++ {
					if c := g.Cell(row, col); c.IsMine && !c.Revealed && !c.Flagged {
						hit = c
						break
					}
				}
			}
			if hit == nil {
				t.Fatal("no hidden mine near the stage")
			}
			g.revealDuel(hit.Row, hit.Col, DuelAI)
			if !hit.Exploded {
				t.Fatal("staged AI step did not set the mine off")
			}
			tt := ui.NewTester(app.view, 1280, 900)
			tt.SetScale(2)
			tt.Frame()
			app.board.Cell = 10
			app.board.center(g, hit.Row, hit.Col)
			tt.Frame()
			board, ok := tt.Find("百万格决斗战场")
			if !ok {
				t.Fatal("duel board not found")
			}
			v := app.board
			x := board.X + (float32(hit.Col)+0.5)*v.Cell - v.X
			y := board.Y + (float32(hit.Row)+0.5)*v.Cell - v.Y
			got := tt.Image().RGBAAt(int(x*2), int(y*2))
			want := paperFor(dark).BoomInk
			if !near(got.R, want.R) || !near(got.G, want.G) || !near(got.B, want.B) {
				t.Errorf("hit mine at cell size 10 painted #%02X%02X%02X, want its ink %s", got.R, got.G, got.B, hexOf(want))
			}
		})
	}
}

func near(a, b uint8) bool { return int(a)-int(b) <= 3 && int(b)-int(a) <= 3 }
