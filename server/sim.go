package main

import (
	"fmt"
	"log"
	"math"
	"math/rand/v2"
	"time"
)

const (
	KeyForward = 1 << iota
	KeyBack
	KeyLeft
	KeyRight
	KeyJump
	KeyCrouch
	KeyAim
	KeyDescend
)

type WeaponDef struct {
	Id                                                                          uint8
	Name                                                                        string
	Dmg, HeadMult, Rpm, SpreadDeg, MoveSpreadDeg, BloomDeg, SpeedMult, ArmorPen float64
	Mag, Reserve, ReloadMs                                                      int
	Automatic                                                                   bool
	Pellets                                                                     int
}

var Weapons = []WeaponDef{
	{0, "Glock-18", 23, 3.0, 500, .28, 1.20, .08, 1.0, .58, 30, 180, 1400, false, 1},
	{1, "Desert Eagle", 44, 2.45, 250, .25, 1.90, .28, .98, .93, 11, 53, 1800, false, 1},
	{2, "MP5-SD", 22, 3.0, 820, .44, 1.32, .065, 1.0, .65, 45, 180, 1800, true, 1},
	{3, "AK-47", 33, 4.0, 600, .46, 1.85, .17, .92, .78, 45, 135, 2200, true, 1},
	{4, "M4A4", 29, 3.5, 690, .34, 1.55, .12, .93, .72, 45, 135, 2100, true, 1},
	{5, "AWP", 103, 1.25, 32, .06, 4.80, 0, .76, .98, 8, 45, 2800, false, 1},
	{6, "Knife", 34, 1, 150, 0, 0, 0, 1.08, 1, 0, 0, 0, true, 1},
	{7, "USP-S", 24, 3.6, 400, .24, 1.05, .10, 1.0, .62, 18, 36, 1700, false, 1},
	{8, "UMP-45", 27, 2.8, 620, .48, 1.42, .085, .97, .72, 38, 150, 2100, true, 1},
	{9, "FAMAS", 29, 3.2, 720, .38, 1.58, .135, .94, .68, 38, 135, 2200, true, 1},
	{10, "AUG", 27, 3.5, 600, .28, 1.30, .10, .88, .75, 45, 135, 2300, true, 1},
	{11, "SSG 08", 72, 2.0, 48, .07, 3.80, 0, .80, .82, 12, 120, 2600, false, 1},
	{12, "XM1014", 17, 1.2, 200, 1.72, 2.55, .12, .96, .70, 7, 32, 2100, true, 8},
}

func isGun(id uint8) bool     { return int(id) < len(Weapons) && id != 6 }
func isSniper(id uint8) bool  { return id == 5 || id == 11 }
func isShotgun(id uint8) bool { return id == 12 }
func isPrimary(id uint8) bool {
	switch id {
	case 2, 3, 4, 5, 8, 9, 10, 11, 12:
		return true
	}
	return false
}
func isSecondary(id uint8) bool { return id == 0 || id == 1 || id == 7 }

const WeaponHE uint8 = 13

const (
	TickRate         = 60
	TickDT           = 1.0 / TickRate
	WalkSpeed        = 6.4
	SprintMultiplier = 1.3
	GroundAccel      = 44.0
	StopAccel        = 60.0
	AirAccel         = 9.5
	CrouchSpeed      = .6
	// 月球跳：重力 -30%、起跳 +25%，跳跃高度 1.6m → ≈3.7m。
	// 客户端预测常数（client/src/constants.ts 的 PHYS.gravity/jumpVel）必须同步修改。
	Gravity                     = -15.0
	JumpVel                     = 10.5
	MaxRewindTicks              = 8
	MaxHP                       = 100
	SpawnProtectS               = 2 * time.Second
	AWPScopeTime                = 320 * time.Millisecond
	RespawnDelayS               = 3 * time.Second
	KnifeSlashInterval          = 400 * time.Millisecond
	KnifeHeavyInterval          = time.Second
	GrenadeThrowSpeed           = 28.0
	GrenadeLift                 = 4.2
	EyeHeight                   = 1.7
	CrouchEyeH                  = 1.12
	StandingHeight              = 2.1
	CrouchingHeight             = 1.3
	FlightSpeed                 = WalkSpeed
	MaxFlightHeight             = StandingHeight * 25
	RevengeDeathThreshold uint8 = 10
	BondCoverRange              = 20.0
	BondAvengeWindow            = 30 * time.Second

	UltimateRequirement = 7
	UltimateBlackDream  = 1
	UltimateInvincible  = 2
	UltimateGhost       = 3
	UltimateThorns      = 4
	UltimateVampire     = 5
	UltimateRampage     = 6

	UltimateBlackDreamS = 10 * time.Second
	UltimateInvincibleS = 15 * time.Second
	UltimateGhostS      = 10 * time.Second
	UltimateAwakenS     = 10 * time.Second // 觉醒系大招统一时长

	GhostSpeedMultiplier = 1.45
	chatCooldown         = time.Second

	// 非法组队：在存活 bot 身边连续下蹲三次（4 秒窗口内）结为临时小队。
	IllegalTeamRange   = 5.5
	IllegalTeamPresses = 3
	IllegalTeamWindow  = 4 * time.Second

	maxChatRunes = 120
	maxChatBytes = 160
)

type PlayerState struct {
	Id                                                             uint16
	Name, Account                                                  string
	Pos, Vel                                                       Vec3
	Yaw, Pitch                                                     float64
	HP, Armor                                                      uint8
	Alive, IsBot, OnGround, Crouch, Flying                         bool
	Primary, Secondary, ActiveSlot, Weapon, Skin                   uint8
	PrimaryWeaponSkin, SecondaryWeaponSkin, WeaponSkin             uint8
	Mags                                                           [2]int
	Reserves                                                       [2]int
	CmdKeys                                                        uint8
	Reloading                                                      bool
	ReloadEnd, NextFire, InvincibleUntil, RespawnAt, NextGrenadeAt time.Time
	LandingUntil, AimStarted, SpeedUntil, DmgUntil, RecoilUntil    time.Time
	Streak                                                         uint8
	Grenades                                                       int
	UltimatePoints, Ultimate                                       uint8
	BlackDreamUntil, InvincibleUntilUlt, GhostUntil                time.Time
	Kills, Deaths                                                  uint16
	NoKillDeaths                                                   uint8
	RevengeReady, RevengeActive                                    bool
	RevengeShots                                                   uint8
	BondMate                                                       uint16
	IllegalMate                                                    uint16
	BondScore                                                      uint8
	LastKiller                                                     uint16
	KilledAt                                                       time.Time
	LastInputSeq, LastShotSeq                                      uint16
	HasShot                                                        bool
	LastShotAt                                                     time.Time
	NextChatAt                                                     time.Time
	ShotCounter                                                    uint8
	ChickenKills                                                   uint8
	ThornsUntil                                                    time.Time
	LifestealUntil                                                 time.Time
	RampageUntil                                                   time.Time
	HexOffer                                                       [HexOfferCount]uint8
	Hex                                                            uint8
	inputWindowStart                                               time.Time
	inputCount                                                     int
}

func (p *PlayerState) addBondScore() uint8 {
	if p.BondScore < 255 {
		p.BondScore++
	}
	return p.BondScore
}

func (p *PlayerState) ProtectedAt(now time.Time) bool {
	return p.Alive && now.Before(p.InvincibleUntil)
}

func (p *PlayerState) BlackDreamAt(now time.Time) bool {
	return p.Alive && now.Before(p.BlackDreamUntil)
}

func (p *PlayerState) UltimateInvincibleAt(now time.Time) bool {
	return p.Alive && now.Before(p.InvincibleUntilUlt)
}

func (p *PlayerState) GhostAt(now time.Time) bool {
	return p.Alive && now.Before(p.GhostUntil)
}

func (r *Room) AnyBlackDream(now time.Time) bool {
	for _, other := range r.Players {
		if (&other.PlayerState).BlackDreamAt(now) {
			return true
		}
	}
	return false
}
func (p *PlayerState) Height() float64 {
	if p.Crouch {
		return CrouchingHeight
	}
	return StandingHeight
}
func (p *PlayerState) ActiveAmmo() (int, int) {
	switch p.ActiveSlot {
	case 1:
		return p.Mags[0], p.Reserves[0]
	case 2:
		return p.Mags[1], p.Reserves[1]
	}
	return 0, 0
}
func (p *PlayerState) setActiveAmmo(mag, reserve int) {
	if p.ActiveSlot == 1 {
		p.Mags[0], p.Reserves[0] = mag, reserve
	} else if p.ActiveSlot == 2 {
		p.Mags[1], p.Reserves[1] = mag, reserve
	}
}
func validLoadout(primary, secondary uint8) bool {
	return isPrimary(primary) && isSecondary(secondary)
}
func (p *PlayerState) ApplyLoadout(primary, secondary uint8) {
	if !validLoadout(primary, secondary) {
		primary, secondary = 3, 0
	}
	p.Primary, p.Secondary = primary, secondary
	p.Mags = [2]int{Weapons[primary].Mag, Weapons[secondary].Mag}
	p.Reserves = [2]int{Weapons[primary].Reserve, Weapons[secondary].Reserve}
	p.ActiveSlot, p.Weapon, p.Grenades = 1, primary, 1
	p.WeaponSkin = p.PrimaryWeaponSkin
	p.Reloading = false
}
func (p *PlayerState) SwitchSlot(slot uint8) bool {
	var weapon uint8
	switch slot {
	case 1:
		weapon = p.Primary
		p.WeaponSkin = p.PrimaryWeaponSkin
	case 2:
		weapon = p.Secondary
		p.WeaponSkin = p.SecondaryWeaponSkin
	case 3:
		weapon = 6
		p.WeaponSkin = 0
	default:
		return false
	}
	if p.ActiveSlot == slot {
		return false
	}
	p.ActiveSlot, p.Weapon, p.Reloading, p.AimStarted = slot, weapon, false, time.Time{}
	p.ShotCounter = 0
	p.LastShotAt = time.Time{}
	p.NextFire = time.Now().Add(220 * time.Millisecond)
	return true
}

type poseSample struct {
	Tick   uint32
	Pos    Vec3
	Crouch bool
}

type poseHistory struct {
	samples [16]poseSample
	next    int
	count   int
}

func (r *Room) Step(now time.Time) {
	r.StepBots(now)
	r.StepGrenades(now)
	for _, pl := range r.Players {
		p := &pl.PlayerState
		if p.Ultimate != 0 {
			switch p.Ultimate {
			case UltimateBlackDream:
				if !p.BlackDreamAt(now) {
					p.Ultimate = 0
					r.Emit(Event{Type: EvUltimate, Player: p.Id, Kind: UltimateBlackDream})
				}
			case UltimateInvincible:
				if !p.UltimateInvincibleAt(now) {
					p.Ultimate = 0
					r.Emit(Event{Type: EvUltimate, Player: p.Id, Kind: UltimateInvincible})
				}
			case UltimateGhost:
				if !p.GhostAt(now) {
					p.Ultimate = 0
					r.Emit(Event{Type: EvUltimate, Player: p.Id, Kind: UltimateGhost})
				}
			case UltimateThorns:
				if !p.ThornsUntil.After(now) {
					p.Ultimate = 0
					r.Emit(Event{Type: EvUltimate, Player: p.Id, Kind: UltimateThorns})
				}
			case UltimateVampire:
				if !p.LifestealUntil.After(now) {
					p.Ultimate = 0
					r.Emit(Event{Type: EvUltimate, Player: p.Id, Kind: UltimateVampire})
				}
			case UltimateRampage:
				if !p.RampageUntil.After(now) {
					p.Ultimate = 0
					r.Emit(Event{Type: EvUltimate, Player: p.Id, Kind: UltimateRampage})
				}
			default:
				p.Ultimate = 0
			}
		}
		if !p.Alive {
			if !p.RespawnAt.IsZero() && !now.Before(p.RespawnAt) {
				r.Respawn(p, now)
			}
			continue
		}
		r.Move(p, now)
		r.CheckSanity(p)
	}
	r.StepPickups(now)
	r.stepAirdrops(now)
	r.StepChickens(now)
	r.stepStorm(now)
	r.stepZones()
	r.StepWormholes(now)
	r.recordHistory()
}

func approach(cur, target, amount float64) float64 {
	if cur < target {
		return math.Min(target, cur+amount)
	}
	return math.Max(target, cur-amount)
}

func (r *Room) Move(p *PlayerState, now time.Time) {
	wasGrounded := p.OnGround
	k := p.CmdKeys
	fwd, side := 0.0, 0.0
	if k&KeyForward != 0 {
		fwd++
	}
	if k&KeyBack != 0 {
		fwd--
	}
	if k&KeyRight != 0 {
		side++
	}
	if k&KeyLeft != 0 {
		side--
	}
	moving := fwd != 0 || side != 0
	if fwd != 0 && side != 0 {
		fwd /= math.Sqrt2
		side /= math.Sqrt2
	}
	if p.Flying {
		p.Crouch = false
	} else if k&KeyCrouch != 0 {
		p.Crouch = true
	} else if !p.Crouch || r.World.CanOccupy(p.Pos, StandingHeight) {
		p.Crouch = false
	}
	speed := WalkSpeed * Weapons[min(int(p.Weapon), len(Weapons)-1)].SpeedMult
	if p.GhostAt(now) {
		speed *= GhostSpeedMultiplier
	}
	if p.RampageUntil.After(now) {
		speed *= 1.3 // 狂暴：移速加成
	}
	if now.Before(p.SpeedUntil) {
		speed *= p.streakSpeedMul()
	}
	speed *= p.hexSpeedMul() // 海克斯「疾行者」
	if p.Crouch {
		speed *= CrouchSpeed
	}
	// Shift 冲刺：非飞行、非蹲伏且在移动时生效（飞行中 Shift 是下降）。
	if k&KeyDescend != 0 && !p.Flying && moving && !p.Crouch {
		speed *= SprintMultiplier
	}
	sin, cos := math.Sin(p.Yaw), math.Cos(p.Yaw)
	wishX, wishZ := (side*cos-fwd*sin)*speed, (-fwd*cos-side*sin)*speed
	accel := GroundAccel * TickDT
	if !p.OnGround && !p.Flying {
		accel = AirAccel * TickDT
	}
	if !moving && p.OnGround {
		accel = StopAccel * TickDT
	}
	p.Vel.X = approach(p.Vel.X, wishX, accel)
	p.Vel.Z = approach(p.Vel.Z, wishZ, accel)
	horizontalSpeed := math.Hypot(p.Vel.X, p.Vel.Z)
	if horizontalSpeed > speed {
		scale := speed / horizontalSpeed
		p.Vel.X *= scale
		p.Vel.Z *= scale
	}
	if p.Flying {
		wishY := 0.0
		if k&KeyJump != 0 {
			wishY += FlightSpeed
		}
		if k&KeyDescend != 0 {
			wishY -= FlightSpeed
		}
		p.Vel.Y = approach(p.Vel.Y, wishY, GroundAccel*TickDT)
		p.OnGround = r.World.MoveAABB(&p.Pos, &p.Vel, TickDT, StandingHeight, false)
		halfX, halfZ := r.World.Size[0]/2-PlayerHalf, r.World.Size[1]/2-PlayerHalf
		p.Pos.X = math.Max(-halfX, math.Min(halfX, p.Pos.X))
		p.Pos.Z = math.Max(-halfZ, math.Min(halfZ, p.Pos.Z))
		p.Pos.Y = math.Max(0, math.Min(MaxFlightHeight, p.Pos.Y))
		if p.Pos.Y == 0 && p.Vel.Y < 0 || p.Pos.Y == MaxFlightHeight && p.Vel.Y > 0 {
			p.Vel.Y = 0
		}
		return
	}
	if k&KeyJump != 0 && p.OnGround {
		p.Vel.Y = JumpVel
		p.OnGround = false
	}
	p.Vel.Y += Gravity * TickDT
	p.OnGround = r.World.MoveAABB(&p.Pos, &p.Vel, TickDT, p.Height(), p.OnGround)
	if !wasGrounded && p.OnGround {
		p.LandingUntil = now.Add(140 * time.Millisecond)
	}
}

func (r *Room) CheckSanity(p *PlayerState) {
	bad := math.IsNaN(p.Pos.X) || math.IsNaN(p.Pos.Y) || math.IsNaN(p.Pos.Z) ||
		math.Abs(p.Pos.X) > r.World.Size[0]/2+5 || math.Abs(p.Pos.Z) > r.World.Size[1]/2+5 || p.Pos.Y < -10 ||
		r.tick%60 == 0 && !r.World.CanOccupy(p.Pos, p.Height())
	if bad {
		log.Printf("player %d (%s): invalid position reset", p.Id, p.Name)
		p.Pos = r.BestSpawn(p)
		p.Vel = Vec3{}
		p.OnGround, p.Crouch, p.Flying = true, false, false
	}
}

func (r *Room) TryFire(p *PlayerState, yaw, pitch float64, mode uint8, seenTick uint32, shotSeq uint16, now time.Time) bool {
	if !p.Alive || p.Reloading || now.Before(p.NextFire) || !finite(yaw) || !finite(pitch) {
		return false
	}
	// 电磁风暴：枪械间歇性失灵（不耗弹，哑火）。
	if r.empJam(now) {
		return false
	}
	if p.IsBot && r.AnyBlackDream(now) {
		return false
	}
	yaw = math.Remainder(yaw, 2*math.Pi)
	pitch = math.Max(-1.55, math.Min(1.55, pitch))
	if p.HasShot && shotSeq == p.LastShotSeq {
		return false
	}
	p.HasShot, p.LastShotSeq = true, shotSeq
	weapon := min(int(p.Weapon), len(Weapons)-1)
	def := Weapons[weapon]
	mag, reserve := p.ActiveAmmo()
	if isGun(uint8(weapon)) && mag <= 0 {
		return false
	}
	if isGun(uint8(weapon)) {
		p.setActiveAmmo(mag-1, reserve)
	}
	gap := time.Duration(60 / def.Rpm * float64(time.Second))
	if p.RampageUntil.After(now) {
		gap /= 2 // 狂暴：射速 ×2
	}
	if div := p.hexFireRateDiv(); div > 1 {
		gap = time.Duration(float64(gap) / div) // 海克斯「狂热」
	}
	if weapon == 6 {
		gap = KnifeSlashInterval
		if mode&1 != 0 {
			gap = KnifeHeavyInterval
		}
	}
	if weapon == 6 || p.NextFire.IsZero() || now.Sub(p.NextFire) >= gap {
		p.NextFire = now.Add(gap)
	} else {
		p.NextFire = p.NextFire.Add(gap)
	}
	if isGun(uint8(weapon)) && mag == 1 && reserve > 0 {
		r.StartReload(p, now)
	}
	if now.Sub(p.LastShotAt) > 420*time.Millisecond {
		p.ShotCounter = 0
	}
	p.LastShotAt = now
	p.ShotCounter++
	if p.ProtectedAt(now) {
		if !p.RevengeActive {
			p.InvincibleUntil = time.Time{}
		}
	}

	origin := Vec3{p.Pos.X, p.Pos.Y + EyeHeight, p.Pos.Z}
	if p.Crouch {
		origin.Y = p.Pos.Y + CrouchEyeH
	}
	dir := AimDir(yaw, pitch)
	maxDist := 200.0
	if weapon == 6 {
		maxDist = 1.65
	}
	ads := mode&0x80 != 0
	spread := 0.0
	if isGun(uint8(weapon)) {
		spread = weaponSpread(def, p.Vel.X, p.Vel.Z, p.OnGround, p.Crouch, now.Before(p.LandingUntil), ads, max(0, int(p.ShotCounter)-1))
		spread *= p.hexSpreadMul() // 海克斯「稳定」
		settle := AWPScopeTime
		if weapon == 11 {
			settle = 240 * time.Millisecond
		}
		if isSniper(uint8(weapon)) && (!ads || p.AimStarted.IsZero() || now.Sub(p.AimStarted) < settle) {
			spread = math.Max(spread, def.MoveSpreadDeg)
		}
	}
	if seenTick > r.tick {
		seenTick = r.tick
	}
	if r.tick-seenTick > MaxRewindTicks {
		seenTick = r.tick - MaxRewindTicks
	}
	// Revenge is a fixed 10-bullet budget: every gunshot counts, with or without a
	// lock. Invincibility can expire independently without ending the budget.
	if p.RevengeActive && p.RevengeShots > 0 && isGun(uint8(weapon)) {
		if target := r.revengeTarget(p, yaw, pitch, origin, seenTick, now); target != nil {
			r.Damage(p, target, def.Dmg*def.HeadMult, true, uint8(weapon), now)
			p.RevengeShots--
			if p.RevengeShots == 0 {
				p.RevengeActive = false
			}
			return true
		}
		p.RevengeShots--
		if p.RevengeShots == 0 {
			p.RevengeActive = false
		}
	}

	pellets := 1
	if isGun(uint8(weapon)) && def.Pellets > 1 {
		pellets = def.Pellets
	}
	for n := 0; n < pellets; n++ {
		shotDir := dir
		if isGun(uint8(weapon)) {
			if isShotgun(uint8(weapon)) {
				shotDir = shotgunDir(dir, spread, n, int(uint8(shotSeq)), weapon, p.Id)
			} else {
				shotDir = patternDir(dir, spread, spreadSample(shotSeq, pellets, n), weapon, p.Id)
			}
		}
		_, worldDist := r.World.Raycast(origin, shotDir, maxDist)
		var target *PlayerState
		targetDist, hitY, hitHeight := maxDist, 0.0, StandingHeight
		for _, other := range r.Players {
			o := &other.PlayerState
			if o == p || !o.Alive || o.ProtectedAt(now) || o.GhostAt(now) || o.Id == p.IllegalMate {
				continue
			}
			pose := r.poseAt(o.Id, seenTick, o.Pos, o.Crouch)
			height := StandingHeight
			if pose.Crouch {
				height = CrouchingHeight
			}
			if d, ok := RayPlayerAABBHeight(origin, shotDir, pose.Pos, height, math.Min(worldDist, maxDist)); ok && d < targetDist {
				target, targetDist, hitY, hitHeight = o, d, origin.Y+shotDir.Y*d-pose.Pos.Y, height
			}
		}
		if r.chickenShot(p, origin, shotDir, worldDist, targetDist, uint8(weapon), now) {
			continue
		}
		if target == nil {
			continue
		}
		headshot := isGun(uint8(weapon)) && hitY >= hitHeight-.4
		dmg := def.Dmg
		if weapon == 6 {
			if mode&1 != 0 {
				dmg = 55
			}
			toAttacker := norm(Vec3{p.Pos.X - target.Pos.X, 0, p.Pos.Z - target.Pos.Z})
			forward := Vec3{-math.Sin(target.Yaw), 0, -math.Cos(target.Yaw)}
			if toAttacker.X*forward.X+toAttacker.Z*forward.Z < -.5 {
				dmg *= 2
			}
		} else if headshot {
			dmg *= def.HeadMult
		} else if hitY <= .65 && !isShotgun(uint8(weapon)) {
			dmg *= .75
		}
		if isShotgun(uint8(weapon)) && targetDist > 8 {
			dmg *= math.Max(0.38, 1-(targetDist-8)/16)
		}
		r.Damage(p, target, dmg, headshot, uint8(weapon), now)
		if !target.Alive && pellets == 1 {
			break
		}
	}
	hear := 38.0
	if weapon == 6 {
		hear = 14
	}
	r.alertBotsSound(origin, p, now, hear)
	return true
}

func (r *Room) revengeTarget(attacker *PlayerState, yaw, pitch float64, origin Vec3, seenTick uint32, now time.Time) *PlayerState {
	var best *PlayerState
	bestScore := math.MaxFloat64
	forward, right, up := viewAxes(yaw, pitch)
	// Cover a full on-screen rectangle (75° vertical, 21:9 horizontal) so any
	// enemy in view locks, not only someone near the crosshair.
	tanV := math.Tan(37.5 * math.Pi / 180)
	tanH := tanV * 21 / 9
	for _, other := range r.Players {
		target := &other.PlayerState
		if target == attacker || !target.Alive || target.ProtectedAt(now) || target.Id == attacker.IllegalMate {
			continue
		}
		pose := r.poseAt(target.Id, seenTick, target.Pos, target.Crouch)
		height := StandingHeight
		if pose.Crouch {
			height = CrouchingHeight
		}
		visible, centerDot, dist := r.revengeVisible(origin, forward, right, up, pose.Pos, height, tanH, tanV)
		if !visible {
			continue
		}
		score := (1-centerDot)*100 + dist*.001
		if score < bestScore {
			best, bestScore = target, score
		}
	}
	return best
}

func viewAxes(yaw, pitch float64) (forward, right, up Vec3) {
	forward = AimDir(yaw, pitch)
	right = norm(Vec3{-forward.Z, 0, forward.X})
	if right.X == 0 && right.Z == 0 {
		right = Vec3{1, 0, 0}
	}
	up = Vec3{
		right.Y*forward.Z - right.Z*forward.Y,
		right.Z*forward.X - right.X*forward.Z,
		right.X*forward.Y - right.Y*forward.X,
	}
	return
}

func inViewFrustum(origin, forward, right, up, point Vec3, tanH, tanV float64) (dot, dist float64, ok bool) {
	to := Vec3{point.X - origin.X, point.Y - origin.Y, point.Z - origin.Z}
	dist = math.Sqrt(to.X*to.X + to.Y*to.Y + to.Z*to.Z)
	if dist <= 0.05 {
		return 0, dist, false
	}
	z := to.X*forward.X + to.Y*forward.Y + to.Z*forward.Z
	if z <= 0.05 {
		return 0, dist, false
	}
	x := to.X*right.X + to.Y*right.Y + to.Z*right.Z
	y := to.X*up.X + to.Y*up.Y + to.Z*up.Z
	if math.Abs(x) > z*tanH || math.Abs(y) > z*tanV {
		return 0, dist, false
	}
	return z / dist, dist, true
}

func (r *Room) revengeVisible(origin, forward, right, up, pos Vec3, height, tanH, tanV float64) (bool, float64, float64) {
	points := []Vec3{
		{pos.X, pos.Y + height - .12, pos.Z},
		{pos.X, pos.Y + height*.55, pos.Z},
		{pos.X, pos.Y + .2, pos.Z},
		{pos.X + PlayerHalf, pos.Y + height*.55, pos.Z},
		{pos.X - PlayerHalf, pos.Y + height*.55, pos.Z},
		{pos.X, pos.Y + height*.55, pos.Z + PlayerHalf},
		{pos.X, pos.Y + height*.55, pos.Z - PlayerHalf},
	}
	bestDot, bestDist := -1.0, math.MaxFloat64
	any := false
	for _, point := range points {
		dot, dist, ok := inViewFrustum(origin, forward, right, up, point, tanH, tanV)
		if !ok {
			continue
		}
		line := norm(Vec3{point.X - origin.X, point.Y - origin.Y, point.Z - origin.Z})
		if hit, distance := r.World.Raycast(origin, line, dist); hit && distance < dist-.05 {
			continue
		}
		any = true
		if dot > bestDot {
			bestDot, bestDist = dot, dist
		}
	}
	return any, bestDot, bestDist
}

func (r *Room) Damage(attacker, victim *PlayerState, dmg float64, headshot bool, weapon uint8, now time.Time) {
	if !victim.Alive || victim.ProtectedAt(now) || victim.UltimateInvincibleAt(now) {
		return
	}
	if now.Before(attacker.DmgUntil) {
		dmg *= attacker.streakDamageMul()
	}
	dmg *= attacker.hexDamageMul() // 海克斯「强袭」
	dmg *= victim.hexDmgTakenMul() // 海克斯「铁壁」
	actual := dmg
	if victim.Armor > 0 && isGun(weapon) {
		actual = dmg * Weapons[weapon].ArmorPen
		lost := uint8(math.Min(float64(victim.Armor), math.Ceil((dmg-actual)*.5)))
		victim.Armor -= lost
	}
	d := uint8(math.Max(1, math.Min(actual, float64(victim.HP))))
	victim.HP -= d
	// 荆棘甲：受击反伤 30%，递归闸防止双方互反弹无限循环。
	if victim.ThornsUntil.After(now) && attacker.Id != victim.Id && !r.reflecting {
		r.reflecting = true
		r.Damage(victim, attacker, dmg*0.3, false, 6, now)
		r.reflecting = false
	}
	// 吸血：造成伤害回复一半（向下取整）。
	if attacker.LifestealUntil.After(now) && attacker.Id != victim.Id {
		attacker.HP = uint8(min(attacker.maxHP(), int(attacker.HP)+int(d)/2))
	}
	// 海克斯「利爪」：造成伤害 15% 回复生命。
	if attacker.Hex == HexLeech && attacker.Id != victim.Id {
		attacker.HP = uint8(min(attacker.maxHP(), int(attacker.HP)+int(float64(d)*0.15)))
	}
	hs := uint8(0)
	if headshot {
		hs = 1
	}
	r.Emit(Event{Type: EvHit, Player: attacker.Id, Victim: victim.Id, Dmg: d, Headshot: hs})
	if victim.HP > 0 {
		r.botTookHit(victim, attacker, now)
		return
	}
	r.botKilled(victim, attacker, now)
	r.Emit(Event{Type: EvKill, Killer: attacker.Id, Victim: victim.Id, Weapon: weapon, Headshot: hs})
	attacker.Kills++
	victim.Deaths++
	// A kill breaks the attacker's consecutive zero-kill/death streak.
	attacker.NoKillDeaths = 0
	if !victim.IsBot {
		victim.NoKillDeaths++
		if victim.NoKillDeaths >= RevengeDeathThreshold {
			// Consume the accumulated consecutive deaths when arming revenge.
			victim.RevengeReady = true
			victim.NoKillDeaths = 0
		}
	}
	if !attacker.IsBot && attacker.Id != victim.Id && attacker.Ultimate == 0 && attacker.UltimatePoints < UltimateRequirement {
		attacker.UltimatePoints++
	}
	if attacker.Id != victim.Id {
		if attacker.Streak < 255 {
			attacker.Streak++
		}
		r.applyStreakReward(attacker, now)
		r.bountySet(attacker)
	}
	r.announceKillfeedMemes(attacker, victim)
	r.bountyClaim(attacker, victim, now)
	victim.Streak = 0
	victim.UltimatePoints, victim.Ultimate = 0, 0
	victim.BlackDreamUntil, victim.InvincibleUntilUlt, victim.GhostUntil = time.Time{}, time.Time{}, time.Time{}
	victim.DmgUntil, victim.RecoilUntil, victim.SpeedUntil = time.Time{}, time.Time{}, time.Time{}
	victim.ThornsUntil, victim.LifestealUntil, victim.RampageUntil = time.Time{}, time.Time{}, time.Time{}
	victim.Hex = 0 // 海克斯卡随本条命死亡失效（死亡倒计时期间选的新卡由 HexPick 写入，不受此处影响）
	if !attacker.IsBot {
		r.Store.Accumulate(attacker.Account, 1, 0)
		if !victim.IsBot && isGun(weapon) {
			r.Store.AccumulateWeaponKill(attacker.Account, weapon)
		}
	}
	if !victim.IsBot {
		r.Store.Accumulate(victim.Account, 0, 1)
	}
	victim.LastKiller = attacker.Id
	victim.KilledAt = now

	// A kill can trigger at most one bond event; revenge takes priority over cover and resonance.
	if attacker.BondMate != 0 && attacker.BondMate != attacker.Id {
		if mate := r.findPlayer(attacker.BondMate); mate != nil {
			kind := uint8(255)
			if mate.LastKiller == victim.Id && !mate.KilledAt.IsZero() && now.Sub(mate.KilledAt) < BondAvengeWindow {
				kind = 1
				mate.LastKiller, mate.KilledAt = 0, time.Time{}
			} else if mate.Id != victim.Id && mate.Alive {
				dx := attacker.Pos.X - mate.Pos.X
				dz := attacker.Pos.Z - mate.Pos.Z
				if dx*dx+dz*dz <= BondCoverRange*BondCoverRange {
					kind = 0
				} else if attacker.Streak >= 2 && mate.Streak >= 2 {
					kind = 2
				}
			}
			if kind != 255 {
				r.Emit(Event{Type: EvBondEvent, Player: attacker.Id, Victim: mate.Id, Kind: kind, Dmg: attacker.addBondScore(), Name: attacker.Name})
			}
		}
	}

	// 死亡即散伙：非法小队随任一成员阵亡自动解散。
	r.BreakIllegalTeam(victim)
	victim.Alive, victim.Reloading = false, false
	victim.RevengeActive, victim.RevengeShots = false, 0
	victim.InvincibleUntil = time.Time{}
	victim.RespawnAt = now.Add(RespawnDelayS)
	r.offerHex(victim) // 复活倒计时期间发新一轮 3 选 1
}

// 连杀梗播报：服务端以「战场播报」身份走 EvChat 通道整活。
// 仅当房间里有真人时才播，避免无人局被 bot 内战刷屏。
var killstreakLines = map[uint8][]string{
	3: {
		"%s 三连杀！建议对手集体申请工伤",
		"%s 三连杀达成，枪管正在冒烟，请勿触摸",
		"警告：%s 已三连杀，对手的头部保险即将到期",
	},
	5: {
		"%s 五连杀！这已经不是枪法了，是因果律武器",
		"%s 五连杀达成，正在考虑要不要收手（不会）",
		"战况通报：%s 五连杀，正朝主播方向发展",
	},
	8: {
		"%s 八连杀！建议全体玩家原地表演一个投降",
		"%s 八连杀！队友已经开始截图留念",
	},
	10: {
		"%s 十连杀！战场已变成个人直播间",
		"%s 十连杀！服务端递上一杯冰可乐",
		"讣告（对手方）：请为 %s 的十连杀默哀三秒",
	},
}

var deathStreakLines = []string{
	"%s 已连死五次，正在申请工伤认定",
	"%s 连死五次，出生点的地板都认得 TA 了",
	"%s 已连死五次！鼓励一下：起码贡献了别人的击杀数",
}

// deathStreakMemeAt：连续死亡达到该次数时发一条安慰播报（NoKillDeaths 会在
// RevengeDeathThreshold 次时清零，所以选一个小于它的值）。
const deathStreakMemeAt uint8 = 5

func (r *Room) hasHuman() bool {
	for _, p := range r.Players {
		if !p.IsBot {
			return true
		}
	}
	return false
}

func (r *Room) broadcastChat(name, message string) {
	r.Emit(Event{Type: EvChat, Player: 0, Name: name, Message: message})
}

func (r *Room) announceKillfeedMemes(attacker, victim *PlayerState) {
	if !r.hasHuman() {
		return
	}
	if attacker.Id != victim.Id {
		if lines := killstreakLines[attacker.Streak]; len(lines) > 0 {
			r.broadcastChat("战场播报", fmt.Sprintf(lines[rand.IntN(len(lines))], attacker.Name))
		}
	}
	if !victim.IsBot && victim.NoKillDeaths == deathStreakMemeAt {
		r.broadcastChat("战场播报", fmt.Sprintf(deathStreakLines[rand.IntN(len(deathStreakLines))], victim.Name))
	}
}

func (p *PlayerState) streakScale() int {
	return min(int(p.Streak), streakScaleCap)
}

func (p *PlayerState) streakDamageMul() float64 {
	return math.Min(streakDmgCap, 1.16+0.03*float64(p.streakScale()))
}

func (p *PlayerState) streakSpeedMul() float64 {
	return math.Min(streakSpeedCap, 1.30+0.02*float64(p.streakScale()))
}

func (p *PlayerState) grantStreakAmmo() {
	ids := [2]uint8{p.Primary, p.Secondary}
	for i, id := range ids {
		if int(id) >= len(Weapons) {
			continue
		}
		def := Weapons[id]
		add := max(def.Mag/2, 5)
		space := def.Mag - p.Mags[i]
		if space > 0 {
			take := min(space, add)
			p.Mags[i] += take
			add -= take
		}
		p.Reserves[i] = min(def.Reserve, p.Reserves[i]+add)
	}
}

type streakNeed struct {
	kind  uint8
	score int
}

func (p *PlayerState) streakNeeds(now time.Time) []streakNeed {
	needs := make([]streakNeed, 0, 5)
	if s := p.healPressure(); s > 0 {
		needs = append(needs, streakNeed{StreakHeal, s})
	}
	if s := p.ammoPressure(); s > 0 {
		needs = append(needs, streakNeed{StreakAmmo, s})
	}
	if now.After(p.SpeedUntil) || p.SpeedUntil.Sub(now) < 3*time.Second {
		needs = append(needs, streakNeed{StreakSpeed, 28})
	}
	if (now.After(p.RecoilUntil) || p.RecoilUntil.Sub(now) < 3*time.Second) && isGun(p.Weapon) && !isSniper(p.Weapon) {
		needs = append(needs, streakNeed{StreakRecoil, 32})
	}
	if now.After(p.DmgUntil) || p.DmgUntil.Sub(now) < 3*time.Second {
		needs = append(needs, streakNeed{StreakDamage, 26})
	}
	return needs
}

func (p *PlayerState) healPressure() int {
	if p.HP <= 25 {
		return 120
	}
	if p.HP <= 45 {
		return 95
	}
	if p.HP <= 70 {
		return 58
	}
	if int(p.HP) < p.maxHP() {
		return 22
	}
	if p.Armor <= 15 {
		return 42
	}
	if p.Armor <= 40 {
		return 24
	}
	return 0
}

func (p *PlayerState) ammoPressure() int {
	best := 0
	ids := [2]uint8{p.Primary, p.Secondary}
	for i, id := range ids {
		if int(id) >= len(Weapons) {
			continue
		}
		def := Weapons[id]
		if def.Mag <= 0 {
			continue
		}
		mag, res := p.Mags[i], p.Reserves[i]
		score := 0
		if mag == 0 && res == 0 {
			score = 100
		} else if mag == 0 {
			score = 82
		} else if mag*100/def.Mag <= 20 {
			score = 68
		} else if mag*100/def.Mag <= 40 {
			score = 40
		} else if res*100/max(def.Reserve, 1) <= 20 {
			score = 22
		}
		if p.ActiveSlot == uint8(i+1) && score > 0 {
			score += 12
		}
		if score > best {
			best = score
		}
	}
	return best
}

func (r *Room) applyStreakReward(p *PlayerState, now time.Time) {
	n := int(p.Streak)
	if n < 2 {
		return
	}
	needs := p.streakNeeds(now)
	if len(needs) == 0 {
		needs = []streakNeed{{StreakDamage, 1}}
	}
	for i := 1; i < len(needs); i++ {
		for j := i; j > 0 && needs[j].score > needs[j-1].score; j-- {
			needs[j], needs[j-1] = needs[j-1], needs[j]
		}
	}
	picks := 1
	if n >= 5 {
		picks = 2
	}
	if n >= 8 {
		picks = streakPickCap
	}
	if picks > len(needs) {
		picks = len(needs)
	}
	kind := uint8(0)
	n = min(n, streakScaleCap)
	dur := min(streakDurationCap, time.Duration(6+n)*time.Second)
	ms := uint16(dur / time.Millisecond)
	healAmt := min(streakHealCap, 32+n*4)
	for i := 0; i < picks; i++ {
		if needs[i].score <= 0 && i > 0 {
			break
		}
		switch needs[i].kind {
		case StreakAmmo:
			p.grantStreakAmmo()
			kind |= StreakAmmo
		case StreakHeal:
			p.HP = uint8(min(p.maxHP(), int(p.HP)+healAmt))
			p.Armor = uint8(min(100, int(p.Armor)+min(40, 15+n*4)))
			kind |= StreakHeal
		case StreakSpeed:
			if now.Add(dur).After(p.SpeedUntil) {
				p.SpeedUntil = now.Add(dur)
			}
			kind |= StreakSpeed
		case StreakRecoil:
			if now.Add(dur).After(p.RecoilUntil) {
				p.RecoilUntil = now.Add(dur)
			}
			kind |= StreakRecoil
		case StreakDamage:
			if now.Add(dur).After(p.DmgUntil) {
				p.DmgUntil = now.Add(dur)
			}
			kind |= StreakDamage
		}
	}
	r.Emit(Event{Type: EvStreakBuff, Player: p.Id, Kind: kind, Dmg: p.Streak, Ms: ms})
}

func (r *Room) CastUltimate(p *PlayerState, kind uint8, now time.Time) bool {
	if !p.Alive || kind < UltimateBlackDream || kind > UltimateRampage {
		return false
	}
	if p.UltimatePoints < UltimateRequirement || p.Ultimate != 0 {
		return false
	}
	duration := time.Duration(0)
	switch kind {
	case UltimateBlackDream:
		duration = UltimateBlackDreamS
		p.BlackDreamUntil = now.Add(duration)
	case UltimateInvincible:
		duration = UltimateInvincibleS
		p.InvincibleUntilUlt = now.Add(duration)
	case UltimateGhost:
		duration = UltimateGhostS
		p.GhostUntil = now.Add(duration)
	case UltimateThorns:
		duration = UltimateAwakenS
		p.ThornsUntil = now.Add(duration)
	case UltimateVampire:
		duration = UltimateAwakenS
		p.LifestealUntil = now.Add(duration)
	case UltimateRampage:
		duration = UltimateAwakenS
		p.RampageUntil = now.Add(duration)
	}
	p.Ultimate = kind
	p.UltimatePoints = 0
	r.Emit(Event{Type: EvUltimate, Player: p.Id, Kind: kind, Ms: uint16(duration / time.Millisecond), Name: p.Name})
	return true
}

func (r *Room) StartReload(p *PlayerState, now time.Time) bool {
	if !p.Alive || p.Reloading || p.ActiveSlot > 2 {
		return false
	}
	mag, reserve := p.ActiveAmmo()
	def := Weapons[p.Weapon]
	if mag >= def.Mag || reserve <= 0 {
		return false
	}
	reloadMs := int(float64(def.ReloadMs) * p.hexReloadMul()) // 海克斯「快手」
	p.Reloading = true
	p.ReloadEnd = now.Add(time.Duration(reloadMs) * time.Millisecond)
	p.NextFire = p.ReloadEnd
	r.Emit(Event{Type: EvReloadStart, Player: p.Id, Ms: uint16(reloadMs)})
	return true
}

func (r *Room) FinishReloads(now time.Time) {
	for _, pl := range r.Players {
		p := &pl.PlayerState
		if !p.Reloading || now.Before(p.ReloadEnd) {
			continue
		}
		mag, reserve := p.ActiveAmmo()
		need := Weapons[p.Weapon].Mag - mag
		take := min(need, reserve)
		p.setActiveAmmo(mag+take, reserve-take)
		p.Reloading = false
	}
}

func (r *Room) Respawn(p *PlayerState, now time.Time) {
	p.Pos = r.BestSpawn(p)
	p.Vel = Vec3{}
	// 注意：这里不清 p.Hex——死亡倒计时期间选中的海克斯卡要带入新的一命
	//（旧卡的失效只发生在 Damage 的死亡结算块）。
	p.HP = uint8(p.maxHP())
	p.Armor = 100
	p.Alive = true
	p.OnGround, p.Crouch, p.Flying = true, false, false
	p.CmdKeys = 0
	p.ApplyLoadout(p.Primary, p.Secondary)
	p.InvincibleUntil = now.Add(SpawnProtectS)
	if p.RevengeReady && !p.IsBot {
		p.RevengeActive = true
		p.RevengeShots = 10
		p.InvincibleUntil = now.Add(10 * time.Second)
		r.Emit(Event{Type: EvRevenge, Player: p.Id, Name: p.Name})
	}
	p.RevengeReady = false

	p.LandingUntil, p.AimStarted = time.Time{}, time.Time{}
	p.SpeedUntil, p.DmgUntil, p.RecoilUntil = time.Time{}, time.Time{}, time.Time{}
	p.Streak = 0
	p.UltimatePoints, p.Ultimate = 0, 0
	p.BlackDreamUntil, p.InvincibleUntilUlt, p.GhostUntil = time.Time{}, time.Time{}, time.Time{}
	p.RespawnAt = time.Time{}

	// Bond system: auto-pair unbonded human players
	if p.BondMate == 0 && !p.IsBot {
		for _, other := range r.Players {
			o := &other.PlayerState
			if o.Id == p.Id || o.IsBot || o.BondMate != 0 || !o.Alive {
				continue
			}
			p.BondMate = o.Id
			o.BondMate = p.Id
			r.Emit(Event{Type: EvBondEvent, Player: p.Id, Victim: o.Id, Kind: 3, Name: p.Name})
			break
		}
	}
	delete(r.teamAttempts, p.Id)

	r.Emit(Event{Type: EvRespawn, Player: p.Id, Origin: p.Pos})
}

func (r *Room) BestSpawn(p *PlayerState) Vec3 {
	if len(r.World.Spawns) == 0 {
		return Vec3{}
	}
	type scored struct {
		pos   Vec3
		score float64
	}
	best := [4]scored{{score: -math.MaxFloat64}, {score: -math.MaxFloat64}, {score: -math.MaxFloat64}, {score: -math.MaxFloat64}}
	considered := 0
	stride := max(1, (len(r.World.Spawns)+63)/64)
	start := rand.IntN(stride)
	for i := start; i < len(r.World.Spawns); i += stride {
		s := r.World.Spawns[i]
		pos := Vec3{s[0], s[1], s[2]}
		score := 1e9
		for _, other := range r.Players {
			o := &other.PlayerState
			if o == p || !o.Alive {
				continue
			}
			d := math.Sqrt(dist2(pos, o.Pos))
			if d < score {
				score = d
			}
			if d < 24 {
				dir := norm(Vec3{o.Pos.X - pos.X, o.Pos.Y + EyeHeight - pos.Y - EyeHeight, o.Pos.Z - pos.Z})
				if hit, hd := r.World.Raycast(Vec3{pos.X, pos.Y + EyeHeight, pos.Z}, dir, d); !hit || hd >= d-.5 {
					score -= 18
				}
			}
		}
		candidate := scored{pos, score}
		for rank := range best {
			if candidate.score > best[rank].score {
				candidate, best[rank] = best[rank], candidate
			}
		}
		considered++
	}
	n := min(4, considered)
	return best[rand.IntN(n)].pos
}

const (
	PickupAmmo uint8 = iota
	PickupHealth
	PickupSpeed
	PickupAirdrop
	pickupCount = 12
)

const (
	speedBoostDuration         = 8 * time.Second
	speedBoostMultiplier       = 1.35
	streakDmgCap               = 1.35
	streakSpeedCap             = 1.45
	streakHealCap              = 55
	streakDurationCap          = 10 * time.Second
	streakScaleCap             = 8
	streakPickCap              = 3
	StreakAmmo           uint8 = 1
	StreakHeal           uint8 = 2
	StreakSpeed          uint8 = 4
	StreakRecoil         uint8 = 8
	StreakDamage         uint8 = 16
)

type Pickup struct {
	Id        uint16
	Kind      uint8
	Pos       Vec3
	Active    bool
	RespawnAt time.Time
}

func (r *Room) initPickups() {
	if len(r.World.Spawns) == 0 {
		return
	}
	r.Pickups = make([]Pickup, pickupCount)
	for i := range r.Pickups {
		r.Pickups[i] = Pickup{Id: uint16(i + 1), Kind: uint8(rand.IntN(3)), Active: true}
		r.Pickups[i].Pos = r.randomPickupPosition()
	}
}

func (r *Room) randomPickupPosition() Vec3 {
	var pos Vec3
	for range 24 {
		spawn := r.World.Spawns[rand.IntN(len(r.World.Spawns))]
		pos = Vec3{spawn[0], spawn[1], spawn[2]}
		clear := true
		for i := range r.Pickups {
			pickup := &r.Pickups[i]
			dx, dz := pos.X-pickup.Pos.X, pos.Z-pickup.Pos.Z
			if pickup.Active && dx*dx+dz*dz < 36 {
				clear = false
				break
			}
		}
		if clear {
			return pos
		}
	}
	return pos
}

func (r *Room) pickupEvents() []Event {
	events := make([]Event, 0, len(r.Pickups))
	for i := range r.Pickups {
		pickup := &r.Pickups[i]
		if pickup.Active {
			events = append(events, Event{Type: EvPickupSpawn, Player: pickup.Id, Kind: pickup.Kind, Origin: pickup.Pos})
		}
	}
	return events
}

func (r *Room) StepPickups(now time.Time) {
	for i := range r.Pickups {
		pickup := &r.Pickups[i]
		if !pickup.Active {
			if now.Before(pickup.RespawnAt) {
				continue
			}
			pickup.Kind = uint8(rand.IntN(3))
			pickup.Pos = r.randomPickupPosition()
			pickup.Active = true
			r.Emit(Event{Type: EvPickupSpawn, Player: pickup.Id, Kind: pickup.Kind, Origin: pickup.Pos})
		}
		for _, player := range r.Players {
			p := &player.PlayerState
			dx, dz := p.Pos.X-pickup.Pos.X, p.Pos.Z-pickup.Pos.Z
			if !p.Alive || math.Abs(p.Pos.Y-pickup.Pos.Y) > 1.5 || dx*dx+dz*dz > 1.44 || !applyPickup(p, pickup.Kind, now) {
				continue
			}
			pickup.Active = false
			pickup.RespawnAt = now.Add(time.Duration(8+rand.IntN(5)) * time.Second)
			if pickup.Kind == PickupAirdrop {
				// 传奇补给被抢：额外 +2 大招点 + 全房播报；空投箱不进入常规重生轮换。
				pickup.RespawnAt = now.Add(100 * 365 * 24 * time.Hour)
				if p.Ultimate == 0 {
					p.UltimatePoints = uint8(min(UltimateRequirement, int(p.UltimatePoints)+2))
				}
				if r.hasAirdropAudience() {
					r.Emit(Event{Type: EvChat, Player: 0, Name: "战场播报",
						Message: fmt.Sprintf("🏆 %s 抢到了传奇补给：满血 + 满甲 + 弹药全满 +2 大招点！", p.Name)})
				}
			}
			ms := uint16(0)
			if pickup.Kind == PickupSpeed {
				ms = uint16(speedBoostDuration / time.Millisecond)
			}
			r.Emit(Event{Type: EvPickupTaken, Player: pickup.Id, Victim: p.Id, Kind: pickup.Kind, Ms: ms})
			break
		}
	}
}

func applyPickup(p *PlayerState, kind uint8, now time.Time) bool {
	switch kind {
	case PickupAmmo:
		primary, secondary := Weapons[p.Primary], Weapons[p.Secondary]
		if p.Mags[0] == primary.Mag && p.Reserves[0] == primary.Reserve && p.Mags[1] == secondary.Mag && p.Reserves[1] == secondary.Reserve {
			return false
		}
		p.Mags = [2]int{primary.Mag, secondary.Mag}
		p.Reserves = [2]int{primary.Reserve, secondary.Reserve}
		p.Reloading, p.ReloadEnd, p.NextFire = false, time.Time{}, now
	case PickupHealth:
		if int(p.HP) >= p.maxHP() {
			return false
		}
		p.HP = uint8(min(p.maxHP(), int(p.HP)+50))
	case PickupSpeed:
		p.SpeedUntil = now.Add(speedBoostDuration)
	case PickupAirdrop:
		// 传奇补给：满血满甲弹药全满（大招点奖励由 StepPickups 结算时发放）。
		p.HP = uint8(p.maxHP())
		p.Armor = 100
		primary, secondary := Weapons[p.Primary], Weapons[p.Secondary]
		p.Mags = [2]int{primary.Mag, secondary.Mag}
		p.Reserves = [2]int{primary.Reserve, secondary.Reserve}
		p.Reloading, p.ReloadEnd, p.NextFire = false, time.Time{}, now
	default:
		return false
	}
	return true
}

type Grenade struct {
	Id, ThrowerId uint16
	Pos, Vel      Vec3
	ExplodesAt    time.Time
	Active        bool
}

const (
	chickenCount     = 6
	chickenSpeed     = 1.35                    // units per second
	chickenStepDist  = chickenSpeed / TickRate // per-tick travel distance
	chickenWanderR   = 9.0                     // roam radius around home spawn
	chickenHitHeight = 0.62                    // AABB height for bullet tests
	chickenHeartbeat = 5 * time.Second         // max age of an idle chicken's last broadcast

	// 鸡雨：定期空投增援小鸡，鸡口缓慢膨胀到上限为止。
	chickenRainBatch = chickenCount / 2  // 每场鸡雨空投数量
	chickenRainCap   = chickenCount * 3  // 鸡口上限
	chickenRainFirst = 75 * time.Second  // 开局到第一场鸡雨
	chickenRainMin   = 90 * time.Second  // 两场鸡雨最小间隔
	chickenRainMax   = 150 * time.Second // 两场鸡雨最大间隔

	// 鸡群围观：随机选一名存活真人当「偶像」，全体小鸡涌向他围观一阵。
	chickenMobFirst    = 150 * time.Second // 开局到第一场围观
	chickenMobDuration = 12 * time.Second  // 每场围观时长
	chickenMobMinGap   = 180 * time.Second // 两场围观最小间隔
	chickenMobMaxGap   = 300 * time.Second // 两场围观最大间隔
	chickenMobRetry    = 30 * time.Second  // 无真人可围观时重试

	// 《鸡鸡帝国》DLC：金鸡王混入鸡群，护卫队暴露位置，击杀掉超级补给。
	kingFirstAt    = 3 * time.Minute // 开局到首位金鸡王
	kingRespawnMin = 4 * time.Minute // 王驾崩后到下一位的最短间隔
	kingRespawnMax = 6 * time.Minute // 最长间隔
	kingRetry      = 30 * time.Second
	kingHP         = 600
	kingGuardCount = 2
)

// Battlefield chickens are the classic CS-style easter egg: harmless voxel
// birds that roam between spawns and pop into fried-chicken rewards when shot.
type Chicken struct {
	Id          uint16
	Home        Vec3
	Pos         Vec3
	Dir         Vec3 // horizontal heading; zero means idling in place
	Alive       bool
	King        bool   // 金鸡王：多枪才能击杀，击杀掉超级补给
	KingHP      uint16 // 王的血量（仅 King=true 时有效）
	NextTurn    time.Time
	RespawnAt   time.Time
	lastEmitPos Vec3
	lastEmitAt  time.Time
	forceEmit   bool
}

func (r *Room) initChickens() {
	if len(r.World.Spawns) == 0 {
		return
	}
	r.Chickens = make([]Chicken, chickenCount)
	r.nextChickenId = 300 + chickenCount
	for i := range r.Chickens {
		r.Chickens[i] = Chicken{Id: uint16(300 + i)}
		r.respawnChicken(&r.Chickens[i])
	}
}

// chickenRain 空投一批增援小鸡。鸡是常驻的（击杀后照常重生），不做回收，
// 只用 chickenRainCap 限制膨胀；纯 bot 局不播报但照常下雨。
func (r *Room) chickenRain(now time.Time) {
	r.nextChickenRainAt = now.Add(chickenRainMin + time.Duration(rand.IntN(int((chickenRainMax-chickenRainMin)/time.Second)))*time.Second)
	if len(r.Chickens) >= chickenRainCap {
		return
	}
	for range chickenRainBatch {
		r.Chickens = append(r.Chickens, Chicken{Id: r.nextChickenId})
		r.nextChickenId++
		r.respawnChicken(&r.Chickens[len(r.Chickens)-1])
	}
	for _, p := range r.Players {
		if !p.IsBot {
			r.Emit(Event{Type: EvChat, Player: 0, Name: "战场播报",
				Message: fmt.Sprintf("🐔 鸡雨来了！小鸡增援空投中（当前 %d 只）", len(r.Chickens))})
			return
		}
	}
}

func (r *Room) respawnChicken(c *Chicken) {
	c.Home = r.chickenSpot()
	c.Pos = c.Home
	c.Dir = Vec3{}
	c.Alive = true
	c.King = false
	c.KingHP = 0
	c.NextTurn = time.Time{}
	c.forceEmit = true
	r.Emit(Event{Type: EvChickenSpawn, Player: c.Id, Origin: c.Pos})
}

func (r *Room) chickenSpot() Vec3 {
	var pos Vec3
	for range 24 {
		spawn := r.World.Spawns[rand.IntN(len(r.World.Spawns))]
		pos = Vec3{spawn[0] + rand.Float64()*8 - 4, spawn[1], spawn[2] + rand.Float64()*8 - 4}
		clear := true
		for i := range r.Chickens {
			other := &r.Chickens[i]
			if !other.Alive || other == nil {
				continue
			}
			dx, dz := pos.X-other.Pos.X, pos.Z-other.Pos.Z
			if dx*dx+dz*dz < 25 {
				clear = false
				break
			}
		}
		if clear && r.World.CanOccupy(pos, chickenHitHeight) {
			return pos
		}
	}
	return pos
}

func (r *Room) chickenEvents() []Event {
	events := make([]Event, 0, len(r.Chickens))
	for i := range r.Chickens {
		c := &r.Chickens[i]
		if c.Alive {
			events = append(events, Event{Type: EvChickenSpawn, Player: c.Id, Origin: c.Pos, Dir: c.Dir})
		}
	}
	return events
}

func (r *Room) StepChickens(now time.Time) {
	if len(r.Chickens) == 0 {
		return
	}
	if r.nextChickenRainAt.IsZero() {
		r.nextChickenRainAt = now.Add(chickenRainFirst)
	} else if !now.Before(r.nextChickenRainAt) {
		r.chickenRain(now)
	}
	r.stepChickenKing(now)
	for i := range r.Chickens {
		c := &r.Chickens[i]
		if !c.Alive {
			if !now.Before(c.RespawnAt) {
				r.respawnChicken(c)
			}
			continue
		}
		if now.After(c.NextTurn) {
			c.NextTurn = now.Add(time.Duration(1+rand.IntN(3)) * time.Second)
			dx, dz := c.Pos.X-c.Home.X, c.Pos.Z-c.Home.Z
			farFromHome := dx*dx+dz*dz > chickenWanderR*chickenWanderR
			if !farFromHome && rand.IntN(3) == 0 {
				c.Dir = Vec3{} // stand still and peck at nothing in particular
			} else if farFromHome {
				c.Dir = norm(Vec3{c.Home.X - c.Pos.X, 0, c.Home.Z - c.Pos.Z})
			} else {
				angle := rand.Float64() * math.Pi * 2
				c.Dir = Vec3{-math.Sin(angle), 0, -math.Cos(angle)}
			}
		}
		moved := false
		if c.Dir.X != 0 || c.Dir.Z != 0 {
			next := Vec3{c.Pos.X + c.Dir.X*chickenStepDist, c.Pos.Y, c.Pos.Z + c.Dir.Z*chickenStepDist}
			headY := next.Y + chickenHitHeight*0.6
			if hit, dist := r.World.Raycast(Vec3{next.X, headY, next.Z}, c.Dir, chickenStepDist+0.4); !hit || dist > chickenStepDist {
				if hitDown, dDown := r.World.Raycast(Vec3{next.X, next.Y + 1.2, next.Z}, Vec3{0, -1, 0}, 2.2); hitDown {
					next.Y = next.Y + 1.2 - dDown
				}
				c.Pos = next
				moved = true
			} else {
				c.NextTurn = time.Time{} // bump into cover: pick a fresh heading on the next tick
			}
		}
		if c.forceEmit {
			r.Emit(Event{Type: EvChickenSpawn, Player: c.Id, Origin: c.Pos, Dir: c.Dir})
			c.lastEmitPos, c.lastEmitAt, c.forceEmit = c.Pos, now, false
		} else if moved {
			dx, dz := c.Pos.X-c.lastEmitPos.X, c.Pos.Z-c.lastEmitPos.Z
			if dx*dx+dz*dz > 0.25 {
				r.Emit(Event{Type: EvChickenSpawn, Player: c.Id, Origin: c.Pos, Dir: c.Dir})
				c.lastEmitPos, c.lastEmitAt = c.Pos, now
			}
		} else if now.Sub(c.lastEmitAt) >= chickenHeartbeat {
			// idling: occasional heartbeat repairs any lost position packet
			r.Emit(Event{Type: EvChickenSpawn, Player: c.Id, Origin: c.Pos, Dir: c.Dir})
			c.lastEmitPos, c.lastEmitAt = c.Pos, now
		}
	}
	r.stepChickenMob(now)
}

// stepChickenMob 驱动「鸡群围观」：到点随机选一名存活真人当偶像，
// 全体小鸡把家安到偶像脚下并朝他走（借 wander 机制自然聚拢）；
// 围观结束就地定居。无真人则改期重试。
func (r *Room) stepChickenMob(now time.Time) {
	if len(r.Chickens) == 0 {
		return
	}
	if !r.chickenMobUntil.IsZero() {
		if now.Before(r.chickenMobUntil) {
			idol := r.findPlayer(r.chickenMobTarget)
			if idol != nil && idol.Alive {
				for i := range r.Chickens {
					c := &r.Chickens[i]
					if !c.Alive || now.Before(c.NextTurn) {
						continue
					}
					dx, dz := idol.Pos.X-c.Pos.X, idol.Pos.Z-c.Pos.Z
					c.Home = Vec3{idol.Pos.X, idol.Pos.Y, idol.Pos.Z}
					if dx*dx+dz*dz > 0.01 {
						c.Dir = norm(Vec3{dx, 0, dz})
					} else {
						c.Dir = Vec3{} // 已经贴脸：站定仰视
					}
					c.NextTurn = now.Add(500 * time.Millisecond)
				}
				return
			}
			// 偶像阵亡/退出：就地解散
			r.endChickenMob(now)
			return
		}
		r.endChickenMob(now)
		return
	}
	if r.nextChickenMobAt.IsZero() {
		r.nextChickenMobAt = now.Add(chickenMobFirst)
		return
	}
	if !now.Before(r.nextChickenMobAt) {
		idol := r.pickChickenMobIdol()
		gap := chickenMobMinGap + time.Duration(rand.IntN(int((chickenMobMaxGap-chickenMobMinGap)/time.Second)))*time.Second
		r.nextChickenMobAt = now.Add(gap)
		if idol == nil {
			r.nextChickenMobAt = now.Add(chickenMobRetry)
			return
		}
		r.chickenMobTarget = idol.Id
		r.chickenMobUntil = now.Add(chickenMobDuration)
		if r.hasAnyHumanOnline() {
			r.Emit(Event{Type: EvChat, Player: 0, Name: "战场播报",
				Message: fmt.Sprintf("🎤🐣 小鸡们找到了新偶像：%s！全体围观！", idol.Name)})
		}
	}
}

func (r *Room) endChickenMob(now time.Time) {
	for i := range r.Chickens {
		c := &r.Chickens[i]
		if c.Alive {
			c.Home = c.Pos // 就地定居
		}
	}
	if r.hasAnyHumanOnline() {
		r.Emit(Event{Type: EvChat, Player: 0, Name: "战场播报", Message: "小鸡围观结束，就地解散定居"})
	}
	r.chickenMobUntil = time.Time{}
	r.chickenMobTarget = 0
}

func (r *Room) pickChickenMobIdol() *PlayerState {
	humans := make([]*PlayerState, 0, 4)
	for _, p := range r.Players {
		if !p.IsBot && p.Alive {
			humans = append(humans, &p.PlayerState)
		}
	}
	if len(humans) == 0 {
		return nil
	}
	return humans[rand.IntN(len(humans))]
}

func (r *Room) hasAnyHumanOnline() bool {
	for _, p := range r.Players {
		if !p.IsBot {
			return true
		}
	}
	return false
}

// chickenShot tests the pellet ray against every live chicken; the nearest hit
// before the wall (and any player) turns the bird into a fried-chicken reward.
func (r *Room) chickenShot(p *PlayerState, origin, dir Vec3, wallDist, playerDist float64, weapon uint8, now time.Time) bool {
	var best *Chicken
	bestDist := min(wallDist, playerDist)
	for i := range r.Chickens {
		c := &r.Chickens[i]
		if !c.Alive {
			continue
		}
		if d, ok := RayPlayerAABBHeight(origin, dir, c.Pos, chickenHitHeight, bestDist); ok && d < bestDist {
			best, bestDist = c, d
		}
	}
	if best == nil {
		return false
	}
	// 金鸡王：多枪才能击杀；命中即给打击反馈（EvHit，客户端有 hitmarker），
	// 但只有王驾崩才发 EvChickenDeath。击杀奖励：满血 + 满甲 + 弹药大礼包。
	if best.King {
		dmg := Weapons[weapon].Dmg
		dealt := uint16(math.Min(dmg, float64(best.KingHP)))
		best.KingHP -= dealt
		r.Emit(Event{Type: EvHit, Player: p.Id, Victim: best.Id, Dmg: uint8(math.Min(dmg, 255))})
		if best.KingHP > 0 {
			return true
		}
		best.King = false
		p.HP = uint8(p.maxHP())
		p.Armor = 100
		p.grantStreakAmmo()
		r.nextKingAt = now.Add(kingRespawnMin + time.Duration(rand.IntN(int((kingRespawnMax-kingRespawnMin)/time.Second)))*time.Second)
		for _, pl := range r.Players {
			if !pl.IsBot {
				r.Emit(Event{Type: EvChat, Player: 0, Name: "战场播报",
					Message: fmt.Sprintf("👑 %s 击杀了金鸡王！超级补给到账：满血 + 满甲 + 弹药大礼包", p.Name)})
				break
			}
		}
	} else if int(p.HP) < p.maxHP() {
		p.HP = uint8(min(p.maxHP(), int(p.HP)+25))
	}
	best.Alive = false
	best.RespawnAt = now.Add(time.Duration(15+rand.IntN(10)) * time.Second)
	r.chickenKillTitle(p)
	r.Emit(Event{Type: EvChickenDeath, Killer: p.Id, Victim: best.Id, Origin: best.Pos, Weapon: weapon})
	return true
}

// 鸡圈噩梦称号：炸鸡累计到里程碑时全房播报一次（服务端私有计数，零协议改动）。
var chickenKillTitles = map[uint8]string{
	10: "🍗 %s 已斩获 10 只小鸡，荣获「鸡圈噩梦」称号！",
	25: "🏆 %s 炸鸡数达到 25，正式加冕「炸鸡大王」！小鸡们发来抗议信",
	50: "🚨 %s 的第 50 只小鸡倒下了！禽类保护协会已介入调查",
}

func (r *Room) chickenKillTitle(p *PlayerState) {
	if p.ChickenKills == 255 {
		return
	}
	p.ChickenKills++
	title, ok := chickenKillTitles[p.ChickenKills]
	if !ok {
		return
	}
	for _, pl := range r.Players {
		if !pl.IsBot {
			r.Emit(Event{Type: EvChat, Player: 0, Name: "战场播报", Message: fmt.Sprintf(title, p.Name)})
			return
		}
	}
}

// kingChicken 返回在位的金鸡王（可能为 nil）。
func (r *Room) kingChicken() *Chicken {
	for i := range r.Chickens {
		c := &r.Chickens[i]
		if c.King && c.Alive {
			return c
		}
	}
	return nil
}

// stepChickenKing 驱动王的轮回：在位时护卫护送；空位到点则从存活小鸡中随机拥立。
func (r *Room) stepChickenKing(now time.Time) {
	if king := r.kingChicken(); king != nil {
		r.escortGuards(king)
		return
	}
	if r.nextKingAt.IsZero() {
		r.nextKingAt = now.Add(kingFirstAt)
		return
	}
	if !now.Before(r.nextKingAt) {
		r.nextKingAt = now.Add(kingRespawnMin + time.Duration(rand.IntN(int((kingRespawnMax-kingRespawnMin)/time.Second)))*time.Second)
		live := make([]*Chicken, 0, len(r.Chickens))
		for i := range r.Chickens {
			if r.Chickens[i].Alive {
				live = append(live, &r.Chickens[i])
			}
		}
		if len(live) == 0 {
			return
		}
		king := live[rand.IntN(len(live))]
		king.King = true
		king.KingHP = kingHP
		king.forceEmit = true
		r.Emit(Event{Type: EvChickenSpawn, Player: king.Id, Origin: king.Pos, Dir: king.Dir})
		for _, pl := range r.Players {
			if !pl.IsBot {
				r.Emit(Event{Type: EvChat, Player: 0, Name: "战场播报",
					Message: "👑 金鸡王混入了鸡群！护卫队会暴露它的位置——打爆它：满血 + 满甲 + 弹药大礼包"})
				break
			}
		}
	}
}

// escortGuards：离王最近的两只小鸡成为皇家护卫，贴身护送并暴露王的位置。
func (r *Room) escortGuards(king *Chicken) {
	first, second := (*Chicken)(nil), (*Chicken)(nil)
	fd2, sd2 := math.MaxFloat64, math.MaxFloat64
	for i := range r.Chickens {
		c := &r.Chickens[i]
		if !c.Alive || c == king {
			continue
		}
		dx, dz := c.Pos.X-king.Pos.X, c.Pos.Z-king.Pos.Z
		d2 := dx*dx + dz*dz
		if d2 < fd2 {
			second, sd2 = first, fd2
			first, fd2 = c, d2
		} else if d2 < sd2 {
			second, sd2 = c, d2
		}
	}
	for _, g := range []*Chicken{first, second} {
		if g == nil {
			continue
		}
		g.Home = king.Pos
		g.Dir = norm(Vec3{king.Pos.X - g.Pos.X, 0, king.Pos.Z - g.Pos.Z})
	}
}

func (r *Room) ThrowGrenade(p *PlayerState, yaw, pitch float64, now time.Time) {
	if !p.Alive || p.Grenades <= 0 || now.Before(p.NextGrenadeAt) {
		return
	}
	p.Grenades--
	p.NextGrenadeAt = now.Add(2 * time.Second)
	cp := math.Cos(pitch)
	originY := p.Pos.Y + EyeHeight
	if p.Crouch {
		originY = p.Pos.Y + CrouchEyeH
	}
	g := &Grenade{Id: r.nextNadeId, ThrowerId: p.Id, Pos: Vec3{p.Pos.X, originY, p.Pos.Z}, Vel: Vec3{-math.Sin(yaw) * cp * GrenadeThrowSpeed, math.Sin(pitch)*GrenadeThrowSpeed + GrenadeLift, -math.Cos(yaw) * cp * GrenadeThrowSpeed}, ExplodesAt: now.Add(1800 * time.Millisecond), Active: true}
	r.nextNadeId++
	r.Grenades = append(r.Grenades, g)
	r.Emit(Event{Type: EvNadeThrow, Player: p.Id, Origin: g.Pos, Dir: g.Vel})
}
func (r *Room) StepGrenades(now time.Time) {
	live := r.Grenades[:0]
	for _, g := range r.Grenades {
		if !g.Active {
			continue
		}
		if !now.Before(g.ExplodesAt) {
			g.Active = false
			r.Emit(Event{Type: EvExplosion, Origin: g.Pos})
			var thrower *PlayerState
			for _, pl := range r.Players {
				if pl.Id == g.ThrowerId {
					thrower = &pl.PlayerState
					break
				}
			}
			if thrower != nil {
				for _, pl := range r.Players {
					v := &pl.PlayerState
					if v == thrower || !v.Alive || v.ProtectedAt(now) || v.Id == thrower.IllegalMate {
						continue
					}
					d := math.Sqrt(dist2(v.Pos, g.Pos))
					if d > 7.5 {
						continue
					}
					dir := norm(Vec3{v.Pos.X - g.Pos.X, v.Pos.Y + .9 - g.Pos.Y, v.Pos.Z - g.Pos.Z})
					if hit, hd := r.World.Raycast(g.Pos, dir, d); hit && hd < d-.5 {
						continue
					}
					r.Damage(thrower, v, 85*(1-d/7.5), false, WeaponHE, now)
				}
			}
			continue
		}
		g.Vel.Y += Gravity * TickDT
		delta := Vec3{g.Vel.X * TickDT, g.Vel.Y * TickDT, g.Vel.Z * TickDT}
		travel := math.Sqrt(delta.X*delta.X + delta.Y*delta.Y + delta.Z*delta.Z)
		if travel > 0 {
			dir := Vec3{delta.X / travel, delta.Y / travel, delta.Z / travel}
			if hit, distance := r.World.Raycast(g.Pos, dir, travel); hit && distance < travel {
				stop := math.Max(0, distance-.03)
				g.Pos.X += dir.X * stop
				g.Pos.Y += dir.Y * stop
				g.Pos.Z += dir.Z * stop
				g.Vel = Vec3{}
			} else {
				g.Pos.X += delta.X
				g.Pos.Y += delta.Y
				g.Pos.Z += delta.Z
			}
		}
		if g.Pos.Y < 0 {
			g.Pos.Y = 0
			g.Vel = Vec3{}
		}
		live = append(live, g)
	}
	r.Grenades = live
}

func (r *Room) recordHistory() {
	if r.history == nil {
		r.history = make(map[uint16]*poseHistory)
	}
	for _, p := range r.Players {
		h := r.history[p.Id]
		if h == nil {
			h = &poseHistory{}
			r.history[p.Id] = h
		}
		h.samples[h.next] = poseSample{r.tick, p.Pos, p.Crouch}
		h.next = (h.next + 1) % len(h.samples)
		if h.count < len(h.samples) {
			h.count++
		}
	}
}
func (r *Room) poseAt(id uint16, tick uint32, fallback Vec3, crouch bool) poseSample {
	h := r.history[id]
	best := poseSample{tick, fallback, crouch}
	if h == nil {
		return best
	}
	start := (h.next - h.count + len(h.samples)) % len(h.samples)
	for i := 0; i < h.count; i++ {
		s := h.samples[(start+i)%len(h.samples)]
		if s.Tick <= tick {
			best = s
		} else {
			break
		}
	}
	return best
}
func dist2(a, b Vec3) float64 { dx, dy, dz := a.X-b.X, a.Y-b.Y, a.Z-b.Z; return dx*dx + dy*dy + dz*dz }
func (p *PlayerState) InputRateOK(now time.Time) bool {
	if p.inputWindowStart.IsZero() || now.Sub(p.inputWindowStart) > 5*time.Second {
		p.inputWindowStart = now
		p.inputCount = 0
	}
	p.inputCount++
	return p.inputCount <= 90*5
}
func AimDir(yaw, pitch float64) Vec3 {
	cp := math.Cos(pitch)
	return Vec3{-math.Sin(yaw) * cp, math.Sin(pitch), -math.Cos(yaw) * cp}
}
func weaponSpread(def WeaponDef, vx, vz float64, onGround, crouching, landing, aiming bool, burstShots int) float64 {
	floor := 0.0
	if isGun(def.Id) {
		floor = 0.28
		switch {
		case isShotgun(def.Id):
			floor = 1.25
		case isSniper(def.Id):
			if aiming {
				floor = 0.07
			} else {
				floor = 0
			}
		case def.Id == 0 || def.Id == 1 || def.Id == 7:
			floor = 0.22
		}
	}
	base := math.Max(def.SpreadDeg, floor)
	speed := math.Hypot(vx, vz)
	moveFactor := math.Max(0, math.Min(1, (speed-.35)/(3.5-.35)))
	spread := base + (def.MoveSpreadDeg-base)*moveFactor
	spread += math.Min(def.BloomDeg*1.6, float64(burstShots)*def.BloomDeg*.18)
	if crouching {
		if onGround && !landing && speed <= .35 && burstShots == 0 && !isShotgun(def.Id) {
			return 0
		}
		spread *= .68
	}
	if aiming && !isSniper(def.Id) {
		if isShotgun(def.Id) {
			spread *= .85
		} else {
			spread *= .62
		}
	}
	if !onGround {
		spread = math.Max(spread, def.MoveSpreadDeg*1.55+.45)
	}
	if landing {
		spread = math.Max(spread, def.MoveSpreadDeg*1.12)
	}
	return spread
}

func spreadSample(shotSeq uint16, pellets, pellet int) int {
	shot := int(uint8(shotSeq))
	if pellets > 1 {
		return shot*17 + pellet
	}
	return shot
}

func patternDir(dir Vec3, deg float64, shot, weapon int, shooter uint16) Vec3 {
	if deg <= 0 {
		return dir
	}
	seed := uint32(shot)*747796405 + uint32(weapon+1)*2891336453 + uint32(shooter)*2246822519
	seed = seed*1664525 + 1013904223
	radius := math.Sqrt(float64(seed)/4294967296) * math.Tan(deg*math.Pi/180)
	seed = seed*1664525 + 1013904223
	angle := float64(seed) / 4294967296 * math.Pi * 2
	right := norm(cross(dir, Vec3{0, 1, 0}))
	up := cross(right, dir)
	a, b := math.Cos(angle)*radius, math.Sin(angle)*radius
	return norm(Vec3{dir.X + right.X*a + up.X*b, dir.Y + right.Y*a + up.Y*b, dir.Z + right.Z*a + up.Z*b})
}

var xmPattern = [][2]float64{
	{0.18, -0.12},
	{1.00, 0.10},
	{0.62, 0.78},
	{-0.22, 0.98},
	{-0.92, 0.38},
	{-0.85, -0.52},
	{-0.08, -1.00},
	{0.78, -0.62},
}

func shotgunDir(dir Vec3, deg float64, pellet, shot, weapon int, shooter uint16) Vec3 {
	if deg <= 0 {
		return dir
	}
	o := xmPattern[pellet%len(xmPattern)]
	seed := uint32(shot*31+pellet)*747796405 + uint32(weapon+1)*2891336453 + uint32(shooter)*2246822519
	seed = seed*1664525 + 1013904223
	jx := (float64(seed)/4294967296 - 0.5) * 0.28
	seed = seed*1664525 + 1013904223
	jy := (float64(seed)/4294967296 - 0.5) * 0.28
	ring := math.Tan(deg * math.Pi / 180)
	a := (o[0] + jx) * ring
	b := (o[1] + jy) * ring
	right := norm(cross(dir, Vec3{0, 1, 0}))
	up := cross(right, dir)
	return norm(Vec3{dir.X + right.X*a + up.X*b, dir.Y + right.Y*a + up.Y*b, dir.Z + right.Z*a + up.Z*b})
}
func cross(a, b Vec3) Vec3 { return Vec3{a.Y*b.Z - a.Z*b.Y, a.Z*b.X - a.X*b.Z, a.X*b.Y - a.Y*b.X} }
func norm(v Vec3) Vec3 {
	l := math.Sqrt(v.X*v.X + v.Y*v.Y + v.Z*v.Z)
	if l < 1e-9 {
		return v
	}
	return Vec3{v.X / l, v.Y / l, v.Z / l}
}
func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }
