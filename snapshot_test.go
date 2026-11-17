package main

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestSnapshot renders sample screens to PNG for design review and README
// assets. It only runs when MYGO_SNAPSHOT_DIR is set:
//
//	MYGO_SNAPSHOT_DIR=/tmp/shots go test -run TestSnapshot .
func TestSnapshot(t *testing.T) {
	dir := os.Getenv("MYGO_SNAPSHOT_DIR")
	if dir == "" {
		t.Skip("set MYGO_SNAPSHOT_DIR to render snapshots")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, dark := range []bool{false, true} {
		suffix := "day"
		if dark {
			suffix = "night"
		}

		// 16×16 band live, mid-game, hovering a number that can chord.
		app := newMinesweeperApp(20261116)
		app.darkMode = dark
		app.diffIdx = 1
		app.resetGame(true)
		playSampleGame(app)
		app.classicHoverRow, app.classicHoverCol = chordableNumber(app.game)
		writeSnapshot(t, app, filepath.Join(dir, "live-"+suffix+".png"))

		// 9×9 lost: knots, the hit knot, a wrong flag and the right ones.
		app = newMinesweeperApp(7)
		app.darkMode = dark
		app.diffIdx = 0
		app.resetGame(true)
		playLostGame(app)
		writeSnapshot(t, app, filepath.Join(dir, "lost-"+suffix+".png"))
	}
}

func writeSnapshot(t *testing.T, app *minesweeperApp, path string) {
	t.Helper()
	tt := ui.NewTester(app.view, 1280, 900)
	tt.SetScale(2)
	tt.Frame()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, tt.Image()); err != nil {
		t.Fatal(err)
	}
}

// playSampleGame opens a mid-game position: a first click plus a few safe
// openings and correct flags, deterministic for a fixed seed.
func playSampleGame(app *minesweeperApp) {
	g := app.game
	g.Reveal(g.Rows/2, g.Cols/3)
	for i := range g.Cells {
		c := &g.Cells[i]
		if !c.Revealed && !c.IsMine && c.Adjacent == 0 && i%7 == 0 {
			g.Reveal(c.Row, c.Col)
		}
	}
	flagged := 0
	for i := range g.Cells {
		c := &g.Cells[i]
		if c.IsMine && flagged < 6 && hasRevealedNeighbour(g, c.Row, c.Col) {
			g.ToggleFlag(c.Row, c.Col, false)
			flagged++
		}
	}
}

// playLostGame opens the board, plants one right and one wrong flag, then
// steps on a mine.
func playLostGame(app *minesweeperApp) {
	g := app.game
	g.Reveal(g.Rows/2, g.Cols/2)
	var right, wrong, hit = -1, -1, -1
	for i := range g.Cells {
		c := &g.Cells[i]
		switch {
		case c.Revealed:
		case c.IsMine && right < 0 && hasRevealedNeighbour(g, c.Row, c.Col):
			right = i
		case !c.IsMine && wrong < 0:
			wrong = i
		case c.IsMine && hit < 0 && i != right:
			hit = i
		}
	}
	for _, i := range []int{right, wrong} {
		if i >= 0 {
			g.ToggleFlag(g.Cells[i].Row, g.Cells[i].Col, false)
		}
	}
	if hit >= 0 {
		g.Reveal(g.Cells[hit].Row, g.Cells[hit].Col)
	}
}

func chordableNumber(g *Game) (int, int) {
	for i := range g.Cells {
		c := &g.Cells[i]
		if c.Revealed && c.Adjacent > 0 && g.InspectNumberHover(c.Row, c.Col, false).Kind == HintChordable {
			return c.Row, c.Col
		}
	}
	return -1, -1
}

func hasRevealedNeighbour(g *Game, row, col int) bool {
	for _, n := range g.Neighbors(row, col) {
		if n.Revealed {
			return true
		}
	}
	return false
}
