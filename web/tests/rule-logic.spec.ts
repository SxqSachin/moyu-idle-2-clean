import { describe, expect, it } from "vitest"

import { createDefaultRuleCondition, normalizeRuleGroups } from "@services/rule-logic"

describe("rule logic model", () => {
  it("creates new value conditions with a percent HP threshold", () => {
    expect(createDefaultRuleCondition()).toMatchObject({
      compareType: "value",
      target: "self",
      dataType: "hp",
      valueCompareAction: "<=",
      valueConditionValue: "50%"
    })
  })

  it("keeps legacy field/op/value rules editable without dropping fields", () => {
    const groups = normalizeRuleGroups(
      [
        {
          conditions: [
            {
              field: "self_hp_percent",
              op: "<=",
              value: 70
            }
          ]
        }
      ],
      { allowEmpty: true }
    )

    expect(groups).toEqual([
      {
        conditions: [
          expect.objectContaining({
            compareType: "value",
            target: "self",
            dataType: "self_hp_percent",
            valueCompareAction: "<=",
            valueConditionValue: 70,
            field: "self_hp_percent",
            op: "<=",
            value: 70
          })
        ]
      }
    ])
  })

  it("preserves legacy enemy count row filters", () => {
    const groups = normalizeRuleGroups(
      [
        {
          conditions: [
            {
              compareType: "value",
              target: "enemies",
              dataType: "enemy_count",
              enemyCountPosition: "rearguard",
              valueCompareAction: ">=",
              valueConditionValue: 1
            }
          ]
        }
      ],
      { allowEmpty: true }
    )

    expect(groups[0]?.conditions[0]).toMatchObject({
      compareType: "value",
      target: "enemies",
      dataType: "enemy_count",
      enemyCountPosition: "rearguard",
      valueCompareAction: ">=",
      valueConditionValue: 1
    })
  })
})
