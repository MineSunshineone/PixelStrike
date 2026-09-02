package main

// 海克斯强化卡：每条命 3 选 1 的临时强化，死亡即失效、复活重新选择。
// 零快照改动：发卡走个人事件 EvHexOffer（仅本人可见），选卡确认走广播事件 EvHexPick；
// 速度/散布/射速等系数由客户端按自己的 Hex id 本地镜像（与服端同值，保证预测不回弹）。

import (
	"math/rand/v2"
	"time"
)

const (
	HexSprint  uint8 = iota + 1 // 疾行者：移速 +15%
	HexAssault                  // 强袭：伤害 +15%
	HexFrenzy                   // 狂热：射速 +25%
	HexSteady                   // 稳定：散布 -30%
	HexBulwark                  // 铁壁：受到伤害 -20%
	HexLeech                    // 利爪：造成伤害 15% 回复生命
	HexBloodOx                  // 血牛：血量上限 100→140，选卡立即 +40
	HexAmmoBag                  // 弹药扩容：两把武器备弹翻倍
	HexQuicken                  // 快手：换弹耗时 -30%

	HexCardCount      = 9
	HexOfferCount     = 3
	HexBloodOxBonusHP = 40
)

type HexDef struct {
	Id   uint8
	Name string
	Desc string
}

var HexCards = []HexDef{
	{HexSprint, "疾行者", "移速 +15%"},
	{HexAssault, "强袭", "伤害 +15%"},
	{HexFrenzy, "狂热", "射速 +25%"},
	{HexSteady, "稳定", "散布 -30%"},
	{HexBulwark, "铁壁", "受到伤害 -20%"},
	{HexLeech, "利爪", "造成伤害 15% 回复生命"},
	{HexBloodOx, "血牛", "血量上限 100→140，选卡立即 +40"},
	{HexAmmoBag, "弹药扩容", "两把武器备弹翻倍"},
	{HexQuicken, "快手", "换弹耗时 -30%"},
}

// maxHP 返回当前血量上限；「血牛」持有者为 140。所有回血/满血钳制都应使用它。
func (p *PlayerState) maxHP() int {
	if p.Hex == HexBloodOx {
		return MaxHP + HexBloodOxBonusHP
	}
	return MaxHP
}

func (p *PlayerState) hexSpeedMul() float64 {
	if p.Hex == HexSprint {
		return 1.15
	}
	return 1
}

func (p *PlayerState) hexDamageMul() float64 {
	if p.Hex == HexAssault {
		return 1.15
	}
	return 1
}

// hexFireRateDiv 射速除数：开火间隔 /= 1.25 即射速 +25%。
func (p *PlayerState) hexFireRateDiv() float64 {
	if p.Hex == HexFrenzy {
		return 1.25
	}
	return 1
}

func (p *PlayerState) hexSpreadMul() float64 {
	if p.Hex == HexSteady {
		return 0.70
	}
	return 1
}

func (p *PlayerState) hexDmgTakenMul() float64 {
	if p.Hex == HexBulwark {
		return 0.80
	}
	return 1
}

func (p *PlayerState) hexReloadMul() float64 {
	if p.Hex == HexQuicken {
		return 0.70
	}
	return 1
}

// offerHex 为真人玩家抽 3 张不重复卡并广播个人事件；bot 不参与海克斯。
// 在 Join 与每次死亡时调用，覆盖上一轮未选择的 offer。
func (r *Room) offerHex(p *PlayerState) {
	if p.IsBot {
		return
	}
	perm := make([]int, len(HexCards))
	for i := range perm {
		perm[i] = i
	}
	rand.Shuffle(len(perm), func(i, j int) { perm[i], perm[j] = perm[j], perm[i] })
	var offer [HexOfferCount]uint8
	for i := range offer {
		offer[i] = HexCards[perm[i]].Id
	}
	p.HexOffer = offer
	r.Emit(Event{Type: EvHexOffer, Player: p.Id, Cards: offer})
}

// HexPick 校验并生效玩家的选择；不要求存活（死亡倒计时期间即可选）。
// 卡片随本条命持续到下一次死亡，因此死亡结算后才选的卡会带入新的一命。
func (r *Room) HexPick(p *PlayerState, card uint8, now time.Time) bool {
	if !hexInOffer(p.HexOffer, card) {
		return false
	}
	p.Hex = card
	p.HexOffer = [HexOfferCount]uint8{}
	switch card {
	case HexBloodOx:
		p.HP = uint8(min(p.maxHP(), int(p.HP)+HexBloodOxBonusHP))
	case HexAmmoBag:
		p.Reserves[0] = Weapons[p.Primary].Reserve * 2
		p.Reserves[1] = Weapons[p.Secondary].Reserve * 2
	}
	r.Emit(Event{Type: EvHexPick, Player: p.Id, Kind: card, Name: p.Name})
	return true
}

func hexInOffer(offer [HexOfferCount]uint8, card uint8) bool {
	for _, c := range offer {
		if c == card {
			return true
		}
	}
	return false
}
