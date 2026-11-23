# 后端代码结构梳理（按当前代码实际）

## TL;DR
- 你说的判断是对的：**battle 运行时核心目前主要落在 `httpapi` 包**，职责边界不干净。
- 当前不是“无法维护”，但已经进入“**继续叠需求会显著恶化**”区间，应该先做结构收口再继续加功能。
- 建议目标：`httpapi` 只保留协议适配；战斗运行时下沉到独立域（如 `internal/battle/runtime` + `internal/battle/app`）。
- 迁移进度：已完成第一步基础下沉——timeline/hook schema 类型迁移到 `internal/battle/runtime/timeline_schema.go`。

## 1. 当前结构（基于代码盘点）

### 1.1 启动面
- `cmd/server/main.go`: gateway 进程（HTTP/WS + 业务接口）。
- `cmd/battle_worker/main.go`: battle worker 进程（无 HTTP 路由）。
- 两者都复用 `internal/httpapi` 的 `Server` 类型（通过 `NewServer`/`NewBattleWorker` 选项分支行为）。

### 1.2 模块体量（非测试 Go 文件）
- `internal/store`: 仍是最大状态/持久化实现面。
- `internal/httpapi`: 仍承载 HTTP/WS 入口，同时带着大量 battle runtime 逻辑。
- `internal/battle`: 目前已存在 `core/`、`runtime/`、`skills/` 三块，但运行时主体还没有完全迁出。

> 结论：战斗“真正厚逻辑”并没有主要放在 `internal/battle`，而是在 `httpapi`。

### 1.3 battle 相关文件分布（当前）
- `internal/httpapi/battle_runtime_engine.go`（战斗主运行时）
- `internal/httpapi/battle_loop.go`（tick/claim/playback worker 调度）
- `internal/httpapi/battle_buff_runtime.go`（buff 运行态）
- `internal/httpapi/battle_runtime_compile.go` / `battle_runtime_finalize.go` / `battle_runtime_playback.go`（编译、收尾、播放）
- `internal/httpapi/server_battle.go`（房间 HTTP 接口）
- `internal/httpapi/server_battle_metrics.go` + `battle_metrics.go`（监控）
- `internal/battle/core/*`（数学/编排基础能力）
- `internal/battle/runtime/timeline_schema.go`（已迁出的 timeline schema）
- `internal/battle/skills/registry.go`（技能注册表）

> 结论：battle 运行时已经开始向 `internal/battle` 迁，但主体依然留在 `httpapi`，目前仍是“协议层 + 运行时层同包共生”。

## 2. 当前痛点（可维护性视角）

1. **边界反转**
- `httpapi` 名称语义是协议适配层，但承担了 battle runtime 状态机、播放与结算编排。

2. **进程角色与代码角色错位**
- battle_worker 虽是独立进程，但实现仍在 `httpapi`，导致“battle 是独立服务”的心智模型与代码目录不一致。

3. **测试与迭代耦合**
- 修改 battle 运行时时，常常会连带碰到 WS/HTTP 封包与错误码逻辑，回归范围被动扩大。

4. **文档入口不足**
- battle 运行时文档已有（`battle-room-runtime.md`），但“代码归属图 + 迁移路线图”缺失，维护者看不出该改哪层。

## 3. 目标架构（建议）

### 3.1 目录边界
- `internal/httpapi`：只做 HTTP/WS 协议适配与错误码映射。
- `internal/battle/runtime`：run/tick/playback/finalize 状态机与数据流。
- `internal/battle/app`：battle room 应用服务（join/leave/ready/start/end/metrics）。
- `internal/battle/transport`（可选）：timeline 事件编码、快照拼装。

### 3.2 依赖方向
- `httpapi -> battle/app -> battle/runtime -> store`
- `httpapi` 不直接操作 battle runtime 内存状态结构。

### 3.3 渐进迁移原则
- **先搬代码，再换名字**；每一步保持对外 API/WS 契约不变。
- 不做“大重写”，每个阶段都可回滚。

## 4. 分阶段落地计划（建议顺序）

### Phase 0（本周）：文档和边界标注
1. 在 `battle-room-runtime.md` 增加“代码归属现状”与“计划边界”段落。
2. 在 `httpapi` battle 文件顶部统一标注 `TODO: move to internal/battle/runtime`。

### Phase 1（低风险搬迁）：纯逻辑下沉
1. 先迁移不依赖 Fiber/WS 的类型与函数：
   - timeline schema / checkpoint / compile pipeline / buff runtime 纯函数。
2. `httpapi` 保留薄封装调用（兼容现有路径与测试）。

### Phase 2（中风险）：应用服务抽离
1. 把 `server_battle.go` 的业务编排移到 `battle/app` 服务。
2. `server_battle.go` 只做：
   - 参数解析
   - auth/userID
   - 错误码转换

### Phase 3（中高风险）：worker 主循环抽离
1. 把 `battle_loop.go` 拆到 `battle/runtime/worker_loop.go`。
2. `cmd/battle_worker` 直接依赖 `battle/runtime`，不再通过 `httpapi.Server` 持有运行时。

### Phase 4（收口）：清理历史耦合
1. 删除 `httpapi` 内 battle 运行时残留实现。
2. 文档改为“battle 域优先入口”，`httpapi` 仅保留协议清单。

## 5. 你现在就能用的维护规则（临时止血）

1. 新 battle 规则/状态机改动：只动 `battle_*` 运行时文件，不改 `server_*` 接口层。
2. 新接口需求：先加 `server_battle.go` 协议，再调用 battle 域函数，不直接塞逻辑。
3. battle 运行时 bug 修复：优先写 `battle_runtime_engine*_test.go`，不要先改前端兜底。
