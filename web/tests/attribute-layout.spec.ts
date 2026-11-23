import { describe, expect, it } from "vitest"
import {
  buildCharacterAttributeGroups,
  buildLayoutColumns,
  getConfiguredColumnsForScene,
  getScenePreviewLimit,
  sortAttributeKeysForScene
} from "../src/services/attribute-layout"
import { PLAYER_FISHING_SLOT_IDS, PLAYER_WORK_SLOT_IDS } from "../src/services/specialization"

describe("attribute layout", () => {
  it("sorts battle attributes by configured scene order", () => {
    expect(
      sortAttributeKeysForScene("battle_unit_card", ["crit_damage", "hp", "speed", "strength", "hp"], "battle")
    ).toEqual(["strength", "hp", "speed", "crit_damage"])
  })

  it("keeps work attributes in configured scene order", () => {
    expect(
      sortAttributeKeysForScene("character_stats", ["unknown_work_attr", "mining", "cooking_speed", "cooking"], "work")
    ).toEqual(["mining", "cooking", "cooking_speed"])
  })

  it("does not render keys outside configured layout", () => {
    expect(sortAttributeKeysForScene("enemy_quick_ref", ["hp", "unknown_attr", "crit_damage"], "battle")).toEqual([
      "hp",
      "crit_damage"
    ])
  })

  it("reads preview limits from shared config", () => {
    expect(getScenePreviewLimit("battle_unit_card", 6)).toBe(8)
    expect(getScenePreviewLimit("enemy_quick_ref", 6)).toBe(14)
  })

  it("supports multi-column config and placeholder slots", () => {
    const columns = getConfiguredColumnsForScene("battle_unit_card", "battle")
    expect(columns.length).toBe(2)
    expect(columns.flat()).toContain("")

    const layout = buildLayoutColumns(
      "battle_unit_card",
      "battle",
      ["strength", "dexterity", "intelligence", "hp", "physics_damage"].map((key) => ({ key }))
    )
    expect(layout.length).toBe(2)
    expect(layout[0]?.some((cell) => cell.placeholder)).toBe(true)
  })

  it("builds character groups with growth buckets", () => {
    const groups = buildCharacterAttributeGroups([
      { key: "mining", category: "work", growthType: "levelable" } as never,
      { key: "cooking_speed", category: "work", growthType: "static" } as never,
      { key: "strength", category: "battle", growthType: "static" } as never,
      { key: "hp", category: "battle", growthType: "static" } as never
    ])

    expect(groups.map((group) => group.key)).toEqual(["battle", "work"])
    expect(groups[0]?.sections[0]?.rows.map((row) => row.key)).toEqual(["strength"])
    expect(groups[0]?.sections[1]?.rows.map((row) => row.key)).toEqual(["hp"])
    expect(groups[1]?.sections[0]?.rows.map((row) => row.key)).toEqual(["mining"])
    expect(groups[1]?.sections[1]?.rows.map((row) => row.key)).toEqual(["cooking_speed"])
  })

  it("keeps fishing stats together instead of placing them under other", () => {
    const groups = buildCharacterAttributeGroups([
      { key: "fishing", category: "work", growthType: "levelable", order: 53 } as never,
      { key: "fishing_control", category: "work", growthType: "static", order: 54 } as never,
      { key: "fishing_efficiency", category: "work", growthType: "static", order: 56 } as never,
      { key: "fishing_casting", category: "work", growthType: "static", order: 58 } as never,
      { key: "unknown_work_attr", category: "work", growthType: "static", order: 99 } as never
    ])
    const work = groups.find((group) => group.key === "work")
    const fishing = work?.sections.find((section) => section.key === "fishing")
    const other = work?.sections.find((section) => section.key === "other")

    expect(work?.sections[0]?.rows.map((row) => row.key)).toContain("fishing")
    expect(fishing?.titleKey).toBe("game.attrWorkFishingGroup")
    expect(fishing?.rows.map((row) => row.key)).toEqual([
      "fishing_control",
      "fishing_efficiency",
      "fishing_casting"
    ])
    expect(other?.rows.map((row) => row.key)).toEqual(["unknown_work_attr"])
  })

  it("keeps permanent fishing gear out of the life-tool tab", () => {
    expect(PLAYER_FISHING_SLOT_IDS).toEqual(["fishing_rod", "fishing_reel"])
    expect(PLAYER_WORK_SLOT_IDS).not.toContain("fishing_rod")
    expect(PLAYER_WORK_SLOT_IDS).not.toContain("fishing_reel")
  })
})
