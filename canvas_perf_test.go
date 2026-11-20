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
