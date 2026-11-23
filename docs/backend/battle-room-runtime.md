# 战斗房间运行时（当前实现）

## TL;DR
- 房间状态：`idle -> preparing -> entering -> playing`，玩家全灭后进入 `recovering`。
- 房间配置与站位修改仅允许在 `idle / preparing`，`entering / playing / recovering` 一律拒绝。
- 房间人数上限按副本 `world.maxTeamSize` 动态校验（创建/编辑/加入一致生效），不再使用固定人数常量。
- 房间支持自定义名称 `room_name`（创建必填），并支持房主在房间内移交房主身份。
- 房间聊天采用动态频道 `battle_room:{roomId}`：仅房间成员可收发与拉历史，离开房间后自动退出该频道且不可再读历史。
- 每一场战斗使用独立 `battle_runs` 记录（每场一个 `runId`）。
- 仅首场（`wave=1`）拉取玩家实时属性/装备/技能快照；后续场次沿用上一场结算状态（HP/MP/技能冷却等）。
- 从第二场开始，备战回合采用两段式继承：若上一场“备战队列”仍有未行动且存活单位，则下一场仅继承这些剩余备战单位；若上一场备战队列已全部行动完，则回退为从普通回合里“未行动且存活”的友方单位生成备战队列。备战回合完成后再进入常规回合。
- 技能冷却单位统一为“回合”，并在**单位回合结束**后统一 `cooldownRemain - 1`（最低到 0）。
- 战斗技能释放门禁：资源校验（HP/MP）+ 冷却校验 + 自动释放规则（未配置规则默认放行）。
- 若技能序列（含普通攻击）全部因门禁/规则未通过，本回合进入 idle（不再绕过规则强制回退到普通攻击）。
- 战斗物品与技能同属回合内动作：物品使用发生在单位动作前，回放中会生成独立 `item_use` 节点，且同回合只允许使用 1 次。
- 战斗物品使用默认按“自我优先”目标策略执行；物品规则同样走 `player_rule_sets(kind=item)`。
- 回放节拍由 `battle.timelinePushIntervalMs` 控制；场间等待由 `battle.interBattleIntervalMs` 控制（当前配置 `3000ms`）。
- `battle-worker` 采用 `next_tick_at` 驱动调度，不依赖固定间隔轮询；单轮 claim 后会对多个房间做有界并发推进，而不是整批串行执行。

## 1. 职责边界
- `server`（gateway）：HTTP/WS 接入、房间读写、事件转发。
- `battle-worker`：认领房间、推进 entering/playing、编译与回放、结算与场间切换。
- `server/internal/battle`：战斗可扩展逻辑目录（core/skills/buffs/equipment），用于承载核心计算与脚本化扩展点。

## 2. 状态与时序
1. 全员 ready 后房间进入 `entering`，并写入 `entering_started_at`。
2. 到达 entry duration 后进入 `playing`，创建当前场次 run。
3. run 先快速演算（compile），再按 timeline 做分批回放（push batch）。
4. 单场胜利后即时结算该场奖励，等待 `interBattleIntervalMs` 后自动开始下一场（不再有场次上限）。
5. 玩家方玩家单位全灭后进入 `recovering`；重整结束后按房间配置 `auto_reenter_on_defeat` 自动进入 `entering` 或回到 `idle`。
6. `recovering` 期间禁止成员离开房间。
7. 房间关键配置（名称/地牢/可见性/自动重入）与成员站位仅可在 `idle / preparing` 修改，避免战斗链路中状态写入导致脏数据。

## 3. 玩家状态继承规则（关键）
- `wave=1`：从玩家当前实时数据构建战斗单位快照（属性/装备/技能）。
- `wave>1`：从上一场 `finalState.players` 继承玩家战斗状态，不再重新拉取实时玩家属性。
- `wave>1`：会读取上一场结算中的 `nextPreparationOrder`，作为本场一次性 `preparationOrder` 队列；其生成规则是“优先继承上一场剩余备战队列（未行动且存活），否则回退到普通回合未行动且存活单位”。
- 这意味着战斗进行中在外部加点/换装，不会影响当前战斗链路，直到本次房间战斗结束并重新开始新一轮。
- 单位快照对前端提供运行态字段：`attributes(key->value)`、`skillIds`、`equipmentInstanceIds`、`buffs`、`skillCooldowns`、`basicAttack`、`battleItems`（并保留旧字段兼容）。
- `basicAttack` 展示公式语义：优先使用结算同源的 `effects/damageSpec.amountFormula`，并向下兼容旧快照中的 `basicAttack.formula`。
- 出手顺序按单位 `speed` 降序重建（同值按 `unitId` 兜底稳定排序）。
- 玩家单位在每场战斗开场会按站位自动挂载阵位 buff：前排（双抗 +10%）、中排（速度 +10）、后排（物理/魔法增伤 +10%）。
- 角色战斗配置由 `player_skills` 单表驱动：`enabled/cast_order`（自动释放候选顺序）+ `rule_enabled/rule_groups_jsonb`（技能规则）；普通攻击固定存在且不可停用。
- 技能触发路由由技能注册描述符驱动（active/passive + triggers，如 `on_hit`），runtime 不再按技能 id 写分支逻辑。
- 技能回调除返回 `effects[]` 外，还可通过运行时提供的可变上下文直接操作战场（select target / damage / heal / resource / buff / summon），所有关键动作会同步写入回合节点 `steps[]`。
- 技能回调上下文中的 `SkillRuntime` 已携带当前技能等级（来自 `player_skills.level`），脚本可按等级做强度增幅。
- 回放 timeline 采用“**回合主节点**”模型：`battle.timeline.node`（一个单位一次回合），节点内按顺序包含 `steps[]`（如 regen/cast/damage/heal/trigger/summon/turn_end/state_attach/state_detach/stat_recalc）；压缩 step 中 damage 会显式携带暴击标记 `c`（兼容 `crit`）与伤害类型 `dt`（兼容 `damageType`），状态相关 step 可携带 `bi`（兼容 `buffId`）用于标识状态 id，target 可携带 `b/h/m/l/x/y`（beforeHp/afterHp/afterMp/afterAlive/maxHp/maxMp）。
- 每个 step 支持 `delayMs`（可由 hook/技能脚本控制子事件间隔）与 `display`（前端特效提示，如连线颜色/样式/effectKey）。
- 单位战斗态采用分层模型：`baseStats`（战斗内固定基线，玩家取入场 final，敌人取“入场原始值做一次派生投影后的结果”）、`statModifiers`（运行时 Delta，支持正负）、`statSources`（来源拆分）、`stats/attributes`（最终值）；计算语义为 `final = base + delta`，并且派生公式仅作用在 Delta 层，避免对入场已结算值重复计算。
- `stat_recalc` 动作会显式携带 `deltaStats`（与 `statModifiers` 同步），便于前端/回放直接观察增量层。
- 跨 wave 继承时，`baseStats` 与 `statModifiers` 分层原样继承，不做扁平合并；这样 buff/state 失效时可按来源精确回滚。
- `quick_slash` 已切换为技能内自定义目标策略（敌方存活单位：最高血 + 最低血），不再依赖固定 `TargetCount=2` 预选逻辑。
- 目标选择会叠加状态语义：`untargetable_by_enemy` 可让单位在敌方点选时不可作为候选；`treat_all_as_enemy` 可让施法者把全体单位视作敌方候选；`prefer_as_enemy_target` 可让单位在敌方**非全体**选目标时被优先命中（模板技能、脚本 `SelectTargets(side=enemy)`、普通攻击统一生效）。`targetScope=self` / `selectRule=self` 会直接选中自身，并保持现有目标语义链路；当存在该语义目标时，优先级高于技能排位过滤（可跨前/中/后排吸附），但仍受 `untargetable_by_enemy` 约束。`skills.effects[].ignoreUntargetable=true` 时该 effect 可显式忽略 `untargetable_by_enemy` 过滤。
- `camouflage_targeting_remap` 会把带伪装单位在敌方点选关系中重映射为“友方语义”，从而在存在其他敌方候选时优先被避开；若无可替代候选，不会导致非法目标集。
- 运行时单位新增三层侧别语义：`originSide`（稳定归属阵营，用于胜负判定）、`side`（当前所在半场）、`perspectiveSide`（关系判定视角）。常规情况下三者一致；受状态影响时可分离。
- buff DSL 额外支持 `side_override`（临时迁移半场）与 `invert_relation_perspective`（翻转关系视角）。两者都在状态失效后自动恢复到基础归属语义。
- 站位（前/中/后排）本身不提供独立“仇恨/索敌 AI”语义；它只是技能 targeting 的过滤维度之一。每次出手都会按该技能/普攻配置（`targetPositions`、`secondarySeek`、`selectRule`、`targetCount`）+ 当前状态语义重新计算候选。
- effect 层 `targetCount.max`（与 `base/formula` 同层级）用于限制 `base + floor(formula)` 阶段结果；affix `effectMutations.targetCountDelta` 在该上限之后生效，可突破该上限。
- `skills.effects` 与 `basicAttackEffects` 支持 effect 间目标联动：`reuseTargetsFromEffect` 可复用前序 effect 目标；`onlyWhenNoTargetFromEffects` 可在前序 effect 全部无目标时才触发（ALL-empty，1-based 前序索引）。
- 装备战斗钩子新增 `on_battle_start`：在**每次新波次生成后**、首个单位回合开始前，对全场单位各触发一次；适合实现“开场给自己挂状态（如嘲讽）”。
- 新增治疗模板技能：`basic_heal`（最低生命比例友军单体治疗）与 `adv_heal`（群体治疗友军）。
- 冷却语义：单位释放技能后先设置该技能冷却为配置 `cooldown`，再在该单位回合结束时统一对其所有技能冷却 `-1`（最低 0）。
- 伤害阶段顺序：
  1) 先按穿透结算防御（先固定穿透再百分比穿透：`effectiveDefense = max(0, (defense - flatPen) * (1 - percentPen))`），超额固定穿透转为固定加伤；
  2) 再用递减防御曲线结算基础伤害：`damageBeforeCrit = rawDamage * K / (K + effectiveDefense)`，`K` 来自 `shared-global-config.combatBalanceModel`；
  3) 再做物理/法术独立暴击结算，暴击伤害抵抗只削减暴击的额外部分；
  4) 最后按伤害类型执行攻击方增幅（`* (1 + amplification)`）与受击方减免（`* (1 - resist)`）。

## 4. 回放节拍与调度
- `timelinePushIntervalMs`：单位回合结束后的最小等待间隔；开局先缓冲一个间隔，之后下一单位开始时间 = 上一单位节点真实时长 + 该间隔。
- `playbackMaxEventsBatch`：单次 `battle.run.batch` 最大事件数，用于限制单窗口推送尖峰；同一 offset 超量时自动拆成多个批次连续下发。
- `playbackMaxGapMs`：回放推送时允许的最大单次等待间隔；当 timeline 相邻 offset 出现异常大 gap 时，只限制发包等待，不改变 timeline 数据本身。默认 `3000ms`。
- 回放完整快照保存在 worker 内存，不逐批次落 PostgreSQL；轻量播放游标会写入 Redis，worker 重启后可结合数据库中的预演产物恢复。新编译成功交接 playback lease 时会先预热本地 checkpoint，首批快照不再额外等待一次 Redis 冷读取，首批发送后仍按原语义持久化恢复游标。
- `playing + run=running` 场景下，回放由独立 playback worker 在进程内按下一次推送时间驱动，避免每个回放批次都通过主 tick 路径写 `next_tick_at`。
- playback worker 接管后会把房间 owner 释放为 `''` 并仅保留 `lease_until` 防重，避免同 worker 在主循环里重复 claim 同一回放房间。
- playback lease 采用短 TTL（默认 `4s`）并由 playback worker 低频续租；若续租失败会按房间实际状态快速退出，避免僵尸 worker 长时间占有租约。
- worker 重启后的 V2 playback 恢复使用轻量 run/member 查询，并按 `tickShardWorkers` 有界并行启动，避免存量回放房间逐个加载冷 timeline 导致整批长时间静止。
- 播放完成会先进入内部里程碑（playback finished）再触发结算；结算以 run 结果为准执行一次事务收口，重复 finalize 调用会按 run 状态判重后直接跳过，避免重复发奖。
- 网关投递层对离线用户启用统一消息闸门：`battle.run.batch` 属于用户态实时消息，用户 `offline` 时不下发。
- `battle.run.batch` 还受前端 battle tab 订阅状态控制：客户端通过 `battle.stream.set { enabled }` 显式订阅/退订；未订阅连接不下发该消息。
- worker 以 `battle_rooms.next_tick_at` 作为唤醒依据，动态睡眠后再 claim due rooms。
- worker 会优先消费 Redis `battle:due_rooms`（按 next_tick_at score）中的到期房间，再回退到 DB claim；这样可减少 DB 侧“全局到期扫描”压力。
- 调度目标是减少固定轮询抖动，保持多实例下推进顺序稳定；实例内采用 room-id 分片并发（shard 间并行、shard 内串行）推进 room tick。
- 分片并发上限由 `battle.tickShardWorkers` 控制（默认 `8`），用于按实例容量调优吞吐与租约安全余量。
- room claim 批量采用自适应上限：`battle.tickClaimRoomsMin / battle.tickClaimRoomsMax / battle.tickTargetLeaseUtilPct` 共同决定每轮认领量（基于本 worker 观测到的每房间执行耗时 EWMA），在高负载下自动收敛，低负载下自动放大。
- room claim 还会受回放节拍预算约束：当 `timelinePushIntervalMs` 与 EWMA 推导出的批次预算更紧时，认领上限会进一步下调，避免单批执行时间长期超过回放节拍目标。
- 预演编译步数预算按房间玩家单位动态计算：`battle.compileTurnsPerPlayerUnit * 玩家单位数(player+kitty)`（默认每单位 `100`），并受 `battle.compileMaxExecMs`（默认 `2500ms`）守护；若触发预算/执行时限，会标记 compile 失败并记录守护日志。
- 主 tick 不再同步执行预演编译；`compiled` run 只会投递到有界异步 compile 槽，并按 `run_id` 去重，避免单个长编译拖垮整批 room tick lease。
- compile 槽并发由 `battle.compileKickoffSlots` 控制（建议单机 4c8g 从 `4` 起步，根据 CPU 利用率与 `run_start_to_first_push_ms` 再微调）。
- ready-dispatch 只在 compile 队列达到真正 `backlogged` 阈值时降为每 `100ms` 最多启动 `1` 个新 run；编译槽正常满载但队列仍有容量时，会按剩余队列容量继续批量启动并保持忙轮询，批内最多使用 `2` 条有界执行 lane，并将 run 启动后的 room/member/unit 读取合并为单次 PostgreSQL batch 往返（仍保留猫咪人格站位锁定处理），避免跨房间串行等待长期占满固定 3 秒后的启动窗口。tick claim 在真实 backlog 下仍会临时降速，优先消化既有 compile backlog。
- compile backpressure 下会优先保障新开局链路（`prepare/ready_dispatch/ensure_run`）：低优先级来源（`tick_retry/recovery_auto_reenter`）会暂缓投递 compile，且自动重入在高压期会延后启动，避免失败房间反复重进挤占启动吞吐。
- backpressure 分为两级：`pressured`（`pending >= slots/2` 或 `in_flight >= slots`）限制低优先级 compile 投递，但 ready-dispatch 仍按剩余队列容量推进；`backlogged`（`pending >= slots` 或 `in_flight >= slots*2`）会暂停新的 compile-dispatch claim，并按上述低速启动新 run，避免波次切换完全饿死。
- compile 失败会按 `room_id + current_run_id + phase=compiling` 持久化指数退避，并同时保留进程内 not-before 兜底；compiling claim 不允许同一 worker 绕过未到期 lease，损坏场次不能热循环占满 compile 槽。
- 跨波次 hook 限次、冷却与概率计数使用可打印、无歧义且不含 NUL 的复合键；所有直接写入 PostgreSQL JSONB 的战斗快照在事务前拒绝 U+0000。compile 保存先按 room -> run 的统一锁序校验当前场次，再原子提交 run、carry/cold payload 与 V2 `phase=playback`，避免死锁和陈旧编译覆盖；cold final result 复用已编码并校验的 carry JSON，保留完全相同的持久化结构，同时避免对大型单位图做第二次递归编码。
- battle 热路径日志默认降噪：常规 `run started / first batch / finalize / settlement / skill-exp` 不再逐房间刷屏，仅在慢路径阈值触发时输出 `... slow` 日志。
- 慢批次日志会输出 `compiled_runs/compiled_turns`，用于快速判断是否由战斗预演回合数抬高了 tick 耗时。
- 结算 worker 数与 `finalizeSlots` 保持一致（当前均为 4），每个 worker 按小批量 claim（默认每次最多 4 条）串行处理；这样既能用满受控结算槽，又不会在 finalize semaphore 前预领过多任务。技能经验 worker 同样采用小批量 claim，减少高并发下的 claim 往返开销。
- run 收尾（`battle.run.finished` 前的结算事务）遇到 PostgreSQL `40P01/40001` 瞬时并发错误时会做短重试，降低高并发下“击败后停滞”概率。
- 玩家/猫咪属性经验 outbox 与 run 状态、物品掉落和房间推进在同一结算事务创建；属性 worker 每次只领取一条任务，按 claim owner + attempt fencing，并对玩家锁使用按序 try-lock 快速回退，不能阻塞玩家可见的结算事务或让过期 lease 改写新 owner。
- 管理员可通过 `GET /api/admin/battle/metrics`（`X-Release-Token`）查看 batch / compile / playback 的最近样本分位数；响应新增：
  - `runtime`：当前进程生效的 `GLOBAL_CONFIG_PATH` 与 battle 关键节拍参数（用于线上/本地配置对齐）。
  - `metrics.playbackDispatch`：`battle.run.batch` 实际下发节拍（dispatch/base interval/offset gap/next delay/events）。
  - `metrics.wsBatchDelivery`：batch 投递计数（attempts/delivered/skippedPresence/skippedNotSubscribed）。
- compile 相关慢路径调试日志：
  - `battle worker config snapshot`：worker 启动时打印当前 battle 关键调度参数快照。
  - `battle run compile guard triggered`：预演命中 `compileMaxExecMs` 或步数预算守护时打印。
  - `battle run compile slow`：单次 compile 本身耗时过慢时打印，包含 `wave / total_steps / total_turns / timeline_events / final_seq / turn_profile_*`。
  - `battle compile kickoff slow`：从 run 启动到 compile worker 真正处理的整段链路过慢时打印，包含 `queue_since_start_ms / compile_elapsed_ms / total_ms`，用于区分“排队慢”还是“执行慢”。
  - `battle slow run archive upsert failed`：battle-worker 尝试写入慢战斗归档表失败；不影响主 battle 流程，但说明 admin 侧后续无法追溯该次慢战斗。
  - `battle slow run archive trim failed`：battle-worker 写入归档后，按“每房间最多 5 条”裁剪旧归档失败；不影响主 battle 流程，但会导致该房间归档条数暂时超限。
- 新增慢战斗归档：admin 不再依赖跨机读取 `journalctl`，而是由 `battle-worker` 在命中 compile 相关慢/异常条件时把诊断快照直接写入 DB 的 `battle_slow_run_archives`：
  - 触发条件：
    - `queue_since_start_ms >= battleCompileQueueSlowLogThresholdMs`
    - `compile_elapsed_ms >= battleCompileSlowLogThreshold`
    - `compiled.TimedOut == true`
    - `compiled.StepLimited == true`
  - 归档内容：
    - 标量摘要：`room_id / run_id / dungeon_id / worker_id / source / wave / outcome`
    - compile 指标：`queue_since_start_ms / compile_elapsed_ms / total_ms / total_steps / total_turns / timeline_events / final_seq / player_units`
    - 守护与原因：`guard_timed_out / guard_step_limited / trigger_flags_jsonb`
    - 完整诊断快照：`turn_profile_jsonb / compile_input_jsonb / checkpoint_jsonb / timeline_jsonb / final_result_jsonb`
  - 保留策略：
    - 每个 `room_id` 仅保留最新 5 条归档
    - 全局建议仅保留最近 30 天，通过 admin cleanup 分批清理
    - 与 `battle_rooms` / `battle_runs` 生命周期解耦；即使原始 run 被业务清理，归档仍保留，便于追溯
- 新增 admin 接口（均需 `X-Release-Token`）：
  - `GET /api/admin/battle/slow-runs`
    - 查询参数：`roomId`、`runId`、`limit`、`beforeId`
    - 返回摘要列表，不返回大 JSON
    - 每项包含 `analysis`：
      - `primaryCause`：`queue_backlog` / `compile_execution` / `compile_guard` / `mixed`
      - `dominantTurnPhase`：`preAction` / `execute` / `postAction`
      - `dominantTurnPhaseMs`
  - `GET /api/admin/battle/slow-runs/:archiveID`
    - 返回单条完整归档详情，包含 `turnProfile / compileInput / checkpoint / timeline / finalResult`
  - `POST /api/admin/battle/slow-runs/cleanup`
    - 请求体：`keepDays`、`batchSize`、`rounds`
    - 用于分批清理 30 天前的慢战斗归档
- battle 热路径事件推送优先复用当前 room member 列表做定向下发，避免为推送再回源查询房间详情。
- room tick 执行链路采用阶段化 pipeline（load room -> ensure run -> load runtime -> compile -> playback），并缓存 world/enemy 配置解析结果，降低每 tick 重复计算成本。
- battle runtime 代码按职责拆分：`battle_runtime_compile.go`（预演编译）、`battle_runtime_playback.go`（回放推送）、`battle_runtime_finalize.go`（收尾结算）、`battle_compile_policy.go`（队列压力与限流策略）、`battle_tick_state_machine.go`（tick 状态机与阶段执行器），减少在单文件持续打补丁导致的耦合。
- compile 慢日志新增 `turn_profile_*` 指标（`pre_action/execute/post_action/total`），用于定位 `processBattleTurn` 子阶段热点而非只看总耗时。
- 伤害通道已抽象为统一 profile（当前先迁移 `physical/magical`），把防御/增伤/减伤/暴击/穿透等键位映射集中管理，后续扩展新伤害类型时优先走“加通道配置”而非散落 `if magical` 分支。
- 战场级机制由 `world.battlefieldEffects[]` 驱动。运行态、最终状态和跨波 carry 都使用 `battlefieldEffects: [{ id, remaining, progress, goal, windows }]`；紧凑快照使用可选 `bf`。
- `alive_roster_cycle` 在合资格正常行动机会消耗后推进，受压制/跳过行动按配置计数；备战、镜像施法、额外行动和被过滤的召唤物不推进当前流星回廊计数。
- 战场触发固定发生在主行动及其钩子完成后、波次胜负预检前。触发事件顺序为 `battle.field.tick -> battle.field.trigger -> battle.field.window`，因此清掉最后敌人的同一行动仍先承受战场结算。
- 胜利 carry 倒计时、半轮进度和窗口；失败不写战场 carry。退出、解散和从第一波重开自然获得全新状态。
- `weighted_random_per_target` 按合资格存活单位计算抽取数，使用服务端 RNG 无放回抽样，再对每个目标独立选择带权结果。结果中的状态、持续回合和技能冷却前后值随 `battle.field.trigger.targets[]` 下发，客户端只合并权威结果。
- `adjust_skill_cooldown` 是通用有符号效果：正数延长、负数缩短并在 0 截断；可统一用于技能、Buff 和战场效果。旧 `add_skill_cooldown` 仍只接受正数并保持兼容。

## 5. 数据与存储
- 房间主表：`battle_rooms`（含 `owner_worker_id/lease_until/next_tick_at` 认领租约字段，以及 `auto_reenter_on_defeat/recovering_started_at/recovery_ends_at`）。
- 成员表：`battle_room_members`（位置、ready 状态）。
- 房主可在非 `playing` 状态下将 `owner_id` 移交给房间内其他成员。
- 场次表：`battle_runs`（compile input、timeline、playback state、final result）。
- 慢战斗归档表：`battle_slow_run_archives`（compile 慢/异常场次的独立诊断快照，不依赖 `battle_runs` 存活）。
- 历史 run 清理：仅保留每个房间最近 `battle.runHistoryKeepCount` 场（当前配置为 10），其余历史 run 物理删除（排除当前 `current_run_id`）；对存量数据可通过 admin 接口 `POST /api/admin/battle/runs/cleanup` 手动分批清理，避免定时任务持续占用 DB。
- 慢战斗归档清理：与 `battle_runs` 分离，不参与普通 run compact；建议通过 admin 接口 `POST /api/admin/battle/slow-runs/cleanup` 分批删除 30 天前归档。

## 6. 前端表现约定
- 战斗页单位卡片采用独立组件，悬浮展示单位属性/技能/装备摘要。
- 单位属性项支持悬浮构成明细（来源拆分），Buff 改为 quickRef 形式展示。
- 副本预览中的 `EnemyUnitQuickRef` 属性口径与战斗入场一致：展示敌人基础属性经过派生公式投影后的最终值（非 `enemies.json` 原始 stats）。
- 单位卡片 HP/MP/存活由回放快照 + 事件增量驱动，不依赖页面轮询。
- 回放以 `battle.timeline.node` 作为核心事件；前端在节点内按 `steps[]` 顺序播放，step 间隔由 `delayMs` 控制，节点结束后再等待 `timelinePushIntervalMs` 才进入下一个 timeline 节点。
- 战斗状态快照与 `battle.timeline.node` 会携带 `preparationOrder`；前端在“行动顺序”上方单独展示“备战回合”，队列消费完毕后自动隐藏。
- 备战回合继承优先级：先看“上一场备战队列是否有剩余未行动且存活单位”；只有在该集合为空时，才会回退为“上一场普通回合中未行动且存活单位”。
- 前端会为状态增减 step 显示悬浮文字：`buff` 显示 `+状态名`，`state_detach` 显示 `-状态名`（状态名由 buff 定义 i18n 解析）。
- 前端单位卡片 buff 列表由 timeline 增量状态驱动：`buff` step 负责挂载/刷新，`state_detach` step 负责移除；不能仅依赖浮字展示。
- 前端 replay 队列会参考相邻事件的服务端时间差，避免客户端等待时间明显超过服务端推送节奏。
- `battle.run.batch` 常规批次以增量事件为主；中途进入 battle 页时，前端应先补拉 `/api/battle/rooms/{roomId}/run` 建立当前基线，再继续消费增量事件。
- 物品使用语义保留为 `item_use` 阶段，但编码到同一 `battle.timeline.node` 内（与同回合 regen/skill_cast 同节点分阶段播放），不再单独形成一个 `battle.run.batch`。
- 客户端合并规则：当 batch 含 `snapshot`（含 `seq`）时，应将其视为该批次窗口后的权威状态，仅继续应用 `seq > snapshot.seq` 的增量事件，避免同批事件重复演算导致 CD/HP 漂移。
- 战场状态条只消费快照中的 `battlefieldEffects/bf` 与 `battle.field.tick/trigger/window` 事件；本地 world 数据只提供名称、说明、图标和静态初始值，不得在断线重连或中途加入时自行推算倒计时。
- 前端仍保留补拉 `/api/battle/rooms/{roomId}/run` 的兜底路径，中途进入与断档恢复均应以该基线为起点再衔接增量。
- 切回同一战斗房间时复用已有可播放快照，不先清空画面；`/rooms` 已携带自己的完整房间时不重复串行拉 detail。同一 `roomId + runId` 的 `/run` 补拉采用 single-flight，不同 run 到达时必须启动新请求并淘汰旧响应。
- 前端应对长回放增加“停更自愈”：当 `battle.run.batch` 长时间无新包、或事件序列出现断裂（`seq` 非连续）时，主动补拉 `/api/battle/rooms/{roomId}/run` 后继续播放，避免动画停播。
- `battle.run.finished` 事件现在会带上 `failureReason`（如 `compile_timeout` / `compile_step_limit`）以及 `compileTimedOut`、`compileStepLimit`，用于区分“正常战败”与“预演守护触发失败”。
- timeline 播放完成后的最终结算会按玩家技能释放次数统一发放技能经验；普通攻击、无等级技能、满级技能不参与。
- 是否下发 `battle.run.batch` 由两层条件共同决定：全局 presence 投递策略（offline 拦截）+ battle tab 显式订阅状态（未订阅拦截）。
- 前端不再弹出“战斗已开始”提示，房间结束不再依赖固定延迟强刷，避免最后节点回放被打断。
- 房间聊天频道名应显示 `room_name`；`battle.room.self` 若收到 `{ room: null }`，前端应立即移除房间聊天频道并清空该频道未读/历史缓存。

## 7. 限流错误码（RATE_LIMITED）
- 网关限流基于 IP 固定窗口（Redis 计数），超限返回 HTTP `429` + `code=RATE_LIMITED`，并附 `Retry-After`。
- 若 Redis 限流后端异常，则返回 HTTP `503` + `code=RATE_LIMIT_BACKEND_UNAVAILABLE`。

## 8. battle 数据清理安全规约
- battle 外键拓扑存在循环关系：`battle_runs.room_id -> battle_rooms(id) ON DELETE CASCADE` 与 `battle_rooms.current_run_id -> battle_runs(id) ON DELETE SET NULL`。
- 语义上，`DELETE battle_runs` 不会反向删除 `battle_rooms`；`DELETE battle_rooms` 会级联删除对应 `battle_runs`（例如房间成员清空后删除房间时）。
- 生产清理历史 run 时，使用 `POST /api/admin/battle/runs/cleanup`（分批）或显式 `DELETE ... WHERE ...`，不要用 `TRUNCATE ... CASCADE`。
- 目前仓库内 battle 清理实现为 `DELETE` 路径（收尾 compact + admin purge），未发现 battle 域 `TRUNCATE` 代码路径。
- 数据库层已对 `battle_rooms` / `battle_runs` / `battle_room_members` 增加 `BEFORE TRUNCATE` 保护触发器；默认阻断 TRUNCATE。仅当会话显式设置 `SET app.allow_battle_truncate = 'on'` 才可放行（仅用于受控运维窗口）。
