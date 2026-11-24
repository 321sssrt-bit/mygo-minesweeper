package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

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
	order := make([]int, 0, len(Difficulties))
	for i := 1; i < len(Difficulties); i++ {
		order = append(order, i)
	}
	order = append(order, 0)
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

	// Clicking outside the popover dismisses it.
	if err := tt.Click("歌单"); err != nil {
		t.Fatalf("open setlist for outside-click test: %v", err)
	}
	tt.ClickAt(1200, 860)
	if app.setlistOpen {
		t.Fatal("clicking outside should dismiss the setlist popover")
	}
}

func TestSetlistPopoverOutsideClickDoesNotHitBoard(t *testing.T) {
	// Classic board: clicking an unrevealed mine cell outside the open popover
	// must only dismiss the popover without revealing or detonating the cell.
	app := newMinesweeperApp(12345)
	tt := ui.NewTester(app.view, 1280, 900)
	if err := tt.Click("cell-5-5"); err != nil {
		t.Fatalf("start classic game: %v", err)
	}
	targetRect, ok := tt.Find("cell-3-7")
	if !ok {
		t.Fatal("cell-3-7 not found")
	}
	movesBefore := app.game.Moves
	flagsBefore := app.game.FlagsPlaced
	statusBefore := app.game.Status

	if err := tt.Click("歌单"); err != nil {
		t.Fatalf("open setlist popover: %v", err)
	}
	tt.ClickAt(targetRect.X+targetRect.W/2, targetRect.Y+targetRect.H/2)
	if app.setlistOpen {
		t.Fatal("clicking outside popover on classic cell should dismiss popover")
	}
	if app.game.Moves != movesBefore || app.game.Status != statusBefore || app.game.Cell(3, 7).Revealed {
		t.Fatalf("dismiss click leaked onto classic cell (3,7): moves=%d->%d status=%v->%v revealed=%v",
			movesBefore, app.game.Moves, statusBefore, app.game.Status, app.game.Cell(3, 7).Revealed)
	}

	// Right-clicking outside on a cell must also dismiss without placing a flag.
	if err := tt.Click("歌单"); err != nil {
		t.Fatalf("reopen setlist popover for right-click test: %v", err)
	}
	tt.RightClickAt(targetRect.X+targetRect.W/2, targetRect.Y+targetRect.H/2)
	if app.setlistOpen || app.game.FlagsPlaced != flagsBefore {
		t.Fatalf("right-click dismiss leaked onto classic board: open=%v flags=%d->%d",
			app.setlistOpen, flagsBefore, app.game.FlagsPlaced)
	}

	// Duel canvas: clicking outside on the duel board must dismiss without revealing.
	duelApp := newMinesweeperApp(42)
	duelApp.diffIdx = len(Difficulties) - 1
	duelApp.resetGame(false)
	dtt := ui.NewTester(duelApp.view, 1280, 900)
	dtt.Frame()
	boardRect, ok := dtt.Find("决斗棋盘")
	if !ok {
		t.Fatal("duel board not found")
	}
	if err := dtt.Click("歌单"); err != nil {
		t.Fatalf("open setlist in duel: %v", err)
	}
	dtt.ClickAt(boardRect.X+boardRect.W*0.75, boardRect.Y+boardRect.H*0.5)
	if duelApp.setlistOpen || duelApp.game.FirstClick || duelApp.game.Moves != 0 {
		t.Fatalf("dismiss click leaked onto duel board: open=%v firstClick=%v moves=%d",
			duelApp.setlistOpen, duelApp.game.FirstClick, duelApp.game.Moves)
	}
}

func TestSetlistAccentContrastMeetsWCAGAA(t *testing.T) {
	for _, dark := range []bool{false, true} {
		app := newMinesweeperApp(1)
		app.darkMode = dark
		pal := app.paper()
		theme := app.activeTheme()
		cases := []struct {
			name string
			bg   ui.Color
			base ui.Color
		}{
			{"trigger-idle", pal.Slip.Alpha(0.45).Over(theme.Surface), bandColor},
			{"trigger-hover", pal.Slip.Alpha(0.85).Over(theme.Surface), bandColor},
			{"trigger-open", theme.Accent.Alpha(0.14).Over(theme.Surface), bandColor},
			{"popover-selected", theme.Accent.Alpha(0.14).Over(theme.Surface), bandColor},
			{"popover-group-header", theme.Surface, bandColor},
			{"popover-item-idle-muted", theme.Surface, theme.TextMuted},
			{"popover-item-hover-muted", pal.Slip.Alpha(0.6).Over(theme.Surface), theme.TextMuted},
			{"popover-item-selected-muted", theme.Accent.Alpha(0.14).Over(theme.Surface), theme.TextMuted},
		}
		for _, tc := range cases {
			ink := setlistInk(pal, tc.bg, tc.base)
			ratio := contrast(ink, tc.bg)
			if ratio < minInkContrast {
				t.Errorf("dark=%v %s: contrast %.2f < %.1f", dark, tc.name, ratio, minInkContrast)
			}
		}
	}
}

func TestSetlistPopoverSevenTracksFitDefaultWindow(t *testing.T) {
	if len(Difficulties) != 7 {
		t.Fatalf("shipped Difficulties count = %d, want 7", len(Difficulties))
	}

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

	// Expanded state: classic & duel group headers, specs, and all 7 tracks
	// render inside the 1280×900 viewport without clipping.
	if err := tt.Click("歌单"); err != nil {
		t.Fatalf("open 7-track setlist: %v", err)
	}
	classicHdr, ok := tt.Find("经典")
	if !ok {
		t.Fatal("classic group header 「经典」 missing from open popover")
	}
	duelHdr, ok := tt.Find("决斗")
	if !ok {
		t.Fatal("duel group header 「决斗」 missing from open popover")
	}
	for name, r := range map[string]ui.Rect{"经典": classicHdr, "决斗": duelHdr} {
		if r.W <= 0 || r.H <= 0 || r.X < 0 || r.Y < 0 || r.X+r.W > winW || r.Y+r.H > winH {
			t.Fatalf("group header %q overflows %dx%d viewport: %+v", name, winW, winH, r)
		}
	}
	if classicHdr.Y >= duelHdr.Y {
		t.Fatalf("expected classic group header above duel group header: classic=%+v duel=%+v", classicHdr, duelHdr)
	}
	for i, d := range Difficulties {
		r, ok := tt.Find(d.Song)
		if !ok {
			t.Fatalf("track %d (%q) missing from open popover", i, d.Song)
		}
		if r.W <= 0 || r.H <= 0 || r.X < 0 || r.Y < 0 || r.X+r.W > winW || r.Y+r.H > winH {
			t.Fatalf("track %d (%q) overflows %dx%d viewport: %+v", i, d.Song, winW, winH, r)
		}
		if d.Gameplay == GameplayDuel {
			if r.Y <= duelHdr.Y {
				t.Fatalf("duel track %q should appear below duel header: track=%+v duelHdr=%+v", d.Song, r, duelHdr)
			}
		} else {
			if r.Y <= classicHdr.Y || r.Y >= duelHdr.Y {
				t.Fatalf("classic track %q should appear between classic and duel headers: track=%+v classicHdr=%+v duelHdr=%+v", d.Song, r, classicHdr, duelHdr)
			}
		}
		wantSpec := setlistTrackSpec(d)
		specRect, ok := tt.Find(wantSpec)
		if !ok {
			t.Fatalf("track %d (%q) missing spec text %q in open popover", i, d.Song, wantSpec)
		}
		if specRect.X < r.X || specRect.Y < r.Y || specRect.X+specRect.W > r.X+r.W || specRect.Y+specRect.H > r.Y+r.H {
			t.Fatalf("track %d (%q) spec %q rect %+v lies outside track row %+v", i, d.Song, wantSpec, specRect, r)
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

func TestSetlistGroupsFromGameplayFieldAndDisplaysSpecs(t *testing.T) {
	// Misleading IDs, labels, songs, and interleaved order prove grouping
	// follows Difficulty.Gameplay rather than track IDs, names, or positions,
	// and specs come from Cols/Rows/Mines even when Subtitle is empty.
	interleaved := []Difficulty{
		{ID: "easy", Label: "伪装经典", Song: "曲目一", Gameplay: GameplayDuel, Rows: 500, Cols: 500, Mines: 45000, DuelDuration: 180, AIInterval: 1.0},
		{ID: "duel", Label: "伪装决斗", Song: "曲目二", Gameplay: GameplayClassic, Rows: 16, Cols: 30, Mines: 99},
		{ID: "normal", Label: "第二决斗", Song: "曲目三", Gameplay: GameplayDuel, Rows: 1000, Cols: 1000, Mines: 180000, DuelDuration: 300, AIInterval: 0.72},
		{ID: "hard", Label: "默认玩法", Song: "曲目四", Rows: 9, Cols: 9, Mines: 10},
	}

	groups := groupSetlistTracks(interleaved)
	if len(groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(groups))
	}
	if groups[0].Gameplay != GameplayClassic || groups[0].Title != "经典" || len(groups[0].Entries) != 2 {
		t.Fatalf("unexpected classic group: %+v", groups[0])
	}
	if groups[0].Entries[0].Index != 1 || groups[0].Entries[1].Index != 3 {
		t.Fatalf("classic entries lost original indices: %+v", groups[0].Entries)
	}
	if groups[1].Gameplay != GameplayDuel || groups[1].Title != "决斗" || len(groups[1].Entries) != 2 {
		t.Fatalf("unexpected duel group: %+v", groups[1])
	}
	if groups[1].Entries[0].Index != 0 || groups[1].Entries[1].Index != 2 {
		t.Fatalf("duel entries lost original indices: %+v", groups[1].Entries)
	}

	// Single-gameplay slice omits the empty group.
	onlyClassic := groupSetlistTracks([]Difficulty{interleaved[1]})
	if len(onlyClassic) != 1 || onlyClassic[0].Gameplay != GameplayClassic {
		t.Fatalf("expected only classic group, got %+v", onlyClassic)
	}
	onlyDuel := groupSetlistTracks([]Difficulty{interleaved[0]})
	if len(onlyDuel) != 1 || onlyDuel[0].Gameplay != GameplayDuel {
		t.Fatalf("expected only duel group, got %+v", onlyDuel)
	}

	// Verify headless UI renders tracks under their Gameplay group, shows
	// Cols×Rows and grouped Mines, and maps clicks back to original indices.
	orig := Difficulties
	Difficulties = interleaved
	defer func() { Difficulties = orig }()

	app := newMinesweeperApp(12345)
	tt := ui.NewTester(app.view, 1280, 900)
	if err := tt.Click("歌单"); err != nil {
		t.Fatalf("open interleaved setlist: %v", err)
	}
	classicHdr, ok := tt.Find("经典")
	if !ok {
		t.Fatal("classic header missing")
	}
	duelHdr, ok := tt.Find("决斗")
	if !ok {
		t.Fatal("duel header missing")
	}
	wantSpecs := map[string]string{
		"曲目一": "500×500 · 45,000 雷",
		"曲目二": "30×16 · 99 雷",
		"曲目三": "1000×1000 · 180,000 雷",
		"曲目四": "9×9 · 10 雷",
	}
	for _, d := range interleaved {
		r, ok := tt.Find(d.Song)
		if !ok {
			t.Fatalf("track %q not found in popover", d.Song)
		}
		if d.Gameplay == GameplayDuel {
			if r.Y <= duelHdr.Y {
				t.Fatalf("duel track %q rendered above duel header: track=%+v duelHdr=%+v", d.Song, r, duelHdr)
			}
		} else {
			if r.Y <= classicHdr.Y || r.Y >= duelHdr.Y {
				t.Fatalf("classic track %q rendered outside classic group: track=%+v classicHdr=%+v duelHdr=%+v", d.Song, r, classicHdr, duelHdr)
			}
		}
		specRect, ok := tt.Find(wantSpecs[d.Song])
		if !ok {
			t.Fatalf("missing spec %q for %q", wantSpecs[d.Song], d.Song)
		}
		if specRect.X < r.X || specRect.Y < r.Y || specRect.X+specRect.W > r.X+r.W || specRect.Y+specRect.H > r.Y+r.H {
			t.Fatalf("spec %q rect %+v lies outside track row %q %+v", wantSpecs[d.Song], specRect, d.Song, r)
		}
	}

	// Clicking reordered classic track "曲目二" must select original index 1.
	if err := tt.Click("曲目二"); err != nil {
		t.Fatalf("click 曲目二: %v", err)
	}
	if app.diffIdx != 1 || app.game.Difficulty.Song != "曲目二" {
		t.Fatalf("after clicking 曲目二 got idx=%d song=%q, want idx=1", app.diffIdx, app.game.Difficulty.Song)
	}
	// Clicking reordered duel track "曲目三" must select original index 2.
	if err := tt.Click("歌单"); err != nil {
		t.Fatalf("reopen setlist: %v", err)
	}
	if err := tt.Click("曲目三"); err != nil {
		t.Fatalf("click 曲目三: %v", err)
	}
	if app.diffIdx != 2 || app.game.Difficulty.Song != "曲目三" {
		t.Fatalf("after clicking 曲目三 got idx=%d song=%q, want idx=2", app.diffIdx, app.game.Difficulty.Song)
	}
}

func focusSetlistTriggerByTab(t *testing.T, tt *ui.Tester) {
	t.Helper()
	for range 20 {
		if tt.Focused("歌单") {
			return
		}
		tt.Key(0, ui.KeyTab)
	}
	t.Fatal("failed to reach setlist trigger via Tab")
}

func TestSetlistKeyboardNavigationAndSelection(t *testing.T) {
	var displayOrder []setlistEntry
	for _, group := range groupSetlistTracks(Difficulties) {
		displayOrder = append(displayOrder, group.Entries...)
	}
	if len(displayOrder) < 3 {
		t.Fatalf("expected at least 3 tracks in Difficulties, got %d", len(displayOrder))
	}
	first := displayOrder[0]
	second := displayOrder[1]
	penultimate := displayOrder[len(displayOrder)-2]
	last := displayOrder[len(displayOrder)-1]

	app := newMinesweeperApp(12345)
	tt := ui.NewTester(app.view, 1280, 900)

	// Reach trigger via Tab and open with Enter; focus lands on active track.
	focusSetlistTriggerByTab(t, tt)
	tt.Key(0, ui.KeyEnter)
	if !app.setlistOpen {
		t.Fatal("Enter on focused setlist trigger should open popover")
	}
	if !tt.Focused(first.Diff.Song) {
		t.Fatalf("opening setlist with Enter should focus the active track %q", first.Diff.Song)
	}

	// ArrowDown traverses all tracks in display order across groups and wraps around.
	for step := 1; step <= len(displayOrder); step++ {
		want := displayOrder[step%len(displayOrder)].Diff.Song
		tt.Key(0, ui.KeyDown)
		if !tt.Focused(want) {
			t.Fatalf("ArrowDown step %d did not focus %q", step, want)
		}
	}

	// ArrowUp wraps backwards from first to last, then steps up.
	tt.Key(0, ui.KeyUp)
	if !tt.Focused(last.Diff.Song) {
		t.Fatalf("ArrowUp from first track should wrap to %q", last.Diff.Song)
	}
	tt.Key(0, ui.KeyUp)
	if !tt.Focused(penultimate.Diff.Song) {
		t.Fatalf("ArrowUp from %q should focus %q", last.Diff.Song, penultimate.Diff.Song)
	}

	// Home and End jump to first and last track.
	tt.Key(0, ui.KeyHome)
	if !tt.Focused(first.Diff.Song) {
		t.Fatalf("Home should focus first track %q", first.Diff.Song)
	}
	tt.Key(0, ui.KeyEnd)
	if !tt.Focused(last.Diff.Song) {
		t.Fatalf("End should focus last track %q", last.Diff.Song)
	}

	// Move to second track and confirm with Enter; closes popover and returns focus to trigger.
	tt.Key(0, ui.KeyHome)
	tt.Key(0, ui.KeyDown)
	if !tt.Focused(second.Diff.Song) {
		t.Fatalf("expected focus on %q before Enter", second.Diff.Song)
	}
	tt.Key(0, ui.KeyEnter)
	if app.setlistOpen {
		t.Fatal("Enter on option should close setlist popover")
	}
	if app.diffIdx != second.Index || app.game.Difficulty.Song != second.Diff.Song {
		t.Fatalf("Enter selected idx=%d song=%q, want %d / %q", app.diffIdx, app.game.Difficulty.Song, second.Index, second.Diff.Song)
	}
	if !tt.Focused("歌单") {
		t.Fatal("selecting a track via Enter should return focus to setlist trigger")
	}

	// Space opens popover on the newly active track, and Space selects the last track.
	tt.Key(0, ui.KeySpace)
	if !app.setlistOpen || !tt.Focused(second.Diff.Song) {
		t.Fatalf("Space should open setlist focused on active %q: open=%v focused=%v", second.Diff.Song, app.setlistOpen, tt.Focused(second.Diff.Song))
	}
	tt.Key(0, ui.KeyEnd)
	tt.Key(0, ui.KeySpace)
	if app.setlistOpen {
		t.Fatal("Space on option should close setlist popover")
	}
	if app.diffIdx != last.Index || (last.Diff.Gameplay == GameplayDuel) != app.isDuel() || app.game.Difficulty.Song != last.Diff.Song {
		t.Fatalf("Space failed to select %q: idx=%d duel=%v song=%q", last.Diff.Song, app.diffIdx, app.isDuel(), app.game.Difficulty.Song)
	}
	if !tt.Focused("歌单") {
		t.Fatal("selecting a track via Space should return focus to setlist trigger")
	}

	// ArrowDown and ArrowUp on the closed trigger open the popover on the active track,
	// and Escape dismisses it without changing track while returning focus to trigger.
	for _, key := range []ui.Key{ui.KeyDown, ui.KeyUp} {
		tt.Key(0, key)
		if !app.setlistOpen || !tt.Focused(last.Diff.Song) {
			t.Fatalf("key %v on closed trigger did not open focused on %q: open=%v focused=%v", key, last.Diff.Song, app.setlistOpen, tt.Focused(last.Diff.Song))
		}
		tt.Key(0, ui.KeyUp)
		if !tt.Focused(penultimate.Diff.Song) {
			t.Fatalf("expected focus on %q before Escape", penultimate.Diff.Song)
		}
		tt.Key(0, ui.KeyEscape)
		if app.setlistOpen {
			t.Fatal("Escape should close setlist popover")
		}
		if app.game.Difficulty.Song != last.Diff.Song {
			t.Fatalf("Escape changed active track to %q", app.game.Difficulty.Song)
		}
		if !tt.Focused("歌单") {
			t.Fatal("Escape should return focus to setlist trigger")
		}
	}

	// Re-selecting the active track via Enter during active play does not reset the game.
	tt.Key(0, ui.KeyDown)
	tt.Key(0, ui.KeyHome)
	tt.Key(0, ui.KeyEnter)
	if app.diffIdx != first.Index {
		t.Fatalf("expected reset to track %d, got %d", first.Index, app.diffIdx)
	}
	if err := tt.Click("cell-4-4"); err != nil {
		t.Fatalf("start classic game: %v", err)
	}
	beforeGame := app.game
	focusSetlistTriggerByTab(t, tt)
	tt.Key(0, ui.KeyEnter)
	tt.Key(0, ui.KeyEnter)
	if app.setlistOpen || app.game != beforeGame || app.game.Status != StatusPlaying {
		t.Fatalf("re-selecting active track via Enter reset game: open=%v sameGame=%v status=%v", app.setlistOpen, app.game == beforeGame, app.game.Status)
	}
	if !tt.Focused("歌单") {
		t.Fatal("re-selecting active track via Enter should return focus to trigger")
	}

	// Pressing Tab inside the open popover moves focus out and unmounts the popover.
	tt.Key(0, ui.KeyEnter)
	if !app.setlistOpen {
		t.Fatal("expected setlist open before Tab")
	}
	tt.Key(0, ui.KeyTab)
	if app.setlistOpen {
		t.Fatal("Tab leaving setlist popover should close it")
	}
	if _, ok := tt.Find(second.Diff.Song); ok {
		t.Fatal("popover options should be unmounted immediately after Tab closes popover")
	}
	if app.game != beforeGame || app.game.Status != StatusPlaying {
		t.Fatal("Tab closing setlist should preserve active game")
	}

	// Opening with mouse click still allows immediate keyboard navigation and selection,
	// and clicking non-button headers/whitespace inside the popover keeps it open.
	if err := tt.Click("歌单"); err != nil {
		t.Fatalf("open setlist with click: %v", err)
	}
	for _, header := range []string{"排练歌单", "决斗", "经典"} {
		info, ok := tt.AccessByRole(ui.RoleText, header)
		if !ok {
			t.Fatalf("popover header text %q not found", header)
		}
		r := info.Bounds
		tt.ClickAt(r.X+r.W/2, r.Y+r.H/2)
		if !app.setlistOpen {
			t.Fatalf("clicking internal popover header %q (rect %+v) should keep setlist open", header, r)
		}
	}
	// Clicking internal whitespace beside the header also keeps the popover open.
	if hdr, ok := tt.AccessByRole(ui.RoleText, "排练歌单"); ok {
		r := hdr.Bounds
		tt.ClickAt(r.X+r.W+40, r.Y+r.H/2)
		if !app.setlistOpen {
			t.Fatal("clicking internal popover whitespace should keep setlist open")
		}
	}
	// Arrow navigation still works after clicking non-button areas inside the popover.
	if !tt.Focused(first.Diff.Song) {
		t.Fatalf("expected focus on %q after clicking 经典 header and whitespace", first.Diff.Song)
	}
	tt.Key(0, ui.KeyDown)
	tt.Key(0, ui.KeyEnter)
	if app.setlistOpen || app.diffIdx != second.Index || app.game.Difficulty.Song != second.Diff.Song {
		t.Fatalf("mouse-open + header clicks + ArrowDown + Enter failed: open=%v idx=%d song=%q", app.setlistOpen, app.diffIdx, app.game.Difficulty.Song)
	}

	// Interleaved Gameplay order: arrow navigation follows grouped display order
	// and Enter selects the underlying index in Difficulties.
	orig := Difficulties
	Difficulties = []Difficulty{
		{ID: "d1", Song: "曲目一", Gameplay: GameplayDuel, Rows: 500, Cols: 500, Mines: 45000, DuelDuration: 180, AIInterval: 1.0},
		{ID: "c1", Song: "曲目二", Gameplay: GameplayClassic, Rows: 9, Cols: 9, Mines: 10},
		{ID: "c2", Song: "曲目三", Gameplay: GameplayClassic, Rows: 16, Cols: 16, Mines: 40},
	}
	defer func() { Difficulties = orig }()

	app2 := newMinesweeperApp(12345)
	tt2 := ui.NewTester(app2.view, 1280, 900)
	focusSetlistTriggerByTab(t, tt2)
	tt2.Key(0, ui.KeyEnter)
	if !tt2.Focused("曲目一") {
		t.Fatal("expected focus on initial active track 曲目一 (duel)")
	}
	// Display order is 曲目二 (classic), 曲目三 (classic), 曲目一 (duel).
	// ArrowDown from last visual item (曲目一) wraps to first visual item (曲目二).
	tt2.Key(0, ui.KeyDown)
	if !tt2.Focused("曲目二") {
		t.Fatal("expected ArrowDown wrap to first grouped track 曲目二")
	}
	tt2.Key(0, ui.KeyDown)
	if !tt2.Focused("曲目三") {
		t.Fatal("expected ArrowDown to second grouped track 曲目三")
	}
	tt2.Key(0, ui.KeyEnter)
	if app2.diffIdx != 2 || app2.game.Difficulty.Song != "曲目三" {
		t.Fatalf("selected idx=%d song=%q, want 2 / 曲目三", app2.diffIdx, app2.game.Difficulty.Song)
	}
}

func TestSetlistScreenReaderSemantics(t *testing.T) {
	app := newMinesweeperApp(12345)
	tt := ui.NewTester(app.view, 1280, 900)
	tt.UseScreenReader()

	// Collapsed trigger exposes name, current song name as value, popup kind, and collapsed state.
	trig, ok := tt.Access("歌单")
	if !ok {
		t.Fatal("setlist trigger missing from accessibility tree")
	}
	if trig.Name != "歌单" || trig.Value != "迷星叫" || trig.HasPopup != ui.PopupListBox || trig.Expanded || !trig.Focusable {
		t.Fatalf("unexpected collapsed trigger AccessInfo: %+v", trig)
	}

	// Open with keyboard and verify expanded listbox, groups, and options.
	focusSetlistTriggerByTab(t, tt)
	tt.Key(0, ui.KeyEnter)

	trigOpen, ok := tt.Access("歌单")
	if !ok || !trigOpen.Expanded || trigOpen.Value != "迷星叫" {
		t.Fatalf("unexpected expanded trigger AccessInfo: %+v", trigOpen)
	}
	if len(trigOpen.Controls) == 0 || trigOpen.Controls[0] != "排练歌单" {
		t.Fatalf("expected trigger Controls to include 排练歌单, got %v", trigOpen.Controls)
	}

	if _, ok := tt.AccessByRole(ui.RoleListBox, "排练歌单"); !ok {
		t.Fatal("setlist listbox (RoleListBox) missing from accessibility tree")
	}
	for _, groupTitle := range []string{"经典", "决斗"} {
		if _, ok := tt.AccessByRole(ui.RoleGroup, groupTitle); !ok {
			t.Fatalf("setlist group %q (RoleGroup) missing from accessibility tree", groupTitle)
		}
	}

	groups := groupSetlistTracks(Difficulties)
	trackNo := 0
	for _, group := range groups {
		for _, entry := range group.Entries {
			trackNo++
			d := entry.Diff
			opt, ok := tt.AccessByRole(ui.RoleOption, d.Song)
			if !ok {
				t.Fatalf("track option %q (RoleOption) missing from accessibility tree", d.Song)
			}
			wantSelected := entry.Index == app.diffIdx
			if opt.Name != d.Song || opt.Description != app.setlistTrackDescription(d) ||
				opt.Selected != wantSelected || opt.PosInSet != trackNo || opt.SetSize != len(Difficulties) || !opt.Focusable {
				t.Fatalf("unexpected option AccessInfo for %q: %+v", d.Song, opt)
			}
			if opt.Focused != wantSelected {
				t.Fatalf("option %q Focused=%v, want %v", d.Song, opt.Focused, wantSelected)
			}
		}
	}

	// Moving focus updates Focused option in the accessibility tree.
	tt.Key(0, ui.KeyDown)
	opt1, ok := tt.AccessByRole(ui.RoleOption, "碧天伴走")
	if !ok || !opt1.Focused {
		t.Fatalf("expected 碧天伴走 option focused in accessibility tree: ok=%v info=%+v", ok, opt1)
	}
	opt0, ok := tt.AccessByRole(ui.RoleOption, "迷星叫")
	if !ok || opt0.Focused {
		t.Fatalf("expected 迷星叫 option unfocused after ArrowDown: ok=%v info=%+v", ok, opt0)
	}

	// Selecting updates trigger value to the newly chosen song name.
	tt.Key(0, ui.KeyEnter)
	trigAfter, ok := tt.Access("歌单")
	if !ok || trigAfter.Expanded || trigAfter.Value != "碧天伴走" || !trigAfter.Focused {
		t.Fatalf("unexpected trigger AccessInfo after selection: %+v", trigAfter)
	}
}

// TestSetlistLargeTracksPlayOnCanvas selects 50x50 《诗超绊》 and 100x100
// 《迷路日々》 from the shipped setlist popover, verifies both route onto the
// shared classic canvas, drags the bottom-right corner clear of the minimap,
// and plays each through opening, flagging, winning, losing, and resetting via
// headless UI input.
func TestSetlistLargeTracksPlayOnCanvas(t *testing.T) {
	for _, want := range []Difficulty{TrackUtakotoba, TrackMayoiuta} {
		t.Run(want.ID, func(t *testing.T) {
			app := newMinesweeperApp(12345)
			tt := ui.NewTester(app.view, 1280, 900)

			if err := tt.Click("歌单"); err != nil {
				t.Fatalf("open setlist for %s: %v", want.Song, err)
			}
			if err := tt.Click(want.Song); err != nil {
				t.Fatalf("select %s from setlist: %v", want.Song, err)
			}
			if app.game.Difficulty != want || app.isDuel() {
				t.Fatalf("active track = %+v duel=%v, want classic %+v", app.game.Difficulty, app.isDuel(), want)
			}
			// Re-pin deterministic seed after setlist selection (selectDifficulty calls resetGame(false)).
			app.seed = 12345
			app.resetGame(true)
			tt.Frame()

			boardRect, ok := tt.Find("经典棋盘")
			if !ok || boardRect.W <= 0 || boardRect.H <= 0 {
				t.Fatalf("%s: classic canvas missing: %+v ok=%v", want.ID, boardRect, ok)
			}
			if _, hasGrid := tt.Find("cell-0-0"); hasGrid {
				t.Fatalf("%s: rendered button grid instead of classic canvas", want.ID)
			}

			cellPoint := func(row, col int) (float32, float32) {
				v := app.board
				return boardRect.X + (float32(col)+0.5)*v.Cell - v.X,
					boardRect.Y + (float32(row)+0.5)*v.Cell - v.Y
			}

			// First click on the canvas starts the game safely.
			tt.ClickAt(boardRect.X+boardRect.W/2, boardRect.Y+boardRect.H/2)
			if !app.game.FirstClick || app.game.Status != StatusPlaying || app.game.RevealedSafe == 0 {
				t.Fatalf("%s: canvas first click did not start game: first=%v status=%v safe=%d",
					want.ID, app.game.FirstClick, app.game.Status, app.game.RevealedSafe)
			}

			// Drag the canvas to its bottom-right limit so the corner cell clears the minimap.
			mini := app.board.minimapBoundsFor(app.game)
			maxX, maxY := app.board.panLimit(app.game)
			for range 12 {
				if app.board.X >= maxX-0.5 && app.board.Y >= maxY-0.5 {
					break
				}
				sx, sy := boardRect.X+mini.X-20, boardRect.Y+mini.Y-20
				tt.PressButton(2, sx, sy)
				tt.Move(boardRect.X+20, boardRect.Y+20)
				tt.ReleaseButton(2, boardRect.X+20, boardRect.Y+20)
			}
			cornerRow, cornerCol := want.Rows-1, want.Cols-1
			fx, fy := cellPoint(cornerRow, cornerCol)
			if app.board.minimapFor(app.game, fx-boardRect.X, fy-boardRect.Y) {
				t.Fatalf("%s: dragged bottom-right corner (%d,%d) still occluded by minimap", want.ID, cornerRow, cornerCol)
			}
			// Right-click the bottom-right corner on the canvas to place and remove a flag.
			tt.RightClickAt(fx, fy)
			if !app.game.Cell(cornerRow, cornerCol).Flagged || app.game.FlagsPlaced != 1 {
				t.Fatalf("%s: right-click on canvas failed to flag (%d,%d): flagged=%v total=%d",
					want.ID, cornerRow, cornerCol, app.game.Cell(cornerRow, cornerCol).Flagged, app.game.FlagsPlaced)
			}
			tt.RightClickAt(fx, fy)
			if app.game.Cell(cornerRow, cornerCol).Flagged || app.game.FlagsPlaced != 0 {
				t.Fatalf("%s: second right-click on canvas failed to unflag (%d,%d)", want.ID, cornerRow, cornerCol)
			}

			// Clear all zero-adjacent regions first, then all remaining numbered
			// safe cells except the bottom-rightmost one (numbered cells do not
			// flood-fill), and click that final safe cell on the canvas to win.
			for i := range app.game.Cells {
				if c := &app.game.Cells[i]; !c.IsMine && !c.Revealed && c.Adjacent == 0 {
					app.game.Reveal(c.Row, c.Col)
				}
			}
			var remainingSafe []int
			for i := range app.game.Cells {
				if c := &app.game.Cells[i]; !c.IsMine && !c.Revealed {
					remainingSafe = append(remainingSafe, i)
				}
			}
			if len(remainingSafe) == 0 || app.game.Status != StatusPlaying {
				t.Fatalf("%s: expected remaining numbered safe cells while playing: rem=%d status=%v",
					want.ID, len(remainingSafe), app.game.Status)
			}
			for _, idx := range remainingSafe[:len(remainingSafe)-1] {
				c := &app.game.Cells[idx]
				app.game.Reveal(c.Row, c.Col)
			}
			last := &app.game.Cells[remainingSafe[len(remainingSafe)-1]]
			if app.game.Status != StatusPlaying || last.Revealed {
				t.Fatalf("%s: expected one final safe cell while playing: last=(%d,%d) revealed=%v status=%v",
					want.ID, last.Row, last.Col, last.Revealed, app.game.Status)
			}
			app.board.center(app.game, last.Row, last.Col)
			tt.Frame()
			wx, wy := cellPoint(last.Row, last.Col)
			tt.ClickAt(wx, wy)
			if app.game.Status != StatusWon || app.game.FlagsPlaced != want.Mines {
				t.Fatalf("%s: clicking final safe cell (%d,%d) on canvas did not win: status=%v flags=%d/%d",
					want.ID, last.Row, last.Col, app.game.Status, app.game.FlagsPlaced, want.Mines)
			}

			// Reset with R, re-pin seed, reopen via canvas, then hit a mine on the canvas to verify ordinary loss.
			tt.Key(0, ui.KeyR)
			if app.game.Status != StatusIdle || app.game.Difficulty != want {
				t.Fatalf("%s: KeyR failed to reset track: status=%v diff=%+v", want.ID, app.game.Status, app.game.Difficulty)
			}
			app.seed = 12345
			app.resetGame(true)
			tt.Frame()
			tt.ClickAt(boardRect.X+boardRect.W/2, boardRect.Y+boardRect.H/2)
			mineRow, mineCol := -1, -1
			for i := range app.game.Cells {
				if c := &app.game.Cells[i]; c.IsMine && !c.Revealed && !c.Flagged {
					mineRow, mineCol = c.Row, c.Col
					break
				}
			}
			if mineRow < 0 {
				t.Fatalf("%s: no mine found after reopen", want.ID)
			}
			app.board.center(app.game, mineRow, mineCol)
			tt.Frame()
			mx, my := cellPoint(mineRow, mineCol)
			tt.ClickAt(mx, my)
			if app.game.Status != StatusLost || app.haruhikageIntro || app.haruhikageAlert {
				t.Fatalf("%s: mine click on canvas did not produce plain loss: status=%v intro=%v alert=%v",
					want.ID, app.game.Status, app.haruhikageIntro, app.haruhikageAlert)
			}
		})
	}
}

// TestSnapshotSetlistPopover renders the shipped 7-track setlist popover
// expanded in the default 1280×900 viewport for both day and night themes when
// MYGO_SNAPSHOT_DIR is set.
func TestSnapshotSetlistPopover(t *testing.T) {
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
		app := stageLiveScene(dark)
		app.setlistOpen = true
		name := fmt.Sprintf("setlist-popover-default-%s.png", suffix)
		writeSnapshot(t, app, false, 1280, 900, filepath.Join(dir, name))
	}
}

// TestSetlistShowsBestRecordPerClassicTrack clears 《迷星叫》 through the
// headless grid, then reopens the setlist from another track and reads the
// new record on 《迷星叫》's entry while unplayed classic tracks keep the
// placeholder and duel tracks show no record at all.
func TestSetlistShowsBestRecordPerClassicTrack(t *testing.T) {
	app := newMinesweeperApp(7)
	tt := ui.NewTester(app.view, 1280, 900)
	tt.UseScreenReader()
	easy := Difficulties[0]

	checkEntries := func(phase string, recorded map[string]string) {
		t.Helper()
		for _, d := range Difficulties {
			opt, ok := tt.AccessByRole(ui.RoleOption, d.Song)
			if !ok {
				t.Fatalf("%s: option %q missing", phase, d.Song)
			}
			if d.Gameplay == GameplayDuel {
				if opt.Description != setlistTrackSpec(d) {
					t.Fatalf("%s: duel option %q description = %q, want spec only", phase, d.Song, opt.Description)
				}
				continue
			}
			want := "最快 --"
			if rec, ok := recorded[d.ID]; ok {
				want = rec
			}
			if !strings.HasSuffix(opt.Description, " · "+want) {
				t.Fatalf("%s: option %q description = %q, want suffix %q", phase, d.Song, opt.Description, want)
			}
		}
	}

	if err := tt.Click("歌单"); err != nil {
		t.Fatalf("open setlist: %v", err)
	}
	checkEntries("before any clear", nil)
	if err := tt.Click(easy.Song); err != nil {
		t.Fatalf("close setlist on current track: %v", err)
	}

	if err := tt.Click("cell-4-4"); err != nil {
		t.Fatalf("start 《%s》: %v", easy.Song, err)
	}
	var remainingSafe []int
	for i := range app.game.Cells {
		if c := &app.game.Cells[i]; !c.IsMine && !c.Revealed {
			remainingSafe = append(remainingSafe, i)
		}
	}
	if len(remainingSafe) == 0 || app.game.Status != StatusPlaying {
		t.Fatalf("expected safe cells left while playing: rem=%d status=%v", len(remainingSafe), app.game.Status)
	}
	for _, idx := range remainingSafe[:len(remainingSafe)-1] {
		c := &app.game.Cells[idx]
		app.game.Reveal(c.Row, c.Col)
	}
	last := &app.game.Cells[remainingSafe[len(remainingSafe)-1]]
	tt.Advance(7200 * time.Millisecond)
	if err := tt.Click(fmt.Sprintf("cell-%d-%d", last.Row, last.Col)); err != nil {
		t.Fatalf("click final safe cell: %v", err)
	}
	if app.game.Status != StatusWon {
		t.Fatalf("expected 《%s》 won, got %v", easy.Song, app.game.Status)
	}

	// Switch to another track so the status bar no longer shows 《迷星叫》's
	// record and the only 「最快 7 秒」 on screen is the setlist entry.
	if err := tt.Click("歌单"); err != nil {
		t.Fatalf("reopen setlist: %v", err)
	}
	if err := tt.Click(Difficulties[1].Song); err != nil {
		t.Fatalf("select 《%s》: %v", Difficulties[1].Song, err)
	}
	if err := tt.Click("歌单"); err != nil {
		t.Fatalf("reopen setlist after switching: %v", err)
	}
	checkEntries("after clearing easy", map[string]string{easy.ID: "最快 7 秒"})

	row, ok := tt.Find(easy.Song)
	if !ok {
		t.Fatalf("entry %q missing", easy.Song)
	}
	rec, ok := tt.Find("最快 7 秒")
	if !ok {
		t.Fatal("record 「最快 7 秒」 not visible in the setlist")
	}
	if rec.X < row.X || rec.Y < row.Y || rec.X+rec.W > row.X+row.W || rec.Y+rec.H > row.Y+row.H {
		t.Fatalf("record rect %+v lies outside 《%s》 entry %+v", rec, easy.Song, row)
	}
}
