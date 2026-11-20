package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image/png"
	"os"
	"slices"
	"strings"
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

	if !tt.HasText("MyGO!!!!! 迷子扫雷") || !tt.HasText("MINES 10") {
		t.Fatalf("expected title and MINES 010 badge in initial view")
	}

	// Setlist popover switches track to 碧天伴走 (RiNG 排练室)
	if err := tt.Click("歌单"); err != nil {
		t.Fatalf("failed opening setlist popover: %v", err)
	}
	if err := tt.Click("碧天伴走"); err != nil {
		t.Fatalf("failed selecting 碧天伴走 from setlist popover: %v", err)
	}
	if app.diffIdx != 1 || app.game.Cols != 16 || !tt.HasText("MINES 40") {
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
	for i, diff := range Difficulties {
		if diff.Gameplay != GameplayClassic {
			continue
		}
		app := newMinesweeperApp(12345)
		app.diffIdx = i
		app.resetGame(false)
		tt := ui.NewTester(app.view, 1280, 900)
		tt.Frame()
		if diff.Rows > 30 || diff.Cols > 30 {
			// Classic boards past 30x30 route onto the shared paper canvas (#35).
			r, ok := tt.Find("经典棋盘")
			if !ok || r.W <= 0 || r.H <= 0 || r.X < 0 || r.Y < 0 || r.X+r.W > 1280 || r.Y+r.H > 900 {
				t.Fatalf("difficulty %d (%s) classic canvas bounds invalid: %+v found=%v", i, diff.ID, r, ok)
			}
			if _, hasCell := tt.Find("cell-0-0"); hasCell {
				t.Fatalf("difficulty %d (%s) rendered button grid instead of classic canvas", i, diff.ID)
			}
			continue
		}
		if diff.Rows > 20 {
			// Classic boards taller than 20 rows keep the 22px cell floor and scroll inside ScrollBoth.
			r, ok := tt.Find("cell-0-0")
			if !ok || r.W < 22 || r.H < 22 {
				t.Fatalf("difficulty %d (%s) top-left cell invalid: %+v found=%v", i, diff.ID, r, ok)
			}
			continue
		}
		label := fmt.Sprintf("cell-%d-%d", app.game.Rows-1, app.game.Cols-1)
		r, ok := tt.Find(label)
		if !ok || r.Y+r.H > 900 {
			t.Fatalf("difficulty %d (%s) bottom cell clipped: %+v", i, diff.ID, r)
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
	if !app1.haruhikageIntro || app1.haruhikageAlert || !tt1.HasText("……为什么要演奏春日影？！") {
		t.Fatalf("expected Haruhikage overlay on marked non-hard track, intro=%v alert=%v", app1.haruhikageIntro, app1.haruhikageAlert)
	}
	tt1.Advance(haruhikageIntroDuration - 100*time.Millisecond)
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
	if _, ok := tt4.Find("百万格决斗战场"); !ok {
		t.Fatal("duel board missing")
	}
	app4.pendingResult = app4.game.Reveal(0, 0)
	tt4.Frame()
	if app4.haruhikageIntro || app4.haruhikageAlert || tt4.HasText("……为什么要演奏春日影？！") {
		t.Fatalf("expected no Haruhikage presentation on marked duel mine hit, intro=%v alert=%v", app4.haruhikageIntro, app4.haruhikageAlert)
	}
	if !tt4.HasText("踩雷 −100 · 硬直 2.4 秒！别把春日影点出来") {
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
	if !app1.duelEnded || !app1.haruhikageIntro || app1.haruhikageAlert || !tt1.HasText("……为什么要演奏春日影？！") {
		t.Fatalf("expected Haruhikage overlay on marked duel score loss, ended=%v intro=%v alert=%v", app1.duelEnded, app1.haruhikageIntro, app1.haruhikageAlert)
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
	if !app3.duelEnded || !app3.haruhikageIntro || app3.haruhikageAlert || !tt3.HasText("……为什么要演奏春日影？！") {
		t.Fatalf("expected Haruhikage overlay on cleared-board marked duel loss, ended=%v intro=%v alert=%v", app3.duelEnded, app3.haruhikageIntro, app3.haruhikageAlert)
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
	if !appAI.duelEnded || !appAI.haruhikageIntro || appAI.haruhikageAlert || !ttAI.HasText("……为什么要演奏春日影？！") {
		t.Fatalf("expected Haruhikage overlay on AI-cleared score loss, ended=%v intro=%v alert=%v", appAI.duelEnded, appAI.haruhikageIntro, appAI.haruhikageAlert)
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
	cell0, ok := tt.Find("cell-0-0")
	if !ok {
		t.Fatal("cell-0-0 not found on 《影色舞》")
	}
	tt.ClickAt(cell0.X+cell0.W/2, cell0.Y+cell0.H/2)
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
	if !duelApp.duelEnded || !duelApp.haruhikageIntro || duelApp.haruhikageAlert || !dtt.HasText("……为什么要演奏春日影？！") {
		t.Fatalf("expected Haruhikage drama overlay on 《春日影》 duel loss, ended=%v intro=%v alert=%v",
			duelApp.duelEnded, duelApp.haruhikageIntro, duelApp.haruhikageAlert)
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
