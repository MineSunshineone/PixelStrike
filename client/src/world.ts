import * as THREE from 'three';
import { mergeGeometries } from 'three/examples/jsm/utils/BufferGeometryUtils.js';
import type { MapData } from './constants.js';

// ---- 128x128 High-Definition Procedural Voxel Textures ----

function createHDVoxelTexture(type: number): THREE.Texture {
  const canvas = new OffscreenCanvas(256, 256);
  const ctx = canvas.getContext('2d')!;
  let seed = type * 137 + 47;
  const rnd = () => (seed = (seed * 16807) % 2147483647) / 2147483647;

  switch (type) {
    case 0: { // 0: Tactical Sandstone / Matte Ground Pavement (Clean, soft warm stone)
      ctx.fillStyle = '#747b73';
      ctx.fillRect(0, 0, 256, 256);

      // Large 2x2 subtle paver slabs with delicate, soft low-contrast seams
      ctx.fillStyle = 'rgba(90, 80, 65, 0.09)';
      ctx.fillRect(0, 127, 256, 2);
      ctx.fillRect(127, 0, 2, 256);

      // Subtle light bevel highlight on pavers
      ctx.fillStyle = 'rgba(255, 255, 255, 0.14)';
      ctx.fillRect(2, 2, 123, 2);
      ctx.fillRect(2, 2, 2, 123);
      ctx.fillRect(130, 2, 123, 2);
      ctx.fillRect(130, 2, 2, 123);
      ctx.fillRect(2, 130, 123, 2);
      ctx.fillRect(2, 130, 2, 123);
      ctx.fillRect(130, 130, 123, 2);
      ctx.fillRect(130, 130, 2, 123);

      // Soft ambient perimeter vignette
      ctx.fillStyle = 'rgba(0, 0, 0, 0.03)';
      ctx.fillRect(0, 0, 256, 3); ctx.fillRect(0, 253, 256, 3);
      ctx.fillRect(0, 0, 3, 256); ctx.fillRect(253, 0, 3, 256);
      break;
    }
    case 1: { // 1: Reinforced Clean Architectural Concrete (Modern matte smooth panels)
      ctx.fillStyle = '#7d888c';
      ctx.fillRect(0, 0, 256, 256);

      // Two large clean architectural panels
      ctx.fillStyle = 'rgba(40, 45, 52, 0.16)';
      ctx.fillRect(0, 127, 256, 2);
      ctx.fillRect(127, 0, 2, 127);
      ctx.fillRect(127, 128, 2, 128);

      // Subtle panel top highlights
      ctx.fillStyle = 'rgba(255, 255, 255, 0.18)';
      ctx.fillRect(0, 0, 256, 2);
      ctx.fillRect(0, 129, 256, 2);

      // Minimalist architectural tie holes
      const tiePoints = [
        [28, 28], [100, 28], [156, 28], [228, 28],
        [28, 100], [100, 100], [156, 100], [228, 100],
        [28, 156], [100, 156], [156, 156], [228, 156],
        [28, 228], [100, 228], [156, 228], [228, 228],
      ];
      for (const [tx, ty] of tiePoints) {
        ctx.fillStyle = 'rgba(35, 40, 48, 0.22)';
        ctx.beginPath();
        ctx.arc(tx, ty, 3, 0, Math.PI * 2);
        ctx.fill();
        ctx.fillStyle = 'rgba(255, 255, 255, 0.25)';
        ctx.beginPath();
        ctx.arc(tx, ty - 1, 1.5, 0, Math.PI * 2);
        ctx.fill();
      }
      break;
    }
    case 2: { // 2: Tactical Military Wooden Crate (Warm teak with sleek corner reinforcements)
      ctx.fillStyle = '#705138';
      ctx.fillRect(0, 0, 256, 256);

      // 3 Smooth horizontal wood planks
      ctx.fillStyle = '#493522';
      ctx.fillRect(0, 84, 256, 4);
      ctx.fillRect(0, 168, 256, 4);

      ctx.fillStyle = 'rgba(255, 255, 255, 0.10)';
      ctx.fillRect(0, 0, 256, 3);
      ctx.fillRect(0, 88, 256, 3);
      ctx.fillRect(0, 172, 256, 3);

      // Dark steel corner brackets
      ctx.fillStyle = '#2b3038';
      const bracketW = 28;
      ctx.fillRect(0, 0, bracketW, bracketW);
      ctx.fillRect(256 - bracketW, 0, bracketW, bracketW);
      ctx.fillRect(0, 256 - bracketW, bracketW, bracketW);
      ctx.fillRect(256 - bracketW, 256 - bracketW, bracketW, bracketW);

      // Steel bracket rivets
      ctx.fillStyle = '#94a0b0';
      const rivets = [
        [14, 14], [256 - 14, 14], [14, 256 - 14], [256 - 14, 256 - 14],
        [128, 10], [128, 246]
      ];
      for (const [rx, ry] of rivets) {
        ctx.fillRect(rx - 2, ry - 2, 5, 5);
      }

      // Minimalist Tactical Stencil (clean chevron emblem in center)
      ctx.fillStyle = 'rgba(245, 190, 80, 0.65)';
      ctx.beginPath();
      ctx.moveTo(128, 108);
      ctx.lineTo(148, 148);
      ctx.lineTo(108, 148);
      ctx.closePath();
      ctx.fill();

      ctx.fillStyle = '#705138';
      ctx.beginPath();
      ctx.moveTo(128, 120);
      ctx.lineTo(140, 144);
      ctx.lineTo(116, 144);
      ctx.closePath();
      ctx.fill();
      break;
    }
    case 3: { // 3: Diamond Steel Plate / Container Slate
      ctx.fillStyle = '#465762';
      ctx.fillRect(0, 0, 256, 256);

      // Clean recessed industrial panel
      ctx.fillStyle = 'rgba(25, 28, 33, 0.35)';
      ctx.fillRect(8, 8, 240, 240);
      ctx.fillStyle = '#3f4f59';
      ctx.fillRect(12, 12, 232, 232);

      // Subtle panel highlight
      ctx.fillStyle = 'rgba(255, 255, 255, 0.15)';
      ctx.fillRect(12, 12, 232, 3);
      ctx.fillRect(12, 12, 3, 232);

      // Clean, low-contrast grip pattern
      ctx.fillStyle = 'rgba(255, 255, 255, 0.08)';
      for (let y = 32; y < 230; y += 32) {
        for (let x = 32; x < 230; x += 32) {
          ctx.fillRect(x, y, 12, 3);
          ctx.fillRect(x + 16, y + 16, 12, 3);
        }
      }
      break;
    }
    case 4: { // 4: Dark Basalt / Architectural Trim & Curbs
      ctx.fillStyle = '#252b30';
      ctx.fillRect(0, 0, 256, 256);

      // 2 Large architectural stone slabs
      ctx.fillStyle = '#161b1f';
      ctx.fillRect(0, 127, 256, 2);

      ctx.fillStyle = 'rgba(255, 255, 255, 0.10)';
      ctx.fillRect(0, 0, 256, 2);
      ctx.fillRect(0, 129, 256, 2);
      break;
    }
    case 5: { // 5: Tactical Foliage / Green Hedge
      ctx.fillStyle = '#3c5d3b';
      ctx.fillRect(0, 0, 256, 256);

      // Soft stylized leaf clusters
      const leafColors = ['#29462d', '#466b43', '#587b50', '#6b8a5d'];
      for (let i = 0; i < 90; i++) {
        const x = Math.floor(rnd() * 240);
        const y = Math.floor(rnd() * 240);
        const r = 8 + Math.floor(rnd() * 16);
        ctx.fillStyle = leafColors[Math.floor(rnd() * leafColors.length)];
        ctx.beginPath();
        ctx.arc(x, y, r, 0, Math.PI * 2);
        ctx.fill();
      }
      break;
    }
    case 6: { // 6: Golden Sandstone / Citadel Course (Clean architectural stone courses)
      ctx.fillStyle = '#9c9078';
      ctx.fillRect(0, 0, 256, 256);

      // Horizontal architectural masonry course with soft shadows
      ctx.fillStyle = '#6e6658';
      ctx.fillRect(0, 126, 256, 4);

      // Elegant top stone bevels
      ctx.fillStyle = '#bbb095';
      ctx.fillRect(0, 0, 256, 4);
      ctx.fillRect(0, 130, 256, 4);

      // Soft warm central architectural recess
      ctx.fillStyle = 'rgba(69, 64, 55, 0.14)';
      ctx.fillRect(16, 16, 224, 94);
      ctx.fillRect(16, 146, 224, 94);
      break;
    }
    case 7: { // 7: Red Terracotta Brick
      ctx.fillStyle = '#875044';
      ctx.fillRect(0, 0, 256, 256);

      // Clean staggered brick joints
      ctx.fillStyle = '#55312f';
      ctx.fillRect(0, 62, 256, 4);
      ctx.fillRect(0, 126, 256, 4);
      ctx.fillRect(0, 190, 256, 4);

      ctx.fillRect(126, 0, 4, 62);
      ctx.fillRect(62, 66, 4, 60);
      ctx.fillRect(190, 66, 4, 60);
      ctx.fillRect(126, 130, 4, 60);
      ctx.fillRect(62, 194, 4, 62);
      ctx.fillRect(190, 194, 4, 62);

      // Brick top highlights
      ctx.fillStyle = 'rgba(255, 255, 255, 0.12)';
      ctx.fillRect(0, 0, 256, 2);
      ctx.fillRect(0, 66, 256, 2);
      ctx.fillRect(0, 130, 256, 2);
      ctx.fillRect(0, 194, 256, 2);
      break;
    }
    case 8: { // 8: Cyber Beacon / Lamp
      ctx.fillStyle = '#22262d';
      ctx.fillRect(0, 0, 256, 256);

      // Luminous warm amber lantern panel
      ctx.fillStyle = '#f59e0b';
      ctx.fillRect(24, 24, 208, 208);

      // Bright glowing center core
      ctx.fillStyle = '#fffbeb';
      ctx.fillRect(64, 64, 128, 128);

      // Sleek protective titanium corner cage
      ctx.fillStyle = '#181b20';
      ctx.fillRect(0, 0, 256, 16); ctx.fillRect(0, 240, 256, 16);
      ctx.fillRect(0, 0, 16, 256); ctx.fillRect(240, 0, 16, 256);
      ctx.fillRect(120, 0, 16, 256); ctx.fillRect(0, 120, 256, 16);
      break;
    }
    case 9: { // 9: Obsidian Carbon / Tech Slate
      ctx.fillStyle = '#20272c';
      ctx.fillRect(0, 0, 256, 256);

      ctx.fillStyle = '#1c1e24';
      ctx.fillRect(6, 6, 244, 244);
      ctx.fillStyle = '#2a2e38';
      ctx.fillRect(10, 10, 236, 236);
      break;
    }
    case 10: { // 10: Tactical Supply Shelf
      ctx.fillStyle = '#374039';
      ctx.fillRect(0, 0, 256, 256);

      // Steel shelf rack
      ctx.fillStyle = '#222620';
      ctx.fillRect(8, 8, 240, 112);
      ctx.fillRect(8, 136, 240, 112);

      // Clean stylized ammo & gear containers
      const crateTints = ['#5a7346', '#c49a56', '#43698f', '#b8544d'];
      for (let i = 0; i < 4; i++) {
        ctx.fillStyle = crateTints[i];
        ctx.fillRect(20 + i * 56, 24, 46, 80);
        ctx.fillRect(20 + i * 56, 152, 46, 80);
        ctx.fillStyle = 'rgba(255,255,255,0.18)';
        ctx.fillRect(20 + i * 56, 24, 46, 4);
        ctx.fillRect(20 + i * 56, 152, 46, 4);
      }
      break;
    }
    case 11: { // 11: Security Glass
      ctx.fillStyle = 'rgba(48, 139, 156, 0.25)';
      ctx.fillRect(0, 0, 256, 256);

      // Subtle edge frame
      ctx.fillStyle = 'rgba(255, 255, 255, 0.55)';
      ctx.fillRect(0, 0, 256, 3);
      ctx.fillRect(0, 253, 256, 3);
      ctx.fillRect(0, 0, 3, 256);
      ctx.fillRect(253, 0, 3, 256);

      // Clean diagonal glint
      ctx.beginPath();
      ctx.strokeStyle = 'rgba(255, 255, 255, 0.35)';
      ctx.lineWidth = 6;
      ctx.moveTo(30, 30); ctx.lineTo(100, 100);
      ctx.moveTo(150, 150); ctx.lineTo(226, 226);
      ctx.stroke();
      break;
    }
    case 12: { // 12: Azure Water
      ctx.fillStyle = '#287b88';
      ctx.fillRect(0, 0, 256, 256);

      // Soft flowing caustics ripples
      ctx.fillStyle = 'rgba(255, 255, 255, 0.25)';
      for (let i = 0; i < 28; i++) {
        const cx = Math.floor(rnd() * 240);
        const cy = Math.floor(rnd() * 240);
        const cw = 16 + Math.floor(rnd() * 36);
        const ch = 8 + Math.floor(rnd() * 18);
        ctx.beginPath();
        ctx.ellipse(cx, cy, cw / 2, ch / 2, rnd() * Math.PI, 0, Math.PI * 2);
        ctx.fill();
      }
      break;
    }
    default: { // 13: Clean Matte Tactical Asphalt Roadway
      ctx.fillStyle = '#30373b';
      ctx.fillRect(0, 0, 256, 256);

      // Subtle asphalt aggregate texture
      ctx.fillStyle = 'rgba(255, 255, 255, 0.04)';
      for (let i = 0; i < 50; i++) {
        const x = Math.floor(rnd() * 254);
        const y = Math.floor(rnd() * 254);
        ctx.fillRect(x, y, 3, 3);
      }
      ctx.fillStyle = 'rgba(0, 0, 0, 0.06)';
      for (let i = 0; i < 50; i++) {
        const x = Math.floor(rnd() * 254);
        const y = Math.floor(rnd() * 254);
        ctx.fillRect(x, y, 3, 3);
      }
      break;
    }
  }
  if (type !== 8 && type !== 11 && type !== 12) {
    for (let i = 0; i < 18; i++) {
      ctx.fillStyle = `rgba(8, 16, 18, ${0.012 + rnd() * 0.025})`;
      ctx.beginPath();
      ctx.ellipse(rnd() * 256, rnd() * 256, 10 + rnd() * 36, 5 + rnd() * 20, rnd() * Math.PI, 0, Math.PI * 2);
      ctx.fill();
    }
    for (let i = 0; i < 360; i++) {
      const shade = rnd() > 0.48 ? 255 : 0;
      ctx.fillStyle = `rgba(${shade}, ${shade}, ${shade}, ${0.018 + rnd() * 0.028})`;
      const size = rnd() > 0.9 ? 2 : 1;
      ctx.fillRect(Math.floor(rnd() * 256), Math.floor(rnd() * 256), size, size);
    }
  }
  const tex = new THREE.CanvasTexture(canvas as unknown as HTMLCanvasElement);
  tex.magFilter = THREE.LinearFilter;
  tex.minFilter = THREE.LinearMipmapLinearFilter;
  tex.wrapS = THREE.RepeatWrapping;
  tex.wrapT = THREE.RepeatWrapping;
  tex.anisotropy = 4;
  tex.generateMipmaps = true;
  tex.colorSpace = THREE.SRGBColorSpace;
  return tex;
}

// Generate geometry with repeating 1-meter UV mapping so HD textures tile seamlessly across large walls
function createRepeatingBox(w: number, h: number, d: number, x: number, y: number, z: number): THREE.BufferGeometry {
  const geo = new THREE.BoxGeometry(w, h, d);
  const uvs = geo.attributes.uv;
  // BoxGeometry face order: +X (0-3), -X (4-7), +Y (8-11), -Y (12-15), +Z (16-19), -Z (20-23)
  const faceSizes = [
    [d, h], // +X
    [d, h], // -X
    [w, d], // +Y
    [w, d], // -Y
    [w, h], // +Z
    [w, h], // -Z
  ];

  for (let face = 0; face < 6; face++) {
    const [fw, fh] = faceSizes[face];
    const base = face * 4;
    // Map textures at 2-meter scale so surfaces are calm and not visually dense
    const scale = 0.5;
    uvs.setXY(base + 0, 0, fh * scale);
    uvs.setXY(base + 1, fw * scale, fh * scale);
    uvs.setXY(base + 2, 0, 0);
    uvs.setXY(base + 3, fw * scale, 0);
  }
  uvs.needsUpdate = true;

  // Add Vertex Colors for subtle ground contact Ambient Occlusion
  const count = geo.attributes.position.count;
  const colors = new Float32Array(count * 3);
  const pos = geo.attributes.position;
  for (let i = 0; i < count; i++) {
    const py = pos.getY(i);
    // Base contact shadow: vertices at lower edge get subtle grounded shading
    const isBottom = py < 0;
    const factor = isBottom && y <= 0.2 ? 0.82 : isBottom ? 0.88 : 1.0;
    colors[i * 3 + 0] = factor;
    colors[i * 3 + 1] = factor;
    colors[i * 3 + 2] = factor;
  }
  geo.setAttribute('color', new THREE.BufferAttribute(colors, 3));

  geo.translate(x + w / 2, y + h / 2, z + d / 2);
  return geo;
}

export class WorldView {
  group = new THREE.Group();
  clouds = new THREE.Group();
  sun = new THREE.Group();
  private scene: THREE.Scene;
  private waterTex: THREE.Texture | null = null;
  private lastAmbientTick = -1;
  private pickupTemplates: THREE.Group[];
  private pickups = new Map<number, THREE.Group>();
  private chickenTemplate: THREE.Group;
  private chickens = new Map<number, { mesh: THREE.Group; tx: number; ty: number; tz: number; targetYaw: number }>();
  private lastAnimT = 0;
  boxes: { x0: number; x1: number; y0: number; y1: number; z0: number; z1: number }[] = [];
  constructor(scene: THREE.Scene, map: MapData) {
    this.scene = scene;
    this.pickupTemplates = [this.createPickup(0), this.createPickup(1), this.createPickup(2)];
    this.chickenTemplate = this.createChicken();

    // 1. Build Repeating-UV High-Definition Voxel Map with Merged Geometries per Material
    const byMat = new Map<number, THREE.BufferGeometry[]>();
    for (const b of map.blocks) {
      if (b.t !== 12) this.boxes.push({ x0: b.x, x1: b.x + b.w, y0: b.y, y1: b.y + b.h, z0: b.z, z1: b.z + b.d });
      if (!byMat.has(b.t)) byMat.set(b.t, []);
      const geo = createRepeatingBox(b.w, b.h, b.d, b.x, b.y, b.z);
      byMat.get(b.t)!.push(geo);
    }

    for (const [t, geos] of byMat) {
      const tex = createHDVoxelTexture(t);
      let mat: THREE.Material;

      if (t === 8) { // Luminous Cyber Beacon
        mat = new THREE.MeshLambertMaterial({
          map: tex,
          emissive: 0xf59e0b,
          emissiveIntensity: 0.85,
          vertexColors: true,
        });
      } else if (t === 12) { // Water surface
        this.waterTex = tex;
        mat = new THREE.MeshLambertMaterial({
          map: tex,
          transparent: true,
          opacity: 0.76,
          depthWrite: false,
        });
      } else if (t === 11) { // Glass
        mat = new THREE.MeshLambertMaterial({
          map: tex,
          transparent: true,
          opacity: 0.72,
        });
      } else {
        mat = new THREE.MeshLambertMaterial({
          map: tex,
          vertexColors: true,
        });
      }

      const merged = mergeGeometries(geos, false)!;
      for (const g of geos) g.dispose();
      this.group.add(new THREE.Mesh(merged, mat));
    }
    this.scene.add(this.group);

    // 2. High-Tech Sky with Sun & Volumetric Cloud Slabs
    this.setupSky();
    this.setupBackdrop();
    this.setupMarkings();

  }

  private createPickup(kind: number): THREE.Group {
    const group = new THREE.Group();
    const colors = [0xe4a94a, 0xd94c4c, 0x35c9e8];
    const color = colors[kind];
    const core = new THREE.Mesh(
      new THREE.BoxGeometry(0.52, 0.36, 0.52),
      new THREE.MeshLambertMaterial({ color, emissive: color, emissiveIntensity: 0.75 }),
    );
    const glow = new THREE.Mesh(
      new THREE.SphereGeometry(0.68, 12, 8),
      new THREE.MeshBasicMaterial({ color, transparent: true, opacity: 0.16, depthWrite: false, blending: THREE.AdditiveBlending }),
    );
    group.add(core, glow);
    const markerMat = new THREE.MeshBasicMaterial({ color: 0xf5f2df });
    if (kind === 0) {
      const left = new THREE.Mesh(new THREE.BoxGeometry(0.09, 0.32, 0.09), markerMat);
      const right = left.clone();
      left.position.set(-0.12, 0.24, 0);
      right.position.set(0.12, 0.24, 0);
      group.add(left, right);
    } else if (kind === 1) {
      const horizontal = new THREE.Mesh(new THREE.BoxGeometry(0.34, 0.1, 0.08), markerMat);
      const vertical = new THREE.Mesh(new THREE.BoxGeometry(0.1, 0.34, 0.08), markerMat);
      horizontal.position.set(0, 0, -0.3);
      vertical.position.set(0, 0, -0.3);
      group.add(horizontal, vertical);
    } else {
      const shaft = new THREE.Mesh(new THREE.BoxGeometry(0.1, 0.3, 0.08), markerMat);
      const wingL = new THREE.Mesh(new THREE.BoxGeometry(0.1, 0.22, 0.08), markerMat);
      const wingR = wingL.clone();
      shaft.position.set(0, 0.08, -0.3);
      wingL.position.set(-0.1, -0.05, -0.3);
      wingR.position.set(0.1, -0.05, -0.3);
      wingL.rotation.z = -0.7;
      wingR.rotation.z = 0.7;
      group.add(shaft, wingL, wingR);
    }
    return group;
  }

  setPickup(id: number, kind: number, x: number, y: number, z: number) {
    this.removePickup(id);
    const pickup = (this.pickupTemplates[kind] ?? this.pickupTemplates[0]).clone();
    pickup.position.set(x, y + 0.48, z);
    pickup.userData.baseY = pickup.position.y;
    this.pickups.set(id, pickup);
    this.group.add(pickup);
  }

  removePickup(id: number) {
    const pickup = this.pickups.get(id);
    if (!pickup) return;
    this.group.remove(pickup);
    this.pickups.delete(id);
  }

  // ---- Battlefield chickens (CS-style easter egg) ----

  private createChicken(): THREE.Group {
    const white = new THREE.MeshLambertMaterial({ color: 0xf2ede0 });
    const cream = new THREE.MeshLambertMaterial({ color: 0xe4ddc9 });
    const orange = new THREE.MeshLambertMaterial({ color: 0xe8a13a });
    const red = new THREE.MeshLambertMaterial({ color: 0xcc3b36 });
    const black = new THREE.MeshLambertMaterial({ color: 0x24211e });
    const box = (mat: THREE.Material, w: number, h: number, d: number, x: number, y: number, z: number, rx = 0) => {
      const m = new THREE.Mesh(new THREE.BoxGeometry(w, h, d), mat);
      m.position.set(x, y, z);
      m.rotation.x = rx;
      return m;
    };
    const chicken = new THREE.Group();
    chicken.add(box(white, 0.36, 0.32, 0.48, 0, 0.34, 0));            // body
    chicken.add(box(cream, 0.26, 0.22, 0.1, 0, 0.44, 0.28, -0.55));   // tail
    chicken.add(box(cream, 0.06, 0.2, 0.3, -0.21, 0.38, 0.02));       // left wing
    chicken.add(box(cream, 0.06, 0.2, 0.3, 0.21, 0.38, 0.02));        // right wing
    chicken.add(box(orange, 0.05, 0.18, 0.05, -0.08, 0.09, 0));       // legs
    chicken.add(box(orange, 0.05, 0.18, 0.05, 0.08, 0.09, 0));
    chicken.add(box(white, 0.22, 0.22, 0.2, 0, 0.58, -0.27));         // head
    chicken.add(box(red, 0.05, 0.1, 0.16, 0, 0.74, -0.25));           // comb
    chicken.add(box(red, 0.04, 0.07, 0.04, 0, 0.56, -0.42));          // wattle
    chicken.add(box(orange, 0.08, 0.06, 0.12, 0, 0.57, -0.42));       // beak
    chicken.add(box(black, 0.03, 0.05, 0.05, -0.115, 0.62, -0.33));   // eyes
    chicken.add(box(black, 0.03, 0.05, 0.05, 0.115, 0.62, -0.33));
    return chicken;
  }

  setChicken(id: number, x: number, y: number, z: number, dirX: number, dirZ: number) {
    let entry = this.chickens.get(id);
    if (!entry) {
      const mesh = this.chickenTemplate.clone();
      mesh.position.set(x, y, z);
      entry = { mesh, tx: x, ty: y, tz: z, targetYaw: mesh.rotation.y };
      this.chickens.set(id, entry);
      this.group.add(mesh);
    }
    entry.tx = x; entry.ty = y; entry.tz = z;
    if (dirX !== 0 || dirZ !== 0) entry.targetYaw = Math.atan2(-dirX, -dirZ);
  }

  takeChickenDeath(id: number): [number, number, number] | null {
    const entry = this.chickens.get(id);
    if (!entry) return null;
    this.group.remove(entry.mesh);
    this.chickens.delete(id);
    return [entry.mesh.position.x, entry.mesh.position.y, entry.mesh.position.z];
  }

  clearChickens() {
    for (const { mesh } of this.chickens.values()) this.group.remove(mesh);
    this.chickens.clear();
  }

  private setupSky() {
    // Soft glowing sun disc
    const sunCanvas = new OffscreenCanvas(96, 96);
    const sctx = sunCanvas.getContext('2d')!;
    const grad = sctx.createRadialGradient(48, 48, 0, 48, 48, 48);
    grad.addColorStop(0, 'rgba(255, 249, 221, 1.0)');
    grad.addColorStop(0.24, 'rgba(245, 205, 132, 0.72)');
    grad.addColorStop(0.66, 'rgba(221, 168, 92, 0.18)');
    grad.addColorStop(1, 'rgba(190, 130, 70, 0.0)');
    sctx.fillStyle = grad;
    sctx.fillRect(0, 0, 96, 96);
    const sunTex = new THREE.CanvasTexture(sunCanvas as unknown as HTMLCanvasElement);
    sunTex.colorSpace = THREE.SRGBColorSpace;

    const sunMesh = new THREE.Mesh(
      new THREE.PlaneGeometry(52, 52),
      new THREE.MeshBasicMaterial({ map: sunTex, transparent: true, depthWrite: false, blending: THREE.AdditiveBlending })
    );
    sunMesh.position.set(105, 88, -135);
    sunMesh.lookAt(0, 0, 0);
    this.sun.add(sunMesh);
    this.scene.add(this.sun);

    // Low-poly cloud banks retain the voxel style without forming a flat ceiling.
    const cloudMat = new THREE.MeshBasicMaterial({ color: 0xd7e1e2, transparent: true, opacity: 0.24, depthWrite: false });
    const cloudGeos: THREE.BufferGeometry[] = [];
    let cSeed = 42;
    const cRnd = () => (cSeed = (cSeed * 16807) % 2147483647) / 2147483647;
    for (let i = 0; i < 28; i++) {
      const angle = i / 28 * Math.PI * 2;
      const distance = 95 + cRnd() * 175;
      const cloud = new THREE.DodecahedronGeometry(1, 1);
      cloud.scale(10 + cRnd() * 18, 2.2 + cRnd() * 2.4, 7 + cRnd() * 14);
      cloud.translate(Math.cos(angle) * distance + (cRnd() - 0.5) * 45, 55 + cRnd() * 15, Math.sin(angle) * distance + (cRnd() - 0.5) * 45);
      cloudGeos.push(cloud);
    }
    const mergedClouds = mergeGeometries(cloudGeos)!;
    for (const geo of cloudGeos) geo.dispose();
    this.clouds.add(new THREE.Mesh(mergedClouds, cloudMat));
    this.scene.add(this.clouds);
  }
  private setupBackdrop() {
    const geos: THREE.BufferGeometry[] = [];
    let seed = 9173;
    const rnd = () => (seed = (seed * 16807) % 2147483647) / 2147483647;
    for (let i = 0; i < 36; i++) {
      const verticalEdge = i % 2 === 0;
      const side = i % 4 < 2 ? -1 : 1;
      const along = -210 + ((i * 47) % 420);
      const distance = 96 + Math.floor(rnd() * 54);
      const width = 9 + Math.floor(rnd() * 17);
      const depth = 9 + Math.floor(rnd() * 17);
      const height = 9 + Math.floor(rnd() * 27);
      const x = verticalEdge ? side * distance : along;
      const z = verticalEdge ? along : side * distance;
      const geo = new THREE.BoxGeometry(width, height, depth);
      geo.translate(x, height * 0.5 - 1, z);
      geos.push(geo);
    }
    const merged = mergeGeometries(geos)!;
    for (const geo of geos) geo.dispose();
    this.group.add(new THREE.Mesh(merged, new THREE.MeshLambertMaterial({ color: 0x46555a })));
  }
  private setupMarkings() {
    const geos: THREE.BufferGeometry[] = [];
    const add = (x: number, y: number, z: number, w: number, d: number) => {
      const geo = new THREE.BoxGeometry(w, 0.018, d);
      geo.translate(x, y, z);
      geos.push(geo);
    };
    for (let z = -64; z <= 64; z += 8) add(0, 0.036, z, 0.18, 3.2);
    for (let x = -64; x <= 64; x += 8) add(x, 0.037, 2, 3.2, 0.18);
    add(31, 1.526, -51.5, 22, 0.16);
    add(53, 1.526, -51.5, 0.16, 21);
    add(31, 1.526, -30.5, 22, 0.16);
    add(31, 1.526, -41, 0.16, 21);
    for (let i = 0; i < 6; i++) add(-13 + i * 2.2, 0.038, 0.5, 1.1, 0.24);
    const merged = mergeGeometries(geos)!;
    for (const geo of geos) geo.dispose();
    this.group.add(new THREE.Mesh(merged, new THREE.MeshBasicMaterial({ color: 0xd2aa55, transparent: true, opacity: 0.78 })));
  }
  raycastDistance(origin: THREE.Vector3, dir: THREE.Vector3, maxDist: number, outNormal?: THREE.Vector3): number {
    let best = maxDist;
    outNormal?.set(0, 1, 0);
    for (const b of this.boxes) {
      let tmin = 0, tmax = best;
      let nx = 0, ny = 0, nz = 0;
      if (Math.abs(dir.x) < 1e-9) { if (origin.x < b.x0 || origin.x > b.x1) continue; }
      else {
        let t1 = (b.x0 - origin.x) / dir.x, t2 = (b.x1 - origin.x) / dir.x;
        if (t1 > t2) [t1, t2] = [t2, t1];
        if (t1 > tmin) { tmin = t1; nx = dir.x > 0 ? -1 : 1; ny = nz = 0; }
        if (t2 < tmax) tmax = t2;
        if (tmin > tmax) continue;
      }
      if (Math.abs(dir.y) < 1e-9) { if (origin.y < b.y0 || origin.y > b.y1) continue; }
      else {
        let t1 = (b.y0 - origin.y) / dir.y, t2 = (b.y1 - origin.y) / dir.y;
        if (t1 > t2) [t1, t2] = [t2, t1];
        if (t1 > tmin) { tmin = t1; ny = dir.y > 0 ? -1 : 1; nx = nz = 0; }
        if (t2 < tmax) tmax = t2;
        if (tmin > tmax) continue;
      }
      if (Math.abs(dir.z) < 1e-9) { if (origin.z < b.z0 || origin.z > b.z1) continue; }
      else {
        let t1 = (b.z0 - origin.z) / dir.z, t2 = (b.z1 - origin.z) / dir.z;
        if (t1 > t2) [t1, t2] = [t2, t1];
        if (t1 > tmin) { tmin = t1; nz = dir.z > 0 ? -1 : 1; nx = ny = 0; }
        if (t2 < tmax) tmax = t2;
        if (tmin > tmax) continue;
      }
      if (tmin > 0 && tmin < best) {
        best = tmin;
        outNormal?.set(nx, ny, nz);
      }
    }
    return best;
  }

  animate(t: number) {
    const ambientTick = Math.floor(t / 50);
    if (ambientTick !== this.lastAmbientTick) {
      this.lastAmbientTick = ambientTick;
      if (this.clouds.visible) this.clouds.position.x = (t * 0.0018) % 48;
      if (this.waterTex) {
        this.waterTex.offset.x = (t * 0.00005) % 1;
        this.waterTex.offset.y = (t * 0.00008) % 1;
      }
    }
    for (const [id, pickup] of this.pickups) {
      pickup.rotation.y = t * 0.0018 + id;
      pickup.position.y = pickup.userData.baseY + Math.sin(t * 0.003 + id) * 0.08;
      pickup.scale.setScalar(1 + Math.sin(t * 0.004 + id) * 0.06);
    }
    const dt = Math.min(0.1, (t - this.lastAnimT) / 1000);
    this.lastAnimT = t;
    const smooth = 1 - Math.exp(-dt * 6);
    for (const [id, chicken] of this.chickens) {
      const dx = chicken.tx - chicken.mesh.position.x;
      const dz = chicken.tz - chicken.mesh.position.z;
      const gap = Math.hypot(dx, dz);
      const moving = gap > 0.05;
      if (moving) {
        chicken.mesh.position.x += dx * smooth;
        chicken.mesh.position.z += dz * smooth;
        let dyaw = chicken.targetYaw - chicken.mesh.rotation.y;
        dyaw = Math.atan2(Math.sin(dyaw), Math.cos(dyaw));
        chicken.mesh.rotation.y += dyaw * smooth;
      }
      // waddle: quick hops while walking, a gentle breathing bob while idle
      const hopPhase = t * 0.02 + id * 1.7;
      chicken.mesh.position.y = chicken.ty + (moving ? Math.abs(Math.sin(hopPhase)) * 0.07 : Math.sin(t * 0.003 + id) * 0.012);
      chicken.mesh.rotation.z = moving ? Math.sin(hopPhase) * 0.06 : 0;
    }
  }
}

// ---- Client-side AABB sliding collision ----
export type Box = { x0: number; x1: number; y0: number; y1: number; z0: number; z1: number };

const PH = 0.3;
const SU = 0.55;
const EPS = 0.001;

function overlaps(px: number, py: number, pz: number, b: Box, height: number): boolean {
  return px - PH < b.x1 && px + PH > b.x0 && py < b.y1 && py + height > b.y0 && pz - PH < b.z1 && pz + PH > b.z0;
}

export function canOccupy(boxes: Box[], pos: THREE.Vector3, height: number): boolean {
  for (const b of boxes) if (overlaps(pos.x, pos.y, pos.z, b, height)) return false;
  return true;
}


function stepBlocked(boxes: Box[], px: number, newFeet: number, pz: number, height: number): boolean {
  for (const b of boxes) if (overlaps(px, newFeet + EPS, pz, b, height)) return true;
  return false;
}

export function moveAABB(
  pos: THREE.Vector3,
  vel: THREE.Vector3,
  dt: number,
  boxes: Box[],
  height = 1.8,
  canStep = true,
): boolean {
  let grounded = depenetrate(pos, vel, boxes, height);

  const dx = vel.x * dt;
  if (dx !== 0) {
    const startX = pos.x;
    let nextX = startX + dx;
    let blocked = false;
    for (const b of boxes) {
      if (pos.y >= b.y1 || pos.y + height <= b.y0 || pos.z - PH >= b.z1 || pos.z + PH <= b.z0) continue;
      const crossed = dx > 0
        ? startX + PH <= b.x0 + EPS && nextX + PH > b.x0
        : startX - PH >= b.x1 - EPS && nextX - PH < b.x1;
      if (!crossed) continue;
      const stepH = b.y1 - pos.y;
      if (canStep && stepH > 0 && stepH <= SU && !stepBlocked(boxes, nextX, b.y1, pos.z, height)) {
        pos.y = b.y1 + EPS;
        grounded = true;
        continue;
      }
      nextX = dx > 0 ? Math.min(nextX, b.x0 - PH - EPS) : Math.max(nextX, b.x1 + PH + EPS);
      blocked = true;
    }
    pos.x = nextX;
    if (blocked) vel.x = 0;
  }

  const dz = vel.z * dt;
  if (dz !== 0) {
    const startZ = pos.z;
    let nextZ = startZ + dz;
    let blocked = false;
    for (const b of boxes) {
      if (pos.y >= b.y1 || pos.y + height <= b.y0 || pos.x - PH >= b.x1 || pos.x + PH <= b.x0) continue;
      const crossed = dz > 0
        ? startZ + PH <= b.z0 + EPS && nextZ + PH > b.z0
        : startZ - PH >= b.z1 - EPS && nextZ - PH < b.z1;
      if (!crossed) continue;
      const stepH = b.y1 - pos.y;
      if (canStep && stepH > 0 && stepH <= SU && !stepBlocked(boxes, pos.x, b.y1, nextZ, height)) {
        pos.y = b.y1 + EPS;
        grounded = true;
        continue;
      }
      nextZ = dz > 0 ? Math.min(nextZ, b.z0 - PH - EPS) : Math.max(nextZ, b.z1 + PH + EPS);
      blocked = true;
    }
    pos.z = nextZ;
    if (blocked) vel.z = 0;
  }

  const dy = vel.y * dt;
  if (dy !== 0) {
    const startY = pos.y;
    let nextY = startY + dy;
    for (const b of boxes) {
      if (pos.x - PH >= b.x1 || pos.x + PH <= b.x0 || pos.z - PH >= b.z1 || pos.z + PH <= b.z0) continue;
      if (dy < 0 && startY >= b.y1 - EPS && nextY <= b.y1 + EPS) {
        nextY = Math.max(nextY, b.y1 + EPS);
        vel.y = 0;
        grounded = true;
      } else if (dy > 0 && startY + height <= b.y0 + EPS && nextY + height > b.y0) {
        nextY = Math.min(nextY, b.y0 - height - EPS);
        vel.y = 0;
      }
    }
    pos.y = nextY;
  }
  grounded = depenetrate(pos, vel, boxes, height) || grounded;
  return grounded;
}

function depenetrate(pos: THREE.Vector3, vel: THREE.Vector3, boxes: Box[], height: number): boolean {
  let grounded = false;
  for (let pass = 0; pass < 6; pass++) {
    let found = false;
    for (const b of boxes) {
      if (!overlaps(pos.x, pos.y, pos.z, b, height)) continue;
      found = true;
      let axis = 0;
      let delta = b.x0 - EPS - (pos.x + PH);
      let best = Math.abs(delta);
      const choose = (candidate: number, candidateAxis: number) => {
        const size = Math.abs(candidate);
        if (size < best) { best = size; delta = candidate; axis = candidateAxis; }
      };
      choose(b.x1 + EPS - (pos.x - PH), 0);
      choose(b.y0 - EPS - (pos.y + height), 1);
      choose(b.y1 + EPS - pos.y, 1);
      choose(b.z0 - EPS - (pos.z + PH), 2);
      choose(b.z1 + EPS - (pos.z - PH), 2);
      if (axis === 0) { pos.x += delta; vel.x = 0; }
      else if (axis === 1) { pos.y += delta; vel.y = 0; grounded ||= delta > 0; }
      else { pos.z += delta; vel.z = 0; }
      break;
    }
    if (!found) break;
  }
  return grounded;
}
