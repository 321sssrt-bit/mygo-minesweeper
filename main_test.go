package main

import (
	"fmt"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestFirstClickSafetyAndFloodFill(t *testing.T) {
	g := NewGame(Difficulties[0], false, MemberTomori, 42)
	res := g.Reveal(4, 4)
	if res.Kind != ActionRevealed {
		t.Fatalf("expected ActionRevealed, got %s", res.Kind)
	}
	if !g.FirstClick || g.Status != StatusPlaying {
		t.Fatalf("expected playing state after first click")
	}
	center := g.Cell(4, 4)
	if center.IsMine || center.Adjacent != 0 {
		t.Fatalf("center should be safe 0-adjacent cell, got mine=%v adj=%d", center.IsMine, center.Adjacent)
	}
	if res.RevealedCount < 9 {
		t.Fatalf("expected flood fill >= 9 cells, got %d", res.RevealedCount)
	}
}

func TestWinAndMemeAchievements(t *testing.T) {
	// Classic win unlocks forever + anon_best
	g := NewGame(Difficulty{ID: "easy", Rows: 5, Cols: 5, Mines: 2}, false, MemberAnon, 1)
	g.FirstClick = true
	g.Status = StatusPlaying
	g.Cell(0, 0).IsMine = true
	g.Cell(4, 4).IsMine = true
	g.RecomputeAdjacency()

	var last ActionResult
	for i := range g.Cells {
		c := &g.Cells[i]
		if !c.IsMine && !c.Revealed {
			last = g.Reveal(c.Row, c.Col)
		}
	}
	if g.Status != StatusWon || last.Kind != ActionWon {
		t.Fatalf("expected StatusWon, got %v / %s", g.Status, last.Kind)
	}
	if g.Gravity != 0 {
		t.Fatalf("expected Gravity 0 on win, got %d", g.Gravity)
	}
	if len(last.NewMemes) < 2 {
		t.Fatalf("expected forever + anon_best memes, got %v", last.NewMemes)
	}
}

func TestHaruhikageLossTriggersMemeAndAlert(t *testing.T) {
	hard := Difficulties[2] // 春日影 Live
	g := NewGame(hard, false, MemberSoyorin, 7)
	g.FirstClick = true
	g.Status = StatusPlaying
	g.Cell(0, 0).IsMine = true
	g.RecomputeAdjacency()

	res := g.Reveal(0, 0)
	if res.Kind != ActionLost || res.QuoteEvent != "haruhikageLost" {
		t.Fatalf("expected haruhikageLost, got %+v", res)
	}
	if g.Gravity != 100 {
		t.Fatalf("expected 100 gravity on explosion, got %d", g.Gravity)
	}
	foundHaruhikage := false
	foundNeverHappy := false
	for _, m := range res.NewMemes {
		if m == MemeHaruhikage {
			foundHaruhikage = true
		}
		if m == MemeNeverHappy {
			foundNeverHappy = true
		}
	}
	if !foundHaruhikage || !foundNeverHappy {
		t.Fatalf("expected both haruhikage and never_happy memes, got %v", res.NewMemes)
	}
}

func TestSoyorinShieldAndBandSkills(t *testing.T) {
	g := NewGame(Difficulty{ID: "easy", Rows: 5, Cols: 5, Mines: 2, SkillCharges: 1}, true, MemberSoyorin, 7)
	g.FirstClick = true
	g.Status = StatusPlaying
	g.Cell(2, 2).IsMine = true
	g.Cell(0, 0).IsMine = true
	g.RecomputeAdjacency()

	res := g.Reveal(2, 2)
	if res.Kind != ActionShielded || !g.Cell(2, 2).ShieldSaved || g.SkillUsesLeft != 0 {
		t.Fatalf("expected Soyorin shield trigger, got %+v", res)
	}

	g2 := NewGame(Difficulties[0], true, MemberTaki, 99)
	g2.Reveal(4, 4)
	takiRes := g2.UseSkill()
	if takiRes.Kind != ActionSkill || g2.FlagsPlaced != 1 {
		t.Fatalf("expected Taki skill to flag 1 mine, got %+v flags=%d", takiRes, g2.FlagsPlaced)
	}

	g2.MemberID = MemberRana
	g2.SkillUsesLeft = 1
	before := g2.RevealedSafe
	ranaRes := g2.UseSkill()
	if ranaRes.Kind != ActionSkill && ranaRes.Kind != ActionWon {
		t.Fatalf("expected Rana skill reveal, got %+v", ranaRes)
	}
	if g2.RevealedSafe <= before {
		t.Fatalf("expected Rana skill to open safe cells")
	}
}

func TestNativeUIReusedWidgetsAndMemeFlow(t *testing.T) {
	app := newMinesweeperApp(12345)
	tt := ui.NewTester(app.view, 1120, 820)

	if !tt.HasText("MyGO!!!!! 迷子扫雷") || !tt.HasText("MINES 10") {
		t.Fatalf("expected title and MINES 010 badge in initial view")
	}

	// Segmented control switches difficulty to RiNG 排练室
	if err := tt.Click("RiNG 排练室"); err != nil {
		t.Fatalf("failed clicking segmented difficulty: %v", err)
	}
	if app.diffIdx != 1 || app.game.Cols != 16 || !tt.HasText("MINES 40") {
		t.Fatalf("expected 16x16 board after clicking RiNG 排练室, got cols=%d", app.game.Cols)
	}

	// Switch back to 羽丘天文部
	if err := tt.Click("羽丘天文部"); err != nil {
		t.Fatalf("failed clicking 羽丘天文部: %v", err)
	}

	// Click member 椎名立希 (after Tomori -> triggers special banter)
	if err := tt.Click("椎名立希"); err != nil {
		t.Fatalf("failed clicking 椎名立希: %v", err)
	}
	if app.memberID != MemberTaki {
		t.Fatalf("expected member taki, got %s", app.memberID)
	}

	// Toggle 名场面图鉴 drawer
	if err := tt.Click("名场面图鉴"); err != nil {
		t.Fatalf("failed toggling 名场面图鉴: %v", err)
	}
	if !app.showMemes || !tt.HasText("为什么要演奏春日影！？") {
		t.Fatalf("expected meme drawer to display achievements")
	}

	// Right-click a cell to place band-aid flag
	if err := tt.RightClick("cell-0-0"); err != nil {
		t.Fatalf("failed right clicking cell-0-0: %v", err)
	}
	if app.game.FlagsPlaced != 1 || !tt.HasText("MINES 9") {
		t.Fatalf("expected 1 flag placed and MINES 009, got %d", app.game.FlagsPlaced)
	}

	// Start game by clicking cell-4-4
	if err := tt.Click("cell-4-4"); err != nil {
		t.Fatalf("failed clicking cell-4-4: %v", err)
	}
	if app.game.Status != StatusPlaying {
		t.Fatalf("expected StatusPlaying, got %v", app.game.Status)
	}

	// Advance clock past 2 seconds and verify live timer badge
	tt.Advance(2100 * time.Millisecond)
	if app.elapsed < 2 || !tt.HasText("TIME 2s") {
		t.Fatalf("expected TIME 002s, got elapsed=%d", app.elapsed)
	}

	// Shortcuts F, E, R
	tt.Key(0, ui.KeyF)
	if !app.flagMode {
		t.Fatalf("expected flagMode true after F")
	}
	tt.Key(0, ui.KeyE)
	if app.game.SkillUsesLeft != 0 {
		t.Fatalf("expected skill used after E, left=%d", app.game.SkillUsesLeft)
	}
	tt.Key(0, ui.KeyR)
	if app.game.Status != StatusIdle || app.elapsed != 0 {
		t.Fatalf("expected reset after R")
	}
}

func TestClassicBoardUsesWideWindowSpace(t *testing.T) {
	app := newMinesweeperApp(12345)
	tt := ui.NewTester(app.view, 1280, 900)
	r, ok := tt.Find("cell-0-0")
	if !ok || r.W < 55 || r.H < 55 {
		t.Fatalf("classic cells remain tiny: %+v found=%v", r, ok)
	}
}

func TestClassicBoardFitsEveryDifficulty(t *testing.T) {
	for i := 0; i < 4; i++ {
		app := newMinesweeperApp(12345)
		app.diffIdx = i
		app.resetGame(false)
		tt := ui.NewTester(app.view, 1280, 900)
		tt.Frame()
		label := fmt.Sprintf("cell-%d-%d", app.game.Rows-1, app.game.Cols-1)
		r, ok := tt.Find(label)
		if !ok || r.Y+r.H > 900 {
			t.Fatalf("difficulty %d bottom cell clipped: %+v", i, r)
		}
	}
}

func TestHoverHintClassicAndPublicOnly(t *testing.T) {
	g := NewGame(Difficulty{ID: "easy", Rows: 5, Cols: 5, Mines: 1}, false, MemberAnon, 42)
	g.FirstClick = true
	g.Status = StatusPlaying
	g.Cell(0, 0).IsMine = true
	g.RecomputeAdjacency()
	g.Cell(1, 1).Revealed = true

	// Unflagged in Classic -> HintNone (no claimable in Classic).
	if h := g.InspectNumberHover(1, 1, false); h.Kind != HintNone {
		t.Fatalf("expected HintNone in unflagged classic cell, got %+v", h)
	}
	// Flag (0,0) -> satisfied chord -> HintChordable with remaining neighbors.
	g.ToggleFlag(0, 0, false)
	h := g.InspectNumberHover(1, 1, false)
	if h.Kind != HintChordable || h.Center != 1*5+1 || len(h.Targets) != 7 {
		t.Fatalf("expected HintChordable with 7 targets, got %+v", h)
	}
	// Flag mode disables hint.
	if hFlag := g.InspectNumberHover(1, 1, true); hFlag.Kind != HintNone {
		t.Fatalf("expected HintNone when flagMode=true, got %+v", hFlag)
	}
	// Finished status disables hint.
	g.Status = StatusLost
	if hLost := g.InspectNumberHover(1, 1, false); hLost.Kind != HintNone {
		t.Fatalf("expected HintNone when StatusLost, got %+v", hLost)
	}

	// Headless UI Classic hover + flagMode click consistency.
	app := newMinesweeperApp(42)
	app.game = NewGame(Difficulty{ID: "easy", Rows: 5, Cols: 5, Mines: 1}, false, MemberAnon, 42)
	app.game.FirstClick = true
	app.game.Status = StatusPlaying
	app.game.Cell(0, 0).IsMine = true
	app.game.RecomputeAdjacency()
	app.game.Cell(1, 1).Revealed = true
	app.game.ToggleFlag(0, 0, false)

	tt := ui.NewTester(app.view, 1280, 900)
	r, ok := tt.Find("cell-1-1")
	if !ok {
		t.Fatal("cell-1-1 not found")
	}
	tt.Move(r.X+r.W/2, r.Y+r.H/2)
	tt.Advance(2 * time.Millisecond)
	if app.classicHoverHint.Kind != HintChordable || len(app.classicHoverHint.Targets) != 7 {
		t.Fatalf("expected Classic UI hover HintChordable, got %+v", app.classicHoverHint)
	}
	// Toggle flagMode -> hint clears cleanly and clicking number doesn't chord.
	tt.Key(0, ui.KeyF)
	if app.classicHoverHint.Kind != HintNone {
		t.Fatalf("expected Classic hover hint cleared in flagMode, got %+v", app.classicHoverHint)
	}
	beforeSafe := app.game.RevealedSafe
	tt.ClickAt(r.X+r.W/2, r.Y+r.H/2)
	if app.game.RevealedSafe != beforeSafe {
		t.Fatalf("expected no chord when clicking in flagMode")
	}
}

func TestBFSRevealWaveBoundedAndClassicRippleLifecycle(t *testing.T) {
	g := NewGame(Difficulty{ID: "custom", Rows: 20, Cols: 20, Mines: 1}, false, MemberTomori, 7)
	g.FirstClick = true
	g.Status = StatusPlaying
	g.Cell(19, 19).IsMine = true
	g.RecomputeAdjacency()

	res := g.Reveal(0, 0)
	if res.RevealedCount != 399 {
		t.Fatalf("expected all 399 safe cells revealed, got %d", res.RevealedCount)
	}
	if len(res.RevealWave) != MaxRevealWaveEntries {
		t.Fatalf("expected bounded RevealWave=%d, got %d", MaxRevealWaveEntries, len(res.RevealWave))
	}
	if res.RevealWave[0].Index != 0 || res.RevealWave[0].Depth != 0 {
		t.Fatalf("expected depth 0 at origin, got %+v", res.RevealWave[0])
	}
	prevDepth := -1
	for _, entry := range res.RevealWave {
		if entry.Depth < prevDepth {
			t.Fatalf("expected non-decreasing BFS wave depth, got %d after %d", entry.Depth, prevDepth)
		}
		prevDepth = entry.Depth
	}
	if prevDepth == 0 {
		t.Fatalf("expected multi-step BFS depth > 0")
	}

	// Headless UI Classic ripple activation, virtual-clock decay, and R reset cleanup.
	app := newMinesweeperApp(42)
	tt := ui.NewTester(app.view, 1280, 900)
	if err := tt.Click("cell-4-4"); err != nil {
		t.Fatalf("click failed: %v", err)
	}
	if len(app.ripples) == 0 {
		t.Fatal("expected ripples populated on Classic flood-fill")
	}
	tt.Advance(rippleMaxDelay + rippleDuration + 50*time.Millisecond)
	if len(app.ripples) != 0 {
		t.Fatalf("expected ripples expired cleanly under virtual clock, remaining=%d", len(app.ripples))
	}

	// Trigger another ripple (skill) then immediate R reset.
	tt.Key(0, ui.KeyE)
	tt.Key(0, ui.KeyR)
	if len(app.ripples) != 0 {
		t.Fatalf("expected ripples cleared on R reset, remaining=%d", len(app.ripples))
	}
}

func TestClassicDampedShakeHitTestStabilityAndDecay(t *testing.T) {
	app := newMinesweeperApp(42)
	app.memberID = MemberSoyorin
	app.game = NewGame(Difficulty{ID: "easy", Rows: 5, Cols: 5, Mines: 2, SkillCharges: 1}, true, MemberSoyorin, 7)
	app.game.FirstClick = true
	app.game.Status = StatusPlaying
	app.game.Cell(0, 0).IsMine = true
	app.game.Cell(4, 4).IsMine = true
	app.game.RecomputeAdjacency()

	tt := ui.NewTester(app.view, 1280, 900)
	rBefore, ok := tt.Find("cell-0-0")
	if !ok {
		t.Fatal("cell-0-0 not found")
	}
	hudBefore, ok := tt.Find("演奏中 · 重开 (R)")
	if !ok {
		t.Fatal("reset HUD button not found")
	}

	// Trigger Soyorin shield (lighter 4.5 shake).
	tt.ClickAt(rBefore.X+rBefore.W/2, rBefore.Y+rBefore.H/2)
	if !app.game.Cell(0, 0).ShieldSaved {
		t.Fatal("expected Soyorin shield on (0,0)")
	}
	tt.Advance(30 * time.Millisecond)
	sx, sy := app.currentShakeOffset(tt.Now())
	if sx == 0 && sy == 0 {
		t.Fatal("expected non-zero shake offset 30ms after shield")
	}
	rDuring, _ := tt.Find("cell-0-0")
	if rDuring != rBefore {
		t.Fatalf("Classic cell hit-test bounds shifted during shake: before=%+v during=%+v", rBefore, rDuring)
	}
	hudDuring, _ := tt.Find("演奏中 · 重开 (R)")
	if hudDuring != hudBefore {
		t.Fatalf("HUD button shifted during shake: before=%+v during=%+v", hudBefore, hudDuring)
	}

	// Decay to zero past 220ms.
	tt.Advance(250 * time.Millisecond)
	sx2, sy2 := app.currentShakeOffset(tt.Now())
	if sx2 != 0 || sy2 != 0 {
		t.Fatalf("expected shake decayed to 0, got (%v,%v)", sx2, sy2)
	}

	// Hit second mine (9.0 shake) then reset immediately via R.
	mine44, _ := tt.Find("cell-4-4")
	tt.ClickAt(mine44.X+mine44.W/2, mine44.Y+mine44.H/2)
	tt.Advance(20 * time.Millisecond)
	if app.shakeAmplitude != 9.0 {
		t.Fatalf("expected 9.0 shake amplitude on mine loss, got %v", app.shakeAmplitude)
	}
	tt.Key(0, ui.KeyR)
	if app.shakeAmplitude != 0 || !app.shakeStart.IsZero() {
		t.Fatalf("expected immediate shake cleanup on R reset")
	}
}

func TestHaruhikageLossDramaOverlaySkipTimedAndReset(t *testing.T) {
	setupHaruhikageLoss := func() (*minesweeperApp, *ui.Tester) {
		app := newMinesweeperApp(42)
		app.diffIdx = 2 // 春日影 Live
		app.game = NewGame(Difficulties[2], true, MemberTomori, 7)
		app.game.FirstClick = true
		app.game.Status = StatusPlaying
		app.game.Cell(0, 0).IsMine = true
		app.game.RecomputeAdjacency()

		tt := ui.NewTester(app.view, 1280, 900)
		cell0, ok := tt.Find("cell-0-0")
		if !ok {
			t.Fatal("cell-0-0 not found")
		}
		tt.ClickAt(cell0.X+cell0.W/2, cell0.Y+cell0.H/2)
		return app, tt
	}

	// Flow 1: Click-to-skip isolates click from AlertDialog buttons and board tiles, then dismisses cleanly.
	app, tt := setupHaruhikageLoss()
	if !app.haruhikageIntro || app.haruhikageAlert || !tt.HasText("……为什么要演奏春日影？！") {
		t.Fatalf("expected Haruhikage drama overlay before dialog, intro=%v alert=%v", app.haruhikageIntro, app.haruhikageAlert)
	}
	if err := tt.Click("春日影演出遮罩"); err != nil {
		t.Fatalf("failed clicking drama overlay: %v", err)
	}
	if app.haruhikageIntro || !app.haruhikageAlert || !tt.HasText("为什么要演奏春日影？！") {
		t.Fatalf("expected click-to-skip to open AlertDialog, intro=%v alert=%v", app.haruhikageIntro, app.haruhikageAlert)
	}
	if app.game.Status != StatusLost {
		t.Fatalf("skip click leaked into restart: status=%v", app.game.Status)
	}
	if err := tt.Click("对不起，我错了"); err != nil {
		t.Fatalf("failed clicking forgive button: %v", err)
	}
	if app.haruhikageAlert || app.haruhikageIntro || app.game.Status != StatusLost {
		t.Fatalf("expected dialog dismissed with lost board preserved, intro=%v alert=%v status=%v", app.haruhikageIntro, app.haruhikageAlert, app.game.Status)
	}

	// Flow 2: Virtual-clock expiration auto-transitions to AlertDialog, and restart resets all overlay state.
	app2, tt2 := setupHaruhikageLoss()
	tt2.Advance(haruhikageIntroDuration + 50*time.Millisecond)
	if app2.haruhikageIntro || !app2.haruhikageAlert || !tt2.HasText("为什么要演奏春日影？！") {
		t.Fatalf("expected timed transition to AlertDialog, intro=%v alert=%v", app2.haruhikageIntro, app2.haruhikageAlert)
	}
	if err := tt2.Click("一辈子重开"); err != nil {
		t.Fatalf("failed clicking restart button: %v", err)
	}
	if app2.game.Status != StatusIdle || app2.haruhikageAlert || app2.haruhikageIntro || !app2.haruhikageIntroStart.IsZero() {
		t.Fatalf("expected full reset cleanup after restart, status=%v intro=%v alert=%v", app2.game.Status, app2.haruhikageIntro, app2.haruhikageAlert)
	}
}

