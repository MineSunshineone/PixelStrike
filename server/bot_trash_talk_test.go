package main

import (
	"strings"
	"testing"
	"time"
)

func TestBotTrashTalkFromAliveBot(t *testing.T) {
	r := NewRoom(1, &World{Size: [2]float64{64, 64}}, nil)
	human := newTestHuman(10, r)
	talker := newTestBot(11, r)
	corpse := newTestBot(12, r)
	corpse.Alive = false // 阵亡 bot 没有发言权
	r.Players = append(r.Players, human, talker, corpse)

	now := time.Now()
	// 首次调用只排期。
	r.stepBotChat(now)
	if len(r.pending) != 0 {
		t.Fatal("排期阶段不应有弹幕")
	}
	// 到点：播报者必须是存活 bot，且用其身份（Player=Id）发言。
	r.nextBotChatAt = now
	before := len(r.pending)
	r.stepBotChat(now)
	if len(r.pending) != 1 {
		t.Fatalf("应有一条弹幕, 实际 %d", len(r.pending))
	}
	e := r.pending[before]
	if e.Type != EvChat || e.Player != 11 || e.Name != talker.Name {
		t.Fatalf("弹幕应以存活 bot 身份发出, 实际 type=%d player=%d", e.Type, e.Player)
	}
	if strings.TrimSpace(e.Message) == "" {
		t.Fatal("弹幕内容不应为空")
	}
	// 间隔已推进。
	if !r.nextBotChatAt.After(now) {
		t.Fatal("播报后应推进下一次排期")
	}
}

func TestBotTrashTalkSilentWithoutHumans(t *testing.T) {
	r := NewRoom(1, &World{Size: [2]float64{64, 64}}, nil)
	b := newTestBot(11, r)
	r.Players = append(r.Players, b)
	r.nextBotChatAt = time.Now()
	before := len(r.pending)
	r.stepBotChat(time.Now())
	if len(r.pending) != before {
		t.Fatal("无真人观众 bot 应闭麦")
	}
	if r.nextBotChatAt.IsZero() || r.nextBotChatAt.Before(time.Now().Add(botChatMinGap-time.Second)) {
		t.Fatal("闭麦后仍应推进排期")
	}
}
