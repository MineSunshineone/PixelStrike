package main

import (
	"fmt"
	"math"
	"math/rand/v2"
	"time"
)

// 《血月围城》DLC：血月周期性升起，僵尸潮从地图边缘涌向所有活物。
// 僵尸是非玩家实体（ID 段 500+），位置同步复用鸡事件的线格式（新 opcode），
// 玩家可以射杀它们换回血奖励，被咬死则由僵尸拿击杀 credit。

const (
	zombieHitHeight    = 1.7               // 僵尸 AABB 高度（弹道判定）
	zombieSpeed        = 3.6               // units/s，比人慢——跑得动就死不了
	zombieDmg          = 12                // 每口伤害
	zombieBiteRange    = 1.2               // 水平咬合距离
	zombieBiteCooldown = 700 * time.Millisecond
	zombieHP           = 80                // AK 三枪、手枪四枪
	zombieBaseWave     = 10                // 首波规模，之后每波 +2
	zombieMaxPerWave   = 24
	bloodMoonFirst     = 150 * time.Second  // 开局到首次血月
	bloodMoonDuration  = 50 * time.Second   // 每场血月时长
	bloodMoonGap       = 210 * time.Second  // 月落到下一次月升
	zombieKillReward   = 25                 // 击杀僵尸回血
	zombieIdBase       = 500                // 僵尸 ID 段（与玩家 ID 空间隔离）
)

type Zombie struct {
	Id          uint16
	Pos, Dir    Vec3
	HP          uint16
	Alive       bool
	NextBiteAt  time.Time
	lastEmitPos Vec3
	lastEmitAt  time.Time
	forceEmit   bool
}

// stepZombies：血月调度 + 僵尸 AI 主循环（追击/移动/咬人/位置同步）。
func (r *Room) stepZombies(now time.Time) {
	r.stepBloodMoon(now)
	if len(r.Zombies) == 0 {
		return
	}
	moonActive := now.Before(r.bloodMoonUntil)
	for i := range r.Zombies {
		z := &r.Zombies[i]
		if !z.Alive || !moonActive {
			continue
		}
		// 追击最近的存活玩家（真人 bot 一视同仁）。
		var target *PlayerState
		bestD2 := math.MaxFloat64
		for _, pl := range r.Players {
			o := &pl.PlayerState
			if !o.Alive {
				continue
			}
			dx, dz := o.Pos.X-z.Pos.X, o.Pos.Z-z.Pos.Z
			if d2 := dx*dx + dz*dz; d2 < bestD2 {
				target, bestD2 = o, d2
			}
		}
		if target == nil {
			continue
		}
		z.Dir = norm(Vec3{target.Pos.X - z.Pos.X, 0, target.Pos.Z - z.Pos.Z})
		step := zombieSpeed * TickDT
		next := Vec3{z.Pos.X + z.Dir.X*step, z.Pos.Y, z.Pos.Z + z.Dir.Z*step}
		headY := next.Y + zombieHitHeight*0.6
		blocked := false
		if hit, dist := r.World.Raycast(Vec3{next.X, headY, next.Z}, z.Dir, step+0.4); hit && dist <= step {
			blocked = true // 撞墙：本 tick 原地挠墙，下 tick 目标移动自然会改变方向
		}
		if !blocked {
			if hitDown, dDown := r.World.Raycast(Vec3{next.X, next.Y + 1.2, next.Z}, Vec3{0, -1, 0}, 2.4); hitDown {
				next.Y = next.Y + 1.2 - dDown // 贴地
			}
			z.Pos = next
		}
		// 咬合：贴脸且冷却完毕就下嘴（出生保护期内咬不动）。
		dx, dz := target.Pos.X-z.Pos.X, target.Pos.Z-z.Pos.Z
		if dx*dx+dz*dz <= zombieBiteRange*zombieBiteRange && now.After(z.NextBiteAt) && !target.ProtectedAt(now) {
			z.NextBiteAt = now.Add(zombieBiteCooldown)
			r.zombieBite(z, target, now)
		}
		// 位置同步：复用鸡的三段式策略（强制/位移超阈值/心跳）。
		if z.forceEmit {
			r.Emit(Event{Type: EvZombieSpawn, Player: z.Id, Origin: z.Pos, Dir: z.Dir})
			z.lastEmitPos, z.lastEmitAt, z.forceEmit = z.Pos, now, false
		} else if !blocked {
			dx, dz := z.Pos.X-z.lastEmitPos.X, z.Pos.Z-z.lastEmitPos.Z
			if dx*dx+dz*dz > 0.25 {
				r.Emit(Event{Type: EvZombieSpawn, Player: z.Id, Origin: z.Pos, Dir: z.Dir})
				z.lastEmitPos, z.lastEmitAt = z.Pos, now
			}
		} else if now.Sub(z.lastEmitAt) >= 5*time.Second {
			r.Emit(Event{Type: EvZombieSpawn, Player: z.Id, Origin: z.Pos, Dir: z.Dir})
			z.lastEmitPos, z.lastEmitAt = z.Pos, now
		}
	}
}

// stepBloodMoon：月升刷潮 → 月落晨光净化残尸并清场。
func (r *Room) stepBloodMoon(now time.Time) {
	if r.bloodMoonUntil.IsZero() {
		if r.nextBloodMoonAt.IsZero() {
			r.nextBloodMoonAt = now.Add(bloodMoonFirst)
			return
		}
		if !now.Before(r.nextBloodMoonAt) {
			r.bloodMoonUntil = now.Add(bloodMoonDuration)
			r.zombieWave++
			r.spawnZombieWave(now)
			if r.hasZombieAudience() {
				r.Emit(Event{Type: EvChat, Player: 0, Name: "战场播报",
					Message: fmt.Sprintf("🌕 血月升起！第 %d 波僵尸围城（%d 只，持续 %d 秒）——撑住！", r.zombieWave, min(zombieBaseWave+int(r.zombieWave-1)*2, zombieMaxPerWave), int(bloodMoonDuration.Seconds()))})
			}
		}
		return
	}
	if now.Before(r.bloodMoonUntil) {
		return
	}
	// 月落：晨光净化所有残尸。
	for i := range r.Zombies {
		if z := &r.Zombies[i]; z.Alive {
			z.Alive = false
			r.Emit(Event{Type: EvZombieDeath, Killer: 0, Victim: z.Id, Origin: z.Pos, Weapon: 0})
		}
	}
	r.Zombies = r.Zombies[:0]
	r.bloodMoonUntil = time.Time{}
	r.nextBloodMoonAt = now.Add(bloodMoonGap)
	if r.hasZombieAudience() {
		r.Emit(Event{Type: EvChat, Player: 0, Name: "战场播报", Message: "🌅 黎明降临，残存的僵尸在晨光中化灰。下一场血月还会再来。"})
	}
}

func (r *Room) spawnZombieWave(now time.Time) {
	if r.nextZombieId < zombieIdBase {
		r.nextZombieId = zombieIdBase // 确保僵尸 ID 不与玩家 ID 空间重叠
	}
	count := min(zombieBaseWave+int(r.zombieWave-1)*2, zombieMaxPerWave)
	for range count {
		z := Zombie{Id: r.nextZombieId, HP: zombieHP, Alive: true, forceEmit: true}
		r.nextZombieId++
		z.Pos = r.zombieSpawnSpot()
		r.Zombies = append(r.Zombies, z)
		zp := &r.Zombies[len(r.Zombies)-1]
		r.Emit(Event{Type: EvPlayerName, Player: zp.Id, Name: "血月僵尸"})
		r.Emit(Event{Type: EvZombieSpawn, Player: zp.Id, Origin: zp.Pos, Dir: zp.Dir})
	}
}

// zombieSpawnSpot：地图边缘环上随机取点并向地面投影（找不到地面就回落原点）。
func (r *Room) zombieSpawnSpot() Vec3 {
	halfX := r.World.Size[0]/2 - 3
	halfZ := r.World.Size[1]/2 - 3
	for range 10 {
		side := rand.IntN(4)
		along := rand.Float64()*2 - 1
		var pos Vec3
		switch side {
		case 0:
			pos = Vec3{along * halfX, 0, -halfZ}
		case 1:
			pos = Vec3{along * halfX, 0, halfZ}
		case 2:
			pos = Vec3{-halfX, 0, along * halfZ}
		default:
			pos = Vec3{halfX, 0, along * halfZ}
		}
		if hit, d := r.World.Raycast(Vec3{pos.X, 4, pos.Z}, Vec3{0, -1, 0}, 8); hit {
			pos.Y = 4 - d
			return pos
		}
	}
	return Vec3{}
}

// zombieShot：弹道与僵尸 AABB 求交，命中给打击反馈，打空血掉回血奖励。
// 返回 true 表示弹道被该僵尸挡下（与 chickenShot 同语义）。
func (r *Room) zombieShot(p *PlayerState, origin, dir Vec3, wallDist, playerDist float64, weapon uint8, now time.Time) bool {
	var best *Zombie
	bestDist := min(wallDist, playerDist)
	for i := range r.Zombies {
		z := &r.Zombies[i]
		if !z.Alive {
			continue
		}
		if d, ok := RayPlayerAABBHeight(origin, dir, z.Pos, zombieHitHeight, bestDist); ok && d < bestDist {
			best, bestDist = z, d
		}
	}
	if best == nil {
		return false
	}
	dmg := Weapons[weapon].Dmg
	best.HP -= uint16(math.Min(dmg, float64(best.HP)))
	r.Emit(Event{Type: EvHit, Player: p.Id, Victim: best.Id, Dmg: uint8(math.Min(dmg, 255))})
	if best.HP > 0 {
		return true
	}
	best.Alive = false
	r.Emit(Event{Type: EvZombieDeath, Killer: p.Id, Victim: best.Id, Origin: best.Pos, Weapon: weapon})
	if int(p.HP) < p.maxHP() {
		p.HP = uint8(min(p.maxHP(), int(p.HP)+zombieKillReward))
	}
	return true
}

// zombieBite：僵尸咬一口；咬死则走独立的击杀结算（不占玩家连杀/羁绊统计）。
func (r *Room) zombieBite(z *Zombie, target *PlayerState, now time.Time) {
	d := uint8(zombieDmg)
	if target.HP <= d {
		target.HP = 0
		r.zombieBiteKill(z, target, now)
	} else {
		target.HP -= d
	}
	r.Emit(Event{Type: EvHit, Player: z.Id, Victim: target.Id, Dmg: d})
}

// zombieBiteKill：被僵尸咬死的最小死亡结算（对齐 Damage() 的关键字段）。
func (r *Room) zombieBiteKill(z *Zombie, victim *PlayerState, now time.Time) {
	victim.Alive, victim.Reloading = false, false
	victim.RespawnAt = now.Add(RespawnDelayS)
	victim.Deaths++
	victim.Streak = 0
	victim.UltimatePoints, victim.Ultimate = 0, 0
	victim.BlackDreamUntil, victim.InvincibleUntilUlt, victim.GhostUntil = time.Time{}, time.Time{}, time.Time{}
	victim.DmgUntil, victim.RecoilUntil, victim.SpeedUntil = time.Time{}, time.Time{}, time.Time{}
	victim.InvincibleUntil = time.Time{}
	r.BreakIllegalTeam(victim)
	if !victim.IsBot {
		if r.Store != nil {
			r.Store.Accumulate(victim.Account, 0, 1)
		}
	}
	victim.LastKiller = 0
	victim.KilledAt = now
	r.Emit(Event{Type: EvKill, Killer: z.Id, Victim: victim.Id, Weapon: 6, Headshot: 0})
}

func (r *Room) hasZombieAudience() bool {
	for _, pl := range r.Players {
		if !pl.IsBot {
			return true
		}
	}
	return false
}
