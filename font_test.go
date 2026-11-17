package main

import (
	"bytes"
	"testing"
	"unicode"

	"github.com/go-text/typesetting/font"

	"mygo-minesweeper/internal/glyphset"
)

// fontFallbackOK lists characters the subset need not carry: emoji the
// system emoji font draws.
func fontFallbackOK(r rune) bool {
	return r >= 0x1F000 || unicode.Is(unicode.Variation_Selector, r)
}

func TestEmbeddedFontCoversCopy(t *testing.T) {
	runes, err := glyphset.Collect(".")
	if err != nil {
		t.Fatal(err)
	}
	for name, data := range map[string][]byte{"Regular": wenkaiRegular, "Medium": wenkaiMedium} {
		face, err := font.ParseTTF(bytes.NewReader(data))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		var missing []rune
		for _, r := range runes {
			if _, ok := face.NominalGlyph(r); !ok && !fontFallbackOK(r) {
				missing = append(missing, r)
			}
		}
		if len(missing) > 0 {
			t.Errorf("LXGW WenKai %s subset lacks %q; run `go generate` (scripts/subset-font.sh) after changing copy", name, string(missing))
		}
	}
}
