package scanreport

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestCopyRules(t *testing.T) {
	cases := []struct {
		name  string
		text  string
		rules []string
	}{
		{"avoid term", "百万格决斗", []string{RuleAvoidTerm}},
		{"avoid term in label", "百万格决斗战场", []string{RuleAvoidTerm}},
		{"avoid term plus duration", "「百万格决斗只剩五分钟」", []string{RuleAvoidTerm, RuleDuelSpec}},
		{"duel size", "「百万格超大舞台？这波抢地盘流量我吃定了！」", []string{RuleDuelSpec}},
		{"duel size spelled out", "「一百万个格子……迷路的时候，也能一格一格继续前进。」", []string{RuleDuelSpec}},
		{"duel duration", "五分钟百万格抢地：开格 +1，踩雷 −100", []string{RuleDuelSpec}},
		{"duel size in result", "平局！这一百万格的迷茫，恰好一人一半", []string{RuleDuelSpec}},
		{"duel cells spelled out", "「二十五万个格子，我慢慢收。」", []string{RuleDuelSpec}},
		{"duel cells in digits", "「250,000 个格子全是我的」", []string{RuleDuelSpec}},
		{"duel big grid", "「这 500×500 的棋盘归我了」", []string{RuleDuelSpec}},
		{"duel ascii grid", "「500x500 全归我」", []string{RuleDuelSpec}},
		{"duel small grid in context", "「这片 20×20 的格子全是我的」", []string{RuleDuelSpec}},
		{"duel clock", "「限时 05:00 推完就算赢」", []string{RuleDuelSpec}},
		{"duel seconds", "「决斗限时 300 秒见胜负」", []string{RuleDuelSpec}},
		{"duel big seconds", "「180 秒够赢你了！」", []string{RuleDuelSpec}},
		{"duel countdown", "「决斗只剩 7 秒了，快推！」", []string{RuleDuelSpec}},
		{"formatted seconds allowed", "「决斗限时 %d 秒，准备好了吗？」", nil},
		{"idiom seconds allowed", "「决斗中争分夺秒，一起加油吧！」", nil},
		{"mechanic seconds allowed", "「决斗中扫描持续 7 秒，按 E 使用」", nil},
		{"stun duration allowed", "「硬直时长 7 秒」", nil},
		{"few cells not size", "「就差最后几格了」", nil},
		{"idiom cell not size", "「万一这格是雷」", nil},
		{"idiom cell percent", "「百分百安全格」", nil},
		{"scan reach not board", "决斗：空降抢占领地并触发 9×9 扫描", nil},
		{"skill stun not match", "经典：精准标雷；决斗：扫描 7 秒并延迟 AI", nil},
		{"track spec allowed", "1000×1000 · 180000雷 · 05:00", nil},
		{"grouped spec allowed", "500×500 · 45,000雷 · 03:00", nil},
		{"classic spec allowed", "9×9 · 10雷", nil},
		{"bare dimension allowed", "500×500", nil},
		{"formatted spec allowed", "%d×%d · %d雷 · %d分钟", nil},
		{"skill scan area", "9×9 扫描：%d 雷", nil},
		{"stun seconds not duel spec", "踩雷 −100 · 硬直 2.4 秒！别把春日影点出来", nil},
		{"score format not clock", "%02d:%02d", nil},
		{"tech term", "OpenGL 渲染", []string{RuleTechTerm}},
		{"tech term case", "ebitengine", []string{RuleTechTerm}},
		{"framework name", "「迷子でもいい、迷子でも進め。」 · egoist/mygo Native UI", []string{RuleTechTerm}},
		{"versioned api", "Direct3D11", []string{RuleTechTerm}},
		{"attached fps", "60FPS 流畅", []string{RuleTechTerm}},
		{"bare fps", "60fps", []string{RuleTechTerm}},
		{"shout label", "MINES ", []string{RuleTechTerm}},
		{"shout label digits", "BEST --", []string{RuleTechTerm}},
		{"shout label fmt", "BEST %ds", []string{RuleTechTerm}},
		{"font name not shout", "LXGW WenKai", nil},
		{"romaji not shout", "Tomori", nil},
		{"svg path not shout", "M6 6L18 18M18 6L6 18", nil},
		{"hex colour not copy", "#FDF2F8", nil},
		{"short hex not copy", "#FFF", nil},
		{"member colour not copy", "#77BBDD", nil},
		{"clean line", "「就算会迷路，也要一块石头一块石头地排下去……」", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := copyRules(tc.text); !slices.Equal(got, tc.rules) {
				t.Errorf("%q: got %v, want %v", tc.text, got, tc.rules)
			}
		})
	}
}

// TestScan proves end-to-end detection on a planted tree: violations are
// found with positions, sanctioned colour files and _test.go files are
// skipped, and clean literals produce no findings.
func TestScan(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"game.go": "package x\n" +
			"import \"github.com/egoist/mygo/ui\"\n" + // import paths are skipped
			"var a = \"百万格决斗\"\n" + // copy finding, line 3
			"var b = \"9×9 扫描：%d 雷\"\n" + // scan reach, clean
			"var c = \"#FFF\"\n" + // short raw colour, line 5
			"var d = \"fill=\\\"#FF8899\\\"\"\n" + // embedded colour, line 6
			"var e = \"「这片 9×9 的格子归我了」\"\n" + // hardcoded size, line 7
			"var f = ui.Hex(\"AABBCC\")\n" +
			"var g = ui.RGB(1, 2, 3)\n" +
			"var h = ui.RGBA(1, 2, 3, 0.5)\n" +
			"var i = ui.Color{R: 1}\n" +
			"var j = ui.Hex(\"#FFF\")\n" +
			"var k = ui.Color{}\n" +
			"var l = ui.Hex(m.ColorHex)\n" +
			"func fakes(ui interface{ RGB(int, int, int) int }) { ui.RGB(1, 2, 3) }\n",
		"aliased.go": "package x\nimport _ \"github.com/egoist/mygo/ui\"\nimport paint \"github.com/egoist/mygo/ui\"\nvar a = paint.RGB(1, 2, 3)\n",
		"dot.go": `package x
import . "github.com/egoist/mygo/ui"
var a = RGB(1, 2, 3)
var b = Hex("AABBCC")
var c = Color{R: 1}
var d = []*Color{1: {R: 2}}
var e = map[Color][]Color{{R: 3}: {{R: 4}, {}}}
var f = []Color{{}}
func local(RGB func(int, int, int) int) { _ = RGB(1, 2, 3) }
`,
		"palette.go": "package x\nimport \"github.com/egoist/mygo/ui\"\nvar p = ui.Hex(\"#AABBCC\")\nvar q = ui.Color{R: 1}\n",
		// The member table is exempt from the colour report only;
		// its copy is still scanned.
		"memes.go":     "package x\nvar m = \"#77BBDD\"\nvar q = \"「百万格超大舞台」\"\n",
		"game_test.go": "package x\nimport \"github.com/egoist/mygo/ui\"\nvar t = ui.Hex(\"#FFF\")\nvar u = \"百万格决斗\"\n",
	}
	for name, src := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	rep, err := Scan(dir)
	if err != nil {
		t.Fatal(err)
	}
	game := filepath.Join(dir, "game.go")
	copyAt := func(path string, line int, frag string) []string {
		for _, f := range rep.Copy {
			if f.Path == path && f.Line == line && strings.Contains(f.Text, frag) {
				return f.Rules
			}
		}
		return nil
	}
	if rules := copyAt(game, 3, "百万格决斗"); !slices.Equal(rules, []string{RuleAvoidTerm}) {
		t.Errorf("line 3: got %v, want [%s]", rules, RuleAvoidTerm)
	}
	if rules := copyAt(game, 4, "9×9"); len(rules) != 0 {
		t.Errorf("line 4: got %v, want clean", rules)
	}
	if rules := copyAt(game, 7, "9×9"); !slices.Equal(rules, []string{RuleDuelSpec}) {
		t.Errorf("line 7: got %v, want [%s]", rules, RuleDuelSpec)
	}
	if rules := copyAt(filepath.Join(dir, "memes.go"), 3, "百万格"); !slices.Equal(rules, []string{RuleDuelSpec}) {
		t.Errorf("memes.go:3: got %v, want [%s]", rules, RuleDuelSpec)
	}
	if len(rep.Copy) != 3 {
		t.Errorf("copy findings = %d, want 3: %+v", len(rep.Copy), rep.Copy)
	}
	for _, list := range [][]Finding{rep.Copy, rep.Colors} {
		for _, f := range list {
			if strings.HasSuffix(f.Path, "_test.go") {
				t.Errorf("finding from test file %s", f.Path)
			}
			if strings.Contains(f.Text, "egoist") {
				t.Errorf("finding from import path %s:%d", f.Path, f.Line)
			}
		}
	}
	wantColors := []Finding{
		{filepath.Join(dir, "aliased.go"), 4, []string{RuleRawColor}, "paint.RGB(1, 2, 3)"},
		{filepath.Join(dir, "dot.go"), 3, []string{RuleRawColor}, "RGB(1, 2, 3)"},
		{filepath.Join(dir, "dot.go"), 4, []string{RuleRawColor}, "AABBCC"},
		{filepath.Join(dir, "dot.go"), 5, []string{RuleRawColor}, "Color{R: 1}"},
		{filepath.Join(dir, "dot.go"), 6, []string{RuleRawColor}, "{R: 2}"},
		{filepath.Join(dir, "dot.go"), 7, []string{RuleRawColor}, "{R: 3}"},
		{filepath.Join(dir, "dot.go"), 7, []string{RuleRawColor}, "{R: 4}"},
		{game, 5, []string{RuleRawColor}, "#FFF"},
		{game, 6, []string{RuleRawColor}, `fill="#FF8899"`},
		{game, 8, []string{RuleRawColor}, "AABBCC"},
		{game, 9, []string{RuleRawColor}, "ui.RGB(1, 2, 3)"},
		{game, 10, []string{RuleRawColor}, "ui.RGBA(1, 2, 3, 0.5)"},
		{game, 11, []string{RuleRawColor}, "ui.Color{R: 1}"},
		{game, 12, []string{RuleRawColor}, "#FFF"},
	}
	if !slices.EqualFunc(rep.Colors, wantColors, func(got, want Finding) bool {
		return got.Path == want.Path && got.Line == want.Line && got.Text == want.Text && slices.Equal(got.Rules, want.Rules)
	}) {
		t.Errorf("colour findings = %+v, want %+v", rep.Colors, wantColors)
	}
	if s := rep.String(); !strings.Contains(s, "game.go:3") || !strings.Contains(s, "文案违规") {
		t.Errorf("report text missing findings:\n%s", s)
	}
}
