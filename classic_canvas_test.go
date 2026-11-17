package main

import (
	"fmt"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// classicCanvasTestView mirrors main.go's classic app shell (action handling,
// timer, shortcuts, Haruhikage overlay/dialog, toolbar toggles, and HUD) while
// mounting a.classicBoard(c) instead of the classic cell-widget grid so tests
// can drive both surfaces identically via ui.Tester without modifying main.go.
func classicCanvasTestView(a *minesweeperApp) func(c *ui.Context) {
	return func(c *ui.Context) {
		now := c.Now()
		if a.pendingResult.Kind != ActionNoop {
			res := a.pendingResult
			a.pendingResult = ActionResult{}
			a.handleAction(c, res, now)
		}
		if a.game.Status == StatusPlaying {
			if a.startedAt.IsZero() {
				a.startedAt = now
			}
			a.elapsed = int(now.Sub(a.startedAt).Seconds())
			nextTick := a.startedAt.Add(time.Duration(a.elapsed+1) * time.Second).Sub(now)
			c.After(max(16*time.Millisecond, nextTick))
		}

		if c.Shortcut(0, ui.KeyR) {
			a.resetGame(false)
		}
		if c.Shortcut(0, ui.KeyF) {
			a.flagMode = !a.flagMode
		}
		if c.Shortcut(0, ui.KeyE) {
			res := a.game.UseSkill()
			a.handleAction(c, res, now)
		}

		baseTheme := a.activeTheme()
		c.SetTheme(baseTheme)
		t := c.Theme()

		if a.haruhikageIntro {
			elapsedIntro := now.Sub(a.haruhikageIntroStart)
			if elapsedIntro >= haruhikageIntroDuration {
				a.haruhikageIntro = false
				a.haruhikageAlert = true
			} else {
				remain := haruhikageIntroDuration - elapsedIntro
				c.After(min(50*time.Millisecond, max(time.Millisecond, remain)))
				ui.Overlay(c, func() {
					overlay := ui.Box(c.Key("haruhikage-drama-overlay")).Fill().Modal().
						Background(ui.Hex("#05070D").Alpha(0.84)).Center().
						Cursor(ui.CursorPointer).Label("春日影演出遮罩")
					overlay.Children(func() {
						ui.Column(c).AlignItems(ui.Center).Gap(10).Padding(24).Radius(14).
							Children(func() {
								ui.Text(c, "♪ 春日影 (Crychic Ver.) — 演奏中断").FontSize(13).Bold()
								ui.Text(c, "「……为什么要演奏春日影？！」").FontSize(22).Bold()
								ui.Text(c, "（长崎爽世的眼神失去了高光 · 点击任意处继续）").FontSize(12)
							})
					})
					if overlay.Clicked() {
						a.haruhikageIntro = false
						a.haruhikageAlert = true
					}
				})
			}
		}

		switch ui.AlertDialog(c, &a.haruhikageAlert,
			"为什么要演奏春日影？！",
			"「明明都约好了不点那一格的……」\n（长崎爽世失去了高光，并把你移出了排练室群聊）",
			"对不起，我错了", "一辈子重开") {
		case 0:
			a.pushChat("长崎爽世", FindMember(MemberSoyorin).ColorHex, "「道歉的话，下次别再盲猜了呢。」")
		case 1:
			a.resetGame(false)
		}

		ui.Column(c).Fill().Padding(14).Gap(10).Children(func() {
			ui.Row(c).Padding(8, 12).Radius(10).Background(t.Surface).Border(1, t.Border).
				AlignItems(ui.Center).Justify(ui.SpaceBetween).Gap(10).Children(func() {
				ui.ToggleGroup(c, func() {
					ui.Toggle(c, &a.flagMode, "标旗模式 (F)")
					ui.Toggle(c, &a.allowQuestion, "? 标记")
					ui.Toggle(c, &a.showMemes, "名场面图鉴")
				}).Label("工具开关")
			})

			ui.Column(c).Grow(1).Padding(12).Gap(10).Radius(12).Background(t.Surface).Border(1, t.Border).Children(func() {
				ui.Row(c).Padding(8, 12).Radius(9).Background(t.Background.Alpha(0.75)).
					Border(1, t.Border).AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
					ui.Badge(c, "MINES "+formatDigits(a.game.RemainingMines()))
					resetLabel := "迷子出发 (R)"
					switch a.game.Status {
					case StatusPlaying:
						resetLabel = "演奏中 · 重开 (R)"
					case StatusWon:
						resetLabel = "Live 大成功！再来一首"
					case StatusLost:
						resetLabel = "炸团了 · 再排练一次"
					}
					if ui.Button(c.Key("reset-btn"), resetLabel).Clicked() {
						a.resetGame(false)
					}
					ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
						bestText := "BEST --"
						if best, ok := a.bestTimes[a.game.Difficulty.ID]; ok {
							bestText = fmt.Sprintf("BEST %ds", best)
						}
						ui.Badge(c, bestText)
						ui.Badge(c, "TIME "+formatDigits(a.elapsed)+"s")
					})
				})

				a.classicBoard(c)
			})
		})
	}
}

type classicParityHarness struct {
	t            *testing.T
	widgetApp    *minesweeperApp
	canvasApp    *minesweeperApp
	widgetTester *ui.Tester
	canvasTester *ui.Tester
}

func newClassicParityHarness(t *testing.T, diff Difficulty, bandMode bool, member MemberID, seed uint64) *classicParityHarness {
	return newClassicParityHarnessWithSetup(t, diff, bandMode, member, seed, nil)
}

func newClassicParityHarnessWithSetup(t *testing.T, diff Difficulty, bandMode bool, member MemberID, seed uint64, setup func(*minesweeperApp)) *classicParityHarness {
	t.Helper()
	wApp := newMinesweeperApp(seed)
	wApp.modeIdx = 1
	if bandMode {
		wApp.modeIdx = 0
	}
	wApp.memberID = member
	wApp.game = NewGame(diff, bandMode, member, seed)
	if setup != nil {
		setup(wApp)
	}

	cApp := newMinesweeperApp(seed)
	cApp.modeIdx = wApp.modeIdx
	cApp.memberID = member
	cApp.game = NewGame(diff, bandMode, member, seed)
	if setup != nil {
		setup(cApp)
	}

	wTT := ui.NewTester(wApp.view, 1280, 900)
	cTT := ui.NewTester(classicCanvasTestView(cApp), 1280, 900)

	h := &classicParityHarness{
		t:            t,
		widgetApp:    wApp,
		canvasApp:    cApp,
		widgetTester: wTT,
		canvasTester: cTT,
	}
	h.assertEqualState("initial")
	return h
}

func (h *classicParityHarness) canvasCellScreenPos(row, col int) (float32, float32) {
	h.t.Helper()
	r, ok := h.canvasTester.Find("经典棋盘")
	if !ok {
		h.t.Fatal("classic canvas board not found")
	}
	b := h.canvasApp.board
	if row < 0 || row >= h.canvasApp.game.Rows || col < 0 || col >= h.canvasApp.game.Cols {
		h.t.Fatalf("cell (%d,%d) out of bounds", row, col)
	}
	// Pan viewport if target cell is not currently visible or is occluded by the minimap.
	lx := (float32(col)+0.5)*b.Cell - b.X
	ly := (float32(row)+0.5)*b.Cell - b.Y
	if lx < 4 || lx >= r.W-4 || ly < 4 || ly >= r.H-4 || b.minimapFor(h.canvasApp.game, lx, ly) {
		b.center(h.canvasApp.game, row, col)
		h.canvasTester.Frame()
		lx = (float32(col)+0.5)*b.Cell - b.X
		ly = (float32(row)+0.5)*b.Cell - b.Y
	}
	return r.X + lx, r.Y + ly
}

func (h *classicParityHarness) leftClickCell(row, col int, step string) {
	h.t.Helper()
	if err := h.widgetTester.Click(fmt.Sprintf("cell-%d-%d", row, col)); err != nil {
		h.t.Fatalf("%s widget left-click (%d,%d) failed: %v", step, row, col, err)
	}
	x, y := h.canvasCellScreenPos(row, col)
	h.canvasTester.ClickAt(x, y)
	h.assertEqualState(step)
}

func (h *classicParityHarness) rightClickCell(row, col int, step string) {
	h.t.Helper()
	if err := h.widgetTester.RightClick(fmt.Sprintf("cell-%d-%d", row, col)); err != nil {
		h.t.Fatalf("%s widget right-click (%d,%d) failed: %v", step, row, col, err)
	}
	x, y := h.canvasCellScreenPos(row, col)
	h.canvasTester.RightClickAt(x, y)
	h.assertEqualState(step)
}

func (h *classicParityHarness) clickButton(label string, step string) {
	h.t.Helper()
	if err := h.widgetTester.Click(label); err != nil {
		h.t.Fatalf("%s widget click %q failed: %v", step, label, err)
	}
	if err := h.canvasTester.Click(label); err != nil {
		h.t.Fatalf("%s canvas click %q failed: %v", step, label, err)
	}
	h.assertEqualState(step)
}

func (h *classicParityHarness) key(mods ui.Modifiers, k ui.Key, step string) {
	h.t.Helper()
	h.widgetTester.Key(mods, k)
	h.canvasTester.Key(mods, k)
	h.assertEqualState(step)
}

func (h *classicParityHarness) advance(d time.Duration, step string) {
	h.t.Helper()
	h.widgetTester.Advance(d)
	h.canvasTester.Advance(d)
	h.assertEqualState(step)
}

func (h *classicParityHarness) assertEqualState(step string) {
	h.t.Helper()
	wg, cg := h.widgetApp.game, h.canvasApp.game
	if wg.Status != cg.Status ||
		wg.FirstClick != cg.FirstClick ||
		wg.Moves != cg.Moves ||
		wg.FlagsPlaced != cg.FlagsPlaced ||
		wg.RevealedSafe != cg.RevealedSafe ||
		wg.SkillUsesLeft != cg.SkillUsesLeft ||
		wg.Gravity != cg.Gravity {
		h.t.Fatalf("[%s] game state mismatch:\n  widget: status=%v first=%v moves=%d flags=%d safe=%d skill=%d grav=%d\n  canvas: status=%v first=%v moves=%d flags=%d safe=%d skill=%d grav=%d",
			step,
			wg.Status, wg.FirstClick, wg.Moves, wg.FlagsPlaced, wg.RevealedSafe, wg.SkillUsesLeft, wg.Gravity,
			cg.Status, cg.FirstClick, cg.Moves, cg.FlagsPlaced, cg.RevealedSafe, cg.SkillUsesLeft, cg.Gravity)
	}
	if h.widgetApp.flagMode != h.canvasApp.flagMode ||
		h.widgetApp.allowQuestion != h.canvasApp.allowQuestion ||
		h.widgetApp.elapsed != h.canvasApp.elapsed ||
		h.widgetApp.haruhikageIntro != h.canvasApp.haruhikageIntro ||
		h.widgetApp.haruhikageAlert != h.canvasApp.haruhikageAlert {
		h.t.Fatalf("[%s] app state mismatch: flag=(%v,%v) q=(%v,%v) elapsed=(%d,%d) intro=(%v,%v) alert=(%v,%v)",
			step,
			h.widgetApp.flagMode, h.canvasApp.flagMode,
			h.widgetApp.allowQuestion, h.canvasApp.allowQuestion,
			h.widgetApp.elapsed, h.canvasApp.elapsed,
			h.widgetApp.haruhikageIntro, h.canvasApp.haruhikageIntro,
			h.widgetApp.haruhikageAlert, h.canvasApp.haruhikageAlert)
	}
	if len(wg.Cells) != len(cg.Cells) {
		h.t.Fatalf("[%s] cell count mismatch: %d vs %d", step, len(wg.Cells), len(cg.Cells))
	}
	for i := range wg.Cells {
		wc, cc := wg.Cells[i], cg.Cells[i]
		if wc.Revealed != cc.Revealed ||
			wc.Flagged != cc.Flagged ||
			wc.Questioned != cc.Questioned ||
			wc.Exploded != cc.Exploded ||
			wc.WrongFlag != cc.WrongFlag ||
			wc.ShieldSaved != cc.ShieldSaved ||
			wc.IsMine != cc.IsMine ||
			wc.Adjacent != cc.Adjacent {
			h.t.Fatalf("[%s] cell(%d,%d) mismatch:\n  widget=%+v\n  canvas=%+v", step, wc.Row, wc.Col, wc, cc)
		}
	}
	minesBadge := "MINES " + formatDigits(wg.RemainingMines())
	if !h.widgetTester.HasText(minesBadge) || !h.canvasTester.HasText(minesBadge) {
		h.t.Fatalf("[%s] expected %q on both views (widget=%v canvas=%v)",
			step, minesBadge, h.widgetTester.HasText(minesBadge), h.canvasTester.HasText(minesBadge))
	}
	timeBadge := "TIME " + formatDigits(h.widgetApp.elapsed) + "s"
	if !h.widgetTester.HasText(timeBadge) || !h.canvasTester.HasText(timeBadge) {
		h.t.Fatalf("[%s] expected %q on both views (widget=%v canvas=%v)",
			step, timeBadge, h.widgetTester.HasText(timeBadge), h.canvasTester.HasText(timeBadge))
	}
	if len(h.widgetApp.unlockedMemes) != len(h.canvasApp.unlockedMemes) {
		h.t.Fatalf("[%s] unlocked memes mismatch: %v vs %v", step, h.widgetApp.unlockedMemes, h.canvasApp.unlockedMemes)
	}
	for k := range h.widgetApp.unlockedMemes {
		if !h.canvasApp.unlockedMemes[k] {
			h.t.Fatalf("[%s] missing unlocked meme %q on canvas", step, k)
		}
	}
}

func TestClassicCanvasRevealFlagQuestionAndFlagModeParity(t *testing.T) {
	h := newClassicParityHarness(t, Difficulties[0], false, MemberTomori, 12345)

	// Right-click before first click places a flag while staying StatusIdle.
	h.rightClickCell(0, 0, "idle-flag")
	if h.canvasApp.game.Status != StatusIdle || h.canvasApp.game.FlagsPlaced != 1 {
		t.Fatalf("expected StatusIdle with 1 flag, got status=%v flags=%d", h.canvasApp.game.Status, h.canvasApp.game.FlagsPlaced)
	}

	// Left-clicking a flagged cell must be a no-op.
	h.leftClickCell(0, 0, "left-click-flagged-noop")
	if h.canvasApp.game.FirstClick || h.canvasApp.game.Cell(0, 0).Revealed {
		t.Fatal("left-clicking flagged cell revealed it")
	}

	// Right-click with allowQuestion=false unflags back to blank.
	h.rightClickCell(0, 0, "right-unflag-no-question")
	if h.canvasApp.game.Cell(0, 0).Flagged || h.canvasApp.game.Cell(0, 0).Questioned {
		t.Fatal("expected unflagged blank cell")
	}

	// Enable ? 标记 and verify full unmarked -> flag -> ? -> unmarked cycle on right-click.
	h.clickButton("? 标记", "enable-question")
	h.rightClickCell(0, 1, "q-cycle-flag")
	if !h.canvasApp.game.Cell(0, 1).Flagged || h.canvasApp.game.Cell(0, 1).Questioned {
		t.Fatal("expected Flagged on step 1")
	}
	h.rightClickCell(0, 1, "q-cycle-question")
	if h.canvasApp.game.Cell(0, 1).Flagged || !h.canvasApp.game.Cell(0, 1).Questioned || h.canvasApp.game.FlagsPlaced != 0 {
		t.Fatal("expected Questioned with 0 flags on step 2")
	}
	h.rightClickCell(0, 1, "q-cycle-clear")
	if h.canvasApp.game.Cell(0, 1).Flagged || h.canvasApp.game.Cell(0, 1).Questioned {
		t.Fatal("expected cleared blank cell on step 3")
	}

	// Questioned cell can be revealed directly by left-click and clears Questioned.
	h.rightClickCell(4, 4, "q-before-reveal-1")
	h.rightClickCell(4, 4, "q-before-reveal-2")
	if !h.canvasApp.game.Cell(4, 4).Questioned {
		t.Fatal("expected (4,4) questioned")
	}
	h.leftClickCell(4, 4, "left-reveal-questioned-first-click")
	if !h.canvasApp.game.Cell(4, 4).Revealed || h.canvasApp.game.Cell(4, 4).Questioned || h.canvasApp.game.Status != StatusPlaying {
		t.Fatal("expected first-click reveal on questioned cell")
	}

	// Advance clock and check TIME badge parity.
	h.advance(2200*time.Millisecond, "advance-clock")

	// Right-click on already revealed cell is a no-op.
	h.rightClickCell(4, 4, "right-click-revealed-noop")

	// Flag mode via F shortcut: left-click cycles flag -> ? -> clear when allowQuestion=true.
	h.key(0, ui.KeyF, "flag-mode-on-key")
	// Pick an unrevealed cell deterministically.
	var targetRow, targetCol int
	for i := range h.canvasApp.game.Cells {
		c := &h.canvasApp.game.Cells[i]
		if !c.Revealed {
			targetRow, targetCol = c.Row, c.Col
			break
		}
	}
	h.leftClickCell(targetRow, targetCol, "flagmode-q-1")
	if !h.canvasApp.game.Cell(targetRow, targetCol).Flagged {
		t.Fatal("expected flag in flagMode")
	}
	h.leftClickCell(targetRow, targetCol, "flagmode-q-2")
	if !h.canvasApp.game.Cell(targetRow, targetCol).Questioned {
		t.Fatal("expected ? in flagMode")
	}
	h.leftClickCell(targetRow, targetCol, "flagmode-q-3")
	if h.canvasApp.game.Cell(targetRow, targetCol).Flagged || h.canvasApp.game.Cell(targetRow, targetCol).Questioned {
		t.Fatal("expected cleared cell in flagMode")
	}

	// Disable ? 标记 and toggle flagMode off/on via toolbar button.
	h.clickButton("? 标记", "disable-question")
	h.leftClickCell(targetRow, targetCol, "flagmode-noq-flag")
	if !h.canvasApp.game.Cell(targetRow, targetCol).Flagged {
		t.Fatal("expected flag without question")
	}
	// Right-click while flagMode is true still toggles flag cleanly.
	h.rightClickCell(targetRow, targetCol, "rightclick-while-flagmode")
	if h.canvasApp.game.Cell(targetRow, targetCol).Flagged {
		t.Fatal("expected right-click to unflag while flagMode=true")
	}
	h.clickButton("标旗模式 (F)", "flag-mode-off-btn")
}

func TestClassicCanvasChordParity(t *testing.T) {
	setupChordBoard := func(app *minesweeperApp) {
		g := NewGame(Difficulty{ID: "easy", Rows: 5, Cols: 5, Mines: 2}, false, MemberAnon, 7)
		g.FirstClick = true
		g.Status = StatusPlaying
		g.Cell(0, 0).IsMine = true
		g.Cell(4, 4).IsMine = true
		g.RecomputeAdjacency()
		g.Cell(1, 1).Revealed = true
		g.Cell(3, 3).Revealed = true
		g.RevealedSafe = 2
		app.game = g
	}

	h := newClassicParityHarnessWithSetup(t, Difficulties[0], false, MemberAnon, 7, setupChordBoard)

	// Setup (0,1) with 1 unflagged neighbor (0,0) and (3,3) with 2 unrevealed safe neighbors.
	revealAllBut00And44 := func(app *minesweeperApp) {
		g := app.game
		for i := range g.Cells {
			c := &g.Cells[i]
			if (c.Row == 0 && c.Col == 0) || (c.Row == 4 && c.Col == 4) {
				continue
			}
			if !c.Revealed {
				c.Revealed = true
				g.RevealedSafe++
			}
		}
		// Hide two safe neighbors of (3,3) so we can test chord after flagging (4,4).
		g.Cell(3, 4).Revealed = false
		g.Cell(4, 3).Revealed = false
		g.RevealedSafe -= 2
	}
	revealAllBut00And44(h.widgetApp)
	revealAllBut00And44(h.canvasApp)
	h.widgetTester.Frame()
	h.canvasTester.Frame()
	h.assertEqualState("single-unknown-mine-setup")

	// (0,1) has Adjacent=1 with 0 flags and 1 unrevealed neighbor (0,0): clicking it in Classic is a no-op.
	h.leftClickCell(0, 1, "classic-single-unknown-number-click-noop")
	if h.canvasApp.game.Cell(0, 0).Flagged || h.canvasApp.game.Cell(0, 0).Revealed {
		t.Fatal("clicking unflagged classic number cell altered neighbor state")
	}

	// Unsatisfied chord on (3,3) (0 flags, Adjacent=1) -> no-op.
	h.leftClickCell(3, 3, "unsatisfied-chord-noop")
	if h.canvasApp.game.Cell(3, 4).Revealed || h.canvasApp.game.Cell(4, 3).Revealed {
		t.Fatal("unsatisfied chord revealed neighbors")
	}

	// Flag (4,4), turn flagMode ON, click (3,3) -> MUST NOT chord while flagMode=true!
	h.rightClickCell(4, 4, "flag-4-4")
	h.key(0, ui.KeyF, "flagmode-blocks-chord-on")
	h.leftClickCell(3, 3, "click-number-in-flagmode-noop")
	if h.canvasApp.game.Cell(3, 4).Revealed || h.canvasApp.game.Cell(4, 3).Revealed {
		t.Fatal("chord fired while flagMode=true")
	}

	// Turn flagMode OFF and click (3,3) -> chords (3,4) and (4,3) and wins the game!
	h.key(0, ui.KeyF, "flagmode-off-before-chord")
	h.leftClickCell(3, 3, "satisfied-chord-wins")
	if h.canvasApp.game.Status != StatusWon || !h.canvasApp.game.Cell(3, 4).Revealed || !h.canvasApp.game.Cell(4, 3).Revealed {
		t.Fatalf("expected chord win, status=%v (3,4)=%v (4,3)=%v",
			h.canvasApp.game.Status, h.canvasApp.game.Cell(3, 4).Revealed, h.canvasApp.game.Cell(4, 3).Revealed)
	}
	if !h.canvasTester.HasText("Live 大成功！再来一首") {
		t.Fatal("expected win button label on classic canvas")
	}

	// Right-click after win must be a no-op.
	h.rightClickCell(0, 0, "right-click-after-win-noop")
}

func TestClassicCanvasWrongFlagChordLossSoyorinShieldAndHaruhikageParity(t *testing.T) {
	// Case 1: Wrong flag + chord triggers Soyorin shield, then second mine triggers Haruhikage flow.
	setupHaruhikageFixture := func(app *minesweeperApp) {
		g := NewGame(Difficulty{
			ID:           "hard",
			Label:        "春日影 Live",
			Song:         "春日影 (Crychic Ver.)",
			Gameplay:     GameplayClassic,
			Haruhikage:   true,
			Rows:         6,
			Cols:         6,
			Mines:        2,
			SkillCharges: 1,
		}, true, MemberSoyorin, 7)
		g.FirstClick = true
		g.Status = StatusPlaying
		g.Cell(0, 0).IsMine = true
		g.Cell(5, 5).IsMine = true
		g.RecomputeAdjacency()
		g.Cell(1, 1).Revealed = true
		g.RevealedSafe = 1
		app.game = g
	}

	h := newClassicParityHarnessWithSetup(t, Difficulties[2], true, MemberSoyorin, 7, setupHaruhikageFixture)

	// Misflag safe (0,1) instead of mine (0,0), then chord on (1,1) -> hits (0,0) mine -> Soyorin shield!
	h.rightClickCell(0, 1, "misflag-0-1")
	h.leftClickCell(1, 1, "wrong-flag-chord-triggers-shield")
	if !h.canvasApp.game.Cell(0, 0).ShieldSaved || h.canvasApp.game.Status != StatusPlaying || h.canvasApp.game.SkillUsesLeft != 0 {
		t.Fatalf("expected Soyorin shield on wrong-flag chord, status=%v shield=%v uses=%d",
			h.canvasApp.game.Status, h.canvasApp.game.Cell(0, 0).ShieldSaved, h.canvasApp.game.SkillUsesLeft)
	}
	if !h.canvasTester.HasText("长崎爽世发动了「温柔的微笑」：这次就当没点过吧！") {
		t.Fatal("expected Soyorin shield toast on classic canvas")
	}

	// Hit second mine (5,5) with shield exhausted -> Haruhikage loss + drama overlay + WrongFlag on (0,1)!
	h.leftClickCell(5, 5, "haruhikage-second-mine-loss")
	if h.canvasApp.game.Status != StatusLost || !h.canvasApp.game.Cell(0, 1).WrongFlag || !h.canvasApp.haruhikageIntro {
		t.Fatalf("expected Haruhikage loss with WrongFlag and drama overlay, status=%v wrong=%v intro=%v",
			h.canvasApp.game.Status, h.canvasApp.game.Cell(0, 1).WrongFlag, h.canvasApp.haruhikageIntro)
	}
	if !h.canvasTester.HasText("……为什么要演奏春日影？！") {
		t.Fatal("expected Haruhikage overlay text on classic canvas")
	}

	// Skip overlay -> AlertDialog -> dismiss keeping StatusLost.
	h.clickButton("春日影演出遮罩", "haruhikage-skip-overlay")
	if !h.canvasApp.haruhikageAlert || !h.canvasTester.HasText("为什么要演奏春日影？！") {
		t.Fatal("expected Haruhikage dialog after skipping overlay")
	}
	h.clickButton("对不起，我错了", "haruhikage-dismiss-dialog")
	if h.canvasApp.haruhikageAlert || h.canvasApp.game.Status != StatusLost {
		t.Fatal("expected lost board preserved after dismissing dialog")
	}
	if !h.canvasTester.HasText("炸团了 · 再排练一次") {
		t.Fatal("expected lost reset button label on classic canvas")
	}

	// Operations after loss are no-ops.
	h.leftClickCell(2, 2, "left-click-after-loss-noop")
	h.rightClickCell(2, 2, "right-click-after-loss-noop")
}

func TestClassicCanvasPanZoomMinimapAndBoundsSafety(t *testing.T) {
	app := newMinesweeperApp(42)
	app.game = NewGame(Difficulty{
		ID:       "large-classic",
		Label:    "迷路日々",
		Song:     "迷路日々",
		Gameplay: GameplayClassic,
		Rows:     30,
		Cols:     40,
		Mines:    120,
	}, false, MemberAnon, 42)
	// Bypass NewGame's 30x40 clamp to also verify 80x80 classic Game shapes work seamlessly.
	app.game.Rows, app.game.Cols, app.game.Mines = 80, 80, 1200
	app.game.Cells = make([]Cell, 80*80)
	for r := range 80 {
		for c := range 80 {
			app.game.Cells[r*80+c] = Cell{Row: r, Col: c}
		}
	}

	tt := ui.NewTester(classicCanvasTestView(app), 1280, 900)
	tt.Frame()

	boardRect, ok := tt.Find("经典棋盘")
	if !ok {
		t.Fatal("classic canvas board missing")
	}
	if _, ok := tt.Find("百万格决斗战场"); ok {
		t.Fatal("classic canvas must not use duel label")
	}
	if tt.HasText("踩雷硬直中") {
		t.Fatal("classic canvas must not show duel stun banner")
	}

	// Minimap interaction checks while every cell beneath the minimap is guaranteed in-bounds & unrevealed.
	mini := app.board.minimapBoundsFor(app.game)
	underRow, underCol, underOK := app.board.cellAt(app.game, mini.X+mini.W/2, mini.Y+mini.H/2)
	if !underOK || app.game.Cell(underRow, underCol).Revealed {
		t.Fatalf("precondition failed: minimap center cell (%d,%d) ok=%v revealed=%v",
			underRow, underCol, underOK, underOK && app.game.Cell(underRow, underCol).Revealed)
	}
	tt.RightClickAt(boardRect.X+mini.X+mini.W/2, boardRect.Y+mini.Y+mini.H/2)
	if app.game.FlagsPlaced != 0 || app.game.Cell(underRow, underCol).Flagged {
		t.Fatal("right-click on minimap leaked to unrevealed cell underneath")
	}

	// Right-press inside minimap border and release 3px outside (<5px drag threshold): must NOT flag adjacent cell.
	adjRow, adjCol, adjOK := app.board.cellAt(app.game, mini.X-2, mini.Y+1)
	if !adjOK || app.game.Cell(adjRow, adjCol).Revealed {
		t.Fatalf("precondition failed: adjacent cell outside minimap (%d,%d) ok=%v", adjRow, adjCol, adjOK)
	}
	tt.PressButton(1, boardRect.X+mini.X+1, boardRect.Y+mini.Y+1)
	tt.Move(boardRect.X+mini.X-2, boardRect.Y+mini.Y+1)
	tt.ReleaseButton(1, boardRect.X+mini.X-2, boardRect.Y+mini.Y+1)
	if app.game.FlagsPlaced != 0 || app.game.Cell(adjRow, adjCol).Flagged {
		t.Fatal("right-click pressed inside minimap and released 3px outside leaked to adjacent cell")
	}

	// Left-click on minimap in normal mode jumps viewport without revealing cells underneath.
	tt.ClickAt(boardRect.X+mini.X+2, boardRect.Y+mini.Y+2)
	if app.board.X != 0 || app.board.Y != 0 {
		t.Fatalf("minimap top-left click did not jump to (0,0): (%v,%v)", app.board.X, app.board.Y)
	}
	if app.game.FirstClick || app.game.Moves != 0 || app.game.RevealedSafe != 0 {
		t.Fatal("left-click on minimap leaked reveal")
	}

	// Left-click on minimap in flagMode jumps viewport without flagging cells underneath.
	tt.Key(0, ui.KeyF)
	if !app.flagMode {
		t.Fatal("expected flagMode=true")
	}
	tt.ClickAt(boardRect.X+mini.X+mini.W-2, boardRect.Y+mini.Y+mini.H-2)
	wantMaxX := max(float32(0), float32(app.game.Cols)*app.board.Cell-app.board.Width)
	wantMaxY := max(float32(0), float32(app.game.Rows)*app.board.Cell-app.board.Height)
	if abs32(app.board.X-wantMaxX) > 0.5 || abs32(app.board.Y-wantMaxY) > 0.5 {
		t.Fatalf("minimap bottom-right click in flagMode did not jump to max (%v,%v): got (%v,%v)",
			wantMaxX, wantMaxY, app.board.X, app.board.Y)
	}
	if app.game.FlagsPlaced != 0 || app.game.FirstClick {
		t.Fatal("left-click on minimap in flagMode leaked flag/reveal")
	}
	tt.Key(0, ui.KeyF)
	app.board.centerBoard(app.game)
	tt.Frame()

	// Dragging > 5px pans viewport without triggering reveal or flag.
	beforeX, beforeY := app.board.X, app.board.Y
	sx, sy := boardRect.X+boardRect.W*0.4, boardRect.Y+boardRect.H*0.4
	tt.Press(sx, sy)
	tt.Move(sx+60, sy+45)
	tt.Release(sx+60, sy+45)
	if app.board.X != beforeX-60 || app.board.Y != beforeY-45 {
		t.Fatalf("drag pan mismatch: got (%v,%v), want (%v,%v)", app.board.X, app.board.Y, beforeX-60, beforeY-45)
	}
	if app.game.FirstClick || app.game.Moves != 0 || app.game.FlagsPlaced != 0 {
		t.Fatal("drag pan triggered unintended cell action")
	}

	// Zoom via scroll, then click center of viewport -> opens exact cell under pointer.
	beforeCell := app.board.Cell
	tt.Scroll(boardRect.X+boardRect.W/2, boardRect.Y+boardRect.H/2, 0, -240)
	if app.board.Cell == beforeCell {
		t.Fatal("scroll did not zoom classic canvas")
	}
	targetRow, targetCol, ok := app.board.cellAt(app.game, boardRect.W/2, boardRect.H/2)
	if !ok {
		t.Fatal("center cellAt failed after zoom")
	}
	tt.ClickAt(boardRect.X+boardRect.W/2, boardRect.Y+boardRect.H/2)
	if !app.game.FirstClick || !app.game.Cell(targetRow, targetCol).Revealed {
		t.Fatalf("click after pan+zoom missed target (%d,%d)", targetRow, targetCol)
	}

	// Small 9x9 board: clicking empty space outside board cells is a safe no-op.
	appSmall := newMinesweeperApp(42)
	ttSmall := ui.NewTester(classicCanvasTestView(appSmall), 1280, 900)
	ttSmall.Frame()
	smallRect, ok := ttSmall.Find("经典棋盘")
	if !ok {
		t.Fatal("small classic canvas missing")
	}
	outsideX := smallRect.X + float32(appSmall.game.Cols)*appSmall.board.Cell + 40
	outsideY := smallRect.Y + 40
	ttSmall.ClickAt(outsideX, outsideY)
	ttSmall.RightClickAt(outsideX, outsideY)
	if appSmall.game.FirstClick || appSmall.game.FlagsPlaced != 0 || appSmall.game.Moves != 0 {
		t.Fatal("clicking outside 9x9 board cells mutated game state")
	}
}
