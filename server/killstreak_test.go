package main

import (
	"strings"
	"testing"
	"time"
)

// 在 Damage 之后统计本次调用新产生的 EvChat 事件数。
func newChatsAfter(r *Room, before int) []Event {
	out := make([]Event, 0, 2)
	for _, e := range r.pending[before:] {
		if e.Type == EvChat {
			out = append(out, e)
		}
	}
	return out
}

func TestKillstreakMemeAtMilestones(t *testing.T) {
	store, err := NewStore(t.TempDir() + "/stats.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	r := newTeamTestRoom()
	r.Store = store
	attacker := newTestHuman(10, r)
	victim := newTestHuman(11, r)
	attacker.Name = "神枪手"
	victim.Name = "倒霉蛋"
	attacker.Pos = Vec3{0, 0, 0}
	victim.Pos = Vec3{3, 0, 0}
	r.Players = append(r.Players, attacker, victim)

	milestones := map[uint8]bool{3: true, 5: true, 8: true, 10: true}
	now := time.Now()
	deathMemes := 0
	for streak := uint8(1); streak <= 10; streak++ {
		victim.HP = MaxHP
		victim.Alive = true
		before := len(r.pending)
		r.Damage(&attacker.PlayerState, &victim.PlayerState, MaxHP, false, 3, now)
		chats := newChatsAfter(r, before)
		killMemes := 0
		for _, c := range chats {
			if c.Name != "战场播报" {
				t.Fatalf("streak %d: 播报名 = %q, 期望 %q", streak, c.Name, "战场播报")
			}
			isDeathMeme := strings.HasPrefix(c.Message, "倒霉蛋") && strings.Contains(c.Message, "连死")
			isKillMeme := strings.Contains(c.Message, "神枪手") && strings.Contains(c.Message, "连杀") && !strings.Contains(c.Message, "悬赏")
			if !isDeathMeme && !isKillMeme {
				continue
			}
			if isDeathMeme {
				deathMemes++
			} else {
				killMemes++
			}
		}
		if milestones[streak] && killMemes != 1 {
			t.Fatalf("streak %d: 期望 1 条连杀播报, 实际 %d (%v)", streak, killMemes, chats)
		}
		if !milestones[streak] && killMemes != 0 {
			t.Fatalf("streak %d: 非里程碑不应有连杀播报", streak)
		}
	}
	if deathMemes != 1 {
		t.Fatalf("连死 5 次应恰好 1 条安慰播报, 实际 %d", deathMemes)
	}
}

func TestKillstreakMemeSilentWithoutHumans(t *testing.T) {
	r := newTeamTestRoom()
	attacker := newTestBot(10, r)
	victim := newTestBot(11, r)
	attacker.Pos = Vec3{0, 0, 0}
	victim.Pos = Vec3{3, 0, 0}
	r.Players = append(r.Players, attacker, victim)

	now := time.Now()
	for range 3 {
		victim.HP = MaxHP
		victim.Alive = true
		before := len(r.pending)
		r.Damage(&attacker.PlayerState, &victim.PlayerState, MaxHP, false, 3, now)
		if chats := newChatsAfter(r, before); len(chats) > 0 {
			t.Fatalf("纯 bot 局不应播报, 实际 %q", chats[0].Message)
		}
	}
	if r.hasHuman() {
		t.Fatal("房间只有 bot 时 hasHuman 应为 false")
	}
}
