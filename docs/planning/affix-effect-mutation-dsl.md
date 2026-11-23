# Affix Effect Mutation DSL（可直接复制）

本文定义 `affix_definitions.effectParams.effectMutations` 的首版结构，用于让 affix 影响武器 `basicAttackEffects` 与技能 `effects`。

## 0. 绑定约束（先看）

1. 只要使用 `effectParams.effectMutations`，`affix_definitions.effectType` 必须为 `effectMutation`。
2. 当 `effectType=effectMutation` 时，`effectParams` 只允许包含 `effectMutations`，不允许与 `attrMods` / `battleHooks` 等其他参数混写。
3. 唯N限制不写在 `effectParams`，而是写在 `affix_definitions` 顶层字段：
   - `maxEffectivePerEquipment`（装备唯N，`0`=不限制）
   - `maxEffectivePerBody`（全身唯N，`0`=不限制）
4. 历史超限数据在运行时只生效前 N 个（按 `slotIndex` 升序）。

## 1. 结构

```json
{
  "effectMutations": [
    {
      "scope": "basic_attack|skill|both",
      "skillIds": ["optional_skill_id"],
      "effectSelector": {
        "mode": "all|index",
        "indexes": [1, 2]
      },
      "ops": {
        "targetCountDelta": 1,
        "amountBaseAddDelta": 10,
        "amountTermsAppend": [
          { "attrKey": "intelligence", "coefficient": 0.3, "perSkillLevel": 0 }
        ],
        "statusIdsAppend": ["bleed"],
        "statusIdsRemove": ["old_status"],
        "appendEffects": []
      },
      "constraints": {
        "targetCountMin": 1
      },
      "scriptRef": "future:custom-script-id"
    }
  ]
}
```

## 2. 规则

1. `indexes` 是 1-based。
2. 多 affix 叠加按槽位顺序应用。
3. 数值类增量相加，列表类去重合并。
4. 目标数最终会按 `constraints` clamp，且不会低于 `1`。
5. `scriptRef` 本期仅作为预留字段，不执行。

## 3. 你提出的案例（可直接复制）

### A. 给武器 effect1 增加 1 个额外目标

```json
{
  "effectMutations": [
    {
      "scope": "basic_attack",
      "effectSelector": { "mode": "index", "indexes": [1] },
      "ops": { "targetCountDelta": 1 },
      "constraints": { "targetCountMin": 1 }
    }
  ]
}
```

### B. 给技能 effect1 增加 10 点伤害，并把目标数 -1（但不低于 1）

```json
{
  "effectMutations": [
    {
      "scope": "skill",
      "skillIds": ["fireball"],
      "effectSelector": { "mode": "index", "indexes": [1] },
      "ops": {
        "amountBaseAddDelta": 10,
        "targetCountDelta": -1
      },
      "constraints": { "targetCountMin": 1 }
    }
  ]
}
```

### C. 对所有 effect 追加状态（all）

```json
{
  "effectMutations": [
    {
      "scope": "both",
      "effectSelector": { "mode": "all" },
      "ops": {
        "statusIdsAppend": ["bleed"]
      }
    }
  ]
}
```

### D. 用公式增加目标数 / 伤害

```json
{
  "effectMutations": [
    {
      "scope": "skill",
      "effectSelector": { "mode": "index", "indexes": [1] },
      "ops": {
        "targetCountDelta": {
          "baseAdd": 0,
          "terms": [{ "attrKey": "intelligence", "coefficient": 0.02, "perSkillLevel": 0 }]
        },
        "amountBaseAddDelta": {
          "baseAdd": 0,
          "terms": [{ "attrKey": "magical_damage", "coefficient": 0.15, "perSkillLevel": 0 }]
        }
      },
      "constraints": { "targetCountMin": 1 }
    }
  ]
}
```
