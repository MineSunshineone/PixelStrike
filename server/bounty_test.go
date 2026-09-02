package main

import (
	"strings"
	"testing"
	"time"
)

func newBountyRoom(t *testing.T) *Room {
	store, err := NewStore(t.TempDir() + "/stats.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	r := NewRoom(1, &World{Size: [2]float64{96, 96}}, store)
	human := newTestHuman(10, r)
	human.Pos = Vec3{0, 0, 0}
	target := newTestHuman(11, r)
	target.Pos = Vec3{4, 0, 0}
	r.Players = append(r.Players, human, target)
	return r
}

func killOnce(r *Room, attacker, victim *Player, now time.Time) {
	victim.HP = 1
	r.Damage(&attacker.PlayerState, &victim.PlayerState, 1, false, 3, now)
	victim.Alive = true // 复活以继续下一轮
	victim.HP = MaxHP
}

// 连杀 5 人挂悬赏：第 4 杀无、第 5 杀有（播报 + map 记录）。
func TestBountySetAtStreakFive(t *testing.T) {
	r := newBountyRoom(t)
	human := &r.Players[0].PlayerState
	now := time.Now()

	for i := 1; i <= 5; i++ {
		before := len(r.pending)
		killOnce(r, r.Players[0], r.Players[1], now)
		bounty := r.bountyOf[10]
		announced := false
		for _, e := range r.pending[before:] {
			if e.Type == EvChat && strings.Contains(e.Message, "悬赏") {
				announced = true
			}
		}
		if i < 5 {
			if bounty != 0 || announced {
				t.Fatalf("第 %d 杀不应挂悬赏", i)
			}
		} else if bounty != bountyAmount || !announced {
			t.Fatalf("第 5 杀应挂 100 悬赏并播报, bounty=%d announced=%v", bounty, announced)
		}
	}
	if human.Streak != 5 {
		t.Fatalf("连杀数 = %d, 期望 5", human.Streak)
	}
}

// 领赏：击杀有悬赏者 → 回血 + 大招点 + 播报 + 悬赏清除；自雷不领。
func TestBountyClaimRewards(t *testing.T) {
	r := newBountyRoom(t)
	now := time.Now()
	human := &r.Players[0].PlayerState
	target := &r.Players[1].PlayerState
	human.HP = 40
	r.bountyOf = map[uint16]uint16{11: bountyAmount}

	before := len(r.pending)
	r.Damage(human, target, MaxHP, false, 3, now) // 击杀有悬赏者
	claimed := false
	for _, e := range r.pending[before:] {
		if e.Type == EvChat && strings.Contains(e.Message, "收割了") {
			claimed = true
		}
	}
	if !claimed {
		t.Fatal("领赏应有播报")
	}
	if human.HP != 40+bountyRewardHP {
		t.Fatalf("领赏回血 = %d, 期望 %d", human.HP, 40+bountyRewardHP)
	}
	// 3 = 击杀本身的 1 点 + 悬赏奖励 2 点。
	if human.UltimatePoints != 1+bountyRewardUltPts {
		t.Fatalf("领赏大招点 = %d, 期望 %d", human.UltimatePoints, 1+bountyRewardUltPts)
	}
	if r.bountyOf[11] != 0 {
		t.Fatal("领赏后悬赏应清除")
	}

	// 自雷不领赏。
	r.bountyOf[10] = bountyAmount
	human.HP = 1
	r.Damage(human, human, 999, false, WeaponHE, now)
	if r.bountyOf[10] != bountyAmount {
		t.Fatal("自雷不应触发领赏清除")
	}
}

// 退出房间带走悬赏（防 map 泄漏与死人领赏）。
func TestBountyCleanupOnRemove(t *testing.T) {
	r := newBountyRoom(t)
	p := r.Players[0]
	r.bountyOf = map[uint16]uint16{10: bountyAmount}
	r.Remove(p)
	if _, ok := r.bountyOf[10]; ok {
		t.Fatal("退出房间应清除悬赏记录")
	}
}
