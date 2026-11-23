import { describe, expect, it } from "vitest"
import type { ActionDefinition } from "@game-data/game-data"
import {
  getActionDefinition,
  isGenericActionVisible,
  listActionDefinitions,
  listActionMainTypeDefinitions
} from "@services/action-definition"

describe("special queue action visibility", () => {
  const specialIDs = [
    "fishing.trip", "equipment_enhance", "building_upgrade",
    "alchemy.convert", "alchemy.bench", "alchemy.inherit", "alchemy.transmute", "alchemy.smelt"
  ]

  it.each(specialIDs)("excludes %s from generic actions and quick references", (id) => {
    expect(isGenericActionVisible({ id } as ActionDefinition)).toBe(false)
    expect(isGenericActionVisible({ id: ` ${id} ` } as ActionDefinition)).toBe(false)
    expect(listActionDefinitions().some((action) => action.id === id)).toBe(false)
    expect(listActionMainTypeDefinitions().some((group) => group.actions.includes(id))).toBe(false)
  })

  it("retains fishing metadata for the dedicated page and existing queue cards", () => {
    expect(getActionDefinition("fishing.trip")?.id).toBe("fishing.trip")
    expect(isGenericActionVisible(undefined)).toBe(false)
  })

  it("keeps ordinary production, including fish cuisine, available", () => {
    const actions = listActionDefinitions()
    expect(actions.length).toBeGreaterThan(0)
    expect(actions.some((action) => action.subType === "cooking_grilled_fish")).toBe(true)
    expect(actions.every(isGenericActionVisible)).toBe(true)
  })
})
