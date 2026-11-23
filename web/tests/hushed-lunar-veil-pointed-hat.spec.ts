import { describe, expect, it } from "vitest"
import { getActionDefinition } from "../src/services/action-definition"
import { buildBattleHookDescriptions } from "../src/services/battle-hook-display"
import { buildEnhanceCumulativeAttrs } from "../src/services/equipment-enhance"
import { getEquipmentTemplateDefinition, getItemDefinition } from "../src/services/item-definition"

const LIBRARY_MATERIALS = [
  "hushed_ink_crystal",
  "wandering_page_silk",
  "blue_night_audit_feather",
  "spell_seal_lock_plate"
] as const

function t(key: string, params?: Record<string, unknown>): string {
  const fixed: Record<string, string> = {
    "buff.silence.name": "沉默",
    "buff.holy_cat_silent_liturgy.name": "肃静礼拜",
    "attr.perception.name": "感知",
    "game.itemDetailBattleHookAttrEquipmentEnhanceLevel": "强化等级",
    "game.itemDetailBattleHookTargetMode.source": "受沉默目标",
    "game.damageType.magical": "魔法"
  }
  if (fixed[key]) return fixed[key]
  if (key === "game.itemDetailBattleHookEventBuffIds") return `状态:${String(params?.buffs)}`
  if (key === "game.itemDetailBattleHookEventApplicationKinds") return `结果:${String(params?.kinds)}`
  if (key === "game.itemDetailBattleHookApplicationKind.attach") return "首次施加"
  if (key === "game.itemDetailBattleHookApplicationKind.refresh") return "刷新"
  if (key === "game.itemDetailBattleHookEventExcludeSelf") return "目标不是自身"
  if (key === "game.itemDetailBattleHookEffectDamage") return `${String(params?.target)}|${String(params?.damageType)}|${String(params?.formula)}`
  return key
}

describe("hushed lunar-veil pointed hat", () => {
  it("uses the generic after-buff event filter and enhancement-scaled damage", () => {
    const template = getEquipmentTemplateDefinition("hushed_lunar_veil_pointed_hat")
    const hook = template?.battleHooks.find((entry) => entry.id === "hushed_lunar_veil_silence_echo")
    const damage = hook?.effects.find((effect) => effect.kind === "damage")

    expect(template?.slot).toBe("head")
    expect(template?.baseAttrs).toEqual({ intelligence: 20, perception: 18, mp_regen: 3.2 })
    expect(hook?.trigger).toBe("on_after_apply_buff")
    expect(hook?.eventFilter).toEqual({
      buffIds: ["silence", "holy_cat_silent_liturgy"],
      applicationKinds: ["attach", "refresh"],
      excludeSelf: true
    })
    expect(damage).toMatchObject({
      targetMode: "source",
      damageType: "magical",
      dispatchEquipmentHooks: false,
      amountFormula: {
        baseAdd: 0,
        terms: [
          { attrKey: "perception", coefficient: 1.6, perSkillLevel: 0 },
          { attrKey: "equipment_enhance_level", coefficient: 4, perSkillLevel: 0 }
        ]
      }
    })

    const description = buildBattleHookDescriptions(template?.battleHooks ?? [], t)[0]
    expect(description?.conditions.map((entry) => entry.text)).toEqual([
      "状态:沉默 / 肃静礼拜",
      "结果:首次施加 / 刷新",
      "目标不是自身"
    ])
    const effect = description?.effects.find((entry) => entry.kind === "text")
    expect(effect && effect.kind === "text" ? effect.text : "").toContain("1.6×感知 + 4×强化等级")
  })

  it("keeps crafting and enhancement material use restrained and linear", () => {
    const item = getItemDefinition("hushed_lunar_veil_pointed_hat")
    const template = getEquipmentTemplateDefinition("hushed_lunar_veil_pointed_hat")
    const action = getActionDefinition("sewing_hushed_lunar_veil_pointed_hat")

    expect(item?.economyBaseValue).toBe(13220)
    expect(action?.costs).toEqual({
      lunar_veil_pointed_hat: 1,
      hushed_ink_crystal: 4,
      wandering_page_silk: 4,
      blue_night_audit_feather: 3,
      spell_seal_lock_plate: 3,
      final_volume_key_sigil: 1
    })
    expect(template?.enhanceCostCurve.overrides).toEqual({})
    expect(Object.keys(template?.enhanceCostCurve.base ?? {}).sort()).toEqual([...LIBRARY_MATERIALS].sort())
    for (const materialID of LIBRARY_MATERIALS) {
      expect(template?.enhanceCostCurve.base[materialID]).toBe(1)
      expect(template?.enhanceCostCurve.step[materialID]).toBe(1)
    }
    expect(Object.keys(template?.enhanceAttrCurve.overrides ?? {})).toEqual(["10", "20", "30", "40"])
  })

  it("finishes at or above the fully enhanced lunar-veil base hat", () => {
    const upgraded = getEquipmentTemplateDefinition("hushed_lunar_veil_pointed_hat")
    const baseHat = getEquipmentTemplateDefinition("lunar_veil_pointed_hat")
    expect(upgraded).toBeTruthy()
    expect(baseHat).toBeTruthy()

    const upgradedEnhance = buildEnhanceCumulativeAttrs(upgraded!, upgraded!.enhanceMaxLevel)
    const baseEnhance = buildEnhanceCumulativeAttrs(baseHat!, baseHat!.enhanceMaxLevel)
    const upgradedFinal = {
      intelligence: (upgraded!.baseAttrs.intelligence ?? 0) + (upgradedEnhance.intelligence ?? 0),
      perception: (upgraded!.baseAttrs.perception ?? 0) + (upgradedEnhance.perception ?? 0),
      mp_regen: (upgraded!.baseAttrs.mp_regen ?? 0) + (upgradedEnhance.mp_regen ?? 0)
    }
    const baseFinal = {
      intelligence: (baseHat!.baseAttrs.intelligence ?? 0) + (baseEnhance.intelligence ?? 0),
      mp_regen: (baseHat!.baseAttrs.mp_regen ?? 0) + (baseEnhance.mp_regen ?? 0)
    }

    expect(upgradedFinal.intelligence).toBeCloseTo(303)
    expect(baseFinal.intelligence).toBeCloseTo(303)
    expect(upgradedFinal.intelligence).toBeGreaterThanOrEqual(baseFinal.intelligence)
    expect(upgradedFinal.mp_regen).toBeGreaterThanOrEqual(baseFinal.mp_regen)
    expect(upgradedFinal.perception).toBeGreaterThan(0)
  })
})
