package main

import (
	"math"
	"testing"
	"time"
)

// newWormholeTestRoom 构造带地板的空旷世界：地板只是让 Step 里的 Move
// 有落点可站（空地图会让玩家无限下坠），出生点判定不受影响
// （玩家盒 y∈[feet, feet+2.1] 与地板 y∈[-1,0] 不相交）。
func newWormholeTestRoom(spawns [][3]float64) *Room {
	w := &World{Size: [2]float64{128, 128}, Spawns: spawns}
	w.aabbs = append(w.aabbs, AABB{Min: Vec3{-64, -1, -64}, Max: Vec3{64, 0, 64}})
	return NewRoom(1, w, nil)
}

func wormholeEvents(r *Room, evType uint8) []Event {
	var out []Event
	for _, e := range r.pending {
		if e.Type == evType {
			out = append(out, e)
		}
	}
	return out
}

func vecClose(a, b Vec3, tol float64) bool {
	dx, dy, dz := a.X-b.X, a.Y-b.Y, a.Z-b.Z
	return dx*dx+dy*dy+dz*dz <= tol*tol
}

func TestWormholePairPicksFarthestWalkableSpawns(t *testing.T) {
	w := &World{Size: [2]float64{128, 128}, Spawns: [][3]float64{
		{0, 0, 0}, {50, 0, 0}, {0, 0, 50}, {-50, 0, -50},
	}}
	a, b, ok := wormholePair(w)
	if !ok {
		t.Fatalf("应当选出一对有效锚点")
	}
	// 最大间距对：{50,0,0}↔{-50,0,-50}（d=12500，唯一最大值）
	if a.X != 50 || a.Z != 0 || b.X != -50 || b.Z != -50 {
		t.Fatalf("应选中相距最远的一对，got a=%v b=%v", a, b)
	}
	a2, b2, ok2 := wormholePair(w)
	if !ok2 || a2 != a || b2 != b {
		t.Fatalf("选址必须逐位确定，got a=%v b=%v vs a2=%v b2=%v", a, b, a2, b2)
	}

	// 被墙体堵死的出生点不参与选址
	w.aabbs = append(w.aabbs, AABB{Min: Vec3{-50.4, 0, -50.4}, Max: Vec3{-49.6, 2.2, -49.6}})
	a3, b3, ok3 := wormholePair(w)
	if !ok3 || a3 != (Vec3{50, 0, 0}) || b3 != (Vec3{0, 0, 50}) {
		t.Fatalf("堵死 {-50,-50} 后应改选 {50,0}↔{0,50} 对，got a=%v b=%v", a3, b3)
	}
}

func TestWormholeDisabledWithoutDistinctSpawns(t *testing.T) {
	// 两个出生点水平重合 → 无有效间距对，功能静默关闭
	r := newWormholeTestRoom([][3]float64{{10, 0, 10}, {10, 0, 10}})
	if r.wormholeOK {
		t.Fatalf("重合出生点不应启用虫洞")
	}
	// 空世界不 panic
	r2 := NewRoom(2, &World{Size: [2]float64{64, 64}}, nil)
	if r2.wormholeOK {
		t.Fatalf("无出生点不应启用虫洞")
	}
}

func TestWormholeTeleportCooldownAndAnnounce(t *testing.T) {
	r := newWormholeTestRoom([][3]float64{{0, 0, 0}, {50, 0, 0}})
	if !r.wormholeOK {
		t.Fatalf("虫洞应当启用")
	}
	human := newTestHuman(10, r)
	human.Pos = Vec3{0.5, 0, 0.3} // A 门圈内
	r.Players = append(r.Players, human)
	r.pending = nil // 丢弃 NewRoom 的小鸡出生事件，只看本测试产生的事件

	base := time.Now()
	r.Step(base)

	// 全房播报恰好一次
	if chats := wormholeEvents(r, EvChat); len(chats) != 1 {
		t.Fatalf("首名玩家到场应播报一次玩法，got %d", len(chats))
	}

	// 传送到 B 门出口：沿 A→B 方向穿出门体 2.6m
	p := &human.PlayerState
	if p.Pos.X < 52.59 || p.Pos.X > 52.61 || p.Pos.Z != 0 || p.Pos.Y != 0 {
		t.Fatalf("应传送到 B 门出口 (52.6,0,0)，got %v", p.Pos)
	}
	blasts := wormholeEvents(r, EvExplosion)
	if len(blasts) != 2 || !vecClose(blasts[0].Origin, Vec3{0.5, 0, 0.3}, 0.01) || !vecClose(blasts[1].Origin, Vec3{52.6, 0, 0}, 0.001) {
		t.Fatalf("应入口/出口各一发传送特效，got %v", blasts)
	}

	// 冷却期内站回 B 门圈不再触发
	r.pending = nil
	p.Pos = Vec3{52.5, 0, 0.2}
	r.Step(base.Add(time.Second))
	if p.Pos.X != 52.5 || p.Pos.Z != 0.2 {
		t.Fatalf("冷却期内不应再次传送，got %v", p.Pos)
	}

	// 冷却过后从 B 门圈传回 A 门出口
	r.pending = nil
	p.Pos = Vec3{50.4, 0, 0.2}
	r.Step(base.Add(4 * time.Second))
	if p.Pos.X > -2.59 || p.Pos.X < -2.61 || p.Pos.Z != 0 {
		t.Fatalf("冷却后应传回 A 门出口 (-2.6,0,0)，got %v", p.Pos)
	}
	// 全程播报仍只有一次
	if chats := wormholeEvents(r, EvChat); len(chats) != 0 {
		t.Fatalf("玩法播报只应发生一次")
	}
}

func TestWormholeSkipsProtectedAndAirborne(t *testing.T) {
	r := newWormholeTestRoom([][3]float64{{0, 0, 0}, {50, 0, 0}})
	human := newTestHuman(10, r)
	human.Pos = Vec3{0.2, 0, 0.2}
	r.Players = append(r.Players, human)
	r.pending = nil
	base := time.Now()

	// 出生保护期内不传送（出生点常与门体重合，保护期就是下车通道）
	human.InvincibleUntil = base.Add(2 * time.Second)
	r.Step(base)
	if p := &human.PlayerState; p.Pos.X != 0.2 || p.Pos.Z != 0.2 {
		t.Fatalf("出生保护期内不应传送，got %v", p.Pos)
	}

	// 纵向窗口外（高处掠过）不传送
	human.InvincibleUntil = time.Time{}
	human.Pos = Vec3{0.2, 5, 0.2}
	human.OnGround = false
	r.Step(base)
	if p := &human.PlayerState; p.Pos.X != 0.2 || p.Pos.Z != 0.2 || p.Pos.Y <= wormholeHeight {
		t.Fatalf("纵向窗口外不应传送，got %v", p.Pos)
	}
	if len(wormholeEvents(r, EvExplosion)) != 0 {
		t.Fatalf("未触发传送不应有特效事件")
	}
}

func TestWormholeExitFallsBackWhenBlocked(t *testing.T) {
	r := newWormholeTestRoom([][3]float64{{0, 0, 0}, {50, 0, 0}})
	// 堵死 B 门出口落点 (52.6,0,0)
	r.World.aabbs = append(r.World.aabbs, AABB{Min: Vec3{52.2, 0, -0.6}, Max: Vec3{53.2, 2.2, 0.6}})
	human := newTestHuman(10, r)
	human.Pos = Vec3{0.3, 0, 0.2}
	r.Players = append(r.Players, human)
	r.pending = nil

	r.Step(time.Now())
	if p := &human.PlayerState; p.Pos != (Vec3{50, 0, 0}) {
		t.Fatalf("出口被堵应退回 B 门锚点 (50,0,0)，got %v", p.Pos)
	}
}

// TestWormholeRealMapPair 用仓库真实地图做交叉校验：选址必须成功、
// 两门必须拉开足够距离、锚点必须可站立。客户端 main.ts 的 wormholeAnchors
// 与本算法逐位同构，双方只依赖同一份 map.json 的出生点数组。
func TestWormholeRealMapPair(t *testing.T) {
	w, err := LoadWorld("../map.json")
	if err != nil {
		t.Skipf("仓库地图不可用，跳过：%v", err)
	}
	a, b, ok := wormholePair(w)
	if !ok {
		t.Fatalf("真实地图应能选出一对虫洞锚点")
	}
	dx, dz := a.X-b.X, a.Z-b.Z
	if d := math.Hypot(dx, dz); d < 60 {
		t.Fatalf("两门间距 %.1fm，应至少 60m 才有传送的戏剧性", d)
	}
	for _, p := range []Vec3{a, b} {
		if !w.CanOccupy(p, StandingHeight) {
			t.Fatalf("锚点 %v 必须可站立", p)
		}
	}
	t.Logf("虫洞锚点：A(%.1f, %.1f, %.1f) ↔ B(%.1f, %.1f, %.1f)，间距 %.1fm",
		a.X, a.Y, a.Z, b.X, b.Y, b.Z, math.Hypot(dx, dz))
}
