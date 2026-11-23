import { describe, expect, it } from "vitest"
import type { ActionSubTypeDefinition } from "@game-data/game-data"
import { listActionDefinitions } from "@services/action-definition"
import { buildActionPanelTabGroups } from "@services/action-panel-grouping"
import { getGameDataTable } from "@services/game-data-runtime"

describe("action panel presentation groups", () => {
  const definitions = () => getGameDataTable<ActionSubTypeDefinition>("action_sub_types")

  it("shows one cuisine tab with grill and soup sections without changing action subtypes", () => {
    const actions = listActionDefinitions().filter((action) => action.mainType === "cooking")
    const before = structuredClone(actions)
    const tabs = buildActionPanelTabGroups(actions, definitions())
    const food = tabs.find((tab) => tab.id === "fish_cuisine")!
    expect(food.nameKey).toBe("game.fishCuisine.category")
    expect(tabs.filter((tab) => tab.id === "fish_cuisine")).toHaveLength(1)
    expect(tabs.some((tab) => ["cooking_grilled_fish", "cooking_fish_soup"].includes(tab.id))).toBe(false)
    expect(food.sections?.map((section) => [section.id, section.actions.length])).toEqual([
      ["cooking_grilled_fish", 16], ["cooking_fish_soup", 11]
    ])
    expect(tabs.flatMap((tab) => tab.actions).map((action) => action.id).sort()).toEqual(actions.map((action) => action.id).sort())
    expect(actions).toEqual(before)
  })

  it("keeps other menus and undefined categories on their existing flat tab path", () => {
    const actions = listActionDefinitions().filter((action) => action.mainType !== "cooking")
    for (const tab of buildActionPanelTabGroups(actions, definitions())) {
      expect(tab.sections).toBeUndefined()
      expect(tab.actions.every((action) => (action.subType || "common") === tab.id)).toBe(true)
    }
    expect(buildActionPanelTabGroups([], definitions())).toEqual([])
  })
})
