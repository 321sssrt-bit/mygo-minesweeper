// Command scanreport prints the copy and colour violations found in source
// literals — the checklist for the copy ticket (issue #5). Report only;
// always exits 0 unless the scan itself fails. Run from the repository root:
//
//	go run ./tools/scanreport
package main

import (
	"fmt"
	"log"

	"mygo-minesweeper/internal/scanreport"
)

func main() {
	rep, err := scanreport.Scan(".")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Print(rep)
}
