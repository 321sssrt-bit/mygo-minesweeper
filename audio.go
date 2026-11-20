package main

import "github.com/egoist/mygo/audio"

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
	return clampPan(maxSoundPan * (2*float64(col)/float64(cols-1) - 1))
}

func clampPan(pan float64) float64 {
	return max(-maxSoundPan, min(maxSoundPan, pan))
}

// playSound plays id at gain (1 = the player's volume) and pan, obeying the
// mute switch and the master volume.
func (a *minesweeperApp) playSound(id SoundID, gain, pan float64) {
	a.cuesPlayed++
	if !a.soundEnabled || a.volume <= 0 || gain <= 0 {
		return
	}
	a.sound.Play(soundCue{
		ID:    id,
		Gain:  gain * float64(min(a.volume, 100)) / 100,
		Pan:   clampPan(pan),
		Speed: 1,
	})
}

// cellActionSound is the stationery foley of a player's move on a cell:
// lifting its slip, or sticking on or peeling off a 旗's bandage. Clearing a
// ? mark touches no bandage and stays silent.
func cellActionSound(kind ActionKind) (SoundID, bool) {
	switch kind {
	case ActionRevealed, ActionWon, ActionLost:
		return SoundPaperLift, true
	case ActionFlagged, ActionShielded:
		return SoundBandageOn, true
	case ActionUnflag, ActionQuestion:
		return SoundBandageOff, true
	}
	return 0, false
}

// flushChatNote plays one note sound for whatever the group chat gained
// since the last flush, however many notes one move posted. A move that
// already made its own sound, as a 旗 posting its quote, makes no second one.
func (a *minesweeperApp) flushChatNote() {
	posted, cued := a.chatPosted != a.chatHeard, a.cuesPlayed != a.cuesHeard
	a.chatHeard = a.chatPosted
	if posted && !cued {
		a.playSound(SoundChatNote, 1, 0)
	}
	a.cuesHeard = a.cuesPlayed
}
