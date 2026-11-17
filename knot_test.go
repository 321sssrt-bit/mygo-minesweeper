package main

import (
	"math"
	"testing"

	"github.com/egoist/mygo/ui"
)

// knotReach renders a knot on its paper and returns how far, in viewBox
// units (the art is drawn on 24×24), its farthest ink reaches from the
// centre of all its ink.
func knotReach(t *testing.T, art *ui.SVG, ink, bg ui.Color) float64 {
	t.Helper()
	const px = 240
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Box(c).Size(px, px).Background(bg).Children(func() {
			ui.Image(c, art).Size(px, px)
		})
	}, px, px)
	tt.Frame()
	img := tt.Image()
	// A pixel is ink once it sits closer to the ink than to the paper.
	near := func(r, g, b uint8, c ui.Color) float64 {
		dr, dg, db := float64(r)-float64(c.R), float64(g)-float64(c.G), float64(b)-float64(c.B)
		return dr*dr + dg*dg + db*db
	}
	var xs, ys []float64
	var sx, sy float64
	for y := 0; y < px; y++ {
		for x := 0; x < px; x++ {
			p := img.RGBAAt(x, y)
			if near(p.R, p.G, p.B, ink) < near(p.R, p.G, p.B, bg) {
				xs, ys = append(xs, float64(x)), append(ys, float64(y))
				sx, sy = sx+float64(x), sy+float64(y)
			}
		}
	}
	if len(xs) == 0 {
		t.Fatal("knot rendered no ink")
	}
	cx, cy := sx/float64(len(xs)), sy/float64(len(ys))
	reach := 0.0
	for i := range xs {
		reach = math.Max(reach, math.Hypot(xs[i]-cx, ys[i]-cy))
	}
	return reach * 24 / px
}

// maxKnotReach bounds how far a knot's ink may stray from its centre, in
// viewBox units. The tangle itself stays within about 8; a stroke poking
// out past it reads as a magnifying glass's handle (the old knot's tail
// reached 9.8). Whether the knot reads as a tangle is left to screenshot
// review.
const maxKnotReach = 8.75

func TestKnotTailDoesNotStickOut(t *testing.T) {
	for _, p := range []*palette{dayPaper, nightPaper} {
		a := artFor(p.Dark)
		for _, k := range []struct {
			name    string
			art     *ui.SVG
			ink, bg ui.Color
		}{
			{"knot", a.Knot, p.Graphite, p.Board},
			// The hit knot sits on the Boom tint, as on the board.
			{"hit knot", a.KnotHit, p.BoomInk, p.Boom.Over(p.Board)},
		} {
			if got := knotReach(t, k.art, k.ink, k.bg); got > maxKnotReach {
				t.Errorf("dark=%v %s ink reaches %.2f from its centre, want <= %.2f: a tail sticks out like a handle", p.Dark, k.name, got, maxKnotReach)
			}
		}
	}
}
