package main

import (
	"math"
	"math/rand/v2"
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

	// Edge clamping and overlap separation bounds check.
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
		px, py := separateMinimapMarker(mini, mx, my, mx, my, 5)
		if abs32(px-mx) < 6 && abs32(py-my) < 6 {
			t.Fatalf("overlapping corner markers not separated at idx=%d: (%v,%v) vs (%v,%v)", idx, px, py, mx, my)
		}
		if px < mini.X+5 || px > mini.X+mini.W-5 || py < mini.Y+5 || py > mini.Y+mini.H-5 {
			t.Fatalf("separated marker out of bounds: (%v,%v)", px, py)
		}
	}
	// Nearby case flagged in review: AI at (500,500), player at (520,480) must maintain >=6px separation.
	ax, ay := minimapMarkerPos(mini, 1000, 1000, 500*1000+500, 5)
	px, py := minimapMarkerPos(mini, 1000, 1000, 520*1000+480, 5)
	px, py = separateMinimapMarker(mini, px, py, ax, ay, 5)
	if abs32(px-ax) < 6 && abs32(py-ay) < 6 {
		t.Fatalf("adjacent markers collapsed together: player=(%v,%v) ai=(%v,%v)", px, py, ax, ay)
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

func TestDuelFixtureBoundsRespectsConfiguredMines(t *testing.T) {
	g := NewGame(Difficulty{Gameplay: GameplayDuel, Rows: 6, Cols: 6, Mines: 4}, true, MemberAnon, 42)
	if g.Rows != 6 || g.Cols != 6 || g.Mines != 4 {
		t.Fatalf("expected 6x6 with 4 mines, got %dx%d mines=%d", g.Rows, g.Cols, g.Mines)
	}
	res := g.Reveal(3, 3)
	if res.Kind != ActionRevealed {
		t.Fatalf("expected ActionRevealed, got %+v", res)
	}
	mines := 0
	for i := range g.Cells {
		if g.Cells[i].IsMine {
			mines++
		}
	}
	if mines != 4 {
		t.Fatalf("expected 4 placed mines, got %d", mines)
	}
}

func TestNewGameBuildsByGameplayNotTrackID(t *testing.T) {
	duel := NewGame(Difficulty{ID: "encore", Gameplay: GameplayDuel, Rows: 50, Cols: 50, Mines: 2}, true, MemberAnon, 42)
	if duel.duel == nil {
		t.Fatal("gameplay duel must build duel rules regardless of track ID")
	}
	if duel.Rows != 50 || duel.Cols != 50 {
		t.Fatalf("duel gameplay keeps the million-cell clamp: %dx%d", duel.Rows, duel.Cols)
	}
	classic := NewGame(Difficulty{ID: "duel", Rows: 50, Cols: 50, Mines: 2}, true, MemberAnon, 42)
	if classic.duel != nil {
		t.Fatal("track ID alone must not trigger duel rules")
	}
	if classic.Rows != 30 || classic.Cols != 40 {
		t.Fatalf("classic gameplay keeps the 30x40 clamp: %dx%d", classic.Rows, classic.Cols)
	}
}

func TestDuelDurationAndAIPaceComeFromTrack(t *testing.T) {
	std := NewGame(Difficulties[4], true, MemberAnon, 42)
	if std.duel.Duration != 300 || std.duel.AIInterval != 0.72 {
		t.Fatalf("standard duel keeps 5min/0.72s: duration=%v interval=%v", std.duel.Duration, std.duel.AIInterval)
	}
	bare := NewGame(Difficulty{Gameplay: GameplayDuel, Rows: 5, Cols: 5, Mines: 1}, true, MemberAnon, 42)
	if bare.duel.Duration != 300 || bare.duel.AIInterval != 0.72 {
		t.Fatalf("unconfigured duel must fall back to 5min/0.72s: duration=%v interval=%v", bare.duel.Duration, bare.duel.AIInterval)
	}
	nan := NewGame(Difficulty{Gameplay: GameplayDuel, Rows: 5, Cols: 5, Mines: 1, DuelDuration: math.NaN(), AIInterval: math.Inf(1)}, true, MemberAnon, 42)
	if nan.duel.Duration != 300 || nan.duel.AIInterval != 0.72 {
		t.Fatalf("non-finite pacing must fall back to 5min/0.72s: duration=%v interval=%v", nan.duel.Duration, nan.duel.AIInterval)
	}

	g := NewGame(Difficulty{Gameplay: GameplayDuel, Rows: 8, Cols: 8, Mines: 32, DuelDuration: 60, AIInterval: 0.25}, true, MemberAnon, 42)
	g.FirstClick = true
	g.Status = StatusPlaying
	g.duel.rng = rand.New(rand.NewPCG(7, 9))
	// Flagged checkerboard mines: every AI step is one safe reveal, no flood or mine hit.
	for i := range g.Cells {
		c := &g.Cells[i]
		if (c.Row+c.Col)%2 == 0 {
			c.IsMine = true
			c.Flagged = true
		}
	}
	g.RecomputeAdjacency()
	if g.duel.Duration != 60 || g.duel.AIInterval != 0.25 {
		t.Fatalf("track pacing not applied: duration=%v interval=%v", g.duel.Duration, g.duel.AIInterval)
	}
	if tick := g.TickDuel(1.05); tick.AITurn != nil {
		t.Fatal("AI acted before its first step")
	}
	if tick := g.TickDuel(0.1); tick.AITurn == nil || g.duel.NextAI != g.duel.Elapsed+0.25 {
		t.Fatalf("AI did not step at the track interval: %+v next=%v elapsed=%v", tick, g.duel.NextAI, g.duel.Elapsed)
	}
	if g.TickDuel(0.1).AITurn != nil {
		t.Fatal("AI stepped early under custom interval")
	}
	if g.TickDuel(0.2).AITurn == nil {
		t.Fatal("AI skipped the custom interval")
	}
	if g.TickDuel(58.5).Finished {
		t.Fatal("duel finished before the track duration")
	}
	if tick := g.TickDuel(0.1); !tick.Finished || g.Status != StatusWon || g.duel.remaining() != 0 {
		t.Fatalf("duel did not end at 60s: %+v status=%v", tick, g.Status)
	}
}

func TestDuelChordCountsExplodedAndDeduplicatesSoyorinShield(t *testing.T) {
	// Case A: Exploded unflagged mine counts once toward Chord.
	g := NewGame(Difficulty{Gameplay: GameplayDuel, Rows: 5, Cols: 5, Mines: 1}, true, MemberAnon, 42)
	g.FirstClick = true
	g.Status = StatusPlaying
	g.Cell(0, 0).IsMine = true
	g.RecomputeAdjacency()
	for i := range g.Cells {
		c := &g.Cells[i]
		if c.Row == 0 && c.Col == 0 {
			c.Exploded = true
			c.Revealed = true
			continue
		}
		if c.Row == 0 && c.Col == 1 {
			continue
		}
		c.Revealed = true
		c.Owner = DuelPlayer
		g.RevealedSafe++
	}
	res := g.Chord(1, 1)
	if res.Kind != ActionRevealed && res.Kind != ActionWon {
		t.Fatalf("expected chord around exploded mine to reveal safe cell, got %+v", res)
	}
	if !g.Cell(0, 1).Revealed {
		t.Fatal("expected (0,1) to be revealed by chord")
	}

	// Case B: Soyorin shield sets both Exploded and Flagged; must count once, not twice.
	g2 := NewGame(Difficulty{Gameplay: GameplayDuel, Rows: 5, Cols: 5, Mines: 2, SkillCharges: 1}, true, MemberSoyorin, 42)
	g2.FirstClick = true
	g2.Status = StatusPlaying
	g2.Cell(0, 0).IsMine = true
	g2.Cell(0, 2).IsMine = true
	g2.RecomputeAdjacency()
	for i := range g2.Cells {
		c := &g2.Cells[i]
		if (c.Row == 0 && c.Col == 0) || (c.Row == 0 && c.Col == 1) || (c.Row == 0 && c.Col == 2) {
			continue
		}
		c.Revealed = true
		c.Owner = DuelPlayer
		g2.RevealedSafe++
	}
	if sr := g2.Reveal(0, 0); sr.Kind != ActionShielded {
		t.Fatalf("expected Soyorin shield, got %+v", sr)
	}
	// Only 1 known mine around (1,1) which has Adjacent=2 -> must Noop, not double-count!
	if nr := g2.Chord(1, 1); nr.Kind != ActionNoop {
		t.Fatalf("Soyorin shield double-counted in chord: %+v", nr)
	}
	// Flagging the second mine satisfies Adjacent=2 -> reveals (0,1).
	g2.ToggleFlag(0, 2, false)
	if wr := g2.Chord(1, 1); wr.Kind != ActionRevealed && wr.Kind != ActionWon {
		t.Fatalf("expected chord to succeed after flagging second mine, got %+v", wr)
	}
	if !g2.Cell(0, 1).Revealed {
		t.Fatal("expected (0,1) revealed after second mine flagged")
	}

	// Stunned chord must Noop.
	g3 := NewGame(Difficulty{Gameplay: GameplayDuel, Rows: 5, Cols: 5, Mines: 1}, true, MemberAnon, 42)
	g3.FirstClick = true
	g3.Status = StatusPlaying
	g3.Cell(0, 0).IsMine = true
	g3.RecomputeAdjacency()
	g3.Cell(1, 1).Revealed = true
	g3.Cell(0, 0).Flagged = true
	g3.duel.StunnedUntil = 2.4
	if sr := g3.Chord(1, 1); sr.Kind != ActionNoop {
		t.Fatalf("expected stunned chord Noop, got %+v", sr)
	}
}

func TestDuelClaimMinesAndWrongFlagPenalty(t *testing.T) {
	// Valid claim awards +75/mine.
	g := NewGame(Difficulty{Gameplay: GameplayDuel, Rows: 5, Cols: 5, Mines: 1}, true, MemberAnon, 42)
	g.FirstClick = true
	g.Status = StatusPlaying
	g.Cell(0, 0).IsMine = true
	g.RecomputeAdjacency()
	for i := range g.Cells {
		c := &g.Cells[i]
		if c.Row == 0 && c.Col == 0 {
			continue
		}
		c.Revealed = true
		c.Owner = DuelPlayer
		g.RevealedSafe++
	}
	res := g.ClaimDuelMines(1, 1)
	if res.Kind != ActionFlagged || res.RevealedCount != 1 || g.duel.PlayerScore != 75 {
		t.Fatalf("expected +75 claim, got %+v score=%d", res, g.duel.PlayerScore)
	}

	// Wrong-flag scenario where public condition matches: must reveal non-mine cell instead of Noop.
	g2 := NewGame(Difficulty{Gameplay: GameplayDuel, Rows: 5, Cols: 5, Mines: 2}, true, MemberAnon, 42)
	g2.FirstClick = true
	g2.Status = StatusPlaying
	g2.Cell(4, 4).IsMine = true
	g2.Cell(4, 3).IsMine = true
	g2.RecomputeAdjacency()
	for i := range g2.Cells {
		c := &g2.Cells[i]
		if (c.Row == 0 && c.Col == 0) || (c.Row == 0 && c.Col == 1) || c.IsMine {
			continue
		}
		c.Revealed = true
		c.Owner = DuelPlayer
		g2.RevealedSafe++
	}
	// Place wrong flag at (0,0) and set (1,1).Adjacent = 2 so unknown(1) == 2 - flagged(1).
	g2.ToggleFlag(0, 0, false)
	g2.Cell(1, 1).Adjacent = 2
	beforeScore := g2.duel.PlayerScore
	claimRes := g2.ClaimDuelMines(1, 1)
	if claimRes.Kind == ActionNoop || !g2.Cell(0, 1).Revealed || g2.duel.PlayerScore != beforeScore+1 {
		t.Fatalf("expected non-mine (0,1) revealed instead of risk-free noop, got %+v revealed=%v score=%d", claimRes, g2.Cell(0, 1).Revealed, g2.duel.PlayerScore)
	}
}

func TestDuelHoverHintsPriorityPublicOnlyAndStun(t *testing.T) {
	// Strictly public-state check: non-mine cell looks claimable from public numbers alone.
	g := NewGame(Difficulty{Gameplay: GameplayDuel, Rows: 5, Cols: 5, Mines: 1}, true, MemberAnon, 42)
	g.FirstClick = true
	g.Status = StatusPlaying
	g.Cell(4, 4).IsMine = true
	g.RecomputeAdjacency()
	for i := range g.Cells {
		c := &g.Cells[i]
		if (c.Row == 0 && c.Col == 0) || c.IsMine {
			continue
		}
		c.Revealed = true
		c.Owner = DuelPlayer
		g.RevealedSafe++
	}
	g.Cell(1, 1).Adjacent = 1 // 1 unknown neighbor (0,0), even though (0,0).IsMine == false
	h := g.InspectNumberHover(1, 1, false)
	if h.Kind != HintClaimable || len(h.Targets) != 1 || h.Targets[0] != 0 {
		t.Fatalf("expected HintClaimable from public state alone, got %+v", h)
	}

	// Exploded neighbor counts toward Chordable hint, and stun suppresses hover hints.
	g2 := NewGame(Difficulty{Gameplay: GameplayDuel, Rows: 5, Cols: 5, Mines: 1}, true, MemberAnon, 42)
	g2.FirstClick = true
	g2.Status = StatusPlaying
	g2.Cell(0, 0).IsMine = true
	g2.RecomputeAdjacency()
	g2.Cell(0, 0).Exploded = true
	g2.Cell(0, 0).Revealed = true
	g2.Cell(1, 1).Revealed = true
	ch := g2.InspectNumberHover(1, 1, false)
	if ch.Kind != HintChordable || len(ch.Targets) != 7 {
		t.Fatalf("expected HintChordable with 7 targets around exploded mine, got %+v", ch)
	}
	g2.duel.StunnedUntil = g2.duel.Elapsed + 2.4
	if st := g2.InspectNumberHover(1, 1, false); st.Kind != HintNone {
		t.Fatalf("expected HintNone while stunned, got %+v", st)
	}

	// Headless UI Duel canvas hover + flagMode toggle + claim click consistency.
	app := newMinesweeperApp(42)
	app.diffIdx = 4
	app.game = NewGame(Difficulty{Gameplay: GameplayDuel, Rows: 6, Cols: 6, Mines: 1}, true, MemberAnon, 42)
	app.game.FirstClick = true
	app.game.Status = StatusPlaying
	app.game.Cell(0, 0).IsMine = true
	app.game.RecomputeAdjacency()
	for i := range app.game.Cells {
		c := &app.game.Cells[i]
		if c.Row == 0 && c.Col == 0 {
			continue
		}
		c.Revealed = true
		c.Owner = DuelPlayer
		app.game.RevealedSafe++
	}
	tt := ui.NewTester(app.view, 1280, 900)
	app.board.center(app.game, 1, 1)
	tt.Frame()
	r, ok := tt.Find("百万格决斗战场")
	if !ok {
		t.Fatal("duel board missing")
	}
	px := r.X + (1.5)*app.board.Cell - app.board.X
	py := r.Y + (1.5)*app.board.Cell - app.board.Y
	tt.Move(px, py)
	if app.board.LastHoverHint.Kind != HintClaimable {
		t.Fatalf("expected duel canvas LastHoverHint=HintClaimable, got %+v", app.board.LastHoverHint)
	}
	tt.Key(0, ui.KeyF)
	tt.Frame()
	if app.board.LastHoverHint.Kind != HintNone {
		t.Fatalf("expected duel LastHoverHint=HintNone in flagMode, got %+v", app.board.LastHoverHint)
	}
	tt.Key(0, ui.KeyF)
	tt.ClickAt(px, py)
	if app.game.duel.PlayerScore != 75 || !app.game.Cell(0, 0).Flagged {
		t.Fatalf("expected left click on HintClaimable cell to claim mine (+75), score=%d flagged=%v", app.game.duel.PlayerScore, app.game.Cell(0, 0).Flagged)
	}
}

func TestDuelHoverSyncsAfterZoomAndDrag(t *testing.T) {
	app := newMinesweeperApp(42)
	app.diffIdx = 4
	app.game = NewGame(Difficulty{Gameplay: GameplayDuel, Rows: 60, Cols: 60, Mines: 2}, true, MemberAnon, 42)
	app.game.FirstClick = true
	app.game.Status = StatusPlaying
	app.game.Cell(20, 20).IsMine = true
	app.game.Cell(30, 30).IsMine = true
	app.game.RecomputeAdjacency()
	for i := range app.game.Cells {
		c := &app.game.Cells[i]
		if c.IsMine {
			continue
		}
		c.Revealed = true
		c.Owner = DuelPlayer
		app.game.RevealedSafe++
	}

	tt := ui.NewTester(app.view, 1280, 900)
	app.board.center(app.game, 25, 25)
	tt.Frame()
	r, ok := tt.Find("百万格决斗战场")
	if !ok {
		t.Fatal("duel board missing")
	}

	// Hover claimable (21,21) near top-left of viewport, then zoom via Ctrl+= without moving pointer.
	px := r.X + (21.5)*app.board.Cell - app.board.X
	py := r.Y + (21.5)*app.board.Cell - app.board.Y
	tt.Move(px, py)
	if app.board.LastHoverHint.Kind != HintClaimable || app.board.LastHoverHint.Center != 21*60+21 {
		t.Fatalf("expected initial hover on claimable (21,21), got %+v", app.board.LastHoverHint)
	}
	tt.Key(ui.Ctrl, ui.KeyEqual)
	zoomRow, zoomCol, ok := app.board.cellAt(app.game, px-r.X, py-r.Y)
	if !ok {
		t.Fatal("pointer out of bounds after zoom")
	}
	if app.board.HoverRow != zoomRow || app.board.HoverCol != zoomCol {
		t.Fatalf("hover row/col stale after zoom: hover=(%d,%d) actual=(%d,%d)", app.board.HoverRow, app.board.HoverCol, zoomRow, zoomCol)
	}
	wantZoomHint := app.game.InspectNumberHover(zoomRow, zoomCol, app.flagMode)
	if app.board.LastHoverHint.Kind != wantZoomHint.Kind || app.board.LastHoverHint.Center != wantZoomHint.Center {
		t.Fatalf("hover hint stale after zoom: got=%+v want=%+v", app.board.LastHoverHint, wantZoomHint)
	}

	// Drag viewport so pointer ends up over (31,31) upon release without an extra Move.
	app.board.center(app.game, 25, 25)
	tt.Frame()
	x21 := r.X + (21.5)*app.board.Cell - app.board.X
	y21 := r.Y + (21.5)*app.board.Cell - app.board.Y
	x31 := r.X + (31.5)*app.board.Cell - app.board.X
	y31 := r.Y + (31.5)*app.board.Cell - app.board.Y
	// Panning maps canvas(31,31) -> (x21,y21) when dragging from x31->x21... wait:
	// Dragging right/down moves v.X/v.Y opposite to delta: to bring (31,31) under release point (x21,y21),
	// start at (x31, y31) and drag to (x21, y21) so delta = (x21-x31, y21-y31), v.X += x31-x21 -> (31,31) lands at (x21,y21)!
	tt.Press(x31, y31)
	tt.Move(x21, y21)
	tt.Release(x21, y21)
	dragRow, dragCol, ok := app.board.cellAt(app.game, x21-r.X, y21-r.Y)
	if !ok || dragRow != 31 || dragCol != 31 {
		t.Fatalf("expected (31,31) under drag release point, got (%d,%d) ok=%v", dragRow, dragCol, ok)
	}
	if app.board.HoverRow != 31 || app.board.HoverCol != 31 {
		t.Fatalf("hover stale after drag release: hover=(%d,%d)", app.board.HoverRow, app.board.HoverCol)
	}
	if app.board.LastHoverHint.Kind != HintClaimable || app.board.LastHoverHint.Center != 31*60+31 {
		t.Fatalf("expected HintClaimable centered at (31,31) after drag, got %+v", app.board.LastHoverHint)
	}
	tt.ClickAt(x21, y21)
	if !app.game.Cell(30, 30).Flagged || app.game.duel.PlayerScore != 75 {
		t.Fatalf("expected click at release point to claim (30,30), flagged=%v score=%d", app.game.Cell(30, 30).Flagged, app.game.duel.PlayerScore)
	}

	// Moving pointer off board onto HUD clears hover hint cleanly.
	hudR, ok := tt.Find("回到战场")
	if !ok {
		t.Fatal("HUD button missing")
	}
	tt.Move(hudR.X+hudR.W/2, hudR.Y+hudR.H/2)
	tt.Frame()
	if app.board.HoverRow != -1 || app.board.LastHoverHint.Kind != HintNone {
		t.Fatalf("expected hover cleared when leaving board, hover=(%d,%d) hint=%+v", app.board.HoverRow, app.board.HoverCol, app.board.LastHoverHint)
	}
}

func TestDuelFloatingScorePopupsAndRippleLifecycle(t *testing.T) {
	app := newMinesweeperApp(42)
	app.diffIdx = 4
	app.game = NewGame(Difficulty{Gameplay: GameplayDuel, Rows: 12, Cols: 12, Mines: 1}, true, MemberAnon, 42)
	app.game.Cell(11, 11).IsMine = true
	app.game.FirstClick = true
	app.game.Status = StatusPlaying
	app.game.RecomputeAdjacency()

	tt := ui.NewTester(app.view, 1280, 900)
	app.board.center(app.game, 5, 5)
	tt.Frame()
	r, _ := tt.Find("百万格决斗战场")

	// Normal flag placement has ScoreDelta==0 -> must NOT spawn a "+0" popup.
	tt.Key(0, ui.KeyF)
	fx := r.X + 0.5*app.board.Cell - app.board.X
	fy := r.Y + 0.5*app.board.Cell - app.board.Y
	tt.ClickAt(fx, fy)
	if len(app.duelPopups) != 0 {
		t.Fatalf("expected no popup for 0-score flag, got %+v", app.duelPopups)
	}
	tt.Key(0, ui.KeyF)

	// Unflag (0,1) so large reveal opens nearly full board (combo/large opening popup + BFS ripples).
	app.game.UnflagDuel(0, 1)
	app.game.duel.PlayerCombo = 3
	cx := r.X + 5.5*app.board.Cell - app.board.X
	cy := r.Y + 5.5*app.board.Cell - app.board.Y
	tt.ClickAt(cx, cy)
	if len(app.ripples) == 0 {
		t.Fatal("expected ripples on large duel opening")
	}
	if len(app.duelPopups) != 1 || app.duelPopups[0].Text == "" {
		t.Fatalf("expected 1 popup on large duel opening, got %+v", app.duelPopups)
	}

	// Claimable mine popup (+75 收雷!) + bounded cap test.
	gClaim := NewGame(Difficulty{Gameplay: GameplayDuel, Rows: 6, Cols: 6, Mines: 1}, true, MemberAnon, 42)
	gClaim.FirstClick = true
	gClaim.Status = StatusPlaying
	gClaim.Cell(0, 0).IsMine = true
	gClaim.RecomputeAdjacency()
	for i := range gClaim.Cells {
		c := &gClaim.Cells[i]
		if c.Row == 0 && c.Col == 0 {
			continue
		}
		c.Revealed = true
		c.Owner = DuelPlayer
		gClaim.RevealedSafe++
	}
	resClaim := gClaim.ClaimDuelMines(1, 1)
	app.game = gClaim
	for i := 0; i < maxDuelPopups+4; i++ {
		app.spawnDuelPopup(1, 1, resClaim, tt.Now())
	}
	if len(app.duelPopups) != maxDuelPopups {
		t.Fatalf("expected duelPopups capped at %d, got %d", maxDuelPopups, len(app.duelPopups))
	}
	if app.duelPopups[len(app.duelPopups)-1].Text != "+75 收雷!" {
		t.Fatalf("expected real score delta popup '+75 收雷!', got %q", app.duelPopups[len(app.duelPopups)-1].Text)
	}

	// Expire both ripples and popups via virtual clock.
	tt.Advance(duelPopupDuration + rippleMaxDelay + rippleDuration + 50*time.Millisecond)
	if len(app.duelPopups) != 0 || len(app.ripples) != 0 {
		t.Fatalf("expected popups & ripples expired under virtual clock, popups=%d ripples=%d", len(app.duelPopups), len(app.ripples))
	}

	// Trigger popup then immediate R reset cleanup.
	app.spawnDuelPopup(1, 1, resClaim, tt.Now())
	tt.Key(0, ui.KeyR)
	if len(app.duelPopups) != 0 || len(app.ripples) != 0 {
		t.Fatalf("expected popups & ripples cleared on R reset, popups=%d ripples=%d", len(app.duelPopups), len(app.ripples))
	}
}

func TestDuelShakeBoundsAndStunWarningHUD(t *testing.T) {
	app := newMinesweeperApp(42)
	app.diffIdx = 4
	app.memberID = MemberTomori
	app.game = NewGame(Difficulty{Gameplay: GameplayDuel, Rows: 8, Cols: 8, Mines: 2, SkillCharges: 1}, true, MemberTomori, 42)
	app.game.FirstClick = true
	app.game.Status = StatusPlaying
	app.game.Cell(3, 3).IsMine = true
	app.game.Cell(4, 4).IsMine = true
	app.game.RecomputeAdjacency()

	tt := ui.NewTester(app.view, 1280, 900)
	app.board.center(app.game, 3, 3)
	tt.Frame()
	boardBefore, ok := tt.Find("百万格决斗战场")
	if !ok {
		t.Fatal("duel board not found")
	}
	hudBefore, ok := tt.Find("回到战场")
	if !ok {
		t.Fatal("duel HUD button not found")
	}

	// Click mine at (3,3) -> 9.0 shake + 2.4s stun + Tomori E cleanse hint!
	mx := boardBefore.X + (3.5)*app.board.Cell - app.board.X
	my := boardBefore.Y + (3.5)*app.board.Cell - app.board.Y
	tt.ClickAt(mx, my)
	if !app.game.duel.playerStunned() || app.shakeAmplitude != 9.0 {
		t.Fatalf("expected duel stun + 9.0 shake, stunned=%v amp=%v", app.game.duel.playerStunned(), app.shakeAmplitude)
	}
	if !tt.HasText("踩雷硬直中") || !tt.HasText("按 E 发动「创可贴指引」立即解除硬直") {
		t.Fatal("expected stun countdown banner with Tomori E cleanse hint")
	}

	// Verify shake offset active after 30ms without shifting duel board or HUD bounds.
	tt.Advance(30 * time.Millisecond)
	sx, sy := app.currentShakeOffset(tt.Now())
	if sx == 0 && sy == 0 {
		t.Fatal("expected active duel shake at 30ms")
	}
	boardDuring, _ := tt.Find("百万格决斗战场")
	hudDuring, _ := tt.Find("回到战场")
	if boardDuring != boardBefore || hudDuring != hudBefore {
		t.Fatalf("duel stage or HUD bounds moved during shake: board=%+v/%+v hud=%+v/%+v", boardBefore, boardDuring, hudBefore, hudDuring)
	}

	// Press E with Tomori -> cleanses stun immediately and hides banner!
	tt.Key(0, ui.KeyE)
	if app.game.duel.playerStunned() || tt.HasText("踩雷硬直中") {
		t.Fatal("expected Tomori E cleanse to clear stun banner immediately")
	}

	// Stun again when Tomori has 0 skill charges -> must show exhausted message, NOT "按 E 发动".
	tt.ClickAt(boardBefore.X+(4.5)*app.board.Cell-app.board.X, boardBefore.Y+(4.5)*app.board.Cell-app.board.Y)
	if !app.game.duel.playerStunned() {
		t.Fatal("expected second stun")
	}
	if tt.HasText("按 E 发动「创可贴指引」立即解除硬直") || !tt.HasText("技能充能已耗尽 · 等待硬直自然结束") {
		t.Fatal("expected exhausted-charge copy instead of E cleanse hint")
	}

	// Switch copy check for non-Tomori member (Anon).
	app.memberID = MemberAnon
	app.game.MemberID = MemberAnon
	app.game.SkillUsesLeft = 1
	tt.Frame()
	if tt.HasText("按 E 发动") || !tt.HasText("暂无法操作棋盘 · 等待硬直结束") {
		t.Fatal("expected non-Tomori stun copy without E cleanse hint")
	}

	// Keep AI from clearing the tiny 8x8 fixture before the 2.4s natural stun check.
	app.game.duel.NextAI = 999
	// Natural expiration after 2.4s clears shake & stun banner while still playing.
	tt.Advance(2500 * time.Millisecond)
	if app.game.duel.playerStunned() || tt.HasText("踩雷硬直中") {
		t.Fatalf("expected stun banner hidden after 2.4s: elapsed=%v stunnedUntil=%v stunned=%v hasText=%v status=%v",
			app.game.duel.Elapsed, app.game.duel.StunnedUntil, app.game.duel.playerStunned(), tt.HasText("踩雷硬直中"), app.game.Status)
	}
	sxEnd, syEnd := app.currentShakeOffset(tt.Now())
	if sxEnd != 0 || syEnd != 0 {
		t.Fatalf("expected shake zeroed, got (%v,%v)", sxEnd, syEnd)
	}

	// Reset while stunned clears banner and shake immediately.
	app.game.duel.StunnedUntil = app.game.duel.Elapsed + 2.4
	app.triggerShake(tt.Now(), 9.0, 320*time.Millisecond)
	tt.Frame()
	if !tt.HasText("踩雷硬直中") {
		t.Fatal("expected stun banner before R reset")
	}
	tt.Key(0, ui.KeyR)
	if app.game.duel.playerStunned() || tt.HasText("踩雷硬直中") || app.shakeAmplitude != 0 {
		t.Fatal("expected R reset to clear stun banner and shake immediately")
	}

	// Duel finish while stunned immediately hides stun banner.
	app.game.Status = StatusPlaying
	app.game.duel.StunnedUntil = app.game.duel.Elapsed + 2.4
	tt.Frame()
	if !tt.HasText("踩雷硬直中") {
		t.Fatal("expected stun banner before duel finish")
	}
	app.game.TickDuel(300)
	tt.Frame()
	if app.game.Status != StatusWon || tt.HasText("踩雷硬直中") {
		t.Fatalf("expected stun banner hidden on duel finish: status=%v hasText=%v", app.game.Status, tt.HasText("踩雷硬直中"))
	}
}

func TestDuelRecenterUsesBoardCenter(t *testing.T) {
	cases := []struct {
		name string
		rows int
		cols int
	}{
		{name: "standard-1000x1000", rows: 1000, cols: 1000},
		{name: "medium-500x500", rows: 500, cols: 500},
		{name: "small-100x100", rows: 100, cols: 100},
		{name: "rect-60x80", rows: 60, cols: 80},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := newMinesweeperApp(42)
			app.diffIdx = 4
			app.game = NewGame(Difficulty{Gameplay: GameplayDuel, Rows: tc.rows, Cols: tc.cols, Mines: 10}, true, MemberAnon, 42)
			tt := ui.NewTester(app.view, 1280, 900)
			tt.Frame()

			r, ok := tt.Find("百万格决斗战场")
			if !ok {
				t.Fatal("duel board missing")
			}

			wantRow, wantCol := tc.rows/2, tc.cols/2

			// Pan to top-left corner, then click "回到战场".
			app.board.center(app.game, 0, 0)
			tt.Frame()
			if err := tt.Click("回到战场"); err != nil {
				t.Fatalf("failed clicking 回到战场: %v", err)
			}
			row, col, ok := app.board.cellAt(app.game, r.W/2, r.H/2)
			if !ok || row != wantRow || col != wantCol {
				t.Fatalf("button recenter got (%d,%d) ok=%v, want (%d,%d)", row, col, ok, wantRow, wantCol)
			}

			// Pan to bottom-right corner, then press Ctrl+0.
			app.board.center(app.game, tc.rows-1, tc.cols-1)
			tt.Frame()
			tt.Key(ui.Ctrl, ui.Key0)
			row, col, ok = app.board.cellAt(app.game, r.W/2, r.H/2)
			if !ok || row != wantRow || col != wantCol {
				t.Fatalf("Ctrl+0 recenter got (%d,%d) ok=%v, want (%d,%d)", row, col, ok, wantRow, wantCol)
			}
		})
	}

	// Small board fully inside viewport clamps cleanly to (0,0) without panic or negative offsets.
	app := newMinesweeperApp(42)
	app.diffIdx = 4
	app.game = NewGame(Difficulty{Gameplay: GameplayDuel, Rows: 12, Cols: 12, Mines: 1}, true, MemberAnon, 42)
	tt := ui.NewTester(app.view, 1280, 900)
	tt.Frame()
	tt.Key(ui.Ctrl, ui.Key0)
	if app.board.X != 0 || app.board.Y != 0 {
		t.Fatalf("expected small board recenter clamped to (0,0), got (%v,%v)", app.board.X, app.board.Y)
	}
}

func TestDuelUIUsesGameplayNotTrackID(t *testing.T) {
	orig := Difficulties
	defer func() { Difficulties = orig }()

	// Case 1: Non-"duel" track ID with GameplayDuel activates all duel UI branches.
	Difficulties = append([]Difficulty(nil), orig...)
	Difficulties[0] = Difficulty{
		ID:           "encore",
		Label:        "测试决斗",
		Song:         "潜在表明",
		Subtitle:     "12×12 · 5雷",
		Gameplay:     GameplayDuel,
		Rows:         12,
		Cols:         12,
		Mines:        5,
		SkillCharges: 3,
	}
	app := newMinesweeperApp(42)
	app.modeIdx = 1
	app.resetGame(false)

	if app.modeIdx != 0 || !app.game.BandMode || app.game.SkillUsesLeft != 3 {
		t.Fatalf("expected GameplayDuel track to force band mode, modeIdx=%d band=%v charges=%d", app.modeIdx, app.game.BandMode, app.game.SkillUsesLeft)
	}
	foundDuelChat := false
	for _, msg := range app.chatLog {
		if msg.Speaker == "MyGO!!!!! 决斗室" {
			foundDuelChat = true
			break
		}
	}
	if !foundDuelChat {
		t.Fatal("expected duel intro chat for GameplayDuel track regardless of ID")
	}

	tt := ui.NewTester(app.view, 1280, 900)
	tt.Frame()
	if _, ok := tt.Find("百万格决斗战场"); !ok || !tt.HasText("灯 (AI) 0") {
		t.Fatal("expected duel HUD and board for GameplayDuel track")
	}
	beforeGame := app.game
	_ = tt.Click("纯正经典")
	if app.modeIdx != 0 || app.game != beforeGame {
		t.Fatalf("expected mode switch disabled in GameplayDuel, got modeIdx=%d reset=%v", app.modeIdx, app.game != beforeGame)
	}
	app.game.FirstClick = true
	app.game.Status = StatusPlaying
	app.game.TickDuel(300)
	tt.Frame()
	if !tt.HasText("决斗结束 · 再来一局 (R)") {
		t.Fatal("expected duel won reset label after match finish")
	}

	// Case 2: Swapping app.game directly to GameplayDuel still disables mode switch and shows duel HUD even when diffIdx points to classic.
	Difficulties = append([]Difficulty(nil), orig...)
	app2 := newMinesweeperApp(42)
	app2.game = NewGame(Difficulty{ID: "encore", Gameplay: GameplayDuel, Rows: 12, Cols: 12, Mines: 5}, true, MemberAnon, 42)
	tt2 := ui.NewTester(app2.view, 1280, 900)
	tt2.Frame()
	if _, ok := tt2.Find("百万格决斗战场"); !ok {
		t.Fatal("expected duel board when game has GameplayDuel")
	}
	beforeGame2 := app2.game
	_ = tt2.Click("纯正经典")
	if app2.modeIdx != 0 || app2.game != beforeGame2 {
		t.Fatalf("expected mode switch disabled for GameplayDuel game, got modeIdx=%d reset=%v", app2.modeIdx, app2.game != beforeGame2)
	}

	// Case 3: Track ID "duel" with GameplayClassic stays classic in every UI branch.
	Difficulties = append([]Difficulty(nil), orig...)
	Difficulties[0] = Difficulty{
		ID:           "duel",
		Label:        "同名经典",
		Song:         "迷星叫",
		Subtitle:     "9×9 · 10雷",
		Gameplay:     GameplayClassic,
		Rows:         9,
		Cols:         9,
		Mines:        10,
		SkillCharges: 1,
	}
	app3 := newMinesweeperApp(42)
	app3.modeIdx = 1
	app3.resetGame(false)

	if app3.modeIdx != 1 || app3.game.BandMode {
		t.Fatalf("track ID 'duel' with GameplayClassic must not force band mode: modeIdx=%d band=%v", app3.modeIdx, app3.game.BandMode)
	}
	for _, msg := range app3.chatLog {
		if msg.Speaker == "MyGO!!!!! 决斗室" {
			t.Fatal("track ID 'duel' with GameplayClassic must not push duel intro chat")
		}
	}
	tt3 := ui.NewTester(app3.view, 1280, 900)
	tt3.Frame()
	if _, ok := tt3.Find("cell-0-0"); !ok {
		t.Fatal("expected classic board for GameplayClassic track even with ID 'duel'")
	}
	if _, ok := tt3.Find("百万格决斗战场"); ok || tt3.HasText("灯 (AI) 0") {
		t.Fatal("track ID 'duel' with GameplayClassic must not render duel HUD/board")
	}
	if err := tt3.Click("乐队 Live"); err != nil {
		t.Fatalf("failed clicking enabled mode switch: %v", err)
	}
	if app3.modeIdx != 0 {
		t.Fatalf("expected mode switch enabled for GameplayClassic track, got modeIdx=%d", app3.modeIdx)
	}
}

func TestBoardCanvasSupports50And100Sizes(t *testing.T) {
	cases := []struct {
		name  string
		rows  int
		cols  int
		mines int
	}{
		{name: "50x50", rows: 50, cols: 50, mines: 430},
		{name: "100x100", rows: 100, cols: 100, mines: 1600},
		{name: "60x80", rows: 60, cols: 80, mines: 800},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := newMinesweeperApp(42)
			app.diffIdx = 4
			app.game = NewGame(Difficulty{
				ID:       "test-" + tc.name,
				Gameplay: GameplayDuel,
				Rows:     tc.rows,
				Cols:     tc.cols,
				Mines:    tc.mines,
			}, true, MemberAnon, 42)

			tt := ui.NewTester(app.view, 960, 640)
			tt.Frame()

			r, ok := tt.Find("百万格决斗战场")
			if !ok {
				t.Fatal("duel board missing")
			}

			// Initial viewport centers on board middle rather than clamping to bottom-right.
			centerRow, centerCol, ok := app.board.cellAt(app.game, r.W/2, r.H/2)
			if !ok || centerRow != tc.rows/2 || centerCol != tc.cols/2 {
				t.Fatalf("initial center got (%d,%d) ok=%v, want (%d,%d)", centerRow, centerCol, ok, tc.rows/2, tc.cols/2)
			}

			// Clicking center reveals the centered cell.
			tt.ClickAt(r.X+r.W/2, r.Y+r.H/2)
			if !app.game.FirstClick || !app.game.Cell(centerRow, centerCol).Revealed {
				t.Fatalf("click at initial center (%d,%d) did not reveal", centerRow, centerCol)
			}

			// Scroll zoom keeps world anchor fixed and respects [10, 42] bounds.
			anchorX, anchorY := r.W*0.35, r.H*0.4
			wx := (app.board.X + anchorX) / app.board.Cell
			wy := (app.board.Y + anchorY) / app.board.Cell
			tt.Scroll(r.X+anchorX, r.Y+anchorY, 0, -240)
			if math.Abs(float64((app.board.X+anchorX)/app.board.Cell-wx)) > 0.02 ||
				math.Abs(float64((app.board.Y+anchorY)/app.board.Cell-wy)) > 0.02 {
				t.Fatalf("zoom anchor drifted on %s", tc.name)
			}

			// Zoom all the way out to Cell=10 (smaller than viewport for 50x50) and verify clamping & cellAt edges.
			app.board.zoom(app.game, r.W/2, r.H/2, 0.01)
			if app.board.Cell != 10 {
				t.Fatalf("expected min cell size 10, got %v", app.board.Cell)
			}
			maxX := max(float32(0), float32(tc.cols)*app.board.Cell-app.board.Width)
			maxY := max(float32(0), float32(tc.rows)*app.board.Cell-app.board.Height)
			if app.board.X < 0 || app.board.Y < 0 || app.board.X > maxX || app.board.Y > maxY {
				t.Fatalf("clamped offset out of bounds: (%v,%v) max=(%v,%v)", app.board.X, app.board.Y, maxX, maxY)
			}
			if row, col, ok := app.board.cellAt(app.game, -5, 10); ok {
				t.Fatalf("cellAt accepted negative x: (%d,%d)", row, col)
			}
			if row, col, ok := app.board.cellAt(app.game, float32(tc.cols)*app.board.Cell+5, 10); ok {
				t.Fatalf("cellAt accepted x past board edge: (%d,%d)", row, col)
			}

			// Minimap viewport rect remains strictly inside minimap bounds even when board < viewport.
			mini := app.board.minimapBoundsFor(app.game)
			mini.X += r.X
			mini.Y += r.Y
			vp := minimapViewportRect(mini, tc.rows, tc.cols, app.board.X, app.board.Y, r.W, r.H, app.board.Cell)
			if vp.X < mini.X || vp.Y < mini.Y || vp.X+vp.W > mini.X+mini.W+0.01 || vp.Y+vp.H > mini.Y+mini.H+0.01 {
				t.Fatalf("minimap viewport escaped mini bounds at min zoom: vp=%+v mini=%+v", vp, mini)
			}

			// Zoom to max (Cell=42) so both 50x50 and 100x100 exceed viewport, then test minimap click-to-jump.
			app.board.zoom(app.game, r.W/2, r.H/2, 10)
			if app.board.Cell != 42 {
				t.Fatalf("expected max cell size 42, got %v", app.board.Cell)
			}

			localMini := app.board.minimapBoundsFor(app.game)
			// Click top-left corner of minimap -> clamps to (0,0).
			tt.ClickAt(r.X+localMini.X+1, r.Y+localMini.Y+1)
			if app.board.X != 0 || app.board.Y != 0 {
				t.Fatalf("minimap top-left jump did not clamp to (0,0): (%v,%v)", app.board.X, app.board.Y)
			}

			// Click bottom-right corner of minimap -> clamps to (maxX, maxY).
			tt.ClickAt(r.X+localMini.X+localMini.W-1, r.Y+localMini.Y+localMini.H-1)
			wantMaxX := max(float32(0), float32(tc.cols)*app.board.Cell-app.board.Width)
			wantMaxY := max(float32(0), float32(tc.rows)*app.board.Cell-app.board.Height)
			if math.Abs(float64(app.board.X-wantMaxX)) > 0.5 || math.Abs(float64(app.board.Y-wantMaxY)) > 0.5 {
				t.Fatalf("minimap bottom-right jump got (%v,%v), want (%v,%v)", app.board.X, app.board.Y, wantMaxX, wantMaxY)
			}

			// Click interior target (rows/3, cols*2/3) on minimap -> centers viewport on that exact cell.
			targetRow, targetCol := tc.rows/3, tc.cols*2/3
			clickX := r.X + localMini.X + (float32(targetCol)+0.5)/float32(tc.cols)*localMini.W
			clickY := r.Y + localMini.Y + (float32(targetRow)+0.5)/float32(tc.rows)*localMini.H
			tt.ClickAt(clickX, clickY)
			gotRow, gotCol, ok := app.board.cellAt(app.game, r.W/2, r.H/2)
			if !ok || absInt(gotRow-targetRow) > 1 || absInt(gotCol-targetCol) > 1 {
				t.Fatalf("minimap jump to (%d,%d) landed at (%d,%d) ok=%v", targetRow, targetCol, gotRow, gotCol, ok)
			}

			// Minimap marker clamping stays within minimap bounds for all board corners.
			for _, idx := range []int{0, tc.cols - 1, (tc.rows - 1) * tc.cols, tc.rows*tc.cols - 1} {
				mx, my := minimapMarkerPos(mini, tc.rows, tc.cols, idx, 5)
				if mx < mini.X+5 || mx > mini.X+mini.W-5 || my < mini.Y+5 || my > mini.Y+mini.H-5 {
					t.Fatalf("marker out of minimap bounds on %s idx=%d: (%v,%v)", tc.name, idx, mx, my)
				}
			}

			// Zoom-out then Ctrl+0 recenters cleanly.
			tt.Key(ui.Ctrl, ui.KeyMinus)
			tt.Key(ui.Ctrl, ui.Key0)
			reRow, reCol, ok := app.board.cellAt(app.game, r.W/2, r.H/2)
			if !ok || reRow != tc.rows/2 || reCol != tc.cols/2 {
				t.Fatalf("Ctrl+0 recenter got (%d,%d) ok=%v, want (%d,%d)", reRow, reCol, ok, tc.rows/2, tc.cols/2)
			}
		})
	}
}

func TestDuel1000FirstFrameParity(t *testing.T) {
	app := newMinesweeperApp(42)
	app.diffIdx = 4
	app.resetGame(false)

	tt := ui.NewTester(app.view, 1280, 900)
	tt.Frame()

	// Preserve original 1000x1000 initial offset (487*26, 487*26) and fixed 148x148 minimap.
	if app.board.X != 487*26 || app.board.Y != 487*26 || app.board.Cell != 26 {
		t.Fatalf("1000x1000 initial viewport drifted: X=%v Y=%v Cell=%v", app.board.X, app.board.Y, app.board.Cell)
	}
	v := newBoardView()
	v.Width, v.Height = 120, 120
	mini := v.minimapBoundsFor(app.game)
	if mini.W != 148 || mini.H != 148 {
		t.Fatalf("1000x1000 minimap size changed: %+v", mini)
	}
}

func TestBoardMinimapCoversSmallBoardsWithoutGaps(t *testing.T) {
	// Sub-100 boards (50x50 and non-divisor 60x80): production minimapAxisSlots must map every grid slot without gaps.
	for _, tc := range []struct {
		rows int
		cols int
	}{
		{rows: 50, cols: 50},
		{rows: 60, cols: 80},
	} {
		g := NewGame(Difficulty{Gameplay: GameplayDuel, Rows: tc.rows, Cols: tc.cols, Mines: 1}, true, MemberAnon, 42)
		// Reveal from (0,0) with mine tucked at bottom-right corner using public Game API.
		g.FirstClick = true
		g.Status = StatusPlaying
		g.Cell(tc.rows-1, tc.cols-1).IsMine = true
		g.RecomputeAdjacency()
		if res := g.ToggleFlag(tc.rows-1, tc.cols-1, false); res.Kind != ActionFlagged {
			t.Fatalf("%dx%d corner flag failed: %+v", tc.rows, tc.cols, res)
		}
		if res := g.Reveal(0, 0); res.Kind != ActionRevealed && res.Kind != ActionWon {
			t.Fatalf("%dx%d reveal failed: %+v", tc.rows, tc.cols, res)
		}

		rowMap, colMap := minimapAxisSlots(g)
		emptyMapped := 0
		for br := range minimapGridSize {
			for bc := range minimapGridSize {
				blk := g.duel.Minimap[rowMap[br]*minimapGridSize+colMap[bc]]
				if blk.Player == 0 {
					emptyMapped++
				}
			}
		}
		if emptyMapped > 0 {
			t.Fatalf("%dx%d minimapAxisSlots left %d empty slots", tc.rows, tc.cols, emptyMapped)
		}
	}

	// Verify classic-shaped Game (duel == nil) on 50x50 and 100x100 works safely with boardView helpers.
	for _, size := range []int{50, 100} {
		classic := &Game{Rows: size, Cols: size, Cells: make([]Cell, size*size)}
		v := newBoardView()
		v.Width, v.Height = 800, 600
		v.centerBoard(classic)
		row, col, ok := v.cellAt(classic, v.Width/2, v.Height/2)
		if !ok || row != size/2 || col != size/2 {
			t.Fatalf("classic %dx%d boardView center got (%d,%d) ok=%v", size, size, row, col, ok)
		}
		v.zoom(classic, 100, 100, 0.2)
		v.clamp(classic)
		mini := v.minimapBoundsFor(classic)
		vp := minimapViewportRect(mini, classic.Rows, classic.Cols, v.X, v.Y, v.Width, v.Height, v.Cell)
		if vp.W <= 0 || vp.H <= 0 || vp.X < mini.X || vp.Y < mini.Y || vp.X+vp.W > mini.X+mini.W+0.01 || vp.Y+vp.H > mini.Y+mini.H+0.01 {
			t.Fatalf("classic %dx%d minimap viewport invalid: vp=%+v mini=%+v", size, size, vp, mini)
		}
	}
}

func TestBoardViewNonFiniteAndZeroBounds(t *testing.T) {
	nan := float32(math.NaN())
	inf := float32(math.Inf(1))
	ninf := float32(math.Inf(-1))
	mini := ui.Rect{X: 100, Y: 100, W: 148, H: 148}

	invalidRects := []ui.Rect{
		minimapViewportRect(mini, 50, 50, 0, 0, nan, 100, 26),
		minimapViewportRect(mini, 50, 50, nan, 0, 100, 100, 26),
		minimapViewportRect(mini, 50, 50, 0, 0, inf, 100, 26),
		minimapViewportRect(mini, 50, 50, 0, 0, -1, 100, 26),
		minimapViewportRect(mini, 50, 50, 0, 0, 100, 100, 0),
		minimapViewportRect(mini, 0, 50, 0, 0, 100, 100, 26),
		minimapViewportRect(ui.Rect{X: 10, Y: 10, W: 0, H: 148}, 50, 50, 0, 0, 100, 100, 26),
	}
	for i, r := range invalidRects {
		if r != (ui.Rect{}) {
			t.Fatalf("case %d: expected empty rect for invalid geometry, got %+v", i, r)
		}
		for _, f := range []float32{r.X, r.Y, r.W, r.H} {
			if math.IsNaN(float64(f)) || math.IsInf(float64(f), 0) {
				t.Fatalf("case %d: rect contains non-finite field: %+v", i, r)
			}
		}
	}

	g := NewGame(Difficulty{Gameplay: GameplayDuel, Rows: 50, Cols: 50, Mines: 10}, true, MemberAnon, 42)
	v := newBoardView()
	v.Width, v.Height = 800, 600
	v.centerBoard(g)

	// Non-finite cellAt coordinates must return (-1, -1, false).
	for _, pt := range [][2]float32{{nan, 10}, {10, nan}, {inf, 10}, {10, ninf}} {
		if r, c, ok := v.cellAt(g, pt[0], pt[1]); ok || r != -1 || c != -1 {
			t.Fatalf("cellAt(%v,%v) = (%d,%d,%v), want (-1,-1,false)", pt[0], pt[1], r, c, ok)
		}
	}

	// Non-finite zoom inputs are ignored without poisoning boardView fields.
	v.zoom(g, nan, 10, 1.2)
	v.zoom(g, 10, 10, nan)
	v.zoom(g, 10, 10, -1)
	if math.IsNaN(float64(v.X)) || math.IsNaN(float64(v.Y)) || math.IsNaN(float64(v.Cell)) {
		t.Fatalf("zoom poisoned boardView with NaN: %+v", v)
	}

	// clamp recovers from corrupted non-finite fields.
	v.X, v.Y, v.Cell, v.Width, v.Height = nan, inf, nan, nan, ninf
	v.clamp(g)
	for _, f := range []float32{v.X, v.Y, v.Cell, v.Width, v.Height} {
		if math.IsNaN(float64(f)) || math.IsInf(float64(f), 0) {
			t.Fatalf("clamp left non-finite value in boardView: %+v", v)
		}
	}
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
