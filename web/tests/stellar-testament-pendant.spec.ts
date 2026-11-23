import { describe, expect, it } from "vitest"
import { getActionDefinition } from "../src/services/action-definition"
import { buildEnhanceCumulativeAttrs } from "../src/services/equipment-enhance"
import { getEquipmentTemplateDefinition } from "../src/services/item-definition"
import gameData from "../../packages/game-data/generated/game-data.json"

const METEOR_MATERIALS = [
  "star_rail_copper_plate",
  "refraction_lens_shard",
  "meteor_core_fragment",
  "condensed_stardust"
] as const

describe("stellar testament pendant", () => {
  it("uses a generic deferred death-to-star contract without recursive stars", () => {
    const template = getEquipmentTemplateDefinition("stellar_testament_pendant")
    const deathHook = template?.battleHooks.find((hook) => hook.id === "stellar_testament_unit_death")
    const summon = deathHook?.effects.find((effect) => effect.kind === "summon")
    const payload = summon?.summonPayload as {
      unitId?: string
      side?: string
      maxCountPerUnit?: number
      maxCountFormula?: { baseAdd?: number; terms?: Array<{ attrKey?: string; coefficient?: number }> }
      persistAcrossBattles?: boolean
      onCasterDeath?: string
      onCapOverflow?: string
      statsFormula?: Record<string, { terms?: Array<{ attrKey?: string; coefficient?: number }> }>
    } | undefined

    expect(deathHook?.trigger).toBe("on_unit_death")
    expect(deathHook?.subjectFilter?.excludeSummonTemplateIds).toEqual(["stellar_testament_star"])
    expect(payload).toMatchObject({
      unitId: "stellar_testament_star",
      side: "source",
      maxCountPerUnit: 6,
      persistAcrossBattles: false,
      onCasterDeath: "despawn_immediately",
      onCapOverflow: "replace_oldest"
    })
    expect(payload?.maxCountFormula).toEqual({
      baseAdd: 6,
      terms: [expect.objectContaining({ attrKey: "equipment_enhance_level", coefficient: 0.1 })]
    })
    expect(payload?.statsFormula?.magical_damage?.terms).toEqual(expect.arrayContaining([
      expect.objectContaining({ attrKey: "perception", coefficient: 0.2 }),
      expect.objectContaining({ attrKey: "magical_damage", coefficient: 0.1 }),
      expect.objectContaining({ attrKey: "equipment_enhance_level", coefficient: 5 })
    ]))

    const bindHook = template?.battleHooks.find((hook) => hook.id === "stellar_testament_bind_star")
    expect(bindHook?.trigger).toBe("on_after_summon")
    expect(bindHook?.subjectFilter?.includeSummonTemplateIds).toEqual(["stellar_testament_star"])

    const commandRule = gameData.buff_effects.find((rule) => rule.id === "stellar_testament_command_wa")
    expect(commandRule?.effects).toContainEqual(expect.objectContaining({
      kind: "trigger_synced_summon_turn",
      includeSummonTemplateIds: ["stellar_testament_star"]
    }))
  })

  it("keeps two stats and uses every corridor material at every enhancement level", () => {
    const template = getEquipmentTemplateDefinition("stellar_testament_pendant")
    const action = getActionDefinition("craft_stellar_testament_pendant")

    expect(Object.keys(template?.baseAttrs ?? {})).toHaveLength(2)
    expect(template?.baseAttrs).toEqual({ perception: 14, magical_damage: 10 })
    expect(template?.enhanceMaxLevel).toBe(40)
    expect(template?.enhanceCostCurve.overrides).toEqual({})
    for (const materialId of METEOR_MATERIALS) {
      expect(template?.enhanceCostCurve.base[materialId]).toBe(1)
      expect(template?.enhanceCostCurve.step[materialId]).toBe(1)
      expect(action?.costs[materialId]).toBeGreaterThan(0)
    }
    expect(action?.costs.timekeeper_core).toBe(1)

    const maxAttrs = buildEnhanceCumulativeAttrs(template, 40)
    expect(Object.keys(maxAttrs).sort()).toEqual(["magical_damage", "perception"])
    expect(maxAttrs.perception).toBeCloseTo(52.55)
    expect(maxAttrs.magical_damage).toBeCloseTo(108)
  })
})
