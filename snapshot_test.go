package main

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// snapshotViewports are the window sizes every scene is shot at: the
// default window plus one smaller and one larger.
var snapshotViewports = []struct {
	name          string
	width, height int
}{
	{"small", 960, 640},
	{"default", 1280, 900},
	{"large", 1680, 1050},
}

type snapshotScene struct {
	name  string
	stage func(dark bool) *minesweeperApp
}

var snapshotScenes = []snapshotScene{
	{"live", stageLiveScene},
	{"won", stageWonScene},
	{"lost", stageLostScene},
	{"duel", stageDuelScene},
}

// TestSnapshot renders sample screens to PNG for design review and README
// assets. It only runs when MYGO_SNAPSHOT_DIR is set:
//
//	MYGO_SNAPSHOT_DIR=/tmp/shots go test -run TestSnapshot .
//
// Every shot is named <scene>-<viewport>-<day|night>.png, so running the
// same command on another checkout (e.g. the default branch) into a second
// directory yields the matching "before" set for comparison.
func TestSnapshot(t *testing.T) {
	dir := os.Getenv("MYGO_SNAPSHOT_DIR")
	if dir == "" {
		t.Skip("set MYGO_SNAPSHOT_DIR to render snapshots")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, scene := range snapshotScenes {
		for _, dark := range []bool{false, true} {
			suffix := "day"
			if dark {
				suffix = "night"
			}
			for _, vp := range snapshotViewports {
				app := scene.stage(dark)
				name := fmt.Sprintf("%s-%s-%s.png", scene.name, vp.name, suffix)
				writeSnapshot(t, app, vp.width, vp.height, filepath.Join(dir, name))
			}
		}
	}
}

// pinStartQuote replaces the randomly drawn start quote with the pool's
// first entry — in the banner and its chat bubble — so shots do not drift
// between runs.
func pinStartQuote(app *minesweeperApp) {
	app.quote = memberQuotes[app.memberID]["start"][0]
	app.chatLog[len(app.chatLog)-1].Text = app.quote
}

// stageLiveScene stages a live, mid-game 16×16 band with a hover on a
// number that can chord.
func stageLiveScene(dark bool) *minesweeperApp {
	app := newMinesweeperApp(20261116)
	app.darkMode = dark
	app.diffIdx = 1
	app.resetGame(true)
	pinStartQuote(app)
	playSampleGame(app)
	app.classicHoverRow, app.classicHoverCol = chordableNumber(app.game)
	return app
}

// stageWonScene stages a cleared 9×9 board at a fixed 23-second clock,
// with every mine flagged by the engine.
func stageWonScene(dark bool) *minesweeperApp {
	app := newMinesweeperApp(7)
	app.darkMode = dark
	app.diffIdx = 0
	app.resetGame(true)
	pinStartQuote(app)
	playWonGame(app)
	app.elapsed = 23
	app.bestTimes[app.game.Difficulty.ID] = 23
	return app
}

// stageLostScene stages a lost 9×9 board: knots, the hit knot, one wrong
// flag, and the correct flags.
func stageLostScene(dark bool) *minesweeperApp {
	app := newMinesweeperApp(7)
	app.darkMode = dark
	app.diffIdx = 0
	app.resetGame(true)
	pinStartQuote(app)
	playLostGame(app)
	return app
}

// Duel snapshots are framed on the skirmish staged around this cell.
const duelStageRow, duelStageCol = 500, 498

// stageDuelScene stages the million-cell duel mid-fight, with player and
// AI territory, each side's flags, scores, and the minimap.
func stageDuelScene(dark bool) *minesweeperApp {
	app := newMinesweeperApp(9)
	app.darkMode = dark
	app.diffIdx = 4
	app.resetGame(true)
	pinStartQuote(app)
	playDuelGame(app)
	return app
}

// writeSnapshot renders app at width×height into path, pinning
// time-dependent state so the PNG is reproducible.
func writeSnapshot(t *testing.T, app *minesweeperApp, width, height int, path string) {
	t.Helper()
	// Saved before NewTester: its first frame clears a hover the pointer
	// is not on.
	hoverRow, hoverCol := app.classicHoverRow, app.classicHoverCol
	tt := ui.NewTester(app.view, width, height)
	tt.SetScale(2)
	if app.game.duel != nil {
		// Park the AI and land the minimap pulse on a phase boundary, then
		// re-pin the duel clock so the shot is byte-stable across runs.
		app.game.duel.NextAI = 1e9
		tt.Advance(time.Duration(1200-tt.Now().UnixMilli()%1200) * time.Millisecond)
		app.game.duel.Elapsed = 137
		app.elapsed = 137
		app.lastTick = tt.Now()
		// The board's real size only exists after a frame's draw.
		app.board.center(app.game, duelStageRow, duelStageCol)
	} else if hoverRow >= 0 {
		// A staged hover only survives while the pointer is really on
		// that cell; the hint needs a refresh frame plus its 1ms timer.
		if r, ok := tt.Find(fmt.Sprintf("cell-%d-%d", hoverRow, hoverCol)); ok {
			tt.Move(r.X+r.W/2, r.Y+r.H/2)
			tt.Advance(20 * time.Millisecond)
		}
	}
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

// playWonGame reveals every safe cell; the engine flags the mines itself.
func playWonGame(app *minesweeperApp) {
	g := app.game
	g.Reveal(g.Rows/2, g.Cols/2)
	for i := range g.Cells {
		c := &g.Cells[i]
		if !c.IsMine && !c.Revealed {
			g.Reveal(c.Row, c.Col)
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

// playDuelGame opens a mid-duel skirmish around the stage cell: player
// territory with flags on real mines, an AI pocket with a flag of its own,
// and both minimap markers. Deterministic for the app's seed.
func playDuelGame(app *minesweeperApp) {
	g := app.game
	g.Reveal(duelStageRow, duelStageCol)
	playerOpened, aiOpened, playerFlagged, aiFlagged := 0, 0, 0, 0
	for row := duelStageRow - 6; row <= duelStageRow+6; row++ {
		for col := duelStageCol - 8; col <= duelStageCol+12; col++ {
			c := g.Cell(row, col)
			if c == nil || c.Revealed || c.Flagged {
				continue
			}
			// The AI pocket stays within sight of the stage cell on every
			// viewport; the four-column gap reads as contested ground.
			nearStage := row >= duelStageRow-3 && row <= duelStageRow+3
			switch {
			case !c.IsMine && col < duelStageCol && playerOpened < 5 && hasRevealedNeighbour(g, row, col):
				g.Reveal(row, col)
				playerOpened++
			case !c.IsMine && col >= duelStageCol+4 && nearStage && aiOpened < 5:
				g.revealDuel(row, col, DuelAI)
				g.duel.LastAI = row*g.Cols + col
				aiOpened++
			case c.IsMine && col < duelStageCol && playerFlagged < 2 && hasRevealedNeighbour(g, row, col):
				g.ToggleFlag(row, col, false)
				playerFlagged++
			case c.IsMine && col >= duelStageCol+4 && nearStage && aiFlagged < 1 && hasRevealedNeighbour(g, row, col):
				g.flagDuel(row, col, DuelAI)
				aiFlagged++
			}
		}
	}
	// Mid-duel clock, far enough in that both sides have scores.
	g.duel.Elapsed = 137
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
