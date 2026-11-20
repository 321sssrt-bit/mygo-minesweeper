package main

import (
	_ "embed"
	"fmt"
	"math"

	"github.com/egoist/mygo/ui"
)

// The notebook art direction's single colour source (docs/adr/0001).
// Member colours are the official 代表色 in Members; everything a member
// paints derives from it. bandColor is the official 乐队色.
var bandColor = ui.Hex("#3388BB")

// goldHighlighter is the warm gold highlighter used for duel mine-claim
// hints, the 9×9 scan overlay, and milestone chat notes.
var goldHighlighter = ui.Hex("#E8A849")

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

// minimapColors are the notebook colours of a classic board's minimap:
// desk paper for unopened ground, band-blue wash for opened progress,
// band-aid tone for flags, boom ink for hit mines / wrong flags, and ink
// for the viewport frame.
type minimapColors struct {
	Bg, Border, Opened, Flag, Boom, Viewport ui.Color
}

var dayMinimap, nightMinimap = newMinimapColors(dayPaper), newMinimapColors(nightPaper)

func classicMinimapFor(dark bool) *minimapColors {
	if dark {
		return nightMinimap
	}
	return dayMinimap
}

func newMinimapColors(p *palette) *minimapColors {
	opened := bandColor.Alpha(0.65).Over(p.Desk)
	flag := p.InkOf(p.BandAidEdge)
	if p.Dark {
		opened = p.InkOf(bandColor).Alpha(0.65).Over(p.Desk)
		flag = p.BandAid
	}
	return &minimapColors{
		Bg:       p.Desk,
		Border:   p.Rule.Alpha(1.6),
		Opened:   opened,
		Flag:     flag,
		Boom:     p.BoomInk,
		Viewport: p.Ink,
	}
}

// duelColors are the high-contrast player-blue and AI-pink colours used for
// duel territory washes, flag outlines, reveal ripples, and the latest AI
// move frame.
type duelColors struct {
	Player, AI ui.Color
}

var duelPalette = duelColors{
	Player: ui.Hex("#55B2D6"),
	AI:     ui.Hex("#F07AA4"),
}

// duelMinimapColors holds the dark navy duel minimap backdrop, territory
// block colours, live markers, and viewport frame.
type duelMinimapColors struct {
	Bg, Contested, Player, AI  ui.Color
	PlayerMarker, PlayerStroke ui.Color
	AIMarker, AIStroke         ui.Color
	Viewport                   ui.Color
}

var duelMinimapPalette = duelMinimapColors{
	Bg:           ui.Hex("#0B1423"),
	Contested:    ui.Hex("#A78BFA"),
	Player:       duelPalette.Player,
	AI:           duelPalette.AI,
	PlayerMarker: ui.Hex("#38BDF8"),
	PlayerStroke: ui.Hex("#F4F8FD"),
	AIMarker:     duelPalette.AI,
	AIStroke:     ui.Hex("#FDF2F8"),
	Viewport:     ui.Hex("#F4F8FD"),
}

// duelTerritoryWash is how strongly a side's colour washes over the paper
// it has opened: enough to read as territory, light enough to stay paper.
const duelTerritoryWash = 0.16

// duelOwnerColor marks a side on the duel paper: its territory, its flags'
// outline and its reveal ripples.
func duelOwnerColor(owner DuelOwner) ui.Color {
	if owner == DuelAI {
		return duelPalette.AI
	}
	return duelPalette.Player
}

// duelCellPaper is what a revealed cell's number is written on: the bare
// board, or the board under a side's territory wash.
func duelCellPaper(p *palette, owner DuelOwner) ui.Color {
	if owner == DuelNobody {
		return p.Board
	}
	return duelOwnerColor(owner).Alpha(duelTerritoryWash).Over(p.Board)
}

// duelInks are the number inks per owner: numberInk, deepened (or lit,
// at night) where a territory wash would leave it under minInkContrast.
type duelInks [DuelAI + 1][len(cellDigits)]ui.Color

var dayDuelInks, nightDuelInks = newDuelInks(dayPaper), newDuelInks(nightPaper)

func duelInksFor(dark bool) *duelInks {
	if dark {
		return nightDuelInks
	}
	return dayDuelInks
}

func newDuelInks(p *palette) *duelInks {
	var inks duelInks
	for owner := DuelNobody; owner <= DuelAI; owner++ {
		under := *p
		under.Board = duelCellPaper(p, owner)
		under.Sheet = under.Board
		for n := 1; n < len(cellDigits); n++ {
			inks[owner][n] = under.InkOf(numberInk(n, p.Dark))
		}
	}
	return &inks
}

// duelPopupKind classifies a floating duel score pill so its palette
// colours follow the active day or night paper.
type duelPopupKind uint8

const (
	duelPopupScore duelPopupKind = iota
	duelPopupCombo
	duelPopupClaim
	duelPopupBoom
)

// duelPopupStyle holds the pill backdrop, border, and text colours for one
// floating duel score popup.
type duelPopupStyle struct {
	Bg, Border, Text ui.Color
}

// duelPromptColors holds the palette colours for duel canvas overlays:
// claim highlighter, 9×9 scan frame & corner badge, and floating score pills.
type duelPromptColors struct {
	ClaimFill, ClaimStroke       ui.Color
	ScanFrame                    ui.Color
	ScanBadgeBg, ScanBadgeBorder ui.Color
	ScanBadgeText                ui.Color
	Popups                       [duelPopupBoom + 1]duelPopupStyle
}

var dayDuelPrompts, nightDuelPrompts = newDuelPromptColors(dayPaper), newDuelPromptColors(nightPaper)

func duelPromptsFor(dark bool) *duelPromptColors {
	if dark {
		return nightDuelPrompts
	}
	return dayDuelPrompts
}

func newDuelPromptColors(p *palette) *duelPromptColors {
	scanBgAlpha := float32(0.24)
	popupBg := p.Sheet
	if p.Dark {
		scanBgAlpha = 0.20
		popupBg = p.Desk
	}
	scanBg := goldHighlighter.Alpha(scanBgAlpha).Over(p.Sheet)
	goldInk := p.InkOf(goldHighlighter)
	popupStyle := func(base ui.Color) duelPopupStyle {
		ink := p.inkOn(popupBg, base)
		border := ink
		if p.Dark {
			border = base
		}
		return duelPopupStyle{Bg: popupBg, Border: border, Text: ink}
	}
	return &duelPromptColors{
		ClaimFill:       goldHighlighter,
		ClaimStroke:     goldInk,
		ScanFrame:       goldInk,
		ScanBadgeBg:     scanBg,
		ScanBadgeBorder: goldInk,
		ScanBadgeText:   p.inkOn(scanBg, goldHighlighter),
		Popups: [duelPopupBoom + 1]duelPopupStyle{
			duelPopupScore: popupStyle(duelPalette.Player),
			duelPopupCombo: popupStyle(duelMinimapPalette.PlayerMarker),
			duelPopupClaim: popupStyle(goldHighlighter),
			duelPopupBoom:  popupStyle(p.BoomInk),
		},
	}
}

// badgeColors holds the composited pill background and WCAG-AA text ink for
// one duel HUD badge.
type badgeColors struct {
	Bg, Text ui.Color
}

// duelStunColors holds the palette colours for the canvas stun warning card
// and the pulsing alert border around the duel board.
type duelStunColors struct {
	Bg, Border, Title, Sub, Pulse ui.Color
}

// duelHUDColors holds the day/night notebook palette for the duel HUD badges
// and stun alert overlays.
type duelHUDColors struct {
	Player, AI, Stun, Scan badgeColors
	Banner                 duelStunColors
}

var dayDuelHUD, nightDuelHUD = newDuelHUDColors(dayPaper, dayDuelPrompts), newDuelHUDColors(nightPaper, nightDuelPrompts)

func duelHUDFor(dark bool) *duelHUDColors {
	if dark {
		return nightDuelHUD
	}
	return dayDuelHUD
}

func newDuelHUDColors(p *palette, dp *duelPromptColors) *duelHUDColors {
	scoreAlpha := float32(0.22)
	stunBadgeAlpha := float32(0.20)
	bannerBase := p.Sheet
	bannerAlpha := float32(0.14)
	if p.Dark {
		scoreAlpha = 0.20
		stunBadgeAlpha = 0.24
		bannerBase = p.Desk
		bannerAlpha = 0.22
	}
	badge := func(base ui.Color, alpha float32) badgeColors {
		bg := base.Alpha(alpha).Over(p.Sheet)
		return badgeColors{Bg: bg, Text: p.inkOn(bg, base)}
	}
	bannerBg := p.BoomInk.Alpha(bannerAlpha).Over(bannerBase)
	return &duelHUDColors{
		Player: badge(duelPalette.Player, scoreAlpha),
		AI:     badge(duelPalette.AI, scoreAlpha),
		Stun:   badge(p.BoomInk, stunBadgeAlpha),
		Scan:   badgeColors{Bg: dp.ScanBadgeBg, Text: dp.ScanBadgeText},
		Banner: duelStunColors{
			Bg:     bannerBg,
			Border: p.BoomInk,
			Title:  p.inkOn(bannerBg, p.BoomInk),
			Sub:    p.inkOn(bannerBg, p.Graphite),
			Pulse:  p.BoomInk,
		},
	}
}

// haruhikageDramaColors holds the notebook palette for the Haruhikage
// lights-out drama overlay and its follow-up questioning AlertDialog.
type haruhikageDramaColors struct {
	Scrim      ui.Color
	CardBg     ui.Color
	CardBorder ui.Color
	CardRule   ui.Color
	CardShadow ui.Color
	AccentBar  ui.Color
	Eyebrow    ui.Color
	Title      ui.Color
	Subtitle   ui.Color

	DialogBg         ui.Color
	DialogSurface    ui.Color
	DialogHover      ui.Color
	DialogPressed    ui.Color
	DialogBorder     ui.Color
	DialogText       ui.Color
	DialogMuted      ui.Color
	DialogAccent     ui.Color
	DialogAccentText ui.Color
}

var dayHaruhikage, nightHaruhikage = newHaruhikageDramaColors(dayPaper), newHaruhikageDramaColors(nightPaper)

func haruhikageFor(dark bool) *haruhikageDramaColors {
	if dark {
		return nightHaruhikage
	}
	return dayHaruhikage
}

func newHaruhikageDramaColors(p *palette) *haruhikageDramaColors {
	lightsOut := ui.Hex("#120F0C")
	soyo := memberImage(MemberSoyorin)
	scrim := lightsOut.Alpha(0.86)
	cardBg := nightPaper.Board
	accentBar := dayPaper.BoomInk
	dialogBg := p.Sheet.Mix(p.Slip, 0.35)
	dialogSurface := p.Sheet
	dialogAccentText := p.Sheet
	if p.Dark {
		scrim = lightsOut.Alpha(0.90)
		cardBg = nightPaper.Desk.Mix(nightPaper.Sheet, 0.4)
		accentBar = dayPaper.BoomInk.Mix(nightPaper.BoomInk, 0.35)
		dialogBg = p.Sheet
		dialogSurface = p.Sheet.Mix(p.Slip, 0.45)
		dialogAccentText = p.Desk
	}
	return &haruhikageDramaColors{
		Scrim:            scrim,
		CardBg:           cardBg,
		CardBorder:       soyo.Alpha(0.62).Over(cardBg),
		CardRule:         nightPaper.Ink.Alpha(0.06),
		CardShadow:       nightPaper.SlipShade.Alpha(0.62),
		AccentBar:        accentBar,
		Eyebrow:          nightPaper.inkOn(cardBg, soyo),
		Title:            nightPaper.inkOn(cardBg, nightPaper.Ink),
		Subtitle:         nightPaper.inkOn(cardBg, nightPaper.InkMuted),
		DialogBg:         dialogBg,
		DialogSurface:    dialogSurface,
		DialogHover:      dialogSurface.Mix(p.Ink, 0.06),
		DialogPressed:    dialogSurface.Mix(p.Ink, 0.12),
		DialogBorder:     p.BoomInk.Alpha(0.5).Over(dialogBg),
		DialogText:       p.inkOn(dialogBg, p.Ink),
		DialogMuted:      p.inkOn(dialogBg, p.InkMuted),
		DialogAccent:     p.BoomInk,
		DialogAccentText: dialogAccentText,
	}
}

// dialogTheme derives the AlertDialog widget theme from base so the
// questioning prompt sits on notebook paper with heart-knot emphasis
// while keeping AlertDialog's native semantics and focus order.
func (d *haruhikageDramaColors) dialogTheme(base *ui.Theme) *ui.Theme {
	t := base.WithAccent(d.DialogAccent)
	t.AccentText = d.DialogAccentText
	t.Background = d.DialogBg
	t.Surface = d.DialogSurface
	t.SurfaceHover = d.DialogHover
	t.SurfacePressed = d.DialogPressed
	t.Border = d.DialogBorder
	t.Text = d.DialogText
	t.TextMuted = d.DialogMuted
	return t
}

// hoverHintColors returns the highlighter fill and crisp cell outline for
// a number-hover hint on this paper.
func hoverHintColors(kind HoverHintKind, memberID MemberID, dark bool) (fill, stroke ui.Color) {
	switch kind {
	case HintClaimable:
		dp := duelPromptsFor(dark)
		return dp.ClaimFill, dp.ClaimStroke
	case HintChordable:
		base := memberImage(memberID)
		return base, paperFor(dark).InkOf(base)
	default:
		return ui.Color{}, ui.Color{}
	}
}

// inkOn derives the variant of c that reaches minInkContrast against bg,
// darkened on the day page and lightened under the night lamp.
func (p *palette) inkOn(bg, c ui.Color) ui.Color {
	under := *p
	under.Board = bg
	under.Sheet = bg
	return under.InkOf(c)
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

// knotSVG is a 心结: one pencil stroke tangled into a knot. Its loose end
// stays a short curl hugging the knot, so nothing reads as a handle.
func knotSVG(c ui.Color) string {
	return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path d="M6.3 16.6c-.1-.7.5-1.2.7-2.1-2.2-3.6.6-8.4 5-8.2 4.6.2 6.4 5 3.8 8.1-2.4 2.9-7.6 2.3-8.2-1.3-.6-3.3 3.4-5.6 6.4-4.1 2.8 1.4 2.4 5.4-.6 6.1-2.4.6-4.3-1.6-3.2-3.6 1-1.8 3.8-1.4 3.9.6" fill="none" stroke="` +
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

//go:embed resources/icon.svg
var appIconSVGSource []byte

var appIconSVG = ui.MustParseSVG(appIconSVGSource)

// bandMark is the small app icon, rendered from the same SVG source as the
// shipped application icon. It stays on the day paper in both themes.
func bandMark(c *ui.Context, size float32) {
	ui.Image(c, appIconSVG).Size(size, size)
}

// drawHatchOverlay paints pencil diagonal hatching over a bar's filled span
// and strokes its notebook outline. Callers attach it via DrawOver so the
// widget's native fill stays intact and the outline is not clipped in half.
func drawHatchOverlay(p *ui.Painter, r ui.Rect, frac float64, col ui.Color, pal *palette) {
	frac = math.Max(0, math.Min(1, frac))
	w := r.W * float32(frac)
	if w > 0 {
		fill := ui.Rect{X: r.X, Y: r.Y, W: w, H: r.H}
		p.Clip(fill, 5, func() {
			for x := r.X - r.H; x < r.X+w; x += 4.5 {
				p.Line(x, r.Y+r.H, x+r.H*0.75, r.Y, 1.4, col)
			}
		})
	}
	p.Stroke(r, pal.Rule.Alpha(1.6), 5, 1.2)
}

// hatchProgress renders a ui.Progress bar with notebook pencil hatching
// overlaid on its native fill so assistive technology reads its progress role
// and value.
func hatchProgress(c *ui.Context, labelEl ui.Element, frac float64, col ui.Color, pal *palette) {
	ui.Progress(c, frac).Height(10).FillWidth().Radius(5).
		Background(ui.Transparent).TextColor(col.Alpha(0.22)).
		LabelledBy(labelEl).
		DrawOver(func(p *ui.Painter, r ui.Rect) {
			drawHatchOverlay(p, r, frac, col, pal)
		})
}

// hatchMeter renders a ui.Meter bar in [0, 100] with notebook pencil hatching
// overlaid on its native fill so assistive technology reads its meter role
// and value.
func hatchMeter(c *ui.Context, labelEl ui.Element, value float64, col ui.Color, pal *palette) {
	ui.Meter(c, value, 0, 100, &ui.MeterLevels{NormalColor: col.Alpha(0.22)}).
		Height(10).FillWidth().Radius(5).
		Background(ui.Transparent).
		LabelledBy(labelEl).
		DrawOver(func(p *ui.Painter, r ui.Rect) {
			drawHatchOverlay(p, r, value/100, col, pal)
		})
}
