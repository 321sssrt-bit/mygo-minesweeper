// Package glyphset collects every character the game can draw from its own
// source, so the embedded font can be subset to exactly those characters
// and a test can fail when copy gains a character the subset lacks.
package glyphset

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// Collect returns the sorted, de-duplicated characters of every string and
// rune literal in the non-test Go files directly inside dir, plus printable
// ASCII (numbers and formatted values are built at run time).
func Collect(dir string) ([]rune, error) {
	seen := map[rune]bool{}
	for r := rune(0x20); r <= 0x7E; r++ {
		seen[r] = true
	}
	files, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		return nil, err
	}
	fset := token.NewFileSet()
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		f, err := parser.ParseFile(fset, path, src, 0)
		if err != nil {
			return nil, err
		}
		ast.Inspect(f, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok {
				return true
			}
			switch lit.Kind {
			case token.STRING:
				if s, err := strconv.Unquote(lit.Value); err == nil {
					for _, r := range s {
						if r >= 0x20 {
							seen[r] = true
						}
					}
				}
			case token.CHAR:
				if r, _, _, err := strconv.UnquoteChar(strings.Trim(lit.Value, "'"), '\''); err == nil && r >= 0x20 {
					seen[r] = true
				}
			}
			return true
		})
	}
	out := make([]rune, 0, len(seen))
	for r := range seen {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out, nil
}
