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
		{"clean tree", "game.go", `var label = "迷星叫"`, ""},
		{"banned wording", "game.go", `var label = "百万格决斗"`, `copy violation at game.go:3 [禁用语「百万格决斗」] "百万格决斗"`},
		{"raw colour", "game.go", `var colour = ui.RGB(1, 2, 3)`, `raw colour violation at game.go:3 "ui.RGB(1, 2, 3)"`},
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
