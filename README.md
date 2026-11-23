# MySecondWebGame 基础框架

项目源码采用 [PolyForm Noncommercial 1.0.0](LICENSE)，可用于非商业用途，并可在许可条款下修改和分发。第三方内容适用各自的许可。

## 目录
- `server/` Go 高并发后端（Fiber + WebSocket + PostgreSQL + Redis + NATS）
- `web/` Vue3 + Element Plus + TailwindCSS 深色前端
- `desktop/` Electron 客户端壳
- `packages/game-data/` 前后端共享游戏配置单一数据源
- `infra/` 本地基础设施（PostgreSQL + Redis + NATS）

## 快速开始
1. 一键准备本地环境：`bash infra/scripts/setup-mac-local.sh`
2. 启动后端：在 `server/` 下分别运行 `go run ./cmd/server` 和 `go run ./cmd/action_worker`
3. 启动前端：在 `web/` 下 `npm install && npm run dev`
4. 启动桌面端（可选）：在 `desktop/` 下 `npm install && WEB_DEV_URL=http://localhost:5173 npm start`
5. 清空本地开发数据库并重建：`make dev-reset-db`

> 若你在国内网络环境，建议先配置 Docker Desktop `registry-mirrors` 后再执行第 1 步。

### 数据库脚本（统一入口）
- 初始化（不存在则创建 `game` 库并建表）：`bash infra/scripts/init-db-local.sh`
- 破坏性重置（删库重建）：`bash infra/scripts/reset-dev-db.sh --yes`
- 手动执行 schema SQL：`bash infra/scripts/apply-schema-sql.sh init.sql`
- 扩展用户语言约束（支持 `zh-CN/zh-TW/en-US/ja/ko/ru`）：`bash infra/scripts/migrate-user-settings-language.sh`

## 本地生产近似运行（dist + Docker Nginx）
用于本地验证“前端打包产物 + 同源 `/api` `/ws` 反代后端”的真实链路。

1. 启动后端（默认 `localhost:8080`）：
   - `cd server && go run ./cmd/server`
   - `cd server && go run ./cmd/action_worker`
2. 一键构建并启动 dist 网关（默认端口 `8081`）：
   - `bash infra/scripts/run-local-dist-nginx.sh start`
3. 浏览器访问：
   - `http://localhost:8081`

### 常用命令
- 停止网关：`bash infra/scripts/run-local-dist-nginx.sh stop`
- 查看状态：`bash infra/scripts/run-local-dist-nginx.sh status`
- 查看日志：`bash infra/scripts/run-local-dist-nginx.sh logs`
- 仅重启网关不重新 build：`bash infra/scripts/run-local-dist-nginx.sh start --no-build`

### 可配置参数（环境变量）
- `LOCAL_DIST_PORT`：网关端口（默认 `8081`）
- `BACKEND_HOST`：后端主机（默认 `host.docker.internal`）
- `BACKEND_PORT`：后端端口（默认 `8080`）
- `CONTAINER_NAME`：容器名（默认 `mysecondwebgame-local-dist`）

示例（后端在本机 18080，网关用 9090）：

```bash
LOCAL_DIST_PORT=9090 BACKEND_HOST=host.docker.internal BACKEND_PORT=18080 \
bash infra/scripts/run-local-dist-nginx.sh start
```

### 故障排查
- 页面可打开但 API 报错：先确认后端是否已启动并监听对应端口。
- WS 连接失败：检查网关日志 + 后端日志，确认 `/ws` 可升级。
- Linux 下无法访问宿主机后端：请升级 Docker 到支持 `host-gateway` 的版本（脚本会自动注入 `host.docker.internal` 映射）。

## 鉴权
- 已切换为 `JWT`。
- 注册：`POST /api/register` 返回 `userId + token`
- 登录：`POST /api/login` 返回 `userId + token`
- WebSocket 绑定：`auth.bind` payload 需传 `{ token }`
- 注册/登录最小长度约束：`username >= 3`、`password >= 6`
- 鉴权错误响应已统一为结构化格式：`{ code, message, debug? }`
  - 生产环境：返回 `code + message`（不暴露内部细节）
  - 开发环境（`APP_ENV=development`）：额外返回 `debug` 便于排错

## 前端入口与鉴权路由
- 单一 SPA 路由：
  - `/` 游戏主页（介绍与入口）
  - `/instructions` 说明页
  - `/auth` 登录/注册页
  - `/auth/forgot-password` 忘记密码页
  - `/game/*` 主游戏页（受路由守卫保护，左侧菜单路由化）
    - `/game/home`
    - `/game/market`
    - `/game/shop`
    - `/game/auction`
    - `/game/combat`
    - `/game/attributes`
    - `/game/training`
    - `/game/work`
    - `/game/exploration`
    - `/game/building`
    - `/game/crafting`
    - `/game/warehouse`
    - `/game/enhance`
    - `/game/affix`
    - `/game/tasks`
    - `/game/panel/:tab`
- 未登录访问 `/game` 会被重定向到 `/auth`。
- `/game/market` 与 `/game/auction` 额外要求邮箱已验证，未验证会跳到 `/game/panel/settings?subTab=email`。

## 接口分层（HTTP / WS）
- HTTP（按领域列当前主入口，完整真值见 `server/internal/httpapi/server_http_base.go`）：
  - 公共：`GET /healthz`、`GET /api/system/version`
  - 认证：`POST /api/register`、`POST /api/login`、`POST /api/password/forgot/send|verify|reset`
  - 人物：`GET /api/bootstrap/init`、`GET /api/player/stats`、`GET/POST /api/player/presence`、`GET/POST /api/player/settings/*`、`GET/POST /api/player/loadouts/*`
  - 背包与装备：`GET /api/inventory/instances`、`GET /api/items/detail`、`POST /api/items/use-skillbook`、`POST /api/items/recycle`、`POST /api/equipment/equip|unequip|unequip-instance|repair`
  - 市场与拍卖：`POST /api/market/list|list-instance|buy`、`GET /api/market/listings|buy-orders|fees/summary`、`POST /api/market/buy-orders|buy-orders/fill|buy-orders/cancel`、`GET /api/auction/listings|detail|bids`、`POST /api/auction/list-instance|bid|cancel`
  - 商店：`GET /api/shop/offers`、`POST /api/shop/buy|sell`
  - 战斗：`GET/POST /api/battle/skills/config`、`POST /api/battle/skills/owned/grant|revoke`、`GET/POST /api/battle/rooms*`、`GET/POST /api/battle/points*`
  - 装备扩展：`POST /api/equipment/affix/detach|attach|enchant`、`GET /api/equipment/operation-info/:instanceID`
  - 猫舍：`POST /api/cattery/adopt`、`GET /api/cattery/commissions/*`、`POST /api/cattery/commissions/start|claim`、`GET/POST /api/cattery/kitties/:kittyID/*`
  - 队列与任务：`GET /api/actions/catalog`、`POST /api/actions/queue/batch`、`GET /api/tasks/daily`、`POST /api/tasks/daily/:taskID/submit`、`GET /api/warehouses/logs`
  - 管理：`POST /api/admin/release/announce`、`GET /api/admin/battle/metrics`、`POST /api/admin/battle/runs/cleanup|battle/rooms/reinitialize`、`POST /api/admin/inventory/grant`、`POST /api/admin/cattery/trigger`、`GET/POST /api/admin/users/*`
- WebSocket：
  - 连接：`GET /ws`
  - 上行消息：`auth.bind`、`game.state.get`、`idle.tick.request`、`chat.send` / `chat.global.send`、`action.run`、`action.enqueue`、`action.queue.get`、`action.cancel`、`action.queue.move`、`action.queue.run_now`、`action.offline.rewards.get`、`presence.ping`、`battle.stream.set`
  - 下行消息：`auth.bound`、`auth.kicked`、`game.state.patch`、`idle.tick.result`、`chat.message` / `chat.global.message`、`action.queue.updated`、`player.attributes.updated`、`action.offline.rewards.result`、`battle.stream.state`、`error`

### HTTP / WS 选型准则（强制）
- **优先 HTTP**：需要明确请求-响应闭环、可重试、可审计、可幂等的读写（例如配置保存、市场交易、仓库摆放）。
- **优先 WS**：需要低延迟交互 + 服务端持续推送的链路（例如动作队列执行、战斗事件、实时状态 patch）。
- **同一写操作只保留一个入口**：不要同时暴露 HTTP 与 WS 两个写入入口，避免一致性分叉。
- **快照与增量分层**：只读快照默认走 HTTP；实时增量更新走 WS 推送。
- **幂等边界**：凡是需要 `idempotency key` 或事务回放能力的命令，必须走 HTTP。

## 统一响应结构（HTTP + WS）
- HTTP 成功：`{ ok: true, ts, data }`
- HTTP 失败：`{ ok: false, ts, error: { code, message, debug?, instanceState? } }`
- HTTP 成功/失败都会附带 `cost`
- WS 事件：`{ type, ok, ts, data?, error? }`
  - 错误事件：`type = "error"`，`ok = false`，`error = { code, message, debug? }`
- 说明：前端可基于 `ok` 和 `error.code` 做统一分流处理，避免按接口手写分支。

## HTTP / WS 自适应压缩封包
- 后端统一支持响应压缩：当 JSON 响应体超过阈值后，返回压缩封包。
- 压缩封包格式（当前）：`zlib-base64`
- 压缩配置来源：`packages/game-config/source/global-config.json`
  - `transport.compression.enabled`
  - `transport.compression.minBytes`
  - `transport.compression.level`
  - `transport.compression.format`
- HTTP 成功响应可能为：
  - 普通：`{ ok, ts, data }`
  - 压缩：`{ ok, ts, compressed: { format, data, originalBytes, compressedBytes } }`
- WS 事件可能为：
  - 普通：`{ type, ok, ts, data|error }`
  - 压缩：`{ cmp: "zlib-base64", compressed: { ... } }`
- 前端已在统一服务层自动识别并解压，业务层无需手动处理。
- WS 鉴权：连接建立后需先发送 `auth.bind { token }`，其余业务消息在未绑定状态会返回 `error`（`code: WS_UNAUTHORIZED`）。
- WS 支持握手自动绑定（可选）：可在连接时通过 `Authorization: Bearer <token>` 或 `?token=<jwt>` 传入，服务端会自动完成绑定并下发 `auth.bound`。
- WS 错误响应统一为：`{ type: "error", payload: { code, message } }`，如 `WS_UNKNOWN_MESSAGE_TYPE`。

## 安全防护（接口防爆破 / 防篡改）
- 框架级统一处理（可扩展友好）：
  - `/api` 通过路由分组统一挂载安全中间件链（限流 + 鉴权）。
  - 受保护接口统一走 `authMiddleware`，新增接口只要挂在 protected group 下即可自动继承鉴权防护。
- 防爆破：
  - 已增加 API 速率限制（按 IP）：
    - `/api/register`：每分钟 6 次
    - `/api/login`：每分钟 12 次
    - `/api/*` 全局：每分钟 120 次
  - 已增加登录失败锁定（按 `IP + 用户名`）：
    - 10 分钟窗口内累计失败达到 5 次，锁定 10 分钟
  - 登录失败路径增加固定延迟，降低凭据爆破效率。
- 防篡改：
  - 所有玩家资源相关接口都从 JWT 解析 `userId`，不信任客户端传入的用户身份。
  - 装备/市场交易在服务端执行所有权与状态校验，并在数据库事务中落库。
  - 市场与 WS 输入增加边界约束（价格、数量、聊天长度、战斗间隔），拒绝异常参数。
  - `POST /api/market/buy` 强制 `X-Idempotency-Key`，降低重放与重复扣款风险。
- Redis 运行态只读巡检脚本（不扫 key、不写数据）：`bash infra/scripts/check-redis-runtime.sh`

## 前端主题规范
- 全站配色通过 `web/src/style.css` 统一主题变量控制（含 Element Plus CSS Variables）。
- 页面应优先复用 Element Plus 组件，不再在页面内硬编码深色值（如 `neutral-950/900`）。

## 调度内核
- 可切换策略：`minheap`（默认）/`timewheel`
- 配置项：
  - `SCHEDULER_ENGINE`
  - `SCHEDULER_TICK_MS`
  - `SCHEDULER_WHEEL_SIZE`

## 消息队列（MQ）
- 已接入 `NATS`（默认）。
- 关键事件通过 MQ 发布（示例：`market.listed`、`market.bought`）。

## 共享配置单一数据源
- 源数据：`packages/game-data/source/items.json`（或 `items.csv`）+ `version.json`
- 生成输出：
  - `packages/game-data/generated/game-data.json`
  - `packages/game-data/generated/game_data.go`
  - `packages/game-data/generated/game-data.ts`
- 构建脚本：`node packages/game-data/scripts/build.mjs`

## 全局运行配置（独立于 game-data）
- 单一配置文件：`packages/game-config/source/global-config.json`
- 用途：仓库默认尺寸、仓库记录默认查询窗口、离线推进上限、Redis 固定窗口限流阈值、派生属性公式（`derivedStats`）等“运行时阈值”。
- 约束：`game-data` 只用于内容配置；运行时全局变量统一放在 `game-config`。

## 前后端共享配置
- 共享配置文件：`packages/game-config/source/shared-global-config.json`
- 用途：前后端都需要一致执行的公开常量（例如市场价格步长规则 `market.priceSteps`）。
- 说明：`shared-global-config` 仅放可公开常量；敏感运行参数仍保留在 `global-config.json`。

## 市场接口并发约束
- 受保护 HTTP `POST` 默认受 `replayGuardMiddleware` 保护，调用方应统一携带 `X-Idempotency-Key`。
- `POST /api/market/buy` 必须带 `X-Idempotency-Key`，用于重试幂等。
- 已支持阶梯手续费配置（`market_fee_tiers`），默认基准费率 5%。
- 已支持玩家求购单与拍卖链路：
  - `GET /api/market/buy-orders`
  - `POST /api/market/buy-orders`
  - `POST /api/market/buy-orders/fill`
  - `GET /api/auction/listings`
  - `POST /api/auction/bid`
- 手续费观测接口：`GET /api/market/fees/summary`

## 日常委托任务
- `GET /api/tasks/daily`：拉取任务列表（每 4 小时刷新，每次 +2，最多持有 24，不过期）。
- `POST /api/tasks/daily/:taskID/submit`：提交任务，消耗资源并结算金币/材料奖励。

## 多动作与执行队列
- `GET /api/actions/catalog`：获取战斗/锻炼/劳作/制作动作目录（来自 `packages/game-data`）。
- WS `action.run`：立即执行指定动作（支持 `count` 次）。
- WS `action.enqueue`：将动作加入队列（支持 `count` 次）。
- WS `action.queue.get`：拉取当前执行队列与属性经验。
- 下行推送：
  - `action.queue.updated`
  - `player.attributes.updated`
  - `game.state.patch`（资源变更同步）

## 仓库明细模型（无布局单容器）
- 存储结构：`player_item_instances`（`state='inventory'` 作为仓库单容器视图） + `player_warehouse_change_logs`（审计日志）。
- 并发写采用实例行锁与事务保证一致性。
- 缓存策略：`GET /api/inventory/instances` 使用 Redis 快照缓存（键：`warehouse:items:{userId}`，TTL 30s），任一仓库/库存写路径会先失效再回填。

## 环境与部署文档
- 文档总览入口：`docs/index.md`
- 前端开发阅读入口：`docs/frontend/README.md`
- 后端开发阅读入口：`docs/backend/README.md`
- 部署基线（native split）：`docs/deployment/native-split-deployment.md`
- 本地一键：`bash infra/scripts/setup-mac-local.sh`
- 线上按角色部署：`bash infra/scripts/deploy-linux-native-split.sh <role> [git_repo_url] [app_dir]`
- 线上快速更新：`bash infra/scripts/update-linux-native-split.sh <server|battle-worker|backend|web|all> [app_dir]`
- 调度设计：`docs/architecture/scheduler-design.md`
- 市场事务设计：`docs/backend/market-transaction.md`
- 装备与属性设计：`docs/frontend/equipment-and-stats.md`
- 动作队列与属性经验：`docs/backend/action-queue-design.md`
- AI 开发规范：`docs/catalog/ai-development-guidelines.md`
- 共享数据管线：`docs/backend/shared-game-data.md`
- 线上配置与扩容：`docs/ops/infra-sizing-and-scale.md`
- Harness 质量门禁：`docs/ops/harness-standard.md`
- **Harness 编码规范（Claude Code 强制）**：`docs/ops/harness-coding-standard.md`

## Harness 执行（严格 PR 门禁）
1. `node packages/game-data/scripts/build.mjs`
2. `cd server && go mod tidy && go test ./...`
3. `cd web && npm ci && npm run build && npm run test:unit && npm run test:e2e`
4. 或在仓库根目录执行：`make harness-pr`
