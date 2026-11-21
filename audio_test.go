package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/audio"
	"github.com/egoist/mygo/ui"
)

// soundRecorder is the in-memory output headless tests inject.
type soundRecorder struct{ cues []soundCue }

func (r *soundRecorder) Play(c soundCue) { r.cues = append(r.cues, c) }

// take returns the cues played since the last take.
func (r *soundRecorder) take() []soundCue {
	c := r.cues
	r.cues = nil
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
	assertSoundIDs(t, "shield", rec.take(), SoundBandageOn)
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
		{"夜间台灯", func() {
			sw, ok := tt.AccessByRole(ui.RoleSwitch, "夜间台灯")
			if !ok {
				t.Fatal("missing 夜间台灯 switch")
			}
			tt.ClickAt(sw.Bounds.X+sw.Bounds.W/2, sw.Bounds.Y+sw.Bounds.H/2)
		}},
		{"? 标记", func() { must(t, tt.Click("? 标记")) }},
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
