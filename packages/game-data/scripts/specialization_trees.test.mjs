import assert from "node:assert/strict"
import fs from "node:fs"
import test from "node:test"

import { validateSpecializationGrantSnapshotRetention, validateSpecializationMutationSemantics, validateSpecializationTrees, validateStableBattleEffectSourceIDs } from "./build.mjs"

const readTable = (name) => {
  const parsed = JSON.parse(fs.readFileSync(`packages/game-data/source/json/${name}.json`, "utf8"))
  return parsed[name]
}

const sourceTrees = readTable("specialization_trees")
const attributes = readTable("attributes")
const items = readTable("items")
const skills = readTable("skills")
const buffs = readTable("buff")
const buffEffects = readTable("buff_effects")
const equipmentTemplates = readTable("equipment_templates")

const cloneTrees = () => structuredClone(sourceTrees)
const validateTrees = (trees) => validateSpecializationTrees(trees, attributes, items, skills, buffs, equipmentTemplates)
const firstHook = (trees) => trees[0].nodes
  .flatMap((node) => node.levels[0].grants)
  .find((grant) => grant.kind === "battle_hook")
  .battleHooks[0]
const hookByTrigger = (trees, trigger) => trees[0].nodes
  .flatMap((node) => node.levels[0].grants)
  .filter((grant) => grant.kind === "battle_hook")
  .flatMap((grant) => grant.battleHooks)
  .find((hook) => hook.trigger === trigger)
const firstMutation = (trees) => trees[0].nodes
  .flatMap((node) => node.levels[0].grants)
  .find((grant) => grant.kind === "effect_mutation")
  .effectMutations[0]
const appendedEffects = (trees) => trees
  .flatMap((tree) => tree.nodes)
  .flatMap((node) => node.levels[0].grants)
  .flatMap((grant) => grant.effectMutations || [])
  .flatMap((mutation) => mutation.ops?.appendEffects || [])

const mutationDescriptors = (trees) => trees.flatMap((tree) => tree.nodes.flatMap((node) => node.levels[0].grants
  .filter((grant) => grant.kind === "effect_mutation")
  .flatMap((grant) => grant.effectMutations.map((mutation, mutationIndex) => ({ tree, node, grant, mutation, mutationIndex })))))

const mutateMutationSnapshots = (descriptor, mutate) => {
  for (const level of descriptor.node.levels) {
    const grant = level.grants.find((entry) => entry.id === descriptor.grant.id)
    const mutation = grant.effectMutations.find((entry) => entry.id === descriptor.mutation.id)
    mutate(mutation, level)
  }
}

const mutateFirstMutationSnapshots = (trees, mutate) => mutateMutationSnapshots(mutationDescriptors(trees)[0], mutate)

const appendConsumer = (descriptor, effectID, { priority = -1, includeGenerated = true, ops = { amountMultiplier: 1.1 } } = {}) => {
  const consumer = {
    id: `test_consumer_${descriptor.grant.effectMutations.length + 1}`,
    priority,
    scope: "both",
    effectSelector: { mode: "effect_id", effectIds: [effectID], includeGenerated },
    ops
  }
  for (const level of descriptor.node.levels) {
    const grant = level.grants.find((entry) => entry.id === descriptor.grant.id)
    grant.effectMutations.push(structuredClone(consumer))
  }
  return descriptor.grant.effectMutations.at(-1)
}

test("stable battle effect source IDs are lowercase, globally unique, and reject conflicting legacy aliases before normalization", () => {
  validateStableBattleEffectSourceIDs(skills, equipmentTemplates)
  assert.throws(() => validateStableBattleEffectSourceIDs([
    { id: "skill_a", effects: [{ id: "shared_damage", kind: "damage" }] },
    { id: "skill_b", effects: [{ id: "SHARED_DAMAGE", kind: "damage" }] }
  ], []), /lowercase letters/)
  assert.throws(() => validateStableBattleEffectSourceIDs([
    { id: "skill_a", effects: [{ id: "canonical_damage", effectId: "legacy_damage", kind: "damage" }] }
  ], []), /stable id mismatch/)
  assert.throws(() => validateStableBattleEffectSourceIDs([
    { id: "skill_a", effects: [{ id: "shared_damage", kind: "damage" }] }
  ], [
    { id: "weapon_a", basicAttackEffects: [{ id: "shared_damage", kind: "damage" }] }
  ]), /battle effect stable id duplicate/)
})

test("weapon specializations add at most one bounded target and never outnumber the matching weapon skill", () => {
  const tree = sourceTrees[0]
  const node = (id) => tree.nodes.find((entry) => entry.id === id)
  const maxMutation = (nodeID) => node(nodeID).levels.at(-1).grants
    .flatMap((grant) => grant.effectMutations || [])[0]

  const scythes = equipmentTemplates.filter((template) => template.weaponClass?.includes("scythe"))
  assert.ok(scythes.length > 0)
  assert.equal(scythes.flatMap((template) => template.basicAttackEffects || [])
    .every((effect) => (effect.targeting?.targetCount?.base || 1) === 1), true)
  const scytheSecondary = maxMutation("dexterity_terminal_reaping_sweep").ops.appendEffects[0]
  assert.deepEqual({ kind: scytheSecondary.kind, policy: scytheSecondary.targetPolicy, ratio: scytheSecondary.amountRatio }, {
    kind: "secondary_damage", policy: "same_row", ratio: 0.4
  })
  assert.equal("targetCount" in scytheSecondary, false)

  const spears = equipmentTemplates.filter((template) => template.weaponClass?.includes("spear"))
  assert.ok(spears.length > 0)
  for (const template of spears) {
    const damageEffects = (template.basicAttackEffects || []).filter((effect) => effect.kind === "damage")
    assert.deepEqual(damageEffects.map((effect) => effect.targeting.targetPositions[0]), ["vanguard", "midguard"], template.id)
    assert.equal(damageEffects.every((effect) => (effect.targeting.targetCount?.base || 1) === 1), true, template.id)
  }
  const spearSecondary = maxMutation("perception_terminal_foresight").ops.appendEffects[0]
  assert.deepEqual({ kind: spearSecondary.kind, policy: spearSecondary.targetPolicy, position: spearSecondary.targetPosition, ratio: spearSecondary.amountRatio }, {
    kind: "secondary_damage", policy: "fixed_row", position: "rearguard", ratio: 0.45
  })
  const spearSkill = skills.find((skill) => skill.id === "spear_line_pierce")
  assert.deepEqual(spearSkill.effects.filter((effect) => effect.kind === "damage").map((effect) => effect.targeting.targetPositions[0]), ["vanguard", "midguard", "rearguard"])
  assert.equal(spearSkill.effects.filter((effect) => effect.kind === "damage").every((effect) => effect.targeting.targetCount.base === 1), true)

  const longsword = maxMutation("confluence_strength_dexterity_swift_force")
  assert.equal(longsword.scope, "skill")
  assert.deepEqual(longsword.weaponClasses, ["sword"])
  assert.deepEqual(longsword.skillCategories, ["blade"])
  assert.equal(longsword.ops.appendEffects[0].targetPolicy, "all_rows")
  assert.equal(longsword.ops.appendEffects[0].amountRatio, 0.36)

  const greatblade = node("strength_terminal_titan_combo").levels.at(-1).grants[0].battleHooks[0]
  assert.equal(greatblade.limits.perTurn, 1)
  assert.equal(greatblade.conditions.some((condition) => condition.kind === "self_hp_ratio_lte" && condition.value === 0.55), true)
  assert.equal(greatblade.effects[0].kind, "runtime_damage_amp")
  assert.equal(greatblade.effects[0].value, 0.2)
  assert.doesNotMatch(JSON.stringify(greatblade), /hitCountDelta|targetCountDelta|repeat_context_effect/)
})

test("secondary-target DSL rejects oversized ratios and invalid row contracts", () => {
  const oversized = cloneTrees()
  mutationDescriptors(oversized).find(({ mutation }) => mutation.ops?.appendEffects?.some((effect) => effect.kind === "secondary_damage"))
    .mutation.ops.appendEffects.find((effect) => effect.kind === "secondary_damage").amountRatio = 0.5001
  assert.throws(() => validateTrees(oversized), /amountRatio invalid/)

  const missingRow = cloneTrees()
  const fixed = mutationDescriptors(missingRow).find(({ mutation }) => mutation.ops?.appendEffects?.some((effect) => effect.kind === "secondary_damage" && effect.targetPolicy === "fixed_row"))
    .mutation.ops.appendEffects.find((effect) => effect.kind === "secondary_damage" && effect.targetPolicy === "fixed_row")
  delete fixed.targetPosition
  assert.throws(() => validateTrees(missingRow), /targetPosition must be vanguard, midguard, or rearguard for fixed_row/)

  const strayRow = cloneTrees()
  mutationDescriptors(strayRow).find(({ mutation }) => mutation.ops?.appendEffects?.some((effect) => effect.kind === "secondary_damage" && effect.targetPolicy === "same_row"))
    .mutation.ops.appendEffects.find((effect) => effect.kind === "secondary_damage" && effect.targetPolicy === "same_row").targetPosition = "rearguard"
  assert.throws(() => validateTrees(strayRow), /targetPosition is only valid for fixed_row/)

  const stackedSecondary = cloneTrees()
  const stackedMutation = mutationDescriptors(stackedSecondary).find(({ mutation }) => mutation.ops?.appendEffects?.some((effect) => effect.kind === "secondary_damage")).mutation
  const secondary = stackedMutation.ops.appendEffects.find((effect) => effect.kind === "secondary_damage")
  stackedMutation.ops.appendEffects.push({ ...structuredClone(secondary), id: `${secondary.id}_extra` })
  assert.throws(() => validateTrees(stackedSecondary), /supports at most one secondary_damage/)

  for (const mutate of [
    (mutation) => { mutation.ops.targetCountDelta = 1 },
    (mutation) => { mutation.ops.hitCountDelta = 1 },
    (mutation) => { mutation.ops.appendEffects.push({ id: "secondary_with_splash", kind: "splash_damage", targetCount: 1, amountRatio: 0.2 }) }
  ]) {
    const expanded = cloneTrees()
    const mutation = mutationDescriptors(expanded).find(({ mutation: row }) => row.ops?.appendEffects?.some((effect) => effect.kind === "secondary_damage")).mutation
    mutate(mutation)
    assert.throws(() => validateTrees(expanded), /secondary_damage cannot be combined with another hit or target expansion/)
  }
})

test("specialization requires an explicit positive balance revision", () => {
  const current = cloneTrees()
  assert.equal(current[0].balanceRevision, 4)

  const missing = cloneTrees()
  delete missing[0].balanceRevision
  assert.throws(() => validateTrees(missing), /balanceRevision invalid/)

  const zero = cloneTrees()
  zero[0].balanceRevision = 0
  assert.throws(() => validateTrees(zero), /balanceRevision invalid/)
})

test("attack-stat policy is explicit and limited to damage effects", () => {
  const invalidPolicy = cloneTrees()
  const invalidPolicyOwner = mutationDescriptors(invalidPolicy).find(({ mutation }) => mutation.ops?.attackStatPolicy !== undefined)
  mutateMutationSnapshots(invalidPolicyOwner, (mutation) => { mutation.ops.attackStatPolicy = "future_policy" })
  assert.throws(() => validateTrees(invalidPolicy), /attackStatPolicy invalid/)

  const broadSelector = cloneTrees()
  const broadSelectorOwner = mutationDescriptors(broadSelector).find(({ mutation }) => mutation.ops?.attackStatPolicy !== undefined)
  mutateMutationSnapshots(broadSelectorOwner, (mutation) => { mutation.effectSelector = { mode: "all" } })
  assert.throws(() => validateTrees(broadSelector), /attackStatPolicy requires a damage-only selector/)

  const healSelector = cloneTrees()
  const healSelectorOwner = mutationDescriptors(healSelector).find(({ mutation }) => mutation.ops?.attackStatPolicy !== undefined)
  mutateMutationSnapshots(healSelectorOwner, (mutation) => { mutation.effectSelector = { mode: "kind", kinds: ["heal"] } })
  assert.throws(() => validateTrees(healSelector), /damage cannot target effect kind: heal/)
})

test("specialization v6 front-loads 60 points by level 300 while limiting active root routes", () => {
  validateTrees(sourceTrees)
  const tree = sourceTrees[0]
  const nodesByID = new Map(tree.nodes.map((node) => [node.id, node]))
  const nodeCost = (node, rank = node.levels.length) => node.levels
    .slice(0, rank)
    .reduce((total, level) => total + level.cost, 0)
  const mapCost = (ranks) => [...ranks].reduce((total, [nodeID, rank]) => total + nodeCost(nodesByID.get(nodeID), rank), 0)
  const mergeRanks = (left, right) => {
    const merged = new Map(left)
    for (const [nodeID, rank] of right) merged.set(nodeID, Math.max(merged.get(nodeID) || 0, rank))
    return merged
  }
  const routeClosure = (targetID, fullRank) => {
    const visit = (nodeID, stack = new Set()) => {
      assert.equal(stack.has(nodeID), false, "unexpected cycle while measuring " + targetID)
      const node = nodesByID.get(nodeID)
      assert.ok(node, "missing route node " + nodeID)
      let result = new Map([[nodeID, fullRank ? node.levels.length : 1]])
      const nextStack = new Set(stack).add(nodeID)
      for (const requirement of node.requirements.all) {
        if (requirement.kind !== "node_rank") continue
        result = mergeRanks(result, visit(requirement.nodeId, nextStack))
        if (!fullRank) result.set(requirement.nodeId, Math.max(result.get(requirement.nodeId) || 0, requirement.minRank))
      }
      for (const group of node.requirements.anyOf) {
        let cheapest
        for (const requirement of group.filter((entry) => entry.kind === "node_rank")) {
          const candidate = visit(requirement.nodeId, nextStack)
          if (!fullRank) candidate.set(requirement.nodeId, Math.max(candidate.get(requirement.nodeId) || 0, requirement.minRank))
          const merged = mergeRanks(result, candidate)
          if (!cheapest || mapCost(merged) < mapCost(cheapest)) cheapest = merged
        }
        if (cheapest) result = cheapest
      }
      return result
    }
    return visit(targetID)
  }
  const pointsAt = (battleLevel) => tree.initialPoints + tree.pointBands.reduce((total, band) => {
    if (battleLevel < band.fromLevel) return total
    const lastLevel = Math.min(battleLevel, band.toLevel)
    return total + (Math.floor((lastLevel - band.fromLevel) / band.everyLevels) + 1) * band.pointsPerGrant
  }, 0)

  assert.equal(tree.id, "player_combat_specialization")
  assert.equal(tree.schemaVersion, 2)
  assert.equal(tree.rulesetVersion, "specialization-v6")
  assert.equal(tree.balanceRevision, 4)
  assert.equal("migrations" in tree, false)
  assert.equal(tree.ui.layout, "five_spoke")
  assert.equal(tree.nodes.length, 70)
  assert.deepEqual(Object.fromEntries(["root", "technique", "terminal", "confluence"].map((kind) => [kind, tree.nodes.filter((node) => node.kind === kind).length])), {
    root: 10,
    technique: 20,
    terminal: 20,
    confluence: 20
  })
  assert.equal(tree.unlock.battleLevel, 10)
  assert.equal(tree.initialPoints, 2)
  assert.equal(tree.maxPoints, 65)
  assert.equal(tree.maxActiveRootNodes, 3)
  assert.deepEqual(
    [10, 90, 120, 195, 200, 210, 300, 350, 450, 499, 500].map(pointsAt),
    [2, 18, 23, 38, 40, 42, 60, 61, 63, 63, 65]
  )
  assert.deepEqual(tree.reset, { itemId: "gold_coin", count: 5000 })

  const rankDistribution = tree.nodes.reduce((out, node) => ({ ...out, [node.levels.length]: (out[node.levels.length] || 0) + 1 }), {})
  assert.deepEqual(rankDistribution, { 1: 2, 3: 35, 4: 23, 6: 10 })
  assert.equal(tree.nodes.reduce((total, node) => total + nodeCost(node), 0), 604)
  assert.deepEqual(tree.nodes.filter((node) => node.kind === "root").map((node) => node.levels.map((level) => level.cost)), Array(10).fill([2, 1, 1, 1, 1, 1]))
  assert.equal(tree.nodes.filter((node) => node.kind === "technique").every((node) => JSON.stringify(node.levels.map((level) => level.cost)) === "[2,1,1,1]"), true)
  const terminalTopologyIDs = new Set([
    "strength_terminal_three_armed",
    "dexterity_terminal_shadow_dance",
    "intelligence_terminal_archmage",
    "stamina_terminal_absolute_defense"
  ])
  assert.equal(tree.nodes.filter((node) => node.kind === "terminal" && !terminalTopologyIDs.has(node.id)).every((node) => JSON.stringify(node.levels.map((level) => level.cost)) === "[4,2,2]"), true)
  assert.equal(tree.nodes.filter((node) => node.kind === "confluence" && node.id !== "confluence_intelligence_perception_crownless_astrolabe").every((node) => JSON.stringify(node.levels.map((level) => level.cost)) === "[6,2,2]"), true)

  const schoolCosts = Object.fromEntries(tree.schools.map((school) => [school.id, tree.nodes
    .filter((node) => node.kind !== "confluence" && node.schoolIds[0] === school.id)
    .reduce((total, node) => total + nodeCost(node), 0)]))
  assert.deepEqual(schoolCosts, { strength: 80, dexterity: 86, intelligence: 86, perception: 66, stamina: 86 })

  const terminals = tree.nodes.filter((node) => node.kind === "terminal")
  const confluences = tree.nodes.filter((node) => node.kind === "confluence")
  assert.deepEqual([...new Set(terminals.map((node) => mapCost(routeClosure(node.id, true))).sort((left, right) => left - right))], [20, 34, 40])
  assert.deepEqual([...new Set(confluences.map((node) => mapCost(routeClosure(node.id, true))))], [34])
  assert.deepEqual([...new Set(terminals.filter((node) => node.id !== "strength_terminal_three_armed").map((node) => mapCost(routeClosure(node.id, false))))], [16])
  assert.deepEqual([...new Set(confluences.filter((node) => node.id !== "confluence_intelligence_perception_crownless_astrolabe").map((node) => mapCost(routeClosure(node.id, false))))], [26])

  const threeArmed = nodesByID.get("strength_terminal_three_armed")
  const crownless = nodesByID.get("confluence_intelligence_perception_crownless_astrolabe")
  assert.deepEqual(threeArmed.levels.map((level) => level.cost), [22])
  assert.deepEqual(crownless.levels.map((level) => level.cost), [10])
  assert.equal(mapCost(routeClosure(threeArmed.id, false)), 34)
  assert.equal(mapCost(routeClosure(crownless.id, false)), 34)

  const roots = tree.nodes.filter((node) => node.kind === "root")
  const allRootsAtOne = new Map(roots.map((node) => [node.id, 1]))
  assert.equal(mapCost(allRootsAtOne), 20)
  assert.ok(roots.length > tree.maxActiveRootNodes)
  const ordinaryTerminals = terminals.filter((node) => !terminalTopologyIDs.has(node.id))
  assert.equal(ordinaryTerminals.every((node) => mapCost(mergeRanks(allRootsAtOne, routeClosure(node.id, true))) === 38), true)
  let cheapestTwoFinishers = Number.POSITIVE_INFINITY
  for (let left = 0; left < ordinaryTerminals.length; left += 1) {
    for (let right = left + 1; right < ordinaryTerminals.length; right += 1) {
      const leftNode = ordinaryTerminals[left]
      const rightNode = ordinaryTerminals[right]
      if (leftNode.exclusiveGroupId && leftNode.exclusiveGroupId === rightNode.exclusiveGroupId) continue
      cheapestTwoFinishers = Math.min(cheapestTwoFinishers, mapCost(mergeRanks(mergeRanks(
        allRootsAtOne,
        routeClosure(leftNode.id, true)
      ), routeClosure(rightNode.id, true))))
    }
  }
  assert.equal(cheapestTwoFinishers, 56)
  assert.ok(cheapestTwoFinishers <= tree.maxPoints, "the root-route limit, not point scarcity, must prevent broad root sampling")
  assert.equal(Math.min(...confluences.filter((node) => node.id !== crownless.id).map((node) => mapCost(mergeRanks(allRootsAtOne, routeClosure(node.id, false))))), 42)
  assert.ok(mapCost(mergeRanks(allRootsAtOne, routeClosure(crownless.id, false))) <= tree.maxPoints)
  assert.ok(pointsAt(200) >= Math.max(...terminals.map((node) => mapCost(routeClosure(node.id, true)))))
  assert.equal(tree.maxPoints - pointsAt(300), 5)

  const formulaValuesAt20 = (value, out = []) => {
    if (Array.isArray(value)) {
      for (const entry of value) formulaValuesAt20(entry, out)
      return out
    }
    if (!value || typeof value !== "object") return out
    if (value.valueFormula) out.push(value.valueFormula.baseAdd + value.valueFormula.terms.reduce((total, term) => total + term.coefficient * 20, 0))
    for (const child of Object.values(value)) formulaValuesAt20(child, out)
    return out
  }
  for (const root of roots.filter((node) => !["perception_root_eagle_eye", "intelligence_root_pattern_weaving", "dexterity_root_deadly_strike"].includes(node.id))) {
    const rankOne = formulaValuesAt20(root.levels[0].grants)
    const rankSix = formulaValuesAt20(root.levels[5].grants)
    assert.equal(rankOne.length, 1, root.id + " needs one legible core magnitude")
    assert.equal(rankSix.length, 1, root.id + " needs one legible full-rank magnitude")
    assert.ok(Math.abs(rankSix[0] / rankOne[0] - 4) < 0.001, root.id + " rank 1 must be 25% of rank 6")
  }
  const patternRatios = nodesByID.get("intelligence_root_pattern_weaving").levels.map((level) => level.grants
    .flatMap((grant) => grant.battleHooks || [])[0].effects[0].atMax.mpRatio)
  assert.deepEqual(patternRatios, [0.03, 0.035, 0.04, 0.045, 0.05, 0.06])
  const deadlyRatios = nodesByID.get("dexterity_root_deadly_strike").levels.map((level) => level.grants
    .flatMap((grant) => grant.battleHooks || [])[0].effects[0].amountRatio)
  assert.deepEqual(deadlyRatios, [0.2, 0.25, 0.3, 0.35, 0.4, 0.45])
  assert.equal(tree.nodes.flatMap((node) => node.levels)
    .flatMap((level) => level.grants)
    .some((grant) => grant.kind === "stat_modifier"), false)
  const eagleTriggers = nodesByID.get("perception_root_eagle_eye").levels.map((level) => level.grants
    .flatMap((grant) => grant.battleHooks || [])
    .find((hook) => hook.id === "perception_eagle_eye_mark").trigger)
  assert.deepEqual(eagleTriggers, ["onWaveStart", "onWaveStart", "onWaveStart", "onWaveStart", "onWaveStart", "onWaveStart"])

  for (const technique of tree.nodes.filter((node) => node.kind === "technique")) {
    const rootRequirements = technique.requirements.all.filter((requirement) => requirement.kind === "node_rank")
    assert.equal(rootRequirements.length, 1)
    assert.equal(rootRequirements[0].minRank, 6)
    assert.equal(nodesByID.get(rootRequirements[0].nodeId).kind, "root")
  }
  for (const terminal of terminals) {
    const parentRequirements = [...terminal.requirements.all, ...terminal.requirements.anyOf.flat()].filter((requirement) => requirement.kind === "node_rank")
    assert.equal(parentRequirements.every((requirement) => requirement.minRank === 4), true)
    assert.deepEqual(terminal.requirements.all.filter((requirement) => requirement.kind === "pure_school_points_gte"), [
      { kind: "pure_school_points_gte", schoolId: terminal.schoolIds[0], points: 11 }
    ])
  }
  for (const confluence of confluences) {
    const topology = confluence.id === crownless.id
    const parentRequirements = confluence.requirements.anyOf.flat().filter((requirement) => requirement.kind === "node_rank")
    assert.equal(parentRequirements.every((requirement) => requirement.minRank === (topology ? 4 : 2)), true)
    assert.equal(confluence.requirements.all.filter((requirement) => requirement.kind === "pure_school_points_gte")
      .every((requirement) => requirement.points === (topology ? 11 : 9)), true)
  }

  const mutationRows = tree.nodes.flatMap((node) => node.levels.flatMap((level) => level.grants
    .flatMap((grant) => (grant.effectMutations || []).map((mutation) => ({ nodeId: node.id, mutation })))))
    .filter(({ mutation }) => Array.isArray(mutation.ops?.amountTermsAppend))
  assert.equal(mutationRows.length, 15)
  assert.equal(new Set(mutationRows.map((row) => row.nodeId)).size, 4)
  assert.deepEqual(Object.fromEntries(Object.entries(Object.groupBy(mutationRows, (row) => row.mutation.scope)).map(([scope, rows]) => [scope, rows.length])), {
    skill: 11,
    basic_attack: 4
  })
  assert.ok(Math.max(...mutationRows.flatMap(({ mutation }) => mutation.ops.amountTermsAppend.map((term) => Math.abs(term.coefficient)))) <= 0.05)
  assert.ok(Math.max(...mutationRows.map(({ mutation }) => mutation.ops.amountTermsAppend.reduce((total, term) => total + Math.abs(term.coefficient), 0))) <= 0.05)

  assert.equal(tree.keywords.length, 24)
  const keywordsByID = new Map(tree.keywords.map((keyword) => [keyword.id, keyword]))
  const keywordCopy = tree.i18n["zh-CN"].keywords
  assert.match(keywordCopy.break_momentum.mechanicDetails, /每次造成物理伤害获得 1 层.*最多 2 层/)
  assert.match(keywordCopy.echo_cast.mechanicDetails, /不会再次消耗法力.*不会进入冷却/)
  assert.match(keywordCopy.intercept_damage.mechanicDetails, /你承受原伤害.*原目标承受剩余/)
  assert.match(keywordCopy.extra_turn.mechanicDetails, /不会改变正常回合顺序/)
  assert.match(keywordCopy.extra_turn.mechanicDetails, /每场战斗.*跨所有敌人刷新波次.*每波.*新敌人刷新/)
  const debuffKeywords = tree.keywords.filter((keyword) => keyword.mechanicKind === "debuff")
  assert.equal(debuffKeywords.length, 9)
  const recognizedDebuffRuntimeIDs = new Set(debuffKeywords.flatMap((keyword) => keyword.runtimeIds))
  assert.equal(recognizedDebuffRuntimeIDs.size, 9)
  const recognizedDebuffConditions = tree.nodes.flatMap((node) => node.levels)
    .flatMap((level) => level.grants)
    .flatMap((grant) => grant.battleHooks || [])
    .flatMap((hook) => hook.conditions || [])
    .filter((condition) => condition.kind === "target_has_any_buff")
  assert.ok(recognizedDebuffConditions.length > 0)
  assert.equal(recognizedDebuffConditions.every((condition) => {
    const authored = new Set(condition.buffIds)
    return authored.size === recognizedDebuffRuntimeIDs.size
      && [...recognizedDebuffRuntimeIDs].every((runtimeID) => authored.has(runtimeID))
  }), true)
  assert.equal(tree.keywords.filter((keyword) => keyword.mechanicKind === "buff").length, 5)
  assert.equal(tree.keywords.filter((keyword) => keyword.mechanicKind === "counter").length, 5)
  const recognizedDebuffs = keywordsByID.get("recognized_debuffs")
  assert.equal(recognizedDebuffs.mechanicKind, "debuff_family")
  const criticalExposureKeyword = keywordsByID.get("critical_exposure")
  assert.deepEqual(criticalExposureKeyword.runtimeIds, ["specialization_critical_exposure"])
  assert.deepEqual(criticalExposureKeyword.sourceNodeIds, [
    "perception_tech_reveal_weakness",
    "confluence_dexterity_perception_hunt_sequence"
  ])
  const criticalExposure = buffs.find((buff) => buff.id === "specialization_critical_exposure")
  assert.deepEqual({
    kind: criticalExposure.kind,
    durationTurns: criticalExposure.durationTurns,
    stackPolicy: criticalExposure.stackPolicy,
    maxStacks: criticalExposure.maxStacks,
    dispellable: criticalExposure.dispellable
  }, {
    kind: "debuff",
    durationTurns: 6,
    stackPolicy: "stack",
    maxStacks: 6,
    dispellable: true
  })
  const criticalExposureEffect = buffEffects.find((effect) => effect.id === "specialization_critical_exposure_while_active")
  assert.deepEqual(criticalExposureEffect.effects, [
    { kind: "add_stat", attr: "crit_damage_resist", value: -0.05 }
  ])
  const preemptiveEffect = buffEffects.find((effect) => effect.id === "specialization_preemptive_target_prefer_as_enemy_target_while_active")
  assert.deepEqual(preemptiveEffect.effects, [
    { kind: "prefer_as_enemy_target" },
    { kind: "add_stat", attr: "physics_damage_resist", value: -0.15 },
    { kind: "add_stat", attr: "magical_damage_resist", value: -0.15 }
  ])
  for (const node of tree.nodes) {
    const specificMarkIDs = node.keywordIds.filter((keywordID) => keywordID !== "specialization_mark" && keywordsByID.get(keywordID)?.mechanicKind === "mark")
    assert.equal(!(node.keywordIds.includes("specialization_mark") && specificMarkIDs.length > 0), true)
    for (const localeTag of ["zh-CN", "en-US"]) {
      const rankCopy = tree.i18n[localeTag].nodes[node.id].levelEffectSummary
      assert.equal(rankCopy.length, node.levels.length)
      for (const lines of rankCopy) {
        const summary = lines.join(" ")
        for (const keywordID of node.keywordIds) assert.match(summary, new RegExp("\\[\\[keyword:" + keywordID + "\\]\\]"))
      }
    }
  }
  const vaguePlayerCopy = /(少量|小幅|随[^。；]*提高|较低效果|约\s*\d|small amount|slightly|scales with|lower effect|roughly|about\s+\d)/i
  const leakedInternalToken = /\b(?:physics_damage_amplification|physics_damage_resist|magical_damage_resist|mp_regen|hp_regen|fallbackSelfIfEmpty|secondarySeek|selectRule|targetScope|knife_fan|mind_orb|single_edged_sword)\b/
  for (const localeTag of ["zh-CN", "en-US"]) {
    const labels = localeTag === "zh-CN"
      ? { trigger: "触发：", condition: "条件：", effect: "效果：" }
      : { trigger: "Trigger:", condition: "Condition", effect: "Effect:" }
    for (const node of tree.nodes) {
      for (const lines of tree.i18n[localeTag].nodes[node.id].levelEffectSummary) {
        const summary = lines.join(" ")
        assert.doesNotMatch(summary, vaguePlayerCopy, `${localeTag}.${node.id} must use explicit values`)
        assert.doesNotMatch(summary, leakedInternalToken, `${localeTag}.${node.id} leaked an internal DSL token`)
        const triggerIndex = lines.findIndex((line) => String(line).includes(labels.trigger))
        const conditionIndex = lines.findIndex((line) => String(line).includes(labels.condition))
        const effectIndex = lines.findIndex((line) => String(line).includes(labels.effect))
        if (triggerIndex < 0 && conditionIndex < 0 && effectIndex < 0) continue
        assert.ok(triggerIndex >= 0, `${localeTag}.${node.id} structured copy must start with a trigger`)
        assert.ok(effectIndex > triggerIndex, `${localeTag}.${node.id} must put its effect after its trigger`)
        if (conditionIndex >= 0) {
          assert.ok(conditionIndex > triggerIndex, `${localeTag}.${node.id} must put its condition after its trigger`)
          assert.ok(conditionIndex < effectIndex, `${localeTag}.${node.id} must put its condition before its effect`)
        }
      }
    }
  }

  const impact = nodesByID.get("confluence_strength_dexterity_impact_dash")
  assert.deepEqual(impact.levels.map((level) => level.grants[0].battleHooks[0].limits), [
    { perTurn: 1 },
    { perTurn: 1 },
    { perTurn: 1 }
  ])
  assert.equal(impact.levels.every((level) => level.grants[0].battleHooks[0].conditions.some((condition) => condition.kind === "self_weapon_class" && condition.weaponClasses.includes("spear"))), true)
  assert.equal(impact.levels.flatMap((level) => level.grants[0].battleHooks[0].effects).every((effect) => !["extra_turn", "adjust_skill_cooldown"].includes(effect.kind)), true)
  assert.match(tree.i18n["zh-CN"].nodes[impact.id].levelEffectSummary[0].join(" "), /装备矛/)
  assert.match(tree.i18n["zh-CN"].nodes[impact.id].levelEffectSummary[2].join(" "), /每回合第一次/)
  assert.match(tree.i18n["en-US"].nodes[impact.id].levelEffectSummary[1].join(" "), /first.*each turn.*spear/i)
  assert.doesNotMatch(JSON.stringify(tree), /action_meter_gain|action meter|行动条|行动进度|出手提前|行动提前/i)
  const zhText = JSON.stringify(tree.i18n["zh-CN"])
  assert.doesNotMatch(zhText, /爪爪|猫猫|小脑袋|大箱优先|偷来一拍|做完再歇/)

  const capacityNodes = tree.nodes.filter((node) => node.levels.some((level) => level.grants.some((grant) => grant.kind === "equipment_slot_capacity")))
  assert.deepEqual(capacityNodes.map((node) => node.id).sort(), [
    "confluence_intelligence_perception_crownless_astrolabe",
    "dexterity_terminal_shadow_dance",
    "intelligence_terminal_archmage",
    "stamina_terminal_absolute_defense",
    "strength_terminal_three_armed"
  ])
  const combinedDeltas = capacityNodes.flatMap((node) => node.levels.flatMap((level) => level.grants)).reduce((out, grant) => {
    for (const [family, delta] of Object.entries(grant.deltas || {})) out[family] = (out[family] || 0) + delta
    return out
  }, {})
  assert.deepEqual(combinedDeltas, { accessory: 2, right_hand: 0, head: 0, legs: -1, feet: 1, body: 0, arm: 1 })
})

test("specialization ranked mechanics and player copy describe the same concrete upgrade", () => {
  const tree = sourceTrees[0]
  const node = (nodeID) => tree.nodes.find((entry) => entry.id === nodeID)
  const hooks = (nodeID, hookID) => node(nodeID).levels.map((level) => level.grants
    .flatMap((grant) => grant.battleHooks || [])
    .find((hook) => hook.id === hookID))
  const mutations = (nodeID, mutationID) => node(nodeID).levels.map((level) => level.grants
    .flatMap((grant) => grant.effectMutations || [])
    .find((mutation) => mutation.id === mutationID))
  const copy = (locale, nodeID) => tree.i18n[locale].nodes[nodeID].levelEffectSummary.map((lines) => lines.join(" "))

  for (const rankedNode of tree.nodes.filter((entry) => entry.levels.length > 1)) {
    for (let levelIndex = 1; levelIndex < rankedNode.levels.length; levelIndex += 1) {
      const previousCore = rankedNode.levels[levelIndex - 1].grants.filter((grant) => grant.kind !== "stat_modifier")
      const currentCore = rankedNode.levels[levelIndex].grants.filter((grant) => grant.kind !== "stat_modifier")
      assert.notEqual(JSON.stringify(currentCore), JSON.stringify(previousCore), `${rankedNode.id}.Lv${levelIndex + 1} cannot be a stat-only toll`)
    }
  }

  const cooldownLedger = mutations("intelligence_tech_cooldown_ledger", "intelligence_staff_cooldown_ledger")
  assert.deepEqual(cooldownLedger.map((mutation) => mutation.ops.cooldownDelta), [-1, -1, -1, -1])
  assert.deepEqual(cooldownLedger.map((mutation) => mutation.ops.cooldownFloor), [1, 1, 1, 1])
  assert.deepEqual(cooldownLedger.map((mutation) => mutation.ops.mpCostMultiplier), [4.5, 4, 3.5, 3])
  assert.equal(cooldownLedger.every((mutation) => JSON.stringify(mutation.weaponClasses) === '["staff","mind_orb"]'), true)
  assert.deepEqual(copy("zh-CN", "intelligence_tech_cooldown_ledger").map((line) => Number(line.match(/\d+%/)?.[0].slice(0, -1))), [450, 400, 350, 300])

  const quickFormula = mutations("confluence_dexterity_intelligence_quick_formula", "quick_formula_weapon_cooldown")
  assert.deepEqual(quickFormula.map((mutation) => mutation.ops.cooldownFloor), [1, 1, 1])
  assert.deepEqual(quickFormula.map((mutation) => mutation.ops.mpCostMultiplier), [4, 3.5, 3])
  assert.equal(quickFormula.every((mutation) => JSON.stringify(mutation.weaponClasses) === '["staff","mind_orb"]'), true)

  const reveal = mutations("perception_tech_reveal_weakness", "perception_reveal_on_damage")
  assert.deepEqual(reveal.map((mutation) => mutation.ops.appendEffects[0].statusPayload.durationBaseTurns), [2, 3, 4, 5])
  assert.deepEqual(copy("en-US", "perception_tech_reveal_weakness").map((line) => Number(line.match(/\d+/)?.[0])), [2, 3, 4, 5])
  const mindOrbExposure = hooks("perception_tech_reveal_weakness", "perception_mind_orb_critical_exposure")
  assert.equal(mindOrbExposure.every((hook) => hook.trigger === "onAfterDealDamage" && hook.limits.perTurn === 1), true)
  assert.equal(mindOrbExposure.every((hook) => hook.conditions.some((condition) => condition.kind === "self_weapon_class" && JSON.stringify(condition.weaponClasses) === '["mind_orb"]')), true)
  assert.equal(mindOrbExposure.every((hook) => hook.effects.some((effect) => effect.kind === "add_buff" && effect.buffId === "specialization_critical_exposure" && effect.durationTurns === 6 && effect.targetMode === "target")), true)
  assert.equal(copy("zh-CN", "perception_tech_reveal_weakness").every((line) => /装备念珠/.test(line) && /最多 6 层（30 个百分点）/.test(line)), true)

  const scytheExposure = hooks("confluence_dexterity_perception_hunt_sequence", "scythe_reaper_wound")
  assert.equal(scytheExposure.every((hook) => hook.conditions.some((condition) => condition.kind === "self_weapon_class" && JSON.stringify(condition.weaponClasses) === '["scythe"]')), true)
  assert.equal(scytheExposure.every((hook) => hook.effects.some((effect) => effect.kind === "add_buff" && effect.buffId === "specialization_critical_exposure" && effect.durationTurns === 6 && effect.targetMode === "target")), true)
  assert.equal(copy("zh-CN", "confluence_dexterity_perception_hunt_sequence").every((line) => /镰刀/.test(line) && /最多 6 层.*30 个百分点/.test(line)), true)

  const reapingSweep = mutations("dexterity_terminal_reaping_sweep", "dexterity_reaping_sweep")
  const afterimage = mutations("dexterity_terminal_afterimage_flurry", "dexterity_afterimage_basic_splash")
  assert.deepEqual(reapingSweep.map((mutation) => mutation.ops.appendEffects[0].amountRatio), [0.3, 0.35, 0.4])
  assert.equal(reapingSweep.every((mutation) => mutation.ops.appendEffects[0].kind === "secondary_damage" && mutation.ops.appendEffects[0].targetPolicy === "same_row"), true)
  assert.equal(reapingSweep.every((mutation) => JSON.stringify(mutation.weaponClasses) === '["scythe"]'), true)
  assert.deepEqual(afterimage.map((mutation) => mutation.ops.appendEffects[0].amountRatio), [0.35, 0.45, 0.55])
  assert.deepEqual(copy("zh-CN", "dexterity_terminal_reaping_sweep").map((line) => Number(line.match(/\d+%/)?.[0].slice(0, -1))), [30, 35, 40])
  assert.deepEqual(copy("en-US", "dexterity_terminal_afterimage_flurry").map((line) => Number(line.match(/\d+%/)?.[0].slice(0, -1))), [35, 45, 55])

  const manaFlood = hooks("intelligence_terminal_mana_flood", "intelligence_mana_flood_cycle")
  assert.deepEqual(manaFlood.map((hook) => hook.effects[0].maxStacks), [2, 2, 2])
  assert.equal(copy("zh-CN", "intelligence_terminal_mana_flood").every((line) => /2 层/.test(line)), true)

  for (const [nodeID, hookID] of [
    ["confluence_dexterity_stamina_roving_formation", "roving_formation_step"],
    ["confluence_perception_stamina_watchful_aid", "watchful_aid_turn_end"]
  ]) {
    assert.equal(hooks(nodeID, hookID).every((hook) => hook.conditions.some((condition) => condition.kind === "adjacent_ally_exists")), true)
    assert.equal(copy("zh-CN", nodeID).every((line) => /身旁(?:有|存在存活)队友|仍需身旁有队友/.test(line)), true)
  }

  const rupture = mutations("strength_tech_rupture", "strength_blood_forge_price")
  assert.deepEqual(rupture.map((mutation) => mutation.ops.appendEffects.find((effect) => effect.id === "strength_blood_forge_amp").value), [0.1, 0.11, 0.12, 0.13])
  assert.equal(rupture.every((mutation) => mutation.ops.amountTermsAppend === undefined), true)
  assert.equal(copy("en-US", "strength_tech_rupture").every((line) => !/Strength scaling|Strength term/i.test(line)), true)

  const manaEdge = mutations("confluence_strength_intelligence_mana_edge", "mana_edge_dual_scaling")
  const mindLock = mutations("confluence_intelligence_perception_mind_lock", "mind_lock_low_hp_targeting")
  assert.deepEqual(manaEdge.map((mutation) => mutation.ops.amountMultiplier), [1.02, 1.04, 1.06])
  assert.equal(manaEdge.every((mutation) => mutation.ops.attackStatPolicy === "higher" && mutation.ops.damageTypePolicy === undefined), true)
  assert.equal(manaEdge.every((mutation) => mutation.effectSelector.mode === "kind" && JSON.stringify(mutation.effectSelector.kinds) === '["damage"]'), true)
  assert.deepEqual(mindLock.map((mutation) => mutation.ops.amountMultiplier), [1.02, 1.04, 1.06])
  assert.equal([...manaEdge, ...mindLock].every((mutation) => mutation.ops.amountTermsAppend === undefined), true)

  const weakpoint = hooks("confluence_strength_perception_weakpoint_break", "spear_weakpoint_repeat")
  assert.equal(weakpoint.every((entry) => entry.conditions.some((condition) => condition.kind === "self_weapon_class" && condition.weaponClasses.includes("spear"))), true)
  assert.deepEqual(weakpoint.map((entry) => entry.effects[0].amountRatio), [0.25, 0.35, 0.45])
  assert.equal(weakpoint.every((entry) => entry.effects[0].dispatchHooks === false && entry.effects[0].valueFormula === undefined), true)
  assert.match(JSON.stringify(tree.i18n["zh-CN"].nodes.confluence_strength_perception_weakpoint_break), /不消耗减益/)

  const arcaneOverflow = hooks("intelligence_tech_arcane_overflow", "staff_arcane_overflow_repeat")
  assert.equal(arcaneOverflow.every((entry) => entry.trigger === "onAfterDealDamage" && entry.conditions.some((condition) => condition.kind === "self_mp_ratio_gte" && condition.value === 0.6)), true)
  assert.deepEqual(arcaneOverflow.map((entry) => entry.effects[0].amountRatio), [0.2, 0.25, 0.3, 0.35])

  const swiftForce = mutations("confluence_strength_dexterity_swift_force", "longsword_divided_arc")
  assert.equal(swiftForce.every((entry) => entry.scope === "skill" && entry.skillCategories.includes("blade") && entry.weaponClasses.includes("sword")), true)
  assert.equal(swiftForce.every((entry) => entry.ops.appendEffects[0].kind === "secondary_damage" && entry.ops.appendEffects[0].targetPolicy === "all_rows"), true)
  assert.deepEqual(swiftForce.map((entry) => entry.ops.appendEffects[0].amountRatio), [0.24, 0.3, 0.36])

  const critical = hooks("dexterity_tech_critical_reprise", "dexterity_bow_rhythm_crit")
  const adapt = hooks("dexterity_tech_adapt_on_hit", "dexterity_spear_brace_counter")
  assert.equal(critical.every((entry) => entry.conditions.some((condition) => condition.kind === "self_weapon_class" && condition.weaponClasses.includes("bow"))), true)
  assert.deepEqual(critical.map((entry) => entry.effects[0].durationTurns), [1, 2, 3, 4])
  assert.equal(adapt.every((entry) => entry.conditions.some((condition) => condition.kind === "self_weapon_class" && condition.weaponClasses.length === 1 && condition.weaponClasses[0] === "spear")), true)
  assert.deepEqual(adapt.map((entry) => entry.effects[0].amountFormula.baseAdd), [6, 10, 14, 18])
  assert.equal([...critical, ...adapt].flatMap((entry) => entry.effects).every((effect) => effect.kind !== "adjust_skill_cooldown"), true)

  const hunter = hooks("perception_tech_hunter_mark", "perception_designate_priority_target")
  assert.deepEqual(hunter.map((hook) => hook.trigger), ["onBattleStart", "onWaveStart", "onWaveStart", "onWaveStart"])
  assert.deepEqual(hunter.map((hook) => hook.effects.length), [1, 1, 2, 3])
  const mindFocus = hooks("intelligence_tech_mind_focus", "intelligence_mind_focus_damage")
  assert.deepEqual(mindFocus.map((entry) => entry.effects[0].value), [0.04, 0.06, 0.08, 0.1])
  assert.equal(mindFocus.every((entry) => entry.effects.every((effect) => effect.kind !== "adjust_skill_cooldown")), true)
  const pursuit = hooks("perception_tech_relentless_pursuit", "perception_bow_retarget")
  assert.equal(pursuit.every((entry) => entry.conditions.some((condition) => condition.kind === "target_has_any_buff")), true)
  assert.equal(pursuit.every((entry) => entry.conditions.some((condition) => condition.kind === "self_weapon_class" && condition.weaponClasses.includes("bow"))), true)
  assert.equal(pursuit.every((entry) => entry.effects.every((effect) => effect.kind !== "adjust_skill_cooldown")), true)

  const eagleEye = hooks("perception_root_eagle_eye", "perception_eagle_eye_mark")
  assert.deepEqual(eagleEye.map((entry) => entry.trigger), Array(6).fill("onWaveStart"))
  const eagleOpenings = hooks("perception_root_eagle_eye", "perception_eagle_eye_bow_opening")
  assert.deepEqual(eagleOpenings.map((entry) => entry.effects[0].value), [0.06, 0.08, 0.1, 0.12, 0.15, 0.18])

  const stolenBeat = hooks("dexterity_terminal_stolen_beat", "dexterity_stolen_beat_turn")
  assert.deepEqual(stolenBeat.map((entry) => entry.effects.length), [1, 1, 1])
  assert.deepEqual(stolenBeat.map((hook) => hook.effects.find((effect) => effect.kind === "extra_turn").procChanceBase), [0.2, 0.3, 0.4])
  assert.deepEqual(stolenBeat.map((entry) => entry.effects.find((effect) => effect.kind === "extra_turn").procChanceMax), [0.4, 0.4, 0.4])
  assert.equal(stolenBeat.every((hook) => hook.effects.find((effect) => effect.kind === "extra_turn").abortHookOnFail === true), true)
  assert.equal(stolenBeat.every((entry) => entry.conditions.some((condition) => condition.kind === "self_hp_ratio_lte" && condition.value === 0.7)), true)
  assert.match(copy("zh-CN", "dexterity_terminal_stolen_beat")[0], /生命不高于 70%.*20%/)
  assert.match(copy("zh-CN", "dexterity_terminal_stolen_beat")[2], /40%/)

  const spearDepth = mutations("perception_terminal_foresight", "perception_spear_rear_thrust")
  assert.equal(spearDepth.every((entry) => entry.scope === "basic_attack" && entry.weaponClasses.includes("spear")), true)
  assert.equal(spearDepth.every((entry) => entry.ops.appendEffects[0].kind === "secondary_damage" && entry.ops.appendEffects[0].targetPolicy === "fixed_row" && entry.ops.appendEffects[0].targetPosition === "rearguard"), true)
  assert.deepEqual(spearDepth.map((entry) => entry.ops.appendEffects[0].amountRatio), [0.25, 0.35, 0.45])
  const greatbladeBrink = hooks("strength_terminal_titan_combo", "strength_greatblade_desperate_strike")
  assert.equal(greatbladeBrink.every((entry) => entry.trigger === "onBeforeDealDamage" && entry.limits?.perTurn === 1), true)
  assert.equal(greatbladeBrink.every((entry) => entry.conditions.some((condition) => condition.kind === "self_hp_ratio_lte" && condition.value === 0.55)), true)
  assert.deepEqual(greatbladeBrink.map((entry) => entry.effects[0].value), [0.12, 0.16, 0.2])
  const shadowDance = hooks("dexterity_terminal_shadow_dance", "dexterity_shadowstep_momentum")
  assert.deepEqual(shadowDance.map((entry) => entry.effects[0].durationTurns), [1, 2, 3, 4])
  assert.equal(shadowDance.every((entry) => entry.effects.every((effect) => !["extra_turn", "adjust_skill_cooldown"].includes(effect.kind))), true)
  const reactiveBulwark = hooks("stamina_terminal_reactive_bulwark", "stamina_retaliatory_wall")
  assert.equal(reactiveBulwark.every((entry) => entry.limits?.perTurn === 1 && entry.effects[0].dispatchHooks === false), true)
  const judgement = hooks("perception_terminal_judgement_window", "perception_consume_exposed")
  assert.deepEqual(judgement.map((hook) => hook.effects.find((effect) => effect.kind === "remove_buff_by_kind").maxBuffs), [1, 2, 2])
  assert.deepEqual(judgement.map((hook) => hook.effects.find((effect) => effect.kind === "adjust_skill_cooldown").amount), [1, 1, 2])
  const preemptive = hooks("perception_terminal_preemptive_strike", "perception_preemptive_spear_opening")
  assert.deepEqual(preemptive.map((entry) => entry.effects[0].value), [0.1, 0.15, 0.2])
  const volley = hooks("confluence_dexterity_perception_eagle_volley", "eagle_volley_multihit_mark")
  assert.deepEqual(volley.map((hook) => hook.effects.find((effect) => effect.kind === "add_buff").durationTurns), [2, 3, 4])
  const guardDash = hooks("confluence_dexterity_stamina_guard_dash", "guard_dash_turn_start")
  assert.deepEqual(guardDash.map((hook) => hook.effects.find((effect) => effect.kind === "shift_position").maxSteps), [1, 2, 3])

  assert.match(copy("en-US", "dexterity_root_deadly_strike")[0], /scythe.*fixed 20% ratio/i)
  assert.equal(tree.nodes.flatMap((entry) => entry.levels)
    .flatMap((level) => level.grants)
    .some((grant) => grant.kind === "stat_modifier"), false)
  assert.doesNotMatch(JSON.stringify(tree.i18n), /当前等级总属性|当前总属性|Total attribute at this rank|Rank total/)

  const keywordDetails = (locale, keywordID) => tree.i18n[locale].keywords[keywordID].mechanicDetails
  assert.match(keywordDetails("zh-CN", "hunter_mark"), /开战时.*2 级起每波刷新/)
  assert.match(keywordDetails("en-US", "hunter_mark"), /battle start.*rank 2.*each wave/i)
  assert.match(keywordDetails("zh-CN", "eagle_eye_mark"), /每个新敌人刷新波次开始时/)
  assert.match(keywordDetails("en-US", "eagle_eye_mark"), /start of every enemy wave/i)
  assert.match(keywordDetails("zh-CN", "eagle_volley_mark"), /持续时间由节点当前等级决定/)
  assert.match(keywordDetails("en-US", "eagle_volley_mark"), /duration is set by the node's current rank/i)
})

test("specialization cooldown reductions require a one-turn floor and at least 300% MP cost", () => {
  const withoutFloor = cloneTrees()
  const floorOwner = mutationDescriptors(withoutFloor).find(({ mutation }) => Number(mutation.ops?.cooldownDelta) < 0)
  mutateMutationSnapshots(floorOwner, (mutation) => delete mutation.ops.cooldownFloor)
  assert.throws(() => validateTrees(withoutFloor), /cooldown reduction requires cooldownFloor >= 1/)

  const cheapCost = cloneTrees()
  const costOwner = mutationDescriptors(cheapCost).find(({ mutation }) => Number(mutation.ops?.cooldownDelta) < 0)
  const finalCostGrant = costOwner.node.levels.at(-1).grants.find((grant) => grant.id === costOwner.grant.id)
  finalCostGrant.effectMutations.find((mutation) => mutation.id === costOwner.mutation.id).ops.mpCostMultiplier = 2.99
  assert.throws(() => validateTrees(cheapCost), /cooldown reduction requires mpCostMultiplier >= 3/)

  const discountedCost = cloneTrees()
  const discountOwner = mutationDescriptors(discountedCost).find(({ mutation }) => Number(mutation.ops?.cooldownDelta) < 0)
  mutateMutationSnapshots(discountOwner, (mutation) => { mutation.ops.mpCostDelta = -1 })
  assert.throws(() => validateTrees(discountedCost), /cooldown reduction requires mpCostDelta >= 0/)

  const weaponless = cloneTrees()
  const weaponlessOwner = mutationDescriptors(weaponless).find(({ mutation }) => Number(mutation.ops?.cooldownDelta) < 0)
  mutateMutationSnapshots(weaponlessOwner, (mutation) => delete mutation.weaponClasses)
  assert.throws(() => validateTrees(weaponless), /cooldown reduction requires an explicit weaponClasses restriction/)

  const outsideIntelligence = cloneTrees()
  const outsideOwner = mutationDescriptors(outsideIntelligence).find(({ mutation }) => Number(mutation.ops?.cooldownDelta) < 0)
  outsideOwner.node.schoolIds = ["dexterity"]
  assert.throws(() => validateTrees(outsideIntelligence), /cooldown reduction is restricted to Intelligence routes/)
})

test("specialization extra-hit mutations require an explicit weapon school", () => {
  const trees = cloneTrees()
  const owner = mutationDescriptors(trees).find(({ mutation }) => Number(mutation.ops?.hitCountDelta) > 0)
  mutateMutationSnapshots(owner, (mutation) => delete mutation.weaponClasses)
  assert.throws(() => validateTrees(trees), /extra-hit mutation requires an explicit weaponClasses restriction/)
})

test("specialization rejects unstable effect-array index selectors", () => {
  const trees = cloneTrees()
  const node = trees[0].nodes.find((row) => row.id === "dexterity_tech_chain_steps")
  node.levels[0].grants[0].effectMutations[0].effectSelector = { mode: "index", indexes: [1] }
  assert.throws(() => validateTrees(trees), /must not use unstable array indexes/)
})

test("specialization rejects the legacy three-point confluence entry price", () => {
  const trees = cloneTrees()
  trees[0].nodes.find((node) => node.kind === "confluence").levels[0].cost = 3
  assert.throws(() => validateTrees(trees), /levels\.cost invalid/)
})

test("specialization accepts a real catalog stable effect id selector", () => {
  const trees = cloneTrees()
  const skillEffectIds = skills.flatMap((skill) => skill.effects.map((effect) => effect.id))
  const equipmentEffectIds = equipmentTemplates.flatMap((template) => (template.basicAttackEffects || []).map((effect) => effect.id))
  const effectIds = [...skillEffectIds, ...equipmentEffectIds]
  assert.ok(skillEffectIds.length > 0)
  assert.ok(equipmentEffectIds.length > 0)
  assert.equal(effectIds.every(Boolean), true)
  assert.equal(new Set(effectIds).size, effectIds.length)
  mutateFirstMutationSnapshots(trees, (mutation) => {
    mutation.effectSelector = {
      mode: "effect_id",
      effectIds: [skillEffectIds[0], equipmentEffectIds[0]],
      includeGenerated: false
    }
  })
  validateTrees(trees)
})

test("specialization registers appended stable effect IDs for generated-effect selectors", () => {
  const trees = cloneTrees()
  const producer = mutationDescriptors(trees).find(({ mutation }) => mutation.ops?.appendEffects?.some((effect) => effect.kind === "splash_damage"))
  const appendedEffectID = producer.mutation.ops.appendEffects.find((effect) => effect.kind === "splash_damage").id
  appendConsumer(producer, appendedEffectID)
  const statusProducer = mutationDescriptors(trees).find(({ mutation }) => mutation.ops?.appendEffects?.some((effect) => effect.kind === "status"))
  const dependentNode = trees[0].nodes.find((node) => node.id === "perception_terminal_judgement_window")
  const dependentGrant = {
    id: "test_dependent_generated_consumer",
    kind: "effect_mutation",
    effectMutations: [{
      id: "dependent_mark_consumer",
      priority: -1,
      scope: "both",
      effectSelector: { mode: "effect_id", effectIds: [statusProducer.mutation.ops.appendEffects[0].id], includeGenerated: true },
      ops: { statusIdsRemove: ["rage_power"] }
    }]
  }
  for (const level of dependentNode.levels) level.grants.push(structuredClone(dependentGrant))
  validateTrees(trees)
})

test("specialization appended stable effect IDs are lowercase and globally unique with catalog effects", () => {
  const uppercase = cloneTrees()
  appendedEffects(uppercase)[0].id = "Generated_Damage"
  assert.throws(() => validateTrees(uppercase), /stable effect ids must use lowercase letters/)

  const catalogCollision = cloneTrees()
  appendedEffects(catalogCollision)[0].id = equipmentTemplates.flatMap((template) => template.basicAttackEffects || [])[0].id
  assert.throws(() => validateTrees(catalogCollision), /battle effect stable id duplicate/)

  const appendCollision = cloneTrees()
  const appendRows = appendedEffects(appendCollision)
  appendRows[2].id = appendRows[0].id.toUpperCase()
  assert.throws(() => validateTrees(appendCollision), /stable effect ids must use lowercase letters|battle effect stable id duplicate/)
  appendRows[2].id = appendRows[0].id
  assert.throws(() => validateTrees(appendCollision), /battle effect stable id duplicate/)

  const sameNodeCollision = cloneTrees()
  const collisionNode = sameNodeCollision[0].nodes.find((node) => node.id === "dexterity_terminal_reaping_sweep")
  for (const level of collisionNode.levels) {
    const grant = level.grants.find((entry) => entry.id === "grant_dexterity_terminal_reaping_sweep")
    const secondProducer = structuredClone(grant.effectMutations[0])
    secondProducer.id = "dexterity_blade_fan_second_producer"
    grant.effectMutations.push(secondProducer)
  }
  assert.throws(() => validateTrees(sameNodeCollision), /battle effect stable id duplicate/)

  const sameProducerAcrossRanks = cloneTrees()
  const stableNode = sameProducerAcrossRanks[0].nodes.find((node) => node.id === "dexterity_terminal_reaping_sweep")
  assert.equal(new Set(stableNode.levels.map((level) => level.grants.find((entry) => entry.id === "grant_dexterity_terminal_reaping_sweep").effectMutations[0].ops.appendEffects[0].id)).size, 1)
  validateTrees(sameProducerAcrossRanks)
})

test("specialization rejects dependency cycles", () => {
  const trees = cloneTrees()
  const left = trees[0].nodes.find((row) => row.id === "strength_tech_heavy_momentum")
  const right = trees[0].nodes.find((row) => row.id === "strength_tech_rupture")
  left.requirements.all.push({ kind: "node_rank", nodeId: right.id, minRank: 1 })
  right.requirements.all.push({ kind: "node_rank", nodeId: left.id, minRank: 1 })
  assert.throws(() => validateTrees(trees), /dependency cycle detected/)
})

test("specialization rejects transitively redundant prerequisite edges", () => {
  const trees = cloneTrees()
  const node = trees[0].nodes.find((row) => row.id === "strength_tech_heavy_momentum")
  node.requirements.all.push({ kind: "node_rank", nodeId: "strength_tech_rupture", minRank: 1 })
  assert.throws(() => validateTrees(trees), /transitively redundant prerequisite/)
})

test("specialization terminal choices stay within one root and use one canonical requirement form", () => {
  const crossRoot = cloneTrees()
  const crossRootTerminal = crossRoot[0].nodes.find((node) => node.id === "dexterity_terminal_reaping_sweep")
  crossRootTerminal.requirements.anyOf[0][1].nodeId = "dexterity_tech_rapid_strike"
  assert.throws(() => validateTrees(crossRoot), /terminal choices must stay under one root/)

  const mixed = cloneTrees()
  const mixedTerminal = mixed[0].nodes.find((node) => node.id === "dexterity_terminal_reaping_sweep")
  mixedTerminal.requirements.all.push({ kind: "node_rank", nodeId: "dexterity_tech_chain_steps", minRank: 1 })
  assert.throws(() => validateTrees(mixed), /must require one direct technique or one two-technique choice/)
})

test("specialization multi-rank nodes preserve and improve their complete gameplay snapshot", () => {
  const missingCore = cloneTrees()
  const missingCoreNode = missingCore[0].nodes.find((row) => row.id === "strength_root_break_stance")
  const replacement = structuredClone(missingCoreNode.levels[1].grants[0])
  replacement.id = "replacement_break_cycle"
  missingCoreNode.levels[1].grants = [replacement]
  assert.throws(() => validateTrees(missingCore), /must retain prior rank grant id\/kind/)

  const changedKind = cloneTrees()
  const changedKindNode = changedKind[0].nodes.find((row) => row.id === "strength_root_break_stance")
  const changedKindGrant = changedKindNode.levels[1].grants.find((grant) => grant.id === "grant_strength_root_break_cycle")
  Object.assign(changedKindGrant, { kind: "stat_modifier", scope: "character", modifiers: [{ attributeKey: "strength", operation: "add", value: 1 }] })
  delete changedKindGrant.battleHooks
  assert.throws(() => validateTrees(changedKind), /must retain prior rank grant id\/kind: grant_strength_root_break_cycle\/battle_hook/)

  const renamedHook = cloneTrees()
  const renamedHookNode = renamedHook[0].nodes.find((row) => row.id === "strength_root_break_stance")
  renamedHookNode.levels[1].grants.find((grant) => grant.id === "grant_strength_root_break_cycle").battleHooks[0].id = "renamed_break_hook"
  assert.throws(() => validateTrees(renamedHook), /must retain battleHook stable payload id/)

  const droppedHook = cloneTrees()
  const droppedHookNode = droppedHook[0].nodes.find((row) => row.id === "strength_root_break_stance")
  droppedHookNode.levels[1].grants.find((grant) => grant.id === "grant_strength_root_break_cycle").battleHooks.shift()
  assert.throws(() => validateTrees(droppedHook), /must retain battleHook stable payload id/)

  const renamedMutation = cloneTrees()
  const renamedMutationNode = renamedMutation[0].nodes.find((row) => row.id === "dexterity_tech_chain_steps")
  renamedMutationNode.levels[1].grants.find((grant) => grant.id === "grant_dexterity_tech_chain_steps").effectMutations[0].id = "renamed_chain_steps"
  assert.throws(() => validateTrees(renamedMutation), /must retain effectMutation stable payload id/)

  const renamedAppendEffect = cloneTrees()
  const renamedAppendNode = renamedAppendEffect[0].nodes.find((row) => row.id === "strength_tech_rupture")
  renamedAppendNode.levels[1].grants.find((grant) => grant.id === "grant_strength_tech_rupture")
    .effectMutations[0].ops.appendEffects[0].id = "renamed_blood_forge_cost"
  assert.throws(() => validateTrees(renamedAppendEffect), /must retain effectMutation appended stable effect id/)

  const repurposedAppendEffect = cloneTrees()
  const repurposedAppendNode = repurposedAppendEffect[0].nodes.find((row) => row.id === "strength_tech_rupture")
  const repurposedEffect = repurposedAppendNode.levels[1].grants.find((grant) => grant.id === "grant_strength_tech_rupture")
    .effectMutations[0].ops.appendEffects[0]
  Object.keys(repurposedEffect).forEach((key) => delete repurposedEffect[key])
  Object.assign(repurposedEffect, { id: "strength_blood_forge_hp_cost", kind: "splash_damage", targetCount: 1, amountRatio: 0.2 })
  assert.throws(() => validateTrees(repurposedAppendEffect), /must retain effectMutation appended effect kind/)

  const narrowedScope = cloneTrees()
  const narrowedScopeNode = narrowedScope[0].nodes.find((row) => row.id === "dexterity_terminal_reaping_sweep")
  narrowedScopeNode.levels[1].grants.find((grant) => grant.id === "grant_dexterity_terminal_reaping_sweep").effectMutations[0].scope = "skill"
  assert.throws(() => validateTrees(narrowedScope), /must retain or widen effectMutation scope/)

  const changedSelectorMode = cloneTrees()
  const changedSelectorNode = changedSelectorMode[0].nodes.find((row) => row.id === "dexterity_tech_chain_steps")
  changedSelectorNode.levels[1].grants.find((grant) => grant.id === "grant_dexterity_tech_chain_steps").effectMutations[0].effectSelector = { mode: "all" }
  assert.throws(() => validateTrees(changedSelectorMode), /must retain effectMutation selector mode/)

  const narrowedSelectorValues = cloneTrees()
  const quickFormulaNode = narrowedSelectorValues[0].nodes.find((row) => row.id === "confluence_dexterity_intelligence_quick_formula")
  quickFormulaNode.levels[1].grants.find((grant) => grant.id === "grant_rework_confluence_dexterity_intelligence_quick_formula")
    .effectMutations[0].skillCategories = ["arcane"]
  assert.throws(() => validateTrees(narrowedSelectorValues), /must retain prior selector value: tactical/)

  const addedNarrowingFilter = cloneTrees()
  const addedFilterNode = addedNarrowingFilter[0].nodes.find((row) => row.id === "dexterity_tech_chain_steps")
  addedFilterNode.levels[1].grants.find((grant) => grant.id === "grant_dexterity_tech_chain_steps")
    .effectMutations[0].skillCategories = ["arcane"]
  assert.throws(() => validateTrees(addedNarrowingFilter), /must not add a narrowing filter after an unfiltered rank/)

  const changedHookTrigger = cloneTrees()
  const changedTriggerNode = changedHookTrigger[0].nodes.find((row) => row.id === "strength_root_break_stance")
  changedTriggerNode.levels[1].grants.find((grant) => grant.id === "grant_strength_root_break_cycle")
    .battleHooks[0].trigger = "onAfterDealCrit"
  assert.throws(() => validateTrees(changedHookTrigger), /must retain battleHook trigger/)

  const addedNarrowingCondition = cloneTrees()
  const addedConditionNode = addedNarrowingCondition[0].nodes.find((row) => row.id === "strength_root_break_stance")
  addedConditionNode.levels[1].grants.find((grant) => grant.id === "grant_strength_root_break_cycle")
    .battleHooks.find((hook) => hook.id === "strength_break_momentum_cycle").conditions.push({ kind: "self_hp_ratio_lte", value: 0.01 })
  assert.throws(() => validateTrees(addedNarrowingCondition), /must not add or narrow battleHook conditions/)

  const narrowedOuterLimit = cloneTrees()
  const narrowedLimitNode = narrowedOuterLimit[0].nodes.find((row) => row.id === "confluence_strength_dexterity_impact_dash")
  narrowedLimitNode.levels[2].grants.find((grant) => grant.kind === "battle_hook")
    .battleHooks.find((hook) => hook.id === "impact_dash_spear_followup").limits = { perBattle: 1 }
  assert.throws(() => validateTrees(narrowedOuterLimit), /must not narrow battleHook limits/)

  const addedNarrowingEventFilter = cloneTrees()
  const narrowedEventNode = addedNarrowingEventFilter[0].nodes.find((row) => row.id === "strength_root_break_stance")
  narrowedEventNode.levels[1].grants.find((grant) => grant.id === "grant_strength_root_break_cycle")
    .battleHooks.find((hook) => hook.id === "strength_break_momentum_cycle").eventFilter = { skillIds: ["basic_attack"] }
  assert.throws(() => validateTrees(addedNarrowingEventFilter), /must not add a narrowing eventFilter/)

  const reducedCounterCapacity = cloneTrees()
  const reducedCounterNode = reducedCounterCapacity[0].nodes.find((row) => row.id === "strength_root_break_stance")
  reducedCounterNode.levels[1].grants.find((grant) => grant.id === "grant_strength_root_break_cycle")
    .battleHooks.find((hook) => hook.id === "strength_break_momentum_cycle").effects[0].maxStacks = 1
  assert.throws(() => validateTrees(reducedCounterCapacity), /maxStacks 1 is below required 2/)

  const droppedHookEffectKind = cloneTrees()
  const stolenBeatNode = droppedHookEffectKind[0].nodes.find((row) => row.id === "dexterity_terminal_stolen_beat")
  const stolenBeatHook = stolenBeatNode.levels[1].grants.find((grant) => grant.id === "grant_rework_dexterity_terminal_stolen_beat").battleHooks[0]
  stolenBeatHook.effects = [{ kind: "heal", targetMode: "self", maxHpRatio: 0.01 }]
  assert.throws(() => validateTrees(droppedHookEffectKind), /must retain prior hook effect kind: extra_turn/)

  const reducedExtraTurnChance = cloneTrees()
  const reducedChanceNode = reducedExtraTurnChance[0].nodes.find((row) => row.id === "dexterity_terminal_stolen_beat")
  reducedChanceNode.levels[1].grants.find((grant) => grant.id === "grant_rework_dexterity_terminal_stolen_beat")
    .battleHooks[0].effects.find((effect) => effect.kind === "extra_turn").procChanceBase = 0.1
  assert.throws(() => validateTrees(reducedExtraTurnChance), /must not reduce extra_turn procChanceBase/)

  const degradedSecondaryRatio = cloneTrees()
  const reapingNode = degradedSecondaryRatio[0].nodes.find((row) => row.id === "dexterity_terminal_reaping_sweep")
  reapingNode.levels[1].grants.find((grant) => grant.id === "grant_dexterity_terminal_reaping_sweep")
    .effectMutations[0].ops.appendEffects[0].amountRatio = 0.01
  assert.throws(() => validateTrees(degradedSecondaryRatio), /must not reduce secondary_damage amountRatio/)

  const changedSecondaryPolicy = cloneTrees()
  const changedPolicyNode = changedSecondaryPolicy[0].nodes.find((row) => row.id === "dexterity_terminal_reaping_sweep")
  changedPolicyNode.levels[1].grants.find((grant) => grant.id === "grant_dexterity_terminal_reaping_sweep")
    .effectMutations[0].ops.appendEffects[0].targetPolicy = "all_rows"
  assert.throws(() => validateTrees(changedSecondaryPolicy), /must retain secondary_damage target policy/)

  const changedCounterIdentity = cloneTrees()
  const changedCounterNode = changedCounterIdentity[0].nodes.find((row) => row.id === "strength_root_break_stance")
  const changedCounterHooks = changedCounterNode.levels[1].grants.find((grant) => grant.id === "grant_strength_root_break_cycle").battleHooks
  const changedCounterCycle = changedCounterHooks.find((hook) => hook.id === "strength_break_momentum_cycle")
  const unchangedCounterCondition = changedCounterHooks.find((hook) => hook.id === "strength_break_momentum_release")
    .conditions.find((condition) => condition.kind === "counter_gte")
  changedCounterCycle.effects[0].counterId = "unrelated_break_counter"
  changedCounterCycle.conditions.push(structuredClone(unchangedCounterCondition))
  assert.throws(() => validateTrees(changedCounterIdentity), /must retain battleHook effect resource binding: cycle_counter#0\.counterId=strength_break_momentum/)

  const changedAppendedStatusIdentity = cloneTrees()
  const statusDescriptor = mutationDescriptors(changedAppendedStatusIdentity)
    .find(({ mutation }) => mutation.ops?.appendEffects?.some((effect) => effect.kind === "status"))
  const rankTwoStatusGrant = statusDescriptor.node.levels[1].grants.find((grant) => grant.id === statusDescriptor.grant.id)
  rankTwoStatusGrant.effectMutations.find((mutation) => mutation.id === statusDescriptor.mutation.id)
    .ops.appendEffects.find((effect) => effect.kind === "status").statusPayload.statusIds = ["rage_power"]
  assert.throws(() => validateTrees(changedAppendedStatusIdentity), /statusPayload\.statusIds must retain prior selector value/)

  assert.throws(() => validateSpecializationGrantSnapshotRetention(
    { id: "topology_swap", kind: "equipment_slot_capacity", equipType: "battle", deltas: { head: -1, accessory: 2 } },
    { id: "topology_swap", kind: "equipment_slot_capacity", equipType: "battle", deltas: { accessory: 2 } }
  ), /must retain equipment slot family: topology_swap\/head/)

  const legalAddition = cloneTrees()
  const legalAdditionNode = legalAddition[0].nodes.find((row) => row.id === "strength_root_break_stance")
  const addedHook = structuredClone(legalAdditionNode.levels[0].grants.find((grant) => grant.id === "grant_strength_root_break_cycle").battleHooks[0])
  addedHook.id = "strength_break_rank_bonus"
  for (const level of legalAdditionNode.levels.slice(1)) level.grants.find((grant) => grant.id === "grant_strength_root_break_cycle").battleHooks.push(structuredClone(addedHook))
  validateTrees(legalAddition)
  assert.doesNotThrow(() => validateSpecializationGrantSnapshotRetention(
    { id: "topology_swap", kind: "equipment_slot_capacity", equipType: "battle", deltas: { head: -1, accessory: 2 } },
    { id: "topology_swap", kind: "equipment_slot_capacity", equipType: "battle", deltas: { head: -2, accessory: 2, right_hand: 1 } }
  ))
  const ruptureNode = legalAddition[0].nodes.find((row) => row.id === "strength_tech_rupture")
  assert.doesNotThrow(() => validateSpecializationGrantSnapshotRetention(
    ruptureNode.levels[0].grants.find((grant) => grant.id === "grant_strength_tech_rupture"),
    ruptureNode.levels[1].grants.find((grant) => grant.id === "grant_strength_tech_rupture")
  ))
  assert.doesNotThrow(() => validateSpecializationGrantSnapshotRetention(
    { id: "mutation", kind: "effect_mutation", effectMutations: [{ id: "payload", scope: "skill", effectSelector: { mode: "kind", kinds: ["damage"] }, ops: {} }] },
    { id: "mutation", kind: "effect_mutation", effectMutations: [{ id: "payload", scope: "both", effectSelector: { mode: "kind", kinds: ["damage", "heal"] }, ops: {} }] }
  ))

  const statsOnly = cloneTrees()
  const statsOnlyNode = statsOnly[0].nodes.find((row) => row.id === "strength_root_break_stance")
  const rankOneCore = statsOnlyNode.levels[0].grants.filter((grant) => grant.kind !== "stat_modifier")
  statsOnlyNode.levels[1].grants = [
    ...structuredClone(rankOneCore),
    ...statsOnlyNode.levels[1].grants.filter((grant) => grant.kind === "stat_modifier")
  ]
  assert.throws(() => validateTrees(statsOnly), /strictly enhance a non-stat core mechanic/)

  const reorderedOnly = cloneTrees()
  const reorderedNode = reorderedOnly[0].nodes.find((row) => row.id === "strength_root_break_stance")
  const reorderedCore = structuredClone(reorderedNode.levels[0].grants.filter((grant) => grant.kind !== "stat_modifier"))
  reorderedCore.find((grant) => grant.kind === "battle_hook").battleHooks.reverse()
  reorderedNode.levels[1].grants = [...reorderedCore, ...reorderedNode.levels[1].grants.filter((grant) => grant.kind === "stat_modifier")]
  assert.throws(() => validateTrees(reorderedOnly), /strictly enhance a non-stat core mechanic/)

  const reorderedAppendOnly = cloneTrees()
  const reorderedAppendOnlyNode = reorderedAppendOnly[0].nodes.find((row) => row.id === "strength_tech_rupture")
  const rankOneRuptureCore = structuredClone(reorderedAppendOnlyNode.levels[0].grants.filter((grant) => grant.kind !== "stat_modifier"))
  rankOneRuptureCore[0].effectMutations[0].ops.appendEffects.reverse()
  reorderedAppendOnlyNode.levels[1].grants = [...rankOneRuptureCore, ...reorderedAppendOnlyNode.levels[1].grants.filter((grant) => grant.kind === "stat_modifier")]
  assert.throws(() => validateTrees(reorderedAppendOnly), /strictly enhance a non-stat core mechanic/)

  const reorderedConditionsOnly = cloneTrees()
  const reorderedConditionsNode = reorderedConditionsOnly[0].nodes.find((row) => row.id === "strength_root_break_stance")
  const conditionCore = structuredClone(reorderedConditionsNode.levels[0].grants.filter((grant) => grant.kind !== "stat_modifier"))
  conditionCore.find((grant) => grant.kind === "battle_hook").battleHooks.find((hook) => hook.conditions.length > 1).conditions.reverse()
  reorderedConditionsNode.levels[1].grants = [...conditionCore, ...reorderedConditionsNode.levels[1].grants.filter((grant) => grant.kind === "stat_modifier")]
  assert.throws(() => validateTrees(reorderedConditionsOnly), /strictly enhance a non-stat core mechanic/)

  const reorderedSelectorValuesOnly = cloneTrees()
  const reorderedSelectorNode = reorderedSelectorValuesOnly[0].nodes.find((row) => row.id === "confluence_dexterity_intelligence_quick_formula")
  const selectorCore = structuredClone(reorderedSelectorNode.levels[0].grants.filter((grant) => grant.kind !== "stat_modifier"))
  selectorCore[0].effectMutations[0].skillCategories.reverse()
  reorderedSelectorNode.levels[1].grants = [...selectorCore, ...reorderedSelectorNode.levels[1].grants.filter((grant) => grant.kind === "stat_modifier")]
  assert.throws(() => validateTrees(reorderedSelectorValuesOnly), /strictly enhance a non-stat core mechanic/)

  const missingCopy = cloneTrees()
  missingCopy[0].i18n["zh-CN"].nodes.strength_root_break_stance.levelEffectSummary.pop()
  assert.throws(() => validateTrees(missingCopy), /levelEffectSummary must match node ranks/)
})

test("specialization terminals require the canonical eleven-point pure-school gate", () => {
  const missingGate = cloneTrees()
  const missingTerminal = missingGate[0].nodes.find((node) => node.kind === "terminal")
  missingTerminal.requirements.all = missingTerminal.requirements.all.filter((requirement) => requirement.kind !== "pure_school_points_gte")
  assert.throws(() => validateTrees(missingGate), /terminal must require 11 points already invested/)

  const wrongSchool = cloneTrees()
  const wrongTerminal = wrongSchool[0].nodes.find((node) => node.kind === "terminal")
  wrongTerminal.requirements.all.find((requirement) => requirement.kind === "pure_school_points_gte").schoolId = "dexterity"
  assert.throws(() => validateTrees(wrongSchool), /terminal must require 11 points already invested/)
})

test("specialization rejects a point-band total that disagrees with maxPoints", () => {
  const trees = cloneTrees()
  trees[0].pointBands[1].pointsPerGrant = 2
  assert.throws(() => validateTrees(trees), /maxPoints does not match pointBands/)
})

test("specialization preserves the level 200 and 300 front-loaded checkpoints", () => {
  const trees = cloneTrees()
  trees[0].pointBands.find((band) => band.id === "level_200").pointsPerGrant = 1
  trees[0].pointBands.find((band) => band.id === "level_cap").pointsPerGrant = 3
  assert.throws(() => validateTrees(trees), /point checkpoint invalid.*level=200/)
})

test("specialization requires exactly three active root routes", () => {
  const missing = cloneTrees()
  delete missing[0].maxActiveRootNodes
  assert.throws(() => validateTrees(missing), /maxActiveRootNodes invalid/)

  const tooBroad = cloneTrees()
  tooBroad[0].maxActiveRootNodes = 4
  assert.throws(() => validateTrees(tooBroad), /maxActiveRootNodes invalid/)
})

test("specialization rejects an unsafe equipment capacity envelope", () => {
  const trees = cloneTrees()
  const node = trees[0].nodes.find((row) => row.id === "strength_terminal_three_armed")
  node.levels[0].grants[0].deltas.accessory = -5
  assert.throws(() => validateTrees(trees), /capacity envelope outside bounds/)
})

test("specialization rejects an unsupported DSL schema version", () => {
  const trees = cloneTrees()
  trees[0].schemaVersion = 1
  assert.throws(
    () => validateTrees(trees),
    /schemaVersion unsupported/
  )
})

test("specialization capacity grants share the complete battle slot-family catalog", () => {
  const trees = cloneTrees()
  const node = trees[0].nodes.find((row) => row.id === "strength_terminal_three_armed")
  node.levels[0].grants[0].deltas = { cloak: -1, right_hand: 1 }
  validateTrees(trees)
})

test("specialization roots cannot be pure stat filler", () => {
  const trees = cloneTrees()
  const node = trees[0].nodes.find((row) => row.id === "stamina_root_hold_line")
  for (const [levelIndex, level] of node.levels.entries()) {
    level.grants = [{
      id: "stamina_stat_filler",
      kind: "stat_modifier",
      scope: "character",
      modifiers: [{ attributeKey: "stamina", operation: "add", value: levelIndex + 1 }]
    }]
  }
  assert.throws(() => validateTrees(trees), /strictly enhance a non-stat core mechanic|root must define a combat loop|must scale with school attributes|must define a gameplay grant/)
})

test("specialization player copy uses declared keyword tokens", () => {
  const missingToken = cloneTrees()
  const breakCopy = missingToken[0].i18n["zh-CN"].nodes.strength_root_break_stance.levelEffectSummary[0]
  const tokenLineIndex = breakCopy.findIndex((line) => line.includes("[[keyword:break_momentum]]"))
  breakCopy[tokenLineIndex] = breakCopy[tokenLineIndex].replace("[[keyword:break_momentum]]", "破势")
  assert.throws(() => validateTrees(missingToken), /must tag keyword break_momentum/)

  const undeclaredToken = cloneTrees()
  undeclaredToken[0].i18n["zh-CN"].nodes.strength_root_break_stance.levelEffectSummary[0][0] += " [[keyword:echo_cast]]"
  assert.throws(() => validateTrees(undeclaredToken), /keyword not declared by the node/)
})

test("specialization accepts the canonical optional event filter, limits, dispatch, and targeting patch shapes", () => {
  const trees = cloneTrees()
  const hook = firstHook(trees)
  hook.eventFilter = {
    skillIds: ["basic_attack"],
    skillCategories: ["blade"],
    damageTypes: ["physical"],
    includeBasicAttack: true,
    unitTypes: ["player"]
  }
  hook.limits = { perTurn: 1, perBattle: 3 }
  hook.dispatchHooks = false
  hookByTrigger(trees, "onTurnEnd").eventFilter = {
    turnKinds: ["ordered"],
    includePreparation: false,
    includeSuppressedTurns: false
  }
  mutateFirstMutationSnapshots(trees, (mutation) => {
    mutation.ops.targetingPatch = {
      targetScope: "enemy",
      targetPositions: ["vanguard"],
      secondarySeek: "front_to_rear",
      selectRule: "attr_max",
      selectAttrKey: "strength",
      fallbackSelfIfEmpty: false,
      targetCount: 1
    }
    mutation.effectSelector.includeGenerated = true
    mutation.skillCategories = ["blade"]
    mutation.weaponClasses = ["sword"]
    mutation.damageTypes = ["physical"]
  })
  validateTrees(trees)
})

test("specialization hook trigger reachability mirrors the battle runtime", async (t) => {
  const cases = [
    {
      name: "damage condition needs damage context",
      mutate: (trees) => { firstHook(trees).trigger = "onBattleStart" },
      pattern: /context_damage_type requires a damage trigger/
    },
    {
      name: "mp-insufficient condition needs before-skill context",
      mutate: (trees) => {
        const hook = firstHook(trees)
        hook.conditions = [{ kind: "context_mp_insufficient" }]
      },
      pattern: /context_mp_insufficient requires onBeforeSkillCast/
    },
    {
      name: "before-damage effect needs before-damage trigger",
      mutate: (trees) => {
        const hook = firstHook(trees)
        hook.conditions = []
        hook.effects = [{ kind: "runtime_damage_amp", value: 0.1, valueFormula: { baseAdd: 0.05, terms: [{ attributeKey: "strength", coefficient: 0.0001 }] } }]
      },
      pattern: /runtime_damage_amp requires a before-damage trigger/
    },
    {
      name: "context-skill cooldown needs skill context",
      mutate: (trees) => {
        const hook = trees[0].nodes.find((node) => node.id === "perception_tech_hunter_mark").levels[0].grants[0].battleHooks[0]
        hook.effects = [{ kind: "adjust_skill_cooldown", targetMode: "self", amount: -1, selectionMode: "context_skill", maxSkills: 1 }]
      },
      pattern: /context_skill requires a skill or damage trigger/
    },
    {
      name: "source target needs event subject",
      mutate: (trees) => {
        const hook = trees[0].nodes.find((node) => node.id === "perception_tech_hunter_mark").levels[0].grants[0].battleHooks[0]
        hook.effects = [{ kind: "heal", targetMode: "source", maxHpRatio: 0.1, valueFormula: { baseAdd: 0.05, terms: [{ attributeKey: "perception", coefficient: 0.0001 }] } }]
      },
      pattern: /targetMode source requires an event subject/
    },
    {
      name: "buff filters are unreachable",
      mutate: (trees) => { firstHook(trees).eventFilter = { buffIds: ["rage_power"] } },
      pattern: /buff trigger, which specializations do not expose/
    },
    {
      name: "probability filters are unreachable",
      mutate: (trees) => { firstHook(trees).eventFilter = { probabilityKinds: ["equipment_proc"] } },
      pattern: /probability trigger, which specializations do not expose/
    },
    {
      name: "skill filters need skill context",
      mutate: (trees) => {
        const hook = trees[0].nodes.find((node) => node.id === "perception_tech_hunter_mark").levels[0].grants[0].battleHooks[0]
        hook.eventFilter = { includeBasicAttack: true }
      },
      pattern: /includeBasicAttack requires a skill or damage trigger/
    },
    {
      name: "damage filters need damage context",
      mutate: (trees) => { hookByTrigger(trees, "onAfterSkillCast").eventFilter = { damageTypes: ["physical"] } },
      pattern: /damageTypes requires a damage trigger/
    },
    {
      name: "subject filters need event subject",
      mutate: (trees) => { hookByTrigger(trees, "onAfterSkillCast").eventFilter = { excludeSelf: true } },
      pattern: /excludeSelf requires an event subject/
    },
    {
      name: "turn filters need turn trigger",
      mutate: (trees) => { firstHook(trees).eventFilter = { turnKinds: ["ordered"] } },
      pattern: /turn filters require onTurnStart or onTurnEnd/
    },
    {
      name: "suppressed-turn filter needs turn end",
      mutate: (trees) => { hookByTrigger(trees, "onTurnStart").eventFilter = { includeSuppressedTurns: true } },
      pattern: /includeSuppressedTurns requires onTurnEnd/
    }
  ]
  for (const row of cases) {
    await t.test(row.name, () => {
      const trees = cloneTrees()
      row.mutate(trees)
      assert.throws(() => validateTrees(trees), row.pattern)
    })
  }
})

test("specialization hook effect cross-field contracts mirror the battle runtime", async (t) => {
  const effectByKind = (trees, kind) => trees[0].nodes.flatMap((node) => node.levels[0].grants)
    .flatMap((grant) => grant.battleHooks || [])
    .flatMap((hook) => hook.effects)
    .find((effect) => effect.kind === kind)
  const cases = [
    { name: "redirect is self only", mutate: (trees) => { effectByKind(trees, "redirect_damage").targetMode = "enemy" }, pattern: /targetMode must be self/ },
    { name: "retaliate targets source", mutate: (trees) => { effectByKind(trees, "retaliate").targetMode = "enemy" }, pattern: /targetMode must be source/ },
    { name: "extra turn targets self", mutate: (trees) => { effectByKind(trees, "extra_turn").targetMode = "enemy" }, pattern: /only supports self\/full/ },
    { name: "extra turn chance is capped", mutate: (trees) => { effectByKind(trees, "extra_turn").procChanceMax = 1.1 }, pattern: /procChanceMax/ },
    { name: "extra turn cap cannot be below base chance", mutate: (trees) => { effectByKind(trees, "extra_turn").procChanceMax = 0.1 }, pattern: /at least procChanceBase/ },
    { name: "extra turn abort requires probability", mutate: (trees) => { const effect = effectByKind(trees, "extra_turn"); delete effect.procChanceBase; delete effect.procChanceMax }, pattern: /abortHookOnFail requires procChanceBase/ },
    {
      name: "buff selection only targets enemies",
      mutate: (trees) => {
        const effect = trees[0].nodes.flatMap((node) => node.levels[0].grants)
          .flatMap((grant) => grant.battleHooks || [])
          .flatMap((hook) => hook.effects)
          .find((candidate) => candidate.kind === "add_buff" && candidate.selectionMode)
        effect.targetMode = "self"
      },
      pattern: /selectionMode requires enemy targetMode/
    },
    {
      name: "context skill selects at most one skill",
      mutate: (trees) => {
        const effect = effectByKind(trees, "adjust_skill_cooldown")
        effect.selectionMode = "context_skill"
        effect.maxSkills = 2
      },
      pattern: /maxSkills must be 1/
    },
    {
      name: "remove buff cannot carry duration",
      mutate: (trees) => {
        const hook = firstHook(trees)
        hook.conditions = []
        hook.effects = [{ kind: "remove_buff", buffId: "rage_power", durationTurns: 1, dispatchHooks: false }]
      },
      pattern: /durationTurns is an unknown key/
    },
    {
      name: "before-take heal must target self",
      mutate: (trees) => {
        const hook = hookByTrigger(trees, "onBeforeTakeDamage")
        hook.conditions = []
        hook.effects = [{ kind: "heal", targetMode: "lowest_hp_ally", maxHpRatio: 0.1, valueFormula: { baseAdd: 0.05, terms: [{ attributeKey: "stamina", coefficient: 0.0001 }] } }]
      },
      pattern: /onBeforeTakeDamage targetMode must be self/
    }
  ]
  for (const row of cases) {
    await t.test(row.name, () => {
      const trees = cloneTrees()
      row.mutate(trees)
      assert.throws(() => validateTrees(trees), row.pattern)
    })
  }
})

test("specialization targeting patches require scalar counts and paired attribute selection", async (t) => {
  const cases = [
    {
      name: "attribute rule requires key",
      patch: { selectRule: "attr_max" },
      pattern: /selectRule attr_max\/attr_min and selectAttrKey must be authored together/
    },
    {
      name: "attribute key requires rule",
      patch: { selectRule: "first", selectAttrKey: "strength" },
      pattern: /selectRule attr_max\/attr_min and selectAttrKey must be authored together/
    },
    {
      name: "target count is a scalar max",
      patch: { targetCount: { min: 1, max: 2 } },
      pattern: /targetCount invalid/
    }
  ]
  for (const row of cases) {
    await t.test(row.name, () => {
      const trees = cloneTrees()
      firstMutation(trees).ops.targetingPatch = row.patch
      assert.throws(() => validateTrees(trees), row.pattern)
    })
  }
})

test("specialization mutation selectors are dependency-ordered and operation compatible", async (t) => {
  const catalogEffects = [
    ...skills.flatMap((skill) => skill.effects || []),
    ...equipmentTemplates.flatMap((template) => template.basicAttackEffects || [])
  ]
  const equipmentDamageID = equipmentTemplates.flatMap((template) => template.basicAttackEffects || []).find((effect) => effect.kind === "damage").id
  const cases = [
    {
      name: "priority must be an integer",
      mutate: (trees) => { mutateFirstMutationSnapshots(trees, (mutation) => { mutation.priority = 0.5 }) },
      pattern: /priority invalid/
    },
    {
      name: "amount operations only target damage or heal",
      semanticOnly: true,
      mutate: (trees) => {
        mutateFirstMutationSnapshots(trees, (mutation) => {
          mutation.effectSelector = { mode: "kind", kinds: ["status"] }
          mutation.ops = { amountMultiplier: 1.05 }
        })
      },
      pattern: /amount cannot target effect kind: status/
    },
    {
      name: "hit count only targets damage",
      semanticOnly: true,
      mutate: (trees) => {
        mutateFirstMutationSnapshots(trees, (mutation) => {
          mutation.effectSelector = { mode: "kind", kinds: ["heal"] }
          mutation.ops = { hitCountDelta: 1, amountMultiplier: 0.7, amountTermsAppend: [{ attributeKey: "strength", coefficient: 0.001 }] }
        })
      },
      pattern: /damage cannot target effect kind: heal/
    },
    {
      name: "status operations only target status",
      semanticOnly: true,
      mutate: (trees) => {
        mutateFirstMutationSnapshots(trees, (mutation) => {
          mutation.effectSelector = { mode: "kind", kinds: ["damage"] }
          mutation.ops = { statusIdsAppend: ["rage_power"], amountTermsAppend: [{ attributeKey: "strength", coefficient: 0.001 }] }
        })
      },
      pattern: /status cannot target effect kind: damage/
    },
    {
      name: "damage append only targets damage",
      semanticOnly: true,
      mutate: (trees) => {
        mutateFirstMutationSnapshots(trees, (mutation) => {
          mutation.effectSelector = { mode: "kind", kinds: ["heal"] }
          mutation.ops = { amountTermsAppend: [{ attributeKey: "strength", coefficient: 0.001 }], appendEffects: [{ id: "test_runtime_amp", kind: "runtime_damage_amp", value: 0.1 }] }
        })
      },
      pattern: /appendEffects cannot target effect kind: heal/
    },
    {
      name: "generated selector requires includeGenerated",
      mutate: (trees) => {
        const producer = mutationDescriptors(trees).find(({ mutation }) => mutation.ops?.appendEffects?.some((effect) => effect.kind === "splash_damage"))
        appendConsumer(producer, producer.mutation.ops.appendEffects[0].id, { includeGenerated: false })
      },
      pattern: /without includeGenerated/
    },
    {
      name: "generated selector rejects self",
      mutate: (trees) => {
        const producer = mutationDescriptors(trees).find(({ mutation }) => mutation.ops?.appendEffects?.some((effect) => effect.kind === "splash_damage"))
        const effectID = producer.mutation.ops.appendEffects[0].id
        mutateMutationSnapshots(producer, (mutation) => {
          mutation.effectSelector = { mode: "effect_id", effectIds: [effectID], includeGenerated: true }
        })
      },
      pattern: /cannot select its own generated effect/
    },
    {
      name: "generated selector requires producer dependency",
      mutate: (trees) => {
        const producer = mutationDescriptors(trees).find(({ mutation }) => mutation.ops?.appendEffects?.some((effect) => effect.kind === "splash_damage"))
        const target = trees[0].nodes.find((node) => node.id === "strength_root_break_stance")
        const grant = {
          id: "test_unrelated_generated_consumer",
          kind: "effect_mutation",
          effectMutations: [{ id: "unrelated_consumer", priority: -1, scope: "both", effectSelector: { mode: "effect_id", effectIds: [producer.mutation.ops.appendEffects[0].id], includeGenerated: true }, ops: { amountMultiplier: 1.1 } }]
        }
        for (const level of target.levels) level.grants.push(structuredClone(grant))
      },
      pattern: /without requiring producer node/
    },
    {
      name: "generated selector executes after producer",
      mutate: (trees) => {
        const producer = mutationDescriptors(trees).find(({ mutation }) => mutation.ops?.appendEffects?.some((effect) => effect.kind === "splash_damage"))
        appendConsumer(producer, producer.mutation.ops.appendEffects[0].id, { priority: (producer.mutation.priority || 0) + 1 })
      },
      pattern: /before its producer executes/
    },
    {
      name: "non-materialized generated effects cannot be selected",
      mutate: (trees) => {
        const producer = mutationDescriptors(trees).find(({ mutation }) => mutation.ops?.appendEffects?.some((effect) => effect.kind === "consume_hp"))
        appendConsumer(producer, producer.mutation.ops.appendEffects.find((effect) => effect.kind === "consume_hp").id)
      },
      pattern: /kind consume_hp is not mutable/
    },
    {
      name: "generated effect cannot select skill scalar operations",
      semanticOnly: true,
      mutate: (trees) => {
        const producer = mutationDescriptors(trees).find(({ mutation }) => mutation.ops?.appendEffects?.some((effect) => effect.kind === "splash_damage"))
        appendConsumer(producer, producer.mutation.ops.appendEffects[0].id, { ops: { cooldownDelta: -1, cooldownFloor: 1, mpCostMultiplier: 3 } })
      },
      pattern: /generated effect .*skill scalar operations/
    },
    {
      name: "equipment basic effect cannot select skill scalar operations",
      semanticOnly: true,
      mutate: (trees) => {
        mutateFirstMutationSnapshots(trees, (mutation) => {
          mutation.effectSelector = { mode: "effect_id", effectIds: [equipmentDamageID], includeGenerated: false }
          mutation.ops = { cooldownDelta: -1, amountTermsAppend: [{ attributeKey: "strength", coefficient: 0.001 }] }
        })
      },
      pattern: /equipment basic-attack effect .*skill scalar operations/
    },
    {
      name: "static effect id compatibility uses catalog kind",
      semanticOnly: true,
      mutate: (trees) => {
        const statusID = catalogEffects.find((effect) => effect.kind === "status").id
        mutateFirstMutationSnapshots(trees, (mutation) => {
          mutation.effectSelector = { mode: "effect_id", effectIds: [statusID], includeGenerated: false }
          mutation.ops = { amountMultiplier: 1.05 }
        })
      },
      pattern: /amount cannot target effect kind: status/
    }
  ]
  for (const row of cases) {
    await t.test(row.name, () => {
      const trees = cloneTrees()
      row.mutate(trees)
      if (row.semanticOnly) {
        assert.throws(() => validateSpecializationMutationSemantics(trees, {
          effectKindsByID: new Map(catalogEffects.map((effect) => [effect.id, effect.kind])),
          equipmentEffectIDs: new Set(equipmentTemplates.flatMap((template) => (template.basicAttackEffects || []).map((effect) => effect.id)))
        }), row.pattern)
      } else {
        assert.throws(() => validateTrees(trees), row.pattern)
      }
    })
  }
})

test("specialization mutation semantics validate every rank and every reachable generated producer rank", () => {
  const lateRankKind = cloneTrees()
  const lateKindNode = lateRankKind[0].nodes.find((node) => node.id === "dexterity_tech_chain_steps")
  const lateKindMutation = lateKindNode.levels.at(-1).grants.find((grant) => grant.id === "grant_dexterity_tech_chain_steps").effectMutations[0]
  lateKindMutation.effectSelector = { mode: "kind", kinds: ["damage", "status"], includeGenerated: false }
  assert.throws(() => validateTrees(lateRankKind), /amount cannot target effect kind: status/)

  const generatedOrder = cloneTrees()
  const producer = mutationDescriptors(generatedOrder).find(({ mutation }) => mutation.ops?.appendEffects?.some((effect) => effect.kind === "splash_damage"))
  const effectID = producer.mutation.ops.appendEffects.find((effect) => effect.kind === "splash_damage").id
  appendConsumer(producer, effectID, { priority: -1 })
  validateTrees(generatedOrder)
  const rankTwoGrant = producer.node.levels[1].grants.find((grant) => grant.id === producer.grant.id)
  rankTwoGrant.effectMutations.find((mutation) => mutation.id === "test_consumer_2").priority = (producer.mutation.priority || 0) + 1
  assert.throws(() => validateTrees(generatedOrder), /before its producer executes/)

  const generatedGrant = (mutation) => ({ id: "generated_grant", kind: "effect_mutation", effectMutations: [mutation] })
  const producerMutation = (priority) => ({
    id: "generated_producer",
    priority,
    scope: "skill",
    effectSelector: { mode: "kind", kinds: ["damage"], includeGenerated: false },
    ops: { appendEffects: [{ id: "generated_rank_effect", kind: "splash_damage", targetCount: 1, amountRatio: 0.2 }] }
  })
  const consumerMutation = (priority) => ({
    id: "generated_consumer",
    priority,
    scope: "skill",
    effectSelector: { mode: "effect_id", effectIds: ["generated_rank_effect"], includeGenerated: true },
    ops: { amountMultiplier: 1.1 }
  })
  const semanticTree = (producerPriorities, consumerPriorities, producerMinRank = 1) => [{
    id: "semantic_tree",
    nodes: [
      {
        id: "producer_node",
        requirements: { all: [], anyOf: [] },
        levels: producerPriorities.map((priority, index) => ({ level: index + 1, grants: priority === null ? [] : [generatedGrant(producerMutation(priority))] }))
      },
      {
        id: "consumer_node",
        requirements: { all: [{ kind: "node_rank", nodeId: "producer_node", minRank: producerMinRank }], anyOf: [] },
        levels: consumerPriorities.map((priority, index) => ({ level: index + 1, grants: [generatedGrant(consumerMutation(priority))] }))
      }
    ]
  }]
  const semanticRefs = { effectKindsByID: new Map(), equipmentEffectIDs: new Set() }
  assert.throws(
    () => validateSpecializationMutationSemantics(semanticTree([null, 1], [0, 0], 1), semanticRefs),
    /without requiring producer node producer_node at rank 2/
  )
  assert.throws(
    () => validateSpecializationMutationSemantics(semanticTree([1, -1], [0, 0], 1), semanticRefs),
    /before its producer executes/
  )
})

test("specialization treats unfiltered skill damage as wide and rejects unsafe attribute scaling", () => {
  const trees = cloneTrees()
  const mutation = firstMutation(trees)
  mutation.scope = "skill"
  mutation.effectSelector = { mode: "kind", kinds: ["damage"] }
  delete mutation.skillIds
  delete mutation.skillCategories
  delete mutation.weaponClasses
  delete mutation.damageTypes
  mutation.ops = { amountTermsAppend: [{ attributeKey: "strength", coefficient: 0.041 }] }
  assert.throws(() => validateTrees(trees), /wide per-term\/total coefficient envelope/)
})

test("specialization direct extra actions always require a one-shot wave or battle outer cap", () => {
  const configure = (trees, trigger, { limits, conditionLimit } = {}) => {
    const node = trees[0].nodes.find((entry) => entry.id === "dexterity_terminal_stolen_beat")
    for (const level of node.levels) {
      const hook = level.grants.flatMap((grant) => grant.battleHooks || []).find((entry) => entry.id === "dexterity_stolen_beat_turn")
      hook.trigger = trigger
      hook.conditions = hook.conditions.filter((condition) => condition.kind !== "trigger_limit")
      if (conditionLimit) hook.conditions.push({ kind: "trigger_limit", ...conditionLimit })
      if (limits) hook.limits = { ...limits }
      else delete hook.limits
    }
  }
  for (const limits of [undefined, { perTurn: 1 }, { perWave: 2 }, { perBattle: 2 }]) {
    const trees = cloneTrees()
    configure(trees, "onKill", { limits })
    assert.throws(() => validateTrees(trees), /extra_turn requires a weapon gate and perWave\/perBattle limit|direct extra_turn requires an explicit perWave=1 or perBattle=1 outer limit/)
  }
  const conditionCapped = cloneTrees()
  configure(conditionCapped, "onKill", { limits: { perTurn: 1 }, conditionLimit: { perWave: 1 } })
  assert.doesNotThrow(() => validateTrees(conditionCapped))
  const hookCapped = cloneTrees()
  configure(hookCapped, "onKill", { limits: { perBattle: 1 }, conditionLimit: { perTurn: 1 } })
  assert.doesNotThrow(() => validateTrees(hookCapped))
})

test("specialization echoes and follow-up hits retain their non-recursive safety gates", () => {
  const mutateHooks = (trees, nodeID, hookID, mutate) => {
    const node = trees[0].nodes.find((entry) => entry.id === nodeID)
    for (const level of node.levels) {
      const hook = level.grants.flatMap((grant) => grant.battleHooks || []).find((entry) => entry.id === hookID)
      mutate(hook, level)
    }
  }

  const weakEchoGate = cloneTrees()
  mutateHooks(weakEchoGate, "intelligence_terminal_first_echo", "intelligence_first_echo_cast", (hook) => {
    hook.conditions.find((condition) => condition.kind === "self_mp_ratio_gte").value = 0.5
  })
  assert.throws(() => validateTrees(weakEchoGate), /mirror_cast_skill requires a weapon gate, non-basic skill, 75% MP gate/)

  const recursiveEcho = cloneTrees()
  mutateHooks(recursiveEcho, "intelligence_terminal_first_echo", "intelligence_first_echo_cast", (hook) => {
    hook.effects.find((effect) => effect.kind === "mirror_cast_skill").allowMirroredCastTrigger = true
  })
  assert.throws(() => validateTrees(recursiveEcho), /mirror_cast_skill must be non-recursive and capped at 55%/)

  const unboundedFollowUp = cloneTrees()
  mutateHooks(unboundedFollowUp, "dexterity_root_deadly_strike", "scythe_deadly_repeat", (hook) => { delete hook.limits })
  assert.throws(() => validateTrees(unboundedFollowUp), /repeat_context_effect requires a weapon gate and explicit trigger limit/)

  const oversizedFollowUp = cloneTrees()
  mutateHooks(oversizedFollowUp, "dexterity_root_deadly_strike", "scythe_deadly_repeat", (hook, level) => {
    hook.effects.find((effect) => effect.kind === "repeat_context_effect").amountRatio = 0.45 + level.level * 0.01
  })
  assert.throws(() => validateTrees(oversizedFollowUp), /repeat_context_effect must be non-recursive and capped at 45%/)
})

test("specialization character formulas must use exactly their declared school attributes", () => {
  const replaceFormulaAttribute = (value, from, to) => {
    if (Array.isArray(value)) return value.forEach((entry) => replaceFormulaAttribute(entry, from, to))
    if (!value || typeof value !== "object") return
    if (value.attributeKey === from && Number.isFinite(value.coefficient)) value.attributeKey = to
    for (const child of Object.values(value)) replaceFormulaAttribute(child, from, to)
  }

  const foreignSingle = cloneTrees()
  replaceFormulaAttribute(foreignSingle[0].nodes.find((node) => node.id === "strength_root_break_stance").levels, "strength", "dexterity")
  assert.throws(() => validateTrees(foreignSingle), /formula school attributes must exactly match schoolIds/)

  const oneSidedConfluence = cloneTrees()
  replaceFormulaAttribute(oneSidedConfluence[0].nodes.find((node) => node.id === "confluence_strength_perception_pressure_point").levels, "perception", "strength")
  assert.throws(() => validateTrees(oneSidedConfluence), /formula school attributes must exactly match schoolIds/)

  const genericRuntimeAttribute = cloneTrees()
  replaceFormulaAttribute(genericRuntimeAttribute[0].nodes.find((node) => node.id === "strength_root_break_stance").levels, "strength", "hp")
  assert.throws(() => validateTrees(genericRuntimeAttribute), /formula school attributes must exactly match schoolIds/)
})

test("specialization payload IDs are lowercase snake case and mutation ordering uses stable IDs", () => {
  for (const [kind, mutate] of [
    ["grant", (trees) => { trees[0].nodes[0].levels.forEach((level) => { level.grants[0].id = "bad:grant" }) }],
    ["hook", (trees) => { trees[0].nodes.find((node) => node.id === "strength_root_break_stance").levels.forEach((level) => { level.grants.find((grant) => grant.kind === "battle_hook").battleHooks[0].id = "BadHook" }) }],
    ["mutation", (trees) => { trees[0].nodes.find((node) => node.id === "dexterity_terminal_reaping_sweep").levels.forEach((level) => { level.grants.find((grant) => grant.kind === "effect_mutation").effectMutations[0].id = "bad-mutation" }) }]
  ]) {
    const trees = cloneTrees()
    mutate(trees)
    assert.throws(() => validateTrees(trees), /id must match \/\^\[a-z\]\[a-z0-9_\]\*\$\//, kind)
  }

  const reordered = cloneTrees()
  const node = reordered[0].nodes.find((entry) => entry.id === "dexterity_terminal_reaping_sweep")
  for (const level of node.levels) {
    const grant = level.grants.find((entry) => entry.kind === "effect_mutation")
    const generatedEffectID = grant.effectMutations[0].ops.appendEffects[0].id
    grant.effectMutations.push({
      id: "zz_generated_consumer",
      priority: 0,
      scope: "both",
      effectSelector: { mode: "effect_id", effectIds: [generatedEffectID], includeGenerated: true },
      ops: { amountMultiplier: 1.05 }
    })
  }
  assert.doesNotThrow(() => validateTrees(reordered))
  for (const level of node.levels) level.grants.find((entry) => entry.kind === "effect_mutation").effectMutations.reverse()
  assert.doesNotThrow(() => validateTrees(reordered))
})

test("specialization DSL rejects unknown hook, trigger, condition, effect, and mutation op keys", async (t) => {
  const cases = [
    {
      name: "hook key",
      mutate: (trees) => { firstHook(trees).typoPriority = 10 },
      pattern: /typoPriority is an unknown key/
    },
    {
      name: "trigger",
      mutate: (trees) => { firstHook(trees).trigger = "onDefinitelyNotACombatEvent" },
      pattern: /trigger invalid/
    },
    {
      name: "condition kind",
      mutate: (trees) => { firstHook(trees).conditions = [{ kind: "unknown_predicate" }] },
      pattern: /conditions\[0\]\.kind invalid/
    },
    {
      name: "effect kind",
      mutate: (trees) => { firstHook(trees).effects = [{ kind: "unknown_effect" }] },
      pattern: /effects\[0\]\.kind invalid/
    },
    {
      name: "mutation op",
      mutate: (trees) => { firstMutation(trees).ops.unknownOp = 1 },
      pattern: /ops\.unknownOp is an unknown key/
    },
    {
      name: "event filter key",
      mutate: (trees) => { firstHook(trees).eventFilter = { unknownFilter: true } },
      pattern: /eventFilter\.unknownFilter is an unknown key/
    },
    {
      name: "limit key",
      mutate: (trees) => { firstHook(trees).limits = { perRound: 1 } },
      pattern: /limits\.perRound is an unknown key/
    },
    {
      name: "unsupported per-target limit",
      mutate: (trees) => { firstHook(trees).limits = { perTarget: 1 } },
      pattern: /limits\.perTarget is an unknown key/
    },
    {
      name: "dispatchHooks type",
      mutate: (trees) => { firstHook(trees).dispatchHooks = "false" },
      pattern: /dispatchHooks must be boolean/
    },
    {
      name: "dispatchHooks required",
      mutate: (trees) => { delete firstHook(trees).dispatchHooks },
      pattern: /dispatchHooks must be boolean/
    },
    {
      name: "targeting patch key",
      mutate: (trees) => { firstMutation(trees).ops.targetingPatch = { targetScope: "enemy", typoSeek: "front_to_rear" } },
      pattern: /targetingPatch\.typoSeek is an unknown key/
    },
    {
      name: "generated effect selector type",
      mutate: (trees) => { firstMutation(trees).effectSelector.includeGenerated = "yes" },
      pattern: /effectSelector\.includeGenerated must be boolean/
    },
    {
      name: "generated effect stable id",
      mutate: (trees) => {
        const mutation = trees[0].nodes
          .flatMap((node) => node.levels[0].grants)
          .flatMap((grant) => grant.effectMutations || [])
          .find((entry) => entry.ops?.appendEffects?.length)
        delete mutation.ops.appendEffects[0].id
      },
      pattern: /appendEffects\[0\]\.id must be a non-empty string/
    }
  ]
  for (const row of cases) {
    await t.test(row.name, () => {
      const trees = cloneTrees()
      row.mutate(trees)
      assert.throws(() => validateTrees(trees), row.pattern)
    })
  }
})

test("specialization DSL rejects dangling item, attribute, skill, buff, and effect references", async (t) => {
  const cases = [
    {
      name: "item",
      mutate: (trees) => { trees[0].reset.itemId = "missing_item" },
      pattern: /reset\.itemId not found/
    },
    {
      name: "attribute",
      mutate: (trees) => {
        const hook = firstHook(trees)
        hook.effects = [{
          kind: "damage",
          targetMode: "enemy",
          damageType: "physical",
          amountFormula: { baseAdd: 0, terms: [{ attributeKey: "missing_attribute", coefficient: 1 }] }
        }]
      },
      pattern: /attributeKey not found: missing_attribute/
    },
    {
      name: "skill",
      mutate: (trees) => { firstMutation(trees).skillIds = ["missing_skill"] },
      pattern: /skillIds reference not found: missing_skill/
    },
    {
      name: "buff",
      mutate: (trees) => { firstMutation(trees).ops.statusIdsAppend = ["missing_buff"] },
      pattern: /statusIdsAppend reference not found: missing_buff/
    },
    {
      name: "effect id",
      mutate: (trees) => { firstMutation(trees).effectSelector = { mode: "effect_id", effectIds: ["missing_effect"] } },
      pattern: /effectIds reference not found: missing_effect/
    },
    {
      name: "weapon class",
      mutate: (trees) => { firstMutation(trees).weaponClasses = ["missing_weapon_class"] },
      pattern: /weaponClasses reference not found: missing_weapon_class/
    }
  ]
  for (const row of cases) {
    await t.test(row.name, () => {
      const trees = cloneTrees()
      row.mutate(trees)
      assert.throws(() => validateTrees(trees), row.pattern)
    })
  }
})
