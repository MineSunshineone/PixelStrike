import { WEAPONS, type MapData, type PlayerSnap, type RosterEntry } from './constants.js';
import { CharacterPreview } from './preview.js';

function el(id: string): HTMLElement {
  return document.getElementById(id)!;
}

function esc(s: string): string {
  return s.replace(/[&<>"']/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c] ?? c));
}


const WEAPON_BADGES: Record<number, string> = {
  0: 'GLOCK-18',
  1: 'DEAGLE',
  2: 'MP5-SD',
  3: 'AK-47',
  4: 'M4A4',
  5: 'AWP',
  6: 'KNIFE',
  7: 'USP-S',
  8: 'UMP-45',
  9: 'FAMAS',
  10: 'AUG',
  11: 'SSG 08',
  12: 'XM1014',
  13: 'HE',
};

const BLOCK_RADAR_COLORS: Record<number, string> = {
  0: '#385e2b',
  1: '#5c5e58',
  2: '#7d5830',
  3: '#8c949c',
  4: '#4a4d46',
  5: '#2d5422',
  6: '#a89d6e',
  7: '#6e3028',
  8: '#d49b28',
  9: '#181226',
  10: '#754b28',
  11: '#7ab3cf',
  12: '#2a5ebd',
  13: '#6e4f35',
};

const WEAPON_SVGS: Record<number, string> = {
  0: `<svg viewBox="0 0 32 32"><rect x="6" y="11" width="16" height="5" fill="#1b1b1e"/><rect x="8" y="16" width="12" height="3" fill="#2c3038"/><rect x="8" y="19" width="5" height="7" fill="#1b1b1e" transform="rotate(20 8 19)"/><circle cx="20" cy="12" r="1.5" fill="#39ff14"/></svg>`,
  1: `<svg viewBox="0 0 32 32"><rect x="5" y="10" width="18" height="6" fill="#d2d6dc"/><rect x="7" y="16" width="14" height="3" fill="#9fa3ab"/><rect x="7" y="19" width="6" height="8" fill="#1b1b1e" transform="rotate(20 7 19)"/><rect x="10" y="21" width="2" height="2" fill="#d4af37"/></svg>`,
  2: `<svg viewBox="0 0 32 32"><rect x="3" y="15" width="6" height="4" fill="#22252a"/><rect x="9" y="13" width="9" height="6" fill="#22252a"/><rect x="18" y="12" width="11" height="7" rx="3" fill="#223547"/><rect x="14" y="19" width="3" height="8" fill="#22252a" transform="rotate(20 14 19)"/><rect x="6" y="18" width="3" height="6" fill="#22252a" transform="rotate(25 6 18)"/></svg>`,
  3: `<svg viewBox="0 0 32 32"><rect x="3" y="15" width="7" height="6" fill="#7a3a1a"/><rect x="10" y="13" width="10" height="6" fill="#22252a"/><rect x="20" y="14" width="9" height="3" fill="#555d68"/><rect x="16" y="15" width="4" height="5" fill="#7a3a1a"/><rect x="14" y="19" width="3" height="8" fill="#22252a" transform="rotate(20 14 19)"/><rect x="5" y="19" width="3" height="6" fill="#7a3a1a" transform="rotate(25 5 19)"/></svg>`,
  4: `<svg viewBox="0 0 32 32"><rect x="3" y="14" width="7" height="6" fill="#22252a"/><rect x="10" y="13" width="10" height="6" fill="#333842"/><rect x="20" y="13" width="7" height="6" fill="#555d68"/><rect x="27" y="14" width="4" height="3" fill="#88929e"/><rect x="14" y="19" width="3" height="8" fill="#22252a" transform="rotate(25 7 19)"/><rect x="7" y="19" width="3" height="6" fill="#22252a" transform="rotate(25 7 19)"/></svg>`,
  5: `<svg viewBox="0 0 32 32"><rect x="2" y="15" width="8" height="6" fill="#344d37"/><rect x="10" y="14" width="12" height="6" fill="#344d37"/><rect x="22" y="15" width="8" height="3" fill="#22252a"/><rect x="29" y="14" width="2" height="5" fill="#555d68"/><rect x="11" y="9" width="11" height="4" rx="1" fill="#1b1b1e"/><rect x="13" y="13" width="2" height="2" fill="#555d68"/><rect x="19" y="13" width="2" height="2" fill="#555d68"/><rect x="14" y="20" width="4" height="5" fill="#22252a"/></svg>`,
  6: `<svg viewBox="0 0 32 32"><path d="M6 26 L12 20 L15 21 L9 27 Z" fill="#22252a"/><path d="M11 19 L14 17 L16 19 L13 21 Z" fill="#555d68"/><path d="M14 17 L25 5 L27 7 L16 19 Z" fill="#d2d6dc"/><path d="M23 7 L25 5 L27 7 L25 9 Z" fill="#ffffff"/></svg>`,
  7: `<svg viewBox="0 0 32 32"><rect x="7" y="12" width="15" height="5" fill="#2a3340"/><rect x="20" y="13" width="6" height="3" fill="#223547"/><rect x="9" y="17" width="5" height="7" fill="#1b1b1e" transform="rotate(18 9 17)"/></svg>`,
  8: `<svg viewBox="0 0 32 32"><rect x="4" y="13" width="18" height="7" fill="#4a4036"/><rect x="20" y="14" width="8" height="5" fill="#22252a"/><rect x="10" y="20" width="4" height="7" fill="#22252a" transform="rotate(20 10 20)"/><rect x="12" y="18" width="5" height="8" fill="#1b1b1e"/></svg>`,
  9: `<svg viewBox="0 0 32 32"><rect x="3" y="12" width="20" height="8" fill="#5a6848"/><rect x="22" y="14" width="7" height="4" fill="#555d68"/><rect x="8" y="20" width="3" height="7" fill="#22252a" transform="rotate(18 8 20)"/></svg>`,
  10: `<svg viewBox="0 0 32 32"><rect x="4" y="13" width="22" height="7" fill="#3d4a3a"/><rect x="14" y="8" width="8" height="5" rx="1" fill="#1b1b1e"/><rect x="24" y="14" width="5" height="4" fill="#555d68"/></svg>`,
  11: `<svg viewBox="0 0 32 32"><rect x="2" y="15" width="24" height="4" fill="#2f3a48"/><rect x="12" y="9" width="10" height="4" rx="1" fill="#1b1b1e"/><rect x="26" y="14" width="4" height="3" fill="#555d68"/></svg>`,
  12: `<svg viewBox="0 0 32 32"><rect x="3" y="13" width="20" height="7" fill="#6a5a3a"/><rect x="22" y="14" width="7" height="5" fill="#22252a"/><rect x="9" y="20" width="4" height="7" fill="#22252a" transform="rotate(18 9 20)"/></svg>`,
  13: `<svg viewBox="0 0 32 32"><ellipse cx="16" cy="18" rx="7" ry="9" fill="#475e38"/><rect x="11" y="17" width="10" height="2" fill="#2e3f24"/><rect x="15" y="11" width="2" height="14" fill="#2e3f24"/><rect x="14" y="7" width="4" height="4" fill="#9fa3ab"/><circle cx="11" cy="8" r="2.5" fill="none" stroke="#d2d6dc" stroke-width="1.5"/></svg>`,
};
export class Hud {
  root = el('hud');
  sensitivity = 0.00216;
  adsSensitivity = 0.85;
  touchSensitivity = 0.0035;
  volume = 0.8;
  hipFov = 62;
  bobScale = 0.55;
  quality: 'low' | 'medium' | 'high' = 'medium';
  crosshairStyle: string = 'cross-dot';
  crosshairColor: string = '';
  crosshairSize = 7;
  crosshairThickness = 2;
  crosshairGap = 3;
  crosshairDot = true;
  crosshairOutline = true;
  crosshairDynamic = true;
  private loadoutPrimary = -1;
  private loadoutSecondary = -1;
  characterPreview: CharacterPreview | null = null;
  onJoin: ((name: string, primary: number, secondary: number, skin: number, primaryWeaponSkin: number, secondaryWeaponSkin: number) => void) | null = null;
  onPractice: (() => void) | null = null;
  onVolumeChange: ((v: number) => void) | null = null;
  onFovChange: ((fov: number) => void) | null = null;
  onQualityChange: ((q: 'low' | 'medium' | 'high') => void) | null = null;
  onLoadoutChange: ((primary: number, secondary: number) => void) | null = null;
  onFlightToggle: (() => void) | null = null;
  onExit: (() => void) | null = null;
  onSettingsClose: (() => void) | null = null;
  onTouchLayoutEdit: (() => void) | null = null;
  private menu = el('menu');
  private scoreboard = el('scoreboard');
  private settings = el('settings-modal');
  private pause = el('pause-overlay');
  private disconnect = el('disconnect-overlay');
  private killfeed = el('killfeed');
  private hit = el('hitmarker');
  private streak = el('kill-streak');
  private killMedal = el('kill-medal');
  private crosshair = el('crosshair');
  private damage = el('damage-flash');
  private scope = el('sniper-scope');
  private deathCountdown = el('death-countdown');
  private radar = el('radar') as HTMLCanvasElement;
  private radarBase = document.createElement('canvas');

  private map: MapData | null = null;
  private lastRadar = 0;
  private hitTimer = 0;
  private damageTimer = 0;
  private streakTimer = 0;
  private killMedalTimer = 0;
  private lastHp = -1;
  private lastArmor = -1;
  private lastInventory = '';
  private lastPrimary = -1;
  private lastSecondary = -1;
  private lastNetwork = '';
  private lastShield = false;
  private lastScope: boolean | null = null;
  private lastCrosshair = -1;
  private lastDynamic = true;
  private lastCrosshairSpread = 0;
  private lastDeathCountdown = -2;
  private lastReloading: boolean | null = null;
  private lastReloadPct = -1;
  private lastAmmo = '';
  private hurtTimer = 0;
  private toastTimer = 0;
  private announcementTimer = 0;
  private reconnectTimer = 0;
  private refreshWeaponProgress: (() => void) | null = null;

  constructor() {
    const primary = el('primary-select') as HTMLSelectElement;
    const secondary = el('secondary-select') as HTMLSelectElement;
    const primaryWeaponSkin = el('primary-weapon-skin') as HTMLSelectElement;
    const secondaryWeaponSkin = el('secondary-weapon-skin') as HTMLSelectElement;
    const weaponKills = new Map<number, number>();
    const refreshSkinOptions = (weaponSelect: HTMLSelectElement, skinSelect: HTMLSelectElement) => {
      const weapon = +weaponSelect.value;
      const kills = weaponKills.get(weapon) ?? 0;
      for (const option of [...skinSelect.options]) {
        if (option.value === '1') option.disabled = weapon >= 0 && kills < 100;
        if (option.value === '2') option.disabled = weapon >= 0 && kills < 500;
      }
      if (skinSelect.selectedOptions[0]?.disabled) skinSelect.value = '3';
    };
    const loadWeaponProgress = () => {
      void fetch('/api/progression', { cache: 'no-store' }).then((response) => response.ok ? response.json() : Promise.reject()).then((data: { weapons?: { weapon: number; kills: number }[] }) => {
        weaponKills.clear();
        for (const row of data.weapons ?? []) weaponKills.set(row.weapon, row.kills);
        const selected = +primary.value;
        el('weapon-skin-progress').textContent = selected < 0 ? '随机武器会先抽取枪械，再从该枪已解锁的皮肤中随机。' : `${WEAPONS[selected].name}：${weaponKills.get(selected) ?? 0} 杀敌 · 黄金 100 / 钻石 500`;
        refreshSkinOptions(primary, primaryWeaponSkin);
        refreshSkinOptions(secondary, secondaryWeaponSkin);
      }).catch(() => { el('weapon-skin-progress').textContent = '进度暂时无法读取，服务器仍会校验已解锁皮肤。'; });
    };
    this.refreshWeaponProgress = loadWeaponProgress;
    loadWeaponProgress();
    // Restore the last deployment choices.
    const savedPrimary = localStorage.getItem('pixel_strike_primary');
    if (savedPrimary && primary) {
      primary.value = savedPrimary;
    } else if (primary) {
      primary.value = '-1';
    }
    const savedSecondary = localStorage.getItem('pixel_strike_secondary');
    if (savedSecondary && secondary) {
      secondary.value = savedSecondary;
    } else if (secondary) {
      secondary.value = '-1';
    }
    primaryWeaponSkin.value = localStorage.getItem('pixel_strike_primary_skin') ?? '3';
    secondaryWeaponSkin.value = localStorage.getItem('pixel_strike_secondary_skin') ?? '3';

    const updateWeaponSpecs = (id: number) => {
      const tag = el('weapon-spec-tag');
      const dmgVal = el('spec-dmg-val');
      const rpmVal = el('spec-rpm-val');
      const headVal = el('spec-head-val');
      const magVal = el('spec-mag-val');
      if (!tag || !dmgVal || !rpmVal || !headVal || !magVal) return;
      if (id === -1) {
        tag.textContent = '随机武器 / 每次重生随机生成';
        dmgVal.textContent = 'RANDOM';
        rpmVal.textContent = 'RANDOM';
        headVal.textContent = 'RANDOM';
        magVal.textContent = 'RANDOM';
        return;
      }
      const weapon = WEAPONS[id];
      if (!weapon) return;
      const kind = id === 5 || id === 11 ? '狙击步枪' : id === 2 || id === 8 ? '冲锋枪' : id === 12 ? '霰弹枪' : id === 10 ? '光学步枪' : '突击步枪';
      tag.textContent = `${weapon.name.toUpperCase()} / ${kind}`;
      dmgVal.textContent = String(weapon.dmg);
      rpmVal.textContent = `${weapon.rpm} RPM`;
      headVal.textContent = `${weapon.headMult.toFixed(1)} ×`;
      magVal.textContent = `${weapon.mag} / ${weapon.reserve}`;
    };

    this.loadoutPrimary = +primary.value;
    this.loadoutSecondary = +secondary.value;
    if (primary) {
      updateWeaponSpecs(this.loadoutPrimary);
      primary.addEventListener('change', () => {
        localStorage.setItem('pixel_strike_primary', primary.value);
        this.loadoutPrimary = +primary.value;
        updateWeaponSpecs(this.loadoutPrimary);
        this.onLoadoutChange?.(this.loadoutPrimary, this.loadoutSecondary);
        refreshSkinOptions(primary, primaryWeaponSkin);
        const progress = el('weapon-skin-progress');
        progress.textContent = this.loadoutPrimary < 0 ? '随机武器会先抽取枪械，再从该枪已解锁的皮肤中随机。' : `${WEAPONS[this.loadoutPrimary].name}：${weaponKills.get(this.loadoutPrimary) ?? 0} 杀敌 · 黄金 100 / 钻石 500`;
      });
    }
    secondary?.addEventListener('change', () => {
      localStorage.setItem('pixel_strike_secondary', secondary.value);
      this.loadoutSecondary = +secondary.value;
      this.onLoadoutChange?.(this.loadoutPrimary, this.loadoutSecondary);
      refreshSkinOptions(secondary, secondaryWeaponSkin);
      if (+primary.value === -1 && +secondary.value !== -1) {
        this.characterPreview?.setWeapon(+secondary.value);
      }
    });
    primaryWeaponSkin.addEventListener('change', () => localStorage.setItem('pixel_strike_primary_skin', primaryWeaponSkin.value));
    secondaryWeaponSkin.addEventListener('change', () => localStorage.setItem('pixel_strike_secondary_skin', secondaryWeaponSkin.value));

    const previewCanvas = el('character-preview-canvas') as HTMLCanvasElement;
    if (previewCanvas) {
      this.characterPreview = new CharacterPreview(previewCanvas);
      const initialWeapon = this.loadoutPrimary === -1 ? 3 : this.loadoutPrimary;
      this.characterPreview.setWeapon(initialWeapon);

      primary?.addEventListener('change', () => {
        const weaponId = +primary.value === -1 ? 3 : +primary.value;
        this.characterPreview?.setWeapon(weaponId);
      });
      el('random-skin-btn')?.addEventListener('click', () => {
        this.characterPreview?.randomizeSkin();
      });
    }
    const deployOverlay = el('deploy-loader-overlay');
    const progressBar = el('deploy-progress-bar');
    const statusText = el('deploy-status-text');
    const pctText = el('deploy-pct-text');

    let deploying = false;
    const startDeploy = () => {
      if (deploying) return;
      const enteredName = window.prompt('请输入玩家名字（最多 16 个字符）', localStorage.getItem('pixel_strike_name') ?? '');
      if (enteredName === null) return;
      const n = [...enteredName.trim()].slice(0, 16).join('');
      if (!n) {
        window.alert('玩家名字不能为空');
        return;
      }
      localStorage.setItem('pixel_strike_name', n);
      this.loadoutPrimary = +primary.value;
      this.loadoutSecondary = +secondary.value;

      if (deployOverlay && progressBar && statusText && pctText) {
        deploying = true;
        deployOverlay.style.display = 'flex';
        let progress = 0;
        progressBar.style.width = '0%';
        pctText.textContent = '0%';
        statusText.textContent = '[1/4] 连接 60Hz 权威物理服务器...';

        const steps = [
          { at: 25, text: '[1/4] 连接 60Hz 权威物理服务器...' },
          { at: 55, text: '[2/4] 加载 512x512 非对称要塞地图...' },
          { at: 82, text: '[3/4] 装配 3D 战术武器与皮肤...' },
          { at: 100, text: '[4/4] 部署完成 · 准备交火!' },
        ];
        let stepIdx = 0;

        const timer = setInterval(() => {
          progress += Math.floor(7 + Math.random() * 12);
          if (progress > 100) progress = 100;
          progressBar.style.width = `${progress}%`;
          pctText.textContent = `${progress}%`;

          while (stepIdx < steps.length && progress >= steps[stepIdx].at) {
            statusText.textContent = steps[stepIdx].text;
            stepIdx++;
          }

          if (progress >= 100) {
            clearInterval(timer);
            setTimeout(() => {
              deployOverlay.style.display = 'none';
              this.enterGameUI();
              deploying = false;
              this.onJoin?.(n, this.loadoutPrimary, this.loadoutSecondary, this.characterPreview?.getSkin() ?? 0, +primaryWeaponSkin.value, +secondaryWeaponSkin.value);
            }, 140);
          }
        }, 32);
      } else {
        this.enterGameUI();
        this.onJoin?.(n, this.loadoutPrimary, this.loadoutSecondary, this.characterPreview?.getSkin() ?? 0, +primaryWeaponSkin.value, +secondaryWeaponSkin.value);
      }
    };

    el('join-btn').addEventListener('click', startDeploy);

    el('lb-refresh-btn')?.addEventListener('click', () => this.loadLeaderboard());
    el('sb-close-btn')?.addEventListener('click', () => this.toggleScoreboard(false));

    this.setupSettings();
    this.loadLeaderboard();
  }

  private setupSettings() {
    const sens = el('sens-slider') as HTMLInputElement;
    const adsSens = el('ads-sens-slider') as HTMLInputElement;
    const vol = el('vol-slider') as HTMLInputElement;
    const quality = el('quality-select') as HTMLSelectElement;
    const fov = el('fov-slider') as HTMLInputElement;
    const bob = el('bob-slider') as HTMLInputElement;
    const touchSens = el('touch-sens-slider') as HTMLInputElement;

    const savedSens = localStorage.getItem('ps_sens');
    const savedAdsSens = localStorage.getItem('ps_ads_sens');
    const savedVol = localStorage.getItem('ps_vol');
    const savedQ = localStorage.getItem('ps_quality') as typeof this.quality | null;
    // v2 gives existing players the tighter default FOV once.
    const savedFov = localStorage.getItem('ps_hip_fov_v2');
    const savedBob = localStorage.getItem('ps_gun_bob');
    const savedTouchSens = localStorage.getItem('ps_touch_sens');

    if (savedSens && sens) sens.value = savedSens;
    if (savedAdsSens && adsSens) adsSens.value = savedAdsSens;
    if (savedVol && vol) vol.value = savedVol;
    if (savedQ && quality) quality.value = savedQ;
    if (savedFov && fov) fov.value = savedFov;
    if (savedBob && bob) bob.value = savedBob;
    if (savedTouchSens && touchSens) touchSens.value = savedTouchSens;

    this.sensitivity = sens ? (+sens.value / 50) * 0.0024 : 0.00216;
    this.adsSensitivity = adsSens ? +adsSens.value / 100 : 0.85;
    this.touchSensitivity = touchSens ? (+touchSens.value / 50) * 0.0035 : 0.0035;
    this.volume = vol ? +vol.value / 100 : 0.8;
    this.quality = quality ? (quality.value as typeof this.quality) : 'medium';
    this.hipFov = fov ? +fov.value : 62;
    this.bobScale = bob ? +bob.value / 100 : 0.55;

    sens?.addEventListener('input', () => {
      this.sensitivity = (+sens.value / 50) * 0.0024;
      localStorage.setItem('ps_sens', sens.value);
    });

    adsSens?.addEventListener('input', () => {
      this.adsSensitivity = +adsSens.value / 100;
      localStorage.setItem('ps_ads_sens', adsSens.value);
    });
    touchSens?.addEventListener('input', () => {
      this.touchSensitivity = (+touchSens.value / 50) * 0.0035;
      localStorage.setItem('ps_touch_sens', touchSens.value);
    });


    vol?.addEventListener('input', () => {
      this.volume = +vol.value / 100;
      localStorage.setItem('ps_vol', vol.value);
      this.onVolumeChange?.(this.volume);
    });

    quality?.addEventListener('change', () => {
      this.quality = quality.value as typeof this.quality;
      localStorage.setItem('ps_quality', this.quality);
      this.onQualityChange?.(this.quality);
    });

    fov?.addEventListener('input', () => {
      this.hipFov = +fov.value;
      localStorage.setItem('ps_hip_fov_v2', fov.value);
      this.onFovChange?.(this.hipFov);
    });

    bob?.addEventListener('input', () => {
      this.bobScale = +bob.value / 100;
      localStorage.setItem('ps_gun_bob', bob.value);
    });

    const chStyle = el('ch-style-select') as HTMLSelectElement;
    const chColorInput = el('ch-color-input') as HTMLInputElement;
    const chSize = el('ch-size-slider') as HTMLInputElement;
    const chThick = el('ch-thick-slider') as HTMLInputElement;
    const chGap = el('ch-gap-slider') as HTMLInputElement;
    const chDot = el('ch-dot-toggle') as HTMLInputElement;
    const chOutline = el('ch-outline-toggle') as HTMLInputElement;
    const chDynamic = el('ch-dynamic-toggle') as HTMLInputElement;
    const chReset = el('ch-reset-btn') as HTMLButtonElement;

    this.crosshairStyle = localStorage.getItem('ps_ch_style') ?? 'cross-dot';
    this.crosshairColor = localStorage.getItem('ps_ch_color') ?? '';
    this.crosshairSize = +(localStorage.getItem('ps_ch_size') ?? '7');
    this.crosshairThickness = +(localStorage.getItem('ps_ch_thick') ?? '2');
    this.crosshairGap = +(localStorage.getItem('ps_ch_gap') ?? '3');
    this.crosshairDot = (localStorage.getItem('ps_ch_dot') ?? '1') !== '0';
    this.crosshairOutline = (localStorage.getItem('ps_ch_outline') ?? '1') !== '0';
    this.crosshairDynamic = (localStorage.getItem('ps_ch_dynamic') ?? '1') !== '0';

    if (chStyle) chStyle.value = this.crosshairStyle;
    if (chColorInput) chColorInput.value = this.crosshairColor || '#f4f1e4';
    if (chSize) chSize.value = String(this.crosshairSize);
    if (chThick) chThick.value = String(this.crosshairThickness);
    if (chGap) chGap.value = String(this.crosshairGap);
    if (chDot) chDot.checked = this.crosshairDot;
    if (chOutline) chOutline.checked = this.crosshairOutline;
    if (chDynamic) chDynamic.checked = this.crosshairDynamic;
    for (const s of document.querySelectorAll<HTMLButtonElement>('.ch-swatch')) {
      s.classList.toggle('active', s.dataset.color === this.crosshairColor);
    }

    const saveCrosshair = () => {
      localStorage.setItem('ps_ch_style', this.crosshairStyle);
      localStorage.setItem('ps_ch_size', String(this.crosshairSize));
      localStorage.setItem('ps_ch_thick', String(this.crosshairThickness));
      localStorage.setItem('ps_ch_gap', String(this.crosshairGap));
      localStorage.setItem('ps_ch_dot', this.crosshairDot ? '1' : '0');
      localStorage.setItem('ps_ch_outline', this.crosshairOutline ? '1' : '0');
      localStorage.setItem('ps_ch_dynamic', this.crosshairDynamic ? '1' : '0');
    };

    chStyle?.addEventListener('change', () => {
      this.crosshairStyle = chStyle.value;
      saveCrosshair();
      this.applyCrosshair();
      this.updateCrosshairPreview();
    });
    chSize?.addEventListener('input', () => {
      this.crosshairSize = +chSize.value;
      saveCrosshair();
      this.applyCrosshair();
      this.updateCrosshairPreview();
    });
    chThick?.addEventListener('input', () => {
      this.crosshairThickness = +chThick.value;
      saveCrosshair();
      this.applyCrosshair();
      this.updateCrosshairPreview();
    });
    chGap?.addEventListener('input', () => {
      this.crosshairGap = +chGap.value;
      saveCrosshair();
      this.applyCrosshair();
      this.updateCrosshairPreview();
    });
    chDot?.addEventListener('change', () => {
      this.crosshairDot = chDot.checked;
      saveCrosshair();
      this.applyCrosshair();
      this.updateCrosshairPreview();
    });
    chOutline?.addEventListener('change', () => {
      this.crosshairOutline = chOutline.checked;
      saveCrosshair();
      this.applyCrosshair();
      this.updateCrosshairPreview();
    });
    chDynamic?.addEventListener('change', () => {
      this.crosshairDynamic = chDynamic.checked;
      saveCrosshair();
      this.applyCrosshair();
      this.updateCrosshairPreview();
    });
    for (const s of document.querySelectorAll<HTMLButtonElement>('.ch-swatch')) {
      s.addEventListener('click', () => this.setCrosshairColor(s.dataset.color ?? ''));
    }
    chColorInput?.addEventListener('input', () => this.setCrosshairColor(chColorInput.value));
    chReset?.addEventListener('click', () => this.resetCrosshair());

    this.applyCrosshair();
    this.updateCrosshairPreview();

    el('open-settings-btn')?.addEventListener('click', () => this.toggleSettings(true));
    el('close-settings-btn')?.addEventListener('click', () => this.toggleSettings(false));
    el('exit-btn')?.addEventListener('click', () => this.onExit?.());
    el('practice-btn')?.addEventListener('click', () => this.startPractice());
    el('practice-join-btn')?.addEventListener('click', () => this.startPractice());
    el('pause-resume-btn')?.addEventListener('click', () => this.showPause(false));
    el('pause-flight-btn')?.addEventListener('click', () => this.onFlightToggle?.());
    el('pause-exit-btn')?.addEventListener('click', () => this.onExit?.());
    el('pause-settings-btn')?.addEventListener('click', () => {
      this.showPause(false);
      this.toggleSettings(true);
    });
    el('touch-layout-edit-btn')?.addEventListener('click', () => this.onTouchLayoutEdit?.());
  }

  applyCrosshair() {
    this.crosshair.dataset.style = this.crosshairStyle;
    this.crosshair.dataset.outline = this.crosshairOutline ? 'on' : 'off';
    this.crosshair.dataset.dot = this.crosshairDot ? 'on' : 'off';
    this.crosshair.dataset.customColor = this.crosshairColor ? 'on' : 'off';
    if (this.crosshairColor) this.crosshair.style.setProperty('--crosshair-color', this.crosshairColor);
    else this.crosshair.style.removeProperty('--crosshair-color');
    this.crosshair.style.setProperty('--ch-size', `${this.crosshairSize}px`);
    this.crosshair.style.setProperty('--ch-thickness', `${this.crosshairThickness}px`);
    this.crosshair.style.setProperty('--ch-dot', `${this.crosshairThickness}px`);
    this.lastCrosshair = -1;
    this.setCrosshair(this.lastCrosshairSpread);
  }

  updateCrosshairPreview() {
    const preview = document.getElementById('crosshair-preview');
    if (!preview) return;
    preview.dataset.style = this.crosshairStyle;
    preview.dataset.outline = this.crosshairOutline ? 'on' : 'off';
    preview.dataset.dot = this.crosshairDot ? 'on' : 'off';
    preview.dataset.customColor = this.crosshairColor ? 'on' : 'off';
    if (this.crosshairColor) preview.style.setProperty('--crosshair-color', this.crosshairColor);
    else preview.style.removeProperty('--crosshair-color');
    preview.style.setProperty('--ch-size', `${this.crosshairSize}px`);
    preview.style.setProperty('--ch-thickness', `${this.crosshairThickness}px`);
    preview.style.setProperty('--ch-dot', `${this.crosshairThickness}px`);
    preview.style.setProperty('--spread', `${this.crosshairGap}px`);
  }

  private setCrosshairColor(color: string) {
    this.crosshairColor = color;
    localStorage.setItem('ps_ch_color', color);
    const input = el('ch-color-input') as HTMLInputElement | null;
    if (input) input.value = color || '#f4f1e4';
    for (const s of document.querySelectorAll<HTMLButtonElement>('.ch-swatch')) {
      s.classList.toggle('active', s.dataset.color === color);
    }
    this.applyCrosshair();
    this.updateCrosshairPreview();
  }

  resetCrosshair() {
    this.crosshairStyle = 'cross-dot';
    this.crosshairColor = '';
    this.crosshairSize = 7;
    this.crosshairThickness = 2;
    this.crosshairGap = 3;
    this.crosshairDot = true;
    this.crosshairOutline = true;
    this.crosshairDynamic = true;
    for (const key of ['style', 'color', 'size', 'thick', 'gap', 'dot', 'outline', 'dynamic']) {
      localStorage.removeItem(`ps_ch_${key}`);
    }
    const style = el('ch-style-select') as HTMLSelectElement;
    if (style) style.value = this.crosshairStyle;
    const size = el('ch-size-slider') as HTMLInputElement;
    if (size) size.value = String(this.crosshairSize);
    const thick = el('ch-thick-slider') as HTMLInputElement;
    if (thick) thick.value = String(this.crosshairThickness);
    const gap = el('ch-gap-slider') as HTMLInputElement;
    if (gap) gap.value = String(this.crosshairGap);
    const dot = el('ch-dot-toggle') as HTMLInputElement;
    if (dot) dot.checked = true;
    const outline = el('ch-outline-toggle') as HTMLInputElement;
    if (outline) outline.checked = true;
    const dyn = el('ch-dynamic-toggle') as HTMLInputElement;
    if (dyn) dyn.checked = true;
    this.setCrosshairColor('');
  }

  private enterGameUI() {
    this.menu.style.display = 'none';
    this.characterPreview?.setVisible(false);
    this.root.style.display = 'block';
  }

  startPractice() {
    this.toggleSettings(false);
    this.enterGameUI();
    this.onPractice?.();
  }

  setPractice() {
    const strip = el('match-strip');
    const middle = strip?.children[1];
    if (middle) middle.textContent = '模拟练习 · PRACTICE';
    const shield = el('shield-badge');
    if (shield) shield.style.display = 'none';
    const netStats = el('net-stats');
    if (netStats) netStats.style.display = 'none';
  }

  setMap(map: MapData) {
    this.map = map;
    this.radarBase.width = 256;
    this.radarBase.height = 256;
    const ctx = this.radarBase.getContext('2d')!;
    const ARENA_SPAN = 136; // Focus on active 128m fortress area (-68 to +68)
    const scale = this.radarBase.width / ARENA_SPAN;
    const origin = ARENA_SPAN / 2;

    // Dark high-tech tactical radar background
    ctx.fillStyle = '#0d1117';
    ctx.fillRect(0, 0, this.radarBase.width, this.radarBase.height);

    // Subtle 32m tactical radar grid
    ctx.strokeStyle = 'rgba(255, 255, 255, 0.05)';
    ctx.lineWidth = 1;
    for (let g = 0; g <= ARENA_SPAN; g += 32) {
      const pos = g * scale;
      ctx.beginPath();
      ctx.moveTo(pos, 0); ctx.lineTo(pos, this.radarBase.height);
      ctx.moveTo(0, pos); ctx.lineTo(this.radarBase.width, pos);
      ctx.stroke();
    }

    // Draw active structures and cover (ignore outer 512m floor slab)
    for (const b of map.blocks) {
      if (b.t === 0) continue; // Skip giant ground slab to avoid empty padding
      ctx.fillStyle = BLOCK_RADAR_COLORS[b.t] ?? '#444850';
      const bx = (b.x + origin) * scale;
      const bz = (b.z + origin) * scale;
      ctx.fillRect(bx, bz, Math.max(1.5, b.w * scale), Math.max(1.5, b.d * scale));
    }
    this.drawRadar(0, 0, 0, true);
  }

  updateRadar(x: number, z: number, yaw: number, now: number) {
    if (now - this.lastRadar < 70) return;
    this.lastRadar = now;
    this.drawRadar(x, z, yaw, false);
  }

  private drawRadar(x: number, z: number, yaw: number, staticOnly: boolean) {
    if (!this.map || !this.radar) return;
    const c = this.radar;
    const ctx = c.getContext('2d');
    if (!ctx) return;
    const ARENA_SPAN = 136;
    const scale = c.width / ARENA_SPAN;
    const origin = ARENA_SPAN / 2;
    ctx.drawImage(this.radarBase, 0, 0, c.width, c.height);

    if (staticOnly) return;

    const px = (x + origin) * scale;
    const pz = (z + origin) * scale;

    // Player Tactical Beacon & Heading Cone
    ctx.save();
    ctx.translate(px, pz);
    ctx.rotate(-yaw);

    // Direction cone
    ctx.fillStyle = 'rgba(16, 185, 129, 0.18)';
    ctx.beginPath();
    ctx.moveTo(0, 0);
    ctx.arc(0, 0, 18, -Math.PI * 0.65, -Math.PI * 0.35);
    ctx.closePath();
    ctx.fill();

    // Sharp tactical arrow
    ctx.fillStyle = '#10b981';
    ctx.shadowColor = '#10b981';
    ctx.shadowBlur = 6;
    ctx.beginPath();
    ctx.moveTo(0, -9);
    ctx.lineTo(6, 6);
    ctx.lineTo(0, 3);
    ctx.lineTo(-6, 6);
    ctx.closePath();
    ctx.fill();
    ctx.restore();
  }

  setHp(v: number) {
    if (v === this.lastHp) return;
    this.lastHp = v;
    const hpEl = el('hp');
    if (hpEl) hpEl.textContent = String(v);
    const fillEl = el('hp-bar-fill');
    if (fillEl) fillEl.style.width = `${Math.max(0, Math.min(100, v))}%`;
    this.root.classList.toggle('low-hp', v > 0 && v <= 30);
  }

  setAmmoDisplay(weapon: string, mag: string, reserve: string) {
    const state = `${weapon}:${mag}:${reserve}`;
    if (state === this.lastAmmo) return;
    this.lastAmmo = state;
    const nameEl = el('ammo-weapon');
    if (nameEl) nameEl.textContent = weapon;
    const magEl = el('ammo');
    if (magEl) {
      magEl.textContent = mag;
      const n = Number(mag);
      magEl.classList.toggle('empty', Number.isFinite(n) && n <= 0);
      magEl.classList.toggle('low', Number.isFinite(n) && n > 0 && n <= 5);
    }
    const reserveEl = el('ammo-reserve');
    if (reserveEl) reserveEl.textContent = reserve;
    el('ammo-divider').style.display = reserve ? '' : 'none';
  }

  showHurtDir(radians: number) {
    const arc = el('hurt-arc');
    if (!arc) return;
    const deg = ((radians * 180 / Math.PI) + 360) % 360;
    arc.style.setProperty('--hurt-dir', `${deg}deg`);
    arc.classList.add('on');
    clearTimeout(this.hurtTimer);
    this.hurtTimer = window.setTimeout(() => arc.classList.remove('on'), 280);
  }

  setArmor(v: number) {
    if (v === this.lastArmor) return;
    this.lastArmor = v;
    const armorEl = el('armor');
    if (armorEl) armorEl.textContent = String(v);
    const fillEl = el('armor-bar-fill');
    if (fillEl) fillEl.style.width = `${Math.max(0, Math.min(100, v))}%`;
  }

  setInventory(primary: number, secondary: number, active: number, mags: readonly number[], nades: number, primed: boolean) {
    const state = `${primary}:${secondary}:${active}:${mags[1]}:${mags[2]}:${nades}:${primed}`;
    if (state === this.lastInventory) return;
    this.lastInventory = state;
    const slot1Icon = el('slot-1-icon');
    if (slot1Icon && (slot1Icon.innerHTML === '' || this.lastPrimary !== primary)) {
      this.lastPrimary = primary;
      slot1Icon.innerHTML = WEAPON_SVGS[primary] ?? WEAPON_SVGS[3];
    }
    const slot2Icon = el('slot-2-icon');
    if (slot2Icon && (slot2Icon.innerHTML === '' || this.lastSecondary !== secondary)) {
      this.lastSecondary = secondary;
      slot2Icon.innerHTML = WEAPON_SVGS[secondary] ?? WEAPON_SVGS[0];
    }
    const slot3Icon = el('slot-3-icon');
    if (slot3Icon && slot3Icon.innerHTML === '') slot3Icon.innerHTML = WEAPON_SVGS[6];
    const slot4Icon = el('slot-4-icon');
    if (slot4Icon && slot4Icon.innerHTML === '') slot4Icon.innerHTML = WEAPON_SVGS[13];

    const slot1Ammo = el('slot-1-ammo');
    if (slot1Ammo) slot1Ammo.textContent = String(mags[1] ?? 30);
    const slot2Ammo = el('slot-2-ammo');
    if (slot2Ammo) slot2Ammo.textContent = String(mags[2] ?? 20);
    const slot3Ammo = el('slot-3-ammo');
    if (slot3Ammo) slot3Ammo.textContent = '-';
    const slot4Ammo = el('slot-4-ammo');
    if (slot4Ammo) slot4Ammo.textContent = primed ? 'PIN' : String(nades);
    const grenadeReady = el('grenade-ready');
    grenadeReady.style.display = active === 4 ? 'block' : 'none';
    grenadeReady.textContent = primed ? '手雷已拉栓 · 左键投掷' : '右键拉栓';
    grenadeReady.classList.toggle('primed', primed);
    el('slot-4').classList.toggle('primed', primed);

    for (let s = 1; s <= 4; s++) {
      const slotEl = el(`slot-${s}`);
      if (slotEl) {
        slotEl.classList.toggle('active', s === active);
      }
    }
  }

  setReloading(reloading: boolean, progress = 0) {
    if (reloading !== this.lastReloading) {
      this.lastReloading = reloading;
      const wrap = el('reload-bar-wrap');
      if (wrap) wrap.style.display = reloading ? 'block' : 'none';
      const ammoEl = el('ammo');
      if (ammoEl) ammoEl.style.color = reloading ? 'var(--accent-hi)' : 'var(--paper)';
      this.crosshair.classList.toggle('reloading', reloading);
      if (!reloading) this.lastReloadPct = -1;
    }
    if (!reloading) return;
    const pct = Math.round(Math.max(0, Math.min(1, progress)) * 100);
    if (pct === this.lastReloadPct) return;
    this.lastReloadPct = pct;
    const pctEl = el('reload-pct');
    const fillEl = el('reload-fill');
    if (pctEl) pctEl.textContent = `${pct}%`;
    if (fillEl) fillEl.style.width = `${pct}%`;
  }
  setScope(v: boolean) {
    if (v === this.lastScope) return;
    this.lastScope = v;
    if (this.scope) this.scope.style.display = v ? 'block' : 'none';
    this.crosshair.style.display = v ? 'none' : 'block';
  }


  setSpawnShield(v: boolean) {
    if (v === this.lastShield) return;
    this.lastShield = v;
    const shield = el('shield-badge');
    if (shield) shield.style.display = v ? 'block' : 'none';
  }

  setOnlineRank(online: number, rank: number) {
    const onlineEl = el('online-count');
    if (onlineEl) onlineEl.textContent = String(online);
    const rankEl = el('rank-line');
    if (rankEl) rankEl.textContent = online ? `${rank}/${online}` : '-';
  }

  setNetworkStats(latencyMs: number, outboundBps: number) {
    const latency = latencyMs ? String(Math.round(latencyMs)) : '--';
    const bandwidth = (outboundBps * 8 / 1e6).toFixed(1);
    const state = `${latency}:${bandwidth}`;
    if (state === this.lastNetwork) return;
    this.lastNetwork = state;
    el('latency-value').textContent = `${latency} ms`;
    el('bandwidth-value').textContent = `${bandwidth} Mbps`;
  }

  hitMarker(head = false) {
    this.hit.className = '';
    void this.hit.offsetWidth;
    this.hit.className = head ? 'active head' : 'active';
    clearTimeout(this.hitTimer);
    this.hitTimer = window.setTimeout(() => { this.hit.className = ''; }, head ? 220 : 150);
  }

  showKillStreak(count: number, head = false, reward = '') {
    if (count < 2) return;
    const title = count === 2 ? '双杀' : count === 3 ? '三杀' : count === 4 ? '疯狂四杀' : `${count} 连杀`;
    const extra = reward ? ` · ${reward}` : '';
    this.streak.innerHTML = `<strong>${title}</strong><span>${head ? '爆头 · ' : ''}连续击杀 ${count} 人${extra}</span>`;
    this.streak.className = 'active';
    clearTimeout(this.streakTimer);
    this.streakTimer = window.setTimeout(() => { this.streak.className = ''; }, 1800);
  }

  showKillMedal(head = false) {
    if (!this.killMedal) return;
    this.killMedal.className = '';
    void this.killMedal.offsetWidth;
    this.killMedal.className = head ? 'active head' : 'active';
    clearTimeout(this.killMedalTimer);
    this.killMedalTimer = window.setTimeout(() => { this.killMedal.className = ''; }, 1150);
  }

  setCrosshair(spread: number) {
    this.lastCrosshairSpread = spread;
    const dynamic = this.crosshairDynamic;
    const gap = this.crosshairGap;
    const px = dynamic ? Math.round((gap + Math.max(0, Math.min(32 - gap, spread))) * 10) / 10 : gap;
    if (px === this.lastCrosshair && dynamic === this.lastDynamic) return;
    this.lastCrosshair = px;
    this.lastDynamic = dynamic;
    this.crosshair.style.setProperty('--spread', `${px}px`);
    const cone = dynamic ? px - gap : 0;
    this.crosshair.dataset.accuracy = dynamic ? (cone < 4 ? 'tight' : cone < 11 ? 'warm' : 'wide') : 'fixed';
  }


  setDeathCountdown(seconds: number) {
    if (seconds === this.lastDeathCountdown) return;
    this.lastDeathCountdown = seconds;
    this.deathCountdown.style.display = seconds >= 0 ? 'block' : 'none';
    if (seconds >= 0) this.deathCountdown.textContent = `${seconds} 秒后重新部署`;
  }

  damageFlash() {
    if (!this.damage) return;
    this.damage.style.display = 'block';
    clearTimeout(this.damageTimer);
    this.damageTimer = window.setTimeout(() => {
      if (this.damage) this.damage.style.display = 'none';
    }, 130);
  }

  killFeedEntry(killer: string, victim: string, weapon: number, head: boolean, mine: boolean) {
    if (!this.killfeed) return;
    const row = document.createElement('div');
    row.className = 'kill-row' + (mine ? ' mine' : '');
    const badge = WEAPON_BADGES[weapon] ?? 'HE';
    const formatKillName = (n: string) => n.startsWith('[BOT]') ? esc(n) : `<span class="human-tag">[真人]</span>${esc(n)}`;
    row.innerHTML = `<span class="killer">${formatKillName(killer)}</span><span class="action">使用</span><span class="weapon">${esc(badge)}</span>${head ? '<span class="head-badge">爆头</span>' : ''}<span class="action">击杀</span><span class="victim">${formatKillName(victim)}</span>`;
    this.killfeed.prepend(row);
    while (this.killfeed.children.length > 6) {
      this.killfeed.lastElementChild?.remove();
    }
    setTimeout(() => row.remove(), 4200);
  }

  showPickupNotice(message: string, kind: 'ammo' | 'health' | 'speed' | 'buff') {
    const toast = el('pickup-toast');
    toast.textContent = message;
    toast.dataset.kind = kind;
    toast.classList.remove('show');
    void toast.offsetWidth;
    toast.classList.add('show');
    clearTimeout(this.toastTimer);
    this.toastTimer = window.setTimeout(() => toast.classList.remove('show'), 1200);
  }

  setFlightState(enabled: boolean, available: boolean) {
    const button = el('pause-flight-btn') as HTMLButtonElement;
    if (!button) return;
    button.disabled = !available;
    button.classList.toggle('active', enabled);
    button.setAttribute('aria-pressed', String(enabled));
    button.title = enabled ? '已开启；再次点击关闭飞行模式' : '开启飞行模式';
  }

  showFlightAnnouncement(name: string) {
    const banner = el('flight-announcement');
    banner.textContent = `${name || '特战队员'} 能飞`;
    banner.classList.remove('revenge', 'bond', 'show');
    void banner.offsetWidth;
    banner.classList.add('show');
    clearTimeout(this.announcementTimer);
    this.announcementTimer = window.setTimeout(() => banner.classList.remove('show'), 2600);
  }

  showRevengeAnnouncement(name: string) {
    const banner = el('flight-announcement');
    banner.textContent = `（${name || '玩家'}）来复仇了！`;
    banner.classList.remove('bond', 'show');
    banner.classList.add('revenge');
    void banner.offsetWidth;
    banner.classList.add('show');
    clearTimeout(this.announcementTimer);
    this.announcementTimer = window.setTimeout(() => banner.classList.remove('show', 'revenge'), 4000);
  }

  showBondEvent(kind: number, name: string, score: number) {
    const banner = el('flight-announcement');
    const actor = name || (kind === 3 ? '特战队员' : '羁绊者');
    switch (kind) {
      case 1: banner.textContent = `🔥 ${actor} 为TA报仇了 · 羁绊值 ${score}`; break;
      case 2: banner.textContent = `⚡ ${actor} 心有灵犀 · 羁绊值 ${score}`; break;
      case 3: banner.textContent = `💘 ${actor} 缔结战场羁绊`; break;
      default: banner.textContent = `💕 ${actor} 守护了TA · 羁绊值 ${score}`;
    }
    banner.classList.remove('revenge', 'show');
    banner.classList.add('bond');
    void banner.offsetWidth;
    banner.classList.add('show');
    clearTimeout(this.announcementTimer);
    this.announcementTimer = window.setTimeout(() => banner.classList.remove('show', 'bond'), 3200);
  }



  updateScoreboard(roster: RosterEntry[], states: Map<number, PlayerSnap>, myId: number) {
    const sorted = [...roster].sort((a, b) => b.kills - a.kills || a.deaths - b.deaths || a.id - b.id);
    const rank = Math.max(1, sorted.findIndex((p) => p.id === myId) + 1);
    this.setOnlineRank(sorted.length, rank);
    if (this.scoreboard.style.display !== 'block') return;

    const roomInfo = el('sb-room-info');
    if (roomInfo) roomInfo.textContent = `实时对战 · ${sorted.length} 名特战队员`;

    const body = el('sb-body');
    if (!body) return;
    body.innerHTML = sorted.map((p, i) => {
      const s = states.get(p.id);
      const alive = !!(s?.state && s.state & 1);
      const weapon = WEAPON_BADGES[s?.weapon ?? 3] ?? 'AK-47';
      const kd = p.deaths ? (p.kills / p.deaths).toFixed(2) : p.kills.toFixed(1);
      const isBot = p.name.startsWith('[BOT]') || p.name.startsWith('bot-');
      const nameHtml = isBot
        ? `<span class="bot-badge">[AI]</span><span>${esc(p.name.replace(/^\[BOT\]\s*/, ''))}</span>`
        : `<span class="human-badge">[真人]</span><b>${esc(p.name)}</b>`;
      return `
        <tr class="${p.id === myId ? 'me' : ''}">
          <td><span class="rank-badge ${i === 0 ? 'rank-gold' : i === 1 ? 'rank-silver' : i === 2 ? 'rank-bronze' : ''}">${i + 1}</span></td>
          <td>${nameHtml}${p.id === myId ? ' (你)' : ''}</td>
          <td>${alive ? '<span style="color:#10b981;font-weight:700;">存活</span>' : '<span style="color:#64748b;">阵亡</span>'}</td>
          <td>${esc(weapon)}</td>
          <td style="color:#10b981;font-weight:800;">${p.kills}</td>
          <td style="color:#f43f5e;">${p.deaths}</td>
          <td style="color:#f59e0b;font-weight:800;">${kd}</td>
        </tr>
      `;
    }).join('');
  }

  toggleScoreboard(v: boolean) {
    if (this.scoreboard) this.scoreboard.style.display = v ? 'block' : 'none';
  }

  toggleSettings(force?: boolean) {
    if (!this.settings) return;
    const open = force ?? this.settings.style.display !== 'flex';
    this.settings.style.display = open ? 'flex' : 'none';
    if (!open) this.onSettingsClose?.();
  }

  isSettingsOpen(): boolean {
    return this.settings?.style.display === 'flex';
  }

  isPaused(): boolean {
    return this.pause?.style.display === 'flex';
  }

  showPause(v: boolean) {
    if (this.pause) this.pause.style.display = v ? 'flex' : 'none';
  }

  onPauseClick(fn: () => void) {
    el('pause-resume-btn')?.addEventListener('click', fn);
  }

  showDisconnect(message = '正在重新连接…') {
    clearInterval(this.reconnectTimer);
    this.disconnect.classList.remove('upgrade');
    el('disconnect-label').textContent = 'NETWORK STATUS';
    el('disconnect-message').textContent = message;
    el('disconnect-detail').textContent = '正在重新连接并同步世界状态，请稍候。';
    this.disconnect.style.display = 'flex';
  }

  showUpgrade(seconds: number) {
    clearInterval(this.reconnectTimer);
    this.disconnect.classList.add('upgrade');
    el('disconnect-label').textContent = 'LIVE UPDATE';
    el('disconnect-message').textContent = '服务器正在升级';
    const detail = el('disconnect-detail');
    let remaining = Math.max(1, seconds);
    const update = () => { detail.textContent = `保留当前战局，预计 ${remaining} 秒后自动重连。`; };
    update();
    this.reconnectTimer = window.setInterval(() => {
      remaining = Math.max(0, remaining - 1);
      if (remaining === 0) {
        clearInterval(this.reconnectTimer);
        detail.textContent = '升级已完成，正在恢复连接…';
      } else update();
    }, 1000);
    this.disconnect.style.setProperty('--upgrade-duration', `${remaining}s`);
    this.disconnect.style.display = 'flex';
  }

  hideDisconnect() {
    clearInterval(this.reconnectTimer);
    this.disconnect.style.display = 'none';
  }


  exitMatch() {
    document.exitPointerLock?.();
    void document.exitFullscreen?.();
    this.root.style.display = 'none';
    this.root.classList.remove('low-hp');
    this.menu.style.display = 'block';
    this.characterPreview?.setVisible(true);
    if (this.pause) this.pause.style.display = 'none';
    if (this.settings) this.settings.style.display = 'none';
    if (this.scoreboard) this.scoreboard.style.display = 'none';
    const strip = el('match-strip');
    const middle = strip?.children[1];
    if (middle) middle.textContent = '黄昏要塞';
    const shield = el('shield-badge');
    if (shield) shield.style.display = '';
    const netStats = el('net-stats');
    if (netStats) netStats.style.display = '';
    this.loadLeaderboard();
    this.refreshWeaponProgress?.();
  }

  async loadLeaderboard() {
    try {
      const res = await fetch('/api/leaderboard');
      if (!res.ok) throw new Error('leaderboard fetch failed');
      const rows: { name: string; kills: number; deaths: number; kd: number }[] = await res.json();
      const lbEl = el('leaderboard');
      if (!lbEl) return;
      if (!rows.length) {
        lbEl.innerHTML = '<tr><td colspan="5" style="text-align:center;color:#94a3b8;padding:16px;">暂无战绩数据</td></tr>';
        return;
      }
      lbEl.innerHTML = rows.map((r, i) => `
        <tr>
          <td><span class="rank-badge ${i === 0 ? 'rank-gold' : i === 1 ? 'rank-silver' : i === 2 ? 'rank-bronze' : ''}">${i + 1}</span></td>
          <td><span class="human-badge">[真人]</span><b>${esc(r.name)}</b></td>
          <td style="color:#10b981;font-weight:800;">${r.kills}</td>
          <td style="color:#f43f5e;">${r.deaths}</td>
          <td style="color:#f59e0b;font-weight:800;">${r.kd.toFixed(2)}</td>
        </tr>
      `).join('');
    } catch {
      const lbEl = el('leaderboard');
      if (lbEl) lbEl.innerHTML = '<tr><td colspan="5" style="text-align:center;color:#f43f5e;padding:16px;">排行榜暂不可用</td></tr>';
    }
  }
}
