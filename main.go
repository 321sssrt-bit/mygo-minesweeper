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
	peelDuration            = 150 * time.Millisecond
	rippleDuration          = 280 * time.Millisecond
	bandageBounceDuration   = 140 * time.Millisecond
	bandagePeelDuration     = 110 * time.Millisecond
	maxBandageAnims         = MaxRevealWaveEntries
	duelPopupDuration       = 950 * time.Millisecond
	maxDuelPopups           = 16
	quoteStripDuration      = 1200 * time.Millisecond
	memeStripDuration       = 2400 * time.Millisecond
	skillPulseDuration      = 720 * time.Millisecond
	knotBurstDuration       = 520 * time.Millisecond
	maxKnotBursts           = 8
	confettiDuration        = 1800 * time.Millisecond
	confettiCount           = 32
	haruhikageIntroDuration    = 1600 * time.Millisecond
	haruhikageScrimFade        = 260 * time.Millisecond
	haruhikageBeatDelay        = 250 * time.Millisecond
	haruhikageLineFade         = 240 * time.Millisecond
	chatNoteEnterDuration      = 220 * time.Millisecond
	chatNoteReducedFade        = 120 * time.Millisecond
	popoverEnterDuration       = 140 * time.Millisecond
	popoverExitDuration        = 110 * time.Millisecond
	popoverReducedFadeDuration = 110 * time.Millisecond
	drawerOpenDuration         = 180 * time.Millisecond
	drawerCloseDuration        = 150 * time.Millisecond
	drawerReducedFadeDuration  = 110 * time.Millisecond
	memberHighlightDuration    = 180 * time.Millisecond
	lampTransitionDuration     = 240 * time.Millisecond
	meterGlideDuration         = 220 * time.Millisecond
)

// waveSteps is the last step of a reveal wave; deeper cells share it.
const waveSteps = int(rippleMaxDelay / rippleStepDelay)

// waveStep is the step at which a reveal wave reaches a cell depth away.
func waveStep(depth int) int { return min(depth, waveSteps) }

type chatMessage struct {
	ID       int
	Speaker  string
	ColorHex string
	Text     string
}

type chatNoteKey int

// smoothScalar eases a displayed value toward its real target across frames
// while letting callers snap immediately on initial mount or reduced motion.
type smoothScalar struct {
	init   bool
	from   float64
	value  float64
	target float64
	start  time.Time
}

func (s *smoothScalar) snap(v float64) {
	s.init = true
	s.from = v
	s.value = v
	s.target = v
	s.start = time.Time{}
}

func (s *smoothScalar) update(target float64, now time.Time, dur time.Duration, reduced bool) (float64, bool) {
	if !s.init || reduced {
		s.snap(target)
		return target, false
	}
	if target != s.target {
		if !s.start.IsZero() {
			s.sample(now, dur)
		}
		s.from = s.value
		s.target = target
		if math.Abs(s.target-s.from) < 1e-4 {
			s.snap(target)
			return target, false
		}
		s.start = now
		return s.value, true
	}
	if s.start.IsZero() {
		return s.value, false
	}
	return s.sample(now, dur)
}

func (s *smoothScalar) sample(now time.Time, dur time.Duration) (float64, bool) {
	elapsed := now.Sub(s.start)
	if elapsed <= 0 {
		return s.from, true
	}
	if elapsed >= dur {
		s.snap(s.target)
		return s.value, false
	}
	u := float64(elapsed) / float64(dur)
	ease := 1 - (1-u)*(1-u)*(1-u)
	s.value = s.from + (s.target-s.from)*ease
	return s.value, true
}

type tileRipple struct {
	Start time.Time
	Delay time.Duration
	// Color locks the selected member's 代表色 when a single move reveals >= 10 cells;
	// zero falls back to the board's default wash colour.
	Color ui.Color
}

type tilePeelStage uint8

const (
	tilePeelNone tilePeelStage = iota
	tilePeelCovered
	tilePeelLifting
	tilePeelWashOnly
)

type tilePeelState struct {
	stage     tilePeelStage
	peelT     float32
	washAlpha float32
	washColor ui.Color
}

type bandageAnimKind uint8

const (
	bandageBounce bandageAnimKind = iota + 1
	bandagePeel
)

type bandageAnim struct {
	Start  time.Time
	Kind   bandageAnimKind
	Shield bool
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

type stageQuoteStrip struct {
	Active   bool
	MemberID MemberID
	Text     string
	Start    time.Time
}

type stageMemeStrip struct {
	Active bool
	Memes  []MemeAchievement
	Start  time.Time
}

type skillTargetPulse struct {
	Index    int
	MemberID MemberID
	Start    time.Time
}

type knotBurstPulse struct {
	Index int
	Start time.Time
}

type confettiPiece struct {
	MemberID MemberID
	StartX   float32
	StartY   float32
	DriftX   float32
	FallY    float32
	SwayFreq float32
	SwayAmp  float32
	Phase    float32
	Rot0     float32
	RotSpeed float32
	W, H     float32
	Exclaim  bool
	Delay    time.Duration
}

type pendingCellAction struct {
	result ActionResult
	row    int
	col    int
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
	pendingActions       []pendingCellAction
	duelNoticeAt         time.Time
	duelEnded            bool
	classicSize          float32
	ripples              map[int]tileRipple
	bandages             map[int]bandageAnim
	duelPopups           []duelPopup
	quoteStrip           stageQuoteStrip
	memeStrip            stageMemeStrip
	skillPulses          []skillTargetPulse
	knotBursts           []knotBurstPulse
	confetti             []confettiPiece
	confettiStart        time.Time
	shakeStart           time.Time
	shakeAmplitude       float32
	shakeDuration        time.Duration
	haruhikageIntroStart time.Time
	classicHoverRow      int
	classicHoverCol      int
	classicHoverHint     HoverHint
	gridPress            gridPointerPress

	viewMounted       bool
	lastFrameNow      time.Time
	popoverEnterKind  appPopover
	popoverEnterStart time.Time
	popoverExitKind   appPopover
	popoverExitStart  time.Time
	popoverExitDur    time.Duration
	popoverExitRect   ui.Rect
	popoverRects      [3]ui.Rect

	memeDrawerInit   bool
	memeDrawerOpen   bool
	memeDrawerFrom   float32
	memeDrawerValue  float32
	memeDrawerStart  time.Time
	memeDrawerDur    time.Duration
	memeDrawerHeight float32

	nightBlendInit   bool
	nightBlendFrom   float32
	nightBlendValue  float32
	nightBlendTarget float32
	nightBlendStart  time.Time

	progressVisual smoothScalar
	gravityVisual  smoothScalar

	chatViewInit   bool
	chatLastSeenID int
	chatAnims      map[int]time.Time
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
		a.bandages = nil
		a.skillPulses = nil
		a.knotBursts = nil
		a.confetti = nil
		a.confettiStart = time.Time{}
		a.shakeStart = time.Time{}
		a.shakeAmplitude = 0
		a.shakeDuration = 0
		a.board.finishGlide(a.game)
		if a.game != nil {
			if a.progressVisual.init {
				a.progressVisual.snap(a.game.Progress())
			}
			if a.gravityVisual.init {
				a.gravityVisual.snap(float64(a.game.Gravity))
			}
		}
		if a.memeDrawerInit && !a.memeDrawerOpen && !a.memeDrawerStart.IsZero() {
			a.memeDrawerValue = 0
			a.memeDrawerFrom = 0
			a.memeDrawerStart = time.Time{}
		}
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
	startExit := func(closing appPopover) {
		if closing == popoverNone {
			return
		}
		r := a.popoverRects[closing]
		if r.W <= 0 || r.H <= 0 {
			return
		}
		a.popoverExitKind = closing
		a.popoverExitRect = r
		// Anchor on the first rendered exit frame rather than a potentially
		// stale lastFrameNow after an idle pause (>110ms) before an outside click.
		a.popoverExitStart = time.Time{}
		a.popoverExitDur = popoverExitDuration
		if a.motionReducedActive {
			a.popoverExitDur = popoverReducedFadeDuration
		}
	}
	switch {
	case kind == prev:
	case kind == popoverNone:
		if a.viewMounted {
			startExit(prev)
			a.popoverEnterKind = popoverNone
			a.popoverEnterStart = time.Time{}
		}
		a.playSound(SoundPopoverClose, 1, 0)
	default:
		a.gridPress.cancel()
		if a.viewMounted {
			if prev != popoverNone {
				startExit(prev)
			} else if a.popoverExitKind == kind {
				a.popoverExitKind = popoverNone
				a.popoverExitStart = time.Time{}
				a.popoverExitRect = ui.Rect{}
			}
			a.popoverEnterKind = kind
			a.popoverEnterStart = a.lastFrameNow
		}
		a.playSound(SoundPopoverOpen, 1, 0)
	}
}

func (a *minesweeperApp) popoverExiting(now time.Time) bool {
	if a.popoverExitKind == popoverNone {
		return false
	}
	if a.popoverExitStart.IsZero() {
		return true
	}
	elapsed := now.Sub(a.popoverExitStart)
	if elapsed < 0 || elapsed >= a.popoverExitDur {
		a.popoverExitKind = popoverNone
		a.popoverExitStart = time.Time{}
		a.popoverExitRect = ui.Rect{}
		return false
	}
	return true
}

func (a *minesweeperApp) clearPopoverMotion() {
	a.popoverEnterKind = popoverNone
	a.popoverEnterStart = time.Time{}
	a.popoverExitKind = popoverNone
	a.popoverExitStart = time.Time{}
	a.popoverExitRect = ui.Rect{}
}

func (a *minesweeperApp) applyPopoverMotion(c *ui.Context, popover ui.Element, kind appPopover, now time.Time) {
	if a.popoverEnterKind != kind || a.popoverEnterStart.IsZero() {
		return
	}
	dur := popoverEnterDuration
	if a.motionReducedActive {
		dur = popoverReducedFadeDuration
	}
	elapsed := now.Sub(a.popoverEnterStart)
	if elapsed < 0 {
		elapsed = 0
	}
	if elapsed < dur {
		u := float32(elapsed) / float32(dur)
		ease := 1 - (1-u)*(1-u)*(1-u)
		if a.motionReducedActive {
			popover.Opacity(0.35 + 0.65*u)
		} else {
			scale := 0.96 + 0.04*ease
			offsetY := -8 * (1 - ease)
			popover.Opacity(0.18 + 0.82*ease).
				Transform(ui.Scaling(scale, scale).Then(ui.Translation(0, offsetY)))
		}
		c.AnimationFrame()
		return
	}
	a.popoverEnterKind = popoverNone
	a.popoverEnterStart = time.Time{}
}

// renderPopoverExitGhost draws an inert fading sheet for the popover that just
// closed, driven directly by app three-state motion rather than mygo's
// system-only Element.Transition so motionStandard overrides system
// ReduceMotion, mid-exit switches to motionReduced drop displacement on the
// same frame, and resetGame/settleDuel clear the ghost immediately.
func (a *minesweeperApp) renderPopoverExitGhost(c *ui.Context, pal *palette, t *ui.Theme, now time.Time) {
	if a.popoverExitKind == popoverNone {
		return
	}
	if a.popoverExitStart.IsZero() {
		a.popoverExitStart = now
	}
	if !a.popoverExiting(now) {
		return
	}
	r := a.popoverExitRect
	if r.W <= 0 || r.H <= 0 {
		return
	}
	u := float32(now.Sub(a.popoverExitStart)) / float32(a.popoverExitDur)
	ease := u * u
	kind := a.popoverExitKind
	c.AnimationFrame()
	ui.Overlay(c, func() {
		ghost := ui.Box(c.Key("popover-exit-ghost")).
			Absolute().Left(r.X).Top(r.Y).Size(r.W, r.H).
			Radius(t.Radius + 2).
			Background(t.Surface).
			Border(1, t.Border).
			Shadow(0, 6, 20, 0, popoverExitShadow).
			IgnoreInput(true).AccessHidden(true).PassThrough()
		if a.motionReducedActive {
			ghost.Opacity(1 - u)
		} else {
			scale := 1 - 0.04*ease
			offsetY := -8 * ease
			ghost.Opacity(1 - ease).
				Transform(ui.Scaling(scale, scale).Then(ui.Translation(0, offsetY)))
		}
		ghost.Draw(func(p *ui.Painter, box ui.Rect) {
			a.paintPopoverExitContents(p, box, kind, pal, t)
		})
	})
}

func (a *minesweeperApp) paintPopoverExitContents(p *ui.Painter, box ui.Rect, kind appPopover, pal *palette, t *ui.Theme) {
	pad := t.Space(1)
	inner := ui.Rect{X: box.X + pad, Y: box.Y + pad, W: max(0, box.W-pad*2), H: max(0, box.H-pad*2)}
	p.Clip(box, t.Radius+2, func() {
		switch kind {
		case popoverSetlist:
			y := inner.Y + 4
			p.Text(inner.X+6, y, "排练歌单", 12, t.TextMuted)
			p.Line(inner.X, y+20, inner.X+inner.W, y+20, 1, pal.Rule)
			y += 28
			groupInk := setlistAccentInk(pal, t.Surface)
			selectedBg := t.Accent.Alpha(0.14).Over(t.Surface)
			selectedInk := setlistAccentInk(pal, selectedBg)
			for gIdx, group := range groupSetlistTracks(Difficulties) {
				if gIdx > 0 {
					p.Line(inner.X+4, y, inner.X+inner.W-4, y, 1, pal.Rule)
					y += 8
				}
				p.Fill(ui.Rect{X: inner.X + 6, Y: y + 2, W: 3, H: 11}, groupInk, 1.5)
				p.Text(inner.X+15, y, group.Title, 11, groupInk)
				y += 20
				for _, entry := range group.Entries {
					rowH := float32(36)
					if y+rowH > inner.Y+inner.H {
						return
					}
					if entry.Index == a.diffIdx {
						p.Fill(ui.Rect{X: inner.X, Y: y, W: inner.W, H: rowH}, selectedBg, 6)
						p.Text(inner.X+30, y+5, entry.Diff.Song, 13, selectedInk)
					} else {
						p.Text(inner.X+30, y+5, entry.Diff.Song, 13, t.Text)
					}
					p.Text(inner.X+30, y+21, setlistTrackSpec(entry.Diff), 11, t.TextMuted)
					if entry.Diff.Gameplay != GameplayDuel {
						bestInk := t.TextMuted
						if _, ok := a.bestTimes[entry.Diff.ID]; ok {
							bestInk = t.Text
						}
						best := ui.Span{Text: a.bestTimeText(entry.Diff.ID), Size: 11, Color: bestInk, Features: "tnum"}
						bw, _ := p.MeasureText(0, best)
						p.RichText(inner.X+inner.W-31-bw, y+21, 0, best)
					}
					y += rowH + 2
				}
			}
		case popoverSettings:
			sc := settingsColorsForPaper(pal)
			drawSection := func(y float32, title string, rule bool) float32 {
				if rule {
					p.Line(inner.X+4, y, inner.X+inner.W-4, y, 1, pal.Rule)
					y += 10
				}
				p.Fill(ui.Rect{X: inner.X + 4, Y: y + 2, W: 3, H: 11}, sc.GroupInk, 1.5)
				p.Text(inner.X+13, y, title, 11, sc.GroupInk)
				return y + 22
			}
			drawSwitch := func(y float32, on bool) {
				tr := ui.Rect{X: inner.X + inner.W - 42, Y: y + 1, W: 36, H: 20}
				trackCol := sc.SegTrackBg
				knobX := tr.X + 3
				if on {
					trackCol = t.Accent
					knobX = tr.X + tr.W - 17
				}
				p.Fill(tr, trackCol, 10)
				p.Stroke(tr, pal.Rule, 10, 1)
				p.Fill(ui.Rect{X: knobX, Y: tr.Y + 3, W: 14, H: 14}, pal.Sheet, 7)
			}

			y := inner.Y + 4
			p.Text(inner.X+6, y, settingsPanelTitle, 12, sc.MutedInk)
			escSpan := ui.Span{Text: settingsDismissHint, Size: 11, Color: sc.MutedInk}
			tw, _ := p.MeasureText(0, escSpan)
			p.RichText(inner.X+inner.W-6-tw, y+1, 0, escSpan)
			p.Line(inner.X, y+20, inner.X+inner.W, y+20, 1, pal.Rule)
			y += 30

			// Group 1: Display & Motion
			y = drawSection(y, "画面与动效", false)
			p.Text(inner.X+6, y+3, "夜间台灯", 13, sc.LabelInk)
			drawSwitch(y, a.darkMode)
			y += 28

			p.Text(inner.X+6, y+2, "动效", 13, sc.LabelInk)
			y += 22
			segTrack := ui.Rect{X: inner.X + 4, Y: y, W: inner.W - 8, H: 26}
			p.Fill(segTrack, sc.SegTrackBg, 7)
			p.Stroke(segTrack, pal.Rule, 7, 1)
			motionLabels := [3]string{"跟随系统", "标准", "减弱"}
			motionIdx := motionToIndex(a.motion)
			segW := (segTrack.W - 8) / 3
			for i, lbl := range motionLabels {
				sr := ui.Rect{X: segTrack.X + 2 + float32(i)*(segW+2), Y: segTrack.Y + 2, W: segW, H: segTrack.H - 4}
				txtCol := sc.SegIdleInk
				if i == motionIdx {
					p.Fill(sr, sc.TriggerOpenBg, 5)
					txtCol = sc.TriggerOpenInk
				}
				span := ui.Span{Text: lbl, Size: 12, Color: txtCol}
				tw, th := p.MeasureText(0, span)
				p.RichText(sr.X+(sr.W-tw)/2, sr.Y+(sr.H-th)/2, 0, span)
			}
			y += 36

			// Group 2: Sound
			y = drawSection(y, "声音", true)
			p.Text(inner.X+6, y+3, "音效", 13, sc.LabelInk)
			keyBox := ui.Rect{X: inner.X + 40, Y: y + 3, W: 20, H: 16}
			p.Fill(keyBox, sc.KeyBg, 4)
			p.Stroke(keyBox, pal.Rule, 4, 1)
			p.Text(keyBox.X+5, keyBox.Y+2, "M", 10.5, sc.KeyInk)
			drawSwitch(y, a.soundEnabled)
			y += 28

			volLabelInk, volValueInk, sliderFill := sc.LabelInk, sc.GroupInk, t.Accent
			if !a.soundEnabled {
				volLabelInk, volValueInk, sliderFill = sc.MutedInk, sc.MutedInk, sc.MutedInk.Alpha(0.45)
			}
			p.Text(inner.X+6, y+2, "音量", 13, volLabelInk)
			volText := settingsVolumeReadout(a.soundEnabled, a.volume)
			volSpan := ui.Span{Text: volText, Size: 12, Weight: 700, Color: volValueInk, Features: "tnum"}
			vw, _ := p.MeasureText(0, volSpan)
			p.RichText(inner.X+inner.W-6-vw, y+2, 0, volSpan)
			y += 24
			slTrack := ui.Rect{X: inner.X + 6, Y: y, W: inner.W - 12, H: 4}
			p.Fill(slTrack, sc.SegTrackBg, 2)
			frac := max(float32(0), min(float32(1), float32(a.volume)/100))
			fillW := slTrack.W * frac
			if fillW > 0 {
				p.Fill(ui.Rect{X: slTrack.X, Y: slTrack.Y, W: fillW, H: slTrack.H}, sliderFill, 2)
			}
			knobCX := slTrack.X + fillW
			p.Fill(ui.Rect{X: knobCX - 6, Y: slTrack.Y - 4, W: 12, H: 12}, sliderFill, 6)
			y += 20

			// Group 3: Gameplay
			y = drawSection(y, "扫雷操作", true)
			if y+22 <= inner.Y+inner.H {
				p.Text(inner.X+6, y+3, "右键 ? 标记", 13, sc.LabelInk)
				drawSwitch(y, a.allowQuestion)
				if y+38 <= inner.Y+inner.H {
					p.Text(inner.X+6, y+25, settingsQuestionHint, 11, sc.MutedInk)
				}
			}
		}
	})
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
	now := c.Now()
	a.lastFrameNow = now
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
		popover.DrawOver(func(_ *ui.Painter, r ui.Rect) {
			a.popoverRects[kind] = r
		})
		a.applyPopoverMotion(c, popover, kind, now)
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
// outside dismiss click never leaks onto a board cell or mine, while leaving
// the board immediately playable once a popover is dismissed (such as via
// Escape) as its inert exit ghost finishes fading out.
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
	a.chatPosted++
	a.chatLog = append(a.chatLog, chatMessage{
		ID:       a.chatPosted,
		Speaker:  speaker,
		ColorHex: colorHex,
		Text:     text,
	})
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
	a.chatAnims = nil
	a.chatViewInit = false
	if !initial {
		a.seed = 0
	} else {
		a.progressVisual = smoothScalar{}
		a.gravityVisual = smoothScalar{}
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
	a.pendingActions = nil
	a.duelEnded = false
	a.duelNoticeAt = time.Time{}
	a.classicSize = 0
	a.board = newBoardView()
	a.ripples = nil
	a.bandages = nil
	a.clearCues()
	a.duelTickFocused = false
	a.duelPopups = nil
	a.quoteStrip = stageQuoteStrip{}
	a.memeStrip = stageMemeStrip{}
	a.skillPulses = nil
	a.knotBursts = nil
	a.confetti = nil
	a.confettiStart = time.Time{}
	a.shakeStart = time.Time{}
	a.shakeAmplitude = 0
	a.shakeDuration = 0
	a.haruhikageAlert = false
	a.haruhikageIntro = false
	a.haruhikageIntroStart = time.Time{}
	a.focusDuelSettlement = false
	a.dismissPopovers()
	a.clearPopoverMotion()
	a.classicHoverRow = -1
	a.classicHoverCol = -1
	a.classicHoverHint = HoverHint{}
	a.gridPress.cancel()
	a.gridPress.clearSuppressed()
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

func (a *minesweeperApp) enqueueRipple(now time.Time, wave []RevealWaveEntry, lockedColor ui.Color) {
	if a.motionReducedActive || len(wave) == 0 {
		return
	}
	a.pruneRipples(now)
	if a.ripples == nil {
		a.ripples = make(map[int]tileRipple, len(wave))
	}
	needed := 0
	for _, entry := range wave {
		if _, exists := a.ripples[entry.Index]; !exists {
			needed++
		}
	}
	if overflow := len(a.ripples) + needed - MaxRevealWaveEntries; overflow > 0 {
		type candidate struct {
			idx      int
			washOnly bool
			start    time.Time
			delay    time.Duration
		}
		cands := make([]candidate, 0, len(a.ripples))
		for idx, rip := range a.ripples {
			elapsed := now.Sub(rip.Start)
			cands = append(cands, candidate{
				idx:      idx,
				washOnly: elapsed >= rip.Delay+peelDuration,
				start:    rip.Start,
				delay:    rip.Delay,
			})
		}
		// Prefer evicting older wash-only entries first, then the furthest-delayed
		// waiting slips of older waves so a brand-new move always gets its peel.
		slices.SortFunc(cands, func(x, y candidate) int {
			if x.washOnly != y.washOnly {
				if x.washOnly {
					return -1
				}
				return 1
			}
			if !x.start.Equal(y.start) {
				if x.start.Before(y.start) {
					return -1
				}
				return 1
			}
			if x.delay != y.delay {
				if x.delay > y.delay {
					return -1
				}
				return 1
			}
			return x.idx - y.idx
		})
		for i := 0; i < overflow && i < len(cands); i++ {
			delete(a.ripples, cands[i].idx)
		}
	}
	for _, entry := range wave {
		delete(a.bandages, entry.Index)
		delay := time.Duration(waveStep(entry.Depth)) * rippleStepDelay
		a.ripples[entry.Index] = tileRipple{Start: now, Delay: delay, Color: lockedColor}
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

func (a *minesweeperApp) tilePeelAt(idx int, now time.Time) tilePeelState {
	if a.motionReducedActive {
		return tilePeelState{}
	}
	rip, ok := a.ripples[idx]
	if !ok {
		return tilePeelState{}
	}
	elapsed := now.Sub(rip.Start)
	if elapsed < 0 || elapsed >= rip.Delay+rippleDuration {
		return tilePeelState{}
	}
	if elapsed < rip.Delay {
		return tilePeelState{stage: tilePeelCovered, washColor: rip.Color}
	}
	since := elapsed - rip.Delay
	washAlpha := 1 - float32(since)/float32(rippleDuration)
	if since < peelDuration {
		return tilePeelState{
			stage:     tilePeelLifting,
			peelT:     float32(since) / float32(peelDuration),
			washAlpha: washAlpha,
			washColor: rip.Color,
		}
	}
	return tilePeelState{
		stage:     tilePeelWashOnly,
		washAlpha: washAlpha,
		washColor: rip.Color,
	}
}

func (a *minesweeperApp) enqueueBandage(now time.Time, idx int, kind bandageAnimKind, shield bool) {
	if a.motionReducedActive {
		return
	}
	a.pruneBandages(now)
	if a.bandages == nil {
		a.bandages = make(map[int]bandageAnim, 8)
	}
	if _, exists := a.bandages[idx]; !exists && len(a.bandages) >= maxBandageAnims {
		return
	}
	a.bandages[idx] = bandageAnim{Start: now, Kind: kind, Shield: shield}
}

func (a *minesweeperApp) pruneBandages(now time.Time) {
	for idx, b := range a.bandages {
		elapsed := now.Sub(b.Start)
		dur := bandageBounceDuration
		if b.Kind == bandagePeel {
			dur = bandagePeelDuration
		}
		if elapsed < 0 || elapsed >= dur {
			delete(a.bandages, idx)
		}
	}
}

func (a *minesweeperApp) bandageAnimAt(idx int, now time.Time) (bandageAnim, bool) {
	if a.motionReducedActive {
		return bandageAnim{}, false
	}
	b, ok := a.bandages[idx]
	if !ok {
		return bandageAnim{}, false
	}
	elapsed := now.Sub(b.Start)
	dur := bandageBounceDuration
	if b.Kind == bandagePeel {
		dur = bandagePeelDuration
	}
	if elapsed < 0 || elapsed >= dur {
		return bandageAnim{}, false
	}
	return b, true
}

// bandageBounceTransform returns the explicit short-period spring scale and
// diagonal slap tilt for a newly placed band-aid, converging to (1, 0) at
// bandageBounceDuration without relying on a long default spring.
func bandageBounceTransform(elapsed time.Duration) (scale float32, rotDeg float32) {
	if elapsed < 0 || elapsed >= bandageBounceDuration {
		return 1, 0
	}
	u := float64(elapsed) / float64(bandageBounceDuration)
	env := math.Exp(-2.6*u) * (1 - u)
	osc := math.Cos(u * 2 * math.Pi * 0.95)
	impulse := env * osc
	return float32(1 + 0.25*impulse), float32(10 * impulse)
}

func tileSlipTilt(cellIndex int) float32 {
	return float32((cellIndex*37)%7-3) * 0.22
}

// paintPeelOverlay draws a waiting or lifting paper slip decoration over an
// already-revealed cell without shifting its hit-test or layout bounds.
func paintPeelOverlay(
	p *ui.Painter,
	tile ui.Rect,
	radius float32,
	bg, shade ui.Color,
	baseTiltDeg float32,
	cellIndex int,
	lifting bool,
	peelT float32,
	detail bool,
) {
	cx, cy := tile.X+tile.W/2, tile.Y+tile.H/2
	if !lifting {
		if baseTiltDeg != 0 {
			p.Transform(ui.Rotation(baseTiltDeg), cx, cy, func() {
				if detail {
					p.Shadow(tile, radius, 0, 1, 1.5, 0, shade)
				}
				p.Fill(tile, bg, radius)
			})
			return
		}
		if detail {
			p.Shadow(tile, radius, 0, 1, 1.5, 0, shade)
		}
		p.Fill(tile, bg, radius)
		return
	}

	t := peelT
	ease := 1 - (1-t)*(1-t)*(1-t)
	alpha := 1 - t
	shadeAlpha := (1 - t) * (1 - t)

	dir := float32(1)
	if cellIndex%2 == 0 {
		dir = -1
	}
	liftY := -min(float32(10), max(float32(4), tile.H*0.24)) * ease
	shiftX := dir * min(float32(4), tile.W*0.09) * ease
	rotDeg := baseTiltDeg + dir*9.5*ease
	scale := 1 + 0.08*ease

	xf := ui.Scaling(scale, scale).Then(ui.Rotation(rotDeg)).Then(ui.Translation(shiftX, liftY))
	p.Transform(xf, cx, cy, func() {
		if detail && shadeAlpha > 0.02 {
			p.Shadow(tile, radius, 0, 1+2.5*ease, 1.5+3*ease, 0, shade.Alpha(shadeAlpha))
		}
		if alpha > 0.01 {
			p.Fill(tile, bg.Alpha(alpha), radius)
		}
	})
}

// paintBandagePeel draws a removed band-aid lifting diagonally and fading out
// over the restored paper slip (or ? mark).
func paintBandagePeel(p *ui.Painter, tile ui.Rect, bmp *ui.Bitmap, iconScale, peelT float32) {
	alpha := 1 - peelT
	if alpha <= 0.01 {
		return
	}
	ease := 1 - (1-peelT)*(1-peelT)
	s := min(tile.W, tile.H) * iconScale
	dst := ui.Rect{
		X: tile.X + (tile.W-s)/2,
		Y: tile.Y + (tile.H-s)/2,
		W: s,
		H: s,
	}
	cx, cy := dst.X+dst.W/2, dst.Y+dst.H/2
	dx := tile.W * 0.28 * ease
	dy := -tile.H * 0.24 * ease
	rotDeg := 16 * ease
	scale := 1 + 0.12*ease
	xf := ui.Scaling(scale, scale).Then(ui.Rotation(rotDeg)).Then(ui.Translation(dx, dy))
	p.Transform(xf, cx, cy, func() {
		p.ImageRegion(bmp, ui.Rect{}, dst, ui.ImageOptions{Tint: fadeTint(alpha)})
	})
}

// cellAccessValue returns the immediate frame-0 accessibility value for a cell
// ("0".."8" for revealed safe cells, "?" for questioned slips, "" otherwise).
func cellAccessValue(cell *Cell) string {
	switch {
	case cell.Revealed && !cell.IsMine:
		if cell.Adjacent > 0 {
			return strconv.Itoa(cell.Adjacent)
		}
		return "0"
	case cell.Questioned:
		return "?"
	default:
		return ""
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

func (a *minesweeperApp) showQuoteStrip(memberID MemberID, text string, now time.Time) {
	a.quoteStrip = stageQuoteStrip{
		Active:   true,
		MemberID: memberID,
		Text:     text,
		Start:    now,
	}
}

func (a *minesweeperApp) pruneQuoteStrip(now time.Time) bool {
	if !a.quoteStrip.Active {
		return false
	}
	elapsed := now.Sub(a.quoteStrip.Start)
	if elapsed < 0 || elapsed >= quoteStripDuration {
		a.quoteStrip = stageQuoteStrip{}
		return false
	}
	return true
}

func (a *minesweeperApp) enqueueMemeStrip(meme MemeAchievement, now time.Time) {
	if a.pruneMemeStrip(now) {
		a.memeStrip.Memes = append(a.memeStrip.Memes, meme)
		a.memeStrip.Start = now
		return
	}
	a.memeStrip = stageMemeStrip{
		Active: true,
		Memes:  []MemeAchievement{meme},
		Start:  now,
	}
}

func (a *minesweeperApp) pruneMemeStrip(now time.Time) bool {
	if !a.memeStrip.Active {
		return false
	}
	elapsed := now.Sub(a.memeStrip.Start)
	if elapsed < 0 || elapsed >= memeStripDuration {
		a.memeStrip = stageMemeStrip{}
		return false
	}
	return true
}

func (a *minesweeperApp) enqueueSkillPulses(memberID MemberID, targets []int, now time.Time) {
	if a.motionReducedActive || len(targets) == 0 {
		return
	}
	a.pruneSkillPulses(now)
	for _, idx := range targets {
		a.skillPulses = append(a.skillPulses, skillTargetPulse{
			Index:    idx,
			MemberID: memberID,
			Start:    now,
		})
	}
}

func (a *minesweeperApp) pruneSkillPulses(now time.Time) {
	if len(a.skillPulses) == 0 {
		return
	}
	kept := a.skillPulses[:0]
	for _, pulse := range a.skillPulses {
		elapsed := now.Sub(pulse.Start)
		if elapsed >= 0 && elapsed < skillPulseDuration {
			kept = append(kept, pulse)
		}
	}
	a.skillPulses = kept
}

func (a *minesweeperApp) enqueueKnotBurst(now time.Time, idx int) {
	if a.motionReducedActive {
		return
	}
	a.pruneKnotBursts(now)
	a.knotBursts = append(a.knotBursts, knotBurstPulse{
		Index: idx,
		Start: now,
	})
	if len(a.knotBursts) > maxKnotBursts {
		a.knotBursts = a.knotBursts[len(a.knotBursts)-maxKnotBursts:]
	}
}

func (a *minesweeperApp) pruneKnotBursts(now time.Time) {
	if len(a.knotBursts) == 0 {
		return
	}
	kept := a.knotBursts[:0]
	for _, burst := range a.knotBursts {
		elapsed := now.Sub(burst.Start)
		if elapsed >= 0 && elapsed < knotBurstDuration {
			kept = append(kept, burst)
		}
	}
	a.knotBursts = kept
}

func explodedCellIndex(g *Game) int {
	if g.duel != nil {
		return g.duel.LastAction
	}
	for i := range g.Cells {
		if g.Cells[i].Exploded {
			return i
		}
	}
	return 0
}

// buildConfettiPieces creates a deterministic, bounded cascade of five-member
// paper scraps and hand-drawn exclamation marks so victory frames never drift
// across runs.
func buildConfettiPieces() []confettiPiece {
	pieces := make([]confettiPiece, confettiCount)
	state := uint32(0x5A17C0DE)
	nextUnit := func() float32 {
		state = state*1664525 + 1013904223
		return float32(state>>8) / float32(1<<24)
	}
	for i := range confettiCount {
		exclaim := i%4 == 0
		baseX := (float32(i) + 0.5) / float32(confettiCount)
		jitterX := (nextUnit() - 0.5) * 0.045
		startX := max(float32(0.05), min(float32(0.95), baseX+jitterX))
		pieces[i] = confettiPiece{
			MemberID: iconOrder[i%len(iconOrder)],
			StartX:   startX,
			StartY:   -0.02 + nextUnit()*0.18,
			DriftX:   (nextUnit() - 0.5) * 110,
			FallY:    280 + nextUnit()*300,
			SwayFreq: 2.2 + nextUnit()*2.4,
			SwayAmp:  10 + nextUnit()*14,
			Phase:    nextUnit() * 6.2831853,
			Rot0:     (nextUnit() - 0.5) * 70,
			RotSpeed: (nextUnit() - 0.5) * 280,
			W:        9 + nextUnit()*5,
			H:        5 + nextUnit()*3,
			Exclaim:  exclaim,
			Delay:    time.Duration(int(nextUnit()*140)) * time.Millisecond,
		}
	}
	return pieces
}

func (a *minesweeperApp) spawnConfetti(now time.Time) {
	if a.motionReducedActive {
		return
	}
	a.confetti = buildConfettiPieces()
	a.confettiStart = now
}

func (a *minesweeperApp) pruneConfetti(now time.Time) bool {
	if len(a.confetti) == 0 || a.confettiStart.IsZero() {
		a.confetti = nil
		a.confettiStart = time.Time{}
		return false
	}
	elapsed := now.Sub(a.confettiStart)
	if elapsed < 0 || elapsed >= confettiDuration {
		a.confetti = nil
		a.confettiStart = time.Time{}
		return false
	}
	return true
}

func paintSkillTargetPulse(p *ui.Painter, tile ui.Rect, radius float32, memberID MemberID, pal *palette, t float32) {
	base := memberImage(memberID)
	ink := pal.blendInkOf(base)
	ease := 1 - (1-t)*(1-t)
	fade := (1 - t) * (1 - t)

	p.Fill(tile, base.Alpha(0.28*fade), radius)

	grow := min(tile.W, tile.H) * 0.44 * ease
	ring := ui.Rect{
		X: tile.X - grow,
		Y: tile.Y - grow,
		W: tile.W + grow*2,
		H: tile.H + grow*2,
	}
	ringRadius := radius + grow*0.45
	ringWidth := 2.6 * (1 - 0.35*t)
	p.Stroke(ring, base.Alpha(0.90*fade), ringRadius, ringWidth)
	p.Stroke(ring, ink.Alpha(0.42*fade), ringRadius, 1.1)

	if t > 0.14 {
		t2 := (t - 0.14) / 0.86
		ease2 := 1 - (1-t2)*(1-t2)
		fade2 := (1 - t2) * (1 - t2)
		grow2 := min(tile.W, tile.H) * 0.32 * ease2
		ring2 := ui.Rect{
			X: tile.X - grow2,
			Y: tile.Y - grow2,
			W: tile.W + grow2*2,
			H: tile.H + grow2*2,
		}
		p.Stroke(ring2, base.Alpha(0.65*fade2), radius+grow2*0.45, 1.8*(1-0.3*t2))
	}
}

var knotBurstRays = [...]struct {
	angle float64
	r0    float32
	r1    float32
	bend  float32
	red   bool
}{
	{0.18, 0.55, 1.85, 0.18, false},
	{0.84, 0.50, 1.55, -0.22, true},
	{1.42, 0.60, 1.95, 0.14, false},
	{2.08, 0.52, 1.68, -0.16, true},
	{2.68, 0.58, 1.90, 0.20, false},
	{3.32, 0.48, 1.62, -0.18, true},
	{3.95, 0.56, 2.02, 0.15, false},
	{4.62, 0.50, 1.72, -0.20, true},
	{5.24, 0.58, 1.88, 0.16, false},
	{5.84, 0.52, 1.60, -0.14, true},
}

func paintKnotBurst(p *ui.Painter, tile ui.Rect, pal *palette, t float32) {
	ease := 1 - (1-t)*(1-t)*(1-t)
	fade := 1 - t
	cx, cy := tile.X+tile.W/2, tile.Y+tile.H/2
	baseR := min(tile.W, tile.H) * 0.5

	// Expanding red-ink bloom wash and crisp ink ring around the hit knot.
	bloomR := baseR * (0.60 + 0.95*ease)
	p.Fill(ui.Rect{X: cx - bloomR, Y: cy - bloomR, W: bloomR * 2, H: bloomR * 2}, pal.BoomInk.Alpha(0.24*fade*fade), bloomR)
	ringR := baseR * (0.85 + 1.25*ease)
	p.Stroke(ui.Rect{X: cx - ringR, Y: cy - ringR, W: ringR * 2, H: ringR * 2}, pal.BoomInk.Alpha(0.86*fade), ringR, 2.2*(1-0.4*t))

	// Irregular outward pencil scribbles radiating from the knot.
	for _, ray := range knotBurstRays {
		cosA := float32(math.Cos(ray.angle))
		sinA := float32(math.Sin(ray.angle))
		perpX, perpY := -sinA, cosA
		inner := baseR * (ray.r0 + 0.35*ease)
		outer := baseR * (ray.r0 + (ray.r1-ray.r0)*ease)
		mid := (inner + outer) * 0.5
		jitter := baseR * ray.bend * (0.4 + 0.6*ease)

		x0, y0 := cx+cosA*inner, cy+sinA*inner
		xm, ym := cx+cosA*mid+perpX*jitter, cy+sinA*mid+perpY*jitter
		x1, y1 := cx+cosA*outer, cy+sinA*outer

		col := pal.Graphite
		width := float32(1.45)
		if ray.red {
			col = pal.BoomInk
			width = 1.65
		}
		p.StrokePolyline([][2]float32{{x0, y0}, {xm, ym}, {x1, y1}}, width*(1-0.3*t), col.Alpha(0.92*fade))
	}
}

func paintHandExclaim(p *ui.Painter, cx, cy, scale, rotDeg float32, col ui.Color) {
	p.Transform(ui.Rotation(rotDeg), cx, cy, func() {
		// Slightly slanted hand-drawn stem + dot using pure primitives (zero extra glyphs).
		stemTopY := cy - 6.5*scale
		stemBotY := cy + 2.2*scale
		p.Line(cx+0.6*scale, stemTopY, cx-0.3*scale, stemBotY, 2.6*scale, col)
		dotR := 1.6 * scale
		p.Fill(ui.Rect{X: cx - 0.3*scale - dotR, Y: cy + 4.4*scale - dotR, W: dotR * 2, H: dotR * 2}, col, dotR)
	})
}

func (a *minesweeperApp) paintConfetti(p *ui.Painter, r ui.Rect, pal *palette, now time.Time) {
	if a.motionReducedActive || !a.pruneConfetti(now) {
		return
	}
	elapsed := now.Sub(a.confettiStart)
	p.Clip(r, 8, func() {
		for _, pc := range a.confetti {
			if elapsed < pc.Delay {
				continue
			}
			life := confettiDuration - pc.Delay
			u := float32(elapsed-pc.Delay) / float32(life)
			fallEase := u * (0.35 + 0.65*u)
			sway := float32(math.Sin(float64(pc.Phase+u*pc.SwayFreq*6.2831853))) * pc.SwayAmp
			cx := r.X + pc.StartX*r.W + pc.DriftX*u + sway
			cy := r.Y + pc.StartY*r.H + pc.FallY*fallEase
			rot := pc.Rot0 + pc.RotSpeed*u
			alpha := float32(1)
			if u < 0.08 {
				alpha = u / 0.08
			} else if u > 0.68 {
				alpha = 1 - (u-0.68)/0.32
			}
			if alpha <= 0.01 {
				continue
			}
			fillCol, markCol := confettiPieceColors(pc.MemberID, pal.Dark)
			if pc.Exclaim {
				paintHandExclaim(p, cx, cy, 1.05, rot, markCol.Alpha(alpha))
				continue
			}
			rect := ui.Rect{X: cx - pc.W/2, Y: cy - pc.H/2, W: pc.W, H: pc.H}
			p.Transform(ui.Rotation(rot), cx, cy, func() {
				p.Fill(rect, fillCol.Alpha(alpha), 1.5)
				p.Stroke(rect, markCol.Alpha(0.45*alpha), 1.5, 0.8)
			})
		}
	})
	p.AnimationFrame()
}

// stripSlide returns the vertical slide offset and next wake delay for a stage
// strip. Strips stay at full opacity across enter, hold, and exit—sliding out
// from and back under the clipped stage top edge—so text keeps >= 4.5:1
// contrast on every visible frame; reduced motion holds statically at 0 offset.
func stripSlide(elapsed, total, enter, exit time.Duration, reduced bool) (offsetY float32, nextWake time.Duration) {
	remain := total - elapsed
	if reduced {
		return 0, max(time.Millisecond, remain)
	}
	exitStart := total - exit
	if elapsed < enter {
		u := float32(elapsed) / float32(enter)
		ease := 1 - (1-u)*(1-u)*(1-u)
		return -36 * (1 - ease), min(16*time.Millisecond, max(time.Millisecond, enter-elapsed))
	}
	if elapsed >= exitStart {
		u := float32(elapsed-exitStart) / float32(exit)
		ease := u * u
		return -56 * ease, min(16*time.Millisecond, max(time.Millisecond, remain))
	}
	return 0, max(time.Millisecond, exitStart-elapsed)
}

func (a *minesweeperApp) stageHighlightsOverlay(c *ui.Context, pal *palette, now time.Time) {
	quoteActive := a.pruneQuoteStrip(now)
	memeActive := a.pruneMemeStrip(now)
	confettiActive := !a.motionReducedActive && a.pruneConfetti(now)
	if !quoteActive && !memeActive && !confettiActive {
		return
	}

	if confettiActive {
		ui.Box(c.Key("stage-confetti")).Absolute().Top(0).Bottom(0).Start(0).End(0).
			IgnoreInput(true).Draw(func(p *ui.Painter, r ui.Rect) {
			a.paintConfetti(p, r, pal, p.Now())
		})
	}

	if !quoteActive && !memeActive {
		return
	}

	topOffset := float32(10)
	if a.isDuel() && a.game.Status == StatusPlaying && a.game.duel.playerStunned() {
		topOffset = 78
	}

	ui.Column(c.Key("stage-highlights-stack")).Absolute().Top(topOffset).Start(12).End(12).
		AlignItems(ui.Center).Gap(6).IgnoreInput(true).Children(func() {
		if quoteActive {
			qs := a.quoteStrip
			elapsed := now.Sub(qs.Start)
			offsetY, nextWake := stripSlide(elapsed, quoteStripDuration, 140*time.Millisecond, 180*time.Millisecond, a.motionReducedActive)
			c.After(nextWake)
			m := FindMember(qs.MemberID)
			sc := quoteStripColorsFor(qs.MemberID, pal.Dark)
			strip := ui.Row(c.Key("stage-quote-strip")).MaxWidth(480).Padding(8, 12).Gap(9).Radius(4).
				AlignItems(ui.Center).Background(sc.Bg).Border(1, sc.Border).
				Shadow(0, 2, 7, 0, pal.SlipShade).Label("台词").
				Draw(func(p *ui.Painter, r ui.Rect) {
					p.Fill(ui.Rect{X: r.X, Y: r.Y, W: 3.5, H: r.H}, sc.Accent, 2)
					tape := ui.Rect{X: r.X + 20, Y: r.Y - 5, W: 48, H: 11}
					p.Transform(ui.Rotation(-4), tape.X+tape.W/2, tape.Y+tape.H/2, func() {
						p.Fill(tape, sc.Tape, 1)
					})
				})
			if offsetY != 0 {
				strip.Translate(0, offsetY)
			}
			strip.Children(func() {
				memberAvatar(c, m, 26, pal)
				ui.Column(c).Gap(1).Children(func() {
					ui.Row(c).AlignItems(ui.Baseline).Gap(6).Children(func() {
						ui.Text(c, m.Name).FontSize(12.5).Bold().TextColor(sc.NameInk).SingleLine()
						ui.Text(c, m.Role).FontSize(10.5).TextColor(sc.RoleInk).SingleLine()
					})
					ui.Text(c, qs.Text).FontSize(12.5).TextColor(sc.TextInk)
				})
			})
		}

		if memeActive {
			ms := a.memeStrip
			elapsed := now.Sub(ms.Start)
			offsetY, nextWake := stripSlide(elapsed, memeStripDuration, 160*time.Millisecond, 200*time.Millisecond, a.motionReducedActive)
			c.After(nextWake)
			mc := memeStripColorsFor(pal.Dark)
			card := ui.Column(c.Key("stage-meme-strip")).MaxWidth(460).Padding(7, 13).Gap(2).Radius(4).
				Background(mc.Bg).Border(1.2, mc.Border).
				Shadow(0, 2, 7, 0, pal.SlipShade).
				Draw(func(p *ui.Painter, r ui.Rect) {
					p.Fill(ui.Rect{X: r.X, Y: r.Y, W: 4, H: r.H}, mc.Accent, 2)
					tape := ui.Rect{X: r.X + r.W - 58, Y: r.Y - 5, W: 44, H: 10}
					p.Transform(ui.Rotation(3.5), tape.X+tape.W/2, tape.Y+tape.H/2, func() {
						p.Fill(tape, mc.Tape, 1)
					})
				})
			if offsetY != 0 {
				card.Translate(0, offsetY)
			}
			card.Children(func() {
				ui.Text(c, "名场面解锁").FontSize(11).Bold().TextColor(mc.EyebrowInk).SingleLine()
				for _, meme := range ms.Memes {
					ui.Text(c, "★ "+meme.Title).FontSize(13).Bold().TextColor(mc.TitleInk).SingleLine()
				}
			})
		}
	})
}

func (a *minesweeperApp) haruhikageOverlay(c *ui.Context, pal *palette, baseTheme *ui.Theme, now time.Time) {
	if a.haruhikageIntro {
		elapsedIntro := now.Sub(a.haruhikageIntroStart)
		if elapsedIntro < 0 {
			elapsedIntro = 0
		}
		if elapsedIntro >= haruhikageIntroDuration {
			a.haruhikageIntro = false
			a.haruhikageAlert = true
		} else {
			remain := haruhikageIntroDuration - elapsedIntro
			step := 50 * time.Millisecond
			if !a.motionReducedActive && elapsedIntro < haruhikageBeatDelay+haruhikageLineFade {
				step = 16 * time.Millisecond
			}
			c.After(min(step, max(time.Millisecond, remain)))
			drama := haruhikageFor(pal.Dark)

			scrimFrac := float32(1)
			showCard := true
			cardAlpha := float32(1)
			cardOffsetY := float32(0)
			if !a.motionReducedActive {
				scrimT := min(float32(1), float32(elapsedIntro)/float32(haruhikageScrimFade))
				scrimEase := 1 - (1-scrimT)*(1-scrimT)
				scrimFrac = 0.52 + 0.48*scrimEase
				if elapsedIntro < haruhikageBeatDelay {
					showCard = false
				} else {
					lineT := min(float32(1), float32(elapsedIntro-haruhikageBeatDelay)/float32(haruhikageLineFade))
					lineEase := 1 - (1-lineT)*(1-lineT)*(1-lineT)
					cardAlpha = 0.18 + 0.82*lineEase
					cardOffsetY = 10 * (1 - lineEase)
				}
			}

			ui.Overlay(c, func() {
				overlay := ui.Box(c.Key("haruhikage-drama-overlay")).Fill().Modal().
					Background(drama.Scrim.Alpha(scrimFrac)).Center().
					Cursor(ui.CursorPointer).Focusable().FocusRing(false).Label("春日影演出遮罩").
					HandleInput(func(ev ui.InputEvent) bool {
						switch ev.Kind {
						case ui.InputScroll, ui.InputZoom:
							return true
						case ui.InputPointerDown, ui.InputPointerUp:
							if ev.Button != 0 {
								return true
							}
						}
						return false
					})
				if a.focusDuelSettlement {
					a.focusDuelSettlement = false
					overlay.Focus()
				}
				if showCard {
					overlay.Children(func() {
						card := ui.Column(c).AlignItems(ui.Center).Gap(10).Padding(26, 32).Radius(6).
							Background(drama.CardBg).Border(1.5, drama.CardBorder).
							Shadow(0, 12, 32, 0, drama.CardShadow).Opacity(cardAlpha).PassThrough().
							Draw(func(p *ui.Painter, r ui.Rect) {
								p.Clip(r, 6, func() {
									p.Fill(ui.Rect{X: r.X, Y: r.Y, W: r.W, H: 3}, drama.AccentBar, 0)
									for y := r.Y + 24; y < r.Y+r.H-8; y += 22 {
										p.Line(r.X+14, y, r.X+r.W-14, y, 1, drama.CardRule)
									}
								})
							})
						if cardOffsetY != 0 {
							card.Translate(0, cardOffsetY)
						}
						card.Children(func() {
							ui.Text(c, "♪ 《春日影》 — 乐声骤停").FontSize(13).Bold().TextColor(drama.Eyebrow).PassThrough()
							ui.Text(c, "「……为什么要演奏春日影？！」").FontSize(22).Bold().TextColor(drama.Title).PassThrough()
							ui.Text(c, "（长崎爽世的眼神失去了高光 · 点击任意处继续）").FontSize(12).TextColor(drama.Subtitle).PassThrough()
						})
					})
				}
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
}

func (a *minesweeperApp) handleAction(c *ui.Context, res ActionResult, now time.Time) {
	if res.Kind == ActionNoop {
		return
	}
	clickRow, clickCol := a.pendingRow, a.pendingCol
	if a.game.Status == StatusPlaying && !a.isDuel() && a.startedAt.IsZero() {
		a.startedAt = now
	}
	var lockedWash ui.Color
	if res.RevealedCount >= bigFloodCells {
		lockedWash = memberImage(a.memberID)
	}
	a.enqueueRipple(now, res.RevealWave, lockedWash)
	if res.Kind == ActionSkill && !a.motionReducedActive {
		for _, idx := range res.SkillTargets {
			if a.game.Cells[idx].Flagged {
				a.enqueueBandage(now, idx, bandageBounce, a.game.Cells[idx].ShieldSaved)
			}
		}
	}
	if a.isDuel() {
		popRow, popCol := clickRow, clickCol
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

	actorID := a.memberID
	if res.Kind == ActionShielded {
		actorID = MemberSoyorin
	}
	m := FindMember(actorID)
	quote := a.applyQuoteFor(actorID, res.QuoteEvent)
	a.highlightSound(res, now)

	fromSkill := res.Kind == ActionSkill || (res.Kind == ActionWon && len(res.SkillTargets) > 0)
	if fromSkill || res.Kind == ActionShielded {
		a.showQuoteStrip(actorID, quote, now)
	}
	if len(res.SkillTargets) > 0 {
		if a.isDuel() || a.classicOnCanvas() {
			first := res.SkillTargets[0]
			targets := res.SkillTargets
			a.board.ensureVisible(a.game, first/a.game.Cols, first%a.game.Cols, now, a.motionReducedActive, func(at time.Time) {
				a.enqueueSkillPulses(actorID, targets, at)
			})
		} else {
			a.enqueueSkillPulses(actorID, res.SkillTargets, now)
		}
	}

	switch res.Kind {
	case ActionShielded:
		a.triggerShake(now, 4.5, 220*time.Millisecond)
		c.Toast("长崎爽世发动了「温柔的微笑」：这次就当没点过吧！")
	case ActionSkill:
		c.Toast(fmt.Sprintf("%s 发动「%s」", m.Name, m.SkillName))
	case ActionLost:
		a.enqueueKnotBurst(now, explodedCellIndex(a.game))
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
		a.spawnConfetti(now)
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
	a.applyQuoteFor(a.memberID, event)
}

func (a *minesweeperApp) applyQuoteFor(memberID MemberID, event string) string {
	if event == "" {
		return ""
	}
	m := FindMember(memberID)
	a.quote = PickQuote(memberID, event)
	a.pushChat(m.Name, m.ColorHex, a.quote)
	return a.quote
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
	a.enqueueMemeStrip(meme, now)
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

// renderedPaper returns the currently interpolated notebook palette (snapping
// directly to dayPaper/nightPaper before the first render and once settled).
func (a *minesweeperApp) renderedPaper() *palette {
	if !a.nightBlendInit {
		return a.paper()
	}
	return blendPaper(a.nightBlendValue)
}

func (a *minesweeperApp) stepNightBlend(now time.Time) (*palette, bool) {
	target := float32(0)
	if a.darkMode {
		target = 1
	}
	if !a.nightBlendInit {
		a.nightBlendInit = true
		a.nightBlendFrom = target
		a.nightBlendValue = target
		a.nightBlendTarget = target
		a.nightBlendStart = time.Time{}
		return blendPaper(target), false
	}
	if target != a.nightBlendTarget {
		if !a.nightBlendStart.IsZero() {
			elapsed := now.Sub(a.nightBlendStart)
			if elapsed >= lampTransitionDuration {
				a.nightBlendValue = a.nightBlendTarget
			} else if elapsed > 0 {
				u := float32(elapsed) / float32(lampTransitionDuration)
				ease := 1 - (1-u)*(1-u)*(1-u)
				a.nightBlendValue = a.nightBlendFrom + (a.nightBlendTarget-a.nightBlendFrom)*ease
			}
		}
		a.nightBlendFrom = a.nightBlendValue
		a.nightBlendTarget = target
		a.nightBlendStart = now
		return blendPaper(a.nightBlendValue), true
	}
	if a.nightBlendStart.IsZero() {
		return blendPaper(a.nightBlendValue), false
	}
	elapsed := now.Sub(a.nightBlendStart)
	if elapsed <= 0 {
		a.nightBlendValue = a.nightBlendFrom
		return blendPaper(a.nightBlendValue), true
	}
	if elapsed >= lampTransitionDuration {
		a.nightBlendValue = a.nightBlendTarget
		a.nightBlendFrom = a.nightBlendTarget
		a.nightBlendStart = time.Time{}
		return blendPaper(a.nightBlendValue), false
	}
	u := float32(elapsed) / float32(lampTransitionDuration)
	ease := 1 - (1-u)*(1-u)*(1-u)
	a.nightBlendValue = a.nightBlendFrom + (a.nightBlendTarget-a.nightBlendFrom)*ease
	return blendPaper(a.nightBlendValue), true
}

func (a *minesweeperApp) themeFor(p *palette) *ui.Theme {
	accent := bandColor.Mix(nightPaper.InkOf(bandColor), p.nightFrac)
	var t *ui.Theme
	if p.Dark {
		t = ui.DarkTheme().WithAccent(accent)
	} else {
		t = ui.LightTheme().WithAccent(accent)
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

// activeTheme is the notebook paper as a widget theme: the desk behind,
// sheets for panels, ink for text, the 乐队色 as accent, 文楷 throughout.
func (a *minesweeperApp) activeTheme() *ui.Theme {
	return a.themeFor(a.paper())
}

func (a *minesweeperApp) settleDuel(c *ui.Context, now time.Time, result string) {
	d := a.game.duel
	a.duelEnded = true
	posted := a.chatPosted
	a.dismissPopovers()
	a.clearPopoverMotion()
	a.focusDuelSettlement = true
	a.pushChat("决斗结算", hexOf(goldHighlighter), result)
	a.applyQuote(d.QuoteEvent)
	a.unlockMemes(c, d.NewMemes)
	if d.PlayerScore > d.AIScore {
		a.spawnConfetti(now)
	}
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
			a.enqueueRipple(now, tick.AITurn.RevealWave, ui.Color{})
			if tick.AITurn.Kind == ActionFlagged {
				a.enqueueBandage(now, tick.AICell, bandageBounce, a.game.Cells[tick.AICell].ShieldSaved)
			}
			if tick.AITurn.RevealedCount > 0 {
				a.board.LastEffect = now
				a.board.EffectCell = tick.AICell
			}
			a.voiceAITurn(tick.AITurn.Kind, tick.AICell)
			if tick.AITurn.Kind == ActionLost {
				a.enqueueKnotBurst(now, tick.AICell)
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
	if pal.nightFrac > 0 && pal.nightFrac < 1 {
		return autoInkOn(bg, dayPaper.InkMuted, nightPaper.InkMuted)
	}
	under := *pal
	under.Board = bg
	under.Sheet = bg
	return under.InkOf(base)
}

// setlistAccentInk derives a WCAG-AA (4.5:1+) ink from the 乐队色 against
// the exact composited background a setlist control paints beneath its text.
func setlistAccentInk(pal *palette, bg ui.Color) ui.Color {
	if pal.nightFrac > 0 && pal.nightFrac < 1 {
		return pal.blendInkOf(bandColor)
	}
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

// bestTimeText is the fastest-clear line shown for a track, with a
// placeholder until the track has been cleared.
func (a *minesweeperApp) bestTimeText(id string) string {
	if best, ok := a.bestTimes[id]; ok {
		return fmt.Sprintf("最快 %d 秒", best)
	}
	return "最快 --"
}

// setlistTrackDescription is what screen readers hear for a setlist entry:
// its spec plus, for classic tracks, the fastest clear.
func (a *minesweeperApp) setlistTrackDescription(d Difficulty) string {
	if d.Gameplay == GameplayDuel {
		return setlistTrackSpec(d)
	}
	return setlistTrackSpec(d) + " · " + a.bestTimeText(d.ID)
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
								Description(a.setlistTrackDescription(d)).
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
										ui.Row(c).FillWidth().AlignItems(ui.Baseline).Justify(ui.SpaceBetween).Gap(8).PassThrough().Children(func() {
											ui.Text(c, setlistTrackSpec(d)).FontSize(11).TextColor(itemMutedInk).FontFeatures("tnum").PassThrough()
											if d.Gameplay != GameplayDuel {
												bestInk := itemMutedInk
												if _, ok := a.bestTimes[d.ID]; ok {
													bestInk = t.Text
												}
												ui.Text(c, a.bestTimeText(d.ID)).FontSize(11).TextColor(bestInk).FontFeatures("tnum").PassThrough()
											}
										})
									})
									// Unselected rows keep the check's slot so records line up in one column.
									if selected {
										ui.Icon(c, setlistCheckIcon).Size(13, 13).TextColor(selectedInk).PassThrough()
									} else {
										ui.Box(c).Size(13, 13).PassThrough()
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

const (
	settingsPanelTitle   = "笔记本设置"
	settingsDismissHint  = "按 Esc 收起"
	settingsMutedHint    = "已静音，开启音效后恢复此音量"
	settingsQuestionHint = "右键贴过创可贴后，再点一次标上 ? 标记"
)

func settingsVolumeReadout(enabled bool, volume int) string {
	if !enabled {
		return fmt.Sprintf("已静音 · 保留 %d%%", volume)
	}
	return fmt.Sprintf("%d%%", volume)
}

func (a *minesweeperApp) settingsControl(c *ui.Context, pal *palette) {
	sc := settingsColorsForPaper(pal)
	isOpen := a.popoverOpen(popoverSettings)
	trigger := ui.ButtonBase(c.Key("settings-trigger")).
		Padding(6, 11).Radius(6).Cursor(ui.CursorPointer).
		Label("设置").HasPopup(ui.PopupDialog).Expanded(isOpen).
		Tooltip("调整台灯、动效、声音与扫雷习惯")

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
		panel := ui.Column(c.Key("settings-panel")).Width(320).Gap(10).Label(settingsPanelTitle)
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
				ui.Text(c, settingsPanelTitle).FontSize(12).Bold().TextColor(sc.MutedInk)
				ui.Text(c, settingsDismissHint).FontSize(11).TextColor(sc.MutedInk)
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
					if focusFirst || sw.Pressed() || sw.Clicked() {
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
					motionTooltips := [3]string{
						"跟随系统的减弱动态效果设置",
						"完整展示揭纸波浪、纸屑与镜头滑行",
						"收起揭纸位移、震屏、纸屑与镜头滑行",
					}
					parts := ui.SegmentedBase(c.Key("settings-motion"), &motionIdx, len(motionLabels))
					parts.Track.FillWidth().AlignItems(ui.Stretch).Padding(2).Gap(2).Radius(7).
						Background(sc.SegTrackBg).Border(1, pal.Rule).LabelledBy(motionLabel).
						Children(func() {
							for i, label := range motionLabels {
								seg := parts.Segment(i).Grow(1).Padding(5, 8).Radius(5).Cursor(ui.CursorPointer).
									Tooltip(motionTooltips[i])
								if seg.Pressed() || seg.Clicked() {
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
						KeyShortcuts(0, ui.KeyM).Tooltip("开关纸笔与乐器音效（按 M 切换）")
					if soundSwitch.Pressed() || soundSwitch.Clicked() {
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
						ui.Text(c, settingsVolumeReadout(a.soundEnabled, a.volume)).FontSize(12).Bold().TextColor(volValueInk).FontFeatures("tnum").SingleLine()
					})
					vol := float64(a.volume)
					volWrap := ui.Box(c.Key("settings-volume-wrap")).FillWidth()
					if !a.soundEnabled {
						// Disabled controls skip hover tooltips in mygo/ui, so attach
						// the muted hover tooltip to the non-disabled wrapper instead.
						volWrap.Tooltip(settingsMutedHint)
					}
					var volSlider ui.Element
					volWrap.Children(func() {
						volSlider = ui.Slider(c.Key("settings-volume"), &vol, 0, 100).
							Step(1).FillWidth().LabelledBy(volLabel).Value(fmt.Sprintf("%d%%", a.volume)).
							Disabled(!a.soundEnabled)
						if a.soundEnabled {
							volSlider.Tooltip("左右拖动或按方向键调节音量")
						} else {
							volSlider.Description(settingsMutedHint)
						}
					})
					if a.soundEnabled && (volSlider.Pressed() || volSlider.Clicked() || volSlider.Changed()) {
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

				ui.Column(c).FillWidth().Padding(2, 4).Gap(3).Children(func() {
					ui.Row(c).FillWidth().AlignItems(ui.Center).Justify(ui.SpaceBetween).Gap(8).Children(func() {
						qLabel := ui.Text(c, "右键 ? 标记").FontSize(13).TextColor(sc.LabelInk)
						allowQ := a.allowQuestion
						qSwitch := ui.Switch(c.Key("settings-allow-question"), &allowQ).LabelledBy(qLabel).
							Description(settingsQuestionHint).Tooltip(settingsQuestionHint)
						if qSwitch.Pressed() || qSwitch.Clicked() {
							qSwitch.Focus()
						}
						if qSwitch.Changed() {
							a.updatePrefs(func(p *prefs) { p.AllowQuestion = allowQ })
						}
					})
					ui.Text(c, settingsQuestionHint).FontSize(11).TextColor(sc.MutedInk)
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
	hud := duelHUDFor(a.renderedPaper().Dark)
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
	a.gridPress.cancel()
	a.playSound(SoundToggle, 1, 0)
}

// gridPointerPress tracks pointer press state on the widget grid: normal left
// presses arm and cancel on leave, while right presses and flag-mode left
// presses commit on press. Left-button releases suppress mygo/ui's duplicate
// Clicks() so keyboard and screen-reader activation keep working through Clicks().
type gridPointerPress struct {
	downActive bool
	downIdx    int
	armed      bool
	row, col   int
	suppressed []int
}

func (p *gridPointerPress) cancel() {
	p.armed = false
	p.row = -1
	p.col = -1
}

func (p *gridPointerPress) clearSuppressed() {
	p.suppressed = p.suppressed[:0]
}

func (p *gridPointerPress) consumeClicks(idx, clicks int, pressed bool) int {
	matched := 0
	kept := p.suppressed[:0]
	for _, s := range p.suppressed {
		if s != idx {
			kept = append(kept, s)
			continue
		}
		if matched < clicks {
			matched++
			continue
		}
		if pressed {
			// Retain unmatched pointer-up suppressions while mygo/ui holds a
			// deferred releaseLater click for this tile until endPass.
			kept = append(kept, s)
		}
	}
	p.suppressed = kept
	return clicks - matched
}

func canArmGridCell(g *Game, row, col int) bool {
	if g.Status == StatusWon || g.Status == StatusLost {
		return false
	}
	cell := g.Cell(row, col)
	if cell == nil {
		return false
	}
	if !cell.Revealed {
		return !cell.Flagged
	}
	return g.InspectNumberHover(row, col, false).Kind == HintChordable
}

func (a *minesweeperApp) enqueuePendingAction(res ActionResult, row, col int) {
	if res.Kind == "" || res.Kind == ActionNoop {
		return
	}
	a.pendingActions = append(a.pendingActions, pendingCellAction{
		result: res,
		row:    row,
		col:    col,
	})
}

func (a *minesweeperApp) flushPendingActions(c *ui.Context, now time.Time) {
	if a.pendingResult.Kind != "" && a.pendingResult.Kind != ActionNoop {
		res, row, col := a.pendingResult, a.pendingRow, a.pendingCol
		a.pendingResult = ActionResult{}
		a.pendingRow, a.pendingCol = -1, -1
		a.pendingActions = append([]pendingCellAction{{result: res, row: row, col: col}}, a.pendingActions...)
	}
	if len(a.pendingActions) == 0 {
		return
	}
	actions := a.pendingActions
	a.pendingActions = nil
	for _, act := range actions {
		a.pendingRow = act.row
		a.pendingCol = act.col
		// Pointer-handled cell moves queue a result; their foley already played.
		posted := a.chatPosted
		a.handleAction(c, act.result, now)
		if _, voiced := cellActionSound(act.result.Kind); voiced {
			a.chatVoicedSince(posted)
		}
	}
}

type gridPressResult struct {
	handled      bool
	armed        bool
	commit       bool
	commitRow    int
	commitCol    int
	commitButton int
}

func (p *gridPointerPress) handleEvent(g *Game, flagMode bool, idx, row, col int, bounds ui.Rect, ev ui.InputEvent) gridPressResult {
	switch ev.Kind {
	case ui.InputPointerDown:
		if ev.Button == 1 {
			p.cancel()
			return gridPressResult{handled: true, commit: true, commitRow: row, commitCol: col, commitButton: 1}
		}
		if ev.Button != 0 {
			p.cancel()
			return gridPressResult{}
		}
		p.downActive = true
		p.downIdx = idx
		if flagMode {
			p.cancel()
			return gridPressResult{handled: true, commit: true, commitRow: row, commitCol: col, commitButton: 0}
		}
		if canArmGridCell(g, row, col) {
			p.armed = true
			p.row = row
			p.col = col
			return gridPressResult{handled: true, armed: true}
		}
		p.cancel()
		return gridPressResult{handled: true}

	case ui.InputPointerMove:
		if !p.downActive || p.downIdx != idx {
			return gridPressResult{}
		}
		if p.armed && !bounds.Contains(ev.X, ev.Y) {
			p.cancel()
		}
		return gridPressResult{handled: true}

	case ui.InputPointerUp:
		if ev.Button == 1 {
			return gridPressResult{handled: true}
		}
		if !p.downActive || p.downIdx != idx || ev.Button != 0 {
			return gridPressResult{}
		}
		p.downActive = false
		inBounds := bounds.Contains(ev.X, ev.Y)
		if !ev.Canceled && inBounds {
			p.suppressed = append(p.suppressed, idx)
		}
		if p.armed && !ev.Canceled && inBounds {
			r, c := p.row, p.col
			p.cancel()
			return gridPressResult{handled: true, commit: true, commitRow: r, commitCol: c, commitButton: 0}
		}
		p.cancel()
		return gridPressResult{handled: true}
	}
	return gridPressResult{}
}

func (a *minesweeperApp) handleGridTileInput(idx, row, col int, size float32, ev ui.InputEvent) bool {
	res := a.gridPress.handleEvent(
		a.game,
		a.flagMode,
		idx, row, col,
		ui.Rect{W: size, H: size},
		ev,
	)
	if res.armed {
		a.playPaperTouch(col)
	}
	if res.commit {
		act := a.applyCellAction(res.commitRow, res.commitCol, res.commitButton, ev.Time)
		a.enqueuePendingAction(act, res.commitRow, res.commitCol)
	}
	return res.handled
}

func (a *minesweeperApp) view(c *ui.Context) {
	now := c.Now()
	a.lastFrameNow = now
	defer func() { a.viewMounted = true }()
	defer a.flushChatNote()
	defer a.scheduleCues(c, now)
	a.syncWindowFocus(c)
	a.flushCues(now)
	a.systemReduceMotion = c.Preferences().ReduceMotion
	a.syncMotionState(a.reducedMotion(c))
	a.flushPendingActions(c, now)
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
		a.gridPress.cancel()
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

	pal, lampMoving := a.stepNightBlend(now)
	if lampMoving {
		c.AnimationFrame()
	}
	baseTheme := a.themeFor(pal)
	c.SetTheme(baseTheme)
	t := c.Theme()

	art := artFor(pal.Dark)
	member := FindMember(a.memberID)
	// memberColor is the leader's 代表色, used as a highlighter; memberInk
	// is it as ink, for text and the skill button on this paper.
	memberColor := ui.Hex(member.ColorHex)
	memberInk := pal.blendInkOf(memberColor)
	memberTheme := baseTheme.WithAccent(memberInk)

	a.haruhikageOverlay(c, pal, baseTheme, now)

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
						mInk := pal.blendInkOf(mc)
						selected := m.ID == a.memberID
						item := ui.Box(c.Key(string(m.ID))).Padding(7, 9).Radius(9).
							Cursor(ui.CursorPointer).Label(m.Name).Tooltip(m.SkillDesc)
						targetTint := ui.Transparent
						if selected {
							// A highlighter stroke in her 代表色.
							targetTint = mc.Alpha(0.36)
						} else if item.Hovered() {
							targetTint = mc.Alpha(0.14)
						}
						tint := item.AnimateColor("member-bg", targetTint, ui.Animation{
							Duration: memberHighlightDuration,
							Ease:     ui.EaseOut,
						})
						if tint.A > 0 {
							item.Background(tint.Over(t.Surface))
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
					targetProg := a.game.Progress()
					displayProg, progMoving := a.progressVisual.update(targetProg, now, meterGlideDuration, a.motionReducedActive)
					ui.Row(c).Justify(ui.SpaceBetween).Children(func() {
						ui.Text(c, "排练完成度").FontSize(12).TextColor(t.TextMuted)
						ui.Textf(c, "%.0f%%", targetProg*100).FontSize(12).Bold().TextColor(t.Accent)
					})
					hatchProgress(c, "排练完成度", targetProg, displayProg, pal.blendInkOf(bandColor), pal)

					targetGrav := float64(a.game.Gravity)
					displayGrav, gravMoving := a.gravityVisual.update(targetGrav, now, meterGlideDuration, a.motionReducedActive)
					gravityInk := pal.blendInkOf(memberImage(MemberSoyorin))
					ui.Row(c).Justify(ui.SpaceBetween).Children(func() {
						ui.Text(c, "乐队重力值").FontSize(12).TextColor(t.TextMuted)
						ui.Textf(c, "%d%%", a.game.Gravity).FontSize(12).Bold().TextColor(gravityInk)
					})
					hatchMeter(c, "乐队重力值", targetGrav, displayGrav, gravityInk, pal)
					if progMoving || gravMoving {
						c.AnimationFrame()
					}
				})

				// Band Group Chat: sticky notes in each speaker's colour.
				ui.Text(c, "MyGO!!!!! 排练室群聊").FontSize(12).Bold().TextColor(t.TextMuted)
				if !a.chatViewInit {
					a.chatViewInit = true
					for _, msg := range a.chatLog {
						if msg.ID > a.chatLastSeenID {
							a.chatLastSeenID = msg.ID
						}
					}
				} else {
					for _, msg := range a.chatLog {
						if msg.ID > a.chatLastSeenID {
							if a.chatAnims == nil {
								a.chatAnims = make(map[int]time.Time, 4)
							}
							a.chatAnims[msg.ID] = now
							a.chatLastSeenID = msg.ID
						}
					}
				}
				if len(a.chatAnims) > 0 {
					dur := chatNoteEnterDuration
					if a.motionReducedActive {
						dur = chatNoteReducedFade
					}
					present := make(map[int]bool, len(a.chatLog))
					for _, msg := range a.chatLog {
						present[msg.ID] = true
					}
					for id, start := range a.chatAnims {
						if !present[id] || now.Sub(start) < 0 || now.Sub(start) >= dur {
							delete(a.chatAnims, id)
						}
					}
					if len(a.chatAnims) > 0 {
						c.AnimationFrame()
					}
				}
				ui.Scroll(c).Grow(1).Padding(6, 4).Children(func() {
					ui.Column(c).Children(func() {
						for idx := len(a.chatLog) - 1; idx >= 0; idx-- {
							msg := a.chatLog[idx]
							speaker := ui.Hex(msg.ColorHex)
							tilt := float32(0.7)
							if msg.ID%2 == 1 {
								tilt = -0.8
							}
							bottomGap := float32(9)
							if idx == 0 {
								bottomGap = 0
							}
							start, animating := a.chatAnims[msg.ID]
							slot := ui.Column(c.Key(chatNoteKey(msg.ID))).Shrink(0)
							var inner ui.Element
							slot.Children(func() {
								inner = ui.Column(c).Shrink(0).Padding(0, 0, bottomGap, 0)
								inner.Children(func() {
									note := ui.Column(c).Gap(2).Padding(6, 8).Radius(2).
										Background(speaker.Alpha(0.2).Over(t.Surface)).
										Shadow(0, 2, 4, 0, pal.SlipShade.Alpha(0.6))
									if animating {
										if a.motionReducedActive {
											u := float32(now.Sub(start)) / float32(chatNoteReducedFade)
											note.Rotate(tilt).Opacity(0.30 + 0.70*u)
										} else {
											u := float32(now.Sub(start)) / float32(chatNoteEnterDuration)
											ease := 1 - (1-u)*(1-u)
											scale := 0.96 + 0.04*ease
											rot := tilt + 2.5*(1-ease)
											offsetY := -10 * (1 - ease)
											note.Opacity(0.15 + 0.85*ease).
												Transform(ui.Scaling(scale, scale).Then(ui.Rotation(rot)).Then(ui.Translation(0, offsetY)))
										}
									} else {
										note.Rotate(tilt)
									}
									note.Children(func() {
										ui.Text(c, msg.Speaker).FontSize(11).Bold().TextColor(pal.blendInkOf(speaker))
										ui.Text(c, msg.Text).FontSize(12)
									})
								})
							})
							if animating && !a.motionReducedActive {
								u := float32(now.Sub(start)) / float32(chatNoteEnterDuration)
								ease := 1 - (1-u)*(1-u)
								slot.ClipY().Height(ease * inner.Bounds().H)
							}
						}
					})
				})
			})

			// Right Main Stage: Companion Banner + HUD + Board (+ Optional Memes Drawer)
			ui.Column(c).Grow(1).Gap(10).Children(func() {
				// Active Member Banner themed to the selected member!
				banner := ui.Themed(c.Key("member-banner"), memberTheme).Padding(10, 12).Radius(4).Background(t.Surface).
					Shadow(0, 1, 3, 0, pal.SlipShade)
				tapeColor := banner.AnimateColor("member-tape", memberColor.Alpha(0.5), ui.Animation{
					Duration: memberHighlightDuration,
					Ease:     ui.EaseOut,
				})
				banner.Draw(func(p *ui.Painter, r ui.Rect) {
					// A strip of masking tape in her 代表色 holds the note on.
					tape := ui.Rect{X: r.X + 26, Y: r.Y - 6, W: 64, H: 14}
					p.Transform(ui.Rotation(-4), tape.X+tape.W/2, tape.Y+tape.H/2, func() {
						p.Fill(tape, tapeColor, 1)
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
				if !a.memeDrawerInit {
					a.memeDrawerInit = true
					a.memeDrawerOpen = a.showMemes
					if a.showMemes {
						a.memeDrawerValue = 1
					} else {
						a.memeDrawerValue = 0
					}
					a.memeDrawerStart = time.Time{}
				} else if a.showMemes != a.memeDrawerOpen {
					a.memeDrawerOpen = a.showMemes
					if a.motionReducedActive {
						if a.showMemes {
							a.memeDrawerFrom = 0
							a.memeDrawerValue = 0
							a.memeDrawerStart = now
							a.memeDrawerDur = drawerReducedFadeDuration
						} else {
							a.memeDrawerFrom = 0
							a.memeDrawerValue = 0
							a.memeDrawerStart = time.Time{}
						}
					} else {
						a.memeDrawerFrom = a.memeDrawerValue
						a.memeDrawerStart = now
						if a.showMemes {
							a.memeDrawerDur = drawerOpenDuration
						} else {
							a.memeDrawerDur = drawerCloseDuration
						}
					}
				}
				drawerAnimating := false
				if !a.memeDrawerStart.IsZero() {
					elapsed := now.Sub(a.memeDrawerStart)
					target := float32(0)
					if a.memeDrawerOpen {
						target = 1
					}
					if elapsed < 0 {
						elapsed = 0
					}
					if elapsed >= a.memeDrawerDur {
						a.memeDrawerValue = target
						a.memeDrawerFrom = target
						a.memeDrawerStart = time.Time{}
					} else {
						u := float32(elapsed) / float32(a.memeDrawerDur)
						ease := 1 - (1-u)*(1-u)*(1-u)
						if !a.memeDrawerOpen {
							ease = 1 - (1-u)*(1-u)
						}
						a.memeDrawerValue = a.memeDrawerFrom + (target-a.memeDrawerFrom)*ease
						drawerAnimating = true
						c.AnimationFrame()
					}
				}
				if a.memeDrawerOpen || a.memeDrawerValue > 0 {
					clip := ui.Column(c.Key("memes-drawer-clip")).Shrink(0)
					if !a.memeDrawerOpen {
						clip.IgnoreInput(true).AccessHidden(true)
					}
					var drawer ui.Element
					clip.Children(func() {
						drawer = ui.Column(c.Key("memes-drawer")).Shrink(0).Padding(10, 12).Gap(8).Radius(12).Background(t.Surface).Border(1, t.Border)
						if drawerAnimating {
							if a.motionReducedActive {
								drawer.Opacity(0.30 + 0.70*a.memeDrawerValue)
							} else {
								drawer.Opacity(0.20 + 0.80*a.memeDrawerValue).Translate(0, -6*(1-a.memeDrawerValue))
							}
						}
						drawer.Children(func() {
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
					})
					if h := drawer.Bounds().H; h > 0 {
						a.memeDrawerHeight = h
					}
					if drawerAnimating && !a.motionReducedActive {
						clip.ClipY().Height(a.memeDrawerValue * a.memeDrawerHeight)
					}
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
							ui.Badge(c, a.bestTimeText(a.game.Difficulty.ID))
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
					ui.Column(c.Key("classic-grid-stage")).Grow(1).FillWidth().Clip().Children(func() {
						stage := ui.ScrollBoth(c).Grow(1).FillWidth().Clip().Draw(func(p *ui.Painter, bounds ui.Rect) {
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
						a.pruneBandages(now)
						a.pruneSkillPulses(now)
						a.pruneKnotBursts(now)
						if len(a.ripples) > 0 || len(a.bandages) > 0 || len(a.skillPulses) > 0 || len(a.knotBursts) > 0 {
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
								type gridLiftingSlip struct {
									row, col, idx int
									peelT         float32
									tilt          float32
								}
								type gridPeelingBandage struct {
									row, col int
									shield   bool
									peelT    float32
									tilt     float32
								}
								var gridLifting []gridLiftingSlip
								var gridPeeling []gridPeelingBandage
								grid.Children(func() {
									for i := range a.game.Cells {
										cell := &a.game.Cells[i]
										row, col := cell.Row, cell.Col
										cellIdx, cellRow, cellCol, tileSize := i, row, col, cellSize
										tile := ui.Box(c.Key(i)).Size(cellSize, cellSize).Radius(4).Center().
											TabStop(false).Label(fmt.Sprintf("cell-%d-%d", row, col)).
											HandleInput(func(ev ui.InputEvent) bool {
												return a.handleGridTileInput(cellIdx, cellRow, cellCol, tileSize, ev)
											})
										if tile.Hovered() {
											nextHoverRow, nextHoverCol = row, col
										}
										isHintCenter := hoverHint.Kind != HintNone && i == hoverHint.Center
										isHintTarget := hoverHint.Kind != HintNone && containsIndex(hoverHint.Targets, i)

										var peel tilePeelState
										if cell.Revealed && !cell.Flagged {
											peel = a.tilePeelAt(i, now)
										}
										washActive := peel.stage == tilePeelLifting || peel.stage == tilePeelWashOnly
										washColor := bandColor
										if peel.washColor.A > 0 {
											washColor = peel.washColor
										}
										bAnim, hasBandage := a.bandageAnimAt(i, now)

										// Revealed cells are the bare graph paper (no fill); hidden
										// ones are paper slips stuck on it, each a touch askew.
										var tileBg ui.Color
										var slip bool
										var tileCursor bool
										var tileText string
										var tileTextSize float32
										var tileTextColor ui.Color
										var tileArt *ui.SVG

										if val := cellAccessValue(cell); val != "" {
											tile.Value(val).Description(val)
										}
										switch {
										case cell.Revealed:
											switch {
											case cell.Exploded:
												tileBg, tileArt = pal.Boom, art.KnotHit
											case cell.IsMine:
												tileArt = art.Knot
											case cell.Adjacent > 0:
												if peel.stage != tilePeelCovered {
													if isHintCenter {
														tileBg = hintColor.Alpha(0.38)
													} else if washActive {
														tileBg = washColor.Alpha(0.22 * peel.washAlpha)
													} else if tile.Hovered() {
														tileBg = pal.Ink.Alpha(0.05)
													}
												}
												tileCursor = true
												tileText = fmt.Sprintf("%d", cell.Adjacent)
												tileTextSize = cellFont * 1.08
												tileTextColor = numberInk(cell.Adjacent, pal.Dark)
											default:
												if washActive {
													tileBg = washColor.Alpha(0.26 * peel.washAlpha)
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

										tiltCopy := tileSlipTilt(i)
										if slip {
											// Paint the slip's subtle hand-stuck tilt and shadow in the
											// decorative layer so the cell's hit-test and focus bounds
											// never shift when revealed, flagged, or shaken.
											bgCopy := tileBg
											shadeCopy := pal.SlipShade
											sx, sy := shakeX, shakeY
											tile.Draw(func(p *ui.Painter, r ui.Rect) {
												cx, cy := r.X+r.W/2, r.Y+r.H/2
												xf := ui.Rotation(tiltCopy)
												if sx != 0 || sy != 0 {
													xf = xf.Then(ui.Translation(sx, sy))
												}
												p.Transform(xf, cx, cy, func() {
													p.Shadow(r, 4, 0, 1, 1.5, 0, shadeCopy)
													if bgCopy.A > 0 {
														p.Fill(r, bgCopy, 4)
													}
												})
											})
										} else if shakeX != 0 || shakeY != 0 {
											bgCopy := tileBg
											sx, sy := shakeX, shakeY
											tile.Draw(func(p *ui.Painter, r ui.Rect) {
												if bgCopy.A == 0 {
													return
												}
												p.Transform(ui.Translation(sx, sy), 0, 0, func() {
													p.Fill(r, bgCopy, 4)
												})
											})
										} else {
											tile.Background(tileBg)
										}
										if tileCursor {
											tile.Cursor(ui.CursorPointer)
										}
										if tileText != "" || tileArt != nil {
											hasBounce := cell.Flagged && !cell.WrongFlag && hasBandage && bAnim.Kind == bandageBounce
											var bounceScale, bounceRot float32
											if hasBounce {
												bounceScale, bounceRot = bandageBounceTransform(now.Sub(bAnim.Start))
											}
											tile.Children(func() {
												var el ui.Element
												if tileArt != nil {
													el = ui.Image(c, tileArt).Size(cellSize*0.82, cellSize*0.82).PassThrough()
												} else {
													el = ui.Text(c, tileText).FontSize(tileTextSize).Bold().TextColor(tileTextColor).PassThrough()
												}
												switch {
												case hasBounce:
													xf := ui.Scaling(bounceScale, bounceScale).Then(ui.Rotation(tiltCopy + bounceRot))
													if shakeX != 0 || shakeY != 0 {
														xf = xf.Then(ui.Translation(shakeX, shakeY))
													}
													el.Transform(xf)
												case slip:
													xf := ui.Rotation(tiltCopy)
													if shakeX != 0 || shakeY != 0 {
														xf = xf.Then(ui.Translation(shakeX, shakeY))
													}
													el.Transform(xf)
												case shakeX != 0 || shakeY != 0:
													el.Translate(shakeX, shakeY)
												}
											})
										}
										switch {
										case peel.stage == tilePeelCovered:
											coverBg := pal.Slip
											if tile.Hovered() {
												coverBg = pal.Slip.Mix(pal.Sheet, 0.45)
											}
											shadeCopy := pal.SlipShade
											cellIdx := i
											sx, sy := shakeX, shakeY
											tile.DrawOver(func(p *ui.Painter, r ui.Rect) {
												draw := func() {
													paintPeelOverlay(p, r, 4, coverBg, shadeCopy, tiltCopy, cellIdx, false, 0, true)
												}
												if sx != 0 || sy != 0 {
													p.Transform(ui.Translation(sx, sy), 0, 0, draw)
												} else {
													draw()
												}
											})
										case peel.stage == tilePeelLifting:
											gridLifting = append(gridLifting, gridLiftingSlip{
												row: row, col: col, idx: i, peelT: peel.peelT, tilt: tiltCopy,
											})
										case !cell.Flagged && hasBandage && bAnim.Kind == bandagePeel:
											gridPeeling = append(gridPeeling, gridPeelingBandage{
												row:    row,
												col:    col,
												shield: bAnim.Shield,
												peelT:  float32(now.Sub(bAnim.Start)) / float32(bandagePeelDuration),
												tilt:   tiltCopy,
											})
										}

										clicks := tile.Clicks()
										if a.gridPress.consumeClicks(i, clicks, tile.Pressed()) > 0 {
											a.cellMove(c, row, col, 0, now)
										}
									}
								})
								hasGridPulses := !a.motionReducedActive && len(a.skillPulses) > 0
								hasGridBursts := !a.motionReducedActive && len(a.knotBursts) > 0
								if len(gridLifting) > 0 || len(gridPeeling) > 0 || hasGridPulses || hasGridBursts {
									liftingCopy := gridLifting
									peelingCopy := gridPeeling
									pulsesCopy := slices.Clone(a.skillPulses)
									burstsCopy := slices.Clone(a.knotBursts)
									cols := a.game.Cols
									slipCol, shadeCol := pal.Slip, pal.SlipShade
									flagBmp, shieldBmp := art.FlagBitmap, art.ShieldBitmap
									sx, sy := shakeX, shakeY
									palCopy := pal
									grid.DrawOver(func(p *ui.Painter, r ui.Rect) {
										drawNow := p.Now()
										drawOverlays := func() {
											for _, ls := range liftingCopy {
												cellRect := ui.Rect{
													X: r.X + gridPad + float32(ls.col)*pitch,
													Y: r.Y + gridPad + float32(ls.row)*pitch,
													W: cellSize,
													H: cellSize,
												}
												paintPeelOverlay(p, cellRect, 4, slipCol, shadeCol, ls.tilt, ls.idx, true, ls.peelT, true)
											}
											for _, pb := range peelingCopy {
												cellRect := ui.Rect{
													X: r.X + gridPad + float32(pb.col)*pitch,
													Y: r.Y + gridPad + float32(pb.row)*pitch,
													W: cellSize,
													H: cellSize,
												}
												bmp := flagBmp
												if pb.shield {
													bmp = shieldBmp
												}
												cx, cy := cellRect.X+cellRect.W/2, cellRect.Y+cellRect.H/2
												p.Transform(ui.Rotation(pb.tilt), cx, cy, func() {
													paintBandagePeel(p, cellRect, bmp, 0.82, pb.peelT)
												})
											}
											if hasGridPulses {
												for _, pulse := range pulsesCopy {
													elapsed := drawNow.Sub(pulse.Start)
													if elapsed < 0 || elapsed >= skillPulseDuration {
														continue
													}
													row, col := pulse.Index/cols, pulse.Index%cols
													cellRect := ui.Rect{
														X: r.X + gridPad + float32(col)*pitch,
														Y: r.Y + gridPad + float32(row)*pitch,
														W: cellSize,
														H: cellSize,
													}
													paintSkillTargetPulse(p, cellRect, 4, pulse.MemberID, palCopy, float32(elapsed)/float32(skillPulseDuration))
												}
											}
											if hasGridBursts {
												for _, burst := range burstsCopy {
													elapsed := drawNow.Sub(burst.Start)
													if elapsed < 0 || elapsed >= knotBurstDuration {
														continue
													}
													row, col := burst.Index/cols, burst.Index%cols
													cellRect := ui.Rect{
														X: r.X + gridPad + float32(col)*pitch,
														Y: r.Y + gridPad + float32(row)*pitch,
														W: cellSize,
														H: cellSize,
													}
													paintKnotBurst(p, cellRect, palCopy, float32(elapsed)/float32(knotBurstDuration))
												}
											}
										}
										if sx != 0 || sy != 0 {
											p.Transform(ui.Translation(sx, sy), 0, 0, drawOverlays)
										} else {
											drawOverlays()
										}
										if hasGridPulses || hasGridBursts {
											p.AnimationFrame()
										}
									})
								}
							})
						})
						a.stageHighlightsOverlay(c, pal, now)
						if nextHoverRow != a.classicHoverRow || nextHoverCol != a.classicHoverCol {
							a.classicHoverRow = nextHoverRow
							a.classicHoverCol = nextHoverCol
							a.classicHoverHint = a.game.InspectNumberHover(nextHoverRow, nextHoverCol, a.flagMode)
							c.After(time.Millisecond)
						}
					})
				})
			})

			a.popoverWorkspaceShield(c)
		})
	})
	a.renderPopoverExitGhost(c, pal, t, now)
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
