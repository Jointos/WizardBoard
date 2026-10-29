import type { LastActionView } from './types';

export type ResultKind = 'win' | 'lose' | 'neutral';

/**
 * Tiny synthesized sound engine (Web Audio, no asset files). The context is
 * created lazily on the first user gesture so autoplay policies are respected.
 */
export class AudioFX {
  private ctx: AudioContext | null = null;
  private master: GainNode | null = null;
  private noiseBuf: AudioBuffer | null = null;
  private enabled = true;
  private last = new Map<string, number>();

  constructor() {
    try {
      this.enabled = localStorage.getItem('wb-sound') !== '0';
    } catch {
      this.enabled = true;
    }
  }

  isEnabled(): boolean {
    return this.enabled;
  }

  setEnabled(on: boolean): void {
    this.enabled = on;
    try {
      localStorage.setItem('wb-sound', on ? '1' : '0');
    } catch {
      // localStorage may be unavailable; keep the in-memory setting.
    }
    if (this.ctx && this.master) {
      this.master.gain.setTargetAtTime(on ? 0.5 : 0, this.ctx.currentTime, 0.01);
    }
  }

  toggle(): boolean {
    const on = !this.enabled;
    this.setEnabled(on);
    if (on) {
      this.unlock();
      this.click();
    }
    return on;
  }

  unlock(): void {
    if (!this.ctx) {
      const Ctor =
        window.AudioContext ??
        (window as unknown as { webkitAudioContext?: typeof AudioContext }).webkitAudioContext;
      if (!Ctor) return;
      try {
        this.ctx = new Ctor();
        this.master = this.ctx.createGain();
        this.master.gain.value = this.enabled ? 0.5 : 0;
        this.master.connect(this.ctx.destination);
        this.noiseBuf = this.makeNoise();
      } catch {
        this.ctx = null;
        this.master = null;
        return;
      }
    }
    if (this.ctx.state === 'suspended') void this.ctx.resume();
  }

  click(): void {
    if (!this.enabled || !this.guard('click', 25)) return;
    this.tone(520, 0.03, 'triangle', 0.04);
  }

  undo(): void {
    if (!this.enabled) return;
    this.unlock();
    this.tone(320, 0.14, 'triangle', 0.08, 0, 130);
    this.noise(0.08, 0.02, 500, 'lowpass');
  }

  error(): void {
    if (!this.enabled) return;
    this.unlock();
    this.tone(150, 0.2, 'sawtooth', 0.07, 0, 90);
  }

  result(kind: ResultKind): void {
    if (!this.enabled) return;
    this.unlock();
    if (kind === 'win') {
      [523, 659, 784, 1047].forEach((f, i) => this.tone(f, 0.22, 'triangle', 0.1, i * 0.11));
    } else if (kind === 'lose') {
      [392, 330, 262, 196].forEach((f, i) => this.tone(f, 0.28, 'sine', 0.09, i * 0.13));
    } else {
      [392, 523].forEach((f, i) => this.tone(f, 0.2, 'sine', 0.05, i * 0.08));
    }
  }

  action(a: LastActionView): void {
    if (!this.enabled) return;
    this.unlock();
    switch (a.kind) {
      case 'move':
      case 'freemove':
        this.move();
        break;
      case 'moveturn':
        this.move();
        this.turn();
        break;
      case 'turn':
        this.turn();
        break;
      case 'capture':
        if (a.targetType === 'wizard') this.takedown();
        else this.capture();
        break;
      case 'shoot': {
        this.shoot();
        if (a.targetType) {
          const big = a.targetType === 'wizard';
          window.setTimeout(() => {
            if (!this.enabled) return;
            if (big) this.takedown();
            else this.capture();
          }, 110);
        }
        break;
      }
      case 'cast':
        this.cast(a.spell);
        break;
      case 'teleport':
        this.teleport();
        break;
      case 'swap':
        this.move();
        break;
      case 'skip':
      case 'pass':
        this.skip();
        break;
      default:
        this.click();
    }
  }

  private move(): void {
    if (!this.guard('move', 25)) return;
    this.tone(180, 0.09, 'triangle', 0.14, 0, 120);
    this.noise(0.035, 0.025, 480, 'lowpass');
  }

  private turn(): void {
    if (!this.guard('turn', 25)) return;
    this.tone(240, 0.045, 'square', 0.045);
    this.noise(0.02, 0.015, 1400, 'highpass');
  }

  private capture(): void {
    if (!this.guard('capture', 35)) return;
    this.tone(95, 0.18, 'sine', 0.3, 0, 45);
    this.noise(0.06, 0.1, 900, 'lowpass');
  }

  private takedown(): void {
    if (!this.guard('takedown', 60)) return;
    this.tone(80, 0.42, 'sine', 0.34, 0, 34);
    this.noise(0.3, 0.1, 700, 'bandpass', 0, 160, 0.8);
    this.tone(660, 0.18, 'triangle', 0.05, 0.04, 330);
  }

  private shoot(): void {
    if (!this.guard('shoot', 35)) return;
    this.tone(700, 0.12, 'sawtooth', 0.09, 0, 180);
    this.noise(0.05, 0.07, 1800, 'highpass');
  }

  private cast(spell: string): void {
    switch (spell) {
      case 'buff':
        [440, 660, 880].forEach((f, i) => this.tone(f, 0.13, 'sine', 0.08, i * 0.07));
        break;
      case 'nerf':
        this.tone(320, 0.3, 'sawtooth', 0.08, 0, 110);
        this.noise(0.18, 0.035, 420, 'lowpass');
        break;
      case 'cleanse':
        [880, 1320, 1760].forEach((f, i) => this.tone(f, 0.12, 'sine', 0.05, i * 0.05));
        break;
      default:
        this.tone(440, 0.12, 'sine', 0.06);
    }
  }

  private teleport(): void {
    if (!this.guard('teleport', 50)) return;
    this.noise(0.38, 0.08, 220, 'bandpass', 0, 2200, 1.2);
    this.tone(320, 0.3, 'sine', 0.04, 0, 900);
  }

  private skip(): void {
    if (!this.guard('skip', 30)) return;
    this.tone(330, 0.05, 'sine', 0.045);
  }

  private guard(key: string, ms: number): boolean {
    const now = performance.now();
    const last = this.last.get(key) ?? 0;
    if (now - last < ms) return false;
    this.last.set(key, now);
    return true;
  }

  private tone(
    freq: number,
    dur: number,
    type: OscillatorType,
    gain: number,
    delay = 0,
    slideTo?: number,
  ): void {
    const ctx = this.ctx;
    const master = this.master;
    if (!ctx || !master) return;
    const t = ctx.currentTime + delay;
    const osc = ctx.createOscillator();
    const g = ctx.createGain();
    osc.type = type;
    osc.frequency.setValueAtTime(Math.max(20, freq), t);
    if (slideTo !== undefined) {
      osc.frequency.exponentialRampToValueAtTime(Math.max(20, slideTo), t + dur);
    }
    g.gain.setValueAtTime(0.0001, t);
    g.gain.exponentialRampToValueAtTime(Math.max(0.0002, gain), t + Math.min(0.02, dur / 3));
    g.gain.exponentialRampToValueAtTime(0.0001, t + dur);
    osc.connect(g).connect(master);
    osc.start(t);
    osc.stop(t + dur + 0.02);
  }

  private noise(
    dur: number,
    gain: number,
    freq: number,
    type: BiquadFilterType = 'bandpass',
    delay = 0,
    sweepTo?: number,
    q = 1,
  ): void {
    const ctx = this.ctx;
    const master = this.master;
    if (!ctx || !master || !this.noiseBuf) return;
    const t = ctx.currentTime + delay;
    const src = ctx.createBufferSource();
    src.buffer = this.noiseBuf;
    src.loop = true;
    const filter = ctx.createBiquadFilter();
    filter.type = type;
    filter.frequency.setValueAtTime(Math.max(40, freq), t);
    filter.Q.value = q;
    if (sweepTo !== undefined) {
      filter.frequency.exponentialRampToValueAtTime(Math.max(40, sweepTo), t + dur);
    }
    const g = ctx.createGain();
    g.gain.setValueAtTime(0.0001, t);
    g.gain.exponentialRampToValueAtTime(Math.max(0.0002, gain), t + Math.min(0.02, dur / 3));
    g.gain.exponentialRampToValueAtTime(0.0001, t + dur);
    src.connect(filter).connect(g).connect(master);
    src.start(t);
    src.stop(t + dur + 0.02);
  }

  private makeNoise(): AudioBuffer {
    const ctx = this.ctx as AudioContext;
    const buf = ctx.createBuffer(1, Math.floor(ctx.sampleRate * 0.6), ctx.sampleRate);
    const data = buf.getChannelData(0);
    for (let i = 0; i < data.length; i++) data[i] = Math.random() * 2 - 1;
    return buf;
  }
}
