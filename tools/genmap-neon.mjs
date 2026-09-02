// 《霓虹都会》地图生成器 — Neon City DLC (Pixel Strike)
// 赛博朋克下城区：棋盘式摩天楼街区 + 中央地标塔 + 水运河 + 霓虹灯柱。
// 确定性输出（固定种子），产物写入 maps/neon-city.json。
// 用法: node tools/genmap-neon.mjs
import { setTimeout as sleep } from 'node:timers/promises';

const SIZE = 512;
const HALF = SIZE / 2;

// 材质类型（与 genmap.mjs 的 T 表一致）
const T = {
  FLOOR: 0, CONCRETE: 1, CRATE: 2, STEEL: 3, BASALT: 4, FOLIAGE: 5,
  SANDSTONE: 6, TERRACOTTA: 7, BEACON: 8, CARBON: 9, SHELF: 10,
  GLASS: 11, WATER: 12, ROAD: 13,
};

// 确定性随机（mulberry32）
function mulberry32(seed) {
  let a = seed >>> 0;
  return () => {
    a |= 0; a = (a + 0x6d2b79f5) | 0;
    let t = Math.imul(a ^ (a >>> 15), 1 | a);
    t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t;
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}
const rng = mulberry32(20260829);
const rr = (min, max) => min + rng() * (max - min);
const ri = (min, max) => Math.floor(rr(min, max + 1));

const blocks = [];
const spawns = [];
const solid = []; // {x0,x1,y0,y1,z0,z1} 用于出生点排空检测（含水体除外）

function addBox(x, y, z, w, h, d, t) {
  blocks.push({ x, y, z, w, h, d, t });
  if (t !== T.WATER) {
    solid.push({ x0: x, x1: x + w, y0: y, y1: y + h, z0: z, z1: z + d });
  }
  return blocks[blocks.length - 1];
}

// 出生点有效性：与任何实体块保持 0.6m 间距且站立空间 2.1m 无遮挡
function spawnClear(x, y, z) {
  const m = 0.6;
  for (const b of solid) {
    if (x + m > b.x0 && x - m < b.x1 && z + m > b.z0 && z - m < b.z1 && y + 2.1 > b.y0 && y < b.y1) return false;
  }
  return true;
}
function tryAddSpawn(x, y, z) {
  if (spawns.length >= 220) return false;
  const ry = Math.round(y * 100) / 100;
  const rx = Math.round(x * 10) / 10;
  const rz = Math.round(z * 10) / 10;
  if (spawnClear(rx, ry, rz) && !spawns.some((s) => s[0] === rx && s[1] === ry && s[2] === rz)) {
    spawns.push([rx, ry, rz]);
    return true;
  }
  return false;
}

// ---------- 0. 地面 + 边界 ----------
addBox(-HALF, -1, -HALF, SIZE, 1, SIZE, T.FLOOR);
const WALL_H = 10;
addBox(-HALF, 0, -HALF, SIZE, WALL_H, 2, T.CONCRETE);
addBox(-HALF, 0, HALF - 2, SIZE, WALL_H, 2, T.CONCRETE);
addBox(-HALF, 0, -HALF, 2, WALL_H, SIZE, T.CONCRETE);
addBox(HALF - 2, 0, -HALF, 2, WALL_H, SIZE, T.CONCRETE);

// ---------- 1. 街道网格（含一条水运河） ----------
const STREET = 14;
const streetCenters = [-192, -128, -64, 0, 64, 128, 192];
const CANAL_STREET = 64; // x=64 的南北街改为水运河
for (const c of streetCenters) {
  // 东西向街道
  addBox(-HALF + 4, 0, c - STREET / 2, SIZE - 8, 0.2, STREET, T.ROAD);
  // 南北向街道（运河街只铺一半：南半段道路、北半段运河）
  if (c === CANAL_STREET) {
    addBox(c - STREET / 2, 0, 6, STREET, 0.2, HALF - 10, T.ROAD);
    // 运河：凹槽水体（非碰撞），底部深色
    addBox(c - STREET / 2, -1.4, -HALF + 4, STREET, 0.9, HALF - 10, T.WATER);
    addBox(c - STREET / 2, -1, -HALF + 4, STREET, 0.9, HALF - 10, T.BASALT);
  } else {
    addBox(c - STREET / 2, 0, -HALF + 4, STREET, 0.2, SIZE - 8, T.ROAD);
  }
}

// ---------- 2. 街区摩天楼（棋盘格子，格心避开街道） ----------
const TOWER_MATS = [T.GLASS, T.CONCRETE, T.CARBON, T.TERRACOTTA];
const cells = [];
for (const cx of streetCenters.slice(0, -1).map((v) => v + 32)) {
  for (const cz of streetCenters.slice(0, -1).map((v) => v + 32)) {
    cells.push([cx, cz]);
  }
}
for (const [cx, cz] of cells) {
  if (Math.abs(cx) < 40 && Math.abs(cz) < 40) continue; // 中央广场区留空
  const towers = ri(1, 2);
  for (let k = 0; k < towers; k++) {
    const w0 = rr(14, 24);
    const d0 = rr(14, 24);
    let tw = w0, td = d0;
    let ox = cx + rr(-10, 10) - w0 / 2;
    let oz = cz + rr(-10, 10) - d0 / 2;
    const mat = TOWER_MATS[ri(0, TOWER_MATS.length - 1)];
    const tiers = ri(2, 3);
    let y = 0;
    for (let t = 0; t < tiers; t++) {
      const h = rr(6, 12);
      addBox(ox, y, oz, tw, h, td, mat);
      y += h;
      const nw = tw * rr(0.6, 0.85);
      const nd = td * rr(0.6, 0.85);
      ox += (tw - nw) / 2;
      oz += (td - nd) / 2;
      tw = nw;
      td = nd;
    }
    // 低层塔加外置楼梯通楼顶：中心留作屋顶出生点，霓虹信标放角落
    if (y < 20 && rng() < 0.7) {
      const steps = Math.ceil(y / 1.2);
      for (let s = 0; s < steps; s++) {
        addBox(ox - 2 - s * 1.4, 0, oz + td / 2, 1.4, (s + 1) * 1.2, 3, T.STEEL);
      }
      if (tryAddSpawn(ox + tw / 2, y + 0.1, oz + td / 2)) {
        addBox(ox + tw - 1, y, oz + td - 1, 2, 1.2, 2, T.BEACON);
      }
    } else if (rng() < 0.65) {
      addBox(ox + tw - 1, y, oz + td - 1, 2, 1.2, 2, T.BEACON);
    }
  }
}

// ---------- 3. 中央广场：地标塔 + 环形灯阵 ----------
addBox(-6, 0, -6, 12, 30, 12, T.GLASS);
addBox(-8, 0, -8, 16, 2, 16, T.CARBON);
addBox(-8, 2, -8, 16, 0.4, 16, T.STEEL);
addBox(-2, 30, -2, 4, 1.6, 4, T.BEACON);
for (const [lx, lz] of [[-30, -30], [30, -30], [-30, 30], [30, 30]]) {
  addBox(lx - 0.8, 0, lz - 0.8, 1.6, 5, 1.6, T.STEEL);
  addBox(lx - 1.6, 5, lz - 1.6, 3.2, 1, 3.2, T.BEACON);
}

// ---------- 4. 街道灯柱（沿主街每 32 格） ----------
for (const c of streetCenters.filter((v) => v % 128 === 0)) {
  for (let p = -HALF + 16; p <= HALF - 16; p += 48) {
    addBox(p - 0.4, 0.2, c - STREET / 2 - 1.6, 0.8, 4, 0.8, T.STEEL);
    addBox(p - 0.8, 4.2, c - STREET / 2 - 2, 1.6, 0.7, 1.6, T.BEACON);
    addBox(c + STREET / 2 + 0.8, 0.2, p - 0.4, 0.8, 4, 0.8, T.STEEL);
    addBox(c + STREET / 2 + 0.6, 4.2, p - 0.8, 1.6, 0.7, 1.6, T.BEACON);
  }
}

// ---------- 5. 出生点：街道 + 广场 + 屋顶 ----------
let rooftopSpawns = 0;
for (const c of streetCenters) {
  for (let p = -HALF + 16; p <= HALF - 16; p += 9) {
    tryAddSpawn(p, 0.25, c);
    if (!(c === CANAL_STREET && p + 4.5 < 6)) tryAddSpawn(c, 0.25, p + 4.5);
    if (c !== CANAL_STREET) tryAddSpawn(p + 4.5, 0.25, c + 7);
  }
}
for (let x = -30; x <= 30; x += 7.5) {
  for (let z = -30; z <= 30; z += 7.5) {
    tryAddSpawn(x, 0.25, z);
  }
}
// 屋顶出生点：扫描低层塔顶
for (const b of blocks) {
  if (rooftopSpawns >= 20) break;
  if (b.t !== T.GLASS && b.t !== T.CONCRETE) continue;
  if (b.y + b.h < 8 || b.y + b.h > 20) continue;
  const cx = b.x + b.w / 2;
  const cz = b.z + b.d / 2;
  if (tryAddSpawn(cx, b.y + b.h + 0.1, cz)) rooftopSpawns++;
}

// 兜底：街道网格补齐到 220
for (let x = -HALF + 20; x <= HALF - 20 && spawns.length < 220; x += 6) {
  for (let z = -HALF + 20; z <= HALF - 20 && spawns.length < 220; z += 6) {
    if (streetCenters.some((c) => Math.abs(z - c) < STREET / 2 + 2 && Math.abs(x - CANAL_STREET) < STREET / 2 + 2)) continue;
    tryAddSpawn(x, 0.25, z);
  }
}

const mapData = { size: [SIZE, SIZE], blocks, spawns };
const fs = await import('node:fs');
const out = new URL('../maps/neon-city.json', import.meta.url);
fs.mkdirSync(new URL('../maps/', import.meta.url), { recursive: true });
fs.writeFileSync(out, JSON.stringify(mapData, null, 2));
const crypto = await import('node:crypto');
const revision = crypto.createHash('sha256').update(fs.readFileSync(out)).digest().readUInt32LE(0);
console.log(`Neon City generated: ${blocks.length} blocks, ${spawns.length} spawns`);
console.log(`Revision: 0x${revision.toString(16).padStart(8, '0')}`);
