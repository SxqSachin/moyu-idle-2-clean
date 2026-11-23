import { describe, expect, it } from "vitest"
import zhCN from "../src/locales/game/zh-CN"
import { getActionDefinition } from "../src/services/action-definition"
import { buildBattleHookDescriptions } from "../src/services/battle-hook-display"
import { buildEnhanceCumulativeAttrs } from "../src/services/equipment-enhance"
import { getEquipmentTemplateDefinition, getItemDefinition } from "../src/services/item-definition"

const CORRIDOR_MATERIALS = [
  "star_rail_copper_plate",
  "refraction_lens_shard",
  "meteor_core_fragment",
  "condensed_stardust"
] as const

function t(key: string, params?: Record<string, unknown>): string {
  const fixed: Record<string, string> = {
    "attr.magical_damage.name": "魔法攻击",
    "game.itemDetailBattleHookAttrEquipmentEnhanceLevel": "强化等级",
    "game.damageType.magical": "魔法",
    "game.itemDetailBattleHookTurnKind.ordered": "正常行动",
    "game.itemDetailBattleHookTurnScope.ordered": "仅计正常行动",
    "game.itemDetailBattleHookTurnScopeNote.excludePreparation": "不计准备回合",
    "game.itemDetailBattleHookTurnScopeNote.includeSuppressed": "受控跳过仍计入",
    "game.itemDetailBattleHookTurnScopeNoteSeparator": "，"
  }
  if (fixed[key]) return fixed[key]
  if (key === "game.itemDetailBattleHookEventTurnKinds") return `仅计入${String(params?.kinds)}`
  if (key === "game.itemDetailBattleHookEventExcludePreparation") return "不计准备回合"
  if (key === "game.itemDetailBattleHookEventIncludeSuppressedTurns") return "受控跳过仍计入"
  if (key === "game.itemDetailBattleHookTurnScopeWithNotes") return `${String(params?.scope)}（${String(params?.notes)}）`
  if (key === "game.itemDetailBattleHookTargetMode.targeting") return "所选目标"
  if (key === "game.targetScope.all") return "全场"
  if (key === "game.itemDetailBattleHookEffectDamageByRelation") {
    return `${String(params?.target)}|友军:${String(params?.allyFormula)}|敌军:${String(params?.enemyFormula)}`
  }
  if (key === "game.itemDetailBattleHookEffectDamageSimultaneous") return "批量结算"
  return key
}

describe("five-tick starfall lunar veil robe", () => {
  it("uses generic ordered-turn filters and team-relation damage formulas", () => {
    const template = getEquipmentTemplateDefinition("five_tick_starfall_lunar_veil_robe")
    const countHook = template?.battleHooks.find((hook) => hook.id === "five_tick_starfall_lunar_veil_count")
    const burstHook = template?.battleHooks.find((hook) => hook.id === "five_tick_starfall_lunar_veil_burst")
    const damage = burstHook?.effects.find((effect) => effect.kind === "damage")

    expect(countHook?.eventFilter).toMatchObject({
      turnKinds: ["ordered"],
      includePreparation: false,
      includeSuppressedTurns: true
    })
    expect(damage).toMatchObject({
      relationMode: "team_side",
      simultaneous: true,
      dispatchEquipmentHooks: false,
      amountFormulaByRelation: {
        ally: {
          baseAdd: 600,
          terms: [
            { attrKey: "magical_damage", coefficient: 0.45, perSkillLevel: 0 },
            { attrKey: "equipment_enhance_level", coefficient: -8, perSkillLevel: 0 }
          ]
        },
        enemy: {
          baseAdd: 600,
          terms: [
            { attrKey: "magical_damage", coefficient: 0.45, perSkillLevel: 0 },
            { attrKey: "equipment_enhance_level", coefficient: 60, perSkillLevel: 0 }
          ]
        }
      }
    })

    const descriptions = buildBattleHookDescriptions(template?.battleHooks ?? [], t)
    expect(descriptions[0]?.conditions.map((entry) => entry.text)).toEqual([
      "仅计正常行动（不计准备回合，受控跳过仍计入）"
    ])
    const damageText = descriptions[1]?.effects.find((effect) => effect.kind === "text")
    expect(damageText && damageText.kind === "text" ? damageText.text : "").toContain("友军:600 + 0.45×魔法攻击 - 8×强化等级")
    expect(damageText && damageText.kind === "text" ? damageText.text : "").toContain("敌军:600 + 0.45×魔法攻击 + 60×强化等级")
    expect(damageText && damageText.kind === "text" ? damageText.text : "").toContain("批量结算")
  })

  it("uses the predecessor, every corridor material, and a boss core", () => {
    const item = getItemDefinition("five_tick_starfall_lunar_veil_robe")
    const template = getEquipmentTemplateDefinition("five_tick_starfall_lunar_veil_robe")
    const action = getActionDefinition("sewing_five_tick_starfall_lunar_veil_robe")

    expect(item?.economyBaseValue).toBe(16400)
    expect(action?.costs).toMatchObject({ lunar_veil_robe: 1, timekeeper_core: 1 })
    expect(template?.enhanceCostCurve.overrides).toEqual({})
    expect(Object.keys(template?.enhanceCostCurve.base ?? {}).sort()).toEqual([...CORRIDOR_MATERIALS].sort())
    for (const materialID of CORRIDOR_MATERIALS) {
      expect(action?.costs[materialID]).toBeGreaterThan(0)
      expect(template?.enhanceCostCurve.base[materialID]).toBe(1)
      expect(template?.enhanceCostCurve.step[materialID]).toBe(1)
    }
  })

  it("finishes slightly above the fully enhanced lunar-veil robe", () => {
    const upgraded = getEquipmentTemplateDefinition("five_tick_starfall_lunar_veil_robe")
    const predecessor = getEquipmentTemplateDefinition("lunar_veil_robe")
    expect(upgraded).toBeTruthy()
    expect(predecessor).toBeTruthy()

    const upgradedEnhance = buildEnhanceCumulativeAttrs(upgraded!, upgraded!.enhanceMaxLevel)
    const predecessorEnhance = buildEnhanceCumulativeAttrs(predecessor!, predecessor!.enhanceMaxLevel)
    const upgradedIntelligence = (upgraded!.baseAttrs.intelligence ?? 0) + (upgradedEnhance.intelligence ?? 0)
    const upgradedMagic = (upgraded!.baseAttrs.magical_damage ?? 0) + (upgradedEnhance.magical_damage ?? 0)
    const predecessorIntelligence = (predecessor!.baseAttrs.intelligence ?? 0) + (predecessorEnhance.intelligence ?? 0)
    const predecessorMagic = (predecessor!.baseAttrs.magical_damage ?? 0) + (predecessorEnhance.magical_damage ?? 0)

    expect(upgradedIntelligence).toBeCloseTo(172)
    expect(upgradedMagic).toBeCloseTo(146)
    expect(upgradedIntelligence).toBeGreaterThanOrEqual(predecessorIntelligence)
    expect(upgradedMagic).toBeGreaterThanOrEqual(predecessorMagic)
    expect((upgraded!.baseAttrs.magical_defence ?? 0) + (upgradedEnhance.magical_defence ?? 0)).toBeCloseTo(62.3)
  })

  it("localizes adjust_skill_cooldown instead of exposing the DSL token", () => {
    expect((zhCN as { game: { skillEffectKind: Record<string, string> } }).game.skillEffectKind.adjust_skill_cooldown).toBe("调整技能冷却")
  })
})
