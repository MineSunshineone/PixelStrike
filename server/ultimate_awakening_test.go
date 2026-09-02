package main

import (
	"testing"
	"time"
)

func newAwakenRoom(t *testing.T) *Room {
	store, err := NewStore(t.TempDir() + "/stats.db")
	if err != nil {
		t.Fatal(err)
	}
	r := NewRoom(1, &World{Size: [2]float64{96, 96}}, store)
	// 测试世界补一块地面 AABB（y: -1~0），否则玩家悬空下坠走空中加速度。
	r.World.aabbs = append(r.World.aabbs, AABB{Min: Vec3{X: -48, Y: -1, Z: -48}, Max: Vec3{X: 48, Y: 0, Z: 48}})
	a := newTestHuman(10, r)
	a.Pos = Vec3{0, 0, 0}
	b := newTestHuman(11, r)
	b.Pos = Vec3{4, 0, 0}
	r.Players = append(r.Players, a, b)
	return r
}

func chargeUltimate(p *PlayerState) {
	p.UltimatePoints = UltimateRequirement
}

func TestAwakenedUltimatesCastAndExpire(t *testing.T) {
	r := newAwakenRoom(t)
	now := time.Now()
	a := &r.Players[0].PlayerState
	chargeUltimate(a)

	for _, kind := range []uint8{UltimateThorns, UltimateVampire, UltimateRampage} {
		a.Ultimate, a.UltimatePoints = 0, UltimateRequirement
		if !r.CastUltimate(a, kind, now) {
			t.Fatalf("kind %d 应可施放", kind)
		}
		if a.Ultimate != kind {
			t.Fatalf("施放后 Ultimate = %d, 期望 %d", a.Ultimate, kind)
		}
		if a.UltimatePoints != 0 {
			t.Fatalf("施放后点数应清零, 实际 %d", a.UltimatePoints)
		}
	}
	// 到期：Step 结算并发结束事件（施放事件已在前面的 pending 里，不计）。
	a.Ultimate, a.ThornsUntil = UltimateThorns, now.Add(-time.Second)
	stepBefore := len(r.pending)
	r.Step(now)
	if a.Ultimate != 0 {
		t.Fatal("到期后 Ultimate 应清零")
	}
	endEvents := 0
	for _, e := range r.pending[stepBefore:] {
		if e.Type == EvUltimate && e.Kind == UltimateThorns {
			endEvents++
		}
	}
	if endEvents != 1 {
		t.Fatalf("到期应发一次 EvUltimate 结束事件, 实际 %d", endEvents)
	}
	// 未觉醒的大招不可施放。
	a.UltimatePoints = 0
	if r.CastUltimate(a, UltimateThorns, now) {
		t.Fatal("点数不足不应可施放")
	}
}

// 荆棘甲：受击反弹 30%；双方都开荆棘不无限循环；反伤致死给足 credit。
func TestThornsReflectsWithoutLoop(t *testing.T) {
	r := newAwakenRoom(t)
	now := time.Now()
	a := &r.Players[0].PlayerState
	b := &r.Players[1].PlayerState
	b.ThornsUntil = now.Add(time.Minute)

	// A 攻击 B（B 有荆棘）：B 掉 20，A 被反弹 6。
	before := len(r.pending)
	r.Damage(a, b, 20, false, 3, now)
	if b.HP != MaxHP-20 {
		t.Fatalf("B 受击后 HP = %d, 期望 %d", b.HP, MaxHP-20)
	}
	// 荆棘反伤的 EvHit 方向：持甲者(11) → 攻击者(10)，伤害为 30%×20=6。
	seen := map[uint8]bool{}
	for _, e := range r.pending[before:] {
		if e.Type == EvHit && e.Player == 11 && e.Victim == 10 {
			seen[e.Dmg] = true
		}
	}
	if !seen[6] {
		t.Fatalf("应有 6 点反伤 EvHit, 实际 %v", seen)
	}
	if a.HP != MaxHP-6 {
		t.Fatalf("反伤扣血后 A HP = %d, 期望 %d", a.HP, MaxHP-6)
	}

	// 双方荆棘：A 也开荆棘，B 攻击 A——反伤只走一层，不无限递归。
	a.ThornsUntil = now.Add(time.Minute)
	b.HP = MaxHP
	a.HP = MaxHP
	r.Damage(b, a, 20, false, 3, now)
	// 关键断言：能跑完即无死循环；A 受 20 正常伤害，B 吃一层反伤 6。
	if a.HP != MaxHP-20 {
		t.Fatalf("双向荆棘下 A 仍应受到正常伤害, HP=%d", a.HP)
	}
	if b.HP != MaxHP-6 {
		t.Fatalf("双向荆棘下 B 应吃一层反伤, HP=%d", b.HP)
	}

	// 反伤致死：B 残血攻击 A 的荆棘，被反死，credit 归 A。
	b.HP = 1
	r.Damage(b, a, 20, false, 3, now)
	if b.Alive {
		t.Fatal("反伤应能致死")
	}
	if a.Kills != 1 {
		t.Fatalf("荆棘致死 credit 应归 A, 实际 %d", a.Kills)
	}
}

// 吸血：造成伤害回复一半，不超过 MaxHP；自伤不吸血。
func TestVampireHealsHalfDamage(t *testing.T) {
	r := newAwakenRoom(t)
	now := time.Now()
	a := &r.Players[0].PlayerState
	b := &r.Players[1].PlayerState
	a.HP = 50
	a.LifestealUntil = now.Add(time.Minute)

	r.Damage(a, b, 20, false, 3, now)
	if a.HP != 60 {
		t.Fatalf("吸血后 HP = %d, 期望 60（50+10）", a.HP)
	}
	// 自雷（雷击/自爆）不吸血。
	a.HP = 50
	r.Damage(a, a, 20, false, 3, now)
	if a.HP != 30 {
		t.Fatalf("自伤不吸血: HP = %d, 期望 30", a.HP)
	}
}

// 狂暴：射速 ×2（gap 减半）、移速 ×1.3。
func TestRampageDoublesFireRateAndSpeed(t *testing.T) {
	r := newAwakenRoom(t)
	now := time.Now()
	a := &r.Players[0].PlayerState

	// 基准 gap：AK-47 RPM 600 → 100ms。
	r.TryFire(a, 0, 0, 0, 0, 1, now)
	baseGap := time.Duration(60.0 / Weapons[3].Rpm * float64(time.Second))
	next1 := a.NextFire.Sub(now)
	if next1 <= 0 {
		// NextFire 可能走累加分支，退化为检查不大于 baseGap
		if next1 > baseGap {
			t.Fatalf("基准 NextFire 间隔异常: %v", next1)
		}
	} else if next1 != baseGap {
		t.Fatalf("基准 gap = %v, 期望 %v", next1, baseGap)
	}

	// 狂暴下 gap 减半。
	a.RampageUntil = now.Add(time.Minute)
	a.NextFire = time.Time{}
	r.TryFire(a, 0, 0, 0, 0, 2, now)
	got := a.NextFire.Sub(now)
	if got != baseGap/2 {
		t.Fatalf("狂暴 gap = %v, 期望 %v", got, baseGap/2)
	}

	// 移速：按住前进键模拟 Move，狂暴加速应生效（yaw=0 前进方向为 -Z）。
	r2 := newAwakenRoom(t)
	m := &r2.Players[0].PlayerState
	m.CmdKeys = KeyForward
	r2.Move(m, now)
	for range 20 {
		r2.Move(m, now)
	}
	normal := m.Vel.Z
	m2 := &r2.Players[1].PlayerState
	m2.RampageUntil = now.Add(time.Minute)
	m2.CmdKeys = KeyForward
	for range 20 {
		r2.Move(m2, now)
	}
	if m2.Vel.Z >= normal {
		t.Fatalf("狂暴移速应更快（更负的 -Z）: normal=%.2f rage=%.2f", normal, m2.Vel.Z)
	}
}
