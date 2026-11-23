import type { DataRow } from '../model/catalog';
import {
  SPECIALIZATION_CONDITION_KINDS,
  SPECIALIZATION_HOOK_EFFECT_KINDS,
  SPECIALIZATION_MUTATION_APPEND_EFFECT_KINDS,
  specializationConditionKindsForTrigger,
  specializationEventFilterKeysForTrigger,
  specializationHookEffectKindsForTrigger,
  specializationTriggerHasEventSubject,
  specializationTriggerHasSkillContext
} from '../model/specialization.ts';
import type { DslFieldSpec, DslObjectSchema, DslObjectVariant } from './DslObjectEditor.vue';

export type SpecializationDslRefs = {
  attributeIds: string[];
  skillIds: string[];
  skillCategories: string[];
  weaponClasses: string[];
  buffIds: string[];
  buffKinds: string[];
  effectIds: string[];
  skillEffectKinds: string[];
};

const text = (defaultValue = '', required = false): DslFieldSpec => ({ type: 'text', defaultValue, required });
const number = (defaultValue: number, options: Pick<DslFieldSpec, 'required' | 'min' | 'max' | 'step'> = {}): DslFieldSpec => ({ type: 'number', defaultValue, ...options });
const boolean = (defaultValue = false, required = false): DslFieldSpec => ({ type: 'boolean', defaultValue, required });
const select = (defaultValue: string, options: readonly string[], required = false): DslFieldSpec => ({ type: 'select', defaultValue, options, required });
const stringList = (defaultValue: string[] = [], options?: readonly string[]): DslFieldSpec => ({ type: 'string_list', defaultValue, options });
const object = (defaultValue: DataRow, schema: DslObjectSchema, required = false): DslFieldSpec => ({ type: 'object', defaultValue, schema, required });
const objectList = (defaultValue: DataRow[], schema?: DslObjectSchema, variants?: Record<string, DslObjectVariant>): DslFieldSpec => ({ type: 'object_list', defaultValue, schema, variants });
const variant = (defaultValue: DataRow, schema: DslObjectSchema): DslObjectVariant => ({ defaultValue, schema });
const kindField = (value: string, options: readonly string[]): DslFieldSpec => select(value, options, true);

function formulaWithRefs(refs: SpecializationDslRefs, coefficient = 1): { value: DataRow; schema: DslObjectSchema } {
  const attribute = refs.attributeIds.includes('strength') ? 'strength' : refs.attributeIds[0] ?? 'strength';
  const termSchema: DslObjectSchema = {
    attributeKey: select(attribute, refs.attributeIds, true),
    coefficient: number(coefficient, { required: true })
  };
  return {
    value: { baseAdd: 0, terms: [{ attributeKey: attribute, coefficient }] },
    schema: {
      baseAdd: number(0, { required: true }),
      terms: objectList([{ attributeKey: attribute, coefficient }], termSchema)
    }
  };
}

export function createEventFilterSchema(refs: SpecializationDslRefs, trigger?: string): DslObjectSchema {
  const schema: DslObjectSchema = {
    skillIds: stringList([], refs.skillIds),
    excludeSkillIds: stringList([], refs.skillIds),
    skillCategories: stringList([], refs.skillCategories),
    damageTypes: stringList([], ['physical', 'magical', 'true']),
    turnKinds: stringList([], ['ordered', 'extra']),
    includeBasicAttack: boolean(false),
    includePreparation: boolean(false),
    includeSuppressedTurns: boolean(false),
    excludeSelf: boolean(false),
    unitTypes: stringList([], ['player', 'kitty', 'enemy', 'summon'])
  };
  if (!trigger) return schema;
  const reachable = new Set(specializationEventFilterKeysForTrigger(trigger));
  return Object.fromEntries(Object.entries(schema).filter(([key]) => reachable.has(key)));
}

export const SPECIALIZATION_LIMITS_SCHEMA: DslObjectSchema = {
  perTurn: number(1, { min: 1, step: 1 }),
  perWave: number(1, { min: 1, step: 1 }),
  perBattle: number(1, { min: 1, step: 1 })
};

export function createConditionVariants(trigger?: string, refs?: SpecializationDslRefs): Record<string, DslObjectVariant> {
  const variants: Record<string, DslObjectVariant> = {};
  const allowedKinds = trigger ? specializationConditionKindsForTrigger(trigger) : [...SPECIALIZATION_CONDITION_KINDS];
  const noPayload = ['adjacent_ally_exists', 'context_mp_insufficient', 'context_skill_non_basic', 'incoming_damage_would_be_lethal', 'source_is_enemy', 'target_is_lowest_hp_ally'];
  for (const kind of noPayload) variants[kind] = variant({ kind }, { kind: kindField(kind, allowedKinds) });
  variants.context_damage_type = variant({ kind: 'context_damage_type', value: 'physical' }, { kind: kindField('context_damage_type', allowedKinds), value: select('physical', ['physical', 'magical', 'true'], true) });
  variants.context_hit_count_gte = variant({ kind: 'context_hit_count_gte', value: 2 }, { kind: kindField('context_hit_count_gte', allowedKinds), value: number(2, { required: true, min: 1, step: 1 }) });
  variants.counter_gte = variant({ kind: 'counter_gte', counterId: 'counter_id', value: 1 }, { kind: kindField('counter_gte', allowedKinds), counterId: text('counter_id', true), value: number(1, { required: true, min: 1, step: 1 }) });
  variants.self_hp_ratio_lte = variant({ kind: 'self_hp_ratio_lte', value: 0.5 }, { kind: kindField('self_hp_ratio_lte', allowedKinds), value: number(0.5, { required: true, min: 0, max: 1 }) });
  variants.self_mp_ratio_gte = variant({ kind: 'self_mp_ratio_gte', value: 0.5 }, { kind: kindField('self_mp_ratio_gte', allowedKinds), value: number(0.5, { required: true, min: 0, max: 1 }) });
  variants.self_weapon_class = variant({ kind: 'self_weapon_class', weaponClasses: refs?.weaponClasses.slice(0, 1) ?? [] }, { kind: kindField('self_weapon_class', allowedKinds), weaponClasses: stringList(refs?.weaponClasses.slice(0, 1) ?? [], refs?.weaponClasses) });
  variants.target_has_buff = variant({ kind: 'target_has_buff', buffId: refs?.buffIds[0] ?? '' }, { kind: kindField('target_has_buff', allowedKinds), buffId: select(refs?.buffIds[0] ?? '', refs?.buffIds ?? [], true) });
  variants.target_has_any_buff = variant({ kind: 'target_has_any_buff', buffIds: refs?.buffIds.slice(0, 1) ?? [] }, { kind: kindField('target_has_any_buff', allowedKinds), buffIds: stringList(refs?.buffIds.slice(0, 1) ?? [], refs?.buffIds) });
  variants.trigger_limit = variant({ kind: 'trigger_limit', perTurn: 1 }, { kind: kindField('trigger_limit', allowedKinds), ...SPECIALIZATION_LIMITS_SCHEMA });
  return Object.fromEntries(Object.entries(variants).filter(([kind]) => allowedKinds.includes(kind)));
}

function payoffVariants(refs: SpecializationDslRefs): Record<string, DslObjectVariant> {
  const kinds = ['empower_next_hit', 'restore_mp', 'guard_lowest_hp_ally'];
  const formulaShape = formulaWithRefs(refs, 0.01);
  const valueFormula = () => object(formulaShape.value, formulaShape.schema);
  return {
    empower_next_hit: variant({ kind: 'empower_next_hit', damageAmp: 0.1, consumeStacks: 1, valueFormula: formulaShape.value }, { kind: kindField('empower_next_hit', kinds), damageAmp: number(0.1, { required: true, min: 0 }), consumeStacks: number(1, { required: true, min: 1, step: 1 }), valueFormula: valueFormula() }),
    restore_mp: variant({ kind: 'restore_mp', mpRatio: 0.1, consumeStacks: 1, valueFormula: formulaShape.value }, { kind: kindField('restore_mp', kinds), mpRatio: number(0.1, { required: true, min: 0, max: 0.5 }), consumeStacks: number(1, { required: true, min: 1, step: 1 }), valueFormula: valueFormula() }),
    guard_lowest_hp_ally: variant({ kind: 'guard_lowest_hp_ally', damageReduction: 0.1, durationTurns: 1, consumeStacks: 1, valueFormula: formulaShape.value }, { kind: kindField('guard_lowest_hp_ally', kinds), damageReduction: number(0.1, { required: true, min: 0, max: 1 }), durationTurns: number(1, { required: true, min: 1, step: 1 }), consumeStacks: number(1, { required: true, min: 1, step: 1 }), valueFormula: valueFormula() })
  };
}

export function createHookEffectVariants(refs: SpecializationDslRefs, trigger?: string): Record<string, DslObjectVariant> {
  const variants: Record<string, DslObjectVariant> = {};
  const formulaShape = formulaWithRefs(refs);
  const allowedKinds = trigger ? specializationHookEffectKindsForTrigger(trigger) : [...SPECIALIZATION_HOOK_EFFECT_KINDS];
  const targetModes = ['self', 'source', 'target', 'enemy', 'marked_enemy', 'targeting', 'lowest_hp_ally'].filter((mode) => !trigger || specializationTriggerHasEventSubject(trigger) || (mode !== 'source' && mode !== 'targeting'));
  const selectionModes = ['highest_attack', 'highest_threat', 'lowest_hp', 'random'];
  const cooldownSelectionModes = ['max_remaining', 'all', ...(!trigger || specializationTriggerHasSkillContext(trigger) ? ['context_skill'] : [])];
  const recursiveDispatch: DslObjectSchema = { dispatchHooks: boolean(false, true) };
  const valueFormula = () => object(formulaShape.value, formulaShape.schema);
  const withKind = (kind: string, schema: DslObjectSchema): DslObjectSchema => ({ kind: kindField(kind, allowedKinds), ...schema });
  variants.adjust_skill_cooldown = variant({ kind: 'adjust_skill_cooldown', amount: 1, selectionMode: 'max_remaining' }, withKind('adjust_skill_cooldown', { targetMode: select('self', targetModes), amount: number(1, { required: true, min: 1, step: 1 }), selectionMode: select('max_remaining', cooldownSelectionModes), maxSkills: number(1, { min: 1, step: 1 }), skillIds: stringList([], refs.skillIds) }));
  variants.consume_counter_for_damage_amp = variant({ kind: 'consume_counter_for_damage_amp', counterId: 'counter_id', consume: 1, value: 0.1, valueFormula: formulaShape.value }, withKind('consume_counter_for_damage_amp', { counterId: text('counter_id', true), consume: number(1, { required: true, min: 1, step: 1 }), value: number(0.1, { required: true, min: 0 }), percentDefensePenetration: number(0.1, { min: 0, max: 1 }), valueFormula: valueFormula() }));
  variants.cycle_counter = variant({ kind: 'cycle_counter', counterId: 'counter_id', gain: 1, maxStacks: 2 }, withKind('cycle_counter', { counterId: text('counter_id', true), gain: number(1, { required: true, min: 1, step: 1 }), maxStacks: number(2, { required: true, min: 1, step: 1 }), atMax: { type: 'object', defaultValue: { kind: 'empower_next_hit', damageAmp: 0.1, consumeStacks: 2, valueFormula: formulaShape.value }, variants: payoffVariants(refs) } }));
  variants.damage = variant({ kind: 'damage', targetMode: 'enemy', damageType: 'physical', amountFormula: formulaShape.value, dispatchHooks: false }, withKind('damage', { targetMode: select('enemy', targetModes, true), damageType: select('physical', ['physical', 'magical', 'true'], true), amountFormula: object(formulaShape.value, formulaShape.schema, true), ...recursiveDispatch }));
  variants.extra_turn = variant({ kind: 'extra_turn', targetMode: 'self', turnMode: 'full', procChanceBase: 0.2, procChanceMax: 0.4, abortHookOnFail: true, dispatchHooks: false }, withKind('extra_turn', {
    targetMode: select('self', ['self'], true),
    turnMode: select('full', ['full'], true),
    procChanceBase: number(0.2, { min: 0, max: 1 }),
    procChanceMax: number(0.4, { min: 0, max: 1 }),
    abortHookOnFail: boolean(false),
    ...recursiveDispatch
  }));
  variants.flat_defense_penetration = variant({ kind: 'flat_defense_penetration', valueFormula: formulaShape.value }, withKind('flat_defense_penetration', { valueFormula: object(formulaShape.value, formulaShape.schema, true) }));
  variants.heal = variant({ kind: 'heal', targetMode: 'self', maxHpRatio: 0.1, valueFormula: formulaShape.value }, withKind('heal', { targetMode: select('self', trigger === 'onBeforeTakeDamage' ? ['self'] : targetModes, true), maxHpRatio: number(0.1, { min: 0, max: 1 }), missingHpRatio: number(0.1, { min: 0, max: 1 }), valueFormula: valueFormula() }));
  variants.buff_cycle = variant({ kind: 'buff_cycle', buffId: refs.buffIds[0] ?? '', firstHit: 'apply_buff', buffedHit: { kind: 'runtime_damage_amp', value: 0.1, consumeBuff: true, valueFormula: formulaShape.value } }, withKind('buff_cycle', { buffId: select(refs.buffIds[0] ?? '', refs.buffIds, true), firstHit: select('apply_buff', ['apply_buff'], true), buffedHit: object({ kind: 'runtime_damage_amp', value: 0.1, consumeBuff: true, valueFormula: formulaShape.value }, { kind: select('runtime_damage_amp', ['runtime_damage_amp'], true), value: number(0.1, { required: true, min: 0 }), consumeBuff: boolean(true, true), valueFormula: valueFormula() }, true) }));
  variants.mirror_cast_skill = variant({ kind: 'mirror_cast_skill', amountRatio: 0.45, applyCooldown: false, allowMirroredCastTrigger: false, dispatchHooks: false }, withKind('mirror_cast_skill', { amountRatio: number(0.45, { required: true, min: 0, max: 0.55 }), applyCooldown: boolean(false, true), allowMirroredCastTrigger: boolean(false, true), skillIds: stringList([], refs.skillIds), ...recursiveDispatch }));
  variants.prevent_lethal = variant({ kind: 'prevent_lethal', remainingHpRatio: 0.1, valueFormula: formulaShape.value }, withKind('prevent_lethal', { remainingHpRatio: number(0.1, { required: true, min: 0, max: 1 }), valueFormula: valueFormula() }));
  variants.redirect_damage = variant({ kind: 'redirect_damage', targetMode: 'self', ratio: 0.1, valueFormula: formulaShape.value }, withKind('redirect_damage', { targetMode: select('self', ['self'], true), ratio: number(0.1, { required: true, min: 0, max: 1 }), valueFormula: valueFormula() }));
  variants.remove_buff_by_kind = variant({ kind: 'remove_buff_by_kind', buffKind: refs.buffKinds[0] ?? '', selectionMode: 'max_remaining', maxBuffs: 1 }, withKind('remove_buff_by_kind', { buffKind: select(refs.buffKinds[0] ?? '', refs.buffKinds, true), selectionMode: select('max_remaining', ['max_remaining'], true), maxBuffs: number(1, { required: true, min: 1, step: 1 }) }));
  variants.repeat_context_effect = variant({ kind: 'repeat_context_effect', amountRatio: 0.4, targetMode: 'enemy', dispatchHooks: false }, withKind('repeat_context_effect', { amountRatio: number(0.4, { required: true, min: 0, max: 0.45 }), targetMode: select('enemy', targetModes, true), ...recursiveDispatch }));
  variants.resource_debt_cast = variant({ kind: 'resource_debt_cast', hpCostRatio: 0.1, cooldownDelta: 1, minimumHp: 1, valueFormula: formulaShape.value }, withKind('resource_debt_cast', { hpCostRatio: number(0.1, { required: true, min: 0, max: 1 }), cooldownDelta: number(1, { required: true, step: 1 }), minimumHp: number(1, { required: true, min: 1, step: 1 }), valueFormula: valueFormula() }));
  variants.resource_shield = variant({ kind: 'resource_shield', resource: 'mp', absorbRatio: 0.2, resourcePerDamage: 1, valueFormula: formulaShape.value }, withKind('resource_shield', { resource: select('mp', ['mp'], true), absorbRatio: number(0.2, { required: true, min: 0, max: 1 }), resourcePerDamage: number(1, { required: true, min: 0 }), valueFormula: valueFormula() }));
  variants.restore_mp = variant({ kind: 'restore_mp', maxMpRatio: 0.1, valueFormula: formulaShape.value }, withKind('restore_mp', { maxMpRatio: number(0.1, { required: true, min: 0, max: 1 }), valueFormula: valueFormula() }));
  variants.retaliate = variant({ kind: 'retaliate', targetMode: 'source', strengthRatio: 0.2, staminaRatio: 0.2, dispatchHooks: false }, withKind('retaliate', { targetMode: select('source', ['source'], true), strengthRatio: number(0.2, { required: true, min: 0 }), staminaRatio: number(0.2, { required: true, min: 0 }), ...recursiveDispatch }));
  variants.runtime_damage_amp = variant({ kind: 'runtime_damage_amp', value: 0.1, valueFormula: formulaShape.value }, withKind('runtime_damage_amp', { value: number(0.1, { required: true, min: 0 }), valueFormula: valueFormula() }));
  variants.shared_damage_reduction = variant({ kind: 'shared_damage_reduction', value: 0.1, durationTurns: 1, valueFormula: formulaShape.value }, withKind('shared_damage_reduction', { value: number(0.1, { required: true, min: 0, max: 1 }), durationTurns: number(1, { required: true, min: 1, step: 1 }), valueFormula: valueFormula() }));
  variants.shift_position = variant({ kind: 'shift_position', policy: 'toward_lowest_hp_ally', maxSteps: 1 }, withKind('shift_position', { policy: select('toward_lowest_hp_ally', ['toward_lowest_hp_ally'], true), maxSteps: number(1, { required: true, min: 1, step: 1 }) }));
  variants.add_buff = variant({ kind: 'add_buff', buffId: refs.buffIds[0] ?? '', targetMode: 'target', dispatchHooks: false }, withKind('add_buff', { buffId: select(refs.buffIds[0] ?? '', refs.buffIds, true), durationTurns: number(1, { min: 1, step: 1 }), targetMode: select('target', targetModes), selectionMode: select('highest_attack', selectionModes), ...recursiveDispatch }));
  variants.remove_buff = variant({ kind: 'remove_buff', buffId: refs.buffIds[0] ?? '', targetMode: 'target', dispatchHooks: false }, withKind('remove_buff', { buffId: select(refs.buffIds[0] ?? '', refs.buffIds, true), targetMode: select('target', targetModes), ...recursiveDispatch }));
  variants.cast_skill = variant({ kind: 'cast_skill', skillId: refs.skillIds[0] ?? '', dispatchHooks: false }, withKind('cast_skill', { skillId: select(refs.skillIds[0] ?? '', refs.skillIds, true), amountRatio: number(1, { min: 0 }), ...recursiveDispatch }));
  return Object.fromEntries(Object.entries(variants).filter(([kind]) => allowedKinds.includes(kind)));
}

function appendEffectVariants(refs: SpecializationDslRefs, lockStableIds = false): Record<string, DslObjectVariant> {
  const withKind = (kind: string, schema: DslObjectSchema): DslObjectSchema => ({ kind: kindField(kind, SPECIALIZATION_MUTATION_APPEND_EFFECT_KINDS), ...schema });
  const stableId = (kind: string): DslFieldSpec => ({ ...text(`${kind}_effect`, true), readonly: lockStableIds });
  return {
    consume_hp: variant({ id: 'consume_hp_effect', kind: 'consume_hp', maxHpRatio: 0.05, minimumHp: 1 }, withKind('consume_hp', { id: stableId('consume_hp'), maxHpRatio: number(0.05, { required: true, min: 0, max: 1 }), minimumHp: number(1, { required: true, min: 1, step: 1 }) })),
    runtime_damage_amp: variant({ id: 'runtime_damage_amp_effect', kind: 'runtime_damage_amp', value: 0.1 }, withKind('runtime_damage_amp', { id: stableId('runtime_damage_amp'), value: number(0.1, { required: true, min: 0 }) })),
    splash_damage: variant({ id: 'splash_damage_effect', kind: 'splash_damage', targetCount: 2, amountRatio: 0.5 }, withKind('splash_damage', { id: stableId('splash_damage'), targetCount: number(2, { required: true, min: 1, step: 1 }), amountRatio: number(0.5, { required: true, min: 0 }) })),
    secondary_damage: variant({ id: 'secondary_damage_effect', kind: 'secondary_damage', targetPolicy: 'same_row', amountRatio: 0.3 }, withKind('secondary_damage', { id: stableId('secondary_damage'), targetPolicy: select('same_row', ['same_row', 'fixed_row', 'all_rows'], true), targetPosition: select('rearguard', ['vanguard', 'midguard', 'rearguard']), amountRatio: number(0.3, { required: true, min: 0, max: 0.5 }) })),
    status: variant({ id: 'status_effect', kind: 'status', statusPayload: { statusIds: refs.buffIds.slice(0, 1), durationBaseTurns: 1, durationPerSkillLevelTurns: 0 } }, withKind('status', { id: stableId('status'), statusPayload: object({ statusIds: refs.buffIds.slice(0, 1), durationBaseTurns: 1, durationPerSkillLevelTurns: 0 }, { statusIds: stringList(refs.buffIds.slice(0, 1), refs.buffIds), durationBaseTurns: number(1, { required: true, min: 1, step: 1 }), durationPerSkillLevelTurns: number(0, { required: true, min: 0, step: 1 }) }, true) }))
  };
}

export function createMutationOpsSchema(refs: SpecializationDslRefs, options: { lockStableIds?: boolean } = {}): DslObjectSchema {
  const formulaShape = formulaWithRefs(refs);
  return {
    targetCountDelta: number(1, { step: 1 }),
    amountBaseAddDelta: number(1),
    cooldownDelta: number(-1, { step: 1 }),
    cooldownFloor: number(1, { min: 1, step: 1 }),
    mpCostDelta: number(1, { step: 1 }),
    mpCostMultiplier: number(3, { min: 3 }),
    hitCountDelta: number(1, { step: 1 }),
    amountMultiplier: number(1.05, { min: 0 }),
    amountTermsAppend: objectList(formulaShape.value.terms as DataRow[], formulaShape.schema.terms.schema),
    statusIdsAppend: stringList([], refs.buffIds),
    statusIdsRemove: stringList([], refs.buffIds),
    appendEffects: objectList([], undefined, appendEffectVariants(refs, options.lockStableIds === true)),
    attackStatPolicy: select('higher', ['higher']),
    damageTypePolicy: select('preserve', ['preserve', 'physical', 'magical', 'higher_final_damage']),
    targetingPatch: object({ targetScope: 'enemy' }, {
      targetScope: select('enemy', ['self', 'ally', 'enemy', 'all']),
      targetPositions: stringList([], ['vanguard', 'midguard', 'rearguard']),
      secondarySeek: select('none', ['front_to_rear', 'rear_to_front', 'random_alive', 'none']),
      selectRule: select('first', ['hp_max', 'hp_min', 'hp_percent_max', 'hp_percent_min', 'first', 'boss_first', 'attr_max', 'attr_min', 'random', 'self']),
      selectAttrKey: select(refs.attributeIds[0] ?? 'strength', refs.attributeIds),
      fallbackSelfIfEmpty: boolean(false),
      targetCount: { ...number(1, { min: 1, step: 1 }), label: 'targetCount (max)' }
    })
  };
}
