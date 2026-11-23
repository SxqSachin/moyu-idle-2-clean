import { describe, expect, it } from "vitest"
import { getActionDefinition } from "../src/services/action-definition"
import { buildBattleHookDescriptions } from "../src/services/battle-hook-display"
import { getEquipmentTemplateDefinition, getItemDefinition } from "../src/services/item-definition"

const METEOR_MATERIALS = [
  "star_rail_copper_plate",
  "refraction_lens_shard",
  "meteor_core_fragment",
  "condensed_stardust"
] as const

const LIBRARY_MATERIALS = [
  "hushed_ink_crystal",
  "wandering_page_silk",
  "blue_night_audit_feather",
  "spell_seal_lock_plate"
] as const

function t(key: string, params?: Record<string, unknown>): string {
  const fixed: Record<string, string> = {
    "skill_type.arcane.name": "奥术",
    "skill_type.perception.name": "感知",
    "skill_type.snowpact.name": "雪契",
    "skill_type.holy.name": "圣愈",
    "skill_type.holy_cat.name": "圣猫祷术",
    "buff.forbidden_binding_free_transfer.name": "免罚转移"
  }
  if (fixed[key]) return fixed[key]
  if (key === "game.itemDetailBattleHookEffectTransferSkillCooldown") {
    return `${String(params?.categories)}|${String(params?.amount)}|${String(params?.waiver)}`
  }
  if (key === "game.itemDetailBattleHookEffectTransferSkillCooldownWaiver") {
    return `觉醒:${String(params?.buff)}`
  }
  return key
}

function expectEveryMaterial(costs: Record<string, number>, materialIds: readonly string[]): void {
  for (const materialId of materialIds) expect(costs[materialId]).toBeGreaterThan(0)
}

function expectLinearMaterialCurve(curve: {
  base: Record<string, number>
  step: Record<string, number>
  overrides: Record<string, Record<string, number>>
}): void {
  expect(curve.overrides).toEqual({})
  expect(Object.keys(curve.step).sort()).toEqual(Object.keys(curve.base).sort())
  for (const materialId of Object.keys(curve.base)) expect(curve.step[materialId]).toBe(1)
}

describe("forbidden binding bracer", () => {
  it("transfers cooldown only among the five existing caster categories", () => {
    const template = getEquipmentTemplateDefinition("forbidden_binding_bracer")
    expect(template).toBeTruthy()

    const transferHook = template?.battleHooks.find((hook) => hook.id === "forbidden_binding_bracer_transfer")
    const transfer = transferHook?.effects.find((effect) => effect.kind === "transfer_skill_cooldown")
    const from = transfer?.from as Record<string, unknown> | undefined
    const to = transfer?.to as Record<string, unknown> | undefined
    const categories = ["arcane", "perception", "snowpact", "holy", "holy_cat"]
    expect(transferHook?.trigger).toBe("on_after_skill_cooldown_tick")
    expect(from?.skillCategories).toEqual(categories)
    expect(to?.skillCategories).toEqual(categories)
    expect(transfer?.requireBoth).toBe(true)

    const descriptions = buildBattleHookDescriptions(template?.battleHooks ?? [], t)
    const text = descriptions.flatMap((row) => row.effects)
      .find((effect) => effect.kind === "text" && effect.text.includes("奥术"))
    expect(text && text.kind === "text" ? text.text : "").toContain("奥术 / 感知 / 雪契 / 圣愈 / 圣猫祷术")
  })

  it("uses every material with a linear +1 step and no cost milestones", () => {
    const wheel = getEquipmentTemplateDefinition("timekeeper_eight_tick_wheel")
    const bracer = getEquipmentTemplateDefinition("forbidden_binding_bracer")
    expect(wheel).toBeTruthy()
    expect(bracer).toBeTruthy()

    expectEveryMaterial(wheel!.enhanceCostCurve.base, METEOR_MATERIALS)
    expectEveryMaterial(bracer!.enhanceCostCurve.base, LIBRARY_MATERIALS)
    expectLinearMaterialCurve(wheel!.enhanceCostCurve)
    expectLinearMaterialCurve(bracer!.enhanceCostCurve)
    expect(Object.keys(wheel!.enhanceAttrCurve.overrides)).toEqual(["10", "20", "30", "40"])
    expect(Object.keys(bracer!.enhanceAttrCurve.overrides)).toEqual(["10", "20", "30", "40"])
  })

  it("keeps the stat budget restrained and the material values modest", () => {
    const template = getEquipmentTemplateDefinition("forbidden_binding_bracer")
    const action = getActionDefinition("craft_forbidden_binding_bracer")
    expect(template?.baseAttrs).toEqual({ intelligence: 5, perception: 8, mp: 70, magical_defence: 10 })
    expect(template?.enhanceMaxLevel).toBe(40)
    expectEveryMaterial(action?.costs ?? {}, LIBRARY_MATERIALS)
    expect(action?.costs.final_volume_key_sigil).toBe(1)

    const expectedValues: Record<string, number> = {
      star_rail_copper_plate: 110,
      refraction_lens_shard: 190,
      meteor_core_fragment: 205,
      condensed_stardust: 285,
      timekeeper_core: 480,
      hushed_ink_crystal: 120,
      wandering_page_silk: 205,
      blue_night_audit_feather: 220,
      spell_seal_lock_plate: 310,
      final_volume_key_sigil: 520
    }
    for (const [itemId, value] of Object.entries(expectedValues)) {
      expect(getItemDefinition(itemId)?.economyBaseValue).toBe(value)
    }
  })
})
