# 游戏服务端设计方案文档（Action Queue）

## TL;DR（先读这里）

- 本文核心：服务端采用**时间驱动 + 事件驱动 + 懒计算**，避免全局 tick 扫描。
- 生活链路：动作队列按需结算，强调可恢复、可重放、可幂等。
- 涉及并发与多实例时，配套阅读：`docs/architecture/scale-architecture.md`。
- 战斗房间当前实现已迁移到：`docs/backend/battle-room-runtime.md`（本文件中的战斗章节视为历史草稿，不再作为实现依据）。

## 当前重构方向（通用队列 + 业务处理器）

- actionQueue 核心只负责：队列状态机、顺序与抢占、调度与幂等编排。
- 队列项通过 `action_type + payload` 交给业务 handler 处理，队列核心不写业务分支。
- 快进同样由 handler 负责具体结算语义（普通动作聚合快进、强化动作 attempt 级快进）。
- 每个执行者队列（玩家 / 单只猫咪）同时处于 `pending` 或 `running` 的条目上限来自 `shared-global-config.json`：
  - `action.queueLaneLimit`：玩家队列上限
  - `action.kittyQueueLaneLimit`：猫咪助手队列上限
- 队列启动（`startQueueRunTx`）采用事务内 `SAVEPOINT` 包裹：启动失败先回滚子事务，再执行删队列/取消分支，避免“先扣材料后删队列”导致净损失。
- 默认快进预算配置：
  - `actionQueue.fastForward.maxSteps = 1000`
  - `actionQueue.fastForward.maxExecMs = 500`
- 战斗与生活 action 已解耦：战斗状态不再暂停或补偿 action queue 时间轴，队列始终按调度推进。
- 批量排产入口：`POST /api/actions/queue/batch`（单请求提交多步骤，服务端按顺序入队，避免前端多次发送导致的乱序/部分提交问题）。
  - `insertMode=append`：追加到队尾（原行为）。
  - `insertMode=run_now_like_prepend`：按“立即执行”语义插入队首；若拼接后超出对应 lane 上限，则按最终队列从尾部截断。

## 何时阅读本文

- 你在修改：动作队列、离线快进、调度器。
- 你在排查：队列顺序异常、长离线写放大。

## 常见任务直达

- 生活系统与队列：看“生活系统”相关章节
- 设计原则与约束：看“一、总体设计原则”

---

# 一、总体设计原则

## 1.1 核心思想

* **去 tick 化（No Tick）**
* **时间驱动（Time Driven）**
* **懒计算（Lazy Evaluation）**
* **事件驱动（Event Driven）**

---

## 1.2 系统划分

### 战斗系统

* 离散事件时间轴（Discrete Event Timeline）
* 支持组队、实时表现
* 服务端权威计算

### 生活系统

* 连续时间区间（Continuous Time Range）
* 队列驱动
* 按需结算

---

# 二、战斗系统设计（服务端）

---

> ⚠️ 本章为历史设计草稿，当前线上实现不以本章为准。  
> 战斗房间状态机、逐场 run、回放节拍与场间继承规则请以 `docs/backend/battle-room-runtime.md` 为唯一准入文档。

## 2.1 战斗模型：事件时间轴

### 核心结构

```ts
type BattleEvent = {
  time: number        // 事件发生时间（相对战斗开始）
  type: string        // attack / skill / death / buff / ...
  actorId: string
  targetId?: string
  data?: any
}
```

---

## 2.2 战斗实例（Battle Instance）

```ts
type BattleInstance = {
  battleId: string
  startTime: number
  currentTime: number

  participants: Actor[]
  eventQueue: MinHeap<BattleEvent>

  seed: number  // 随机种子（保证一致性）
}
```

---

## 2.3 Actor（单位）

```ts
type Actor = {
  id: string
  hp: number
  atk: number

  attackInterval: number
  nextAttackTime: number

  skillCd: number
  nextSkillTime: number

  buffs: Buff[]
}
```

---

## 2.4 核心循环（无 tick）

```ts
function processBattle(instance: BattleInstance, untilTime: number) {
  while (true) {
    const next = instance.eventQueue.peek()
    if (!next || next.time > untilTime) break

    instance.eventQueue.pop()
    instance.currentTime = next.time

    handleEvent(instance, next)
  }
}
```

---

## 2.5 事件处理

```ts
function handleEvent(instance: BattleInstance, event: BattleEvent) {
  switch (event.type) {
    case "attack":
      // 计算伤害
      // 更新 hp
      // 推入下一次攻击
      break

    case "skill":
      // 技能效果
      break

    case "death":
      // 处理死亡
      break
  }
}
```

---

## 2.6 事件生成

```ts
function scheduleNextAttack(actor: Actor, now: number) {
  return {
    time: now + actor.attackInterval,
    type: "attack",
    actorId: actor.id
  }
}
```

---

# 三、战斗前后端交互（WebSocket）

---

## 3.1 连接流程

```text
客户端 -> WS连接
客户端 -> 发送 battle_subscribe
服务端 -> 返回初始状态
服务端 -> 持续推送事件
```

---

## 3.2 消息协议

### 订阅战斗

```json
{
  "op": "battle_subscribe",
  "battleId": "xxx"
}
```

---

### 初始同步

```json
{
  "op": "battle_init",
  "battleId": "xxx",
  "serverTime": 1000,
  "events": [],
  "snapshot": {}
}
```

---

### 事件推送（批量）

```json
{
  "op": "battle_events",
  "battleId": "xxx",
  "events": [
    { "t": 1.2, "type": "attack", "actorId": "A" },
    { "t": 2.0, "type": "skill", "actorId": "B" }
  ]
}
```

---

### 战斗结束

```json
{
  "op": "battle_end",
  "battleId": "xxx",
  "result": {
    "win": true,
    "rewards": []
  }
}
```

---

## 3.3 推送策略

### 推荐方案（混合）

* 每 200~500ms 批量推送事件
* 客户端本地排时间播放

---

## 3.4 客户端职责

* 按时间轴播放动画
* 本地插值（平滑）
* 不做权威计算

---

# 四、生活队列系统设计

---

## 4.1 核心抽象

👉 所有行为统一为：

```ts
type QueueTask = {
  id: string
  type: string
  configId: number

  baseDuration: number

  progress: number
  lastUpdateTime: number

  segments: Segment[]

  extra: Record<string, any>
}
```

---

## 4.2 分段结构（支持装备变化）

```ts
type Segment = {
  startTime: number
  endTime: number
  speed: number
}
```

---

## 4.3 进度计算

```ts
function calcProgress(task: QueueTask, now: number): number {
  let progress = task.progress

  for (const seg of task.segments) {
    const end = Math.min(seg.endTime, now)
    const delta = end - seg.startTime

    progress += (delta * seg.speed) / task.baseDuration
  }

  return Math.min(progress, 1)
}
```

---

## 4.4 队列结构

```ts
type ActionQueue = {
  queueId: string
  ownerId: string

  tasks: QueueTask[]
  startTime: number
}
```

---

# 五、支持的行为类型（统一扩展）

---

## 5.1 普通行动（采集/钓鱼等）

```json
{
  "type": "gather",
  "configId": 1001
}
```

---

## 5.2 强化

```json
{
  "type": "enhance",
  "configId": 2001,
  "extra": {
    "itemId": "xxx"
  }
}
```

---

## 5.3 炼金

```json
{
  "type": "alchemy",
  "configId": 3001,
  "extra": {
    "recipeId": "xxx"
  }
}
```

---

## 5.4 分解

```json
{
  "type": "decompose",
  "configId": 4001,
  "extra": {
    "items": []
  }
}
```

---

## 5.5 可扩展设计

👉 新功能只需新增：

* type
* config
* handler

---

# 六、队列推进逻辑

---

## 6.1 当前任务计算

```ts
function getCurrentTask(queue: ActionQueue, now: number) {
  let time = now - queue.startTime

  for (const task of queue.tasks) {
    const duration = getDynamicDuration(task)

    if (time < duration) {
      return task
    }

    time -= duration
  }

  return null
}
```

---

## 6.2 装备变化处理

```ts
function onEquipChange(task: QueueTask, now: number, newSpeed: number) {
  const lastSeg = task.segments[task.segments.length - 1]
  lastSeg.endTime = now

  task.segments.push({
    startTime: now,
    endTime: Infinity,
    speed: newSpeed
  })
}
```

---

# 七、存储设计建议

---

## 7.1 数据库存储

### 战斗

* battle_instance（短期，可内存+缓存）
* 不建议持久化完整事件

---

### 生活队列

```json
{
  "queueId": "",
  "tasks": [],
  "startTime": 0
}
```

---

## 7.2 Redis 使用

* 当前战斗实例
* 活跃玩家队列

---

# 八、性能与扩展性

---

## 8.1 战斗

* O(事件数)
* 无 tick
* 支持 10k+ 并发

---

## 8.2 生活系统

* O(1) 结算
* 无后台任务
* 无实时更新

---

## 8.3 扩展性

支持：

* 新玩法（只加 type）
* 新 buff（只影响 speed）
* 新系统（复用队列）

---

# 九、总结

---

## 战斗

* 事件时间轴
* 服务端权威
* WS 推送事件

---

## 生活

* 区间时间模型
* 分段加速
* 队列驱动

---

## 核心优势

* 极低计算成本
* 高扩展性
* 易维护
* 适合中大型放置类游戏

---

# 十、动作队列 WS 操作约定（当前实现）

---

## 10.1 客户端消息类型

```text
action.run              // 立即执行：创建队列项并立刻抢占进入 running（等价 run_now 语义）
action.enqueue          // 加入队列：创建 pending 队列项，不抢占当前 running
action.queue.get        // 拉取并推进队列快照
action.queue.move       // 移动 pending 队列项（up/down）
action.queue.run_now    // 将指定 pending 队列项提升为优先执行
action.cancel           // 取消指定队列项（pending/running）
```

### `action.run` 请求

```json
{
  "type": "action.run",
  "payload": {
    "actionId": "work_mining",
    "mode": "run",
    "count": 10,
    "args": {
      "materialInstanceBindings": {
        "copper_sword": [12345]
      },
      "inheritFromInstanceId": 12345
    }
  }
}
```

语义：

* 会先创建队列项，再立即提升并进入 `running`。
* 若已有 `running`，当前 `running` 会按服务端退款规则回退为 `pending`，新目标项立即执行。

### `action.enqueue` 请求

```json
{
  "type": "action.enqueue",
  "payload": {
    "actionId": "work_mining",
    "mode": "queue",
    "count": 10
  }
}
```

语义：

* 仅创建 `pending` 队列项，不抢占当前 `running`。
* 不做状态门槛前置拦截（允许提前排队，后续在真正启动该队列项时再校验门槛/资源）。
* `equipment_enhance` 例外：入队阶段会做目标实例与参数边界合法性校验（如所有权、实例状态、目标等级、次数正整数），但**不在入队校验材料库存**；材料与保护道具在执行阶段统一校验。

### `POST /api/actions/queue/batch`（批量）

请求体新增 `insertMode`：

```json
{
  "steps": [
    { "actionId": "work_mining", "count": 10 },
    { "actionId": "work_logging", "count": 5 }
  ],
  "actorType": "player",
  "actorId": 0,
  "insertMode": "run_now_like_prepend"
}
```

返回：

```json
{
  "ok": true,
  "enqueuedCount": 2,
  "droppedCount": 1
}
```

语义：

* `append`：沿用旧行为，超上限返回冲突错误。
* `run_now_like_prepend`：将步骤序列插入队首；若该 lane 存在 `running`，会按 run_now 语义回退为 `pending` 并后移。
* 若最终 lane 超过 20 条，按最终顺序从尾部删除多余任务，并通过 `droppedCount` 返回截断数量。

### `action.queue.run_now` 请求

```json
{
  "type": "action.queue.run_now",
  "payload": { "queueId": 123 }
}
```

### `action.cancel` 请求

```json
{
  "type": "action.cancel",
  "payload": { "queueId": 123 }
}
```

## 10.2 服务端推送

成功后统一推送（当前约定）：

```text
action.queue.updated                 // 全量队列快照
player.attributes.updated           // patch: upsert/remove + attributeProgressUpsert/remove + meta.version
inventory.instances.updated         // patch: upsert(整物品对象)/remove(instanceId[]) + meta.version
equipment.instances.updated         // patch: upsert(整物品对象)/remove(instanceId[]) + meta.version
equipment.stats.updated             // 对象级整包替换（非字段级 patch）
game.state.patch                    // 轻量信号（userId/updatedAt）
```

补充：

- 物品域 patch 仅做“对象级变更”：某个物品发生变化时，`upsert` 发送该物品完整对象；删除仅在 `remove` 中发送 `instanceId`。
- `meta.version` 为 patch 事件必填，客户端按版本去重/防乱序。

## 10.2.1 队列核心与执行器职责边界（当前实现）

- `action_queue` 核心仅负责：生命周期状态机（pending/running/done）、顺序推进、调度时间与统一 summary 聚合。
- 具体业务（参数、每次执行的消耗、完成产出、取消回滚）由对应 `QueueActionExecutor` 实现，不在队列核心硬编码分支。
- 队列耗时重算是执行器可选能力：执行器自行声明允许重算 `pending`、`running` 或两者均不允许，并负责在不破坏自身 runtime 的前提下更新耗时；队列核心不解析业务 runtime，也不维护 action ID 特例表。
- 所有执行器只需遵守 runtime envelope 的公共 `durationMs` 调度字段；队列核心读取该公共字段，不按 executor/action 类型枚举耗时解析规则。
- 统一语义：
  - **开始执行时先消耗**（`StartRunTx`）
  - **完成时才产出/落库**（`Settle*Tx`）
  - **执行中取消时回滚当前运行周期已消耗**（`CancelRunTx`）
- Generic 动作支持可选实例材料参数 `args.materialInstanceBindings`：
  - 绑定实例会优先按 `instanceId` 消耗；不足部分回退模板扣减。
  - 绑定实例仅在 affix 与模板 `defaultAffixSlots`（`slotIndex + affixId`）一致时可用；若不一致会拒绝执行（返回通用参数错误，不暴露具体 instanceId）。
  - 使用实例绑定时 `count` 必须为 `1`。
  - 若同时提供 `inheritFromInstanceId`，产物可强化时按 `floor(sourceLevel * ratio)` 继承强化等级，`ratio` 来自 `shared-global-config.json`（默认 `0.5`），并裁剪到产物 `enhanceMaxLevel`；入包后按新等级重算快照。
- 快进与在线都经过同一执行器语义，避免两套逻辑漂移。
- 队列返回项可携带 `display` 元数据（可选），用于前端按 action 类型渲染卡片细节（如强化目标物品与目标等级）。

## 10.3 错误码约定（队列相关）

```text
WS_ACTION_QUEUE_ID_REQUIRED        // queueId 缺失
WS_ACTION_QUEUE_MOVE_FAILED        // 移动队列项失败
WS_ACTION_QUEUE_RUN_NOW_FAILED     // 立即执行失败（兜底）
WS_ACTION_CANCEL_FAILED            // 取消队列项失败
WS_ACTION_INSUFFICIENT_ITEM        // 立即执行推进时材料不足
```

说明：

* `run_now` 失败会优先返回可判别错误（如 `WS_ACTION_INSUFFICIENT_ITEM`），未知错误再回落到 `WS_ACTION_QUEUE_RUN_NOW_FAILED`。
* `run_now` 若资源/门槛不满足，会直接失败并保留该队列项（不会自动删队列）。
* `action.cancel` 支持取消 `pending` 与 `running` 队列项；取消 `running` 时按服务端退款与队列推进逻辑处理。

---

# 十一、并发 / 防重放 / 全局同步补充（当前实现）

---

## 11.1 属性经验升级与属性值联动

* 属性经验按 `growth.xlsx / attribute_exp_profiles` 计算：`expRequired` 表示“当前等级升到下一级所需经验”。
* 每次升级后按等级切换到下一档经验池继续累计；达到 profile 最大等级后封顶（`nextExp=0`，新增经验不再生效）。
* 当属性等级跨阈值时，服务端在同一事务内将对应 `player_base_stats` 属性值 `+1`（按跨级差值累加）。
* 发生属性值变化时，同事务重算派生属性缓存，确保战斗/展示一致。
* 队列结算写入属性经验时，若玩家缺失该属性行，服务端会先补齐属性种子再应用经验，避免离线快进场景经验丢失。

## 11.2 WS 防重放

* 所有 mutating WS 消息（如 `action.run/enqueue/cancel/move/run_now`）必须携带 `requestId`。
* 服务端对 `(userId, messageType, requestId)` 做短窗口去重（Redis 优先，内存兜底）。
* 重复请求返回：`WS_DUPLICATE_REQUEST`。

## 11.3 全局订阅与队列推进

* 队列推进不应依赖某个业务页面挂载。
* 客户端在全局层（Shell/Store）定时触发 `action.queue.get`，保障任意入口页面都能持续推进并拿到库存/状态更新。
* 页面层仅消费实时状态，不承担关键同步链路初始化职责。

## 11.4 属性变动后的队列耗时重算（当前实现）

* 当玩家属性发生写入型变更（如穿脱装备、词条拆装/附魔、强化结算、战斗点分配/重置、属性经验升级）时，服务端在同一事务内执行：
  1. 重算玩家属性缓存（derived stats）
  2. 加载该用户的 `pending` / `running` 队列项，并把每一项交给其 `QueueActionExecutor` 判断是否支持对应状态的耗时重算
* 队列核心只负责在执行器返回新 payload 后持久化，并在 `running` 耗时确实变化时按原 `current_run_started_at` 更新结束时间；它不按 action ID 或 actor 类型解释业务。
* Generic 执行器支持 `pending` 与 `running` 重算，并按队列项自己的 `actorType/actorId` 读取属性，因此主角换装只会让主角动作使用新增益，不会把主角属性套到猫咪动作上。
* 强化、建筑、炼丹等运行时由执行器控制的动作目前只允许重算 `pending`；已经开始的一轮保持其执行器时序语义。
* 钓鱼等状态机执行器不实现耗时重算能力，其 trip、阶段、补给、渔获、冻结装备等完整 runtime 始终由该执行器独占维护。
* 后续新增探险等特殊队列时，只需由新执行器决定是否实现该能力以及允许的状态，不需要修改队列核心的分支或名单。
* 只读请求链路（如纯查询 stats/bootstrap 读取路径）不负责触发重算；重算仅在写入型 mutation 链路触发。

## 11.5 队列展示字段

* `player_action_queue.payload` 中的 runtime 只保存执行所需信息；客户端展示层不要自行按静态配置推导 ETA。
* `action.queue.updated` 下发的 queue item `display` 会附带 `effectiveDurationMs`，表示该条队列项当前应使用的真实时长（已包含效率加成）。
* 旧字段（如 building 的 `durationS`）保留用于兼容展示，不作为主页队列 ETA 的主来源。

---

# 十二、按 Queue 聚合快进（新增）

## 12.1 目标

* 支持长离线下的大步数快进，不按单步逐次入库。
* 保持严格队列顺序：前序 queue 完成后才检查后序 queue 门槛。
* 支持“训练解锁后续动作”场景（如力量训练完成后自动进入挖矿）。

## 12.2 结算策略

* 对当前 queue 计算可完成次数 `n`（由 `elapsed / unitDuration` 与 `remaining` 共同决定）。
* 奖励、属性经验、`done_count` 按 `n` 聚合后写入（同一事务）。
* 若 queue 完成则切到下一个 queue；若仅部分完成则保留剩余进度继续 running。
* 启动下一个 `pending` 队列项时，才校验状态门槛与材料：
  * 门槛不满足：该队列项会被取消并写入 `queueCanceled`（`STATUS_REQUIREMENT_NOT_MET`），然后继续推进后续队列项。
  * 材料不足：该队列项会被取消并写入 `queueCanceled`（`COST_INSUFFICIENT`），然后继续推进后续队列项。

## 12.3 随机奖励

* 使用 queue 级稳定随机源（基于 `userId + queueId + doneCountBefore`）。
* 概率奖励按批采样后聚合入库，避免逐步写放大。

## 12.4 run_now 语义

* `action.queue.run_now` 的目标是“队列优先级立即提升并进入执行态”。
* 执行后应在同次推进结果中体现为目标项 `running`（若资源/门槛满足）。

## 12.5 快进奖励回传（前端展示）

* 客户端在进入游戏后主动请求 `action.offline.rewards.get`。
* 服务端仅在该请求中检查并结算离线快进，返回 `action.offline.rewards.result`：
  * `protocolVersion` 与原请求 `requestId`
  * `status`（`complete` / `progress` / `committed_sync_pending` / `stalled` / `failed_before_commit`）
  * `snapshotSynced`（本次提交后的权威快照是否已经广播）
  * `projectionDeferred`（客户端明确支持终态同步，且本次仍有可续跑 chunk）
  * `completedSteps`
  * `itemRewards`（itemId -> qty）
  * `attrExp`（attrKey -> gain）
  * `itemCosts`（itemId -> qty）
  * `queueCompleted`（已完成/移除的 queueId）
  * `queueCanceled`（自动取消原因明细）
* 前端收到该响应后展示离线快进收益（物品 + 属性经验）。
* 离线快进待结算标记使用 Redis 共享键（`aq:offline_ff_ready:<userId>`）。结算结果成功进入 WS 发送队列后，服务端仅原子删除本次开始时读取到的完整标记（其中包含冻结的 `targetAtMS`）；旧请求不得删除更新的离线窗口。
* 可见性驱动规则：`hidden -> presence=offline`，`visible -> 先确认 presence=online 成功，再调用统一入口 `action.offline.rewards.get`；禁止多处并行触发离线快进。
* `visible`/`hidden` 的离线切换阈值来自 `shared-global-config.json` 的 `presence.visibilityOfflineDelaySeconds`；只有超过阈值才真正切到 offline。服务端仅使用自身时间戳处理 presence 切换，不采信客户端提交的观察时间。

## 12.6 离线快进统一编排契约

离线快进只增加一种“推进方式”，不增加第二套玩法语义。统一链路如下：

1. WS 分发层只把请求交给 `handleOfflineFastForwardRequest`；该协调器先做 marker 快速检查，取得用户级 claim 后必须重新读取并冻结当前 `targetAtMS`，避免多标签页拿旧 marker 重复推进。Redis 不可用时使用同进程互斥 claim，数据库用户锁仍是最终写入串行边界。
2. `Runtime.FastForwardOffline` 只调用 mutation-only 的 chunk 边界；单个 chunk 在一个数据库事务和用户锁内完成，仍受步数与时间双预算保护。预算耗尽时只允许为刚完成的 lane 启动其下一条 pending 项，以持久化时间接力点和编排元数据，不得再结算新的玩家可见步骤；其他 lane 不得抢占这个收尾阶段。
3. 队列核心只负责选取 due row、切换生命周期、汇总 `SettleResult`。Generic、建筑、炼金、强化、钓鱼及后续动作统一通过 `QueueActionExecutor` 执行业务结算。
4. 工作套装仍保持原有离线语义：每个动作开始时解析并冻结对应套装属性，但不反复物理换装。最近一次成功解析的套装 ID 随现有队列 JSON runtime 跨 chunk 传递；玩家动作 lane 追平冻结目标时间后，在同一事务提交前应用一次最终套装。若当前玩家动作因执行器无进度而停止自动续跑，则应用该动作已冻结的套装，使物理装备与当前 running 动作一致；其他 lane 的停顿不得提前收敛玩家套装。kitty lane 不得阻塞玩家套装收敛，套装应用失败则回滚该 chunk，不允许提交后静默失败。
5. 执行器若既没有完成动作、推进状态机，也没有请求取消，编排器以 `yieldReason=no_progress` 停止该 chunk，禁止在同一 due row 上空转；响应同时把 `hasMoreDue` 置为 `false` 以阻止新旧客户端自动重试，但服务端保留 marker 供后续显式恢复。
6. 数据库提交成功后，网关使用独立的短超时读取并广播权威快照。该读取失败只返回 `committed_sync_pending`，不能把已提交结算重新解释成失败；客户端随后走既有全量同步兜底。
7. 仅在最终结果成功进入 WS 发送队列、汇总中的 `TargetAtMS` 与本次冻结目标严格相等后，才条件消费同一份离线标记。`hasMoreDue=true`、结算失败、发送失败、目标不一致和 `stalled` 都保留标记，以便安全续跑或重试。

最终套装应用成功后（包括当前装备已匹配而无需换装），必须在同一事务内清除玩家 running 行上匹配的 `deferredWorkLoadoutId`。该字段是一次性编排意图，不是永久绑定；否则长队列下一次离线恢复会重放旧套装，覆盖玩家后续手动换装。清理只删除该 JSON 字段，不改变冻结属性、动作效果快照、消耗或队列时间；清理失败仍回滚整个 chunk。

`OperationEffectsChanged` 与装备、钓鱼等独立领域变更一样，构成中间批次必须同步的条件。效果次数可能在非终态批次耗尽，不能假设最后一批一定再次返回变更标记；后续新增独立投影也必须明确接入这条规则。

协议版本 `2` 支持客户端请求 `projectionPolicy=terminal`。请求和响应均使用显式类型，由协调器集中推导 `status`、`hasMoreDue` 与同步状态，避免字段在不同分支漂移。只有明确声明该能力的新客户端，服务端才会在 `hasMoreDue=true` 时延后可由终态快照完整覆盖的队列、玩家状态、属性和仓库投影。钓鱼、强化、装备变更及 kitty 属性变更仍立即同步，不能在没有跨 chunk 服务端失效标记的情况下丢弃其领域刷新信息。终态、`stalled` 以及未声明能力的旧客户端同样执行完整同步；静态钓鱼变异目录只在客户端终态响应携带，避免每个 continuation chunk 重复序列化。服务端回传原 WS `requestId`，客户端丢弃超时或断线后到达的旧响应。busy、超时或结算失败同时保留旧客户端依赖的 result 事件，并标记 `failed_before_commit`；任一后续 chunk 失败或超时，客户端仍执行现有全量权威同步兜底。

离线收益汇总只在终态写入前端响应式 Store。强化结果既可由 `equipment.enhance.result` 实时到达，也可由终态收益补齐；两条路径按 `queueId + attemptNo + equipmentInstanceId` 去重，禁止将累计收益中的历史结果在每个 chunk 重复追加。实时 Store 的既有窗口只有最近 100 条，因此单个离线 chunk 只扇出最后 100 条强化事件；离线 result 仍携带该 chunk 的完整强化明细，收益弹窗语义不变。

新增动作接入时必须满足以下条件：

* 实现完整 `QueueActionExecutor`，在线与离线复用同一份冻结 runtime、消耗、奖励、随机种子和取消规则。
* 必须实现 `OfflineProjectionPolicy`：只有全部副作用都能由统一终态快照重建的执行器才可声明 `QueueOfflineProjectionTerminal`；包含独立领域状态或事件流的执行器必须声明默认安全的 `QueueOfflineProjectionImmediate`。新增专用执行器未声明时应在编译期失败。
* `SettleFastForwardTx` 只能在与重复 `SettleNormalTx` 数学等价时批处理；否则逐次执行，但仍必须遵守 `FastForwardBudget`。
* 状态机在未完成一个玩家可见动作但已经持久化阶段进度时返回 `Progressed=true`，并用 `BudgetIterations` 上报内部工作量；无进度不得伪造完成数。
* `SettleResult` 必须完整返回本次事务中的物品消耗、物品产出、属性经验、专属事件和可恢复的 `NextPayload`，由统一汇总层处理展示与后续同步。
* 回归测试至少覆盖：在线/离线结果等价、不同 chunk 切分结果一致、预算中断后可续跑、材料不足或目标失效的取消语义、冻结 runtime 的旧数据兼容。

以上契约只使用现有队列表与 JSON runtime，不要求数据库迁移。

---

# 十三、战斗时间轴与 Hook 数据结构约定（新增）

## 13.1 目标

* 战斗采用“固定 seed + 确定重演”：开战后可一次性计算完整时间轴与最终结果。
* 服务端按窗口节奏推送（非一次性全量下发），兼顾沉浸感与吞吐。
* Hook（含回合外动作）作为时间轴一等事件，支持扩展与稳定重放。

## 13.2 全局配置

`global-config.json`：

```json
{
  "battle": {
    "timelinePushIntervalMs": 1000
  }
}
```

语义：

* 推送窗口间隔（毫秒）。
* 仅影响“服务端对客户端的批量推送节奏”，不改变确定性结算结果。

## 13.3 Hook 配置结构（game-data）

```ts
type BattleHookDef = {
  id: string
  trigger: "battle_start" | "turn_start" | "before_damage" | "after_damage" | "on_kill" | "on_death" | "on_hit" | "periodic"
  chance?: number
  priority?: number
  actionDurationMs?: number // undefined -> 200; 0 -> 0; >0 -> 自定义
  maxTriggersPerBattle?: number
  cooldownMs?: number
  target: {
    side: "self" | "ally" | "enemy" | "any"
    selector: "self" | "random" | "lowest_hp" | "highest_atk" | "by_position"
    count?: number
    position?: "vanguard" | "midguard" | "rearguard"
  }
  effects: BattleEffectDef[]
}
```

## 13.4 时间轴事件结构

```ts
type BattleTimelineEvent = {
  seq: number
  offsetMs: number
  lane: "turn" | "hook" | "system"
  phase: "pre" | "main" | "post"
  parentSeq?: number
  groupId?: string
  orderKey: string
  actorId?: string
  actorUserId?: number
  hookId?: string
  actionDurationMs: number
  payload: Record<string, any>
}
```

约束：

* 同 `offsetMs` 可存在多个“看起来同时”的事件，通过 `groupId` 聚合表现。
* 执行顺序由 `orderKey` 稳定决定，避免跨实例重放顺序漂移。

## 13.5 回合外动作（Hook）顺序模型

* Hook 不是插播补丁，而是完整时间轴事件。
* 例如“受击后 20% 反击”：生成 `lane=hook`、`parentSeq=<受击事件>` 的新事件。
* 该事件可与父事件同 `offsetMs`，但通过 `phase/sub-order` 保证“父结算后、下回合前”顺序。

## 13.6 RNG 固定性

* 使用房间级 seed（结合 run 维度）初始化随机源。
* 候选 Hook 按固定顺序遍历（位次、单位 id、hook id）。
* 每次触发判定与随机选目标都走统一 RNG 入口，保证同输入同输出。

## 13.7 编译产物与恢复点

```ts
type BattleCompiledArtifact = {
  runId: number
  roomId: number
  seed: string
  rulesVersion: string
  snapshotHash: string
  totalDurationMs: number
  events: BattleTimelineEvent[]
  result: Record<string, any>
}

type BattlePlayCheckpoint = {
  runId: number
  lastSeq: number
  lastOffsetMs: number
  updatedAtMs: number
}
```

## 13.8 WS 批量推送协议

```ts
type BattleEventBatch = {
  runId: number
  fromSeq: number
  toSeq: number
  windowStartMs: number
  windowEndMs: number
  events: BattleTimelineEvent[]
}
```

客户端要求：

* 按 `seq` 去重，支持重复包重放安全。
* 按 `offsetMs` + 本地时钟播放，窗口内插值渲染。
