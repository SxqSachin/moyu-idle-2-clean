# 服务器部署方案（前后端）

## 架构
- `web`：前端静态资源构建与静态站点服务（不反代 `/api` 与 `/ws`）
- `server`：Go Fiber + WebSocket 网关服务（HTTP/WS 接入与转发）
- `battle-worker`：独立战斗推进服务（不提供 HTTP/WS）
- `pg`：PostgreSQL 事务库
- `redis`：在线状态/调度索引/聊天通道
- `nats`：异步事件总线（削峰、解耦、重试入口）

## 单机起步部署（可扩展）
```bash
cd infra
docker compose -f docker-compose.prod.yml up -d --build
```

## Linux 快速部署脚本（推荐）
```bash
bash infra/scripts/deploy-linux-prod.sh <git_repo_url> [app_dir]
```
或（已在仓库目录内）：
```bash
bash infra/scripts/deploy-linux-prod.sh
```

按角色拆分部署（含 battle-worker）：
```bash
bash infra/scripts/deploy-linux-prod-split.sh battle-worker
```

## Linux 原生（非 Docker）单机全套部署脚本
```bash
bash infra/scripts/deploy-linux-native-all.sh <git_repo_url> [app_dir]
```
- 目标：`Nginx + server + battle-worker + PostgreSQL + Redis + NATS` 全部以 systemd 原生服务部署。
- 适用：Debian/Ubuntu（`apt + systemd`）。

原生按角色拆分部署：
```bash
bash infra/scripts/deploy-linux-native-split.sh <role> [git_repo_url] [app_dir]
```
- `role`: `pg | redis | nats | server | battle-worker | web | lb | all`

### 原生 split 脚本参数说明（按 role）

> 脚本通过**环境变量**传参，不读取仓库内 `.env`。  
> 后端最终环境会写入：`/etc/mysecondwebgame/server.env`、`/etc/mysecondwebgame/battle-worker.env`。

#### 公共可选参数（大多数 role 可用）
- `BRANCH`：git 分支（默认 `main`）
- `INSTALL_DEPS`：是否自动 `apt install`（`1`=安装，`0`=跳过）
- `APP_USER` / `APP_GROUP`：systemd 运行用户组（默认 `mysecondwebgame`）
- `APP_PORT`：网关端口（默认 `8080`）
- `APP_DIR`：第三个位置参数（默认 `/opt/mysecondwebgame`）

#### 基础设施 role
1. `pg`
   - 必填建议：`DB_USER`、`DB_PASSWORD`、`DB_NAME`
   - 可选：`PG_PORT`（默认 `5432`）
2. `redis`
   - 无必填业务参数（默认安装并启动系统 `redis-server`）
3. `nats`
   - 无必填业务参数（默认安装并启动系统 `nats-server`）

#### 应用 role
1. `server`
   - 必填建议：`DB_PASSWORD`、`JWT_SECRET`
   - 可选常用：`DB_HOST`、`DB_USER`、`DB_NAME`、`PG_PORT`、`REDIS_ADDR`、`NATS_URL`、`MQ_TOPIC_PREFIX`
   - **split 多机部署时必须显式设置**：`DB_HOST`、`REDIS_ADDR`、`NATS_URL`（不能依赖 localhost 默认值）
   - 不在服务器本地构建 shared game-data；要求代码仓库已包含 `packages/game-data/generated/` 产物
2. `battle-worker`
   - 必填建议：`DB_PASSWORD`、`JWT_SECRET`
   - 可选常用：同 `server`（同样要求 `packages/game-data/generated/` 已随代码提交）
   - **split 多机部署时必须显式设置**：`DB_HOST`、`REDIS_ADDR`、`NATS_URL`
3. `web`
   - 无必填业务参数（会执行 `npm ci && npm run build`）
   - 要求 Node `v22.14+`（严格检查）
   - 默认会自动准备 Node 22 运行时（依赖 `INSTALL_DEPS=1`）
   - 该 role 会部署本机 Nginx 仅托管静态资源，不反代 `/api` `/ws`
   - 默认启用 gzip 压缩（文本/JS/CSS/SVG/JSON）
4. `lb`
   - 前置负载均衡入口，默认使用 `moyu-idle.com` / `www.moyu-idle.com`
   - 必填：`LB_WEB_UPSTREAMS`、`LB_API_UPSTREAMS`
   - 可选：`LB_WS_UPSTREAMS`、`LB_DOMAIN`、`LB_WWW_DOMAIN`、`LB_CERT_FULLCHAIN_PATH`、`LB_CERT_PRIVKEY_PATH`

`web` 角色是应用侧静态站点；`lb` 是对外的前置负载均衡入口机。

#### 一次性全量 role
1. `all`
   - 建议至少传：`DB_PASSWORD`、`JWT_SECRET`
   - 其余参数按上面默认值可先跑通

#### 全部服务部署脚本用例（split，按 role）
```bash
ssh-keygen -t ed25519 -C "deploy-key-your-project"
cat .ssh/id_ed25519.pub

# 约定：首次部署统一拉代码
REPO_URL=https://github.com/OWNER/moyu-idle-2-clean.git
APP_DIR=/opt/moyu-idle

# 注意：以下命令分别在各自角色机器执行（不是同一台）

# 1) PostgreSQL 机器
DB_USER=game DB_PASSWORD='replace-me' DB_NAME=game \
bash deploy-linux-native-split.sh pg "${REPO_URL}" "${APP_DIR}"

# 2) Redis 机器
bash deploy-linux-native-split.sh redis "${REPO_URL}" "${APP_DIR}"

# 3) NATS 机器
bash deploy-linux-native-split.sh nats "${REPO_URL}" "${APP_DIR}"

# 4) API/WS 网关机器（server）
# 可先做连通性检查（在 server 机器执行）
nc -vz 192.0.2.10 5432 && nc -vz 192.0.2.20 6379 && nc -vz 192.0.2.30 4222

DB_HOST=192.0.2.10 PG_PORT=5432 DB_USER=game DB_PASSWORD='replace-me' DB_NAME=game \
JWT_SECRET='replace-me' APP_PORT=8080 \
REDIS_ADDR=192.0.2.20:6379 NATS_URL=nats://192.0.2.30:4222 \
bash deploy-linux-native-split.sh server "${REPO_URL}" "${APP_DIR}"

# 5) battle-worker 机器
# 可先做连通性检查（在 battle-worker 机器执行）
nc -vz 192.0.2.10 5432 && nc -vz 192.0.2.20 6379 && nc -vz 192.0.2.30 4222

DB_HOST=192.0.2.10 PG_PORT=5432 DB_USER=game DB_PASSWORD='replace-me' DB_NAME=game \
JWT_SECRET='replace-me' \
REDIS_ADDR=192.0.2.20:6379 NATS_URL=nats://192.0.2.30:4222 \
bash deploy-linux-native-split.sh battle-worker "${REPO_URL}" "${APP_DIR}"

# 6) web（构建前端静态资源）
# 默认会自动安装 Node 22 运行时；也可手动预装后设置 INSTALL_DEPS=0
bash infra/scripts/deploy-linux-native-split.sh web "${REPO_URL}" "${APP_DIR}"

# 7) lb（前置负载均衡入口，独立前置机）
LB_DOMAIN=moyu-idle.com \
LB_WWW_DOMAIN=www.moyu-idle.com \
LB_CERT_FULLCHAIN_PATH=/etc/letsencrypt/live/moyu-idle.com/fullchain.pem \
LB_CERT_PRIVKEY_PATH=/etc/letsencrypt/live/moyu-idle.com/privkey.pem \
LB_WEB_UPSTREAMS=192.0.2.31:80 \
LB_API_UPSTREAMS=198.51.100.15:8080 \
LB_WS_UPSTREAMS=198.51.100.15:8080 \
bash deploy-linux-native-lb.sh
```

#### 全量一键用例（split all）
```bash
DB_USER=game DB_PASSWORD='replace-me' DB_NAME=game \
JWT_SECRET='replace-me' APP_PORT=8080 \
REDIS_ADDR=127.0.0.1:6379 NATS_URL=nats://127.0.0.1:4222 \
bash infra/scripts/deploy-linux-native-split.sh all https://github.com/<owner>/<repo>.git /opt/mysecondwebgame
```

#### 单机全套脚本用例（native-all）
```bash
DB_USER=game DB_PASSWORD='replace-me' DB_NAME=game \
JWT_SECRET='replace-me' APP_PORT=8080 \
REDIS_ADDR=127.0.0.1:6379 NATS_URL=nats://127.0.0.1:4222 \
bash infra/scripts/deploy-linux-native-all.sh https://github.com/<owner>/<repo>.git /opt/mysecondwebgame
```

`native-all` 也会在 web 构建前自动准备 Node 22 运行时，并严格校验 `v22.14+`。

### split 多机部署端点 preflight（新增）
- `server` / `battle-worker` 默认开启端点 preflight，会在构建前检查 PostgreSQL、Redis、NATS 可达性。
- 默认要求显式设置：`DB_HOST`、`REDIS_ADDR`、`NATS_URL`（`SPLIT_REQUIRE_EXPLICIT_ENDPOINTS=1`）。
- 默认禁止 loopback 端点（`127.0.0.1` / `localhost` / `::1`）（`SPLIT_ALLOW_LOOPBACK_ENDPOINTS=0`）。
- 如确需本机依赖，可显式覆盖：
  - `SPLIT_REQUIRE_EXPLICIT_ENDPOINTS=0`
  - `SPLIT_ALLOW_LOOPBACK_ENDPOINTS=1`

常见报错排查：
```bash
# 1) 检查部署命令注入的环境变量
env | grep -E '^(DB_HOST|PG_PORT|REDIS_ADDR|NATS_URL)='

# 2) 检查端口连通性（示例）
nc -vz "$DB_HOST" "${PG_PORT:-5432}"
nc -vz "${REDIS_ADDR%:*}" "${REDIS_ADDR##*:}"
NATS_ENDPOINT="${NATS_URL#nats://}"; NATS_ENDPOINT="${NATS_ENDPOINT%%,*}"
nc -vz "${NATS_ENDPOINT%:*}" "${NATS_ENDPOINT##*:}"

# 3) PostgreSQL 节点上确认监听与访问控制（示例）
sudo ss -lntp | grep ':5432'
sudo -u postgres psql -tAc "SHOW listen_addresses;"
sudo -u postgres psql -tAc "SHOW port;"
```

若报错 `PostgreSQL is unreachable at <ip>:5432`：
1. 先确认部署命令传的是 `PG_PORT`（不是 `DB_PORT`）。
2. 确认 PostgreSQL 实际监听了该 IP/端口（不是只监听 127.0.0.1）。
3. 确认云安全组/防火墙放通了 server 或 battle-worker 机器到 PostgreSQL 节点的 5432。
4. 若 `pg_hba.conf` 只有 `127.0.0.1/32` 和 `::1/128`，远端机器一定会被拒绝；必须新增应用网段规则。

PostgreSQL 节点最小修复清单（Ubuntu/Debian）：
```bash
# 1) postgresql.conf: 放开监听
sudo sed -i "s/^#\\?listen_addresses.*/listen_addresses = '*'/" /etc/postgresql/*/main/postgresql.conf

# 2) pg_hba.conf: 放行应用子网（示例 198.51.100.0/16）
echo "host    all    all    198.51.100.0/16    md5" | sudo tee -a /etc/postgresql/*/main/pg_hba.conf

# 3) 重启并复查
sudo systemctl restart postgresql
sudo ss -lntp | grep ':5432'
sudo -u postgres psql -tAc "SHOW listen_addresses;"
```

若报错 `Redis is unreachable at <ip>:6379`：
1. 确认 Redis 监听地址不是仅 `127.0.0.1`（`bind` 配置）。
2. 确认 `protected-mode` 与 ACL/密码配置允许来自应用网段访问。
3. 确认云安全组/防火墙放通 6379。
4. 当前 `native-split` / `native-all` 的 Redis 角色会临时关闭 `protected-mode` 并放开监听，以便 split 部署先跑通；上线前建议改回密码/ACL 方案。

Redis 节点最小修复清单（Ubuntu/Debian）：
```bash
# 1) 放开监听（按需改为更小网段网卡 IP）
sudo sed -i "s/^bind .*/bind 0.0.0.0 ::0/" /etc/redis/redis.conf

# 2) 临时关闭 protected-mode（仅用于受控内网）
sudo sed -i "s/^protected-mode .*/protected-mode no/" /etc/redis/redis.conf

# 3) 生产环境建议保留 protected-mode，并至少配置 requirepass / ACL
# 若你已配置密码，部署端要同时设置 REDIS_PASSWORD

# 4) 重启并复查
sudo systemctl restart redis-server
sudo ss -lntp | grep ':6379'
```

若报错 `NATS is unreachable at <ip>:4222`：
1. 确认 NATS client 监听地址不是仅 loopback。
2. 确认安全组/防火墙放通 4222（如启用监控端口也检查 8222）。

NATS 节点最小修复清单（Ubuntu/Debian）：
```bash
# 1) 写入最小远程监听配置
sudo tee /etc/nats/nats-server.conf >/dev/null <<'EOF'
listen: 0.0.0.0:4222
http: 0.0.0.0:8222
EOF

# 2) 重启并复查
sudo systemctl restart nats-server
sudo ss -lntp | grep ':4222'
```

### shared game-data 产物提交流程（server / battle-worker 必需）
1. 在构建环境执行：
   ```bash
   node packages/game-data/scripts/build.mjs
   ```
2. 提交完整目录：
   - `packages/game-data/generated/`
3. 提交前强校验：
   - 必须至少执行一次上述 build 命令（不强制 `generated` 目录 diff 为空）。

若部署时报错 `Missing required shared game-data artifact`，说明仓库缺少生成产物；请先执行 build 并把 `packages/game-data/generated/` 提交后再部署。

#### LB 角色（前置负载均衡入口）
```bash
LB_DOMAIN=moyu-idle.com \
LB_WWW_DOMAIN=www.moyu-idle.com \
LB_CERT_FULLCHAIN_PATH=/etc/letsencrypt/live/moyu-idle.com/fullchain.pem \
LB_CERT_PRIVKEY_PATH=/etc/letsencrypt/live/moyu-idle.com/privkey.pem \
LB_WEB_UPSTREAMS=10.0.0.11:80,10.0.0.12:80 \
LB_API_UPSTREAMS=10.0.0.21:8080,10.0.0.22:8080 \
LB_WS_UPSTREAMS=10.0.0.21:8080,10.0.0.22:8080 \
bash infra/scripts/deploy-linux-native-lb.sh
```

certbot certonly --preferred-challenges dns --manual  -d *.moyu-idle.com --server https://acme-v02.api.letsencrypt.org/directory


无证书纯 HTTP：
```bash
LB_ENABLE_TLS=0 \
LB_DOMAIN=moyu-idle.com \
LB_WWW_DOMAIN=www.moyu-idle.com \
LB_WEB_UPSTREAMS=192.0.2.31:80 \
LB_API_UPSTREAMS=198.51.100.15:8080 \
LB_WS_UPSTREAMS=198.51.100.15:8080 \
bash infra/scripts/deploy-linux-native-lb.sh

LB_ENABLE_TLS=0 \
LB_DOMAIN=moyu-idle.com \
LB_WWW_DOMAIN=www.moyu-idle.com \
LB_WEB_UPSTREAMS=192.0.2.31:80 \
LB_API_UPSTREAMS=198.51.100.15:8080 \
LB_WS_UPSTREAMS=198.51.100.15:8080 \
bash dv.sh lb
```

该角色会在前置服务器上部署 Nginx，并把：

- `https://moyu-idle.com` / `http://moyu-idle.com` 重定向到 `https://www.moyu-idle.com`（TLS 模式）
- `http://moyu-idle.com` / `http://www.moyu-idle.com` 可在 `LB_ENABLE_TLS=0` 时直接提供纯 HTTP
- `www` 域名的页面请求转发到 `LB_WEB_UPSTREAMS`
- `/api/` 转发到 `LB_API_UPSTREAMS`
- `/ws` 转发到 `LB_WS_UPSTREAMS`（默认复用 `LB_API_UPSTREAMS`）
- 三类 upstream 都按 `remote_addr` 做 consistent hash，属于纯 IP sticky

> 注意：`web` role 本机 Nginx 仅提供静态资源，不处理 `/api` 或 `/ws` 反代；这些统一由 `lb` role 转发。

使用的证书路径和域名配置如下：

```nginx
# 1) 裸域名 HTTP/HTTPS 重定向到带 www 的 HTTPS
server {
    listen 80;
    listen 443 ssl;
    server_name moyu-idle.com;

    ssl_certificate /etc/letsencrypt/live/moyu-idle.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/moyu-idle.com/privkey.pem;

    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers HIGH:!aNULL:!MD5;

    return 301 https://www.moyu-idle.com$request_uri;
}

# 2) 带 www 的 HTTP 请求重定向到 HTTPS
server {
    listen 80;
    server_name www.moyu-idle.com;

    return 301 https://www.moyu-idle.com$request_uri;
}

# 3) 带 www 的 HTTPS 正常服务（前端、API、WS 都在这里转发）
server {
    listen 443 ssl;
    server_name www.moyu-idle.com;

    ssl_certificate /etc/letsencrypt/live/moyu-idle.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/moyu-idle.com/privkey.pem;

    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers HIGH:!aNULL:!MD5;

    # 页面请求转发到 web 节点池
    location / {
        proxy_pass http://mysecondwebgame_lb_web;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    location /assets/ {
        proxy_pass http://mysecondwebgame_lb_web;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        expires 30d;
        add_header Cache-Control "public, max-age=2592000, immutable";
    }

    location /api/ {
        proxy_pass http://mysecondwebgame_lb_api;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    location /ws {
        proxy_pass http://mysecondwebgame_lb_ws;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection $connection_upgrade;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

## 镜像构建脚本（网关 + battle-worker + web）
```bash
bash infra/scripts/build-images.sh
```

## 生产建议
1. JWT_SECRET、数据库密码改为强随机并通过密钥管理注入。
2. 前端挂在 CDN；Nginx/ALB 前加 HTTPS 证书（LetsEncrypt 或云厂商证书）。
3. server（网关）与 battle-worker 可分别横向扩容；Redis 开启持久化；NATS 建议开启 JetStream；PostgreSQL 做主从或托管服务。
4. 指标与日志接入（Prometheus/Grafana + Loki/ELK）。
5. 多实例 Nginx 参考配置：`infra/nginx/multi-instance.conf`（API 负载均衡 + WS 一致性哈希）。
6. battle-worker 存活检查可基于 Redis 心跳键：`<MQ_TOPIC_PREFIX>:battle:worker:heartbeat:<worker_id>`（TTL 由 `battle.workerHeartbeat.ttlSeconds` 控制）。

## 纯自建 Nginx 的后端自动扩容（Redis 注册 + upstream 同步）
1. 每个后端实例设置：
   - `NODE_ADVERTISE_ADDR=<内网IP:PORT>`
   - `NODE_REGISTRY_PREFIX=ws:backend:node:`
   - `NODE_HEARTBEAT_SECONDS=30`
   - `NODE_HEARTBEAT_INTERVAL_SECONDS=10`
   - `WS_TARGETED_ROUTING_ENABLED=true`（启用后用户定向消息按节点分流）
2. Nginx 使用 `infra/nginx/multi-instance.conf`，并确保 include 文件路径存在：
   - `/etc/nginx/conf.d/upstreams/game_backend_api.nodes.conf`
   - `/etc/nginx/conf.d/upstreams/game_backend_ws.nodes.conf`
3. 在 Nginx 机器运行同步脚本：
   ```bash
   REDIS_ADDR=127.0.0.1:6379 \
   NODE_KEY_PREFIX=ws:backend:node: \
   SYNC_INTERVAL_SECONDS=15 \
   HEALTHCHECK_ENABLED=true \
   HEALTHCHECK_PATH=/healthz \
   NGINX_RELOAD_CMD='docker exec game-nginx-lb nginx -t && docker exec game-nginx-lb nginx -s reload' \
   bash infra/scripts/sync-nginx-upstreams.sh
   ```
4. 自检：
   - Redis 中存在 `ws:backend:node:<ip:port>` 活跃 key；
   - 扩容后 15 秒内 upstream 文件出现新节点；
   - `nginx -t` 通过并完成平滑 reload。

## WS 广播强约束
1. NATS 是跨实例 WS 广播必选项，后端启动时若 NATS 不可用将直接失败。
2. WS 发送采用单连接单写协程 + 有界队列（64），慢连接满队列时会被断开以保护整体服务稳态。
3. `target_users` 查不到路由时重试 3 次后丢弃并告警（日志 + 指标），不会回退为全局广播。
4. WS 一致性哈希键顺序：`arg_token` > `Authorization` > `remote_addr`，多 LB 节点需保持相同配置。

## Action 快进观测建议（新增）
1. 单次快进完成步数（按 queue 维度）与总耗时。
2. 快进聚合写批次数与失败率（库存扣减、属性更新、队列状态更新）。
3. `action.queue.run_now` 成功率与“请求到进入 running”的延迟分布。

## 发布流程
1. 构建 shared game-data 并发布 `data_version`。
2. 后端发布前先跑数据库迁移。
3. 先发布 battle-worker，再灰度 server（网关）实例，观察战斗推进与 WS 事件转发后全量。
4. 最后发布前端与 Electron 客户端。

## 战斗链路冒烟（发布后）
1. 触发一局多场战斗，确认每场使用独立 run 记录并按 `timelinePushIntervalMs` 回放。
2. 确认每场胜利后即时结算奖励，场间按 `interBattleIntervalMs` 再进入下一场。
3. 确认仅首场（wave=1）抓取实时玩家快照；后续场次沿用上一场结算状态，不受外部临时加点/换装影响。
4. 确认失败时房间状态回收正确，且此前胜利场次收益不回滚。
5. 确认历史 run 清理生效：仅最近 `battle.runHistoryKeepCount` 场保留完整 timeline，旧 run 已降级摘要。
6. 确认 worker 以 `next_tick_at` 驱动推进（非固定轮询），但回放节拍仍由 `timelinePushIntervalMs` 决定。

## 参考
- 线上配置与扩容建议：`docs/ops/infra-sizing-and-scale.md`
