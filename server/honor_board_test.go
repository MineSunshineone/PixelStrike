package main

import (
	"strings"
	"testing"
	"time"
)

func TestHonorBoardReportsKings(t *testing.T) {
	r := NewRoom(1, &World{Size: [2]float64{64, 64}}, nil)
	human := newTestHuman(10, r)
	ace := newTestBot(11, r)
	fodder := newTestBot(12, r)
	ace.Kills, fodder.Deaths = 12, 9
	r.Players = append(r.Players, human, ace, fodder)

	now := time.Now()
	// 首次调用只排期。
	r.reportHonor(now)
	if len(r.pending) != 0 {
		t.Fatal("排期阶段不应有播报")
	}
	// 到点：一期完整荣誉榜。
	r.nextHonorReportAt = now
	before := len(r.pending)
	r.reportHonor(now)
	if len(r.pending) != 1 {
		t.Fatalf("应有一条荣誉榜, 实际 %d", len(r.pending))
	}
	e := r.pending[before]
	if e.Type != EvChat || e.Name != "战场播报" {
		t.Fatalf("荣誉榜应为战场播报 EvChat, 实际 type=%d name=%q", e.Type, e.Name)
	}
	if !strings.Contains(e.Message, "击杀王") || !strings.Contains(e.Message, "12") {
		t.Fatalf("荣誉榜应含击杀王与杀数, 实际 %q", e.Message)
	}
	if !strings.Contains(e.Message, "打工皇帝") || !strings.Contains(e.Message, "9") {
		t.Fatalf("荣誉榜应含打工皇帝与阵亡数, 实际 %q", e.Message)
	}
	if !r.nextHonorReportAt.After(now) {
		t.Fatal("播报后应推进下一期排期")
	}
}

func TestHonorBoardSkipsEmptyAndNoShows(t *testing.T) {
	r := NewRoom(1, &World{Size: [2]float64{64, 64}}, nil)
	now := time.Now()
	r.nextHonorReportAt = now
	r.reportHonor(now) // 空房间：直接返回不 panic
	if len(r.pending) != 0 {
		t.Fatal("空房间不应播报")
	}

	// 全员零杀零阵亡：凑不出榜单，不播。
	b := newTestBot(11, r)
	human := newTestHuman(10, r)
	r.Players = append(r.Players, human, b)
	r.reportHonor(now)
	if len(r.pending) != 0 {
		t.Fatal("无数据不应播报")
	}

	// 纯 bot 局：有数据也不播。
	ace := newTestBot(12, r)
	ace.Kills = 5
	r.Players = append(r.Players, ace)
	r.reportHonor(now)
	if len(r.pending) != 0 {
		t.Fatal("无真人不应播报")
	}
}
