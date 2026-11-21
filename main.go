package main

import (
	_ "embed"
	"fmt"
	"log"
	"math"
	"os"
	"slices"
	"strconv"
	"strings"
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

// waveSteps is the last step of a reveal wave; deeper cells share it.
const waveSteps = int(rippleMaxDelay / rippleStepDelay)

// waveStep is the step at which a reveal wave reaches a cell depth away.
func waveStep(depth int) int { return min(depth, waveSteps) }

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
	Row   int
	Col   int
	Text  string
	Kind  duelPopupKind
	Start time.Time
}

type appPopover uint8

const (
	popoverNone appPopover = iota
	popoverSetlist
	popoverSettings
)

type appOptions struct {
	Seed      uint64
	Prefs     *prefs
	SavePrefs func(prefs) error
	// Sound defaults to silence; only the desktop entry point plays aloud.
	Sound soundOutput
}

type memeUnlockNotice struct {
	Meme MemeAchievement
	At   time.Time
}

type minesweeperApp struct {
	diffIdx             int
	modeIdx             int // 0 = Band Live Mode, 1 = Classic Mode
	darkMode            bool
	motion              motionMode
	soundEnabled        bool
	volume              int
	memberID            MemberID
	flagMode            bool
	allowQuestion       bool
	showMemes           bool
	setlistOpen         bool
	activePopover       appPopover
	restorePopover      appPopover
	focusDuelSettlement bool
	haruhikageAlert     bool
	haruhikageIntro     bool

	savePrefsFn         func(prefs) error
	sound               soundOutput
	cues                cueQueue
	systemReduceMotion  bool
	motionReducedActive bool
	windowActive        bool
	desktopFocused      bool
	// duelTickFocused is whether the window was in front at the last duel
	// tick, so the first tick back from the background stays quiet.
	duelTickFocused bool

	game                 *Game
	quote                string
	startedAt            time.Time
	elapsed              int
	bestTimes            map[string]int
	unlockedMemes        map[string]bool
	memeUnlockLog        []memeUnlockNotice
	chatLog              []chatMessage
	chatPosted           int
	chatHeard            int
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
	return newMinesweeperAppWith(appOptions{Seed: s})
}

func newMinesweeperAppWith(opts appOptions) *minesweeperApp {
	p := defaultPrefs()
	if opts.Prefs != nil {
		p = *opts.Prefs
	}
	app := &minesweeperApp{
		diffIdx:         0,
		modeIdx:         0,
		memberID:        MemberTomori,
		savePrefsFn:     opts.SavePrefs,
		sound:           opts.Sound,
		windowActive:    true,
		desktopFocused:  true,
		bestTimes:       make(map[string]int),
		unlockedMemes:   make(map[string]bool),
		seed:            opts.Seed,
		board:           newBoardView(),
		pendingRow:      -1,
		pendingCol:      -1,
		classicHoverRow: -1,
		classicHoverCol: -1,
	}
	if app.sound == nil {
		app.sound = silentSound{}
	}
	app.applyPrefs(p)
	app.resetGame(true)
	return app
}

func newDesktopApp(userConfigDir func() (string, error), save func(string, prefs) error, sound soundOutput) *minesweeperApp {
	p := defaultPrefs()
	var persist func(prefs) error
	if dir, err := userConfigDir(); err == nil && dir != "" {
		p = loadPrefs(dir)
		persist = func(next prefs) error {
			return save(dir, next)
		}
	}
	return newMinesweeperAppWith(appOptions{
		Prefs:     &p,
		SavePrefs: persist,
		Sound:     sound,
	})
}

func (a *minesweeperApp) currentPrefs() prefs {
	return prefs{
		DarkMode:      a.darkMode,
		Motion:        a.motion,
		SoundEnabled:  a.soundEnabled,
		Volume:        a.volume,
		AllowQuestion: a.allowQuestion,
	}
}

func (a *minesweeperApp) applyPrefs(p prefs) {
	a.darkMode = p.DarkMode
	a.motion = p.Motion
	a.soundEnabled = p.SoundEnabled
	a.volume = p.Volume
	a.allowQuestion = p.AllowQuestion
	a.syncMotionState(a.motion.reducedMotion(a.systemReduceMotion))
}

// updatePrefs applies a preference mutation in memory and persists it when any
// field changes. Persistence errors leave the updated in-memory value intact.
func (a *minesweeperApp) updatePrefs(fn func(*prefs)) bool {
	prev := a.currentPrefs()
	next := prev
	fn(&next)
	if next == prev {
		return false
	}
	a.applyPrefs(next)
	if next.DarkMode != prev.DarkMode || next.Motion != prev.Motion ||
		next.SoundEnabled != prev.SoundEnabled || next.AllowQuestion != prev.AllowQuestion {
		a.playSound(SoundToggle, 1, 0)
	}
	if a.savePrefsFn != nil {
		_ = a.savePrefsFn(next)
	}
	return true
}

func (a *minesweeperApp) toggleMute() bool {
	return a.updatePrefs(func(p *prefs) {
		p.SoundEnabled = !p.SoundEnabled
	})
}

func (a *minesweeperApp) reducedMotion(c *ui.Context) bool {
	return a.motion.reducedMotion(c.Preferences().ReduceMotion)
}

func (a *minesweeperApp) reducedMotionPainter(p *ui.Painter) bool {
	return a.motion.reducedMotion(p.Preferences().ReduceMotion)
}

func (a *minesweeperApp) syncMotionState(reduced bool) {
	a.motionReducedActive = reduced
	if reduced {
		a.ripples = nil
		a.shakeStart = time.Time{}
		a.shakeAmplitude = 0
		a.shakeDuration = 0
		a.board.finishGlide(a.game)
	}
}

func (a *minesweeperApp) setWindowFocused(focused bool) {
	a.desktopFocused = focused
	if !focused {
		a.duelTickFocused = false
	}
}

func (a *minesweeperApp) syncWindowFocus(c *ui.Context) {
	a.windowActive = c.WindowActive() && !c.WindowHidden()
	if !a.isWindowFocused() {
		a.duelTickFocused = false
	}
}

func (a *minesweeperApp) isWindowFocused() bool {
	return a.windowActive && a.desktopFocused
}

func (a *minesweeperApp) syncSetlistPopover() {
	if a.setlistOpen && a.activePopover != popoverSetlist {
		a.activePopover = popoverSetlist
	} else if !a.setlistOpen && a.activePopover == popoverSetlist {
		a.activePopover = popoverNone
	}
}

func (a *minesweeperApp) popoverOpen(kind appPopover) bool {
	a.syncSetlistPopover()
	return kind != popoverNone && a.activePopover == kind
}

func (a *minesweeperApp) hasOpenPopover() bool {
	a.syncSetlistPopover()
	return a.activePopover != popoverNone
}

// setActivePopover is the one place a popover opens or closes, so each
// change rustles its paper once.
func (a *minesweeperApp) setActivePopover(kind appPopover) {
	a.syncSetlistPopover()
	prev := a.activePopover
	a.activePopover = kind
	a.setlistOpen = kind == popoverSetlist
	switch {
	case kind == prev:
	case kind == popoverNone:
		a.playSound(SoundPopoverClose, 1, 0)
	default:
		a.playSound(SoundPopoverOpen, 1, 0)
	}
}

func (a *minesweeperApp) openPopover(kind appPopover) {
	a.setActivePopover(kind)
	a.restorePopover = popoverNone
}

func (a *minesweeperApp) togglePopover(kind appPopover) bool {
	if a.popoverOpen(kind) {
		a.closePopover(kind, false)
		return false
	}
	a.openPopover(kind)
	return kind != popoverNone
}

func (a *minesweeperApp) closePopover(kind appPopover, restoreFocus bool) {
	a.syncSetlistPopover()
	if kind == popoverNone {
		kind = a.activePopover
	}
	if kind == popoverNone {
		return
	}
	if a.activePopover == kind {
		a.setActivePopover(popoverNone)
	}
	if restoreFocus {
		a.restorePopover = kind
	}
}

func (a *minesweeperApp) dismissPopovers() {
	a.setActivePopover(popoverNone)
	a.restorePopover = popoverNone
}

func (a *minesweeperApp) consumePopoverFocusRestore(kind appPopover) bool {
	if kind != popoverNone && a.restorePopover == kind {
		a.restorePopover = popoverNone
		return true
	}
	return false
}

// bindPopover mounts a mutually-exclusive non-modal popover anchored to
// trigger, handling Escape dismissal with focus return to trigger and
// FocusLeft auto-close.
func (a *minesweeperApp) bindPopover(
	c *ui.Context,
	kind appPopover,
	key string,
	trigger ui.Element,
	body func(closeWithFocus func()) ui.Element,
) ui.Element {
	open := a.popoverOpen(kind)
	wasOpen := open
	closedByApp := false
	closeWithFocus := func() {
		a.closePopover(kind, true)
		open = false
		closedByApp = true
	}
	if open && trigger.Shortcut(0, ui.KeyEscape) {
		closeWithFocus()
	}
	var popover ui.Element
	if open {
		// Guard with open so c.Key(key) is never left unconsumed when ui.Popover returns nil.
		popover = ui.Popover(c.Key(key), trigger, &open, func() {
			panel := body(closeWithFocus)
			if panel.Valid() && panel.Shortcut(0, ui.KeyEscape) {
				closeWithFocus()
			}
		}).Background(c.Theme().Surface).KeepFocus()
	}

	if !closedByApp && wasOpen && !open {
		// ui.Popover dismisses on either Escape or outside press;
		// restore trigger focus only for Escape dismissal.
		restoreFocus := popover.Valid() && popover.Dismissed() && !popover.PressedOutside()
		a.closePopover(kind, restoreFocus)
	} else if popover.Valid() && a.popoverOpen(kind) && popover.FocusLeft() {
		a.closePopover(kind, false)
	}
	if a.consumePopoverFocusRestore(kind) {
		trigger.Focus()
	}
	return popover
}

// popoverWorkspaceShield covers the workspace while any popover is open so an
// outside dismiss click never leaks onto a board cell or mine.
func (a *minesweeperApp) popoverWorkspaceShield(c *ui.Context) {
	if !a.hasOpenPopover() {
		return
	}
	ui.Box(c.Key("popover-workspace-shield")).Absolute().Top(0).Bottom(0).Start(0).End(0).
		HandleInput(func(ev ui.InputEvent) bool {
			switch ev.Kind {
			case ui.InputPointerDown:
				a.dismissPopovers()
				return true
			case ui.InputPointerUp, ui.InputScroll, ui.InputZoom:
				return true
			}
			return false
		})
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

// classicGridMaxSide is the widest classic board, 《影色舞》's 30×30, that
// is still laid out as one widget per cell.
const classicGridMaxSide = 30

// classicOnCanvas reports whether the current classic board is too large
// for per-cell widgets and plays on the shared pan/zoom canvas instead.
func (a *minesweeperApp) classicOnCanvas() bool {
	return !a.isDuel() && max(a.game.Rows, a.game.Cols) > classicGridMaxSide
}

func (a *minesweeperApp) pushChat(speaker, colorHex, text string) {
	a.chatLog = append(a.chatLog, chatMessage{
		Speaker:  speaker,
		ColorHex: colorHex,
		Text:     text,
	})
	a.chatPosted++
	if len(a.chatLog) > 18 {
		a.chatLog = a.chatLog[len(a.chatLog)-18:]
	}
}

func (a *minesweeperApp) resetGame(initial bool) {
	posted, closing := a.chatPosted, a.hasOpenPopover()
	prevDiffID := ""
	if a.game != nil {
		prevDiffID = a.game.Difficulty.ID
	}
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
	a.classicSize = 0
	a.board = newBoardView()
	a.ripples = nil
	a.clearCues()
	a.duelTickFocused = false
	a.duelPopups = nil
	a.shakeStart = time.Time{}
	a.shakeAmplitude = 0
	a.shakeDuration = 0
	a.haruhikageAlert = false
	a.haruhikageIntro = false
	a.haruhikageIntroStart = time.Time{}
	a.focusDuelSettlement = false
	a.dismissPopovers()
	a.classicHoverRow = -1
	a.classicHoverCol = -1
	a.classicHoverHint = HoverHint{}
	a.quote = PickQuote(a.memberID, "start")
	m := FindMember(a.memberID)
	switch {
	case initial:
		a.pushChat("MyGO!!!!! 排练室", hexOf(bandColor), fmt.Sprintf("翻开谱本：《%s》（%s）", diff.Song, diff.Subtitle))
	case prevDiffID != diff.ID:
		a.pushChat("MyGO!!!!! 排练室", hexOf(bandColor), fmt.Sprintf("翻到新谱：《%s》（%s）", diff.Song, diff.Subtitle))
	default:
		a.pushChat("排练重开", hexOf(bandColor), fmt.Sprintf("重新演奏《%s》", diff.Song))
	}
	if diff.Gameplay == GameplayDuel {
		a.pushChat("MyGO!!!!! 决斗室", hexOf(bandColor), "同盘限时抢地：开安全格 +1，踩雷 −100；手动插旗终局结算对错，不提前露底。")
	}
	a.pushChat(m.Name, m.ColorHex, a.quote)
	// The opening notes are on the wall before anyone listens, and a restart
	// that puts a popover away makes that sound for its notes.
	if initial || closing {
		a.chatVoicedSince(posted)
	}
}

func (a *minesweeperApp) enqueueRipple(now time.Time, wave []RevealWaveEntry) {
	if a.motionReducedActive || len(wave) == 0 {
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
		delay := time.Duration(waveStep(entry.Depth)) * rippleStepDelay
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
	var text string
	var kind duelPopupKind
	switch {
	case res.Kind == ActionFlagged:
		text = fmt.Sprintf("+%d 收雷!", res.ScoreDelta)
		kind = duelPopupClaim
	case res.Kind == ActionLost && res.ScoreDelta < 0:
		text = fmt.Sprintf("%d 踩雷!", res.ScoreDelta)
		kind = duelPopupBoom
	case res.RevealedCount >= 8 || a.game.duel.PlayerCombo >= 3:
		text = fmt.Sprintf("+%d · 连击×%d", res.ScoreDelta, a.game.duel.PlayerCombo)
		kind = duelPopupCombo
	default:
		text = fmt.Sprintf("+%d", res.ScoreDelta)
		kind = duelPopupScore
	}
	kept := a.duelPopups[:0]
	for _, p := range a.duelPopups {
		if now.Sub(p.Start) >= 0 && now.Sub(p.Start) < duelPopupDuration {
			kept = append(kept, p)
		}
	}
	kept = append(kept, duelPopup{Row: row, Col: col, Text: text, Kind: kind, Start: now})
	if len(kept) > maxDuelPopups {
		kept = kept[len(kept)-maxDuelPopups:]
	}
	a.duelPopups = kept
}

func (a *minesweeperApp) triggerShake(now time.Time, amp float32, dur time.Duration) {
	if a.motionReducedActive {
		return
	}
	a.shakeStart = now
	a.shakeAmplitude = amp
	a.shakeDuration = dur
}

func (a *minesweeperApp) currentShakeOffset(now time.Time) (float32, float32) {
	if a.motionReducedActive {
		a.shakeAmplitude = 0
		return 0, 0
	}
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
	a.applyQuote(res.QuoteEvent)
	a.highlightSound(res, now)

	switch res.Kind {
	case ActionShielded:
		a.triggerShake(now, 4.5, 220*time.Millisecond)
		c.Toast("长崎爽世发动了「温柔的微笑」：这次就当没点过吧！")
	case ActionSkill:
		c.Toast(fmt.Sprintf("%s 发动「%s」", m.Name, m.SkillName))
	case ActionLost:
		a.triggerShake(now, 9.0, 320*time.Millisecond)
		if a.isDuel() {
			c.Toast("踩雷 −100 · 硬直 2.4 秒！小心被灯拉开比分")
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
		c.Toast(fmt.Sprintf("《%s》Live 大成功！用时 %d 秒", a.game.Difficulty.Song, a.elapsed))
	}

	a.unlockMemes(c, res.NewMemes)
}

func (a *minesweeperApp) applyQuote(event string) {
	if event == "" {
		return
	}
	m := FindMember(a.memberID)
	a.quote = PickQuote(a.memberID, event)
	a.pushChat(m.Name, m.ColorHex, a.quote)
}

func findMeme(id string) (MemeAchievement, bool) {
	for _, meme := range AllMemes {
		if meme.ID == id {
			return meme, true
		}
	}
	return MemeAchievement{}, false
}

// recordMemeUnlock marks memeID unlocked once per run and records a single
// first-unlock notice for downstream presentation without emitting UI side
// effects itself.
func (a *minesweeperApp) recordMemeUnlock(memeID string, now time.Time) (MemeAchievement, bool) {
	if a.unlockedMemes[memeID] {
		return MemeAchievement{}, false
	}
	a.unlockedMemes[memeID] = true
	meme, ok := findMeme(memeID)
	if !ok {
		return MemeAchievement{}, false
	}
	a.memeUnlockLog = append(a.memeUnlockLog, memeUnlockNotice{
		Meme: meme,
		At:   now,
	})
	return meme, true
}

func (a *minesweeperApp) unlockMemes(c *ui.Context, memeIDs []string) {
	now := c.Now()
	for _, memeID := range memeIDs {
		meme, unlocked := a.recordMemeUnlock(memeID, now)
		if !unlocked {
			continue
		}
		c.Toast("解锁名场面：「" + meme.Title + "」")
		a.pushChat("名场面解锁", hexOf(goldHighlighter), "★ "+meme.Title+" — "+meme.Desc)
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

func (a *minesweeperApp) settleDuel(c *ui.Context, now time.Time, result string) {
	d := a.game.duel
	a.duelEnded = true
	posted := a.chatPosted
	a.dismissPopovers()
	a.focusDuelSettlement = true
	a.pushChat("决斗结算", hexOf(goldHighlighter), result)
	a.applyQuote(d.QuoteEvent)
	a.unlockMemes(c, d.NewMemes)
	if slices.Contains(d.NewMemes, MemeHaruhikage) {
		a.haruhikageIntro = true
		a.haruhikageIntroStart = now
	}
	// The settlement's own sound carries its notes, as does a popover it
	// puts away.
	a.chatVoicedSince(posted)
	a.settleSound(d.PlayerScore > d.AIScore, slices.Contains(d.NewMemes, MemeHaruhikage))
	c.ToastAction(result, "再战一局", func() { a.resetGame(false) })
}

func (a *minesweeperApp) tickDuel(c *ui.Context, now time.Time) {
	if !a.isDuel() {
		return
	}
	d := a.game.duel
	if !a.lastTick.IsZero() && a.game.Status == StatusPlaying {
		before := d.remaining()
		tick := a.game.TickDuel(now.Sub(a.lastTick).Seconds())
		a.elapsed = int(d.Elapsed)
		if tick.AITurn != nil && tick.AITurn.Kind != ActionNoop {
			a.enqueueRipple(now, tick.AITurn.RevealWave)
			if tick.AITurn.RevealedCount > 0 {
				a.board.LastEffect = now
				a.board.EffectCell = tick.AICell
			}
			a.voiceAITurn(tick.AITurn.Kind, tick.AICell)
			if tick.AITurn.Kind == ActionLost {
				c.Toast("高松灯踩雷 −100")
			}
		}
		if a.game.Status == StatusPlaying {
			a.voiceCountdown(before, d.remaining())
		}
		if tick.Finished {
			a.elapsed = int(d.Duration)
			result := "平局！这一整页的迷茫，恰好一人一半"
			if d.PlayerScore > d.AIScore {
				result = fmt.Sprintf("决斗胜利！%d : %d · 赢了就是赢了！", d.PlayerScore, d.AIScore)
				a.recordMemeUnlock(MemeForever, now)
			} else if d.PlayerScore < d.AIScore {
				result = fmt.Sprintf("决斗惜败 %d : %d · 这局剪掉不准发到 SNS 上！", d.PlayerScore, d.AIScore)
			}
			a.settleDuel(c, now, result)
		}
	}
	if a.game.Status == StatusWon && !a.duelEnded {
		result := fmt.Sprintf("清盘决斗结算：你 %d · 高松灯 %d", d.PlayerScore, d.AIScore)
		a.settleDuel(c, now, result)
	}
	a.lastTick = now
	a.duelTickFocused = a.isWindowFocused()
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

var (
	setlistChevronDown  = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16"><path d="M3.5 6L8 10.5L12.5 6" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"/></svg>`))
	setlistChevronUp    = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16"><path d="M3.5 10L8 5.5L12.5 10" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"/></svg>`))
	setlistCheckIcon    = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16"><path d="M3 8.5L6.5 12L13 4.5" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/></svg>`))
	settingsSlidersIcon = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16"><path d="M2.5 4.5h11M2.5 8h11M2.5 11.5h11" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/><circle cx="5.5" cy="4.5" r="1.6" fill="currentColor"/><circle cx="10.5" cy="8" r="1.6" fill="currentColor"/><circle cx="6.5" cy="11.5" r="1.6" fill="currentColor"/></svg>`))
)

// setlistInk derives a WCAG-AA (4.5:1+) ink from base against the exact
// composited background a setlist control paints beneath its text.
func setlistInk(pal *palette, bg, base ui.Color) ui.Color {
	under := *pal
	under.Board = bg
	under.Sheet = bg
	return under.InkOf(base)
}

// setlistAccentInk derives a WCAG-AA (4.5:1+) ink from the 乐队色 against
// the exact composited background a setlist control paints beneath its text.
func setlistAccentInk(pal *palette, bg ui.Color) ui.Color {
	return setlistInk(pal, bg, bandColor)
}

type setlistEntry struct {
	Index int
	Diff  Difficulty
}

type setlistGroup struct {
	Gameplay Gameplay
	Title    string
	Entries  []setlistEntry
}

func groupSetlistTracks(tracks []Difficulty) []setlistGroup {
	var classic, duel []setlistEntry
	for i, d := range tracks {
		entry := setlistEntry{Index: i, Diff: d}
		if d.Gameplay == GameplayDuel {
			duel = append(duel, entry)
		} else {
			classic = append(classic, entry)
		}
	}
	var groups []setlistGroup
	if len(classic) > 0 {
		groups = append(groups, setlistGroup{
			Gameplay: GameplayClassic,
			Title:    "经典",
			Entries:  classic,
		})
	}
	if len(duel) > 0 {
		groups = append(groups, setlistGroup{
			Gameplay: GameplayDuel,
			Title:    "决斗",
			Entries:  duel,
		})
	}
	return groups
}

func formatGroupedInt(n int) string {
	s := strconv.Itoa(n)
	if len(s) <= 3 {
		return s
	}
	var b strings.Builder
	rem := len(s) % 3
	if rem > 0 {
		b.WriteString(s[:rem])
		b.WriteByte(',')
	}
	for i := rem; i < len(s); i += 3 {
		if i > rem {
			b.WriteByte(',')
		}
		b.WriteString(s[i : i+3])
	}
	return b.String()
}

func setlistTrackSpec(d Difficulty) string {
	return fmt.Sprintf("%d×%d · %s 雷", d.Cols, d.Rows, formatGroupedInt(d.Mines))
}

func (a *minesweeperApp) setlistControl(c *ui.Context, pal *palette, t *ui.Theme) {
	cur := a.game.Difficulty
	isOpen := a.popoverOpen(popoverSetlist)
	trigger := ui.ButtonBase(c.Key("setlist-trigger")).
		Padding(6, 12).Radius(6).Cursor(ui.CursorPointer).
		Label("歌单").Value(cur.Song).HasPopup(ui.PopupListBox).Expanded(isOpen)

	var triggerBg ui.Color
	var triggerBorder ui.Color
	switch {
	case isOpen:
		triggerBg = t.Accent.Alpha(0.14).Over(t.Surface)
		triggerBorder = t.Accent.Alpha(0.7)
	case trigger.Hovered():
		triggerBg = pal.Slip.Alpha(0.85).Over(t.Surface)
		triggerBorder = pal.Rule
	default:
		triggerBg = pal.Slip.Alpha(0.45).Over(t.Surface)
		triggerBorder = pal.Rule
	}
	triggerInk := setlistAccentInk(pal, triggerBg)
	trigger.Background(triggerBg).Border(1, triggerBorder).
		Shadow(0, 1, 2, 0, pal.SlipShade.Alpha(0.45))

	trigger.Children(func() {
		ui.Row(c).AlignItems(ui.Center).Gap(8).PassThrough().Children(func() {
			ui.Box(c).Size(3, 14).Radius(2).Background(triggerInk).PassThrough()
			ui.Text(c, "歌单").FontSize(12).Bold().TextColor(triggerInk).PassThrough()
			ui.Box(c).Size(1, 14).Background(pal.Rule).PassThrough()
			ui.Text(c, fmt.Sprintf("《%s》", cur.Song)).FontSize(13).Bold().TextColor(t.Text).PassThrough()
			if cur.Label != "" && cur.Label != cur.Song {
				ui.Text(c, cur.Label).FontSize(11).TextColor(t.TextMuted).PassThrough()
			}
			chevron, chevronColor := setlistChevronDown, pal.InkMuted
			if isOpen {
				chevron, chevronColor = setlistChevronUp, triggerInk
			}
			ui.Icon(c, chevron).Size(13, 13).TextColor(chevronColor).PassThrough()
		})
	})

	focusSelected := false
	openArrow := trigger.Shortcut(0, ui.KeyDown) || trigger.Shortcut(0, ui.KeyUp)
	switch {
	case trigger.Clicked():
		focusSelected = a.togglePopover(popoverSetlist)
	case openArrow:
		a.openPopover(popoverSetlist)
		focusSelected = true
	}

	var listbox ui.Element
	popover := a.bindPopover(c, popoverSetlist, "setlist-popover", trigger, func(closeWithFocus func()) ui.Element {
		selectedBg := t.Accent.Alpha(0.14).Over(t.Surface)
		selectedInk := setlistAccentInk(pal, selectedBg)
		groupInk := setlistAccentInk(pal, t.Surface)
		groups := groupSetlistTracks(Difficulties)

		panel := ui.Column(c).Width(332).Gap(4)
		panel.Children(func() {
			ui.Row(c).FillWidth().Padding(2, 6, 6, 6).AlignItems(ui.Center).Justify(ui.SpaceBetween).
				Draw(func(p *ui.Painter, r ui.Rect) {
					p.Line(r.X, r.Y+r.H-0.5, r.X+r.W, r.Y+r.H-0.5, 1, pal.Rule)
				}).Children(func() {
				ui.Text(c, "排练歌单").FontSize(12).Bold().TextColor(t.TextMuted)
				ui.Textf(c, "%d 首", len(Difficulties)).FontSize(11).TextColor(t.TextMuted).FontFeatures("tnum")
			})

			listbox = ui.Column(c.Key("setlist-listbox")).FillWidth().Gap(6).
				Role(ui.RoleListBox).Label("排练歌单").FocusGroup(ui.Vertical)
			listbox.Children(func() {
				trackNo := 0
				for gIdx, group := range groups {
					section := ui.Column(c.Key(string(group.Gameplay))).FillWidth().Gap(2).Role(ui.RoleGroup)
					if gIdx > 0 {
						section.Padding(6, 0, 0, 0).Draw(func(p *ui.Painter, r ui.Rect) {
							p.Line(r.X+4, r.Y+0.5, r.X+r.W-4, r.Y+0.5, 1, pal.Rule)
						})
					}
					var groupHeading ui.Element
					section.Children(func() {
						ui.Row(c).FillWidth().Padding(2, 6, 2, 6).AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
							ui.Row(c).AlignItems(ui.Center).Gap(6).Children(func() {
								ui.Box(c).Size(3, 11).Radius(1.5).Background(groupInk)
								groupHeading = ui.Text(c, group.Title).FontSize(11).Bold().TextColor(groupInk)
							})
							ui.Textf(c, "%d 首", len(group.Entries)).FontSize(11).TextColor(t.TextMuted).FontFeatures("tnum")
						})

						for j, entry := range group.Entries {
							trackNo++
							d := entry.Diff
							selected := entry.Index == a.diffIdx
							item := ui.ButtonBase(c.Key(entry.Index)).FillWidth().Padding(6, 10).Radius(6).
								Cursor(ui.CursorPointer).Role(ui.RoleOption).Label(d.Song).
								Description(setlistTrackSpec(d)).
								AccessSelected(selected).PositionInSet(trackNo, len(Difficulties))
							if focusSelected && selected {
								item.Focus()
							}

							itemBg := t.Surface
							if selected {
								itemBg = selectedBg
								item.Background(selectedBg)
							} else if item.Hovered() {
								itemBg = pal.Slip.Alpha(0.6).Over(t.Surface)
								item.Background(itemBg)
							}
							itemMutedInk := setlistInk(pal, itemBg, t.TextMuted)

							if j < len(group.Entries)-1 {
								item.Draw(func(p *ui.Painter, r ui.Rect) {
									p.Line(r.X+6, r.Y+r.H-0.5, r.X+r.W-6, r.Y+r.H-0.5, 1, pal.Grid)
								})
							}

							item.Children(func() {
								ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(8).PassThrough().Children(func() {
									ui.Textf(c, "%02d", trackNo).Width(18).FontSize(11).TextColor(itemMutedInk).FontFeatures("tnum").PassThrough()
									ui.Column(c).Grow(1).Gap(2).PassThrough().Children(func() {
										ui.Row(c).FillWidth().AlignItems(ui.Baseline).Gap(6).PassThrough().Children(func() {
											songEl := ui.Text(c, d.Song).FontSize(13).PassThrough()
											if selected {
												songEl.Bold().TextColor(selectedInk)
											} else {
												songEl.TextColor(t.Text)
											}
											if d.Label != "" && d.Label != d.Song {
												ui.Text(c, d.Label).FontSize(11).TextColor(itemMutedInk).PassThrough()
											}
										})
										ui.Text(c, setlistTrackSpec(d)).FontSize(11).TextColor(itemMutedInk).FontFeatures("tnum").PassThrough()
									})
									if selected {
										ui.Icon(c, setlistCheckIcon).Size(13, 13).TextColor(selectedInk).PassThrough()
									}
								})
							})

							if item.Clicked() {
								if entry.Index != a.diffIdx {
									a.diffIdx = entry.Index
									a.resetGame(false)
								}
								closeWithFocus()
							}
						}
					})
					section.LabelledBy(groupHeading)
				}
			})
		})
		return panel
	})

	if popover.Valid() {
		trigger.Controls(listbox)
	}
}

func motionToIndex(m motionMode) int {
	switch m {
	case motionStandard:
		return 1
	case motionReduced:
		return 2
	default:
		return 0
	}
}

func indexToMotion(idx int) motionMode {
	switch idx {
	case 1:
		return motionStandard
	case 2:
		return motionReduced
	default:
		return motionSystem
	}
}

func (a *minesweeperApp) settingsControl(c *ui.Context, pal *palette) {
	sc := settingsColorsFor(pal.Dark)
	isOpen := a.popoverOpen(popoverSettings)
	trigger := ui.ButtonBase(c.Key("settings-trigger")).
		Padding(6, 11).Radius(6).Cursor(ui.CursorPointer).
		Label("设置").HasPopup(ui.PopupDialog).Expanded(isOpen).
		Tooltip("画面、声音与扫雷偏好设置")

	triggerBg, triggerBorder := sc.TriggerIdleBg, sc.TriggerBorder
	iconColor, textColor, chevronColor := sc.TriggerIdleInk, sc.TriggerIdleText, sc.TriggerIdleIcon
	chevron := setlistChevronDown
	switch {
	case isOpen:
		triggerBg, triggerBorder = sc.TriggerOpenBg, sc.TriggerOpenBorder
		iconColor, textColor, chevronColor = sc.TriggerOpenInk, sc.TriggerOpenInk, sc.TriggerOpenInk
		chevron = setlistChevronUp
	case trigger.Hovered():
		triggerBg = sc.TriggerHoverBg
		iconColor, textColor, chevronColor = sc.TriggerHoverInk, sc.TriggerHoverText, sc.TriggerHoverIcon
	}
	trigger.Background(triggerBg).Border(1, triggerBorder).
		Shadow(0, 1, 2, 0, pal.SlipShade.Alpha(0.45))

	trigger.Children(func() {
		ui.Row(c).AlignItems(ui.Center).Gap(6).PassThrough().Children(func() {
			ui.Icon(c, settingsSlidersIcon).Size(13, 13).TextColor(iconColor).PassThrough()
			ui.Text(c, "设置").FontSize(12).Bold().TextColor(textColor).SingleLine().PassThrough()
			ui.Icon(c, chevron).Size(12, 12).TextColor(chevronColor).PassThrough()
		})
	})

	focusFirst := false
	switch {
	case trigger.Clicked():
		focusFirst = a.togglePopover(popoverSettings)
	case trigger.Shortcut(0, ui.KeyDown):
		a.openPopover(popoverSettings)
		focusFirst = true
	}

	var panelEl ui.Element
	popover := a.bindPopover(c, popoverSettings, "settings-popover", trigger, func(_ func()) ui.Element {
		panel := ui.Column(c.Key("settings-panel")).Width(320).Gap(10).Label("偏好设置")
		panelEl = panel
		sectionHeader := func(title string) ui.Element {
			var heading ui.Element
			ui.Row(c).FillWidth().Padding(2, 4, 2, 4).AlignItems(ui.Center).Gap(6).Children(func() {
				ui.Box(c).Size(3, 11).Radius(1.5).Background(sc.GroupInk)
				heading = ui.Text(c, title).FontSize(11).Bold().TextColor(sc.GroupInk)
			})
			return heading
		}
		divider := func(p *ui.Painter, r ui.Rect) {
			p.Line(r.X+4, r.Y+0.5, r.X+r.W-4, r.Y+0.5, 1, pal.Rule)
		}

		panel.Children(func() {
			ui.Row(c).FillWidth().Padding(2, 6, 6, 6).AlignItems(ui.Center).Justify(ui.SpaceBetween).
				Draw(func(p *ui.Painter, r ui.Rect) {
					p.Line(r.X, r.Y+r.H-0.5, r.X+r.W, r.Y+r.H-0.5, 1, pal.Rule)
				}).Children(func() {
				ui.Text(c, "偏好设置").FontSize(12).Bold().TextColor(sc.MutedInk)
				ui.Text(c, "按 Esc 收起").FontSize(11).TextColor(sc.MutedInk)
			})

			// Group 1: Display & Motion
			displayGroup := ui.Column(c.Key("settings-group-display")).FillWidth().Gap(8).Role(ui.RoleGroup)
			var displayHeading ui.Element
			displayGroup.Children(func() {
				displayHeading = sectionHeader("画面与动效")

				ui.Row(c).FillWidth().Padding(2, 4).AlignItems(ui.Center).Justify(ui.SpaceBetween).Gap(8).Children(func() {
					lampLabel := ui.Text(c, "夜间台灯").FontSize(13).TextColor(sc.LabelInk)
					dark := a.darkMode
					sw := ui.Switch(c.Key("settings-dark-mode"), &dark).LabelledBy(lampLabel).
						Tooltip("切换日间方格纸与深夜台灯下的笔记本")
					if focusFirst || sw.Pressed() {
						sw.Focus()
					}
					if sw.Changed() {
						a.updatePrefs(func(p *prefs) { p.DarkMode = dark })
					}
				})

				ui.Column(c).FillWidth().Padding(2, 4).Gap(6).Children(func() {
					motionLabel := ui.Text(c, "动效").FontSize(13).TextColor(sc.LabelInk)
					motionIdx := motionToIndex(a.motion)
					motionLabels := [3]string{"跟随系统", "标准", "减弱"}
					parts := ui.SegmentedBase(c.Key("settings-motion"), &motionIdx, len(motionLabels))
					parts.Track.FillWidth().AlignItems(ui.Stretch).Padding(2).Gap(2).Radius(7).
						Background(sc.SegTrackBg).Border(1, pal.Rule).LabelledBy(motionLabel).
						Children(func() {
							for i, label := range motionLabels {
								seg := parts.Segment(i).Grow(1).Padding(5, 8).Radius(5).Cursor(ui.CursorPointer)
								if seg.Pressed() {
									seg.Focus()
								}
								active := i == motionIdx
								textColor := sc.SegIdleInk
								if active {
									seg.Background(sc.TriggerOpenBg).Shadow(0, 1, 2, 0, pal.SlipShade.Alpha(0.45))
									textColor = sc.TriggerOpenInk
								} else if seg.Hovered() {
									seg.Background(sc.TriggerHoverBg)
									textColor = sc.TriggerHoverText
								}
								seg.Children(func() {
									txt := ui.Text(c, label).FontSize(12).TextColor(textColor).SingleLine().PassThrough()
									if active {
										txt.Bold()
									}
								})
							}
						})
					if parts.Track.Changed() {
						a.updatePrefs(func(p *prefs) { p.Motion = indexToMotion(motionIdx) })
					}
				})
			})
			displayGroup.LabelledBy(displayHeading)

			// Group 2: Sound
			soundGroup := ui.Column(c.Key("settings-group-sound")).FillWidth().Gap(8).
				Padding(10, 0, 0, 0).Role(ui.RoleGroup).Draw(divider)
			var soundHeading ui.Element
			soundGroup.Children(func() {
				soundHeading = sectionHeader("声音")

				var soundSwitch ui.Element
				ui.Row(c).FillWidth().Padding(2, 4).AlignItems(ui.Center).Justify(ui.SpaceBetween).Gap(8).Children(func() {
					var soundLabel ui.Element
					ui.Row(c).AlignItems(ui.Center).Gap(6).Children(func() {
						soundLabel = ui.Text(c, "音效").FontSize(13).TextColor(sc.LabelInk)
						ui.Box(c).Padding(1, 6).Radius(4).Background(sc.KeyBg).Border(1, pal.Rule).Children(func() {
							ui.Text(c, "M").FontSize(10.5).Bold().TextColor(sc.KeyInk).SingleLine()
						})
					})
					soundOn := a.soundEnabled
					soundSwitch = ui.Switch(c.Key("settings-sound"), &soundOn).LabelledBy(soundLabel).
						KeyShortcuts(0, ui.KeyM).Tooltip("开关文具与乐器音效（按 M 切换）")
					if soundSwitch.Pressed() {
						soundSwitch.Focus()
					}
					if soundSwitch.Changed() {
						a.updatePrefs(func(p *prefs) { p.SoundEnabled = soundOn })
					}
				})

				ui.Column(c).FillWidth().Padding(2, 4).Gap(4).Children(func() {
					volLabelInk, volValueInk := sc.LabelInk, sc.GroupInk
					if !a.soundEnabled {
						volLabelInk, volValueInk = sc.MutedInk, sc.MutedInk
					}
					var volLabel ui.Element
					ui.Row(c).FillWidth().AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
						volLabel = ui.Text(c, "音量").FontSize(13).TextColor(volLabelInk)
						ui.Textf(c, "%d%%", a.volume).FontSize(12).Bold().TextColor(volValueInk).FontFeatures("tnum")
					})
					vol := float64(a.volume)
					volSlider := ui.Slider(c.Key("settings-volume"), &vol, 0, 100).
						Step(1).FillWidth().LabelledBy(volLabel).Value(fmt.Sprintf("%d%%", a.volume)).
						Disabled(!a.soundEnabled)
					if volSlider.Pressed() {
						volSlider.Focus()
					}
					if !a.soundEnabled && volSlider.Focused() {
						// Muting via M while the slider holds focus disables it;
						// keep focus inside the popover so FocusLeft does not close it.
						soundSwitch.Focus()
					}
					if volSlider.Changed() {
						a.updatePrefs(func(p *prefs) { p.Volume = int(math.Round(vol)) })
					}
				})
			})
			soundGroup.LabelledBy(soundHeading)

			// Group 3: Minesweeper Controls
			gameplayGroup := ui.Column(c.Key("settings-group-gameplay")).FillWidth().Gap(8).
				Padding(10, 0, 0, 0).Role(ui.RoleGroup).Draw(divider)
			var gameplayHeading ui.Element
			gameplayGroup.Children(func() {
				gameplayHeading = sectionHeader("扫雷操作")

				ui.Row(c).FillWidth().Padding(2, 4).AlignItems(ui.Center).Justify(ui.SpaceBetween).Gap(8).Children(func() {
					qLabel := ui.Text(c, "右键 ? 标记").FontSize(13).TextColor(sc.LabelInk)
					allowQ := a.allowQuestion
					qSwitch := ui.Switch(c.Key("settings-allow-question"), &allowQ).LabelledBy(qLabel).
						Tooltip("右键循环加入「？」不确定标记")
					if qSwitch.Pressed() {
						qSwitch.Focus()
					}
					if qSwitch.Changed() {
						a.updatePrefs(func(p *prefs) { p.AllowQuestion = allowQ })
					}
				})
			})
			gameplayGroup.LabelledBy(gameplayHeading)
		})
		return panel
	})

	if popover.Valid() {
		popover.AttachTo(trigger, ui.AnchorBottomEnd, ui.AnchorTopEnd)
		trigger.Controls(panelEl)
	}
}

func (a *minesweeperApp) duelHUD(c *ui.Context) {
	d := a.game.duel
	hud := duelHUDFor(a.darkMode)
	ui.Row(c).Grow(0).Padding(8, 10).Gap(12).AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Badge(c, fmt.Sprintf("你 %d", d.PlayerScore)).Background(hud.Player.Bg).TextColor(hud.Player.Text)
			ui.Badge(c, fmt.Sprintf("高松灯 %d", d.AIScore)).Background(hud.AI.Bg).TextColor(hud.AI.Text)
			ui.Badge(c, "剩余 "+formatClock(int(d.remaining()))).Background(c.Theme().Accent.Alpha(0.18))
		})
		now := c.Now()
		ui.Row(c).Gap(6).Children(func() {
			if ui.Button(c.Key("duel-home"), "回到中心").Clicked() {
				a.board.glideCenterBoard(a.game, now, a.motionReducedActive)
			}
			if ui.Button(c.Key("duel-follow"), "定位高松灯").Clicked() && d.LastAI >= 0 {
				a.board.glideCenter(a.game, d.LastAI/a.game.Cols, d.LastAI%a.game.Cols, now, a.motionReducedActive)
			}
			if ui.Button(c.Key("duel-zoom-out"), "−").Label("缩小").Clicked() {
				a.board.glideZoom(a.game, a.board.Width/2, a.board.Height/2, 1/1.2, now, a.motionReducedActive)
			}
			if ui.Button(c.Key("duel-zoom-in"), "+").Label("放大").Clicked() {
				a.board.glideZoom(a.game, a.board.Width/2, a.board.Height/2, 1.2, now, a.motionReducedActive)
			}
		})
	})
	ui.Row(c).Padding(4, 8).Gap(8).Children(func() {
		ui.Badge(c, fmt.Sprintf("连击 ×%d", d.PlayerCombo))
		if d.playerStunned() {
			ui.Badge(c, fmt.Sprintf("硬直 %.1f 秒", d.StunnedUntil-d.Elapsed)).Background(hud.Stun.Bg).TextColor(hud.Stun.Text)
		} else {
			ui.Badge(c, "确定全雷时左键点数字收雷 · +75/雷")
		}
		if d.Elapsed < d.ScanUntil {
			ui.Badge(c, fmt.Sprintf("扫描 %.1f 秒", d.ScanUntil-d.Elapsed)).Background(hud.Scan.Bg).TextColor(hud.Scan.Text)
		}
		ui.Badge(c, fmt.Sprintf("插旗 %d", a.game.FlagsPlaced))
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

func (a *minesweeperApp) toggleFlagMode() {
	a.flagMode = !a.flagMode
	a.playSound(SoundToggle, 1, 0)
}

func (a *minesweeperApp) view(c *ui.Context) {
	now := c.Now()
	defer a.flushChatNote()
	defer a.scheduleCues(c, now)
	a.syncWindowFocus(c)
	a.flushCues(now)
	a.systemReduceMotion = c.Preferences().ReduceMotion
	a.syncMotionState(a.reducedMotion(c))
	if a.pendingResult.Kind != ActionNoop {
		res := a.pendingResult
		a.pendingResult = ActionResult{}
		// Only canvas cell moves queue a result; their foley already played.
		posted := a.chatPosted
		a.handleAction(c, res, now)
		if _, voiced := cellActionSound(res.Kind); voiced {
			a.chatVoicedSince(posted)
		}
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
		a.toggleFlagMode()
	}
	if c.Shortcut(0, ui.KeyE) {
		a.useSkill(c, now)
	}
	if c.Shortcut(0, ui.KeyM) && !c.ShortcutRepeated() {
		a.toggleMute()
	}
	if a.isDuel() || a.classicOnCanvas() {
		if c.Shortcut(ui.Ctrl, ui.KeyEqual) {
			a.board.glideZoom(a.game, a.board.Width/2, a.board.Height/2, 1.15, now, a.motionReducedActive)
		}
		if c.Shortcut(ui.Ctrl, ui.KeyMinus) {
			a.board.glideZoom(a.game, a.board.Width/2, a.board.Height/2, 1/1.15, now, a.motionReducedActive)
		}
		if c.Shortcut(ui.Ctrl, ui.Key0) {
			a.board.glideCenterBoard(a.game, now, a.motionReducedActive)
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
			drama := haruhikageFor(pal.Dark)
			ui.Overlay(c, func() {
				overlay := ui.Box(c.Key("haruhikage-drama-overlay")).Fill().Modal().
					Background(drama.Scrim).Center().
					Cursor(ui.CursorPointer).Focusable().FocusRing(false).Label("春日影演出遮罩")
				if a.focusDuelSettlement {
					a.focusDuelSettlement = false
					overlay.Focus()
				}
				overlay.Children(func() {
					ui.Column(c).AlignItems(ui.Center).Gap(10).Padding(26, 32).Radius(6).
						Background(drama.CardBg).Border(1.5, drama.CardBorder).
						Shadow(0, 12, 32, 0, drama.CardShadow).
						Draw(func(p *ui.Painter, r ui.Rect) {
							p.Clip(r, 6, func() {
								p.Fill(ui.Rect{X: r.X, Y: r.Y, W: r.W, H: 3}, drama.AccentBar, 0)
								for y := r.Y + 24; y < r.Y+r.H-8; y += 22 {
									p.Line(r.X+14, y, r.X+r.W-14, y, 1, drama.CardRule)
								}
							})
						}).
						Children(func() {
							ui.Text(c, "♪ 《春日影》 — 乐声骤停").FontSize(13).Bold().TextColor(drama.Eyebrow)
							ui.Text(c, "「……为什么要演奏春日影？！」").FontSize(22).Bold().TextColor(drama.Title)
							ui.Text(c, "（长崎爽世的眼神失去了高光 · 点击任意处继续）").FontSize(12).TextColor(drama.Subtitle)
						})
				})
				if overlay.Clicked() {
					a.haruhikageIntro = false
					a.haruhikageAlert = true
				}
				if overlay.OverlayShortcut(0, ui.KeyM) && !c.ShortcutRepeated() {
					a.toggleMute()
				}
			})
		}
	}

	// 春日影名场面弹窗
	// Scope AlertDialog via a zero-size ui.Themed container so mygo/ui's
	// overlay binds the dialog theme (focus ring, text, buttons) locally
	// instead of falling back to the window theme.
	ui.Themed(c, haruhikageFor(pal.Dark).dialogTheme(baseTheme)).Absolute().Size(0, 0).Children(func() {
		switch ui.AlertDialog(c, &a.haruhikageAlert,
			"为什么要演奏春日影？！",
			"「明明说好不再弹这首曲子的……」\n（长崎爽世失去了高光，并把你移出了排练室群聊）",
			"对不起，我错了", "一辈子重开") {
		case 0:
			a.pushChat("长崎爽世", FindMember(MemberSoyorin).ColorHex, "「道歉的话，下次别再擅自演奏这首曲子了呢。」")
		case 1:
			a.resetGame(false)
		}
	})
	if a.haruhikageAlert {
		// Modal dialogs block root shortcuts; keep unmodified M mute active above AlertDialog.
		ui.Overlay(c, func() {
			if ui.Box(c.Key("haruhikage-alert-mute")).Size(0, 0).PassThrough().
				OverlayShortcut(0, ui.KeyM) && !c.ShortcutRepeated() {
				a.toggleMute()
			}
		})
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
				a.settingsControl(c, pal)
			})
		})

		// Setlist & Toolbar Row
		ui.Row(c).Padding(8, 12).Radius(10).Background(t.Surface).Border(1, t.Border).
			AlignItems(ui.Center).Justify(ui.SpaceBetween).Gap(10).Wrap().Children(func() {
			a.setlistControl(c, pal, t)

			ui.ToggleGroup(c, func() {
				flagMode := a.flagMode
				if ui.Toggle(c, &flagMode, "插旗模式 (F)").Tooltip("开启后左键直接贴创可贴插旗").Changed() {
					a.toggleFlagMode()
				}
				if ui.Toggle(c, &a.showMemes, "名场面图鉴").Tooltip("展开或收起已解锁的 MyGO!!!!! 名场面").Changed() {
					a.playSound(SoundToggle, 1, 0)
				}
			}).Label("工具开关")
		})

		// Main 2-Column Workspace: Left Sidebar + Right Stage
		ui.Row(c).Grow(1).Gap(12).AlignItems(ui.Stretch).Children(func() {
			// Left Sidebar: Member Roster + Live Meters + Band Group Chat
			ui.Column(c).Width(272).Padding(10).Gap(10).Radius(12).
				Background(t.Surface).Border(1, t.Border).Children(func() {
				ui.Text(c, "乐队成员").FontSize(13).Bold().TextColor(t.TextMuted)

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
					hatchProgress(c, "排练完成度", a.game.Progress(), pal.InkOf(bandColor), pal)

					gravityInk := pal.InkOf(memberImage(MemberSoyorin))
					ui.Row(c).Justify(ui.SpaceBetween).Children(func() {
						ui.Text(c, "乐队重力值").FontSize(12).TextColor(t.TextMuted)
						ui.Textf(c, "%d%%", a.game.Gravity).FontSize(12).Bold().TextColor(gravityInk)
					})
					hatchMeter(c, "乐队重力值", float64(a.game.Gravity), gravityInk, pal)
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
										skillTip = "纯正经典：关闭技能，仅保留台词应援"
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
							a.useSkill(c, now)
						}
					})
				})

				// Collapsible Meme Achievements Panel
				if a.showMemes {
					ui.Column(c).Padding(10, 12).Gap(8).Radius(12).Background(t.Surface).Border(1, t.Border).Children(func() {
						ui.Row(c).AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
							ui.Text(c, "MyGO!!!!! 名场面图鉴").FontSize(13).Bold()
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
							ui.Badge(c, "剩雷 "+formatDigits(a.game.RemainingMines())).
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
						resetBtn := ui.Button(c.Key("reset-btn"), resetLabel)
						if a.focusDuelSettlement && !a.haruhikageIntro {
							a.focusDuelSettlement = false
							resetBtn.Focus()
						}
						if resetBtn.Clicked() {
							a.resetGame(false)
						}

						ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
							bestText := "最快 --"
							if best, ok := a.bestTimes[a.game.Difficulty.ID]; ok {
								bestText = fmt.Sprintf("最快 %d 秒", best)
							}
							ui.Badge(c, bestText)
							ui.Badge(c, "用时 "+formatDigits(a.elapsed)+" 秒").
								Background(t.Accent.Alpha(0.18)).TextColor(t.Accent).FontFeatures("tnum")
						})
					})

					if a.isDuel() {
						a.duelHUD(c)
						a.duelBoard(c)
						return
					}
					if a.classicOnCanvas() {
						a.classicBoard(c)
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
										a.cellMove(c, row, col, 1, now)
									} else if tile.Clicked() {
										a.cellMove(c, row, col, 0, now)
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

			a.popoverWorkspaceShield(c)
		})
	})
}

func main() {
	app := newDesktopApp(os.UserConfigDir, savePrefs, newDesktopSound(NewSoundBank()))
	mygo.App.WhenReady(func() {
		w := mygo.NewWindow(mygo.WindowOptions{
			Title:           "MyGO!!!!! 迷子扫雷",
			Width:           1280,
			Height:          900,
			BackgroundColor: hexOf(app.paper().Desk),
			StateKey:        "mygo-minesweeper-v2",
			Content:         ui.View(app.view),
		})
		w.OnFocus(func() {
			app.setWindowFocused(true)
		})
		w.OnBlur(func() {
			app.setWindowFocused(false)
		})
		_ = w.SetIcon(appIconPNG)
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
