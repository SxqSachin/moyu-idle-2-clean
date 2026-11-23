# 本地开发环境安装（macOS）

## 一键脚本（推荐）
```bash
bash infra/scripts/setup-mac-local.sh
```

## 1) 安装本机运行时（不安装本机 PG/Redis）
```bash
brew update
brew install go node docker docker-compose
go version
```

## 2) 启动 docker 基础设施（PG + Redis + NATS）
```bash
cp infra/.env.example infra/.env
bash infra/scripts/pull-images.sh
docker compose -f infra/docker-compose.yml up -d
```

> 可直接在 `infra/.env` 里改为其他公共源（无需阿里专属地址）。

### 若拉镜像超时（Docker Hub 网络问题）
1. Docker Desktop -> **Settings** -> **Docker Engine**，加入镜像加速（示例）：
```json
{
  "features": { "buildkit": true },
  "registry-mirrors": [
    "https://docker.1ms.run",
    "https://hub-mirror.c.163.com",
    "https://mirror.ccs.tencentyun.com"
  ]
}
```
> 可选：阿里云镜像加速地址（账号专属）`https://<your-id>.mirror.aliyuncs.com`，建议放到第一位。

2. 点击 **Apply & Restart** 后重试：
```bash
bash infra/scripts/pull-images.sh
docker compose -f infra/docker-compose.yml up -d
```

## 3) 初始化数据库（通过 docker 内 pg 执行）
```bash
bash infra/scripts/init-db-local.sh
# 或 make local-init-db
```
- 脚本会直接执行 `server/schema/init.sql` 作为数据库初始化真值来源；可重复运行。

### 本地重置数据库（破坏性）
```bash
bash infra/scripts/reset-dev-db.sh --yes
# 或 make dev-reset-db
```
- 会直接 drop/create `game` 数据库后重新应用 `server/schema/init.sql`，不做任何兼容或迁移。

### 数据库脚本统一用法
```bash
# 1) 初始化（不存在则创建 game 库并建表）
bash infra/scripts/init-db-local.sh

# 2) 破坏性重置（删库重建）
bash infra/scripts/reset-dev-db.sh --yes

# 3) 手动执行指定 schema SQL（默认从 postgres 连接执行）
bash infra/scripts/apply-schema-sql.sh init.sql
```

## 4) 启动网关后端（HTTP + WS）
```bash
cd server
cp .env.example .env
cd ..
node packages/game-data/scripts/build.mjs
cd server
go mod tidy
go run ./cmd/server
```

## 5) 启动战斗 worker（独立进程，无 HTTP/WS）
```bash
cd server
go run ./cmd/battle_worker
```

- worker 启动后会定期写入 Redis 心跳键：`<MQ_TOPIC_PREFIX>:battle:worker:heartbeat:<worker_id>`。
- 默认心跳配置来自 `packages/game-config/source/global-config.json` 的 `battle.workerHeartbeat`（TTL 30s，刷新 5s）。
- 历史 run 数据清理默认保留最近 `battle.runHistoryKeepCount` 场完整 timeline（默认 20），更早记录自动降级为摘要。

## 6) 启动前端
```bash
cd web
npm install
npm run dev
```

## 战斗回放本地回归检查
1. 房间全员 ready 后进入战斗，观察单位卡片 HP/MP/存活状态会随回放事件变化。
2. 单场胜利后应先结算该场奖励，再按 `battle.interBattleIntervalMs` 进入下一场准备。
3. 第一场（wave=1）会抓取玩家实时属性/装备/技能；后续场次应继承上一场结算状态（HP/MP/技能冷却），不会再同步外部实时属性。
4. 多场流程中若后续失败，应保留此前已胜利场次的结算收益。
5. 战斗中的单位卡片悬浮应可看到该单位属性、装备摘要与技能冷却信息。
6. worker 调度应由 `next_tick_at` 驱动（非固定轮询），回放节拍仍遵循 `battle.timelinePushIntervalMs`。

更多前端工程约定（含路径别名）请见：`docs/frontend/frontend-development.md`。

## 7) 启动 Electron（可选）
```bash
cd desktop
npm install
WEB_DEV_URL=http://localhost:5173 npm start
```
