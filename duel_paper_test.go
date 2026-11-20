package main

import (
	"fmt"
	"image/png"
	"math"
	"os"
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
			board, ok := tt.Find("决斗棋盘")
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
			board, ok := tt.Find("决斗棋盘")
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

			board, ok := tt.Find("决斗棋盘")
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

func findOrPrepareClaimable(g *Game, centerRow, centerCol int) (int, int, bool) {
	for row := centerRow - 5; row <= centerRow+5; row++ {
		for col := centerCol - 7; col <= centerCol+7; col++ {
			if g.InspectNumberHover(row, col, false).Kind == HintClaimable {
				return row, col, true
			}
		}
	}
	for row := centerRow - 5; row <= centerRow+5; row++ {
		for col := centerCol - 7; col <= centerCol+7; col++ {
			c := g.Cell(row, col)
			if c == nil || !c.Revealed || c.Exploded || c.Adjacent == 0 {
				continue
			}
			hasUnflaggedMine := false
			for _, n := range g.Neighbors(row, col) {
				if n.IsMine && !n.Revealed && !n.Flagged {
					hasUnflaggedMine = true
					break
				}
			}
			if !hasUnflaggedMine {
				continue
			}
			for _, n := range g.Neighbors(row, col) {
				if !n.IsMine && !n.Revealed && !n.Flagged {
					g.Reveal(n.Row, n.Col)
				}
			}
			if g.InspectNumberHover(row, col, false).Kind == HintClaimable {
				return row, col, true
			}
		}
	}
	return -1, -1, false
}

func findOrPrepareChordable(g *Game, centerRow, centerCol int) (int, int, bool) {
	for row := centerRow - 5; row <= centerRow+5; row++ {
		for col := centerCol - 7; col <= centerCol+7; col++ {
			if g.InspectNumberHover(row, col, false).Kind == HintChordable {
				return row, col, true
			}
		}
	}
	for row := centerRow - 5; row <= centerRow+5; row++ {
		for col := centerCol - 7; col <= centerCol+7; col++ {
			c := g.Cell(row, col)
			if c == nil || !c.Revealed || c.Exploded || c.Adjacent == 0 {
				continue
			}
			hasHiddenSafe := false
			for _, n := range g.Neighbors(row, col) {
				if !n.IsMine && !n.Revealed && !n.Flagged {
					hasHiddenSafe = true
					break
				}
			}
			if !hasHiddenSafe {
				continue
			}
			for _, n := range g.Neighbors(row, col) {
				if n.IsMine && !n.Revealed && !n.Flagged {
					g.ToggleFlag(n.Row, n.Col, false)
				}
			}
			if g.InspectNumberHover(row, col, false).Kind == HintChordable {
				return row, col, true
			}
		}
	}
	return -1, -1, false
}

// Duel prompts (gold claim highlighter, leader chord highlighter, 9×9 scan
// badge, four floating score pills, and milestone chat highlights) stay
// WCAG-AA readable on both papers and leave no migrated raw hex literals in
// board.go or main.go.
func TestDuelPromptsContrastAndScanReport(t *testing.T) {
	for _, dark := range []bool{false, true} {
		pal, dp := paperFor(dark), duelPromptsFor(dark)
		if got := contrast(dp.ScanBadgeText, dp.ScanBadgeBg); got < minInkContrast {
			t.Errorf("dark=%v scan badge text %s on %s: contrast %.2f < %.1f",
				dark, hexOf(dp.ScanBadgeText), hexOf(dp.ScanBadgeBg), got, minInkContrast)
		}
		for kind := duelPopupScore; kind <= duelPopupBoom; kind++ {
			st := dp.Popups[kind]
			if got := contrast(st.Text, st.Bg); got < minInkContrast {
				t.Errorf("dark=%v popup kind=%d text %s on %s: contrast %.2f < %.1f",
					dark, kind, hexOf(st.Text), hexOf(st.Bg), got, minInkContrast)
			}
		}
		claimFill, claimStroke := hoverHintColors(HintClaimable, MemberTomori, dark)
		if claimFill != goldHighlighter || claimStroke != pal.InkOf(goldHighlighter) {
			t.Errorf("dark=%v HintClaimable colors = (%s, %s), want (%s, %s)",
				dark, hexOf(claimFill), hexOf(claimStroke), hexOf(goldHighlighter), hexOf(pal.InkOf(goldHighlighter)))
		}
		for _, m := range Members {
			base := memberImage(m.ID)
			fill, stroke := hoverHintColors(HintChordable, m.ID, dark)
			if fill != base || stroke != pal.InkOf(base) {
				t.Errorf("dark=%v member=%s HintChordable colors = (%s, %s), want (%s, %s)",
					dark, m.ID, hexOf(fill), hexOf(stroke), hexOf(base), hexOf(pal.InkOf(base)))
			}
		}
	}

	app := stageDuelScene(false)
	now := time.Unix(1000, 0)
	app.spawnDuelPopup(duelStageRow, duelStageCol, ActionResult{Kind: ActionFlagged, ScoreDelta: 75}, now)
	app.spawnDuelPopup(duelStageRow, duelStageCol, ActionResult{Kind: ActionLost, ScoreDelta: -100}, now)
	app.game.duel.PlayerCombo = 4
	app.spawnDuelPopup(duelStageRow, duelStageCol, ActionResult{Kind: ActionRevealed, RevealedCount: 9, ScoreDelta: 12}, now)
	app.game.duel.PlayerCombo = 1
	app.spawnDuelPopup(duelStageRow, duelStageCol, ActionResult{Kind: ActionRevealed, RevealedCount: 5, ScoreDelta: 6}, now)
	wantKinds := []duelPopupKind{duelPopupClaim, duelPopupBoom, duelPopupCombo, duelPopupScore}
	if len(app.duelPopups) != len(wantKinds) {
		t.Fatalf("spawnDuelPopup created %d popups, want %d", len(app.duelPopups), len(wantKinds))
	}
	for i, want := range wantKinds {
		if app.duelPopups[i].Kind != want {
			t.Errorf("popup[%d].Kind = %d, want %d", i, app.duelPopups[i].Kind, want)
		}
	}

	// Verify milestone chat notes use palette goldHighlighter instead of a raw literal.
	tt := ui.NewTester(func(c *ui.Context) {
		if len(app.unlockedMemes) == 0 {
			app.unlockMemes(c, []string{MemeForever})
		}
		if !app.duelEnded {
			app.settleDuel(c, c.Now(), "决斗测试结算")
		}
	}, 400, 300)
	tt.Frame()
	wantGoldHex := hexOf(goldHighlighter)
	seenUnlock, seenSettle := false, false
	for _, msg := range app.chatLog {
		if msg.Speaker == "名场面解锁" && msg.ColorHex == wantGoldHex {
			seenUnlock = true
		}
		if msg.Speaker == "决斗结算" && msg.ColorHex == wantGoldHex {
			seenSettle = true
		}
	}
	if !seenUnlock || !seenSettle {
		t.Errorf("expected goldHighlighter (%s) on unlockMemes (%v) and settleDuel (%v) chat logs", wantGoldHex, seenUnlock, seenSettle)
	}

	rep, err := scanreport.Scan(".")
	if err != nil {
		t.Fatalf("scanreport.Scan failed: %v", err)
	}
	boardForbidden := []string{"#E8A849", "#3C301D", "#F5D99C", "#070D17"}
	mainForbidden := []string{"#070D17", "#38BDF8", "#FB7185"}
	for _, f := range rep.Colors {
		base := filepath.Base(f.Path)
		upper := strings.ToUpper(f.Text)
		if base == "board.go" {
			for _, hex := range boardForbidden {
				if strings.Contains(upper, hex) {
					t.Errorf("board.go:%d still defines migrated raw colour %s", f.Line, f.Text)
				}
			}
		}
		if base == "main.go" {
			for _, hex := range mainForbidden {
				if strings.Contains(upper, hex) {
					t.Errorf("main.go:%d still defines migrated raw colour %s", f.Line, f.Text)
				}
			}
		}
	}
}

// Rendered duel hover hints (gold claim highlighter and leader chord
// highlighter), 9×9 scan frame & corner badge, four floating score popup
// capsules, and the recent-action pulse frame all paint palette colours in
// both day and night themes.
func TestDuelHintsScanAndPopupsPaintPaletteColors(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(fmt.Sprintf("dark=%v", dark), func(t *testing.T) {
			app := stageDuelScene(dark)
			g := app.game
			g.duel.NextAI = 1e9
			claimRow, claimCol, ok := findOrPrepareClaimable(g, duelStageRow, duelStageCol)
			if !ok {
				t.Fatal("failed to stage a HintClaimable duel cell")
			}
			g.duel.ScanUntil = g.duel.Elapsed + 6

			tt := ui.NewTester(app.view, 1280, 900)
			tt.SetScale(2)
			tt.Frame()
			app.board.center(g, claimRow, claimCol)
			tt.Frame()

			board, ok := tt.Find("决斗棋盘")
			if !ok {
				t.Fatal("duel board not found")
			}
			v := app.board
			pal, dp := paperFor(dark), duelPromptsFor(dark)

			// 1. Hover the claimable number with active 9×9 scan.
			hx := board.X + (float32(claimCol)+0.5)*v.Cell - v.X
			hy := board.Y + (float32(claimRow)+0.5)*v.Cell - v.Y
			tt.Move(hx, hy)
			tt.Frame()
			if app.board.LastHoverHint.Kind != HintClaimable {
				t.Fatalf("LastHoverHint.Kind = %v, want HintClaimable", app.board.LastHoverHint.Kind)
			}
			if _, ok := g.ScanAt(claimRow, claimCol); !ok {
				t.Fatalf("ScanAt(%d,%d) = false, want true", claimRow, claimCol)
			}

			// Trigger recent-action pulse on a nearby open cell outside the 9×9 border.
			effectRow, effectCol := claimRow-6, claimCol-6
			app.board.EffectCell = effectRow*g.Cols + effectCol
			app.board.LastEffect = tt.Now()

			// Spawn all four popup kinds at well-separated cells.
			popupCells := []struct {
				kind duelPopupKind
				row  int
				col  int
				res  ActionResult
				cmb  int
			}{
				{duelPopupClaim, claimRow - 5, claimCol - 4, ActionResult{Kind: ActionFlagged, ScoreDelta: 75}, 1},
				{duelPopupBoom, claimRow - 5, claimCol + 6, ActionResult{Kind: ActionLost, ScoreDelta: -100}, 0},
				{duelPopupCombo, claimRow + 6, claimCol - 4, ActionResult{Kind: ActionRevealed, RevealedCount: 9, ScoreDelta: 12}, 4},
				{duelPopupScore, claimRow + 6, claimCol + 6, ActionResult{Kind: ActionRevealed, RevealedCount: 5, ScoreDelta: 6}, 1},
			}
			popNow := tt.Now()
			for _, pc := range popupCells {
				g.duel.PlayerCombo = pc.cmb
				app.spawnDuelPopup(pc.row, pc.col, pc.res, popNow)
			}
			tt.Frame()
			img := tt.Image()

			// Verify HintClaimable center & target slip colours.
			claimFill, claimStroke := hoverHintColors(HintClaimable, app.memberID, dark)
			claimCell := g.Cell(claimRow, claimCol)
			wantCenterFill := claimFill.Alpha(0.24).Over(duelCellPaper(pal, claimCell.Owner))
			centerRect := classicCanvasCellRect(board, v, claimRow, claimCol)
			if n := countMatchingPixels(img, centerRect, 2, wantCenterFill, 14); n < 16 {
				t.Errorf("HintClaimable center (%d,%d) had only %d px near %s", claimRow, claimCol, n, hexOf(wantCenterFill))
			}
			if n := countMatchingPixels(img, centerRect, 2, claimStroke, 18); n < 12 {
				t.Errorf("HintClaimable center (%d,%d) stroke had only %d px near %s", claimRow, claimCol, n, hexOf(claimStroke))
			}
			if len(app.board.LastHoverHint.Targets) == 0 {
				t.Fatal("HintClaimable produced no targets")
			}
			tIdx := app.board.LastHoverHint.Targets[0]
			tRow, tCol := tIdx/g.Cols, tIdx%g.Cols
			wantTargetFill := claimFill.Alpha(0.32).Over(pal.Slip)
			targetRect := classicCanvasCellRect(board, v, tRow, tCol)
			if n := countMatchingPixels(img, targetRect, 2, wantTargetFill, 14); n < 16 {
				t.Errorf("HintClaimable target (%d,%d) fill had only %d px near %s", tRow, tCol, n, hexOf(wantTargetFill))
			}
			if n := countMatchingPixels(img, targetRect, 2, claimStroke, 18); n < 12 {
				t.Errorf("HintClaimable target (%d,%d) outline had only %d px near %s", tRow, tCol, n, hexOf(claimStroke))
			}

			// Verify 9×9 scan frame uses dp.ScanFrame.
			scanTop := ui.Rect{
				X: board.X + float32(claimCol-4)*v.Cell - v.X + 8,
				Y: board.Y + float32(claimRow-4)*v.Cell - v.Y - 2,
				W: 9*v.Cell - 16,
				H: 4,
			}
			if n := countMatchingPixels(img, scanTop, 2, dp.ScanFrame, 18); n < 24 {
				t.Errorf("9×9 scan frame had only %d px near %s", n, hexOf(dp.ScanFrame))
			}

			// Verify top-left 9×9 scan badge uses palette bg, border, and text.
			badgeBox := ui.Rect{X: board.X + 12, Y: board.Y + 12, W: 166, H: 34}
			assertColorNear(t, "scan badge bg", sampleCanvasPixel(img, badgeBox.X+10, badgeBox.Y+17, 2), dp.ScanBadgeBg)
			badgeTop := ui.Rect{X: badgeBox.X + 8, Y: badgeBox.Y - 1, W: badgeBox.W - 16, H: 3}
			if n := countMatchingPixels(img, badgeTop, 2, dp.ScanBadgeBorder, 18); n < 16 {
				t.Errorf("scan badge border had only %d px near %s", n, hexOf(dp.ScanBadgeBorder))
			}
			badgeInner := ui.Rect{X: badgeBox.X + 4, Y: badgeBox.Y + 4, W: badgeBox.W - 8, H: badgeBox.H - 8}
			if n := countMatchingPixels(img, badgeInner, 2, dp.ScanBadgeText, 24); n < 24 {
				t.Errorf("scan badge text had only %d px near %s", n, hexOf(dp.ScanBadgeText))
			}

			// Verify all four popup capsules paint their palette bg, border, and text.
			for i, pc := range popupCells {
				st := dp.Popups[pc.kind]
				pop := app.duelPopups[i]
				px := board.X + (float32(pop.Col)+0.5)*v.Cell - v.X - float32(len([]rune(pop.Text)))*3.5
				py := board.Y + float32(pop.Row)*v.Cell - v.Y - 16
				px = max(board.X+8, min(board.X+board.W-120, px))
				py = max(board.Y+8, min(board.Y+board.H-28, py))
				pillW := float32(max(44, len([]rune(pop.Text))*9+14))
				pill := ui.Rect{X: px - 6, Y: py - 3, W: pillW, H: 22}
				pillTop := ui.Rect{X: pill.X + 6, Y: pill.Y - 1, W: pill.W - 12, H: 3}
				pillInner := ui.Rect{X: pill.X + 3, Y: pill.Y + 3, W: pill.W - 6, H: pill.H - 6}
				if n := countMatchingPixels(img, pillInner, 2, st.Bg, 18); n < 40 {
					t.Errorf("popup kind=%d missing pill bg %s (matched %d px)", pc.kind, hexOf(st.Bg), n)
				}
				wantBorder := st.Border.Alpha(0.8).Over(st.Bg)
				if n := countMatchingPixels(img, pillTop, 2, wantBorder, 28); n < 16 {
					t.Errorf("popup kind=%d missing pill border %s (matched %d px)", pc.kind, hexOf(wantBorder), n)
				}
				if n := countMatchingPixels(img, pillInner, 2, st.Text, 28); n < 16 {
					t.Errorf("popup kind=%d missing pill text %s (matched %d px)", pc.kind, hexOf(st.Text), n)
				}
			}

			// Verify EffectCell pulse uses activeTheme().Accent (from palette.go).
			wantAccent := app.activeTheme().Accent
			effectRect := ui.Rect{
				X: board.X + float32(effectCol)*v.Cell - v.X - 2,
				Y: board.Y + float32(effectRow)*v.Cell - v.Y - 2,
				W: v.Cell + 4,
				H: v.Cell + 4,
			}
			if n := countMatchingPixels(img, effectRect, 2, wantAccent, 20); n < 12 {
				t.Errorf("EffectCell pulse had only %d px near accent %s", n, hexOf(wantAccent))
			}

			// 2. Switch leader to MemberAnon and verify HintChordable uses her 代表色 highlighter.
			app.memberID = MemberAnon
			app.duelPopups = nil
			g.duel.ScanUntil = 0
			chordRow, chordCol, ok := findOrPrepareChordable(g, duelStageRow, duelStageCol)
			if !ok {
				t.Fatal("failed to stage a HintChordable duel cell")
			}
			app.board.center(g, chordRow, chordCol)
			tt.Frame()
			chx := board.X + (float32(chordCol)+0.5)*v.Cell - v.X
			chy := board.Y + (float32(chordRow)+0.5)*v.Cell - v.Y
			tt.Move(chx, chy)
			tt.Frame()
			if app.board.LastHoverHint.Kind != HintChordable {
				t.Fatalf("LastHoverHint.Kind = %v, want HintChordable", app.board.LastHoverHint.Kind)
			}
			img2 := tt.Image()
			chordFill, chordStroke := hoverHintColors(HintChordable, MemberAnon, dark)
			chordCell := g.Cell(chordRow, chordCol)
			wantChordCenter := chordFill.Alpha(0.24).Over(duelCellPaper(pal, chordCell.Owner))
			chordCenterRect := classicCanvasCellRect(board, v, chordRow, chordCol)
			if n := countMatchingPixels(img2, chordCenterRect, 2, wantChordCenter, 14); n < 16 {
				t.Errorf("HintChordable center (%d,%d) had only %d px near %s", chordRow, chordCol, n, hexOf(wantChordCenter))
			}
			if n := countMatchingPixels(img2, chordCenterRect, 2, chordStroke, 18); n < 12 {
				t.Errorf("HintChordable center (%d,%d) stroke had only %d px near %s", chordRow, chordCol, n, hexOf(chordStroke))
			}
			if len(app.board.LastHoverHint.Targets) == 0 {
				t.Fatal("HintChordable produced no targets")
			}
			ctIdx := app.board.LastHoverHint.Targets[0]
			ctRow, ctCol := ctIdx/g.Cols, ctIdx%g.Cols
			wantChordTarget := chordFill.Alpha(0.32).Over(pal.Slip)
			chordTargetRect := classicCanvasCellRect(board, v, ctRow, ctCol)
			if n := countMatchingPixels(img2, chordTargetRect, 2, wantChordTarget, 14); n < 16 {
				t.Errorf("HintChordable target (%d,%d) fill had only %d px near %s", ctRow, ctCol, n, hexOf(wantChordTarget))
			}
			if n := countMatchingPixels(img2, chordTargetRect, 2, chordStroke, 18); n < 12 {
				t.Errorf("HintChordable target (%d,%d) outline had only %d px near %s", ctRow, ctCol, n, hexOf(chordStroke))
			}
		})
	}
}

// TestSnapshotDuelPrompts renders day and night snapshots showcasing duel
// claim gold highlighter, 9×9 scan frame & corner badge, and all four
// floating score popups when MYGO_SNAPSHOT_DIR is set.
func TestSnapshotDuelPrompts(t *testing.T) {
	dir := os.Getenv("MYGO_SNAPSHOT_DIR")
	if dir == "" {
		t.Skip("set MYGO_SNAPSHOT_DIR to render snapshots")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, dark := range []bool{false, true} {
		suffix := "day"
		if dark {
			suffix = "night"
		}
		app := stageDuelScene(dark)
		g := app.game
		claimRow, claimCol, ok := findOrPrepareClaimable(g, duelStageRow, duelStageCol)
		if !ok {
			t.Fatal("failed to stage a HintClaimable duel cell for snapshot")
		}
		elapsed := g.duel.Elapsed
		g.duel.NextAI = 1e9
		g.duel.ScanUntil = elapsed + 6.4

		tt := ui.NewTester(app.view, 1280, 900)
		tt.SetScale(2)
		tt.Advance(time.Duration(1200-tt.Now().UnixMilli()%1200) * time.Millisecond)
		g.duel.Elapsed = elapsed
		app.elapsed = int(elapsed)
		app.lastTick = tt.Now()
		app.board.center(g, claimRow, claimCol)
		tt.Frame()

		board, ok := tt.Find("决斗棋盘")
		if !ok {
			t.Fatal("duel board not found")
		}
		v := app.board
		hx := board.X + (float32(claimCol)+0.5)*v.Cell - v.X
		hy := board.Y + (float32(claimRow)+0.5)*v.Cell - v.Y
		tt.Move(hx, hy)

		popNow := tt.Now()
		app.spawnDuelPopup(claimRow-5, claimCol-5, ActionResult{Kind: ActionFlagged, ScoreDelta: 75}, popNow)
		app.spawnDuelPopup(claimRow-5, claimCol+6, ActionResult{Kind: ActionLost, ScoreDelta: -100}, popNow)
		g.duel.PlayerCombo = 4
		app.spawnDuelPopup(claimRow+6, claimCol-5, ActionResult{Kind: ActionRevealed, RevealedCount: 9, ScoreDelta: 12}, popNow)
		g.duel.PlayerCombo = 1
		app.spawnDuelPopup(claimRow+6, claimCol+6, ActionResult{Kind: ActionRevealed, RevealedCount: 5, ScoreDelta: 6}, popNow)
		tt.Frame()

		path := filepath.Join(dir, fmt.Sprintf("duel-prompts-default-%s.png", suffix))
		f, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, tt.Image()); err != nil {
			f.Close()
			t.Fatal(err)
		}
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
	}
}

// advanceToStunPulsePeak advances the tester clock to the nearest millisecond
// where sin(ms/120) reaches its peak so the canvas stun border is rendered at
// full alert opacity deterministically.
func advanceToStunPulsePeak(tt *ui.Tester) float32 {
	base := tt.Now().UnixMilli()
	bestOffset := int64(0)
	bestSin := -2.0
	for off := int64(0); off < 755; off++ {
		s := math.Sin(float64(base+off) / 120.0)
		if s > bestSin {
			bestSin = s
			bestOffset = off
		}
	}
	if bestOffset > 0 {
		tt.Advance(time.Duration(bestOffset) * time.Millisecond)
	}
	return 0.45 + 0.55*float32(0.5+0.5*math.Sin(float64(tt.Now().UnixMilli())/120.0))
}

// Duel HUD badges (player score, AI score, stun, scan) and the canvas stun
// banner keep WCAG-AA contrast on both papers, share the scan note palette
// with duelPromptsFor, and leave zero raw colours in board.go and duelHUD.
func TestDuelHUDAndStunContrastAndScanReport(t *testing.T) {
	for _, dark := range []bool{false, true} {
		pal, dp, hud := paperFor(dark), duelPromptsFor(dark), duelHUDFor(dark)
		badges := []struct {
			name string
			c    badgeColors
		}{
			{"player", hud.Player},
			{"ai", hud.AI},
			{"stun", hud.Stun},
			{"scan", hud.Scan},
		}
		for _, b := range badges {
			if got := contrast(b.c.Text, b.c.Bg); got < minInkContrast {
				t.Errorf("dark=%v %s badge text %s on %s: contrast %.2f < %.1f",
					dark, b.name, hexOf(b.c.Text), hexOf(b.c.Bg), got, minInkContrast)
			}
		}
		if hud.Scan.Bg != dp.ScanBadgeBg || hud.Scan.Text != dp.ScanBadgeText {
			t.Errorf("dark=%v HUD scan badge (%s/%s) differs from duelPromptsFor (%s/%s)",
				dark, hexOf(hud.Scan.Bg), hexOf(hud.Scan.Text), hexOf(dp.ScanBadgeBg), hexOf(dp.ScanBadgeText))
		}
		if got := contrast(hud.Banner.Title, hud.Banner.Bg); got < minInkContrast {
			t.Errorf("dark=%v stun banner title %s on %s: contrast %.2f < %.1f",
				dark, hexOf(hud.Banner.Title), hexOf(hud.Banner.Bg), got, minInkContrast)
		}
		if got := contrast(hud.Banner.Sub, hud.Banner.Bg); got < minInkContrast {
			t.Errorf("dark=%v stun banner sub %s on %s: contrast %.2f < %.1f",
				dark, hexOf(hud.Banner.Sub), hexOf(hud.Banner.Bg), got, minInkContrast)
		}
		if hud.Banner.Border != pal.BoomInk || hud.Banner.Pulse != pal.BoomInk {
			t.Errorf("dark=%v stun banner border/pulse = (%s, %s), want BoomInk %s",
				dark, hexOf(hud.Banner.Border), hexOf(hud.Banner.Pulse), hexOf(pal.BoomInk))
		}
	}

	rep, err := scanreport.Scan(".")
	if err != nil {
		t.Fatalf("scanreport.Scan failed: %v", err)
	}
	if len(rep.Copy) != 0 {
		t.Errorf("copy violations = %d, want 0:\n%s", len(rep.Copy), rep)
	}
	duelHUDForbidden := []string{"#173B49", "#55B2D6", "#3B2740", "#F07AA4", "#701F2B", "#3C301D", "#F5D99C"}
	boardColors := 0
	for _, f := range rep.Colors {
		base := filepath.Base(f.Path)
		if base == "board.go" {
			boardColors++
			t.Errorf("board.go:%d still defines raw colour %s", f.Line, f.Text)
		}
		if base == "main.go" {
			upper := strings.ToUpper(f.Text)
			for _, hex := range duelHUDForbidden {
				if strings.Contains(upper, hex) {
					t.Errorf("main.go:%d still defines migrated duelHUD raw colour %s", f.Line, f.Text)
				}
			}
		}
	}
	if boardColors != 0 {
		t.Errorf("board.go raw colours = %d, want 0", boardColors)
	}
	if len(rep.Colors) > 7 {
		t.Errorf("total raw colours = %d, want <= 7 (#23 remaining at most):\n%s", len(rep.Colors), rep)
	}
}

// Rendered duel HUD badges (player, AI, stun, scan), the canvas stun warning
// banner (bg, border, title, sub), and the pulsing canvas stun border all
// paint their palette colours in both day and night themes.
func TestDuelHUDAndStunPaintPaletteColors(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(fmt.Sprintf("dark=%v", dark), func(t *testing.T) {
			app := stageDuelScene(dark)
			g := app.game
			elapsed := g.duel.Elapsed
			g.duel.NextAI = 1e9
			g.duel.StunnedUntil = elapsed + 2.1
			g.duel.ScanUntil = elapsed + 6.4

			tt := ui.NewTester(app.view, 1280, 900)
			tt.SetScale(2)
			pulse := advanceToStunPulsePeak(tt)
			g.duel.Elapsed = elapsed
			app.elapsed = int(elapsed)
			app.lastTick = tt.Now()
			tt.Frame()
			app.board.center(g, duelStageRow, duelStageCol)
			tt.Frame()

			img := tt.Image()
			pal, hud := paperFor(dark), duelHUDFor(dark)

			// 1. Verify all four duel HUD badges paint palette bg and text.
			badgeCases := []struct {
				name  string
				label string
				style badgeColors
			}{
				{"player", fmt.Sprintf("你 %d", g.duel.PlayerScore), hud.Player},
				{"ai", fmt.Sprintf("高松灯 %d", g.duel.AIScore), hud.AI},
				{"stun", fmt.Sprintf("硬直 %.1f 秒", g.duel.StunnedUntil-g.duel.Elapsed), hud.Stun},
				{"scan", fmt.Sprintf("扫描 %.1f 秒", g.duel.ScanUntil-g.duel.Elapsed), hud.Scan},
			}
			for _, tc := range badgeCases {
				r, ok := tt.Find(tc.label)
				if !ok {
					t.Fatalf("%s badge %q not found", tc.name, tc.label)
				}
				if n := countMatchingPixels(img, r, 2, tc.style.Bg, 14); n < 40 {
					t.Errorf("%s badge %q missing palette bg %s (matched %d px)", tc.name, tc.label, hexOf(tc.style.Bg), n)
				}
				if n := countMatchingPixels(img, r, 2, tc.style.Text, 24); n < 12 {
					t.Errorf("%s badge %q missing palette text %s (matched %d px)", tc.name, tc.label, hexOf(tc.style.Text), n)
				}
			}

			// 2. Verify canvas duel-stun-banner card bg, border, title, and subtitle.
			titleLabel := fmt.Sprintf("踩雷硬直中 · %.1f 秒", g.duel.StunnedUntil-g.duel.Elapsed)
			subLabel := "按 E 发动「创可贴指引」立即解除硬直"
			titleRect, ok := tt.Find(titleLabel)
			if !ok {
				t.Fatalf("stun banner title %q not found", titleLabel)
			}
			subRect, ok := tt.Find(subLabel)
			if !ok {
				t.Fatalf("stun banner subtitle %q not found", subLabel)
			}
			leftText := min(titleRect.X, subRect.X)
			rightText := max(titleRect.X+titleRect.W, subRect.X+subRect.W)
			// Sample strictly inside the card's 18px horizontal padding with tight
			// tolerance so dayPaper.Slip (#ECE4D3) cannot match hud.Banner.Bg (#F4E0DD).
			assertColorNear(t, "stun banner bg",
				sampleCanvasPixel(img, leftText-10, (titleRect.Y+subRect.Y+subRect.H)/2, 2),
				hud.Banner.Bg)
			innerPad := ui.Rect{
				X: leftText - 14,
				Y: titleRect.Y,
				W: 10,
				H: (subRect.Y + subRect.H) - titleRect.Y,
			}
			if n := countMatchingPixels(img, innerPad, 2, hud.Banner.Bg, 4); n < 40 {
				t.Errorf("stun banner inner padding missing palette bg %s (matched %d px)", hexOf(hud.Banner.Bg), n)
			}
			cardTopBorder := ui.Rect{X: leftText, Y: titleRect.Y - 13, W: rightText - leftText, H: 6}
			if n := countMatchingPixels(img, cardTopBorder, 2, hud.Banner.Border, 20); n < 40 {
				t.Errorf("stun banner missing palette border %s (matched %d px)", hexOf(hud.Banner.Border), n)
			}
			if n := countMatchingPixels(img, titleRect, 2, hud.Banner.Title, 24); n < 24 {
				t.Errorf("stun banner title missing palette text %s (matched %d px)", hexOf(hud.Banner.Title), n)
			}
			if n := countMatchingPixels(img, subRect, 2, hud.Banner.Sub, 24); n < 24 {
				t.Errorf("stun banner sub missing palette text %s (matched %d px)", hexOf(hud.Banner.Sub), n)
			}

			// 3. Verify canvas pulsing stun border paints hud.Banner.Pulse.
			board, ok := tt.Find("决斗棋盘")
			if !ok {
				t.Fatal("duel board not found")
			}
			wantPulse := hud.Banner.Pulse.Alpha(pulse).Over(pal.Board)
			topEdge := ui.Rect{X: board.X + 20, Y: board.Y, W: board.W - 40, H: 8}
			if n := countMatchingPixels(img, topEdge, 2, wantPulse, 20); n < 80 {
				t.Errorf("canvas stun pulse border missing palette stroke %s (matched %d px)", hexOf(wantPulse), n)
			}
		})
	}
}

// TestSnapshotDuelHUDStun renders day and night snapshots showcasing the duel
// HUD score/stun/scan badges, canvas stun banner, pulsing stun border, and
// active 9×9 scan overlay when MYGO_SNAPSHOT_DIR is set.
func TestSnapshotDuelHUDStun(t *testing.T) {
	dir := os.Getenv("MYGO_SNAPSHOT_DIR")
	if dir == "" {
		t.Skip("set MYGO_SNAPSHOT_DIR to render snapshots")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, dark := range []bool{false, true} {
		suffix := "day"
		if dark {
			suffix = "night"
		}
		app := stageDuelScene(dark)
		g := app.game
		elapsed := g.duel.Elapsed
		g.duel.NextAI = 1e9
		g.duel.StunnedUntil = elapsed + 2.1
		g.duel.ScanUntil = elapsed + 6.4

		tt := ui.NewTester(app.view, 1280, 900)
		tt.SetScale(2)
		advanceToStunPulsePeak(tt)
		g.duel.Elapsed = elapsed
		app.elapsed = int(elapsed)
		app.lastTick = tt.Now()
		tt.Frame()
		app.board.center(g, duelStageRow, duelStageCol)
		tt.Frame()

		board, ok := tt.Find("决斗棋盘")
		if !ok {
			t.Fatal("duel board not found")
		}
		v := app.board
		hx := board.X + (float32(duelStageCol)+0.5)*v.Cell - v.X
		hy := board.Y + (float32(duelStageRow)+0.5)*v.Cell - v.Y
		tt.Move(hx, hy)
		tt.Frame()

		path := filepath.Join(dir, fmt.Sprintf("duel-hud-stun-default-%s.png", suffix))
		f, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, tt.Image()); err != nil {
			f.Close()
			t.Fatal(err)
		}
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
	}
}
