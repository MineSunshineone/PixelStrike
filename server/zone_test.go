package main

import (
	"strings"
	"testing"
)

func newZoneRoom(t *testing.T) *Room {
	store, err := NewStore(t.TempDir() + "/stats.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	// 中央据点 B(0,160) 在本测试世界无墙体；A/C 同理（空世界无阻挡）。
	r := NewRoom(1, &World{Size: [2]float64{512, 512}}, store)
	human := newTestHuman(10, r)
	human.Pos = Vec3{X: zoneCenters[0].X, Y: 0, Z: zoneCenters[0].Z}
	human.OnGround = true
	r.Players = append(r.Players, human)
	return r
}

func TestZoneControlAccruesAndRewards(t *testing.T) {
	r := newZoneRoom(t)
	human := &r.Players[0].PlayerState
	human.HP = 50
	r.pending = nil

	// 占领播报：第一 tick 即宣布首占。
	r.stepZones()
	firstCap := false
	for _, e := range r.pending {
		if e.Type == EvChat && strings.Contains(e.Message, "A 点已被") {
			firstCap = true
		}
	}
	if !firstCap {
		t.Fatal("首占应有播报")
	}

	// 巩固 10 秒：奖励 + 巩固播报。
	r.pending = nil
	for range zoneHoldTicks {
		r.stepZones()
	}
	rewarded := false
	for _, e := range r.pending {
		if e.Type == EvChat && strings.Contains(e.Message, "巩固了 A 点") {
			rewarded = true
		}
	}
	if !rewarded {
		t.Fatal("巩固满 10 秒应有奖励播报")
	}
	if human.UltimatePoints != zoneRewardUlt {
		t.Fatalf("大招点 = %d, 期望 %d", human.UltimatePoints, zoneRewardUlt)
	}
	if human.HP != 50+zoneRewardHP {
		t.Fatalf("奖励回血 = %d, 期望 %d", human.HP, 50+zoneRewardHP)
	}
}

func TestZoneContestFreezesProgress(t *testing.T) {
	r := newZoneRoom(t)
	// 第二名玩家进点：争夺冻结。
	b := newTestHuman(11, r)
	b.Pos = Vec3{X: zoneCenters[0].X + 1, Y: 0, Z: zoneCenters[0].Z}
	b.OnGround = true
	r.Players = append(r.Players, b)

	for range zoneHoldTicks + 10 {
		r.stepZones()
	}
	if r.zoneOwners[0] != 0 && r.zoneHoldTicks[0] != 0 {
		// 争夺期间不应产生新的巩固进度
	}
	// 首占可能已在单人 tick 完成（b 加入前），但加人后进度必须冻结：
	heldBefore := r.zoneHoldTicks[0]
	for range 50 {
		r.stepZones()
	}
	if r.zoneHoldTicks[0] != heldBefore {
		t.Fatalf("争夺中进度应冻结: %d → %d", heldBefore, r.zoneHoldTicks[0])
	}
}

func TestZoneDeadZoneSkipped(t *testing.T) {
	// 中央被墙体占用的据点自动休眠（测试：中心放一个方块的世界）。
	store, err := NewStore(t.TempDir() + "/stats.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	w := &World{Size: [2]float64{512, 512}, Spawns: [][3]float64{{0, 0, 0}}}
	w.aabbs = append(w.aabbs, AABB{Min: Vec3{X: zoneCenters[1].X - 3, Y: 0, Z: zoneCenters[1].Z - 3}, Max: Vec3{X: zoneCenters[1].X + 3, Y: 8, Z: zoneCenters[1].Z + 3}})
	r := NewRoom(1, w, store)
	human := newTestHuman(10, r)
	human.Pos = Vec3{X: zoneCenters[1].X + 5, Y: 0, Z: zoneCenters[1].Z}
	human.OnGround = true
	r.Players = append(r.Players, human)

	r.stepZones()
	if r.zoneOwners[1] != 0 {
		t.Fatal("被墙占用的据点应休眠（不可占领）")
	}
}
