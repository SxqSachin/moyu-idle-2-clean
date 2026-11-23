# 召唤同步行动饰品 — 终版设计

## 目录

1. [需求概述](#一需求概述)
2. [系统评估](#二系统评估)
3. [架构设计](#三架构设计)
4. [详细设计](#四详细设计)
5. [Timeline 兼容](#五timeline-兼容)
6. [边界情况](#六边界情况)
7. [数据定义](#七数据定义)
8. [实现步骤](#八实现步骤)
9. [验证方案](#九验证方案)

---

## 一、需求概述

### 1.1 目标

新增战斗饰品。装备后，持有者召唤的召唤物**不加入行动队列**，改为在召唤者行动完毕后**同步行动**（完整回合：技能选择执行、冷却递减、buff 推进等全部正常发生），且前端体现为这些单位的动画同时播放，而不是每个单位依次播放，且同时行动时，在同步行动的batch中，需要包含每个单位的行动情况作为sub。

### 1.2 核心约束

1. **效果标记驱动**：引擎代码不判断具体 buff ID，仅通过 while_active 效果类型标记驱动
2. **Timeline 无感扩展**：同步召唤物事件通过 `parentSeq` 嵌套到主行动者事件下，前端可区分
3. **热路径零开销**：行为标记在 `battleRecomputeUnitStats` 预计算为 bool，热路径 O(1)
4. **通用回合抽象**：提取 `executeUnitTurn`，主循环和同步召唤物复用同一回合逻辑
5. **无缝上线**：无饰品时所有改动均为 no-op，纯新增 JSON 数据不修改已有记录
6. **性能优异**：战斗是预测性的，同一回合内的同步行动不能造成 CPU 尖峰

---

## 二、系统评估

### 2.1 时序分析

```
emit("battle.turn.action", ...)
  → state.Seq++              // Seq 先增
  → event.Seq = state.Seq    // 事件取增值后的 Seq
  → state.Events.append(event)
  // 返回后 state.Seq = 刚发射事件的 Seq

重要推论：emit 返回后，state.Seq 就是刚发射事件的 seq。
因此 mainActorSeq = state.Seq（不是 state.Seq+1）。
```

### 2.2 改动破坏性矩阵

| 改动 | 无饰品时行为 | 破坏性 |
|------|-------------|--------|
| `battleUnitState` +2 bool（零值 false） | 不变 | 零 |
| `battleRecomputeUnitStats` +behavior flag | 新 effect 无 attr，等价于原 `attr==""→continue` | 零 |
| `reseedOrder` +`!unit.SkipTurnOrder` | SkipTurnOrder=false → no-op | 零 |
| `addBattleSummon` +条件 buff | HasSyncedSummonTrigger=false → no-op | 零 |
| `processBattleTurn` 提取 `executeUnitTurn` | 纯重构 | 需回归 |
| `encodeBattleTimeline` +parentSeq 提取 | 无 parentSeq → no-op | 零 |
| JSON 数据追加 | 纯新增 | 零 |

### 2.3 Timeline 路径

战斗流程：预编译（`processBattleTurn` × N）→ `state.Events` 累积 → `encodeBattleTimeline` 编码 → 落库 → `decodeBattleTimeline` 解码 → 推前端。**唯一一条路径**。

`wsbus.BattleTimelineEvent`（`wsbus/event.go:34`）和前端 TS 类型（`web/src/types/ws.ts:18`）已定义 `ParentSeq` / `parentSeq` 字段。`decodeBattleTimeline` 也已有 parentSeq 读取逻辑。只需在 `encodeBattleTimeline` 中提取 parentSeq 即可打通全链路。

---

## 三、架构设计

### 3.1 总体数据流

```
饰品装备
  → on_battle_start hook → add_buff(summon_sync_trinket, target=self)
    → battleRecomputeUnitStats
      → 扫描 while_active → 发现 trigger_synced_summon_turn
      → actor.HasSyncedSummonTrigger = true

玩家使用召唤技能
  → addBattleSummon
    → 检测 sourceUnit.HasSyncedSummonTrigger
    → add_buff(summon_sync_entrained, target=summon)
    → battleRecomputeUnitStats(&summon)
      → 扫描 while_active → 发现 skip_turn_order
      → summon.SkipTurnOrder = true

reseedOrder
  → 收集存活单位时过滤 SkipTurnOrder
  → 同步召唤物不进入 state.Order

processBattleTurn
  → executeUnitTurn(mainActor)
  → casterDeathPolicy / cleanup
  → emit main turn action (seq=N)
  → mainActorSeq = state.Seq (缓存!)
  → if actor.HasSyncedSummonTrigger:
      executeSyncedSummonTurns(actor, mainActorSeq)
        → 收集 SkipTurnOrder && SummonSourceUnitID==actor 的召唤物
        → 按 speed 降序排序
        → 逐个: executeUnitTurn(summon) → payload["parentSeq"]=mainActorSeq → emit
  → advanceTurnCursor
```

### 3.2 分层职责

```
processBattleTurn          — 编排层：波次、胜负、游标
  ├── executeUnitTurn      — 单位回合层：通用回合执行（无场景感知）
  ├── casterDeathPolicy    — 全局维护层
  └── executeSyncedSummonTurns — 同步调度层：查找+排序+调用 executeUnitTurn
```

---

## 四、详细设计

### 4.1 `battleUnitState` 新增字段

`battle_runtime_engine.go` ~133 行：

```go
// 行为标记：由 battleRecomputeUnitStats 根据 while_active buff effects 预计算
// json:"-" 不序列化，每次 buff 变更后通过 battleRecomputeUnitStats 重新计算
SkipTurnOrder          bool `json:"-"`
HasSyncedSummonTrigger bool `json:"-"`
```

### 4.2 常量

`battle_runtime_engine.go` ~510 行：

```go
// while_active 效果类型标记（引擎仅判断这些标记，不依赖具体 buff ID）
battleEffectSkipTurnOrder           = "skip_turn_order"
battleEffectTriggerSyncedSummonTurn = "trigger_synced_summon_turn"

// buff ID（仅用于 addBattleSummon 中 add_buff 调用和 JSON 数据引用）
battleSummonSyncTrinketBuffID   = "summon_sync_trinket"
battleSummonSyncEntrainedBuffID = "summon_sync_entrained"
```

### 4.3 `battleRecomputeUnitStats` 注入

`battle_buff_runtime.go` ~793 行。两处改动：

**改动 A** — 循环前初始化：
```go
unit.SkipTurnOrder = false
unit.HasSyncedSummonTrigger = false
```

**改动 B** — effect kind 提取后、attr 提取前：
```go
// 行为标记效果（无 attr 字段，在此提前捕获）
// 原逻辑中这些 kind 会因 attr=="" 被 continue，提前处理等价且更高效
switch kind {
case battleEffectSkipTurnOrder:
    unit.SkipTurnOrder = true
    continue
case battleEffectTriggerSyncedSummonTurn:
    unit.HasSyncedSummonTrigger = true
    continue
}
```

**安全性**：新 effect kind 无 `attr` 字段。原逻辑走 `attr=="" → continue`，新逻辑走提前 `continue`。两条路径等价，零行为差异。

### 4.4 `reseedOrder`

`battle_runtime_engine.go` ~6814 行。**签名不变**（无需 *Server）。

```go
func (state *battleRuntimeState) reseedOrder() {
    units := make([]battleUnitState, 0, len(state.Players)+len(state.Enemies))
    for _, unit := range state.Players {
        if unit.Alive && unit.HP > 0 && !unit.SkipTurnOrder {
            units = append(units, unit)
        }
    }
    for _, unit := range state.Enemies {
        if unit.Alive && unit.HP > 0 && !unit.SkipTurnOrder {
            units = append(units, unit)
        }
    }
    // ... 排序逻辑完全不变（speed 降序，同速按 UnitID）...
}
```

### 4.5 `addBattleSummon`

`battle_runtime_engine.go` ~12482 行，`battleApplyOrRefreshBuff(&unit, battleSummonBindingBuffID, ...)` 之后：

```go
// 召唤者持有同步饰品 → 给召唤物附加从属标记 buff
if sourceUnit := battleUnitByID(state, strings.TrimSpace(summon.SourceUnitID)); sourceUnit != nil {
    if sourceUnit.HasSyncedSummonTrigger {
        remainTurns := summon.DurationTurns
        if remainTurns == 0 {
            remainTurns = -1
        }
        battleApplyOrRefreshBuff(&unit, battleSummonSyncEntrainedBuffID, remainTurns, nil, sourceUnit.UnitID, "refresh_duration")
    }
}
```

然后在追加 unit 到 Players/Enemies 后、清空 Order 前，调用 recompute：

```go
if side == "player" {
    state.Players = append(state.Players, unit)
    server.battleRecomputeUnitStats(&state.Players[len(state.Players)-1])
} else {
    state.Enemies = append(state.Enemies, unit)
    server.battleRecomputeUnitStats(&state.Enemies[len(state.Enemies)-1])
}
state.Order = []string{}
state.rebuildOrderIfNeeded()
```

`battleRecomputeUnitStats` 调用必须在 `rebuildOrderIfNeeded` → `reseedOrder` 之前，确保 `SkipTurnOrder` 在过滤时已正确设置。

### 4.6 `executeUnitTurn`

从 `processBattleTurn` 行 1547-1705 机械提取。**不改逻辑，仅移动代码**。

```go
// executeUnitTurn 执行一个单位的完整战斗回合。
// 不感知调用场景（主循环 / 同步召唤 / 未来扩展均复用）。
// 返回 actionPayload，调用方负责 emit。
func (s *Server) executeUnitTurn(
    state *battleRuntimeState,
    now time.Time,
    actor *battleUnitState,
    side string,
    isPreparationTurn bool,
) map[string]any
```

内部执行顺序（与 processBattleTurn 原代码完全对齐）：

| 步骤 | 原行号 | 内容 |
|------|--------|------|
| 1 | 1547 | `state.markUnitActed(actor.UnitID)` |
| 2 | 1550-1551 | `on_turn_start` buff + equipment hooks |
| 3 | 1554-1569 | `applyTurnRegen` + emit `battle.turn.regen` |
| 4 | 1570 | `battleUnitCheckTurnSuppressions` |
| 5 | 1571 | `tryExecuteBattleTurnItem` |
| 6 | 1572-1582 | 技能选择 + HP/MP 消耗扣除 |
| 7 | 1583-1630 | actionPayload 构建 + `executeBattleSkill`（或 idle） |
| 8 | 1633-1639 | `on_turn_end` buff + equipment hooks |
| 9 | 1641-1685 | `tickBattleBuffDurations` + 过期处理 + stat recalc |
| 10 | 1690-1696 | resultMeta + `populateTargetSlots` + 最终 HP/MP/Alive |
| 11 | 1701-1705 | 冷却预增 + `decrementBattleSkillCooldowns` + `decrementBattleItemCooldowns` |
| — | — | `return actionPayload` |

**排除**（保留在 processBattleTurn）：
- `battleApplySummonCasterDeathPolicy` (1686-1687) — 全局操作
- `battleCleanupSummonBindingForDeadUnits` (1689) — 全局操作
- `state.emit("battle.turn.action", ...)` (1706) — 调用方负责
- `advanceTurnCursor` (1707-1709) — 编排层职责

**turnProfile 计时调整**：提取后 processBattleTurn 在外层计时，细粒度 PreAction/Execute/PostAction 合并为单次 ExecuteMs。总耗时准确，调试性微降。

### 4.7 重构后的 `processBattleTurn`

```go
func (s *Server) processBattleTurn(...) (status string, advanced bool, err error) {
    // ====== 编排层：波次生成 / 胜负检测 ======
    // ... 行 1494-1544 完全不变 ...

    // ====== 主行动者回合 ======
    actionPayload := s.executeUnitTurn(state, now, actor, side, isPreparationTurn)

    // ====== 全局维护（每次主循环迭代仅执行一次）======
    if casterDeathActions := battleApplySummonCasterDeathPolicy(state); len(casterDeathActions) > 0 {
        actionPayload["actionLog"] = append(
            actionPayload["actionLog"].([]map[string]any), casterDeathActions...)
    }
    battleCleanupSummonBindingForDeadUnits(state)

    // ====== 发射主行动者事件 ======
    state.emit("battle.turn.action", now, actionPayload)
    // emit 内部先 state.Seq++ 再赋给事件，返回后 state.Seq == 刚发射事件的 seq
    mainActorSeq := state.Seq

    // ====== 同步召唤物回合 ======
    if actor.HasSyncedSummonTrigger {
        s.executeSyncedSummonTurns(state, now, actor, side, mainActorSeq)
    }

    // ====== 推进游标 ======
    if !isPreparationTurn {
        state.advanceTurnCursor(idx)
    }
    state.LastActionAt = now.UnixMilli()
    return
}
```

### 4.8 `executeSyncedSummonTurns`

```go
func (s *Server) executeSyncedSummonTurns(
    state *battleRuntimeState,
    now time.Time,
    summoner *battleUnitState,
    side string,
    parentSeq int64,
) {
    summonerID := strings.TrimSpace(summoner.UnitID)

    // 仅遍历召唤者所在 side
    units := state.Players
    unitSide := "player"
    if side == "enemy" {
        units = state.Enemies
        unitSide = "enemy"
    }

    // step 1: 收集符合条件的同步召唤物
    candidates := make([]*battleUnitState, 0)
    for i := range units {
        unit := &units[i]
        if !unit.Alive || unit.HP <= 0 { continue }
        if !unit.SkipTurnOrder { continue }
        if strings.TrimSpace(unit.SummonSourceUnitID) != summonerID { continue }
        candidates = append(candidates, unit)
    }
    if len(candidates) == 0 { return }

    // step 2: 按 speed 降序排序（与 reseedOrder 规则一致）
    sort.SliceStable(candidates, func(i, j int) bool {
        si := candidates[i].Stats["speed"]
        sj := candidates[j].Stats["speed"]
        if si == sj {
            return candidates[i].UnitID < candidates[j].UnitID
        }
        return si > sj
    })

    // step 3: 逐个执行，每个前重新检查存活
    for _, unit := range candidates {
        if !unit.Alive || unit.HP <= 0 { continue }
        payload := s.executeUnitTurn(state, now, unit, unitSide, false)
        payload["parentSeq"] = parentSeq
        state.emit("battle.turn.action", now, payload)
    }
}
```

**设计要点**：
- 排序确保行动次序确定性（与普通队列规则一致）
- 每个候选执行前重新检查存活：前置同步召唤物的技能可能误伤后排召唤物

---

## 五、Timeline 兼容

战斗流程：预编译 → `encodeBattleTimeline` 编码 → 落库 → `decodeBattleTimeline` 解码 → 推前端。只需在编码环节传递 parentSeq。

### 5.1 `encodeBattleTimeline` 改动

`battle_runtime_engine.go` ~5139 行。`battle.turn.action` 处理分支末尾加一行：

```go
case "battle.turn.action":
    data := cloneAnyMap(event.Data)
    // ... 现有 regenData / itemData 提取 + buildBattleTimelineNodeData + appendTimelineEvent 不变 ...
    nextNodeOffset += battleTimelineNodeDurationMs(nodeData) + nodeIntervalMs

    // 同步召唤物事件：传递 parentSeq 到 timeline item
    if ps, ok := event.Data["parentSeq"].(float64); ok {
        out[len(out)-1]["parentSeq"] = int64(ps)
    }
```

**一行改动**。`event.Data` 是 `actionPayload` 的 clone，同步召唤物的 payload 中含 `parentSeq`。无饰品时 `parentSeq` 不存在，`ok == false`，no-op。

### 5.2 `decodeBattleTimeline` — 无需改动

`decodeBattleTimeline` (line 5175) **已有** parentSeq 读取：
```go
if v := item["parentSeq"]; v != nil {
    ps := toInt64Loose(v, -1)
    if ps >= 0 { ev.ParentSeq = &ps }
}
```

编解码链路完整：`encodeBattleTimeline` 写入 → 落库 → `decodeBattleTimeline` 读出 → `BattleTimelineEvent.ParentSeq` 设置完成。

### 5.3 前端

TS 类型已有 `parentSeq?: number`（`web/src/types/ws.ts:18`）。播放时 `parentSeq != null` 则紧跟父事件无延迟播放。无饰品的战斗 `parentSeq` 始终 omit，不受影响。

---

## 六、边界情况

| 场景 | 行为 | 安全机制 |
|------|------|----------|
| 无饰品 | SkipTurnOrder/HasSyncedSummonTrigger 均为 false，所有分支不触发 | 零值 bool |
| 召唤者当回合死亡 | casterDeathPolicy 先执行 → 召唤物被 despawn → executeSyncedSummonTurns 时已死，skip | 执行顺序保证 |
| 同步召唤物被击杀 | pre-action `Alive && HP>0` 检查 | executeSyncedSummonTurns 过滤 |
| 同步召唤物被眩晕/沉默/缴械 | executeUnitTurn 内 `battleUnitCheckTurnSuppressions` → idle | 与普通单位一致 |
| 同步召唤物相互击杀 | 按 speed 降序行动，前置击杀后置 → 后置 pre-action 检查 skip | 排序 + 存活检查 |
| 同步召唤物再次召唤 | 子召唤物 source=召唤物(A)，不为 summonerID。A 在下一轮同步行动时其子召唤物会被找到 | SummonSourceUnitID 比对保证 |
| PVP 双方均有饰品 | 敌方召唤物 source=敌方 unitID，不会被我方回合触发 | SummonSourceUnitID 比对保证 |
| buff 净化/驱散 | 两个 buff 均 `dispellable: false` | buff 定义保证 |
| 饰品战斗中卸下 | 当前系统战斗装备为快照机制（`buildBattleKittySnapshot` 一次性读取），战斗中无装备变更路径。此条目为未来缴械系统的防御性文档：`summon_sync_trinket` 移除 → HasSyncedSummonTrigger=false，已存在召唤物的 SkipTurnOrder 独立不变 | 快照机制 + 独立标记字段 |
| 波次切换 | 非 persist 召唤物丢弃。persist 召唤物的 buff 保留（permanent），下波次初始化时 recompute 重新设置 SkipTurnOrder | battleRecomputeUnitStats 在 initBattleRuntimeState 调用 |
| 技能经验结算 | `battleSkillExpRewardsFromTimeline` 处理 `battle.turn.action` 事件，同步召唤物同类型 → 正确计入 | event type 一致 |
| `battleFilterPreparationOrder` | 该函数已在 line 6627 显式跳过 `battleUnitIsSummon`。同步召唤物也是 summon → 自然被跳过 | 已有逻辑 |
| Timeline 编解码 | `encodeBattleTimeline` 写入 parentSeq → 落库 → `decodeBattleTimeline` 读出 → 前端播放 | 唯一路径，覆盖完整 |

---

## 七、数据定义

### 7.1 `buff.json`

```json
{
    "id": "summon_sync_trinket",
    "nameKey": "buff.summon_sync_trinket.name",
    "descKey": "buff.summon_sync_trinket.desc",
    "durationTurns": 0,
    "permanent": true,
    "kind": "buff",
    "dispellable": false,
    "type": "accessory",
    "icon": "game-icons:linked-rings",
    "disabled": false
}
```

```json
{
    "id": "summon_sync_entrained",
    "nameKey": "buff.summon_sync_entrained.name",
    "descKey": "buff.summon_sync_entrained.desc",
    "durationTurns": 0,
    "permanent": true,
    "kind": "buff",
    "dispellable": false,
    "type": "accessory",
    "icon": "game-icons:teleport",
    "disabled": false
}
```

### 7.2 `buff_effects.json`

```json
{
    "id": "summon_sync_trinket_wa",
    "buffId": "summon_sync_trinket",
    "trigger": "while_active",
    "priority": 0,
    "conditions": [],
    "effects": [{ "kind": "trigger_synced_summon_turn" }],
    "disabled": false
}
```

```json
{
    "id": "summon_sync_entrained_wa",
    "buffId": "summon_sync_entrained",
    "trigger": "while_active",
    "priority": 0,
    "conditions": [],
    "effects": [{ "kind": "skip_turn_order" }],
    "disabled": false
}
```

### 7.3 `equipment_templates.json`

```json
{
    "id": "summon_sync_trinket_001",
    "slot": "accessory",
    "equipType": "battle",
    "baseAttrs": { "speed": 5, "mp_regen": 3 },
    "battleHooks": [{
        "id": "summon_sync_trinket_on_start",
        "trigger": "on_battle_start",
        "priority": 0,
        "conditions": [],
        "effects": [{
            "kind": "add_buff",
            "buffId": "summon_sync_trinket",
            "durationTurns": 0,
            "targetMode": "self"
        }]
    }],
    "maxDurability": 200,
    "enhanceMaxLevel": 50,
    "enhanceDurationS": 10,
    "enhanceProtectItemId": ["moon_essence"],
    "enhanceCostCurve": {
        "base": { "base4_shadow_iron": 1, "gold": 2 },
        "step": { "base4_shadow_iron": 1, "gold": 1 },
        "overrides": {}
    },
    "enhanceExp": 5,
    "enhanceAttrCurve": {
        "base": { "speed": 2, "mp_regen": 1 },
        "step": { "speed": 1 },
        "overrides": {
            "10": { "speed": 5 },
            "25": { "speed": 8 },
            "40": { "speed": 12 },
            "50": { "speed": 18 }
        }
    },
    "disabled": false
}
```

### 7.4 `items.json`

```json
{
    "id": "summon_sync_trinket_001",
    "type": ["equipment"],
    "rarity": "rare",
    "isEquipment": true,
    "baseAttrs": { "speed": 5, "mp_regen": 3 },
    "stackable": false,
    "carryableInCombat": false,
    "marketListable": true,
    "iconId": "item.summon_sync_trinket_001",
    "nameKey": "item.summon_sync_trinket_001.name",
    "descKey": "item.summon_sync_trinket_001.desc",
    "effectDescKey": "item.summon_sync_trinket_001.effect",
    "economyBaseValue": 2500,
    "useSkillExpGain": 0,
    "disabled": false
}
```

### 7.5 `combat-model-vue3/src/App.vue`

```typescript
// BUFF_EFFECT_KINDS 追加
'skip_turn_order',
'trigger_synced_summon_turn',

// OPTION_LABEL_OVERRIDES 追加
skip_turn_order: '跳过行动队列',
trigger_synced_summon_turn: '触发同步召唤物行动',
```

---

## 八、实现步骤

### 阶段一：纯重构（零行为变更）

| 步 | 文件 | 内容 |
|----|------|------|
| 1 | `battle_runtime_engine.go` | 新增 `SkipTurnOrder`、`HasSyncedSummonTrigger` 字段（零值 bool） |
| 2 | `battle_runtime_engine.go` | 新增 4 个常量 |
| 3 | `battle_buff_runtime.go` | `battleRecomputeUnitStats` 注入行为标记计算（零行为变更） |
| 4 | `battle_runtime_engine.go` | 提取 `executeUnitTurn`（纯机械复制），重构 `processBattleTurn` 调用它 |
| 5 | — | **回归验证**：相同输入 → 相同 actionPayload + emit 序列 |

### 阶段二：新增功能

| 步 | 文件 | 内容 |
|----|------|------|
| 6 | `battle_runtime_engine.go` | `reseedOrder` 加 `SkipTurnOrder` 过滤 |
| 7 | `battle_runtime_engine.go` | `addBattleSummon` 加从属 buff 逻辑 + recompute |
| 8 | `battle_runtime_engine.go` | 新增 `executeSyncedSummonTurns` |
| 9 | `battle_runtime_engine.go` | `processBattleTurn` 加同步召唤物分支 |
| 10 | `battle_runtime_engine.go` | `encodeBattleTimeline` 加 parentSeq 提取（一行改动） |

### 阶段三：数据与 UI

| 步 | 文件 | 内容 |
|----|------|------|
| 12 | `buff.json` | 新增 2 个 buff |
| 13 | `buff_effects.json` | 新增 2 条 while_active 规则 |
| 14 | `equipment_templates.json` | 新增饰品模板 |
| 15 | `items.json` | 新增物品定义 |
| 16 | `App.vue` | DSL 编辑 UI 常量追加 |
| 17 | — | 运行 game-data 生成脚本 |

### 阶段四：验证

| 步 | 内容 |
|----|------|
| 18 | 无饰品全量战斗回归 |
| 19 | 装备饰品 + 召唤 → 端到端功能验证 |
| 20 | 编译战斗 → 确认 parentSeq 编码 + 落库 + 解码正确 |
| 21 | 性能基准对比 |

---

## 九、验证方案

### 9.1 正确性

- [ ] `executeUnitTurn` 逐行对比原始 `processBattleTurn`，确认无遗漏
- [ ] 无饰品：100 回合战斗 timeline 重构前后逐帧一致
- [ ] 无饰品：actionPayload 结构完全一致
- [ ] 无饰品：所有现有 buff/skill/equipment 行为不变
- [ ] `battleRecomputeUnitStats` 新 continue 不跳过任何原有 stat 计算
- [ ] `reseedOrder` 过滤在 SkipTurnOrder=false 时为 no-op
- [ ] `addBattleSummon` 分支在 HasSyncedSummonTrigger=false 时不触发
- [ ] `encodeBattleTimeline` parentSeq 提取在无 parentSeq 时为 no-op

### 9.2 功能

- [ ] 装备饰品 → 召唤 → SkipTurnOrder=true，召唤物不在 Order 中
- [ ] 召唤物按 speed 降序依次行动
- [ ] 召唤物完整回合：技能冷却递减、buff 持续递减、on_turn_start/end 触发
- [ ] Timeline：主 actor seq=N，召唤物 parentSeq=N（encode → 落库 → decode 链路完整）
- [ ] 召唤者当回合死亡 → 召唤物不行动
- [ ] 召唤物被眩晕 → idle

### 9.3 性能

- [ ] `reseedOrder` 无新增 map lookup
- [ ] `processBattleTurn` 热路径仅 1 次 bool 检查
- [ ] `battleRecomputeUnitStats` 增量 < 0.1%
- [ ] 100 召唤物同步行动时无 CPU 尖峰（召唤物数量上限由 MaxCountPerUnit 限制，通常 ≤5）
