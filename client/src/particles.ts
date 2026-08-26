import * as THREE from 'three';
import { mergeGeometries } from 'three/examples/jsm/utils/BufferGeometryUtils.js';

const UP = new THREE.Vector3(0, 1, 0);
const FORWARD = new THREE.Vector3(0, 0, 1);
const MAX_PARTICLES = 512;
const dummy = new THREE.Object3D();
const pColor = new THREE.Color();
const grenadeStep = new THREE.Vector3();
const FIRE_COLORS = [0xff2200, 0xff5500, 0xff9900, 0xffdd33, 0xfffa88];
const SMOKE_COLORS = [0x1e1e22, 0x38383e, 0x55555c, 0x777780];
const DEATH_COLORS = [0x00a8aa, 0x2b3577, 0xd9a377, 0x3a3a3a];

interface ParticleData {
  x: number;
  y: number;
  z: number;
  vx: number;
  vy: number;
  vz: number;
  color: number;
  born: number;
  life: number;
  size: number;
  gravity: number;
}

interface GrenadeVisual {
  mesh: THREE.Mesh;
  player: number;
  vx: number;
  vy: number;
  vz: number;
  born: number;
}

interface BlastVisual {
  mesh: THREE.Mesh<THREE.SphereGeometry, THREE.MeshBasicMaterial>;
  light: THREE.PointLight;
  born: number;
}

interface GrenadeCollider {
  raycastDistance(origin: THREE.Vector3, dir: THREE.Vector3, maxDist: number): number;
}

export class ParticleSystem {
  group = new THREE.Group();
  private particles: ParticleData[] = [];
  private geo = new THREE.BoxGeometry(0.06, 0.06, 0.06);
  private instancedMesh: THREE.InstancedMesh;
  private grenades: GrenadeVisual[] = [];
  private blasts: BlastVisual[] = [];
  private bulletHoleMesh: THREE.InstancedMesh;
  private bulletHoleCursor = 0;
  private blastGeo = new THREE.SphereGeometry(0.5, 12, 8);
  private grenadeGeo = (() => {
    const body = new THREE.CylinderGeometry(0.075, 0.075, 0.18, 12);
    const rib = new THREE.BoxGeometry(0.16, 0.04, 0.16);
    const fuze = new THREE.BoxGeometry(0.05, 0.07, 0.09);
    fuze.translate(0, 0.12, 0);
    const ring = new THREE.CylinderGeometry(0.025, 0.025, 0.02, 8);
    ring.translate(0.04, 0.13, 0);
    return mergeGeometries([body, rib, fuze, ring])!;
  })();
  private grenadeMat = new THREE.MeshLambertMaterial({ color: 0x475e38 });

  constructor(scene: THREE.Scene) {
    this.instancedMesh = new THREE.InstancedMesh(this.geo, new THREE.MeshLambertMaterial({ color: 0xffffff }), MAX_PARTICLES);
    this.instancedMesh.instanceMatrix.setUsage(THREE.DynamicDrawUsage);
    this.instancedMesh.count = 0;
    this.bulletHoleMesh = new THREE.InstancedMesh(
      new THREE.CircleGeometry(0.055, 8),
      new THREE.MeshBasicMaterial({ color: 0x17120f, polygonOffset: true, polygonOffsetFactor: -2 }),
      128,
    );
    this.bulletHoleMesh.instanceMatrix.setUsage(THREE.DynamicDrawUsage);
    this.bulletHoleMesh.frustumCulled = false;
    this.bulletHoleMesh.count = 0;
    this.group.add(this.instancedMesh, this.bulletHoleMesh);
    scene.add(this.group);
  }

  /** Spawn voxel debris on bullet hitting a surface */
  spawnImpact(pos: THREE.Vector3, normal: THREE.Vector3, color = 0xd6b36e, count = 8) {
    const available = MAX_PARTICLES - this.particles.length;
    count = Math.min(count, Math.max(0, available));
    if (count <= 0) return;
    const now = performance.now();
    for (let i = 0; i < count; i++) {
      const speed = 2.5 + Math.random() * 4.5;
      const rx = (Math.random() - 0.5) * 1.5;
      const ry = (Math.random() - 0.5) * 1.5;
      const rz = (Math.random() - 0.5) * 1.5;

      this.particles.push({
        x: pos.x,
        y: pos.y,
        z: pos.z,
        vx: (normal.x + rx) * speed,
        vy: Math.abs(normal.y + ry) * speed + 1.5,
        vz: (normal.z + rz) * speed,
        color,
        born: now,
        life: 400 + Math.random() * 300,
        size: 0.7 + Math.random() * 0.7,
        gravity: 18,
      });
    }
    const sparks = Math.min(5, Math.max(0, MAX_PARTICLES - this.particles.length));
    for (let i = 0; i < sparks; i++) {
      const speed = 5 + Math.random() * 6;
      this.particles.push({
        x: pos.x,
        y: pos.y,
        z: pos.z,
        vx: (normal.x + (Math.random() - 0.5)) * speed,
        vy: Math.abs(normal.y) * speed + 2,
        vz: (normal.z + (Math.random() - 0.5)) * speed,
        color: FIRE_COLORS[i % FIRE_COLORS.length],
        born: now,
        life: 180 + Math.random() * 140,
        size: 0.45 + Math.random() * 0.35,
        gravity: 12,
      });
    }
  }
  spawnBulletHole(pos: THREE.Vector3, normal: THREE.Vector3) {
    const slot = this.bulletHoleCursor++ % 128;
    dummy.position.copy(pos).addScaledVector(normal, 0.006);
    dummy.quaternion.setFromUnitVectors(FORWARD, normal);
    dummy.rotateZ(Math.random() * Math.PI * 2);
    dummy.scale.setScalar(0.75 + Math.random() * 0.5);
    dummy.updateMatrix();
    this.bulletHoleMesh.setMatrixAt(slot, dummy.matrix);
    this.bulletHoleMesh.count = Math.min(this.bulletHoleCursor, 128);
    this.bulletHoleMesh.instanceMatrix.needsUpdate = true;
  }
  clearBulletHoles() {
    this.bulletHoleCursor = 0;
    this.bulletHoleMesh.count = 0;
  }
  spawnDeath(pos: THREE.Vector3, headshot = false) {
    const count = Math.min(headshot ? 42 : 30, MAX_PARTICLES - this.particles.length);
    const now = performance.now();
    for (let i = 0; i < count; i++) {
      const angle = Math.random() * Math.PI * 2;
      const speed = 2.5 + Math.random() * (headshot ? 7 : 5);
      this.particles.push({
        x: pos.x + (Math.random() - 0.5) * 0.45,
        y: pos.y + (Math.random() - 0.5) * 0.9,
        z: pos.z + (Math.random() - 0.5) * 0.3,
        vx: Math.cos(angle) * speed,
        vy: 2 + Math.random() * (headshot ? 8 : 5),
        vz: Math.sin(angle) * speed,
        color: headshot && i % 4 === 0 ? 0xc92a2a : DEATH_COLORS[i % DEATH_COLORS.length],
        born: now,
        life: 650 + Math.random() * 500,
        size: 1.8 + Math.random() * 2.2,
        gravity: 18,
      });
    }
  }

  spawnGrenade(player: number, pos: THREE.Vector3, vel: THREE.Vector3) {
    const now = performance.now();
    const existing = this.grenades.find((g) => g.player === player && now - g.born < 400);
    if (existing) {
      existing.mesh.position.copy(pos);
      existing.vx = vel.x;
      existing.vy = vel.y;
      existing.vz = vel.z;
      return;
    }
    const mesh = new THREE.Mesh(this.grenadeGeo, this.grenadeMat);
    mesh.position.copy(pos);
    this.group.add(mesh);
    this.grenades.push({ mesh, player, vx: vel.x, vy: vel.y, vz: vel.z, born: now });
  }

  /** Spawn dynamic high-impact explosion fire, sparks, and billowing smoke */
  spawnExplosion(pos: THREE.Vector3) {
    const now = performance.now();
    const blastMat = new THREE.MeshBasicMaterial({ color: 0xff8a22, transparent: true, opacity: 0.95, depthWrite: false, blending: THREE.AdditiveBlending });
    const blast = new THREE.Mesh(this.blastGeo, blastMat);
    const light = new THREE.PointLight(0xff7a22, 14, 20, 2);
    blast.position.copy(pos).y += 0.35;
    light.position.copy(blast.position);
    this.group.add(blast, light);
    this.blasts.push({ mesh: blast, light, born: now });
    // 1. Fiery Blast Core & Shrapnel Sparks (Spherical burst)
    for (let i = 0; i < 48; i++) {
      if (this.particles.length >= MAX_PARTICLES) break;
      const speed = 4.0 + Math.random() * 8.5;
      const theta = Math.random() * Math.PI * 2;
      const phi = Math.acos(Math.random() * 2 - 1);
      const vx = Math.sin(phi) * Math.cos(theta) * speed;
      const vy = Math.abs(Math.cos(phi)) * speed * 0.9 + 2.0;
      const vz = Math.sin(phi) * Math.sin(theta) * speed;
      this.particles.push({
        x: pos.x,
        y: pos.y + 0.2,
        z: pos.z,
        vx,
        vy,
        vz,
        color: FIRE_COLORS[i % FIRE_COLORS.length],
        born: now,
        life: 550 + Math.random() * 450,
        size: 2.5 + Math.random() * 2.5,
        gravity: 14,
      });
    }

    // 2. Billowing Volumetric Smoke Plumes (Rising ash & dark smoke)
    for (let i = 0; i < 30; i++) {
      if (this.particles.length >= MAX_PARTICLES) break;
      const speed = 1.0 + Math.random() * 2.5;
      const angle = Math.random() * Math.PI * 2;
      this.particles.push({
        x: pos.x + (Math.random() - 0.5) * 0.8,
        y: pos.y + 0.3 + Math.random() * 0.4,
        z: pos.z + (Math.random() - 0.5) * 0.8,
        vx: Math.cos(angle) * speed,
        vy: 2.5 + Math.random() * 3.5, // Ascending smoke
        vz: Math.sin(angle) * speed,
        color: SMOKE_COLORS[i % SMOKE_COLORS.length],
        born: now,
        life: 1400 + Math.random() * 800,
        size: 6 + Math.random() * 5,
        gravity: -0.35,
      });
    }

    // 3. Ground Shrapnel & Debris
    this.spawnImpact(pos, UP, 0x8a7a60, 18);

    let nearest = -1;
    let nearestSq = 4;
    for (let i = 0; i < this.grenades.length; i++) {
      const d = this.grenades[i].mesh.position.distanceToSquared(pos);
      if (d < nearestSq) { nearest = i; nearestSq = d; }
    }
    if (nearest >= 0) {
      this.group.remove(this.grenades[nearest].mesh);
      this.grenades.splice(nearest, 1);
    }
  }

  update(dt: number, now = performance.now(), world: GrenadeCollider | null = null) {
    let alive = 0;
    let colorsDirty = false;

    for (let i = 0; i < this.particles.length; i++) {
      const p = this.particles[i];
      const age = now - p.born;
      if (age > p.life) continue;

      p.vy -= p.gravity * dt;
      p.x += p.vx * dt;
      p.y += p.vy * dt;
      p.z += p.vz * dt;
      const scale = p.size * Math.max(0.08, 1 - age / p.life);

      dummy.position.set(p.x, p.y, p.z);
      dummy.scale.setScalar(scale);
      dummy.updateMatrix();

      this.instancedMesh.setMatrixAt(alive, dummy.matrix);
      this.instancedMesh.setColorAt(alive, pColor.setHex(p.color));
      colorsDirty = true;

      this.particles[alive++] = p;
    }
    this.particles.length = alive;
    this.instancedMesh.count = alive;
    if (alive > 0) {
      this.instancedMesh.instanceMatrix.needsUpdate = true;
      if (colorsDirty && this.instancedMesh.instanceColor) {
        this.instancedMesh.instanceColor.needsUpdate = true;
      }
    }
    let liveBlasts = 0;
    for (const blast of this.blasts) {
      const progress = (now - blast.born) / 260;
      if (progress >= 1) {
        this.group.remove(blast.mesh, blast.light);
        blast.mesh.material.dispose();
        continue;
      }
      blast.mesh.scale.setScalar(0.5 + progress * 7);
      blast.mesh.material.opacity = (1 - progress) * (1 - progress);
      blast.light.intensity = 14 * (1 - progress);
      this.blasts[liveBlasts++] = blast;
    }
    this.blasts.length = liveBlasts;
    let liveGrenades = 0;
    for (const g of this.grenades) {
      if (now - g.born > 2100) {
        this.group.remove(g.mesh);
        continue;
      }
      g.vy -= 22 * dt;
      grenadeStep.set(g.vx * dt, g.vy * dt, g.vz * dt);
      const travel = grenadeStep.length();
      if (world && travel > 0) {
        grenadeStep.multiplyScalar(1 / travel);
        const hit = world.raycastDistance(g.mesh.position, grenadeStep, travel);
        if (hit < travel) {
          g.mesh.position.addScaledVector(grenadeStep, Math.max(0, hit - 0.03));
          g.vx = g.vy = g.vz = 0;
        } else {
          g.mesh.position.addScaledVector(grenadeStep, travel);
        }
      } else {
        g.mesh.position.add(grenadeStep);
      }
      if (g.mesh.position.y < 0.09) {
        g.mesh.position.y = 0.09;
        g.vx = g.vy = g.vz = 0;
      }
      g.mesh.rotation.x += dt * 9;
      g.mesh.rotation.z += dt * 6;
      this.grenades[liveGrenades++] = g;
    }
    this.grenades.length = liveGrenades;
  }
}
