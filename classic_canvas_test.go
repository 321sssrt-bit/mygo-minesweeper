package main

import (
	"fmt"
	"image"
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

func classicCanvasCellRect(board ui.Rect, v *boardView, row, col int) ui.Rect {
	x := board.X + float32(col)*v.Cell - v.X
	y := board.Y + float32(row)*v.Cell - v.Y
	return ui.Rect{X: x + 1, Y: y + 1, W: v.Cell - 2, H: v.Cell - 2}
}

func countMatchingPixels(img *image.RGBA, r ui.Rect, scale float32, want ui.Color, tol int) int {
	x0, y0 := int(r.X*scale), int(r.Y*scale)
	x1, y1 := int((r.X+r.W)*scale), int((r.Y+r.H)*scale)
	count := 0
	for py := y0; py < y1; py++ {
		for px := x0; px < x1; px++ {
			got := img.RGBAAt(px, py)
			if absInt(int(got.R)-int(want.R)) <= tol &&
				absInt(int(got.G)-int(want.G)) <= tol &&
				absInt(int(got.B)-int(want.B)) <= tol {
				count++
			}
		}
	}
	return count
}

func stageClassicCanvasAllStatesBoard(dark bool) (*minesweeperApp, map[string][2]int) {
	app := newMinesweeperApp(7)
	app.darkMode = dark
	app.memberID = MemberSoyorin
	app.allowQuestion = true
	app.game = NewGame(Difficulty{
		ID:           "classic-paper-test",
		Label:        "经典画布测试",
		Song:         "迷星叫",
		Gameplay:     GameplayClassic,
		Rows:         6,
		Cols:         6,
		Mines:        4,
		SkillCharges: 1,
	}, true, MemberSoyorin, 7)
	g := app.game
	g.FirstClick = true
	g.Status = StatusPlaying
	// Mines at (0,0), (0,2), (0,4), (5,5): keeps bottom-left area open and
	// lets us stage every single classic visual state via public Game API.
	g.Cell(0, 0).IsMine = true // right flag (art.Flag)
	g.Cell(0, 2).IsMine = true // Soyorin shield (art.Shield)
	g.Cell(0, 4).IsMine = true // unhit mine revealed on loss (art.Knot)
	g.Cell(5, 5).IsMine = true // hit mine triggering loss (art.KnotHit)
	g.RecomputeAdjacency()

	// Reveal safe empty (3,0) -> flood-fills open empty ground and numbered border.
	g.Reveal(3, 0)
	// Flag mine (0,0) -> normal band-aid.
	g.ToggleFlag(0, 0, false)
	// Trigger Soyorin shield on mine (0,2) -> golden shield band-aid.
	g.Reveal(0, 2)
	// Misflag safe (0,1) -> becomes WrongFlag on loss.
	g.ToggleFlag(0, 1, false)
	// Mark safe (0,3) with ? -> stays Questioned on loss.
	g.ToggleFlag(0, 3, true)
	g.ToggleFlag(0, 3, true)
	// Leave safe (0,5) unrevealed as a plain paper slip.
	// Step on mine (5,5) with shield spent -> StatusLost, revealing (0,4) and (5,5).
	g.Reveal(5, 5)

	coords := map[string][2]int{
		"flag":     {0, 0},
		"wrong":    {0, 1},
		"shield":   {0, 2},
		"question": {0, 3},
		"knot":     {0, 4},
		"slip":     {0, 5},
		"number":   {1, 0},
		"empty":    {3, 0},
		"hit":      {5, 5},
	}
	return app, coords
}

// Classic cells drawn on the shared canvas reuse the notebook paper style in
// both day and night themes, covering every classic cell state without picking
// up duel territory washes or duel flag outlines.
func TestClassicCanvasPaintsPaperCellsDayAndNight(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(fmt.Sprintf("dark=%v", dark), func(t *testing.T) {
			app, coords := stageClassicCanvasAllStatesBoard(dark)
			if app.game.Status != StatusLost {
				t.Fatalf("expected StatusLost, got %v", app.game.Status)
			}
			tt := ui.NewTester(classicCanvasTestView(app), 1280, 900)
			tt.SetScale(2)
			tt.Frame()

			board, ok := tt.Find("经典棋盘")
			if !ok {
				t.Fatal("classic canvas board missing")
			}
			img := tt.Image()
			pal := paperFor(dark)
			v := app.board

			sampleCenter := func(name string) ui.Color {
				at := coords[name]
				tile := classicCanvasCellRect(board, v, at[0], at[1])
				px := img.RGBAAt(int((tile.X+tile.W/2)*2), int((tile.Y+tile.H/2)*2))
				return ui.Color{R: px.R, G: px.G, B: px.B, A: px.A}
			}

			// Plain paper slip paints pal.Slip; bare empty revealed cell paints pal.Board.
			if got := sampleCenter("slip"); !near(got.R, pal.Slip.R) || !near(got.G, pal.Slip.G) || !near(got.B, pal.Slip.B) {
				t.Errorf("slip center painted %s, want %s", hexOf(got), hexOf(pal.Slip))
			}
			if got := sampleCenter("empty"); !near(got.R, pal.Board.R) || !near(got.G, pal.Board.G) || !near(got.B, pal.Board.B) {
				t.Errorf("empty revealed cell painted %s, want %s", hexOf(got), hexOf(pal.Board))
			}

			// Number cell uses bare paper + classic numberInk (no duel territory wash).
			numAt := coords["number"]
			numCell := app.game.Cell(numAt[0], numAt[1])
			if !numCell.Revealed || numCell.Adjacent <= 0 {
				t.Fatalf("expected revealed number at %v, got %+v", numAt, numCell)
			}
			numTile := classicCanvasCellRect(board, v, numAt[0], numAt[1])
			numInk := numberInk(numCell.Adjacent, dark)
			if got := countMatchingPixels(img, numTile, 2, numInk, 18); got < 12 {
				t.Errorf("number %d cell had only %d pixels near ink %s", numCell.Adjacent, got, hexOf(numInk))
			}
			if corner := img.RGBAAt(int((numTile.X+2)*2), int((numTile.Y+2)*2)); !near(corner.R, pal.Board.R) || !near(corner.G, pal.Board.G) || !near(corner.B, pal.Board.B) {
				t.Errorf("number cell background painted #%02X%02X%02X, want bare paper %s", corner.R, corner.G, corner.B, hexOf(pal.Board))
			}

			// Normal classic flag draws band-aid without duel owner outline.
			flagTile := classicCanvasCellRect(board, v, coords["flag"][0], coords["flag"][1])
			if got := countMatchingPixels(img, flagTile, 2, pal.BandAid, 18); got < 20 {
				t.Errorf("normal flag had only %d pixels near band-aid %s", got, hexOf(pal.BandAid))
			}
			if duelBorder := countMatchingPixels(img, flagTile, 2, duelOwnerColor(DuelPlayer), 16); duelBorder > 0 {
				t.Errorf("classic flag picked up %d duel player outline pixels", duelBorder)
			}

			// Soyorin shield flag draws golden shield band-aid without duel owner outline.
			shieldTile := classicCanvasCellRect(board, v, coords["shield"][0], coords["shield"][1])
			soyoGold := memberImage(MemberSoyorin)
			if got := countMatchingPixels(img, shieldTile, 2, soyoGold, 18); got < 20 {
				t.Errorf("shield flag had only %d pixels near Soyorin gold %s", got, hexOf(soyoGold))
			}
			if duelBorder := countMatchingPixels(img, shieldTile, 2, duelOwnerColor(DuelPlayer), 16); duelBorder > 0 {
				t.Errorf("classic shield flag picked up %d duel player outline pixels", duelBorder)
			}

			// Wrong flag draws red cross ink over the band-aid.
			wrongTile := classicCanvasCellRect(board, v, coords["wrong"][0], coords["wrong"][1])
			if got := countMatchingPixels(img, wrongTile, 2, pal.BoomInk, 24); got < 16 {
				t.Errorf("wrong flag had only %d pixels near boom ink %s", got, hexOf(pal.BoomInk))
			}

			// ? mark draws a graphite ? glyph on the paper slip (distinct from blank slip).
			qTile := classicCanvasCellRect(board, v, coords["question"][0], coords["question"][1])
			if got := countMatchingPixels(img, qTile, 2, pal.Graphite, 24); got < 12 {
				t.Errorf("questioned cell had only %d pixels near graphite %s", got, hexOf(pal.Graphite))
			}
			slipTile := classicCanvasCellRect(board, v, coords["slip"][0], coords["slip"][1])
			if got := countMatchingPixels(img, slipTile, 2, pal.Graphite, 24); got != 0 {
				t.Errorf("blank slip unexpectedly had %d graphite pixels", got)
			}

			// Revealed unhit mine draws graphite knot on bare paper.
			knotTile := classicCanvasCellRect(board, v, coords["knot"][0], coords["knot"][1])
			if got := countMatchingPixels(img, knotTile, 2, pal.Graphite, 24); got < 20 {
				t.Errorf("unhit mine had only %d knot pixels near %s", got, hexOf(pal.Graphite))
			}

			// Hit exploded mine paints pal.Boom wash over pal.Board + pal.BoomInk knot.
			hitTile := classicCanvasCellRect(board, v, coords["hit"][0], coords["hit"][1])
			boomBg := pal.Boom.Over(pal.Board)
			if corner := img.RGBAAt(int((hitTile.X+2)*2), int((hitTile.Y+2)*2)); !near(corner.R, boomBg.R) || !near(corner.G, boomBg.G) || !near(corner.B, boomBg.B) {
				t.Errorf("hit mine background painted #%02X%02X%02X, want %s", corner.R, corner.G, corner.B, hexOf(boomBg))
			}
			if got := countMatchingPixels(img, hitTile, 2, pal.BoomInk, 24); got < 20 {
				t.Errorf("hit mine had only %d knot pixels near %s", got, hexOf(pal.BoomInk))
			}
		})
	}
}

// Zoomed out below the prop/glyph detail threshold (Cell=10), every classic
// mark and mine keeps a distinct ink dot so flags, shields, wrong flags, ?
// marks, and mines never collapse into blank slips or empty ground.
func TestClassicCanvasZoomedOutMarksStayReadable(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(fmt.Sprintf("dark=%v", dark), func(t *testing.T) {
			app, coords := stageClassicCanvasAllStatesBoard(dark)
			tt := ui.NewTester(classicCanvasTestView(app), 1280, 900)
			tt.SetScale(2)
			tt.Frame()
			app.board.Cell = 10
			app.board.clamp(app.game)
			tt.Frame()

			board, ok := tt.Find("经典棋盘")
			if !ok {
				t.Fatal("classic canvas board missing")
			}
			img := tt.Image()
			pal := paperFor(dark)
			v := app.board

			wantCenter := map[string]ui.Color{
				"slip":     pal.Slip,
				"empty":    pal.Board,
				"flag":     pal.BandAidEdge,
				"shield":   pal.InkOf(memberImage(MemberSoyorin)),
				"wrong":    pal.BoomInk,
				"question": pal.Graphite,
				"knot":     pal.Graphite,
				"hit":      pal.BoomInk,
			}
			for kind, want := range wantCenter {
				at := coords[kind]
				tile := classicCanvasCellRect(board, v, at[0], at[1])
				got := img.RGBAAt(int((tile.X+tile.W/2)*2), int((tile.Y+tile.H/2)*2))
				if !near(got.R, want.R) || !near(got.G, want.G) || !near(got.B, want.B) {
					t.Errorf("zoomed-out %s (%d,%d) center painted #%02X%02X%02X, want %s",
						kind, at[0], at[1], got.R, got.G, got.B, hexOf(want))
				}
			}
		})
	}
}

// Zoomed-out state dots are classic-only: in duel mode, shield flags and
// settled wrong flags keep their side's outline without adding center dots.
func TestDuelZoomedOutShieldAndWrongFlagStayDotFree(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(fmt.Sprintf("dark=%v", dark), func(t *testing.T) {
			app := newMinesweeperApp(7)
			app.darkMode = dark
			app.memberID = MemberSoyorin
			app.diffIdx = 4
			app.game = NewGame(Difficulty{
				ID:           "duel-zoom-test",
				Gameplay:     GameplayDuel,
				Rows:         8,
				Cols:         8,
				Mines:        2,
				SkillCharges: 1,
				DuelDuration: 60,
				AIInterval:   100,
			}, true, MemberSoyorin, 7)
			g := app.game
			g.FirstClick = true
			g.Status = StatusPlaying
			g.Cell(0, 0).IsMine = true
			g.Cell(7, 7).IsMine = true
			g.RecomputeAdjacency()
			// Trigger Soyorin shield on (0,0) and plant a wrong flag on safe (0,1), then settle.
			g.Reveal(0, 0)
			g.ToggleFlag(0, 1, false)
			g.TickDuel(60)
			if !g.Cell(0, 0).ShieldSaved || !g.Cell(0, 1).WrongFlag {
				t.Fatalf("expected duel shield and wrong flag, got shield=%+v wrong=%+v", g.Cell(0, 0), g.Cell(0, 1))
			}

			tt := ui.NewTester(app.view, 1280, 900)
			tt.SetScale(2)
			tt.Frame()
			app.board.Cell = 11
			app.board.clamp(g)
			tt.Frame()

			board, ok := tt.Find("百万格决斗战场")
			if !ok {
				t.Fatal("duel board missing")
			}
			img := tt.Image()
			pal := paperFor(dark)
			v := app.board
			for name, at := range map[string][2]int{"shield": {0, 0}, "wrong": {0, 1}} {
				tile := classicCanvasCellRect(board, v, at[0], at[1])
				got := img.RGBAAt(int((tile.X+tile.W/2)*2), int((tile.Y+tile.H/2)*2))
				if !near(got.R, pal.Slip.R) || !near(got.G, pal.Slip.G) || !near(got.B, pal.Slip.B) {
					t.Errorf("duel zoomed-out %s center painted #%02X%02X%02X, want pure slip %s",
						name, got.R, got.G, got.B, hexOf(pal.Slip))
				}
			}
		})
	}
}

func TestClassicCanvasSnapshotScenesCoverAllStates(t *testing.T) {
	inStageWindow := func(row, col int) bool {
		return row >= classicCanvasStageRow-6 && row <= classicCanvasStageRow+6 &&
			col >= classicCanvasStageCol-8 && col <= classicCanvasStageCol+8
	}

	live := stageClassicCanvasLiveScene(false)
	if live.game.Status != StatusPlaying {
		t.Fatalf("classic-canvas-live status=%v, want StatusPlaying", live.game.Status)
	}
	if !inStageWindow(live.classicHoverRow, live.classicHoverCol) ||
		live.game.InspectNumberHover(live.classicHoverRow, live.classicHoverCol, false).Kind != HintChordable {
		t.Fatalf("classic-canvas-live missing centered chordable hover cell: (%d,%d)", live.classicHoverRow, live.classicHoverCol)
	}
	var liveSlips, liveNums, liveFlags, liveShield, liveQ int
	for i := range live.game.Cells {
		c := &live.game.Cells[i]
		if !inStageWindow(c.Row, c.Col) {
			continue
		}
		switch {
		case c.Revealed && c.Adjacent > 0:
			liveNums++
		case c.Flagged && c.ShieldSaved:
			liveShield++
		case c.Flagged:
			liveFlags++
		case c.Questioned:
			liveQ++
		case !c.Revealed:
			liveSlips++
		}
	}
	if liveSlips == 0 || liveNums == 0 || liveFlags == 0 || liveShield == 0 || liveQ == 0 {
		t.Fatalf("classic-canvas-live missing centered state: slips=%d nums=%d flags=%d shield=%d q=%d",
			liveSlips, liveNums, liveFlags, liveShield, liveQ)
	}

	won := stageClassicCanvasWonScene(false)
	if won.game.Status != StatusWon || won.game.FlagsPlaced != won.game.Mines {
		t.Fatalf("classic-canvas-won invalid: status=%v flags=%d mines=%d", won.game.Status, won.game.FlagsPlaced, won.game.Mines)
	}

	lost := stageClassicCanvasLostScene(false)
	if lost.game.Status != StatusLost {
		t.Fatalf("classic-canvas-lost status=%v, want StatusLost", lost.game.Status)
	}
	var lostSlips, lostNums, lostFlags, lostShield, lostWrong, lostQ, lostKnots, lostHit int
	for i := range lost.game.Cells {
		c := &lost.game.Cells[i]
		if !inStageWindow(c.Row, c.Col) {
			continue
		}
		switch {
		case c.Exploded:
			lostHit++
		case c.Revealed && c.IsMine:
			lostKnots++
		case c.Revealed && c.Adjacent > 0:
			lostNums++
		case c.WrongFlag:
			lostWrong++
		case c.Flagged && c.ShieldSaved:
			lostShield++
		case c.Flagged:
			lostFlags++
		case c.Questioned:
			lostQ++
		case !c.Revealed:
			lostSlips++
		}
	}
	if lostSlips == 0 || lostNums == 0 || lostFlags == 0 || lostShield == 0 || lostWrong == 0 || lostQ == 0 || lostKnots == 0 || lostHit == 0 {
		t.Fatalf("classic-canvas-lost missing centered state: slips=%d nums=%d flags=%d shield=%d wrong=%d q=%d knots=%d hit=%d",
			lostSlips, lostNums, lostFlags, lostShield, lostWrong, lostQ, lostKnots, lostHit)
	}
}

func sampleCanvasPixel(img *image.RGBA, x, y, scale float32) ui.Color {
	px := img.RGBAAt(int(x*scale), int(y*scale))
	return ui.Color{R: px.R, G: px.G, B: px.B, A: px.A}
}

func assertColorNear(t *testing.T, label string, got, want ui.Color) {
	t.Helper()
	if !near(got.R, want.R) || !near(got.G, want.G) || !near(got.B, want.B) {
		t.Errorf("%s painted %s, want %s", label, hexOf(got), hexOf(want))
	}
}

// Classic hover hints on the canvas match the classic cell widgets: soft
// member-colour highlighter fills with no border strokes, subtle ink hover on
// non-chordable numbers, slip lift on hovered slips, bare empty ground, and
// clean flagMode / off-board / end-of-game clearing.
func TestClassicCanvasHoverHighlighterParity(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(fmt.Sprintf("dark=%v", dark), func(t *testing.T) {
			app := newMinesweeperApp(42)
			app.darkMode = dark
			app.memberID = MemberTomori
			app.allowQuestion = true
			app.game = NewGame(Difficulty{
				ID:       "classic-hover-fx",
				Gameplay: GameplayClassic,
				Rows:     7,
				Cols:     7,
				Mines:    2,
			}, true, MemberTomori, 42)
			g := app.game
			g.FirstClick = true
			g.Status = StatusPlaying
			// Mines at (0,0) and (0,5): (1,1) is chordable once (0,0) is flagged;
			// (1,5) stays an unchordable revealed number; (4,4) is revealed empty;
			// (6,0) is a standalone unrevealed slip outside any chord range.
			g.Cell(0, 0).IsMine = true
			g.Cell(0, 5).IsMine = true
			g.RecomputeAdjacency()
			g.Cell(1, 1).Revealed = true
			g.Cell(1, 5).Revealed = true
			g.Cell(4, 4).Revealed = true
			g.RevealedSafe = 3
			g.ToggleFlag(0, 0, false)
			// Mark chord target (0,1) with ? so both blank and questioned targets are covered.
			g.ToggleFlag(0, 1, true)
			g.ToggleFlag(0, 1, true)

			tt := ui.NewTester(classicCanvasTestView(app), 1280, 900)
			tt.SetScale(2)
			tt.Frame()

			board, ok := tt.Find("经典棋盘")
			if !ok {
				t.Fatal("classic canvas board missing")
			}
			pal := paperFor(dark)
			v := app.board

			moveToCell := func(row, col int) ui.Rect {
				tile := classicCanvasCellRect(board, v, row, col)
				tt.Move(tile.X+tile.W/2, tile.Y+tile.H/2)
				tt.Frame()
				return tile
			}

			// 1. Hover chordable (1,1): center uses 0.38 highlighter, targets use 0.42 over slip, neither has a solid outline.
			centerTile := moveToCell(1, 1)
			if v.LastHoverHint.Kind != HintChordable || v.LastHoverHint.Center != 1*g.Cols+1 || len(v.LastHoverHint.Targets) != 7 {
				t.Fatalf("expected HintChordable at (1,1) with 7 targets, got %+v", v.LastHoverHint)
			}
			img := tt.Image()
			hintColor := memberImage(MemberTomori)
			wantCenter := hintColor.Alpha(0.38).Over(pal.Board)
			wantTarget := hintColor.Alpha(0.42).Over(pal.Slip)

			assertColorNear(t, "hint center interior", sampleCanvasPixel(img, centerTile.X+3, centerTile.Y+3, 2), wantCenter)
			// Sampling the top edge ensures no duel-style solid hintColor border stroke was drawn.
			assertColorNear(t, "hint center top edge (no stroke)", sampleCanvasPixel(img, centerTile.X+centerTile.W/2, centerTile.Y+0.5, 2), wantCenter)

			blankTarget := classicCanvasCellRect(board, v, 0, 2)
			assertColorNear(t, "blank target interior", sampleCanvasPixel(img, blankTarget.X+blankTarget.W/2, blankTarget.Y+blankTarget.H/2, 2), wantTarget)
			assertColorNear(t, "blank target top edge (no outline)", sampleCanvasPixel(img, blankTarget.X+blankTarget.W/2, blankTarget.Y+0.5, 2), wantTarget)

			qTarget := classicCanvasCellRect(board, v, 0, 1)
			assertColorNear(t, "questioned target corner", sampleCanvasPixel(img, qTarget.X+3, qTarget.Y+3, 2), wantTarget)

			// 2. Changing leader swaps the highlighter to the new member's 代表色.
			app.memberID = MemberAnon
			tt.Frame()
			img = tt.Image()
			anonColor := memberImage(MemberAnon)
			assertColorNear(t, "anon hint center", sampleCanvasPixel(img, centerTile.X+3, centerTile.Y+3, 2), anonColor.Alpha(0.38).Over(pal.Board))
			assertColorNear(t, "anon hint target", sampleCanvasPixel(img, blankTarget.X+blankTarget.W/2, blankTarget.Y+blankTarget.H/2, 2), anonColor.Alpha(0.42).Over(pal.Slip))

			// 3. Toggling flagMode clears hint, restores target slip, and drops center to plain number hover tint.
			tt.Key(0, ui.KeyF)
			tt.Frame()
			if v.LastHoverHint.Kind != HintNone {
				t.Fatalf("expected HintNone in flagMode, got %+v", v.LastHoverHint)
			}
			img = tt.Image()
			assertColorNear(t, "center in flagMode (plain number hover)", sampleCanvasPixel(img, centerTile.X+3, centerTile.Y+3, 2), pal.Ink.Alpha(0.05).Over(pal.Board))
			assertColorNear(t, "target restored in flagMode", sampleCanvasPixel(img, blankTarget.X+blankTarget.W/2, blankTarget.Y+blankTarget.H/2, 2), pal.Slip)
			tt.Key(0, ui.KeyF)

			// 4. Hovering a non-chordable revealed number paints subtle pal.Ink.Alpha(0.05) tint.
			nonChordTile := moveToCell(1, 5)
			if v.LastHoverHint.Kind != HintNone {
				t.Fatalf("expected HintNone on unchordable (1,5), got %+v", v.LastHoverHint)
			}
			assertColorNear(t, "non-chordable number hover", sampleCanvasPixel(tt.Image(), nonChordTile.X+3, nonChordTile.Y+3, 2), pal.Ink.Alpha(0.05).Over(pal.Board))

			// 5. Hovering a revealed empty cell leaves bare pal.Board.
			emptyTile := moveToCell(4, 4)
			assertColorNear(t, "empty revealed hover stays bare", sampleCanvasPixel(tt.Image(), emptyTile.X+emptyTile.W/2, emptyTile.Y+emptyTile.H/2, 2), pal.Board)

			// 6. Hovering an unrevealed slip mixes pal.Slip toward pal.Sheet by 0.45.
			slipTile := moveToCell(6, 0)
			assertColorNear(t, "hovered slip", sampleCanvasPixel(tt.Image(), slipTile.X+slipTile.W/2, slipTile.Y+slipTile.H/2, 2), pal.Slip.Mix(pal.Sheet, 0.45))

			// 7. Leaving the board clears hover and hint cleanly.
			moveToCell(1, 1)
			if v.LastHoverHint.Kind != HintChordable {
				t.Fatalf("expected HintChordable before leaving board, got %+v", v.LastHoverHint)
			}
			resetBtn, ok := tt.Find("演奏中 · 重开 (R)")
			if !ok {
				t.Fatal("reset button missing")
			}
			tt.Move(resetBtn.X+resetBtn.W/2, resetBtn.Y+resetBtn.H/2)
			tt.Frame()
			if v.HoverRow != -1 || v.LastHoverHint.Kind != HintNone {
				t.Fatalf("expected hover cleared off board, hover=(%d,%d) hint=%+v", v.HoverRow, v.HoverCol, v.LastHoverHint)
			}
			assertColorNear(t, "center restored after leaving board", sampleCanvasPixel(tt.Image(), centerTile.X+3, centerTile.Y+3, 2), pal.Board)

			// 8. End-of-game (StatusLost) suppresses hover hint even while pointer sits on (1,1).
			moveToCell(1, 1)
			g.Status = StatusLost
			tt.Frame()
			if v.LastHoverHint.Kind != HintNone {
				t.Fatalf("expected HintNone after StatusLost, got %+v", v.LastHoverHint)
			}
		})
	}
}

// Classic flood-fill, chord, skill, and winning reveals play the bandColor
// ripple wave (0.26 on empty cells, 0.22 on numbered cells, no stroke, no
// duel EffectCell ring), yield to isHintCenter, expire under the virtual
// clock, and clear immediately on reset.
func TestClassicCanvasRippleWaveVisualAndLifecycle(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(fmt.Sprintf("dark=%v", dark), func(t *testing.T) {
			app := newMinesweeperApp(7)
			app.darkMode = dark
			app.memberID = MemberTomori
			app.game = NewGame(Difficulty{
				ID:           "classic-ripple-fx",
				Gameplay:     GameplayClassic,
				Rows:         8,
				Cols:         8,
				Mines:        1,
				SkillCharges: 2,
			}, true, MemberTomori, 7)
			g := app.game
			g.FirstClick = true
			g.Status = StatusPlaying
			// Single mine at (0,7): clicking (0,0) flood-fills left-to-right across
			// depth 0..6, reaching numbered frontier (0,6) at depth 6.
			g.Cell(0, 7).IsMine = true
			g.RecomputeAdjacency()

			tt := ui.NewTester(classicCanvasTestView(app), 1280, 900)
			tt.SetScale(2)
			tt.Frame()

			board, ok := tt.Find("经典棋盘")
			if !ok {
				t.Fatal("classic canvas board missing")
			}
			pal := paperFor(dark)
			v := app.board

			originTile := classicCanvasCellRect(board, v, 0, 0)
			tt.ClickAt(originTile.X+originTile.W/2, originTile.Y+originTile.H/2)

			if len(app.ripples) == 0 {
				t.Fatal("expected classic flood-fill to populate ripples")
			}
			if v.EffectCell != -1 {
				t.Fatalf("classic click must not set duel EffectCell, got %d", v.EffectCell)
			}

			// At t=0, depth-0 empty cell (0,0) paints bandColor.Alpha(0.26) with no stroke.
			img := tt.Image()
			wantEmptyRipple := bandColor.Alpha(0.26).Over(pal.Board)
			assertColorNear(t, "depth-0 empty ripple interior", sampleCanvasPixel(img, originTile.X+originTile.W/2, originTile.Y+originTile.H/2, 2), wantEmptyRipple)
			assertColorNear(t, "depth-0 empty ripple edge (no stroke)", sampleCanvasPixel(img, originTile.X+originTile.W/2, originTile.Y+0.5, 2), wantEmptyRipple)

			// Depth-6 numbered cell (0,6) has not started yet at t=0.
			numIdx := 0*g.Cols + 6
			numRip, ok := app.ripples[numIdx]
			if !ok || numRip.Delay == 0 {
				t.Fatalf("expected delayed ripple on numbered frontier (0,6), got ok=%v rip=%+v", ok, numRip)
			}
			numTile := classicCanvasCellRect(board, v, 0, 6)
			assertColorNear(t, "delayed numbered cell before wave", sampleCanvasPixel(img, numTile.X+3, numTile.Y+3, 2), pal.Board)

			// Advance to the exact start of (0,6)'s wave and render: numbered cell paints bandColor.Alpha(0.22) with no stroke.
			tt.Advance(numRip.Delay)
			tt.Frame()
			img = tt.Image()
			wantNumRipple := bandColor.Alpha(0.22).Over(pal.Board)
			assertColorNear(t, "numbered ripple interior", sampleCanvasPixel(img, numTile.X+3, numTile.Y+3, 2), wantNumRipple)
			assertColorNear(t, "numbered ripple edge (no stroke)", sampleCanvasPixel(img, numTile.X+numTile.W/2, numTile.Y+0.5, 2), wantNumRipple)

			// Advancing further without manual Frame() lets p.AnimationFrame() repaint the decaying wave.
			tt.Advance(140 * time.Millisecond)
			midGot := sampleCanvasPixel(tt.Image(), numTile.X+3, numTile.Y+3, 2)
			if near(midGot.R, wantNumRipple.R) && near(midGot.G, wantNumRipple.G) && near(midGot.B, wantNumRipple.B) {
				t.Errorf("numbered ripple did not decay over 140ms: still %s", hexOf(midGot))
			}

			// Advance past full wave lifetime: ripples expire and cells return to bare pal.Board.
			tt.Advance(rippleMaxDelay + rippleDuration + 50*time.Millisecond)
			if len(app.ripples) != 0 {
				t.Fatalf("expected ripples expired cleanly, remaining=%d", len(app.ripples))
			}
			assertColorNear(t, "origin restored after ripple", sampleCanvasPixel(tt.Image(), originTile.X+originTile.W/2, originTile.Y+originTile.H/2, 2), pal.Board)
			assertColorNear(t, "numbered restored after ripple", sampleCanvasPixel(tt.Image(), numTile.X+3, numTile.Y+3, 2), pal.Board)

			// Hover hint precedence: when a chordable number also has an active ripple,
			// isHintCenter (0.38 highlighter) wins over ripple (matching main.go).
			g.Status = StatusPlaying
			g.Cell(0, 7).Flagged = true
			g.FlagsPlaced = 1
			g.Cell(1, 7).Revealed = false
			g.RevealedSafe--
			app.ripples = map[int]tileRipple{
				numIdx: {Start: tt.Now(), Delay: 0},
			}
			tt.Move(numTile.X+numTile.W/2, numTile.Y+numTile.H/2)
			tt.Frame()
			wantHintCenter := memberImage(MemberTomori).Alpha(0.38).Over(pal.Board)
			assertColorNear(t, "hint center overrides ripple", sampleCanvasPixel(tt.Image(), numTile.X+3, numTile.Y+3, 2), wantHintCenter)

			// Reset clears active ripples immediately.
			tt.Key(0, ui.KeyR)
			if len(app.ripples) != 0 {
				t.Fatalf("expected KeyR to clear ripples, remaining=%d", len(app.ripples))
			}

			// Skill reveal (KeyE) on a deterministic in-progress classic board also populates ripples and clears on KeyR.
			skillGame := NewGame(Difficulty{
				ID:           "classic-skill-ripple-fx",
				Gameplay:     GameplayClassic,
				Rows:         6,
				Cols:         6,
				Mines:        1,
				SkillCharges: 1,
			}, true, MemberTomori, 7)
			skillGame.FirstClick = true
			skillGame.Status = StatusPlaying
			skillGame.Cell(5, 5).IsMine = true
			skillGame.RecomputeAdjacency()
			skillGame.Cell(0, 0).Revealed = true
			skillGame.RevealedSafe = 1
			app.game = skillGame
			tt.Frame()
			tt.Key(0, ui.KeyE)
			if len(app.ripples) == 0 {
				t.Fatal("expected KeyE skill reveal on classic canvas to populate ripples")
			}
			tt.Key(0, ui.KeyR)
			if len(app.ripples) != 0 {
				t.Fatalf("expected KeyR to clear skill ripples, remaining=%d", len(app.ripples))
			}
		})
	}
}

// Classic shield saves (4.5 / 220ms) and mine losses (9.0 / 320ms) shake the
// rendered canvas pixels without shifting board or HUD hit-test bounds, decay
// back to exact alignment under the virtual clock, and clear on reset.
func TestClassicCanvasShakeVisualDecayAndHitTestStability(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(fmt.Sprintf("dark=%v", dark), func(t *testing.T) {
			app := newMinesweeperApp(7)
			app.darkMode = dark
			app.memberID = MemberSoyorin
			app.game = NewGame(Difficulty{
				ID:           "classic-shake-fx",
				Gameplay:     GameplayClassic,
				Rows:         6,
				Cols:         6,
				Mines:        2,
				SkillCharges: 1,
			}, true, MemberSoyorin, 7)
			g := app.game
			g.FirstClick = true
			g.Status = StatusPlaying
			g.Cell(0, 0).IsMine = true
			g.Cell(5, 5).IsMine = true
			g.RecomputeAdjacency()
			// Reveal row 2..3 so row 1 (slips) -> row 2 (bare board) forms a crisp
			// horizontal colour edge inside the viewport for pixel-shift checks.
			for col := 0; col < g.Cols; col++ {
				g.Cell(2, col).Revealed = true
				g.Cell(3, col).Revealed = true
				g.RevealedSafe += 2
			}

			tt := ui.NewTester(classicCanvasTestView(app), 1280, 900)
			tt.SetScale(2)
			tt.Frame()

			boardBefore, ok := tt.Find("经典棋盘")
			if !ok {
				t.Fatal("classic canvas board missing")
			}
			hudBefore, ok := tt.Find("演奏中 · 重开 (R)")
			if !ok {
				t.Fatal("reset HUD button missing")
			}
			pal := paperFor(dark)
			v := app.board

			// Probe the bottom pixel edge of slip (1,2) against revealed empty (2,2)
			// so shake tests prove actual rendered pixel translation, not just interior hits.
			emptyTile := classicCanvasCellRect(boardBefore, v, 2, 2)
			slipTile := classicCanvasCellRect(boardBefore, v, 1, 2)
			slipBottomEdgeY := func(img *image.RGBA) int {
				px := int((slipTile.X + slipTile.W/2) * 2)
				yMin := int((slipTile.Y - 12) * 2)
				yMax := int((emptyTile.Y + emptyTile.H) * 2)
				last := -1
				for py := yMin; py <= yMax; py++ {
					c := img.RGBAAt(px, py)
					if near(c.R, pal.Slip.R) && near(c.G, pal.Slip.G) && near(c.B, pal.Slip.B) {
						last = py
					}
				}
				return last
			}
			baseEdgeY := slipBottomEdgeY(tt.Image())
			if baseEdgeY < 0 {
				t.Fatal("could not locate baseline slip bottom edge")
			}

			// 1. Trigger Soyorin shield on (0,0) -> lighter 4.5 / 220ms shake.
			mine0 := classicCanvasCellRect(boardBefore, v, 0, 0)
			tt.ClickAt(mine0.X+mine0.W/2, mine0.Y+mine0.H/2)
			if !g.Cell(0, 0).ShieldSaved || app.shakeAmplitude != 4.5 || app.shakeDuration != 220*time.Millisecond {
				t.Fatalf("expected shield 4.5/220ms shake, shield=%v amp=%v dur=%v", g.Cell(0, 0).ShieldSaved, app.shakeAmplitude, app.shakeDuration)
			}
			if v.EffectCell != -1 {
				t.Fatalf("classic shield click must not set duel EffectCell, got %d", v.EffectCell)
			}
			if shieldEdge0 := slipBottomEdgeY(tt.Image()); shieldEdge0 <= baseEdgeY+2 {
				t.Fatalf("expected immediate shield shake (+dy) to shift slip edge down: base=%d got=%d", baseEdgeY, shieldEdge0)
			}

			tt.Advance(30 * time.Millisecond)
			sx, sy := app.currentShakeOffset(tt.Now())
			if sx == 0 && sy == 0 {
				t.Fatal("expected non-zero shield shake offset at 30ms")
			}
			boardDuring, _ := tt.Find("经典棋盘")
			hudDuring, _ := tt.Find("演奏中 · 重开 (R)")
			if boardDuring != boardBefore || hudDuring != hudBefore {
				t.Fatalf("bounds shifted during shield shake: board=%+v/%+v hud=%+v/%+v", boardBefore, boardDuring, hudBefore, hudDuring)
			}

			// Decay past 220ms restores zero offset and exact unshifted pixel edge.
			tt.Advance(250 * time.Millisecond)
			if sx2, sy2 := app.currentShakeOffset(tt.Now()); sx2 != 0 || sy2 != 0 {
				t.Fatalf("expected shield shake decayed to 0, got (%v,%v)", sx2, sy2)
			}
			if afterEdgeY := slipBottomEdgeY(tt.Image()); afterEdgeY != baseEdgeY {
				t.Fatalf("slip edge did not restore after shield shake: base=%d got=%d", baseEdgeY, afterEdgeY)
			}

			// 2. Hit second mine (5,5) with shield spent -> heavier 9.0 / 320ms shake on StatusLost.
			mine5 := classicCanvasCellRect(boardBefore, v, 5, 5)
			tt.ClickAt(mine5.X+mine5.W/2, mine5.Y+mine5.H/2)
			if g.Status != StatusLost || app.shakeAmplitude != 9.0 || app.shakeDuration != 320*time.Millisecond {
				t.Fatalf("expected loss 9.0/320ms shake, status=%v amp=%v dur=%v", g.Status, app.shakeAmplitude, app.shakeDuration)
			}
			if lossEdge0 := slipBottomEdgeY(tt.Image()); lossEdge0 <= baseEdgeY+6 {
				t.Fatalf("expected immediate 9.0 loss shake (+dy) to shift slip edge down: base=%d got=%d", baseEdgeY, lossEdge0)
			}
			tt.Advance(30 * time.Millisecond)
			lsx, lsy := app.currentShakeOffset(tt.Now())
			if lsx == 0 && lsy == 0 {
				t.Fatal("expected active loss shake at 30ms")
			}
			boardLost, _ := tt.Find("经典棋盘")
			if boardLost != boardBefore {
				t.Fatalf("board rect shifted during loss shake: before=%+v during=%+v", boardBefore, boardLost)
			}

			// 3. KeyR immediately clears active shake state.
			tt.Key(0, ui.KeyR)
			if app.shakeAmplitude != 0 || !app.shakeStart.IsZero() {
				t.Fatalf("expected immediate shake cleanup on KeyR, amp=%v start=%v", app.shakeAmplitude, app.shakeStart)
			}
		})
	}
}

func minimapCellCenter(board, mini ui.Rect, rows, cols, row, col int) (float32, float32) {
	plot := classicMinimapPlot(mini)
	return board.X + plot.X + (float32(col)+0.5)/float32(cols)*plot.W,
		board.Y + plot.Y + (float32(row)+0.5)/float32(rows)*plot.H
}

// Classic minimap renders only the player's own progress (opened safe cells,
// flags, and boom/wrong-flag markers) in the notebook palette across day and
// night, omits unhit mines revealed on loss and all duel AI colours, and
// supports clicking any target cell to jump the viewport without leaking edits.
func TestClassicMinimapShowsOwnProgressOnlyAndClickToJump(t *testing.T) {
	duelAINavy := ui.Hex("#0B1423")
	duelAIPink := ui.Hex("#F07AA4")
	duelContested := ui.Hex("#A78BFA")

	for _, dark := range []bool{false, true} {
		t.Run(fmt.Sprintf("dark=%v", dark), func(t *testing.T) {
			mc := classicMinimapFor(dark)
			if got := contrast(mc.Viewport, mc.Bg); got < minInkContrast {
				t.Errorf("viewport contrast on bg = %.2f < %.1f", got, minInkContrast)
			}
			if got := contrast(mc.Viewport, mc.Opened); got < 3.5 {
				t.Errorf("viewport contrast on opened = %.2f < 3.5", got)
			}

			// Stage a 10x10 classic board with distinct regions for every state so
			// each cell spans ~14.8px on the minimap and stays cleanly away from
			// the viewport outline when zoomed in.
			app := newMinesweeperApp(7)
			app.darkMode = dark
			app.memberID = MemberSoyorin
			app.allowQuestion = true
			app.game = NewGame(Difficulty{
				ID:           "classic-minimap-test",
				Gameplay:     GameplayClassic,
				Rows:         10,
				Cols:         10,
				Mines:        4,
				SkillCharges: 1,
			}, true, MemberSoyorin, 7)
			g := app.game
			g.FirstClick = true
			g.Status = StatusPlaying
			// Mines: (1,7) flagged, (3,7) shielded, (8,8) unhit mine revealed on loss, (5,7) hit mine.
			g.Cell(1, 7).IsMine = true
			g.Cell(3, 7).IsMine = true
			g.Cell(8, 8).IsMine = true
			g.Cell(5, 7).IsMine = true
			g.RecomputeAdjacency()

			tt := ui.NewTester(classicCanvasTestView(app), 1280, 900)
			tt.SetScale(2)
			tt.Frame()

			board, ok := tt.Find("经典棋盘")
			if !ok {
				t.Fatal("classic canvas board missing")
			}
			v := app.board
			mini := v.minimapBoundsFor(g)
			miniScreen := ui.Rect{X: board.X + mini.X, Y: board.Y + mini.Y, W: mini.W, H: mini.H}
			miniInterior := ui.Rect{X: miniScreen.X + 4, Y: miniScreen.Y + 4, W: miniScreen.W - 8, H: miniScreen.H - 8}

			// 1. Fresh board: minimap interior is pure notebook Bg, zero duel colours.
			img := tt.Image()
			sampleCell := func(img *image.RGBA, row, col int) ui.Color {
				cx, cy := minimapCellCenter(board, mini, g.Rows, g.Cols, row, col)
				return sampleCanvasPixel(img, cx, cy, 2)
			}
			assertColorNear(t, "idle minimap cell (2,2)", sampleCell(img, 2, 2), mc.Bg)
			for name, forbidden := range map[string]ui.Color{
				"duel navy":      duelAINavy,
				"duel AI pink":   duelAIPink,
				"duel contested": duelContested,
			} {
				if n := countMatchingPixels(img, miniInterior, 2, forbidden, 12); n != 0 {
					t.Errorf("idle classic minimap painted %d %s pixels", n, name)
				}
			}

			// 2. Live progress: open safe numbered cell (1,6) (no flood-fill),
			// flag mine (1,7), shield mine (3,7), misflag safe (5,2), mark ?
			// at (8,2), and leave (8,5) unrevealed.
			g.Reveal(1, 6)
			g.ToggleFlag(1, 7, false)
			g.Reveal(3, 7)
			g.ToggleFlag(5, 2, false)
			g.ToggleFlag(8, 2, true)
			g.ToggleFlag(8, 2, true)
			tt.Frame()
			img = tt.Image()

			assertColorNear(t, "live opened safe (1,6)", sampleCell(img, 1, 6), mc.Opened)
			assertColorNear(t, "live normal flag (1,7)", sampleCell(img, 1, 7), mc.Flag)
			assertColorNear(t, "live shield flag (3,7)", sampleCell(img, 3, 7), mc.Flag)
			assertColorNear(t, "live misflagged safe (5,2) stays flag", sampleCell(img, 5, 2), mc.Flag)
			assertColorNear(t, "live questioned (8,2) stays bg", sampleCell(img, 8, 2), mc.Bg)
			assertColorNear(t, "live unrevealed (8,5) stays bg", sampleCell(img, 8, 5), mc.Bg)
			for name, forbidden := range map[string]ui.Color{
				"duel AI pink":   duelAIPink,
				"duel contested": duelContested,
			} {
				if n := countMatchingPixels(img, miniInterior, 2, forbidden, 12); n != 0 {
					t.Errorf("live classic minimap painted %d %s pixels", n, name)
				}
			}

			// 3. Loss: hit mine (5,7) and wrong flag (5,2) paint Boom, while unhit
			// mine (8,8) revealed by finalizeLoss stays Bg (not player progress).
			g.Reveal(5, 7)
			if g.Status != StatusLost || !g.Cell(8, 8).Revealed || !g.Cell(5, 2).WrongFlag {
				t.Fatalf("expected StatusLost with revealed unhit mine and wrong flag, status=%v (8,8)=%+v (5,2)=%+v",
					g.Status, g.Cell(8, 8), g.Cell(5, 2))
			}
			tt.Frame()
			img = tt.Image()
			assertColorNear(t, "lost exploded mine (5,7)", sampleCell(img, 5, 7), mc.Boom)
			assertColorNear(t, "lost wrong flag (5,2)", sampleCell(img, 5, 2), mc.Boom)
			assertColorNear(t, "lost unhit mine (8,8) stays bg", sampleCell(img, 8, 8), mc.Bg)
			assertColorNear(t, "lost opened safe (1,6) preserved", sampleCell(img, 1, 6), mc.Opened)
			assertColorNear(t, "lost right flag (1,7) preserved", sampleCell(img, 1, 7), mc.Flag)

			// 4. Click-to-jump on a large 50x50 classic board: clicking interior
			// coordinates on the minimap centers the viewport on that cell and
			// moves the rendered viewport outline without mutating game state.
			bigApp := newMinesweeperApp(42)
			bigApp.darkMode = dark
			bigApp.game = NewGame(TrackUtakotoba, true, MemberTomori, 42)
			bigTT := ui.NewTester(classicCanvasTestView(bigApp), 960, 640)
			bigTT.SetScale(2)
			bigTT.Frame()
			bigBoard, ok := bigTT.Find("经典棋盘")
			if !ok {
				t.Fatal("50x50 classic canvas board missing")
			}
			bigApp.board.Cell = 36
			bigApp.board.clamp(bigApp.game)
			bigTT.Frame()

			bigMini := bigApp.board.minimapBoundsFor(bigApp.game)
			targets := [][2]int{{12, 38}, {38, 12}, {0, 0}, {49, 49}}
			for _, tgt := range targets {
				cx, cy := minimapCellCenter(bigBoard, bigMini, bigApp.game.Rows, bigApp.game.Cols, tgt[0], tgt[1])
				bigTT.ClickAt(cx, cy)
				if bigApp.game.FirstClick || bigApp.game.Moves != 0 || bigApp.game.FlagsPlaced != 0 {
					t.Fatalf("minimap jump to %v leaked board action", tgt)
				}
				if tgt[0] > 0 && tgt[0] < bigApp.game.Rows-1 {
					gotR, gotC, ok := bigApp.board.cellAt(bigApp.game, bigBoard.W/2, bigBoard.H/2)
					if !ok || absInt(gotR-tgt[0]) > 1 || absInt(gotC-tgt[1]) > 1 {
						t.Fatalf("minimap interior jump to %v landed at (%d,%d) ok=%v", tgt, gotR, gotC, ok)
					}
				}
				// Verify outset viewport frame appears along the computed viewport rect.
				plotAbs := classicMinimapPlot(ui.Rect{X: bigBoard.X + bigMini.X, Y: bigBoard.Y + bigMini.Y, W: bigMini.W, H: bigMini.H})
				vp := minimapViewportRect(plotAbs, bigApp.game.Rows, bigApp.game.Cols, bigApp.board.X, bigApp.board.Y, bigBoard.W, bigBoard.H, bigApp.board.Cell)
				probe := ui.Rect{X: vp.X + vp.W/2 - 3, Y: vp.Y - 3, W: 6, H: 4}
				if got := countMatchingPixels(bigTT.Image(), probe, 2, mc.Viewport, 28); got < 4 {
					t.Errorf("minimap viewport frame missing near %+v after jump to %v (matched %d px)", probe, tgt, got)
				}
			}
		})
	}
}

// On a 100x100 classic board, corner cells stay inside the rounded card clip
// and edge-row flags stay visible while the viewport sits flush against that
// edge, at both 1x and 2x scale in day and night themes.
func TestClassicMinimap100x100CornersAndEdgeFlagsStayVisible(t *testing.T) {
	for _, dark := range []bool{false, true} {
		for _, scale := range []float32{1, 2} {
			t.Run(fmt.Sprintf("dark=%v/scale=%.0fx", dark, scale), func(t *testing.T) {
				app := newMinesweeperApp(42)
				app.darkMode = dark
				app.game = NewGame(TrackMayoiuta, true, MemberTomori, 42)
				g := app.game

				tt := ui.NewTester(classicCanvasTestView(app), 1280, 900)
				tt.SetScale(scale)
				tt.Frame()

				board, ok := tt.Find("经典棋盘")
				if !ok {
					t.Fatal("classic canvas board missing")
				}
				mc := classicMinimapFor(dark)
				mini := app.board.minimapBoundsFor(g)
				plotAbs := classicMinimapPlot(ui.Rect{X: board.X + mini.X, Y: board.Y + mini.Y, W: mini.W, H: mini.H})
				cw := plotAbs.W / float32(g.Cols)
				ch := plotAbs.H / float32(g.Rows)
				cellProbe := func(row, col int) ui.Rect {
					return ui.Rect{
						X: plotAbs.X + float32(col)*cw - 1,
						Y: plotAbs.Y + float32(row)*ch - 1,
						W: cw + 2,
						H: ch + 2,
					}
				}

				// 1. Flag all four corners, jump to center, and verify every corner renders mc.Flag.
				corners := [][2]int{{0, 0}, {0, g.Cols - 1}, {g.Rows - 1, 0}, {g.Rows - 1, g.Cols - 1}}
				for _, pt := range corners {
					g.ToggleFlag(pt[0], pt[1], false)
				}
				cx, cy := minimapCellCenter(board, mini, g.Rows, g.Cols, g.Rows/2, g.Cols/2)
				tt.ClickAt(cx, cy)
				tt.Frame()
				img := tt.Image()
				for _, pt := range corners {
					if n := countMatchingPixels(img, cellProbe(pt[0], pt[1]), scale, mc.Flag, 45); n == 0 {
						t.Errorf("100x100 corner flag (%d,%d) clipped or invisible on minimap", pt[0], pt[1])
					}
				}

				// 2. Pin viewport to each board edge and flag the middle cell on that edge.
				edges := [][2]int{{0, g.Cols / 2}, {g.Rows - 1, g.Cols / 2}, {g.Rows / 2, 0}, {g.Rows / 2, g.Cols - 1}}
				for _, pt := range edges {
					g.ToggleFlag(pt[0], pt[1], false)
					app.board.center(g, pt[0], pt[1])
					tt.Frame()
					if n := countMatchingPixels(tt.Image(), cellProbe(pt[0], pt[1]), scale, mc.Flag, 45); n == 0 {
						t.Errorf("100x100 edge flag (%d,%d) obscured by flush viewport stroke", pt[0], pt[1])
					}
				}
			})
		}
	}
}

// classicSurfaceCellAt returns where the real app shell shows a classic
// cell: its widget on the grid, or its spot on the canvas, panned into view.
func classicSurfaceCellAt(t *testing.T, tt *ui.Tester, app *minesweeperApp, row, col int) (float32, float32) {
	t.Helper()
	if r, ok := tt.Find(fmt.Sprintf("cell-%d-%d", row, col)); ok {
		return r.X + r.W/2, r.Y + r.H/2
	}
	r, ok := tt.Find("经典棋盘")
	if !ok {
		t.Fatalf("neither cell-%d-%d nor the classic canvas is on screen", row, col)
	}
	v := app.board
	v.center(app.game, row, col)
	tt.Frame()
	return r.X + (float32(col)+0.5)*v.Cell - v.X, r.Y + (float32(row)+0.5)*v.Cell - v.Y
}

// TestClassicBoardSurfaceFollowsThreshold mounts the real app shell: every
// shipped classic track and anything up to 《影色舞》's 30×30 keeps one widget
// per cell, while a board wider or taller than that plays on the shared
// canvas, where flags, mine counts and losses work the same way.
func TestClassicBoardSurfaceFollowsThreshold(t *testing.T) {
	type surfaceCase struct {
		name   string
		diff   Difficulty
		canvas bool
	}
	var cases []surfaceCase
	for _, d := range Difficulties {
		if d.Gameplay == GameplayClassic {
			cases = append(cases, surfaceCase{"shipped " + d.ID, d, false})
		}
	}
	cases = append(cases,
		surfaceCase{"30x30 at the threshold", TrackKageiromai, false},
		surfaceCase{"30x31 one column past", Difficulty{ID: "wide", Gameplay: GameplayClassic, Rows: 30, Cols: 31, Mines: 150}, true},
		surfaceCase{"31x30 one row past", Difficulty{ID: "tall", Gameplay: GameplayClassic, Rows: 31, Cols: 30, Mines: 150}, true},
		surfaceCase{"50x50 utakotoba", TrackUtakotoba, true},
		surfaceCase{"100x100 mayoiuta", TrackMayoiuta, true},
	)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := newMinesweeperApp(7)
			app.game = NewGame(tc.diff, app.bandMode(), app.memberID, 7)
			tt := ui.NewTester(app.view, 1280, 900)
			g := app.game

			_, onCanvas := tt.Find("经典棋盘")
			_, onGrid := tt.Find("cell-0-0")
			if onCanvas != tc.canvas || onGrid == tc.canvas {
				t.Fatalf("%dx%d board: canvas=%v grid=%v, want canvas=%v", g.Rows, g.Cols, onCanvas, onGrid, tc.canvas)
			}

			g.FirstClick = true
			g.Status = StatusPlaying
			g.Cell(0, 0).IsMine = true
			g.RecomputeAdjacency()
			x, y := classicSurfaceCellAt(t, tt, app, 0, 1)
			tt.RightClickAt(x, y)
			wantMines := "MINES " + formatDigits(g.Mines-1)
			if !g.Cell(0, 1).Flagged || !tt.HasText(wantMines) {
				t.Fatalf("right-click (0,1): flagged=%v, want flag and %q on screen", g.Cell(0, 1).Flagged, wantMines)
			}
			x, y = classicSurfaceCellAt(t, tt, app, 0, 0)
			tt.ClickAt(x, y)
			if g.Status != StatusLost || !tt.HasText("炸团危机！……要假装无事发生重开吗？") {
				t.Fatalf("click on mine (0,0): status=%v, want loss and the defeat toast", g.Status)
			}
		})
	}
}

// TestSetlistSwitchesClassicSurfaceAcrossThreshold picks tracks from the
// setlist on the seven-track ladder: the board surface follows each pick,
// both onto the canvas and back to the widget grid.
func TestSetlistSwitchesClassicSurfaceAcrossThreshold(t *testing.T) {
	orig := Difficulties
	Difficulties = sevenTrackSetlist()
	defer func() { Difficulties = orig }()

	app := newMinesweeperApp(42)
	tt := ui.NewTester(app.view, 1280, 900)
	for _, step := range []struct {
		track  Difficulty
		canvas bool
	}{
		{TrackMayoiuta, true},
		{TrackKageiromai, false},
		{TrackUtakotoba, true},
		{Difficulties[0], false},
	} {
		if err := tt.Click("歌单"); err != nil {
			t.Fatalf("open setlist before %s: %v", step.track.ID, err)
		}
		if err := tt.Click(step.track.Song); err != nil {
			t.Fatalf("pick %s: %v", step.track.ID, err)
		}
		if app.game.Difficulty.ID != step.track.ID {
			t.Fatalf("picked %q, playing %q", step.track.ID, app.game.Difficulty.ID)
		}
		_, onCanvas := tt.Find("经典棋盘")
		_, onGrid := tt.Find("cell-0-0")
		if onCanvas != step.canvas || onGrid == step.canvas {
			t.Fatalf("%s: canvas=%v grid=%v, want canvas=%v", step.track.ID, onCanvas, onGrid, step.canvas)
		}
	}
}
