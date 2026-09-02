package main

import (
	"testing"
	"time"
)

func newHexRoom(t *testing.T) *Room {
	store, err := NewStore(t.TempDir() + "/stats.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	r := NewRoom(1, &World{Size: [2]float64{96, 96}}, store)
	// 测试世界补一块地面 AABB（y: -1~0），否则玩家悬空下坠走空中加速度。
	r.World.aabbs = append(r.World.aabbs, AABB{Min: Vec3{X: -48, Y: -1, Z: -48}, Max: Vec3{X: 48, Y: 0, Z: 48}})
	a := newTestHuman(10, r)
	a.Pos = Vec3{0, 0, 0}
	b := newTestHuman(11, r)
	b.Pos = Vec3{4, 0, 0}
	r.Players = append(r.Players, a, b)
	return r
}

func lastHexOffer(r *Room, playerId uint16) (Event, bool) {
	for i := len(r.pending) - 1; i >= 0; i-- {
		e := r.pending[i]
		if e.Type == EvHexOffer && e.Player == playerId {
			return e, true
		}
	}
	return Event{}, false
}

// 发卡：真人抽 3 张合法且不重复；bot 不参与。
func TestHexOfferDealtToHumansOnly(t *testing.T) {
	r := newHexRoom(t)
	a := &r.Players[0].PlayerState
	bot := newTestBot(20, r)
	r.Players = append(r.Players, bot)

	r.offerHex(a)
	offer, ok := lastHexOffer(r, a.Id)
	if !ok {
		t.Fatal("真人应有 EvHexOffer")
	}
	seen := map[uint8]bool{}
	for _, c := range offer.Cards {
		if c < 1 || c > HexCardCount {
			t.Fatalf("发卡 %d 超出卡池范围", c)
		}
		if seen[c] {
			t.Fatalf("发卡重复：%v", offer.Cards)
		}
		seen[c] = true
	}
	for i, c := range a.HexOffer {
		if c != offer.Cards[i] {
			t.Fatalf("HexOffer 与事件不一致：%v vs %v", a.HexOffer, offer.Cards)
		}
	}

	before := len(r.pending)
	r.offerHex(&bot.PlayerState)
	if bot.HexOffer != [HexOfferCount]uint8{} {
		t.Fatal("bot 不应被发卡")
	}
	for _, e := range r.pending[before:] {
		if e.Type == EvHexOffer {
			t.Fatal("bot 不应收到 EvHexOffer")
		}
	}
}

// 选卡：合法选中生效并广播；非法与重复选择被拒绝。
func TestHexPickValidation(t *testing.T) {
	r := newHexRoom(t)
	now := time.Now()
	a := &r.Players[0].PlayerState
	a.HexOffer = [HexOfferCount]uint8{HexSprint, HexBloodOx, HexQuicken}

	if !r.HexPick(a, HexBloodOx, now) {
		t.Fatal("选择 offer 内的卡应成功")
	}
	if a.Hex != HexBloodOx || a.HexOffer != [HexOfferCount]uint8{} {
		t.Fatal("选中后 Hex 应生效且 offer 清空")
	}
	found := false
	for _, e := range r.pending {
		if e.Type == EvHexPick && e.Player == a.Id && e.Kind == HexBloodOx {
			found = true
		}
	}
	if !found {
		t.Fatal("选中应广播 EvHexPick")
	}
	if r.HexPick(a, HexSprint, now) {
		t.Fatal("offer 清空后不应再能选卡")
	}
	a.HexOffer = [HexOfferCount]uint8{HexSteady, HexLeech, HexFrenzy}
	if r.HexPick(a, HexAmmoBag, now) {
		t.Fatal("选择 offer 之外的卡应被拒绝")
	}
}

// 死亡失效：旧卡清零并立刻发新一轮；复活不清 Hex（倒计时期间选的卡带入新命）。
func TestHexDeathClearsAndReoffers(t *testing.T) {
	r := newHexRoom(t)
	now := time.Now()
	a := &r.Players[0].PlayerState
	b := &r.Players[1].PlayerState
	a.Hex = HexSprint

	r.Damage(b, a, 500, false, 3, now)
	if a.Alive {
		t.Fatal(" victim 应死亡")
	}
	if a.Hex != 0 {
		t.Fatalf("死亡后 Hex 应清零, 实际 %d", a.Hex)
	}
	if _, ok := lastHexOffer(r, a.Id); !ok {
		t.Fatal("死亡应触发新一轮发卡")
	}
	// 倒计时期间选卡 → 复活后应保留。固定 offer 以便断言血牛。
	a.HexOffer = [HexOfferCount]uint8{HexBloodOx, HexSprint, HexSteady}
	if !r.HexPick(a, HexBloodOx, now.Add(time.Second)) {
		t.Fatal("死亡期间应可选卡")
	}
	r.Respawn(a, now.Add(4*time.Second))
	if a.Hex != HexBloodOx {
		t.Fatal("复活不应清掉死亡期间选中的卡")
	}
	if a.HP != uint8(MaxHP+HexBloodOxBonusHP) {
		t.Fatalf("血牛持有者复活应满 140 血, 实际 %d", a.HP)
	}
}

// 血牛：上限 140、选卡立即 +40 且按新上限钳制；死亡回到 100。
func TestHexBloodOxMaxHP(t *testing.T) {
	r := newHexRoom(t)
	now := time.Now()
	a := &r.Players[0].PlayerState
	a.HexOffer = [HexOfferCount]uint8{HexBloodOx, HexSprint, HexSteady}
	a.HP = 70

	if !r.HexPick(a, HexBloodOx, now) {
		t.Fatal("应可选中血牛")
	}
	if a.maxHP() != MaxHP+HexBloodOxBonusHP {
		t.Fatalf("血牛上限应为 %d, 实际 %d", MaxHP+HexBloodOxBonusHP, a.maxHP())
	}
	if a.HP != 110 {
		t.Fatalf("选卡应立即 +40（70→110）, 实际 %d", a.HP)
	}
	// 回血钳制到 140：医疗包 +50。
	if !applyPickup(a, PickupHealth, now) {
		t.Fatal("未满血应可拾取医疗包")
	}
	if a.HP != uint8(MaxHP+HexBloodOxBonusHP) {
		t.Fatalf("医疗包后应钳制在 140, 实际 %d", a.HP)
	}
	// 满血（按新上限）时医疗包应无效。
	if applyPickup(a, PickupHealth, now) {
		t.Fatal("血牛满血时医疗包应无效")
	}
	// 无卡时按 100 钳制。
	a.Hex = 0
	a.HP = 50
	if !applyPickup(a, PickupHealth, now) {
		t.Fatal("低血应可拾取医疗包")
	}
	if a.HP != MaxHP {
		t.Fatalf("无卡上限应回到 %d, 实际 %d", MaxHP, a.HP)
	}
}

// 弹药扩容：两把武器备弹翻倍。
func TestHexAmmoBagDoublesReserve(t *testing.T) {
	r := newHexRoom(t)
	now := time.Now()
	a := &r.Players[0].PlayerState
	base0, base1 := Weapons[a.Primary].Reserve, Weapons[a.Secondary].Reserve
	a.HexOffer = [HexOfferCount]uint8{HexAmmoBag, HexSprint, HexSteady}

	if !r.HexPick(a, HexAmmoBag, now) {
		t.Fatal("应可选中弹药扩容")
	}
	if a.Reserves[0] != base0*2 || a.Reserves[1] != base1*2 {
		t.Fatalf("备弹应翻倍: %v, 期望 [%d %d]", a.Reserves, base0*2, base1*2)
	}
}

// 疾行者：地面直线移动位移比 ≈1.15。
func TestHexSprintSpeedMultiplier(t *testing.T) {
	r := newHexRoom(t)
	now := time.Now()
	base := &r.Players[0].PlayerState
	hex := &r.Players[1].PlayerState
	hex.Hex = HexSprint

	run := func(p *PlayerState) float64 {
		p.CmdKeys = KeyForward
		for i := 0; i < 90; i++ {
			r.Move(p, now) // Move 的 now 只参与 buff 判断，位移由 tick 常数积分
		}
		return p.Pos.Z
	}
	dBase, dHex := run(base), run(hex)
	if dBase == 0 {
		t.Fatal("基线位移不应为 0")
	}
	ratio := -dHex / -dBase
	if ratio < 1.10 || ratio > 1.20 {
		t.Fatalf("疾行者位移比应在 1.10~1.20, 实际 %.3f", ratio)
	}
}

// 强袭/铁壁：伤害 ×1.15 与受伤 ×0.80。
func TestHexAssaultAndBulwarkDamage(t *testing.T) {
	r := newHexRoom(t)
	now := time.Now()
	a := &r.Players[0].PlayerState
	b := &r.Players[1].PlayerState

	r.Damage(a, b, 20, false, 3, now)
	if b.HP != MaxHP-20 {
		t.Fatalf("基线伤害 20 应扣 20 血, 实际 %d", MaxHP-b.HP)
	}

	b.HP = MaxHP
	a.Hex = HexAssault
	r.Damage(a, b, 20, false, 3, now)
	if got := MaxHP - b.HP; got != 23 {
		t.Fatalf("强袭应打出 23 伤害, 实际 %d", got)
	}

	b.HP = MaxHP
	a.Hex = 0
	b.Hex = HexBulwark
	r.Damage(a, b, 20, false, 3, now)
	if got := MaxHP - b.HP; got != 16 {
		t.Fatalf("铁壁应只受 16 伤害, 实际 %d", got)
	}
}

// 利爪：造成伤害 15% 回血（向下取整）。
func TestHexLeechLifesteal(t *testing.T) {
	r := newHexRoom(t)
	now := time.Now()
	a := &r.Players[0].PlayerState
	b := &r.Players[1].PlayerState
	a.Hex = HexLeech
	a.HP = 50

	r.Damage(a, b, 20, false, 3, now)
	if a.HP != 53 {
		t.Fatalf("利爪应回血 3（20×15%%）, 实际 %d", a.HP-50)
	}

	a.HP = 99
	r.Damage(a, b, 20, false, 3, now)
	if a.HP != MaxHP {
		t.Fatalf("回血应按 100 上限钳制（99+3）, 实际 %d", a.HP)
	}
}

// 狂热：开火间隔缩短到 1/1.25。
func TestHexFrenzyFireRate(t *testing.T) {
	r := newHexRoom(t)
	now := time.Now()
	a := &r.Players[0].PlayerState

	fireGap := func(p *PlayerState) time.Duration {
		p.NextFire = time.Time{}
		r.TryFire(p, 0, 0, 0, r.tick, 1, now)
		return p.NextFire.Sub(now)
	}
	base := fireGap(a)
	a.Hex = HexFrenzy
	a.LastShotSeq = 0
	a.HasShot = false
	got := fireGap(a)
	if got >= base || float64(got) > float64(base)/1.2 {
		t.Fatalf("狂热间隔应明显短于基线: base=%v got=%v", base, got)
	}
}

// 快手：换弹耗时 ×0.70，EvReloadStart 的 ms 同步缩放。
func TestHexQuickenReload(t *testing.T) {
	r := newHexRoom(t)
	now := time.Now()
	a := &r.Players[0].PlayerState
	a.Mags[0] = 10 // AK 未满弹匣才会启动换弹

	baseReloadMs := Weapons[a.Primary].ReloadMs
	if !r.StartReload(a, now) {
		t.Fatal("应能开始换弹")
	}
	if got := a.ReloadEnd.Sub(now); got != time.Duration(baseReloadMs)*time.Millisecond {
		t.Fatalf("基线换弹时长 %v, 实际 %v", baseReloadMs, got)
	}

	a.Reloading = false
	a.Mags[0] = 10
	a.Hex = HexQuicken
	if !r.StartReload(a, now.Add(time.Second)) {
		t.Fatal("应能再次换弹")
	}
	want := time.Duration(int(float64(baseReloadMs)*0.70)) * time.Millisecond
	if got := a.ReloadEnd.Sub(now.Add(time.Second)); got != want {
		t.Fatalf("快手换弹时长应 %v, 实际 %v", want, got)
	}
	found := false
	for _, e := range r.pending {
		if e.Type == EvReloadStart && e.Player == a.Id && e.Ms == uint16(want/time.Millisecond) {
			found = true
		}
	}
	if !found {
		t.Fatal("EvReloadStart 应携带缩放后的 ms")
	}
}
