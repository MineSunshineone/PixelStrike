package main

import (
	"math"
	"time"
)

// 《虫洞》DLC：纠缠传送门玩法。
// 战场两端各出现一扇光环虫洞，走进任何一扇，瞬间从另一扇穿出。
// 零协议改动：传送本身只改服务端私有坐标（快照自然携带），
// 特效复用 EvExplosion 爆炸粒子，播报复用 EvChat。

const (
	wormholeRadius   = 1.5             // 触发半径（水平距离）
	wormholeHeight   = 2.0             // 触发纵向窗口（允许跳跃穿越）
	wormholeExitDist = 2.6             // 出口沿穿行方向距门体的落点（保证落在触发圈外，防原地打转）
	wormholeCooldown = 3 * time.Second // 每人冷却，防止两门间乒乓
)

// wormholePair 从出生点里挑相距最远、且能完整站立的两处作为纠缠门锚点。
// 纯几何 + 严格大于，遍历顺序只依赖地图 JSON 的出生点数组——
// 客户端 main.ts 的 wormholeAnchors 按同一算法逐位复刻，两侧选址天然一致。
func wormholePair(w *World) (a, b Vec3, ok bool) {
	best := -1.0
	for i := range w.Spawns {
		si := w.Spawns[i]
		pi := Vec3{si[0], si[1], si[2]}
		if !w.CanOccupy(pi, StandingHeight) {
			continue
		}
		for j := i + 1; j < len(w.Spawns); j++ {
			sj := w.Spawns[j]
			pj := Vec3{sj[0], sj[1], sj[2]}
			dx, dz := pi.X-pj.X, pi.Z-pj.Z
			if d := dx*dx + dz*dz; d > best && w.CanOccupy(pj, StandingHeight) {
				best = d
				a, b = pi, pj
			}
		}
	}
	return a, b, best > 0
}

func (r *Room) initWormholes() {
	a, b, ok := wormholePair(r.World)
	if !ok {
		return
	}
	r.Wormholes = [2]Vec3{a, b}
	r.wormholeOK = true
	r.wormholeCooldowns = make(map[uint16]time.Time)
}

// StepWormholes 每 tick 扫描存活玩家：站进任一扇门且冷却已过、
// 且不在出生保护期（出生点常与门体重合，保护期就是天然的下车通道）即传送。
// 首名玩家到场后全房播报一次玩法。
func (r *Room) StepWormholes(now time.Time) {
	if !r.wormholeOK {
		return
	}
	if !r.wormholeAnnounced {
		if len(r.Players) == 0 {
			return
		}
		r.wormholeAnnounced = true
		r.Emit(Event{Type: EvChat, Name: "🌀 时空乱流", Message: "一对纠缠虫洞已在战场两端张开——走进光环，瞬间从世界的另一头穿出！"})
	}
	for _, pl := range r.Players {
		p := &pl.PlayerState
		if !p.Alive || now.Before(p.InvincibleUntil) {
			continue
		}
		if until, ok := r.wormholeCooldowns[p.Id]; ok && now.Before(until) {
			continue
		}
		enter := -1
		for k, wh := range r.Wormholes {
			dx, dz := p.Pos.X-wh.X, p.Pos.Z-wh.Z
			if dx*dx+dz*dz <= wormholeRadius*wormholeRadius && math.Abs(p.Pos.Y-wh.Y) <= wormholeHeight {
				enter = k
				break
			}
		}
		if enter < 0 {
			continue
		}
		exit := r.wormholeExit(enter)
		r.Emit(Event{Type: EvExplosion, Origin: p.Pos})
		p.Pos = exit
		r.Emit(Event{Type: EvExplosion, Origin: exit})
		r.wormholeCooldowns[p.Id] = now.Add(wormholeCooldown)
	}
}

// wormholeExit 计算从 enter 号门穿出后的落点：沿两门连线方向穿出门体
// wormholeExitDist 米（虫洞的隧道语义：从哪边进，就顺着穿出去）；
// 落点被墙挡住则退回对门锚点，最后收敛在世界边界内。
func (r *Room) wormholeExit(enter int) Vec3 {
	enterPos, exitPos := r.Wormholes[enter], r.Wormholes[1-enter]
	dx, dz := exitPos.X-enterPos.X, exitPos.Z-enterPos.Z
	dist := math.Hypot(dx, dz)
	out := exitPos
	if dist > 0.001 {
		out.X += dx / dist * wormholeExitDist
		out.Z += dz / dist * wormholeExitDist
	}
	if !r.World.CanOccupy(out, StandingHeight) {
		out = exitPos
	}
	halfX, halfZ := r.World.Size[0]/2-PlayerHalf, r.World.Size[1]/2-PlayerHalf
	out.X = math.Max(-halfX, math.Min(halfX, out.X))
	out.Z = math.Max(-halfZ, math.Min(halfZ, out.Z))
	out.Y = exitPos.Y
	return out
}
