package main

import (
	"fmt"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func toolbarRectsOverlap(a, b ui.Rect) bool {
	return a.X < b.X+b.W && a.X+a.W > b.X && a.Y < b.Y+b.H && a.Y+a.H > b.Y
}

func toolbarRectInside(inner, outer ui.Rect) bool {
	return inner.X >= outer.X-0.5 &&
		inner.Y >= outer.Y-0.5 &&
		inner.X+inner.W <= outer.X+outer.W+0.5 &&
		inner.Y+inner.H <= outer.Y+outer.H+0.5
}

func focusByTab(t *testing.T, tt *ui.Tester, target string, maxSteps int) {
	t.Helper()
	for range maxSteps {
		if tt.Focused(target) {
			return
		}
		tt.Key(0, ui.KeyTab)
	}
	t.Fatalf("Tab traversal did not reach %q within %d steps", target, maxSteps)
}

func TestCanvasCornerToolbarLayoutVisibilityAndNonOverlap(t *testing.T) {
	t.Run("duel_and_classic_canvas_across_viewports", func(t *testing.T) {
		cases := []struct {
			name      string
			trackID   string
			boardName string
			wantAI    bool
		}{
			{"duel-1000x1000", "duel", "决斗棋盘", true},
			{"duel-500x500", "senzaihyoumei", "决斗棋盘", true},
			{"classic-30x30", TrackKageiromai.ID, "经典棋盘", false},
			{"classic-50x50", TrackUtakotoba.ID, "经典棋盘", false},
			{"classic-100x100", TrackMayoiuta.ID, "经典棋盘", false},
		}

		for _, tc := range cases {
			for _, vp := range snapshotViewports {
				t.Run(fmt.Sprintf("%s/%s", tc.name, vp.name), func(t *testing.T) {
					app := newMinesweeperApp(42)
					app.diffIdx = findDifficultyIdx(tc.trackID)
					app.resetGame(true)
					if tc.wantAI {
						app.game.duel.NextAI = 1e9
					}
					tt := ui.NewTester(app.view, vp.width, vp.height)
					tt.UseScreenReader()
					tt.Frame()

					boardRect, ok := tt.Find(tc.boardName)
					if !ok {
						t.Fatalf("%s missing", tc.boardName)
					}
					barInfo, ok := tt.AccessByRole(ui.RoleToolbar, "画布镜头")
					if !ok {
						t.Fatal("canvas toolbar missing from accessibility tree")
					}
					if !toolbarRectInside(barInfo.Bounds, boardRect) {
						t.Fatalf("toolbar %+v escaped board %+v", barInfo.Bounds, boardRect)
					}
					// Sits in the bottom-left corner of the canvas.
					if barInfo.Bounds.X-boardRect.X > 24 || (boardRect.Y+boardRect.H)-(barInfo.Bounds.Y+barInfo.Bounds.H) > 24 {
						t.Fatalf("toolbar %+v not anchored in bottom-left of board %+v", barInfo.Bounds, boardRect)
					}

					// Never overlaps the bottom-right minimap when present.
					if mini := app.board.minimapBoundsFor(app.game); mini.W > 0 && mini.H > 0 {
						miniScreen := ui.Rect{
							X: boardRect.X + mini.X,
							Y: boardRect.Y + mini.Y,
							W: mini.W,
							H: mini.H,
						}
						if toolbarRectsOverlap(barInfo.Bounds, miniScreen) {
							t.Fatalf("toolbar %+v overlaps minimap %+v", barInfo.Bounds, miniScreen)
						}
					}

					// Old top camera buttons and old bottom-left static caption are gone.
					for _, retired := range []string{"放大", "缩小"} {
						if _, found := tt.Find(retired); found {
							t.Fatalf("retired camera button %q still mounted", retired)
						}
					}
					if tt.HasText("拖动平移 / 滚轮缩放") {
						t.Fatal("retired static corner caption still rendered")
					}

					// Required buttons exist inside the toolbar with RoleButton and focusability.
					for _, name := range []string{"回到中心", "回标准大小"} {
						btn, ok := tt.AccessByRole(ui.RoleButton, name)
						if !ok || !btn.Focusable || btn.Disabled {
							t.Fatalf("button %q invalid: ok=%v focusable=%v disabled=%v", name, ok, btn.Focusable, btn.Disabled)
						}
						if !toolbarRectInside(btn.Bounds, barInfo.Bounds) {
							t.Fatalf("button %q %+v outside toolbar %+v", name, btn.Bounds, barInfo.Bounds)
						}
					}
					resetBtn, _ := tt.AccessByRole(ui.RoleButton, "回标准大小")
					if resetBtn.Value != "100%" {
						t.Fatalf("initial 回标准大小 value = %q, want 100%%", resetBtn.Value)
					}

					followBtn, hasFollow := tt.AccessByRole(ui.RoleButton, "定位高松灯")
					if hasFollow != tc.wantAI {
						t.Fatalf("定位高松灯 present=%v, want %v", hasFollow, tc.wantAI)
					}
					if tc.wantAI {
						if !followBtn.Focusable || followBtn.Disabled || !toolbarRectInside(followBtn.Bounds, barInfo.Bounds) {
							t.Fatalf("定位高松灯 invalid: focusable=%v disabled=%v bounds=%+v bar=%+v",
								followBtn.Focusable, followBtn.Disabled, followBtn.Bounds, barInfo.Bounds)
						}
					}
				})
			}
		}
	})

	t.Run("small_window_classic_fallback_has_toolbar_without_follow_and_grid_omits_it", func(t *testing.T) {
		app := newMinesweeperApp(42)
		app.diffIdx = 1 // 16x16: grid at 1280x900, canvas fallback at 960x640
		app.resetGame(true)
		tt := ui.NewTester(app.view, 1280, 900)
		tt.UseScreenReader()
		tt.Frame()

		if app.classicOnCanvas() {
			t.Fatal("expected 16x16 on widget grid at 1280x900")
		}
		if _, ok := tt.AccessByRole(ui.RoleToolbar, "画布镜头"); ok {
			t.Fatal("widget grid should not mount canvas corner toolbar")
		}
		for _, name := range []string{"回到中心", "回标准大小", "定位高松灯"} {
			if _, ok := tt.Find(name); ok {
				t.Fatalf("widget grid unexpectedly mounted %q", name)
			}
		}

		tt.SetSize(960, 640)
		tt.Frame()
		if !app.classicOnCanvas() {
			t.Fatal("expected 16x16 to fall back to canvas at 960x640")
		}
		if _, ok := tt.AccessByRole(ui.RoleButton, "回到中心"); !ok {
			t.Fatal("canvas fallback missing 回到中心")
		}
		if _, ok := tt.AccessByRole(ui.RoleButton, "回标准大小"); !ok {
			t.Fatal("canvas fallback missing 回标准大小")
		}
		if _, ok := tt.AccessByRole(ui.RoleButton, "定位高松灯"); ok {
			t.Fatal("classic canvas fallback must not show 定位高松灯")
		}
	})
}

func TestCanvasCornerToolbarActionsDuelAndClassic(t *testing.T) {
	t.Run("duel_all_three_buttons", func(t *testing.T) {
		app, tt, boardRect := setupCameraDuelApp(t)
		tt.UseScreenReader()
		v, g := app.board, app.game

		// 1. 定位高松灯 before AI moves is a safe no-op.
		beforeX, beforeY := v.X, v.Y
		if err := tt.Click("定位高松灯"); err != nil {
			t.Fatalf("click 定位高松灯 before AI move: %v", err)
		}
		if v.gliding() || v.X != beforeX || v.Y != beforeY {
			t.Fatalf("定位高松灯 moved before LastAI set: gliding=%v pos=(%v,%v)", v.gliding(), v.X, v.Y)
		}

		// 2. Pan away, zoom out, then click 回到中心: centers board while keeping zoom.
		v.zoom(g, boardRect.W/2, boardRect.H/2, 0.75)
		zoomedCell := v.Cell
		v.center(g, 120, 240)
		tt.Frame()
		if btn, ok := tt.AccessByRole(ui.RoleButton, "回标准大小"); !ok || btn.Value != v.zoomLabel() {
			t.Fatalf("回标准大小 value = %q ok=%v, want %q", btn.Value, ok, v.zoomLabel())
		}

		if err := tt.Click("回到中心"); err != nil {
			t.Fatalf("click 回到中心: %v", err)
		}
		if !v.gliding() {
			t.Fatal("回到中心 did not start glide")
		}
		tt.Advance(260 * time.Millisecond)
		if v.gliding() {
			t.Fatal("回到中心 did not settle")
		}
		if gotR, gotC, ok := v.cellAt(g, boardRect.W/2, boardRect.H/2); !ok || gotR != g.Rows/2 || gotC != g.Cols/2 {
			t.Fatalf("回到中心 landed at (%d,%d) ok=%v, want (%d,%d)", gotR, gotC, ok, g.Rows/2, g.Cols/2)
		}
		if abs32(v.Cell-zoomedCell) > 0.05 {
			t.Fatalf("回到中心 changed zoom from %.2f to %.2f", zoomedCell, v.Cell)
		}

		// 3. Click 定位高松灯: centers on LastAI while keeping current zoom.
		g.duel.LastAI = 320*g.Cols + 640
		if err := tt.Click("定位高松灯"); err != nil {
			t.Fatalf("click 定位高松灯: %v", err)
		}
		if !v.gliding() {
			t.Fatal("定位高松灯 did not start glide")
		}
		tt.Advance(260 * time.Millisecond)
		if v.gliding() {
			t.Fatal("定位高松灯 did not settle")
		}
		if gotR, gotC, ok := v.cellAt(g, boardRect.W/2, boardRect.H/2); !ok || gotR != 320 || gotC != 640 {
			t.Fatalf("定位高松灯 landed at (%d,%d) ok=%v, want (320,640)", gotR, gotC, ok)
		}
		if abs32(v.Cell-zoomedCell) > 0.05 {
			t.Fatalf("定位高松灯 changed zoom from %.2f to %.2f", zoomedCell, v.Cell)
		}

		// 4. Click 回标准大小 while centered on (320,640): restores exact standardCellPitch
		// while staying anchored on (320,640).
		if err := tt.Click("回标准大小"); err != nil {
			t.Fatalf("click 回标准大小: %v", err)
		}
		if !v.gliding() {
			t.Fatal("回标准大小 did not start glide")
		}
		tt.Advance(260 * time.Millisecond)
		if v.gliding() || v.Cell != standardCellPitch {
			t.Fatalf("回标准大小 settled cell=%.2f gliding=%v, want %.2f", v.Cell, v.gliding(), standardCellPitch)
		}
		if gotR, gotC, ok := v.cellAt(g, boardRect.W/2, boardRect.H/2); !ok || gotR != 320 || gotC != 640 {
			t.Fatalf("回标准大小 drifted viewport center to (%d,%d) ok=%v, want (320,640)", gotR, gotC, ok)
		}
		if btn, ok := tt.AccessByRole(ui.RoleButton, "回标准大小"); !ok || btn.Value != "100%" {
			t.Fatalf("回标准大小 settled value = %q ok=%v, want 100%%", btn.Value, ok)
		}

		// 5. Reduced motion snaps 回标准大小 on frame 0 without gliding.
		tt.SetPreferences(ui.Preferences{ReduceMotion: true})
		v.zoom(g, boardRect.W/2, boardRect.H/2, 1.3)
		tt.Frame()
		if v.Cell == standardCellPitch {
			t.Fatal("precondition failed: expected zoomed-in cell")
		}
		if err := tt.Click("回标准大小"); err != nil {
			t.Fatalf("click 回标准大小 (reduced): %v", err)
		}
		if v.gliding() || v.Cell != standardCellPitch {
			t.Fatalf("reduced 回标准大小: cell=%.2f gliding=%v, want %.2f gliding=false", v.Cell, v.gliding(), standardCellPitch)
		}
	})

	t.Run("classic_large_canvas_center_and_standard_pitch", func(t *testing.T) {
		app, tt, boardRect := setupCameraClassicApp(t, TrackMayoiuta)
		v, g := app.board, app.game

		v.zoom(g, boardRect.W/2, boardRect.H/2, 0.5)
		v.center(g, 40, 60)
		tt.Frame()

		if err := tt.Click("回标准大小"); err != nil {
			t.Fatalf("classic click 回标准大小: %v", err)
		}
		tt.Advance(260 * time.Millisecond)
		if v.gliding() || v.Cell != standardCellPitch {
			t.Fatalf("classic 回标准大小 settled cell=%.2f gliding=%v, want %.2f", v.Cell, v.gliding(), standardCellPitch)
		}
		if gotR, gotC, ok := v.cellAt(g, boardRect.W/2, boardRect.H/2); !ok || gotR != 40 || gotC != 60 {
			t.Fatalf("classic 回标准大小 drifted center to (%d,%d) ok=%v, want (40,60)", gotR, gotC, ok)
		}

		if err := tt.Click("回到中心"); err != nil {
			t.Fatalf("classic click 回到中心: %v", err)
		}
		tt.Advance(260 * time.Millisecond)
		if v.gliding() {
			t.Fatal("classic 回到中心 did not settle")
		}
		if gotR, gotC, ok := v.cellAt(g, boardRect.W/2, boardRect.H/2); !ok || gotR != g.Rows/2 || gotC != g.Cols/2 {
			t.Fatalf("classic 回到中心 landed at (%d,%d) ok=%v, want (%d,%d)", gotR, gotC, ok, g.Rows/2, g.Cols/2)
		}
	})
}

func TestCanvasCornerToolbarKeyboardFocusAndActivation(t *testing.T) {
	for _, key := range []struct {
		name string
		k    ui.Key
	}{
		{"Enter", ui.KeyEnter},
		{"Space", ui.KeySpace},
	} {
		t.Run("duel/"+key.name, func(t *testing.T) {
			app, tt, boardRect := setupCameraDuelApp(t)
			v, g := app.board, app.game
			g.duel.LastAI = 420*g.Cols + 580

			// 1. Focus and activate 回到中心 via keyboard.
			v.center(g, 100, 100)
			tt.Frame()
			focusByTab(t, tt, "回到中心", 64)
			tt.Key(0, key.k)
			tt.Advance(260 * time.Millisecond)
			if r, c, ok := v.cellAt(g, boardRect.W/2, boardRect.H/2); !ok || r != g.Rows/2 || c != g.Cols/2 {
				t.Fatalf("%s on 回到中心 landed at (%d,%d) ok=%v, want (%d,%d)", key.name, r, c, ok, g.Rows/2, g.Cols/2)
			}

			// 2. Tab to 定位高松灯 and activate via keyboard.
			focusByTab(t, tt, "定位高松灯", 8)
			tt.Key(0, key.k)
			tt.Advance(260 * time.Millisecond)
			if r, c, ok := v.cellAt(g, boardRect.W/2, boardRect.H/2); !ok || r != 420 || c != 580 {
				t.Fatalf("%s on 定位高松灯 landed at (%d,%d) ok=%v, want (420,580)", key.name, r, c, ok)
			}

			// 3. Zoom away, Tab to 回标准大小 and activate via keyboard.
			v.zoom(g, boardRect.W/2, boardRect.H/2, 0.55)
			tt.Frame()
			focusByTab(t, tt, "回标准大小", 8)
			tt.Key(0, key.k)
			tt.Advance(260 * time.Millisecond)
			if v.gliding() || v.Cell != standardCellPitch {
				t.Fatalf("%s on 回标准大小 settled cell=%.2f gliding=%v, want %.2f", key.name, v.Cell, v.gliding(), standardCellPitch)
			}
		})
	}

	t.Run("classic/Enter_and_Space", func(t *testing.T) {
		app, tt, boardRect := setupCameraClassicApp(t, TrackUtakotoba)
		v, g := app.board, app.game

		v.center(g, 5, 5)
		v.zoom(g, boardRect.W/2, boardRect.H/2, 0.6)
		tt.Frame()

		focusByTab(t, tt, "回到中心", 64)
		tt.Key(0, ui.KeyEnter)
		tt.Advance(260 * time.Millisecond)
		if r, c, ok := v.cellAt(g, boardRect.W/2, boardRect.H/2); !ok || r != g.Rows/2 || c != g.Cols/2 {
			t.Fatalf("classic Enter on 回到中心 landed at (%d,%d) ok=%v, want (%d,%d)", r, c, ok, g.Rows/2, g.Cols/2)
		}

		focusByTab(t, tt, "回标准大小", 8)
		tt.Key(0, ui.KeySpace)
		tt.Advance(260 * time.Millisecond)
		if v.gliding() || v.Cell != standardCellPitch {
			t.Fatalf("classic Space on 回标准大小 settled cell=%.2f gliding=%v, want %.2f", v.Cell, v.gliding(), standardCellPitch)
		}
	})
}

func TestCanvasCornerToolbarBlocksPointerLeakageToBoard(t *testing.T) {
	for _, isDuel := range []bool{false, true} {
		mode := "classic"
		if isDuel {
			mode = "duel"
		}
		t.Run(mode, func(t *testing.T) {
			var app *minesweeperApp
			var tt *ui.Tester
			var boardRect ui.Rect
			if isDuel {
				app, tt, boardRect = setupCameraDuelApp(t)
			} else {
				app, tt, boardRect = setupCameraClassicApp(t, TrackMayoiuta)
			}
			tt.UseScreenReader()
			v, g := app.board, app.game

			barInfo, ok := tt.AccessByRole(ui.RoleToolbar, "画布镜头")
			if !ok {
				t.Fatal("toolbar missing")
			}
			homeBtn, ok := tt.AccessByRole(ui.RoleButton, "回到中心")
			if !ok {
				t.Fatal("回到中心 missing")
			}
			resetBtn, ok := tt.AccessByRole(ui.RoleButton, "回标准大小")
			if !ok {
				t.Fatal("回标准大小 missing")
			}

			// Precondition: unrevealed cells sit directly beneath the toolbar buttons and edge.
			underHomeR, underHomeC, okH := v.cellAt(g, homeBtn.Bounds.X+homeBtn.Bounds.W/2-boardRect.X, homeBtn.Bounds.Y+homeBtn.Bounds.H/2-boardRect.Y)
			underEdgeR, underEdgeC, okE := v.cellAt(g, barInfo.Bounds.X+2-boardRect.X, barInfo.Bounds.Y+2-boardRect.Y)
			if !okH || !okE {
				t.Fatalf("precondition failed: expected board cells under toolbar (home=%v edge=%v)", okH, okE)
			}

			// Hovering the board then moving onto the toolbar clears board hover.
			tt.Move(boardRect.X+boardRect.W/2, boardRect.Y+boardRect.H/2)
			tt.Frame()
			if v.HoverRow < 0 {
				t.Fatal("precondition failed: expected board hover")
			}
			tt.Move(homeBtn.Bounds.X+homeBtn.Bounds.W/2, homeBtn.Bounds.Y+homeBtn.Bounds.H/2)
			tt.Frame()
			if v.HoverRow != -1 || v.HoverCol != -1 {
				t.Fatalf("moving pointer onto toolbar kept board hover (%d,%d)", v.HoverRow, v.HoverCol)
			}

			// Left-click, right-click, flag-mode click, middle-drag, scroll, and pinch on
			// both a button and the strip padding must never reveal, flag, pan, or zoom the canvas.
			pts := []struct {
				name string
				x, y float32
			}{
				{"home-button", homeBtn.Bounds.X + homeBtn.Bounds.W/2, homeBtn.Bounds.Y + homeBtn.Bounds.H/2},
				{"reset-button", resetBtn.Bounds.X + resetBtn.Bounds.W/2, resetBtn.Bounds.Y + resetBtn.Bounds.H/2},
				{"toolbar-padding", barInfo.Bounds.X + 2, barInfo.Bounds.Y + 2},
			}
			for _, pt := range pts {
				camX, camY, camCell := v.X, v.Y, v.Cell
				tt.ClickAt(pt.x, pt.y)
				tt.RightClickAt(pt.x, pt.y)
				tt.PressButton(2, pt.x, pt.y)
				tt.Move(pt.x+30, pt.y-20)
				tt.ReleaseButton(2, pt.x+30, pt.y-20)
				tt.Scroll(pt.x, pt.y, 40, 40)
				tt.Pinch(pt.x, pt.y, 1.25)

				app.flagMode = true
				tt.Frame()
				tt.ClickAt(pt.x, pt.y)
				app.flagMode = false
				tt.Frame()

				if g.FirstClick || g.Moves != 0 || g.FlagsPlaced != 0 ||
					g.Cell(underHomeR, underHomeC).Revealed || g.Cell(underHomeR, underHomeC).Flagged ||
					g.Cell(underEdgeR, underEdgeC).Revealed || g.Cell(underEdgeR, underEdgeC).Flagged {
					t.Fatalf("%s leaked cell mutation: first=%v moves=%d flags=%d",
						pt.name, g.FirstClick, g.Moves, g.FlagsPlaced)
				}
				if v.X != camX || v.Y != camY || v.Cell != camCell {
					t.Fatalf("%s leaked camera motion: got (%v,%v,%.2f), want (%v,%v,%.2f)",
						pt.name, v.X, v.Y, v.Cell, camX, camY, camCell)
				}
			}
		})
	}
}

func TestCanvasCornerToolbarPaletteContrast(t *testing.T) {
	for _, dark := range []bool{false, true} {
		pal := paperFor(dark)
		sc := settingsColorsForPaper(pal)
		tomori := memberImage(MemberTomori)

		pairs := []struct {
			name   string
			fg, bg ui.Color
		}{
			{"idle text", sc.LabelInk, pal.Sheet},
			{"idle icon", sc.MutedInk, pal.Sheet},
			{"idle tomori icon", autoInkOn(pal.Sheet, dayPaper.InkOf(tomori), nightPaper.InkOf(tomori)), pal.Sheet},
			{"hover text", sc.TriggerHoverText, sc.TriggerHoverBg},
			{"hover icon", sc.TriggerHoverInk, sc.TriggerHoverBg},
			{"pressed text", autoInkOn(pal.Slip, dayPaper.Ink, nightPaper.Ink), pal.Slip},
			{"pressed icon", autoInkOn(pal.Slip, dayPaper.InkOf(bandColor), nightPaper.InkOf(bandColor)), pal.Slip},
			{"zoomed pill", sc.TriggerOpenInk, sc.TriggerOpenBg},
		}
		for _, p := range pairs {
			if cr := contrast(p.fg, p.bg); cr < minInkContrast {
				t.Errorf("dark=%v %s contrast = %.2f < %.1f (fg=%s bg=%s)",
					dark, p.name, cr, minInkContrast, hexOf(p.fg), hexOf(p.bg))
			}
		}
	}
}
