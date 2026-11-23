import { asArray, asNumber, asString, isRecord, type DataRow } from './catalog';
import { levelBudget, round } from './scoring';
import {
  effectBreakdown,
  formulaBreakdown,
  targetCountBreakdown,
  type AttrDerivation,
  type ScoringConfig
} from './realScoring';
import { hitDamageEquivalentSum, normalizeHitDamageScaling, type PoweredExponentialDecayDefinition } from './hitDamageScaling';

/**
 * 计算指定属性的主属性等价换算信息。
 * 返回：1点该属性对应多少主属性点消耗，以及最优来源主属性。
 *
 * 规则：
 * - 若 attrKey 本身就是某个主属性（在 derivations 中有 primaryKey=attrKey 的条目），cost=1.0
 * - 否则：找所有以 attrKey 为目标的 rates，取最高换算率（最优来源），cost = 1/rate
 * - 若无已知来源：cost = 0（显示为"无换算数据"）
 */
function getPrimaryEquiv(
  attrKey: string,
  derivations: AttrDerivation[]
): { factor: number; source: string | null } {
  if (derivations.some((d) => d.primaryKey === attrKey)) {
    return { factor: 1.0, source: attrKey };
  }
  let bestRate = 0;
  let bestSource: string | null = null;
  for (const d of derivations) {
    const rate = d.rates[attrKey] ?? 0;
    if (rate > bestRate) {
      bestRate = rate;
      bestSource = d.primaryKey;
    }
  }
  return bestRate > 0 ? { factor: round(1 / bestRate, 6), source: bestSource } : { factor: 0, source: null };
}

export interface FormulaTermDisplay {
  attrKey: string;
  attrLabel: string;
  coefficient: number;
  perSkillLevel: number;
  /** 用于评分系统的单位价值（原有unitValue） */
  unitValue: number;
  /** 主属性等价换算系数（1点该属性=多少主属性点消耗），来自 derivation 表 */
  primaryEquivFactor: number;
  /** 该项最优来源主属性 key，null 表示本身就是主属性或无已知来源 */
  primaryEquivSource: string | null;
  /** 主属性等价权重 = coefficient × primaryEquivFactor */
  termWeight: number;
}

export interface EffectDisplay {
  index: number;
  kind: string;
  damageType: string;
  targetPositions: string[];
  targetScope: string;
  secondarySeek: string;
  selectRule: string;
  selectAttrKey: string;
  fallbackSelf: boolean;
  targetCountBase: number;
  targetCountMax: number | null;
  hitCountBase: number;
  hitDamageEquivalent: number;
  hitDamageScaling: PoweredExponentialDecayDefinition | null;
  baseAdd: number;
  formulaTerms: FormulaTermDisplay[];
  rawFormulaWeight: number;
  effectScore: number;
}

export interface BalanceVerdict {
  baseBudget: number;
  rarityMult: number;
  rarityLabel: string;
  budgetAllowed: number;
  totalEffectScore: number;
  ratio: number;
  status: 'ok' | 'caution' | 'overtuned';
  suggestedRarity: string | null;
  suggestedRarityLabel: string | null;
}

export interface SkillOverview {
  level: number;
  rarity: string;
  cooldown: number;
  mpCost: number;
  effects: EffectDisplay[];
  balance: BalanceVerdict;
}

export interface BattleHookSummary {
  id: string;
  trigger: string;
  effectKinds: string[];
  conditionCount: number;
  hidden: boolean;
}

export interface AffixMilestoneSummary {
  level: string;
  entries: Array<{
    affixId: string;
    rarityId: string;
    rollValue: number | null;
  }>;
}

export interface EquipmentOverview {
  slot: string;
  enhanceMaxLevel: number;
  basicAttackEffects: EffectDisplay[];
  battleHookCount: number;
  battleHooks: BattleHookSummary[];
  affixMilestones: AffixMilestoneSummary[];
  mechanismScore: number;
  balance: BalanceVerdict;
}

function attrLabelMap(config: ScoringConfig): Record<string, string> {
  return Object.fromEntries(config.attributes.map((a) => [a.key, a.label]));
}

function attrUnitMap(config: ScoringConfig): Record<string, number> {
  return Object.fromEntries(config.attributes.map((a) => [a.key, a.unitValue]));
}

function buildEffectDisplay(effect: unknown, config: ScoringConfig, level: number, index: number): EffectDisplay {
  const labels = attrLabelMap(config);
  const units = attrUnitMap(config);

  const e = isRecord(effect) ? effect : {};
  const kind = asString(e.kind);
  const damageType = asString(e.damageType ?? (isRecord(e.damageSpec) ? e.damageSpec.damageType : ''));
  const targeting = isRecord(e.targeting) ? e.targeting : {};
  const positions = asArray(targeting.targetPositions).map((p) => asString(p)).filter(Boolean);
  const targetScope = asString(targeting.targetScope);
  const secondarySeek = asString(targeting.secondarySeek);
  const selectRule = asString(targeting.selectRule);
  const selectAttrKey = asString(targeting.selectAttrKey ?? '');
  const fallbackSelf = Boolean(targeting.fallbackSelfIfEmpty);

  const countInfo = targetCountBreakdown(targeting, config, level);
  const targetCountBase = isRecord(targeting.targetCount) ? Math.max(1, asNumber(targeting.targetCount.base, 1)) : 1;
  const targetCountMaxRaw = isRecord(targeting.targetCount) ? asNumber(targeting.targetCount.max, Number.NaN) : Number.NaN;
  const targetCountMax = Number.isFinite(targetCountMaxRaw) && targetCountMaxRaw > 0 ? Math.trunc(targetCountMaxRaw) : null;

  const damageSpec = isRecord(e.damageSpec) ? e.damageSpec : {};
  const hitCountNode = isRecord(damageSpec.hitCount) ? damageSpec.hitCount : (isRecord(e.hitCount) ? e.hitCount : null);
  const hitCountBase = hitCountNode ? Math.max(1, asNumber(hitCountNode.base, 1)) : 1;
  const hitDamageScaling = normalizeHitDamageScaling(damageSpec.hitDamageScaling);
  const hitDamageEquivalent = hitDamageScaling ? hitDamageEquivalentSum(hitDamageScaling, hitCountBase) : hitCountBase;

  // Build formula terms
  const amountFormula = isRecord(e.amountFormula) ? e.amountFormula : {};
  const baseAdd = asNumber(amountFormula.baseAdd, 0);
  const formulaTerms: FormulaTermDisplay[] = asArray(amountFormula.terms)
    .filter(isRecord)
    .map((term) => {
      const attrKey = asString(term.attrKey).trim();
      const coefficient = asNumber(term.coefficient, 0);
      const perSkillLevel = asNumber(term.perSkillLevel, 0);
      const unitValue = units[attrKey] ?? 1;
      const equiv = getPrimaryEquiv(attrKey, config.attrDerivations ?? []);
      return {
        attrKey,
        attrLabel: labels[attrKey] ?? attrKey,
        coefficient,
        perSkillLevel,
        unitValue,
        primaryEquivFactor: equiv.factor,
        primaryEquivSource: equiv.source,
        termWeight: round(Math.abs(coefficient) * equiv.factor, 4)
      };
    })
    .filter((t) => t.coefficient !== 0 || t.perSkillLevel !== 0);

  // rawFormulaWeight = 每次命中每目标的主属性等价权重合计
  const rawFormulaWeight = round(
    Math.abs(baseAdd) + formulaTerms.reduce((sum, t) => sum + t.termWeight, 0),
    3
  );

  // Use the existing effectBreakdown for the effect's aggregate score
  const section = effectBreakdown(effect, config, level, `效果 #${index + 1}`);
  const effectScore = section.score;

  return {
    index,
    kind,
    damageType,
    targetPositions: positions,
    targetScope,
    secondarySeek,
    selectRule,
    selectAttrKey,
    fallbackSelf,
    targetCountBase,
    targetCountMax,
    hitCountBase,
    hitDamageEquivalent,
    hitDamageScaling,
    baseAdd,
    formulaTerms,
    rawFormulaWeight,
    effectScore
  };
}

function buildBalanceVerdict(
  effects: EffectDisplay[],
  level: number,
  rarity: string,
  config: ScoringConfig
): BalanceVerdict {
  const warnThreshold = config.constants.skillBalanceWarnRatio ?? 1.0;
  const overtuneThreshold = config.constants.skillBalanceOvertuneRatio ?? 1.5;

  const baseBudget = round(levelBudget(level, config.constants), 2);
  const rarityRule = config.rarities.find((r) => r.key === rarity);
  const rarityMult = rarityRule?.multiplier ?? 1;
  const rarityLabel = rarityRule?.label ?? rarity;
  const budgetAllowed = round(baseBudget * rarityMult, 2);

  const totalEffectScore = round(effects.reduce((sum, e) => sum + e.effectScore, 0), 2);
  const ratio = budgetAllowed > 0 ? round(totalEffectScore / budgetAllowed, 4) : 0;

  let status: 'ok' | 'caution' | 'overtuned';
  if (ratio <= warnThreshold) {
    status = 'ok';
  } else if (ratio <= overtuneThreshold) {
    status = 'caution';
  } else {
    status = 'overtuned';
  }

  // Find lowest rarity where ratio <= warnThreshold
  let suggestedRarity: string | null = null;
  let suggestedRarityLabel: string | null = null;
  if (status !== 'ok') {
    const sorted = [...config.rarities].sort((a, b) => a.multiplier - b.multiplier);
    for (const r of sorted) {
      const allowed = baseBudget * r.multiplier;
      if (allowed > 0 && totalEffectScore / allowed <= warnThreshold) {
        suggestedRarity = r.key;
        suggestedRarityLabel = r.label;
        break;
      }
    }
  }

  return {
    baseBudget,
    rarityMult,
    rarityLabel,
    budgetAllowed,
    totalEffectScore,
    ratio,
    status,
    suggestedRarity,
    suggestedRarityLabel
  };
}

export function buildSkillOverview(row: DataRow, config: ScoringConfig): SkillOverview {
  const level = Math.max(1, asNumber(row.level, 1));
  const rarity = asString(row.rarity ?? '');
  const cooldown = asNumber(row.cooldown, 0);
  const mpCost = asNumber(row.mpCost, 0);

  const effects: EffectDisplay[] = asArray(row.effects)
    .filter(isRecord)
    .map((effect, i) => buildEffectDisplay(effect, config, level, i));

  const balance = buildBalanceVerdict(effects, level, rarity, config);

  return { level, rarity, cooldown, mpCost, effects, balance };
}

export function buildEquipmentOverview(row: DataRow, config: ScoringConfig): EquipmentOverview {
  const slot = asString(row.slot ?? '');
  const enhanceMaxLevel = asNumber(row.enhanceMaxLevel, 0);
  const battleHooks: BattleHookSummary[] = asArray(row.battleHooks)
    .filter(isRecord)
    .map((hook, index) => ({
      id: asString(hook.id || `hook_${index + 1}`),
      trigger: asString(hook.trigger),
      effectKinds: asArray(hook.effects)
        .filter(isRecord)
        .map((effect) => asString(effect.kind || effect.type))
        .filter(Boolean),
      conditionCount: asArray(hook.conditions).length,
      hidden: Boolean(hook.hideDescription)
    }));
  const battleHookCount = battleHooks.length;
  const affixMilestones: AffixMilestoneSummary[] = Object.entries(isRecord(row.enhanceAffixMilestonePools) ? row.enhanceAffixMilestonePools : {})
    .map(([level, pool]) => ({
      level,
      entries: asArray(pool).filter(isRecord).map((entry) => ({
        affixId: asString(entry.affixId),
        rarityId: asString(entry.rarityId),
        rollValue: Number.isFinite(asNumber(entry.rollValue, Number.NaN)) ? asNumber(entry.rollValue, 0) : null
      }))
    }))
    .sort((a, b) => asNumber(a.level, 0) - asNumber(b.level, 0));

  const basicAttackEffects: EffectDisplay[] = asArray(row.basicAttackEffects)
    .filter(isRecord)
    .map((effect, i) => buildEffectDisplay(effect, config, 1, i));

  // Equipment doesn't have rarity in the template; use empty to get multiplier=1
  const balance = buildBalanceVerdict(basicAttackEffects, 1, '', config);
  const hookValue = asNumber(config.constants.battleHookValue, 0);
  const mechanismScore = round(
    battleHooks.reduce((sum, hook) => sum + hookValue + hook.effectKinds.length * hookValue * 0.35 + hook.conditionCount * hookValue * 0.12, 0),
    2
  );

  return { slot, enhanceMaxLevel, basicAttackEffects, battleHookCount, battleHooks, affixMilestones, mechanismScore, balance };
}
