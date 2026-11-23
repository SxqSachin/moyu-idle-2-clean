# 炼金（Alchemy）系统设计

> 状态：草案 v1
> 范围：新增"炼金"顶级菜单，整合材料转换、词条合成、点金转换、熔炉四个子玩法；新增"炼金等级"生活技能与一组从 `luck` 派生的属性。
> 入口约定：本文件作为团队对外的需求/设计文档，先于代码合入；实现阶段以本文为基准。

---

## 1. 概览

### 1.1 背景

- 现状：熔炉（Smelt）作为附魔（Affix）页内的一个 Tab 存在；物品转金币走独立的 `/items/recycle` 即时路径。
- 缺口：玩家缺少一个"材料炼化"统一入口；现有附魔系统不支持运行时自定义词条；缺少与"幸运"挂钩的生活玩法。
- 目标：新增"炼金"顶级菜单，四个独立子页面，配置 DSL 统一收口在 `packages/game-data/source/json/`，行动队列原生支持。

### 1.2 四个子玩法

| 子玩法 | 入口 | 核心动作 | 说明 |
|---|---|---|---|
| 转换材料 | `/game/alchemy/convert` | `alchemy.convert` | 按"炼金分组"权重把一种材料随机转换为同组内另一种 |
| 炼金台 | `/game/alchemy/bench` | `alchemy.bench` | 用拥有"特性"的材料 + 词条载体，合成运行时自定义附魔 |
| 点金 | `/game/alchemy/transmute` | `alchemy.transmute` | 物品 → 金币（80%~95%）+ 幸运派生属性驱动的额外奖励 |
| 熔炉 | `/game/alchemy/smelt` | `alchemy.smelt`（占位） | 从附魔页迁移；v1 保持即时执行，v2 入队 |

### 1.3 新增的玩家属性

| 属性 | 类型 | 说明 |
|---|---|---|
| `alchemy_level` | 生活技能等级 | 作为门槛 + 派生公式参数，唯一的"成长曲线" |
| `alchemy_precision` | 派生（来自 `luck` + `alchemy_level`） | 浮动值落点偏置（"靠近上限"程度） |
| `alchemy_lucky_chance` | 派生（来自 `luck`） | 点金幸运抽奖触发概率加成 |
| `alchemy_craft_success` | 派生（来自 `luck`） | 炼金台合成成功率加成 |

`luck` 是项目已有的生活基础属性，本设计不改动其定义，仅在 `global-config.json.derivedStats` 段新增三条派生项。

---

## 2. 设计原则

### 2.1 单一数据源（CLAUDE.md 强制）

所有 DSL 定义只允许出现在 `packages/game-data/source/json/`；TS 与 Go 类型一律由 `packages/game-data/scripts/build.mjs` 生成；任何"手工双份"配置都视为缺陷。

### 2.2 低耦合 / 单一职责

- 四个子玩法各自一个 `QueueActionExecutor` 实现，互不依赖。
- 跨子玩法的助手（如 `sampleLuckSkewed`、命名算法、发现表写入）抽取到 `store_alchemy.go` 作为纯函数或 store 方法，所有 executor 调用，避免重复实现。
- DSL 文件**按职责拆分**：`alchemy_groups.json / item_traits.json / alchemy_combo_recipes.json / alchemy_transmute_rules.json` 各管一段，互不交叠。

### 2.3 继承现有架构（不重造轮子）

| 复用点 | 来源 | 复用方式 |
|---|---|---|
| 行动队列 | `server/internal/store/action_queue.go` `QueueActionExecutor` | 四个子玩法实现该接口；非自定义部分用 `genericExecutor` 套路（duration/exp/loadout） |
| 工作装备自动切换 | `store_loadouts.go:ResolveWorkLoadoutRuntime` | `WorkActionBindings["alchemy"]` 一次绑定覆盖全部子玩法 |
| 现有附魔渲染 | `EquipmentAffixSlot` JSON 列 | 自定义附魔复用同一结构，新增 `SourceKind` 等字段；前端零适配 |
| 熔炉 / 拆卸路径 | `affix_smelt_rules.json` | 自定义附魔走材料最高稀有度落入现有 rarity 桶，零额外配置 |
| 强化成功率派生 | `global-config.json.derivedStats` | 新派生项与"强化成功率"并列，复用同一引擎 |
| 物品发现 / 解锁 | （新建） | `alchemy_discoveries` 表单一职责，独立 API |

### 2.4 扩展性铺垫

设计时为以下扩展点预留接口（不实现，但 schema/代码结构允许零拆迁直接扩展）：

| 扩展方向 | 已铺垫的钩子 |
|---|---|
| trait DSL 增加 `minStackForRarity` | trait 已是 object，加字段即可；命名算法不受影响 |
| `outputAffix` 增加 `effectType` 类型 | `effectType + params` 是泛型字段；新增类型只需新写一段 effect-apply 代码 |
| 自定义附魔加 `decay`（衰减） | `EquipmentAffixSlot.EffectParams` 可塞任意字段；renderer 视字段存在性决策 |
| combo 配方加多种产出（组合产出多个 affix） | `outputAffix` 可改为 `outputAffixes: [...]`；当前 v1 视为单元素数组的退化 |
| 后期熔炉入队 | `alchemy.smelt` ActionSpec 已写入 `actions.json`，仅需在 `executorForAction` 新增 case |
| 多档手册"标记珍藏" | `alchemy_discoveries` 表已用 `(user_id, kind, id)` 三元主键，加新 kind 不破坏现有索引 |

### 2.5 健壮性保证

- 所有钱物变动在一个 settle 事务中完成，开头 `lockUserTx`；炼金台另加 `LockInstanceTx(carrierInstanceId)`。
- 失败路径不静默：`fmt.Errorf("alchemy.xxx: %w", err)` 强制 wrap；前端按 `ClassifyStartError` 给出 i18n 文案。
- DSL 校验放在 `build.mjs`：缺字段、悬挂引用、混 kind、缺 i18n 文案 → 阻断构建，禁止上线。
- 浮点漂移：所有自定义附魔数值在落盘前用 `valuePrecision` 位四舍五入，再参与 `AffixID` hash，杜绝 0.001 飘移破坏聚合。

---

## 3. 玩家流程

### 3.1 转换材料（alchemy.convert）

```
背包选中 N 个 base1_moss_spore
  ↓
查"该 itemId 属于哪个 alchemy_group" → group.base_herb
  ↓
入队 alchemy.convert × N（args = { sourceItemId, groupId, sourceCount: N }）
  ↓
每个 tick（10 秒/次，可被快进吸收）：
  - 消耗 1 × base1_moss_spore
  - 按组内 weight 抽 1 件并发放
  - alchemy_level XP += 2
  ↓
首次抽中某产物且产物有 trait → 解锁手册条目（trait kind）
```

### 3.2 炼金台（alchemy.bench）

```
玩家进入炼金台
  ↓
左：背包筛选"有 traits 的物品" → 选 K 种 + 各自数量（按 unitBasis 步进）
中：选载体（必须 maxCustomAffixes > 0 且 allowedTraitKind 兼容）
右：实时预览
  - 当前组合 kind（life/combat）
  - 各 trait 最终值范围（按 valuePerUnit × count → [min, max]）
  - 当前精度 → "最可能产出 X.X 附近"
  - 当前成功率（基于等级差 + alchemy_craft_success）
  - 命名预览（按 A.4 算法即时计算）
  - 手册抽屉可查 "已发现 N/总数 M"
  ↓
点击"开始炼金" → 入队 alchemy.bench × 1
  ↓
30 秒后 settle：
  - roll 成功率
    - 失败 → 消耗 50% 材料，载体不动，发放 30% XP
    - 成功 → 消耗全部材料，载体 AffixesRaw 追加一条新 EquipmentAffixSlot
      - SourceKind="craft"
      - RarityID=材料最高稀有度
      - AffixID="custom:<sha1>"
      - DisplayNameKey + DisplayNameArgs = 命名算法输出
      - EffectType + EffectParams = trait/combo 描述
      - 发全额 XP
```

### 3.3 点金（alchemy.transmute）

```
背包选 N 个物品
  ↓
入队 alchemy.transmute × N
  ↓
每个 tick（6 秒/次）：
  - 消耗 1 个物品
  - gold = floor(economyBaseValue × sampleLuckSkewed(0.80, 0.95, precision))
  - 入账 gold
  - roll luckyChance（luckyChanceBase + alchemy_lucky_chance）
    - 命中 → 抽 luckyTiers（gold_multiplier 翻倍 / extra_item 额外物品）
  - alchemy_level XP += 1.5
```

### 3.4 熔炉（alchemy.smelt，v1 即时）

```
玩家进入熔炉
  ↓
显示带词条的载体列表
  ↓
选 1~N 个载体 → 显示预期回报
  ↓
确认 → 调 POST /api/equipment/affix/smelt（沿用现有路径）
  ↓
载体消失 + 余辉/辉核入账
```

> v2 升级路径：把 `AffixSmeltPanel` 提交从 `apiPost` 切到 `useActionSubmit().enqueueWithArgs('alchemy.smelt', ...)`；后端补 `alchemySmeltExecutor`。代码骨架 v1 已留好（ActionSpec 已存在）。

---

## 4. 数据模型 / DSL

### 4.1 新增源文件

#### 4.1.1 `alchemy_groups.json`

```json
{
  "version": 1,
  "alchemy_groups": [
    {
      "id": "group.base_herb",
      "nameKey": "alchemy.group.base_herb.name",
      "descKey": "alchemy.group.base_herb.desc",
      "minAlchemyLevel": 1,
      "entries": [
        { "itemId": "base1_moss_spore",   "weight": 50 },
        { "itemId": "base2_dewdrop_petal","weight": 30 },
        { "itemId": "base4_shadow_silk",  "weight": 5  }
      ],
      "disabled": false
    }
  ]
}
```

约束：
- `entries[].itemId` 必须存在且 `items.category == "alchemy_material"`；
- 单个 itemId 仅允许出现在一个分组（避免歧义路由）；
- 分组本身的 `minAlchemyLevel` 是"该分组可被使用"的门槛，与组内 trait 的门槛独立。

#### 4.1.2 `item_traits.json`

```json
{
  "version": 1,
  "item_traits": [
    {
      "itemId": "base2_moss_spore",
      "nameAffix": {
        "prefixKey": "alchemy.affix.prefix.glowing",
        "suffixKey": "alchemy.affix.suffix.moss"
      },
      "traits": [
        {
          "traitId": "trait.hp_regen_minor",
          "kind": "life",
          "effectType": "add_stat",
          "params": { "attr": "hp_regen" },
          "valuePerUnit": { "min": 0.04, "max": 0.08 },
          "unitBasis": 10,
          "maxBasisMultiplier": 10,
          "minAlchemyLevel": 3,
          "failBaseChance": 0.30,
          "failChancePerLevelAbove": 0.05
        }
      ]
    }
  ]
}
```

字段释义：

| 字段 | 含义 |
|---|---|
| `nameAffix.prefixKey / suffixKey` | i18n 键，物品的"形容词"与"名词"。用于自定义附魔命名拼接。**所有有 traits 的物品必须配。** |
| `kind` | `"life" \| "combat"`。单 itemId 下所有 traits 必须同 kind（build 校验） |
| `effectType + params` | 通用 effect 描述。本期支持 `add_stat`；params 必须含 `attr` |
| `valuePerUnit.{min,max}` | 每个素材贡献的随机范围（避免固定值导致过快毕业） |
| `unitBasis` | 投入数量的最小档（"基底用量"）；必须为正整数。1 = 无门槛 |
| `maxBasisMultiplier` | 最多档数；实际单次投入上限 = `unitBasis × maxBasisMultiplier` |
| `minAlchemyLevel` | 使用该 trait 的最低炼金等级 |
| `failBaseChance` | 玩家等级正好等于 `minAlchemyLevel` 时的基础失败率 |
| `failChancePerLevelAbove` | 每超 minAlchemyLevel 1 级降低的失败率 |

#### 4.1.3 `alchemy_combo_recipes.json`

```json
{
  "version": 1,
  "alchemy_combo_recipes": [
    {
      "id": "recipe.iron_resolve",
      "nameKey": "alchemy.recipe.iron_resolve.name",
      "descKey": "alchemy.recipe.iron_resolve.desc",
      "minAlchemyLevel": 8,
      "kind": "combat",
      "inputs": { "white_coin_shard": 3, "base4_shadow_silk": 2 },
      "outputAffix": {
        "effectType": "add_stat",
        "params": { "attr": "physical_attack" },
        "valueRange": { "min": 10, "max": 14 }
      },
      "outputRarity": null,
      "failBaseChance": 0.40,
      "failChancePerLevelAbove": 0.04,
      "priority": 100,
      "disabled": false
    }
  ]
}
```

匹配语义：**精确匹配独占（用户决策）**。玩家投入的 `materials` 与 `inputs` 完全一致才触发；触发时**只产 combo 附魔**，不再叠加逐项求和。

- `outputRarity == null`（默认）→ 取 `inputs` 中物品的最高稀有度；
- `outputRarity == "epic"` 等显式值 → 覆盖默认；
- 多个 combo 同时匹配时按 `priority` 降序、id 字典序选首位。

#### 4.1.4 `alchemy_transmute_rules.json`

```json
{
  "version": 1,
  "alchemy_transmute_rules": [
    {
      "id": "transmute.default",
      "returnRatio": { "minRatio": 0.80, "maxRatio": 0.95 },
      "luckyChanceBase": 0.05,
      "luckyTiers": [
        { "id": "crit_double", "weight": 80, "kind": "gold_multiplier", "value": 2.0 },
        { "id": "rare_shard",  "weight": 15, "kind": "extra_item", "itemId": "afterglow", "min": 1, "max": 2 },
        { "id": "rare_core",   "weight": 5,  "kind": "extra_item", "itemId": "glow_core", "min": 1, "max": 1 }
      ],
      "minAlchemyLevel": 1,
      "disabled": false
    }
  ]
}
```

单次点金：
```
ratio    = sampleLuckSkewed(returnRatio.minRatio, returnRatio.maxRatio, precision)
gold     = floor(item.economyBaseValue × ratio)
luck_p   = luckyChanceBase + alchemy_lucky_chance
if rand() < luck_p:
    tier = weightedPick(luckyTiers)
    case tier.kind:
        gold_multiplier: gold *= tier.value
        extra_item:      grant tier.itemId × U[min, max]
grant gold + extras
```

> 设计意图：单一加权抽奖池，DSL 简洁，总概率可预测；扩展时只需在 `luckyTiers` 加新 kind（如 `bonus_xp` / `random_group_item`）。

### 4.2 现有文件改动

| 文件 | 改动 | 兼容策略 |
|---|---|---|
| `items.json` | 每条新增 `category: string` | build.mjs 按 `type[]`+`isEquipment` 推导默认值并写回，零人工 |
| `affix_carrier_rules.json` | 新增 `maxCustomAffixes: int`（默认 0）、`allowedTraitKind: "life"\|"combat"\|"both"`（默认 "both"） | 老字段不变；本期只对 `engrave_stone` 等开通 |
| `attributes.json` | 新增 `alchemy_level`（kind=life，初始 1 级） | 老属性不动；`luck` 已存在 |
| `global-config.json` `derivedStats` 段 | 追加 `alchemy_precision / alchemy_lucky_chance / alchemy_craft_success` | 与"强化成功率"并列，复用现有引擎 |
| `attribute_exp_profiles.json` | 新增 `alchemy_level` 升级曲线 | 参照 `cooking_level` |
| `actions.json` | 新增 4 条 ActionSpec | 复用 `mainType=alchemy` |
| `action_main_types.json` | 新增 `alchemy` mainType | 紧跟 `affix` |

### 4.3 派生属性公式（落在 `global-config.json.derivedStats`）

```
alchemy_precision     = clamp(luck × 0.01  + alchemy_level × 0.005, 0, 2.5)
alchemy_lucky_chance  = clamp(luck × 0.002,                          0, 0.20)
alchemy_craft_success = clamp(luck × 0.001,                          0, 0.10)
```

设计意图：
- 三条派生独立，互不耦合，方便策划单独调；
- 上限存在，避免堆 luck 直接毕业；
- 系数与上限通过 DSL 调，代码不需要改。

### 4.4 自定义附魔命名系统

#### 4.4.1 命名算法（pure function）

```
input: materials [(itemId, count, rarity)], comboRecipeId?

1. 若有 comboRecipeId 且精确匹配
   → name = i18n(combo.nameKey)
   END

2. 若 materials 只有 1 种
   → prefix = i18n(item.nameAffix.prefixKey)
   → suffix = i18n(item.nameAffix.suffixKey)
   → name = i18n("alchemy.affix.template.single", { prefix, suffix })
   END

3. 若 materials >= 2 种
   primary    = 选 rarity 最高的 material（同稀有度按 itemId 字典序取首位）
   secondaries = 其余 materials，按 (rarity 升序, itemId 字典序) 排列
   suffix     = i18n(primary.nameAffix.suffixKey)
   prefixes   = [ i18n(item.nameAffix.prefixKey) for item in secondaries ]
   joiner     = i18n("alchemy.affix.prefixSeparator")
   name       = i18n("alchemy.affix.template.multi", { prefixes: prefixes.join(joiner), suffix })
```

确定性：同一 materials + combo 组合产出同一 name，前端预览与后端落盘必然一致。

#### 4.4.2 i18n 模板（zh-CN）

```
alchemy.affix.template.single = "{prefix}{suffix}"
alchemy.affix.template.multi  = "{prefixes}的{suffix}"
alchemy.affix.prefixSeparator = ""
```

zh-CN 示例：
- 1 素材：`发光 + 苔藓` → "发光苔藓"
- 2 素材（苔藓 + 蝠翼）：`prefixes=["发光"], suffix="蝠翼"` → "发光的蝠翼"
- 3 素材（苔藓 + 蝠翼 + 卷轴）：`prefixes=["发光","灵动"], suffix="卷轴"` → "发光灵动的卷轴"

en-US 示例：
```
alchemy.affix.template.single = "{prefix} {suffix}"
alchemy.affix.template.multi  = "{suffix} of {prefixes}"
alchemy.affix.prefixSeparator = " & "
```
→ `"Batwing of Glowing & Nimble"`

#### 4.4.3 词缀文案规范

- 命名键空间：`alchemy.affix.prefix.<slug>` / `alchemy.affix.suffix.<slug>`
- 文案风格："契合物品的优雅意境"；严禁机翻；策划/本地化团队负责填写。
- 数据完整性：build.mjs 校验 `nameAffix.prefixKey / suffixKey` 必须在 `zh-CN.ts` 与 `en-US.ts` 中存在，否则阻断构建。

### 4.5 build.mjs 新增校验清单

- `items[].category` 必在允许枚举内
- `alchemy_groups.entries[].itemId` 存在且 `category=="alchemy_material"`，且单个 itemId 全局只出现一次
- `item_traits[].itemId` 存在；同一 itemId 的 `traits[].kind` 必须一致
- `item_traits[].nameAffix.prefixKey/suffixKey` 必须在两个 i18n 文件中都存在
- `item_traits[].traits[].unitBasis >= 1`；`maxBasisMultiplier >= 1`；`valuePerUnit.min <= max`
- `alchemy_combo_recipes[].inputs` 键必须是已知 itemId；`outputAffix.valueRange.min <= max`
- 任何 `valueRange.minRatio <= maxRatio`，且都 ≥ 0
- `luckyTiers[].itemId`（若 kind=extra_item）必须存在

---

## 5. 行动队列接入

### 5.1 ActionSpec（新加到 `actions.json`）

四个 spec 共享 `mainType: "alchemy"`，这样 `WorkActionBindings["alchemy"]` 一次绑定即覆盖全部子玩法。

| id | subType | durationS | costs | itemReward | statusRequirement | statusExpReward |
|---|---|---|---|---|---|---|
| `alchemy.convert`   | convert   | 10 | `{}` | `{}` | `{alchemy_level: 1}` | `{alchemy_level: 2.0}` |
| `alchemy.bench`     | bench     | 30 | `{}` | `{}` | `{alchemy_level: 1}` | `{alchemy_level: 6.0}` |
| `alchemy.transmute` | transmute | 6  | `{}` | `{}` | `{alchemy_level: 1}` | `{alchemy_level: 1.5}` |
| `alchemy.smelt`     | smelt     | 12 | `{}` | `{}` | `{alchemy_level: 1}` | `{alchemy_level: 2.5}` |

`costs` 留空：物品/载体消耗经由 `RunContext.args` 传入并在 executor 内显式扣除（沿用 `genericExecutor` 的 `MaterialInstanceBindings` 套路）。

### 5.2 自动切换生活装备

`alchemy` 作为新 mainType 加入 `WorkActionBindings` 候选；前端"工作装备绑定"页自动列出，无需改动。

### 5.3 快进 / 取消 / 部分完成

- `alchemy.convert / .transmute`：count = N，每 tick 单次原子操作；`maxAffordableActionStartsTx` 在快进时按可承担次数截断。
- `alchemy.bench`：count = 1，单次合成不分拆；快进对它没有意义（无副作用）。
- `alchemy.smelt`（v2）：count = 选中的载体数；与 convert 同模式。

---

## 6. 数值模型

### 6.1 单调幂曲线偏置（`sampleLuckSkewed`）

```go
func sampleLuckSkewed(min, max, precision float64, rng *rand.Rand) float64 {
    u := rng.Float64()                                    // [0, 1)
    t := math.Pow(u, 1.0 / (1.0 + precision))             // precision=0 → t=u; precision↑ → t↑→1
    return min + (max - min) * t
}
```

性质：
- `precision = 0` → 均匀分布
- `precision > 0` → 单调向 max 偏移；均值随 `precision` 单调增长
- 永远落在 `[min, max]`

应用：
- 炼金台 trait 实际值：`sampleLuckSkewed(min×count, max×count, alchemy_precision)`
- 点金返还率：`sampleLuckSkewed(returnRatio.min, returnRatio.max, alchemy_precision)`

### 6.2 炼金台失败率

```
基础公式：
  failChance = failBaseChance
             - (playerLevel - minAlchemyLevel) × failChancePerLevelAbove
             - alchemy_craft_success

  clamp(failChance, 0, 1)

多 trait 时：取所有参与 trait 的 failChance 最大值（瓶颈语义）；
combo 时：直接用 combo 自身的 failBaseChance / failChancePerLevelAbove。
```

失败处理：
- 消耗 50% 投入材料（向下取整），载体不消耗
- 不写入新 affix slot
- 发放 30% 的 `statusExpReward`
- WS 推送结果：`{ failed: true, reason }`；前端弹失败原因

### 6.3 点金幸运抽奖

```
luckyChance = luckyChanceBase + alchemy_lucky_chance
if rand() < luckyChance:
    tier = weightedPick(luckyTiers)   // 按 weight 加权
    apply(tier)
```

---

## 7. 后端架构

### 7.1 文件布局

```
server/internal/store/
├── action_executor.go                    # 在 executorForAction 加 3 个 case
├── action_executor_alchemy_convert.go    # 新建：alchemyConvertExecutor
├── action_executor_alchemy_bench.go      # 新建：alchemyBenchExecutor
├── action_executor_alchemy_transmute.go  # 新建：alchemyTransmuteExecutor
├── store_alchemy.go                      # 新建：跨子玩法助手
├── models.go                             # 扩展 EquipmentAffixSlot
└── affix.go                              # reader 兼容 SourceKind 默认值

server/internal/httpapi/
└── server_alchemy.go                     # 新建：GET /api/alchemy/discoveries

server/internal/db/migrations/
└── NNNN_create_alchemy_discoveries.sql   # 新建 + 对应 .down.sql
```

### 7.2 Executor 模式

每个 executor 是一个独立的 struct，组合 `*Store`，实现 `QueueActionExecutor` 接口的 5 个 hook：

```go
type alchemyConvertExecutor struct{ s *Store }

func (e *alchemyConvertExecutor) ValidateAndNormalizeArgs(ctx, raw) (Args, error) { ... }
func (e *alchemyConvertExecutor) FreezeRunContextTx(ctx, tx, ...) (RunContext, error) { ... }
func (e *alchemyConvertExecutor) StartRunTx(ctx, tx, ...) error { ... }
func (e *alchemyConvertExecutor) SettleNormalTx(ctx, tx, ...) (SettleResult, error) { ... }
func (e *alchemyConvertExecutor) SettleFastForwardTx(ctx, tx, ..., budget FastForwardBudget) (SettleResult, error) { ... }
func (e *alchemyConvertExecutor) ClassifyStartError(err error) (cancel bool, reason string) { ... }
```

所有"标准动作行为"（资源校验、装备加成、duration 缩短、attr exp）通过把 `genericExecutor` 已暴露的助手抽成可重用方法/函数（如 `resolveDurationAndBonusTx / maxAffordableActionStartsTx / scaleAttrExp / applyAttrExpTx`）直接复用；executor 只写"专属"逻辑。

### 7.3 RunContext 与状态机

| Executor | RunContext 字段 | 说明 |
|---|---|---|
| convert | `{ sourceItemId, groupId, sourceCount }` | 每 tick 消耗 1，按 group entries 抽 1 |
| bench | `{ carrierInstanceId, materials map[itemId]int64, comboRecipeId?, frozenSnapshot }` | `frozenSnapshot` 在 freeze 时把 carrier 当前 slot 状态、属性等冻结，防 freeze→start 间被外部修改导致中间状态错位 |
| transmute | `{ itemId, instanceId?, totalCount }` | 每 tick 消耗 1，roll 返还 + 幸运 |

### 7.4 自定义附魔的存储

复用 `equipment_instance.AffixesRaw` JSON 列。`EquipmentAffixSlot` 扩展：

```go
type EquipmentAffixSlot struct {
    SlotIndex       int                    `json:"slotIndex"`
    AffixID         string                 `json:"affixId"`
    RarityID        string                 `json:"rarityId"`
    SourceKind      string                 `json:"sourceKind,omitempty"`      // "template" | "craft"；空表示 template
    DisplayNameKey  string                 `json:"displayNameKey,omitempty"`  // i18n key（custom：模板键 / combo：固定键）
    DisplayNameArgs map[string]interface{} `json:"displayNameArgs,omitempty"` // 模板参数（custom）
    EffectType      string                 `json:"effectType,omitempty"`
    EffectParams    map[string]interface{} `json:"effectParams,omitempty"`
    AttrMods        map[string]float64     `json:"attrMods,omitempty"`
}
```

编码规则：
- `AffixID = "custom:" + sha1(effectType + sortedParamsJSON + roundedValue)[:12]`
  - 数值用 `valuePrecision` 位四舍五入后再 hash；这样两次同 effect/同结果的合成会聚合到同一 `AffixID`，让 `AffixMaxEffectivePerBody` 自然生效
- `RarityID = max(materials.rarity)`（combo 可被 `outputRarity` 覆盖）
- `SourceKind = "craft"`
- `DisplayName*` 由命名算法填

向后兼容：旧 JSON 无 `SourceKind` → JSON 解码默认空字符串 → reader 归一为 `"template"`。

### 7.5 容量约束

| 约束 | 计算方式 | 实施位置 |
|---|---|---|
| 总槽位 | `len(slots) ≤ template.maxAffixSlots` | bench `StartRunTx` 校验；其它路径已存在 |
| 自定义子限 | `count(slots[i].SourceKind=="craft") ≤ carrier.maxCustomAffixes` | bench `StartRunTx` 校验 |
| 单 affix 单装备上限 | `FilterEffectiveAffixSlotsByLimit(affixId, equip)` | 现有路径 |
| 单 affix 全身上限 | `FilterEffectiveAffixSlotsByLimit(affixId, body)` | 现有路径 |
| 载体 kind | materials 全部 kind ↔ carrier.allowedTraitKind 兼容 | bench `StartRunTx` 校验 |

### 7.6 store_alchemy.go（助手）

```go
// 数值
func sampleLuckSkewed(min, max, precision float64, rng *rand.Rand) float64

// 派生属性读取（一次查询打满 3 项）
func (s *Store) resolveAlchemyDerivedTx(ctx, tx, userID int64) (AlchemyDerived, error)

// 命名（pure，前端会有 TS 镜像；fixture 共享）
func ComputeCustomAffixName(materials []MaterialEntry, comboRecipeId string, catalog *catalog.Catalog, t i18n.Translator) ComputedName

// 自定义附魔编码
func EncodeCustomAffixID(effectType string, params map[string]interface{}, value float64, valuePrecision int) string

// 子流程：分组转换一次
func (s *Store) alchemyGroupConvertOnceTx(ctx, tx, userID, sourceItemId, groupId) (rewardItemId string, err error)

// 子流程：炼金台一次合成
func (s *Store) alchemyBenchCraftTx(ctx, tx, userID, carrierInstanceId, materials, comboRecipeId) (CraftResult, error)

// 子流程：点金一次
func (s *Store) alchemyTransmuteOnceTx(ctx, tx, userID, itemId, instanceId int64, rule *catalog.AlchemyTransmuteRule) (TransmuteResult, error)

// 发现表写入（幂等）
func (s *Store) markAlchemyDiscoveredTx(ctx, tx, userID, kind, id) error
```

### 7.7 发现表

```sql
CREATE TABLE alchemy_discoveries (
  user_id        BIGINT      NOT NULL,
  kind           VARCHAR(16) NOT NULL,  -- "group" | "trait" | "combo"
  id             VARCHAR(64) NOT NULL,
  discovered_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (user_id, kind, id)
);
CREATE INDEX idx_alchemy_discoveries_user ON alchemy_discoveries (user_id);
```

写入时机（都用 `INSERT ... ON CONFLICT DO NOTHING`）：
- 转换材料成功：写 `(group, groupId)` + 产物 trait（若产物 itemId 有 trait）
- 炼金台成功：写每个参与 trait `(trait, traitId)`；命中 combo 时额外写 `(combo, recipeId)`
- 任何路径下"物品入背包"时：若该 itemId 有 trait，写 `(trait, traitId)`

读取：`GET /api/alchemy/discoveries → { groups, traits, combos }`。

### 7.8 错误分类

| Err | ClassifyStartError | i18n |
|---|---|---|
| `ErrAlchemyLevelInsufficient` | `(cancel, "STATUS_REQUIREMENT")` | `game.alchemy.err.levelInsufficient` |
| `ErrAlchemyMixedKind` | `(cancel, "TARGET_INVALID")` | `game.alchemy.bench.mixedKindError` |
| `ErrAlchemyKindMismatch` | `(cancel, "TARGET_INVALID")` | `game.alchemy.bench.kindMismatch` |
| `ErrAlchemyCarrierFull` | `(cancel, "TARGET_INVALID")` | `game.alchemy.bench.carrierFull` |
| `ErrAlchemyMaterialOverflow` | `(cancel, "TARGET_INVALID")` | `game.alchemy.bench.materialOverflow` |
| `ErrAlchemyGroupNotFound` | `(cancel, "TARGET_INVALID")` | `game.alchemy.err.groupNotFound` |
| `ErrAlchemyBasisInvalid` | `(cancel, "TARGET_INVALID")` | `game.alchemy.bench.basisInvalid` |

"炼金失败"不作为错误返回，而是 settle 结果的 `failed: true`，让队列正常推进。

### 7.9 并发与事务

- 每次 settle 一次 tx；开头 `lockUserTx`；bench 加 `LockInstanceTx(carrierInstanceId)`
- 转换/点金的快进迭代用 `maxAffordableActionStartsTx` 截断；不会出现"扣了材料但没产出"的情况
- 点金天然幂等：队列行 = 一次 settle；不需 idempotency key

---

## 8. 前端架构

### 8.1 菜单 + 路由

`web/src/views/game/shell/index.vue`：

```ts
// routeBackedTabEntries（行 482-498 附近）追加
["alchemy",           "/game/alchemy"],
["alchemy.convert",   "/game/alchemy/convert"],
["alchemy.bench",     "/game/alchemy/bench"],
["alchemy.transmute", "/game/alchemy/transmute"],
["alchemy.smelt",     "/game/alchemy/smelt"],

// menuRegistry（行 540 附近）在 affix 项下方插入
{ type: "submenu", key: "alchemy", icon: "game-icons:cauldron", index: "alchemy",
  titleKey: "game.menu.alchemy", children: [
    { key: "alchemy.convert",   index: "alchemy.convert",   titleKey: "game.menu.alchemy.convert"   },
    { key: "alchemy.bench",     index: "alchemy.bench",     titleKey: "game.menu.alchemy.bench"     },
    { key: "alchemy.transmute", index: "alchemy.transmute", titleKey: "game.menu.alchemy.transmute" },
    { key: "alchemy.smelt",     index: "alchemy.smelt",     titleKey: "game.menu.alchemy.smelt"     },
  ]
}
```

### 8.2 视图层次

```
web/src/views/game/alchemy/
├── index.vue              # <router-view/> 父壳
├── convert/index.vue      # AlchemyConvertPage
├── bench/index.vue        # AlchemyBenchPage
├── transmute/index.vue    # AlchemyTransmutePage
└── smelt/index.vue        # 渲染既有 AffixSmeltPanel

web/src/components/game/alchemy/
├── MaterialPickerByGroup.vue
├── MaterialPickerByTrait.vue
├── TraitSummaryCard.vue
├── CarrierPicker.vue
├── AlchemyPrecisionBadge.vue
├── AlchemyNamePreview.vue          # 共享给 bench 用
├── AlchemyHandbookDrawer.vue       # 手册抽屉
└── composables/
    ├── useAlchemyDiscoveries.ts    # 拉取并缓存 /api/alchemy/discoveries
    ├── useAlchemyNaming.ts         # TS 镜像命名算法
    └── useAlchemyFormulas.ts       # 失败率 / 期望值 / 幸运率 即时计算
```

### 8.3 命名算法的"前后端镜像"

- 后端落盘时计算并存到 `DisplayName*`
- 前端"预览"实时计算，结果须与后端一致
- 用一份共享 fixture（`packages/game-data/source/json/alchemy_naming_fixtures.json` 或 build 阶段生成 `tests/alchemy_naming.cases.json`）跑前端 / 后端单测，确保两份实现等价

### 8.4 提交方式

```ts
const { enqueueWithArgs } = useActionSubmit()

// convert
enqueueWithArgs("alchemy.convert", count, { sourceItemId, sourceCount: count, groupId })

// bench
enqueueWithArgs("alchemy.bench", 1, { carrierInstanceId, materials, comboRecipeId })

// transmute
enqueueWithArgs("alchemy.transmute", totalCount, { itemId, instanceId, totalCount })

// smelt v1: 沿用 POST /api/equipment/affix/smelt
```

### 8.5 i18n 键清单（zh-CN 示例）

```
# 菜单
game.menu.alchemy = "炼金"
game.menu.alchemy.convert = "转换材料"
game.menu.alchemy.bench = "炼金台"
game.menu.alchemy.transmute = "点金"
game.menu.alchemy.smelt = "熔炉"

# 共用
game.alchemy.precisionBadge = "炼金精度 {value}"
game.alchemy.luckyChanceBadge = "幸运概率 +{value}%"

# 炼金台
game.alchemy.bench.title = "炼金台"
game.alchemy.bench.successRate = "预期成功率 {rate}"
game.alchemy.bench.mixedKindError = "生活特性与战斗特性不能混合"
game.alchemy.bench.kindMismatch = "载体只能承载 {kind} 类型的特性"
game.alchemy.bench.carrierFull = "载体自定义附魔已达上限 ({max})"
game.alchemy.bench.materialOverflow = "{itemName} 单次最多投入 {max} 个"
game.alchemy.bench.basisInvalid = "{itemName} 需以 {basis} 个为单位投入"
game.alchemy.bench.basisUnit = "基底用量 {basis} (每档)"
game.alchemy.bench.comboMatched = "命中组合：{name}"
game.alchemy.bench.namePreview = "附魔名称：{name}"
game.alchemy.bench.craftFailed = "炼金失败，部分材料损失"

# 转换
game.alchemy.convert.noGroup = "该物品不可参与炼金转换"

# 点金
game.alchemy.transmute.returnRange = "返还范围 {min}% ~ {max}%"
game.alchemy.transmute.luckyTriggered = "幸运触发：{tierName}"

# 手册
game.alchemy.handbook.title = "炼金手册"
game.alchemy.handbook.sectionGroups = "已发现的分组 {n}/{total}"
game.alchemy.handbook.sectionTraits = "已发现的特性 {n}/{total}"
game.alchemy.handbook.sectionCombos = "已发现的配方 {n}/{total}"
game.alchemy.handbook.unknown = "未发现"

# 错误
game.alchemy.err.levelInsufficient = "炼金等级不足，需要 {required}"
game.alchemy.err.groupNotFound = "未找到对应的炼金分组"

# 命名模板（前后端共享）
alchemy.affix.template.single = "{prefix}{suffix}"
alchemy.affix.template.multi  = "{prefixes}的{suffix}"
alchemy.affix.prefixSeparator = ""

# 词缀（策划填写，下方仅占位示例）
alchemy.affix.prefix.glowing = "发光"
alchemy.affix.suffix.moss = "苔藓"
alchemy.affix.prefix.nimble = "灵动"
alchemy.affix.suffix.batwing = "蝠翼"

# 动作 + 属性 + 分组 + 配方（按 DSL id 派生，此处略）
```

### 8.6 老入口处理

- `web/src/views/game/affix/index.vue` 的 smelt Tab 直接删除（按用户决策）
- 进入 affix 视图时如带 `?tab=smelt` → 单次 `router.replace('/game/alchemy/smelt')` 兜底书签

---

## 9. combat-model-vue3 编辑器接入

`packages/combat-model-vue3/src/model/catalog.ts`：

```ts
// COMBAT_TABLES 追加
'alchemy_groups',
'alchemy_combo_recipes',
'alchemy_transmute_rules',
'item_traits',

// TABLE_LABELS 追加
alchemy_groups:         '炼金分组',
alchemy_combo_recipes:  '炼金组合配方',
alchemy_transmute_rules:'点金规则',
item_traits:            '物品特性',
```

字段类型：
- 大多数字段（数字、字符串、枚举）走现有 grid 自动渲染
- `alchemy_groups.entries` / `alchemy_combo_recipes.inputs` 用现有 `numberMap` kind
- `item_traits.traits` 用 JSON kind（v1 可接受，参考既有 `effects` 字段；后续若编辑负担大可加 `traitList` 自定义 widget）
- `outputAffix` / `valueRange` 用 JSON kind
- `items` grid 上新增 `category` 列（select 类型）

`makeDefaultRow` 为四张表各加一个最小可用模板，便于策划"加新行"。

---

## 10. 风险与边界

| 风险 | 处置 |
|---|---|
| 玩家在动作开始前升级炼金等级 | requirement 检查在 `StartRunTx`，升级只能放宽要求 → 无需取消 |
| 载体上既有模板附魔又有自定义附魔 | 共享槽位预算：总数 ≤ `template.maxAffixSlots`；`maxCustomAffixes` 是其中"自定义类型"的子限 |
| 1 个最高稀有度 + 大量低稀有度 → 高稀有度产物 | 用户接受当前语义；v2 可加 `minStackForRarity` 字段（已铺垫） |
| 两素材同稀有度时谁出 suffix | 按 itemId 字典序选首位，结果可重现 |
| 多素材公共 suffixKey | 拼接结果如"发光灵动苔藓"，自然连贯，无需特殊处理 |
| nameAffix i18n key 缺失 | build.mjs 阻断构建 |
| 玩家投入非 unitBasis 整倍数 | 前端禁用提交；后端 `ValidateAndNormalizeArgs` 二次校验返回 `ErrAlchemyBasisInvalid` |
| 组合配方与逐项求和的优先级 | 精确匹配独占；多 combo 同时匹配按 `priority` 降序 + id 升序 |
| 旧 items 无 category | build.mjs 推导默认并写回，零人工 |
| 浮点漂移破坏聚合 | hash 输入前对数值做 `valuePrecision` 位四舍五入 |
| 快进时材料中途耗尽 | `maxAffordableActionStartsTx` 在 settle 前截断 |
| 自定义附魔走熔炉给什么 | 按其 RarityID（材料最高稀有度）走现有 `affix_smelt_rules`，无需特例 |
| 玩家未捏 `luck` | 派生属性 `EnsurePlayerAttributes` 兜底 0；`sampleLuckSkewed` 在 precision=0 时退化均匀 |

---

## 11. 验证计划

### 11.1 端到端（手动）

1. `node packages/game-data/scripts/build.mjs`：观察新 4 张表与 `items.category` 生成，零校验错误
2. `cd server && go mod tidy && go test ./internal/store/...`
3. `cd web && npm run typecheck && npm run build && npm run test:unit`
4. 浏览器：
   - 调试授予 50× `base1_moss_spore`，入队 `alchemy.convert` count=50，开快进 → 收到 50 个分组内物品
   - 绑定 `alchemy` mainType 的生活装备 → 入队任意子动作 → 装备自动切换
   - 入队 `alchemy.bench`：engrave_stone + 100×moss_spore → 载体多 1 条自定义附魔，数值落 [4.0, 8.0]
   - 把 luck 拉高，重复合成 30 次 → 数值均值显著靠近 8.0
   - 入队 `alchemy.transmute` 10 个物品 → 入账落区间，偶发暴击 / 稀有掉落
   - 命中 combo（精确投入）→ 仅产 combo 附魔，名字用 combo nameKey
   - 多余材料 → combo 不触发，按逐项求和
   - 混 life + combat 材料 → 提交被拒
   - 手册抽屉显示"已发现 N/总数 M"，未发现项 ???

### 11.2 自动化

- `action_executor_alchemy_convert_test.go`：固定种子 10000 次抽样，频率与权重偏差 < 2σ
- `action_executor_alchemy_bench_test.go`：
  - 单 kind 校验
  - 载体 kind 不匹配拒收
  - `maxCustomAffixes` / `maxAffixSlots` 上限
  - combo 精确匹配 + 多 combo 优先级
  - 失败路径：50% 材料消耗、30% XP、载体不动
  - AffixID hash 聚合（同输入两次 → 同 ID）
- `action_executor_alchemy_transmute_test.go`：返还落区间、幸运 tier 加权命中分布、金币 ledger
- `store_alchemy_sample_test.go`：`sampleLuckSkewed` 性质（precision=0 均匀、precision↑ 均值单调↑、永远 [min,max]）
- `store_alchemy_concurrency_test.go`：并发点金同物品总金币 = 串行总金币（`lockUserTx` 生效）
- `store_alchemy_naming_test.go`：命名算法对所有 fixture cases 通过
- 前端 `useAlchemyNaming.spec.ts` 使用同一 fixture，确保前后端一致
- `build.mjs` 自测：混 kind / 缺 nameAffix / 缺 i18n / 非法 unitBasis / 悬挂 itemId → 全部阻断

---

## 12. 迁移与发布

### 12.1 数据库迁移

- `NNNN_create_alchemy_discoveries.sql` + `.down.sql`
- 无破坏性 schema 改动（无 alter / drop）

### 12.2 配置迁移

- `items.json` 加 `category`：由 build.mjs 推导默认值并写回；提交时一并 review
- `affix_carrier_rules.json` 加 `maxCustomAffixes / allowedTraitKind`：默认空/0，老载体不受影响

### 12.3 分 PR 发布顺序

| PR | 内容 | 风险 |
|---|---|---|
| PR0 | 本设计文档 | 0 |
| PR1 | 数据层：attributes + alchemy_level + derived + items.category + build 校验 + combat-model-vue3 表注册 | 低（纯加字段）|
| PR2 | 转换 + 点金 + 发现表 + 菜单"炼金"父项 + i18n 基础 | 中（首次入队两个新 mainType）|
| PR3 | 炼金台：item_traits + combo + bench executor + 自定义附魔 SourceKind + 命名 + 手册抽屉 + 失败率 UI | 高（载体写入新结构）|
| PR4 | 熔炉搬家：菜单 smelt 子项启用 + 附魔页 smelt tab 移除 + 路由跳转兜底 | 低 |
| PR5（后续）| 熔炉入队 | 中 |

---

## 13. 扩展路线图

本期之外、但 schema 已为之留路：

| 方向 | 触达 |
|---|---|
| trait 稀有度门槛（防止"碰瓷"） | trait 加 `minStackForRarity: { rarity, count }`；命名/稀有度算法在算最高稀有度时按该约束过滤 |
| 自定义附魔随时间衰减 | `EffectParams` 加 `decayPerDay`；renderer 读取并按当前服务器时间显示 |
| 多产出 combo | `outputAffix` → `outputAffixes: [...]`；bench executor 按数组迭代写入 slot |
| 配方解锁机制 | `alchemy_discoveries` 增加 `kind="recipe_unlocked"`；bench 入口校验 |
| 炼金"工艺等级"分支（专精方向） | 新增 `alchemy_specialization_id` 属性；`item_traits` / combo 加 `specializationGate` |
| 后期熔炉入队 | `alchemy.smelt` ActionSpec 已存在；只需加 executor + 切前端提交方式 |
| 炼金台"批量"合成 | `alchemy.bench` count > 1，每 tick 一次合成；UI 默认 count=1 不影响现状 |
| 季节性 / 活动专属 trait | trait 加 `availabilityWindow: { from, to }`；build / runtime 双重校验 |

---

## 14. 关键文件清单

后端：
- `server/internal/store/action_executor.go`（dispatch case）
- `server/internal/store/action_executor_alchemy_*.go`（3 新文件）
- `server/internal/store/store_alchemy.go`（新建）
- `server/internal/store/models.go`（`EquipmentAffixSlot` 扩展）
- `server/internal/store/affix.go`（reader 兼容 `SourceKind`）
- `server/internal/httpapi/server_alchemy.go`（新 GET /api/alchemy/discoveries）
- `server/internal/db/migrations/NNNN_create_alchemy_discoveries.sql` + `.down.sql`

数据：
- `packages/game-data/scripts/build.mjs`（4 表导出 + 校验 + items.category 推导）
- `packages/game-data/source/json/`：
  - 新增：`alchemy_groups.json`、`item_traits.json`、`alchemy_combo_recipes.json`、`alchemy_transmute_rules.json`
  - 改：`items.json`、`affix_carrier_rules.json`、`attributes.json`、`global-config.json`、`attribute_exp_profiles.json`、`actions.json`、`action_main_types.json`

前端：
- `web/src/views/game/shell/index.vue`（菜单 + 路由）
- `web/src/views/game/alchemy/**`
- `web/src/components/game/alchemy/**`
- `web/src/views/game/affix/index.vue`（删 smelt tab + 兜底重定向）

编辑器：
- `packages/combat-model-vue3/src/model/catalog.ts`

i18n：
- `web/src/i18n/lang/zh-CN.ts`
- `web/src/i18n/lang/en-US.ts`

---

## 15. 决策回顾

下表汇总本设计中的所有关键决策与理由，作为后续维护者的参考。

| 决策 | 选项 | 理由 |
|---|---|---|
| 路由形式 | 独立子路由 | 组件可拆分、可深链接；与现有 affix 不冲突 |
| 自定义附魔存储 | 复用 `AffixesRaw` + `SourceKind` 字段 | 零数据库迁移；现有读路径自动兼容 |
| combo 匹配语义 | 精确匹配独占 | 玩家心智清晰；DSL 简单；与"多余材料则不触发"语义一致 |
| 熔炉入队 | v1 不入队 | 范围控制；现有路径无回归；ActionSpec 占位便于 v2 |
| 物品 category 枚举 | 7 类（alchemy_material / currency / loot / misc / carrier / equipment / consumable） | 覆盖现有物品；后续可扩展 |
| 派生属性来源 | `luck`（已存在）+ `alchemy_level` | 复用基础属性；不引入新概念；策划可调系数 |
| 派生属性配置位置 | `global-config.json.derivedStats` | 与现有"强化成功率"派生并列；引擎成熟 |
| 槽位关系 | 共享 maxAffixSlots，maxCustomAffixes 是子限 | 装备总数受限，自定义类型有进一步上限；模型清晰 |
| 自定义附魔稀有度 | 取材料最高稀有度 | 简单可预测；激励玩家追求高稀有度材料 |
| 命名系统 | 前缀+后缀 + i18n 模板 | 自动生成、契合 i18n、可扩展到任意素材组合 |
| valuePerUnit | 范围 `{min, max}` | 避免固定值导致毕业过快；与 luck 派生形成"幸运越高越接近上限"的成长感 |
| 单位基底 | `unitBasis` + `maxBasisMultiplier` | 部分 trait 需 10 个为一档；策划可灵活控制 |
| 失败机制 | 与等级差挂钩 + luck 加成；失败损 50% 材料 + 30% XP | 给"升等级"实质收益；不至于让低等级玩家完全无产出 |
| 转换炼炸 | 不炼炸 | 与现有回收对标，体验顺畅 |
| 全身属性硬上限 | 不加 | 已有 maxEffectivePerEquipment / maxEffectivePerBody；不再加新限制 |
| 载体 kind | `allowedTraitKind: life/combat/both` | 明确 UX；策划可决定哪些载体走哪条路 |
| 自定义附魔拆卸 | 只能走熔炉 | 玩法简单；与现有"拆卸丢失附魔"语义一致 |
| 配方书 | 已发现 N / 总数 M | 玩家可感知探索进度；不过度铺路 |
| 等级成长感 | 只是门槛 | 最纯净；避免叠床架屋 |
| 熔炉老入口 | 直接移除 | 代码干净；兜底重定向防书签 404 |
| 失败 i18n | 不作为 error 返回，settle 结果带 failed | 让队列正常推进；UX 与"任务完成弹窗"统一 |
