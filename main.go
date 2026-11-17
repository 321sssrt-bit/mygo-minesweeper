package main

import (
	"fmt"
	"log"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

type chatMessage struct {
	Speaker  string
	ColorHex string
	Text     string
}

type minesweeperApp struct {
	diffIdx         int
	modeIdx         int // 0 = Band Live Mode, 1 = Classic Mode
	darkMode        bool
	memberID        MemberID
	flagMode        bool
	allowQuestion   bool
	showMemes       bool
	haruhikageAlert bool

	game          *Game
	quote         string
	startedAt     time.Time
	elapsed       int
	bestTimes     map[string]int
	unlockedMemes map[string]bool
	chatLog       []chatMessage
	seed          uint64
	board         *boardView
	lastTick      time.Time
	pendingResult ActionResult
	duelNoticeAt  time.Time
	duelEnded     bool
	classicSize   float32
}

func newMinesweeperApp(seed ...uint64) *minesweeperApp {
	var s uint64
	if len(seed) > 0 {
		s = seed[0]
	}
	app := &minesweeperApp{
		diffIdx:       0,
		modeIdx:       0,
		darkMode:      true,
		memberID:      MemberTomori,
		bestTimes:     make(map[string]int),
		unlockedMemes: make(map[string]bool),
		seed:          s,
		board:         newBoardView(),
	}
	app.resetGame(true)
	return app
}

func (a *minesweeperApp) currentDiff() Difficulty {
	if a.diffIdx < 0 || a.diffIdx >= len(Difficulties) {
		return Difficulties[0]
	}
	return Difficulties[a.diffIdx]
}

func (a *minesweeperApp) bandMode() bool {
	return a.modeIdx == 0
}

func (a *minesweeperApp) pushChat(speaker, colorHex, text string) {
	a.chatLog = append(a.chatLog, chatMessage{
		Speaker:  speaker,
		ColorHex: colorHex,
		Text:     text,
	})
	if len(a.chatLog) > 18 {
		a.chatLog = a.chatLog[len(a.chatLog)-18:]
	}
}

func (a *minesweeperApp) resetGame(initial bool) {
	a.chatLog = nil
	if !initial {
		a.seed = 0
	}
	diff := a.currentDiff()
	if diff.ID == "duel" {
		a.modeIdx = 0
	}
	if a.seed != 0 {
		a.game = NewGame(diff, a.bandMode(), a.memberID, a.seed)
	} else {
		a.game = NewGame(diff, a.bandMode(), a.memberID)
	}
	a.elapsed = 0
	a.startedAt = time.Time{}
	a.lastTick = time.Time{}
	a.duelEnded = false
	a.duelNoticeAt = time.Time{}
	a.board = newBoardView()
	a.haruhikageAlert = false
	a.quote = PickQuote(a.memberID, "start")
	m := FindMember(a.memberID)
	if initial {
		a.pushChat("MyGO!!!!! 排练室", "#55B2D6", fmt.Sprintf("曲目加载：《%s》(%s)", diff.Song, diff.Subtitle))
	} else {
		a.pushChat("排练重开", "#55B2D6", fmt.Sprintf("重新演奏《%s》", diff.Song))
	}
	if diff.ID == "duel" {
		a.pushChat("MyGO!!!!! 决斗室", "#55B2D6", "五分钟百万格抢地：开格 +1，踩雷 −100；旗帜终局按对错计分，不靠比分偷看雷。")
	}
	a.pushChat(m.Name, m.ColorHex, a.quote)
}

func (a *minesweeperApp) handleAction(c *ui.Context, res ActionResult, now time.Time) {
	if res.Kind == ActionNoop {
		return
	}
	if a.game.Status == StatusPlaying && a.game.duel == nil && a.startedAt.IsZero() {
		a.startedAt = now
	}

	m := FindMember(a.memberID)
	if res.QuoteEvent != "" {
		a.quote = PickQuote(a.memberID, res.QuoteEvent)
		a.pushChat(m.Name, m.ColorHex, a.quote)
	}

	switch res.Kind {
	case ActionShielded:
		c.Toast("长崎爽世发动了「温柔的微笑」：这次就当没点过吧！")
	case ActionSkill:
		c.Toast(fmt.Sprintf("%s 发动「%s」", m.Name, m.SkillName))
	case ActionLost:
		if a.game.duel != nil {
			c.Toast("踩雷 −100 · 硬直 2.4 秒！别把春日影点出来")
			break
		}
		if !a.startedAt.IsZero() {
			a.elapsed = int(now.Sub(a.startedAt).Seconds())
		}
		if a.game.Difficulty.ID == "hard" {
			a.haruhikageAlert = true
		} else {
			c.ToastAction("炸团危机！……要假装无事发生重开吗？", "一辈子重开", func() {
				a.resetGame(false)
			})
		}
	case ActionWon:
		if a.game.duel != nil {
			break
		}
		if !a.startedAt.IsZero() {
			a.elapsed = int(now.Sub(a.startedAt).Seconds())
		}
		diffID := a.game.Difficulty.ID
		if prev, ok := a.bestTimes[diffID]; !ok || a.elapsed < prev {
			a.bestTimes[diffID] = a.elapsed
		}
		c.Toast(fmt.Sprintf("《%s》Live 大成功！用时 %ds", a.game.Difficulty.Song, a.elapsed))
	}

	for _, memeID := range res.NewMemes {
		if !a.unlockedMemes[memeID] {
			a.unlockedMemes[memeID] = true
			for _, meme := range AllMemes {
				if meme.ID == memeID {
					c.Toast("解锁名场面：「" + meme.Title + "」")
					a.pushChat("名场面解锁", "#E8A849", "★ "+meme.Title+" — "+meme.Desc)
					break
				}
			}
		}
	}
}

func formatDigits(n int) string {
	if n < 0 {
		return fmt.Sprintf("−%d", -n)
	}
	return fmt.Sprintf("%d", n)
}

func formatClock(seconds int) string {
	return fmt.Sprintf("%02d:%02d", max(0, seconds)/60, max(0, seconds)%60)
}

func numberColor(n int) ui.Color {
	switch n {
	case 1:
		return ui.Hex("#67B7E1") // 高松灯
	case 2:
		return ui.Hex("#68C178") // 要乐奈
	case 3:
		return ui.Hex("#F07AA4") // 千早爱音
	case 4:
		return ui.Hex("#A78BFA") // 椎名立希
	case 5:
		return ui.Hex("#F5B041") // 长崎爽世
	case 6:
		return ui.Hex("#22D3EE")
	case 7:
		return ui.Hex("#FB7185")
	default:
		return ui.Hex("#CBD5E1")
	}
}

func (a *minesweeperApp) activeTheme() *ui.Theme {
	mygoTeal := ui.Hex("#55B2D6")
	if a.darkMode {
		t := ui.DarkTheme().WithAccent(mygoTeal)
		t.Background = ui.Hex("#0B101B")
		t.Surface = ui.Hex("#131C2E")
		t.SurfaceHover = ui.Hex("#1C2942")
		t.SurfacePressed = ui.Hex("#233352")
		t.Border = ui.Hex("#273754")
		t.Text = ui.Hex("#EDF4FB")
		t.TextMuted = ui.Hex("#93A7C2")
		return t
	}
	t := ui.LightTheme().WithAccent(ui.Hex("#2980A8"))
	return t
}

func (a *minesweeperApp) tickDuel(c *ui.Context, now time.Time) {
	d := a.game.duel
	if d == nil {
		return
	}
	if !a.lastTick.IsZero() && a.game.Status == StatusPlaying {
		tick := a.game.TickDuel(now.Sub(a.lastTick).Seconds())
		a.elapsed = int(d.Elapsed)
		if tick.AITurn != nil && tick.AITurn.Kind != ActionNoop {
			if tick.AITurn.RevealedCount > 0 {
				a.board.LastEffect = now
				a.board.EffectCell = tick.AICell
			}
			if tick.AITurn.Kind == ActionLost {
				c.Toast("高松灯 (AI) 踩雷 −100")
			}
		}
		if tick.Finished {
			a.elapsed = int(d.Duration)
			a.duelEnded = true
			result := "平局！这一百万格的迷茫，恰好一人一半"
			if d.PlayerScore > d.AIScore {
				result = fmt.Sprintf("决斗胜利！%d : %d · 赢了就是赢了！", d.PlayerScore, d.AIScore)
				a.unlockedMemes[MemeForever] = true
			} else if d.PlayerScore < d.AIScore {
				result = fmt.Sprintf("决斗惜败 %d : %d · 这局剪掉不准发到 SNS 上！", d.PlayerScore, d.AIScore)
			}
			a.pushChat("决斗结算", "#E8A849", result)
			c.ToastAction(result, "再战一局", func() { a.resetGame(false) })
		}
	}
	if a.game.Status == StatusWon && !a.duelEnded {
		a.duelEnded = true
		result := fmt.Sprintf("清盘决斗结算：你 %d · 灯 (AI) %d", d.PlayerScore, d.AIScore)
		a.pushChat("决斗结算", "#E8A849", result)
		c.ToastAction(result, "再战一局", func() { a.resetGame(false) })
	}
	a.lastTick = now
	if a.game.Status == StatusPlaying {
		c.After(50 * time.Millisecond)
	}
	if a.game.Status == StatusPlaying && now.Sub(a.duelNoticeAt) > 10*time.Second {
		event := ""
		if d.PlayerScore > d.AIScore {
			event = "duelLead"
		} else if d.PlayerScore < d.AIScore {
			event = "duelBehind"
		}
		if speaker, text := pickDuelLine(a.memberID, event); speaker != "" {
			a.pushChat(speaker, FindMember(a.memberID).ColorHex, text)
		}
		a.duelNoticeAt = now
	}
}

func (a *minesweeperApp) duelHUD(c *ui.Context) {
	d := a.game.duel
	ui.Row(c).Grow(0).Padding(8, 10).Gap(12).AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Badge(c, fmt.Sprintf("你 %d", d.PlayerScore)).Background(ui.Hex("#173B49")).TextColor(ui.Hex("#55B2D6"))
			ui.Badge(c, fmt.Sprintf("灯 (AI) %d", d.AIScore)).Background(ui.Hex("#3B2740")).TextColor(ui.Hex("#F07AA4"))
			ui.Badge(c, "剩余 "+formatClock(int(d.remaining()))).Background(c.Theme().Accent.Alpha(0.18))
		})
		ui.Row(c).Gap(6).Children(func() {
			if ui.Button(c.Key("duel-home"), "回到战场").Clicked() {
				a.board.center(a.game, 500, 500)
			}
			if ui.Button(c.Key("duel-follow"), "追踪 AI").Clicked() && d.LastAI >= 0 {
				a.board.center(a.game, d.LastAI/a.game.Cols, d.LastAI%a.game.Cols)
			}
			if ui.Button(c.Key("duel-zoom-out"), "−").Label("缩小").Clicked() {
				a.board.zoom(a.game, a.board.Width/2, a.board.Height/2, 1/1.2)
			}
			if ui.Button(c.Key("duel-zoom-in"), "+").Label("放大").Clicked() {
				a.board.zoom(a.game, a.board.Width/2, a.board.Height/2, 1.2)
			}
		})
	})
	ui.Row(c).Padding(4, 8).Gap(8).Children(func() {
		ui.Badge(c, fmt.Sprintf("连击 ×%d", d.PlayerCombo))
		if d.playerStunned() {
			ui.Badge(c, fmt.Sprintf("硬直 %.1fs", d.StunnedUntil-d.Elapsed)).Background(ui.Hex("#701F2B"))
		} else {
			ui.Badge(c, "数字全未知时左键收雷 · +75/雷")
		}
		if d.Elapsed < d.ScanUntil {
			ui.Badge(c, fmt.Sprintf("扫描 %.1fs", d.ScanUntil-d.Elapsed)).Background(ui.Hex("#3C301D")).TextColor(ui.Hex("#F5D99C"))
		}
		ui.Badge(c, fmt.Sprintf("旗帜 %d", a.game.FlagsPlaced))
	})
	if a.game.Status == StatusWon {
		label := "平局 · 势均力敌"
		if d.PlayerScore > d.AIScore {
			label = "险胜也是爱音大胜利"
		} else if d.PlayerScore < d.AIScore {
			label = "惜败 · 这局别发 SNS"
		}
		ui.Row(c).Padding(4, 8).Children(func() {
			ui.Badge(c, label).Background(c.Theme().Accent.Alpha(0.22))
		})
	}
}

func (a *minesweeperApp) view(c *ui.Context) {
	now := c.Now()
	if a.pendingResult.Kind != ActionNoop {
		res := a.pendingResult
		a.pendingResult = ActionResult{}
		a.handleAction(c, res, now)
	}
	if a.game.duel != nil {
		a.tickDuel(c, now)
	} else if a.game.Status == StatusPlaying {
		if a.startedAt.IsZero() {
			a.startedAt = now
		}
		a.elapsed = int(now.Sub(a.startedAt).Seconds())
		nextTick := a.startedAt.Add(time.Duration(a.elapsed+1) * time.Second).Sub(now)
		c.After(max(16*time.Millisecond, nextTick))
	}

	if c.Shortcut(0, ui.KeyR) {
		a.resetGame(false)
	}
	if c.Shortcut(0, ui.KeyF) {
		a.flagMode = !a.flagMode
	}
	if c.Shortcut(0, ui.KeyE) {
		res := a.game.UseSkill()
		a.handleAction(c, res, now)
	}
	if a.game.duel != nil {
		if c.Shortcut(ui.Ctrl, ui.KeyEqual) {
			a.board.zoom(a.game, a.board.Width/2, a.board.Height/2, 1.15)
		}
		if c.Shortcut(ui.Ctrl, ui.KeyMinus) {
			a.board.zoom(a.game, a.board.Width/2, a.board.Height/2, 1/1.15)
		}
		if c.Shortcut(ui.Ctrl, ui.Key0) {
			a.board.center(a.game, 500, 500)
		}
	}

	baseTheme := a.activeTheme()
	c.SetTheme(baseTheme)
	t := c.Theme()

	member := FindMember(a.memberID)
	memberColor := ui.Hex(member.ColorHex)
	memberTheme := baseTheme.WithAccent(memberColor)

	// 春日影名场面弹窗
	switch ui.AlertDialog(c, &a.haruhikageAlert,
		"为什么要演奏春日影？！",
		"「明明都约好了不点那一格的……」\n（长崎爽世失去了高光，并把你移出了排练室群聊）",
		"对不起，我错了", "一辈子重开") {
	case 0:
		a.pushChat("长崎爽世", "#E8A849", "「道歉的话，下次别再盲猜了呢。」")
	case 1:
		a.resetGame(false)
	}

	ui.Column(c).Fill().Padding(14).Gap(10).Children(func() {
		// Top Header Bar
		ui.Row(c).AlignItems(ui.Center).Justify(ui.SpaceBetween).Gap(12).Children(func() {
			ui.Row(c).AlignItems(ui.Center).Gap(10).Children(func() {
				ui.Avatar(c, "MyGO!!!!!", nil)
				ui.Column(c).Gap(2).Children(func() {
					ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
						ui.Text(c, "MyGO!!!!! 迷子扫雷").FontSize(20).Bold()
						ui.Badge(c, fmt.Sprintf("《%s》", a.game.Difficulty.Song)).
							Background(t.Accent.Alpha(0.18)).TextColor(t.Accent)
						ui.Badge(c, fmt.Sprintf("名场面 %d/%d", len(a.unlockedMemes), len(AllMemes)))
					})
					ui.Text(c, "「迷子でもいい、迷子でも進め。」 · egoist/mygo Native UI").
						FontSize(12).TextColor(t.TextMuted)
				})
			})

			ui.Row(c).AlignItems(ui.Center).Gap(10).Children(func() {
				if ui.Segmented(c, &a.modeIdx, "乐队 Live", "纯正经典").Label("游戏模式").Disabled(a.currentDiff().ID == "duel").Changed() {
					a.resetGame(false)
				}
				ui.Row(c).AlignItems(ui.Center).Gap(6).Children(func() {
					ui.Text(c, "夜间").FontSize(12).TextColor(t.TextMuted)
					ui.Switch(c, &a.darkMode).Label("夜间模式")
				})
			})
		})

		// Setlist & Toolbar Row
		ui.Row(c).Padding(8, 12).Radius(10).Background(t.Surface).Border(1, t.Border).
			AlignItems(ui.Center).Justify(ui.SpaceBetween).Gap(10).Wrap().Children(func() {
			if ui.Segmented(c, &a.diffIdx,
				Difficulties[0].Label,
				Difficulties[1].Label,
				Difficulties[2].Label,
				Difficulties[3].Label,
				Difficulties[4].Label,
			).Label("排练曲目难度").Changed() {
				a.resetGame(false)
			}

			ui.ToggleGroup(c, func() {
				ui.Toggle(c, &a.flagMode, "标旗模式 (F)").Tooltip("开启后左键直接贴创可贴/插旗")
				ui.Toggle(c, &a.allowQuestion, "? 标记").Tooltip("右键循环加入「？」不确定标记")
				ui.Toggle(c, &a.showMemes, "名场面图鉴").Tooltip("展开或收起已解锁的 MyGO!!!!! 梗成就")
			}).Label("工具开关")
		})

		// Main 2-Column Workspace: Left Sidebar + Right Stage
		ui.Row(c).Grow(1).Gap(12).AlignItems(ui.Stretch).Children(func() {
			// Left Sidebar: Member Roster + Live Meters + Band Group Chat
			ui.Column(c).Width(272).Padding(10).Gap(10).Radius(12).
				Background(t.Surface).Border(1, t.Border).Children(func() {
				ui.Text(c, "乐队成员应援").FontSize(13).Bold().TextColor(t.TextMuted)

				ui.Column(c).Gap(6).Children(func() {
					for _, m := range Members {
						mc := ui.Hex(m.ColorHex)
						selected := m.ID == a.memberID
						item := ui.Box(c.Key(string(m.ID))).Padding(7, 9).Radius(9).
							Cursor(ui.CursorPointer).Label(m.Name).Tooltip(m.SkillDesc)
						if selected {
							item.Background(mc.Alpha(0.18)).Border(1, mc)
						} else if item.Hovered() {
							item.Background(t.SurfaceHover).Border(1, mc.Alpha(0.5))
						} else {
							item.Background(t.Background.Alpha(0.55)).Border(1, t.Border)
						}
						item.Children(func() {
							ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
								ui.Avatar(c, m.AvatarKey, nil)
								ui.Column(c).Grow(1).Gap(1).Children(func() {
									ui.Row(c).AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
										ui.Textf(c, "%s · %s", m.Name, m.Role).FontSize(13).Bold()
										if selected && a.bandMode() {
											tag := fmt.Sprintf("×%d", a.game.SkillUsesLeft)
											if m.Passive {
												tag = fmt.Sprintf("盾×%d", a.game.SkillUsesLeft)
											}
											ui.Badge(c, tag).Background(mc.Alpha(0.24)).TextColor(mc)
										}
									})
									ui.Text(c, m.SkillName+" · "+m.Tagline).FontSize(11).TextColor(t.TextMuted).SingleLine()
								})
							})
						})
						if item.Clicked() && !selected {
							prev := a.memberID
							a.memberID = m.ID
							if a.game.Status != StatusIdle {
								a.resetGame(false)
							}
							a.game.MemberID = m.ID
							a.quote = PickQuote(m.ID, "start")
							if prev == MemberTomori && m.ID == MemberTaki {
								a.quote = "「灯刚才还在看呢，你给我认真点排雷！」"
							}
							a.pushChat(m.Name, m.ColorHex, a.quote)
							c.Toast("已切换领队：" + m.Name)
						}
					}
				})

				// Progress & Gravity Meter
				ui.Column(c).Gap(6).Padding(8).Radius(9).Background(t.Background.Alpha(0.6)).Border(1, t.Border).Children(func() {
					ui.Row(c).Justify(ui.SpaceBetween).Children(func() {
						ui.Text(c, "排练完成度").FontSize(11).TextColor(t.TextMuted)
						ui.Textf(c, "%.0f%%", a.game.Progress()*100).FontSize(11).Bold().TextColor(t.Accent)
					})
					ui.Progress(c, a.game.Progress()).Label("Live 完成度")

					ui.Row(c).Justify(ui.SpaceBetween).Children(func() {
						ui.Text(c, "乐队重力值 (Gravity)").FontSize(11).TextColor(t.TextMuted)
						ui.Textf(c, "%d%%", a.game.Gravity).FontSize(11).Bold().TextColor(ui.Hex("#E8A849"))
					})
					ui.Meter(c, float64(a.game.Gravity), 0, 100, &ui.MeterLevels{
						Warning:  55,
						Critical: 80,
					}).Label("乐队重力值")
				})

				// Band Group Chat
				ui.Text(c, "MyGO!!!!! 排练室群聊").FontSize(12).Bold().TextColor(t.TextMuted)
				ui.Scroll(c).Grow(1).Padding(6).Radius(9).Background(t.Background.Alpha(0.6)).Border(1, t.Border).Children(func() {
					ui.Column(c).Gap(6).Children(func() {
						for idx := len(a.chatLog) - 1; idx >= 0; idx-- {
							msg := a.chatLog[idx]
							ui.Column(c.Key(idx)).Gap(1).Padding(5, 7).Radius(7).Background(t.Surface.Alpha(0.75)).Children(func() {
								ui.Text(c, msg.Speaker).FontSize(11).Bold().TextColor(ui.Hex(msg.ColorHex))
								ui.Text(c, msg.Text).FontSize(12)
							})
						}
					})
				})
			})

			// Right Main Stage: Companion Banner + HUD + Board (+ Optional Memes Drawer)
			ui.Column(c).Grow(1).Gap(10).Children(func() {
				// Active Member Banner themed to the selected member!
				ui.Themed(c, memberTheme).Padding(10, 12).Radius(12).Background(t.Surface).
					Border(1, memberColor.Alpha(0.55)).Children(func() {
					ui.Row(c).AlignItems(ui.Center).Justify(ui.SpaceBetween).Gap(12).Children(func() {
						ui.Row(c).Grow(1).AlignItems(ui.Center).Gap(10).Children(func() {
							ui.Avatar(c, member.AvatarKey, nil)
							ui.Column(c).Grow(1).Gap(2).Children(func() {
								ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
									ui.Text(c, member.Name).FontSize(14).Bold().TextColor(memberColor)
									ui.Badge(c, member.Role)
									skillTip := member.SkillName + " · " + member.SkillDesc
									if !a.bandMode() {
										skillTip = "纯正经典模式：仅保留语音应援"
									}
									ui.Text(c, skillTip).FontSize(12).TextColor(t.TextMuted).SingleLine()
								})
								ui.Text(c, a.quote).FontSize(13).Bold()
							})
						})

						skillLabel := fmt.Sprintf("发动技能 (E) ×%d", a.game.SkillUsesLeft)
						canUse := a.bandMode() && !member.Passive && a.game.Status == StatusPlaying && a.game.SkillUsesLeft > 0
						if !a.bandMode() {
							skillLabel = "经典模式"
						} else if member.Passive {
							if a.game.SkillUsesLeft > 0 {
								skillLabel = fmt.Sprintf("微笑守护中 ×%d", a.game.SkillUsesLeft)
							} else {
								skillLabel = "护盾已耗尽"
							}
						}
						if ui.PrimaryButton(c.Key("skill-btn"), skillLabel).Disabled(!canUse).
							Tooltip(member.SkillDesc).Clicked() {
							res := a.game.UseSkill()
							a.handleAction(c, res, now)
						}
					})
				})

				// Collapsible Meme Achievements Panel
				if a.showMemes {
					ui.Column(c).Padding(10, 12).Gap(8).Radius(12).Background(t.Surface).Border(1, t.Border).Children(func() {
						ui.Row(c).AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
							ui.Text(c, "MyGO!!!!! 名场面梗图鉴").FontSize(13).Bold()
							ui.Badge(c, fmt.Sprintf("已收集 %d / %d", len(a.unlockedMemes), len(AllMemes)))
						})
						ui.Grid(c).Columns(3).Gap(8).Children(func() {
							for _, meme := range AllMemes {
								unlocked := a.unlockedMemes[meme.ID]
								card := ui.Column(c.Key(meme.ID)).Padding(8).Gap(3).Radius(8).Border(1, t.Border)
								if unlocked {
									card.Background(t.Accent.Alpha(0.14)).Border(1, t.Accent.Alpha(0.6))
								} else {
									card.Background(t.Background.Alpha(0.5))
								}
								card.Children(func() {
									ui.Row(c).AlignItems(ui.Center).Justify(ui.SpaceBetween).Gap(4).Children(func() {
										ui.Text(c, meme.Title).FontSize(12).Bold().SingleLine()
										if unlocked {
											ui.Badge(c, "已解锁").Background(t.Accent).TextColor(t.AccentText)
										} else {
											ui.Badge(c, "未解锁")
										}
									})
									ui.Text(c, meme.Desc).FontSize(11).TextColor(t.TextMuted)
								})
							}
						})
					})
				}

				// Board Stage
				ui.Column(c).Grow(1).Padding(12).Gap(10).Radius(12).Background(t.Surface).Border(1, t.Border).Children(func() {
					// HUD Bar
					ui.Row(c).Padding(8, 12).Radius(9).Background(t.Background.Alpha(0.75)).
						Border(1, t.Border).AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
						ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
							ui.Badge(c, "MINES "+formatDigits(a.game.RemainingMines())).
								Background(t.Accent.Alpha(0.18)).TextColor(t.Accent).FontFeatures("tnum")
							ui.Badge(c, a.game.Difficulty.Subtitle)
						})

						resetLabel := "迷子出发 (R)"
						switch a.game.Status {
						case StatusPlaying:
							resetLabel = "演奏中 · 重开 (R)"
						case StatusWon:
							resetLabel = "Live 大成功！再来一首"
							if a.game.duel != nil {
								resetLabel = "决斗结束 · 再来一局 (R)"
							}
						case StatusLost:
							resetLabel = "炸团了 · 再排练一次"
						}
						if ui.Button(c.Key("reset-btn"), resetLabel).Clicked() {
							a.resetGame(false)
						}

						ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
							bestText := "BEST --"
							if best, ok := a.bestTimes[a.game.Difficulty.ID]; ok {
								bestText = fmt.Sprintf("BEST %ds", best)
							}
							ui.Badge(c, bestText)
							ui.Badge(c, "TIME "+formatDigits(a.elapsed)+"s").
								Background(t.Accent.Alpha(0.18)).TextColor(t.Accent).FontFeatures("tnum")
						})
					})

					if a.game.duel != nil {
						a.duelHUD(c)
						a.duelBoard(c)
						return
					}
					// Minesweeper Grid
					cellSize := float32(40)
					cellFont := float32(15)
					if a.game.Cols >= 28 {
						cellSize = 32
						cellFont = 15
					} else if a.game.Cols >= 18 {
						cellSize = 36
						cellFont = 16
					} else if a.game.Cols >= 16 {
						cellSize = 38
						cellFont = 17
					}

					availableW, availableH := c.Size()
					byWidth := (availableW - 70 - float32(a.game.Cols-1)*3) / float32(a.game.Cols)
					byHeight := (availableH - 300 - float32(a.game.Rows-1)*3) / float32(a.game.Rows)
					if a.game.duel == nil {
						byWidth = (availableW - 342 - float32(a.game.Cols-1)*3) / float32(a.game.Cols)
						byHeight = (availableH - 230 - float32(a.game.Rows-1)*3) / float32(a.game.Rows)
					}
					cellSize = max(22, min(64, min(byWidth, byHeight)))
					cellFont = max(12, cellSize*0.48)
					stage := ui.ScrollBoth(c).Grow(1).Draw(func(p *ui.Painter, bounds ui.Rect) {
						byWidth := (bounds.W - 28 - float32(a.game.Cols-1)*3) / float32(a.game.Cols)
						byHeight := (bounds.H - 28 - float32(a.game.Rows-1)*3) / float32(a.game.Rows)
						size := max(22, min(64, min(byWidth, byHeight)))
						if size != a.classicSize {
							a.classicSize = size
							p.After(time.Millisecond)
						}
					})
					if a.classicSize > 0 {
						cellSize = a.classicSize
						cellFont = max(12, cellSize*0.48)
					}
					stage.Children(func() {
						ui.Column(c).FillWidth().AlignItems(ui.Center).Padding(4).Children(func() {
							ui.Grid(c).Columns(a.game.Cols).Gap(3).Padding(8).Radius(10).
								Background(t.Background).Border(1, t.Border).Children(func() {
								for i := range a.game.Cells {
									cell := &a.game.Cells[i]
									row, col := cell.Row, cell.Col
									tile := ui.Box(c.Key(i)).Size(cellSize, cellSize).Radius(5).Center().
										Label(fmt.Sprintf("cell-%d-%d", row, col))

									switch {
									case cell.Revealed:
										switch {
										case cell.Exploded:
											tile.Background(ui.Hex("#991B1B")).Border(1, ui.Hex("#FCA5A5"))
											tile.Children(func() {
												ui.Text(c, "💥").FontSize(cellFont - 1)
											})
										case cell.IsMine:
											tile.Background(t.Background).Border(1, ui.Hex("#EF4444").Alpha(0.55))
											tile.Children(func() {
												ui.Text(c, "💣").FontSize(cellFont - 1)
											})
										case cell.Adjacent > 0:
											bg := t.Background
											if tile.Hovered() {
												bg = t.SurfaceHover.Alpha(0.45)
											}
											tile.Background(bg).Border(1, t.Border.Alpha(0.5)).Cursor(ui.CursorPointer)
											tile.Children(func() {
												ui.Textf(c, "%d", cell.Adjacent).FontSize(cellFont).Bold().
													TextColor(numberColor(cell.Adjacent))
											})
										default:
											tile.Background(t.Background).Border(1, t.Border.Alpha(0.32))
										}
									case cell.WrongFlag:
										tile.Background(ui.Hex("#4C1D2D")).Border(1, ui.Hex("#FB7185"))
										tile.Children(func() {
											ui.Text(c, "✕").FontSize(cellFont - 1).Bold().TextColor(ui.Hex("#FB7185"))
										})
									case cell.Flagged:
										bd := t.Accent.Alpha(0.75)
										mark := "🩹"
										if cell.ShieldSaved {
											bd = ui.Hex("#E8A849")
											mark = "🛡"
										}
										tile.Background(t.SurfaceHover).Border(1, bd).Cursor(ui.CursorPointer)
										tile.Children(func() {
											ui.Text(c, mark).FontSize(cellFont - 1)
										})
									case cell.Questioned:
										tile.Background(t.SurfaceHover).Border(1, t.Accent.Alpha(0.55)).Cursor(ui.CursorPointer)
										tile.Children(func() {
											ui.Text(c, "?").FontSize(cellFont).Bold().TextColor(t.Accent)
										})
									default:
										bg := t.Surface
										bd := t.Border
										if tile.Hovered() {
											bg = t.SurfaceHover
											bd = t.Accent.Alpha(0.75)
										}
										tile.Background(bg).Border(1, bd).Cursor(ui.CursorPointer)
									}

									if tile.RightClicked() {
										res := a.game.ToggleFlag(row, col, a.allowQuestion)
										a.handleAction(c, res, now)
									} else if tile.Clicked() {
										if cell.Revealed {
											res := a.game.Chord(row, col)
											a.handleAction(c, res, now)
										} else if a.flagMode {
											res := a.game.ToggleFlag(row, col, a.allowQuestion)
											a.handleAction(c, res, now)
										} else {
											res := a.game.Reveal(row, col)
											a.handleAction(c, res, now)
										}
									}
								}
							})
						})
					})
				})
			})
		})
	})
}

func main() {
	app := newMinesweeperApp()
	mygo.App.WhenReady(func() {
		mygo.NewWindow(mygo.WindowOptions{
			Title:           "MyGO!!!!! 迷子扫雷",
			Width:           1280,
			Height:          900,
			BackgroundColor: "#0b101b",
			StateKey:        "mygo-minesweeper-v2",
			Content:         ui.View(app.view),
		})
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
