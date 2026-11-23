import { describe, expect, it } from "vitest"
import { getActionDefinition } from "../src/services/action-definition"
import { buildBattleHookDescriptions } from "../src/services/battle-hook-display"
import { getEquipmentTemplateDefinition } from "../src/services/item-definition"

function t(key: string, params?: Record<string, unknown>): string {
  const fixed: Record<string, string> = {
    "skill_type.arcane.name": "奥术",
    "skill_type.perception.name": "感知",
    "skill_type.snowpact.name": "雪契",
    "game.itemDetailBattleHookTargetMode.self": "自身",
    "game.itemDetailBattleHookCooldownSelection.max_remaining": "剩余冷却最高",
    "game.itemDetailBattleHookCooldownCountAll": "全部",
    "game.itemDetailBattleHookSkillCategoryAny": "任意类型"
  }
  if (fixed[key]) return fixed[key]
  if (key === "game.itemDetailBattleHookConditionSelfSkillCooldownCountGte") {
    return `至少${String(params?.value)}个${String(params?.categories)}技能正在冷却`
  }
  if (key === "game.itemDetailBattleHookConditionEquipmentEnhanceLevelGte") {
    return `装备强化等级≥${String(params?.value)}`
  }
  if (key === "game.itemDetailBattleHookEffectAdjustSkillCooldown") {
    return `${String(params?.categories)}|${String(params?.selection)}|${String(params?.count)}|${String(params?.change)}`
  }
  return key
}

describe("timekeeper eight-tick wheel battle hook display", () => {
  it("renders category filtering, highest-cooldown selection, and awakening threshold", () => {
    const template = getEquipmentTemplateDefinition("timekeeper_eight_tick_wheel")
    expect(template).toBeTruthy()

    const descriptions = buildBattleHookDescriptions(template?.battleHooks ?? [], t)
    expect(descriptions).toHaveLength(2)
    expect(descriptions[0]?.conditions.map((entry) => entry.text).join(" / ")).toContain("奥术 / 感知 / 雪契")

    const turnEffect = descriptions[0]?.effects.find((effect) => effect.kind === "text")
    expect(turnEffect && turnEffect.kind === "text" ? turnEffect.text : "").toBe("奥术 / 感知 / 雪契|剩余冷却最高|1|-1")

    expect(descriptions[1]?.conditions.map((entry) => entry.text)).toContain("装备强化等级≥40")
  })

  it("keeps material costs linear while retaining attribute milestones", () => {
    const template = getEquipmentTemplateDefinition("timekeeper_eight_tick_wheel")
    const action = getActionDefinition("craft_timekeeper_eight_tick_wheel")

    expect(template?.baseAttrs).toEqual({ intelligence: 8, perception: 8 })
    expect(template?.enhanceMaxLevel).toBe(40)
    expect(template?.enhanceCostCurve.overrides).toEqual({})
    expect(Object.keys(template?.enhanceCostCurve.step ?? {}).sort()).toEqual(Object.keys(template?.enhanceCostCurve.base ?? {}).sort())
    expect(Object.values(template?.enhanceCostCurve.step ?? {}).every((value) => value === 1)).toBe(true)
    expect(template?.enhanceAttrCurve.overrides["40"]).toEqual({ intelligence: 16, perception: 14 })
    expect(action?.costs).toMatchObject({
      star_rail_copper_plate: 4,
      condensed_stardust: 4,
      refraction_lens_shard: 2,
      meteor_core_fragment: 2,
      timekeeper_core: 1
    })
  })
})
