package main

import (
	"strings"
	"testing"
	"time"
)

func newEmpireRoom() *Room {
	r := NewRoom(1, &World{Size: [2]float64{128, 128}, Spawns: [][3]float64{{0, 0, 0}, {20, 0, 20}}}, nil)
	r.initChickens()
	human := newTestHuman(10, r)
	human.Pos = Vec3{40, 0, 40}
	r.Players = append(r.Players, human)
	return r
}

// 拥立：到点后某只存活小鸡成为王（600 血），有播报，护卫就位。
func TestKingPromotionAndGuards(t *testing.T) {
	r := newEmpireRoom()
	now := time.Now()
	r.pending = nil // 丢弃 initChickens 的出生事件，只看本次调用增量
	r.stepChickenKing(now) // 排期
	if len(r.pending) != 0 {
		t.Fatal("排期阶段不应有事件")
	}

	r.nextKingAt = now
	before := len(r.pending)
	r.stepChickenKing(now)
	king := r.kingChicken()
	if king == nil {
		t.Fatal("到点应拥立金鸡王")
	}
	if king.KingHP != kingHP {
		t.Fatalf("王血量 = %d, 期望 %d", king.KingHP, kingHP)
	}
	announced := false
	for _, e := range r.pending[before:] {
		if e.Type == EvChat && strings.Contains(e.Message, "金鸡王") {
			announced = true
		}
	}
	if !announced {
		t.Fatal("拥立应有播报")
	}

	// 护卫：最近的 kingGuardCount 只小鸡把家安到王脚下并朝王走。
	r.stepChickenKing(now)
	guards := 0
	for i := range r.Chickens {
		c := &r.Chickens[i]
		if c == king || !c.Alive {
			continue
		}
		if c.Home == king.Pos {
			guards++
			if c.Dir.X == 0 && c.Dir.Z == 0 {
				t.Fatalf("护卫应朝王移动, 鸡 %d 方向为零", c.Id)
			}
		}
	}
	if guards != kingGuardCount {
		t.Fatalf("护卫数 = %d, 期望 %d", guards, kingGuardCount)
	}
}

// 王要多枪才能杀：中间命中发 EvHit 且不死；最后一枪驾崩掉超级补给。
func TestKingTakesMultipleShotsAndDropsLoot(t *testing.T) {
	r := newEmpireRoom()
	now := time.Now()
	r.nextKingAt = now
	r.stepChickenKing(now)
	king := r.kingChicken()
	king.Pos = Vec3{5, 0, 5}
	king.Home = king.Pos

	shooter := &r.Players[0].PlayerState // AK-47（33 伤害一发）
	shooter.HP = 30
	shooter.Armor = 0
	shotsToKill := 0
	dmgPerShot := uint8(Weapons[3].Dmg)

	for round := 1; ; round++ {
		before := len(r.pending)
		r.Chickens[0].Alive = true
		// 对王所在位置直接判定命中（绕过 raycast，命中逻辑自 chickenShot 起）。
		// 直接调用 chickenShot 的命中段：把王作为 best 的路径难以独立构造，
		// 这里用射线真实打：从王盒内穿过的射线。
		hit := r.chickenShot(shooter, Vec3{5, 0.3, 8}, Vec3{0, 0, -1}, 1e9, 1e9, 3, now)
		if !hit {
			t.Fatalf("第 %d 枪应命中王", round)
		}
		shotsToKill = round
		if !king.Alive {
			// 驾崩：应有 EvChickenDeath + 奖励播报
			death, reward := false, false
			for _, e := range r.pending[before:] {
				if e.Type == EvChickenDeath {
					death = true
				}
				if e.Type == EvChat && strings.Contains(e.Message, "超级补给") {
					reward = true
				}
			}
			if !death || !reward {
				t.Fatalf("王驾崩应有死亡事件+奖励播报, death=%v reward=%v", death, reward)
			}
			break
		}
		// 未死：应有 EvHit 反馈、无死亡事件、血量递减。
		hits, deaths := 0, 0
		for _, e := range r.pending[before:] {
			if e.Type == EvHit && e.Victim == king.Id {
				hits++
				if e.Dmg != dmgPerShot {
					t.Fatalf("EvHit 伤害 = %d, 期望 %d", e.Dmg, dmgPerShot)
				}
			}
			if e.Type == EvChickenDeath {
				deaths++
			}
		}
		if hits != 1 || deaths != 0 {
			t.Fatalf("第 %d 枪后: EvHit=%d EvDeath=%d, 期望 1/0", round, hits, deaths)
		}
		if round > 40 {
			t.Fatal("王血量异常，未能被击杀")
		}
	}
	want := (kingHP + uint16(dmgPerShot) - 1) / uint16(dmgPerShot)
	if uint16(shotsToKill) != want {
		t.Fatalf("击杀王需 %d 枪, 期望 %d", shotsToKill, want)
	}
	// 超级补给：满血满甲。
	if shooter.HP != MaxHP || shooter.Armor != 100 {
		t.Fatalf("补给后 HP=%d Armor=%d, 期望 %d/100", shooter.HP, shooter.Armor, MaxHP)
	}
	// 王位已清空，轮回排期已推进。
	if r.kingChicken() != nil {
		t.Fatal("王驾崩后不应再有在位王")
	}
	if !r.nextKingAt.After(now) {
		t.Fatal("驾崩后应推进下一位王的排期")
	}
}

// 王驾崩后按普通小鸡重生，且王位状态清零。
func TestKingRespawnsAsCommoner(t *testing.T) {
	r := newEmpireRoom()
	now := time.Now()
	r.nextKingAt = now
	r.stepChickenKing(now)
	king := r.kingChicken()
	king.KingHP = 1
	r.chickenShot(&r.Players[0].PlayerState, Vec3{king.Pos.X, 0.3, king.Pos.Z + 3}, Vec3{0, 0, -1}, 1e9, 1e9, 3, now)
	if king.King {
		t.Fatal("驾崩后 King 标志应清除")
	}
	// 到重生时间复活。
	later := now.Add(30 * time.Second)
	r.StepChickens(later)
	if !king.Alive || king.King || king.KingHP != 0 {
		t.Fatalf("复活后应为平民小鸡: Alive=%v King=%v HP=%d", king.Alive, king.King, king.KingHP)
	}
}
