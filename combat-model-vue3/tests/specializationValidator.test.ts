import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import {
  SPECIALIZATION_CONDITION_KINDS,
  SPECIALIZATION_HOOK_EFFECT_KINDS,
  SPECIALIZATION_MUTATION_OP_KEYS,
  SPECIALIZATION_RECURSIVE_EFFECT_KINDS,
  collectSpecializationEffectIds,
  createDefaultSpecializationNode,
  createNextSpecializationLevel,
  createDefaultSpecializationTree,
  normalizeSpecializationTrees,
  specializationGrantSnapshotRetentionProblems,
  validateSpecializationMutationSemantics,
  validateSpecializationTrees
} from '../src/model/specialization.ts';
import {
  createConditionVariants,
  createEventFilterSchema,
  createHookEffectVariants,
  createMutationOpsSchema,
  type SpecializationDslRefs
} from '../src/components/specializationDslSchemas.ts';

type Row = Record<string, any>;

function loadTable(name: string): Row[] {
  const path = new URL(`../../packages/game-data/source/json/${name}.json`, import.meta.url);
  return JSON.parse(readFileSync(path, 'utf8'))[name];
}

const skills = loadTable('skills');
const buffs = loadTable('buff');
const attributes = loadTable('attributes');
const items = loadTable('items');
const equipmentTemplates = loadTable('equipment_templates');
const skillEffects = skills.flatMap((row) => row.effects ?? []);
const equipmentEffects = equipmentTemplates.flatMap((row) => row.basicAttackEffects ?? []);
const sourceEffects = [...skillEffects, ...equipmentEffects];
const refs = {
  attributeIds: attributes.map((row) => row.id),
  itemIds: items.map((row) => row.id),
  skillIds: skills.map((row) => row.id),
  skillCategories: skills.map((row) => row.category).filter(Boolean),
  weaponClasses: equipmentTemplates.flatMap((row) => row.weaponClass ?? []).filter(Boolean),
  buffIds: buffs.map((row) => row.id),
  buffKinds: buffs.map((row) => row.kind).filter(Boolean),
  skillEffectKinds: sourceEffects.map((effect: Row) => effect.kind).filter(Boolean),
  effectIds: sourceEffects.flatMap((effect: Row) => [effect.id, effect.effectId]).filter(Boolean),
  effectKindsById: Object.fromEntries(sourceEffects.flatMap((effect: Row) => [effect.id, effect.effectId].filter(Boolean).map((id) => [id, effect.kind]))),
  equipmentEffectIds: equipmentEffects.flatMap((effect: Row) => [effect.id, effect.effectId]).filter(Boolean)
};

function trees(): Row[] {
  return structuredClone(loadTable('specialization_trees'));
}

function hooks(rows: Row[]): Row[] {
  return rows.flatMap((tree) => tree.nodes).flatMap((node) => node.levels).flatMap((level) => level.grants).filter((grant) => grant.kind === 'battle_hook').flatMap((grant) => grant.battleHooks);
}

function mutations(rows: Row[]): Row[] {
  return rows.flatMap((tree) => tree.nodes).flatMap((node) => node.levels).flatMap((level) => level.grants).filter((grant) => grant.kind === 'effect_mutation').flatMap((grant) => grant.effectMutations);
}

function appendedEffects(rows: Row[]): Row[] {
  return rows.flatMap((tree) => tree.nodes).flatMap((node) => node.levels[0].grants)
    .filter((grant) => grant.kind === 'effect_mutation')
    .flatMap((grant) => grant.effectMutations)
    .flatMap((mutation) => mutation.ops?.appendEffects ?? []);
}

function mutationOwner(rows: Row[], predicate: (mutation: Row) => boolean): { tree: Row; node: Row; grant: Row; mutation: Row } {
  for (const tree of rows) for (const node of tree.nodes) for (const level of node.levels) for (const grant of level.grants) {
    if (grant.kind !== 'effect_mutation') continue;
    const mutation = grant.effectMutations.find(predicate);
    if (mutation) return { tree, node, grant, mutation };
  }
  throw new Error('mutation owner not found');
}

function mutateMutationSnapshots(owner: { node: Row; grant: Row; mutation: Row }, mutate: (mutation: Row, level: Row) => void) {
  for (const level of owner.node.levels) {
    const grant = level.grants.find((entry: Row) => entry.id === owner.grant.id);
    const mutation = grant.effectMutations.find((entry: Row) => entry.id === owner.mutation.id);
    mutate(mutation, level);
  }
}

function mutateFirstMutationSnapshots(rows: Row[], mutate: (mutation: Row, level: Row) => void) {
  mutateMutationSnapshots(mutationOwner(rows, () => true), mutate);
}

function appendMutationToGrantSnapshots(owner: { node: Row; grant: Row }, mutation: Row) {
  for (const level of owner.node.levels) {
    const grant = level.grants.find((entry: Row) => entry.id === owner.grant.id);
    grant.effectMutations.push(structuredClone(mutation));
  }
}

function makeDispatchExplicit(rows: Row[]) {
  const recursiveKinds = new Set<string>(SPECIALIZATION_RECURSIVE_EFFECT_KINDS);
  for (const hook of hooks(rows)) {
    hook.dispatchHooks ??= false;
    for (const effect of hook.effects) if (recursiveKinds.has(effect.kind)) effect.dispatchHooks ??= false;
  }
}

function expectIssue(rows: Row[], fragment: string) {
  const messages = validateSpecializationTrees(rows, refs).map((issue) => issue.message);
  assert.ok(messages.some((message) => message.includes(fragment)), `expected issue containing ${fragment}\n${messages.join('\n')}`);
}

test('current specialization source passes the editor validator', () => {
  const rows = trees();
  assert.equal(rows[0].rulesetVersion, 'specialization-v6');
  assert.equal(rows[0].balanceRevision, 4);
  assert.equal(createDefaultSpecializationTree().rulesetVersion, 'specialization-v6');
  assert.equal(createDefaultSpecializationTree().balanceRevision, 1);
  assert.equal(createDefaultSpecializationNode(['strength', 'dexterity']).levels[0].cost, 6);
  assert.equal(rows[0].maxPoints, 65);
  assert.equal(rows[0].maxActiveRootNodes, 3);
  assert.equal(rows[0].nodes.reduce((total: number, node: Row) => total + node.levels.reduce((nodeTotal: number, level: Row) => nodeTotal + level.cost, 0), 0), 604);
  assert.equal(rows[0].nodes.filter((node: Row) => node.kind === 'root').every((node: Row) => JSON.stringify(node.levels.map((level: Row) => level.cost)) === '[2,1,1,1,1,1]'), true);
  assert.equal(rows[0].nodes.filter((node: Row) => node.kind === 'confluence' && node.id !== 'confluence_intelligence_perception_crownless_astrolabe').every((node: Row) => node.levels[0].cost === 6), true);
  assert.equal(rows[0].nodes.find((node: Row) => node.id === 'confluence_intelligence_perception_crownless_astrolabe').levels[0].cost, 10);
  assert.equal(rows[0].nodes.flatMap((node: Row) => node.levels).flatMap((level: Row) => level.grants).some((grant: Row) => grant.kind === 'stat_modifier'), false);
  assert.equal('migrations' in rows[0], false);
  assert.deepEqual(validateSpecializationTrees(rows, refs), []);

  const legacyPrice = trees();
  legacyPrice[0].nodes.find((node: Row) => node.kind === 'confluence').levels[0].cost = 3;
  expectIssue(legacyPrice, 'Lv.1 cost 必须是 6');
});

test('specialization balance revision is explicit and positive', () => {
  const missing = trees();
  delete missing[0].balanceRevision;
  expectIssue(missing, 'balanceRevision');

  const zero = trees();
  zero[0].balanceRevision = 0;
  expectIssue(zero, 'balanceRevision');
});

test('specialization active root route limit is explicit and fixed at three', () => {
  const missing = trees();
  delete missing[0].maxActiveRootNodes;
  expectIssue(missing, '最多激活根路线必须固定为 3');

  const tooBroad = trees();
  tooBroad[0].maxActiveRootNodes = 4;
  expectIssue(tooBroad, '最多激活根路线必须固定为 3');
});

test('specialization editor preserves the level 200 and 300 point checkpoints', () => {
  const shifted = trees();
  shifted[0].pointBands.find((band: Row) => band.id === 'level_200').pointsPerGrant = 1;
  shifted[0].pointBands.find((band: Row) => band.id === 'level_cap').pointsPerGrant = 3;
  expectIssue(shifted, '200级必须累计获得 40 点');
});

test('new specialization ranks clone the complete previous grant snapshot', () => {
  const node = trees()[0].nodes.find((entry: Row) => entry.id === 'strength_tech_rupture');
  const next = createNextSpecializationLevel(node.levels[0], node.kind);
  assert.equal(next.level, 2);
  assert.equal(next.cost, 1);
  assert.deepEqual(next.grants, node.levels[0].grants);
  assert.notEqual(next.grants, node.levels[0].grants);
  next.grants[0].id = 'changed_without_aliasing';
  assert.notEqual(next.grants[0].id, node.levels[0].grants[0].id);
});

test('editor requires the canonical eleven-point pure-school gate on every terminal', () => {
  const rows = trees();
  const terminals = rows[0].nodes.filter((node: Row) => node.kind === 'terminal');
  assert.equal(terminals.length, 20);
  for (const node of terminals) assert.deepEqual(node.requirements.all.filter((requirement: Row) => requirement.kind === 'pure_school_points_gte'), [
    { kind: 'pure_school_points_gte', schoolId: node.schoolIds[0], points: 11 }
  ]);

  const missingGate = trees();
  missingGate[0].nodes.find((node: Row) => node.kind === 'terminal').requirements.all = missingGate[0].nodes.find((node: Row) => node.kind === 'terminal').requirements.all.filter((requirement: Row) => requirement.kind !== 'pure_school_points_gte');
  expectIssue(missingGate, '终式必须要求本流派已投入 11 点');

  const wrongGate = trees();
  wrongGate[0].nodes.find((node: Row) => node.kind === 'terminal').requirements.all.find((requirement: Row) => requirement.kind === 'pure_school_points_gte').points = 10;
  expectIssue(wrongGate, '终式必须要求本流派已投入 11 点');
});

test('editor accepts local terminal choices and rejects ambiguous or cross-root routes', () => {
  const valid = trees();
  const flexible = valid[0].nodes.find((node: Row) => node.id === 'dexterity_terminal_reaping_sweep');
  assert.equal(flexible.requirements.all.some((requirement: Row) => requirement.kind === 'node_rank'), false);
  assert.deepEqual(flexible.requirements.anyOf[0].map((requirement: Row) => requirement.nodeId), [
    'dexterity_tech_chain_steps',
    'dexterity_tech_critical_reprise'
  ]);
  assert.deepEqual(validateSpecializationTrees(valid, refs), []);

  const crossRoot = trees();
  crossRoot[0].nodes.find((node: Row) => node.id === 'dexterity_terminal_reaping_sweep').requirements.anyOf[0][1].nodeId = 'dexterity_tech_rapid_strike';
  expectIssue(crossRoot, '同一根基下的两个技巧任选其一');

  const mixed = trees();
  mixed[0].nodes.find((node: Row) => node.id === 'dexterity_terminal_reaping_sweep').requirements.all.push({
    kind: 'node_rank',
    nodeId: 'dexterity_tech_chain_steps',
    minRank: 1
  });
  expectIssue(mixed, '同一根基下的两个技巧任选其一');
});

test('localized player effect summaries survive the editor save round trip', () => {
  const rows = trees();
  const nodeId = rows[0].nodes[0].id;
  rows[0].i18n['zh-CN'].nodes[nodeId].name = '往返名称';
  rows[0].i18n['zh-CN'].nodes[nodeId].desc = '往返风味。';
  const tokenPrefix = rows[0].nodes[0].keywordIds.map((keywordId: string) => `[[keyword:${keywordId}]]`).join(' ');
  const sevenSummaryLines = Array.from({ length: 7 }, (_, index) => `${tokenPrefix} 第 ${index + 1} 条效果可以独立排序。`);
  rows[0].i18n['zh-CN'].nodes[nodeId].effectSummary = sevenSummaryLines;
  rows[0].i18n['zh-CN'].nodes[nodeId].levelEffectSummary = rows[0].nodes[0].levels.map((level: Row) => [`${tokenPrefix} 等级 ${level.level} 的实际收益。`]);
  const roundTripped = JSON.parse(JSON.stringify(normalizeSpecializationTrees(rows)));
  assert.deepEqual(roundTripped[0].i18n['zh-CN'].nodes[nodeId], {
    name: '往返名称',
    desc: '往返风味。',
    effectSummary: sevenSummaryLines,
    levelEffectSummary: rows[0].nodes[0].levels.map((level: Row) => [`${tokenPrefix} 等级 ${level.level} 的实际收益。`])
  });
  assert.deepEqual(validateSpecializationTrees(roundTripped, refs), []);

  const eightLines = trees();
  eightLines[0].i18n['zh-CN'].nodes[nodeId].effectSummary = Array.from({ length: 8 }, (_, index) => `${tokenPrefix} 第 ${index + 1} 条。`);
  expectIssue(eightLines, 'effectSummary 必须包含 1-7 行');
});

test('editor validator rejects numeric values that the game-data build rejects', () => {
  const badSchemaVersion = trees();
  badSchemaVersion[0].schemaVersion = 1;
  expectIssue(badSchemaVersion, 'schemaVersion 必须固定为 2');

  const badPriority = trees();
  hooks(badPriority)[0].priority = 0.5;
  expectIssue(badPriority, '.priority: 数值非法');

  const badLimit = trees();
  hooks(badLimit)[0].limits = { perTurn: 0 };
  expectIssue(badLimit, '.limits.perTurn: 数值非法');

  const badRatio = trees();
  const ratioCondition = hooks(badRatio).flatMap((hook) => hook.conditions).find((condition) => condition.kind === 'self_hp_ratio_lte');
  ratioCondition.value = 2;
  expectIssue(badRatio, '.value: 数值非法');

  const badFormula = trees();
  const damageEffect = hooks(badFormula).flatMap((hook) => hook.effects).find((effect) => effect.kind === 'damage');
  damageEffect.amountFormula.terms[0].coefficient = 0;
  expectIssue(badFormula, '.coefficient: 数值非法');

  const badValueFormula = trees();
  const valueFormulaEffect = hooks(badValueFormula).flatMap((hook) => hook.effects).find((effect) => effect.valueFormula);
  valueFormulaEffect.valueFormula.terms[0].coefficient = 0;
  expectIssue(badValueFormula, '.valueFormula.terms[0].coefficient: 数值非法');

  const badDelta = trees();
  mutations(badDelta).find((mutation) => mutation.ops.cooldownDelta !== undefined).ops.cooldownDelta = 0.5;
  expectIssue(badDelta, '.ops.cooldownDelta: 数值非法');

  const missingCooldownFloor = trees();
  const cooldownOwner = mutationOwner(missingCooldownFloor, (mutation) => mutation.ops.cooldownDelta < 0);
  mutateMutationSnapshots(cooldownOwner, (mutation) => { delete mutation.ops.cooldownFloor; });
  expectIssue(missingCooldownFloor, '降低冷却时 cooldownFloor 必须 >= 1');

  const cheapCooldown = trees();
  const cheapCooldownOwner = mutationOwner(cheapCooldown, (mutation) => mutation.ops.cooldownDelta < 0);
  mutateMutationSnapshots(cheapCooldownOwner, (mutation) => { mutation.ops.mpCostMultiplier = 2.99; });
  expectIssue(cheapCooldown, '降低冷却时 mpCostMultiplier 必须 >= 3');

  const discountedCooldown = trees();
  const discountedCooldownOwner = mutationOwner(discountedCooldown, (mutation) => mutation.ops.cooldownDelta < 0);
  mutateMutationSnapshots(discountedCooldownOwner, (mutation) => { mutation.ops.mpCostDelta = -1; });
  expectIssue(discountedCooldown, '降低冷却时 mpCostDelta 必须 >= 0');

  const weaponlessCooldown = trees();
  const weaponlessCooldownOwner = mutationOwner(weaponlessCooldown, (mutation) => mutation.ops.cooldownDelta < 0);
  mutateMutationSnapshots(weaponlessCooldownOwner, (mutation) => { delete mutation.weaponClasses; });
  expectIssue(weaponlessCooldown, '冷却缩减必须限定 weaponClasses');

  const outsideIntelligence = trees();
  const outsideIntelligenceOwner = mutationOwner(outsideIntelligence, (mutation) => mutation.ops.cooldownDelta < 0);
  outsideIntelligenceOwner.node.schoolIds = ['dexterity'];
  expectIssue(outsideIntelligence, '冷却缩减只允许出现在智力路线');

  const weaponlessExtraHit = trees();
  const weaponlessExtraHitOwner = mutationOwner(weaponlessExtraHit, (mutation) => mutation.ops.hitCountDelta > 0);
  mutateMutationSnapshots(weaponlessExtraHitOwner, (mutation) => { delete mutation.weaponClasses; });
  expectIssue(weaponlessExtraHit, '额外命中必须限定 weaponClasses');

  const badAppendEffect = trees();
  const appendMutation = mutations(badAppendEffect).find((mutation) => mutation.ops.appendEffects?.some((effect: Row) => effect.kind === 'status'));
  appendMutation.ops.appendEffects.find((effect: Row) => effect.kind === 'status').statusPayload.durationBaseTurns = 0;
  expectIssue(badAppendEffect, '.statusPayload.durationBaseTurns: 数值非法');

  const oversizedSecondary = trees();
  mutations(oversizedSecondary).find((mutation) => mutation.ops.appendEffects?.some((effect: Row) => effect.kind === 'secondary_damage'))
    .ops.appendEffects.find((effect: Row) => effect.kind === 'secondary_damage').amountRatio = 0.51;
  expectIssue(oversizedSecondary, '.amountRatio: 数值非法');

  const missingFixedRow = trees();
  const fixedSecondary = mutations(missingFixedRow).find((mutation) => mutation.ops.appendEffects?.some((effect: Row) => effect.kind === 'secondary_damage' && effect.targetPolicy === 'fixed_row'))
    .ops.appendEffects.find((effect: Row) => effect.kind === 'secondary_damage' && effect.targetPolicy === 'fixed_row');
  delete fixedSecondary.targetPosition;
  expectIssue(missingFixedRow, 'fixed_row 必须指定合法排位');

  const stackedSecondary = trees();
  const stackedMutation = mutations(stackedSecondary).find((mutation) => mutation.ops.appendEffects?.some((effect: Row) => effect.kind === 'secondary_damage'));
  const secondary = stackedMutation.ops.appendEffects.find((effect: Row) => effect.kind === 'secondary_damage');
  stackedMutation.ops.appendEffects.push({ ...structuredClone(secondary), id: `${secondary.id}_extra` });
  expectIssue(stackedSecondary, 'secondary_damage 最多只能配置 1 个');

  for (const mutate of [
    (mutation: Row) => { mutation.ops.targetCountDelta = 1; },
    (mutation: Row) => { mutation.ops.hitCountDelta = 1; },
    (mutation: Row) => { mutation.ops.appendEffects.push({ id: 'secondary_with_splash', kind: 'splash_damage', targetCount: 1, amountRatio: 0.2 }); },
  ]) {
    const expanded = trees();
    const mutation = mutations(expanded).find((row) => row.ops.appendEffects?.some((effect: Row) => effect.kind === 'secondary_damage'));
    mutate(mutation);
    expectIssue(expanded, 'secondary_damage 不能与其他命中或目标扩展组合');
  }

  const badPerTarget = trees();
  hooks(badPerTarget)[0].limits = { perTarget: 1 };
  expectIssue(badPerTarget, '.limits.perTarget: 未知字段');

  const missingAppendId = trees();
  const appended = mutations(missingAppendId).find((mutation) => mutation.ops.appendEffects?.length).ops.appendEffects[0];
  delete appended.id;
  expectIssue(missingAppendId, '.id: 必须是非空字符串');

  const duplicateAppendId = trees();
  const appendList = mutations(duplicateAppendId).find((mutation) => mutation.ops.appendEffects?.length > 1).ops.appendEffects;
  appendList[1].id = appendList[0].id;
  expectIssue(duplicateAppendId, 'id 重复');

  const wideSkillDamage = trees();
  const wideMutation = mutations(wideSkillDamage).find((mutation) => mutation.ops.amountTermsAppend?.length);
  wideMutation.scope = 'skill';
  wideMutation.effectSelector = { mode: 'kind', kinds: ['damage'] };
  delete wideMutation.skillIds;
  delete wideMutation.skillCategories;
  delete wideMutation.weaponClasses;
  delete wideMutation.damageTypes;
  wideMutation.ops = { amountTermsAppend: [{ attributeKey: 'strength', coefficient: 0.041 }] };
  expectIssue(wideSkillDamage, '超出宽域单项或总系数上限');
});

test('structured editor schemas cover every accepted plan discriminator and operation', () => {
  const editorRefs: SpecializationDslRefs = {
    attributeIds: refs.attributeIds,
    skillIds: refs.skillIds,
    skillCategories: refs.skillCategories,
    weaponClasses: refs.weaponClasses,
    buffIds: refs.buffIds,
    buffKinds: refs.buffKinds,
    skillEffectKinds: refs.skillEffectKinds,
    effectIds: refs.effectIds
  };
  assert.deepEqual(Object.keys(createConditionVariants()).sort(), [...SPECIALIZATION_CONDITION_KINDS].sort());
  const effectVariants = createHookEffectVariants(editorRefs);
  assert.deepEqual(Object.keys(effectVariants).sort(), [...SPECIALIZATION_HOOK_EFFECT_KINDS].sort());
  assert.deepEqual(Object.keys(createMutationOpsSchema(editorRefs)).sort(), [...SPECIALIZATION_MUTATION_OP_KEYS].sort());
  for (const kind of SPECIALIZATION_RECURSIVE_EFFECT_KINDS) {
    assert.equal(effectVariants[kind].defaultValue.dispatchHooks, false, `${kind} must default dispatchHooks explicitly`);
    assert.equal(effectVariants[kind].schema.dispatchHooks.required, true, `${kind} dispatchHooks must stay visible and required`);
  }
  const formulaKinds = ['consume_counter_for_damage_amp', 'heal', 'prevent_lethal', 'redirect_damage', 'resource_debt_cast', 'resource_shield', 'restore_mp', 'runtime_damage_amp', 'shared_damage_reduction'];
  for (const kind of formulaKinds) assert.ok(effectVariants[kind].schema.valueFormula, `${kind} must expose valueFormula`);
  for (const kind of ['mirror_cast_skill', 'repeat_context_effect']) assert.equal(effectVariants[kind].schema.valueFormula, undefined, `${kind} must stay fixed-ratio`);
  assert.ok(effectVariants.extra_turn.schema.procChanceBase, 'extra_turn must expose base proc chance');
  assert.ok(effectVariants.extra_turn.schema.procChanceMax, 'extra_turn must expose its proc chance cap');
  assert.ok(effectVariants.extra_turn.schema.abortHookOnFail, 'extra_turn must expose failure gating for later effects');
  assert.equal(effectVariants.cycle_counter.schema.atMax.required, undefined, 'cycle_counter atMax must remain optional');
  const appendEffects = createMutationOpsSchema(editorRefs).appendEffects;
  for (const variant of Object.values(appendEffects.variants ?? {})) {
    assert.equal(variant.schema.id.required, true, 'append effect id must be required');
    assert.equal(typeof variant.defaultValue.id, 'string', 'append effect must receive a semantic stable id');
  }
  assert.deepEqual(Object.keys(createEventFilterSchema(editorRefs, 'onBattleStart')), []);
  assert.deepEqual(Object.keys(createEventFilterSchema(editorRefs, 'onAfterDealDamage')).sort(), ['damageTypes', 'excludeSelf', 'excludeSkillIds', 'includeBasicAttack', 'skillCategories', 'skillIds', 'unitTypes'].sort());
  assert.deepEqual(Object.keys(createEventFilterSchema(editorRefs, 'onTurnEnd')).sort(), ['includePreparation', 'includeSuppressedTurns', 'turnKinds'].sort());
  assert.ok(!('buffIds' in createEventFilterSchema(editorRefs)), 'specialization schema must never expose buff filters');
  assert.ok(!('probabilityKinds' in createEventFilterSchema(editorRefs)), 'specialization schema must never expose probability filters');
  assert.deepEqual(createHookEffectVariants(editorRefs, 'onAllyBeforeTakeDamage').redirect_damage.schema.targetMode.options, ['self']);
  assert.deepEqual(createHookEffectVariants(editorRefs, 'onAfterTakeDamage').retaliate.schema.targetMode.options, ['source']);
  assert.ok(!('durationTurns' in createHookEffectVariants(editorRefs).remove_buff.schema));
  assert.ok(!('context_damage_type' in createConditionVariants('onBattleStart')));
  assert.ok(!('runtime_damage_amp' in createHookEffectVariants(editorRefs, 'onBattleStart')));
});

test('hook validation mirrors runtime event-context reachability and specialization filter domains', () => {
  const conditionRows = trees();
  hooks(conditionRows)[0].conditions = [{ kind: 'context_mp_insufficient' }];
  expectIssue(conditionRows, '无法在 onAfterDealDamage');

  const effectRows = trees();
  hooks(effectRows)[0].effects = [{ kind: 'runtime_damage_amp', value: 0.1 }];
  expectIssue(effectRows, '效果 runtime_damage_amp 无法在 onAfterDealDamage');

  const skillFilterRows = trees();
  const battleStart = hooks(skillFilterRows).find((hook) => hook.trigger === 'onBattleStart');
  battleStart.eventFilter = { includeBasicAttack: true };
  expectIssue(skillFilterRows, 'onBattleStart 不提供该事件上下文');

  const damageFilterRows = trees();
  const afterSkill = hooks(damageFilterRows).find((hook) => hook.trigger === 'onAfterSkillCast');
  afterSkill.eventFilter = { damageTypes: ['physical'] };
  expectIssue(damageFilterRows, 'onAfterSkillCast 不提供该事件上下文');

  const forbiddenRows = trees();
  hooks(forbiddenRows)[0].eventFilter = { buffIds: [refs.buffIds[0]] };
  expectIssue(forbiddenRows, '专精未暴露 buff/probability 触发域');

  const probabilityRows = trees();
  hooks(probabilityRows)[0].eventFilter = { probabilityKinds: ['equipment_proc'] };
  expectIssue(probabilityRows, '专精未暴露 buff/probability 触发域');

  const subjectRows = trees();
  hooks(subjectRows).find((hook) => hook.trigger === 'onAfterSkillCast').eventFilter = { unitTypes: ['enemy'] };
  expectIssue(subjectRows, 'onAfterSkillCast 不提供该事件上下文');

  const suppressedTurnRows = trees();
  hooks(suppressedTurnRows).find((hook) => hook.trigger === 'onTurnStart').eventFilter = { includeSuppressedTurns: true };
  expectIssue(suppressedTurnRows, 'onTurnStart 不提供该事件上下文');

  const configureExtraTurn = (rows: Row[], trigger: string, options: { limits?: Row; conditionLimit?: Row }) => {
    const node = rows[0].nodes.find((entry: Row) => entry.id === 'dexterity_terminal_stolen_beat');
    for (const level of node.levels) {
      const hook = level.grants.flatMap((grant: Row) => grant.battleHooks ?? []).find((entry: Row) => entry.id === 'dexterity_stolen_beat_turn');
      hook.trigger = trigger;
      hook.conditions = hook.conditions.filter((condition: Row) => condition.kind !== 'trigger_limit');
      if (options.conditionLimit) hook.conditions.push({ kind: 'trigger_limit', ...options.conditionLimit });
      if (options.limits) hook.limits = { ...options.limits };
      else delete hook.limits;
    }
  };
  for (const trigger of ['onKill', 'onAfterDealCrit', 'onWaveStart']) for (const limits of [undefined, { perTurn: 1 }, { perWave: 2 }, { perBattle: 2 }]) {
    const unsafe = trees();
    configureExtraTurn(unsafe, trigger, { limits });
    expectIssue(unsafe, '直接额外行动必须明确限制为每波 1 次或每场 1 次');
  }
  const conditionCapped = trees();
  configureExtraTurn(conditionCapped, 'onKill', { limits: { perTurn: 1 }, conditionLimit: { perWave: 1 } });
  assert.deepEqual(validateSpecializationTrees(conditionCapped, refs), []);
  const hookCapped = trees();
  configureExtraTurn(hookCapped, 'onKill', { limits: { perBattle: 1 }, conditionLimit: { perTurn: 1 } });
  assert.deepEqual(validateSpecializationTrees(hookCapped, refs), []);
});

test('hook validation rejects unsupported target combinations and context-only fields', () => {
  const redirectRows = trees();
  hooks(redirectRows).flatMap((hook) => hook.effects).find((effect) => effect.kind === 'redirect_damage').targetMode = 'enemy';
  expectIssue(redirectRows, 'redirect_damage 仅允许 self');

  const retaliateRows = trees();
  hooks(retaliateRows).flatMap((hook) => hook.effects).find((effect) => effect.kind === 'retaliate').targetMode = 'self';
  expectIssue(retaliateRows, 'retaliate 仅允许 source');

  const sourceRows = trees();
  const onBattleStart = hooks(sourceRows).find((hook) => hook.trigger === 'onBattleStart');
  onBattleStart.effects[0].targetMode = 'source';
  expectIssue(sourceRows, 'source 需要事件目标上下文');

  const targetingRows = trees();
  const waveStart = hooks(targetingRows).find((hook) => hook.trigger === 'onWaveStart');
  waveStart.effects.push({ kind: 'adjust_skill_cooldown', targetMode: 'targeting', amount: -1, selectionMode: 'all' });
  expectIssue(targetingRows, 'targeting 需要事件目标上下文');

  const buffRows = trees();
  const selectedBuff = hooks(buffRows).flatMap((hook) => hook.effects).find((effect) => effect.kind === 'add_buff' && effect.selectionMode);
  selectedBuff.targetMode = 'self';
  expectIssue(buffRows, '仅能与 enemy targetMode 一起使用');

  const cooldownRows = trees();
  const cooldown = hooks(cooldownRows).flatMap((hook) => hook.effects).find((effect) => effect.kind === 'adjust_skill_cooldown');
  cooldown.selectionMode = 'context_skill';
  cooldown.maxSkills = 2;
  expectIssue(cooldownRows, 'context_skill 仅允许 1');

  const extraTurnRows = trees();
  hooks(extraTurnRows).flatMap((hook) => hook.effects).find((effect) => effect.kind === 'extra_turn').targetMode = 'enemy';
  expectIssue(extraTurnRows, '仅支持 self/full');

  const invalidExtraTurnChance = trees();
  hooks(invalidExtraTurnChance).flatMap((hook) => hook.effects).find((effect) => effect.kind === 'extra_turn').procChanceMax = 0.1;
  expectIssue(invalidExtraTurnChance, '不能低于 procChanceBase');

  const missingExtraTurnChance = trees();
  const missingChanceEffect = hooks(missingExtraTurnChance).flatMap((hook) => hook.effects).find((effect) => effect.kind === 'extra_turn');
  delete missingChanceEffect.procChanceBase;
  delete missingChanceEffect.procChanceMax;
  expectIssue(missingExtraTurnChance, '需要 procChanceBase');

  const removeRows = trees();
  hooks(removeRows)[0].effects.push({ kind: 'remove_buff', buffId: refs.buffIds[0], durationTurns: 1, dispatchHooks: false });
  expectIssue(removeRows, 'durationTurns: 未知字段');

  const healRows = trees();
  const lethalHook = hooks(healRows).find((hook) => hook.trigger === 'onBeforeTakeDamage');
  lethalHook.effects.find((effect: Row) => effect.kind === 'heal').targetMode = 'enemy';
  expectIssue(healRows, 'onBeforeTakeDamage 治疗仅允许 self');
});

test('targetingPatch requires paired attribute selection and a scalar positive targetCount max', () => {
  const missingAttribute = trees();
  mutations(missingAttribute)[0].ops.targetingPatch = { selectRule: 'attr_max' };
  expectIssue(missingAttribute, '必须与 selectAttrKey 同时填写');

  const orphanAttribute = trees();
  mutations(orphanAttribute)[0].ops.targetingPatch = { selectRule: 'first', selectAttrKey: 'strength' };
  expectIssue(orphanAttribute, '必须与 selectAttrKey 同时填写');

  const objectCount = trees();
  mutations(objectCount)[0].ops.targetingPatch = { targetCount: { base: 2, max: 3 } };
  expectIssue(objectCount, '.targetCount: 数值非法');

  const valid = trees();
  mutateFirstMutationSnapshots(valid, (mutation) => { mutation.ops.targetingPatch = { selectRule: 'attr_min', selectAttrKey: 'strength', targetCount: 2 }; });
  assert.deepEqual(validateSpecializationTrees(valid, refs), []);
});

test('editor validator accepts canonical mutation selectors and rejects legacy generated-effect keys', () => {
  const rows = trees();
  mutateFirstMutationSnapshots(rows, (mutation) => {
    mutation.skillCategories = [refs.skillCategories[0]];
    mutation.weaponClasses = [refs.weaponClasses[0]];
    mutation.damageTypes = ['physical'];
    mutation.effectSelector.includeGenerated = true;
  });
  assert.deepEqual(validateSpecializationTrees(rows, refs), []);

  mutations(rows)[0].effectSelector.includeGeneratedEffects = true;
  expectIssue(rows, 'includeGeneratedEffects: 未知字段');
});

test('editor validator limits attack-stat policy to the supported damage-only contract', () => {
  const invalidPolicy = trees();
  const invalidPolicyOwner = mutationOwner(invalidPolicy, (mutation) => mutation.ops?.attackStatPolicy !== undefined);
  mutateMutationSnapshots(invalidPolicyOwner, (mutation) => { mutation.ops.attackStatPolicy = 'future_policy'; });
  expectIssue(invalidPolicy, '.ops.attackStatPolicy: 非法');

  const broadSelector = trees();
  const broadSelectorOwner = mutationOwner(broadSelector, (mutation) => mutation.ops?.attackStatPolicy !== undefined);
  mutateMutationSnapshots(broadSelectorOwner, (mutation) => { mutation.effectSelector = { mode: 'all' }; });
  expectIssue(broadSelector, '.ops.attackStatPolicy: 必须显式限定为伤害效果');

  const healSelector = trees();
  const healSelectorOwner = mutationOwner(healSelector, (mutation) => mutation.ops?.attackStatPolicy !== undefined);
  mutateMutationSnapshots(healSelectorOwner, (mutation) => { mutation.effectSelector = { mode: 'kind', kinds: ['heal'] }; });
  expectIssue(healSelector, '操作 damage 不能用于效果类型 heal');
});

test('editor validator accepts a real catalog stable skill effect ID selector', () => {
  const rows = trees();
  assert.ok(refs.effectIds.includes('basic_attack_damage'));
  mutateFirstMutationSnapshots(rows, (mutation) => { mutation.effectSelector = { mode: 'effect_id', effectIds: ['basic_attack_damage'], includeGenerated: false }; });
  assert.deepEqual(validateSpecializationTrees(rows, refs), []);
});

test('editor validator accepts a real equipment basic-attack stable effect ID selector', () => {
  const rows = trees();
  assert.ok(refs.effectIds.includes('equipment_wood_sword_damage'));
  mutateFirstMutationSnapshots(rows, (mutation) => { mutation.effectSelector = { mode: 'effect_id', effectIds: ['equipment_wood_sword_damage'], includeGenerated: false }; });
  assert.deepEqual(validateSpecializationTrees(rows, refs), []);
});

test('editor registers appended stable effect IDs for generated-effect selectors and authoring options', () => {
  const rows = trees();
  const appendedEffectId = appendedEffects(rows).find((effect) => effect.kind === 'splash_damage').id;
  assert.ok(collectSpecializationEffectIds(rows, refs.effectIds).includes(appendedEffectId));
  const producer = mutationOwner(rows, (mutation) => mutation.ops?.appendEffects?.some((effect: Row) => effect.id === appendedEffectId));
  appendMutationToGrantSnapshots(producer, { id: 'dexterity_reaping_sweep_consumer', priority: 0, scope: 'skill', effectSelector: { mode: 'effect_id', effectIds: [appendedEffectId], includeGenerated: true }, ops: { amountMultiplier: 1.1 } });
  assert.deepEqual(validateSpecializationTrees(rows, refs), []);
});

test('editor binds every character formula attribute to the declared schools', () => {
  const replaceFormulaAttribute = (value: unknown, from: string, to: string): void => {
    if (Array.isArray(value)) return value.forEach((entry) => replaceFormulaAttribute(entry, from, to));
    if (!value || typeof value !== 'object') return;
    const row = value as Row;
    if (row.attributeKey === from && typeof row.coefficient === 'number' && Number.isFinite(row.coefficient)) row.attributeKey = to;
    Object.values(row).forEach((entry) => replaceFormulaAttribute(entry, from, to));
  };

  const foreignSingle = trees();
  replaceFormulaAttribute(foreignSingle[0].nodes.find((node: Row) => node.id === 'strength_root_break_stance').levels, 'strength', 'dexterity');
  expectIssue(foreignSingle, '使用流派属性公式时必须完整且仅覆盖 schoolIds');

  const oneSidedConfluence = trees();
  replaceFormulaAttribute(oneSidedConfluence[0].nodes.find((node: Row) => node.id === 'confluence_strength_perception_pressure_point').levels, 'perception', 'strength');
  expectIssue(oneSidedConfluence, '使用流派属性公式时必须完整且仅覆盖 schoolIds');

  const genericRuntimeAttribute = trees();
  replaceFormulaAttribute(genericRuntimeAttribute[0].nodes.find((node: Row) => node.id === 'strength_root_break_stance').levels, 'strength', 'hp');
  expectIssue(genericRuntimeAttribute, '使用流派属性公式时必须完整且仅覆盖 schoolIds');
});

test('editor payload IDs are lowercase snake case and mutation order is stable across array reorder', () => {
  const invalidGrant = trees();
  invalidGrant[0].nodes[0].levels.forEach((level: Row) => { level.grants[0].id = 'bad:grant'; });
  expectIssue(invalidGrant, 'id 必须使用小写字母、数字和下划线');

  const invalidHook = trees();
  invalidHook[0].nodes.find((node: Row) => node.id === 'strength_root_break_stance').levels.forEach((level: Row) => {
    level.grants.find((grant: Row) => grant.kind === 'battle_hook').battleHooks[0].id = 'BadHook';
  });
  expectIssue(invalidHook, 'id 必须使用小写字母、数字和下划线');

  const invalidMutation = trees();
  invalidMutation[0].nodes.find((node: Row) => node.id === 'dexterity_terminal_reaping_sweep').levels.forEach((level: Row) => {
    level.grants.find((grant: Row) => grant.kind === 'effect_mutation').effectMutations[0].id = 'bad-mutation';
  });
  expectIssue(invalidMutation, 'id 必须使用小写字母、数字和下划线');

  const reordered = trees();
  const node = reordered[0].nodes.find((entry: Row) => entry.id === 'dexterity_terminal_reaping_sweep');
  for (const level of node.levels) {
    const grant = level.grants.find((entry: Row) => entry.kind === 'effect_mutation');
    const generatedEffectId = grant.effectMutations[0].ops.appendEffects[0].id;
    grant.effectMutations.push({ id: 'zz_generated_consumer', priority: 0, scope: 'both', effectSelector: { mode: 'effect_id', effectIds: [generatedEffectId], includeGenerated: true }, ops: { amountMultiplier: 1.05 } });
  }
  assert.deepEqual(validateSpecializationTrees(reordered, refs), []);
  for (const level of node.levels) level.grants.find((entry: Row) => entry.kind === 'effect_mutation').effectMutations.reverse();
  assert.deepEqual(validateSpecializationTrees(reordered, refs), []);
});

test('mutation compatibility rejects operations that cannot apply to explicitly selected effect kinds', () => {
  const statusId = Object.entries(refs.effectKindsById).find(([, kind]) => kind === 'status')?.[0];
  const healId = Object.entries(refs.effectKindsById).find(([, kind]) => kind === 'heal')?.[0];
  assert.ok(statusId && healId);

  const amountRows = trees();
  mutateFirstMutationSnapshots(amountRows, (mutation) => {
    mutation.effectSelector = { mode: 'effect_id', effectIds: [statusId], includeGenerated: false };
    mutation.ops = { amountMultiplier: 1.1 };
  });
  expectIssue(amountRows, '操作 amount 不能用于效果类型 status');

  const hitRows = trees();
  mutateFirstMutationSnapshots(hitRows, (mutation) => {
    mutation.effectSelector = { mode: 'effect_id', effectIds: [healId], includeGenerated: false };
    mutation.ops = { hitCountDelta: 1 };
  });
  expectIssue(hitRows, '操作 damage 不能用于效果类型 heal');

  const statusRows = trees();
  mutateFirstMutationSnapshots(statusRows, (mutation) => {
    mutation.effectSelector = { mode: 'kind', kinds: ['damage'], includeGenerated: false };
    mutation.ops = { statusIdsAppend: [refs.buffIds[0]] };
  });
  expectIssue(statusRows, '操作 status 不能用于效果类型 damage');

  const appendRows = trees();
  mutateFirstMutationSnapshots(appendRows, (mutation) => {
    mutation.effectSelector = { mode: 'kind', kinds: ['heal'], includeGenerated: false };
    mutation.ops = { appendEffects: [{ id: 'invalid_heal_runtime_amp', kind: 'runtime_damage_amp', value: 0.1 }] };
  });
  expectIssue(appendRows, '操作 appendEffects 不能用于效果类型 heal');
});

test('generated effect consumers require includeGenerated, guaranteed producers, and runtime order', () => {
  const missingInclude = trees();
  const missingOwner = mutationOwner(missingInclude, (mutation) => mutation.ops?.appendEffects?.some((effect: Row) => effect.kind === 'splash_damage'));
  const splashId = missingOwner.mutation.ops.appendEffects.find((effect: Row) => effect.kind === 'splash_damage').id;
  appendMutationToGrantSnapshots(missingOwner, { id: 'missing_include_consumer', priority: 0, scope: 'skill', effectSelector: { mode: 'effect_id', effectIds: [splashId], includeGenerated: false }, ops: { amountMultiplier: 1.1 } });
  expectIssue(missingInclude, '必须启用 includeGenerated');

  const badOrder = trees();
  const orderOwner = mutationOwner(badOrder, (mutation) => mutation.ops?.appendEffects?.some((effect: Row) => effect.kind === 'splash_damage'));
  appendMutationToGrantSnapshots(orderOwner, { id: 'early_consumer', priority: (orderOwner.mutation.priority ?? 0) + 1, scope: 'skill', effectSelector: { mode: 'effect_id', effectIds: [splashId], includeGenerated: true }, ops: { amountMultiplier: 1.1 } });
  expectIssue(badOrder, '必须先于消费变异执行');

  const selfRows = trees();
  const selfOwner = mutationOwner(selfRows, (mutation) => mutation.ops?.appendEffects?.some((effect: Row) => effect.kind === 'splash_damage'));
  mutateMutationSnapshots(selfOwner, (mutation) => { mutation.effectSelector = { mode: 'effect_id', effectIds: [splashId], includeGenerated: true }; });
  expectIssue(selfRows, '不能选择自身生成的效果');

  const dependencyRows = trees();
  const dependencyOwner = mutationOwner(dependencyRows, (mutation) => mutation.ops?.appendEffects?.some((effect: Row) => effect.kind === 'splash_damage'));
  const unrelated = dependencyRows[0].nodes.find((node: Row) => node.id === 'strength_root_break_stance');
  const unrelatedGrant = { id: 'unrelated_generated_consumer_grant', kind: 'effect_mutation', effectMutations: [{ id: 'unrelated_generated_consumer', priority: -1, scope: 'skill', effectSelector: { mode: 'effect_id', effectIds: [splashId], includeGenerated: true }, ops: { amountMultiplier: 1.1 } }] };
  for (const level of unrelated.levels) level.grants.push(structuredClone(unrelatedGrant));
  expectIssue(dependencyRows, `需要生产节点 ${dependencyOwner.node.id} 至少 Lv.1`);

  const guaranteedRows = trees();
  const statusOwner = mutationOwner(guaranteedRows, (mutation) => mutation.ops?.appendEffects?.some((effect: Row) => effect.kind === 'status'));
  const statusId = statusOwner.mutation.ops.appendEffects.find((effect: Row) => effect.kind === 'status').id;
  const guaranteedConsumer = guaranteedRows[0].nodes.find((node: Row) => node.id === 'perception_terminal_judgement_window');
  const guaranteedGrant = { id: 'guaranteed_status_consumer_grant', kind: 'effect_mutation', effectMutations: [{ id: 'guaranteed_status_consumer', priority: 0, scope: 'skill', effectSelector: { mode: 'effect_id', effectIds: [statusId], includeGenerated: true }, ops: { statusIdsRemove: [refs.buffIds[0]] } }] };
  for (const level of guaranteedConsumer.levels) level.grants.push(structuredClone(guaranteedGrant));
  assert.deepEqual(validateSpecializationTrees(guaranteedRows, refs), []);
});

test('editor validates higher-rank mutations and every reachable generated producer snapshot', () => {
  const lateRankKind = trees();
  const lateKindNode = lateRankKind[0].nodes.find((node: Row) => node.id === 'dexterity_tech_chain_steps');
  const lateKindMutation = lateKindNode.levels[1].grants
    .find((grant: Row) => grant.id === 'grant_dexterity_tech_chain_steps').effectMutations[0];
  lateKindMutation.effectSelector = { mode: 'kind', kinds: ['damage', 'status'], includeGenerated: false };
  expectIssue(lateRankKind, '操作 amount 不能用于效果类型 status');

  const generatedOrder = trees();
  const orderOwner = mutationOwner(generatedOrder, (mutation) => mutation.ops?.appendEffects?.some((effect: Row) => effect.kind === 'splash_damage'));
  const effectId = orderOwner.mutation.ops.appendEffects.find((effect: Row) => effect.kind === 'splash_damage').id;
  appendMutationToGrantSnapshots(orderOwner, {
    id: 'generated_rank_order_consumer',
    priority: -1,
    scope: 'skill',
    effectSelector: { mode: 'effect_id', effectIds: [effectId], includeGenerated: true },
    ops: { amountMultiplier: 1.1 }
  });
  assert.deepEqual(validateSpecializationTrees(generatedOrder, refs), []);
  const rankTwoMutations = orderOwner.node.levels[1].grants
    .find((grant: Row) => grant.id === orderOwner.grant.id).effectMutations;
  const rankTwoProducer = rankTwoMutations.find((mutation: Row) => mutation.id === orderOwner.mutation.id);
  rankTwoMutations.find((mutation: Row) => mutation.id === 'generated_rank_order_consumer').priority = (rankTwoProducer.priority ?? 0) + 1;
  expectIssue(generatedOrder, '必须先于消费变异执行');

  const generatedGrant = (mutation: Row): Row => ({ id: 'generated_grant', kind: 'effect_mutation', effectMutations: [mutation] });
  const producerMutation = (priority: number): Row => ({
    id: 'generated_producer',
    priority,
    scope: 'skill',
    effectSelector: { mode: 'kind', kinds: ['damage'], includeGenerated: false },
    ops: { appendEffects: [{ id: 'generated_rank_effect', kind: 'splash_damage', targetCount: 1, amountRatio: 0.2 }] }
  });
  const consumerMutation = (priority: number): Row => ({
    id: 'generated_consumer',
    priority,
    scope: 'skill',
    effectSelector: { mode: 'effect_id', effectIds: ['generated_rank_effect'], includeGenerated: true },
    ops: { amountMultiplier: 1.1 }
  });
  const semanticTree = (producerPriorities: Array<number | null>, consumerPriorities: number[], producerMinRank = 1): Row[] => [{
    id: 'semantic_tree',
    nodes: [
      {
        id: 'producer_node',
        requirements: { all: [], anyOf: [] },
        levels: producerPriorities.map((priority, index) => ({
          level: index + 1,
          grants: priority === null ? [] : [generatedGrant(producerMutation(priority))]
        }))
      },
      {
        id: 'consumer_node',
        requirements: { all: [{ kind: 'node_rank', nodeId: 'producer_node', minRank: producerMinRank }], anyOf: [] },
        levels: consumerPriorities.map((priority, index) => ({ level: index + 1, grants: [generatedGrant(consumerMutation(priority))] }))
      }
    ]
  }];
  const semanticMessages = (rows: Row[]): string[] => {
    const issues: Array<{ message: string }> = [];
    validateSpecializationMutationSemantics(issues, rows, { effectKindsById: {}, equipmentEffectIds: [] });
    return issues.map((issue) => issue.message);
  };

  assert.ok(semanticMessages(semanticTree([null, 1], [0, 0], 1)).some((message) => message.includes('至少 Lv.2')));
  assert.ok(semanticMessages(semanticTree([1, -1], [0, 0], 1)).some((message) => message.includes('必须先于消费变异执行')));
});

test('editor validator enforces complete multi-rank snapshots and per-rank copy', () => {
  const missingCore = trees();
  const missingCoreNode = missingCore[0].nodes.find((node: Row) => node.id === 'strength_root_break_stance');
  const replacement = structuredClone(missingCoreNode.levels[1].grants[0]);
  replacement.id = 'replacement_break_cycle';
  missingCoreNode.levels[1].grants = [replacement];
  expectIssue(missingCore, '必须保留前一级 grant id/kind');

  const changedKind = trees();
  const changedKindNode = changedKind[0].nodes.find((node: Row) => node.id === 'strength_root_break_stance');
  const changedKindGrant = changedKindNode.levels[1].grants.find((grant: Row) => grant.id === 'grant_strength_root_break_cycle');
  Object.assign(changedKindGrant, { kind: 'stat_modifier', scope: 'character', modifiers: [{ attributeKey: 'strength', operation: 'add', value: 1 }] });
  delete changedKindGrant.battleHooks;
  expectIssue(changedKind, 'grant_strength_root_break_cycle/battle_hook');

  const renamedHook = trees();
  const renamedHookNode = renamedHook[0].nodes.find((node: Row) => node.id === 'strength_root_break_stance');
  renamedHookNode.levels[1].grants.find((grant: Row) => grant.id === 'grant_strength_root_break_cycle').battleHooks[0].id = 'renamed_break_hook';
  expectIssue(renamedHook, '必须保留 battleHook 稳定 payload id');

  const droppedHook = trees();
  const droppedHookNode = droppedHook[0].nodes.find((node: Row) => node.id === 'strength_root_break_stance');
  droppedHookNode.levels[1].grants.find((grant: Row) => grant.id === 'grant_strength_root_break_cycle').battleHooks.shift();
  expectIssue(droppedHook, '必须保留 battleHook 稳定 payload id');

  const renamedMutation = trees();
  const renamedMutationNode = renamedMutation[0].nodes.find((node: Row) => node.id === 'dexterity_tech_chain_steps');
  renamedMutationNode.levels[1].grants.find((grant: Row) => grant.id === 'grant_dexterity_tech_chain_steps').effectMutations[0].id = 'renamed_chain_steps';
  expectIssue(renamedMutation, '必须保留 effectMutation 稳定 payload id');

  const renamedAppendEffect = trees();
  const renamedAppendNode = renamedAppendEffect[0].nodes.find((node: Row) => node.id === 'strength_tech_rupture');
  renamedAppendNode.levels[1].grants.find((grant: Row) => grant.id === 'grant_strength_tech_rupture')
    .effectMutations[0].ops.appendEffects[0].id = 'renamed_blood_forge_cost';
  expectIssue(renamedAppendEffect, '必须保留 effectMutation 生成效果稳定 ID');

  const repurposedAppendEffect = trees();
  const repurposedAppendNode = repurposedAppendEffect[0].nodes.find((node: Row) => node.id === 'strength_tech_rupture');
  const repurposedEffect = repurposedAppendNode.levels[1].grants.find((grant: Row) => grant.id === 'grant_strength_tech_rupture')
    .effectMutations[0].ops.appendEffects[0];
  for (const key of Object.keys(repurposedEffect)) delete repurposedEffect[key];
  Object.assign(repurposedEffect, { id: 'strength_blood_forge_hp_cost', kind: 'splash_damage', targetCount: 1, amountRatio: 0.2 });
  expectIssue(repurposedAppendEffect, '必须保留 effectMutation 生成效果 kind');

  const narrowedScope = trees();
  const narrowedScopeNode = narrowedScope[0].nodes.find((node: Row) => node.id === 'dexterity_terminal_reaping_sweep');
  narrowedScopeNode.levels[1].grants.find((grant: Row) => grant.id === 'grant_dexterity_terminal_reaping_sweep').effectMutations[0].scope = 'skill';
  expectIssue(narrowedScope, '必须保留或扩大 effectMutation scope');

  const changedSelectorMode = trees();
  const changedSelectorNode = changedSelectorMode[0].nodes.find((node: Row) => node.id === 'dexterity_tech_chain_steps');
  changedSelectorNode.levels[1].grants.find((grant: Row) => grant.id === 'grant_dexterity_tech_chain_steps').effectMutations[0].effectSelector = { mode: 'all' };
  expectIssue(changedSelectorMode, '必须保留 effectMutation selector mode');

  const narrowedSelectorValues = trees();
  const quickFormulaNode = narrowedSelectorValues[0].nodes.find((node: Row) => node.id === 'confluence_dexterity_intelligence_quick_formula');
  quickFormulaNode.levels[1].grants.find((grant: Row) => grant.id === 'grant_rework_confluence_dexterity_intelligence_quick_formula')
    .effectMutations[0].skillCategories = ['arcane'];
  expectIssue(narrowedSelectorValues, 'tactical');

  const addedNarrowingFilter = trees();
  const addedFilterNode = addedNarrowingFilter[0].nodes.find((node: Row) => node.id === 'dexterity_tech_chain_steps');
  addedFilterNode.levels[1].grants.find((grant: Row) => grant.id === 'grant_dexterity_tech_chain_steps')
    .effectMutations[0].skillCategories = ['arcane'];
  expectIssue(addedNarrowingFilter, '前一级未限定，本级不能新增过滤条件');

  const changedHookTrigger = trees();
  const changedTriggerNode = changedHookTrigger[0].nodes.find((node: Row) => node.id === 'strength_root_break_stance');
  changedTriggerNode.levels[1].grants.find((grant: Row) => grant.id === 'grant_strength_root_break_cycle')
    .battleHooks[0].trigger = 'onAfterDealCrit';
  expectIssue(changedHookTrigger, '必须保留 battleHook trigger');

  const addedNarrowingCondition = trees();
  const addedConditionNode = addedNarrowingCondition[0].nodes.find((node: Row) => node.id === 'strength_root_break_stance');
  addedConditionNode.levels[1].grants.find((grant: Row) => grant.id === 'grant_strength_root_break_cycle')
    .battleHooks.find((hook: Row) => hook.id === 'strength_break_momentum_cycle').conditions.push({ kind: 'self_hp_ratio_lte', value: 0.01 });
  expectIssue(addedNarrowingCondition, 'conditions 不能退化');

  const narrowedOuterLimit = trees();
  const narrowedLimitNode = narrowedOuterLimit[0].nodes.find((node: Row) => node.id === 'confluence_strength_dexterity_impact_dash');
  narrowedLimitNode.levels[2].grants.find((grant: Row) => grant.kind === 'battle_hook')
    .battleHooks.find((hook: Row) => hook.id === 'impact_dash_spear_followup').limits = { perBattle: 1 };
  expectIssue(narrowedOuterLimit, 'battleHook limits 不能退化');

  const addedNarrowingEventFilter = trees();
  const narrowedEventNode = addedNarrowingEventFilter[0].nodes.find((node: Row) => node.id === 'strength_root_break_stance');
  narrowedEventNode.levels[1].grants.find((grant: Row) => grant.id === 'grant_strength_root_break_cycle')
    .battleHooks.find((hook: Row) => hook.id === 'strength_break_momentum_cycle').eventFilter = { skillIds: ['basic_attack'] };
  expectIssue(addedNarrowingEventFilter, 'battleHook eventFilter 不能退化');

  const reducedCounterCapacity = trees();
  const reducedCounterNode = reducedCounterCapacity[0].nodes.find((node: Row) => node.id === 'strength_root_break_stance');
  reducedCounterNode.levels[1].grants.find((grant: Row) => grant.id === 'grant_strength_root_break_cycle')
    .battleHooks.find((hook: Row) => hook.id === 'strength_break_momentum_cycle').effects[0].maxStacks = 1;
  expectIssue(reducedCounterCapacity, 'maxStacks 1 低于同机制需求 2');

  const droppedHookEffectKind = trees();
  const stolenBeatNode = droppedHookEffectKind[0].nodes.find((node: Row) => node.id === 'dexterity_terminal_stolen_beat');
  const stolenBeatHook = stolenBeatNode.levels[1].grants.find((grant: Row) => grant.id === 'grant_rework_dexterity_terminal_stolen_beat').battleHooks[0];
  stolenBeatHook.effects = stolenBeatHook.effects.filter((effect: Row) => effect.kind !== 'extra_turn');
  expectIssue(droppedHookEffectKind, '必须保留 battleHook effect kind');

  const reducedExtraTurnChance = trees();
  const reducedChanceNode = reducedExtraTurnChance[0].nodes.find((node: Row) => node.id === 'dexterity_terminal_stolen_beat');
  reducedChanceNode.levels[1].grants.find((grant: Row) => grant.id === 'grant_rework_dexterity_terminal_stolen_beat')
    .battleHooks[0].effects.find((effect: Row) => effect.kind === 'extra_turn').procChanceBase = 0.1;
  expectIssue(reducedExtraTurnChance, 'procChanceBase 不能降低');

  const degradedSecondaryRatio = trees();
  const reapingNode = degradedSecondaryRatio[0].nodes.find((node: Row) => node.id === 'dexterity_terminal_reaping_sweep');
  reapingNode.levels[1].grants.find((grant: Row) => grant.id === 'grant_dexterity_terminal_reaping_sweep')
    .effectMutations[0].ops.appendEffects[0].amountRatio = 0.01;
  expectIssue(degradedSecondaryRatio, '不能降低 secondary_damage amountRatio');

  const changedSecondaryPolicy = trees();
  const changedPolicyNode = changedSecondaryPolicy[0].nodes.find((node: Row) => node.id === 'dexterity_terminal_reaping_sweep');
  changedPolicyNode.levels[1].grants.find((grant: Row) => grant.id === 'grant_dexterity_terminal_reaping_sweep')
    .effectMutations[0].ops.appendEffects[0].targetPolicy = 'all_rows';
  expectIssue(changedSecondaryPolicy, '必须保留 secondary_damage 的索敌策略');

  const changedCounterIdentity = trees();
  const changedCounterNode = changedCounterIdentity[0].nodes.find((node: Row) => node.id === 'strength_root_break_stance');
  const changedCounterHooks = changedCounterNode.levels[1].grants.find((grant: Row) => grant.id === 'grant_strength_root_break_cycle').battleHooks;
  const changedCounterCycle = changedCounterHooks.find((hook: Row) => hook.id === 'strength_break_momentum_cycle');
  const unchangedCounterCondition = changedCounterHooks.find((hook: Row) => hook.id === 'strength_break_momentum_release')
    .conditions.find((condition: Row) => condition.kind === 'counter_gte');
  changedCounterCycle.effects[0].counterId = 'unrelated_break_counter';
  changedCounterCycle.conditions.push(structuredClone(unchangedCounterCondition));
  expectIssue(changedCounterIdentity, '必须保留 battleHook effect 资源绑定');

  const changedAppendedStatusIdentity = trees();
  const changedStatusOwner = mutationOwner(changedAppendedStatusIdentity, (mutation) => mutation.ops?.appendEffects?.some((effect: Row) => effect.kind === 'status'));
  changedStatusOwner.node.levels[1].grants.find((grant: Row) => grant.id === changedStatusOwner.grant.id)
    .effectMutations.find((mutation: Row) => mutation.id === changedStatusOwner.mutation.id)
    .ops.appendEffects.find((effect: Row) => effect.kind === 'status').statusPayload.statusIds = [refs.buffIds[0]];
  expectIssue(changedAppendedStatusIdentity, '必须保留 status');

  assert.ok(specializationGrantSnapshotRetentionProblems(
    { id: 'topology_swap', kind: 'equipment_slot_capacity', equipType: 'battle', deltas: { head: -1, accessory: 2 } },
    { id: 'topology_swap', kind: 'equipment_slot_capacity', equipType: 'battle', deltas: { accessory: 2 } }
  ).some((problem) => problem.includes('必须保留装备槽 family')));

  const legalAddition = trees();
  const legalAdditionNode = legalAddition[0].nodes.find((node: Row) => node.id === 'strength_root_break_stance');
  const addedHook = structuredClone(legalAdditionNode.levels[0].grants.find((grant: Row) => grant.id === 'grant_strength_root_break_cycle').battleHooks[0]);
  addedHook.id = 'strength_break_rank_bonus';
  for (const level of legalAdditionNode.levels.slice(1)) level.grants.find((grant: Row) => grant.id === 'grant_strength_root_break_cycle').battleHooks.push(structuredClone(addedHook));
  assert.deepEqual(validateSpecializationTrees(legalAddition, refs), []);
  assert.deepEqual(specializationGrantSnapshotRetentionProblems(
    { id: 'topology_swap', kind: 'equipment_slot_capacity', equipType: 'battle', deltas: { head: -1, accessory: 2 } },
    { id: 'topology_swap', kind: 'equipment_slot_capacity', equipType: 'battle', deltas: { head: -2, accessory: 2, right_hand: 1 } }
  ), []);
  const ruptureNode = legalAddition[0].nodes.find((node: Row) => node.id === 'strength_tech_rupture');
  assert.deepEqual(specializationGrantSnapshotRetentionProblems(
    ruptureNode.levels[0].grants.find((grant: Row) => grant.id === 'grant_strength_tech_rupture'),
    ruptureNode.levels[1].grants.find((grant: Row) => grant.id === 'grant_strength_tech_rupture')
  ), []);
  assert.deepEqual(specializationGrantSnapshotRetentionProblems(
    { id: 'mutation', kind: 'effect_mutation', effectMutations: [{ id: 'payload', scope: 'skill', effectSelector: { mode: 'kind', kinds: ['damage'] }, ops: {} }] },
    { id: 'mutation', kind: 'effect_mutation', effectMutations: [{ id: 'payload', scope: 'both', effectSelector: { mode: 'kind', kinds: ['damage', 'heal'] }, ops: {} }] }
  ), []);

  const statsOnly = trees();
  const statsOnlyNode = statsOnly[0].nodes.find((node: Row) => node.id === 'strength_root_break_stance');
  statsOnlyNode.levels[1].grants = [
    ...structuredClone(statsOnlyNode.levels[0].grants.filter((grant: Row) => grant.kind !== 'stat_modifier')),
    ...statsOnlyNode.levels[1].grants.filter((grant: Row) => grant.kind === 'stat_modifier')
  ];
  expectIssue(statsOnly, '必须严格增强至少一项非属性核心机制');

  const reorderedOnly = trees();
  const reorderedNode = reorderedOnly[0].nodes.find((node: Row) => node.id === 'strength_root_break_stance');
  const reorderedCore = structuredClone(reorderedNode.levels[0].grants.filter((grant: Row) => grant.kind !== 'stat_modifier'));
  reorderedCore.find((grant: Row) => grant.kind === 'battle_hook').battleHooks.reverse();
  reorderedNode.levels[1].grants = [...reorderedCore, ...reorderedNode.levels[1].grants.filter((grant: Row) => grant.kind === 'stat_modifier')];
  expectIssue(reorderedOnly, '必须严格增强至少一项非属性核心机制');

  const reorderedAppendOnly = trees();
  const reorderedAppendNode = reorderedAppendOnly[0].nodes.find((node: Row) => node.id === 'strength_tech_rupture');
  const reorderedAppendCore = structuredClone(reorderedAppendNode.levels[0].grants.filter((grant: Row) => grant.kind !== 'stat_modifier'));
  reorderedAppendCore[0].effectMutations[0].ops.appendEffects.reverse();
  reorderedAppendNode.levels[1].grants = [...reorderedAppendCore, ...reorderedAppendNode.levels[1].grants.filter((grant: Row) => grant.kind === 'stat_modifier')];
  expectIssue(reorderedAppendOnly, '必须严格增强至少一项非属性核心机制');

  const reorderedConditionsOnly = trees();
  const reorderedConditionsNode = reorderedConditionsOnly[0].nodes.find((node: Row) => node.id === 'strength_root_break_stance');
  const conditionCore = structuredClone(reorderedConditionsNode.levels[0].grants.filter((grant: Row) => grant.kind !== 'stat_modifier'));
  conditionCore.find((grant: Row) => grant.kind === 'battle_hook').battleHooks.find((hook: Row) => hook.conditions.length > 1).conditions.reverse();
  reorderedConditionsNode.levels[1].grants = [...conditionCore, ...reorderedConditionsNode.levels[1].grants.filter((grant: Row) => grant.kind === 'stat_modifier')];
  expectIssue(reorderedConditionsOnly, '必须严格增强至少一项非属性核心机制');

  const reorderedSelectorValuesOnly = trees();
  const reorderedSelectorNode = reorderedSelectorValuesOnly[0].nodes.find((node: Row) => node.id === 'confluence_dexterity_intelligence_quick_formula');
  const selectorCore = structuredClone(reorderedSelectorNode.levels[0].grants.filter((grant: Row) => grant.kind !== 'stat_modifier'));
  selectorCore[0].effectMutations[0].skillCategories.reverse();
  reorderedSelectorNode.levels[1].grants = [...selectorCore, ...reorderedSelectorNode.levels[1].grants.filter((grant: Row) => grant.kind === 'stat_modifier')];
  expectIssue(reorderedSelectorValuesOnly, '必须严格增强至少一项非属性核心机制');

  const missingCopy = trees();
  missingCopy[0].i18n['zh-CN'].nodes.strength_root_break_stance.levelEffectSummary.pop();
  expectIssue(missingCopy, 'levelEffectSummary 必须逐级覆盖');

  const topologyRank = trees();
  const topologyNode = topologyRank[0].nodes.find((node: Row) => node.id === 'strength_terminal_three_armed');
  topologyNode.levels.push({ level: 2, cost: 1, grants: structuredClone(topologyNode.levels[0].grants) });
  topologyRank[0].i18n['zh-CN'].nodes[topologyNode.id].levelEffectSummary.push(['不应存在的第二级。']);
  topologyRank[0].i18n['en-US'].nodes[topologyNode.id].levelEffectSummary.push(['An invalid second rank.']);
  expectIssue(topologyRank, '装备槽拓扑只能在节点最终等级引入');
});

test('generated non-materialized effects and skill scalar selectors fail closed', () => {
  const nonMaterialized = trees();
  const strengthOwner = mutationOwner(nonMaterialized, (mutation) => mutation.ops?.appendEffects?.some((effect: Row) => effect.kind === 'consume_hp'));
  const consumeId = strengthOwner.mutation.ops.appendEffects.find((effect: Row) => effect.kind === 'consume_hp').id;
  appendMutationToGrantSnapshots(strengthOwner, { id: 'consume_hp_consumer', priority: -1, scope: 'skill', effectSelector: { mode: 'effect_id', effectIds: [consumeId], includeGenerated: true }, ops: { targetCountDelta: 1 } });
  expectIssue(nonMaterialized, '不会物化为可变异效果');

  const runtimeAmpRows = trees();
  const ampOwner = mutationOwner(runtimeAmpRows, (mutation) => mutation.ops?.appendEffects?.some((effect: Row) => effect.kind === 'runtime_damage_amp'));
  const ampId = ampOwner.mutation.ops.appendEffects.find((effect: Row) => effect.kind === 'runtime_damage_amp').id;
  appendMutationToGrantSnapshots(ampOwner, { id: 'runtime_amp_consumer', priority: -1, scope: 'skill', effectSelector: { mode: 'effect_id', effectIds: [ampId], includeGenerated: true }, ops: { targetCountDelta: 1 } });
  expectIssue(runtimeAmpRows, '不会物化为可变异效果');

  const generatedScalar = trees();
  const splashOwner = mutationOwner(generatedScalar, (mutation) => mutation.ops?.appendEffects?.some((effect: Row) => effect.kind === 'splash_damage'));
  const splashId = splashOwner.mutation.ops.appendEffects.find((effect: Row) => effect.kind === 'splash_damage').id;
  appendMutationToGrantSnapshots(splashOwner, { id: 'generated_scalar_consumer', priority: -1, scope: 'skill', effectSelector: { mode: 'effect_id', effectIds: [splashId], includeGenerated: true }, ops: { cooldownDelta: -1, cooldownFloor: 1, mpCostMultiplier: 3 } });
  expectIssue(generatedScalar, '生成效果');
  expectIssue(generatedScalar, '不能使用技能级标量操作');

  const equipmentScalar = trees();
  mutateFirstMutationSnapshots(equipmentScalar, (mutation) => {
    mutation.effectSelector = { mode: 'effect_id', effectIds: ['equipment_wood_sword_damage'], includeGenerated: false };
    mutation.ops = { mpCostDelta: 1 };
  });
  expectIssue(equipmentScalar, '装备普攻效果 equipment_wood_sword_damage');

  const badPriority = trees();
  mutateFirstMutationSnapshots(badPriority, (mutation) => { mutation.priority = 0.5; });
  expectIssue(badPriority, '.priority: 数值非法');
});

test('editor rejects uppercase and globally conflicting appended stable effect IDs', () => {
  const uppercase = trees();
  appendedEffects(uppercase)[0].id = 'Generated_Damage';
  expectIssue(uppercase, '稳定效果 ID 只能使用小写字母');

  const catalogCollision = trees();
  appendedEffects(catalogCollision)[0].id = 'equipment_wood_sword_damage';
  expectIssue(catalogCollision, '稳定效果 ID 全局重复');

  const appendCollision = trees();
  const appended = appendedEffects(appendCollision);
  appended[2].id = appended[0].id.toUpperCase();
  expectIssue(appendCollision, '稳定效果 ID 全局重复');

  const sameNodeCollision = trees();
  const producerNode = sameNodeCollision[0].nodes.find((node: Row) => node.id === 'dexterity_terminal_reaping_sweep');
  for (const level of producerNode.levels) {
    const grant = level.grants.find((entry: Row) => entry.id === 'grant_dexterity_terminal_reaping_sweep');
    const duplicateProducer = structuredClone(grant.effectMutations[0]);
    duplicateProducer.id = 'second_blade_fan_splash_producer';
    grant.effectMutations.push(duplicateProducer);
  }
  expectIssue(sameNodeCollision, '稳定效果 ID 全局重复');

  const stableAcrossRanks = trees();
  const stableNode = stableAcrossRanks[0].nodes.find((node: Row) => node.id === 'dexterity_terminal_reaping_sweep');
  assert.equal(new Set(stableNode.levels.map((level: Row) => level.grants
    .find((grant: Row) => grant.id === 'grant_dexterity_terminal_reaping_sweep').effectMutations[0].ops.appendEffects[0].id)).size, 1);
  assert.deepEqual(validateSpecializationTrees(stableAcrossRanks, refs), []);
});

test('keyword tokens stay declared by the node and appear in every rank summary', () => {
  const missingToken = trees();
  const entry = missingToken[0].i18n['zh-CN'].nodes.strength_root_break_stance;
  const tokenLineIndex = entry.levelEffectSummary[0].findIndex((line: string) => line.includes('[[keyword:break_momentum]]'));
  entry.levelEffectSummary[0][tokenLineIndex] = entry.levelEffectSummary[0][tokenLineIndex].replace('[[keyword:break_momentum]]', '');
  expectIssue(missingToken, '必须标记关键词 break_momentum');

  const foreignToken = trees();
  foreignToken[0].i18n['zh-CN'].nodes.strength_root_break_stance.effectSummary[0] = `[[keyword:echo_cast]] ${foreignToken[0].i18n['zh-CN'].nodes.strength_root_break_stance.effectSummary[0]}`;
  expectIssue(foreignToken, '关键词 echo_cast 不属于该节点');
});

test('explicit recursive dispatch true remains valid for ancestry-guarded runtime builds', () => {
  const rows = trees();
  makeDispatchExplicit(rows);
  const recursiveKinds = new Set<string>(SPECIALIZATION_RECURSIVE_EFFECT_KINDS);
  const effect = hooks(rows).flatMap((hook) => hook.effects).find((entry) => recursiveKinds.has(entry.kind));
  effect.dispatchHooks = true;
  assert.deepEqual(validateSpecializationTrees(rows, refs), []);
});
