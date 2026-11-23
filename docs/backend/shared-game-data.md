# 前后端共享游戏数据管线

## 目标
- 只维护一份游戏配置（物品/装备/敌人等），同时供后端和前端消费。

## 目录
- 源数据：`packages/game-data/source/`
  - `json/*.json`（当前主维护形态，按数据域拆分）
  - `workbooks/*.xlsx`（推荐策划入口，按数据域拆分的多个 Excel）
  - `game-data.xlsx`（仅兼容旧流程，只有在 `workbooks/` 目录为空时才会读取）
  - `version.json`
- 生成产物：`packages/game-data/generated/`
  - `game-data.json`
  - `game-data-manifest.json`
  - `chunks/core.json`
  - `chunks/<table>.json`（按表拆分）
  - `game_data.go`
  - `game-data.ts`
- 前端同步类型：`web/src/types/generated/game-data.ts`

## 共享全局配置
- 动作队列上限与猫咪助手队列上限由 `packages/game-config/source/shared-global-config.json` 下的 `action.queueLaneLimit` / `action.kittyQueueLaneLimit` 统一配置，前后端都从该文件读取。
- 猫咪战斗等级默认上限由 `shared-global-config.json` 的 `kitty.battleLevelCapDefault` 统一定义，单只猫咪可在运行时通过喂食提升。
- 猫咪性格预设为独立表：`packages/game-data/source/json/kitty_personalities.json`（不再放在 `shared-global-config.json` 中）。

## 构建
```bash
node packages/game-data/scripts/build.mjs
```

### Excel 约定
- 当 `source/workbooks/*.xlsx` 存在时，构建脚本会按文件名排序读取所有工作簿，并把**同名 Sheet 追加合并**。
- 当 `source/workbooks/` 为空时，仍兼容读取 `source/game-data.xlsx`。
- 当前仓库里 `source/json/*.json` 也处于启用状态，常见表如 `items`、`actions`、`skills`、`equipment_templates`、`world`、`kitty_personalities` 已按域拆分存放。
- 解析器位于 `packages/game-data/scripts/read_xlsx.py`（标准库实现，无第三方依赖）。
- Sheet 名需要与表名一致，例如：
  - `items`、`items_*`（会按 `items` 结构合并解析）
  - `item_types`、`icons`
   - `actions`、`action_main_types`、`action_sub_types`
  - 或按业务拆分 `actions_<mainType>`（Sheet 数量不固定，行内仍需 `mainType`）
  - `attribute_exp_profiles`、`warehouse_rules`
- 单元格中若填写 JSON 字符串（如 `{}` 或 `[]`），会自动解析为对象/数组。
- 构建脚本会把游戏条目结构自动同步到 `web/src/locales/zh-CN.ts` 和 `web/src/locales/en-US.ts` 的自动托管区块。
- 构建脚本会额外输出前端拆包友好的分片产物：
  - `generated/chunks/core.json`（`version/items/icons`）
  - `generated/chunks/*.json`（其余表按表拆分）
  - `generated/game-data-manifest.json`（分片索引）

### 多 Excel 命名规范（建议）

| Excel 文件名 | 主要 Sheet（数据来源） |
|---|---|
| `items.xlsx` | `items`、`items_*`（可按业务分表）、`item_types` |
| `actions.xlsx` | `actions`、`action_main_types`、`action_sub_types`（可选扩展 `actions_<mainType>`） |
| `combat.xlsx` | `skills`、`skill_type`、`buff`、`buff_type`、`buff_effects`、`combat_config_rules`、`skill_rule_sets`、`item_rule_sets`、`effects` |
| `economy.xlsx` | `market_fee_tiers`、`market_npc_offers`、`drop_tables`、`warehouse_rules` |
| `growth.xlsx` | `attribute_exp_profiles`、`daily_tasks`、`daily_task_refresh_rules`、`attributes` |
| `equipment.xlsx` | `equipment_templates`、`affix_definitions`、`affix_detach_rules`、`affix_enchant_rules`、`affix_carrier_rules` |
| `common.xlsx` | `rarity_definitions` |
| `world.xlsx` | `enemies`、`events_world`、`activity_mutex_rules` |

### 示例模板生成

已提供模板生成脚本：

```bash
python3 packages/game-data/scripts/generate_workbook_templates.py
```

执行后会在 `packages/game-data/source/workbooks/` 生成上述命名的示例 `.xlsx` 文件，并为每个 Sheet 写入表头，便于直接填数。

### 从现有 JSON/CSV 迁移到多工作簿

已提供迁移脚本（可重复执行，会按当前源数据重写 `workbooks/*.xlsx`）：

```bash
python3 packages/game-data/scripts/migrate_json_to_workbooks.py
```

推荐迁移流程：
1. 先执行模板脚本（若 workbooks 目录为空）
2. 执行迁移脚本导入历史 `*.json/*.csv`
3. 之后只维护 `workbooks/*.xlsx`
4. 执行 `node packages/game-data/scripts/build.mjs` 产出 JSON/TS/Go

## 校验规则
- `id` 唯一且为字符串
- `specialization_trees` 当前持久化兼容版本为 `specialization-v6`。战斗等级 200/300/500 分别累计获得 40/60/65 点，且 `maxActiveRootNodes=3`，升级已激活根节点不额外占路线名额。`rulesetVersion` 只表示旧投入能否继续解释：使用全新稳定 ID 增加专精树或节点，且不修改既有节点、点数与装备槽契约时，保持 `rulesetVersion` 不变并递增 `balanceRevision`；删除或重命名既有 ID、修改既有等级成本/前置/互斥关系、点数上限与来源、根路线上限或装备槽位拓扑时，才提升 `rulesetVersion`。真正不兼容的存量状态会由服务端免费清空投入，同时在一个事务内把不再合法的装备按整件卸下并退回背包、重算属性与装备条件、停用失效技能，并通过玩家专精接口保留最近一次通知。
- `balanceRevision` 不写入玩家专精状态表。战斗第 1 波生成快照时会连同当前 DSL 写入运行态与摘要，后续波次继续沿用该快照；强制应用新版需要先更新 Server/Battle Worker，再重置活跃战斗。带 `expectedSpecialization` 的管理端热更新流程会读取带 TTL 的 Worker 心跳，只有全部存活 Worker 与 Server 的专精版本一致时才允许重置。
- `items/actions/world/enemies` 支持 `disabled`（boolean，默认 `false`）。
  - `action.disabled=true`：后端不接受入队/执行（当前沿用 `WS_ACTION_NOT_FOUND` 语义）。
  - `world.disabled=true`：不出现在战斗世界列表，且不可用于建房/改房。
  - `enemy.disabled=true`：世界引用时运行时自动忽略（不做构建期阻断）。
  - `item.disabled=true`：前端定义初始化阶段直接剔除（`list/get` 均不可见），依赖该定义的跨链接入口会自然失效。
  - 前端 `item/action/enemy` 定义服务在模块初始化时统一剔除 disabled 数据，避免 quick-ref/弹窗等跨链接入口解析到已禁用定义。
  - 服务端 `battle worlds` 列表与 `battleWorldConfigByID` 共享同一套 world 归一化逻辑，确保 disabled world/enemy 过滤语义一致。
- `type` / `rarity` 必填字符串
- `stackable=true` 代表可无限叠加（受数据库 `BIGINT` 上限保护）
- 物品定义不再支持 `width` / `height` / `backpackGrid` 字段（仓库为无网格模式）
- `baseAttrs` 必须为对象，值为 number
- `items.baseAttrs` 与 `equipment_templates.baseAttrs` 都支持整数与小数（含百分比语义值）
  - 示例：`{"physics_defence":10,"physics_damage_resist":0.0216}`
- `items.kittyFeed`（可选）：
  - 结构：`{ "battleLevelCapGain": number }`
  - 约束：`battleLevelCapGain > 0` 时该物品必须包含 `food` 类型。
  - 语义：该物品可用于猫舍喂食接口，提升单只猫咪的 `battle_level_cap`。
- `affix_enchant_rules` / `affix_detach_rules` / `affix_definitions` / `affix_carrier_rules` 都属于装备扩展表，构建时会同步到前后端共享类型。
- `iconId` / `nameKey` / `descKey` / `effectDescKey` 为空时会按 `id` 自动生成默认值
- `attributes.icon` 用于 Iconify 图标名（例如 `game-icons:...`），前端属性卡片与状态标签优先渲染它；为空时再回退 `iconId` / emoji。
- `affix_definitions.nameKey` / `affix_definitions.descKey` 由构建脚本按 `id` 强制生成（不依赖手填列）。
- `affix_definitions.type` 必填且只能是 `work|battle`（构建阶段强校验）。
- `affix_definitions` 唯N字段：
  - `maxEffectivePerEquipment`：单件装备上该 affix 的最大生效数量（`0` 表示不限制）。
  - `maxEffectivePerBody`：单位全身已装备中该 affix 的最大生效数量（`0` 表示不限制）。
  - 写入阶段：`附魔/贴附` 校验装备维度上限，`装备` 校验全身维度上限（失败即拒绝写入）。
  - 运行阶段：历史超限数据仅按槽位 `slotIndex` 升序生效前 N 个，其余不参与属性/战斗结算。
- `affix_enchant_rules.allowedAffixIds=[]` 表示从全部启用词条随机（再叠加装备词条白名单/稀有度交集过滤）。
- `affix_enchant_rules.allowedAffixScopes` 支持按 `equipType + slot + weaponClass` 覆盖附魔池。
  - 优先级：`equipType+slot+weaponClass` > `equipType+slot` > `equipType+weaponClass` > `equipType` > 回退 `allowedAffixIds`。
  - 同一优先级命中多个 scope 时按 `allowedAffixIds` 做并集去重。
- `affix_enchant_rules.costs` 追加额外资源消耗，允许为空 `{}`；若非空，服务端会在附魔事务中一并校验和扣除。
- `equipment_templates.affixAllowedIds=[]` 表示该装备不限制词条池（仅作为可选过滤口子）。
- `equipment_templates.equipTargets`（可选）：
  - 允许值：`player`、`kitty`
  - 空数组/同时包含两者：不限制
  - 仅包含一者：表示该装备只允许对应对象佩戴
- 武器普攻新增字段（`equipment_templates`）：
  - `basicAttackEffects`: `skills.effects` 同结构数组；优先使用，未填时回退到旧三字段
  - `basicAttackTargetDirection`: `vanguard|midguard|rearguard`（可选）
  - `targetSeekStrategy`: `rear_to_front|front_to_rear|random_alive`（可选）
- 装备战斗钩子字段（`equipment_templates.battleHooks`）：
  - 结构：`[{ id?, trigger, priority?, conditions?, effects[] }]`
  - 条件可复用 Buff 条件；`self_skill_cooldown_count_gte` 会按技能范围统计正在冷却的技能，`equipment_enhance_level_gte` 按该钩子来源装备的强化等级判断；`player_party_equipped_unit_count_eq` 按装备模板 ID 统计队伍中的真实玩家单位，召唤物与幻象不计。
  - `on_after_skill_cooldown_tick` 只在单位主动施放技能并完成本次普通冷却递减后触发；镜像施法、装备追施和空行动不触发，并通过上下文提供本次主动技能 ID。
  - `adjust_skill_cooldown` 使用有符号 `amount`，并支持 `skillCategories`、`skillIds`、`excludeSkillIds`、`includeBasic`、`selectionMode=all|max_remaining` 与 `maxSkills`。技能分类直接引用 `skill_types.id`，不另建法系分类。
  - `transfer_skill_cooldown` 在同一单位的两组技能选择器之间原子转移冷却。`to.selectionMode=context_skill` 指向本次主动技能，`from` 可用现有技能分类筛出另一项正在冷却的技能并以 `max_remaining` 选择；任一端不满足时两端都不改动。`waiveIncreaseBuffId` 可配置一次性免除 `to` 端增加量的通用 Buff，成功转移后才消费。
  - `add_buff` 结构为 `{ kind: "add_buff", buffId, durationTurns?, targetMode?, targeting? }`。
  - `targetMode`：`self|source|targeting`；`targeting` 复用技能/普攻目标规则结构。
- 装备默认词条装配字段（`equipment_templates.defaultAffixSlots`）：
  - 结构：`[{ slotIndex, affixId, rarityId, rollValue? }]`
  - 用途：仅在“新实例生成”时注入默认 affix，不回填历史库存实例。
  - 约束：`slotIndex` 命中 `affixSlotCount`，`affixId` 命中启用中的 `affix_definitions`，`rarityId` 命中 `rarity_definitions` 且满足该词条稀有度限制，并且 `affix.type` 必须与装备 `equipType` 一致。
- 词条战斗钩子（`affix_definitions.effectType=battle_trigger`）：
  - `effectParams.battleHooks` 复用 `equipment_templates.battleHooks` DSL。
  - 运行时会把“装备模板 hook + 实例 affix hook”合并到同一触发链路执行。
- `basicAttackFormula`: `{ damageType, formula }`（可选）
- `formula` 结构：`[{ attrKey, coefficient }]`（至少 1 项，`coefficient` 为有限数，可为负；仅在填写 `basicAttackFormula` 时校验）
  - `basicAttackFormula` 示例：
    - 力量(0.6)+敏捷(0.4)造成物理伤害：`{"damageType":"physical","formula":[{"attrKey":"strength","coefficient":0.6},{"attrKey":"dexterity","coefficient":0.4}]}`
    - 智力(1.2)造成魔法伤害：`{"damageType":"magical","formula":[{"attrKey":"intelligence","coefficient":1.2}]}`
  - 若上述字段为空，构建阶段不会自动补默认值，前端装备详情也不会渲染对应普攻信息区块。
- 技能/武器 effect 的寻敌规则：
  - `effects[].targeting.selectRule`: `hp_max|hp_min|hp_percent_max|hp_percent_min|first|boss_first|attr_max|attr_min|random`
  - `targetSeekStrategy`: `rear_to_front|front_to_rear|random_alive`（默认 `front_to_rear`，仅 legacy 兼容）
- 技能战斗字段（`skills`）：
  - `damageType`: `physical|magical`
  - `targetDirection`: `vanguard|midguard|rearguard`
  - `targetMode`: `single|multi|all`
  - `targetBaseCount`: 目标基础数量（默认 `1`）
  - `targetFormula`: 目标数量附加公式，结构 `[{attrKey,coefficient,perSkillLevel}]`
    - 最终目标数：`max(1, targetBaseCount + floor(Σ(actor.stats[attrKey] * (coefficient + perSkillLevel*(skillLevel-1)))))`
  - `targeting`：通用目标模板，字段为 `targetPositions/secondarySeek/targetScope/selectRule/selectAttrKey/targetCount/excludeSelf`
  - `effects`：效果数组，按顺序执行，支持 `damage/heal/resource/status/summon/corpse_explosion/adjust_skill_cooldown`
  - `effects[].amountFormula`：`{ baseAdd, terms:[{attrKey,coefficient,perSkillLevel}] }`
  - `effects[].statusPayload`：`{ statusIds, durationBaseTurns, durationPerSkillLevelTurns, durationFormula, applyChanceBase?, applyChancePerSkillLevel?, applyChanceFormula?, applyChanceMin?, applyChanceMax? }`；未配置概率字段时必定施加。
  - `effects[].requiredCasterStatuses/requiredTargetStatuses`：全部具备时执行；`forbiddenCasterStatuses/forbiddenTargetStatuses`：任一存在时跳过。四组状态条件会随完整 DSL 写入战斗快照。
  - `effects[].damageSpec.damageType` 是伤害效果最终伤害类型；外层 `effects[].damageType` 仅作 legacy 回退，不建议再写。
  - `effects[].damageSpec` 穿透字段支持通用与按伤害类型区分两套：
    - 通用：`flatDefensePenetrationFormula` / `percentDefensePenetrationFormula`
    - 物理专用：`physicalFlatDefensePenetrationFormula` / `physicalPercentDefensePenetrationFormula`
    - 魔法专用：`magicalFlatDefensePenetrationFormula` / `magicalPercentDefensePenetrationFormula`
    - 运行时结算顺序：先百分比穿透，再固定穿透；若固定穿透超出剩余防御，超出部分会转为固定伤害并在暴击前并入基础伤害，之后继续参与 `damage_amplification` / `damage_resist` 乘区。
  - `effects[].damageSpec.bonusAgainstTargetTypes`：目标类型增伤映射（如 `{"boss":0.2}`）
  - `targeting.fallbackSelfIfEmpty`：目标为空时回退到施法者
  - `targeting.excludeSelf`：从候选目标中排除施法者，可表达“不包含自己”的全体友军或全体单位。
  - `targeting.targetPredicates` 的状态判断支持 `require_target_has_status` 与 `exclude_target_has_status`；先过滤候选池，再执行随机/排序和目标数量上限。
  - `adjust_skill_cooldown`：`amount` 为非零整数，正数延长、负数缩短；`includeBasic=false` 默认排除普攻，可通过技能白名单、排除列表或 `skillCategories` 进一步收窄，`selectionMode=max_remaining` 可配合 `maxSkills` 只调整剩余冷却最高的若干技能。
  - `statusRequirement`: 属性门槛对象（如 `{"intelligence":21}`）
  - `weaponRequirement`: 武器类别数组（如 `["bow","crossbow"]`）
- 装备武器分类字段（`equipment_templates`）：
  - `weaponClass`: 武器类别数组，推荐通过逗号填写并在构建阶段标准化
- 装备二级分类字段（`equipment_templates`）：
  - `equipmentSecondaryClass`: 非武器装备二级分类（字符串）
  - 武器仍以 `weaponClass` 作为二级分组来源；非武器必须配置 `equipmentSecondaryClass`
  - 分类定义源由 `combat-model-vue3/src/model/equipment-secondary-classes.json` 维护，`build.mjs` 在构建阶段做合法性校验并同步 `equipment_secondary_class.*` 的 i18n 基线
- 武器类别表（`weapon_classes`）：
  - 字段：`id/nameKey/descKey/icon/color/order`
  - 用途：统一技能武器限制枚举来源
- 装备强化字段在 `equipment_templates` 中维护：
  - `enhanceMaxLevel`：`0~1000` 整数，`0` 表示不可强化
  - `enhanceDurationS`：正整数（秒）
  - `enhanceExp`：每次强化发放到 `enhance` 的属性经验（`>=0`，成功/失败都发放）
  - `enhanceProtectItemId`：字符串数组（Excel 中可填英文逗号分隔，构建后标准化为数组）
  - `enhanceCostCurve`：`{base, step, overrides}`，值必须是有限数且最终为非负整数
  - `enhanceAttrCurve`：`{base, step, overrides}`，值必须是有限数
  - `enhanceAttrMilestoneOverrides`：Excel 独立里程碑覆盖列（`{level -> attrMap}`），构建时合并进 `enhanceAttrCurve.overrides`
  - `enhanceAffixMilestonePools`：Excel 强化附魔池列（`{level -> [{affixId, rarityId, rollValue?}]}`），强化成功命中该等级时会随机挑一个候选；单元素数组等价固定词条；如果没有空词条槽位，则按强化取消语义直接取消本次强化。
    - 领取约束：同一个装备实例（instance）同一个里程碑等级只会触发一次，后续上下强化回到该等级不会重复发放。
- 强化曲线统一规则：
  - 尝试等级索引：`nextLevel = currentLevel + 1`
  - `enhanceCostCurve`：`value = base + step * (level - 1)`，`overrides[level]` 优先
  - `enhanceAttrCurve`：`level=1` 使用 `base`，`level>1` 使用 `step`，`overrides[level]` 作为该级增量覆盖；总加成为各级增量累加
  - `enhanceAttrMilestoneOverrides` 与 `enhanceAttrCurve.overrides` 同等级冲突时，前者优先（仅影响构建结果，不改变运行时语义）
  - 强化基础成功率由 `rarity + enhanceMaxLevel + level` 公式统一计算（见 `docs/enhancement_success_rate_design.md`），再叠加玩家 `enhancement_success_rate`，并保留 4 位小数
  - `overrides` 的 key 必须是合法正整数且 `<= enhanceMaxLevel`
- 若维护了 `item_types`，则 `items.type` 必须出现在 `item_types.id` 中
- 构建阶段会基于 `skills` 自动生成技能书物品：
  - 自动生成 id：`skillbook_<skillId>`（排除 `basic_attack`）
  - 自动补：`useGrantSkillId=<skillId>`、`useSkillExpGain=100`
  - 若手工配置了同 id 物品，按“手工优先”保留手工值，仅补缺失关键字段
  - 构建会自动确保 `item_types.skillbook` 存在（未配置时注入默认条目）

## item_types 字段约定（新增）
- 用途：集中维护物品类型元数据，避免 `items.type` 散落定义
- 建议字段：
  - `id`: 类型 id（与 `items.type` 对应）
  - `nameKey`: 类型名称 i18n key
  - `descKey`: 类型描述 i18n key（可选）

## Action 字段约定（更新）
- 每条 action 必须携带 `mainType`，用于动态主菜单分组与面板筛选。
- 每条 action 必须携带 `subType`；未填写时构建阶段自动补为 `common`。
- `nameKey` 可留空，构建时按 `action.<id>.name` 自动生成。
- `iconId` 可留空，构建时按 `action.<id>` 自动生成（前端可直接按 TypeIcon 解析并显示）。
- `rewardDestination` 已移除（奖励统一进仓库模型）
- 奖励拆分为：
  - `itemReward`：物品奖励（支持固定数量、范围数量、概率）
  - `statusExpReward`：属性经验奖励
- `itemReward` 支持两种写法：
  - 简写：`"stone": 2`（等价于 `count=2, percent=1`）
  - 详细：`"wood": { "count": 2 }` 或 `{"range":{"min":1,"max":3},"percent":0.44}`
- `action_main_types` 用于声明主类元数据（`id/nameKey/icon/color/order`），若缺失会按 action 中出现的 `mainType` 自动推导。
- `action_sub_types` 用于声明子类元数据（`id/nameKey/icon/color/order`），若缺失会按 action 中出现的 `subType` 自动推导。
- `world` 表新增字段约定：
  - `normalEnemyCount` 支持 number 或 `{min,max}` 区间对象（构建后统一归一为区间对象）。
  - `extraEnemyPerPlayer` 使用 `{min,max}` 区间对象，表示每多 1 个玩家单位额外生成的敌人数区间。
  - `customLineupConf` 为阵容数组，`nameKey/descKey` 为空时自动生成 `world.<worldId>.customLineup.<id>.name|desc`。
  - `customLineupConf.drops.items` 使用 `{min,max,percent}` 结构；后端在该 lineup 击败时额外执行一次掉落检查。

## 战斗配置相关共享表（新增）
- `skills`
  - 用途：定义可入槽技能的基础目录。
  - 当前核心字段：`id`、`nameKey`、`descKey`、`category`、`hpCost`、`mpCost`、`cooldown`（回合数）、`maxLevel`（默认 200）。
  - 约束：`id` 必须全局唯一，供 `player_skills.skill_id` 引用。
  - 默认 key：`nameKey=skill.{id}.name`、`descKey=skill.{id}.desc`（未填写时自动生成）。
- `skill_type`
  - 用途：定义技能类型元数据（名称/描述/颜色）。
  - 当前核心字段：`id`、`nameKey`、`descKey`、`color`。
  - 默认 key：`nameKey=skill_type.{id}.name`、`descKey=skill_type.{id}.desc`（未填写时自动生成）。
  - 约束：`color` 必须是 HEX；`skills.category` 必须命中 `skill_type.id`。
- `combat_config_rules`
  - 用途：统一前后端槽位计算规则（如基础槽位、智力换算、最小/最大槽位）。
  - 当前约定字段：`base_skill_slots`、`int_per_extra_slot`、`min_skill_slots`、`max_skill_slots`、`default_basic_skill_id`（可选）。
  - 规则：前后端都必须读取该表结果，不允许在 Web/Server 各自硬编码一套常量。
- `buff`
  - 用途：定义状态（buff/debuff）基础元数据。
  - 当前核心字段：`id`、`nameKey`、`descKey`、`durationTurns`、`permanent`、`kind`（buff/debuff）、`dispellable`、`type`。
  - 默认 key：`nameKey=buff.{id}.name`、`descKey=buff.{id}.desc`（未填写时自动生成）。
  - 约束：`kind` 必须是 `buff|debuff`；`type` 必须命中 `buff_type.id`（可留空）。
- `buff_type`
  - 用途：定义状态类型元数据（名称/描述/颜色）。
  - 当前核心字段：`id`、`nameKey`、`descKey`、`color`。
  - 默认 key：`nameKey=buff_type.{id}.name`、`descKey=buff_type.{id}.desc`（未填写时自动生成）。
  - 约束：`color` 必须是 HEX。
- `buff_effects`
  - 用途：定义状态的触发规则与效果（DSL）。
  - 当前核心字段：`id`、`buffId`、`trigger`、`priority`、`conditions`、`effects`。
  - 约束：`buffId` 必须命中 `buff.id`；`conditions/effects` 必须是合法 JSON（`effects` 至少一项）。
  - `turn_ineligible` 表示单位不进入准备、正常或完整行动队列，也不能获得额外回合；它与仍可被同步行动或额外回合调用的 `skip_turn_order` 不同。
  - 目标重映射效果支持 `camouflage_targeting_remap`（用于“伪装”索敌语义）。
- `arena_formats`
  - 用途：定义猫村联赛当前开放的赛制。`standard` 也是普通数据条目；其他赛制通过 `baseFormatId` 继承标准赛，因此标准生命补偿会与附加赛场效果一同生效。`enabled=true` 的条目会出现在主场选择器和对手筛选页，同时启用多条即可并行开放多个赛制。
  - 身份字段：`id` 为稳定数据 ID；`schemaVersion=1`、`rulesetVersion=arena-format-v1`；只调整数值时递增 `balanceRevision`。当前功能尚未发布，不保留空赛制、缺失赛制 ID 或旧字段的兼容分支。
  - 触发器：当前支持 `lineup_snapshot`、`battle_start` 与 `after_summon`。效果支持 `specialization_policy`（冻结阵容时决定双方玩家是否载入专精）、`level_scaled_buff`（按战斗等级把数值传给既有 `arena_fortification` Buff DSL）、`turn_barrier`（按行动回合消耗层数并保护该回合）和 `damage`（可按目标最大生命造成全场伤害）。构建器会拒绝未知字段、悬挂 buff、越界数值、不继承标准赛的条目和不匹配的事件/效果组合。
  - 权威链路：主场发布时，服务端从共享目录解析并冻结完整定义及 `definitionDigest`，不信任客户端上传的效果内容；发起挑战时再复制到 `arena_challenges.format_snapshot_jsonb`。`battle-worker` 和录像只读取这份快照，因此热更新不会改写进行中战斗或历史录像。
  - 数值入口：在 `packages/game-data/source/json/arena_formats.json` 调整 `levelScale`、`charges` 或 `amount`，再执行 `node packages/game-data/scripts/build.mjs`。新增同类赛制只需要编辑该数据源与对应文案，不需要数据库迁移；新增事件或效果 kind 时才需要同步扩展构建校验、Go 运行时、玩家可读翻译和回放测试。
- `combat_trial_metrics` / `combat_trial_formats`
  - 用途：分别定义猫猫试炼的计分指标与完整项目。首个开放条目是 `standard`，其他条目可通过 `baseFormatId` 继承它的限制、遭遇、结束条件和触发器；多条 `enabled=true` 项目会直接出现在试炼项目选择器中。
  - 指标 DSL：事件支持 `damage`、`heal`、`battle_end`，聚合支持 `sum`、`max`、`final`；`sourceScope` / `targetScope` 决定统计范围，`rankDirection` 决定升序或降序，`eligibleEndReasons` 决定哪些结局可以写入排行榜。`battle_end + final + player_main_actions` 可直接实现“击败目标所用行动数”。
  - 遭遇 DSL：`training_target` 生成不攻击且生命恒满的木桩，其 `damageResist` 与 `criticalDamageResist` 直接进入冻结战斗属性；`authored_enemy` 通过 `enemyId` 引用并冻结 `enemies` 目录中的完整敌人快照，可正常行动和死亡。遭遇或波次上的 `enemyStatProfile` 先按难度倍率与固定加值缩放敌人，单位上的 `statOverrides` 再覆盖需要独立校准的最终战斗属性，适合为试炼 Boss 明确编写生命、攻防、速度和抗性。`objectiveTarget` 决定指标所称的试炼目标。
  - 流程 DSL：`lineupRules.positionPolicy` 支持允许同排的 `shared` 与要求不同排的 `distinct`；`limits` 当前支持玩家侧主行动预算；`endConditions` 按声明顺序判断队伍倒下、敌方全灭和预算耗尽，并通过稳定 `reason` 与玩家文案 `labelKey` 输出结局。`triggers` 当前支持按物理、魔法或全部伤害类型配置玩家侧最终伤害倍率，可选择是否包含召唤物。
  - 权威链路：开战时服务端解析继承、冻结 `DefenseK` 与引用敌人、生成 `definitionDigest`，再把完整项目写进运行快照。worker、入榜资格、历史摘要和录像只读取该快照；数据热更新会自动形成新榜，不会改写排队战斗或旧录像。
  - 扩展入口：在 `packages/game-data/source/json/combat_trial_metrics.json` 新增指标，在 `combat_trial_formats.json` 新增项目并补齐六种语言文案，然后运行 `node packages/game-data/scripts/build.mjs`。组合既有事件、遭遇、条件和效果不需要数据库迁移；新增 DSL kind 时才需要同时扩展构建校验、Go 解释器、前端公共类型与回放测试。

## 图标与 Excel 配置建议
- 在 `items/actions` 表里只放 `iconId`，不要直接放文件路径。
- 在 `icons` 表维护 `id -> path`，例如：
  - `item.wood_sword -> /assets/icons/items/wood_sword.png`
- 图标素材目录建议：
  - `web/public/assets/icons/items/`
  - `web/public/assets/icons/actions/`
  - `web/public/assets/icons/common/`（跨类型复用兜底）
- 文件命名规则统一为：`<id>.png`（例如 `wood_sword.png`），避免同一资源多命名探测。
- 优点：
  1. 图标资源迁移时只改一张表。
  2. 同图标可复用给多个 item/action。
  3. 请求路径更稳定，减少无效 404 探测。

## 版本策略
- `source/version.json` 维护 `version`
- 发布时服务端与客户端应校验 `data_version` 一致

## shared-global-config（共享运行时配置）
- 文件：`packages/game-config/source/shared-global-config.json`
- `auction` 字段约定（前后端共享）：
  - `feeRateBps`：拍卖成交手续费（基点，`500` 代表 `5%`）
  - `minBidIncrement`：保留兼容字段（历史默认最小加价值）；新拍卖创建改为请求内按 `startPrice` 动态输入并做区间校验。
  - `antiSnipeTriggerSeconds`：触发反狙击窗口（秒）
  - `antiSnipeExtendSeconds`：触发后每次延长时长（秒，可重复叠加）
- `kitty` 字段约定（前后端共享）：
  - `actionDurationMultiplier`：猫咪执行生活动作的基础耗时倍率。
  - `statusExpMultiplier`：猫咪执行生活动作时的状态经验倍率。
  - `battleAttrExpMultiplier`：猫咪战斗属性经验倍率（建议 `0..1`，默认 `1` 表示全额）。
  - `battleSkillExpMultiplier`：猫咪战斗技能经验倍率（建议 `0..1`，默认 `1` 表示全额）。
  - `battleEquipSlots`：猫咪可用战斗装备槽位白名单。
  - `removePersonalityItemId`：玩家移除猫咪性格时消耗的道具 ID。
  - `removePersonalityCount`：玩家移除猫咪性格时消耗数量。
  - `defaultPersonalityId`：猫咪默认性格 ID。
- `kitty_personalities` 表（`packages/game-data/source/json/kitty_personalities.json`）：
  - 性格 DSL 配置；`effects.kind` 目前支持：
    - `attr_modifier`
    - `battle_position_lock`
    - `battle_hook`（复用现有 `battleHooks`）
    - `effect_mutation`（复用现有 `effectMutations`）

## game-data-tool（source/json 并行链路）

### skill_alchemy_modules

- 所有启用的 `loot` 战利品与 `mythic` 生活产出必须各自对应且只对应一个启用模块；同一物品不能同时修正多个 DSL 节点。
- 目标站位至少使用 `legendary` 素材，命中次数只允许 `mythic` 素材；公式素材可使用较低品质，但构建器按品质限制每批最大贡献。`select_rule=attr_max|attr_min` 必须配置合法的 `attrKey`。
- 启用模块按品质固定数量步长：`common/uncommon/rare=1000`、`epic=400`、`legendary=200`、`mythic=50`；不会累加的目标、索敌和条件选项按该品质步长的 5 倍一次性消耗。单种素材的最大投入由 `unitBasis × maxBasisMultiplier` 明确封顶。
- 技能炼成编译不应用炼金等级、精度或稳定度修正。技能书数量提供基础加成；每批素材按模块表中的 `successChanceModifier` 独立增减成功率，服务端按实际批数汇总并截断在 `0%-100%`。
- `nodeKind=structure + compileOp.kind=effect_slot` 表示额外效果槽素材；素材数量直接决定可增加的效果数，最终仍受全局 `skillForge.maxEffects` 限制。
- 结果技能稀有度按实际有效效果数计算：1 个为 rare、2 个为 epic、3 个为 legendary；只有解锁但未配置的空效果不计入稀有度。
- `formula_term.effectKind=heal` 只允许 `attrKey=intelligence|perception`，`skill_level_term` 单独提供治疗的技能等级系数。构建器会阻断越权组合。
- `self_current_hp_loss` 生成指向施法者自身的 HP 资源扣减；数值为当前生命值比例，不能由客户端直接提交系数。

- 新增目录：`packages/game-data/source/json/`，用于 `game-data-tool` 维护结构化数据。
- 文件命名必须与表名一致，例如：`items.json`、`actions.json`、`equipment_templates.json`、`enemies.json`。
- 文件结构统一：
  - 根对象包含 `version`（number）与同名表字段（array）。
  - 示例：`{ "version": 1, "items": [ ... ] }`。
- `build.mjs` 合并顺序：
  1. 读取 Excel / 旧 fallback；
  2. 读取 `source/json/*.json`；
  3. 合并后进入现有 normalize + validate；
  4. 输出 generated 与 i18n 同步。
- 冲突策略：
  - 同表同 `id` 若同时存在于两侧来源，构建直接失败；
  - 错误信息包含 `table + id + 来源`，禁止静默覆盖。
