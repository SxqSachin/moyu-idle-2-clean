# 库存实例、装备与属性设计

## 数据结构
- `player_item_instances`：唯一实例装备（不同词条/强化差异）。
- `player_attributes`：玩家属性主表（按 `attr_key` 存储，包含 `base_level/attr_exp/source_mods_jsonb/final_value/lock_version`）。
- `player_equip_slots`：角色装备槽位到实例映射（按 `equip_type=battle/work` 分组；支持同一实例占用多个槽位，如双手武器）。
- `player_derived_stats_cache`：最终属性缓存与版本号。
- `packages/game-config/source/shared-global-config.json.attributeLayout`：属性布局配置入口，按场景（`character_stats` / `battle_unit_card` / `enemy_quick_ref`）维护 `battlePrimary` / `battle` / `work` 的展示顺序与摘要条数。
- `equipment_templates.affixSlotCount`：装备可拥有的词条槽位上限。
- `rarity_definitions`：全局稀有度定义，item 与 affix 共用。
- `affix_definitions`：词条定义、效果类型、参数 schema、稀有度开放范围；`valuePrecision` 只控制词条数值展示精度。
- `affix_detach_rules`：词条拆卸规则与道具约束。
- `affix_enchant_rules`：词条附魔规则与材料/资源消耗配置；`toolItemId + consumeCount` 仍是触发道具，`costs` 负责额外资源。
- `GET /api/player/stats`：返回动态属性 map、`attributeProgress` 与 `formula(version=v2)`，供前端属性页展示与本地预估计算。
- `player_action_queue`：多动作执行队列（支持次数与串行状态流转）。
- `player_item_instances`：统一库存与实例数据（`state='inventory'` 即仓库内容）。

## 一致性策略
- 装备穿脱使用事务，实例状态与槽位状态同时更新。
- 装备校验由 `equipment_templates` 驱动：`equipType + slot` 控制可装备槽位，`occupySlots` 支持多槽占用。
- 饰品槽位为通用 4 槽（`accessory_1~4`），同模板饰品同时只能装备一个实例。
- 市场实例上架将状态置为 `market`，成交后所有权迁移并恢复 `inventory`。
- 最终属性重算采用“事件触发 + 缓存更新”模式。
- 词条拆卸/装配在单事务内完成：校验所有权、扣道具、写回装备槽位与词条实例。
- 词条附魔在单事务内完成：校验空槽、扣材料、随机稀有度与词条、写回目标槽位。
- 仓库实例变更在单事务内完成：基于实例行锁与状态校验保证并发一致性。
- 洗点后会在服务端重新校验已装备物品需求；不满足需求的装备将自动脱下并同步推送装备/属性状态。
- 探索/战斗奖励直接入仓，并写入 `player_warehouse_change_logs` 审计流水。
- 仓库无布局坐标语义（无 x/y/lockVersion）；前端以 `state='inventory'` 的实例列表直接渲染。
- `GET /api/inventory/instances` 使用服务端 Redis 快照缓存（30s），写路径触发失效与回填。

## 属性布局配置
- 属性展示顺序不再写死在页面组件里，统一从 `shared-global-config.json.attributeLayout` 读取。
- `character_stats` 场景用于角色 -> 属性页，支持 `battlePrimary`（核心战斗属性）/ `battle`（战斗次要属性）/ `work` 分组。
- `battle_unit_card` 与 `enemy_quick_ref` 场景用于战斗单位卡与敌人快速详情，支持预览数量控制与同源排序。
- 布局字段支持两种写法：
  - 单数组：按常规 grid 自动换行；
  - 多数组：按列渲染（每个子数组对应一列，顺序即列顺序）。
- 数组中允许空字符串 `""`，表示占位空格（保留布局空间，不渲染属性卡）。
- 后续新增属性布局时，优先改配置，不要在组件里追加新的硬编码排序规则。

## 角色页装备槽位交互（角色 -> 装备槽位）
- 对齐 `deep-article/components/Game/Character/Equipment/Index.vue` 的结构：
  - 外层 `work` / `battle` 双 Tab。
  - 主装备区使用固定 `3 x 3` 槽位栅格（含占位空槽）。
  - 饰品区单独分区并以独立栅格展示。
- 槽位交互规则：
  - 点击任意可用槽位，在该槽位下方打开候选 dropdown（再次点击同槽位关闭）。
  - 候选仅包含 `state='inventory'` 且 `equipment_templates.equipType + slot` 命中当前页签槽位的装备实例。
  - 候选与通用装备选择器统一按“部位一级 + 二级分类”分组：武器二级分组取 `weaponClass`，非武器取 `equipmentSecondaryClass`。
- dropdown 按两栏分组展示：`可装备` / `不可装备`（不可装备项显示需求差异并禁用装备按钮）。
- 点击候选后立即调用 `POST /api/equipment/equip`（包含 `equipType`），不增加二次确认。
- 替换一致性由后端事务保障（冲突槽位清理、实例状态切换、属性重算）。
- 失败反馈统一走错误码映射（`resolveErrorMessage`），不做静默失败；`EQUIP_INSTANCE_STATE_INVALID` 会在 `error.instanceState` / 顶层 `instanceState` 返回当前实例状态，便于排障。
- 加点弹窗中每个可分配属性显示“当前值 + 问号提示”；问号浮层展示该属性“每1点影响”。
- `Card` 组件在传入 `instanceId` 且实例强化等级大于 0 时，右上角显示 `+N` 强化角标。
- 实例详情请求归口到统一本地 store（按 `instanceId` 缓存 + 并发去重），减少页面分散重复请求。

## API
- `GET /api/inventory/instances`
- `POST /api/equipment/equip`
- `POST /api/equipment/unequip`（请求体需包含 `equipType`）
- `POST /api/equipment/repair`
- `POST /api/equipment/affix/detach`
- `POST /api/equipment/affix/attach`
- `POST /api/equipment/affix/enchant`
- `GET /api/player/stats`
- `GET /api/warehouses/logs`
- `POST /api/battle/points/allocate_batch`（请求需带 `X-Idempotency-Key`）
- `POST /api/battle/points/reset`（请求需带 `X-Idempotency-Key`）

## 装备详情展示
- `ItemPopoverContent` 会把装备效果拆成基础属性与词条效果，并展示每个词条槽位的状态。
- 实例态展示优先直接使用 `baseSnapshot` 的聚合结果；词条明细只用于分项展示，不会再参与总属性二次累计，避免前端显示翻倍。
- `EquipmentStaticInfo` 会可视化渲染 `equipment_templates.battleHooks`（触发时机 + 效果文案，i18n）。
- 词条可通过 `effectType=battle_trigger` 承载战斗钩子；实例详情中的 affix 仍通过词条名/描述展示，实际触发由后端战斗运行时统一处理。
- `battleHooks.add_buff` 目标支持 `self/source/targeting`，当使用 `targeting` 时展示目标范围、排位、筛选与寻敌规则摘要。
- 当装备存在词条槽时，详情页底部提供提取、镶嵌与附魔入口；附魔仅对空槽位生效。
- 当装备实例在背包中且模板可强化（`enhanceMaxLevel > 0`）时，详情页操作条显示“强化”入口；点击会跳转 `/game/enhance` 并优先以该实例作为强化目标（通过 `instanceId` query + `equipmentStore.operationTarget` 预填）。
- 回收只允许“模板一致”库存实例：`enhanceLevel = 0` 且 affix 与模板默认槽位（`defaultAffixSlots`）在 `slotIndex + affixId` 维度一致；否则不显示回收入口且后端拒绝。
