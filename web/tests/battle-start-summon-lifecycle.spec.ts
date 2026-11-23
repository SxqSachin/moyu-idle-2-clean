import { describe, expect, it } from "vitest"
import enGameData from "../src/locales/game-data/en-US"
import zhGameData from "../src/locales/game-data/zh-CN"
import enGame from "../src/locales/game/en-US"
import zhGame from "../src/locales/game/zh-CN"
import { buildBattleHookDescriptions } from "../src/services/battle-hook-display"
import { getEquipmentTemplateDefinition } from "../src/services/item-definition"

type LocaleTree = Record<string, unknown>

function createT(...trees: LocaleTree[]) {
  return (key: string, params: Record<string, unknown> = {}): string => {
    let value: unknown
    for (const tree of trees) {
      value = key.split(".").reduce<unknown>((cursor, part) => {
        if (!cursor || typeof cursor !== "object" || Array.isArray(cursor)) return undefined
        return (cursor as LocaleTree)[part]
      }, tree)
      if (typeof value === "string") break
    }
    if (typeof value !== "string") return key
    return value.replace(/\{([^}]+)\}/g, (_, name: string) => String(params[name] ?? `{${name}}`))
  }
}

const zhT = createT(zhGame as LocaleTree, zhGameData as LocaleTree)
const enT = createT(enGame as LocaleTree, enGameData as LocaleTree)
const recurringSummonHooks = new Map([
  ["rock_golem_silkwood_staff", "rock_golem_staff_on_battle_start"],
  ["berry_birds_beloved_staff", "berry_birds_beloved_staff_battle_start"],
  ["king_of_amusement_park", "king_of_amusement_park_opening_parade"],
])

describe("battle-start summon lifecycle", () => {
  it("shows every-wave timing without a whole-battle trigger cap", () => {
    for (const [itemID, hookID] of recurringSummonHooks) {
      const hooks = getEquipmentTemplateDefinition(itemID)?.battleHooks ?? []
      const zh = buildBattleHookDescriptions(hooks, zhT).find((hook) => hook.key === hookID)
      const en = buildBattleHookDescriptions(hooks, enT).find((hook) => hook.key === hookID)

      expect(zh?.triggerText, itemID).toBe("每波战斗开始时")
      expect(en?.triggerText, itemID).toBe("At the start of every wave")
      expect(zh?.conditions.map((condition) => condition.text).join(" "), itemID).not.toContain("每场战斗")
      expect(en?.conditions.map((condition) => condition.text).join(" "), itemID).not.toContain("Per-battle trigger limit")
    }
  })
})
