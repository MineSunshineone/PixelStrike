package main

import (
	"math/rand/v2"
	"time"
)

// 《空投战争》DLC：运输机周期性掠过战场，投下传奇补给箱（金色空投）。
// 箱子 45 秒无人认领则自动回收；抢到 = 满血满甲弹药全满 +2 大招点。
// 复用拾取实体与 EvPickupSpawn/Taken 事件，零协议改动。

const (
	airdropFirst        = 90 * time.Second
	airdropGapMin       = 120 * time.Second
	airdropGapMax       = 180 * time.Second
	airdropLinger       = 45 * time.Second
	airdropIdBase uint16 = 600 // 空投箱 ID 段（与玩家 1+、僵尸 500+ 隔离）
)

// Room 侧字段见 room.go：nextAirdropAt / airdropSeq / airdropDeadlines。

type airdropDeadline struct {
	pickupId  uint16
	expiresAt time.Time
}

// stepAirdrops：空投调度 + 无人认领的箱子到期回收。
func (r *Room) stepAirdrops(now time.Time) {
	if len(r.World.Spawns) == 0 {
		return // 无出生点数据（空测试世界）时不做空投
	}
	if r.nextAirdropAt.IsZero() {
		r.nextAirdropAt = now.Add(airdropFirst)
		return
	}
	// 到期回收（从尾往头删，避免迭代位移问题）。
	for i := len(r.airdropDeadlines) - 1; i >= 0; i-- {
		if now.Before(r.airdropDeadlines[i].expiresAt) {
			continue
		}
		id := r.airdropDeadlines[i].pickupId
		r.airdropDeadlines = append(r.airdropDeadlines[:i], r.airdropDeadlines[i+1:]...)
		// 箱子若已被拾取则不在 Pickups 里（拾取时置为永久休眠），回收仅对还在场的生效。
		for j := range r.Pickups {
			if r.Pickups[j].Id == id && r.Pickups[j].Active && r.Pickups[j].Kind == PickupAirdrop {
				r.Pickups[j].Active = false
				r.Pickups[j].RespawnAt = now.Add(100 * 365 * 24 * time.Hour)
				r.Emit(Event{Type: EvPickupTaken, Player: id, Victim: 0, Kind: PickupAirdrop, Origin: r.Pickups[j].Pos})
				break
			}
		}
	}
	if !now.Before(r.nextAirdropAt) {
		gap := airdropGapMin + time.Duration(rand.IntN(int((airdropGapMax-airdropGapMin)/time.Second)))*time.Second
		r.nextAirdropAt = now.Add(gap)
		if len(r.Pickups) > 64 {
			return // 拾取池异常膨胀时跳过本轮，保护快照体积
		}
		if r.hasAirdropAudience() {
			r.Emit(Event{Type: EvChat, Player: 0, Name: "战场播报",
				Message: "✈ 运输机正在接近！传奇补给即将空投——满血满甲弹药全满 +2 大招点，先到先得！"})
		}
		pos := r.randomPickupPosition()
		id := r.airdropSeq
		if id < airdropIdBase {
			id = airdropIdBase
		}
		r.airdropSeq = id + 1
		r.Pickups = append(r.Pickups, Pickup{Id: id, Kind: PickupAirdrop, Pos: pos, Active: true, RespawnAt: now.Add(100 * 365 * 24 * time.Hour)})
		r.airdropDeadlines = append(r.airdropDeadlines, airdropDeadline{pickupId: id, expiresAt: now.Add(airdropLinger)})
		r.Emit(Event{Type: EvPickupSpawn, Player: id, Kind: PickupAirdrop, Origin: pos})
	}
}

func (r *Room) hasAirdropAudience() bool {
	for _, pl := range r.Players {
		if !pl.IsBot {
			return true
		}
	}
	return false
}
