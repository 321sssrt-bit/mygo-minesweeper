package main

import (
	"fmt"
	"math"

	"github.com/egoist/mygo/ui"
)

// The notebook art direction's single colour source (docs/adr/0001).
// Member colours are the official 代表色 in Members; everything a member
// paints derives from it. bandColor is the official 乐队色.
var bandColor = ui.Hex("#3388BB")

// palette is one paper: the day notebook or the night desk-lamp notebook.
type palette struct {
	Dark bool
	// Desk is the window behind the sheets; Sheet the panels; Board the
	// graph paper the cells sit on; Grid its ruled lines.
	Desk, Sheet, Board, Grid ui.Color
	// Slip is an unrevealed cell, a scrap of paper stuck on the page.
	Slip, SlipShade               ui.Color
	Ink, InkMuted, Rule, Graphite ui.Color
	// Boom tints the cell whose mine was hit; BoomInk draws its knot.
	Boom, BoomInk ui.Color
	// Lamp is the warm glow in the night notebook's top-left corner.
	Lamp                             ui.Color
	BandAid, BandAidPad, BandAidEdge ui.Color
}

var dayPaper = &palette{
	Desk: ui.Hex("#F3EEE2"), Sheet: ui.Hex("#FFFDF8"), Board: ui.Hex("#FFFEFA"), Grid: bandColor.Alpha(0.16),
	Slip: ui.Hex("#ECE4D3"), SlipShade: ui.Hex("#5A4628").Alpha(0.28),
	Ink: ui.Hex("#2E2A25"), InkMuted: ui.Hex("#7A7064"), Rule: ui.Hex("#2E2A25").Alpha(0.14), Graphite: ui.Hex("#4A4742"),
	Boom: ui.Hex("#C23D63").Alpha(0.16), BoomInk: ui.Hex("#B0303A"),
	BandAid: ui.Hex("#E8C49A"), BandAidPad: ui.Hex("#F5DFC2"), BandAidEdge: ui.Hex("#BF946A"),
}

var nightPaper = &palette{
	Dark: true,
	Desk: ui.Hex("#1C1814"), Sheet: ui.Hex("#26211B"), Board: ui.Hex("#2A241E"), Grid: ui.Hex("#FFD6A0").Alpha(0.08),
	Slip: ui.Hex("#3D352C"), SlipShade: ui.Hex("#000000").Alpha(0.5),
	Ink: ui.Hex("#EDE3D3"), InkMuted: ui.Hex("#A8998A"), Rule: ui.Hex("#EDE3D3").Alpha(0.12), Graphite: ui.Hex("#CFC6B8"),
	Boom: ui.Hex("#FF788C").Alpha(0.18), BoomInk: ui.Hex("#FF8C8C"),
	Lamp:    ui.Hex("#FFBE6E").Alpha(0.16),
	BandAid: ui.Hex("#D9B488"), BandAidPad: ui.Hex("#EBD3B2"), BandAidEdge: ui.Hex("#A27A52"),
}

func paperFor(dark bool) *palette {
	if dark {
		return nightPaper
	}
	return dayPaper
}

// minInkContrast is the WCAG AA ratio every ink keeps against the paper.
const minInkContrast = 4.5

// Ink derives the variant of c that reads on this paper: darkened on the
// day page, lightened under the lamp, keeping its hue, until it reaches
// minInkContrast against Board (and Sheet).
func (p *palette) InkOf(c ui.Color) ui.Color {
	h, s, l := toHSL(c)
	step := -0.01
	if p.Dark {
		step = 0.01
	}
	out := c
	for i := 0; i < 100; i++ {
		if contrast(out, p.Board) >= minInkContrast+0.1 && contrast(out, p.Sheet) >= minInkContrast+0.1 {
			return out
		}
		l = math.Max(0, math.Min(1, l+step))
		out = fromHSL(h, s, l)
	}
	return out
}

// numberInk is the colour of a revealed number: 1 灯, 2 乐奈, 3 爱音, 4 立希,
// 5 爽世 (their 代表色 as ink), then neutral inks.
func numberInk(n int, dark bool) ui.Color {
	p := paperFor(dark)
	switch n {
	case 1:
		return p.InkOf(memberImage(MemberTomori))
	case 2:
		return p.InkOf(memberImage(MemberRana))
	case 3:
		return p.InkOf(memberImage(MemberAnon))
	case 4:
		return p.InkOf(memberImage(MemberTaki))
	case 5:
		return p.InkOf(memberImage(MemberSoyorin))
	case 6:
		return p.Graphite
	case 7:
		return p.InkMuted
	default:
		return p.Ink
	}
}

func memberImage(id MemberID) ui.Color { return ui.Hex(FindMember(id).ColorHex) }

func hexOf(c ui.Color) string { return fmt.Sprintf("#%02X%02X%02X", c.R, c.G, c.B) }

func linear(v uint8) float64 {
	c := float64(v) / 255
	if c <= 0.04045 {
		return c / 12.92
	}
	return math.Pow((c+0.055)/1.055, 2.4)
}

func luminance(c ui.Color) float64 {
	return 0.2126*linear(c.R) + 0.7152*linear(c.G) + 0.0722*linear(c.B)
}

func contrast(a, b ui.Color) float64 {
	la, lb := luminance(a), luminance(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

func toHSL(c ui.Color) (h, s, l float64) {
	r, g, b := float64(c.R)/255, float64(c.G)/255, float64(c.B)/255
	mx, mn := math.Max(r, math.Max(g, b)), math.Min(r, math.Min(g, b))
	l = (mx + mn) / 2
	if mx == mn {
		return 0, 0, l
	}
	d := mx - mn
	if l > 0.5 {
		s = d / (2 - mx - mn)
	} else {
		s = d / (mx + mn)
	}
	switch mx {
	case r:
		h = (g - b) / d
		if g < b {
			h += 6
		}
	case g:
		h = (b-r)/d + 2
	default:
		h = (r-g)/d + 4
	}
	return h / 6, s, l
}

func fromHSL(h, s, l float64) ui.Color {
	hue := func(p, q, t float64) float64 {
		if t < 0 {
			t++
		}
		if t > 1 {
			t--
		}
		switch {
		case t < 1.0/6:
			return p + (q-p)*6*t
		case t < 0.5:
			return q
		case t < 2.0/3:
			return p + (q-p)*(2.0/3-t)*6
		}
		return p
	}
	var r, g, b float64
	if s == 0 {
		r, g, b = l, l, l
	} else {
		q := l * (1 + s)
		if l >= 0.5 {
			q = l + s - l*s
		}
		p := 2*l - q
		r, g, b = hue(p, q, h+1.0/3), hue(p, q, h), hue(p, q, h-1.0/3)
	}
	to8 := func(v float64) uint8 { return uint8(math.Round(math.Max(0, math.Min(1, v)) * 255)) }
	return ui.Color{R: to8(r), G: to8(g), B: to8(b), A: 255}
}

// paperArt are the hand-drawn props of one paper, as SVG.
type paperArt struct {
	Flag, Shield, WrongFlag, Knot, KnotHit *ui.SVG
}

var dayArt, nightArt = newPaperArt(dayPaper), newPaperArt(nightPaper)

func artFor(dark bool) *paperArt {
	if dark {
		return nightArt
	}
	return dayArt
}

func newPaperArt(p *palette) *paperArt {
	soyo := ui.Hex("#FFDD88")
	return &paperArt{
		Flag:      ui.MustParseSVG([]byte(bandAidSVG(p.BandAid, p.BandAidPad, p.BandAidEdge, ""))),
		Shield:    ui.MustParseSVG([]byte(bandAidSVG(soyo, soyo.Mix(ui.Hex("#FFFFFF"), 0.5), p.InkOf(soyo), ""))),
		WrongFlag: ui.MustParseSVG([]byte(bandAidSVG(p.BandAid, p.BandAidPad, p.BandAidEdge, hexOf(p.BoomInk)))),
		Knot:      ui.MustParseSVG([]byte(knotSVG(p.Graphite))),
		KnotHit:   ui.MustParseSVG([]byte(knotSVG(p.BoomInk))),
	}
}

// bandAidSVG is a 创可贴 stuck on at an angle; a non-empty cross draws a
// red ✕ over it (a flag that turned out wrong).
func bandAidSVG(fill, pad, edge ui.Color, cross string) string {
	e := hexOf(edge)
	s := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><g transform="rotate(-38 12 12)">` +
		`<rect x="1.5" y="8.3" width="21" height="7.4" rx="3.7" fill="` + hexOf(fill) + `" stroke="` + e + `" stroke-width="0.8"/>` +
		`<rect x="9" y="8.3" width="6" height="7.4" fill="` + hexOf(pad) + `"/>` +
		`<circle cx="4.6" cy="11" r="0.55" fill="` + e + `"/><circle cx="6.4" cy="13" r="0.55" fill="` + e + `"/>` +
		`<circle cx="17.6" cy="11" r="0.55" fill="` + e + `"/><circle cx="19.4" cy="13" r="0.55" fill="` + e + `"/></g>`
	if cross != "" {
		s += `<path d="M6 6L18 18M18 6L6 18" stroke="` + cross + `" stroke-width="2.2" stroke-linecap="round"/>`
	}
	return s + `</svg>`
}

// knotSVG is a 心结: a pencil scribble tangled into a knot.
func knotSVG(c ui.Color) string {
	return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path d="M7 14.5c-2.2-3.6.6-8.4 5-8.2 4.6.2 6.4 5 3.8 8.1-2.4 2.9-7.6 2.3-8.2-1.3-.6-3.3 3.4-5.6 6.4-4.1 2.8 1.4 2.4 5.4-.6 6.1-2.4.6-4.3-1.6-3.2-3.6 1-1.8 3.8-1.4 3.9.6M6.2 17.6l2-2.1M16.4 15.2l2.3 2.6" fill="none" stroke="` +
		hexOf(c) + `" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/></svg>`
}

// avatarInk writes on a 代表色 avatar; every 代表色 is a light pastel.
var avatarInk = ui.Hex("#2E2A25")

// memberAvatar is mygo's ui.Avatar showing one 文楷 character on the
// member's 代表色, in place of the colour Avatar hashes from a name.
func memberAvatar(c *ui.Context, m Member, size float32, pal *palette) {
	ui.Avatar(c, m.Glyph, nil).Label(m.Name).Size(size, size).Radius(size/2).
		Background(ui.Hex(m.ColorHex)).TextColor(avatarInk).Shadow(0, 1, 2, 0, pal.SlipShade)
}

// iconOrder is the band order the icon's five exclamation marks follow.
var iconOrder = []MemberID{MemberTomori, MemberAnon, MemberRana, MemberSoyorin, MemberTaki}

// bandMark is the small app icon: graph paper with a !!!!! in the five
// members' inks. It is the same on both papers.
func bandMark(c *ui.Context, size float32) {
	paper := ui.Hex("#FBF8F1")
	ui.Box(c).Size(size, size).Radius(size*0.26).Background(paper).
		Shadow(0, 1, 3, 0, dayPaper.SlipShade).Center().Clip().
		Draw(func(p *ui.Painter, r ui.Rect) {
			step := size / 5
			for x := r.X + step; x < r.X+r.W; x += step {
				p.Line(x, r.Y, x, r.Y+r.H, 1, bandColor.Alpha(0.2))
			}
			for y := r.Y + step; y < r.Y+r.H; y += step {
				p.Line(r.X, y, r.X+r.W, y, 1, bandColor.Alpha(0.2))
			}
		}).Children(func() {
		ui.Row(c).Gap(0).PassThrough().Children(func() {
			for _, id := range iconOrder {
				ui.Text(c, "!").FontSize(size * 0.42).Bold().TextColor(dayPaper.InkOf(memberImage(id))).PassThrough()
			}
		})
	})
}

// hatchMeter is a pencil-hatched bar filled to frac.
func hatchMeter(c *ui.Context, label string, frac float64, col ui.Color, pal *palette) {
	frac = math.Max(0, math.Min(1, frac))
	ui.Box(c).Height(10).FillWidth().Label(label).Draw(func(p *ui.Painter, r ui.Rect) {
		w := r.W * float32(frac)
		if w > 0 {
			fill := ui.Rect{X: r.X, Y: r.Y, W: w, H: r.H}
			p.Clip(fill, 5, func() {
				p.Fill(fill, col.Alpha(0.22), 5)
				for x := r.X - r.H; x < r.X+w; x += 4.5 {
					p.Line(x, r.Y+r.H, x+r.H*0.75, r.Y, 1.4, col)
				}
			})
		}
		p.Stroke(r, pal.Rule.Alpha(1.6), 5, 1.2)
	})
}
