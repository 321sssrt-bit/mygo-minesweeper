package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"unicode"

	"github.com/egoist/mygo/ui"
	"golang.org/x/image/font/sfnt"
)

// The two README overview cards are rendered with the app's own notebook
// palette, props and ui package at 2x scale. Exporting requires the full
// WenKai TTFs in the local font cache (populated by `go generate`). Regenerate
// both PNGs after editing card copy or layout with:
//
//	MYGO_CARDS_EXPORT=1 go test -run TestCardsExport .
const (
	gameplayCardPath = "docs/assets/card-gameplay.png"
	membersCardPath  = "docs/assets/card-members.png"

	gameplayCardW = 1160
	gameplayCardH = 424
	membersCardW  = 1160
	membersCardH  = 360
	cardScale     = 2

	cardFontFamily   = "LXGW WenKai Cards"
	cardFontVersion  = "v1.530"
	cardFontRegular  = "LXGWWenKai-Regular.ttf"
	cardFontMedium   = "LXGWWenKai-Medium.ttf"
	cardsExportUsage = "MYGO_CARDS_EXPORT=1 go test -run TestCardsExport ."
)

type gameplayBullet struct {
	Title string
	Body  string
	Icon  *ui.SVG
}

type gameplayKeyRow struct {
	Key  string
	Desc string
	Icon *ui.SVG
}

type gameplayCardSpec struct {
	Title          string
	Subtitle       string
	ModePill       string
	ClassicHeading string
	ClassicGroup   string
	ClassicTracks  string
	DuelGroup      string
	DuelTracks     string
	ModeNote       string
	ClassicPoints  []gameplayBullet
	DuelHeading    string
	DuelSubtitle   string
	DuelPoints     []gameplayBullet
	LegendLabels   [5]string
	ControlHeading string
	ControlSub     string
	KeyRows        []gameplayKeyRow
}

type memberCardEntry struct {
	Member      Member
	TriggerTag  string
	ClassicDesc string
	DuelDesc    string
}

type membersCardSpec struct {
	Title    string
	Subtitle string
	RightTip string
	Classic  string
	Duel     string
	Entries  []memberCardEntry
}

func formatTrackGroupList(entries []setlistEntry) string {
	parts := make([]string, len(entries))
	for i, e := range entries {
		parts[i] = fmt.Sprintf("《%s》%d×%d", e.Diff.Song, e.Diff.Cols, e.Diff.Rows)
	}
	if len(parts) > 3 {
		return strings.Join(parts[:3], " · ") + "\n" + strings.Join(parts[3:], " · ")
	}
	return strings.Join(parts, " · ")
}

func gameplayCardData() gameplayCardSpec {
	groups := groupSetlistTracks(Difficulties)
	var classicEntries, duelEntries []setlistEntry
	for _, g := range groups {
		if g.Gameplay == GameplayDuel {
			duelEntries = g.Entries
		} else {
			classicEntries = g.Entries
		}
	}
	haruhikageTitle := ""
	for _, m := range AllMemes {
		if m.ID == MemeHaruhikage {
			haruhikageTitle = m.Title
			break
		}
	}
	return gameplayCardSpec{
		Title:          "一图看懂玩法、歌单与操作",
		Subtitle:       "灯的笔记本 · 七首曲目阶梯、蓝粉领地决斗与键鼠速查",
		ModePill:       "乐队 Live / 纯正经典",
		ClassicHeading: "七首曲目歌单 · 经典排练",
		ClassicGroup:   fmt.Sprintf("经典 %d 首", len(classicEntries)),
		ClassicTracks:  formatTrackGroupList(classicEntries),
		DuelGroup:      fmt.Sprintf("决斗 %d 首", len(duelEntries)),
		DuelTracks:     formatTrackGroupList(duelEntries),
		ModeNote:       "可切「乐队 Live」与「纯正经典」",
		ClassicPoints: []gameplayBullet{
			{
				Title: "首格必安全",
				Body:  "开局首击必安全，周围空地一圈圈自动翻开",
			},
			{
				Title: "代表色荧光笔连开",
				Body:  "悬停已插满旗的数字，代表色荧光笔高亮，左键一键连开",
			},
			{
				Title: "春日影名场面",
				Body:  fmt.Sprintf("在《春日影》决斗中比分输给高松灯，\n触发「%s」名场面", haruhikageTitle),
			},
		},
		DuelHeading:  "蓝粉领地决斗",
		DuelSubtitle: "与高松灯同盘限时抢占蓝粉领地，计时结束以比分定胜负",
		DuelPoints: []gameplayBullet{
			{
				Title: "抢地与金色收雷",
				Body:  "开安全格 +1；金色荧光笔高亮提示，\n确定全雷时左键点数字收雷 · +75/雷",
			},
			{
				Title: "踩雷硬直与解围",
				Body:  "踩雷 −100 并硬直 2.4 秒；高松灯领队按 E 可立即解除硬直",
				Icon:  dayArt.KnotHit,
			},
			{
				Title: "大棋盘与决斗小地图",
				Body:  "50×50 / 100×100 大棋盘与决斗均配备右下角小地图，点击即跳转视野",
			},
		},
		LegendLabels:   [5]string{"你", "高松灯", "收雷", "插旗", "心结雷"},
		ControlHeading: "键鼠与视野速查",
		ControlSub:     "单手快捷键配合鼠标，小棋盘与大棋盘无缝切换",
		KeyRows: []gameplayKeyRow{
			{
				Key:  "左键",
				Desc: "翻开格子、数字一键连开，或决斗金色收雷",
			},
			{
				Key:  "右键",
				Desc: "贴创可贴插旗\n经典曲目可开启「? 标记」循环",
				Icon: dayArt.Flag,
			},
			{
				Key:  "E / F / R",
				Desc: "E 发动技能（乐队 Live / 决斗有充能时）\nF 切换「插旗模式 (F)」· R 重开本曲",
			},
			{
				Key:  "视野导航",
				Desc: "拖动平移 · 滚轮缩放 · 决斗可点「回到中心」与「定位高松灯」",
			},
		},
	}
}

func membersCardData() membersCardSpec {
	descriptions := map[MemberID][2]string{
		MemberTomori: {
			"优先翻开安全边界格，\n并缓和乐队重力值",
			"9×9 扫描 7 秒，\n立即解除踩雷硬直",
		},
		MemberAnon: {
			"随机翻开 2 个安全格，\n快速打开当前局面",
			"9×9 扫描 7 秒，\n连击转为最高 +75 奖励分",
		},
		MemberSoyorin: {
			"踩雷自动消耗护盾，\n并替你贴上创可贴",
			"被动护盾抵消 −100 与硬直；\n按 E 扫描 7 秒并延缓灯 1.2 秒",
		},
		MemberTaki: {
			"精准找出边界隐藏雷，\n并贴上创可贴插旗",
			"9×9 扫描 7 秒，\n延缓高松灯行动 3.0 秒",
		},
		MemberRana: {
			"展开一片 0 雷空白区，\n并缓和乐队重力值",
			"9×9 扫描 7 秒，\n延缓高松灯行动 1.2 秒",
		},
	}
	entries := make([]memberCardEntry, len(Members))
	for i, m := range Members {
		tag := "主动 E"
		if m.Passive {
			tag = "护盾 / E"
		}
		desc := descriptions[m.ID]
		entries[i] = memberCardEntry{
			Member:      m,
			TriggerTag:  tag,
			ClassicDesc: desc[0],
			DuelDesc:    desc[1],
		}
	}
	return membersCardSpec{
		Title:    "MyGO!!!!! 五人领队技能图鉴",
		Subtitle: "全员均取官方代表色入墨，技能仅在「乐队 Live」与决斗有充能时生效",
		RightTip: fmt.Sprintf("左侧栏随时切换领队 · 内置 %d 项名场面图鉴与排练室群聊", len(AllMemes)),
		Classic:  "经典曲目 · 乐队 Live",
		Duel:     "决斗曲目",
		Entries:  entries,
	}
}

func (s gameplayCardSpec) allStrings() []string {
	out := []string{
		s.Title, s.Subtitle, s.ModePill,
		s.ClassicHeading, s.ClassicGroup, s.ClassicTracks,
		s.DuelGroup, s.DuelTracks, s.ModeNote,
		s.DuelHeading, s.DuelSubtitle,
		s.ControlHeading, s.ControlSub,
	}
	for _, p := range s.ClassicPoints {
		out = append(out, p.Title, p.Body)
	}
	for _, p := range s.DuelPoints {
		out = append(out, p.Title, p.Body)
	}
	for _, l := range s.LegendLabels {
		out = append(out, l)
	}
	for _, k := range s.KeyRows {
		out = append(out, k.Key, k.Desc)
	}
	return out
}

func (s membersCardSpec) allStrings() []string {
	out := []string{s.Title, s.Subtitle, s.RightTip, s.Classic, s.Duel}
	for _, e := range s.Entries {
		out = append(out,
			e.Member.Name, e.Member.Glyph, e.Member.Romaji, e.Member.Role,
			e.Member.Tagline, "「"+e.Member.SkillName+"」",
			e.TriggerTag, e.ClassicDesc, e.DuelDesc,
		)
	}
	return out
}

func cardFontCacheDir() (string, error) {
	base := os.Getenv("XDG_CACHE_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".cache")
	}
	return filepath.Join(base, "mygo-minesweeper", "fonts", cardFontVersion), nil
}

var cardFontOnce sync.Once
var cardFontErr error

func registerCardFonts(t *testing.T, texts []string) {
	t.Helper()
	dir, err := cardFontCacheDir()
	if err != nil {
		t.Fatalf("resolve font cache dir: %v", err)
	}
	fontFiles := []string{
		filepath.Join(dir, cardFontRegular),
		filepath.Join(dir, cardFontMedium),
	}
	var payloads [][]byte
	for _, path := range fontFiles {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read full WenKai font %s (run `go generate` once to populate cache): %v", path, err)
		}
		verifyFontCoversTexts(t, filepath.Base(path), data, texts)
		payloads = append(payloads, data)
	}
	cardFontOnce.Do(func() {
		for _, data := range payloads {
			if err := ui.RegisterFont(data, cardFontFamily); err != nil {
				cardFontErr = err
				return
			}
		}
	})
	if cardFontErr != nil {
		t.Fatalf("register card fonts: %v", cardFontErr)
	}
}

func verifyFontCoversTexts(t *testing.T, label string, ttfData []byte, texts []string) {
	t.Helper()
	f, err := sfnt.Parse(ttfData)
	if err != nil {
		t.Fatalf("parse font %s: %v", label, err)
	}
	var buf sfnt.Buffer
	seen := make(map[rune]bool)
	var missing []string
	for _, s := range texts {
		for _, r := range s {
			if unicode.IsSpace(r) || seen[r] {
				continue
			}
			seen[r] = true
			gid, err := f.GlyphIndex(&buf, r)
			if err != nil || gid == 0 {
				missing = append(missing, fmt.Sprintf("%q (U+%04X)", r, r))
			}
		}
	}
	if len(missing) > 0 {
		t.Fatalf("%s is missing %d card glyphs: %s", label, len(missing), strings.Join(missing, ", "))
	}
}

func cardTheme() *ui.Theme {
	t := ui.LightTheme().WithAccent(bandColor)
	t.Background = dayPaper.Desk
	t.Surface = dayPaper.Sheet
	t.Border = dayPaper.Rule
	t.Text = dayPaper.Ink
	t.TextMuted = dayPaper.InkMuted
	t.Font = cardFontFamily
	return t
}

func paintGraphPaper(p *ui.Painter, r ui.Rect, radius, step float32, bg ui.Color) {
	p.Clip(r, radius, func() {
		p.Fill(r, bg, radius)
		for x := r.X + step; x < r.X+r.W; x += step {
			p.Line(x, r.Y, x, r.Y+r.H, 1, dayPaper.Grid.Alpha(0.55))
		}
		for y := r.Y + step; y < r.Y+r.H; y += step {
			p.Line(r.X, y, r.X+r.W, y, 1, dayPaper.Grid.Alpha(0.55))
		}
	})
}

func paintMaskingTape(p *ui.Painter, cx, cy, w, h, angle float32, tint ui.Color) {
	tape := ui.Rect{X: cx - w/2, Y: cy - h/2, W: w, H: h}
	p.Transform(ui.Rotation(angle), cx, cy, func() {
		p.Fill(tape, tint, 1.5)
	})
}

func cardSheet(c *ui.Context, key string, tapeColor ui.Color, tapeAngle float32) ui.Element {
	return ui.Column(c.Key(key)).Grow(1).FillHeight().Padding(16, 15, 13, 15).Gap(9).Radius(8).
		Background(dayPaper.Sheet).Border(1, dayPaper.Rule).
		Shadow(0, 2, 6, 0, dayPaper.SlipShade).
		Draw(func(p *ui.Painter, r ui.Rect) {
			paintGraphPaper(p, r, 8, 22, dayPaper.Sheet)
		}).
		DrawOver(func(p *ui.Painter, r ui.Rect) {
			paintMaskingTape(p, r.X+52, r.Y+1, 60, 12, tapeAngle, tapeColor)
		})
}

func cardDuelSheet(c *ui.Context, key string) ui.Element {
	return ui.Column(c.Key(key)).Grow(1).FillHeight().Padding(16, 15, 13, 15).Gap(9).Radius(8).
		Background(dayPaper.Sheet).Border(1, dayPaper.Rule).
		Shadow(0, 2, 6, 0, dayPaper.SlipShade).
		Draw(func(p *ui.Painter, r ui.Rect) {
			paintGraphPaper(p, r, 8, 22, dayPaper.Sheet)
		}).
		DrawOver(func(p *ui.Painter, r ui.Rect) {
			paintMaskingTape(p, r.X+42, r.Y+1, 40, 12, -3.5, duelPalette.Player.Alpha(0.52))
			paintMaskingTape(p, r.X+76, r.Y+1, 40, 12, 3.0, duelPalette.AI.Alpha(0.52))
		})
}

func gameplayCardView(spec gameplayCardSpec) func(*ui.Context) {
	bandInk := dayPaper.InkOf(bandColor)
	playerInk := dayPaper.InkOf(duelPalette.Player)
	aiInk := dayPaper.InkOf(duelPalette.AI)
	soyoColor := memberImage(MemberSoyorin)
	soyoInk := dayPaper.InkOf(soyoColor)

	return func(c *ui.Context) {
		c.SetTheme(cardTheme())
		ui.Column(c).Fill().Background(dayPaper.Desk).Padding(16, 20, 18, 20).Gap(13).Children(func() {
			// Header
			ui.Row(c).FillWidth().AlignItems(ui.Center).Justify(ui.SpaceBetween).Padding(0, 2, 6, 2).
				Draw(func(p *ui.Painter, r ui.Rect) {
					p.Line(r.X, r.Y+r.H-0.5, r.X+r.W, r.Y+r.H-0.5, 1, dayPaper.Rule)
				}).Children(func() {
				ui.Row(c).AlignItems(ui.Center).Gap(11).Children(func() {
					bandMark(c, 36)
					ui.Column(c).Gap(2).Children(func() {
						ui.Text(c, spec.Title).FontSize(20).Bold().TextColor(dayPaper.Ink)
						ui.Text(c, spec.Subtitle).FontSize(11.5).TextColor(dayPaper.InkMuted)
					})
				})

				ui.Row(c).AlignItems(ui.Center).Gap(10).Children(func() {
					ui.Row(c).AlignItems(ui.Center).Gap(5).Children(func() {
						for _, id := range iconOrder {
							ui.Box(c.Key("swatch-"+string(id))).Size(22, 8).Radius(4).
								Background(memberImage(id)).Border(1, dayPaper.Rule)
						}
					})
					ui.Box(c).Padding(4, 10).Radius(6).Background(dayPaper.Sheet).Border(1, dayPaper.Rule).
						Shadow(0, 1, 2, 0, dayPaper.SlipShade.Alpha(0.45)).Children(func() {
						ui.Text(c, spec.ModePill).FontSize(11.5).Bold().TextColor(bandInk)
					})
				})
			})

			// 3 Equal Notebook Columns
			ui.Row(c).Grow(1).FillWidth().FillEqually().AlignItems(ui.Stretch).Gap(14).Children(func() {
				// Column 1: Setlist & Classic
				cardSheet(c, "card-classic", bandColor.Alpha(0.38), -3.2).Children(func() {
					ui.Row(c).AlignItems(ui.Center).Gap(7).Children(func() {
						ui.Image(c, dayArt.Flag).Size(18, 18)
						ui.Text(c, spec.ClassicHeading).FontSize(14.5).Bold().TextColor(bandInk)
					})

					// 7-Track Ladder Inset
					classicTagBg := bandColor.Alpha(0.16).Over(dayPaper.Board)
					classicTagInk := setlistInk(dayPaper, classicTagBg, bandColor)
					duelTagBg := duelPalette.AI.Alpha(0.18).Over(dayPaper.Board)
					duelTagInk := setlistInk(dayPaper, duelTagBg, duelPalette.AI)
					ui.Column(c).FillWidth().Padding(8, 10).Gap(6).Radius(6).
						Background(dayPaper.Board).Border(1, dayPaper.Rule).Children(func() {
						ui.Column(c).FillWidth().Gap(3).Children(func() {
							ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(7).Children(func() {
								ui.Box(c).Shrink(0).Padding(1.5, 6).Radius(4).Background(classicTagBg).Children(func() {
									ui.Text(c, spec.ClassicGroup).FontSize(10.5).Bold().TextColor(classicTagInk).SingleLine()
								})
								ui.Text(c, spec.ModeNote).Grow(1).FontSize(10.5).TextColor(dayPaper.InkMuted).SingleLine()
							})
							ui.Text(c, spec.ClassicTracks).FontSize(11).LineHeight(1.38).TextColor(dayPaper.Ink).FontFeatures("tnum")
						})
						ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(7).Children(func() {
							ui.Box(c).Shrink(0).Padding(1.5, 6).Radius(4).Background(duelTagBg).Children(func() {
								ui.Text(c, spec.DuelGroup).FontSize(10.5).Bold().TextColor(duelTagInk).SingleLine()
							})
							ui.Text(c, spec.DuelTracks).Grow(1).FontSize(11).LineHeight(1.36).TextColor(dayPaper.Ink).FontFeatures("tnum")
						})
					})

					// Classic Highlights
					ui.Column(c).FillWidth().Gap(7).Children(func() {
						for i, pt := range spec.ClassicPoints {
							ui.Row(c.Key(fmt.Sprintf("cp-%d", i))).FillWidth().AlignItems(ui.Start).Gap(7).Children(func() {
								ui.Box(c).Padding(5, 0, 0, 0).Children(func() {
									ui.Box(c).Size(6, 6).Radius(3).Background(bandInk)
								})
								ui.Column(c).Grow(1).Gap(1).Children(func() {
									ui.Text(c, pt.Title).FontSize(12.2).Bold().TextColor(dayPaper.Ink)
									ui.Text(c, pt.Body).FontSize(11.2).LineHeight(1.34).TextColor(dayPaper.InkMuted)
								})
							})
						}
					})
				})

				// Column 2: Duel
				cardDuelSheet(c, "card-duel").Children(func() {
					ui.Row(c).AlignItems(ui.Center).Gap(7).Children(func() {
						ui.Row(c).AlignItems(ui.Center).Gap(3).Children(func() {
							ui.Box(c).Size(10, 10).Radius(3).Background(duelPalette.Player)
							ui.Box(c).Size(10, 10).Radius(3).Background(duelPalette.AI)
						})
						ui.Text(c, spec.DuelHeading).FontSize(14.5).Bold().TextColor(aiInk)
					})
					ui.Text(c, spec.DuelSubtitle).FontSize(11.2).LineHeight(1.35).TextColor(dayPaper.InkMuted)

					ui.Column(c).Grow(1).FillWidth().Gap(9).Children(func() {
						for i, pt := range spec.DuelPoints {
							dotColor := playerInk
							if i == 1 {
								dotColor = dayPaper.BoomInk
							} else if i == 2 {
								dotColor = aiInk
							}
							ui.Row(c.Key(fmt.Sprintf("dp-%d", i))).FillWidth().AlignItems(ui.Start).Gap(7).Children(func() {
								ui.Box(c).Padding(5, 0, 0, 0).Children(func() {
									ui.Box(c).Size(6, 6).Radius(3).Background(dotColor)
								})
								ui.Column(c).Grow(1).Gap(2).Children(func() {
									ui.Row(c).AlignItems(ui.Center).Gap(5).Children(func() {
										ui.Text(c, pt.Title).FontSize(12.5).Bold().TextColor(dayPaper.Ink)
										if pt.Icon != nil {
											ui.Image(c, pt.Icon).Size(14, 14)
										}
									})
									ui.Text(c, pt.Body).FontSize(11.3).LineHeight(1.38).TextColor(dayPaper.InkMuted)
								})
							})
						}
					})

					// Mini Notebook Visual Legend
					lgPlayerBg := duelPalette.Player.Alpha(0.24).Over(dayPaper.Board)
					lgAIBg := duelPalette.AI.Alpha(0.24).Over(dayPaper.Board)
					lgClaimBg := soyoColor.Alpha(0.45).Over(dayPaper.Board)
					ui.Row(c).FillWidth().Padding(6, 8).Radius(6).Background(dayPaper.Board).
						Border(1, dayPaper.Rule).AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
						legendTile(c, "lg-player", lgPlayerBg, duelPalette.Player, "1", setlistInk(dayPaper, lgPlayerBg, duelPalette.Player), nil, spec.LegendLabels[0])
						legendTile(c, "lg-ai", lgAIBg, duelPalette.AI, "2", setlistInk(dayPaper, lgAIBg, duelPalette.AI), nil, spec.LegendLabels[1])
						legendTile(c, "lg-claim", lgClaimBg, soyoInk, "3", setlistInk(dayPaper, lgClaimBg, soyoColor), nil, spec.LegendLabels[2])
						legendTile(c, "lg-flag", dayPaper.Slip, dayPaper.Rule, "", dayPaper.Ink, dayArt.Flag, spec.LegendLabels[3])
						legendTile(c, "lg-knot", dayPaper.Boom.Over(dayPaper.Board), dayPaper.BoomInk.Alpha(0.45), "", dayPaper.Ink, dayArt.KnotHit, spec.LegendLabels[4])
					})
				})

				// Column 3: Controls & Navigation
				cardSheet(c, "card-controls", soyoColor.Alpha(0.62), 2.8).Children(func() {
					ui.Row(c).AlignItems(ui.Center).Gap(7).Children(func() {
						ui.Image(c, dayArt.Shield).Size(18, 18)
						ui.Text(c, spec.ControlHeading).FontSize(14.5).Bold().TextColor(soyoInk)
					})
					ui.Text(c, spec.ControlSub).FontSize(11.2).LineHeight(1.35).TextColor(dayPaper.InkMuted)

					ui.Column(c).Grow(1).FillWidth().Gap(9).Justify(ui.SpaceBetween).Children(func() {
						for i, row := range spec.KeyRows {
							ui.Row(c.Key(fmt.Sprintf("kr-%d", i))).FillWidth().Padding(9, 10).Radius(6).
								Background(dayPaper.Board).Border(1, dayPaper.Rule).
								AlignItems(ui.Center).Gap(9).Children(func() {
								ui.Box(c).Width(68).Padding(4, 6).Radius(5).Center().
									Background(dayPaper.Slip).Border(1, dayPaper.Rule).
									Shadow(0, 1, 2, 0, dayPaper.SlipShade).Children(func() {
									ui.Text(c, row.Key).FontSize(11).Bold().TextColor(dayPaper.Ink).SingleLine()
								})
								ui.Row(c).Grow(1).AlignItems(ui.Center).Gap(5).Children(func() {
									if row.Icon != nil {
										ui.Image(c, row.Icon).Size(15, 15)
									}
									ui.Text(c, row.Desc).Grow(1).FontSize(11.3).LineHeight(1.36).TextColor(dayPaper.Ink)
								})
							})
						}
					})
				})
			})
		})
	}
}

func legendTile(c *ui.Context, key string, bg, border ui.Color, digit string, digitColor ui.Color, prop *ui.SVG, label string) {
	ui.Row(c.Key(key)).AlignItems(ui.Center).Gap(4).Children(func() {
		ui.Box(c).Size(18, 18).Radius(3.5).Center().Background(bg).Border(1, border).Children(func() {
			if prop != nil {
				ui.Image(c, prop).Size(14, 14)
			} else if digit != "" {
				ui.Text(c, digit).FontSize(11).Bold().TextColor(digitColor)
			}
		})
		ui.Text(c, label).FontSize(10.8).TextColor(dayPaper.InkMuted).SingleLine()
	})
}

func membersCardView(spec membersCardSpec) func(*ui.Context) {
	classicInk := dayPaper.InkOf(bandColor)
	duelInk := dayPaper.InkOf(duelPalette.AI)
	tapeAngles := [5]float32{-3.5, 2.8, -2.4, 3.2, -3.0}

	return func(c *ui.Context) {
		c.SetTheme(cardTheme())
		ui.Column(c).Fill().Background(dayPaper.Desk).Padding(16, 20, 18, 20).Gap(13).Children(func() {
			// Header
			ui.Row(c).FillWidth().AlignItems(ui.Center).Justify(ui.SpaceBetween).Padding(0, 2, 6, 2).
				Draw(func(p *ui.Painter, r ui.Rect) {
					p.Line(r.X, r.Y+r.H-0.5, r.X+r.W, r.Y+r.H-0.5, 1, dayPaper.Rule)
				}).Children(func() {
				ui.Row(c).AlignItems(ui.Center).Gap(11).Children(func() {
					bandMark(c, 36)
					ui.Column(c).Gap(2).Children(func() {
						ui.Text(c, spec.Title).FontSize(20).Bold().TextColor(dayPaper.Ink)
						ui.Text(c, spec.Subtitle).FontSize(11.5).TextColor(dayPaper.InkMuted)
					})
				})
				ui.Box(c).Padding(5, 10).Radius(6).Background(dayPaper.Sheet).Border(1, dayPaper.Rule).Children(func() {
					ui.Text(c, spec.RightTip).FontSize(11.2).TextColor(dayPaper.InkMuted)
				})
			})

			// 5 Equal Member Cards
			ui.Row(c).Grow(1).FillWidth().FillEqually().AlignItems(ui.Stretch).Gap(11).Children(func() {
				for i, entry := range spec.Entries {
					m := entry.Member
					mc := memberImage(m.ID)
					roleBg := mc.Alpha(0.28).Over(dayPaper.Sheet)
					roleInk := setlistInk(dayPaper, roleBg, mc)
					skillBg := mc.Alpha(0.24).Over(dayPaper.Sheet)
					skillInk := setlistInk(dayPaper, skillBg, mc)
					trigBg := dayPaper.Sheet.Alpha(0.85).Over(skillBg)
					trigInk := setlistInk(dayPaper, trigBg, mc)
					angle := tapeAngles[i%len(tapeAngles)]

					ui.Column(c.Key(string(m.ID))).Grow(1).FillHeight().Padding(15, 12, 12, 12).Gap(8).Radius(8).
						Background(dayPaper.Sheet).Border(1, dayPaper.Rule).
						Shadow(0, 2, 6, 0, dayPaper.SlipShade).
						Draw(func(p *ui.Painter, r ui.Rect) {
							paintGraphPaper(p, r, 8, 20, dayPaper.Sheet)
						}).
						DrawOver(func(p *ui.Painter, r ui.Rect) {
							paintMaskingTape(p, r.X+r.W/2, r.Y+1, 56, 12, angle, mc.Alpha(0.58))
						}).Children(func() {
						// Avatar + Name + Role + Romaji
						ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(9).Children(func() {
							memberAvatar(c, m, 36, dayPaper)
							ui.Column(c).Grow(1).Gap(2).Children(func() {
								ui.Row(c).FillWidth().AlignItems(ui.Center).Justify(ui.SpaceBetween).Gap(4).Children(func() {
									ui.Text(c, m.Name).FontSize(14.5).Bold().TextColor(dayPaper.Ink).SingleLine()
									ui.Box(c).Padding(1.5, 6).Radius(4).Background(roleBg).Children(func() {
										ui.Text(c, m.Role).FontSize(10.5).Bold().TextColor(roleInk).SingleLine()
									})
								})
								ui.Text(c, m.Romaji).FontSize(11).TextColor(dayPaper.InkMuted).SingleLine()
							})
						})

						// Verbatim Member Tagline
						ui.Text(c, m.Tagline).FontSize(11).TextColor(dayPaper.InkMuted).SingleLine()

						// Skill Name Strip
						ui.Row(c).FillWidth().Padding(6, 7).Radius(6).
							Background(skillBg).
							AlignItems(ui.Center).Justify(ui.SpaceBetween).Gap(4).Children(func() {
							ui.Row(c).AlignItems(ui.Center).Gap(5).Children(func() {
								ui.Box(c).Size(3, 13).Radius(1.5).Background(skillInk)
								ui.Text(c, "「"+m.SkillName+"」").FontSize(12).Bold().TextColor(skillInk).SingleLine()
							})
							ui.Box(c).Padding(1.5, 5).Radius(4).Background(trigBg).
								Border(1, dayPaper.Rule).Children(func() {
								ui.Text(c, entry.TriggerTag).FontSize(9.8).Bold().TextColor(trigInk).SingleLine()
							})
						})

						// Classic & Duel Effect Boxes
						ui.Column(c).Grow(1).FillWidth().Gap(7).Children(func() {
							ui.Column(c).Grow(1).FillWidth().Padding(7, 8).Gap(3).Radius(6).
								Background(dayPaper.Board).Border(1, dayPaper.Rule).Children(func() {
								ui.Row(c).AlignItems(ui.Center).Gap(5).Children(func() {
									ui.Box(c).Size(5, 5).Radius(2.5).Background(classicInk)
									ui.Text(c, spec.Classic).FontSize(10.8).Bold().TextColor(classicInk)
								})
								ui.Text(c, entry.ClassicDesc).FontSize(11.2).LineHeight(1.36).TextColor(dayPaper.Ink)
							})

							ui.Column(c).Grow(1).FillWidth().Padding(7, 8).Gap(3).Radius(6).
								Background(dayPaper.Board).Border(1, dayPaper.Rule).Children(func() {
								ui.Row(c).AlignItems(ui.Center).Gap(5).Children(func() {
									ui.Box(c).Size(5, 5).Radius(2.5).Background(duelInk)
									ui.Text(c, spec.Duel).FontSize(10.8).Bold().TextColor(duelInk)
								})
								ui.Text(c, entry.DuelDesc).FontSize(11.2).LineHeight(1.36).TextColor(dayPaper.Ink)
							})
						})
					})
				}
			})
		})
	}
}

func renderCardPNG(t *testing.T, width, height int, view func(*ui.Context)) *image.RGBA {
	t.Helper()
	tt := ui.NewTester(view, width, height)
	tt.SetScale(cardScale)
	tt.Frame()
	return tt.Image()
}

func writeCardPNG(t *testing.T, path string, img image.Image) {
	t.Helper()
	var buf bytes.Buffer
	enc := png.Encoder{CompressionLevel: png.BestCompression}
	if err := enc.Encode(&buf, img); err != nil {
		t.Fatalf("encode %s: %v", path, err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestCardsExport(t *testing.T) {
	if os.Getenv("MYGO_CARDS_EXPORT") == "" {
		t.Skip("set MYGO_CARDS_EXPORT=1 to export README overview cards: " + cardsExportUsage)
	}
	gp := gameplayCardData()
	mb := membersCardData()
	allTexts := append(gp.allStrings(), mb.allStrings()...)
	registerCardFonts(t, allTexts)

	writeCardPNG(t, gameplayCardPath, renderCardPNG(t, gameplayCardW, gameplayCardH, gameplayCardView(gp)))
	writeCardPNG(t, membersCardPath, renderCardPNG(t, membersCardW, membersCardH, membersCardView(mb)))
}

func TestCardCopyContracts(t *testing.T) {
	gp := gameplayCardData()
	mb := membersCardData()

	gpJoined := strings.Join(gp.allStrings(), "\n")
	mbJoined := strings.Join(mb.allStrings(), "\n")

	// Every track in Difficulties must appear with its song name and grid size.
	for _, d := range Difficulties {
		want := fmt.Sprintf("《%s》%d×%d", d.Song, d.Cols, d.Rows)
		if !strings.Contains(gpJoined, want) {
			t.Errorf("gameplay card missing track %q", want)
		}
	}

	// Required in-game labels and terminology must stay aligned.
	for _, required := range []string{
		"乐队 Live",
		"纯正经典",
		"一圈圈自动翻开",
		"代表色荧光笔",
		"连开",
		"确定全雷时左键点数字收雷 · +75/雷",
		"踩雷 −100",
		"2.4 秒",
		"插旗模式 (F)",
		"经典曲目可开启「? 标记」循环",
		"有充能时",
		"拖动平移",
		"滚轮缩放",
		"回到中心",
		"定位高松灯",
		"为什么要演奏春日影！？",
	} {
		if !strings.Contains(gpJoined, required) {
			t.Errorf("gameplay card missing aligned in-game copy %q", required)
		}
	}

	if !strings.Contains(mbJoined, "经典曲目 · 乐队 Live") {
		t.Errorf("members card must note 经典曲目 · 乐队 Live so classic-mode skills are clear")
	}

	// Every band member must appear verbatim with official tagline and skill name.
	if len(mb.Entries) != len(Members) {
		t.Fatalf("members card has %d entries, want %d", len(mb.Entries), len(Members))
	}
	for _, m := range Members {
		for _, want := range []string{m.Name, m.Role, m.Tagline, "「" + m.SkillName + "」"} {
			if !strings.Contains(mbJoined, want) {
				t.Errorf("members card missing %s's %q", m.Name, want)
			}
		}
	}

	bannedPhrases := []string{
		"百万格决斗", "对战", "PVP", "HOW TO PLAY", "BAND MEMBERS",
		"BFS", "Chord", "Live 完成度", "Gravity",
		"主题色", "领队色", "成员色", "应援色",
		"标旗", "地雷", "炸弹", "挡雷", "免死",
		"档位", "关卡", "难度", "进度条",
	}
	wordAI := regexp.MustCompile(`(?i)\bAI\b`)
	allCapsLabel := regexp.MustCompile(`\b[A-Z]{3,}\b`)

	// Verbatim member taglines and the exact in-game "? 标记" toggle are
	// explicitly preserved per AGENTS.md and ADR 0001.
	taglines := make(map[string]bool, len(Members))
	for _, m := range Members {
		taglines[m.Tagline] = true
	}

	checkStrings := func(cardName string, list []string) {
		for _, s := range list {
			for _, bad := range bannedPhrases {
				if strings.Contains(strings.ToLower(s), strings.ToLower(bad)) {
					t.Errorf("%s contains banned term %q in %q", cardName, bad, s)
				}
			}
			if wordAI.MatchString(s) {
				t.Errorf("%s contains technical term AI in %q", cardName, s)
			}
			if !taglines[s] {
				if strings.Contains(s, "石头") {
					t.Errorf("%s calls a mine 石头 in %q", cardName, s)
				}
				if m := allCapsLabel.FindString(s); m != "" {
					t.Errorf("%s contains all-caps label %q in %q", cardName, m, s)
				}
			}
			withoutToggle := strings.ReplaceAll(s, "? 标记", "")
			if strings.Contains(withoutToggle, "标记") {
				t.Errorf("%s uses 标记 outside the ? 标记 toggle in %q", cardName, s)
			}
		}
	}
	checkStrings("card-gameplay", gp.allStrings())
	checkStrings("card-members", mb.allStrings())
}

func decodeCardPNG(t *testing.T, path string, wantW, wantH int) image.Image {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
	b := img.Bounds()
	if b.Dx() != wantW || b.Dy() != wantH {
		t.Fatalf("%s is %dx%d, want %dx%d (run %s)", path, b.Dx(), b.Dy(), wantW, wantH, cardsExportUsage)
	}
	return img
}

func colorNear(a color.NRGBA, target ui.Color, tol int) bool {
	diff := func(u, v uint8) int {
		d := int(u) - int(v)
		if d < 0 {
			return -d
		}
		return d
	}
	return diff(a.R, target.R) <= tol && diff(a.G, target.G) <= tol && diff(a.B, target.B) <= tol
}

func countPaletteHits(img image.Image, targets []ui.Color, tol int) (hits []int, lightPaper, darkNavy, total int) {
	hits = make([]int, len(targets))
	oldNavy := ui.Hex("#0B0F19")
	b := img.Bounds()
	// Sample every 2nd pixel along both axes so CI checks stay sub-second
	// while still inspecting >400k pixels per card.
	for y := b.Min.Y; y < b.Max.Y; y += 2 {
		for x := b.Min.X; x < b.Max.X; x += 2 {
			px := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
			total++
			if px.R >= 225 && px.G >= 218 && px.B >= 200 {
				lightPaper++
			}
			if colorNear(px, oldNavy, 18) {
				darkNavy++
			}
			for i, target := range targets {
				if colorNear(px, target, tol) {
					hits[i]++
				}
			}
		}
	}
	return hits, lightPaper, darkNavy, total
}

func TestCardAssetsMatchNotebookPalette(t *testing.T) {
	type swatch struct {
		name  string
		color ui.Color
		minPx int
	}

	gpImg := decodeCardPNG(t, gameplayCardPath, gameplayCardW*cardScale, gameplayCardH*cardScale)
	gpSwatches := []swatch{
		{"dayPaper.Desk", dayPaper.Desk, 10000},
		{"dayPaper.Sheet", dayPaper.Sheet, 20000},
		{"dayPaper.Ink", dayPaper.Ink, 500},
		{"bandColor ink", dayPaper.InkOf(bandColor), 200},
		{"duelPalette.Player", duelPalette.Player, 80},
		{"duelPalette.AI", duelPalette.AI, 80},
	}
	for _, m := range Members {
		gpSwatches = append(gpSwatches, swatch{m.Name + " 代表色", memberImage(m.ID), 40})
	}

	gpColors := make([]ui.Color, len(gpSwatches))
	for i, sw := range gpSwatches {
		gpColors[i] = sw.color
	}
	hits, light, navy, total := countPaletteHits(gpImg, gpColors, 12)
	if float64(light)/float64(total) < 0.65 {
		t.Errorf("%s light paper ratio = %.2f, want >= 0.65", gameplayCardPath, float64(light)/float64(total))
	}
	if float64(navy)/float64(total) > 0.01 {
		t.Errorf("%s still contains legacy dark-navy pixels (%.2f%%)", gameplayCardPath, 100*float64(navy)/float64(total))
	}
	for i, sw := range gpSwatches {
		if hits[i] < sw.minPx {
			t.Errorf("%s missing %s (%s): sampled %d px, want >= %d", gameplayCardPath, sw.name, hexOf(sw.color), hits[i], sw.minPx)
		}
	}

	mbImg := decodeCardPNG(t, membersCardPath, membersCardW*cardScale, membersCardH*cardScale)
	mbSwatches := []swatch{
		{"dayPaper.Desk", dayPaper.Desk, 10000},
		{"dayPaper.Sheet", dayPaper.Sheet, 20000},
		{"dayPaper.Ink", dayPaper.Ink, 500},
	}
	for _, m := range Members {
		mc := memberImage(m.ID)
		skillInk := setlistInk(dayPaper, mc.Alpha(0.24).Over(dayPaper.Sheet), mc)
		mbSwatches = append(mbSwatches,
			swatch{m.Name + " 代表色", mc, 120},
			swatch{m.Name + " 墨色", skillInk, 60},
		)
	}
	mbColors := make([]ui.Color, len(mbSwatches))
	for i, sw := range mbSwatches {
		mbColors[i] = sw.color
	}
	hits, light, navy, total = countPaletteHits(mbImg, mbColors, 12)
	if float64(light)/float64(total) < 0.65 {
		t.Errorf("%s light paper ratio = %.2f, want >= 0.65", membersCardPath, float64(light)/float64(total))
	}
	if float64(navy)/float64(total) > 0.01 {
		t.Errorf("%s still contains legacy dark-navy pixels (%.2f%%)", membersCardPath, 100*float64(navy)/float64(total))
	}
	for i, sw := range mbSwatches {
		if hits[i] < sw.minPx {
			t.Errorf("%s missing %s (%s): sampled %d px, want >= %d", membersCardPath, sw.name, hexOf(sw.color), hits[i], sw.minPx)
		}
	}
}
