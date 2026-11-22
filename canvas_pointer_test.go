package main

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func setupDeterministicCanvasBoard(t *testing.T, isDuel bool) (*minesweeperApp, *ui.Tester, ui.Rect) {
	t.Helper()
	app := newMinesweeperApp(42)
	label := "经典棋盘"
	if isDuel {
		label = "决斗棋盘"
		app.diffIdx = findDifficultyIdx("duel")
		app.game = NewGame(Difficulty{
			ID:           "canvas-ptr-duel",
			Song:         "春日影",
			Gameplay:     GameplayDuel,
			Rows:         40,
			Cols:         40,
			Mines:        2,
			SkillCharges: 2,
			DuelDuration: 300,
			AIInterval:   1e9,
		}, true, MemberTomori, 42)
		app.game.duel.NextAI = 1e9
	} else {
		app.game = NewGame(Difficulty{
			ID:           "canvas-ptr-classic",
			Song:         "诗超绊",
			Gameplay:     GameplayClassic,
			Rows:         40,
			Cols:         40,
			Mines:        2,
			SkillCharges: 2,
		}, true, MemberTomori, 42)
	}
	app.game.FirstClick = true
	app.game.Status = StatusPlaying
	app.game.Cell(18, 18).IsMine = true
	app.game.Cell(39, 39).IsMine = true
	app.game.RecomputeAdjacency()

	tt := ui.NewTester(app.view, 1280, 900)
	tt.Frame()
	r, ok := tt.Find(label)
	if !ok {
		t.Fatalf("%q not found", label)
	}
	app.board.center(app.game, 20, 20)
	tt.Frame()
	return app, tt, r
}

func canvasCellRect(board ui.Rect, v *boardView, row, col int) ui.Rect {
	return ui.Rect{
		X: board.X + float32(col)*v.Cell - v.X,
		Y: board.Y + float32(row)*v.Cell - v.Y,
		W: v.Cell,
		H: v.Cell,
	}
}

func TestCanvasPointerPressReleaseAndCancel(t *testing.T) {
	for _, isDuel := range []bool{false, true} {
		modeName := "classic-canvas"
		if isDuel {
			modeName = "duel-canvas"
		}
		t.Run(modeName, func(t *testing.T) {
			t.Run("press arms without mutating rules and any in-cell displacement reveals on release", func(t *testing.T) {
				app, tt, board := setupDeterministicCanvasBoard(t, isDuel)
				v, g := app.board, app.game
				// Use cell (19,19) which has Adjacent=1 (single safe reveal, no flood).
				r := canvasCellRect(board, v, 19, 19)
				beforeX, beforeY := v.X, v.Y

				tt.Press(r.X+2, r.Y+2)
				if g.Cell(19, 19).Revealed || g.Moves != 0 || g.RevealedSafe != 0 {
					t.Fatalf("press mutated rules early: revealed=%v moves=%d safe=%d",
						g.Cell(19, 19).Revealed, g.Moves, g.RevealedSafe)
				}
				if isDuel && g.duel.PlayerScore != 0 {
					t.Fatalf("press mutated duel score early: %d", g.duel.PlayerScore)
				}

				// Move >5px across the same cell (e.g. 22+22=44px Manhattan at Cell=26):
				// must stay armed without panning camera or mutating rules.
				tt.Move(r.X+r.W-2, r.Y+r.H-2)
				if v.X != beforeX || v.Y != beforeY {
					t.Fatalf("in-cell move shifted camera: got (%v,%v), want (%v,%v)", v.X, v.Y, beforeX, beforeY)
				}
				if g.Cell(19, 19).Revealed || g.Moves != 0 {
					t.Fatalf("in-cell move revealed cell before release: revealed=%v moves=%d",
						g.Cell(19, 19).Revealed, g.Moves)
				}

				tt.Release(r.X+r.W-2, r.Y+r.H-2)
				if v.X != beforeX || v.Y != beforeY {
					t.Fatalf("same-cell release shifted camera: got (%v,%v), want (%v,%v)", v.X, v.Y, beforeX, beforeY)
				}
				if !g.Cell(19, 19).Revealed || g.Moves != 1 || g.RevealedSafe != 1 {
					t.Fatalf("same-cell release failed to reveal cleanly: revealed=%v moves=%d safe=%d",
						g.Cell(19, 19).Revealed, g.Moves, g.RevealedSafe)
				}
			})

			t.Run("leave cell before release cancels without panning", func(t *testing.T) {
				app, tt, board := setupDeterministicCanvasBoard(t, isDuel)
				v, g := app.board, app.game
				origin := canvasCellRect(board, v, 19, 19)
				neighbor := canvasCellRect(board, v, 19, 20)
				beforeX, beforeY := v.X, v.Y

				tt.Press(origin.X+origin.W/2, origin.Y+origin.H/2)
				tt.Move(neighbor.X+neighbor.W/2, neighbor.Y+neighbor.H/2)
				tt.Release(neighbor.X+neighbor.W/2, neighbor.Y+neighbor.H/2)

				if v.X != beforeX || v.Y != beforeY {
					t.Fatalf("leaving cell panned camera: got (%v,%v), want (%v,%v)", v.X, v.Y, beforeX, beforeY)
				}
				if g.Cell(19, 19).Revealed || g.Cell(19, 20).Revealed || g.Moves != 0 {
					t.Fatalf("leaving cell still revealed: (19,19)=%v (19,20)=%v moves=%d",
						g.Cell(19, 19).Revealed, g.Cell(19, 20).Revealed, g.Moves)
				}
			})

			t.Run("leave cell then return and release does not recover", func(t *testing.T) {
				app, tt, board := setupDeterministicCanvasBoard(t, isDuel)
				v, g := app.board, app.game
				origin := canvasCellRect(board, v, 19, 19)
				neighbor := canvasCellRect(board, v, 19, 20)
				beforeX, beforeY := v.X, v.Y

				tt.Press(origin.X+origin.W/2, origin.Y+origin.H/2)
				tt.Move(neighbor.X+neighbor.W/2, neighbor.Y+neighbor.H/2)
				tt.Move(origin.X+origin.W/2, origin.Y+origin.H/2)
				tt.Release(origin.X+origin.W/2, origin.Y+origin.H/2)

				if v.X != beforeX || v.Y != beforeY {
					t.Fatalf("leave-and-return panned camera: got (%v,%v), want (%v,%v)", v.X, v.Y, beforeX, beforeY)
				}
				if g.Cell(19, 19).Revealed || g.Moves != 0 || g.RevealedSafe != 0 {
					t.Fatalf("returning after leave revealed cell: revealed=%v moves=%d safe=%d",
						g.Cell(19, 19).Revealed, g.Moves, g.RevealedSafe)
				}

				// Direct release back on origin without an intermediate Move back must also stay canceled.
				tt.Press(origin.X+origin.W/2, origin.Y+origin.H/2)
				tt.Move(neighbor.X+neighbor.W/2, neighbor.Y+neighbor.H/2)
				tt.Release(origin.X+origin.W/2, origin.Y+origin.H/2)
				if g.Cell(19, 19).Revealed || g.Moves != 0 {
					t.Fatalf("direct release on origin after leave bypassed cancel: revealed=%v moves=%d",
						g.Cell(19, 19).Revealed, g.Moves)
				}

				// Leaving onto the minimap and returning to origin must also cancel.
				mini := v.minimapBoundsFor(g)
				tt.Press(origin.X+origin.W/2, origin.Y+origin.H/2)
				tt.Move(board.X+mini.X+mini.W/2, board.Y+mini.Y+mini.H/2)
				tt.Move(origin.X+origin.W/2, origin.Y+origin.H/2)
				tt.Release(origin.X+origin.W/2, origin.Y+origin.H/2)
				if g.Cell(19, 19).Revealed || g.Moves != 0 {
					t.Fatalf("leaving onto minimap and returning revealed cell: revealed=%v moves=%d",
						g.Cell(19, 19).Revealed, g.Moves)
				}
			})

			t.Run("system cancel and mid-press flag toggle or reset do not commit", func(t *testing.T) {
				app, tt, board := setupDeterministicCanvasBoard(t, isDuel)
				v, g := app.board, app.game
				origin := canvasCellRect(board, v, 19, 19)

				tt.Press(origin.X+origin.W/2, origin.Y+origin.H/2)
				tt.CancelPress()
				if g.Cell(19, 19).Revealed || g.Moves != 0 {
					t.Fatalf("CancelPress revealed cell: revealed=%v moves=%d", g.Cell(19, 19).Revealed, g.Moves)
				}

				// Toggling flagMode mid-press cancels armed reveal and does not flag on release.
				tt.Press(origin.X+origin.W/2, origin.Y+origin.H/2)
				tt.Key(0, ui.KeyF)
				tt.Release(origin.X+origin.W/2, origin.Y+origin.H/2)
				if g.Cell(19, 19).Revealed || g.Cell(19, 19).Flagged || g.Moves != 0 {
					t.Fatalf("release after mid-press KeyF mutated cell: revealed=%v flagged=%v moves=%d",
						g.Cell(19, 19).Revealed, g.Cell(19, 19).Flagged, g.Moves)
				}

				// Toggling flagMode on then back off mid-press must stay canceled.
				tt.Key(0, ui.KeyF)
				tt.Press(origin.X+origin.W/2, origin.Y+origin.H/2)
				tt.Key(0, ui.KeyF)
				tt.Key(0, ui.KeyF)
				tt.Release(origin.X+origin.W/2, origin.Y+origin.H/2)
				if g.Cell(19, 19).Revealed || g.Cell(19, 19).Flagged || g.Moves != 0 {
					t.Fatalf("release after double KeyF mid-press mutated cell: revealed=%v flagged=%v moves=%d",
						g.Cell(19, 19).Revealed, g.Cell(19, 19).Flagged, g.Moves)
				}

				// Mid-press R reset must not reveal on the new board upon release.
				tt.Press(origin.X+origin.W/2, origin.Y+origin.H/2)
				tt.Key(0, ui.KeyR)
				tt.Release(origin.X+origin.W/2, origin.Y+origin.H/2)
				if app.game.Status != StatusIdle || app.game.Moves != 0 {
					t.Fatalf("release after mid-press reset started new board: status=%v moves=%d",
						app.game.Status, app.game.Moves)
				}
			})

			t.Run("mid-press camera command keeps armed while pointer stays in cell and cancels once cell leaves", func(t *testing.T) {
				// 1. Ctrl+0 on an already-centered board keeps pointer in the same cell -> release reveals.
				app, tt, board := setupDeterministicCanvasBoard(t, isDuel)
				v, g := app.board, app.game
				r := canvasCellRect(board, v, 19, 19)
				px, py := r.X+r.W/2, r.Y+r.H/2

				tt.Press(px, py)
				tt.Key(ui.Ctrl, ui.Key0)
				tt.Release(px, py)
				if !g.Cell(19, 19).Revealed || g.Moves != 1 {
					t.Fatalf("no-op Ctrl+0 canceled same-cell press: revealed=%v moves=%d",
						g.Cell(19, 19).Revealed, g.Moves)
				}

				// 2. Small scroll (2 DIPs) while pointer stays inside the same cell -> release reveals.
				app2, tt2, board2 := setupDeterministicCanvasBoard(t, isDuel)
				v2, g2 := app2.board, app2.game
				r2 := canvasCellRect(board2, v2, 19, 19)
				px2, py2 := r2.X+r2.W/2, r2.Y+r2.H/2

				tt2.Press(px2, py2)
				tt2.Scroll(px2, py2, 2, 0)
				tt2.Release(px2, py2)
				if !g2.Cell(19, 19).Revealed || g2.Moves != 1 {
					t.Fatalf("in-cell scroll canceled press: revealed=%v moves=%d",
						g2.Cell(19, 19).Revealed, g2.Moves)
				}

				// 3. Large scroll moves cell away from pointer, then scrolling back does NOT recover.
				app3, tt3, board3 := setupDeterministicCanvasBoard(t, isDuel)
				v3, g3 := app3.board, app3.game
				r3 := canvasCellRect(board3, v3, 19, 19)
				px3, py3 := r3.X+r3.W/2, r3.Y+r3.H/2

				tt3.Press(px3, py3)
				tt3.Scroll(px3, py3, 60, 0)
				tt3.Scroll(px3, py3, -60, 0)
				tt3.Release(px3, py3)
				if g3.Cell(19, 19).Revealed || g3.Moves != 0 {
					t.Fatalf("scrolling away and back unexpectedly recovered press: revealed=%v moves=%d",
						g3.Cell(19, 19).Revealed, g3.Moves)
				}
			})
		})
	}
}

func TestCanvasPointerNumberChordDuelClaimAndSameFrame(t *testing.T) {
	t.Run("classic canvas number cell chords on same-cell release and cancels on leave", func(t *testing.T) {
		app, tt, board := setupDeterministicCanvasBoard(t, false)
		v, g := app.board, app.game
		// Mine is at (18,18); reveal number (19,19) (Adjacent=1) and flag (18,18).
		g.Cell(19, 19).Revealed = true
		g.RevealedSafe = 1
		g.ToggleFlag(18, 18, false)
		tt.Frame()

		numRect := canvasCellRect(board, v, 19, 19)
		otherRect := canvasCellRect(board, v, 19, 20)

		// Leave and return cancels chord.
		tt.Press(numRect.X+numRect.W/2, numRect.Y+numRect.H/2)
		if g.RevealedSafe != 1 || g.Cell(18, 19).Revealed {
			t.Fatalf("pressing number cell chorded early: safe=%d", g.RevealedSafe)
		}
		tt.Move(otherRect.X+otherRect.W/2, otherRect.Y+otherRect.H/2)
		tt.Move(numRect.X+numRect.W/2, numRect.Y+numRect.H/2)
		tt.Release(numRect.X+numRect.W/2, numRect.Y+numRect.H/2)
		if g.RevealedSafe != 1 || g.Cell(18, 19).Revealed {
			t.Fatalf("number cell chorded after leave and return: safe=%d", g.RevealedSafe)
		}

		// In-cell displacement (>5px) still chords on release without mutating on press.
		tt.Press(numRect.X+2, numRect.Y+2)
		if g.RevealedSafe != 1 || g.Cell(18, 19).Revealed {
			t.Fatalf("pressing chordable number mutated early: safe=%d", g.RevealedSafe)
		}
		tt.Move(numRect.X+numRect.W-2, numRect.Y+numRect.H-2)
		if g.RevealedSafe != 1 || g.Cell(18, 19).Revealed {
			t.Fatalf("moving inside chordable number mutated early: safe=%d", g.RevealedSafe)
		}
		tt.Release(numRect.X+numRect.W-2, numRect.Y+numRect.H-2)
		if !g.Cell(18, 19).Revealed || !g.Cell(19, 18).Revealed || g.RevealedSafe <= 1 {
			t.Fatalf("same-cell release on classic number did not chord: safe=%d (18,19)=%v (19,18)=%v",
				g.RevealedSafe, g.Cell(18, 19).Revealed, g.Cell(19, 18).Revealed)
		}
	})

	t.Run("duel canvas number cell claims mine first then chords on same-cell release", func(t *testing.T) {
		app, tt, board := setupDeterministicCanvasBoard(t, true)
		v, g := app.board, app.game
		// Reveal all neighbors of (19,19) except mine (18,18) so (19,19) is HintClaimable,
		// while leaving (17,19) unrevealed so (18,19) becomes HintChordable once (18,18) is claimed.
		for dr := -1; dr <= 1; dr++ {
			for dc := -1; dc <= 1; dc++ {
				r, c := 19+dr, 19+dc
				if r == 18 && c == 18 {
					continue
				}
				g.Cell(r, c).Revealed = true
				g.Cell(r, c).Owner = DuelPlayer
				g.RevealedSafe++
			}
		}
		tt.Frame()

		claimRect := canvasCellRect(board, v, 19, 19)
		chordRect := canvasCellRect(board, v, 18, 19)
		awayRect := canvasCellRect(board, v, 20, 20)

		// Leave and return on claimable number cancels.
		tt.Press(claimRect.X+claimRect.W/2, claimRect.Y+claimRect.H/2)
		if g.Cell(18, 18).Flagged || g.duel.PlayerScore != 0 {
			t.Fatalf("pressing claimable number mutated early: flagged=%v score=%d",
				g.Cell(18, 18).Flagged, g.duel.PlayerScore)
		}
		tt.Move(awayRect.X+awayRect.W/2, awayRect.Y+awayRect.H/2)
		tt.Move(claimRect.X+claimRect.W/2, claimRect.Y+claimRect.H/2)
		tt.Release(claimRect.X+claimRect.W/2, claimRect.Y+claimRect.H/2)
		if g.Cell(18, 18).Flagged || g.duel.PlayerScore != 0 {
			t.Fatalf("claimable number committed after leave and return: flagged=%v score=%d",
				g.Cell(18, 18).Flagged, g.duel.PlayerScore)
		}

		// Same-cell release on (19,19) prioritizes claiming mine (18,18) for +75.
		tt.Press(claimRect.X+2, claimRect.Y+2)
		if g.Cell(18, 18).Flagged || g.duel.PlayerScore != 0 {
			t.Fatalf("pressing claimable number claimed early: flagged=%v score=%d",
				g.Cell(18, 18).Flagged, g.duel.PlayerScore)
		}
		tt.Move(claimRect.X+claimRect.W-2, claimRect.Y+claimRect.H-2)
		tt.Release(claimRect.X+claimRect.W-2, claimRect.Y+claimRect.H-2)
		if !g.Cell(18, 18).Flagged || !g.Cell(18, 18).FlagScored || g.duel.PlayerScore != 75 {
			t.Fatalf("same-cell release did not claim duel mine first: flagged=%v scored=%v score=%d",
				g.Cell(18, 18).Flagged, g.Cell(18, 18).FlagScored, g.duel.PlayerScore)
		}

		// Now (18,19) (Adjacent=1, 1 flagged mine at (18,18)) is HintChordable:
		// press stays inert, same-cell release chords unrevealed safe neighbors like (17,19).
		scoreBeforeChord := g.duel.PlayerScore
		tt.Press(chordRect.X+chordRect.W/2, chordRect.Y+chordRect.H/2)
		if g.Cell(17, 19).Revealed || g.duel.PlayerScore != scoreBeforeChord {
			t.Fatalf("pressing chordable duel number chorded early: (17,19)=%v score=%d",
				g.Cell(17, 19).Revealed, g.duel.PlayerScore)
		}
		tt.Release(chordRect.X+chordRect.W/2, chordRect.Y+chordRect.H/2)
		if !g.Cell(17, 19).Revealed || g.duel.PlayerScore <= scoreBeforeChord {
			t.Fatalf("same-cell release on chordable duel number did not chord: (17,19)=%v score=%d (was %d)",
				g.Cell(17, 19).Revealed, g.duel.PlayerScore, scoreBeforeChord)
		}
	})

	t.Run("same-frame press and release commits reliably on both canvas surfaces", func(t *testing.T) {
		for _, isDuel := range []bool{false, true} {
			// 1. Instant ClickAt through ui.Tester (zero elapsed clock).
			app, tt, board := setupDeterministicCanvasBoard(t, isDuel)
			r := canvasCellRect(board, app.board, 19, 19)
			tt.ClickAt(r.X+r.W/2, r.Y+r.H/2)
			if !app.game.Cell(19, 19).Revealed || app.game.Moves != 1 {
				t.Fatalf("duel=%v same-timestamp ClickAt did not reveal cleanly: revealed=%v moves=%d",
					isDuel, app.game.Cell(19, 19).Revealed, app.game.Moves)
			}

			// 2. Back-to-back InputPointerDown and InputPointerUp in handleCanvasInput with zero intervening frames.
			app2, tt2, board2 := setupDeterministicCanvasBoard(t, isDuel)
			r2 := canvasCellRect(board2, app2.board, 19, 19)
			lx, ly := r2.X-board2.X+r2.W/2, r2.Y-board2.Y+r2.H/2
			now := tt2.Now()
			if !app2.handleCanvasInput(ui.InputEvent{
				Kind:   ui.InputPointerDown,
				Button: 0,
				X:      lx,
				Y:      ly,
				Time:   now,
			}, now) {
				t.Fatalf("duel=%v InputPointerDown not handled", isDuel)
			}
			if app2.game.Cell(19, 19).Revealed || app2.game.Moves != 0 {
				t.Fatalf("duel=%v InputPointerDown mutated rules before InputPointerUp", isDuel)
			}
			if !app2.handleCanvasInput(ui.InputEvent{
				Kind:   ui.InputPointerUp,
				Button: 0,
				X:      lx,
				Y:      ly,
				Time:   now,
			}, now) {
				t.Fatalf("duel=%v InputPointerUp not handled", isDuel)
			}
			tt2.Frame()
			if !app2.game.Cell(19, 19).Revealed || app2.game.Moves != 1 {
				t.Fatalf("duel=%v same-frame down+up did not commit once: revealed=%v moves=%d",
					isDuel, app2.game.Cell(19, 19).Revealed, app2.game.Moves)
			}
		}
	})
}

func TestCanvasUnmodifiedLeftDragNoPanAndPreservedAccessibilityAndKeyboardPaths(t *testing.T) {
	// 1. Unmodified left-button drag across cells never moves the camera on either canvas surface,
	// in both normal mode and flag mode.
	for _, isDuel := range []bool{false, true} {
		for _, flagMode := range []bool{false, true} {
			app, tt, board := setupDeterministicCanvasBoard(t, isDuel)
			v, g := app.board, app.game
			app.flagMode = flagMode
			tt.Frame()

			startX, startY, startCell := v.X, v.Y, v.Cell
			origin := canvasCellRect(board, v, 19, 19)
			far := canvasCellRect(board, v, 23, 25)
			tt.Press(origin.X+origin.W/2, origin.Y+origin.H/2)
			tt.Move(far.X+far.W/2, far.Y+far.H/2)
			tt.Release(far.X+far.W/2, far.Y+far.H/2)

			if v.X != startX || v.Y != startY || v.Cell != startCell || v.gliding() {
				t.Fatalf("duel=%v flagMode=%v unmodified left drag moved camera: got (%v,%v,%.2f) gliding=%v, want (%v,%v,%.2f)",
					isDuel, flagMode, v.X, v.Y, v.Cell, v.gliding(), startX, startY, startCell)
			}
			if flagMode {
				if g.Moves != 0 || g.FlagsPlaced != 1 || !g.Cell(19, 19).Flagged || g.Cell(23, 25).Flagged {
					t.Fatalf("duel=%v flagMode=true unmodified left drag unexpected flag state: moves=%d flags=%d origin=%v far=%v",
						isDuel, g.Moves, g.FlagsPlaced, g.Cell(19, 19).Flagged, g.Cell(23, 25).Flagged)
				}
			} else if g.Moves != 0 || g.FlagsPlaced != 0 {
				t.Fatalf("duel=%v flagMode=false unmodified left drag leaked cell action: moves=%d flags=%d",
					isDuel, g.Moves, g.FlagsPlaced)
			}
		}
	}

	// 2. Canvas existing accessibility and keyboard paths remain intact (Tower Ruling Plan A):
	// boardBox stays focusable, hover Value/Description exposes cell state, and canvas shortcuts work.
	for _, tc := range []struct {
		isDuel bool
		label  string
	}{
		{false, "经典棋盘"},
		{true, "决斗棋盘"},
	} {
		app, tt, board := setupDeterministicCanvasBoard(t, tc.isDuel)
		tt.UseScreenReader()
		app.game.Cell(19, 19).Revealed = true
		app.game.RevealedSafe = 1
		tt.Frame()

		cellRect := canvasCellRect(board, app.board, 19, 19)
		tt.Move(cellRect.X+cellRect.W/2, cellRect.Y+cellRect.H/2)
		tt.Frame()

		info, ok := tt.Access(tc.label)
		if !ok {
			t.Fatalf("%q missing from accessibility tree", tc.label)
		}
		if !info.Focusable || info.Disabled {
			t.Fatalf("%q accessibility focusable/disabled unexpected: focusable=%v disabled=%v",
				tc.label, info.Focusable, info.Disabled)
		}
		if info.Value != "1" || info.Description != "1" {
			t.Fatalf("%q hover accessibility value/desc = (%q,%q), want (\"1\",\"1\")",
				tc.label, info.Value, info.Description)
		}

		// Canvas keyboard shortcuts (Ctrl+=, Ctrl+-, Ctrl+0, F, E, R) stay operational.
		beforeCell := app.board.Cell
		tt.Key(ui.Ctrl, ui.KeyEqual)
		tt.Advance(260 * time.Millisecond)
		if app.board.Cell <= beforeCell {
			t.Fatalf("%q Ctrl+= did not zoom in: before=%.2f after=%.2f", tc.label, beforeCell, app.board.Cell)
		}
		zoomedCell := app.board.Cell
		tt.Key(ui.Ctrl, ui.KeyMinus)
		tt.Advance(260 * time.Millisecond)
		if app.board.Cell >= zoomedCell {
			t.Fatalf("%q Ctrl+- did not zoom out: zoomed=%.2f after=%.2f", tc.label, zoomedCell, app.board.Cell)
		}
		app.board.center(app.game, 0, 0)
		tt.Frame()
		tt.Key(ui.Ctrl, ui.Key0)
		tt.Advance(260 * time.Millisecond)
		if rEnd, cEnd, ok := app.board.cellAt(app.game, board.W/2, board.H/2); !ok || rEnd != app.game.Rows/2 || cEnd != app.game.Cols/2 {
			t.Fatalf("%q Ctrl+0 landed at (%d,%d), want center (%d,%d)", tc.label, rEnd, cEnd, app.game.Rows/2, app.game.Cols/2)
		}
		tt.Key(0, ui.KeyF)
		if !app.flagMode {
			t.Fatalf("%q KeyF did not toggle flagMode on", tc.label)
		}
		tt.Key(0, ui.KeyF)
		skillsBefore := app.game.SkillUsesLeft
		tt.Key(0, ui.KeyE)
		if app.game.SkillUsesLeft != skillsBefore-1 {
			t.Fatalf("%q KeyE did not activate skill: left=%d (was %d)", tc.label, app.game.SkillUsesLeft, skillsBefore)
		}
		tt.Key(0, ui.KeyR)
		if app.game.Status != StatusIdle {
			t.Fatalf("%q KeyR did not reset game: status=%v", tc.label, app.game.Status)
		}
	}

	// 3. Existing widget-grid focused-cell keyboard activation (Enter/Space) stays unaffected.
	gApp, gTT := setupDeterministicGridBoard(t)
	r11 := mustFindCell(t, gTT, "cell-1-1")
	r12 := mustFindCell(t, gTT, "cell-1-2")
	gTT.Press(r11.X+r11.W/2, r11.Y+r11.H/2)
	gTT.Move(r12.X+r12.W/2, r12.Y+r12.H/2)
	gTT.Release(r12.X+r12.W/2, r12.Y+r12.H/2)
	if !gTT.Focused("cell-1-1") || gApp.game.Cell(1, 1).Revealed {
		t.Fatalf("grid focus precondition failed: focused=%v revealed=%v", gTT.Focused("cell-1-1"), gApp.game.Cell(1, 1).Revealed)
	}
	gTT.Key(0, ui.KeyEnter)
	if !gApp.game.Cell(1, 1).Revealed || gApp.game.Moves != 1 {
		t.Fatalf("grid Enter did not activate focused cell-1-1: revealed=%v moves=%d", gApp.game.Cell(1, 1).Revealed, gApp.game.Moves)
	}
	gTT.Press(r12.X+r12.W/2, r12.Y+r12.H/2)
	gTT.Move(r11.X+r11.W/2, r11.Y+r11.H/2)
	gTT.Release(r11.X+r11.W/2, r11.Y+r11.H/2)
	gTT.Key(0, ui.KeySpace)
	if !gApp.game.Cell(1, 2).Revealed || gApp.game.Moves != 2 {
		t.Fatalf("grid Space did not activate focused cell-1-2: revealed=%v moves=%d", gApp.game.Cell(1, 2).Revealed, gApp.game.Moves)
	}
}

func TestCanvasPointerFlagOnPressAndMinimapRightClick(t *testing.T) {
	for _, isDuel := range []bool{false, true} {
		modeName := "classic-canvas"
		if isDuel {
			modeName = "duel-canvas"
		}
		t.Run(modeName, func(t *testing.T) {
			t.Run("right press toggles immediately without double toggle on release", func(t *testing.T) {
				app, tt, board := setupDeterministicCanvasBoard(t, isDuel)
				v, g := app.board, app.game
				r := canvasCellRect(board, v, 19, 19)
				cx, cy := r.X+r.W/2, r.Y+r.H/2
				cell := g.Cell(19, 19)

				tt.PressButton(1, cx, cy)
				if !cell.Flagged || cell.Questioned || g.FlagsPlaced != 1 {
					t.Fatalf("right press did not flag before release: flagged=%v q=%v total=%d",
						cell.Flagged, cell.Questioned, g.FlagsPlaced)
				}
				if isDuel && cell.FlagOwner != DuelPlayer {
					t.Fatalf("duel right press did not assign FlagOwner=DuelPlayer: got %v", cell.FlagOwner)
				}
				tt.ReleaseButton(1, cx, cy)
				tt.Frame()
				if !cell.Flagged || cell.Questioned || g.FlagsPlaced != 1 {
					t.Fatalf("right release toggled a second time: flagged=%v q=%v total=%d",
						cell.Flagged, cell.Questioned, g.FlagsPlaced)
				}

				// Second right press unflags immediately; release does not re-flag.
				tt.PressButton(1, cx, cy)
				if cell.Flagged || cell.Questioned || g.FlagsPlaced != 0 {
					t.Fatalf("second right press did not unflag before release: flagged=%v q=%v total=%d",
						cell.Flagged, cell.Questioned, g.FlagsPlaced)
				}
				tt.ReleaseButton(1, cx, cy)
				tt.Frame()
				if cell.Flagged || cell.Questioned || g.FlagsPlaced != 0 {
					t.Fatalf("second right release toggled again: flagged=%v q=%v total=%d",
						cell.Flagged, cell.Questioned, g.FlagsPlaced)
				}

				if !isDuel {
					// Classic canvas with allowQuestion=true steps blank -> flag -> ? -> blank on press.
					app.allowQuestion = true
					for step, want := range []struct {
						flagged    bool
						questioned bool
					}{
						{true, false},
						{false, true},
						{false, false},
					} {
						tt.PressButton(1, cx, cy)
						if cell.Flagged != want.flagged || cell.Questioned != want.questioned {
							t.Fatalf("allowQuestion step %d press: got (%v,%v) want (%v,%v)",
								step+1, cell.Flagged, cell.Questioned, want.flagged, want.questioned)
						}
						tt.ReleaseButton(1, cx, cy)
						tt.Frame()
						if cell.Flagged != want.flagged || cell.Questioned != want.questioned {
							t.Fatalf("allowQuestion step %d release double-toggled: got (%v,%v) want (%v,%v)",
								step+1, cell.Flagged, cell.Questioned, want.flagged, want.questioned)
						}
					}
				}

				// Dragging right button from origin to neighbor flags origin once without panning or flagging neighbor.
				app.allowQuestion = false
				neighbor := canvasCellRect(board, v, 19, 20)
				beforeX, beforeY := v.X, v.Y
				tt.PressButton(1, cx, cy)
				if !cell.Flagged || g.FlagsPlaced != 1 {
					t.Fatalf("right press before drag did not flag origin: flagged=%v total=%d", cell.Flagged, g.FlagsPlaced)
				}
				tt.Move(neighbor.X+neighbor.W/2, neighbor.Y+neighbor.H/2)
				tt.ReleaseButton(1, neighbor.X+neighbor.W/2, neighbor.Y+neighbor.H/2)
				tt.Frame()
				if v.X != beforeX || v.Y != beforeY {
					t.Fatalf("right drag panned camera: got (%v,%v) want (%v,%v)", v.X, v.Y, beforeX, beforeY)
				}
				if !cell.Flagged || g.Cell(19, 20).Flagged || g.FlagsPlaced != 1 {
					t.Fatalf("right drag altered flags on move/release: origin=%v neighbor=%v total=%d",
						cell.Flagged, g.Cell(19, 20).Flagged, g.FlagsPlaced)
				}
			})

			t.Run("flag mode left press toggles immediately without double toggle on release", func(t *testing.T) {
				app, tt, board := setupDeterministicCanvasBoard(t, isDuel)
				v, g := app.board, app.game
				r := canvasCellRect(board, v, 19, 19)
				cx, cy := r.X+r.W/2, r.Y+r.H/2
				cell := g.Cell(19, 19)

				tt.Key(0, ui.KeyF)
				if !app.flagMode {
					t.Fatal("expected flagMode=true after KeyF")
				}

				tt.Press(cx, cy)
				if !cell.Flagged || cell.Questioned || g.FlagsPlaced != 1 {
					t.Fatalf("flag-mode left press did not flag before release: flagged=%v q=%v total=%d",
						cell.Flagged, cell.Questioned, g.FlagsPlaced)
				}
				tt.Release(cx, cy)
				tt.Frame()
				if !cell.Flagged || cell.Questioned || g.FlagsPlaced != 1 {
					t.Fatalf("flag-mode left release toggled a second time: flagged=%v q=%v total=%d",
						cell.Flagged, cell.Questioned, g.FlagsPlaced)
				}

				// Second left press in flagMode unflags immediately; release does not toggle again.
				tt.Press(cx, cy)
				if cell.Flagged || g.FlagsPlaced != 0 {
					t.Fatalf("second flag-mode left press did not unflag before release: flagged=%v total=%d",
						cell.Flagged, g.FlagsPlaced)
				}
				tt.Release(cx, cy)
				tt.Frame()
				if cell.Flagged || g.FlagsPlaced != 0 {
					t.Fatalf("second flag-mode left release toggled again: flagged=%v total=%d",
						cell.Flagged, g.FlagsPlaced)
				}

				if !isDuel {
					app.allowQuestion = true
					for step, want := range []struct {
						flagged    bool
						questioned bool
					}{
						{true, false},
						{false, true},
						{false, false},
					} {
						tt.Press(cx, cy)
						if cell.Flagged != want.flagged || cell.Questioned != want.questioned {
							t.Fatalf("flagMode allowQuestion step %d press: got (%v,%v) want (%v,%v)",
								step+1, cell.Flagged, cell.Questioned, want.flagged, want.questioned)
						}
						tt.Release(cx, cy)
						tt.Frame()
						if cell.Flagged != want.flagged || cell.Questioned != want.questioned {
							t.Fatalf("flagMode allowQuestion step %d release double-toggled: got (%v,%v) want (%v,%v)",
								step+1, cell.Flagged, cell.Questioned, want.flagged, want.questioned)
						}
					}
					app.allowQuestion = false
				}

				// Toggling flagMode off mid-press must not reveal or unflag on release.
				tt.Press(cx, cy)
				if !cell.Flagged {
					t.Fatal("expected cell flagged on press in flagMode")
				}
				tt.Key(0, ui.KeyF)
				tt.Release(cx, cy)
				tt.Frame()
				if !cell.Flagged || cell.Revealed || g.Moves != 0 {
					t.Fatalf("release after mid-press KeyF off mutated cell: flagged=%v revealed=%v moves=%d",
						cell.Flagged, cell.Revealed, g.Moves)
				}

				// Left click on minimap in flagMode still glides camera without flagging underlying cells.
				tt.Key(0, ui.KeyF)
				mini := v.minimapBoundsFor(g)
				underRow, underCol, ok := v.cellAt(g, mini.X+2, mini.Y+2)
				if !ok {
					t.Fatal("expected valid board cell under minimap top-left")
				}
				flagsBefore := g.FlagsPlaced
				tt.Press(board.X+mini.X+2, board.Y+mini.Y+2)
				tt.Release(board.X+mini.X+2, board.Y+mini.Y+2)
				if g.FlagsPlaced != flagsBefore || g.Cell(underRow, underCol).Flagged {
					t.Fatalf("flagMode left click on minimap flagged underlying cell (%d,%d): flags=%d (was %d)",
						underRow, underCol, g.FlagsPlaced, flagsBefore)
				}
				if !v.gliding() {
					t.Fatal("flagMode left click on minimap did not start camera glide")
				}
			})

			t.Run("right click on minimap does not change any cell", func(t *testing.T) {
				app, tt, board := setupDeterministicCanvasBoard(t, isDuel)
				v, g := app.board, app.game
				mini := v.minimapBoundsFor(g)

				centerLX, centerLY := mini.X+mini.W/2, mini.Y+mini.H/2
				underRow, underCol, ok := v.cellAt(g, centerLX, centerLY)
				if !ok || g.Cell(underRow, underCol).Revealed || g.Cell(underRow, underCol).Flagged {
					t.Fatalf("precondition failed for minimap center cell (%d,%d): ok=%v", underRow, underCol, ok)
				}

				// Right press and release inside minimap (normal mode).
				tt.PressButton(1, board.X+centerLX, board.Y+centerLY)
				if g.FlagsPlaced != 0 || g.Cell(underRow, underCol).Flagged || g.Cell(underRow, underCol).Questioned || g.Moves != 0 {
					t.Fatalf("minimap right press mutated cell (%d,%d): flags=%d flagged=%v q=%v moves=%d",
						underRow, underCol, g.FlagsPlaced, g.Cell(underRow, underCol).Flagged, g.Cell(underRow, underCol).Questioned, g.Moves)
				}
				tt.ReleaseButton(1, board.X+centerLX, board.Y+centerLY)
				tt.Frame()
				if g.FlagsPlaced != 0 || g.Cell(underRow, underCol).Flagged || g.Moves != 0 {
					t.Fatalf("minimap right release mutated cell (%d,%d): flags=%d flagged=%v moves=%d",
						underRow, underCol, g.FlagsPlaced, g.Cell(underRow, underCol).Flagged, g.Moves)
				}

				// Right press inside minimap border, drag outside onto adjacent unrevealed cell, release outside.
				adjRow, adjCol, adjOK := v.cellAt(g, mini.X-2, mini.Y+2)
				if !adjOK || g.Cell(adjRow, adjCol).Revealed {
					t.Fatalf("precondition failed for adjacent cell outside minimap (%d,%d): ok=%v", adjRow, adjCol, adjOK)
				}
				tt.PressButton(1, board.X+mini.X+1, board.Y+mini.Y+2)
				tt.Move(board.X+mini.X-2, board.Y+mini.Y+2)
				tt.ReleaseButton(1, board.X+mini.X-2, board.Y+mini.Y+2)
				tt.Frame()
				if g.FlagsPlaced != 0 || g.Cell(adjRow, adjCol).Flagged || g.Cell(underRow, underCol).Flagged {
					t.Fatalf("minimap right press dragged outside leaked flag: flags=%d adj=%v under=%v",
						g.FlagsPlaced, g.Cell(adjRow, adjCol).Flagged, g.Cell(underRow, underCol).Flagged)
				}

				// Right click on minimap while flagMode=true also leaves cells untouched.
				tt.Key(0, ui.KeyF)
				tt.PressButton(1, board.X+centerLX, board.Y+centerLY)
				tt.ReleaseButton(1, board.X+centerLX, board.Y+centerLY)
				tt.Frame()
				if g.FlagsPlaced != 0 || g.Cell(underRow, underCol).Flagged {
					t.Fatalf("minimap right click in flagMode mutated cell: flags=%d flagged=%v",
						g.FlagsPlaced, g.Cell(underRow, underCol).Flagged)
				}

				// Right click on minimap over an already-flagged cell underneath must not unflag it.
				g.ToggleFlag(underRow, underCol, false)
				tt.Frame()
				tt.PressButton(1, board.X+centerLX, board.Y+centerLY)
				tt.ReleaseButton(1, board.X+centerLX, board.Y+centerLY)
				tt.Frame()
				if !g.Cell(underRow, underCol).Flagged || g.FlagsPlaced != 1 {
					t.Fatalf("minimap right click unflagged existing flag underneath: flagged=%v total=%d",
						g.Cell(underRow, underCol).Flagged, g.FlagsPlaced)
				}
			})
		})
	}
}
