package main

import (
	_ "embed"
	"fmt"
	"log"
	"math"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

//go:embed resources/icon.png
var appIconPNG []byte

const (
	rippleStepDelay         = 24 * time.Millisecond
	rippleMaxDelay          = 240 * time.Millisecond
	rippleDuration          = 280 * time.Millisecond
	duelPopupDuration       = 950 * time.Millisecond
	maxDuelPopups           = 16
	haruhikageIntroDuration = 1600 * time.Millisecond
)

type chatMessage struct {
	Speaker  string
	ColorHex string
	Text     string
}

type tileRipple struct {
	Start time.Time
	Delay time.Duration
}

type duelPopup struct {
	Row      int
	Col      int
	Text     string
	ColorHex string
	Start    time.Time
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
	haruhikageIntro bool

	game                 *Game
	quote                string
	startedAt            time.Time
	elapsed              int
	bestTimes            map[string]int
	unlockedMemes        map[string]bool
	chatLog              []chatMessage
	seed                 uint64
	board                *boardView
	lastTick             time.Time
	pendingResult        ActionResult
	pendingRow           int
	pendingCol           int
	duelNoticeAt         time.Time
	duelEnded            bool
	classicSize          float32
	ripples              map[int]tileRipple
	duelPopups           []duelPopup
	shakeStart           time.Time
	shakeAmplitude       float32
	shakeDuration        time.Duration
	haruhikageIntroStart time.Time
	classicHoverRow      int
	classicHoverCol      int
	classicHoverHint     HoverHint
}

func newMinesweeperApp(seed ...uint64) *minesweeperApp {
	var s uint64
	if len(seed) > 0 {
		s = seed[0]
	}
	app := &minesweeperApp{
		diffIdx:         0,
		modeIdx:         0,
		darkMode:        true,
		memberID:        MemberTomori,
		bestTimes:       make(map[string]int),
		unlockedMemes:   make(map[string]bool),
		seed:            s,
		board:           newBoardView(),
		pendingRow:      -1,
		pendingCol:      -1,
		classicHoverRow: -1,
		classicHoverCol: -1,
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

func (a *minesweeperApp) isDuel() bool {
	return a.game != nil && a.game.Difficulty.Gameplay == GameplayDuel && a.game.duel != nil
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
	if diff.Gameplay == GameplayDuel {
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
	a.pendingResult = ActionResult{}
	a.pendingRow = -1
	a.pendingCol = -1
	a.duelEnded = false
	a.duelNoticeAt = time.Time{}
	a.board = newBoardView()
	a.ripples = nil
	a.duelPopups = nil
	a.shakeStart = time.Time{}
	a.shakeAmplitude = 0
	a.shakeDuration = 0
	a.haruhikageAlert = false
	a.haruhikageIntro = false
	a.haruhikageIntroStart = time.Time{}
	a.classicHoverRow = -1
	a.classicHoverCol = -1
	a.classicHoverHint = HoverHint{}
	a.quote = PickQuote(a.memberID, "start")
	m := FindMember(a.memberID)
	if initial {
		a.pushChat("MyGO!!!!! 排练室", hexOf(bandColor), fmt.Sprintf("曲目加载：《%s》(%s)", diff.Song, diff.Subtitle))
	} else {
		a.pushChat("排练重开", hexOf(bandColor), fmt.Sprintf("重新演奏《%s》", diff.Song))
	}
	if diff.Gameplay == GameplayDuel {
		a.pushChat("MyGO!!!!! 决斗室", hexOf(bandColor), "五分钟百万格抢地：开格 +1，踩雷 −100；旗帜终局按对错计分，不靠比分偷看雷。")
	}
	a.pushChat(m.Name, m.ColorHex, a.quote)
}

func (a *minesweeperApp) enqueueRipple(now time.Time, wave []RevealWaveEntry) {
	if len(wave) == 0 {
		return
	}
	a.pruneRipples(now)
	if a.ripples == nil {
		a.ripples = make(map[int]tileRipple, len(wave))
	}
	for _, entry := range wave {
		if len(a.ripples) >= MaxRevealWaveEntries {
			break
		}
		delay := min(rippleMaxDelay, time.Duration(entry.Depth)*rippleStepDelay)
		a.ripples[entry.Index] = tileRipple{Start: now, Delay: delay}
	}
}

func (a *minesweeperApp) pruneRipples(now time.Time) {
	for idx, rip := range a.ripples {
		elapsed := now.Sub(rip.Start)
		if elapsed < 0 || elapsed >= rip.Delay+rippleDuration {
			delete(a.ripples, idx)
		}
	}
}

func (a *minesweeperApp) spawnDuelPopup(row, col int, res ActionResult, now time.Time) {
	if !a.isDuel() || res.ScoreDelta == 0 {
		return
	}
	shouldShow := res.Kind == ActionFlagged || res.Kind == ActionLost || res.Kind == ActionSkill || res.RevealedCount >= 5 || res.ScoreDelta >= 10
	if !shouldShow {
		return
	}
	var text, colorHex string
	switch {
	case res.Kind == ActionFlagged:
		text = fmt.Sprintf("+%d 收雷!", res.ScoreDelta)
		colorHex = "#E8A849"
	case res.Kind == ActionLost && res.ScoreDelta < 0:
		text = fmt.Sprintf("%d 踩雷!", res.ScoreDelta)
		colorHex = "#FB7185"
	case res.RevealedCount >= 8 || a.game.duel.PlayerCombo >= 3:
		text = fmt.Sprintf("+%d · 连击×%d", res.ScoreDelta, a.game.duel.PlayerCombo)
		colorHex = "#38BDF8"
	default:
		text = fmt.Sprintf("+%d", res.ScoreDelta)
		colorHex = "#55B2D6"
	}
	kept := a.duelPopups[:0]
	for _, p := range a.duelPopups {
		if now.Sub(p.Start) >= 0 && now.Sub(p.Start) < duelPopupDuration {
			kept = append(kept, p)
		}
	}
	kept = append(kept, duelPopup{Row: row, Col: col, Text: text, ColorHex: colorHex, Start: now})
	if len(kept) > maxDuelPopups {
		kept = kept[len(kept)-maxDuelPopups:]
	}
	a.duelPopups = kept
}

func (a *minesweeperApp) triggerShake(now time.Time, amp float32, dur time.Duration) {
	a.shakeStart = now
	a.shakeAmplitude = amp
	a.shakeDuration = dur
}

func (a *minesweeperApp) currentShakeOffset(now time.Time) (float32, float32) {
	if a.shakeAmplitude <= 0 || a.shakeStart.IsZero() || a.shakeDuration <= 0 {
		return 0, 0
	}
	elapsed := now.Sub(a.shakeStart)
	if elapsed < 0 || elapsed >= a.shakeDuration {
		a.shakeAmplitude = 0
		return 0, 0
	}
	progress := float64(elapsed) / float64(a.shakeDuration)
	decay := math.Exp(-4.5*progress) * (1 - progress)
	angle := progress * 2 * math.Pi * 4.5
	dx := float32(math.Sin(angle) * decay * float64(a.shakeAmplitude))
	dy := float32(math.Cos(angle*1.3) * decay * float64(a.shakeAmplitude) * 0.65)
	return dx, dy
}

func (a *minesweeperApp) handleAction(c *ui.Context, res ActionResult, now time.Time) {
	if res.Kind == ActionNoop {
		return
	}
	if a.game.Status == StatusPlaying && !a.isDuel() && a.startedAt.IsZero() {
		a.startedAt = now
	}
	a.enqueueRipple(now, res.RevealWave)
	if a.isDuel() {
		popRow, popCol := a.pendingRow, a.pendingCol
		if popRow < 0 || popCol < 0 {
			if a.game.duel.LastCell >= 0 {
				popRow, popCol = a.game.duel.LastCell/a.game.Cols, a.game.duel.LastCell%a.game.Cols
			} else if a.board.HoverRow >= 0 && a.board.HoverCol >= 0 {
				popRow, popCol = a.board.HoverRow, a.board.HoverCol
			} else {
				popRow, popCol = a.game.Rows/2, a.game.Cols/2
			}
		}
		a.spawnDuelPopup(popRow, popCol, res, now)
	}
	a.pendingRow, a.pendingCol = -1, -1

	m := FindMember(a.memberID)
	if res.QuoteEvent != "" {
		a.quote = PickQuote(a.memberID, res.QuoteEvent)
		a.pushChat(m.Name, m.ColorHex, a.quote)
	}

	switch res.Kind {
	case ActionShielded:
		a.triggerShake(now, 4.5, 220*time.Millisecond)
		c.Toast("长崎爽世发动了「温柔的微笑」：这次就当没点过吧！")
	case ActionSkill:
		c.Toast(fmt.Sprintf("%s 发动「%s」", m.Name, m.SkillName))
	case ActionLost:
		a.triggerShake(now, 9.0, 320*time.Millisecond)
		if a.isDuel() {
			c.Toast("踩雷 −100 · 硬直 2.4 秒！别把春日影点出来")
			break
		}
		if !a.startedAt.IsZero() {
			a.elapsed = int(now.Sub(a.startedAt).Seconds())
		}
		if a.game.Difficulty.Haruhikage {
			a.haruhikageIntro = true
			a.haruhikageIntroStart = now
		} else {
			c.ToastAction("炸团危机！……要假装无事发生重开吗？", "一辈子重开", func() {
				a.resetGame(false)
			})
		}
	case ActionWon:
		if a.isDuel() {
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

func (a *minesweeperApp) paper() *palette { return paperFor(a.darkMode) }

// activeTheme is the notebook paper as a widget theme: the desk behind,
// sheets for panels, ink for text, the 乐队色 as accent, 文楷 throughout.
func (a *minesweeperApp) activeTheme() *ui.Theme {
	p := a.paper()
	var t *ui.Theme
	if p.Dark {
		t = ui.DarkTheme().WithAccent(p.InkOf(bandColor))
	} else {
		t = ui.LightTheme().WithAccent(bandColor)
	}
	t.Background = p.Desk
	t.Surface = p.Sheet
	t.SurfaceHover = p.Sheet.Mix(p.Ink, 0.05)
	t.SurfacePressed = p.Sheet.Mix(p.Ink, 0.1)
	t.Border = p.Rule
	t.Text = p.Ink
	t.TextMuted = p.InkMuted
	t.Font = fontFamily
	return t
}

func (a *minesweeperApp) tickDuel(c *ui.Context, now time.Time) {
	if !a.isDuel() {
		return
	}
	d := a.game.duel
	if !a.lastTick.IsZero() && a.game.Status == StatusPlaying {
		tick := a.game.TickDuel(now.Sub(a.lastTick).Seconds())
		a.elapsed = int(d.Elapsed)
		if tick.AITurn != nil && tick.AITurn.Kind != ActionNoop {
			a.enqueueRipple(now, tick.AITurn.RevealWave)
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
				a.board.centerBoard(a.game)
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
	if a.isDuel() {
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
	if a.isDuel() {
		if c.Shortcut(ui.Ctrl, ui.KeyEqual) {
			a.board.zoom(a.game, a.board.Width/2, a.board.Height/2, 1.15)
		}
		if c.Shortcut(ui.Ctrl, ui.KeyMinus) {
			a.board.zoom(a.game, a.board.Width/2, a.board.Height/2, 1/1.15)
		}
		if c.Shortcut(ui.Ctrl, ui.Key0) {
			a.board.centerBoard(a.game)
		}
	}

	baseTheme := a.activeTheme()
	c.SetTheme(baseTheme)
	t := c.Theme()

	pal := a.paper()
	art := artFor(pal.Dark)
	member := FindMember(a.memberID)
	// memberColor is the leader's 代表色, used as a highlighter; memberInk
	// is it as ink, for text and the skill button on this paper.
	memberColor := ui.Hex(member.ColorHex)
	memberInk := pal.InkOf(memberColor)
	memberTheme := baseTheme.WithAccent(memberInk)

	if a.haruhikageIntro {
		elapsedIntro := now.Sub(a.haruhikageIntroStart)
		if elapsedIntro >= haruhikageIntroDuration {
			a.haruhikageIntro = false
			a.haruhikageAlert = true
		} else {
			remain := haruhikageIntroDuration - elapsedIntro
			c.After(min(50*time.Millisecond, max(time.Millisecond, remain)))
			ui.Overlay(c, func() {
				overlay := ui.Box(c.Key("haruhikage-drama-overlay")).Fill().Modal().
					Background(ui.Hex("#05070D").Alpha(0.84)).Center().
					Cursor(ui.CursorPointer).Label("春日影演出遮罩")
				overlay.Children(func() {
					ui.Column(c).AlignItems(ui.Center).Gap(10).Padding(24).Radius(14).
						Background(ui.Hex("#140B16").Alpha(0.95)).Border(1, ui.Hex("#E8A849").Alpha(0.8)).
						Children(func() {
							ui.Text(c, "♪ 春日影 (Crychic Ver.) — 演奏中断").FontSize(13).Bold().TextColor(ui.Hex("#E8A849"))
							ui.Text(c, "「……为什么要演奏春日影？！」").FontSize(22).Bold().TextColor(ui.Hex("#F8FAFC"))
							ui.Text(c, "（长崎爽世的眼神失去了高光 · 点击任意处继续）").FontSize(12).TextColor(ui.Hex("#CBD5E1"))
						})
				})
				if overlay.Clicked() {
					a.haruhikageIntro = false
					a.haruhikageAlert = true
				}
			})
		}
	}

	// 春日影名场面弹窗
	switch ui.AlertDialog(c, &a.haruhikageAlert,
		"为什么要演奏春日影？！",
		"「明明都约好了不点那一格的……」\n（长崎爽世失去了高光，并把你移出了排练室群聊）",
		"对不起，我错了", "一辈子重开") {
	case 0:
		a.pushChat("长崎爽世", FindMember(MemberSoyorin).ColorHex, "「道歉的话，下次别再盲猜了呢。」")
	case 1:
		a.resetGame(false)
	}

	root := ui.Column(c).Fill().Padding(14).Gap(10)
	if pal.Lamp.A > 0 {
		// The desk lamp warms the night notebook's top-left corner.
		root.RadialGradient(ui.RadialGradient{
			Stops:  []ui.GradientStop{{Color: pal.Lamp, At: 0}, {Color: pal.Lamp.Alpha(0), At: 1}},
			Center: ui.Point{X: 0.12, Y: 0.02},
			Radius: 720,
		})
	}
	root.Children(func() {
		// Top Header Bar
		ui.Row(c).AlignItems(ui.Center).Justify(ui.SpaceBetween).Gap(12).Children(func() {
			ui.Row(c).AlignItems(ui.Center).Gap(10).Children(func() {
				bandMark(c, 40)
				ui.Column(c).Gap(2).Children(func() {
					ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
						ui.Text(c, "MyGO!!!!! 迷子扫雷").FontSize(21).Bold()
						ui.Badge(c, fmt.Sprintf("《%s》", a.game.Difficulty.Song)).
							Background(t.Accent.Alpha(0.14)).TextColor(t.Accent)
						ui.Badge(c, fmt.Sprintf("名场面 %d/%d", len(a.unlockedMemes), len(AllMemes)))
					})
					ui.Text(c, "「迷子でもいい、迷子でも進め。」").
						FontSize(12).TextColor(t.TextMuted)
				})
			})

			ui.Row(c).AlignItems(ui.Center).Gap(10).Children(func() {
				if ui.Segmented(c, &a.modeIdx, "乐队 Live", "纯正经典").Label("游戏模式").Disabled(a.isDuel()).Changed() {
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
						mInk := pal.InkOf(mc)
						selected := m.ID == a.memberID
						item := ui.Box(c.Key(string(m.ID))).Padding(7, 9).Radius(9).
							Cursor(ui.CursorPointer).Label(m.Name).Tooltip(m.SkillDesc)
						if selected {
							// A highlighter stroke in her 代表色.
							item.Background(mc.Alpha(0.36).Over(t.Surface))
						} else if item.Hovered() {
							item.Background(mc.Alpha(0.14).Over(t.Surface))
						}
						item.Children(func() {
							ui.Row(c).AlignItems(ui.Center).Gap(9).Children(func() {
								memberAvatar(c, m, 34, pal)
								ui.Column(c).Grow(1).Gap(1).Children(func() {
									ui.Row(c).AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
										ui.Row(c).AlignItems(ui.Baseline).Gap(5).Children(func() {
											ui.Text(c, m.Name).FontSize(14).Bold()
											ui.Textf(c, "%s · %s", m.Romaji, m.Role).FontSize(11).TextColor(t.TextMuted)
										})
										if selected && a.bandMode() {
											tag := fmt.Sprintf("×%d", a.game.SkillUsesLeft)
											if m.Passive {
												tag = fmt.Sprintf("盾×%d", a.game.SkillUsesLeft)
											}
											ui.Badge(c, tag).Background(t.Surface.Alpha(0.7)).TextColor(mInk)
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
				ui.Column(c).Gap(5).Padding(8, 4).Children(func() {
					ui.Row(c).Justify(ui.SpaceBetween).Children(func() {
						ui.Text(c, "排练完成度").FontSize(12).TextColor(t.TextMuted)
						ui.Textf(c, "%.0f%%", a.game.Progress()*100).FontSize(12).Bold().TextColor(t.Accent)
					})
					hatchMeter(c, "Live 完成度", a.game.Progress(), pal.InkOf(bandColor), pal)

					ui.Row(c).Justify(ui.SpaceBetween).Children(func() {
						ui.Text(c, "乐队重力值 (Gravity)").FontSize(12).TextColor(t.TextMuted)
						ui.Textf(c, "%d%%", a.game.Gravity).FontSize(12).Bold().TextColor(pal.InkOf(memberImage(MemberSoyorin)))
					})
					hatchMeter(c, "乐队重力值", float64(a.game.Gravity)/100, pal.InkOf(memberImage(MemberSoyorin)), pal)
				})

				// Band Group Chat: sticky notes in each speaker's colour.
				ui.Text(c, "MyGO!!!!! 排练室群聊").FontSize(12).Bold().TextColor(t.TextMuted)
				ui.Scroll(c).Grow(1).Padding(6, 4).Children(func() {
					ui.Column(c).Gap(9).Children(func() {
						for idx := len(a.chatLog) - 1; idx >= 0; idx-- {
							msg := a.chatLog[idx]
							speaker := ui.Hex(msg.ColorHex)
							tilt := float32(0.7)
							if idx%2 == 0 {
								tilt = -0.8
							}
							ui.Column(c.Key(idx)).Gap(2).Padding(6, 8).Radius(2).
								Background(speaker.Alpha(0.2).Over(t.Surface)).
								Shadow(0, 2, 4, 0, pal.SlipShade.Alpha(0.6)).Rotate(tilt).Children(func() {
								ui.Text(c, msg.Speaker).FontSize(11).Bold().TextColor(pal.InkOf(speaker))
								ui.Text(c, msg.Text).FontSize(12)
							})
						}
					})
				})
			})

			// Right Main Stage: Companion Banner + HUD + Board (+ Optional Memes Drawer)
			ui.Column(c).Grow(1).Gap(10).Children(func() {
				// Active Member Banner themed to the selected member!
				ui.Themed(c, memberTheme).Padding(10, 12).Radius(4).Background(t.Surface).
					Shadow(0, 1, 3, 0, pal.SlipShade).Draw(func(p *ui.Painter, r ui.Rect) {
					// A strip of masking tape in her 代表色 holds the note on.
					tape := ui.Rect{X: r.X + 26, Y: r.Y - 6, W: 64, H: 14}
					p.Transform(ui.Rotation(-4), tape.X+tape.W/2, tape.Y+tape.H/2, func() {
						p.Fill(tape, memberColor.Alpha(0.5), 1)
					})
				}).Children(func() {
					ui.Row(c).AlignItems(ui.Center).Justify(ui.SpaceBetween).Gap(12).Children(func() {
						ui.Row(c).Grow(1).AlignItems(ui.Center).Gap(10).Children(func() {
							memberAvatar(c, member, 38, pal)
							ui.Column(c).Grow(1).Gap(2).Children(func() {
								ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
									ui.Text(c, member.Name).FontSize(15).Bold().TextColor(memberInk)
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
							if a.isDuel() {
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

					if a.isDuel() {
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
					byWidth := (availableW - 342 - float32(a.game.Cols-1)*3) / float32(a.game.Cols)
					byHeight := (availableH - 230 - float32(a.game.Rows-1)*3) / float32(a.game.Rows)
					cellSize = max(22, min(64, min(byWidth, byHeight)))
					cellFont = max(12, cellSize*0.48)
					stage := ui.ScrollBoth(c).Grow(1).Clip().Draw(func(p *ui.Painter, bounds ui.Rect) {
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
					a.pruneRipples(now)
					if len(a.ripples) > 0 {
						c.After(16 * time.Millisecond)
					}
					shakeX, shakeY := a.currentShakeOffset(now)
					if shakeX != 0 || shakeY != 0 {
						c.After(16 * time.Millisecond)
					}
					hoverHint := a.game.InspectNumberHover(a.classicHoverRow, a.classicHoverCol, a.flagMode)
					a.classicHoverHint = hoverHint
					hintColor := memberColor
					nextHoverRow, nextHoverCol := -1, -1

					stage.Children(func() {
						ui.Column(c).FillWidth().AlignItems(ui.Center).Padding(4).Children(func() {
							// Graph paper: ruled lines run through the gaps between cells.
							const gridGap, gridPad = float32(3), float32(8)
							grid := ui.Grid(c).Columns(a.game.Cols).Gap(gridGap).Padding(gridPad).Radius(4).
								Shadow(0, 1, 3, 0, pal.SlipShade)
							pitch := cellSize + gridGap
							grid.Draw(func(p *ui.Painter, r ui.Rect) {
								p.Transform(ui.Translation(shakeX, shakeY), 0, 0, func() {
									p.Fill(r, pal.Board, 4)
									for x := r.X + gridPad - gridGap/2; x <= r.X+r.W; x += pitch {
										p.Line(x, r.Y, x, r.Y+r.H, 1, pal.Grid)
									}
									for y := r.Y + gridPad - gridGap/2; y <= r.Y+r.H; y += pitch {
										p.Line(r.X, y, r.X+r.W, y, 1, pal.Grid)
									}
								})
							})
							grid.Children(func() {
								for i := range a.game.Cells {
									cell := &a.game.Cells[i]
									row, col := cell.Row, cell.Col
									tile := ui.Box(c.Key(i)).Size(cellSize, cellSize).Radius(4).Center().
										Label(fmt.Sprintf("cell-%d-%d", row, col))
									if tile.Hovered() {
										nextHoverRow, nextHoverCol = row, col
									}
									isHintCenter := hoverHint.Kind != HintNone && i == hoverHint.Center
									isHintTarget := hoverHint.Kind != HintNone && containsIndex(hoverHint.Targets, i)

									rippleActive := false
									var rippleAlpha float32
									if rip, ok := a.ripples[i]; ok {
										elapsed := now.Sub(rip.Start)
										if elapsed >= rip.Delay && elapsed < rip.Delay+rippleDuration {
											rippleActive = true
											rippleAlpha = 1 - float32(elapsed-rip.Delay)/float32(rippleDuration)
										}
									}

									// Revealed cells are the bare graph paper (no fill); hidden
									// ones are paper slips stuck on it, each a touch askew.
									var tileBg ui.Color
									var slip bool
									var tileCursor bool
									var tileText string
									var tileTextSize float32
									var tileTextColor ui.Color
									var tileArt *ui.SVG

									switch {
									case cell.Revealed:
										switch {
										case cell.Exploded:
											tileBg, tileArt = pal.Boom, art.KnotHit
										case cell.IsMine:
											tileArt = art.Knot
										case cell.Adjacent > 0:
											if isHintCenter {
												tileBg = hintColor.Alpha(0.38)
											} else if rippleActive {
												tileBg = bandColor.Alpha(0.22 * rippleAlpha)
											} else if tile.Hovered() {
												tileBg = pal.Ink.Alpha(0.05)
											}
											tileCursor = true
											tileText = fmt.Sprintf("%d", cell.Adjacent)
											tileTextSize = cellFont * 1.08
											tileTextColor = numberInk(cell.Adjacent, pal.Dark)
										default:
											if rippleActive {
												tileBg = bandColor.Alpha(0.26 * rippleAlpha)
											}
										}
									case cell.WrongFlag:
										slip, tileArt = true, art.WrongFlag
									case cell.Flagged:
										slip, tileArt, tileCursor = true, art.Flag, true
										if cell.ShieldSaved {
											tileArt = art.Shield
										}
									case cell.Questioned:
										slip, tileCursor = true, true
										tileText, tileTextSize, tileTextColor = "?", cellFont*1.08, pal.Graphite
									default:
										slip, tileCursor = true, true
									}
									if slip {
										tileBg = pal.Slip
										if isHintTarget {
											tileBg = hintColor.Alpha(0.42).Over(pal.Slip)
										} else if tile.Hovered() {
											tileBg = pal.Slip.Mix(pal.Sheet, 0.45)
										}
									}

									if shakeX != 0 || shakeY != 0 {
										bgCopy := tileBg
										tile.Draw(func(p *ui.Painter, r ui.Rect) {
											p.Transform(ui.Translation(shakeX, shakeY), 0, 0, func() {
												p.Fill(r, bgCopy, 4)
											})
										})
									} else {
										tile.Background(tileBg)
										if slip {
											tile.Shadow(0, 1, 1.5, 0, pal.SlipShade)
										}
									}
									if slip {
										// Same tilt while shaking, so hit-testing does not move.
										tile.Rotate(float32((i*37)%7-3) * 0.22)
									}
									if tileCursor {
										tile.Cursor(ui.CursorPointer)
									}
									if tileText != "" || tileArt != nil {
										tile.Children(func() {
											var el ui.Element
											if tileArt != nil {
												el = ui.Image(c, tileArt).Size(cellSize*0.82, cellSize*0.82).PassThrough()
											} else {
												el = ui.Text(c, tileText).FontSize(tileTextSize).Bold().TextColor(tileTextColor).PassThrough()
											}
											if shakeX != 0 || shakeY != 0 {
												el.Translate(shakeX, shakeY)
											}
										})
									}

									if tile.RightClicked() {
										res := a.game.ToggleFlag(row, col, a.allowQuestion)
										a.handleAction(c, res, now)
									} else if tile.Clicked() {
										if a.flagMode {
											res := a.game.ToggleFlag(row, col, a.allowQuestion)
											a.handleAction(c, res, now)
										} else if cell.Revealed {
											res := a.game.Chord(row, col)
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
					if nextHoverRow != a.classicHoverRow || nextHoverCol != a.classicHoverCol {
						a.classicHoverRow = nextHoverRow
						a.classicHoverCol = nextHoverCol
						a.classicHoverHint = a.game.InspectNumberHover(nextHoverRow, nextHoverCol, a.flagMode)
						c.After(time.Millisecond)
					}
				})
			})
		})
	})
}

func main() {
	app := newMinesweeperApp()
	mygo.App.WhenReady(func() {
		w := mygo.NewWindow(mygo.WindowOptions{
			Title:           "MyGO!!!!! 迷子扫雷",
			Width:           1280,
			Height:          900,
			BackgroundColor: "#F3EEE2",
			StateKey:        "mygo-minesweeper-v2",
			Content:         ui.View(app.view),
		})
		_ = w.SetIcon(appIconPNG)
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
