# Native Split 部署指南（唯一保留方案）

## 适用范围
- 目标：生产环境按角色拆分部署（`pg | redis | nats | server | action-worker | battle-worker | kitty-worker | server-with-kitty-worker | web | lb`）。
- 平台：Debian / Ubuntu（`systemd + apt`）。
- 脚本入口：`infra/scripts/deploy-linux-native-split.sh`。

## TL;DR
1. 每台角色机器执行同一脚本，不同 `role`。
2. `server` / `action-worker` / `battle-worker` / `kitty-worker` 必须显式设置 `DB_HOST`、`REDIS_ADDR`、`NATS_URL`。
3. 先部署基础设施（`pg/redis/nats`），再部署应用（`server/action-worker/battle-worker/kitty-worker/web`），最后部署入口（`lb`）。
4. 以脚本 preflight + `/readyz` + systemd 状态作为上线门禁。

## 角色与拓扑
- `pg`：PostgreSQL
- `redis`：Redis
- `nats`：NATS
- `server`：API + WS 网关
- `action-worker`：普通动作/强化/建筑等 action queue 到期推进 worker
- `battle-worker`：战斗推进 worker
- `kitty-worker`：猫舍随记/随遇生成 worker
- `server-with-kitty-worker`：同机部署 API 网关、action-worker 与 kitty-worker（适合你当前单机后端形态）
- `web`：前端静态资源节点（本机 Nginx，仅静态）
- `lb`：统一入口 Nginx（反代 web/api/ws）

## 前置准备
1. 代码仓库可访问（推荐 SSH deploy key）。
2. 机器间内网放通端口：
   - PostgreSQL `5432`
   - Redis `6379`
   - NATS `4222`（可选监控 `8222`）
   - server `8080`
   - web/lb `80/443`
3. 仓库包含共享产物目录：`packages/game-data/generated/`。

## 标准部署顺序（按机器执行）
```bash
ssh-keygen -t ed25519 -C "deploy-key-your-project"
cat .ssh/id_ed25519.pub

REPO_URL=https://github.com/OWNER/moyu-idle-2-clean.git
APP_DIR=/opt/moyu-idle

# 1) PG
DB_USER=game DB_PASSWORD='replace-me' DB_NAME=game \
bash infra/scripts/deploy-linux-native-split.sh pg "${REPO_URL}" "${APP_DIR}"

# 2) Redis
bash infra/scripts/deploy-linux-native-split.sh redis "${REPO_URL}" "${APP_DIR}"

# 3) NATS
bash infra/scripts/deploy-linux-native-split.sh nats "${REPO_URL}" "${APP_DIR}"

# 4) Server（必须显式端点；脚本会同时部署 action-worker）
DB_HOST=10.0.0.10 PG_PORT=5432 DB_USER=game DB_PASSWORD='replace-me' DB_NAME=game \
JWT_SECRET='replace-me' APP_PORT=8080 \
REDIS_ADDR=10.0.0.20:6379 NATS_URL=nats://10.0.0.30:4222 \
REAL_IP_TRUSTED_PROXY_CHECK=true \
REAL_IP_TRUSTED_PROXIES=198.51.100.10,198.51.100.11 \
REAL_IP_PROXY_HEADER=X-Forwarded-For \
bash infra/scripts/deploy-linux-native-split.sh server "${REPO_URL}" "${APP_DIR}"

# 5) Action Worker（若你没有通过 server/server-with-kitty-worker 自动部署，则单独执行）
DB_HOST=192.0.2.10 PG_PORT=5432 DB_USER=game DB_PASSWORD='replace-me' DB_NAME=game \
JWT_SECRET='replace-me' \
REDIS_ADDR=192.0.2.20:6379 NATS_URL=nats://192.0.2.20:4222 \
ACTION_WORKER_DB_POOL_MAX_CONNS=4 ACTION_WORKER_BATCH_SIZE=8 ACTION_WORKER_USER_TIMEOUT_MS=2000 \
bash infra/scripts/deploy-linux-native-split.sh action-worker "${REPO_URL}" "${APP_DIR}"

# 6) Battle Worker（必须显式端点）
DB_HOST=10.0.0.10 PG_PORT=5432 DB_USER=game DB_PASSWORD='replace-me' DB_NAME=game \
JWT_SECRET='replace-me' \
REDIS_ADDR=10.0.0.20:6379 NATS_URL=nats://10.0.0.30:4222 \
bash infra/scripts/deploy-linux-native-split.sh battle-worker "${REPO_URL}" "${APP_DIR}"

# 7) Kitty Worker（必须显式端点）
DB_HOST=10.0.0.10 PG_PORT=5432 DB_USER=game DB_PASSWORD='replace-me' DB_NAME=game \
JWT_SECRET='replace-me' \
REDIS_ADDR=10.0.0.20:6379 NATS_URL=nats://10.0.0.30:4222 \
bash infra/scripts/deploy-linux-native-split.sh kitty-worker "${REPO_URL}" "${APP_DIR}"

DB_HOST=192.0.2.10 PG_PORT=5432 DB_USER=game DB_PASSWORD='replace-me' DB_NAME=game \
JWT_SECRET='replace-me' \
REDIS_ADDR=192.0.2.20:6379 NATS_URL=nats://192.0.2.30:4222 \
bash infra/scripts/deploy-linux-native-split.sh kitty-worker "${REPO_URL}" "${APP_DIR}"


# 7-alt) 若你计划与 server 同机部署 kitty-worker（单机后端推荐；脚本会同时部署 action-worker）
DB_HOST=10.0.0.10 PG_PORT=5432 DB_USER=game DB_PASSWORD='replace-me' DB_NAME=game \
JWT_SECRET='replace-me' APP_PORT=8080 \
REDIS_ADDR=10.0.0.20:6379 NATS_URL=nats://10.0.0.30:4222 \
REAL_IP_TRUSTED_PROXY_CHECK=true \
REAL_IP_TRUSTED_PROXIES=198.51.100.10,198.51.100.11 \
REAL_IP_PROXY_HEADER=X-Forwarded-For \
bash infra/scripts/deploy-linux-native-split.sh server-with-kitty-worker "${REPO_URL}" "${APP_DIR}"

# 8) Web
bash infra/scripts/deploy-linux-native-split.sh web "${REPO_URL}" "${APP_DIR}"

# 9) LB
LB_ENABLE_TLS=0 \
LB_DOMAIN=moyu-idle.com \
LB_WWW_DOMAIN=test.moyu-idle.com \
LB_CERT_FULLCHAIN_PATH=/etc/letsencrypt/live/moyu-idle.com/fullchain.pem \
LB_CERT_PRIVKEY_PATH=/etc/letsencrypt/live/moyu-idle.com/privkey.pem \
LB_WEB_UPSTREAMS=198.51.100.10:80 \
LB_API_UPSTREAMS=198.51.100.15:8080 \
LB_WS_UPSTREAMS=198.51.100.15:8080 \
bash infra/scripts/deploy-linux-native-split.sh lb "${REPO_URL}" "${APP_DIR}"
```

## 线上快速更新（增量）
- 脚本：`infra/scripts/update-linux-native-split.sh`
- 作用：
  - `backend`：更新代码 -> 重编译 server/action-worker/battle-worker/kitty-worker -> 重启四个后端服务。
  - `server`：更新 server + action-worker。
  - `action-worker` / `battle-worker` / `kitty-worker`：只更新单个 worker 服务。
  - `server-with-kitty-worker`：同机更新 server + action-worker + kitty-worker（不触发 battle-worker）。
  - `web`：更新代码并重新 build 前端静态资源，**不重启 nginx**。

### 线上 schema 热修（存量库）
- 本次市场购买修复涉及 `market_listings.quantity` 约束调整，存量库需要执行一次 DDL：
  - `bash infra/scripts/apply-schema-sql.sh server/schema/migrations/20260516_market_listing_quantity_nonnegative.sql`
- 玩家改名历史需要在存量库创建记录表；手动部署时执行：
  - `bash infra/scripts/migrate-player-username-history-native.sh /opt/moyu-idle /etc/mysecondwebgame/server.env`
  - 容器内热修也可执行 `bash infra/scripts/apply-schema-sql.sh server/schema/migrations/20260827_player_username_changes.sql`，该脚本会把 migration 应用到容器的 `${POSTGRES_DB:-game}`，而 `init.sql` 仍应用到 `postgres` 后再由 SQL 内部切库。
  - 回滚时必须先回滚依赖该表的服务版本，再执行 `bash infra/scripts/apply-schema-sql.sh server/schema/migrations/20260827_player_username_changes_down.sql`；该操作会永久删除曾用名记录，执行前需备份。
- 战斗结算、技能经验与属性经验隔离状态需要先扩展 `battle_settlement_jobs.status`、`battle_skill_exp_jobs.status`、`battle_attr_exp_jobs.status` 约束，再更新 battle-worker：
  - `bash infra/scripts/apply-schema-sql.sh server/schema/migrations/20260829_battle_settlement_quarantine.sql`
  - 迁移带 5 秒锁等待上限；应在低峰执行。若锁超时，脚本会安全退出，数据库要么仍保留旧约束，要么已换成仍约束新写入的 `NOT VALID` 新约束；稍后可直接重试，不能跳过迁移更新 worker。
  - 单机使用 `systemctl restart` 时旧进程退出后再启动新进程即可；多副本部署必须先 drain 并完全停止旧 battle-worker，再启动新版本，不允许新旧结算消费者重叠运行。
- 鱼苗池 V5 不是滚动兼容迁移：先从入口 drain 并停止所有旧版 API 节点，再执行迁移，最后启动携带新 game-data 的 V5 后端。禁止新旧鱼塘写入节点重叠运行。
  - `bash infra/scripts/apply-schema-sql.sh server/schema/migrations/20260903_fishing_pond_v5_cutover.sql`
  - `bash infra/scripts/apply-schema-sql.sh server/schema/migrations/checks/20260903_fishing_pond_v5_cutover_check.sql`；未知规则版本或非法区域不为 `0` 时脚本会失败，必须先处理再启动 V5。
  - 存量鱼塘的首次请求会先按 v4 结清 `last_settled_at` 之后的历史区间，再原子提升为 v5；因此旧主池已满时不会倒灌生成鱼苗。
  - 回滚前必须使用 V5 服务把鱼苗池清空，随后停止 V5 节点、执行 `20260903_fishing_pond_v5_cutover_down.sql`，再启动旧版；down 脚本检测到任何鱼苗时会拒绝执行，不能绕过该保护。
- 新初始化环境会直接使用 `server/schema/init.sql` 中的最新约束，无需额外执行该热修 SQL。
- native 增量更新不会自动执行数据库迁移。迁移 SQL、手动执行脚本以及服务端的存量规则兼容逻辑仍保留；涉及新表或字段时，先按需手动执行一次迁移，再更新对应服务，普通服务更新不会重复执行 DDL。使用新 ID 追加专精树或节点只递增 `balanceRevision`，不触发玩家状态迁移；只有不兼容的 `rulesetVersion` 更新或已无法解释的存量投入才由服务端事务化免费重置，并保留最近一次玩家通知。战斗属性经验通过 `battle_attr_exp_jobs` 持续重试且按 `run_id` 幂等，并由独立的 `battle.attrExpWorkers` 消费池处理（生产配置为 `8`，不再复用技能经验消费者数量）。手动迁移命令：
  - `bash infra/scripts/migrate-player-specializations-native.sh /opt/moyu-idle /etc/mysecondwebgame/server.env`
  - 仅部署战斗服务的机器使用：`bash infra/scripts/migrate-player-specializations-native.sh /opt/moyu-idle /etc/mysecondwebgame/battle-worker.env`
  - `bash infra/scripts/migrate-player-username-history-native.sh /opt/moyu-idle /etc/mysecondwebgame/server.env`
- 属性经验积压或单次处理超过阈值时，日志 `battle attr exp diagnostic` 会给出 `job_lock_ms`、`user_lock_ms`、`player_attr_ms`、`kitty_attr_ms` 与 `done_ms`；日志全局限频为每 5 秒一次，可直接区分领取索引、用户锁等待和属性写入成本。

```bash
# 后端整组更新（含重启）
bash infra/scripts/update-linux-native-split.sh backend /opt/moyu-idle

# 只更新网关与 action-worker
bash infra/scripts/update-linux-native-split.sh server /opt/moyu-idle

# 只更新 action-worker（不会重启 server，玩家通常无感）
bash infra/scripts/update-linux-native-split.sh action-worker /opt/moyu-idle

# 只更新前端（不重启 nginx）
bash infra/scripts/update-linux-native-split.sh web /opt/moyu-idle

# 可选：发布后通知在线客户端（WS 推送 + 前端延时自动刷新）
curl -X POST http://127.0.0.1:8080/api/admin/release/announce \
  -H "Content-Type: application/json" \
  -H "X-Release-Token: ${RELEASE_ANNOUNCE_TOKEN}" \
  -d '{
    "releaseId":"2026-05-15.1",
    "kind":"soft",
    "recommendedDelaySec":120,
    "messageI18nKey":"game.clientUpdateAvailable",
    "minAcceptedBuild":"2026-05-15.1"
  }'

# 可选：admin 库存发放（首版支持 template/enhanced）
curl -X POST http://127.0.0.1:8080/api/admin/inventory/grant \
  -H "Content-Type: application/json" \
  -H "X-Release-Token: ${RELEASE_ANNOUNCE_TOKEN}" \
  -d '{
    "userId": 10001,
    "itemId": "wood_sword",
    "quantity": 1,
    "grantMode": "enhanced",
    "enhanceLevel": 3
  }'

# 可选：admin 重置全部猫村联赛排行榜（积分回到配置基准，攻防胜负清零）
curl -X POST http://127.0.0.1:8080/api/admin/arena/leaderboard/reset \
  -H "X-Release-Token: ${RELEASE_ANNOUNCE_TOKEN}"

# 可选：admin 查询封号理由枚举（来源 global-config）
curl -X GET http://127.0.0.1:8080/api/admin/users/ban-reasons \
  -H "X-Release-Token: ${RELEASE_ANNOUNCE_TOKEN}"

# 可选：admin 封号（支持 userId 或 username；reason 需命中 global-config.admin.userBanReasons）
curl -X POST http://127.0.0.1:8080/api/admin/users/ban \
  -H "Content-Type: application/json" \
  -H "X-Release-Token: ${RELEASE_ANNOUNCE_TOKEN}" \
  -d '{
    "userId": 10001,
    "reason": "abnormal_trade",
    "isPermanent": false,
    "banUntil": "2026-12-31T23:59:59Z"
  }'

# 可选：admin 解封（支持 userId 或 username）
curl -X POST http://127.0.0.1:8080/api/admin/users/unban \
  -H "Content-Type: application/json" \
  -H "X-Release-Token: ${RELEASE_ANNOUNCE_TOKEN}" \
  -d '{
    "username": "player_10001"
  }'

# 可选：admin 手动触发猫舍随记（100% 触发）
curl -X POST http://127.0.0.1:8080/api/admin/cattery/trigger \
  -H "Content-Type: application/json" \
  -H "X-Release-Token: ${RELEASE_ANNOUNCE_TOKEN}" \
  -d '{
    "userId": 10001,
    "kittyId": 20001,
    "triggerType": "diary"
  }'

# 可选：admin 手动触发猫舍随遇（100% 触发；可指定目标猫）
curl -X POST http://127.0.0.1:8080/api/admin/cattery/trigger \
  -H "Content-Type: application/json" \
  -H "X-Release-Token: ${RELEASE_ANNOUNCE_TOKEN}" \
  -d '{
    "userId": 10001,
    "kittyId": 20001,
    "triggerType": "encounter",
    "targetUserId": 10002,
    "targetKittyId": 20077
  }'

# 可选：admin 手动清理 battle_runs（每次分批执行，重复调用直到 deleted=0）
curl -X POST http://127.0.0.1:8080/api/admin/battle/runs/cleanup \
  -H "Content-Type: application/json" \
  -H "X-Release-Token: ${RELEASE_ANNOUNCE_TOKEN}" \
  -d '{
    "keepCount": 10,
    "batchSize": 2000,
    "rounds": 20
  }'

# 可选：admin 查询 battle 慢战斗归档摘要（不跨机读 journalctl）
curl "http://127.0.0.1:8080/api/admin/battle/slow-runs?limit=20&roomId=1234" \
  -H "X-Release-Token: ${RELEASE_ANNOUNCE_TOKEN}"

# 可选：admin 查询某条慢战斗归档详情（返回 timeline / checkpoint / finalResult）
curl "http://127.0.0.1:8080/api/admin/battle/slow-runs/88" \
  -H "X-Release-Token: ${RELEASE_ANNOUNCE_TOKEN}"

# 可选：admin 手动清理 battle 慢战斗归档（默认建议 keepDays=30）
curl -X POST http://127.0.0.1:8080/api/admin/battle/slow-runs/cleanup \
  -H "Content-Type: application/json" \
  -H "X-Release-Token: ${RELEASE_ANNOUNCE_TOKEN}" \
  -d '{
    "keepDays": 30,
    "batchSize": 1000,
    "rounds": 20
  }'

# 可选：先确认 Server 已加载目标专精版本；Battle Worker 心跳的 specialization 也必须一致
curl "http://127.0.0.1:8080/api/admin/battle/metrics" \
  -H "X-Release-Token: ${RELEASE_ANNOUNCE_TOKEN}"

# 可选：admin 全量重整战斗房间（当前活跃 Run 标记为 interrupted，再进入 recovering）
curl -X POST http://127.0.0.1:8080/api/admin/battle/rooms/reinitialize \
  -H "Content-Type: application/json" \
  -H "X-Release-Token: ${RELEASE_ANNOUNCE_TOKEN}" \
  -d '{
    "statuses": ["entering", "playing", "recovering"],
    "expectedSpecialization": {
      "treeId": "player_combat_specialization",
      "rulesetVersion": "specialization-v6",
      "balanceRevision": 3
    }
  }'
```

`expectedSpecialization` 应填写 metrics 返回的 `runtime.specialization`。接口还会校验 `runtime.battleWorkers`：没有存活 Worker 心跳时返回 `503`，任一 Worker 缺少版本或与 Server 不一致时返回 `409`，均不会重置房间。先滚动更新所有 Battle Worker，确认 `runtime.battleWorkers.ready=true` 后再执行重置；自动重入房间会在恢复后从第 1 波采用新快照，未开启自动重入的房间回到 idle 等待玩家再次开始。

> 注意：battle 表存在循环外键拓扑，生产环境禁止使用 `TRUNCATE battle_runs CASCADE` / `TRUNCATE battle_rooms CASCADE` 等清理方式；统一走上面的 admin cleanup 分批删除。  
> 数据库已默认阻断 battle 域 TRUNCATE；仅在受控窗口下、且明确评估影响后，才可在单会话内临时执行：`SET app.allow_battle_truncate = 'on';`。

## 必要约束
- `server` / `action-worker` / `battle-worker` / `kitty-worker` 默认启用端点 preflight，禁止隐式 localhost 依赖。
- 生产必须替换 `JWT_SECRET`、数据库密码、证书路径。
- 回滚时优先用 systemd 级别回滚（上一个可用二进制与环境文件）。
- 只有 `server` 对外暴露 HTTP/WS；`action-worker`、`battle-worker`、`kitty-worker` 不进入 LB upstream。
- `NODE_ADVERTISE_ADDR` 必须是 server 内网可达地址（例如 `198.51.100.15:8080`），不要在跨机 LB 场景使用 `127.0.0.1:8080`。
- `web` 节点静态缓存策略：
  - `/assets/*` 返回 `Cache-Control: public, max-age=2592000, immutable`（配合 Vite hash 文件名长期缓存）。
  - `index.html` 返回 `Cache-Control: no-cache, must-revalidate`（保证发布后入口页及时更新并指向新 hash 资源）。
- `server` 如需启用“发布通知客户端刷新”，需配置：
  - `RELEASE_ANNOUNCE_TOKEN`：发布公告接口鉴权令牌（必填）。
  - `packages/game-config/source/global-config.json` 的 `release` 节点：
    - `release.buildId`：当前发布版本标识（用于 `/api/system/version`）。
    - `release.defaultReloadDelaySeconds`：默认刷新延时（默认 120 秒）。
  - 可选环境变量覆盖（仅用于紧急临时覆盖）：
    - `RELEASE_BUILD_ID`
    - `RELEASE_RELOAD_DELAY_SECONDS`
- 邮件验证码通道（阿里云 DM）：
  - 推荐：在 `server/email-aliyun.json` 放置阿里云 DirectMail 配置文件（仓库已忽略此文件）：
    ```json
    {
      "accessKeyId": "<阿里云 AK>",
      "accessKeySecret": "<阿里云 SK>",
      "accountName": "<已验证发信地址>",
      "regionId": "cn-hangzhou",
      "endpoint": "dm.aliyuncs.com",
      "fromAlias": "MySecondWebGame",
      "templateId": "<可选：模板ID>"
    }
    ```
  - 说明：`server` 读取该文件后按 DM `SingleSendMailWithOptions` 发送；若配置了 `templateId`，会按模板变量 `name/code/title/expiry_minutes` 渲染。未配置或配置错误会导致邮箱验证与忘记密码发送失败。
- 若 `lb` 在前置代理层终止流量，`server` 应配置真实 IP 信任：
  - `REAL_IP_TRUSTED_PROXY_CHECK=true`
  - `REAL_IP_TRUSTED_PROXIES` 填写 **LB 到 server 这跳源地址**（IP 或 CIDR，逗号分隔）
  - `REAL_IP_PROXY_HEADER` 一般为 `X-Forwarded-For`

## action-worker 部署参数建议
- 职责边界：
  - 负责：`aq:due_users` 到期用户领取、普通 action 到期结算、强化/建筑等 action queue 推进、写入待同步账本并发布内部 NATS 事件 `action.runtime.settled` 唤醒 server。
  - 不负责：玩家点击开始/取消/移动/立即执行、登录态鉴权、离线快进奖励弹窗、HTTP/WS 入口。
- 并发互斥：
  - action-worker 与 server 离线快进共用 Redis claim：`aq:claim:<userID>`。
  - 同一用户同一时刻只允许一个执行者结算，避免 worker 与登录触发的离线快进并发踩队列。
- 推荐基线（2C/4C 小规格均可起步）：
  - `ACTION_WORKER_DB_POOL_MAX_CONNS=4`
  - `ACTION_WORKER_BATCH_SIZE=8`
  - `ACTION_WORKER_CONCURRENCY=1`
  - `ACTION_WORKER_USER_TIMEOUT_MS=2000`
  - `ACTION_WORKER_POLL_MS=200`
  - `ACTION_WORKER_CLAIM_TTL_MS=5000`
  - `ACTION_WORKER_STUCK_EXIT_MS=120000`
  - `ACTION_WORKER_RETRY_BASE_MS=1000`
  - `ACTION_WORKER_RETRY_MAX_MS=30000`
  - `ACTION_WORKER_DEAD_AFTER_FAILURES=10`
  - `ACTION_WORKER_DEAD_RETRY_MS=300000`
  - `ACTION_WORKER_DEAD_RETRY_MAX_MS=3600000`
- systemd 服务：
  - unit：`mysecondwebgame-action-worker.service`
  - env：`/etc/mysecondwebgame/action-worker.env`
  - 二进制：`${APP_DIR}/bin/mysecondwebgame-action-worker`
  - 无监听端口；健康判断以 `systemctl is-active`、日志心跳与 Redis/NATS/DB 连通性为主。
- 与 server 同机时：
  - 推荐 action-worker DB pool 保持 `2-4`，不要和 server 抢满 DB 连接；默认串行结算不会主动使用多条并发连接。
  - server 节点建议 `SERVER_DB_POOL_MAX_CONNS=24 AUTH_DB_POOL_MAX_CONNS=4 ACTION_WORKER_DB_POOL_MAX_CONNS=4 KITTY_WORKER_DB_POOL_MAX_CONNS=4`。
  - `AUTH_DB_POOL_MAX_CONNS` 是 server 专用的鉴权保留池，默认 `4`；`AUTH_DB_TIMEOUT_MS` 默认 `800`，用于在鉴权池饱和时快速返回 `AUTH_BACKEND_BUSY`，避免请求排队几十秒。
  - PostgreSQL `max_connections` 需要覆盖 server main/auth、action-worker、battle-worker、kitty-worker 与运维连接余量；扩容业务池前先确认总连接预算。
- 线上首次启用：
  ```bash
  DB_HOST=192.0.2.10 PG_PORT=5432 DB_USER=game DB_PASSWORD='replace-me' DB_NAME=game \
  JWT_SECRET='replace-me' \
  REDIS_ADDR=192.0.2.20:6379 NATS_URL=nats://192.0.2.20:4222 \
  ACTION_WORKER_DB_POOL_MAX_CONNS=4 ACTION_WORKER_BATCH_SIZE=4 ACTION_WORKER_USER_TIMEOUT_MS=4000 \
  bash infra/scripts/deploy-linux-native-split.sh action-worker "${REPO_URL}" "${APP_DIR}"
  ```
  - `action-worker` 角色只会写入 `/etc/mysecondwebgame/action-worker.env`，不会重写 `/etc/mysecondwebgame/server.env`。
  - `server` 与 `server-with-kitty-worker` 角色会默认部署并启动 action-worker；开发阶段不再通过 `ACTION_WORKER_ENABLED` 之类的冗余开关控制。
- 线上无感更新 action-worker：
  ```bash
  bash infra/scripts/update-linux-native-split.sh action-worker /opt/moyu-idle
  ```
  - 若旧机器尚未注册 `mysecondwebgame-action-worker.service`，执行 `bash infra/scripts/update-linux-native-split.sh server /opt/moyu-idle` 会先自动补齐 action-worker service/env，再重启 server 与 action-worker，避免 server 已更新但 worker 单元不存在的半更新状态。
  - update 自愈不要求再次传入 `DB_HOST/REDIS_ADDR/NATS_URL`；若缺少 `/etc/mysecondwebgame/action-worker.env`，会从现有 `/etc/mysecondwebgame/server.env` 派生。每次更新都会把 action-worker `DATABASE_URL` 的 `pool_max_conns` 调整为 `ACTION_WORKER_DB_POOL_MAX_CONNS`（默认 4，可显式覆盖）。
- 运行时保护：
  - action-worker 领取用户后会先检查 `aq:offline_ff_ready:<userID>`；存在登录离线快进待弹窗时直接跳过并从 due 集合移除，保留给 server 登录链路懒推进。
  - action-worker 只处理有活跃 WS presence session 的用户；离线用户刷新/重连时仍由 server 的离线快进或 action queue 懒推进结算。
  - 每次推进后会根据剩余队列重新写入 `aq:due_users` 的下一次到期时间；没有可调度项时移除该用户。
  - 默认并发为 `1`，沿用串行结算，不增加数据库并发或默认路径的 goroutine/channel 开销；只有显式设置 `ACTION_WORKER_CONCURRENCY` 才会并行，代码上限为 `4`。
  - worker 结算路径只回读下一次调度所需的队列和结算摘要，不再重复读取完整玩家状态与属性；网关消费结算回执时仍会统一读取并推送权威快照。按当前读模型，这会减少每次 worker 结算后约 4-5 条重复 SQL；动作规格也在 worker 启动时构建一次，不再为每个到期玩家重复分配整张规格表。
  - 串行容量近似为 `1000 / 单次结算耗时毫秒数` 人/秒。扩并发前先观察到期积压与 PostgreSQL 锁等待；当前数据库存在用户级 advisory lock 竞争时，优先缩短单次事务，不建议直接提高并发。
  - 现有 30 秒健康日志会输出 `batch_users`、`batch_advanced`、`batch_duration_ms` 和 `batch_full`，不新增数据库或 Redis 探测。`batch_full=true` 持续出现且批次耗时不断升高，才说明串行 worker 已形成到期积压。
  - `aq:due_users` 是 Redis 派生索引，不是任务权威状态。action-worker 不新增 PostgreSQL 恢复扫描；玩家连接、读取队列或修改队列时，server 会用权威队列状态重建该玩家的调度项。worker 启动时只从 Redis 的 `aq:dead_users` 恢复最多 1000 个旧版遗留的永久熔断项，并使用 `ZADD NX`，不会覆盖正常调度、指数退避或熔断探测时间；遗留项会分散到最长探测窗口内，避免重启瞬间集中访问数据库。
  - action-worker 不直接发布前端 WS patch；它先写 Redis 待同步账本 `aq:sync_pending_users` / `aq:sync_pending_info:<pendingID>`，再发布内部 NATS 事件 `action.runtime.settled`。server 收到事件后统一推送队列、背包、属性、装备、玩家状态和提示事件，避免 worker 复制网关层状态同步逻辑。
  - NATS 事件只作为唤醒信号；若 server 重启或 NATS 短暂丢事件，server 会周期扫描待同步账本补偿推送。每条 pending record 会用 `aq:sync_claim:<pendingID>` 做短 TTL 领取，避免多台 server 重复推送强化结果/套装提示；账本在 server 完成一次状态快照推送后清理。
  - watchdog 区分“健康轮询”和“业务推进”：空队列不会被判定为卡死；只有连续轮询错误超过 `ACTION_WORKER_STUCK_EXIT_MS` 才主动退出交给 systemd 复活。
  - 单用户失败隔离：用户处理失败会写入 `aq:fail:<userID>` / `aq:fail_info:<userID>` 并按 `ACTION_WORKER_RETRY_BASE_MS` 到 `ACTION_WORKER_RETRY_MAX_MS` 指数退避；超过 `ACTION_WORKER_DEAD_AFTER_FAILURES` 后进入 `aq:dead_users` 和 `aq:dead_info:<userID>`，从 `ACTION_WORKER_DEAD_RETRY_MS` 起继续指数退避，最长不超过 `ACTION_WORKER_DEAD_RETRY_MAX_MS`。短暂数据库拥塞恢复后可自行闭合熔断，持续坏任务则会快速降到每小时一次探测，避免形成固定负载尖峰。
  - dead user 紧急手动恢复：修正数据后执行 `DEL aq:fail:<userID> aq:fail_info:<userID> aq:dead_info:<userID>`、`ZREM aq:dead_users <userID>`、`ZADD aq:due_users <now_ms> <userID>`；通常无需人工处理，成功推进会自动清理这些记录。
- endpoint 默认：
  - 分拆部署时，请显式配置 `DB_HOST`、`REDIS_ADDR` 和 `NATS_URL`。
  - 如需强制每次部署显式传入 endpoint，可设置 `SPLIT_REQUIRE_EXPLICIT_ENDPOINTS=1`；loopback endpoint 仍默认禁止，除非显式设置 `SPLIT_ALLOW_LOOPBACK_ENDPOINTS=1`。

## battle-worker 调度参数建议（250 活跃房间 / 4C8G）
- DB pool 建议从 `pool_max_conns=16` 起步，并根据 PostgreSQL 的总连接预算显式配置 `/etc/mysecondwebgame/battle-worker.env`。`update-linux-native-split.sh` 不会改写现有 `DATABASE_URL` 或连接池参数，避免普通代码更新意外挤占 server 的连接名额。4C8G PG 上不要直接把 battle-worker 拉到 40 这类高连接数，容易让结算阶段因为 DB 连接竞争和上下文切换变慢。
- 推荐基线（`packages/game-config/source/global-config.json`）：
  - `battle.tickShardWorkers=16`
  - `battle.tickClaimRoomsMin=32`
  - `battle.tickClaimRoomsMax=300`
  - `battle.tickTargetLeaseUtilPct=60`
  - `battle.timelinePushIntervalMs=1380`
- 观测门禁（发布后 10~30 分钟）：
  - `battle tick batch` 日志中的 `due_popped`、`due_claimed` 持续为 0 说明 Redis due 队列未生效；
  - `playbackDispatch.dispatchIntervalMs.p95` 目标不超过 `1.6 * timelinePushIntervalMs`；
  - `schedule_lateness_ms` 不应长期维持在 800ms+。

## battle 慢战斗排查
- battle-worker 仍保留关键 compile 慢日志，便于现场快速观察：
  - `battle worker config snapshot`
  - `battle run compile guard triggered`
  - `battle run compile slow`
  - `battle compile kickoff slow`
- 线上如果 `server` 与 `battle-worker` 不在同机，优先通过 DB 归档 + admin API 追溯，不再依赖跨机抓 `journalctl`。
- 慢战斗归档只覆盖 compile 相关异常，触发条件为：
  - `queue_since_start_ms >= battleCompileQueueSlowLogThresholdMs`
  - `compile_elapsed_ms >= battleCompileSlowLogThreshold`
  - `compiled.TimedOut == true`
  - `compiled.StepLimited == true`
- 归档追溯建议顺序：
  1. `GET /api/admin/battle/slow-runs` 先筛出目标 `roomId/runId`
  2. `GET /api/admin/battle/slow-runs/:archiveID` 取完整 `timeline/checkpoint/finalResult`
  3. 结合日志里的 `battle run compile slow` / `battle compile kickoff slow` 判断是排队慢、执行慢还是 guard 触发
- 如果 battle-worker 日志中出现：
  - `battle slow run archive upsert failed`
  - `battle slow run archive trim failed`
  说明 battle 本身仍会继续跑，但 admin 侧慢战斗追溯数据可能缺失或暂时超量，需要优先检查 DB 可用性。

## 验收检查
1. `systemctl status mysecondwebgame-server mysecondwebgame-action-worker mysecondwebgame-battle-worker mysecondwebgame-kitty-worker` 均为 active。
2. `curl -fsS http://127.0.0.1:8080/readyz` 在 server 节点返回成功；LB upstream healthcheck 使用 `/readyz`。
3. 仅 server 监听 `8080`；`action-worker` / `battle-worker` / `kitty-worker` 不应暴露 HTTP 端口。
4. LB 可访问 `/`、`/readyz`、`/ws`（升级握手正常）。
5. action 队列可用：开始动作、批量入队、取消、移动、立即执行、到期自动结算、刷新后懒推进。
6. 关键链路可用：登录、WS 绑定、战斗推进、跨实例广播。

## 历史文档
- 旧部署文档已迁移至：`docs/deployment/deprecated/`。
- 如需对比历史方案，仅用于回溯，不作为当前发布基线。
- 以下旧部署脚本文档路径已归档为过时：
  - `infra/scripts/deploy-linux-prod.sh`
  - `infra/scripts/deploy-linux-prod-split.sh`
  - `infra/docker-compose.prod.yml`
