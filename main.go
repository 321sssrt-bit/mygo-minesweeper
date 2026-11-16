package main

import (
	"fmt"
	"log"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

var (
	colBgDeep      = ui.Hex("#0B101B")
	colPanel       = ui.Hex("#121A2A")
	colCard        = ui.Hex("#182236")
	colBorder      = ui.Hex("#283956")
	colTeal        = ui.Hex("#55B2D6")
	colTextMain    = ui.Hex("#EDF4FB")
	colTextMuted   = ui.Hex("#93A7C2")
	colHiddenTile  = ui.Hex("#24334E")
	colHiddenHover = ui.Hex("#2F4366")
	colRevealed    = ui.Hex("#0E1523")
	colExploded    = ui.Hex("#991B1B")
	colWrongFlag   = ui.Hex("#4C1D2D")
	colGold        = ui.Hex("#E8A849")
	colPink        = ui.Hex("#F07AA4")
)

func numberColor(n int) ui.Color {
	switch n {
	case 1:
		return ui.Hex("#67B7E1") // Tomori
	case 2:
		return ui.Hex("#68C178") // Rana
	case 3:
		return ui.Hex("#F07AA4") // Anon
	case 4:
		return ui.Hex("#A78BFA") // Taki
	case 5:
		return ui.Hex("#F5B041") // Soyorin
	case 6:
		return ui.Hex("#22D3EE")
	case 7:
		return ui.Hex("#FB7185")
	default:
		return ui.Hex("#CBD5E1")
	}
}

type minesweeperApp struct {
	diffID        DifficultyID
	bandMode      bool
	memberID      MemberID
	flagMode      bool
	allowQuestion bool
	game          *Game
	quote         string
	startedAt     time.Time
	elapsed       int
	bestTimes     map[DifficultyID]int
	seed          uint64
}

func newMinesweeperApp(seed ...uint64) *minesweeperApp {
	var s uint64
	if len(seed) > 0 {
		s = seed[0]
	}
	app := &minesweeperApp{
		diffID:    DiffEasy,
		bandMode:  true,
		memberID:  MemberTomori,
		bestTimes: make(map[DifficultyID]int),
		seed:      s,
	}
	app.resetGame()
	return app
}

func (a *minesweeperApp) resetGame() {
	diff := FindDifficulty(a.diffID)
	if a.seed != 0 {
		a.game = NewGame(diff, a.bandMode, a.memberID, a.seed)
	} else {
		a.game = NewGame(diff, a.bandMode, a.memberID)
	}
	a.elapsed = 0
	a.startedAt = time.Time{}
	a.quote = PickQuote(a.memberID, "start")
}

func (a *minesweeperApp) applyResult(res ActionResult, now time.Time) {
	if res.Kind == ActionNoop {
		return
	}
	if a.game.Status == StatusPlaying && a.startedAt.IsZero() {
		a.startedAt = now
	}
	if res.Kind == ActionWon {
		if !a.startedAt.IsZero() {
			a.elapsed = int(now.Sub(a.startedAt).Seconds())
		}
		if prev, ok := a.bestTimes[a.diffID]; !ok || a.elapsed < prev {
			a.bestTimes[a.diffID] = a.elapsed
		}
	}
	if res.Kind == ActionLost {
		if !a.startedAt.IsZero() {
			a.elapsed = int(now.Sub(a.startedAt).Seconds())
		}
	}
	if res.QuoteEvent != "" {
		a.quote = PickQuote(a.memberID, res.QuoteEvent)
	}
}

func formatCounter(n int) string {
	if n < -99 {
		n = -99
	}
	if n > 999 {
		n = 999
	}
	if n < 0 {
		return fmt.Sprintf("-%02d", -n)
	}
	return fmt.Sprintf("%03d", n)
}

func (a *minesweeperApp) view(c *ui.Context) {
	now := c.Now()
	if a.game.Status == StatusPlaying {
		if a.startedAt.IsZero() {
			a.startedAt = now
		}
		a.elapsed = min(999, int(now.Sub(a.startedAt).Seconds()))
		c.After(time.Second)
	}

	if c.Shortcut(0, ui.KeyR) {
		a.resetGame()
	}
	if c.Shortcut(0, ui.KeyF) {
		a.flagMode = !a.flagMode
	}
	if c.Shortcut(0, ui.KeyE) {
		res := a.game.UseSkill()
		a.applyResult(res, now)
	}

	c.Root().Background(colBgDeep)
	member := FindMember(a.memberID)
	memberColor := ui.Hex(member.ColorHex)

	ui.Column(c).Fill().Padding(16).Gap(12).Children(func() {
		// Header
		ui.Row(c).AlignItems(ui.Center).Justify(ui.SpaceBetween).Gap(12).Children(func() {
			ui.Column(c).Gap(2).Children(func() {
				ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
					ui.Text(c, "MyGO!!!!!").FontSize(22).Bold().TextColor(colTextMain)
					ui.Text(c, "迷子扫雷").FontSize(20).Bold().TextColor(colTeal)
					ui.Text(c, "Native UI").FontSize(11).TextColor(colTeal).
						Padding(2, 7).Radius(999).Background(colTeal.Alpha(0.16)).Border(1, colTeal.Alpha(0.45))
				})
				ui.Text(c, "「迷子でもいい、迷子でも進め。」 · Built with egoist/mygo").FontSize(12).TextColor(colTextMuted)
			})

			ui.Row(c).Gap(8).Children(func() {
				modeLabel := "乐队应援模式"
				if !a.bandMode {
					modeLabel = "纯正经典模式"
				}
				modeBtn := ui.Button(c.Key("mode-toggle"), modeLabel).Radius(999).Padding(6, 12)
				if a.bandMode {
					modeBtn.Background(colTeal.Alpha(0.22)).Border(1, colTeal).TextColor(colTextMain)
				} else {
					modeBtn.Background(colCard).Border(1, colBorder).TextColor(colTextMuted)
				}
				if modeBtn.Clicked() {
					a.bandMode = !a.bandMode
					a.resetGame()
				}
			})
		})

		// Difficulty & Tool Controls
		ui.Row(c).Padding(8, 12).Radius(12).Background(colPanel).Border(1, colBorder).
			Justify(ui.SpaceBetween).AlignItems(ui.Center).Gap(10).Wrap().Children(func() {
			ui.Row(c).Gap(6).Children(func() {
				for _, d := range Difficulties {
					active := d.ID == a.diffID
					btn := ui.Box(c.Key(string(d.ID))).Padding(6, 12).Radius(9).Cursor(ui.CursorPointer).
						Label(d.Label)
					if active {
						btn.Background(colTeal.Alpha(0.22)).Border(1, colTeal)
					} else if btn.Hovered() {
						btn.Background(colCard).Border(1, colTeal.Alpha(0.5))
					} else {
						btn.Background(colBgDeep.Alpha(0.75)).Border(1, colBorder)
					}
					btn.Children(func() {
						ui.Text(c, d.Label).FontSize(13).Bold().TextColor(colTextMain)
						ui.Text(c, d.Subtitle).FontSize(11).TextColor(colTextMuted)
					})
					if btn.Clicked() && !active {
						a.diffID = d.ID
						a.resetGame()
					}
				}
			})

			ui.Row(c).Gap(8).Children(func() {
				flagLabel := "标旗模式: 关"
				if a.flagMode {
					flagLabel = "标旗模式: 开"
				}
				flagBtn := ui.Button(c.Key("flag-mode"), flagLabel).Radius(9).Padding(6, 11)
				if a.flagMode {
					flagBtn.Background(colPink.Alpha(0.22)).Border(1, colPink).TextColor(colTextMain)
				} else {
					flagBtn.Background(colCard).Border(1, colBorder).TextColor(colTextMuted)
				}
				if flagBtn.Clicked() {
					a.flagMode = !a.flagMode
				}

				qLabel := "? 标记: 关"
				if a.allowQuestion {
					qLabel = "? 标记: 开"
				}
				qBtn := ui.Button(c.Key("question-mode"), qLabel).Radius(9).Padding(6, 11)
				if a.allowQuestion {
					qBtn.Background(colTeal.Alpha(0.22)).Border(1, colTeal).TextColor(colTextMain)
				} else {
					qBtn.Background(colCard).Border(1, colBorder).TextColor(colTextMuted)
				}
				if qBtn.Clicked() {
					a.allowQuestion = !a.allowQuestion
				}
			})
		})

		// Member Companion Panel
		ui.Column(c).Padding(10, 12).Gap(10).Radius(12).Background(colPanel).Border(1, colBorder).Children(func() {
			ui.Row(c).Gap(8).Children(func() {
				for _, m := range Members {
					mc := ui.Hex(m.ColorHex)
					selected := m.ID == a.memberID
					chip := ui.Box(c.Key(string(m.ID))).Grow(1).Padding(7, 10).Radius(10).
						Cursor(ui.CursorPointer).Label(m.Name)
					if selected {
						chip.Background(mc.Alpha(0.2)).Border(1, mc)
					} else if chip.Hovered() {
						chip.Background(colCard).Border(1, mc.Alpha(0.55))
					} else {
						chip.Background(colBgDeep.Alpha(0.75)).Border(1, colBorder)
					}
					chip.Children(func() {
						ui.Row(c).AlignItems(ui.Center).Gap(7).Children(func() {
							ui.Box(c).Size(9, 9).Radius(999).Background(mc)
							ui.Column(c).Gap(1).Children(func() {
								ui.Textf(c, "%s %s", m.Name, m.Role).FontSize(13).Bold().TextColor(colTextMain)
								ui.Text(c, m.SkillName).FontSize(11).TextColor(colTextMuted)
							})
						})
					})
					if chip.Clicked() && !selected {
						a.memberID = m.ID
						if a.game.Status == StatusIdle || a.game.Status == StatusPlaying {
							a.game.MemberID = m.ID
						}
						a.quote = PickQuote(a.memberID, "start")
					}
				}
			})

			ui.Row(c).Padding(9, 12).Radius(10).Background(colBgDeep.Alpha(0.82)).
				Border(1, memberColor.Alpha(0.45)).AlignItems(ui.Center).Justify(ui.SpaceBetween).Gap(12).Children(func() {
				ui.Column(c).Grow(1).Gap(3).Children(func() {
					ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
						ui.Text(c, member.Name).FontSize(14).Bold().TextColor(memberColor)
						ui.Text(c, member.Role).FontSize(11).TextColor(colTextMuted).
							Padding(1, 6).Radius(5).Background(colCard)
						skillInfo := member.SkillName + " · " + member.SkillDesc
						if !a.bandMode {
							skillInfo = "经典模式：仅语音应援"
						}
						ui.Text(c, skillInfo).FontSize(12).TextColor(colTextMuted)
					})
					ui.Text(c, a.quote).FontSize(13).TextColor(colTextMain)
				})

				skillText := fmt.Sprintf("发动技能 ×%d", a.game.SkillUsesLeft)
				canTrigger := a.bandMode && !member.Passive && a.game.Status == StatusPlaying && a.game.SkillUsesLeft > 0
				if !a.bandMode {
					skillText = "经典模式"
				} else if member.Passive {
					if a.game.SkillUsesLeft > 0 {
						skillText = fmt.Sprintf("被动守护 ×%d", a.game.SkillUsesLeft)
					} else {
						skillText = "护盾已触发"
					}
				}
				skillBtn := ui.Button(c.Key("skill-btn"), skillText).Radius(9).Padding(8, 13).
					Disabled(!canTrigger).Background(memberColor.Alpha(0.24)).Border(1, memberColor).TextColor(colTextMain)
				if skillBtn.Clicked() {
					res := a.game.UseSkill()
					a.applyResult(res, now)
				}
			})
		})

		// Board Stage
		ui.Column(c).Grow(1).Padding(12).Gap(12).Radius(14).Background(colPanel).Border(1, colBorder).Children(func() {
			// HUD
			ui.Row(c).Padding(8, 12).Radius(10).Background(colBgDeep.Alpha(0.88)).
				Border(1, colBorder).AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
				ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
					ui.Text(c, "MINES").FontSize(11).Bold().TextColor(colTextMuted)
					ui.Text(c, formatCounter(a.game.RemainingMines())).FontSize(18).Bold().
						FontFeatures("tnum").TextColor(colTeal).
						Padding(3, 10).Radius(7).Background(ui.Hex("#060911")).Border(1, colTeal.Alpha(0.28))
				})

				resetLabel := "迷子出发"
				switch a.game.Status {
				case StatusPlaying:
					resetLabel = "演奏中 · 重开"
				case StatusWon:
					resetLabel = "Live 大成功!"
				case StatusLost:
					resetLabel = "再排练一次"
				}
				resetBtn := ui.Button(c.Key("reset-btn"), resetLabel).Radius(9).Padding(7, 16).
					Background(colTeal.Alpha(0.2)).Border(1, colTeal.Alpha(0.65)).TextColor(colTextMain)
				if resetBtn.Clicked() {
					a.resetGame()
				}

				ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
					bestText := "BEST --"
					if best, ok := a.bestTimes[a.diffID]; ok {
						bestText = fmt.Sprintf("BEST %ds", best)
					}
					ui.Text(c, bestText).FontSize(11).Bold().TextColor(colTextMuted).
						Padding(4, 8).Radius(7).Background(colCard)

					ui.Text(c, "TIME").FontSize(11).Bold().TextColor(colTextMuted)
					ui.Text(c, formatCounter(a.elapsed)).FontSize(18).Bold().
						FontFeatures("tnum").TextColor(colTeal).
						Padding(3, 10).Radius(7).Background(ui.Hex("#060911")).Border(1, colTeal.Alpha(0.28))
				})
			})

			// Grid
			cellSize := float32(34)
			cellFont := float32(15)
			if a.game.Cols >= 28 {
				cellSize = 26
				cellFont = 13
			} else if a.game.Cols >= 16 {
				cellSize = 29
				cellFont = 14
			}

			ui.ScrollBoth(c).Grow(1).Children(func() {
				ui.Column(c).FillWidth().AlignItems(ui.Center).Padding(4).Children(func() {
					ui.Grid(c).Columns(a.game.Cols).Gap(3).Padding(8).Radius(11).
						Background(ui.Hex("#090E18")).Border(1, colTeal.Alpha(0.24)).Children(func() {
						for i := range a.game.Cells {
							cell := &a.game.Cells[i]
							row, col := cell.Row, cell.Col
							tile := ui.Box(c.Key(i)).Size(cellSize, cellSize).Radius(5).Center().
								Label(fmt.Sprintf("cell-%d-%d", row, col))

							switch {
							case cell.Revealed:
								switch {
								case cell.Exploded:
									tile.Background(colExploded).Border(1, ui.Hex("#FCA5A5"))
									tile.Children(func() {
										ui.Text(c, "✕").FontSize(cellFont).Bold().TextColor(ui.Hex("#FEE2E2"))
									})
								case cell.IsMine:
									tile.Background(colRevealed).Border(1, ui.Hex("#EF4444").Alpha(0.55))
									tile.Children(func() {
										ui.Text(c, "●").FontSize(cellFont).Bold().TextColor(ui.Hex("#F87171"))
									})
								case cell.Adjacent > 0:
									bg := colRevealed
									if tile.Hovered() {
										bg = ui.Hex("#151F33")
									}
									tile.Background(bg).Border(1, colBorder.Alpha(0.45)).Cursor(ui.CursorPointer)
									tile.Children(func() {
										ui.Textf(c, "%d", cell.Adjacent).FontSize(cellFont).Bold().
											TextColor(numberColor(cell.Adjacent))
									})
								default:
									tile.Background(colRevealed).Border(1, colBorder.Alpha(0.32))
								}
							case cell.WrongFlag:
								tile.Background(colWrongFlag).Border(1, ui.Hex("#FB7185"))
								tile.Children(func() {
									ui.Text(c, "✕").FontSize(cellFont - 1).Bold().TextColor(ui.Hex("#FB7185"))
								})
							case cell.Flagged:
								borderCol := colTeal.Alpha(0.7)
								label := "⚑"
								labelCol := colGold
								if cell.ShieldSaved {
									borderCol = colGold
									label = "★"
								}
								tile.Background(colHiddenTile).Border(1, borderCol).Cursor(ui.CursorPointer)
								tile.Children(func() {
									ui.Text(c, label).FontSize(cellFont).Bold().TextColor(labelCol)
								})
							case cell.Questioned:
								tile.Background(colHiddenTile).Border(1, colTeal.Alpha(0.5)).Cursor(ui.CursorPointer)
								tile.Children(func() {
									ui.Text(c, "?").FontSize(cellFont).Bold().TextColor(colTeal)
								})
							default:
								bg := colHiddenTile
								bd := colBorder
								if tile.Hovered() {
									bg = colHiddenHover
									bd = colTeal.Alpha(0.75)
								}
								tile.Background(bg).Border(1, bd).Cursor(ui.CursorPointer)
							}

							if tile.RightClicked() {
								res := a.game.ToggleFlag(row, col, a.allowQuestion)
								a.applyResult(res, now)
							} else if tile.Clicked() {
								if cell.Revealed {
									res := a.game.Chord(row, col)
									a.applyResult(res, now)
								} else if a.flagMode {
									res := a.game.ToggleFlag(row, col, a.allowQuestion)
									a.applyResult(res, now)
								} else {
									res := a.game.Reveal(row, col)
									a.applyResult(res, now)
								}
							}
						}
					})
				})
			})
		})

		// Footer shortcuts
		ui.Row(c).Justify(ui.SpaceBetween).Children(func() {
			ui.Text(c, "左键翻开 · 右键标旗 · 点击已翻开数字快速排雷 (Chord)").FontSize(11).TextColor(colTextMuted)
			ui.Text(c, "快捷键：R 重开 · F 标旗模式 · E 发动技能").FontSize(11).TextColor(colTextMuted)
		})
	})
}

func main() {
	app := newMinesweeperApp()
	mygo.App.WhenReady(func() {
		mygo.NewWindow(mygo.WindowOptions{
			Title:           "MyGO!!!!! 迷子扫雷",
			Width:           960,
			Height:          780,
			BackgroundColor: "#0b101b",
			StateKey:        "mygo-minesweeper-main",
			Content:         ui.View(app.view),
		})
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
