package main

import (
	"math"
	"math/rand/v2"
	"time"
)

// Expanded 12 Bot roster across all sectors of the map
var BotNames = []string{
	"[BOT] Phoenix",
	"[BOT] Hunter",
	"[BOT] Viper",
	"[BOT] Ghost",
	"[BOT] Maverick",
	"[BOT] Raven",
	"[BOT] Striker",
	"[BOT] Valkyrie",
	"[BOT] Apex",
	"[BOT] Shadow",
	"[BOT] Frost",
	"[BOT] Titan",
}

type BotAI struct {
	TargetPos      Vec3
	LastPos        Vec3
	StuckCount     int
	NextWaypointAt time.Time
	StrafeDir      float64
	NextStrafeAt   time.Time
	FireCooldown   time.Time
	// Target caches the last LOS-verified enemy between (staggered) scans.
	Target *PlayerState
	// TargetDist is the horizontal distance to Target at scan time.
	TargetDist float64
	NextNadeAt time.Time
	ShotSeq    uint16
}

func (r *Room) SetBotCount(count int) {
	if count < 0 {
		count = 0
	}
	if count > len(BotNames) {
		count = len(BotNames)
	}

	// Count existing bots
	var currentBots []*Player
	var humanPlayers []*Player
	for _, pl := range r.Players {
		if pl.IsBot {
			currentBots = append(currentBots, pl)
		} else {
			humanPlayers = append(humanPlayers, pl)
		}
	}
	count = min(count, RoomCap-len(humanPlayers))

	// If already exact, return
	if len(currentBots) == count {
		return
	}

	if len(currentBots) > count {
		// Trim excess bots
		for _, bot := range currentBots[count:] {
			delete(r.botAIs, bot.Id)
			delete(r.history, bot.Id)
			for _, other := range r.Players {
				delete(other.netCache, bot.Id)
				delete(other.netFullAt, bot.Id)
			}
			r.Emit(Event{Type: EvPlayerLeave, Player: bot.Id})
		}
		r.Players = append(humanPlayers, currentBots[:count]...)
	} else {
		// Spawn more bots
		for i := len(currentBots); i < count; i++ {
			name := BotNames[i]
			id := r.allocPlayerID()
			bot := &Player{
				PlayerState: PlayerState{
					Id:         id,
					Name:       name,
					HP:         MaxHP,
					Alive:      false,
					Primary:    3,
					Secondary:  0,
					ActiveSlot: 1,
					Weapon:     3,
					Skin:       uint8(i % int(SkinCount)),
					Mags:       [2]int{Weapons[3].Mag, Weapons[0].Mag},
					Reserves:   [2]int{Weapons[3].Reserve, Weapons[0].Reserve},
					Grenades:   1,
					IsBot:      true,
				},
				joined: true,
				Room:   r,
			}
			r.Players = append(r.Players, bot)
			r.botAIs[id] = &BotAI{
				NextWaypointAt: time.Now(),
			}
			r.Respawn(&bot.PlayerState, time.Now())
			r.Emit(Event{Type: EvPlayerName, Player: bot.Id, Name: bot.Name})
		}
	}
}

// StepBots updates AI behavior for all bots in the room. Called during Room.Step.
func (r *Room) StepBots(now time.Time) {
	for _, pl := range r.Players {
		if !pl.IsBot || !pl.Alive {
			continue
		}
		ai, ok := r.botAIs[pl.Id]
		if !ok {
			ai = &BotAI{NextWaypointAt: now}
			r.botAIs[pl.Id] = ai
		}

		p := &pl.PlayerState

		mag, _ := p.ActiveAmmo()
		// 1. Check if need reload
		if mag <= 0 && !p.Reloading {
			r.StartReload(p, now)
		}
		// 1b. Restock one grenade a while after the last throw so bots keep
		// using them across a long match.
		if p.Grenades == 0 && now.After(ai.NextNadeAt.Add(25*time.Second)) {
			p.Grenades = 1
		}

		// Scan for nearby visible enemy players (human or other bots).
		// Full LOS raycasts run at 1/3 rate (staggered per bot) to keep tick
		// cost flat with many bots; the cached target is chased between scans.
		if ai.Target != nil {
			stillHere := false
			for _, other := range r.Players {
				if &other.PlayerState == ai.Target {
					stillHere = true
					break
				}
			}
			if !stillHere || !ai.Target.Alive || ai.Target.ProtectedAt(now) {
				ai.Target = nil
				ai.TargetDist = 0
			}
		}
		if ai.Target == nil || (r.tick+uint32(p.Id))%12 == 0 {
			previousTarget := ai.Target
			ai.Target = nil
			bestDistSq := 24.0 * 24.0
			eyePos := Vec3{p.Pos.X, p.Pos.Y + EyeHeight, p.Pos.Z}
			for _, other := range r.Players {
				if other.Id == p.Id || !other.Alive || other.ProtectedAt(now) {
					continue
				}
				dx := other.Pos.X - p.Pos.X
				dz := other.Pos.Z - p.Pos.Z
				distSq := dx*dx + dz*dz
				if distSq > bestDistSq {
					continue
				}
				targetEye := Vec3{other.Pos.X, other.Pos.Y + EyeHeight*0.82, other.Pos.Z}
				if dx*dx+dz*dz > 0.0001 {
					forward := (-math.Sin(p.Yaw)*dx - math.Cos(p.Yaw)*dz) / math.Sqrt(dx*dx+dz*dz)
					if forward < math.Cos(100.0*math.Pi/360.0) {
						continue
					}
				}
				dir := Vec3{targetEye.X - eyePos.X, targetEye.Y - eyePos.Y, targetEye.Z - eyePos.Z}
				dLen := math.Sqrt(dir.X*dir.X + dir.Y*dir.Y + dir.Z*dir.Z)
				if dLen > 0.001 {
					dir.X /= dLen
					dir.Y /= dLen
					dir.Z /= dLen
					hit, hitDist := r.World.Raycast(eyePos, dir, dLen)
					if !hit || hitDist >= dLen-0.6 {
						bestDistSq = distSq
						ai.Target = &other.PlayerState
					}
				}
			}
			ai.TargetDist = math.Sqrt(bestDistSq)
			if ai.Target != nil && ai.Target != previousTarget {
				ai.FireCooldown = now.Add(time.Duration(450+rand.IntN(451)) * time.Millisecond)
			}
		}
		targetEnemy := ai.Target
		bestDist := ai.TargetDist

		// 4. Stuck Detection & Auto-Unstuck routine
		lastDX, lastDZ := p.Pos.X-ai.LastPos.X, p.Pos.Z-ai.LastPos.Z
		ai.LastPos = p.Pos
		if lastDX*lastDX+lastDZ*lastDZ < 0.0036 {
			ai.StuckCount++
		} else {
			ai.StuckCount = 0
		}

		var moveKeys uint8 = 0

		if ai.StuckCount >= 10 {
			// Bot is stuck against a wall or obstacle: jump and choose a distant waypoint
			moveKeys |= KeyJump
			if rand.Float64() < 0.5 {
				moveKeys |= KeyLeft
			} else {
				moveKeys |= KeyRight
			}
			if ai.StuckCount >= 20 {
				// Pick a fresh random waypoint across the entire map
				if len(r.World.Spawns) > 0 {
					sp := r.World.Spawns[rand.IntN(len(r.World.Spawns))]
					ai.TargetPos = Vec3{sp[0], sp[1], sp[2]}
				}
				ai.StuckCount = 0
				p.Yaw += (rand.Float64() - 0.5) * math.Pi
			}
		}

		// 5. Combat / Movement AI state machine
		if targetEnemy != nil {
			// Face enemy with smooth human-like aiming
			dx := targetEnemy.Pos.X - p.Pos.X
			dz := targetEnemy.Pos.Z - p.Pos.Z
			dy := (targetEnemy.Pos.Y + 1.0) - (p.Pos.Y + EyeHeight)
			targetYaw := math.Atan2(-dx, -dz)
			targetPitch := math.Atan2(dy, math.Hypot(dx, dz))

			yawDiff := targetYaw - p.Yaw
			for yawDiff > math.Pi {
				yawDiff -= 2 * math.Pi
			}
			for yawDiff < -math.Pi {
				yawDiff += 2 * math.Pi
			}
			p.Yaw += yawDiff * 0.12
			p.Pitch += (targetPitch - p.Pitch) * 0.12

			// Combat movement: strafe and advance/retreat
			if now.After(ai.NextStrafeAt) {
				ai.NextStrafeAt = now.Add(time.Duration(500+rand.IntN(700)) * time.Millisecond)
				if rand.Float64() < 0.5 {
					ai.StrafeDir = -1
				} else {
					ai.StrafeDir = 1
				}
			}

			if bestDist > 14 {
				moveKeys |= KeyForward
			} else if bestDist < 5 {
				moveKeys |= KeyBack
			}

			if ai.StrafeDir > 0 {
				moveKeys |= KeyRight
			} else if ai.StrafeDir < 0 {
				moveKeys |= KeyLeft
			}

			// Jump randomly to dodge fire
			if rand.Float64() < 0.008 && p.OnGround {
				moveKeys |= KeyJump
			}

			// Lob a grenade at mid-range enemies pinned behind cover
			if p.Grenades > 0 && now.After(ai.NextNadeAt) && bestDist > 6 && bestDist < 18 {
				gdx := targetEnemy.Pos.X - p.Pos.X
				gdz := targetEnemy.Pos.Z - p.Pos.Z
				gdy := (targetEnemy.Pos.Y + 1.0) - (p.Pos.Y + EyeHeight)
				gYaw := math.Atan2(-gdx, -gdz)
				gPitch := math.Atan2(gdy, math.Hypot(gdx, gdz)) + 0.24 // arc compensation
				r.ThrowGrenade(p, gYaw, gPitch, now)
				ai.NextNadeAt = now.Add(time.Duration(9+rand.IntN(8)) * time.Second)
			}

			// Fire weapon
			if !p.Reloading && mag > 0 && now.After(ai.FireCooldown) {
				aimYaw := p.Yaw + (rand.Float64()-0.5)*0.14
				aimPitch := p.Pitch + (rand.Float64()-0.5)*0.1
				ai.ShotSeq++
				if r.TryFire(p, aimYaw, aimPitch, 0, r.tick, ai.ShotSeq, now) {
					w := Weapons[p.Weapon]
					ai.FireCooldown = now.Add(time.Duration(60000.0/w.Rpm*2.6+float64(120+rand.IntN(161))) * time.Millisecond)
				}
			}
		} else {
			// Patrol mode: navigate across whole map
			waypointDX, waypointDZ := p.Pos.X-ai.TargetPos.X, p.Pos.Z-ai.TargetPos.Z
			if now.After(ai.NextWaypointAt) || waypointDX*waypointDX+waypointDZ*waypointDZ < 9 {
				ai.NextWaypointAt = now.Add(time.Duration(4+rand.IntN(6)) * time.Second)
				if len(r.World.Spawns) > 0 {
					// 65% chance to patrol central engagement zone
					if rand.Float64() < 0.65 {
						ai.TargetPos = Vec3{(rand.Float64() - 0.5) * 48.0, 0, (rand.Float64() - 0.5) * 48.0}
					} else {
						sp := r.World.Spawns[rand.IntN(len(r.World.Spawns))]
						ai.TargetPos = Vec3{sp[0], sp[1], sp[2]}
					}
				}
			}

			dx := ai.TargetPos.X - p.Pos.X
			dz := ai.TargetPos.Z - p.Pos.Z
			targetYaw := math.Atan2(-dx, -dz)
			p.Yaw += (targetYaw - p.Yaw) * 0.2
			p.Pitch = 0

			moveKeys |= KeyForward

			// Multi-ray obstacle avoidance (front, left 30°, right 30°)
			frontHit, _ := r.World.Raycast(Vec3{p.Pos.X, p.Pos.Y + 0.4, p.Pos.Z}, Vec3{-math.Sin(p.Yaw), 0, -math.Cos(p.Yaw)}, 1.5)
			if frontHit && p.OnGround {
				moveKeys |= KeyJump
			}
		}

		p.CmdKeys = moveKeys
	}
}
