package main

import (
	"bytes"
	"encoding/xml"
	"image"
	"image/color"
	"image/png"
	"os"
	"regexp"
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
	iconSource = "resources/icon.svg"
	iconExport = "resources/icon.png"
	iconSize   = 1024
)

func readIconSource(t *testing.T) []byte {
	t.Helper()
	src, err := os.ReadFile(iconSource)
	if err != nil {
		t.Fatalf("read %s: %v", iconSource, err)
	}
	return src
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
		t.Skip("set MYGO_ICON_EXPORT=1 to export resources/icon.svg to resources/icon.png")
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
// vector shapes the rasterizer draws in full, no lettering or embedded
// artwork (so no band name or logo glyphs), and colours only from the palette.
func TestAppIconSourceIsOriginalVectorArt(t *testing.T) {
	src := readIconSource(t)
	s, err := ui.ParseSVG(src)
	if err != nil {
		t.Fatalf("parse %s: %v", iconSource, err)
	}
	if missing := s.Unsupported(); len(missing) > 0 {
		t.Errorf("%s uses features the export leaves out: %v", iconSource, missing)
	}

	lower := strings.ToLower(string(src))
	for _, banned := range []string{"mygo", "<text", "<tspan", "<textpath", "<image", "<font", "font-family", "href", "@import", "url(http", "url(data"} {
		if strings.Contains(lower, banned) {
			t.Errorf("%s contains %q: the icon is drawn shapes only", iconSource, banned)
		}
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
	for _, hex := range regexp.MustCompile(`#[0-9A-Fa-f]{3,8}\b`).FindAllString(string(src), -1) {
		if !allowed[strings.ToUpper(hex)] {
			t.Errorf("%s colour %s is not from the day palette", iconSource, hex)
		}
	}

	// Colours may only be written as palette hex values, not by name or function.
	dec := xml.NewDecoder(bytes.NewReader(src))
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		el, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		for _, a := range el.Attr {
			if a.Name.Local != "fill" && a.Name.Local != "stroke" {
				continue
			}
			if v := a.Value; v != "none" && !strings.HasPrefix(v, "url(#") && !strings.HasPrefix(v, "#") {
				t.Errorf("%s <%s %s=%q>: write colours as palette hex", iconSource, el.Name.Local, a.Name.Local, v)
			}
		}
	}
}

// TestAppIconPNGIsTheSourceExport checks the shipped PNG is the current
// export of the SVG, so the vector source never drifts from the icon.
func TestAppIconPNGIsTheSourceExport(t *testing.T) {
	want := renderIcon(t, readIconSource(t))
	got, err := png.Decode(bytes.NewReader(appIconPNG))
	if err != nil {
		t.Fatalf("decode embedded icon: %v", err)
	}
	if got.Bounds() != want.Bounds() {
		t.Fatalf("%s is %v, the export is %v", iconExport, got.Bounds(), want.Bounds())
	}
	// Antialiased edges may round differently across CPU architectures.
	const tolerance = 3
	off := 0
	for y := 0; y < iconSize; y++ {
		for x := 0; x < iconSize; x++ {
			a := color.RGBA64Model.Convert(got.At(x, y)).(color.RGBA64)
			b := want.RGBA64At(x, y)
			for _, d := range [][2]uint16{{a.R, b.R}, {a.G, b.G}, {a.B, b.B}, {a.A, b.A}} {
				if diff := int(d[0]>>8) - int(d[1]>>8); diff > tolerance || diff < -tolerance {
					off++
					break
				}
			}
		}
	}
	if limit := iconSize * iconSize / 1000; off > limit {
		t.Fatalf("%s differs from the export of %s at %d pixels (limit %d); run MYGO_ICON_EXPORT=1 go test -run TestAppIconExport .", iconExport, iconSource, off, limit)
	}
}

// TestAppIconMarksFollowBandOrder reads the shipped icon for the five
// members' day inks, wherever the marks sit: each ink is there, and left to
// right they are 灯, 爱音, 乐奈, 爽世 and 立希.
func TestAppIconMarksFollowBandOrder(t *testing.T) {
	img, err := png.Decode(bytes.NewReader(appIconPNG))
	if err != nil {
		t.Fatalf("decode embedded icon: %v", err)
	}
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
