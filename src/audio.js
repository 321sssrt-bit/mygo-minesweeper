class SoundEngine {
  constructor() {
    this.ctx = null;
    this.muted = false;
  }

  ensureContext() {
    if (this.muted) return null;
    if (!this.ctx && typeof window !== 'undefined') {
      const AudioCtx = window.AudioContext || window.webkitAudioContext;
      if (AudioCtx) {
        this.ctx = new AudioCtx();
      }
    }
    if (this.ctx && this.ctx.state === 'suspended') {
      this.ctx.resume().catch(() => {});
    }
    return this.ctx;
  }

  playTone({ freq = 440, type = 'triangle', duration = 0.08, gain = 0.08, delay = 0, slideTo = null }) {
    const ctx = this.ensureContext();
    if (!ctx) return;

    const now = ctx.currentTime + delay;
    const osc = ctx.createOscillator();
    const vol = ctx.createGain();

    osc.type = type;
    osc.frequency.setValueAtTime(freq, now);
    if (slideTo) {
      osc.frequency.exponentialRampToValueAtTime(slideTo, now + duration);
    }

    vol.gain.setValueAtTime(0.0001, now);
    vol.gain.exponentialRampToValueAtTime(gain, now + 0.01);
    vol.gain.exponentialRampToValueAtTime(0.0001, now + duration);

    osc.connect(vol);
    vol.connect(ctx.destination);

    osc.start(now);
    osc.stop(now + duration + 0.02);
  }

  reveal(count = 1) {
    if (count >= 8) {
      // Bright guitar arpeggio cascade
      [392, 493.88, 587.33, 783.99].forEach((f, idx) => {
        this.playTone({ freq: f, type: 'triangle', duration: 0.14, gain: 0.06, delay: idx * 0.035 });
      });
    } else {
      this.playTone({ freq: 520, slideTo: 640, type: 'sine', duration: 0.055, gain: 0.06 });
    }
  }

  flag(isFlagged) {
    if (isFlagged) {
      this.playTone({ freq: 660, type: 'triangle', duration: 0.06, gain: 0.06 });
      this.playTone({ freq: 880, type: 'triangle', duration: 0.08, gain: 0.05, delay: 0.04 });
    } else {
      this.playTone({ freq: 440, slideTo: 330, type: 'sine', duration: 0.06, gain: 0.05 });
    }
  }

  skill() {
    [523.25, 659.25, 783.99, 1046.5].forEach((f, idx) => {
      this.playTone({ freq: f, type: 'triangle', duration: 0.15, gain: 0.07, delay: idx * 0.04 });
    });
  }

  shield() {
    this.playTone({ freq: 440, type: 'sine', duration: 0.22, gain: 0.09 });
    this.playTone({ freq: 554.37, type: 'triangle', duration: 0.25, gain: 0.07, delay: 0.06 });
  }

  boom() {
    // Broken guitar string + low impact
    this.playTone({ freq: 180, slideTo: 48, type: 'sawtooth', duration: 0.38, gain: 0.12 });
    this.playTone({ freq: 233, slideTo: 110, type: 'square', duration: 0.25, gain: 0.07, delay: 0.03 });
  }

  win() {
    const notes = [523.25, 659.25, 783.99, 987.77, 1046.5];
    notes.forEach((f, idx) => {
      this.playTone({ freq: f, type: 'triangle', duration: 0.24, gain: 0.08, delay: idx * 0.07 });
    });
  }
}

export const sound = new SoundEngine();
