package main

import (
	"testing"
	"time"
)

func newTauntRoom() *Room {
	// 空地图：无墙体、无出生点，Raycast/CanOccupy 均为确定性空行为。
	return NewRoom(1, &World{Size: [2]float64{64, 64}}, nil)
}

// Bot 得手（无论击杀真人还是 bot）都应概率触发嘲讽：TauntUntil 置为未来、方向为 ±1。
func TestBotTauntTriggersOnKill(t *testing.T) {
	old := botTauntChance
	botTauntChance = 1
	defer func() { botTauntChance = old }()

	r := newTauntRoom()
	killer := newTestBot(10, r)
	r.Players = append(r.Players, killer)
	r.botAIs[10] = &BotAI{}

	now := time.Now()
	// 击杀真人也嘲讽（victim 非 bot 不影响触发）。
	r.botKilled(&PlayerState{Id: 77}, &killer.PlayerState, now)
	ai := r.botAIs[10]
	if !now.Before(ai.TauntUntil) {
		t.Fatal("击杀后应进入嘲讽状态")
	}
	if ai.TauntUntil.Sub(now) != botTauntDuration {
		t.Fatalf("嘲讽时长 = %v, 期望 %v", ai.TauntUntil.Sub(now), botTauntDuration)
	}
	if ai.TauntSpin != -1 && ai.TauntSpin != 1 {
		t.Fatalf("嘲讽方向 = %v, 期望 ±1", ai.TauntSpin)
	}

	// 自雷不嘲讽。
	ai.TauntUntil = time.Time{}
	r.botKilled(&killer.PlayerState, &killer.PlayerState, now)
	if !ai.TauntUntil.IsZero() {
		t.Fatal("自雷不应触发嘲讽")
	}

	// 概率为 0 时不触发。
	botTauntChance = 0
	ai.TauntUntil = time.Time{}
	r.botKilled(&PlayerState{Id: 77}, &killer.PlayerState, now)
	if !ai.TauntUntil.IsZero() {
		t.Fatal("概率 0 时不应触发嘲讽")
	}
}

// 嘲讽期间 bot 应原地转圈：CmdKeys 为 0、yaw 按方向恒速旋转；结束后恢复常态移动。
func TestBotTauntSpinsInPlace(t *testing.T) {
	r := newTauntRoom()
	killer := newTestBot(10, r)
	killer.Pos = Vec3{5, 0, 5}
	r.Players = append(r.Players, killer)
	r.botAIs[10] = &BotAI{TargetPos: Vec3{-20, 0, -20}, NextGlanceAt: time.Now().Add(time.Hour), TauntUntil: time.Now().Add(time.Second), TauntSpin: 1}
	killer.Yaw = 0

	now := time.Now()
	for range 10 {
		r.StepBots(now)
		now = now.Add(time.Second / TickRate)
	}
	if killer.CmdKeys != 0 {
		t.Fatalf("嘲讽期间移动键应为 0, 实际 %d", killer.CmdKeys)
	}
	want := 10 * botTauntSpinRate
	if killer.Yaw < want*0.99 || killer.Yaw > want*1.01 {
		t.Fatalf("10 tick 后 yaw = %f, 期望 ≈ %f", killer.Yaw, want)
	}

	// 嘲讽结束后恢复常态：朝目标点移动。
	r.botAIs[10].TauntUntil = now.Add(-time.Second)
	r.StepBots(now)
	if killer.CmdKeys&KeyForward == 0 {
		t.Fatalf("嘲讽结束后应恢复移动, CmdKeys = %d", killer.CmdKeys)
	}
}
