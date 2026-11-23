import type { EnemyDifficultyAuthoringGuardrails } from './types';

type EnemyStatRow = Record<string, unknown>;

export type EnemyDefensiveProjection = {
  physicsDefence: number;
  magicalDefence: number;
  critDamageResist: number;
};

export const DEFAULT_ENEMY_DIFFICULTY_AUTHORING_GUARDRAILS: EnemyDifficultyAuthoringGuardrails = {
  minEnemyLevel: 40,
  defenseRetainRatio: 0.12,
  defenseBaseCapsByEnemyType: {
    normal: 20,
    elite: 25,
    boss: 30,
    world_boss: 35
  },
  critDamageResistRetainRatio: 0.25,
  critDamageResistBaseFloorsByEnemyType: {
    normal: 0,
    elite: 0,
    boss: 0.12,
    world_boss: 0.14
  },
  critDamageResistBaseCapsByEnemyType: {
    normal: 0.116667,
    elite: 0.125,
    boss: 0.133333,
    world_boss: 0.141667
  }
};

function finiteNumber(raw: unknown, fallback = 0): number {
  const parsed = Number(raw);
  return Number.isFinite(parsed) ? parsed : fallback;
}

function finiteRatio(raw: unknown, fallback: number): number {
  return Math.max(0, Math.min(1, finiteNumber(raw, fallback)));
}

function numberMap(raw: unknown, fallback: Record<string, number>): Record<string, number> {
  if (!raw || typeof raw !== 'object' || Array.isArray(raw)) return { ...fallback };
  const out = { ...fallback };
  for (const [rawKey, rawValue] of Object.entries(raw)) {
    const key = rawKey.trim();
    const value = finiteNumber(rawValue, Number.NaN);
    if (!key || !Number.isFinite(value) || value < 0) continue;
    out[key] = value;
  }
  return out;
}

function ratioMap(raw: unknown, fallback: Record<string, number>): Record<string, number> {
  const out = numberMap(raw, fallback);
  for (const key of Object.keys(out)) out[key] = Math.min(1, out[key]);
  return out;
}

function roundGuardrailValue(value: number): number {
  return Math.round(value * 1_000_000) / 1_000_000;
}

function enemyType(row: EnemyStatRow): string {
  return String(row.type || 'normal').trim() || 'normal';
}

function capForEnemyType(caps: Record<string, number>, type: string): number {
  const direct = finiteNumber(caps[type], Number.NaN);
  if (Number.isFinite(direct) && direct >= 0) return direct;
  return Math.max(0, finiteNumber(caps.normal, 0));
}

export function parseEnemyDifficultyAuthoringGuardrails(raw: unknown): EnemyDifficultyAuthoringGuardrails {
  const source = raw && typeof raw === 'object' && !Array.isArray(raw)
    ? raw as Record<string, unknown>
    : {};
  const defaults = DEFAULT_ENEMY_DIFFICULTY_AUTHORING_GUARDRAILS;
  const critFloors = ratioMap(
    source.critDamageResistBaseFloorsByEnemyType,
    defaults.critDamageResistBaseFloorsByEnemyType
  );
  const critCaps = ratioMap(
    source.critDamageResistBaseCapsByEnemyType,
    defaults.critDamageResistBaseCapsByEnemyType
  );
  for (const [type, floor] of Object.entries(critFloors)) {
    if (Object.prototype.hasOwnProperty.call(critCaps, type)) critCaps[type] = Math.max(floor, critCaps[type]);
  }
  return {
    minEnemyLevel: Math.max(1, Math.trunc(finiteNumber(source.minEnemyLevel, defaults.minEnemyLevel))),
    defenseRetainRatio: finiteRatio(source.defenseRetainRatio, defaults.defenseRetainRatio),
    defenseBaseCapsByEnemyType: numberMap(
      source.defenseBaseCapsByEnemyType,
      defaults.defenseBaseCapsByEnemyType
    ),
    critDamageResistRetainRatio: finiteRatio(
      source.critDamageResistRetainRatio,
      defaults.critDamageResistRetainRatio
    ),
    critDamageResistBaseFloorsByEnemyType: critFloors,
    critDamageResistBaseCapsByEnemyType: critCaps
  };
}

export function projectEnemyDefensiveStats(row: EnemyStatRow): EnemyDefensiveProjection {
  const strength = finiteNumber(row.strength);
  const stamina = finiteNumber(row.stamina);
  const perception = finiteNumber(row.perception);
  return {
    physicsDefence: finiteNumber(row.physics_defence) + stamina * 0.45 + strength * 0.15,
    magicalDefence: finiteNumber(row.magical_defence) + stamina * 0.35 + perception * 0.5,
    critDamageResist: finiteNumber(row.crit_damage_resist) + strength * 0.001 + perception * 0.001 + stamina * 0.002
  };
}

export function buildEnemyDifficultyGuardrailPatch(
  row: EnemyStatRow,
  guardrails: EnemyDifficultyAuthoringGuardrails
): Partial<Record<'physics_defence' | 'magical_defence' | 'crit_damage_resist', number>> {
  const level = Math.trunc(finiteNumber(row.level, 1));
  const strength = finiteNumber(row.strength);
  const stamina = finiteNumber(row.stamina);
  const perception = finiteNumber(row.perception);
  const type = enemyType(row);
  if (!Object.prototype.hasOwnProperty.call(guardrails.defenseBaseCapsByEnemyType, type)
    || !Object.prototype.hasOwnProperty.call(guardrails.critDamageResistBaseFloorsByEnemyType, type)
    || !Object.prototype.hasOwnProperty.call(guardrails.critDamageResistBaseCapsByEnemyType, type)) {
    return {};
  }

  const bossLike = type === 'boss' || type === 'world_boss';
  const defenseGuarded = level >= guardrails.minEnemyLevel || bossLike;
  const critGuarded = level >= guardrails.minEnemyLevel
    || capForEnemyType(guardrails.critDamageResistBaseFloorsByEnemyType, type) > 0;
  if (!defenseGuarded && !critGuarded) return {};

  const derivedPhysicsDefence = stamina * 0.45 + strength * 0.15;
  const derivedMagicalDefence = stamina * 0.35 + perception * 0.5;
  const derivedCritDamageResist = strength * 0.001 + perception * 0.001 + stamina * 0.002;

  const patch: Partial<Record<'physics_defence' | 'magical_defence' | 'crit_damage_resist', number>> = {};
  if (defenseGuarded) {
    const defenseCap = capForEnemyType(guardrails.defenseBaseCapsByEnemyType, type);
    patch.physics_defence = roundGuardrailValue(
      Math.min(derivedPhysicsDefence * guardrails.defenseRetainRatio, defenseCap) - derivedPhysicsDefence
    );
    patch.magical_defence = roundGuardrailValue(
      Math.min(derivedMagicalDefence * guardrails.defenseRetainRatio, defenseCap) - derivedMagicalDefence
    );
  }
  if (critGuarded) {
    const critFloor = capForEnemyType(guardrails.critDamageResistBaseFloorsByEnemyType, type);
    const critCap = Math.max(
      critFloor,
      capForEnemyType(guardrails.critDamageResistBaseCapsByEnemyType, type)
    );
    const targetCritDamageResist = Math.max(
      critFloor,
      Math.min(derivedCritDamageResist * guardrails.critDamageResistRetainRatio, critCap)
    );
    patch.crit_damage_resist = roundGuardrailValue(targetCritDamageResist - derivedCritDamageResist);
  }
  return patch;
}

export function applyEnemyDifficultyGuardrails(
  row: EnemyStatRow,
  guardrails: EnemyDifficultyAuthoringGuardrails
): EnemyStatRow {
  Object.assign(row, buildEnemyDifficultyGuardrailPatch(row, guardrails));
  return row;
}
