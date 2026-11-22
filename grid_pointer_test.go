package main

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func setupDeterministicGridBoard(t *testing.T) (*minesweeperApp, *ui.Tester) {
	t.Helper()
	app := newMinesweeperApp(42)
	app.game = NewGame(Difficulty{
		ID:       "grid-ptr",
		Song:     "迷星叫",
		Gameplay: GameplayClassic,
		Rows:     5,
		Cols:     5,
		Mines:    1,
	}, false, MemberTomori, 42)
	app.game.FirstClick = true
	app.game.Status = StatusPlaying
	app.game.Cell(0, 0).IsMine = true
	app.game.RecomputeAdjacency()

	tt := ui.NewTester(app.view, 1280, 900)
	return app, tt
}

func mustFindCell(t *testing.T, tt *ui.Tester, label string) ui.Rect {
	t.Helper()
	r, ok := tt.Find(label)
	if !ok {
		t.Fatalf("cell %q not found", label)
	}
	return r
}

func TestGridPointerPressReleaseAndCancel(t *testing.T) {
	t.Run("press arms without mutating rules and same-cell release reveals", func(t *testing.T) {
		app, tt := setupDeterministicGridBoard(t)
		r := mustFindCell(t, tt, "cell-1-1")

		// Press near top-left of the cell: rules stay untouched.
		tt.Press(r.X+4, r.Y+4)
		if app.game.Cell(1, 1).Revealed || app.game.Moves != 0 || app.game.RevealedSafe != 0 {
			t.Fatalf("press mutated rules early: revealed=%v moves=%d safe=%d",
				app.game.Cell(1, 1).Revealed, app.game.Moves, app.game.RevealedSafe)
		}

		// Large pointer jitter within the same cell does not cancel the press.
		tt.Move(r.X+r.W-4, r.Y+r.H-4)
		if app.game.Cell(1, 1).Revealed {
			t.Fatal("in-cell move revealed cell before release")
		}

		tt.Release(r.X+r.W-4, r.Y+r.H-4)
		if !app.game.Cell(1, 1).Revealed || app.game.Moves != 1 || app.game.RevealedSafe != 1 {
			t.Fatalf("same-cell release failed to reveal cleanly: revealed=%v moves=%d safe=%d",
				app.game.Cell(1, 1).Revealed, app.game.Moves, app.game.RevealedSafe)
		}
	})

	t.Run("leave cell before release cancels", func(t *testing.T) {
		app, tt := setupDeterministicGridBoard(t)
		r := mustFindCell(t, tt, "cell-1-1")
		neighbor := mustFindCell(t, tt, "cell-1-2")

		tt.Press(r.X+r.W/2, r.Y+r.H/2)
		tt.Move(neighbor.X+neighbor.W/2, neighbor.Y+neighbor.H/2)
		tt.Release(neighbor.X+neighbor.W/2, neighbor.Y+neighbor.H/2)

		if app.game.Cell(1, 1).Revealed || app.game.Cell(1, 2).Revealed || app.game.Moves != 0 {
			t.Fatalf("leaving cell still revealed: (1,1)=%v (1,2)=%v moves=%d",
				app.game.Cell(1, 1).Revealed, app.game.Cell(1, 2).Revealed, app.game.Moves)
		}
	})

	t.Run("leave cell then return and release does not recover", func(t *testing.T) {
		app, tt := setupDeterministicGridBoard(t)
		r := mustFindCell(t, tt, "cell-1-1")
		neighbor := mustFindCell(t, tt, "cell-1-2")

		tt.Press(r.X+r.W/2, r.Y+r.H/2)
		tt.Move(neighbor.X+neighbor.W/2, neighbor.Y+neighbor.H/2)
		tt.Move(r.X+r.W/2, r.Y+r.H/2)
		tt.Release(r.X+r.W/2, r.Y+r.H/2)

		if app.game.Cell(1, 1).Revealed || app.game.Moves != 0 || app.game.RevealedSafe != 0 {
			t.Fatalf("returning after leave unexpectedly revealed cell: revealed=%v moves=%d safe=%d",
				app.game.Cell(1, 1).Revealed, app.game.Moves, app.game.RevealedSafe)
		}

		// Releasing directly back on the origin cell without an intermediate Move
		// causes mygo/ui to defer the synthetic click via releaseLater; it must
		// still stay canceled across subsequent passes.
		tt.Press(r.X+r.W/2, r.Y+r.H/2)
		tt.Move(neighbor.X+neighbor.W/2, neighbor.Y+neighbor.H/2)
		tt.Release(r.X+r.W/2, r.Y+r.H/2)
		tt.Frame()

		if app.game.Cell(1, 1).Revealed || app.game.Moves != 0 || app.game.RevealedSafe != 0 {
			t.Fatalf("direct release back on origin after leave bypassed cancel: revealed=%v moves=%d safe=%d",
				app.game.Cell(1, 1).Revealed, app.game.Moves, app.game.RevealedSafe)
		}

		// Subsequent keyboard activation on the still-focused cell must not be
		// swallowed by a leftover suppression entry.
		tt.Key(0, ui.KeyEnter)
		if !app.game.Cell(1, 1).Revealed || app.game.Moves != 1 {
			t.Fatalf("Enter after suppressed deferred click did not activate cell: revealed=%v moves=%d",
				app.game.Cell(1, 1).Revealed, app.game.Moves)
		}
	})

	t.Run("in-cell press-to-release displacement does not double-trigger chord", func(t *testing.T) {
		app, tt := setupDeterministicGridBoard(t)
		// Flag the only mine adjacent to (1,1) before revealing (1,1), so a
		// stray deferred click on the newly revealed number would chord the board.
		app.game.ToggleFlag(0, 0, false)
		tt.Frame()

		r := mustFindCell(t, tt, "cell-1-1")
		tt.Press(r.X+4, r.Y+4)
		tt.Release(r.X+r.W-4, r.Y+r.H-4)
		tt.Frame()

		if !app.game.Cell(1, 1).Revealed || app.game.RevealedSafe != 1 || app.game.Moves != 1 || app.game.Status != StatusPlaying {
			t.Fatalf("displaced same-cell release double-acted: revealed=%v safe=%d moves=%d status=%v",
				app.game.Cell(1, 1).Revealed, app.game.RevealedSafe, app.game.Moves, app.game.Status)
		}
	})

	t.Run("system press cancel and mid-press reset or flag toggle do not commit", func(t *testing.T) {
		app, tt := setupDeterministicGridBoard(t)
		r := mustFindCell(t, tt, "cell-1-1")

		tt.Press(r.X+r.W/2, r.Y+r.H/2)
		tt.CancelPress()
		if app.game.Cell(1, 1).Revealed || app.game.Moves != 0 {
			t.Fatalf("CancelPress unexpectedly revealed: revealed=%v moves=%d", app.game.Cell(1, 1).Revealed, app.game.Moves)
		}

		// Holding left button while toggling flag mode with F must cancel the
		// armed reveal and must not plant a flag on release.
		tt.Press(r.X+r.W/2, r.Y+r.H/2)
		tt.Key(0, ui.KeyF)
		tt.Release(r.X+r.W/2, r.Y+r.H/2)
		if app.game.Cell(1, 1).Revealed || app.game.Cell(1, 1).Flagged || app.game.Moves != 0 {
			t.Fatalf("release after mid-press flagMode toggle mutated cell: revealed=%v flagged=%v moves=%d",
				app.game.Cell(1, 1).Revealed, app.game.Cell(1, 1).Flagged, app.game.Moves)
		}

		// Holding left button while resetting with R must not open on the fresh board.
		tt.Key(0, ui.KeyF)
		tt.Press(r.X+r.W/2, r.Y+r.H/2)
		tt.Key(0, ui.KeyR)
		tt.Release(r.X+r.W/2, r.Y+r.H/2)
		if app.game.Status != StatusIdle || app.game.Moves != 0 {
			t.Fatalf("release after mid-press reset started new board: status=%v moves=%d",
				app.game.Status, app.game.Moves)
		}
	})
}

func TestGridPointerNumberChordAndSameFrameClick(t *testing.T) {
	setupChordable := func(t *testing.T) (*minesweeperApp, *ui.Tester) {
		t.Helper()
		app, tt := setupDeterministicGridBoard(t)
		app.game.Cell(1, 1).Revealed = true
		app.game.RevealedSafe = 1
		app.game.ToggleFlag(0, 0, false)
		tt.Frame()
		return app, tt
	}

	t.Run("number cell press stays inert until same-cell release chords", func(t *testing.T) {
		app, tt := setupChordable(t)
		r := mustFindCell(t, tt, "cell-1-1")

		tt.Press(r.X+r.W/2, r.Y+r.H/2)
		if app.game.RevealedSafe != 1 || app.game.Cell(0, 1).Revealed {
			t.Fatalf("pressing number cell chorded early: safe=%d", app.game.RevealedSafe)
		}

		tt.Release(r.X+r.W/2, r.Y+r.H/2)
		if !app.game.Cell(0, 1).Revealed || !app.game.Cell(1, 0).Revealed || app.game.RevealedSafe <= 1 {
			t.Fatalf("releasing on chordable number cell did not chord: safe=%d (0,1)=%v (1,0)=%v",
				app.game.RevealedSafe, app.game.Cell(0, 1).Revealed, app.game.Cell(1, 0).Revealed)
		}
	})

	t.Run("number cell leave and return cancels chord", func(t *testing.T) {
		app, tt := setupChordable(t)
		r := mustFindCell(t, tt, "cell-1-1")
		other := mustFindCell(t, tt, "cell-1-2")

		tt.Press(r.X+r.W/2, r.Y+r.H/2)
		tt.Move(other.X+other.W/2, other.Y+other.H/2)
		tt.Move(r.X+r.W/2, r.Y+r.H/2)
		tt.Release(r.X+r.W/2, r.Y+r.H/2)

		if app.game.RevealedSafe != 1 || app.game.Cell(0, 1).Revealed {
			t.Fatalf("number cell chorded after leaving and returning: safe=%d", app.game.RevealedSafe)
		}
	})

	t.Run("same-frame press and release commits reliably", func(t *testing.T) {
		// 1. Instant ClickAt through ui.Tester (zero elapsed time).
		app, tt := setupDeterministicGridBoard(t)
		r := mustFindCell(t, tt, "cell-1-1")
		tt.ClickAt(r.X+r.W/2, r.Y+r.H/2)
		if !app.game.Cell(1, 1).Revealed || app.game.Moves != 1 {
			t.Fatalf("same-timestamp ClickAt did not reveal cleanly: revealed=%v moves=%d",
				app.game.Cell(1, 1).Revealed, app.game.Moves)
		}

		// 2. Back-to-back InputPointerDown and InputPointerUp with zero intervening frames.
		app2, tt2 := setupDeterministicGridBoard(t)
		r2 := mustFindCell(t, tt2, "cell-1-1")
		idx := 1*app2.game.Cols + 1
		now := tt2.Now()
		if !app2.handleGridTileInput(idx, 1, 1, r2.W, ui.InputEvent{
			Kind:   ui.InputPointerDown,
			Button: 0,
			X:      r2.W / 2,
			Y:      r2.H / 2,
			Time:   now,
		}) {
			t.Fatal("InputPointerDown was not handled")
		}
		if app2.game.Cell(1, 1).Revealed {
			t.Fatal("InputPointerDown mutated game state before InputPointerUp")
		}
		if !app2.handleGridTileInput(idx, 1, 1, r2.W, ui.InputEvent{
			Kind:   ui.InputPointerUp,
			Button: 0,
			X:      r2.W / 2,
			Y:      r2.H / 2,
			Time:   now,
		}) {
			t.Fatal("InputPointerUp was not handled")
		}
		tt2.Frame()
		if !app2.game.Cell(1, 1).Revealed || app2.game.Moves != 1 {
			t.Fatalf("same-frame down+up did not commit once: revealed=%v moves=%d",
				app2.game.Cell(1, 1).Revealed, app2.game.Moves)
		}
	})
}

func TestGridKeyboardActivationAndScreenReaderPressAction(t *testing.T) {
	app, tt := setupDeterministicGridBoard(t)
	tt.UseScreenReader()
	tt.Frame()

	// Screen reader sees the cell as a focusable button (mygo/ui assigns
	// RoleButton only when both flagFocusable and flagClickable/ActionPress are set).
	info, ok := tt.AccessByRole(ui.RoleButton, "cell-1-1")
	if !ok {
		t.Fatal("cell-1-1 missing RoleButton in accessibility tree (press action not exposed)")
	}
	if !info.Focusable || info.Disabled {
		t.Fatalf("cell-1-1 accessibility state unexpected: focusable=%v disabled=%v", info.Focusable, info.Disabled)
	}

	// Focus cell-1-1 via a canceled pointer press (so the cell stays unrevealed
	// while receiving keyboard focus), then activate with Enter.
	r11 := mustFindCell(t, tt, "cell-1-1")
	r12 := mustFindCell(t, tt, "cell-1-2")
	tt.Press(r11.X+r11.W/2, r11.Y+r11.H/2)
	tt.Move(r12.X+r12.W/2, r12.Y+r12.H/2)
	tt.Release(r12.X+r12.W/2, r12.Y+r12.H/2)
	if !tt.Focused("cell-1-1") {
		t.Fatal("expected cell-1-1 to hold focus after press")
	}
	if app.game.Cell(1, 1).Revealed {
		t.Fatal("cell-1-1 should stay unrevealed after canceled drag")
	}
	tt.Key(0, ui.KeyEnter)
	if !app.game.Cell(1, 1).Revealed || app.game.Moves != 1 {
		t.Fatalf("Enter did not activate focused cell-1-1: revealed=%v moves=%d",
			app.game.Cell(1, 1).Revealed, app.game.Moves)
	}

	// Focus cell-1-2 via a canceled pointer press and activate with Space.
	tt.Press(r12.X+r12.W/2, r12.Y+r12.H/2)
	tt.Move(r11.X+r11.W/2, r11.Y+r11.H/2)
	tt.Release(r11.X+r11.W/2, r11.Y+r11.H/2)
	if !tt.Focused("cell-1-2") {
		t.Fatal("expected cell-1-2 to hold focus after press")
	}
	if app.game.Cell(1, 2).Revealed {
		t.Fatal("cell-1-2 should stay unrevealed after canceled drag")
	}
	tt.Key(0, ui.KeySpace)
	if !app.game.Cell(1, 2).Revealed || app.game.Moves != 2 {
		t.Fatalf("Space did not activate focused cell-1-2: revealed=%v moves=%d",
			app.game.Cell(1, 2).Revealed, app.game.Moves)
	}
}
