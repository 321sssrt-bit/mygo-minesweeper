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
	name   string
	stage  func(dark bool) *minesweeperApp
	canvas bool
}

var snapshotScenes = []snapshotScene{
	{name: "live", stage: stageLiveScene},
	{name: "won", stage: stageWonScene},
	{name: "lost", stage: stageLostScene},
	{name: "duel", stage: stageDuelScene},
	{name: "duel-open", stage: stageDuelOpenScene},
	{name: "senzaihyoumei", stage: stageSenzaihyoumeiScene},
	{name: "setlist-shipped", stage: stageShippedSetlistScene},
	{name: "settings-shipped", stage: stageShippedSettingsScene},
	{name: "classic-canvas-open", stage: stageClassicCanvasOpenScene},
	{name: "classic-canvas-live", stage: stageClassicCanvasLiveScene, canvas: true},
	{name: "classic-canvas-won", stage: stageClassicCanvasWonScene, canvas: true},
	{name: "classic-canvas-lost", stage: stageClassicCanvasLostScene, canvas: true},
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
				writeSnapshot(t, app, scene.canvas, vp.width, vp.height, filepath.Join(dir, name))
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

func findDifficultyIdx(id string) int {
	for i, d := range Difficulties {
		if d.ID == id {
			return i
		}
	}
	panic("difficulty not found: " + id)
}

// stageDuelScene stages the million-cell duel mid-fight, with player and
// AI territory, each side's flags, scores, and the minimap.
func stageDuelScene(dark bool) *minesweeperApp {
	app := newMinesweeperApp(9)
	app.darkMode = dark
	app.diffIdx = findDifficultyIdx("duel")
	app.resetGame(true)
	pinStartQuote(app)
	playDuelGame(app, duelStageRow, duelStageCol, 137)
	return app
}

// stageDuelOpenScene stages the opening state of the million-cell duel so the
// snapshot captures the initial standard-pitch centered canvas and 100% readout.
func stageDuelOpenScene(dark bool) *minesweeperApp {
	app := newMinesweeperApp(9)
	app.darkMode = dark
	app.diffIdx = findDifficultyIdx("duel")
	app.resetGame(true)
	pinStartQuote(app)
	return app
}

// stageClassicCanvasOpenScene stages the opening state of the 50×50 classic
// canvas track through the full app shell so the snapshot captures the initial
// standard-pitch centered canvas and 100% readout.
func stageClassicCanvasOpenScene(dark bool) *minesweeperApp {
	app := newMinesweeperApp(20261116)
	app.darkMode = dark
	app.diffIdx = findDifficultyIdx(TrackUtakotoba.ID)
	app.resetGame(true)
	pinStartQuote(app)
	return app
}

// stageSenzaihyoumeiScene stages the 500×500 《潜在表明》 duel mid-fight,
// centered on its skirmish with player and AI territory, flags, and minimap.
func stageSenzaihyoumeiScene(dark bool) *minesweeperApp {
	app := newMinesweeperApp(9)
	app.darkMode = dark
	app.diffIdx = findDifficultyIdx("senzaihyoumei")
	app.resetGame(true)
	pinStartQuote(app)
	playDuelGame(app, app.game.Rows/2, app.game.Cols/2-2, 83)
	return app
}

// stageShippedSetlistScene stages the live scene with the shipped setlist
// popover expanded, showing the real Difficulties groups and duel entries.
// The two smallest classic tracks carry fixed records so the shot shows both
// a recorded entry and the placeholder on unplayed tracks.
func stageShippedSetlistScene(dark bool) *minesweeperApp {
	app := stageLiveScene(dark)
	app.classicHoverRow, app.classicHoverCol = -1, -1
	app.bestTimes["easy"] = 23
	app.bestTimes["normal"] = 87
	app.setlistOpen = true
	return app
}

// stageShippedSettingsScene stages the live scene with the shipped settings
// popover expanded, showing all three preference groups and controls.
func stageShippedSettingsScene(dark bool) *minesweeperApp {
	app := stageLiveScene(dark)
	app.classicHoverRow, app.classicHoverCol = -1, -1
	app.openPopover(popoverSettings)
	return app
}

// Classic canvas snapshots use the 50×50 TrackUtakotoba board framed around
// its center so every viewport (small, default, large) is filled with paper
// cells just like a real large classic track.
const classicCanvasStageRow, classicCanvasStageCol = 25, 25

// stageClassicCanvasLiveScene stages a mid-game 50×50 classic board for the
// paper canvas with slips, numbers, flags, a Soyorin shield flag, ? marks,
// and a chordable number under hover near the viewport center.
func stageClassicCanvasLiveScene(dark bool) *minesweeperApp {
	app := newMinesweeperApp(20261116)
	app.darkMode = dark
	app.memberID = MemberSoyorin
	app.allowQuestion = true
	app.game = NewGame(TrackUtakotoba, true, MemberSoyorin, 20261116)
	pinStartQuote(app)
	g := app.game
	g.Reveal(classicCanvasStageRow, classicCanvasStageCol)
	for row := classicCanvasStageRow - 6; row <= classicCanvasStageRow+6; row++ {
		for col := classicCanvasStageCol - 8; col <= classicCanvasStageCol+8; col++ {
			c := g.Cell(row, col)
			if c != nil && !c.Revealed && !c.IsMine && c.Adjacent == 0 && (row*g.Cols+col)%5 == 0 {
				g.Reveal(row, col)
			}
		}
	}
	flagged := 0
	shielded := false
	for row := classicCanvasStageRow - 6; row <= classicCanvasStageRow+6; row++ {
		for col := classicCanvasStageCol - 8; col <= classicCanvasStageCol+8; col++ {
			c := g.Cell(row, col)
			if c == nil || !c.IsMine || c.Revealed || c.Flagged || !hasRevealedNeighbour(g, row, col) {
				continue
			}
			if flagged < 6 {
				g.ToggleFlag(row, col, false)
				flagged++
			} else if !shielded {
				g.Reveal(row, col)
				shielded = true
			}
		}
	}
	hoverRow, hoverCol := -1, -1
	for row := classicCanvasStageRow - 5; row <= classicCanvasStageRow+5 && hoverRow < 0; row++ {
		for col := classicCanvasStageCol - 7; col <= classicCanvasStageCol+7; col++ {
			c := g.Cell(row, col)
			if c != nil && c.Revealed && c.Adjacent > 0 && g.InspectNumberHover(row, col, false).Kind == HintChordable {
				hoverRow, hoverCol = row, col
				break
			}
		}
	}
	if hoverRow < 0 {
		hoverRow, hoverCol = chordableNumber(g)
	}
	app.classicHoverRow, app.classicHoverCol = hoverRow, hoverCol
	hintTargets := g.InspectNumberHover(hoverRow, hoverCol, false).Targets
	questioned := 0
	for row := classicCanvasStageRow - 5; row <= classicCanvasStageRow+5 && questioned < 2; row++ {
		for col := classicCanvasStageCol - 7; col <= classicCanvasStageCol+7 && questioned < 2; col++ {
			c := g.Cell(row, col)
			idx := row*g.Cols + col
			if c != nil && !c.IsMine && !c.Revealed && !c.Flagged && !c.Questioned &&
				hasRevealedNeighbour(g, row, col) && !containsIndex(hintTargets, idx) {
				g.ToggleFlag(row, col, true)
				g.ToggleFlag(row, col, true)
				questioned++
			}
		}
	}
	return app
}

// stageClassicCanvasWonScene stages a cleared 50×50 classic board for the
// paper canvas with every mine flagged by the engine.
func stageClassicCanvasWonScene(dark bool) *minesweeperApp {
	app := newMinesweeperApp(7)
	app.darkMode = dark
	app.game = NewGame(TrackUtakotoba, true, MemberTomori, 7)
	pinStartQuote(app)
	playWonGame(app)
	app.elapsed = 23
	app.bestTimes[app.game.Difficulty.ID] = 23
	return app
}

// stageClassicCanvasLostScene stages a lost 50×50 classic board for the paper
// canvas covering every classic cell state near the viewport center: paper
// slips, numbers, band-aid flag, Soyorin shield band-aid, wrong flag, ? mark,
// knots, and hit knot.
func stageClassicCanvasLostScene(dark bool) *minesweeperApp {
	app := newMinesweeperApp(7)
	app.darkMode = dark
	app.memberID = MemberSoyorin
	app.allowQuestion = true
	diff := TrackUtakotoba
	diff.SkillCharges = 1
	app.game = NewGame(diff, true, MemberSoyorin, 7)
	pinStartQuote(app)
	g := app.game
	g.Reveal(classicCanvasStageRow, classicCanvasStageCol)
	var right, shield, wrong, question, hit = -1, -1, -1, -1, -1
	for row := classicCanvasStageRow - 5; row <= classicCanvasStageRow+5; row++ {
		for col := classicCanvasStageCol - 7; col <= classicCanvasStageCol+7; col++ {
			c := g.Cell(row, col)
			if c == nil || c.Revealed {
				continue
			}
			i := row*g.Cols + col
			nearOpened := hasRevealedNeighbour(g, row, col)
			switch {
			case c.IsMine && right < 0 && nearOpened:
				right = i
			case c.IsMine && shield < 0 && i != right && nearOpened:
				shield = i
			case !c.IsMine && wrong < 0 && nearOpened:
				wrong = i
			case !c.IsMine && question < 0 && i != wrong && nearOpened:
				question = i
			case c.IsMine && hit < 0 && i != right && i != shield && nearOpened:
				hit = i
			}
		}
	}
	if right >= 0 {
		g.ToggleFlag(g.Cells[right].Row, g.Cells[right].Col, false)
	}
	if wrong >= 0 {
		g.ToggleFlag(g.Cells[wrong].Row, g.Cells[wrong].Col, false)
	}
	if question >= 0 {
		g.ToggleFlag(g.Cells[question].Row, g.Cells[question].Col, true)
		g.ToggleFlag(g.Cells[question].Row, g.Cells[question].Col, true)
	}
	if shield >= 0 {
		g.Reveal(g.Cells[shield].Row, g.Cells[shield].Col)
	}
	if hit >= 0 {
		g.Reveal(g.Cells[hit].Row, g.Cells[hit].Col)
	}
	return app
}

// writeSnapshot renders app at width×height into path, pinning
// time-dependent state so the PNG is reproducible.
func writeSnapshot(t *testing.T, app *minesweeperApp, canvas bool, width, height int, path string) {
	t.Helper()
	// Saved before NewTester: its first frame clears a hover the pointer
	// is not on.
	hoverRow, hoverCol := app.classicHoverRow, app.classicHoverCol
	viewFn := app.view
	if canvas {
		viewFn = classicCanvasTestView(app)
	}
	tt := ui.NewTester(viewFn, width, height)
	tt.SetScale(2)
	if app.game.duel != nil {
		// Park the AI and land the minimap pulse on a phase boundary, then
		// re-pin the duel clock so the shot is byte-stable across runs.
		elapsed := app.game.duel.Elapsed
		app.game.duel.NextAI = 1e9
		tt.Advance(time.Duration(1200-tt.Now().UnixMilli()%1200) * time.Millisecond)
		app.game.duel.Elapsed = elapsed
		app.elapsed = int(elapsed)
		app.lastTick = tt.Now()
		if app.game.Status != StatusIdle {
			// The board's real size only exists after a frame's draw.
			app.board.center(app.game, app.game.Rows/2, app.game.Cols/2-2)
		}
	} else if canvas {
		// The board's real size only exists after a frame's draw.
		tt.Frame()
		app.board.center(app.game, classicCanvasStageRow, classicCanvasStageCol)
		tt.Frame()
		if hoverRow >= 0 {
			if r, ok := tt.Find("经典棋盘"); ok {
				v := app.board
				x := r.X + (float32(hoverCol)+0.5)*v.Cell - v.X
				y := r.Y + (float32(hoverRow)+0.5)*v.Cell - v.Y
				tt.Move(x, y)
			}
		}
	} else if hoverRow >= 0 {
		if r, ok := tt.Find(fmt.Sprintf("cell-%d-%d", hoverRow, hoverCol)); ok {
			// A staged hover only survives while the pointer is really on
			// that cell; the hint needs a refresh frame plus its 1ms timer.
			tt.Move(r.X+r.W/2, r.Y+r.H/2)
			tt.Advance(20 * time.Millisecond)
		} else if r, ok := tt.Find("经典棋盘"); ok {
			tt.Frame()
			v := app.board
			if !v.shows(app.game, hoverRow, hoverCol) {
				v.center(app.game, hoverRow, hoverCol)
				tt.Frame()
			}
			x := r.X + (float32(hoverCol)+0.5)*v.Cell - v.X
			y := r.Y + (float32(hoverRow)+0.5)*v.Cell - v.Y
			tt.Move(x, y)
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

// playDuelGame opens a mid-duel skirmish around (stageRow, stageCol): player
// territory with flags on real mines, an AI pocket with a flag of its own,
// and both minimap markers. Deterministic for the app's seed.
func playDuelGame(app *minesweeperApp, stageRow, stageCol int, elapsed float64) {
	g := app.game
	g.Reveal(stageRow, stageCol)
	playerOpened, aiOpened, playerFlagged, aiFlagged := 0, 0, 0, 0
	for row := stageRow - 6; row <= stageRow+6; row++ {
		for col := stageCol - 8; col <= stageCol+12; col++ {
			c := g.Cell(row, col)
			if c == nil || c.Revealed || c.Flagged {
				continue
			}
			// The AI pocket stays within sight of the stage cell on every
			// viewport; the four-column gap reads as contested ground.
			nearStage := row >= stageRow-3 && row <= stageRow+3
			switch {
			case !c.IsMine && col < stageCol && playerOpened < 5 && hasRevealedNeighbour(g, row, col):
				g.Reveal(row, col)
				playerOpened++
			case !c.IsMine && col >= stageCol+4 && nearStage && aiOpened < 5:
				g.revealDuel(row, col, DuelAI)
				g.duel.LastAI = row*g.Cols + col
				aiOpened++
			case c.IsMine && col < stageCol && playerFlagged < 2 && hasRevealedNeighbour(g, row, col):
				g.ToggleFlag(row, col, false)
				playerFlagged++
			case c.IsMine && col >= stageCol+4 && nearStage && aiFlagged < 1 && hasRevealedNeighbour(g, row, col):
				g.flagDuel(row, col, DuelAI)
				aiFlagged++
			}
		}
	}
	// Mid-duel clock, far enough in that both sides have scores.
	g.duel.Elapsed = elapsed
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
