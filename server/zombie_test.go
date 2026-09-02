package main

import (
	"encoding/binary"
	"math"
	"strings"
	"testing"
	"time"
)

// 僵尸事件复用鸡事件的线格式（新 opcode 追加在尾部，既有编码零改动）。
func TestZombieEventEncoding(t *testing.T) {
	b := Events([]Event{{Type: EvZombieSpawn, Player: 500, Origin: Vec3{X: 1.5, Y: 2, Z: 3}, Dir: Vec3{X: -1}}})
	if len(b) != 29 || b[0] != OpEvents || b[1] != 1 || b[2] != EvZombieSpawn || binary.LittleEndian.Uint16(b[3:]) != 500 {
		t.Fatalf("bad zombie spawn event: %v", b)
	}
	if f := float32FromBits(binary.LittleEndian.Uint32(b[5:])); f != 1.5 {
		t.Fatalf("bad zombie origin X: %v", f)
	}
	b = Events([]Event{{Type: EvZombieDeath, Killer: 7, Victim: 501, Origin: Vec3{}, Weapon: 6}})
	if len(b) != 20 || b[2] != EvZombieDeath || binary.LittleEndian.Uint16(b[3:]) != 7 || binary.LittleEndian.Uint16(b[5:]) != 501 || b[19] != 6 {
		t.Fatalf("bad zombie death event: %v", b)
	}
}

func float32FromBits(b uint32) float64 {
	return float64(math.Float32frombits(b))
}

func newBloodMoonRoom(t *testing.T) *Room {
	store, err := NewStore(t.TempDir() + "/stats.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	r := NewRoom(1, &World{Size: [2]float64{96, 96}}, store)
	human := newTestHuman(10, r)
	human.Pos = Vec3{0, 0, 0}
	r.Players = append(r.Players, human)
	return r
}

// 血月生命周期：到点刷潮 + 播报 → 月落晨光净化 + 清场。
func TestBloodMoonLifecycle(t *testing.T) {
	r := newBloodMoonRoom(t)
	now := time.Now()
	r.pending = nil

	r.stepBloodMoon(now) // 首调只排期
	if len(r.pending) != 0 || len(r.Zombies) != 0 {
		t.Fatal("排期阶段不应刷僵尸")
	}

	r.nextBloodMoonAt = now
	before := len(r.pending)
	r.stepBloodMoon(now)
	if r.bloodMoonUntil.IsZero() || !now.Before(r.bloodMoonUntil) {
		t.Fatal("血月应处于进行中")
	}
	if len(r.Zombies) != zombieBaseWave {
		t.Fatalf("首波僵尸 = %d, 期望 %d", len(r.Zombies), zombieBaseWave)
	}
	announced := false
	names := 0
	spawns := 0
	for _, e := range r.pending[before:] {
		if e.Type == EvChat && strings.Contains(e.Message, "血月升起") {
			announced = true
		}
		if e.Type == EvPlayerName && strings.Contains(e.Name, "血月僵尸") {
			names++
		}
		if e.Type == EvZombieSpawn {
			spawns++
		}
	}
	if !announced || names != zombieBaseWave || spawns != zombieBaseWave {
		t.Fatalf("月升播报=%v 名字=%d 生成=%d", announced, names, spawns)
	}
	if r.nextZombieId < 500 {
		t.Fatalf("僵尸 ID 段应从 500 起, 实际 %d", r.nextZombieId)
	}

	// 月落：全部净化（Killer=0）、清场、黎明播报。
	dawn := now.Add(bloodMoonDuration + time.Second)
	before = len(r.pending)
	r.stepBloodMoon(dawn)
	if !r.bloodMoonUntil.IsZero() || len(r.Zombies) != 0 {
		t.Fatal("月落后应清场")
	}
	purified := false
	for _, e := range r.pending[before:] {
		if e.Type == EvChat && strings.Contains(e.Message, "黎明降临") {
			purified = true
		}
		if e.Type == EvZombieDeath && e.Killer != 0 {
			t.Fatal("净化死亡的 Killer 应为 0（晨光）")
		}
	}
	if !purified {
		t.Fatal("月落应有黎明播报")
	}
}

// 僵尸 AI：向最近玩家移动 + 贴脸咬人掉血；咬死走独立结算。
func TestZombieChaseBiteAndKillCredit(t *testing.T) {
	r := newBloodMoonRoom(t)
	now := time.Now()
	human := &r.Players[0].PlayerState
	z := Zombie{Id: 500, HP: zombieHP, Alive: true, forceEmit: true}
	z.Pos = Vec3{4, 0, 0}
	r.Zombies = append(r.Zombies, z)
	r.bloodMoonUntil = now.Add(time.Minute)

	// 追击：向玩家方向推进。
	for range 10 {
		r.stepZombies(now)
		now = now.Add(time.Second / TickRate)
	}
	zp := &r.Zombies[0]
	if zp.Pos.X >= 4 {
		t.Fatalf("僵尸应向玩家（x=0）推进, 实际 x=%.2f", zp.Pos.X)
	}

	// 瞬移贴脸咬一口。
	zp.Pos = Vec3{0.8, 0, 0}
	hpBefore := human.HP
	before := len(r.pending)
	r.stepZombies(now)
	bites := 0
	for _, e := range r.pending[before:] {
		if e.Type == EvHit && e.Player == 500 && e.Victim == 10 {
			bites++
			if e.Dmg != zombieDmg {
				t.Fatalf("咬伤 = %d, 期望 %d", e.Dmg, zombieDmg)
			}
		}
	}
	if bites != 1 || human.HP != hpBefore-zombieDmg {
		t.Fatalf("贴脸应咬一口: bites=%d hp=%d→%d", bites, hpBefore, human.HP)
	}

	// 咬死：冷却过后（700ms）HP=1 被咬 → Alive=false + EvKill（Killer=僵尸）。
	human.HP = 1
	now = now.Add(time.Second)
	before = len(r.pending)
	r.stepZombies(now)
	if human.Alive {
		t.Fatal("HP=1 被咬应死亡")
	}
	killed := false
	for _, e := range r.pending[before:] {
		if e.Type == EvKill && e.Killer == 500 && e.Victim == 10 {
			killed = true
		}
	}
	if !killed {
		t.Fatal("咬死应产生 EvKill 且 Killer 为僵尸")
	}
	if human.RespawnAt.IsZero() || human.Deaths != 1 {
		t.Fatal("死亡结算应含重生排期与阵亡计数")
	}
	// 僵尸咬人冷却：紧随其后的一步不应再次掉血（玩家已死则更不会有目标）。
	stepBefore := len(r.pending)
	r.stepZombies(now)
	extra := 0
	for _, e := range r.pending[stepBefore:] {
		if e.Type == EvHit && e.Player == 500 {
			extra++
		}
	}
	if extra != 0 {
		t.Fatal("咬人冷却期内不应再次掉血")
	}
}

// 玩家射僵尸：多枪掉血 + EvHit 反馈 + 击杀回血奖励；弹道被僵尸挡下。
func TestZombieShotMultiHitAndReward(t *testing.T) {
	r := newBloodMoonRoom(t)
	now := time.Now()
	shooter := &r.Players[0].PlayerState // AK-47：33 伤害
	shooter.HP = 60
	z := Zombie{Id: 500, HP: zombieHP, Alive: true, forceEmit: true}
	z.Pos = Vec3{5, 0, 5}
	r.Zombies = append(r.Zombies, z)
	r.bloodMoonUntil = now.Add(time.Minute)

	shots := 0
	for {
		shots++
		if shots > 10 {
			t.Fatal("僵尸血量异常，未能击杀")
		}
		before := len(r.pending)
		hit := r.zombieShot(shooter, Vec3{5, 0.8, 8}, Vec3{0, 0, -1}, 1e9, 1e9, 3, now)
		if !hit {
			t.Fatalf("第 %d 枪应命中僵尸", shots)
		}
		death := false
		for _, e := range r.pending[before:] {
			if e.Type == EvZombieDeath && e.Victim == 500 {
				death = true
			}
		}
		if death {
			break
		}
		if shooter.HP != 60 {
			t.Fatal("未击杀前不应回血")
		}
	}
	want := (zombieHP + uint16(Weapons[3].Dmg) - 1) / uint16(Weapons[3].Dmg)
	if shots != int(want) {
		t.Fatalf("击杀僵尸需 %d 枪, 期望 %d", shots, want)
	}
	if shooter.HP != 60+zombieKillReward {
		t.Fatalf("击杀奖励回血 = %d, 期望 %d", shooter.HP, 60+zombieKillReward)
	}
	if r.Zombies[0].Alive {
		t.Fatal("僵尸应已死亡")
	}
}
