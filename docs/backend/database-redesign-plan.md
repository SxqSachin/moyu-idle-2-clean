# 数据表重构方案（可清库重建）

## 1. 范围与约束
- 范围：后端业务库全量治理（初始 44 张表，当前清理后 37 张表）。
- 约束：
  - 允许破坏性重构（开发期可清库重建）。
  - 命名统一 `snake_case`。
  - 主键统一 `bigint` 策略，默认 `id`。
  - 审计字段统一 `created_at/updated_at`。

## 2. 重构目标
- 减少重复事实：库存堆叠、装备实例、交易账本等仅保留一份权威事实来源。
- 强化一致性：交易与结算链路全部事务化，幂等键强制化。
- 降低维护成本：按业务域分层聚合，清理历史试验表与未使用表。

## 3. 领域分组（目标模型）
- `identity_*`：用户、资料、认证相关。
- `player_*`：资源、成长、活动状态、规则配置。
- `inventory_*`：堆叠物品、实例化物品、装备位、仓库布局。
- `action_*`：动作队列、属性经验、执行时序。
- `market_*`：挂单、订单、账本、手续费、NPC 报价。
- `task_*`：日常模板、玩家任务、刷新状态。
- `combat_*`：战斗实体与事件。
- `system_*`：聊天、世界事件、配置快照等系统级数据。

## 4. 现有表治理策略
- 保留并规范：核心在线链路表（用户、资源、库存、市场、队列、任务、仓库）。
- 合并：语义重叠或可从事实表推导的缓存表（仅保留必要缓存并标注失效策略）。
- 下线：无读取入口或历史试验用途表（迁移中删除）。

## 5. 表级审计与分类（v1）
| 现有表 | 分类 | 目标动作 |
|---|---|---|
| `users` | 保留 | 迁移到 `identity_users`（统一审计字段） |
| `player_profiles` | 保留 | 迁移到 `player_profiles`（补齐 `updated_at`） |
| `player_resources` | 保留 | 迁移到 `player_resources`（统一资源字段命名） |
| `player_progression` | 保留 | 合并成长相关点数字段并保留 |
| `player_activity_state` | 保留 | 保留并强化活动状态枚举约束 |
| `idle_jobs` | 合并 | 并入 `player_routine_jobs`，统一作业模型 |
| `player_routine_jobs` | 保留 | 保留，强化状态机字段 |
| `player_routine_settlements` | 保留 | 保留，幂等键字段标准化 |
| `player_items` | 保留 | 保留，作为堆叠物品事实表 |
| `player_item_instances` | 保留 | 保留，作为实例化物品事实表 |
| `player_equip_slots` | 保留 | 保留，补齐审计字段与索引命名 |
| `player_base_stats` | 保留 | 保留，字段统一到属性命名规范 |
| `player_attr_modifiers` | 下线 | 已移除，动态属性修饰器明细不再独立建表 |
| `player_derived_stats_cache` | 合并 | 转为缓存层策略，不作为业务事实表 |
| `player_warehouses` | 下线 | 已移除，仓库容器改由 `player_item_instances(state='inventory')` 表达 |
| `player_warehouse_items` | 下线 | 已移除，仓库映射关系不再独立建表 |
| `player_action_queue` | 保留 | 保留，状态与时间列统一 |
| `player_attribute_exp` | 保留 | 保留，属性键统一映射 |
| `player_queues` | 下线 | 历史扩展表，迁移到统一 `action/routine` 模型后移除 |
| `market_listings` | 保留 | 保留，挂单类型枚举与状态约束收紧 |
| `market_orders` | 保留 | 保留，幂等唯一约束保留 |
| `market_ledger` | 保留 | 保留，增加方向与业务类型字段 |
| `market_fee_tiers` | 保留 | 保留，改为配置域标准表 |
| `market_npc_offers` | 保留 | 保留，增加有效期字段 |
| `daily_task_templates` | 保留 | 保留，模板配置统一来源（后续可对接 game-data） |
| `player_daily_task_state` | 保留 | 保留，刷新时间策略不变 |
| `player_daily_tasks` | 保留 | 保留，状态与完成时间索引优化 |
| `equipment_enhance_profiles` | 下线 | 已移除，升级为新 affix 配置体系（game-data） |
| `equipment_mutation_profiles` | 下线 | 已移除，升级为新 affix 配置体系（game-data） |
| `enchant_book_profiles` | 下线 | 已移除，升级为新 affix 配置体系（game-data） |
| `player_rule_sets` | 保留 | 保留，规则分组格式标准化 |
| `player_combat_configs` | 保留 | 保留，配置字段标准化 |
| `combat_entities` | 保留 | 保留，转入战斗域命名规范 |
| `combat_events` | 保留 | 保留，按 `battle_id + trigger_at` 优化索引 |
| `search_grids` | 下线 | 设计期实验表，未形成稳定读写路径 |
| `chat_messages` | 保留 | 保留，后续可冷热分层 |
| `world_events` | 保留 | 保留，系统事件存档表 |
| `game_data_items` | 下线 | 与 `packages/game-data` 功能重复，迁移后移除 |

## 6. 迁移批次（执行顺序）
1. **批次 1：基线清库与核心域重建**
   - 身份/玩家/库存基础表 + 最小索引 + FK。
2. **批次 2：动作、市场、任务链路重建**
   - 幂等、事务、状态机约束一次性落地。
3. **批次 3：成长系统与扩展域重建**
   - 装备成长、仓库细节规则。
4. **批次 4：清理与收敛**
   - 删除旧表、统一索引命名、完善文档与门禁。

## 7. 风险控制
- 所有重构批次必须同步更新 `docs/backend/database-schema.md`。
- 关键链路（市场/队列/任务）每批次都做并发回归。
- 若发现模型歧义，先冻结接口再继续迁移，避免前后端契约漂移。

## 8. 已执行进展（当前轮）
- 已切换为基线初始化：`server/schema/init.sql`
- 已清理未使用表：`player_queues`、`search_grids`、`game_data_items`、`world_events`、`combat_events`、`combat_entities`、`player_profiles`
- 已清理旧装备成长表：`equipment_enhance_profiles`、`equipment_mutation_profiles`、`enchant_book_profiles`
- 已同步更新：`docs/backend/database-schema.md`（表总数、分组、清理记录）
