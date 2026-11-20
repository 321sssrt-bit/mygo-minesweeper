// Package scanreport audits Go source for the notebook art
// direction's copy and colour constraints (issue #5): banned wording,
// technical terms, duel lines hardcoding board size or duration, and raw
// colour values outside the palette. The source test fails on both copy
// and raw colour violations.
package scanreport

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

const bannedTerm = "百万格决斗"

// Rule labels attached to each finding.
const (
	RuleAvoidTerm = "禁用语「" + bannedTerm + "」"
	RuleTechTerm  = "技术字样"
	RuleDuelSpec  = "决斗台词写死棋盘尺寸或时长"
	RuleRawColor  = "调色板之外的裸色值"
)

// Finding is one source occurrence that breaks a rule.
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
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	rep := &Report{}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		path := filepath.Join(dir, name)
		src, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, src, 0)
		if err != nil {
			return nil, err
		}
		rep.scanFile(fset.File(file.Pos()), file, src, !colorFiles[name])
	}
	return rep, nil
}

func (r *Report) scanFile(positions *token.File, file *ast.File, src []byte, checkColors bool) {
	uiName := uiImportName(file)
	addColor := func(pos token.Pos, value string) {
		if checkColors {
			r.Colors = append(r.Colors, Finding{positions.Name(), positions.Line(pos), []string{RuleRawColor}, value})
		}
	}
	expressionText := func(expr ast.Expr) string {
		return string(src[positions.Offset(expr.Pos()):positions.Offset(expr.End())])
	}
	ast.Inspect(file, func(node ast.Node) bool {
		switch n := node.(type) {
		case *ast.ImportSpec:
			return false
		case *ast.CallExpr:
			switch uiSelector(n.Fun, uiName) {
			case "Hex":
				if len(n.Args) == 1 {
					if arg, ok := ast.Unparen(n.Args[0]).(*ast.BasicLit); ok && arg.Kind == token.STRING {
						value, _ := strconv.Unquote(arg.Value)
						addColor(arg.Pos(), value)
						return false
					}
				}
			case "RGB", "RGBA":
				addColor(n.Pos(), expressionText(n))
			}
		case *ast.CompositeLit:
			if len(n.Elts) > 0 && uiSelector(n.Type, uiName) == "Color" {
				addColor(n.Pos(), expressionText(n))
			}
		case *ast.BasicLit:
			if n.Kind == token.STRING {
				value, _ := strconv.Unquote(n.Value)
				if hexInside.MatchString(value) {
					addColor(n.Pos(), value)
				}
				if rules := copyRules(value); len(rules) > 0 {
					r.Copy = append(r.Copy, Finding{positions.Name(), positions.Line(n.Pos()), rules, value})
				}
			}
		}
		return true
	})
}

func uiImportName(file *ast.File) string {
	for _, spec := range file.Imports {
		path, _ := strconv.Unquote(spec.Path.Value)
		if path == "github.com/egoist/mygo/ui" {
			if spec.Name != nil {
				return spec.Name.Name
			}
			return "ui"
		}
	}
	return ""
}

func uiSelector(expr ast.Expr, uiName string) string {
	if selector, ok := ast.Unparen(expr).(*ast.SelectorExpr); ok {
		// A locally declared name has an object; an import name does not.
		if id, ok := selector.X.(*ast.Ident); ok && id.Name == uiName && id.Obj == nil {
			return selector.Sel.Name
		}
	}
	return ""
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
