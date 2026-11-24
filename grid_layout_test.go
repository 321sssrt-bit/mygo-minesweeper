package main

import (
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func assertRectEqual(t *testing.T, label string, got, want ui.Rect) {
	t.Helper()
	if got != want {
		t.Fatalf("%s shifted: got %+v, want %+v", label, got, want)
	}
}

func TestGridLayoutFixedPitchAndCenteredInStage(t *testing.T) {
	for _, tc := range []struct {
		name          string
		diffIdx       int
		width, height int
	}{
		{"9x9 small", 0, 960, 640},
		{"9x9 default", 0, 1280, 900},
		{"9x9 large", 0, 1680, 1050},
		{"16x16 default", 1, 1280, 900},
		{"16x16 large", 1, 1680, 1050},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := newMinesweeperApp(42)
			app.diffIdx = tc.diffIdx
			app.resetGame(true)
			tt := ui.NewTester(app.view, tc.width, tc.height)
			tt.Frame()

			if app.classicOnCanvas() {
				t.Fatalf("expected grid surface for %s", tc.name)
			}
			if _, ok := tt.Find("经典棋盘"); ok {
				t.Fatalf("unexpected canvas mounted for %s", tc.name)
			}

			r00 := mustFindCell(t, tt, "cell-0-0")
			r01 := mustFindCell(t, tt, "cell-0-1")
			r10 := mustFindCell(t, tt, "cell-1-0")
			lastLabel := fmt.Sprintf("cell-%d-%d", app.game.Rows-1, app.game.Cols-1)
			rLast := mustFindCell(t, tt, lastLabel)

			if abs32(r00.W-gridCellSize) > 0.05 || abs32(r00.H-gridCellSize) > 0.05 {
				t.Fatalf("cell size = %.4fx%.4f, want %.1fx%.1f", r00.W, r00.H, gridCellSize, gridCellSize)
			}
			if pitchX, pitchY := r01.X-r00.X, r10.Y-r00.Y; abs32(pitchX-standardCellPitch) > 0.05 || abs32(pitchY-standardCellPitch) > 0.05 {
				t.Fatalf("grid pitch = (%.4f, %.4f), want (%.1f, %.1f)", pitchX, pitchY, standardCellPitch, standardCellPitch)
			}

			// Verify grid is centered horizontally and vertically within the stage card
			// (using the HUD reset bar as the horizontal span and bottom of HUD to window bottom).
			stageW, stageH := app.estimatedStageSize(float32(tc.width), float32(tc.height))
			if app.stageChromeW <= 0 || app.stageChromeH <= 0 {
				t.Fatalf("stage chrome not measured: W=%.1f H=%.1f", app.stageChromeW, app.stageChromeH)
			}
			boardLeft := r00.X - gridBoardPad
			boardRight := rLast.X + rLast.W + gridBoardPad
			boardTop := r00.Y - gridBoardPad
			boardBottom := rLast.Y + rLast.H + gridBoardPad
			boardW := boardRight - boardLeft
			boardH := boardBottom - boardTop

			// Stage bounds sit flush at the right/bottom inside root padding (14) + board card padding/border (13).
			stageRight := float32(tc.width) - 27
			stageBottom := float32(tc.height) - 27
			stageLeft := stageRight - stageW
			stageTop := stageBottom - stageH

			leftMargin := boardLeft - stageLeft
			rightMargin := stageRight - boardRight
			topMargin := boardTop - stageTop
			bottomMargin := stageBottom - boardBottom

			if leftMargin < gridStageMargin-1 || rightMargin < gridStageMargin-1 || topMargin < gridStageMargin-1 || bottomMargin < gridStageMargin-1 {
				t.Fatalf("grid lacks paper margin around board (%.1fx%.1f): L=%.1f R=%.1f T=%.1f B=%.1f",
					boardW, boardH, leftMargin, rightMargin, topMargin, bottomMargin)
			}
			if abs32(leftMargin-rightMargin) > 2 {
				t.Fatalf("grid not horizontally centered: L=%.1f R=%.1f (stageW=%.1f boardW=%.1f)", leftMargin, rightMargin, stageW, boardW)
			}
			if abs32(topMargin-bottomMargin) > 2 {
				t.Fatalf("grid not vertically centered: T=%.1f B=%.1f (stageH=%.1f boardH=%.1f)", topMargin, bottomMargin, stageH, boardH)
			}

			// Scroll wheel on the grid must not scroll or shift cell coordinates.
			tt.Scroll(r00.X+r00.W/2, r00.Y+r00.H/2, 80, 120)
			tt.Frame()
			assertRectEqual(t, "cell-0-0 after scroll", mustFindCell(t, tt, "cell-0-0"), r00)
			assertRectEqual(t, lastLabel+" after scroll", mustFindCell(t, tt, lastLabel), rLast)
		})
	}
}

func TestGridLayoutStableAcrossGameplayPopoversAndChat(t *testing.T) {
	app := newMinesweeperApp(42)
	app.game = NewGame(Difficulty{
		ID:           "stable-grid",
		Label:        "羽丘天文部",
		Song:         "迷星叫",
		Gameplay:     GameplayClassic,
		Rows:         9,
		Cols:         9,
		Mines:        5,
		SkillCharges: 2,
	}, true, MemberTomori, 42)
	app.game.FirstClick = true
	app.game.Status = StatusPlaying
	for col := 0; col < 5; col++ {
		app.game.Cell(0, col).IsMine = true
	}
	app.game.RecomputeAdjacency()

	tt := ui.NewTester(app.view, 1280, 900)
	tt.Frame()

	wantFirst := mustFindCell(t, tt, "cell-0-0")
	wantLast := mustFindCell(t, tt, "cell-8-8")
	assertStable := func(step string) {
		t.Helper()
		if app.classicOnCanvas() {
			t.Fatalf("%s unexpectedly switched grid to canvas", step)
		}
		gotFirst := mustFindCell(t, tt, "cell-0-0")
		gotLast := mustFindCell(t, tt, "cell-8-8")
		assertRectEqual(t, step+" cell-0-0", gotFirst, wantFirst)
		assertRectEqual(t, step+" cell-8-8", gotLast, wantLast)
	}

	// 1. Reveal a numbered safe cell (avoids winning on move 1 so skill can still fire).
	if err := tt.Click("cell-1-0"); err != nil {
		t.Fatalf("reveal cell-1-0: %v", err)
	}
	tt.Advance(rippleMaxDelay + rippleDuration + 20*time.Millisecond)
	assertStable("after reveal")

	// 2. Plant and remove a band-aid flag.
	if err := tt.RightClick("cell-0-0"); err != nil {
		t.Fatalf("flag cell-0-0: %v", err)
	}
	if !app.game.Cell(0, 0).Flagged {
		t.Fatal("expected cell-0-0 flagged")
	}
	assertStable("after flag")

	// 3. Trigger band skill via E (shows quote strip overlay, updates banner quote & chat).
	tt.Key(0, ui.KeyE)
	tt.Frame()
	if app.game.SkillUsesLeft != 1 {
		t.Fatalf("expected skill used, remaining=%d", app.game.SkillUsesLeft)
	}
	assertStable("after skill")
	tt.Advance(quoteStripDuration + 50*time.Millisecond)
	assertStable("after quote strip exit")

	// 4. Open and close setlist and settings popovers.
	if err := tt.Click("歌单"); err != nil {
		t.Fatalf("open setlist popover: %v", err)
	}
	assertStable("with setlist popover open")
	tt.Key(0, ui.KeyEscape)
	tt.Advance(popoverExitDuration + 20*time.Millisecond)
	assertStable("after setlist popover close")

	if err := tt.Click("设置"); err != nil {
		t.Fatalf("open settings popover: %v", err)
	}
	assertStable("with settings popover open")
	tt.Key(0, ui.KeyEscape)
	tt.Advance(popoverExitDuration + 20*time.Millisecond)
	assertStable("after settings popover close")

	// 5. Grow band group chat past scroll capacity and test a very long banner quote.
	for i := range 16 {
		app.pushChat("千早爱音", FindMember(MemberAnon).ColorHex, fmt.Sprintf("排练室群聊长消息 #%d：伦敦留学实力派正在疯狂刷屏确认棋盘位置纹丝不动！", i+1))
	}
	app.quote = "「这是一段故意写得很长很长很长很长很长很长很长很长很长很长很长很长的台词，确认单行显示不会把下方棋盘往下挤！」"
	tt.Frame()
	tt.Advance(chatNoteEnterDuration + 30*time.Millisecond)
	assertStable("after chat growth and long quote")
}

func TestGridLayoutWindowResizeKeepsPitchAndSelectsSurfaceByFit(t *testing.T) {
	// 1. Resizing window across viewports where 9x9 fits keeps cell size & pitch identical.
	app := newMinesweeperApp(42)
	tt := ui.NewTester(app.view, 1280, 900)
	tt.Frame()

	for _, vp := range []struct{ w, h int }{
		{1280, 900},
		{1680, 1050},
		{960, 640},
		{1400, 860},
	} {
		tt.SetSize(vp.w, vp.h)
		tt.Frame()
		if app.classicOnCanvas() {
			t.Fatalf("9x9 unexpectedly fell back to canvas at %dx%d", vp.w, vp.h)
		}
		r00 := mustFindCell(t, tt, "cell-0-0")
		r01 := mustFindCell(t, tt, "cell-0-1")
		r10 := mustFindCell(t, tt, "cell-1-0")
		if abs32(r00.W-gridCellSize) > 0.05 || abs32(r00.H-gridCellSize) > 0.05 {
			t.Fatalf("at %dx%d cell size = %.4fx%.4f, want %.1fx%.1f", vp.w, vp.h, r00.W, r00.H, gridCellSize, gridCellSize)
		}
		if abs32((r01.X-r00.X)-standardCellPitch) > 0.05 || abs32((r10.Y-r00.Y)-standardCellPitch) > 0.05 {
			t.Fatalf("at %dx%d pitch = (%.4f, %.4f), want %.1f", vp.w, vp.h, r01.X-r00.X, r10.Y-r00.Y, standardCellPitch)
		}
	}

	// 2. Every shipped classic track across all three snapshot viewports matches stage-fit rule.
	for _, vp := range snapshotViewports {
		for i, diff := range Difficulties {
			if diff.Gameplay != GameplayClassic {
				continue
			}
			t.Run(fmt.Sprintf("%s/%s", diff.ID, vp.name), func(t *testing.T) {
				a := newMinesweeperApp(42)
				a.diffIdx = i
				a.resetGame(true)
				vt := ui.NewTester(a.view, vp.width, vp.height)
				vt.Frame()
				// Settle any one-shot measurement adjustment on unusual viewports.
				vt.Advance(5 * time.Millisecond)

				stageW, stageH := a.estimatedStageSize(float32(vp.width), float32(vp.height))
				wantCanvas := !classicGridFitsStage(diff.Rows, diff.Cols, stageW, stageH)
				_, hasCanvas := vt.Find("经典棋盘")
				_, hasGrid := vt.Find("cell-0-0")
				if a.classicOnCanvas() != wantCanvas || hasCanvas != wantCanvas || hasGrid == wantCanvas {
					t.Fatalf("%s (%dx%d) at %s (%dx%d, stage %.0fx%.0f): onCanvas=%v hasCanvas=%v hasGrid=%v, wantCanvas=%v",
						diff.ID, diff.Rows, diff.Cols, vp.name, vp.width, vp.height, stageW, stageH,
						a.classicOnCanvas(), hasCanvas, hasGrid, wantCanvas)
				}
			})
		}
	}

	// 3. Exact stage boundary: largest fitting grid stays Grid, +1 row or +1 col falls back to Canvas.
	probeApp := newMinesweeperApp(42)
	probeTT := ui.NewTester(probeApp.view, 1280, 900)
	probeTT.Frame()
	stageW, stageH := probeApp.estimatedStageSize(1280, 900)
	inset := gridBoardPad*2 + gridStageMargin*2
	maxCols := int((stageW + gridCellGap - inset) / standardCellPitch)
	maxRows := int((stageH + gridCellGap - inset) / standardCellPitch)

	for _, tc := range []struct {
		name       string
		rows, cols int
		wantCanvas bool
	}{
		{"exact fit", maxRows, maxCols, false},
		{"one row over", maxRows + 1, maxCols, true},
		{"one col over", maxRows, maxCols + 1, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := newMinesweeperApp(42)
			a.game = NewGame(Difficulty{
				ID:       "bound-" + tc.name,
				Gameplay: GameplayClassic,
				Rows:     tc.rows,
				Cols:     tc.cols,
				Mines:    10,
			}, false, MemberTomori, 42)
			bt := ui.NewTester(a.view, 1280, 900)
			bt.Frame()
			bt.Advance(5 * time.Millisecond)

			_, hasCanvas := bt.Find("经典棋盘")
			_, hasGrid := bt.Find("cell-0-0")
			if hasCanvas != tc.wantCanvas || hasGrid == tc.wantCanvas {
				t.Fatalf("%s (%dx%d in stage %.1fx%.1f): hasCanvas=%v hasGrid=%v, wantCanvas=%v",
					tc.name, tc.rows, tc.cols, stageW, stageH, hasCanvas, hasGrid, tc.wantCanvas)
			}
		})
	}

	// 4. Opening the memes drawer without changing track or window size must NOT flip surface.
	drawerApp := newMinesweeperApp(42)
	drawerApp.diffIdx = 1 // 16x16 fits at 1280x900
	drawerApp.resetGame(true)
	drawerTT := ui.NewTester(drawerApp.view, 1280, 900)
	drawerTT.Frame()
	if drawerApp.classicOnCanvas() {
		t.Fatal("16x16 should start on grid at 1280x900")
	}
	if err := drawerTT.Click("名场面图鉴"); err != nil {
		t.Fatalf("open memes drawer: %v", err)
	}
	drawerTT.Advance(drawerOpenDuration + 30*time.Millisecond)
	if drawerApp.classicOnCanvas() {
		t.Fatal("opening memes drawer unexpectedly switched 16x16 grid to canvas")
	}
}

func TestGridLayoutMidGameResizePreservesStateAcrossSurfaceSwitch(t *testing.T) {
	app := newMinesweeperApp(42)
	app.diffIdx = 1 // 16x16 《碧天伴走》: Grid at 1280x900, Canvas at 960x640
	app.resetGame(true)
	app.game.FirstClick = true
	app.game.Status = StatusPlaying
	for i := range app.game.Cells {
		app.game.Cells[i].IsMine = false
	}
	app.game.Cell(0, 0).IsMine = true
	app.game.Cell(0, 1).IsMine = true
	app.game.Cell(15, 15).IsMine = true
	app.game.Mines = 3
	app.game.RecomputeAdjacency()

	tt := ui.NewTester(app.view, 1280, 900)
	tt.Frame()

	if app.classicOnCanvas() {
		t.Fatal("expected 16x16 to start on grid at 1280x900")
	}
	if err := tt.Click("cell-1-1"); err != nil {
		t.Fatalf("reveal cell-1-1 on grid: %v", err)
	}
	if err := tt.RightClick("cell-0-0"); err != nil {
		t.Fatalf("flag cell-0-0 on grid: %v", err)
	}
	tt.Advance(2300 * time.Millisecond)

	type cellSnap struct {
		revealed, flagged, questioned, shield bool
	}
	snapshotCells := func(g *Game) []cellSnap {
		out := make([]cellSnap, len(g.Cells))
		for i, c := range g.Cells {
			out[i] = cellSnap{c.Revealed, c.Flagged, c.Questioned, c.ShieldSaved}
		}
		return out
	}

	beforeCells := snapshotCells(app.game)
	beforeMoves := app.game.Moves
	beforeSafe := app.game.RevealedSafe
	beforeFlags := app.game.FlagsPlaced
	beforeElapsed := app.elapsed
	if beforeSafe == 0 || beforeFlags != 1 || beforeElapsed < 2 {
		t.Fatalf("precondition failed: safe=%d flags=%d elapsed=%d", beforeSafe, beforeFlags, beforeElapsed)
	}

	// Shrink window so 16x16 no longer fits -> switches to Canvas mid-game without losing state.
	tt.SetSize(960, 640)
	tt.Frame()
	if !app.classicOnCanvas() {
		t.Fatal("expected 16x16 to fall back to canvas at 960x640")
	}
	if _, ok := tt.Find("经典棋盘"); !ok {
		t.Fatal("classic canvas not mounted after shrink")
	}
	if _, ok := tt.Find("cell-0-0"); ok {
		t.Fatal("grid cells still mounted after shrink to canvas")
	}
	if app.board.Cell != standardCellPitch {
		t.Fatalf("canvas opened at cell=%.1f, want standard pitch %.1f", app.board.Cell, standardCellPitch)
	}
	afterShrinkCells := snapshotCells(app.game)
	for i := range beforeCells {
		if beforeCells[i] != afterShrinkCells[i] {
			t.Fatalf("cell %d mutated on grid->canvas resize: before=%+v after=%+v", i, beforeCells[i], afterShrinkCells[i])
		}
	}
	if app.game.Status != StatusPlaying || app.game.Moves != beforeMoves || app.game.RevealedSafe != beforeSafe ||
		app.game.FlagsPlaced != beforeFlags || app.elapsed < beforeElapsed {
		t.Fatalf("game stats mutated on grid->canvas resize: status=%v moves=%d/%d safe=%d/%d flags=%d/%d elapsed=%d/%d",
			app.game.Status, app.game.Moves, beforeMoves, app.game.RevealedSafe, beforeSafe, app.game.FlagsPlaced, beforeFlags, app.elapsed, beforeElapsed)
	}

	// Flag another cell while on Canvas, then enlarge back to Grid and confirm continuity.
	x, y := classicSurfaceCellAt(t, tt, app, 0, 1)
	tt.RightClickAt(x, y)
	if !app.game.Cell(0, 1).Flagged || app.game.FlagsPlaced != 2 {
		t.Fatalf("failed to flag (0,1) on canvas after shrink: flagged=%v total=%d", app.game.Cell(0, 1).Flagged, app.game.FlagsPlaced)
	}
	beforeExpandCells := snapshotCells(app.game)

	tt.SetSize(1280, 900)
	tt.Frame()
	if app.classicOnCanvas() {
		t.Fatal("expected 16x16 to switch back to grid at 1280x900")
	}
	if _, ok := tt.Find("cell-0-0"); !ok {
		t.Fatal("grid cells missing after enlarging back to 1280x900")
	}
	afterExpandCells := snapshotCells(app.game)
	for i := range beforeExpandCells {
		if beforeExpandCells[i] != afterExpandCells[i] {
			t.Fatalf("cell %d mutated on canvas->grid resize: before=%+v after=%+v", i, beforeExpandCells[i], afterExpandCells[i])
		}
	}
	if app.game.FlagsPlaced != 2 || !app.game.Cell(0, 0).Flagged || !app.game.Cell(0, 1).Flagged || !app.game.Cell(1, 1).Revealed {
		t.Fatalf("state lost returning to grid: flags=%d (0,0)=%v (0,1)=%v (1,1)=%v",
			app.game.FlagsPlaced, app.game.Cell(0, 0).Flagged, app.game.Cell(0, 1).Flagged, app.game.Cell(1, 1).Revealed)
	}
}

func TestGridAndCanvasSlipAppearanceParity(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(fmt.Sprintf("dark=%v", dark), func(t *testing.T) {
			gridApp, coords := stageClassicCanvasAllStatesBoard(dark)
			canvasApp, _ := stageClassicCanvasAllStatesBoard(dark)

			gridTT := ui.NewTester(gridApp.view, 1280, 900)
			gridTT.SetScale(2)
			gridTT.Frame()

			canvasTT := ui.NewTester(classicCanvasTestView(canvasApp), 1280, 900)
			canvasTT.SetScale(2)
			canvasTT.Frame()

			if gridApp.classicOnCanvas() {
				t.Fatal("6x6 board should render on widget grid")
			}
			boardRect, ok := canvasTT.Find("经典棋盘")
			if !ok {
				t.Fatal("classic canvas missing")
			}

			r00 := mustFindCell(t, gridTT, "cell-0-0")
			r01 := mustFindCell(t, gridTT, "cell-0-1")
			c00 := classicCanvasCellRect(boardRect, canvasApp.board, 0, 0)
			c01 := classicCanvasCellRect(boardRect, canvasApp.board, 0, 1)

			if r00.W != c00.W || r00.H != c00.H {
				t.Fatalf("tile size mismatch: grid=%.1fx%.1f canvas=%.1fx%.1f", r00.W, r00.H, c00.W, c00.H)
			}
			if r01.X-r00.X != c01.X-c00.X {
				t.Fatalf("tile pitch mismatch: grid=%.1f canvas=%.1f", r01.X-r00.X, c01.X-c00.X)
			}

			gridImg := gridTT.Image()
			canvasImg := canvasTT.Image()
			pal := paperFor(dark)

			gridTile := func(name string) ui.Rect {
				at := coords[name]
				return mustFindCell(t, gridTT, fmt.Sprintf("cell-%d-%d", at[0], at[1]))
			}
			canvasTile := func(name string) ui.Rect {
				at := coords[name]
				return classicCanvasCellRect(boardRect, canvasApp.board, at[0], at[1])
			}

			for _, name := range []string{"slip", "empty"} {
				gt := gridTile(name)
				ct := canvasTile(name)
				gp := sampleCanvasPixel(gridImg, gt.X+gt.W/2, gt.Y+gt.H/2, 2)
				cp := sampleCanvasPixel(canvasImg, ct.X+ct.W/2, ct.Y+ct.H/2, 2)
				assertColorNear(t, name+" grid vs canvas center", gp, cp)
			}

			// Verify ruled grid line in the 2px gap between cells matches on both surfaces.
			gapGrid := sampleCanvasPixel(gridImg, r00.X+r00.W+1, r00.Y+r00.H/2, 2)
			gapCanvas := sampleCanvasPixel(canvasImg, c00.X+c00.W+1, c00.Y+c00.H/2, 2)
			assertColorNear(t, "gap ruled line grid vs canvas", gapGrid, gapCanvas)

			// Verify prop/ink presence matches across key states on grid.
			flagTile := gridTile("flag")
			if n := countMatchingPixels(gridImg, flagTile, 2, pal.BandAid, 18); n < 20 {
				t.Errorf("grid flag had only %d band-aid pixels", n)
			}
			shieldTile := gridTile("shield")
			if n := countMatchingPixels(gridImg, shieldTile, 2, memberImage(MemberSoyorin), 18); n < 20 {
				t.Errorf("grid shield had only %d Soyorin gold pixels", n)
			}
			wrongTile := gridTile("wrong")
			if n := countMatchingPixels(gridImg, wrongTile, 2, pal.BoomInk, 24); n < 16 {
				t.Errorf("grid wrong flag had only %d boom ink pixels", n)
			}
			hitTile := gridTile("hit")
			boomBg := pal.Boom.Over(pal.Board)
			assertColorNear(t, "grid hit mine bg", sampleCanvasPixel(gridImg, hitTile.X+2, hitTile.Y+2, 2), boomBg)
		})
	}
}

func TestGridLayoutResizeEdgeCases(t *testing.T) {
	t.Run("wrapped toolbar shrinks stage and rebuilds to canvas in idle state", func(t *testing.T) {
		app := newMinesweeperApp(42)
		tt := ui.NewTester(app.view, 1280, 900)
		tt.Frame()
		if app.classicOnCanvas() {
			t.Fatal("9x9 should start on grid at 1280x900")
		}

		// Narrow window wraps the top toolbar so stage height drops below 9x9's footprint
		// even though the previous wide chrome would predict a fit; Invalidate must
		// rebuild the widget tree to canvas without any stopwatch timer running.
		_, needH := gridBoardOuterSize(app.game.Rows, app.game.Cols)
		narrowH := int(math.Ceil(float64(needH + app.stageChromeH + 10)))
		tt.SetSize(650, narrowH)
		tt.Frame()
		if !app.classicOnCanvas() {
			t.Fatalf("9x9 should switch to canvas when wrapped toolbar shrinks stage at 650x%d", narrowH)
		}
		if _, hasCanvas := tt.Find("经典棋盘"); !hasCanvas {
			t.Fatal("widget tree stayed on grid after measureClassicStage flipped to canvas")
		}
		if _, hasGrid := tt.Find("cell-0-0"); hasGrid {
			t.Fatal("stale grid widgets remained mounted after canvas fallback")
		}
	})

	t.Run("stage fit at 800x900 is path-independent and never shrinks sidebar", func(t *testing.T) {
		// Shrinking directly from 1280x900 to 800x900.
		appDown := newMinesweeperApp(42)
		appDown.diffIdx = 1
		appDown.resetGame(true)
		ttDown := ui.NewTester(appDown.view, 1280, 900)
		ttDown.Frame()
		ttDown.SetSize(800, 900)
		ttDown.Frame()

		// Shrinking to 640x900 first, then enlarging to 800x900.
		appUp := newMinesweeperApp(42)
		appUp.diffIdx = 1
		appUp.resetGame(true)
		ttUp := ui.NewTester(appUp.view, 1280, 900)
		ttUp.Frame()
		ttUp.SetSize(640, 900)
		ttUp.Frame()
		ttUp.SetSize(800, 900)
		ttUp.Frame()

		if !appDown.classicOnCanvas() || !appUp.classicOnCanvas() {
			t.Fatalf("16x16 at 800x900 must use canvas regardless of path: down=%v up=%v",
				appDown.classicOnCanvas(), appUp.classicOnCanvas())
		}
		if _, ok := ttUp.Find("经典棋盘"); !ok {
			t.Fatal("enlarging 640x900 -> 800x900 failed to mount canvas")
		}
		if _, ok := ttUp.Find("cell-0-0"); ok {
			t.Fatal("enlarging 640x900 -> 800x900 left grid mounted by crushing sidebar")
		}
	})

	t.Run("resizing while memes drawer is open rechecks once drawer closes", func(t *testing.T) {
		// Measure this platform's unwrapped wide chrome so targetH sits comfortably
		// above 16x16's footprint at width 1280, yet below it under wrapped narrow chrome.
		probe := newMinesweeperApp(42)
		probe.diffIdx = 1
		probe.resetGame(true)
		probeTT := ui.NewTester(probe.view, 1280, 900)
		probeTT.Frame()
		wideChromeH := probe.stageChromeH
		_, needH := gridBoardOuterSize(probe.game.Rows, probe.game.Cols)
		targetH := int(math.Ceil(float64(needH + wideChromeH + 12)))

		app := newMinesweeperApp(42)
		app.diffIdx = 1
		app.resetGame(true)
		tt := ui.NewTester(app.view, 640, 640)
		tt.Frame()
		if !app.classicOnCanvas() {
			t.Fatal("16x16 should start on canvas at 640x640")
		}
		if float32(targetH)-app.stageChromeH >= needH {
			t.Fatalf("precondition failed: narrow wrapped chrome %.1f did not exceed wide chrome %.1f enough for targetH=%d",
				app.stageChromeH, wideChromeH, targetH)
		}

		if err := tt.Click("名场面图鉴"); err != nil {
			t.Fatalf("open memes drawer: %v", err)
		}
		tt.Advance(drawerOpenDuration + 20*time.Millisecond)

		tt.SetSize(1280, targetH)
		tt.Frame()

		if err := tt.Click("名场面图鉴"); err != nil {
			t.Fatalf("close memes drawer: %v", err)
		}
		tt.Advance(drawerCloseDuration + 30*time.Millisecond)
		tt.Frame()

		if app.classicOnCanvas() {
			t.Fatalf("16x16 stayed stuck on canvas after closing memes drawer at 1280x%d", targetH)
		}
		if _, ok := tt.Find("cell-0-0"); !ok {
			t.Fatalf("grid cells not mounted after closing memes drawer at 1280x%d", targetH)
		}
	})

	t.Run("canvas to grid switch during skill glide preserves skill target pulses", func(t *testing.T) {
		app := newMinesweeperApp(99)
		app.memberID = MemberAnon
		app.diffIdx = 1
		app.resetGame(true)
		g := app.game
		g.FirstClick = true
		g.Status = StatusPlaying
		for i := range g.Cells {
			g.Cells[i].IsMine = false
			g.Cells[i].Revealed = false
		}
		// Place mines so (0,0), (0,2), and (0,4) are isolated numbered safe cells;
		// Anon's skill then reliably opens 2 off-viewport targets without flood-filling.
		g.Cell(0, 1).IsMine = true
		g.Cell(0, 3).IsMine = true
		g.Cell(15, 15).IsMine = true
		g.Mines = 3
		g.RecomputeAdjacency()
		for i := range g.Cells {
			c := &g.Cells[i]
			if !c.IsMine && !(c.Row == 0 && (c.Col == 0 || c.Col == 2 || c.Col == 4)) {
				c.Revealed = true
				g.RevealedSafe++
			}
		}

		tt := ui.NewTester(app.view, 960, 640)
		tt.Frame()
		if !app.classicOnCanvas() {
			t.Fatal("16x16 should start on canvas at 960x640")
		}

		// Pan to bottom-right so row 0 is well off-viewport.
		app.board.center(g, 15, 15)
		tt.Frame()
		if app.board.shows(g, 0, 0) {
			t.Fatal("precondition failed: (0,0) should be off-viewport before skill")
		}

		tt.Key(0, ui.KeyE)
		if !app.board.gliding() {
			t.Fatal("expected skill to trigger off-viewport camera glide before resize")
		}
		if len(app.skillPulses) != 0 {
			t.Fatalf("expected skill pulses deferred during glide, got %d", len(app.skillPulses))
		}

		// Enlarge mid-glide so the board switches back to Grid: deferred pulses must appear.
		tt.SetSize(1280, 900)
		tt.Frame()
		if app.classicOnCanvas() {
			t.Fatal("expected 16x16 to switch back to grid at 1280x900")
		}
		if len(app.skillPulses) != 2 {
			t.Fatalf("canvas->grid switch mid-glide lost skill target pulses: got %d, want 2", len(app.skillPulses))
		}
	})
}
