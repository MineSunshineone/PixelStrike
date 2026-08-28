package main

import (
	"strings"
	"testing"
	"time"
)

func TestBotTiltAfterDeathStreak(t *testing.T) {
	old := botTiltChance
	botTiltChance = 1
	defer func() { botTiltChance = old }()

	r := NewRoom(1, &World{Size: [2]float64{64, 64}}, nil)
	human := newTestHuman(10, r) // 围观群众（播报门控需要真人在线）
	victim := newTestBot(11, r)
	killer := newTestBot(12, r)
	victim.Pos = Vec3{0, 0, 0}
	killer.Pos = Vec3{3, 0, 0}
	r.Players = append(r.Players, human, victim, killer)
	r.botAIs[11] = &BotAI{}
	r.botAIs[12] = &BotAI{DeathStreak: 2} // 击杀者自己此前也连败 2 场

	now := time.Now()
	// 连败 6 场触发躺平：TiltUntil = now + RespawnDelayS + botTiltDuration。
	announced := false
	for i := 1; i <= 6; i++ {
		before := len(r.pending)
		r.botKilled(&victim.PlayerState, &killer.PlayerState, now)
		for _, e := range r.pending[before:] {
			if e.Type == EvChat && strings.Contains(e.Message, "躺平") {
				announced = true
			}
		}
	}
	ai := r.botAIs[11]
	if ai.DeathStreak != 6 {
		t.Fatalf("连败计数 = %d, 期望 6", ai.DeathStreak)
	}
	if !announced {
		t.Fatal("第 6 次连败应播报躺平")
	}
	if want := RespawnDelayS + botTiltDuration; ai.TiltUntil.Sub(now) != want {
		t.Fatalf("躺平窗口 = %v, 期望 %v（含 3s 重生延迟）", ai.TiltUntil.Sub(now), want)
	}
	// 击杀者（bot）连败计数被清零。
	if got := r.botAIs[12].DeathStreak; got != 0 {
		t.Fatalf("击杀者连败计数应清零, 实际 %d", got)
	}
}

func TestBotTiltCrouchesThenRecovers(t *testing.T) {
	r := NewRoom(1, &World{Size: [2]float64{64, 64}}, nil)
	b := newTestBot(11, r)
	b.Pos = Vec3{5, 0, 5}
	r.Players = append(r.Players, b)
	r.botAIs[11] = &BotAI{
		TargetPos:    Vec3{-20, 0, -20},
		NextGlanceAt: time.Now().Add(time.Hour),
		TiltUntil:    time.Now().Add(time.Second),
	}

	now := time.Now()
	r.StepBots(now)
	if b.CmdKeys != KeyCrouch {
		t.Fatalf("躺平期间应只蹲不动, CmdKeys = %d", b.CmdKeys)
	}

	// 躺平结束恢复常态移动。
	r.botAIs[11].TiltUntil = now.Add(-time.Second)
	r.StepBots(now)
	if b.CmdKeys&KeyForward == 0 {
		t.Fatalf("躺平结束后应恢复移动, CmdKeys = %d", b.CmdKeys)
	}
	if r.botAIs[11].Target == nil && b.CmdKeys&KeyCrouch != 0 {
		t.Fatal("躺平结束后不应继续保持蹲姿输入")
	}
}
