# AI 生成 i18n 指南（落地到 `web/src/locales`）

本文用于指导 AI 批量生成物品/动作相关 i18n，并最终写入：

- `web/src/locales/zh-CN.ts`
- `web/src/locales/zh-TW.ts`
- `web/src/locales/en-US.ts`
- `web/src/locales/ja.ts`
- `web/src/locales/ko.ts`
- `web/src/locales/ru.ts`

## 1. 目标

- 对 `web/src/locales/*` 的空值 key 生成可读文案。
- 保持六语言语义对齐（`zh-CN/zh-TW/en-US/ja/ko/ru`）。
- 最终产出必须是可直接合并进 locales 文件的对象片段。
- 统一结构对齐：
  - `item.{id}.name/desc/effect`
  - `action.{id}.name`
  - `action_main_type.{id}.name/desc`
  - `item_type.{id}.name/desc`
  - `rarity.{id}.name`

## 2. 数据来源

1. 数据清单：`packages/game-data/generated/game-data.json` 中 `items[]`、`actions[]`、`action_main_types[]`、`item_types[]`、`rarity_definitions[]`
2. i18n key 结构：`web/src/locales/zh-CN.ts` 及其他语言文件中的 `item/action/action_main_type/item_type/rarity`（由 `build.mjs` 自动同步 key 结构）
3. 现有翻译基线：`web/src/locales/zh-CN.ts`

## 3. 生成范围规则

- 处理 `item.`、`action.`、`action_main_type.`、`item_type.`、`rarity.` 前缀 key。
- 已有高质量人工文案的 key 不覆盖。
- 占位值（值等于 key）优先生成。

## 4. 输出格式要求（必须）

AI 输出必须是六个对象片段（不要解释文字）：

1. `zh-CN` 片段
2. `zh-TW` 片段
3. `en-US` 片段
4. `ja` 片段
5. `ko` 片段
6. `ru` 片段

示例：

```ts
// zh-CN patch
item: {
  wood_sword: {
    name: "木剑",
    desc: "一把基础近战武器，适合新手。",
    effect: "攻击力小幅提升。"
  }
},
action: {
  training_running: { name: "长跑训练" }
},
action_main_type: {
  exercise: { name: "日常锻炼" }
},
item_type: {
  weapon: { name: "武器", desc: "可装备的近战或远程作战道具。" }
},
rarity: {
  legendary: { name: "传世" }
}
```

```ts
// en-US patch
item: {
  wood_sword: {
    name: "Wooden Sword",
    desc: "A basic melee weapon suitable for beginners.",
    effect: "Slightly increases attack power."
  }
},
action: {
  training_running: { name: "Running Drill" }
},
action_main_type: {
  exercise: { name: "Daily Training" }
},
item_type: {
  weapon: { name: "Weapon", desc: "Equipment used for close-range or ranged combat." }
},
rarity: {
  legendary: { name: "Legendary" }
}
```

## 5. 推荐提示词模板（给 AI）

```text
你是本项目 i18n 文案助手。请基于我提供的 item/action/action_main_type/item_type/rarity id 列表，生成六语言文案（zh-CN/zh-TW/en-US/ja/ko/ru）。

要求：
1) 输出六个 TS 对象片段：zh-CN、zh-TW、en-US、ja、ko、ru。
2) key 结构必须使用 item/action/action_main_type/item_type/rarity 五个根节点。
3) 中文自然、简洁，英文地道、简洁；两者语义一致。
4) 不要输出解释文字，不要改动非 item/action/action_main_type/item_type/rarity 域。
5) 对于信息不足的条目，写“中性不误导”的描述。
```

## 6. 落地步骤

1. 从 `web/src/locales/zh-CN.ts`（或 `en-US.ts`）过滤待补齐 key（空字符串值）。
2. 把 item/action/action_main_type/item_type/rarity id 列表喂给 AI，拿到六语言 patch。
3. 合并到 `web/src/locales/*.ts` 的对应节点。
4. 运行前端类型检查与构建，确认无语法问题。

## 7. 文案质量检查清单

- 名称与描述是否匹配物品类型（weapon/armor/material/consumable）。
- 中文是否出现机器直译腔；英文是否有明显语法问题。
- `effect` 是否与玩法语义一致，避免“承诺型”描述（如必定掉落、必定暴击）。
- 同类物品术语是否统一（如“耐久 / Durability”）。
