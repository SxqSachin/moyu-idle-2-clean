# 全局操作效果基座

当前料理内容、差异化食材权重、特殊变异、生命周期覆盖与新业务接线见 [鱼料理 V2](fish-cuisine-v2.md)。下文保留 V1 基座背景；与 V2 有差异的料理数值和普通动作适用范围以 V2 为准。

## 目标与边界

这套基座承载“食用后影响后续操作”的全局效果，但不把效果逻辑散落到队列、离线快进、HTTP 或 UI 中。

核心边界如下：

- 效果在每个实际操作开始时判定，不在队列提交时判定。
- 已经开始的操作使用冻结快照；之后新增、过期或消耗的效果都不能改变它。
- 时间型效果采用半开区间 `[validFrom, validUntil)`，按服务器现实时间流逝，离线也会过期。
- 次数型效果表示“接下来 N 次符合条件的操作”。只要操作成功开始就消耗一次，与概率分支是否命中无关。
- 普通动作效果的 `life_action` 域只对应通用 `Action` 执行器，不包含强化、炼金、钓鱼或建造。
- 业务结算仍由各业务拥有。基座只负责选中规则、冻结快照和可靠消费次数，不直接修改任意业务结果。

当前已注册的规则包括：

- `life_action.material_refund_chance`
- `life_action.extra_main_output_chance`
- `alchemy.infusion_capacity_floor_tradeoff`
- `alchemy.infusion_precision_ceiling_bonus`

首批内容由两道鱼料理启用：次数型的苔火双鲜烤鱼，以及时间型的星露银鳞鱼汤。不相关动作没有匹配规则时仍保持原行为，也不会为了效果系统写入空 payload。

## 分层结构

### 1. 内容定义层

权威数据位于 `packages/game-data/source/json/operation_effects.json`。定义只描述稳定配置：

```json
{
  "id": "food.effect.example",
  "rulesetVersion": "example-v1",
  "domain": "life_action",
  "kind": "life_action.material_refund_chance",
  "nameKey": "operation_effect.food.effect.example.name",
  "descKey": "operation_effect.food.effect.example.desc",
  "iconId": "item.example_food",
  "priority": 100,
  "allowedTargets": ["player", "kitty"],
  "lifetime": { "kind": "charges", "charges": 10 },
  "stacking": { "key": "life_action.material_refund", "mode": "add_charges", "maxCharges": 50 },
  "selector": { "mainTypes": ["gather"] },
  "params": { "chance": 0.5, "refundRate": 1.0 }
}
```

物品通过 `useOperationEffectId` 引用定义，并且必须是 `food` 类型。构建脚本同时校验：

- ID、版本、域、规则种类、目标、生命周期和叠加方式；
- selector 引用的动作与分类；
- 每种规则自己的参数结构和数值范围；
- 物品引用存在且类型正确。

运行时不会接受未经注册的 `kind`。新增 JSON 但忘记实现处理器会在构建或服务启动阶段失败，而不是静默失效。

`kind` 是长期兼容的语义契约，不能在原标识下重新解释旧参数。参数结构或结算含义不兼容时必须新增 `kind`；旧处理器至少保留到对应授权和运行快照全部退出，确保已经冻结的 `rulesetVersion` 仍可结算。定义的生命周期、目标、selector、叠加或 params 发生任何行为变化时都必须提升 `rulesetVersion`，不能原地改写一个已经发放过的版本。

### 2. 通用生命周期内核

`server/internal/operationeffect` 只认识五个概念：

- `Definition`：版本化规则定义；
- `Grant`：玩家持有的一次效果授权；
- `Operation`：某个业务即将开始的原子操作；
- `PreparedOperation`：尚未提交的匹配结果；
- `Snapshot`：操作开始后不可变的规则快照。

业务以强类型 `RuleHandler` 注册匹配能力：

```go
type RuleHandler interface {
    Domain() Domain
    ValidateParams(json.RawMessage) error
    Applies(Operation, json.RawMessage) bool
}
```

这里刻意没有通用 `map[string]any` 事件总线，也没有“统一修改任意结果”的接口。每个域自己定义参数和输出补丁，避免一个新效果破坏其他结算。

### 3. 业务适配层

普通动作适配器位于 `server/internal/operationeffect/lifeaction`。它接收冻结的 `Snapshot` 和普通动作输入，返回纯结果补丁：

- 应返还的模板素材；
- 应追加的主产物。

适配器不访问数据库、不管理队列、不直接发物品。实际入账仍走库存 ledger，并保留效果定义、授权、队列和操作 key 的审计信息。

这使职责保持单向：

```text
定义与授权 -> 开始时匹配/冻结 -> 业务纯计算 -> 原业务事务内记账
```

炼金适配器位于 `server/internal/operationeffect/alchemy`，只匹配玩家的普通材料灌注 `alchemy.bench`，不会匹配转换、转化、继承或鱼类灌注：

- `alchemy.infusion_capacity_floor_tradeoff` 参数为 `extraMaterialQuantity`（固定数量）和 `positiveFloorReductionRate`（0 到 1 的比例）；
- `alchemy.infusion_precision_ceiling_bonus` 参数为 `positiveCeilingBonusRate`（在基础 75% 正向上限上增加的百分点比例，最终不超过 100%）。

多个实际命中的同类效果会相加并在处理器硬上限处截断。每份被扣次数的授权都真实参与计算，不会出现“扣了次数但因隐式择优而没有贡献”的情况。容量效果不会改变材料的 `unitBasis` 步长；服务端仍要求输入数量满足原材料步长，只提高允许上限。下限惩罚只作用于全正数范围，不会放大负面词条。

这两种炼金效果只允许配置为 `player` 目标，并且 action selector 若存在只能引用 `alchemy.bench`。构建器和服务启动会双重拒绝“可以吃、但永远匹配不到炼金动作”的死配置；这项限制只属于这两个处理器，不限制未来其他炼金效果面向猫咪。

## 鱼料理制作与品质快照

鱼料理配方的权威表是 `packages/game-data/source/json/fish_food_recipes.json`。它只声明动作、产物、料理类别、品质规则版本，以及每种鱼的物种和数量。普通战利品与食物材料仍放在动作 `costs` 中，鱼实例不会伪装成同一种模板素材数量。

每份配方至少需要两种不同鱼类。烤鱼只能引用次数型效果，鱼汤只能引用时间型效果；构建脚本同时验证动作分类、产物类型、市场配置、普通材料和效果生命周期，错误组合不能进入生成数据。

制作有两个明确模式：

- `automatic`：每次实际开始时从未归档的背包渔获中按物种锁定鱼，优先无变异、体型较小的实例；成品使用定义中的标准效果参数。连续制作仍逐份预留，取消只退还当前运行已预留的原鱼实例和普通材料。
- `specific`：请求必须给出配方中每个物种对应的精确实例 ID，且一次只能制作一份。服务端在事务内复验所有权、状态、归档标记、模板、物种和数量，然后才移动到队列预留态。前端预览只是镜像，不能替代服务端计算。

`fish-food-quality-v1` 对每尾鱼计算：物种区间内的体长分位、由体重与 `shapeFactor` 反推的等效体长分位、六项基因均值，以及最多三种变异奖励。每尾倍率限制为 `0.75..1.35`，成品倍率取所有食材倍率的平均值。当前参数映射为：

- 用量上限固定值乘倍率后四舍五入；
- 正向下限降低比例除以倍率，因此更好的食材同时增加收益并减轻代价；
- 正向精度上限比例乘倍率。

指定食材成品在 `base_snapshot` 中固化两段版本化数据：`fishFood` 保存配方版本、品质规则版本、汇总品质和不含来源实例 ID 的食材摘要；`operationEffect` 只允许覆盖同一定义、同一规则版本的 `params`。食用时服务端重新以强类型处理器校验参数，不能借自定义快照替换 kind、生命周期、目标、selector 或叠加策略。

标准成品保留空基础快照，可进入模板商品市场；指定食材成品是独立实例，只能按实例进入拍卖，不能被模板上架流程抹平品质快照。两者都允许交易，公开实例预览会展示指定食材的品质、鱼种、尺寸和最终效果数值，但不会泄露已消耗渔获的实例 ID。

不同强度的同类料理不会被静默合并：若同一目标已有相同定义但参数不同的有效授权，食用返回强度冲突；玩家需要先用完或等待到期。参数完全相同时才按原定义增加次数或延长时间，避免授权账本与实际生效强度脱节。

## 在线与离线使用同一路径

每个队列操作使用稳定 key：

```text
action_queue:<queue_id>:<one_based_ordinal>:<logical_started_at_unix_nano>
```

逻辑开始时间属于 key 的一部分：同一次已持久化运行的重试继续读取原快照；被中断后再次启动同一队列序号则是新的实际操作，会获得新的 key，并按规则再次消耗次数。

队列启动一次操作时：

1. 在用户事务锁内加载候选授权；
2. 以该次操作的逻辑 `startedAt` 调用 `Session.Prepare`；
3. 业务 `StartRunTx` 成功并写入运行中 payload；
4. 把同一快照写进 payload；
5. 最后调用 `Session.Commit` 记录次数消费意图。

任何启动校验失败都会回滚到 savepoint，既没有运行中动作，也不会扣次数。

结算时只读取 payload 中的快照，不重新查询“现在有什么效果”。实际没有命中任何规则时省略该字段，字段缺失就是稳定的空快照语义。因此：

- 运行中途吃东西不会追授效果；
- 开始后时间效果过期不会撤销本次效果；
- 旧版本运行中动作没有快照时按空快照处理，不会在升级后意外获益。

离线快进仍按每个逻辑开始时间创建并冻结快照。时间型效果、玩家独享次数和指定猫咪独享次数通常可以在各自单一执行通道内批量计算；只有 `all_kitties` 或 `player_and_kitties` 这类跨执行通道共享的次数池会退化为按全局时间逐次交错推进。炼金容量会决定下一次灌注能否合法开始，因此仅在存在相关炼金授权或当前运行带有炼金快照时按实际开始逐次推进；无效果的炼金队列继续走原批量快进。授权仍只在整次事务末尾批量写回，不会形成每一步一次授权数据库查询。

普通在线结算发生延迟时，下一次操作以真正写入 `running` 的结算时刻作为开始时间；离线快进则保留历史时间线，以上一操作的逻辑完成时刻作为下一次开始时间。两者不会混用“计划结束时间”和“实际开始时间”。炼金逐次推进还会在开始时完成容量校验、目标校验、素材预留和效果次数提交；任一步失败都不会扣下一次效果。

普通模板奖励会按授权、定义和物品聚合后一次入账，同时记录命中次数及首尾 operation key，避免长时间离线产生逐动作数据库写入。需要逐件生成随机词条的主产物仍按各自 operation key 独立派生随机数并入账，以保持分批快进和在线执行结果一致。

炼金产物会在每个 `alchemyComponents` 成分内固化本次实际参与计算的效果定义 ID、版本、kind 与参数，但不会写入玩家的 grant ID。竞技场校验只对当前炼金规则版本、结构合法且仍能通过对应强类型处理器校验的固化来源放宽容量与正向上限；缺失、损坏或越过硬上限的来源一律按原上限校验。继承时证明随被保留的成分流转，因此不会把某个父项的食物效果粗略套给整件产物。

## 一致性与幂等

数据库分为四类状态：

- `player_operation_effect_state`：玩家级单调 revision；
- `player_operation_effect_grants`：冻结定义、目标、时间或剩余次数；
- `player_operation_effect_activations`：食用请求幂等账本；
- `player_operation_effect_charge_receipts`：`(grant_id, operation_key)` 唯一的次数消费凭证。

食用、库存扣除、授权叠加、激活账本和 revision 更新处于同一事务。动作启动、快照持久化、次数凭证和剩余次数更新也处于同一事务。

向仍有效的旧授权补充次数前，必须在同一用户事务锁内确认该账号没有待结算的历史运行：`running` 且结束时间为空或不晚于食用时间时，返回 `OPERATION_EFFECT_RECOVERY_PENDING`，不消耗料理。检查包含暂停中的执行通道，避免恢复后把新补充的次数追授给过去的操作；无需等待尚未到期的当前操作或整个排队列表完成。新授权有独立的 `validFrom`，不受这项补充限制；时间型授权只延长尚未到期的尾部，也不改变历史生效区间。幂等食用重试仍优先读取原激活凭证，不重复执行补充。

叠加身份由 `definitionId + rulesetVersion + stacking.key + target` 共同确定。不同效果定义即使误用了相同 `stacking.key` 也不会彼此静默覆盖；若未来需要“同组只保留最强效果”，应新增明确的互斥策略，而不是复用叠加字段制造隐含行为。

`independent` 表示每份授权独立存在并独立结算，因此不能配置共享的 `maxCharges` 或 `maxDurationSeconds`；构建与服务启动都会拒绝这种看似有限、实际无法兑现的配置。

次数写回使用唯一凭证后按授权聚合扣减。重复执行相同 operation key 不会重复扣次数；若内存计划和数据库剩余量不一致，事务失败关闭，不会产生负数或部分提交。

概率随机数使用独立命名空间 `operation-effect-v1`，由操作 key、授权 ID、规则 kind 和阶段派生。它不会推进原动作的随机序列，因此启用或移除效果不会改变原本的掉落、词条等随机结果。

离线回归测试分别覆盖普通动作不同分片大小的产出与次数一致性、炼金最后一次冻结效果、强化的逻辑开始时间，以及钓鱼逐竿冻结、过期清除和启动失败不扣次数。`operation_effect_postgres_test.go` 通过 `OPERATION_EFFECT_TEST_DATABASE_URL` 显式启用真实 PostgreSQL 校验，限定回环地址，仅使用事务内临时表并最终回滚，验证历史过期授权读取、补充拦截、凭证去重、次数不足和事务回滚；不运行迁移，也不触碰持久游戏表。

## 目标模型

目标与生命周期彼此独立：

- `player`：只匹配玩家动作；
- `kitty`：只匹配指定猫咪；
- `all_kitties`：匹配该玩家任意猫咪；
- `player_and_kitties`：匹配玩家及其全部猫咪。

次数属于授权而非角色。若一份授权目标是 `all_kitties`，所有猫咪竞争同一个剩余次数；若希望每只猫独立计数，应为每只猫创建独立的 `kitty` 授权，不能在结算时临时拆分含义。

## 新增效果的标准步骤

### 同一业务域内新增规则

1. 在对应域包增加有类型的参数结构、校验器和纯 `Resolve` 分支。
2. 向 registry 注册新的 `kind`。
3. 在 game-data 构建校验中登记该 `kind` 的参数规则。
4. 添加定义和物品引用，重建 game-data。
5. 测试目标匹配、边界时间、次数消费、概率确定性和零副作用路径。

不需要修改队列编排、数据库表、HTTP 食用接口或顶部 UI。

### 新增业务域

1. 定义新的 `Domain` 和独立业务包，例如 `operationeffect/enhancement`。
2. 让该业务包返回它自己的强类型补丁，不复用普通动作产出结构。
3. 找到真正的原子开始点，例如“一次强化尝试”或“一次抛竿”，在那里构造 `Operation`。
4. 在业务现有事务中执行 `Prepare -> 原操作开始成功 -> Commit`，并把快照持久化到该业务自己的运行记录。
5. 在线结算与离线结算都只消费该快照，并使用相同 resolver。
6. 接入 revision 推送并补充在线/离线等价测试。

`domain` 在数据库中只约束稳定标识格式，不枚举业务名称；新增业务域不需要修改效果表结构。

不能只让某个队列执行器实现可选的 `OperationEffectDomain()` 接口就算接入完成。强化、炼金和钓鱼的“队列项”未必等于一次实际尝试；接错边界会造成次数多扣、少扣或离线结果不一致。

## 前端职责

前端只做三件事：

- 展示服务端投影与基于 `serverNow` 推算的剩余现实时间；
- 让玩家从定义允许的目标中选择，并提交幂等食用请求；
- 以 revision 拒绝乱序的 WebSocket 快照。

前端倒计时归零只隐藏时间效果，不负责删除授权或决定结算。服务端始终是唯一权威。

`useOperationEffects()` 是业务组件的唯一读取入口，Pinia store 是唯一内存缓存。composable 负责 bootstrap/WS 快照接入、revision 防乱序、请求去重、恢复可见或重新联网后的刷新、服务端时间推算，以及按 domain/selector/target 查询效果。时间推算同时观察单调时钟和本地墙钟的正向增量：普通运行不受系统校时回拨影响，设备休眠导致单调时钟暂停时也不会把已过期效果继续显示。恢复联网或页面可见后仍以服务端快照校正。

请求失败只把缓存标成 stale，不清空最后一份可用投影；登出时统一 reset。每次请求还绑定当前认证 epoch，登出或切换账号前发出的迟到响应不能重新填回缓存。炼金面板只读取 composable 暴露的强类型 modifier，不解析数据库或队列 payload。

## 发布顺序

迁移是手动部署边界，必须先应用：

```text
server/schema/migrations/20260904_operation_effects.sql
```

然后发布包含本基座、鱼料理执行器和前端的服务端与网页版本，最后启用包含首批料理的 game-data。旧客户端看不到新动作；旧服务端因启动期定义校验也不会静默接受自己无法处理的效果类型。
