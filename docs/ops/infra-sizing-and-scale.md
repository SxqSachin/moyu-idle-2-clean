# 线上预期配置与横向扩展

## 推荐配置
| 阶段 | 应用节点 | 数据库 | Redis | MQ | 说明 |
|---|---|---|---|---|---|
| MVP/内测 | 1 x 2C4G | 单实例 PG 2C4G | 单实例 1C2G | NATS 单实例 | 快速验证功能 |
| 1k-5k 在线 | 2 x 4C8G | PG 4C8G（托管优先） | Redis 2C4G | NATS + JetStream | 应用多副本 + LB |
| 5k-20k 在线 | 3-5 x 8C16G | PG 主从或高可用 | Redis 哨兵/集群 | NATS 集群 | 读写分离 + 监控完善 |
| 20k+ 在线 | 按压测扩容 | 分库/分片规划 | Redis Cluster | 分区主题 + 消费组 | 异步链路独立扩展 |

## 快速横向扩展路径
1. 应用层无状态化，`server` 多副本，负载均衡分发 HTTP/WS。
2. WS 跨节点广播使用消息总线（当前实现为 NATS 广播 `*.ws.broadcast` 主题）。
3. 调度器按玩家分片：提升 `SCHEDULER_SHARDS`，保持同玩家串行。
4. 数据层先做连接池和索引优化，再做读写分离/分片。
5. 市场交易保持数据库事务与幂等键，禁止跨节点内存态扣减。

## 压测与扩容触发建议
- P95 > 120ms 持续 5 分钟：先扩应用副本。
- DB CPU > 70% 持续 10 分钟：优化 SQL/索引，必要时读写分离。
- WS 在线数接近单机上限：扩容网关副本并启用连接均衡。

## 建议新增观测项（库存与限流）
- Redis 限流键增长速率、被限流请求占比、`Retry-After` 分布。
- 库存写事务冲突率（版本冲突、锁等待）、关键 SQL P95/P99。
- 仓库写路径失败码分布（冲突、权限、库存不足）与重试成功率。

## 服务器部署方案（纯自建 Nginx + 多后端，明确版）

### 1) 角色分层
1. **入口层（Nginx 节点）**：承载前端静态资源，反代 `/api` 与 `/ws`。
2. **应用层（server 节点）**：运行 Go 服务，多实例部署。
3. **基础层（共享）**：PostgreSQL、Redis、NATS（建议独立部署）。

### 2) 后端自动扩容机制
1. 每个后端实例启动时用 Redis 注册存活节点：`ws:backend:node:<ip:port>`（TTL 心跳续期）。
2. Nginx 节点运行 `infra/scripts/sync-nginx-upstreams.sh`，每 15 秒拉取活跃节点：
   - 生成 API upstream 列表（`least_conn`）；
   - 生成 WS upstream 列表（`hash $ws_sticky_key consistent`，粘性键优先取 `arg_token`，其次 `Authorization`，最后 `remote_addr`）；
   - 健康检查通过（`/healthz`）后入池；
   - 仅在节点变化时 `nginx -s reload`。
3. 后端扩容时只需新增 server 实例并配置 `NODE_ADVERTISE_ADDR`，无需手工改 Nginx upstream。

### 3) 必选配置（后端）
- `NODE_ADVERTISE_ADDR=<内网IP:PORT>`
- `NODE_REGISTRY_PREFIX=ws:backend:node:`
- `NODE_HEARTBEAT_SECONDS=30`
- `NODE_HEARTBEAT_INTERVAL_SECONDS=10`
- `NATS_URL=<可用地址>`（不可用时后端启动失败）

### 4) 必选配置（Nginx）
- 使用 `infra/nginx/multi-instance.conf` 并启用 include：
  - `/etc/nginx/conf.d/upstreams/game_backend_api.nodes.conf`
  - `/etc/nginx/conf.d/upstreams/game_backend_ws.nodes.conf`
- 启动同步脚本（建议 systemd 守护）：
  ```bash
  REDIS_ADDR=127.0.0.1:6379 \
  NODE_KEY_PREFIX=ws:backend:node: \
  SYNC_INTERVAL_SECONDS=15 \
  HEALTHCHECK_ENABLED=true \
  HEALTHCHECK_PATH=/healthz \
  NGINX_RELOAD_CMD='docker exec game-nginx-lb nginx -t && docker exec game-nginx-lb nginx -s reload' \
  bash infra/scripts/sync-nginx-upstreams.sh
  ```

### 5) 可靠性强约束
1. WS 发送模型必须是“单连接单写协程 + 有界队列（64）”，慢连接满队列断开。
2. NATS 连接失败禁止静默降级，服务应启动失败并告警。
3. Redis/NATS/PostgreSQL 必须是所有后端实例共享的同一套服务。

### 6) 上线验收清单
1. 新增后端实例后 15 秒内出现在 Nginx upstream 文件中。
2. 扩容/缩容时，现有连接可重连恢复，跨实例广播可达。
3. 任一后端下线不影响其余实例继续处理 `/api` 与 `/ws`。
