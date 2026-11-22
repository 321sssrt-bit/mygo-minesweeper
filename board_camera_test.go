package main

import (
	"math"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func setupCameraDuelApp(t *testing.T) (*minesweeperApp, *ui.Tester, ui.Rect) {
	t.Helper()
	app := newMinesweeperApp(42)
	app.diffIdx = findDifficultyIdx("duel")
	app.resetGame(false)
	app.game.duel.NextAI = 1e9
	tt := ui.NewTester(app.view, 1280, 900)
	tt.Frame()
	r, ok := tt.Find("决斗棋盘")
	if !ok {
		t.Fatal("duel board missing")
	}
	return app, tt, r
}

func setupCameraClassicApp(t *testing.T, track Difficulty) (*minesweeperApp, *ui.Tester, ui.Rect) {
	t.Helper()
	app := newMinesweeperApp(42)
	app.diffIdx = findDifficultyIdx(track.ID)
	app.resetGame(false)
	tt := ui.NewTester(app.view, 1280, 900)
	tt.Frame()
	r, ok := tt.Find("经典棋盘")
	if !ok {
		t.Fatal("classic canvas board missing")
	}
	return app, tt, r
}

func viewportSpansBetween(x0, y0, x1, y1, w, h float32) float64 {
	return math.Hypot(float64(x0-x1)/float64(w), float64(y0-y1)/float64(h))
}

func TestCanvasCameraImmediateProgrammaticDragAndScroll(t *testing.T) {
	for _, isDuel := range []bool{false, true} {
		name := "classic-canvas"
		if isDuel {
			name = "duel-canvas"
		}
		t.Run(name, func(t *testing.T) {
			var app *minesweeperApp
			var tt *ui.Tester
			var boardRect ui.Rect
			if isDuel {
				app, tt, boardRect = setupCameraDuelApp(t)
			} else {
				app, tt, boardRect = setupCameraClassicApp(t, TrackMayoiuta)
			}
			v, g := app.board, app.game

			// Programmatic center and centerBoard land immediately on frame 0 without gliding.
			v.center(g, 12, 18)
			tt.Frame()
			if v.gliding() {
				t.Fatal("programmatic center unexpectedly started glide")
			}
			gotR, gotC, ok := v.cellAt(g, boardRect.W/2, boardRect.H/2)
			if !ok || gotR != 12 || gotC != 18 {
				t.Fatalf("programmatic center landed at (%d,%d) ok=%v, want (12,18)", gotR, gotC, ok)
			}

			v.centerBoard(g)
			tt.Frame()
			if v.gliding() {
				t.Fatal("programmatic centerBoard unexpectedly started glide")
			}
			gotR, gotC, ok = v.cellAt(g, boardRect.W/2, boardRect.H/2)
			if !ok || gotR != g.Rows/2 || gotC != g.Cols/2 {
				t.Fatalf("programmatic centerBoard landed at (%d,%d) ok=%v, want (%d,%d)", gotR, gotC, ok, g.Rows/2, g.Cols/2)
			}

			// Drag pan updates immediately on frame 0 without gliding or cell edits.
			beforeX, beforeY := v.X, v.Y
			sx, sy := boardRect.X+boardRect.W*0.4, boardRect.Y+boardRect.H*0.4
			tt.Press(sx, sy)
			tt.Move(sx+75, sy+50)
			tt.Release(sx+75, sy+50)
			if v.gliding() {
				t.Fatal("drag pan unexpectedly started glide")
			}
			if v.X != beforeX-75 || v.Y != beforeY-50 {
				t.Fatalf("drag pan = (%v,%v), want (%v,%v)", v.X, v.Y, beforeX-75, beforeY-50)
			}
			if g.FirstClick || g.Moves != 0 || g.FlagsPlaced != 0 {
				t.Fatal("drag pan leaked cell action")
			}

			// Unmodified Scroll (touchpad) and Wheel (mouse) both pan immediately on frame 0 without changing Cell.
			anchorX, anchorY := boardRect.W*0.45, boardRect.H*0.45
			scrollCases := []struct {
				name string
				send func(x, y, dx, dy float32)
			}{
				{"scroll", tt.Scroll},
				{"wheel", tt.Wheel},
			}
			for _, sc := range scrollCases {
				panBeforeX, panBeforeY, panBeforeCell := v.X, v.Y, v.Cell
				sc.send(boardRect.X+anchorX, boardRect.Y+anchorY, 64, 48)
				if v.gliding() || v.Cell != panBeforeCell {
					t.Fatalf("unmodified %s changed cell or started glide: cell=%.2f (was %.2f) gliding=%v",
						sc.name, v.Cell, panBeforeCell, v.gliding())
				}
				if v.X != panBeforeX+64 || v.Y != panBeforeY+48 {
					t.Fatalf("unmodified %s pan = (%v,%v), want (%v,%v)",
						sc.name, v.X, v.Y, panBeforeX+64, panBeforeY+48)
				}
				if g.FirstClick || g.Moves != 0 || g.FlagsPlaced != 0 {
					t.Fatalf("unmodified %s pan leaked cell action", sc.name)
				}
			}

			// Ctrl or Cmd (Super) + Scroll/Wheel zooms immediately on frame 0 and preserves anchor without panning.
			// mygo@v0.9.2 ui.Tester.Scroll/Wheel hardcode Mods=0, so feed modified InputScroll
			// events directly into handleCanvasInput on the Tester-laid-out viewport.
			preciseCases := []struct {
				name    string
				precise bool
			}{
				{"scroll", true},
				{"wheel", false},
			}
			modCases := []struct {
				name string
				mod  ui.Modifiers
			}{
				{"ctrl", ui.Ctrl},
				{"super", ui.Super},
			}
			for _, pc := range preciseCases {
				for _, mc := range modCases {
					wx := (v.X + anchorX) / v.Cell
					wy := (v.Y + anchorY) / v.Cell
					beforeCell := v.Cell
					if !app.handleCanvasInput(ui.InputEvent{
						Kind:    ui.InputScroll,
						X:       anchorX,
						Y:       anchorY,
						DX:      40,
						DY:      -240,
						Mods:    mc.mod,
						Precise: pc.precise,
					}, tt.Now()) {
						t.Fatalf("%s+%s zoom-in not handled", mc.name, pc.name)
					}
					if v.gliding() || v.Cell <= beforeCell {
						t.Fatalf("%s+%s zoom-in = %.2f gliding=%v, want immediate > %.2f",
							mc.name, pc.name, v.Cell, v.gliding(), beforeCell)
					}
					if math.Abs(float64((v.X+anchorX)/v.Cell-wx)) > 0.02 || math.Abs(float64((v.Y+anchorY)/v.Cell-wy)) > 0.02 {
						t.Fatalf("%s+%s zoom-in drifted world anchor", mc.name, pc.name)
					}

					wx = (v.X + anchorX) / v.Cell
					wy = (v.Y + anchorY) / v.Cell
					beforeCell = v.Cell
					if !app.handleCanvasInput(ui.InputEvent{
						Kind:    ui.InputScroll,
						X:       anchorX,
						Y:       anchorY,
						DX:      -40,
						DY:      240,
						Mods:    mc.mod,
						Precise: pc.precise,
					}, tt.Now()) {
						t.Fatalf("%s+%s zoom-out not handled", mc.name, pc.name)
					}
					if v.gliding() || v.Cell >= beforeCell {
						t.Fatalf("%s+%s zoom-out = %.2f gliding=%v, want immediate < %.2f",
							mc.name, pc.name, v.Cell, v.gliding(), beforeCell)
					}
					if math.Abs(float64((v.X+anchorX)/v.Cell-wx)) > 0.02 || math.Abs(float64((v.Y+anchorY)/v.Cell-wy)) > 0.02 {
						t.Fatalf("%s+%s zoom-out drifted world anchor", mc.name, pc.name)
					}
				}
			}

			// Trackpad pinch zoom also updates immediately on frame 0 and preserves anchor.
			pwx := (v.X + anchorX) / v.Cell
			pwy := (v.Y + anchorY) / v.Cell
			beforeCell := v.Cell
			tt.Pinch(boardRect.X+anchorX, boardRect.Y+anchorY, 0.85)
			if v.gliding() || v.Cell >= beforeCell {
				t.Fatalf("pinch zoom = %.2f gliding=%v, want immediate < %.2f", v.Cell, v.gliding(), beforeCell)
			}
			if math.Abs(float64((v.X+anchorX)/v.Cell-pwx)) > 0.02 || math.Abs(float64((v.Y+anchorY)/v.Cell-pwy)) > 0.02 {
				t.Fatal("pinch zoom drifted world anchor")
			}
		})
	}
}

func TestCanvasCameraSpringGlideAllInteractiveTriggers(t *testing.T) {
	app, tt, boardRect := setupCameraDuelApp(t)
	v, g := app.board, app.game

	// 1. Zoom-in ("放大") and zoom-out ("缩小") buttons at (500,500) on 1000x1000:
	// must glide smoothly over ~200ms without triggering far-jump pan drift.
	v.center(g, 500, 500)
	tt.Frame()
	cell0 := v.Cell
	wantIn := min(float32(42), cell0*1.2)
	if err := tt.Click("放大"); err != nil {
		t.Fatalf("click 放大: %v", err)
	}
	if !v.gliding() || v.Cell != cell0 {
		t.Fatalf("放大 frame 0: cell=%.2f gliding=%v, want %.2f gliding=true", v.Cell, v.gliding(), cell0)
	}
	tt.Advance(60 * time.Millisecond)
	if !v.gliding() || v.Cell <= cell0 || v.Cell >= wantIn {
		t.Fatalf("放大 mid-glide: cell=%.2f gliding=%v, want in (%.2f, %.2f)", v.Cell, v.gliding(), cell0, wantIn)
	}
	midR, midC, ok := v.cellAt(g, boardRect.W/2, boardRect.H/2)
	if !ok || midR != 500 || midC != 500 {
		t.Fatalf("放大 mid-glide drifted center to (%d,%d), want (500,500)", midR, midC)
	}
	tt.Advance(180 * time.Millisecond)
	if v.gliding() || abs32(v.Cell-wantIn) > 0.05 {
		t.Fatalf("放大 settled: cell=%.2f gliding=%v, want %.2f gliding=false", v.Cell, v.gliding(), wantIn)
	}
	if rEnd, cEnd, ok := v.cellAt(g, boardRect.W/2, boardRect.H/2); !ok || rEnd != 500 || cEnd != 500 {
		t.Fatalf("放大 settled center = (%d,%d), want (500,500)", rEnd, cEnd)
	}

	cellIn := v.Cell
	wantOut := max(float32(10), cellIn/1.2)
	if err := tt.Click("缩小"); err != nil {
		t.Fatalf("click 缩小: %v", err)
	}
	if !v.gliding() || v.Cell != cellIn {
		t.Fatalf("缩小 frame 0: cell=%.2f gliding=%v, want %.2f gliding=true", v.Cell, v.gliding(), cellIn)
	}
	tt.Advance(60 * time.Millisecond)
	if !v.gliding() || v.Cell >= cellIn || v.Cell <= wantOut {
		t.Fatalf("缩小 mid-glide: cell=%.2f gliding=%v, want in (%.2f, %.2f)", v.Cell, v.gliding(), wantOut, cellIn)
	}
	tt.Advance(180 * time.Millisecond)
	if v.gliding() || abs32(v.Cell-wantOut) > 0.05 {
		t.Fatalf("缩小 settled: cell=%.2f gliding=%v, want %.2f gliding=false", v.Cell, v.gliding(), wantOut)
	}

	// Ctrl+= and Ctrl+- keyboard zoom shortcuts keep gliding zoom behavior intact.
	keyCell0 := v.Cell
	tt.Key(ui.Ctrl, ui.KeyEqual)
	if !v.gliding() {
		t.Fatal("Ctrl+= did not start zoom-in glide")
	}
	tt.Advance(240 * time.Millisecond)
	if v.gliding() || v.Cell <= keyCell0 {
		t.Fatalf("Ctrl+= settled: cell=%.2f gliding=%v, want > %.2f gliding=false", v.Cell, v.gliding(), keyCell0)
	}
	keyCellIn := v.Cell
	tt.Key(ui.Ctrl, ui.KeyMinus)
	if !v.gliding() {
		t.Fatal("Ctrl+- did not start zoom-out glide")
	}
	tt.Advance(240 * time.Millisecond)
	if v.gliding() || v.Cell >= keyCellIn || abs32(v.Cell-keyCell0) > 0.05 {
		t.Fatalf("Ctrl+- settled: cell=%.2f gliding=%v, want ~%.2f (< %.2f) gliding=false", v.Cell, v.gliding(), keyCell0, keyCellIn)
	}

	// 2. "回到中心" button from a mid-range offset (~0.9 viewport spans).
	v.center(g, 500, 525)
	tt.Frame()
	startX, startY := v.X, v.Y
	wantX, wantY := v.centerTarget(g, 500, 500, v.Cell)
	if err := tt.Click("回到中心"); err != nil {
		t.Fatalf("click 回到中心: %v", err)
	}
	if !v.gliding() || v.X != startX || v.Y != startY {
		t.Fatalf("回到中心 frame 0: pos=(%v,%v) gliding=%v, want start=(%v,%v)", v.X, v.Y, v.gliding(), startX, startY)
	}
	tt.Advance(60 * time.Millisecond)
	if !v.gliding() || v.X >= startX || v.X <= wantX {
		t.Fatalf("回到中心 mid-glide: X=%v gliding=%v, want in (%v, %v)", v.X, v.gliding(), wantX, startX)
	}
	tt.Advance(180 * time.Millisecond)
	if v.gliding() || abs32(v.X-wantX) > 0.5 || abs32(v.Y-wantY) > 0.5 {
		t.Fatalf("回到中心 settled: pos=(%v,%v) gliding=%v, want (%v,%v)", v.X, v.Y, v.gliding(), wantX, wantY)
	}

	// 3. "定位高松灯" button from a mid-range offset.
	g.duel.LastAI = 515*g.Cols + 520
	wantAIX, wantAIY := v.centerTarget(g, 515, 520, v.Cell)
	startX, startY = v.X, v.Y
	if err := tt.Click("定位高松灯"); err != nil {
		t.Fatalf("click 定位高松灯: %v", err)
	}
	if !v.gliding() || v.X != startX || v.Y != startY {
		t.Fatalf("定位高松灯 frame 0: pos=(%v,%v) gliding=%v, want start=(%v,%v)", v.X, v.Y, v.gliding(), startX, startY)
	}
	tt.Advance(60 * time.Millisecond)
	if !v.gliding() || v.X <= startX || v.X >= wantAIX || v.Y <= startY || v.Y >= wantAIY {
		t.Fatalf("定位高松灯 mid-glide: pos=(%v,%v) gliding=%v, want between (%v,%v) and (%v,%v)",
			v.X, v.Y, v.gliding(), startX, startY, wantAIX, wantAIY)
	}
	tt.Advance(180 * time.Millisecond)
	if v.gliding() || abs32(v.X-wantAIX) > 0.5 || abs32(v.Y-wantAIY) > 0.5 {
		t.Fatalf("定位高松灯 settled: pos=(%v,%v) gliding=%v, want (%v,%v)", v.X, v.Y, v.gliding(), wantAIX, wantAIY)
	}

	// 4. Classic large canvas minimap click + zero pending animation frames once converged.
	cApp, cTT, cBoard := setupCameraClassicApp(t, TrackMayoiuta)
	cv, cg := cApp.board, cApp.game
	cv.center(cg, 50, 50)
	cTT.Frame()
	mini := cv.minimapBoundsFor(cg)
	mx, my := minimapCellCenter(cBoard, mini, cg.Rows, cg.Cols, 35, 35)
	cTT.ClickAt(mx, my)
	if cg.FirstClick || cg.Moves != 0 || cg.FlagsPlaced != 0 {
		t.Fatal("classic minimap click leaked board edit on frame 0")
	}
	if !cv.gliding() {
		t.Fatal("classic minimap click did not start glide")
	}
	cx0, cy0 := cv.X, cv.Y
	cTT.Advance(60 * time.Millisecond)
	if !cv.gliding() || cv.X >= cx0 || cv.Y >= cy0 {
		t.Fatalf("classic minimap mid-glide not transitioning: start=(%v,%v) mid=(%v,%v) gliding=%v",
			cx0, cy0, cv.X, cv.Y, cv.gliding())
	}
	cTT.Advance(200 * time.Millisecond)
	if cv.gliding() {
		t.Fatal("classic minimap glide did not settle after 260ms")
	}
	cTT.Frame()
	if cTT.FramePending() {
		t.Fatal("classic canvas kept requesting animation frames after camera glide settled")
	}
	gotR, gotC, ok := cv.cellAt(cg, cBoard.W/2, cBoard.H/2)
	if !ok || absInt(gotR-35) > 1 || absInt(gotC-35) > 1 {
		t.Fatalf("classic minimap settled at (%d,%d) ok=%v, want near (35,35)", gotR, gotC, ok)
	}
}

func TestCanvasCameraFarJumpClampsToViewportSpan(t *testing.T) {
	app, tt, boardRect := setupCameraDuelApp(t)
	v, g := app.board, app.game

	// Case A: Cross-board minimap jump from top-left (0,0) to bottom-right (999,999)
	// (> 20 viewport spans) must clamp the glide start to ~1.5 viewport spans from target.
	v.center(g, 0, 0)
	tt.Frame()
	mini := v.minimapBoundsFor(g)
	clickX := boardRect.X + mini.X + mini.W - 2
	clickY := boardRect.Y + mini.Y + mini.H - 2
	tgtRow, tgtCol := v.minimapCellAt(g, mini.X+mini.W-2, mini.Y+mini.H-2)
	wantX, wantY := v.centerTarget(g, tgtRow, tgtCol, v.Cell)
	rawSpans := viewportSpansBetween(v.X, v.Y, wantX, wantY, v.Width, v.Height)
	if rawSpans <= 20 {
		t.Fatalf("precondition failed: cross-board span = %.2f, want > 20", rawSpans)
	}

	tt.ClickAt(clickX, clickY)
	if g.FirstClick || g.Moves != 0 {
		t.Fatal("far minimap click leaked board reveal")
	}
	if !v.gliding() {
		t.Fatal("far minimap click did not start glide")
	}
	clampedSpans := viewportSpansBetween(v.X, v.Y, wantX, wantY, v.Width, v.Height)
	if clampedSpans < 1.45 || clampedSpans > 1.55 {
		t.Fatalf("far minimap jump clamped span = %.3f viewports, want ~1.5 (raw was %.2f)", clampedSpans, rawSpans)
	}
	clampedX, clampedY := v.X, v.Y
	tt.Advance(60 * time.Millisecond)
	midSpans := viewportSpansBetween(v.X, v.Y, wantX, wantY, v.Width, v.Height)
	if !v.gliding() || midSpans <= 0.05 || midSpans >= clampedSpans*0.8 {
		t.Fatalf("far minimap mid-glide span = %.3f (clamped=%.3f, gliding=%v)", midSpans, clampedSpans, v.gliding())
	}
	tt.Advance(200 * time.Millisecond)
	if v.gliding() || abs32(v.X-wantX) > 0.5 || abs32(v.Y-wantY) > 0.5 {
		t.Fatalf("far minimap did not converge: pos=(%v,%v) want=(%v,%v) start=(%v,%v) gliding=%v",
			v.X, v.Y, wantX, wantY, clampedX, clampedY, v.gliding())
	}

	// Case B: "定位高松灯" far jump across the million-cell board also clamps to ~1.5 spans.
	v.center(g, 900, 900)
	tt.Frame()
	g.duel.LastAI = 100*g.Cols + 100
	wantAIX, wantAIY := v.centerTarget(g, 100, 100, v.Cell)
	if err := tt.Click("定位高松灯"); err != nil {
		t.Fatalf("click 定位高松灯: %v", err)
	}
	aiClampedSpans := viewportSpansBetween(v.X, v.Y, wantAIX, wantAIY, v.Width, v.Height)
	if !v.gliding() || aiClampedSpans < 1.45 || aiClampedSpans > 1.55 {
		t.Fatalf("far 定位高松灯 clamped span = %.3f gliding=%v, want ~1.5", aiClampedSpans, v.gliding())
	}
	tt.Advance(240 * time.Millisecond)
	if v.gliding() || abs32(v.X-wantAIX) > 0.5 || abs32(v.Y-wantAIY) > 0.5 {
		t.Fatalf("far 定位高松灯 did not converge: pos=(%v,%v) want=(%v,%v) gliding=%v", v.X, v.Y, wantAIX, wantAIY, v.gliding())
	}

	// Case C: Mid-range jump (<= 2 viewport spans, e.g. ~1.6 spans) is NOT clamped.
	v.center(g, 500, 500)
	tt.Frame()
	wantMidX, wantMidY := v.centerTarget(g, 500, 500, v.Cell)
	offsetCols := int(math.Round(1.6 * float64(v.Width) / float64(v.Cell)))
	v.center(g, 500, 500+offsetCols)
	tt.Frame()
	unclampedX, unclampedY := v.X, v.Y
	midRawSpans := viewportSpansBetween(unclampedX, unclampedY, wantMidX, wantMidY, v.Width, v.Height)
	if midRawSpans <= 1.5 || midRawSpans >= 2.0 {
		t.Fatalf("precondition failed: mid-range span = %.2f, want in (1.5, 2.0)", midRawSpans)
	}
	if err := tt.Click("回到中心"); err != nil {
		t.Fatalf("click 回到中心 for mid-range check: %v", err)
	}
	if !v.gliding() || v.X != unclampedX || v.Y != unclampedY {
		t.Fatalf("mid-range jump (%.2f spans) should not clamp start: got=(%v,%v) want=(%v,%v) gliding=%v",
			midRawSpans, v.X, v.Y, unclampedX, unclampedY, v.gliding())
	}
	tt.Advance(240 * time.Millisecond)
	if v.gliding() || abs32(v.X-wantMidX) > 0.5 || abs32(v.Y-wantMidY) > 0.5 {
		t.Fatalf("mid-range jump did not converge: pos=(%v,%v) want=(%v,%v)", v.X, v.Y, wantMidX, wantMidY)
	}

	// Case D: Zooming out from Cell=42 in a corner then pressing Ctrl+0 early
	// in the zoom glide must still clamp the pan run-in to ~1.5 current viewport
	// spans (measured at the current displayed Cell, not the smaller target Cell).
	v.Cell = 42
	v.center(g, 0, 0)
	tt.Frame()
	if err := tt.Click("缩小"); err != nil {
		t.Fatalf("click 缩小 before Ctrl+0: %v", err)
	}
	tt.Advance(20 * time.Millisecond)
	if v.Cell >= 42 || v.Cell <= 35 {
		t.Fatalf("precondition failed: expected mid-glide zoom-out cell in (35, 42), got %.2f", v.Cell)
	}
	tt.Key(ui.Ctrl, ui.Key0)
	if !v.gliding() {
		t.Fatal("Ctrl+0 during zoom-out glide did not keep glide active")
	}
	curCell := float64(v.Cell)
	curCenterX := (float64(v.X) + float64(v.Width)/2) / curCell
	curCenterY := (float64(v.Y) + float64(v.Height)/2) / curCell
	tgtCell := float32(v.glide.targetCell)
	wantCenterX, wantCenterY := v.centerTarget(g, g.Rows/2, g.Cols/2, tgtCell)
	tgtWX := (float64(wantCenterX) + float64(v.Width)/2) / float64(tgtCell)
	tgtWY := (float64(wantCenterY) + float64(v.Height)/2) / float64(tgtCell)
	curViewCellsW := float64(v.Width) / curCell
	curViewCellsH := float64(v.Height) / curCell
	combinedSpans := math.Hypot((curCenterX-tgtWX)/curViewCellsW, (curCenterY-tgtWY)/curViewCellsH)
	if combinedSpans < 1.45 || combinedSpans > 1.55 {
		t.Fatalf("zoom-out + Ctrl+0 clamped span = %.3f current viewports, want ~1.5 (curCell=%.2f tgtCell=%.2f)",
			combinedSpans, curCell, tgtCell)
	}
	tt.Advance(240 * time.Millisecond)
	if v.gliding() || abs32(v.Cell-35) > 0.05 {
		t.Fatalf("zoom-out + Ctrl+0 did not settle at Cell=35: cell=%.2f gliding=%v", v.Cell, v.gliding())
	}
	if rEnd, cEnd, ok := v.cellAt(g, boardRect.W/2, boardRect.H/2); !ok || rEnd != g.Rows/2 || cEnd != g.Cols/2 {
		t.Fatalf("zoom-out + Ctrl+0 settled at (%d,%d), want (%d,%d)", rEnd, cEnd, g.Rows/2, g.Cols/2)
	}
}

func TestCanvasCameraMidGlideManualInputTakeover(t *testing.T) {
	// 1. Mid-glide left-click stops spring immediately and reveals the exact cell displayed on screen.
	app, tt, boardRect := setupCameraDuelApp(t)
	v, g := app.board, app.game
	v.center(g, 400, 400)
	tt.Frame()
	if err := tt.Click("回到中心"); err != nil {
		t.Fatalf("click 回到中心: %v", err)
	}
	tt.Advance(60 * time.Millisecond)
	if !v.gliding() {
		t.Fatal("expected glide still active at 60ms")
	}
	shownX, shownY, shownCell := v.X, v.Y, v.Cell
	probeLocalX, probeLocalY := boardRect.W*0.35, boardRect.H*0.4
	wantRow, wantCol, ok := v.cellAt(g, probeLocalX, probeLocalY)
	if !ok {
		t.Fatal("probe point out of bounds mid-glide")
	}
	// Verify the target cell visually corresponds to the displayed frame before clicking.
	if c := g.Cell(wantRow, wantCol); c.Revealed || c.Flagged {
		t.Fatalf("precondition failed: displayed cell (%d,%d) already modified", wantRow, wantCol)
	}

	tt.ClickAt(boardRect.X+probeLocalX, boardRect.Y+probeLocalY)
	if v.gliding() {
		t.Fatal("mid-glide left-click did not cancel spring")
	}
	if v.X != shownX || v.Y != shownY || v.Cell != shownCell {
		t.Fatalf("mid-glide click shifted camera from displayed (%v,%v,%.2f) to (%v,%v,%.2f)",
			shownX, shownY, shownCell, v.X, v.Y, v.Cell)
	}
	if !g.Cell(wantRow, wantCol).Revealed {
		t.Fatalf("mid-glide click missed displayed cell (%d,%d); LastCell=(%d,%d)",
			wantRow, wantCol, g.duel.LastCell/g.Cols, g.duel.LastCell%g.Cols)
	}
	// Camera stays permanently at the taken-over coordinates even after advancing >200ms.
	tt.Advance(300 * time.Millisecond)
	if v.gliding() || v.X != shownX || v.Y != shownY || v.Cell != shownCell {
		t.Fatalf("camera resumed gliding after click takeover: pos=(%v,%v,%.2f) want=(%v,%v,%.2f)",
			v.X, v.Y, v.Cell, shownX, shownY, shownCell)
	}

	// 2. Mid-glide right-click stops spring immediately and flags the exact displayed cell.
	cApp, cTT, cBoard := setupCameraClassicApp(t, TrackMayoiuta)
	cv, cg := cApp.board, cApp.game
	cv.center(cg, 10, 10)
	cTT.Frame()
	cTT.Key(ui.Ctrl, ui.Key0)
	cTT.Advance(60 * time.Millisecond)
	if !cv.gliding() {
		t.Fatal("expected classic glide active at 60ms")
	}
	flagShownX, flagShownY := cv.X, cv.Y
	fLocalX, fLocalY := cBoard.W*0.4, cBoard.H*0.45
	fRow, fCol, ok := cv.cellAt(cg, fLocalX, fLocalY)
	if !ok {
		t.Fatal("classic probe point out of bounds mid-glide")
	}
	cTT.RightClickAt(cBoard.X+fLocalX, cBoard.Y+fLocalY)
	if cv.gliding() || cv.X != flagShownX || cv.Y != flagShownY {
		t.Fatalf("mid-glide right-click failed takeover: gliding=%v pos=(%v,%v) want=(%v,%v)",
			cv.gliding(), cv.X, cv.Y, flagShownX, flagShownY)
	}
	if !cg.Cell(fRow, fCol).Flagged || cg.FlagsPlaced != 1 {
		t.Fatalf("mid-glide right-click missed displayed cell (%d,%d)", fRow, fCol)
	}

	// 3. Mid-glide drag stops spring immediately and pans from exact displayed coordinates.
	cv.center(cg, 10, 10)
	cTT.Frame()
	cTT.Key(ui.Ctrl, ui.Key0)
	cTT.Advance(60 * time.Millisecond)
	if !cv.gliding() {
		t.Fatal("expected classic glide active before drag")
	}
	dragShownX, dragShownY := cv.X, cv.Y
	dx, dy := float32(48), float32(36)
	sx, sy := cBoard.X+cBoard.W*0.4, cBoard.Y+cBoard.H*0.4
	movesBefore := cg.Moves
	flagsBefore := cg.FlagsPlaced
	cTT.Press(sx, sy)
	if cv.gliding() || cv.X != dragShownX || cv.Y != dragShownY {
		t.Fatalf("PointerDown mid-glide did not freeze at displayed pos: gliding=%v pos=(%v,%v) want=(%v,%v)",
			cv.gliding(), cv.X, cv.Y, dragShownX, dragShownY)
	}
	cTT.Move(sx+dx, sy+dy)
	cTT.Release(sx+dx, sy+dy)
	wantDragX, wantDragY := max(float32(0), dragShownX-dx), max(float32(0), dragShownY-dy)
	if cv.gliding() || abs32(cv.X-wantDragX) > 0.5 || abs32(cv.Y-wantDragY) > 0.5 {
		t.Fatalf("mid-glide drag ended at (%v,%v) gliding=%v, want (%v,%v)", cv.X, cv.Y, cv.gliding(), wantDragX, wantDragY)
	}
	if cg.Moves != movesBefore || cg.FlagsPlaced != flagsBefore {
		t.Fatal("mid-glide drag leaked cell action")
	}
	cTT.Advance(300 * time.Millisecond)
	if abs32(cv.X-wantDragX) > 0.5 || abs32(cv.Y-wantDragY) > 0.5 {
		t.Fatalf("camera drifted after drag takeover: pos=(%v,%v) want=(%v,%v)", cv.X, cv.Y, wantDragX, wantDragY)
	}

	// 4. Mid-glide unmodified scroll/wheel pans immediately from displayed coordinates,
	// while Ctrl/Cmd+scroll and pinch cancel spring and zoom around displayed coordinates.
	ax, ay := cBoard.W*0.4, cBoard.H*0.4
	for _, sc := range []struct {
		name string
		send func(x, y, dx, dy float32)
	}{
		{"scroll", cTT.Scroll},
		{"wheel", cTT.Wheel},
	} {
		cv.center(cg, 10, 10)
		cTT.Frame()
		cTT.Key(ui.Ctrl, ui.Key0)
		cTT.Advance(60 * time.Millisecond)
		if !cv.gliding() {
			t.Fatalf("expected classic glide active before mid-glide %s pan", sc.name)
		}
		panX, panY, panCell := cv.X, cv.Y, cv.Cell
		sc.send(cBoard.X+ax, cBoard.Y+ay, 36, 24)
		wantPanX, wantPanY := panX+36, panY+24
		if cv.gliding() || cv.Cell != panCell || abs32(cv.X-wantPanX) > 0.5 || abs32(cv.Y-wantPanY) > 0.5 {
			t.Fatalf("mid-glide %s pan failed takeover: pos=(%v,%v,%.2f) gliding=%v, want (%v,%v,%.2f)",
				sc.name, cv.X, cv.Y, cv.Cell, cv.gliding(), wantPanX, wantPanY, panCell)
		}
		cTT.Advance(300 * time.Millisecond)
		if cv.gliding() || cv.Cell != panCell || abs32(cv.X-wantPanX) > 0.5 || abs32(cv.Y-wantPanY) > 0.5 {
			t.Fatalf("camera drifted after mid-glide %s pan takeover: pos=(%v,%v,%.2f) want (%v,%v,%.2f)",
				sc.name, cv.X, cv.Y, cv.Cell, wantPanX, wantPanY, panCell)
		}
	}

	for _, zc := range []struct {
		name    string
		mod     ui.Modifiers
		precise bool
	}{
		{"ctrl+scroll", ui.Ctrl, true},
		{"ctrl+wheel", ui.Ctrl, false},
		{"super+scroll", ui.Super, true},
		{"super+wheel", ui.Super, false},
	} {
		cv.Cell = 26
		cv.center(cg, 10, 10)
		cTT.Frame()
		cTT.Key(ui.Ctrl, ui.Key0)
		cTT.Advance(60 * time.Millisecond)
		if !cv.gliding() {
			t.Fatalf("expected classic glide active before mid-glide %s zoom", zc.name)
		}
		scrollX, scrollY, scrollCell := cv.X, cv.Y, cv.Cell
		wx := (scrollX + ax) / scrollCell
		wy := (scrollY + ay) / scrollCell
		if !cApp.handleCanvasInput(ui.InputEvent{
			Kind:    ui.InputScroll,
			X:       ax,
			Y:       ay,
			DY:      -240,
			Mods:    zc.mod,
			Precise: zc.precise,
		}, cTT.Now()) {
			t.Fatalf("mid-glide %s not handled", zc.name)
		}
		if cv.gliding() || cv.Cell <= scrollCell {
			t.Fatalf("mid-glide %s failed takeover: cell=%.2f (was %.2f) gliding=%v", zc.name, cv.Cell, scrollCell, cv.gliding())
		}
		if math.Abs(float64((cv.X+ax)/cv.Cell-wx)) > 0.02 || math.Abs(float64((cv.Y+ay)/cv.Cell-wy)) > 0.02 {
			t.Fatalf("mid-glide %s did not anchor on displayed coordinates", zc.name)
		}
	}

	cv.center(cg, 10, 10)
	cTT.Frame()
	cTT.Key(ui.Ctrl, ui.Key0)
	cTT.Advance(60 * time.Millisecond)
	pinchX, pinchY, pinchCell := cv.X, cv.Y, cv.Cell
	pwx := (pinchX + ax) / pinchCell
	pwy := (pinchY + ay) / pinchCell
	cTT.Pinch(cBoard.X+ax, cBoard.Y+ay, 0.85)
	if cv.gliding() || cv.Cell >= pinchCell {
		t.Fatalf("mid-glide pinch failed takeover: cell=%.2f (was %.2f) gliding=%v", cv.Cell, pinchCell, cv.gliding())
	}
	if math.Abs(float64((cv.X+ax)/cv.Cell-pwx)) > 0.02 || math.Abs(float64((cv.Y+ay)/cv.Cell-pwy)) > 0.02 {
		t.Fatal("mid-glide pinch did not anchor on displayed coordinates")
	}

	// 5. Hovering without pressing during a glide must NOT cancel the glide.
	cv.center(cg, 10, 10)
	cTT.Frame()
	cTT.Key(ui.Ctrl, ui.Key0)
	cTT.Advance(40 * time.Millisecond)
	cTT.Move(cBoard.X+cBoard.W*0.3, cBoard.Y+cBoard.H*0.3)
	if !cv.gliding() {
		t.Fatal("plain hover movement unexpectedly canceled camera glide")
	}
	cTT.Advance(220 * time.Millisecond)
	if cv.gliding() {
		t.Fatal("glide did not converge after hover movement")
	}
	if rEnd, cEnd, ok := cv.cellAt(cg, cBoard.W/2, cBoard.H/2); !ok || rEnd != cg.Rows/2 || cEnd != cg.Cols/2 {
		t.Fatalf("glide after hover landed at (%d,%d), want (%d,%d)", rEnd, cEnd, cg.Rows/2, cg.Cols/2)
	}

	// 6. Mid-glide minimap click during a zoom glide cancels the zoom spring
	// immediately and starts the minimap pan from the displayed zoom & position.
	zApp, zTT, zBoard := setupCameraDuelApp(t)
	zv, zg := zApp.board, zApp.game
	zv.center(zg, 500, 500)
	zTT.Frame()
	if err := zTT.Click("放大"); err != nil {
		t.Fatalf("click 放大 before mid-glide minimap click: %v", err)
	}
	zTT.Advance(50 * time.Millisecond)
	if !zv.gliding() {
		t.Fatal("expected zoom glide active at 50ms")
	}
	frozenCell := zv.Cell
	if frozenCell <= 26 || frozenCell >= 31.1 {
		t.Fatalf("expected mid-glide zoom cell in (26, 31.1), got %.2f", frozenCell)
	}
	zMini := zv.minimapBoundsFor(zg)
	// Click near (505, 505) on the minimap (well within 2 viewport spans so no far clamp).
	zClickX := zBoard.X + zMini.X + (505.5/float32(zg.Cols))*zMini.W
	zClickY := zBoard.Y + zMini.Y + (505.5/float32(zg.Rows))*zMini.H
	beforeMiniX, beforeMiniY := zv.X, zv.Y
	zTT.ClickAt(zClickX, zClickY)
	if !zv.gliding() {
		t.Fatal("minimap click during zoom glide did not start pan glide")
	}
	if zv.Cell != frozenCell || zv.X != beforeMiniX || zv.Y != beforeMiniY {
		t.Fatalf("minimap click did not take over from displayed zoom/pos: got (%.2f,%v,%v), want (%.2f,%v,%v)",
			zv.Cell, zv.X, zv.Y, frozenCell, beforeMiniX, beforeMiniY)
	}
	zTT.Advance(240 * time.Millisecond)
	if zv.gliding() || abs32(zv.Cell-frozenCell) > 0.02 {
		t.Fatalf("minimap takeover continued old zoom glide: cell=%.2f want=%.2f gliding=%v", zv.Cell, frozenCell, zv.gliding())
	}
}

func TestCanvasCameraReducedMotionAndResetCleanup(t *testing.T) {
	// 1. Effective reduced motion (both motionReduced and motionSystem + system ReduceMotion)
	// lands directly at the target on frame 0 without gliding.
	for _, tc := range []struct {
		name         string
		mode         motionMode
		systemReduce bool
		wantGlide    bool
	}{
		{"reduced_overrides_system_off", motionReduced, false, false},
		{"system_follows_on", motionSystem, true, false},
		{"standard_overrides_system_on", motionStandard, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := defaultPrefs()
			p.Motion = tc.mode
			app := newMinesweeperAppWith(appOptions{Seed: 42, Prefs: &p})
			app.diffIdx = findDifficultyIdx("duel")
			app.resetGame(false)
			app.game.duel.NextAI = 1e9
			tt := ui.NewTester(app.view, 1280, 900)
			tt.SetPreferences(ui.Preferences{ReduceMotion: tc.systemReduce})
			tt.Frame()
			r, ok := tt.Find("决斗棋盘")
			if !ok {
				t.Fatal("duel board missing")
			}

			app.board.center(app.game, 100, 100)
			tt.Frame()
			wantX, wantY := app.board.centerTarget(app.game, 500, 500, app.board.Cell)
			if err := tt.Click("回到中心"); err != nil {
				t.Fatalf("click 回到中心: %v", err)
			}
			if app.board.gliding() != tc.wantGlide {
				t.Fatalf("回到中心 gliding=%v, want %v", app.board.gliding(), tc.wantGlide)
			}
			if !tc.wantGlide {
				if abs32(app.board.X-wantX) > 0.5 || abs32(app.board.Y-wantY) > 0.5 {
					t.Fatalf("reduced 回到中心 did not snap to target: got=(%v,%v) want=(%v,%v)",
						app.board.X, app.board.Y, wantX, wantY)
				}
			}

			// Minimap click under the same motion mode.
			mini := app.board.minimapBoundsFor(app.game)
			tt.ClickAt(r.X+mini.X+2, r.Y+mini.Y+2)
			if app.board.gliding() != tc.wantGlide {
				t.Fatalf("minimap click gliding=%v, want %v", app.board.gliding(), tc.wantGlide)
			}
		})
	}

	// 2. Switching to reduced motion mid-glide immediately snaps to the target on that same frame.
	app, tt, _ := setupCameraDuelApp(t)
	app.board.center(app.game, 0, 0)
	tt.Frame()
	wantX, wantY := app.board.centerTarget(app.game, 500, 500, app.board.Cell)
	if err := tt.Click("回到中心"); err != nil {
		t.Fatalf("click 回到中心: %v", err)
	}
	tt.Advance(50 * time.Millisecond)
	if !app.board.gliding() || (abs32(app.board.X-wantX) <= 1 && abs32(app.board.Y-wantY) <= 1) {
		t.Fatal("expected active glide before mid-glide ReduceMotion toggle")
	}
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	if app.board.gliding() || abs32(app.board.X-wantX) > 0.5 || abs32(app.board.Y-wantY) > 0.5 {
		t.Fatalf("system ReduceMotion=true mid-glide did not snap straight to target: gliding=%v pos=(%v,%v) want=(%v,%v)",
			app.board.gliding(), app.board.X, app.board.Y, wantX, wantY)
	}

	// Also verify switching app preference to motionReduced mid-glide while system ReduceMotion=false.
	tt.SetPreferences(ui.Preferences{ReduceMotion: false})
	app.board.center(app.game, 0, 0)
	tt.Frame()
	if err := tt.Click("回到中心"); err != nil {
		t.Fatalf("click 回到中心 (2): %v", err)
	}
	tt.Advance(50 * time.Millisecond)
	if !app.board.gliding() {
		t.Fatal("expected active glide before updatePrefs(motionReduced)")
	}
	app.updatePrefs(func(p *prefs) { p.Motion = motionReduced })
	if app.board.gliding() || abs32(app.board.X-wantX) > 0.5 || abs32(app.board.Y-wantY) > 0.5 {
		t.Fatalf("updatePrefs(motionReduced) mid-glide did not snap straight to target: gliding=%v pos=(%v,%v) want=(%v,%v)",
			app.board.gliding(), app.board.X, app.board.Y, wantX, wantY)
	}

	// 3. Reset (R), track switch (换曲), and gameplay mode switch (换玩法模式) all clear active glide immediately.
	// 3a. R restart mid-glide on duel canvas.
	rApp, rTT, _ := setupCameraDuelApp(t)
	rApp.board.center(rApp.game, 0, 0)
	rTT.Frame()
	if err := rTT.Click("回到中心"); err != nil {
		t.Fatalf("click 回到中心 before R: %v", err)
	}
	rTT.Advance(50 * time.Millisecond)
	if !rApp.board.gliding() {
		t.Fatal("expected active glide before KeyR")
	}
	rTT.Key(0, ui.KeyR)
	if rApp.board.gliding() {
		t.Fatal("KeyR did not clear in-flight camera glide")
	}

	// 3b. Track switch via setlist mid-glide.
	sApp, sTT, _ := setupCameraDuelApp(t)
	sApp.board.center(sApp.game, 0, 0)
	sTT.Frame()
	if err := sTT.Click("回到中心"); err != nil {
		t.Fatalf("click 回到中心 before track switch: %v", err)
	}
	sTT.Advance(50 * time.Millisecond)
	if !sApp.board.gliding() {
		t.Fatal("expected active glide before track switch")
	}
	if err := sTT.Click("歌单"); err != nil {
		t.Fatalf("open setlist: %v", err)
	}
	if err := sTT.Click(TrackMayoiuta.Song); err != nil {
		t.Fatalf("switch track to %s: %v", TrackMayoiuta.Song, err)
	}
	if sApp.board.gliding() || sApp.game.Difficulty.ID != TrackMayoiuta.ID {
		t.Fatalf("track switch did not clear in-flight glide: gliding=%v track=%q", sApp.board.gliding(), sApp.game.Difficulty.ID)
	}

	// 3c. Gameplay mode switch (乐队 Live -> 纯正经典) mid-glide on large classic canvas.
	mApp, mTT, _ := setupCameraClassicApp(t, TrackMayoiuta)
	mApp.board.center(mApp.game, 0, 0)
	mTT.Frame()
	mTT.Key(ui.Ctrl, ui.Key0)
	mTT.Advance(50 * time.Millisecond)
	if !mApp.board.gliding() {
		t.Fatal("expected active glide before mode switch")
	}
	if err := mTT.Click("纯正经典"); err != nil {
		t.Fatalf("switch mode to 纯正经典: %v", err)
	}
	if mApp.board.gliding() || mApp.modeIdx != 1 {
		t.Fatalf("mode switch did not clear in-flight glide: gliding=%v modeIdx=%d", mApp.board.gliding(), mApp.modeIdx)
	}
}

func TestCanvasCameraOptionAndMiddleDragPan(t *testing.T) {
	for _, isDuel := range []bool{false, true} {
		name := "classic-canvas"
		if isDuel {
			name = "duel-canvas"
		}
		t.Run(name, func(t *testing.T) {
			var app *minesweeperApp
			var tt *ui.Tester
			var boardRect ui.Rect
			if isDuel {
				app, tt, boardRect = setupCameraDuelApp(t)
			} else {
				app, tt, boardRect = setupCameraClassicApp(t, TrackMayoiuta)
			}
			v, g := app.board, app.game
			v.centerBoard(g)
			tt.Frame()

			localX, localY := boardRect.W*0.4, boardRect.H*0.4
			screenX, screenY := boardRect.X+localX, boardRect.Y+localY

			// 1. Option (Alt) + left-click with zero movement through ui.Tester must not reveal.
			beforeX, beforeY := v.X, v.Y
			tt.ClickAtWith(ui.Alt, screenX, screenY)
			if v.gliding() || v.X != beforeX || v.Y != beforeY {
				t.Fatalf("Option+click shifted camera or glided: pos=(%v,%v) gliding=%v, want (%v,%v) gliding=false",
					v.X, v.Y, v.gliding(), beforeX, beforeY)
			}
			if g.FirstClick || g.Moves != 0 || g.FlagsPlaced != 0 {
				t.Fatal("Option+click leaked cell action")
			}

			// 2. Option (Alt) + left-drag pans immediately (even within the 5px threshold,
			// across larger drags where Option is released mid-drag, in flag mode, and starting
			// over the minimap) without revealing or flagging any cell.
			// mygo@v0.9.2 ui.Tester.PressButton hardcodes Mods=0, so feed modified pointer events
			// directly into handleCanvasInput on the Tester-laid-out viewport.
			v.centerBoard(g)
			tt.Frame()
			beforeX, beforeY = v.X, v.Y
			if !app.handleCanvasInput(ui.InputEvent{
				Kind:   ui.InputPointerDown,
				X:      localX,
				Y:      localY,
				Button: 0,
				Mods:   ui.Alt,
			}, tt.Now()) {
				t.Fatal("Option+PointerDown (small drag) not handled")
			}
			if !app.handleCanvasInput(ui.InputEvent{
				Kind:   ui.InputPointerMove,
				X:      localX + 3,
				Y:      localY + 2,
				Button: 0,
				Mods:   ui.Alt,
			}, tt.Now()) {
				t.Fatal("Option+PointerMove (small drag) not handled")
			}
			if !app.handleCanvasInput(ui.InputEvent{
				Kind:   ui.InputPointerUp,
				X:      localX + 3,
				Y:      localY + 2,
				Button: 0,
				Mods:   ui.Alt,
			}, tt.Now()) {
				t.Fatal("Option+PointerUp (small drag) not handled")
			}
			if v.gliding() || v.X != beforeX-3 || v.Y != beforeY-2 {
				t.Fatalf("Option+left small drag = (%v,%v) gliding=%v, want (%v,%v) gliding=false",
					v.X, v.Y, v.gliding(), beforeX-3, beforeY-2)
			}
			if g.FirstClick || g.Moves != 0 || g.FlagsPlaced != 0 {
				t.Fatal("Option+left small drag leaked cell action")
			}

			// Larger Option+left drag where Option is released before the final move and release.
			v.centerBoard(g)
			tt.Frame()
			beforeX, beforeY = v.X, v.Y
			if !app.handleCanvasInput(ui.InputEvent{
				Kind:   ui.InputPointerDown,
				X:      localX,
				Y:      localY,
				Button: 0,
				Mods:   ui.Alt,
			}, tt.Now()) {
				t.Fatal("Option+PointerDown (large drag) not handled")
			}
			if !app.handleCanvasInput(ui.InputEvent{
				Kind:   ui.InputPointerMove,
				X:      localX + 40,
				Y:      localY + 24,
				Button: 0,
				Mods:   ui.Alt,
			}, tt.Now()) {
				t.Fatal("Option+PointerMove step 1 not handled")
			}
			if !app.handleCanvasInput(ui.InputEvent{
				Kind:   ui.InputPointerMove,
				X:      localX + 64,
				Y:      localY + 40,
				Button: 0,
				Mods:   0,
			}, tt.Now()) {
				t.Fatal("Option+PointerMove step 2 (Option released mid-drag) not handled")
			}
			if !app.handleCanvasInput(ui.InputEvent{
				Kind:   ui.InputPointerUp,
				X:      localX + 64,
				Y:      localY + 40,
				Button: 0,
				Mods:   0,
			}, tt.Now()) {
				t.Fatal("Option+PointerUp (Option released mid-drag) not handled")
			}
			if v.gliding() || v.X != beforeX-64 || v.Y != beforeY-40 {
				t.Fatalf("Option+left drag = (%v,%v) gliding=%v, want (%v,%v) gliding=false",
					v.X, v.Y, v.gliding(), beforeX-64, beforeY-40)
			}
			if g.FirstClick || g.Moves != 0 || g.FlagsPlaced != 0 {
				t.Fatal("Option+left drag leaked cell action")
			}

			// Option+left drag in flag mode pans without placing a flag.
			app.flagMode = true
			v.centerBoard(g)
			tt.Frame()
			beforeX, beforeY = v.X, v.Y
			app.handleCanvasInput(ui.InputEvent{Kind: ui.InputPointerDown, X: localX, Y: localY, Button: 0, Mods: ui.Alt}, tt.Now())
			app.handleCanvasInput(ui.InputEvent{Kind: ui.InputPointerMove, X: localX + 28, Y: localY + 18, Button: 0, Mods: ui.Alt}, tt.Now())
			app.handleCanvasInput(ui.InputEvent{Kind: ui.InputPointerUp, X: localX + 28, Y: localY + 18, Button: 0, Mods: ui.Alt}, tt.Now())
			app.flagMode = false
			if v.gliding() || v.X != beforeX-28 || v.Y != beforeY-18 {
				t.Fatalf("Option+left drag in flagMode = (%v,%v) gliding=%v, want (%v,%v) gliding=false",
					v.X, v.Y, v.gliding(), beforeX-28, beforeY-18)
			}
			if g.FirstClick || g.Moves != 0 || g.FlagsPlaced != 0 {
				t.Fatal("Option+left drag in flagMode leaked cell action")
			}

			// Option+left drag starting over the minimap pans instead of triggering a minimap glide.
			v.centerBoard(g)
			tt.Frame()
			beforeX, beforeY = v.X, v.Y
			mini := v.minimapBoundsFor(g)
			mx, my := mini.X+mini.W*0.5, mini.Y+mini.H*0.5
			app.handleCanvasInput(ui.InputEvent{Kind: ui.InputPointerDown, X: mx, Y: my, Button: 0, Mods: ui.Alt}, tt.Now())
			app.handleCanvasInput(ui.InputEvent{Kind: ui.InputPointerMove, X: mx - 30, Y: my - 20, Button: 0, Mods: ui.Alt}, tt.Now())
			app.handleCanvasInput(ui.InputEvent{Kind: ui.InputPointerUp, X: mx - 30, Y: my - 20, Button: 0, Mods: ui.Alt}, tt.Now())
			if v.gliding() || v.X != beforeX+30 || v.Y != beforeY+20 {
				t.Fatalf("Option+left drag from minimap = (%v,%v) gliding=%v, want (%v,%v) gliding=false",
					v.X, v.Y, v.gliding(), beforeX+30, beforeY+20)
			}
			if g.FirstClick || g.Moves != 0 || g.FlagsPlaced != 0 {
				t.Fatal("Option+left drag from minimap leaked cell action")
			}

			// Mid-glide Option+left drag cancels the spring immediately and pans from displayed coordinates.
			v.center(g, 10, 10)
			tt.Frame()
			tt.Key(ui.Ctrl, ui.Key0)
			tt.Advance(60 * time.Millisecond)
			if !v.gliding() {
				t.Fatal("expected active glide before mid-glide Option+left drag")
			}
			glideX, glideY := v.X, v.Y
			app.handleCanvasInput(ui.InputEvent{Kind: ui.InputPointerDown, X: localX, Y: localY, Button: 0, Mods: ui.Alt}, tt.Now())
			if v.gliding() || v.X != glideX || v.Y != glideY {
				t.Fatalf("mid-glide Option+PointerDown did not freeze camera: gliding=%v pos=(%v,%v) want=(%v,%v)",
					v.gliding(), v.X, v.Y, glideX, glideY)
			}
			app.handleCanvasInput(ui.InputEvent{Kind: ui.InputPointerMove, X: localX - 32, Y: localY - 24, Button: 0, Mods: ui.Alt}, tt.Now())
			app.handleCanvasInput(ui.InputEvent{Kind: ui.InputPointerUp, X: localX - 32, Y: localY - 24, Button: 0, Mods: ui.Alt}, tt.Now())
			wantGlideX, wantGlideY := glideX+32, glideY+24
			if v.gliding() || abs32(v.X-wantGlideX) > 0.5 || abs32(v.Y-wantGlideY) > 0.5 {
				t.Fatalf("mid-glide Option+left drag ended at (%v,%v) gliding=%v, want (%v,%v) gliding=false",
					v.X, v.Y, v.gliding(), wantGlideX, wantGlideY)
			}
			if g.FirstClick || g.Moves != 0 || g.FlagsPlaced != 0 {
				t.Fatal("mid-glide Option+left drag leaked cell action")
			}

			// 3. Middle-button drag (both <=5px and >5px) and zero-move click remain unchanged via ui.Tester.
			v.centerBoard(g)
			tt.Frame()
			beforeX, beforeY = v.X, v.Y
			tt.PressButton(2, screenX, screenY)
			tt.Move(screenX+3, screenY+2)
			tt.ReleaseButton(2, screenX+3, screenY+2)
			if v.gliding() || v.X != beforeX-3 || v.Y != beforeY-2 {
				t.Fatalf("middle small drag = (%v,%v) gliding=%v, want (%v,%v) gliding=false",
					v.X, v.Y, v.gliding(), beforeX-3, beforeY-2)
			}

			v.centerBoard(g)
			tt.Frame()
			beforeX, beforeY = v.X, v.Y
			tt.PressButton(2, screenX, screenY)
			tt.Move(screenX+60, screenY+45)
			tt.ReleaseButton(2, screenX+60, screenY+45)
			if v.gliding() || v.X != beforeX-60 || v.Y != beforeY-45 {
				t.Fatalf("middle drag = (%v,%v) gliding=%v, want (%v,%v) gliding=false",
					v.X, v.Y, v.gliding(), beforeX-60, beforeY-45)
			}

			beforeX, beforeY = v.X, v.Y
			tt.PressButton(2, screenX, screenY)
			tt.ReleaseButton(2, screenX, screenY)
			if v.gliding() || v.X != beforeX || v.Y != beforeY {
				t.Fatalf("middle click shifted camera: pos=(%v,%v) want=(%v,%v)", v.X, v.Y, beforeX, beforeY)
			}
			if g.FirstClick || g.Moves != 0 || g.FlagsPlaced != 0 {
				t.Fatal("middle-button gestures leaked cell action")
			}

			// 4. Unmodified left-button drag (>5px) still pans without revealing,
			// while unmodified left-button click (<=5px) still reveals without panning.
			v.centerBoard(g)
			tt.Frame()
			beforeX, beforeY = v.X, v.Y
			tt.Press(screenX, screenY)
			tt.Move(screenX+70, screenY+45)
			tt.Release(screenX+70, screenY+45)
			if v.gliding() || v.X != beforeX-70 || v.Y != beforeY-45 {
				t.Fatalf("unmodified left drag = (%v,%v) gliding=%v, want (%v,%v) gliding=false",
					v.X, v.Y, v.gliding(), beforeX-70, beforeY-45)
			}
			if g.FirstClick || g.Moves != 0 || g.FlagsPlaced != 0 {
				t.Fatal("unmodified left drag leaked cell action")
			}

			beforeX, beforeY = v.X, v.Y
			wantR, wantC, ok := v.cellAt(g, localX+2, localY+1)
			if !ok {
				t.Fatal("target cell out of bounds for unmodified left click")
			}
			tt.Press(screenX, screenY)
			tt.Move(screenX+2, screenY+1)
			tt.Release(screenX+2, screenY+1)
			if v.X != beforeX || v.Y != beforeY {
				t.Fatalf("unmodified left click (<=5px) unexpectedly panned: pos=(%v,%v) want=(%v,%v)",
					v.X, v.Y, beforeX, beforeY)
			}
			if !g.Cell(wantR, wantC).Revealed {
				t.Fatalf("unmodified left click did not reveal (%d,%d)", wantR, wantC)
			}
		})
	}
}
