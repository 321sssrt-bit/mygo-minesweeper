package main

import (
	"testing"
	"time"

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

func TestGridPointerFlagOnPressNoDoubleToggle(t *testing.T) {
	t.Run("right press toggles flag and question immediately without double toggle on release", func(t *testing.T) {
		app, tt := setupDeterministicGridBoard(t)
		r := mustFindCell(t, tt, "cell-1-1")
		cx, cy := r.X+r.W/2, r.Y+r.H/2
		cell := app.game.Cell(1, 1)

		tt.PressButton(1, cx, cy)
		if !cell.Flagged || cell.Questioned || app.game.FlagsPlaced != 1 {
			t.Fatalf("right press did not flag before release: flagged=%v q=%v total=%d",
				cell.Flagged, cell.Questioned, app.game.FlagsPlaced)
		}
		if !tt.HasText("剩雷 0") {
			t.Fatal("HUD remaining mines badge did not update on right press")
		}
		tt.ReleaseButton(1, cx, cy)
		tt.Frame()
		if !cell.Flagged || cell.Questioned || app.game.FlagsPlaced != 1 {
			t.Fatalf("right release toggled flag a second time: flagged=%v q=%v total=%d",
				cell.Flagged, cell.Questioned, app.game.FlagsPlaced)
		}

		// Second right press unflags immediately; release does not re-flag.
		tt.PressButton(1, cx, cy)
		if cell.Flagged || cell.Questioned || app.game.FlagsPlaced != 0 {
			t.Fatalf("second right press did not unflag before release: flagged=%v q=%v total=%d",
				cell.Flagged, cell.Questioned, app.game.FlagsPlaced)
		}
		tt.ReleaseButton(1, cx, cy)
		tt.Frame()
		if cell.Flagged || cell.Questioned || app.game.FlagsPlaced != 0 {
			t.Fatalf("second right release toggled again: flagged=%v q=%v total=%d",
				cell.Flagged, cell.Questioned, app.game.FlagsPlaced)
		}

		// With allowQuestion=true, each press steps blank -> flag -> ? -> blank.
		app.allowQuestion = true
		tt.PressButton(1, cx, cy)
		if !cell.Flagged || cell.Questioned {
			t.Fatalf("allowQuestion step 1 press failed: flagged=%v q=%v", cell.Flagged, cell.Questioned)
		}
		tt.ReleaseButton(1, cx, cy)
		if !cell.Flagged || cell.Questioned {
			t.Fatalf("allowQuestion step 1 release double-toggled: flagged=%v q=%v", cell.Flagged, cell.Questioned)
		}

		tt.PressButton(1, cx, cy)
		if cell.Flagged || !cell.Questioned {
			t.Fatalf("allowQuestion step 2 press failed: flagged=%v q=%v", cell.Flagged, cell.Questioned)
		}
		tt.ReleaseButton(1, cx, cy)
		if cell.Flagged || !cell.Questioned {
			t.Fatalf("allowQuestion step 2 release double-toggled: flagged=%v q=%v", cell.Flagged, cell.Questioned)
		}

		tt.PressButton(1, cx, cy)
		if cell.Flagged || cell.Questioned {
			t.Fatalf("allowQuestion step 3 press failed: flagged=%v q=%v", cell.Flagged, cell.Questioned)
		}
		tt.ReleaseButton(1, cx, cy)
		if cell.Flagged || cell.Questioned {
			t.Fatalf("allowQuestion step 3 release double-toggled: flagged=%v q=%v", cell.Flagged, cell.Questioned)
		}
	})

	t.Run("flag mode left press toggles immediately without double toggle on release", func(t *testing.T) {
		app, tt := setupDeterministicGridBoard(t)
		r11 := mustFindCell(t, tt, "cell-1-1")
		r12 := mustFindCell(t, tt, "cell-1-2")
		cx, cy := r11.X+r11.W/2, r11.Y+r11.H/2
		cell := app.game.Cell(1, 1)

		tt.Key(0, ui.KeyF)
		if !app.flagMode {
			t.Fatal("expected flagMode=true after KeyF")
		}

		tt.Press(cx, cy)
		if !cell.Flagged || cell.Questioned || app.game.FlagsPlaced != 1 {
			t.Fatalf("flag-mode left press did not flag before release: flagged=%v q=%v total=%d",
				cell.Flagged, cell.Questioned, app.game.FlagsPlaced)
		}
		tt.Release(cx, cy)
		tt.Frame()
		if !cell.Flagged || cell.Questioned || app.game.FlagsPlaced != 1 {
			t.Fatalf("flag-mode left release toggled a second time: flagged=%v q=%v total=%d",
				cell.Flagged, cell.Questioned, app.game.FlagsPlaced)
		}

		// Subsequent keyboard Enter on the focused cell still works in flagMode.
		tt.Key(0, ui.KeyEnter)
		if cell.Flagged || app.game.FlagsPlaced != 0 {
			t.Fatalf("Enter on focused cell in flagMode did not unflag: flagged=%v total=%d",
				cell.Flagged, app.game.FlagsPlaced)
		}

		// Full allowQuestion cycle via flag-mode left press.
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

		// Leave and directly release back on origin (deferred releaseLater click) must not double-toggle.
		app.allowQuestion = false
		tt.Press(cx, cy)
		if !cell.Flagged {
			t.Fatal("expected flag on press before drag")
		}
		tt.Move(r12.X+r12.W/2, r12.Y+r12.H/2)
		tt.Release(cx, cy)
		tt.Frame()
		if !cell.Flagged || app.game.Cell(1, 2).Flagged || app.game.FlagsPlaced != 1 {
			t.Fatalf("deferred release after drag toggled flag: (1,1)=%v (1,2)=%v total=%d",
				cell.Flagged, app.game.Cell(1, 2).Flagged, app.game.FlagsPlaced)
		}

		// Toggling flagMode off while holding left button must not reveal on release.
		tt.Press(r12.X+r12.W/2, r12.Y+r12.H/2)
		if !app.game.Cell(1, 2).Flagged {
			t.Fatal("expected (1,2) flagged on press in flagMode")
		}
		tt.Key(0, ui.KeyF)
		tt.Release(r12.X+r12.W/2, r12.Y+r12.H/2)
		tt.Frame()
		if !app.game.Cell(1, 2).Flagged || app.game.Cell(1, 2).Revealed || app.game.Moves != 0 {
			t.Fatalf("release after mid-press KeyF off mutated (1,2): flagged=%v revealed=%v moves=%d",
				app.game.Cell(1, 2).Flagged, app.game.Cell(1, 2).Revealed, app.game.Moves)
		}
	})

	t.Run("right press while holding left press flags immediately and cancels left reveal", func(t *testing.T) {
		app, tt := setupDeterministicGridBoard(t)
		r11 := mustFindCell(t, tt, "cell-1-1")
		r12 := mustFindCell(t, tt, "cell-1-2")

		tt.Press(r11.X+r11.W/2, r11.Y+r11.H/2)
		tt.PressButton(1, r12.X+r12.W/2, r12.Y+r12.H/2)
		if !app.game.Cell(1, 2).Flagged {
			t.Fatal("right press while holding left did not flag (1,2) immediately")
		}
		tt.ReleaseButton(1, r12.X+r12.W/2, r12.Y+r12.H/2)
		tt.Release(r11.X+r11.W/2, r11.Y+r11.H/2)
		tt.Frame()
		if !app.game.Cell(1, 2).Flagged || app.game.Cell(1, 1).Revealed || app.game.Moves != 0 {
			t.Fatalf("unexpected state after interleaved right+left release: (1,2).Flagged=%v (1,1).Revealed=%v moves=%d",
				app.game.Cell(1, 2).Flagged, app.game.Cell(1, 1).Revealed, app.game.Moves)
		}
	})

	t.Run("middle press cancels armed left press without leaking reveal or blocking keyboard", func(t *testing.T) {
		app, tt := setupDeterministicGridBoard(t)
		r11 := mustFindCell(t, tt, "cell-1-1")
		r12 := mustFindCell(t, tt, "cell-1-2")

		// Middle press on the same cell cancels the held left press.
		tt.Press(r11.X+r11.W/2, r11.Y+r11.H/2)
		tt.PressButton(2, r11.X+r11.W/2, r11.Y+r11.H/2)
		tt.Release(r11.X+r11.W/2, r11.Y+r11.H/2)
		tt.ReleaseButton(2, r11.X+r11.W/2, r11.Y+r11.H/2)
		tt.Frame()
		if app.game.Cell(1, 1).Revealed || app.game.Moves != 0 {
			t.Fatalf("left release after same-cell middle press revealed cell: revealed=%v moves=%d",
				app.game.Cell(1, 1).Revealed, app.game.Moves)
		}

		// Middle press on a neighbor cell also cancels the held left press.
		tt.Press(r11.X+r11.W/2, r11.Y+r11.H/2)
		tt.PressButton(2, r12.X+r12.W/2, r12.Y+r12.H/2)
		tt.Release(r11.X+r11.W/2, r11.Y+r11.H/2)
		tt.ReleaseButton(2, r12.X+r12.W/2, r12.Y+r12.H/2)
		tt.Frame()
		if app.game.Cell(1, 1).Revealed || app.game.Moves != 0 {
			t.Fatalf("left release after neighbor middle press revealed cell: revealed=%v moves=%d",
				app.game.Cell(1, 1).Revealed, app.game.Moves)
		}

		// Focused cell still activates via keyboard Enter afterward.
		tt.Key(0, ui.KeyEnter)
		if !app.game.Cell(1, 1).Revealed || app.game.Moves != 1 {
			t.Fatalf("Enter after middle-canceled press did not activate cell: revealed=%v moves=%d",
				app.game.Cell(1, 1).Revealed, app.game.Moves)
		}
	})

	t.Run("same-frame left shield and right press flag preserve both feedbacks in order", func(t *testing.T) {
		app := newMinesweeperApp(42)
		app.memberID = MemberSoyorin
		app.game = NewGame(Difficulty{
			ID:           "grid-ptr-shield",
			Song:         "迷星叫",
			Gameplay:     GameplayClassic,
			Rows:         5,
			Cols:         5,
			Mines:        2,
			SkillCharges: 1,
		}, true, MemberSoyorin, 42)
		app.game.FirstClick = true
		app.game.Status = StatusPlaying
		app.game.Cell(0, 0).IsMine = true
		app.game.Cell(4, 4).IsMine = true
		app.game.RecomputeAdjacency()

		tt := ui.NewTester(app.view, 1280, 900)
		r00 := mustFindCell(t, tt, "cell-0-0")
		r11 := mustFindCell(t, tt, "cell-1-1")
		now := tt.Now()

		// Left press+release on mine (0,0) triggers Soyorin shield; right press on (1,1) arrives before the next frame.
		app.handleGridTileInput(0, 0, 0, r00.W, ui.InputEvent{
			Kind: ui.InputPointerDown, Button: 0, X: r00.W / 2, Y: r00.H / 2, Time: now,
		})
		app.handleGridTileInput(0, 0, 0, r00.W, ui.InputEvent{
			Kind: ui.InputPointerUp, Button: 0, X: r00.W / 2, Y: r00.H / 2, Time: now,
		})
		app.handleGridTileInput(1*app.game.Cols+1, 1, 1, r11.W, ui.InputEvent{
			Kind: ui.InputPointerDown, Button: 1, X: r11.W / 2, Y: r11.H / 2, Time: now,
		})
		tt.Frame()

		if !app.game.Cell(0, 0).ShieldSaved || !app.game.Cell(1, 1).Flagged {
			t.Fatalf("expected both shield and flag applied: shield=%v flag=%v",
				app.game.Cell(0, 0).ShieldSaved, app.game.Cell(1, 1).Flagged)
		}
		tt.Advance(22 * time.Millisecond)
		if sx, sy := app.currentShakeOffset(tt.Now()); sx == 0 && sy == 0 {
			t.Fatal("shield shake was lost when same-frame right press followed left release")
		}
		if _, ok := tt.Find("台词"); !ok || !app.quoteStrip.Active || app.quoteStrip.MemberID != MemberSoyorin {
			t.Fatal("Soyo's quote strip was lost when same-frame right press followed left release")
		}
		if !tt.HasText("长崎爽世发动了「温柔的微笑」：这次就当没点过吧！") {
			t.Fatal("shield toast was lost when same-frame right press followed left release")
		}
	})
}
