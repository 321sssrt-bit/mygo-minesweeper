package main

import (
	"fmt"
	"image"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"mygo-minesweeper/internal/scanreport"
)

func diffPixelCountInRect(a, b *image.RGBA, r ui.Rect, scale float32) int {
	x0 := max(0, int(r.X*scale))
	y0 := max(0, int(r.Y*scale))
	x1 := min(a.Bounds().Dx(), int((r.X+r.W)*scale))
	y1 := min(a.Bounds().Dy(), int((r.Y+r.H)*scale))
	changed := 0
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			ca := a.RGBAAt(x, y)
			cb := b.RGBAAt(x, y)
			if absInt(int(ca.R)-int(cb.R)) > 8 || absInt(int(ca.G)-int(cb.G)) > 8 || absInt(int(ca.B)-int(cb.B)) > 8 {
				changed++
			}
		}
	}
	return changed
}

func rectContainsColorScaled(img *image.RGBA, r ui.Rect, scale float32, want ui.Color, tol int) bool {
	return rectContainsColor(img, ui.Rect{
		X: r.X * scale,
		Y: r.Y * scale,
		W: r.W * scale,
		H: r.H * scale,
	}, want, tol)
}

func TestStageHighlightPaletteContrastAndScan(t *testing.T) {
	for _, dark := range []bool{false, true} {
		for _, m := range Members {
			qs := quoteStripColorsFor(m.ID, dark)
			for name, fg := range map[string]ui.Color{
				"NameInk": qs.NameInk,
				"TextInk": qs.TextInk,
				"RoleInk": qs.RoleInk,
			} {
				if cr := contrast(fg, qs.Bg); cr < minInkContrast {
					t.Errorf("dark=%v member=%s quoteStrip %s contrast %.2f < %.1f (fg=%s bg=%s)",
						dark, m.ID, name, cr, minInkContrast, hexOf(fg), hexOf(qs.Bg))
				}
			}
		}
		ms := memeStripColorsFor(dark)
		for name, fg := range map[string]ui.Color{
			"EyebrowInk": ms.EyebrowInk,
			"TitleInk":   ms.TitleInk,
		} {
			if cr := contrast(fg, ms.Bg); cr < minInkContrast {
				t.Errorf("dark=%v memeStrip %s contrast %.2f < %.1f (fg=%s bg=%s)",
					dark, name, cr, minInkContrast, hexOf(fg), hexOf(ms.Bg))
			}
		}
	}

	rep, err := scanreport.Scan(".")
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Copy) != 0 || len(rep.Colors) != 0 {
		t.Fatalf("expected zero scanreport findings, got:\n%s", rep)
	}
}

func TestQuoteStripAndSkillTargetPulsesBothBackends(t *testing.T) {
	// 1. Active skills on both grid & canvas: exact same-source quote, 1 & 2 targets,
	// pass-through click & scroll, 1.2s lifecycle, and consecutive trigger replacement.
	for _, backend := range []string{"grid", "canvas"} {
		t.Run(backend+"/anon-two-targets-passthrough-and-lifecycle", func(t *testing.T) {
			app := newMinesweeperApp(99)
			app.memberID = MemberAnon
			rows, cols, mines := 16, 16, 40
			if backend == "canvas" {
				rows, cols, mines = 50, 50, 425
			}
			app.game = NewGame(Difficulty{
				ID:           "skill-test",
				Song:         "迷星叫",
				Subtitle:     fmt.Sprintf("%d×%d", rows, cols),
				Gameplay:     GameplayClassic,
				Rows:         rows,
				Cols:         cols,
				Mines:        mines,
				SkillCharges: 3,
			}, true, MemberAnon, 99)

			viewFn := app.view
			if backend == "canvas" {
				viewFn = classicCanvasTestView(app)
			}
			tt := ui.NewTester(viewFn, 1280, 900)
			tt.SetScale(2)
			tt.Frame()

			// Idle KeyE must be a complete noop: no quote strip, no skill pulses.
			tt.Key(0, ui.KeyE)
			if app.quoteStrip.Active || len(app.skillPulses) != 0 {
				t.Fatalf("idle KeyE must not show quote strip or pulses: strip=%+v pulses=%d", app.quoteStrip, len(app.skillPulses))
			}

			// Start the game near center and settle initial wave.
			startRow, startCol := rows/2, cols/2
			if backend == "grid" {
				rc, _ := tt.Find(fmt.Sprintf("cell-%d-%d", startRow, startCol))
				tt.ClickAt(rc.X+rc.W/2, rc.Y+rc.H/2)
			} else {
				br, _ := tt.Find("经典棋盘")
				app.board.center(app.game, startRow, startCol)
				tt.Frame()
				cc := classicCanvasCellRect(br, app.board, startRow, startCol)
				tt.ClickAt(cc.X+cc.W/2, cc.Y+cc.H/2)
			}
			tt.Advance(600 * time.Millisecond)

			// Fire Anon's 2-target skill.
			tt.Key(0, ui.KeyE)
			if !app.quoteStrip.Active || app.quoteStrip.MemberID != MemberAnon {
				t.Fatalf("expected active Anon quote strip, got %+v", app.quoteStrip)
			}
			lastChat := app.chatLog[len(app.chatLog)-1]
			if app.quoteStrip.Text != lastChat.Text || app.quoteStrip.Text != app.quote {
				t.Fatalf("quote strip must reuse exact chat quote: strip=%q chat=%q app=%q",
					app.quoteStrip.Text, lastChat.Text, app.quote)
			}
			if len(app.skillPulses) != 2 {
				t.Fatalf("expected 2 Anon skill target pulses, got %d (%+v)", len(app.skillPulses), app.skillPulses)
			}
			firstStart := app.quoteStrip.Start
			wantStripBg := quoteStripColorsFor(MemberAnon, app.darkMode).Bg

			// Enter transition frame (t=+40ms): strip slides down at full opaque Bg (no contrast washout).
			tt.Advance(40 * time.Millisecond)
			stripRectEnter, ok := tt.Find("台词")
			if !ok {
				t.Fatal("quote strip not found during enter frame")
			}
			if !rectContainsColorScaled(tt.Image(), ui.Rect{X: stripRectEnter.X + 4, Y: max(float32(0), stripRectEnter.Y + 4), W: stripRectEnter.W - 8, H: max(float32(4), stripRectEnter.H - 8)}, 2, wantStripBg, 4) {
				t.Errorf("quote strip enter frame did not render full opaque Bg %s", hexOf(wantStripBg))
			}

			// Advance into pulse peak & hold (t=+160ms total) and verify visible ring pixels.
			tt.Advance(120 * time.Millisecond)
			targetIdx := app.skillPulses[0].Index
			tRow, tCol := targetIdx/app.game.Cols, targetIdx%app.game.Cols
			var haloRect ui.Rect
			if backend == "grid" {
				tr, ok := tt.Find(fmt.Sprintf("cell-%d-%d", tRow, tCol))
				if !ok {
					t.Fatalf("target cell-(%d,%d) not found", tRow, tCol)
				}
				haloRect = ui.Rect{X: tr.X - tr.W*0.5, Y: tr.Y - tr.H*0.5, W: tr.W * 2, H: tr.H * 2}
			} else {
				app.board.center(app.game, tRow, tCol)
				tt.Frame()
				br, _ := tt.Find("经典棋盘")
				tr := classicCanvasCellRect(br, app.board, tRow, tCol)
				haloRect = ui.Rect{X: tr.X - tr.W*0.5, Y: tr.Y - tr.H*0.5, W: tr.W * 2, H: tr.H * 2}
			}
			pulseImg := tt.Image()

			// Verify pass-through directly over the quote strip's 26x26 avatar glyph center
			// (not just the text middle!) so IgnoreInput cascades into ui.Avatar's inner text.
			stripRect, ok := tt.Find("台词")
			if !ok {
				t.Fatal("quote strip not found during hold frame")
			}
			if !rectContainsColorScaled(tt.Image(), ui.Rect{X: stripRect.X + 4, Y: stripRect.Y + 4, W: stripRect.W - 8, H: stripRect.H - 8}, 2, wantStripBg, 4) {
				t.Errorf("quote strip hold frame did not render full opaque Bg %s", hexOf(wantStripBg))
			}
			// Avatar sits at left padding (12) + radius (13) horizontally, vertically centered in strip.
			avatarX := stripRect.X + 12 + 13
			avatarY := stripRect.Y + stripRect.H/2
			if backend == "grid" {
				hitCell := false
				for row := 0; row <= 2 && !hitCell; row++ {
					for col := range app.game.Cols {
						rc, ok := tt.Find(fmt.Sprintf("cell-%d-%d", row, col))
						if !ok {
							continue
						}
						if avatarX >= rc.X && avatarX < rc.X+rc.W && avatarY >= rc.Y && avatarY < rc.Y+rc.H {
							cell := app.game.Cell(row, col)
							if cell.Revealed {
								// Ensure an unrevealed cell sits under the avatar for flag toggle check.
								cell.Revealed = false
								tt.Frame()
							}
							tt.RightClickAt(avatarX, avatarY)
							if !cell.Flagged {
								t.Fatalf("quote strip avatar glyph blocked right-click on cell (%d,%d) at (%.1f,%.1f)", row, col, avatarX, avatarY)
							}
							hitCell = true
							break
						}
					}
				}
				if !hitCell {
					t.Fatalf("could not align avatar center (%.1f,%.1f) over a grid cell", avatarX, avatarY)
				}
			} else {
				br, _ := tt.Find("经典棋盘")
				// Scroll directly over the avatar glyph center: canvas zoom must change!
				beforeCell := app.board.Cell
				tt.Scroll(avatarX, avatarY, 0, -120)
				if app.board.Cell == beforeCell {
					t.Fatalf("quote strip avatar glyph blocked wheel zoom at (%.1f,%.1f) (cell stayed %.2f)", avatarX, avatarY, beforeCell)
				}
				// Right-click directly over the avatar glyph center: must flag the cell beneath!
				row, col, ok := app.board.cellAt(app.game, avatarX-br.X, avatarY-br.Y)
				if !ok {
					t.Fatalf("avatar center (%.1f,%.1f) did not map to a canvas cell (board=%+v)", avatarX, avatarY, br)
				}
				cell := app.game.Cell(row, col)
				if cell.Revealed {
					cell.Revealed = false
					tt.Frame()
				}
				tt.RightClickAt(avatarX, avatarY)
				if !cell.Flagged {
					t.Fatalf("quote strip avatar glyph blocked click pass-through on canvas cell (%d,%d)", row, col)
				}
			}

			// Advance to 600ms and fire a second skill: must replace old strip and reset timer.
			tt.Advance(440 * time.Millisecond)
			if !app.quoteStrip.Active {
				t.Fatal("expected first quote strip still active at 600ms")
			}
			tt.Key(0, ui.KeyE)
			if !app.quoteStrip.Active || !app.quoteStrip.Start.After(firstStart) {
				t.Fatalf("second skill trigger did not replace strip timer: first=%v second=%v", firstStart, app.quoteStrip.Start)
			}

			// Check exit transition frame (+1080ms after 2nd trigger, inside final 180ms slide-out):
			// visible strip pixels must still keep full opaque sc.Bg (no alpha washout).
			tt.Advance(1080 * time.Millisecond)
			if !app.quoteStrip.Active {
				t.Fatal("replaced quote strip expired prematurely before 1.2s")
			}
			if exitRect, ok := tt.Find("台词"); ok {
				if !rectContainsColorScaled(tt.Image(), ui.Rect{X: exitRect.X + 4, Y: max(float32(0), exitRect.Y + 4), W: exitRect.W - 8, H: max(float32(4), exitRect.H - 8)}, 2, wantStripBg, 4) {
					t.Errorf("quote strip exit frame faded Bg away from opaque %s", hexOf(wantStripBg))
				}
			}

			// After full 1.2s + pulse expiry, both strip and pulses must settle completely.
			tt.Advance(220 * time.Millisecond)
			if app.quoteStrip.Active || len(app.skillPulses) != 0 {
				t.Fatalf("expected quote strip & skill pulses cleared after lifecycle: strip=%v pulses=%d",
					app.quoteStrip.Active, len(app.skillPulses))
			}
			settledImg := tt.Image()
			if changed := diffPixelCountInRect(pulseImg, settledImg, haloRect, 2); changed < 20 {
				t.Errorf("expected visible skill target pulse ring pixels around (%d,%d), changed=%d", tRow, tCol, changed)
			}
			if backend == "canvas" && tt.FramePending() {
				t.Fatal("canvas still requested animation frames after quote strip and skill pulses settled")
			}
		})
	}

	// 2. Soyorin passive shield shows MemberSoyorin even if a.memberID is a different member,
	// with 0 target pulses and exact same-source quote in chatLog.
	t.Run("soyorin-shield-uses-actual-actor-not-selected-member", func(t *testing.T) {
		app := newMinesweeperApp(7)
		app.memberID = MemberTomori // deliberately different from MemberSoyorin
		app.game = makeCorridorGame(6, 6, true, MemberSoyorin, [][2]int{{0, 0}, {5, 5}})
		tt := ui.NewTester(app.view, 1280, 900)
		tt.Frame()

		// Trigger Soyorin shield on (0,0) while selected UI member is MemberTomori.
		r00, _ := tt.Find("cell-0-0")
		tt.ClickAt(r00.X+r00.W/2, r00.Y+r00.H/2)
		if !app.game.Cell(0, 0).ShieldSaved {
			t.Fatal("expected Soyorin shield to save (0,0)")
		}
		if !app.quoteStrip.Active || app.quoteStrip.MemberID != MemberSoyorin {
			t.Fatalf("shield quote strip must show MemberSoyorin, got %+v", app.quoteStrip)
		}
		if len(app.skillPulses) != 0 {
			t.Fatalf("shield has 0 SkillTargets and must not fabricate target pulses, got %d", len(app.skillPulses))
		}
		// Chat log entry for shield must also be from 长崎爽世 with identical quote text.
		foundShieldChat := false
		for _, msg := range app.chatLog {
			if msg.Speaker == "长崎爽世" && msg.Text == app.quoteStrip.Text {
				foundShieldChat = true
				break
			}
		}
		if !foundShieldChat {
			t.Fatalf("shield quote strip text %q not found under 长崎爽世 in chatLog %+v", app.quoteStrip.Text, app.chatLog)
		}
	})

	// 3. Skill direct win triggers quote strip, skill target pulses, AND victory confetti together.
	t.Run("winning-skill-triggers-strip-pulses-and-confetti", func(t *testing.T) {
		app := newMinesweeperApp(23)
		app.memberID = MemberAnon
		app.game = fixedBoard(map[int]bool{0: true, 24: true}, 1, 23)
		app.game.BandMode = true
		app.game.MemberID = MemberAnon
		app.game.SkillUsesLeft = 1
		tt := ui.NewTester(app.view, 1280, 900)
		tt.Frame()

		tt.Key(0, ui.KeyE)
		if app.game.Status != StatusWon {
			t.Fatalf("expected skill direct win, status=%v", app.game.Status)
		}
		if !app.quoteStrip.Active || app.quoteStrip.MemberID != MemberAnon {
			t.Fatalf("expected quote strip on winning skill, got %+v", app.quoteStrip)
		}
		if len(app.skillPulses) != 2 {
			t.Fatalf("expected 2 target pulses on winning Anon skill, got %d", len(app.skillPulses))
		}
		if len(app.confetti) != confettiCount {
			t.Fatalf("expected victory confetti on winning skill, got %d", len(app.confetti))
		}
	})

	// 4. Duel active skill (0 targets) shows quote strip and spawns ZERO target pulses;
	// exhausted skill press (ActionNoop) shows nothing.
	t.Run("duel-zero-target-skill-and-noop-guard", func(t *testing.T) {
		app := newMinesweeperApp(7)
		app.memberID = MemberRana
		app.game = NewGame(Difficulty{
			ID:           "duel-skill-test",
			Song:         "潜在表明",
			Gameplay:     GameplayDuel,
			Rows:         10,
			Cols:         10,
			Mines:        12,
			SkillCharges: 1,
			DuelDuration: 60,
			AIInterval:   10,
		}, true, MemberRana, 7)
		app.game.Reveal(5, 5)
		tt := ui.NewTester(app.view, 1280, 900)
		tt.Frame()

		tt.Key(0, ui.KeyE)
		if !app.quoteStrip.Active || app.quoteStrip.MemberID != MemberRana {
			t.Fatalf("expected Rana duel quote strip, got %+v", app.quoteStrip)
		}
		if len(app.skillPulses) != 0 {
			t.Fatalf("0-target duel skill must not fabricate target pulses, got %d", len(app.skillPulses))
		}
		tt.Advance(quoteStripDuration + 50*time.Millisecond)
		if app.quoteStrip.Active {
			t.Fatal("expected duel quote strip expired")
		}
		// Second press with 0 charges left -> ActionNoop -> must not show strip!
		tt.Key(0, ui.KeyE)
		if app.quoteStrip.Active || len(app.skillPulses) != 0 {
			t.Fatalf("exhausted skill press must not show quote strip or pulses: strip=%+v pulses=%d", app.quoteStrip, len(app.skillPulses))
		}
	})
}

func TestKnotBurstPulseBothBackendsAndDuel(t *testing.T) {
	for _, backend := range []string{"grid", "canvas"} {
		t.Run(backend, func(t *testing.T) {
			app := newMinesweeperApp(7)
			app.memberID = MemberTomori
			app.game = makeCorridorGame(7, 7, true, MemberTomori, [][2]int{{3, 3}, {6, 6}})
			viewFn := app.view
			if backend == "canvas" {
				viewFn = classicCanvasTestView(app)
			}
			tt := ui.NewTester(viewFn, 1280, 900)
			tt.SetScale(2)
			tt.Frame()

			var mineRect ui.Rect
			if backend == "grid" {
				mineRect, _ = tt.Find("cell-3-3")
			} else {
				br, _ := tt.Find("经典棋盘")
				mineRect = classicCanvasCellRect(br, app.board, 3, 3)
			}
			tt.ClickAt(mineRect.X+mineRect.W/2, mineRect.Y+mineRect.H/2)
			if len(app.knotBursts) != 1 || app.knotBursts[0].Index != 3*7+3 {
				t.Fatalf("expected knot burst at (3,3), got %+v", app.knotBursts)
			}

			tt.Advance(110 * time.Millisecond)
			burstImg := tt.Image()
			halo := ui.Rect{X: mineRect.X - mineRect.W*0.6, Y: mineRect.Y - mineRect.H*0.6, W: mineRect.W * 2.2, H: mineRect.H * 2.2}

			tt.Advance(knotBurstDuration + 50*time.Millisecond)
			if len(app.knotBursts) != 0 {
				t.Fatalf("expected knot burst pruned after lifecycle, got %d", len(app.knotBursts))
			}
			settledImg := tt.Image()
			if changed := diffPixelCountInRect(burstImg, settledImg, halo, 2); changed < 20 {
				t.Errorf("expected visible knot burst pixels around (3,3), changed=%d", changed)
			}
		})
	}

	// Duel: player mine hit AND Tomori AI mine hit both enqueue knot burst; shield does not.
	t.Run("duel-player-ai-mine-burst-and-shield-no-burst", func(t *testing.T) {
		app := newMinesweeperApp(7)
		app.memberID = MemberSoyorin
		app.game = NewGame(Difficulty{
			ID:           "duel-burst",
			Song:         "潜在表明",
			Gameplay:     GameplayDuel,
			Rows:         6,
			Cols:         6,
			Mines:        3,
			SkillCharges: 1,
			DuelDuration: 60,
			AIInterval:   0.5,
		}, true, MemberSoyorin, 7)
		app.game.FirstClick = true
		app.game.Status = StatusPlaying
		app.game.Cell(0, 0).IsMine = true
		app.game.Cell(0, 1).IsMine = true
		app.game.Cell(5, 5).IsMine = true
		app.game.RecomputeAdjacency()
		app.game.duel.NextAI = 99
		tt := ui.NewTester(app.view, 1280, 900)
		tt.Frame()

		// Move 1: shield saves (0,0) -> NO knot burst.
		br, _ := tt.Find("决斗棋盘")
		c00 := classicCanvasCellRect(br, app.board, 0, 0)
		tt.ClickAt(c00.X+c00.W/2, c00.Y+c00.H/2)
		if len(app.knotBursts) != 0 {
			t.Fatalf("shield save must not spawn knot burst, got %+v", app.knotBursts)
		}

		// Move 2: player hits (0,1) with 0 charges -> knot burst at idx 1.
		c01 := classicCanvasCellRect(br, app.board, 0, 1)
		tt.ClickAt(c01.X+c01.W/2, c01.Y+c01.H/2)
		if len(app.knotBursts) != 1 || app.knotBursts[0].Index != 1 {
			t.Fatalf("expected duel player knot burst at index 1, got %+v", app.knotBursts)
		}
		tt.Advance(knotBurstDuration + 50*time.Millisecond)
		if len(app.knotBursts) != 0 {
			t.Fatalf("expected duel player knot burst pruned, got %d", len(app.knotBursts))
		}

		// Move 3: force Tomori AI to step on the mine at (5,5) by planting a wrong flag
		// beside (4,4) so flags == Adjacent == 1 and chooseAIAction reveals (5,5).
		for i := range app.game.Cells {
			c := &app.game.Cells[i]
			c.IsMine = false
			c.Revealed = false
			c.Flagged = false
			c.Exploded = false
			c.Owner = DuelNobody
			c.FlagOwner = DuelNobody
		}
		app.game.Cell(5, 5).IsMine = true
		app.game.RecomputeAdjacency()
		for i := range app.game.Cells {
			c := &app.game.Cells[i]
			if i != 5*6+5 && i != 5*6+4 {
				c.Revealed = true
				c.Owner = DuelPlayer
			}
		}
		app.game.Cell(5, 4).Flagged = true
		app.game.Cell(5, 4).FlagOwner = DuelPlayer
		app.game.duel.frontier = map[int]bool{5*6 + 5: true}
		app.game.duel.NextAI = app.game.duel.Elapsed + 0.1
		tt.Advance(200 * time.Millisecond)
		if len(app.knotBursts) != 1 || app.knotBursts[0].Index != 5*6+5 {
			t.Fatalf("expected duel AI knot burst at (5,5), got %+v", app.knotBursts)
		}
	})
}

func TestVictoryConfettiClassicAndDuelTruthTable(t *testing.T) {
	// 1. Classic win spawns 5-color confetti + hand-drawn '!', passes clicks through, and settles cleanly.
	t.Run("classic-win-five-colors-and-exclaim", func(t *testing.T) {
		app := newMinesweeperApp(7)
		app.memberID = MemberTomori
		app.game = makeCorridorGame(5, 5, true, MemberTomori, [][2]int{{3, 4}})
		for i := range app.game.Cells {
			c := &app.game.Cells[i]
			if !c.IsMine && !(c.Row == 4 && c.Col == 4) {
				app.game.Reveal(c.Row, c.Col)
			}
		}
		tt := ui.NewTester(app.view, 1280, 900)
		tt.SetScale(2)
		tt.Frame()

		r44, _ := tt.Find("cell-4-4")
		tt.ClickAt(r44.X+r44.W/2, r44.Y+r44.H/2)
		if app.game.Status != StatusWon {
			t.Fatalf("expected classic win, status=%v", app.game.Status)
		}
		if len(app.confetti) != confettiCount {
			t.Fatalf("expected %d confetti pieces, got %d", confettiCount, len(app.confetti))
		}
		seenMembers := map[MemberID]bool{}
		exclaims := 0
		for _, pc := range app.confetti {
			seenMembers[pc.MemberID] = true
			if pc.Exclaim {
				exclaims++
			}
		}
		if len(seenMembers) != len(Members) || exclaims == 0 {
			t.Fatalf("expected all 5 member colours and hand-drawn '!' confetti, seen=%d exclaims=%d", len(seenMembers), exclaims)
		}

		tt.Advance(420 * time.Millisecond)
		midImg := tt.Image()
		tt.Advance(confettiDuration)
		if len(app.confetti) != 0 {
			t.Fatalf("expected confetti cleared after %v, remaining=%d", confettiDuration, len(app.confetti))
		}
		afterImg := tt.Image()
		stageRect := ui.Rect{X: 320, Y: 220, W: 880, H: 460}
		if changed := diffPixelCountInRect(midImg, afterImg, stageRect, 2); changed < 60 {
			t.Errorf("expected visible confetti pixels across stage, changed=%d", changed)
		}
	})

	// 2. Duel settlement truth table: strict PlayerScore > AIScore spawns confetti; draw & loss do NOT.
	for _, tc := range []struct {
		name       string
		player, ai int
		want       bool
	}{
		{"duel-win", 200, 120, true},
		{"duel-draw", 150, 150, false},
		{"duel-loss", 80, 190, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := newMinesweeperApp(7)
			app.memberID = MemberAnon
			app.game = NewGame(Difficulty{
				ID:           "duel-confetti-" + tc.name,
				Song:         "潜在表明",
				Gameplay:     GameplayDuel,
				Rows:         6,
				Cols:         6,
				Mines:        1,
				DuelDuration: 1,
				AIInterval:   10,
			}, true, MemberAnon, 7)
			app.game.FirstClick = true
			app.game.Status = StatusPlaying
			app.game.duel.PlayerScore = tc.player
			app.game.duel.AIScore = tc.ai

			tt := ui.NewTester(app.view, 1280, 900)
			tt.Advance(1100 * time.Millisecond)
			if !app.duelEnded {
				t.Fatal("expected duel settled")
			}
			got := len(app.confetti) > 0
			if got != tc.want {
				t.Fatalf("%s (%d vs %d): confetti=%v (count=%d), want %v", tc.name, tc.player, tc.ai, got, len(app.confetti), tc.want)
			}
			if tc.want {
				// Canvas wheel zoom still passes through while confetti falls.
				br, _ := tt.Find("决斗棋盘")
				beforeCell := app.board.Cell
				tt.Scroll(br.X+br.W/2, br.Y+br.H/2, 0, -120)
				if app.board.Cell == beforeCell {
					t.Fatal("confetti blocked wheel zoom pass-through on duel canvas")
				}
				tt.Advance(confettiDuration + 50*time.Millisecond)
				if len(app.confetti) != 0 {
					t.Fatalf("expected duel confetti pruned, remaining=%d", len(app.confetti))
				}
			}
		})
	}
}

func TestMemeUnlockGoldenStripFirstOnlyAndTimedDuel(t *testing.T) {
	// 1. Timed duel win unlocks MemeForever via recordMemeUnlock, pops golden strip once,
	// and never re-pops on a second timed duel win in the same run.
	app := newMinesweeperApp(42)
	setupDuelWin := func() *ui.Tester {
		app.memberID = MemberTomori
		app.game = NewGame(Difficulty{
			ID:           "duel-meme-strip",
			Song:         "潜在表明",
			Gameplay:     GameplayDuel,
			Rows:         5,
			Cols:         5,
			Mines:        1,
			DuelDuration: 1,
			AIInterval:   10,
		}, true, MemberTomori, 7)
		app.game.FirstClick = true
		app.game.Status = StatusPlaying
		app.game.duel.PlayerScore = 240
		app.game.duel.AIScore = 90
		app.duelEnded = false
		app.lastTick = time.Time{}
		return ui.NewTester(app.view, 1280, 900)
	}

	tt := setupDuelWin()
	tt.Advance(1100 * time.Millisecond)
	if !app.memeStrip.Active || len(app.memeStrip.Memes) != 1 || app.memeStrip.Memes[0].ID != MemeForever {
		t.Fatalf("expected golden meme strip for MemeForever on timed duel win, got %+v", app.memeStrip)
	}
	if !tt.HasText("名场面解锁") || !tt.HasText("★ 一辈子！") {
		t.Fatal("expected golden meme strip copy visible on timed duel win")
	}

	// Pass-through scroll over the golden meme strip on duel canvas.
	br, _ := tt.Find("决斗棋盘")
	beforeCell := app.board.Cell
	tt.Scroll(br.X+br.W/2, br.Y+28, 0, -120)
	if app.board.Cell == beforeCell {
		t.Fatal("golden meme strip blocked wheel zoom pass-through")
	}

	// Advance into the final 200ms exit slide (duel settled ~100ms before tt.Advance(1100ms) ended).
	tt.Advance(memeStripDuration - 220*time.Millisecond)
	if !app.memeStrip.Active {
		t.Fatal("expected golden meme strip still visible during exit slide")
	}
	if memeRect, ok := tt.Find("★ 一辈子！"); ok {
		wantMemeBg := memeStripColorsFor(app.darkMode).Bg
		if !rectContainsColor(tt.Image(), ui.Rect{X: memeRect.X - 4, Y: max(float32(0), memeRect.Y), W: memeRect.W + 8, H: memeRect.H}, wantMemeBg, 4) {
			t.Errorf("meme strip exit frame faded Bg away from opaque %s", hexOf(wantMemeBg))
		}
	}
	tt.Advance(260 * time.Millisecond)
	if app.memeStrip.Active {
		t.Fatal("expected golden meme strip expired")
	}

	// Second timed duel win in same run must NOT re-pop the commemorative strip.
	tt2 := setupDuelWin()
	tt2.Advance(1100 * time.Millisecond)
	if app.memeStrip.Active {
		t.Fatalf("repeat MemeForever unlock in same run must not re-pop strip: %+v", app.memeStrip)
	}

	// 2. Classic win in a fresh run unlocking multiple memes simultaneously (MemeForever + MemeAnonBest)
	// displays both without dropping either, and restarting + winning again does not re-pop.
	appMulti := newMinesweeperApp(7)
	appMulti.modeIdx = 1 // pure classic: no skill -> MemeForever + MemeAnonBest
	appMulti.game = makeCorridorGame(5, 5, false, MemberAnon, [][2]int{{3, 4}})
	for i := range appMulti.game.Cells {
		c := &appMulti.game.Cells[i]
		if !c.IsMine && !(c.Row == 4 && c.Col == 4) {
			appMulti.game.Reveal(c.Row, c.Col)
		}
	}
	ttMulti := ui.NewTester(appMulti.view, 1280, 900)
	ttMulti.Frame()
	r44, _ := ttMulti.Find("cell-4-4")
	ttMulti.ClickAt(r44.X+r44.W/2, r44.Y+r44.H/2)
	if !appMulti.memeStrip.Active || len(appMulti.memeStrip.Memes) != 2 {
		t.Fatalf("expected 2 unlocked memes in golden strip, got %+v", appMulti.memeStrip)
	}
	if !ttMulti.HasText("★ 一辈子！") || !ttMulti.HasText("★ 不愧是爱音大人") {
		t.Fatal("expected both multi-unlock meme titles visible in golden strip")
	}

	// Restart clears active strip without forgetting unlockedMemes.
	ttMulti.Key(0, ui.KeyR)
	if appMulti.memeStrip.Active {
		t.Fatal("expected reset to clear active meme strip")
	}
	if !appMulti.unlockedMemes[MemeForever] || !appMulti.unlockedMemes[MemeAnonBest] {
		t.Fatal("reset must preserve per-run unlockedMemes")
	}
	// Re-win same board -> neither meme re-pops the golden strip!
	appMulti.game = makeCorridorGame(5, 5, false, MemberAnon, [][2]int{{3, 4}})
	for i := range appMulti.game.Cells {
		c := &appMulti.game.Cells[i]
		if !c.IsMine && !(c.Row == 4 && c.Col == 4) {
			appMulti.game.Reveal(c.Row, c.Col)
		}
	}
	ttMulti.Frame()
	r44, _ = ttMulti.Find("cell-4-4")
	ttMulti.ClickAt(r44.X+r44.W/2, r44.Y+r44.H/2)
	if appMulti.memeStrip.Active {
		t.Fatalf("repeat win must not re-pop already unlocked memes: %+v", appMulti.memeStrip)
	}
}

func TestHaruhikageMaskFadeHalfBeatAndFrame0AntiLeak(t *testing.T) {
	for _, backend := range []string{"grid", "canvas"} {
		t.Run(backend+"/fade-half-beat-and-frame0-skip-no-leak", func(t *testing.T) {
			setup := func() (*minesweeperApp, *ui.Tester) {
				app := newMinesweeperApp(7)
				app.memberID = MemberTomori
				diff := Difficulty{
					ID:         "haruhikage-beat-test",
					Song:       "春日影",
					Gameplay:   GameplayClassic,
					Haruhikage: true,
					Rows:       6,
					Cols:       6,
					Mines:      1,
				}
				app.game = NewGame(diff, false, MemberTomori, 7)
				app.game.FirstClick = true
				app.game.Status = StatusPlaying
				app.game.Cell(0, 0).IsMine = true
				app.game.RecomputeAdjacency()
				viewFn := app.view
				if backend == "canvas" {
					viewFn = classicCanvasTestView(app)
				}
				tt := ui.NewTester(viewFn, 1280, 900)
				tt.Frame()
				if backend == "grid" {
					r00, _ := tt.Find("cell-0-0")
					tt.ClickAt(r00.X+r00.W/2, r00.Y+r00.H/2)
				} else {
					br, _ := tt.Find("经典棋盘")
					c00 := classicCanvasCellRect(br, app.board, 0, 0)
					tt.ClickAt(c00.X+c00.W/2, c00.Y+c00.H/2)
				}
				return app, tt
			}

			// Part A: Frame-0 scrim darkening + half-beat pause before core line emerges.
			appA, ttA := setup()
			if !appA.haruhikageIntro || appA.haruhikageAlert {
				t.Fatalf("expected haruhikageIntro on frame 0, intro=%v alert=%v", appA.haruhikageIntro, appA.haruhikageAlert)
			}
			if ttA.HasText("……为什么要演奏春日影？！") {
				t.Fatal("core questioning line must pause for a half-beat on frame 0")
			}
			pal := appA.paper()
			drama := haruhikageFor(pal.Dark)
			img0 := ttA.Image()
			wantFrame0Scrim := drama.Scrim.Alpha(0.52).Over(pal.Desk)
			if got := img0.At(1270, 890); !pixelNearColor(got, wantFrame0Scrim, 10) {
				r, g, b, _ := got.RGBA()
				t.Errorf("frame-0 scrim pixel = #%02X%02X%02X, want near %s", r>>8, g>>8, b>>8, hexOf(wantFrame0Scrim))
			}

			// Still paused just before half-beat (200ms).
			ttA.Advance(200 * time.Millisecond)
			if ttA.HasText("……为什么要演奏春日影？！") {
				t.Fatal("core questioning line appeared before half-beat delay")
			}

			// Past half-beat (260ms): core line emerges!
			ttA.Advance(60 * time.Millisecond)
			if !ttA.HasText("……为什么要演奏春日影？！") {
				t.Fatal("expected core questioning line visible after half-beat")
			}

			// Full scrim + full card after fade completes.
			ttA.Advance(haruhikageLineFade + 20*time.Millisecond)
			imgFull := ttA.Image()
			wantFullScrim := drama.Scrim.Over(pal.Desk)
			if got := imgFull.At(1270, 890); !pixelNearColor(got, wantFullScrim, 8) {
				r, g, b, _ := got.RGBA()
				t.Errorf("settled scrim pixel = #%02X%02X%02X, want near %s", r>>8, g>>8, b>>8, hexOf(wantFullScrim))
			}

			// Part B: Frame-0 click-to-skip at screen center (exact AlertDialog button zone!)
			// immediately skips intro, opens AlertDialog, and does NOT double-dismiss dialog or leak to board.
			appB, ttB := setup()
			if !appB.haruhikageIntro || appB.haruhikageAlert {
				t.Fatalf("expected haruhikageIntro active on frame 0, intro=%v alert=%v", appB.haruhikageIntro, appB.haruhikageAlert)
			}
			// Find exact position of AlertDialog's "对不起，我错了" button from a probe skip, or click screen center directly.
			ttB.ClickAt(640, 450)
			if appB.haruhikageIntro || !appB.haruhikageAlert {
				t.Fatalf("frame-0 center click must skip intro and leave AlertDialog open (no double-dismiss): intro=%v alert=%v",
					appB.haruhikageIntro, appB.haruhikageAlert)
			}
			if !ttB.HasText("对不起，我错了") || !ttB.HasText("一辈子重开") {
				t.Fatal("AlertDialog buttons missing after frame-0 skip")
			}
			// Also test clicking the exact AlertDialog primary/secondary button coordinates on frame 0!
			forgiveRect, ok := ttB.Find("对不起，我错了")
			if !ok {
				t.Fatal("forgive button rect not found")
			}
			appC, ttC := setup()
			ttC.ClickAt(forgiveRect.X+forgiveRect.W/2, forgiveRect.Y+forgiveRect.H/2)
			if appC.haruhikageIntro || !appC.haruhikageAlert {
				t.Fatalf("frame-0 click on AlertDialog button coords must NOT leak into AlertDialog: intro=%v alert=%v",
					appC.haruhikageIntro, appC.haruhikageAlert)
			}
		})
	}
}

func TestStageHighlightsMotionReducedAndThreeResetPaths(t *testing.T) {
	// 1. Mid-flight toggle to reduced motion (via system pref & 设置 popover) immediately
	// drops confetti, skill pulses, and knot bursts, snaps Haruhikage mask, and keeps
	// static quote strip & golden meme strip visible!
	t.Run("mid-flight-reduced-clears-motion-keeps-static-strips", func(t *testing.T) {
		app := newMinesweeperApp(23)
		app.memberID = MemberAnon
		app.game = fixedBoard(map[int]bool{0: true, 24: true}, 1, 23)
		app.game.BandMode = true
		app.game.MemberID = MemberAnon
		app.game.SkillUsesLeft = 1
		tt := ui.NewTester(app.view, 1280, 900)
		tt.Frame()

		// Winning skill activates quote strip, skill pulses, victory confetti, AND golden meme strip simultaneously!
		tt.Key(0, ui.KeyE)
		if !app.quoteStrip.Active || !app.memeStrip.Active || len(app.skillPulses) == 0 || len(app.confetti) == 0 {
			t.Fatalf("expected all highlights active before reduced toggle: quote=%v meme=%v pulses=%d confetti=%d",
				app.quoteStrip.Active, app.memeStrip.Active, len(app.skillPulses), len(app.confetti))
		}
		// Also enqueue a knot burst to verify it clears on the same frame too.
		app.enqueueKnotBurst(tt.Now(), 12)
		if len(app.knotBursts) != 1 {
			t.Fatal("expected active knot burst")
		}

		// System switches ReduceMotion=true mid-flight -> same-frame drop of motion effects, static strips kept!
		tt.SetPreferences(ui.Preferences{ReduceMotion: true})
		if !app.motionReducedActive {
			t.Fatal("expected motionReducedActive=true under motionSystem")
		}
		if len(app.confetti) != 0 || len(app.skillPulses) != 0 || len(app.knotBursts) != 0 {
			t.Fatalf("reduced motion must clear confetti/pulses/bursts same frame: confetti=%d pulses=%d bursts=%d",
				len(app.confetti), len(app.skillPulses), len(app.knotBursts))
		}
		if !app.quoteStrip.Active || !app.memeStrip.Active {
			t.Fatalf("reduced motion must keep static quote & meme strips visible: quote=%v meme=%v",
				app.quoteStrip.Active, app.memeStrip.Active)
		}

		// Manual 标准 overrides system ReduceMotion=true -> full motion restored.
		if err := tt.Click("设置"); err != nil {
			t.Fatalf("open 设置: %v", err)
		}
		if err := tt.Click("标准"); err != nil {
			t.Fatalf("select 标准: %v", err)
		}
		tt.Key(0, ui.KeyEscape)
		if app.motionReducedActive {
			t.Fatal("motionStandard must override system ReduceMotion=true")
		}
		app.spawnConfetti(tt.Now())
		if len(app.confetti) != confettiCount {
			t.Fatalf("expected full confetti under manual 标准 with system ReduceMotion=true, got %d", len(app.confetti))
		}

		// Switch to 减弱 via 设置 popover mid-flight -> clears confetti on same frame.
		if err := tt.Click("设置"); err != nil {
			t.Fatalf("reopen 设置: %v", err)
		}
		if err := tt.Click("减弱"); err != nil {
			t.Fatalf("select 减弱: %v", err)
		}
		if !app.motionReducedActive || len(app.confetti) != 0 {
			t.Fatalf("设置 减弱 must clear confetti same frame: reduced=%v confetti=%d", app.motionReducedActive, len(app.confetti))
		}
	})

	// 2. Reduced motion snaps Haruhikage mask immediately to full scrim + static line (no half-beat delay).
	t.Run("reduced-motion-haruhikage-immediate-static", func(t *testing.T) {
		app := newMinesweeperApp(7)
		app.motion = motionReduced
		app.applyPrefs(app.currentPrefs())
		app.game = NewGame(Difficulty{
			ID:         "haruhikage-reduced",
			Song:       "春日影",
			Gameplay:   GameplayClassic,
			Haruhikage: true,
			Rows:       5,
			Cols:       5,
			Mines:      1,
		}, false, MemberTomori, 7)
		app.game.FirstClick = true
		app.game.Status = StatusPlaying
		app.game.Cell(0, 0).IsMine = true
		app.game.RecomputeAdjacency()
		tt := ui.NewTester(app.view, 1280, 900)
		tt.Frame()

		r00, _ := tt.Find("cell-0-0")
		tt.ClickAt(r00.X+r00.W/2, r00.Y+r00.H/2)
		if !app.haruhikageIntro || !tt.HasText("……为什么要演奏春日影？！") {
			t.Fatalf("reduced motion should show static Haruhikage line immediately on frame 0: intro=%v", app.haruhikageIntro)
		}
	})

	// 3. All three reset paths (KeyR, 歌单 track switch, top-bar 乐队 Live/纯正经典 switch)
	// immediately clear quote strip, meme strip, confetti, skill pulses, knot bursts, AND duelPopups.
	for _, path := range []string{"keyR", "setlist", "modeSwitch"} {
		t.Run("reset_"+path, func(t *testing.T) {
			app := newMinesweeperApp(7)
			app.memberID = MemberAnon
			tt := ui.NewTester(app.view, 1280, 900)
			tt.Frame()

			now := tt.Now()
			app.showQuoteStrip(MemberAnon, "「测试签条」", now)
			app.recordMemeUnlock(MemeForever, now)
			app.enqueueSkillPulses(MemberAnon, []int{0, 1}, now)
			app.enqueueKnotBurst(now, 2)
			app.spawnConfetti(now)
			app.duelPopups = append(app.duelPopups, duelPopup{Row: 1, Col: 1, Text: "+75 收雷!", Kind: duelPopupClaim, Start: now})
			tt.Frame()

			switch path {
			case "keyR":
				tt.Key(0, ui.KeyR)
			case "setlist":
				if err := tt.Click("歌单"); err != nil {
					t.Fatalf("open 歌单: %v", err)
				}
				if err := tt.Click("碧天伴走"); err != nil {
					t.Fatalf("pick 碧天伴走: %v", err)
				}
			case "modeSwitch":
				if err := tt.Click("纯正经典"); err != nil {
					t.Fatalf("switch to 纯正经典: %v", err)
				}
			}

			if app.quoteStrip.Active || app.memeStrip.Active ||
				len(app.confetti) != 0 || len(app.skillPulses) != 0 ||
				len(app.knotBursts) != 0 || len(app.duelPopups) != 0 {
				t.Fatalf("%s did not clear all stage highlights & duelPopups: quote=%v meme=%v confetti=%d pulses=%d bursts=%d popups=%d",
					path, app.quoteStrip.Active, app.memeStrip.Active,
					len(app.confetti), len(app.skillPulses), len(app.knotBursts), len(app.duelPopups))
			}
			if !app.unlockedMemes[MemeForever] {
				t.Fatalf("%s must not wipe per-run unlockedMemes", path)
			}
		})
	}
}

// TestStageHighlightKeyframeSnapshots exports 30 deterministic PNG keyframes
// across small/default/large viewports and day/night themes for human review:
//
//	MYGO_HIGHLIGHT_SNAPSHOT_DIR=/tmp/kf go test -run '^TestStageHighlightKeyframeSnapshots$' .
func TestStageHighlightKeyframeSnapshots(t *testing.T) {
	dir := os.Getenv("MYGO_HIGHLIGHT_SNAPSHOT_DIR")
	if dir == "" {
		t.Skip("set MYGO_HIGHLIGHT_SNAPSHOT_DIR to render stage highlight keyframe snapshots")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	type keyframeSpec struct {
		name   string
		render func(t *testing.T, dark bool, width, height int) *image.RGBA
	}

	specs := []keyframeSpec{
		{
			name: "quote-strip-skill-targets",
			render: func(t *testing.T, dark bool, width, height int) *image.RGBA {
				app := newMinesweeperApp(99)
				app.darkMode = dark
				app.memberID = MemberAnon
				app.diffIdx = 0
				app.resetGame(true)
				pinStartQuote(app)
				app.game = NewGame(Difficulty{
					ID:           "easy",
					Label:        "羽丘天文部",
					Song:         "迷星叫",
					Subtitle:     "9×9 · 10雷",
					Gameplay:     GameplayClassic,
					Rows:         9,
					Cols:         9,
					Mines:        10,
					SkillCharges: 2,
				}, true, MemberAnon, 99)

				tt := ui.NewTester(app.view, width, height)
				tt.SetScale(2)
				tt.Frame()
				r44, ok := tt.Find("cell-4-4")
				if !ok {
					t.Fatal("cell-4-4 missing")
				}
				tt.ClickAt(r44.X+r44.W/2, r44.Y+r44.H/2)
				tt.Advance(550 * time.Millisecond)
				tt.Key(0, ui.KeyE)
				// Freeze at t=+180ms: quote strip is in full-opacity hold and both
				// Anon target cells show expanding pink fluorescent rings.
				tt.Advance(180 * time.Millisecond)
				tt.Frame()
				return tt.Image()
			},
		},
		{
			name: "knot-burst-pulse",
			render: func(t *testing.T, dark bool, width, height int) *image.RGBA {
				app := newMinesweeperApp(7)
				app.darkMode = dark
				app.memberID = MemberTomori
				app.diffIdx = 0
				app.resetGame(true)
				pinStartQuote(app)
				app.game = makeCorridorGame(9, 9, true, MemberTomori, [][2]int{{4, 5}, {1, 1}, {7, 7}})
				app.game.Difficulty.Song = "迷星叫"
				app.game.Difficulty.Subtitle = "9×9 · 10雷"
				// Pre-unlock MemeNeverHappy so the knot burst shot stays uncluttered by a meme strip.
				app.unlockedMemes[MemeNeverHappy] = true

				tt := ui.NewTester(app.view, width, height)
				tt.SetScale(2)
				tt.Frame()
				r44, _ := tt.Find("cell-4-4")
				tt.ClickAt(r44.X+r44.W/2, r44.Y+r44.H/2)
				tt.Advance(550 * time.Millisecond)
				r45, _ := tt.Find("cell-4-5")
				tt.ClickAt(r45.X+r45.W/2, r45.Y+r45.H/2)
				// Freeze at t=+110ms: outward pencil scribbles and red-ink bloom ring at peak visibility.
				tt.Advance(110 * time.Millisecond)
				tt.Frame()
				return tt.Image()
			},
		},
		{
			name: "victory-confetti",
			render: func(t *testing.T, dark bool, width, height int) *image.RGBA {
				app := newMinesweeperApp(7)
				app.darkMode = dark
				app.memberID = MemberTomori
				app.diffIdx = 0
				app.resetGame(true)
				pinStartQuote(app)
				app.game = NewGame(Difficulties[0], true, MemberTomori, 7)
				// Reveal all safe cells except one corner cell so clicking it finishes cleanly.
				app.game.Reveal(4, 4)
				lastRow, lastCol := -1, -1
				for i := range app.game.Cells {
					c := &app.game.Cells[i]
					if !c.IsMine && !c.Revealed {
						if lastRow >= 0 {
							app.game.Reveal(lastRow, lastCol)
						}
						lastRow, lastCol = c.Row, c.Col
					}
				}
				// Pre-unlock win memes so this keyframe showcases pure five-color confetti & '!'.
				app.unlockedMemes[MemeForever] = true
				app.unlockedMemes[MemeAnonBest] = true

				tt := ui.NewTester(app.view, width, height)
				tt.SetScale(2)
				tt.Frame()
				rlast, ok := tt.Find(fmt.Sprintf("cell-%d-%d", lastRow, lastCol))
				if !ok {
					t.Fatalf("final cell-(%d,%d) missing", lastRow, lastCol)
				}
				tt.ClickAt(rlast.X+rlast.W/2, rlast.Y+rlast.H/2)
				// Freeze at t=+440ms: all 32 five-color paper scraps and hand-drawn '!' cascade across the stage.
				tt.Advance(440 * time.Millisecond)
				tt.Frame()
				return tt.Image()
			},
		},
		{
			name: "meme-unlock-strip",
			render: func(t *testing.T, dark bool, width, height int) *image.RGBA {
				app := newMinesweeperApp(7)
				app.darkMode = dark
				app.memberID = MemberAnon
				app.diffIdx = 0
				app.resetGame(true)
				pinStartQuote(app)
				app.game = NewGame(Difficulties[0], true, MemberAnon, 7)
				app.game.Reveal(4, 4)
				lastRow, lastCol := -1, -1
				for i := range app.game.Cells {
					c := &app.game.Cells[i]
					if !c.IsMine && !c.Revealed {
						if lastRow >= 0 {
							app.game.Reveal(lastRow, lastCol)
						}
						lastRow, lastCol = c.Row, c.Col
					}
				}

				tt := ui.NewTester(app.view, width, height)
				tt.SetScale(2)
				tt.Frame()
				rlast, _ := tt.Find(fmt.Sprintf("cell-%d-%d", lastRow, lastCol))
				tt.ClickAt(rlast.X+rlast.W/2, rlast.Y+rlast.H/2)
				// Freeze at t=+1850ms: confetti just settled (1800ms) while the golden
				// commemorative strip holds at full clarity showing both unlocked memes.
				tt.Advance(1850 * time.Millisecond)
				tt.Frame()
				return tt.Image()
			},
		},
		{
			name: "haruhikage-mask-beat",
			render: func(t *testing.T, dark bool, width, height int) *image.RGBA {
				app := stageDuelScene(dark)
				tt := ui.NewTester(app.view, width, height)
				tt.SetScale(2)
				elapsed := app.game.duel.Elapsed
				app.game.duel.NextAI = 1e9
				tt.Advance(time.Duration(1200-tt.Now().UnixMilli()%1200) * time.Millisecond)
				app.game.duel.Elapsed = elapsed
				app.elapsed = int(elapsed)
				app.lastTick = tt.Now()
				app.board.center(app.game, app.game.Rows/2, app.game.Cols/2-2)
				// Stage the overlay right after the 250ms half-beat pause (t=380ms):
				// full darkened scrim with the questioning card emerging cleanly.
				app.haruhikageIntro = true
				app.haruhikageIntroStart = tt.Now().Add(-380 * time.Millisecond)
				app.haruhikageAlert = false
				tt.Frame()
				tt.Frame()
				return tt.Image()
			},
		},
	}

	for _, spec := range specs {
		for _, dark := range []bool{false, true} {
			suffix := "day"
			if dark {
				suffix = "night"
			}
			for _, vp := range snapshotViewports {
				img := spec.render(t, dark, vp.width, vp.height)
				name := fmt.Sprintf("%s-%s-%s.png", spec.name, vp.name, suffix)
				saveTesterPNG(t, img, filepath.Join(dir, name))
			}
		}
	}
}
