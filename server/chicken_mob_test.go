package main

import (
	"strings"
	"testing"
	"time"
)

func newMobRoom() *Room {
	r := NewRoom(1, &World{Size: [2]float64{64, 64}, Spawns: [][3]float64{{0, 0, 0}, {10, 0, 10}}}, nil)
	r.initChickens()
	human := newTestHuman(10, r)
	human.Name = "偶像哥"
	human.Pos = Vec3{20, 0, 20}
	human.Alive = true
	r.Players = append(r.Players, human)
	return r
}

func TestChickenMobChasesIdol(t *testing.T) {
	r := newMobRoom()
	now := time.Now()
	r.pending = nil // 丢弃 initChickens 的出生事件，只看本次调用增量
	// 排期阶段不动。
	r.stepChickenMob(now)
	if len(r.pending) != 0 {
		t.Fatal("排期阶段不应有事件")
	}
	// 到点开场：偶像选定 + 播报。
	r.nextChickenMobAt = now
	before := len(r.pending)
	r.stepChickenMob(now)
	if r.chickenMobTarget != 10 {
		t.Fatalf("偶像应为唯一真人 10, 实际 %d", r.chickenMobTarget)
	}
	announced := false
	for _, e := range r.pending[before:] {
		if e.Type == EvChat && strings.Contains(e.Message, "偶像哥") {
			announced = true
		}
	}
	if !announced {
		t.Fatal("开场应有含偶像名的播报")
	}
	// 围观中：小鸡把家安到偶像脚下并朝他走。
	r.stepChickenMob(now.Add(time.Second))
	c := &r.Chickens[0]
	if c.Home.X != 20 || c.Home.Z != 20 {
		t.Fatalf("围观中小鸡的家应迁到偶像脚下, 实际 %v", c.Home)
	}
	if c.Dir.X == 0 && c.Dir.Z == 0 {
		t.Fatal("远距离围观中小鸡应朝偶像方向移动")
	}
	// 偶像阵亡 → 就地解散：定居 + 清态 + 播报。
	human := r.Players[0]
	human.Alive = false
	before = len(r.pending)
	r.stepChickenMob(now.Add(2 * time.Second))
	if !r.chickenMobUntil.IsZero() || r.chickenMobTarget != 0 {
		t.Fatal("偶像阵亡后围观状态应清空")
	}
	if c.Home != c.Pos {
		t.Fatalf("解散后小鸡应就地定居, Home=%v Pos=%v", c.Home, c.Pos)
	}
	dissolved := false
	for _, e := range r.pending[before:] {
		if e.Type == EvChat && strings.Contains(e.Message, "解散") {
			dissolved = true
		}
	}
	if !dissolved {
		t.Fatal("解散应有播报")
	}
}

func TestChickenMobSkipsWithoutHumans(t *testing.T) {
	r := newMobRoom()
	r.Players = nil // 纯 bot/无人局
	now := time.Now()
	r.nextChickenMobAt = now
	before := len(r.pending)
	r.stepChickenMob(now)
	if !r.chickenMobUntil.IsZero() {
		t.Fatal("无真人不应开场")
	}
	if len(r.pending) != before {
		t.Fatal("无真人不应播报")
	}
	if !r.nextChickenMobAt.After(now) {
		t.Fatal("无真人应改期重试")
	}
}
