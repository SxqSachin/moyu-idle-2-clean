# 游戏 i18n 逻辑说明（全链路）

本文定义游戏数据到前端多语言展示的统一规则，当前以 `zh-CN` 作为基线语言。

## 1. 单一来源与生成链路

1. 策划在 `packages/game-data/source/workbooks/*.xlsx` 维护条目与可选 i18n key。
2. 执行 `node packages/game-data/scripts/build.mjs` 生成 `packages/game-data/generated/game-data.json`。
3. `build.mjs` 会把游戏条目 key 结构自动同步到：
   - `web/src/locales/zh-CN.ts`
   - `web/src/locales/en-US.ts`
4. 前端统一从 `web/src/i18n.ts` 读取语言注册表（当前支持 `zh-CN/zh-TW/en-US/ja/ko/ru`），所有语言选择入口使用同一来源，不再各自硬编码选项。
5. 运行时加载策略：`zh-CN/en-US` 随首包加载，其余语言包按需动态加载（切换时再拉取对应 chunk）。

## 2. 统一命名规范（必须）

- 根节点统一为：
  - `item`
  - `action`
  - `action_main_type`
  - `action_sub_type`
  - `item_type`
  - `skill`
  - `skill_type`
  - `weapon_class`
  - `attr`
  - `rarity`
- 统一使用下划线命名，不使用 `actionMainType`、`items`、`actions` 等并行旧命名。

默认 key 规则（Excel 未填写时自动生成）：

- 物品：
  - `item.{id}.name`
  - `item.{id}.desc`
  - `item.{id}.effect`
- 动作：
  - `action.{id}.name`
  - `action.{id}.desc`
- 动作主类：
  - `action_main_type.{id}.name`
  - `action_main_type.{id}.desc`
- 动作子类：
  - `action_sub_type.{id}.name`
  - `action_sub_type.{id}.desc`
- 物品分类（来自 `item_types`）：
  - `item_type.{id}.name`
  - `item_type.{id}.desc`
- 稀有度（来自 `rarity_definitions`）：
  - `rarity.{id}.name`
- 战斗敌人与战斗区域（来自 `enemies` / `world`）：
  - `enemy.{id}.name`
  - `enemy.{id}.desc`
  - `world.{id}.name`
  - `world.{id}.desc`
- 技能与技能类型（来自 `skills` / `skill_type`）：
  - `skill.{id}.name`
  - `skill.{id}.desc`
  - `skill_type.{id}.name`
  - `skill_type.{id}.desc`
- 武器类型（来自 `weapon_classes`）：
  - `weapon_class.{id}.name`
  - `weapon_class.{id}.desc`

## 3. zh-CN 基线策略

- `zh-CN.ts` 是结构基线：所有 key 必须先在这里存在（值可先为空字符串）。
- 其他语言（如 `zh-TW.ts`、`en-US.ts`、`ja.ts`、`ko.ts`、`ru.ts`）从 `zh-CN` 基线翻译扩展，保持 key 结构一致。
- 空字符串视为“待翻译”，前端可回退显示可读默认值，避免 UI 空白。

## 4. 语言初始化与切换规则

- 首次访问且本地无 `locale` 时，按浏览器语言初始化：
  - `zh-TW/zh-HK/zh-MO -> zh-TW`
  - `zh* -> zh-CN`
  - `en* -> en-US`
  - `ja* -> ja`
  - `ko* -> ko`
  - `ru* -> ru`
- 一旦用户手动选择语言，或已经存在本地/远端用户设置，后续不再被浏览器语言覆盖。
- 登录页、首页、游戏设置页等语言选择入口统一读取 i18n 注册表，新增语言后会自动出现在这些入口。

## 5. 迁移后约束

- 构建脚本与前端代码统一使用 `action_main_type.*`、`action_sub_type.*`、`item_type.*`、`rarity.*`。
- locale 结构统一为 `item/action/action_main_type/action_sub_type/item_type/skill/skill_type/attr/rarity`，不再依赖旧节点。
- 战斗链路新增 `enemy/world` i18n 节点，战斗展示统一走 `nameKey/descKey`。
- 技能链路新增 `skill/skill_type` i18n 节点，技能与类型文案统一走 key。
- 武器类型链路新增 `weapon_class` i18n 节点，武器类别文案统一走 key。
- 新增开发与数据维护一律使用新命名，保持全链路一致。

## 6. AI 批量补全文案建议流程

1. 先跑一次 `build.mjs`，确保最新 key 已同步到 locale。
2. 从 `zh-CN.ts` 中筛选空值 key，批量交给 AI 生成中文。
3. 以 `zh-CN` 为语义基线生成 `en-US` 对应翻译。
4. 合并后再次执行 `build.mjs`，确认结构无回退与冲突。

## 7. 变更检查清单

- Excel 模板字段是否保持 `nameKey/descKey/effectDescKey` 约定。
- `build.mjs` 默认 key 是否仍符合 `item/action/action_main_type/action_sub_type/item_type/rarity`。
- `build.mjs` 默认 key 是否仍符合 `item/action/action_main_type/action_sub_type/item_type/skill/skill_type/rarity`。
- `web/src/locales/*` 是否只保留统一根节点。
- 前端 `t(...)` 调用是否只引用统一前缀。
