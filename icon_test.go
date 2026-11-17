package main

import (
	"bytes"
	"encoding/xml"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

// The app icon is drawn by hand as resources/icon.svg and shipped as
// resources/icon.png, its export at 1024 px by the ui package's own SVG
// rasterizer. After editing the SVG, export it again with:
//
//	MYGO_ICON_EXPORT=1 go test -run TestAppIconExport .
const (
	iconSource     = "resources/icon.svg"
	iconExport     = "resources/icon.png"
	iconSize       = 1024
	iconExportHint = "MYGO_ICON_EXPORT=1 go test -run TestAppIconExport ."
)

func readIconSource(t *testing.T) []byte {
	t.Helper()
	src, err := os.ReadFile(iconSource)
	if err != nil {
		t.Fatalf("read %s: %v", iconSource, err)
	}
	return src
}

// shippedIcon is the icon the app embeds. A run that exports a new one
// still embeds the old, so it leaves the shipped icon's checks to the next run.
func shippedIcon(t *testing.T) image.Image {
	t.Helper()
	if os.Getenv("MYGO_ICON_EXPORT") != "" {
		t.Skip("exporting " + iconExport + "; run the tests again to check it")
	}
	img, err := png.Decode(bytes.NewReader(appIconPNG))
	if err != nil {
		t.Fatalf("decode embedded icon: %v", err)
	}
	return img
}

func renderIcon(t *testing.T, src []byte) *image.RGBA {
	t.Helper()
	s, err := ui.ParseSVG(src)
	if err != nil {
		t.Fatalf("parse %s: %v", iconSource, err)
	}
	return s.Image(iconSize, iconSize, nil)
}

func TestAppIconExport(t *testing.T) {
	if os.Getenv("MYGO_ICON_EXPORT") == "" {
		t.Skip("set MYGO_ICON_EXPORT=1 to export " + iconSource + " to " + iconExport)
	}
	var buf bytes.Buffer
	enc := png.Encoder{CompressionLevel: png.BestCompression}
	if err := enc.Encode(&buf, renderIcon(t, readIconSource(t))); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(iconExport, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestAppIconSourceIsOriginalVectorArt keeps the icon's source honest: plain
// vector shapes the rasterizer draws in full, no lettering, links or
// embedded artwork (so no band name or logo glyphs), and every colour it
// paints from the day palette.
func TestAppIconSourceIsOriginalVectorArt(t *testing.T) {
	src := readIconSource(t)
	s, err := ui.ParseSVG(src)
	if err != nil {
		t.Fatalf("parse %s: %v", iconSource, err)
	}
	if missing := s.Unsupported(); len(missing) > 0 {
		t.Errorf("%s uses features the export leaves out: %v", iconSource, missing)
	}

	allowed := map[string]bool{hexOf(bandColor): true}
	for _, c := range []ui.Color{
		dayPaper.Desk, dayPaper.Sheet, dayPaper.Board, dayPaper.Grid, dayPaper.Slip, dayPaper.SlipShade,
		dayPaper.Ink, dayPaper.InkMuted, dayPaper.Rule, dayPaper.Graphite,
		dayPaper.BandAid, dayPaper.BandAidPad, dayPaper.BandAidEdge,
	} {
		allowed[hexOf(c)] = true
	}
	for _, id := range iconOrder {
		allowed[hexOf(dayPaper.InkOf(memberImage(id)))] = true
	}

	notShapes := map[string]bool{
		"text": true, "tspan": true, "textPath": true, "image": true, "font": true, "font-face": true,
		"foreignObject": true, "style": true, "script": true, "a": true,
	}
	paints := map[string]bool{"fill": true, "stroke": true, "stop-color": true, "flood-color": true, "lighting-color": true, "color": true}
	filled := map[string]bool{"path": true, "rect": true, "circle": true, "ellipse": true, "polygon": true, "polyline": true}
	mentionsBand := func(s string) bool { return strings.Contains(strings.ToLower(s), "mygo") }

	// Each open element: whether a fill is set on it or an ancestor, and
	// whether it is inside a clip path, whose shapes paint nothing.
	type scope struct{ fill, clip bool }
	stack := []scope{{}}
	dec := xml.NewDecoder(bytes.NewReader(src))
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("read %s: %v", iconSource, err)
		}
		switch tok := tok.(type) {
		case xml.StartElement:
			name := tok.Name.Local
			sc := stack[len(stack)-1]
			sc.clip = sc.clip || name == "clipPath"
			if notShapes[name] {
				t.Errorf("%s has <%s>: the icon is drawn shapes only", iconSource, name)
			}
			for _, a := range tok.Attr {
				k, v := a.Name.Local, strings.TrimSpace(a.Value)
				switch {
				case k == "href" || k == "style":
					t.Errorf("%s <%s %s>: no links, and colours only as attributes", iconSource, name, k)
				case paints[k]:
					if k == "fill" {
						sc.fill = true
					}
					if v != "none" && !strings.HasPrefix(v, "url(#") && !allowed[strings.ToUpper(v)] {
						t.Errorf("%s <%s %s=%q>: not a day palette colour", iconSource, name, k, v)
					}
				}
				if mentionsBand(v) {
					t.Errorf("%s <%s %s=%q> names the band", iconSource, name, k, v)
				}
			}
			if filled[name] && !sc.fill && !sc.clip {
				t.Errorf("%s <%s> sets no fill, so it would paint black", iconSource, name)
			}
			stack = append(stack, sc)
		case xml.EndElement:
			stack = stack[:len(stack)-1]
		case xml.CharData:
			if mentionsBand(string(tok)) {
				t.Errorf("%s has text naming the band: %q", iconSource, strings.TrimSpace(string(tok)))
			}
		}
	}
}

// TestAppIconPNGIsTheSourceExport checks the shipped PNG is the current
// export of the SVG, so the vector source never drifts from the icon.
func TestAppIconPNGIsTheSourceExport(t *testing.T) {
	got := shippedIcon(t)
	want := renderIcon(t, readIconSource(t))
	if got.Bounds() != want.Bounds() {
		t.Fatalf("%s is %v, the export is %v", iconExport, got.Bounds(), want.Bounds())
	}
	// Antialiased edges may round a step or two differently on another CPU
	// architecture; anything more is a stale export.
	const nudge, maxDiff = 2, 8
	off := 0
	for y := 0; y < iconSize; y++ {
		for x := 0; x < iconSize; x++ {
			a := color.RGBA64Model.Convert(got.At(x, y)).(color.RGBA64)
			b := want.RGBA64At(x, y)
			worst := 0
			for _, d := range [][2]uint16{{a.R, b.R}, {a.G, b.G}, {a.B, b.B}, {a.A, b.A}} {
				diff := int(d[0]>>8) - int(d[1]>>8)
				worst = max(worst, diff, -diff)
			}
			if worst > maxDiff {
				t.Fatalf("%s differs from the export of %s at (%d, %d) by %d/255; run %s", iconExport, iconSource, x, y, worst, iconExportHint)
			}
			if worst > nudge {
				off++
			}
		}
	}
	if limit := iconSize * iconSize / 1000; off > limit {
		t.Fatalf("%s differs from the export of %s at %d pixels (limit %d); run %s", iconExport, iconSource, off, limit, iconExportHint)
	}
}

// TestAppIconMarksFollowBandOrder reads the shipped icon for the five
// members' day inks, wherever the marks sit: each ink is there, and left to
// right they are 灯, 爱音, 乐奈, 爽世 and 立希.
func TestAppIconMarksFollowBandOrder(t *testing.T) {
	img := shippedIcon(t)
	inks := map[color.NRGBA]int{}
	for i, id := range iconOrder {
		c := dayPaper.InkOf(memberImage(id))
		inks[color.NRGBA{R: c.R, G: c.G, B: c.B, A: 255}] = i
	}
	sumX := make([]int, len(iconOrder))
	area := make([]int, len(iconOrder))
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if i, ok := inks[color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)]; ok {
				sumX[i] += x
				area[i]++
			}
		}
	}
	// A mark is at least a small stroke of solid ink, not a few stray pixels.
	const minArea = iconSize * iconSize / 400
	prev, prevID := -1.0, MemberID("")
	for i, id := range iconOrder {
		if area[i] < minArea {
			t.Fatalf("%s's day ink covers %d px of the icon, want a mark of at least %d px", id, area[i], minArea)
		}
		x := float64(sumX[i]) / float64(area[i])
		if x <= prev {
			t.Fatalf("%s's mark (centre x %.0f) is not right of %s's (%.0f); want band order %v", id, x, prevID, prev, iconOrder)
		}
		prev, prevID = x, id
	}
}
