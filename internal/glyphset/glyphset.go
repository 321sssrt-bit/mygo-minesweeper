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

// Literal is one string or rune literal in a non-test Go file, positioned so
// reports can cite file and line.
type Literal struct {
	Path   string
	Line   int
	Text   string
	IsChar bool
}

// Literals returns every string and rune literal in the non-test Go files
// directly inside dir, in file-name order and then source order. Import
// paths are skipped: they are build detail, never player-facing copy.
func Literals(dir string) ([]Literal, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		return nil, err
	}
	fset := token.NewFileSet()
	var out []Literal
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
			if _, ok := n.(*ast.ImportSpec); ok {
				return false
			}
			lit, ok := n.(*ast.BasicLit)
			if !ok {
				return true
			}
			pos := fset.Position(lit.Pos())
			switch lit.Kind {
			case token.STRING:
				if s, err := strconv.Unquote(lit.Value); err == nil {
					out = append(out, Literal{Path: path, Line: pos.Line, Text: s})
				}
			case token.CHAR:
				if r, _, _, err := strconv.UnquoteChar(strings.Trim(lit.Value, "'"), '\''); err == nil {
					out = append(out, Literal{Path: path, Line: pos.Line, Text: string(r), IsChar: true})
				}
			}
			return true
		})
	}
	return out, nil
}

// Collect returns the sorted, de-duplicated characters of every string and
// rune literal in the non-test Go files directly inside dir, plus printable
// ASCII (numbers and formatted values are built at run time).
func Collect(dir string) ([]rune, error) {
	seen := map[rune]bool{}
	for r := rune(0x20); r <= 0x7E; r++ {
		seen[r] = true
	}
	lits, err := Literals(dir)
	if err != nil {
		return nil, err
	}
	for _, lit := range lits {
		for _, r := range lit.Text {
			if r >= 0x20 {
				seen[r] = true
			}
		}
	}
	out := make([]rune, 0, len(seen))
	for r := range seen {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out, nil
}
