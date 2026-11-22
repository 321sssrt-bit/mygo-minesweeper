package main

import "math"

// SynthRate is the sample rate of every synthesized sound, all of them mono.
const SynthRate = 48000

// SoundID names one sound of the synthesized bank.
type SoundID int

const (
	// Stationery foley for everyday moves.
	SoundPaperLift SoundID = iota
	SoundBandageOn
	SoundBandageOff
	SoundToggle
	SoundPopoverOpen
	SoundPopoverClose
	SoundChatNote

	// One short instrumental motif per member, for highlight moments.
	SoundMotifTomori
	SoundMotifAnon
	SoundMotifRana
	SoundMotifSoyo
	SoundMotifTaki
	// SoundHaruhikageStop is Soyo's dissonant accent that cuts off dead.
	SoundHaruhikageStop
	// SoundWinChord is the five-member closing chord of a won game.
	SoundWinChord

	// Duel cues.
	SoundDuelPen
	SoundDuelMineCue
	SoundDuelTick

	// Appended so existing IDs keep their synthesis seeds.
	SoundPaperTouch
	SoundPaperLiftHeavy

	soundCount
)

var soundNames = [soundCount]string{
	SoundPaperLift:      "paper-lift",
	SoundBandageOn:      "bandage-on",
	SoundBandageOff:     "bandage-off",
	SoundToggle:         "toggle",
	SoundPopoverOpen:    "popover-open",
	SoundPopoverClose:   "popover-close",
	SoundChatNote:       "chat-note",
	SoundMotifTomori:    "motif-tomori",
	SoundMotifAnon:      "motif-anon",
	SoundMotifRana:      "motif-rana",
	SoundMotifSoyo:      "motif-soyo",
	SoundMotifTaki:      "motif-taki",
	SoundHaruhikageStop: "haruhikage-stop",
	SoundWinChord:       "win-chord",
	SoundDuelPen:        "duel-pen",
	SoundDuelMineCue:    "duel-mine-cue",
	SoundDuelTick:       "duel-tick",
	SoundPaperTouch:     "paper-touch",
	SoundPaperLiftHeavy: "paper-lift-heavy",
}

func (id SoundID) String() string { return soundNames[id] }

// SoundBank holds every synthesized sound as 48 kHz mono samples in
// [-1, 1]. Each sound owns its backing array.
type SoundBank struct {
	samples [soundCount][]float32
}

// NewSoundBank synthesizes the whole bank from code alone, the same samples
// on every call.
func NewSoundBank() *SoundBank {
	b := &SoundBank{}
	for id := range soundCount {
		recipe := soundRecipes[id]
		r := &synthRand{s: uint64(id+1) * 0x9E3779B97F4A7C15}
		buf := recipe.render(r)
		if recipe.drive > 0 {
			for i, x := range buf {
				buf[i] = math.Tanh(recipe.drive * x)
			}
		}
		b.samples[id] = finish(buf, recipe.peak, recipe.fadeOut)
	}
	return b
}

// Samples returns the sound's samples, not a copy.
func (b *SoundBank) Samples(id SoundID) []float32 { return b.samples[id] }

type soundRecipe struct {
	render func(r *synthRand) []float64
	// peak is the normalized loudness; drive, when set, soft-clips before it.
	peak, drive float64
	fadeOut     float64
}

var soundRecipes = [soundCount]soundRecipe{
	SoundPaperLift:      {render: paperLift, peak: 0.45, fadeOut: 0.01},
	SoundBandageOn:      {render: bandageOn, peak: 0.5, fadeOut: 0.02},
	SoundBandageOff:     {render: bandageOff, peak: 0.5, fadeOut: 0.02},
	SoundToggle:         {render: toggle, peak: 0.4, fadeOut: 0.005},
	SoundPopoverOpen:    {render: popoverOpen, peak: 0.4, fadeOut: 0.03},
	SoundPopoverClose:   {render: popoverClose, peak: 0.4, fadeOut: 0.02},
	SoundChatNote:       {render: chatNote, peak: 0.45, fadeOut: 0.02},
	SoundMotifTomori:    {render: motifTomori, peak: 0.7, fadeOut: 0.2},
	SoundMotifAnon:      {render: motifAnon, peak: 0.7, fadeOut: 0.15},
	SoundMotifRana:      {render: motifRana, peak: 0.7, drive: 1.6, fadeOut: 0.15},
	SoundMotifSoyo:      {render: motifSoyo, peak: 0.75, fadeOut: 0.15},
	SoundMotifTaki:      {render: motifTaki, peak: 0.7, fadeOut: 0.03},
	SoundHaruhikageStop: {render: haruhikageStop, peak: 0.8, drive: 1.2, fadeOut: 0.004},
	SoundWinChord:       {render: winChord, peak: 0.8, fadeOut: 0.35},
	SoundDuelPen:        {render: duelPen, peak: 0.4, fadeOut: 0.01},
	SoundDuelMineCue:    {render: duelMineCue, peak: 0.5, fadeOut: 0.06},
	SoundDuelTick:       {render: duelTick, peak: 0.45, fadeOut: 0.01},
	SoundPaperTouch:     {render: paperTouch, peak: 0.25, fadeOut: 0.005},
	SoundPaperLiftHeavy: {render: paperLiftHeavy, peak: 0.5, fadeOut: 0.015},
}

func paperLift(r *synthRand) []float64 {
	out := samples(0.07)
	addNoise(out, r, 0, filt{kind: bandPass, from: 3200, to: 4200, sweep: 0.05, q: 0.9}, shape{0.002, 0.018}, 1)
	addCrackle(out, r, 0, 260, filt{kind: bandPass, from: 4500, q: 1.4}, shape{0, 0.02}, 1.4)
	return out
}

func paperTouch(r *synthRand) []float64 {
	out := samples(0.028)
	addNoise(out, r, 0, filt{kind: bandPass, from: 1600, q: 0.8}, shape{0.001, 0.006}, 1)
	return out
}

func paperLiftHeavy(r *synthRand) []float64 {
	out := samples(0.095)
	addNoise(out, r, 0, filt{kind: bandPass, from: 1300, to: 3200, sweep: 0.06, q: 0.8}, shape{0.004, 0.046}, 1)
	addCrackle(out, r, 0, 420, filt{kind: bandPass, from: 4000, q: 1.2}, shape{0.003, 0.033}, 0.45)
	return out
}

func bandageOn(r *synthRand) []float64 {
	out := samples(0.12)
	addNoise(out, r, 0, filt{kind: lowPass, from: 900, to: 500, sweep: 0.04, q: 0.7}, shape{0.003, 0.025}, 1)
	addTone(out, 0, 150, shape{0.002, 0.03}, 0.6)
	addCrackle(out, r, seconds(0.03), 140, filt{kind: bandPass, from: 2500, q: 1.2}, shape{0.01, 0.03}, 0.6)
	return out
}

func bandageOff(r *synthRand) []float64 {
	out := samples(0.2)
	addNoise(out, r, 0, filt{kind: bandPass, from: 1400, to: 4800, sweep: 0.15, q: 0.8}, shape{0.14, 0.02}, 0.8)
	addCrackle(out, r, 0, 900, filt{kind: bandPass, from: 3000, q: 1}, shape{0.14, 0.02}, 1.2)
	return out
}

func toggle(r *synthRand) []float64 {
	out := samples(0.045)
	addClick(out, r, 0, 2400, 1)
	addClick(out, r, seconds(0.018), 2000, 0.6)
	return out
}

func popoverOpen(r *synthRand) []float64 {
	out := samples(0.15)
	addNoise(out, r, 0, filt{kind: lowPass, from: 700, to: 5200, sweep: 0.1, q: 0.9}, shape{0.07, 0.04}, 1)
	return out
}

func popoverClose(r *synthRand) []float64 {
	out := samples(0.13)
	addNoise(out, r, 0, filt{kind: lowPass, from: 4800, to: 700, sweep: 0.08, q: 0.9}, shape{0.03, 0.04}, 1)
	tap := seconds(0.09)
	addNoise(out, r, tap, filt{kind: lowPass, from: 600, q: 0.7}, shape{0.002, 0.015}, 0.8)
	addTone(out, tap, 180, shape{0.002, 0.02}, 0.4)
	return out
}

func chatNote(r *synthRand) []float64 {
	out := samples(0.16)
	addNoise(out, r, 0, filt{kind: bandPass, from: 2000, to: 3200, sweep: 0.05, q: 0.8}, shape{0.03, 0.02}, 0.7)
	pat := seconds(0.06)
	addNoise(out, r, pat, filt{kind: lowPass, from: 900, q: 0.7}, shape{0.002, 0.02}, 1)
	addTone(out, pat, 220, shape{0.002, 0.025}, 0.5)
	return out
}

// Motifs use original intervals and chords only: no melody from any song.

func motifTomori(r *synthRand) []float64 {
	out := samples(0.8)
	addBell(out, r, 0, midiHz(83), 1)
	addBell(out, r, seconds(0.17), midiHz(88), 0.9)
	return out
}

func motifAnon(r *synthRand) []float64 {
	out := samples(0.75)
	strum(out, r, 0, []float64{40, 47, 52, 56, 59, 64}, 0.009, 0.6, 0.5)
	strum(out, r, seconds(0.2), []float64{64, 59, 56, 52}, 0.007, 0.7, 0.35)
	return out
}

func motifRana(r *synthRand) []float64 {
	out := samples(0.8)
	addHarmonic(out, 0, midiHz(79), 0.8)
	addHarmonic(out, 0, midiHz(86), 0.5)
	for i, note := range []float64{81, 84, 79, 86} {
		start := seconds(0.24 + 0.075*float64(i))
		addPluck(out, r, start, seconds(0.06), midiHz(note), 0.8, 0.8, 0.6)
	}
	return out
}

func motifSoyo(r *synthRand) []float64 {
	out := samples(0.8)
	addBass(out, r, 0, 33, 1)
	addBass(out, r, seconds(0.22), 40, 0.85)
	return out
}

func motifTaki(r *synthRand) []float64 {
	out := samples(0.6)
	addStick(out, r, 0, 0.7)
	addStick(out, r, seconds(0.13), 0.8)
	addRim(out, r, seconds(0.27), 1)
	addHat(out, r, seconds(0.38), 0.6)
	addHat(out, r, seconds(0.46), 0.5)
	return out
}

func haruhikageStop(r *synthRand) []float64 {
	out := samples(0.36)
	for _, note := range []float64{40, 41, 46} {
		addPluck(out, r, 0, len(out), midiHz(note), 2, 0.9, 0.55)
	}
	for _, note := range []float64{63, 64} {
		addPluck(out, r, 0, len(out), midiHz(note), 1.5, 0.8, 0.3)
	}
	addNoise(out, r, 0, filt{kind: lowPass, from: 1500, q: 0.7}, shape{0.001, 0.02}, 0.5)
	return out
}

func winChord(r *synthRand) []float64 {
	out := samples(1.3)
	addRim(out, r, 0, 0.6)
	addHat(out, r, 0, 0.3)
	addBass(out, r, 0, 38, 0.9)
	strum(out, r, seconds(0.01), []float64{50, 57, 62, 66, 69}, 0.01, 0.6, 0.45)
	addHarmonic(out, seconds(0.03), midiHz(78), 0.4)
	addBell(out, r, seconds(0.05), midiHz(86), 0.5)
	addBell(out, r, seconds(0.2), midiHz(81), 0.4)
	return out
}

func duelPen(r *synthRand) []float64 {
	out := samples(0.05)
	addNoise(out, r, 0, filt{kind: bandPass, from: 3000, to: 2600, sweep: 0.02, q: 1.2}, shape{0.001, 0.008}, 1)
	addTone(out, 0, 900, shape{0.0005, 0.01}, 0.4)
	addCrackle(out, r, 0, 500, filt{kind: bandPass, from: 5000, q: 1.2}, shape{0.002, 0.015}, 0.5)
	return out
}

func duelMineCue(r *synthRand) []float64 {
	out := samples(0.3)
	addPluck(out, r, 0, seconds(0.12), midiHz(64), 0.4, 0.35, 1)
	addPluck(out, r, seconds(0.11), len(out)-seconds(0.11), midiHz(60), 0.4, 0.35, 1)
	addTone(out, 0, 110, shape{0.003, 0.06}, 0.4)
	return out
}

func duelTick(r *synthRand) []float64 {
	out := samples(0.06)
	addTone(out, 0, 1700, shape{0.0003, 0.012}, 1)
	addTone(out, 0, 2900, shape{0.0003, 0.006}, 0.5)
	addNoise(out, r, 0, filt{kind: highPass, from: 4000, q: 0.7}, shape{0.0002, 0.001}, 0.4)
	return out
}

// Instruments shared by the motifs and the closing chord.

func addClick(out []float64, r *synthRand, start int, freq, gain float64) {
	addTone(out, start, freq, shape{0.0003, 0.003}, gain)
	addNoise(out, r, start, filt{kind: highPass, from: 3000, q: 0.7}, shape{0.0002, 0.0012}, 0.5*gain)
}

// addBell is a music-box tine: inharmonic partials that die faster the
// higher they sit, over a faint mallet tick.
func addBell(out []float64, r *synthRand, start int, freq, gain float64) {
	for _, p := range []struct{ ratio, amp, decay float64 }{
		{1, 1, 0.7}, {2.76, 0.3, 0.25}, {5.40, 0.12, 0.12}, {8.93, 0.05, 0.06},
	} {
		addTone(out, start, freq*p.ratio, shape{0.001, p.decay}, gain*p.amp)
	}
	addNoise(out, r, start, filt{kind: highPass, from: 6000, q: 0.7}, shape{0.0002, 0.002}, 0.15*gain)
}

// addHarmonic is a guitar natural harmonic: nearly a pure sine.
func addHarmonic(out []float64, start int, freq, gain float64) {
	addTone(out, start, freq, shape{0.004, 1}, gain)
	addTone(out, start, 2*freq, shape{0.004, 0.5}, 0.15*gain)
	addTone(out, start, 3*freq, shape{0.004, 0.25}, 0.06*gain)
}

func strum(out []float64, r *synthRand, start int, notes []float64, gap, brightness, gain float64) {
	for i, note := range notes {
		at := start + seconds(gap*float64(i))
		addPluck(out, r, at, len(out)-at, midiHz(note), 1.4, brightness, gain)
	}
}

// addBass is a warm fingered string with its fundamental reinforced, which
// small speakers would otherwise lose.
func addBass(out []float64, r *synthRand, start int, note, gain float64) {
	addPluck(out, r, start, len(out)-start, midiHz(note), 1.6, 0.35, gain)
	addTone(out, start, midiHz(note), shape{0.004, 0.6}, 0.5*gain)
}

func addStick(out []float64, r *synthRand, start int, gain float64) {
	addTone(out, start, 2300, shape{0.0002, 0.012}, gain)
	addTone(out, start, 3700, shape{0.0002, 0.007}, 0.5*gain)
	addNoise(out, r, start, filt{kind: highPass, from: 4000, q: 0.7}, shape{0.0002, 0.003}, 0.6*gain)
}

// addRim is a snare rim shot: shell tones, snare-wire noise and the
// stick's crack.
func addRim(out []float64, r *synthRand, start int, gain float64) {
	addTone(out, start, 330, shape{0.0005, 0.05}, 0.7*gain)
	addTone(out, start, 185, shape{0.0005, 0.07}, 0.6*gain)
	addTone(out, start, 1800, shape{0.0002, 0.004}, 0.8*gain)
	addNoise(out, r, start, filt{kind: bandPass, from: 1900, q: 0.7}, shape{0.0005, 0.09}, gain)
}

func addHat(out []float64, r *synthRand, start int, gain float64) {
	addNoise(out, r, start, filt{kind: highPass, from: 7500, q: 0.7}, shape{0.0005, 0.03}, gain)
	for _, freq := range []float64{3140, 4410, 5270, 6680, 7850, 9230} {
		addTone(out, start, freq, shape{0.0005, 0.025}, 0.08*gain)
	}
}

// Synthesis primitives. Each adds into out from start and stops once its
// envelope has faded or out ends.

func seconds(t float64) int { return int(math.Round(t * SynthRate)) }

func samples(t float64) []float64 { return make([]float64, seconds(t)) }

func midiHz(note float64) float64 { return 440 * math.Pow(2, (note-69)/12) }

// shape is a linear attack then an exponential decay, both in seconds.
type shape struct{ attack, decay float64 }

func (s shape) at(t float64) float64 {
	if t < s.attack {
		return t / s.attack
	}
	return math.Exp(-(t - s.attack) / s.decay)
}

func (s shape) faded(t float64) bool { return t > s.attack && s.at(t) < 1e-5 }

func addTone(out []float64, start int, freq float64, s shape, gain float64) {
	for i := start; i < len(out); i++ {
		t := float64(i-start) / SynthRate
		if s.faded(t) {
			break
		}
		out[i] += gain * s.at(t) * math.Sin(2*math.Pi*freq*t)
	}
}

type filterKind int

const (
	lowPass filterKind = iota
	bandPass
	highPass
)

// filt sweeps its cutoff exponentially from from to to over sweep seconds;
// with no sweep it stays at from.
type filt struct {
	kind            filterKind
	from, to, sweep float64
	q               float64
}

func (f filt) cutoff(t float64) float64 {
	if f.sweep == 0 {
		return f.from
	}
	return f.from * math.Pow(f.to/f.from, math.Min(t/f.sweep, 1))
}

// svf is a topology-preserving state-variable filter, which stays stable
// while its cutoff moves every sample.
type svf struct{ ic1, ic2 float64 }

func (v *svf) step(x float64, f filt, t float64) float64 {
	g := math.Tan(math.Pi * f.cutoff(t) / SynthRate)
	k := 1 / f.q
	a1 := 1 / (1 + g*(g+k))
	a2 := g * a1
	a3 := g * a2
	v3 := x - v.ic2
	v1 := a1*v.ic1 + a2*v3
	v2 := v.ic2 + a2*v.ic1 + a3*v3
	v.ic1 = 2*v1 - v.ic1
	v.ic2 = 2*v2 - v.ic2
	switch f.kind {
	case lowPass:
		return v2
	case bandPass:
		return v1
	default:
		return x - k*v1 - v2
	}
}

func addNoise(out []float64, r *synthRand, start int, f filt, s shape, gain float64) {
	var v svf
	for i := start; i < len(out); i++ {
		t := float64(i-start) / SynthRate
		if s.faded(t) {
			break
		}
		out[i] += gain * s.at(t) * v.step(r.signed(), f, t)
	}
}

// addCrackle is sparse random impulses, perSecond on average, rung through
// a filter: paper grain and tape crackle.
func addCrackle(out []float64, r *synthRand, start int, perSecond float64, f filt, s shape, gain float64) {
	var v svf
	p := perSecond / SynthRate
	for i := start; i < len(out); i++ {
		t := float64(i-start) / SynthRate
		if s.faded(t) {
			break
		}
		x := 0.0
		if r.float() < p {
			x = r.signed() * 4
		}
		out[i] += gain * s.at(t) * v.step(x, f, t)
	}
}

// addPluck is a Karplus-Strong string sounding for length samples:
// brightness in (0, 1] lowpasses the pluck, t60 is the ring time in seconds.
func addPluck(out []float64, r *synthRand, start, length int, freq, t60, brightness, gain float64) {
	// The loop is the line, the two-tap average (half a sample) and an
	// allpass carrying the fractional rest, kept within [0.1, 1.1) samples
	// where its delay stays flat, so the string sounds in tune.
	period := SynthRate / freq
	n := int(period - 0.6)
	frac := period - 0.5 - float64(n)
	c := (1 - frac) / (1 + frac)
	line := make([]float64, n)
	lp, mean := 0.0, 0.0
	for i := range line {
		lp += brightness * (r.signed() - lp)
		line[i] = lp
		mean += lp / float64(n)
	}
	top := 0.0
	for i := range line {
		line[i] -= mean
		top = math.Max(top, math.Abs(line[i]))
	}
	loss := math.Pow(10, -3/(t60*freq))
	mute := seconds(0.004)
	end := min(start+length, len(out))
	prev, apIn, apOut := 0.0, 0.0, 0.0
	for i, j := start, 0; i < end; i, j = i+1, (j+1)%n {
		g := gain / top
		if left := end - i; left < mute {
			g *= float64(left) / float64(mute)
		}
		cur := line[j]
		out[i] += g * cur
		avg := 0.5 * (cur + prev)
		apOut = c*avg + apIn - c*apOut
		apIn, prev = avg, cur
		line[j] = loss * apOut
	}
}

// finish fades the edges against clicks, normalizes to peak and copies into
// a fresh float32 slice of exact capacity.
func finish(buf []float64, peak, fadeOut float64) []float32 {
	in, out := seconds(0.001), seconds(fadeOut)
	top := 0.0
	for i := range buf {
		if i < in {
			buf[i] *= float64(i) / float64(in)
		}
		if left := len(buf) - i; left <= out {
			buf[i] *= float64(left-1) / float64(out)
		}
		top = math.Max(top, math.Abs(buf[i]))
	}
	samples := make([]float32, len(buf))
	for i, x := range buf {
		samples[i] = float32(x * peak / top)
	}
	return samples
}

// synthRand is a seeded xorshift generator, so the bank comes out the same
// every time without touching shared random state.
type synthRand struct{ s uint64 }

func (r *synthRand) float() float64 {
	r.s ^= r.s << 13
	r.s ^= r.s >> 7
	r.s ^= r.s << 17
	return float64(r.s>>11) / (1 << 53)
}

func (r *synthRand) signed() float64 { return 2*r.float() - 1 }
