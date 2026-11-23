import assert from "node:assert/strict"
import fs from "node:fs"
import test from "node:test"

const readTable = (name) => {
  const parsed = JSON.parse(fs.readFileSync(new URL(`../source/json/${name}.json`, import.meta.url), "utf8"))
  return parsed[name]
}

const byID = (name) => new Map(readTable(name).map((row) => [row.id, row]))

const items = byID("items")
const commissions = byID("cat_village_commissions")
const battleItems = byID("battle_usable_items")
const enemies = byID("enemies")
const skills = byID("skills")
const buffs = byID("buff")
const effects = byID("buff_effects")
const equipment = readTable("equipment_templates")

test("faceless understudy rumor advances through a repeatable two-stage invitation", () => {
  const clue = commissions.get("cvc_rumor_faceless_understudy_playbill")
  const invitation = commissions.get("cvc_rumor_faceless_understudy_invitation")

  assert.equal(clue.isRumor, true)
  assert.equal(clue.oncePerAccount, false)
  assert.equal(clue.notDeletable, undefined)
  assert.deepEqual(clue.prereqRumorIds, [])
  assert.deepEqual(clue.rewardPool, { smudged_encore_playbill: { min: 1, max: 1 } })

  assert.equal(invitation.isRumor, true)
  assert.equal(invitation.oncePerAccount, false)
  assert.equal(invitation.notDeletable, true)
  assert.deepEqual(invitation.prereqRumorIds, ["cvc_rumor_faceless_understudy_playbill"])
  assert.deepEqual(invitation.clearCompletionIds, [
    "cvc_rumor_faceless_understudy_playbill",
    "cvc_rumor_faceless_understudy_invitation"
  ])
  assert.deepEqual(invitation.rewardPool, { faceless_understudy_invitation: { min: 1, max: 1 } })
  assert.equal(invitation.costs.parade_king_medal.min, 8)
  assert.equal(invitation.costs.spell_seal_lock_plate.min, 4)
  assert.equal(invitation.costs.holy_cat_royal_jewel.min, 2)
})

test("invitation creates one deterministic park encounter for the party", () => {
  const invitationItem = items.get("faceless_understudy_invitation")
  const trigger = battleItems.get("trigger_faceless_understudy_invitation")

  assert.equal(invitationItem.battleUsableItemId, trigger.id)
  assert.equal(invitationItem.carryableInCombat, true)
  assert.equal(invitationItem.iconId, "item.faceless_understudy_invitation")
  assert.deepEqual(trigger.dungeonIds, ["amusement_park_001"])
  assert.deepEqual(trigger.spawnEnemyIds, ["faceless_understudy_001"])
  assert.equal(trigger.minWave, 17)
  assert.equal(trigger.intervalWaves, 8)
  assert.equal(trigger.perWaveChance, 1)
  assert.equal(trigger.partyTriggerGroup, "amusement_faceless_understudy")
})

test("faceless understudy is a gated two-phase boss with deterministic mask progress", () => {
  const boss = enemies.get("faceless_understudy_001")
  assert.equal(boss.type, "boss")
  assert.ok(boss.level > enemies.get("midnight_ticket_collector_001").level)
  assert.deepEqual(boss.initialBuffIds, ["faceless_understudy_three_curtains", "faceless_understudy_last_bow"])
  assert.deepEqual(boss.skillIds.slice(0, 2), ["faceless_understudy_stolen_bow", "faceless_understudy_finale_debt"])
  assert.deepEqual(boss.drops.faceless_understudy_mask, { percent: 1, range: { min: 1, max: 1 } })

  assert.equal(items.get("faceless_understudy_mask").iconId, "item.faceless_understudy_mask")
  assert.equal(buffs.get("faceless_understudy_three_curtains").maxCharges, 3)
  assert.equal(buffs.get("faceless_understudy_three_curtains").dispellable, true)
  assert.deepEqual([
    effects.get("faceless_understudy_three_curtains_block"),
    effects.get("faceless_understudy_three_curtains_block_second_act"),
    effects.get("faceless_understudy_three_curtains_block_final_act")
  ].map((rule) => ({
    remaining: rule.conditions.find((condition) => condition.type === "self_charges_remaining_gte")?.value,
    multiplier: rule.effects.find((effect) => effect.kind === "set_damage_multiplier")?.value
  })), [
    { remaining: 3, multiplier: 0.01 },
    { remaining: 2, multiplier: 0.35 },
    { remaining: 1, multiplier: 0.5 }
  ])

  const lastBow = effects.get("faceless_understudy_last_bow_rewrite")
  assert.deepEqual(lastBow.conditions, [
    { type: "incoming_damage_would_be_lethal" },
    { type: "self_charges_remaining_gte", value: 1 }
  ])
  assert.deepEqual(lastBow.effects.map((effect) => effect.kind), ["set_damage_multiplier", "heal", "add_buff", "consume_charge"])

  assert.equal(skills.get("faceless_understudy_stolen_bow").effects[1].statusPayload.statusIds[0], "faceless_understudy_forgotten_cue")
  assert.equal(skills.get("faceless_understudy_finale_debt").effects[1].statusPayload.statusIds[0], "faceless_understudy_stage_debt")
})

test("understudy mask remains a recipe gate instead of a linear enhancement tax", () => {
  for (const template of equipment) {
    const curve = template.enhanceCostCurve || {}
    assert.equal(Object.hasOwn(curve.base || {}, "faceless_understudy_mask"), false, `${template.id} base enhancement cost`)
    assert.equal(Object.hasOwn(curve.step || {}, "faceless_understudy_mask"), false, `${template.id} step enhancement cost`)
    for (const [level, costs] of Object.entries(curve.overrides || {})) {
      assert.equal(Object.hasOwn(costs || {}, "faceless_understudy_mask"), false, `${template.id} +${level} enhancement cost`)
    }
  }
})
