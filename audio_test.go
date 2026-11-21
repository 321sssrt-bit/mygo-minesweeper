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

	start := tt.Now()
	must(t, tt.Click("cell-9-0"))
	if !g.Cell(0, 0).Revealed || g.Cell(0, 13).Revealed {
		t.Fatal("expected the click to flood the left side at once")
	}
	// Mid-flood, a quick reveal elsewhere keeps clear of the grains in flight.
	tt.Advance(10 * time.Millisecond)
	must(t, tt.Click("cell-0-13"))
	if !g.Cell(0, 13).Revealed {
		t.Fatal("a reveal must take effect even when its grain is held back")
	}
	tt.Advance(time.Second)

	grains, at := grainsOf(rec)
	if len(grains) < 2 || len(grains) > 6 {
		t.Fatalf("one flood lifted %d grains, want 2 to 6", len(grains))
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

	// Quick single reveals share the gap; the next one past it sounds again.
	must(t, tt.Click("cell-2-13"))
	tt.Advance(10 * time.Millisecond)
	must(t, tt.Click("cell-4-13"))
	tt.Advance(30 * time.Millisecond)
	must(t, tt.Click("cell-6-13"))
	tt.Advance(time.Second)
	grains, at = grainsOf(rec)
	if len(grains) != 2 || at[1].Sub(at[0]) != 40*time.Millisecond {
		t.Fatalf("quick reveals played %d grains at %v, want the first and the third", len(grains), at)
	}
	for _, row := range []int{2, 4, 6} {
		if !g.Cell(row, 13).Revealed {
			t.Fatalf("cell-%d-13 should be revealed", row)
		}
	}
	if grains[0].Pan != columnPan(13, g.Cols) {
		t.Fatalf("single reveal pan %v", grains[0].Pan)
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
		// The skill's line rides on its motif.
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
