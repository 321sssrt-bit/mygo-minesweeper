package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
)

// sevenTrackSetlist builds a seven-track ladder from the existing and
// pre-staged engine tracks to verify the setlist popover against the
// target seven-track layout without altering Difficulties globally.
func sevenTrackSetlist() []Difficulty {
	return []Difficulty{
		Difficulties[0],
		Difficulties[1],
		TrackKageiromai,
		TrackUtakotoba,
		TrackMayoiuta,
		TrackSenzaihyoumei,
		Difficulties[4],
	}
}

func TestSetlistPopoverSelectBySongName(t *testing.T) {
	app := newMinesweeperApp(12345)
	tt := ui.NewTester(app.view, 1280, 900)

	if app.setlistOpen {
		t.Fatal("setlist popover should start collapsed")
	}
	if _, ok := tt.Find("碧天伴走"); ok {
		t.Fatal("non-active track should not be visible while setlist popover is collapsed")
	}

	// Select every track in Difficulties by its song name, including
	// returning to the first track and re-selecting the active track.
	order := []int{1, 2, 3, 4, 0}
	for _, wantIdx := range order {
		want := Difficulties[wantIdx]
		if err := tt.Click("歌单"); err != nil {
			t.Fatalf("open setlist for %s: %v", want.Song, err)
		}
		if !app.setlistOpen {
			t.Fatalf("setlist did not open before selecting %s", want.Song)
		}
		if err := tt.Click(want.Song); err != nil {
			t.Fatalf("select track %q by song name: %v", want.Song, err)
		}
		if app.setlistOpen {
			t.Fatalf("setlist stayed open after selecting %s", want.Song)
		}
		if app.diffIdx != wantIdx || app.game.Difficulty.ID != want.ID {
			t.Fatalf("selected idx=%d id=%q, want idx=%d id=%q", app.diffIdx, app.game.Difficulty.ID, wantIdx, want.ID)
		}
	}

	// Re-selecting the already active track closes the popover without
	// resetting an in-progress game.
	if err := tt.Click("cell-4-4"); err != nil {
		t.Fatalf("start game: %v", err)
	}
	beforeGame := app.game
	if err := tt.Click("歌单"); err != nil {
		t.Fatalf("open setlist during play: %v", err)
	}
	if err := tt.Click(Difficulties[0].Song); err != nil {
		t.Fatalf("re-select active track %q: %v", Difficulties[0].Song, err)
	}
	if app.setlistOpen || app.game != beforeGame || app.game.Status != StatusPlaying {
		t.Fatalf("re-selecting active track should only close popover: open=%v sameGame=%v status=%v", app.setlistOpen, app.game == beforeGame, app.game.Status)
	}

	// Clicking the trigger again toggles the popover closed.
	if err := tt.Click("歌单"); err != nil {
		t.Fatalf("open setlist for toggle test: %v", err)
	}
	if !app.setlistOpen {
		t.Fatal("setlist did not open on trigger click")
	}
	if err := tt.Click("歌单"); err != nil {
		t.Fatalf("close setlist by clicking trigger again: %v", err)
	}
	if app.setlistOpen {
		t.Fatal("clicking setlist trigger again should close popover")
	}

	// Escape dismisses an open popover.
	if err := tt.Click("歌单"); err != nil {
		t.Fatalf("open setlist for Escape test: %v", err)
	}
	tt.Key(0, ui.KeyEscape)
	if app.setlistOpen {
		t.Fatal("Escape should dismiss the setlist popover")
	}

	// Clicking outside the popover dismisses it.
	if err := tt.Click("歌单"); err != nil {
		t.Fatalf("open setlist for outside-click test: %v", err)
	}
	tt.ClickAt(1200, 860)
	if app.setlistOpen {
		t.Fatal("clicking outside should dismiss the setlist popover")
	}
}

func TestSetlistPopoverSevenTracksFitDefaultWindow(t *testing.T) {
	orig := Difficulties
	Difficulties = sevenTrackSetlist()
	defer func() { Difficulties = orig }()

	const winW, winH = 1280, 900
	app := newMinesweeperApp(12345)
	tt := ui.NewTester(app.view, winW, winH)

	// Collapsed state: trigger and toolbar toggles stay on one row inside 1280 width.
	triggerRect, ok := tt.Find("歌单")
	if !ok {
		t.Fatal("setlist trigger not found")
	}
	toggleRect, ok := tt.Find("名场面图鉴")
	if !ok {
		t.Fatal("toolbar toggle not found")
	}
	if triggerRect.X < 0 || triggerRect.X+triggerRect.W > winW {
		t.Fatalf("setlist trigger overflows 1280 width: %+v", triggerRect)
	}
	if toggleRect.X+toggleRect.W > winW {
		t.Fatalf("toolbar toggle overflows 1280 width: %+v", toggleRect)
	}
	if abs32(triggerRect.Y-toggleRect.Y) > 12 {
		t.Fatalf("toolbar wrapped onto two lines at 1280 width: trigger=%+v toggle=%+v", triggerRect, toggleRect)
	}

	// Expanded state: all 7 tracks render inside the 1280×900 viewport without clipping.
	if err := tt.Click("歌单"); err != nil {
		t.Fatalf("open 7-track setlist: %v", err)
	}
	for i, d := range Difficulties {
		r, ok := tt.Find(d.Song)
		if !ok {
			t.Fatalf("track %d (%q) missing from open popover", i, d.Song)
		}
		if r.W <= 0 || r.H <= 0 || r.X < 0 || r.Y < 0 || r.X+r.W > winW || r.Y+r.H > winH {
			t.Fatalf("track %d (%q) overflows %dx%d viewport: %+v", i, d.Song, winW, winH, r)
		}
	}

	// Select each of the 7 tracks by song name via Tester.
	for i := len(Difficulties) - 1; i >= 0; i-- {
		want := Difficulties[i]
		if !app.setlistOpen {
			if err := tt.Click("歌单"); err != nil {
				t.Fatalf("reopen setlist for track %d (%q): %v", i, want.Song, err)
			}
		}
		if err := tt.Click(want.Song); err != nil {
			t.Fatalf("click track %d (%q): %v", i, want.Song, err)
		}
		if app.diffIdx != i || app.game.Difficulty.ID != want.ID {
			t.Fatalf("after clicking %q got idx=%d id=%q, want idx=%d id=%q", want.Song, app.diffIdx, app.game.Difficulty.ID, i, want.ID)
		}
	}
}

// TestSnapshotSetlistPopover renders the 7-track setlist popover expanded
// in the default 1280×900 viewport for both day and night themes when
// MYGO_SNAPSHOT_DIR is set.
func TestSnapshotSetlistPopover(t *testing.T) {
	dir := os.Getenv("MYGO_SNAPSHOT_DIR")
	if dir == "" {
		t.Skip("set MYGO_SNAPSHOT_DIR to render snapshots")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	orig := Difficulties
	Difficulties = sevenTrackSetlist()
	defer func() { Difficulties = orig }()

	for _, dark := range []bool{false, true} {
		suffix := "day"
		if dark {
			suffix = "night"
		}
		app := stageLiveScene(dark)
		app.setlistOpen = true
		name := fmt.Sprintf("setlist-popover-default-%s.png", suffix)
		writeSnapshot(t, app, 1280, 900, filepath.Join(dir, name))
	}
}
