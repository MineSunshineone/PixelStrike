package main

import (
	"fmt"
	"math"
)

// 《据点战争》DLC：A/B/C 三据点占领积分赛。
// 单独一人在据点内持续站立即开始「巩固」；巩固满 10 秒获得一次
// 大招点奖励并播报；多人同处则进入争夺冻结。零协议改动。

const (
	zoneRadius        = 7.0           // 据点半径（水平）
	zoneHoldTicks     = 10 * TickRate // 巩固一轮所需 tick（10 秒）
	zoneRewardHP      = 25            // 每轮巩固奖励回血
	zoneRewardUlt     = 1             // 每轮巩固奖励大招点
	zoneCenterBlocked = -1.0          // 中心被墙占用的死区标记
)

// 地图无关的三个据点位（CanOccupy 判定为墙时该据点自动休眠）。
var zoneCenters = [3]Vec3{
	{X: -96, Z: -96},
	{X: 0, Z: 160},
	{X: 96, Z: 96},
}

var zoneNames = [3]string{"A", "B", "C"}

// stepZones：每 tick 扫描三个据点的占员情况（3×N，开销可忽略）。
func (r *Room) stepZones() {
	for zi := range zoneCenters {
		center := zoneCenters[zi]
		if !r.World.CanOccupy(Vec3{X: center.X, Y: 0.1, Z: center.Z}, StandingHeight) {
			continue // 中心被墙体占用：据点休眠
		}
		occupant := (*PlayerState)(nil)
		contested := false
		for _, pl := range r.Players {
			p := &pl.PlayerState
			if !p.Alive || !p.OnGround {
				continue
			}
			dx, dz := p.Pos.X-center.X, p.Pos.Z-center.Z
			if dx*dx+dz*dz > zoneRadius*zoneRadius {
				continue
			}
			if occupant == nil {
				occupant = p
			} else {
				contested = true
			}
		}
		if contested || occupant == nil {
			continue // 争夺中/无人：冻结不倒退
		}
		if r.zoneOwners[zi] != occupant.Id {
			// 换人占领：重置巩固进度并播报首占。
			r.zoneOwners[zi] = occupant.Id
			r.zoneHoldTicks[zi] = 0
			if r.hasZoneAudience() {
				r.Emit(Event{Type: EvChat, Player: 0, Name: "战场播报",
					Message: fmt.Sprintf("🏛 %s 点已被 %s 占领，站稳 10 秒领取奖励！", zoneNames[zi], occupant.Name)})
			}
			continue
		}
		r.zoneHoldTicks[zi]++
		if r.zoneHoldTicks[zi] >= zoneHoldTicks {
			r.zoneHoldTicks[zi] = 0
			if occupant.Ultimate == 0 {
				occupant.UltimatePoints = uint8(min(UltimateRequirement, int(occupant.UltimatePoints)+zoneRewardUlt))
			}
			occupant.HP = uint8(min(occupant.maxHP(), int(occupant.HP)+zoneRewardHP))
			if r.hasZoneAudience() {
				r.Emit(Event{Type: EvChat, Player: 0, Name: "战场播报",
					Message: fmt.Sprintf("🏛 %s 巩固了 %s 点（+%d 大招点 +%d HP）", occupant.Name, zoneNames[zi], zoneRewardUlt, zoneRewardHP)})
			}
		}
	}
}

func (r *Room) hasZoneAudience() bool {
	for _, pl := range r.Players {
		if !pl.IsBot {
			return true
		}
	}
	return false
}

// 供测试：把一名玩家放到据点内。
func zoneDistance(p Vec3, zi int) float64 {
	return math.Hypot(p.X-zoneCenters[zi].X, p.Z-zoneCenters[zi].Z)
}
