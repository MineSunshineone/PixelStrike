package main

import (
	"math"
	"math/rand/v2"
	"time"
)

// 《天灾工厂》DLC：轮换天灾系统。每 120 秒随机降临一场天灾（3 秒预警 +
// 14 秒持续）：雷暴（脚下落雷 AoE）、流星雨（大半径轰击）、电磁风暴
//（枪械间歇性失灵）。伤害全部走既有 Damage/EvExplosion 管线，零协议改动。

const (
	stormFirst        = 120 * time.Second
	stormGap          = 120 * time.Second
	stormWarnDuration = 3 * time.Second
	stormDuration     = 14 * time.Second
	stormThunderR     = 3.5
	stormThunderDmg   = 60.0
	stormMeteorR      = 4.5
	stormMeteorDmg    = 70.0
)

const (
	StormNone uint8 = iota
	StormThunder
	StormMeteor
	StormEMP
)

// empJamChance 用 var 方便测试注入 100%。
var empJamChance = 0.35

// Room 侧字段见 room.go：stormNextAt / stormKind / stormEndsAt /
// stormNextStrikeAt / stormStrikesLeft / stormStrikePos。

func (r *Room) stepStorm(now time.Time) {
	if r.stormKind == StormNone {
		if r.stormNextAt.IsZero() {
			r.stormNextAt = now.Add(stormFirst)
			return
		}
		if !now.Before(r.stormNextAt) {
			kinds := []uint8{StormThunder, StormMeteor, StormEMP}
			r.stormKind = kinds[rand.IntN(len(kinds))]
			r.stormEndsAt = now.Add(stormWarnDuration + stormDuration)
			r.stormNextStrikeAt = now.Add(stormWarnDuration)
			r.stormStrikesLeft = map[uint8]int{StormThunder: 6, StormMeteor: 3, StormEMP: 0}[r.stormKind]
			r.stormAnnounce(map[uint8]string{
				StormThunder: "⚠ 天灾预警：雷暴即将来临，小心脚下的落雷！",
				StormMeteor:  "⚠ 天灾预警：流星雨来袭，抬头没有用，跑起来！",
				StormEMP:     "⚠ 天灾预警：电磁风暴将至，枪械将间歇性失灵！",
			}[r.stormKind])
		}
		return
	}
	if now.After(r.stormEndsAt) {
		kind := r.stormKind
		r.stormKind = StormNone
		r.stormNextAt = now.Add(stormGap)
		if r.hasStormAudience() {
			r.Emit(Event{Type: EvChat, Player: 0, Name: "战场播报", Message: map[uint8]string{
				StormThunder: "⚡ 雷暴过去了，别放松，下一场天灾在路上。",
				StormMeteor:  "☄ 流星雨结束了，捡一捡地上有没有陨铁。",
				StormEMP:     "📡 电磁风暴消散，枪械恢复准头。",
			}[kind]})
		}
		return
	}
	if now.Before(r.stormNextStrikeAt) || r.stormStrikesLeft <= 0 {
		return
	}
	switch r.stormKind {
	case StormThunder:
		r.stormStrikeAt(now, r.randomPlayerVicinity(), stormThunderR, stormThunderDmg, "⚡ 轰隆——！")
		r.stormNextStrikeAt = now.Add(2200 * time.Millisecond)
	case StormMeteor:
		r.stormStrikeAt(now, r.randomPlayerVicinity(), stormMeteorR, stormMeteorDmg, "☄ 流星坠落！")
		r.stormNextStrikeAt = now.Add(3500 * time.Millisecond)
	}
	r.stormStrikesLeft--
}

// randomPlayerVicinity：随机存活玩家周边 ±12m 的落点。
func (r *Room) randomPlayerVicinity() Vec3 {
	live := make([]*PlayerState, 0, 4)
	for _, pl := range r.Players {
		if pl.Alive {
			live = append(live, &pl.PlayerState)
		}
	}
	if len(live) == 0 {
		return Vec3{}
	}
	anchor := live[rand.IntN(len(live))]
	return Vec3{anchor.Pos.X + rand.Float64()*24 - 12, 0, anchor.Pos.Z + rand.Float64()*24 - 12}
}

// stormStrikeAt：在指定落点砸一击，AoE 伤害走 Damage（自雷语义）。
// 落点写进 stormStrikePos，测试可确定性断言。
func (r *Room) stormStrikeAt(now time.Time, pos Vec3, radius, dmg float64, shout string) {
	r.stormStrikePos = pos
	r.Emit(Event{Type: EvExplosion, Origin: pos})
	for _, pl := range r.Players {
		v := &pl.PlayerState
		if !v.Alive {
			continue
		}
		dx, dz := v.Pos.X-pos.X, v.Pos.Z-pos.Z
		d := dx*dx + dz*dz
		if d > radius*radius {
			continue
		}
		dist := math.Sqrt(d)
		r.Damage(v, v, dmg*(1-dist/radius), false, WeaponHE, now)
	}
	if r.hasStormAudience() {
		r.Emit(Event{Type: EvChat, Player: 0, Name: "战场播报", Message: shout})
	}
}

// empJam：电磁风暴持续期内枪械按概率失灵（TryFire 顶部调用）。
func (r *Room) empJam(now time.Time) bool {
	return r.stormKind == StormEMP &&
		now.Before(r.stormEndsAt) &&
		r.stormEndsAt.Sub(now) < stormDuration && // 预警期不 jam
		rand.Float64() < empJamChance
}

func (r *Room) stormAnnounce(msg string) {
	if r.hasStormAudience() {
		r.Emit(Event{Type: EvChat, Player: 0, Name: "战场播报", Message: msg})
	}
}

func (r *Room) hasStormAudience() bool {
	for _, pl := range r.Players {
		if !pl.IsBot {
			return true
		}
	}
	return false
}


