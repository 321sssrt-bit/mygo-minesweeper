package main

import (
	"math"
	"slices"
	"time"

	"github.com/egoist/mygo/audio"
	"github.com/egoist/mygo/ui"
)

// maxSoundPan bounds the stereo placement of every cue, so a board edge
// leans to one ear without leaving the other.
const maxSoundPan = 0.35

// soundCue is one sound to play: Gain is already scaled by the player's
// volume, Pan is clamped to ±maxSoundPan, Speed 1 plays at recorded pitch.
type soundCue struct {
	ID    SoundID
	Gain  float64
	Pan   float64
	Speed float64
}

// soundOutput plays cues. Only the desktop entry point binds a real output;
// every other constructor plays into silentSound.
type soundOutput interface {
	Play(cue soundCue)
}

type silentSound struct{}

func (silentSound) Play(soundCue) {}

// soundVoice is the part of *audio.Player a cue needs, so tests can stand in
// for the system output.
type soundVoice interface {
	SetVolume(v float64)
	SetPan(pan float64)
	SetSpeed(s float64)
	Play()
}

type audioVoice struct{ p *audio.Player }

func (v audioVoice) SetVolume(g float64) { v.p.SetVolume(g) }
func (v audioVoice) SetPan(pan float64)  { v.p.SetPan(pan) }
func (v audioVoice) SetSpeed(s float64)  { v.p.SetSpeed(s) }
func (v audioVoice) Play()               { v.p.Play() }

// desktopSound plays the synthesized bank through mygo/audio.
type desktopSound struct {
	sounds   [soundCount]*audio.Sound
	newVoice func(*audio.Sound) soundVoice
}

func newDesktopSound(bank *SoundBank) *desktopSound {
	d := &desktopSound{newVoice: func(s *audio.Sound) soundVoice {
		return audioVoice{s.NewPlayer()}
	}}
	for id := range soundCount {
		d.sounds[id] = audio.NewSound(SynthRate, 1, bank.Samples(id))
	}
	return d
}

func (d *desktopSound) Play(cue soundCue) {
	v := d.newVoice(d.sounds[cue.ID])
	// A paused player takes its gain, pan and speed before its first buffer
	// mixes; set after Play, the opening milliseconds would sound at defaults.
	v.SetVolume(cue.Gain)
	v.SetPan(cue.Pan)
	v.SetSpeed(cue.Speed)
	v.Play()
}

// columnPan places a board column on the stereo field, the leftmost column
// at -maxSoundPan and the rightmost at +maxSoundPan.
func columnPan(col, cols int) float64 {
	if cols <= 1 {
		return 0
	}
	return maxSoundPan * (2*float64(col)/float64(cols-1) - 1)
}

func clampPan(pan float64) float64 {
	return max(-maxSoundPan, min(maxSoundPan, pan))
}

// playSound plays id at gain (1 = the player's volume) and pan, obeying the
// mute switch and the master volume.
func (a *minesweeperApp) playSound(id SoundID, gain, pan float64) {
	// A classic 雷 hit cuts off any queued grains or skill motif right away;
	// waiting for settleSound would let view's flushCues fire a cue due on the
	// same frame before pendingResult reaches handleAction.
	if id == SoundMineThud && !a.isDuel() {
		a.clearCues()
	}
	a.playCue(soundCue{ID: id, Gain: gain, Pan: pan, Speed: 1})
}

// playPaperTouch voices a cell entering the armed press state, panned by column.
func (a *minesweeperApp) playPaperTouch(col int) {
	a.playSound(SoundPaperTouch, 1, columnPan(col, a.game.Cols))
}

// playCue plays cue with its Gain still relative to the player's volume.
func (a *minesweeperApp) playCue(cue soundCue) {
	if !a.soundEnabled || a.volume == 0 {
		return
	}
	cue.Gain *= float64(a.volume) / 100
	cue.Pan = clampPan(cue.Pan)
	a.sound.Play(cue)
}

const (
	// paperGrainGap is the least time between a move's hit and its follow-up
	// flood grains, or between queued flood grains, so stacked slips never clip.
	paperGrainGap = 38 * time.Millisecond
	// paperGrainsPerWave caps the grains a single flood lifts.
	paperGrainsPerWave = 6
	// paperGrainRise raises a grain's pitch for each wave step it lies out.
	paperGrainRise = 0.012
	// revealStreakWindow is the longest pause between consecutive reveals that
	// still climbs the primary paper lift's pitch.
	revealStreakWindow = 450 * time.Millisecond
	// revealStreakRise is the pitch step added per rapid reveal within
	// revealStreakWindow.
	revealStreakRise = 0.05
	// revealStreakMaxSpeed caps the primary paper lift's pitch multiplier on a
	// rapid reveal streak.
	revealStreakMaxSpeed = 1.25
	// cueLateness is how late a queued cue may still sound. Past it, as after
	// a stalled frame or a window coming back, the cue is dropped.
	cueLateness = 30 * time.Millisecond
	// bigFloodCells is the reveal size that tints a flood with the leader's
	// 代表色 wash and delays a flooding skill's motif to its wave close.
	bigFloodCells = 10

	// duelPenGain voices 高松灯's pen at a quarter of the player's volume.
	duelPenGain = 0.25
	// duelCueGain keeps the duel's 雷 cue and countdown ticks soft.
	duelCueGain = 0.5
	// countdownFrom is the first whole second of the duel's countdown.
	countdownFrom = 10
	// countdownLateness is the most duel time one frame may cover and still
	// tick the second it crossed; a frame catching up on a stall or a hidden
	// window lets the second pass silently.
	countdownLateness = 0.25
)

// queuedCue is a sound due later, its Gain relative to the player's volume.
type queuedCue struct {
	At    time.Time
	Cue   soundCue
	Grain bool
}

// cueQueue holds the sounds a move schedules ahead: the paper grains along a
// flood and a flooding skill's closing motif, plus the rapid-reveal pitch streak.
type cueQueue struct {
	pending      []queuedCue
	lastGrain    time.Time
	lastReveal   time.Time
	revealStreak int
}

// revealSpeed advances the rapid-reveal streak for a reveal at now and returns
// the primary paper lift's pitch multiplier.
func (q *cueQueue) revealSpeed(now time.Time) float64 {
	gap := now.Sub(q.lastReveal)
	if !q.lastReveal.IsZero() && gap >= 0 && gap <= revealStreakWindow {
		q.revealStreak++
	} else {
		q.revealStreak = 0
	}
	q.lastReveal = now
	return min(revealStreakMaxSpeed, 1+float64(q.revealStreak)*revealStreakRise)
}

func (q *cueQueue) add(c queuedCue) {
	i := len(q.pending)
	for i > 0 && q.pending[i-1].At.After(c.At) {
		i--
	}
	q.pending = slices.Insert(q.pending, i, c)
}

// grainFits reports whether a grain at at keeps paperGrainGap from every
// grain played or queued.
func (q *cueQueue) grainFits(at time.Time) bool {
	near := func(t time.Time) bool {
		d := at.Sub(t)
		return d > -paperGrainGap && d < paperGrainGap
	}
	if !q.lastGrain.IsZero() && near(q.lastGrain) {
		return false
	}
	for _, c := range q.pending {
		if c.Grain && near(c.At) {
			return false
		}
	}
	return true
}

// next is when the earliest queued cue falls due.
func (q *cueQueue) next() (time.Time, bool) {
	if len(q.pending) == 0 {
		return time.Time{}, false
	}
	return q.pending[0].At, true
}

// clearCues drops every queued sound and resets the rapid-reveal pitch streak,
// as a new game starts.
func (a *minesweeperApp) clearCues() {
	a.cues.pending = nil
	a.cues.lastReveal = time.Time{}
	a.cues.revealStreak = 0
}

// flushCues plays the queued cues now due and drops those overdue past
// cueLateness. A grain that a late frame would sound within paperGrainGap
// of the last one is dropped too.
func (a *minesweeperApp) flushCues(now time.Time) {
	var due []queuedCue
	kept := a.cues.pending[:0]
	for _, c := range a.cues.pending {
		switch {
		case c.At.After(now):
			kept = append(kept, c)
		case now.Sub(c.At) <= cueLateness:
			due = append(due, c)
		}
	}
	a.cues.pending = kept
	for _, c := range due {
		if c.Grain {
			if !a.cues.lastGrain.IsZero() && now.Sub(a.cues.lastGrain) < paperGrainGap {
				continue
			}
			a.cues.lastGrain = now
		}
		a.playCue(c.Cue)
	}
}

// scheduleCues asks for a frame when the next queued cue falls due.
func (a *minesweeperApp) scheduleCues(c *ui.Context, now time.Time) {
	if at, ok := a.cues.next(); ok {
		c.After(max(time.Millisecond, at.Sub(now)))
	}
}

// waveSpan is how long a reveal wave takes to reach its last step.
func waveSpan(wave []RevealWaveEntry) time.Duration {
	step := 0
	for _, e := range wave {
		step = max(step, waveStep(e.Depth))
	}
	return time.Duration(step) * rippleStepDelay
}

// playFlood sounds the move's primary lift immediately (SoundPaperLift for a
// cell reveal, SoundPaperLiftHeavy for an effective chord), rising in pitch
// across rapid reveals within revealStreakWindow, then queues follow-up grains
// along its reveal wave: each wave step that keeps paperGrainGap from other
// grains, up to paperGrainsPerWave total (including the primary hit), rising
// in pitch with depth.
func (a *minesweeperApp) playFlood(now time.Time, pan float64, primary SoundID, wave []RevealWaveEntry) {
	baseSpeed := a.cues.revealSpeed(now)
	a.playCue(soundCue{ID: primary, Gain: 1, Pan: pan, Speed: baseSpeed})
	a.cues.lastGrain = now
	var steps [waveSteps + 1]bool
	for _, e := range wave {
		steps[waveStep(e.Depth)] = true
	}
	lifted := 1
	for step := 1; step <= waveSteps; step++ {
		at := now.Add(time.Duration(step) * rippleStepDelay)
		if !steps[step] || lifted == paperGrainsPerWave || !a.cues.grainFits(at) {
			continue
		}
		a.cues.add(queuedCue{At: at, Grain: true, Cue: soundCue{
			ID:    SoundPaperLift,
			Gain:  1,
			Pan:   pan,
			Speed: baseSpeed + paperGrainRise*float64(step),
		}})
		lifted++
	}
}

// memberMotif is the short instrumental phrase of a band member.
func memberMotif(id MemberID) SoundID {
	switch id {
	case MemberAnon:
		return SoundMotifAnon
	case MemberRana:
		return SoundMotifRana
	case MemberSoyorin:
		return SoundMotifSoyo
	case MemberTaki:
		return SoundMotifTaki
	}
	return SoundMotifTomori
}

// highlightSound voices a skill that took effect, Soyo's 护盾 and a classic
// game's settlement. A skill that floods at least bigFloodCells delays its
// motif until the wave closes; ordinary floods only lift paper. A duel ends
// in settleDuel instead, and a 雷 there only costs points.
func (a *minesweeperApp) highlightSound(res ActionResult, now time.Time) {
	switch res.Kind {
	case ActionSkill:
		motif := memberMotif(a.memberID)
		if res.RevealedCount >= bigFloodCells {
			a.closeFlood(now, res.RevealWave, motif)
		} else {
			a.playSound(motif, 1, 0)
		}
	case ActionShielded:
		a.playSound(SoundMotifSoyo, 1, 0)
	case ActionWon, ActionLost:
		if !a.isDuel() {
			a.settleSound(res.Kind == ActionWon, res.Kind == ActionLost && a.game.Difficulty.Haruhikage)
		}
	}
}

// closeFlood queues motif for the moment a reveal wave reaches its last step.
func (a *minesweeperApp) closeFlood(now time.Time, wave []RevealWaveEntry, motif SoundID) {
	a.cues.add(queuedCue{
		At:  now.Add(waveSpan(wave)),
		Cue: soundCue{ID: motif, Gain: 1, Speed: 1},
	})
	a.flushCues(now)
}

// settleSound voices game settlement: a classic non-《春日影》 loss stays
// silent here because the 雷 thud already voiced the move, while a duel draw
// or ordinary loss closes gently without a member motif.
func (a *minesweeperApp) settleSound(won, haruhikage bool) {
	if haruhikage {
		a.playSound(SoundHaruhikageStop, 1, 0)
		return
	}
	if won {
		a.playSound(memberMotif(a.memberID), 1, 0)
		a.playSound(SoundWinChord, 1, 0)
		return
	}
	if a.isDuel() {
		a.playSound(SoundDuelClose, 1, 0)
	}
}

// duelHint plays one of the duel's passive cues, which a window in the
// background keeps quiet and never replays, the tick that brings it back
// included.
func (a *minesweeperApp) duelHint(id SoundID, gain, pan float64) {
	if a.isWindowFocused() && a.duelTickFocused {
		a.playSound(id, gain, pan)
	}
}

// voiceAITurn sounds 高松灯's move: her pen when she opens a cell the player
// can see, a cue when she hits a 雷. Her flags stay silent.
func (a *minesweeperApp) voiceAITurn(kind ActionKind, cell int) {
	g := a.game
	row, col := cell/g.Cols, cell%g.Cols
	pan := columnPan(col, g.Cols)
	switch kind {
	case ActionRevealed, ActionWon:
		if a.board.shows(g, row, col) {
			a.duelHint(SoundDuelPen, duelPenGain, pan)
		}
	case ActionLost:
		a.duelHint(SoundDuelMineCue, duelCueGain, pan)
	}
}

// voiceCountdown ticks when the duel clock, going from before to after
// seconds left within one ordinary frame, crosses a whole second of its last
// countdownFrom. Seconds a stall or a hidden window skipped past are never
// replayed.
func (a *minesweeperApp) voiceCountdown(before, after float64) {
	s := math.Ceil(after)
	if s >= 1 && s <= countdownFrom && s < math.Ceil(before) && before-after <= countdownLateness {
		a.duelHint(SoundDuelTick, duelCueGain, 0)
	}
}

// cellActionSound is the immediate foley of a player's move on a cell:
// lifting its slip, a dull thud on a 雷, or sticking on or peeling off a 旗's
// bandage. Clearing a ? mark touches no bandage and stays silent.
func cellActionSound(kind ActionKind) (SoundID, bool) {
	switch kind {
	case ActionRevealed, ActionWon:
		return SoundPaperLift, true
	case ActionLost:
		return SoundMineThud, true
	case ActionFlagged, ActionShielded:
		return SoundBandageOn, true
	case ActionUnflag, ActionQuestion:
		return SoundBandageOff, true
	}
	return 0, false
}

// chatVoicedSince marks the notes posted since posted as already heard: the
// move that posted them, as a 旗 posting its quote, made its own sound.
func (a *minesweeperApp) chatVoicedSince(posted int) {
	a.chatHeard += a.chatPosted - posted
}

// flushChatNote plays one note sound for the notes no move has voiced since
// the last flush, however many there are. Notes that arrive while the window
// is in the background stay silent and are not replayed on return.
func (a *minesweeperApp) flushChatNote() {
	if a.chatPosted == a.chatHeard {
		return
	}
	a.chatHeard = a.chatPosted
	if a.isWindowFocused() {
		a.playSound(SoundChatNote, 1, 0)
	}
}

// useSkill fires the leader's skill; the line a skill that takes effect
// posts rides on its motif, and an empty press stays silent.
func (a *minesweeperApp) useSkill(c *ui.Context, now time.Time) {
	posted := a.chatPosted
	res := a.game.UseSkill()
	a.handleAction(c, res, now)
	if res.Kind != ActionNoop {
		a.chatVoicedSince(posted)
	}
}

// cellMove applies a click on a classic grid cell; the notes its outcome
// posts ride on the move's own foley.
func (a *minesweeperApp) cellMove(c *ui.Context, row, col, button int, now time.Time) {
	posted := a.chatPosted
	res := a.applyCellAction(row, col, button, now)
	a.handleAction(c, res, now)
	if _, voiced := cellActionSound(res.Kind); voiced {
		a.chatVoicedSince(posted)
	}
}
