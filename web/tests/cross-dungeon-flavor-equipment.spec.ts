import { describe, expect, it } from "vitest"
import { getActionDefinition } from "../src/services/action-definition"
import { buildBattleHookDescriptions } from "../src/services/battle-hook-display"
import { buildEnhanceCumulativeAttrs } from "../src/services/equipment-enhance"
import { getEquipmentTemplateDefinition } from "../src/services/item-definition"

const EQUIPMENT_IDS = [
  "holy_cat_revenge_star_staff",
  "prayer_ash_venom_censer",
  "cat_ancestor_forbidden_seal_crown",
  "volatile_spore_fuse_ring"
] as const

function t(key: string, params?: Record<string, unknown>): string {
  const fixed: Record<string, string> = {
    "attr.intelligence.name": "智力",
    "attr.magical_damage.name": "魔法攻击",
    "attr.perception.name": "感知",
    "attr.mp_regen.name": "魔法回复",
    "attr.magical_defence.name": "魔法防御",
    "attr.magical_damage_resist.name": "魔法减伤",
    "attr.magic_crit_damage.name": "魔法暴击伤害",
    "attr.speed.name": "速度",
    "game.itemDetailBattleHookAttrEquipmentEnhanceLevel": "强化等级",
    "game.damageType.magical": "魔法",
    "game.itemDetailBattleHookTargetMode.source": "受影响目标",
    "game.itemDetailBattleHookBuffKind.debuff": "负面状态",
    "game.itemDetailBattleHookBuffSelection.max_remaining": "剩余回合最长优先"
  }
  if (fixed[key]) return fixed[key]
  if (key === "game.itemDetailBattleHookEffectDamage") {
    return `${String(params?.target)}|${String(params?.damageType)}|${String(params?.formula)}`
  }
  if (key === "game.itemDetailBattleHookEffectRemoveBuffByKind") {
    return `${String(params?.target)}移除${String(params?.count)}个${String(params?.kind)}|${String(params?.selection)}${String(params?.dispellable)}`
  }
  if (key === "game.itemDetailBattleHookEffectDispellableOnly") return "|仅可驱散"
  if (key === "game.itemDetailBattleHookEffectProcChanceWithMax") {
    return `${String(params?.chance)}|最高${String(params?.max)}`
  }
  if (key === "game.itemDetailBattleHookEffectProcChance") return String(params?.chance)
  if (key === "game.itemDetailBattleHookEventBuffKinds") return `状态类型:${String(params?.kinds)}`
  if (key === "game.itemDetailBattleHookEventFirstBuffInstanceOnly") return "仅状态首层附着"
  if (key === "game.itemDetailBattleHookEventExcludeBuffIds") return `排除:${String(params?.buffs)}`
  if (key === "game.itemDetailBattleHookEventApplicationKinds") return `结果:${String(params?.kinds)}`
  if (key === "game.itemDetailBattleHookEventExcludeSelf") return "目标不是自身"
  if (key === "game.itemDetailBattleHookApplicationKind.attach") return "首次施加"
  if (key === "game.itemDetailBattleHookApplicationKind.refresh") return "刷新"
  if (key === "game.itemDetailBattleHookConditionTargetDistinctSourceBuffCountGte") {
    return `至少${String(params?.value)}种自身来源${String(params?.kind)}`
  }
  if (key === "game.itemDetailBattleHookConditionTargetBuffCountGte") {
    return `${String(params?.buff)}至少${String(params?.value)}层`
  }
  if (key === "game.skillTargetScope.enemy") return "敌军"
  if (key === "game.targetDirection.vanguard") return "前排"
  if (key === "game.targetDirection.midguard") return "中排"
  if (key === "game.targetDirection.rearguard") return "后排"
  if (key === "game.targetSeekStrategy.none") return "无次要寻敌"
  if (key === "game.skillSelectRule.random") return "随机"
  if (key === "game.itemDetailBattleHookTargetingRule") {
    return `${String(params?.scope)}中的${String(params?.positions)}目标（${String(params?.seek)}，优先${String(params?.select)}，数量${String(params?.count)}）`
  }
  return key
}

describe("cross-dungeon flavor equipment", () => {
  it("keeps slots and base attributes restrained", () => {
    expect(getEquipmentTemplateDefinition("holy_cat_revenge_star_staff")?.slot).toBe("left_hand")
    expect(getEquipmentTemplateDefinition("prayer_ash_venom_censer")?.slot).toBe("left_hand")
    expect(getEquipmentTemplateDefinition("cat_ancestor_forbidden_seal_crown")?.slot).toBe("head")
    expect(getEquipmentTemplateDefinition("volatile_spore_fuse_ring")?.slot).toBe("accessory")

    expect(getEquipmentTemplateDefinition("holy_cat_revenge_star_staff")?.baseAttrs).toEqual({ intelligence: 10, mp_regen: 0.5 })
    expect(getEquipmentTemplateDefinition("prayer_ash_venom_censer")?.baseAttrs).toEqual({ magical_defence: 12, magical_damage_resist: 0.008 })
    expect(getEquipmentTemplateDefinition("cat_ancestor_forbidden_seal_crown")?.baseAttrs).toEqual({ magical_damage: 8, magic_crit_damage: 0.02 })
    expect(getEquipmentTemplateDefinition("volatile_spore_fuse_ring")?.baseAttrs).toEqual({ perception: 12, speed: 2 })

    for (const id of EQUIPMENT_IDS) {
      const template = getEquipmentTemplateDefinition(id)
      expect(template).toBeTruthy()
      expect(Object.keys(template?.baseAttrs ?? {})).toHaveLength(2)
      expect(Object.keys(template?.enhanceAttrCurve.base ?? {}).sort()).toEqual(Object.keys(template?.baseAttrs ?? {}).sort())
      expect(Object.keys(template?.enhanceAttrCurve.step ?? {}).sort()).toEqual(Object.keys(template?.baseAttrs ?? {}).sort())
      expect(template?.affixSlotCount).toBe(0)
    }
  })

  it("preserves enhancement scaling inside generic formula terms", () => {
    const starHook = getEquipmentTemplateDefinition("holy_cat_revenge_star_staff")?.battleHooks[0]
    const starDamage = starHook?.effects.find((effect) => effect.kind === "damage")
    expect(starDamage).toMatchObject({
      targetMode: "targeting",
      targeting: {
        targetPositions: ["vanguard", "midguard", "rearguard"],
        targetScope: "enemy",
        selectRule: "random",
        targetCount: { base: 1, max: 1 }
      },
      dispatchEquipmentHooks: false,
      amountFormula: {
        baseAdd: 50,
        terms: [
          { attrKey: "intelligence", coefficient: 0.55, perEnhanceLevel: 0.0125 },
          { attrKey: "equipment_enhance_level", coefficient: 4 }
        ]
      }
    })

    const crownDamage = getEquipmentTemplateDefinition("cat_ancestor_forbidden_seal_crown")
      ?.battleHooks[0]?.effects.find((effect) => effect.kind === "damage")
    expect(crownDamage).toMatchObject({
      amountFormula: {
        terms: [
          { attrKey: "magical_damage", coefficient: 1.4, perEnhanceLevel: 0.035 },
          { attrKey: "equipment_enhance_level", coefficient: 10 }
        ]
      }
    })

    const starText = buildBattleHookDescriptions([starHook!], t)[0]?.effects[0]
    expect(starText?.kind).toBe("text")
    if (starText?.kind === "text") {
      expect(starText.text).toContain("(0.55 + 0.0125×强化等级)×智力")
      expect(starText.text).toContain("优先随机")
      expect(starText.text).not.toContain("最近")
    }
  })

  it("keeps the four +40 attribute profiles distinct and restrained", () => {
    const expectedFinalAttrs = {
      holy_cat_revenge_star_staff: { intelligence: 54.6, mp_regen: 2.765 },
      prayer_ash_venom_censer: { magical_defence: 60.45, magical_damage_resist: 0.0415 },
      cat_ancestor_forbidden_seal_crown: { magical_damage: 51.2, magic_crit_damage: 0.1845 },
      volatile_spore_fuse_ring: { perception: 73.2, speed: 13.85 }
    } as const

    for (const [id, expected] of Object.entries(expectedFinalAttrs)) {
      const template = getEquipmentTemplateDefinition(id)
      expect(template).toBeTruthy()
      if (!template) continue
      const enhanced = buildEnhanceCumulativeAttrs(template, 40)
      const finalAttrs = Object.fromEntries(Object.keys(expected).map((attrKey) => [
        attrKey,
        Number(template.baseAttrs[attrKey] || 0) + Number(enhanced[attrKey] || 0)
      ]))
      for (const [attrKey, value] of Object.entries(expected)) {
        expect(finalAttrs[attrKey]).toBeCloseTo(value, 6)
      }
    }
  })

  it("describes probabilistic cleanse and source-owned debuff verdict without exposing DSL tokens", () => {
    const censer = getEquipmentTemplateDefinition("prayer_ash_venom_censer")
    const cleanseDescription = buildBattleHookDescriptions(censer?.battleHooks ?? [], t)[0]
    const cleanse = cleanseDescription?.effects[0]
    expect(cleanse?.kind).toBe("text")
    if (cleanse?.kind === "text") {
      expect(cleanse.text).toContain("负面状态")
      expect(cleanse.text).toContain("仅可驱散")
      expect(cleanse.text).toContain("最高75%")
      expect(cleanse.text).not.toContain("remove_buff_by_kind")
    }

    const crown = getEquipmentTemplateDefinition("cat_ancestor_forbidden_seal_crown")
    const crownDescription = buildBattleHookDescriptions(crown?.battleHooks ?? [], t)[0]
    expect(crownDescription?.conditions.map((entry) => entry.text)).toContain("至少3种自身来源负面状态")
  })

  it("uses linear all-material enhancement costs and predecessor recipes", () => {
    const predecessorByAction = {
      craft_holy_cat_revenge_star_staff: "moonwood_staff",
      craft_prayer_ash_venom_censer: "holy_cat_incense_censer",
      sewing_cat_ancestor_forbidden_seal_crown: "lunar_veil_pointed_hat",
      craft_volatile_spore_fuse_ring: "moss_sleep_ring"
    } as const

    for (const [actionId, predecessor] of Object.entries(predecessorByAction)) {
      expect(getActionDefinition(actionId)?.costs[predecessor]).toBe(1)
    }
    for (const id of EQUIPMENT_IDS) {
      const curve = getEquipmentTemplateDefinition(id)?.enhanceCostCurve
      expect(curve?.overrides).toEqual({})
      expect(Object.keys(curve?.base ?? {}).length).toBeGreaterThanOrEqual(4)
      for (const materialId of Object.keys(curve?.base ?? {})) {
        expect(curve?.base[materialId]).toBe(1)
        expect(curve?.step[materialId]).toBe(1)
      }
    }
  })
})
