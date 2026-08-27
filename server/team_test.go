package main

import (
	"math"
	"testing"
	"time"
)

func newTeamTestRoom() *Room {
	// 空地图：无墙体遮挡，射线与距离判定完全确定。
	return NewRoom(1, &World{Size: [2]float64{64, 64}}, nil)
}

func newTestHuman(id uint16, room *Room) *Player {
	p := &Player{Room: room, joined: true}
	p.PlayerState = PlayerState{Id: id, Name: "tester", Primary: 3, Secondary: 0, ActiveSlot: 1}
	p.ApplyLoadout(3, 0)
	p.HP = MaxHP
	p.Alive = true
	p.OnGround = true
	return p
}

func newTestBot(id uint16, room *Room) *Player {
	b := &Player{Room: room, joined: true}
	b.PlayerState = PlayerState{Id: id, Name: "[BOT] Test", IsBot: true, Primary: 3, Secondary: 0, ActiveSlot: 1}
	b.ApplyLoadout(3, 0)
	b.HP = MaxHP
	b.Alive = true
	b.OnGround = true
	return b
}

// aimAngles 计算从射手到目标低位躯干点（y+0.55，必非爆头且单发不致命）的 yaw/pitch；
// 全员蹲姿静止时散布为 0，命中判定完全确定。
func aimAngles(shooter *PlayerState, to Vec3) (float64, float64) {
	dx := to.X - shooter.Pos.X
	dz := to.Z - shooter.Pos.Z
	eye := EyeHeight
	if shooter.Crouch {
		eye = CrouchEyeH
	}
	dy := (to.Y + 0.55) - (shooter.Pos.Y + eye)
	return math.Atan2(-dx, -dz), math.Atan2(dy, math.Hypot(dx, dz))
}

// 靠近 bot 连续蹲三次（触发沿经 applyQueuedInput 进入）应结成非法小队。
func TestIllegalTeamFormsAfterThreeCrouchTaps(t *testing.T) {
	r := newTeamTestRoom()
	human := newTestHuman(10, r)
	bot := newTestBot(11, r)
	human.Pos = Vec3{0, 0, 0}
	bot.Pos = Vec3{2, 0, 0}
	human.CmdKeys = KeyForward // 首拍前处于未下蹲状态，保证是触发沿
	r.Players = append(r.Players, human, bot)

	base := time.Now()
	seq := uint16(0)
	for i := range 3 {
		if i > 0 {
			human.queueInput(seq, KeyForward, 0, 0, base.Add(time.Duration(i)*300*time.Millisecond))
			seq++
			human.applyQueuedInput()
		}
		human.queueInput(seq, KeyCrouch, 0, 0, base.Add(time.Duration(i)*300*time.Millisecond))
		seq++
		human.applyQueuedInput()
	}

	if human.IllegalMate != bot.Id || bot.IllegalMate != human.Id {
		t.Fatalf("expected illegal pair %d<->%d, got %d<->%d", human.Id, bot.Id, human.IllegalMate, bot.IllegalMate)
	}
	found := false
	for _, e := range r.pending {
		if e.Type == EvBondEvent && e.Kind == EvKindIllegalJoin && e.Player == human.Id && e.Victim == bot.Id {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected join broadcast, pending=%v", r.pending)
	}
}

// 不在 bot 附近蹲再多次也不会组队；超出时间窗口计数重置。
func TestIllegalTeamRequiresNearbyBotAndFreshWindow(t *testing.T) {
	r := newTeamTestRoom()
	human := newTestHuman(10, r)
	loneBot := newTestBot(11, r)
	nearBot := newTestBot(12, r)
	human.Pos = Vec3{0, 0, 0}
	loneBot.Pos = Vec3{30, 0, 30} // 超出 IllegalTeamRange
	nearBot.Pos = Vec3{-20, 0, -20}
	r.Players = append(r.Players, human, loneBot, nearBot)

	now := time.Now()
	for i := range 3 {
		r.NoteCrouchTap(&human.PlayerState, now.Add(time.Duration(i)*200*time.Millisecond))
	}
	if human.IllegalMate != 0 {
		t.Fatalf("should not team without nearby bot, got mate %d", human.IllegalMate)
	}

	// 前两次在有效期内，第三次已超窗（相对 firstAt），应重新计数且不组队。
	r.NoteCrouchTap(&human.PlayerState, now.Add(10*time.Second))
	r.NoteCrouchTap(&human.PlayerState, now.Add(13*time.Second))
	r.NoteCrouchTap(&human.PlayerState, now.Add(17*time.Second))
	if human.IllegalMate != 0 {
		t.Fatal("stale window taps must not form a team")
	}
	// 紧接着的三次（窗口内）应当与最近的 bot 组队成功。
	nearBot.Pos = Vec3{1, 0, 0}
	for i := range 3 {
		r.NoteCrouchTap(&human.PlayerState, now.Add(17200*time.Millisecond+time.Duration(i)*300*time.Millisecond))
	}
	if human.IllegalMate != nearBot.Id {
		t.Fatalf("expected pair with nearest bot %d, got %d", nearBot.Id, human.IllegalMate)
	}
}

// 组队后：队友间子弹直接穿过（互相免伤），对第三方伤害照常，bot 反击被忽略。
func TestIllegalTeamBlocksFriendlyDamage(t *testing.T) {
	r := newTeamTestRoom()
	human := newTestHuman(10, r)
	bot := newTestBot(11, r)
	stranger := newTestHuman(12, r)
	human.Pos = Vec3{0, 0, 0}
	bot.Pos = Vec3{0, 0, -4}
	stranger.Pos = Vec3{6, 0, 0}
	for _, p := range []*Player{human, bot, stranger} {
		p.Crouch = true // 站立不动散布归零，命中确定性
		r.history[p.Id] = &poseHistory{}
	}
	r.Players = append(r.Players, human, bot, stranger)
	r.FormIllegalTeam(&human.PlayerState, &bot.PlayerState)

	now := time.Now()
	seq := uint16(1)
	// 人类射击路人：正常掉血。
	yawS1, pitchS1 := aimAngles(&human.PlayerState, stranger.Pos)
	if !r.TryFire(&human.PlayerState, yawS1, pitchS1, 0, r.tick, seq, now) {
		t.Fatal("human fire rejected")
	}
	if stranger.HP >= MaxHP {
		t.Fatalf("human shot at stranger should connect, HP=%d", stranger.HP)
	}

	// Bot 射击自己的非法队友（人类）：子弹穿过、伤害为零。
	seq++
	yawB, pitchB := aimAngles(&bot.PlayerState, human.Pos)
	if !r.TryFire(&bot.PlayerState, yawB, pitchB, 0, r.tick, seq, now.Add(time.Second)) {
		t.Fatal("bot fire rejected")
	}
	if human.HP != MaxHP {
		t.Fatalf("teammate friendly fire leaked: human HP=%d", human.HP)
	}

	// 人类反身射击 bot 队友：同样应为零伤害（目标被排除）。
	seq++
	yawH, pitchH := aimAngles(&human.PlayerState, bot.Pos)
	r.TryFire(&human.PlayerState, yawH, pitchH, 0, r.tick, seq, now.Add(1500*time.Millisecond))
	if bot.HP != MaxHP {
		t.Fatalf("human damaged own illegal mate: bot HP=%d", bot.HP)
	}

	// 第三人打 bot 应正常掉血。
	seq++
	stranger.Streak = 0
	yawS2, pitchS2 := aimAngles(&stranger.PlayerState, bot.Pos)
	if !r.TryFire(&stranger.PlayerState, yawS2, pitchS2, 0, r.tick, seq, now.Add(2*time.Second)) {
		t.Fatal("stranger fire rejected")
	}
	if bot.HP >= MaxHP {
		t.Fatalf("stranger damage blocked incorrectly: bot HP=%d", bot.HP)
	}
}

// 任一成员死亡/解散时双向清除并广播破裂事件。
func TestIllegalTeamBreaksOnDemand(t *testing.T) {
	r := newTeamTestRoom()
	human := newTestHuman(10, r)
	bot := newTestBot(11, r)
	r.Players = append(r.Players, human, bot)
	r.FormIllegalTeam(&human.PlayerState, &bot.PlayerState)

	r.BreakIllegalTeam(&human.PlayerState)
	if human.IllegalMate != 0 || bot.IllegalMate != 0 {
		t.Fatalf("break failed: human->%d bot->%d", human.IllegalMate, bot.IllegalMate)
	}
	found := false
	for _, e := range r.pending {
		if e.Type == EvBondEvent && e.Kind == EvKindIllegalBreak {
			found = true
		}
	}
	if !found {
		t.Fatal("expected breakup broadcast")
	}
}

// 已有小队时无法再次与其他 bot 结盟。
func TestIllegalTeamIsExclusive(t *testing.T) {
	r := newTeamTestRoom()
	human := newTestHuman(10, r)
	botA := newTestBot(11, r)
	botB := newTestBot(12, r)
	r.Players = append(r.Players, human, botA, botB)
	r.FormIllegalTeam(&human.PlayerState, &botA.PlayerState)
	r.FormIllegalTeam(&human.PlayerState, &botB.PlayerState)
	if human.IllegalMate != botA.Id || botB.IllegalMate != 0 {
		t.Fatalf("exclusive pairing violated: human->%d botB->%d", human.IllegalMate, botB.IllegalMate)
	}
}

// Bot 的索敌循环必须跳过非法队友。
func TestBotTargetScanSkipsIllegalMate(t *testing.T) {
	r := newTeamTestRoom()
	human := newTestHuman(10, r)
	bot := newTestBot(11, r)
	human.Pos = Vec3{0, 0, 0}
	bot.Pos = Vec3{0, 0, -3}
	r.Players = append(r.Players, human, bot)
	r.botAIs[bot.Id] = &BotAI{NextWaypointAt: time.Now()}
	r.FormIllegalTeam(&bot.PlayerState, &human.PlayerState)

	now := time.Now()
	r.StepBots(now)
	if ai := r.botAIs[bot.Id]; ai.Target != nil {
		t.Fatalf("bot targeted its illegal mate %d", ai.Target.Id)
	}
}
