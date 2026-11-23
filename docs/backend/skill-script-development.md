# 技能脚本开发指南（server/internal/battle/skills）

## TL;DR

- 常规技能优先用 `combat.xlsx.skills.effects` 配置驱动；`targeting` 仅保留 legacy 兼容。
- 复杂到 DSL 无法覆盖的目标规则或特殊结算，才写 Go 技能脚本。
- 已覆盖到模板的技能应从 `server/internal/battle/skills/*.go` 移除，避免双轨维护。
- 技能脚本统一放在 `server/internal/battle/skills/*.go`，通过 `Register("<skill_id>", fn)` 绑定。
- 优先复用工具方法：`DefaultAttack`、`BuildDefaultAttackEffect`、`Runtime.SelectTargets/ApplyDamage`。

## 何时需要写脚本

以下情况建议写脚本：

1. 目标规则超出通用 DSL（例如“前中后排各 1”、“前排 1 + 中排 1”）。
2. 伤害公式需要动态改写（如基于某属性临时覆写攻击值）。
3. 需要在一次技能里混合多个效果（伤害 + 资源 + buff + 召唤）。

仅“随机 N 个目标 / 前排全体 / 后排单体”等规则，优先用 Excel 配置，不必写脚本。

## 代码结构

- 注册中心：`server/internal/battle/skills/registry.go`
- 技能脚本目录：`server/internal/battle/skills/`
- 运行时调用入口：`httpapi/battle_runtime_engine.go -> executeBattleSkill`

新增脚本文件后，只要在 `init()` 里调用 `Register`，会自动生效。

## SkillContext 核心字段

- `ctx.Actor`：施法者（含 `Stats/HP/MP/Position/Side`）
- `ctx.Skill`：技能运行态（含 `Effects` 与基础消耗、冷却、等级信息；`Targeting` 仅 legacy）
- `ctx.Targets`：按通用规则预选中的目标
- `ctx.Runtime`：可读写战斗状态的接口（选目标、应用效果）
- `ctx.TargetRolls`：目标随机数（用于稳定重放）

## 常用工具方法

### 1) `DefaultAttack(ctx)`

直接按当前技能配置执行默认攻击，适合绝大多数纯伤害技能。

### 2) `BuildDefaultAttackEffect(ctx, target, damageType, coeff, rand)`

构造单目标伤害效果，常用于“先自定义选目标，再逐个结算”。

### 3) `ctx.Runtime.SelectTargets(query)`

按 `Side/AliveOnly/SortBy/SortOrder/Limit` 查询目标，适合“最高血/最低血”等规则。

- `query.Side="enemy"` 时会受运行时状态语义影响（如 `treat_all_as_enemy` 可把候选扩展到全体）。
- `query.IgnoreUntargetable=true` 可在该次查询中忽略“不可被敌方点选（如潜行）”过滤。

### 4) `ctx.Runtime.ApplyDamage(action)`

把伤害写回运行时（会落日志、更新目标状态），推荐优先使用。

### 5) `resolveTargetRoll(ctx, target.UnitID)`

从 `ctx.TargetRolls` 读取该目标随机数，保证回放一致性。

## 示例 1：前中后各 1（贯穿飞弹）

实现文件参考：`server/internal/battle/skills/row_strike.go`

核心思路：

1. 从 `ctx.Runtime.Memory()` 获取敌方单位快照。
2. 按 `vanguard -> midguard -> rearguard` 各选 1 个存活目标。
3. 对每个目标调用 `BuildDefaultAttackEffect` + `Runtime.ApplyDamage`。

## 示例 2：前排 1 + 中排 1（刺击）

实现文件参考：`server/internal/battle/skills/row_strike.go`

和示例 1 类似，只是排位列表改为 `vanguard -> midguard`。

## Excel 与脚本协作约定

- Excel 里 `skills.id` 必须与 `Register("<skill_id>", ...)` 一致。
- `targeting.targetCount` 用于可成长目标数：`base + floor(Σ(...))`。
- `effects[].amountFormula` 用于伤害/治疗数值：`baseAdd + floor(Σ(actor.stats[attrKey] * (coefficient + perSkillLevel*(level-1))))`。
- `effects[].statusPayload` 用于状态施加：可一次配置多个 `statusId`；持续回合与施加概率都支持基础值、等级成长和属性公式。未配置 `applyChance*` 时保持兼容，必定施加。
- `effects[].requiredCasterStatuses/requiredTargetStatuses` 表达“有这些状态才执行”，`forbiddenCasterStatuses/forbiddenTargetStatuses` 表达“有这些状态则不执行”。技能炼成的判断素材会把“满足/否则”两条连线分别编译成这两组字段。
- `targeting.selectRule` 额外支持 `hp_percent_min/hp_percent_max/first/boss_first/self`，`targeting.targetScope` 额外支持 `self`；`targeting.excludeSelf` 可从友军或全体候选中排除施法者，`targeting.fallbackSelfIfEmpty` 可用于单人技能的自回退。
- 技能炼成图从同一来源节点分出多个效果分支；空白技能默认只有 1 个效果，高稀有度 `effect_slot` 素材每份再解锁 1 个，且受 `global-config.json.skillForge.maxEffects` 统一限制（当前为 3）。玩家不直接创建效果节点，拖入效果槽素材后由编辑器补齐分支。
- 每条分支的效果类型由首个有类型的素材确定，随后只接受同类素材。治疗公式仅允许技能等级、智力和感知，其中属性素材只允许 `intelligence/perception`；伤害、治疗、状态和生命资源变化不能混接。编译后严格按 `effectIndex` 顺序执行。
- `skill_level`、`current_hp/current_mp`、`max_hp/max_mp` 是公式求值器提供的只读虚拟属性。当前 `self_current_hp_loss` 会编译为指向自己的负数 HP `resource` 效果，不经过伤害减免链。
- `adjust_skill_cooldown` 使用有符号 `amount` 调整目标技能剩余冷却，可用 `includeBasic/skillIds/excludeSkillIds/skillCategories` 控制范围，并以 `selectionMode=all|max_remaining` 和 `maxSkills` 控制选择数量；负数最低截断到 0。`skillCategories` 直接引用现有技能分类 ID。旧 Buff 效果 `add_skill_cooldown` 保持正数语义。
- 装备钩子的 `transfer_skill_cooldown` 复用相同技能选择器，且必须配置为原子转移：`from` 排除本次技能并选取另一项正在冷却的技能，`to.selectionMode=context_skill` 指向本次主动技能。没有合法来源技能时不能给本次技能额外增加冷却；可选觉醒 Buff 也只在成功转移后消费。
- 判断素材从来源的“条件”端口接入，再用“满足/否则”端口接到不同效果，服务端按实际目标状态过滤每个分支。
- 启用限制（`statusRequirement/weaponRequirement`）在服务端配置阶段校验，脚本不再重复校验。

## 绑定技能书优化

- 技能炼成的空白草稿只用于编辑、自动保存和权威预览，`forge` 接口会返回 `BLANK_DRAFT_PREVIEW_ONLY`，不能产出技能或物品。
- 可提交的设计必须来自账号库存中的静态官方技能书。客户端只提交物品 ID、集中配置允许的数量、素材模块与连线；来源技能、定义哈希和基础 DSL 均由服务端从物品配置推导。
- 技能书投入位于 `global-config.json.skillForge`：所有环境默认 `200-1000`、步长 `100`。投入 200 本时基础成功率为 `30%`，此后每多 100 本增加 `10` 个百分点；最终成功率仍会叠加每批素材的正负修正并截断在 `0%-100%`。数据库只校验非负，最终范围始终由服务端配置判定。
- 技能书数量不改变预算或节点参数。每一批数值或机制素材都会降低成功率，提高 HP/MP 消耗、冷却或自损的取舍素材则按批提高成功率；成功全额扣除，失败时每种投入返还 `floor(数量/2)`，并保存不可变的失败 revision、掷点及消耗明细。
- 成功 revision 接纳后铸造一件账号绑定、不可堆叠的 `forged_skillbook` 实例，不直接写入角色技能。完整 DSL 仅保存在 revision，物品读模型只暴露受控摘要。
- 改造书使用时只能选择玩家本人或一只所属猫咪，消费后授予独立 `custom:<uuid>`，固定从等级 1、经验 0 开始。未使用的书不进入玩家或猫咪技能目录；书和已学技能都不能再次作为优化来源。
- 同一战斗单位可以拥有官方技能及其多个衍生版本，但装配与规则中只能同时启用一个同源技能。战斗开始时仍把解析后的完整 DSL 写入单位状态和战斗快照，之后销毁、归档或普通定义更新不会改写已开始战斗；只有显式禁用整个规则版本的紧急开关会让运行时安全跳过该技能。
- 每个不可变 revision 保存独立的 `ruleset_version`。发布新规则时先提升 `skillForge.rulesetVersion`，再把旧版本写入 `disabledRulesetVersions`；被禁用版本的成品书不能学习，已学技能不能装配、读取或在战斗中执行，但资产和 revision 仍保留用于审计与永久销毁。已有数据库通过 `20260815_skill_forge_ruleset_version.sql` 回填为 V1。

## 新增技能最小清单

1. `combat.xlsx.skills` 增加技能行（使用 `targeting/effects`，并可选填写 `icon`）。
2. （可选）新增脚本文件并 `Register`。
3. 补齐 i18n：`skill.<id>.name/desc`。
4. 运行 `node packages/game-data/scripts/build.mjs` 生成产物。
