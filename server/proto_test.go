package main

import (
	"encoding/binary"
	"math"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func TestWelcomeV6(t *testing.T) {
	b := Welcome(42, 0x12345678)
	if len(b) != 8 || b[0] != OpWelcome || b[1] != ProtocolVersion || binary.LittleEndian.Uint16(b[2:]) != 42 {
		t.Fatalf("bad welcome: %v", b)
	}
}
func TestSnapshotCarriesSkinAndUltimate(t *testing.T) {
	p := &Player{PlayerState: PlayerState{Id: 1, Alive: true, Skin: 7, WeaponSkin: 2}}
	state := quantizeState(&p.PlayerState, 0)
	full := p.BuildSnapshot(0, []*Player{p}, []quantState{state}, time.Unix(0, 0))
	if len(full) != 32 || full[7] != 1 || binary.LittleEndian.Uint16(full[10:]) != 0x8000 || full[29] != 7 || full[30] != 2 || full[31] != 0 {
		t.Fatalf("full skin snapshot = %v", full)
	}
	p.Ultimate = UltimateGhost
	state = quantizeState(&p.PlayerState, 1000)
	clear(p.netCache)
	ultimateFull := p.BuildSnapshot(1, []*Player{p}, []quantState{state}, time.Unix(1000, 0))
	if len(ultimateFull) != 32 || ultimateFull[31] != UltimateGhost {
		t.Fatalf("ultimate full snapshot = %v", ultimateFull)
	}

	p.Skin = 3
	p.Ultimate = 0
	state = quantizeState(&p.PlayerState, 0)
	clear(p.netCache)
	p.netFullAt = map[uint16]uint32{1: 2}
	skinDelta := p.BuildSnapshot(2, []*Player{p}, []quantState{state}, time.Unix(0, 0))
	if len(skinDelta) != 32 || binary.LittleEndian.Uint16(skinDelta[10:]) != 1<<15 || skinDelta[29] != 3 || skinDelta[31] != 0 {
		t.Fatalf("delta skin snapshot = %v", skinDelta)
	}

	p.WeaponSkin = 1
	state = quantizeState(&p.PlayerState, 0)
	clear(p.netCache)
	weaponSkinDelta := p.BuildSnapshot(3, []*Player{p}, []quantState{state}, time.Unix(0, 0))
	if len(weaponSkinDelta) != 32 || binary.LittleEndian.Uint16(weaponSkinDelta[10:]) != 1<<15 || weaponSkinDelta[30] != 1 {
		t.Fatalf("delta weapon skin snapshot = %v", weaponSkinDelta)
	}
}

func TestUnchangedSnapshotIsSkipped(t *testing.T) {
	p := &Player{PlayerState: PlayerState{Id: 1, Alive: true}}
	players := []*Player{p}
	states := []quantState{quantizeState(&p.PlayerState, 0)}
	now := time.Unix(0, 0)
	if first := p.BuildSnapshot(0, players, states, now); first == nil {
		t.Fatal("initial snapshot was skipped")
	}
	if unchanged := p.BuildSnapshot(1, players, states, now); unchanged != nil {
		t.Fatalf("unchanged snapshot = %v, want nil", unchanged)
	}
}

func TestMaintenanceNotice(t *testing.T) {
	b := Maintenance(2)
	if len(b) != 2 || b[0] != OpMaintenance || b[1] != 2 {
		t.Fatalf("bad maintenance notice: %v", b)
	}
}

func TestQueuedInputKeepsLatest(t *testing.T) {
	p := &Player{}
	now := time.Now()
	p.queueInput(1, KeyForward, .1, .2, now)
	p.queueInput(2, KeyRight|KeyAim, .3, .4, now.Add(time.Millisecond))
	p.applyQueuedInput()
	if p.LastInputSeq != 2 || p.CmdKeys != KeyRight|KeyAim || p.Yaw != .3 || p.Pitch != .4 || !p.AimStarted.Equal(now.Add(time.Millisecond)) {
		t.Fatalf("applied input = seq:%d keys:%d yaw:%v pitch:%v aim:%v", p.LastInputSeq, p.CmdKeys, p.Yaw, p.Pitch, p.AimStarted)
	}
}

func TestSnapshotDropForcesKeyframe(t *testing.T) {
	p := &Player{send: make(chan []byte, 1), latestSnapshot: make(chan []byte, 1), snapshotBuffers: make(chan []byte, 2)}
	p.Send([]byte{OpSnapshot, 1})
	p.Send([]byte{OpSnapshot, 2})
	select {
	case got := <-p.latestSnapshot:
		t.Fatalf("kept undecodable delta snapshot %v", got)
	default:
	}
	if !p.netReset.Load() || len(p.snapshotBuffers) != 2 {
		t.Fatal("dropped snapshots did not reset the baseline and release buffers")
	}
	p.PlayerState = PlayerState{Id: 1, Alive: true, HP: 100}
	state := quantizeState(&p.PlayerState, time.Now().UnixNano())
	p.netCache = map[uint16]quantState{1: state}
	p.netFullAt = map[uint16]uint32{1: 1}
	snapshot := p.BuildSnapshot(2, []*Player{p}, []quantState{state}, time.Unix(0, 0))
	if len(snapshot) < 12 || binary.LittleEndian.Uint16(snapshot[10:]) != 0x8000 {
		t.Fatalf("snapshot after drop is not a keyframe: %v", snapshot)
	}
}

func TestWebSocketHeartbeatToleratesThrottledBrowserTimers(t *testing.T) {
	if pingPeriod*2 >= readDeadline {
		t.Fatalf("heartbeat %v cannot keep read deadline %v alive", pingPeriod, readDeadline)
	}
	if writeChanSize < 64 {
		t.Fatalf("write queue too small for short event bursts: %d", writeChanSize)
	}
}

func TestBalanceValues(t *testing.T) {
	// 月球跳（Gravity=-15 / JumpVel=10.5）是有意的平衡变更，见 sim.go 常数注释。
	if RespawnDelayS != 3*time.Second || WalkSpeed != 6.4 || GroundAccel != 44 || StopAccel != 60 || AirAccel != 9.5 || JumpVel != 10.5 || MaxRewindTicks != 8 {
		t.Fatalf("unexpected movement balance")
	}
	wantDamage := []float64{23, 44, 22, 33, 29, 103, 34, 24, 27, 29, 27, 72, 17}
	wantMag := []int{30, 11, 45, 45, 45, 8, 0, 18, 38, 38, 45, 12, 7}
	wantReserve := []int{180, 53, 180, 135, 135, 45, 0, 36, 150, 135, 135, 120, 32}
	for i, weapon := range Weapons {
		if weapon.Dmg != wantDamage[i] || weapon.Mag != wantMag[i] || weapon.Reserve != wantReserve[i] {
			t.Fatalf("%s balance = %.0f %d/%d", weapon.Name, weapon.Dmg, weapon.Mag, weapon.Reserve)
		}
	}
	now := time.Now()
	r := &Room{}
	attacker := &PlayerState{IsBot: true, Alive: true}
	victim := &PlayerState{IsBot: true, Alive: true, HP: 1, Weapon: 3}
	r.Damage(attacker, victim, 10, false, 3, now)
	if victim.RespawnAt.Sub(now) != 3*time.Second {
		t.Fatalf("wrong respawn delay: %v", victim.RespawnAt.Sub(now))
	}
	if len(r.pending) != 2 || r.pending[0].Type != EvHit || r.pending[1].Type != EvKill {
		t.Fatalf("unexpected lethal events: %#v", r.pending)
	}
}

func TestUltimatePointsAndDeathReset(t *testing.T) {
	r := &Room{}
	store, err := NewStore(t.TempDir() + "/stats.db")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	r.Store = store
	attacker := &PlayerState{Id: 1, Alive: true, IsBot: false, HP: MaxHP}
	human := &PlayerState{Id: 2, Alive: true, IsBot: false, HP: 1}
	bot := &PlayerState{Id: 3, Alive: true, IsBot: true, HP: 1}
	now := time.Unix(1, 0)
	r.Damage(attacker, bot, 10, false, 3, now)
	if attacker.UltimatePoints != 1 {
		t.Fatalf("bot kill points = %d, want 1", attacker.UltimatePoints)
	}
	for i := 2; i < UltimateRequirement; i++ {
		human.HP = 1
		human.Alive = true
		r.Damage(attacker, human, 10, false, 3, now)
		if int(attacker.UltimatePoints) != i {
			t.Fatalf("human kill points = %d, want %d", attacker.UltimatePoints, i)
		}
	}
	human.HP = 1
	human.Alive = true
	r.Damage(attacker, human, 10, false, 3, now)
	if !r.CastUltimate(attacker, UltimateGhost, now) || attacker.UltimatePoints != 0 {
		t.Fatalf("ghost cast failed: points=%d ultimate=%d", attacker.UltimatePoints, attacker.Ultimate)
	}
	if !attacker.GhostAt(now) {
		t.Fatal("ghost state inactive")
	}

	attacker.HP = 1
	r.Damage(human, attacker, 10, false, 3, now)
	if attacker.Ultimate != 0 || !attacker.GhostUntil.IsZero() {
		t.Fatalf("death did not reset ultimate: %d %v", attacker.Ultimate, attacker.GhostUntil)
	}
}

func TestUltimateEffects(t *testing.T) {
	r := &Room{World: &World{}, history: make(map[uint16]*poseHistory)}
	blackDream := &PlayerState{Id: 1, Alive: true, IsBot: true}
	bot := &PlayerState{Id: 2, Alive: true, IsBot: true}
	human := &PlayerState{Id: 3, Alive: true}
	ghost := &PlayerState{Id: 4, Alive: true}
	invincible := &PlayerState{Id: 5, Alive: true, HP: 1}
	players := []*Player{
		{PlayerState: *blackDream},
		{PlayerState: *bot},
		{PlayerState: *human},
		{PlayerState: *ghost},
		{PlayerState: *invincible},
	}
	for _, p := range players {
		p.ApplyLoadout(3, 0)
	}
	blackDream, bot, human, ghost, invincible = &players[0].PlayerState, &players[1].PlayerState, &players[2].PlayerState, &players[3].PlayerState, &players[4].PlayerState
	r.Players = players
	now := time.Unix(1, 0)
	blackDream.UltimatePoints = UltimateRequirement
	ghost.UltimatePoints = UltimateRequirement
	invincible.UltimatePoints = UltimateRequirement
	if !r.CastUltimate(blackDream, UltimateBlackDream, now) || !r.AnyBlackDream(now) {
		t.Fatalf("black dream cast failed: points=%d ultimate=%d", blackDream.UltimatePoints, blackDream.Ultimate)
	}
	bot.NextFire = time.Time{}
	if r.TryFire(bot, 0, 0, 0, 0, 1, now) {
		t.Fatal("bot fired during black dream")
	}
	if !r.TryFire(human, 0, 0, 0, 0, 1, now) {
		t.Fatal("human was forbidden to fire during black dream")
	}

	if !r.CastUltimate(ghost, UltimateGhost, now) {
		t.Fatal("ghost cast failed")
	}
	startSpeed := WalkSpeed * Weapons[3].SpeedMult
	r.Move(ghost, now)
	if got := math.Hypot(ghost.Vel.X, ghost.Vel.Z); got > startSpeed*GhostSpeedMultiplier+1e-9 {
		t.Fatalf("ghost exceeded capped speed: %.3f", got)
	}

	if !r.CastUltimate(invincible, UltimateInvincible, now) {
		t.Fatal("invincible cast failed")
	}
	r.Damage(human, invincible, 100, false, 3, now)
	if invincible.HP != 1 || !invincible.Alive {
		t.Fatalf("invincible player took damage: hp=%d alive=%v", invincible.HP, invincible.Alive)
	}
}

func TestHitEventCarriesHeadshotWithoutGrowing(t *testing.T) {
	b := Events([]Event{{Type: EvHit, Player: 1, Victim: 2, Dmg: 42, Headshot: 1}})
	if len(b) != 8 || b[7] != (42|0x80) {
		t.Fatalf("bad headshot hit event: %v", b)
	}
}

func TestCeilingCollisionIsNotGround(t *testing.T) {
	w := &World{aabbs: []AABB{{Min: Vec3{-2, 2, -2}, Max: Vec3{2, 2.5, 2}}}}
	pos, vel := Vec3{}, Vec3{Y: 7}
	if w.MoveAABB(&pos, &vel, .1, StandingHeight, false) {
		t.Fatal("ceiling collision reported as grounded")
	}
	w.aabbs = []AABB{{Min: Vec3{-2, -1, -2}, Max: Vec3{2, 0, 2}}}
	pos, vel = Vec3{Y: .1}, Vec3{Y: -2}
	if !w.MoveAABB(&pos, &vel, .1, StandingHeight, false) {
		t.Fatal("floor collision did not report grounded")
	}
}

func TestSweptCollisionAndCrouchClearance(t *testing.T) {
	w := &World{aabbs: []AABB{{Min: Vec3{1, 0, -2}, Max: Vec3{1.05, 3, 2}}}}
	pos, vel := Vec3{}, Vec3{X: 20}
	w.MoveAABB(&pos, &vel, .1, StandingHeight, false)
	if pos.X > 1-PlayerHalf || vel.X != 0 || !w.CanOccupy(pos, StandingHeight) {
		t.Fatalf("swept wall failed: pos=%v vel=%v", pos, vel)
	}

	w.aabbs = []AABB{{Min: Vec3{-2, 1.4, -2}, Max: Vec3{2, 2, 2}}}
	if !w.CanOccupy(Vec3{}, CrouchingHeight) || w.CanOccupy(Vec3{}, StandingHeight) {
		t.Fatal("crouch clearance check failed")
	}
}

func TestDiagonalWallSlideDoesNotStickOrTunnel(t *testing.T) {
	w := &World{aabbs: []AABB{{Min: Vec3{1, 0, -10}, Max: Vec3{1.05, 3, 10}}}}
	pos, vel := Vec3{}, Vec3{X: 20, Z: 6}
	w.MoveAABB(&pos, &vel, .1, StandingHeight, false)
	if pos.X > 1-PlayerHalf || pos.Z < .59 || vel.X != 0 || !w.CanOccupy(pos, StandingHeight) {
		t.Fatalf("diagonal slide failed: pos=%v vel=%v", pos, vel)
	}
	for range 20 {
		vel.X, vel.Z = 6, 6
		w.MoveAABB(&pos, &vel, 1.0/60, StandingHeight, false)
	}
	if pos.Z < 2.5 || !w.CanOccupy(pos, StandingHeight) {
		t.Fatalf("wall movement stuck or penetrated: pos=%v", pos)
	}
}

func TestJumpRequiresGround(t *testing.T) {
	w := &World{aabbs: []AABB{{Min: Vec3{-20, -1, -20}, Max: Vec3{20, 0, 20}}}}
	r := &Room{World: w}
	p := &PlayerState{Alive: true, OnGround: true, Pos: Vec3{Y: Epsilon}, CmdKeys: KeyJump | KeyForward}
	p.ApplyLoadout(3, 0)
	maxY, maxSpeed := p.Pos.Y, 0.0
	jumps := 0
	for range 180 {
		wasGrounded := p.OnGround
		r.Move(p, time.Now())
		if wasGrounded && !p.OnGround {
			jumps++
		}
		maxY = max(maxY, p.Pos.Y)
		maxSpeed = max(maxSpeed, math.Hypot(p.Vel.X, p.Vel.Z))
	}
	speedLimit := WalkSpeed * Weapons[3].SpeedMult
	if maxY < 1.45 || jumps < 2 || maxSpeed > speedLimit+1e-9 {
		t.Fatalf("bad bunny hop: apex=%.3f jumps=%d speed=%.3f limit=%.3f", maxY, jumps, maxSpeed, speedLimit)
	}
	p.Pos.Y, p.OnGround, p.Vel.Y = 2, false, -1
	r.Move(p, time.Now())
	if p.Vel.Y >= 0 {
		t.Fatal("jump without solid support was accepted")
	}
}

func TestLastRoundStartsReload(t *testing.T) {
	r := &Room{World: &World{}, history: make(map[uint16]*poseHistory)}
	p := &PlayerState{Alive: true, OnGround: true}
	p.ApplyLoadout(3, 0)
	p.Mags[0] = 1
	now := time.Unix(1, 0)
	if !r.TryFire(p, 0, 0, 0, 0, 1, now) || p.Mags[0] != 0 || !p.Reloading {
		t.Fatalf("last round did not start reload: mag=%d reloading=%v", p.Mags[0], p.Reloading)
	}
}

func TestReloadRequestDoesNotRestartActiveReload(t *testing.T) {
	r := &Room{}
	p := &PlayerState{Alive: true}
	p.ApplyLoadout(3, 0)
	p.Mags[0]--
	now := time.Unix(1, 0)
	if !r.StartReload(p, now) {
		t.Fatal("initial reload rejected")
	}
	deadline := p.ReloadEnd
	if r.StartReload(p, now.Add(time.Second)) || !p.ReloadEnd.Equal(deadline) || len(r.pending) != 1 {
		t.Fatalf("active reload restarted: end=%v events=%d", p.ReloadEnd, len(r.pending))
	}
}

func TestReloadAfterRejectedLastRoundRefillsMagazine(t *testing.T) {
	player := &Player{PlayerState: PlayerState{Alive: true, OnGround: true}}
	p := &player.PlayerState
	r := &Room{World: &World{}, Players: []*Player{player}, history: make(map[uint16]*poseHistory)}
	p.ApplyLoadout(3, 0)
	p.Mags[0] = 1
	now := time.Unix(1, 0)
	p.NextFire = now.Add(time.Second)
	if r.TryFire(p, 0, 0, 0, 0, 1, now) || p.Mags[0] != 1 {
		t.Fatalf("rejected shot consumed last round: mag=%d", p.Mags[0])
	}
	if !r.StartReload(p, now) {
		t.Fatal("reload after rejected last round was rejected")
	}
	r.FinishReloads(p.ReloadEnd)
	if p.Mags[0] != Weapons[3].Mag {
		t.Fatalf("reload left %d rounds, want %d", p.Mags[0], Weapons[3].Mag)
	}
}

func TestGrenadeThrowConsumesOnceAndEmitsTrajectory(t *testing.T) {
	r := &Room{}
	p := &PlayerState{Id: 7, Alive: true, Crouch: true, Grenades: 1}
	now := time.Unix(1, 0)
	r.ThrowGrenade(p, .4, .2, now)
	r.ThrowGrenade(p, .4, .2, now)
	if p.Grenades != 0 || len(r.Grenades) != 1 || len(r.pending) != 1 {
		t.Fatalf("duplicate grenade throw: grenades=%d live=%d events=%d", p.Grenades, len(r.Grenades), len(r.pending))
	}
	e := r.pending[0]
	if e.Type != EvNadeThrow || e.Player != p.Id || math.Abs(math.Hypot(e.Dir.X, e.Dir.Z)-math.Cos(.2)*GrenadeThrowSpeed) > 1e-9 || e.Dir.Y <= GrenadeLift || e.Origin.Y != CrouchEyeH {
		t.Fatalf("grenade trajectory event missing: %#v", e)
	}
}

func TestRevengeShotHeadshotsAnyoneOnScreen(t *testing.T) {
	now := time.Unix(1, 0)
	attacker := &Player{PlayerState: PlayerState{
		Id: 1, Alive: true, IsBot: true, HP: MaxHP, RevengeActive: true, RevengeShots: 10,
		InvincibleUntil: now.Add(10 * time.Second),
	}}
	victim := &Player{PlayerState: PlayerState{
		Id: 2, Alive: true, IsBot: true, HP: MaxHP, Armor: 100, Pos: Vec3{X: 8, Z: -6},
	}}
	attacker.ApplyLoadout(3, 0)
	r := &Room{World: &World{}, Players: []*Player{attacker, victim}, history: make(map[uint16]*poseHistory)}
	if !r.TryFire(&attacker.PlayerState, 0, 0, 0, 0, 1, now) {
		t.Fatal("revenge shot rejected")
	}
	if victim.Alive || victim.HP != 0 {
		t.Fatalf("on-screen target should be headshot without aiming: alive=%v hp=%d", victim.Alive, victim.HP)
	}
	if attacker.RevengeShots != 9 {
		t.Fatalf("revenge ammo not consumed: %d", attacker.RevengeShots)
	}
	behind := &Player{PlayerState: PlayerState{Id: 3, Alive: true, IsBot: true, HP: MaxHP, Armor: 100, Pos: Vec3{Z: 8}}}
	r.Players = []*Player{attacker, behind}
	attacker.NextFire = time.Time{}
	if !r.TryFire(&attacker.PlayerState, 0, 0, 0, 0, 2, now.Add(time.Second)) {
		t.Fatal("second revenge shot rejected")
	}
	if !behind.Alive || behind.HP != MaxHP {
		t.Fatal("player outside the view should not be auto-headshot")
	}
	if attacker.RevengeShots != 8 {
		t.Fatalf("every gunshot should burn revenge ammo: %d", attacker.RevengeShots)
	}
	attacker.InvincibleUntil = now
	attacker.NextFire = time.Time{}
	next := &Player{PlayerState: PlayerState{Id: 4, Alive: true, IsBot: true, HP: MaxHP, Armor: 100, Pos: Vec3{X: -4, Z: -5}}}
	r.Players = []*Player{attacker, next}
	if !r.TryFire(&attacker.PlayerState, 0, 0, 0, 0, 3, now.Add(11*time.Second)) {
		t.Fatal("revenge shot after invuln expired rejected")
	}
	if next.Alive || attacker.RevengeShots != 7 || !attacker.RevengeActive {
		t.Fatalf("invuln expiry must not end the 10-shot budget: alive=%v shots=%d active=%v", next.Alive, attacker.RevengeShots, attacker.RevengeActive)
	}
}

func TestKnifeAttackDoesNotRequireAmmoAndRespectsCadence(t *testing.T) {
	r := &Room{World: &World{}, history: make(map[uint16]*poseHistory)}
	attacker := &Player{PlayerState: PlayerState{Id: 1, Alive: true, IsBot: true, HP: MaxHP, Pos: Vec3{}}}
	victim := &Player{PlayerState: PlayerState{Id: 2, Alive: true, IsBot: true, HP: MaxHP, Pos: Vec3{Z: -1.2}, Yaw: math.Pi}}
	attacker.ApplyLoadout(3, 0)
	attacker.SwitchSlot(3)
	attacker.NextFire = time.Time{}
	r.Players = []*Player{attacker, victim}
	start := time.Unix(1, 0)
	if !r.TryFire(&attacker.PlayerState, 0, 0, 0, 0, 1, start) || victim.HP != 66 || attacker.NextFire.Sub(start) != KnifeSlashInterval {
		t.Fatalf("light knife attack failed: hp=%d next=%v", victim.HP, attacker.NextFire.Sub(start))
	}
	victim.HP, victim.Alive = MaxHP, true
	heavyAt := start.Add(KnifeSlashInterval + 200*time.Millisecond)
	if !r.TryFire(&attacker.PlayerState, 0, 0, 1, 0, 2, heavyAt) || victim.HP != 45 || attacker.NextFire.Sub(heavyAt) != KnifeHeavyInterval {
		t.Fatalf("heavy knife attack failed: hp=%d next=%v", victim.HP, attacker.NextFire.Sub(heavyAt))
	}
	if r.TryFire(&attacker.PlayerState, 0, 0, 0, 0, 3, heavyAt.Add(KnifeHeavyInterval-time.Millisecond)) || !r.TryFire(&attacker.PlayerState, 0, 0, 0, 0, 3, heavyAt.Add(KnifeHeavyInterval)) {
		t.Fatal("knife cooldown accepted an early swing or rejected an on-time swing")
	}
}

func TestArsenalRolesStaySeparated(t *testing.T) {
	ak, m4, aug := Weapons[3], Weapons[4], Weapons[10]
	if aug.Dmg*aug.ArmorPen >= ak.Dmg*ak.ArmorPen {
		t.Fatalf("AUG body (%.1f) should not match AK (%.1f)", aug.Dmg*aug.ArmorPen, ak.Dmg*ak.ArmorPen)
	}
	if ak.Dmg*ak.HeadMult*ak.ArmorPen < MaxHP {
		t.Fatal("AK armored headshot should be lethal")
	}
	deagle := Weapons[1]
	if deagle.Dmg*deagle.HeadMult*deagle.ArmorPen < MaxHP {
		t.Fatal("Deagle armored headshot should be lethal")
	}
	awp := Weapons[5]
	if awp.Dmg*awp.ArmorPen < MaxHP {
		t.Fatal("AWP armored body shot should be lethal")
	}
	if m4.SpreadDeg >= ak.SpreadDeg {
		t.Fatal("M4 should stay more accurate than AK")
	}
	if Weapons[7].Dmg >= 30 {
		t.Fatalf("USP damage too rifle-like: %v", Weapons[7].Dmg)
	}
	ssg := Weapons[11]
	if ssg.Dmg*ssg.ArmorPen >= MaxHP {
		t.Fatalf("SSG should not body one-shot: %.1f", ssg.Dmg*ssg.ArmorPen)
	}
	xm := Weapons[12]
	if float64(xm.Pellets)*xm.Dmg < MaxHP {
		t.Fatalf("XM should one-tap unarmored if the cone connects: %.1f", float64(xm.Pellets)*xm.Dmg)
	}
	if float64(xm.Pellets)*xm.Dmg*xm.ArmorPen >= MaxHP {
		t.Fatalf("XM should not armor-dump with all pellets: %.1f", float64(xm.Pellets)*xm.Dmg*xm.ArmorPen)
	}
	if xm.Rpm < 180 || !xm.Automatic {
		t.Fatal("XM follow-up is still too slow")
	}
	if xm.SpreadDeg > 2.0 {
		t.Fatalf("XM cone still too wide: %.2f", xm.SpreadDeg)
	}
	if Weapons[8].SpreadDeg > Weapons[2].SpreadDeg+0.05 {
		t.Fatal("UMP spread is still unusable next to MP5")
	}
	if Weapons[2].SpreadDeg >= .7 {
		t.Fatal("MP5 spread still unusable")
	}
	if Weapons[8].Rpm >= Weapons[2].Rpm {
		t.Fatal("UMP should not out-cycle MP5")
	}
}

func TestLoadoutAcceptsExpandedArsenal(t *testing.T) {
	if !validLoadout(12, 7) || !validLoadout(11, 1) || !validLoadout(8, 0) {
		t.Fatal("new primary/secondary pairs rejected")
	}
	if validLoadout(7, 3) || validLoadout(6, 0) || validLoadout(3, 8) {
		t.Fatal("invalid loadout accepted")
	}
}

func TestWeaponSpreadStaysControllable(t *testing.T) {
	ak := Weapons[3]
	first := weaponSpread(ak, 0, 0, true, false, false, false, 0)
	hipSpray := weaponSpread(ak, 0, 0, true, false, false, false, 20)
	ads := weaponSpread(ak, 0, 0, true, false, false, true, 20)
	if first > ak.SpreadDeg+0.05 {
		t.Fatalf("first shot inaccuracy too high: %.3f", first)
	}
	if hipSpray > first+0.3 {
		t.Fatalf("spray bloom still dominates recoil: first=%.3f spray=%.3f", first, hipSpray)
	}
	if ads >= hipSpray {
		t.Fatalf("ADS did not tighten inaccuracy: ads=%.3f hip=%.3f", ads, hipSpray)
	}
	adsFirst := weaponSpread(ak, 0, 0, true, false, false, true, 0)
	if adsFirst < 0.26 {
		t.Fatalf("ADS first shot is still a laser: %.3f", adsFirst)
	}
	if adsFirst >= first-0.05 {
		t.Fatalf("ADS should still tighten first shot: ads=%.3f hip=%.3f", adsFirst, first)
	}
	awpScoped := weaponSpread(Weapons[5], 0, 0, true, false, false, true, 0)
	if awpScoped < 0.06 {
		t.Fatalf("scoped AWP has no spread: %.3f", awpScoped)
	}
}

func TestPatternDirStaysInsideSpread(t *testing.T) {
	aim := AimDir(.3, -.2)
	limit := math.Cos(2 * math.Pi / 180)
	for shot := 1; shot <= 64; shot++ {
		got := patternDir(aim, 2, shot, 3, 7)
		if dot := aim.X*got.X + aim.Y*got.Y + aim.Z*got.Z; dot < limit-1e-12 {
			t.Fatalf("shot %d left spread cone: cos=%.9f limit=%.9f", shot, dot, limit)
		}
	}
	if patternDir(aim, 2, 1, 3, 7) == patternDir(aim, 2, 1, 3, 8) {
		t.Fatal("different players received the same spread pattern")
	}
}

func TestSnapshotOrdersNearbyPlayersFirst(t *testing.T) {
	recv := &Player{PlayerState: PlayerState{Id: 5, Alive: true, Pos: Vec3{}}}
	far := &Player{PlayerState: PlayerState{Id: 1, Alive: true, Pos: Vec3{Z: 90}}}
	near := &Player{PlayerState: PlayerState{Id: 2, Alive: true, Pos: Vec3{Z: 8}}}
	players := []*Player{far, recv, near}
	states := make([]quantState, 3)
	for i, p := range players {
		states[i] = quantizeState(&p.PlayerState, 0)
	}
	snap := recv.BuildSnapshot(0, players, states, time.Unix(0, 0))
	if snap[7] != 3 {
		t.Fatalf("expected 3 players, got %d", snap[7])
	}
	id0 := binary.LittleEndian.Uint16(snap[8:])
	id1 := binary.LittleEndian.Uint16(snap[8+24:])
	id2 := binary.LittleEndian.Uint16(snap[8+48:])
	if id0 != 5 || id1 != 2 || id2 != 1 {
		t.Fatalf("snapshot order = %d,%d,%d want self,near,far", id0, id1, id2)
	}
}

func TestSpreadSampleKeepsSingleShotsAndSeparatesPellets(t *testing.T) {
	if got := spreadSample(259, 1, 0); got != 3 {
		t.Fatalf("single-shot sample = %d, want 3", got)
	}
	if a, b := spreadSample(259, 6, 0), spreadSample(259, 6, 1); a != 51 || b != 52 {
		t.Fatalf("pellet samples = %d,%d, want 51,52", a, b)
	}
}

func TestSnapshotShotUsesPersistentSequence(t *testing.T) {
	p := &PlayerState{ShotCounter: 1, LastShotSeq: 257}
	first := quantizeState(p, 0).shot
	p.LastShotSeq++
	p.ShotCounter = 1
	second := quantizeState(p, 0).shot
	if first != 1 || second != 2 {
		t.Fatalf("shot sequence = %d,%d, want 1,2", first, second)
	}
}

func TestAutomaticFireKeepsCadenceAcrossFrames(t *testing.T) {
	r := &Room{World: &World{}, history: make(map[uint16]*poseHistory)}
	p := &PlayerState{Alive: true, OnGround: true}
	p.ApplyLoadout(2, 0)
	now := time.Unix(1, 0)
	if !r.TryFire(p, 0, 0, 0, 0, 1, now) {
		t.Fatal("first shot rejected")
	}
	firstDeadline := p.NextFire
	gap := time.Duration(60 / Weapons[2].Rpm * float64(time.Second))
	if !r.TryFire(p, 0, 0, 0, 0, 2, firstDeadline.Add(8*time.Millisecond)) {
		t.Fatal("late frame shot rejected")
	}
	if want := firstDeadline.Add(gap); !p.NextFire.Equal(want) {
		t.Fatalf("fire cadence drifted: got %v want %v", p.NextFire, want)
	}
}

func TestDeltaSnapshotIsSmallerThanKeyframe(t *testing.T) {
	now := time.Now()
	receiver := &Player{PlayerState: PlayerState{Id: 1, Alive: true, HP: 100, Armor: 100, InvincibleUntil: now.Add(time.Second)}}
	receiver.ApplyLoadout(3, 0)
	other := &Player{PlayerState: PlayerState{Id: 2, Alive: true, HP: 100, Armor: 100, Pos: Vec3{10, 0, 10}}}
	other.ApplyLoadout(4, 0)
	players := []*Player{receiver, other}
	states := quantizePlayers(nil, players, now.UnixNano())
	full := receiver.BuildSnapshot(0, players, states, now)
	other.Pos.X += .1
	states = quantizePlayers(states, players, now.UnixNano())
	delta := receiver.BuildSnapshot(2, players, states, now)
	if len(delta) >= len(full) || delta[7] == 0 {
		t.Fatalf("full=%d delta=%d records=%d", len(full), len(delta), delta[7])
	}
}

func TestMapSupportsHundredPlayerRoom(t *testing.T) {
	w, err := LoadWorld("../map.json")
	if err != nil {
		t.Fatal(err)
	}
	if w.Size != [2]float64{512, 512} || len(w.Spawns) < 64 {
		t.Fatalf("size=%v spawns=%d", w.Size, len(w.Spawns))
	}
	for i, spawn := range w.Spawns {
		box := playerBox(Vec3{spawn[0], spawn[1], spawn[2]}, StandingHeight)
		for _, block := range w.aabbs {
			if intersects(box, block) {
				t.Fatalf("spawn %d intersects map: %v", i, spawn)
			}
		}
	}
}

func TestBestSpawnNeverReturnsUnscoredOrigin(t *testing.T) {
	w := &World{Spawns: [][3]float64{{10, 0, 10}, {20, 0, 10}, {10, 0, 20}, {20, 0, 20}}}
	r := &Room{World: w}
	for id, spawn := range w.Spawns {
		r.Players = append(r.Players, &Player{PlayerState: PlayerState{Id: uint16(id + 1), Alive: true, Pos: Vec3{spawn[0], spawn[1], spawn[2]}}})
	}
	if got := r.BestSpawn(&PlayerState{}); got == (Vec3{}) {
		t.Fatal("crowded spawn selection returned unscored origin")
	}
}

func TestEveryGunRewardsCrouchingAndPenalizesMovement(t *testing.T) {
	for _, def := range Weapons {
		if !isGun(def.Id) {
			continue
		}
		aiming := isSniper(def.Id)
		crouched := weaponSpread(def, 0, 0, true, true, false, aiming, 0)
		standing := weaponSpread(def, 0, 0, true, false, false, aiming, 0)
		nearStopped := weaponSpread(def, .35, 0, true, false, false, aiming, 0)
		moving := weaponSpread(def, 3.5, 0, true, false, false, aiming, 0)
		if isShotgun(def.Id) {
			if !(crouched < standing) {
				t.Fatalf("%s crouched spread=%v standing=%v", def.Name, crouched, standing)
			}
		} else if crouched != 0 {
			t.Fatalf("%s stationary crouched first shot is not exact: %v", def.Name, crouched)
		}
		if nearStopped != standing || standing >= moving {
			t.Fatalf("%s spread stopped=%v standing=%v moving=%v", def.Name, nearStopped, standing, moving)
		}
	}
}

func TestStreakBuffsRespectCaps(t *testing.T) {
	p := &PlayerState{Streak: 20}
	if p.streakDamageMul() > streakDmgCap+1e-9 {
		t.Fatalf("damage mul %v exceeds cap", p.streakDamageMul())
	}
	if p.streakSpeedMul() > streakSpeedCap+1e-9 {
		t.Fatalf("speed mul %v exceeds cap", p.streakSpeedMul())
	}
	if p.streakScale() > streakScaleCap {
		t.Fatalf("scale %d exceeds cap", p.streakScale())
	}
	if botSkill(99) > maxBotSkill {
		t.Fatalf("bot skill %d exceeds cap", botSkill(99))
	}
}

func TestStreakPicksHealWhenLowAndAmmoWhenDry(t *testing.T) {
	r := &Room{history: make(map[uint16]*poseHistory)}
	now := time.Unix(1, 0)
	hurt := &PlayerState{Id: 1, Alive: true, IsBot: true, HP: 30, Armor: 10, Primary: 3, Secondary: 0, ActiveSlot: 1, Weapon: 3, Mags: [2]int{30, 12}, Reserves: [2]int{90, 24}}
	victim := &PlayerState{Id: 11, Alive: true, IsBot: true, HP: 1}
	hurt.Streak = 1
	r.Damage(hurt, victim, 80, false, 3, now)
	if hurt.HP <= 30 {
		t.Fatalf("low HP should get heal, hp=%d", hurt.HP)
	}
	var healKind uint8
	for _, e := range r.pending {
		if e.Type == EvStreakBuff {
			healKind = e.Kind
		}
	}
	if healKind&StreakHeal == 0 {
		t.Fatalf("expected heal, got kind=%d", healKind)
	}

	r.pending = nil
	dry := &PlayerState{Id: 2, Alive: true, IsBot: true, HP: MaxHP, Armor: 100, Primary: 3, Secondary: 0, ActiveSlot: 1, Weapon: 3, Mags: [2]int{0, 12}, Reserves: [2]int{0, 24}}
	victim2 := &PlayerState{Id: 12, Alive: true, IsBot: true, HP: 1}
	dry.Streak = 1
	r.Damage(dry, victim2, 80, false, 3, now)
	if dry.Mags[0] == 0 {
		t.Fatal("empty mag should get ammo")
	}
	var ammoKind uint8
	for _, e := range r.pending {
		if e.Type == EvStreakBuff {
			ammoKind = e.Kind
		}
	}
	if ammoKind&StreakAmmo == 0 {
		t.Fatalf("expected ammo, got kind=%d", ammoKind)
	}
}

func TestBotMarksRevengeOnDeath(t *testing.T) {
	killer := &Player{PlayerState: PlayerState{Id: 1, Alive: true, IsBot: true, HP: MaxHP}}
	victim := &Player{PlayerState: PlayerState{Id: 2, Alive: true, IsBot: true, HP: 1}}
	r := &Room{Players: []*Player{killer, victim}, botAIs: map[uint16]*BotAI{2: {}}, history: make(map[uint16]*poseHistory)}
	now := time.Now()
	r.Damage(&killer.PlayerState, &victim.PlayerState, 80, false, 3, now)
	ai := r.botAIs[2]
	if ai == nil || ai.RevengeID != 1 || !now.Before(ai.RevengeUntil) {
		t.Fatalf("bot did not mark revenge: %+v", ai)
	}
}

func TestLastHumanClosesBotRoom(t *testing.T) {
	human := &Player{PlayerState: PlayerState{Id: 1}}
	bot := &Player{PlayerState: PlayerState{Id: 2, IsBot: true}}
	r := &Room{Players: []*Player{human, bot}, botAIs: map[uint16]*BotAI{2: {}}, history: make(map[uint16]*poseHistory)}
	r.Remove(human)
	if !r.Empty() || len(r.Players) != 0 {
		t.Fatalf("room not closed: closed=%v players=%d", r.closed, len(r.Players))
	}
}

func TestEventsNeverSplitsUTF8Rune(t *testing.T) {
	name := strings.Repeat("汉", 25) // 75 bytes > 64 limit
	b := Events([]Event{{Type: EvPlayerName, Player: 1, Name: name}})
	n := int(b[5])
	got := string(b[6 : 6+n])
	if !utf8.ValidString(got) {
		t.Fatalf("truncated name is not valid UTF-8: %q", got)
	}
	if !strings.HasPrefix(name, got) || len(got)%3 != 0 {
		t.Fatalf("bad truncation: %q", got)
	}
}

func TestChatEncodingAndSanitize(t *testing.T) {
	text := "  收到，A点集合！\n\r\u0000  "
	want := "收到，A点集合！"
	if got := sanitizeChat(text); got != want {
		t.Fatalf("sanitizeChat = %q, want %q", got, want)
	}
	long := strings.Repeat("汉", 140)
	if got := sanitizeChat(long); len([]rune(got)) != maxChatRunes {
		t.Fatalf("sanitized chat rune count = %d", len([]rune(got)))
	}
	b := Events([]Event{{Type: EvChat, Player: 7, Name: "甲", Message: "你好"}})
	if len(b) != 16 || b[0] != OpEvents || b[1] != 1 || b[2] != EvChat || binary.LittleEndian.Uint16(b[3:]) != 7 || b[5] != 3 || string(b[6:9]) != "甲" || b[9] != 6 || string(b[10:]) != "你好" {
		t.Fatalf("bad chat event: %v", b)
	}
}

func TestBondAvengeSurvivesRespawnAndScoresOnce(t *testing.T) {
	now := time.Unix(100, 0)
	attacker := &Player{PlayerState: PlayerState{Id: 1, IsBot: true, Alive: true, BondMate: 2}}
	mate := &Player{PlayerState: PlayerState{Id: 2, IsBot: true, BondMate: 1, LastKiller: 3, KilledAt: now, Pos: Vec3{X: 100}}}
	killer := &Player{PlayerState: PlayerState{Id: 3, IsBot: true, Alive: true, HP: 1}}
	r := &Room{World: &World{Spawns: [][3]float64{{100, 0, 0}}}, Players: []*Player{attacker, mate, killer}, history: make(map[uint16]*poseHistory)}
	r.Respawn(&mate.PlayerState, now.Add(RespawnDelayS))
	if mate.LastKiller != killer.Id {
		t.Fatal("respawn cleared the active revenge target")
	}
	r.pending = nil
	r.Damage(&attacker.PlayerState, &killer.PlayerState, 10, false, 3, now.Add(4*time.Second))
	if attacker.BondScore != 1 || mate.LastKiller != 0 {
		t.Fatalf("revenge score=%d last killer=%d", attacker.BondScore, mate.LastKiller)
	}
	bondEvents := 0
	for _, e := range r.pending {
		if e.Type == EvBondEvent {
			bondEvents++
			if e.Kind != 1 || e.Dmg != 1 {
				t.Fatalf("bad revenge event: %#v", e)
			}
		}
	}
	if bondEvents != 1 {
		t.Fatalf("revenge emitted %d bond events", bondEvents)
	}

	r.pending = nil
	killer.Alive, killer.HP = true, 1
	r.Damage(&attacker.PlayerState, &killer.PlayerState, 10, false, 3, now.Add(5*time.Second))
	for _, e := range r.pending {
		if e.Type == EvBondEvent {
			t.Fatalf("same death rewarded twice: %#v", e)
		}
	}
}

func TestKillingBondMateIsNotCover(t *testing.T) {
	attacker := &Player{PlayerState: PlayerState{Id: 1, IsBot: true, Alive: true, BondMate: 2}}
	mate := &Player{PlayerState: PlayerState{Id: 2, IsBot: true, Alive: true, HP: 1, BondMate: 1}}
	r := &Room{Players: []*Player{attacker, mate}, history: make(map[uint16]*poseHistory)}
	r.Damage(&attacker.PlayerState, &mate.PlayerState, 10, false, 3, time.Unix(1, 0))
	for _, e := range r.pending {
		if e.Type == EvBondEvent {
			t.Fatalf("killing the bond mate emitted cover: %#v", e)
		}
	}
}

func TestBondEventEncoding(t *testing.T) {
	b := Events([]Event{{Type: EvBondEvent, Player: 1, Victim: 2, Kind: 1, Dmg: 7, Name: "甲"}})
	if len(b) != 13 || b[0] != OpEvents || b[1] != 1 || b[2] != EvBondEvent || binary.LittleEndian.Uint16(b[3:]) != 1 || binary.LittleEndian.Uint16(b[5:]) != 2 || b[7] != 1 || b[8] != 7 || b[9] != 3 || string(b[10:]) != "甲" {
		t.Fatalf("bad bond event: %v", b)
	}
}

func TestChickenEventEncoding(t *testing.T) {
	b := Events([]Event{{Type: EvChickenSpawn, Player: 300, Origin: Vec3{X: 1.5, Y: 2, Z: 3}, Dir: Vec3{X: -1}}})
	if len(b) != 29 || b[0] != OpEvents || b[1] != 1 || b[2] != EvChickenSpawn || binary.LittleEndian.Uint16(b[3:]) != 300 {
		t.Fatalf("bad chicken spawn event: %v", b)
	}
	if f := math.Float32frombits(binary.LittleEndian.Uint32(b[5:])); f != 1.5 {
		t.Fatalf("bad chicken origin X: %v", f)
	}
	b = Events([]Event{{Type: EvChickenDeath, Killer: 7, Victim: 301, Origin: Vec3{}, Weapon: 6}})
	if len(b) != 20 || b[2] != EvChickenDeath || binary.LittleEndian.Uint16(b[3:]) != 7 || binary.LittleEndian.Uint16(b[5:]) != 301 || b[19] != 6 {
		t.Fatalf("bad chicken death event: %v", b)
	}
}

func TestPoseHistoryRing(t *testing.T) {
	p := &Player{PlayerState: PlayerState{Id: 1}}
	r := &Room{Players: []*Player{p}, history: make(map[uint16]*poseHistory)}
	for i := uint32(0); i < 20; i++ {
		r.tick, p.Pos.X = i, float64(i)
		r.recordHistory()
	}
	if got := r.poseAt(1, 10, Vec3{}, false); got.Tick != 10 || got.Pos.X != 10 {
		t.Fatalf("pose at 10 = %#v", got)
	}
	if got := r.poseAt(1, 2, Vec3{X: -1}, false); got.Pos.X != -1 {
		t.Fatalf("expired pose did not use fallback: %#v", got)
	}
}

func TestSanitizeNameUTF8(t *testing.T) {
	if s := sanitizeName("小明\xff\xfe<bot>"); s != "小明bot" {
		t.Fatalf("sanitizeName = %q", s)
	}
	if s := sanitizeName(strings.Repeat("汉", 20)); len([]rune(s)) != 16 {
		t.Fatalf("sanitizeName len = %d", len([]rune(s)))
	}
	if s := sanitizeName(" \t\n "); s != "" {
		t.Fatalf("blank sanitizeName = %q", s)
	}
}

func TestIPIsTheProgressionAccount(t *testing.T) {
	s, err := NewStore(t.TempDir() + "/stats.db")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	a := s.GetOrCreatePlayer("203.0.113.10", "browser-a", "Alice")
	b := s.GetOrCreatePlayer("203.0.113.10", "browser-b", "Bob")
	if a != "Alice" || b != "Alice" {
		t.Fatalf("same IP did not resolve to one account: %q %q", a, b)
	}
}

func TestWeaponSkinUnlocks(t *testing.T) {
	s, err := NewStore(t.TempDir() + "/stats.db")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	name := s.GetOrCreatePlayer("203.0.113.11", "ignored", "Alice")
	for range GoldKillRequirement {
		s.AccumulateWeaponKill(name, 3)
	}
	s.Flush()
	if got := s.UnlockedWeaponSkin(name, 3, 1); got != 1 {
		t.Fatalf("gold skin = %d", got)
	}
	if got := s.UnlockedWeaponSkin(name, 3, 2); got != 0 {
		t.Fatalf("diamond unlocked early = %d", got)
	}
	progress, err := s.WeaponProgress(name)
	if err != nil || len(progress) != 1 || progress[0].Kills != GoldKillRequirement || !progress[0].Gold || progress[0].Diamond {
		t.Fatalf("weapon progress = %#v, %v", progress, err)
	}
}

func TestBotKillsDoNotUnlockWeaponSkins(t *testing.T) {
	store, err := NewStore(t.TempDir() + "/stats.db")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	room := &Room{Store: store}
	attacker := &PlayerState{Name: "Manual Display Name", Account: "Alice"}
	room.Damage(attacker, &PlayerState{Alive: true, IsBot: true, HP: 1}, 10, false, 3, time.Now())
	store.Flush()
	progress, err := store.WeaponProgress(attacker.Account)
	if err != nil || len(progress) != 0 {
		t.Fatalf("bot kill changed weapon progress: %#v, %v", progress, err)
	}
	room.Damage(attacker, &PlayerState{Alive: true, HP: 1}, 10, false, 3, time.Now())
	store.Flush()
	progress, err = store.WeaponProgress(attacker.Account)
	if err != nil || len(progress) != 1 || progress[0].Kills != 1 {
		t.Fatalf("human kill did not change weapon progress: %#v, %v", progress, err)
	}
}

func TestCrouchedPlayerCanMove(t *testing.T) {
	w := &World{aabbs: []AABB{{Min: Vec3{-20, -1, -20}, Max: Vec3{20, 0, 20}}}}
	p := &PlayerState{Alive: true, OnGround: true, Pos: Vec3{Y: Epsilon}, CmdKeys: KeyCrouch | KeyForward}
	p.ApplyLoadout(3, 0)
	(&Room{World: w}).Move(p, time.Now())
	if !p.Crouch || p.Vel.Z >= 0 {
		t.Fatalf("crouched movement failed: crouch=%v vel=%v", p.Crouch, p.Vel)
	}
}

func TestFlightMovementAndBounds(t *testing.T) {
	w := &World{
		Size:  [2]float64{512, 512},
		aabbs: []AABB{{Min: Vec3{-256, -1, -256}, Max: Vec3{256, 0, 256}}},
	}
	r := &Room{World: w}
	p := &PlayerState{Alive: true, Flying: true, Pos: Vec3{X: 255.6, Y: MaxFlightHeight - .01}, CmdKeys: KeyRight | KeyJump}
	p.ApplyLoadout(3, 0)
	for range 120 {
		r.Move(p, time.Now())
	}
	if p.Pos.X > 256-PlayerHalf || p.Pos.Y > MaxFlightHeight {
		t.Fatalf("flight escaped map bounds: pos=%v", p.Pos)
	}
	if p.Vel.Y != 0 {
		t.Fatalf("flight ceiling did not stop ascent: vel=%v", p.Vel)
	}

	p.Pos.Y, p.Vel.Y, p.CmdKeys = 1, 0, KeyDescend
	for range 120 {
		r.Move(p, time.Now())
	}
	if p.Pos.Y < 0 || p.Vel.Y < 0 {
		t.Fatalf("flight descended through ground: pos=%v vel=%v", p.Pos, p.Vel)
	}
}

func TestFlightSnapshotStateBit(t *testing.T) {
	state := quantizeState(&PlayerState{Alive: true, Flying: true}, 0)
	if state.state&32 == 0 {
		t.Fatalf("flight state bit missing: %08b", state.state)
	}
}
