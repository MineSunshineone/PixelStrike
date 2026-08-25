import * as THREE from 'three';
import { getMCSkin, normalizeSkin, SKIN_NAMES } from './skins.js';
import { createPlayerArmGeometry, createPlayerHeadGeometry, createPlayerLegGeometry, createPlayerTorsoGeometry, STANDING_VISUAL_OFFSET, STANDING_VISUAL_SCALE } from './player.js';
import { assembleViewWeapon } from './viewmodels.js';
import { disposeObject, mergeMeshesByMaterial } from './weapons.js';

export class CharacterPreview {
  private canvas: HTMLCanvasElement;
  private renderer: THREE.WebGLRenderer | null = null;
  private scene = new THREE.Scene();
  private camera: THREE.PerspectiveCamera;
  private charGroup = new THREE.Group();
  private weaponGroup = new THREE.Group();

  private headMesh!: THREE.Mesh;
  private bodyMesh!: THREE.Mesh;
  private armRMesh!: THREE.Mesh;
  private armLMesh!: THREE.Mesh;
  private legRMesh!: THREE.Mesh;
  private legLMesh!: THREE.Mesh;

  private headMat!: THREE.MeshLambertMaterial;
  private bodyMat!: THREE.MeshLambertMaterial;
  private armRMat!: THREE.MeshLambertMaterial;
  private armLMat!: THREE.MeshLambertMaterial;
  private legRMat!: THREE.MeshLambertMaterial;
  private legLMat!: THREE.MeshLambertMaterial;

  private activeSkin = 0;
  private activeWeapon = -1;
  private rotationY = 0.35;
  private targetRotationY = 0.35;
  private dragging = false;
  private lastMouseX = 0;
  private running = true;
  private lastFrame = 0;
  private animT = 0;

  constructor(canvas: HTMLCanvasElement) {
    this.canvas = canvas;
    this.camera = new THREE.PerspectiveCamera(40, 1, 0.1, 50);
    this.camera.position.set(0, 1.05, 2.95);
    this.camera.lookAt(0, 0.95, 0);
    this.createRenderer();

    const hemi = new THREE.HemisphereLight(0xfff6ea, 0x5a6270, 1.4);
    const key = new THREE.DirectionalLight(0xfff5e8, 2.2);
    key.position.set(2.5, 3.5, 3.5);
    const fill = new THREE.DirectionalLight(0xa0c8f8, 1.2);
    fill.position.set(-3, 2, 1.5);
    const frontFill = new THREE.DirectionalLight(0xfffaee, 1.4);
    frontFill.position.set(0, 1.0, 3.2);
    const rim = new THREE.DirectionalLight(0xffd080, 1.2);
    rim.position.set(0, 3, -3);
    this.scene.add(hemi, key, fill, frontFill, rim);

    this.buildCharacterMesh();
    this.charGroup.add(this.weaponGroup);
    this.scene.add(this.charGroup);

    const savedSkin = parseInt(localStorage.getItem('pixel_strike_skin') || '0', 10);
    this.setSkin(isNaN(savedSkin) ? Math.floor(Math.random() * SKIN_NAMES.length) : savedSkin);
    this.setWeapon(3);

    this.setupInteractions();
    requestAnimationFrame(this.renderLoop);
  }

  private createRenderer() {
    const rect = this.canvas.getBoundingClientRect();
    const width = Math.max(280, rect.width || 320);
    const height = Math.max(380, rect.height || 420);
    const renderer = this.renderer ?? new THREE.WebGLRenderer({
      canvas: this.canvas,
      alpha: true,
      antialias: false,
      powerPreference: 'high-performance',
    });
    if (this.renderer) renderer.forceContextRestore();
    renderer.setPixelRatio(Math.min(window.devicePixelRatio || 1, 1.5));
    renderer.setSize(width, height, false);
    renderer.toneMapping = THREE.ACESFilmicToneMapping;
    renderer.toneMappingExposure = 1.25;
    this.camera.aspect = width / height;
    this.camera.updateProjectionMatrix();
    this.renderer = renderer;
  }

  private releaseRenderer() {
    if (!this.renderer) return;
    this.renderer.forceContextLoss();
    this.canvas.width = 1;
    this.canvas.height = 1;
  }

  private buildCharacterMesh() {
    this.headMat = new THREE.MeshLambertMaterial();
    this.bodyMat = new THREE.MeshLambertMaterial();
    this.armRMat = new THREE.MeshLambertMaterial();
    this.armLMat = new THREE.MeshLambertMaterial();
    this.legRMat = new THREE.MeshLambertMaterial();
    this.legLMat = new THREE.MeshLambertMaterial();

    this.headMesh = new THREE.Mesh(createPlayerHeadGeometry(), this.headMat);
    this.bodyMesh = new THREE.Mesh(createPlayerTorsoGeometry(), this.bodyMat);
    this.armRMesh = new THREE.Mesh(createPlayerArmGeometry(), this.armRMat);
    this.armLMesh = new THREE.Mesh(createPlayerArmGeometry(), this.armLMat);
    this.legRMesh = new THREE.Mesh(createPlayerLegGeometry(true), this.legRMat);
    this.legLMesh = new THREE.Mesh(createPlayerLegGeometry(false), this.legLMat);

    for (const mesh of [this.headMesh, this.bodyMesh, this.armRMesh, this.armLMesh, this.legRMesh, this.legLMesh]) {
      mesh.scale.y = STANDING_VISUAL_SCALE;
    }
    this.headMesh.position.set(0, 1.24 * STANDING_VISUAL_SCALE + STANDING_VISUAL_OFFSET, 0);
    this.bodyMesh.position.set(0, 0.94 * STANDING_VISUAL_SCALE + STANDING_VISUAL_OFFSET, 0);
    this.armRMesh.position.set(0.24, 1.2 * STANDING_VISUAL_SCALE + STANDING_VISUAL_OFFSET, 0);
    this.armLMesh.position.set(-0.24, 1.2 * STANDING_VISUAL_SCALE + STANDING_VISUAL_OFFSET, 0);
    this.legRMesh.position.set(0.12, 0.64 * STANDING_VISUAL_SCALE + STANDING_VISUAL_OFFSET, 0);
    this.legLMesh.position.set(-0.12, 0.64 * STANDING_VISUAL_SCALE + STANDING_VISUAL_OFFSET, 0);

    this.charGroup.add(this.headMesh, this.bodyMesh, this.armRMesh, this.armLMesh, this.legRMesh, this.legLMesh);
    this.charGroup.position.set(0, -0.15, 0);
  }

  setSkin(id: number) {
    this.activeSkin = normalizeSkin(id);
    localStorage.setItem('pixel_strike_skin', String(this.activeSkin));
    const textures = getMCSkin(this.activeSkin);
    this.headMat.map = textures.head;
    this.headMat.needsUpdate = true;
    this.bodyMat.map = textures.torso;
    this.bodyMat.needsUpdate = true;
    this.armRMat.map = textures.arm;
    this.armRMat.needsUpdate = true;
    this.armLMat.map = textures.arm;
    this.armLMat.needsUpdate = true;
    this.legRMat.map = textures.leg;
    this.legRMat.needsUpdate = true;
    this.legLMat.map = textures.leg;
    this.legLMat.needsUpdate = true;

    const label = document.getElementById('skin-name-badge');
    if (label) label.textContent = SKIN_NAMES[this.activeSkin];
  }

  getSkin(): number {
    return this.activeSkin;
  }

  randomizeSkin() {
    let next = (this.activeSkin + 1 + Math.floor(Math.random() * (SKIN_NAMES.length - 1))) % SKIN_NAMES.length;
    this.setSkin(next);
  }

  setWeapon(id: number) {
    const nextId = id < 0 ? 3 : id;
    if (nextId === this.activeWeapon) return;
    this.activeWeapon = nextId;
    while (this.weaponGroup.children.length > 0) {
      const child = this.weaponGroup.children[0];
      this.weaponGroup.remove(child);
      disposeObject(child);
    }

    const weapon = assembleViewWeapon(this.activeWeapon, null, null);
    mergeMeshesByMaterial(weapon.root);

    weapon.root.scale.setScalar(0.88);
    if (this.activeWeapon === 6) { // Knife
      weapon.root.position.set(0.22, 0.72, 0.18);
      weapon.root.rotation.set(0.35, 0.25, 0.15);
    } else if (this.activeWeapon === 0 || this.activeWeapon === 1 || this.activeWeapon === 7) { // Pistol
      weapon.root.position.set(0.12, 0.76, 0.24);
      weapon.root.rotation.set(-0.05, -0.20, 0.05);
    } else { // Rifle / Sniper / Shotgun / SMG
      weapon.root.position.set(0.16, 0.74, 0.24);
      weapon.root.rotation.set(-0.20, -0.42, 0.10);
    }
    this.weaponGroup.add(weapon.root);
  }

  private setupInteractions() {
    const onDown = (clientX: number) => {
      this.dragging = true;
      this.lastMouseX = clientX;
    };

    const onMove = (clientX: number) => {
      if (!this.dragging) return;
      const dx = clientX - this.lastMouseX;
      this.lastMouseX = clientX;
      this.targetRotationY += dx * 0.015;
    };

    const onUp = () => {
      this.dragging = false;
    };

    this.canvas.addEventListener('mousedown', (e) => onDown(e.clientX));
    window.addEventListener('mousemove', (e) => onMove(e.clientX));
    window.addEventListener('mouseup', onUp);

    this.canvas.addEventListener('touchstart', (e) => {
      if (e.touches.length > 0) onDown(e.touches[0].clientX);
    }, { passive: true });
    window.addEventListener('touchmove', (e) => {
      if (e.touches.length > 0) onMove(e.touches[0].clientX);
    }, { passive: true });
    window.addEventListener('touchend', onUp);

    const resize = () => {
      const rect = this.canvas.getBoundingClientRect();
      if (rect.width > 0 && rect.height > 0) {
        this.renderer?.setSize(rect.width, rect.height, false);
        this.camera.aspect = rect.width / rect.height;
        this.camera.updateProjectionMatrix();
      }
    };
    window.addEventListener('resize', resize);
  }

  private renderLoop = (now: number) => {
    if (!this.running) return;
    requestAnimationFrame(this.renderLoop);
    if (document.hidden || now - this.lastFrame < 1000 / 30) return;
    this.lastFrame = now;

    this.animT += 0.05;

    // Idle gentle auto-rotation when not dragging
    if (!this.dragging) {
      this.targetRotationY += 0.007;
    }
    this.rotationY += (this.targetRotationY - this.rotationY) * 0.19;
    this.charGroup.rotation.y = this.rotationY;

    // Natural Minecraft Idle Breathing Animation
    const breath = Math.sin(this.animT) * 0.02;
    const nod = Math.sin(this.animT * 0.5) * 0.03;
    const isPistol = this.activeWeapon === 0 || this.activeWeapon === 1 || this.activeWeapon === 7;
    const isKnife = this.activeWeapon === 6;

    this.headMesh.rotation.x = nod;
    this.headMesh.rotation.y = Math.sin(this.animT * 0.3) * 0.05;
    this.bodyMesh.position.y = 0.94 * STANDING_VISUAL_SCALE + STANDING_VISUAL_OFFSET + breath * 0.3;

    if (isKnife) {
      this.armRMesh.rotation.set(0.42 + breath, 0.12, -0.06);
      this.armLMesh.rotation.set(-0.06 + breath, 0, 0.06);
    } else if (isPistol) {
      this.armRMesh.rotation.set(0.58 + breath, -0.18, 0.08);
      this.armLMesh.rotation.set(0.62 + breath, 0.28, -0.12);
    } else {
      this.armRMesh.rotation.set(0.68 + breath, -0.24, 0.12);
      this.armLMesh.rotation.set(0.78 + breath, 0.42, -0.22);
    }
    this.weaponGroup.position.y = breath * 0.3;
    this.renderer?.render(this.scene, this.camera);
  };
  setVisible(visible: boolean) {
    if (!visible) {
      this.running = false;
      this.releaseRenderer();
      return;
    }
    if (this.running) return;
    this.createRenderer();
    this.running = true;
    this.lastFrame = 0;
    requestAnimationFrame(this.renderLoop);
  }

  destroy() {
    this.running = false;
    this.renderer?.dispose();
    this.releaseRenderer();
    this.renderer = null;
  }
}
