# 后端单机内存关键状态盘点与改造计划

## 问题与目标
- 目标：识别后端业务中“关键状态依赖单进程内存”的节点，形成改造优先级与实施路线。
- 原则：**进程内存仅用于缓存/临时运行态**；关键状态（幂等、防重放、调度所有权、安全与一致性状态）统一外部化到 PostgreSQL/Redis。
- 当前执行范围（已确认）：**先改 P0 的第 1/2 项（防重放 + 限流/登录保护），战斗循环先不动**。

## 当前扫描结论（按风险分级）

### P0（优先改造，涉及正确性/安全）
1. **HTTP/WS 防重放兜底使用进程内存**
   - 位置：`server/internal/httpapi/server_http_base.go` `markReplay`
   - 现状：Redis 不可用时，回退 `replaySeen map[string]time.Time`。
   - 风险：多实例下去重不一致；重放防护退化为单实例有效。

2. **限流与登录失败锁定兜底使用进程内存**
   - 位置：`server/internal/httpapi/server_http_base.go` `ipRateLimiter` / `loginAttemptGuard`
   - 现状：Redis 异常时分别回退 `counters map` / `records map`。
   - 风险：多实例绕过限流/锁定，安全策略不一致。

3. **战斗循环是每实例本地 ticker，无全局所有权协调**
   - 位置：`server/internal/httpapi/battle_loop.go`、`battle_runtime_engine.go`
   - 现状：每实例周期扫描 fighting/paused 房间并推进；仅本地 `battleTickMu` 串行。
   - 风险：多实例可能并发推进同一房间（重复结算/重复事件推送窗口）。

### P1（中优先级，影响跨实例行为一致性）
1. **离线快进触发资格驻留进程内存**
   - 位置：`server/internal/httpapi/server.go` `offlineFFReady map[int64]int64`
   - 现状：登录/绑定时计算并写内存，消费后删除。
   - 风险：实例重启/切换后状态丢失，触发行为不稳定。

2. **WS 事件去重缓存驻留进程内存**
   - 位置：`server/internal/httpapi/ws_actions.go` `eventDedup map[string]string`
   - 现状：仅本实例去重。
   - 风险：跨实例无法去重，客户端可能看到重复事件。

3. **队列定时推送调度为本地 timer 状态**
   - 位置：`server/internal/httpapi/ws_actions.go` `actionPushStop map[int64]chan struct{}`
   - 现状：推送定时器与停止信号仅在本实例内存。
   - 风险：实例迁移后推送节奏波动（通常可被客户端轮询兜底）。

### P2（运行态内存，可保留但需明确“非关键”）
1. **WS Hub 连接表**
   - 位置：`server/internal/ws/hub.go` `clients map[*websocket.Conn]*Client`
   - 说明：连接生命周期天然本地化；关键状态应通过 Redis/DB 派生，不应作为权威真相。

2. **WS Patch 快照与版本缓存**
   - 位置：`server/internal/httpapi/ws_patch.go` `inventorySnap/equipmentSnap/attrSnap/patchVersion`
   - 说明：属推送优化缓存；可丢失但不应影响最终一致性。

3. **静态配置运行时缓存**
   - 位置：`server/internal/store/enhance_runtime_cache.go`、`server/internal/market/service.go`
   - 说明：来自 game-data 的只读缓存，不是用户关键状态。

4. **通用 Scheduler 内存队列（当前未承载关键业务）**
   - 位置：`server/internal/scheduler/scheduler.go`、`server/internal/game/service.go`
   - 说明：当前 `handleEvent` 为空，未成为关键链路；若未来承载关键任务需先外部化。

## 改造策略（分阶段）

### 阶段 A：先封堵“单机兜底”导致的一致性退化（P0，当前只做 1/2）
1. 防重放（HTTP + WS）
   - 去掉内存兜底路径：Redis 不可用时明确失败（返回可识别错误码）。
   - 保证多实例统一去重窗口。
2. 限流/登录保护
   - 去掉内存兜底；Redis 错误统一失败或按策略降级为“拒绝”而非“放行”。
   - 补充结构化日志字段（scope/user/ip/key）。
3. 战斗循环所有权（本轮不动）
   - 引入“房间推进分布式锁”或“实例所有权租约”（Redis SET NX EX）。
   - 锁粒度建议：`battle:tick:room:<roomId>`；短 TTL + 心跳续租。

### 阶段 B：把“会影响用户感知”的内存状态外部化（P1）
1. `offlineFFReady` 外部化到 Redis（带 TTL、一次消费语义）。
2. `eventDedup` 外部化到 Redis（短窗口 hash/指纹去重）。
3. 评估 `actionPushStop`：
   - 若保留本地 timer：明确“仅优化，不保证送达”，核心一致性依赖 DB+客户端主动拉取。
   - 若需强一致推送节奏：改为 Redis 延迟队列/stream 驱动。

### 阶段 C：制度化与防回归
1. 建立“关键状态白名单/黑名单”文档（哪些可内存、哪些必须外部化）。
2. 新增架构守卫：
   - 关键路径禁止 `redis nil -> memory fallback`。
   - PR checklist 增加“多实例一致性与幂等”检查项。
3. 补充压测/故障演练：
   - Redis 抖动、实例滚动重启、跨实例 WS 广播一致性。

## 待办清单（执行顺序）
1. 定义关键状态判定标准与改造优先级（P0/P1/P2）
2. 移除 replay/限流/登录保护的内存兜底并统一错误语义
3. （延期）实现战斗房间推进分布式锁与冲突保护
4. 外部化 offlineFFReady 与 eventDedup
5. 补齐文档与守卫（架构文档 + PR 检查项 + 观测指标）

## Redis 容量评估（针对本轮 P0-1/2）
- 结论：**2C4G 对本轮改造通常够用**（防重放 `SETNX+TTL`、限流 `INCR+EXPIRE/Lua`、登录失败计数与锁键）。
- 前提：在线规模在中小量级，2 分钟窗口内幂等键和限流键数量不会爆炸增长。
- 风险边界：若短时间写入峰值很高（例如瞬时大量 mutating 请求），Redis CPU 可能先到瓶颈。
- 本轮需同步加观测：键基数（`replay:*`、`ratelimit:*`）、内存使用、OPS、慢日志；建议 `maxmemory-policy noeviction`。

## 备注
- 本次仅完成盘点与计划，不改业务代码。
- 后续建议按“P0 → P1 → P2”逐步落地，每阶段独立可回滚。
