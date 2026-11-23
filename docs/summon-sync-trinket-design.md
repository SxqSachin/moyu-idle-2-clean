# 召唤同步行动饰品 — 设计方案

## 需求概述

帮我设计一下，我想要一个新的战斗饰品，让装备此饰品的单位所召唤的召唤物，不再加入到行动队列中。而是转而在召唤物所有者行动的回合一起进行行动。这些逻辑必须使用现有的DSL来实现，且必须用combat-model-vue3的数据流转逻辑进行实现，我想的是如果玩家装备了这个饰品，触发了召唤动作，那么为召唤物附加一个额外的buff，在buff内来实现这些效果。你先设计看看怎么实现这个DSL以及在战斗中如何去实现这种效果。

新增战斗饰品。装备后，持有者召唤的召唤物**不加入行动队列**，改为在召唤者行动完毕后**同步行动**（完整回合：技能选择执行、冷却递减、buff 推进等全部正常发生），且前端体现为这些单位的动画同时播放，而不是每个单位依次播放，且同时行动时，在同步行动的batch中，需要包含每个单位的行动情况作为sub。

## 核心思路

双行动队列 + 行为标记驱动：

- **FullOrder**（全行动顺序）：所有存活单位的完整排序，用于恢复位置
- **Order**（真实行动顺序）：FullOrder 过滤掉有 SkipTurnOrder 标记的单位

行为标记 `SkipTurnOrder` 由 `battleRecomputeUnitStats` 根据 while_active buff effect 自动计算，不硬编码 buff ID。标记属于单位运行时状态（`json:"-"`），buff 变化时自动重置。

```
buff 变化 → battleRecomputeUnitStats → 设/清 SkipTurnOrder → 下一次 reseedOrder 同步
```

---

## 关键改动点

### 1. `battleUnitState` 新增运行时标记

位置：`battle_runtime_engine.go` ~133 行，`battleUnitState` 结构体

```go
SkipTurnOrder bool `json:"-"`
```

- 仅运行时标记，不序列化
- 零值 `false`，所有现有单位不受影响
- 只由 `battleRecomputeUnitStats` 写入

### 2. `battleRecomputeUnitStats` 注入标记计算

位置：`battle_buff_runtime.go:793`，while_active 循环中

循环开始前设两个 flag 为 false，effect kind 提取后立即 switch：

```go
switch kind {
case "skip_turn_order":
    unit.SkipTurnOrder = true; continue
case "trigger_synced_summon_turn":
    unit.HasSyncedSummonTrigger = true; continue
}
```

注意点：
- 这两个 effect 无 `attr` 字段。原逻辑 `attr == ""` 时 `continue`，新逻辑提前 `continue`
- 两种路径等效。新 effect 的规则不参与任何 stat 计算
- `continue` 确保不跳过同次迭代中其他 effect 的处理（两个规则各占一条 effect record，没有其他效果需要处理）

### 3. `reseedOrder` — 双队列架构

位置：`battle_runtime_engine.go:6814`

行为变更：

```
旧：收集存活单位 → 排序 → 写 state.Order
新：收集存活单位 → 排序 → 写 state.FullOrder
                          → 过滤 SkipTurnOrder=false → 写 state.Order
```

`FullOrder` 是排序后的全量顺序，`Order` 是过滤后的实际行动顺序。

关键保证：
- 过滤不改变相对顺序（保持 speed 排序结果）
- `Order` 长度 ≤ `FullOrder` 长度
- 当 `SkipTurnOrder` 从 true 变 false，下一次 `reseedOrder` 自动恢复
- `advanceTurnCursor` / `battleNextActor` 只操作 `Order`
- `orderMatchesAliveUnits` 检查 `FullOrder` 而非 `Order`

`reseedOrder` 不关心 buff ID，不关心触发原因——只读 flag。

### 4. `addBattleSummon` — 时序保障

位置：`battle_runtime_engine.go:12389`，尾部 ~12481 行后

现有流程：
```
绑定 buff → append → Order=[] → reseedOrder（召唤物进入 Order）
```

需要确保 `SkipTurnOrder` 在 `reseedOrder` 之前已设。做法：

在定义 `unit` 结构体后、append 之前：
1. 查 source 是否有 trinket buff（`findBuffByID(source, "summon_sync_trinket")`）
2. 若有，给 unit 设 entrained buff（`battleApplyOrRefreshBuff`）
3. 调用 `server.battleRecomputeUnitStats(&unit)` → `SkipTurnOrder=true`
4. 之后再 append + reseedOrder

`battleRecomputeUnitStats` 只需 unit 指针，不依赖 state。append 前调用安全。

注意点：
- 若 `battleUnitByID` 查不到 source（已经被击杀），走正常的无饰品路径
- `battleApplyOrRefreshBuff` 不自动触发 `battleRecomputeUnitStats`，必须手动调用
- 这一步不创建新 effect 记录（不 emit），只是在内存中设了个 bool

### 5. `processBattleTurn` — 触发同步召唤

位置：`battle_runtime_engine.go:1487`，主行动者 emit 后

在 `executeUnitTurn` 提取后（v2 重构），emit 主行动者事件后：

```go
if findBuffByID(actor, "summon_sync_trinket") != nil {
    s.executeSyncedSummonTurns(state, now, actor, side, mainActorSeq)
}
```

用 `findBuffByID` 直接查 buff 比预计算标记更可靠——actor 身上 3-10 个 buff，O(n) 开销可忽略。

`mainActorSeq` 取 `state.Seq + 1`（emit 内先 ++ 再赋值），不依赖 emit 实现细节。

### 6. `executeSyncedSummonTurns` — 同步召唤物执行

位置：`battle_runtime_engine.go`（新函数）

逻辑：
1. 只遍历 summoner 所在 side 的 Players/Enemies
2. 过滤条件：Alive && HP > 0 && SummonSourceUnitID == summonerID && `SkipTurnOrder == true`
3. 按 Stats["speed"] 降序排序，speed 相同则 UnitID asc（与 reseedOrder 规则一致）
4. 逐个调用 `executeUnitTurn`，每个调用前重新检查 Alive && HP > 0
5. emit 前设 `payload["parentSeq"] = mainActorSeq` + `payload["batchGroup"] = mainActorSeq`

排序保证：高速召唤物先行动，行动中若击杀低速召唤物 → pre-action 死亡检查捕获。

### 7. `nextEventBatch` — ParentSeq 提取

位置：`battle_runtime_engine.go:~4785`

从 `event.Data["parentSeq"]` 提取到 `BattleTimelineEvent.ParentSeq`，然后删除 `event.Data["parentSeq"]`，重新 marshal payload。

这样：
- Timeline 持久化看到 `ParentSeq`
- 前端 `event.d` 看不到 `parentSeq`（已被提取），但仍然能看到 `batchGroup`

### 8. JSON 数据定义

纯新增，不修改已有记录。

**buff.json**：`summon_sync_trinket`（永久/不可净化）+ `summon_sync_entrained`（永久/不可净化）

**buff_effects.json**：
- `summon_sync_trinket_wa`（while_active）→ effect.kind = `trigger_synced_summon_turn`
- `summon_sync_entrained_wa`（while_active）→ effect.kind = `skip_turn_order`

**equipment_templates.json**：`summon_sync_trinket_001`（accessory 槽位，on_battle_start → add_buff summon_sync_trinket to self）

### 9. 前端并行动画

位置：`BattleView.vue`

新增 `currentBatch` ref。`pumpReplayQueue` 中检测 `data.batchGroup`，连续取出所有同组事件设为 currentBatch。

`watch(currentBatch)` 合并批次内所有 action 的轨迹 + 浮动标签，一次性渲染。

### 10. combat-model-vue3

BUFF_EFFECT_KINDS 追加 `skip_turn_order`、`trigger_synced_summon_turn`。

---

## 概念对比

| 概念 | 用途 | 访问方式 | 示例 |
|------|------|----------|------|
| FullOrder | 全量排序（含跳过单位） | `state.FullOrder` | [A, **skipped_B**, C, D] |
| Order | 真实行动队列 | `state.Order` | [A, C, D]（B 被过滤） |
| SkipTurnOrder | 行为标记 | `unit.SkipTurnOrder` | while_active 计算 |
| 饰品 buff | 佩戴者标识 | `findBuffByID` | trigger_synced_summon_turn |

---

## 边界情况

### 标记生命周期

```
buff 附加 → while_active 扫描 → SkipTurnOrder=true → reseedOrder 过滤 → Order 中排除
buff 到期 → while_active 失效 → SkipTurnOrder=false → 下一轮 reseedOrder → Order 中恢复
```

不需要 `on_detach` 或 `on_attach` 钩子。标记由 while_active 持续维护，buff 消失后下一次 `battleRecomputeUnitStats` 自动清零。

### 多召唤者（PVP 双方各带饰品）

- 各召唤者的召唤物 `SummonSourceUnitID` 指向各自 source
- `executeSyncedSummonTurns` 按 `SummonSourceUnitID == summonerID` 过滤
- 双方均有召唤物时，各自在己方召唤者回合内触发同步
- A 的召唤物不会被 B 的其他行动者误触发

### 召唤物再召唤子召唤物

- 子召唤物的 `SummonSourceUnitID` 指向直接召唤者（召唤物 A）
- 召唤物 A 身上没有 trinket buff → `executeSyncedSummonTurns` 检查 A 的 source（主行动者）时，不会找到 A 的子召唤物
- 当召唤物 A 作为同步召唤物行动时，A 的技能产生子召唤物 B → B 的 source 是 A → 下一轮主行动者回合执行同步时不会误触发 B

### 同步召唤物击杀同步召唤物

- A（高速）行动时击杀 B（低速）
- A emit 后，B 的 pre-action 死亡检查 `!unit.Alive || unit.HP <= 0` → skip
- 不会出现已死亡单位仍行动

### 同步召唤物击杀召唤者

- 同步召唤物行动中击杀召唤者
- `executeUnitTurn` 后 casterDeathPolicy 会在主行动者的下一轮 processBattleTurn 中处理
- 当前批次的剩余同步召唤物继续执行完毕
- 召唤者死亡后，剩余同步召唤物在下轮被 casterDeathPolicy 清理

### 波次切换 + persist 召唤物

- 进入新波次：`rebuildBattleOrder` → `reseedOrder`
- `battleRecomputeUnitStats` 在新波次初始化时重新扫描 while_active
- persist 召唤物身上的 `summon_sync_entrained` buff 保留（permanent）→ `SkipTurnOrder=true` → 继续从 Order 排除
- `FullOrder` 中保留其速度位置

### 饰品卸下后已有召唤物

- 已存在的同步召唤物不受影响（`SkipTurnOrder` 已被计算为 true）
- 召唤者失去 trinket buff → 新召唤物不再获得 entrained buff → 新召唤物正常进 Order
- 旧的同步召唤物在 entrained buff 失效后自动恢复进 Order
- 现有的 entrained buff 是 permanent（无 duration），但可以用波次切换后的全局 recompute 处理

这有个隐含问题：permanent buff 不会过期，`while_active` 扫描一直返回 true。需要确认 entrained buff 的生命周期策略。

**决策：entrained buff 用有限 duration（召唤物存活回合数）而非 permanent。** 当召唤物存活回合结束（SummonRemainTurns 耗尽、被系统清理），entrained buff 随召唤物一起移除，问题自然解决。若召唤物存活到战斗结束，entrained buff 也保留到战斗结束，没有副作用。

### 行动队列重建触发点

以下场景会触发 `reseedOrder`（需在 `FullOrder` + `Order` 模式下一致处理）：

| 触发点 | 触发方式 | 注意事项 |
|--------|----------|---------|
| `addBattleSummon` 召唤 | 显式 `Order=[]; rebuildOrderIfNeeded()` | 需要之前已算好 SkipTurnOrder |
| 装备/性格 clone 效果 | 显式 `Order=[]; rebuildOrderIfNeeded()` | 同上 |
| `battleNextActor` 选下一个行动者 | `rebuildOrderIfNeeded` | 自动处理 |
| 波次生成/切换 | `rebuildBattleOrder` | 自动处理 |
| `reseedOrderIfEmpty` | 仅在 Order 为空时 | 自动处理 |

---

## 破坏性评估

| 改动点 | 风险 | 理由 |
|--------|------|------|
| `battleUnitState` 加 `SkipTurnOrder` | 零 | 零值 false，所有现有单位无影响 |
| `battleRecomputeUnitStats` 加 2 个 effect kind | 零 | 无 attr，原路径被 continue 跳过 |
| `reseedOrder` 双队列 | 低 | 新增 `FullOrder` 不改变现有 `Order` 计算，过滤不影响排序 |
| `addBattleSummon` 尾部追加 | 低 | 条件分支，无饰品时完全不触发 |
| `processBattleTurn` 提取 `executeUnitTurn` | 中 | 纯重构，需回归验证 |
| `executeSyncedSummonTurns` 新函数 | 低 | 纯新增 |
| `nextEventBatch` parentSeq 提取 | 低 | 无 parentSeq 时 no-op |
| JSON 数据 | 低 | 纯新增，不修改已有记录 |

---

## 验证清单

1. `addBattleSummon` 在 source 有 trinket buff 时 → 召唤物 SkipTurnOrder=true 在 reseedOrder 之前
2. `addBattleSummon` 在 source 无 trinket buff 时 → 召唤物 SkipTurnOrder=false → Order 正常含该单位
3. `reseedOrder` 双队列一致性：FullOrder 含所有存活单位，Order 只含 SkipTurnOrder=false 的单位
4. 同步召唤物按速度降序行动，前一个行动击杀后一个 → 死亡重检捕获
5. 前端 N 个同步召唤物同时行动，轨迹同时渲染
6. 普通战斗全回归：reseedOrder/processBattleTurn/emit 序列不变
7. buff 过期 → SkipTurnOrder 清零 → 下次 reseedOrder 恢复入 Order
8. 无同步召唤物的战斗 → executeSyncedSummonTurns 不调用
