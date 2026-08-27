import * as THREE from 'three';
import { Net, type GameEvent } from './net.js';
import { WorldView, canOccupy, moveAABB } from './world.js';
import { LocalPlayer, RemotePlayers } from './player.js';
import { Weapons } from './weapons.js';
import { Hud } from './hud.js';
import { AudioEngine, type SfxName } from './audio.js';
import { ParticleSystem } from './particles.js';
import { KEY, PHYS, WEAPONS, XM_PELLETS, ULTIMATE_REQUIREMENT, isGun, isPistol, isShotgun, isSniper, scopeSettleMs, type MapData, type PlayerSnap, type RosterEntry, type WeaponDef } from './constants.js';
import bundledMap from '../../map.json';

const proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
const wsUrl = import.meta.env.VITE_WS_URL || `${proto}//${location.host}/ws`;
const mapSize = (bundledMap as MapData).size;

const scene = new THREE.Scene();
scene.background = new THREE.Color(0x78939d);
scene.fog = new THREE.Fog(0x8ea5a8, 52, 265);

const camera = new THREE.PerspectiveCamera(75, window.innerWidth / window.innerHeight, 0.05, 450);
camera.rotation.order = 'YXZ';
camera.layers.enable(0);
camera.layers.disable(1);
const renderer = new THREE.WebGLRenderer({ antialias: true, powerPreference: 'high-performance' });
renderer.toneMapping = THREE.ACESFilmicToneMapping;
renderer.toneMappingExposure = 1.07;
renderer.outputColorSpace = THREE.SRGBColorSpace;
document.body.appendChild(renderer.domElement);

const hemiLight = new THREE.HemisphereLight(0xdbe8e7, 0x485247, 1.22);
scene.add(hemiLight);

const sun = new THREE.DirectionalLight(0xffe2b0, 1.22);
sun.position.set(90, 120, -70);
scene.add(sun);
scene.add(camera);

const fillLight = new THREE.DirectionalLight(0x789fb0, 0.5);
fillLight.position.set(-70, 50, 70);
scene.add(fillLight);
const hud = new Hud();
const audio = new AudioEngine();
window.addEventListener('pointerdown', () => audio.init(), { once: true, capture: true });
window.addEventListener('touchstart', () => audio.init(), { once: true, capture: true, passive: true });
const net = new Net();
const local = new LocalPlayer();
const remotes = new RemotePlayers(scene);
const particles = new ParticleSystem(scene);
const weapons = new Weapons(camera, scene);
weapons.onPlaySound = (name, vol, pitch) => audio.play(name, vol, pitch);
let world: WorldView | null = null;
let joined = false;
let practice = false;
let alive = false;
let myName = '';
let killerId = -1;
let killStreak = 0;
let recoilBoostUntil = 0;
let respawnAt = 0;
let screenShake = 0;
let aiming = false;
let lastStep = 0;
let inputSeq = 0;
let shotSeq = 0;
let lastInput = 0;
let lastSentInputKeys = -1;
let lastSentInputYaw = NaN;
let lastSentInputPitch = NaN;
let lastWheelSwitch = 0;
const INPUT_INTERVAL = 1000 / 60;
const SPEED_BOOST_MULTIPLIER = 1.35;
let fireHeld = false;
let firePressed = false;
let knifeHeavyQueued = false;
let mouseX = 0;
let mouseY = 0;
let mag = 30;
let reserve = 90;
let nades = 1;
let reloadPendingSlot = 0;
let reloadPendingMag = 0;
let reloadPendingReserve = 0;
let landingKick = 0;
let cameraEyeHeight = PHYS.eyeHeight;
let landingPenaltyUntil = 0;
let aimStartedAt = 0;
let awpRescopeAt = 0;
let speedBoostUntil = 0;
let ultimatePoints = 0;
let ultimateKind = 0;
let ultimateUntil = 0;
let blackDreamUntil = 0;
let patternShots = 0;
let lastPatternShot = 0;
let latencyMs = 0;
let serverOutboundBps = 0;
let crosshairScale = 1;
let displayedCrosshair = 0;
let activeSlot = 1;
let lastWeaponSlot = 1;
let grenadePrimed = false;
let userPrimaryChoice = -1;
let userSecondaryChoice = -1;
let userPrimaryWeaponSkin = 3;
let userSecondaryWeaponSkin = 3;
// 非法组队与辅助瞄准状态
let myAllyBotId = -1;
let myBondMateId = -1;
let assistTargetId = -1;
let assistLocked = false;
let assistYaw = 0;
let assistPitch = 0;
const PRIMARY_IDS = [3, 4, 2, 5, 8, 9, 10, 11, 12];
const SECONDARY_IDS = [0, 1, 7];
const CROSSHAIR_RECOVERY = [15, 8, 18, 11, 14, 7, 14, 17, 15, 16, 15, 9, 10] as const;
const GRENADE_THROW_SPEED = 28;
const GRENADE_LIFT = 4.2;
// 移动端辅助瞄准：锁敌锥角、磁吸速率与弹道修正上限。
const AIM_ASSIST_RANGE = 38;
const AIM_ASSIST_CONE_HIP = 9 * Math.PI / 180;
const AIM_ASSIST_CONE_ADS = 6 * Math.PI / 180;
const AIM_ASSIST_DROP = 14 * Math.PI / 180;
const AIM_ASSIST_SHOT_SNAP_MAX = 3.5 * Math.PI / 180;
const AIM_ASSIST_PULL_HIP = 110 * Math.PI / 180;
const AIM_ASSIST_PULL_ADS = 70 * Math.PI / 180;
const GRENADE_PREVIEW_STEPS = 45;
const GRENADE_PREVIEW_DT = 1.8 / GRENADE_PREVIEW_STEPS;

function resolveLoadout(p: number, s: number): { primary: number; secondary: number } {
  const actualPrimary = p === -1 ? PRIMARY_IDS[Math.floor(Math.random() * PRIMARY_IDS.length)] : p;
  const actualSecondary = s === -1 ? SECONDARY_IDS[Math.floor(Math.random() * SECONDARY_IDS.length)] : s;
  return { primary: actualPrimary, secondary: actualSecondary };
}

let primaryWeapon = 3;
let secondaryWeapon = 0;
let primaryWeaponSkin = 0;
let secondaryWeaponSkin = 0;
const slotMags = [0, WEAPONS[3].mag, WEAPONS[0].mag, 0];
const slotReserves = [0, WEAPONS[3].reserve, WEAPONS[0].reserve, 0];
let cameraStepOffset = 0;
const states = new Map<number, PlayerSnap>();
const names = new Map<number, string>();
const roster = new Map<number, RosterEntry>();
const pickupStates = new Map<number, { kind: number; origin: [number, number, number] }>();
remotes.nameOf = (id) => names.get(id) ?? `特战队员${id}`;
remotes.isAllyOf = (id) => id === myAllyBotId;
remotes.isBondOf = (id) => id === myBondMateId;
const deathTarget = new THREE.Vector3();
const remoteShotOrigin = new THREE.Vector3();
const remoteShotDir = new THREE.Vector3();
const localShotOrigin = new THREE.Vector3();
const localShotDir = new THREE.Vector3();
const localPelletDir = new THREE.Vector3();
const impactPoint = new THREE.Vector3();
const impactNormal = new THREE.Vector3(0, 1, 0);
const eventOrigin = new THREE.Vector3();
const eventVelocity = new THREE.Vector3();
const grenadeOrigin = new THREE.Vector3();
const grenadeVelocityVec = new THREE.Vector3();
const shotRight = new THREE.Vector3();
const shotUp = new THREE.Vector3();
const assistRayOrigin = new THREE.Vector3();
const assistRayDir = new THREE.Vector3();
const cameraCorrection = new THREE.Vector3();
const grenadePreviewPositions = new Float32Array((GRENADE_PREVIEW_STEPS + 1) * 3);
const grenadePreviewGeometry = new THREE.BufferGeometry();
const grenadePreviewPositionAttr = new THREE.BufferAttribute(grenadePreviewPositions, 3);
grenadePreviewPositionAttr.setUsage(THREE.DynamicDrawUsage);
grenadePreviewGeometry.setAttribute('position', grenadePreviewPositionAttr);
const grenadePreviewMaterial = new THREE.LineDashedMaterial({ color: 0xffd166, transparent: true, opacity: 0.95, dashSize: 0.34, gapSize: 0.14, depthTest: false });
const grenadePreview = new THREE.Line(grenadePreviewGeometry, grenadePreviewMaterial);
grenadePreview.visible = false;
grenadePreview.frustumCulled = false;
grenadePreview.renderOrder = 4;
scene.add(grenadePreview);
const grenadePreviewPos = new THREE.Vector3();
const grenadePreviewVel = new THREE.Vector3();
const grenadePreviewDelta = new THREE.Vector3();
const grenadePreviewDir = new THREE.Vector3();

function resize() {
  const ratio = hud.quality === 'low' ? 0.75 : hud.quality === 'high' ? Math.min(window.devicePixelRatio, 1.5) : 1;
  renderer.setPixelRatio(ratio);
  renderer.setSize(window.innerWidth, window.innerHeight);
  camera.aspect = window.innerWidth / window.innerHeight;
  camera.updateProjectionMatrix();
  weapons.setAspect(camera.aspect);
  crosshairScale = window.innerHeight * 0.5 / Math.tan(camera.fov * Math.PI / 360);
}
resize();
window.addEventListener('resize', resize);

hud.onQualityChange = (q) => {
  if (world) world.clouds.visible = q !== 'low';
  resize();
};
hud.onVolumeChange = (v) => audio.setVolume(v);
hud.onFovChange = () => {
  if (!aiming) {
    camera.fov = hud.hipFov;
    camera.updateProjectionMatrix();
    crosshairScale = window.innerHeight * 0.5 / Math.tan(camera.fov * Math.PI / 360);
  }
};
camera.fov = hud.hipFov;
camera.updateProjectionMatrix();
crosshairScale = window.innerHeight * 0.5 / Math.tan(camera.fov * Math.PI / 360);
audio.setVolume(hud.volume);

const keys = new Set<string>();
let touchForward = false;
let touchBack = false;
let touchLeft = false;
let touchRight = false;
let touchJump = false;
let touchCrouch = false;
let touchSprint = false;

function keyMask(): number {
  let k = 0;
  if (keys.has('KeyW') || touchForward) k |= KEY.Forward;
  if (keys.has('KeyS') || touchBack) k |= KEY.Back;
  if (keys.has('KeyA') || touchLeft) k |= KEY.Left;
  if (keys.has('KeyD') || touchRight) k |= KEY.Right;
  if (keys.has('Space') || touchJump) k |= KEY.Jump;
  if (keys.has('KeyC') || keys.has('ControlLeft') || keys.has('ControlRight') || touchCrouch) k |= KEY.Crouch;
  // Shift：地面 = 冲刺，飞行中 = 下降（服务端同一语义）
  if (keys.has('ShiftLeft') || keys.has('ShiftRight') || touchSprint) k |= KEY.Descend;
  return k;
}

function resetInventory(primary: number, secondary: number) {
  primaryWeapon = primary;
  secondaryWeapon = secondary;
  slotMags[1] = WEAPONS[primary].mag;
  slotReserves[1] = WEAPONS[primary].reserve;
  slotMags[2] = WEAPONS[secondary].mag;
  slotReserves[2] = WEAPONS[secondary].reserve;
}

function refreshWeaponHud() {
  hud.setInventory(primaryWeapon, secondaryWeapon, activeSlot, slotMags, nades, grenadePrimed);
  if (activeSlot === 3) hud.setAmmoDisplay('KNIFE', '—', '');
  else if (activeSlot === 4) hud.setAmmoDisplay('HE', String(nades), grenadePrimed ? 'PIN' : '');
  else hud.setAmmoDisplay(WEAPONS[weapons.weaponId]?.name ?? '', String(mag), String(reserve));
  const touchAimLabel = document.querySelector('#btn-touch-aim span');
  if (touchAimLabel) touchAimLabel.textContent = activeSlot === 3 ? '重击' : '开镜';
}

function grenadeVelocity(out: THREE.Vector3) {
  const cp = Math.cos(local.pitch);
  return out.set(-Math.sin(local.yaw) * cp * GRENADE_THROW_SPEED, Math.sin(local.pitch) * GRENADE_THROW_SPEED + GRENADE_LIFT, -Math.cos(local.yaw) * cp * GRENADE_THROW_SPEED);
}

function updateGrenadePreview(t: number) {
  if (!joined || !alive || activeSlot !== 4 || !grenadePrimed) {
    grenadePreview.visible = false;
    return;
  }
  grenadePreview.visible = true;
  grenadePreviewPos.set(local.pos.x, local.eyeY(), local.pos.z);
  grenadeVelocity(grenadePreviewVel);
  let count = 1;
  grenadePreviewPositions[0] = grenadePreviewPos.x;
  grenadePreviewPositions[1] = grenadePreviewPos.y;
  grenadePreviewPositions[2] = grenadePreviewPos.z;
  for (let i = 0; i < GRENADE_PREVIEW_STEPS; i++) {
    grenadePreviewVel.y += PHYS.gravity * GRENADE_PREVIEW_DT;
    grenadePreviewDelta.copy(grenadePreviewVel).multiplyScalar(GRENADE_PREVIEW_DT);
    const travel = grenadePreviewDelta.length();
    if (world && travel > 0) {
      grenadePreviewDir.copy(grenadePreviewDelta).multiplyScalar(1 / travel);
      const hit = world.raycastDistance(grenadePreviewPos, grenadePreviewDir, travel);
      if (hit < travel) {
        grenadePreviewPos.addScaledVector(grenadePreviewDir, Math.max(0, hit - 0.03));
        const offset = count++ * 3;
        grenadePreviewPositions[offset] = grenadePreviewPos.x;
        grenadePreviewPositions[offset + 1] = grenadePreviewPos.y;
        grenadePreviewPositions[offset + 2] = grenadePreviewPos.z;
        break;
      }
    }
    grenadePreviewPos.add(grenadePreviewDelta);
    if (grenadePreviewPos.y < 0.09) grenadePreviewPos.y = 0.09;
    const offset = count++ * 3;
    grenadePreviewPositions[offset] = grenadePreviewPos.x;
    grenadePreviewPositions[offset + 1] = grenadePreviewPos.y;
    grenadePreviewPositions[offset + 2] = grenadePreviewPos.z;
    if (grenadePreviewPos.y <= 0.09) break;
  }
  grenadePreviewGeometry.setDrawRange(0, count);
  grenadePreviewPositionAttr.needsUpdate = true;
  grenadePreview.computeLineDistances();
  grenadePreviewMaterial.opacity = 0.78 + Math.sin(t * 0.008) * 0.2;
}
function clearCombatInput() {
  keys.clear();
  touchForward = false;
  touchBack = false;
  touchLeft = false;
  touchRight = false;
  touchJump = false;
  touchCrouch = false;
  touchSprint = false;
  fireHeld = false;
  firePressed = false;
  knifeHeavyQueued = false;
  grenadePrimed = false;
  assistLocked = false;
  assistTargetId = -1;
  hud.setAssistLock(false);
  stopAiming();
  weapons.resetMotion();
  const knob = document.getElementById('touch-joystick-knob');
  if (knob) knob.style.transform = 'translate(0px, 0px)';
  document.querySelectorAll('#touch-controls .active').forEach((button) => button.classList.remove('active'));
  if (joined && alive && !practice) net.sendInput(inputSeq++, 0, local.yaw, local.pitch);
  refreshWeaponHud();
}

const keyboard = (navigator as Navigator & { keyboard?: { lock(keys?: string[]): Promise<void>; unlock(): void } }).keyboard;
let capturePending = false;
let pointerLockedAt = 0;
let pointerUnlockRequested = false;
let historyGuarded = false;

function guardMatchHistory() {
  if (historyGuarded) return;
  history.pushState({ pixelStrikeMatch: true }, '');
  historyGuarded = true;
}

function releaseMatchHistory() {
  if (!historyGuarded) return;
  historyGuarded = false;
  if (history.state?.pixelStrikeMatch) history.back();
}

async function lockPointer() {
  capturePending = false;
  if (document.fullscreenElement) {
    try { await keyboard?.lock(['Escape']); } catch {}
  }
  if (document.pointerLockElement !== renderer.domElement) {
    try {
      await renderer.domElement.requestPointerLock({ unadjustedMovement: true });
    } catch (error) {
      if (error instanceof DOMException && error.name === 'NotSupportedError') {
        try { await renderer.domElement.requestPointerLock(); } catch {}
      }
    }
  }
}

function captureGame() {
  pointerUnlockRequested = false;
  if (document.activeElement instanceof HTMLElement) document.activeElement.blur();
  if (document.body.classList.contains('touch-device')) {
    capturePending = false;
    if (!document.fullscreenElement && document.fullscreenEnabled) void document.documentElement.requestFullscreen({ navigationUI: 'hide' }).catch(() => {});
    return;
  }
  capturePending = true;
  if (!document.fullscreenElement && document.fullscreenEnabled) {
    void document.documentElement.requestFullscreen({ navigationUI: 'hide' }).catch(() => void lockPointer());
    return;
  }
  void lockPointer();
}

document.addEventListener('fullscreenchange', () => {
  if (capturePending && document.fullscreenElement) void lockPointer();
  else if (!document.fullscreenElement) {
    capturePending = false;
    keyboard?.unlock();
  }
});


function stopAiming() {
  aiming = false;
  aimStartedAt = 0;
  awpRescopeAt = 0;
  document.getElementById('btn-touch-aim')?.classList.remove('active');
}

function isInteractiveTarget(target: EventTarget | null): boolean {
  return target instanceof Element && !!target.closest('input, select, textarea, button, a, [contenteditable="true"]');
}


function startReload(t = performance.now(), notifyServer = true) {
  if (!alive || activeSlot > 2 || reloadPendingSlot === activeSlot || weapons.isReloading(t) || !weapons.startReload(t, reserve)) return;
  reloadPendingSlot = activeSlot;
  reloadPendingMag = weapons.ammoLocal;
  reloadPendingReserve = reserve;
  if (notifyServer && !practice) net.sendReload();
  stopAiming();
}

function selectSlot(slot: number) {
  if (!joined || !alive || slot === activeSlot) return;
  if (slot === 4 ? nades <= 0 : slot < 1 || slot > 3) return;
  fireHeld = false;
  firePressed = false;
  knifeHeavyQueued = false;
  patternShots = 0;
  lastPatternShot = 0;
  weapons.nextFireAt = Math.max(weapons.nextFireAt, performance.now() + 220);
  if (slot === 4) {
    activeSlot = 4;
    grenadePrimed = false;
    stopAiming();
    weapons.cancelReload();
    weapons.group.visible = false;
    audio.play('weapon_switch', 0.7);
    refreshWeaponHud();
    return;
  }
  activeSlot = slot;
  lastWeaponSlot = slot;
  grenadePrimed = false;
  weapons.group.visible = true;
  stopAiming();
  weapons.cancelReload();
  if (slot !== reloadPendingSlot) reloadPendingSlot = 0;
  const weapon = slot === 1 ? primaryWeapon : slot === 2 ? secondaryWeapon : 6;
  const weaponSkin = slot === 1 ? primaryWeaponSkin : slot === 2 ? secondaryWeaponSkin : 0;
  if (weapon !== weapons.weaponId || weaponSkin !== weapons.weaponSkin) weapons.build(weapon, weaponSkin);
  local.weaponId = weapon;
  mag = slot <= 2 ? slotMags[slot] : 0;
  reserve = slot <= 2 ? slotReserves[slot] : 0;
  weapons.ammoLocal = mag;
  if (!practice) net.switchSlot(slot);
  audio.play('weapon_switch', 0.7);
  refreshWeaponHud();
}
function primeGrenade() {
  if (activeSlot !== 4 || grenadePrimed || nades <= 0) return;
  grenadePrimed = true;
  audio.play('bolt_rack', 0.55);
  refreshWeaponHud();
}

function throwGrenade() {
  if (activeSlot !== 4 || !grenadePrimed || nades <= 0) return;
  grenadePrimed = false;
  nades--;
  grenadeOrigin.set(local.pos.x, local.eyeY(), local.pos.z);
  particles.spawnGrenade(net.yourId, grenadeOrigin, grenadeVelocity(grenadeVelocityVec));
  if (!practice) net.sendGrenade(local.yaw, local.pitch);
  activeSlot = lastWeaponSlot;
  weapons.group.visible = true;
  audio.play('weapon_switch', 0.7);
  refreshWeaponHud();
}

window.addEventListener('keydown', (e) => {
  const isLock = document.pointerLockElement === renderer.domElement;
  const interactive = !isLock && isInteractiveTarget(e.target);

  if (e.code === 'Escape' && joined) {
    pointerUnlockRequested = true;
    e.preventDefault();
    clearCombatInput();
    keyboard?.unlock();
    document.exitPointerLock?.();
    if (document.fullscreenElement) void document.exitFullscreen();
    hud.showPause(true);
    return;
  }
  if (!joined || interactive) return;
  if (e.code === 'KeyT' && !e.repeat && !e.metaKey && !e.altKey && !e.ctrlKey) {
    e.preventDefault();
    clearCombatInput();
    hud.setChatInputOpen(true);
    hud.chatInput.focus();
    return;
  }
  if (e.code === 'Enter' && !e.repeat && !e.metaKey && !e.altKey && !e.ctrlKey) {
    // Enter 与 T 一样打开聊天（输入框内 Enter 由输入框自己处理发送）
    if (hud.isChatComposing()) return;
    e.preventDefault();
    clearCombatInput();
    hud.setChatInputOpen(true);
    hud.chatInput.focus();
    return;
  }
  if (e.metaKey || e.altKey) return;
  if (e.ctrlKey && e.code !== 'ControlLeft' && e.code !== 'ControlRight') {
    if (!/^(Key[WASDRG]|Digit[1234]|Space)$/.test(e.code)) return;
    e.preventDefault();
  }
  if (e.code === 'Tab') {
    e.preventDefault();
    hud.toggleScoreboard(true);
    refreshScoreboard();
    if (!e.repeat) net.requestRoster();
    return;
  }
  if (!alive) return;
  keys.add(e.code);
  if (e.repeat) return;
  if (/^Key(?:Z|X|V)$/.test(e.code)) {
    const kind = e.code === 'KeyZ' ? 1 : e.code === 'KeyX' ? 2 : 3;
    if (ultimatePoints >= ULTIMATE_REQUIREMENT) {
      net.castUltimate(kind);
      hud.toggleUltimateSelector(false);
    }
    return;
  }
  if (/^Digit[1234]$/.test(e.code)) {
    e.preventDefault();
    selectSlot(+e.code.at(-1)!);
    return;
  }
  if (e.code === 'KeyR') {
    e.preventDefault();
    startReload();
    return;
  }
  if (e.code === 'KeyG') {
    e.preventDefault();
    selectSlot(4);
  }
}, { capture: true });

window.addEventListener('keyup', (e) => {
  keys.delete(e.code);
  if (e.code === 'Tab') hud.toggleScoreboard(false);
}, { capture: true });

window.addEventListener('blur', clearCombatInput);
document.addEventListener('visibilitychange', () => {
  if (document.hidden) clearCombatInput();
});

window.addEventListener('mousemove', (e) => {
  if (!alive || document.pointerLockElement !== renderer.domElement) return;
  if (e.buttons & 2) e.preventDefault();
  if (performance.now() - pointerLockedAt < 120) return; // Suppress initial pointerlock re-center jump

  // Pointer-lock deltas are raw device counts; preserve fast flicks while rejecting impossible recenter spikes.
  if (Math.abs(e.movementX) > 1200 || Math.abs(e.movementY) > 1200) return;

  const mx = Math.max(-512, Math.min(512, e.movementX));
  const my = Math.max(-512, Math.min(512, e.movementY));
  const adsScale = aiming ? Math.tan(camera.fov * Math.PI / 360) / Math.tan(hud.hipFov * Math.PI / 360) * hud.adsSensitivity : 1;
  const sens = hud.sensitivity * adsScale;
  local.yaw -= mx * sens;
  local.pitch = Math.max(-1.45, Math.min(1.45, local.pitch - my * sens));
  mouseX = Math.max(-160, Math.min(160, mouseX + mx));
  mouseY = Math.max(-160, Math.min(160, mouseY + my));
}, { capture: true, passive: false });
window.addEventListener('mousedown', (e) => {
  if (!joined) return;
  const interactive = isInteractiveTarget(e.target);
  if (!interactive || e.button !== 0) e.preventDefault();
  if (e.button !== 0) e.stopImmediatePropagation();
  if (interactive) return;
  document.getSelection()?.removeAllRanges();
  if (!alive || hud.isSettingsOpen()) return;
  if (document.pointerLockElement !== renderer.domElement) {
    void captureGame();
    return;
  }
  if (activeSlot === 4) {
    if (e.button === 2) primeGrenade();
    else if (e.button === 0) throwGrenade();
    return;
  }
  if (e.button === 0) {
    fireHeld = true;
    firePressed = true;
  } else if (e.button === 2) {
    if (weapons.weaponId === 6) {
      knifeHeavyQueued = true;
    } else {
      if (isSniper(weapons.weaponId) && awpRescopeAt) return;
      if (aiming) stopAiming();
      else {
        aimStartedAt = performance.now();
        aiming = true;
      }
    }
  }
}, { capture: true });

window.addEventListener('mouseup', (e) => {
  if (joined && (!isInteractiveTarget(e.target) || e.button !== 0)) e.preventDefault();
  if (e.button !== 0) e.stopImmediatePropagation();
  if (e.button === 0) fireHeld = false;
}, { capture: true });

for (const type of ['pointerdown', 'pointerup']) {
  window.addEventListener(type, (e) => {
    if (joined && e instanceof PointerEvent && e.pointerType === 'mouse' && e.button >= 3) {
      e.preventDefault();
      e.stopImmediatePropagation();
    }
  }, { capture: true, passive: false });
}


window.addEventListener('popstate', () => {
  if (!historyGuarded) return;
  history.pushState({ pixelStrikeMatch: true }, '');
  if (joined) void captureGame();
});

window.addEventListener('pointermove', (e) => {
  if (joined && e.pointerType === 'mouse' && e.buttons && !hud.isSettingsOpen()) {
    e.preventDefault();
    e.stopPropagation();
  }
}, { capture: true, passive: false });
document.addEventListener('contextmenu', (e) => {
  e.preventDefault();
  e.stopImmediatePropagation();
}, { capture: true });
document.addEventListener('auxclick', (e) => {
  e.preventDefault();
  e.stopImmediatePropagation();
}, { capture: true });
for (const type of ['selectstart', 'dragstart', 'dragover', 'drop', 'copy', 'cut', 'paste', 'gesturestart', 'gesturechange', 'gestureend']) {
  document.addEventListener(type, (e) => {
    if (!isInteractiveTarget(e.target)) e.preventDefault();
  }, { capture: true });
}
window.addEventListener('wheel', (e) => {
  if (joined && alive && !hud.isSettingsOpen()) {
    e.preventDefault();
    const now = performance.now();
    if (now - lastWheelSwitch < 110) return;
    lastWheelSwitch = now;
    if (e.deltaY > 0) {
      const next = activeSlot === 4 ? 1 : activeSlot + 1;
      selectSlot(next);
    } else if (e.deltaY < 0) {
      const prev = activeSlot === 1 ? 4 : activeSlot - 1;
      selectSlot(prev);
    }
  } else if (joined) {
    e.preventDefault();
  }
}, { capture: true, passive: false });

document.addEventListener('pointerlockchange', () => {
  const isLocked = document.pointerLockElement === renderer.domElement;
  if (!isLocked) {
    clearCombatInput();
    if (joined && alive && document.fullscreenElement && document.hasFocus() && !pointerUnlockRequested && !hud.isSettingsOpen()) {
      void lockPointer();
      return;
    }
  } else {
    pointerUnlockRequested = false;
    pointerLockedAt = performance.now();
    if (document.activeElement instanceof HTMLElement) document.activeElement.blur();
    document.getSelection()?.removeAllRanges();
  }
  const isTouch = document.body.classList.contains('touch-device');
  if (joined && alive && !isTouch) hud.showPause(!isLocked && !hud.isSettingsOpen());
  else if (!joined || !alive) hud.showPause(false);
});

function setupTouchControls() {
  const isTouchPointer = (e: PointerEvent) => e.pointerType === 'touch' || e.pointerType === 'pen';
  if (matchMedia('(pointer: coarse)').matches) document.body.classList.add('touch-device');
  window.addEventListener('pointerdown', (e) => {
    if (isTouchPointer(e)) document.body.classList.add('touch-device');
  }, { once: true, passive: true });

  const joystickZone = document.getElementById('touch-joystick-zone');
  const joystickBase = document.getElementById('touch-joystick-base');
  const joystickKnob = document.getElementById('touch-joystick-knob');
  let joystickPointerId: number | null = null;
  let joystickCenter = { x: 0, y: 0 };

  const updateJoystick = (clientX: number, clientY: number) => {
    if (!joined || !alive || hud.isSettingsOpen() || hud.isPaused()) return;
    const dx = clientX - joystickCenter.x;
    const dy = clientY - joystickCenter.y;
    const dist = Math.hypot(dx, dy);
    const maxRadius = 38;
    const clampedDist = Math.min(dist, maxRadius);
    const angle = Math.atan2(dy, dx);
    const kx = Math.cos(angle) * clampedDist;
    const ky = Math.sin(angle) * clampedDist;
    if (joystickKnob) joystickKnob.style.transform = `translate(${kx.toFixed(1)}px, ${ky.toFixed(1)}px)`;
    const nx = clampedDist > 8 ? kx / maxRadius : 0;
    const ny = clampedDist > 8 ? ky / maxRadius : 0;
    touchForward = ny < -0.26;
    touchBack = ny > 0.26;
    touchLeft = nx < -0.26;
    touchRight = nx > 0.26;
  };

  joystickZone?.addEventListener('pointerdown', (e) => {
    if (!isTouchPointer(e) || joystickPointerId !== null) return;
    e.preventDefault();
    e.stopPropagation();
    joystickPointerId = e.pointerId;
    joystickZone.setPointerCapture(e.pointerId);
    const rect = joystickBase?.getBoundingClientRect();
    if (rect) joystickCenter = { x: rect.left + rect.width / 2, y: rect.top + rect.height / 2 };
    updateJoystick(e.clientX, e.clientY);
  });
  joystickZone?.addEventListener('pointermove', (e) => {
    if (e.pointerId === joystickPointerId) updateJoystick(e.clientX, e.clientY);
  });
  const resetJoystick = (e: PointerEvent) => {
    if (e.pointerId !== joystickPointerId) return;
    joystickPointerId = null;
    touchForward = touchBack = touchLeft = touchRight = false;
    if (joystickKnob) joystickKnob.style.transform = 'translate(0px, 0px)';
  };
  joystickZone?.addEventListener('pointerup', resetJoystick);
  joystickZone?.addEventListener('pointercancel', resetJoystick);

  let lookPointerId: number | null = null;
  let lastLookX = 0;
  let lastLookY = 0;
  window.addEventListener('pointerdown', (e) => {
    if (!isTouchPointer(e) || !joined || !alive || hud.isSettingsOpen() || hud.isPaused()) return;
    const target = e.target as HTMLElement | null;
    if (target?.closest('button, select, input, .mc-slot, #touch-joystick-zone, #menu, .overlay, #scoreboard')) return;
    if (lookPointerId === null) {
      lookPointerId = e.pointerId;
      lastLookX = e.clientX;
      lastLookY = e.clientY;
    }
  }, { passive: true });
  window.addEventListener('pointermove', (e) => {
    if (e.pointerId !== lookPointerId || !joined || !alive || hud.isSettingsOpen() || hud.isPaused()) return;
    const dx = e.clientX - lastLookX;
    const dy = e.clientY - lastLookY;
    lastLookX = e.clientX;
    lastLookY = e.clientY;
    if (Math.abs(dx) > 300 || Math.abs(dy) > 300) return;
    const adsScale = aiming ? (Math.tan(camera.fov * Math.PI / 360) / Math.tan(hud.hipFov * Math.PI / 360)) * hud.adsSensitivity : 1;
    const sens = hud.touchSensitivity * adsScale;
    local.yaw -= dx * sens;
    local.pitch = Math.max(-1.45, Math.min(1.45, local.pitch - dy * sens));
    mouseX = Math.max(-160, Math.min(160, mouseX + dx));
    mouseY = Math.max(-160, Math.min(160, mouseY + dy));
  }, { passive: true });
  const stopLook = (e: PointerEvent) => {
    if (e.pointerId === lookPointerId) lookPointerId = null;
  };
  window.addEventListener('pointerup', stopLook, { passive: true });
  window.addEventListener('pointercancel', stopLook, { passive: true });

  const bindHold = (button: HTMLElement | null, start: () => boolean, stop: () => void) => {
    let pointerId: number | null = null;
    button?.addEventListener('pointerdown', (e) => {
      if (!isTouchPointer(e) || pointerId !== null) return;
      e.preventDefault();
      e.stopPropagation();
      if (!start()) return;
      pointerId = e.pointerId;
      button.setPointerCapture(e.pointerId);
      button.classList.add('active');
    });
    const end = (e: PointerEvent) => {
      if (e.pointerId !== pointerId) return;
      pointerId = null;
      stop();
      button?.classList.remove('active');
    };
    button?.addEventListener('pointerup', end);
    button?.addEventListener('pointercancel', end);
  };
  const bindPress = (button: HTMLElement | null, action: () => void) => {
    button?.addEventListener('pointerdown', (e) => {
      if (!isTouchPointer(e)) return;
      e.preventDefault();
      e.stopPropagation();
      action();
    });
  };

  const fireBtn = document.getElementById('btn-touch-fire');
  bindHold(fireBtn, () => {
    if (!alive) return false;
    if (activeSlot === 4) {
      if (grenadePrimed) throwGrenade();
      else primeGrenade();
      return false;
    }
    fireHeld = true;
    firePressed = true;
    return true;
  }, () => { fireHeld = false; });

  const aimBtn = document.getElementById('btn-touch-aim');
  bindPress(aimBtn, () => {
    if (!alive) return;
    if (weapons.weaponId === 6) {
      knifeHeavyQueued = true;
      return;
    }
    if (isSniper(weapons.weaponId) && awpRescopeAt) return;
    if (aiming) stopAiming();
    else {
      aimStartedAt = performance.now();
      aiming = true;
      aimBtn?.classList.add('active');
    }
  });

  const jumpBtn = document.getElementById('btn-touch-jump');
  bindHold(jumpBtn, () => {
    if (!alive) return false;
    touchJump = true;
    return true;
  }, () => { touchJump = false; });

  const sprintBtn = document.getElementById('btn-touch-sprint');
  bindHold(sprintBtn, () => {
    if (!alive) return false;
    touchSprint = true;
    return true;
  }, () => { touchSprint = false; });

  const crouchBtn = document.getElementById('btn-touch-crouch');
  bindPress(crouchBtn, () => {
    if (!alive) return;
    touchCrouch = !touchCrouch;
    crouchBtn?.classList.toggle('active', touchCrouch);
  });
  bindPress(document.getElementById('btn-touch-reload'), () => startReload());
  bindPress(document.getElementById('btn-touch-nade'), () => {
    if (!alive) return;
    if (activeSlot !== 4) {
      selectSlot(4);
      primeGrenade();
    } else if (grenadePrimed) throwGrenade();
    else primeGrenade();
  });

  document.getElementById('btn-touch-pause')?.addEventListener('click', (e) => {
    e.preventDefault();
    e.stopPropagation();
    clearCombatInput();
    hud.showPause(true);
  });
  document.getElementById('btn-touch-chat')?.addEventListener('click', (e) => {
    e.preventDefault();
    e.stopPropagation();
    if (!joined || hud.isChatComposing()) return;
    clearCombatInput();
    hud.setChatInputOpen(true);
    hud.chatInput.focus();
  });
  document.getElementById('btn-touch-score')?.addEventListener('click', (e) => {
    e.preventDefault();
    e.stopPropagation();
    const sb = document.getElementById('scoreboard');
    const isOpen = sb?.style.display === 'block';
    hud.toggleScoreboard(!isOpen);
    if (!isOpen) {
      refreshScoreboard();
      net.requestRoster();
    }
  });
  for (let i = 1; i <= 4; i++) {
    document.getElementById(`slot-${i}`)?.addEventListener('pointerdown', (e) => {
      e.preventDefault();
      e.stopPropagation();
      selectSlot(i);
    });
  }

  const movableIds = ['touch-top-bar', 'touch-joystick-zone', 'btn-touch-fire', 'btn-touch-jump', 'btn-touch-crouch', 'btn-touch-sprint', 'btn-touch-aim', 'btn-touch-reload', 'btn-touch-nade'];
  const offsets: Record<string, [number, number]> = {};
  try {
    const saved = JSON.parse(localStorage.getItem('ps_touch_layout') ?? '{}') as Record<string, unknown>;
    for (const id of movableIds) {
      const pair = saved[id];
      if (Array.isArray(pair) && pair.length === 2 && pair.every(Number.isFinite)) offsets[id] = [Number(pair[0]), Number(pair[1])];
    }
  } catch {}
  const applyTouchLayout = () => {
    for (const id of movableIds) {
      const element = document.getElementById(id);
      const offset = offsets[id];
      if (element) element.style.setProperty('translate', offset ? `${offset[0] * innerWidth}px ${offset[1] * innerHeight}px` : '');
    }
  };
  applyTouchLayout();
  window.addEventListener('resize', applyTouchLayout);

  let editingLayout = false;
  for (const id of movableIds) {
    const element = document.getElementById(id);
    let pointerId: number | null = null;
    let startX = 0;
    let startY = 0;
    let startOffset: [number, number] = [0, 0];
    let startRect: DOMRect | null = null;
    element?.addEventListener('pointerdown', (e) => {
      if (!editingLayout || !isTouchPointer(e)) return;
      e.preventDefault();
      e.stopImmediatePropagation();
      pointerId = e.pointerId;
      startX = e.clientX;
      startY = e.clientY;
      startOffset = offsets[id] ?? [0, 0];
      startRect = element.getBoundingClientRect();
      element.setPointerCapture(e.pointerId);
    }, { capture: true });
    element?.addEventListener('pointermove', (e) => {
      if (e.pointerId !== pointerId || !startRect) return;
      e.preventDefault();
      e.stopImmediatePropagation();
      const dx = Math.max(-startRect.left, Math.min(innerWidth - startRect.right, e.clientX - startX));
      const dy = Math.max(-startRect.top, Math.min(innerHeight - startRect.bottom, e.clientY - startY));
      offsets[id] = [startOffset[0] + dx / innerWidth, startOffset[1] + dy / innerHeight];
      applyTouchLayout();
    }, { capture: true });
    const finishDrag = (e: PointerEvent) => {
      if (e.pointerId !== pointerId) return;
      e.preventDefault();
      e.stopImmediatePropagation();
      pointerId = null;
      localStorage.setItem('ps_touch_layout', JSON.stringify(offsets));
    };
    element?.addEventListener('pointerup', finishDrag, { capture: true });
    element?.addEventListener('pointercancel', finishDrag, { capture: true });
  }
  window.addEventListener('click', (e) => {
    if (editingLayout && (e.target as Element | null)?.closest('#touch-controls') && !(e.target as Element | null)?.closest('#touch-layout-toolbar')) {
      e.preventDefault();
      e.stopImmediatePropagation();
    }
  }, { capture: true });
  const finishLayoutEdit = () => {
    editingLayout = false;
    document.body.classList.remove('touch-layout-edit');
    hud.root.style.display = joined ? 'block' : 'none';
    hud.toggleSettings(true);
  };
  document.getElementById('touch-layout-reset')?.addEventListener('click', () => {
    for (const id of movableIds) delete offsets[id];
    localStorage.removeItem('ps_touch_layout');
    applyTouchLayout();
  });
  document.getElementById('touch-layout-save')?.addEventListener('click', finishLayoutEdit);
  hud.onTouchLayoutEdit = () => {
    clearCombatInput();
    editingLayout = true;
    hud.toggleSettings(false);
    hud.showPause(false);
    hud.root.style.display = 'block';
    document.body.classList.add('touch-device', 'touch-layout-edit');
  };
}
setupTouchControls();

hud.onPauseClick(() => {
  if (joined && !hud.isSettingsOpen()) void captureGame();
});
hud.onFlightToggle = () => {
  if (!joined || !alive) return;
  if (practice) {
    local.flying = !local.flying;
    hud.setFlightState(local.flying, true);
  } else {
    net.toggleFlight();
  }
};

hud.onJoin = (name, primary, secondary, skin, primarySkin, secondarySkin) => {
  myName = name;
  killStreak = 0;
  userPrimaryChoice = primary;
  userSecondaryChoice = secondary;
  userPrimaryWeaponSkin = primarySkin;
  userSecondaryWeaponSkin = secondarySkin;
  const resolved = resolveLoadout(primary, secondary);
  primaryWeapon = resolved.primary;
  secondaryWeapon = resolved.secondary;
  weapons.build(primaryWeapon);
  resetInventory(primaryWeapon, secondaryWeapon);
  activeSlot = lastWeaponSlot = 1;
  grenadePrimed = false;
  mag = slotMags[1];
  reserve = slotReserves[1];
  nades = 1;
  weapons.ammoLocal = mag;
  weapons.group.visible = true;
  refreshWeaponHud();
  guardMatchHistory();
  net.connect(wsUrl, name, primaryWeapon, secondaryWeapon, skin, primarySkin, secondarySkin);
  void captureGame(); // Fullscreen and pointer lock
  audio.init();
};

interface DummyBox { x0: number; y0: number; z0: number; x1: number; y1: number; z1: number; head: boolean }
interface DummyTarget { group: THREE.Group; boxes: DummyBox[]; cx: number; cy: number; cz: number }
const dummies: DummyTarget[] = [];

function enterPractice() {
  myName = '模拟练习';
  practice = true;
  joined = true;
  alive = true;
  killStreak = 0;
  killerId = -1;
  respawnAt = 0;
  pointerUnlockRequested = false;
  if (!world) {
    const map = bundledMap as MapData;
    world = new WorldView(scene, map);
    world.clouds.visible = hud.quality !== 'low';
    hud.setMap(map);
  }
  const map = bundledMap as MapData;
  let spawn: [number, number, number] = map.spawns?.[0] ?? [0, 0, 0];
  if (world && map.spawns) {
    for (const s of map.spawns) {
      if (canOccupy(world.boxes, local.pos.set(s[0], s[1], s[2]), PHYS.standingHeight)) {
        spawn = s;
        break;
      }
    }
  }
  local.pos.set(spawn[0], spawn[1], spawn[2]);
  local.vel.set(0, 0, 0);
  local.onGround = true;
  local.flying = false;
  local.yaw = Math.atan2(local.pos.x, local.pos.z);
  local.pitch = 0;
  const resolved = resolveLoadout(userPrimaryChoice, userSecondaryChoice);
  primaryWeapon = resolved.primary;
  secondaryWeapon = resolved.secondary;
  primaryWeaponSkin = userPrimaryWeaponSkin === 3 ? 0 : userPrimaryWeaponSkin;
  secondaryWeaponSkin = userSecondaryWeaponSkin === 3 ? 0 : userSecondaryWeaponSkin;
  weapons.build(primaryWeapon, primaryWeaponSkin);
  resetInventory(primaryWeapon, secondaryWeapon);
  activeSlot = lastWeaponSlot = 1;
  grenadePrimed = false;
  mag = slotMags[1];
  reserve = slotReserves[1];
  nades = 1;
  weapons.ammoLocal = mag;
  weapons.group.visible = true;
  refreshWeaponHud();
  spawnDummies();
  hud.setPractice();
  hud.setFlightState(local.flying, true);
  guardMatchHistory();
  audio.init();
  if (!document.body.classList.contains('touch-device')) void lockPointer();
}

hud.onPractice = () => enterPractice();

function spawnDummies() {
  if (dummies.length || !world) return;
  const bodyMat = new THREE.MeshStandardMaterial({ color: 0xc9822b, roughness: 0.85 });
  const headMat = new THREE.MeshStandardMaterial({ color: 0xffce54, roughness: 0.7 });
  const forward = new THREE.Vector3(-Math.sin(local.yaw), 0, -Math.cos(local.yaw)).normalize();
  const right = new THREE.Vector3(-forward.z, 0, forward.x);
  const groundY = 0;
  const probe = new THREE.Vector3();
  const rows: [number, number][] = [[12, 0], [18, 3.5], [12, -3.5], [24, 0], [30, 5]];
  for (const [dist, lat] of rows) {
    const cx = local.pos.x + forward.x * dist + right.x * lat;
    const cz = local.pos.z + forward.z * dist + right.z * lat;
    if (!canOccupy(world.boxes, probe.set(cx, groundY, cz), 0.6)) continue;
    const group = new THREE.Group();
    const boxes: DummyBox[] = [];
    const addBox = (sx: number, sy: number, sz: number, w: number, h: number, d: number, mat: THREE.Material, head: boolean) => {
      const mesh = new THREE.Mesh(new THREE.BoxGeometry(w, h, d), mat);
      mesh.position.set(cx + sx, groundY + sy + h / 2, cz + sz);
      group.add(mesh);
      boxes.push({
        x0: cx + sx - w / 2, x1: cx + sx + w / 2,
        y0: groundY + sy, y1: groundY + sy + h,
        z0: cz + sz - d / 2, z1: cz + sz + d / 2,
        head,
      });
    };
    addBox(-0.18, 0, 0, 0.3, 0.72, 0.3, bodyMat, false);
    addBox(0.18, 0, 0, 0.3, 0.72, 0.3, bodyMat, false);
    addBox(0, 0.72, 0, 0.72, 0.72, 0.36, bodyMat, false);
    addBox(-0.48, 0.78, 0, 0.22, 0.6, 0.22, bodyMat, false);
    addBox(0.48, 0.78, 0, 0.22, 0.6, 0.22, bodyMat, false);
    addBox(0, 1.44, 0, 0.34, 0.34, 0.34, headMat, true);
    scene.add(group);
    dummies.push({ group, boxes, cx, cy: groundY + 1.15, cz });
  }
}

function raycastDummies(origin: THREE.Vector3, dir: THREE.Vector3, maxDist: number): { dist: number; head: boolean } | null {
  let best = maxDist;
  let hit: { dist: number; head: boolean } | null = null;
  for (const t of dummies) {
    for (const b of t.boxes) {
      let tmin = -Infinity;
      let tmax = Infinity;
      if (dir.x !== 0) {
        let t1 = (b.x0 - origin.x) / dir.x;
        let t2 = (b.x1 - origin.x) / dir.x;
        if (t1 > t2) { const tmp = t1; t1 = t2; t2 = tmp; }
        tmin = Math.max(tmin, t1);
        tmax = Math.min(tmax, t2);
      } else if (origin.x < b.x0 || origin.x > b.x1) continue;
      if (dir.y !== 0) {
        let t1 = (b.y0 - origin.y) / dir.y;
        let t2 = (b.y1 - origin.y) / dir.y;
        if (t1 > t2) { const tmp = t1; t1 = t2; t2 = tmp; }
        tmin = Math.max(tmin, t1);
        tmax = Math.min(tmax, t2);
      } else if (origin.y < b.y0 || origin.y > b.y1) continue;
      if (dir.z !== 0) {
        let t1 = (b.z0 - origin.z) / dir.z;
        let t2 = (b.z1 - origin.z) / dir.z;
        if (t1 > t2) { const tmp = t1; t1 = t2; t2 = tmp; }
        tmin = Math.max(tmin, t1);
        tmax = Math.min(tmax, t2);
      } else if (origin.z < b.z0 || origin.z > b.z1) continue;
      if (tmin <= tmax && tmin >= 0 && tmin < best) {
        best = tmin;
        hit = { dist: tmin, head: b.head };
      }
    }
  }
  return hit;
}

function removeDummies() {
  for (const t of dummies) {
    scene.remove(t.group);
    t.group.traverse((o) => {
      if (o instanceof THREE.Mesh) {
        o.geometry.dispose();
        const m = o.material;
        if (Array.isArray(m)) m.forEach((mm) => mm.dispose());
        else m.dispose();
      }
    });
  }
  dummies.length = 0;
}

hud.onLoadoutChange = (p, s) => {
  userPrimaryChoice = p;
  userSecondaryChoice = s;
  const next = resolveLoadout(p, s);
  if (!practice) net.setLoadout(next.primary, next.secondary, userPrimaryWeaponSkin, userSecondaryWeaponSkin);
};
hud.onExit = () => {
  joined = false;
  practice = false;
  alive = false;
  respawnAt = 0;
  pointerUnlockRequested = true;
  releaseMatchHistory();
  clearCombatInput();
  document.exitPointerLock?.();
  if (document.fullscreenElement) void document.exitFullscreen();
  net.disconnect();
  hud.exitMatch();
  for (const id of [...remotes.ids()]) remotes.remove(id);
  removeDummies();
  particles.clearBulletHoles();
  states.clear();
  roster.clear();
  names.clear();
  myAllyBotId = -1;
  myBondMateId = -1;
  hud.setBondMate(null);
};

hud.onSettingsClose = () => {
  if (joined && alive) void captureGame();
};

net.onWelcome = (id, _revision) => {
  if (joined) {
    for (const playerId of [...remotes.ids()]) remotes.remove(playerId);
    states.clear();
    roster.clear();
    names.clear();
    myAllyBotId = -1;
    myBondMateId = -1;
    hud.setBondMate(null);
  }
  for (const pickupId of pickupStates.keys()) world?.removePickup(pickupId);
  pickupStates.clear();
  world?.clearChickens();
  joined = true;
  lastSentInputKeys = -1;
  hud.hideDisconnect();
  if (!world) {
    const map = bundledMap as MapData;
    world = new WorldView(scene, map);
    world.clouds.visible = hud.quality !== 'low';
    hud.setMap(map);
    for (const [pickupId, pickup] of pickupStates) world.setPickup(pickupId, pickup.kind, ...pickup.origin);
  }
  names.set(id, myName);
};

net.onReject = (reason) => {
  releaseMatchHistory();
  hud.showDisconnect(reason);
};
net.onMaintenance = (retryAfter) => {
  alive = false;
  respawnAt = 0;
  reloadPendingSlot = 0;
  weapons.cancelReload();
  clearCombatInput();
  hud.showUpgrade(retryAfter);
};
net.onDisconnect = (upgrading) => {
  if (!joined) releaseMatchHistory();
  alive = false;
  respawnAt = 0;
  reloadPendingSlot = 0;
  weapons.cancelReload();
  clearCombatInput();
  myAllyBotId = -1;
  myBondMateId = -1;
  hud.setBondMate(null);
  if (!upgrading) hud.showDisconnect();
};
net.onLatency = (ms, outboundBps) => {
  latencyMs = ms;
  serverOutboundBps = outboundBps;
  hud.setNetworkStats(latencyMs, serverOutboundBps);
};

net.onSnapshot = (_tick, _ack, updates) => {
  const now = performance.now();
  for (const p of updates) {
    states.set(p.id, p);
    if (p.id === net.yourId) {
      const wasAlive = alive;
      alive = !!(p.state & 1);
      local.flying = alive && !!(p.state & 32);
      hud.setFlightState(local.flying, alive);
      if (wasAlive && !alive) {
        if (!respawnAt) respawnAt = now + 3000;
        weapons.group.visible = false;
      }
      const beforeX = local.pos.x, beforeY = local.pos.y, beforeZ = local.pos.z;
      local.reconcile(p.x, p.y, p.z, p.vx, p.vz, latencyMs, world?.boxes, !!(p.state & 8));
      cameraCorrection.x += beforeX - local.pos.x;
      cameraCorrection.y += beforeY - local.pos.y;
      cameraCorrection.z += beforeZ - local.pos.z;
      if (cameraCorrection.lengthSq() > 0.1225) cameraCorrection.setLength(0.35);
      hud.setHp(p.hp);
      hud.setArmor(p.armor);
      hud.setSpawnShield(!!(p.state & 2));
      // 黑梦只影响其他真人玩家；发起者自己视线不变暗。
      if (p.id !== net.yourId && p.ultimate === 1) {
        blackDreamUntil = performance.now() + 10000;
      } else if (p.id !== net.yourId && p.ultimate !== 1) {
        blackDreamUntil = 0;
      }
      continue;
    }
    remotes.sample(p, now);
  }
};

net.onSelf = (s) => {
  nades = s.nades;
  ultimatePoints = s.ultimatePoints;
  ultimateKind = s.ultimate;
  hud.setUltimate(ultimatePoints, ultimateKind);
  if (s.slot === 1) primaryWeapon = s.weapon;
  else if (s.slot === 2) secondaryWeapon = s.weapon;
  if (s.slot === 1) primaryWeaponSkin = s.weaponSkin;
  else if (s.slot === 2) secondaryWeaponSkin = s.weaponSkin;
  if (s.slot === 1 || s.slot === 2) {
    slotMags[s.slot] = s.mag;
    slotReserves[s.slot] = s.reserve;
  }
  if (s.slot === reloadPendingSlot && s.mag > reloadPendingMag && s.reserve < reloadPendingReserve) reloadPendingSlot = 0;
  if (activeSlot !== 4 && s.slot !== activeSlot) {
    refreshWeaponHud();
    return;
  }
  mag = s.mag;
  reserve = s.reserve;
  if (s.weapon !== weapons.weaponId || s.weaponSkin !== weapons.weaponSkin) {
    weapons.cancelReload();
    weapons.build(s.weapon, s.weaponSkin);
    local.weaponId = s.weapon;
    stopAiming();
  }
  weapons.ammoLocal = mag;
  refreshWeaponHud();
};

net.onRoster = (list) => {
  roster.clear();
  for (const p of list) {
    roster.set(p.id, p);
    names.set(p.id, p.name);
    if (p.id === net.yourId) myName = p.name;
  }
  refreshScoreboard();
};

net.onEvents = (events) => {
  for (const e of events) handleEvent(e);
};

function handleEvent(e: GameEvent) {
  if (e.type === 17) {
    hud.addChatMessage(e.name ?? nameOf(e.player), e.message ?? '', e.player === net.yourId, isBotId(e.player));
    return;
  }
  if (e.type === 14) {
    hud.showRevengeAnnouncement(e.name ?? nameOf(e.player));
    return;
  }
  if (e.type === 15) {
    const kind = e.kind ?? 0;
    if (kind === 4 && e.player === net.yourId) {
      // 非法组队成立：记录 bot 队友（辅助瞄准排除、HUD 高亮）
      myAllyBotId = e.victim ?? -1;
    }
    if (kind === 5 && (e.player === net.yourId || e.victim === net.yourId)) {
      myAllyBotId = -1;
    }
    // 播报显示"对方"的名字：加入事件带 bot（victim）；解散事件取双方中不是自己的一方。
    const otherId = kind === 5
      ? (e.player === net.yourId ? e.victim : e.player)
      : e.victim;
    const who = kind >= 4 ? nameOf(otherId) : (e.name ?? nameOf(e.player));
    // 羁绊事件个性化：说清楚"和谁"羁绊、谁为谁做了什么
    let text: string | undefined;
    if (kind === 3) {
      const mateId = e.player === net.yourId ? e.victim : (e.victim === net.yourId ? e.player : -1);
      if (mateId !== -1 && mateId !== undefined) {
        myBondMateId = mateId;
        hud.setBondMate(stripBotPrefix(nameOf(mateId)));
        text = `💘 你已与 ${nameOf(mateId)} 缔结战场羁绊：Ta 会掩护你、为你复仇`;
      }
    } else if (kind === 6) {
      // 羁绊队友离开：Player=留守者，Victim=离开者
      if (e.player === net.yourId) {
        myBondMateId = -1;
        hud.setBondMate(null);
        text = `💔 你的羁绊队友 ${nameOf(e.victim)} 离开了战场，羁绊解除`;
      }
    } else if (kind === 0) {
      // 掩护：Player 击杀了靠近 Victim 的敌人
      if (e.victim === net.yourId) text = `🛡 羁绊队友 ${who} 替你挡下了敌人 · 羁绊值 ${e.streak ?? 0}`;
      else if (e.player === net.yourId) text = `🛡 你守护了羁绊队友 ${who} · 羁绊值 ${e.streak ?? 0}`;
    } else if (kind === 1) {
      // 复仇：Player 为 Victim 报了仇
      if (e.player === net.yourId) text = `🔥 你为羁绊队友 ${who} 报仇了 · 羁绊值 ${e.streak ?? 0}`;
      else if (e.victim === net.yourId) text = `🔥 羁绊队友 ${who} 为你报仇了 · 羁绊值 ${e.streak ?? 0}`;
    } else if (kind === 2) {
      if (e.player === net.yourId || e.victim === net.yourId) text = `⚡ 你与 ${who} 心有灵犀 · 羁绊值 ${e.streak ?? 0}`;
    }
    hud.showBondEvent(kind, who, e.streak ?? 0, text);
    refreshScoreboard();
    return;
  }
  if (e.type === 0) {
    // EvKill
    const killer = nameOf(e.killer);
    const victim = nameOf(e.victim);
    const mine = e.killer === net.yourId || e.victim === net.yourId;
    hud.killFeedEntry(killer, victim, e.weapon ?? 3, e.headshot === 1, mine, isBotId(e.killer), isBotId(e.victim));
    if (e.killer === net.yourId && e.victim !== net.yourId) {
      // 大招期间不涨点，本地先预估，服务端 SelfState 会同步修正
      if (ultimateKind === 0) {
        ultimatePoints++;
        hud.showKillPoint(ultimatePoints);
        hud.setUltimate(ultimatePoints, ultimateKind);
        if (ultimatePoints >= ULTIMATE_REQUIREMENT) {
          hud.showUltimateReady();
        }
      }
      hud.showKillStreak(++killStreak, e.headshot === 1);
      hud.showKillMedal(e.headshot === 1);
      audio.play(e.headshot === 1 ? 'headshot_kill' : 'kill_confirm', 0.9, 1, 0, true);
    }
    const victimState = states.get(e.victim ?? -1);
    if (victimState) particles.spawnDeath(eventOrigin.set(victimState.x, victimState.y + 0.9, victimState.z), e.headshot === 1);
    else if (e.victim === net.yourId) particles.spawnDeath(eventOrigin.set(local.pos.x, local.pos.y + 0.9, local.pos.z), e.headshot === 1);
    if (e.victim === net.yourId) audio.play('death', 0.65);
    else if (victimState) playSpatial('death', victimState.x, victimState.z, 0.35, 45);
    const k = roster.get(e.killer ?? -1);
    const v = roster.get(e.victim ?? -1);
    if (k) k.kills++;
    if (v) v.deaths++;
    refreshScoreboard();
  if (e.victim === net.yourId) {
      killStreak = 0;
      alive = false;
      weapons.group.visible = false;
      respawnAt = performance.now() + 3000;
      reloadPendingSlot = 0;
      weapons.cancelReload();
      speedBoostUntil = 0;
      recoilBoostUntil = 0;
      ultimatePoints = 0;
      ultimateKind = 0;
      ultimateUntil = 0;
      blackDreamUntil = 0;
      hud.setUltimate(ultimatePoints, ultimateKind);
      local.flying = false;
      hud.setFlightState(false, false);
      clearCombatInput();
      killerId = e.killer ?? -1;
      const next = resolveLoadout(userPrimaryChoice, userSecondaryChoice);
      primaryWeapon = next.primary;
      secondaryWeapon = next.secondary;
      net.setLoadout(next.primary, next.secondary, userPrimaryWeaponSkin, userSecondaryWeaponSkin);
    }
    return;
  }
  if (e.type === 1) {
    // EvHit
    if (e.victim === net.yourId) {
      hud.damageFlash();
      audio.play('hurt', 0.7);
      screenShake = Math.max(screenShake, 0.05);
      const attacker = states.get(e.player ?? -1);
      if (attacker) {
        const dx = attacker.x - local.pos.x;
        const dz = attacker.z - local.pos.z;
        const fx = -Math.sin(local.yaw);
        const fz = -Math.cos(local.yaw);
        const rx = Math.cos(local.yaw);
        const rz = -Math.sin(local.yaw);
        hud.showHurtDir(Math.atan2(dx * rx + dz * rz, dx * fx + dz * fz));
      } else {
        hud.showHurtDir(Math.PI);
      }
    }
    if (e.player === net.yourId) {
      const headshot = e.headshot === 1;
      hud.hitMarker(headshot);
      audio.play(headshot ? 'headshot_ding' : 'hitmarker', headshot ? 1 : 0.65, 1, 0, headshot);
      remotes.flashHit(e.victim ?? -1);
      const hitState = states.get(e.victim ?? -1);
      if (hitState) {
        particles.spawnImpact(eventOrigin.set(hitState.x, hitState.y + (headshot ? 1.65 : 1), hitState.z), impactNormal, headshot ? 0xf2c14e : 0xd8574f, headshot ? 10 : 6);
      }
      if (headshot) screenShake = Math.max(screenShake, 0.014);
    }
    return;
  }
  if (e.type === 2 && e.player === net.yourId && e.origin) {
    // EvRespawn
    respawnAt = 0;
    local.pos.set(...e.origin);
    local.vel.set(0, 0, 0);
    local.onGround = true;
    local.flying = false;
    hud.setFlightState(false, true);
    cameraEyeHeight = PHYS.eyeHeight;
    landingPenaltyUntil = 0;
    speedBoostUntil = 0;
    recoilBoostUntil = 0;
    ultimatePoints = 0;
    ultimateKind = 0;
    ultimateUntil = 0;
    blackDreamUntil = 0;
    hud.setUltimate(ultimatePoints, ultimateKind);
    patternShots = 0;
    reloadPendingSlot = 0;
    weapons.cancelReload();
    resetInventory(primaryWeapon, secondaryWeapon);
    activeSlot = lastWeaponSlot = 1;
    grenadePrimed = false;
    cameraStepOffset = 0;
    cameraCorrection.set(0, 0, 0);
    weapons.group.visible = true;
    refreshWeaponHud();
    return;
  }
  if (e.type === 16 && e.player !== undefined) {
    if ((e.ms ?? 0) > 0) {
      // 有人开启大招，全局广播
      const playerName = e.name ?? nameOf(e.player);
      hud.showUltimateAnnouncement(e.kind ?? 0, playerName);
    }
    if (e.player === net.yourId) {
      if ((e.ms ?? 0) > 0) {
        ultimateKind = e.kind ?? 0;
        ultimateUntil = performance.now() + (e.ms ?? 0);
        // 发起者自己不受黑梦影响。
        blackDreamUntil = 0;
      } else {
        ultimateKind = 0;
        ultimateUntil = 0;
        blackDreamUntil = 0;
      }
      hud.setUltimate(ultimatePoints, ultimateKind);
    } else if ((e.ms ?? 0) > 0 && e.kind === 1) {
      // 其他真人开启黑梦，本地进入被影响状态。
      blackDreamUntil = performance.now() + (e.ms ?? 0);
    } else if (e.kind === 1) {
      // 其他真人的黑梦结束。
      blackDreamUntil = 0;
    }
    return;
  }
  if (e.type === 5) {
    // EvReloadStart
    if (e.player === net.yourId) startReload(performance.now(), false);
    else {
      const s = states.get(e.player ?? -1);
      if (s) playSpatial('reload_click', s.x, s.z, 0.42, 28);
    }
    return;
  }
  if (e.type === 6 && e.player && e.name) {
    // EvPlayerName
    names.set(e.player, e.name);
    return;
  }
  if (e.type === 7 && e.origin) {
    // EvExplosion
    particles.spawnExplosion(eventOrigin.set(...e.origin));
    playSpatial('grenade_explode', e.origin[0], e.origin[2], 0.9, 95);
    const d = Math.hypot(e.origin[0] - local.pos.x, e.origin[2] - local.pos.z);
    if (d < 45) {
      screenShake = Math.max(screenShake, (1 - d / 45) * 0.18);
    }
    return;
  }
  if (e.type === 8 && e.player !== undefined && e.origin && e.dir) {
    // EvNadeThrow
    particles.spawnGrenade(e.player, eventOrigin.set(...e.origin), eventVelocity.set(...e.dir));
    if (e.player !== net.yourId) playSpatial('bolt_rack', e.origin[0], e.origin[2], 0.4, 30);
    return;
  }
  if (e.type === 9 && e.player) {
    // EvPlayerLeave
    if (e.player === myAllyBotId) myAllyBotId = -1;
    if (e.player === myBondMateId) {
      myBondMateId = -1;
      hud.setBondMate(null);
    }
    remotes.remove(e.player);
    states.delete(e.player);
    roster.delete(e.player);
    names.delete(e.player);
    net.forget(e.player);
    refreshScoreboard();
    return;
  }
  if (e.type === 10 && e.pickup !== undefined && e.kind !== undefined && e.origin) {
    pickupStates.set(e.pickup, { kind: e.kind, origin: e.origin });
    world?.setPickup(e.pickup, e.kind, ...e.origin);
    return;
  }
  if (e.type === 11 && e.pickup !== undefined) {
    pickupStates.delete(e.pickup);
    world?.removePickup(e.pickup);
    if (e.victim === net.yourId) {
      if (e.kind === 0) {
        reloadPendingSlot = 0;
        weapons.cancelReload();
        audio.play('mag_in', 0.9);
        hud.showPickupNotice('弹药已补满', 'ammo');
      } else if (e.kind === 1) {
        audio.play('hitmarker', 0.55);
        hud.showPickupNotice('生命恢复', 'health');
      } else if (e.kind === 2) {
        speedBoostUntil = performance.now() + (e.ms ?? 8000);
        audio.play('weapon_switch', 0.7, 1.2);
        hud.showPickupNotice(`速度提升 ${(e.ms ?? 8000) / 1000} 秒`, 'speed');
      }
    }
    return;
  }
  if (e.type === 12 && e.player !== undefined) {
    if (e.kind === 1) hud.showFlightAnnouncement(e.name || nameOf(e.player));
    return;
  }
  if (e.type === 18 && e.chicken !== undefined && e.origin) {
    world?.setChicken(e.chicken, e.origin[0], e.origin[1], e.origin[2], e.dir?.[0] ?? 0, e.dir?.[2] ?? 0);
    return;
  }
  if (e.type === 19 && e.chicken !== undefined) {
    const pos = world?.takeChickenDeath(e.chicken);
    const at = pos ?? e.origin;
    if (at) {
      particles.spawnFeathers(eventOrigin.set(at[0], at[1], at[2]));
      playSpatial('cluck', at[0], at[2], 0.7, 60, 1 + Math.random() * 0.25 - 0.12);
    }
    const killer = roster.get(e.killer ?? -1);
    if (e.killer !== undefined && e.killer !== net.yourId) {
      hud.killFeedEntry(killer ? killer.name : nameOf(e.killer), '战场小鸡', e.weapon ?? 6, false, false);
    } else if (e.killer === net.yourId) {
      hud.killFeedEntry(myName, '战场小鸡', e.weapon ?? 6, false, true);
      audio.play('kill_confirm', 0.7, 1.3, 0, true);
      hud.showPickupNotice('🍗 战场小鸡做成炸鸡:+25 HP', 'health');
    }
    return;
  }
  if (e.type === 13 && e.player === net.yourId) {
    const kind = e.kind ?? 0;
    const ms = e.ms ?? 8000;
    const parts: string[] = [];
    if (kind & 1) {
      reloadPendingSlot = 0;
      weapons.cancelReload();
      parts.push('弹药补充');
    }
    if (kind & 2) parts.push('生命恢复');
    if (kind & 4) {
      speedBoostUntil = performance.now() + ms;
      parts.push('移速提升');
    }
    if (kind & 8) {
      recoilBoostUntil = performance.now() + ms;
      parts.push('后坐降低');
    }
    if (kind & 16) parts.push('伤害提升');
    const label = parts.join(' · ') || '连杀奖励';
    hud.showPickupNotice(label, kind & 1 ? 'ammo' : kind & 2 ? 'health' : kind & 4 ? 'speed' : 'buff');
    audio.play(kind & 2 ? 'hitmarker' : 'weapon_switch', 0.75, 1.15);
    return;
  }
}

hud.onChatSubmit = (text) => {
  if (!practice) net.sendChat(text);
};

function nameOf(id?: number): string {
  const known = id === net.yourId ? myName : names.get(id ?? -1);
  return known?.trim() || `特战队员${id ?? '?'}`;
}

// 服务端快照 bit128 是 bot 的权威标识；名字前缀只兜底旧服务器
function isBotId(id?: number): boolean {
  const s = states.get(id ?? -1);
  if (s) return !!(s.state & 128);
  const n = names.get(id ?? -1) ?? '';
  return n.startsWith('[BOT]') || n.startsWith('bot-');
}

function stripBotPrefix(name: string): string {
  return name.replace(/^\[BOT\]\s*/, '');
}

function refreshScoreboard() {
  hud.updateScoreboard([...roster.values()], states, net.yourId, myAllyBotId, myBondMateId);
}

remotes.onShot = (s, position, burstShots) => {
  if (s.weapon === 6) {
    playSpatial('knife_slash', position.x, position.z, 0.55, 22);
    return;
  }
  if (!world) return;
  const o = remoteShotOrigin.set(position.x, position.y + (s.state & 4 ? PHYS.crouchEye : PHYS.eyeHeight), position.z);
  const shotgun = isShotgun(s.weapon);
  const shotSample = shotgun ? s.shot : ((WEAPONS[s.weapon]?.pellets ?? 1) > 1 ? s.shot * 17 : s.shot);
  const d = shotDirection(remoteShotDir, s.yaw, s.pitch, remoteSpread(s, burstShots), shotSample, s.weapon, s.id, shotgun ? 0 : undefined);
  const dist = world.raycastDistance(o, d, 180, impactNormal);
  weapons.spawnTracer(o, d, dist, false);
  if (dist < 180) {
    impactPoint.copy(o).addScaledVector(d, dist);
    particles.spawnImpact(impactPoint, impactNormal, 0xd6b36e, 3);
    particles.spawnBulletHole(impactPoint, impactNormal);
  }
  playSpatial(fireSound(s.weapon), position.x, position.z, 0.65, 120, 0.97 + Math.random() * 0.06);
};

remotes.onStep = (position) => playSpatial('step', position.x, position.z, 0.16, 26, 0.92);

let prev = performance.now(), lastLabels = 0;
function frame(t: number) {
  requestAnimationFrame(frame);
  const dt = Math.min(0.05, (t - prev) / 1000);
  prev = t;
  if (document.hidden) return;

  if (joined && alive) {
    local.speedMultiplier = (t < speedBoostUntil ? SPEED_BOOST_MULTIPLIER : 1) * (ultimateKind === 3 && t < ultimateUntil ? 1.45 : 1);
    local.keys = keyMask();
    let remaining = dt;
    let landed = false;
    let impactSpeed = 0;
    while (remaining > 0.00001) {
      const step = Math.min(1 / 60, remaining);
      const wasGrounded = local.onGround;
      const feetY = local.pos.y;
      const canStand = !local.crouch || !!(local.keys & KEY.Crouch) || !world || canOccupy(world.boxes, local.pos, PHYS.standingHeight);
      local.update(step, canStand);
      const fallSpeed = local.vel.y;
      let grounded = false;
      if (world) grounded = moveAABB(local.pos, local.vel, step, world.boxes, local.height(), local.onGround);
      else local.pos.addScaledVector(local.vel, step);
      if (local.flying) {
        const halfX = mapSize[0] / 2 - 0.3;
        const halfZ = mapSize[1] / 2 - 0.3;
        local.pos.x = Math.max(-halfX, Math.min(halfX, local.pos.x));
        local.pos.z = Math.max(-halfZ, Math.min(halfZ, local.pos.z));
        local.pos.y = Math.max(0, Math.min(PHYS.maxFlightHeight, local.pos.y));
        if (local.pos.y === 0 && local.vel.y < 0 || local.pos.y === PHYS.maxFlightHeight && local.vel.y > 0) local.vel.y = 0;
      }
      if (local.pos.y < 0) {
        local.pos.y = 0;
        local.vel.y = 0;
        grounded = true;
      }
      local.onGround = grounded;
      if (wasGrounded && grounded && local.pos.y > feetY + 0.01) {
        cameraStepOffset = Math.max(-0.55, cameraStepOffset - (local.pos.y - feetY));
      }
      if (!wasGrounded && grounded) {
        landed = true;
        impactSpeed = Math.min(impactSpeed, fallSpeed);
      }
      remaining -= step;
    }
    if (landed) landingPenaltyUntil = t + 140;
    if (landed && impactSpeed < -5.5) {
      landingKick = Math.min(0.04, -impactSpeed * 0.004);
      audio.play('step', 0.22, 0.82);
    }

    const moveSpeed = Math.hypot(local.vel.x, local.vel.z);
    const moving = moveSpeed > 0.15;
    const stepInterval = Math.max(340, 460 - moveSpeed * 11);
    if (moveSpeed > 0.8 && local.onGround && !local.crouch && t - lastStep > stepInterval) {
      lastStep = t;
      audio.play('step', 0.18, 0.92 + Math.random() * 0.16);
    }

    if (awpRescopeAt && t >= awpRescopeAt && isSniper(weapons.weaponId) && reloadPendingSlot !== activeSlot && !weapons.isReloading(t)) {
      awpRescopeAt = 0;
      aiming = true;
      aimStartedAt = t;
    }
    const inputKeys = local.keys | (aiming ? KEY.Aim : 0);
    if (t - lastInput >= INPUT_INTERVAL && (inputKeys !== lastSentInputKeys || local.yaw !== lastSentInputYaw || local.pitch !== lastSentInputPitch)) {
      lastInput = t - ((t - lastInput) % INPUT_INTERVAL);
      if (!practice) {
        lastSentInputKeys = inputKeys;
        lastSentInputYaw = local.yaw;
        lastSentInputPitch = local.pitch;
        net.sendInput(inputSeq++, inputKeys, local.yaw, local.pitch);
      }
    }

    // 移动端辅助瞄准：锁敌并在开火时磁吸视角（触屏设备生效）。
    updateAimAssist(dt);

    const shouldFire = activeSlot !== 4 && (firePressed || !!WEAPONS[weapons.weaponId]?.automatic && fireHeld);
    if (weapons.weaponId === 6 && knifeHeavyQueued) {
      if (weapons.canFire(t)) {
        fire(1, t);
        knifeHeavyQueued = false;
      }
    } else if (shouldFire) {
      if (reloadPendingSlot !== activeSlot && weapons.canFire(t)) {
        fire(0, t);
      } else if (firePressed && reloadPendingSlot !== activeSlot && weapons.ammoLocal === 0 && !weapons.isReloading(t) && t >= weapons.nextFireAt && weapons.weaponId !== 6) {
        if (reserve > 0) {
          startReload(t, true);
        } else {
          audio.play('empty_click', 0.65);
          weapons.nextFireAt = t + 280;
        }
      }
    }
    firePressed = false;
    landingKick *= Math.exp(-dt * 14);
    cameraStepOffset *= Math.exp(-dt * 10);
    cameraCorrection.multiplyScalar(Math.exp(-dt * 12));
    cameraEyeHeight += ((local.crouch ? PHYS.crouchEye : PHYS.eyeHeight) - cameraEyeHeight) * (1 - Math.exp(-dt * 18));
    camera.position.set(
      local.pos.x + cameraCorrection.x,
      local.pos.y + cameraEyeHeight + cameraCorrection.y + cameraStepOffset - landingKick,
      local.pos.z + cameraCorrection.z,
    );
    if (screenShake > 0.0005) {
      const shake = screenShake * 0.14;
      camera.position.x += Math.sin(t * 0.031) * shake;
      camera.position.y += Math.sin(t * 0.047) * shake * 0.42;
      camera.position.z += Math.cos(t * 0.037) * shake;
      screenShake *= Math.exp(-dt * 9);
    } else {
      screenShake = 0;
    }
    camera.rotation.y = local.yaw;
    camera.rotation.x = local.pitch;
    camera.rotation.z = 0;

    const targetFov = aiming ? WEAPONS[weapons.weaponId].adsFov : hud.hipFov;
    const aimRate = isSniper(weapons.weaponId) ? 5.5 : weapons.weaponId === 10 ? 8 : 14;
    const nextFov = camera.fov + (targetFov - camera.fov) * Math.min(1, dt * aimRate);
    if (Math.abs(nextFov - camera.fov) > 0.01) {
      camera.fov = nextFov;
      camera.updateProjectionMatrix();
      crosshairScale = window.innerHeight * 0.5 / Math.tan(camera.fov * Math.PI / 360);
    }

    if (reloadPendingSlot === activeSlot && !weapons.isReloading(t)) {
      const loaded = Math.min(WEAPONS[weapons.weaponId].mag - reloadPendingMag, reloadPendingReserve);
      mag = reloadPendingMag + loaded;
      reserve = reloadPendingReserve - loaded;
      slotMags[activeSlot] = mag;
      slotReserves[activeSlot] = reserve;
      weapons.ammoLocal = mag;
      reloadPendingSlot = 0;
      refreshWeaponHud();
    }

    weapons.animate(t, dt, moving && local.onGround, aiming, mouseX, mouseY, activeSlot !== 4, hud.bobScale);
    mouseX = 0;
    mouseY = 0;

    hud.updateRadar(local.pos.x, local.pos.z, local.yaw, t);
    const localReloading = weapons.isReloading(t);
    const reloadPending = reloadPendingSlot === activeSlot;
    hud.setReloading(activeSlot <= 2 && (localReloading || reloadPending), localReloading ? weapons.getReloadProgress(t) : 1);
  } else if (joined && !alive) {
    camera.rotation.z = 0;
    deathCam();
  }
  hud.setScope(joined && alive && activeSlot !== 4 && isSniper(weapons.weaponId) && weapons.adsProgress > (aiming ? 0.86 : 0.14));

  hud.setDeathCountdown(joined && !alive && respawnAt ? Math.max(0, Math.ceil((respawnAt - t) / 1000)) : -1);
  const spread = joined && alive && activeSlot !== 4 && weapons.weaponId !== 6 ? localSpread(t) : 0;
  const targetCrosshair = Math.tan(spread * Math.PI / 180) * crosshairScale;
  const crosshairResponse = targetCrosshair > displayedCrosshair ? 24 : CROSSHAIR_RECOVERY[weapons.weaponId] ?? 12;
  displayedCrosshair += (targetCrosshair - displayedCrosshair) * (1 - Math.exp(-dt * crosshairResponse));
  hud.setCrosshair(displayedCrosshair);
  updateGrenadePreview(t);

  if (joined) {
    hud.setBlackDream(t < blackDreamUntil);
    remotes.update(t);
    particles.update(dt, t, world);
    world?.animate(t);
    weapons.syncFrom(camera);
    renderer.render(scene, camera);
    if (alive) weapons.renderOverlay(renderer, scene);
    if (t - lastLabels >= 1000 / 30) {
      lastLabels = t;
      remotes.updateLabels(camera, world);
    }
  }
}
requestAnimationFrame(frame);

function fire(mode: number, t: number) {
  const origin = localShotOrigin.set(local.pos.x, local.eyeY(), local.pos.z);
  const autoRescope = isSniper(weapons.weaponId) && aiming && weapons.ammoLocal > 1;
  // 辅助瞄准：把弹道（含服务端判定与曳光）修正到目标方向，最多偏离准星 3.5°。
  let shotYaw = local.yaw;
  let shotPitch = local.pitch;
  if (assistLocked && assistTargetId !== -1 && weapons.weaponId !== 6 && activeSlot !== 4) {
    shotYaw += clampAbs(wrapAngleDelta(assistYaw - local.yaw), AIM_ASSIST_SHOT_SNAP_MAX);
    shotPitch = Math.max(-1.45, Math.min(1.45, shotPitch + clampAbs(assistPitch - local.pitch, AIM_ASSIST_SHOT_SNAP_MAX)));
  }
  if (t - lastPatternShot > 420) patternShots = 0;
  const spread = weapons.weaponId === 6 ? 0 : localSpread(t);
  lastPatternShot = t;
  patternShots++;
  const pellets = Math.max(1, WEAPONS[weapons.weaponId]?.pellets ?? 1);
  const shotSample = (++shotSeq) & 0xff;
  const dir = shotDirection(localShotDir, shotYaw, shotPitch, spread, pellets > 1 ? shotSample * 17 : shotSample, weapons.weaponId, net.yourId);
  weapons.onFired(t, weapons.weaponId === 6 && (mode & 1) !== 0 ? 1000 : undefined);
  if (!practice) net.sendFire(shotSeq, net.lastServerTick, mode | (aiming ? 0x80 : 0), shotYaw, shotPitch);
  mag = weapons.ammoLocal;
  if (isSniper(weapons.weaponId)) {
    stopAiming();
    if (autoRescope) awpRescopeAt = weapons.nextFireAt - scopeSettleMs(weapons.weaponId);
  }
  if (activeSlot === 1 || activeSlot === 2) slotMags[activeSlot] = mag;
  refreshWeaponHud();
  if (weapons.weaponId !== 6) {
    audio.play(fireSound(weapons.weaponId), 1, 0.985 + Math.random() * 0.03, 0, true);
    for (let i = 0; i < pellets; i++) {
      const pelletDir = i === 0 && !isShotgun(weapons.weaponId)
        ? dir
        : shotDirection(localPelletDir, shotYaw, shotPitch, spread, isShotgun(weapons.weaponId) ? shotSample : shotSample * 17 + i, weapons.weaponId, net.yourId, isShotgun(weapons.weaponId) ? i : undefined);
      let dist = world?.raycastDistance(origin, pelletDir, 180, impactNormal) ?? 180;
      const dummy = raycastDummies(origin, pelletDir, dist);
      let impactColor = 0xd6b36e;
      let impactCount = pellets > 1 ? 3 : 5;
      if (dummy) {
        dist = dummy.dist;
        if (i === 0) {
          hud.hitMarker(dummy.head);
          audio.play(dummy.head ? 'headshot_ding' : 'hitmarker', dummy.head ? 1 : 0.65, 1, 0, dummy.head);
        }
        impactColor = dummy.head ? 0xf2c14e : 0xd8574f;
        impactCount = dummy.head ? 7 : 5;
      }
      weapons.spawnTracer(origin, pelletDir, dist, true);
      if (dist < 180) {
        particles.spawnImpact(impactPoint.copy(origin).addScaledVector(pelletDir, dist), impactNormal, impactColor, impactCount);
        if (!dummy) particles.spawnBulletHole(impactPoint, impactNormal);
      }
    }
    applyRecoil(weapons.weaponId, patternShots, aiming);
  } else {
    const heavy = (mode & 1) !== 0;
    weapons.onKnifeSlash(heavy);
    audio.play('knife_slash', heavy ? 1 : 0.85, heavy ? 0.72 : 1);
    const dist = world?.raycastDistance(origin, dir, 2.0) ?? 2.0;
    const dummy = raycastDummies(origin, dir, dist);
    if (dummy) {
      hud.hitMarker(false);
      audio.play('hitmarker', 0.65, 1, 0, false);
    }
    if (dist < 2.0) {
      particles.spawnImpact(impactPoint.copy(origin).addScaledVector(dir, dist), impactNormal, 0xd6b36e, 3);
    }
  }
  if (mag === 0 && reserve > 0 && weapons.weaponId !== 6) startReload(t);
}

function deathCam() {
  weapons.group.visible = false;
  const k = remotes.posOf(killerId);
  if (!k) return;
  const backDist = 3.5;
  const x = k.position.x - Math.sin(k.yaw) * backDist;
  const y = k.position.y + 1.8;
  const z = k.position.z - Math.cos(k.yaw) * backDist;
  camera.position.lerp(deathTarget.set(x, y, z), 0.1);
  const dx = k.position.x - camera.position.x;
  const dy = k.position.y + 1.2 - camera.position.y;
  const dz = k.position.z - camera.position.z;
  camera.rotation.y = Math.atan2(-dx, -dz);
  camera.rotation.x = Math.atan2(dy, Math.hypot(dx, dz));
}

function weaponSpread(def: WeaponDef, vx: number, vz: number, onGround: boolean, crouching: boolean, landing: boolean, ads: boolean, burstShots: number): number {
  let floor = 0;
  if (isGun(def.id)) {
    floor = 0.28;
    if (isShotgun(def.id)) floor = 1.25;
    else if (isSniper(def.id)) floor = ads ? 0.07 : 0;
    else if (isPistol(def.id)) floor = 0.22;
  }
  const base = Math.max(def.spread, floor);
  const speed = Math.hypot(vx, vz);
  const moveFactor = Math.max(0, Math.min(1, (speed - 0.35) / (3.5 - 0.35)));
  let spread = base + (def.moveSpread - base) * moveFactor;
  spread += Math.min(def.bloom * 1.6, burstShots * def.bloom * 0.18);
  if (crouching) {
    if (onGround && !landing && speed <= 0.35 && burstShots === 0 && !isShotgun(def.id)) return 0;
    spread *= 0.68;
  }
  if (ads && !isSniper(def.id)) spread *= isShotgun(def.id) ? 0.85 : 0.62;
  if (!onGround) spread = Math.max(spread, def.moveSpread * 1.55 + 0.45);
  if (landing) spread = Math.max(spread, def.moveSpread * 1.12);
  return spread;
}

function localSpread(t: number): number {
  const def = WEAPONS[weapons.weaponId];
  const burstShots = t - lastPatternShot <= 420 ? patternShots : 0;
  let spread = weaponSpread(def, local.vel.x, local.vel.z, local.onGround, local.crouch, t < landingPenaltyUntil, aiming, burstShots);
  if (isSniper(weapons.weaponId) && (!aiming || t - aimStartedAt < scopeSettleMs(weapons.weaponId))) spread = Math.max(spread, def.moveSpread);
  return spread;
}

function remoteSpread(s: PlayerSnap, burstShots: number): number {
  const def = WEAPONS[s.weapon];
  let spread = weaponSpread(def, s.vx, s.vz, !!(s.state & 8), !!(s.state & 4), false, !!(s.state & 16), Math.max(0, burstShots - 1));
  if (isSniper(s.weapon) && !(s.state & 16)) spread = Math.max(spread, def.moveSpread);
  return spread;
}

function wrapAngleDelta(d: number): number {
  return Math.atan2(Math.sin(d), Math.cos(d));
}

function clampAbs(v: number, limit: number): number {
  return Math.max(-limit, Math.min(limit, v));
}

interface AssistTarget { id: number; yaw: number; pitch: number }

// 在锁敌锥角内挑选离准星最近的存活敌人；排除出生保护、隐身和非法队友，要求通视。
function pickAimAssistTarget(): AssistTarget | null {
  if (!joined || !alive || activeSlot === 4 || weapons.weaponId === 6) return null;
  if (!document.body.classList.contains('touch-device') || !world) return null;
  const ox = local.pos.x;
  const oy = local.eyeY();
  const oz = local.pos.z;
  const cp = Math.cos(local.pitch);
  const fx = -Math.sin(local.yaw) * cp;
  const fy = Math.sin(local.pitch);
  const fz = -Math.cos(local.yaw) * cp;
  let best: AssistTarget | null = null;
  let bestAngle = Math.min(aiming ? AIM_ASSIST_CONE_ADS : AIM_ASSIST_CONE_HIP, AIM_ASSIST_DROP);
  const consider = (id: number, tx: number, ty: number, tz: number) => {
    const dx = tx - ox, dy = ty - oy, dz = tz - oz;
    const dist = Math.hypot(dx, dy, dz);
    if (dist < 0.8 || dist > AIM_ASSIST_RANGE) return;
    const dot = (dx * fx + dy * fy + dz * fz) / dist;
    const angle = Math.acos(Math.max(-1, Math.min(1, dot)));
    if (angle >= bestAngle) return;
    assistRayDir.set(dx / dist, dy / dist, dz / dist);
    if (world!.raycastDistance(assistRayOrigin.set(ox, oy, oz), assistRayDir, dist - 0.35) < dist - 0.4) return;
    bestAngle = angle;
    best = { id, yaw: Math.atan2(-dx, -dz), pitch: Math.atan2(dy, Math.hypot(dx, dz)) };
  };
  for (const s of states.values()) {
    if (s.id === net.yourId || s.id === myAllyBotId) continue;
    if (!(s.state & 1) || s.state & 2 || s.ultimate === 3) continue;
    consider(s.id, s.x, s.y + (s.state & 4 ? 0.85 : 1.15), s.z);
  }
  for (let i = 0; i < dummies.length; i++) {
    consider(-1 - i, dummies[i].cx, dummies[i].cy, dummies[i].cz);
  }
  return best;
}

// 移动端辅助瞄准：锁敌、开火时按受限速率把视角拉向目标。返回本帧是否处于有效磁吸状态。
function updateAimAssist(dt: number): boolean {
  const target = pickAimAssistTarget();
  hud.setAssistLock(!!target);
  if (!target || !(fireHeld || firePressed)) {
    assistLocked = false;
    assistTargetId = -1;
    return false;
  }
  assistTargetId = target.id;
  assistYaw = target.yaw;
  assistPitch = target.pitch;
  const dYaw = wrapAngleDelta(target.yaw - local.yaw);
  const dPitch = target.pitch - local.pitch;
  if (Math.abs(dYaw) >= AIM_ASSIST_DROP || Math.abs(dPitch) >= AIM_ASSIST_DROP) {
    assistLocked = false;
    return false;
  }
  const pull = aiming ? AIM_ASSIST_PULL_ADS : AIM_ASSIST_PULL_HIP;
  const stepCap = pull * dt;
  const frac = Math.min(1, dt * 9);
  local.yaw = local.yaw + clampAbs(dYaw * frac, stepCap);
  local.pitch = Math.max(-1.45, Math.min(1.45, local.pitch + clampAbs(dPitch * frac, stepCap)));
  assistLocked = true;
  return true;
}

function applyRecoil(weapon: number, shot: number, ads: boolean) {
  const i = Math.max(0, shot - 1);
  let scale = ads ? 0.45 : 1;
  if (performance.now() < recoilBoostUntil) scale *= 0.50;
  if (local.crouch) scale *= 0.78;
  let pitch = 0;
  let yaw = 0;
  switch (weapon) {
    case 0:
      pitch = 0.008 + Math.min(i, 5) * 0.0008;
      yaw = (i % 2 ? 1 : -1) * 0.0012;
      break;
    case 1:
      pitch = 0.030;
      yaw = i % 2 ? 0.0045 : -0.0035;
      break;
    case 2:
      pitch = 0.0055 + Math.min(i, 8) * 0.0005;
      yaw = Math.sin(i * 1.7) * 0.0015;
      break;
    case 3: {
      const akUp = [0.014, 0.015, 0.0165, 0.0175, 0.018, 0.0165, 0.015, 0.0135, 0.012, 0.011, 0.0105, 0.01, 0.0095, 0.009, 0.009, 0.0085];
      const akSide = [0.0003, 0.0007, -0.0005, 0.0016, 0.0022, -0.0010, 0.0028, 0.0018, -0.0034, -0.0040, 0.0036, 0.0044, -0.0042, 0.0032, -0.0046, 0.0038];
      const idx = Math.min(i, akUp.length - 1);
      pitch = akUp[idx];
      yaw = akSide[idx];
      break;
    }
    case 4:
      pitch = 0.014 + Math.min(i, 7) * 0.0018;
      yaw = i < 8 ? (i % 2 ? 0.0024 : -0.0024) : 0.0038;
      break;
    case 5:
      pitch = 0.044;
      break;
    case 7:
      pitch = 0.009;
      yaw = (i % 2 ? 1 : -1) * 0.0010;
      break;
    case 8:
      pitch = 0.011 + Math.min(i, 7) * 0.0014;
      yaw = Math.sin(i * 1.4) * 0.0028;
      break;
    case 9:
      pitch = 0.016 + Math.min(i, 6) * 0.0019;
      yaw = i < 7 ? (i % 2 ? 0.0028 : -0.0024) : 0.0042;
      break;
    case 10:
      pitch = 0.012 + Math.min(i, 6) * 0.0014;
      yaw = i % 2 ? 0.0020 : -0.0018;
      break;
    case 11:
      pitch = 0.034;
      break;
    case 12:
      pitch = 0.016;
      yaw = i % 2 ? 0.003 : -0.0026;
      break;
    default:
      return;
  }
  local.pitch = Math.min(1.45, local.pitch + pitch * scale);
  local.yaw -= yaw * scale;
}

function shotDirection(out: THREE.Vector3, yaw: number, pitch: number, spread: number, shot: number, weapon: number, shooter: number, pellet?: number): THREE.Vector3 {
  const cp = Math.cos(pitch);
  out.set(-Math.sin(yaw) * cp, Math.sin(pitch), -Math.cos(yaw) * cp);
  if (spread <= 0) return out;
  shotRight.set(-out.z, 0, out.x).normalize();
  shotUp.crossVectors(shotRight, out);
  if (isShotgun(weapon) && pellet !== undefined) {
    const o = XM_PELLETS[pellet % XM_PELLETS.length];
    let seed = (Math.imul(shot * 31 + pellet, 747796405) + Math.imul(weapon + 1, 2891336453) + Math.imul(shooter, 2246822519)) >>> 0;
    seed = (Math.imul(seed, 1664525) + 1013904223) >>> 0;
    const jx = (seed / 4294967296 - 0.5) * 0.28;
    seed = (Math.imul(seed, 1664525) + 1013904223) >>> 0;
    const jy = (seed / 4294967296 - 0.5) * 0.28;
    const ring = Math.tan(spread * Math.PI / 180);
    return out.addScaledVector(shotRight, (o[0] + jx) * ring).addScaledVector(shotUp, (o[1] + jy) * ring).normalize();
  }
  let seed = (Math.imul(shot, 747796405) + Math.imul(weapon + 1, 2891336453) + Math.imul(shooter, 2246822519)) >>> 0;
  seed = (Math.imul(seed, 1664525) + 1013904223) >>> 0;
  const radius = Math.sqrt(seed / 4294967296) * Math.tan(spread * Math.PI / 180);
  seed = (Math.imul(seed, 1664525) + 1013904223) >>> 0;
  const angle = seed / 4294967296 * Math.PI * 2;
  return out.addScaledVector(shotRight, Math.cos(angle) * radius).addScaledVector(shotUp, Math.sin(angle) * radius).normalize();
}

function playSpatial(name: SfxName, x: number, z: number, volume: number, maxDistance: number, pitch = 1) {
  const dx = x - local.pos.x;
  const dz = z - local.pos.z;
  const distance = Math.hypot(dx, dz);
  if (distance >= maxDistance) return;
  const attenuation = 1 - distance / maxDistance;
  const pan = distance > 0.01 ? (dx * Math.cos(local.yaw) - dz * Math.sin(local.yaw)) / distance : 0;
  audio.play(name, volume * attenuation, pitch, pan);
}

const sounds: Record<number, SfxName> = {
  0: 'fire_glock',
  1: 'fire_deagle',
  2: 'fire_mp5',
  3: 'fire_ak47',
  4: 'fire_m4a4',
  5: 'fire_awp',
  7: 'fire_usp',
  8: 'fire_ump',
  9: 'fire_famas',
  10: 'fire_aug',
  11: 'fire_scout',
  12: 'fire_xm',
};

function fireSound(id: number): SfxName {
  return sounds[id] ?? 'fire_ak47';
}
