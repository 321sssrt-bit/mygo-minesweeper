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
