package main

import (
	"fmt"
	"math/rand/v2"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// BenchmarkCanvasFrame renders the shipped view, including the canvas minimap.
// Use -run='^$' -bench='^BenchmarkCanvasFrame$' -benchmem -benchtime=1x for a smoke run.
func BenchmarkCanvasFrame(b *testing.B) {
	for _, track := range []string{TrackMayoiuta.ID, "duel"} {
		for _, density := range []struct {
			name       string
			attemptPct int
		}{{"sparse", 1}, {"dense", 50}} {
			for _, cellSize := range []int{10, 26, 42} {
				for _, motion := range []string{"still", "pan"} {
					name := fmt.Sprintf("%s/%s/cell%d/%s", track, density.name, cellSize, motion)
					b.Run(name, func(b *testing.B) {
						benchmarkCanvasFrame(b, track, density.attemptPct, float32(cellSize), motion == "pan")
					})
				}
			}
		}
	}
}

func benchmarkCanvasFrame(b *testing.B, track string, attemptPct int, cellSize float32, pan bool) {
	app := newMinesweeperApp(36)
	app.diffIdx = findDifficultyIdx(track)
	app.darkMode = false
	app.resetGame(true)
	pinStartQuote(app)
	g, v := app.game, app.board
	openPct := stageCanvasBenchmark(g, attemptPct)
	if g.duel != nil {
		g.duel.NextAI = g.duel.Duration + 1
		// The last-move markers pulse and make Tester.Frame settle multiple frames.
		g.duel.LastCell, g.duel.LastAI = -1, -1
		app.duelNoticeAt = time.Now()
	}

	tt := ui.NewTester(app.view, 1280, 900)
	tt.SetScale(2)
	v.Cell = cellSize
	v.centerBoard(g)
	tt.Frame()
	if tt.FramePending() {
		b.Fatal("canvas benchmark still has pending frames after warmup")
	}
	anchorX, anchorY := v.X, v.Y
	frame := 0
	b.ReportAllocs()
	for b.Loop() {
		if pan {
			// Reverse every 24 frames to stay over the same staged region.
			frame = (frame + 1) % 48
			step := float32(min(frame, 48-frame))
			v.X, v.Y = anchorX+3*step, anchorY+2*step
		}
		tt.Frame()
	}
	b.ReportMetric(float64(b.Elapsed())/float64(b.N)/float64(time.Millisecond), "ms/frame")
	b.ReportMetric(openPct, "staged-open-%")
}

func stageCanvasBenchmark(g *Game, attemptPct int) float64 {
	// The full classic board and a same-sized duel skirmish use real reveals
	// so adjacency, owner colors and minimap progress agree with the cells.
	const side = 100
	centerRow, centerCol := g.Rows/2, g.Cols/2
	firstRow, firstCol := centerRow-side/2, centerCol-side/2
	g.Reveal(centerRow, centerCol)
	rng := rand.New(rand.NewPCG(36, 1))
	for row := firstRow; row < firstRow+side; row++ {
		for col := firstCol; col < firstCol+side; col++ {
			cell := g.Cell(row, col)
			if rng.IntN(100) >= attemptPct || cell.IsMine || cell.Revealed {
				continue
			}
			if g.duel != nil && col > centerCol {
				g.revealDuel(row, col, DuelAI)
			} else {
				g.Reveal(row, col)
			}
		}
	}
	opened := 0
	for row := firstRow; row < firstRow+side; row++ {
		for col := firstCol; col < firstCol+side; col++ {
			if g.Cell(row, col).Revealed {
				opened++
			}
		}
	}
	return 100 * float64(opened) / (side * side)
}

func BenchmarkCanvasActiveFrameByScale(b *testing.B) {
	for _, tc := range activeCanvasTracks {
		b.Run(tc.name, func(b *testing.B) {
			benchmarkActiveFrame(b, tc.track, true)
		})
	}
}

var activeCanvasTracks = []struct{ name, track string }{
	{"50x50", TrackUtakotoba.ID},
	{"100x100", TrackMayoiuta.ID},
	{"500x500", TrackSenzaihyoumei.ID},
	{"1000x1000", "duel"},
}

func BenchmarkClassicGrid30x30ActiveFrame(b *testing.B) {
	benchmarkActiveFrame(b, TrackKageiromai.ID, false)
}

func benchmarkActiveFrame(b *testing.B, track string, canvas bool) {
	app := newMinesweeperApp(36)
	app.diffIdx = findDifficultyIdx(track)
	app.resetGame(true)
	pinStartQuote(app)
	g, v := app.game, app.board
	openPct := stageActiveBenchmark(g)
	row, col := g.Rows/2, g.Cols/2
	if d := g.duel; d != nil {
		d.NextAI = d.Duration + 1
		d.LastCell, d.LastAI = -1, -1
	}

	center := row*g.Cols + col
	samples := []struct {
		idx        int
		age, delay time.Duration
		stage      tilePeelStage
	}{
		{center - 3, 0, rippleStepDelay, tilePeelCovered},
		{center - 4, peelDuration / 2, 0, tilePeelLifting},
		{center - 5, (peelDuration + rippleDuration) / 2, 0, tilePeelWashOnly},
	}

	// Frame settles up to 20 redraws at the same instant for live animations.
	// One test-clock deadline per iteration rebuilds and paints the active view
	// once, before another display refresh is due.
	const step = time.Millisecond
	active := false
	builds := 0
	var anchorX, anchorY float32
	var phase int
	tt := ui.NewTester(func(c *ui.Context) {
		now := c.Now()
		if g.duel != nil {
			app.lastTick, app.duelNoticeAt = now, now
		}
		if active {
			phase = (phase + 1) % 32
			offset := min(phase, 32-phase)
			if canvas {
				pan := float32(3 * (offset - 8))
				v.X, v.Y = anchorX+pan, anchorY+pan
				v.HoverX = (float32(col)+0.5)*v.Cell - v.X
				v.HoverY = (float32(row)+0.5)*v.Cell - v.Y
				v.updateHover(g)
			}
			// Sweep within the same phase windows, so the rasterizer sees
			// changing pixels even when the classic grid is stationary.
			ageOffset := time.Duration(offset) * time.Millisecond
			for _, sample := range samples {
				ripple := app.ripples[sample.idx]
				ripple.Start = now.Add(-sample.age - ageOffset)
				app.ripples[sample.idx] = ripple
			}
			for idx, bandage := range app.bandages {
				bandage.Start = now.Add(-35*time.Millisecond - ageOffset)
				app.bandages[idx] = bandage
			}
			c.After(step)
			builds++
		}
		app.view(c)
	}, 1280, 900)
	tt.SetScale(2)
	if canvas {
		v.Cell = 26
		v.centerBoard(g)
		anchorX, anchorY = v.X, v.Y
		v.Pressed, v.Dragging, v.HasHover = true, true, true
	} else {
		cell, ok := tt.Find(fmt.Sprintf("cell-%d-%d", row, col))
		if !ok {
			b.Fatal("staged grid number is missing")
		}
		tt.Move(cell.X+cell.W/2, cell.Y+cell.H/2)
		tt.Advance(2 * time.Millisecond)
	}

	app.ripples = make(map[int]tileRipple, len(samples))
	for _, sample := range samples {
		app.ripples[sample.idx] = tileRipple{Delay: sample.delay, Color: memberImage(app.memberID)}
	}
	if !canvas {
		app.bandages = map[int]bandageAnim{
			(row+2)*g.Cols + col - 3: {Kind: bandageBounce},
			(row+2)*g.Cols + col - 4: {Kind: bandagePeel},
		}
	}

	active = true
	tt.Frame()
	baseline := builds
	requireSilentSound(b, app)
	hint := app.classicHoverHint
	if canvas {
		hint = v.LastHoverHint
	}
	if hint.Kind != HintChordable || len(hint.Targets) == 0 {
		b.Fatalf("staged hover hint = %+v, want chordable with targets", hint)
	}
	for _, sample := range samples {
		if got := app.tilePeelAt(sample.idx, tt.Now()).stage; got != sample.stage {
			b.Fatalf("staged peel at %d = %v, want %v", sample.idx, got, sample.stage)
		}
	}

	b.ReportAllocs()
	for b.Loop() {
		tt.Advance(step)
	}
	if got := builds - baseline; got != b.N {
		b.Fatalf("built %d views in %d measured frames", got, b.N)
	}
	if len(app.ripples) != len(samples) || (!canvas && len(app.bandages) != 2) {
		b.Fatal("staged effects expired during measurement")
	}
	b.ReportMetric(float64(b.Elapsed())/float64(b.N)/float64(time.Millisecond), "ms/frame")
	if canvas {
		b.ReportMetric(openPct, "staged-open-%")
	}
}

func stageActiveBenchmark(g *Game) float64 {
	// Every track takes the same centered crop of one seeded 100×100 sample.
	// The reserved center guarantees the same hint and effect tiles.
	const side = 100
	rng := rand.New(rand.NewPCG(36, 1))
	centerRow, centerCol := g.Rows/2, g.Cols/2
	cropped := 0
	g.FirstClick, g.Status = true, StatusPlaying
	for dr := -side / 2; dr < side/2; dr++ {
		for dc := -side / 2; dc < side/2; dc++ {
			mine, opened := rng.IntN(5) == 0, rng.IntN(2) == 0
			cell := g.Cell(centerRow+dr, centerCol+dc)
			if cell == nil {
				continue
			}
			cropped++
			if absInt(dr) <= 2 && absInt(dc) <= 5 {
				mine = dr == -1 && dc == 0
				opened = dr == 0 && (dc <= -3 || dc == 0)
				cell.Flagged = mine || (g.Rows == 30 && dr == 2 && dc == -3)
				if cell.Flagged {
					g.FlagsPlaced++
				}
			}
			cell.IsMine = mine
			if opened && !mine {
				cell.Revealed = true
				g.RevealedSafe++
				if d := g.duel; d != nil {
					cell.Owner = DuelPlayer
					d.adjustMinimap(g, cell.Row, cell.Col, DuelNobody, cell.Owner)
				}
			}
		}
	}
	g.RecomputeAdjacency()
	return 100 * float64(g.RevealedSafe) / float64(cropped)
}
