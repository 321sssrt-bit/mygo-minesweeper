// Package scanreport audits Go source literals for the notebook art
// direction's copy and colour constraints (issue #5): banned wording,
// technical terms, duel lines hardcoding board size or duration, and raw
// colour values outside the palette. The source test fails on copy
// violations. Raw colours remain report-only until issue #24.
package scanreport

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"mygo-minesweeper/internal/glyphset"
)

const bannedTerm = "百万格决斗"

// Rule labels attached to each finding.
const (
	RuleAvoidTerm = "禁用语「" + bannedTerm + "」"
	RuleTechTerm  = "技术字样"
	RuleDuelSpec  = "决斗台词写死棋盘尺寸或时长"
	RuleRawColor  = "调色板之外的裸色值"
)

// Finding is one literal that breaks a rule.
type Finding struct {
	Path  string
	Line  int
	Rules []string
	Text  string
}

// Report holds the two lists the ticket asks for: copy violations and raw
// colour values.
type Report struct {
	Copy   []Finding
	Colors []Finding
}

const hexPattern = `#(?:[0-9A-Fa-f]{3}|[0-9A-Fa-f]{4}|[0-9A-Fa-f]{6}|[0-9A-Fa-f]{8})`

var (
	// Framework (including egoist/mygo's "Native UI"), graphics-API and
	// developer jargon that must not reach the player, per the spec's
	// "no technical voice" constraint; trailing \d* catches versioned APIs
	// like Direct3D11.
	techTerms = regexp.MustCompile(`(?i)\b(ebiten|ebitengine|opengl|direct3d|directx|vulkan|metal|webgpu|webview|cgo|gpu|shader|\d+\s*fps|fps|hud|api|gui|debug|egoist|mygo/ui|native\s*ui)\d*\b`)
	// Board-size and match-duration words that pin duel copy to one track's
	// numbers: 百万, N百/千/万(个)格 or big digit counts of cells, numerals+分钟,
	// 3+-digit grids, m:ss. A numeral before 分钟 is required so "%d 分钟"
	// stays a format; 几格 ("a few cells") states no size.
	duelSpec = regexp.MustCompile(`百万|[百千万]个?格|[\d,]{4,}\s*个?格|[0-9一二三四五六七八九十百千万两半]+\s*分钟|\d{3,}\s*[×xX]\s*\d{3,}|\d+:\d{2}`)
	// A small N×N is a violation only beside board wording — "20×20 的棋盘"
	// hardcodes the board while "领地…并触发 9×9 扫描" is the skill's reach.
	dimCtx = regexp.MustCompile(`(?:棋盘|格|尺寸|大小|领地|地盘).{0,3}\d+\s*[×xX]\s*\d+|\d+\s*[×xX]\s*\d+.{0,3}(?:棋盘|格|尺寸|大小|领地|地盘)`)
	// 秒 counts only with a fixed numeral — match-scale counts (180 秒,
	// 三百秒) unconditionally, smaller ones beside a match-duration word;
	// mechanic seconds (硬直/扫描/持续/冷却/充能) are tuning, not match
	// time, and "%d 秒"/争分夺秒 carry no number at all.
	numSeconds  = regexp.MustCompile(`[0-9一二三四五六七八九十百千万两半]+\s*秒`)
	bigSeconds  = regexp.MustCompile(`(?:[0-9]{3,}|[一二三四五六七八九十两]*[百千万][一二三四五六七八九十百千万两]*)\s*秒`)
	timeCtx     = regexp.MustCompile(`限时|时长|决斗|胜负|结束|剩余|只剩`)
	mechanicSec = regexp.MustCompile(`硬直|扫描|持续|冷却|充能`)
	// Track subtitles legitimately state their spec ("9×9 · 10雷",
	// "500×500 · 45,000雷 · 03:00", a bare "500×500"); the constraint
	// covers prose lines, not spec metadata.
	trackSpec = regexp.MustCompile(`^(?:\d+\s*[×xX]\s*\d+|[\d,]+\s*雷|\d+:\d{2}|[0-9,一二三四五六七八九十百千万两半]+\s*分钟|·|\s)+$`)
	// shortcut: only #-prefixed hex literals are caught; bare ui.Hex args
	// and ui.RGB/ui.Color forms need call-site analysis — upgrade when the
	// gate ticket (#24) turns this scan into fail-mode.
	hexColor  = regexp.MustCompile("^" + hexPattern + "$")
	hexInside = regexp.MustCompile(hexPattern + `\b`)
)

// colorFiles may define raw colours: the palette itself, and the member
// table that is the canonical source of each official 代表色.
var colorFiles = map[string]bool{"palette.go": true, "memes.go": true}

var fmtVerb = regexp.MustCompile(`%[\d.]*[a-zA-Z]`)

// shoutLabel reports ASCII dashboard labels like "MINES " or "BEST %ds":
// after fmt verbs are stripped, every word of 2+ letters is all-caps and at
// least one is a 3+-letter shout. Mixed-case words like "WenKai" are prose
// or proper names, not labels.
func shoutLabel(s string) bool {
	s = fmtVerb.ReplaceAllString(s, " ")
	maxCaps, runLen, runCaps := 0, 0, 0
	flush := func() bool {
		if runLen > 1 && runCaps < runLen {
			return false
		}
		if runLen > maxCaps && runCaps == runLen {
			maxCaps = runLen
		}
		runLen, runCaps = 0, 0
		return true
	}
	for _, r := range s {
		switch {
		case r >= 'A' && r <= 'Z':
			runLen++
			runCaps++
		case r >= 'a' && r <= 'z':
			runLen++
		case r > 0x7F:
			return false
		default:
			if !flush() {
				return false
			}
		}
	}
	return flush() && maxCaps >= 3
}

// copyRules returns the rule labels a literal breaks; a literal that is
// itself a colour belongs to the colour report, not the copy one.
func copyRules(text string) []string {
	if hexColor.MatchString(text) {
		return nil
	}
	var rules []string
	if strings.Contains(text, bannedTerm) {
		rules = append(rules, RuleAvoidTerm)
	}
	if techTerms.MatchString(text) || shoutLabel(text) {
		rules = append(rules, RuleTechTerm)
	}
	// The banned term itself is not a hardcoded spec: strip it so labels
	// like "百万格决斗战场" (the label ticket's, not the duel-line ticket's)
	// don't carry a misleading 决斗台词 tag.
	prose := strings.ReplaceAll(text, bannedTerm, "")
	hardcoded := duelSpec.MatchString(prose) ||
		dimCtx.MatchString(prose) ||
		bigSeconds.MatchString(prose) ||
		(numSeconds.MatchString(prose) && timeCtx.MatchString(prose) && !mechanicSec.MatchString(prose))
	if hardcoded && !trackSpec.MatchString(text) {
		rules = append(rules, RuleDuelSpec)
	}
	return rules
}

// Scan audits the non-test Go files directly inside dir.
func Scan(dir string) (*Report, error) {
	lits, err := glyphset.Literals(dir)
	if err != nil {
		return nil, err
	}
	rep := &Report{}
	for _, lit := range lits {
		if lit.IsChar {
			continue
		}
		if !colorFiles[filepath.Base(lit.Path)] && hexInside.MatchString(lit.Text) {
			rep.Colors = append(rep.Colors, Finding{lit.Path, lit.Line, []string{RuleRawColor}, lit.Text})
		}
		if rules := copyRules(lit.Text); len(rules) > 0 {
			rep.Copy = append(rep.Copy, Finding{lit.Path, lit.Line, rules, lit.Text})
		}
	}
	for _, list := range [][]Finding{rep.Copy, rep.Colors} {
		sort.Slice(list, func(i, j int) bool {
			if list[i].Path != list[j].Path {
				return list[i].Path < list[j].Path
			}
			return list[i].Line < list[j].Line
		})
	}
	return rep, nil
}

func (r *Report) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "文案违规（%d 条）\n", len(r.Copy))
	for _, f := range r.Copy {
		fmt.Fprintf(&b, "  %s:%d [%s] %q\n", f.Path, f.Line, strings.Join(f.Rules, "、"), f.Text)
	}
	fmt.Fprintf(&b, "调色板之外的裸色值（%d 条）\n", len(r.Colors))
	for _, f := range r.Colors {
		fmt.Fprintf(&b, "  %s:%d %q\n", f.Path, f.Line, f.Text)
	}
	return b.String()
}
