package main

import (
	"math/rand/v2"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// TestSenzaihyoumeiDefinition pins TrackSenzaihyoumei's 500x500 duel definition
// (45,000 mines, 5 skill charges, 3 minutes, 1.0s AI interval) and verifies it
// is listed first in the duel group of Difficulties (#38).
func TestSenzaihyoumeiDefinition(t *testing.T) {
	d := TrackSenzaihyoumei
	if d.ID != "senzaihyoumei" || d.Subtitle != "500×500 · 45000雷 · 03:00" {
		t.Fatalf("TrackSenzaihyoumei = %+v, want id senzaihyoumei, subtitle 500×500 · 45000雷 · 03:00", d)
	}
	if d.Rows != 500 || d.Cols != 500 || d.Mines != 45000 || d.SkillCharges != 5 {
		t.Fatalf("TrackSenzaihyoumei = %+v, want 500x500, 45000 mines, 5 charges", d)
	}
	if d.Gameplay != GameplayDuel || d.Song != "潜在表明" || d.Label != "潜在表明" {
		t.Fatalf("TrackSenzaihyoumei = %+v, want duel 《潜在表明》", d)
	}
	if d.Haruhikage {
		t.Fatal("senzaihyoumei must not carry the haruhikage drama marker")
	}
	if got := float64(d.Mines) / float64(d.Rows*d.Cols); got < 0.17 || got > 0.19 {
		t.Fatalf("mine density %.3f, want ~18%%", got)
	}
	groups := groupSetlistTracks(Difficulties)
	if len(groups) != 2 || groups[1].Gameplay != GameplayDuel || groups[1].Title != "决斗" {
		t.Fatalf("setlist groups = %+v, want classic and duel groups", groups)
	}
	duelEntries := groups[1].Entries
	if len(duelEntries) != 2 || duelEntries[0].Diff != d || duelEntries[1].Diff.ID != "duel" {
		t.Fatalf("duel group entries = %+v, want [TrackSenzaihyoumei, duel]", duelEntries)
	}
	g := NewGame(d, true, MemberTomori, 1)
	if g.Rows != 500 || g.Cols != 500 || g.Mines != 45000 || g.SkillUsesLeft != 5 {
		t.Fatalf("NewGame(TrackSenzaihyoumei) = %dx%d %d mines %d charges, want 500x500 45000 mines 5 charges",
			g.Rows, g.Cols, g.Mines, g.SkillUsesLeft)
	}
	if g.duel == nil || g.duel.Duration != 180 || g.duel.AIInterval != 1.0 {
		t.Fatalf("duel = %+v, want 180s duration and 1.0s AI interval", g.duel)
	}
}

// TestSenzaihyoumeiDurationEndsMatch: the 3-minute clock settles the match
// when it runs out, not a moment sooner.
func TestSenzaihyoumeiDurationEndsMatch(t *testing.T) {
	g := NewGame(TrackSenzaihyoumei, true, MemberTomori, 42)
	if res := g.Reveal(250, 250); res.Kind == ActionNoop {
		t.Fatal("first click did not start the duel")
	}
	mines := 0
	for i := range g.Cells {
		if g.Cells[i].IsMine {
			mines++
		}
	}
	if mines != 45000 {
		t.Fatalf("board holds %d mines, want 45000", mines)
	}
	if tick := g.TickDuel(179.9); tick.Finished {
		t.Fatal("duel finished before its 180s duration")
	}
	tick := g.TickDuel(0.1)
	if !tick.Finished || g.Status != StatusWon || !g.duel.Settled {
		t.Fatalf("at 180s: finished=%v status=%v settled=%v", tick.Finished, g.Status, g.duel.Settled)
	}
	if g.duel.Elapsed != 180 {
		t.Fatalf("elapsed %v, want clamped to 180", g.duel.Elapsed)
	}
}

// TestSenzaihyoumeiAIPace: Tomori steps once per interval — a tick that
// reaches NextAI yields exactly one AI turn and re-arms 1.0s out (2.4s when
// the AI hits a mine and slows down).
func TestSenzaihyoumeiAIPace(t *testing.T) {
	g := NewGame(TrackSenzaihyoumei, true, MemberTomori, 42)
	if res := g.Reveal(250, 250); res.Kind == ActionNoop {
		t.Fatal("first click did not start the duel")
	}
	// Seed the AI picker like qa_independent_test.go does: with an empty
	// frontier its first IntN lands on a probed safe cell, so the 1.0s
	// re-arm branch is exercised deterministically, not by luck.
	seed := uint64(0)
	for {
		probe := rand.New(rand.NewPCG(seed, 0))
		if c := &g.Cells[probe.IntN(len(g.Cells))]; !c.IsMine && !c.Revealed && !c.Flagged && c.Owner != DuelPlayer {
			break
		}
		seed++
	}
	g.duel.rng = rand.New(rand.NewPCG(seed, 0))
	g.duel.frontier = map[int]bool{}
	if tick := g.TickDuel(1.0); tick.AITurn != nil {
		t.Fatalf("AI moved at %.1fs, before its first 1.1s step", g.duel.Elapsed)
	}
	moves, safeMoves := 0, 0
	for g.Status == StatusPlaying && g.duel.Elapsed < 6 {
		next := g.duel.NextAI
		tick := g.TickDuel(0.4)
		if tick.AITurn == nil {
			if g.duel.Elapsed >= next {
				t.Fatalf("no AI turn at %.2fs though NextAI was %.2f", g.duel.Elapsed, next)
			}
			continue
		}
		moves++
		want := g.duel.Elapsed + 1.0
		if tick.AITurn.Kind == ActionLost {
			want = g.duel.Elapsed + 2.4
		} else {
			safeMoves++
		}
		if g.duel.NextAI != want {
			t.Fatalf("after AI turn at %.2fs NextAI = %.2f, want %.2f", g.duel.Elapsed, g.duel.NextAI, want)
		}
	}
	if moves == 0 || safeMoves == 0 {
		t.Fatalf("AI pace: moves=%d safeMoves=%d, want at least one 1.0s re-arm", moves, safeMoves)
	}
	// Deterministic mine hit: an AI reveal on a real mine re-arms NextAI at
	// the 2.4s stun horizon instead of the usual 1.0s interval.
	g.duel.Elapsed = 2
	g.duel.NextAI = g.duel.Elapsed + g.duel.AIInterval
	for i := range g.Cells {
		if c := &g.Cells[i]; c.IsMine && !c.Revealed && !c.Flagged {
			g.revealDuel(c.Row, c.Col, DuelAI)
			break
		}
	}
	if want := g.duel.Elapsed + 2.4; g.duel.NextAI != want {
		t.Fatalf("AI mine hit re-armed NextAI = %v, want %v (elapsed + 2.4 stun)", g.duel.NextAI, want)
	}
}

// TestSenzaihyoumeiSetlistSelectionAndFullGame verifies the shipped setlist
// lists 《潜在表明》 (500×500) before 《春日影》 (1000×1000) in the duel group,
// every shipped track (including the 6th track 《春日影》) is selectable by
// song name, and a player can pick 《潜在表明》 and play a full 3-minute duel
// through settlement and rematch without triggering the Haruhikage scene.
func TestSenzaihyoumeiSetlistSelectionAndFullGame(t *testing.T) {
	app := newMinesweeperApp(42)
	tt := ui.NewTester(app.view, 1280, 900)

	// Verify shipped popover duel group ordering and specs.
	if err := tt.Click("歌单"); err != nil {
		t.Fatalf("open setlist popover: %v", err)
	}
	duelHdr, ok := tt.Find("决斗")
	if !ok {
		t.Fatal("duel group header 「决斗」 missing from shipped setlist")
	}
	senzaiRect, ok := tt.Find("潜在表明")
	if !ok {
		t.Fatal("《潜在表明》 missing from shipped setlist")
	}
	haruRect, ok := tt.Find("春日影")
	if !ok {
		t.Fatal("《春日影》 missing from shipped setlist")
	}
	if senzaiRect.Y <= duelHdr.Y || haruRect.Y <= senzaiRect.Y {
		t.Fatalf("duel group order wrong: hdr=%+v senzai=%+v haru=%+v", duelHdr, senzaiRect, haruRect)
	}
	for _, wantSpec := range []string{"500×500 · 45,000 雷", "1000×1000 · 180,000 雷"} {
		if _, ok := tt.Find(wantSpec); !ok {
			t.Fatalf("shipped setlist missing duel spec %q", wantSpec)
		}
	}

	// Verify both duel tracks in the shipped duel group are selectable by song name.
	if err := tt.Click("春日影"); err != nil {
		t.Fatalf("select 《春日影》: %v", err)
	}
	if wantIdx := findDifficultyIdx("duel"); app.diffIdx != wantIdx || app.game.Difficulty.ID != "duel" {
		t.Fatalf("selected idx=%d id=%q, want idx=%d id=%q", app.diffIdx, app.game.Difficulty.ID, wantIdx, "duel")
	}

	// Switch to 《潜在表明》 via setlist and play a complete 3-minute duel.
	if err := tt.Click("歌单"); err != nil {
		t.Fatalf("open setlist for senzaihyoumei play: %v", err)
	}
	if err := tt.Click("潜在表明"); err != nil {
		t.Fatalf("select 《潜在表明》: %v", err)
	}
	if wantIdx := findDifficultyIdx("senzaihyoumei"); app.diffIdx != wantIdx || app.game.Difficulty.ID != "senzaihyoumei" {
		t.Fatalf("selected idx=%d id=%q, want idx=%d id=%q", app.diffIdx, app.game.Difficulty.ID, wantIdx, "senzaihyoumei")
	}
	// Pin board and AI RNGs after setlist reset so the end-to-end duel is deterministic.
	app.game.rng = rand.New(rand.NewPCG(42, 42^0x9e3779b97f4a7c15))
	app.game.duel.rng = rand.New(rand.NewPCG(7, 9))
	if !app.isDuel() || app.game.Rows != 500 || app.game.Cols != 500 || app.game.Mines != 45000 || app.game.SkillUsesLeft != 5 {
		t.Fatalf("senzaihyoumei setup wrong: duel=%v %dx%d mines=%d charges=%d",
			app.isDuel(), app.game.Rows, app.game.Cols, app.game.Mines, app.game.SkillUsesLeft)
	}
	if !tt.HasText("《潜在表明》") || !tt.HasText("剩余 03:00") || !tt.HasText("500×500 · 45000雷 · 03:00") {
		t.Fatal("senzaihyoumei duel HUD missing expected title, subtitle, or 03:00 timer")
	}

	boardRect, ok := tt.Find("决斗棋盘")
	if !ok {
		t.Fatal("duel board missing on 《潜在表明》")
	}
	centerRow, centerCol, ok := app.board.cellAt(app.game, boardRect.W/2, boardRect.H/2)
	if !ok || centerRow != 250 || centerCol != 250 {
		t.Fatalf("senzaihyoumei initial center = (%d,%d) ok=%v, want (250,250)", centerRow, centerCol, ok)
	}

	// First click opens the board and starts the duel.
	tt.ClickAt(boardRect.X+boardRect.W/2, boardRect.Y+boardRect.H/2)
	if app.game.Status != StatusPlaying || !app.game.FirstClick || app.game.duel.PlayerScore <= 0 {
		t.Fatalf("first click did not start 《潜在表明》: status=%v score=%d", app.game.Status, app.game.duel.PlayerScore)
	}

	// Place a flag via right-click and use one skill charge.
	var flagRow, flagCol int
	foundUnrevealed := false
	for dr := -4; dr <= 4 && !foundUnrevealed; dr++ {
		for dc := -4; dc <= 4; dc++ {
			c := app.game.Cell(centerRow+dr, centerCol+dc)
			if c != nil && !c.Revealed && !c.Flagged {
				flagRow, flagCol, foundUnrevealed = c.Row, c.Col, true
				break
			}
		}
	}
	if !foundUnrevealed {
		t.Fatal("no unrevealed cell near center for flag test")
	}
	fx := boardRect.X + (float32(flagCol)+0.5)*app.board.Cell - app.board.X
	fy := boardRect.Y + (float32(flagRow)+0.5)*app.board.Cell - app.board.Y
	tt.RightClickAt(fx, fy)
	if app.game.FlagsPlaced != 1 {
		t.Fatalf("flagsPlaced = %d, want 1", app.game.FlagsPlaced)
	}
	tt.Key(0, ui.KeyE)
	if app.game.SkillUsesLeft != 4 {
		t.Fatalf("skillUsesLeft = %d, want 4 after KeyE", app.game.SkillUsesLeft)
	}

	// Let Tomori AI take turns under the virtual clock, then run out the 3-minute clock.
	tt.Advance(2500 * time.Millisecond)
	if app.game.duel.LastAI < 0 {
		t.Fatal("Tomori AI did not take a turn during 《潜在表明》")
	}
	// Guarantee a score-loss finish to also verify 《潜在表明》 never triggers Haruhikage drama.
	app.game.duel.AIScore = app.game.duel.PlayerScore + 500
	app.game.duel.NextAI = 999
	app.game.duel.Elapsed = 179.5
	tt.Advance(600 * time.Millisecond)

	if !app.duelEnded || app.game.Status != StatusWon || !app.game.duel.Settled || app.game.duel.remaining() != 0 {
		t.Fatalf("《潜在表明》 did not settle at 03:00: ended=%v status=%v settled=%v rem=%v",
			app.duelEnded, app.game.Status, app.game.duel.Settled, app.game.duel.remaining())
	}
	if !tt.HasText("决斗结束 · 再来一局 (R)") || !tt.HasText("惜败 · 这局别发 SNS") || tt.HasText("剩余 00:00") {
		t.Fatal("settled 《潜在表明》 status bar should swap the timer for the end-of-duel verdict")
	}
	if app.haruhikageIntro || app.haruhikageAlert || app.unlockedMemes[MemeHaruhikage] {
		t.Fatalf("《潜在表明》 score loss must not trigger Haruhikage drama: intro=%v alert=%v", app.haruhikageIntro, app.haruhikageAlert)
	}
	settledChat := false
	for _, msg := range app.chatLog {
		if msg.Speaker == "决斗结算" {
			settledChat = true
			break
		}
	}
	if !settledChat {
		t.Fatal("missing 「决斗结算」 chat entry after 《潜在表明》 finished")
	}

	// Rematch resets cleanly back to an idle 500×500 《潜在表明》 duel.
	if err := tt.Click("再战一局"); err != nil {
		t.Fatalf("rematch click failed: %v", err)
	}
	if app.duelEnded || app.game.Status != StatusIdle || app.game.Difficulty.ID != "senzaihyoumei" || !tt.HasText("剩余 03:00") {
		t.Fatalf("rematch did not reset 《潜在表明》 cleanly: ended=%v status=%v id=%q",
			app.duelEnded, app.game.Status, app.game.Difficulty.ID)
	}
}
