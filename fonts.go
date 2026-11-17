package main

import (
	_ "embed"

	"github.com/egoist/mygo/ui"
)

//go:generate sh scripts/subset-font.sh

// fontFamily is LXGW WenKai (SIL OFL 1.1, resources/fonts/OFL.txt), subset to
// the characters the source uses; see scripts/subset-font.sh.
const fontFamily = "LXGW WenKai"

var (
	//go:embed resources/fonts/LXGWWenKai-Regular.subset.ttf
	wenkaiRegular []byte
	//go:embed resources/fonts/LXGWWenKai-Medium.subset.ttf
	wenkaiMedium []byte
)

func init() {
	for _, data := range [][]byte{wenkaiRegular, wenkaiMedium} {
		if err := ui.RegisterFont(data, fontFamily); err != nil {
			panic(err)
		}
	}
}
