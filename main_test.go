package main

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestFirstClickSafetyAndFloodFill(t *testing.T) {
	g := NewGame(FindDifficulty(DiffEasy), false, MemberTomori, 42)
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
		t.Fatalf("expected flood fill of >= 9 cells, got %d", res.RevealedCount)
	}
}

func TestWinAutoFlagsRemainingMines(t *testing.T) {
	g := NewGame(Difficulty{Rows: 5, Cols: 5, Mines: 2}, false, MemberTomori, 1)
	g.FirstClick = true
	g.Status = StatusPlaying
	g.Cell(0, 0).IsMine = true
	g.Cell(4, 4).IsMine = true
	g.RecomputeAdjacency()

	for i := range g.Cells {
		c := &g.Cells[i]
		if !c.IsMine && !c.Revealed {
			g.Reveal(c.Row, c.Col)
		}
	}

	if g.Status != StatusWon {
		t.Fatalf("expected StatusWon, got %v", g.Status)
	}
	if !g.Cell(0, 0).Flagged || !g.Cell(4, 4).Flagged {
		t.Fatalf("expected remaining mines to be auto-flagged on win")
	}
	if g.RemainingMines() != 0 {
		t.Fatalf("expected 0 remaining mines, got %d", g.RemainingMines())
	}
}

func TestChordRevealsSafeNeighbors(t *testing.T) {
	g := NewGame(Difficulty{Rows: 5, Cols: 5, Mines: 1}, false, MemberTomori, 1)
	g.FirstClick = true
	g.Status = StatusPlaying
	g.Cell(0, 0).IsMine = true
	g.RecomputeAdjacency()

	g.Reveal(1, 1)
	g.ToggleFlag(0, 0, false)
	res := g.Chord(1, 1)
	if res.Kind != ActionRevealed && res.Kind != ActionWon {
		t.Fatalf("expected reveal/win on chord, got %s", res.Kind)
	}
	if !g.Cell(0, 1).Revealed || !g.Cell(1, 0).Revealed {
		t.Fatalf("expected safe neighbors around (1,1) to be revealed")
	}
}

func TestSoyorinShieldAndActiveSkills(t *testing.T) {
	g := NewGame(Difficulty{Rows: 5, Cols: 5, Mines: 2, SkillCharges: 1}, true, MemberSoyorin, 7)
	g.FirstClick = true
	g.Status = StatusPlaying
	g.Cell(2, 2).IsMine = true
	g.Cell(0, 0).IsMine = true
	g.RecomputeAdjacency()

	res := g.Reveal(2, 2)
	if res.Kind != ActionShielded {
		t.Fatalf("expected ActionShielded, got %s", res.Kind)
	}
	if g.Status != StatusPlaying || g.SkillUsesLeft != 0 || !g.Cell(2, 2).ShieldSaved {
		t.Fatalf("unexpected state after Soyorin shield: status=%v uses=%d saved=%v", g.Status, g.SkillUsesLeft, g.Cell(2, 2).ShieldSaved)
	}

	res2 := g.Reveal(0, 0)
	if res2.Kind != ActionLost || g.Status != StatusLost {
		t.Fatalf("expected loss on second mine hit, got %s", res2.Kind)
	}

	// Taki auto-flags a mine, Tomori reveals a safe cell
	g2 := NewGame(FindDifficulty(DiffEasy), true, MemberTaki, 99)
	g2.Reveal(4, 4)
	takiRes := g2.UseSkill()
	if takiRes.Kind != ActionSkill || g2.FlagsPlaced != 1 {
		t.Fatalf("expected Taki skill to flag 1 mine, got %+v flags=%d", takiRes, g2.FlagsPlaced)
	}

	g2.MemberID = MemberTomori
	g2.SkillUsesLeft = 1
	before := g2.RevealedSafe
	tomoriRes := g2.UseSkill()
	if tomoriRes.Kind != ActionSkill && tomoriRes.Kind != ActionWon {
		t.Fatalf("expected Tomori skill to reveal safe cell, got %+v", tomoriRes)
	}
	if g2.RevealedSafe <= before {
		t.Fatalf("expected revealed safe count to increase")
	}
}

func TestNativeUIViewAndInteractions(t *testing.T) {
	app := newMinesweeperApp(12345)
	tt := ui.NewTester(app.view, 960, 780)

	if !tt.HasText("MyGO!!!!!") || !tt.HasText("010") {
		t.Fatalf("expected header and 010 mine counter in initial view")
	}

	// Switch companion to 椎名立希
	if err := tt.Click("椎名立希"); err != nil {
		t.Fatalf("failed clicking member 椎名立希: %v", err)
	}
	if app.memberID != MemberTaki {
		t.Fatalf("expected memberID taki, got %s", app.memberID)
	}

	// Right-click a cell to flag it
	if err := tt.RightClick("cell-0-0"); err != nil {
		t.Fatalf("failed right-clicking cell-0-0: %v", err)
	}
	if app.game.FlagsPlaced != 1 || !tt.HasText("009") {
		t.Fatalf("expected 1 flag and 009 mines left, got flags=%d", app.game.FlagsPlaced)
	}

	// Left-click center cell to start the game
	if err := tt.Click("cell-4-4"); err != nil {
		t.Fatalf("failed clicking cell-4-4: %v", err)
	}
	if app.game.Status != StatusPlaying {
		t.Fatalf("expected StatusPlaying after clicking cell-4-4, got %v", app.game.Status)
	}

	// Advance clock and verify live timer updates
	tt.Advance(2 * time.Second)
	if app.elapsed < 2 || !tt.HasText("002") {
		t.Fatalf("expected timer >= 2s and '002' in view, got elapsed=%d", app.elapsed)
	}

	// Keyboard shortcut F toggles flag mode, E triggers skill, R resets
	tt.Key(0, ui.KeyF)
	if !app.flagMode {
		t.Fatalf("expected flagMode true after pressing F")
	}
	tt.Key(0, ui.KeyE)
	if app.game.SkillUsesLeft != 0 {
		t.Fatalf("expected skill to be consumed after pressing E, usesLeft=%d", app.game.SkillUsesLeft)
	}
	tt.Key(0, ui.KeyR)
	if app.game.Status != StatusIdle || app.elapsed != 0 {
		t.Fatalf("expected reset after pressing R, status=%v elapsed=%d", app.game.Status, app.elapsed)
	}
}
