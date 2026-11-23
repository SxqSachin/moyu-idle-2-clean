import { describe, expect, it } from "vitest"
import {
  aggregateEquipmentAttrChanges,
  buildAffixSlotSummary,
  formatSignedNumber,
  resolveAffixMaxEffectiveLimits
} from "../src/services/equipment-affix-display"

describe("equipment affix display helpers", () => {
  it("builds slot summary with empty placeholders", () => {
    const summary = buildAffixSlotSummary(3, [
      { slotIndex: 1, affixId: "affix.work_speed_minor", rarityId: "rare" }
    ])
    expect(summary).toEqual([
      { slotIndex: 0, affixId: "", rarityId: "", effectType: "" },
      { slotIndex: 1, affixId: "affix.work_speed_minor", rarityId: "rare" },
      { slotIndex: 2, affixId: "", rarityId: "", effectType: "" }
    ])
  })

  it("aggregates base attrs with affix attr mods", () => {
    const attrs = aggregateEquipmentAttrChanges(
      { strength: 2, dexterity: 1 },
      [
        { slotIndex: 0, affixId: "affix.strength_minor", rarityId: "common", attrMods: { strength: 3 } },
        { slotIndex: 1, affixId: "affix.dexterity_minor", rarityId: "rare", attrMods: { dexterity: 2 } }
      ]
    )
    expect(attrs).toEqual({
      strength: 5,
      dexterity: 3
    })
  })

  it("formats signed numbers for display", () => {
    expect(formatSignedNumber(3)).toBe("+3")
    expect(formatSignedNumber(-2.5)).toBe("-2.5")
  })

  it("resolves affix max-effective limits", () => {
    expect(resolveAffixMaxEffectiveLimits({
      maxEffectivePerEquipment: 2,
      maxEffectivePerBody: 5
    })).toEqual({
      maxEffectivePerEquipment: 2,
      maxEffectivePerBody: 5
    })
    expect(resolveAffixMaxEffectiveLimits({
      maxEffectivePerEquipment: 0,
      maxEffectivePerBody: -1
    })).toEqual({
      maxEffectivePerEquipment: null,
      maxEffectivePerBody: null
    })
  })
})
