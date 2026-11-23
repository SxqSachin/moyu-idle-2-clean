import assert from "node:assert/strict"
import fs from "node:fs"
import test from "node:test"

const readTable = (name) => {
  const parsed = JSON.parse(fs.readFileSync(new URL(`../source/json/${name}.json`, import.meta.url), "utf8"))
  return parsed[name]
}

const enemiesByID = new Map(readTable("enemies").map((row) => [row.id, row]))
const worldsByID = new Map(readTable("world").map((row) => [row.id, row]))
const buffsByID = new Map(readTable("buff").map((row) => [row.id, row]))
const effectsByID = new Map(readTable("buff_effects").map((row) => [row.id, row]))
const skillsByID = new Map(readTable("skills").map((row) => [row.id, row]))
const globalConfig = JSON.parse(fs.readFileSync(new URL("../../game-config/source/global-config.json", import.meta.url), "utf8"))
const sharedGlobalConfig = JSON.parse(fs.readFileSync(new URL("../../game-config/source/shared-global-config.json", import.meta.url), "utf8"))

const derivedFormulaByAttr = new Map(globalConfig.derivedStats.formulas.map((formula) => [formula.targetAttr, formula]))

function projectedStat(enemyID, attrKey) {
  const enemy = enemiesByID.get(enemyID)
  assert.ok(enemy, `${enemyID} must exist`)
  const formula = derivedFormulaByAttr.get(attrKey)
  assert.ok(formula, `${attrKey} formula must exist`)
  let value = formula.includeBase ? Number(enemy[attrKey] || 0) : 0
  for (const term of formula.terms) value += Number(enemy[term.attrKey] || 0) * Number(term.coefficient || 0)
  return value
}

function projectedSkillAmount(enemyID, skillID, effectID) {
  const enemy = enemiesByID.get(enemyID)
  const skill = skillsByID.get(skillID)
  assert.ok(enemy, `${enemyID} must exist`)
  assert.ok(skill, `${skillID} must exist`)
  const effect = skill.effects.find((row) => row.id === effectID)
  assert.ok(effect, `${effectID} must exist`)
  const formula = effect.amountFormula
  return Number(formula.baseAdd || 0) + formula.terms.reduce((total, term) => {
    const attrValue = derivedFormulaByAttr.has(term.attrKey)
      ? projectedStat(enemyID, term.attrKey)
      : Number(enemy[term.attrKey] || 0)
    return total + attrValue * Number(term.coefficient || 0)
  }, 0)
}

function dominantOffense(enemyID) {
  return Math.max(projectedStat(enemyID, "physics_damage"), projectedStat(enemyID, "magical_damage"))
}

function averageDefense(enemyID) {
  return (projectedStat(enemyID, "physics_defence") + projectedStat(enemyID, "magical_defence")) / 2
}

function median(values) {
  const ordered = [...values].sort((a, b) => a - b)
  const middle = Math.floor(ordered.length / 2)
  return ordered.length % 2 === 1 ? ordered[middle] : (ordered[middle - 1] + ordered[middle]) / 2
}

function effectKinds(ruleID) {
  const row = effectsByID.get(ruleID)
  assert.ok(row, `${ruleID} must exist`)
  return row.effects.map((effect) => effect.kind)
}

test("late dungeons preserve health and offense while defensive difficulty scaling is softened", () => {
  const expectedDifficulty = {
    meteor_corridor_001: [
      ["hp", 1023, 1.3],
      ["physics_defence", 10, 0.25],
      ["magical_defence", 10, 0.25],
      ["physics_damage", 76.26, 0.58],
      ["magical_damage", 76.26, 0.58],
      ["speed", 64, 0.55]
    ],
    forbidden_library_001: [
      ["hp", 1335, 1.35],
      ["physics_defence", 12, 0.27],
      ["magical_defence", 14, 0.28],
      ["physics_damage", 85.44, 0.62],
      ["magical_damage", 99.68, 0.65],
      ["speed", 72, 0.58]
    ]
  }
  for (const [worldID, rows] of Object.entries(expectedDifficulty)) {
    assert.deepEqual(
      worldsByID.get(worldID).difficultySettings.statBonuses,
      rows.map(([attrKey, base, perLevelMult]) => ({ attrKey, base, perLevelMult }))
    )
  }

  const meteorStarburst = worldsByID.get("meteor_corridor_001").battlefieldEffects
    .find((effect) => effect.id === "meteor_corridor_starburst")
    .triggerEffects.find((effect) => effect.kind === "damage")
  assert.deepEqual(meteorStarburst.amountByDifficulty, { base: 3520, perLevelMult: 0.17 })

  const correctedEarlyDefense = {
    base_4: {
      physics_defence: { base: 15, perLevelMult: 0.4 },
      magical_defence: { base: 18, perLevelMult: 0.4 }
    },
    holy_cat_temple_001: {
      physics_defence: { base: 20, perLevelMult: 0.4 },
      magical_defence: { base: 20, perLevelMult: 0.4 }
    }
  }
  for (const [worldID, expectedByAttr] of Object.entries(correctedEarlyDefense)) {
    const bonusByAttr = new Map(worldsByID.get(worldID).difficultySettings.statBonuses.map((row) => [row.attrKey, row]))
    for (const [attrKey, expected] of Object.entries(expectedByAttr)) {
      assert.deepEqual(bonusByAttr.get(attrKey), { attrKey, ...expected })
    }
  }

  const baselineProfiles = {
    astral_guard_001: [5185.2, 141.05, 27, 79.65, 62, 27],
    astral_mage_001: [5260.6, 27.4, 193.3, 36, 86.15, 14.85],
    starlight_healer_001: [5417, 24, 163, 53.55, 139.65, 11.7],
    refraction_mirror_doll_001: [5699, 29, 209.1, 38.1, 91.55, 15.75],
    meteor_core_porter_001: [10169.8, 228.2, 43.1, 106.8, 86.85, 61.35],
    deranged_calibrator_001: [10524.2, 41.8, 300.9, 55.8, 133.45, 22.95],
    corridor_timekeeper_cat_boss_001: [13805, 95.75, 422.7, 56.1, 179.55, 60],
    silence_tome_001: [6400.4, 34.6, 203.1, 45.9, 109.8, 18.9],
    brass_book_guard_001: [7200.2, 84.05, 54.9, 149.7, 123.9, 24.75],
    living_bookmark_001: [7000, 143.25, 70.9, 55.5, 81.5, 97.65],
    watcher_owl_001: [7199.6, 45.15, 226.9, 42.75, 113.65, 31.95],
    binding_librarian_001: [13984, 88.5, 218.5, 143.85, 197.3, 33.75],
    banished_spellcat_001: [15080, 62, 334.5, 52.2, 167.1, 56.4],
    forbidden_library_keeper_boss_001: [32390.2, 88.3, 476.5, 74.1, 238.1, 78]
  }
  const meteorEnemies = new Set([
    ...worldsByID.get("meteor_corridor_001").enemyIds.split(","),
    ...worldsByID.get("meteor_corridor_001").specialEnemyIds.split(",")
  ])
  const attrs = ["hp", "physics_damage", "magical_damage"]
  for (const [enemyID, baseline] of Object.entries(baselineProfiles)) {
    const factor = meteorEnemies.has(enemyID) ? 0.93 : 0.89
    for (let index = 0; index < attrs.length; index += 1) {
      assert.ok(
        Math.abs(projectedStat(enemyID, attrs[index]) - baseline[index] * factor) <= 1e-6,
        `${enemyID} ${attrs[index]} staged reduction`
      )
    }
  }

  const guardrails = sharedGlobalConfig.combatDifficulty.authoringGuardrails
  const difficultyLevels = sharedGlobalConfig.combatDifficulty.levels
  const difficultyMultipliers = difficultyLevels.map((row) => row.statMultiplierCurve.base)
  const expectedRewardBonuses = [0.1, 0.15, 0.2, 0.3, 0.45, 0.6, 0.8, 1.05, 1.3, 1.6]
  const minDifficultyMultiplier = Math.min(...difficultyMultipliers)
  const maxDifficultyMultiplier = Math.max(...difficultyMultipliers)
  assert.deepEqual(difficultyMultipliers, [1.15, 1.35, 1.65, 2.1, 2.6, 3.15, 3.8, 4.5, 5.2, 6])
  assert.deepEqual(difficultyLevels.map((row) => row.battleExpBonus), expectedRewardBonuses)
  assert.deepEqual(difficultyLevels.map((row) => row.battleDropRateBonus), expectedRewardBonuses)
  for (const enemy of enemiesByID.values()) {
    const type = enemy.type
    const bossLike = type === "boss" || type === "world_boss"
    const defenseGuarded = type in guardrails.defenseBaseCapsByEnemyType
      && (enemy.level >= guardrails.minEnemyLevel || bossLike)
    const critGuarded = type in guardrails.critDamageResistBaseCapsByEnemyType
      && type in guardrails.critDamageResistBaseFloorsByEnemyType
      && (enemy.level >= guardrails.minEnemyLevel || guardrails.critDamageResistBaseFloorsByEnemyType[type] > 0)
    if (defenseGuarded) {
      assert.ok(projectedStat(enemy.id, "physics_defence") <= guardrails.defenseBaseCapsByEnemyType[type] + 1e-6)
      assert.ok(projectedStat(enemy.id, "magical_defence") <= guardrails.defenseBaseCapsByEnemyType[type] + 1e-6)
    }
    if (critGuarded) {
      const projectedCritResist = projectedStat(enemy.id, "crit_damage_resist")
      assert.ok(projectedCritResist >= guardrails.critDamageResistBaseFloorsByEnemyType[type] - 1e-6)
      assert.ok(projectedCritResist <= guardrails.critDamageResistBaseCapsByEnemyType[type] + 1e-6)
      assert.ok(
        projectedCritResist * maxDifficultyMultiplier <= 0.850002 + 1e-6,
        `${enemy.id} maximum-difficulty crit resistance`
      )
      if (bossLike) {
        assert.ok(
          projectedCritResist * minDifficultyMultiplier
            >= guardrails.critDamageResistBaseFloorsByEnemyType[type] * minDifficultyMultiplier - 1e-6,
          `${enemy.id} must retain base boss crit resistance at the lowest difficulty`
        )
      }
    }
  }
  const lateWorldIDs = ["meteor_corridor_001", "forbidden_library_001", "amusement_park_001"]
  for (const worldID of lateWorldIDs) {
    const world = worldsByID.get(worldID)
    const enemyIDs = new Set([...world.enemyIds.split(","), ...world.specialEnemyIds.split(",")])
    const bonusByAttr = new Map(world.difficultySettings.statBonuses.map((row) => [
      row.attrKey,
      row.base * (1 + row.perLevelMult * (world.difficultySettings.maxLevel - 1))
    ]))
    for (const enemyID of enemyIDs) {
      const enemy = enemiesByID.get(enemyID)
      const type = enemy.type
      const defenseCap = guardrails.defenseBaseCapsByEnemyType[type]
      const critCap = guardrails.critDamageResistBaseCapsByEnemyType[type]
      assert.ok(projectedStat(enemyID, "physics_defence") <= defenseCap + 1e-6, `${enemyID} physical defense authoring cap`)
      assert.ok(projectedStat(enemyID, "magical_defence") <= defenseCap + 1e-6, `${enemyID} magical defense authoring cap`)
      assert.ok(projectedStat(enemyID, "crit_damage_resist") <= critCap + 1e-6, `${enemyID} crit resistance authoring cap`)
      assert.ok(
        projectedStat(enemyID, "crit_damage_resist") * maxDifficultyMultiplier <= 0.8 + 1e-6,
        `${enemyID} maximum-difficulty crit resistance`
      )
      for (const attrKey of ["physics_defence", "magical_defence"]) {
        const finalValue = projectedStat(enemyID, attrKey) * maxDifficultyMultiplier + (bonusByAttr.get(attrKey) || 0)
        assert.ok(finalValue <= defenseCap * maxDifficultyMultiplier + (bonusByAttr.get(attrKey) || 0) + 1e-6)
      }
    }
  }

  const preservedDirectStats = {
    starlight_healer_001: { mp_regen: 2 },
    meteor_core_porter_001: { speed: 6 },
    corridor_timekeeper_cat_boss_001: { speed: 6 },
    banished_spellcat_001: { speed: 6 },
    forbidden_library_keeper_boss_001: { speed: 6 },
    stardust_mouse_001: { speed: 22 },
    carousel_cat_001: { speed: 12 },
    candy_clown_001: { speed: 16 },
    plush_guard_001: { speed: 5 },
    starlight_healer_002: { speed: 12, mp: 360 },
    ghostly_rider_001: { speed: 24, crit_chance: 0.09 },
    parade_king_boss_001: { speed: 18, crit_chance: 0.06 },
    midnight_ticket_collector_001: { speed: 18, crit_chance: 0.07 },
    faceless_understudy_001: { speed: 28, crit_chance: 0.1, mp: 1200 }
  }
  for (const [enemyID, expected] of Object.entries(preservedDirectStats)) {
    for (const [attrKey, value] of Object.entries(expected)) {
      assert.equal(enemiesByID.get(enemyID)[attrKey], value, `${enemyID} explicit ${attrKey}`)
    }
  }
})

test("Star-Night Amusement Park follows Forbidden Library and scales above it at every difficulty", () => {
  const worldRows = readTable("world")
  const libraryIndex = worldRows.findIndex((row) => row.id === "forbidden_library_001")
  const amusementIndex = worldRows.findIndex((row) => row.id === "amusement_park_001")
  assert.equal(amusementIndex, libraryIndex + 1, "amusement park must immediately follow Forbidden Library")

  const difficulty = (worldID) => new Map(worldsByID.get(worldID).difficultySettings.statBonuses.map((row) => [row.attrKey, row]))
  const amusement = difficulty("amusement_park_001")
  const library = difficulty("forbidden_library_001")
  const expectedAmusementDifficulty = {
    hp: { base: 1402.5, perLevelMult: 1.4, level10: 19074 },
    physics_defence: { base: 14, perLevelMult: 0.3, level10: 51.8 },
    magical_defence: { base: 16, perLevelMult: 0.3, level10: 59.2 },
    physics_damage: { base: 88.4, perLevelMult: 0.66, level10: 613.496 },
    magical_damage: { base: 103.7, perLevelMult: 0.69, level10: 747.677 },
    speed: { base: 78, perLevelMult: 0.61, level10: 506.22 }
  }
  for (const [attrKey, expected] of Object.entries(expectedAmusementDifficulty)) {
    const row = amusement.get(attrKey)
    const libraryRow = library.get(attrKey)
    assert.deepEqual(row, { attrKey, base: expected.base, perLevelMult: expected.perLevelMult })
    const level10 = row.base * (1 + row.perLevelMult * 9)
    const libraryLevel10 = libraryRow.base * (1 + libraryRow.perLevelMult * 9)
    assert.ok(Math.abs(level10 - expected.level10) <= 1e-9, `${attrKey} level 10 bonus`)
    assert.ok(row.base >= libraryRow.base * 1.01, `${attrKey} level 1 must exceed Forbidden Library`)
    assert.ok(level10 >= libraryLevel10 * 1.05, `${attrKey} level 10 must exceed Forbidden Library`)
  }
})

test("amusement enemies use final derived stats above each Forbidden Library tier", () => {
  assert.equal(enemiesByID.get("midnight_ticket_collector_001")?.type, "elite")
  assert.equal(enemiesByID.get("plush_stunt_double_001")?.type, "normal")
  const expectedStats = {
    stardust_mouse_001: [235, 7650, 90.525, 276.25, 6.579, 15.9375, 100.4125],
    carousel_cat_001: [240, 9350, 201.2375, 64.175, 13.3875, 11.6535, 63.6375],
    candy_clown_001: [245, 8500, 88.485, 317.475, 7.3746, 18.768, 77.2],
    plush_guard_001: [250, 12750, 168.9375, 64.175, 18.7425, 16.626, 31.775],
    starlight_healer_002: [255, 8075, 58.31, 368.475, 6.9921, 20, 50.25],
    ghostly_rider_001: [260, 17000, 320.025, 181.9, 16.83, 21.063, 150.225],
    parade_king_boss_001: [275, 42500, 223.125, 510, 20.349, 30, 102.15],
    midnight_ticket_collector_001: [280, 51000, 467.5, 288.15, 25, 25, 132.75],
    faceless_understudy_001: [290, 107400.9, 370.6, 768.4, 30, 30, 158.05]
  }
  const attrs = ["hp", "physics_damage", "magical_damage", "physics_defence", "magical_defence", "speed"]
  for (const [enemyID, [level, ...expected]] of Object.entries(expectedStats)) {
    assert.equal(enemiesByID.get(enemyID).level, level, `${enemyID} level`)
    for (let index = 0; index < attrs.length; index += 1) {
      assert.ok(Math.abs(projectedStat(enemyID, attrs[index]) - expected[index]) <= 1e-6, `${enemyID} ${attrs[index]}`)
    }
  }

  const libraryNormals = ["silence_tome_001", "brass_book_guard_001", "living_bookmark_001", "watcher_owl_001"]
  const amusementNormals = ["stardust_mouse_001", "carousel_cat_001", "candy_clown_001", "plush_guard_001", "starlight_healer_002"]
  const libraryElites = ["binding_librarian_001", "banished_spellcat_001"]
  assert.ok(Math.min(...amusementNormals.map((id) => projectedStat(id, "hp"))) >= Math.max(...libraryNormals.map((id) => projectedStat(id, "hp"))) * 1.15)
  assert.ok(median(amusementNormals.map(dominantOffense)) >= median(libraryNormals.map(dominantOffense)) * 1.45)
  assert.ok(median(amusementNormals.map(averageDefense)) >= median(libraryNormals.map(averageDefense)) * 1.25)
  assert.ok(median(amusementNormals.map((id) => projectedStat(id, "speed"))) >= median(libraryNormals.map((id) => projectedStat(id, "speed"))) * 1.75)

  assert.ok(projectedStat("ghostly_rider_001", "hp") >= Math.max(...libraryElites.map((id) => projectedStat(id, "hp"))) * 1.2)
  assert.ok(dominantOffense("ghostly_rider_001") >= Math.max(...libraryElites.map(dominantOffense)) * 1.03)
  assert.ok(averageDefense("ghostly_rider_001") >= Math.max(...libraryElites.map(averageDefense)) * 1.03)
  assert.ok(projectedStat("ghostly_rider_001", "speed") >= Math.max(...libraryElites.map((id) => projectedStat(id, "speed"))) * 1.2)

  const libraryBoss = "forbidden_library_keeper_boss_001"
  assert.ok(projectedStat("parade_king_boss_001", "hp") >= projectedStat(libraryBoss, "hp") * 1.4)
  assert.ok(dominantOffense("parade_king_boss_001") >= dominantOffense(libraryBoss) * 1.15)
  assert.ok(averageDefense("parade_king_boss_001") >= averageDefense(libraryBoss) * 1.4)
  assert.ok(projectedStat("parade_king_boss_001", "speed") >= projectedStat(libraryBoss, "speed") * 1.35)

  assert.ok(projectedStat("midnight_ticket_collector_001", "hp") > projectedStat("parade_king_boss_001", "hp"))
  assert.ok(averageDefense("midnight_ticket_collector_001") < averageDefense("parade_king_boss_001"))
  assert.ok(projectedStat("midnight_ticket_collector_001", "speed") > projectedStat("parade_king_boss_001", "speed"))
  for (const metric of ["hp", "speed"]) {
    assert.ok(projectedStat("faceless_understudy_001", metric) > projectedStat("midnight_ticket_collector_001", metric))
  }
  assert.ok(dominantOffense("faceless_understudy_001") > dominantOffense("parade_king_boss_001"))
  assert.ok(averageDefense("faceless_understudy_001") > averageDefense("midnight_ticket_collector_001"))

  const stuntDouble = enemiesByID.get("plush_stunt_double_001")
  assert.equal(stuntDouble.level, 1, "runtime-scaled summon template must not receive static dungeon stats")
  assert.deepEqual(stuntDouble.initialBuffIds, ["plush_stunt_double_role"])
})

test("amusement damage pressure uses readable row attacks and dispellable monster-only resistance pierce", () => {
  const libraryNormalDamage = [
    ["silence_tome_001", "silence_tome_hushing_hex", "silence_tome_hushing_hex_damage"],
    ["brass_book_guard_001", "brass_book_guard_bash", "brass_book_guard_bash_damage"],
    ["living_bookmark_001", "living_bookmark_hand_seal", "living_bookmark_hand_seal_damage"],
    ["watcher_owl_001", "watcher_owl_audit_strike", "watcher_owl_audit_strike_damage"]
  ].map((args) => projectedSkillAmount(...args))
  const amusementNormalDamage = {
    stardust_mana_pickpocket_damage: projectedSkillAmount("stardust_mouse_001", "stardust_mana_pickpocket", "stardust_mana_pickpocket_damage"),
    carousel_roundabout_lock_damage: projectedSkillAmount("carousel_cat_001", "carousel_roundabout_lock", "carousel_roundabout_lock_damage"),
    candy_clown_double_prank_damage: projectedSkillAmount("candy_clown_001", "candy_clown_double_prank", "candy_clown_double_prank_damage")
  }
  const expectedAmusementNormalDamage = {
    stardust_mana_pickpocket_damage: 425.475,
    carousel_roundabout_lock_damage: 358.051875,
    candy_clown_double_prank_damage: 483.378
  }
  for (const [effectID, expected] of Object.entries(expectedAmusementNormalDamage)) {
    assert.ok(Math.abs(amusementNormalDamage[effectID] - expected) <= 1e-9, `${effectID} projected amount`)
  }
  assert.ok(Math.min(...Object.values(amusementNormalDamage)) >= Math.max(...libraryNormalDamage) * 1.15)

  const libraryEliteDamage = [
    ["binding_librarian_001", "binding_librarian_lock", "binding_librarian_lock_damage"],
    ["binding_librarian_001", "binding_librarian_press", "binding_librarian_press_damage"],
    ["banished_spellcat_001", "banished_spellcat_flood", "banished_spellcat_flood_damage"],
    ["banished_spellcat_001", "banished_spellcat_echo", "banished_spellcat_echo_damage"]
  ].map((args) => projectedSkillAmount(...args))
  assert.ok(Math.abs(projectedSkillAmount("ghostly_rider_001", "ghostly_rider_lowest_chase", "ghostly_rider_lowest_chase_damage") - 721.03375) <= 1e-9)
  assert.ok(721.03375 >= Math.max(...libraryEliteDamage) * 1.8)

  const libraryBossDamage = [
    projectedSkillAmount("forbidden_library_keeper_boss_001", "library_keeper_gag_order", "library_keeper_gag_order_damage"),
    projectedSkillAmount("forbidden_library_keeper_boss_001", "library_keeper_final_suppression", "library_keeper_final_suppression_damage")
  ]
  assert.equal(projectedSkillAmount("parade_king_boss_001", "parade_king_team_carnival", "parade_king_team_carnival_damage"), 765)
  assert.ok(765 > Math.max(...libraryBossDamage))
  assert.equal(projectedSkillAmount("midnight_ticket_collector_001", "midnight_ticket_punch", "midnight_ticket_punch_damage"), 745.875)
  assert.ok(Math.abs(projectedSkillAmount("faceless_understudy_001", "faceless_understudy_stolen_bow", "faceless_understudy_stolen_bow_damage") - 1355.58) <= 1e-9)
  assert.ok(Math.abs(projectedSkillAmount("faceless_understudy_001", "faceless_understudy_finale_debt", "faceless_understudy_finale_debt_damage") - 1984.92) <= 1e-9)

  const rowContracts = {
    carousel_roundabout_lock: ["carousel_roundabout_lock_damage", "vanguard"],
    candy_clown_double_prank: ["candy_clown_double_prank_damage", "midguard"],
    parade_king_team_carnival: ["parade_king_team_carnival_damage", "vanguard"],
    midnight_ticket_punch: ["midnight_ticket_punch_damage", "vanguard"],
    faceless_understudy_stolen_bow: ["faceless_understudy_stolen_bow_damage", "midguard"]
  }
  for (const [skillID, [effectID, position]] of Object.entries(rowContracts)) {
    const skill = skillsByID.get(skillID)
    const effect = skill.effects.find((row) => row.id === effectID)
    assert.equal(skill.enemyOnly, true, `${skillID} target-priority bypass must remain monster-only`)
    assert.deepEqual(effect.targeting.targetPositions, [position], `${skillID} row`)
    assert.equal(effect.targeting.secondarySeek, "none", `${skillID} must not spill into another row`)
    assert.equal(effect.targeting.ignoreTargetPriority, true, `${skillID} row must not be replaced by taunt or preferred-target ordering`)
    assert.equal(effect.targeting.targetCount.base, 999, `${skillID} must cover the occupied row`)
  }

  for (const [skillID, effectID] of [
    ["carousel_roundabout_lock", "carousel_roundabout_lock_status"],
    ["candy_clown_double_prank", "candy_clown_double_prank_weighted_random_per_target"],
    ["midnight_ticket_punch", "midnight_ticket_punch_status"],
    ["faceless_understudy_stolen_bow", "faceless_understudy_stolen_bow_status"]
  ]) {
    const companion = skillsByID.get(skillID).effects.find((row) => row.id === effectID)
    assert.equal(companion.targeting.ignoreTargetPriority, true, `${effectID} must preserve the same row target set`)
  }

  const pierceContracts = {
    candy_clown_double_prank: ["candy_clown_double_prank_damage", 0.3, ["candy_poison", "candy_frozen", "candy_silence", "candy_burning"]],
    ghostly_rider_lowest_chase: ["ghostly_rider_lowest_chase_damage", 0.45, ["ghostly_rider_preyscent"]],
    parade_king_team_carnival: ["parade_king_team_carnival_damage", 0.35, ["amusement_spotlight"]],
    midnight_ticket_punch: ["midnight_ticket_punch_damage", 0.4, ["candy_sticky"]],
    faceless_understudy_stolen_bow: ["faceless_understudy_stolen_bow_damage", 0.35, ["faceless_understudy_forgotten_cue"]],
    faceless_understudy_finale_debt: ["faceless_understudy_finale_debt_damage", 0.55, ["faceless_understudy_stage_debt"]]
  }
  for (const [skillID, [effectID, ratio, statusIDs]] of Object.entries(pierceContracts)) {
    const skill = skillsByID.get(skillID)
    const effect = skill.effects.find((row) => row.id === effectID)
    assert.equal(skill.enemyOnly, true, `${skillID} must be monster-only`)
    assert.equal(effect.damageSpec.enemyOnlyDamageReductionPierce, ratio, `${skillID} pierce ratio`)
    assert.deepEqual(effect.damageSpec.enemyOnlyDamageReductionPierceTargetStatuses, statusIDs, `${skillID} readable marks`)
    for (const statusID of statusIDs) {
      const buff = buffsByID.get(statusID)
      assert.equal(buff.dispellable, true, `${statusID} must preserve cleanse counterplay`)
      assert.equal(buff.showInBattleCard, true, `${statusID} must remain visible`)
    }
  }

  const sourceItemsAndProducers = JSON.stringify({
    itemIDs: readTable("items").map((row) => row.id),
    actions: readTable("actions"),
    drops: readTable("enemies").map((row) => row.drops || {})
  })
  for (const skillID of new Set([...Object.keys(pierceContracts), ...Object.keys(rowContracts)])) {
    assert.equal(sourceItemsAndProducers.includes(`skillbook_${skillID}`), false, `${skillID} must have no player acquisition path`)
  }
})

test("curtains are dispellable ordered-turn mitigation with a readable burst window", () => {
  const curtainContracts = {
    amusement_opening_curtain: { charges: 1, multipliers: { 1: 0.5 } },
    amusement_vip_curtain: { charges: 2, multipliers: { 1: 0.5, 2: 0.35 } },
    parade_three_act_curtain: { charges: 3, multipliers: { 1: 0.5, 2: 0.35, 3: 0.01 } }
  }
  for (const [buffID, contract] of Object.entries(curtainContracts)) {
    const buff = buffsByID.get(buffID)
    assert.equal(buff.maxCharges, contract.charges, `${buffID} charges`)
    assert.equal(buff.dispellable, true, `${buffID} must retain purge counterplay`)
    const actualMultipliers = {}
    const blockRules = readTable("buff_effects").filter((row) => row.buffId === buffID && row.trigger === "on_before_take_damage" && row.effects.some((effect) => effect.kind === "set_damage_multiplier"))
    for (const rule of blockRules) {
      assert.ok(rule.priority >= 1000, `${rule.id} must resolve before expendable mitigation`)
      const atLeast = rule.conditions.find((condition) => condition.type === "self_charges_remaining_gte")?.value
      const atMost = rule.conditions.find((condition) => condition.type === "self_charges_remaining_lte")?.value
      const remaining = atLeast === atMost ? atLeast : contract.charges === 1 ? 1 : null
      assert.ok(remaining, `${rule.id} must identify one curtain act`)
      actualMultipliers[remaining] = rule.effects.find((effect) => effect.kind === "set_damage_multiplier").value
    }
    assert.deepEqual(actualMultipliers, contract.multipliers, `${buffID} layered direct-damage mitigation`)
  }

  for (const rule of readTable("buff_effects").filter((row) => row.buffId === "amusement_vip_curtain" || row.buffId === "parade_three_act_curtain" || row.id === "amusement_opening_curtain_finale")) {
    if (rule.trigger !== "on_turn_end") continue
    assert.deepEqual(rule.eventFilter, {
      turnKinds: ["ordered"],
      includePreparation: false,
      includeSuppressedTurns: true
    }, `${rule.id} turn contract`)
  }

  const vulnerability = buffsByID.get("amusement_curtain_fall_vulnerability")
  assert.equal(vulnerability.durationTurns, 1)
  assert.deepEqual(effectsByID.get("amusement_curtain_fall_vulnerability_before_damage").effects, [{ kind: "set_damage_multiplier", value: 1.25 }])
  assert.equal(buffsByID.get("amusement_spotlight").durationTurns, 1)
  assert.deepEqual(effectsByID.get("amusement_spotlight_while_active").effects, [{ kind: "prefer_as_enemy_target" }])
  const spotlightApplications = readTable("buff_effects").flatMap((rule) => rule.effects.filter((effect) => effect.kind === "add_buff" && effect.buffId === "amusement_spotlight"))
  assert.ok(spotlightApplications.length > 0)
  for (const effect of spotlightApplications) {
    assert.equal(effect.durationTurns, 1, "curtain Spotlight must stay brief")
  }
})

test("amusement roles keep distinct anti-burst counterplay", () => {
  const cover = effectsByID.get("plush_guard_cover_link_while_active")
  assert.deepEqual(cover.effects, [
    { kind: "reduce_damage_percent", value: 0.15 },
    { kind: "damage_redirect_to_source", percent: 0.55 }
  ])
  assert.deepEqual(effectKinds("starlight_backstage_pass_before_damage"), ["reduce_damage_percent", "consume_charge"])
  assert.equal(effectsByID.get("starlight_backstage_pass_before_damage").effects[0].value, 0.6)

  const tack = effectsByID.get("ghostly_rider_ethereal_tack_while_active").effects
  assert.deepEqual(tack, [
    { kind: "add_stat", attr: "physics_damage_resist", value: 0.55 },
    { kind: "add_stat", attr: "magical_damage_resist", value: -0.18 }
  ])

  const secondAct = effectsByID.get("parade_three_act_curtain_open_second_act")
  const finale = effectsByID.get("parade_three_act_curtain_open_finale")
  assert.deepEqual(effectKinds(secondAct.id), ["add_buff", "consume_charge"])
  assert.deepEqual(effectKinds(finale.id), ["add_buff", "consume_charge"])
  assert.equal(secondAct.effects[0].buffId, "parade_second_act")
  assert.equal(finale.effects[0].buffId, "parade_finale")

  const audit = effectsByID.get("midnight_audit_watch_mark_source")
  assert.equal(audit.maxTriggersPerTurn, 1)
  assert.equal(audit.effects[0].targetMode, "source")
  assert.equal(buffsByID.get("midnight_punched_stamp").maxStacks, 3)
})
