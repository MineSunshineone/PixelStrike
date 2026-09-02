package main

import (
	"math"
	"testing"
	"time"
)

// 月球跳：按服务端 Move 的离散积分模拟一次满额起跳，验证跳跃高度
// 与新解析解 ≈3.7m 吻合（旧参数重力 -22、起跳 8.4 只有 1.6m）。
// 注意：客户端预测常数（client/src/constants.ts 的 PHYS.gravity/jumpVel）
// 与服务端 Gravity/JumpVel 必须逐位一致，改动时两端同步。
func TestMoonJumpApex(t *testing.T) {
	r := NewRoom(1, &World{Size: [2]float64{64, 64}}, nil)
	p := &Player{Room: r, joined: true}
	p.PlayerState = PlayerState{Id: 10, Name: "moon", Primary: 3, Secondary: 0, ActiveSlot: 1}
	p.ApplyLoadout(3, 0)
	p.HP = MaxHP
	p.Alive = true
	p.OnGround = true
	p.Pos = Vec3{0, 0, 0}
	p.CmdKeys = KeyJump
	r.Players = append(r.Players, p)

	now := time.Now()
	apex := 0.0
	for range 240 { // 4 秒足够完成一次起跳-落地
		r.Move(&p.PlayerState, now)
		now = now.Add(time.Second / TickRate)
		if p.Pos.Y > apex {
			apex = p.Pos.Y
		}
	}
	// 解析解 v²/2g = 10.5²/30 ≈ 3.675；离散积分误差在容差内。
	want := JumpVel * JumpVel / (2 * math.Abs(Gravity))
	if apex < want-0.25 || apex > want+0.25 {
		t.Fatalf("月球跳顶点 = %.3f, 期望 %.3f ±0.25", apex, want)
	}
	if apex < 3.0 {
		t.Fatalf("月球跳顶点 %.3f 应显著高于旧参数的 1.6m", apex)
	}
	if p.OnGround && p.Pos.Y > 0.01 {
		t.Fatalf("跳跃结束后应回到地面, 实际 y = %.3f", p.Pos.Y)
	}
}
