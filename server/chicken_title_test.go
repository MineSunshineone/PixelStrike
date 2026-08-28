package main

import (
	"strings"
	"testing"
	"time"
)

func newChickenTitleRoom() *Room {
	r := NewRoom(1, &World{Size: [2]float64{64, 64}, Spawns: [][3]float64{{0, 0, 0}}}, nil)
	r.initChickens()
	human := newTestHuman(10, r)
	r.Players = append(r.Players, human)
	shooter := newTestBot(11, r)
	shooter.Pos = Vec3{5, 0, 5}
	r.Players = append(r.Players, shooter)
	return r
}

// 炸鸡里程碑：10/25/50 各播一次称号，其余次数静默。
func TestChickenKillTitlesAtMilestones(t *testing.T) {
	r := newChickenTitleRoom()
	shooter := r.Players[1]
	r.Chickens[0].Alive = true
	r.Chickens[0].Pos = Vec3{5, 0, 5}
	r.Chickens[0].Home = Vec3{5, 0, 5}

	milestones := map[int]string{10: "鸡圈噩梦", 25: "炸鸡大王", 50: "禽类保护协会"}
	titles := 0
	for i := 1; i <= 50; i++ {
		r.Chickens[0].Alive = true // 每次复活同一只鸡供射杀
		before := len(r.pending)
		// 从鸡盒（高度 0~0.62m）内水平穿过的射线，wallDist/playerDist 放行为最近命中。
		hit := r.chickenShot(&shooter.PlayerState, Vec3{5, 0.3, 8}, Vec3{0, 0, -1}, 1e9, 1e9, 3, time.Now())
		if !hit {
			t.Fatalf("第 %d 次应命中小鸡", i)
		}
		if shooter.PlayerState.ChickenKills != uint8(i) {
			t.Fatalf("炸鸡计数 = %d, 期望 %d", shooter.PlayerState.ChickenKills, i)
		}
		chat := 0
		want, isMilestone := milestones[i]
		for _, e := range r.pending[before:] {
			if e.Type != EvChat {
				continue
			}
			if !isMilestone {
				t.Fatalf("第 %d 次炸鸡不应有称号播报, 实际 %q", i, e.Message)
			}
			if !strings.Contains(e.Message, want) || !strings.Contains(e.Message, shooter.Name) {
				t.Fatalf("第 %d 次炸鸡的称号播报异常: %q", i, e.Message)
			}
			chat++
		}
		if isMilestone {
			if chat != 1 {
				t.Fatalf("第 %d 次应恰好 1 条称号播报, 实际 %d", i, chat)
			}
			titles++
		}
	}
	if titles != 3 {
		t.Fatalf("50 次炸鸡应触发 3 个称号, 实际 %d", titles)
	}
}
