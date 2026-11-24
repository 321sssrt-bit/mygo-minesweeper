package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func requireSilentSound(t testing.TB, app *minesweeperApp) {
	t.Helper()
	if _, ok := app.sound.(silentSound); !ok {
		t.Fatalf("headless sound = %T, want silentSound", app.sound)
	}
}

func TestActiveFrameBenchmarkSmoke(t *testing.T) {
	// testing.Benchmark uses the global benchtime just like -bench does.
	// Keep this serial and restore the flag before the benchmark runner starts.
	benchTime := flag.Lookup("test.benchtime").Value
	old := benchTime.String()
	t.Cleanup(func() {
		if err := benchTime.Set(old); err != nil {
			t.Fatal(err)
		}
	})
	if err := benchTime.Set("3x"); err != nil {
		t.Fatal(err)
	}

	check := func(t *testing.T, run func(*testing.B), canvas bool) testing.BenchmarkResult {
		t.Helper()
		r := testing.Benchmark(run)
		if r.N != 3 || r.NsPerOp() <= 0 || r.Extra["ms/frame"] <= 0 ||
			r.AllocedBytesPerOp() <= 0 || r.AllocsPerOp() <= 0 ||
			(canvas && (r.Extra["staged-open-%"] <= 0 || r.Extra["staged-open-%"] >= 100)) {
			t.Fatalf("invalid benchmark metrics: %v %s", r, r.MemString())
		}
		t.Logf("%v %s", r, r.MemString())
		return r
	}
	results := make(map[string]testing.BenchmarkResult, len(activeCanvasTracks))
	for _, tc := range activeCanvasTracks {
		t.Run(tc.name, func(t *testing.T) {
			results[tc.name] = check(t, func(b *testing.B) { benchmarkActiveFrame(b, tc.track, true) }, true)
		})
	}
	t.Run("classic-grid", func(t *testing.T) {
		check(t, BenchmarkClassicGrid16x16ActiveFrame, false)
	})

	large := results["1000x1000"]
	for _, name := range []string{"100x100", "500x500"} {
		smaller := results[name]
		if smaller.N != 0 && large.N != 0 &&
			(large.AllocsPerOp() > 2*smaller.AllocsPerOp() || large.AllocedBytesPerOp() > 2*smaller.AllocedBytesPerOp()) {
			t.Errorf("1000x1000 %s exceeds twice the %s memory per frame (%s)", large.MemString(), name, smaller.MemString())
		}
	}
}

type convergenceView struct {
	t      *testing.T
	app    *minesweeperApp
	tt     *ui.Tester
	builds int
}

func newConvergenceView(t *testing.T, app *minesweeperApp) *convergenceView {
	t.Helper()
	v := &convergenceView{t: t, app: app}
	v.tt = ui.NewTester(func(c *ui.Context) {
		v.builds++
		app.view(c)
		// The toolkit's independent toast fades are outside these motion
		// cases. Close them on creation so the quiet window starts as soon
		// as the last app effect expires, even on loss and settlement.
		ui.ToastViewportBase(c, func(_ ui.Element, toasts []ui.Toast) {
			if len(toasts) != 0 {
				c.CloseToast("")
			}
		})
	}, 1080, 760)
	v.tt.Advance(10 * time.Millisecond)
	requireSilentSound(t, app)
	return v
}

func (v *convergenceView) click(label string) {
	v.t.Helper()
	if err := v.tt.Click(label); err != nil {
		v.t.Fatal(err)
	}
}

func (v *convergenceView) cell(row, col, button int) {
	v.t.Helper()
	if v.app.classicOnCanvas() {
		board, ok := v.tt.Find("经典棋盘")
		if !ok {
			v.t.Fatal("classic canvas missing")
		}
		r := classicCanvasCellRect(board, v.app.board, row, col)
		x, y := r.X+r.W/2, r.Y+r.H/2
		if button == 1 {
			v.tt.RightClickAt(x, y)
		} else {
			v.tt.ClickAt(x, y)
		}
		return
	}
	label := fmt.Sprintf("cell-%d-%d", row, col)
	if button == 1 {
		if err := v.tt.RightClick(label); err != nil {
			v.t.Fatal(err)
		}
	} else {
		v.click(label)
	}
}

func testerPaintCount(t testing.TB, tt *ui.Tester) uint64 {
	t.Helper()
	// mygo v0.9.2 exposes pending refreshes but not completed paints. Its
	// counter includes partial Painter.After repaints, which can bypass both
	// the app view and any separate Draw callback attached by this test.
	engine := reflect.ValueOf(tt).Elem().FieldByName("rt")
	if engine.Kind() == reflect.Pointer {
		if count := engine.Elem().FieldByName("paintFrames"); count.Kind() == reflect.Uint64 {
			return count.Uint()
		}
	}
	t.Fatal("mygo Tester paint counter moved; update paint-only convergence probe")
	return 0
}

func (v *convergenceView) converge(duration time.Duration) {
	v.t.Helper()
	pending, before, paints := v.tt.FramePending(), v.builds, testerPaintCount(v.t, v.tt)
	const probe = 40 * time.Millisecond
	v.tt.Advance(probe)
	v.t.Logf("active effect: pending=%v, %d paints and %d builds in 40 ms", pending, testerPaintCount(v.t, v.tt)-paints, v.builds-before)
	if testerPaintCount(v.t, v.tt)-paints < 2 {
		v.t.Fatal("active effect did not paint successive frames")
	}
	v.tt.Advance(duration - probe)
	if v.tt.FramePending() {
		v.t.Fatalf("animation frame still pending after %v", duration)
	}
}

func (v *convergenceView) quiet(status GameStatus) {
	v.t.Helper()
	if v.app.game.Status != status || v.tt.FramePending() {
		v.t.Fatalf("steady state: status=%v want %v, pending=%v", v.app.game.Status, status, v.tt.FramePending())
	}
	want := 0
	if status == StatusPlaying {
		phase := v.tt.Now().Sub(v.app.startedAt) % time.Second
		// Align the probe 100 ms after a stopwatch tick.
		align := (time.Second + 100*time.Millisecond - phase) % time.Second
		v.advanceQuiet(align, int((phase+align)/time.Second))
		v.advanceQuiet(400*time.Millisecond, 0)

		want = 3
	}
	elapsed := v.app.elapsed
	v.advanceQuiet(3*time.Second, want)
	if v.app.elapsed-elapsed != want {
		v.t.Fatalf("stopwatch advanced %d, want %d", v.app.elapsed-elapsed, want)
	}
}

func (v *convergenceView) advanceQuiet(d time.Duration, want int) {
	v.t.Helper()
	before, paints := v.builds, testerPaintCount(v.t, v.tt)
	v.tt.Advance(d)
	gotBuilds, gotPaints := v.builds-before, testerPaintCount(v.t, v.tt)-paints
	v.t.Logf("status=%v pending=%v, %d paints and %d builds in %v (want %d each)",
		v.app.game.Status, v.tt.FramePending(), gotPaints, gotBuilds, d, want)
	if gotBuilds != want || gotPaints != uint64(want) || v.tt.FramePending() {
		v.t.Fatal("unexpected frame activity in steady state")
	}
}

func convergenceClassic(side int, member MemberID) *minesweeperApp {
	app := newMinesweeperApp(7)
	app.memberID = member
	app.allowQuestion = false
	mines := make([][2]int, side)
	for row := range side {
		mines[row] = [2]int{row, side/2 + 2}
	}
	app.game = makeCorridorGame(side, side, true, member, mines)
	pinStartQuote(app)
	return app
}

func TestFrameConvergenceBoardEffects(t *testing.T) {
	t.Parallel()
	for _, backend := range []struct {
		name string
		side int
	}{{"grid", 9}, {"canvas", 31}} {
		t.Run(backend.name, func(t *testing.T) {
			side, mid := backend.side, backend.side/2
			t.Run("wave-and-cues", func(t *testing.T) {
				app := convergenceClassic(side, MemberTomori)
				v := newConvergenceView(t, app)
				v.cell(mid, mid-2, 0)
				if len(app.ripples) == 0 || len(app.cues.pending) == 0 {
					t.Fatal("reveal did not start both the wave and the grain queue")
				}
				v.converge(rippleMaxDelay + rippleDuration + 40*time.Millisecond)
				if len(app.ripples)+len(app.cues.pending) != 0 {
					t.Fatalf("wave or cues still queued: ripples=%d cues=%d", len(app.ripples), len(app.cues.pending))
				}
				v.quiet(StatusPlaying)
			})
			t.Run("bandage-bounce-and-peel", func(t *testing.T) {
				app := convergenceClassic(side, MemberTomori)
				v := newConvergenceView(t, app)
				idx := mid*side + mid
				for _, tc := range []struct {
					kind bandageAnimKind
					dur  time.Duration
				}{{bandageBounce, bandageBounceDuration}, {bandagePeel, bandagePeelDuration}} {
					v.cell(mid, mid, 1)
					if got := app.bandages[idx].Kind; got != tc.kind {
						t.Fatalf("bandage animation = %v, want %v", got, tc.kind)
					}
					v.converge(max(tc.dur, chatNoteEnterDuration) + 40*time.Millisecond)
					if len(app.bandages) != 0 {
						t.Fatal("bandage animation still queued")
					}
					v.quiet(StatusPlaying)
				}
			})
			for _, tc := range []struct {
				name   string
				member MemberID
				status GameStatus
			}{{"shield-shake", MemberSoyorin, StatusPlaying}, {"loss-shake-and-knot", MemberTomori, StatusLost}} {
				t.Run(tc.name, func(t *testing.T) {
					app := convergenceClassic(side, tc.member)
					v := newConvergenceView(t, app)
					v.cell(mid, mid+2, 0)
					if app.game.Status != tc.status || app.shakeAmplitude == 0 {
						t.Fatal("mine click did not produce the expected status and shake")
					}
					if tc.status == StatusLost && len(app.knotBursts) != 1 {
						t.Fatal("loss did not produce a knot burst")
					}
					// A mine on the first move can also unlock a commemorative strip.
					v.converge(memeStripDuration + 40*time.Millisecond)
					if app.shakeAmplitude != 0 || len(app.knotBursts) != 0 {
						t.Fatal("shake or knot burst did not expire")
					}
					v.quiet(tc.status)
				})
			}
		})
	}
}

func TestFrameConvergenceStageHighlights(t *testing.T) {
	for _, side := range []int{9, 31} {
		t.Run(fmt.Sprintf("classic-%dx%d", side, side), func(t *testing.T) {
			t.Run("skill-strip-and-pulses", func(t *testing.T) {
				app := convergenceClassic(side, MemberTaki)
				v := newConvergenceView(t, app)
				v.tt.Key(0, ui.KeyE)
				if !app.quoteStrip.Active || len(app.skillPulses) == 0 {
					t.Fatal("skill did not post a quote strip and pulse")
				}
				v.converge(quoteStripDuration + 40*time.Millisecond)
				if app.quoteStrip.Active || len(app.skillPulses) != 0 {
					t.Fatal("skill strip or pulses did not expire")
				}
				v.quiet(StatusPlaying)
			})
			t.Run("win-confetti-and-first-unlock", func(t *testing.T) {
				app := convergenceClassic(side, MemberTomori)
				mid := side / 2
				for i := range app.game.Cells {
					c := &app.game.Cells[i]
					if !c.IsMine && (c.Row != mid || c.Col != mid+1) {
						c.Revealed = true
						app.game.RevealedSafe++
					}
				}
				v := newConvergenceView(t, app)
				v.cell(mid, mid+1, 0)
				requireVictoryEffects(t, app)
				v.converge(memeStripDuration + 40*time.Millisecond)
				requireVictoryEffectsExpired(t, app)
				v.quiet(StatusWon)
			})
		})
	}
	t.Run("duel-win-confetti-and-first-unlock", func(t *testing.T) {
		app := newMinesweeperApp(7)
		diff := TrackSenzaihyoumei
		diff.Rows, diff.Cols, diff.DuelDuration = 9, 9, 1
		app.game = NewGame(diff, true, MemberTomori, 7)
		app.game.FirstClick, app.game.Status = true, StatusPlaying
		d := app.game.duel
		d.NextAI, d.PlayerScore, d.AIScore = 2, 200, 120
		v := newConvergenceView(t, app)
		v.tt.Advance(1100 * time.Millisecond)
		if !app.duelEnded || !d.Settled {
			t.Fatal("timed duel did not settle")
		}
		requireVictoryEffects(t, app)
		v.converge(memeStripDuration + 40*time.Millisecond)
		requireVictoryEffectsExpired(t, app)
		v.quiet(StatusWon)
	})
	t.Run("confetti-only", func(t *testing.T) {
		app := newMinesweeperApp(7)
		app.game = makeCorridorGame(5, 5, true, MemberTomori, [][2]int{{0, 0}})
		app.game.Reveal(4, 4)
		v := newConvergenceView(t, app)
		app.spawnConfetti(v.tt.Now())
		v.tt.Frame()
		if len(app.confetti) != confettiCount {
			t.Fatal("confetti did not start")
		}
		v.converge(confettiDuration + 40*time.Millisecond)
		requireVictoryEffectsExpired(t, app)
		v.quiet(StatusWon)
	})
	t.Run("first-unlock-strip-only", func(t *testing.T) {
		app := newMinesweeperApp(7)
		v := newConvergenceView(t, app)
		_, unlocked := app.recordMemeUnlock(MemeForever, v.tt.Now())
		v.tt.Frame()
		if !unlocked || !app.memeStrip.Active {
			t.Fatal("first unlock did not show the commemorative strip")
		}
		v.converge(memeStripDuration + 40*time.Millisecond)
		if app.memeStrip.Active {
			t.Fatal("commemorative strip did not expire")
		}
		v.quiet(StatusIdle)
	})
}

func requireVictoryEffects(t *testing.T, app *minesweeperApp) {
	t.Helper()
	if app.game.Status != StatusWon || len(app.confetti) != confettiCount || !app.memeStrip.Active {
		t.Fatalf("expected win, confetti and first-unlock strip, got status=%v confetti=%d strip=%v",
			app.game.Status, len(app.confetti), app.memeStrip.Active)
	}
}

func requireVictoryEffectsExpired(t *testing.T, app *minesweeperApp) {
	t.Helper()
	if len(app.confetti) != 0 || app.memeStrip.Active {
		t.Fatal("confetti or commemorative strip did not expire")
	}
}

func (v *convergenceView) toggleLamp() {
	v.t.Helper()
	sw, ok := v.tt.AccessByRole(ui.RoleSwitch, "夜间台灯")
	if !ok {
		v.t.Fatal("night lamp switch missing")
	}
	r := sw.Bounds
	v.tt.ClickAt(r.X+r.W/2, r.Y+r.H/2)
	if !v.app.darkMode || v.app.nightBlendStart.IsZero() {
		v.t.Fatal("night lamp did not start blending")
	}
}

func TestFrameConvergencePeripheral(t *testing.T) {
	t.Run("chat-note", func(t *testing.T) {
		app := newMinesweeperApp(7)
		v := newConvergenceView(t, app)
		app.pushChat("高松灯", FindMember(MemberTomori).ColorHex, "排练室便签")
		v.tt.Frame()
		if len(app.chatAnims) == 0 {
			t.Fatal("new note did not animate")
		}
		v.converge(chatNoteEnterDuration + 40*time.Millisecond)
		v.quiet(StatusIdle)
	})
	for _, tc := range []struct {
		label string
		kind  appPopover
	}{{"歌单", popoverSetlist}, {"设置", popoverSettings}} {
		t.Run(tc.label, func(t *testing.T) {
			app := newMinesweeperApp(7)
			v := newConvergenceView(t, app)
			v.click(tc.label)
			if app.popoverEnterKind != tc.kind {
				t.Fatal("popover did not start entering")
			}
			v.converge(popoverEnterDuration + 40*time.Millisecond)
			v.quiet(StatusIdle)

			v.tt.Key(0, ui.KeyEscape)
			if app.popoverExitKind != tc.kind {
				t.Fatal("popover did not start an exit afterimage")
			}
			v.converge(popoverExitDuration + 40*time.Millisecond)
			v.quiet(StatusIdle)
		})
	}
	t.Run("drawer-open-and-close", func(t *testing.T) {
		app := newMinesweeperApp(7)
		v := newConvergenceView(t, app)
		for _, wantOpen := range []bool{true, false} {
			v.click("名场面图鉴")
			if app.memeDrawerOpen != wantOpen || app.memeDrawerStart.IsZero() {
				t.Fatalf("drawer did not animate toward open=%v", wantOpen)
			}
			v.converge(drawerOpenDuration + 40*time.Millisecond)
			v.quiet(StatusIdle)
		}
	})
	t.Run("member-color", func(t *testing.T) {
		app := newMinesweeperApp(7)
		v := newConvergenceView(t, app)
		// Change only the selected member so chat and toast cannot mask
		// missing AnimateColor frame requests.
		app.memberID, app.game.MemberID = MemberAnon, MemberAnon
		v.tt.Frame()
		v.converge(memberHighlightDuration + 40*time.Millisecond)
		v.quiet(StatusIdle)
	})
	t.Run("night-lamp", func(t *testing.T) {
		app := newMinesweeperApp(7)
		app.darkMode = false
		v := newConvergenceView(t, app)
		v.tt.UseScreenReader()
		v.click("设置")
		v.tt.Advance(popoverEnterDuration + 40*time.Millisecond)
		v.toggleLamp()
		v.converge(lampTransitionDuration + 40*time.Millisecond)
		v.quiet(StatusIdle)
	})
	t.Run("both-meters", func(t *testing.T) {
		app := convergenceClassic(9, MemberTomori)
		v := newConvergenceView(t, app)
		// Change meter inputs without starting a reveal or a shield effect.
		app.game.Reveal(4, 2)
		v.tt.Frame()
		if app.progressVisual.start.IsZero() || app.gravityVisual.start.IsZero() {
			t.Fatal("reveal did not start both meter transitions")
		}
		v.converge(meterGlideDuration + 40*time.Millisecond)
		v.quiet(StatusPlaying)
	})
}

func TestFrameConvergenceCamera(t *testing.T) {
	app := newMinesweeperApp(7)
	app.diffIdx = findDifficultyIdx("duel")
	app.resetGame(true)
	v := newConvergenceView(t, app)
	g, board := app.game, app.board
	board.centerBoard(g)
	v.tt.Frame()

	for _, tc := range []struct {
		name    string
		trigger func()
	}{
		{"reset-standard-pitch", func() {
			board.Cell = 22
			v.tt.Frame()
			v.click("回标准大小")
		}},
		{"zoom-shortcut", func() { v.tt.Key(ui.Ctrl, ui.KeyMinus) }},
		{"return-to-center", func() {
			board.center(g, 500, 520)
			v.tt.Frame()
			v.click("回到中心")
		}},
		{"follow-tomori", func() {
			g.duel.LastAI = 515*g.Cols + 520
			v.click("定位高松灯")
		}},
		{"clamped-minimap-jump", func() {
			board.center(g, 0, 0)
			v.tt.Frame()
			r, ok := v.tt.Find("决斗棋盘")
			if !ok {
				v.t.Fatal("duel canvas missing")
			}
			mini := board.minimapBoundsFor(g)
			v.tt.ClickAt(r.X+mini.X+mini.W-2, r.Y+mini.Y+mini.H-2)
			spans := viewportSpansBetween(board.X, board.Y, board.glide.targetX, board.glide.targetY, board.Width, board.Height)
			if spans < 1.45 || spans > 1.55 {
				v.t.Fatalf("far minimap jump starts %.2f viewports from target, want 1.5", spans)
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v.t = t
			tc.trigger()
			if !board.gliding() {
				t.Fatal("camera glide did not start")
			}
			v.converge(300 * time.Millisecond)
			if board.gliding() {
				t.Fatal("camera still gliding")
			}
			v.quiet(StatusIdle)
		})
	}
}

func TestFrameConvergenceCombined(t *testing.T) {
	app := convergenceClassic(31, MemberSoyorin)
	app.darkMode = false
	// Use the next mine click for the shield and the member skill for a
	// distinct target pulse in the same game.
	v := newConvergenceView(t, app)
	v.tt.UseScreenReader()
	v.cell(15, 13, 0)
	v.cell(15, 18, 1)
	v.cell(15, 17, 0)
	// The roster UI restarts a live game. Stage the new skill actor in the
	// existing game so the effects above remain in flight.
	app.memberID, app.game.MemberID = MemberTaki, MemberTaki
	v.tt.Key(0, ui.KeyE)
	v.click("设置")
	v.toggleLamp()
	v.tt.Key(0, ui.KeyEscape)
	v.tt.Key(ui.Ctrl, ui.KeyEqual)

	if len(app.ripples) == 0 || len(app.bandages) == 0 || app.shakeAmplitude == 0 ||
		!app.quoteStrip.Active || len(app.skillPulses) == 0 || len(app.chatAnims) == 0 ||
		app.nightBlendStart.IsZero() || app.progressVisual.start.IsZero() || app.gravityVisual.start.IsZero() ||
		app.popoverExitKind != popoverSettings || !app.board.gliding() {
		t.Fatal("combined fixture did not leave every required effect in flight")
	}
	v.converge(memeStripDuration + 40*time.Millisecond)
	v.quiet(StatusPlaying)
}

func TestHeadlessEntryPointsUseSilentSound(t *testing.T) {
	requireSilentSound(t, newMinesweeperApp(7))
	requireSilentSound(t, newMinesweeperAppWith(appOptions{}))
	for _, scene := range snapshotScenes {
		t.Run(scene.name, func(t *testing.T) {
			for _, dark := range []bool{false, true} {
				requireSilentSound(t, scene.stage(dark))
			}
		})
	}
}

func TestDesktopAudioConstructionBoundary(t *testing.T) {
	fset := token.NewFileSet()
	err := filepath.WalkDir(".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() && path != "." && strings.HasPrefix(entry.Name(), ".") {
			return filepath.SkipDir
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return err
		}
		for _, violation := range desktopAudioViolations(fset, path, f) {
			t.Error(violation)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Log("desktop output restricted to main; adapter tests use sample data, a paused player and a substitute voice")
}

func desktopAudioViolations(fset *token.FileSet, path string, f *ast.File) []string {
	var violations []string
	report := func(at token.Pos, message string) {
		violations = append(violations, fmt.Sprintf("%s: %s", fset.Position(at), message))
	}
	for _, imp := range f.Imports {
		if p, _ := strconv.Unquote(imp.Path.Value); p == "github.com/egoist/mygo/audio" &&
			path != "audio.go" && path != "audio_test.go" {
			report(imp.Pos(), "desktop audio import outside the adapter")
		}
	}
	mainCalls := 0
	for _, decl := range f.Decls {
		fn, _ := decl.(*ast.FuncDecl)
		main := path == "main.go" && fn != nil && fn.Recv == nil && fn.Name.Name == "main"
		adapter := path == "audio.go" && fn != nil && fn.Recv == nil && fn.Name.Name == "newDesktopSound"
		adapterTest := path == "audio_test.go" && fn != nil && fn.Name.Name == "TestAudioDesktopAdapterSetsVoiceBeforePlay"
		playerTest := path == "audio_test.go" && fn != nil && fn.Name.Name == "TestAudioVoiceForwardsToPlayer"

		var local, constructor *ast.Ident
		var fakeVoiceAt token.Pos
		if adapterTest {
			local, constructor = desktopAdapterBinding(fn)
			if local == nil {
				report(fn.Pos(), "desktop sound must be bound by a top-level short declaration")
			}
			fakeVoiceAt = fakeVoiceInstallPos(fn, local)
			if !fakeVoiceAt.IsValid() {
				report(fn.Pos(), "adapter must install the fake voice on its own binding")
			}
		}

		checkWrite := func(expr ast.Expr) {
			if id, ok := expr.(*ast.Ident); ok && id != local && sameAdapterBinding(id, local) {
				report(id.Pos(), "desktop adapter binding must not be reassigned")
			}
		}
		ast.Inspect(decl, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.CallExpr:
				if id, ok := ast.Unparen(x.Fun).(*ast.Ident); main && ok && id.Name == "newDesktopSound" {
					mainCalls++
				}
			case *ast.Ident:
				if x.Name == "newDesktopSound" && !main && x != constructor && !(adapter && x == fn.Name) {
					report(x.Pos(), "desktop sound constructor is only allowed in main or the test's single binding")
				}
			case *ast.AssignStmt:
				for _, lhs := range x.Lhs {
					checkWrite(ast.Unparen(lhs))
				}
			case *ast.RangeStmt:
				checkWrite(x.Key)
				checkWrite(x.Value)
			case *ast.UnaryExpr:
				if x.Op == token.AND && sameAdapterBinding(ast.Unparen(x.X), local) {
					report(x.Pos(), "desktop adapter binding must not be addressed")
				}
			case *ast.KeyValueExpr:
				if key, ok := x.Key.(*ast.Ident); ok && key.Name == "Sound" && adapterTest {
					if !sameAdapterBinding(x.Value, local) {
						report(x.Value.Pos(), "Sound injection must use the same desktop adapter binding")
					} else if !fakeVoiceAt.IsValid() || fakeVoiceAt >= x.Pos() {
						report(x.Pos(), "adapter must install the fake voice before injection")
					}
				}
			case *ast.SelectorExpr:
				if (x.Sel.Name == "NewSound" || x.Sel.Name == "NewPlayer") && !adapter && !playerTest {
					report(x.Pos(), x.Sel.Name+" referenced outside the desktop adapter and its paused-player test")
				}
				if path == "audio_test.go" && x.Sel.Name == "Play" {
					report(x.Pos(), "playback must go through the test's substitute voice")
				}
			}
			return true
		})
	}
	if path == "main.go" && mainCalls != 1 {
		report(f.Package, fmt.Sprintf("main calls newDesktopSound %d times, want exactly 1", mainCalls))
	}
	return violations
}

func desktopAdapterBinding(fn *ast.FuncDecl) (local, constructor *ast.Ident) {
	for _, stmt := range fn.Body.List {
		assign, ok := stmt.(*ast.AssignStmt)
		if !ok || assign.Tok != token.DEFINE || len(assign.Lhs) != 1 || len(assign.Rhs) != 1 {
			continue
		}
		id, idOK := assign.Lhs[0].(*ast.Ident)
		call, callOK := assign.Rhs[0].(*ast.CallExpr)
		if !idOK || !callOK || id.Obj == nil {
			continue
		}
		if ctor, ok := call.Fun.(*ast.Ident); ok && ctor.Name == "newDesktopSound" {
			return id, ctor
		}
	}
	return nil, nil
}

func sameAdapterBinding(expr ast.Expr, local *ast.Ident) bool {
	id, ok := expr.(*ast.Ident)
	// Parser-resolved locals distinguish the original variable from shadows.
	return ok && local != nil && id.Obj == local.Obj
}

// The assignment must be unconditional and the only reference to the factory.
func fakeVoiceInstallPos(fn *ast.FuncDecl, local *ast.Ident) token.Pos {
	var installed token.Pos
	references := 0
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		if sel, ok := n.(*ast.SelectorExpr); ok && sel.Sel.Name == "newVoice" {
			references++
		}
		return true
	})
	if references != 1 {
		return token.NoPos
	}
	for _, stmt := range fn.Body.List {
		assign, ok := stmt.(*ast.AssignStmt)
		if !ok || assign.Tok != token.ASSIGN || len(assign.Lhs) != 1 || len(assign.Rhs) != 1 {
			continue
		}
		sel, ok := assign.Lhs[0].(*ast.SelectorExpr)
		factory, literal := assign.Rhs[0].(*ast.FuncLit)
		if !ok || sel.Sel.Name != "newVoice" || !literal || !sameAdapterBinding(sel.X, local) {
			continue
		}
		ast.Inspect(factory.Body, func(n ast.Node) bool {
			if value, ok := n.(*ast.CompositeLit); ok {
				if id, ok := value.Type.(*ast.Ident); ok && id.Name == "fakeVoice" {
					installed = assign.End()
				}
			}
			return true
		})
	}
	return installed
}

func TestDesktopAudioGuardRejectsUnsafeSource(t *testing.T) {
	const construct = "d := newDesktopSound(bank)\n"
	const install = "d.newVoice = func() soundVoice { return &fakeVoice{} }\n"
	const inject = "_ = appOptions{Sound: d}\n"
	const adapterTest = "package main\nfunc TestAudioDesktopAdapterSetsVoiceBeforePlay() {\n"
	for _, tc := range []struct {
		name, source, want string
	}{
		{"same-adapter", adapterTest + construct + install + inject + "}", ""},
		{"fresh-adapter", adapterTest + construct + install + "_ = appOptions{Sound: newDesktopSound(bank)}\n}", "same desktop adapter binding"},
		{"second-construction", adapterTest + construct + install + inject + "_ = newDesktopSound(bank)\n}", "test's single binding"},
		{"late-install", adapterTest + construct + inject + install + "}", "install the fake voice before injection"},
		{"shadow", adapterTest + construct + install + "{ d := other; " + inject + "}}", "same desktop adapter binding"},
		{"other-factory-receiver", adapterTest + construct + strings.Replace(install, "d.", "other.", 1) + inject + "}", "fake voice on its own binding"},
		{"reassign", adapterTest + construct + install + "d = other\n" + inject + "}", "must not be reassigned"},
		{"address", adapterTest + construct + install + "_ = &d\n" + inject + "}", "must not be addressed"},
		{"deferred-play", "package main\nfunc TestAudioVoiceForwardsToPlayer() { defer v.Play() }", "playback must go through the test's substitute voice"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fset := token.NewFileSet()
			const path = "audio_test.go"
			f, err := parser.ParseFile(fset, path, tc.source, 0)
			if err != nil {
				t.Fatal(err)
			}
			violations := desktopAudioViolations(fset, path, f)
			if tc.want == "" {
				if len(violations) != 0 {
					t.Fatalf("safe fixture rejected: %v", violations)
				}
				return
			}
			for _, violation := range violations {
				if strings.Contains(violation, tc.want) {
					t.Log(violation)
					return
				}
			}
			t.Fatalf("wanted violation %q, got %v", tc.want, violations)
		})
	}
}
