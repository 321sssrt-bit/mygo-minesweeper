package main

import (
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

	if !tt.HasText("MyGO!!!!! 迷子扫雷") || !tt.HasText("MINES 010") {
		t.Fatalf("expected title and MINES 010 badge in initial view")
	}

	// Segmented control switches difficulty to RiNG 排练室
	if err := tt.Click("RiNG 排练室"); err != nil {
		t.Fatalf("failed clicking segmented difficulty: %v", err)
	}
	if app.diffIdx != 1 || app.game.Cols != 16 || !tt.HasText("MINES 040") {
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
	if app.game.FlagsPlaced != 1 || !tt.HasText("MINES 009") {
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
	if app.elapsed < 2 || !tt.HasText("TIME 002s") {
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
