package main

import (
	"errors"
	"image"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestPrefsMemoryInjectionAndZeroDiskAccess(t *testing.T) {
	// Point user-config env vars at a temp directory and seed prefs.json at
	// the exact path os.UserConfigDir() resolves on this OS, so any accidental
	// UserConfigDir read or write from default/snapshot constructors is caught.
	cfgHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfgHome)
	t.Setenv("APPDATA", cfgHome)
	t.Setenv("HOME", cfgHome)
	cfgRoot, err := os.UserConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(cfgRoot, cfgHome) {
		t.Fatalf("UserConfigDir %q escaped temp dir %q", cfgRoot, cfgHome)
	}
	diskPrefs := prefs{
		DarkMode:      false,
		Motion:        motionReduced,
		SoundEnabled:  false,
		Volume:        12,
		AllowQuestion: true,
	}
	if err := savePrefs(cfgRoot, diskPrefs); err != nil {
		t.Fatal(err)
	}

	// Default constructor ignores disk and uses deterministic in-memory defaults.
	defApp := newMinesweeperApp(42)
	if got := defApp.currentPrefs(); got != defaultPrefs() {
		t.Fatalf("newMinesweeperApp loaded %+v, want deterministic defaults %+v", got, defaultPrefs())
	}
	if defApp.savePrefsFn != nil {
		t.Fatal("newMinesweeperApp must not bind disk persistence")
	}

	// Snapshot stage entrypoints also stay purely in-memory and ignore disk prefs.
	snapApp := stageLiveScene(true)
	if got := snapApp.currentPrefs(); got != defaultPrefs() {
		t.Fatalf("stageLiveScene loaded %+v, want deterministic defaults %+v", got, defaultPrefs())
	}

	// Mutating preferences in headless default app never touches disk.
	beforeBytes, err := os.ReadFile(prefsPath(cfgRoot))
	if err != nil {
		t.Fatal(err)
	}
	ttDef := ui.NewTester(defApp.view, 1280, 900)
	ttDef.Key(0, ui.KeyM)
	if defApp.soundEnabled {
		t.Fatal("expected M to mute default app in memory")
	}
	afterBytes, err := os.ReadFile(prefsPath(cfgRoot))
	if err != nil {
		t.Fatal(err)
	}
	if string(afterBytes) != string(beforeBytes) {
		t.Fatalf("headless default app mutated disk prefs: %s -> %s", beforeBytes, afterBytes)
	}

	// Inject distinct in-memory preferences and verify they take effect in headless UI.
	injected := prefs{
		DarkMode:      false,
		Motion:        motionStandard,
		SoundEnabled:  false,
		Volume:        33,
		AllowQuestion: true,
	}
	app := newMinesweeperAppWith(appOptions{
		Seed:  42,
		Prefs: &injected,
	})
	if got := app.currentPrefs(); got != injected {
		t.Fatalf("injected prefs = %+v, want %+v", got, injected)
	}
	tt := ui.NewTester(app.view, 1280, 900)
	if app.paper().Dark {
		t.Fatal("expected injected DarkMode=false to render light notebook paper")
	}
	// AllowQuestion=true allows right-click cycle: unflagged -> flagged -> questioned.
	if err := tt.RightClick("cell-0-0"); err != nil {
		t.Fatalf("first right click: %v", err)
	}
	if err := tt.RightClick("cell-0-0"); err != nil {
		t.Fatalf("second right click: %v", err)
	}
	if !app.game.Cell(0, 0).Questioned {
		t.Fatal("expected injected AllowQuestion=true to cycle cell-0-0 into questioned state")
	}
	t.Logf("verified injected prefs %+v and zero disk access in default/snapshot constructors", injected)
}

func TestDesktopPrefsLoadSaveAndSaveFailureRetention(t *testing.T) {
	// 1. Desktop startup loads from configDir and atomically saves changes.
	dir := t.TempDir()
	initial := prefs{
		DarkMode:      false,
		Motion:        motionReduced,
		SoundEnabled:  true,
		Volume:        80,
		AllowQuestion: true,
	}
	if err := savePrefs(dir, initial); err != nil {
		t.Fatal(err)
	}
	desktopSound := &soundRecorder{}
	app := newDesktopApp(func() (string, error) { return dir, nil }, savePrefs, desktopSound)
	if got := app.currentPrefs(); got != initial {
		t.Fatalf("desktop startup loaded %+v, want %+v", got, initial)
	}
	tt := ui.NewTester(app.view, 1280, 900)
	tt.UseScreenReader()

	// Toggle DarkMode via the 夜间台灯 switch, AllowQuestion via ? 标记, and mute via M.
	sw, ok := tt.AccessByRole(ui.RoleSwitch, "夜间台灯")
	if !ok {
		t.Fatal("missing 夜间台灯 switch")
	}
	tt.ClickAt(sw.Bounds.X+sw.Bounds.W/2, sw.Bounds.Y+sw.Bounds.H/2)
	if err := tt.Click("? 标记"); err != nil {
		t.Fatalf("click ? 标记: %v", err)
	}
	tt.Key(0, ui.KeyM)
	// The desktop app plays through the output it was given: two switch
	// flips at 80% volume, then the mute itself is silent.
	if got := desktopSound.take(); len(got) != 2 || got[0].ID != SoundToggle || got[1].ID != SoundToggle || got[0].Gain != 0.8 {
		t.Fatalf("desktop app sounds = %+v, want two toggles at gain 0.8", got)
	}

	wantSaved := prefs{
		DarkMode:      true,
		Motion:        motionReduced,
		SoundEnabled:  false,
		Volume:        80,
		AllowQuestion: false,
	}
	if got := app.currentPrefs(); got != wantSaved {
		t.Fatalf("in-memory prefs after UI mutations = %+v, want %+v", got, wantSaved)
	}
	if gotDisk := loadPrefs(dir); gotDisk != wantSaved {
		t.Fatalf("persisted disk prefs = %+v, want %+v", gotDisk, wantSaved)
	}

	// 2. UserConfigDir failure falls back gracefully to defaults without panicking or writing.
	fallbackApp := newDesktopApp(func() (string, error) {
		return "", errors.New("config dir unavailable")
	}, func(string, prefs) error {
		t.Fatal("save must not be called when UserConfigDir fails")
		return nil
	}, nil)
	if _, silent := fallbackApp.sound.(silentSound); !silent {
		t.Fatalf("desktop app without an output plays into %T, want silentSound", fallbackApp.sound)
	}
	if got := fallbackApp.currentPrefs(); got != defaultPrefs() {
		t.Fatalf("fallback desktop prefs = %+v, want %+v", got, defaultPrefs())
	}
	fallbackApp.toggleMute()
	if fallbackApp.soundEnabled {
		t.Fatal("in-memory mute toggle should still work when UserConfigDir is unavailable")
	}

	// 3. Failing save stub: disk error never reverts in-memory session preferences.
	saveErr := errors.New("disk full")
	var attempts []prefs
	failApp := newDesktopApp(func() (string, error) { return dir, nil }, func(gotDir string, p prefs) error {
		if gotDir != dir {
			t.Fatalf("save received dir %q, want %q", gotDir, dir)
		}
		attempts = append(attempts, p)
		return saveErr
	}, nil)
	ttFail := ui.NewTester(failApp.view, 1280, 900)
	ttFail.Key(0, ui.KeyM)
	failApp.updatePrefs(func(p *prefs) {
		p.Volume = 42
		p.Motion = motionStandard
	})
	ttFail.Frame()

	wantRetained := prefs{
		DarkMode:      true,
		Motion:        motionStandard,
		SoundEnabled:  true,
		Volume:        42,
		AllowQuestion: false,
	}
	if got := failApp.currentPrefs(); got != wantRetained {
		t.Fatalf("prefs after failed save = %+v, want retained %+v", got, wantRetained)
	}
	if len(attempts) != 2 || attempts[len(attempts)-1] != wantRetained {
		t.Fatalf("expected 2 save attempts ending in %+v, got %+v", wantRetained, attempts)
	}
	// Disk file still holds the earlier committed snapshot, untouched by failed stub.
	if gotDisk := loadPrefs(dir); gotDisk != wantSaved {
		t.Fatalf("disk file changed despite failing save stub: got %+v, want %+v", gotDisk, wantSaved)
	}
	t.Logf("verified desktop load/save round-trip and retained %+v after %d failed save attempts", wantRetained, len(attempts))
}

func TestMotionThreeStateResolutionBidirectional(t *testing.T) {
	cases := []struct {
		name         string
		mode         motionMode
		systemReduce bool
		wantReduced  bool
	}{
		{"system_follows_off", motionSystem, false, false},
		{"system_follows_on", motionSystem, true, true},
		{"standard_overrides_system_on", motionStandard, true, false},
		{"standard_with_system_off", motionStandard, false, false},
		{"reduced_overrides_system_off", motionReduced, false, true},
		{"reduced_with_system_on", motionReduced, true, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := defaultPrefs()
			p.Motion = tc.mode
			app := newMinesweeperAppWith(appOptions{Seed: 42, Prefs: &p})
			app.memberID = MemberSoyorin
			app.game = NewGame(Difficulty{ID: "easy", Rows: 5, Cols: 5, Mines: 2, SkillCharges: 1}, true, MemberSoyorin, 7)
			app.game.FirstClick = true
			app.game.Status = StatusPlaying
			app.game.Cell(0, 0).IsMine = true
			app.game.Cell(4, 4).IsMine = true
			app.game.RecomputeAdjacency()

			tt := ui.NewTester(app.view, 1280, 900)
			tt.SetPreferences(ui.Preferences{ReduceMotion: tc.systemReduce})
			if app.motionReducedActive != tc.wantReduced {
				t.Fatalf("motionReducedActive=%v, want %v", app.motionReducedActive, tc.wantReduced)
			}

			// Trigger Soyorin shield shake at (0,0), then flood-fill ripple at (2,2).
			if err := tt.Click("cell-0-0"); err != nil {
				t.Fatalf("click cell-0-0: %v", err)
			}
			if err := tt.Click("cell-2-2"); err != nil {
				t.Fatalf("click cell-2-2: %v", err)
			}
			tt.Advance(20 * time.Millisecond)
			sx, sy := app.currentShakeOffset(tt.Now())

			if tc.wantReduced {
				if len(app.ripples) != 0 || sx != 0 || sy != 0 || app.shakeAmplitude != 0 {
					t.Fatalf("expected reduced motion to suppress ripples & shake, got ripples=%d shake=(%v,%v) amp=%v",
						len(app.ripples), sx, sy, app.shakeAmplitude)
				}
			} else {
				if len(app.ripples) == 0 || (sx == 0 && sy == 0) {
					t.Fatalf("expected full motion ripples & shake, got ripples=%d shake=(%v,%v)",
						len(app.ripples), sx, sy)
				}
			}
		})
	}

	// Dynamic transitions mid-animation: switching to reduced immediately settles
	// active ripples and shake on the same frame; forcing standard under system
	// ReduceMotion=true re-enables motion on both classic grid and canvas.
	app := newMinesweeperApp(42)
	app.diffIdx = findDifficultyIdx("duel")
	app.game = NewGame(Difficulty{Gameplay: GameplayDuel, Rows: 8, Cols: 8, Mines: 2, SkillCharges: 1}, true, MemberTomori, 42)
	app.game.FirstClick = true
	app.game.Status = StatusPlaying
	app.game.Cell(3, 3).IsMine = true
	app.game.Cell(4, 4).IsMine = true
	app.game.RecomputeAdjacency()

	tt := ui.NewTester(app.view, 1280, 900)
	app.board.center(app.game, 3, 3)
	tt.Frame()
	boardRect, ok := tt.Find("决斗棋盘")
	if !ok {
		t.Fatal("duel board not found")
	}
	mx := boardRect.X + 3.5*app.board.Cell - app.board.X
	my := boardRect.Y + 3.5*app.board.Cell - app.board.Y
	tt.ClickAt(mx, my)
	tt.Advance(20 * time.Millisecond)
	if sx, sy := app.currentShakeOffset(tt.Now()); sx == 0 && sy == 0 {
		t.Fatal("expected active canvas shake before switching to reduced motion")
	}
	// System toggles ReduceMotion on while motionSystem is active -> immediate same-frame settle.
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	if !app.motionReducedActive || app.shakeAmplitude != 0 || len(app.ripples) != 0 {
		t.Fatalf("expected immediate settle on SetPreferences(ReduceMotion=true): reduced=%v amp=%v ripples=%d",
			app.motionReducedActive, app.shakeAmplitude, len(app.ripples))
	}
	// User forces motionStandard while system ReduceMotion remains true -> motion resumes.
	app.updatePrefs(func(p *prefs) { p.Motion = motionStandard })
	tt.Frame()
	if app.motionReducedActive {
		t.Fatal("motionStandard must override system ReduceMotion=true on canvas")
	}
	app.game.duel.StunnedUntil = 0
	mx2 := boardRect.X + 4.5*app.board.Cell - app.board.X
	my2 := boardRect.Y + 4.5*app.board.Cell - app.board.Y
	tt.ClickAt(mx2, my2)
	tt.Advance(20 * time.Millisecond)
	if sx, sy := app.currentShakeOffset(tt.Now()); sx == 0 && sy == 0 {
		t.Fatal("expected active canvas shake under motionStandard even with system ReduceMotion=true")
	}

	// Duel score popup must not drift upward when effective reduced motion is active,
	// and must still drift upward when motionStandard overrides system ReduceMotion=true.
	findPopupTop := func(baseImg, curImg *image.RGBA, board ui.Rect) int {
		b := curImg.Bounds()
		minX := max(b.Min.X, int(board.X)+20)
		maxX := min(b.Max.X-1, int(board.X+board.W)-200)
		minY := max(b.Min.Y, int(board.Y)+20)
		maxY := min(b.Max.Y-1, int(board.Y+board.H)-60)
		for y := minY; y <= maxY; y++ {
			for x := minX; x <= maxX; x++ {
				if curImg.RGBAAt(x, y) != baseImg.RGBAAt(x, y) {
					return y
				}
			}
		}
		return -1
	}
	for _, tc := range []struct {
		name        string
		mode        motionMode
		wantDrifted bool
	}{
		{"reduced_no_popup_drift", motionSystem, false},
		{"standard_allows_popup_drift", motionStandard, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := defaultPrefs()
			p.Motion = tc.mode
			popApp := newMinesweeperAppWith(appOptions{Seed: 42, Prefs: &p})
			popApp.diffIdx = findDifficultyIdx("duel")
			popApp.game = NewGame(Difficulty{Gameplay: GameplayDuel, Rows: 12, Cols: 12, Mines: 1}, true, MemberAnon, 42)
			popApp.game.FirstClick = true
			popApp.game.Status = StatusPlaying
			popApp.game.Cell(11, 11).IsMine = true
			popApp.game.RecomputeAdjacency()
			popApp.game.duel.NextAI = 1e9

			ptt := ui.NewTester(popApp.view, 1280, 900)
			ptt.SetPreferences(ui.Preferences{ReduceMotion: true})
			popApp.board.center(popApp.game, 5, 5)
			ptt.Frame()
			br, ok := ptt.Find("决斗棋盘")
			if !ok {
				t.Fatal("duel board missing")
			}
			baseCopy := image.NewRGBA(ptt.Image().Bounds())
			copy(baseCopy.Pix, ptt.Image().Pix)
			popApp.spawnDuelPopup(5, 5, ActionResult{Kind: ActionFlagged, ScoreDelta: 75}, ptt.Now())
			ptt.Frame()
			y0 := findPopupTop(baseCopy, ptt.Image(), br)
			if y0 < 0 {
				t.Fatal("expected duel popup pill rendered at t=0")
			}
			ptt.Advance(430 * time.Millisecond)
			y1 := findPopupTop(baseCopy, ptt.Image(), br)
			if y1 < 0 {
				t.Fatal("expected duel popup pill still visible at t=430ms")
			}
			if tc.wantDrifted {
				if y1 >= y0-4 {
					t.Fatalf("expected popup to drift upward under motionStandard: y0=%d y1=%d", y0, y1)
				}
			} else if y1 != y0 {
				t.Fatalf("expected zero vertical popup drift under reduced motion: y0=%d y1=%d", y0, y1)
			}
		})
	}
}

func TestGlobalMuteShortcutRepeatAndVolumePreservation(t *testing.T) {
	initial := prefs{
		DarkMode:      true,
		Motion:        motionSystem,
		SoundEnabled:  true,
		Volume:        73,
		AllowQuestion: false,
	}
	var saved []prefs
	app := newMinesweeperAppWith(appOptions{
		Seed:  42,
		Prefs: &initial,
		SavePrefs: func(p prefs) error {
			saved = append(saved, p)
			return nil
		},
	})
	tt := ui.NewTester(app.view, 1280, 900)

	// Modified M shortcuts (Ctrl+M, Shift+M, Alt+M, Super+M) must be ignored.
	for _, mod := range []ui.Modifiers{ui.Ctrl, ui.Shift, ui.Alt, ui.Super} {
		tt.Key(mod, ui.KeyM)
		if !app.soundEnabled || app.volume != 73 || len(saved) != 0 {
			t.Fatalf("modified M (%v) unexpectedly mutated mute: enabled=%v vol=%d saves=%d", mod, app.soundEnabled, app.volume, len(saved))
		}
	}

	// Hold unmodified M with repeated key-repeat events: toggles ONCE only and preserves Volume.
	tt.KeyDown(ui.KeyM)
	if app.soundEnabled || app.volume != 73 {
		t.Fatalf("initial KeyDown(M) did not mute while keeping volume: enabled=%v vol=%d", app.soundEnabled, app.volume)
	}
	for range 5 {
		tt.KeyRepeat(0, ui.KeyM)
		if app.soundEnabled || app.volume != 73 {
			t.Fatalf("auto-repeat KeyRepeat(M) must not re-toggle mute: enabled=%v vol=%d", app.soundEnabled, app.volume)
		}
	}
	if len(saved) != 1 || saved[0].SoundEnabled || saved[0].Volume != 73 {
		t.Fatalf("expected exactly 1 persisted mute transition, got %+v", saved)
	}

	// Releasing M and pressing again unmutes and keeps original volume 73.
	tt.KeyUp(ui.KeyM)
	tt.KeyDown(ui.KeyM)
	if !app.soundEnabled || app.volume != 73 {
		t.Fatalf("second press after release did not unmute with volume 73: enabled=%v vol=%d", app.soundEnabled, app.volume)
	}
	tt.KeyRepeat(0, ui.KeyM)
	if !app.soundEnabled {
		t.Fatal("auto-repeat after second KeyDown(M) must not re-mute")
	}
	tt.KeyUp(ui.KeyM)
	if len(saved) != 2 || !saved[1].SoundEnabled || saved[1].Volume != 73 {
		t.Fatalf("expected 2 persisted mute transitions, got %+v", saved)
	}

	// Unmodified M also works while the setlist popover is open.
	if err := tt.Click("歌单"); err != nil {
		t.Fatalf("open setlist: %v", err)
	}
	if !app.popoverOpen(popoverSetlist) {
		t.Fatal("expected setlist popover open")
	}
	tt.Key(0, ui.KeyM)
	if app.soundEnabled || app.volume != 73 {
		t.Fatalf("M while popover open failed: enabled=%v vol=%d", app.soundEnabled, app.volume)
	}
	tt.KeyRepeat(0, ui.KeyM)
	if app.soundEnabled {
		t.Fatal("auto-repeat M while popover open must be ignored")
	}
	// Unmodified M also works while Haruhikage modal drama overlay and AlertDialog are open.
	modalApp := newMinesweeperAppWith(appOptions{
		Seed:  42,
		Prefs: &initial,
	})
	modalApp.game = NewGame(Difficulty{
		ID:         "marked-classic",
		Song:       "春日影",
		Gameplay:   GameplayClassic,
		Haruhikage: true,
		Rows:       5,
		Cols:       5,
		Mines:      1,
	}, false, MemberTomori, 7)
	modalApp.game.FirstClick = true
	modalApp.game.Status = StatusPlaying
	modalApp.game.Cell(0, 0).IsMine = true
	modalApp.game.RecomputeAdjacency()
	mtt := ui.NewTester(modalApp.view, 1280, 900)
	if err := mtt.Click("cell-0-0"); err != nil {
		t.Fatalf("trigger haruhikage loss: %v", err)
	}
	if !modalApp.haruhikageIntro || modalApp.haruhikageAlert {
		t.Fatalf("expected haruhikageIntro active: intro=%v alert=%v", modalApp.haruhikageIntro, modalApp.haruhikageAlert)
	}
	mtt.KeyDown(ui.KeyM)
	if modalApp.soundEnabled || modalApp.volume != 73 {
		t.Fatalf("M while haruhikageIntro open failed: enabled=%v vol=%d", modalApp.soundEnabled, modalApp.volume)
	}
	mtt.KeyRepeat(0, ui.KeyM)
	if modalApp.soundEnabled {
		t.Fatal("auto-repeat M while haruhikageIntro open must be ignored")
	}
	mtt.KeyUp(ui.KeyM)
	mtt.Key(ui.Ctrl, ui.KeyM)
	if modalApp.soundEnabled {
		t.Fatal("Ctrl+M while haruhikageIntro open must be ignored")
	}
	if err := mtt.Click("春日影演出遮罩"); err != nil {
		t.Fatalf("advance to haruhikageAlert: %v", err)
	}
	if modalApp.haruhikageIntro || !modalApp.haruhikageAlert {
		t.Fatalf("expected haruhikageAlert active: intro=%v alert=%v", modalApp.haruhikageIntro, modalApp.haruhikageAlert)
	}
	mtt.KeyDown(ui.KeyM)
	if !modalApp.soundEnabled || modalApp.volume != 73 {
		t.Fatalf("M while haruhikageAlert open failed: enabled=%v vol=%d", modalApp.soundEnabled, modalApp.volume)
	}
	mtt.KeyRepeat(0, ui.KeyM)
	if !modalApp.soundEnabled {
		t.Fatal("auto-repeat M while haruhikageAlert open must be ignored")
	}
	mtt.KeyUp(ui.KeyM)
	mtt.Key(ui.Ctrl, ui.KeyM)
	if !modalApp.soundEnabled {
		t.Fatal("Ctrl+M while haruhikageAlert open must be ignored")
	}
	if err := mtt.Click("对不起，我错了"); err != nil {
		t.Fatalf("dismiss haruhikageAlert after M toggle: %v", err)
	}
	if modalApp.haruhikageAlert {
		t.Fatal("expected haruhikageAlert dismissed")
	}
	t.Logf("verified M mute toggle with auto-repeat suppression across %d persisted transitions (including modal overlays), volume retained=%d", len(saved), app.volume)
}

func TestWindowFocusTracking(t *testing.T) {
	app := newMinesweeperApp(42)
	tt := ui.NewTester(app.view, 1280, 900)
	if !app.isWindowFocused() {
		t.Fatal("expected newly created window to start focused")
	}

	// Losing active state via Tester updates foreground tracking immediately.
	tt.SetWindowActive(false)
	if app.isWindowFocused() {
		t.Fatal("expected SetWindowActive(false) to mark window unfocused")
	}
	tt.SetWindowActive(true)
	if !app.isWindowFocused() {
		t.Fatal("expected SetWindowActive(true) to restore window focus")
	}

	// Minimizing/hiding window marks window unfocused until shown again.
	tt.SetWindowHidden(true)
	if app.isWindowFocused() {
		t.Fatal("expected SetWindowHidden(true) to mark window unfocused")
	}
	tt.SetWindowHidden(false)
	if !app.isWindowFocused() {
		t.Fatal("expected SetWindowHidden(false) to restore window focus")
	}

	// Desktop OnBlur / OnFocus callbacks update focus tracking directly.
	app.setWindowFocused(false)
	if app.isWindowFocused() {
		t.Fatal("expected setWindowFocused(false) to mark window unfocused")
	}
	app.setWindowFocused(true)
	if !app.isWindowFocused() {
		t.Fatal("expected setWindowFocused(true) to restore window focus")
	}
}

func TestSharedPopoverMutualExclusionAndEscapeFocusReturn(t *testing.T) {
	rec := &soundRecorder{}
	app := newMinesweeperAppWith(appOptions{Seed: 12345, Sound: rec})
	var boardClicks int
	var returnEmptyPanel bool
	// Headless fixture exercising the shared popover entrypoints with two
	// sibling popovers (existing setlist + a second fixture settings popover)
	// sharing popoverWorkspaceShield, without adding any controls to app.view.
	fixtureView := func(c *ui.Context) {
		pal := app.paper()
		theme := app.activeTheme()
		c.SetTheme(theme)
		tTheme := c.Theme()

		ui.Column(c).Fill().Padding(14).Gap(10).Children(func() {
			ui.Row(c).AlignItems(ui.Center).Gap(12).Children(func() {
				ui.Button(c.Key("fixture-outside-btn"), "外部按钮")

				settingsOpen := app.popoverOpen(popoverSettings)
				settingsTrigger := ui.Button(c.Key("fixture-settings-trigger"), "设置夹具").
					HasPopup(ui.PopupDialog).Expanded(settingsOpen)
				focusFixtureInner := false
				if settingsTrigger.Clicked() {
					focusFixtureInner = app.togglePopover(popoverSettings)
				}
				app.bindPopover(c, popoverSettings, "fixture-settings-popover", settingsTrigger, func(closeWithFocus func()) ui.Element {
					panel := ui.Column(c).Width(220).Padding(8).Gap(6)
					panel.Children(func() {
						btn := ui.Button(c.Key("fixture-inner-btn"), "夹具开关")
						if focusFixtureInner {
							btn.Focus()
						}
						if btn.Clicked() {
							app.toggleMute()
						}
					})
					if returnEmptyPanel {
						return ui.Element{}
					}
					return panel
				})

				app.setlistControl(c, pal, tTheme)
			})

			ui.Box(c).Grow(1).FillWidth().Children(func() {
				board := ui.Box(c.Key("fixture-board")).Fill().Label("夹具棋盘")
				if board.Clicked() {
					boardClicks++
				}
				app.popoverWorkspaceShield(c)
			})
		})
	}

	tt := ui.NewTester(fixtureView, 1280, 900)

	// 1. Open setlist, then single-click settings trigger -> mutually switches in one click.
	if err := tt.Click("歌单"); err != nil {
		t.Fatalf("open setlist: %v", err)
	}
	if !app.popoverOpen(popoverSetlist) || app.popoverOpen(popoverSettings) {
		t.Fatalf("expected only setlist open, active=%v", app.activePopover)
	}
	if err := tt.Click("设置夹具"); err != nil {
		t.Fatalf("single-click switch to settings fixture: %v", err)
	}
	if !app.popoverOpen(popoverSettings) || app.popoverOpen(popoverSetlist) || app.setlistOpen {
		t.Fatalf("expected single-click switch from setlist to settings, active=%v setlistOpen=%v", app.activePopover, app.setlistOpen)
	}
	// The switching click's press puts the setlist away and its release
	// opens the settings sheet.
	assertSoundIDs(t, "open setlist then switch to settings", rec.take(), SoundPopoverOpen, SoundPopoverClose, SoundPopoverOpen)

	// 2. While settings popover is open, Escape closes it and returns focus to its trigger.
	if !tt.Focused("夹具开关") {
		t.Fatal("expected focus inside settings fixture popover")
	}
	tt.Key(0, ui.KeyEscape)
	if app.hasOpenPopover() {
		t.Fatalf("Escape should close settings fixture popover, active=%v", app.activePopover)
	}
	if !tt.Focused("设置夹具") {
		t.Fatal("Escape from settings fixture popover should return focus to its trigger")
	}
	assertSoundIDs(t, "Escape from settings", rec.take(), SoundPopoverClose)

	// 3. Open settings again, then single-click setlist trigger -> mutually switches back in one click.
	if err := tt.Click("设置夹具"); err != nil {
		t.Fatalf("reopen settings fixture: %v", err)
	}
	if err := tt.Click("歌单"); err != nil {
		t.Fatalf("single-click switch to setlist: %v", err)
	}
	if !app.popoverOpen(popoverSetlist) || app.popoverOpen(popoverSettings) {
		t.Fatalf("expected single-click switch from settings to setlist, active=%v", app.activePopover)
	}

	// 4. Escape from setlist closes it and returns focus to 歌单 trigger.
	tt.Key(0, ui.KeyEscape)
	if app.hasOpenPopover() || app.setlistOpen {
		t.Fatalf("Escape should close setlist popover, active=%v setlistOpen=%v", app.activePopover, app.setlistOpen)
	}
	if !tt.Focused("歌单") {
		t.Fatal("Escape from setlist should return focus to 歌单 trigger")
	}

	// 5. Clicking workspace outside settings popover dismisses it without leaking click onto board.
	if err := tt.Click("设置夹具"); err != nil {
		t.Fatalf("open settings fixture for shield check: %v", err)
	}
	boardRect, ok := tt.Find("夹具棋盘")
	if !ok {
		t.Fatal("fixture board not found")
	}
	tt.ClickAt(boardRect.X+boardRect.W/2, boardRect.Y+boardRect.H/2)
	if app.hasOpenPopover() {
		t.Fatalf("outside click on workspace should close settings popover, active=%v", app.activePopover)
	}
	if boardClicks != 0 {
		t.Fatalf("outside dismiss click leaked to board: clicks=%d", boardClicks)
	}
	// Steps 3-5: open, switch, Escape, open, outside dismiss.
	assertSoundIDs(t, "steps 3-5", rec.take(),
		SoundPopoverOpen, SoundPopoverClose, SoundPopoverOpen, SoundPopoverClose, SoundPopoverOpen, SoundPopoverClose)

	// 5b. Programmatic openPopover while focus sits on an unrelated control:
	// Escape must still close the popover and return focus to its trigger.
	if err := tt.Click("外部按钮"); err != nil {
		t.Fatalf("focus outside button: %v", err)
	}
	app.openPopover(popoverSettings)
	tt.Frame()
	tt.Key(0, ui.KeyEscape)
	if app.hasOpenPopover() {
		t.Fatalf("Escape should close programmatically opened settings popover, active=%v", app.activePopover)
	}
	if !tt.Focused("设置夹具") {
		t.Fatal("Escape on programmatically opened settings popover must return focus to 设置夹具")
	}

	if err := tt.Click("外部按钮"); err != nil {
		t.Fatalf("re-focus outside button: %v", err)
	}
	app.openPopover(popoverSetlist)
	tt.Frame()
	tt.Key(0, ui.KeyEscape)
	if app.hasOpenPopover() || app.setlistOpen {
		t.Fatalf("Escape should close programmatically opened setlist popover, active=%v setlistOpen=%v", app.activePopover, app.setlistOpen)
	}
	if !tt.Focused("歌单") {
		t.Fatal("Escape on programmatically opened setlist popover must return focus to 歌单")
	}

	// 5c. Popover whose body returns an empty ui.Element{} still restores trigger focus on Escape.
	returnEmptyPanel = true
	if err := tt.Click("设置夹具"); err != nil {
		t.Fatalf("open empty-panel settings fixture: %v", err)
	}
	tt.Key(0, ui.KeyEscape)
	returnEmptyPanel = false
	if app.hasOpenPopover() {
		t.Fatalf("Escape should close empty-panel settings popover, active=%v", app.activePopover)
	}
	if !tt.Focused("设置夹具") {
		t.Fatal("Escape on empty-panel settings popover must return focus to 设置夹具")
	}

	// 6. Duel settlement auto-closes whichever popover is open.
	duelApp := newMinesweeperApp(42)
	duelApp.game = NewGame(Difficulty{
		ID:           "duel-popover",
		Song:         "春日影",
		Gameplay:     GameplayDuel,
		Rows:         5,
		Cols:         5,
		Mines:        1,
		DuelDuration: 1,
		AIInterval:   10,
	}, true, MemberTomori, 7)
	duelApp.game.FirstClick = true
	duelApp.game.Status = StatusPlaying
	duelApp.game.duel.PlayerScore = 100
	duelApp.game.duel.AIScore = 50
	dtt := ui.NewTester(duelApp.view, 1280, 900)
	duelApp.openPopover(popoverSettings)
	dtt.Advance(1100 * time.Millisecond)
	if !duelApp.duelEnded || duelApp.hasOpenPopover() {
		t.Fatalf("expected duel settlement to dismiss open popover: ended=%v active=%v", duelApp.duelEnded, duelApp.activePopover)
	}
}

func TestDuelTimedWinRecordsSingleMemeUnlockWithoutAlteringOutput(t *testing.T) {
	setupDuelWin := func(app *minesweeperApp) *ui.Tester {
		app.memberID = MemberTomori
		app.game = NewGame(Difficulty{
			ID:           "duel-win-test",
			Label:        "决斗计时测试",
			Song:         "潜在表明",
			Gameplay:     GameplayDuel,
			Rows:         5,
			Cols:         5,
			Mines:        1,
			DuelDuration: 1,
			AIInterval:   10,
		}, true, MemberTomori, 7)
		app.game.FirstClick = true
		app.game.Status = StatusPlaying
		app.game.duel.PlayerScore = 240
		app.game.duel.AIScore = 90
		app.duelEnded = false
		app.lastTick = time.Time{}
		return ui.NewTester(app.view, 1280, 900)
	}

	app := newMinesweeperApp(42)
	tt := setupDuelWin(app)
	chatBefore := len(app.chatLog)

	tt.Advance(1100 * time.Millisecond)
	// Extra frames after settlement must not duplicate the unlock record.
	tt.Advance(200 * time.Millisecond)
	tt.Frame()

	if !app.duelEnded || app.game.Status != StatusWon {
		t.Fatalf("expected timed duel win settled, ended=%v status=%v", app.duelEnded, app.game.Status)
	}
	if !app.unlockedMemes[MemeForever] {
		t.Fatal("expected MemeForever unlocked on timed duel win")
	}
	if len(app.memeUnlockLog) != 1 || app.memeUnlockLog[0].Meme.ID != MemeForever {
		t.Fatalf("expected exactly 1 first-unlock record for %q, got %+v", MemeForever, app.memeUnlockLog)
	}

	// Preserve existing chat & toast presentation on the timed duel win path:
	// only the settlement chat message is appended (no extra "名场面解锁" chat
	// entry and no extra "解锁名场面" toast).
	if len(app.chatLog) != chatBefore+1 {
		t.Fatalf("timed duel win chat entries changed: before=%d after=%d log=%+v", chatBefore, len(app.chatLog), app.chatLog)
	}
	lastChat := app.chatLog[len(app.chatLog)-1]
	if lastChat.Speaker != "决斗结算" || !strings.Contains(lastChat.Text, "决斗胜利！240 : 90") {
		t.Fatalf("unexpected settlement chat message: %+v", lastChat)
	}
	for _, msg := range app.chatLog {
		if msg.Speaker == "名场面解锁" {
			t.Fatalf("timed duel win must not alter chat with 名场面解锁 entry: %+v", msg)
		}
	}
	if !tt.HasText("决斗胜利！240 : 90 · 赢了就是赢了！") || !tt.HasText("再战一局") {
		t.Fatal("expected existing duel victory toast action to remain unchanged")
	}
	foreverMeme, ok := findMeme(MemeForever)
	if !ok {
		t.Fatal("missing MemeForever metadata")
	}
	if tt.HasText("解锁名场面：「" + foreverMeme.Title + "」") {
		t.Fatal("timed duel win must not add a new unlock toast on this path")
	}

	// Winning a second timed duel in the same run must not record a second unlock.
	tt2 := setupDuelWin(app)
	tt2.Advance(1100 * time.Millisecond)
	if len(app.memeUnlockLog) != 1 {
		t.Fatalf("second timed duel win duplicated unlock record: %+v", app.memeUnlockLog)
	}

	// Winning a classic game afterwards in the same run skips MemeForever and
	// records only newly unlocked memes (e.g. MemeAnonBest) once.
	app.memberID = MemberAnon
	app.game = NewGame(Difficulty{ID: "easy", Rows: 5, Cols: 5, Mines: 1}, false, MemberAnon, 7)
	app.game.FirstClick = true
	app.game.Status = StatusPlaying
	app.game.Cell(3, 4).IsMine = true
	app.game.RecomputeAdjacency()
	for i := range app.game.Cells {
		c := &app.game.Cells[i]
		if !c.IsMine && !(c.Row == 4 && c.Col == 4) {
			app.game.Reveal(c.Row, c.Col)
		}
	}
	if app.game.Status != StatusPlaying || app.game.Cell(4, 4).Revealed {
		t.Fatalf("expected (4,4) to remain the final unrevealed safe cell: status=%v revealed=%v", app.game.Status, app.game.Cell(4, 4).Revealed)
	}
	ttClassic := ui.NewTester(app.view, 1280, 900)
	if err := ttClassic.Click("cell-4-4"); err != nil {
		t.Fatalf("finish classic win: %v", err)
	}
	foreverCount := 0
	anonBestCount := 0
	for _, notice := range app.memeUnlockLog {
		switch notice.Meme.ID {
		case MemeForever:
			foreverCount++
		case MemeAnonBest:
			anonBestCount++
		}
	}
	if foreverCount != 1 || anonBestCount != 1 {
		t.Fatalf("unexpected unlock counts after follow-up classic win: forever=%d anonBest=%d log=%+v",
			foreverCount, anonBestCount, app.memeUnlockLog)
	}

	// Sanity check: prefs file path stays strictly inside configDir.
	if got := prefsPath(filepath.Join("root", "cfg")); !strings.HasSuffix(got, filepath.Join("dev.mygo.minesweeper", "prefs.json")) {
		t.Fatalf("unexpected prefsPath: %q", got)
	}
	t.Logf("verified timed duel win recorded %d MemeForever unlock notice with unchanged chat/toast", foreverCount)
}
