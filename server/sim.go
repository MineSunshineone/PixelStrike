package main

import (
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
	{0, "Glock-18", 21, 3.0, 480, .24, 1.05, .09, 1.0, .58, 30, 180, 1400, false, 1},
	{1, "Desert Eagle", 44, 2.45, 250, .14, 1.65, .30, .98, .93, 11, 53, 1800, false, 1},
	{2, "MP5-SD", 21, 3.0, 820, .34, 1.15, .05, 1.0, .65, 45, 180, 1800, true, 1},
	{3, "AK-47", 33, 4.0, 600, .26, 1.6, .14, .92, .78, 45, 135, 2200, true, 1},
	{4, "M4A4", 29, 3.5, 690, .20, 1.35, .09, .93, .72, 45, 135, 2100, true, 1},
	{5, "AWP", 103, 1.25, 32, .015, 4.4, 0, .76, .98, 8, 45, 2800, false, 1},
	{6, "Knife", 34, 1, 150, 0, 0, 0, 1.08, 1, 0, 0, 0, false, 1},
	{7, "USP-S", 24, 3.6, 400, .11, 1.0, .16, 1.0, .62, 18, 36, 1700, false, 1},
	{8, "UMP-45", 25, 2.8, 620, .42, 1.25, .08, .97, .70, 38, 150, 2100, true, 1},
	{9, "FAMAS", 28, 3.2, 720, .28, 1.45, .11, .94, .68, 38, 135, 2200, true, 1},
	{10, "AUG", 27, 3.5, 600, .16, 1.1, .08, .88, .75, 45, 135, 2300, true, 1},
	{11, "SSG 08", 72, 2.0, 48, .025, 3.4, 0, .80, .82, 12, 120, 2600, false, 1},
	{12, "XM1014", 14, 1.25, 180, 2.6, 3.6, .28, .93, .62, 8, 36, 2600, false, 6},
}

func isGun(id uint8) bool    { return int(id) < len(Weapons) && id != 6 }
func isSniper(id uint8) bool { return id == 5 || id == 11 }
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
	TickRate        = 60
	TickDT          = 1.0 / TickRate
	WalkSpeed       = 6.4
	GroundAccel     = 44.0
	StopAccel       = 60.0
	AirAccel        = 9.5
	CrouchSpeed     = .6
	Gravity         = -22.0
	JumpVel         = 8.4
	MaxRewindTicks  = 8
	MaxHP           = 100
	SpawnProtectS   = 2 * time.Second
	AWPScopeTime    = 320 * time.Millisecond
	RespawnDelayS   = 3 * time.Second
	EyeHeight       = 1.7
	CrouchEyeH      = 1.12
	StandingHeight  = 2.1
	CrouchingHeight = 1.3
	FlightSpeed     = WalkSpeed
	MaxFlightHeight = StandingHeight * 25
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
	LandingUntil, AimStarted, SpeedUntil                           time.Time
	Grenades                                                       int
	Kills, Deaths                                                  uint16
	LastInputSeq, LastShotSeq                                      uint16
	HasShot                                                        bool
	LastShotAt                                                     time.Time
	ShotCounter                                                    uint8
	inputWindowStart                                               time.Time
	inputCount                                                     int
}

func (p *PlayerState) ProtectedAt(now time.Time) bool {
	return p.Alive && now.Before(p.InvincibleUntil)
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
	if now.Before(p.SpeedUntil) {
		speed *= speedBoostMultiplier
	}
	if p.Crouch {
		speed *= CrouchSpeed
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
	if weapon == 6 && mode&1 != 0 {
		gap = time.Second
	}
	if p.NextFire.IsZero() || now.Sub(p.NextFire) >= gap {
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
		p.InvincibleUntil = time.Time{}
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
	pellets := 1
	if isGun(uint8(weapon)) && def.Pellets > 1 {
		pellets = def.Pellets
	}
	for n := 0; n < pellets; n++ {
		shotDir := dir
		if isGun(uint8(weapon)) {
			shotDir = patternDir(dir, spread, spreadSample(shotSeq, pellets, n), weapon, p.Id)
		}
		_, worldDist := r.World.Raycast(origin, shotDir, maxDist)
		var target *PlayerState
		targetDist, hitY, hitHeight := maxDist, 0.0, StandingHeight
		for _, other := range r.Players {
			o := &other.PlayerState
			if o == p || !o.Alive || o.ProtectedAt(now) {
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
		} else if hitY <= .65 {
			dmg *= .75
		}
		r.Damage(p, target, dmg, headshot, uint8(weapon), now)
		if !target.Alive && pellets == 1 {
			break
		}
	}
	return true
}

func (r *Room) Damage(attacker, victim *PlayerState, dmg float64, headshot bool, weapon uint8, now time.Time) {
	if !victim.Alive || victim.ProtectedAt(now) {
		return
	}
	actual := dmg
	if victim.Armor > 0 && isGun(weapon) {
		actual = dmg * Weapons[weapon].ArmorPen
		lost := uint8(math.Min(float64(victim.Armor), math.Ceil((dmg-actual)*.5)))
		victim.Armor -= lost
	}
	d := uint8(math.Max(1, math.Min(actual, float64(victim.HP))))
	victim.HP -= d
	hs := uint8(0)
	if headshot {
		hs = 1
	}
	r.Emit(Event{Type: EvHit, Player: attacker.Id, Victim: victim.Id, Dmg: d, Headshot: hs})
	if victim.HP > 0 {
		return
	}
	r.Emit(Event{Type: EvKill, Killer: attacker.Id, Victim: victim.Id, Weapon: weapon, Headshot: hs})
	attacker.Kills++
	victim.Deaths++
	if !attacker.IsBot {
		r.Store.Accumulate(attacker.Account, 1, 0)
		if !victim.IsBot && isGun(weapon) {
			r.Store.AccumulateWeaponKill(attacker.Account, weapon)
		}
	}
	if !victim.IsBot {
		r.Store.Accumulate(victim.Account, 0, 1)
	}
	victim.Alive, victim.Reloading = false, false
	victim.RespawnAt = now.Add(RespawnDelayS)
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
	p.Reloading = true
	p.ReloadEnd = now.Add(time.Duration(def.ReloadMs) * time.Millisecond)
	p.NextFire = p.ReloadEnd
	r.Emit(Event{Type: EvReloadStart, Player: p.Id, Ms: uint16(def.ReloadMs)})
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
	p.HP = MaxHP
	p.Armor = 100
	p.Alive = true
	p.OnGround, p.Crouch, p.Flying = true, false, false
	p.CmdKeys = 0
	p.ApplyLoadout(p.Primary, p.Secondary)
	p.InvincibleUntil = now.Add(SpawnProtectS)
	p.LandingUntil, p.AimStarted = time.Time{}, time.Time{}
	p.SpeedUntil = time.Time{}
	p.RespawnAt = time.Time{}
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
	pickupCount = 12
)

const (
	speedBoostDuration   = 8 * time.Second
	speedBoostMultiplier = 1.35
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
		if p.HP >= MaxHP {
			return false
		}
		p.HP = uint8(min(MaxHP, int(p.HP)+50))
	case PickupSpeed:
		p.SpeedUntil = now.Add(speedBoostDuration)
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

func (r *Room) ThrowGrenade(p *PlayerState, yaw, pitch float64, now time.Time) {
	if !p.Alive || p.Grenades <= 0 || now.Before(p.NextGrenadeAt) {
		return
	}
	p.Grenades--
	p.NextGrenadeAt = now.Add(2 * time.Second)
	cp := math.Cos(pitch)
	g := &Grenade{Id: r.nextNadeId, ThrowerId: p.Id, Pos: Vec3{p.Pos.X, p.Pos.Y + EyeHeight, p.Pos.Z}, Vel: Vec3{-math.Sin(yaw) * cp * 22, math.Sin(pitch)*22 + 3.2, -math.Cos(yaw) * cp * 22}, ExplodesAt: now.Add(1800 * time.Millisecond), Active: true}
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
					if v == thrower || !v.Alive || v.ProtectedAt(now) {
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
	moveFactor := math.Min(1, math.Hypot(vx, vz)/3)
	spread := def.SpreadDeg + (def.MoveSpreadDeg-def.SpreadDeg)*moveFactor
	// Bloom used to dump the whole spray into a random cone, which made
	// recoil unreadable. Keep a tiny residual so long sprays aren't lasers.
	spread += math.Min(.22, float64(burstShots)*def.BloomDeg*.12)
	if !onGround {
		spread = math.Max(spread, def.MoveSpreadDeg*1.55+.45)
	}
	if crouching {
		spread *= .55
	}
	if aiming && !isSniper(def.Id) {
		spread *= .48
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
func cross(a, b Vec3) Vec3 { return Vec3{a.Y*b.Z - a.Z*b.Y, a.Z*b.X - a.X*b.Z, a.X*b.Y - a.Y*b.X} }
func norm(v Vec3) Vec3 {
	l := math.Sqrt(v.X*v.X + v.Y*v.Y + v.Z*v.Z)
	if l < 1e-9 {
		return v
	}
	return Vec3{v.X / l, v.Y / l, v.Z / l}
}
func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }
