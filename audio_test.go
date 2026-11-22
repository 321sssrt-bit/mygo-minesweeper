package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"math"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/audio"
	"github.com/egoist/mygo/ui"
)

// soundRecorder is the in-memory output headless tests inject. With a clock
// it also stamps each cue with the time it played.
type soundRecorder struct {
	cues  []soundCue
	at    []time.Time
	clock func() time.Time
}

func (r *soundRecorder) Play(c soundCue) {
	r.cues = append(r.cues, c)
	if r.clock != nil {
		r.at = append(r.at, r.clock())
	}
}

// take returns the cues played since the last take.
func (r *soundRecorder) take() []soundCue {
	c := r.cues
	r.cues, r.at = nil, nil
	return c
}

func soundIDs(cues []soundCue) []SoundID {
	ids := make([]SoundID, len(cues))
	for i, c := range cues {
		ids[i] = c.ID
	}
	return ids
}

func assertSoundIDs(t *testing.T, step string, got []soundCue, want ...SoundID) {
	t.Helper()
	if !slices.Equal(soundIDs(got), want) {
		t.Fatalf("%s: sounds = %v, want %v", step, soundIDs(got), want)
	}
}

func newRecordedApp(seed uint64, p prefs) (*minesweeperApp, *soundRecorder) {
	rec := &soundRecorder{}
	return newMinesweeperAppWith(appOptions{Seed: seed, Prefs: &p, Sound: rec}), rec
}

// fakeVoice stands in for an *audio.Player and logs what it was told.
type fakeVoice struct {
	sound *audio.Sound
	calls []string
}

func (v *fakeVoice) SetVolume(g float64) { v.calls = append(v.calls, fmt.Sprintf("volume %.2f", g)) }
func (v *fakeVoice) SetPan(p float64)    { v.calls = append(v.calls, fmt.Sprintf("pan %+.2f", p)) }
func (v *fakeVoice) SetSpeed(s float64)  { v.calls = append(v.calls, fmt.Sprintf("speed %.2f", s)) }
func (v *fakeVoice) Play()               { v.calls = append(v.calls, "play") }

func TestAudioDesktopAdapterSetsVoiceBeforePlay(t *testing.T) {
	bank := NewSoundBank()
	d := newDesktopSound(bank)
	for id := range soundCount {
		want := time.Duration(len(bank.Samples(id))) * time.Second / SynthRate
		if got := d.sounds[id].Duration(); got != want {
			t.Fatalf("%v: sound lasts %v, want %v of mono 48 kHz samples", id, got, want)
		}
	}
	var voices []*fakeVoice
	d.newVoice = func(s *audio.Sound) soundVoice {
		v := &fakeVoice{sound: s}
		voices = append(voices, v)
		return v
	}

	// Through the app: volume 40 scales the gain, column 0 of 9 pans hard left.
	p := defaultPrefs()
	p.Volume = 40
	app := newMinesweeperAppWith(appOptions{Seed: 7, Prefs: &p, Sound: d})
	tt := ui.NewTester(app.view, 1280, 900)
	if err := tt.RightClick("cell-0-0"); err != nil {
		t.Fatal(err)
	}
	if err := tt.RightClick("cell-0-8"); err != nil {
		t.Fatal(err)
	}
	want := [][]string{
		{"volume 0.40", "pan -0.35", "speed 1.00", "play"},
		{"volume 0.40", "pan +0.35", "speed 1.00", "play"},
	}
	if len(voices) != len(want) {
		t.Fatalf("played %d voices, want %d", len(voices), len(want))
	}
	for i, v := range voices {
		if !slices.Equal(v.calls, want[i]) {
			t.Fatalf("voice %d calls = %v, want gain, pan and speed before play: %v", i, v.calls, want[i])
		}
		if v.sound != d.sounds[SoundBandageOn] {
			t.Fatalf("voice %d plays the wrong sound", i)
		}
	}
}

func TestAudioVoiceForwardsToPlayer(t *testing.T) {
	s := audio.NewSound(SynthRate, 1, NewSoundBank().Samples(SoundToggle))
	v := audioVoice{s.NewPlayer()}
	v.SetVolume(0.25)
	v.SetPan(-0.3)
	v.SetSpeed(1.1)
	if v.p.Volume() != 0.25 || v.p.Pan() != -0.3 || v.p.Speed() != 1.1 || v.p.Playing() {
		t.Fatalf("player volume=%v pan=%v speed=%v playing=%v", v.p.Volume(), v.p.Pan(), v.p.Speed(), v.p.Playing())
	}
}

// TestAudioOnlyDesktopEntryOpensOutput checks that only main binds the real
// output: every other constructor, the snapshot scenes and the benchmarks
// play into silence.
func TestAudioOnlyDesktopEntryOpensOutput(t *testing.T) {
	for name, app := range map[string]*minesweeperApp{
		"newMinesweeperApp":     newMinesweeperApp(1),
		"newMinesweeperAppWith": newMinesweeperAppWith(appOptions{}),
		"stageLiveScene":        stageLiveScene(true),
	} {
		if _, ok := app.sound.(silentSound); !ok {
			t.Fatalf("%s plays into %T, want silentSound", name, app.sound)
		}
	}

	fset := token.NewFileSet()
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	var callers []string
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		for _, imp := range f.Imports {
			if p, _ := strconv.Unquote(imp.Path.Value); p == "github.com/egoist/mygo/audio" && path != "audio.go" {
				t.Fatalf("%s imports mygo/audio; only audio.go may", path)
			}
		}
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			ast.Inspect(fn, func(n ast.Node) bool {
				if call, ok := n.(*ast.CallExpr); ok {
					if id, ok := call.Fun.(*ast.Ident); ok && id.Name == "newDesktopSound" {
						callers = append(callers, fn.Name.Name)
					}
				}
				return true
			})
		}
	}
	if !slices.Equal(callers, []string{"main"}) {
		t.Fatalf("newDesktopSound called from %v, want only main", callers)
	}
}

func TestAudioColumnPanRange(t *testing.T) {
	for _, cols := range []int{9, 16, 50, 1000} {
		prev := -1.0
		for col := range cols {
			pan := columnPan(col, cols)
			if pan <= prev || pan < -maxSoundPan || pan > maxSoundPan {
				t.Fatalf("cols=%d col=%d pan=%v after %v, want rising within ±%v", cols, col, pan, prev, maxSoundPan)
			}
			prev = pan
		}
		if columnPan(0, cols) != -maxSoundPan || columnPan(cols-1, cols) != maxSoundPan {
			t.Fatalf("cols=%d edges pan %v..%v", cols, columnPan(0, cols), columnPan(cols-1, cols))
		}
	}
	if columnPan(0, 1) != 0 || clampPan(-2) != -maxSoundPan || clampPan(2) != maxSoundPan {
		t.Fatal("single column must center and out-of-range pans clamp")
	}
}

func TestSoundMuteAndVolume(t *testing.T) {
	p := defaultPrefs()
	p.Volume = 40
	app, rec := newRecordedApp(7, p)
	tt := ui.NewTester(app.view, 1280, 900)
	if err := tt.RightClick("cell-0-0"); err != nil {
		t.Fatal(err)
	}
	if got := rec.take(); len(got) != 1 || got[0].Gain != 0.4 || got[0].Speed != 1 {
		t.Fatalf("volume 40 cue = %+v, want gain 0.4 at speed 1", got)
	}

	tt.Key(0, ui.KeyM)
	if err := tt.RightClick("cell-0-0"); err != nil {
		t.Fatal(err)
	}
	tt.Key(0, ui.KeyF)
	if app.soundEnabled || app.game.Cell(0, 0).Flagged || !app.flagMode {
		t.Fatal("muted moves must still take effect")
	}
	assertSoundIDs(t, "muted", rec.take())

	tt.Key(0, ui.KeyM)
	app.updatePrefs(func(p *prefs) { p.Volume = 0 })
	tt.Key(0, ui.KeyF)
	if err := tt.RightClick("cell-0-1"); err != nil {
		t.Fatal(err)
	}
	// Unmuting rustles the switch once; at volume 0 nothing sounds.
	assertSoundIDs(t, "unmute then volume 0", rec.take(), SoundToggle)

	app.updatePrefs(func(p *prefs) { p.Volume = 100 })
	assertSoundIDs(t, "volume change alone", rec.take())
}

func TestSoundClassicGridFoley(t *testing.T) {
	app, rec := newRecordedApp(7, defaultPrefs())
	tt := ui.NewTester(app.view, 1280, 900)
	g := app.game
	tt.Frame()
	assertSoundIDs(t, "startup", rec.take())

	// Bandages along row 0 pan from the left ear to the right.
	prev := -1.0
	for col := range g.Cols {
		if err := tt.RightClick(fmt.Sprintf("cell-0-%d", col)); err != nil {
			t.Fatal(err)
		}
		got := rec.take()
		assertSoundIDs(t, "flag row 0", got, SoundBandageOn)
		if got[0].Pan <= prev {
			t.Fatalf("col %d pan %v not right of %v", col, got[0].Pan, prev)
		}
		prev = got[0].Pan
		if err := tt.RightClick(fmt.Sprintf("cell-0-%d", col)); err != nil {
			t.Fatal(err)
		}
		assertSoundIDs(t, "unflag row 0", rec.take(), SoundBandageOff)
	}

	// A 旗 posts its quote to the chat but sounds only its bandage.
	posted := app.chatPosted
	if err := tt.RightClick("cell-1-1"); err != nil {
		t.Fatal(err)
	}
	if app.chatPosted == posted {
		t.Fatal("expected the flag to post a quote")
	}
	assertSoundIDs(t, "flag with quote", rec.take(), SoundBandageOn)

	// Left-clicking a flag does nothing and stays silent.
	if err := tt.RightClick("cell-4-0"); err != nil {
		t.Fatal(err)
	}
	rec.take()
	if err := tt.Click("cell-4-0"); err != nil {
		t.Fatal(err)
	}
	assertSoundIDs(t, "left-click on a flag", rec.take())

	// One reveal touches on press and lifts one slip on release, however far it floods.
	if err := tt.Click("cell-4-4"); err != nil {
		t.Fatal(err)
	}
	got := rec.take()
	assertSoundIDs(t, "reveal", got, SoundPaperTouch, SoundPaperLift)
	if !g.Cell(4, 4).Revealed {
		t.Fatal("cell-4-4 should be revealed")
	}
	if got[0].Pan != 0 || got[1].Pan != 0 {
		t.Fatalf("center column pans = (%v, %v)", got[0].Pan, got[1].Pan)
	}

	// Right-clicking a revealed cell changes nothing.
	if err := tt.RightClick("cell-4-4"); err != nil {
		t.Fatal(err)
	}
	assertSoundIDs(t, "right-click revealed", rec.take())

	// With ? marks on: 旗 sticks on, ? peels it off, blank erases quietly.
	app.allowQuestion = true
	hidden := firstHiddenUnflagged(t, g)
	label := fmt.Sprintf("cell-%d-%d", hidden.Row, hidden.Col)
	for _, want := range [][]SoundID{{SoundBandageOn}, {SoundBandageOff}, nil} {
		if err := tt.RightClick(label); err != nil {
			t.Fatal(err)
		}
		assertSoundIDs(t, "right-click cycle", rec.take(), want...)
	}
	if hidden.Flagged || hidden.Questioned {
		t.Fatal("cycle should end blank")
	}

	// Flag mode turns left clicks into bandages.
	tt.Key(0, ui.KeyF)
	assertSoundIDs(t, "F", rec.take(), SoundToggle)
	if err := tt.Click(label); err != nil {
		t.Fatal(err)
	}
	assertSoundIDs(t, "flag-mode click", rec.take(), SoundBandageOn)
}

func firstHiddenUnflagged(t *testing.T, g *Game) *Cell {
	t.Helper()
	for i := range g.Cells {
		if c := &g.Cells[i]; !c.Revealed && !c.Flagged && !c.Questioned {
			return c
		}
	}
	t.Fatal("no hidden cell left")
	return nil
}

// canvasCellAt pans a canvas board to a cell and returns its screen point.
func canvasCellAt(t *testing.T, tt *ui.Tester, app *minesweeperApp, label string, row, col int) (float32, float32) {
	t.Helper()
	r, ok := tt.Find(label)
	if !ok {
		t.Fatalf("%s not on screen", label)
	}
	v := app.board
	v.center(app.game, row, col)
	tt.Frame()
	return r.X + (float32(col)+0.5)*v.Cell - v.X, r.Y + (float32(row)+0.5)*v.Cell - v.Y
}

func TestSoundClassicCanvasFoley(t *testing.T) {
	app, rec := newRecordedApp(7, defaultPrefs())
	app.game = NewGame(TrackUtakotoba, app.bandMode(), app.memberID, 7)
	tt := ui.NewTester(app.view, 1280, 900)
	g := app.game

	for _, tc := range []struct {
		col int
		pan float64
	}{{0, -maxSoundPan}, {g.Cols - 1, maxSoundPan}} {
		x, y := canvasCellAt(t, tt, app, "经典棋盘", 0, tc.col)
		tt.RightClickAt(x, y)
		got := rec.take()
		assertSoundIDs(t, "canvas flag", got, SoundBandageOn)
		if got[0].Pan != tc.pan {
			t.Fatalf("col %d pan = %v, want %v", tc.col, got[0].Pan, tc.pan)
		}
		tt.RightClickAt(x, y)
		assertSoundIDs(t, "canvas unflag", rec.take(), SoundBandageOff)
	}

	x, y := canvasCellAt(t, tt, app, "经典棋盘", g.Rows/2, g.Cols/2)
	tt.ClickAt(x, y)
	assertSoundIDs(t, "canvas reveal", rec.take(), SoundPaperTouch, SoundPaperLift)
	if !g.Cell(g.Rows/2, g.Cols/2).Revealed {
		t.Fatal("canvas reveal did not open the cell")
	}
	tt.RightClickAt(x, y)
	assertSoundIDs(t, "canvas right-click revealed", rec.take())
}

func TestSoundDuelCanvasFoley(t *testing.T) {
	app, rec := newRecordedApp(42, defaultPrefs())
	app.game = NewGame(Difficulty{
		ID:           "duel-sound",
		Song:         "春日影",
		Gameplay:     GameplayDuel,
		Rows:         40,
		Cols:         40,
		Mines:        200,
		DuelDuration: 300,
		AIInterval:   1000,
	}, true, MemberTomori, 7)
	tt := ui.NewTester(app.view, 1280, 900)
	g := app.game

	x, y := canvasCellAt(t, tt, app, "决斗棋盘", 20, 0)
	tt.RightClickAt(x, y)
	got := rec.take()
	assertSoundIDs(t, "duel flag", got, SoundBandageOn)
	if got[0].Pan != -maxSoundPan {
		t.Fatalf("duel col 0 pan = %v", got[0].Pan)
	}
	tt.ClickAt(x, y)
	assertSoundIDs(t, "duel left-click on own flag", rec.take())
	tt.RightClickAt(x, y)
	assertSoundIDs(t, "duel unflag", rec.take(), SoundBandageOff)
	if g.Cell(20, 0).Flagged {
		t.Fatal("duel flag should be peeled off")
	}

	// Hold back the duel's timed chatter, which would otherwise post its
	// first line as the opening reveal starts the clock.
	app.duelNoticeAt = tt.Now()
	x, y = canvasCellAt(t, tt, app, "决斗棋盘", 20, 20)
	tt.ClickAt(x, y)
	assertSoundIDs(t, "duel reveal", rec.take(), SoundPaperTouch, SoundPaperLift)
	if !g.Cell(20, 20).Revealed {
		t.Fatal("duel reveal did not open the cell")
	}
}

func TestSoundShieldSticksBandage(t *testing.T) {
	app, rec := newRecordedApp(7, defaultPrefs())
	app.memberID = MemberSoyorin
	app.game = NewGame(Difficulty{ID: "easy", Rows: 5, Cols: 5, Mines: 2, SkillCharges: 1}, true, MemberSoyorin, 7)
	g := app.game
	g.FirstClick = true
	g.Status = StatusPlaying
	g.Cell(2, 2).IsMine = true
	g.Cell(0, 0).IsMine = true
	g.RecomputeAdjacency()
	tt := ui.NewTester(app.view, 1280, 900)
	posted := app.chatPosted
	must(t, tt.Click("cell-2-2"))
	if !g.Cell(2, 2).ShieldSaved || app.chatPosted == posted {
		t.Fatal("expected the shield to save the cell and post Soyo's line")
	}
	assertSoundIDs(t, "shield", rec.take(), SoundPaperTouch, SoundBandageOn, SoundMotifSoyo)
}

func TestSoundSwitchesToggleOnce(t *testing.T) {
	app, rec := newRecordedApp(7, defaultPrefs())
	tt := ui.NewTester(app.view, 1280, 900)
	tt.UseScreenReader()

	steps := []struct {
		name string
		do   func()
	}{
		{"插旗模式", func() { must(t, tt.Click("插旗模式 (F)")) }},
		{"F", func() { tt.Key(0, ui.KeyF) }},
		{"名场面图鉴", func() { must(t, tt.Click("名场面图鉴")) }},
	}
	for _, s := range steps {
		before := app.currentPrefs()
		flag, memes := app.flagMode, app.showMemes
		s.do()
		if app.currentPrefs() == before && app.flagMode == flag && app.showMemes == memes {
			t.Fatalf("%s changed nothing", s.name)
		}
		assertSoundIDs(t, s.name, rec.take(), SoundToggle)
	}

	// Preference switches inside the 设置 popover also sound once per toggle.
	must(t, tt.Click("设置"))
	assertSoundIDs(t, "open settings", rec.take(), SoundPopoverOpen)
	settingsSteps := []struct {
		name string
		do   func()
	}{
		{"夜间台灯", func() {
			sw, ok := tt.AccessByRole(ui.RoleSwitch, "夜间台灯")
			if !ok {
				t.Fatal("missing 夜间台灯 switch")
			}
			tt.ClickAt(sw.Bounds.X+sw.Bounds.W/2, sw.Bounds.Y+sw.Bounds.H/2)
		}},
		{"动效", func() { must(t, tt.Click("减弱")) }},
		{"右键 ? 标记", func() {
			sw, ok := tt.AccessByRole(ui.RoleSwitch, "右键 ? 标记")
			if !ok {
				t.Fatal("missing 右键 ? 标记 switch")
			}
			tt.ClickAt(sw.Bounds.X+sw.Bounds.W/2, sw.Bounds.Y+sw.Bounds.H/2)
		}},
	}
	for _, s := range settingsSteps {
		before := app.currentPrefs()
		s.do()
		if app.currentPrefs() == before {
			t.Fatalf("%s changed nothing", s.name)
		}
		assertSoundIDs(t, s.name, rec.take(), SoundToggle)
	}
	must(t, tt.Click("设置"))
	assertSoundIDs(t, "close settings", rec.take(), SoundPopoverClose)

	// Muting is silent; unmuting rustles once; a held M repeats nothing.
	tt.Key(0, ui.KeyM)
	assertSoundIDs(t, "mute", rec.take())
	tt.KeyDown(ui.KeyM)
	tt.KeyRepeat(0, ui.KeyM)
	tt.KeyRepeat(0, ui.KeyM)
	tt.KeyUp(ui.KeyM)
	if !app.soundEnabled {
		t.Fatal("expected M to unmute")
	}
	assertSoundIDs(t, "unmute held", rec.take(), SoundToggle)
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func TestSoundSetlistPopoverOpenClose(t *testing.T) {
	app, rec := newRecordedApp(7, defaultPrefs())
	tt := ui.NewTester(app.view, 1280, 900)

	must(t, tt.Click("歌单"))
	assertSoundIDs(t, "open", rec.take(), SoundPopoverOpen)
	must(t, tt.Click("歌单"))
	assertSoundIDs(t, "close by trigger", rec.take(), SoundPopoverClose)
	must(t, tt.Click("歌单"))
	tt.Key(0, ui.KeyEscape)
	assertSoundIDs(t, "open then Escape", rec.take(), SoundPopoverOpen, SoundPopoverClose)

	// Picking another track closes the popover once; the new score's notes
	// ride on that sound.
	must(t, tt.Click("歌单"))
	rec.take()
	must(t, tt.Click(Difficulties[1].Song))
	if app.diffIdx != 1 || app.hasOpenPopover() {
		t.Fatalf("diffIdx=%d open=%v", app.diffIdx, app.hasOpenPopover())
	}
	assertSoundIDs(t, "pick track", rec.take(), SoundPopoverClose)

	// Repeated entrypoint calls only sound on a real change.
	app.openPopover(popoverSettings)
	app.openPopover(popoverSettings)
	app.openPopover(popoverSetlist)
	app.dismissPopovers()
	app.dismissPopovers()
	app.closePopover(popoverSetlist, false)
	assertSoundIDs(t, "entrypoints", rec.take(), SoundPopoverOpen, SoundPopoverOpen, SoundPopoverClose)
}

func TestSoundChatNoteOncePerPost(t *testing.T) {
	app, rec := newRecordedApp(7, defaultPrefs())
	tt := ui.NewTester(app.view, 1280, 900)
	tt.Frame()
	assertSoundIDs(t, "opening notes", rec.take())

	// R posts two notes at once and sounds one.
	posted := app.chatPosted
	tt.Key(0, ui.KeyR)
	if app.chatPosted-posted < 2 {
		t.Fatalf("restart posted %d notes", app.chatPosted-posted)
	}
	assertSoundIDs(t, "restart", rec.take(), SoundChatNote)

	must(t, tt.Click(FindMember(MemberAnon).Name))
	assertSoundIDs(t, "switch leader", rec.take(), SoundChatNote)

	tt.Frame()
	tt.Advance(time.Second)
	assertSoundIDs(t, "idle frames", rec.take())

	// A note that lands in the same frame as an unrelated switch still sounds.
	app.pushChat("MyGO!!!!! 排练室", hexOf(bandColor), "note")
	tt.Key(0, ui.KeyF)
	assertSoundIDs(t, "note beside a switch", rec.take(), SoundToggle, SoundChatNote)

	// R with a popover open puts it away; the restart's notes ride on that.
	must(t, tt.Click("歌单"))
	rec.take()
	tt.Key(0, ui.KeyR)
	assertSoundIDs(t, "restart with popover", rec.take(), SoundPopoverClose)

	// Snapshot scenes re-stage the opening; it stays as quiet as startup.
	app.resetGame(true)
	tt.Frame()
	assertSoundIDs(t, "re-staged opening", rec.take())
}

func TestSoundDuelChatSilentInBackground(t *testing.T) {
	app, rec := newRecordedApp(42, defaultPrefs())
	app.game = NewGame(Difficulty{
		ID:           "duel-chat",
		Song:         "春日影",
		Gameplay:     GameplayDuel,
		Rows:         40,
		Cols:         40,
		Mines:        300,
		DuelDuration: 300,
		AIInterval:   1000,
	}, true, MemberTomori, 7)
	app.game.PlaceMines(20, 20)
	app.game.FirstClick = true
	app.game.Status = StatusPlaying
	// Leading keeps the band commenting every ten seconds.
	app.game.duel.PlayerScore = 100
	tt := ui.NewTester(app.view, 1280, 900)
	tt.Advance(11 * time.Second)
	rec.take()

	// The duel's timed chatter keeps posting while the window is away,
	// silently, and is not replayed when it comes back.
	app.setWindowFocused(false)
	posted := app.chatPosted
	tt.Advance(11 * time.Second)
	if app.chatPosted == posted {
		t.Fatal("expected a timed duel line while in the background")
	}
	assertSoundIDs(t, "background duel line", rec.take())
	app.setWindowFocused(true)
	tt.Frame()
	assertSoundIDs(t, "back in front", rec.take())

	posted = app.chatPosted
	tt.Advance(11 * time.Second)
	if app.chatPosted == posted {
		t.Fatal("expected a timed duel line in front")
	}
	assertSoundIDs(t, "foreground duel line", rec.take(), SoundChatNote)
}

// wallBoard stages a started 10×20 classic board cut by two columns of 雷,
// at cols 12 and 16, and settles its layout: a click in column 0 floods the
// left side, column 13 holds single number cells, and no skill can clear
// all three sides at once.
func wallBoard(tt *ui.Tester, app *minesweeperApp, charges int) *Game {
	g := NewGame(Difficulty{ID: "wall", Rows: 10, Cols: 20, Mines: 20, SkillCharges: charges}, app.bandMode(), app.memberID, 7)
	g.FirstClick = true
	g.Status = StatusPlaying
	for row := range g.Rows {
		g.Cell(row, 12).IsMine = true
		g.Cell(row, 16).IsMine = true
	}
	g.RecomputeAdjacency()
	app.game = g
	// Let the grid's cells settle to the new board's size.
	tt.Advance(time.Second)
	tt.Frame()
	return g
}

// cornerBoard stages a started 5×5 classic board whose only 雷 is cell 0-0,
// so clicking 4-4 wins and clicking 0-0 loses.
func cornerBoard(tt *ui.Tester, app *minesweeperApp, haruhikage bool) *Game {
	g := NewGame(Difficulty{ID: "corner", Rows: 5, Cols: 5, Mines: 1, Haruhikage: haruhikage}, app.bandMode(), app.memberID, 7)
	g.FirstClick = true
	g.Status = StatusPlaying
	g.Cell(0, 0).IsMine = true
	g.RecomputeAdjacency()
	app.game = g
	// Let the grid's cells settle to the new board's size.
	tt.Advance(time.Second)
	tt.Frame()
	return g
}

func newClockedApp(p prefs) (*minesweeperApp, *soundRecorder, *ui.Tester) {
	app, rec := newRecordedApp(7, p)
	tt := ui.NewTester(app.view, 1280, 900)
	rec.clock = tt.Now
	return app, rec, tt
}

// grainsOf returns the paper grains among cues with the times they played.
func grainsOf(rec *soundRecorder) ([]soundCue, []time.Time) {
	var cues []soundCue
	var at []time.Time
	for i, c := range rec.cues {
		if c.ID == SoundPaperLift {
			cues = append(cues, c)
			at = append(at, rec.at[i])
		}
	}
	return cues, at
}

func assertGrainGaps(t *testing.T, at []time.Time) {
	t.Helper()
	for i := 1; i < len(at); i++ {
		if gap := at[i].Sub(at[i-1]); gap < 38*time.Millisecond {
			t.Fatalf("grains %d and %d only %v apart", i-1, i, gap)
		}
	}
}

func TestSoundFloodGrainsThrottled(t *testing.T) {
	app, rec, tt := newClockedApp(defaultPrefs())
	g := wallBoard(tt, app, 0)
	tt.Frame()
	rec.take()

	// One flood lifts a primary hit plus follow-up grains throttled by
	// paperGrainGap and capped at paperGrainsPerWave.
	start := tt.Now()
	must(t, tt.Click("cell-9-0"))
	if !g.Cell(0, 0).Revealed || g.Cell(0, 13).Revealed {
		t.Fatal("expected the click to flood the left side at once")
	}
	tt.Advance(time.Second)

	grains, at := grainsOf(rec)
	if len(grains) < 2 || len(grains) > paperGrainsPerWave {
		t.Fatalf("one flood lifted %d grains, want 2 to %d", len(grains), paperGrainsPerWave)
	}
	if !at[0].Equal(start) {
		t.Fatalf("first grain at +%v, want at the click", at[0].Sub(start))
	}
	assertGrainGaps(t, at)
	for i, c := range grains {
		if c.Pan != columnPan(0, g.Cols) {
			t.Fatalf("grain %d pan %v, want the flood's column %v", i, c.Pan, columnPan(0, g.Cols))
		}
		if i > 0 && c.Speed <= grains[i-1].Speed {
			t.Fatalf("grain %d speed %v not above %v", i, c.Speed, grains[i-1].Speed)
		}
	}
	if grains[0].Speed != 1 || grains[len(grains)-1].Speed > 1.2 {
		t.Fatalf("pitch should rise gently from 1, got %v..%v", grains[0].Speed, grains[len(grains)-1].Speed)
	}
	// Ordinary big floods play touch plus paper-lift grains only; no member motif closes them.
	if len(rec.cues) != len(grains)+1 || rec.cues[0].ID != SoundPaperTouch {
		t.Fatalf("flood sounds = %v, want paper-touch plus paper-lift grains", soundIDs(rec.cues))
	}
	rec.take()

	// Quick single reveals spaced under paperGrainGap still sound every primary hit.
	firstAt := tt.Now()
	must(t, tt.Click("cell-2-13"))
	tt.Advance(10 * time.Millisecond)
	must(t, tt.Click("cell-4-13"))
	tt.Advance(30 * time.Millisecond)
	must(t, tt.Click("cell-6-13"))
	tt.Advance(time.Second)
	grains, at = grainsOf(rec)
	if len(grains) != 3 {
		t.Fatalf("quick reveals played %d grains (%v), want 3 primary hits", len(grains), at)
	}
	if at[0] != firstAt || at[1].Sub(at[0]) != 10*time.Millisecond || at[2].Sub(at[1]) != 30*time.Millisecond {
		t.Fatalf("quick reveal timestamps = %v from %v, want +0ms, +10ms, +40ms", at, firstAt)
	}
	for i, c := range grains {
		wantSpeed := min(revealStreakMaxSpeed, 1+float64(i)*revealStreakRise)
		if c.Pan != columnPan(13, g.Cols) || !gainNear(c.Speed, wantSpeed) {
			t.Fatalf("quick reveal %d = %+v, want col 13 pan at speed %v", i, c, wantSpeed)
		}
	}
	for _, row := range []int{2, 4, 6} {
		if !g.Cell(row, 13).Revealed {
			t.Fatalf("cell-%d-13 should be revealed", row)
		}
	}
	rec.take()

	// Mid-flood, a quick reveal elsewhere sounds its primary hit immediately
	// while the flood's follow-up grains stay throttled by paperGrainGap.
	g = wallBoard(tt, app, 0)
	tt.Frame()
	rec.take()
	start = tt.Now()
	must(t, tt.Click("cell-9-0"))
	tt.Advance(20 * time.Millisecond)
	must(t, tt.Click("cell-0-13"))
	if !g.Cell(0, 13).Revealed {
		t.Fatal("quick reveal mid-flood must open the cell")
	}
	tt.Advance(time.Second)
	grains, at = grainsOf(rec)
	if len(grains) < 3 || len(grains) > paperGrainsPerWave+1 {
		t.Fatalf("mid-flood reveal played %d grains, want 3 to %d", len(grains), paperGrainsPerWave+1)
	}
	if !at[0].Equal(start) || grains[0].Pan != columnPan(0, g.Cols) || grains[0].Speed != 1 {
		t.Fatalf("flood primary hit = %+v at +%v, want col 0 at +0ms speed 1", grains[0], at[0].Sub(start))
	}
	wantSecondSpeed := min(revealStreakMaxSpeed, 1+revealStreakRise)
	if at[1].Sub(start) != 20*time.Millisecond || grains[1].Pan != columnPan(13, g.Cols) || !gainNear(grains[1].Speed, wantSecondSpeed) {
		t.Fatalf("mid-flood primary hit = %+v at +%v, want col 13 at +20ms speed %v", grains[1], at[1].Sub(start), wantSecondSpeed)
	}
	assertGrainGaps(t, at[1:])
	for i := 2; i < len(grains); i++ {
		if grains[i].Pan != columnPan(0, g.Cols) || grains[i].Speed <= 1 {
			t.Fatalf("follow-up grain %d = %+v, want col 0 with rising speed", i, grains[i])
		}
		if i > 2 && grains[i].Speed <= grains[i-1].Speed {
			t.Fatalf("follow-up grain %d speed %v not above %v", i, grains[i].Speed, grains[i-1].Speed)
		}
	}
	if len(rec.cues) != len(grains)+2 {
		t.Fatalf("mid-flood sounds = %v, want two paper-touch cues plus paper-lift grains", soundIDs(rec.cues))
	}
	touches := 0
	for _, c := range rec.cues {
		switch c.ID {
		case SoundPaperTouch:
			touches++
		case SoundPaperLift:
		default:
			t.Fatalf("mid-flood sounds = %v, want only paper-touch and paper-lift cues", soundIDs(rec.cues))
		}
	}
	if touches != 2 {
		t.Fatalf("mid-flood touches = %d, want 2", touches)
	}
	rec.take()

	// A flood started under paperGrainGap after a single reveal sounds both
	// primary hits and still throttles its own follow-up wave grains.
	g = wallBoard(tt, app, 0)
	tt.Frame()
	rec.take()
	start = tt.Now()
	must(t, tt.Click("cell-0-13"))
	tt.Advance(10 * time.Millisecond)
	must(t, tt.Click("cell-9-0"))
	tt.Advance(time.Second)
	grains, at = grainsOf(rec)
	if len(grains) < 3 || len(grains) > paperGrainsPerWave+1 {
		t.Fatalf("single then flood played %d grains, want 3 to %d", len(grains), paperGrainsPerWave+1)
	}
	if !at[0].Equal(start) || grains[0].Pan != columnPan(13, g.Cols) || grains[0].Speed != 1 {
		t.Fatalf("first primary hit = %+v at +%v", grains[0], at[0].Sub(start))
	}
	if at[1].Sub(start) != 10*time.Millisecond || grains[1].Pan != columnPan(0, g.Cols) || !gainNear(grains[1].Speed, wantSecondSpeed) {
		t.Fatalf("flood primary hit = %+v at +%v, want speed %v", grains[1], at[1].Sub(start), wantSecondSpeed)
	}
	assertGrainGaps(t, at[1:])
	if grains[2].Speed <= grains[1].Speed {
		t.Fatalf("flood follow-up grain speed %v should rise above its primary hit %v", grains[2].Speed, grains[1].Speed)
	}
	if floodGrains := grains[1:]; len(floodGrains) > paperGrainsPerWave {
		t.Fatalf("flood lifted %d grains, want at most %d", len(floodGrains), paperGrainsPerWave)
	}
}

func TestSoundFloodDropsStaleGrains(t *testing.T) {
	app, rec := newRecordedApp(7, defaultPrefs())
	stalled := false
	tt := ui.NewTester(func(c *ui.Context) {
		if stalled {
			c.After(16 * time.Millisecond)
			return
		}
		app.view(c)
	}, 1280, 900)
	wallBoard(tt, app, 0)
	tt.Frame()
	rec.take()

	must(t, tt.Click("cell-9-0"))
	assertSoundIDs(t, "flood starts", rec.take(), SoundPaperTouch, SoundPaperLift)
	// A stall swallows the frames the rest of the wave was due in: those
	// grains are dropped, never played late.
	stalled = true
	tt.Advance(400 * time.Millisecond)
	stalled = false
	tt.Advance(time.Second)
	assertSoundIDs(t, "after the stall", rec.take())

	// So are those due while the window was hidden.
	wallBoard(tt, app, 0)
	tt.Advance(time.Second)
	rec.take()
	must(t, tt.Click("cell-9-0"))
	assertSoundIDs(t, "second flood starts", rec.take(), SoundPaperTouch, SoundPaperLift)
	tt.SetWindowHidden(true)
	tt.Advance(400 * time.Millisecond)
	tt.SetWindowHidden(false)
	tt.Advance(time.Second)
	assertSoundIDs(t, "after the window comes back", rec.take())
}

func TestSoundNewGameClearsQueuedGrains(t *testing.T) {
	for _, tc := range []struct {
		name string
		do   func(t *testing.T, tt *ui.Tester)
	}{
		{"R", func(t *testing.T, tt *ui.Tester) { tt.Key(0, ui.KeyR) }},
		{"track", func(t *testing.T, tt *ui.Tester) {
			must(t, tt.Click("歌单"))
			must(t, tt.Click(Difficulties[1].Song))
		}},
		{"mode", func(t *testing.T, tt *ui.Tester) { must(t, tt.Click("纯正经典")) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app, rec, tt := newClockedApp(defaultPrefs())
			wallBoard(tt, app, 0)
			tt.Frame()
			rec.take()
			must(t, tt.Click("cell-9-0"))
			assertSoundIDs(t, "flood starts", rec.take(), SoundPaperTouch, SoundPaperLift)
			tc.do(t, tt)
			tt.Advance(time.Second)
			for _, c := range rec.take() {
				if c.ID == SoundPaperLift || c.ID == SoundMotifTomori {
					t.Fatalf("%v sounded after the new game started", c.ID)
				}
			}
		})
	}
}

var wantMotif = map[MemberID]SoundID{
	MemberTomori:  SoundMotifTomori,
	MemberAnon:    SoundMotifAnon,
	MemberRana:    SoundMotifRana,
	MemberSoyorin: SoundMotifSoyo,
	MemberTaki:    SoundMotifTaki,
}

func TestSoundMotifsFollowLeaderInBothModes(t *testing.T) {
	for _, mode := range []int{0, 1} {
		for _, m := range Members {
			t.Run(fmt.Sprintf("%s/mode%d", m.ID, mode), func(t *testing.T) {
				motif := wantMotif[m.ID]
				stage := func() (*minesweeperApp, *soundRecorder, *ui.Tester) {
					app, rec, tt := newClockedApp(defaultPrefs())
					app.memberID, app.modeIdx = m.ID, mode
					return app, rec, tt
				}

				app, rec, tt := stage()
				g := wallBoard(tt, app, 0)
				must(t, tt.Click("cell-9-0"))
				if g.RevealedSafe < bigFloodCells {
					t.Fatalf("big flood revealed %d cells, want >= %d", g.RevealedSafe, bigFloodCells)
				}
				wantWash := memberImage(m.ID)
				washFound := false
				for _, rip := range app.ripples {
					if rip.Color == wantWash {
						washFound = true
						break
					}
				}
				if !washFound {
					t.Fatalf("big flood must keep %s's 代表色 wash in ripples", m.ID)
				}
				tt.Advance(time.Second)
				got := rec.take()
				if len(got) < 2 || got[0].ID != SoundPaperTouch {
					t.Fatalf("big flood sounds = %v, want paper-touch followed by paper-lift grains", soundIDs(got))
				}
				for _, c := range got[1:] {
					if c.ID != SoundPaperLift {
						t.Fatalf("big flood sounds = %v, want only paper-lift grains after touch with no closing motif", soundIDs(got))
					}
				}

				app, rec, tt = stage()
				cornerBoard(tt, app, false)
				must(t, tt.Click("cell-4-4"))
				if app.game.Status != StatusWon {
					t.Fatal("expected a win")
				}
				assertSoundIDs(t, "win", rec.take(), SoundPaperTouch, SoundPaperLift, motif, SoundWinChord)

				app, rec, tt = stage()
				g = cornerBoard(tt, app, false)
				must(t, tt.Click("cell-0-0"))
				if app.game.Status != StatusLost {
					t.Fatal("expected a loss")
				}
				lossGot := rec.take()
				assertSoundIDs(t, "loss", lossGot, SoundPaperTouch, SoundMineThud)
				if lossGot[0].Pan != columnPan(0, g.Cols) || lossGot[1].Pan != columnPan(0, g.Cols) {
					t.Fatalf("loss pans = (%v, %v), want %v", lossGot[0].Pan, lossGot[1].Pan, columnPan(0, g.Cols))
				}
				tt.Advance(time.Second)
				assertSoundIDs(t, "loss follow-up", rec.take())

				app, rec, tt = stage()
				cornerBoard(tt, app, true)
				must(t, tt.Click("cell-0-0"))
				assertSoundIDs(t, "春日影 loss", rec.take(), SoundPaperTouch, SoundMineThud, SoundHaruhikageStop)
				tt.Advance(time.Second)
				assertSoundIDs(t, "春日影 loss follow-up", rec.take())
			})
		}
	}
}

func TestSoundSkillMotifOnlyWhenItTakesEffect(t *testing.T) {
	for _, id := range []MemberID{MemberTomori, MemberAnon, MemberRana, MemberTaki} {
		app, rec, tt := newClockedApp(defaultPrefs())
		app.memberID = id
		g := wallBoard(tt, app, 2)
		rec.take()
		posted := app.chatPosted
		tt.Key(0, ui.KeyE)
		if g.SkillUsesLeft != 1 || app.chatPosted == posted {
			t.Fatalf("%s: expected the skill to take effect and post its line", id)
		}
		// The skill's line rides on its one motif, at once or as its flood
		// closes.
		tt.Advance(time.Second)
		assertSoundIDs(t, string(id)+" skill", rec.take(), wantMotif[id])
	}

	for _, tc := range []struct {
		name  string
		stage func(tt *ui.Tester, app *minesweeperApp) *Game
	}{
		{"before the first click", func(tt *ui.Tester, app *minesweeperApp) *Game {
			g := wallBoard(tt, app, 2)
			g.FirstClick, g.Status = false, StatusIdle
			return g
		}},
		{"no charge left", func(tt *ui.Tester, app *minesweeperApp) *Game { return wallBoard(tt, app, 0) }},
		{"纯正经典", func(tt *ui.Tester, app *minesweeperApp) *Game { app.modeIdx = 1; return wallBoard(tt, app, 2) }},
		{"Soyo's passive", func(tt *ui.Tester, app *minesweeperApp) *Game {
			app.memberID = MemberSoyorin
			return wallBoard(tt, app, 2)
		}},
	} {
		app, rec, tt := newClockedApp(defaultPrefs())
		g := tc.stage(tt, app)
		rec.take()
		before := *g
		tt.Key(0, ui.KeyE)
		if g.SkillUsesLeft != before.SkillUsesLeft || g.RevealedSafe != before.RevealedSafe || g.FlagsPlaced != before.FlagsPlaced {
			t.Fatalf("%s: empty E press changed the game", tc.name)
		}
		assertSoundIDs(t, tc.name, rec.take())
	}
}

// newDuelSoundApp starts a 100×100 duel at volume 80 whose AI moves once
// staged and whose timed chatter stays quiet.
func newDuelSoundApp() (*minesweeperApp, *soundRecorder, *ui.Tester) {
	p := defaultPrefs()
	p.Volume = 80
	app, rec, tt := newClockedApp(p)
	app.game = NewGame(Difficulty{
		ID:           "duel-cues",
		Gameplay:     GameplayDuel,
		Rows:         100,
		Cols:         100,
		Mines:        10,
		DuelDuration: 300,
		AIInterval:   1000,
	}, true, MemberTomori, 7)
	app.game.FirstClick = true
	app.game.Status = StatusPlaying
	app.game.duel.NextAI = app.game.duel.Duration + 1
	app.duelNoticeAt = tt.Now().Add(time.Hour)
	tt.Frame()
	rec.take()
	return app, rec, tt
}

// stageAITurn makes (row, col) 高松灯's next move: every other cell is the
// player's, and a 雷 lies beside it, or under it when mine is set.
func stageAITurn(g *Game, row, col int, mine bool) {
	for i := range g.Cells {
		c := &g.Cells[i]
		*c = Cell{Row: c.Row, Col: c.Col, Owner: DuelPlayer}
	}
	g.Cell(row, col).Owner = DuelNobody
	if mine {
		g.Cell(row, col).IsMine = true
	} else {
		g.Cell(row, col+1).IsMine = true
	}
	g.RecomputeAdjacency()
	g.duel.frontier = map[int]bool{row*g.Cols + col: true}
	g.duel.NextAI = g.duel.Elapsed
}

// stageAIFlag makes 高松灯's next move a lone flag on the 雷 at (row, col),
// which the revealed 1 to its left gives away.
func stageAIFlag(g *Game, row, col int) {
	stageAITurn(g, row, col, true)
	g.Cell(row, col-1).Revealed = true
}

func gainNear(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestSoundDuelAIPenOnlyInView(t *testing.T) {
	app, rec, tt := newDuelSoundApp()
	g := app.game

	stageAITurn(g, 50, 50, false)
	app.board.center(g, 50, 50)
	tt.Advance(100 * time.Millisecond)
	if !g.Cell(50, 50).Revealed || g.Cell(50, 50).Owner != DuelAI {
		t.Fatal("expected 高松灯 to open 50-50")
	}
	got := rec.take()
	assertSoundIDs(t, "AI opens a cell in view", got, SoundDuelPen)
	if !gainNear(got[0].Gain, 0.25*0.8) || got[0].Pan != columnPan(50, g.Cols) {
		t.Fatalf("pen cue = %+v, want a quarter of volume 80 panned to column 50", got[0])
	}

	stageAITurn(g, 90, 90, false)
	app.board.center(g, 10, 10)
	tt.Advance(100 * time.Millisecond)
	if !g.Cell(90, 90).Revealed {
		t.Fatal("expected 高松灯 to open 90-90")
	}
	assertSoundIDs(t, "AI opens a cell out of view", rec.take())

	stageAIFlag(g, 50, 50)
	app.board.center(g, 50, 50)
	tt.Advance(100 * time.Millisecond)
	if c := g.Cell(50, 50); !c.Flagged || c.FlagOwner != DuelAI {
		t.Fatal("expected 高松灯 to flag 50-50")
	}
	assertSoundIDs(t, "AI flags a cell in view", rec.take())

	stageAITurn(g, 90, 90, true)
	app.board.center(g, 10, 10)
	tt.Advance(100 * time.Millisecond)
	if !g.Cell(90, 90).Exploded {
		t.Fatal("expected 高松灯 to hit the 雷 at 90-90")
	}
	got = rec.take()
	assertSoundIDs(t, "AI hits a 雷 out of view", got, SoundDuelMineCue)
	if !gainNear(got[0].Gain, 0.5*0.8) {
		t.Fatalf("雷 cue gain %v", got[0].Gain)
	}
}

func TestSoundDuelCountdownTicksWholeSeconds(t *testing.T) {
	app, rec, tt := newDuelSoundApp()
	d := app.game.duel
	d.Elapsed = d.Duration - 12.5
	tt.Advance(2 * time.Second)
	assertSoundIDs(t, "before the last ten seconds", rec.take())

	tt.Advance(10*time.Second + 400*time.Millisecond)
	ticks, at := rec.cues, rec.at
	if len(ticks) != countdownFrom {
		t.Fatalf("last ten seconds played %v, want %d ticks", soundIDs(ticks), countdownFrom)
	}
	for i, c := range ticks {
		if c.ID != SoundDuelTick || !gainNear(c.Gain, 0.5*0.8) {
			t.Fatalf("tick %d = %+v", i, c)
		}
		if i > 0 {
			if gap := at[i].Sub(at[i-1]); gap < 950*time.Millisecond || gap > 1050*time.Millisecond {
				t.Fatalf("ticks %d and %d are %v apart", i-1, i, gap)
			}
		}
	}
	rec.take()
	tt.Advance(time.Second)
	if app.game.Status != StatusWon {
		t.Fatal("expected the duel to settle")
	}
	// A 0 : 0 draw settles on the gentle closing cue without a member motif.
	assertSoundIDs(t, "settlement", rec.take(), SoundDuelClose)
}

func TestSoundDuelCountdownSkipsStalledSeconds(t *testing.T) {
	app, rec := newRecordedApp(7, func() prefs { p := defaultPrefs(); p.Volume = 80; return p }())
	stalled := false
	tt := ui.NewTester(func(c *ui.Context) {
		if stalled {
			c.After(16 * time.Millisecond)
			return
		}
		app.view(c)
	}, 1280, 900)
	app.game = NewGame(Difficulty{ID: "duel-stall", Gameplay: GameplayDuel, Rows: 100, Cols: 100, Mines: 10, DuelDuration: 300, AIInterval: 1000}, true, MemberTomori, 7)
	app.game.FirstClick = true
	app.game.Status = StatusPlaying
	d := app.game.duel
	d.NextAI = d.Duration + 1
	d.Elapsed = d.Duration - 10.5
	app.duelNoticeAt = tt.Now().Add(time.Hour)
	tt.Frame()
	rec.take()

	// Seconds a stall swallowed pass silently; the next one ticks on time.
	stalled = true
	tt.Advance(3 * time.Second)
	stalled = false
	tt.Advance(50 * time.Millisecond)
	assertSoundIDs(t, "stall across three seconds", rec.take())
	tt.Advance(time.Second)
	assertSoundIDs(t, "next second after the stall", rec.take(), SoundDuelTick)

	// So do those that passed while the window was hidden.
	tt.SetWindowHidden(true)
	tt.Advance(2 * time.Second)
	tt.SetWindowHidden(false)
	tt.Frame()
	assertSoundIDs(t, "window back after two seconds", rec.take())
	tt.Advance(time.Second)
	assertSoundIDs(t, "next second after the window is back", rec.take(), SoundDuelTick)
}

func TestSoundLateFrameKeepsGrainGap(t *testing.T) {
	app, rec := newRecordedApp(7, defaultPrefs())
	stalled := false
	tt := ui.NewTester(func(c *ui.Context) {
		if stalled {
			c.After(16 * time.Millisecond)
			return
		}
		app.view(c)
	}, 1280, 900)
	rec.clock = tt.Now
	wallBoard(tt, app, 0)
	rec.take()

	// A short stall makes the second grain sound late, still within its
	// lateness; the one after must keep the gap from it, not from its slot.
	must(t, tt.Click("cell-9-0"))
	stalled = true
	tt.Advance(60 * time.Millisecond)
	stalled = false
	tt.Advance(time.Second)
	grains, at := grainsOf(rec)
	if len(grains) < 2 {
		t.Fatalf("expected a late grain after the stall, got %v", soundIDs(rec.cues))
	}
	assertGrainGaps(t, at)
}

func TestSoundCanvasFloodGrains(t *testing.T) {
	app, rec := newRecordedApp(7, defaultPrefs())
	g := NewGame(Difficulty{ID: "canvas-wall", Rows: 40, Cols: 40, Mines: 40}, true, MemberTomori, 7)
	g.FirstClick = true
	g.Status = StatusPlaying
	for row := range g.Rows {
		g.Cell(row, 20).IsMine = true
	}
	g.RecomputeAdjacency()
	app.game = g
	tt := ui.NewTester(app.view, 1280, 900)
	rec.clock = tt.Now
	x, y := canvasCellAt(t, tt, app, "经典棋盘", 20, 5)
	// The board sits idle a while, so the last frame is well before the click.
	tt.Advance(200 * time.Millisecond)
	rec.take()
	start := tt.Now()
	tt.ClickAt(x, y)
	tt.Advance(time.Second)
	grains, at := grainsOf(rec)
	if len(grains) < 2 || len(grains) > 6 || !at[0].Equal(start) {
		t.Fatalf("canvas flood grains at %v from %v, want 2 to 6 starting at the click", at, start)
	}
	assertGrainGaps(t, at)
	if len(rec.cues) != len(grains)+1 || rec.cues[0].ID != SoundPaperTouch {
		t.Fatalf("canvas flood sounds = %v, want paper-touch plus paper-lift grains with no closing motif", soundIDs(rec.cues))
	}
}

func TestSoundClassicCanvasMineThudOnly(t *testing.T) {
	app, rec := newRecordedApp(7, defaultPrefs())
	g := NewGame(Difficulty{ID: "canvas-mine", Rows: 40, Cols: 40, Mines: 40}, true, MemberAnon, 7)
	g.FirstClick = true
	g.Status = StatusPlaying
	for row := range g.Rows {
		g.Cell(row, 20).IsMine = true
	}
	g.RecomputeAdjacency()
	app.game = g
	tt := ui.NewTester(app.view, 1280, 900)
	x, y := canvasCellAt(t, tt, app, "经典棋盘", 20, 20)
	rec.take()
	tt.ClickAt(x, y)
	if g.Status != StatusLost {
		t.Fatal("expected classic canvas mine click to lose")
	}
	got := rec.take()
	assertSoundIDs(t, "canvas mine hit", got, SoundPaperTouch, SoundMineThud)
	if got[0].Pan != columnPan(20, g.Cols) || got[1].Pan != columnPan(20, g.Cols) {
		t.Fatalf("canvas mine hit pans = (%v, %v), want %v", got[0].Pan, got[1].Pan, columnPan(20, g.Cols))
	}
	tt.Advance(time.Second)
	assertSoundIDs(t, "canvas mine hit follow-up", rec.take())
}

func TestSoundClassicMineHitClearsQueuedCues(t *testing.T) {
	for _, haruhikage := range []bool{false, true} {
		want := []SoundID{SoundPaperTouch, SoundMineThud}
		label := "classic"
		if haruhikage {
			want = []SoundID{SoundPaperTouch, SoundMineThud, SoundHaruhikageStop}
			label = "haruhikage"
		}

		t.Run(label+"/after big flood", func(t *testing.T) {
			app, rec, tt := newClockedApp(defaultPrefs())
			g := wallBoard(tt, app, 0)
			g.Difficulty.Haruhikage = haruhikage
			tt.Frame()
			rec.take()

			must(t, tt.Click("cell-9-0"))
			assertSoundIDs(t, "flood starts", rec.take(), SoundPaperTouch, SoundPaperLift)
			if len(app.cues.pending) == 0 {
				t.Fatal("expected follow-up flood grains queued")
			}

			tt.Advance(10 * time.Millisecond)
			must(t, tt.Click("cell-0-12"))
			if g.Status != StatusLost {
				t.Fatal("expected clicking mine to lose")
			}
			tt.Advance(time.Second)
			assertSoundIDs(t, "mine hit after flood", rec.take(), want...)
		})

		t.Run(label+"/after flooding skill", func(t *testing.T) {
			app, rec, tt := newClockedApp(defaultPrefs())
			app.memberID = MemberRana
			g := wallBoard(tt, app, 2)
			g.Difficulty.Haruhikage = haruhikage
			tt.Frame()
			rec.take()

			tt.Key(0, ui.KeyE)
			if g.RevealedSafe < bigFloodCells || g.Status != StatusPlaying {
				t.Fatalf("Rana's skill opened %d cells with status %v, want a big flood that plays on", g.RevealedSafe, g.Status)
			}
			assertSoundIDs(t, "skill fires", rec.take())
			if len(app.cues.pending) == 0 {
				t.Fatal("expected closing skill motif queued")
			}

			tt.Advance(10 * time.Millisecond)
			must(t, tt.Click("cell-0-12"))
			if g.Status != StatusLost {
				t.Fatal("expected clicking mine to lose")
			}
			tt.Advance(time.Second)
			assertSoundIDs(t, "mine hit after flooding skill", rec.take(), want...)
		})
	}
}

func TestSoundDuelPassiveCuesHushInBackground(t *testing.T) {
	for _, tc := range []struct {
		name  string
		want  SoundID
		stage func(app *minesweeperApp)
	}{
		{"pen", SoundDuelPen, func(app *minesweeperApp) {
			stageAITurn(app.game, 50, 50, false)
			app.board.center(app.game, 50, 50)
		}},
		{"雷 cue", SoundDuelMineCue, func(app *minesweeperApp) {
			stageAITurn(app.game, 50, 50, true)
		}},
		{"tick", SoundDuelTick, func(app *minesweeperApp) {
			// First 10.6 s left; the second time the clock runs on into 9.
			if d := app.game.duel; d.remaining() > 11 {
				d.Elapsed = d.Duration - 10.6
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app, rec, tt := newDuelSoundApp()
			tt.SetWindowActive(false)
			tt.Frame()
			tc.stage(app)
			tt.Advance(time.Second)
			assertSoundIDs(t, "in the background", rec.take())

			tt.SetWindowActive(true)
			tt.Frame()
			assertSoundIDs(t, "back in front", rec.take())

			tc.stage(app)
			tt.Advance(time.Second)
			assertSoundIDs(t, "next cue in front", rec.take(), tc.want)
		})
	}
}

func TestSoundDuelSettlement(t *testing.T) {
	for _, m := range Members {
		motif := wantMotif[m.ID]
		for _, tc := range []struct {
			name       string
			haruhikage bool
			player, ai int
			want       []SoundID
		}{
			{"win", false, 120, 80, []SoundID{motif, SoundWinChord}},
			{"draw", false, 100, 100, []SoundID{SoundDuelClose}},
			{"loss", false, 80, 120, []SoundID{SoundDuelClose}},
			{"春日影 win", true, 120, 80, []SoundID{motif, SoundWinChord}},
			{"春日影 draw", true, 100, 100, []SoundID{SoundDuelClose}},
			{"春日影 loss", true, 80, 120, []SoundID{SoundHaruhikageStop}},
		} {
			t.Run(fmt.Sprintf("%s/%s", m.ID, tc.name), func(t *testing.T) {
				app, rec, tt := newDuelSoundApp()
				app.memberID = m.ID
				app.game.MemberID = m.ID
				app.game.Difficulty.Haruhikage = tc.haruhikage
				d := app.game.duel
				d.PlayerScore, d.AIScore = tc.player, tc.ai
				d.Elapsed = d.Duration - 0.2
				tt.Advance(time.Second)
				if app.game.Status != StatusWon || !app.duelEnded {
					t.Fatal("expected the duel to settle on its clock")
				}
				got := rec.take()
				assertSoundIDs(t, tc.name, got, tc.want...)
				for i, c := range got {
					if !gainNear(c.Gain, 0.8) || c.Pan != 0 || c.Speed != 1 {
						t.Fatalf("%s cue %d = %+v, want gain 0.8 pan 0 speed 1", tc.name, i, c)
					}
				}
			})
		}
	}

	// Master volume and mute govern duel settlement cues as well.
	for _, tc := range []struct {
		name    string
		enabled bool
		volume  int
		want    []SoundID
		gain    float64
	}{
		{"muted", false, 80, nil, 0},
		{"volume 0", true, 0, nil, 0},
		{"volume 40", true, 40, []SoundID{SoundDuelClose}, 0.4},
	} {
		t.Run("master/"+tc.name, func(t *testing.T) {
			app, rec, tt := newDuelSoundApp()
			app.soundEnabled = tc.enabled
			app.volume = tc.volume
			d := app.game.duel
			d.PlayerScore, d.AIScore = 80, 120
			d.Elapsed = d.Duration - 0.2
			tt.Advance(time.Second)
			if app.game.Status != StatusWon || !app.duelEnded {
				t.Fatal("expected the duel to settle on its clock")
			}
			got := rec.take()
			assertSoundIDs(t, tc.name, got, tc.want...)
			for i, c := range got {
				if !gainNear(c.Gain, tc.gain) || c.Pan != 0 || c.Speed != 1 {
					t.Fatalf("%s cue %d = %+v, want gain %v pan 0 speed 1", tc.name, i, c, tc.gain)
				}
			}
		})
	}
}

func TestSoundSkillFloodMotifAtClose(t *testing.T) {
	app, rec, tt := newClockedApp(defaultPrefs())
	app.memberID = MemberRana
	g := wallBoard(tt, app, 2)
	rec.take()
	start := tt.Now()
	tt.Key(0, ui.KeyE)
	if opened := g.RevealedSafe; opened < bigFloodCells || g.Status != StatusPlaying {
		t.Fatalf("Rana's skill opened %d cells with status %v, want a big flood that plays on", opened, g.Status)
	}
	assertSoundIDs(t, "skill fires", rec.take())
	tt.Advance(time.Second)
	assertSoundIDs(t, "flood closes", rec.cues, SoundMotifRana)
	if at := rec.at[0].Sub(start); at <= 0 || at > 240*time.Millisecond {
		t.Fatalf("motif at +%v, want as the flood closes", at)
	}
}

func TestSoundDuelCountdownQuietOnResumeFrames(t *testing.T) {
	app, rec := newRecordedApp(7, func() prefs { p := defaultPrefs(); p.Volume = 80; return p }())
	stalled := false
	tt := ui.NewTester(func(c *ui.Context) {
		if stalled {
			c.After(16 * time.Millisecond)
			return
		}
		app.view(c)
	}, 1280, 900)
	app.game = NewGame(Difficulty{ID: "duel-resume", Gameplay: GameplayDuel, Rows: 100, Cols: 100, Mines: 10, DuelDuration: 300, AIInterval: 1000}, true, MemberTomori, 7)
	app.game.FirstClick = true
	app.game.Status = StatusPlaying
	d := app.game.duel
	d.NextAI = d.Duration + 1
	app.duelNoticeAt = tt.Now().Add(time.Hour)
	tt.Frame()

	// 9.10 s left, then a stall until about 6.95 s: the frame that catches
	// up crosses 7 just after it turned, and still stays quiet.
	d.Elapsed = d.Duration - 9.10
	tt.Frame()
	rec.take()
	stalled = true
	tt.Advance(2140 * time.Millisecond)
	stalled = false
	tt.Advance(20 * time.Millisecond)
	if left := d.remaining(); left >= 7 || left < 6.8 {
		t.Fatalf("resumed at %.2f s left, want just under 7", left)
	}
	assertSoundIDs(t, "stall from 9.10 s", rec.take())
	tt.Advance(time.Second)
	assertSoundIDs(t, "next second after the stall", rec.take(), SoundDuelTick)

	// A hide shorter than a quarter second across 5 s: the tick that brings
	// the window back does not sound it.
	d.Elapsed = d.Duration - 5.10
	tt.Frame()
	rec.take()
	tt.SetWindowHidden(true)
	tt.Advance(150 * time.Millisecond)
	tt.SetWindowHidden(false)
	tt.Frame()
	if left := d.remaining(); left >= 5 {
		t.Fatalf("back at %.2f s left, want past 5", left)
	}
	assertSoundIDs(t, "short hide across 5 s", rec.take())
	tt.Advance(time.Second)
	assertSoundIDs(t, "next second after the hide", rec.take(), SoundDuelTick)
}

func TestSoundQuickCanvasRevealsPlayEveryPrimaryHit(t *testing.T) {
	app, rec := newRecordedApp(7, defaultPrefs())
	g := NewGame(Difficulty{ID: "canvas-quick", Rows: 40, Cols: 40, Mines: 40}, true, MemberTomori, 7)
	g.FirstClick = true
	g.Status = StatusPlaying
	for row := range g.Rows {
		g.Cell(row, 20).IsMine = true
	}
	g.RecomputeAdjacency()
	app.game = g
	tt := ui.NewTester(app.view, 1280, 900)
	rec.clock = tt.Now

	r, ok := tt.Find("经典棋盘")
	if !ok {
		t.Fatal("经典棋盘 not on screen")
	}
	v := app.board
	v.center(g, 20, 13)
	tt.Frame()
	rec.take()
	pointOf := func(row, col int) (float32, float32) {
		return r.X + (float32(col)+0.5)*v.Cell - v.X, r.Y + (float32(row)+0.5)*v.Cell - v.Y
	}

	// Two single reveals under paperGrainGap apart both sound their primary hit.
	start := tt.Now()
	x1, y1 := pointOf(19, 21)
	x2, y2 := pointOf(21, 21)
	tt.ClickAt(x1, y1)
	tt.Advance(10 * time.Millisecond)
	tt.ClickAt(x2, y2)
	tt.Advance(time.Second)
	if !g.Cell(19, 21).Revealed || !g.Cell(21, 21).Revealed {
		t.Fatal("both canvas cells should be revealed")
	}
	grains, at := grainsOf(rec)
	if len(grains) != 2 {
		t.Fatalf("quick canvas reveals played %d grains (%v), want 2 primary hits", len(grains), at)
	}
	if at[0] != start || at[1].Sub(at[0]) != 10*time.Millisecond {
		t.Fatalf("canvas reveal timestamps = %v from %v, want +0ms and +10ms", at, start)
	}
	for i, c := range grains {
		wantSpeed := min(revealStreakMaxSpeed, 1+float64(i)*revealStreakRise)
		if c.Pan != columnPan(21, g.Cols) || !gainNear(c.Speed, wantSpeed) {
			t.Fatalf("canvas primary hit %d = %+v, want col 21 pan at speed %v", i, c, wantSpeed)
		}
	}
	rec.take()

	// A quick single reveal 10ms into a canvas flood still sounds its primary hit,
	// while the flood's follow-up grains stay throttled and capped.
	start = tt.Now()
	fx, fy := pointOf(20, 5)
	qx, qy := pointOf(23, 21)
	tt.ClickAt(fx, fy)
	tt.Advance(10 * time.Millisecond)
	tt.ClickAt(qx, qy)
	tt.Advance(time.Second)
	if !g.Cell(20, 5).Revealed || !g.Cell(23, 21).Revealed {
		t.Fatal("canvas flood and mid-flood reveal should both open")
	}
	grains, at = grainsOf(rec)
	if len(grains) < 3 || len(grains) > paperGrainsPerWave+1 {
		t.Fatalf("canvas mid-flood played %d grains, want 3 to %d", len(grains), paperGrainsPerWave+1)
	}
	if !at[0].Equal(start) || grains[0].Pan != columnPan(5, g.Cols) || grains[0].Speed != 1 {
		t.Fatalf("canvas flood primary hit = %+v at +%v", grains[0], at[0].Sub(start))
	}
	wantSecondSpeed := min(revealStreakMaxSpeed, 1+revealStreakRise)
	if at[1].Sub(start) != 10*time.Millisecond || grains[1].Pan != columnPan(21, g.Cols) || !gainNear(grains[1].Speed, wantSecondSpeed) {
		t.Fatalf("canvas quick reveal hit = %+v at +%v, want speed %v", grains[1], at[1].Sub(start), wantSecondSpeed)
	}
	assertGrainGaps(t, at[1:])
}

func TestSoundRevealStreakPitch(t *testing.T) {
	app, rec, tt := newClockedApp(defaultPrefs())
	wallBoard(tt, app, 0)
	tt.Frame()
	rec.take()

	// Consecutive single reveals within revealStreakWindow climb strictly until
	// revealStreakMaxSpeed and stay capped there.
	const streakClicks = 8
	for row := range streakClicks {
		if row > 0 {
			tt.Advance(200 * time.Millisecond)
		}
		must(t, tt.Click(fmt.Sprintf("cell-%d-13", row)))
	}
	grains, _ := grainsOf(rec)
	if len(grains) != streakClicks {
		t.Fatalf("streak played %d paper lifts, want %d", len(grains), streakClicks)
	}
	if grains[0].Speed != 1 {
		t.Fatalf("first reveal speed = %v, want 1", grains[0].Speed)
	}
	reachedCap := false
	for i := 1; i < len(grains); i++ {
		prev, cur := grains[i-1].Speed, grains[i].Speed
		if cur > revealStreakMaxSpeed {
			t.Fatalf("reveal %d speed %v exceeds cap %v", i, cur, revealStreakMaxSpeed)
		}
		if prev < revealStreakMaxSpeed {
			if cur <= prev {
				t.Fatalf("reveal %d speed %v did not rise above %v", i, cur, prev)
			}
		} else if cur != revealStreakMaxSpeed {
			t.Fatalf("reveal %d speed %v drifted from cap %v", i, cur, revealStreakMaxSpeed)
		}
		if cur == revealStreakMaxSpeed {
			reachedCap = true
		}
	}
	if !reachedCap {
		t.Fatalf("streak of %d reveals peaked at %v, never reached cap %v", streakClicks, grains[len(grains)-1].Speed, revealStreakMaxSpeed)
	}
	rec.take()

	// Pausing past revealStreakWindow resets to baseline; a follow-up right at
	// the window boundary starts climbing again.
	tt.Advance(revealStreakWindow + time.Millisecond)
	must(t, tt.Click("cell-8-13"))
	tt.Advance(revealStreakWindow)
	must(t, tt.Click("cell-9-13"))
	grains, _ = grainsOf(rec)
	if len(grains) != 2 {
		t.Fatalf("post-pause reveals played %d paper lifts, want 2", len(grains))
	}
	if grains[0].Speed != 1 {
		t.Fatalf("reveal after pause played at speed %v, want 1", grains[0].Speed)
	}
	if grains[1].Speed <= 1 || grains[1].Speed > revealStreakMaxSpeed {
		t.Fatalf("boundary follow-up played at speed %v, want within (1, %v]", grains[1].Speed, revealStreakMaxSpeed)
	}
	rec.take()

	// Restarting or switching track resets the streak even when the next reveal
	// lands well inside revealStreakWindow.
	for _, tc := range []struct {
		name string
		do   func(t *testing.T, tt *ui.Tester)
	}{
		{"restart", func(t *testing.T, tt *ui.Tester) { tt.Key(0, ui.KeyR) }},
		{"switch track", func(t *testing.T, tt *ui.Tester) {
			must(t, tt.Click("歌单"))
			must(t, tt.Click(Difficulties[1].Song))
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app, rec, tt := newClockedApp(defaultPrefs())
			wallBoard(tt, app, 0)
			tt.Frame()
			rec.take()

			must(t, tt.Click("cell-0-13"))
			tt.Advance(100 * time.Millisecond)
			must(t, tt.Click("cell-1-13"))
			if grains, _ := grainsOf(rec); len(grains) != 2 || grains[1].Speed <= 1 {
				t.Fatalf("expected elevated streak before %s, got %+v", tc.name, grains)
			}
			rec.take()

			tc.do(t, tt)
			tt.Advance(100 * time.Millisecond)
			must(t, tt.Click("cell-0-0"))
			grains, _ := grainsOf(rec)
			if len(grains) == 0 || grains[0].Speed != 1 {
				t.Fatalf("first reveal after %s = %+v, want primary hit at speed 1", tc.name, grains)
			}
		})
	}
}

func TestSoundInvalidClicksStaySilent(t *testing.T) {
	t.Run("classic grid", func(t *testing.T) {
		app, rec, tt := newClockedApp(defaultPrefs())
		g := wallBoard(tt, app, 0)
		tt.Frame()
		rec.take()

		// Non-interactive background, already-selected leader, and disabled skill button.
		tt.ClickAt(2, 2)
		assertSoundIDs(t, "empty background", rec.take())
		must(t, tt.Click(FindMember(app.memberID).Name))
		assertSoundIDs(t, "already-selected leader", rec.take())
		must(t, tt.Click("发动技能 (E) ×0"))
		assertSoundIDs(t, "disabled skill button", rec.take())

		// Open left side: cell-0-0 is revealed empty (0), cell-0-11 is an unchordable 2.
		must(t, tt.Click("cell-9-0"))
		tt.Advance(time.Second)
		rec.take()
		if !g.Cell(0, 0).Revealed || g.Cell(0, 0).Adjacent != 0 || !g.Cell(0, 11).Revealed || g.Cell(0, 11).Adjacent == 0 {
			t.Fatal("expected cell-0-0 empty and cell-0-11 numbered after flood")
		}

		must(t, tt.Click("cell-0-0"))
		assertSoundIDs(t, "left-click revealed empty", rec.take())
		must(t, tt.Click("cell-0-11"))
		assertSoundIDs(t, "left-click unchordable number", rec.take())
		must(t, tt.RightClick("cell-0-0"))
		assertSoundIDs(t, "right-click revealed empty", rec.take())
		must(t, tt.RightClick("cell-0-11"))
		assertSoundIDs(t, "right-click revealed number", rec.take())

		// Flag both mines beside cell-0-11: left-clicking a flag or an already-satisfied
		// number with no remaining hidden neighbors has no effect and stays silent.
		must(t, tt.RightClick("cell-0-12"))
		must(t, tt.RightClick("cell-1-12"))
		rec.take()
		must(t, tt.Click("cell-0-12"))
		assertSoundIDs(t, "left-click flagged cell", rec.take())
		must(t, tt.Click("cell-0-11"))
		assertSoundIDs(t, "left-click satisfied number with no hidden neighbors", rec.take())

		// Post-game clicks on won and lost boards stay silent.
		cornerBoard(tt, app, false)
		must(t, tt.Click("cell-4-4"))
		tt.Advance(time.Second)
		if app.game.Status != StatusWon {
			t.Fatal("expected won board")
		}
		rec.take()
		must(t, tt.Click("cell-0-0"))
		must(t, tt.RightClick("cell-0-0"))
		must(t, tt.Click("cell-4-4"))
		assertSoundIDs(t, "clicks after win", rec.take())

		cornerBoard(tt, app, false)
		must(t, tt.Click("cell-0-0"))
		tt.Advance(time.Second)
		if app.game.Status != StatusLost {
			t.Fatal("expected lost board")
		}
		rec.take()
		must(t, tt.Click("cell-4-4"))
		must(t, tt.RightClick("cell-4-4"))
		must(t, tt.Click("cell-0-0"))
		assertSoundIDs(t, "clicks after loss", rec.take())
	})

	t.Run("classic canvas", func(t *testing.T) {
		app, rec := newRecordedApp(7, defaultPrefs())
		g := NewGame(Difficulty{ID: "canvas-invalid", Rows: 40, Cols: 40, Mines: 40}, true, MemberTomori, 7)
		g.FirstClick = true
		g.Status = StatusPlaying
		for row := range g.Rows {
			g.Cell(row, 20).IsMine = true
		}
		g.RecomputeAdjacency()
		app.game = g
		tt := ui.NewTester(app.view, 1280, 900)

		x, y := canvasCellAt(t, tt, app, "经典棋盘", 20, 15)
		tt.ClickAt(x, y)
		tt.Advance(time.Second)
		rec.take()

		tt.ClickAt(x, y)
		assertSoundIDs(t, "canvas left-click revealed empty", rec.take())
		tt.RightClickAt(x, y)
		assertSoundIDs(t, "canvas right-click revealed empty", rec.take())

		nx, ny := canvasCellAt(t, tt, app, "经典棋盘", 20, 19)
		tt.ClickAt(nx, ny)
		assertSoundIDs(t, "canvas left-click unchordable number", rec.take())
		tt.RightClickAt(nx, ny)
		assertSoundIDs(t, "canvas right-click revealed number", rec.take())

		fx, fy := canvasCellAt(t, tt, app, "经典棋盘", 20, 20)
		tt.RightClickAt(fx, fy)
		rec.take()
		tt.ClickAt(fx, fy)
		assertSoundIDs(t, "canvas left-click flagged cell", rec.take())

		// Lose by opening a mine, then verify clicks on remaining cells stay silent.
		mx, my := canvasCellAt(t, tt, app, "经典棋盘", 19, 20)
		tt.ClickAt(mx, my)
		tt.Advance(time.Second)
		rec.take()
		rx, ry := canvasCellAt(t, tt, app, "经典棋盘", 21, 21)
		tt.ClickAt(rx, ry)
		tt.RightClickAt(rx, ry)
		assertSoundIDs(t, "canvas clicks after loss", rec.take())
	})

	t.Run("duel canvas", func(t *testing.T) {
		app, rec, tt := newDuelSoundApp()
		g := app.game

		// Right-clicking AI's flag does nothing and stays silent.
		stageAIFlag(g, 50, 50)
		app.board.center(g, 50, 50)
		tt.Advance(100 * time.Millisecond)
		if c := g.Cell(50, 50); !c.Flagged || c.FlagOwner != DuelAI {
			t.Fatal("expected AI flag at 50-50")
		}
		rec.take()
		ax, ay := canvasCellAt(t, tt, app, "决斗棋盘", 50, 50)
		tt.RightClickAt(ax, ay)
		assertSoundIDs(t, "duel right-click AI flag", rec.take())

		// Clicking during stun does nothing and stays silent.
		g.duel.StunnedUntil = g.duel.Elapsed + 2.4
		tt.Frame()
		rec.take()
		sx, sy := canvasCellAt(t, tt, app, "决斗棋盘", 52, 52)
		tt.ClickAt(sx, sy)
		tt.RightClickAt(sx, sy)
		assertSoundIDs(t, "duel clicks while stunned", rec.take())
		if g.Cell(52, 52).Revealed || g.Cell(52, 52).Flagged {
			t.Fatal("clicks while stunned must not alter the cell")
		}

		// Left-clicking a revealed 0 cell in duel stays silent.
		g.duel.StunnedUntil = 0
		g.Cell(45, 45).Revealed = true
		g.Cell(45, 45).Adjacent = 0
		tt.Frame()
		rec.take()
		zx, zy := canvasCellAt(t, tt, app, "决斗棋盘", 45, 45)
		tt.ClickAt(zx, zy)
		tt.RightClickAt(zx, zy)
		assertSoundIDs(t, "duel clicks on revealed empty", rec.take())

		// Clicks after duel settlement stay silent.
		g.duel.Elapsed = g.duel.Duration - 0.1
		tt.Advance(time.Second)
		if app.game.Status != StatusWon || !app.duelEnded {
			t.Fatal("expected duel settlement")
		}
		rec.take()
		tt.ClickAt(sx, sy)
		tt.RightClickAt(sx, sy)
		assertSoundIDs(t, "duel clicks after settlement", rec.take())
	})
}

func TestSoundPressTouchReleaseLiftAndCancel(t *testing.T) {
	p := defaultPrefs()
	p.Volume = 80

	t.Run("classic grid", func(t *testing.T) {
		app, rec, tt := newClockedApp(p)
		g := wallBoard(tt, app, 0)
		tt.Frame()
		rec.take()

		r := mustFindCell(t, tt, "cell-2-13")
		other := mustFindCell(t, tt, "cell-2-14")
		wantPan := columnPan(13, g.Cols)

		// Press arms and plays paper-touch immediately at the press timestamp;
		// moving inside the same cell stays quiet; releasing plays paper-lift at
		// the release timestamp.
		pressAt := tt.Now()
		tt.Press(r.X+4, r.Y+4)
		if g.Cell(2, 13).Revealed {
			t.Fatal("press must not reveal cell before release")
		}
		if len(rec.cues) != 1 || rec.cues[0].ID != SoundPaperTouch || !rec.at[0].Equal(pressAt) ||
			!gainNear(rec.cues[0].Gain, 0.8) || rec.cues[0].Pan != wantPan || rec.cues[0].Speed != 1 {
			t.Fatalf("grid press cue = %+v at %v (want paper-touch at %v, gain 0.8, pan %v, speed 1)",
				rec.cues, rec.at, pressAt, wantPan)
		}
		rec.take()

		tt.Advance(40 * time.Millisecond)
		tt.Move(r.X+r.W-4, r.Y+r.H-4)
		assertSoundIDs(t, "grid in-cell move", rec.take())

		tt.Advance(40 * time.Millisecond)
		releaseAt := tt.Now()
		tt.Release(r.X+r.W-4, r.Y+r.H-4)
		if !g.Cell(2, 13).Revealed {
			t.Fatal("same-cell release must reveal cell")
		}
		if len(rec.cues) != 1 || rec.cues[0].ID != SoundPaperLift || !rec.at[0].Equal(releaseAt) ||
			!gainNear(rec.cues[0].Gain, 0.8) || rec.cues[0].Pan != wantPan {
			t.Fatalf("grid release cue = %+v at %v (want paper-lift at %v, gain 0.8, pan %v)",
				rec.cues, rec.at, releaseAt, wantPan)
		}
		rec.take()

		// Leaving the cell (and even returning before release) cancels silently.
		rCancel := mustFindCell(t, tt, "cell-4-13")
		tt.Press(rCancel.X+rCancel.W/2, rCancel.Y+rCancel.H/2)
		assertSoundIDs(t, "grid cancel press", rec.take(), SoundPaperTouch)
		tt.Advance(30 * time.Millisecond)
		tt.Move(other.X+other.W/2, other.Y+other.H/2)
		tt.Move(rCancel.X+rCancel.W/2, rCancel.Y+rCancel.H/2)
		tt.Release(rCancel.X+rCancel.W/2, rCancel.Y+rCancel.H/2)
		if g.Cell(4, 13).Revealed {
			t.Fatal("canceled grid press must not reveal")
		}
		assertSoundIDs(t, "grid leave-and-return cancel", rec.take())

		// System cancel stays silent.
		tt.Press(rCancel.X+rCancel.W/2, rCancel.Y+rCancel.H/2)
		assertSoundIDs(t, "grid system cancel press", rec.take(), SoundPaperTouch)
		tt.CancelPress()
		if g.Cell(4, 13).Revealed {
			t.Fatal("CancelPress must not reveal")
		}
		assertSoundIDs(t, "grid system cancel", rec.take())

		// Edge column pan mapping on press and release.
		rEdge := mustFindCell(t, tt, "cell-0-19")
		tt.Press(rEdge.X+rEdge.W/2, rEdge.Y+rEdge.H/2)
		edgeTouch := rec.take()
		assertSoundIDs(t, "grid edge press", edgeTouch, SoundPaperTouch)
		if edgeTouch[0].Pan != maxSoundPan {
			t.Fatalf("right edge press pan = %v, want %v", edgeTouch[0].Pan, maxSoundPan)
		}
		tt.Release(rEdge.X+rEdge.W/2, rEdge.Y+rEdge.H/2)
		edgeLift := rec.take()
		if len(edgeLift) == 0 || edgeLift[0].ID != SoundPaperLift || edgeLift[0].Pan != maxSoundPan {
			t.Fatalf("right edge release = %+v, want paper-lift at pan %v", edgeLift, maxSoundPan)
		}
	})

	for _, isDuel := range []bool{false, true} {
		name := "classic canvas"
		label := "经典棋盘"
		if isDuel {
			name = "duel canvas"
			label = "决斗棋盘"
		}
		t.Run(name, func(t *testing.T) {
			app, rec, tt := newClockedApp(p)
			if isDuel {
				app.game = NewGame(Difficulty{
					ID:           "duel-press-sound",
					Song:         "春日影",
					Gameplay:     GameplayDuel,
					Rows:         40,
					Cols:         40,
					Mines:        2,
					DuelDuration: 300,
					AIInterval:   1e9,
				}, true, MemberTomori, 7)
				app.game.duel.NextAI = 1e9
				app.duelNoticeAt = tt.Now().Add(time.Hour)
			} else {
				app.game = NewGame(Difficulty{
					ID:       "classic-press-sound",
					Song:     "诗超绊",
					Gameplay: GameplayClassic,
					Rows:     40,
					Cols:     40,
					Mines:    2,
				}, true, MemberTomori, 7)
			}
			g := app.game
			g.FirstClick = true
			g.Status = StatusPlaying
			g.Cell(18, 18).IsMine = true
			g.Cell(39, 39).IsMine = true
			g.RecomputeAdjacency()
			tt.Frame()
			board, ok := tt.Find(label)
			if !ok {
				t.Fatalf("%s not found", label)
			}
			app.board.center(g, 20, 20)
			tt.Frame()
			rec.take()

			r := canvasCellRect(board, app.board, 19, 19)
			other := canvasCellRect(board, app.board, 19, 20)
			wantPan := columnPan(19, g.Cols)

			pressAt := tt.Now()
			tt.Press(r.X+2, r.Y+2)
			if g.Cell(19, 19).Revealed {
				t.Fatal("canvas press must not reveal before release")
			}
			if len(rec.cues) != 1 || rec.cues[0].ID != SoundPaperTouch || !rec.at[0].Equal(pressAt) ||
				!gainNear(rec.cues[0].Gain, 0.8) || rec.cues[0].Pan != wantPan || rec.cues[0].Speed != 1 {
				t.Fatalf("canvas press cue = %+v at %v (want paper-touch at %v, gain 0.8, pan %v, speed 1)",
					rec.cues, rec.at, pressAt, wantPan)
			}
			rec.take()

			tt.Advance(35 * time.Millisecond)
			tt.Move(r.X+r.W-2, r.Y+r.H-2)
			assertSoundIDs(t, "canvas in-cell move", rec.take())

			tt.Advance(45 * time.Millisecond)
			releaseAt := tt.Now()
			tt.Release(r.X+r.W-2, r.Y+r.H-2)
			if !g.Cell(19, 19).Revealed {
				t.Fatal("canvas same-cell release must reveal")
			}
			if len(rec.cues) != 1 || rec.cues[0].ID != SoundPaperLift || !rec.at[0].Equal(releaseAt) ||
				!gainNear(rec.cues[0].Gain, 0.8) || rec.cues[0].Pan != wantPan {
				t.Fatalf("canvas release cue = %+v at %v (want paper-lift at %v, gain 0.8, pan %v)",
					rec.cues, rec.at, releaseAt, wantPan)
			}
			rec.take()

			// Leave cell and return cancels silently.
			rCancel := canvasCellRect(board, app.board, 18, 19)
			tt.Press(rCancel.X+rCancel.W/2, rCancel.Y+rCancel.H/2)
			assertSoundIDs(t, "canvas cancel press", rec.take(), SoundPaperTouch)
			tt.Move(other.X+other.W/2, other.Y+other.H/2)
			tt.Move(rCancel.X+rCancel.W/2, rCancel.Y+rCancel.H/2)
			tt.Release(rCancel.X+rCancel.W/2, rCancel.Y+rCancel.H/2)
			if g.Cell(18, 19).Revealed {
				t.Fatal("canceled canvas press must not reveal")
			}
			assertSoundIDs(t, "canvas leave-and-return cancel", rec.take())

			// System cancel stays silent.
			tt.Press(rCancel.X+rCancel.W/2, rCancel.Y+rCancel.H/2)
			assertSoundIDs(t, "canvas system cancel press", rec.take(), SoundPaperTouch)
			tt.CancelPress()
			if g.Cell(18, 19).Revealed {
				t.Fatal("canvas CancelPress must not reveal")
			}
			assertSoundIDs(t, "canvas system cancel", rec.take())

			// Middle-button and Alt+left pan drags do not arm or play paper-touch.
			tt.PressButton(2, rCancel.X+rCancel.W/2, rCancel.Y+rCancel.H/2)
			tt.ReleaseButton(2, rCancel.X+rCancel.W/2, rCancel.Y+rCancel.H/2)
			lx, ly := rCancel.X-board.X+rCancel.W/2, rCancel.Y-board.Y+rCancel.H/2
			panNow := tt.Now()
			app.handleCanvasInput(ui.InputEvent{Kind: ui.InputPointerDown, Button: 0, Mods: ui.Alt, X: lx, Y: ly, Time: panNow}, panNow)
			app.handleCanvasInput(ui.InputEvent{Kind: ui.InputPointerUp, Button: 0, Mods: ui.Alt, X: lx, Y: ly, Time: panNow}, panNow)
			assertSoundIDs(t, "canvas pan drags", rec.take())
		})
	}
}

func TestSoundChordHeavyLiftAndInvalidNumberSilence(t *testing.T) {
	t.Run("classic grid single-step chord, flooding chord, win chord, keyboard chord, and invalid clicks", func(t *testing.T) {
		app, rec, tt := newClockedApp(defaultPrefs())
		g := wallBoard(tt, app, 0)
		// Reveal number (1,11) (Adjacent=3: mines at (0,12),(1,12),(2,12)).
		// Flag only two mines first -> unchordable number must stay silent on press and release.
		g.Cell(1, 11).Revealed = true
		g.RevealedSafe = 1
		g.ToggleFlag(0, 12, false)
		g.ToggleFlag(1, 12, false)
		tt.Frame()
		rec.take()

		rNum := mustFindCell(t, tt, "cell-1-11")
		tt.Press(rNum.X+rNum.W/2, rNum.Y+rNum.H/2)
		assertSoundIDs(t, "unchordable number press", rec.take())
		tt.Release(rNum.X+rNum.W/2, rNum.Y+rNum.H/2)
		assertSoundIDs(t, "unchordable number release", rec.take())

		// Flag the third mine so (1,11) becomes chordable and floods the left side.
		g.ToggleFlag(2, 12, false)
		tt.Frame()
		rec.take()

		wantPan := columnPan(11, g.Cols)
		pressAt := tt.Now()
		tt.Press(rNum.X+rNum.W/2, rNum.Y+rNum.H/2)
		if len(rec.cues) != 1 || rec.cues[0].ID != SoundPaperTouch || !rec.at[0].Equal(pressAt) || rec.cues[0].Pan != wantPan {
			t.Fatalf("chordable number press = %+v at %v, want paper-touch at pan %v", rec.cues, rec.at, wantPan)
		}
		rec.take()

		tt.Advance(60 * time.Millisecond)
		releaseAt := tt.Now()
		tt.Release(rNum.X+rNum.W/2, rNum.Y+rNum.H/2)
		if !g.Cell(0, 11).Revealed || !g.Cell(0, 0).Revealed {
			t.Fatal("chording cell-1-11 should flood the left side")
		}
		if len(rec.cues) != 1 || rec.cues[0].ID != SoundPaperLiftHeavy || !rec.at[0].Equal(releaseAt) || rec.cues[0].Pan != wantPan {
			t.Fatalf("chord primary hit = %+v at %v, want paper-lift-heavy at %v pan %v", rec.cues, rec.at, releaseAt, wantPan)
		}
		tt.Advance(time.Second)
		if len(rec.cues) < 2 {
			t.Fatalf("flooding chord played %v, want heavy primary hit plus follow-up paper-lift grains", soundIDs(rec.cues))
		}
		for i := 1; i < len(rec.cues); i++ {
			if rec.cues[i].ID != SoundPaperLift || rec.cues[i].Pan != wantPan || rec.cues[i].Speed <= rec.cues[i-1].Speed {
				t.Fatalf("chord follow-up grain %d = %+v after %+v, want rising paper-lift at pan %v",
					i, rec.cues[i], rec.cues[i-1], wantPan)
			}
		}
		assertGrainGaps(t, rec.at)
		rec.take()

		// Clicking the now-satisfied number with zero hidden neighbors stays silent on press and release.
		tt.Press(rNum.X+rNum.W/2, rNum.Y+rNum.H/2)
		tt.Release(rNum.X+rNum.W/2, rNum.Y+rNum.H/2)
		assertSoundIDs(t, "satisfied number with no hidden neighbors", rec.take())

		// Non-flooding chord (reveals numbered cells only) + keyboard Enter chord + win chord.
		g = cornerBoard(tt, app, false)
		// Only mine is (0,0). With (0,2) and (1,2) already open and (0,0) flagged,
		// chording (0,1) opens just (1,0) and (1,1): no flood and no win yet.
		g.Cell(0, 1).Revealed = true
		g.Cell(1, 2).Revealed = true
		g.Cell(0, 2).Revealed = true
		g.RevealedSafe = 3
		g.ToggleFlag(0, 0, false)
		tt.Frame()
		rec.take()

		r01 := mustFindCell(t, tt, "cell-0-1")
		r02 := mustFindCell(t, tt, "cell-0-2")
		// Focus cell-0-1 via canceled drag, then activate via keyboard Enter.
		tt.Press(r01.X+r01.W/2, r01.Y+r01.H/2)
		assertSoundIDs(t, "focus drag press", rec.take(), SoundPaperTouch)
		tt.Move(r02.X+r02.W/2, r02.Y+r02.H/2)
		tt.Release(r02.X+r02.W/2, r02.Y+r02.H/2)
		assertSoundIDs(t, "focus drag cancel", rec.take())
		tt.Key(0, ui.KeyEnter)
		got := rec.take()
		assertSoundIDs(t, "keyboard chord", got, SoundPaperLiftHeavy)
		if got[0].Pan != columnPan(1, g.Cols) {
			t.Fatalf("keyboard chord pan = %v, want %v", got[0].Pan, columnPan(1, g.Cols))
		}
		if !g.Cell(1, 0).Revealed || !g.Cell(1, 1).Revealed || g.Status != StatusPlaying {
			t.Fatalf("expected non-winning chord to open (1,0) and (1,1): status=%v", g.Status)
		}

		// Chord that wins the game uses paper-lift-heavy as primary hit, followed by win motif & chord.
		for i := range g.Cells {
			c := &g.Cells[i]
			if !c.IsMine && !(c.Row == 2 && c.Col == 2) && !c.Revealed {
				c.Revealed = true
				g.RevealedSafe++
			}
		}
		tt.Frame()
		rec.take()
		r11 := mustFindCell(t, tt, "cell-1-1")
		tt.ClickAt(r11.X+r11.W/2, r11.Y+r11.H/2)
		if g.Status != StatusWon {
			t.Fatalf("expected chord on (1,1) to win, status=%v", g.Status)
		}
		assertSoundIDs(t, "winning chord", rec.take(), SoundPaperTouch, SoundPaperLiftHeavy, wantMotif[app.memberID], SoundWinChord)
	})

	t.Run("classic canvas chord and invalid number click", func(t *testing.T) {
		app, rec, tt := newClockedApp(defaultPrefs())
		g := NewGame(Difficulty{
			ID:       "canvas-chord-sound",
			Song:     "诗超绊",
			Gameplay: GameplayClassic,
			Rows:     40,
			Cols:     40,
			Mines:    41,
		}, true, MemberTomori, 7)
		g.FirstClick = true
		g.Status = StatusPlaying
		g.Cell(18, 18).IsMine = true
		for row := range g.Rows {
			g.Cell(row, 25).IsMine = true
		}
		g.RecomputeAdjacency()
		g.Cell(19, 19).Revealed = true
		g.RevealedSafe = 1
		app.game = g
		tt.Frame()
		board, ok := tt.Find("经典棋盘")
		if !ok {
			t.Fatal("经典棋盘 not found")
		}
		app.board.center(g, 19, 19)
		tt.Frame()
		rec.take()

		numR := canvasCellRect(board, app.board, 19, 19)
		// Unchordable number (mine not flagged yet) stays silent on press and release.
		tt.Press(numR.X+numR.W/2, numR.Y+numR.H/2)
		tt.Release(numR.X+numR.W/2, numR.Y+numR.H/2)
		assertSoundIDs(t, "canvas unchordable number", rec.take())

		// Flag the mine and chord: press -> paper-touch, release -> paper-lift-heavy (+ flood grains).
		g.ToggleFlag(18, 18, false)
		tt.Frame()
		rec.take()
		wantPan := columnPan(19, g.Cols)
		tt.Press(numR.X+numR.W/2, numR.Y+numR.H/2)
		touchGot := rec.take()
		assertSoundIDs(t, "canvas chord press", touchGot, SoundPaperTouch)
		if touchGot[0].Pan != wantPan {
			t.Fatalf("canvas chord press pan = %v, want %v", touchGot[0].Pan, wantPan)
		}
		tt.Advance(50 * time.Millisecond)
		tt.Release(numR.X+numR.W/2, numR.Y+numR.H/2)
		primaryGot := rec.take()
		assertSoundIDs(t, "canvas chord release primary", primaryGot, SoundPaperLiftHeavy)
		if primaryGot[0].Pan != wantPan {
			t.Fatalf("canvas chord release pan = %v, want %v", primaryGot[0].Pan, wantPan)
		}
		tt.Advance(time.Second)
		for _, c := range rec.take() {
			if c.ID != SoundPaperLift || c.Pan != wantPan {
				t.Fatalf("canvas chord follow-up = %+v, want paper-lift at pan %v", c, wantPan)
			}
		}

		// Now all neighbors of (19,19) are revealed/flagged: clicking again stays silent.
		tt.Press(numR.X+numR.W/2, numR.Y+numR.H/2)
		tt.Release(numR.X+numR.W/2, numR.Y+numR.H/2)
		assertSoundIDs(t, "canvas satisfied number", rec.take())
	})

	t.Run("duel canvas claimable mine vs chordable number and invalid number click", func(t *testing.T) {
		app, rec, tt := newClockedApp(defaultPrefs())
		g := NewGame(Difficulty{
			ID:           "duel-chord-sound",
			Song:         "春日影",
			Gameplay:     GameplayDuel,
			Rows:         40,
			Cols:         40,
			Mines:        41,
			DuelDuration: 300,
			AIInterval:   1e9,
		}, true, MemberTomori, 7)
		g.duel.NextAI = 1e9
		g.FirstClick = true
		g.Status = StatusPlaying
		g.Cell(18, 18).IsMine = true
		for row := range g.Rows {
			g.Cell(row, 25).IsMine = true
		}
		g.RecomputeAdjacency()
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
		app.game = g
		app.duelNoticeAt = tt.Now().Add(time.Hour)
		tt.Frame()
		board, ok := tt.Find("决斗棋盘")
		if !ok {
			t.Fatal("决斗棋盘 not found")
		}
		app.board.center(g, 19, 19)
		tt.Frame()
		rec.take()

		// (18,19) is unchordable before mine (18,18) is flagged -> silent.
		chordR := canvasCellRect(board, app.board, 18, 19)
		tt.Press(chordR.X+chordR.W/2, chordR.Y+chordR.H/2)
		tt.Release(chordR.X+chordR.W/2, chordR.Y+chordR.H/2)
		assertSoundIDs(t, "duel unchordable number", rec.take())

		// (19,19) is HintClaimable: press plays paper-touch, release flags mine with bandage-on (not heavy lift).
		claimR := canvasCellRect(board, app.board, 19, 19)
		tt.Press(claimR.X+claimR.W/2, claimR.Y+claimR.H/2)
		assertSoundIDs(t, "duel claim press", rec.take(), SoundPaperTouch)
		tt.Release(claimR.X+claimR.W/2, claimR.Y+claimR.H/2)
		assertSoundIDs(t, "duel claim release", rec.take(), SoundBandageOn)
		if !g.Cell(18, 18).Flagged {
			t.Fatal("expected duel claim to flag (18,18)")
		}

		// Now (18,19) is HintChordable: press plays paper-touch, release plays paper-lift-heavy.
		wantPan := columnPan(19, g.Cols)
		tt.Press(chordR.X+chordR.W/2, chordR.Y+chordR.H/2)
		touchGot := rec.take()
		assertSoundIDs(t, "duel chord press", touchGot, SoundPaperTouch)
		if touchGot[0].Pan != wantPan {
			t.Fatalf("duel chord press pan = %v, want %v", touchGot[0].Pan, wantPan)
		}
		tt.Advance(50 * time.Millisecond)
		tt.Release(chordR.X+chordR.W/2, chordR.Y+chordR.H/2)
		primaryGot := rec.take()
		assertSoundIDs(t, "duel chord release primary", primaryGot, SoundPaperLiftHeavy)
		if primaryGot[0].Pan != wantPan {
			t.Fatalf("duel chord release pan = %v, want %v", primaryGot[0].Pan, wantPan)
		}
		if !g.Cell(17, 19).Revealed {
			t.Fatal("expected duel chord on (18,19) to reveal (17,19)")
		}
	})
}
