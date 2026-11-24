package main

import (
	"bytes"
	"fmt"
	"image"
	"image/draw"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// statusBarApp mounts a 9×9 classic track with mines only in the four
// corners, so cell-4-4 floods to a win and cell-0-0 loses. The game starts
// idle; startStatusBarGame moves it into play without moving any mine.
func statusBarApp(t *testing.T, member MemberID, w, h int) (*minesweeperApp, *ui.Tester) {
	t.Helper()
	app := newMinesweeperApp(42)
	app.memberID = member
	diff := Difficulties[0]
	diff.ID = "status-bar"
	diff.Mines = 4
	app.game = NewGame(diff, app.bandMode(), member, 42)
	app.game.FirstClick = true
	for _, rc := range [][2]int{{0, 0}, {0, 8}, {8, 0}, {8, 8}} {
		app.game.Cell(rc[0], rc[1]).IsMine = true
	}
	app.game.RecomputeAdjacency()
	tt := ui.NewTester(app.view, w, h)
	tt.UseScreenReader()
	tt.Frame()
	return app, tt
}

func startStatusBarGame(app *minesweeperApp, tt *ui.Tester) {
	app.game.Status = StatusPlaying
	tt.Frame()
}

func mustFind(t *testing.T, tt *ui.Tester, s string) ui.Rect {
	t.Helper()
	r, ok := tt.Find(s)
	if !ok {
		t.Fatalf("%q not on screen; texts: %q", s, tt.Texts())
	}
	return r
}

func rectInside(inner, outer ui.Rect) bool {
	const eps = 0.5
	return inner.X >= outer.X-eps && inner.Y >= outer.Y-eps &&
		inner.X+inner.W <= outer.X+outer.W+eps && inner.Y+inner.H <= outer.Y+outer.H+eps
}

// statusBarItems checks the bar holds 剩雷, 用时, 排练完成度, the skill entry
// and 重开 on one line, left to right, and returns the bar's box.
func statusBarItems(t *testing.T, app *minesweeperApp, tt *ui.Tester, skillLabel, resetLabel string) ui.Rect {
	t.Helper()
	bar := mustFind(t, tt, "状态栏")
	prog, ok := tt.AccessByRole(ui.RoleProgress, "排练完成度")
	if !ok {
		t.Fatal("status bar has no 排练完成度 progress node")
	}
	items := []struct {
		name string
		r    ui.Rect
	}{
		{"剩雷", mustFind(t, tt, "剩雷 "+formatDigits(app.game.RemainingMines()))},
		{"用时", mustFind(t, tt, "用时 "+formatDigits(app.elapsed)+" 秒")},
		{"排练完成度", prog.Bounds},
		{"技能", mustFind(t, tt, skillLabel)},
		{"重开", mustFind(t, tt, resetLabel)},
	}
	for i, it := range items {
		if !rectInside(it.r, bar) {
			t.Fatalf("%s %+v spills out of the status bar %+v", it.name, it.r, bar)
		}
		if i > 0 && it.r.X <= items[i-1].r.X+items[i-1].r.W {
			t.Fatalf("%s %+v is not right of %s %+v", it.name, it.r, items[i-1].name, items[i-1].r)
		}
	}
	return bar
}

func TestStatusBarSingleLineFixedHeightAcrossGame(t *testing.T) {
	for _, vp := range []struct {
		name string
		w, h int
	}{{"small", 960, 640}, {"default", 1280, 900}, {"large", 1680, 1050}} {
		t.Run(vp.name, func(t *testing.T) {
			app, tt := statusBarApp(t, MemberTomori, vp.w, vp.h)
			idle := statusBarItems(t, app, tt, "发动技能 (E) ×"+formatDigits(app.game.SkillUsesLeft), "迷子出发 (R)")

			startStatusBarGame(app, tt)
			must(t, tt.Click("cell-0-1"))
			if app.game.Status != StatusPlaying {
				t.Fatalf("status after numbered reveal = %v, want playing", app.game.Status)
			}
			playing := statusBarItems(t, app, tt, "发动技能 (E) ×"+formatDigits(app.game.SkillUsesLeft), "演奏中 · 重开 (R)")

			must(t, tt.Click("cell-4-4"))
			if app.game.Status != StatusWon {
				t.Fatalf("status after flood = %v, want won", app.game.Status)
			}
			won := statusBarItems(t, app, tt, "发动技能 (E) ×"+formatDigits(app.game.SkillUsesLeft), "Live 大成功！再来一首")

			lostApp, lostTT := statusBarApp(t, MemberTomori, vp.w, vp.h)
			startStatusBarGame(lostApp, lostTT)
			must(t, lostTT.Click("cell-0-0"))
			if lostApp.game.Status != StatusLost {
				t.Fatalf("status after mine = %v, want lost", lostApp.game.Status)
			}
			lost := statusBarItems(t, lostApp, lostTT, "发动技能 (E) ×"+formatDigits(lostApp.game.SkillUsesLeft), "炸团了 · 再排练一次")

			for name, r := range map[string]ui.Rect{"playing": playing, "won": won, "lost": lost} {
				if r.H != idle.H || r.Y != idle.Y {
					t.Errorf("%s status bar %+v moved or resized from idle %+v", name, r, idle)
				}
			}
		})
	}
}

func TestStatusBarDropsSetlistDuplicates(t *testing.T) {
	app, tt := statusBarApp(t, MemberTomori, 1280, 900)
	app.bestTimes[app.game.Difficulty.ID] = 23
	tt.Frame()
	bar := mustFind(t, tt, "状态栏")
	for _, s := range tt.Texts() {
		r, _ := tt.Find(s)
		if !rectInside(r, bar) {
			continue
		}
		if strings.Contains(s, "最快") || strings.Contains(s, app.game.Difficulty.Subtitle) {
			t.Errorf("status bar still shows setlist detail %q", s)
		}
	}
	tomori := FindMember(MemberTomori)
	if tt.HasText(tomori.SkillName + " · " + tomori.SkillDesc) {
		t.Error("member note with the skill tip is still mounted above the board")
	}
}

func TestStatusBarProgressReadsRealValue(t *testing.T) {
	app, tt := statusBarApp(t, MemberTomori, 1280, 900)
	startStatusBarGame(app, tt)
	bar := mustFind(t, tt, "状态栏")
	prog, ok := tt.AccessByRole(ui.RoleProgress, "排练完成度")
	if !ok || prog.Now != 0 || prog.Min != 0 || prog.Max != 1 || !rectInside(prog.Bounds, bar) {
		t.Fatalf("idle progress = %+v (ok=%v), want 0 of [0,1] inside bar %+v", prog, ok, bar)
	}
	for _, cell := range []string{"cell-0-1", "cell-1-0", "cell-1-1"} {
		must(t, tt.Click(cell))
	}
	want := app.game.Progress()
	if want <= 0 || want >= 1 {
		t.Fatalf("partial progress = %v, want in (0, 1)", want)
	}
	prog, ok = tt.AccessByRole(ui.RoleProgress, "排练完成度")
	if !ok || prog.Now != want {
		t.Fatalf("progress node = %+v (ok=%v), want Now=%v on the same frame", prog, ok, want)
	}
}

func TestStatusBarSkillEntryClickAndKey(t *testing.T) {
	for _, trigger := range []string{"click", "E"} {
		t.Run(trigger, func(t *testing.T) {
			app, tt := statusBarApp(t, MemberTomori, 1280, 900)
			startStatusBarGame(app, tt)
			must(t, tt.Click("cell-0-1"))
			before := app.game.SkillUsesLeft
			if before < 1 {
				t.Fatal("track has no skill charge")
			}
			if trigger == "click" {
				must(t, tt.Click("发动技能 (E) ×"+formatDigits(before)))
			} else {
				tt.Key(0, ui.KeyE)
			}
			if app.game.SkillUsesLeft != before-1 {
				t.Fatalf("%s left %d charges, want %d", trigger, app.game.SkillUsesLeft, before-1)
			}
		})
	}
}

func TestStatusBarQuotesMoveToChat(t *testing.T) {
	assertQuoteInChat := func(t *testing.T, app *minesweeperApp, tt *ui.Tester, speaker, when string) {
		t.Helper()
		if app.quote == "" || !slices.ContainsFunc(app.chatLog, func(m chatMessage) bool {
			return m.Speaker == speaker && m.Text == app.quote
		}) {
			t.Fatalf("%s: group chat %+v lacks %s saying %q", when, app.chatLog, speaker, app.quote)
		}
		if !tt.HasText(app.quote) {
			t.Fatalf("%s: quote %q not shown in the group chat", when, app.quote)
		}
	}

	app, tt := statusBarApp(t, MemberTomori, 1280, 900)
	must(t, tt.Click("迷子出发 (R)"))
	assertQuoteInChat(t, app, tt, "高松灯", "start")

	must(t, tt.Click("千早爱音"))
	if app.memberID != MemberAnon {
		t.Fatalf("leader = %v, want Anon", app.memberID)
	}
	assertQuoteInChat(t, app, tt, "千早爱音", "leader change")

	won, wonTT := statusBarApp(t, MemberTomori, 1280, 900)
	startStatusBarGame(won, wonTT)
	must(t, wonTT.Click("cell-4-4"))
	if won.game.Status != StatusWon {
		t.Fatalf("status = %v, want won", won.game.Status)
	}
	assertQuoteInChat(t, won, wonTT, "高松灯", "won")

	lost, lostTT := statusBarApp(t, MemberTomori, 1280, 900)
	startStatusBarGame(lost, lostTT)
	must(t, lostTT.Click("cell-0-0"))
	if lost.game.Status != StatusLost {
		t.Fatalf("status = %v, want lost", lost.game.Status)
	}
	assertQuoteInChat(t, lost, lostTT, "高松灯", "lost")
}

// duelStatusBarApp mounts the million-cell duel before its first move, with
// the board and AI dice pinned.
func duelStatusBarApp(t *testing.T, w, h int) (*minesweeperApp, *ui.Tester) {
	t.Helper()
	app := newMinesweeperApp(42)
	app.diffIdx = findDifficultyIdx("duel")
	app.resetGame(false)
	app.game.rng = rand.New(rand.NewPCG(42, 42^0x9e3779b97f4a7c15))
	app.game.duel.rng = rand.New(rand.NewPCG(7, 9))
	tt := ui.NewTester(app.view, w, h)
	tt.Frame()
	return app, tt
}

// duelStatusBarItems checks the bar holds both scores, then the countdown
// and combo (or the verdict once settled), the skill entry and 重开 on one
// line, left to right, with nothing else between the bar and the board.
func duelStatusBarItems(t *testing.T, app *minesweeperApp, tt *ui.Tester, middle []string, resetLabel string) ui.Rect {
	t.Helper()
	d := app.game.duel
	bar := mustFind(t, tt, "状态栏")
	labels := []string{fmt.Sprintf("你 %d", d.PlayerScore), fmt.Sprintf("高松灯 %d", d.AIScore)}
	labels = append(labels, middle...)
	labels = append(labels, fmt.Sprintf("发动技能 (E) ×%d", app.game.SkillUsesLeft), resetLabel)
	var prev ui.Rect
	for i, s := range labels {
		r := mustFind(t, tt, s)
		if !rectInside(r, bar) {
			t.Fatalf("%q %+v spills out of the status bar %+v", s, r, bar)
		}
		if i > 0 && r.X <= prev.X+prev.W {
			t.Fatalf("%q %+v is not right of %q %+v", s, r, labels[i-1], prev)
		}
		prev = r
	}
	for _, gone := range []string{"剩雷", "用时", "排练完成度", "插旗 ", "确定全雷时"} {
		for _, s := range tt.Texts() {
			if r, _ := tt.Find(s); strings.HasPrefix(s, gone) && rectInside(r, bar) {
				t.Errorf("duel status bar still shows classic item %q", s)
			}
		}
	}
	board := mustFind(t, tt, "决斗棋盘")
	if gap := board.Y - (bar.Y + bar.H); gap < 0 || gap > 10.5 {
		t.Errorf("board top %v is %v below the status bar bottom %v, want only the stage gap", board.Y, gap, bar.Y+bar.H)
	}
	return bar
}

func TestStatusBarDuelReadsScoresCountdownAndCombo(t *testing.T) {
	for _, vp := range []struct {
		name string
		w, h int
	}{{"small", 960, 640}, {"default", 1280, 900}, {"large", 1680, 1050}} {
		t.Run(vp.name, func(t *testing.T) {
			_, classicTT := statusBarApp(t, MemberTomori, vp.w, vp.h)
			classicBar := mustFind(t, classicTT, "状态栏")

			app, tt := duelStatusBarApp(t, vp.w, vp.h)
			d := app.game.duel
			idle := duelStatusBarItems(t, app, tt, []string{"剩余 05:00", "连击 ×0"}, "迷子出发 (R)")
			if idle.Y != classicBar.Y || idle.H != classicBar.H || idle.H != statusBarHeight {
				t.Fatalf("duel status bar %+v differs from the classic bar %+v", idle, classicBar)
			}

			board := mustFind(t, tt, "决斗棋盘")
			tt.ClickAt(board.X+board.W/2, board.Y+board.H/2)
			if app.game.Status != StatusPlaying || d.PlayerScore <= 0 || d.PlayerCombo != 1 {
				t.Fatalf("first click: status=%v score=%d combo=%d", app.game.Status, d.PlayerScore, d.PlayerCombo)
			}
			// The AI moves until it scores (flags score only at the
			// whistle), then is parked so the clock alone runs.
			for i := 0; i < 20 && d.AIScore == 0; i++ {
				d.NextAI = d.Elapsed
				tt.Advance(60 * time.Millisecond)
			}
			d.NextAI = 1e9
			if d.AIScore == 0 {
				t.Fatal("AI turn left its score at 0")
			}
			tt.Advance(2500 * time.Millisecond)
			clock := "剩余 " + formatClock(int(d.remaining()))
			if clock == "剩余 05:00" {
				t.Fatalf("countdown did not move: %s", clock)
			}
			playing := duelStatusBarItems(t, app, tt, []string{clock, "连击 ×1"}, "演奏中 · 重开 (R)")

			d.StunnedUntil = d.Elapsed + 2
			tt.Frame()
			if !tt.HasText(fmt.Sprintf("踩雷硬直中 · %.1f 秒", d.StunnedUntil-d.Elapsed)) {
				t.Fatal("stun time left the board banner")
			}
			stunned := duelStatusBarItems(t, app, tt, []string{"剩余 " + formatClock(int(d.remaining())), "连击 ×1"}, "演奏中 · 重开 (R)")
			d.StunnedUntil = 0

			d.AIScore = d.PlayerScore + 50
			d.Elapsed = d.Duration - 0.05
			tt.Advance(100 * time.Millisecond)
			if !app.duelEnded {
				t.Fatal("duel did not settle at the whistle")
			}
			settled := duelStatusBarItems(t, app, tt, []string{"惜败 · 这局别发 SNS"}, "决斗结束 · 再来一局 (R)")
			if tt.HasText("剩余 00:00") {
				t.Error("settled status bar still shows the countdown")
			}

			for name, r := range map[string]ui.Rect{"playing": playing, "stunned": stunned, "settled": settled} {
				if r != idle {
					t.Errorf("%s status bar %+v moved or resized from idle %+v", name, r, idle)
				}
			}
		})
	}
}

// The board's scan badge carries the scan countdown now that the duel rows
// above the board are gone: it shows up for a scan started from the status
// bar with the pointer off the board, ticks down, and leaves at the whistle.
func TestStatusBarDuelScanBadgeCountsDown(t *testing.T) {
	app, tt := duelStatusBarApp(t, 1280, 900)
	board := mustFind(t, tt, "决斗棋盘")
	tt.ClickAt(board.X+board.W/2, board.Y+board.H/2)
	d := app.game.duel
	d.NextAI = 1e9
	badge := image.Rect(int(board.X)+14, int(board.Y)+14, int(board.X)+120, int(board.Y)+44)
	crop := func() []byte {
		src := tt.Image()
		out := image.NewRGBA(image.Rect(0, 0, badge.Dx(), badge.Dy()))
		draw.Draw(out, out.Bounds(), src, badge.Min, draw.Src)
		return out.Pix
	}
	tt.Advance(1500 * time.Millisecond)
	plain := crop()

	must(t, tt.Click(fmt.Sprintf("发动技能 (E) ×%d", app.game.SkillUsesLeft)))
	if d.Elapsed >= d.ScanUntil {
		t.Fatal("skill did not start a scan")
	}
	tt.Advance(1500 * time.Millisecond)
	if _, hovered := app.game.ScanAt(app.board.HoverRow, app.board.HoverCol); hovered {
		t.Fatal("pointer is still over a board cell")
	}
	first := crop()
	if bytes.Equal(first, plain) {
		t.Fatal("no scan badge while scanning with the pointer off the board")
	}
	tt.Advance(1000 * time.Millisecond)
	if bytes.Equal(first, crop()) {
		t.Error("scan badge looks the same a second later; its countdown is not shown")
	}

	// A draw keeps the shot free of confetti and the 春日影 drama.
	d.AIScore = d.PlayerScore
	d.Elapsed = d.Duration - 0.05
	d.ScanUntil = d.Duration + 5
	tt.Advance(100 * time.Millisecond)
	if !app.duelEnded || d.Elapsed >= d.ScanUntil {
		t.Fatalf("want a settled duel inside the scan window: ended=%v elapsed=%v scanUntil=%v", app.duelEnded, d.Elapsed, d.ScanUntil)
	}
	if !bytes.Equal(plain, crop()) {
		t.Error("scan badge still drawn on the settled board")
	}
}
