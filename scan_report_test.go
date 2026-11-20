package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"mygo-minesweeper/internal/scanreport"
)

// TestSourceScanReport fails on copy violations. Raw colours remain
// report-only until issue #24.
func TestSourceScanReport(t *testing.T) {
	rep, err := scanreport.Scan(".")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("\n%s", rep)
	for _, f := range rep.Copy {
		t.Errorf("copy violation at %s:%d [%s] %q", f.Path, f.Line, strings.Join(f.Rules, "、"), f.Text)
	}
}

func TestSourceScanReportGate(t *testing.T) {
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name     string
		extra    string
		wantCopy string
	}{
		{name: "raw colour only"},
		{
			name:     "banned wording",
			extra:    "var label = \"百万格决斗\"\n",
			wantCopy: `copy violation at game.go:3 [禁用语「百万格决斗」] "百万格决斗"`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			src := "package main\nvar colour = \"#FFF\"\n" + tc.extra
			if err := os.WriteFile(filepath.Join(dir, "game.go"), []byte(src), 0o600); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command(binary, "-test.run=^TestSourceScanReport$", "-test.v")
			cmd.Dir = dir
			out, err := cmd.CombinedOutput()
			t.Logf("\n%s", out)
			if !strings.Contains(string(out), "调色板之外的裸色值（1 条）") {
				t.Error("expected one raw colour finding")
			}
			if tc.wantCopy == "" {
				if err != nil {
					t.Fatalf("raw colour alone should pass the gate: %v", err)
				}
				return
			}
			if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 1 {
				t.Fatalf("expected the copy gate to exit with status 1, got %v", err)
			}
			if !strings.Contains(string(out), tc.wantCopy) {
				t.Errorf("missing copy diagnostic %q", tc.wantCopy)
			}
		})
	}
}

// TestSourceScanWellFormed checks the scan of the real tree stays wired:
// every finding cites a shipped source file, a line and the offending
// literal. Detection itself is covered by scanreport's fixture test.
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
