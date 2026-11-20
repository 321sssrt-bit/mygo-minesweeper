package main

import (
	"fmt"
	"math"
	"time"

	"github.com/egoist/mygo/ui"
)

type boardView struct {
	X, Y           float32
	Cell           float32
	Width, Height  float32
	needsCenter    bool
	HoverRow       int
	HoverCol       int
	HoverX, HoverY float32
	HasHover       bool
	WasHovered     bool
	PressX, PressY float32
	DragX, DragY   float32
	Dragging       bool
	Pressed        bool
	PressButton    int
	LastEffect     time.Time
	EffectCell     int
	LastHoverHint  HoverHint
}

const (
	minimapSize       = float32(148)
	minimapMargin     = float32(10)
	classicMinimapPad = float32(4)
)

// classicMinimapPlot insets the board area inside the rounded classic minimap
// card so corner cells of 100x100 boards stay inside the clip and an outset
// viewport frame clears edge rows and columns.
func classicMinimapPlot(mini ui.Rect) ui.Rect {
	return ui.Rect{
		X: mini.X + classicMinimapPad,
		Y: mini.Y + classicMinimapPad,
		W: mini.W - classicMinimapPad*2,
		H: mini.H - classicMinimapPad*2,
	}
}

func newBoardView() *boardView {
	return &boardView{Cell: 26, X: 487 * 26, Y: 487 * 26, needsCenter: true, HoverRow: -1, HoverCol: -1, EffectCell: -1}
}

func isFinite32(v float32) bool {
	return !math.IsNaN(float64(v)) && !math.IsInf(float64(v), 0)
}

func (v *boardView) updateHover(g *Game) {
	if !v.HasHover || v.minimapFor(g, v.HoverX, v.HoverY) || v.HoverX < 0 || v.HoverY < 0 || v.HoverX >= v.Width || v.HoverY >= v.Height {
		v.HoverRow, v.HoverCol = -1, -1
		return
	}
	row, col, ok := v.cellAt(g, v.HoverX, v.HoverY)
	if !ok {
		v.HoverRow, v.HoverCol = -1, -1
		return
	}
	v.HoverRow, v.HoverCol = row, col
}

func (v *boardView) panLimit(g *Game) (float32, float32) {
	boardW := float32(g.Cols) * v.Cell
	boardH := float32(g.Rows) * v.Cell
	maxX := max(float32(0), boardW-v.Width)
	maxY := max(float32(0), boardH-v.Height)
	if g.duel == nil {
		// Reserve overscroll past the bottom-right minimap so corner cells of
		// large classic boards can always be dragged clear and clicked.
		if mini := v.minimapBoundsFor(g); boardW > mini.X && boardH > mini.Y {
			maxX = max(maxX, boardW-max(float32(0), mini.X-minimapMargin))
			maxY = max(maxY, boardH-max(float32(0), mini.Y-minimapMargin))
		}
	}
	return maxX, maxY
}

func (v *boardView) clamp(g *Game) {
	if !isFinite32(v.Cell) {
		v.Cell = 26
	}
	v.Cell = max(10, min(42, v.Cell))
	if !isFinite32(v.Width) || v.Width < 100 {
		v.Width = 100
	}
	if !isFinite32(v.Height) || v.Height < 100 {
		v.Height = 100
	}
	if g == nil || g.Rows <= 0 || g.Cols <= 0 {
		v.X, v.Y = 0, 0
		v.HoverRow, v.HoverCol = -1, -1
		return
	}
	if !isFinite32(v.X) {
		v.X = 0
	}
	if !isFinite32(v.Y) {
		v.Y = 0
	}
	maxX, maxY := v.panLimit(g)
	v.X = max(0, min(maxX, v.X))
	v.Y = max(0, min(maxY, v.Y))
	v.updateHover(g)
}

func (v *boardView) center(g *Game, row, col int) {
	v.needsCenter = false
	if g == nil || g.Rows <= 0 || g.Cols <= 0 {
		return
	}
	v.X = (float32(col)+0.5)*v.Cell - v.Width/2
	v.Y = (float32(row)+0.5)*v.Cell - v.Height/2
	v.clamp(g)
}

func (v *boardView) centerBoard(g *Game) {
	if g == nil || g.Rows <= 0 || g.Cols <= 0 {
		return
	}
	v.center(g, g.Rows/2, g.Cols/2)
}

func (v *boardView) zoom(g *Game, atX, atY, factor float32) {
	if g == nil || factor <= 0 || !isFinite32(factor) || !isFinite32(atX) || !isFinite32(atY) {
		return
	}
	v.needsCenter = false
	if !isFinite32(v.Cell) {
		v.Cell = 26
	}
	old := max(float32(10), v.Cell)
	next := max(10, min(42, old*factor))
	worldX := (v.X + atX) / old
	worldY := (v.Y + atY) / old
	v.Cell = next
	v.X = worldX*next - atX
	v.Y = worldY*next - atY
	v.clamp(g)
}

func (v *boardView) cellAt(g *Game, x, y float32) (int, int, bool) {
	if g == nil || g.Rows <= 0 || g.Cols <= 0 || v.Cell <= 0 ||
		!isFinite32(v.Cell) || !isFinite32(x) || !isFinite32(y) || !isFinite32(v.X) || !isFinite32(v.Y) {
		return -1, -1, false
	}
	if x < 0 || y < 0 || (v.Width > 0 && x >= v.Width) || (v.Height > 0 && y >= v.Height) {
		return -1, -1, false
	}
	wx, wy := v.X+x, v.Y+y
	if wx < 0 || wy < 0 {
		return -1, -1, false
	}
	colF, rowF := wx/v.Cell, wy/v.Cell
	if colF >= float32(g.Cols) || rowF >= float32(g.Rows) {
		return -1, -1, false
	}
	return int(rowF), int(colF), true
}

func (v *boardView) minimapBoundsFor(g *Game) ui.Rect {
	if g != nil && g.Rows == 1000 && g.Cols == 1000 {
		return ui.Rect{
			X: v.Width - minimapSize - minimapMargin,
			Y: v.Height - minimapSize - minimapMargin,
			W: minimapSize,
			H: minimapSize,
		}
	}
	s := min(minimapSize, max(float32(20), min(v.Width, v.Height)-minimapMargin*2))
	return ui.Rect{
		X: v.Width - minimapMargin - s,
		Y: v.Height - minimapMargin - s,
		W: s,
		H: s,
	}
}

func (v *boardView) minimapFor(g *Game, x, y float32) bool {
	if !isFinite32(x) || !isFinite32(y) {
		return false
	}
	mini := v.minimapBoundsFor(g)
	return x >= mini.X && y >= mini.Y && x <= mini.X+mini.W && y <= mini.Y+mini.H
}

func (v *boardView) minimapCellAt(g *Game, x, y float32) (int, int) {
	if g == nil || g.Rows <= 0 || g.Cols <= 0 || !isFinite32(x) || !isFinite32(y) {
		return 0, 0
	}
	mini := v.minimapBoundsFor(g)
	if g.duel == nil {
		mini = classicMinimapPlot(mini)
	}
	if !isFinite32(mini.W) || !isFinite32(mini.H) || mini.W <= 0 || mini.H <= 0 {
		return 0, 0
	}
	fx := max(float32(0), min(float32(1), (x-mini.X)/mini.W))
	fy := max(float32(0), min(float32(1), (y-mini.Y)/mini.H))
	row := min(g.Rows-1, max(0, int(fy*float32(g.Rows))))
	col := min(g.Cols-1, max(0, int(fx*float32(g.Cols))))
	return row, col
}

func containsIndex(list []int, target int) bool {
	for _, idx := range list {
		if idx == target {
			return true
		}
	}
	return false
}

func (a *minesweeperApp) applyCanvasCellAction(row, col, button int) ActionResult {
	if a.game == nil {
		return ActionResult{Kind: ActionNoop}
	}
	cell := a.game.Cell(row, col)
	if cell == nil {
		return ActionResult{Kind: ActionNoop}
	}
	if a.game.duel == nil {
		if button == 1 || (button == 0 && a.flagMode) {
			return a.game.ToggleFlag(row, col, a.allowQuestion)
		}
		if button == 0 {
			if cell.Revealed {
				return a.game.Chord(row, col)
			}
			return a.game.Reveal(row, col)
		}
		return ActionResult{Kind: ActionNoop}
	}
	if button == 1 || (button == 0 && a.flagMode) {
		result := a.game.ToggleFlag(row, col, false)
		if result.Kind == ActionNoop {
			result = a.game.UnflagDuel(row, col)
		}
		return result
	}
	if button == 0 {
		if cell.Revealed && !cell.Exploded && cell.Adjacent > 0 {
			result := a.game.ClaimDuelMines(row, col)
			if result.Kind == ActionNoop {
				result = a.game.Chord(row, col)
			}
			return result
		}
		if !cell.Revealed {
			return a.game.Reveal(row, col)
		}
	}
	return ActionResult{Kind: ActionNoop}
}

func (a *minesweeperApp) classicBoard(c *ui.Context) {
	a.canvasBoard(c, "classic-stage", "classic-board", "经典棋盘")
}

func (a *minesweeperApp) duelBoard(c *ui.Context) {
	a.canvasBoard(c, "duel-stage", "duel-board", "决斗棋盘")
}

func (a *minesweeperApp) canvasBoard(c *ui.Context, stageKey, boardKey, label string) {
	v := a.board
	v.clamp(a.game)
	ui.Column(c.Key(stageKey)).Grow(1).FillWidth().Children(func() {
		boardBox := ui.Box(c.Key(boardKey)).Grow(1).FillWidth().Label(label).
			Cursor(ui.CursorPointer).Focusable()
		hovered := boardBox.Hovered()
		if v.WasHovered && !hovered && !v.Pressed {
			v.HasHover = false
			v.HoverRow, v.HoverCol = -1, -1
		}
		v.WasHovered = hovered
		boardBox.HandleInput(func(ev ui.InputEvent) bool {
			if ev.Kind == ui.InputPointerMove || ev.Kind == ui.InputScroll || ev.Kind == ui.InputZoom {
				v.HoverX, v.HoverY, v.HasHover = ev.X, ev.Y, true
				v.updateHover(a.game)
			}
			switch ev.Kind {
			case ui.InputScroll:
				v.zoom(a.game, ev.X, ev.Y, 1+max(-0.35, min(0.35, -ev.DY/400)))
				return true
			case ui.InputZoom:
				v.zoom(a.game, ev.X, ev.Y, ev.Scale)
				return true
			case ui.InputPointerDown:
				v.HoverX, v.HoverY, v.HasHover = ev.X, ev.Y, true
				if ev.Button < 0 || ev.Button > 2 {
					return false
				}
				if ev.Button == 0 && v.minimapFor(a.game, ev.X, ev.Y) {
					row, col := v.minimapCellAt(a.game, ev.X, ev.Y)
					v.center(a.game, row, col)
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
					v.needsCenter = false
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
				v.HoverX, v.HoverY, v.HasHover = ev.X, ev.Y, true
				button, dragged := v.PressButton, v.Dragging
				v.Pressed, v.Dragging = false, false
				v.updateHover(a.game)
				if ev.Canceled || dragged || v.minimapFor(a.game, ev.X, ev.Y) || v.minimapFor(a.game, v.PressX, v.PressY) {
					return true
				}
				row, col, ok := v.cellAt(a.game, ev.X, ev.Y)
				if !ok {
					return true
				}
				result := a.applyCanvasCellAction(row, col, button)
				if result.Kind != ActionNoop {
					a.pendingResult = result
					a.pendingRow = row
					a.pendingCol = col
					if a.game.duel != nil {
						v.LastEffect = c.Now()
						v.EffectCell = row*a.game.Cols + col
					}
				}
				return true
			}
			return false
		}).Draw(func(p *ui.Painter, r ui.Rect) {
			v.Width, v.Height = r.W, r.H
			if v.needsCenter {
				v.needsCenter = false
				if a.game != nil && (a.game.Rows != 1000 || a.game.Cols != 1000) {
					v.centerBoard(a.game)
				} else {
					v.clamp(a.game)
				}
			} else {
				v.clamp(a.game)
			}
			a.paintBoard(p, r)
		})

		if a.game != nil && a.game.duel != nil && a.game.Status == StatusPlaying && a.game.duel.playerStunned() {
			d := a.game.duel
			hint := "暂无法操作棋盘 · 等待硬直结束"
			if a.bandMode() && a.memberID == MemberTomori {
				if a.game.SkillUsesLeft > 0 {
					hint = "按 E 发动「创可贴指引」立即解除硬直"
				} else {
					hint = "技能充能已耗尽 · 等待硬直自然结束"
				}
			}
			ui.Row(c.Key("duel-stun-banner")).Absolute().Top(16).Start(0).End(0).
				Justify(ui.Center).PassThrough().Children(func() {
				ui.Column(c).Padding(10, 18).Gap(3).Radius(10).AlignItems(ui.Center).
					Background(ui.Hex("#450A18").Alpha(0.88)).Border(1.5, ui.Hex("#FB7185")).
					PassThrough().Children(func() {
					ui.Textf(c, "踩雷硬直中 · %.1f 秒", max(0, d.StunnedUntil-d.Elapsed)).
						FontSize(15).Bold().TextColor(ui.Hex("#FFE4E6"))
					ui.Text(c, hint).FontSize(12).TextColor(ui.Hex("#FDA4AF"))
				})
			})
		}
	})
}

func (a *minesweeperApp) paintBoard(p *ui.Painter, r ui.Rect) {
	v, g := a.board, a.game
	t := p.Theme()
	now := p.Now()
	hoverHint := g.InspectNumberHover(v.HoverRow, v.HoverCol, a.flagMode)
	v.LastHoverHint = hoverHint
	hintFill, hintStroke := hoverHintColors(hoverHint.Kind, a.memberID, a.darkMode)
	shakeX, shakeY := a.currentShakeOffset(now)
	if shakeX != 0 || shakeY != 0 {
		p.AnimationFrame()
	}

	pal, art, inks, dp := paperFor(a.darkMode), artFor(a.darkMode), duelInksFor(a.darkMode), duelPromptsFor(a.darkMode)
	classic := g.duel == nil
	radius := min(4, v.Cell/7)
	// Numbers, props and slip shadows only once a cell is big enough to
	// show them; zoomed out, the board is flat slips, washes and state dots.
	detail := v.Cell >= 16
	var shieldDot ui.Color
	if !detail && classic {
		shieldDot = pal.InkOf(memberImage(MemberSoyorin))
	}
	numberSize := v.Cell * 0.55
	var numberW, numberH [len(cellDigits)]float32
	var questionW, questionH float32

	p.Clip(r, 8, func() {
		// The canvas is graph paper, as under the classic grid.
		p.Fill(r, pal.Board, 8)
		p.Transform(ui.Translation(shakeX, shakeY), 0, 0, func() {
			// One cell of margin before the view too: a shake (at most a
			// cell) must not bare the page along the top or left edge.
			firstCol, lastCol := int(v.X/v.Cell)-1, min(g.Cols, int((v.X+r.W)/v.Cell)+2)
			firstRow, lastRow := int(v.Y/v.Cell)-1, min(g.Rows, int((v.Y+r.H)/v.Cell)+2)
			// Ruled lines run along the cell edges, only as far as the board,
			// and past the view by the shake so its edges keep their lines.
			sx, sy := abs32(shakeX), abs32(shakeY)
			top, bottom := max(r.Y-sy, r.Y-v.Y), min(r.Y+r.H+sy, r.Y+float32(g.Rows)*v.Cell-v.Y)
			left, right := max(r.X-sx, r.X-v.X), min(r.X+r.W+sx, r.X+float32(g.Cols)*v.Cell-v.X)
			for col := max(0, firstCol); col <= lastCol; col++ {
				x := r.X + float32(col)*v.Cell - v.X
				p.Line(x, top, x, bottom, 1, pal.Grid)
			}
			for row := max(0, firstRow); row <= lastRow; row++ {
				y := r.Y + float32(row)*v.Cell - v.Y
				p.Line(left, y, right, y, 1, pal.Grid)
			}
			for row := max(0, firstRow); row < lastRow; row++ {
				for col := max(0, firstCol); col < lastCol; col++ {
					cell := g.Cell(row, col)
					idx := row*g.Cols + col
					x := r.X + float32(col)*v.Cell - v.X
					y := r.Y + float32(row)*v.Cell - v.Y
					tile := ui.Rect{X: x + 1, Y: y + 1, W: v.Cell - 2, H: v.Cell - 2}
					isHovered := row == v.HoverRow && col == v.HoverCol
					isHintCenter := hoverHint.Kind != HintNone && idx == hoverHint.Center
					isHintTarget := hoverHint.Kind != HintNone && !isHintCenter && containsIndex(hoverHint.Targets, idx)

					// Flags (a shielded mine included) and unopened cells are
					// paper slips stuck on the page; opened cells are the page.
					if cell.Flagged || !cell.Revealed {
						bg, outline, dot := pal.Slip, ui.Color{}, ui.Color{}
						var prop *ui.SVG
						question := false
						switch {
						case cell.WrongFlag:
							prop, dot = art.WrongFlag, pal.BoomInk
						case cell.Flagged && cell.ShieldSaved:
							prop, dot = art.Shield, shieldDot
						case cell.Flagged:
							prop, dot = art.Flag, pal.BandAidEdge
						case cell.Questioned:
							question, dot = true, pal.Graphite
						}
						// Duel flags already carry their side's outline when
						// zoomed out; state dots are classic-only.
						if !classic {
							dot = ui.Color{}
						}
						// A duel flag is outlined in its side's colour; classic
						// flags on the canvas belong to no side.
						if cell.Flagged && cell.FlagOwner != DuelNobody {
							outline = duelOwnerColor(cell.FlagOwner)
						}
						if isHintTarget {
							if classic {
								bg = hintFill.Alpha(0.42).Over(pal.Slip)
							} else {
								bg, outline = hintFill.Alpha(0.32).Over(pal.Slip), hintStroke
							}
						} else if hoverHint.Kind == HintNone && isHovered {
							bg = pal.Slip.Mix(pal.Sheet, 0.45)
						}
						paintSlip(p, tile, radius, bg, pal.SlipShade, outline, prop, dot, detail)
						if detail && question {
							mark := ui.Span{Text: "?", Size: numberSize, Weight: 700, Color: pal.Graphite}
							if questionW == 0 {
								questionW, questionH = p.MeasureText(0, mark)
							}
							p.RichText(tile.X+(tile.W-questionW)/2, tile.Y+(tile.H-questionH)/2, 0, mark)
						}
						continue
					}

					if classic {
						switch {
						case cell.Exploded:
							p.Fill(tile, pal.Boom, radius)
						case cell.IsMine:
						case cell.Adjacent > 0:
							if isHintCenter {
								p.Fill(tile, hintFill.Alpha(0.38), radius)
							} else if k, ok := a.rippleAlphaAt(idx, now); ok {
								p.Fill(tile, bandColor.Alpha(0.22*k), radius)
							} else if isHovered {
								p.Fill(tile, pal.Ink.Alpha(0.05), radius)
							}
						default:
							if k, ok := a.rippleAlphaAt(idx, now); ok {
								p.Fill(tile, bandColor.Alpha(0.26*k), radius)
							}
						}
					} else {
						switch {
						case cell.Exploded:
							p.Fill(tile, pal.Boom, radius)
						case cell.Owner != DuelNobody:
							p.Fill(tile, duelOwnerColor(cell.Owner).Alpha(duelTerritoryWash), radius)
						}
						if isHintCenter {
							p.Fill(tile, hintFill.Alpha(0.24), radius)
							if v.Cell >= 15 {
								p.Stroke(tile, hintStroke, radius, 1)
							}
						}
						if k, ok := a.rippleAlphaAt(idx, now); ok {
							ripColor := duelOwnerColor(cell.Owner)
							p.Fill(tile, ripColor.Alpha(0.36*k), radius)
							if v.Cell >= 15 {
								p.Stroke(tile, ripColor.Alpha(0.4+0.6*k), radius, 1)
							}
						}
					}
					if !detail {
						// Too small for a knot: a dot of its ink still tells a
						// mine (and a hit one) from opened ground.
						if cell.IsMine {
							ink := pal.Graphite
							if cell.Exploded {
								ink = pal.BoomInk
							}
							paintCellDot(p, tile, ink, 0.5)
						}
						continue
					}
					switch {
					case cell.Exploded:
						p.Image(art.KnotHit, propRect(tile), ui.Contain)
					case cell.IsMine:
						p.Image(art.Knot, propRect(tile), ui.Contain)
					case cell.Adjacent > 0 && cell.Adjacent < len(cellDigits):
						n := cell.Adjacent
						digit := ui.Span{Text: cellDigits[n], Size: numberSize, Weight: 700, Color: inks[cell.Owner][n]}
						if numberW[n] == 0 {
							numberW[n], numberH[n] = p.MeasureText(0, digit)
						}
						p.RichText(tile.X+(tile.W-numberW[n])/2, tile.Y+(tile.H-numberH[n])/2, 0, digit)
					}
				}
			}
			if _, ok := g.ScanAt(v.HoverRow, v.HoverCol); ok {
				x := r.X + float32(v.HoverCol-4)*v.Cell - v.X
				y := r.Y + float32(v.HoverRow-4)*v.Cell - v.Y
				p.Stroke(ui.Rect{X: x, Y: y, W: 9 * v.Cell, H: 9 * v.Cell}, dp.ScanFrame, 5, 2)
			}
			if g.duel != nil && g.Cols > 0 && g.duel.LastAI >= 0 {
				row, col := g.duel.LastAI/g.Cols, g.duel.LastAI%g.Cols
				x := r.X + float32(col)*v.Cell - v.X
				y := r.Y + float32(row)*v.Cell - v.Y
				p.Stroke(ui.Rect{X: x, Y: y, W: v.Cell, H: v.Cell}, duelPalette.AI, 4, 2)
			}
			if v.EffectCell >= 0 && g.Cols > 0 {
				elapsed := now.Sub(v.LastEffect)
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
			a.pruneAndDrawPopups(p, r, now)
		})

		a.pruneRipples(now)
		if len(a.ripples) > 0 {
			p.AnimationFrame()
		}
		if count, ok := g.ScanAt(v.HoverRow, v.HoverCol); ok {
			badge := ui.Span{Text: fmt.Sprintf("9×9 扫描：%d 雷", count), Size: 14, Weight: 700, Color: dp.ScanBadgeText}
			bw, bh := p.MeasureText(0, badge)
			box := ui.Rect{X: r.X + 12, Y: r.Y + 12, W: max(float32(166), bw+24), H: 34}
			p.Shadow(box, 6, 0, 1, 3, 0, pal.SlipShade)
			p.Fill(box, dp.ScanBadgeBg, 6)
			p.Stroke(box, dp.ScanBadgeBorder, 6, 1.2)
			p.RichText(box.X+(box.W-bw)/2, box.Y+(box.H-bh)/2, 0, badge)
		}
		if g.duel != nil && g.Status == StatusPlaying && g.duel.playerStunned() {
			pulse := 0.45 + 0.55*float32(0.5+0.5*math.Sin(float64(now.UnixMilli())/120.0))
			p.Stroke(ui.Rect{X: r.X + 2, Y: r.Y + 2, W: r.W - 4, H: r.H - 4}, ui.Hex("#FB7185").Alpha(pulse), 8, 4)
			p.AnimationFrame()
		}
		mini := v.minimapBoundsFor(g)
		mini.X += r.X
		mini.Y += r.Y
		if classic {
			plot := classicMinimapPlot(mini)
			view := minimapViewportRect(plot, g.Rows, g.Cols, v.X, v.Y, r.W, r.H, v.Cell)
			paintClassicMinimap(p, g, mini, plot, view, pal, classicMinimapFor(a.darkMode))
		} else {
			view := minimapViewportRect(mini, g.Rows, g.Cols, v.X, v.Y, r.W, r.H, v.Cell)
			paintDuelMinimap(p, g, mini, view, t.Border)
		}
		// The caption sits on a scrap of the sheet so it reads over slips.
		captionText := fmt.Sprintf("%.0f%% · 拖动平移 / 滚轮缩放", v.Cell/42*100)
		if v.HoverRow >= 0 && v.HoverCol >= 0 {
			captionText = fmt.Sprintf("(%d, %d) · %s", v.HoverRow, v.HoverCol, captionText)
		}
		caption := ui.Span{Text: captionText, Size: 12, Color: t.TextMuted}
		cw, ch := p.MeasureText(0, caption)
		p.Fill(ui.Rect{X: r.X + 8, Y: r.Y + r.H - 36, W: cw + 12, H: ch + 8}, pal.Sheet.Alpha(0.92), 6)
		p.RichText(r.X+14, r.Y+r.H-32, 0, caption)
	})
}

func (a *minesweeperApp) rippleAlphaAt(idx int, now time.Time) (float32, bool) {
	rip, ok := a.ripples[idx]
	if !ok {
		return 0, false
	}
	elapsed := now.Sub(rip.Start)
	if elapsed < rip.Delay || elapsed >= rip.Delay+rippleDuration {
		return 0, false
	}
	return 1 - float32(elapsed-rip.Delay)/float32(rippleDuration), true
}

// cellDigits are the numbers a revealed cell can show, by count.
var cellDigits = [...]string{"", "1", "2", "3", "4", "5", "6", "7", "8"}

// propRect is where a knot or band-aid sits in a cell.
func propRect(tile ui.Rect) ui.Rect {
	s := min(tile.W, tile.H) * 0.86
	return ui.Rect{X: tile.X + (tile.W-s)/2, Y: tile.Y + (tile.H-s)/2, W: s, H: s}
}

// paintCellDot draws a centered circular marker inside tile when zoomed out
// past the prop/glyph detail threshold.
func paintCellDot(p *ui.Painter, tile ui.Rect, ink ui.Color, scale float32) {
	s := min(tile.W, tile.H) * scale
	p.Fill(ui.Rect{X: tile.X + (tile.W-s)/2, Y: tile.Y + (tile.H-s)/2, W: s, H: s}, ink, s/2)
}

// paintSlip paints a paper slip stuck on the page, with an optional
// outline and prop on it; without detail, only a thinner outline and an
// optional state dot. Unlike the classic grid's slips these stay square:
// the canvas repaints thousands a frame, the tilt is under a pixel at
// canvas sizes, and turned shadows cost far more to draw.
func paintSlip(p *ui.Painter, tile ui.Rect, radius float32, bg, shade, outline ui.Color, prop *ui.SVG, dot ui.Color, detail bool) {
	if detail {
		p.Shadow(tile, radius, 0, 1, 1.5, 0, shade)
	}
	p.Fill(tile, bg, radius)
	if outline.A > 0 {
		width := float32(1.5)
		if !detail {
			width = 1
		}
		p.Stroke(tile, outline, radius, width)
	}
	if detail {
		if prop != nil {
			p.Image(prop, propRect(tile), ui.Contain)
		}
		return
	}
	if dot.A > 0 {
		paintCellDot(p, tile, dot, 0.45)
	}
}

func (a *minesweeperApp) pruneAndDrawPopups(p *ui.Painter, r ui.Rect, now time.Time) {
	if len(a.duelPopups) == 0 {
		return
	}
	v := a.board
	pal, dp := paperFor(a.darkMode), duelPromptsFor(a.darkMode)
	kept := a.duelPopups[:0]
	for _, pop := range a.duelPopups {
		elapsed := now.Sub(pop.Start)
		if elapsed < 0 || elapsed >= duelPopupDuration {
			continue
		}
		kept = append(kept, pop)
		k := float32(elapsed) / float32(duelPopupDuration)
		alpha := 1 - k*0.85
		style := dp.Popups[pop.Kind]
		label := ui.Span{Text: pop.Text, Size: 13, Weight: 700, Color: style.Text.Alpha(alpha)}
		tw, th := p.MeasureText(0, label)
		pillW := max(float32(44), tw+16)
		pillH := max(float32(22), th+8)
		pillX := max(r.X+8, min(r.X+r.W-pillW-8, r.X+(float32(pop.Col)+0.5)*v.Cell-v.X-pillW/2))
		pillY := max(r.Y+8, min(r.Y+r.H-pillH-8, r.Y+float32(pop.Row)*v.Cell-v.Y-16-k*28))
		pill := ui.Rect{X: pillX, Y: pillY, W: pillW, H: pillH}
		p.Shadow(pill, 6, 0, 1, 2, 0, pal.SlipShade.Alpha(0.5*alpha))
		p.Fill(pill, style.Bg.Alpha(0.94*alpha), 6)
		p.Stroke(pill, style.Border.Alpha(0.8*alpha), 6, 1.2)
		p.RichText(pill.X+(pill.W-tw)/2, pill.Y+(pill.H-th)/2, 0, label)
	}
	a.duelPopups = kept
	if len(a.duelPopups) > 0 {
		p.AnimationFrame()
	}
}

type classicMinimapMark uint8

const (
	classicMarkNone classicMinimapMark = iota
	classicMarkOpened
	classicMarkFlag
	classicMarkBoom
)

func classicCellMark(cell *Cell) classicMinimapMark {
	switch {
	case cell.Exploded || cell.WrongFlag:
		return classicMarkBoom
	case cell.Flagged:
		return classicMarkFlag
	case cell.Revealed && !cell.IsMine:
		return classicMarkOpened
	default:
		return classicMarkNone
	}
}

func (mc *minimapColors) colorFor(mark classicMinimapMark) ui.Color {
	switch mark {
	case classicMarkBoom:
		return mc.Boom
	case classicMarkFlag:
		return mc.Flag
	default:
		return mc.Opened
	}
}

func paintClassicMinimap(p *ui.Painter, g *Game, mini, plot, view ui.Rect, pal *palette, mc *minimapColors) {
	p.Shadow(mini, 6, 0, 2, 6, 0, pal.SlipShade)
	p.Clip(mini, 6, func() {
		p.Fill(mini, mc.Bg, 6)
		cw := plot.W / float32(g.Cols)
		ch := plot.H / float32(g.Rows)
		for row := range g.Rows {
			y0 := plot.Y + float32(row)*ch
			y1 := plot.Y + float32(row+1)*ch
			rowOff := row * g.Cols
			for col := 0; col < g.Cols; {
				mark := classicCellMark(&g.Cells[rowOff+col])
				end := col + 1
				for end < g.Cols && classicCellMark(&g.Cells[rowOff+end]) == mark {
					end++
				}
				if mark != classicMarkNone {
					x0 := plot.X + float32(col)*cw
					x1 := plot.X + float32(end)*cw
					p.Fill(ui.Rect{X: x0, Y: y0, W: x1 - x0, H: y1 - y0}, mc.colorFor(mark), 0)
				}
				col = end
			}
		}
		// Outset the frame by 2px so even at 1x scale the 1.5px stroke clears
		// edge rows and columns when the viewport sits flush on the boundary.
		const (
			frameStroke = float32(1.5)
			frameOutset = float32(2)
		)
		frame := ui.Rect{
			X: view.X - frameOutset,
			Y: view.Y - frameOutset,
			W: view.W + frameOutset*2,
			H: view.H + frameOutset*2,
		}
		p.Stroke(frame, mc.Viewport, 2, frameStroke)
	})
	p.Stroke(mini, mc.Border, 6, 1)
}

func paintDuelMinimap(p *ui.Painter, g *Game, mini, view ui.Rect, border ui.Color) {
	dm := duelMinimapPalette
	p.Fill(mini, dm.Bg.Alpha(0.94), 6)
	if len(g.duel.Minimap) == minimapGridSize*minimapGridSize {
		rowMap, colMap := minimapAxisSlots(g)
		bw := mini.W / float32(minimapGridSize)
		bh := mini.H / float32(minimapGridSize)
		fillW := max(1.5, bw)
		fillH := max(1.5, bh)
		for br := range minimapGridSize {
			srcRowOff := rowMap[br] * minimapGridSize
			for bc := range minimapGridSize {
				blk := g.duel.Minimap[srcRowOff+colMap[bc]]
				if blk.Player == 0 && blk.AI == 0 {
					continue
				}
				var color ui.Color
				switch {
				case blk.Player > 0 && blk.AI > 0:
					density := min(float32(1), float32(blk.Player+blk.AI)/24)
					color = dm.Contested.Alpha(0.65 + 0.35*density)
				case blk.Player > 0:
					density := min(float32(1), float32(blk.Player)/20)
					color = dm.Player.Alpha(0.65 + 0.35*density)
				default:
					density := min(float32(1), float32(blk.AI)/20)
					color = dm.AI.Alpha(0.65 + 0.35*density)
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
			p.Stroke(ring, dm.AIMarker.Alpha(1-pulse*0.75), 99, 1.5)
		}
		p.Fill(ui.Rect{X: ax - 2.5, Y: ay - 2.5, W: 5, H: 5}, dm.AIMarker, 1)
		p.Stroke(ui.Rect{X: ax - 2.5, Y: ay - 2.5, W: 5, H: 5}, dm.AIStroke, 1, 1)
	}
	if hasPlayer {
		px, py := minimapMarkerPos(mini, g.Rows, g.Cols, g.duel.LastCell, 5)
		if hasAI {
			px, py = separateMinimapMarker(mini, px, py, ax, ay, 5)
		}
		if pulsing {
			ring := clampMinimapRect(mini, px-(3+pulse*3.5), py-(3+pulse*3.5), (3+pulse*3.5)*2)
			p.Stroke(ring, dm.PlayerMarker.Alpha(1-pulse*0.75), 99, 1.5)
		}
		p.Fill(ui.Rect{X: px - 2.5, Y: py - 2.5, W: 5, H: 5}, dm.PlayerMarker, 99)
		p.Stroke(ui.Rect{X: px - 2.5, Y: py - 2.5, W: 5, H: 5}, dm.PlayerStroke, 99, 1)
	}
	if pulsing && (hasAI || hasPlayer) {
		p.AnimationFrame()
	}
	p.Stroke(view, dm.Viewport, 2, 1.5)
	p.Stroke(mini, border, 6, 1)
}

func minimapAxisSlots(g *Game) (rowMap, colMap [minimapGridSize]int) {
	for i := range minimapGridSize {
		if g != nil && g.duel != nil && g.Rows > 0 && g.Rows < minimapGridSize {
			rowMap[i] = g.duel.minimapBlockIndex(g, i*g.Rows/minimapGridSize, 0) / minimapGridSize
		} else {
			rowMap[i] = i
		}
		if g != nil && g.duel != nil && g.Cols > 0 && g.Cols < minimapGridSize {
			colMap[i] = g.duel.minimapBlockIndex(g, 0, i*g.Cols/minimapGridSize) % minimapGridSize
		} else {
			colMap[i] = i
		}
	}
	return rowMap, colMap
}

func minimapViewportRect(mini ui.Rect, rows, cols int, viewX, viewY, viewW, viewH, cellSize float32) ui.Rect {
	if rows <= 0 || cols <= 0 ||
		!isFinite32(mini.X) || !isFinite32(mini.Y) || !isFinite32(mini.W) || !isFinite32(mini.H) ||
		mini.W <= 0 || mini.H <= 0 ||
		!isFinite32(viewX) || !isFinite32(viewY) || !isFinite32(viewW) || !isFinite32(viewH) || !isFinite32(cellSize) ||
		viewW < 0 || viewH < 0 || cellSize <= 0 {
		return ui.Rect{}
	}
	boardW := float32(cols) * cellSize
	boardH := float32(rows) * cellSize
	x1 := max(float32(0), min(float32(1), viewX/boardW))
	x2 := max(x1, min(float32(1), (viewX+viewW)/boardW))
	y1 := max(float32(0), min(float32(1), viewY/boardH))
	y2 := max(y1, min(float32(1), (viewY+viewH)/boardH))
	w := min(mini.W, (x2-x1)*mini.W)
	h := min(mini.H, (y2-y1)*mini.H)
	x := min(mini.X+mini.W-w, mini.X+x1*mini.W)
	y := min(mini.Y+mini.H-h, mini.Y+y1*mini.H)
	return ui.Rect{X: x, Y: y, W: w, H: h}
}

func minimapMarkerPos(mini ui.Rect, rows, cols, cellIdx int, margin float32) (float32, float32) {
	if rows <= 0 || cols <= 0 {
		return mini.X + mini.W/2, mini.Y + mini.H/2
	}
	row, col := cellIdx/cols, cellIdx%cols
	x := mini.X + (float32(col)+0.5)/float32(cols)*mini.W
	y := mini.Y + (float32(row)+0.5)/float32(rows)*mini.H
	x = max(mini.X+margin, min(mini.X+mini.W-margin, x))
	y = max(mini.Y+margin, min(mini.Y+mini.H-margin, y))
	return x, y
}

func separateMinimapMarker(mini ui.Rect, px, py, ax, ay, margin float32) (float32, float32) {
	dx, dy := px-ax, py-ay
	const minSep = 6.0
	if abs32(dx) >= minSep || abs32(dy) >= minSep {
		return px, py
	}
	minX, maxX := mini.X+margin, mini.X+mini.W-margin
	minY, maxY := mini.Y+margin, mini.Y+mini.H-margin
	if abs32(dx) >= abs32(dy) {
		dir := float32(1)
		if dx < 0 {
			dir = -1
		}
		candX := ax + dir*minSep
		if candX < minX || candX > maxX {
			candX = ax - dir*minSep
		}
		px = max(minX, min(maxX, candX))
	} else {
		dir := float32(1)
		if dy < 0 {
			dir = -1
		}
		candY := ay + dir*minSep
		if candY < minY || candY > maxY {
			candY = ay - dir*minSep
		}
		py = max(minY, min(maxY, candY))
	}
	return px, py
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
