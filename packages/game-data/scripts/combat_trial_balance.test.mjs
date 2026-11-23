import assert from "node:assert/strict"
import fs from "node:fs"
import test from "node:test"

const readJSON = (relativePath) => JSON.parse(fs.readFileSync(new URL(relativePath, import.meta.url), "utf8"))
const formats = readJSON("../source/json/combat_trial_formats.json").combat_trial_formats
const metrics = readJSON("../source/json/combat_trial_metrics.json").combat_trial_metrics
const buffs = readJSON("../source/json/buff.json").buff
const buffEffects = readJSON("../source/json/buff_effects.json").buff_effects
const worlds = readJSON("../source/json/world.json").world
const enemies = readJSON("../source/json/enemies.json").enemies
const globalConfig = readJSON("../../game-config/source/global-config.json")
const sharedConfig = readJSON("../../game-config/source/shared-global-config.json")

const difficultyMultiplier = new Map(
  sharedConfig.combatDifficulty.levels.map((row) => [row.level, row.statMultiplierCurve.base])
)
const worldByID = new Map(worlds.map((row) => [row.id, row]))
const enemyByID = new Map(enemies.map((row) => [row.id, row]))
const derivedFormulaByAttr = new Map(globalConfig.derivedStats.formulas.map((row) => [row.targetAttr, row]))

function flatBonuses(worldID, level) {
  return Object.fromEntries(worldByID.get(worldID).difficultySettings.statBonuses.map((row) => [
    row.attrKey,
    Math.round(row.base * (1 + row.perLevelMult * (level - 1)) * 10000) / 10000
  ]))
}

function projectedStat(enemyID, attrKey) {
  const enemy = enemyByID.get(enemyID)
  const formula = derivedFormulaByAttr.get(attrKey)
  assert.ok(enemy)
  assert.ok(formula)
  let value = formula.includeBase ? Number(enemy[attrKey] || 0) : 0
  for (const term of formula.terms) value += Number(enemy[term.attrKey] || 0) * Number(term.coefficient || 0)
  return value
}

test("gauntlet owns explicit N2/N5/N10/N15/N20 enemy stat profiles", () => {
  const gauntlet = formats.find((row) => row.id === "gauntlet")
  assert.ok(gauntlet)
  assert.equal(gauntlet.balanceRevision, 4)

  const waves = gauntlet.encounter.waves
  assert.deepEqual(waves.map((wave) => wave.enemyStatProfile.difficultyLevel), [2, 5, 10, 15, 20])
  assert.deepEqual(waves.map((wave) => wave.enemyStatProfile.multiplier), [
    difficultyMultiplier.get(2),
    difficultyMultiplier.get(5),
    difficultyMultiplier.get(10),
    1 + 0.05 * 15 ** 2,
    1 + 0.05 * 20 ** 2
  ])
  assert.deepEqual(waves.map((wave) => wave.enemyStatProfile.flatBonuses), [
    flatBonuses("base_2", 2),
    flatBonuses("base_2", 5),
    flatBonuses("base_3", 10),
    flatBonuses("base_3", 15),
    flatBonuses("base_3", 20)
  ])

  assert.deepEqual(waves.map((wave) => wave.units.map((unit) => unit.statOverrides.hp)), [
    [14000, 15000, 12000],
    [30000, 34000, 40000],
    [90000, 70000, 65000],
    [240000, 210000, 200000],
    [1250000, 1100000]
  ])
  assert.deepEqual(waves.map((wave) => wave.units.map((unit) => Math.max(
    unit.statOverrides.physics_damage,
    unit.statOverrides.magical_damage
  ))), [
    [500, 650, 550],
    [1100, 1600, 1600],
    [2500, 1800, 2100],
    [5200, 4400, 6500],
    [9000, 12000]
  ])

  const library = worldByID.get("forbidden_library_001")
  const libraryN10HPBonus = library.difficultySettings.statBonuses
    .find((row) => row.attrKey === "hp")
  const libraryBossN10HP = projectedStat("forbidden_library_keeper_boss_001", "hp") * difficultyMultiplier.get(10)
    + libraryN10HPBonus.base * (1 + libraryN10HPBonus.perLevelMult * 9)
  const finalBosses = waves.at(-1).units
  assert.ok(Math.abs(libraryBossN10HP - 190518.918) <= 1e-6)
  assert.ok(finalBosses.every((unit) => unit.statOverrides.hp >= 1000000))
  assert.ok(finalBosses.every((unit) => unit.statOverrides.hp >= libraryBossN10HP * 5))

  const defenseK = sharedConfig.combatBalanceModel.defenseK
  const finalResist = waves.at(-1).enemyStatProfile.flatBonuses.physics_damage_resist
  const combinedReduction = (defense) => 1 - (1 - defense / (defenseK + defense)) * (1 - finalResist)
  assert.deepEqual(finalBosses.map((unit) => [
    Math.round(combinedReduction(unit.statOverrides.physics_defence) * 10000) / 10000,
    Math.round(combinedReduction(unit.statOverrides.magical_defence) * 10000) / 10000
  ]), [
    [0.8276, 0.7845],
    [0.7845, 0.8563]
  ])
  assert.ok(waves.flatMap((wave) => wave.units).every((unit) => unit.statOverrides.crit_damage_resist <= 0.8))
})

test("gauntlet ranks full clears by fewer elapsed player-side main turns", () => {
  const gauntlet = formats.find((row) => row.id === "gauntlet")
  const elapsedTurns = metrics.find((row) => row.id === "actions_to_victory")
  assert.ok(gauntlet)
  assert.ok(elapsedTurns)
  assert.deepEqual(gauntlet.metricIds, ["waves_cleared", "actions_to_victory"])
  assert.equal(elapsedTurns.valueSource, "player_main_actions")
  assert.equal(elapsedTurns.rankDirection, "asc")
  assert.ok(elapsedTurns.eligibleEndReasons.includes("all_waves_cleared"))
})

test("recovery owns a permanent initial deterioration buff and three passive targets", () => {
  const recovery = formats.find((row) => row.id === "recovery")
  const deterioration = buffs.find((row) => row.id === "combat_trial_patient_deterioration")
  const deteriorationHook = buffEffects.find((row) => row.id === "combat_trial_patient_deterioration_on_turn_start")
  assert.ok(recovery)
  assert.ok(deterioration)
  assert.ok(deteriorationHook)
  assert.equal(recovery.balanceRevision, 2)
  assert.deepEqual(recovery.triggers, [])

  const patient = recovery.encounter.units.find((unit) => unit.id === "wounded_patient")
  assert.ok(patient)
  assert.equal(patient.maxHp, 100_000_000)
  assert.equal(patient.initialHpRatio, 0.5)
  assert.deepEqual(patient.initialBuffs, [{
    buffId: "combat_trial_patient_deterioration",
    payload: { max_hp_loss_ratio: 0.05 }
  }])
  assert.equal(deterioration.permanent, true)
  assert.equal(deterioration.dispellable, false)
  assert.equal(deteriorationHook.trigger, "on_turn_start")
  assert.deepEqual(deteriorationHook.effects, [{
    kind: "resource",
    resource: "hp",
    value: 0,
    payloadValueKey: "max_hp_loss_ratio",
    payloadValueCoeff: -1,
    valueScale: "max_resource",
    minimumHp: 1
  }])

  const passiveTargets = recovery.encounter.units.filter((unit) => unit.side !== "player")
  assert.equal(passiveTargets.length, 3)
  assert.deepEqual(passiveTargets.map((unit) => unit.position), ["vanguard", "midguard", "rearguard"])
  assert.ok(passiveTargets.every((unit) => unit.behavior === "passive" && unit.healthMode === "immutable_full" && unit.objectiveTarget === false))
})
