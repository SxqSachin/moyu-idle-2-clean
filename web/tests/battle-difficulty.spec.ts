import { describe, expect, it } from "vitest"
import {
  getCombatDifficultyLevel,
  listCombatDifficultyLevels,
  normalizeWorldDifficultySettings,
  projectBattleEnemyDifficultyStats,
  resolveDifficultyCurveValue,
  resolveWorldDifficultyStatBonuses
} from "../src/services/battle-difficulty"

describe("battle enemy difficulty projection", () => {
  it("limits selectable levels to the dungeon-authored maximum", () => {
    const settings = normalizeWorldDifficultySettings({ maxLevel: 3 })

    expect(listCombatDifficultyLevels(settings).map((row) => row.level)).toEqual([1, 2, 3])
    expect(getCombatDifficultyLevel(4, settings)).toBeNull()
  })

  it("matches the runtime order of global scaling followed by dungeon flat bonuses", () => {
    const settings = {
      maxLevel: 2,
      statBonuses: [
        { attrKey: "hp", base: 20, perLevelMult: 0.5 },
        { attrKey: "speed", base: 5, perLevelMult: 0 }
      ],
      specialDrops: []
    }

    expect(resolveWorldDifficultyStatBonuses(settings, 2)).toMatchObject([
      { attrKey: "hp", bonus: 30 },
      { attrKey: "speed", bonus: 5 }
    ])
    expect(projectBattleEnemyDifficultyStats({
      hp: 100,
      speed: 10,
      battle: 60,
      non_combat_marker: 7
    }, 2, settings)).toEqual({
      hp: 165,
      speed: 18.5,
      battle: 60,
      non_combat_marker: 7
    })
  })

  it("does not invent missing attributes or apply unsupported difficulty levels", () => {
    const settings = {
      maxLevel: 1,
      statBonuses: [{ attrKey: "magical_defence", base: 50, perLevelMult: 0 }],
      specialDrops: []
    }
    const base = { hp: 100 }

    expect(projectBattleEnemyDifficultyStats(base, 1, settings).hp).toBeCloseTo(115, 10)
    expect(projectBattleEnemyDifficultyStats(base, 2, settings)).toEqual(base)
    expect(projectBattleEnemyDifficultyStats(base, 0, settings)).toEqual(base)
  })

  it("resolves explicit curve overrides and falls back safely", () => {
    expect(resolveDifficultyCurveValue({ base: 2, step: 0.5 }, 3)).toBe(3)
    expect(resolveDifficultyCurveValue({ base: 2, overrides: { 3: 7 } }, 3)).toBe(7)
    expect(resolveDifficultyCurveValue({ base: 2, overrides: { 3: "invalid" } }, 3)).toBe(2)
  })
})
