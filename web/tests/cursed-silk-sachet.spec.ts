import { describe, expect, it } from "vitest"
import zhGameData from "../src/locales/game-data/zh-CN"
import zhGame from "../src/locales/game/zh-CN"
import { buildBattleHookDescriptions } from "../src/services/battle-hook-display"
import { getEquipmentTemplateDefinition } from "../src/services/item-definition"

type LocaleTree = Record<string, unknown>

function readLocale(tree: LocaleTree, key: string): unknown {
  let cursor: unknown = tree
  for (const part of key.split(".")) {
    if (!cursor || typeof cursor !== "object" || Array.isArray(cursor) || !(part in cursor)) return undefined
    cursor = (cursor as LocaleTree)[part]
  }
  return cursor
}

function t(key: string, params: Record<string, unknown> = {}): string {
  const value = [zhGame, zhGameData]
    .map((tree) => readLocale(tree as LocaleTree, key))
    .find((entry) => typeof entry === "string")
  if (typeof value !== "string") return key
  return value.replace(/\{([^}]+)\}/g, (_, name: string) => String(params[name] ?? `{${name}}`))
}

describe("cursed silk sachet battle DSL display", () => {
  it("shows a per-wave curse and direct extra Boss refresh milestones", () => {
    const template = getEquipmentTemplateDefinition("cursed_silk_sachet")
    const descriptions = buildBattleHookDescriptions(template?.battleHooks ?? [], t)

    expect(descriptions).toHaveLength(2)
    expect(descriptions[0]?.triggerText).toBe("每波开始时")
    expect(descriptions[0]?.effects[0]?.kind).toBe("add_buff")
    expect(descriptions[1]?.triggerText).toBe("创建本波敌人时")

    const bossEffect = descriptions[1]?.effects[0]
    expect(bossEffect?.kind).toBe("text")
    const text = bossEffect?.kind === "text" ? bossEffect.text : ""
    expect(text).toContain("强化 +20 后为 2")
    expect(text).toContain("强化 +40 后为 3")
    expect(text).toContain("不是召唤物")
    expect(text).toContain("正常提供经验与掉落")
    expect(text).toContain("同队多件只取最高强化效果")
    expect(text).not.toMatch(/extra_enemy_refresh|current_wave_boss|equipment_enhance_level/)
  })
})
