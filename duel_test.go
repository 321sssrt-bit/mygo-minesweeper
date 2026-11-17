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

func TestDuelMinimapTerritoryAggregationAndPersistence(t *testing.T) {
	g := NewGame(Difficulties[4], true, MemberSoyorin, 42)
	pTot, aiTot := g.duel.MinimapTotals()
	if pTot != 0 || aiTot != 0 || g.duel.LastCell != -1 || g.duel.LastAI != -1 {
		t.Fatalf("expected clean initial minimap, got p=%d ai=%d last=(%d,%d)", pTot, aiTot, g.duel.LastCell, g.duel.LastAI)
	}

	res := g.Reveal(500, 500)
	pTot, aiTot = g.duel.MinimapTotals()
	if pTot != res.RevealedCount || pTot <= 0 || aiTot != 0 {
		t.Fatalf("expected player minimap total %d, got p=%d ai=%d", res.RevealedCount, pTot, aiTot)
	}
	earlyBlock := g.duel.Minimap[g.duel.minimapBlockIndex(g, 500, 500)]
	if earlyBlock.Player == 0 {
		t.Fatal("expected early territory block (50,50) to have player territory")
	}

	// Simulate >1200 AI reveals across distant blocks and verify early territory is never truncated.
	aiAdded := 0
	for r := 10; r < 150 && aiAdded < 1205; r++ {
		for c := 10; c < 40 && aiAdded < 1205; c++ {
			cell := g.Cell(r, c)
			if !cell.IsMine && !cell.Revealed && !cell.Flagged {
				prev := g.duel.AIScore
				ar := g.revealDuel(r, c, DuelAI)
				if ar.Kind == ActionRevealed {
					aiAdded += g.duel.AIScore - prev
				}
			}
		}
	}
	if aiAdded <= 1000 {
		t.Fatalf("expected >1000 AI cells for persistence test, got %d", aiAdded)
	}
	_, aiTot = g.duel.MinimapTotals()
	if aiTot != aiAdded {
		t.Fatalf("expected persistent AI minimap total %d without truncation, got %d", aiAdded, aiTot)
	}
	if g.duel.Minimap[g.duel.minimapBlockIndex(g, 500, 500)].Player == 0 {
		t.Fatal("early player block was lost after >1000 AI actions")
	}

	// Flag and unflag updates territory and LastCell.
	var flagCell *Cell
	for i := range g.Cells {
		c := &g.Cells[i]
		if !c.Revealed && !c.Flagged && !c.IsMine {
			flagCell = c
			break
		}
	}
	beforeFlagP, _ := g.duel.MinimapTotals()
	if fRes := g.ToggleFlag(flagCell.Row, flagCell.Col, false); fRes.Kind != ActionFlagged {
		t.Fatalf("flag failed: %+v", fRes)
	}
	afterFlagP, _ := g.duel.MinimapTotals()
	if afterFlagP != beforeFlagP+1 || g.duel.LastCell != flagCell.Row*g.Cols+flagCell.Col {
		t.Fatalf("flag territory/marker mismatch: before=%d after=%d lastCell=%d", beforeFlagP, afterFlagP, g.duel.LastCell)
	}
	if uRes := g.UnflagDuel(flagCell.Row, flagCell.Col); uRes.Kind != ActionUnflag {
		t.Fatalf("unflag failed: %+v", uRes)
	}
	afterUnflagP, _ := g.duel.MinimapTotals()
	if afterUnflagP != beforeFlagP || g.duel.LastCell != flagCell.Row*g.Cols+flagCell.Col {
		t.Fatalf("unflag did not reverse territory or update LastCell: before=%d after=%d", beforeFlagP, afterUnflagP)
	}

	// Soyorin shield counts once (never double-counted).
	var mineCell *Cell
	for i := range g.Cells {
		c := &g.Cells[i]
		if c.IsMine && !c.Revealed && !c.Flagged {
			mineCell = c
			break
		}
	}
	beforeShieldP, _ := g.duel.MinimapTotals()
	if sRes := g.Reveal(mineCell.Row, mineCell.Col); sRes.Kind != ActionShielded {
		t.Fatalf("expected Soyorin shield, got %+v", sRes)
	}
	afterShieldP, _ := g.duel.MinimapTotals()
	if afterShieldP != beforeShieldP+1 {
		t.Fatalf("Soyorin shield should count once in minimap: before=%d after=%d", beforeShieldP, afterShieldP)
	}
}

func TestDuelMinimapLiveMarkersAndReset(t *testing.T) {
	app := newMinesweeperApp(42)
	app.diffIdx = 4
	app.resetGame(false)
	tt := ui.NewTester(app.view, 1280, 900)

	if app.game.duel.LastCell != -1 || app.game.duel.LastAI != -1 {
		t.Fatalf("expected no initial markers, got player=%d ai=%d", app.game.duel.LastCell, app.game.duel.LastAI)
	}

	// Edge clamping bounds check.
	mini := ui.Rect{X: 100, Y: 100, W: 148, H: 148}
	for _, idx := range []int{0, 999, 999 * 1000, 999*1000 + 999} {
		mx, my := minimapMarkerPos(mini, 1000, 1000, idx, 5)
		if mx < mini.X+5 || mx > mini.X+mini.W-5 || my < mini.Y+5 || my > mini.Y+mini.H-5 {
			t.Fatalf("marker out of minimap bounds for idx=%d: (%v,%v)", idx, mx, my)
		}
		ring := clampMinimapRect(mini, mx-10, my-10, 20)
		if ring.X < mini.X+1 || ring.Y < mini.Y+1 || ring.X+ring.W > mini.X+mini.W-1 || ring.Y+ring.H > mini.Y+mini.H-1 {
			t.Fatalf("pulse ring escaped minimap bounds: %+v", ring)
		}
	}

	r, ok := tt.Find("百万格决斗战场")
	if !ok {
		t.Fatal("duel board missing")
	}
	tt.ClickAt(r.X+r.W/2, r.Y+r.H/2)
	if app.game.duel.LastCell < 0 {
		t.Fatal("expected player marker after click")
	}

	// Force overlap of player and AI markers and advance virtual clock.
	app.game.duel.LastAI = app.game.duel.LastCell
	tt.Advance(350 * time.Millisecond)
	tt.Advance(450 * time.Millisecond)

	// Match end + reset cleanup.
	app.game.TickDuel(300)
	tt.Frame()
	if app.game.Status != StatusWon {
		t.Fatalf("expected StatusWon after 300s, got %v", app.game.Status)
	}
	tt.Key(0, ui.KeyR)
	pTot, aiTot := app.game.duel.MinimapTotals()
	if pTot != 0 || aiTot != 0 || app.game.duel.LastCell != -1 || app.game.duel.LastAI != -1 {
		t.Fatalf("expected full minimap reset, got p=%d ai=%d last=(%d,%d)", pTot, aiTot, app.game.duel.LastCell, app.game.duel.LastAI)
	}
}

