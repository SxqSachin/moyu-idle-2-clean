# Excel 数据规则（策划）

## TL;DR（先读这里）

- 本文定义 `packages/game-data/source/workbooks/*.xlsx` 的字段规范与填写约束。
- 必守规则：首行字段名、JSON 字段必须合法 JSON、`id` 不随版本随意改名、文案优先 i18n key。
- 开发与策划协作时，优先按“工作簿 -> sheet -> 字段表”定位，不需要通读全文。

## 何时阅读本文

- 新增/调整 Excel 字段结构。
- 排查导表失败、字段缺失、i18n key 生成不符合预期。

## 常见任务直达

- 动作配置：看 `actions.xlsx`
- 物品配置：看 `items.xlsx`
- 战斗物品配置：看 `battle_usable_items.xlsx`
- i18n key 规则：看“总体规则”与各 sheet 默认 key 说明

本文档定义 `packages/game-data/source/workbooks/*.xlsx` 的可用字段与填写约束。

## 1. 总体规则

- 所有工作簿首行必须是字段名。
- 标记为 JSON 的字段，单元格内容必须是合法 JSON（以 `{` 或 `[` 开头）。
- `id` 字段必须全局语义稳定，不要随版本重命名历史已上线 id。
- 可见文案建议走 i18n key（`nameKey/descKey/effectDescKey`），不要直接写多语言文案。

## 2. 当前工作簿与字段

## 2.1 actions.xlsx

### actions

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | string | 动作唯一标识 |
| `mainType` | string | 动作主类（用于动态菜单分组） |
| `subType` | string | 动作子类（未填时构建阶段补为 `common`） |
| `iconId` | string | 动作图标 id（可选，建议 `action.xxx`） |
| `nameKey` | string | i18n key（如 `actions.exercise.running`） |
| `descKey` | string | 动作描述 i18n key（可选） |
| `durationS` | number | 基础耗时（秒） |
| `queueable` | boolean | 是否允许排队 |
| `economyIndustryTier` | number | 工业层级倍率（默认 `1`） |
| `economyDifficultyMultiplier` | number | 难度倍率（默认 `1`） |
| `costs` | JSON object | 消耗物品映射，如 `{"gold_coin": 5}` |
| `itemReward` | JSON object | 产出物品映射 |
| `statusExpReward` | JSON object | 属性经验映射（`str/agi/vit/int/luck`） |

默认 i18n key 规则（未显式填写时自动生成）：

- `nameKey` -> `action.{id}.name`
- `descKey` -> `action.{id}.desc`
- `iconId` -> `action.{id}`

### action_main_types

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | string | 主类 id（与 actions.mainType 对应） |
| `nameKey` | string | 主类名称 i18n key |
| `descKey` | string | 主类描述 i18n key（可选） |
| `icon` | string | 菜单图标（可为空） |
| `color` | string | 菜单色值（可为空） |
| `order` | number | 菜单排序 |
| `actions` | JSON array | 可选，主类下 action id 列表（可由构建自动推导） |

默认 i18n key 规则（未显式填写时自动生成）：

- `nameKey` -> `action_main_type.{id}.name`
- `descKey` -> `action_main_type.{id}.desc`

### action_sub_types

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | string | 子类 id |
| `nameKey` | string | 子类名称 i18n key |
| `descKey` | string | 子类描述 i18n key（可选） |
| `icon` | string | 子类图标（可为空） |
| `color` | string | 子类色值（可为空） |
| `order` | number | 子类排序 |
| `actions` | JSON array | 可选，子类下 action id 列表（可由构建自动推导） |

默认 i18n key 规则（未显式填写时自动生成）：

- `nameKey` -> `action_sub_type.{id}.name`
- `descKey` -> `action_sub_type.{id}.desc`

## 2.2 items.xlsx

### items

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | string | 物品模板 id |
| `type` | string | 物品类型（weapon/armor/currency/consumable...） |
| `rarity` | string | 稀有度（引用 `rarity_definitions.id`） |
| `stackable` | boolean | 是否可堆叠 |
| `carryableInCombat` | boolean | 战斗中可携带 |
| `iconId` | string | 图标 id（可选，建议 `item.{id}`） |
| `emojiIcon` | string | emoji 图标（可选） |
| `nameKey` | string | i18n key（可选） |
| `descKey` | string | i18n key（可选） |
| `effectDescKey` | string | i18n key（可选） |
| `economyBaseValue` | number | 物品基础价值锚点（策划填写，默认 `0`） |
| `skillForgeEligible` | boolean | 技能炼成资格；`loot` 省略时沿用默认开放，显式 `false` 可关闭，生活材料需显式 `true` 开放 |
| `useGrantSkillId` | string | 使用后授予技能 id（可选） |
| `useSkillExpGain` | number | 使用后技能经验（可选） |
| `battleUsableItemId` | string | 战斗可用物品关系 id（关联 `battle_usable_items.id`，可选） |

补充规则：
- 允许在同一工作簿中使用 `items_*` 命名的扩展 Sheet（如 `items_weapon`、`items_material`）。
- `build.mjs` 会把 `items` 与全部 `items_*` Sheet 按行合并后，统一按 `items` 字段规则解析。
- `item_types` 是独立定义表，不参与 `items_*` 合并。

默认 i18n key 规则（未显式填写时自动生成）：

- `nameKey` -> `item.{id}.name`
- `descKey` -> `item.{id}.desc`
- `effectDescKey` -> `item.{id}.effect`

### battle_usable_items

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | string | 战斗物品定义 id |
| `itemId` | string | 对应物品模板 id |
| `cooldownTurns` | number | 冷却回合数 |
| `effects` | JSON array | 效果数组，结构尽量与 `skills.effects` 对齐 |
| `targeting` | JSON object | 可选目标逻辑；默认自我优先 |
| `ruleHint` | string | 可选规则提示文案 key |

补充规则：
- `itemId` 必须命中 `items.id`，且对应物品通常应设置 `carryableInCombat=true`。
- `battleUsableItemId` 与 `battle_usable_items.id` 形成一对一关系，构建阶段会校验有效性。
- `effects` 建议优先复用技能效果子集，避免战斗物品另写一套执行语义。

### item_types

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | string | 物品类型 id（需与 `items.type` 对应） |
| `nameKey` | string | 类型名称 i18n key |
| `descKey` | string | 类型描述 i18n key（可选） |

默认 i18n key 规则（未显式填写时自动生成）：

- `nameKey` -> `item_type.{id}.name`
- `descKey` -> `item_type.{id}.desc`

说明：
- `items.xlsx` 不再维护 `baseAttrs`，装备基础属性统一放到 `equipment.xlsx.equipment_templates.baseAttrs`。

## 2.3 equipment.xlsx

### equipment_templates

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | string | 装备模板 id（应与可装备物品 id 对齐） |
| `equipType` | string | 装备类型：`battle`（战斗）/`work`（生活），默认 `battle` |
| `slot` | string | 装备槽位（如 `weapon` / `armor` / `helmet` / `accessory`） |
| `occupySlots` | JSON array | 实际占用槽位（可选；为空则按选择槽位） |
| `weaponClass` | string | 武器类别（逗号分隔或 JSON array，构建后归一化为数组） |
| `statusRequirement` | JSON object | 穿戴属性门槛（可选） |
| `accessoryGroup` | string | 饰品互斥组（可选） |
| `affixSlotCount` | number | 词条槽位数量（首版默认全部可用，不做解锁） |
| `affixAllowedIds` | JSON array | 装备可用词条白名单（可选；为空表示不限制，预留过滤口子） |
| `baseAttrs` | JSON object | 基础装备属性 |
| `maxDurability` | number | 最大耐久（默认 100） |
| `decayRate` | number | 耐久衰减速率（可选，默认 0） |
| `enhanceMaxLevel` | number | 最大强化等级（`0` 表示不可强化） |
| `enhanceDurationS` | number | 单次强化耗时（秒） |
| `enhanceExp` | number | 每次强化提供给 `enhance` 属性的经验（成功/失败都发放） |
| `enhanceProtectItemId` | string | 保护道具模板 id 列表（可选，英文逗号分隔） |
| `enhanceCostCurve` | JSON object | 强化材料消耗曲线（见下方结构） |
| `enhanceAttrCurve` | JSON object | 强化属性增量曲线（见下方结构） |
| `enhanceAttrMilestoneOverrides` | JSON object | 里程碑强化增量覆盖列（`level -> attrMap`），构建时合并到 `enhanceAttrCurve.overrides` |
| `enhanceAffixMilestonePools` | JSON object | 强化附魔池列（`level -> [{affixId, rarityId, rollValue?}]`），单元素数组等价固定词条 |
| `battleHooks` | JSON array | 装备战斗钩子（如受伤后加 buff） |
| `defaultAffixSlots` | JSON array | 装备默认词条槽位装配（新生成实例时注入） |
| `basicAttackEffects` | JSON array | 武器普攻效果数组，结构与 `skills.effects` 对齐；优先使用 |
| `basicAttackTargetDirection` | string | 兼容保留字段（Deprecated）：默认目标位置（`vanguard/midguard/rearguard`）；`weapon-tool` 不再编辑/生成 |
| `targetSeekStrategy` | string | 兼容保留字段（Deprecated）：寻敌方向（`rear_to_front/front_to_rear/random_alive`）；`weapon-tool` 不再编辑/生成 |
| `basicAttackFormula` | JSON object | 兼容保留字段（Deprecated）：旧版普攻公式；`weapon-tool` 不再编辑/生成，统一改用 `basicAttackEffects` |

补充规则：
- 允许在同一工作簿中使用 `equipment_templates*` 命名的扩展 Sheet（如 `equipment_templates_weapon`、`equipment_templates_armor`）。
- `build.mjs` 会把全部 `equipment_templates*` Sheet 按行合并后，统一按 `equipment_templates` 字段规则解析。
- 若多个 `equipment_templates*` Sheet 出现重复 `id`，构建阶段会直接报错（不做覆盖合并）。
- `battleHooks` 当前支持：
  - `trigger`: `on_after_take_damage` / `on_battle_start`（每个新波次开场前：全单位初始化后、首回合前触发一次）
  - `effects[].kind`: `add_buff`（需提供 `buffId`，可选 `durationTurns`）
  - `effects[].targetMode`: `self|source|targeting`
  - `effects[].targeting`: 复用 `skills.effects[].targeting` 结构（当 `targetMode=targeting` 时生效）
- `defaultAffixSlots` 用于声明“新实例默认携带词条”，结构示例：
  - `[{"slotIndex":0,"affixId":"battle_taunt_start","rarityId":"rare","rollValue":0}]`
  - `slotIndex` 必须命中 `0..affixSlotCount-1` 且同模板内不能重复。
  - `affixId` 必须命中启用中的 `affix_definitions.id`。
  - `rarityId` 必须命中 `rarity_definitions.id` 且满足该词条 `allowedRarities`（若该词条配置了限制）。

强化曲线结构：

{
  overrides: {
    [level: string]: number // percent eg. 0.03 / 0.1
  }
}


- `enhanceCostCurve`（map 曲线）
  - `base: { [itemId: string]: int }`
  - `step: { [itemId: string]: int }`
  - `overrides: { [level: string]: { [itemId: string]: int } }`
- `enhanceAttrCurve`（map 曲线）
  - `base: { [attrKey: string]: number }`
  - `step: { [attrKey: string]: number }`
  - `overrides: { [level: string]: { [attrKey: string]: number } }`
- `enhanceAttrMilestoneOverrides`（Excel 单独编辑入口）
  - `{ [level: string]: { [attrKey: string]: number } }`
  - 所有 `equipment_templates*` 分表均生效（如 `equipment_templates_weapon`）
  - 构建时写入 `enhanceAttrCurve.overrides`；若同等级在两处都配置，`enhanceAttrMilestoneOverrides[level]` 优先
- `enhanceAffixMilestonePools`（Excel 单独编辑入口）
  - `{ [level: string]: [{ affixId: string, rarityId: string, rollValue?: number }] }`
  - 达到该等级时，从池子里随机取 1 个词条；如果数组只有 1 项，就是固定词条
  - `rollValue` 可选，不填时由运行时随机生成
  - 若触发时没有空词条槽位，则按强化取消语义直接取消本次强化
  - 同一装备实例同一里程碑等级只触发一次，重复上下强化不会重复获得

强化补充规则：

- 尝试索引使用 `nextLevel = currentLevel + 1`。
- `enhanceCostCurve` 公式：`value = base + step * (level - 1)`；`overrides[level]` 优先级最高。
- `enhanceAttrCurve` 语义：
  - 每级增量：`level=1` 取 `base`，`level>1` 取 `step`；`overrides[level]` 作为该级增量覆盖值。
  - 总加成：按 1..level 的“每级增量”累加（因此在无 override 时等价 `base + (level - 1) * step`）。
- `enhanceAttrMilestoneOverrides` 仅是 `enhanceAttrCurve.overrides` 的 Excel 便捷入口，不会改变运行时语义（仍是“该级替换、逐级累计”）。
- 强化基础成功率不再来自 `enhanceSuccessCurve`，统一由 `rarity + enhanceMaxLevel + level` 公式计算（见 `docs/enhancement_success_rate_design.md`），再叠加玩家 `enhancement_success_rate` 加成。
- `enhanceMaxLevel` 必须是 `0~1000` 的整数；`enhanceDurationS` 必须为正整数。
- `enhanceExp` 必须是有限数且 `>= 0`。
- `overrides` 的 key 必须是合法正整数等级，且 `<= enhanceMaxLevel`。
- `enhanceCostCurve` 所有值必须是有限数且最终为非负整数（禁止 `NaN/Inf/负数`）。
- `enhanceAttrCurve` 所有值必须为有限数（禁止 `NaN/Inf`）。
- 成功率运行时会 clamp 到 `[0, 1]`，并保留 4 位小数。

示例：

```json
{
  "enhanceMaxLevel": 10,
  "enhanceDurationS": 8,
  "enhanceExp": 1.5,
  "enhanceProtectItemId": "protect_scroll,moon_essence",
  "enhanceCostCurve": {
    "base": { "gold_coin": 1000, "enhance_stone": 1 },
    "step": { "gold_coin": 200, "enhance_stone": 1 },
    "overrides": { "7": { "gold_coin": 5000, "enhance_stone": 6 } }
  },
  "enhanceAttrCurve": {
    "base": { "atk": 2, "def": 1 },
    "step": { "atk": 1.5, "def": 0.8 },
    "overrides": { "10": { "atk": 25, "def": 12 } }
  },
  "enhanceAttrMilestoneOverrides": {
    "10": { "physics_damage": 12, "magical_damage": 12 },
    "20": { "physics_damage": 18, "magical_damage": 18 }
  },
  "basicAttackTargetDirection": "vanguard",
  "targetSeekStrategy": "front_to_rear",
  "basicAttackFormula": {
    "damageType": "physical",
    "formula": [
      { "attrKey": "strength", "coefficient": 0.6 },
      { "attrKey": "dexterity", "coefficient": 0.4 }
    ]
  }
}
```



武器案例（equipment_templates）：

```json
[
  {
    "id": "long_sword",
    "slot": "weapon",
    "basicAttackTargetDirection": "vanguard",
    "targetSeekStrategy": "front_to_rear",
    "basicAttackFormula": { "damageType": "physical", "formula": [{ "attrKey": "strength", "coefficient": 1 }] }
  },
  {
    "id": "war_bow",
    "slot": "weapon",
    "basicAttackTargetDirection": "rearguard",
    "targetSeekStrategy": "rear_to_front",
    "basicAttackFormula": { "damageType": "physical", "formula": [{ "attrKey": "strength", "coefficient": 1 }] }
  },
  {
    "id": "iron_spear",
    "slot": "weapon",
    "basicAttackTargetDirection": "midguard",
    "targetSeekStrategy": "front_to_rear",
    "basicAttackFormula": { "damageType": "physical", "formula": [{ "attrKey": "strength", "coefficient": 1 }] }
  },
  {
    "id": "apprentice_staff",
    "slot": "weapon",
    "basicAttackTargetDirection": "rearguard",
    "targetSeekStrategy": "random_alive",
    "basicAttackFormula": { "damageType": "magical", "formula": [{ "attrKey": "intelligence", "coefficient": 1 }] }
  }
]
```

普通攻击公式示例（`basicAttackFormula`）：

1. 用力量（x1）造成物理伤害

```json
{ "damageType": "physical", "formula": [{ "attrKey": "strength", "coefficient": 1 }] }
```

2. 用敏捷（x0.4）+ 力量（x0.6）造成物理伤害

```json
{
  "damageType": "physical",
  "formula": [
    { "attrKey": "dexterity", "coefficient": 0.4 },
    { "attrKey": "strength", "coefficient": 0.6 }
  ]
}
```

3. 用智力（x1.2）造成魔法伤害

```json
{ "damageType": "magical", "formula": [{ "attrKey": "intelligence", "coefficient": 1.2 }] }
```

### affix_definitions（词条定义）

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | string | 词条 id |
| `effectType` | string | 效果类型（如 `attr_bonus` / `battle_trigger` / `effectMutation`） |
| `effectParams` | JSON object | 效果参数（业务侧按 `effectType` 解析） |
| `type` | string | 词条类型：`work` / `battle`（必填） |
| `valuePrecision` | number | 数值精度（每个词条独立配置） |
| `allowedRarities` | JSON array | 该词条可出现的稀有度 id 列表 |
| `active` | boolean | 是否启用 |

补充规则：
- `nameKey/descKey` 不再由表维护，构建时按 `id` 自动生成：
  - `id` 以 `affix.` 开头时：`nameKey={id}.name`，`descKey={id}.desc`
  - 否则：`nameKey=affix.{id}.name`，`descKey=affix.{id}.desc`
- `allowedRarities=[]` 表示该词条可被任意稀有度抽中。
- `type` 必须是 `work|battle`，构建阶段会强校验；缺失或非法值直接报错。
- 装备与词条类型必须一致：`work` 装备只能使用 `work` 词条，`battle` 装备只能使用 `battle` 词条。
- `effectParams` 支持按稀有度分层配置，不强依赖 `allowedRarities`；示例：

```json
{
  "attrMods": {
    "rare": {
      "strength": { "min": 1, "max": 4 },
      "dexterity": { "min": 2, "max": 5 }
    },
    "epic": {
      "strength": { "min": 5, "max": 8 },
      "dexterity": { "min": 6, "max": 12 }
    }
  }
}
```
 - `effectType=battle_trigger` 时，`effectParams` 需包含 `battleHooks`（数组），结构复用 `equipment_templates.battleHooks`：
   - `trigger`: `on_battle_start | on_after_take_damage`
   - `effects[].kind`: 目前仅支持 `add_buff`
   - 示例：
```json
{
  "battleHooks": [
    {
      "trigger": "on_battle_start",
      "priority": 0,
      "conditions": [],
      "effects": [
        { "kind": "add_buff", "buffId": "taunt", "durationTurns": 4, "targetMode": "self" }
      ]
    }
  ]
}
```
- `effectType=effectMutation` 时：
  - `effectParams` 必须包含 `effectMutations`（数组）；
  - `effectParams` 不允许混入其他业务键（如 `attrMods` / `battleHooks`）。
{
  "attrMods": {
    "uncommon": { "strength": { "min": 2, "max": 5 }, },
    "rare": { "strength": { "min": 5, "max": 9 }, },
    "epic": { "strength": { "min": 9, "max": 12 }, },
    "legendary": { "strength": { "min": 12, "max": 20 }, }
  }
}


### affix_detach_rules（词条拆卸业务规则）

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | string | 规则 id |
| `toolItemId` | string | 拆卸道具物品 id（如 `craftsmans_hammer` / `hammer_of_divine_work`） |
| `destroyChanceBase` | number | 基础装备销毁概率（0~1） |
| `luckImpactFactor` | number | 幸运值影响系数（首版可设 0） |
| `affixDropBeforeDestroyCheck` | boolean | 是否“先产出词条再判定销毁” |
| `active` | boolean | 是否启用 |

补充规则：
- 拆卸时必须同时选择一个词条载体实例（见 `affix_carrier_rules`）。
- 载体实例必须在背包内、数量为 `1`、且当前未附着词条。
- 拆卸成功后，词条会写入所选载体实例（原地写入，不创建新实例）。

### affix_enchant_rules（词条附魔业务规则）

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | string | 规则 id |
| `toolItemId` | string | 附魔材料物品 id |
| `consumeCount` | number | 单次附魔消耗数量 |
| `costs` | JSON object | 额外资源消耗，结构与 `actions.costs` / 强化 cost 输出一致，空对象表示无额外消耗 |
| `failChance` | number | 附魔失败概率（0~1） |
| `rarityWeights` | JSON object | 稀有度权重，如 `{"common": 60, "rare": 30}` |
| `allowedAffixIds` | string / JSON array | 候选词条 id 白名单（推荐英文逗号分隔字符串，如 `affix.a,affix.b`；兼容 JSON array） |
| `allowedAffixScopes` | JSON array | 按装备类型/部位细分附魔池（可选，见下方结构） |
| `active` | boolean | 是否启用 |

补充规则：
- 稀有度抽取只在“`rarityWeights` 中权重大于 0 且当前候选词条可支持”的稀有度上进行。
- 若配置了不可用稀有度，会自动忽略并对剩余可用稀有度重算权重。
- 若没有任何可用稀有度交集，本次附魔直接失败（不会回退到未配置稀有度）。
- `toolItemId + consumeCount` 仍然是附魔触发道具；`costs` 仅表示额外资源消耗，允许为空 `{}`。
- `allowedAffixIds` 推荐填写示例：`affix.strength,affix.dexterity,affix.work_speed_minor`。
- `allowedAffixScopes` 用于覆盖默认池，结构为：
  - `equipType`: `battle|work`（必填）
  - `slots`: `string[]`（可空；为空表示该 equipType 默认池）
  - `weaponClasses`: `string[]`（可空；为空表示该 scope 不限制武器类别）
  - `allowedAffixIds`: `string[]`（可空；为空表示该 scope 不额外限制）
- 命中优先级：`equipType+slot+weaponClass` > `equipType+slot` > `equipType+weaponClass` > `equipType` > 回退 `allowedAffixIds`。
- 同一优先级命中多个 scope 时，`allowedAffixIds` 做并集去重。
- `allowedAffixScopes.slots` 必须使用 `equipment_templates.slot` 原始值（如 `right_hand`/`head`）。
- `allowedAffixScopes.weaponClasses` 必须来自 `weapon_classes.id`（如 `dagger` / `shield`）。
- 同一条规则内，若出现同 `equipType + slot + weaponClass` 重复命中（`slots`/`weaponClasses` 为空时按 `*` 维度计算），构建阶段会直接报错。

可直接复制的配置模板（覆盖常见场景）：

```json
{
  "id": "enchant.moon_essence",
  "toolItemId": "moon_essence",
  "consumeCount": 1,
  "costs": { "gold_coin": 1000, "enhance_stone": 2 },
  "failChance": 0.05,
  "rarityWeights": { "uncommon": 60, "rare": 30, "epic": 8, "legendary": 2 },
  "allowedAffixIds": [],
  "allowedAffixScopes": [
    {
      "equipType": "battle",
      "slots": [],
      "allowedAffixIds": ["strength", "dexterity", "hp", "crit_chance", "physics_damage"]
    },
    {
      "equipType": "battle",
      "slots": ["right_hand"],
      "weaponClasses": ["dagger", "shield"],
      "allowedAffixIds": ["physics_damage", "crit_chance", "physics_percent_defense_penetration"]
    },
    {
      "equipType": "battle",
      "slots": ["head", "chest"],
      "allowedAffixIds": ["hp", "stamina", "physics_damage_resist", "magical_damage_resist"]
    },
    {
      "equipType": "work",
      "slots": [],
      "allowedAffixIds": ["stamina", "hp_regen", "mp_regen"]
    },
    {
      "equipType": "work",
      "slots": ["right_hand"],
      "allowedAffixIds": []
    }
  ],
  "active": true
}
```

模板语义说明：
- `allowedAffixIds=[]`（规则根级）表示默认池不限制；只有在 scope 未命中时才生效。
- `battle + slots=[]`：战斗装备默认池。
- `battle + slots=["right_hand"]`：战斗武器位精确覆盖（优先级高于 battle 默认池）。
- `work + slots=[]`：生活装备默认池。
- `work + slots=["right_hand"] + allowedAffixIds=[]`：生活右手位放开限制（仍会叠加 `equipment_templates.affixAllowedIds` 与稀有度可用性过滤）。

### affix_carrier_rules（词条载体规则）

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | string | 规则 id |
| `carrierItemId` | string | 载体道具物品 id |
| `allowedRarityIds` | JSON array | 可承载词条稀有度列表（为空表示全稀有度） |
| `active` | boolean | 是否启用 |

补充规则：
- 装配词条时，词条来源实例必须命中一条启用中的 `carrierItemId` 规则。
- 拆卸词条时，选中的载体实例也必须命中规则，且目标词条稀有度满足 `allowedRarityIds`。
- 当 `allowedRarityIds=[]` 时，表示该载体可接受任意稀有度词条。

### 已废弃（删除）

- `affix_rarity_profiles`
- `affix_pools`
- `durability_profiles`
- `equipment_enhance_levels`
- `equipment_enhance_costs`
- `equipment_mutation_pools`
- `enchant_books`
- `enchant_extract_rules`

## 2.4 common.xlsx

### rarity_definitions（统一稀有度定义，item/affix 共用）

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | string | 稀有度 id（如 `common` / `legendary`） |
| `nameKey` | string | i18n key |
| `order` | number | 展示排序 |
| `color` | string | 展示色值（可选） |
| `economyMultiplier` | number | 稀有度经济倍率（默认 `1`） |

默认 i18n key 规则（未显式填写时自动生成）：

- `nameKey` -> `rarity.{id}`
## 2.5 combat.xlsx

### skills

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | string | 技能 id |
| `nameKey` | string | 技能名称 i18n key（可选，默认 `skill.{id}.name`） |
| `descKey` | string | 技能描述 i18n key（可选，默认 `skill.{id}.desc`） |
| `category` | string | 技能分类 |
| `icon` | string | 技能图标（可选，Iconify 图标名，如 `mdi:sword`） |
| `hpCost` | number | HP 消耗 |
| `mpCost` | number | MP 消耗 |
| `cooldown` | number | 冷却回合数（单位行动回合） |
| `targeting` | JSON object | 统一目标配置（唯一入口） |
| `effects` | JSON array | 统一效果配置（唯一入口，按顺序执行） |
| `castGuards` | JSON array | 施法门禁（可选，示例：敌方存在某 while_active effect 时禁止释放） |
| `statusRequirement` | JSON object | 启用属性门槛（如 `{"intelligence":21}`） |
| `weaponRequirement` | JSON array | 启用武器门槛（如 `["bow","crossbow"]`） |

新通用技能模板字段（优先使用）：

- Excel 入口固定为：`packages/game-data/source/workbooks/combat.xlsx` 的 `skills` sheet。
- 可直接在 `skills` 行里填写两列：`targeting`（JSON object）和 `effects`（JSON array）；`effects` 是执行主入口，`targeting` 仅保留 legacy 回退。
- `targeting`：目标配置对象，包含 `targetPositions/secondarySeek/targetScope/selectRule/selectAttrKey/targetCount/targetPredicates/excludeSelf`
- `castGuards`：施法前门禁数组，当前支持 `enemy_side_not_has_effect`（例如：敌方任一单位存在 `invert_relation_perspective` 时禁用技能）
- `effects[].targeting.targetCount` 与 `basicAttackEffects[].targeting.targetCount` 支持可选上限字段 `max`（与 `base/formula` 同层级）：
  - 先计算 `base + floor(formula)`，再应用 `max` 上限；
  - 仅对这一步生效，`effectMutations.targetCountDelta` 可在此基础上继续突破上限。
- `effects`：效果数组，按顺序执行，支持 `damage/heal/status`
- `effects[].onlyWhenNoTargetFromEffects`：可选 `number[]`（1-based 前序 effect 索引）；仅当这些被引用 effect **都未选中目标**（ALL empty）时，当前 effect 才执行
- `effects[].reuseTargetsFromEffect`：可选 `number`（1-based 前序 effect 索引）；当前 effect 复用该 effect 的目标集合，不再独立重新选目标
- `effects[].ignoreUntargetable`：可选 boolean，`true` 时该 effect 选目标会忽略“不可被敌方点选（如潜行）”限制
- `effects[].amountFormula`：`{ baseAdd, terms:[{attrKey,coefficient,perSkillLevel}] }`
- `effects[].targeting.targetPredicates`：目标后置过滤器（按目标逐个判定）；当前支持：
  - `compare_attr`（如 `actor.perception > target.perception`）
  - `exclude_enemy_type`（如排除 `boss`）
  - `require_target_has_status`（如仅命中有 `panic` 的目标）
  - `exclude_target_has_status`（如仅命中没有 `armor_break` 的目标；判断节点的“否则”分支使用）
- `effects[].consumeStatusesOnHit`：命中后消费状态（如 `["panic"]`）
- `effects[].statusPayload`：`{ statusIds: string[], durationBaseTurns, durationPerSkillLevelTurns, durationFormula, applyChanceBase?, applyChancePerSkillLevel?, applyChanceFormula?, applyChanceMin?, applyChanceMax? }`
- `effects[].statusPayload.durationFormula`：同 `amountFormula` 结构；最终持续回合 = `durationBaseTurns + durationPerSkillLevelTurns*(level-1) + floor(durationFormula)`
- `effects[].statusPayload.applyChance*`：最终施加率 = `clamp(applyChanceBase + applyChancePerSkillLevel*(level-1) + applyChanceFormula, applyChanceMin, applyChanceMax)`；每个目标、每个状态独立掷点，未配置时为 `100%`。
- `effects[].requiredCasterStatuses/requiredTargetStatuses`：状态全部存在才执行；`effects[].forbiddenCasterStatuses/forbiddenTargetStatuses`：任一状态存在就跳过，用于判断节点的“否则”分支。
- `effects[].damageSpec.bonusAgainstTargetTypes`：目标类型增伤映射（如 `{"boss":0.2}`）
- `targeting.fallbackSelfIfEmpty`：目标为空时回退到施法者
- `targeting.excludeSelf`：从候选目标中排除施法者，可用于“所有友军但不包含自己”或“所有单位但不包含自己”。
- `targeting.selectRule` 支持：`hp_max/hp_min/hp_percent_max/hp_percent_min/first/boss_first/attr_max/attr_min/random`
- 如果觉得直接写 Excel 太累，可以先用仓库根目录的 `skill-tool/index.html`，用可视化编辑器生成 `targeting` / `effects` JSON，再复制回 `skills` sheet。
- 武器模板可用仓库根目录的 `weapon-tool/index.html`，生成 `equipment_templates` 单行 TSV 后直接复制回 `equipment.xlsx`。
- affix 可用仓库根目录 `affix-tool/index.html`，可视化编辑 `allowedRarities` 与 `effectParams.effectMutations`，并支持一键复制 `affix_definitions` TSV、文档示例 JSON 与 `effectParams` JSON。
- `affix-tool` 中每个输入控件都提供可见 label（不依赖 placeholder），便于策划逐项配置与校对。
- `weapon-tool` 的下拉统一为“中文展示 + 英文值写入”：页面显示中文选项，但导出的 JSON/TSV 枚举值保持构建所需英文（如 `damage/heal/status`、`physical/magical`、`battle/work`）。
- `weapon-tool` 里 `occupySlots/weaponClass/statusRequirement/baseAttrs/enhanceAttrCurve/basicAttackEffects` 已支持交互编辑，不需要手填 JSON 文本。
- `weapon-tool` 不再编辑或生成兼容冗余字段 `basicAttackTargetDirection/targetSeekStrategy/basicAttackFormula`，统一以 `basicAttackEffects` 作为普攻配置入口。
- `weapon-tool` 的 `damageSpec` 中以下字段允许留空且默认导出为空：穿透固定/百分比、物理/魔法穿透、额外增伤、目标类型加成、命中次数（仅保留填写项）。
- `weapon-tool` 对 `damage` 效果仅暴露 `damageSpec.amountFormula`；`effect.amountFormula` 不再单独编辑，保存时自动按 `damageSpec.amountFormula` 同步生成。
- `weapon-tool` 已重新暴露 `effect.damageType`（damage/heal 都可编辑）；若不改会按默认值落到 `physical`，与构建归一化行为一致。
- `buff / buff_type / buff_effects` 可用仓库根目录 `buff-tool/index.html` 以“状态”为中心编辑：列表点“编辑”后，在**一个弹窗**里同时维护分类、状态本体、规则配置，导出时会按 3 个表分别输出 TSV。
- `buff-tool` 的可见下拉与分类文案默认中文展示（值仍保留英文），`buff_effects` 的 `conditions/effects` 通过交互控件配置，并同步生成 JSON（含只读预览）。

配置示例（skills）：

| id | icon | targeting | statusRequirement | weaponRequirement | effects |
|---|---|---|---|---|---|
| `arcane_missile` | `mdi:star-four-points` | `{"targetPositions":["vanguard"],"secondarySeek":"random_alive","targetScope":"enemy","selectRule":"random","targetCount":{"base":3,"formula":[]}}` | `{}` | `[]` | `[{"kind":"damage","damageType":"magical","amountFormula":{"baseAdd":0,"terms":[{"attrKey":"magical_damage","coefficient":1.25,"perSkillLevel":0.02}]}}]` |

模板示例：

```json
{
  "targeting": {
    "targetPositions": ["vanguard", "rearguard"],
    "secondarySeek": "front_to_rear",
    "targetScope": "enemy",
    "selectRule": "hp_min",
    "targetCount": { "base": 2, "formula": [] }
  },
  "effects": [
    {
      "kind": "damage",
      "damageType": "physical",
      "amountFormula": { "baseAdd": 10, "terms": [{ "attrKey": "strength", "coefficient": 1, "perSkillLevel": 0 }] }
    },
    {
      "kind": "status",
      "statusPayload": { "statusIds": ["bleed"], "durationBaseTurns": 2, "durationPerSkillLevelTurns": 1 }
    }
  ]
}
```

可复制模板（分别用于 `targeting` 列 与 `effects` 列）：

1. 治疗（最低血量百分比友军，空目标时回退自己）

`targeting`：

```json
{
  "targetPositions": ["vanguard", "midguard", "rearguard"],
  "secondarySeek": "none",
  "targetScope": "ally",
  "selectRule": "hp_percent_min",
  "fallbackSelfIfEmpty": true,
  "targetCount": { "base": 1, "formula": [] }
}
```

`effects`：

```json
[
  {
    "kind": "heal",
    "damageType": "magical",
    "amountFormula": {
      "baseAdd": 0,
      "terms": [
        { "attrKey": "magical_damage", "coefficient": 1.2, "perSkillLevel": 0 },
        { "attrKey": "intelligence", "coefficient": 0.6, "perSkillLevel": 0 }
      ]
    }
  }
]
```

2. 伤害（按行首目标依次打：前排→中排→后排）

`targeting`：

```json
{
  "targetPositions": ["vanguard"],
  "secondarySeek": "none",
  "targetScope": "enemy",
  "selectRule": "first",
  "targetCount": { "base": 1, "formula": [] }
}
```

`effects`：

```json
[
  {
    "kind": "damage",
    "damageType": "magical",
    "targeting": {
      "targetPositions": ["vanguard"],
      "secondarySeek": "none",
      "targetScope": "enemy",
      "selectRule": "first",
      "targetCount": { "base": 1, "formula": [] }
    },
    "amountFormula": {
      "baseAdd": 0,
      "terms": [{ "attrKey": "magical_damage", "coefficient": 1.25, "perSkillLevel": 0.02 }]
    }
  },
  {
    "kind": "damage",
    "damageType": "magical",
    "targeting": {
      "targetPositions": ["midguard"],
      "secondarySeek": "none",
      "targetScope": "enemy",
      "selectRule": "first",
      "targetCount": { "base": 1, "formula": [] }
    },
    "amountFormula": {
      "baseAdd": 0,
      "terms": [{ "attrKey": "magical_damage", "coefficient": 1.25, "perSkillLevel": 0.02 }]
    }
  },
  {
    "kind": "damage",
    "damageType": "magical",
    "targeting": {
      "targetPositions": ["rearguard"],
      "secondarySeek": "none",
      "targetScope": "enemy",
      "selectRule": "first",
      "targetCount": { "base": 1, "formula": [] }
    },
    "amountFormula": {
      "baseAdd": 0,
      "terms": [{ "attrKey": "magical_damage", "coefficient": 1.25, "perSkillLevel": 0.02 }]
    }
  }
]
```

3. 状态（对全体友军上 blessing）

`targeting`：

```json
{
  "targetPositions": ["vanguard", "midguard", "rearguard"],
  "secondarySeek": "none",
  "targetScope": "ally",
  "selectRule": "hp_percent_min",
  "fallbackSelfIfEmpty": true,
  "targetCount": { "base": 999, "formula": [] }
}
```

`effects`：

```json
[
  {
    "kind": "status",
    "statusPayload": {
      "statusIds": ["blessing"],
      "durationBaseTurns": 0,
      "durationPerSkillLevelTurns": 0
    }
  }
]
```

4. 目标联动（可直接复制）

技能 `effects`：前排没目标时，后续效果才改打后排；并让第三个效果复用第二个效果目标。

```json
[
  {
    "kind": "damage",
    "damageType": "physical",
    "targeting": {
      "targetPositions": ["vanguard"],
      "secondarySeek": "none",
      "targetScope": "enemy",
      "selectRule": "first",
      "targetCount": { "base": 1, "formula": [] }
    },
    "amountFormula": {
      "baseAdd": 0,
      "terms": [{ "attrKey": "physics_damage", "coefficient": 1, "perSkillLevel": 0 }]
    }
  },
  {
    "kind": "damage",
    "damageType": "physical",
    "onlyWhenNoTargetFromEffects": [1],
    "targeting": {
      "targetPositions": ["rearguard"],
      "secondarySeek": "none",
      "targetScope": "enemy",
      "selectRule": "first",
      "targetCount": { "base": 1, "formula": [] }
    },
    "amountFormula": {
      "baseAdd": 0,
      "terms": [{ "attrKey": "physics_damage", "coefficient": 0.9, "perSkillLevel": 0 }]
    }
  },
  {
    "kind": "status",
    "reuseTargetsFromEffect": 2,
    "targeting": {
      "targetPositions": ["vanguard"],
      "secondarySeek": "none",
      "targetScope": "enemy",
      "selectRule": "first",
      "targetCount": { "base": 1, "formula": [] }
    },
    "statusPayload": {
      "statusIds": ["bleed"],
      "durationBaseTurns": 2,
      "durationPerSkillLevelTurns": 0
    }
  }
]
```

5. 武器普攻（Boss 匕首）

`basicAttackEffects`：

```json
[
  {
    "kind": "damage",
    "targeting": {
      "targetPositions": ["vanguard"],
      "secondarySeek": "front_to_rear",
      "targetScope": "enemy",
      "selectRule": "boss_first",
      "targetCount": { "base": 1, "formula": [] }
    },
    "amountFormula": {
      "baseAdd": 0,
      "terms": [{ "attrKey": "strength", "coefficient": 0.8, "perSkillLevel": 0 }]
    },
    "damageSpec": {
      "damageType": "physical",
      "amountFormula": {
        "baseAdd": 0,
        "terms": [{ "attrKey": "strength", "coefficient": 0.8, "perSkillLevel": 0 }]
      },
      "hitCount": { "base": 1, "formula": [] },
      "flatDefensePenetrationFormula": { "baseAdd": 0, "terms": [] },
      "percentDefensePenetrationFormula": { "baseAdd": 0, "terms": [] },
      "physicalFlatDefensePenetrationFormula": { "baseAdd": 0, "terms": [] },
      "physicalPercentDefensePenetrationFormula": { "baseAdd": 0, "terms": [] },
      "magicalFlatDefensePenetrationFormula": { "baseAdd": 0, "terms": [] },
      "magicalPercentDefensePenetrationFormula": { "baseAdd": 0, "terms": [] },
      "extraDamageAmplificationFormula": { "baseAdd": 0, "terms": [] },
      "bonusAgainstTargetTypes": { "boss": 0.2 }
    }
  }
]
```

说明：`basicAttackEffects` 的伤害类效果以 `damageSpec.damageType` 为准；外层 `damageType` 为 legacy 兼容字段，可不填写。
说明：`basicAttackEffects` 与 `skills.effects` 共享同一套 effect 联动字段：
说明：`basicAttackEffects` 的 `attrKey/selectAttrKey` 可引用已定义在 `attributes` 表中的任意属性（包含 `work` 类，如 `sewing`），未定义属性会在构建阶段报错。
说明：前端展示属性名走 `attr.<attrKey>.name` i18n key；未翻译时会回退显示原始 `attrKey`。

- `onlyWhenNoTargetFromEffects`：1-based 且仅允许引用前序 effect，按 ALL-empty 判定；
- `reuseTargetsFromEffect`：1-based 且仅允许引用前序 effect。

6. 武器普攻（长矛双 effect，无次要寻敌回退）

```json
[
  {
    "kind": "damage",
    "damageType": "physical",
    "targeting": {
      "targetPositions": ["vanguard"],
      "secondarySeek": "none",
      "targetScope": "enemy",
      "selectRule": "first",
      "targetCount": { "base": 1, "formula": [] }
    },
    "amountFormula": {
      "baseAdd": 5,
       "terms": [{ "attrKey": "physics_damage", "coefficient": 0.5, "perSkillLevel": 0 }, { "attrKey": "strength", "coefficient": 0.15, "perSkillLevel": 0 }, { "attrKey": "strength", "coefficient": 0.15, "perSkillLevel": 0 }]
    },
    "damageSpec": {
      "damageType": "physical",
      "amountFormula": {
        "baseAdd": 5,
        "terms": [{ "attrKey": "physics_damage", "coefficient": 0.1, "perSkillLevel": 0 }, { "attrKey": "strength", "coefficient": 0.15, "perSkillLevel": 0 }, { "attrKey": "strength", "coefficient": 0.15, "perSkillLevel": 0 }]
      },
      "hitCount": { "base": 1, "formula": [] },
      "flatDefensePenetrationFormula": { "baseAdd": 0, "terms": [] },
      "percentDefensePenetrationFormula": { "baseAdd": 0, "terms": [] },
      "extraDamageAmplificationFormula": { "baseAdd": 0, "terms": [] }
    }
  },
  {
    "kind": "damage",
    "damageType": "physical",
    "targeting": {
      "targetPositions": ["midguard"],
      "secondarySeek": "none",
      "targetScope": "enemy",
      "selectRule": "first",
      "targetCount": { "base": 1, "formula": [] }
    },
    "amountFormula": {
      "baseAdd": 0,
       "terms": [{ "attrKey": "physics_damage", "coefficient": 0.5, "perSkillLevel": 0 }, { "attrKey": "strength", "coefficient": 0.15, "perSkillLevel": 0 }, { "attrKey": "strength", "coefficient": 0.15, "perSkillLevel": 0 }]
    },
    "damageSpec": {
      "damageType": "physical",
      "amountFormula": {
        "baseAdd": 0,
        "terms": [{ "attrKey": "physics_damage", "coefficient": 0.1, "perSkillLevel": 0 }, { "attrKey": "strength", "coefficient": 0.15, "perSkillLevel": 0 }, { "attrKey": "strength", "coefficient": 0.15, "perSkillLevel": 0 }]
      },
      "hitCount": { "base": 1, "formula": [] },
      "flatDefensePenetrationFormula": { "baseAdd": 0, "terms": [] },
      "percentDefensePenetrationFormula": { "baseAdd": 0, "terms": [] },
      "extraDamageAmplificationFormula": { "baseAdd": 0, "terms": [] }
    }
  }
]
```

```json 镰刀
[
  {
    "kind": "damage",
    "damageType": "physical",
    "targeting": {
      "targetPositions": ["vanguard", "midguard"],
      "secondarySeek": "none",
      "targetScope": "enemy",
      "selectRule": "first",
      "targetCount": { "base": 2, "formula": [] }
    },
    "amountFormula": {
      "baseAdd": 0,
      "terms": [{ "attrKey": "physics_damage", "coefficient": 0.3, "perSkillLevel": 0 }, { "attrKey": "strength", "coefficient": 0.1, "perSkillLevel": 0 }, { "attrKey": "dexterity", "coefficient": 0.1, "perSkillLevel": 0 }]
    },
    "damageSpec": {
      "damageType": "physical",
      "amountFormula": {
        "baseAdd": 0,
       "terms": [{ "attrKey": "physics_damage", "coefficient": 0.3, "perSkillLevel": 0 }, { "attrKey": "strength", "coefficient": 0.1, "perSkillLevel": 0 }, { "attrKey": "dexterity", "coefficient": 0.1, "perSkillLevel": 0 }]
      },
      "hitCount": { "base": 2, "formula": [] },
      "flatDefensePenetrationFormula": { "baseAdd": 0, "terms": [] },
      "percentDefensePenetrationFormula": { "baseAdd": 0, "terms": [] },
      "extraDamageAmplificationFormula": { "baseAdd": 0, "terms": [] }
    }
  }
]

```

### skill_type

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | string | 技能类型 id（供 `skills.category` 引用） |
| `nameKey` | string | 技能类型名称 i18n key（可选，默认 `skill_type.{id}.name`） |
| `descKey` | string | 技能类型描述 i18n key（可选，默认 `skill_type.{id}.desc`） |
| `color` | string | 类型颜色（必须是 HEX 色值，如 `#409EFF`） |

### weapon_classes

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | string | 武器类别 id（如 `sword/spear/bow/crossbow/staff`） |
| `nameKey` | string | 类别名称 i18n key（默认 `weapon_class.{id}.name`） |
| `descKey` | string | 类别描述 i18n key（默认 `weapon_class.{id}.desc`） |
| `icon` | string | 图标 id（可选） |
| `color` | string | 展示色值（HEX） |
| `order` | number | 排序值 |

### buff

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | string | 状态 id |
| `nameKey` | string | 状态名称 i18n key（可选，默认 `buff.{id}.name`） |
| `descKey` | string | 状态描述 i18n key（可选，默认 `buff.{id}.desc`） |
| `durationTurns` | number | 持续回合数（`permanent=true` 时可为 `0`） |
| `maxCharges` | number | 最大触发次数（`0` 表示不限制；与持续回合可同时存在） |
| `permanent` | boolean | 是否永久状态 |
| `kind` | string | 状态类型：`buff` 或 `debuff` |
| `stackPolicy` | string | 重复施加策略：`refresh_duration`（重置持续）或 `stack`（允许叠层） |
| `dispellable` | boolean | 是否可驱散 |
| `type` | string | 状态分类（引用 `buff_type.id`，可选） |

### buff_type

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | string | 状态分类 id（供 `buff.type` 引用） |
| `nameKey` | string | 状态分类名称 i18n key（可选，默认 `buff_type.{id}.name`） |
| `descKey` | string | 状态分类描述 i18n key（可选，默认 `buff_type.{id}.desc`） |
| `color` | string | 类型颜色（必须是 HEX 色值，如 `#409EFF`） |

### buff_effects

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | string | 效果规则 id |
| `buffId` | string | 关联状态 id（引用 `buff.id`） |
| `trigger` | string | 触发时机（如 `onAttach/onTurnStart/onBeforeTakeDamage`） |
| `priority` | number | 同触发点执行优先级（值越大越先执行） |
| `conditions` | JSON array | 触发条件（可空） |
| `effects` | JSON array | 触发效果（必填，至少一项） |

配置规范（DSL）：

1. `trigger` 支持值（大小写/下划线不敏感）：
   - `while_active`：状态存在期间持续生效（通常用于属性加成/减益）
   - `on_attach` / `on_detach`：状态挂载/卸载时
   - `on_turn_start` / `on_turn_end`：单位回合开始/结束
   - `on_before_take_damage` / `on_after_take_damage`：受击前/受击后
   - `on_before_heal`：受治疗前
   - `on_before_deal_damage` / `on_after_deal_damage`：造成伤害前/后

2. `conditions[].kind` 支持值：
   - `self_hp_percent_lte` / `self_hp_percent_gte`：自身生命比例阈值判断（`value` 范围 `0~1`）
   - `incoming_damage_gte` / `incoming_damage_lte`：即将结算伤害阈值判断（`value >= 0`）
   - `incoming_heal_gte` / `incoming_heal_lte`：即将结算治疗阈值判断（`value >= 0`）
   - `has_buff`：自身是否有指定 buff（`buffId`）
   - `source_has_buff`：来源单位是否有指定 buff（`buffId`）

3. `effects[].kind` 支持值：
   - 属性类：`add_stat`（`attr`,`value`），`add_stat_percent`（`attr`,`value`）
   - 数值类：`heal`（`value`），`resource`（`resource=hp/mp`,`value`），`set_heal_value`（`value`）
   - 状态类：`add_buff`（`buffId`,`durationMs?`,`payload?`），`remove_buff`（`buffId?`，为空时移除当前 buff）
   - 受击前修正：`reduce_damage_percent`（`value`），`set_damage_multiplier`（`value`），`set_damage_value`（`value`），`consume_charge`（`buffId?`，为空时作用于当前 buff）
   - 造成伤害前修正：`add_damage_percent`（`value`），`add_damage_flat`（`value`），`set_damage_value`（`value`）
   - 选目标语义：`untargetable_by_enemy`（使单位在敌方点选时不可作为候选）、`treat_all_as_enemy`（使施法者在敌方选目标时把全体单位都视作敌方候选）、`prefer_as_enemy_target`（使单位在敌方非全体选目标时被优先命中）

4. 最小示例：
   - `conditions`:
     - `[{"kind":"self_hp_percent_lte","value":0.5}]`
   - `effects`:
     - `[{"kind":"reduce_damage_percent","value":0.1}]`
     - `[{"kind":"add_stat","attr":"strength","value":3},{"kind":"add_stat_percent","attr":"crit_chance","value":0.1}]`
     - 骨盾示例：`{"buff":[{"id":"bone_shield","durationTurns":3,"maxCharges":3,"kind":"buff","dispellable":true,"type":"guard"}],"buff_effects":[{"id":"bone_shield_take_damage","buffId":"bone_shield","trigger":"on_before_take_damage","priority":-100,"conditions":[],"effects":[{"kind":"set_damage_value","value":1},{"kind":"consume_charge"}]}]}`
     - 骨愈示例：`{"buff":[{"id":"bone_mend","durationTurns":3,"maxCharges":3,"kind":"buff","dispellable":true,"type":"support"}],"buff_effects":[{"id":"bone_mend_heal","buffId":"bone_mend","trigger":"on_before_heal","priority":-100,"conditions":[],"effects":[{"kind":"set_heal_value","value":1},{"kind":"consume_charge"}]}]}`

### 本次扩展概括

- `buff.maxCharges`：把“触发次数”从回合持续时间里拆出来，允许持续回合和触发次数并存。
- 新增固定值改写 effect：`set_damage_value`、`set_heal_value`，分别用于受击前/受治疗前把结果钉到指定值。
- 新增状态操作 effect：`consume_charge`，用于每次触发后递增计数并在耗尽时移除 buff。
- 现有 hooks 保持通用：`on_before_take_damage`、`on_before_heal`、`on_before_deal_damage` 都可以复用同一套“条件 + effect”结构。
- 实战约定：优先级低的规则适合做“最终兜底固定值”，高优先级规则适合做前置修正；骨盾示例用低优先级，确保前置减伤后仍能落到 1。

### combat_config_rules

| 字段 | 类型 | 说明 |
|---|---|---|
| `base_skill_slots` | number | 基础技能槽位 |
| `int_per_extra_slot` | number | 每多少点智力 +1 槽位 |
| `min_skill_slots` | number | 最小槽位数 |
| `max_skill_slots` | number | 最大槽位数 |
| `default_basic_skill_id` | string | 默认基础技能 id（可选，建议显式配置） |

说明：
- 前后端都必须读取该表结果，不允许单边硬编码。

### skill_rule_sets

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | string | 规则集 id |
| `ruleKind` | string | 规则类型（技能规则） |
| `groups` | JSON array | 规则组，组内 AND，组间 OR |

### item_rule_sets

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | string | 规则集 id |
| `ruleKind` | string | 规则类型（道具规则） |
| `groups` | JSON array | 规则组，组内 AND，组间 OR |

### effects

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | string | 效果 id |
| `effectType` | string | 效果类型 |
| `params` | JSON object | 效果参数 |

### 说明

- `battle` 页里的技能槽与规则配置，依赖 `combat.xlsx` 和 `combat_config_rules`。
- 规则 JSON 结构里，组内为 AND、组间为 OR。
- `skills.category` 必须命中 `skill_type.id`，否则构建报错。
- `buff.type` 必须命中 `buff_type.id`（非空时）。
- `buff_effects.buffId` 必须命中 `buff.id`，且 `effects` 必须是非空 JSON 数组。
- 构建会按 `skills` 自动生成技能书物品（排除 `basic_attack`），规则：
  - 物品 id：`skillbook_<skillId>`
  - 自动写入：`useGrantSkillId=<skillId>`、`useSkillExpGain=100`
  - 与手工同 id 物品冲突时，手工优先，仅补缺失字段

## 2.6 enemy.xlsx

### world

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | string | 战斗区域 id，供 `/api/battle/worlds` 和战斗区域下拉使用 |
| `enemyIds` | JSON array | 普通敌人池 |
| `specialEnemyIds` | JSON array | 特殊敌人池 |
| `specialWaves` | JSON array / JSON object | 特殊波次（固定波次或周期随机） |
| `entryDurationSeconds` | number | 进入持续时长 |
| `entryCosts` | JSON object | 进入消耗 |
| `normalEnemyCount` | number / JSON object | 普通敌人数（固定值或区间 `{min,max}`） |
| `extraEnemyPerPlayer` | JSON object | 每多 1 个玩家单位额外生成敌人数区间 `{min,max}` |
| `customLineupConf` | JSON array | 特殊阵容配置（按 `percent` 概率命中后替换整波普通生成） |

说明：
- 战斗 UI 的“战斗区域选择”直接读取该 sheet。
- 数组/对象单元格必须写成合法 JSON。
- `specialWaves` 支持两种格式：
  - 固定波次（旧格式，兼容）：`[3,6,9]`
  - 周期随机（推荐）：`{"every":15,"minCount":1,"maxCount":2}`，表示每 15/30/45... 波，从 `specialEnemyIds` 中随机生成 `1~2` 个（有放回抽样）。
- `normalEnemyCount` 支持：
  - 固定值：`3`
  - 区间值：`{"min":3,"max":5}`
- `extraEnemyPerPlayer` 示例：`{"min":0,"max":2}`（`min/max` 可为 0）。
- `customLineupConf` 字段说明：
  - `id`：阵容 id（同一 world 内唯一）
  - `nameKey/descKey`：可留空，构建时自动生成为 `world.<worldId>.customLineup.<id>.name|desc`
  - `percent`：命中概率，范围 `[0,1]`
  - `isSpecialWaveLineup`：`true` 仅在 specialWave 候选，`false` 仅在非 specialWave 候选
  - `lineup`：敌人 id 数组；命中后该波直接使用该阵容，不再从普通池或 `specialEnemyIds` 额外生成
  - `drops.items`：该 lineup 被击败后额外执行一次掉落检查，结构为 `{"itemId":{"min":1,"max":2,"percent":0.4}}`

示例（JSON）：

- `enemyIds`：`["enemy_slime","enemy_wolf"]`
- `specialEnemyIds`：`["enemy_elite_slime","enemy_wolf_alpha"]`
- `specialWaves`（固定波次）：`[3,6,9]`
- `specialWaves`（周期随机）：`{"every":15,"minCount":1,"maxCount":2}`
- `entryCosts`：`{"stamina": 10, "gold_coin": 100}`
- `normalEnemyCount`（区间）：`{"min":3,"max":5}`
- `extraEnemyPerPlayer`：`{"min":0,"max":1}`
- `customLineupConf`：
  ```json
  [
    {
      "id": "wolf_pack",
      "percent": 0.03,
      "isSpecialWaveLineup": false,
      "lineup": ["enemy_wolf", "enemy_wolf", "enemy_wolf_alpha"],
      "drops": {
        "items": {
          "wolf_fang": {
            "min": 1,
            "max": 2,
            "percent": 0.4
          }
        }
      }
    }
  ]
  ```

### enemies

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | string | 敌人 id |
| `name` | string | 名称 |
| `level` | number | 等级（先只做数据/展示） |
| `type` | enum | 敌人类型：`normal` / `elite` / `boss` |
| `position` | string | 站位 |
| `exp` | number | 经验 |
| `drops` | JSON array | 掉落列表 |
| `hp` | number | 生命值 |
| `hp_regen` | number | 生命恢复 |
| `mp` | number | 法力值 |
| `mp_regen` | number | 法力恢复 |
| `strength` | number | 力量 |
| `dexterity` | number | 敏捷 |
| `intelligence` | number | 智力 |
| `stamina` | number | 体力 |
| `luck` | number | 幸运 |
| `physics_damage` | number | 物理攻击 |
| `physics_defence` | number | 物理防御 |
| `magical_damage` | number | 魔法攻击 |
| `magical_defence` | number | 魔法防御 |
| `crit_chance` | number | 暴击率 |
| `crit_damage` | number | 暴击伤害 |

示例（JSON）：

- `drops`：
  ```json
  {
    "gold": {
      "range": {
        "min": 10,
        "max": 10,
      },
      "percent": 0.25,
    }
  }
  ```

构建说明：
- `level/type` 为空时会回退到默认值（`level=1`, `type=normal`）；若 `id` 含 `boss`，构建会自动推断为 `boss`。

## 2.7 economy.xlsx

- `market_fee_tiers(id,scope,min_total,max_total,fee_bps,priority,active)`
- `market_npc_offers(id,item_id,npc_buy_price,npc_sell_price,active)`
- `drop_tables(id,source,entries)`
- `warehouse_rules(id,warehouseType,cols,rows)`

## 2.8 growth.xlsx

### attributes

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | string | 属性唯一标识，强制小写蛇形（如 `attack_speed`） |
| `icon` | string | Iconify 图标名（可空，建议 `game-icons:...`） |
| `iconId` | string | 图标 id（可空） |
| `emojiIcon` | string | emoji 图标（可空） |
| `nameKey` | string | i18n key，强制 `attr.{id}.name` |
| `descKey` | string | i18n key，强制 `attr.{id}.desc` |
| `baseDefault` | number | 基础默认值 |
| `growthType` | enum | `levelable`（可升级）或 `static`（静态值） |
| `expProfileId` | string | 经验曲线 id；仅 `levelable` 时可填且必填 |
| `unit` | enum | 展示单位，首版 `number` / `percent` |
| `order` | number | 展示排序，必须唯一 |
| `category` | enum | 前端分组标签：`work` / `battle` / `other` |
| `relatedActionMainType` | string | 关联动作主类（可选，用于动作效率映射） |
| `relatedActionEffect` | string | 关联动作效果（可选，当前支持 `actionEfficiency`） |

构建校验规则：
- `category` 必填且必须在枚举内。
- `icon` 若填写，必须是可被前端属性图标组件识别的 Iconify 标识；渲染优先级高于 `iconId` / `emojiIcon`。
- `order` 必须唯一，重复即报错。
- `growthType=levelable` 时 `expProfileId` 必填；`growthType=static` 时禁止填写 `expProfileId`。

### attribute_exp_profiles

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | string | 经验曲线 id（供 attributes.expProfileId 引用） |
| `attr` | string | 兼容旧字段，建议填写 `*` 或属性类别标识 |
| `level` | number | 等级 |
| `expRequired` | number | 升到该等级所需经验值 |

### daily_tasks

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | string | 任务 id |
| `title` | string | 标题 |
| `description` | string | 描述 |
| `requirements` | JSON | 任务条件 |
| `rewards` | JSON | 奖励配置 |

### daily_task_refresh_rules

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | string | 刷新规则 id |
| `ruleType` | string | 刷新规则类型 |
| `value` | number/string | 刷新规则值 |

## 2.9 world.xlsx

- `enemies(id,name,level,attrs,dropTableId)`
- `events_world(id,eventType,conditions,effects)`
- `activity_mutex_rules(id,activityA,activityB,mutex)`

## 2.10 world.battlefieldEffects（combat-model JSON）

战场级机制以 `packages/game-data/source/json/world.json` 为权威源，不写入 Excel，也不得在战斗运行时按副本 id 分支。旧世界可以省略 `battlefieldEffects`，省略时行为不变。

- `counter.source` 当前支持 `turn_consumed`。
- `counter.aggregation` 支持 `per_event` 与 `alive_roster_cycle`。
- `actorFilter` 必须显式声明阵营、单位类型、召唤物、回合类型和备战回合过滤。
- `alive_roster_cycle` 运行态只保存 `remaining` 与 `progress`；`goal` 由当前存活合资格阵容计算，阵容变化时将进度夹在 `0..goal-1`。
- `triggerEffects` 支持经过正常防御、抗性和减伤链路的战场伤害，以及通用 `weighted_random_per_target`。
- `weighted_random_per_target.targetCount` 以合资格存活人数为输入，支持 `ceil/floor`、除数和 `min/max`；`selection=uniform_without_replacement` 保证单轮不重复抽到同一单位。
- `weighted_random_per_target.choices` 的权重总和必须为 `1`；当前结果效果支持 `status` 与有符号 `adjust_skill_cooldown`，状态和技能引用均在构建阶段校验。
- `postTriggerWindows` 的 `id` 必须唯一；`conditionalModifiers.inactiveDuringWindow` 必须引用同一战场效果中存在的窗口。
- 战场效果的 `nameKey/descKey` 强制使用 `battlefield_effect.{id}.name/desc`，中英文基线缺失会使构建失败。
- 修改后运行 `node packages/game-data/scripts/build.mjs`，由构建器重新生成目录、类型和本地化托管区。
- 副本 7、8 及后续同类内容都应直接维护 `packages/game-data/source/json`，并通过 `combat-model-vue3` 生成或诊断敌人属性；不要回写工作簿，也不要手改 `packages/game-data/generated`。

## 2.11 skill_alchemy_modules（source/json）

- 权威源为 `packages/game-data/source/json/skill_alchemy_modules.json`。启用的 `loot` 战利品默认必须进入技能炼成目录；物品源数据显式设置 `skillForgeEligible: false` 时可只保留战利品分类而不开放炼成。生活产出则仅在显式设置 `skillForgeEligible: true` 时进入目录；缺少对应模块会阻断构建。
- 一种物品只能拥有一个启用的炼成模块，也就是只能修正一种 DSL 参数；同一物品重复提供不同节点效果会阻断构建。每项开放的 DSL 参数至少保留一种可用素材。
- 基础目标数量、最大目标数量、命中次数与效果槽只允许 `mythic` 素材；目标站位与属性要求降低至少为 `legendary`；索敌方向至少为 `rare`；目标阵营、效果连锁、施加状态至少为 `epic`。数值公式允许较低稀有度素材，但每批贡献必须按品质上限衰减。
- 启用模块的数量步长固定为 `common/uncommon/rare=1000`、`epic=400`、`legendary=200`、`mythic=50`。整数参数每批只能增加或减少 `1`；小数公式每批最大贡献依次为 `0.004/0.006/0.01/0.02/0.04/0.08`。
- 每种素材的单节点上限为 `unitBasis × maxBasisMultiplier`，启用模块的 `maxBasisMultiplier` 必须在 `1..5`；固定选项只能投入一批，且一次性消耗量为对应品质普通步长的 5 倍，不能用堆量绕过目标、索敌或条件边界。
- `select_rule` 支持生命值、生命比例、顺序、首领优先、随机及指定属性高低等固定规则；属性规则必须同时声明合法 `attrKey`，客户端不能自行上传规则或属性。
- 效果数量不是玩家手动填写的 DSL 字段。`effect_slot` 必须使用最高稀有度素材，数量按 `unitBasis` 换算新增效果槽；全局硬上限读取 `skillForge.maxEffects`。
- 治疗分支的属性公式仅接受智力、感知，技能等级使用独立 `skill_level_term`；新增治疗素材若引用其他属性，game-data build 必须失败。
- 类型、目标、公式、状态、条件与资源素材只能通过模块 ID 和数量提交；参数范围仍由模块表决定，客户端不得上传任意 DSL 数值。技能炼成不读取炼金等级、精度或稳定度：数值范围和素材失败率均直接使用模块表原值。
- 每批素材必须填写非零 `successChanceModifier`，而且必须符合构建器的集中平衡策略：数值越强、机制越稀有，成功率惩罚越高；提高技能代价、自损或延长冷却则按批返还成功率。未显式关闭技能炼成的新增战利品若没有唯一模块或没有明确成功率取舍，构建必须失败。
- 官方技能书炼成的金币消耗由 `skillForge.goldCostBase + 素材总数量 × goldCostPerMaterial + ceil(硬预算 × goldCostPerBudget)` 计算，并与素材一起进入库存账本；客户端只显示权威预览结果。

## 3. 动作效率规则（属性驱动）

动作耗时统一通过属性映射计算，不再支持装备/词条时长修正字段：

- 公式：`duration = baseDuration / (1 + efficiencyBonus)`
- `efficiencyBonus` 由 attributes 动态决定：
  - `relatedActionMainType == action.mainType`
  - `relatedActionEffect == "actionEfficiency"`（空值按 `actionEfficiency` 处理）
- 支持动态扩展：新增属性时，只需填写 `relatedActionMainType/relatedActionEffect`，无需改代码。

## 4. 维护建议

- 新增玩法动作时，如需影响耗时，优先新增对应效率属性并填写 `relatedActionMainType/relatedActionEffect`。
- 调整字段后，执行 `node packages/game-data/scripts/build.mjs` 重新生成 `generated/*`。
- 物品 i18n 可自动兜底：当 `items.{id}.name/desc/effect` 未提供人工文案时，前端会基于 `id` 自动生成中英文占位文案（可后续由 AI 批量润色替换）。

## 5. game-data-tool 并行规则（首期）

- Excel 流程保持可用；`game-data-tool` 为并行新入口。
- 新工具写入：`packages/game-data/source/json/*.json`（文件名=表名）。
- 首期覆盖表：`items`、`actions`、`equipment_templates`、`enemies`。
- 若 Excel 与 `source/json` 同表同 `id` 冲突，`build.mjs` 直接报错并终止构建。
- 公式仅在 `game-data-tool` 执行并落纯结果值；构建与运行时不执行表达式。
