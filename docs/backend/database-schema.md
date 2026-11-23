# 后端数据表手册（Database Schema）

## TL;DR
- 真值来源：`server/schema/init.sql`，本文是可读版索引。
- 改表时必须同时更新 `init.sql` 和本文。
- `check-schema-doc-sync.sh` 只能检查“是否一起改过”，不能保证内容完整，新增表时要手工补本文。

## 何时阅读本文
- 新增或修改表结构、索引、字段约束。
- 排查接口写入了哪些表、某张表的业务语义是什么。
- 评估迁移、清理、回滚风险。

## 当前表清单（按领域）
- 账号与认证：`users`、`player_username_changes`、`password_reset_tokens`、`user_settings`、`user_policy_acceptances`、`player_presence`
- 物品与成长：`player_item_instances`、`player_equip_slots`、`player_loadouts`、`player_work_loadout_bindings`、`player_derived_stats_cache`、`player_attributes`、`player_battle_point_state`
- 市场与商店：`market_listings`、`market_orders`、`market_ledger`、`market_fee_tiers`、`shop_offers`、`market_buy_orders`、`market_auctions`、`market_auction_bids`
- 聊天与任务：`chat_messages`、`daily_task_templates`、`player_daily_task_state`、`player_daily_tasks`
- 战斗与规则：`player_rule_sets`、`player_skills`、`player_battle_item_loadout`、`player_specialization_states`、`player_specialization_allocations`、`player_specialization_ledger`、`battle_rooms`、`battle_room_members`、`battle_runs`、`battle_run_payloads`、`battle_settlement_jobs`、`battle_skill_exp_jobs`、`battle_attr_exp_jobs`、`arena_player_states`、`arena_defenses`、`arena_challenges`、`arena_challenge_payloads`、`arena_replay_shares`、`arena_shop_orders`
- 建筑与猫舍：`player_buildings`、`player_kitties`、`player_kitty_personalities`、`player_kitty_battle_point_state`、`player_kitty_attributes`、`player_kitty_stats_cache`、`player_kitty_skills`、`player_kitty_equip_slots`、`battle_room_member_units`、`player_kitty_commissions`、`player_kitty_encounters`、`player_kitty_journals`
- 钓鱼与鱼塘：`fishing_ponds`、`fishing_pond_pairs`、`fishing_pond_events`；塘内鱼仍由 `player_item_instances` 持有
- 动作队列与审计：`player_action_queue`、`player_warehouse_change_logs`
- 成就：`achievement_events`、`player_achievement_states`

## 表定义与字段摘要

### 账号与认证

#### `users`
作用：账号主表。

关键字段：
- `username`：唯一登录名。
- `email` / `email_verified` / `email_verified_at`：邮箱与验证状态。
- `ban_reason` / `ban_until` / `ban_is_permanent`：封号状态。

#### `player_username_changes`
作用：保存玩家每次改名前后的用户名、操作者来源，以及当次实际消耗的物品快照。现有账号不回填伪造历史；首次改名时才记录旧名。

关键字段：
- `old_username` / `new_username`：当次改名前后的用户名。
- `changed_by_user_id` / `change_source`：操作者与来源；自助改名时操作者就是玩家本人。
- `cost_item_id` / `cost_item_count`：当次实际扣除的配置物品与数量，不随后续配置变化。
- `created_at`：改名完成时间。

关键索引：
- `idx_player_username_changes_user_created`：按玩家倒序读取最近改名记录。

#### `password_reset_tokens`
作用：一次性密码重置令牌。

关键字段：
- `token_hash`：唯一哈希值。
- `expires_at` / `used_at`：过期和消耗状态。

#### `user_settings`
作用：用户偏好设置。

关键字段：
- `language`
- `theme`
- `extra_jsonb`

#### `user_policy_acceptances`
作用：条款同意记录。

关键字段：
- `policy_key`
- `policy_version`
- `source`
- `accepted_at`

关键约束：
- `UNIQUE(user_id, policy_key, policy_version)`

#### `player_presence`
作用：在线状态与会话版本。

关键字段：
- `status`
- `last_seen_at`
- `session_version`

### 物品与成长

#### `player_item_instances`
作用：统一库存实例和堆叠物品。

关键字段：
- `template_item_id`
- `enhance_level`
- `base_snapshot`
- `affixes`
- `milestone_affix_claims`
- `durability` / `max_durability`
- `quantity`
- `state`

关键索引：
- `idx_player_item_instances_user_state`

#### `player_equip_slots`
作用：玩家装备槽位。

关键字段：
- `equip_type`：`battle|work`
- `slot`
- `instance_id`

关键约束/索引：
- `PRIMARY KEY(user_id, equip_type, slot)`
- `idx_player_equip_slots_user_instance`
- `idx_player_equip_slots_user_type_instance`
- `uq_player_equip_slots_battle_instance`（`battle` 实例唯一）

#### `player_loadouts`
作用：玩家预设。

关键字段：
- `loadout_id`
- `type`：`battle|work`
- `name`
- `payload_jsonb`

关键约束/索引：
- `UNIQUE(user_id, loadout_id)`
- `idx_player_loadouts_user_type_updated`

#### `player_work_loadout_bindings`
作用：劳作主类到 work loadout 的绑定。

关键字段：
- `action_main_type`
- `loadout_id`

关键约束/索引：
- `PRIMARY KEY(user_id, action_main_type)`
- `idx_player_work_loadout_bindings_user_loadout`

#### `player_derived_stats_cache`
作用：派生属性缓存。

关键字段：
- `stats_jsonb`
- `version`

#### `player_attributes`
作用：玩家动态属性明细。

关键字段：
- `attr_key`
- `base_level`
- `skill_point_level`
- `attr_exp`：`DOUBLE PRECISION`
- `source_mods_jsonb`
- `final_value`
- `lock_version`

关键约束/索引：
- `PRIMARY KEY(user_id, attr_key)`
- `idx_player_attributes_user`

#### `player_battle_point_state`
作用：玩家战斗加点状态。

关键字段：
- `total_points`
- `spent_points`

### 市场与商店

#### `market_listings`
作用：市场挂单。

关键字段：
- `item_id`
- `listing_kind`
- `instance_id`
- `price`
- `quantity`
- `status`

关键约束/索引：
- `quantity >= 0`
- `idx_market_listings_status`
- `uq_market_active_instance_listing`

#### `market_orders`
作用：购买成交单。

关键字段：
- `listing_id`
- `buyer_id`
- `total_price`
- `fee_rate_bps`
- `fee_amount`
- `seller_amount`
- `idempotency_key`

关键约束/索引：
- `uq_market_orders_buyer_idempotency`

#### `market_ledger`
作用：市场资金流水。

关键字段：
- `order_id`
- `from_user`
- `to_user`
- `amount`

#### `market_fee_tiers`
作用：手续费梯度配置。

关键字段：
- `scope`
- `min_total`
- `max_total`
- `fee_bps`
- `priority`
- `active`

关键索引：
- `idx_market_fee_tiers_active`

#### `shop_offers`
作用：系统商店报价。

关键字段：
- `item_id`
- `shop_buy_price`
- `shop_sell_price`
- `active`

#### `market_buy_orders`
作用：求购单。

关键字段：
- `buyer_id`
- `item_id`
- `price`
- `quantity`
- `remaining_quantity`
- `status`

关键索引：
- `idx_market_buy_orders_status`
- `idx_market_buy_orders_item`

#### `market_auctions`
作用：拍卖挂单。

关键字段：
- `seller_id`
- `item_id`
- `instance_id`
- `listing_kind`
- `quantity`
- `start_price`
- `current_price`
- `current_bidder_id`
- `buyout_price`
- `status`
- `ends_at`

关键约束/索引：
- `idx_market_auctions_status`
- `idx_market_auctions_item`
- `idx_market_auctions_ends_at`
- `uq_market_active_instance_auction`

语义：
- `listing_kind='instance'` 时 `quantity=1`，活跃拍卖必须绑定实例。
- 终态拍卖会把 `instance_id` 置空，避免活跃唯一约束残留。

#### `market_auction_bids`
作用：拍卖出价记录。

关键字段：
- `auction_id`
- `bidder_id`
- `bid_amount`

关键索引：
- `idx_market_auction_bids_auction`

### 聊天与任务

#### `chat_messages`
作用：聊天落库。

关键字段：
- `user_id`
- `channel`
- `content`
- `attachment_jsonb`：结构化聊天附件；当前支持随最近 50 场比赛记录滚动保留的猫村联赛录像分享，不依赖正文文本。
- `sent_at`

#### `daily_task_templates`
作用：日常任务模板池。

关键字段：
- `required_item_id`
- `required_quantity`
- `reward_items`
- `weight`
- `active`

#### `player_daily_task_state`
作用：玩家日常刷新游标。

关键字段：
- `next_refresh_at`
- `updated_at`

#### `player_daily_tasks`
作用：玩家当前持有的任务实例。

关键字段：
- `template_id`
- `required_item_id`
- `required_quantity`
- `reward_items`
- `status`
- `completed_at`

关键索引：
- `idx_player_daily_tasks_user_status`

### 战斗与规则

#### `player_rule_sets`
作用：规则组配置。

关键字段：
- `rule_kind`
- `slot_key`
- `enabled`
- `groups_jsonb`

关键约束：
- `UNIQUE(user_id, rule_kind, slot_key)`

#### `player_skills`
作用：玩家技能配置与成长。

关键字段：
- `skill_id`
- `enabled`
- `cast_order`
- `rule_enabled`
- `rule_groups_jsonb`
- `equipment_config_jsonb`：仅装备授技行使用，保存版本、修订号、有序技能引用、启用状态、独立施放偏好与组合施放规则；随现有技能查询读取，不新增战斗查询。不是永久技能所有权。
- `level`
- `exp`

关键约束/索引：
- `PRIMARY KEY(user_id, skill_id)`
- `idx_player_skills_user_order`

#### `player_battle_item_loadout`
作用：战斗道具袋持久化配置。

关键字段：
- `slot_no`
- `item_id`
- `quantity`

关键约束：
- `PRIMARY KEY(user_id, item_id)`
- `UNIQUE(user_id, slot_no)`

#### `player_specialization_states` / `player_specialization_allocations` / `player_specialization_ledger`
作用：分别保存玩家专精树的当前规则版本与点数账面、逐节点投入快照，以及分配/重置操作的幂等审计结果。

关键一致性字段：
- `ruleset_version`：持久化兼容版本，只在旧投入无法继续解释时提升。使用新 ID 追加专精树或节点不会改变既有状态的该字段，也不触发重置；版本不匹配，或同版本下存量投入已无法由当前定义解释时，服务端才会在玩家事务内执行一次免费强制重置并回写当前版本。
- `revision`：分配与重置的乐观锁版本；客户端必须提交当前版本。
- `rank` / `paid_cost`：节点当前等级与按当前专精定义重算后必须完全一致的累计消耗。
- `idempotency_key` / `result_jsonb`：同一玩家、树和幂等键只产生一个可重放结果。
- `last_ruleset_reset_jsonb`：保留最近一次强制重置通知，直到玩家下一次主动分配或重置；包括被清空投入、整件退回背包的装备、失效技能和前后 revision。

强制重置与投入清空、装备槽修复、整件装备回包、派生属性重算、装备条件级联检查和失效技能停用在同一数据库事务内完成。任何一步失败都会整体回滚，不会留下“专精已清空但装备仍占用已删除槽位”的中间状态。

关键约束/索引：
- 状态表与投入表分别使用 `PRIMARY KEY(user_id, tree_id)` 和 `PRIMARY KEY(user_id, tree_id, node_id)`；投入表无需重复索引。
- `UNIQUE(user_id, tree_id, idempotency_key)`：保证写操作幂等。
- `idx_player_specialization_ledger_user_tree`：按玩家、树与倒序 revision 审计操作。

#### `battle_rooms`
作用：战斗房间和调度租约。

关键字段：
- `owner_id`
- `dungeon_id`
- `auto_reenter_on_defeat`
- `is_public`
- `status`
- `entering_started_at`
- `recovering_started_at`
- `recovery_ends_at`
- `completed_runs`
- `current_run_id`
- `owner_worker_id`
- `lease_until`
- `next_tick_at`
- `phase`
- `schema_version`

关键索引：
- `idx_battle_rooms_public`
- `idx_battle_rooms_owner`
- `idx_battle_rooms_v1_status_due`
- `idx_battle_rooms_v2_playing_phase_due`
- `idx_battle_rooms_v2_state_due`

调度索引故意不包含 `lease_until`；V2 查询上线且替代索引生效后，存量库通过
`20260828_battle_room_claim_indexes_cleanup.sql` 并发删除旧的
`idx_battle_rooms_tick_claim` 与 `idx_battle_rooms_version_phase_claim`，避免高频租约更新继续维护旧索引。

删除语义：
- `battle_runs.room_id -> battle_rooms.id ON DELETE CASCADE`
- `battle_rooms.current_run_id -> battle_runs.id ON DELETE SET NULL`

注意：
- battle 域默认禁止 `TRUNCATE`，需要显式 `SET app.allow_battle_truncate = 'on'` 才能临时放开。

#### `battle_room_members`
作用：房间玩家成员。

关键字段：
- `user_id`
- `position`
- `is_ready`
- `ready_at`
- `joined_at`

关键约束/索引：
- `PRIMARY KEY(room_id, user_id)`
- `uq_battle_room_members_user`
- `idx_battle_room_members_room`

#### `battle_runs`
作用：单场 battle 编译与播放持久化。

关键字段：
- `room_id`
- `wave`
- `status`：`compiled|playing|settling|ended|failed|interrupted`
- `compile_input_jsonb`
- `timeline_jsonb`
- `playback_state_jsonb`
- `final_result_jsonb`
- `started_at`
- `ended_at`

关键索引：
- `idx_battle_runs_room`

#### `battle_run_payloads`
作用：单场 battle 冷数据 payload。运行表保留播放与汇总字段，完整编译输入、timeline 和最终结果落到此表。

关键字段：
- `run_id`
- `compile_input_jsonb`：兼容字段，新写入保持为空对象
- `timeline_jsonb`：兼容字段，新写入保持为空数组
- `final_result_jsonb`：兼容字段，新写入保持为空对象
- `compile_input_raw`
- `timeline_raw`
- `final_result_raw`

关键约束：
- `PRIMARY KEY(run_id)`
- `FOREIGN KEY(run_id) REFERENCES battle_runs(id) ON DELETE CASCADE`

#### `battle_settlement_jobs`
作用：结算异步作业。

关键字段：
- `room_id`
- `run_id`
- `status`
- `attempts`
- `last_error`
- `available_at`
- `locked_by`
- `locked_until`

`status='quarantined'` 表示作业与房间当前 run 的身份关系已确定性冲突。隔离行不会被 worker 自动领取，也不会被历史清理删除；修复数据后可人工改回 `pending` 重放。

关键约束/索引：
- `UNIQUE(run_id)`
- `idx_battle_settlement_jobs_claim`

#### `battle_skill_exp_jobs`
作用：战斗后技能经验发放作业。

关键字段：
- `run_id`
- `status`
- `payload_jsonb`
- `attempts`
- `last_error`
- `available_at`
- `locked_by`
- `locked_until`

`status='quarantined'` 表示 payload 缺失、损坏或包含无效玩家/猫咪/技能经验行。隔离行不会被 worker 自动领取，也不会被历史清理删除；确认并修复 payload 后可人工改回 `pending` 重放。

关键约束/索引：
- `UNIQUE(run_id)`
- `idx_battle_skill_exp_jobs_claim`
- 玩家与猫咪技能经验写入和 `status='done'` 在同一事务提交；领取、失败回写与完成写入都校验 `locked_by + attempts`，过期消费者不能重复发放或覆盖新 lease。
- worker 每次只 claim 一条任务，避免串行等待中的任务在本地过期后被其他消费者重复领取。

#### `battle_attr_exp_jobs`
作用：战斗后玩家与猫咪属性经验的可恢复发放作业，替代进程内有限次数重试。

关键字段：
- `run_id`
- `status`
- `payload_jsonb`
- `attempts`
- `last_error`
- `available_at`
- `locked_by`
- `locked_until`
- `applied_at`

`status='quarantined'` 表示 payload 为空、损坏，或包含无效玩家、猫咪、属性键、经验值及重复玩家行。隔离行不会被 worker 自动领取，也不会标记为已发放；修复 payload 后才可人工改回 `pending` 重放。

最后一名成员离开时，只要该房间仍有未完成或已隔离的结算、属性经验、技能经验作业，房间会隐藏并进入惰性的 `phase='reward_cleanup'`，避免外键级联删除待发奖励。最后一个作业的完成标记与空房删除在同一事务提交；隔离作业会保留房间、run 与 payload 供人工修复。

关键约束/索引：
- `UNIQUE(run_id)`：结算重试只能为同一战斗创建一个属性经验作业。
- `idx_battle_attr_exp_jobs_claim`
- `idx_battle_attr_exp_jobs_ready`：只覆盖未发放的 `pending/failed` 作业，避免完成记录增长后拖慢正常领取。
- `idx_battle_attr_exp_jobs_expired_lease`：只覆盖未发放的 `running` 作业，用于回收过期租约。
- outbox 行与 run 收尾、物品掉落和房间推进在同一事务创建；不存在“房间已进入下一场但属性经验作业尚未落库”的提交窗口。
- 属性经验写入与 `status=done`、`applied_at` 在同一事务提交；提交结果不明确时，重试会先锁定作业并检查 `applied_at`，避免重复发放。Apply/失败回写同时校验 `locked_by + attempts`，过期消费者不能干扰新 lease。
- worker 每次只 claim 一条任务，并在真正写经验前按 user id 顺序 try-lock；任一用户正被玩家可见事务占用时整条作业快速回退重试，不占连接等待。
- 经验进度仍逐次精确写入；只有跨越属性等级时才重算派生属性、行动时长、装备与成就依赖。
- 战斗历史压缩与全局清理会跳过仍有 `applied_at IS NULL` 作业的运行记录，避免外键级联删除待发奖励。

#### `arena_player_states`
作用：猫村联赛的玩家积分、奖章与主客场胜负统计；与普通战斗成长和奖励账本分离。

关键字段：
- `rating`：匹配展示用积分，不低于 0。
- `tokens`：奖章兑换处使用的猫村奖章。
- `attack_wins` / `attack_losses` / `defense_wins` / `defense_losses`：攻防两种身份分别累计。
- `challenge_charges` / `challenge_charge_refreshed_at`：可累计的比赛次数及其恢复基准。进入联赛页面或发起比赛时按数据库时间懒计算；默认每 10 分钟恢复 1 次，最多 10 次，满额期间不继续积蓄时间。

排行榜规则：
- 只统计已经公开主场小队，或至少完成过一场比赛的真实账号；仅打开联赛页面不会上榜。
- 先按 `rating` 降序，再按总胜场降序、总负场升序和 `user_id` 升序确定唯一名次。
- `idx_arena_player_states_leaderboard` 覆盖积分、胜负与稳定 ID 的排序路径；排行榜不写入额外快照表，也不生成机器人或测试玩家。
- 管理员可调用 `POST /api/admin/arena/leaderboard/reset`（`X-Release-Token`）原子重置全部玩家的排行榜状态：积分回到 `global-config.json.arena.defaultRating`，四项攻防胜负清零。猫村奖章、挑战次数、防守阵容、比赛记录与录像不受影响；重置完成后结算的新比赛会正常重新累计。

#### `arena_defenses`
作用：玩家公开的“一名玩家加可选猫咪”防守阵容及其冻结快照。当前业务允许零至两只猫咪；猫咪 ID 使用数组、阵容使用 JSONB 保存，未来提高上阵数量不需要增加固定列。

关键字段：
- `revision`：每次重新发布递增；worker 结算时必须与挑战创建时一致。
- `selection_jsonb` / `lineup_jsonb` / `lineup_digest`：选择、完整战斗快照与摘要哈希。选择内同时冻结主场采用的赛制定义；发布后换装或热更新赛制都不会改写已公开快照。
- `bounty_tokens` / `successful_defenses`：连续防守成功累积的悬赏与次数；首次被攻破时发放并归零。
- `active`：是否继续进入联赛目标候选池。

关键约束/索引：
- `arena_defenses_valid_kitty_ids`：猫咪 ID 必须为正数且互不重复，不在数据库内固化数量上限；当前上限由战斗服务的阵容校验统一控制。
- `idx_arena_defenses_active_level`：按战斗等级读取公开目标。
- `idx_arena_defenses_active_user`：围绕每日稳定随机起点有界读取最多 100 支活跃小队，并支持快速取得当前最大活跃玩家 ID。
- `idx_arena_defenses_active_format`：按标准赛或具体赛制筛选公开目标。

#### `arena_challenges`
作用：由 `battle-worker` 异步领取、编译和结算的猫村联赛队列，同时也是主客场双方的比赛记录索引。

关键字段：
- `battle_kind`：固定为 `arena`，用于与普通战斗明确区分。
- `attacker_lineup_jsonb` / `defender_lineup_jsonb`：创建挑战时冻结的双方完整快照。
- `format_snapshot_jsonb`：从主场小队复制的完整赛制 DSL 快照；worker 和录像只读取该快照，不受后续热更新影响。
- `rating_eligible`：挑战创建时冻结的计分资格。只有服务端签发的推荐小队凭证可以置为 `true`；从更多小队发起的比赛仍正常结算奖章、悬赏和胜负记录，但双方积分变化固定为 0。
- `seed`：确定性战斗种子；与 payload 一起支持参战双方回放。
- `status`：`queued|running|attacker_win|defender_win|failed`。
- `worker_id` / `lease_until` / `attempts`：可恢复 worker 租约。
- `attacker_reward_tokens` / `defender_reward_tokens` / `bounty_reward_tokens`：本场实际结算快照。

关键约束/索引：
- `uq_arena_challenges_attacker_active`：同一进攻方最多一场排队或运行中的挑战。
- `idx_arena_challenges_claim`：worker 使用 `FOR UPDATE SKIP LOCKED` 领取。
- 每场进攻会在同一事务内消耗 1 次 `challenge_charges`；写入前先取得进攻方用户锁并懒恢复次数，因此并发请求不能透支。worker 最终处理失败时在状态切换事务内退还次数。
- 推荐池固定最多 12 支小队：前 7 支从等级最接近的候选带选取，后 4 支从适当放宽的候选带选取，最后 1 支从其余候选中选取。玩家可主动换一批，默认冷却 5 分钟；换批时间与稳定种子只保存在 Redis，原子校验冷却，键在 24 小时加冷却时长后自动回收。Redis 状态丢失只会回到当日默认推荐，不影响排名、次数或比赛结算。推荐响应携带短期签名凭证，客户端不能把更多小队中的自由挑战伪造成计分赛。
- 更多小队的新版分页请求每页返回 20 支，但分页范围始终限制在服务端按玩家与 UTC 日期稳定抽取的最多 100 支样本内；未携带 `offset` 的旧客户端仍一次取得同一份有界样本。切换赛制只筛选同一批样本，旧版 `scope=all` 也不能越过样本末尾继续枚举。
- 同一对手的再次比赛间隔仍直接读取 `created_at` 与 `status`，由 `global-config.json.arena.opponentChallengeCooldownSeconds` 控制；`failed` 记录不参与限制，查询使用攻守组合索引。
- 攻方、守方各有独立倒序历史索引；清理时只有一场终局记录同时超出双方最近 50 场才会删除，保证每位参与者可滚动查看最多 50 场。

#### `arena_challenge_payloads`
作用：保存猫村联赛的初始状态、压缩时间线与最终结果。参赛双方可直接读取；其他玩家只能通过仍有效的分享令牌读取。

关键约束：
- `PRIMARY KEY(challenge_id)`，随挑战级联删除。

#### `arena_replay_shares`
作用：保存可投递到聊天的比赛录像分享令牌。任意已登录玩家可在录像仍属于创建者最近 50 场比赛记录时观看，但不能据此读取其他记录。

关键字段：
- `token`：高熵随机令牌，仅在创建分享时返回。
- `challenge_id`：对应录像；战绩因双方最近 50 场滚动保留而删除时，分享会级联失效。
- 同一参与者对同一场录像重复分享会复用仍有效的令牌；令牌离开创建者最近 50 场后立即不可读取，避免旧录像绕过滚动保留边界。
- `expires_at`：可空的预留人工过期边界；默认不设置时间失效。
- `revoked_at`：预留的主动撤销边界。

关键索引：
- `idx_arena_replay_shares_challenge`

#### `arena_shop_orders`
作用：预留的猫村奖章兑换审计和幂等凭据；当前兑换配置为空。后续开放兑换时，奖章扣除、物品入库与订单写入仍须在同一事务完成。

关键约束/索引：
- `UNIQUE(user_id, idempotency_key)`：客户端重试不会重复扣款或发货。
- `idx_arena_shop_orders_user_created`：按玩家读取购买记录。

### 建筑与猫舍

#### `player_buildings`
作用：建筑等级。

关键字段：
- `building_id`
- `level`

关键约束/索引：
- `PRIMARY KEY(user_id, building_id)`
- `idx_player_buildings_user`

#### `player_kitties`
作用：猫咪主档。

关键字段：
- `name`
- `level`
- `battle_level_cap`
- `exp`
- `total_exp`
- `bond_exp`
- `mood`
- `status`
- `last_commission_at`
- `extra_jsonb`

关键索引：
- `idx_player_kitties_user_created`

#### `player_kitty_personalities`
作用：猫咪性格装配。

关键字段：
- `kitty_id`
- `personality_id`

关键约束/索引：
- `PRIMARY KEY(kitty_id, personality_id)`
- `idx_player_kitty_personalities_user_kitty`

#### `player_kitty_battle_point_state`
作用：猫咪战斗加点状态。

关键字段：
- `total_points`
- `spent_points`

#### `player_kitty_attributes`
作用：猫咪动态属性明细。

关键字段：
- `attr_key`
- `base_level`
- `skill_point_level`
- `attr_exp`
- `source_mods_jsonb`
- `final_value`
- `lock_version`

关键索引：
- `idx_player_kitty_attributes_kitty`

#### `player_kitty_stats_cache`
作用：猫咪派生属性缓存。

关键字段：
- `stats_jsonb`
- `version`

#### `player_kitty_skills`
作用：猫咪技能配置与成长。

关键字段：
- `skill_id`
- `enabled`
- `cast_order`
- `rule_enabled`
- `rule_groups_jsonb`
- `level`
- `exp`

关键约束/索引：
- `PRIMARY KEY(kitty_id, skill_id)`
- `idx_player_kitty_skills_kitty_order`

#### `player_kitty_equip_slots`
作用：猫咪战斗装备槽位。

关键字段：
- `equip_type`
- `slot`
- `instance_id`

关键约束/索引：
- `PRIMARY KEY(kitty_id, equip_type, slot)`
- `idx_player_kitty_equip_slots_kitty_instance`
- `uq_player_kitty_equip_slots_instance`

#### `battle_room_member_units`
作用：房间内的实际出战单位清单。

关键字段：
- `owner_user_id`
- `unit_type`：`player|kitty`
- `unit_id`
- `kitty_id`
- `position`
- `is_ready`

关键约束/索引：
- `PRIMARY KEY(room_id, owner_user_id, unit_type, unit_id)`
- `uq_battle_room_member_units_kitty_active`
- `idx_battle_room_member_units_room`
- `idx_battle_room_member_units_owner`

#### `player_kitty_commissions`
作用：猫舍委托运行记录。

关键字段：
- `user_id`
- `kitty_id`
- `commission_id`
- `status`
- `costs_jsonb`
- `reward_jsonb`
- `ready_at`
- `claimed_at`

关键索引：
- `idx_player_kitty_commissions_user_kitty_started`
- `idx_player_kitty_commissions_user_kitty_status`

#### `player_kitty_encounters`
作用：猫咪相遇事件。

关键字段：
- `from_user_id` / `from_kitty_id`
- `to_user_id` / `to_kitty_id`
- `content_key`
- `content_params_jsonb`

关键索引：
- `idx_player_kitty_encounters_from_kitty_created`
- `idx_player_kitty_encounters_to_kitty_created`
- `idx_player_kitty_encounters_from_user_created`
- `idx_player_kitty_encounters_to_user_created`

#### `player_kitty_journals`
作用：猫咪日记与随遇日志。

关键字段：
- `user_id`
- `kitty_id`
- `encounter_id`
- `entry_type`
- `title_key`
- `content_key`
- `content_params_jsonb`
- `reward_jsonb`
- `extra_jsonb`

关键索引：
- `idx_player_kitty_journals_user_kitty_created`
- `idx_player_kitty_journals_encounter`

### 钓鱼与鱼塘

#### `fishing_ponds`
作用：保存每位玩家共享的鱼塘饲料库存、结算游标和鱼塘子规则版本。主池与鱼苗池共用这一行饲料库存，不拆成两套经济账户。

关键字段：
- `feed_stock_jsonb`：主池与鱼苗池共同消耗的营养库存。
- `last_settled_at`：离线生态结算的权威游标。
- `ruleset_version`：仅允许 `fishing-pond-v4|fishing-pond-v5`。存量行迁移时标记为 v4，服务端先按 v4 结清历史区间，再在同一事务中提升到 v5。
- `event_seq`：鱼塘事件序列。

#### `fishing_pond_pairs` / `fishing_pond_events`
作用：分别保存繁殖配对槽与玩家可见的鱼塘事件。配对鱼必须来自主池；鱼苗池满时不会消耗新的繁殖机会。

#### `player_item_instances.fish_data.pondZone`
作用：鱼仍是同一张实例表中的同一类 `fish_catch`，`pondZone=main|nursery` 只划分鱼塘内部区域。

关键约束：
- `chk_player_item_instances_fishing_pond_zone`：所有处于 `fishing_pond` 状态的鱼实例必须携带合法区域。旧服务若在重写 JSON 时丢掉该字段，整笔事务会失败，避免静默合池。
- `20260903_fishing_pond_v5_cutover.sql`：为旧行建立 v4 结算边界并补齐主池区域；必须在旧节点完全停止写入后手动执行。
- `20260903_fishing_pond_v5_cutover_down.sql`：鱼苗池仍有鱼时主动拒绝回滚，禁止无提示丢失区域语义。
- `server/schema/migrations/checks/20260903_fishing_pond_v5_cutover_check.sql`：只读检查未知规则版本、非法区域、待激活 v4 行和当前鱼苗数量。

### 动作队列与审计

#### `player_action_queue`
作用：动作执行队列。

关键字段：
- `user_id`
- `actor_type`：`player|kitty`
- `actor_id`
- `action_id`
- `action_type`
- `mode`：`run|queue`
- `total_count`
- `done_count`
- `queue_order`
- `status`
- `payload`
- `run_context_json`
- `next_process_at`
- `claim_version`
- `current_run_started_at`
- `current_run_ends_at`

关键索引：
- `idx_player_action_queue_user_actor_status_order`
- `idx_player_action_queue_active_lane_order`
- `idx_player_action_queue_player_next_process`
- `idx_player_action_queue_due`

#### `player_warehouse_change_logs`
作用：仓库变更审计日志。

关键字段：
- `event_type`
- `item_instance_id`
- `template_item_id`
- `quantity`
- `source_surface`
- `target_surface`
- `category`
- `direction`
- `subject_type`
- `subject_id`
- `item_totals`
- `instance_ids`
- `audit_key`
- `details`

关键索引：
- `idx_warehouse_change_logs_user_created`
- `idx_warehouse_change_logs_user_category_created`
- `uq_warehouse_change_logs_audit_key`

写入原则：
- 库存真值事务成功后再 best-effort 写审计日志。
- 日志写入失败不应反向回滚库存真值。

### 成就

#### `achievement_events`
作用：短生命周期的成就事件队列。该表是 `UNLOGGED` 表，不作为永久真值，数据库崩溃恢复后可能被清空。

关键索引：
- `achievement_events_pkey`：按 `event_id` 幂等写入、锁定和完成事件。
- `idx_achievement_events_pending_claim` / `idx_achievement_events_processing_claim`：按状态领取待处理事件和回收过期租约。
- `idx_achievement_events_pending_due` / `idx_achievement_events_processing_due`：与两条 `FOR UPDATE SKIP LOCKED` 候选查询的状态谓词及排序一致。
- `idx_achievement_events_processed`：批量清理已完成事件。

存量库通过 `20260902_achievement_event_index_cleanup.sql` 并发删除已经被上述索引覆盖的
`idx_achievement_events_claim`，以及业务代码不再读取的 `idx_achievement_events_user_status`。
索引物理膨胀的回收属于线上维护，不放进 schema 迁移；需要时在低峰单独执行
`REINDEX TABLE CONCURRENTLY achievement_events`。

#### `player_achievement_states`
作用：玩家各成就的进度、完成、领奖状态和定义版本快照。

关键索引：
- `idx_player_achievement_states_user_completed`
- `idx_player_achievement_states_user_claimed`

### 技能书优化

#### `player_custom_skills` / `player_custom_skill_drafts` / `player_custom_skill_revisions`
作用：分别保存账号自定义技能资产、活动设计草稿，以及不可变的炼成尝试与编译结果。在线状态接口只读取活动草稿、待处理结果和未归档已学习技能；失败与归档历史不进入常规状态响应。

关键一致性字段：
- `revision_no`：草稿乐观锁；炼成必须同时提交预期 revision 与权威预览图哈希。
- `ruleset_version`：运行规则版本；禁用某版本后，其产出的技能不能进入新战斗且战斗热路径不查库。
- `result_status`：`pending|accepted|discarded|superseded|failed`。
- `idempotency_key`：炼成扣料与掷点的账号级幂等键。

关键索引：
- `idx_player_custom_skill_drafts_user_active_updated`：有界读取活动草稿，避免扫描历史完整图。
- `idx_player_custom_skill_revisions_user_pending_created`：有界读取待处理结果。
- `idx_player_custom_skill_revisions_ruleset`：按规则版本停用或审计产物。

#### `player_forged_skillbooks`
作用：绑定唯一物品实例与已接纳 revision，保存 `inventory|learned_player|learned_kitty|destroyed|archived` 生命周期。未使用成品必须走销毁接口，不能通过技能归档接口留下仍可消费的孤立物品。

迁移归属：
- `20260814_player_custom_skill_forge.sql` 是整套结构的唯一创建与完整回滚所有者。
- 后续 bound、book-count、ruleset 升级脚本是兼容旧测试库的前向补丁；对应 down 不得删除基础迁移拥有的表、字段或安全版本开关。

### 离线恢复与战斗奖励

#### `player_offline_recoveries`
作用：持久化离线时间窗、累计结算回执、检查点 revision、失败重试和客户端确认状态。库存、队列推进、累计回执在同一事务提交；`schedule_pending` 保留提交后恢复正常队列调度的意图。

`background=true` 表示离线期间持续推进的未封闭时间窗，同一用户最多一个。登录时在用户锁下封闭原记录并增加 revision；后台分段完成不移动封顶时间窗、不恢复在线调度，也不提前确认报表已读。

索引覆盖用户最新任务、未读回执、未完成任务、到期任务和待恢复调度。`acknowledged_at` 只确认已收到回执，不删除时间窗或改变发奖结果。不能用 Redis TTL 或浏览器连接生命周期清理任务。

#### `battle_reward_receipts`
作用：与战斗库存奖励同事务写入的发奖凭证，以 `(run_id,user_id)` 唯一约束防止重复发放。同一键不同奖励计划直接报错。用户与结算时间索引用于离线汇总。

不关联可清理的战斗房间/回放外键，避免清理回放时删除发奖证据。旧仓库日志只作为明确标识的兼容汇总，不作为补发依据。

迁移：依次执行 `20260907_offline_recovery.sql`、`20260908_background_offline_recovery.sql`。上线顺序、验证和保留边界见 [offline-recovery.md](offline-recovery.md)。

## 已清理/废弃
- `player_backpack_items`
- `player_backpacks`
- `player_loot_cache_entries`
- `player_activity_state`
- `player_routine_jobs`
- `player_routine_settlements`
- `player_warehouse_items`
- `player_warehouses`
- `idle_jobs`

## 更新流程
1. 修改 `server/schema/init.sql`
2. 同步本文
3. 执行 `bash infra/scripts/check-schema-doc-sync.sh`
4. 执行 `cd server && go test ./...`
