import { describe, expect, it } from "vitest"
import gameData from "../../packages/game-data/generated/game-data.json"
import zhCN from "../src/locales/game/zh-CN"
import { getActionDefinition } from "../src/services/action-definition"
import { buildBattleHookDescriptions } from "../src/services/battle-hook-display"
import { buildEnhanceCumulativeAttrs } from "../src/services/equipment-enhance"
import { getEquipmentTemplateDefinition, getItemDefinition } from "../src/services/item-definition"

function t(key: string, params?: Record<string, unknown>): string {
  const fixed: Record<string, string> = {
    "buff.five_vow_answered_prayer.name": "应愿香火",
    "buff.five_vow_echo_cooldown.name": "回响余烬"
  }
  if (fixed[key]) return fixed[key]
  if (key === "game.itemDetailBattleHookConditionIncomingHealGte") {
    return `本次对该友方的实际治疗不低于 ${String(params?.value)}`
  }
  if (key === "game.itemDetailBattleHookConditionSelfBuffCount") {
    return `自身「${String(params?.buff)}」层数 ${String(params?.op)} ${String(params?.value)}`
  }
  return key
}

describe("five-vow echo censer content chain", () => {
  it("connects the two-part rumor to the temple boss", () => {
    const prequel = gameData.cat_village_commissions.find((row) => row.id === "cvc_rumor_vow_eater_prequel")
    const finale = gameData.cat_village_commissions.find((row) => row.id === "cvc_rumor_vow_eater")
    const trigger = gameData.battle_usable_items.find((row) => row.id === "trigger_vow_eater_incense_bundle")
    const boss = gameData.enemies.find((row) => row.id === "vow_eater_001")

    expect(prequel?.costs).toEqual({
      holy_cat_blessed_fur: { min: 120, max: 120 },
      holy_cat_incense: { min: 80, max: 80 },
      holy_cat_sage_scroll: { min: 30, max: 30 }
    })
    expect(prequel?.rewardPool).toMatchObject({ vow_eater_prayer_rubbing: { min: 1, max: 1 } })
    expect(finale?.prereqRumorIds).toContain("cvc_rumor_vow_eater_prequel")
    expect(finale?.costs).toEqual({
      vow_eater_prayer_rubbing: { min: 1, max: 1 },
      holy_cat_guardian_medal: { min: 80, max: 80 },
      holy_cat_sage_scroll: { min: 30, max: 30 },
      holy_cat_royal_jewel: { min: 8, max: 8 }
    })
    expect(finale?.rewardPool).toMatchObject({ vow_eater_incense_bundle: { min: 1, max: 1 } })
    expect(finale?.clearCompletionIds).toEqual(expect.arrayContaining([
      "cvc_rumor_vow_eater_prequel",
      "cvc_rumor_vow_eater"
    ]))
    expect(trigger?.dungeonIds).toContain("holy_cat_temple_001")
    expect(trigger?.spawnEnemyIds).toContain("vow_eater_001")
    expect(boss?.drops.congealed_vow_ember).toMatchObject({ percent: 0.2, range: { min: 1, max: 1 } })
  })

  it("treats the boss core as a full mythic alchemy material", () => {
    const core = gameData.items.find((row) => row.id === "congealed_vow_ember")
    const trait = gameData.item_traits.find((row) => row.itemId === "congealed_vow_ember")
    const forgeModule = gameData.skill_alchemy_modules.find((row) => row.itemId === "congealed_vow_ember")

    expect(core).toMatchObject({
      type: ["loot"],
      rarity: "mythic",
      category: "alchemy_material",
      alchemy: { convertExcluded: true }
    })
    expect(trait?.traits[0]).toMatchObject({ family: "arcane", unitBasis: 5 })
    expect(forgeModule).toMatchObject({
      nodeKind: "formula",
      unitBasis: 15,
      compileOp: { kind: "formula_term", effectKind: "heal", attrKey: "perception" }
    })
  })

  it("uses the core for crafting and every mythic enhancement milestone", () => {
    const item = getItemDefinition("five_vow_echo_censer")
    const action = getActionDefinition("craft_five_vow_echo_censer")
    const template = getEquipmentTemplateDefinition("five_vow_echo_censer")
    const milestoneLevels = ["10", "20", "30", "40", "50"]

    expect(item?.rarity).toBe("mythic")
    expect(item?.economyBaseValue).toBe(82000)
    expect(action?.rarity).toBe("mythic")
    expect(action?.costs).toMatchObject({
      congealed_vow_ember: 1,
      holy_cat_incense: 24,
      holy_cat_blessed_fur: 24,
      holy_cat_sage_scroll: 12,
      holy_cat_royal_jewel: 4
    })

    expect(template?.enhanceMaxLevel).toBe(50)
    expect(Object.keys(template?.enhanceCostCurve.overrides ?? {}).sort()).toEqual(milestoneLevels)
    expect(Object.keys(template?.enhanceAttrCurve.overrides ?? {}).sort()).toEqual(milestoneLevels)
    const totalMilestoneCores = milestoneLevels.reduce(
      (total, level) => total + Number(template?.enhanceCostCurve.overrides[level]?.congealed_vow_ember ?? 0),
      0
    )
    expect(totalMilestoneCores).toBe(8)

    const maxAttrs = buildEnhanceCumulativeAttrs(template, 50)
    expect(maxAttrs.intelligence).toBeCloseTo(96.3)
    expect(maxAttrs.perception).toBeCloseTo(84)
    expect(maxAttrs.mp_regen).toBeCloseTo(5.87)
    expect(template?.battleHooks).toContainEqual(expect.objectContaining({
      id: "five_vow_echo_awakened_wave_start",
      trigger: "on_wave_start",
      conditions: expect.arrayContaining([
        expect.objectContaining({ type: "equipment_enhance_level_gte", value: 50 })
      ])
    }))
  })

  it("localizes actual-healing DSL and gates the extra action with a cooldown buff", () => {
    const template = getEquipmentTemplateDefinition("five_vow_echo_censer")
    const cooldown = gameData.buff.find((buff) => buff.id === "five_vow_echo_cooldown")
    const recordHook = template?.battleHooks.find((hook) => hook.id === "five_vow_echo_record_heal")
    const ringHook = template?.battleHooks.find((hook) => hook.id === "five_vow_echo_ring")

    expect(zhCN.game.itemDetailBattleHookConditionIncomingHealGte).toContain("实际治疗")
    expect(cooldown).toMatchObject({
      durationTurns: 2,
      permanent: false,
      dispellable: false,
      stackPolicy: "refresh_duration",
      showInBattleCard: true
    })
    expect(ringHook?.conditions).toContainEqual({
      type: "self_buff_count_eq",
      buffId: "five_vow_echo_cooldown",
      value: 0
    })
    expect(ringHook?.effects.map((effect) => effect.kind)).toEqual([
      "remove_buff",
      "add_buff",
      "extra_turn"
    ])
    expect(ringHook?.effects[1]).toMatchObject({
      buffId: "five_vow_echo_cooldown",
      durationTurns: 2,
      targetMode: "self"
    })

    const description = buildBattleHookDescriptions(recordHook ? [recordHook] : [], t)[0]
    expect(description?.conditions[0]?.text).toBe("本次对该友方的实际治疗不低于 1")
    expect(description?.conditions[0]?.text).not.toContain("incoming_heal_gte")
  })
})
