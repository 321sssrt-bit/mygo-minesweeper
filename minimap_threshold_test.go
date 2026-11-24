package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func formerMinimapRect(v *boardView) ui.Rect {
	s := min(minimapSize, max(float32(20), min(v.Width, v.Height)-minimapMargin*2))
	return ui.Rect{
		X: v.Width - minimapMargin - s,
		Y: v.Height - minimapMargin - s,
		W: s,
		H: s,
	}
}

func makeThresholdBoard(rows, cols int, isDuel bool, dark bool) (*minesweeperApp, string) {
	app := newMinesweeperApp(42)
	app.darkMode = dark
	label := "经典棋盘"
	if isDuel {
		label = "决斗棋盘"
		app.diffIdx = findDifficultyIdx("duel")
		app.game = NewGame(Difficulty{
			ID:           fmt.Sprintf("threshold-duel-%dx%d", rows, cols),
			Song:         "春日影",
			Gameplay:     GameplayDuel,
			Rows:         rows,
			Cols:         cols,
			Mines:        2,
			SkillCharges: 2,
			DuelDuration: 300,
			AIInterval:   1e9,
		}, true, MemberTomori, 42)
		app.game.duel.NextAI = 1e9
	} else {
		app.game = NewGame(Difficulty{
			ID:           fmt.Sprintf("threshold-classic-%dx%d", rows, cols),
			Song:         "诗超绊",
			Gameplay:     GameplayClassic,
			Rows:         rows,
			Cols:         cols,
			Mines:        2,
			SkillCharges: 2,
		}, true, MemberTomori, 42)
	}
	app.game.FirstClick = true
	app.game.Status = StatusPlaying
	app.game.Cell(0, 0).IsMine = true
	app.game.Cell(0, 1).IsMine = true
	app.game.RecomputeAdjacency()
	return app, label
}

func TestMinimapThresholdVisibilityAndBounds(t *testing.T) {
	cases := []struct {
		name        string
		rows, cols  int
		isDuel      bool
		useTestView bool
		wantMinimap bool
	}{
		{"classic_30x30_canvas", 30, 30, false, true, false},
		{"classic_31x31", 31, 31, false, false, false},
		{"classic_40x40", 40, 40, false, false, false},
		{"classic_49x49", 49, 49, false, false, false},
		{"duel_30x30", 30, 30, true, false, false},
		{"duel_49x49", 49, 49, true, false, false},
		{"classic_50x49", 50, 49, false, false, true},
		{"classic_49x50", 49, 50, false, false, true},
		{"classic_50x50", 50, 50, false, false, true},
		{"classic_100x100", 100, 100, false, false, true},
		{"duel_50x49", 50, 49, true, false, true},
		{"duel_49x50", 49, 50, true, false, true},
		{"duel_50x50", 50, 50, true, false, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app, label := makeThresholdBoard(tc.rows, tc.cols, tc.isDuel, false)
			viewFn := app.view
			if tc.useTestView {
				viewFn = classicCanvasTestView(app)
			}
			tt := ui.NewTester(viewFn, 960, 640)
			tt.Frame()
			if _, ok := tt.Find(label); !ok {
				t.Fatalf("%q not found on screen", label)
			}

			v, g := app.board, app.game
			mini := v.minimapBoundsFor(g)
			hasRect := mini.W > 0 && mini.H > 0
			if hasRect != tc.wantMinimap {
				t.Fatalf("%dx%d duel=%v minimapBoundsFor=%+v, want present=%v",
					tc.rows, tc.cols, tc.isDuel, mini, tc.wantMinimap)
			}

			ghost := formerMinimapRect(v)
			centerHit := v.minimapFor(g, ghost.X+ghost.W/2, ghost.Y+ghost.H/2)
			if centerHit != tc.wantMinimap {
				t.Fatalf("%dx%d duel=%v minimapFor(bottom-right)=%v, want %v",
					tc.rows, tc.cols, tc.isDuel, centerHit, tc.wantMinimap)
			}
			if v.minimapFor(g, 0, 0) {
				t.Fatalf("%dx%d duel=%v minimapFor(0,0) unexpectedly returned true", tc.rows, tc.cols, tc.isDuel)
			}

			maxX, maxY := v.panLimit(g)
			baseMaxX := max(float32(0), float32(g.Cols)*v.Cell-v.Width)
			baseMaxY := max(float32(0), float32(g.Rows)*v.Cell-v.Height)
			if !tc.wantMinimap {
				wantY := baseMaxY
				if !tc.isDuel {
					tb := v.toolbarBounds()
					if tb.W > 0 && tb.H > 0 {
						wantY = max(wantY, float32(g.Rows)*v.Cell-max(float32(0), tb.Y-canvasToolbarMargin))
					}
				}
				if maxX != baseMaxX || maxY != wantY {
					t.Fatalf("%dx%d added minimap overscroll: got (%v,%v), want (%v,%v)",
						tc.rows, tc.cols, maxX, maxY, baseMaxX, wantY)
				}
			} else if !tc.isDuel {
				if maxX <= baseMaxX || maxY <= baseMaxY {
					t.Fatalf("%dx%d classic >=50 lost minimap overscroll: got (%v,%v), base (%v,%v)",
						tc.rows, tc.cols, maxX, maxY, baseMaxX, baseMaxY)
				}
			}
		})
	}
}

func TestMinimapBelow50CanvasOmitsMinimapAndActsAsBoard(t *testing.T) {
	for _, isDuel := range []bool{false, true} {
		modeName := "classic-49x49"
		if isDuel {
			modeName = "duel-49x49"
		}
		for _, dark := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/dark=%v", modeName, dark), func(t *testing.T) {
				app, label := makeThresholdBoard(49, 49, isDuel, dark)
				tt := ui.NewTester(app.view, 960, 640)
				tt.SetScale(2)
				tt.UseScreenReader()
				tt.Frame()

				board, ok := tt.Find(label)
				if !ok {
					t.Fatalf("%q not found", label)
				}
				v, g := app.board, app.game
				// Place the top-left of the 49x49 board at (0,0) so cells span
				// well past the 960x640 canvas right across the bottom-right corner.
				v.X, v.Y = 0, 0
				v.clamp(g)
				tt.Frame()

				ghost := formerMinimapRect(v)
				// Pick three distinct, mutually non-adjacent cells inside the former minimap rect
				// so flagging (r2,c2) and (r3,c3) does not turn (r1,c1) into a chordable hint center.
				p1x, p1y := ghost.X+ghost.W*0.18, ghost.Y+ghost.H*0.20
				p2x, p2y := ghost.X+ghost.W*0.82, ghost.Y+ghost.H*0.20
				p3x, p3y := ghost.X+ghost.W*0.50, ghost.Y+ghost.H*0.82
				r1, c1, ok1 := v.cellAt(g, p1x, p1y)
				r2, c2, ok2 := v.cellAt(g, p2x, p2y)
				r3, c3, ok3 := v.cellAt(g, p3x, p3y)
				if !ok1 || !ok2 || !ok3 || (r1 == r2 && c1 == c2) || (r1 == r3 && c1 == c3) || (r2 == r3 && c2 == c3) ||
					(absInt(r2-r1) <= 1 && absInt(c2-c1) <= 1) || (absInt(r3-r1) <= 1 && absInt(c3-c1) <= 1) {
					t.Fatalf("failed to pick 3 distinct non-adjacent cells in former minimap area: (%d,%d,%v) (%d,%d,%v) (%d,%d,%v)",
						r1, c1, ok1, r2, c2, ok2, r3, c3, ok3)
				}
				// Place a mine at (r1-1, c1-1) so (r1, c1) is a numbered safe cell (Adjacent=1),
				// avoiding flood-fill and letting us assert its accessibility value "1" after reveal.
				g.Cell(r1-1, c1-1).IsMine = true
				g.RecomputeAdjacency()
				tt.Frame()

				// 1. Rendered pixels inside former minimap area show board paper slips, not a minimap card.
				pal := paperFor(dark)
				tile1 := classicCanvasCellRect(board, v, r1, c1)
				gotCenter := sampleCanvasPixel(tt.Image(), tile1.X+tile1.W/2, tile1.Y+tile1.H/2, 2)
				assertColorNear(t, "former minimap area renders unrevealed slip", gotCenter, pal.Slip)
				if isDuel {
					ghostScreen := ui.Rect{X: board.X + ghost.X, Y: board.Y + ghost.Y, W: ghost.W, H: ghost.H}
					if n := countMatchingPixels(tt.Image(), ghostScreen, 2, duelMinimapPalette.Bg, 10); n != 0 {
						t.Fatalf("duel 49x49 unexpectedly drew %d duel minimap background pixels", n)
					}
				} else {
					mc := classicMinimapFor(dark)
					ghostBorderProbe := ui.Rect{X: board.X + ghost.X, Y: board.Y + ghost.Y, W: ghost.W, H: 4}
					if n := countMatchingPixels(tt.Image(), ghostBorderProbe, 2, mc.Border, 8); n > 0 {
						t.Fatalf("classic 49x49 unexpectedly drew %d classic minimap border pixels", n)
					}
				}

				// 2. Visibility and hover treat the former minimap area as normal board space.
				if !v.shows(g, r1, c1) {
					t.Fatalf("v.shows(%d,%d) = false in former minimap area, want true", r1, c1)
				}
				tx, ty := v.visibleTarget(g, r1, c1, v.Cell)
				wantX, wantY := v.centerTarget(g, r1, c1, v.Cell)
				if tx != wantX || ty != wantY {
					t.Fatalf("visibleTarget(%d,%d) = (%v,%v), want centerTarget (%v,%v)", r1, c1, tx, ty, wantX, wantY)
				}
				tt.Move(tile1.X+tile1.W/2, tile1.Y+tile1.H/2)
				tt.Frame()
				if v.HoverRow != r1 || v.HoverCol != c1 {
					t.Fatalf("hover in former minimap area = (%d,%d), want (%d,%d)", v.HoverRow, v.HoverCol, r1, c1)
				}

				// 3. Right click in former minimap area flags the board cell immediately.
				camX, camY := v.X, v.Y
				tile2 := classicCanvasCellRect(board, v, r2, c2)
				tt.RightClickAt(tile2.X+tile2.W/2, tile2.Y+tile2.H/2)
				if !g.Cell(r2, c2).Flagged || g.FlagsPlaced != 1 {
					t.Fatalf("right click in former minimap area did not flag (%d,%d): flagged=%v total=%d",
						r2, c2, g.Cell(r2, c2).Flagged, g.FlagsPlaced)
				}

				// 4. Flag-mode left click in former minimap area flags the cell without gliding.
				tt.Key(0, ui.KeyF)
				tile3 := classicCanvasCellRect(board, v, r3, c3)
				tt.ClickAt(tile3.X+tile3.W/2, tile3.Y+tile3.H/2)
				if v.gliding() || v.X != camX || v.Y != camY {
					t.Fatalf("flag-mode left click in former minimap area moved camera: gliding=%v pos=(%v,%v)",
						v.gliding(), v.X, v.Y)
				}
				if !g.Cell(r3, c3).Flagged || g.FlagsPlaced != 2 {
					t.Fatalf("flag-mode left click in former minimap area did not flag (%d,%d): flagged=%v total=%d",
						r3, c3, g.Cell(r3, c3).Flagged, g.FlagsPlaced)
				}
				tt.Key(0, ui.KeyF)

				// 5. Left click in former minimap area reveals the board cell without gliding camera,
				// paints bare paper (no minimap card on top), and exposes its accessibility value on hover.
				tt.ClickAt(tile1.X+tile1.W/2, tile1.Y+tile1.H/2)
				if v.gliding() || v.X != camX || v.Y != camY {
					t.Fatalf("left click in former minimap area moved camera: gliding=%v pos=(%v,%v) want (%v,%v)",
						v.gliding(), v.X, v.Y, camX, camY)
				}
				if !g.Cell(r1, c1).Revealed || g.Moves != 1 {
					t.Fatalf("left click in former minimap area did not reveal (%d,%d): revealed=%v moves=%d",
						r1, c1, g.Cell(r1, c1).Revealed, g.Moves)
				}
				tt.Move(tile1.X+tile1.W/2, tile1.Y+tile1.H/2)
				tt.Frame()
				if info, ok := tt.Access(label); !ok || info.Value != "1" || info.Description != "1" {
					t.Fatalf("accessibility hover value in former minimap area = (%q,%q) ok=%v, want (\"1\",\"1\")",
						info.Value, info.Description, ok)
				}
				if !isDuel {
					tt.Advance(rippleDuration + 50*time.Millisecond)
					gotRevealed := sampleCanvasPixel(tt.Image(), tile1.X+2, tile1.Y+2, 2)
					assertColorNear(t, "former minimap area reveals hovered number paper", gotRevealed, pal.Ink.Alpha(0.05).Over(pal.Board))
				}
			})
		}
	}
}

func TestMinimapAt50PreservesDrawingAndInteraction(t *testing.T) {
	for _, isDuel := range []bool{false, true} {
		name := "classic-50x50"
		if isDuel {
			name = "duel-50x50"
		}
		t.Run(name, func(t *testing.T) {
			app, label := makeThresholdBoard(50, 50, isDuel, false)
			tt := ui.NewTester(app.view, 960, 640)
			tt.SetScale(2)
			tt.Frame()

			board, ok := tt.Find(label)
			if !ok {
				t.Fatalf("%q not found", label)
			}
			v, g := app.board, app.game
			v.X, v.Y = 0, 0
			v.clamp(g)
			tt.Frame()

			mini := v.minimapBoundsFor(g)
			if mini.W <= 0 || mini.H <= 0 {
				t.Fatalf("expected non-empty minimap on 50x50, got %+v", mini)
			}

			// Rendered pixels inside the minimap match the minimap background.
			wantBg := classicMinimapFor(false).Bg
			if isDuel {
				wantBg = duelMinimapPalette.Bg.Alpha(0.94).Over(paperFor(false).Slip)
			}
			probe := ui.Rect{
				X: board.X + mini.X + 8,
				Y: board.Y + mini.Y + 8,
				W: mini.W - 16,
				H: mini.H - 16,
			}
			if n := countMatchingPixels(tt.Image(), probe, 2, wantBg, 24); n < 100 {
				t.Fatalf("50x50 minimap background missing: matched only %d px near %s", n, hexOf(wantBg))
			}

			// Hover over minimap clears board cell hover.
			cx, cy := board.X+mini.X+mini.W/2, board.Y+mini.Y+mini.H/2
			tt.Move(cx, cy)
			tt.Frame()
			if v.HoverRow != -1 || v.HoverCol != -1 {
				t.Fatalf("hover over 50x50 minimap leaked cell (%d,%d)", v.HoverRow, v.HoverCol)
			}

			// Right click over minimap does not flag underlying cell.
			tt.RightClickAt(cx, cy)
			if g.FlagsPlaced != 0 {
				t.Fatalf("right click on 50x50 minimap flagged underlying cell: flags=%d", g.FlagsPlaced)
			}

			// Left click near bottom-right of minimap glides camera without revealing.
			tt.ClickAt(board.X+mini.X+mini.W-6, board.Y+mini.Y+mini.H-6)
			if !v.gliding() || g.Moves != 0 {
				t.Fatalf("left click on 50x50 minimap: gliding=%v moves=%d, want gliding=true moves=0", v.gliding(), g.Moves)
			}
		})
	}
}

func stageMinimapBelow50Scene(dark bool) *minesweeperApp {
	app := newMinesweeperApp(20261116)
	app.darkMode = dark
	app.memberID = MemberSoyorin
	app.allowQuestion = true
	diff := TrackUtakotoba
	diff.ID = "minimap-below50"
	diff.Rows = 49
	diff.Cols = 49
	diff.Mines = 380
	app.game = NewGame(diff, true, MemberSoyorin, 20261116)
	pinStartQuote(app)
	g := app.game
	g.Reveal(classicCanvasStageRow, classicCanvasStageCol)
	for row := classicCanvasStageRow - 6; row <= classicCanvasStageRow+6; row++ {
		for col := classicCanvasStageCol - 8; col <= classicCanvasStageCol+8; col++ {
			c := g.Cell(row, col)
			if c != nil && !c.Revealed && !c.IsMine && c.Adjacent == 0 && (row*g.Cols+col)%5 == 0 {
				g.Reveal(row, col)
			}
		}
	}
	flagged := 0
	shielded := false
	for row := classicCanvasStageRow - 6; row <= classicCanvasStageRow+6; row++ {
		for col := classicCanvasStageCol - 8; col <= classicCanvasStageCol+8; col++ {
			c := g.Cell(row, col)
			if c == nil || !c.IsMine || c.Revealed || c.Flagged || !hasRevealedNeighbour(g, row, col) {
				continue
			}
			if flagged < 6 {
				g.ToggleFlag(row, col, false)
				flagged++
			} else if !shielded {
				g.Reveal(row, col)
				shielded = true
			}
		}
	}
	hoverRow, hoverCol := -1, -1
	for row := classicCanvasStageRow - 5; row <= classicCanvasStageRow+5 && hoverRow < 0; row++ {
		for col := classicCanvasStageCol - 7; col <= classicCanvasStageCol+7; col++ {
			c := g.Cell(row, col)
			if c != nil && c.Revealed && c.Adjacent > 0 && g.InspectNumberHover(row, col, false).Kind == HintChordable {
				hoverRow, hoverCol = row, col
				break
			}
		}
	}
	if hoverRow < 0 {
		hoverRow, hoverCol = chordableNumber(g)
	}
	app.classicHoverRow, app.classicHoverCol = hoverRow, hoverCol
	hintTargets := g.InspectNumberHover(hoverRow, hoverCol, false).Targets
	questioned := 0
	for row := classicCanvasStageRow - 5; row <= classicCanvasStageRow+5 && questioned < 2; row++ {
		for col := classicCanvasStageCol - 7; col <= classicCanvasStageCol+7 && questioned < 2; col++ {
			c := g.Cell(row, col)
			idx := row*g.Cols + col
			if c != nil && !c.IsMine && !c.Revealed && !c.Flagged && !c.Questioned &&
				hasRevealedNeighbour(g, row, col) && !containsIndex(hintTargets, idx) {
				g.ToggleFlag(row, col, true)
				g.ToggleFlag(row, col, true)
				questioned++
			}
		}
	}
	return app
}

// TestSnapshotMinimapBelow50 renders the 49x49 classic canvas scene (just below
// the 50-cell minimap threshold) across all viewports in day and night themes
// when MYGO_SNAPSHOT_DIR is set.
func TestSnapshotMinimapBelow50(t *testing.T) {
	dir := os.Getenv("MYGO_SNAPSHOT_DIR")
	if dir == "" {
		t.Skip("set MYGO_SNAPSHOT_DIR to render snapshots")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, dark := range []bool{false, true} {
		suffix := "day"
		if dark {
			suffix = "night"
		}
		for _, vp := range snapshotViewports {
			app := stageMinimapBelow50Scene(dark)
			name := fmt.Sprintf("minimap-below50-%s-%s.png", vp.name, suffix)
			writeSnapshot(t, app, true, vp.width, vp.height, filepath.Join(dir, name))
		}
	}
}
