import { readFileSync } from "node:fs"
import { resolve } from "node:path"
import { describe, expect, it } from "vitest"
import zhGameData from "../src/locales/game-data/zh-CN"
import zhGame from "../src/locales/game/zh-CN"
import { buildEffectDetailSections } from "../src/services/effect-detail-renderer"

function sourceRows(filename: string, table: string): Array<Record<string, unknown>> {
  const path = resolve(process.cwd(), `../packages/game-data/source/json/${filename}`)
  return JSON.parse(readFileSync(path, "utf8"))[table]
}

function translate(key: string, params: Record<string, unknown> = {}): string {
  let cursor: unknown = zhGame
  for (const part of key.split(".")) {
    if (!cursor || typeof cursor !== "object" || Array.isArray(cursor) || !(part in cursor)) return key
    cursor = (cursor as Record<string, unknown>)[part]
  }
  if (typeof cursor !== "string") return key
  return cursor.replace(/\{([^}]+)\}/g, (_, name: string) => String(params[name] ?? `{${name}}`))
}

describe("curse-lure talisman DSL", () => {
  it("links the item and models one extra normal current-map enemy per refresh", () => {
    const item = sourceRows("items.json", "items").find((row) => row.id === "curse_lure_talisman")
    const battleItem = sourceRows("battle_usable_items.json", "battle_usable_items")
      .find((row) => row.id === "battle_item_curse_lure_talisman")
    const buff = sourceRows("buff.json", "buff")
      .find((row) => row.id === "curse_lure_enemy_refresh")
    const buffEffect = sourceRows("buff_effects.json", "buff_effects")
      .find((row) => row.id === "curse_lure_enemy_refresh_on_enemy_refresh")
    const action = sourceRows("actions.json", "actions")
      .find((row) => row.id === "craft_curse_lure_talisman")

    expect(item).toMatchObject({
      iconId: "item.curse_lure_talisman",
      battleUsableItemId: "battle_item_curse_lure_talisman"
    })
    expect(battleItem).toMatchObject({ itemId: "curse_lure_talisman", cooldownTurns: 5 })
    expect(action).toMatchObject({
      iconId: "item.curse_lure_talisman",
      mainType: "crafting",
      menu: "crafting",
      subType: "craft_special",
      statusRequirement: { crafting: 40 },
      costs: {
        base4_cursed_wing_membrane: 4,
        base4_shadow_silk: 4,
        silverleaf_fiber: 12,
        clear_resin: 4
      },
      itemReward: { curse_lure_talisman: 1 },
      mainOutput: "curse_lure_talisman"
    })
    expect(battleItem).toMatchObject({
      effects: [{
        kind: "status",
        targeting: { targetScope: "self", selectRule: "self" },
        statusPayload: {
          statusIds: ["curse_lure_enemy_refresh"],
          durationBaseTurns: 5,
          applyChanceBase: 1
        }
      }]
    })
    expect(buff).toMatchObject({
      durationTurns: 5,
      kind: "debuff",
      stackPolicy: "refresh_duration",
      type: "curse"
    })
    expect(buffEffect).toMatchObject({
      buffId: "curse_lure_enemy_refresh",
      trigger: "on_enemy_refresh",
      effects: [{
        kind: "extra_enemy_refresh",
        enemyPool: "current_world",
        count: 1
      }]
    })

    const sections = buildEffectDetailSections({
      effects: battleItem?.effects as never,
      t: translate,
      variant: "battle_item"
    })
    expect(sections[0]?.rows.find((row) => row.label === "施加状态")?.buffIds)
      .toEqual(["curse_lure_enemy_refresh"])

    const itemCopy = (zhGameData.item as Record<string, { effect?: string }>).curse_lure_talisman?.effect || ""
    expect(itemCopy).toContain("每次敌人刷新")
    expect(itemCopy).toContain("不受地图常规数量限制")
    expect(itemCopy).toContain("正常提供经验与掉落")
  })
})
