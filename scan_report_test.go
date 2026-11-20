package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"mygo-minesweeper/internal/scanreport"
)

// TestSourceScanReport fails on copy and raw colour violations.
func TestSourceScanReport(t *testing.T) {
	rep, err := scanreport.Scan(".")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("\n%s", rep)
	for _, f := range rep.Copy {
		t.Errorf("copy violation at %s:%d [%s] %q", f.Path, f.Line, strings.Join(f.Rules, "、"), f.Text)
	}
	for _, f := range rep.Colors {
		t.Errorf("raw colour violation at %s:%d %q", f.Path, f.Line, f.Text)
	}
}

func TestSourceScanReportGate(t *testing.T) {
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		file string
		decl string
		want string
	}{
		{"banned wording", "game.go", `var label = "百万格决斗"`, `copy violation at game.go:3 [禁用语「百万格决斗」] "百万格决斗"`},
		{"hash hex", "game.go", `var colour = "#FFF"`, `raw colour violation at game.go:3 "#FFF"`},
		{"bare Hex argument", "game.go", `var colour = ui.Hex("AABBCC")`, `raw colour violation at game.go:3 "AABBCC"`},
		{"RGB call", "game.go", `var colour = ui.RGB(1, 2, 3)`, `raw colour violation at game.go:3 "ui.RGB(1, 2, 3)"`},
		{"RGBA call", "game.go", `var colour = ui.RGBA(1, 2, 3, 0.5)`, `raw colour violation at game.go:3 "ui.RGBA(1, 2, 3, 0.5)"`},
		{"nonempty Color", "game.go", `var colour = ui.Color{R: 1}`, `raw colour violation at game.go:3 "ui.Color{R: 1}"`},
		{"elided Color type", "game.go", `var colours = []ui.Color{{R: 1}}`, `raw colour violation at game.go:3 "{R: 1}"`},
		{"palette", "palette.go", `var colour = ui.Hex("AABBCC")`, ""},
		{"palette banned wording", "palette.go", `var colour = ui.Hex("百万格决斗")`, `copy violation at palette.go:3 [禁用语「百万格决斗」] "百万格决斗"`},
		{"member table", "memes.go", `var colour = ui.RGB(1, 2, 3)`, ""},
		{"test file", "game_test.go", `var colour = ui.Color{R: 1}`, ""},
		{"empty Color", "game.go", `var colour = ui.Color{}`, ""},
		{"Hex from member", "game.go", `var colour = ui.Hex(m.ColorHex)`, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			src := "package main\nimport \"github.com/egoist/mygo/ui\"\n" + tc.decl + "\n"
			if err := os.WriteFile(filepath.Join(dir, tc.file), []byte(src), 0o600); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command(binary, "-test.run=^TestSourceScanReport$", "-test.v")
			cmd.Dir = dir
			out, err := cmd.CombinedOutput()
			t.Logf("\n%s", out)
			if tc.want == "" {
				if err != nil {
					t.Fatalf("expected the gate to pass, got %v", err)
				}
				return
			}
			if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 1 {
				t.Fatalf("expected the gate to exit with status 1, got %v", err)
			}
			if !strings.Contains(string(out), tc.want) {
				t.Errorf("missing diagnostic %q", tc.want)
			}
		})
	}
}

// TestSourceScanWellFormed checks the scan of the real tree stays wired:
// every finding cites a shipped source file, a line and the offending
// source text. Detection itself is covered by scanreport's fixture test.
func TestSourceScanWellFormed(t *testing.T) {
	rep, err := scanreport.Scan(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, list := range [][]scanreport.Finding{rep.Copy, rep.Colors} {
		for _, f := range list {
			if f.Path == "" || f.Line == 0 || f.Text == "" || len(f.Rules) == 0 {
				t.Errorf("malformed finding: %+v", f)
			}
			if strings.HasSuffix(f.Path, "_test.go") {
				t.Errorf("report scanned test file %s", f.Path)
			}
		}
	}
}
