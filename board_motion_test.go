package main

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func saveTesterPNG(t *testing.T, img *image.RGBA, path string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, img); err != nil {
		f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

func makeCorridorGame(rows, cols int, bandMode bool, member MemberID, mines [][2]int) *Game {
	g := NewGame(Difficulty{
		ID:           "motion-corridor",
		Song:         "迷星叫",
		Subtitle:     fmt.Sprintf("%d×%d", rows, cols),
		Gameplay:     GameplayClassic,
		Rows:         rows,
		Cols:         cols,
		Mines:        len(mines),
		SkillCharges: 2,
	}, bandMode, member, 7)
	g.FirstClick = true
	g.Status = StatusPlaying
	for _, pt := range mines {
		g.Cell(pt[0], pt[1]).IsMine = true
	}
	g.RecomputeAdjacency()
	return g
}

// TestBoundedWavePeelThreeStagesBothBackends verifies the three-stage reveal
// wave (covered slip -> lifting peel -> exposed wash/bare paper), frame-0
// accessibility/hit-box updates, non-blocking input
// mid-wave, over-256-budget distant cells, and pre-staged boards on both the
// classic widget grid and the pan/zoom canvas.
func TestBoundedWavePeelThreeStagesBothBackends(t *testing.T) {
	t.Parallel()
	for _, backend := range []string{"grid", "canvas"} {
		for _, dark := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/dark=%v", backend, dark), func(t *testing.T) {
				app := newMinesweeperApp(7)
				app.darkMode = dark
				app.memberID = MemberAnon
				// 13x13 with one mine at (0,12): clicking (0,0) opens 168 safe cells (>=10),
				// reaching (0,6) at depth 6 (144ms) and (0,11) at depth 11 (capped at 240ms).
				app.game = makeCorridorGame(13, 13, true, MemberAnon, [][2]int{{0, 12}})

				viewFn := app.view
				if backend == "canvas" {
					viewFn = classicCanvasTestView(app)
				}
				tt := ui.NewTester(viewFn, 1280, 900)
				scale := float32(1)
				if backend == "canvas" && dark {
					scale = 2
					tt.SetScale(scale)
				}
				tt.UseScreenReader()
				tt.Frame()

				pal := paperFor(dark)
				cellRect := func(row, col int) ui.Rect {
					if backend == "grid" {
						r, ok := tt.Find(fmt.Sprintf("cell-%d-%d", row, col))
						if !ok {
							t.Fatalf("grid cell-(%d,%d) missing", row, col)
						}
						return r
					}
					br, ok := tt.Find("经典棋盘")
					if !ok {
						t.Fatal("classic canvas missing")
					}
					return classicCanvasCellRect(br, app.board, row, col)
				}

				// Pre-unlock win memes and use (11,0) (depth 11 along left column) so the
				// top-center stage highlight overlays do not sit over the wave sample cell.
				app.unlockedMemes[MemeForever] = true
				app.unlockedMemes[MemeAnonBest] = true
				originRectBefore := cellRect(0, 0)
				farRectBefore := cellRect(11, 0)
				tt.ClickAt(originRectBefore.X+originRectBefore.W/2, originRectBefore.Y+originRectBefore.H/2)

				// Frame 0: game rules, progress, hit-box, and screen reader state update immediately.
				if !app.game.Cell(0, 0).Revealed || !app.game.Cell(0, 6).Revealed || !app.game.Cell(11, 0).Revealed {
					t.Fatal("expected flood-fill cells logically revealed on frame 0")
				}
				if app.game.Status != StatusWon {
					t.Fatalf("expected immediate win on frame 0, status=%v", app.game.Status)
				}
				if got := cellRect(11, 0); got != farRectBefore {
					t.Fatalf("cell hit-box shifted on frame 0: before=%+v after=%+v", farRectBefore, got)
				}
				if backend == "grid" {
					accNum, ok := tt.Access("cell-0-11")
					if !ok || accNum.Value != "1" || accNum.Description != "1" {
						t.Fatalf("expected frame-0 screen reader value/description \"1\" on cell-0-11, got ok=%v node=%+v", ok, accNum)
					}
					accEmpty, ok := tt.Access("cell-0-0")
					if !ok || accEmpty.Value != "0" || accEmpty.Description != "0" {
						t.Fatalf("expected frame-0 screen reader value/description \"0\" on cell-0-0, got ok=%v node=%+v", ok, accEmpty)
					}
				} else {
					accOrigin, ok := tt.Access("经典棋盘")
					if !ok || accOrigin.Value != "0" || accOrigin.Description != "0" {
						t.Fatalf("expected frame-0 canvas screen reader value/description \"0\" at (0,0), got ok=%v node=%+v", ok, accOrigin)
					}
					numRect := cellRect(0, 11)
					tt.Move(numRect.X+numRect.W/2, numRect.Y+numRect.H/2)
					tt.Frame()
					accFar, ok := tt.Access("经典棋盘")
					if !ok || accFar.Value != "1" || accFar.Description != "1" {
						t.Fatalf("expected frame-0 canvas screen reader value/description \"1\" while hovering still-covered (0,11), got ok=%v node=%+v", ok, accFar)
					}
					// Move pointer back to origin so (11,0) paints unhovered pal.Slip for stage-1 check.
					tt.Move(originRectBefore.X+originRectBefore.W/2, originRectBefore.Y+originRectBefore.H/2)
					tt.Frame()
				}

				// Stage 1 (t=0): depth-11 cell (11,0) is still covered by a full paper slip.
				assertColorNear(t, "stage 1 covered far slip", sampleCanvasPixel(tt.Image(), farRectBefore.X+farRectBefore.W/2, farRectBefore.Y+farRectBefore.H/2, scale), pal.Slip)

				// Stage 2 (t=75ms): origin (0,0) is mid-peel, while depth-11 (240ms) is still covered.
				tt.Advance(75 * time.Millisecond)
				originMid := sampleCanvasPixel(tt.Image(), originRectBefore.X+originRectBefore.W/2, originRectBefore.Y+originRectBefore.H/2, scale)
				if near(originMid.R, pal.Slip.R) && near(originMid.G, pal.Slip.G) && near(originMid.B, pal.Slip.B) {
					t.Errorf("origin slip still opaque pal.Slip at 75ms: %s", hexOf(originMid))
				}
				assertColorNear(t, "stage 1 far slip still covered at 75ms", sampleCanvasPixel(tt.Image(), farRectBefore.X+farRectBefore.W/2, farRectBefore.Y+farRectBefore.H/2, scale), pal.Slip)
				if got := cellRect(11, 0); got != farRectBefore {
					t.Fatalf("cell hit-box shifted mid-wave: before=%+v mid=%+v", farRectBefore, got)
				}

				// Stage 2 for far cell (t=240ms+75ms=315ms): far cell is lifting; origin (315ms > 280ms) is already bare paper.
				tt.Advance(240 * time.Millisecond)
				assertColorNear(t, "origin bare after 315ms", sampleCanvasPixel(tt.Image(), originRectBefore.X+originRectBefore.W/2, originRectBefore.Y+originRectBefore.H/2, scale), pal.Board)
				farMid := sampleCanvasPixel(tt.Image(), farRectBefore.X+3, farRectBefore.Y+3, scale)
				if near(farMid.R, pal.Slip.R) && near(farMid.G, pal.Slip.G) && near(farMid.B, pal.Slip.B) {
					t.Errorf("far slip still opaque pal.Slip at delay+75ms: %s", hexOf(farMid))
				}

				// Stage 3 for far cell (t=240ms+160ms=400ms): slip is gone, locked Anon wash exposed.
				tt.Advance(85 * time.Millisecond)
				farWashAlpha := 1 - float32(160*time.Millisecond)/float32(rippleDuration)
				wantFarWash := memberImage(MemberAnon).Alpha(0.26 * farWashAlpha).Over(pal.Board)
				assertColorNear(t, "stage 3 far locked Anon wash", sampleCanvasPixel(tt.Image(), farRectBefore.X+3, farRectBefore.Y+3, scale), wantFarWash)

				// Full convergence: wave entries pruned; once victory highlights settle too,
				// zero lingering animation frames remain.
				tt.Advance(rippleDuration)
				if len(app.ripples) != 0 {
					t.Fatalf("expected all ripples pruned, remaining=%d", len(app.ripples))
				}
				tt.Advance(memeStripDuration)
				assertColorNear(t, "far cell bare after settle", sampleCanvasPixel(tt.Image(), farRectBefore.X+3, farRectBefore.Y+3, scale), pal.Board)
				if backend == "canvas" && tt.FramePending() {
					t.Fatal("canvas still requested animation frames after wave settled")
				}
			})
		}
	}

	// Single-cell reveal (depth=0, delay=0) executes zero-predelay 150ms peel,
	// and mid-animation input (clicking another cell & flagging) works immediately.
	t.Run("single-cell-peel-and-mid-wave-input", func(t *testing.T) {
		app := newMinesweeperApp(7)
		app.memberID = MemberTomori
		// Mines surrounding (1,1) so (0,0) has Adjacent=1 (single-cell reveal),
		// and (0,4) has Adjacent=1 (second click mid-wave), and (4,4) is flaggable.
		app.game = makeCorridorGame(6, 6, true, MemberTomori, [][2]int{{1, 0}, {1, 4}, {5, 5}})
		tt := ui.NewTester(app.view, 1280, 900)
		tt.SetScale(2)
		tt.Frame()
		pal := app.paper()

		r00, _ := tt.Find("cell-0-0")
		tt.ClickAt(r00.X+r00.W/2, r00.Y+r00.H/2)
		if !app.game.Cell(0, 0).Revealed || len(app.ripples) != 1 || app.ripples[0].Delay != 0 {
			t.Fatalf("expected single-cell depth-0 ripple, revealed=%v ripples=%+v", app.game.Cell(0, 0).Revealed, app.ripples)
		}
		if app.ripples[0].Color.A != 0 {
			t.Fatalf("single-cell reveal (<10) must not lock member colour, got %+v", app.ripples[0].Color)
		}

		// Mid-peel at t=50ms: click (0,4) and right-click (4,4) without waiting!
		tt.Advance(50 * time.Millisecond)
		r04, _ := tt.Find("cell-0-4")
		r44, _ := tt.Find("cell-4-4")
		tt.ClickAt(r04.X+r04.W/2, r04.Y+r04.H/2)
		tt.RightClickAt(r44.X+r44.W/2, r44.Y+r44.H/2)
		if !app.game.Cell(0, 4).Revealed || !app.game.Cell(4, 4).Flagged {
			t.Fatalf("mid-animation input blocked: (0,4) revealed=%v (4,4) flagged=%v", app.game.Cell(0, 4).Revealed, app.game.Cell(4, 4).Flagged)
		}

		// At t=160ms total: (0,0) has finished its 150ms peel and shows bandColor wash.
		tt.Advance(110 * time.Millisecond)
		washAlpha := 1 - float32(160*time.Millisecond)/float32(rippleDuration)
		wantBandWash := bandColor.Alpha(0.22 * washAlpha).Over(pal.Board)
		assertColorNear(t, "single-cell <10 bandColor wash", sampleCanvasPixel(tt.Image(), r00.X+3, r00.Y+3, 2), wantBandWash)
	})

	// Over-256-budget distant cells and pre-staged revealed boards render their
	// stable revealed final state immediately on frame 0 without peel delay.
	t.Run("over-budget-and-prestaged-frame0-final", func(t *testing.T) {
		app := newMinesweeperApp(7)
		app.memberID = MemberRana
		app.game = makeCorridorGame(20, 20, true, MemberRana, [][2]int{{19, 19}})
		tt := ui.NewTester(classicCanvasTestView(app), 1280, 900)
		tt.SetScale(2)
		tt.Frame()
		board, _ := tt.Find("经典棋盘")
		app.board.center(app.game, 10, 10)
		tt.Frame()

		// Click (0,0) via app.board panned to (0,0): opens 399 cells, wave caps at 256.
		app.board.center(app.game, 0, 0)
		tt.Frame()
		originTile := classicCanvasCellRect(board, app.board, 0, 0)
		tt.ClickAt(originTile.X+originTile.W/2, originTile.Y+originTile.H/2)
		if len(app.ripples) != MaxRevealWaveEntries {
			t.Fatalf("expected bounded %d wave entries, got %d", MaxRevealWaveEntries, len(app.ripples))
		}
		// Cell (18,18) is outside the 256-entry BFS budget and already revealed.
		farIdx := 18*20 + 18
		if _, inWave := app.ripples[farIdx]; inWave {
			t.Fatal("expected (18,18) beyond 256-cell wave budget")
		}
		if !app.game.Cell(18, 18).Revealed {
			t.Fatal("expected (18,18) logically revealed")
		}
		// Center on (18,18) without advancing clock: frame 0 must show bare pal.Board (NOT pal.Slip).
		app.board.center(app.game, 18, 18)
		tt.Frame()
		pal := app.paper()
		farTile := classicCanvasCellRect(board, app.board, 18, 18)
		assertColorNear(t, "over-budget distant cell frame 0 bare paper", sampleCanvasPixel(tt.Image(), farTile.X+3, farTile.Y+3, 2), pal.Board)

		// Pre-staged revealed board (direct g.Reveal) must have zero ripples and bare paper on frame 0.
		staged := newMinesweeperApp(7)
		staged.game = makeCorridorGame(8, 8, true, MemberRana, [][2]int{{7, 7}})
		staged.game.Reveal(0, 0)
		stt := ui.NewTester(staged.view, 1280, 900)
		stt.SetScale(2)
		stt.Frame()
		if len(staged.ripples) != 0 {
			t.Fatalf("pre-staged revealed board populated %d ripples", len(staged.ripples))
		}
		r00, _ := stt.Find("cell-0-0")
		assertColorNear(t, "pre-staged revealed cell frame 0 bare paper", sampleCanvasPixel(stt.Image(), r00.X+r00.W/2, r00.Y+r00.H/2, 2), staged.paper().Board)
	})

	// When a previous flood filled all 256 wave slots, a subsequent click 50ms later
	// still evicts an older slot and peels crisply while keeping the map bounded at 256.
	t.Run("second-move-peels-when-wave-full", func(t *testing.T) {
		var mines [][2]int
		for col := range 20 {
			mines = append(mines, [2]int{18, col})
		}
		app := newMinesweeperApp(7)
		app.game = makeCorridorGame(20, 20, true, MemberTomori, mines)
		tt := ui.NewTester(classicCanvasTestView(app), 1280, 900)
		tt.SetScale(2)
		tt.Frame()
		board, _ := tt.Find("经典棋盘")

		app.board.center(app.game, 0, 0)
		tt.Frame()
		r00 := classicCanvasCellRect(board, app.board, 0, 0)
		tt.ClickAt(r00.X+r00.W/2, r00.Y+r00.H/2)
		if len(app.ripples) != MaxRevealWaveEntries {
			t.Fatalf("expected full %d-entry wave from upper flood, got %d", MaxRevealWaveEntries, len(app.ripples))
		}

		// 50ms later, reveal (19,0) below the mine barrier: must get its own zero-delay peel!
		tt.Advance(50 * time.Millisecond)
		app.board.center(app.game, 19, 0)
		tt.Frame()
		r19 := classicCanvasCellRect(board, app.board, 19, 0)
		tt.ClickAt(r19.X+r19.W/2, r19.Y+r19.H/2)
		idx19 := 19 * 20
		if len(app.ripples) > MaxRevealWaveEntries {
			t.Fatalf("wave map exceeded budget %d: got %d", MaxRevealWaveEntries, len(app.ripples))
		}
		rip19, ok := app.ripples[idx19]
		if !ok || rip19.Delay != 0 {
			t.Fatalf("expected second move (19,0) to enqueue with 0 delay, got ok=%v rip=%+v", ok, rip19)
		}
		// Mid-peel at +75ms: neither pure slip nor pure bare paper.
		tt.Advance(75 * time.Millisecond)
		pal := app.paper()
		gotMid := sampleCanvasPixel(tt.Image(), r19.X+r19.W/2, r19.Y+r19.H/2, 2)
		if near(gotMid.R, pal.Slip.R) && near(gotMid.G, pal.Slip.G) && near(gotMid.B, pal.Slip.B) {
			t.Errorf("(19,0) slip did not lift after 75ms: still %s", hexOf(gotMid))
		}
		if near(gotMid.R, pal.Board.R) && near(gotMid.G, pal.Board.G) && near(gotMid.B, pal.Board.B) {
			t.Errorf("(19,0) skipped peel and jumped straight to bare board %s", hexOf(gotMid))
		}
	})
}

// TestBigFloodLocksMemberColorWashAcrossModesAndSwitches verifies that >=10
// reveals lock the selected member's 代表色 in both Band Live and Pure Classic
// modes, survive mid-animation member/mode changes without shifting colour, and
// fall back to bandColor / duelOwnerColor for <10 reveals.
func TestBigFloodLocksMemberColorWashAcrossModesAndSwitches(t *testing.T) {
	for _, tc := range []struct {
		name     string
		bandMode bool
		member   MemberID
		canvas   bool
	}{
		{"grid_band_live_rana", true, MemberRana, false},
		{"grid_pure_classic_anon", false, MemberAnon, false},
		{"canvas_pure_classic_taki", false, MemberTaki, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := newMinesweeperApp(7)
			if tc.bandMode {
				app.modeIdx = 0
			} else {
				app.modeIdx = 1
			}
			app.memberID = tc.member
			app.game = makeCorridorGame(8, 8, tc.bandMode, tc.member, [][2]int{{7, 7}})

			viewFn := app.view
			if tc.canvas {
				viewFn = classicCanvasTestView(app)
			}
			tt := ui.NewTester(viewFn, 1280, 900)
			tt.Frame()

			var origin ui.Rect
			if tc.canvas {
				br, _ := tt.Find("经典棋盘")
				origin = classicCanvasCellRect(br, app.board, 0, 0)
			} else {
				origin, _ = tt.Find("cell-0-0")
			}
			tt.ClickAt(origin.X+origin.W/2, origin.Y+origin.H/2)

			locked := memberImage(tc.member)
			if rip := app.ripples[0]; rip.Color != locked {
				t.Fatalf("expected locked colour %s, got %s", hexOf(locked), hexOf(rip.Color))
			}

			// Mutate memberID and modeIdx mid-wave: the in-flight wash must remain locked!
			app.memberID = MemberSoyorin
			app.modeIdx = 1 - app.modeIdx
			tt.Advance(peelDuration + 10*time.Millisecond)

			pal := app.paper()
			washAlpha := 1 - float32(peelDuration+10*time.Millisecond)/float32(rippleDuration)
			wantLocked := locked.Alpha(0.26 * washAlpha).Over(pal.Board)
			assertColorNear(t, "mid-wave locked member wash unchanged after member switch",
				sampleCanvasPixel(tt.Image(), origin.X+origin.W/2, origin.Y+origin.H/2, 1), wantLocked)
		})
	}

	// Duel <10 keeps duelOwnerColor(DuelPlayer), while >=10 locks memberImage(a.memberID).
	t.Run("duel_small_vs_big_flood", func(t *testing.T) {
		app := newMinesweeperApp(7)
		app.diffIdx = findDifficultyIdx("duel")
		app.memberID = MemberRana
		app.game = NewGame(Difficulty{Gameplay: GameplayDuel, Rows: 8, Cols: 8, Mines: 2, DuelDuration: 60, AIInterval: 100}, true, MemberRana, 7)
		app.game.FirstClick = true
		app.game.Status = StatusPlaying
		app.game.Cell(0, 1).IsMine = true
		app.game.Cell(7, 7).IsMine = true
		app.game.RecomputeAdjacency()

		tt := ui.NewTester(app.view, 1280, 900)
		app.board.center(app.game, 3, 3)
		tt.Frame()
		br, _ := tt.Find("决斗棋盘")

		// Click (0,0): Adjacent=1, Reveals=1 (<10) -> Color must be zero (falls back to duelOwnerColor).
		r00 := classicCanvasCellRect(br, app.board, 0, 0)
		tt.ClickAt(r00.X+r00.W/2, r00.Y+r00.H/2)
		if rip, ok := app.ripples[0]; !ok || rip.Color.A != 0 {
			t.Fatalf("duel <10 reveal should not lock member colour: ok=%v rip=%+v", ok, rip)
		}
		// Click (4,4): floods >=10 cells -> locks MemberRana 代表色.
		r44 := classicCanvasCellRect(br, app.board, 4, 4)
		tt.ClickAt(r44.X+r44.W/2, r44.Y+r44.H/2)
		if rip, ok := app.ripples[4*8+4]; !ok || rip.Color != memberImage(MemberRana) {
			t.Fatalf("duel >=10 reveal should lock Rana 代表色: ok=%v rip=%+v", ok, rip)
		}
	})
}

func cloneScaledRect(img *image.RGBA, r ui.Rect, scale float32) *image.RGBA {
	x0 := max(img.Bounds().Min.X, int(r.X*scale))
	y0 := max(img.Bounds().Min.Y, int(r.Y*scale))
	x1 := min(img.Bounds().Max.X, int((r.X+r.W)*scale))
	y1 := min(img.Bounds().Max.Y, int((r.Y+r.H)*scale))
	w, h := max(0, x1-x0), max(0, y1-y0)
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		srcOff := img.PixOffset(x0, y0+y)
		dstOff := dst.PixOffset(0, y)
		copy(dst.Pix[dstOff:dstOff+w*4], img.Pix[srcOff:srcOff+w*4])
	}
	return dst
}

func diffRectPixels(a, b *image.RGBA, target ui.Color, tol int) (totalDiff, matchingDiff int) {
	tr, tg, tb := int(target.R), int(target.G), int(target.B)
	for i := 0; i+3 < len(a.Pix) && i+3 < len(b.Pix); i += 4 {
		if a.Pix[i] == b.Pix[i] && a.Pix[i+1] == b.Pix[i+1] && a.Pix[i+2] == b.Pix[i+2] && a.Pix[i+3] == b.Pix[i+3] {
			continue
		}
		totalDiff++
		dr := int(a.Pix[i]) - tr
		dg := int(a.Pix[i+1]) - tg
		db := int(a.Pix[i+2]) - tb
		if dr < 0 {
			dr = -dr
		}
		if dg < 0 {
			dg = -dg
		}
		if db < 0 {
			db = -db
		}
		if dr <= tol && dg <= tol && db <= tol {
			matchingDiff++
		}
	}
	return totalDiff, matchingDiff
}

// TestBandageBounceAndPeelOffBothBackends verifies the ~140ms 1.25x slap bounce
// (manual flag, Soyorin shield + shake, duel claim & AI flag), ~110ms diagonal
// peel-off (plain unflag and right-click -> ?), zoomed-out <16px skip, and
// frame-0 steady rendering for pre-staged flagged boards.
func TestBandageBounceAndPeelOffBothBackends(t *testing.T) {
	// Verify spring curve starts at 1.25x with slap angle, undershoots slightly, and settles cleanly at 140ms.
	s0, r0 := bandageBounceTransform(0)
	if s0 < 1.24 || s0 > 1.26 || r0 < 9.5 {
		t.Fatalf("t=0 bounce transform = (%.3f, %.2f°), want (~1.25, ~10°)", s0, r0)
	}
	sMid, _ := bandageBounceTransform(70 * time.Millisecond)
	if sMid >= 1.0 {
		t.Fatalf("expected tactile press-in undershoot around 70ms, got scale=%.3f", sMid)
	}
	sEnd, rEnd := bandageBounceTransform(bandageBounceDuration)
	if sEnd != 1 || rEnd != 0 {
		t.Fatalf("expected exact settle (1, 0) at %v, got (%.3f, %.2f°)", bandageBounceDuration, sEnd, rEnd)
	}

	for _, backend := range []string{"grid", "canvas"} {
		t.Run(backend, func(t *testing.T) {
			app := newMinesweeperApp(7)
			app.memberID = MemberSoyorin
			app.allowQuestion = false
			app.game = makeCorridorGame(6, 6, true, MemberSoyorin, [][2]int{{0, 0}, {5, 5}})

			viewFn := app.view
			if backend == "canvas" {
				viewFn = classicCanvasTestView(app)
			}
			tt := ui.NewTester(viewFn, 1280, 900)
			tt.SetScale(2)
			tt.Frame()

			cellRect := func(row, col int) ui.Rect {
				if backend == "grid" {
					r, _ := tt.Find(fmt.Sprintf("cell-%d-%d", row, col))
					return r
				}
				br, _ := tt.Find("经典棋盘")
				return classicCanvasCellRect(br, app.board, row, col)
			}

			pal := app.paper()
			// 1. Manual right-click flag on (1,1): starts ~1.25x bounce without shifting hit-box.
			r11Before := cellRect(1, 1)
			peelProbe := ui.Rect{X: r11Before.X, Y: r11Before.Y - 8, W: r11Before.W + 10, H: r11Before.H + 8}
			tt.RightClickAt(r11Before.X+r11Before.W/2, r11Before.Y+r11Before.H/2)
			idx11 := 1*6 + 1
			if b, ok := app.bandages[idx11]; !ok || b.Kind != bandageBounce || b.Shield {
				t.Fatalf("expected normal bandageBounce on (1,1), got ok=%v b=%+v", ok, b)
			}
			if got := cellRect(1, 1); got != r11Before {
				t.Fatalf("hit-box shifted during bandage bounce: before=%+v after=%+v", r11Before, got)
			}
			pxBounce := countMatchingPixels(tt.Image(), r11Before, 2, pal.BandAid, 24)
			tt.Advance(bandageBounceDuration + 20*time.Millisecond)
			if len(app.bandages) != 0 {
				t.Fatalf("expected bandage bounce converged after %v, remaining=%d", bandageBounceDuration, len(app.bandages))
			}
			pxSteady := countMatchingPixels(tt.Image(), r11Before, 2, pal.BandAid, 24)
			if pxBounce <= pxSteady {
				t.Errorf("expected 1.25x bouncing band-aid to paint more pixels than steady 1.0x: bounce=%d steady=%d", pxBounce, pxSteady)
			}

			// 2a. Plain unflag (allowQuestion=false -> blank unrevealed slip, no "?" text glyph):
			// plays 110ms diagonal peel-off and leaves zero band-aid pixels after settle.
			tt.RightClickAt(r11Before.X+r11Before.W/2, r11Before.Y+r11Before.H/2)
			if app.game.Cell(1, 1).Flagged || app.game.Cell(1, 1).Questioned {
				t.Fatal("expected (1,1) plain unflagged on frame 0")
			}
			if b, ok := app.bandages[idx11]; !ok || b.Kind != bandagePeel {
				t.Fatalf("expected bandagePeel on Flag->blank, got ok=%v b=%+v", ok, b)
			}
			tt.Advance(35 * time.Millisecond)
			pxPeeling := countMatchingPixels(tt.Image(), peelProbe, 2, pal.BandAid, 48)
			tt.Advance(bandagePeelDuration)
			if len(app.bandages) != 0 {
				t.Fatalf("expected plain unflag bandage peel converged, remaining=%d", len(app.bandages))
			}
			pxAfterPeel := countMatchingPixels(tt.Image(), peelProbe, 2, pal.BandAid, 24)
			if pxPeeling == 0 || pxAfterPeel != 0 {
				t.Errorf("expected plain unflag peeling band-aid visible mid-peel and gone after settle: mid=%d after=%d", pxPeeling, pxAfterPeel)
			}

			// 2b. Flag -> ? (allowQuestion=true): compare against an independent pre-staged
			// static "?" slip hovered at the same point so platform LCD subpixel AA (e.g.
			// Windows ClearType around "?") cannot trigger band-aid color false positives.
			refApp := newMinesweeperApp(7)
			refApp.memberID = MemberSoyorin
			refApp.allowQuestion = true
			refApp.game = makeCorridorGame(6, 6, true, MemberSoyorin, [][2]int{{0, 0}, {5, 5}})
			refApp.game.Cell(1, 1).Questioned = true
			refViewFn := refApp.view
			if backend == "canvas" {
				refViewFn = classicCanvasTestView(refApp)
			}
			refTT := ui.NewTester(refViewFn, 1280, 900)
			refTT.SetScale(2)
			refTT.Frame()
			refTT.Move(r11Before.X+r11Before.W/2, r11Before.Y+r11Before.H/2)
			refTT.Frame()
			steadyQImg := cloneScaledRect(refTT.Image(), peelProbe, 2)

			app.allowQuestion = true
			tt.RightClickAt(r11Before.X+r11Before.W/2, r11Before.Y+r11Before.H/2)
			tt.Advance(bandageBounceDuration + 20*time.Millisecond)
			tt.RightClickAt(r11Before.X+r11Before.W/2, r11Before.Y+r11Before.H/2)
			if app.game.Cell(1, 1).Flagged || !app.game.Cell(1, 1).Questioned {
				t.Fatal("expected (1,1) transitioned to ? on frame 0")
			}
			if b, ok := app.bandages[idx11]; !ok || b.Kind != bandagePeel {
				t.Fatalf("expected bandagePeel on Flag->?, got ok=%v b=%+v", ok, b)
			}
			tt.Advance(35 * time.Millisecond)
			midQImg := cloneScaledRect(tt.Image(), peelProbe, 2)
			tt.Advance(bandagePeelDuration)
			if len(app.bandages) != 0 {
				t.Fatalf("expected Flag->? bandage peel converged, remaining=%d", len(app.bandages))
			}
			afterQImg := cloneScaledRect(tt.Image(), peelProbe, 2)

			_, midBandAidDiff := diffRectPixels(midQImg, steadyQImg, pal.BandAid, 48)
			afterTotalDiff, _ := diffRectPixels(afterQImg, steadyQImg, pal.BandAid, 48)
			if midBandAidDiff < 20 || afterTotalDiff != 0 {
				t.Errorf("expected Flag->? peeling band-aid visible mid-peel and pixel-identical to static ? after settle: midBandAidDiff=%d afterTotalDiff=%d", midBandAidDiff, afterTotalDiff)
			}
			// ? glyph remains cleanly rendered underneath after peel completes.
			if qPx := countMatchingPixels(tt.Image(), r11Before, 2, pal.Graphite, 28); qPx < 10 {
				t.Errorf("expected ? glyph visible after bandage peel-off, graphite px=%d", qPx)
			}

			// 3. Soyorin shield on mine (0,0): golden shield bandage bounce + 4.5/220ms shake simultaneously.
			r00 := cellRect(0, 0)
			tt.ClickAt(r00.X+r00.W/2, r00.Y+r00.H/2)
			if !app.game.Cell(0, 0).ShieldSaved {
				t.Fatal("expected Soyorin shield triggered on (0,0)")
			}
			if b, ok := app.bandages[0]; !ok || b.Kind != bandageBounce || !b.Shield {
				t.Fatalf("expected shield bandageBounce on (0,0), got ok=%v b=%+v", ok, b)
			}
			if app.shakeAmplitude != 4.5 || app.shakeDuration != 220*time.Millisecond {
				t.Fatalf("expected 4.5/220ms shield shake alongside bounce, got amp=%v dur=%v", app.shakeAmplitude, app.shakeDuration)
			}
			tt.Advance(240 * time.Millisecond)
			if len(app.bandages) != 0 || app.shakeAmplitude != 0 {
				t.Fatalf("expected shield bounce & shake settled: bandages=%d amp=%v", len(app.bandages), app.shakeAmplitude)
			}
		})
	}

	// Duel claim mines & AI flag both trigger bandageBounce; zoomed-out <16px skips transforms cleanly.
	t.Run("duel-claim-ai-flag-and-zoomed-out-skip", func(t *testing.T) {
		app := newMinesweeperApp(7)
		app.diffIdx = findDifficultyIdx("duel")
		app.game = NewGame(Difficulty{Gameplay: GameplayDuel, Rows: 6, Cols: 6, Mines: 2, DuelDuration: 60, AIInterval: 0.5}, true, MemberAnon, 7)
		g := app.game
		g.FirstClick = true
		g.Status = StatusPlaying
		g.Cell(0, 0).IsMine = true
		g.Cell(5, 5).IsMine = true
		g.RecomputeAdjacency()
		for i := range g.Cells {
			c := &g.Cells[i]
			if c.IsMine {
				continue
			}
			c.Revealed = true
			c.Owner = DuelPlayer
			g.RevealedSafe++
		}
		g.duel.NextAI = 1e9

		tt := ui.NewTester(app.view, 1280, 900)
		app.board.center(g, 1, 1)
		tt.Frame()
		br, _ := tt.Find("决斗棋盘")

		// Left-click (1,1) claims mine (0,0) -> bandageBounce on (0,0)!
		r11 := classicCanvasCellRect(br, app.board, 1, 1)
		tt.ClickAt(r11.X+r11.W/2, r11.Y+r11.H/2)
		if !g.Cell(0, 0).Flagged {
			t.Fatal("expected (0,0) claimed by duel click")
		}
		if b, ok := app.bandages[0]; !ok || b.Kind != bandageBounce {
			t.Fatalf("expected bandageBounce on duel-claimed mine (0,0), got ok=%v b=%+v", ok, b)
		}

		// Zoom out below 16px detail threshold while bounce is active: renders cleanly without error.
		app.board.Cell = 11
		app.board.clamp(g)
		tt.Frame()
		tt.Advance(bandageBounceDuration + 20*time.Millisecond)
		if len(app.bandages) != 0 {
			t.Fatalf("expected duel bounce expired, remaining=%d", len(app.bandages))
		}

		// Let AI flag (5,5) on tick -> triggers bandageBounce on AI flag cell too.
		g.duel.frontier[5*6+5] = true
		g.duel.NextAI = g.duel.Elapsed + 0.1
		tt.Advance(200 * time.Millisecond)
		if !g.Cell(5, 5).Flagged || g.Cell(5, 5).FlagOwner != DuelAI {
			t.Fatalf("expected AI to flag (5,5), cell=%+v", g.Cell(5, 5))
		}
		if b, ok := app.bandages[5*6+5]; !ok || b.Kind != bandageBounce {
			t.Fatalf("expected bandageBounce on AI flag (5,5), got ok=%v b=%+v", ok, b)
		}
	})

	// Unflagging a Soyorin-shielded mine in duel leaves cell.Revealed == true while
	// still rendering the golden shield band-aid's 110ms diagonal peel-off.
	t.Run("duel-shield-unflag-peels-golden-bandage", func(t *testing.T) {
		app := newMinesweeperApp(7)
		app.diffIdx = findDifficultyIdx("duel")
		app.memberID = MemberSoyorin
		app.game = NewGame(Difficulty{Gameplay: GameplayDuel, Rows: 6, Cols: 6, Mines: 2, SkillCharges: 1, DuelDuration: 60, AIInterval: 100}, true, MemberSoyorin, 7)
		g := app.game
		g.FirstClick = true
		g.Status = StatusPlaying
		g.Cell(0, 0).IsMine = true
		g.Cell(5, 5).IsMine = true
		g.RecomputeAdjacency()
		g.duel.NextAI = 1e9

		tt := ui.NewTester(app.view, 1280, 900)
		tt.SetScale(2)
		app.board.center(g, 0, 0)
		tt.Frame()
		br, _ := tt.Find("决斗棋盘")
		r00 := classicCanvasCellRect(br, app.board, 0, 0)

		// Trigger Soyorin shield on (0,0), let bounce & shake settle, then right-click to unflag.
		tt.ClickAt(r00.X+r00.W/2, r00.Y+r00.H/2)
		if !g.Cell(0, 0).ShieldSaved || !g.Cell(0, 0).Revealed || !g.Cell(0, 0).Flagged {
			t.Fatalf("expected shielded revealed+flagged duel cell, got %+v", g.Cell(0, 0))
		}
		tt.Advance(260 * time.Millisecond)

		tt.RightClickAt(r00.X+r00.W/2, r00.Y+r00.H/2)
		if g.Cell(0, 0).Flagged || !g.Cell(0, 0).Revealed {
			t.Fatalf("expected duel shield unflagged while keeping Revealed=true, got %+v", g.Cell(0, 0))
		}
		if b, ok := app.bandages[0]; !ok || b.Kind != bandagePeel || !b.Shield {
			t.Fatalf("expected golden shield bandagePeel on (0,0), got ok=%v b=%+v", ok, b)
		}
		tt.Advance(35 * time.Millisecond)
		peelProbe := ui.Rect{X: r00.X, Y: r00.Y - 8, W: r00.W + 10, H: r00.H + 8}
		soyoGold := memberImage(MemberSoyorin)
		pxMid := countMatchingPixels(tt.Image(), peelProbe, 2, soyoGold, 56)
		tt.Advance(bandagePeelDuration)
		if len(app.bandages) != 0 {
			t.Fatalf("expected duel shield peel settled, remaining=%d", len(app.bandages))
		}
		pxAfter := countMatchingPixels(tt.Image(), peelProbe, 2, soyoGold, 28)
		if pxMid == 0 || pxAfter != 0 {
			t.Errorf("expected golden shield peel visible mid-flight and gone after settle: mid=%d after=%d", pxMid, pxAfter)
		}
	})
}

// TestClassicGridShakePreservesSlipShadowsAndHitBoxes verifies that classic
// widget grid shakes keep pal.SlipShade shadows beneath shifted paper slips
// (unrevealed, flagged, shielded, questioned, and wave-covered) without moving
// cell hit-test bounds or HUD bounds.
func TestClassicGridShakePreservesSlipShadowsAndHitBoxes(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(fmt.Sprintf("dark=%v", dark), func(t *testing.T) {
			app := newMinesweeperApp(7)
			app.darkMode = dark
			app.memberID = MemberSoyorin
			app.allowQuestion = true
			app.game = makeCorridorGame(6, 6, true, MemberSoyorin, [][2]int{{0, 0}, {5, 5}})
			// Reveal row 2..3 so slip (1,2) sits directly above bare pal.Board (2,2),
			// making both pixel shift and the slip's drop shadow cleanly measurable.
			for col := range 6 {
				app.game.Cell(2, col).Revealed = true
				app.game.Cell(3, col).Revealed = true
				app.game.RevealedSafe += 2
			}
			app.game.ToggleFlag(1, 1, false)
			app.game.ToggleFlag(1, 3, true)
			app.game.ToggleFlag(1, 3, true)

			tt := ui.NewTester(app.view, 1280, 900)
			tt.SetScale(2)
			tt.Frame()

			pal := paperFor(dark)
			rSlipBefore, ok := tt.Find("cell-1-2")
			if !ok {
				t.Fatal("cell-1-2 missing")
			}
			rMine0, _ := tt.Find("cell-0-0")
			hudBefore, _ := tt.Find("演奏中 · 重开 (R)")

			// Measure baseline slip bottom edge and verify shadow darkening right below it.
			findSlipBottomY := func(img *image.RGBA) int {
				px := int((rSlipBefore.X + rSlipBefore.W/2) * 2)
				yMin := int((rSlipBefore.Y - 12) * 2)
				yMax := int((rSlipBefore.Y + rSlipBefore.H + 16) * 2)
				last := -1
				for py := yMin; py <= yMax; py++ {
					c := img.RGBAAt(px, py)
					if near(c.R, pal.Slip.R) && near(c.G, pal.Slip.G) && near(c.B, pal.Slip.B) {
						last = py
					}
				}
				return last
			}
			hasShadowBelow := func(img *image.RGBA, bottomY int) bool {
				px := int((rSlipBefore.X + rSlipBefore.W/2) * 2)
				boardSum := int(pal.Board.R) + int(pal.Board.G) + int(pal.Board.B)
				// Inspect 1..5 physical pixels below the slip's bottom edge for pal.SlipShade
				// darkening below pal.Board (works in both day and low-luminance night themes).
				for dy := 1; dy <= 5; dy++ {
					c := img.RGBAAt(px, bottomY+dy)
					if int(c.R)+int(c.G)+int(c.B) <= boardSum-12 {
						return true
					}
				}
				return false
			}

			baseBottom := findSlipBottomY(tt.Image())
			if baseBottom < 0 {
				t.Fatal("could not locate baseline slip bottom edge on classic grid")
			}
			if !hasShadowBelow(tt.Image(), baseBottom) {
				t.Fatalf("baseline slip missing shadow below y=%d", baseBottom)
			}

			// Trigger Soyorin shield (4.5/220ms) and advance 22ms into active displacement.
			tt.ClickAt(rMine0.X+rMine0.W/2, rMine0.Y+rMine0.H/2)
			tt.Advance(22 * time.Millisecond)
			sx, sy := app.currentShakeOffset(tt.Now())
			if sx == 0 && sy == 0 {
				t.Fatal("expected non-zero classic grid shake at 22ms")
			}
			if rDuring, _ := tt.Find("cell-1-2"); rDuring != rSlipBefore {
				t.Fatalf("classic grid hit-box moved during shake: before=%+v during=%+v", rSlipBefore, rDuring)
			}
			if hudDuring, _ := tt.Find("演奏中 · 重开 (R)"); hudDuring != hudBefore {
				t.Fatalf("HUD moved during shake: before=%+v during=%+v", hudBefore, hudDuring)
			}

			shakeImg := tt.Image()
			shakeBottom := findSlipBottomY(shakeImg)
			if shakeBottom < 0 {
				t.Fatal("could not locate shifted slip bottom edge during shake")
			}
			if !hasShadowBelow(shakeImg, shakeBottom) {
				t.Errorf("slip shadow vanished below shifted bottom edge y=%d (base=%d, shake=(%.2f,%.2f))", shakeBottom, baseBottom, sx, sy)
			}

			// Also verify stronger mine-loss shake (9.0/320ms) retains shadows and shifts visually.
			rMine5, _ := tt.Find("cell-5-5")
			tt.ClickAt(rMine5.X+rMine5.W/2, rMine5.Y+rMine5.H/2)
			lossImg0 := tt.Image()
			lossBottom := findSlipBottomY(lossImg0)
			if lossBottom <= baseBottom+4 {
				t.Errorf("expected 9.0 loss shake to shift slip edge down: base=%d loss=%d", baseBottom, lossBottom)
			}
			if !hasShadowBelow(lossImg0, lossBottom) {
				t.Errorf("slip shadow vanished during 9.0 loss shake below y=%d", lossBottom)
			}
		})
	}
}

// TestMotionReducedMidFlightAndThreeResetPathsClearAllEffects verifies:
//  1. Effective reduced motion skips peel/bandage/shake on frame 0;
//  2. Switching to reduced motion mid-flight (via settings popover or system
//     preference) immediately settles all in-flight effects on the same frame;
//  3. System ReduceMotion=true + manual motionStandard runs full motion;
//  4. KeyR, setlist track switch, and top-bar gameplay mode switch all clear
//     ripples, bandages, and shake immediately.
func TestMotionReducedMidFlightAndThreeResetPathsClearAllEffects(t *testing.T) {
	// 1. Mid-flight switch to reduced via 设置 popover & system preference settles immediately.
	t.Run("mid-flight-reduced-same-frame-settle", func(t *testing.T) {
		app := newMinesweeperApp(7)
		app.memberID = MemberSoyorin
		// Mines at (0,7), (6,6), (6,7), (7,6) leave (7,7) safe and unrevealed after
		// flooding from (0,0), keeping the game in StatusPlaying for flag + shield actions.
		app.game = makeCorridorGame(8, 8, true, MemberSoyorin, [][2]int{{0, 7}, {6, 6}, {6, 7}, {7, 6}})
		tt := ui.NewTester(app.view, 1280, 900)
		tt.SetScale(2)
		tt.Frame()
		pal := app.paper()

		// Trigger flood wave + flag bounce + shield shake simultaneously.
		r00, _ := tt.Find("cell-0-0")
		r07, _ := tt.Find("cell-0-7")
		r77, _ := tt.Find("cell-7-7")
		tt.ClickAt(r00.X+r00.W/2, r00.Y+r00.H/2)
		tt.RightClickAt(r77.X+r77.W/2, r77.Y+r77.H/2)
		tt.ClickAt(r07.X+r07.W/2, r07.Y+r07.H/2)
		if len(app.ripples) == 0 || len(app.bandages) == 0 || app.shakeAmplitude == 0 {
			t.Fatalf("expected active wave, bandages, and shake before switching: ripples=%d bandages=%d amp=%v",
				len(app.ripples), len(app.bandages), app.shakeAmplitude)
		}
		// Cell (6,0) is still covered by its waiting slip at t=0 (away from the top-center
		// Soyorin shield quote strip, and clear of the digit once revealed).
		r60, _ := tt.Find("cell-6-0")
		assertColorNear(t, "covered before reduced toggle", sampleCanvasPixel(tt.Image(), r60.X+10, r60.Y+12, 2), pal.Slip)

		// System toggles ReduceMotion=true under motionSystem -> immediate same-frame settle!
		tt.SetPreferences(ui.Preferences{ReduceMotion: true})
		if !app.motionReducedActive || len(app.ripples) != 0 || len(app.bandages) != 0 || app.shakeAmplitude != 0 {
			t.Fatalf("expected same-frame clear on ReduceMotion=true: reduced=%v ripples=%d bandages=%d amp=%v",
				app.motionReducedActive, len(app.ripples), len(app.bandages), app.shakeAmplitude)
		}
		assertColorNear(t, "exposed final bare paper on same frame", sampleCanvasPixel(tt.Image(), r60.X+10, r60.Y+12, 2), pal.Board)

		// Open 设置 popover and switch to 标准 while system ReduceMotion remains true -> full motion resumes.
		if err := tt.Click("设置"); err != nil {
			t.Fatalf("open 设置: %v", err)
		}
		if err := tt.Click("标准"); err != nil {
			t.Fatalf("select 标准: %v", err)
		}
		tt.Key(0, ui.KeyEscape)
		if app.motionReducedActive {
			t.Fatal("motionStandard must override system ReduceMotion=true")
		}
		tt.RightClickAt(r77.X+r77.W/2, r77.Y+r77.H/2)
		if len(app.bandages) == 0 {
			t.Fatal("expected bandage peel-off under motionStandard with system ReduceMotion=true")
		}

		// Switch to 减弱 via 设置 popover mid-animation -> same-frame clear!
		if err := tt.Click("设置"); err != nil {
			t.Fatalf("reopen 设置: %v", err)
		}
		if err := tt.Click("减弱"); err != nil {
			t.Fatalf("select 减弱: %v", err)
		}
		if !app.motionReducedActive || len(app.ripples) != 0 || len(app.bandages) != 0 || app.shakeAmplitude != 0 {
			t.Fatalf("expected same-frame clear via 设置 减弱: reduced=%v ripples=%d bandages=%d amp=%v",
				app.motionReducedActive, len(app.ripples), len(app.bandages), app.shakeAmplitude)
		}
	})

	// 2. Three reset paths (KeyR, 歌单 track switch, top-bar 乐队 Live/纯正经典 switch) clear everything immediately.
	for _, path := range []string{"keyR", "setlist", "modeSwitch"} {
		t.Run("reset_"+path, func(t *testing.T) {
			app := newMinesweeperApp(7)
			app.memberID = MemberSoyorin
			app.game = makeCorridorGame(8, 8, true, MemberSoyorin, [][2]int{{0, 7}, {6, 6}, {6, 7}, {7, 6}})
			tt := ui.NewTester(app.view, 1280, 900)
			tt.Frame()

			r00, _ := tt.Find("cell-0-0")
			r07, _ := tt.Find("cell-0-7")
			tt.ClickAt(r00.X+r00.W/2, r00.Y+r00.H/2)
			tt.ClickAt(r07.X+r07.W/2, r07.Y+r07.H/2)
			if len(app.ripples) == 0 || len(app.bandages) == 0 || app.shakeAmplitude == 0 {
				t.Fatalf("expected active effects before %s: ripples=%d bandages=%d amp=%v",
					path, len(app.ripples), len(app.bandages), app.shakeAmplitude)
			}

			switch path {
			case "keyR":
				tt.Key(0, ui.KeyR)
			case "setlist":
				if err := tt.Click("歌单"); err != nil {
					t.Fatalf("open setlist: %v", err)
				}
				if err := tt.Click("碧天伴走"); err != nil {
					t.Fatalf("pick 碧天伴走: %v", err)
				}
			case "modeSwitch":
				if err := tt.Click("纯正经典"); err != nil {
					t.Fatalf("switch to 纯正经典: %v", err)
				}
			}

			if len(app.ripples) != 0 || len(app.bandages) != 0 || app.shakeAmplitude != 0 {
				t.Fatalf("%s did not clear all in-flight effects: ripples=%d bandages=%d amp=%v",
					path, len(app.ripples), len(app.bandages), app.shakeAmplitude)
			}
		})
	}
}

// TestMotionKeyframeSnapshots exports deterministic mid-motion keyframes across
// small/default/large viewports and day/night themes for human visual review:
//
//	MYGO_KEYFRAME_SNAPSHOT_DIR=/tmp/kf go test -run '^TestMotionKeyframeSnapshots$' .
func TestMotionKeyframeSnapshots(t *testing.T) {
	dir := os.Getenv("MYGO_KEYFRAME_SNAPSHOT_DIR")
	if dir == "" {
		t.Skip("set MYGO_KEYFRAME_SNAPSHOT_DIR to render motion keyframe snapshots")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	type keyframeSpec struct {
		name   string
		render func(t *testing.T, dark bool, width, height int) *image.RGBA
	}

	specs := []keyframeSpec{
		{
			name: "wave-peel-mid-grid",
			render: func(t *testing.T, dark bool, width, height int) *image.RGBA {
				app := newMinesweeperApp(20261116)
				app.darkMode = dark
				app.memberID = MemberAnon
				app.diffIdx = 1
				app.resetGame(true)
				pinStartQuote(app)
				// Deterministic 16x16 corridor with mines along the right/bottom edge so
				// clicking (2,2) sweeps a clean radial wave across the visible board.
				var mines [][2]int
				for r := range 16 {
					mines = append(mines, [2]int{r, 15})
				}
				for c := 0; c < 15; c++ {
					mines = append(mines, [2]int{15, c})
				}
				app.game = makeCorridorGame(16, 16, true, MemberAnon, mines)

				tt := ui.NewTester(app.view, width, height)
				tt.SetScale(2)
				tt.Frame()
				r22, ok := tt.Find("cell-2-2")
				if !ok {
					t.Fatal("cell-2-2 missing")
				}
				tt.ClickAt(r22.X+r22.W/2, r22.Y+r22.H/2)
				// Freeze at t=175ms: depth 0..1 fully peeled with locked Anon pink wash,
				// depth 2..7 actively lifting/rotating/fading, depth 8+ still covered by slips.
				tt.Advance(175 * time.Millisecond)
				tt.Frame()
				return tt.Image()
			},
		},
		{
			name: "wave-peel-mid-canvas",
			render: func(t *testing.T, dark bool, width, height int) *image.RGBA {
				app := newMinesweeperApp(20261116)
				app.darkMode = dark
				app.memberID = MemberAnon
				app.game = NewGame(TrackUtakotoba, true, MemberAnon, 20261116)
				pinStartQuote(app)
				g := app.game
				g.FirstClick = true
				g.Status = StatusPlaying
				// Place a crisp perimeter of mines around the center viewport so clicking
				// (25,25) creates a clear >=10 radial wave across the canvas.
				for r := 16; r <= 34; r++ {
					g.Cell(r, 16).IsMine = true
					g.Cell(r, 34).IsMine = true
				}
				for c := 17; c <= 33; c++ {
					g.Cell(16, c).IsMine = true
					g.Cell(34, c).IsMine = true
				}
				g.RecomputeAdjacency()

				tt := ui.NewTester(classicCanvasTestView(app), width, height)
				tt.SetScale(2)
				tt.Frame()
				app.board.center(g, 25, 25)
				tt.Frame()
				br, ok := tt.Find("经典棋盘")
				if !ok {
					t.Fatal("classic canvas missing")
				}
				c25 := classicCanvasCellRect(br, app.board, 25, 25)
				tt.ClickAt(c25.X+c25.W/2, c25.Y+c25.H/2)
				tt.Advance(175 * time.Millisecond)
				tt.Frame()
				return tt.Image()
			},
		},
		{
			name: "bandage-bounce-mid",
			render: func(t *testing.T, dark bool, width, height int) *image.RGBA {
				app := newMinesweeperApp(7)
				app.darkMode = dark
				app.memberID = MemberSoyorin
				app.diffIdx = 0
				app.resetGame(true)
				pinStartQuote(app)
				app.game = makeCorridorGame(9, 9, true, MemberSoyorin, [][2]int{{3, 5}, {0, 0}, {8, 8}})
				// Open a central patch so steady flags sit beside live bouncing flags.
				for r := 1; r <= 2; r++ {
					for c := 1; c <= 7; c++ {
						app.game.Cell(r, c).Revealed = true
						app.game.RevealedSafe++
					}
				}
				// Steady baseline flags on (3,2) and (5,2) for direct scale comparison,
				// plus a flag on (5,5) that we will peel off diagonally.
				app.game.ToggleFlag(3, 2, false)
				app.game.ToggleFlag(5, 2, false)
				app.game.ToggleFlag(5, 5, false)

				tt := ui.NewTester(app.view, width, height)
				tt.SetScale(2)
				tt.Frame()

				// Start a diagonal peel-off on (5,5) slightly earlier so it is mid-flight,
				// then slap a normal flag on (3,3) and trigger Soyorin's golden shield on (3,5).
				r55, _ := tt.Find("cell-5-5")
				tt.RightClickAt(r55.X+r55.W/2, r55.Y+r55.H/2)
				tt.Advance(32 * time.Millisecond)
				r33, _ := tt.Find("cell-3-3")
				r35, _ := tt.Find("cell-3-5")
				tt.RightClickAt(r33.X+r33.W/2, r33.Y+r33.H/2)
				tt.ClickAt(r35.X+r35.W/2, r35.Y+r35.H/2)
				// Freeze at t=+6ms: both fresh band-aids sit near ~1.22x with slap tilt
				// right next to the 1.0x steady band-aid on (3,2), while (5,5) peels off.
				tt.Advance(6 * time.Millisecond)
				tt.Frame()
				return tt.Image()
			},
		},
		{
			name: "classic-shake-shadow-mid",
			render: func(t *testing.T, dark bool, width, height int) *image.RGBA {
				app := newMinesweeperApp(7)
				app.darkMode = dark
				app.memberID = MemberSoyorin
				app.allowQuestion = true
				app.diffIdx = 0
				app.resetGame(true)
				pinStartQuote(app)
				app.game = makeCorridorGame(9, 9, true, MemberSoyorin, [][2]int{{0, 0}, {8, 8}})
				for r := 4; r <= 6; r++ {
					for c := 1; c <= 7; c++ {
						app.game.Cell(r, c).Revealed = true
						app.game.RevealedSafe++
					}
				}
				app.game.ToggleFlag(3, 2, false)
				app.game.ToggleFlag(3, 4, true)
				app.game.ToggleFlag(3, 4, true)

				tt := ui.NewTester(app.view, width, height)
				tt.SetScale(2)
				tt.Frame()

				// Trigger Soyorin shield on (0,0) (light shake) then hit (8,8) (9.0 shake)
				// so the board shows shielded flag, normal flag, ? slip, plain slips, and
				// clear slip shadows translated mid-shake at t=+18ms.
				r00, _ := tt.Find("cell-0-0")
				r88, _ := tt.Find("cell-8-8")
				tt.ClickAt(r00.X+r00.W/2, r00.Y+r00.H/2)
				tt.Advance(250 * time.Millisecond)
				tt.ClickAt(r88.X+r88.W/2, r88.Y+r88.H/2)
				tt.Advance(18 * time.Millisecond)
				tt.Frame()
				return tt.Image()
			},
		},
	}

	for _, spec := range specs {
		for _, dark := range []bool{false, true} {
			suffix := "day"
			if dark {
				suffix = "night"
			}
			for _, vp := range snapshotViewports {
				img := spec.render(t, dark, vp.width, vp.height)
				name := fmt.Sprintf("%s-%s-%s.png", spec.name, vp.name, suffix)
				saveTesterPNG(t, img, filepath.Join(dir, name))
			}
		}
	}
}
