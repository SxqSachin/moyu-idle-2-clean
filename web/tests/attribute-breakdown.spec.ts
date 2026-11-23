import { describe, expect, it } from "vitest"
import { buildAttributeBreakdownLines } from "../src/services/attribute-breakdown"

const i18nMap: Record<string, string> = {
  "item.wood_sword.name": "长剑",
  "affix.speed_rune.name": "速度符文",
  "attr.dexterity.name": "敏捷",
  "game.attrSourceEquipmentBase": "装备",
  "game.attrSourceFromAffix": "来自affix"
}

function t(key: string): string {
  return i18nMap[key] || key
}

describe("attribute breakdown renderer", () => {
  it("renders equipment + affix + formula lines", () => {
    const lines = buildAttributeBreakdownLines(
      "dexterity",
      {
        sources: {
          version: "v2",
          equipment: [
            {
              templateItemId: "wood_sword",
              total: 1.5,
              base: 1,
              affixes: [{ affixId: "speed_rune", value: 0.5 }]
            }
          ],
          formula: {
            expression: "dexterity*1",
            sources: [
              {
                attrKey: "dexterity",
                value: 2,
                sourceBreakdown: {
                  base: { value: 1 },
                  equipment: [
                    {
                      templateItemId: "wood_sword",
                      total: 1,
                      base: 0,
                      affixes: [{ affixId: "speed_rune", value: 1 }]
                    }
                  ]
                }
              }
            ]
          }
        }
      },
      t
    )

    expect(lines).toEqual([
      "- 长剑: 1.5",
      "  - 装备: 1",
      "  - 速度符文: 0.5 (来自affix)",
      "- 敏捷: 2"
    ])
  })

  it("formats composition lines as percent when target attribute unit is percent", () => {
    const lines = buildAttributeBreakdownLines(
      "crit_chance",
      {
        unit: "percent",
        sources: {
          version: "v2",
          base: {
            baseDefault: 0.015
          },
          formula: {
            expression: "dexterity*0.001",
            sources: [
              {
                attrKey: "dexterity",
                value: 0.0075
              }
            ]
          }
        }
      },
      t
    )

    expect(lines).toEqual([
      "- game.attrSourceBase: 1.5%",
      "- 敏捷: 0.75%"
    ])
  })
})
