package main

import "time"

// 非法组队事件种类（复用 EvBondEvent 通道，kind 从 4 起避免与羁绊冲突）。
const (
	EvKindIllegalJoin  uint8 = 4
	EvKindIllegalBreak uint8 = 5
	// 羁绊破裂：配对一方离开房间时通知留守方（Player=留守者，Victim=离开者）。
	EvKindBondBreak uint8 = 6
)

// NoteCrouchTap 记录真人玩家的一次下蹲触发沿（抬起后再次按下）。在存活 bot 的
// IllegalTeamRange 范围内、IllegalTeamWindow 窗口内累计 IllegalTeamPresses 次即结为
// 非法小队。调用方需持有房间锁。
func (r *Room) NoteCrouchTap(p *PlayerState, now time.Time) {
	if !p.Alive || p.Flying || p.IsBot || p.IllegalMate != 0 {
		return
	}
	bot := r.nearestLivingBot(p)
	if bot == nil {
		delete(r.teamAttempts, p.Id)
		return
	}
	a := r.teamAttempts[p.Id]
	if a == nil || now.Sub(a.firstAt) > IllegalTeamWindow {
		a = &teamAttempt{firstAt: now}
		r.teamAttempts[p.Id] = a
	}
	a.count++
	a.lastAt = now
	if a.count >= IllegalTeamPresses {
		delete(r.teamAttempts, p.Id)
		r.FormIllegalTeam(p, bot)
	}
}

func (r *Room) nearestLivingBot(p *PlayerState) *PlayerState {
	var best *PlayerState
	bestSq := IllegalTeamRange * IllegalTeamRange
	for _, other := range r.Players {
		o := &other.PlayerState
		if !o.IsBot || !o.Alive {
			continue
		}
		d := dist2(o.Pos, p.Pos)
		if d <= bestSq {
			bestSq = d
			best = o
		}
	}
	return best
}

// FormIllegalTeam 结成非法小队并向全房广播。双方此前已有队友时静默失败。
func (r *Room) FormIllegalTeam(human, bot *PlayerState) {
	if human.IsBot || !bot.IsBot || human.IllegalMate != 0 || bot.IllegalMate != 0 {
		return
	}
	human.IllegalMate = bot.Id
	bot.IllegalMate = human.Id
	r.Emit(Event{Type: EvBondEvent, Player: human.Id, Victim: bot.Id, Kind: EvKindIllegalJoin, Name: human.Name})
}

// BreakIllegalTeam 解散 p 所在的非法小队（死亡、离开或 bot 被裁撤），并广播破裂消息。
func (r *Room) BreakIllegalTeam(p *PlayerState) {
	mateId := p.IllegalMate
	if mateId == 0 {
		return
	}
	p.IllegalMate = 0
	delete(r.teamAttempts, p.Id)
	if mate := r.findPlayer(mateId); mate != nil && mate.IllegalMate == p.Id {
		mate.IllegalMate = 0
		delete(r.teamAttempts, mate.Id)
		r.Emit(Event{Type: EvBondEvent, Player: p.Id, Victim: mate.Id, Kind: EvKindIllegalBreak})
	}
}
