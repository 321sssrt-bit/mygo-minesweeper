package main

import (
	"fmt"
	"image"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func makeCenterTestBoard(rows, cols int, isDuel, dark bool) (*minesweeperApp, string) {
	app := newMinesweeperApp(42)
	app.darkMode = dark
	label := "经典棋盘"
	if isDuel {
		label = "决斗棋盘"
		app.diffIdx = findDifficultyIdx("duel")
		app.game = NewGame(Difficulty{
			ID:           fmt.Sprintf("center-duel-%dx%d", rows, cols),
			Song:         "春日影",
			Gameplay:     GameplayDuel,
			Rows:         rows,
			Cols:         cols,
			Mines:        2,
			SkillCharges: 2,
			DuelDuration: 300,
			AIInterval:   1e9,
		}, true, MemberTomori, 42)
		app.game.duel.NextAI = 1e9
	} else {
		app.game = NewGame(Difficulty{
			ID:           fmt.Sprintf("center-classic-%dx%d", rows, cols),
			Song:         "诗超绊",
			Gameplay:     GameplayClassic,
			Rows:         rows,
			Cols:         cols,
			Mines:        2,
			SkillCharges: 2,
		}, true, MemberTomori, 42)
	}
	app.game.FirstClick = true
	app.game.Status = StatusPlaying
	app.game.Cell(0, 0).IsMine = true
	app.game.Cell(0, 1).IsMine = true
	app.game.RecomputeAdjacency()
	return app, label
}

// expectedCenteredCellRect computes a cell's screen rectangle from stage and
// board dimensions directly when the board is centered on smaller axes.
func expectedCenteredCellRect(board ui.Rect, v *boardView, g *Game, row, col int) ui.Rect {
	boardW := float32(g.Cols) * v.Cell
	boardH := float32(g.Rows) * v.Cell
	ox := -v.X
	if boardW < board.W {
		ox = (board.W - boardW) / 2
	}
	oy := -v.Y
	if boardH < board.H {
		oy = (board.H - boardH) / 2
	}
	return ui.Rect{
		X: board.X + ox + float32(col)*v.Cell,
		Y: board.Y + oy + float32(row)*v.Cell,
		W: v.Cell,
		H: v.Cell,
	}
}

func canvasRectDiffers(a, b image.Image, r ui.Rect, scale float32) bool {
	bounds := a.Bounds()
	x0 := max(bounds.Min.X, int(r.X*scale))
	y0 := max(bounds.Min.Y, int(r.Y*scale))
	x1 := min(bounds.Max.X, int((r.X+r.W)*scale))
	y1 := min(bounds.Max.Y, int((r.Y+r.H)*scale))
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			if a.At(x, y) != b.At(x, y) {
				return true
			}
		}
	}
	return false
}

func TestCanvasZoomOutCenteredGeometryMarginsAndHitTesting(t *testing.T) {
	cases := []struct {
		name        string
		rows, cols  int
		isDuel      bool
		useTestView bool
		winW, winH  int
		cell        float32
	}{
		{"classic_30x30_canvas_small", 30, 30, false, true, 960, 640, minCanvasCell},
		{"classic_31x31_small", 31, 31, false, false, 960, 640, minCanvasCell},
		{"classic_31x31_default", 31, 31, false, false, 1280, 900, minCanvasCell},
		{"classic_31x31_large", 31, 31, false, false, 1680, 1050, minCanvasCell},
		{"classic_50x50_default_with_minimap", 50, 50, false, false, 1280, 900, minCanvasCell},
		{"classic_50x50_large_with_minimap", 50, 50, false, false, 1680, 1050, minCanvasCell},
		{"duel_30x30_default", 30, 30, true, false, 1280, 900, minCanvasCell},
		{"duel_50x50_large_with_minimap", 50, 50, true, false, 1680, 1050, minCanvasCell},
	}

	for _, tc := range cases {
		for _, dark := range []bool{false, true} {
			theme := "day"
			if dark {
				theme = "night"
			}
			t.Run(fmt.Sprintf("%s/%s", tc.name, theme), func(t *testing.T) {
				app, label := makeCenterTestBoard(tc.rows, tc.cols, tc.isDuel, dark)
				viewFn := app.view
				if tc.useTestView {
					viewFn = classicCanvasTestView(app)
				}
				tt := ui.NewTester(viewFn, tc.winW, tc.winH)
				tt.SetScale(2)
				tt.Frame()

				board, ok := tt.Find(label)
				if !ok {
					t.Fatalf("%q not found", label)
				}
				v, g := app.board, app.game
				v.zoom(g, board.W/2, board.H/2, tc.cell/v.Cell)
				tt.Frame()

				boardW := float32(g.Cols) * v.Cell
				boardH := float32(g.Rows) * v.Cell
				if boardW >= board.W || boardH >= board.H {
					t.Fatalf("precondition failed: board %.1fx%.1f not smaller than stage %.1fx%.1f",
						boardW, boardH, board.W, board.H)
				}

				wantX := (boardW - board.W) / 2
				wantY := (boardH - board.H) / 2
				if abs32(v.X-wantX) > 0.01 || abs32(v.Y-wantY) > 0.01 {
					t.Fatalf("centered offset = (%v,%v), want (%v,%v)", v.X, v.Y, wantX, wantY)
				}

				leftMargin := -v.X
				rightMargin := board.W - (boardW - v.X)
				topMargin := -v.Y
				bottomMargin := board.H - (boardH - v.Y)
				if abs32(leftMargin-rightMargin) > 0.01 || abs32(topMargin-bottomMargin) > 0.01 {
					t.Fatalf("margins asymmetric: L=%.2f R=%.2f T=%.2f B=%.2f",
						leftMargin, rightMargin, topMargin, bottomMargin)
				}

				// Rendered pixels in all 4 margins match notebook paper (not slips),
				// while the centered board interior renders slips.
				pal := paperFor(dark)
				img := tt.Image()
				assertColorNear(t, "left margin paper", sampleCanvasPixel(img, board.X+leftMargin/2, board.Y+board.H/2, 2), pal.Board)
				assertColorNear(t, "right margin paper", sampleCanvasPixel(img, board.X+board.W-rightMargin/2, board.Y+board.H/2, 2), pal.Board)
				assertColorNear(t, "top margin paper", sampleCanvasPixel(img, board.X+board.W/2, board.Y+topMargin/2, 2), pal.Board)
				assertColorNear(t, "bottom margin paper", sampleCanvasPixel(img, board.X+board.W/2, board.Y+board.H-bottomMargin/2, 2), pal.Board)

				// Symmetry check on painted slip edges: sample inside the interior of
				// edge cells (avoiding ruled grid lines) and 3px outside each board edge.
				midRowY := board.Y + topMargin + (float32(g.Rows/2)+0.5)*v.Cell
				midColX := board.X + leftMargin + (float32(g.Cols/2)+0.5)*v.Cell
				edgeSamples := []struct {
					name       string
					inX, inY   float32
					outX, outY float32
				}{
					{"left", board.X + leftMargin + v.Cell/2, midRowY, board.X + leftMargin - 3, midRowY},
					{"right", board.X + leftMargin + boardW - v.Cell/2, midRowY, board.X + leftMargin + boardW + 3, midRowY},
					{"top", midColX, board.Y + topMargin + v.Cell/2, midColX, board.Y + topMargin - 3},
					{"bottom", midColX, board.Y + topMargin + boardH - v.Cell/2, midColX, board.Y + topMargin + boardH + 3},
				}
				for _, es := range edgeSamples {
					assertColorNear(t, es.name+" inside board is slip", sampleCanvasPixel(img, es.inX, es.inY, 2), pal.Slip)
					assertColorNear(t, es.name+" outside board is paper", sampleCanvasPixel(img, es.outX, es.outY, 2), pal.Board)
				}

				// Panning attempts keep both axes locked at center when clear of the minimap.
				v.pan(g, 120, -90)
				if abs32(v.X-wantX) > 0.01 || abs32(v.Y-wantY) > 0.01 {
					t.Fatalf("pan shifted centered board: got (%v,%v), want (%v,%v)", v.X, v.Y, wantX, wantY)
				}
				tt.Scroll(board.X+board.W/2, board.Y+board.H/2, 80, 60)
				if abs32(v.X-wantX) > 0.01 || abs32(v.Y-wantY) > 0.01 {
					t.Fatalf("scroll shifted centered board: got (%v,%v), want (%v,%v)", v.X, v.Y, wantX, wantY)
				}

				// Clicking in all 4 paper margins hits no cell and mutates nothing.
				marginPts := [][2]float32{
					{leftMargin / 2, board.H / 2},
					{board.W - rightMargin/2, board.H / 2},
					{board.W / 2, topMargin / 2},
					{board.W / 2, board.H - bottomMargin/2},
				}
				for _, pt := range marginPts {
					if r, c, hit := v.cellAt(g, pt[0], pt[1]); hit {
						t.Fatalf("margin point (%v,%v) unexpectedly hit cell (%d,%d)", pt[0], pt[1], r, c)
					}
					if !v.minimapFor(g, pt[0], pt[1]) {
						tt.Move(board.X+pt[0], board.Y+pt[1])
						tt.Frame()
						if v.HoverRow != -1 || v.HoverCol != -1 {
							t.Fatalf("margin hover leaked cell (%d,%d)", v.HoverRow, v.HoverCol)
						}
						tt.ClickAt(board.X+pt[0], board.Y+pt[1])
						if g.Moves != 0 || g.FlagsPlaced != 0 {
							t.Fatalf("margin click mutated game: moves=%d flags=%d", g.Moves, g.FlagsPlaced)
						}
					}
				}

				// Clicking and hovering exact cells on the centered board hit those exact cells.
				// Use (1,1) (Adjacent=2, single safe cell, no flood) for reveal and (1,0) for flag.
				revealR, revealC := 1, 1
				flagR, flagC := 1, 0
				revRect := expectedCenteredCellRect(board, v, g, revealR, revealC)
				flagRect := expectedCenteredCellRect(board, v, g, flagR, flagC)

				tt.Move(revRect.X+revRect.W/2, revRect.Y+revRect.H/2)
				tt.Frame()
				if v.HoverRow != revealR || v.HoverCol != revealC {
					t.Fatalf("hover on centered (%d,%d) = (%d,%d)", revealR, revealC, v.HoverRow, v.HoverCol)
				}
				tt.ClickAt(revRect.X+revRect.W/2, revRect.Y+revRect.H/2)
				if !g.Cell(revealR, revealC).Revealed || g.Moves != 1 {
					t.Fatalf("click on centered (%d,%d) missed: revealed=%v moves=%d",
						revealR, revealC, g.Cell(revealR, revealC).Revealed, g.Moves)
				}
				tt.RightClickAt(flagRect.X+flagRect.W/2, flagRect.Y+flagRect.H/2)
				if !g.Cell(flagR, flagC).Flagged || g.FlagsPlaced != 1 {
					t.Fatalf("right click on centered (%d,%d) missed: flagged=%v total=%d",
						flagR, flagC, g.Cell(flagR, flagC).Flagged, g.FlagsPlaced)
				}

				// Minimap viewport rect spans the whole minimap when the entire board is visible.
				if mini := v.minimapBoundsFor(g); mini.W > 0 && mini.H > 0 {
					plot := mini
					if !tc.isDuel {
						plot = classicMinimapPlot(mini)
					}
					view := minimapViewportRect(plot, g.Rows, g.Cols, v.X, v.Y, board.W, board.H, v.Cell)
					if abs32(view.X-plot.X) > 0.01 || abs32(view.Y-plot.Y) > 0.01 ||
						abs32(view.W-plot.W) > 0.01 || abs32(view.H-plot.H) > 0.01 {
						t.Fatalf("minimap viewport = %+v, want full plot %+v", view, plot)
					}
					// Clicking minimap does not shift the centered board.
					tt.ClickAt(board.X+mini.X+mini.W/4, board.Y+mini.Y+mini.H/4)
					if v.gliding() || abs32(v.X-wantX) > 0.01 || abs32(v.Y-wantY) > 0.01 {
						t.Fatalf("minimap click shifted centered board: gliding=%v pos=(%v,%v) want=(%v,%v)",
							v.gliding(), v.X, v.Y, wantX, wantY)
					}
				}
			})
		}
	}
}

func TestCanvasZoomOutSingleAxisCenteringAndMinimapOverscroll(t *testing.T) {
	t.Run("narrow_tall_centers_X_and_pans_Y", func(t *testing.T) {
		app, label := makeCenterTestBoard(80, 30, false, false)
		tt := ui.NewTester(app.view, 1280, 900)
		tt.Frame()
		board, ok := tt.Find(label)
		if !ok {
			t.Fatalf("%q not found", label)
		}
		v, g := app.board, app.game
		v.zoom(g, board.W/2, board.H/2, 16/v.Cell)
		tt.Frame()

		boardW := float32(g.Cols) * v.Cell
		boardH := float32(g.Rows) * v.Cell
		if boardW >= board.W || boardH <= board.H {
			t.Fatalf("precondition failed: board=%.1fx%.1f stage=%.1fx%.1f", boardW, boardH, board.W, board.H)
		}
		wantX := (boardW - board.W) / 2
		if abs32(v.X-wantX) > 0.01 {
			t.Fatalf("narrow axis X = %v, want centered %v", v.X, wantX)
		}

		v.pan(g, 200, 120)
		if abs32(v.X-wantX) > 0.01 {
			t.Fatalf("narrow axis X shifted on pan: %v, want %v", v.X, wantX)
		}
		if v.Y <= 0 {
			t.Fatalf("tall axis Y did not pan: %v", v.Y)
		}

		// Hit test after panning Y hits the exact visible cell.
		row, col, hit := v.cellAt(g, board.W/2, board.H/2)
		if !hit || col != g.Cols/2 {
			t.Fatalf("center screen hit (%d,%d) ok=%v, want col=%d", row, col, hit, g.Cols/2)
		}
		cr := expectedCenteredCellRect(board, v, g, row, col)
		tt.RightClickAt(cr.X+cr.W/2, cr.Y+cr.H/2)
		if !g.Cell(row, col).Flagged {
			t.Fatalf("right click at centered-X panned-Y cell (%d,%d) missed", row, col)
		}
	})

	t.Run("wide_short_centers_Y_and_pans_X", func(t *testing.T) {
		// 14x80 at Cell=16 (1280x224) is wider than the stage and shorter than
		// the stage with clear vertical margin above the bottom-right minimap
		// across platform font metrics.
		app, label := makeCenterTestBoard(14, 80, false, false)
		tt := ui.NewTester(app.view, 1280, 900)
		tt.Frame()
		board, ok := tt.Find(label)
		if !ok {
			t.Fatalf("%q not found", label)
		}
		v, g := app.board, app.game
		v.zoom(g, board.W/2, board.H/2, 16/v.Cell)
		tt.Frame()

		boardW := float32(g.Cols) * v.Cell
		boardH := float32(g.Rows) * v.Cell
		if boardW <= board.W || boardH >= board.H {
			t.Fatalf("precondition failed: board=%.1fx%.1f stage=%.1fx%.1f", boardW, boardH, board.W, board.H)
		}
		wantY := (boardH - board.H) / 2
		if abs32(v.Y-wantY) > 0.01 {
			t.Fatalf("short axis Y = %v, want centered %v", v.Y, wantY)
		}
		v.pan(g, 150, -150)
		if abs32(v.Y-wantY) > 0.01 {
			t.Fatalf("short axis Y shifted on pan: %v, want %v", v.Y, wantY)
		}
		if v.X <= 0 {
			t.Fatalf("wide axis X did not pan: %v", v.X)
		}
	})

	t.Run("small_centered_board_overlapping_minimap_reserves_overscroll_for_corner_and_skill", func(t *testing.T) {
		// 50x50 classic board in 960x900 zoomed to Cell=10 (500x500):
		// both axes are smaller than the stage, yet the centered bottom-right
		// corner sits beneath the 148x148 minimap. Scrolling or ensureVisible
		// must still pull (49,49) clear of the minimap so it can be flagged.
		app, label := makeCenterTestBoard(50, 50, false, false)
		tt := ui.NewTester(app.view, 960, 900)
		tt.Frame()
		board, ok := tt.Find(label)
		if !ok {
			t.Fatalf("%q not found", label)
		}
		v, g := app.board, app.game
		for range 12 {
			tt.Key(ui.Ctrl, ui.KeyMinus)
			tt.Advance(260 * time.Millisecond)
			if v.Cell <= minCanvasCell {
				break
			}
		}
		if v.Cell != minCanvasCell {
			t.Fatalf("Ctrl+- did not reach minCanvasCell: got %.2f", v.Cell)
		}
		boardW, boardH := float32(g.Cols)*v.Cell, float32(g.Rows)*v.Cell
		if boardW >= board.W || boardH >= board.H {
			t.Fatalf("precondition failed: 50x50 at min zoom %.1fx%.1f not smaller than 960x900 stage %.1fx%.1f",
				boardW, boardH, board.W, board.H)
		}
		// Precondition: at the centered resting position, (49,49) is hidden under the minimap.
		centerX := (boardW - board.W) / 2
		centerY := (boardH - board.H) / 2
		if v.showsAt(g, 49, 49, centerX, centerY, v.Cell) {
			t.Fatalf("precondition failed: expected (49,49) occluded by minimap at centered (%v,%v) on stage %.1fx%.1f",
				centerX, centerY, board.W, board.H)
		}

		// Scrolling pulls (49,49) clear of the minimap and allows right-click flagging.
		tt.Scroll(board.X+board.W/2, board.Y+board.H/2, 200, 200)
		tt.Frame()
		if !v.shows(g, 49, 49) {
			t.Fatalf("(49,49) remained behind minimap after scrolling: pos=(%v,%v)", v.X, v.Y)
		}
		cornerRect := canvasCellRect(board, v, 49, 49)
		tt.RightClickAt(cornerRect.X+cornerRect.W/2, cornerRect.Y+cornerRect.H/2)
		if !g.Cell(49, 49).Flagged {
			t.Fatal("right click on scrolled-clear (49,49) did not flag")
		}

		// visibleTarget also clears (49,49) from the minimap for skill glides.
		tx, ty := v.visibleTarget(g, 49, 49, v.Cell)
		if !v.showsAt(g, 49, 49, tx, ty, v.Cell) {
			t.Fatalf("visibleTarget(49,49) = (%v,%v) still occluded by minimap", tx, ty)
		}

		// Crossing boardH == v.Height by 1px while overscrolled past the minimap
		// must not jump discontinuously.
		v.Width, v.Height, v.Cell = 900, 500, minCanvasCell
		_, maxYAt := v.panLimitFor(g, v.Cell)
		v.Y = maxYAt
		v.clamp(g)
		yBefore := v.Y
		v.Height = 501
		v.clamp(g)
		if abs32(v.Y-yBefore) > 2 {
			t.Fatalf("1px resize across boardH==Height jumped Y from %v to %v", yBefore, v.Y)
		}
	})
}

func TestCanvasZoomThresholdSmoothnessAndResize(t *testing.T) {
	app, label := makeCenterTestBoard(50, 50, false, false)
	tt := ui.NewTester(app.view, 1280, 900)
	tt.Frame()
	board, ok := tt.Find(label)
	if !ok {
		t.Fatalf("%q not found", label)
	}
	v, g := app.board, app.game

	// 1. Zoom out in small steps from exact geometric center across the
	// boardW == stageW and boardH == stageH thresholds: board edges move
	// continuously without any jump.
	v.X = (float32(g.Cols)*v.Cell - board.W) / 2
	v.Y = (float32(g.Rows)*v.Cell - board.H) / 2
	v.clamp(g)
	tt.Frame()
	prevLeft, prevTop := -v.X, -v.Y
	for range 25 {
		beforeCell := v.Cell
		v.zoom(g, board.W/2, board.H/2, 0.94)
		curLeft, curTop := -v.X, -v.Y
		dCell := beforeCell - v.Cell
		wantDeltaX := float32(g.Cols) * dCell * 0.5
		wantDeltaY := float32(g.Rows) * dCell * 0.5
		if abs32((curLeft-prevLeft)-wantDeltaX) > 0.1 {
			t.Fatalf("zoom-out left edge jumped: prev=%.2f cur=%.2f delta=%.2f want=%.2f (cell %.2f->%.2f)",
				prevLeft, curLeft, curLeft-prevLeft, wantDeltaX, beforeCell, v.Cell)
		}
		if abs32((curTop-prevTop)-wantDeltaY) > 0.1 {
			t.Fatalf("zoom-out top edge jumped: prev=%.2f cur=%.2f delta=%.2f want=%.2f (cell %.2f->%.2f)",
				prevTop, curTop, curTop-prevTop, wantDeltaY, beforeCell, v.Cell)
		}
		prevLeft, prevTop = curLeft, curTop
		if v.Cell <= minCanvasCell {
			break
		}
	}
	if v.Cell != minCanvasCell {
		t.Fatalf("expected to reach minCanvasCell %.0f, got %.2f", minCanvasCell, v.Cell)
	}

	// 2. Zoom back in anchored on an off-center cell (35, 35): stays centered while
	// smaller than the stage and clear of the minimap, then preserves the world anchor
	// once panning becomes free.
	anchorRect := expectedCenteredCellRect(board, v, g, 35, 35)
	atX := anchorRect.X - board.X + anchorRect.W/2
	atY := anchorRect.Y - board.Y + anchorRect.H/2
	for range 25 {
		oldX, oldY, oldCell := v.X, v.Y, v.Cell
		wx := (oldX + atX) / oldCell
		wy := (oldY + atY) / oldCell
		tx, ty, nextCell := v.zoomTarget(g, oldX, oldY, oldCell, atX, atY, 1.07)
		v.zoom(g, atX, atY, 1.07)
		if abs32(v.X-tx) > 0.01 || abs32(v.Y-ty) > 0.01 || abs32(v.Cell-nextCell) > 0.01 {
			t.Fatalf("zoom vs zoomTarget mismatch: got (%v,%v,%.2f) want (%v,%v,%.2f)",
				v.X, v.Y, v.Cell, tx, ty, nextCell)
		}
		minX, maxX, minY, maxY := v.panBoundsFor(g, v.Cell)
		if minX == maxX {
			wantX := (float32(g.Cols)*v.Cell - board.W) / 2
			if abs32(v.X-wantX) > 0.01 {
				t.Fatalf("zoom-in while locked centered drifted X: got %v want %v", v.X, wantX)
			}
		} else if v.X > minX && v.X < maxX {
			if math.Abs(float64((v.X+atX)/v.Cell-wx)) > 0.02 {
				t.Fatalf("zoom-in past threshold drifted X anchor: got %.3f want %.3f", (v.X+atX)/v.Cell, wx)
			}
		}
		if minY == maxY {
			wantY := (float32(g.Rows)*v.Cell - board.H) / 2
			if abs32(v.Y-wantY) > 0.01 {
				t.Fatalf("zoom-in while locked centered drifted Y: got %v want %v", v.Y, wantY)
			}
		} else if v.Y > minY && v.Y < maxY {
			if math.Abs(float64((v.Y+atY)/v.Cell-wy)) > 0.02 {
				t.Fatalf("zoom-in past threshold drifted Y anchor: got %.3f want %.3f", (v.Y+atY)/v.Cell, wy)
			}
		}
		if v.Cell >= standardCellPitch {
			break
		}
	}

	// 3. Glide zoom out into centered state and back up to 100% via Ctrl+0
	// settle cleanly with no lingering animation frames or drift.
	// Derive the starting pitch from the laid-out stage so one Ctrl+- step
	// reliably crosses into dual-axis centering regardless of platform font metrics.
	fitCell := min(board.W/float32(g.Cols), board.H/float32(g.Rows))
	v.Cell = max(minCanvasCell*1.1, fitCell*1.05)
	v.centerBoard(g)
	tt.Frame()
	tt.Key(ui.Ctrl, ui.KeyMinus)
	if !v.gliding() {
		t.Fatal("Ctrl+- did not start glide")
	}
	tt.Advance(260 * time.Millisecond)
	if v.gliding() {
		t.Fatal("glide zoom into centered state did not settle")
	}
	if float32(g.Cols)*v.Cell >= board.W || float32(g.Rows)*v.Cell >= board.H {
		t.Fatalf("precondition failed: post-glide board %.1fx%.1f not smaller than stage %.1fx%.1f",
			float32(g.Cols)*v.Cell, float32(g.Rows)*v.Cell, board.W, board.H)
	}
	wantX := (float32(g.Cols)*v.Cell - board.W) / 2
	wantY := (float32(g.Rows)*v.Cell - board.H) / 2
	if abs32(v.X-wantX) > 0.5 || abs32(v.Y-wantY) > 0.5 {
		t.Fatalf("glide zoom into centered state landed at (%v,%v), want (%v,%v)", v.X, v.Y, wantX, wantY)
	}
	tt.Frame()
	if tt.FramePending() {
		t.Fatal("canvas kept requesting frames after centered zoom glide settled")
	}

	// Ctrl+0 from centered small state glides cleanly back to standard pitch centered on board.
	tt.Key(ui.Ctrl, ui.Key0)
	if !v.gliding() {
		t.Fatal("Ctrl+0 from centered small state did not start glide")
	}
	tt.Advance(260 * time.Millisecond)
	if v.gliding() || abs32(v.Cell-standardCellPitch) > 0.05 {
		t.Fatalf("Ctrl+0 from centered small state settled cell=%.2f gliding=%v", v.Cell, v.gliding())
	}
	if rEnd, cEnd, ok := v.cellAt(g, board.W/2, board.H/2); !ok || rEnd != g.Rows/2 || cEnd != g.Cols/2 {
		t.Fatalf("Ctrl+0 from centered small state landed at (%d,%d), want (%d,%d)", rEnd, cEnd, g.Rows/2, g.Cols/2)
	}

	// 4. Resizing the window updates centering automatically.
	v.zoom(g, board.W/2, board.H/2, minCanvasCell/v.Cell)
	tt.Frame()
	tt.SetSize(1680, 1050)
	tt.Frame()
	largeBoard, ok := tt.Find(label)
	if !ok {
		t.Fatalf("%q missing after resize", label)
	}
	wantLargeX := (float32(g.Cols)*v.Cell - largeBoard.W) / 2
	wantLargeY := (float32(g.Rows)*v.Cell - largeBoard.H) / 2
	if abs32(v.X-wantLargeX) > 0.5 || abs32(v.Y-wantLargeY) > 0.5 {
		t.Fatalf("resize to large did not recenter: pos=(%v,%v), want (%v,%v)", v.X, v.Y, wantLargeX, wantLargeY)
	}
}

func TestCanvasZoomOutCenteredEffectsAndOverlaysAlign(t *testing.T) {
	app, label := makeCenterTestBoard(31, 31, false, false)
	tt := ui.NewTester(app.view, 1280, 900)
	tt.SetScale(2)
	tt.Frame()

	board, ok := tt.Find(label)
	if !ok {
		t.Fatalf("%q not found", label)
	}
	v, g := app.board, app.game
	v.zoom(g, board.W/2, board.H/2, minCanvasCell/v.Cell)
	tt.Frame()

	// Hovering cell (10,10) changes pixels at its centered screen position,
	// not at the uncentered top-left position (10*Cell, 10*Cell).
	before := tt.Image()
	centeredRect := expectedCenteredCellRect(board, v, g, 10, 10)
	uncenteredX := board.X + (10.5)*v.Cell
	uncenteredY := board.Y + (10.5)*v.Cell
	tt.Move(centeredRect.X+centeredRect.W/2, centeredRect.Y+centeredRect.H/2)
	tt.Frame()
	afterHover := tt.Image()

	if sampleCanvasPixel(before, centeredRect.X+centeredRect.W/2, centeredRect.Y+centeredRect.H/2, 2) ==
		sampleCanvasPixel(afterHover, centeredRect.X+centeredRect.W/2, centeredRect.Y+centeredRect.H/2, 2) {
		t.Fatal("hover highlight did not render at centered cell (10,10)")
	}
	if sampleCanvasPixel(before, uncenteredX, uncenteredY, 2) !=
		sampleCanvasPixel(afterHover, uncenteredX, uncenteredY, 2) {
		t.Fatal("hover highlight unexpectedly painted at uncentered top-left coordinates")
	}

	// Skill target pulse (triggered by KeyE) renders at the centered target cell,
	// not at uncentered top-left coordinates. Reveal (1,1) (Adjacent=2) so the
	// board stays live without flood-clearing the whole grid.
	g.Reveal(1, 1)
	tt.Move(board.X+2, board.Y+2)
	tt.Frame()
	beforePulse := tt.Image()
	tt.Key(0, ui.KeyE)
	if len(app.skillPulses) == 0 {
		t.Fatal("KeyE did not spawn skill target pulse on centered board")
	}
	pulseIdx := app.skillPulses[0].Index
	pulseRow, pulseCol := pulseIdx/g.Cols, pulseIdx%g.Cols
	tt.Advance(80 * time.Millisecond)
	afterPulse := tt.Image()
	pulseRect := expectedCenteredCellRect(board, v, g, pulseRow, pulseCol)
	if !canvasRectDiffers(beforePulse, afterPulse, pulseRect, 2) {
		t.Fatalf("skill target pulse did not render at centered cell (%d,%d)", pulseRow, pulseCol)
	}

	// Clicking a mine at its centered screen position triggers knot burst at
	// the centered cell coordinates.
	tt.Advance(skillPulseDuration)
	tt.Frame()
	mineRect := expectedCenteredCellRect(board, v, g, 0, 0)
	beforeBurst := tt.Image()
	tt.ClickAt(mineRect.X+mineRect.W/2, mineRect.Y+mineRect.H/2)
	if !g.Cell(0, 0).Exploded || len(app.knotBursts) == 0 {
		t.Fatalf("clicking centered mine (0,0) did not explode with knot burst: exploded=%v bursts=%d",
			g.Cell(0, 0).Exploded, len(app.knotBursts))
	}
	tt.Advance(80 * time.Millisecond)
	afterBurst := tt.Image()
	if !canvasRectDiffers(beforeBurst, afterBurst, mineRect, 2) {
		t.Fatal("knot burst did not render at centered mine (0,0)")
	}
	uncenteredMineRect := ui.Rect{X: board.X, Y: board.Y, W: v.Cell, H: v.Cell}
	if canvasRectDiffers(beforeBurst, afterBurst, uncenteredMineRect, 2) {
		t.Fatal("knot burst unexpectedly rendered at uncentered top-left (0,0)")
	}
}

func stageCanvasZoomOutCenteredScene(dark bool) *minesweeperApp {
	app := newMinesweeperApp(20261116)
	app.darkMode = dark
	app.memberID = MemberSoyorin
	app.allowQuestion = true
	app.diffIdx = findDifficultyIdx(TrackUtakotoba.ID)
	app.resetGame(true)
	pinStartQuote(app)
	g := app.game
	g.Reveal(classicCanvasStageRow, classicCanvasStageCol)
	for row := classicCanvasStageRow - 8; row <= classicCanvasStageRow+8; row++ {
		for col := classicCanvasStageCol - 10; col <= classicCanvasStageCol+10; col++ {
			c := g.Cell(row, col)
			if c != nil && !c.Revealed && !c.IsMine && c.Adjacent == 0 && (row*g.Cols+col)%5 == 0 {
				g.Reveal(row, col)
			}
		}
	}
	flagged := 0
	for row := classicCanvasStageRow - 8; row <= classicCanvasStageRow+8; row++ {
		for col := classicCanvasStageCol - 10; col <= classicCanvasStageCol+10; col++ {
			c := g.Cell(row, col)
			if c == nil || !c.IsMine || c.Revealed || c.Flagged || !hasRevealedNeighbour(g, row, col) {
				continue
			}
			if flagged < 8 {
				g.ToggleFlag(row, col, false)
				flagged++
			}
		}
	}
	return app
}

// TestSnapshotCanvasZoomOutCentered renders a large classic canvas board zoomed
// out to its minimum cell size across all three viewports in day and night
// themes when MYGO_SNAPSHOT_DIR is set.
func TestSnapshotCanvasZoomOutCentered(t *testing.T) {
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
		for _, vp := range snapshotViewports {
			app := stageCanvasZoomOutCenteredScene(dark)
			tt := ui.NewTester(app.view, vp.width, vp.height)
			tt.SetScale(2)
			tt.Frame()
			board, ok := tt.Find("经典棋盘")
			if !ok {
				t.Fatalf("经典棋盘 missing at %s", vp.name)
			}
			app.board.zoom(app.game, board.W/2, board.H/2, minCanvasCell/app.board.Cell)
			tt.Frame()
			path := filepath.Join(dir, fmt.Sprintf("canvas-zoomout-center-%s-%s.png", vp.name, suffix))
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
}
