package main

import (
	"github.com/egoist/mygo/ui"
	"math"
	"math/rand/v2"
	"runtime"
	"testing"
	"time"
	"unsafe"
)

func qaApp(t *testing.T) (*minesweeperApp, *ui.Tester, ui.Rect) {
	t.Helper()
	a := newMinesweeperApp(42)
	a.diffIdx = findDifficultyIdx("duel")
	a.resetGame(false)
	tt := ui.NewTester(a.view, 1280, 900)
	r, ok := tt.Find("决斗棋盘")
	if !ok {
		t.Fatal("board missing")
	}
	return a, tt, r
}

func TestQAFirstFlag(t *testing.T) {
	g := NewGame(Difficulties[findDifficultyIdx("duel")], true, MemberAnon, 42)
	g.ToggleFlag(500, 500, false)
	if !g.Cell(500, 500).IsMine && g.duel.PlayerScore == -30 && g.FirstClick {
		t.Fatalf("first flag forced safe and penalized: score=%d status=%v", g.duel.PlayerScore, g.Status)
	}
}

func TestQAFrozenSettlement(t *testing.T) {
	g := NewGame(Difficulties[findDifficultyIdx("duel")], true, MemberAnon, 42)
	g.ToggleFlag(500, 500, false)
	g.Reveal(500, 501)
	g.TickDuel(300)
	before := g.duel.PlayerScore
	res := g.UnflagDuel(500, 500)
	if res.Kind != ActionNoop || g.duel.PlayerScore != before {
		t.Fatalf("settled score changed %d -> %d, result=%s", before, g.duel.PlayerScore, res.Kind)
	}
}

func TestQAStunnedUnflag(t *testing.T) {
	g := NewGame(Difficulties[findDifficultyIdx("duel")], true, MemberAnon, 42)
	g.ToggleFlag(500, 500, false)
	g.duel.StunnedUntil = 2.4
	if res := g.UnflagDuel(500, 500); res.Kind != ActionNoop {
		t.Fatalf("unflag accepted while stunned: %s", res.Kind)
	}
}

func TestQAMinimapCenter(t *testing.T) {
	a, tt, r := qaApp(t)
	a.board.center(a.game, 480, 480)
	tt.Frame()
	x0, y0 := a.board.X, a.board.Y
	tt.ClickAt(r.X+r.W-84, r.Y+r.H-84)
	if a.game.FirstClick || a.game.Moves != 0 {
		t.Fatal("minimap center click leaked reveal")
	}
	if !a.board.gliding() {
		t.Fatal("minimap center click did not start camera glide")
	}
	tt.Advance(60 * time.Millisecond)
	if !a.board.gliding() || (a.board.X == x0 && a.board.Y == y0) {
		t.Fatalf("minimap center mid-glide not in transition: start=(%v,%v) mid=(%v,%v) gliding=%v",
			x0, y0, a.board.X, a.board.Y, a.board.gliding())
	}
	tt.Advance(200 * time.Millisecond)
	if a.board.gliding() {
		t.Fatal("minimap center glide did not settle after 260ms")
	}
	row, col, _ := a.board.cellAt(a.game, r.W/2, r.H/2)
	if row < 490 || row > 510 || col < 490 || col > 510 {
		t.Fatalf("minimap center navigated to (%d,%d); offset=(%v,%v)", row, col, a.board.X, a.board.Y)
	}
}

func TestQAInputNavigationReset(t *testing.T) {
	a, tt, r := qaApp(t)
	x, y := r.X+100, r.Y+100
	row, col, _ := a.board.cellAt(a.game, 100, 100)
	tt.ClickAt(x, y)
	if !a.game.FirstClick || !a.game.Cell(row, col).Revealed {
		t.Fatalf("click not routed: board=%+v target=%d,%d", r, row, col)
	}
	beforeX, beforeY := a.board.X, a.board.Y
	moves := a.game.Moves
	tt.PressButton(2, x, y)
	tt.Move(x+40, y+30)
	tt.ReleaseButton(2, x+40, y+30)
	if a.board.X != beforeX-40 || a.board.Y != beforeY-30 || a.game.Moves != moves {
		t.Fatalf("drag mismatch: offsets %v,%v moves %d", a.board.X, a.board.Y, a.game.Moves)
	}
	wx, wy := (a.board.X+100)/a.board.Cell, (a.board.Y+100)/a.board.Cell
	tt.Pinch(x, y, 1.2)
	if math.Abs(float64((a.board.X+100)/a.board.Cell-wx)) > 0.001 || math.Abs(float64((a.board.Y+100)/a.board.Cell-wy)) > 0.001 {
		t.Fatal("zoom anchor moved")
	}
	tt.Advance(2100 * time.Millisecond)
	if a.game.duel.Elapsed < 2 {
		t.Fatalf("clock=%f", a.game.duel.Elapsed)
	}
	tt.Key(0, ui.KeyR)
	if a.game.Status != StatusIdle || a.game.duel.Elapsed != 0 || a.game.duel.PlayerScore != 0 || a.game.duel.AIScore != 0 || a.game.FlagsPlaced != 0 || a.board.Pressed {
		t.Fatal("reset leaked state")
	}
}

func TestQAFrontierFlood(t *testing.T) {
	g := NewGame(Difficulties[findDifficultyIdx("duel")], true, MemberAnon, 42)
	g.Reveal(500, 500)
	actual, missing := 0, 0
	for i := range g.Cells {
		c := &g.Cells[i]
		if c.Revealed || c.Flagged {
			continue
		}
		for _, n := range g.Neighbors(c.Row, c.Col) {
			if n.Revealed && !n.IsMine {
				actual++
				if _, ok := g.duel.frontier[i]; !ok {
					missing++
				}
				break
			}
		}
	}
	if missing > 0 {
		t.Fatalf("flood frontier actual=%d missing=%d stored=%d", actual, missing, len(g.duel.frontier))
	}
}

func TestQAAIPublicOnlyAndTurns(t *testing.T) {
	g := NewGame(Difficulties[findDifficultyIdx("duel")], true, MemberAnon, 42)
	g.Reveal(500, 500)
	g.duel.frontier = map[int]bool{}
	g.duel.rng = rand.New(rand.NewPCG(7, 9))
	i, ok := g.chooseAIAction()
	for n := range g.Cells {
		if !g.Cells[n].Revealed {
			g.Cells[n].IsMine = !g.Cells[n].IsMine
			g.Cells[n].Adjacent = 8
		}
	}
	g.duel.rng = rand.New(rand.NewPCG(7, 9))
	j, ok2 := g.chooseAIAction()
	if i != j || ok != ok2 {
		t.Fatal("hidden mine data affected random AI decision")
	}
	g.duel.NextAI = 1.1
	if g.TickDuel(1).AITurn != nil {
		t.Fatal("AI acted early")
	}
	if g.TickDuel(.11).AITurn == nil {
		t.Fatal("AI missed boundary")
	}
	if g.TickDuel(.01).AITurn != nil {
		t.Fatal("AI acted twice")
	}
	g.TickDuel(300)
	before := g.Moves
	if g.TickDuel(1).AITurn != nil || g.Moves != before {
		t.Fatal("AI acted after finish")
	}
}

func TestQAEarlySettlement(t *testing.T) {
	a, tt, _ := qaApp(t)
	g := a.game
	g.FirstClick = true
	g.Status = StatusPlaying
	g.RevealedSafe = g.Rows*g.Cols - g.Mines - 1
	g.Cell(0, 0).Adjacent = 1
	a.pendingResult = g.Reveal(0, 0)
	tt.Frame()
	found := false
	for _, m := range a.chatLog {
		if m.Speaker == "决斗结算" {
			found = true
		}
	}
	if !found {
		t.Fatalf("board completion status=%v but no settlement", g.Status)
	}
}

func TestQAMemory(t *testing.T) {
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	start := time.Now()
	g := NewGame(Difficulties[findDifficultyIdx("duel")], true, MemberAnon, 42)
	construct := time.Since(start)
	start = time.Now()
	g.Reveal(500, 500)
	first := time.Since(start)
	runtime.ReadMemStats(&after)
	t.Logf("cell=%d bytes cells=%.2f MiB allocated=%.2f MiB construct=%s first reveal=%s", unsafe.Sizeof(Cell{}), float64(len(g.Cells))*float64(unsafe.Sizeof(Cell{}))/(1<<20), float64(after.TotalAlloc-before.TotalAlloc)/(1<<20), construct, first)
	runtime.KeepAlive(g)
}

func TestQAAIKnownExplodedMine(t *testing.T) {
	g := NewGame(Difficulty{Rows: 5, Cols: 5, Mines: 1}, false, MemberAnon, 42)
	g.duel = newDuel(g)
	g.Status = StatusPlaying
	g.FirstClick = true
	for i := range g.Cells {
		g.Cells[i].Revealed = true
	}
	g.Cell(0, 0).IsMine = true
	g.Cell(0, 0).Exploded = true
	g.RecomputeAdjacency()
	target := g.Cell(2, 2)
	target.Revealed = false
	g.duel.frontier[12] = true
	i, ok := g.chooseAIAction()
	if !ok || i != 12 || target.AITurn {
		t.Fatalf("AI flags provably safe cell beside already exploded mine: index=%d flag=%v", i, target.AITurn)
	}
}

func TestQAScoreAccounting(t *testing.T) {
	g := NewGame(Difficulties[findDifficultyIdx("duel")], true, MemberAnon, 42)
	res := g.Reveal(500, 500)
	if g.duel.PlayerScore != res.RevealedCount {
		t.Fatal("reveal score mismatch")
	}
	for i := range g.Cells {
		c := &g.Cells[i]
		if c.Revealed {
			continue
		}
		before := g.duel.PlayerScore
		g.ToggleFlag(c.Row, c.Col, false)
		if g.duel.PlayerScore != before {
			t.Fatal("flag score mismatch")
		}
		g.UnflagDuel(c.Row, c.Col)
		if g.duel.PlayerScore != before {
			t.Fatal("unflag accounting mismatch")
		}
		if i > 100 {
			break
		}
	}
}

func TestQALosingHUD(t *testing.T) {
	a, tt, _ := qaApp(t)
	a.game.Status = StatusPlaying
	a.game.FirstClick = true
	a.game.duel.AIScore = 100
	a.game.TickDuel(300)
	tt.Frame()
	if tt.HasText("Live 大成功！再来一首") {
		t.Fatal("losing duel displays success label")
	}
}

func BenchmarkQAIdleFrame(b *testing.B) {
	a := newMinesweeperApp(42)
	a.diffIdx = findDifficultyIdx("duel")
	a.resetGame(false)
	tt := ui.NewTester(a.view, 1280, 900)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		tt.Frame()
	}
}

func BenchmarkQARevealedFrame(b *testing.B) {
	a := newMinesweeperApp(42)
	a.diffIdx = findDifficultyIdx("duel")
	a.resetGame(false)
	a.game.Reveal(500, 500)
	tt := ui.NewTester(a.view, 1280, 900)
	a.board.EffectCell = -1
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		tt.Frame()
	}
}

func BenchmarkQAPanFrame(b *testing.B) {
	a := newMinesweeperApp(42)
	a.diffIdx = findDifficultyIdx("duel")
	a.resetGame(false)
	a.game.Reveal(500, 500)
	tt := ui.NewTester(a.view, 1280, 900)
	a.board.EffectCell = -1
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		a.board.X += 1
		if a.board.X > 14000 {
			a.board.X = 12000
		}
		tt.Frame()
	}
}

func TestQAFlagProbeEvidence(t *testing.T) {
	g := NewGame(Difficulties[findDifficultyIdx("duel")], true, MemberAnon, 42)
	g.Reveal(500, 500)
	inferred := 0
	for i := range g.Cells {
		c := &g.Cells[i]
		if c.Revealed {
			continue
		}
		score := g.duel.PlayerScore
		g.ToggleFlag(c.Row, c.Col, false)
		g.UnflagDuel(c.Row, c.Col)
		if g.duel.PlayerScore != score {
			t.Fatal("probe unexpected")
		}
		inferred++
		if inferred == 100 {
			break
		}
	}
	t.Logf("verified %d flags do not change score", inferred)
}

func TestQASettlementAwardsFlagAccuracy(t *testing.T) {
	g := NewGame(Difficulties[findDifficultyIdx("duel")], true, MemberAnon, 42)
	g.Reveal(500, 500)
	before := g.duel.PlayerScore
	safe := -1
	mine := -1
	for i := range g.Cells {
		if g.Cells[i].Revealed {
			continue
		}
		if g.Cells[i].IsMine && mine < 0 {
			mine = i
		} else if !g.Cells[i].IsMine && safe < 0 {
			safe = i
		}
	}
	g.ToggleFlag(safe/g.Cols, safe%g.Cols, false)
	g.ToggleFlag(mine/g.Cols, mine%g.Cols, false)
	g.finishDuel()
	want := before + 45
	before = g.duel.PlayerScore
	g.finishDuel()
	if g.duel.PlayerScore != before {
		t.Fatal("settlement is not idempotent")
	}
	if g.duel.PlayerScore != want {
		t.Fatalf("settlement flag score=%d want=%d", g.duel.PlayerScore, want)
	}
}

func TestQADuelSkillAndIdleFlagEdges(t *testing.T) {
	app := newMinesweeperApp(42)
	app.modeIdx = 1
	app.diffIdx = findDifficultyIdx("duel")
	app.resetGame(false)
	if !app.game.BandMode || app.game.SkillUsesLeft == 0 {
		t.Fatal("duel inherited classic disabled skills")
	}
	g := app.game
	g.ToggleFlag(500, 500, false)
	if g.UnflagDuel(500, 500).Kind != ActionUnflag {
		t.Fatal("idle flag cannot be removed")
	}
	g.Reveal(500, 500)
	g.duel.StunnedUntil = g.duel.Elapsed + 2.4
	if g.UseSkill().Kind != ActionSkill || g.duel.playerStunned() {
		t.Fatal("Tomori skill did not clear stun")
	}
}

func TestQADuelMineAndClaimKnownMines(t *testing.T) {
	g := NewGame(Difficulty{Gameplay: GameplayDuel, Rows: 5, Cols: 5, Mines: 2}, true, MemberAnon, 42)
	g.FirstClick = true
	g.Status = StatusPlaying
	g.Cell(0, 0).Revealed = true
	g.Cell(0, 0).Adjacent = 2
	g.Cell(1, 1).IsMine = true
	g.Cell(1, 1).Exploded = true
	g.Cell(1, 1).Revealed = true
	if g.ClaimDuelMines(0, 0).Kind != ActionNoop {
		t.Fatal("claim reads a hidden answer in a two-way guess")
	}
	g.duel.NextAI = 2
	g.duel.Elapsed = 1
	g.Cell(1, 1).Revealed = false
	g.revealDuel(1, 1, DuelAI)
	if g.duel.NextAI != 3.4 {
		t.Fatalf("AI mine has no stun: next=%v", g.duel.NextAI)
	}
}
