package main

import (
	"strings"
	"testing"
)

func mkKills(n int) []Event {
	evts := make([]Event, 0, n)
	for i := 0; i < n; i++ {
		evts = append(evts, Event{Type: EvKill, Killer: 10, Victim: uint16(20 + i)})
	}
	return evts
}

func TestDJBroadcastsEveryFifteenKills(t *testing.T) {
	r := NewRoom(1, &World{Size: [2]float64{64, 64}}, nil)
	human := newTestHuman(10, r)
	r.Players = append(r.Players, human)

	// 14 杀：不播。
	r.djOnEvents(mkKills(14))
	if len(r.pending) != 0 {
		t.Fatal("未满 15 杀不应播报")
	}
	// 第 15 杀：恰好 1 条 DJ 播报，计数归零。
	r.djOnEvents(mkKills(1))
	if len(r.pending) != 1 {
		t.Fatalf("第 15 杀应恰好 1 条播报, 实际 %d", len(r.pending))
	}
	e := r.pending[0]
	if e.Type != EvChat || e.Name != "战场DJ" {
		t.Fatalf("播报应为战场DJ EvChat, 实际 type=%d name=%q", e.Type, e.Name)
	}
	if !strings.Contains(e.Message, "🎵") {
		t.Fatalf("播报应来自电台文案, 实际 %q", e.Message)
	}
	r.pending = nil

	// 计数回绕后再来一轮。
	r.djOnEvents(mkKills(15))
	if len(r.pending) != 1 {
		t.Fatalf("回绕后第 15 杀应再次播报, 实际 %d", len(r.pending))
	}
}

func TestDJSkipsSelfKillsAndEmptyRooms(t *testing.T) {
	r := NewRoom(1, &World{Size: [2]float64{64, 64}}, nil)

	// 自雷不计入击杀数。
	selfKills := make([]Event, 0, 20)
	for i := 0; i < 20; i++ {
		selfKills = append(selfKills, Event{Type: EvKill, Killer: 10, Victim: 10})
	}
	r.djOnEvents(selfKills)
	if r.djKillCount != 0 {
		t.Fatalf("自雷不应累计 DJ 计数, 实际 %d", r.djKillCount)
	}

	// 无人在线：计数照常累计回绕但不播报。
	human := newTestHuman(10, r)
	human.IsBot = true // 伪造无真人场景
	r.Players = append(r.Players, human)
	r.djOnEvents(mkKills(15))
	if len(r.pending) != 0 {
		t.Fatal("无真人不应播报")
	}
	if r.djKillCount != 0 {
		t.Fatal("满 15 杀后计数应回绕")
	}
}
