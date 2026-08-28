package main

import (
	"strings"
	"testing"
	"time"
)

func newChickenRainRoom(t *testing.T) *Room {
	t.Helper()
	r := NewRoom(1, &World{Size: [2]float64{64, 64}, Spawns: [][3]float64{{0, 0, 0}, {10, 0, 10}}}, nil)
	human := newTestHuman(10, r)
	human.Pos = Vec3{0, 0, 0}
	r.Players = append(r.Players, human)
	return r
}

func TestChickenRainAppendsChickensAndAnnounces(t *testing.T) {
	r := newChickenRainRoom(t)
	if len(r.Chickens) != chickenCount {
		t.Fatalf("初始鸡口 = %d, 期望 %d", len(r.Chickens), chickenCount)
	}
	now := time.Now()
	// 第一次 StepChickens 只排期不触发。
	r.StepChickens(now)
	if !r.nextChickenRainAt.After(now) {
		t.Fatal("首次 StepChickens 应排期第一场鸡雨")
	}
	if len(r.Chickens) != chickenCount {
		t.Fatalf("排期阶段不应增加鸡口, 实际 %d", len(r.Chickens))
	}

	// 到点触发：鸡口 +chickenRainBatch，新小鸡有生成事件，有真人在线应有播报。
	r.nextChickenRainAt = now
	before := len(r.pending)
	r.StepChickens(now)
	if len(r.Chickens) != chickenCount+chickenRainBatch {
		t.Fatalf("鸡雨后鸡口 = %d, 期望 %d", len(r.Chickens), chickenCount+chickenRainBatch)
	}
	newIds := map[uint16]bool{}
	for _, c := range r.Chickens[chickenCount:] {
		if c.Id != uint16(300+chickenCount+len(newIds)) {
			t.Fatalf("新小鸡 ID = %d, 期望顺序分配", c.Id)
		}
		newIds[c.Id] = true
	}
	spawned := map[uint16]bool{}
	announced := false
	for _, e := range r.pending[before:] {
		if e.Type == EvChickenSpawn && newIds[e.Player] {
			spawned[e.Player] = true
		}
		if e.Type == EvChat && strings.Contains(e.Message, "鸡雨") {
			announced = true
		}
	}
	if len(spawned) != chickenRainBatch {
		t.Fatalf("新增小鸡应有各自生成事件, 实际 %d/%d: %v", len(spawned), chickenRainBatch, newIds)
	}
	if !announced {
		t.Fatal("真人在线时鸡雨应有战场播报")
	}

	// 多轮鸡雨后触及上限，不再增长，但排期照常推进。
	r.nextChickenRainAt = now
	for len(r.Chickens) < chickenRainCap {
		r.StepChickens(now)
		r.nextChickenRainAt = now
	}
	if len(r.Chickens) != chickenRainCap {
		t.Fatalf("鸡口应停在 %d, 实际 %d", chickenRainCap, len(r.Chickens))
	}
	next := r.nextChickenRainAt
	r.chickenRain(now)
	if len(r.Chickens) != chickenRainCap {
		t.Fatalf("达上限后鸡雨不应继续增加鸡口, 实际 %d", len(r.Chickens))
	}
	if !r.nextChickenRainAt.After(next) {
		t.Fatal("达上限后仍应推进下一场鸡雨排期")
	}
}

func TestChickenRainRainDeadChickenStillRespawns(t *testing.T) {
	r := newChickenRainRoom(t)
	now := time.Now()
	r.nextChickenRainAt = now
	r.StepChickens(now)
	bonus := &r.Chickens[len(r.Chickens)-1]
	bonus.Alive = false
	bonus.RespawnAt = now
	r.StepChickens(now.Add(time.Second))
	if !bonus.Alive {
		t.Fatal("鸡雨小鸡被击杀后应与其他小鸡一样重生")
	}
}
