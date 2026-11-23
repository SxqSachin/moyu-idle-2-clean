import { describe, expect, it } from "vitest"
import { getEquipmentTemplateDefinition } from "../src/services/item-definition"
import { buildBattleHookDescriptions } from "../src/services/battle-hook-display"

function t(key: string, params?: Record<string, unknown>): string {
  if (key === "attr.strength.name") return "力量"
  if (key === "attr.dexterity.name") return "敏捷"
  if (key === "attr.speed.name") return "速度"
  if (key === "attr.physics_damage.name") return "物理攻击"
  if (key === "attr.perception.name") return "感知"
  if (key === "game.itemDetailBattleHookAttrEquipmentEnhanceLevel") return "装备强化等级"
  if (key === "game.damageType.physical") return "物理"
  if (key === "game.skillTargetScope.enemy") return "敌方"
  if (key === "game.targetDirection.vanguard") return "前排"
  if (key === "game.targetSeekStrategy.front_to_rear") return "前排 -> 后排"
  if (key === "game.skillSelectRule.first") return "首位"
  if (key === "game.itemDetailBattleHookTargetingRule") {
    return `${String(params?.scope)}中的${String(params?.positions)}目标（${String(params?.seek)}，优先${String(params?.select)}，数量${String(params?.count)}）`
  }
  if (key === "game.itemDetailBattleHookEffectDamage") return `对${String(params?.target ?? "")}造成${String(params?.damageType ?? "")}伤害：${String(params?.formula ?? "")}`
  if (key === "game.itemDetailBattleHookEffectProcChance") return `触发概率：${String(params?.chance ?? "")}`
  if (key === "game.itemDetailBattleHookEffectProcChanceWithMax") {
    return `${String(params?.chance ?? "")} (max ${String(params?.max ?? "")})`
  }
  if (key === "game.itemDetailBattleHookEffectProcChanceClamped") {
    return `min ${String(params?.min ?? "")}, max ${String(params?.max ?? "")}, ${String(params?.expr ?? "")}`
  }
  if (key === "game.itemDetailBattleHookTargetMode.self") return "自己"
  if (key === "game.itemDetailBattleHookEffectSummonDuration") return `持续 ${String(params?.turns ?? "")} 回合`
  if (key === "game.itemDetailBattleHookEffectSummonCount") return `数量 ${String(params?.count ?? "")}`
  if (key === "game.itemDetailBattleHookEffectSummonLimit") return `数量上限 ${String(params?.count ?? "")}`
  if (key === "game.itemDetailBattleHookEffectSummonLimitFormula") return `数量上限 ${String(params?.count ?? "")}（向下取整）`
  return key
}

describe("battle hook proc chance display", () => {
  it("keeps add_buff procChance fields after equipment template normalization", () => {
    const template = getEquipmentTemplateDefinition("forbidden_magic_bow")
    expect(template).toBeTruthy()
    const descriptions = buildBattleHookDescriptions(template?.battleHooks ?? [], t)
    const silenceHook = descriptions.find((node) => node.key === "forbidden_magic_bow_silence_on_hit")
    expect(silenceHook).toBeTruthy()
    const addBuffEffect = silenceHook?.effects.find((effect) => effect.kind === "add_buff")
    expect(addBuffEffect && addBuffEffect.kind === "add_buff" ? addBuffEffect.procChanceText : null).toContain("33%")
    expect(addBuffEffect && addBuffEffect.kind === "add_buff" ? addBuffEffect.procChanceText : null).toContain("85%")
    expect(addBuffEffect && addBuffEffect.kind === "add_buff" ? addBuffEffect.procChanceText : null).toContain("感知")
  })

  it("displays equipment enhance level in hook chance formulas", () => {
    const template = getEquipmentTemplateDefinition("frostspore_scale_shield")
    expect(template).toBeTruthy()
    const descriptions = buildBattleHookDescriptions(template?.battleHooks ?? [], t)
    const freezeHook = descriptions.find((node) => node.key === "frostspore_scale_shield_freeze_after_deal_damage")
    const addBuffEffect = freezeHook?.effects.find((effect) => effect.kind === "add_buff")
    const text = addBuffEffect && addBuffEffect.kind === "add_buff" ? addBuffEffect.procChanceText : null
    expect(text).toContain("33%")
    expect(text).toContain("装备强化等级×0.4%")
    expect(text).toContain("70%")
  })

  it("keeps summon hook display concise and structured for enemy quick refs", () => {
    const template = getEquipmentTemplateDefinition("frostspore_scale_shield")
    expect(template).toBeTruthy()
    const descriptions = buildBattleHookDescriptions(template?.battleHooks ?? [], t)
    const summonHook = descriptions.find((node) => node.key === "frostspore_scale_shield_slime_after_take_damage")
    const effect = summonHook?.effects.find((entry) => entry.kind === "summon")
    expect(effect?.kind).toBe("summon")
    if (!effect || effect.kind !== "summon") return
    expect(effect.unitId).toBe("ice_slime_001")
    expect(effect.durationText).toContain("2")
    expect(effect.countText).toBe("数量上限 1")
    expect(effect.procChanceText).toContain("18%")
    expect(effect.procChanceText).toContain("装备强化等级×0.2%")
    expect(JSON.stringify(effect)).not.toContain("策略")
    expect(JSON.stringify(effect)).not.toContain("floor(1)")
  })

  it("displays enhancement-scaled summon limits from the generic formula", () => {
    const template = getEquipmentTemplateDefinition("stellar_testament_pendant")
    expect(template).toBeTruthy()
    const descriptions = buildBattleHookDescriptions(template?.battleHooks ?? [], t)
    const summonHook = descriptions.find((node) => node.key === "stellar_testament_unit_death")
    const effect = summonHook?.effects.find((entry) => entry.kind === "summon")
    expect(effect?.kind).toBe("summon")
    if (!effect || effect.kind !== "summon") return
    expect(effect.countText).toContain("数量上限")
    expect(effect.countText).toContain("6")
    expect(effect.countText).toContain("0.1×装备强化等级")
    expect(effect.countText).toContain("向下取整")
  })

  it("splits summon stat bonuses by attribute and hides fixed duration formulas", () => {
    const descriptions = buildBattleHookDescriptions([
      {
        id: "summon_with_bonus",
        trigger: "on_after_take_damage",
        effects: [
          {
            kind: "summon",
            targetMode: "self",
            summonPayload: {
              unitId: "ice_slime_001",
              durationTurns: 3,
              durationFormula: { baseAdd: 3, terms: [] },
              statsFormula: {
                hp: { baseAdd: 0, terms: [{ attrKey: "equipment_enhance_level", coefficient: 15 }] },
                physics_damage: { baseAdd: 0, terms: [{ attrKey: "perception", coefficient: 0.25 }] }
              }
            }
          }
        ]
      }
    ], t)
    const effect = descriptions[0]?.effects[0]
    expect(effect?.kind).toBe("summon")
    if (!effect || effect.kind !== "summon") return
    expect(effect.durationText).toBe("持续 3 回合")
    expect(effect.statBonusEntries).toEqual([
      { key: "hp", label: "hp", value: "+15×装备强化等级" },
      { key: "physics_damage", label: "物理攻击", value: "+0.25×感知" }
    ])
  })

  it("describes damage hook effects instead of falling back to unknown text", () => {
    const descriptions = buildBattleHookDescriptions([
      {
        id: "opening_stab",
        trigger: "on_battle_start",
        effects: [
          {
            kind: "damage",
            targetMode: "targeting",
            damageType: "physical",
            targeting: {
              targetPositions: ["vanguard"],
              secondarySeek: "front_to_rear",
              targetScope: "enemy",
              selectRule: "first",
              targetCount: { base: 1, formula: [], max: 1 }
            },
            amountFormula: {
              baseAdd: 8,
              terms: [
                { attrKey: "dexterity", coefficient: 0.28 },
                { attrKey: "speed", coefficient: 0.12 },
                { attrKey: "physics_damage", coefficient: 0.25 }
              ]
            }
          }
        ]
      }
    ], t)
    const effect = descriptions[0]?.effects[0]
    expect(effect?.kind).toBe("text")
    if (!effect || effect.kind !== "text") return
    expect(effect.text).toContain("对敌方中的前排目标（前排 -> 后排，优先首位，数量1）造成物理伤害")
    expect(effect.text).toContain("8 + 0.28×敏捷")
    expect(effect.text).not.toContain("未知效果")
  })
})
