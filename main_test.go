package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"mygo-minesweeper/internal/scanreport"
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
	marked := Difficulty{
		ID:         "marked-classic",
		Label:      "春日影 Live",
		Song:       "春日影",
		Gameplay:   GameplayClassic,
		Haruhikage: true,
		Rows:       16,
		Cols:       30,
		Mines:      99,
	}
	g := NewGame(marked, false, MemberSoyorin, 7)
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

func TestHaruhikageSceneFollowsMarkerNotID(t *testing.T) {
	hitMine := func(diff Difficulty, moves int) ActionResult {
		g := NewGame(diff, false, MemberAnon, 7)
		g.FirstClick = true
		g.Moves = moves
		g.Cell(0, 0).IsMine = true
		return g.Reveal(0, 0)
	}

	// A marked track plays the scene under any ID.
	if marked := hitMine(Difficulty{ID: "encore", Haruhikage: true, Rows: 5, Cols: 5, Mines: 1}, 0); marked.QuoteEvent != "haruhikageLost" || !slices.Contains(marked.NewMemes, MemeHaruhikage) {
		t.Fatalf("expected marker to trigger haruhikage scene, got %+v", marked)
	}
	// An unmarked track never does, even when its ID is "hard".
	if unmarked := hitMine(Difficulty{ID: "hard", Rows: 5, Cols: 5, Mines: 1}, 0); unmarked.QuoteEvent != "lost" || slices.Contains(unmarked.NewMemes, MemeHaruhikage) {
		t.Fatalf("expected no haruhikage scene without the marker, got %+v", unmarked)
	}
	// A marked loss past the early moves still skips never_happy.
	if late := hitMine(Difficulty{ID: "encore", Haruhikage: true, Rows: 5, Cols: 5, Mines: 1}, 5); late.QuoteEvent != "haruhikageLost" || !slices.Contains(late.NewMemes, MemeHaruhikage) || slices.Contains(late.NewMemes, MemeNeverHappy) {
		t.Fatalf("expected haruhikage only on a late marked loss, got %+v", late)
	}
	// The marker is classic-only: a marked duel mine stays duelMine.
	if duel := hitMine(Difficulty{Gameplay: GameplayDuel, Haruhikage: true, Rows: 5, Cols: 5, Mines: 1}, 0); duel.QuoteEvent != "duelMine" || slices.Contains(duel.NewMemes, MemeHaruhikage) {
		t.Fatalf("expected marked duel loss to stay duelMine, got %+v", duel)
	}
	// A shield save on a marked track is not a loss and skips the scene.
	shielded := NewGame(Difficulty{Haruhikage: true, Rows: 5, Cols: 5, Mines: 1, SkillCharges: 1}, true, MemberSoyorin, 7)
	shielded.FirstClick = true
	shielded.Cell(0, 0).IsMine = true
	if shieldRes := shielded.Reveal(0, 0); shieldRes.Kind != ActionShielded || shieldRes.QuoteEvent != "shield" || slices.Contains(shieldRes.NewMemes, MemeHaruhikage) {
		t.Fatalf("expected shield save without the scene meme, got %+v", shieldRes)
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

	if !tt.HasText("MyGO!!!!! 迷子扫雷") || !tt.HasText("剩雷 10") {
		t.Fatalf("expected title and 剩雷 10 badge in initial view")
	}

	// Setlist popover switches track to 碧天伴走 (RiNG 排练室)
	if err := tt.Click("歌单"); err != nil {
		t.Fatalf("failed opening setlist popover: %v", err)
	}
	if err := tt.Click("碧天伴走"); err != nil {
		t.Fatalf("failed selecting 碧天伴走 from setlist popover: %v", err)
	}
	if app.diffIdx != 1 || app.game.Cols != 16 || !tt.HasText("剩雷 40") {
		t.Fatalf("expected 16x16 board after selecting 碧天伴走, got cols=%d", app.game.Cols)
	}

	// Switch back to 迷星叫 (羽丘天文部)
	if err := tt.Click("歌单"); err != nil {
		t.Fatalf("failed reopening setlist popover: %v", err)
	}
	if err := tt.Click("迷星叫"); err != nil {
		t.Fatalf("failed selecting 迷星叫 from setlist popover: %v", err)
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
	if app.game.FlagsPlaced != 1 || !tt.HasText("剩雷 9") {
		t.Fatalf("expected 1 flag placed and 剩雷 9, got %d", app.game.FlagsPlaced)
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
	if app.elapsed < 2 || !tt.HasText("用时 2 秒") {
		t.Fatalf("expected 用时 2 秒, got elapsed=%d", app.elapsed)
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

func TestClassicBoardUsesStandardPitch(t *testing.T) {
	app := newMinesweeperApp(12345)
	tt := ui.NewTester(app.view, 1280, 900)
	r00, ok := tt.Find("cell-0-0")
	r01, ok1 := tt.Find("cell-0-1")
	r10, ok2 := tt.Find("cell-1-0")
	if !ok || !ok1 || !ok2 {
		t.Fatalf("classic grid cells missing: (0,0)=%v (0,1)=%v (1,0)=%v", ok, ok1, ok2)
	}
	if r00.W != gridCellSize || r00.H != gridCellSize {
		t.Fatalf("classic cell size = %.1fx%.1f, want %.1fx%.1f", r00.W, r00.H, gridCellSize, gridCellSize)
	}
	if pitchX, pitchY := r01.X-r00.X, r10.Y-r00.Y; pitchX != standardCellPitch || pitchY != standardCellPitch {
		t.Fatalf("classic grid pitch = (%.1f, %.1f), want (%.1f, %.1f)", pitchX, pitchY, standardCellPitch, standardCellPitch)
	}
}

func TestClassicBoardFitsEveryDifficulty(t *testing.T) {
	for i, diff := range Difficulties {
		if diff.Gameplay != GameplayClassic {
			continue
		}
		app := newMinesweeperApp(12345)
		app.diffIdx = i
		app.resetGame(false)
		tt := ui.NewTester(app.view, 1280, 900)
		tt.Frame()
		if app.classicOnCanvas() {
			r, ok := tt.Find("经典棋盘")
			if !ok || r.W <= 0 || r.H <= 0 || r.X < 0 || r.Y < 0 || r.X+r.W > 1280 || r.Y+r.H > 900 {
				t.Fatalf("difficulty %d (%s) classic canvas bounds invalid: %+v found=%v", i, diff.ID, r, ok)
			}
			if _, hasCell := tt.Find("cell-0-0"); hasCell {
				t.Fatalf("difficulty %d (%s) rendered button grid instead of classic canvas", i, diff.ID)
			}
			continue
		}
		r0, ok0 := tt.Find("cell-0-0")
		label := fmt.Sprintf("cell-%d-%d", app.game.Rows-1, app.game.Cols-1)
		rLast, okLast := tt.Find(label)
		if !ok0 || !okLast || r0.W != gridCellSize || r0.H != gridCellSize || rLast.X+rLast.W > 1280 || rLast.Y+rLast.H > 900 {
			t.Fatalf("difficulty %d (%s) grid cells invalid or clipped: first=%+v last=%+v", i, diff.ID, r0, rLast)
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
		app.game = NewGame(Difficulty{
			ID:           "marked-classic",
			Label:        "春日影 Live",
			Song:         "春日影",
			Gameplay:     GameplayClassic,
			Haruhikage:   true,
			Rows:         9,
			Cols:         9,
			Mines:        1,
			SkillCharges: 3,
		}, true, MemberTomori, 7)
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

	// Flow 1: Frame-0 click-to-skip isolates click from AlertDialog buttons and board tiles, then dismisses cleanly.
	app, tt := setupHaruhikageLoss()
	if !app.haruhikageIntro || app.haruhikageAlert || tt.HasText("……为什么要演奏春日影？！") {
		t.Fatalf("expected Haruhikage drama overlay in half-beat pause on frame 0, intro=%v alert=%v", app.haruhikageIntro, app.haruhikageAlert)
	}
	if _, ok := tt.Find("春日影演出遮罩"); !ok {
		t.Fatal("expected Haruhikage drama overlay mounted on frame 0")
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

func TestHaruhikagePresentationFollowsMarkerNotID(t *testing.T) {
	setupClassicMineHit := func(diff Difficulty, band bool, member MemberID, skillCharges int) (*minesweeperApp, *ui.Tester) {
		app := newMinesweeperApp(42)
		app.memberID = member
		diff.SkillCharges = skillCharges
		app.game = NewGame(diff, band, member, 7)
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

	// Case 1: Marked classic track under a non-"hard" ID still plays the drama overlay and dialog, with unchanged timing.
	app1, tt1 := setupClassicMineHit(Difficulty{
		ID:         "encore",
		Label:      "安可曲目",
		Song:       "影色舞",
		Gameplay:   GameplayClassic,
		Haruhikage: true,
		Rows:       5,
		Cols:       5,
		Mines:      1,
	}, false, MemberTomori, 0)
	if !app1.haruhikageIntro || app1.haruhikageAlert {
		t.Fatalf("expected Haruhikage overlay on marked non-hard track, intro=%v alert=%v", app1.haruhikageIntro, app1.haruhikageAlert)
	}
	tt1.Advance(haruhikageBeatDelay + 20*time.Millisecond)
	if !tt1.HasText("……为什么要演奏春日影？！") {
		t.Fatalf("expected Haruhikage line after half-beat on marked non-hard track")
	}
	tt1.Advance(haruhikageIntroDuration - haruhikageBeatDelay - 120*time.Millisecond)
	if !app1.haruhikageIntro || app1.haruhikageAlert {
		t.Fatalf("expected overlay to stay visible before haruhikageIntroDuration, intro=%v alert=%v", app1.haruhikageIntro, app1.haruhikageAlert)
	}
	tt1.Advance(150 * time.Millisecond)
	if app1.haruhikageIntro || !app1.haruhikageAlert || !tt1.HasText("为什么要演奏春日影？！") {
		t.Fatalf("expected dialog after haruhikageIntroDuration, intro=%v alert=%v", app1.haruhikageIntro, app1.haruhikageAlert)
	}

	// Case 2: Unmarked classic track with ID "hard" never triggers the Haruhikage overlay or dialog.
	app2, tt2 := setupClassicMineHit(Difficulty{
		ID:       "hard",
		Label:    "普通高难",
		Song:     "迷星叫",
		Gameplay: GameplayClassic,
		Rows:     5,
		Cols:     5,
		Mines:    1,
	}, false, MemberTomori, 0)
	if app2.game.Status != StatusLost {
		t.Fatalf("expected StatusLost on unmarked hard-ID mine hit, got %v", app2.game.Status)
	}
	if app2.haruhikageIntro || app2.haruhikageAlert || tt2.HasText("……为什么要演奏春日影？！") || tt2.HasText("为什么要演奏春日影？！") {
		t.Fatalf("expected no Haruhikage presentation without marker, intro=%v alert=%v", app2.haruhikageIntro, app2.haruhikageAlert)
	}
	tt2.Advance(haruhikageIntroDuration + 50*time.Millisecond)
	if app2.haruhikageIntro || app2.haruhikageAlert || tt2.HasText("为什么要演奏春日影？！") {
		t.Fatalf("expected no delayed Haruhikage dialog without marker, intro=%v alert=%v", app2.haruhikageIntro, app2.haruhikageAlert)
	}
	if !tt2.HasText("炸团危机！……要假装无事发生重开吗？") {
		t.Fatalf("expected plain defeat toast on unmarked hard-ID track")
	}

	// Case 3: Shield save on a marked classic track is not a loss and skips the Haruhikage presentation.
	app3, tt3 := setupClassicMineHit(Difficulty{
		ID:         "encore",
		Gameplay:   GameplayClassic,
		Haruhikage: true,
		Rows:       5,
		Cols:       5,
		Mines:      1,
	}, true, MemberSoyorin, 1)
	if app3.game.Status != StatusPlaying || !app3.game.Cell(0, 0).ShieldSaved {
		t.Fatalf("expected Soyorin shield save to keep playing, status=%v shielded=%v", app3.game.Status, app3.game.Cell(0, 0).ShieldSaved)
	}
	if app3.haruhikageIntro || app3.haruhikageAlert || tt3.HasText("……为什么要演奏春日影？！") {
		t.Fatalf("expected no Haruhikage presentation on shield save, intro=%v alert=%v", app3.haruhikageIntro, app3.haruhikageAlert)
	}

	// Case 4: Marked duel track stays classic-only for #11 and does not open Haruhikage presentation on mine stun.
	app4 := newMinesweeperApp(42)
	app4.memberID = MemberTomori
	app4.game = NewGame(Difficulty{
		ID:         "encore-duel",
		Gameplay:   GameplayDuel,
		Haruhikage: true,
		Rows:       5,
		Cols:       5,
		Mines:      1,
	}, true, MemberTomori, 7)
	app4.game.FirstClick = true
	app4.game.Status = StatusPlaying
	app4.game.Cell(0, 0).IsMine = true
	app4.game.RecomputeAdjacency()
	tt4 := ui.NewTester(app4.view, 1280, 900)
	if _, ok := tt4.Find("决斗棋盘"); !ok {
		t.Fatal("duel board missing")
	}
	app4.pendingResult = app4.game.Reveal(0, 0)
	tt4.Frame()
	if app4.haruhikageIntro || app4.haruhikageAlert || tt4.HasText("……为什么要演奏春日影？！") {
		t.Fatalf("expected no Haruhikage presentation on marked duel mine hit, intro=%v alert=%v", app4.haruhikageIntro, app4.haruhikageAlert)
	}
	if !tt4.HasText("踩雷 −100 · 硬直 2.4 秒！小心被灯拉开比分") {
		t.Fatalf("expected duel stun toast on marked duel mine hit")
	}
}

func TestDuelHaruhikageDefeatPresentation(t *testing.T) {
	setupTimedDuel := func(marked bool, player, ai int, preUnlocked bool) (*minesweeperApp, *ui.Tester) {
		app := newMinesweeperApp(42)
		app.memberID = MemberTomori
		if preUnlocked {
			app.unlockedMemes[MemeHaruhikage] = true
		}
		app.game = NewGame(Difficulty{
			ID:           "duel-test",
			Label:        "决斗测试",
			Song:         "春日影",
			Gameplay:     GameplayDuel,
			Haruhikage:   marked,
			Rows:         5,
			Cols:         5,
			Mines:        1,
			DuelDuration: 1,
			AIInterval:   10,
		}, true, MemberTomori, 7)
		app.game.FirstClick = true
		app.game.Status = StatusPlaying
		app.game.duel.PlayerScore = player
		app.game.duel.AIScore = ai

		tt := ui.NewTester(app.view, 1280, 900)
		tt.Advance(1100 * time.Millisecond)
		return app, tt
	}

	// Case 1: Marked duel timeout score loss plays drama overlay, unlocks meme, emits quote, and click-to-skip opens AlertDialog once.
	app1, tt1 := setupTimedDuel(true, 10, 100, false)
	if !app1.duelEnded || !app1.haruhikageIntro || app1.haruhikageAlert {
		t.Fatalf("expected Haruhikage overlay on marked duel score loss, ended=%v intro=%v alert=%v", app1.duelEnded, app1.haruhikageIntro, app1.haruhikageAlert)
	}
	tt1.Advance(haruhikageBeatDelay + 20*time.Millisecond)
	if !tt1.HasText("……为什么要演奏春日影？！") {
		t.Fatalf("expected Haruhikage overlay line after half-beat on marked duel score loss")
	}
	if !app1.unlockedMemes[MemeHaruhikage] {
		t.Fatalf("expected MemeHaruhikage unlocked on marked duel score loss")
	}
	if wantQuote := memberQuotes[MemberTomori]["haruhikageLost"][0]; app1.quote != wantQuote {
		t.Fatalf("expected haruhikageLost quote %q, got %q", wantQuote, app1.quote)
	}
	if err := tt1.Click("春日影演出遮罩"); err != nil {
		t.Fatalf("failed clicking duel drama overlay: %v", err)
	}
	if app1.haruhikageIntro || !app1.haruhikageAlert || !tt1.HasText("为什么要演奏春日影？！") {
		t.Fatalf("expected click-to-skip to open AlertDialog, intro=%v alert=%v", app1.haruhikageIntro, app1.haruhikageAlert)
	}
	if err := tt1.Click("对不起，我错了"); err != nil {
		t.Fatalf("failed clicking forgive button: %v", err)
	}
	tt1.Frame()
	if app1.haruhikageAlert || app1.haruhikageIntro || tt1.HasText("……为什么要演奏春日影？！") {
		t.Fatalf("expected dialog dismissed without retriggering, intro=%v alert=%v", app1.haruhikageIntro, app1.haruhikageAlert)
	}
	if err := tt1.Click("再战一局"); err != nil {
		t.Fatalf("failed clicking ToastAction rematch button: %v", err)
	}
	if app1.duelEnded || app1.game.Status != StatusIdle {
		t.Fatalf("expected ToastAction rematch reset, ended=%v status=%v", app1.duelEnded, app1.game.Status)
	}

	// Case 2: Replayability + timed auto-transition + restart reset cleanup.
	app2, tt2 := setupTimedDuel(true, 10, 100, true)
	if !app2.haruhikageIntro || app2.haruhikageAlert {
		t.Fatalf("expected replayable Haruhikage overlay even when meme already unlocked, intro=%v alert=%v", app2.haruhikageIntro, app2.haruhikageAlert)
	}
	tt2.Advance(haruhikageIntroDuration + 50*time.Millisecond)
	if app2.haruhikageIntro || !app2.haruhikageAlert || !tt2.HasText("为什么要演奏春日影？！") {
		t.Fatalf("expected timed transition to AlertDialog, intro=%v alert=%v", app2.haruhikageIntro, app2.haruhikageAlert)
	}
	if err := tt2.Click("一辈子重开"); err != nil {
		t.Fatalf("failed clicking restart button: %v", err)
	}
	if app2.duelEnded || app2.haruhikageAlert || app2.haruhikageIntro || !app2.haruhikageIntroStart.IsZero() {
		t.Fatalf("expected full reset cleanup after duel restart, ended=%v intro=%v alert=%v", app2.duelEnded, app2.haruhikageIntro, app2.haruhikageAlert)
	}

	// Case 3: Cleared-board marked duel score loss also plays the drama presentation.
	app3 := newMinesweeperApp(42)
	app3.memberID = MemberTomori
	app3.game = NewGame(Difficulty{
		ID:           "duel-clear",
		Gameplay:     GameplayDuel,
		Haruhikage:   true,
		Rows:         5,
		Cols:         5,
		Mines:        24,
		DuelDuration: 60,
		AIInterval:   10,
	}, true, MemberTomori, 7)
	app3.game.duel.AIScore = 500
	tt3 := ui.NewTester(app3.view, 1280, 900)
	app3.pendingResult = app3.game.Reveal(0, 0)
	tt3.Frame()
	if !app3.duelEnded || !app3.haruhikageIntro || app3.haruhikageAlert {
		t.Fatalf("expected Haruhikage overlay on cleared-board marked duel loss, ended=%v intro=%v alert=%v", app3.duelEnded, app3.haruhikageIntro, app3.haruhikageAlert)
	}
	tt3.Advance(haruhikageBeatDelay + 20*time.Millisecond)
	if !tt3.HasText("……为什么要演奏春日影？！") {
		t.Fatalf("expected Haruhikage line after half-beat on cleared-board marked duel loss")
	}
	if !app3.unlockedMemes[MemeHaruhikage] || app3.unlockedMemes[MemeForever] {
		t.Fatalf("expected MemeHaruhikage without MemeForever on cleared-board loss, unlocked=%v", app3.unlockedMemes)
	}
	if wantQuote := memberQuotes[MemberTomori]["haruhikageLost"][0]; app3.quote != wantQuote {
		t.Fatalf("expected haruhikageLost quote %q after score-loss clear, got %q", wantQuote, app3.quote)
	}
	if err := tt3.Click("春日影演出遮罩"); err != nil {
		t.Fatalf("failed skipping overlay in cleared-board case: %v", err)
	}
	if err := tt3.Click("对不起，我错了"); err != nil {
		t.Fatalf("failed dismissing dialog in cleared-board case: %v", err)
	}
	tt3.Key(0, ui.KeyR)
	if app3.duelEnded || app3.haruhikageIntro || app3.haruhikageAlert || !app3.haruhikageIntroStart.IsZero() || app3.game.Status != StatusIdle {
		t.Fatalf("expected KeyR reset cleanup after dialog dismiss, ended=%v intro=%v alert=%v status=%v", app3.duelEnded, app3.haruhikageIntro, app3.haruhikageAlert, app3.game.Status)
	}

	// Case 3b: AI-initiated board clear with player trailing on score also plays the drama presentation.
	appAI := newMinesweeperApp(42)
	appAI.memberID = MemberTomori
	appAI.game = NewGame(Difficulty{
		ID:           "duel-ai-clear",
		Gameplay:     GameplayDuel,
		Haruhikage:   true,
		Rows:         5,
		Cols:         5,
		Mines:        23,
		DuelDuration: 60,
		AIInterval:   0.5,
	}, true, MemberTomori, 7)
	appAI.game.FirstClick = true
	appAI.game.Status = StatusPlaying
	for i := range appAI.game.Cells {
		appAI.game.Cells[i].IsMine = true
	}
	appAI.game.Cells[23].IsMine = false
	appAI.game.Cells[24].IsMine = false
	appAI.game.RecomputeAdjacency()
	appAI.game.Cells[23].Revealed = true
	appAI.game.RevealedSafe = 1
	for _, n := range appAI.game.Neighbors(4, 3) {
		if n.IsMine {
			n.Flagged = true
			n.FlagOwner = DuelAI
		}
	}
	appAI.game.duel.frontier[24] = true
	appAI.game.duel.AIScore = 500
	appAI.game.duel.NextAI = 0.2
	ttAI := ui.NewTester(appAI.view, 1280, 900)
	ttAI.Advance(300 * time.Millisecond)
	if !appAI.duelEnded || !appAI.haruhikageIntro || appAI.haruhikageAlert {
		t.Fatalf("expected Haruhikage overlay on AI-cleared score loss, ended=%v intro=%v alert=%v", appAI.duelEnded, appAI.haruhikageIntro, appAI.haruhikageAlert)
	}
	ttAI.Advance(haruhikageBeatDelay + 20*time.Millisecond)
	if !ttAI.HasText("……为什么要演奏春日影？！") {
		t.Fatalf("expected Haruhikage line after half-beat on AI-cleared score loss")
	}

	// Case 4: Marked draw, marked win, and unmarked score loss never show the overlay or dialog.
	for _, tc := range []struct {
		name       string
		marked     bool
		player, ai int
	}{
		{"marked draw", true, 100, 100},
		{"marked win", true, 200, 100},
		{"unmarked loss", false, 10, 100},
	} {
		appN, ttN := setupTimedDuel(tc.marked, tc.player, tc.ai, false)
		if !appN.duelEnded {
			t.Fatalf("%s: expected duel settled", tc.name)
		}
		if appN.haruhikageIntro || appN.haruhikageAlert || appN.unlockedMemes[MemeHaruhikage] ||
			ttN.HasText("……为什么要演奏春日影？！") || ttN.HasText("为什么要演奏春日影？！") {
			t.Fatalf("%s must not trigger Haruhikage presentation or meme, intro=%v alert=%v unlocked=%v",
				tc.name, appN.haruhikageIntro, appN.haruhikageAlert, appN.unlockedMemes[MemeHaruhikage])
		}
		ttN.Advance(haruhikageIntroDuration + 50*time.Millisecond)
		if appN.haruhikageIntro || appN.haruhikageAlert || ttN.HasText("为什么要演奏春日影？！") {
			t.Fatalf("%s must not trigger delayed Haruhikage dialog", tc.name)
		}
	}
}

func TestAppIconAndBundleConfig(t *testing.T) {
	img, err := png.Decode(bytes.NewReader(appIconPNG))
	if err != nil {
		t.Fatalf("embedded appIconPNG failed to decode: %v", err)
	}
	b := img.Bounds()
	if b.Dx() != 1024 || b.Dy() != 1024 {
		t.Fatalf("expected 1024x1024 app icon, got %dx%d", b.Dx(), b.Dy())
	}
	// Outer corner (0,0) must be transparent (squircle mask), center (512,560) must be opaque.
	if _, _, _, a0 := img.At(0, 0).RGBA(); a0 != 0 {
		t.Fatalf("expected transparent outer corner at (0,0), got alpha=%d", a0)
	}
	if _, _, _, ac := img.At(512, 560).RGBA(); ac != 0xffff {
		t.Fatalf("expected opaque center at (512,560), got alpha=%d", ac)
	}

	rawCfg, err := os.ReadFile("mygo.json")
	if err != nil {
		t.Fatalf("failed reading mygo.json: %v", err)
	}
	var cfg struct {
		Icon    string `json:"icon"`
		License string `json:"license"`
	}
	if err := json.Unmarshal(rawCfg, &cfg); err != nil {
		t.Fatalf("failed parsing mygo.json: %v", err)
	}
	if cfg.Icon != "resources/icon.png" || cfg.License != "MIT" {
		t.Fatalf("unexpected mygo.json icon/license config: %+v", cfg)
	}
}

func TestHaruhikageMovedToDuelAndKageiromaiLossIsPlain(t *testing.T) {
	// AC 1: Setlist has exactly one 春日影 (the 1000x1000 duel) and classic slot #3 is 《影色舞》.
	haruhikageCount := 0
	markedCount := 0
	for i, d := range Difficulties {
		if d.ID == "hard" || strings.Contains(d.Song, "Crychic") {
			t.Fatalf("retired classic Haruhikage track still listed at %d: %+v", i, d)
		}
		if strings.Contains(d.Song, "春日影") || strings.Contains(d.Label, "春日影") {
			haruhikageCount++
		}
		if d.Haruhikage {
			markedCount++
		}
	}
	if haruhikageCount != 1 || markedCount != 1 {
		t.Fatalf("expected exactly 1 春日影 track and 1 Haruhikage marker, got name=%d marked=%d", haruhikageCount, markedCount)
	}
	if Difficulties[2] != TrackKageiromai || Difficulties[2].Haruhikage {
		t.Fatalf("Difficulties[2] = %+v, want unmarked TrackKageiromai", Difficulties[2])
	}
	duelDiff := Difficulties[len(Difficulties)-1]
	if duelDiff.Gameplay != GameplayDuel || duelDiff.Rows != 1000 || duelDiff.Cols != 1000 ||
		duelDiff.Label != "解不完的心结" || duelDiff.Song != "春日影" || !duelDiff.Haruhikage {
		t.Fatalf("1000x1000 duel = %+v, want 解不完的心结《春日影》 with Haruhikage=true", duelDiff)
	}

	// Open the live setlist popover and verify track titles.
	app := newMinesweeperApp(42)
	tt := ui.NewTester(app.view, 1280, 900)
	if err := tt.Click("歌单"); err != nil {
		t.Fatalf("open setlist popover: %v", err)
	}
	if _, ok := tt.Find("影色舞"); !ok {
		t.Fatal("expected 《影色舞》 in setlist popover")
	}
	if _, ok := tt.Find("春日影"); !ok {
		t.Fatal("expected 《春日影》 in setlist popover")
	}
	if _, ok := tt.Find("春日影 (Crychic Ver.)"); ok {
		t.Fatal("retired 《春日影 (Crychic Ver.)》 must not appear in setlist popover")
	}

	// AC 2: Losing on 《影色舞》 (Difficulties[2]) shows only the plain defeat toast, no Haruhikage drama.
	if err := tt.Click("影色舞"); err != nil {
		t.Fatalf("select 《影色舞》: %v", err)
	}
	if app.diffIdx != 2 || app.game.Difficulty.ID != "kageiromai" {
		t.Fatalf("selected diffIdx=%d id=%q, want 2 / kageiromai", app.diffIdx, app.game.Difficulty.ID)
	}
	app.game.FirstClick = true
	app.game.Status = StatusPlaying
	app.game.Cell(0, 0).IsMine = true
	app.game.RecomputeAdjacency()
	x0, y0 := classicSurfaceCellAt(t, tt, app, 0, 0)
	tt.ClickAt(x0, y0)
	if app.game.Status != StatusLost {
		t.Fatalf("expected StatusLost on 《影色舞》 mine hit, got %v", app.game.Status)
	}
	if app.haruhikageIntro || app.haruhikageAlert || app.unlockedMemes[MemeHaruhikage] ||
		tt.HasText("……为什么要演奏春日影？！") || tt.HasText("为什么要演奏春日影？！") {
		t.Fatalf("《影色舞》 loss must not trigger Haruhikage drama or meme: intro=%v alert=%v memes=%v",
			app.haruhikageIntro, app.haruhikageAlert, app.unlockedMemes)
	}
	if !tt.HasText("炸团危机！……要假装无事发生重开吗？") {
		t.Fatal("expected plain defeat toast on 《影色舞》 loss")
	}
	tt.Advance(haruhikageIntroDuration + 50*time.Millisecond)
	if app.haruhikageIntro || app.haruhikageAlert || tt.HasText("为什么要演奏春日影？！") {
		t.Fatal("《影色舞》 loss must not open delayed Haruhikage dialog")
	}

	// AC 3: Losing the shipped 《春日影》 1000x1000 duel to Tomori AI triggers drama overlay, dialog, and meme.
	duelApp := newMinesweeperApp(42)
	duelApp.memberID = MemberTomori
	dtt := ui.NewTester(duelApp.view, 1280, 900)
	if err := dtt.Click("歌单"); err != nil {
		t.Fatalf("open setlist popover for duel: %v", err)
	}
	if err := dtt.Click("春日影"); err != nil {
		t.Fatalf("select 《春日影》 duel: %v", err)
	}
	if !duelApp.isDuel() || duelApp.game.Difficulty.Song != "春日影" || !duelApp.game.Difficulty.Haruhikage {
		t.Fatalf("expected shipped 春日影 duel active, got %+v", duelApp.game.Difficulty)
	}
	duelApp.game.FirstClick = true
	duelApp.game.Status = StatusPlaying
	duelApp.game.duel.PlayerScore = 20
	duelApp.game.duel.AIScore = 120
	duelApp.game.duel.NextAI = duelApp.game.duel.Duration + 10
	duelApp.game.duel.Elapsed = duelApp.game.duel.Duration - 0.5
	dtt.Frame()
	dtt.Advance(600 * time.Millisecond)
	if !duelApp.duelEnded || !duelApp.haruhikageIntro || duelApp.haruhikageAlert {
		t.Fatalf("expected Haruhikage drama overlay on 《春日影》 duel loss, ended=%v intro=%v alert=%v",
			duelApp.duelEnded, duelApp.haruhikageIntro, duelApp.haruhikageAlert)
	}
	dtt.Advance(haruhikageBeatDelay + 20*time.Millisecond)
	if !dtt.HasText("……为什么要演奏春日影？！") {
		t.Fatalf("expected Haruhikage line after half-beat on 《春日影》 duel loss")
	}
	if !duelApp.unlockedMemes[MemeHaruhikage] {
		t.Fatal("expected MemeHaruhikage unlocked on 《春日影》 duel loss")
	}
	if wantQuote := memberQuotes[MemberTomori]["haruhikageLost"][0]; duelApp.quote != wantQuote {
		t.Fatalf("expected haruhikageLost quote %q, got %q", wantQuote, duelApp.quote)
	}
	dtt.Advance(haruhikageIntroDuration + 50*time.Millisecond)
	if duelApp.haruhikageIntro || !duelApp.haruhikageAlert || !dtt.HasText("为什么要演奏春日影？！") {
		t.Fatalf("expected Haruhikage dialog after drama overlay, intro=%v alert=%v",
			duelApp.haruhikageIntro, duelApp.haruhikageAlert)
	}
}

func TestMemesAndChatCopyContracts(t *testing.T) {
	// Frozen meme count and IDs, updated Haruhikage duel trigger, and glossary checks.
	wantIDs := []string{MemeForever, MemeHaruhikage, MemeNeverHappy, MemeHuman, MemeMatcha, MemeAnonBest}
	if len(AllMemes) != len(wantIDs) {
		t.Fatalf("AllMemes count = %d, want %d", len(AllMemes), len(wantIDs))
	}
	for i, meme := range AllMemes {
		if meme.ID != wantIDs[i] {
			t.Fatalf("AllMemes[%d].ID = %q, want %q", i, meme.ID, wantIDs[i])
		}
		for _, bad := range []string{"难度", "地雷", "春日影 Live"} {
			if strings.Contains(meme.Desc, bad) {
				t.Fatalf("AllMemes[%s].Desc %q contains forbidden/outdated phrase %q", meme.ID, meme.Desc, bad)
			}
		}
		if meme.ID == MemeHaruhikage {
			if !strings.Contains(meme.Desc, "《春日影》") || !strings.Contains(meme.Desc, "决斗") || !strings.Contains(meme.Desc, "高松灯") {
				t.Fatalf("MemeHaruhikage.Desc = %q, want 《春日影》 duel defeat against 高松灯", meme.Desc)
			}
		}
	}

	// Frozen member taglines must remain untouched.
	wantTaglines := map[MemberID]string{
		MemberTomori:  "创可贴与石头收集家",
		MemberAnon:    "虚荣SNS偶像担当",
		MemberSoyorin: "重力微笑与拉黑之神",
		MemberTaki:    "压力怪 & 护灯狂魔",
		MemberRana:    "野猫 & 抹茶芭菲爱好者",
	}
	for _, m := range Members {
		if wantTaglines[m.ID] != m.Tagline {
			t.Fatalf("Members[%s].Tagline = %q, want %q", m.ID, m.Tagline, wantTaglines[m.ID])
		}
	}

	// Member quotes obey glossary and drop outdated single-cell Haruhikage wording.
	for memberID, events := range memberQuotes {
		for event, pool := range events {
			for _, line := range pool {
				for _, bad := range []string{"地雷", "难度"} {
					if strings.Contains(line, bad) {
						t.Fatalf("memberQuotes[%s][%s] %q contains forbidden word %q", memberID, event, line, bad)
					}
				}
				if event == "haruhikageLost" {
					for _, outdated := range []string{"盲猜", "点这格", "刚弹一半"} {
						if strings.Contains(line, outdated) {
							t.Fatalf("memberQuotes[%s][haruhikageLost] %q keeps outdated premise %q", memberID, line, outdated)
						}
					}
				}
			}
		}
	}

	// Initial load, track switching (including new tracks), and same-track restart chat entries.
	app := newMinesweeperApp(42)
	tt := ui.NewTester(app.view, 1280, 900)
	if len(app.chatLog) < 2 || app.chatLog[0].Speaker != "MyGO!!!!! 排练室" ||
		!strings.HasPrefix(app.chatLog[0].Text, "翻开谱本：《"+app.game.Difficulty.Song+"》") {
		t.Fatalf("unexpected initial chat entry: %+v", app.chatLog)
	}

	for _, diff := range Difficulties[1:] {
		if err := tt.Click("歌单"); err != nil {
			t.Fatalf("open setlist before %s: %v", diff.Song, err)
		}
		if err := tt.Click(diff.Song); err != nil {
			t.Fatalf("pick %s: %v", diff.Song, err)
		}
		wantEntry := fmt.Sprintf("翻到新谱：《%s》（%s）", diff.Song, diff.Subtitle)
		if len(app.chatLog) < 2 || app.chatLog[0].Speaker != "MyGO!!!!! 排练室" || app.chatLog[0].Text != wantEntry {
			t.Fatalf("switching to %s produced unexpected chatLog[0]: %+v (want %q)", diff.Song, app.chatLog, wantEntry)
		}
		if diff.Gameplay == GameplayDuel {
			if len(app.chatLog) < 3 || app.chatLog[1].Speaker != "MyGO!!!!! 决斗室" ||
				!strings.Contains(app.chatLog[1].Text, "手动插旗终局结算对错") {
				t.Fatalf("duel track %s missing updated duel chat intro: %+v", diff.Song, app.chatLog)
			}
		}
	}

	// Restarting the same track via R uses the restart entry.
	lastSong := Difficulties[len(Difficulties)-1].Song
	tt.Key(0, ui.KeyR)
	if len(app.chatLog) < 2 || app.chatLog[0].Speaker != "排练重开" ||
		app.chatLog[0].Text != "重新演奏《"+lastSong+"》" {
		t.Fatalf("same-track restart produced unexpected chatLog[0]: %+v", app.chatLog)
	}

	// Haruhikage duel defeat overlay, dialog, and Soyorin's apology reply use updated copy.
	if app.game.Difficulty.Song != "春日影" {
		if err := tt.Click("歌单"); err != nil {
			t.Fatalf("open setlist for 春日影: %v", err)
		}
		if err := tt.Click("春日影"); err != nil {
			t.Fatalf("select 春日影: %v", err)
		}
	}
	app.game.FirstClick = true
	app.game.Status = StatusPlaying
	app.game.duel.PlayerScore = 10
	app.game.duel.AIScore = 200
	app.game.duel.NextAI = app.game.duel.Duration + 10
	app.game.duel.Elapsed = app.game.duel.Duration - 0.5
	tt.Frame()
	tt.Advance(600 * time.Millisecond)
	tt.Advance(haruhikageBeatDelay + 20*time.Millisecond)

	if !tt.HasText("♪ 《春日影》 — 乐声骤停") || tt.HasText("Crychic") {
		t.Fatal("expected updated Haruhikage drama overlay header without Crychic")
	}
	if err := tt.Click("春日影演出遮罩"); err != nil {
		t.Fatalf("skip Haruhikage overlay: %v", err)
	}
	if !tt.HasText("明明说好不再弹这首曲子的") || tt.HasText("不点那一格") {
		t.Fatal("expected updated Haruhikage dialog body")
	}
	if err := tt.Click("对不起，我错了"); err != nil {
		t.Fatalf("dismiss Haruhikage dialog: %v", err)
	}
	lastMsg := app.chatLog[len(app.chatLog)-1]
	if lastMsg.Speaker != "长崎爽世" || !strings.Contains(lastMsg.Text, "擅自演奏这首曲子") {
		t.Fatalf("unexpected Soyorin apology reply: %+v", lastMsg)
	}
}

func TestUILabelsAndButtonsCopyContract(t *testing.T) {
	app := newMinesweeperApp(42)
	tt := ui.NewTester(app.view, 1280, 900)
	tt.UseScreenReader()

	for _, want := range []string{
		"设置",
		"插旗模式 (F)",
		"名场面图鉴",
		"乐队成员",
		"排练完成度",
		"乐队重力值",
		"剩雷 10",
		"用时 0 秒",
	} {
		if !tt.HasText(want) {
			t.Errorf("classic view missing finalized copy %q", want)
		}
	}
	for _, avoid := range []string{
		"MINES ",
		"BEST ",
		"TIME ",
		"乐队重力值 (Gravity)",
		"乐队成员应援",
		"标旗模式",
		"夜间台灯",
		"? 标记",
		"最快 --",
	} {
		if tt.HasText(avoid) {
			t.Errorf("classic view still contains retired or moved copy %q", avoid)
		}
	}
	if _, ok := tt.AccessByRole(ui.RoleProgress, "排练完成度"); !ok {
		t.Error("progress meter missing 排练完成度 progress accessibility node")
	}
	if _, ok := tt.AccessByRole(ui.RoleMeter, "乐队重力值"); !ok {
		t.Error("gravity meter missing 乐队重力值 meter accessibility node")
	}
	if _, ok := tt.Access("Live 完成度"); ok {
		t.Error("progress meter still uses retired Live 完成度 label")
	}
	if err := tt.Click("设置"); err != nil {
		t.Fatalf("opening settings popover failed: %v", err)
	}
	for _, want := range []string{
		"笔记本设置",
		"按 Esc 收起",
		"画面与动效",
		"夜间台灯",
		"动效",
		"跟随系统",
		"标准",
		"减弱",
		"声音",
		"音效",
		"音量",
		"65%",
		"扫雷操作",
		"右键 ? 标记",
		"右键贴过创可贴后，再点一次标上 ? 标记",
	} {
		if !tt.HasText(want) {
			t.Errorf("settings popover missing copy %q", want)
		}
	}
	if tt.HasText("偏好设置") {
		t.Error("settings popover still contains retired 偏好设置 heading")
	}
	tt.KeyDown(ui.KeyM)
	tt.KeyUp(ui.KeyM)
	if !tt.HasText("已静音 · 保留 65%") {
		t.Error("muted settings popover missing 已静音 · 保留 65% volume readout")
	}
	tt.KeyDown(ui.KeyM)
	tt.KeyUp(ui.KeyM)
	sw, ok := tt.AccessByRole(ui.RoleSwitch, "夜间台灯")
	if !ok {
		t.Fatal("night lamp switch missing 夜间台灯 accessibility label inside settings popover")
	}
	wasDark := app.darkMode
	tt.ClickAt(sw.Bounds.X+sw.Bounds.W/2, sw.Bounds.Y+sw.Bounds.H/2)
	if app.darkMode == wasDark {
		t.Fatal("clicking 夜间台灯 switch did not toggle darkMode")
	}
	tt.Key(0, ui.KeyEscape)

	if err := tt.Click("名场面图鉴"); err != nil {
		t.Fatalf("opening memes drawer failed: %v", err)
	}
	if !tt.HasText("MyGO!!!!! 名场面图鉴") || tt.HasText("MyGO!!!!! 名场面梗图鉴") {
		t.Error("memes drawer heading did not use finalized MyGO!!!!! 名场面图鉴")
	}

	if err := tt.Click("纯正经典"); err != nil {
		t.Fatalf("switching to 纯正经典 failed: %v", err)
	}
	if !tt.HasText("经典模式") || tt.HasText("语音应援") {
		t.Error("纯正经典 skill entry did not read 经典模式")
	}

	// The status bar formats elapsed seconds in Chinese and leaves the best
	// time to the setlist.
	app.bestTimes[app.game.Difficulty.ID] = 23
	app.elapsed = 23
	tt.Frame()
	if !tt.HasText("用时 23 秒") || tt.HasText("最快 23 秒") {
		t.Error("status bar should show 用时 23 秒 without the 最快 23 秒 setlist record")
	}

	// Duel view: finalized HUD labels, buttons, and board accessibility label.
	duelApp := newMinesweeperApp(42)
	duelApp.diffIdx = findDifficultyIdx("duel")
	duelApp.resetGame(false)
	dtt := ui.NewTester(duelApp.view, 1280, 900)
	for _, want := range []string{
		"你 0",
		"高松灯 0",
		"回到中心",
		"定位高松灯",
		"剩余 05:00",
		"连击 ×0",
	} {
		if !dtt.HasText(want) {
			t.Errorf("duel view missing finalized copy %q", want)
		}
	}
	for _, avoid := range []string{
		"灯 (AI)",
		"回到战场",
		"追踪 AI",
		"旗帜 ",
		"数字全未知时",
		"确定全雷时左键点数字收雷 · +75/雷",
		"插旗 0",
	} {
		if dtt.HasText(avoid) {
			t.Errorf("duel view still contains retired copy %q", avoid)
		}
	}
	if _, ok := dtt.Find("决斗棋盘"); !ok {
		t.Error("duel board missing 决斗棋盘 label")
	}
	if _, ok := dtt.Find("百万格决斗战场"); ok {
		t.Error("duel board still uses banned 百万格决斗战场 label")
	}

	// Clicking 定位高松灯 glides the viewport smoothly onto Tomori's last move.
	duelApp.game.duel.LastAI = 120*duelApp.game.Cols + 240
	duelApp.board.center(duelApp.game, 500, 500)
	dtt.Frame()
	if err := dtt.Click("定位高松灯"); err != nil {
		t.Fatalf("clicking 定位高松灯 failed: %v", err)
	}
	r, _ := dtt.Find("决斗棋盘")
	if !duelApp.board.gliding() {
		t.Fatal("定位高松灯 did not start camera glide")
	}
	x0, y0 := duelApp.board.X, duelApp.board.Y
	dtt.Advance(60 * time.Millisecond)
	if !duelApp.board.gliding() || (duelApp.board.X == x0 && duelApp.board.Y == y0) {
		t.Fatalf("定位高松灯 mid-glide not in transition: start=(%v,%v) mid=(%v,%v) gliding=%v",
			x0, y0, duelApp.board.X, duelApp.board.Y, duelApp.board.gliding())
	}
	dtt.Advance(200 * time.Millisecond)
	if duelApp.board.gliding() {
		t.Fatal("定位高松灯 still gliding after 260ms")
	}
	gotRow, gotCol, ok := duelApp.board.cellAt(duelApp.game, r.W/2, r.H/2)
	if !ok || gotRow != 120 || gotCol != 240 {
		t.Fatalf("定位高松灯 centered on (%d,%d) ok=%v, want (120,240)", gotRow, gotCol, ok)
	}
}

func TestSnapshotMemesDialog(t *testing.T) {
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
		app.classicHoverRow, app.classicHoverCol = -1, -1
		app.showMemes = true
		app.unlockedMemes[MemeForever] = true
		app.unlockedMemes[MemeHaruhikage] = true
		name := fmt.Sprintf("memes-dialog-default-%s.png", suffix)
		writeSnapshot(t, app, false, 1280, 900, filepath.Join(dir, name))
	}
}

func pixelNearColor(got color.Color, want ui.Color, tol int) bool {
	gr, gg, gb, _ := got.RGBA()
	diff := func(a, b int) int {
		if a > b {
			return a - b
		}
		return b - a
	}
	return diff(int(gr>>8), int(want.R)) <= tol &&
		diff(int(gg>>8), int(want.G)) <= tol &&
		diff(int(gb>>8), int(want.B)) <= tol
}

func rectContainsColor(img image.Image, r ui.Rect, want ui.Color, tol int) bool {
	b := img.Bounds()
	minX := max(b.Min.X, int(r.X))
	maxX := min(b.Max.X-1, int(r.X+r.W))
	minY := max(b.Min.Y, int(r.Y))
	maxY := min(b.Max.Y-1, int(r.Y+r.H))
	for y := minY; y <= maxY; y += 2 {
		for x := minX; x <= maxX; x += 2 {
			if pixelNearColor(img.At(x, y), want, tol) {
				return true
			}
		}
	}
	return false
}

func TestHaruhikagePaperThemeContrastPixelsAndFlows(t *testing.T) {
	setupLoss := func(dark bool) (*minesweeperApp, *ui.Tester) {
		app := newMinesweeperApp(42)
		app.darkMode = dark
		app.game = NewGame(Difficulty{
			ID:           "marked-classic",
			Label:        "春日影 Live",
			Song:         "春日影",
			Gameplay:     GameplayClassic,
			Haruhikage:   true,
			Rows:         9,
			Cols:         9,
			Mines:        1,
			SkillCharges: 3,
		}, true, MemberTomori, 7)
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

	for _, dark := range []bool{false, true} {
		mode := "day"
		if dark {
			mode = "night"
		}
		t.Run(mode, func(t *testing.T) {
			pal := paperFor(dark)
			drama := haruhikageFor(dark)

			// 1. Contrast checks for drama overlay card and questioning dialog.
			pairs := []struct {
				name   string
				fg, bg ui.Color
			}{
				{"eyebrow on card", drama.Eyebrow, drama.CardBg},
				{"title on card", drama.Title, drama.CardBg},
				{"subtitle on card", drama.Subtitle, drama.CardBg},
				{"dialog text on bg", drama.DialogText, drama.DialogBg},
				{"dialog text on surface", drama.DialogText, drama.DialogSurface},
				{"dialog muted on bg", drama.DialogMuted, drama.DialogBg},
				{"dialog accent text on accent", drama.DialogAccentText, drama.DialogAccent},
				{"dialog accent on bg", drama.DialogAccent, drama.DialogBg},
			}
			for _, p := range pairs {
				if cr := contrast(p.fg, p.bg); cr < minInkContrast {
					t.Errorf("%s contrast = %.2f (< %.1f), fg=%s bg=%s", p.name, cr, minInkContrast, hexOf(p.fg), hexOf(p.bg))
				}
			}

			// 2. Headless overlay render pixel verification + click-to-skip flow.
			app, tt := setupLoss(dark)
			baseTheme := app.activeTheme()
			baseAccent, baseBg := baseTheme.Accent, baseTheme.Background
			dlgTheme := drama.dialogTheme(baseTheme)
			if dlgTheme.Font != fontFamily || baseTheme.Accent != baseAccent || baseTheme.Background != baseBg {
				t.Fatalf("dialogTheme mutated baseTheme or dropped font: font=%q", dlgTheme.Font)
			}
			if !app.haruhikageIntro || app.haruhikageAlert {
				t.Fatalf("expected drama overlay active, intro=%v alert=%v", app.haruhikageIntro, app.haruhikageAlert)
			}
			tt.Advance(haruhikageBeatDelay + haruhikageLineFade + 20*time.Millisecond)
			tt.Frame()
			img := tt.Image()
			wantScrimOnDesk := drama.Scrim.Over(pal.Desk)
			if got := img.At(1270, 890); !pixelNearColor(got, wantScrimOnDesk, 6) {
				r, g, b, _ := got.RGBA()
				t.Errorf("overlay scrim pixel at (1270,890) = #%02X%02X%02X, want near %s", r>>8, g>>8, b>>8, hexOf(wantScrimOnDesk))
			}
			titleRect, ok := tt.Find("「……为什么要演奏春日影？！」")
			if !ok {
				t.Fatal("drama overlay title not found")
			}
			cardPadBand := ui.Rect{X: titleRect.X - 20, Y: titleRect.Y, W: 16, H: titleRect.H}
			if !rectContainsColor(img, cardPadBand, drama.CardBg, 4) {
				t.Errorf("drama card padding around %+v did not render palette CardBg %s", titleRect, hexOf(drama.CardBg))
			}

			if err := tt.Click("春日影演出遮罩"); err != nil {
				t.Fatalf("failed clicking drama overlay: %v", err)
			}
			if app.haruhikageIntro || !app.haruhikageAlert || !tt.HasText("为什么要演奏春日影？！") {
				t.Fatalf("expected click-to-skip to open AlertDialog, intro=%v alert=%v", app.haruhikageIntro, app.haruhikageAlert)
			}
			tt.Frame()
			dlgImg := tt.Image()
			dlgRect, ok := tt.Find("为什么要演奏春日影？！")
			if !ok {
				t.Fatal("AlertDialog title not found")
			}
			dlgPadBand := ui.Rect{X: dlgRect.X + 8, Y: dlgRect.Y + 6, W: dlgRect.W - 16, H: 14}
			if !rectContainsColor(dlgImg, dlgPadBand, drama.DialogBg, 4) {
				t.Errorf("AlertDialog header band %+v did not render palette DialogBg %s", dlgPadBand, hexOf(drama.DialogBg))
			}
			if forgiveRect, ok := tt.Find("对不起，我错了"); !ok || !rectContainsColor(dlgImg, forgiveRect, drama.DialogSurface, 4) {
				t.Errorf("AlertDialog secondary button %+v (found=%v) did not render palette DialogSurface %s", forgiveRect, ok, hexOf(drama.DialogSurface))
			}
			restartRect, ok := tt.Find("一辈子重开")
			if !ok || !rectContainsColor(dlgImg, restartRect, drama.DialogAccent, 4) {
				t.Errorf("AlertDialog primary button %+v (found=%v) did not render palette DialogAccent %s", restartRect, ok, hexOf(drama.DialogAccent))
			}

			// Verify keyboard focus stays scoped to the dialog theme (not baseTheme.Focus).
			tt.Key(0, ui.KeyTab)
			tt.Frame()
			focusImg := tt.Image()
			forgiveRect, _ := tt.Find("对不起，我错了")
			ringBand := ui.Rect{
				X: min(forgiveRect.X, restartRect.X) - 6,
				Y: min(forgiveRect.Y, restartRect.Y) - 6,
				W: max(forgiveRect.X+forgiveRect.W, restartRect.X+restartRect.W) - min(forgiveRect.X, restartRect.X) + 12,
				H: max(forgiveRect.Y+forgiveRect.H, restartRect.Y+restartRect.H) - min(forgiveRect.Y, restartRect.Y) + 12,
			}
			if rectContainsColor(focusImg, ringBand, baseTheme.Focus, 6) {
				t.Errorf("AlertDialog keyboard focus ring leaked baseTheme.Focus %s instead of dialog theme focus %s", hexOf(baseTheme.Focus), hexOf(dlgTheme.Focus))
			}

			if err := tt.Click("对不起，我错了"); err != nil {
				t.Fatalf("failed clicking forgive button: %v", err)
			}
			if app.haruhikageAlert || app.game.Status != StatusLost {
				t.Fatalf("expected dialog dismissed keeping lost board, alert=%v status=%v", app.haruhikageAlert, app.game.Status)
			}
			lastChat := app.chatLog[len(app.chatLog)-1]
			if lastChat.Speaker != "长崎爽世" || !strings.Contains(lastChat.Text, "擅自演奏这首曲子") {
				t.Fatalf("expected Soyorin apology reply, got %+v", lastChat)
			}

			// 3. Timed auto-transition to AlertDialog + restart button flow.
			app2, tt2 := setupLoss(dark)
			tt2.Advance(haruhikageIntroDuration + 50*time.Millisecond)
			if app2.haruhikageIntro || !app2.haruhikageAlert || !tt2.HasText("为什么要演奏春日影？！") {
				t.Fatalf("expected timed transition to AlertDialog, intro=%v alert=%v", app2.haruhikageIntro, app2.haruhikageAlert)
			}
			if err := tt2.Click("一辈子重开"); err != nil {
				t.Fatalf("failed clicking restart button: %v", err)
			}
			if app2.game.Status != StatusIdle || app2.haruhikageAlert || app2.haruhikageIntro {
				t.Fatalf("expected clean reset after restart, status=%v intro=%v alert=%v", app2.game.Status, app2.haruhikageIntro, app2.haruhikageAlert)
			}
		})
	}

	// 4. Whole-repo scanreport must report zero copy violations and zero raw colours.
	rep, err := scanreport.Scan(".")
	if err != nil {
		t.Fatalf("scanreport.Scan failed: %v", err)
	}
	if len(rep.Copy) != 0 || len(rep.Colors) != 0 {
		t.Fatalf("expected zero scanreport findings, got:\n%s", rep)
	}
}

func TestProgressAndGravityMeterA11yAndHatching(t *testing.T) {
	setupBoard := func(dark bool) (*minesweeperApp, *ui.Tester) {
		app := newMinesweeperApp(42)
		app.darkMode = dark
		app.game = NewGame(Difficulty{
			ID:       "meter-test",
			Song:     "迷星叫",
			Gameplay: GameplayClassic,
			Rows:     5,
			Cols:     5,
			Mines:    2,
		}, false, MemberTomori, 7)
		app.game.FirstClick = true
		app.game.Status = StatusPlaying
		app.game.Cell(0, 0).IsMine = true
		app.game.Cell(4, 4).IsMine = true
		app.game.RecomputeAdjacency()
		tt := ui.NewTester(app.view, 1280, 900)
		tt.UseScreenReader()
		tt.Frame()
		return app, tt
	}

	barHasColor := func(img image.Image, r ui.Rect, want ui.Color, tol int) bool {
		b := img.Bounds()
		minX := max(b.Min.X, int(r.X))
		maxX := min(b.Max.X-1, int(r.X+r.W))
		minY := max(b.Min.Y, int(r.Y))
		maxY := min(b.Max.Y-1, int(r.Y+r.H))
		for y := minY; y <= maxY; y++ {
			for x := minX; x <= maxX; x++ {
				if pixelNearColor(img.At(x, y), want, tol) {
					return true
				}
			}
		}
		return false
	}

	for _, dark := range []bool{false, true} {
		mode := "day"
		if dark {
			mode = "night"
		}
		t.Run(mode, func(t *testing.T) {
			pal := paperFor(dark)
			progInk := pal.InkOf(bandColor)
			// The progress bar sits in the status bar, a desk tint over the sheet.
			progWash := progInk.Alpha(0.22).Over(pal.Desk.Alpha(0.75).Over(pal.Sheet))
			gravInk := pal.InkOf(memberImage(MemberSoyorin))
			gravWash := gravInk.Alpha(0.22).Over(pal.Sheet)

			// 1. Initial state: Progress=0, Gravity starts at NewGame's baseline (>0); native roles exposed (not RoleGroup).
			app, tt := setupBoard(dark)
			if _, ok := tt.AccessByRole(ui.RoleGroup, "排练完成度"); ok {
				t.Fatal("排练完成度 should not fall back to RoleGroup")
			}
			if _, ok := tt.AccessByRole(ui.RoleGroup, "乐队重力值"); ok {
				t.Fatal("乐队重力值 should not fall back to RoleGroup")
			}
			prog, ok := tt.AccessByRole(ui.RoleProgress, "排练完成度")
			if !ok {
				t.Fatal("missing RoleProgress for 排练完成度")
			}
			if prog.Min != 0 || prog.Max != 1 || prog.Now != 0 {
				t.Fatalf("initial progress range/value = [%.2f,%.2f] now=%.2f, want [0,1] now=0", prog.Min, prog.Max, prog.Now)
			}
			meter, ok := tt.AccessByRole(ui.RoleMeter, "乐队重力值")
			if !ok {
				t.Fatal("missing RoleMeter for 乐队重力值")
			}
			initGravity := float64(app.game.Gravity)
			if initGravity <= 0 || initGravity >= 100 {
				t.Fatalf("expected initial gravity in (0, 100), got %d", app.game.Gravity)
			}
			if meter.Min != 0 || meter.Max != 100 || meter.Now != initGravity {
				t.Fatalf("initial gravity range/value = [%.0f,%.0f] now=%.0f, want [0,100] now=%.0f", meter.Min, meter.Max, meter.Now, initGravity)
			}
			initImg := tt.Image()
			initGravFill := ui.Rect{
				X: meter.Bounds.X + 3,
				Y: meter.Bounds.Y + 2,
				W: meter.Bounds.W*float32(initGravity/100) - 6,
				H: meter.Bounds.H - 4,
			}
			if !barHasColor(initImg, initGravFill, gravInk, 18) {
				t.Errorf("initial gravity meter missing pencil hatch ink %s in %+v", hexOf(gravInk), initGravFill)
			}
			initGravEmpty := ui.Rect{
				X: meter.Bounds.X + meter.Bounds.W*float32(initGravity/100) + 6,
				Y: meter.Bounds.Y + 2,
				W: meter.Bounds.W*float32(1-initGravity/100) - 10,
				H: meter.Bounds.H - 4,
			}
			if barHasColor(initImg, initGravEmpty, gravInk, 18) {
				t.Errorf("initial gravity meter leaked hatch ink into unfilled span %+v", initGravEmpty)
			}

			// 2. Partial reveal: opening numbered cells (no flood-fill) advances Progress and draws pencil hatching only inside the filled span.
			for _, label := range []string{"cell-0-1", "cell-1-0", "cell-1-1", "cell-3-3", "cell-3-4", "cell-4-3"} {
				if err := tt.Click(label); err != nil {
					t.Fatalf("click %s failed: %v", label, err)
				}
			}
			tt.Frame()
			wantProg := app.game.Progress()
			if wantProg <= 0.2 || wantProg >= 1 {
				t.Fatalf("expected partial progress in (0.2, 1), got %.4f", wantProg)
			}
			prog, ok = tt.AccessByRole(ui.RoleProgress, "排练完成度")
			if !ok || prog.Now != wantProg {
				t.Fatalf("mid-game progress access = %+v (ok=%v), want Now=%.4f", prog, ok, wantProg)
			}
			tt.Advance(meterGlideDuration + 20*time.Millisecond)
			img := tt.Image()
			filledSpan := ui.Rect{
				X: prog.Bounds.X + 4,
				Y: prog.Bounds.Y + 2,
				W: prog.Bounds.W*float32(wantProg) - 8,
				H: prog.Bounds.H - 4,
			}
			if !barHasColor(img, filledSpan, progInk, 18) {
				t.Errorf("mid-game progress bar missing pencil hatch ink %s in %+v", hexOf(progInk), filledSpan)
			}
			if !barHasColor(img, filledSpan, progWash, 12) {
				t.Errorf("mid-game progress bar missing translucent fill wash %s in %+v", hexOf(progWash), filledSpan)
			}
			unfilledSpan := ui.Rect{
				X: prog.Bounds.X + prog.Bounds.W*float32(wantProg) + 6,
				Y: prog.Bounds.Y + 2,
				W: prog.Bounds.W*float32(1-wantProg) - 10,
				H: prog.Bounds.H - 4,
			}
			if unfilledSpan.W > 4 && barHasColor(img, unfilledSpan, progInk, 18) {
				t.Errorf("mid-game progress bar leaked hatch ink into unfilled span %+v", unfilledSpan)
			}

			// 3. Win state: Progress=1.0, Gravity=0; full progress bar hatched, gravity bar stays empty.
			for i := range app.game.Cells {
				cell := &app.game.Cells[i]
				if !cell.IsMine && !cell.Revealed {
					app.game.Reveal(cell.Row, cell.Col)
				}
			}
			tt.Frame()
			if app.game.Status != StatusWon || app.game.Progress() != 1 || app.game.Gravity != 0 {
				t.Fatalf("expected won state (progress=1, gravity=0), got status=%v progress=%.2f gravity=%d",
					app.game.Status, app.game.Progress(), app.game.Gravity)
			}
			prog, ok = tt.AccessByRole(ui.RoleProgress, "排练完成度")
			if !ok || prog.Now != 1 {
				t.Fatalf("won progress access = %+v (ok=%v), want Now=1", prog, ok)
			}
			meter, ok = tt.AccessByRole(ui.RoleMeter, "乐队重力值")
			if !ok || meter.Now != 0 {
				t.Fatalf("won gravity access = %+v (ok=%v), want Now=0", meter, ok)
			}
			tt.Advance(meterGlideDuration + 20*time.Millisecond)
			winImg := tt.Image()
			winTail := ui.Rect{
				X: prog.Bounds.X + prog.Bounds.W - 24,
				Y: prog.Bounds.Y + 2,
				W: 18,
				H: prog.Bounds.H - 4,
			}
			if !barHasColor(winImg, winTail, progInk, 18) {
				t.Errorf("won progress bar tail %+v missing hatch ink %s", winTail, hexOf(progInk))
			}
			emptyMeter := ui.Rect{
				X: meter.Bounds.X + 4,
				Y: meter.Bounds.Y + 2,
				W: meter.Bounds.W - 8,
				H: meter.Bounds.H - 4,
			}
			if barHasColor(winImg, emptyMeter, gravInk, 18) {
				t.Errorf("zero gravity meter unexpectedly drew hatch ink %s", hexOf(gravInk))
			}

			// 4. Mine hit (loss): Gravity=100; meter exposes 100/100 and draws Soyorin pencil hatching.
			lostApp, lostTT := setupBoard(dark)
			if err := lostTT.Click("cell-0-1"); err != nil {
				t.Fatalf("click safe cell before mine hit failed: %v", err)
			}
			if err := lostTT.Click("cell-0-0"); err != nil {
				t.Fatalf("click mine cell-0-0 failed: %v", err)
			}
			lostTT.Frame()
			if lostApp.game.Status != StatusLost || lostApp.game.Gravity != 100 {
				t.Fatalf("expected lost state with gravity=100, got status=%v gravity=%d", lostApp.game.Status, lostApp.game.Gravity)
			}
			lostProg, ok := lostTT.AccessByRole(ui.RoleProgress, "排练完成度")
			if !ok || lostProg.Now != lostApp.game.Progress() {
				t.Fatalf("lost progress access = %+v (ok=%v), want Now=%.4f", lostProg, ok, lostApp.game.Progress())
			}
			lostMeter, ok := lostTT.AccessByRole(ui.RoleMeter, "乐队重力值")
			if !ok || lostMeter.Min != 0 || lostMeter.Max != 100 || lostMeter.Now != 100 {
				t.Fatalf("lost gravity access = %+v (ok=%v), want [0,100] Now=100", lostMeter, ok)
			}
			lostTT.Advance(meterGlideDuration + 20*time.Millisecond)
			lostImg := lostTT.Image()
			gravInterior := ui.Rect{
				X: lostMeter.Bounds.X + 4,
				Y: lostMeter.Bounds.Y + 2,
				W: lostMeter.Bounds.W - 8,
				H: lostMeter.Bounds.H - 4,
			}
			if !barHasColor(lostImg, gravInterior, gravInk, 18) {
				t.Errorf("lost gravity meter missing pencil hatch ink %s in %+v", hexOf(gravInk), gravInterior)
			}
			if !barHasColor(lostImg, gravInterior, gravWash, 12) {
				t.Errorf("lost gravity meter missing translucent fill wash %s in %+v", hexOf(gravWash), gravInterior)
			}
		})
	}
}

func TestSnapshotHaruhikage(t *testing.T) {
	dir := os.Getenv("MYGO_SNAPSHOT_DIR")
	if dir == "" {
		t.Skip("set MYGO_SNAPSHOT_DIR to render snapshots")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeHaruhikage := func(dark, showDialog bool, width, height int, filename string) {
		app := stageDuelScene(dark)
		tt := ui.NewTester(app.view, width, height)
		tt.SetScale(2)
		elapsed := app.game.duel.Elapsed
		app.game.duel.NextAI = 1e9
		tt.Advance(time.Duration(1200-tt.Now().UnixMilli()%1200) * time.Millisecond)
		app.game.duel.Elapsed = elapsed
		app.elapsed = int(elapsed)
		app.lastTick = tt.Now()
		app.board.center(app.game, app.game.Rows/2, app.game.Cols/2-2)
		if showDialog {
			app.haruhikageIntro = false
			app.haruhikageAlert = true
		} else {
			app.haruhikageIntro = true
			app.haruhikageIntroStart = tt.Now().Add(-(haruhikageBeatDelay + haruhikageLineFade))
			app.haruhikageAlert = false
		}
		tt.Frame()
		tt.Frame()
		f, err := os.Create(filepath.Join(dir, filename))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
	for _, dark := range []bool{false, true} {
		suffix := "day"
		if dark {
			suffix = "night"
		}
		for _, vp := range snapshotViewports {
			writeHaruhikage(dark, false, vp.width, vp.height, fmt.Sprintf("haruhikage-overlay-%s-%s.png", vp.name, suffix))
			writeHaruhikage(dark, true, vp.width, vp.height, fmt.Sprintf("haruhikage-dialog-%s-%s.png", vp.name, suffix))
		}
	}
}
