package main

import (
	"math"
	"os"
	"regexp"
	"strconv"
	"testing"
)

// 《霓虹都会》DLC 交叉校验：
// 1. maps/neon-city.json 可被 LoadWorld 正常解析（方块/出生点/revision）；
// 2. 客户端 main.ts 里硬编码的 NEON_CITY_REVISION 与服务端按文件字节
//    计算的 revision 逐位一致——地图重新生成后若忘记同步客户端常量，
//    该测试会立刻红。
func TestNeonCityMapAndRevisionSync(t *testing.T) {
	w, err := LoadWorld("../maps/neon-city.json")
	if err != nil {
		t.Fatalf("霓虹都会地图加载失败: %v", err)
	}
	if len(w.Spawns) != 220 {
		t.Fatalf("出生点 = %d, 期望 220", len(w.Spawns))
	}
	if len(w.Blocks) < 300 {
		t.Fatalf("方块数 = %d, 期望 ≥300 的城市体量", len(w.Blocks))
	}
	// 出生点必须站在实体地面上（CanOccupy 允许站立）。
	for i, s := range w.Spawns {
		pos := Vec3{X: s[0], Y: s[1], Z: s[2]}
		if pos.Y > 1 && !w.CanOccupy(pos, StandingHeight) {
			// 高处出生点（屋顶）可能贴着楼体边沿，仅报告明显悬空的
			suspended := true
			for _, b := range w.Blocks {
				if b.T == 12 {
					continue
				}
				if pos.X >= b.X && pos.X <= b.X+b.W && pos.Z >= b.Z && pos.Z <= b.Z+b.D && math.Abs(b.Y+b.H-pos.Y) < 0.5 {
					suspended = false
					break
				}
			}
			if suspended {
				t.Fatalf("出生点 %d (%.1f,%.1f,%.1f) 悬空", i, pos.X, pos.Y, pos.Z)
			}
		}
	}

	// 客户端常量与服务端 revision 交叉校验。
	src, err := readFile("../client/src/main.ts")
	if err != nil {
		t.Fatal(err)
	}
	re := regexp.MustCompile(`NEON_CITY_REVISION = (0x[0-9a-fA-F]+)`)
	m := re.FindSubmatch(src)
	if m == nil {
		t.Fatal("客户端 main.ts 缺少 NEON_CITY_REVISION 常量")
	}
	clientRev, err := strconv.ParseUint(string(m[1]), 0, 32)
	if err != nil {
		t.Fatal(err)
	}
	if uint32(clientRev) != w.Revision {
		t.Fatalf("revision 两端不一致: 服务端 0x%x vs 客户端 0x%x（重新生成地图后需同步客户端常量）", w.Revision, clientRev)
	}
}

func readFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}
