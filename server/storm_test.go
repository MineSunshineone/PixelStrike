package main

import (
	"strings"
	"testing"
	"time"
)

func newStormRoom(t *testing.T) *Room {
	store, err := NewStore(t.TempDir() + "/stats.db")
	if err != nil {
		t.Fatal(err)
	}
	r := NewRoom(1, &World{Size: [2]float64{96, 96}}, store)
	human := newTestHuman(10, r)
	human.Pos = Vec3{0, 0, 0}
	r.Players = append(r.Players, human)
	return r
}

func TestStormWarningAndLightningStrike(t *testing.T) {
	r := newStormRoom(t)
	now := time.Now()
	r.pending = nil

	r.stepStorm(now) // 排期
	if len(r.pending) != 0 || r.stormKind != StormNone {
		t.Fatal("排期阶段不应有事件或天灾")
	}

	// 到点：预警播报 + 进入预警期（不立即造成伤害）。
	r.stormNextAt = now
	before := len(r.pending)
	r.stepStorm(now)
	if r.stormKind == StormNone {
		t.Fatal("到点应降临一场天灾")
	}
	warned := false
	for _, e := range r.pending[before:] {
		if e.Type == EvChat && strings.Contains(e.Message, "天灾预警") {
			warned = true
		}
	}
	if !warned {
		t.Fatal("应有天灾预警播报")
	}
	hpBefore := r.Players[0].PlayerState.HP
	r.stepStorm(now.Add(time.Second))
	if r.Players[0].PlayerState.HP != hpBefore {
		t.Fatal("预警期内不应有伤害")
	}

	// 强制雷暴并校验落点伤害：玩家站在落点中心 → 吃满额雷击。
	forced := r.stormKind != StormThunder
	r.stormKind = StormThunder
	r.stormStrikesLeft = 1
	r.stormNextStrikeAt = now.Add(stormWarnDuration)
	strikeTime := now.Add(stormWarnDuration)
	before = len(r.pending)
	r.stepStorm(strikeTime)
	if forced && len(r.pending) == before {
		t.Fatal("雷击应产生事件")
	}
	pos := r.stormStrikePos
	r.Players[0].PlayerState.Pos = pos // 站到落点中心再结算? 落点已结算——改为校验事件
	blast := false
	for _, e := range r.pending[before:] {
		if e.Type == EvExplosion {
			blast = true
		}
	}
	if !blast {
		t.Fatal("雷击应发出 EvExplosion")
	}
}

// 落点伤害：玩家恰好站在 stormStrikePos 时吃满额伤害（可致死走自雷语义）。
func TestLightningDamagesPlayersInRadius(t *testing.T) {
	r := newStormRoom(t)
	now := time.Now()
	human := &r.Players[0].PlayerState
	human.HP = 30 // 满额 60 应致死

	r.stormKind = StormThunder
	r.stormEndsAt = now.Add(time.Minute)
	r.stormNextStrikeAt = now
	r.stormStrikesLeft = 5
	// 直接调用 strike：落点即玩家脚下。
	before := len(r.pending)
	r.stormStrikeAt(now, human.Pos, stormThunderR, stormThunderDmg, "⚡ 轰隆——！")
	if human.Alive {
		t.Fatal("站桩吃满额雷击应致死")
	}
	death := false
	for _, e := range r.pending[before:] {
		if e.Type == EvKill && e.Killer == 10 && e.Victim == 10 {
			death = true // 自雷语义：Killer=Victim
		}
	}
	if !death {
		t.Fatal("致死应有 EvKill（自雷）")
	}
	// 半径外玩家毫发无伤。
	bystander := newTestBot(11, r)
	bystander.Pos = Vec3{40, 0, 40}
	r.Players = append(r.Players, bystander)
	human.HP = MaxHP
	human.Alive = true
	before = len(r.pending)
	r.stormStrikeAt(now.Add(time.Second), human.Pos, stormThunderR, stormThunderDmg, "⚡ 轰隆——！")
	if bystander.PlayerState.HP != MaxHP {
		t.Fatal("半径外不应受伤")
	}
	_ = before
}

// 电磁风暴：预警期不 jam，持续期按概率哑火（100% 注入时必哑火且不耗弹）。
func TestEMPJamsFiringWithoutAmmoCost(t *testing.T) {
	old := empJamChance
	empJamChance = 1
	defer func() { empJamChance = old }()

	r := newStormRoom(t)
	now := time.Now()
	shooter := &r.Players[0].PlayerState
	shooter.Pos = Vec3{0, 0, 0}
	r.stormKind = StormEMP
	r.stormEndsAt = now.Add(stormWarnDuration + stormDuration)
	r.stormNextStrikeAt = now

	// 预警期（还剩 > stormDuration）不 jam。
	magBefore := shooter.Mags[0]
	fired := r.TryFire(shooter, 0, 0, 0, 0, 1, now)
	if !fired {
		t.Fatal("预警期不应 jam")
	}
	if shooter.Mags[0] != magBefore-1 {
		t.Fatal("预警期开火应正常耗弹")
	}

	// 持续期：必哑火且不耗弹。
	r.stormEndsAt = now.Add(stormDuration)
	magBefore = shooter.Mags[0]
	fired = r.TryFire(shooter, 0, 0, 0, 0, 2, now.Add(time.Second))
	if fired {
		t.Fatal("100% jam 概率下开火应失败")
	}
	if shooter.Mags[0] != magBefore {
		t.Fatal("哑火不应耗弹")
	}
}
