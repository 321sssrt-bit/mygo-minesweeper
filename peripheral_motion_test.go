package main

import (
	"fmt"
	"image"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func uiColorNear(a, b ui.Color, tol int) bool {
	return absInt(int(a.R)-int(b.R)) <= tol &&
		absInt(int(a.G)-int(b.G)) <= tol &&
		absInt(int(a.B)-int(b.B)) <= tol
}

func hatchBarFilledExtent(img *image.RGBA, r ui.Rect, scale float32, ink ui.Color, tol int) int {
	x0 := max(img.Bounds().Min.X, int((r.X+2)*scale))
	x1 := min(img.Bounds().Max.X-1, int((r.X+r.W-2)*scale))
	y0 := max(img.Bounds().Min.Y, int((r.Y+2)*scale))
	y1 := min(img.Bounds().Max.Y-1, int((r.Y+r.H-2)*scale))
	maxX := -1
	for x := x0; x <= x1; x++ {
		for y := y0; y <= y1; y++ {
			if pixelNearColor(img.At(x, y), ink, tol) {
				maxX = x
				break
			}
		}
	}
	if maxX < 0 {
		return 0
	}
	return maxX - x0 + 1
}

func TestChatNoteStableKeysEnterShiftAndEviction(t *testing.T) {
	app := newMinesweeperApp(42)
	app.darkMode = false
	tt := ui.NewTester(app.view, 1280, 900)
	tt.Frame()

	// Opening notes sit steady on frame 0 with strictly increasing stable IDs.
	if len(app.chatLog) < 2 {
		t.Fatalf("expected >= 2 opening chat messages, got %d", len(app.chatLog))
	}
	for i := 1; i < len(app.chatLog); i++ {
		if app.chatLog[i].ID <= app.chatLog[i-1].ID {
			t.Fatalf("chat IDs not strictly increasing: %+v", app.chatLog)
		}
	}
	if len(app.chatAnims) != 0 {
		t.Fatalf("opening chat notes must not animate on frame 0, got %+v", app.chatAnims)
	}
	if tt.FramePending() {
		t.Fatal("initial idle screen must not leave pending animation frames")
	}

	olderText := app.chatLog[len(app.chatLog)-1].Text
	olderBefore, ok := tt.Find(olderText)
	if !ok {
		t.Fatalf("could not locate existing chat note %q", olderText)
	}

	// Push a runtime note: new note enters from top while older note slides smoothly down.
	app.pushChat("千早爱音", FindMember(MemberAnon).ColorHex, "排练室新贴上的便签！")
	tt.Frame()
	newID := app.chatLog[len(app.chatLog)-1].ID
	if _, active := app.chatAnims[newID]; !active {
		t.Fatalf("expected new chat note ID=%d to enter chatAnims", newID)
	}
	if !tt.FramePending() {
		t.Fatal("expected pending animation frames during chat note enter")
	}

	olderAt0, ok := tt.Find(olderText)
	if !ok {
		t.Fatalf("existing note %q missing at t=0", olderText)
	}
	tt.Advance(70 * time.Millisecond)
	olderMid, ok := tt.Find(olderText)
	if !ok {
		t.Fatalf("existing note %q missing at t=70ms", olderText)
	}
	tt.Advance(chatNoteEnterDuration)
	olderSettled, ok := tt.Find(olderText)
	if !ok {
		t.Fatalf("existing note %q missing after settle", olderText)
	}

	if !(olderAt0.Y < olderMid.Y && olderMid.Y < olderSettled.Y) {
		t.Fatalf("expected existing chat note to slide smoothly downward: before=%.2f t0=%.2f mid=%.2f settled=%.2f",
			olderBefore.Y, olderAt0.Y, olderMid.Y, olderSettled.Y)
	}
	if len(app.chatAnims) != 0 {
		t.Fatalf("expected chatAnims pruned after settle, remaining=%+v", app.chatAnims)
	}
	if tt.FramePending() {
		t.Fatal("expected FramePending()==false once chat note transition converges")
	}

	// Fill past the 18-message cap and push a 19th message: surviving notes must keep
	// their stable IDs and NEVER replay enter animations; only the single new ID animates.
	for i := range 20 {
		app.pushChat("高松灯", FindMember(MemberTomori).ColorHex, fmt.Sprintf("排练记录 #%d", i+1))
	}
	tt.Frame()
	tt.Advance(chatNoteEnterDuration + 30*time.Millisecond)
	if len(app.chatLog) != 18 || len(app.chatAnims) != 0 {
		t.Fatalf("expected settled 18-message log, len=%d anims=%d", len(app.chatLog), len(app.chatAnims))
	}
	survivingIDs := make([]int, 0, 17)
	for _, msg := range app.chatLog[1:] {
		survivingIDs = append(survivingIDs, msg.ID)
	}

	app.pushChat("椎名立希", FindMember(MemberTaki).ColorHex, "裁剪旧便签后只动画这一条")
	tt.Frame()
	newestID := app.chatLog[len(app.chatLog)-1].ID
	if len(app.chatAnims) != 1 {
		t.Fatalf("expected exactly 1 animating note after >18 eviction, got %+v", app.chatAnims)
	}
	if _, ok := app.chatAnims[newestID]; !ok {
		t.Fatalf("expected only newest ID=%d animating, got %+v", newestID, app.chatAnims)
	}
	for i, id := range survivingIDs {
		if app.chatLog[i].ID != id {
			t.Fatalf("surviving note index %d ID drifted: got %d want %d", i, app.chatLog[i].ID, id)
		}
		if _, replayed := app.chatAnims[id]; replayed {
			t.Fatalf("surviving note ID=%d unexpectedly replayed enter animation", id)
		}
	}
	tt.Advance(chatNoteEnterDuration + 30*time.Millisecond)
	if len(app.chatAnims) != 0 || tt.FramePending() {
		t.Fatalf("expected clean settle after eviction animation: anims=%d pending=%v", len(app.chatAnims), tt.FramePending())
	}
}

func TestPopoverExitMidwaySingleClickMutualSwitchAndShield(t *testing.T) {
	for _, tc := range []struct {
		name        string
		firstTrig   string
		firstKind   appPopover
		secondTrig  string
		secondKind  appPopover
		secondFocus string
	}{
		{"setlist-exiting-to-settings", "歌单", popoverSetlist, "设置", popoverSettings, "夜间台灯"},
		{"settings-exiting-to-setlist", "设置", popoverSettings, "歌单", popoverSetlist, "迷星叫"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := newMinesweeperApp(12345)
			app.darkMode = false
			tt := ui.NewTester(app.view, 1280, 900)
			tt.SetScale(2)
			tt.Frame()

			closedBaseline := image.NewRGBA(tt.Image().Bounds())
			copy(closedBaseline.Pix, tt.Image().Pix)

			// 1. Open first popover and settle its enter transition.
			if err := tt.Click(tc.firstTrig); err != nil {
				t.Fatalf("open %s: %v", tc.firstTrig, err)
			}
			tt.Advance(popoverEnterDuration + 20*time.Millisecond)
			if !app.popoverOpen(tc.firstKind) {
				t.Fatalf("expected %s open", tc.firstTrig)
			}

			// 2. Close via Escape and step midway into the real exit transition.
			tt.Key(0, ui.KeyEscape)
			if app.hasOpenPopover() {
				t.Fatalf("Escape did not start closing %s", tc.firstTrig)
			}
			tt.Advance(45 * time.Millisecond)
			if !app.popoverExiting(tt.Now()) || !tt.FramePending() {
				t.Fatalf("expected %s actively exiting at 45ms: exiting=%v pending=%v",
					tc.firstTrig, app.popoverExiting(tt.Now()), tt.FramePending())
			}
			// Visually confirm the exiting ghost is still fading over the closed baseline.
			fullRect := ui.Rect{X: 0, Y: 0, W: 1280, H: 900}
			if diff := diffPixelCountInRect(closedBaseline, tt.Image(), fullRect, 2); diff < 500 {
				t.Fatalf("expected visible exiting popover ghost at 45ms, pixel diff=%d", diff)
			}

			// 3. Mid-exit anti-leak shield: left-click and right-click on a board cell must NOT reveal or flag.
			cellRect, ok := tt.Find("cell-4-4")
			if !ok {
				t.Fatal("cell-4-4 missing")
			}
			tt.ClickAt(cellRect.X+cellRect.W/2, cellRect.Y+cellRect.H/2)
			tt.RightClickAt(cellRect.X+cellRect.W/2, cellRect.Y+cellRect.H/2)
			if app.game.Moves != 0 || app.game.FirstClick || app.game.FlagsPlaced != 0 {
				t.Fatalf("mid-exit board click leaked through shield: moves=%d first=%v flags=%d",
					app.game.Moves, app.game.FirstClick, app.game.FlagsPlaced)
			}

			// 4. Directly single-click the other trigger while the first popover is still mid-exit!
			if err := tt.Click(tc.secondTrig); err != nil {
				t.Fatalf("single-click %s mid-exit: %v", tc.secondTrig, err)
			}
			if !app.popoverOpen(tc.secondKind) || app.popoverOpen(tc.firstKind) {
				t.Fatalf("single-click mid-exit failed to open %s cleanly: active=%v", tc.secondTrig, app.activePopover)
			}
			if !tt.Focused(tc.secondFocus) {
				t.Fatalf("expected focus on %q after mid-exit single-click switch", tc.secondFocus)
			}

			// 5. Close second popover and confirm full convergence (FramePending() == false).
			tt.Key(0, ui.KeyEscape)
			if !tt.Focused(tc.secondTrig) {
				t.Fatalf("Escape did not restore focus to %s", tc.secondTrig)
			}
			tt.Advance(popoverExitDuration + 40*time.Millisecond)
			if app.hasOpenPopover() || app.popoverExiting(tt.Now()) || tt.FramePending() {
				t.Fatalf("popover exit did not settle cleanly: open=%v exiting=%v pending=%v",
					app.hasOpenPopover(), app.popoverExiting(tt.Now()), tt.FramePending())
			}
		})
	}
}

func TestMemeDrawerSmoothOpenCloseAndConvergence(t *testing.T) {
	app := newMinesweeperApp(42)
	tt := ui.NewTester(app.view, 1280, 900)
	tt.UseScreenReader()
	tt.Frame()
	if tt.FramePending() {
		t.Fatal("idle view with closed drawer must not request animation frames")
	}

	boardTopClosed, ok := tt.Find("迷子出发 (R)")
	if !ok {
		t.Fatal("reset button missing")
	}

	// Open drawer: board smoothly shifts downward while unfolding.
	if err := tt.Click("名场面图鉴"); err != nil {
		t.Fatalf("open meme drawer: %v", err)
	}
	if !app.showMemes || !tt.FramePending() {
		t.Fatalf("expected drawer animating open: showMemes=%v pending=%v", app.showMemes, tt.FramePending())
	}
	tt.Advance(60 * time.Millisecond)
	boardMidOpen, _ := tt.Find("迷子出发 (R)")
	tt.Advance(drawerOpenDuration)
	boardFullOpen, _ := tt.Find("迷子出发 (R)")

	if !(boardTopClosed.Y < boardMidOpen.Y && boardMidOpen.Y < boardFullOpen.Y) {
		t.Fatalf("expected board to shift smoothly down as meme drawer unfolds: closed=%.2f mid=%.2f open=%.2f",
			boardTopClosed.Y, boardMidOpen.Y, boardFullOpen.Y)
	}
	if tt.FramePending() {
		t.Fatal("expected FramePending()==false once meme drawer finishes opening")
	}

	// Close drawer: smoothly collapses back up and stops requesting frames.
	if err := tt.Click("名场面图鉴"); err != nil {
		t.Fatalf("close meme drawer: %v", err)
	}
	if app.showMemes || !tt.FramePending() {
		t.Fatalf("expected drawer animating closed: showMemes=%v pending=%v", app.showMemes, tt.FramePending())
	}
	tt.Advance(55 * time.Millisecond)
	boardMidClose, _ := tt.Find("迷子出发 (R)")
	if !(boardMidClose.Y < boardFullOpen.Y && boardMidClose.Y > boardTopClosed.Y) {
		t.Fatalf("expected board to rise smoothly during drawer close: open=%.2f mid=%.2f closed=%.2f",
			boardFullOpen.Y, boardMidClose.Y, boardTopClosed.Y)
	}
	tt.Advance(drawerCloseDuration)
	boardAfterClose, _ := tt.Find("迷子出发 (R)")
	if abs32(boardAfterClose.Y-boardTopClosed.Y) > 0.5 {
		t.Fatalf("board did not return to closed Y: got %.2f want %.2f", boardAfterClose.Y, boardTopClosed.Y)
	}
	if tt.FramePending() {
		t.Fatal("expected FramePending()==false once meme drawer finishes closing")
	}
}

func TestMemberHighlightAndLampWarmthSmoothBlend(t *testing.T) {
	// 1. Static pre-staged dark/light apps render exact target palette on frame 0 with zero pending frames.
	for _, dark := range []bool{false, true} {
		app := stageLiveScene(dark)
		tt := ui.NewTester(app.view, 1280, 900)
		tt.Frame()
		wantDesk := paperFor(dark).Desk
		gotDesk := tt.Image().At(1275, 895)
		if !pixelNearColor(gotDesk, wantDesk, 2) {
			r, g, b, _ := gotDesk.RGBA()
			t.Fatalf("static dark=%v frame-0 desk = #%02X%02X%02X, want exact %s", dark, r>>8, g>>8, b>>8, hexOf(wantDesk))
		}
		if tt.FramePending() {
			t.Fatalf("static dark=%v scene must not request extra frames on frame 0", dark)
		}
	}

	// 2. Runtime member switch smoothly interpolates member highlighter background and settles.
	app := newMinesweeperApp(42)
	app.darkMode = false
	tt := ui.NewTester(app.view, 1280, 900)
	tt.SetScale(2)
	tt.Frame()

	anonRect, ok := tt.Find("千早爱音")
	if !ok {
		t.Fatal("千早爱音 row missing")
	}
	sampleX := anonRect.X + anonRect.W - 8
	sampleY := anonRect.Y + anonRect.H/2
	dayPal := paperFor(false)
	wantIdle := dayPal.Sheet
	wantAnon := memberImage(MemberAnon).Alpha(0.36).Over(dayPal.Sheet)

	beforePx := sampleCanvasPixel(tt.Image(), sampleX, sampleY, 2)
	assertColorNear(t, "anon idle before click", beforePx, wantIdle)

	if err := tt.Click("千早爱音"); err != nil {
		t.Fatalf("click 千早爱音: %v", err)
	}
	// Move pointer away so hover tint does not interfere with selection color sample.
	tt.Move(640, 10)
	tt.Advance(65 * time.Millisecond)
	midPx := sampleCanvasPixel(tt.Image(), sampleX, sampleY, 2)
	if uiColorNear(midPx, wantIdle, 4) || uiColorNear(midPx, wantAnon, 4) {
		t.Fatalf("expected intermediate blended Anon highlighter at 65ms: got %s (idle=%s target=%s)",
			hexOf(midPx), hexOf(wantIdle), hexOf(wantAnon))
	}
	tt.Advance(chatNoteEnterDuration + memberHighlightDuration)
	settledPx := sampleCanvasPixel(tt.Image(), sampleX, sampleY, 2)
	assertColorNear(t, "anon highlighter settled", settledPx, wantAnon)
	if tt.FramePending() {
		t.Fatal("expected FramePending()==false after member highlight settles")
	}

	// 3. Runtime night-lamp toggle smoothly blends Desk, Sheet, Board, and top-left Lamp glow.
	if err := tt.Click("设置"); err != nil {
		t.Fatalf("open settings: %v", err)
	}
	tt.Advance(popoverEnterDuration + 20*time.Millisecond)
	tt.UseScreenReader()
	sw, ok := tt.AccessByRole(ui.RoleSwitch, "夜间台灯")
	if !ok {
		t.Fatal("missing 夜间台灯 switch")
	}
	tt.ClickAt(sw.Bounds.X+sw.Bounds.W/2, sw.Bounds.Y+sw.Bounds.H/2)
	if !app.darkMode {
		t.Fatal("clicking 夜间台灯 did not enable darkMode")
	}
	tt.Advance(80 * time.Millisecond)
	if app.nightBlendValue <= 0.1 || app.nightBlendValue >= 0.9 {
		t.Fatalf("expected intermediate nightBlendValue at 80ms, got %.3f", app.nightBlendValue)
	}
	midDesk := sampleCanvasPixel(tt.Image(), 1275, 895, 2)
	if uiColorNear(midDesk, dayPaper.Desk, 10) || uiColorNear(midDesk, nightPaper.Desk, 10) {
		t.Fatalf("expected blended desk pixel at 80ms: got %s (day=%s night=%s)",
			hexOf(midDesk), hexOf(dayPaper.Desk), hexOf(nightPaper.Desk))
	}
	midLamp := sampleCanvasPixel(tt.Image(), 140, 12, 2)
	if midLamp == midDesk {
		t.Fatalf("expected warm lamp radial gradient to differentiate top-left %s from far desk %s at 80ms",
			hexOf(midLamp), hexOf(midDesk))
	}

	tt.Advance(lampTransitionDuration + 30*time.Millisecond)
	settledDesk := sampleCanvasPixel(tt.Image(), 1275, 895, 2)
	assertColorNear(t, "settled night desk", settledDesk, nightPaper.Desk)
	settledLamp := sampleCanvasPixel(tt.Image(), 140, 12, 2)
	if settledLamp == settledDesk {
		t.Fatalf("expected settled night top-left lamp glow %s brighter/warmer than desk %s",
			hexOf(settledLamp), hexOf(settledDesk))
	}
	if app.nightBlendValue != 1 || tt.FramePending() {
		t.Fatalf("expected lamp transition converged: blend=%.3f pending=%v", app.nightBlendValue, tt.FramePending())
	}
}

func TestMetersSmoothHatchGlideAndFrame0A11y(t *testing.T) {
	// 1. Pre-staged scene renders exact target hatch immediately on frame 0 without pending frames.
	staged := stageLiveScene(false)
	stt := ui.NewTester(staged.view, 1280, 900)
	stt.Frame()
	if staged.progressVisual.value != staged.game.Progress() || staged.gravityVisual.value != float64(staged.game.Gravity) {
		t.Fatalf("pre-staged scene meters did not start at real values: prog=%.3f/%.3f grav=%.1f/%d",
			staged.progressVisual.value, staged.game.Progress(), staged.gravityVisual.value, staged.game.Gravity)
	}
	if stt.FramePending() {
		t.Fatal("pre-staged live scene must not request extra frames for meters on frame 0")
	}

	// 2. Runtime reveal, Soyorin shield gravity jump, and KeyR reset:
	// frame-0 a11y exposes exact target with single node per meter while hatch glides visually.
	app := newMinesweeperApp(7)
	app.darkMode = false
	app.memberID = MemberSoyorin
	// Vertical mine barrier at col 4 keeps the right column unopened after flooding (2,1).
	app.game = makeCorridorGame(6, 6, true, MemberSoyorin, [][2]int{{0, 0}, {0, 4}, {1, 4}, {2, 4}, {3, 4}, {4, 4}, {5, 4}})
	tt := ui.NewTester(app.view, 1280, 900)
	tt.SetScale(2)
	tt.UseScreenReader()
	tt.Frame()

	pal := paperFor(false)
	progInk := pal.InkOf(bandColor)
	gravInk := pal.InkOf(memberImage(MemberSoyorin))

	// Click (2,1) to flood-reveal the left side while staying StatusPlaying -> Progress jumps on frame 0.
	if err := tt.Click("cell-2-1"); err != nil {
		t.Fatalf("click cell-2-1: %v", err)
	}
	if app.game.Status != StatusPlaying {
		t.Fatalf("expected StatusPlaying after partial flood, got %v", app.game.Status)
	}
	wantProg := app.game.Progress()
	if wantProg < 0.4 {
		t.Fatalf("expected significant flood progress >= 0.4, got %.3f", wantProg)
	}
	progNode, ok := tt.AccessByRole(ui.RoleProgress, "排练完成度")
	if !ok || progNode.Min != 0 || progNode.Max != 1 || progNode.Now != wantProg {
		t.Fatalf("frame-0 RoleProgress = %+v (ok=%v), want [0,1] Now=%.4f", progNode, ok, wantProg)
	}
	// Ensure no duplicate decorative RoleProgress or RoleMeter nodes exist.
	var progCount, meterCount int
	for _, n := range tt.AccessAll() {
		if n.Role == ui.RoleProgress && n.Name == "排练完成度" {
			progCount++
		}
		if n.Role == ui.RoleMeter && n.Name == "乐队重力值" {
			meterCount++
		}
	}
	if progCount != 1 || meterCount != 1 {
		t.Fatalf("expected unique a11y nodes (1 progress, 1 meter), got progress=%d meter=%d", progCount, meterCount)
	}

	// Frame-0 hatch starts from 0, glides across 60ms, and reaches target width by 240ms.
	ext0 := hatchBarFilledExtent(tt.Image(), progNode.Bounds, 2, progInk, 24)
	tt.Advance(60 * time.Millisecond)
	extMid := hatchBarFilledExtent(tt.Image(), progNode.Bounds, 2, progInk, 24)
	tt.Advance(meterGlideDuration + 20*time.Millisecond)
	extEnd := hatchBarFilledExtent(tt.Image(), progNode.Bounds, 2, progInk, 24)
	if !(ext0 < extMid && extMid < extEnd) {
		t.Fatalf("expected progress pencil hatch to glide smoothly: t0=%dpx mid=%dpx settled=%dpx", ext0, extMid, extEnd)
	}

	// Trigger Soyorin shield on (0,0) -> Gravity jumps on frame 0 while hatch glides smoothly.
	gravBefore := app.game.Gravity
	if err := tt.Click("cell-0-0"); err != nil {
		t.Fatalf("click shielded mine (0,0): %v", err)
	}
	wantGrav := float64(app.game.Gravity)
	if app.game.Gravity <= gravBefore {
		t.Fatalf("expected shield to raise gravity from %d, got %d", gravBefore, app.game.Gravity)
	}
	meterNode, ok := tt.AccessByRole(ui.RoleMeter, "乐队重力值")
	if !ok || meterNode.Min != 0 || meterNode.Max != 100 || meterNode.Now != wantGrav {
		t.Fatalf("frame-0 RoleMeter = %+v (ok=%v), want [0,100] Now=%.0f", meterNode, ok, wantGrav)
	}
	gExt0 := hatchBarFilledExtent(tt.Image(), meterNode.Bounds, 2, gravInk, 24)
	tt.Advance(60 * time.Millisecond)
	gExtMid := hatchBarFilledExtent(tt.Image(), meterNode.Bounds, 2, gravInk, 24)
	tt.Advance(meterGlideDuration + 20*time.Millisecond)
	gExtEnd := hatchBarFilledExtent(tt.Image(), meterNode.Bounds, 2, gravInk, 24)
	if !(gExt0 < gExtMid && gExtMid < gExtEnd) {
		t.Fatalf("expected gravity pencil hatch to glide smoothly: t0=%dpx mid=%dpx settled=%dpx", gExt0, gExtMid, gExtEnd)
	}

	// Reset via KeyR -> frame-0 RoleProgress drops to 0 immediately while hatch glides down to 0.
	tt.Key(0, ui.KeyR)
	progReset, ok := tt.AccessByRole(ui.RoleProgress, "排练完成度")
	if !ok || progReset.Now != 0 {
		t.Fatalf("frame-0 RoleProgress after KeyR = %+v (ok=%v), want 0", progReset, ok)
	}
	rExt0 := hatchBarFilledExtent(tt.Image(), progReset.Bounds, 2, progInk, 24)
	tt.Advance(60 * time.Millisecond)
	rExtMid := hatchBarFilledExtent(tt.Image(), progReset.Bounds, 2, progInk, 24)
	tt.Advance(meterGlideDuration + 40*time.Millisecond)
	rExtEnd := hatchBarFilledExtent(tt.Image(), progReset.Bounds, 2, progInk, 24)
	if !(rExt0 > rExtMid && rExtMid > rExtEnd && rExtEnd == 0) {
		t.Fatalf("expected KeyR progress hatch to glide smoothly to 0: t0=%dpx mid=%dpx end=%dpx", rExt0, rExtMid, rExtEnd)
	}
	// Let any stage quote strip / chat note settle and verify zero lingering animation frames.
	tt.Advance(quoteStripDuration)
	if tt.FramePending() {
		t.Fatal("expected FramePending()==false after meters and highlights settle")
	}
}

func TestPeripheralThreeStateMotionBidirectionalAndMidFlightReduce(t *testing.T) {
	// Case A: System ReduceMotion == true + user motionStandard MUST still play full smooth motion
	// (chat slide, drawer collapse, meter hatch glide, popover enter shift, lamp color blend).
	t.Run("standard-overrides-system-reduce", func(t *testing.T) {
		p := defaultPrefs()
		p.DarkMode = false
		p.Motion = motionStandard
		app := newMinesweeperAppWith(appOptions{Seed: 7, Prefs: &p})
		app.game = makeCorridorGame(6, 6, true, MemberTomori, [][2]int{{0, 0}, {5, 5}})
		tt := ui.NewTester(app.view, 1280, 900)
		tt.SetPreferences(ui.Preferences{ReduceMotion: true})
		tt.Frame()
		if app.motionReducedActive {
			t.Fatal("motionStandard must override system ReduceMotion=true")
		}

		// Chat note slides existing note downward.
		olderText := app.chatLog[len(app.chatLog)-1].Text
		older0, _ := tt.Find(olderText)
		app.pushChat("要乐奈", FindMember(MemberRana).ColorHex, "标准模式覆盖系统减弱")
		tt.Frame()
		tt.Advance(65 * time.Millisecond)
		olderMid, _ := tt.Find(olderText)
		tt.Advance(chatNoteEnterDuration)
		olderEnd, _ := tt.Find(olderText)
		if !(older0.Y < olderMid.Y && olderMid.Y < olderEnd.Y) {
			t.Fatalf("motionStandard under system ReduceMotion did not slide chat note: 0=%.2f mid=%.2f end=%.2f",
				older0.Y, olderMid.Y, olderEnd.Y)
		}

		// Meme drawer smoothly unfolds board downward.
		boardClosed, _ := tt.Find("演奏中 · 重开 (R)")
		if err := tt.Click("名场面图鉴"); err != nil {
			t.Fatalf("open drawer: %v", err)
		}
		tt.Advance(60 * time.Millisecond)
		boardMid, _ := tt.Find("演奏中 · 重开 (R)")
		tt.Advance(drawerOpenDuration)
		boardOpen, _ := tt.Find("演奏中 · 重开 (R)")
		if !(boardClosed.Y < boardMid.Y && boardMid.Y < boardOpen.Y) {
			t.Fatalf("motionStandard under system ReduceMotion did not unfold drawer: closed=%.2f mid=%.2f open=%.2f",
				boardClosed.Y, boardMid.Y, boardOpen.Y)
		}

		// Meter hatch glides smoothly (does not snap on frame 0).
		if err := tt.Click("cell-3-3"); err != nil {
			t.Fatalf("click cell-3-3: %v", err)
		}
		if app.progressVisual.value >= app.game.Progress()-0.05 {
			t.Fatalf("motionStandard under system ReduceMotion snapped progress immediately: visual=%.3f real=%.3f",
				app.progressVisual.value, app.game.Progress())
		}
		tt.Advance(meterGlideDuration + rippleMaxDelay + rippleDuration + 50*time.Millisecond)
		if app.progressVisual.value != app.game.Progress() {
			t.Fatalf("progress did not settle to real value: visual=%.3f real=%.3f", app.progressVisual.value, app.game.Progress())
		}
	})

	// Case B: User motionReduced under system ReduceMotion == false skips all displacement/stretch delays
	// while keeping gentle color/opacity transitions and converging cleanly to FramePending()==false.
	t.Run("reduced-skips-displacement-keeps-color-and-fade", func(t *testing.T) {
		p := defaultPrefs()
		p.DarkMode = false
		p.Motion = motionReduced
		app := newMinesweeperAppWith(appOptions{Seed: 7, Prefs: &p})
		app.game = makeCorridorGame(6, 6, true, MemberTomori, [][2]int{{0, 0}, {5, 5}})
		tt := ui.NewTester(app.view, 1280, 900)
		tt.SetPreferences(ui.Preferences{ReduceMotion: false})
		tt.Frame()
		if !app.motionReducedActive {
			t.Fatal("motionReduced must activate reduced motion even when system ReduceMotion=false")
		}

		// Chat note skips height-collapse shift: existing note hits final Y immediately on frame 0!
		olderText := app.chatLog[len(app.chatLog)-1].Text
		app.pushChat("高松灯", FindMember(MemberTomori).ColorHex, "减弱动效下直接到位淡入")
		tt.Frame()
		olderFrame0, _ := tt.Find(olderText)
		tt.Advance(chatNoteReducedFade + 30*time.Millisecond)
		olderSettled, _ := tt.Find(olderText)
		if abs32(olderFrame0.Y-olderSettled.Y) > 0.5 {
			t.Fatalf("reduced motion should skip chat slide displacement: frame0=%.2f settled=%.2f", olderFrame0.Y, olderSettled.Y)
		}

		// Meme drawer opens at full height immediately on frame 0 (no layout slide) and closes immediately.
		boardBefore, _ := tt.Find("演奏中 · 重开 (R)")
		if err := tt.Click("名场面图鉴"); err != nil {
			t.Fatalf("open drawer in reduced: %v", err)
		}
		boardOpen0, _ := tt.Find("演奏中 · 重开 (R)")
		tt.Advance(drawerReducedFadeDuration + 30*time.Millisecond)
		boardOpenEnd, _ := tt.Find("演奏中 · 重开 (R)")
		if abs32(boardOpen0.Y-boardOpenEnd.Y) > 0.5 || boardOpen0.Y <= boardBefore.Y {
			t.Fatalf("reduced drawer open should snap to full height on frame 0: before=%.2f f0=%.2f end=%.2f",
				boardBefore.Y, boardOpen0.Y, boardOpenEnd.Y)
		}
		if err := tt.Click("名场面图鉴"); err != nil {
			t.Fatalf("close drawer in reduced: %v", err)
		}
		boardClosed0, _ := tt.Find("演奏中 · 重开 (R)")
		if abs32(boardClosed0.Y-boardBefore.Y) > 0.5 {
			t.Fatalf("reduced drawer close should snap closed immediately: got %.2f want %.2f", boardClosed0.Y, boardBefore.Y)
		}

		// Meter hatch snaps straight to target on frame 0 (zero stretch delay).
		if err := tt.Click("cell-3-3"); err != nil {
			t.Fatalf("click cell-3-3 in reduced: %v", err)
		}
		if app.progressVisual.value != app.game.Progress() {
			t.Fatalf("reduced motion must snap meter hatch on frame 0: visual=%.3f real=%.3f",
				app.progressVisual.value, app.game.Progress())
		}

		// Lamp warmth & paper color blend STILL transitions smoothly in reduced motion, then settles.
		app.updatePrefs(func(pr *prefs) { pr.DarkMode = true })
		tt.Frame()
		tt.Advance(80 * time.Millisecond)
		if app.nightBlendValue <= 0.1 || app.nightBlendValue >= 0.9 {
			t.Fatalf("reduced motion must keep gentle lamp color blend, got nightBlendValue=%.3f", app.nightBlendValue)
		}
		tt.Advance(lampTransitionDuration + 30*time.Millisecond)
		if app.nightBlendValue != 1 || tt.FramePending() {
			t.Fatalf("reduced lamp blend did not settle cleanly: blend=%.3f pending=%v", app.nightBlendValue, tt.FramePending())
		}
	})

	// Case C: Switching to motionReduced MID-ANIMATION immediately snaps displacement/stretch
	// on the same frame via syncMotionState(true) and converges with FramePending()==false.
	t.Run("mid-flight-switch-to-reduced-snaps-same-frame", func(t *testing.T) {
		app := newMinesweeperApp(7)
		app.darkMode = false
		app.game = makeCorridorGame(6, 6, true, MemberTomori, [][2]int{{0, 0}, {0, 4}, {1, 4}, {2, 4}, {3, 4}, {4, 4}, {5, 4}})
		tt := ui.NewTester(app.view, 1280, 900)
		tt.Frame()

		// Kick off meter glide, chat note slide, and meme drawer open simultaneously.
		if err := tt.Click("名场面图鉴"); err != nil {
			t.Fatalf("open drawer: %v", err)
		}
		if err := tt.Click("cell-2-1"); err != nil {
			t.Fatalf("click cell-2-1: %v", err)
		}
		app.pushChat("千早爱音", FindMember(MemberAnon).ColorHex, "动画途中切减弱")
		tt.Advance(45 * time.Millisecond)
		if app.progressVisual.value >= app.game.Progress()-0.01 {
			t.Fatalf("expected meter mid-glide before switching to reduced: visual=%.3f real=%.3f",
				app.progressVisual.value, app.game.Progress())
		}
		boardMidBeforeReduce, _ := tt.Find("演奏中 · 重开 (R)")

		// Switch to motionReduced mid-flight: meter & drawer height snap immediately on the same frame!
		app.updatePrefs(func(pr *prefs) { pr.Motion = motionReduced })
		tt.Frame()
		if app.progressVisual.value != app.game.Progress() {
			t.Fatalf("mid-flight switch to reduced did not snap progress hatch on same frame: visual=%.3f real=%.3f",
				app.progressVisual.value, app.game.Progress())
		}
		boardSnapped, _ := tt.Find("演奏中 · 重开 (R)")
		if boardSnapped.Y <= boardMidBeforeReduce.Y {
			t.Fatalf("mid-flight switch to reduced did not snap opening drawer to full height: before=%.2f snapped=%.2f",
				boardMidBeforeReduce.Y, boardSnapped.Y)
		}
		tt.Advance(chatNoteEnterDuration + drawerOpenDuration + 40*time.Millisecond)
		if tt.FramePending() {
			t.Fatal("expected FramePending()==false after mid-flight reduced transition settles")
		}
	})
}

// TestPeripheralMotionKeyframeSnapshots exports 24 mid-transition keyframe PNGs
// (4 peripheral transition scenes × small/default/large × day/night) when
// MYGO_PERIPHERAL_SNAPSHOT_DIR is set.
func TestPeripheralMotionKeyframeSnapshots(t *testing.T) {
	dir := os.Getenv("MYGO_PERIPHERAL_SNAPSHOT_DIR")
	if dir == "" {
		t.Skip("set MYGO_PERIPHERAL_SNAPSHOT_DIR to render peripheral motion keyframes")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	renderKeyframe := func(name string, width, height int, stage func(w, h int) *image.RGBA) {
		img := stage(width, height)
		saveTesterPNG(t, img, filepath.Join(dir, name))
	}

	for _, dark := range []bool{false, true} {
		suffix := "day"
		if dark {
			suffix = "night"
		}
		for _, vp := range snapshotViewports {
			// 1. Chat note pat-in & older notes shifting downward mid-frame.
			renderKeyframe(fmt.Sprintf("chat-note-enter-mid-%s-%s.png", vp.name, suffix), vp.width, vp.height, func(w, h int) *image.RGBA {
				app := stageLiveScene(dark)
				app.classicHoverRow, app.classicHoverCol = -1, -1
				tt := ui.NewTester(app.view, w, h)
				tt.SetScale(2)
				tt.Frame()
				app.pushChat("千早爱音", FindMember(MemberAnon).ColorHex, "「新便签贴好啦！大家继续加油排练！」")
				tt.Frame()
				tt.Advance(65 * time.Millisecond)
				return tt.Image()
			})

			// 2. Popover exit + single-click switch mid-frame (歌单 fading/lifting out while 设置 enters).
			renderKeyframe(fmt.Sprintf("popover-exit-switch-mid-%s-%s.png", vp.name, suffix), vp.width, vp.height, func(w, h int) *image.RGBA {
				app := stageLiveScene(dark)
				app.classicHoverRow, app.classicHoverCol = -1, -1
				tt := ui.NewTester(app.view, w, h)
				tt.SetScale(2)
				tt.Frame()
				if err := tt.Click("歌单"); err != nil {
					t.Fatalf("open setlist for keyframe: %v", err)
				}
				tt.Advance(popoverEnterDuration + 20*time.Millisecond)
				// Single-click 设置 directly while 歌单 is open -> 歌单 exits as ghost while 设置 enters!
				if err := tt.Click("设置"); err != nil {
					t.Fatalf("switch to settings for keyframe: %v", err)
				}
				tt.Advance(48 * time.Millisecond)
				return tt.Image()
			})

			// 3. Night desk-lamp warmth & paper blend mid-frame (toggling into suffix's theme).
			renderKeyframe(fmt.Sprintf("lamp-warmth-blend-mid-%s-%s.png", vp.name, suffix), vp.width, vp.height, func(w, h int) *image.RGBA {
				// Start from opposite theme so toggling toward dark shows mid-blend warm lamp transition.
				app := stageLiveScene(!dark)
				app.classicHoverRow, app.classicHoverCol = -1, -1
				tt := ui.NewTester(app.view, w, h)
				tt.SetScale(2)
				tt.UseScreenReader()
				tt.Frame()
				if err := tt.Click("设置"); err != nil {
					t.Fatalf("open settings for lamp keyframe: %v", err)
				}
				tt.Advance(popoverEnterDuration + 20*time.Millisecond)
				sw, ok := tt.AccessByRole(ui.RoleSwitch, "夜间台灯")
				if !ok {
					t.Fatal("missing 夜间台灯 switch")
				}
				tt.ClickAt(sw.Bounds.X+sw.Bounds.W/2, sw.Bounds.Y+sw.Bounds.H/2)
				tt.Advance(85 * time.Millisecond)
				return tt.Image()
			})

			// 4. Rehearsal progress & band gravity pencil hatch gliding mid-frame behind frame-0 percentage text.
			renderKeyframe(fmt.Sprintf("meter-hatch-glide-mid-%s-%s.png", vp.name, suffix), vp.width, vp.height, func(w, h int) *image.RGBA {
				app := newMinesweeperApp(7)
				app.darkMode = dark
				app.memberID = MemberSoyorin
				app.game = makeCorridorGame(9, 9, true, MemberSoyorin, [][2]int{
					{0, 0}, {0, 6}, {1, 6}, {2, 6}, {3, 6}, {4, 6}, {5, 6}, {6, 6}, {7, 6}, {8, 6},
				})
				pinStartQuote(app)
				tt := ui.NewTester(app.view, w, h)
				tt.SetScale(2)
				tt.Frame()
				// Flood open left half (keeps StatusPlaying) and trigger Soyorin shield so both meters jump sharply!
				if err := tt.Click("cell-4-2"); err != nil {
					t.Fatalf("flood click for meter keyframe: %v", err)
				}
				if err := tt.Click("cell-0-0"); err != nil {
					t.Fatalf("shield click for meter keyframe: %v", err)
				}
				tt.Advance(55 * time.Millisecond)
				return tt.Image()
			})
		}
	}
}
