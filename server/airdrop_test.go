package main

import (
	"strings"
	"testing"
	"time"
)

func newAirdropRoom(t *testing.T) *Room {
	store, err := NewStore(t.TempDir() + "/stats.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	r := NewRoom(1, &World{Size: [2]float64{96, 96}, Spawns: [][3]float64{{0, 0, 0}, {20, 0, 20}}}, store)
	human := newTestHuman(10, r)
	human.Pos = Vec3{0, 0, 0}
	r.Players = append(r.Players, human)
	return r
}

// 空投生命周期：到点播报 + 箱子落地 → 45 秒无人认领 → 自动回收。
func TestAirdropLifecycle(t *testing.T) {
	r := newAirdropRoom(t)
	now := time.Now()
	r.pending = nil

	r.stepAirdrops(now) // 排期
	if len(r.pending) != 0 {
		t.Fatal("排期阶段不应有事件")
	}

	r.nextAirdropAt = now
	before := len(r.pending)
	r.stepAirdrops(now)
	if len(r.airdropDeadlines) != 1 {
		t.Fatal("应有 1 个在期空投")
	}
	announced := false
	spawned := false
	for _, e := range r.pending[before:] {
		if e.Type == EvChat && strings.Contains(e.Message, "运输机") {
			announced = true
		}
		if e.Type == EvPickupSpawn && e.Kind == PickupAirdrop {
			spawned = true
		}
	}
	if !announced || !spawned {
		t.Fatalf("空投应播报并生成箱子, announced=%v spawned=%v", announced, spawned)
	}
	crateId := r.airdropDeadlines[0].pickupId
	if crateId < airdropIdBase {
		t.Fatalf("空投箱 ID = %d, 应 ≥ %d", crateId, airdropIdBase)
	}

	// 未认领到期：回收（EvPickupTaken Victim=0）。
	dawn := now.Add(airdropLinger + time.Second)
	before = len(r.pending)
	r.stepAirdrops(dawn)
	recycled := false
	for _, e := range r.pending[before:] {
		if e.Type == EvPickupTaken && e.Player == crateId && e.Victim == 0 {
			recycled = true
		}
	}
	if !recycled {
		t.Fatal("到期空投应回收")
	}
	for i := range r.Pickups {
		if r.Pickups[i].Id == crateId && r.Pickups[i].Active {
			t.Fatal("回收后箱子应休眠")
		}
	}
}

// 抢箱结算：满血满甲弹药全满 +2 大招点 + 播报；空投箱拾取后永不重生。
func TestAirdropClaimJackpot(t *testing.T) {
	r := newAirdropRoom(t)
	now := time.Now()
	human := &r.Players[0].PlayerState
	human.HP = 20
	human.Armor = 0
	human.Mags = [2]int{1, 1}
	human.Reserves = [2]int{1, 1}

	// 手动放置一只空投箱在玩家脚下。
	r.Pickups = append(r.Pickups, Pickup{Id: airdropIdBase, Kind: PickupAirdrop, Pos: Vec3{0, 0, 0}, Active: true, RespawnAt: now.Add(time.Hour)})
	r.airdropDeadlines = append(r.airdropDeadlines, airdropDeadline{pickupId: airdropIdBase, expiresAt: now.Add(time.Hour)})

	before := len(r.pending)
	r.StepPickups(now)
	jackpot := false
	for _, e := range r.pending[before:] {
		if e.Type == EvChat && strings.Contains(e.Message, "抢到了传奇补给") {
			jackpot = true
		}
	}
	if !jackpot {
		t.Fatal("抢到空投应有播报")
	}
	if human.HP != MaxHP || human.Armor != 100 {
		t.Fatalf("补给后 HP=%d Armor=%d, 期望 %d/100", human.HP, human.Armor, MaxHP)
	}
	if human.Mags[0] != Weapons[3].Mag || human.Reserves[0] != Weapons[3].Reserve {
		t.Fatal("补给后弹药应全满")
	}
	if human.UltimatePoints != 2 {
		t.Fatalf("大招点 = %d, 期望 2", human.UltimatePoints)
	}
	// 空投箱拾取后永久休眠（不进入常规重生轮换）。
	for i := range r.Pickups {
		if r.Pickups[i].Id == airdropIdBase && r.Pickups[i].Active {
			t.Fatal("空投箱拾取后不应复活")
		}
	}
}
