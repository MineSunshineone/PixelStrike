// High-definition game audio engine with WebAudio fallback synthesis.

const SFX_NAMES = [
  'fire_glock', 'fire_deagle', 'fire_mp5', 'fire_ak47', 'fire_m4a4', 'fire_awp',
  'fire_usp', 'fire_ump', 'fire_famas', 'fire_aug', 'fire_scout', 'fire_xm',
  'headshot_ding', 'hitmarker', 'death', 'hurt', 'grenade_explode',
  'kill_confirm', 'headshot_kill',
  'step', 'reload_click', 'bolt_rack',
  'mag_out', 'mag_in', 'bolt_cycle', 'empty_click',
  'knife_slash', 'knife_hit', 'weapon_switch',
] as const;

export type SfxName = (typeof SFX_NAMES)[number];

export class AudioEngine {
  private ctx: AudioContext | null = null;
  private buffers = new Map<string, AudioBuffer>();
  private loading = new Set<string>();
  private lastPlayed = new Map<string, number>();
  private masterGain: GainNode | null = null;
  public volume = 0.8;

  init() {
    if (this.ctx) return;
    this.ctx = new AudioContext();
    this.masterGain = this.ctx.createGain();
    this.masterGain.gain.value = this.volume;
    const limiter = this.ctx.createDynamicsCompressor();
    limiter.threshold.value = -2;
    limiter.knee.value = 0;
    limiter.ratio.value = 20;
    limiter.attack.value = 0.001;
    limiter.release.value = 0.08;
    this.masterGain.connect(limiter).connect(this.ctx.destination);
    SFX_NAMES.forEach((name, index) => window.setTimeout(() => void this.load(name), 500 + index * 75));
  }

  private async load(name: SfxName) {
    if (!this.ctx || this.buffers.has(name) || this.loading.has(name)) return;
    this.loading.add(name);
    try {
      const res = await fetch(`/sfx/${name}.wav`);
      if (res.ok) this.buffers.set(name, await this.ctx.decodeAudioData(await res.arrayBuffer()));
    } catch { /* synthetic sound already played */ }
    finally { this.loading.delete(name); }
  }

  setVolume(vol: number) {
    this.volume = Math.max(0, Math.min(1, vol));
    if (this.masterGain && this.ctx) {
      this.masterGain.gain.setValueAtTime(this.volume, this.ctx.currentTime);
    }
  }

  play(name: SfxName, volume = 1, pitchRate = 1.0, pan = 0, priority = false) {
    if (!this.ctx || !this.masterGain) return;
    if (this.ctx.state === 'suspended') this.ctx.resume();

    const now = performance.now();
    const last = this.lastPlayed.get(name) ?? 0;
    if (!priority && now - last < 35) return; // spam deduplication
    this.lastPlayed.set(name, now);

    const buf = this.buffers.get(name);
    if (buf) {
      const src = this.ctx.createBufferSource();
      src.buffer = buf;
      src.playbackRate.value = pitchRate;
      const g = this.ctx.createGain();
      const p = this.ctx.createStereoPanner();
      g.gain.value = volume;
      p.pan.value = Math.max(-1, Math.min(1, pan));
      src.connect(g).connect(p).connect(this.masterGain);
      src.start();
      return;
    }
    void this.load(name);
    this.synth(name, volume, pan);
  }

  private synth(name: SfxName, vol: number, pan: number) {
    const ctx = this.ctx!;
    const t0 = ctx.currentTime;
    const out = ctx.createGain();
    const panner = ctx.createStereoPanner();
    out.gain.value = vol * 0.5;
    panner.pan.value = Math.max(-1, Math.min(1, pan));
    out.connect(panner).connect(this.masterGain!);

    const osc = (type: OscillatorType, f0: number, f1: number, dur: number) => {
      const o = ctx.createOscillator();
      o.type = type;
      o.frequency.setValueAtTime(f0, t0);
      o.frequency.exponentialRampToValueAtTime(Math.max(20, f1), t0 + dur);
      o.connect(out);
      o.start(t0);
      o.stop(t0 + dur);
    };

    const noise = (dur: number, cutoff: number) => {
      const n = Math.floor(ctx.sampleRate * dur);
      const b = ctx.createBuffer(1, n, ctx.sampleRate);
      const d = b.getChannelData(0);
      for (let i = 0; i < n; i++) d[i] = (Math.random() * 2 - 1) * Math.exp(-i / n * 8);
      const src = ctx.createBufferSource();
      src.buffer = b;
      const f = ctx.createBiquadFilter();
      f.type = 'lowpass';
      f.frequency.value = cutoff;
      src.connect(f).connect(out);
      src.start(t0);
    };

    switch (name) {
      case 'fire_glock':
        noise(0.18, 3800);
        osc('sine', 160, 45, 0.12);
        break;
      case 'fire_deagle':
        noise(0.32, 2800);
        osc('sine', 120, 30, 0.28);
        osc('triangle', 320, 60, 0.15);
        break;
      case 'fire_mp5':
        noise(0.14, 4500);
        osc('sine', 180, 70, 0.08);
        break;
      case 'fire_ak47':
        noise(0.28, 2600);
        osc('sawtooth', 220, 45, 0.22);
        osc('sine', 95, 30, 0.25);
        break;
      case 'fire_m4a4':
        noise(0.24, 3200);
        osc('triangle', 260, 55, 0.18);
        break;
      case 'fire_awp':
        noise(0.65, 1600);
        osc('sine', 75, 25, 0.6);
        osc('sawtooth', 140, 35, 0.35);
        break;
      case 'fire_usp':
        noise(0.16, 3400);
        osc('sine', 170, 50, 0.1);
        break;
      case 'fire_ump':
        noise(0.2, 2400);
        osc('triangle', 150, 55, 0.12);
        break;
      case 'fire_famas':
        noise(0.22, 3000);
        osc('triangle', 240, 50, 0.16);
        break;
      case 'fire_aug':
        noise(0.23, 2800);
        osc('sine', 200, 48, 0.17);
        break;
      case 'fire_scout':
        noise(0.45, 1800);
        osc('sine', 90, 28, 0.4);
        osc('triangle', 160, 40, 0.22);
        break;
      case 'fire_xm':
        noise(0.4, 1400);
        osc('sine', 70, 22, 0.32);
        osc('sawtooth', 110, 30, 0.2);
        break;
      case 'headshot_ding':
        // CS:GO iconic metallic DINK (twin crystal sine ringing)
        osc('sine', 2800, 2600, 0.35);
        osc('sine', 4200, 3900, 0.25);
        break;
      case 'grenade_explode':
        noise(0.85, 1200);
        osc('sine', 65, 20, 0.8);
        osc('triangle', 110, 30, 0.45);
        break;
      case 'hitmarker':
        osc('sine', 2400, 2400, 0.05);
        break;
      case 'kill_confirm':
        osc('sine', 135, 48, 0.32);
        osc('square', 420, 900, 0.17);
        noise(0.12, 3600);
        break;
      case 'headshot_kill':
        osc('sine', 2600, 2350, 0.28);
        osc('sine', 3900, 3550, 0.2);
        osc('sine', 120, 42, 0.3);
        noise(0.16, 4200);
        break;
      case 'death':
        osc('sawtooth', 280, 40, 0.6);
        break;
      case 'hurt':
        osc('sawtooth', 200, 70, 0.22);
        break;
      case 'step':
        osc('sine', 75, 75, 0.04);
        break;
      case 'reload_click':
        noise(0.06, 6000);
        osc('triangle', 600, 300, 0.05);
        break;
      case 'bolt_rack':
        osc('sawtooth', 350, 180, 0.12);
        noise(0.08, 4000);
        break;
      case 'mag_out':
        noise(0.07, 3200);
        osc('square', 750, 240, 0.06);
        break;
      case 'mag_in':
        noise(0.09, 2200);
        osc('sine', 160, 45, 0.09);
        osc('sawtooth', 880, 280, 0.04);
        break;
      case 'bolt_cycle':
        osc('sawtooth', 420, 160, 0.09);
        noise(0.07, 4500);
        osc('triangle', 720, 220, 0.06);
        break;
      case 'empty_click':
        osc('triangle', 1800, 400, 0.03);
        noise(0.02, 7000);
        break;
      case 'knife_slash':
        noise(0.08, 4200);
        osc('sine', 1100, 240, 0.08);
        break;
      case 'knife_hit':
        osc('sine', 130, 40, 0.12);
        osc('sawtooth', 1400, 300, 0.05);
        noise(0.06, 2800);
        break;
      case 'weapon_switch':
        noise(0.04, 5200);
        osc('sawtooth', 720, 220, 0.05);
        break;
    }
  }
}
