import assert from "node:assert/strict"
import fs from "node:fs"
import test from "node:test"

const readTable = (name) => {
  const parsed = JSON.parse(fs.readFileSync(new URL(`../source/json/${name}.json`, import.meta.url), "utf8"))
  return parsed[name]
}

const equipment = new Map(readTable("equipment_templates").map((row) => [row.id, row]))
const skills = new Map(readTable("skills").map((row) => [row.id, row]))
const buffs = new Map(readTable("buff").map((row) => [row.id, row]))
const buffEffects = new Map(readTable("buff_effects").map((row) => [row.id, row]))
const battleItems = new Map(readTable("battle_usable_items").map((row) => [row.id, row]))
const generatedSkills = new Map(
  JSON.parse(fs.readFileSync(new URL("../generated/chunks/skills.json", import.meta.url), "utf8"))
    .skills.map((row) => [row.id, row])
)
const generatedBattleItems = new Map(
  JSON.parse(fs.readFileSync(new URL("../generated/chunks/battle_usable_items.json", import.meta.url), "utf8"))
    .battle_usable_items.map((row) => [row.id, row])
)

const hook = (itemID, hookID) => equipment.get(itemID)?.battleHooks?.find((row) => row.id === hookID)

test("probability prizes convert outcomes into bounded tactical actions", () => {
  assert.equal(hook("jackpot_biased_token", "jackpot_biased_token_print_ticket").maxTriggersPerTurn, 1)
  assert.equal(hook("jackpot_biased_token", "jackpot_biased_token_print_ticket").effects[0].buffId, "jackpot_payout_ticket")
  assert.equal(buffs.get("jackpot_payout_ticket").maxCharges, 1)
  assert.deepEqual(buffEffects.get("jackpot_payout_ticket_probability").effects, [{ kind: "set_probability", value: 1 }])
  assert.equal(buffEffects.get("jackpot_payout_ticket_probability").priority, -1000, "payout must settle after additive chance modifiers")
  assert.deepEqual(buffEffects.get("jackpot_payout_ticket_redeem_success").effects, [{ kind: "consume_charge" }])
  assert.deepEqual(buffEffects.get("jackpot_payout_ticket_redeem_failure").effects, [{ kind: "consume_charge" }])

  assert.equal(hook("sugarcoat_cheat_pin", "sugarcoat_cheat_pin_harden_shell").effects[0].buffId, "sugar_shell")
  assert.equal(buffs.get("sugar_shell").maxCharges, 1)
  assert.deepEqual(buffEffects.get("sugar_shell_break").effects, [
    { kind: "set_damage_multiplier", value: 0 },
    { kind: "consume_charge" }
  ])

  const paperRedeem = hook("consolation_prize_paper_shield", "consolation_prize_paper_shield_redeem")
  assert.deepEqual(paperRedeem.conditions, [{ type: "self_buff_count_gte", buffId: "consolation_prize_guard", value: 2 }])
  assert.deepEqual(paperRedeem.effects.map((effect) => effect.kind), ["remove_buff_by_kind", "remove_buff"])

  const encore = hook("full_house_applause_crown", "full_house_applause_crown_encore")
  assert.equal(encore.maxTriggersPerBattle, undefined)
  assert.deepEqual(encore.conditions, [
    { type: "self_buff_count_gte", buffId: "full_house_applause", value: 3 },
    { type: "self_buff_count_eq", buffId: "full_house_applause_cooldown", value: 0 }
  ])
  assert.deepEqual(encore.effects.map((effect) => effect.kind), ["remove_buff", "add_buff", "extra_turn"])
  assert.deepEqual(encore.effects[1], {
    kind: "add_buff",
    buffId: "full_house_applause_cooldown",
    durationTurns: 3,
    targetMode: "self"
  })
  assert.equal(buffs.get("full_house_applause_cooldown").kind, "buff")
  assert.equal(buffs.get("full_house_applause_cooldown").dispellable, false)

  const emptyBeat = hook("hollow_heart", "hollow_heart_empty_beat")
  assert.equal(emptyBeat.conditions[0].value, 20)
  assert.deepEqual(emptyBeat.effects.map((effect) => effect.kind), ["remove_buff_by_kind", "remove_buff", "extra_turn"])

  const exit = hook("haunted_house_exit_ticket", "haunted_house_exit_ticket_first_failure")
  assert.equal(exit.maxTriggersPerBattle, undefined)
  assert.equal(exit.maxTriggersPerWave, 1)
  assert.deepEqual(exit.conditions, [{ type: "self_buff_count_eq", buffId: "haunted_exit_route_cooldown", value: 0 }])
  assert.deepEqual(exit.effects.map((effect) => effect.kind), ["add_buff", "add_buff", "extra_turn"])
  assert.deepEqual(exit.effects[1], {
    kind: "add_buff",
    buffId: "haunted_exit_route_cooldown",
    durationTurns: 3,
    targetMode: "self"
  })
  assert.equal(buffs.get("haunted_exit_route_cooldown").kind, "debuff")
  assert.equal(buffs.get("haunted_exit_route_cooldown").dispellable, false)
  assert.deepEqual(buffEffects.get("haunted_exit_route_while_active").effects, [
    { kind: "position_override", position: "rearguard", priority: 80 }
  ])
})

test("amusement relics rewrite the battle rhythm instead of only adding stats", () => {
  const opening = hook("king_of_amusement_park", "king_of_amusement_park_opening_parade")
  const payload = opening.effects[0].summonPayload
  assert.equal(opening.maxTriggersPerBattle, undefined)
  assert.equal(payload.unitId, "candy_clown_001")
  assert.equal(payload.side, "player")
  assert.equal(payload.countFormula.terms[0].coefficient, 0.05)
  assert.equal(payload.maxCountFormula.terms[0].coefficient, 0.05)
  assert.equal(payload.durationFormula.terms[0].coefficient, 0.05)
  const curtainCall = hook("king_of_amusement_park", "king_of_amusement_park_curtain_call")
  assert.deepEqual(curtainCall.subjectFilter, {
    includeUnitTypes: ["summon"],
    includeSummonTemplateIds: ["candy_clown_001"]
  })
  assert.equal(curtainCall.maxTriggersPerBattle, 2)
  assert.deepEqual(curtainCall.conditions, [{ type: "equipment_enhance_level_gte", value: 40 }])
  assert.deepEqual(curtainCall.activationCost, { maxHpRatio: 0.1, maxMpRatio: 0.15 })
  assert.equal(curtainCall.cooldownCompletedTurns, 2)
  assert.deepEqual(curtainCall.effects, [{ kind: "extra_turn", targetMode: "self", turnMode: "full" }])

  assert.equal(buffEffects.get("parade_applause_while_active").effects[0].kind, "position_map")
  assert.equal(buffEffects.get("parade_applause_while_active").effects[0].mapping.rearguard, "midguard")
  assert.equal(buffEffects.get("parade_shelter_while_active").effects[0].mapping.vanguard, "midguard")

  const empress = equipment.get("empress_cloak")
  assert.deepEqual(empress.battleHooks.map((row) => row.id), ["empress_cloak_enthrone", "empress_cloak_command"])
  const uniqueWearerCondition = [{
    type: "player_party_equipped_unit_count_eq",
    itemId: "empress_cloak",
    value: 1
  }]
  const empressEnthrone = hook("empress_cloak", "empress_cloak_enthrone")
  assert.equal(empressEnthrone.trigger, "on_battle_start")
  assert.equal(empressEnthrone.maxTriggersPerBattle, 1)
  assert.deepEqual(empressEnthrone.conditions, uniqueWearerCondition)
  assert.deepEqual(empressEnthrone.effects, [{
    kind: "add_buff",
    buffId: "empress_cloak_enthroned",
    durationTurns: 0,
    targetMode: "self",
    dispatchHooks: false
  }])
  const empressCommand = hook("empress_cloak", "empress_cloak_command")
  assert.equal(empressCommand.trigger, "on_wave_start")
  assert.equal(empressCommand.maxTriggersPerBattle, undefined)
  assert.deepEqual(empressCommand.conditions, uniqueWearerCondition)
  assert.deepEqual(empressCommand.effects.map((effect) => effect.kind), ["add_buff"])
  assert.equal(empressCommand.effects[0].buffId, "empress_command")
  assert.equal(empressCommand.effects[0].targeting.targetScope, "ally")
  assert.equal(empressCommand.effects[0].targeting.excludeSelf, true)
  assert.deepEqual(empressCommand.effects[0].payloadFormula.mainStatPercent, {
    baseAdd: 0.08,
    terms: [{ attrKey: "equipment_enhance_level", coefficient: 0.002, perSkillLevel: 0 }]
  })
  assert.equal(buffs.get("empress_cloak_enthroned").permanent, true)
  assert.equal(buffs.get("empress_cloak_enthroned").dispellable, false)
  assert.equal(buffs.get("empress_cloak_enthroned").showInBattleCard, false)
  assert.deepEqual(buffEffects.get("empress_cloak_enthroned_while_active").effects, [{ kind: "turn_ineligible" }])
  const repriseHook = hook("encore_echo_mask", "encore_echo_mask_reprise")
  const reprise = repriseHook.effects[0]
  assert.deepEqual(repriseHook.activationCost, { maxHpRatio: 0.1, maxMpRatio: 0.15 })
  assert.equal(repriseHook.cooldownCompletedTurns, 2)
  assert.equal(reprise.kind, "mirror_cast_skill")
  assert.equal(reprise.triggerScope, "ally")
  assert.equal(reprise.amountRatioBase, 0.4)
  assert.equal(reprise.amountRatioPerEnhanceLevel, 0.0025)
  assert.equal(reprise.mpCostMultiplier, 0)
  assert.equal(reprise.hpCostMultiplier, 0)
  assert.equal(reprise.allowMirroredCastTrigger, false)
  assert.equal(reprise.dispatchHooks, false)

  const tailLight = buffEffects.get("last_parade_light_catch_lethal")
  assert.deepEqual(tailLight.conditions, [
    { type: "incoming_damage_would_be_lethal" },
    { type: "self_charges_remaining_gte", value: 1 }
  ])
  assert.equal(buffs.get("last_parade_light").maxCharges, 1)
})

test("amusement enemies and active items keep readable non-numeric counterplay", () => {
  assert.deepEqual(skills.get("starlight_group_mend").castGuards, [{ kind: "ally_side_has_non_full_hp" }])
  assert.ok(skills.get("carousel_roundabout_lock").effects[1].statusPayload.statusIds.includes("carousel_reverse_position"))
  assert.ok(buffEffects.get("ghostly_rider_preyscent_while_active").effects.some((effect) => effect.kind === "position_override" && effect.position === "vanguard"))
  assert.deepEqual(buffEffects.get("midnight_punched_stamp_third_strike").effects, [{ kind: "skill_action_suppressed" }])
  const paradeCarnivalStatus = skills.get("parade_king_team_carnival").effects.find((effect) => effect.id === "parade_king_team_carnival_status")
  assert.equal(paradeCarnivalStatus.targeting.selectRule, "self")
  assert.deepEqual(buffEffects.get("parade_frenzy_sync_clowns").effects, [
    { kind: "trigger_synced_summon_turn", includeSummonTemplateIds: ["candy_clown_001"] }
  ])
  assert.equal(skills.get("parade_king_call_clowns").effects[0].summonPayload.side, "enemy")
  assert.equal(generatedSkills.get("parade_king_call_clowns").effects[0].summonPayload.side, "enemy")
  assert.deepEqual(generatedSkills.get("starlight_group_mend").castGuards, [{ kind: "ally_side_has_non_full_hp" }])
  assert.ok(buffEffects.get("parade_royal_aegis_while_active").effects.some((effect) => effect.kind === "damage_redirect_to_source" && effect.percent === 0.7))
  assert.deepEqual(buffEffects.get("plush_stunt_double_role_trap").effects.map((effect) => effect.kind), ["add_buff", "consume_charge"])
  assert.equal(buffs.has("parade_king_fervor"), false)
  assert.equal(buffs.has("parade_king_guard"), false)
  assert.equal(buffEffects.has("parade_king_fervor_while_active"), false)
  assert.equal(buffEffects.has("parade_king_guard_while_active"), false)

  assert.equal(battleItems.get("battle_item_expired_candy_salute").effects[1].kind, "weighted_random_per_target")
  assert.equal(battleItems.get("battle_item_plush_stunt_double_box").effects[0].kind, "summon")
  assert.equal(generatedBattleItems.get("battle_item_plush_stunt_double_box").effects[0].summonPayload.side, "source")
  assert.equal(battleItems.get("battle_item_mirror_house_reflection_candy").effects[0].statusPayload.statusIds[0], "mirror_house_reversal")
  assert.equal(battleItems.get("battle_item_reverse_carousel_spring").effects[0].statusPayload.statusIds[0], "carousel_reverse_position")
})
