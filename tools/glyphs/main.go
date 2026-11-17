// Command glyphs prints every character the game's source can draw, as the
// text file pyftsubset takes. Run from the repository root:
//
//	go run ./tools/glyphs > glyphs.txt
package main

import (
	"fmt"
	"log"

	"mygo-minesweeper/internal/glyphset"
)

func main() {
	runes, err := glyphset.Collect(".")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Print(string(runes))
}
