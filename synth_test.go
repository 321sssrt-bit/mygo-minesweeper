package main

import (
	"go/parser"
	"go/token"
	"math"
	"slices"
	"strconv"
	"testing"
	"time"
)

func allSounds() []SoundID {
	ids := make([]SoundID, 0, soundCount)
	for id := range soundCount {
		ids = append(ids, id)
	}
	return ids
}

func TestSynthSamplesAreClean(t *testing.T) {
	bank := NewSoundBank()
	for _, id := range allSounds() {
		s := bank.Samples(id)
		if len(s) == 0 {
			t.Errorf("%v: no samples", id)
			continue
		}
		var peak, energy float64
		for i, x := range s {
			v := float64(x)
			if math.IsNaN(v) || math.IsInf(v, 0) {
				t.Fatalf("%v: sample %d is %v", id, i, v)
			}
			peak = math.Max(peak, math.Abs(v))
			energy += v * v
		}
		rms := math.Sqrt(energy / float64(len(s)))
		if peak >= 1 {
			t.Errorf("%v: peak %.4f clips", id, peak)
		}
		if rms < 1e-3 {
			t.Errorf("%v: rms %.6f is silent", id, rms)
		}
		t.Logf("%-16v %6d samples  peak %.3f  rms %.4f", id, len(s), peak, rms)
	}
}

func TestSynthMotifLoudness(t *testing.T) {
	bank := NewSoundBank()
	measure := func(id SoundID) float64 {
		db := shortSoundLevel(bank.Samples(id))
		t.Logf("%-16v %6.2f dB", id, db)
		return db
	}
	quietest, loudest := math.Inf(1), math.Inf(-1)
	for id := SoundMotifTomori; id <= SoundMotifTaki; id++ {
		db := measure(id)
		quietest, loudest = min(quietest, db), max(loudest, db)
	}
	t.Logf("motif spread %.2f dB", loudest-quietest)
	if loudest-quietest > 2 {
		t.Errorf("motif spread %.2f dB exceeds 2 dB", loudest-quietest)
	}

	foley := []SoundID{
		SoundPaperTouch, SoundPaperLift, SoundPaperLiftHeavy, SoundBandageOn, SoundBandageOff,
		SoundToggle, SoundPopoverOpen, SoundPopoverClose, SoundChatNote,
	}
	levels := make([]float64, len(foley))
	for i, id := range foley {
		levels[i] = measure(id)
	}
	// Keep the intentionally soft paper touch and strong heavy lift from setting the reference.
	slices.Sort(levels)
	baseline := levels[len(levels)/2]
	t.Logf("foley median %.2f dB, motifs %.2f to %.2f dB above it", baseline, quietest-baseline, loudest-baseline)
	if quietest-baseline < 4 || loudest-baseline > 8 {
		t.Error("each motif must be 4 to 8 dB above the foley median")
	}
	for _, id := range []SoundID{SoundWinChord, SoundHaruhikageStop} {
		if db := measure(id); db > quietest {
			t.Errorf("%v is %.2f dB louder than the quietest motif", id, db-quietest)
		}
	}

	// A win starts the closing chord and the leader's motif together.
	chord := bank.Samples(SoundWinChord)
	for id := SoundMotifTomori; id <= SoundMotifTaki; id++ {
		motif := bank.Samples(id)
		var peak float64
		for i := range min(len(chord), len(motif)) {
			peak = max(peak, math.Abs(float64(chord[i])+float64(motif[i])))
		}
		t.Logf("%v + %v peak %.3f", id, SoundWinChord, peak)
		if peak >= 1 {
			t.Errorf("%v + %v clips at %.3f", id, SoundWinChord, peak)
		}
	}
}

// Use the loudest 50 ms window so silence and reverb tails do not dilute
// the level of a short cue. Filter coefficients are BS.1770 K-weighting at 48 kHz.
func shortSoundLevel(samples []float32) float64 {
	const window = SynthRate / 20
	var energy [window]float64
	var shelf1, shelf2, highPass1, highPass2 float64
	var sum, peak float64
	for i, sample := range samples {
		x := float64(sample)
		shelf := 1.53512485958697*x + shelf1
		shelf1 = -2.69169618940638*x + 1.69065929318241*shelf + shelf2
		shelf2 = 1.19839281085285*x - 0.73248077421585*shelf
		weighted := shelf + highPass1
		highPass1 = -2*shelf + 1.99004745483398*weighted + highPass2
		highPass2 = shelf - 0.99007225036621*weighted

		slot := i % window
		sum -= energy[slot]
		energy[slot] = weighted * weighted
		sum += energy[slot]
		peak = max(peak, sum)
	}
	return 10 * math.Log10(peak/window)
}

func TestSynthPaperFoley(t *testing.T) {
	bank := NewSoundBank()
	measure := func(id SoundID) (length int, peak, energy float64) {
		samples := bank.Samples(id)
		for _, sample := range samples {
			value := float64(sample)
			peak = math.Max(peak, math.Abs(value))
			energy += value * value
		}
		t.Logf("%s: %d samples (%.1f ms), peak %.3f, energy %.3f",
			id, len(samples), 1000*float64(len(samples))/SynthRate, peak, energy)
		return len(samples), peak, energy
	}

	liftLength, liftPeak, liftEnergy := measure(SoundPaperLift)
	touchLength, touchPeak, touchEnergy := measure(SoundPaperTouch)
	heavyLength, heavyPeak, heavyEnergy := measure(SoundPaperLiftHeavy)
	if SoundPaperTouch.String() != "paper-touch" || SoundPaperLiftHeavy.String() != "paper-lift-heavy" {
		t.Error("paper foley names must be paper-touch and paper-lift-heavy")
	}
	if touchLength < liftLength/4 || touchLength > liftLength/2 || touchPeak < liftPeak/2 || touchPeak >= liftPeak || touchEnergy >= liftEnergy {
		t.Error("paper touch should be much shorter and quieter than paper lift")
	}
	if heavyLength <= liftLength || heavyLength > 3*liftLength/2 || heavyPeak < liftPeak || heavyPeak > 1.25*liftPeak || heavyEnergy < 1.5*liftEnergy {
		t.Error("heavy paper lift should be slightly longer with similar peak and at least 50% more energy")
	}
}

func TestSynthMineThud(t *testing.T) {
	if SoundMineThud.String() != "mine-thud" {
		t.Errorf("mine thud name = %q, want mine-thud", SoundMineThud)
	}
	samples := NewSoundBank().Samples(SoundMineThud)
	duration := float64(len(samples)) / SynthRate
	if duration > 0.4 {
		t.Errorf("mine thud lasts %.1f ms, want at most 400 ms", duration*1000)
	}

	var total, low, sum float64
	for _, sample := range samples {
		x := float64(sample)
		total += x * x
		sum += x
	}
	low = sum * sum
	// Use Goertzel to measure only the DFT bins up to 250 Hz.
	// Each positive-frequency bin also counts its negative-frequency mirror.
	for bin := 1; bin*SynthRate <= 250*len(samples); bin++ {
		coeff := 2 * math.Cos(2*math.Pi*float64(bin)/float64(len(samples)))
		var prev, beforePrev float64
		for _, sample := range samples {
			prev, beforePrev = float64(sample)+coeff*prev-beforePrev, prev
		}
		low += 2 * (prev*prev + beforePrev*beforePrev - coeff*prev*beforePrev)
	}
	fraction := low / (float64(len(samples)) * total)
	t.Logf("mine thud: %.1f ms, %.1f%% of energy at or below 250 Hz", duration*1000, fraction*100)
	if !(fraction >= 0.8) {
		t.Errorf("mine thud low-frequency energy fraction = %.3f, want at least 0.8", fraction)
	}
}

func TestSynthDuelClose(t *testing.T) {
	if SoundDuelClose.String() != "duel-close" {
		t.Errorf("duel close name = %q, want duel-close", SoundDuelClose)
	}
	bank := NewSoundBank()
	timeToPeak90 := func(id SoundID) time.Duration {
		samples := bank.Samples(id)
		var peak float64
		for _, sample := range samples {
			peak = math.Max(peak, math.Abs(float64(sample)))
		}
		first := slices.IndexFunc(samples, func(sample float32) bool {
			return math.Abs(float64(sample)) >= 0.9*peak
		})
		return time.Duration(first) * time.Second / SynthRate
	}
	closeAttack := timeToPeak90(SoundDuelClose)
	stopAttack := timeToPeak90(SoundHaruhikageStop)
	t.Logf("time to 90%% of peak: duel close %v, Haruhikage stop %v", closeAttack, stopAttack)
	if closeAttack <= stopAttack {
		t.Errorf("duel close attack %v must be slower than Haruhikage stop %v", closeAttack, stopAttack)
	}
}

func TestSynthSoundsOwnTheirSamples(t *testing.T) {
	bank := NewSoundBank()
	want := make(map[SoundID][]float32)
	for _, id := range allSounds() {
		want[id] = slices.Clone(bank.Samples(id))
	}
	for _, changed := range allSounds() {
		s := bank.Samples(changed)
		for i := range s {
			s[i] = 0.5
		}
		_ = append(s, 0.5, 0.5, 0.5)
		for _, id := range allSounds() {
			if id != changed && !slices.Equal(bank.Samples(id), want[id]) {
				t.Errorf("changing %v altered %v", changed, id)
			}
		}
		copy(s, want[changed])
	}
}

func TestSynthBankBudget(t *testing.T) {
	bank := NewSoundBank()
	var total int
	for _, id := range allSounds() {
		total += len(bank.Samples(id))
	}
	length := time.Duration(total) * time.Second / SynthRate
	bytes := total * 4
	t.Logf("%d sounds, %d samples at %d Hz: %v, %d bytes", soundCount, total, SynthRate, length, bytes)
	if length > 8*time.Second {
		t.Errorf("bank plays %v, want a few seconds", length)
	}
	if bytes > 2<<20 {
		t.Errorf("bank holds %d bytes, want at most 2 MiB", bytes)
	}
}

func TestSynthIsPure(t *testing.T) {
	a, b := NewSoundBank(), NewSoundBank()
	for _, id := range allSounds() {
		if !slices.Equal(a.Samples(id), b.Samples(id)) {
			t.Errorf("%v differs between two banks", id)
		}
	}
	file, err := parser.ParseFile(token.NewFileSet(), "synth.go", nil, parser.ImportsOnly)
	if err != nil {
		t.Fatal(err)
	}
	for _, spec := range file.Imports {
		path, _ := strconv.Unquote(spec.Path.Value)
		if path != "math" {
			t.Errorf("synth.go imports %q; synthesis must be math alone, with no files or audio output", path)
		}
	}
}
