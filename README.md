# Pixel Strike

<p align="center">
  <a href="https://linux.do/" title="LINUX DO"><img src="https://linux.do/logo-64.svg" width="32" height="32" alt="LINUX DO"></a>
  <br>
  <a href="https://linux.do/"><strong>LINUX DO · Where Possible Begins</strong></a>
</p>

浏览器 3D 战术死斗：单房 100 人、256×256 体素地图、Go 权威服务端、Three.js 客户端、WebSocket 二进制协议。

## 一场持续到国庆的共创实验

Pixel Strike 不是一个已经定型的成品，而是一场开放实验：如果每个人都带来一个想法、一张地图、一种武器手感、一次性能优化或一个有趣的 Bug 修复，这个项目最后会变成什么样？

实验从现在持续到 **2026 年国庆（10 月 1 日）**。欢迎通过 Issue、Pull Request、测试反馈或玩法讨论参与。国庆当天我们会回头看看：一个最初由 AI 生成的百人网页 FPS，在大家共同参与之后，究竟长成了什么样子。

## AI 生成免责声明

当前仓库中的代码、UI/UX、玩法机制、枪械数值、地图生成逻辑、合成音效、测试与文档，均由 AI 生成或在 AI 协作下完成。项目具有实验性质，可能存在 Bug、平衡性问题、安全风险、兼容性问题或不完善的实现。

本项目不承诺生产可用性、公平性或持续维护，也不构成任何商业、法律或安全保证。部署、修改或公开运营前，请自行审查代码、依赖、素材许可及当地适用规则，并自行承担使用风险。

本项目仅用于学习、研究与娱乐，不主张对任何可能涉及的第三方名称、视觉风格或其他素材享有权利。如认为仓库内容侵犯您的合法权益，请通过 GitHub Issue 联系并附上权利说明与相关链接；确认后我会立即下架相关内容并配合整改。

## 项目结构

```text
pixel-strike/
├── client/                   # Three.js 客户端、HUD、音效与 Nginx
│   ├── src/                  # 渲染、输入、预测、网络与玩法反馈
│   └── public/sfx/           # 合成游戏音效
├── server/                   # Go 权威服务端、房间、碰撞与持久化
├── tools/                    # 地图/音效生成器与百人压测客户端
├── map.json                  # 体素地图和出生点
├── docker-compose.yml        # 本地 Docker 编排
├── docker-compose.prod.yml   # 生产 Docker 编排
└── README.md
```

## 当前玩法

- 无限 FFA，无回合、经济和购买阶段；3 秒自动复活，2 秒出生保护（开火即解除）。
- 自由选择一把主武器和一把副武器；1/2/3 切换主武器、副武器、刀。
- 12 把枪、刀和一枚 HE，覆盖手枪、冲锋枪、步枪、狙击枪与霰弹枪。
- 真人玩家按服务端解析出的客户端 IP 绑定进度账号；每把枪分别累计击杀，100 杀解锁黄金皮肤、500 杀解锁钻石皮肤。
- 进入游戏时可为主、副武器选择默认、黄金、钻石或随机已解锁皮肤；随机武器会先抽枪，再抽取该枪已解锁的皮肤。
- 弹匣、备弹、换弹、护甲穿透、动态准星、同步弹道、移动/落地散布、蹲伏精度、后坐力、跳跃、爆头与刀背刺。
- AK-47 与 Deagle 奖励精准爆头；AWP 开镜需 320 ms、SSG 08 需 240 ms 稳定，远近枪声、爆炸和脚步按距离与方向播放。
- 服务端 60 Hz 权威模拟，最多 200 ms 延迟补偿。
- 每房最多 100 名真人；首位真人为管理员，可控制 0–12 个填充机器人，真人加入时机器人自动让位。
- 战场小鸡彩蛋：地图上会乱入 6 只游荡的体素小鸡，任意武器一发即可做成"炸鸡"；击杀立刻回复 25 点生命，全服战报同步播报 🍗（每 15–25 秒刷新新鸡）。

## 操作

| 输入 | 动作 |
|---|---|
| `WASD` | 移动 |
| `Space` / `Ctrl` | 跳跃 / 蹲伏 |
| 鼠标左键 / 右键 | 射击 / AWP 开镜 |
| `1` / `2` / `3` | 主武器 / 副武器 / 刀 |
| `R` / `G` | 换弹 / HE |
| `Tab` / `Esc` | 战绩 / 设置 |
| `Esc` → 中国人能飞 | 开关飞行模式，开启时全场广播「玩家名 能飞」 |
| 飞行中 `Space` / `Shift` | 上升 / 下降（贴地后 `Shift` 不再下降） |
| 飞行中 `WASD` | 空中水平移动，限制在地图范围内、最高 25 个角色身高 |

## 性能与带宽

2026-08-23 本机回环实测，100 个同房客户端持续移动、转向、开火：

- 100/100 成功加入且全部产生有效位移；无协议错误、无发送队列丢弃。
- 服务端出口约 1.60–1.91 MB/s（约 12.8–15.3 Mbps），低于 100 Mbps 预算 6 倍以上。
- 服务端工作集约 27.6 MB，私有内存约 64.3 MB；远低于 4 GB 预算。
- 房间 tick P99 约 4.1 ms，60 Hz 单 tick 预算为 16.67 ms。
- 生产 JS gzip 139.33 KB；远端 100 个角色使用 7 层 InstancedMesh 绘制完整人物。

网络策略：客户端固定 60 Hz 输入，服务端 60 Hz 权威模拟；近处玩家 30 Hz，中距 10 Hz，远距移动玩家 5 Hz、静止玩家 1 Hz。状态使用厘米位置、半度角、8 位小增量、字段位图、2 秒关键帧和 2300 字节快照上限。地图只按修订号下载一次，排行榜按需获取。

> CDN 能缓存 HTML、JS、音效和 `map.json`，不能缓存实时 WebSocket 数据。`/ws` 必须启用 WebSocket 透传并关闭代理缓冲。

## 运行

要求：Docker Compose，或 Go 1.22 + Node.js 22。

```bash
docker compose up -d --build
```

- 游戏：<http://localhost:8000>
- 管理面板：<http://localhost:12888/admin.html>（需配置 `ADMIN_PASSWORD`）
- 服务端健康检查：<http://localhost:8080/healthz>
- 运行指标：<http://localhost:8080/api/stats>

本地开发：

```bash
cd server
go run .

cd ../client
npm ci
npm run dev
```

## 验证

```bash
cd server
go test ./...

cd ../client
npm run build

cd ..
node tools/bots.mjs ws://localhost:8080/ws -n 100 --duration 60
```

压测只有在请求数全部加入、每个客户端都移动且无拒绝时才返回成功。

## CDN / 反向代理

同域部署建议只暴露前端 Nginx；仓库内的 `client/nginx.conf` 已代理 `/ws` 和 `/api/`。外层 CDN 应使用：

```nginx
location /ws {
    proxy_pass http://127.0.0.1:8080/ws;
    proxy_http_version 1.1;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "upgrade";
    proxy_buffering off;
    proxy_request_buffering off;
    proxy_read_timeout 300s;
}

location ~* ^/(assets|sfx)/ {
    proxy_pass http://127.0.0.1:8000;
    expires 1y;
    add_header Cache-Control "public, immutable, no-transform";
}

location = /map.json {
    proxy_pass http://127.0.0.1:8000/map.json;
    expires 1y;
    add_header Cache-Control "public, immutable, no-transform";
}
```

不要给 `/ws` 开启压缩转换、缓存或缓冲。若 CDN 有 WebSocket 开关，需要显式启用。动态游戏流量按当前实测仍可直接控制在 100 Mbps 内。

## 环境变量

| 变量 | 默认值 | 说明 |
|---|---|---|
| `PORT` | `8080` | HTTP / WebSocket 端口 |
| `DB_PATH` | `./stats.db` | SQLite 文件 |
| `MAP_PATH` | `../map.json` | 地图文件 |
| `ALLOWED_ORIGIN` | 空 | 生产环境允许的完整 Origin |
| `ADMIN_PASSWORD` | 空 | 管理面板密码；为空时禁用管理接口 |
| `ADMIN_COOKIE_SECURE` | `false` | 强制后台会话 Cookie 仅通过 HTTPS 发送 |
| `TRUSTED_PROXY_CIDRS` | 空 | 信任的反向代理 CIDR 列表，用于正确获取客户端 IP |
| `VITE_WS_URL` | 空 | 跨域部署时的 WebSocket URL；同域保持空 |

Compose 默认强制后台 Cookie 使用 HTTPS，并将 `TRUSTED_PROXY_CIDRS` 设为 Docker 网段 `172.16.0.0/12`；明文 HTTP 调试时设置 `PIXEL_STRIKE_ADMIN_COOKIE_SECURE=false`，其他代理网络请按实际网段覆盖。`ADMIN_PASSWORD` 通过宿主机变量 `PIXEL_STRIKE_ADMIN_PASSWORD` 注入。

Compose 已限制服务端 512 MB、静态前端 128 MB，总上限 640 MB。

## 协议 v6 摘要

所有多字节字段均为 Little-Endian。

- 客户端：Join `01`、Input `02`、Fire `03`、Reload `04`、Grenade `06`、Switch `08`、Loadout `09`、RosterRequest `0A`、ToggleFlight `0B`、Ping `F0`。Join 与 Loadout 均携带主、副武器皮肤选择，服务端按解锁进度校验。
- 服务端：Welcome `81`、Snapshot `82`、Events `83`、Pong `84`、Self `86`、Roster `87`、Reject `88`。
- Snapshot：`tick(u32) + inputAck(u16) + count(u8)`，玩家记录由 `id(u16) + fieldMask(u16)` 开始；`0x8000` 表示完整关键帧，状态包含角色皮肤和当前枪械皮肤。

主要实现位置：`server/netstate.go`（带宽）、`server/sim.go`（权威玩法）、`server/room.go`（60 Hz 房间）、`client/src/net.ts`（协议）、`client/src/player.ts`（预测与实例化角色）、`tools/bots.mjs`（压测）。
