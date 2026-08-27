package main

import (
	"math"
	"testing"
	"time"
)

// Shift 冲刺：仅在有移动输入、非蹲伏、非飞行时提速 1.3 倍。
func TestSprintSpeed(t *testing.T) {
	r := &Room{World: &World{Size: [2]float64{64, 64}}}
	p := &PlayerState{Id: 1, Alive: true, OnGround: true, Weapon: 3, ActiveSlot: 1}
	p.ApplyLoadout(3, 0)
	now := time.Now()

	// 普通前进：速度应收敛到 WalkSpeed * 武器速度系数
	p.CmdKeys = KeyForward
	for range 120 {
		r.Move(p, now)
	}
	base := Hyp(p.Vel.X, p.Vel.Z)
	wantBase := WalkSpeed * Weapons[3].SpeedMult
	if abs(base-wantBase) > 0.05 {
		t.Fatalf("walk speed = %.3f, want ~%.3f", base, wantBase)
	}

	// Shift 冲刺：提到 1.3 倍
	p.CmdKeys = KeyForward | KeyDescend
	for range 120 {
		r.Move(p, now)
	}
	sprint := Hyp(p.Vel.X, p.Vel.Z)
	wantSprint := wantBase * SprintMultiplier
	if abs(sprint-wantSprint) > 0.05 {
		t.Fatalf("sprint speed = %.3f, want ~%.3f", sprint, wantSprint)
	}

	// 蹲伏时 Shift 不应加速
	p.CmdKeys = KeyForward | KeyCrouch | KeyDescend
	for range 120 {
		r.Move(p, now)
	}
	crouched := Hyp(p.Vel.X, p.Vel.Z)
	wantCrouch := wantBase * CrouchSpeed
	if abs(crouched-wantCrouch) > 0.05 {
		t.Fatalf("crouch+shift speed = %.3f, want ~%.3f (冲刺不应作用于蹲伏)", crouched, wantCrouch)
	}

	// 静止时按住 Shift 不应产生速度
	p.CmdKeys = KeyDescend
	for range 120 {
		r.Move(p, now)
	}
	if idle := Hyp(p.Vel.X, p.Vel.Z); idle > 0.01 {
		t.Fatalf("idle sprint speed = %.3f, want ~0", idle)
	}
}

// 快照状态字节 bit128 必须标记 AI bot（客户端徽章的权威来源）。
func TestQuantizeStateMarksBot(t *testing.T) {
	human := &PlayerState{Id: 1, Alive: true}
	bot := &PlayerState{Id: 2, Alive: true, IsBot: true}
	if s := quantizeState(human, 0).state; s&128 != 0 {
		t.Fatalf("human state must not carry bot bit: %08b", s)
	}
	if s := quantizeState(bot, 0).state; s&128 == 0 {
		t.Fatalf("bot state must carry bit128: %08b", s)
	}
}

// 配对的羁绊队友退出房间时，留守方应收到 kind=6 的破裂事件。
func TestBondBreakEventOnRemove(t *testing.T) {
	r := newTeamTestRoom()
	a := newTestHuman(10, r)
	b := newTestHuman(11, r)
	r.Players = append(r.Players, a, b)
	a.BondMate = b.Id
	b.BondMate = a.Id

	r.Remove(b)
	if a.BondMate != 0 {
		t.Fatalf("mate bond should be cleared, got %d", a.BondMate)
	}
	found := false
	for _, e := range r.pending {
		if e.Type == EvBondEvent && e.Kind == EvKindBondBreak && e.Player == a.Id && e.Victim == b.Id {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected bond-break broadcast to remaining player, pending=%v", r.pending)
	}
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

func Hyp(x, z float64) float64 { return math.Hypot(x, z) }
