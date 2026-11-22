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

	// One reveal lifts one slip, however far it floods.
	if err := tt.Click("cell-4-4"); err != nil {
		t.Fatal(err)
	}
	got := rec.take()
	assertSoundIDs(t, "reveal", got, SoundPaperLift)
	if !g.Cell(4, 4).Revealed {
		t.Fatal("cell-4-4 should be revealed")
	}
	if got[0].Pan != 0 {
		t.Fatalf("center column pan = %v", got[0].Pan)
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
	assertSoundIDs(t, "canvas reveal", rec.take(), SoundPaperLift)
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
	assertSoundIDs(t, "duel reveal", rec.take(), SoundPaperLift)
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
	assertSoundIDs(t, "shield", rec.take(), SoundBandageOn, SoundMotifSoyo)
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
	// The big flood closes on the leader's motif once its wave has spread.
	last := len(rec.cues) - 1
	if len(rec.cues) != len(grains)+1 || rec.cues[last].ID != SoundMotifTomori {
		t.Fatalf("flood sounds = %v, want grains then motif-tomori", soundIDs(rec.cues))
	}
	if motifAt := rec.at[last]; motifAt.Before(at[len(at)-1]) || motifAt.Sub(start) > 300*time.Millisecond {
		t.Fatalf("motif at +%v, want after the last grain and within the wave", motifAt.Sub(start))
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
	if last := rec.cues[len(rec.cues)-1]; last.ID != SoundMotifTomori {
		t.Fatalf("mid-flood sounds = %v, want closing motif-tomori", soundIDs(rec.cues))
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
	assertSoundIDs(t, "flood starts", rec.take(), SoundPaperLift)
	// A stall swallows the frames the rest of the wave was due in: those
	// grains and the closing motif are dropped, never played late.
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
	assertSoundIDs(t, "second flood starts", rec.take(), SoundPaperLift)
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
			assertSoundIDs(t, "flood starts", rec.take(), SoundPaperLift)
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
				wallBoard(tt, app, 0)
				must(t, tt.Click("cell-9-0"))
				tt.Advance(time.Second)
				if got := rec.take(); got[len(got)-1].ID != motif {
					t.Fatalf("big flood sounds = %v, want them to close on %v", soundIDs(got), motif)
				}

				app, rec, tt = stage()
				cornerBoard(tt, app, false)
				must(t, tt.Click("cell-4-4"))
				if app.game.Status != StatusWon {
					t.Fatal("expected a win")
				}
				assertSoundIDs(t, "win", rec.take(), SoundPaperLift, motif, SoundWinChord)

				app, rec, tt = stage()
				cornerBoard(tt, app, false)
				must(t, tt.Click("cell-0-0"))
				if app.game.Status != StatusLost {
					t.Fatal("expected a loss")
				}
				assertSoundIDs(t, "loss", rec.take(), SoundPaperLift, motif)

				app, rec, tt = stage()
				cornerBoard(tt, app, true)
				must(t, tt.Click("cell-0-0"))
				assertSoundIDs(t, "春日影 loss", rec.take(), SoundPaperLift, SoundHaruhikageStop)
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
	// A 0 : 0 draw settles on the leader's motif alone.
	assertSoundIDs(t, "settlement", rec.take(), SoundMotifTomori)
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
	if last := rec.cues[len(rec.cues)-1]; last.ID != SoundMotifTomori {
		t.Fatalf("canvas flood sounds = %v, want them to close on motif-tomori", soundIDs(rec.cues))
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
	for _, tc := range []struct {
		name       string
		haruhikage bool
		player, ai int
		want       []SoundID
	}{
		{"win", false, 120, 80, []SoundID{SoundMotifTomori, SoundWinChord}},
		{"loss", false, 80, 120, []SoundID{SoundMotifTomori}},
		{"春日影 loss", true, 80, 120, []SoundID{SoundHaruhikageStop}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app, rec, tt := newDuelSoundApp()
			app.game.Difficulty.Haruhikage = tc.haruhikage
			d := app.game.duel
			d.PlayerScore, d.AIScore = tc.player, tc.ai
			d.Elapsed = d.Duration - 0.2
			tt.Advance(time.Second)
			if app.game.Status != StatusWon || !app.duelEnded {
				t.Fatal("expected the duel to settle on its clock")
			}
			assertSoundIDs(t, tc.name, rec.take(), tc.want...)
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
