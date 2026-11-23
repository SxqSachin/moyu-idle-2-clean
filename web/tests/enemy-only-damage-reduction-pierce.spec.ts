import { describe, expect, it } from "vitest"
import enGameData from "../src/locales/game-data/en-US"
import zhGameData from "../src/locales/game-data/zh-CN"
import enGame from "../src/locales/game/en-US"
import zhGame from "../src/locales/game/zh-CN"
import { buildEffectDetailSections } from "../src/services/effect-detail-renderer"
import { getSkillDefinition, installRuntimeSkillDefinitions } from "../src/services/skill-definition"

type LocaleTree = Record<string, unknown>

function readLocale(tree: LocaleTree, key: string): unknown {
  let cursor: unknown = tree
  for (const part of key.split(".")) {
    if (!cursor || typeof cursor !== "object" || Array.isArray(cursor) || !(part in cursor)) return undefined
    cursor = (cursor as LocaleTree)[part]
  }
  return cursor
}

function createT(...trees: LocaleTree[]) {
  return (key: string, params: Record<string, unknown> = {}): string => {
    const value = trees.map((tree) => readLocale(tree, key)).find((entry) => typeof entry === "string")
    if (typeof value !== "string") return key
    return value.replace(/\{([^}]+)\}/g, (_, name: string) => String(params[name] ?? `{${name}}`))
  }
}

describe("monster-only damage-reduction pierce copy", () => {
  it("preserves the enemy-only contract through runtime skill normalization", () => {
    installRuntimeSkillDefinitions([{
      id: "enemy_only_projection_test",
      enemyOnly: true,
      effects: [{
        kind: "damage",
        targeting: { targetScope: "enemy", targetPositions: ["vanguard"], ignoreTargetPriority: true },
        damageSpec: {
          enemyOnlyDamageReductionPierce: 0.4,
          enemyOnlyDamageReductionPierceTargetStatuses: [" amusement_spotlight "]
        }
      }]
    }])
    const skill = getSkillDefinition("enemy_only_projection_test")
    expect(skill?.enemyOnly).toBe(true)
    expect(skill?.effects[0]?.damageSpec).toMatchObject({
      enemyOnlyDamageReductionPierce: 0.4,
      enemyOnlyDamageReductionPierceTargetStatuses: ["amusement_spotlight"]
    })
    expect(skill?.effects[0]?.targeting.ignoreTargetPriority).toBe(true)
  })

  it("explains the conditional pressure in player language with localized statuses", () => {
    const effect = {
      kind: "damage",
      damageType: "magical",
      targeting: { targetScope: "enemy", targetPositions: ["vanguard"], ignoreTargetPriority: true },
      amountFormula: { baseAdd: 500, terms: [] },
      damageSpec: {
        enemyOnlyDamageReductionPierce: 0.35,
        enemyOnlyDamageReductionPierceTargetStatuses: ["amusement_spotlight"]
      }
    }
    const zhRows = buildEffectDetailSections({ effects: [effect], t: createT(zhGame, zhGameData) })[0]?.rows ?? []
    const enRows = buildEffectDetailSections({ effects: [effect], t: createT(enGame, enGameData) })[0]?.rows ?? []
    const zh = zhRows.find((row) => row.label === "怪物压制")?.value || ""
    const en = enRows.find((row) => row.label === "Monster Pressure")?.value || ""
    const lockedRow = zhRows.find((row) => row.label === "锁定排位")?.value || ""

    expect(zh).toContain("仅限怪物施放")
    expect(zh).toContain("35%")
    expect(zh).toContain(createT(zhGameData)("buff.amusement_spotlight.name"))
    expect(en).toContain("Monster use only")
    expect(en).toContain("35%")
    expect(lockedRow).toContain("前排")
    expect(lockedRow).toContain("无视嘲讽与目标优先效果")
    expect([zh, en].join(" ")).not.toMatch(/enemyOnlyDamageReductionPierce|amusement_spotlight/)
    expect(lockedRow).not.toContain("ignoreTargetPriority")
  })

  it("omits the row when the skill has no active pierce", () => {
    const sections = buildEffectDetailSections({
      t: createT(zhGame, zhGameData),
      effects: [{
        kind: "damage",
        targeting: { targetScope: "enemy" },
        amountFormula: { baseAdd: 100, terms: [] },
        damageSpec: { enemyOnlyDamageReductionPierce: 0 }
      }]
    })
    expect(sections[0]?.rows.some((row) => row.label === "怪物压制")).toBe(false)
  })

  it("keeps authored row attacks legible in the short skill and enemy copy", () => {
    const expectedRows = [
      ["carousel_roundabout_lock", "前排", "front row"],
      ["candy_clown_double_prank", "中排", "middle row"],
      ["parade_king_team_carnival", "前排", "front row"],
      ["midnight_ticket_punch", "前排", "front row"],
      ["faceless_understudy_stolen_bow", "中排", "middle row"]
    ] as const

    for (const [skillId, zhRow, enRow] of expectedRows) {
      expect(readLocale(zhGameData, `skill.${skillId}.desc`)).toContain(zhRow)
      expect(readLocale(enGameData, `skill.${skillId}.desc`)).toContain(enRow)
    }

    expect(readLocale(zhGameData, "enemy.carousel_cat_001.desc")).toContain("前排")
    expect(readLocale(zhGameData, "enemy.candy_clown_001.desc")).toContain("中排")
    expect(readLocale(enGameData, "enemy.carousel_cat_001.desc")).toContain("front row")
    expect(readLocale(enGameData, "enemy.candy_clown_001.desc")).toContain("middle row")
  })
})
