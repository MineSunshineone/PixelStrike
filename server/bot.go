package main

import (
	"math"
	"math/rand/v2"
	"time"
)

// Expanded 12 Bot roster across all sectors of the map
const maxBotSkill = 2

var botPrimaries = []uint8{3, 4, 2, 8, 9, 10, 12, 11}
var botSecondaries = []uint8{0, 7, 1}

func botSkill(id uint16) int {
	return int(id) % (maxBotSkill + 1)
}

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
	TargetDist   float64
	NextNadeAt   time.Time
	ShotSeq      uint16
	RevengeID    uint16
	RevengeUntil time.Time
	HearPos      Vec3
	HearUntil    time.Time
	NextGlanceAt time.Time
	GlanceUntil  time.Time
	GlanceYaw    float64
	LastHurtAt   time.Time
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
			primary := botPrimaries[i%len(botPrimaries)]
			secondary := botSecondaries[i%len(botSecondaries)]
			bot := &Player{
				PlayerState: PlayerState{
					Id:         id,
					Name:       name,
					HP:         MaxHP,
					Alive:      false,
					Primary:    primary,
					Secondary:  secondary,
					ActiveSlot: 1,
					Weapon:     primary,
					Skin:       uint8(i % int(SkinCount)),
					Mags:       [2]int{Weapons[primary].Mag, Weapons[secondary].Mag},
					Reserves:   [2]int{Weapons[primary].Reserve, Weapons[secondary].Reserve},
					Grenades:   1,
					IsBot:      true,
				},
				joined: true,
				Room:   r,
			}
			r.Players = append(r.Players, bot)
			r.botAIs[id] = &BotAI{
				NextWaypointAt: time.Now(),
				NextGlanceAt:   time.Now().Add(time.Duration(700+rand.IntN(2200)) * time.Millisecond),
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
		if mag <= 0 && p.ActiveSlot == 1 && p.Mags[1] > 0 {
			p.SwitchSlot(2)
			mag, _ = p.ActiveAmmo()
		} else if mag <= 0 && p.ActiveSlot == 2 && p.Mags[0] > 0 && p.Reserves[0] == 0 {
			p.SwitchSlot(1)
			mag, _ = p.ActiveAmmo()
		}
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
		if now.After(ai.RevengeUntil) {
			ai.RevengeID = 0
		}
		if (r.tick+uint32(p.Id))%7 == 0 {
			for _, other := range r.Players {
				if other.Id == p.Id || !other.Alive || other.ProtectedAt(now) || other.Crouch {
					continue
				}
				speed := math.Hypot(other.Vel.X, other.Vel.Z)
				if speed < 1.1 || !(other.OnGround || other.Flying) {
					continue
				}
				dx, dz := other.Pos.X-p.Pos.X, other.Pos.Z-p.Pos.Z
				if dx*dx+dz*dz > 18*18 {
					continue
				}
				ai.HearPos = other.Pos
				ai.HearUntil = now.Add(900 * time.Millisecond)
			}
		}
		if ai.Target == nil || (r.tick+uint32(p.Id))%12 == 0 {
			previousTarget := ai.Target
			ai.Target = nil
			bestDistSq := 24.0 * 24.0
			eyePos := Vec3{p.Pos.X, p.Pos.Y + EyeHeight, p.Pos.Z}
			huntingRevenge := ai.RevengeID != 0 && now.Before(ai.RevengeUntil)
			for i := range r.Players {
				other := &r.Players[i].PlayerState
				if other.Id == p.Id || !other.Alive || other.ProtectedAt(now) {
					continue
				}
				dx := other.Pos.X - p.Pos.X
				dz := other.Pos.Z - p.Pos.Z
				distSq := dx*dx + dz*dz
				revenge := huntingRevenge && other.Id == ai.RevengeID
				maxSq := bestDistSq
				if revenge {
					maxSq = 64.0 * 64.0
					ai.TargetPos = other.Pos
					ai.NextWaypointAt = now.Add(800 * time.Millisecond)
				}
				if distSq > maxSq {
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
				if dLen <= 0.001 {
					continue
				}
				dir.X /= dLen
				dir.Y /= dLen
				dir.Z /= dLen
				hit, hitDist := r.World.Raycast(eyePos, dir, dLen)
				if hit && hitDist < dLen-0.6 {
					continue
				}
				if revenge || distSq < bestDistSq {
					bestDistSq = distSq
					ai.Target = other
				}
			}
			ai.TargetDist = math.Sqrt(bestDistSq)
			if ai.Target != nil && ai.Target != previousTarget {
				skill := botSkill(p.Id)
				delay := 280 + rand.IntN(280) - skill*50
				if delay < 140 {
					delay = 140
				}
				if huntingRevenge && ai.Target.Id == ai.RevengeID {
					delay = 120 + rand.IntN(120)
				}
				ai.FireCooldown = now.Add(time.Duration(delay) * time.Millisecond)
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
			skill := botSkill(p.Id)
			turn := 0.18 + 0.05*float64(skill)
			if now.Sub(ai.LastHurtAt) < 700*time.Millisecond {
				fwdX, fwdZ := -math.Sin(p.Yaw), -math.Cos(p.Yaw)
				if dx*fwdX+dz*fwdZ < 0 {
					turn = math.Min(0.42, 0.32+0.05*float64(skill))
				}
			}
			p.Yaw += yawDiff * turn
			p.Pitch += (targetPitch - p.Pitch) * turn

			// Combat movement: strafe and advance/retreat
			if now.After(ai.NextStrafeAt) {
				ai.NextStrafeAt = now.Add(time.Duration(500+rand.IntN(700)) * time.Millisecond)
				if rand.Float64() < 0.5 {
					ai.StrafeDir = -1
				} else {
					ai.StrafeDir = 1
				}
			}

			if bestDist > 12 {
				moveKeys |= KeyForward
			} else if bestDist < 4 {
				moveKeys |= KeyBack
			}
			if bestDist > 10 && isGun(p.Weapon) && !isShotgun(p.Weapon) {
				moveKeys |= KeyAim
			}
			if bestDist > 8 && bestDist < 16 && skill >= 1 && rand.Float64() < 0.01 {
				moveKeys |= KeyCrouch
			}

			if ai.StrafeDir > 0 {
				moveKeys |= KeyRight
			} else if ai.StrafeDir < 0 {
				moveKeys |= KeyLeft
			}

			// Jump randomly to dodge fire
			if rand.Float64() < 0.012 && p.OnGround {
				moveKeys |= KeyJump
			}

			// Lob a grenade at mid-range enemies pinned behind cover
			if p.Grenades > 0 && now.After(ai.NextNadeAt) && bestDist > 5 && bestDist < 20 {
				gdx := targetEnemy.Pos.X - p.Pos.X
				gdz := targetEnemy.Pos.Z - p.Pos.Z
				gdy := (targetEnemy.Pos.Y + 1.0) - (p.Pos.Y + EyeHeight)
				gYaw := math.Atan2(-gdx, -gdz)
				gPitch := math.Atan2(gdy, math.Hypot(gdx, gdz)) + 0.24 // arc compensation
				r.ThrowGrenade(p, gYaw, gPitch, now)
				ai.NextNadeAt = now.Add(time.Duration(9+rand.IntN(8)) * time.Second)
			}

			// Fire weapon
			if !p.Reloading && mag > 0 && now.After(ai.FireCooldown) && math.Abs(yawDiff) < 0.20-0.03*float64(skill) {
				jitter := 0.14 - 0.025*float64(skill)
				aimYaw := p.Yaw + (rand.Float64()-0.5)*jitter
				aimPitch := p.Pitch + (rand.Float64()-0.5)*jitter*0.7
				mode := uint8(0)
				if moveKeys&KeyAim != 0 {
					mode |= 0x80
				}
				ai.ShotSeq++
				if r.TryFire(p, aimYaw, aimPitch, mode, r.tick, ai.ShotSeq, now) {
					w := Weapons[p.Weapon]
					ai.FireCooldown = now.Add(time.Duration(60000.0/w.Rpm*2.6+float64(120+rand.IntN(161))) * time.Millisecond)
				}
			}
		} else {
			hearing := now.Before(ai.HearUntil)
			if hearing {
				ai.TargetPos = ai.HearPos
				ai.NextWaypointAt = now.Add(400 * time.Millisecond)
			}
			waypointDX, waypointDZ := p.Pos.X-ai.TargetPos.X, p.Pos.Z-ai.TargetPos.Z
			if !hearing && (now.After(ai.NextWaypointAt) || waypointDX*waypointDX+waypointDZ*waypointDZ < 9) {
				ai.NextWaypointAt = now.Add(time.Duration(4+rand.IntN(6)) * time.Second)
				if ai.RevengeID != 0 && now.Before(ai.RevengeUntil) {
					if hunted := r.findPlayer(ai.RevengeID); hunted != nil && hunted.Alive {
						ai.TargetPos = hunted.Pos
					}
				} else if len(r.World.Spawns) > 0 {
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
			if now.After(ai.NextGlanceAt) && !hearing {
				sign := 1.0
				if rand.Float64() < 0.5 {
					sign = -1
				}
				ai.GlanceYaw = p.Yaw + sign*(2.2+rand.Float64()*0.7)
				ai.GlanceUntil = now.Add(280 * time.Millisecond)
				ai.NextGlanceAt = now.Add(time.Duration(2800+rand.IntN(4200)) * time.Millisecond)
			}
			if now.Before(ai.GlanceUntil) && !hearing {
				p.Yaw = yawToward(p.Yaw, ai.GlanceYaw, 0.28)
			} else if hearing {
				fwdX, fwdZ := -math.Sin(p.Yaw), -math.Cos(p.Yaw)
				if dx*fwdX+dz*fwdZ < 0.2 {
					p.Yaw = yawToward(p.Yaw, targetYaw, 0.34)
				} else {
					p.Yaw = yawToward(p.Yaw, targetYaw, 0.22)
				}
				moveKeys |= KeyForward
			} else {
				p.Yaw = yawToward(p.Yaw, targetYaw, 0.2)
				moveKeys |= KeyForward
			}
			p.Pitch = 0

			frontHit, _ := r.World.Raycast(Vec3{p.Pos.X, p.Pos.Y + 0.4, p.Pos.Z}, Vec3{-math.Sin(p.Yaw), 0, -math.Cos(p.Yaw)}, 1.5)
			if frontHit && p.OnGround {
				moveKeys |= KeyJump
			}
		}

		p.CmdKeys = moveKeys
	}
}

func yawToward(cur, want, rate float64) float64 {
	diff := want - cur
	for diff > math.Pi {
		diff -= 2 * math.Pi
	}
	for diff < -math.Pi {
		diff += 2 * math.Pi
	}
	return cur + diff*rate
}

func (r *Room) botKilled(victim, killer *PlayerState, now time.Time) {
	if victim == nil || killer == nil || !victim.IsBot || victim.Id == killer.Id {
		return
	}
	ai := r.botAIs[victim.Id]
	if ai == nil {
		return
	}
	ai.RevengeID = killer.Id
	ai.RevengeUntil = now.Add(22 * time.Second)
	ai.HearPos = killer.Pos
	ai.HearUntil = now.Add(4 * time.Second)
	ai.Target = nil
}

func (r *Room) botTookHit(victim, attacker *PlayerState, now time.Time) {
	if victim == nil || attacker == nil || !victim.IsBot {
		return
	}
	ai := r.botAIs[victim.Id]
	if ai == nil {
		return
	}
	ai.LastHurtAt = now
	ai.HearPos = attacker.Pos
	ai.HearUntil = now.Add(2 * time.Second)
}

func (r *Room) alertBotsSound(origin Vec3, shooter *PlayerState, now time.Time, radius float64) {
	rSq := radius * radius
	for _, pl := range r.Players {
		if !pl.IsBot || !pl.Alive || (shooter != nil && pl.Id == shooter.Id) {
			continue
		}
		dx, dz := pl.Pos.X-origin.X, pl.Pos.Z-origin.Z
		if dx*dx+dz*dz > rSq {
			continue
		}
		ai := r.botAIs[pl.Id]
		if ai == nil {
			continue
		}
		ai.HearPos = origin
		ai.HearUntil = now.Add(1400 * time.Millisecond)
	}
}
