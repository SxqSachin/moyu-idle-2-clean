import sharedGlobalConfig from '../../../packages/game-config/source/shared-global-config.json';
import type { AttributeValue, CombatDifficultyLevel, DropPricingConfig, DropPricingRarityRule, EnemyAttributeTemplates, EnemyDifficultyAuthoringGuardrails, EnemyTemplateRule, EnemyTierTemplateRule, ModelState } from './types';
import { scoringConfigFromState, type ScoringConfig } from './realScoring';
import { createDefaultDropPricingConfig, createDefaultEnemyAttributeTemplates, createDefaultState } from './defaults';
import { parseCombatBalanceModel, serializeCombatBalanceModel, type CombatBalanceModel } from './balance';
import { createDefaultAttrDerivations } from './defaults';
import type { AttrDerivation } from './realScoring';
import { parseEnemyDifficultyAuthoringGuardrails } from './enemyDifficultyGuardrails';

export type RawSharedConfig = Record<string, unknown> & {
  combatScoring?: {
    constants?: Record<string, unknown>;
    attributeUnitValues?: Record<string, unknown>;
    rarityMultipliers?: Record<string, unknown>;
    slotMultipliers?: Record<string, unknown>;
    enemyAttributeTemplates?: unknown;
    attrDerivations?: unknown[];
    dropPricing?: unknown;
  };
  combatDifficulty?: {
    levels?: unknown[];
    authoringGuardrails?: unknown;
    [key: string]: unknown;
  };
  combatBalanceModel?: unknown;
};

function parseNumber(raw: unknown, fallback: number): number {
  const parsed = Number(raw);
  return Number.isFinite(parsed) ? parsed : fallback;
}

function numberMap(raw: unknown): Record<string, number> {
  if (!raw || typeof raw !== 'object' || Array.isArray(raw)) return {};
  const out: Record<string, number> = {};
  for (const [key, value] of Object.entries(raw)) {
    const normalized = String(key || '').trim();
    const parsed = Number(value);
    if (!normalized || !Number.isFinite(parsed)) continue;
    out[normalized] = parsed;
  }
  return out;
}

function parseWeights(raw: unknown): Record<string, number> {
  return numberMap(raw);
}

function parseTemplateRule(raw: unknown, fallback: EnemyTemplateRule): EnemyTemplateRule {
  const source = raw && typeof raw === 'object' && !Array.isArray(raw) ? raw as Record<string, unknown> : {};
  const preferredSkillTypes = Array.isArray(source.preferredSkillTypes)
    ? (source.preferredSkillTypes as unknown[]).map(String).filter(Boolean)
    : (fallback.preferredSkillTypes ?? []);
  const rawBaseAttrs = parseWeights(source.baseAttrs);
  const rawMultipliers = parseWeights(source.multipliers);
  const result: EnemyTemplateRule = {
    key: String(source.key || fallback.key).trim() || fallback.key,
    label: String(source.label || fallback.label).trim() || fallback.label,
    weights: Object.keys(parseWeights(source.weights)).length > 0 ? parseWeights(source.weights) : { ...fallback.weights },
    preferredSkillTypes,
    description: typeof source.description === 'string' ? source.description : fallback.description
  };
  if (Object.keys(rawBaseAttrs).length > 0) result.baseAttrs = rawBaseAttrs;
  else if (fallback.baseAttrs && Object.keys(fallback.baseAttrs).length > 0) result.baseAttrs = { ...fallback.baseAttrs };
  if (Object.keys(rawMultipliers).length > 0) result.multipliers = rawMultipliers;
  else if (fallback.multipliers && Object.keys(fallback.multipliers).length > 0) result.multipliers = { ...fallback.multipliers };
  return result;
}

const DEFAULT_TIER_ROUNDS = { min: 4, max: 8 };

function parseTierRounds(raw: unknown, fallback?: { min: number; max: number }): { min: number; max: number } {
  const fb = fallback ?? DEFAULT_TIER_ROUNDS;
  if (!raw || typeof raw !== 'object' || Array.isArray(raw)) return fb;
  const src = raw as Record<string, unknown>;
  const min = Math.max(1, Math.round(parseNumber(src.min, fb.min)));
  const max = Math.max(min, Math.round(parseNumber(src.max, fb.max)));
  return { min, max };
}

function parseTierTemplateRule(raw: unknown, fallback: EnemyTierTemplateRule): EnemyTierTemplateRule {
  const source = raw && typeof raw === 'object' && !Array.isArray(raw) ? raw as Record<string, unknown> : {};
  return {
    key: String(source.key || fallback.key).trim() || fallback.key,
    label: String(source.label || fallback.label).trim() || fallback.label,
    enemyType: String(source.enemyType || fallback.enemyType || 'normal').trim() || 'normal',
    budgetMultiplier: parseNumber(source.budgetMultiplier, fallback.budgetMultiplier ?? 1),
    scoreMult: parseNumber(source.scoreMult, fallback.scoreMult ?? 1),
    rewardMult: parseNumber(source.rewardMult, fallback.rewardMult ?? 1),
    pressurePct: Math.max(0, parseNumber(source.pressurePct, fallback.pressurePct ?? 0.15)),
    targetRounds: parseTierRounds(source.targetRounds, fallback.targetRounds),
    hpMultiplier: parseNumber(source.hpMultiplier, fallback.hpMultiplier ?? 1),
    description: typeof source.description === 'string' ? source.description : fallback.description
  };
}

function mergeTemplateArray<T extends { key: string }>(fallbacks: T[], rawRows: unknown, parse: (raw: unknown, fallback: T) => T): T[] {
  const rawByKey = new Map<string, unknown>();
  if (Array.isArray(rawRows)) {
    for (const row of rawRows) {
      if (!row || typeof row !== 'object' || Array.isArray(row)) continue;
      const key = String((row as Record<string, unknown>).key || '').trim();
      if (key) rawByKey.set(key, row);
    }
  }
  const merged = fallbacks.map((fallback) => parse(rawByKey.get(fallback.key), fallback));
  if (Array.isArray(rawRows)) {
    for (const row of rawRows) {
      if (!row || typeof row !== 'object' || Array.isArray(row)) continue;
      const key = String((row as Record<string, unknown>).key || '').trim();
      if (!key || fallbacks.some((fallback) => fallback.key === key)) continue;
      merged.push(parse(row, { ...(row as T), key } as T));
    }
  }
  return merged;
}

function parseEnemyAttributeTemplates(raw: unknown): EnemyAttributeTemplates {
  const defaults = createDefaultEnemyAttributeTemplates();
  const source = raw && typeof raw === 'object' && !Array.isArray(raw) ? raw as Record<string, unknown> : {};
  const parsed = {
    tiers: mergeTemplateArray(defaults.tiers, source.tiers, parseTierTemplateRule),
    roles: mergeTemplateArray(defaults.roles, source.roles, parseTemplateRule),
    species: mergeTemplateArray(defaults.species, source.species, parseTemplateRule),
    derivedStatMinimums: { ...defaults.derivedStatMinimums }
  };
  if (source.derivedStatMinimums && typeof source.derivedStatMinimums === 'object') {
    for (const [key, value] of Object.entries(source.derivedStatMinimums)) {
      if (typeof value === 'number' && Number.isFinite(value)) {
        parsed.derivedStatMinimums![key] = value;
      }
    }
  }
  return parsed;
}

function parseDropPricingRarityRule(raw: unknown, fallback: DropPricingRarityRule): DropPricingRarityRule {
  const source = raw && typeof raw === 'object' && !Array.isArray(raw) ? raw as Record<string, unknown> : {};
  return {
    key: String(source.key || fallback.key).trim() || fallback.key,
    weight: Math.max(0, parseNumber(source.weight, fallback.weight)),
    softFloor: Math.max(0, parseNumber(source.softFloor, fallback.softFloor)),
    hardCap: Math.max(0, parseNumber(source.hardCap, fallback.hardCap))
  };
}

function parseDropPricingConfig(raw: unknown): DropPricingConfig {
  const defaults = createDefaultDropPricingConfig();
  const source = raw && typeof raw === 'object' && !Array.isArray(raw) ? raw as Record<string, unknown> : {};
  const rawRarityRules = Array.isArray(source.rarityRules) ? source.rarityRules : [];
  const rawRarityByKey = new Map<string, unknown>();
  for (const row of rawRarityRules) {
    if (!row || typeof row !== 'object' || Array.isArray(row)) continue;
    const key = String((row as Record<string, unknown>).key || '').trim();
    if (key) rawRarityByKey.set(key, row);
  }
  const rarityRules = defaults.rarityRules.map((rule) => parseDropPricingRarityRule(rawRarityByKey.get(rule.key), rule));
  for (const row of rawRarityRules) {
    if (!row || typeof row !== 'object' || Array.isArray(row)) continue;
    const key = String((row as Record<string, unknown>).key || '').trim();
    if (!key || rarityRules.some((rule) => rule.key === key)) continue;
    rarityRules.push(parseDropPricingRarityRule(row, { key, weight: 1, softFloor: 0, hardCap: 0 }));
  }
  return {
    targetGoldPerSecond: Math.max(0, parseNumber(source.targetGoldPerSecond, defaults.targetGoldPerSecond)),
    dropBudgetShare: Math.max(0, parseNumber(source.dropBudgetShare, defaults.dropBudgetShare)),
    defaultPlayerCount: Math.max(1, Math.round(parseNumber(source.defaultPlayerCount, defaults.defaultPlayerCount))),
    waveScorePerSecond: Math.max(0.0001, parseNumber(source.waveScorePerSecond, defaults.waveScorePerSecond)),
    specialWaveHorizon: Math.max(1, Math.round(parseNumber(source.specialWaveHorizon, defaults.specialWaveHorizon))),
    worldFloorGrowth: Math.max(0, parseNumber(source.worldFloorGrowth, defaults.worldFloorGrowth)),
    worldFloorMaxMultiplier: Math.max(1, parseNumber(source.worldFloorMaxMultiplier, defaults.worldFloorMaxMultiplier)),
    enemyLevelPriceGrowth: Math.max(0, parseNumber(source.enemyLevelPriceGrowth, defaults.enemyLevelPriceGrowth)),
    enemyLevelPriceMaxMultiplier: Math.max(1, parseNumber(source.enemyLevelPriceMaxMultiplier, defaults.enemyLevelPriceMaxMultiplier)),
    ignoreExistingPriceBelow: Math.max(0, parseNumber(source.ignoreExistingPriceBelow, defaults.ignoreExistingPriceBelow)),
    overpricedWarnRatio: Math.max(1, parseNumber(source.overpricedWarnRatio, defaults.overpricedWarnRatio)),
    enemyTemplateDeviationWarnRatio: Math.max(0, parseNumber(source.enemyTemplateDeviationWarnRatio, defaults.enemyTemplateDeviationWarnRatio)),
    enemyTypeWeights: {
      ...defaults.enemyTypeWeights,
      ...Object.fromEntries(Object.entries(numberMap(source.enemyTypeWeights)).map(([key, value]) => [key, Math.max(0, value)]))
    },
    rarityRules
  };
}

export function scoringConfigFromSharedGlobalConfig(config: RawSharedConfig): ScoringConfig {
  const defaults = scoringConfigFromState(createDefaultState());
  const raw = config.combatScoring || {};
  const constants = { ...defaults.constants };
  for (const key of Object.keys(constants) as Array<keyof ModelState['constants']>) {
    constants[key] = parseNumber(raw.constants?.[key], constants[key]);
  }
  const attrValues = numberMap(raw.attributeUnitValues);
  const rarityMultipliers = numberMap(raw.rarityMultipliers);
  const slotMultipliers = numberMap(raw.slotMultipliers);

  const attributes: AttributeValue[] = defaults.attributes.map((attr) => ({
    ...attr,
    unitValue: attrValues[attr.key] ?? attr.unitValue
  }));
  for (const [key, unitValue] of Object.entries(attrValues)) {
    if (attributes.some((attr) => attr.key === key)) continue;
    attributes.push({ key, label: key, category: 'shared', unitValue });
  }

  const enemyAttributeTemplates = parseEnemyAttributeTemplates(raw.enemyAttributeTemplates);
  const attrDerivations = parseAttrDerivations(raw.attrDerivations);
  const dropPricing = parseDropPricingConfig(raw.dropPricing);

  return {
    constants,
    attributes,
    rarities: defaults.rarities.map((rarity) => ({ ...rarity, multiplier: rarityMultipliers[rarity.key] ?? rarity.multiplier })),
    slots: defaults.slots.map((slot) => ({ ...slot, multiplier: slotMultipliers[slot.key] ?? slot.multiplier })),
    enemyAttributeTemplates,
    attrDerivations,
    dropPricing
  };
}

function parseAttrDerivations(raw: unknown): AttrDerivation[] {
  const defaults = createDefaultAttrDerivations();
  if (!Array.isArray(raw)) return defaults;
  const byKey = new Map(defaults.map((d) => [d.primaryKey, { ...d, rates: { ...d.rates } }]));
  for (const entry of raw) {
    if (!entry || typeof entry !== 'object' || Array.isArray(entry)) continue;
    const src = entry as Record<string, unknown>;
    const key = String(src.primaryKey || '').trim();
    if (!key) continue;
    const rates = numberMap(src.rates);
    if (byKey.has(key)) {
      Object.assign(byKey.get(key)!.rates, rates);
    } else {
      byKey.set(key, { primaryKey: key, rates });
    }
  }
  return [...byKey.values()];
}

export function loadSharedScoringConfig(): ScoringConfig {
  return scoringConfigFromSharedGlobalConfig(sharedGlobalConfig as RawSharedConfig);
}

export function balanceModelFromSharedGlobalConfig(config: RawSharedConfig): CombatBalanceModel {
  return parseCombatBalanceModel(config.combatBalanceModel);
}

export function loadCombatBalanceModel(): CombatBalanceModel {
  return balanceModelFromSharedGlobalConfig(sharedGlobalConfig as RawSharedConfig);
}

export function sharedGlobalConfigWithScoring(
  rawConfig: RawSharedConfig,
  scoring: ScoringConfig,
  balanceModel?: CombatBalanceModel
): RawSharedConfig {
  return {
    ...rawConfig,
    combatScoring: {
      ...(rawConfig.combatScoring || {}),
      constants: { ...scoring.constants },
      attributeUnitValues: Object.fromEntries(scoring.attributes.map((attr) => [attr.key, attr.unitValue])),
      rarityMultipliers: Object.fromEntries(scoring.rarities.map((rarity) => [rarity.key, rarity.multiplier])),
      slotMultipliers: Object.fromEntries(scoring.slots.map((slot) => [slot.key, slot.multiplier])),
      enemyAttributeTemplates: scoring.enemyAttributeTemplates,
      dropPricing: scoring.dropPricing
    },
    ...(balanceModel ? { combatBalanceModel: serializeCombatBalanceModel(balanceModel) } : {})
  };
}

// 通用倍率只承载攻防节奏；生命压力由各世界 difficultySettings.hp 继续补足。
// 奖励: 1 位小数, 超 1 位则以 5 结尾 (0.15, 0.45, 1.05 等), 经验/掉落同曲线
export const DEFAULT_DIFFICULTY_LEVELS: CombatDifficultyLevel[] = [
  { level: 1, labelKey: 'game.difficulty.level1', statMultiplierCurve: { base: 1.15, step: 0, overrides: {} }, battleExpBonus: 0.10, battleDropRateBonus: 0.10 },
  { level: 2, labelKey: 'game.difficulty.level2', statMultiplierCurve: { base: 1.35, step: 0, overrides: {} }, battleExpBonus: 0.15, battleDropRateBonus: 0.15 },
  { level: 3, labelKey: 'game.difficulty.level3', statMultiplierCurve: { base: 1.65, step: 0, overrides: {} }, battleExpBonus: 0.20, battleDropRateBonus: 0.20 },
  { level: 4, labelKey: 'game.difficulty.level4', statMultiplierCurve: { base: 2.10, step: 0, overrides: {} }, battleExpBonus: 0.30, battleDropRateBonus: 0.30 },
  { level: 5, labelKey: 'game.difficulty.level5', statMultiplierCurve: { base: 2.60, step: 0, overrides: {} }, battleExpBonus: 0.45, battleDropRateBonus: 0.45 },
  { level: 6, labelKey: 'game.difficulty.level6', statMultiplierCurve: { base: 3.15, step: 0, overrides: {} }, battleExpBonus: 0.60, battleDropRateBonus: 0.60 },
  { level: 7, labelKey: 'game.difficulty.level7', statMultiplierCurve: { base: 3.80, step: 0, overrides: {} }, battleExpBonus: 0.80, battleDropRateBonus: 0.80 },
  { level: 8, labelKey: 'game.difficulty.level8', statMultiplierCurve: { base: 4.50, step: 0, overrides: {} }, battleExpBonus: 1.05, battleDropRateBonus: 1.05 },
  { level: 9, labelKey: 'game.difficulty.level9', statMultiplierCurve: { base: 5.20, step: 0, overrides: {} }, battleExpBonus: 1.30, battleDropRateBonus: 1.30 },
  { level: 10, labelKey: 'game.difficulty.level10', statMultiplierCurve: { base: 6.00, step: 0, overrides: {} }, battleExpBonus: 1.60, battleDropRateBonus: 1.60 },
];

export function difficultyAuthoringGuardrailsFromSharedGlobalConfig(
  config: RawSharedConfig
): EnemyDifficultyAuthoringGuardrails {
  return parseEnemyDifficultyAuthoringGuardrails(config.combatDifficulty?.authoringGuardrails);
}

export function loadDifficultyAuthoringGuardrails(): EnemyDifficultyAuthoringGuardrails {
  return difficultyAuthoringGuardrailsFromSharedGlobalConfig(sharedGlobalConfig as RawSharedConfig);
}

export function difficultyConfigFromRaw(raw: unknown): CombatDifficultyLevel[] {
  const levels = DEFAULT_DIFFICULTY_LEVELS.map((fallback) => ({ ...fallback }));
  const rawArr = Array.isArray(raw) ? raw : [];
  for (const rawLevel of rawArr) {
    if (!rawLevel || typeof rawLevel !== 'object') continue;
    const src = rawLevel as Record<string, unknown>;
    const lvl = Number(src.level) || 0;
    if (lvl < 1 || lvl > 10) continue;
    const idx = lvl - 1;
    if (src.labelKey && typeof src.labelKey === 'string') levels[idx].labelKey = src.labelKey;
    if (src.statMultiplierCurve && typeof src.statMultiplierCurve === 'object') levels[idx].statMultiplierCurve = src.statMultiplierCurve as Record<string, unknown>;
    if (typeof src.battleExpBonus === 'number') levels[idx].battleExpBonus = src.battleExpBonus;
    if (typeof src.battleDropRateBonus === 'number') levels[idx].battleDropRateBonus = src.battleDropRateBonus;
  }
  return levels;
}

export function sharedGlobalConfigWithDifficulty(
  rawConfig: RawSharedConfig,
  levels: CombatDifficultyLevel[],
  authoringGuardrails = difficultyAuthoringGuardrailsFromSharedGlobalConfig(rawConfig)
): RawSharedConfig {
  return {
    ...rawConfig,
    combatDifficulty: {
      ...(rawConfig.combatDifficulty || {}),
      levels: levels.map((lvl) => ({
        level: lvl.level,
        labelKey: lvl.labelKey,
        statMultiplierCurve: lvl.statMultiplierCurve,
        battleExpBonus: lvl.battleExpBonus,
        battleDropRateBonus: lvl.battleDropRateBonus,
      })),
      authoringGuardrails
    }
  };
}
