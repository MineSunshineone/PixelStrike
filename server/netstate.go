package main

import (
	"math"
	"sort"
	"time"
)

const maxSnapshotBytes = 4096 // ~123 KB/s at 30 Hz, still far under the 100 Mbps budget.

type quantState struct {
	x, y, z                                          int16
	yaw, pitch                                       int16 // half-degrees
	vx, vz                                           int8  // decimetres/second
	hp, armor, state, weapon, shot, skin, weaponSkin uint8
	ultimate                                         uint8
}

func quantizeState(p *PlayerState, nowUnixNano int64) quantState {
	state := uint8(0)
	if p.Alive {
		state |= 1
	}
	if p.Alive && nowUnixNano < p.InvincibleUntil.UnixNano() {
		state |= 2
	}
	if p.Crouch {
		state |= 4
	}
	if p.OnGround {
		state |= 8
	}
	if p.CmdKeys&KeyAim != 0 {
		state |= 16
	}
	if p.Flying {
		state |= 32
	}
	if p.UltimateInvincibleAt(time.Unix(0, nowUnixNano)) {
		state |= 64
	}
	// bit128 = AI bot：让客户端的「真人/AI」徽章不依赖名字前缀。
	if p.IsBot {
		state |= 128
	}
	return quantState{
		x: q16(p.Pos.X * 100), y: q16(p.Pos.Y * 100), z: q16(p.Pos.Z * 100),
		yaw: angleHalfDeg(p.Yaw), pitch: angleHalfDeg(p.Pitch),
		vx: q8(p.Vel.X * 10), vz: q8(p.Vel.Z * 10),
		hp: p.HP, armor: p.Armor, state: state, weapon: p.Weapon, shot: uint8(p.LastShotSeq), skin: p.Skin, weaponSkin: p.WeaponSkin, ultimate: p.Ultimate,
	}
}

func q16(v float64) int16 { return int16(max(-32768, min(32767, int(math.Round(v))))) }
func q8(v float64) int8   { return int8(max(-128, min(127, int(math.Round(v))))) }
func angleHalfDeg(rad float64) int16 {
	deg2 := int(math.Round(rad * 360 / math.Pi))
	for deg2 > 360 {
		deg2 -= 720
	}
	for deg2 < -360 {
		deg2 += 720
	}
	return int16(deg2)
}

func quantizePlayers(dst []quantState, players []*Player, nowUnixNano int64) []quantState {
	if cap(dst) < len(players) {
		dst = make([]quantState, len(players))
	} else {
		dst = dst[:len(players)]
	}
	for i, player := range players {
		dst[i] = quantizeState(&player.PlayerState, nowUnixNano)
	}
	return dst
}

func (p *Player) BuildSnapshot(tick uint32, players []*Player, states []quantState, now time.Time) []byte {
	if p.netCache == nil {
		p.netCache = make(map[uint16]quantState)
		p.netFullAt = make(map[uint16]uint32)
	}
	if p.netReset.Swap(false) {
		clear(p.netCache)
		clear(p.netFullAt)
	}
	w := &Buf{b: p.snapshotBuffer(min(maxSnapshotBytes, 8+len(players)*23))}
	w.b[0] = OpSnapshot
	w.U32(tick)
	w.U16(p.LastInputSeq)
	w.U8(0)
	countAt := len(w.b) - 1
	count := 0

	type cand struct {
		i      int
		distSq float64
	}
	cands := make([]cand, 0, len(players))
	for i, other := range players {
		if ok, distSq := snapshotVisible(&p.PlayerState, &other.PlayerState, tick, now); ok {
			cands = append(cands, cand{i: i, distSq: distSq})
		}
	}
	sort.Slice(cands, func(a, b int) bool {
		if cands[a].distSq != cands[b].distSq {
			return cands[a].distSq < cands[b].distSq
		}
		return players[cands[a].i].Id < players[cands[b].i].Id
	})

	for _, c := range cands {
		if len(w.b) >= maxSnapshotBytes {
			break
		}
		other := players[c.i]
		cur := states[c.i]
		prev, seen := p.netCache[other.Id]
		full := !seen || tick-p.netFullAt[other.Id] >= 120
		if !appendStateDelta(w, other.Id, prev, cur, full) {
			continue
		}
		p.netCache[other.Id] = cur
		if full {
			p.netFullAt[other.Id] = tick
		}
		count++
	}
	w.b[countAt] = byte(count)
	if count == 0 {
		p.releaseSnapshot(w.b)
		return nil
	}
	return w.Bytes()
}

func snapshotVisible(self, other *PlayerState, tick uint32, now time.Time) (bool, float64) {
	if other.Id == self.Id {
		return true, -1
	}
	if self.BlackDreamAt(now) {
		return false, 0
	}
	dx := other.Pos.X - self.Pos.X
	dz := other.Pos.Z - self.Pos.Z
	distSq := dx*dx + dz*dz
	recentShot := !other.LastShotAt.IsZero() && time.Since(other.LastShotAt) < 800*time.Millisecond
	switch {
	case distSq <= 42*42:
		return true, distSq
	case distSq <= 100*100:
		return tick%4 == 0 || recentShot, distSq
	case distSq <= 220*220:
		if recentShot {
			return tick%4 == 0, distSq
		}
		moving := other.Vel.X*other.Vel.X+other.Vel.Z*other.Vel.Z > .0025
		if moving {
			return tick%8 == 0, distSq
		}
		return tick%20 == 0, distSq
	default:
		return recentShot && tick%8 == 0, distSq
	}
}

func (p *Player) snapshotBuffer(size int) []byte {
	if p.snapshotBuffers != nil {
		select {
		case buffer := <-p.snapshotBuffers:
			return buffer[:1]
		default:
		}
	}
	return make([]byte, 1, size)
}

func (p *Player) releaseSnapshot(buffer []byte) {
	if p.snapshotBuffers == nil {
		return
	}
	select {
	case p.snapshotBuffers <- buffer[:0]:
	default:
	}
}

func appendStateDelta(w *Buf, id uint16, prev, cur quantState, full bool) bool {
	var flag uint16
	dx := int(cur.x) - int(prev.x)
	dy := int(cur.y) - int(prev.y)
	dz := int(cur.z) - int(prev.z)
	dyaw := wrapHalfDeg(int(cur.yaw) - int(prev.yaw))
	dpitch := int(cur.pitch) - int(prev.pitch)
	var deltaPos, deltaAngles bool
	if !full {
		deltaPos = inI8(dx) && inI8(dy) && inI8(dz)
		deltaAngles = inI8(dyaw) && inI8(dpitch)
		if cur.x != prev.x || cur.y != prev.y || cur.z != prev.z {
			if deltaPos {
				flag |= 1 << 0
			} else {
				flag |= 1 << 1
			}
		}
		if cur.yaw != prev.yaw || cur.pitch != prev.pitch {
			if deltaAngles {
				flag |= 1 << 2
			} else {
				flag |= 1 << 3
			}
		}
		if cur.vx != prev.vx || cur.vz != prev.vz {
			flag |= 1 << 4
		}
		if cur.hp != prev.hp || cur.armor != prev.armor {
			flag |= 1 << 5
		}
		if cur.state != prev.state {
			flag |= 1 << 6
		}
		if cur.weapon != prev.weapon {
			flag |= 1 << 7
		}
		if cur.shot != prev.shot {
			flag |= 1 << 8
		}
		if cur.skin != prev.skin {
			flag |= 1 << 9
		}
		if cur.weaponSkin != prev.weaponSkin {
			flag |= 1 << 10
		}
		if cur.ultimate != prev.ultimate {
			flag |= 1 << 11
		}
	}
	if full || flag == 0 && (cur.x != prev.x || cur.y != prev.y || cur.z != prev.z || cur.yaw != prev.yaw || cur.pitch != prev.pitch) {
		flag = 1 << 15
	}
	if flag == 0 {
		return false
	}
	start := len(w.b)
	w.U16(id)
	w.U16(flag)
	if flag == 1<<15 {
		writeFullState(w, cur)
		if len(w.b) > maxSnapshotBytes {
			w.b = w.b[:start]
			return false
		}
		return true
	}
	if flag&(1<<0) != 0 {
		w.I8(int8(dx))
		w.I8(int8(dy))
		w.I8(int8(dz))
	}
	if flag&(1<<1) != 0 {
		w.I16(cur.x)
		w.I16(cur.y)
		w.I16(cur.z)
	}
	if flag&(1<<2) != 0 {
		w.I8(int8(dyaw))
		w.I8(int8(dpitch))
	}
	if flag&(1<<3) != 0 {
		w.I16(cur.yaw)
		w.I16(cur.pitch)
	}
	if flag&(1<<4) != 0 {
		w.I8(cur.vx)
		w.I8(cur.vz)
	}
	if flag&(1<<5) != 0 {
		w.U8(cur.hp)
		w.U8(cur.armor)
	}
	if flag&(1<<6) != 0 {
		w.U8(cur.state)
	}
	if flag&(1<<7) != 0 {
		w.U8(cur.weapon)
	}
	if flag&(1<<8) != 0 {
		w.U8(cur.shot)
	}
	if flag&(1<<9) != 0 {
		w.U8(cur.skin)
	}
	if flag&(1<<10) != 0 {
		w.U8(cur.weaponSkin)
	}
	if flag&(1<<11) != 0 {
		w.U8(cur.ultimate)
	}
	if len(w.b) > maxSnapshotBytes {
		w.b = w.b[:start]
		return false
	}
	return true
}

func writeFullState(w *Buf, s quantState) {
	w.I16(s.x)
	w.I16(s.y)
	w.I16(s.z)
	w.I16(s.yaw)
	w.I16(s.pitch)
	w.I8(s.vx)
	w.I8(s.vz)
	w.U8(s.hp)
	w.U8(s.armor)
	w.U8(s.state)
	w.U8(s.weapon)
	w.U8(s.shot)
	w.U8(s.skin)
	w.U8(s.weaponSkin)
	w.U8(s.ultimate)
}

func inI8(v int) bool { return v >= -128 && v <= 127 }
func wrapHalfDeg(v int) int {
	for v > 360 {
		v -= 720
	}
	for v < -360 {
		v += 720
	}
	return v
}
