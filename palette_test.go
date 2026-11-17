package main

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestInksReadOnBothPapers(t *testing.T) {
	for _, p := range []*palette{dayPaper, nightPaper} {
		for n := 1; n <= 8; n++ {
			ink := numberInk(n, p.Dark)
			for name, bg := range map[string]ui.Color{"board": p.Board, "sheet": p.Sheet} {
				if got := contrast(ink, bg); got < minInkContrast {
					t.Errorf("dark=%v number %d ink %s on %s: contrast %.2f < %.1f", p.Dark, n, hexOf(ink), name, got, minInkContrast)
				}
			}
		}
		for _, m := range Members {
			if got := contrast(p.InkOf(memberImage(m.ID)), p.Sheet); got < minInkContrast {
				t.Errorf("dark=%v %s ink contrast %.2f on sheet", p.Dark, m.Name, got)
			}
		}
	}
}

func TestMemberColoursAreOfficialImageColours(t *testing.T) {
	want := map[MemberID]string{
		MemberTomori: "#77BBDD", MemberAnon: "#FF8899", MemberRana: "#77DD77",
		MemberSoyorin: "#FFDD88", MemberTaki: "#7777AA",
	}
	for id, hex := range want {
		if got := FindMember(id).ColorHex; got != hex {
			t.Errorf("%s colour %s, want official 代表色 %s", id, got, hex)
		}
	}
	if hexOf(bandColor) != "#3388BB" {
		t.Errorf("band colour %s, want official 乐队色 #3388BB", hexOf(bandColor))
	}
}
