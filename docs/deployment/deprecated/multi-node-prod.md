# 多机生产部署（前后端与中间件分离）

## 目标
- `pg`、`redis`、`nats`、`server`、`web`、`nginx` 可分别部署到不同机器。
- 每个服务端口在内网可访问。
- 支持多前端、多后端负载均衡。

## 新脚本
- `infra/scripts/deploy-linux-prod-split.sh`

用法：
```bash
bash infra/scripts/deploy-linux-prod-split.sh <role> [repo_url] [app_dir] [env_file]
```
`role` 可选：`pg | redis | nats | server | web | nginx | all`

## 环境变量
复制并修改：
```bash
cp infra/.env.prod.example infra/.env.prod
```

重点字段：
- `DATABASE_URL`
- `REDIS_ADDR` / `REDIS_PASSWORD`
- `NATS_URL`
- `NODE_ADVERTISE_ADDR`（后端机器自己的内网 `ip:8080`）
- `JWT_SECRET`

## 生产数据库初始化

多机 split 部署里，`pg` 角色只负责把 PostgreSQL 容器/服务拉起来；**schema 初始化要单独执行**。

推荐顺序：
```bash
# 1) 先在 PG 机器启动数据库服务
bash infra/scripts/deploy-linux-prod-split.sh pg <repo_url> /opt/MySecondWebGame infra/.env.prod

# 2) 再在同一台 PG 机器执行基线 schema
bash infra/scripts/apply-schema-sql.sh init.sql infra/compose/pg.yml infra/.env.prod
```

- 现在 `deploy-linux-prod-split.sh pg` 会在启动 PG 后自动执行第 2 步。
- 如果你不在 PG 机器上执行，也可以在任意一台能连通数据库的运维机上运行：
  ```bash
  PGHOST=<pg_host> PGADMIN_PASSWORD='replace-me' PGAPP_PASSWORD='replace-me' \
    bash infra/scripts/init-prod-pg-remote.sh
  ```
- 初始化真值来源始终是 `server/schema/init.sql`。
- 该脚本会幂等地创建数据库，并把 schema 应用到远端 PG。
- 后续 `server` / `battle-worker` 只通过 `DATABASE_URL` 连接，不再负责建库。

## 端口规划（内网）
- PostgreSQL: `5432`
- Redis: `6379`
- NATS: `4222`（监控 `8222`）
- Backend: `8080`
- Frontend: `80`
- Nginx LB: `80/443`

请在云厂商安全组/防火墙仅放通内网来源，不要对公网裸露数据库与缓存端口。

## 每台机器部署命令示例
在对应机器执行（先拉代码）：

```bash
# PG 机器
bash infra/scripts/deploy-linux-prod-split.sh pg <repo_url> /opt/MySecondWebGame infra/.env.prod

# Redis 机器
bash infra/scripts/deploy-linux-prod-split.sh redis <repo_url> /opt/MySecondWebGame infra/.env.prod

# NATS 机器
bash infra/scripts/deploy-linux-prod-split.sh nats <repo_url> /opt/MySecondWebGame infra/.env.prod

# Backend 机器（每台都执行）
bash infra/scripts/deploy-linux-prod-split.sh server <repo_url> /opt/MySecondWebGame infra/.env.prod

# Frontend 机器（每台都执行）
bash infra/scripts/deploy-linux-prod-split.sh web <repo_url> /opt/MySecondWebGame infra/.env.prod

# Nginx LB 机器
bash infra/scripts/deploy-linux-prod-split.sh nginx <repo_url> /opt/MySecondWebGame infra/.env.prod
```

## Nginx 配置
使用：`infra/nginx/multi-tier-lb.conf`

upstream 文件：
- `infra/nginx/upstreams/game_frontend.nodes.conf`
- `infra/nginx/upstreams/game_backend_api.nodes.conf`
- `infra/nginx/upstreams/game_backend_ws.nodes.conf`
- WS sticky key 规则：`arg_token` > `Authorization` > `remote_addr`（所有 LB 节点必须一致）

你有 2 个前端 + 2 个后端时，填成：
```nginx
# game_frontend.nodes.conf
server 198.51.100.11:80 max_fails=3 fail_timeout=10s;
server 198.51.100.12:80 max_fails=3 fail_timeout=10s;

# game_backend_api.nodes.conf
server 203.0.113.21:8080 max_fails=3 fail_timeout=10s;
server 203.0.113.22:8080 max_fails=3 fail_timeout=10s;

# game_backend_ws.nodes.conf
server 203.0.113.21:8080 max_fails=3 fail_timeout=10s;
server 203.0.113.22:8080 max_fails=3 fail_timeout=10s;
```

## Nginx 应该放哪台机器
推荐两种：
1. 单独一台 `ingress`（最推荐）：专职反向代理和证书终止，后续扩容最稳。
2. 先临时放在某台前端机：能用，但会和前端资源竞争，不建议长期。

生产建议：至少 2 台 Nginx + DNS/SLB 做高可用。

## Nginx upstream 自动同步（推荐）
在每台 Nginx 机器运行：
```bash
REDIS_ADDR=192.0.2.32:6379 \
NODE_KEY_PREFIX=ws:backend:node: \
SYNC_INTERVAL_SECONDS=15 \
HEALTHCHECK_ENABLED=true \
HEALTHCHECK_PATH=/healthz \
NGINX_RELOAD_CMD='docker exec game-nginx-lb nginx -t && docker exec game-nginx-lb nginx -s reload' \
bash infra/scripts/sync-nginx-upstreams.sh
```
