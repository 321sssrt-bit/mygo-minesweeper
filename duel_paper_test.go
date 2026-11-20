package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"mygo-minesweeper/internal/scanreport"

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

// Duel territory, flag outlines, latest AI move frame, and duel minimap
// colours all come from palette.go with the agreed pink/blue scheme, and
// board.go retains zero raw hex literals for those roles.
func TestDuelPaletteUsesPinkAndBlueFromPalette(t *testing.T) {
	if duelOwnerColor(DuelPlayer) != ui.Hex("#55B2D6") || duelPalette.Player != ui.Hex("#55B2D6") {
		t.Errorf("player duel color = %s, want #55B2D6", hexOf(duelPalette.Player))
	}
	if duelOwnerColor(DuelAI) != ui.Hex("#F07AA4") || duelPalette.AI != ui.Hex("#F07AA4") {
		t.Errorf("AI duel color = %s, want #F07AA4", hexOf(duelPalette.AI))
	}
	wantMinimap := map[string]struct {
		got  ui.Color
		want string
	}{
		"Bg":           {duelMinimapPalette.Bg, "#0B1423"},
		"Contested":    {duelMinimapPalette.Contested, "#A78BFA"},
		"Player":       {duelMinimapPalette.Player, "#55B2D6"},
		"AI":           {duelMinimapPalette.AI, "#F07AA4"},
		"PlayerMarker": {duelMinimapPalette.PlayerMarker, "#38BDF8"},
		"PlayerStroke": {duelMinimapPalette.PlayerStroke, "#F4F8FD"},
		"AIMarker":     {duelMinimapPalette.AIMarker, "#F07AA4"},
		"AIStroke":     {duelMinimapPalette.AIStroke, "#FDF2F8"},
		"Viewport":     {duelMinimapPalette.Viewport, "#F4F8FD"},
	}
	for name, tc := range wantMinimap {
		if hexOf(tc.got) != tc.want {
			t.Errorf("duelMinimapPalette.%s = %s, want %s", name, hexOf(tc.got), tc.want)
		}
	}

	rep, err := scanreport.Scan(".")
	if err != nil {
		t.Fatalf("scanreport.Scan failed: %v", err)
	}
	migrated := []string{"#0B1423", "#A78BFA", "#55B2D6", "#F07AA4", "#38BDF8", "#F4F8FD", "#FDF2F8"}
	for _, f := range rep.Colors {
		if filepath.Base(f.Path) != "board.go" {
			continue
		}
		upper := strings.ToUpper(f.Text)
		for _, hex := range migrated {
			if strings.Contains(upper, hex) {
				t.Errorf("board.go:%d still defines migrated duel raw colour %s", f.Line, f.Text)
			}
		}
	}
}

// Rendered duel flags, last AI move frame, and duel minimap (backdrop,
// player/AI/contested territory blocks, markers, and viewport frame) paint
// the pink/blue and dark-navy palette colours in both day and night themes.
func TestDuelFlagsLastAIAndMinimapPaintPaletteColors(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(fmt.Sprintf("dark=%v", dark), func(t *testing.T) {
			app := stageDuelScene(dark)
			g := app.game
			g.duel.NextAI = 1e9

			// Ensure distinct player-only, AI-only, and contested minimap blocks
			// far from the center skirmish so the stage view stays unaltered.
			pBlockRow, pBlockCol := 100, 100
			aBlockRow, aBlockCol := 100, 200
			cBlockRow, cBlockCol := 100, 300
			setCell := func(row, col int, owner DuelOwner) {
				c := g.Cell(row, col)
				c.IsMine = false
				c.Revealed = true
				c.Owner = owner
				g.duel.adjustMinimap(g, row, col, DuelNobody, owner)
			}
			for d := 0; d < 10; d++ {
				setCell(pBlockRow+d/5, pBlockCol+d%5, DuelPlayer)
				setCell(aBlockRow+d/5, aBlockCol+d%5, DuelAI)
				if d < 5 {
					setCell(cBlockRow, cBlockCol+d, DuelPlayer)
				} else {
					setCell(cBlockRow+1, cBlockCol+d-5, DuelAI)
				}
			}

			tt := ui.NewTester(app.view, 1280, 900)
			tt.SetScale(2)
			tt.Frame()
			app.board.center(g, duelStageRow, duelStageCol)
			// Land pulse on phase 0 so marker rings do not alter solid marker fills.
			tt.Advance(time.Duration(1200-tt.Now().UnixMilli()%1200) * time.Millisecond)
			tt.Frame()

			board, ok := tt.Find("百万格决斗战场")
			if !ok {
				t.Fatal("duel board not found")
			}
			img := tt.Image()
			pal := paperFor(dark)
			v := app.board

			// 1. Player and AI flag outlines on the board use duelPalette colours.
			var pFlag, aFlag *Cell
			for row := duelStageRow - 6; row <= duelStageRow+6; row++ {
				for col := duelStageCol - 8; col <= duelStageCol+12; col++ {
					c := g.Cell(row, col)
					if c.Flagged && c.FlagOwner == DuelPlayer && pFlag == nil {
						pFlag = c
					}
					if c.Flagged && c.FlagOwner == DuelAI && aFlag == nil {
						aFlag = c
					}
				}
			}
			if pFlag == nil || aFlag == nil {
				t.Fatalf("staged duel missing flags: player=%v ai=%v", pFlag, aFlag)
			}
			if n := countMatchingPixels(img, classicCanvasCellRect(board, v, pFlag.Row, pFlag.Col), 2, duelPalette.Player, 18); n < 16 {
				t.Errorf("player flag (%d,%d) had only %d pixels near %s", pFlag.Row, pFlag.Col, n, hexOf(duelPalette.Player))
			}
			if n := countMatchingPixels(img, classicCanvasCellRect(board, v, aFlag.Row, aFlag.Col), 2, duelPalette.AI, 18); n < 16 {
				t.Errorf("AI flag (%d,%d) had only %d pixels near %s", aFlag.Row, aFlag.Col, n, hexOf(duelPalette.AI))
			}

			// 2. LastAI cell frame uses duelPalette.AI.
			lastAIRow, lastAICol := g.duel.LastAI/g.Cols, g.duel.LastAI%g.Cols
			lastAIRect := ui.Rect{
				X: board.X + float32(lastAICol)*v.Cell - v.X - 1,
				Y: board.Y + float32(lastAIRow)*v.Cell - v.Y - 1,
				W: v.Cell + 2,
				H: v.Cell + 2,
			}
			if n := countMatchingPixels(img, lastAIRect, 2, duelPalette.AI, 18); n < 16 {
				t.Errorf("LastAI frame (%d,%d) had only %d pixels near %s", lastAIRow, lastAICol, n, hexOf(duelPalette.AI))
			}

			// 3. Duel minimap backdrop, territory blocks, markers, and viewport frame.
			dm := duelMinimapPalette
			mini := v.minimapBoundsFor(g)
			miniAbs := ui.Rect{X: board.X + mini.X, Y: board.Y + mini.Y, W: mini.W, H: mini.H}
			bgOver := dm.Bg.Alpha(0.94).Over(pal.Board)
			assertColorNear(t, "duel minimap backdrop", sampleCanvasPixel(img, miniAbs.X+8, miniAbs.Y+8, 2), bgOver)

			bw, bh := miniAbs.W/float32(minimapGridSize), miniAbs.H/float32(minimapGridSize)
			blockRect := func(row, col int) ui.Rect {
				br := g.duel.minimapBlockIndex(g, row, col) / minimapGridSize
				bc := g.duel.minimapBlockIndex(g, row, col) % minimapGridSize
				return ui.Rect{X: miniAbs.X + float32(bc)*bw, Y: miniAbs.Y + float32(br)*bh, W: max(2, bw), H: max(2, bh)}
			}
			wantPlayerBlock := dm.Player.Alpha(0.65 + 0.35*min(1, float32(10)/20)).Over(bgOver)
			wantAIBlock := dm.AI.Alpha(0.65 + 0.35*min(1, float32(10)/20)).Over(bgOver)
			wantContestedBlock := dm.Contested.Alpha(0.65 + 0.35*min(1, float32(10)/24)).Over(bgOver)
			if n := countMatchingPixels(img, blockRect(pBlockRow, pBlockCol), 2, wantPlayerBlock, 16); n == 0 {
				t.Errorf("duel minimap player block missing expected colour %s", hexOf(wantPlayerBlock))
			}
			if n := countMatchingPixels(img, blockRect(aBlockRow, aBlockCol), 2, wantAIBlock, 16); n == 0 {
				t.Errorf("duel minimap AI block missing expected colour %s", hexOf(wantAIBlock))
			}
			if n := countMatchingPixels(img, blockRect(cBlockRow, cBlockCol), 2, wantContestedBlock, 16); n == 0 {
				t.Errorf("duel minimap contested block missing expected colour %s", hexOf(wantContestedBlock))
			}
			if n := countMatchingPixels(img, miniAbs, 2, dm.PlayerMarker, 16); n < 4 {
				t.Errorf("duel minimap missing player marker %s (matched %d px)", hexOf(dm.PlayerMarker), n)
			}
			if n := countMatchingPixels(img, miniAbs, 2, dm.AIMarker, 16); n < 4 {
				t.Errorf("duel minimap missing AI marker %s (matched %d px)", hexOf(dm.AIMarker), n)
			}
			// Pan away from the center markers so only the viewport frame sits along vpTop.
			app.board.center(g, 300, 300)
			tt.Frame()
			vp := minimapViewportRect(miniAbs, g.Rows, g.Cols, v.X, v.Y, board.W, board.H, v.Cell)
			vpTop := ui.Rect{X: vp.X, Y: vp.Y - 1, W: vp.W, H: 3}
			if n := countMatchingPixels(tt.Image(), vpTop, 2, dm.Viewport, 16); n < 4 {
				t.Errorf("duel minimap missing viewport frame %s along %+v (matched %d px)", hexOf(dm.Viewport), vpTop, n)
			}
		})
	}
}
