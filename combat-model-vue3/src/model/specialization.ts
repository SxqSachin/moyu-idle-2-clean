import { asArray, asString, clone, isRecord, type DataRow } from './catalog.ts';

export const SPECIALIZATION_SCHOOLS = ['strength', 'dexterity', 'intelligence', 'perception', 'stamina'] as const;
export const SPECIALIZATION_NODE_KINDS = ['root', 'technique', 'terminal', 'confluence'] as const;
export const SPECIALIZATION_REQUIREMENT_KINDS = ['node_rank', 'attribute_gte', 'pure_school_points_gte'] as const;
export const SPECIALIZATION_GRANT_KINDS = ['stat_modifier', 'battle_hook', 'effect_mutation', 'equipment_slot_capacity'] as const;
export const SPECIALIZATION_SLOT_FAMILIES = ['cloak', 'head', 'left_hand', 'body', 'right_hand', 'arm', 'legs', 'feet', 'accessory'] as const;

const SPECIALIZATION_TOPOLOGY_COSTS: Record<string, number[]> = {
  strength_terminal_three_armed: [22],
  confluence_intelligence_perception_crownless_astrolabe: [10],
  dexterity_terminal_shadow_dance: [4, 2, 2, 20],
  intelligence_terminal_archmage: [4, 2, 2, 20],
  stamina_terminal_absolute_defense: [4, 2, 2, 20]
};

const specializationExpectedCosts = (nodeId: string, kind: string): number[] => {
  if (Object.prototype.hasOwnProperty.call(SPECIALIZATION_TOPOLOGY_COSTS, nodeId)) return SPECIALIZATION_TOPOLOGY_COSTS[nodeId]!;
  if (kind === 'root') return [2, 1, 1, 1, 1, 1];
  if (kind === 'technique') return [2, 1, 1, 1];
  if (kind === 'terminal') return [4, 2, 2];
  if (kind === 'confluence') return [6, 2, 2];
  return [];
};

export type SpecializationSchool = (typeof SPECIALIZATION_SCHOOLS)[number];
export type SpecializationRequirementKind = (typeof SPECIALIZATION_REQUIREMENT_KINDS)[number];
export type SpecializationGrantKind = (typeof SPECIALIZATION_GRANT_KINDS)[number];
export type SpecializationInspectorTab = 'basic' | 'requirements' | 'levels' | 'grants' | 'topology';

export const SPECIALIZATION_SCHOOL_META: Record<SpecializationSchool, { label: string; color: string }> = {
  strength: { label: '力量', color: '#c2413b' },
  dexterity: { label: '敏捷', color: '#2f8f6b' },
  intelligence: { label: '智力', color: '#7656a8' },
  perception: { label: '感知', color: '#2d7ea3' },
  stamina: { label: '耐力', color: '#9a6d28' }
};

export type SpecializationLevel = { level: number; cost: number; grants: DataRow[]; [key: string]: unknown };
export type SpecializationRequirements = { all: DataRow[]; anyOf: DataRow[][] };
export type SpecializationNode = {
  id: string;
  kind: string;
  schoolIds: SpecializationSchool[];
  nameKey: string;
  descKey: string;
  keywordIds: string[];
  exclusiveGroupId?: string;
  requirements: SpecializationRequirements;
  levels: SpecializationLevel[];
  ui: { column: number; row: number; branch?: string; accent?: string; x?: number; y?: number; [key: string]: unknown };
  [key: string]: unknown;
};
export type SpecializationKeyword = {
  id: string;
  mechanicKind: string;
  runtimeIds: string[];
  sourceNodeIds: string[];
  nameKey: string;
  summaryKey: string;
  mechanicDetailsKey: string;
  sourceHintKey: string;
};
export type SpecializationTree = DataRow & {
  id: string;
  schemaVersion: number;
  rulesetVersion: string;
  balanceRevision: number;
  ownerType: string;
  nameKey: string;
  descKey: string;
  initialPoints: number;
  maxPoints: number;
  maxActiveRootNodes: number;
  unlock: DataRow;
  pointBands: DataRow[];
  reset: DataRow;
  schools: DataRow[];
  nodes: SpecializationNode[];
  keywords: SpecializationKeyword[];
  ui: DataRow;
};
export type SpecializationIssue = { message: string; treeId?: string; nodeId?: string; tab: SpecializationInspectorTab };

function finite(value: unknown, fallback: number): number {
  const number = Number(value);
  return Number.isFinite(number) ? number : fallback;
}

const nonEmptyStrings = (value: unknown): boolean => Array.isArray(value) && value.length > 0 && value.every((entry) => typeof entry === 'string' && entry.trim().length > 0);

function defaultSchools(): DataRow[] {
  return SPECIALIZATION_SCHOOLS.map((id, index) => ({
    id,
    nameKey: `specialization.school.${id}.name`,
    descKey: `specialization.school.${id}.desc`,
    attributeKey: id,
    ui: { column: index * 2, color: SPECIALIZATION_SCHOOL_META[id].color }
  }));
}

export function createDefaultSpecializationTree(index = 0): SpecializationTree {
  const suffix = index > 0 ? `_${index + 1}` : '';
  return {
    id: `player_combat_specialization${suffix}`,
    schemaVersion: 2,
    rulesetVersion: 'specialization-v6',
    balanceRevision: 1,
    ownerType: 'player',
    nameKey: `specialization.tree.player_combat_specialization${suffix}.name`,
    descKey: `specialization.tree.player_combat_specialization${suffix}.desc`,
    initialPoints: 2,
    maxPoints: 65,
    maxActiveRootNodes: 3,
    unlock: { battleLevel: 10 },
    pointBands: [
      { id: 'early_growth', fromLevel: 15, toLevel: 90, everyLevels: 5, pointsPerGrant: 1 },
      { id: 'rapid_growth', fromLevel: 100, toLevel: 195, everyLevels: 5, pointsPerGrant: 1 },
      { id: 'level_200', fromLevel: 200, toLevel: 200, everyLevels: 1, pointsPerGrant: 2 },
      { id: 'mastery_growth', fromLevel: 210, toLevel: 300, everyLevels: 10, pointsPerGrant: 2 },
      { id: 'late_polish', fromLevel: 350, toLevel: 450, everyLevels: 50, pointsPerGrant: 1 },
      { id: 'level_cap', fromLevel: 500, toLevel: 500, everyLevels: 1, pointsPerGrant: 2 }
    ],
    reset: { itemId: 'gold_coin', count: 5000 },
    schools: defaultSchools(),
    nodes: [],
    keywords: [],
    ui: { layout: 'five_spoke', schoolOrder: [...SPECIALIZATION_SCHOOLS] },
    i18n: { 'zh-CN': {}, 'en-US': {} }
  };
}

export function createDefaultSpecializationNode(schoolIds: SpecializationSchool[] = ['strength'], index = 0): SpecializationNode {
  const primary = schoolIds[0] ?? 'strength';
  const id = `${primary}_node_${index + 1}`;
  return {
    id,
    kind: schoolIds.length > 1 ? 'confluence' : 'technique',
    schoolIds: schoolIds.slice(0, 2),
    nameKey: `specialization.node.${id}.name`,
    descKey: `specialization.node.${id}.desc`,
    keywordIds: [],
    exclusiveGroupId: '',
    requirements: { all: [], anyOf: [] },
    levels: [{ level: 1, cost: schoolIds.length > 1 ? 6 : 2, grants: [] }],
    ui: {
      column: Math.max(1, index + 1),
      row: 0,
      x: Math.cos((SPECIALIZATION_SCHOOLS.indexOf(primary) * Math.PI * 2 / 5) - Math.PI / 2) * 0.5,
      y: Math.sin((SPECIALIZATION_SCHOOLS.indexOf(primary) * Math.PI * 2 / 5) - Math.PI / 2) * 0.5
    }
  };
}

export function createDefaultSpecializationRequirement(kind: SpecializationRequirementKind = 'node_rank'): DataRow {
  if (kind === 'attribute_gte') return { kind, attributeKey: 'strength', value: 1 };
  if (kind === 'pure_school_points_gte') return { kind, schoolId: 'strength', points: 11 };
  return { kind, nodeId: '', minRank: 1 };
}

export function createDefaultSpecializationGrant(kind: SpecializationGrantKind = 'stat_modifier'): DataRow {
  const id = `${kind}_${Date.now()}`;
  if (kind === 'battle_hook') return { id, kind, battleHooks: [{ id: `hook_${Date.now()}`, trigger: 'onBattleStart', priority: 0, conditions: [], effects: [{ kind: 'heal', targetMode: 'self', maxHpRatio: 0.03, valueFormula: { baseAdd: 0.02, terms: [{ attributeKey: 'stamina', coefficient: 0.00002 }] } }], dispatchHooks: false }] };
  if (kind === 'effect_mutation') return { id, kind, effectMutations: [{ id: `mutation_${Date.now()}`, priority: 0, scope: 'skill', effectSelector: { mode: 'all' }, ops: { amountMultiplier: 1.05 } }] };
  if (kind === 'equipment_slot_capacity') return { id, kind, equipType: 'battle', deltas: { accessory: -1, right_hand: 1 } };
  return { id, kind, scope: 'character', modifiers: [{ attributeKey: 'strength', operation: 'add', value: 1 }] };
}

export function createNextSpecializationLevel(previous: SpecializationLevel, nodeKind: string): SpecializationLevel {
  const defaultCosts: Record<string, number> = { root: 1, technique: 1, terminal: 2, confluence: 2 };
  return {
    level: Math.max(1, Math.trunc(finite(previous.level, 1))) + 1,
    cost: defaultCosts[nodeKind] ?? 1,
    grants: clone(previous.grants)
  };
}

function normalizeReqs(raw: unknown): SpecializationRequirements {
  const source = isRecord(raw) ? raw : {};
  return {
    all: asArray(source.all).filter(isRecord).map((entry) => clone(entry)),
    anyOf: asArray(source.anyOf).map((group) => asArray(group).filter(isRecord).map((entry) => clone(entry)))
  };
}

export function normalizeSpecializationNode(raw: unknown, index: number): SpecializationNode {
  const source = isRecord(raw) ? clone(raw) : {};
  const schools = asArray(source.schoolIds)
    .map((value) => asString(value).trim())
    .filter((value): value is SpecializationSchool => SPECIALIZATION_SCHOOLS.includes(value as SpecializationSchool));
  const schoolIds = Array.from(new Set(schools)).slice(0, 2);
  const fallback = createDefaultSpecializationNode(schoolIds.length ? schoolIds : ['strength'], index);
  const ui = isRecord(source.ui) ? source.ui : {};
  const rawLevels = asArray(source.levels);
  return {
    ...source,
    id: asString(source.id).trim() || fallback.id,
    kind: asString(source.kind).trim() || fallback.kind,
    schoolIds: schoolIds.length ? schoolIds : fallback.schoolIds,
    nameKey: asString(source.nameKey, fallback.nameKey),
    descKey: asString(source.descKey, fallback.descKey),
    keywordIds: Array.from(new Set(asArray(source.keywordIds).map((value) => asString(value).trim()).filter(Boolean))),
    exclusiveGroupId: asString(source.exclusiveGroupId),
    requirements: normalizeReqs(source.requirements),
    levels: (rawLevels.length ? rawLevels : fallback.levels).map((rawLevel, levelIndex) => {
      const level = isRecord(rawLevel) ? clone(rawLevel) : {};
      return { ...level, level: Math.max(1, Math.trunc(finite(level.level, levelIndex + 1))), cost: Math.max(0, finite(level.cost, 1)), grants: asArray(level.grants).filter(isRecord).map((grant) => clone(grant)) };
    }),
    ui: {
      ...ui,
      column: Math.trunc(finite(ui.column, fallback.ui.column)),
      row: Math.trunc(finite(ui.row, fallback.ui.row)),
      ...(ui.x !== undefined && ui.y !== undefined ? { x: finite(ui.x, 0), y: finite(ui.y, 0) } : {})
    }
  };
}

export function normalizeSpecializationTree(raw: unknown, index: number): SpecializationTree {
  const source = isRecord(raw) ? clone(raw) : {};
  const fallback = createDefaultSpecializationTree(index);
  const unlock = isRecord(source.unlock) ? source.unlock : {};
  const reset = isRecord(source.reset) ? source.reset : {};
  return {
    ...source,
    id: asString(source.id).trim() || fallback.id,
    schemaVersion: Math.max(1, Math.trunc(finite(source.schemaVersion, 2))),
    rulesetVersion: asString(source.rulesetVersion).trim() || 'specialization-v6',
    balanceRevision: Math.max(1, Math.trunc(finite(source.balanceRevision, 1))),
    ownerType: asString(source.ownerType).trim() || 'player',
    nameKey: asString(source.nameKey, fallback.nameKey),
    descKey: asString(source.descKey, fallback.descKey),
    initialPoints: Math.max(0, Math.trunc(finite(source.initialPoints, 2))),
    maxPoints: Math.max(0, Math.trunc(finite(source.maxPoints, 65))),
    maxActiveRootNodes: Math.max(0, Math.trunc(finite(source.maxActiveRootNodes, 3))),
    unlock: { ...unlock, battleLevel: Math.max(1, Math.trunc(finite(unlock.battleLevel, 10))) },
    pointBands: asArray(source.pointBands).filter(isRecord).map((band) => clone(band)),
    reset: { ...reset, itemId: asString(reset.itemId, 'gold_coin'), count: Math.max(0, finite(reset.count, 5000)) },
    schools: asArray(source.schools).length ? asArray(source.schools).filter(isRecord).map((school) => clone(school)) : defaultSchools(),
    nodes: asArray(source.nodes).map((node, nodeIndex) => normalizeSpecializationNode(node, nodeIndex)),
    keywords: asArray(source.keywords).filter(isRecord).map((keyword) => clone(keyword) as SpecializationKeyword),
    ui: isRecord(source.ui) ? clone(source.ui) : clone(fallback.ui)
  };
}

export function normalizeSpecializationTrees(rows: unknown[]): SpecializationTree[] {
  return rows.map((row, index) => normalizeSpecializationTree(row, index));
}

function push(out: SpecializationIssue[], message: string, tab: SpecializationInspectorTab, treeId?: string, nodeId?: string) {
  out.push({ message, tab, treeId, nodeId });
}

function specializationRuntimeTokens(value: unknown, tokens = new Set<string>(), parentKind = '', key = ''): Set<string> {
  if (Array.isArray(value)) {
    if (parentKind === 'target_has_any_buff' && key === 'buffIds') return tokens;
    for (const entry of value) specializationRuntimeTokens(entry, tokens, parentKind, key);
    return tokens;
  }
  if (isRecord(value)) {
    const kind = asString(value.kind).trim() || parentKind;
    for (const [childKey, childValue] of Object.entries(value)) specializationRuntimeTokens(childValue, tokens, kind, childKey);
    return tokens;
  }
  if (typeof value === 'string') tokens.add(value);
  return tokens;
}

const SPECIALIZATION_UNORDERED_CORE_ARRAY_KEYS = new Set([
  'grants', 'battleHooks', 'effectMutations', 'appendEffects', 'conditions',
  'skillIds', 'skillCategories', 'weaponClasses', 'damageTypes', 'kinds', 'effectIds'
]);

function canonicalSpecializationCoreSnapshot(value: unknown, containerKey = ''): unknown {
  if (Array.isArray(value)) {
    const entries = value.map((entry) => canonicalSpecializationCoreSnapshot(entry));
    if (SPECIALIZATION_UNORDERED_CORE_ARRAY_KEYS.has(containerKey)) {
      entries.sort((left, right) => {
        const leftRow = isRecord(left) ? left : {};
        const rightRow = isRecord(right) ? right : {};
        const idOrder = asString(leftRow.id).localeCompare(asString(rightRow.id));
        return idOrder || JSON.stringify(left).localeCompare(JSON.stringify(right));
      });
    }
    return entries;
  }
  if (!isRecord(value)) return value;
  return Object.fromEntries(Object.keys(value).sort().map((key) => [key, canonicalSpecializationCoreSnapshot(value[key], key)]));
}

function specializationHookEffectStateBindings(effects: unknown): Map<string, string> {
  const bindings = new Map<string, string>();
  const occurrenceByKind = new Map<string, number>();
  for (const effect of asArray(effects).filter(isRecord)) {
    const kind = asString(effect.kind).trim();
    const occurrence = occurrenceByKind.get(kind) ?? 0;
    occurrenceByKind.set(kind, occurrence + 1);
    for (const field of ['counterId', 'buffId'] as const) {
      const value = asString(effect[field]).trim();
      if (value) bindings.set(`${kind}#${occurrence}.${field}`, value);
    }
    for (const field of ['counterIds', 'buffIds'] as const) {
      asArray(effect[field]).forEach((rawValue, index) => {
        const value = asString(rawValue).trim();
        if (value) bindings.set(`${kind}#${occurrence}.${field}[${index}]`, value);
      });
    }
  }
  return bindings;
}

function hookEffectStateRetentionProblem(previousEffects: unknown, currentEffects: unknown): string | undefined {
  const current = specializationHookEffectStateBindings(currentEffects);
  for (const [binding, value] of specializationHookEffectStateBindings(previousEffects)) {
    if (current.get(binding) !== value) return `${binding}=${value}`;
  }
  const previousCycles = asArray(previousEffects).filter(isRecord).filter((effect) => effect.kind === 'cycle_counter');
  const currentCycles = asArray(currentEffects).filter(isRecord).filter((effect) => effect.kind === 'cycle_counter');
  for (const [index, previousEffect] of previousCycles.entries()) {
    if (currentCycles[index] && Number(currentCycles[index].maxStacks) < Number(previousEffect.maxStacks)) return `cycle_counter#${index}.maxStacks 不能降低`;
  }
  const previousExtraTurns = asArray(previousEffects).filter(isRecord).filter((effect) => effect.kind === 'extra_turn');
  const currentExtraTurns = asArray(currentEffects).filter(isRecord).filter((effect) => effect.kind === 'extra_turn');
  for (const [index, previousEffect] of previousExtraTurns.entries()) {
    const currentEffect = currentExtraTurns[index];
    if (!currentEffect) continue;
    if (Number.isFinite(previousEffect.procChanceBase) && Number(currentEffect.procChanceBase) < Number(previousEffect.procChanceBase)) return `extra_turn#${index}.procChanceBase 不能降低`;
    if (Number.isFinite(previousEffect.procChanceMax) && Number(currentEffect.procChanceMax) < Number(previousEffect.procChanceMax)) return `extra_turn#${index}.procChanceMax 不能降低`;
  }
  return undefined;
}

function specializationConditionRelaxes(previousCondition: DataRow, currentCondition: DataRow): boolean {
  const kind = asString(previousCondition.kind).trim();
  if (kind !== asString(currentCondition.kind).trim()) return false;
  if (kind === 'trigger_limit') {
    const order = ['perBattle', 'perWave', 'perTurn'];
    const previousField = order.find((field) => Number.isInteger(previousCondition[field]));
    const currentField = order.find((field) => Number.isInteger(currentCondition[field]));
    if (!previousField || !currentField || order.indexOf(currentField) < order.indexOf(previousField)) return false;
    return Number(currentCondition[currentField]) >= Number(previousCondition[previousField]);
  }
  const thresholdDirection = new Map<string, 'up' | 'down'>([
    ['self_mp_ratio_gte', 'down'],
    ['counter_gte', 'down'],
    ['context_hit_count_gte', 'down'],
    ['self_hp_ratio_lte', 'up']
  ]);
  const direction = thresholdDirection.get(kind);
  if (direction) {
    const previousIdentity = { ...previousCondition };
    const currentIdentity = { ...currentCondition };
    delete previousIdentity.value;
    delete currentIdentity.value;
    if (JSON.stringify(canonicalSpecializationCoreSnapshot(previousIdentity)) !== JSON.stringify(canonicalSpecializationCoreSnapshot(currentIdentity))) return false;
    return direction === 'down' ? Number(currentCondition.value) <= Number(previousCondition.value) : Number(currentCondition.value) >= Number(previousCondition.value);
  }
  return JSON.stringify(canonicalSpecializationCoreSnapshot(previousCondition)) === JSON.stringify(canonicalSpecializationCoreSnapshot(currentCondition));
}

function nonNarrowingConditionsProblem(previousConditions: unknown, currentConditions: unknown): string | undefined {
  const unmatched = asArray(previousConditions).filter(isRecord);
  for (const currentCondition of asArray(currentConditions).filter(isRecord)) {
    const matchIndex = unmatched.findIndex((previousCondition) => specializationConditionRelaxes(previousCondition, currentCondition));
    if (matchIndex < 0) return '不得新增或收紧 conditions';
    unmatched.splice(matchIndex, 1);
  }
  return undefined;
}

const SPECIALIZATION_LIMIT_SCOPE_ORDER = ['perBattle', 'perWave', 'perTurn'] as const;

function nonNarrowingLimitsProblem(previousLimits: unknown, currentLimits: unknown): string | undefined {
  const current = isRecord(currentLimits) ? currentLimits : undefined;
  if (!current) return undefined;
  const previous = isRecord(previousLimits) ? previousLimits : undefined;
  if (!previous) return '前一级不限次，本级不能新增 limits';
  for (const [currentScope, currentCount] of Object.entries(current)) {
    const currentScopeIndex = SPECIALIZATION_LIMIT_SCOPE_ORDER.indexOf(currentScope as typeof SPECIALIZATION_LIMIT_SCOPE_ORDER[number]);
    const impliedByPrevious = Object.entries(previous).some(([previousScope, previousCount]) => {
      const previousScopeIndex = SPECIALIZATION_LIMIT_SCOPE_ORDER.indexOf(previousScope as typeof SPECIALIZATION_LIMIT_SCOPE_ORDER[number]);
      return previousScopeIndex >= 0 && previousScopeIndex <= currentScopeIndex && Number(previousCount) <= Number(currentCount);
    });
    if (!impliedByPrevious) return `不能收紧 limits：${currentScope}=${currentCount}`;
  }
  return undefined;
}

const SPECIALIZATION_EVENT_FILTER_INCLUDE_ARRAYS = [
  'skillIds', 'skillCategories', 'damageTypes', 'turnKinds', 'buffIds', 'buffKinds',
  'applicationKinds', 'probabilityKinds', 'sourceIds', 'unitTypes'
] as const;
const SPECIALIZATION_EVENT_FILTER_EXCLUDE_ARRAYS = ['excludeSkillIds', 'excludeBuffIds', 'excludeSourceIds'] as const;
const SPECIALIZATION_EVENT_FILTER_INCLUDE_BOOLEANS = ['includeBasicAttack', 'includePreparation', 'includeSuppressedTurns'] as const;
const SPECIALIZATION_EVENT_FILTER_EXCLUDE_BOOLEANS = ['excludeSelf', 'firstBuffInstanceOnly'] as const;

function nonNarrowingEventFilterProblem(previousFilter: unknown, currentFilter: unknown): string | undefined {
  const previous = isRecord(previousFilter) ? previousFilter : {};
  const current = isRecord(currentFilter) ? currentFilter : {};
  for (const key of SPECIALIZATION_EVENT_FILTER_INCLUDE_ARRAYS) {
    if (current[key] === undefined) continue;
    if (previous[key] === undefined) return `${key} 不能在前一级未过滤时新增`;
    const missing = missingRetainedString(previous[key], current[key]);
    if (missing) return `${key} 必须保留 ${missing}`;
  }
  for (const key of SPECIALIZATION_EVENT_FILTER_EXCLUDE_ARRAYS) {
    if (current[key] === undefined) continue;
    if (previous[key] === undefined) return `${key} 不能新增排除项`;
    const added = missingRetainedString(current[key], previous[key]);
    if (added) return `${key} 不能新增排除项 ${added}`;
  }
  for (const key of SPECIALIZATION_EVENT_FILTER_INCLUDE_BOOLEANS) {
    if (current[key] === false && previous[key] !== false) return `${key} 不能排除前一级可触发的事件`;
  }
  for (const key of SPECIALIZATION_EVENT_FILTER_EXCLUDE_BOOLEANS) {
    if (current[key] === true && previous[key] !== true) return `${key} 不能排除前一级可触发的事件`;
  }
  return undefined;
}

function battleHookCounterCapacityProblems(hooks: unknown): string[] {
  const capacityByCounter = new Map<string, number>();
  const demandByCounter = new Map<string, number>();
  const rememberMax = (map: Map<string, number>, rawCounterId: unknown, value: unknown) => {
    const counterId = asString(rawCounterId).trim();
    const numericValue = Number(value);
    if (!counterId || !Number.isFinite(numericValue)) return;
    map.set(counterId, Math.max(map.get(counterId) ?? 0, numericValue));
  };
  for (const hook of asArray(hooks).filter(isRecord)) {
    for (const condition of asArray(hook.conditions).filter(isRecord)) {
      if (condition.kind === 'counter_gte') rememberMax(demandByCounter, condition.counterId, condition.value);
    }
    for (const effect of asArray(hook.effects).filter(isRecord)) {
      if (effect.kind === 'cycle_counter') {
        rememberMax(capacityByCounter, effect.counterId, effect.maxStacks);
        if (isRecord(effect.atMax)) rememberMax(demandByCounter, effect.counterId, effect.atMax.consumeStacks);
      }
      if (effect.kind === 'consume_counter_for_damage_amp') rememberMax(demandByCounter, effect.counterId, effect.consume);
    }
  }
  const problems: string[] = [];
  for (const [counterId, capacity] of capacityByCounter) {
    const demand = demandByCounter.get(counterId) ?? 0;
    if (capacity < demand) problems.push(`cycle_counter ${counterId} maxStacks ${capacity} 低于同机制需求 ${demand}`);
  }
  return problems;
}

function missingRetainedString(previousValues: unknown, currentValues: unknown): string | undefined {
  const current = new Set(asArray(currentValues).map((value) => asString(value).trim()).filter(Boolean));
  return asArray(previousValues).map((value) => asString(value).trim()).find((value) => value && !current.has(value));
}

function nonNarrowingFilterProblem(previousValues: unknown, currentValues: unknown): string | undefined {
  const previousIsBounded = Array.isArray(previousValues);
  const currentIsBounded = Array.isArray(currentValues);
  if (!previousIsBounded && currentIsBounded) return '前一级未限定，本级不能新增过滤条件';
  if (!previousIsBounded || !currentIsBounded) return undefined;
  const missing = missingRetainedString(previousValues, currentValues);
  return missing ? `必须保留 ${missing}` : undefined;
}

function retainedEffectKindProblem(previousEffects: unknown, currentEffects: unknown): string | undefined {
  const currentKinds = new Map<string, number>();
  for (const effect of asArray(currentEffects).filter(isRecord)) {
    const kind = asString(effect.kind).trim();
    currentKinds.set(kind, (currentKinds.get(kind) ?? 0) + 1);
  }
  for (const effect of asArray(previousEffects).filter(isRecord)) {
    const kind = asString(effect.kind).trim();
    if ((currentKinds.get(kind) ?? 0) <= 0) return kind || '<missing>';
    currentKinds.set(kind, currentKinds.get(kind)! - 1);
  }
  return undefined;
}

function appendEffectUpgradeProblem(previousEffect: DataRow, currentEffect: DataRow): string | undefined {
  const kind = asString(previousEffect.kind).trim();
  if (kind === 'splash_damage' && (Number(currentEffect.targetCount) < Number(previousEffect.targetCount) || Number(currentEffect.amountRatio) < Number(previousEffect.amountRatio))) return '不能降低 splash targetCount 或 amountRatio';
  if (kind === 'secondary_damage') {
    if (asString(currentEffect.targetPolicy) !== asString(previousEffect.targetPolicy) || asString(currentEffect.targetPosition) !== asString(previousEffect.targetPosition)) return '必须保留 secondary_damage 的索敌策略';
    if (Number(currentEffect.amountRatio) < Number(previousEffect.amountRatio)) return '不能降低 secondary_damage amountRatio';
  }
  if (kind === 'status') {
    const previousPayload = isRecord(previousEffect.statusPayload) ? previousEffect.statusPayload : {};
    const currentPayload = isRecord(currentEffect.statusPayload) ? currentEffect.statusPayload : {};
    const missing = missingRetainedString(previousPayload.statusIds, currentPayload.statusIds);
    if (missing) return `必须保留 status ${missing}`;
    if (Number(currentPayload.durationBaseTurns) < Number(previousPayload.durationBaseTurns)
      || Number(currentPayload.durationPerSkillLevelTurns) < Number(previousPayload.durationPerSkillLevelTurns)) return '不能降低 status duration';
  }
  if (kind === 'runtime_damage_amp' && Number(currentEffect.value) < Number(previousEffect.value)) return '不能降低 runtime_damage_amp value';
  if (kind === 'consume_hp' && (Number(currentEffect.maxHpRatio) > Number(previousEffect.maxHpRatio) || Number(currentEffect.minimumHp) < Number(previousEffect.minimumHp))) return '不能提高 consume_hp 代价';
  return undefined;
}

export function specializationGrantSnapshotRetentionProblems(previous: unknown, current: unknown): string[] {
  const previousGrant = isRecord(previous) ? previous : {};
  const currentGrant = isRecord(current) ? current : {};
  const grantId = asString(previousGrant.id).trim();
  const previousKind = asString(previousGrant.kind).trim().toLowerCase();
  const currentKind = asString(currentGrant.kind).trim().toLowerCase();
  if (!isRecord(current) || !grantId || !previousKind || currentKind !== previousKind) {
    return [`必须保留前一级 grant id/kind：${grantId || '<missing>'}/${previousKind || '<missing>'}`];
  }
  if (previousKind === 'stat_modifier') {
    if (asString(previousGrant.scope).trim().toLowerCase() !== asString(currentGrant.scope).trim().toLowerCase()) {
      return [`必须保留 stat_modifier scope：${grantId}`];
    }
    const signatures = new Map<string, number>();
    for (const modifier of asArray(currentGrant.modifiers).filter(isRecord)) {
      const signature = `${asString(modifier.attributeKey).trim()}\u0000${asString(modifier.operation).trim().toLowerCase()}`;
      signatures.set(signature, (signatures.get(signature) ?? 0) + 1);
    }
    for (const modifier of asArray(previousGrant.modifiers).filter(isRecord)) {
      const signature = `${asString(modifier.attributeKey).trim()}\u0000${asString(modifier.operation).trim().toLowerCase()}`;
      if ((signatures.get(signature) ?? 0) <= 0) return [`必须保留 stat_modifier attributeKey/operation 签名：${grantId}`];
      signatures.set(signature, signatures.get(signature)! - 1);
    }
    return [];
  }
  if (previousKind === 'battle_hook' || previousKind === 'effect_mutation') {
    const field = previousKind === 'battle_hook' ? 'battleHooks' : 'effectMutations';
    const payloadLabel = previousKind === 'battle_hook' ? 'battleHook' : 'effectMutation';
    const currentById = new Map(asArray(currentGrant[field]).filter(isRecord).map((payload) => [asString(payload.id).trim(), payload]));
    for (const payload of asArray(previousGrant[field]).filter(isRecord)) {
      const payloadId = asString(payload.id).trim();
      const currentPayload = currentById.get(payloadId);
      if (!currentPayload) return [`必须保留 ${payloadLabel} 稳定 payload id：${grantId}/${payloadId || '<missing>'}`];
      if (previousKind === 'battle_hook') {
        const previousTrigger = asString(payload.trigger).trim();
        const currentTrigger = asString(currentPayload.trigger).trim();
        if (previousTrigger !== currentTrigger && !(previousTrigger === 'onBattleStart' && currentTrigger === 'onWaveStart')) return [`必须保留 battleHook trigger（仅允许 onBattleStart->onWaveStart）：${grantId}/${payloadId}/${previousTrigger}->${currentTrigger}`];
        const missingEffectKind = retainedEffectKindProblem(payload.effects, currentPayload.effects);
        if (missingEffectKind) return [`必须保留 battleHook effect kind：${grantId}/${payloadId}/${missingEffectKind}`];
        const stateBindingProblem = hookEffectStateRetentionProblem(payload.effects, currentPayload.effects);
        if (stateBindingProblem) return [`必须保留 battleHook effect 资源绑定：${grantId}/${payloadId}/${stateBindingProblem}`];
        const conditionProblem = nonNarrowingConditionsProblem(payload.conditions, currentPayload.conditions);
        if (conditionProblem) return [`battleHook conditions 不能退化：${grantId}/${payloadId}，${conditionProblem}`];
        const limitsProblem = nonNarrowingLimitsProblem(payload.limits, currentPayload.limits);
        if (limitsProblem) return [`battleHook limits 不能退化：${grantId}/${payloadId}，${limitsProblem}`];
        const eventFilterProblem = nonNarrowingEventFilterProblem(payload.eventFilter, currentPayload.eventFilter);
        if (eventFilterProblem) return [`battleHook eventFilter 不能退化：${grantId}/${payloadId}，${eventFilterProblem}`];
        continue;
      }
      const previousScope = asString(payload.scope).trim();
      const currentScope = asString(currentPayload.scope).trim();
      const allowedScopes = previousScope === 'both' ? new Set(['both'])
        : previousScope === 'skill' ? new Set(['skill', 'both'])
          : new Set(['basic_attack', 'both']);
      if (!allowedScopes.has(currentScope)) return [`必须保留或扩大 effectMutation scope：${grantId}/${payloadId}/${previousScope}->${currentScope}`];
      const previousSelector = isRecord(payload.effectSelector) ? payload.effectSelector : {};
      const currentSelector = isRecord(currentPayload.effectSelector) ? currentPayload.effectSelector : {};
      if (asString(previousSelector.mode).trim() !== asString(currentSelector.mode).trim()) return [`必须保留 effectMutation selector mode：${grantId}/${payloadId}`];
      for (const selectorField of ['skillIds', 'skillCategories', 'weaponClasses', 'damageTypes']) {
        const problem = nonNarrowingFilterProblem(payload[selectorField], currentPayload[selectorField]);
        if (problem) return [`effectMutation 过滤范围不能缩窄：${grantId}/${payloadId}.${selectorField}，${problem}`];
      }
      for (const selectorField of ['effectIds', 'kinds']) {
        const missing = missingRetainedString(previousSelector[selectorField], currentSelector[selectorField]);
        if (missing) return [`必须保留 effectMutation 选择值：${grantId}/${payloadId}.effectSelector.${selectorField}/${missing}`];
      }
      const currentAppendEffects = new Map(asArray(isRecord(currentPayload.ops) ? currentPayload.ops.appendEffects : undefined)
        .filter(isRecord).map((effect) => [asString(effect.id).trim(), effect]));
      for (const effect of asArray(isRecord(payload.ops) ? payload.ops.appendEffects : undefined).filter(isRecord)) {
        const effectId = asString(effect.id).trim();
        const currentEffect = currentAppendEffects.get(effectId);
        if (!currentEffect) return [`必须保留 effectMutation 生成效果稳定 ID：${grantId}/${payloadId}/${effectId || '<missing>'}`];
        if (asString(effect.kind).trim() !== asString(currentEffect.kind).trim()) return [`必须保留 effectMutation 生成效果 kind：${grantId}/${payloadId}/${effectId}`];
        const upgradeProblem = appendEffectUpgradeProblem(effect, currentEffect);
        if (upgradeProblem) return [`effectMutation 生成效果不得降级：${grantId}/${payloadId}/${effectId}，${upgradeProblem}`];
      }
    }
    return [];
  }
  if (previousKind === 'equipment_slot_capacity') {
    const previousDeltas = isRecord(previousGrant.deltas) ? previousGrant.deltas : {};
    const currentFamilies = new Set(Object.keys(isRecord(currentGrant.deltas) ? currentGrant.deltas : {}).map((family) => family.trim()));
    for (const family of Object.keys(previousDeltas)) {
      if (!currentFamilies.has(family.trim())) return [`必须保留装备槽 family：${grantId}/${family}`];
    }
    return [];
  }
  return [`无法保留未知 grant kind：${previousKind}`];
}

function validNonNegative(value: unknown): boolean {
  return Number.isFinite(Number(value)) && Number(value) >= 0;
}

type SpecializationRefs = {
  attributeIds?: string[];
  itemIds?: string[];
  skillIds?: string[];
  skillCategories?: string[];
  weaponClasses?: string[];
  buffIds?: string[];
  buffKinds?: string[];
  skillEffectKinds?: string[];
  effectIds?: string[];
  effectKindsById?: Record<string, string>;
  equipmentEffectIds?: string[];
};

export const SPECIALIZATION_BATTLE_TRIGGERS = ['onAfterDealCrit', 'onAfterDealDamage', 'onAfterSkillCast', 'onAfterTakeDamage', 'onAllyBeforeTakeDamage', 'onBattleStart', 'onBeforeDealDamage', 'onBeforeSkillCast', 'onBeforeTakeDamage', 'onKill', 'onTurnEnd', 'onTurnStart', 'onWaveStart'] as const;
export const SPECIALIZATION_CONDITION_KINDS = ['adjacent_ally_exists', 'context_damage_type', 'context_hit_count_gte', 'context_mp_insufficient', 'context_skill_non_basic', 'counter_gte', 'incoming_damage_would_be_lethal', 'self_hp_ratio_lte', 'self_mp_ratio_gte', 'self_weapon_class', 'source_is_enemy', 'target_has_any_buff', 'target_has_buff', 'target_is_lowest_hp_ally', 'trigger_limit'] as const;
export const SPECIALIZATION_HOOK_EFFECT_KINDS = ['adjust_skill_cooldown', 'consume_counter_for_damage_amp', 'cycle_counter', 'damage', 'extra_turn', 'flat_defense_penetration', 'heal', 'buff_cycle', 'mirror_cast_skill', 'prevent_lethal', 'redirect_damage', 'remove_buff_by_kind', 'repeat_context_effect', 'resource_debt_cast', 'resource_shield', 'restore_mp', 'retaliate', 'runtime_damage_amp', 'shared_damage_reduction', 'shift_position', 'add_buff', 'remove_buff', 'cast_skill'] as const;
export const SPECIALIZATION_RECURSIVE_EFFECT_KINDS = ['damage', 'retaliate', 'repeat_context_effect', 'mirror_cast_skill', 'extra_turn', 'add_buff', 'remove_buff', 'cast_skill'] as const;
export const SPECIALIZATION_MUTATION_OP_KEYS = ['targetCountDelta', 'amountBaseAddDelta', 'cooldownDelta', 'cooldownFloor', 'mpCostDelta', 'mpCostMultiplier', 'hitCountDelta', 'amountMultiplier', 'amountTermsAppend', 'statusIdsAppend', 'statusIdsRemove', 'appendEffects', 'attackStatPolicy', 'damageTypePolicy', 'targetingPatch'] as const;
export const SPECIALIZATION_MUTATION_APPEND_EFFECT_KINDS = ['consume_hp', 'runtime_damage_amp', 'splash_damage', 'secondary_damage', 'status'] as const;

const BATTLE_TRIGGERS = new Set<string>(SPECIALIZATION_BATTLE_TRIGGERS);
const STABLE_BATTLE_EFFECT_ID_PATTERN = /^[a-z0-9][a-z0-9._:-]*$/;
const SPECIALIZATION_PAYLOAD_ID_PATTERN = /^[a-z][a-z0-9_]*$/;
const DAMAGE_TYPES = new Set(['physical', 'magical', 'true']);
const TARGET_MODES = new Set(['self', 'source', 'target', 'enemy', 'marked_enemy', 'targeting', 'lowest_hp_ally']);
const MUTATION_EFFECT_KINDS = new Set(['adjust_skill_cooldown', 'corpse_explosion', 'damage', 'heal', 'resource', 'status']);
const DAMAGE_CONTEXT_TRIGGERS = new Set(['onBeforeDealDamage', 'onAfterDealDamage', 'onAfterDealCrit', 'onBeforeTakeDamage', 'onAfterTakeDamage', 'onAllyBeforeTakeDamage', 'onKill']);
const SKILL_CONTEXT_TRIGGERS = new Set(['onBeforeSkillCast', 'onAfterSkillCast', ...DAMAGE_CONTEXT_TRIGGERS]);
const EVENT_SUBJECT_TRIGGERS = DAMAGE_CONTEXT_TRIGGERS;
const SPECIALIZATION_FORBIDDEN_EVENT_FILTER_KEYS = new Set(['buffIds', 'excludeBuffIds', 'buffKinds', 'firstBuffInstanceOnly', 'applicationKinds', 'probabilityKinds', 'sourceIds', 'excludeSourceIds']);
const RECURSIVE_EFFECT_KINDS = new Set<string>(SPECIALIZATION_RECURSIVE_EFFECT_KINDS);
const VALUE_FORMULA_EFFECT_KINDS = new Set([
  'consume_counter_for_damage_amp',
  'heal',
  'mirror_cast_skill',
  'prevent_lethal',
  'redirect_damage',
  'repeat_context_effect',
  'resource_debt_cast',
  'resource_shield',
  'restore_mp',
  'runtime_damage_amp',
  'shared_damage_reduction'
]);
const CONDITION_KEYS: Record<string, string[]> = {
  adjacent_ally_exists: ['kind'], context_mp_insufficient: ['kind'], context_skill_non_basic: ['kind'], incoming_damage_would_be_lethal: ['kind'], source_is_enemy: ['kind'], target_is_lowest_hp_ally: ['kind'],
  context_damage_type: ['kind', 'value'], context_hit_count_gte: ['kind', 'value'], counter_gte: ['kind', 'counterId', 'value'], self_hp_ratio_lte: ['kind', 'value'], self_mp_ratio_gte: ['kind', 'value'], self_weapon_class: ['kind', 'weaponClasses'], target_has_buff: ['kind', 'buffId'], target_has_any_buff: ['kind', 'buffIds'], trigger_limit: ['kind', 'perTurn', 'perWave', 'perBattle']
};
const HOOK_EFFECT_KEYS: Record<string, string[]> = {
  adjust_skill_cooldown: ['kind', 'targetMode', 'amount', 'selectionMode', 'maxSkills', 'skillIds'], consume_counter_for_damage_amp: ['kind', 'counterId', 'consume', 'value', 'percentDefensePenetration', 'valueFormula'], cycle_counter: ['kind', 'counterId', 'gain', 'maxStacks', 'atMax'], damage: ['kind', 'targetMode', 'damageType', 'amountFormula', 'dispatchHooks'], extra_turn: ['kind', 'targetMode', 'turnMode', 'procChanceBase', 'procChanceMax', 'abortHookOnFail', 'dispatchHooks'], flat_defense_penetration: ['kind', 'valueFormula'], heal: ['kind', 'targetMode', 'maxHpRatio', 'missingHpRatio', 'valueFormula'], buff_cycle: ['kind', 'buffId', 'firstHit', 'buffedHit'], mirror_cast_skill: ['kind', 'amountRatio', 'applyCooldown', 'allowMirroredCastTrigger', 'skillIds', 'dispatchHooks', 'valueFormula'], prevent_lethal: ['kind', 'remainingHpRatio', 'valueFormula'], redirect_damage: ['kind', 'targetMode', 'ratio', 'valueFormula'], remove_buff_by_kind: ['kind', 'buffKind', 'selectionMode', 'maxBuffs'], repeat_context_effect: ['kind', 'amountRatio', 'targetMode', 'dispatchHooks', 'valueFormula'], resource_debt_cast: ['kind', 'hpCostRatio', 'cooldownDelta', 'minimumHp', 'valueFormula'], resource_shield: ['kind', 'resource', 'absorbRatio', 'resourcePerDamage', 'valueFormula'], restore_mp: ['kind', 'maxMpRatio', 'valueFormula'], retaliate: ['kind', 'targetMode', 'strengthRatio', 'staminaRatio', 'dispatchHooks'], runtime_damage_amp: ['kind', 'value', 'valueFormula'], shared_damage_reduction: ['kind', 'value', 'durationTurns', 'valueFormula'], shift_position: ['kind', 'policy', 'maxSteps'], add_buff: ['kind', 'buffId', 'durationTurns', 'targetMode', 'selectionMode', 'dispatchHooks'], remove_buff: ['kind', 'buffId', 'targetMode', 'dispatchHooks'], cast_skill: ['kind', 'skillId', 'amountRatio', 'dispatchHooks']
};
const EVENT_FILTER_KEYS = ['skillIds', 'excludeSkillIds', 'skillCategories', 'damageTypes', 'turnKinds', 'includeBasicAttack', 'includePreparation', 'includeSuppressedTurns', 'excludeSelf', 'buffIds', 'excludeBuffIds', 'buffKinds', 'firstBuffInstanceOnly', 'applicationKinds', 'probabilityKinds', 'sourceIds', 'excludeSourceIds', 'unitTypes'];
const MUTATION_OP_KEYS = SPECIALIZATION_MUTATION_OP_KEYS;

export function specializationTriggerHasSkillContext(trigger: string): boolean {
  return SKILL_CONTEXT_TRIGGERS.has(trigger);
}

export function specializationTriggerHasDamageContext(trigger: string): boolean {
  return DAMAGE_CONTEXT_TRIGGERS.has(trigger);
}

export function specializationTriggerHasEventSubject(trigger: string): boolean {
  return EVENT_SUBJECT_TRIGGERS.has(trigger);
}

function specializationConditionAllowedForTrigger(kind: string, trigger: string): boolean {
  if (kind === 'context_damage_type' || kind === 'context_hit_count_gte') return specializationTriggerHasDamageContext(trigger);
  if (kind === 'context_mp_insufficient') return trigger === 'onBeforeSkillCast';
  if (kind === 'context_skill_non_basic') return specializationTriggerHasSkillContext(trigger);
  if (kind === 'incoming_damage_would_be_lethal') return trigger === 'onBeforeTakeDamage' || trigger === 'onAllyBeforeTakeDamage';
  if (['source_is_enemy', 'target_has_any_buff', 'target_has_buff', 'target_is_lowest_hp_ally'].includes(kind)) return specializationTriggerHasEventSubject(trigger);
  return true;
}

export function specializationConditionKindsForTrigger(trigger: string): string[] {
  return SPECIALIZATION_CONDITION_KINDS.filter((kind) => specializationConditionAllowedForTrigger(kind, trigger));
}

function specializationEffectAllowedForTrigger(kind: string, trigger: string): boolean {
  if (['consume_counter_for_damage_amp', 'flat_defense_penetration', 'runtime_damage_amp'].includes(kind)) return trigger === 'onBeforeDealDamage' || trigger === 'onBeforeTakeDamage';
  if (kind === 'redirect_damage') return trigger === 'onAllyBeforeTakeDamage';
  if (kind === 'prevent_lethal' || kind === 'resource_shield') return trigger === 'onBeforeTakeDamage';
  if (kind === 'resource_debt_cast') return trigger === 'onBeforeSkillCast';
  if (kind === 'mirror_cast_skill') return trigger === 'onAfterSkillCast';
  if (kind === 'buff_cycle') return trigger === 'onAfterDealDamage';
  if (kind === 'repeat_context_effect') return ['onBeforeDealDamage', 'onBeforeTakeDamage', 'onAfterDealDamage', 'onAfterTakeDamage', 'onAfterDealCrit'].includes(trigger);
  if (kind === 'retaliate') return specializationTriggerHasEventSubject(trigger);
  return true;
}

export function specializationHookEffectKindsForTrigger(trigger: string): string[] {
  return SPECIALIZATION_HOOK_EFFECT_KINDS.filter((kind) => specializationEffectAllowedForTrigger(kind, trigger));
}

export function specializationEventFilterKeysForTrigger(trigger: string): string[] {
  const keys: string[] = [];
  if (specializationTriggerHasSkillContext(trigger)) keys.push('skillIds', 'excludeSkillIds', 'skillCategories', 'includeBasicAttack');
  if (specializationTriggerHasDamageContext(trigger)) keys.push('damageTypes');
  if (specializationTriggerHasEventSubject(trigger)) keys.push('unitTypes', 'excludeSelf');
  if (trigger === 'onTurnStart' || trigger === 'onTurnEnd') keys.push('turnKinds', 'includePreparation');
  if (trigger === 'onTurnEnd') keys.push('includeSuppressedTurns');
  return keys;
}

function rejectUnknownKeys(out: SpecializationIssue[], value: DataRow, allowed: readonly string[], label: string, tab: SpecializationInspectorTab, treeId?: string, nodeId?: string) {
  const allowedKeys = new Set(allowed);
  for (const key of Object.keys(value)) if (!allowedKeys.has(key)) push(out, `${label}.${key}: 未知字段`, tab, treeId, nodeId);
}

function validateKnownStringList(out: SpecializationIssue[], value: unknown, known: Set<string>, label: string, tab: SpecializationInspectorTab, treeId: string, nodeId: string) {
  if (!Array.isArray(value) || value.length === 0) return push(out, `${label}: 必须是非空字符串数组`, tab, treeId, nodeId);
  const seen = new Set<string>();
  for (const entry of value) {
    const id = typeof entry === 'string' ? entry.trim() : '';
    if (!id || seen.has(id)) push(out, `${label}: 包含空值或重复值 ${id || '(空)'}`, tab, treeId, nodeId);
    else if (!known.has(id)) push(out, `${label}: 引用未定义 ${id}`, tab, treeId, nodeId);
    seen.add(id);
  }
}

type FiniteRules = { integer?: boolean; min?: number; max?: number; nonZero?: boolean };

function validateFiniteField(out: SpecializationIssue[], value: unknown, label: string, treeId: string, nodeId: string, rules: FiniteRules = {}) {
  const { integer = false, min = -Infinity, max = Infinity, nonZero = false } = rules;
  if (typeof value !== 'number' || !Number.isFinite(value) || (integer && !Number.isInteger(value)) || value < min || value > max || (nonZero && value === 0)) {
    push(out, `${label}: 数值非法`, 'grants', treeId, nodeId);
  }
}

function validateRequiredString(out: SpecializationIssue[], value: unknown, label: string, treeId: string, nodeId: string) {
  if (typeof value !== 'string' || value.trim() === '') push(out, `${label}: 必须是非空字符串`, 'grants', treeId, nodeId);
}

function validateBoolean(out: SpecializationIssue[], value: unknown, label: string, treeId: string, nodeId: string) {
  if (typeof value !== 'boolean') push(out, `${label}: 必须是布尔值`, 'grants', treeId, nodeId);
}

function validateAllowedStringList(out: SpecializationIssue[], value: unknown, allowed: Set<string> | undefined, label: string, treeId: string, nodeId: string) {
  if (!Array.isArray(value) || value.length === 0) return push(out, `${label}: 必须是非空字符串数组`, 'grants', treeId, nodeId);
  const seen = new Set<string>();
  for (const raw of value) {
    const entry = typeof raw === 'string' ? raw.trim() : '';
    if (!entry || seen.has(entry)) push(out, `${label}: 包含空值或重复值 ${entry || '(空)'}`, 'grants', treeId, nodeId);
    else if (allowed && !allowed.has(entry)) push(out, `${label}: 值非法 ${entry}`, 'grants', treeId, nodeId);
    seen.add(entry);
  }
}

function validatePlayerCopy(out: SpecializationIssue[], rawLine: unknown, label: string, treeId: string, nodeId: string, keywordIds: Set<string>, declaredKeywordIds: Set<string>) {
  const line = asString(rawLine).trim();
  if (!line || line.length > 180) push(out, `${label}: 必须是 1-180 字符`, 'basic', treeId, nodeId);
  const tokens = [...line.matchAll(/\[\[keyword:([a-z0-9_]+)\]\]/g)];
  for (const token of tokens) {
    const keywordId = token[1]!;
    if (!keywordIds.has(keywordId)) push(out, `${label}: 引用了未定义的关键词 ${keywordId}`, 'basic', treeId, nodeId);
    else if (!declaredKeywordIds.has(keywordId)) push(out, `${label}: 关键词 ${keywordId} 不属于该节点`, 'basic', treeId, nodeId);
  }
  const plainText = line.replace(/\[\[keyword:[a-z0-9_]+\]\]/g, '');
  if (/[{}\[\]]|\b(?:attributeKey|valueFormula|battleHooks|effectMutations|targetMode|selectionMode|dispatchHooks|baseAdd|coefficient|core formula)\b/i.test(plainText)) {
    push(out, `${label}: 必须是玩家文案，不能暴露 DSL`, 'basic', treeId, nodeId);
  }
}

function validateTriggerLimits(out: SpecializationIssue[], limits: DataRow, label: string, treeId: string, nodeId: string) {
  const keys = ['perTurn', 'perWave', 'perBattle'];
  rejectUnknownKeys(out, limits, keys, label, 'grants', treeId, nodeId);
  if (Object.keys(limits).length === 0) push(out, `${label}: 不能为空`, 'grants', treeId, nodeId);
  for (const key of keys) if (limits[key] !== undefined) validateFiniteField(out, limits[key], `${label}.${key}`, treeId, nodeId, { integer: true, min: 1 });
}

function validateFormulaKeys(out: SpecializationIssue[], raw: unknown, label: string, treeId: string, nodeId: string, attributeIds: Set<string>) {
  if (!isRecord(raw)) return push(out, `${label}: 必须是对象`, 'grants', treeId, nodeId);
  rejectUnknownKeys(out, raw, ['baseAdd', 'terms'], label, 'grants', treeId, nodeId);
  validateFiniteField(out, raw.baseAdd, `${label}.baseAdd`, treeId, nodeId);
  if (!Array.isArray(raw.terms) || raw.terms.length === 0) push(out, `${label}.terms: 必须是非空数组`, 'grants', treeId, nodeId);
  asArray(raw.terms).forEach((term, index) => {
    if (!isRecord(term)) return push(out, `${label}.terms[${index}]: 必须是对象`, 'grants', treeId, nodeId);
    rejectUnknownKeys(out, term, ['attributeKey', 'coefficient'], `${label}.terms[${index}]`, 'grants', treeId, nodeId);
    const attribute = asString(term.attributeKey).trim();
    if (!attributeIds.has(attribute)) push(out, `${label}.terms[${index}].attributeKey: 引用未定义 ${attribute}`, 'grants', treeId, nodeId);
    validateFiniteField(out, term.coefficient, `${label}.terms[${index}].coefficient`, treeId, nodeId, { nonZero: true });
  });
}

function formulaEnvelope(raw: unknown): { minimum: number; maximum: number } | null {
  if (!isRecord(raw) || !Number.isFinite(Number(raw.baseAdd)) || !Array.isArray(raw.terms)) return null;
  let minimum = Number(raw.baseAdd);
  let maximum = Number(raw.baseAdd);
  for (const term of raw.terms) {
    if (!isRecord(term) || !Number.isFinite(Number(term.coefficient))) return null;
    const projected = Number(term.coefficient) * 500;
    minimum += Math.min(0, projected);
    maximum += Math.max(0, projected);
  }
  return { minimum, maximum };
}

function validateFormulaEnvelope(out: SpecializationIssue[], raw: unknown, label: string, treeId: string, nodeId: string, bounds: { min: number; max: number; minExclusive?: boolean }) {
  const envelope = formulaEnvelope(raw);
  if (!envelope) return;
  if (envelope.minimum < bounds.min || (bounds.minExclusive && envelope.minimum <= bounds.min)) push(out, `${label}: 属性 0~500 包络下限越界 ${envelope.minimum}`, 'grants', treeId, nodeId);
  if (envelope.maximum > bounds.max) push(out, `${label}: 属性 0~500 包络上限越界 ${envelope.maximum}`, 'grants', treeId, nodeId);
}

function validateConditionDsl(out: SpecializationIssue[], condition: DataRow, label: string, treeId: string, nodeId: string, trigger: string, refs: { buffIds: Set<string>; weaponClasses: Set<string> }) {
  const kind = asString(condition.kind).trim();
  const keys = CONDITION_KEYS[kind];
  if (!keys) return push(out, `${label}.kind: 未知条件 ${kind || '(空)'}`, 'grants', treeId, nodeId);
  rejectUnknownKeys(out, condition, keys, label, 'grants', treeId, nodeId);
  if (kind === 'context_damage_type' && !DAMAGE_TYPES.has(asString(condition.value))) push(out, `${label}.value: 伤害类型非法`, 'grants', treeId, nodeId);
  if (kind === 'context_hit_count_gte') validateFiniteField(out, condition.value, `${label}.value`, treeId, nodeId, { integer: true, min: 1 });
  if (kind === 'counter_gte') {
    validateRequiredString(out, condition.counterId, `${label}.counterId`, treeId, nodeId);
    validateFiniteField(out, condition.value, `${label}.value`, treeId, nodeId, { integer: true, min: 1 });
  }
  if (kind === 'self_hp_ratio_lte' || kind === 'self_mp_ratio_gte') validateFiniteField(out, condition.value, `${label}.value`, treeId, nodeId, { min: 0, max: 1 });
  if (kind === 'self_weapon_class') validateKnownStringList(out, condition.weaponClasses, refs.weaponClasses, `${label}.weaponClasses`, 'grants', treeId, nodeId);
  if (kind === 'target_has_buff') {
    const buffId = asString(condition.buffId).trim();
    validateRequiredString(out, condition.buffId, `${label}.buffId`, treeId, nodeId);
    if (buffId && !refs.buffIds.has(buffId)) push(out, `${label}.buffId: 引用未定义 ${buffId}`, 'grants', treeId, nodeId);
  }
  if (kind === 'target_has_any_buff') validateKnownStringList(out, condition.buffIds, refs.buffIds, `${label}.buffIds`, 'grants', treeId, nodeId);
  if (kind === 'trigger_limit') {
    const limits = Object.fromEntries(Object.entries(condition).filter(([key]) => key !== 'kind'));
    validateTriggerLimits(out, limits, label, treeId, nodeId);
  }
  if (BATTLE_TRIGGERS.has(trigger) && !specializationConditionAllowedForTrigger(kind, trigger)) push(out, `${label}.kind: 条件 ${kind} 无法在 ${trigger} 获得所需事件上下文`, 'grants', treeId, nodeId);
}

function validateCounterPayoff(out: SpecializationIssue[], raw: unknown, label: string, treeId: string, nodeId: string, attributeIds: Set<string>) {
  if (!isRecord(raw)) return push(out, `${label}: 必须是对象`, 'grants', treeId, nodeId);
  const kind = asString(raw.kind).trim();
  const payoffKeys: Record<string, string[]> = {
    empower_next_hit: ['kind', 'damageAmp', 'consumeStacks', 'valueFormula'],
    restore_mp: ['kind', 'mpRatio', 'consumeStacks', 'valueFormula'],
    restore_mp_and_tick_cooldown: ['kind', 'mpRatio', 'cooldownAmount', 'consumeStacks', 'valueFormula'],
    guard_lowest_hp_ally: ['kind', 'damageReduction', 'durationTurns', 'consumeStacks', 'valueFormula']
  };
  if (!payoffKeys[kind]) return push(out, `${label}.kind: 未知效果 ${kind || '(空)'}`, 'grants', treeId, nodeId);
  rejectUnknownKeys(out, raw, payoffKeys[kind], label, 'grants', treeId, nodeId);
  if (kind === 'empower_next_hit') validateFiniteField(out, raw.damageAmp, `${label}.damageAmp`, treeId, nodeId, { min: 0, max: 0.45, nonZero: true });
  if (kind === 'restore_mp' || kind === 'restore_mp_and_tick_cooldown') {
    validateFiniteField(out, raw.mpRatio, `${label}.mpRatio`, treeId, nodeId, { min: 0, max: 0.5, nonZero: true });
    if (kind === 'restore_mp_and_tick_cooldown') validateFiniteField(out, raw.cooldownAmount, `${label}.cooldownAmount`, treeId, nodeId, { integer: true, nonZero: true });
  }
  if (kind === 'guard_lowest_hp_ally') {
    validateFiniteField(out, raw.damageReduction, `${label}.damageReduction`, treeId, nodeId, { min: 0, max: 0.5, nonZero: true });
    validateFiniteField(out, raw.durationTurns, `${label}.durationTurns`, treeId, nodeId, { integer: true, min: 1 });
  }
  if (raw.valueFormula !== undefined) {
    validateFormulaKeys(out, raw.valueFormula, `${label}.valueFormula`, treeId, nodeId, attributeIds);
    const maximum = kind === 'empower_next_hit' ? 0.45 : 0.5;
    validateFormulaEnvelope(out, raw.valueFormula, `${label}.valueFormula`, treeId, nodeId, { min: 0, minExclusive: true, max: maximum });
  }
  validateFiniteField(out, raw.consumeStacks, `${label}.consumeStacks`, treeId, nodeId, { integer: true, min: 1 });
}

function validateHookEffectDsl(out: SpecializationIssue[], effect: DataRow, label: string, treeId: string, nodeId: string, refs: Required<{ attributeIds: Set<string>; skillIds: Set<string>; skillCategories: Set<string>; buffIds: Set<string>; buffKinds: Set<string> }>, trigger: string) {
  const kind = asString(effect.kind).trim();
  const keys = HOOK_EFFECT_KEYS[kind];
  if (!keys) return push(out, `${label}.kind: 未知效果 ${kind || '(空)'}`, 'grants', treeId, nodeId);
  rejectUnknownKeys(out, effect, keys, label, 'grants', treeId, nodeId);
  if (RECURSIVE_EFFECT_KINDS.has(kind)) validateBoolean(out, effect.dispatchHooks, `${label}.dispatchHooks`, treeId, nodeId);
  if (effect.skillIds !== undefined) validateKnownStringList(out, effect.skillIds, refs.skillIds, `${label}.skillIds`, 'grants', treeId, nodeId);
  if (effect.skillId !== undefined && !refs.skillIds.has(asString(effect.skillId))) push(out, `${label}.skillId: 引用未定义 ${asString(effect.skillId)}`, 'grants', treeId, nodeId);
  if (effect.buffId !== undefined && !refs.buffIds.has(asString(effect.buffId))) push(out, `${label}.buffId: 引用未定义 ${asString(effect.buffId)}`, 'grants', treeId, nodeId);
  if (effect.buffKind !== undefined && !refs.buffKinds.has(asString(effect.buffKind))) push(out, `${label}.buffKind: 引用未定义 ${asString(effect.buffKind)}`, 'grants', treeId, nodeId);
  if (VALUE_FORMULA_EFFECT_KINDS.has(kind) && effect.valueFormula !== undefined) {
    validateFormulaKeys(out, effect.valueFormula, `${label}.valueFormula`, treeId, nodeId, refs.attributeIds);
    const bounds: Record<string, { min: number; max: number; minExclusive?: boolean }> = {
      consume_counter_for_damage_amp: { min: 0, minExclusive: true, max: 0.8 },
      heal: { min: 0, minExclusive: true, max: 0.5 },
      mirror_cast_skill: { min: 0, minExclusive: true, max: 0.75 },
      prevent_lethal: { min: 0, minExclusive: true, max: 0.45 },
      redirect_damage: { min: 0, minExclusive: true, max: 0.65 },
      repeat_context_effect: { min: 0, minExclusive: true, max: 0.75 },
      resource_debt_cast: { min: 0.05, max: 0.2 },
      resource_shield: { min: 0, minExclusive: true, max: 0.65 },
      restore_mp: { min: 0, minExclusive: true, max: 0.5 },
      runtime_damage_amp: { min: 0, minExclusive: true, max: 0.8 },
      shared_damage_reduction: { min: 0, minExclusive: true, max: 0.45 }
    };
    if (bounds[kind]) validateFormulaEnvelope(out, effect.valueFormula, `${label}.valueFormula`, treeId, nodeId, bounds[kind]);
  }
  if (kind === 'adjust_skill_cooldown') {
    if (effect.targetMode !== undefined && !TARGET_MODES.has(asString(effect.targetMode))) push(out, `${label}.targetMode: 非法`, 'grants', treeId, nodeId);
    validateFiniteField(out, effect.amount, `${label}.amount`, treeId, nodeId, { integer: true, nonZero: true });
    if (effect.selectionMode !== undefined && !['max_remaining', 'context_skill', 'all'].includes(asString(effect.selectionMode))) push(out, `${label}.selectionMode: 非法`, 'grants', treeId, nodeId);
    if (effect.maxSkills !== undefined) validateFiniteField(out, effect.maxSkills, `${label}.maxSkills`, treeId, nodeId, { integer: true, min: 1 });
    if (effect.selectionMode === 'context_skill' && effect.maxSkills !== undefined && effect.maxSkills !== 1) push(out, `${label}.maxSkills: context_skill 仅允许 1`, 'grants', treeId, nodeId);
    if (effect.selectionMode === undefined && effect.skillIds === undefined) push(out, `${label}: 需要 selectionMode 或 skillIds`, 'grants', treeId, nodeId);
  }
  if (kind === 'consume_counter_for_damage_amp') {
    validateRequiredString(out, effect.counterId, `${label}.counterId`, treeId, nodeId);
    validateFiniteField(out, effect.consume, `${label}.consume`, treeId, nodeId, { integer: true, min: 1 });
    validateFiniteField(out, effect.value, `${label}.value`, treeId, nodeId, { min: 0, max: 0.8, nonZero: true });
    if (effect.percentDefensePenetration !== undefined) validateFiniteField(out, effect.percentDefensePenetration, `${label}.percentDefensePenetration`, treeId, nodeId, { min: 0, max: 1, nonZero: true });
  }
  if (kind === 'cycle_counter') {
    validateRequiredString(out, effect.counterId, `${label}.counterId`, treeId, nodeId);
    validateFiniteField(out, effect.gain, `${label}.gain`, treeId, nodeId, { integer: true, min: 1 });
    validateFiniteField(out, effect.maxStacks, `${label}.maxStacks`, treeId, nodeId, { integer: true, min: typeof effect.gain === 'number' ? effect.gain : 1 });
    if (effect.atMax !== undefined) {
      validateCounterPayoff(out, effect.atMax, `${label}.atMax`, treeId, nodeId, refs.attributeIds);
      if (isRecord(effect.atMax) && typeof effect.atMax.consumeStacks === 'number' && typeof effect.maxStacks === 'number' && effect.atMax.consumeStacks > effect.maxStacks) push(out, `${label}.atMax.consumeStacks: 不能超过 maxStacks`, 'grants', treeId, nodeId);
    }
  }
  if (kind === 'damage') {
    if (!TARGET_MODES.has(asString(effect.targetMode))) push(out, `${label}.targetMode: 非法`, 'grants', treeId, nodeId);
    if (!DAMAGE_TYPES.has(asString(effect.damageType))) push(out, `${label}.damageType: 非法`, 'grants', treeId, nodeId);
    validateFormulaKeys(out, effect.amountFormula, `${label}.amountFormula`, treeId, nodeId, refs.attributeIds);
  }
  if (kind === 'extra_turn') {
    if (effect.targetMode !== 'self' || effect.turnMode !== 'full') push(out, `${label}: 仅支持 self/full`, 'grants', treeId, nodeId);
    const chanceConfigured = effect.procChanceBase !== undefined || effect.procChanceMax !== undefined;
    if (chanceConfigured) {
      if (effect.procChanceBase === undefined) push(out, `${label}.procChanceBase: 配置概率时必填`, 'grants', treeId, nodeId);
      else validateFiniteField(out, effect.procChanceBase, `${label}.procChanceBase`, treeId, nodeId, { min: 0, max: 1 });
      if (effect.procChanceMax !== undefined) {
        validateFiniteField(out, effect.procChanceMax, `${label}.procChanceMax`, treeId, nodeId, { min: 0, max: 1 });
        if (typeof effect.procChanceBase === 'number' && typeof effect.procChanceMax === 'number' && effect.procChanceMax < effect.procChanceBase) push(out, `${label}.procChanceMax: 不能低于 procChanceBase`, 'grants', treeId, nodeId);
      }
    }
    if (effect.abortHookOnFail !== undefined) validateBoolean(out, effect.abortHookOnFail, `${label}.abortHookOnFail`, treeId, nodeId);
    if (effect.abortHookOnFail === true && !chanceConfigured) push(out, `${label}.abortHookOnFail: 需要 procChanceBase`, 'grants', treeId, nodeId);
  }
  if (kind === 'flat_defense_penetration') validateFormulaKeys(out, effect.valueFormula, `${label}.valueFormula`, treeId, nodeId, refs.attributeIds);
  if (kind === 'heal') {
    if (!TARGET_MODES.has(asString(effect.targetMode))) push(out, `${label}.targetMode: 非法`, 'grants', treeId, nodeId);
    const ratioKeys = ['maxHpRatio', 'missingHpRatio'].filter((key) => effect[key] !== undefined);
    if (ratioKeys.length !== 1) push(out, `${label}: 必须且只能填写一种治疗比例`, 'grants', treeId, nodeId);
    else validateFiniteField(out, effect[ratioKeys[0]], `${label}.${ratioKeys[0]}`, treeId, nodeId, { min: 0, max: 1, nonZero: true });
    if (trigger === 'onBeforeTakeDamage' && effect.targetMode !== 'self') push(out, `${label}.targetMode: onBeforeTakeDamage 治疗仅允许 self`, 'grants', treeId, nodeId);
  }
  if (kind === 'buff_cycle') {
    validateRequiredString(out, effect.buffId, `${label}.buffId`, treeId, nodeId);
    if (effect.firstHit !== 'apply_buff') push(out, `${label}.firstHit: 仅允许 apply_buff`, 'grants', treeId, nodeId);
    if (!isRecord(effect.buffedHit)) push(out, `${label}.buffedHit: 必须是对象`, 'grants', treeId, nodeId);
    else {
      rejectUnknownKeys(out, effect.buffedHit, ['kind', 'value', 'consumeBuff', 'valueFormula'], `${label}.buffedHit`, 'grants', treeId, nodeId);
      if (effect.buffedHit.kind !== 'runtime_damage_amp') push(out, `${label}.buffedHit.kind: 非法`, 'grants', treeId, nodeId);
      validateFiniteField(out, effect.buffedHit.value, `${label}.buffedHit.value`, treeId, nodeId, { min: 0, max: 0.8, nonZero: true });
      if (effect.buffedHit.valueFormula !== undefined) {
        validateFormulaKeys(out, effect.buffedHit.valueFormula, `${label}.buffedHit.valueFormula`, treeId, nodeId, refs.attributeIds);
        validateFormulaEnvelope(out, effect.buffedHit.valueFormula, `${label}.buffedHit.valueFormula`, treeId, nodeId, { min: 0, minExclusive: true, max: 0.8 });
      }
      validateBoolean(out, effect.buffedHit.consumeBuff, `${label}.buffedHit.consumeBuff`, treeId, nodeId);
    }
  }
  if (kind === 'mirror_cast_skill') {
    validateFiniteField(out, effect.amountRatio, `${label}.amountRatio`, treeId, nodeId, { min: 0, max: 0.75, nonZero: true });
    validateBoolean(out, effect.applyCooldown, `${label}.applyCooldown`, treeId, nodeId);
    validateBoolean(out, effect.allowMirroredCastTrigger, `${label}.allowMirroredCastTrigger`, treeId, nodeId);
  }
  if (kind === 'prevent_lethal') validateFiniteField(out, effect.remainingHpRatio, `${label}.remainingHpRatio`, treeId, nodeId, { min: 0, max: 0.45, nonZero: true });
  if (kind === 'redirect_damage') {
    if (effect.targetMode !== 'self') push(out, `${label}.targetMode: redirect_damage 仅允许 self`, 'grants', treeId, nodeId);
    validateFiniteField(out, effect.ratio, `${label}.ratio`, treeId, nodeId, { min: 0, max: 0.65, nonZero: true });
  }
  if (kind === 'remove_buff_by_kind') {
    validateRequiredString(out, effect.buffKind, `${label}.buffKind`, treeId, nodeId);
    if (effect.selectionMode !== 'max_remaining') push(out, `${label}.selectionMode: 仅允许 max_remaining`, 'grants', treeId, nodeId);
    validateFiniteField(out, effect.maxBuffs, `${label}.maxBuffs`, treeId, nodeId, { integer: true, min: 1 });
  }
  if (kind === 'repeat_context_effect') {
    validateFiniteField(out, effect.amountRatio, `${label}.amountRatio`, treeId, nodeId, { min: 0, max: 0.75, nonZero: true });
    if (!TARGET_MODES.has(asString(effect.targetMode))) push(out, `${label}.targetMode: 非法`, 'grants', treeId, nodeId);
  }
  if (kind === 'resource_debt_cast') {
    validateFiniteField(out, effect.hpCostRatio, `${label}.hpCostRatio`, treeId, nodeId, { min: 0.05, max: 0.2, nonZero: true });
    validateFiniteField(out, effect.cooldownDelta, `${label}.cooldownDelta`, treeId, nodeId, { integer: true, nonZero: true });
    validateFiniteField(out, effect.minimumHp, `${label}.minimumHp`, treeId, nodeId, { integer: true, min: 1 });
  }
  if (kind === 'resource_shield') {
    if (effect.resource !== 'mp') push(out, `${label}.resource: 仅允许 mp`, 'grants', treeId, nodeId);
    validateFiniteField(out, effect.absorbRatio, `${label}.absorbRatio`, treeId, nodeId, { min: 0, max: 0.65, nonZero: true });
    validateFiniteField(out, effect.resourcePerDamage, `${label}.resourcePerDamage`, treeId, nodeId, { min: 0, nonZero: true });
  }
  if (kind === 'restore_mp') validateFiniteField(out, effect.maxMpRatio, `${label}.maxMpRatio`, treeId, nodeId, { min: 0, max: 0.5, nonZero: true });
  if (kind === 'retaliate') {
    if (effect.targetMode !== 'source') push(out, `${label}.targetMode: retaliate 仅允许 source`, 'grants', treeId, nodeId);
    validateFiniteField(out, effect.strengthRatio, `${label}.strengthRatio`, treeId, nodeId, { min: 0, nonZero: true });
    validateFiniteField(out, effect.staminaRatio, `${label}.staminaRatio`, treeId, nodeId, { min: 0, nonZero: true });
  }
  if (kind === 'runtime_damage_amp') validateFiniteField(out, effect.value, `${label}.value`, treeId, nodeId, { min: 0, max: 0.8, nonZero: true });
  if (kind === 'shared_damage_reduction') {
    validateFiniteField(out, effect.value, `${label}.value`, treeId, nodeId, { min: 0, max: 0.45, nonZero: true });
    validateFiniteField(out, effect.durationTurns, `${label}.durationTurns`, treeId, nodeId, { integer: true, min: 1 });
  }
  if (kind === 'shift_position') {
    if (effect.policy !== 'toward_lowest_hp_ally') push(out, `${label}.policy: 仅允许 toward_lowest_hp_ally`, 'grants', treeId, nodeId);
    validateFiniteField(out, effect.maxSteps, `${label}.maxSteps`, treeId, nodeId, { integer: true, min: 1 });
  }
  if (kind === 'add_buff' || kind === 'remove_buff') {
    validateRequiredString(out, effect.buffId, `${label}.buffId`, treeId, nodeId);
    if (effect.durationTurns !== undefined) validateFiniteField(out, effect.durationTurns, `${label}.durationTurns`, treeId, nodeId, { integer: true, min: 1 });
    if (effect.targetMode !== undefined && !TARGET_MODES.has(asString(effect.targetMode))) push(out, `${label}.targetMode: 非法`, 'grants', treeId, nodeId);
    if (effect.selectionMode !== undefined && !['highest_attack', 'highest_threat', 'lowest_hp', 'random'].includes(asString(effect.selectionMode))) push(out, `${label}.selectionMode: 非法`, 'grants', treeId, nodeId);
    if (effect.selectionMode !== undefined && effect.targetMode !== 'enemy') push(out, `${label}.selectionMode: 仅能与 enemy targetMode 一起使用`, 'grants', treeId, nodeId);
    validateBoolean(out, effect.dispatchHooks, `${label}.dispatchHooks`, treeId, nodeId);
  }
  if (kind === 'cast_skill') {
    validateRequiredString(out, effect.skillId, `${label}.skillId`, treeId, nodeId);
    if (effect.amountRatio !== undefined) validateFiniteField(out, effect.amountRatio, `${label}.amountRatio`, treeId, nodeId, { min: 0, nonZero: true });
    if (effect.dispatchHooks !== undefined) validateBoolean(out, effect.dispatchHooks, `${label}.dispatchHooks`, treeId, nodeId);
  }
  if (BATTLE_TRIGGERS.has(trigger) && !specializationEffectAllowedForTrigger(kind, trigger)) push(out, `${label}.kind: 效果 ${kind} 无法在 ${trigger} 获得所需事件上下文`, 'grants', treeId, nodeId);
  if (effect.selectionMode === 'context_skill' && !specializationTriggerHasSkillContext(trigger)) push(out, `${label}.selectionMode: context_skill 需要技能或伤害事件上下文`, 'grants', treeId, nodeId);
  if (['source', 'targeting'].includes(asString(effect.targetMode)) && !specializationTriggerHasEventSubject(trigger)) push(out, `${label}.targetMode: ${asString(effect.targetMode)} 需要事件目标上下文`, 'grants', treeId, nodeId);
}

function validateBattleHookDsl(out: SpecializationIssue[], hook: DataRow, label: string, treeId: string, nodeId: string, refs: Required<{ attributeIds: Set<string>; skillIds: Set<string>; skillCategories: Set<string>; weaponClasses: Set<string>; buffIds: Set<string>; buffKinds: Set<string> }>) {
  rejectUnknownKeys(out, hook, ['id', 'trigger', 'priority', 'conditions', 'effects', 'eventFilter', 'limits', 'dispatchHooks'], label, 'grants', treeId, nodeId);
  const trigger = asString(hook.trigger).trim();
  if (!BATTLE_TRIGGERS.has(trigger)) push(out, `${label}.trigger: 未知触发器 ${trigger || '(空)'}`, 'grants', treeId, nodeId);
  validateFiniteField(out, hook.priority, `${label}.priority`, treeId, nodeId, { integer: true });
  validateBoolean(out, hook.dispatchHooks, `${label}.dispatchHooks`, treeId, nodeId);
  if (hook.eventFilter !== undefined) {
    if (!isRecord(hook.eventFilter)) push(out, `${label}.eventFilter: 必须是对象`, 'grants', treeId, nodeId);
    else {
      rejectUnknownKeys(out, hook.eventFilter, EVENT_FILTER_KEYS, `${label}.eventFilter`, 'grants', treeId, nodeId);
      for (const key of ['skillIds', 'excludeSkillIds']) if (hook.eventFilter[key] !== undefined) validateKnownStringList(out, hook.eventFilter[key], refs.skillIds, `${label}.eventFilter.${key}`, 'grants', treeId, nodeId);
      if (hook.eventFilter.skillCategories !== undefined) validateKnownStringList(out, hook.eventFilter.skillCategories, refs.skillCategories, `${label}.eventFilter.skillCategories`, 'grants', treeId, nodeId);
      for (const key of ['buffIds', 'excludeBuffIds']) if (hook.eventFilter[key] !== undefined) validateKnownStringList(out, hook.eventFilter[key], refs.buffIds, `${label}.eventFilter.${key}`, 'grants', treeId, nodeId);
      if (hook.eventFilter.buffKinds !== undefined) validateKnownStringList(out, hook.eventFilter.buffKinds, refs.buffKinds, `${label}.eventFilter.buffKinds`, 'grants', treeId, nodeId);
      if (Object.keys(hook.eventFilter).length === 0) push(out, `${label}.eventFilter: 不能为空`, 'grants', treeId, nodeId);
      if (hook.eventFilter.damageTypes !== undefined) validateAllowedStringList(out, hook.eventFilter.damageTypes, DAMAGE_TYPES, `${label}.eventFilter.damageTypes`, treeId, nodeId);
      if (hook.eventFilter.turnKinds !== undefined) validateAllowedStringList(out, hook.eventFilter.turnKinds, new Set(['ordered', 'extra']), `${label}.eventFilter.turnKinds`, treeId, nodeId);
      if (hook.eventFilter.applicationKinds !== undefined) validateAllowedStringList(out, hook.eventFilter.applicationKinds, new Set(['attach', 'refresh']), `${label}.eventFilter.applicationKinds`, treeId, nodeId);
      if (hook.eventFilter.probabilityKinds !== undefined) validateAllowedStringList(out, hook.eventFilter.probabilityKinds, new Set(['equipment_proc', 'status_apply']), `${label}.eventFilter.probabilityKinds`, treeId, nodeId);
      if (hook.eventFilter.unitTypes !== undefined) validateAllowedStringList(out, hook.eventFilter.unitTypes, new Set(['player', 'kitty', 'enemy', 'summon']), `${label}.eventFilter.unitTypes`, treeId, nodeId);
      for (const key of ['sourceIds', 'excludeSourceIds']) if (hook.eventFilter[key] !== undefined) validateAllowedStringList(out, hook.eventFilter[key], undefined, `${label}.eventFilter.${key}`, treeId, nodeId);
      for (const key of ['includeBasicAttack', 'includePreparation', 'includeSuppressedTurns', 'excludeSelf', 'firstBuffInstanceOnly']) if (hook.eventFilter[key] !== undefined) validateBoolean(out, hook.eventFilter[key], `${label}.eventFilter.${key}`, treeId, nodeId);
      const reachableKeys = new Set(specializationEventFilterKeysForTrigger(trigger));
      for (const key of Object.keys(hook.eventFilter)) {
        if (SPECIALIZATION_FORBIDDEN_EVENT_FILTER_KEYS.has(key)) push(out, `${label}.eventFilter.${key}: 专精未暴露 buff/probability 触发域`, 'grants', treeId, nodeId);
        else if (!reachableKeys.has(key)) push(out, `${label}.eventFilter.${key}: ${trigger || '(空)'} 不提供该事件上下文`, 'grants', treeId, nodeId);
      }
    }
  }
  if (hook.limits !== undefined) {
    if (!isRecord(hook.limits)) push(out, `${label}.limits: 必须是对象`, 'grants', treeId, nodeId);
    else validateTriggerLimits(out, hook.limits, `${label}.limits`, treeId, nodeId);
  }
  asArray(hook.conditions).forEach((condition, index) => {
    const childLabel = `${label}.conditions[${index}]`;
    if (!isRecord(condition)) return push(out, `${childLabel}: 必须是对象`, 'grants', treeId, nodeId);
    validateConditionDsl(out, condition, childLabel, treeId, nodeId, trigger, { buffIds: refs.buffIds, weaponClasses: refs.weaponClasses });
  });
  asArray(hook.effects).forEach((effect, index) => {
    const childLabel = `${label}.effects[${index}]`;
    if (!isRecord(effect)) return push(out, `${childLabel}: 必须是对象`, 'grants', treeId, nodeId);
    validateHookEffectDsl(out, effect, childLabel, treeId, nodeId, refs, trigger);
  });
  const grantsExtraTurn = asArray(hook.effects).some((effect) => isRecord(effect) && effect.kind === 'extra_turn');
  const directExtraTurnLimits = [hook.limits, ...asArray(hook.conditions).filter((condition) => isRecord(condition) && condition.kind === 'trigger_limit')].filter(isRecord);
  const hasOneShotOuterLimit = directExtraTurnLimits.some((limit) => limit.perWave === 1 || limit.perBattle === 1);
  if (grantsExtraTurn && !hasOneShotOuterLimit) push(out, `${label}: 直接额外行动必须明确限制为每波 1 次或每场 1 次`, 'grants', treeId, nodeId);
}

function validateNodeRiskPolicy(out: SpecializationIssue[], node: DataRow, label: string, treeId: string, nodeId: string) {
  const schoolIds = new Set(asArray(node.schoolIds).map((value) => asString(value)));
  asArray(node.levels).filter(isRecord).forEach((level, levelIndex) => {
    asArray(level.grants).filter(isRecord).forEach((grant) => {
      asArray(grant.effectMutations).filter(isRecord).forEach((mutation) => {
        const ops = isRecord(mutation.ops) ? mutation.ops : {};
        if (Number(ops.hitCountDelta) > 0 && !nonEmptyStrings(mutation.weaponClasses)) push(out, `${label}: Lv.${levelIndex + 1} 额外命中必须限定 weaponClasses`, 'grants', treeId, nodeId);
        if (!(Number(ops.cooldownDelta) < 0)) return;
        if (!schoolIds.has('intelligence')) push(out, `${label}: Lv.${levelIndex + 1} 冷却缩减只允许出现在智力路线`, 'grants', treeId, nodeId);
        if (Number(ops.cooldownFloor) < 1) push(out, `${label}: Lv.${levelIndex + 1} 冷却缩减必须设置 cooldownFloor >= 1`, 'grants', treeId, nodeId);
        if (Number(ops.mpCostMultiplier) < 3) push(out, `${label}: Lv.${levelIndex + 1} 冷却缩减必须设置 mpCostMultiplier >= 3`, 'grants', treeId, nodeId);
        if (!nonEmptyStrings(mutation.weaponClasses)) push(out, `${label}: Lv.${levelIndex + 1} 冷却缩减必须限定 weaponClasses`, 'grants', treeId, nodeId);
      });

      asArray(grant.battleHooks).filter(isRecord).forEach((hook) => {
        const conditions = asArray(hook.conditions).filter(isRecord);
        const hasWeaponGate = conditions.some((condition) => condition.kind === 'self_weapon_class' && nonEmptyStrings(condition.weaponClasses));
        const limits = [hook.limits, ...conditions.filter((condition) => condition.kind === 'trigger_limit')].filter(isRecord);
        const hasOneShotLimit = limits.some((limit) => limit.perWave === 1 || limit.perBattle === 1);
        const hasAnyLimit = limits.some((limit) => Number(limit.perTurn || limit.perWave || limit.perBattle) > 0);
        const mpGate = conditions.find((condition) => condition.kind === 'self_mp_ratio_gte');

        asArray(hook.effects).filter(isRecord).forEach((effect) => {
          if (effect.kind === 'adjust_skill_cooldown' && Number(effect.amount) < 0) {
            push(out, `${label}: Lv.${levelIndex + 1} 禁止战斗 Hook 直接缩减冷却，请使用受限的智力系技能改写`, 'grants', treeId, nodeId);
          }
          if (effect.kind === 'cycle_counter' && isRecord(effect.atMax)) {
            if (effect.atMax.kind === 'restore_mp_and_tick_cooldown' && Number(effect.atMax.cooldownAmount) < 0) push(out, `${label}: Lv.${levelIndex + 1} 禁止计数器结算缩减冷却`, 'grants', treeId, nodeId);
            if (effect.atMax.kind === 'extra_turn' || effect.atMax.kind === 'immediate_action') push(out, `${label}: Lv.${levelIndex + 1} 禁止嵌套计数器直接获得行动`, 'grants', treeId, nodeId);
          }
          if (effect.kind === 'extra_turn') {
            if (!hasWeaponGate || !hasOneShotLimit) push(out, `${label}: Lv.${levelIndex + 1} 再动必须有兵器条件以及每波或每场 1 次限制`, 'grants', treeId, nodeId);
            const baseChance = Number(effect.procChanceBase);
            const maxChance = Number(effect.procChanceMax);
            if (!(baseChance > 0) || !(maxChance > 0) || baseChance > maxChance || maxChance > 0.4) push(out, `${label}: Lv.${levelIndex + 1} 再动必须显式配置概率且最高不超过 40%`, 'grants', treeId, nodeId);
            if (effect.abortHookOnFail !== true || effect.dispatchHooks !== false) push(out, `${label}: Lv.${levelIndex + 1} 再动失败必须终止 Hook，且派生行动不得再次派发 Hook`, 'grants', treeId, nodeId);
          }
          if (effect.kind === 'mirror_cast_skill') {
            if (!hasWeaponGate || !hasOneShotLimit || !conditions.some((condition) => condition.kind === 'context_skill_non_basic') || Number(mpGate?.value) < 0.75) push(out, `${label}: Lv.${levelIndex + 1} 复现施法必须有兵器、非普攻、至少 75% MP 与每波/每场限制`, 'grants', treeId, nodeId);
            if (effect.allowMirroredCastTrigger !== false || effect.dispatchHooks !== false || effect.valueFormula !== undefined || Number(effect.amountRatio) > 0.55) push(out, `${label}: Lv.${levelIndex + 1} 复现施法必须不可递归、使用固定倍率且最高 55%`, 'grants', treeId, nodeId);
          }
          if (effect.kind === 'repeat_context_effect') {
            if (!hasWeaponGate || !hasAnyLimit) push(out, `${label}: Lv.${levelIndex + 1} 追击必须有兵器条件和明确触发上限`, 'grants', treeId, nodeId);
            if (effect.dispatchHooks !== false || effect.valueFormula !== undefined || !(Number(effect.amountRatio) > 0) || Number(effect.amountRatio) > 0.45) push(out, `${label}: Lv.${levelIndex + 1} 追击必须不可递归、使用固定倍率且最高 45%`, 'grants', treeId, nodeId);
          }
        });
      });
    });
  });
}

function validateMutationDsl(out: SpecializationIssue[], mutation: DataRow, label: string, treeId: string, nodeId: string, refs: Required<{ attributeIds: Set<string>; skillIds: Set<string>; skillCategories: Set<string>; weaponClasses: Set<string>; buffIds: Set<string>; skillEffectKinds: Set<string>; effectIds: Set<string> }>) {
  rejectUnknownKeys(out, mutation, ['id', 'priority', 'scope', 'skillIds', 'skillCategories', 'weaponClasses', 'damageTypes', 'effectSelector', 'ops'], label, 'grants', treeId, nodeId);
  if (mutation.priority !== undefined) validateFiniteField(out, mutation.priority, `${label}.priority`, treeId, nodeId, { integer: true });
  if (mutation.skillIds !== undefined) validateKnownStringList(out, mutation.skillIds, refs.skillIds, `${label}.skillIds`, 'grants', treeId, nodeId);
  if (mutation.skillCategories !== undefined) validateKnownStringList(out, mutation.skillCategories, refs.skillCategories, `${label}.skillCategories`, 'grants', treeId, nodeId);
  if (mutation.weaponClasses !== undefined) validateKnownStringList(out, mutation.weaponClasses, refs.weaponClasses, `${label}.weaponClasses`, 'grants', treeId, nodeId);
  if (mutation.damageTypes !== undefined) validateAllowedStringList(out, mutation.damageTypes, DAMAGE_TYPES, `${label}.damageTypes`, treeId, nodeId);
  if (!isRecord(mutation.effectSelector)) push(out, `${label}.effectSelector: 必须是对象`, 'grants', treeId, nodeId);
  const selector = isRecord(mutation.effectSelector) ? mutation.effectSelector : {};
  const mode = asString(selector.mode);
  const selectorKeys = mode === 'all' ? ['mode', 'includeGenerated'] : mode === 'kind' ? ['mode', 'kinds', 'includeGenerated'] : mode === 'effect_id' ? ['mode', 'effectIds', 'includeGenerated'] : ['mode', 'includeGenerated'];
  rejectUnknownKeys(out, selector, selectorKeys, `${label}.effectSelector`, 'grants', treeId, nodeId);
  if (selector.includeGenerated !== undefined) validateBoolean(out, selector.includeGenerated, `${label}.effectSelector.includeGenerated`, treeId, nodeId);
  if (mode === 'kind') validateKnownStringList(out, selector.kinds, new Set([...refs.skillEffectKinds].filter((kind) => MUTATION_EFFECT_KINDS.has(kind))), `${label}.effectSelector.kinds`, 'grants', treeId, nodeId);
  if (mode === 'effect_id') validateKnownStringList(out, selector.effectIds, refs.effectIds, `${label}.effectSelector.effectIds`, 'grants', treeId, nodeId);
  const ops = isRecord(mutation.ops) ? mutation.ops : {};
  rejectUnknownKeys(out, ops, MUTATION_OP_KEYS, `${label}.ops`, 'grants', treeId, nodeId);
  if (Object.keys(ops).length === 0) push(out, `${label}.ops: 不能为空`, 'grants', treeId, nodeId);
  for (const key of ['targetCountDelta', 'cooldownDelta', 'mpCostDelta', 'hitCountDelta']) if (ops[key] !== undefined) validateFiniteField(out, ops[key], `${label}.ops.${key}`, treeId, nodeId, { integer: true, nonZero: true });
  if (ops.cooldownFloor !== undefined) validateFiniteField(out, ops.cooldownFloor, `${label}.ops.cooldownFloor`, treeId, nodeId, { integer: true, min: 1 });
  if (ops.mpCostMultiplier !== undefined) validateFiniteField(out, ops.mpCostMultiplier, `${label}.ops.mpCostMultiplier`, treeId, nodeId, { min: 0, nonZero: true });
  if (Number(ops.cooldownDelta) < 0) {
    if (!Number.isInteger(ops.cooldownFloor) || Number(ops.cooldownFloor) < 1) push(out, `${label}.ops: 降低冷却时 cooldownFloor 必须 >= 1`, 'grants', treeId, nodeId);
    if (!Number.isFinite(ops.mpCostMultiplier) || Number(ops.mpCostMultiplier) < 3) push(out, `${label}.ops: 降低冷却时 mpCostMultiplier 必须 >= 3`, 'grants', treeId, nodeId);
    if (Number(ops.mpCostDelta || 0) < 0) push(out, `${label}.ops: 降低冷却时 mpCostDelta 必须 >= 0`, 'grants', treeId, nodeId);
  }
  if (ops.amountBaseAddDelta !== undefined) validateFiniteField(out, ops.amountBaseAddDelta, `${label}.ops.amountBaseAddDelta`, treeId, nodeId, { nonZero: true });
  if (ops.amountMultiplier !== undefined) validateFiniteField(out, ops.amountMultiplier, `${label}.ops.amountMultiplier`, treeId, nodeId, { min: 0, nonZero: true });
  for (const key of ['statusIdsAppend', 'statusIdsRemove']) if (ops[key] !== undefined) validateKnownStringList(out, ops[key], refs.buffIds, `${label}.ops.${key}`, 'grants', treeId, nodeId);
  if (ops.amountTermsAppend !== undefined && (!Array.isArray(ops.amountTermsAppend) || ops.amountTermsAppend.length === 0)) push(out, `${label}.ops.amountTermsAppend: 必须是非空数组`, 'grants', treeId, nodeId);
  const amountTerms = asArray(ops.amountTermsAppend).filter(isRecord);
  if (amountTerms.length > 2) push(out, `${label}.ops.amountTermsAppend: 最多允许 2 个受控属性项`, 'grants', treeId, nodeId);
  const termAttributes = new Set<string>();
  amountTerms.forEach((term, index) => {
    if (!isRecord(term)) return push(out, `${label}.ops.amountTermsAppend[${index}]: 必须是对象`, 'grants', treeId, nodeId);
    rejectUnknownKeys(out, term, ['attributeKey', 'coefficient'], `${label}.ops.amountTermsAppend[${index}]`, 'grants', treeId, nodeId);
    const attributeKey = asString(term.attributeKey);
    if (!refs.attributeIds.has(attributeKey)) push(out, `${label}.ops.amountTermsAppend[${index}].attributeKey: 引用未定义 ${attributeKey}`, 'grants', treeId, nodeId);
    if (termAttributes.has(attributeKey)) push(out, `${label}.ops.amountTermsAppend: 属性重复 ${attributeKey}`, 'grants', treeId, nodeId);
    termAttributes.add(attributeKey);
    validateFiniteField(out, term.coefficient, `${label}.ops.amountTermsAppend[${index}].coefficient`, treeId, nodeId, { min: 0, nonZero: true });
  });
  if (amountTerms.length > 0) {
    const positiveHitDelta = Math.max(0, Number(ops.hitCountDelta) || 0);
    const positiveTargetDelta = Math.max(0, Number(ops.targetCountDelta) || 0);
    if (positiveHitDelta > 1 || positiveTargetDelta > 1 || (positiveHitDelta > 0 && positiveTargetDelta > 0)) push(out, `${label}.ops: 最多增加 1 次命中或 1 个目标，不能同时增加`, 'grants', treeId, nodeId);
    const amountMultiplier = ops.amountMultiplier === undefined ? 1 : Number(ops.amountMultiplier);
    if ((positiveHitDelta > 0 || positiveTargetDelta > 0) && (amountMultiplier < 0.5 || amountMultiplier > 0.8)) push(out, `${label}.ops.amountMultiplier: 扩展命中/目标时必须在 0.5~0.8`, 'grants', treeId, nodeId);
    const hasNarrowingFilter = mutation.skillIds !== undefined || mutation.skillCategories !== undefined || mutation.weaponClasses !== undefined || mutation.damageTypes !== undefined;
    const broadSkillSelector = mutation.scope === 'skill' && !hasNarrowingFilter && (mode === 'all' || mode === 'kind');
    const isWide = mutation.scope === 'both' || (mode === 'all' && !hasNarrowingFilter) || broadSkillSelector;
    const perTermLimit = isWide ? 0.04 : 0.05;
    const sumLimit = isWide ? 0.05 : 0.08;
    const projectionLimit = isWide ? 25 : 40;
    const relativeLimit = isWide ? 0.12 : 0.22;
    const coefficients = amountTerms.map((term) => Number(term.coefficient) || 0);
    const coefficientSum = coefficients.reduce((sum, value) => sum + value, 0);
    if (coefficients.some((value) => value > perTermLimit) || coefficientSum > sumLimit) push(out, `${label}.ops.amountTermsAppend: 超出${isWide ? '宽域' : '受限域'}单项或总系数上限`, 'grants', treeId, nodeId);
    if (coefficientSum * 500 > projectionLimit) push(out, `${label}.ops.amountTermsAppend: 属性 500 投影增量过高`, 'grants', treeId, nodeId);
    const coverageAdjusted = coefficientSum / 0.55 * (1 + positiveHitDelta) * (1 + positiveTargetDelta) * amountMultiplier;
    if (coverageAdjusted > relativeLimit) push(out, `${label}.ops.amountTermsAppend: 结合命中、目标与倍率后的覆盖包络过高`, 'grants', treeId, nodeId);
  }
  if (ops.appendEffects !== undefined && (!Array.isArray(ops.appendEffects) || ops.appendEffects.length === 0)) push(out, `${label}.ops.appendEffects: 必须是非空数组`, 'grants', treeId, nodeId);
  const appendedEffects = asArray(ops.appendEffects);
  const appendedEffectIds = new Set<string>();
  appendedEffects.forEach((effect, index) => {
    if (!isRecord(effect)) return push(out, `${label}.ops.appendEffects[${index}]: 必须是对象`, 'grants', treeId, nodeId);
    const appendKeys: Record<string, string[]> = { consume_hp: ['id', 'kind', 'maxHpRatio', 'minimumHp'], runtime_damage_amp: ['id', 'kind', 'value'], splash_damage: ['id', 'kind', 'targetCount', 'amountRatio'], secondary_damage: ['id', 'kind', 'targetPolicy', 'targetPosition', 'amountRatio'], status: ['id', 'kind', 'statusPayload'] };
    const kind = asString(effect.kind);
    if (!appendKeys[kind]) push(out, `${label}.ops.appendEffects[${index}].kind: 未知效果 ${kind || '(空)'}`, 'grants', treeId, nodeId);
    else {
      const effectLabel = `${label}.ops.appendEffects[${index}]`;
      rejectUnknownKeys(out, effect, appendKeys[kind], effectLabel, 'grants', treeId, nodeId);
      const effectId = asString(effect.id).trim();
      validateRequiredString(out, effect.id, `${effectLabel}.id`, treeId, nodeId);
      if (effectId && appendedEffectIds.has(effectId)) push(out, `${label}.ops.appendEffects: id 重复 ${effectId}`, 'grants', treeId, nodeId);
      if (effectId) appendedEffectIds.add(effectId);
      if (kind === 'consume_hp') {
        validateFiniteField(out, effect.maxHpRatio, `${effectLabel}.maxHpRatio`, treeId, nodeId, { min: 0, max: 1, nonZero: true });
        validateFiniteField(out, effect.minimumHp, `${effectLabel}.minimumHp`, treeId, nodeId, { integer: true, min: 1 });
      }
      if (kind === 'runtime_damage_amp') validateFiniteField(out, effect.value, `${effectLabel}.value`, treeId, nodeId, { min: 0, nonZero: true });
      if (kind === 'splash_damage') {
        validateFiniteField(out, effect.targetCount, `${effectLabel}.targetCount`, treeId, nodeId, { integer: true, min: 1 });
        validateFiniteField(out, effect.amountRatio, `${effectLabel}.amountRatio`, treeId, nodeId, { min: 0, nonZero: true });
      }
      if (kind === 'secondary_damage') {
        const policy = asString(effect.targetPolicy);
        if (!['same_row', 'fixed_row', 'all_rows'].includes(policy)) push(out, `${effectLabel}.targetPolicy: 非法`, 'grants', treeId, nodeId);
        validateFiniteField(out, effect.amountRatio, `${effectLabel}.amountRatio`, treeId, nodeId, { min: 0, max: 0.5, nonZero: true });
        if (policy === 'fixed_row') {
          if (!['vanguard', 'midguard', 'rearguard'].includes(asString(effect.targetPosition))) push(out, `${effectLabel}.targetPosition: fixed_row 必须指定合法排位`, 'grants', treeId, nodeId);
        } else if (effect.targetPosition !== undefined) push(out, `${effectLabel}.targetPosition: 仅 fixed_row 可填写`, 'grants', treeId, nodeId);
      }
      if (kind === 'status') {
        if (!isRecord(effect.statusPayload)) push(out, `${effectLabel}.statusPayload: 必须是对象`, 'grants', treeId, nodeId);
        else {
          rejectUnknownKeys(out, effect.statusPayload, ['statusIds', 'durationBaseTurns', 'durationPerSkillLevelTurns'], `${effectLabel}.statusPayload`, 'grants', treeId, nodeId);
          validateKnownStringList(out, effect.statusPayload.statusIds, refs.buffIds, `${effectLabel}.statusPayload.statusIds`, 'grants', treeId, nodeId);
          validateFiniteField(out, effect.statusPayload.durationBaseTurns, `${effectLabel}.statusPayload.durationBaseTurns`, treeId, nodeId, { integer: true, min: 1 });
          validateFiniteField(out, effect.statusPayload.durationPerSkillLevelTurns, `${effectLabel}.statusPayload.durationPerSkillLevelTurns`, treeId, nodeId, { integer: true, min: 0 });
        }
      }
    }
  });
  const secondaryDamageCount = appendedEffects.filter((effect) => isRecord(effect) && asString(effect.kind) === 'secondary_damage').length;
  if (secondaryDamageCount > 1) push(out, `${label}.ops.appendEffects: secondary_damage 最多只能配置 1 个`, 'grants', treeId, nodeId);
  if (secondaryDamageCount === 1 && (
    appendedEffects.some((effect) => isRecord(effect) && asString(effect.kind) === 'splash_damage')
    || Number(ops.targetCountDelta) > 0
    || Number(ops.hitCountDelta) > 0
  )) push(out, `${label}.ops: secondary_damage 不能与其他命中或目标扩展组合`, 'grants', treeId, nodeId);
  if (ops.attackStatPolicy !== undefined) {
    if (asString(ops.attackStatPolicy) !== 'higher') push(out, `${label}.ops.attackStatPolicy: 非法`, 'grants', treeId, nodeId);
    if (mode === 'all') push(out, `${label}.ops.attackStatPolicy: 必须显式限定为伤害效果`, 'grants', treeId, nodeId);
  }
  if (ops.damageTypePolicy !== undefined && !['preserve', 'physical', 'magical', 'higher_final_damage'].includes(asString(ops.damageTypePolicy))) push(out, `${label}.ops.damageTypePolicy: 非法`, 'grants', treeId, nodeId);
  if (ops.targetingPatch !== undefined) {
    if (!isRecord(ops.targetingPatch)) push(out, `${label}.ops.targetingPatch: 必须是对象`, 'grants', treeId, nodeId);
    else {
      rejectUnknownKeys(out, ops.targetingPatch, ['targetScope', 'targetPositions', 'secondarySeek', 'selectRule', 'selectAttrKey', 'fallbackSelfIfEmpty', 'targetCount'], `${label}.ops.targetingPatch`, 'grants', treeId, nodeId);
      if (Object.keys(ops.targetingPatch).length === 0) push(out, `${label}.ops.targetingPatch: 不能为空`, 'grants', treeId, nodeId);
      if (ops.targetingPatch.targetScope !== undefined && !['self', 'ally', 'enemy', 'all'].includes(asString(ops.targetingPatch.targetScope))) push(out, `${label}.ops.targetingPatch.targetScope: 非法`, 'grants', treeId, nodeId);
      if (ops.targetingPatch.targetPositions !== undefined) validateAllowedStringList(out, ops.targetingPatch.targetPositions, new Set(['vanguard', 'midguard', 'rearguard']), `${label}.ops.targetingPatch.targetPositions`, treeId, nodeId);
      if (ops.targetingPatch.secondarySeek !== undefined && !['front_to_rear', 'rear_to_front', 'random_alive', 'none'].includes(asString(ops.targetingPatch.secondarySeek))) push(out, `${label}.ops.targetingPatch.secondarySeek: 非法`, 'grants', treeId, nodeId);
      if (ops.targetingPatch.selectRule !== undefined && !['hp_max', 'hp_min', 'hp_percent_max', 'hp_percent_min', 'first', 'boss_first', 'attr_max', 'attr_min', 'random', 'self'].includes(asString(ops.targetingPatch.selectRule))) push(out, `${label}.ops.targetingPatch.selectRule: 非法`, 'grants', treeId, nodeId);
      const hasSelectAttrKey = Object.prototype.hasOwnProperty.call(ops.targetingPatch, 'selectAttrKey');
      const isAttributeRule = ['attr_max', 'attr_min'].includes(asString(ops.targetingPatch.selectRule));
      if (hasSelectAttrKey && (typeof ops.targetingPatch.selectAttrKey !== 'string' || asString(ops.targetingPatch.selectAttrKey).trim() === '')) push(out, `${label}.ops.targetingPatch.selectAttrKey: 必须是非空字符串`, 'grants', treeId, nodeId);
      if (isAttributeRule !== hasSelectAttrKey) push(out, `${label}.ops.targetingPatch: selectRule attr_max/attr_min 必须与 selectAttrKey 同时填写`, 'grants', treeId, nodeId);
      if (hasSelectAttrKey && !refs.attributeIds.has(asString(ops.targetingPatch.selectAttrKey).trim())) push(out, `${label}.ops.targetingPatch.selectAttrKey: 引用未定义 ${asString(ops.targetingPatch.selectAttrKey)}`, 'grants', treeId, nodeId);
      if (ops.targetingPatch.fallbackSelfIfEmpty !== undefined) validateBoolean(out, ops.targetingPatch.fallbackSelfIfEmpty, `${label}.ops.targetingPatch.fallbackSelfIfEmpty`, treeId, nodeId);
      if (ops.targetingPatch.targetCount !== undefined) validateFiniteField(out, ops.targetingPatch.targetCount, `${label}.ops.targetingPatch.targetCount`, treeId, nodeId, { integer: true, min: 1 });
    }
  }
}

function validateRequirement(out: SpecializationIssue[], requirement: DataRow, label: string, treeId: string, nodeId: string, nodeIds: Set<string>, dependencies: string[], attributeIds: Set<string>) {
  const kind = asString(requirement.kind);
  if (!SPECIALIZATION_REQUIREMENT_KINDS.includes(kind as SpecializationRequirementKind)) {
    push(out, `${label}: kind 非法 ${kind || '(空)'}`, 'requirements', treeId, nodeId);
    return;
  }
  if (kind === 'node_rank') {
    rejectUnknownKeys(out, requirement, ['kind', 'nodeId', 'minRank'], label, 'requirements', treeId, nodeId);
    const target = asString(requirement.nodeId).trim();
    dependencies.push(target);
    if (!target || !nodeIds.has(target)) push(out, `${label}: nodeId 未定义 ${target || '(空)'}`, 'requirements', treeId, nodeId);
    if (target === nodeId) push(out, `${label}: 不能指向自己`, 'requirements', treeId, nodeId);
    if (!Number.isInteger(requirement.minRank) || Number(requirement.minRank) < 1) push(out, `${label}: minRank 必须是正整数`, 'requirements', treeId, nodeId);
  } else if (kind === 'attribute_gte') {
    rejectUnknownKeys(out, requirement, ['kind', 'attributeKey', 'value'], label, 'requirements', treeId, nodeId);
    const attributeKey = asString(requirement.attributeKey).trim();
    if (!SPECIALIZATION_SCHOOLS.includes(attributeKey as SpecializationSchool) || (attributeIds.size && !attributeIds.has(attributeKey))) push(out, `${label}: attributeKey 未定义 ${attributeKey || '(空)'}`, 'requirements', treeId, nodeId);
    if (!validNonNegative(requirement.value)) push(out, `${label}: value 必须是非负数`, 'requirements', treeId, nodeId);
  } else {
    rejectUnknownKeys(out, requirement, ['kind', 'schoolId', 'points'], label, 'requirements', treeId, nodeId);
    const schoolId = asString(requirement.schoolId).trim();
    if (!SPECIALIZATION_SCHOOLS.includes(schoolId as SpecializationSchool)) push(out, `${label}: schoolId 未定义 ${schoolId || '(空)'}`, 'requirements', treeId, nodeId);
    if (!Number.isInteger(Number(requirement.points)) || Number(requirement.points) < 1) push(out, `${label}: points 必须是正整数`, 'requirements', treeId, nodeId);
  }
}

function collectFormulaAttributeIds(value: unknown, out = new Set<string>()): Set<string> {
  if (Array.isArray(value)) {
    for (const entry of value) collectFormulaAttributeIds(entry, out);
    return out;
  }
  if (!isRecord(value)) return out;
  const attributeKey = typeof value.attributeKey === 'string' ? value.attributeKey.trim() : '';
  if (attributeKey && typeof value.coefficient === 'number' && Number.isFinite(value.coefficient) && value.coefficient !== 0) out.add(attributeKey);
  if (value.kind === 'retaliate') {
    if (typeof value.strengthRatio === 'number' && Number.isFinite(value.strengthRatio) && value.strengthRatio !== 0) out.add('strength');
    if (typeof value.staminaRatio === 'number' && Number.isFinite(value.staminaRatio) && value.staminaRatio !== 0) out.add('stamina');
  }
  for (const entry of Object.values(value)) collectFormulaAttributeIds(entry, out);
  return out;
}

type SpecializationAppendEffectEntry = {
  id: string;
  owner: string;
  label: string;
  treeId?: string;
  nodeId?: string;
};

function specializationAppendEffectEntries(rows: unknown[]): SpecializationAppendEffectEntry[] {
  const entries: SpecializationAppendEffectEntry[] = [];
  rows.forEach((rawTree, treeIndex) => {
    if (!isRecord(rawTree)) return;
    const treeId = asString(rawTree.id).trim();
    asArray(rawTree.nodes).forEach((rawNode, nodeIndex) => {
      if (!isRecord(rawNode)) return;
      const nodeId = asString(rawNode.id).trim();
      asArray(rawNode.levels).forEach((rawLevel, levelIndex) => {
        if (!isRecord(rawLevel)) return;
        asArray(rawLevel.grants).forEach((rawGrant, grantIndex) => {
          if (!isRecord(rawGrant)) return;
          asArray(rawGrant.effectMutations).forEach((rawMutation, mutationIndex) => {
            if (!isRecord(rawMutation) || !isRecord(rawMutation.ops)) return;
            asArray(rawMutation.ops.appendEffects).forEach((rawEffect, effectIndex) => {
              if (!isRecord(rawEffect)) return;
              const id = asString(rawEffect.id).trim();
              if (!id) return;
              entries.push({
                id,
                owner: `${treeId}:${nodeId}:${asString(rawGrant.id).trim()}:${asString(rawMutation.id).trim()}:${id.toLowerCase()}`,
                label: `专精树 ${treeId || `#${treeIndex + 1}`} / ${nodeId || `节点 #${nodeIndex + 1}`} / levels[${levelIndex}].grants[${grantIndex}].effectMutations[${mutationIndex}].ops.appendEffects[${effectIndex}]`,
                treeId,
                nodeId
              });
            });
          });
        });
      });
    });
  });
  return entries;
}

export function collectSpecializationEffectIds(rows: unknown[], sourceEffectIds: string[] = []): string[] {
  const idsByIdentity = new Map<string, string>();
  for (const rawId of sourceEffectIds) {
    const id = asString(rawId).trim();
    if (id && !idsByIdentity.has(id.toLowerCase())) idsByIdentity.set(id.toLowerCase(), id);
  }
  for (const entry of specializationAppendEffectEntries(rows)) {
    const identity = entry.id.toLowerCase();
    if (!idsByIdentity.has(identity)) idsByIdentity.set(identity, entry.id);
  }
  return [...idsByIdentity.values()].sort();
}

function collectStableEffectIds(out: SpecializationIssue[], rows: unknown[], sourceEffectIds: string[]): Set<string> {
  const effectIds = new Set<string>();
  const ownerByIdentity = new Map<string, string>();
  for (const rawId of sourceEffectIds) {
    const id = asString(rawId).trim();
    if (!id) continue;
    const identity = id.toLowerCase();
    if (!ownerByIdentity.has(identity)) ownerByIdentity.set(identity, '技能或装备效果');
    effectIds.add(id);
  }
  for (const entry of specializationAppendEffectEntries(rows)) {
    if (!STABLE_BATTLE_EFFECT_ID_PATTERN.test(entry.id)) {
      push(out, `${entry.label}.id: 稳定效果 ID 只能使用小写字母、数字、点、下划线、冒号或连字符`, 'grants', entry.treeId, entry.nodeId);
    }
    const identity = entry.id.toLowerCase();
    const previousOwner = ownerByIdentity.get(identity);
    if (previousOwner && previousOwner !== entry.owner) {
      push(out, `${entry.label}.id: 稳定效果 ID 全局重复 ${entry.id}（${previousOwner}）`, 'grants', entry.treeId, entry.nodeId);
    } else {
      ownerByIdentity.set(identity, entry.owner);
    }
    effectIds.add(entry.id);
  }
  return effectIds;
}

type SpecializationMutationDescriptor = {
  treeId: string;
  nodeId: string;
  rank: number;
  maxRank: number;
  grantId: string;
  runtimeId: string;
  priority: number;
  mutation: DataRow;
  label: string;
};

function collectMutationDescriptors(rows: unknown[]): SpecializationMutationDescriptor[] {
  const descriptors: SpecializationMutationDescriptor[] = [];
  rows.forEach((rawTree) => {
    if (!isRecord(rawTree)) return;
    const treeId = asString(rawTree.id).trim();
    asArray(rawTree.nodes).filter(isRecord).forEach((node) => {
      const nodeId = asString(node.id).trim();
      asArray(node.levels).filter(isRecord).forEach((level, levelIndex) => {
        asArray(level.grants).filter(isRecord).forEach((grant, grantIndex) => {
          if (grant.kind !== 'effect_mutation') return;
          const grantId = asString(grant.id).trim();
          asArray(grant.effectMutations).filter(isRecord).forEach((mutation, mutationIndex) => {
            descriptors.push({
              treeId,
              nodeId,
              rank: Number.isInteger(level.level) ? Number(level.level) : levelIndex + 1,
              maxRank: asArray(node.levels).length,
              grantId,
              runtimeId: `${treeId}:${nodeId}:${grantId}:${asString(mutation.id).trim()}`,
              priority: Number.isInteger(mutation.priority) ? Number(mutation.priority) : 0,
              mutation,
              label: `专精树 ${treeId} / ${nodeId}: Lv.${levelIndex + 1} grant #${grantIndex + 1} effectMutations #${mutationIndex + 1}`
            });
          });
        });
      });
    });
  });
  return descriptors;
}

function guaranteedNodeDependencies(rawTree: DataRow): Map<string, Map<string, number>> {
  const nodes = new Map(asArray(rawTree.nodes).filter(isRecord).map((node) => [asString(node.id).trim(), node]));
  const memo = new Map<string, Map<string, number>>();
  const visiting = new Set<string>();
  const mergeMaximum = (target: Map<string, number>, source: Map<string, number>) => {
    for (const [dependencyId, minRank] of source) target.set(dependencyId, Math.max(target.get(dependencyId) ?? 0, minRank));
  };
  const intersectMinimum = (maps: Map<string, number>[]): Map<string, number> => {
    if (maps.length === 0) return new Map();
    const result = new Map<string, number>();
    for (const dependencyId of maps[0].keys()) {
      if (maps.slice(1).every((map) => map.has(dependencyId))) result.set(dependencyId, Math.min(...maps.map((map) => map.get(dependencyId)!)));
    }
    return result;
  };
  const resolve = (nodeId: string): Map<string, number> => {
    const cached = memo.get(nodeId);
    if (cached) return new Map(cached);
    if (visiting.has(nodeId)) return new Map();
    visiting.add(nodeId);
    const result = new Map<string, number>();
    const node = nodes.get(nodeId);
    const requirements = normalizeReqs(node?.requirements);
    const dependencyMap = (requirement: DataRow): Map<string, number> => {
      if (requirement.kind !== 'node_rank') return new Map();
      const dependency = asString(requirement.nodeId).trim();
      if (!dependency) return new Map();
      const dependencies = resolve(dependency);
      dependencies.set(dependency, Math.max(dependencies.get(dependency) ?? 0, Number(requirement.minRank) || 1));
      return dependencies;
    };
    for (const requirement of requirements.all) mergeMaximum(result, dependencyMap(requirement));
    for (const group of requirements.anyOf) {
      mergeMaximum(result, intersectMinimum(group.map(dependencyMap)));
    }
    visiting.delete(nodeId);
    memo.set(nodeId, result);
    return new Map(result);
  };
  for (const nodeId of nodes.keys()) resolve(nodeId);
  return memo;
}

function validateMutationKindCompatibility(out: SpecializationIssue[], descriptor: SpecializationMutationDescriptor, selectedKinds: string[]) {
  if (selectedKinds.length === 0) return;
  const ops = isRecord(descriptor.mutation.ops) ? descriptor.mutation.ops : {};
  const requireKinds = (operation: string, allowed: Set<string>) => {
    const invalid = selectedKinds.find((kind) => !allowed.has(kind));
    if (invalid) push(out, `${descriptor.label}.ops: 操作 ${operation} 不能用于效果类型 ${invalid}`, 'grants', descriptor.treeId, descriptor.nodeId);
  };
  if (['amountBaseAddDelta', 'amountTermsAppend', 'amountMultiplier'].some((key) => key in ops)) requireKinds('amount', new Set(['damage', 'heal']));
  if (['hitCountDelta', 'attackStatPolicy', 'damageTypePolicy'].some((key) => key in ops)) requireKinds('damage', new Set(['damage']));
  if (['statusIdsAppend', 'statusIdsRemove'].some((key) => key in ops)) requireKinds('status', new Set(['status']));
  if (asArray(ops.appendEffects).filter(isRecord).some((effect) => ['runtime_damage_amp', 'splash_damage', 'secondary_damage'].includes(asString(effect.kind)))) requireKinds('appendEffects', new Set(['damage']));
}

export function validateSpecializationMutationSemantics(out: SpecializationIssue[], rows: unknown[], refs: SpecializationRefs) {
  const descriptors = collectMutationDescriptors(rows);
  const effectKindsById = new Map<string, string>();
  for (const [id, kind] of Object.entries(refs.effectKindsById ?? {})) {
    const normalizedId = id.trim().toLowerCase();
    const normalizedKind = asString(kind).trim().toLowerCase();
    if (normalizedId && normalizedKind) effectKindsById.set(normalizedId, normalizedKind);
  }
  const equipmentEffectIds = new Set((refs.equipmentEffectIds ?? []).map((id) => asString(id).trim().toLowerCase()).filter(Boolean));
  const generated = new Map<string, { producer: SpecializationMutationDescriptor; kind: string }[]>();
  for (const descriptor of descriptors) {
    const ops = isRecord(descriptor.mutation.ops) ? descriptor.mutation.ops : {};
    for (const effect of asArray(ops.appendEffects).filter(isRecord)) {
      const id = asString(effect.id).trim().toLowerCase();
      if (id) {
        const producers = generated.get(id) ?? [];
        producers.push({ producer: descriptor, kind: asString(effect.kind).trim() });
        generated.set(id, producers);
      }
    }
  }
  const dependenciesByTree = new Map<string, Map<string, Map<string, number>>>();
  for (const tree of rows.filter(isRecord)) dependenciesByTree.set(asString(tree.id).trim(), guaranteedNodeDependencies(tree));
  const executesBefore = (producer: SpecializationMutationDescriptor, consumer: SpecializationMutationDescriptor) => producer.priority > consumer.priority || (producer.priority === consumer.priority && producer.runtimeId < consumer.runtimeId);

  for (const descriptor of descriptors) {
    const selector = isRecord(descriptor.mutation.effectSelector) ? descriptor.mutation.effectSelector : {};
    const mode = asString(selector.mode).trim();
    const ops = isRecord(descriptor.mutation.ops) ? descriptor.mutation.ops : {};
    const selectedKinds: string[] = [];
    if (mode === 'kind') selectedKinds.push(...asArray(selector.kinds).map((kind) => asString(kind).trim().toLowerCase()).filter(Boolean));
    if (mode !== 'effect_id') {
      validateMutationKindCompatibility(out, descriptor, selectedKinds);
      continue;
    }
    const hasSkillScalarOps = ops.cooldownDelta !== undefined || ops.cooldownFloor !== undefined || ops.mpCostDelta !== undefined || ops.mpCostMultiplier !== undefined;
    for (const rawEffectId of asArray(selector.effectIds)) {
      const effectId = asString(rawEffectId).trim();
      const normalizedEffectId = effectId.toLowerCase();
      const generatedSnapshots = generated.get(normalizedEffectId) ?? [];
      if (generatedSnapshots.length > 0) {
        if (hasSkillScalarOps) push(out, `${descriptor.label}: 生成效果 ${effectId} 不能使用技能级标量操作`, 'grants', descriptor.treeId, descriptor.nodeId);
        if (selector.includeGenerated !== true) push(out, `${descriptor.label}.effectSelector: 选择生成效果 ${effectId} 时必须启用 includeGenerated`, 'grants', descriptor.treeId, descriptor.nodeId);
        const sameTreeSnapshots = generatedSnapshots.filter((entry) => entry.producer.treeId === descriptor.treeId);
        if (sameTreeSnapshots.length === 0) {
          push(out, `${descriptor.label}.effectSelector: 不能选择其他专精树生成的效果 ${effectId}`, 'grants', descriptor.treeId, descriptor.nodeId);
          continue;
        }
        const producerNodeIds = new Set(sameTreeSnapshots.map((entry) => entry.producer.nodeId));
        if (producerNodeIds.size !== 1) {
          push(out, `${descriptor.label}.effectSelector: 生成效果 ${effectId} 存在多个生产身份`, 'grants', descriptor.treeId, descriptor.nodeId);
          continue;
        }
        const producerNodeId = sameTreeSnapshots[0]!.producer.nodeId;
        let reachableSnapshots: typeof sameTreeSnapshots = [];
        if (producerNodeId === descriptor.nodeId) {
          reachableSnapshots = sameTreeSnapshots.filter((entry) => entry.producer.rank === descriptor.rank);
          if (reachableSnapshots.length === 0) push(out, `${descriptor.label}.effectSelector: 生成效果 ${effectId} 在当前节点 Lv.${descriptor.rank} 不生效`, 'grants', descriptor.treeId, descriptor.nodeId);
        } else {
          const guaranteedRank = dependenciesByTree.get(descriptor.treeId)?.get(descriptor.nodeId)?.get(producerNodeId) ?? 0;
          const firstProducerRank = Math.min(...sameTreeSnapshots.map((entry) => entry.producer.rank));
          if (guaranteedRank < firstProducerRank) {
            push(out, `${descriptor.label}.effectSelector: 生成效果 ${effectId} 需要生产节点 ${producerNodeId} 至少 Lv.${firstProducerRank}`, 'grants', descriptor.treeId, descriptor.nodeId);
          } else {
            reachableSnapshots = sameTreeSnapshots.filter((entry) => entry.producer.rank >= guaranteedRank);
            const producerMaxRank = sameTreeSnapshots[0]!.producer.maxRank;
            for (let rank = guaranteedRank; rank <= producerMaxRank; rank += 1) {
              if (!reachableSnapshots.some((entry) => entry.producer.rank === rank)) push(out, `${descriptor.label}.effectSelector: 生成效果 ${effectId} 在可达生产节点 Lv.${rank} 不生效`, 'grants', descriptor.treeId, descriptor.nodeId);
            }
          }
        }
        for (const generatedEffect of reachableSnapshots) {
          const producer = generatedEffect.producer;
          if (producer === descriptor) push(out, `${descriptor.label}.effectSelector: 变异不能选择自身生成的效果 ${effectId}`, 'grants', descriptor.treeId, descriptor.nodeId);
          else if (!executesBefore(producer, descriptor)) push(out, `${descriptor.label}.effectSelector: 生产变异 ${producer.runtimeId} 必须先于消费变异执行（优先级降序、稳定 ID 升序）`, 'grants', descriptor.treeId, descriptor.nodeId);
          if (generatedEffect.kind === 'splash_damage' || generatedEffect.kind === 'secondary_damage') selectedKinds.push('damage');
          else if (generatedEffect.kind === 'status') selectedKinds.push('status');
          else push(out, `${descriptor.label}.effectSelector: 生成效果 ${effectId} 类型 ${generatedEffect.kind} 不会物化为可变异效果`, 'grants', descriptor.treeId, descriptor.nodeId);
        }
        continue;
      }
      const kind = effectKindsById.get(normalizedEffectId);
      if (!kind) {
        push(out, `${descriptor.label}.effectSelector: 稳定效果 ${effectId} 未知或在运行时不活跃`, 'grants', descriptor.treeId, descriptor.nodeId);
        continue;
      }
      if (equipmentEffectIds.has(normalizedEffectId) && hasSkillScalarOps) push(out, `${descriptor.label}: 装备普攻效果 ${effectId} 不能使用技能级标量操作`, 'grants', descriptor.treeId, descriptor.nodeId);
      selectedKinds.push(kind);
    }
    validateMutationKindCompatibility(out, descriptor, selectedKinds);
  }
}

export function validateSpecializationTrees(rows: unknown[], refs: SpecializationRefs = {}): SpecializationIssue[] {
  const out: SpecializationIssue[] = [];
  const treeIds = new Set<string>();
  const attributeIds = new Set(refs.attributeIds ?? []);
  const itemIds = new Set(refs.itemIds ?? []);
  const skillIds = new Set(refs.skillIds ?? []);
  const skillCategories = new Set(refs.skillCategories ?? []);
  const weaponClasses = new Set(refs.weaponClasses ?? []);
  const buffIds = new Set(refs.buffIds ?? []);
  const buffKinds = new Set(refs.buffKinds ?? []);
  const skillEffectKinds = new Set(refs.skillEffectKinds ?? []);
  const effectIds = collectStableEffectIds(out, rows, refs.effectIds ?? []);
  const capacityRules: Record<string, { base: number; min: number; max: number }> = {
    cloak: { base: 1, min: 0, max: 1 },
    head: { base: 1, min: 0, max: 2 },
    left_hand: { base: 1, min: 0, max: 1 },
    body: { base: 1, min: 0, max: 2 },
    right_hand: { base: 1, min: 0, max: 2 },
    arm: { base: 1, min: 0, max: 2 },
    legs: { base: 1, min: 0, max: 1 },
    feet: { base: 1, min: 0, max: 2 },
    accessory: { base: 4, min: 0, max: 8 }
  };
  const finiteNumber = (value: unknown): value is number => typeof value === 'number' && Number.isFinite(value);
  if (rows.length === 0) push(out, '专精树表不能为空', 'basic');
  rows.forEach((rawTree, treeIndex) => {
    if (!isRecord(rawTree)) return push(out, `专精树 #${treeIndex + 1}: 必须是 JSON 对象`, 'basic');
    const treeId = asString(rawTree.id).trim();
    rejectUnknownKeys(out, rawTree, ['id', 'schemaVersion', 'rulesetVersion', 'balanceRevision', 'ownerType', 'nameKey', 'descKey', 'initialPoints', 'maxPoints', 'maxActiveRootNodes', 'unlock', 'pointBands', 'reset', 'schools', 'nodes', 'keywords', 'ui', 'i18n'], `专精树 ${treeId || `#${treeIndex + 1}`}`, 'basic', treeId);
    if (!treeId || treeIds.has(treeId)) push(out, `专精树 #${treeIndex + 1}: id 空或重复 ${treeId}`, 'basic', treeId);
    treeIds.add(treeId);
    if (rawTree.schemaVersion !== 2) push(out, `专精树 ${treeId}: schemaVersion 必须固定为 2`, 'basic', treeId);
    if (rawTree.rulesetVersion !== 'specialization-v6') push(out, `专精树 ${treeId}: rulesetVersion 必须固定为 specialization-v6`, 'basic', treeId);
    if (!Number.isInteger(rawTree.balanceRevision) || Number(rawTree.balanceRevision) <= 0) push(out, `专精树 ${treeId}: balanceRevision 必须是正整数`, 'basic', treeId);
    if (rawTree.ownerType !== 'player') push(out, `专精树 ${treeId}: ownerType 必须是 player`, 'basic', treeId);
    if (rawTree.nameKey !== `specialization.tree.${treeId}.name`) push(out, `专精树 ${treeId}: nameKey 必须为 specialization.tree.${treeId}.name`, 'basic', treeId);
    if (rawTree.descKey !== `specialization.tree.${treeId}.desc`) push(out, `专精树 ${treeId}: descKey 必须为 specialization.tree.${treeId}.desc`, 'basic', treeId);
    if (Number(rawTree.initialPoints) !== 2) push(out, `专精树 ${treeId}: 10级初始专精点必须固定为 2`, 'basic', treeId);
    if (Number(rawTree.maxPoints) !== 65 || Number(rawTree.maxPoints) <= Number(rawTree.initialPoints)) push(out, `专精树 ${treeId}: 500级专精点上限必须是 65`, 'basic', treeId);
    if (Number(rawTree.maxActiveRootNodes) !== 3) push(out, `专精树 ${treeId}: 最多激活根路线必须固定为 3`, 'basic', treeId);
    const unlock = isRecord(rawTree.unlock) ? rawTree.unlock : {};
    if (isRecord(rawTree.unlock)) rejectUnknownKeys(out, rawTree.unlock, ['battleLevel'], `专精树 ${treeId}.unlock`, 'basic', treeId);
    if (Number(unlock.battleLevel) !== 10) push(out, `专精树 ${treeId}: unlock.battleLevel 必须固定为 10`, 'basic', treeId);
    const reset = isRecord(rawTree.reset) ? rawTree.reset : {};
    if (isRecord(rawTree.reset)) rejectUnknownKeys(out, rawTree.reset, ['itemId', 'count'], `专精树 ${treeId}.reset`, 'basic', treeId);
    const resetItem = asString(reset.itemId).trim();
    if (!resetItem || !Number.isInteger(reset.count) || Number(reset.count) <= 0) push(out, `专精树 ${treeId}: reset 需要 itemId 与正整数 count`, 'basic', treeId);
    if (resetItem && itemIds.size && !itemIds.has(resetItem)) push(out, `专精树 ${treeId}: reset.itemId 未定义 ${resetItem}`, 'basic', treeId);
    const bandIds = new Set<string>();
    const pointBands = asArray(rawTree.pointBands).filter(isRecord);
    if (!Array.isArray(rawTree.pointBands) || rawTree.pointBands.length !== pointBands.length) push(out, `专精树 ${treeId}: pointBands 必须只包含对象`, 'basic', treeId);
    if (pointBands.length === 0) push(out, `专精树 ${treeId}: pointBands 不能为空`, 'basic', treeId);
    let previousToLevel = Number(unlock.battleLevel || 0);
    let awardedPoints = Number(rawTree.initialPoints || 0);
    pointBands.forEach((band, index) => {
      rejectUnknownKeys(out, band, ['id', 'fromLevel', 'toLevel', 'everyLevels', 'pointsPerGrant'], `专精树 ${treeId}.pointBands[${index}]`, 'basic', treeId);
      const id = asString(band.id).trim();
      if (!id || bandIds.has(id)) push(out, `专精树 ${treeId}: pointBands #${index + 1} id 空或重复`, 'basic', treeId);
      bandIds.add(id);
      const from = Number(band.fromLevel);
      const to = Number(band.toLevel);
      const every = Number(band.everyLevels);
      const points = Number(band.pointsPerGrant);
      if (!Number.isInteger(from) || !Number.isInteger(to) || from <= previousToLevel || to < from || to > 500) push(out, `专精树 ${treeId}: pointBands ${id} 等级范围非法`, 'basic', treeId);
      if (!Number.isInteger(every) || every <= 0 || (to - from) % every !== 0) push(out, `专精树 ${treeId}: pointBands ${id} everyLevels 无法整除区间`, 'basic', treeId);
      if (!Number.isInteger(points) || points <= 0) push(out, `专精树 ${treeId}: pointBands ${id} pointsPerGrant 必须是正整数`, 'basic', treeId);
      if (Number.isInteger(every) && every > 0 && to >= from) awardedPoints += (Math.floor((to - from) / every) + 1) * points;
      previousToLevel = to;
    });
    if (awardedPoints !== Number(rawTree.maxPoints)) push(out, `专精树 ${treeId}: pointBands 总点数 ${awardedPoints} 与 maxPoints ${rawTree.maxPoints} 不一致`, 'basic', treeId);
    const pointsAtLevel = (battleLevel: number) => Number(rawTree.initialPoints || 0) + pointBands.reduce((total, band) => {
      const from = Number(band.fromLevel);
      const to = Number(band.toLevel);
      const every = Number(band.everyLevels);
      const points = Number(band.pointsPerGrant);
      if (!Number.isFinite(from) || !Number.isFinite(to) || !Number.isFinite(every) || every <= 0 || battleLevel < from) return total;
      return total + (Math.floor((Math.min(battleLevel, to) - from) / every) + 1) * points;
    }, 0);
    for (const [battleLevel, expectedPoints] of [[200, 40], [300, 60], [500, 65]]) {
      const actualPoints = pointsAtLevel(battleLevel);
      if (actualPoints !== expectedPoints) push(out, `专精树 ${treeId}: ${battleLevel}级必须累计获得 ${expectedPoints} 点，当前为 ${actualPoints}`, 'basic', treeId);
    }
    const schools = asArray(rawTree.schools).filter(isRecord);
    if (!Array.isArray(rawTree.schools) || rawTree.schools.length !== schools.length) push(out, `专精树 ${treeId}: schools 必须只包含对象`, 'basic', treeId);
    if (schools.length !== 5) push(out, `专精树 ${treeId}: schools 必须恰好包含五派`, 'basic', treeId);
    const schoolIds = new Set(schools.map((school) => asString(school.id).trim()));
    const schoolColumns = new Set<number>();
    for (const school of SPECIALIZATION_SCHOOLS) if (!schoolIds.has(school)) push(out, `专精树 ${treeId}: 缺少必需流派 ${school}`, 'basic', treeId);
    for (const school of schools) {
      const id = asString(school.id).trim();
      rejectUnknownKeys(out, school, ['id', 'nameKey', 'descKey', 'attributeKey', 'ui'], `专精树 ${treeId}.schools.${id}`, 'basic', treeId);
      if (!SPECIALIZATION_SCHOOLS.includes(id as SpecializationSchool)) push(out, `专精树 ${treeId}: 未知流派 ${id}`, 'basic', treeId);
      if (school.nameKey !== `specialization.school.${id}.name` || school.descKey !== `specialization.school.${id}.desc`) push(out, `专精树 ${treeId}: 流派 ${id} nameKey/descKey 非法`, 'basic', treeId);
      const attributeKey = asString(school.attributeKey).trim();
      if (attributeKey !== id || (attributeIds.size && !attributeIds.has(attributeKey))) push(out, `专精树 ${treeId}: 流派 ${id} attributeKey 必须引用同名属性`, 'basic', treeId);
      const ui = isRecord(school.ui) ? school.ui : {};
      if (isRecord(school.ui)) rejectUnknownKeys(out, school.ui, ['column', 'color'], `专精树 ${treeId}.schools.${id}.ui`, 'basic', treeId);
      if (!Number.isInteger(ui.column) || Number(ui.column) < 0 || schoolColumns.has(Number(ui.column))) push(out, `专精树 ${treeId}: 流派 ${id} ui.column 非法或重复`, 'basic', treeId);
      schoolColumns.add(Number(ui.column));
      if (typeof ui.color !== 'string' || !/^#[0-9a-f]{6}$/i.test(ui.color)) push(out, `专精树 ${treeId}: 流派 ${id} ui.color 必须是六位十六进制颜色`, 'basic', treeId);
    }
    const treeUI = isRecord(rawTree.ui) ? rawTree.ui : {};
    if (isRecord(rawTree.ui)) rejectUnknownKeys(out, rawTree.ui, ['layout', 'schoolOrder'], `专精树 ${treeId}.ui`, 'basic', treeId);
    if (asString(treeUI.layout) !== 'five_spoke' || asArray(treeUI.schoolOrder).map((value) => asString(value)).join('|') !== SPECIALIZATION_SCHOOLS.join('|')) push(out, `专精树 ${treeId}: ui 必须是 five_spoke 且 schoolOrder 顺序固定`, 'basic', treeId);
    const i18n = isRecord(rawTree.i18n) ? rawTree.i18n : {};
    if (isRecord(rawTree.i18n)) rejectUnknownKeys(out, rawTree.i18n, ['zh-CN', 'en-US'], `专精树 ${treeId}.i18n`, 'basic', treeId);

    const rawNodes = asArray(rawTree.nodes).filter(isRecord);
    if (!Array.isArray(rawTree.nodes) || rawTree.nodes.length !== rawNodes.length) push(out, `专精树 ${treeId}: nodes 必须只包含对象`, 'basic', treeId);
    if (rawNodes.length !== 70) push(out, `专精树 ${treeId}: nodes 必须恰好包含 70 个节点`, 'basic', treeId);
    const nodeIds = new Set<string>();
    const uiPositions = new Set<string>();
    const radialPositions = new Set<string>();
    rawNodes.forEach((node, index) => {
      const id = asString(node.id).trim();
      if (!id || nodeIds.has(id)) push(out, `专精树 ${treeId}: 节点 #${index + 1} id 空或重复 ${id}`, 'basic', treeId, id);
      nodeIds.add(id);
    });
    const nodeMaxRanks = new Map(rawNodes.map((node) => [asString(node.id).trim(), asArray(node.levels).length]));
    const keywordKinds = new Set(['counter', 'repeat_cast', 'repeat_effect', 'damage_intercept', 'extra_turn', 'debuff_family', 'debuff', 'buff']);
    const rawKeywords = asArray(rawTree.keywords).filter(isRecord);
    const keywordIds = new Set<string>();
    const runtimeTokensByNode = new Map(rawNodes.map((node) => [asString(node.id), specializationRuntimeTokens(node.levels)]));
    if (!Array.isArray(rawTree.keywords) || rawKeywords.length === 0 || rawKeywords.length !== asArray(rawTree.keywords).length) push(out, `专精树 ${treeId}: keywords 必须是非空对象数组`, 'basic', treeId);
    rawKeywords.forEach((keyword, index) => {
      const id = asString(keyword.id).trim();
      const label = `专精树 ${treeId}.keywords[${index}]`;
      rejectUnknownKeys(out, keyword, ['id', 'mechanicKind', 'runtimeIds', 'sourceNodeIds', 'nameKey', 'summaryKey', 'mechanicDetailsKey', 'sourceHintKey'], label, 'basic', treeId);
      if (!id || keywordIds.has(id)) push(out, `${label}: id 为空或重复`, 'basic', treeId);
      keywordIds.add(id);
      if (!keywordKinds.has(asString(keyword.mechanicKind))) push(out, `${label}: mechanicKind 非法`, 'basic', treeId);
      const runtimeIds = asArray(keyword.runtimeIds).map((value) => asString(value).trim()).filter(Boolean);
      const sourceNodeIds = asArray(keyword.sourceNodeIds).map((value) => asString(value).trim()).filter(Boolean);
      if (runtimeIds.length === 0 || new Set(runtimeIds).size !== runtimeIds.length) push(out, `${label}: runtimeIds 必须非空且唯一`, 'basic', treeId);
      if (sourceNodeIds.length === 0 || new Set(sourceNodeIds).size !== sourceNodeIds.length || sourceNodeIds.some((nodeId) => !nodeIds.has(nodeId))) push(out, `${label}: sourceNodeIds 必须引用现有节点且唯一`, 'basic', treeId);
      if (sourceNodeIds.some((nodeId) => !runtimeIds.some((runtimeId) => runtimeTokensByNode.get(nodeId)?.has(runtimeId)))) push(out, `${label}: sourceNodeIds 未包含声明的运行时语义`, 'basic', treeId);
      if (runtimeIds.some((runtimeId) => !sourceNodeIds.some((nodeId) => runtimeTokensByNode.get(nodeId)?.has(runtimeId)))) push(out, `${label}: runtimeIds 存在未声明来源节点的运行时语义`, 'basic', treeId);
      if (keyword.nameKey !== `specialization.keyword.${id}.name` || keyword.summaryKey !== `specialization.keyword.${id}.summary`
        || keyword.mechanicDetailsKey !== `specialization.keyword.${id}.mechanic_details` || keyword.sourceHintKey !== `specialization.keyword.${id}.source_hint`) push(out, `${label}: 本地化键必须由稳定 id 派生`, 'basic', treeId);
    });
    for (const node of rawNodes) {
      const nodeId = asString(node.id);
      const runtimeTokens = runtimeTokensByNode.get(nodeId) ?? new Set<string>();
      const expected = rawKeywords.filter((keyword) => asArray(keyword.runtimeIds).some((runtimeId) => runtimeTokens.has(asString(runtimeId)))).map((keyword) => asString(keyword.id)).sort();
      const authored = asArray(node.keywordIds).map((value) => asString(value).trim()).filter(Boolean).sort();
      if (!Array.isArray(node.keywordIds) || new Set(authored).size !== authored.length || authored.some((id) => !keywordIds.has(id)) || authored.join('|') !== expected.join('|')) push(out, `专精树 ${treeId} / ${nodeId}: keywordIds 必须与节点运行时关键词精确一致`, 'basic', treeId, nodeId);
    }
    for (const localeTag of ['zh-CN', 'en-US']) {
      const locale = isRecord(i18n[localeTag]) ? i18n[localeTag] : {};
      if (isRecord(i18n[localeTag])) rejectUnknownKeys(out, locale, ['name', 'desc', 'schools', 'nodes', 'keywords'], `专精树 ${treeId}.i18n.${localeTag}`, 'basic', treeId);
      if (!isRecord(i18n[localeTag])) push(out, `专精树 ${treeId}: i18n.${localeTag} 必须是对象`, 'basic', treeId);
      if (!asString(locale.name).trim() || !asString(locale.desc).trim()) push(out, `专精树 ${treeId}: i18n.${localeTag} name/desc 不能为空`, 'basic', treeId);
      const localeSchools = isRecord(locale.schools) ? locale.schools : {};
      const localeNodes = isRecord(locale.nodes) ? locale.nodes : {};
      const localeKeywords = isRecord(locale.keywords) ? locale.keywords : {};
      if (Object.keys(localeSchools).length !== schoolIds.size) push(out, `专精树 ${treeId}: i18n.${localeTag}.schools 必须覆盖五派`, 'basic', treeId);
      for (const id of schoolIds) {
        const entry = isRecord(localeSchools[id]) ? localeSchools[id] : {};
        if (isRecord(localeSchools[id])) rejectUnknownKeys(out, entry, ['name', 'desc'], `专精树 ${treeId}.i18n.${localeTag}.schools.${id}`, 'basic', treeId);
        if (!asString(entry.name).trim() || !asString(entry.desc).trim()) push(out, `专精树 ${treeId}: i18n.${localeTag}.schools.${id} 缺少 name/desc`, 'basic', treeId);
      }
      if (Object.keys(localeKeywords).length !== keywordIds.size) push(out, `专精树 ${treeId}: i18n.${localeTag}.keywords 必须覆盖全部关键词`, 'basic', treeId);
      for (const id of keywordIds) {
        const entry = isRecord(localeKeywords[id]) ? localeKeywords[id] : {};
        if (isRecord(localeKeywords[id])) rejectUnknownKeys(out, entry, ['name', 'playerSummary', 'mechanicDetails', 'sourceHint'], `专精树 ${treeId}.i18n.${localeTag}.keywords.${id}`, 'basic', treeId);
        if (!asString(entry.name).trim() || !asString(entry.playerSummary).trim() || !asString(entry.mechanicDetails).trim() || !asString(entry.sourceHint).trim()) push(out, `专精树 ${treeId}: i18n.${localeTag}.keywords.${id} 四项文案不能为空`, 'basic', treeId);
      }
      if (Object.keys(localeNodes).length !== nodeIds.size) push(out, `专精树 ${treeId}: i18n.${localeTag}.nodes 必须覆盖全部节点`, 'basic', treeId);
      for (const id of nodeIds) {
        const entry = isRecord(localeNodes[id]) ? localeNodes[id] : {};
        const node = rawNodes.find((candidate) => asString(candidate.id) === id);
        const declaredKeywordIds = new Set(asArray(node?.keywordIds).map((value) => asString(value).trim()).filter(Boolean));
        if (isRecord(localeNodes[id])) rejectUnknownKeys(out, entry, ['name', 'desc', 'effectSummary', 'levelEffectSummary'], `专精树 ${treeId}.i18n.${localeTag}.nodes.${id}`, 'basic', treeId, id);
        if (!asString(entry.name).trim() || !asString(entry.desc).trim()) push(out, `专精树 ${treeId}: i18n.${localeTag}.nodes.${id} 缺少 name/desc`, 'basic', treeId, id);
        const summary = asArray(entry.effectSummary);
        if (!Array.isArray(entry.effectSummary) || summary.length < 1 || summary.length > 7) {
          push(out, `专精树 ${treeId}: i18n.${localeTag}.nodes.${id}.effectSummary 必须包含 1-7 行`, 'basic', treeId, id);
        }
        summary.forEach((rawLine, lineIndex) => {
          validatePlayerCopy(out, rawLine, `专精树 ${treeId}: i18n.${localeTag}.nodes.${id}.effectSummary[${lineIndex}]`, treeId, id, keywordIds, declaredKeywordIds);
        });
        const levelSummaries = asArray(entry.levelEffectSummary);
        if (!Array.isArray(entry.levelEffectSummary) || levelSummaries.length !== nodeMaxRanks.get(id)) {
          push(out, `专精树 ${treeId}: i18n.${localeTag}.nodes.${id}.levelEffectSummary 必须逐级覆盖`, 'levels', treeId, id);
        }
        levelSummaries.forEach((rawLines, levelIndex) => {
          const lines = asArray(rawLines);
          if (!Array.isArray(rawLines) || lines.length < 1 || lines.length > 4) push(out, `专精树 ${treeId}: i18n.${localeTag}.nodes.${id}.levelEffectSummary[${levelIndex}] 必须包含 1-4 行`, 'levels', treeId, id);
          lines.forEach((rawLine, lineIndex) => {
            validatePlayerCopy(out, rawLine, `专精树 ${treeId}: i18n.${localeTag}.nodes.${id}.levelEffectSummary[${levelIndex}][${lineIndex}]`, treeId, id, keywordIds, declaredKeywordIds);
          });
          const rankCopy = lines.map((line) => asString(line)).join('\n');
          for (const keywordId of declaredKeywordIds) if (!rankCopy.includes(`[[keyword:${keywordId}]]`)) push(out, `专精树 ${treeId}: i18n.${localeTag}.nodes.${id}.levelEffectSummary[${levelIndex}] 必须标记关键词 ${keywordId}`, 'levels', treeId, id);
        });
      }
    }
    const graph = new Map<string, string[]>();
    const grantOwners = new Map<string, string>();
    const nodesBySchool = new Map<string, { root: DataRow[]; technique: DataRow[]; terminal: DataRow[] }>(SPECIALIZATION_SCHOOLS.map((id) => [id, { root: [], technique: [], terminal: [] }]));
    const confluencePairs = new Map<string, DataRow[]>();
    const exclusiveGroups = new Map<string, DataRow[]>();
    const independentCapacity: Array<Record<string, number>> = [];
    const groupedCapacity = new Map<string, Array<Record<string, number>>>();
    let totalCost = 0;
    rawNodes.forEach((node, nodeIndex) => {
      const nodeId = asString(node.id).trim() || `#${nodeIndex + 1}`;
      const kind = asString(node.kind);
      rejectUnknownKeys(out, node, ['id', 'kind', 'schoolIds', 'nameKey', 'descKey', 'keywordIds', 'exclusiveGroupId', 'requirements', 'levels', 'ui'], `专精树 ${treeId} / ${nodeId}`, 'basic', treeId, nodeId);
      if (!SPECIALIZATION_NODE_KINDS.includes(kind as typeof SPECIALIZATION_NODE_KINDS[number])) push(out, `专精树 ${treeId} / ${nodeId}: kind 非法 ${kind}`, 'basic', treeId, nodeId);
      const schools = asArray(node.schoolIds).map((value) => asString(value));
      const expectedSchoolCount = kind === 'confluence' ? 2 : 1;
      if (schools.length !== expectedSchoolCount || new Set(schools).size !== schools.length || schools.some((id) => !schoolIds.has(id))) push(out, `专精树 ${treeId} / ${nodeId}: schoolIds 数量或引用非法`, 'basic', treeId, nodeId);
      if (node.nameKey !== `specialization.node.${nodeId}.name` || node.descKey !== `specialization.node.${nodeId}.desc`) push(out, `专精树 ${treeId} / ${nodeId}: nameKey/descKey 非法`, 'basic', treeId, nodeId);
      const ui = isRecord(node.ui) ? node.ui : {};
      if (isRecord(node.ui)) rejectUnknownKeys(out, node.ui, ['column', 'row', 'branch', 'accent', 'x', 'y'], `专精树 ${treeId} / ${nodeId}.ui`, 'basic', treeId, nodeId);
      if (!Number.isInteger(ui.column) || !Number.isInteger(ui.row) || Number(ui.column) < 0 || Number(ui.row) < 0) push(out, `专精树 ${treeId} / ${nodeId}: ui.column/row 必须是非负整数`, 'basic', treeId, nodeId);
      if (!finiteNumber(ui.x) || !finiteNumber(ui.y) || Math.abs(Number(ui.x)) > 1 || Math.abs(Number(ui.y)) > 1) push(out, `专精树 ${treeId} / ${nodeId}: ui.x/y 必须是 [-1, 1] 内的归一化坐标`, 'basic', treeId, nodeId);
      const position = `${ui.column}:${ui.row}`;
      const radial = `${ui.x}:${ui.y}`;
      if (uiPositions.has(position)) push(out, `专精树 ${treeId} / ${nodeId}: ui.column/row 与其他节点重复`, 'basic', treeId, nodeId);
      if (radialPositions.has(radial)) push(out, `专精树 ${treeId} / ${nodeId}: ui.x/y 与其他节点重复`, 'basic', treeId, nodeId);
      uiPositions.add(position);
      radialPositions.add(radial);
      if (typeof ui.accent !== 'string' || !/^#[0-9a-f]{6}$/i.test(ui.accent)) push(out, `专精树 ${treeId} / ${nodeId}: ui.accent 必须是六位十六进制颜色`, 'basic', treeId, nodeId);
      const reqs = normalizeReqs(node.requirements);
      if (isRecord(node.requirements)) rejectUnknownKeys(out, node.requirements, ['all', 'anyOf'], `专精树 ${treeId} / ${nodeId}.requirements`, 'requirements', treeId, nodeId);
      if (!isRecord(node.requirements) || !Array.isArray(node.requirements.all) || !Array.isArray(node.requirements.anyOf)) push(out, `专精树 ${treeId} / ${nodeId}: requirements.all/anyOf 必须是数组`, 'requirements', treeId, nodeId);
      if (isRecord(node.requirements) && Array.isArray(node.requirements.all) && node.requirements.all.length !== reqs.all.length) push(out, `专精树 ${treeId} / ${nodeId}: requirements.all 必须只包含对象`, 'requirements', treeId, nodeId);
      if (isRecord(node.requirements) && Array.isArray(node.requirements.anyOf) && node.requirements.anyOf.some((group) => !Array.isArray(group))) push(out, `专精树 ${treeId} / ${nodeId}: requirements.anyOf 必须只包含数组`, 'requirements', treeId, nodeId);
      const dependencies: string[] = [];
      reqs.all.forEach((req, i) => validateRequirement(out, req, `专精树 ${treeId} / ${nodeId}: all #${i + 1}`, treeId, nodeId, nodeIds, dependencies, attributeIds));
      reqs.anyOf.forEach((group, g) => {
        if (group.length === 0) push(out, `专精树 ${treeId} / ${nodeId}: anyOf ${g + 1} 不能为空`, 'requirements', treeId, nodeId);
        group.forEach((req, i) => validateRequirement(out, req, `专精树 ${treeId} / ${nodeId}: anyOf ${g + 1} #${i + 1}`, treeId, nodeId, nodeIds, dependencies, attributeIds));
      });
      const pureSchoolGates = [...reqs.all, ...reqs.anyOf.flat()].filter((req) => req.kind === 'pure_school_points_gte');
      if (kind === 'terminal') {
        const gate = pureSchoolGates[0];
        if (pureSchoolGates.length !== 1 || !reqs.all.includes(gate) || asString(gate?.schoolId) !== schools[0] || Number(gate?.points) !== 11) push(out, `专精树 ${treeId} / ${nodeId}: 终式必须要求本流派已投入 11 点`, 'requirements', treeId, nodeId);
      } else if (kind === 'confluence') {
        const expectedPoints = nodeId === 'confluence_intelligence_perception_crownless_astrolabe' ? 11 : 9;
        const gatesBySchool = new Map(pureSchoolGates.map((gate) => [asString(gate.schoolId), Number(gate.points)]));
        if (pureSchoolGates.length !== 2 || pureSchoolGates.some((gate) => !reqs.all.includes(gate)) || schools.some((schoolId) => gatesBySchool.get(schoolId) !== expectedPoints)) push(out, `专精树 ${treeId} / ${nodeId}: 交汇必须分别要求两流派已投入 ${expectedPoints} 点`, 'requirements', treeId, nodeId);
      } else if (pureSchoolGates.length > 0) push(out, `专精树 ${treeId} / ${nodeId}: 纯流派投入门槛只允许用于终式与交汇`, 'requirements', treeId, nodeId);
      graph.set(nodeId, dependencies);
      const levels = asArray(node.levels);
      const expectedCosts = specializationExpectedCosts(nodeId, kind);
      if (levels.length !== expectedCosts.length || levels.some((level, index) => !isRecord(level) || level.level !== index + 1)) push(out, `专精树 ${treeId} / ${nodeId}: levels 必须与 v4 层级和等级数一致`, 'levels', treeId, nodeId);
      levels.forEach((rawLevel, levelIndex) => {
        if (!isRecord(rawLevel)) return push(out, `专精树 ${treeId} / ${nodeId}: level #${levelIndex + 1} 非法`, 'levels', treeId, nodeId);
        rejectUnknownKeys(out, rawLevel, ['level', 'cost', 'grants'], `专精树 ${treeId} / ${nodeId}.levels[${levelIndex}]`, 'levels', treeId, nodeId);
        if (rawLevel.cost !== expectedCosts[levelIndex]) push(out, `专精树 ${treeId} / ${nodeId}: Lv.${levelIndex + 1} cost 必须是 ${expectedCosts[levelIndex] ?? '?'}`, 'levels', treeId, nodeId);
        if (!Number.isInteger(rawLevel.cost) || Number(rawLevel.cost) < 1 || Number(rawLevel.cost) > 24) push(out, `专精树 ${treeId} / ${nodeId}: Lv.${levelIndex + 1} cost 必须是 1~24 的整数`, 'levels', treeId, nodeId);
        if (!Array.isArray(rawLevel.grants) || rawLevel.grants.length === 0) push(out, `专精树 ${treeId} / ${nodeId}: grants 必须是非空数组`, 'grants', treeId, nodeId);
        if (Array.isArray(rawLevel.grants) && rawLevel.grants.some((grant) => !isRecord(grant))) push(out, `专精树 ${treeId} / ${nodeId}: grants 必须只包含对象`, 'grants', treeId, nodeId);
        const rankGrantIds = new Set<string>();
        asArray(rawLevel.grants).filter(isRecord).forEach((grant, grantIndex) => {
          const kind = asString(grant.kind);
          const tab = kind === 'equipment_slot_capacity' ? 'topology' : 'grants';
          const label = `专精树 ${treeId} / ${nodeId}: Lv.${levelIndex + 1} grant #${grantIndex + 1}`;
          const grantId = asString(grant.id).trim();
          if (!grantId || rankGrantIds.has(grantId)) push(out, `${label} id 不能为空且同级唯一 ${grantId}`, tab, treeId, nodeId);
          if (grantId && !SPECIALIZATION_PAYLOAD_ID_PATTERN.test(grantId)) push(out, `${label} id 必须使用小写字母、数字和下划线，且以字母开头`, tab, treeId, nodeId);
          rankGrantIds.add(grantId);
          const previousOwner = grantOwners.get(grantId);
          if (grantId && previousOwner && previousOwner !== nodeId) push(out, `${label} id 已属于其他节点 ${grantId}`, tab, treeId, nodeId);
          if (grantId) grantOwners.set(grantId, nodeId);
          if (!SPECIALIZATION_GRANT_KINDS.includes(kind as SpecializationGrantKind)) push(out, `${label} kind 非法 ${kind}`, tab, treeId, nodeId);
          if (kind === 'stat_modifier') {
            rejectUnknownKeys(out, grant, ['id', 'kind', 'scope', 'modifiers'], label, tab, treeId, nodeId);
            if (!['character', 'battle'].includes(asString(grant.scope)) || !Array.isArray(grant.modifiers) || grant.modifiers.length === 0) push(out, `${label} 需要 character/battle scope 与非空 modifiers[]`, tab, treeId, nodeId);
            if (Array.isArray(grant.modifiers) && grant.modifiers.some((modifier) => !isRecord(modifier))) push(out, `${label} modifiers 必须只包含对象`, tab, treeId, nodeId);
            asArray(grant.modifiers).filter(isRecord).forEach((modifier, index) => {
              rejectUnknownKeys(out, modifier, ['attributeKey', 'operation', 'value'], `${label}.modifiers[${index}]`, tab, treeId, nodeId);
              const attr = asString(modifier.attributeKey).trim();
              if (!attr || (attributeIds.size && !attributeIds.has(attr))) push(out, `${label} modifier #${index + 1} attributeKey 未定义 ${attr}`, tab, treeId, nodeId);
              if (!['add', 'multiply'].includes(asString(modifier.operation))) push(out, `${label} modifier #${index + 1} operation 仅允许 add/multiply`, tab, treeId, nodeId);
              if (!finiteNumber(modifier.value) || modifier.value === 0) push(out, `${label} modifier #${index + 1} value 必须是非零有限数`, tab, treeId, nodeId);
            });
          }
          if (kind === 'battle_hook') {
            rejectUnknownKeys(out, grant, ['id', 'kind', 'battleHooks'], label, tab, treeId, nodeId);
            const hooks = asArray(grant.battleHooks);
            if (hooks.length === 0) push(out, `${label} battleHooks 必须是非空数组`, tab, treeId, nodeId);
            if (!Array.isArray(grant.battleHooks) || grant.battleHooks.some((hook) => !isRecord(hook))) push(out, `${label} battleHooks 必须只包含对象`, tab, treeId, nodeId);
            const hookIds = new Set<string>();
            hooks.filter(isRecord).forEach((hook, hookIndex) => {
              const hookLabel = `${label} battleHook #${hookIndex + 1}`;
              const hookId = asString(hook.id).trim();
              if (!hookId || hookIds.has(hookId)) push(out, `${hookLabel} id 空或重复`, tab, treeId, nodeId);
              if (hookId && !SPECIALIZATION_PAYLOAD_ID_PATTERN.test(hookId)) push(out, `${hookLabel} id 必须使用小写字母、数字和下划线，且以字母开头`, tab, treeId, nodeId);
              hookIds.add(hookId);
              if (!asString(hook.trigger).trim()) push(out, `${hookLabel} trigger 不能为空`, tab, treeId, nodeId);
              if (!finiteNumber(hook.priority)) push(out, `${hookLabel} priority 必须是有限数`, tab, treeId, nodeId);
              if (!Array.isArray(hook.conditions)) push(out, `${hookLabel} conditions 必须是数组`, tab, treeId, nodeId);
              asArray(hook.conditions).forEach((condition, index) => { if (!isRecord(condition) || !asString(condition.kind || condition.type).trim()) push(out, `${hookLabel} condition #${index + 1} 缺少 kind`, tab, treeId, nodeId); });
              if (!Array.isArray(hook.effects) || hook.effects.length === 0) push(out, `${hookLabel} effects 必须是非空数组`, tab, treeId, nodeId);
              asArray(hook.effects).forEach((effect, index) => { if (!isRecord(effect) || !asString(effect.kind || effect.type).trim()) push(out, `${hookLabel} effect #${index + 1} 缺少 kind`, tab, treeId, nodeId); });
              validateBattleHookDsl(out, hook, hookLabel, treeId, nodeId, { attributeIds, skillIds, skillCategories, weaponClasses, buffIds, buffKinds });
            });
            for (const problem of battleHookCounterCapacityProblems(hooks)) push(out, `${label} ${problem}`, tab, treeId, nodeId);
          }
          if (kind === 'effect_mutation' && asArray(grant.effectMutations).length === 0) push(out, `${label} effectMutations 必须是非空数组`, tab, treeId, nodeId);
          if (kind === 'effect_mutation') {
            rejectUnknownKeys(out, grant, ['id', 'kind', 'effectMutations'], label, tab, treeId, nodeId);
            if (!Array.isArray(grant.effectMutations) || grant.effectMutations.some((mutation) => !isRecord(mutation))) push(out, `${label} effectMutations 必须只包含对象`, tab, treeId, nodeId);
            const mutationIds = new Set<string>();
            for (const [mutationIndex, mutation] of asArray(grant.effectMutations).filter(isRecord).entries()) {
              const selector = isRecord(mutation.effectSelector) ? mutation.effectSelector : {};
              const mode = asString(selector.mode);
              const selectorLabel = `${label} effectMutations #${mutationIndex + 1}`;
              const mutationId = asString(mutation.id).trim();
              if (!mutationId || mutationIds.has(mutationId)) push(out, `${selectorLabel} id 空或重复`, tab, treeId, nodeId);
              if (mutationId && !SPECIALIZATION_PAYLOAD_ID_PATTERN.test(mutationId)) push(out, `${selectorLabel} id 必须使用小写字母、数字和下划线，且以字母开头`, tab, treeId, nodeId);
              mutationIds.add(mutationId);
              if (!['skill', 'basic_attack', 'both'].includes(asString(mutation.scope))) push(out, `${selectorLabel} scope 非法`, tab, treeId, nodeId);
              if (mutation.skillIds !== undefined && !nonEmptyStrings(mutation.skillIds)) push(out, `${selectorLabel} skillIds 必须是非空字符串数组`, tab, treeId, nodeId);
              if (!['all', 'kind', 'effect_id'].includes(mode)) push(out, `${selectorLabel} effectSelector.mode 仅允许 all / kind / effect_id`, tab, treeId, nodeId);
              if (mode === 'kind' && (!nonEmptyStrings(selector.kinds) || asArray(selector.kinds).length === 0)) push(out, `${selectorLabel} kind 选择器需要 kinds[]`, tab, treeId, nodeId);
              if (mode === 'effect_id' && (!nonEmptyStrings(selector.effectIds) || asArray(selector.effectIds).length === 0)) push(out, `${selectorLabel} effect_id 选择器需要 effectIds[]`, tab, treeId, nodeId);
              if (mode === 'index' || 'indexes' in selector || 'index' in selector || 'effectIndex' in selector) push(out, `${selectorLabel} 禁止使用下标选择器`, tab, treeId, nodeId);
              const ops = isRecord(mutation.ops) ? mutation.ops : {};
              if (!isRecord(mutation.ops) || Object.keys(ops).length === 0) push(out, `${selectorLabel} ops 必须是非空对象`, tab, treeId, nodeId);
              for (const key of ['targetCountDelta', 'amountBaseAddDelta', 'cooldownDelta', 'cooldownFloor', 'mpCostDelta', 'mpCostMultiplier', 'hitCountDelta']) if (ops[key] !== undefined && !finiteNumber(ops[key])) push(out, `${selectorLabel} ops.${key} 必须是有限数`, tab, treeId, nodeId);
              if (ops.amountMultiplier !== undefined && (!finiteNumber(ops.amountMultiplier) || ops.amountMultiplier <= 0)) push(out, `${selectorLabel} ops.amountMultiplier 必须是正有限数`, tab, treeId, nodeId);
              for (const key of ['amountTermsAppend', 'statusIdsAppend', 'statusIdsRemove', 'appendEffects']) if (ops[key] !== undefined && !Array.isArray(ops[key])) push(out, `${selectorLabel} ops.${key} 必须是数组`, tab, treeId, nodeId);
              validateMutationDsl(out, mutation, selectorLabel, treeId, nodeId, { attributeIds, skillIds, skillCategories, weaponClasses, buffIds, skillEffectKinds, effectIds });
            }
          }
          if (kind === 'equipment_slot_capacity') {
            rejectUnknownKeys(out, grant, ['id', 'kind', 'equipType', 'deltas'], label, tab, treeId, nodeId);
            if (asString(grant.equipType) !== 'battle') push(out, `${label} equipType 必须是 battle`, tab, treeId, nodeId);
            const deltas = isRecord(grant.deltas) ? grant.deltas : {};
            if (!isRecord(grant.deltas) || Object.keys(deltas).length === 0) push(out, `${label} deltas 必须是非空对象`, tab, treeId, nodeId);
            let positive = false;
            let negative = false;
            for (const [family, delta] of Object.entries(deltas)) {
              if (!SPECIALIZATION_SLOT_FAMILIES.includes(family as typeof SPECIALIZATION_SLOT_FAMILIES[number])) push(out, `${label} 未知槽位家族 ${family}`, tab, treeId, nodeId);
              if (!Number.isInteger(delta) || delta === 0) push(out, `${label} deltas.${family} 必须是非零整数`, tab, treeId, nodeId);
              positive ||= Number(delta) > 0;
              negative ||= Number(delta) < 0;
            }
            if (!positive || !negative) push(out, `${label} deltas 必须同时包含牺牲槽与换入槽`, tab, treeId, nodeId);
            if (levelIndex !== levels.length - 1) push(out, `${label} 装备槽拓扑只能在节点最终等级引入`, tab, treeId, nodeId);
          }
        });
      });
      const levelRows = levels.filter(isRecord);
      for (const [levelIndex, rawLevel] of levelRows.entries()) {
        if (levelIndex === 0) continue;
        const previousGrants = new Map(asArray(levelRows[levelIndex - 1]?.grants).filter(isRecord).map((grant) => [asString(grant.id), grant]));
        const currentGrants = new Map(asArray(rawLevel.grants).filter(isRecord).map((grant) => [asString(grant.id), grant]));
        for (const [grantId, previousGrant] of previousGrants) {
          const tab = previousGrant.kind === 'equipment_slot_capacity' ? 'topology' : 'grants';
          for (const problem of specializationGrantSnapshotRetentionProblems(previousGrant, currentGrants.get(grantId))) {
            push(out, `专精树 ${treeId} / ${nodeId}: Lv.${levelIndex + 1} ${problem}`, tab, treeId, nodeId);
          }
        }
        const previousCore = asArray(levelRows[levelIndex - 1]?.grants).filter(isRecord).filter((grant) => grant.kind !== 'stat_modifier');
        const currentCore = asArray(rawLevel.grants).filter(isRecord).filter((grant) => grant.kind !== 'stat_modifier');
        if (JSON.stringify(canonicalSpecializationCoreSnapshot(previousCore, 'grants')) === JSON.stringify(canonicalSpecializationCoreSnapshot(currentCore, 'grants'))) push(out, `专精树 ${treeId} / ${nodeId}: Lv.${levelIndex + 1} 必须严格增强至少一项非属性核心机制`, 'grants', treeId, nodeId);
      }
      validateNodeRiskPolicy(out, node, `专精树 ${treeId} / ${nodeId}`, treeId, nodeId);
      const level = isRecord(levels[0]) ? levels[0] : {};
      const levelGrants = asArray(level.grants).filter(isRecord);
      if (!levelGrants.some((grant) => grant.kind !== 'stat_modifier')) push(out, `专精树 ${treeId} / ${nodeId}: 必须包含改变玩法的 grant，不能只有属性加减`, 'grants', treeId, nodeId);
      const formulaSchoolIds = collectFormulaAttributeIds(levels);
      const declaredSchoolIds = new Set(schools);
      const hasForeignFormulaSchool = [...formulaSchoolIds].some((schoolId) => !declaredSchoolIds.has(schoolId));
      const missesDeclaredFormulaSchool = formulaSchoolIds.size > 0 && schools.some((schoolId) => !formulaSchoolIds.has(schoolId));
      if (hasForeignFormulaSchool || missesDeclaredFormulaSchool) push(out, `专精树 ${treeId} / ${nodeId}: 使用流派属性公式时必须完整且仅覆盖 schoolIds`, 'grants', treeId, nodeId);
      const topologyNode = levelRows.some((row) => asArray(row.grants).filter(isRecord).some((grant) => grant.kind === 'equipment_slot_capacity'));
      const expectedTopologyNode = Object.prototype.hasOwnProperty.call(SPECIALIZATION_TOPOLOGY_COSTS, nodeId);
      if (topologyNode !== expectedTopologyNode) push(out, `专精树 ${treeId} / ${nodeId}: 装备槽拓扑只能位于获批的后段高成本节点`, 'topology', treeId, nodeId);
      totalCost += levelRows.reduce((sum, row) => sum + (finiteNumber(row.cost) ? row.cost : 0), 0);
      if (kind === 'confluence') {
        const pair = [...schools].sort().join('+');
        const pairNodes = confluencePairs.get(pair) ?? [];
        pairNodes.push(node);
        confluencePairs.set(pair, pairNodes);
      } else if (nodesBySchool.has(schools[0]) && ['root', 'technique', 'terminal'].includes(kind)) {
        nodesBySchool.get(schools[0])![kind as 'root' | 'technique' | 'terminal'].push(node);
      }
      const exclusive = asString(node.exclusiveGroupId).trim();
      if (exclusive) {
        const group = exclusiveGroups.get(exclusive) ?? [];
        group.push(node);
        exclusiveGroups.set(exclusive, group);
      }
      const deltas: Record<string, number> = {};
      for (const grant of levelRows.flatMap((row) => asArray(row.grants).filter(isRecord))) if (grant.kind === 'equipment_slot_capacity' && isRecord(grant.deltas)) for (const [family, delta] of Object.entries(grant.deltas)) deltas[family] = (deltas[family] ?? 0) + Number(delta);
      if (Object.keys(deltas).length > 0) {
        if (exclusive) {
          const group = groupedCapacity.get(exclusive) ?? [];
          group.push(deltas);
          groupedCapacity.set(exclusive, group);
        } else independentCapacity.push(deltas);
      }
    });
    if (confluencePairs.size !== 10 || [...confluencePairs.values()].some((nodes) => nodes.length !== 2)) push(out, `专精树 ${treeId}: 全部 10 组双派组合都必须各有 2 个横向交汇`, 'requirements', treeId);
    for (const [schoolId, branch] of nodesBySchool) {
      if (branch.root.length !== 2 || branch.technique.length !== 4 || branch.terminal.length !== 4) push(out, `专精树 ${treeId}: ${schoolId} 必须包含 2 个根基、4 个技巧和 4 个终式`, 'requirements', treeId);
      const rootIds = new Set(branch.root.map((root) => asString(root.id)));
      const techniquesByRoot = new Map(branch.root.map((root) => [asString(root.id), [] as DataRow[]]));
      for (const root of branch.root) {
        const rootId = asString(root.id);
        if ((graph.get(rootId)?.length ?? 0) !== 0) push(out, `专精树 ${treeId} / ${rootId}: 根基不能有节点前置`, 'requirements', treeId, rootId);
        if (!asArray((asArray(root.levels).find(isRecord) ?? {}).grants).filter(isRecord).some((grant) => grant.kind === 'battle_hook' || grant.kind === 'effect_mutation')) push(out, `专精树 ${treeId} / ${rootId}: 根基必须定义战斗循环`, 'grants', treeId, rootId);
      }
      for (const technique of branch.technique) {
        const techniqueId = asString(technique.id);
        const roots = (graph.get(techniqueId) ?? []).filter((id) => rootIds.has(id));
        const reqs = normalizeReqs(technique.requirements);
        const rootRequirement = reqs.all.find((requirement) => requirement.kind === 'node_rank' && asString(requirement.nodeId) === roots[0]);
        if (roots.length !== 1 || !rootRequirement || rootRequirement.minRank !== 6 || reqs.anyOf.length !== 0) push(out, `专精树 ${treeId} / ${techniqueId}: 技巧必须要求一条根基达到 6 级`, 'requirements', treeId, techniqueId);
        else techniquesByRoot.get(roots[0])?.push(technique);
      }
      if ([...techniquesByRoot.values()].some((techniques) => techniques.length !== 2)) push(out, `专精树 ${treeId}: ${schoolId} 每条横向根基必须拥有 2 个技巧`, 'requirements', treeId);
      const terminalsByRoot = new Map(branch.root.map((root) => [asString(root.id), [] as DataRow[]]));
      for (const terminal of branch.terminal) {
        const terminalId = asString(terminal.id);
        const dependencies = graph.get(terminalId) ?? [];
        const reqs = normalizeReqs(terminal.requirements);
        const directTechniques = branch.technique.filter((technique) => dependencies.includes(asString(technique.id)));
        const allNodeRequirements = reqs.all.filter((requirement) => requirement.kind === 'node_rank');
        const nodeChoiceGroups = reqs.anyOf.filter((group) => group.some((requirement) => requirement.kind === 'node_rank'));
        const singleRoute = allNodeRequirements.length === 1 && reqs.anyOf.length === 0;
        const choiceRoute = allNodeRequirements.length === 0
          && reqs.anyOf.length === 1
          && nodeChoiceGroups.length === 1
          && nodeChoiceGroups[0].length === 2
          && nodeChoiceGroups[0].every((requirement) => requirement.kind === 'node_rank');
        const rankedRequirements = [...allNodeRequirements, ...nodeChoiceGroups.flat()];
        const parentRootIds = new Set(directTechniques.flatMap((technique) => branch.root
          .filter((root) => graph.get(asString(technique.id))?.includes(asString(root.id)))
          .map((root) => asString(root.id))));
        if ((!singleRoute && !choiceRoute)
          || directTechniques.length !== dependencies.length
          || directTechniques.length < 1
          || directTechniques.length > 2
          || rankedRequirements.some((requirement) => requirement.kind !== 'node_rank' || requirement.minRank !== 4)
          || parentRootIds.size !== 1
          || dependencies.some((id) => rootIds.has(id))) {
          push(out, `专精树 ${treeId} / ${terminalId}: 终式必须声明一个直接技巧，或同一根基下的两个技巧任选其一`, 'requirements', treeId, terminalId);
        } else terminalsByRoot.get([...parentRootIds][0])?.push(terminal);
      }
      if ([...terminalsByRoot.values()].some((terminals) => terminals.length !== 2)) push(out, `专精树 ${treeId}: ${schoolId} 每条横向根基必须拥有 2 个终式`, 'requirements', treeId);
    }
    for (const node of rawNodes.filter((entry) => entry.kind === 'confluence')) {
      const reqs = normalizeReqs(node.requirements);
      const requiredRank = asString(node.id) === 'confluence_intelligence_perception_crownless_astrolabe' ? 4 : 2;
      if (reqs.all.some((req) => req.kind === 'node_rank')) push(out, `专精树 ${treeId} / ${asString(node.id)}: 交汇不得重复声明根基或其他节点前置`, 'requirements', treeId, asString(node.id));
      for (const schoolId of asArray(node.schoolIds).map((value) => asString(value))) {
        const branch = nodesBySchool.get(schoolId);
        const hasChoice = reqs.anyOf.some((group) => {
          const ids = group.filter((req) => req.kind === 'node_rank').map((req) => asString(req.nodeId));
          const nodeRequirements = group.filter((req) => req.kind === 'node_rank');
          if (ids.length !== 2 || nodeRequirements.some((requirement) => requirement.minRank !== requiredRank) || ids.some((id) => !branch?.technique.some((technique) => asString(technique.id) === id))) return false;
          const parentRoots = new Set(ids.flatMap((id) => branch?.root.filter((root) => graph.get(id)?.includes(asString(root.id))).map((root) => asString(root.id)) ?? []));
          return parentRoots.size === 1;
        });
        if (!hasChoice) push(out, `专精树 ${treeId} / ${asString(node.id)}: 交汇必须为 ${schoolId} 提供同一根基下的直接技巧二选一`, 'requirements', treeId, asString(node.id));
      }
    }
    for (const [groupId, group] of exclusiveGroups) if (group.length !== 2 || group.some((node) => node.kind !== 'terminal') || new Set(group.map((node) => asString(asArray(node.schoolIds)[0]))).size !== 1) push(out, `专精树 ${treeId}: 互斥组 ${groupId} 必须是同派两个终式`, 'requirements', treeId);
    if (exclusiveGroups.size !== 10 || rawNodes.some((node) => node.kind === 'terminal' && !asString(node.exclusiveGroupId).trim())) push(out, `专精树 ${treeId}: 每派必须有两组双终式互斥选择`, 'requirements', treeId);
    const visited = new Set<string>();
    const visiting = new Set<string>();
    const visit = (id: string): boolean => {
      if (visiting.has(id)) return true;
      if (visited.has(id)) return false;
      visiting.add(id);
      for (const dep of graph.get(id) ?? []) if (nodeIds.has(dep) && visit(dep)) return true;
      visiting.delete(id);
      visited.add(id);
      return false;
    };
    for (const id of nodeIds) if (visit(id)) { push(out, `专精树 ${treeId}: 前置存在环（${id}）`, 'requirements', treeId, id); break; }
    const reaches = (fromId: string, targetId: string, seen = new Set<string>()): boolean => {
      if (fromId === targetId) return true;
      if (seen.has(fromId)) return false;
      seen.add(fromId);
      return (graph.get(fromId) ?? []).some((dependencyId) => reaches(dependencyId, targetId, seen));
    };
    for (const [nodeId, direct] of graph) {
      for (const dependencyId of direct) {
        if (direct.some((otherId) => otherId !== dependencyId && reaches(otherId, dependencyId))) push(out, `专精树 ${treeId} / ${nodeId}: 存在可由其他前置传递到达的冗余边 ${dependencyId}`, 'requirements', treeId, nodeId);
      }
    }
    if (Number(rawTree.maxPoints) >= totalCost) push(out, `专精树 ${treeId}: maxPoints 必须小于全节点总成本，以保留构筑取舍`, 'basic', treeId);
    for (const [family, rule] of Object.entries(capacityRules)) {
      let minDelta = 0;
      let maxDelta = 0;
      for (const deltas of independentCapacity) { minDelta += Math.min(0, deltas[family] ?? 0); maxDelta += Math.max(0, deltas[family] ?? 0); }
      for (const group of groupedCapacity.values()) {
        const values = group.map((deltas) => deltas[family] ?? 0);
        minDelta += Math.min(0, ...values);
        maxDelta += Math.max(0, ...values);
      }
      if (rule.base + minDelta < rule.min || rule.base + maxDelta > rule.max) push(out, `专精树 ${treeId}: ${family} 槽位容量边界超限 [${rule.base + minDelta}, ${rule.base + maxDelta}]`, 'topology', treeId);
    }
  });
  validateSpecializationMutationSemantics(out, rows, refs);
  return out;
}
