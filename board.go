package main

import (
	"fmt"
	"time"

	"github.com/egoist/mygo/ui"
)

type boardView struct {
	X, Y           float32
	Cell           float32
	Width, Height  float32
	HoverRow       int
	HoverCol       int
	PressX, PressY float32
	DragX, DragY   float32
	Dragging       bool
	Pressed        bool
	PressButton    int
	LastEffect     time.Time
	EffectCell     int
}

func newBoardView() *boardView {
	return &boardView{Cell: 26, X: 487 * 26, Y: 487 * 26, HoverRow: -1, HoverCol: -1, EffectCell: -1}
}

func (v *boardView) clamp(g *Game) {
	v.Cell = max(10, min(42, v.Cell))
	v.Width, v.Height = max(100, v.Width), max(100, v.Height)
	v.X = max(0, min(float32(g.Cols)*v.Cell-v.Width, v.X))
	v.Y = max(0, min(float32(g.Rows)*v.Cell-v.Height, v.Y))
}

func (v *boardView) center(g *Game, row, col int) {
	v.X = (float32(col)+0.5)*v.Cell - v.Width/2
	v.Y = (float32(row)+0.5)*v.Cell - v.Height/2
	v.clamp(g)
}

func (v *boardView) zoom(g *Game, atX, atY, factor float32) {
	old := v.Cell
	next := max(10, min(42, old*factor))
	worldX := (v.X + atX) / old
	worldY := (v.Y + atY) / old
	v.Cell = next
	v.X = worldX*next - atX
	v.Y = worldY*next - atY
	v.clamp(g)
}

func (v *boardView) cellAt(g *Game, x, y float32) (int, int, bool) {
	col, row := int((v.X+x)/v.Cell), int((v.Y+y)/v.Cell)
	return row, col, row >= 0 && row < g.Rows && col >= 0 && col < g.Cols
}

func (v *boardView) minimap(x, y float32) bool {
	return x >= v.Width-158 && y >= v.Height-158 && x <= v.Width-10 && y <= v.Height-10
}

func (a *minesweeperApp) duelBoard(c *ui.Context) {
	v := a.board
	v.clamp(a.game)
	ui.Box(c.Key("duel-board")).Grow(1).FillWidth().Label("百万格决斗战场").
		Cursor(ui.CursorPointer).Focusable().HandleInput(func(ev ui.InputEvent) bool {
		if ev.Kind == ui.InputPointerMove {
			v.HoverRow, v.HoverCol, _ = v.cellAt(a.game, ev.X, ev.Y)
		}
		switch ev.Kind {
		case ui.InputScroll:
			v.zoom(a.game, ev.X, ev.Y, 1+max(-0.35, min(0.35, -ev.DY/400)))
			return true
		case ui.InputZoom:
			v.zoom(a.game, ev.X, ev.Y, ev.Scale)
			return true
		case ui.InputPointerDown:
			if ev.Button > 2 {
				return false
			}
			if ev.Button == 0 && v.minimap(ev.X, ev.Y) {
				v.center(a.game, int((ev.Y-(v.Height-158))/148*float32(a.game.Rows)), int((ev.X-(v.Width-158))/148*float32(a.game.Cols)))
				return true
			}
			v.Pressed = true
			v.PressButton = ev.Button
			v.PressX, v.PressY = ev.X, ev.Y
			v.DragX, v.DragY = ev.X, ev.Y
			v.Dragging = ev.Button == 2
			return true
		case ui.InputPointerMove:
			if !v.Pressed || ev.Button < 0 {
				return false
			}
			if abs32(ev.X-v.PressX)+abs32(ev.Y-v.PressY) > 5 {
				v.Dragging = true
			}
			if v.Dragging {
				v.X -= ev.X - v.DragX
				v.Y -= ev.Y - v.DragY
				v.DragX, v.DragY = ev.X, ev.Y
				v.clamp(a.game)
			}
			return true
		case ui.InputPointerUp:
			if !v.Pressed {
				return false
			}
			button, dragged := v.PressButton, v.Dragging
			v.Pressed, v.Dragging = false, false
			if ev.Canceled || dragged {
				return true
			}
			row, col, ok := v.cellAt(a.game, ev.X, ev.Y)
			if !ok {
				return true
			}
			var result ActionResult
			if button == 1 || (button == 0 && a.flagMode) {
				result = a.game.ToggleFlag(row, col, false)
				if result.Kind == ActionNoop {
					result = a.game.UnflagDuel(row, col)
				}
			} else if button == 0 {
				cell := a.game.Cell(row, col)
				if cell.Revealed && !cell.IsMine {
					result = a.game.ClaimDuelMines(row, col)
					if result.Kind == ActionNoop {
						result = a.game.Chord(row, col)
					}
				} else {
					result = a.game.Reveal(row, col)
				}
			}
			if result.Kind != ActionNoop {
				a.pendingResult = result
				v.LastEffect = c.Now()
				v.EffectCell = row*a.game.Cols + col
			}
			return true
		}
		return false
	}).Draw(func(p *ui.Painter, r ui.Rect) {
		v.Width, v.Height = r.W, r.H
		v.clamp(a.game)
		a.paintDuelBoard(p, r)
	})
}

func (a *minesweeperApp) paintDuelBoard(p *ui.Painter, r ui.Rect) {
	v, g := a.board, a.game
	t := p.Theme()
	p.Fill(r, ui.Hex("#070D17"), 8)
	firstCol, lastCol := int(v.X/v.Cell), min(g.Cols, int((v.X+r.W)/v.Cell)+2)
	firstRow, lastRow := int(v.Y/v.Cell), min(g.Rows, int((v.Y+r.H)/v.Cell)+2)
	for row := max(0, firstRow); row < lastRow; row++ {
		for col := max(0, firstCol); col < lastCol; col++ {
			cell := g.Cell(row, col)
			x := r.X + float32(col)*v.Cell - v.X
			y := r.Y + float32(row)*v.Cell - v.Y
			tile := ui.Rect{X: x + 1, Y: y + 1, W: v.Cell - 2, H: v.Cell - 2}
			bg, border := t.Surface, t.Border.Alpha(0.45)
			if cell.Revealed {
				bg = t.Background
				border = t.Border.Alpha(0.25)
				if cell.Owner == DuelPlayer {
					bg = ui.Hex("#173B49")
					border = ui.Hex("#55B2D6").Alpha(0.55)
				} else if cell.Owner == DuelAI {
					bg = ui.Hex("#3B2740")
					border = ui.Hex("#F07AA4").Alpha(0.45)
				}
			}
			if row == v.HoverRow && col == v.HoverCol && !cell.Revealed {
				border = t.Accent
			}
			if cell.Exploded {
				bg, border = ui.Hex("#701F2B"), ui.Hex("#FB7185")
			}
			p.Fill(tile, bg, min(4, v.Cell/7))
			if v.Cell >= 15 {
				p.Stroke(tile, border, min(4, v.Cell/7), 1)
			}
			if v.Cell < 16 {
				continue
			}
			cx, cy := tile.X+tile.W/2, tile.Y+tile.H/2
			switch {
			case cell.Flagged:
				color := ui.Hex("#55B2D6")
				if cell.FlagOwner == DuelAI {
					color = ui.Hex("#F07AA4")
				}
				if cell.WrongFlag {
					color = ui.Hex("#FB7185")
				}
				p.Line(cx-3, cy-7, cx-3, cy+7, 2, color)
				p.Fill(ui.Rect{X: cx - 2, Y: cy - 7, W: 9, H: 7}, color, 1)
			case cell.Revealed && cell.IsMine:
				p.Line(cx-6, cy-6, cx+6, cy+6, 2.5, ui.Hex("#FB7185"))
				p.Line(cx+6, cy-6, cx-6, cy+6, 2.5, ui.Hex("#FB7185"))
			case cell.Revealed && cell.Adjacent > 0:
				p.Text(cx-float32(len(fmt.Sprint(cell.Adjacent)))*v.Cell*0.15, cy-v.Cell*0.28, fmt.Sprint(cell.Adjacent), v.Cell*0.55, numberColor(cell.Adjacent))
			}
		}
	}
	if count, ok := g.ScanAt(v.HoverRow, v.HoverCol); ok {
		x := r.X + float32(v.HoverCol-4)*v.Cell - v.X
		y := r.Y + float32(v.HoverRow-4)*v.Cell - v.Y
		p.Stroke(ui.Rect{X: x, Y: y, W: 9 * v.Cell, H: 9 * v.Cell}, ui.Hex("#E8A849"), 5, 2)
		p.Fill(ui.Rect{X: r.X + 12, Y: r.Y + 12, W: 166, H: 34}, ui.Hex("#3C301D"), 6)
		p.Text(r.X+23, r.Y+20, fmt.Sprintf("9×9 扫描：%d 雷", count), 14, ui.Hex("#F5D99C"))
	}
	if g.duel.LastAI >= 0 {
		row, col := g.duel.LastAI/g.Cols, g.duel.LastAI%g.Cols
		x := r.X + float32(col)*v.Cell - v.X
		y := r.Y + float32(row)*v.Cell - v.Y
		p.Stroke(ui.Rect{X: x, Y: y, W: v.Cell, H: v.Cell}, ui.Hex("#F07AA4"), 4, 2)
	}
	if v.EffectCell >= 0 {
		elapsed := p.Now().Sub(v.LastEffect)
		if elapsed >= 0 && elapsed < 350*time.Millisecond {
			row, col := v.EffectCell/g.Cols, v.EffectCell%g.Cols
			k := float32(elapsed) / float32(350*time.Millisecond)
			size := v.Cell * (1 + k*1.5)
			x := r.X + (float32(col)+0.5)*v.Cell - v.X - size/2
			y := r.Y + (float32(row)+0.5)*v.Cell - v.Y - size/2
			p.Stroke(ui.Rect{X: x, Y: y, W: size, H: size}, t.Accent.Alpha(1-k), 6, 2)
			p.AnimationFrame()
		}
	}
	mini := ui.Rect{X: r.X + r.W - 158, Y: r.Y + r.H - 158, W: 148, H: 148}
	p.Fill(mini, ui.Hex("#0B1423").Alpha(0.94), 6)
	if len(g.duel.Minimap) == minimapGridSize*minimapGridSize {
		bw := mini.W / float32(minimapGridSize)
		bh := mini.H / float32(minimapGridSize)
		fillW := max(1.5, bw)
		fillH := max(1.5, bh)
		for br := range minimapGridSize {
			for bc := range minimapGridSize {
				blk := g.duel.Minimap[br*minimapGridSize+bc]
				if blk.Player == 0 && blk.AI == 0 {
					continue
				}
				var color ui.Color
				switch {
				case blk.Player > 0 && blk.AI > 0:
					density := min(float32(1), float32(blk.Player+blk.AI)/24)
					color = ui.Hex("#A78BFA").Alpha(0.65 + 0.35*density)
				case blk.Player > 0:
					density := min(float32(1), float32(blk.Player)/20)
					color = ui.Hex("#55B2D6").Alpha(0.65 + 0.35*density)
				default:
					density := min(float32(1), float32(blk.AI)/20)
					color = ui.Hex("#F07AA4").Alpha(0.65 + 0.35*density)
				}
				bx := mini.X + float32(bc)*bw
				by := mini.Y + float32(br)*bh
				p.Fill(ui.Rect{X: bx, Y: by, W: min(fillW, mini.X+mini.W-bx), H: min(fillH, mini.Y+mini.H-by)}, color, 0)
			}
		}
	}
	pulsing := g.Status == StatusPlaying
	pulse := float32(0)
	if pulsing {
		ms := float32(p.Now().UnixMilli() % 1200)
		if ms < 0 {
			ms += 1200
		}
		pulse = ms / 1200.0
	}
	hasAI := g.duel.LastAI >= 0 && g.duel.LastAI < len(g.Cells)
	hasPlayer := g.duel.LastCell >= 0 && g.duel.LastCell < len(g.Cells)
	var ax, ay float32
	if hasAI {
		ax, ay = minimapMarkerPos(mini, g.Rows, g.Cols, g.duel.LastAI, 5)
		if pulsing {
			ring := clampMinimapRect(mini, ax-(3+pulse*4.5), ay-(3+pulse*4.5), (3+pulse*4.5)*2)
			p.Stroke(ring, ui.Hex("#F07AA4").Alpha(1-pulse*0.75), 99, 1.5)
		}
		p.Fill(ui.Rect{X: ax - 2.5, Y: ay - 2.5, W: 5, H: 5}, ui.Hex("#F07AA4"), 1)
		p.Stroke(ui.Rect{X: ax - 2.5, Y: ay - 2.5, W: 5, H: 5}, ui.Hex("#FDF2F8"), 1, 1)
	}
	if hasPlayer {
		px, py := minimapMarkerPos(mini, g.Rows, g.Cols, g.duel.LastCell, 5)
		if hasAI && abs32(px-ax) < 4 && abs32(py-ay) < 4 {
			if px+3 <= mini.X+mini.W-5 {
				px += 3
			} else {
				px -= 3
			}
			if py-3 >= mini.Y+5 {
				py -= 3
			} else {
				py += 3
			}
		}
		if pulsing {
			ring := clampMinimapRect(mini, px-(3+pulse*3.5), py-(3+pulse*3.5), (3+pulse*3.5)*2)
			p.Stroke(ring, ui.Hex("#38BDF8").Alpha(1-pulse*0.75), 99, 1.5)
		}
		p.Fill(ui.Rect{X: px - 2.5, Y: py - 2.5, W: 5, H: 5}, ui.Hex("#38BDF8"), 99)
		p.Stroke(ui.Rect{X: px - 2.5, Y: py - 2.5, W: 5, H: 5}, ui.Hex("#F4F8FD"), 99, 1)
	}
	if pulsing && (hasAI || hasPlayer) {
		p.AnimationFrame()
	}
	view := ui.Rect{X: mini.X + v.X/(float32(g.Cols)*v.Cell)*mini.W, Y: mini.Y + v.Y/(float32(g.Rows)*v.Cell)*mini.H, W: r.W / (float32(g.Cols) * v.Cell) * mini.W, H: r.H / (float32(g.Rows) * v.Cell) * mini.H}
	p.Stroke(view, ui.Hex("#F4F8FD"), 2, 1.5)
	p.Stroke(mini, t.Border, 6, 1)
	p.Text(r.X+14, r.Y+r.H-32, fmt.Sprintf("(%d, %d) · %.0f%% · 拖动平移 / 滚轮缩放", v.HoverRow, v.HoverCol, v.Cell/42*100), 12, t.TextMuted)
}

func minimapMarkerPos(mini ui.Rect, rows, cols, cellIdx int, margin float32) (float32, float32) {
	row, col := cellIdx/cols, cellIdx%cols
	x := mini.X + (float32(col)+0.5)/float32(max(1, cols))*mini.W
	y := mini.Y + (float32(row)+0.5)/float32(max(1, rows))*mini.H
	x = max(mini.X+margin, min(mini.X+mini.W-margin, x))
	y = max(mini.Y+margin, min(mini.Y+mini.H-margin, y))
	return x, y
}

func clampMinimapRect(mini ui.Rect, x, y, size float32) ui.Rect {
	x1 := max(mini.X+1, x)
	y1 := max(mini.Y+1, y)
	x2 := min(mini.X+mini.W-1, x+size)
	y2 := min(mini.Y+mini.H-1, y+size)
	return ui.Rect{X: x1, Y: y1, W: max(2, x2-x1), H: max(2, y2-y1)}
}

func abs32(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}
