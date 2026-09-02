export const PROTOCOL_VERSION = 11;

export const OP = {
  Join: 0x01,
  Input: 0x02,
  Fire: 0x03,
  Reload: 0x04,
  Grenade: 0x06,
  Switch: 0x08,
  Loadout: 0x09,
  RosterRequest: 0x0a,
  ToggleFlight: 0x0b,
  Ultimate: 0x0c,
  Chat: 0x0d,
  Hex: 0x0e,
  Welcome: 0x81,
  Snapshot: 0x82,
  Events: 0x83,
  Pong: 0x84,
  Self: 0x86,
  Roster: 0x87,
  Reject: 0x88,
  Maintenance: 0x89,
  Ping: 0xf0,
} as const;

export interface MapBlock {
  x: number;
  y: number;
  z: number;
  w: number;
  h: number;
  d: number;
  t: number;
}

export interface MapData {
  size: [number, number];
  blocks: MapBlock[];
  spawns: [number, number, number][];
}

export interface PlayerSnap {
  id: number;
  x: number;
  y: number;
  z: number;
  yaw: number;
  pitch: number;
  vx: number;
  vz: number;
  hp: number;
  armor: number;
  state: number;
  weapon: number;
  shot: number;
  skin: number;
  weaponSkin: number;
  ultimate: number;
}

export interface RosterEntry {
  id: number;
  name: string;
  kills: number;
  deaths: number;
}

export interface WeaponDef {
  id: number;
  name: string;
  dmg: number;
  headMult: number;
  rpm: number;
  mag: number;
  reserve: number;
  reloadMs: number;
  speedMult: number;
  spread: number;
  moveSpread: number;
  bloom: number;
  adsFov: number;
  automatic: boolean;
  color: number;
  length: number;
  pellets?: number;
}

export const WEAPONS: WeaponDef[] = [
  { id: 0, name: 'Glock-18', dmg: 23, headMult: 3.0, rpm: 500, mag: 30, reserve: 180, reloadMs: 1400, speedMult: 1.0, spread: 0.28, moveSpread: 1.20, bloom: 0.08, adsFov: 67, automatic: false, color: 0x222225, length: 0.35 },
  { id: 1, name: 'Desert Eagle', dmg: 44, headMult: 2.45, rpm: 250, mag: 11, reserve: 53, reloadMs: 1800, speedMult: 0.98, spread: 0.25, moveSpread: 1.90, bloom: 0.28, adsFov: 62, automatic: false, color: 0x9b9890, length: 0.42 },
  { id: 2, name: 'MP5-SD', dmg: 22, headMult: 3.0, rpm: 820, mag: 45, reserve: 180, reloadMs: 1800, speedMult: 1.0, spread: 0.44, moveSpread: 1.32, bloom: 0.065, adsFov: 58, automatic: true, color: 0x34485a, length: 0.55 },
  { id: 3, name: 'AK-47', dmg: 33, headMult: 4.0, rpm: 600, mag: 45, reserve: 135, reloadMs: 2200, speedMult: 0.92, spread: 0.46, moveSpread: 1.85, bloom: 0.17, adsFov: 54, automatic: true, color: 0x79502f, length: 0.75 },
  { id: 4, name: 'M4A4', dmg: 29, headMult: 3.5, rpm: 690, mag: 45, reserve: 135, reloadMs: 2100, speedMult: 0.93, spread: 0.34, moveSpread: 1.55, bloom: 0.12, adsFov: 52, automatic: true, color: 0x3b463b, length: 0.75 },
  { id: 5, name: 'AWP', dmg: 103, headMult: 1.25, rpm: 32, mag: 8, reserve: 45, reloadMs: 2800, speedMult: 0.76, spread: 0.06, moveSpread: 4.80, bloom: 0, adsFov: 26, automatic: false, color: 0x35463a, length: 1.05 },
  { id: 6, name: 'Knife', dmg: 34, headMult: 1.0, rpm: 150, mag: 0, reserve: 0, reloadMs: 0, speedMult: 1.08, spread: 0, moveSpread: 0, bloom: 0, adsFov: 75, automatic: true, color: 0x777777, length: 0.3 },
  { id: 7, name: 'USP-S', dmg: 24, headMult: 3.6, rpm: 400, mag: 18, reserve: 36, reloadMs: 1700, speedMult: 1.0, spread: 0.24, moveSpread: 1.05, bloom: 0.10, adsFov: 64, automatic: false, color: 0x2a3340, length: 0.38 },
  { id: 8, name: 'UMP-45', dmg: 27, headMult: 2.8, rpm: 620, mag: 38, reserve: 150, reloadMs: 2100, speedMult: 0.97, spread: 0.48, moveSpread: 1.42, bloom: 0.085, adsFov: 56, automatic: true, color: 0x4a4036, length: 0.58 },
  { id: 9, name: 'FAMAS', dmg: 29, headMult: 3.2, rpm: 720, mag: 38, reserve: 135, reloadMs: 2200, speedMult: 0.94, spread: 0.38, moveSpread: 1.58, bloom: 0.135, adsFov: 52, automatic: true, color: 0x5a6848, length: 0.7 },
  { id: 10, name: 'AUG', dmg: 27, headMult: 3.5, rpm: 600, mag: 45, reserve: 135, reloadMs: 2300, speedMult: 0.88, spread: 0.28, moveSpread: 1.30, bloom: 0.10, adsFov: 40, automatic: true, color: 0x3d4a3a, length: 0.78 },
  { id: 11, name: 'SSG 08', dmg: 72, headMult: 2.0, rpm: 48, mag: 12, reserve: 120, reloadMs: 2600, speedMult: 0.80, spread: 0.07, moveSpread: 3.80, bloom: 0, adsFov: 30, automatic: false, color: 0x2f3a48, length: 1.0 },
  { id: 12, name: 'XM1014', dmg: 17, headMult: 1.2, rpm: 200, mag: 7, reserve: 32, reloadMs: 2100, speedMult: 0.96, spread: 1.72, moveSpread: 2.55, bloom: 0.12, adsFov: 64, automatic: true, color: 0x6a5a3a, length: 0.72, pellets: 8 },
];

export const isSniper = (id: number) => id === 5 || id === 11;
export const isPistol = (id: number) => id === 0 || id === 1 || id === 7;
export const isGun = (id: number) => id !== 6;
export const isShotgun = (id: number) => id === 12;

export const XM_PELLETS: readonly [number, number][] = [
  [0.18, -0.12],
  [1.00, 0.10],
  [0.62, 0.78],
  [-0.22, 0.98],
  [-0.92, 0.38],
  [-0.85, -0.52],
  [-0.08, -1.00],
  [0.78, -0.62],
];

export const ULTIMATE_REQUIREMENT = 7;
export const ULTIMATES = [
  { id: 1, key: 'Z', name: '黑梦', durationMs: 10000, description: '真人视线变黑，bot 停火 10 秒' },
  { id: 2, key: 'X', name: '无敌', durationMs: 15000, description: '不受任何伤害 15 秒' },
  { id: 3, key: 'V', name: '幽灵', durationMs: 10000, description: '隐身并提速 10 秒，弹道仍会暴露位置' },
  { id: 4, key: 'C', name: '荆棘甲', durationMs: 10000, description: '受击反弹 30% 伤害 10 秒' },
  { id: 5, key: 'B', name: '吸血', durationMs: 10000, description: '造成伤害回复一半 10 秒' },
  { id: 6, key: 'N', name: '狂暴', durationMs: 10000, description: '射速 ×2、移速 +30% 共 10 秒' },
] as const;
export const scopeSettleMs = (id: number) => id === 5 ? 320 : id === 11 ? 240 : 0;

// 海克斯强化卡：每条命死亡后 3 选 1（id 与 server/hex.go 的 HexCards 对齐）。
// 速度/散布/射速系数由本地镜像保证预测与服务端一致。
export const HEX_CARDS = [
  { id: 1, name: '疾行者', description: '移速 +15%' },
  { id: 2, name: '强袭', description: '伤害 +15%' },
  { id: 3, name: '狂热', description: '射速 +25%' },
  { id: 4, name: '稳定', description: '散布 -30%' },
  { id: 5, name: '铁壁', description: '受到伤害 -20%' },
  { id: 6, name: '利爪', description: '造成伤害 15% 回复生命' },
  { id: 7, name: '血牛', description: '血量上限 100→140，选卡立即 +40' },
  { id: 8, name: '弹药扩容', description: '两把武器备弹翻倍' },
  { id: 9, name: '快手', description: '换弹耗时 -30%' },
] as const;
export const hexCard = (id: number) => HEX_CARDS.find((c) => c.id === id);
export const HEX_SPEED_MULT = 1.15;
export const HEX_SPREAD_MULT = 0.70;
export const HEX_FIRE_RATE_DIV = 1.25;
export const HEX_RELOAD_MULT = 0.70;
export const HEX_BLOOD_OX_MAX_HP = 140;
// 卡牌 id（与 server/hex.go 对齐）：客户端预测需要镜像的几张
export const HEX_SPRINT = 1;
export const HEX_FRENZY = 3;
export const HEX_STEADY = 4;
export const HEX_BLOOD_OX = 7;
export const HEX_QUICKEN = 9;

// 快捷喊话预设：H 键 / 顶栏 ⚡ 打开，数字键或点按直接以聊天通道发送
export const QUICK_CHAT_PHRASES = [
  '收到 👍',
  '撤退！',
  '蹲这儿组队 🤝',
  '集合推中',
  '小心狙击手',
  '炸鸡在这 🍗',
] as const;

export const KEY = {
  Forward: 1,
  Back: 2,
  Left: 4,
  Right: 8,
  Jump: 16,
  Crouch: 32,
  Aim: 64,
  Descend: 128,
} as const;

export const PHYS = {
  walkSpeed: 6.4,
  sprintMultiplier: 1.3,
  groundAccel: 44,
  stopAccel: 60,
  airAccel: 9.5,
  // 月球跳：与服务端 sim.go 的 Gravity/JumpVel 保持逐位一致（预测不回弹）
  gravity: -15.0,
  jumpVel: 10.5,
  crouchSpeed: 0.6,
  eyeHeight: 1.7,
  crouchEye: 1.12,
  standingHeight: 2.1,
  crouchingHeight: 1.3,
  flightSpeed: 6.4,
  maxFlightHeight: 45,
};
