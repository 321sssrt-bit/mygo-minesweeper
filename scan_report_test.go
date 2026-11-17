package main

import (
	"strings"
	"testing"

	"mygo-minesweeper/internal/scanreport"
)

// TestSourceScanReport logs the copy/colour violation report for review
// (`go test -v -run TestSourceScanReport`). Findings never fail the gate;
// the report is the checklist the copy ticket works from (issue #5).
func TestSourceScanReport(t *testing.T) {
	rep, err := scanreport.Scan(".")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("\n%s", rep)
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
