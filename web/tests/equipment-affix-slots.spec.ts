import { describe, expect, it } from "vitest"
import {
  getEmptyAffixSlotIndexes,
  getFilledAffixSlots,
  getOccupiedAffixSlotIndexSet,
  hasAffixCapacity,
  hasAnyAffix,
  isAffixFull
} from "../src/services/equipment-affix-slots"

describe("equipment affix slot helpers", () => {
  it("treats non-positive slot count as no capacity", () => {
    expect(hasAffixCapacity(0)).toBe(false)
    expect(hasAffixCapacity(-1)).toBe(false)
    expect(getEmptyAffixSlotIndexes([], 0)).toEqual([])
  })

  it("ignores invalid or empty affix entries", () => {
    const occupied = getOccupiedAffixSlotIndexSet(
      [
        { slotIndex: 0, affixId: "affix.alpha", rarityId: "rare" },
        { slotIndex: 0, affixId: "affix.beta", rarityId: "epic" },
        { slotIndex: 1, affixId: "", rarityId: "" },
        { slotIndex: -1, affixId: "affix.invalid", rarityId: "rare" },
        { slotIndex: 9, affixId: "affix.invalid", rarityId: "rare" }
      ],
      3
    )
    expect(Array.from(occupied).sort((a, b) => a - b)).toEqual([0])
    expect(getFilledAffixSlots([{ slotIndex: 0, affixId: "affix.alpha", rarityId: "rare" }], 3).length).toBe(1)
  })

  it("derives empty/full flags correctly", () => {
    const slots = [
      { slotIndex: 0, affixId: "affix.alpha", rarityId: "rare" },
      { slotIndex: 1, affixId: "affix.beta", rarityId: "epic" }
    ]
    expect(getEmptyAffixSlotIndexes(slots, 3)).toEqual([2])
    expect(hasAnyAffix(slots, 3)).toBe(true)
    expect(isAffixFull(slots, 3)).toBe(false)
    expect(isAffixFull(slots, 2)).toBe(true)
  })
})
