package main

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestDuelBoardIsFullMillionAndFirstClickSafe(t *testing.T) {
	g := NewGame(Difficulties[4], true, MemberAnon, 42)
	if g.Rows != 1000 || g.Cols != 1000 || len(g.Cells) != 1000000 {
		t.Fatalf("duel dimensions: %dx%d cells=%d", g.Rows, g.Cols, len(g.Cells))
	}
	result := g.Reveal(500, 500)
	if result.Kind != ActionRevealed || g.Status != StatusPlaying {
		t.Fatalf("duel did not start: %+v", result)
	}
	if g.Cell(500, 500).IsMine || g.Cell(500, 500).Owner != DuelPlayer {
		t.Fatal("first cell must be safe and player owned")
	}
	mines := 0
	for i := range g.Cells {
		if g.Cells[i].IsMine {
			mines++
		}
	}
	if mines != 180000 {
		t.Fatalf("mines=%d", mines)
	}
}

func TestDuelMinePenalizesAndStunsInsteadOfEnding(t *testing.T) {
	g := NewGame(Difficulties[4], true, MemberAnon, 42)
	g.Reveal(500, 500)
	mine := -1
	for i := range g.Cells {
		if g.Cells[i].IsMine {
			mine = i
			break
		}
	}
	before := g.duel.PlayerScore
	result := g.revealDuel(mine/g.Cols, mine%g.Cols, DuelPlayer)
	if result.Kind != ActionLost || g.Status != StatusPlaying || !g.duel.playerStunned() {
		t.Fatalf("mine result=%+v status=%v", result, g.Status)
	}
	if g.duel.PlayerScore != before-100 {
		t.Fatalf("score=%d before=%d", g.duel.PlayerScore, before)
	}
	if g.Reveal(500, 501).Kind != ActionNoop {
		t.Fatal("stun should reject input")
	}
}

func TestDuelFlagHidesAnswerAndUnflag(t *testing.T) {
	g := NewGame(Difficulties[4], true, MemberTaki, 42)
	g.Reveal(500, 500)
	index := -1
	for i := range g.Cells {
		if !g.Cells[i].IsMine && !g.Cells[i].Revealed {
			index = i
			break
		}
	}
	row, col := index/g.Cols, index%g.Cols
	before := g.duel.PlayerScore
	result := g.ToggleFlag(row, col, false)
	if result.Kind != ActionFlagged || g.duel.PlayerScore != before {
		t.Fatalf("wrong flag=%+v score=%d", result, g.duel.PlayerScore)
	}
	if g.UnflagDuel(row, col).Kind != ActionUnflag || g.duel.PlayerScore != before {
		t.Fatalf("unflag did not restore score=%d", g.duel.PlayerScore)
	}
}

func TestDuelTimerAndBoardNavigation(t *testing.T) {
	g := NewGame(Difficulties[4], true, MemberAnon, 42)
	g.Reveal(500, 500)
	tick := g.TickDuel(300)
	if !tick.Finished || g.Status != StatusWon || g.duel.remaining() != 0 {
		t.Fatalf("timer did not finish: %+v status=%v", tick, g.Status)
	}
	v := newBoardView()
	v.Width, v.Height = 600, 400
	v.zoom(g, 100, 100, 20)
	v.zoom(g, 100, 100, 0.001)
	v.clamp(g)
	if v.Cell != 10 || v.X < 0 || v.Y < 0 {
		t.Fatalf("invalid view %+v", v)
	}
	v.center(g, 999, 999)
	row, col, ok := v.cellAt(g, 599, 399)
	if !ok || row < 0 || col < 0 || row >= 1000 || col >= 1000 {
		t.Fatalf("cellAt boundary=%d,%d", row, col)
	}
}

func TestDuelNativeCanvasInput(t *testing.T) {
	app := newMinesweeperApp(42)
	app.diffIdx = 4
	app.resetGame(false)
	tt := ui.NewTester(app.view, 1280, 900)
	if !tt.HasText("百万格决斗") || !tt.HasText("灯 (AI) 0") {
		t.Fatal("duel HUD not present")
	}
	tt.Advance(100 * time.Millisecond)
	if app.game.duel == nil || app.board.Width < 100 {
		t.Fatal("duel canvas not laid out")
	}
	r, found := tt.Find("百万格决斗战场")
	if !found {
		t.Fatal("duel board not found")
	}
	x := r.X + r.W/2
	y := r.Y + r.H/2
	row, col, ok := app.board.cellAt(app.game, r.W/2, r.H/2)
	if !ok {
		t.Fatal("center cell not found")
	}
	tt.ClickAt(x, y)
	if app.game.Status != StatusPlaying || app.game.duel.PlayerScore <= 0 {
		t.Fatalf("board click did not reveal: status=%v score=%d", app.game.Status, app.game.duel.PlayerScore)
	}
	if app.game.Cell(row, col).Owner != DuelPlayer {
		t.Fatal("clicked cell did not map to player territory")
	}
	oldCell := app.board.Cell
	tt.Scroll(x, y, 0, -200)
	if app.board.Cell == oldCell {
		t.Fatal("wheel did not zoom")
	}
	tt.Advance(1200 * time.Millisecond)
	if app.game.duel.AIScore == 0 && app.game.duel.LastAI < 0 {
		t.Fatal("AI did not play")
	}
}
