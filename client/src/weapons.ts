import * as THREE from 'three';
import { mergeGeometries } from 'three/examples/jsm/utils/BufferGeometryUtils.js';
import { WEAPONS, isPistol, isSniper } from './constants.js';
import type { SfxName } from './audio.js';
import { applyWeaponSkin, assembleViewWeapon, markViewLayer, VIEWMODEL_LAYER } from './viewmodels.js';

interface Shell {
  mesh: THREE.Mesh;
  vx: number;
  vy: number;
  vz: number;
  rotX: number;
  rotY: number;
  born: number;
}

interface Tracer {
  matrix: THREE.Matrix4;
  color: number;
  born: number;
  ox: number;
  oy: number;
  oz: number;
  dx: number;
  dy: number;
  dz: number;
  start: number;
  end: number;
}

interface SoundCue {
  time: number;
  name: SfxName;
  vol?: number;
  pitch?: number;
}

const RELOAD_PITCH: Record<number, number> = { 0: 1.12, 1: 0.82, 2: 1.18, 3: 0.92, 4: 1, 5: 0.72, 7: 1.08, 8: 1.05, 9: 0.95, 10: 0.9, 11: 0.7, 12: 0.85 };
const FIRE_KICK = [0.26, 0.62, 0.18, 0.39, 0.37, 0.82, 0, 0.16, 0.31, 0.4, 0.34, 0.66, 0.34] as const;
const RECOIL_RECOVERY = [18, 11, 22, 13, 15, 8, 18, 20, 17, 14, 16, 10, 16] as const;
function createMuzzleFlashTexture(): THREE.Texture {
  const canvas = new OffscreenCanvas(64, 64);
  const ctx = canvas.getContext('2d')!;
  const glow = ctx.createRadialGradient(32, 32, 1, 32, 32, 31);
  glow.addColorStop(0, 'rgba(255, 255, 238, 1)');
  glow.addColorStop(0.18, 'rgba(255, 226, 122, .95)');
  glow.addColorStop(0.55, 'rgba(255, 151, 45, .38)');
  glow.addColorStop(1, 'rgba(255, 108, 28, 0)');
  ctx.fillStyle = glow;
  ctx.fillRect(0, 0, 64, 64);
  ctx.fillStyle = 'rgba(255, 249, 210, .92)';
  ctx.beginPath();
  for (let i = 0; i < 16; i++) {
    const angle = -Math.PI / 2 + i * Math.PI / 8;
    const radius = i % 2 ? 7 : i % 4 ? 21 : 29;
    const x = 32 + Math.cos(angle) * radius;
    const y = 32 + Math.sin(angle) * radius;
    if (i === 0) ctx.moveTo(x, y); else ctx.lineTo(x, y);
  }
  ctx.closePath();
  ctx.fill();
  const texture = new THREE.CanvasTexture(canvas as unknown as HTMLCanvasElement);
  texture.colorSpace = THREE.SRGBColorSpace;
  return texture;
}

export type SoundCallback = (name: SfxName, volume?: number, pitch?: number) => void;

export class Weapons {
  group = new THREE.Group();
  tracers = new THREE.Group();
  shellsGroup = new THREE.Group();
  vmCamera = new THREE.PerspectiveCamera(54, 1, 0.02, 3);

  private gunMeshes: THREE.Object3D[] = [];
  private handsGroup = new THREE.Group();
  private muzzleFlash = new THREE.Group();
  private muzzleLight = new THREE.PointLight(0xffdf88, 0, 8);
  private ejectionPort = new THREE.Object3D();
  private muzzleUntil = 0;

  // Animated sub-components for dynamic viewmodel reload
  private magazineMesh: THREE.Group | null = null;
  private boltMesh: THREE.Group | null = null;
  private handLGroup = new THREE.Group();
  private handRGroup = new THREE.Group();

  // Audio callback and scheduled sound cues
  public onPlaySound?: SoundCallback;
  private scheduledSounds: SoundCue[] = [];

  // Viewmodel mechanics
  private recoil = 0;
  private bobT = 0;
  private curBobX = 0;
  private curBobY = 0;
  private drawProgress = 1.0;
  private slashProgress = 0;
  private slashHeavy = false;
  private boltCycleStartedAt = 0;
  private boltCycleUntil = 0;
  private shells: Shell[] = [];
  private shellPool: THREE.Mesh[] = [];
  private shellGeo = (() => {
    const caseTube = new THREE.CylinderGeometry(0.012, 0.012, 0.045, 8);
    caseTube.rotateX(Math.PI / 2);
    const rim = new THREE.CylinderGeometry(0.014, 0.014, 0.008, 8);
    rim.rotateX(Math.PI / 2);
    rim.translate(0, 0, 0.024);
    const primer = new THREE.CylinderGeometry(0.006, 0.006, 0.002, 8);
    primer.rotateX(Math.PI / 2);
    primer.translate(0, 0, 0.028);
    const parts = [caseTube, rim, primer];
    const merged = mergeGeometries(parts)!;
    for (const p of parts) p.dispose();
    return merged;
  })();
  private brassMat = new THREE.MeshLambertMaterial({ color: 0xdfb445 });
  private shellSide = new THREE.Vector3();
  private shellQuaternion = new THREE.Quaternion();
  private activeTracers: Tracer[] = [];
  private tracerMatrixPool: THREE.Matrix4[] = [];
  private tracerGeo = new THREE.BoxGeometry(1, 1, 1);
  private tracerMat = new THREE.MeshBasicMaterial({ color: 0xffffff, transparent: true, opacity: 0.82, depthWrite: false, blending: THREE.AdditiveBlending });
  private tracerTarget = new THREE.Vector3();
  private tracerObject = new THREE.Object3D();
  private tracerSide = new THREE.Vector3();
  private tracerColor = new THREE.Color();
  private tracerMesh = new THREE.InstancedMesh(this.tracerGeo, this.tracerMat, 96);

  // Aim Down Sights (ADS)
  adsProgress = 0;

  // Client-side ammo & timing
  nextFireAt = 0;
  ammoLocal = WEAPONS[3].mag;
  reloadStartedAt = 0;
  reloadingUntil = 0;
  weaponId = 3;
  weaponSkin = 0;

  // Idle breath & sway
  swayX = 0;
  swayY = 0;

  constructor(private camera: THREE.PerspectiveCamera, scene: THREE.Scene) {
    this.vmCamera.rotation.order = 'YXZ';
    this.vmCamera.layers.set(VIEWMODEL_LAYER);
    const fill = new THREE.DirectionalLight(0xfffaea, 1.9);
    fill.position.set(0.5, 1.0, 0.8);
    fill.layers.set(VIEWMODEL_LAYER);
    const rim = new THREE.DirectionalLight(0x9ec8f0, 1.15);
    rim.position.set(-0.6, -0.4, 0.5);
    rim.layers.set(VIEWMODEL_LAYER);
    const hemi = new THREE.HemisphereLight(0xfff6ec, 0x606a78, 1.25);
    hemi.layers.set(VIEWMODEL_LAYER);
    this.vmCamera.add(fill, rim, hemi, this.group);
    scene.add(this.vmCamera);
    this.group.scale.setScalar(0.78);
    scene.add(this.tracers);
    scene.add(this.shellsGroup);
    this.tracerMesh.instanceMatrix.setUsage(THREE.DynamicDrawUsage);
    this.tracerMesh.frustumCulled = false;
    this.tracerMesh.count = 0;
    this.tracers.add(this.tracerMesh);

    const flashMat = new THREE.MeshBasicMaterial({
      map: createMuzzleFlashTexture(),
      color: 0xffffff,
      transparent: true,
      opacity: 0.96,
      depthWrite: false,
      blending: THREE.AdditiveBlending,
      side: THREE.DoubleSide,
    });
    const flashGeos = [
      new THREE.PlaneGeometry(0.34, 0.34),
      new THREE.PlaneGeometry(0.34, 0.34).rotateY(Math.PI / 2),
    ];
    const flash = new THREE.Mesh(mergeGeometries(flashGeos)!, flashMat);
    for (const geo of flashGeos) geo.dispose();
    const flashCore = new THREE.Mesh(
      new THREE.SphereGeometry(0.045, 8, 6),
      new THREE.MeshBasicMaterial({ color: 0xfff3b0, transparent: true, opacity: 0.9, depthWrite: false, blending: THREE.AdditiveBlending }),
    );
    this.muzzleFlash.add(flash, flashCore);
    this.muzzleFlash.visible = false;
    this.group.add(this.muzzleFlash);
    this.group.add(this.muzzleLight);
    this.group.add(this.ejectionPort);

    this.handsGroup.add(this.handLGroup);
    this.handsGroup.add(this.handRGroup);
    this.group.add(this.handsGroup);

    this.build(3);
  }

  build(id: number, skin = this.weaponSkin) {
    this.weaponId = id;
    this.weaponSkin = skin;
    this.drawProgress = 0;
    this.slashProgress = 0;
    this.boltCycleStartedAt = 0;
    this.boltCycleUntil = 0;
    this.scheduledSounds = [];
    this.recoil = 0;
    this.adsProgress = 0;
    this.resetMotion();
    this.magazineMesh = null;
    this.boltMesh = null;

    for (const m of this.gunMeshes) {
      this.group.remove(m);
      disposeObject(m);
    }
    this.gunMeshes = [];
    for (const child of [...this.handLGroup.children]) {
      this.handLGroup.remove(child);
      disposeObject(child);
    }
    for (const child of [...this.handRGroup.children]) {
      this.handRGroup.remove(child);
      disposeObject(child);
    }
    this.handLGroup.clear();
    this.handRGroup.clear();
    this.handLGroup.position.set(0, 0, 0);
    this.handLGroup.rotation.set(0, 0, 0);
    this.handRGroup.position.set(0, 0, 0);
    this.handRGroup.rotation.set(0, 0, 0);

    const assembled = assembleViewWeapon(id, this.handLGroup, this.handRGroup);
    applyWeaponSkin(assembled.root, skin);
    this.magazineMesh = assembled.magazine;
    this.boltMesh = assembled.bolt;
    this.muzzleLight.position.copy(assembled.muzzle);
    this.muzzleFlash.position.copy(assembled.muzzle);
    this.ejectionPort.position.set(isPistol(id) ? 0.065 : 0.09, isPistol(id) ? 0.055 : 0.075, -0.08);
    if (this.magazineMesh) mergeMeshesByMaterial(this.magazineMesh);
    if (this.boltMesh) mergeMeshesByMaterial(this.boltMesh);
    mergeMeshesByMaterial(assembled.root, [this.magazineMesh, this.boltMesh]);
    mergeMeshesByMaterial(this.handLGroup);
    mergeMeshesByMaterial(this.handRGroup);
    markViewLayer(assembled.root);
    markViewLayer(this.handLGroup);
    markViewLayer(this.handRGroup);
    markViewLayer(this.muzzleFlash);
    this.muzzleLight.layers.set(VIEWMODEL_LAYER);
    this.gunMeshes.push(assembled.root);
    this.group.add(assembled.root);
  }
  canFire(t: number): boolean {
    if (this.weaponId === 6) {
      return t >= this.nextFireAt;
    }
    return t >= this.nextFireAt && t >= this.reloadingUntil && this.ammoLocal > 0;
  }

  startReload(t: number, reserve = 999): boolean {
    const def = WEAPONS[this.weaponId] ?? WEAPONS[0];
    if (this.weaponId === 6 || reserve <= 0 || this.ammoLocal >= def.mag || this.isReloading(t)) return false;

    this.reloadStartedAt = t;
    this.reloadingUntil = t + def.reloadMs;
    this.nextFireAt = Math.max(this.nextFireAt, this.reloadingUntil);

    // Schedule high-definition synchronized reload sound cues
    const pistolReload = isPistol(this.weaponId);
    const pitch = RELOAD_PITCH[this.weaponId] ?? 1;
    this.scheduledSounds = [
      { time: t + def.reloadMs * 0.18, name: 'mag_out', vol: 0.75, pitch: pitch * 1.04 },
      { time: t + def.reloadMs * 0.54, name: 'mag_in', vol: 0.85, pitch },
      { time: t + def.reloadMs * 0.78, name: pistolReload ? 'reload_click' : 'bolt_cycle', vol: 0.82, pitch: pitch * 0.96 },
    ];

    return true;
  }

  isReloading(t: number): boolean {
    return t < this.reloadingUntil;
  }

  getReloadProgress(t: number): number {
    if (!this.isReloading(t)) return 0;
    const def = WEAPONS[this.weaponId] ?? WEAPONS[0];
    const dur = def.reloadMs || 1800;
    return Math.max(0, Math.min(1, (t - this.reloadStartedAt) / dur));
  }

  cancelReload() {
    this.reloadStartedAt = 0;
    this.reloadingUntil = 0;
    this.scheduledSounds = [];
    this.boltCycleStartedAt = 0;
    this.boltCycleUntil = 0;
    if (this.magazineMesh) {
      this.magazineMesh.position.set(0, 0, 0);
      this.magazineMesh.rotation.set(0, 0, 0);
    }
    if (this.boltMesh) {
      this.boltMesh.position.set(0, 0, 0);
    }
    this.handLGroup.position.set(0, 0, 0);
    this.handRGroup.position.set(0, 0, 0);
    this.handRGroup.rotation.set(0, 0, 0);
  }

  onKnifeSlash(heavy = false) {
    this.slashProgress = 1;
    this.slashHeavy = heavy;
  }

  resetMotion() {
    this.swayX = 0;
    this.swayY = 0;
    this.curBobX = 0;
    this.curBobY = 0;
  }

  onFired(t: number, intervalOverride?: number) {
    const def = WEAPONS[this.weaponId] ?? WEAPONS[0];
    const interval = intervalOverride ?? 60000 / def.rpm;
    this.nextFireAt = this.nextFireAt > 0 && t - this.nextFireAt < interval ? this.nextFireAt + interval : t + interval;
    if (isSniper(this.weaponId)) {
      this.boltCycleStartedAt = t;
      this.boltCycleUntil = this.nextFireAt;
      this.scheduledSounds.push({ time: t + interval * 0.28, name: 'bolt_cycle', vol: 0.9, pitch: RELOAD_PITCH[this.weaponId] ?? 0.72 });
    }
    if (this.weaponId === 6) return;
    this.ammoLocal = Math.max(0, this.ammoLocal - 1);
    const kick = FIRE_KICK[this.weaponId] ?? 0.38;
    this.recoil = Math.min(1.2, this.recoil + kick);
    const flashScale = isSniper(this.weaponId) ? 1.35 : this.weaponId === 12 ? 1.18 : this.weaponId === 2 || this.weaponId === 7 ? 0.58 : isPistol(this.weaponId) ? 0.76 : 0.94;
    this.muzzleFlash.visible = true;
    this.muzzleFlash.rotation.z = Math.random() * Math.PI;
    this.muzzleFlash.scale.setScalar(flashScale * (0.9 + Math.random() * 0.25));
    this.muzzleLight.intensity = (isSniper(this.weaponId) || this.weaponId === 12 ? 4.2 : 2.8) * flashScale;
    this.muzzleUntil = t + (this.weaponId === 2 || this.weaponId === 7 ? 30 : 42);
    this.ejectShell();
  }

  private viewmodelPointToWorld(marker: THREE.Object3D, out: THREE.Vector3, distance: number) {
    this.camera.updateMatrixWorld();
    this.vmCamera.updateMatrixWorld(true);
    marker.getWorldPosition(out).project(this.vmCamera);
    out.z = 0;
    return out.unproject(this.camera).sub(this.camera.position).normalize().multiplyScalar(distance).add(this.camera.position);
  }

  private ejectShell() {
    if (this.shells.length >= 24) return;
    const mesh = this.shellPool.pop() ?? new THREE.Mesh(this.shellGeo, this.brassMat);
    this.viewmodelPointToWorld(this.ejectionPort, mesh.position, 0.42);
    this.ejectionPort.getWorldQuaternion(this.shellQuaternion);
    mesh.quaternion.copy(this.shellQuaternion);
    mesh.scale.setScalar(1);
    this.shellsGroup.add(mesh);

    const side = this.shellSide.set(1, 0.75, 0.15).normalize().applyQuaternion(this.shellQuaternion);
    const speed = 2.0 + Math.random() * 1.5;

    this.shells.push({
      mesh,
      vx: side.x * speed,
      vy: side.y * speed,
      vz: side.z * speed,
      rotX: (Math.random() - 0.5) * 20,
      rotY: (Math.random() - 0.5) * 20,
      born: performance.now(),
    });
  }

  animate(t: number, dt: number, moving: boolean, isAiming: boolean, mouseDeltaX: number, mouseDeltaY: number, equipped = true, bobScale = 1) {
    const reloading = this.isReloading(t);
    const rlProgress = reloading ? this.getReloadProgress(t) : 0;
    const boltProgress = isSniper(this.weaponId) && !reloading && t < this.boltCycleUntil
      ? Math.max(0, Math.min(1, (t - this.boltCycleStartedAt) / (this.boltCycleUntil - this.boltCycleStartedAt)))
      : 1;
    const awpBoltArc = boltProgress < 0.78 ? Math.sin(boltProgress / 0.78 * Math.PI) : 0;

    // Trigger scheduled audio cues exactly synchronized with visual animation
    if (this.scheduledSounds.length > 0) {
      let pending = 0;
      for (const s of this.scheduledSounds) {
        if (t >= s.time) {
          this.onPlaySound?.(s.name, s.vol, s.pitch);
        } else {
          this.scheduledSounds[pending++] = s;
        }
      }
      this.scheduledSounds.length = pending;
    }

    this.group.visible = equipped && !(isSniper(this.weaponId) && isAiming && !reloading);
    const targetAds = isAiming && !reloading ? 1 : 0;
    const adsRate = isSniper(this.weaponId) ? 5.5 : this.weaponId === 10 ? 8 : 14;
    this.adsProgress += (targetAds - this.adsProgress) * Math.min(1, dt * adsRate);

    this.swayX += (-mouseDeltaX * 0.00028 - this.swayX) * Math.min(1, dt * 18);
    this.swayY += (-mouseDeltaY * 0.00028 - this.swayY) * Math.min(1, dt * 18);
    const motionFactor = 1 - this.adsProgress;

    // Smooth weapon draw / deploy transition
    this.drawProgress = Math.min(1, this.drawProgress + dt * 5.5);
    const drawDip = Math.sin((1 - this.drawProgress) * Math.PI * 0.5);

    // Dynamic knife arcs: quick repeated slashes and a slower, heavier right-click swing.
    if (this.slashProgress > 0) {
      this.slashProgress = Math.max(0, this.slashProgress - dt * (this.slashHeavy ? 2.2 : 5.6));
      if (this.slashProgress === 0) this.slashHeavy = false;
    }
    const slashPhase = this.slashProgress > 0 ? Math.sin((1 - this.slashProgress) * Math.PI) * (this.slashHeavy ? 1.45 : 1) : 0;
    // Continuous, jitter-free viewmodel bobbing with target lerp
    if (moving && !isAiming && !reloading) this.bobT += dt * 6.2;
    const bob = Math.max(0, Math.min(1, bobScale));
    const targetBobY = (moving && !isAiming && !reloading ? Math.sin(this.bobT) * 0.0055 : Math.sin(t * 0.0018) * 0.0012) * (1 - this.adsProgress) * bob;
    const targetBobX = (moving && !isAiming && !reloading ? Math.cos(this.bobT * 0.5) * 0.003 : 0) * (1 - this.adsProgress) * bob;
    this.curBobX += (targetBobX - this.curBobX) * Math.min(1, dt * 16);
    this.curBobY += (targetBobY - this.curBobY) * Math.min(1, dt * 16);

    this.recoil *= Math.exp(-dt * (RECOIL_RECOVERY[this.weaponId] ?? 14));

    // Keep reload motion inside the viewmodel-safe area; animate parts, not the whole gun across the camera.
    const rlTilt = reloading ? Math.sin(Math.PI * rlProgress) : 0;
    const rlSeatImpulse = (rlProgress > 0.52 && rlProgress < 0.64) ? Math.sin((rlProgress - 0.52) * Math.PI / 0.12) * 0.026 : 0;
    const rlBoltCycle = (rlProgress > 0.68 && rlProgress < 0.84) ? Math.sin((rlProgress - 0.68) * Math.PI / 0.16) : 0;

    // Precision ADS alignment: sights sit precisely at center reticle without receiver body blocking view
    // Natural ADS position: Gun stays visible in lower third of screen, never blocking central reticle/sightline
    let hipX = 0.28, hipY = -0.26, hipZ = -0.72;
    let adsX = 0.08, adsY = -0.22, adsZ = -0.66;

    switch (this.weaponId) {
      case 0: // Glock-18
        hipX = 0.22; hipY = -0.20; hipZ = -0.58;
        adsX = 0.055; adsY = -0.18; adsZ = -0.52;
        break;
      case 1: // Desert Eagle
        hipX = 0.24; hipY = -0.22; hipZ = -0.60;
        adsX = 0.055; adsY = -0.19; adsZ = -0.54;
        break;
      case 2: // MP5-SD
        hipX = 0.26; hipY = -0.24; hipZ = -0.68;
        adsX = 0.07; adsY = -0.21; adsZ = -0.62;
        break;
      case 3: // AK-47
        hipX = 0.28; hipY = -0.26; hipZ = -0.72;
        adsX = 0.08; adsY = -0.22; adsZ = -0.66;
        break;
      case 4: // M4A4
        hipX = 0.28; hipY = -0.26; hipZ = -0.72;
        adsX = 0.08; adsY = -0.22; adsZ = -0.66;
        break;
      case 5: // AWP
        hipX = 0.30; hipY = -0.28; hipZ = -0.78;
        adsX = 0.08; adsY = -0.24; adsZ = -0.70;
        break;
      case 7: // USP-S
        hipX = 0.22; hipY = -0.20; hipZ = -0.60;
        adsX = 0.055; adsY = -0.18; adsZ = -0.54;
        break;
      case 8: // UMP-45
        hipX = 0.26; hipY = -0.24; hipZ = -0.66;
        adsX = 0.07; adsY = -0.21; adsZ = -0.60;
        break;
      case 9: // FAMAS
        hipX = 0.28; hipY = -0.25; hipZ = -0.70;
        adsX = 0.08; adsY = -0.21; adsZ = -0.64;
        break;
      case 10: // AUG
        hipX = 0.28; hipY = -0.25; hipZ = -0.72;
        adsX = 0.06; adsY = -0.2; adsZ = -0.62;
        break;
      case 11: // SSG 08
        hipX = 0.30; hipY = -0.27; hipZ = -0.76;
        adsX = 0.08; adsY = -0.23; adsZ = -0.68;
        break;
      case 12: // XM1014
        hipX = 0.27; hipY = -0.24; hipZ = -0.68;
        adsX = 0.075; adsY = -0.21; adsZ = -0.62;
        break;
      default: // Knife
        hipX = adsX = 0.26; hipY = adsY = -0.22; hipZ = adsZ = -0.58;
        break;
    }

    const posX = hipX + (adsX - hipX) * this.adsProgress + this.curBobX + this.swayX * motionFactor - rlTilt * 0.06 + slashPhase * 0.11;
    const posY = hipY + (adsY - hipY) * this.adsProgress + this.curBobY + this.swayY * motionFactor - rlTilt * 0.05 - rlSeatImpulse - drawDip * 0.12 + Math.sin(slashPhase * Math.PI) * 0.045;
    const posZ = hipZ + (adsZ - hipZ) * this.adsProgress + this.recoil * 0.045 + rlSeatImpulse * 0.3 - drawDip * 0.06 - slashPhase * 0.07;
    this.group.position.set(posX, posY, posZ);

    this.group.rotation.x = this.recoil * 0.08 + rlTilt * 0.16 - this.swayY * motionFactor + drawDip * 0.22 - slashPhase * 0.32 + rlBoltCycle * 0.10;
    this.group.rotation.y = -this.recoil * 0.015 + this.swayX * motionFactor + rlTilt * 0.10 + slashPhase * 0.48;
    this.group.rotation.z = -rlTilt * 0.30 - slashPhase * 0.20;
    if (this.magazineMesh) {
      if (reloading && rlProgress < 0.32) {
        const p = rlProgress / 0.32;
        this.magazineMesh.position.set(0, -p * p * 0.34, p * 0.04);
      } else if (reloading && rlProgress < 0.56) {
        const p = (rlProgress - 0.32) / 0.24;
        this.magazineMesh.position.set(0, -(1 - p) * (1 - p) * 0.34, (1 - p) * 0.04);
      } else {
        this.magazineMesh.position.set(0, 0, 0);
      }
      this.magazineMesh.rotation.set(0, 0, 0);
    }
    if (this.boltMesh) {
      const fireBlowback = Math.max(0, this.recoil) * 0.04;
      this.boltMesh.position.set(0, 0, fireBlowback + rlBoltCycle * 0.065 + awpBoltArc * 0.22);
      this.boltMesh.rotation.z = isSniper(this.weaponId) ? -awpBoltArc * 0.65 : 0;
    }
    if (reloading) {
      if (rlProgress < 0.32) {
        const p = rlProgress / 0.32;
        this.handLGroup.position.set(0, -p * 0.20, p * 0.04);
      } else if (rlProgress < 0.56) {
        const p = (rlProgress - 0.32) / 0.24;
        this.handLGroup.position.set(0, -(1 - p) * 0.20, (1 - p) * 0.04);
      } else if (rlProgress < 0.84) {
        const reach = Math.sin((rlProgress - 0.56) / 0.28 * Math.PI);
        this.handLGroup.position.set(-reach * 0.025, reach * 0.06, -reach * 0.08);
      } else {
        this.handLGroup.position.set(0, 0, 0);
      }
    } else {
      this.handLGroup.position.set(0, 0, 0);
    }
    if (isSniper(this.weaponId) && awpBoltArc > 0) {
      this.handRGroup.position.set(awpBoltArc * 0.07, awpBoltArc * 0.08, awpBoltArc * 0.16);
      this.handRGroup.rotation.z = -awpBoltArc * 0.35;
    } else {
      this.handRGroup.position.set(0, 0, 0);
      this.handRGroup.rotation.set(0, 0, 0);
    }
    if (t > this.muzzleUntil) {
      this.muzzleFlash.visible = false;
      this.muzzleLight.intensity = 0;
    }

    const now = t;
    let aliveShells = 0;
    for (const sh of this.shells) {
      const age = now - sh.born;
      if (age > 1200) {
        this.shellsGroup.remove(sh.mesh);
        this.shellPool.push(sh.mesh);
        continue;
      }
      sh.vy -= 14 * dt;
      sh.mesh.position.x += sh.vx * dt;
      sh.mesh.position.y += sh.vy * dt;
      sh.mesh.position.z += sh.vz * dt;
      sh.mesh.rotation.x += sh.rotX * dt;
      sh.mesh.rotation.y += sh.rotY * dt;
      this.shells[aliveShells++] = sh;
    }
    this.shells.length = aliveShells;

    let aliveTracers = 0;
    for (const tracer of this.activeTracers) {
      const ageMs = now - tracer.born;
      const reachedAt = (tracer.end - tracer.start) / 900 * 1000;
      if (ageMs > reachedAt + 45) {
        this.tracerMatrixPool.push(tracer.matrix);
        continue;
      }
      const head = Math.min(tracer.end, tracer.start + ageMs * 0.9);
      const tail = Math.max(tracer.start, head - 0.55);
      const middle = (head + tail) * 0.5;
      this.tracerObject.position.set(tracer.ox + tracer.dx * middle, tracer.oy + tracer.dy * middle, tracer.oz + tracer.dz * middle);
      this.tracerObject.scale.set(0.035, 0.035, head - tail);
      this.tracerObject.lookAt(this.tracerTarget.set(tracer.ox + tracer.dx * head, tracer.oy + tracer.dy * head, tracer.oz + tracer.dz * head));
      this.tracerObject.updateMatrix();
      tracer.matrix.copy(this.tracerObject.matrix);
      this.tracerMesh.setMatrixAt(aliveTracers, tracer.matrix);
      this.tracerMesh.setColorAt(aliveTracers, this.tracerColor.setHex(tracer.color));
      this.activeTracers[aliveTracers++] = tracer;
    }
    this.activeTracers.length = aliveTracers;
    this.tracerMesh.count = aliveTracers;
    if (aliveTracers > 0) this.tracerMesh.instanceMatrix.needsUpdate = true;
    if (this.tracerMesh.instanceColor) this.tracerMesh.instanceColor.needsUpdate = true;
  }

  spawnTracer(origin: THREE.Vector3, dir: THREE.Vector3, dist: number, local = true) {
    if (this.activeTracers.length >= 96) return;
    let tracerDir = dir;
    if (local) {
      this.tracerTarget.copy(origin).addScaledVector(dir, dist);
      this.viewmodelPointToWorld(this.muzzleFlash, this.tracerObject.position, 0.45);
      tracerDir = this.tracerSide.subVectors(this.tracerTarget, this.tracerObject.position);
      dist = tracerDir.length();
      if (dist <= 0.1) return;
      tracerDir.multiplyScalar(1 / dist);
    } else {
      this.tracerObject.position.copy(origin);
    }
    const start = local ? 0.02 : 0.35;
    if (dist - start <= 0.1) return;
    const matrix = this.tracerMatrixPool.pop() ?? new THREE.Matrix4();
    this.activeTracers.push({
      matrix,
      color: local ? 0xffd36a : 0xff8a35,
      born: performance.now(),
      ox: this.tracerObject.position.x,
      oy: this.tracerObject.position.y,
      oz: this.tracerObject.position.z,
      dx: tracerDir.x,
      dy: tracerDir.y,
      dz: tracerDir.z,
      start,
      end: dist,
    });
  }

  setAspect(aspect: number) {
    this.vmCamera.aspect = aspect;
    this.vmCamera.updateProjectionMatrix();
  }

  syncFrom(camera: THREE.PerspectiveCamera) {
    this.vmCamera.position.copy(camera.position);
    this.vmCamera.quaternion.copy(camera.quaternion);
    if (this.vmCamera.aspect !== camera.aspect) this.setAspect(camera.aspect);
  }

  renderOverlay(renderer: THREE.WebGLRenderer, scene: THREE.Scene) {
    if (!this.group.visible) return;
    const background = scene.background;
    const fog = scene.fog;
    scene.background = null;
    scene.fog = null;
    const prevAutoClear = renderer.autoClear;
    renderer.autoClear = false;
    renderer.clear(false, true, false);
    renderer.render(scene, this.vmCamera);
    renderer.autoClear = prevAutoClear;
    scene.background = background;
    scene.fog = fog;
  }
}

export function mergeMeshesByMaterial(root: THREE.Object3D, excludedRoots: (THREE.Object3D | null)[] = []) {
  const excluded = new Set<THREE.Object3D>();
  for (const excludedRoot of excludedRoots) excludedRoot?.traverse((obj) => excluded.add(obj));
  root.updateWorldMatrix(true, true);
  const inverseRoot = new THREE.Matrix4().copy(root.matrixWorld).invert();
  const transform = new THREE.Matrix4();
  const groups = new Map<THREE.Material, { mesh: THREE.Mesh; geometry: THREE.BufferGeometry }[]>();
  root.traverse((obj) => {
    if (!(obj instanceof THREE.Mesh) || excluded.has(obj) || Array.isArray(obj.material)) return;
    const geometry = obj.geometry.clone();
    geometry.applyMatrix4(transform.multiplyMatrices(inverseRoot, obj.matrixWorld));
    const entries = groups.get(obj.material) ?? [];
    entries.push({ mesh: obj, geometry });
    groups.set(obj.material, entries);
  });
  for (const [material, entries] of groups) {
    if (entries.length < 2) {
      entries[0].geometry.dispose();
      continue;
    }
    const merged = mergeGeometries(entries.map((entry) => entry.geometry), false);
    for (const entry of entries) entry.geometry.dispose();
    if (!merged) continue;
    for (const entry of entries) {
      entry.mesh.parent?.remove(entry.mesh);
      entry.mesh.geometry.dispose();
    }
    root.add(new THREE.Mesh(merged, material));
  }
}

export function disposeObject(root: THREE.Object3D) {
  root.traverse((obj) => {
    if (!(obj instanceof THREE.Mesh)) return;
    obj.geometry.dispose();
    const materials = Array.isArray(obj.material) ? obj.material : [obj.material];
    for (const material of materials) material.dispose();
  });
}
