package main

import (
	"fmt"
	"time"
)

// 《赏金猎人》DLC：连杀 5 人的玩家自动挂上悬赏，击杀者领取奖励。
// 纯服务端经济系统：EvChat 播报 + 既有回血/大招点结算，零协议改动。

const (
	bountyStreak       = 5   // 连杀多少场挂悬赏
	bountyAmount       = 100 // 悬赏金额（战报口径）
	bountyRewardHP     = 50  // 领赏回血
	bountyRewardUltPts = 2   // 领赏大招点
)

// bountySet：连杀达到阈值时给攻击者挂上悬赏（在 Damage 击杀流程中调用）。
func (r *Room) bountySet(p *PlayerState) {
	if p.Streak != bountyStreak {
		return
	}
	if r.bountyOf == nil {
		r.bountyOf = make(map[uint16]uint16)
	}
	if r.bountyOf[p.Id] > 0 {
		return
	}
	r.bountyOf[p.Id] = bountyAmount
	if r.hasBountyAudience() {
		r.Emit(Event{Type: EvChat, Player: 0, Name: "战场播报",
			Message: fmt.Sprintf("💰 %s 连杀 %d 人，人头价值 %d 悬赏——拿他！", p.Name, p.Streak, bountyAmount)})
	}
}

// bountyClaim：悬赏目标被击杀时结算奖励（在 Damage 击杀流程中调用）。
// 奖励 = 回血 + 大招点（大招点仅真人；bot 也能领回血与播报 credit）。
func (r *Room) bountyClaim(attacker, victim *PlayerState, now time.Time) {
	amount := r.bountyOf[victim.Id]
	if amount == 0 || attacker.Id == victim.Id {
		return
	}
	delete(r.bountyOf, victim.Id)
	attacker.HP = uint8(min(attacker.maxHP(), int(attacker.HP)+bountyRewardHP))
	if !attacker.IsBot && attacker.Ultimate == 0 {
		attacker.UltimatePoints = uint8(min(UltimateRequirement, int(attacker.UltimatePoints)+bountyRewardUltPts))
	}
	if r.hasBountyAudience() {
		r.Emit(Event{Type: EvChat, Player: 0, Name: "战场播报",
			Message: fmt.Sprintf("💰 %s 收割了 %s 的 %d 悬赏！奖励：+%d HP +2 大招点", attacker.Name, victim.Name, amount, bountyRewardHP)})
	}
}

func (r *Room) hasBountyAudience() bool {
	for _, pl := range r.Players {
		if !pl.IsBot {
			return true
		}
	}
	return false
}
