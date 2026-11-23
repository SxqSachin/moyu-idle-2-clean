import type { AttributeValue, Constants, DropPricingConfig, EnemyAttributeTemplates, ModelState, RarityRule, ScoreBreakdownItem, ScoreBreakdownSection, ScoreResult, SlotRule } from './types';
import { asArray, asNumber, asString, isRecord, type Catalog, type DataRow } from './catalog';
import { getAttrValueMap, levelBudget, round } from './scoring';
import { createDefaultDropPricingConfig } from './defaults';
import { hitDamageEquivalentSum, normalizeHitDamageScaling } from './hitDamageScaling';

/** 主属性 → 派生属性换算率（1点主属性可提供多少该派生属性） */
export interface AttrDerivation {
  primaryKey: string;
  rates: Record<string, number>;
}

export interface ScoringConfig {
  constants: Constants;
  attributes: AttributeValue[];
  rarities: RarityRule[];
  slots: SlotRule[];
  enemyAttributeTemplates: EnemyAttributeTemplates;
  attrDerivations: AttrDerivation[];
  dropPricing: DropPricingConfig;
}

export function scoringConfigFromState(state: ModelState): ScoringConfig {
  return {
    constants: state.constants,
    attributes: state.attributes,
    rarities: state.rarities,
    slots: state.slots,
    enemyAttributeTemplates: { tiers: [], roles: [], species: [] },
    attrDerivations: [],
    dropPricing: createDefaultDropPricingConfig()
  };
}

function emptyScore(detail: Record<string, number> = {}): ScoreResult {
  return { score: 0, goldValue: 0, detail, sections: [] };
}

function valueMap(config: ScoringConfig): Record<string, number> {
  return getAttrValueMap(config.attributes);
}

function asStringList(raw: unknown): string[] {
  if (Array.isArray(raw)) {
    return raw.flatMap((value) => asStringList(value));
  }
  if (typeof raw === 'string') {
    return raw.split(',').map((value) => value.trim()).filter(Boolean);
  }
  const value = asString(raw).trim();
  return value ? [value] : [];
}

function scoreAttrMap(raw: unknown, config: ScoringConfig): number {
  if (!isRecord(raw)) return 0;
  const values = valueMap(config);
  const percentAttrs = new Set(config.attributes.filter((attr) => attr.displayAsPercent).map((attr) => attr.key));
  let total = 0;
  for (const [key, value] of Object.entries(raw)) {
    const amount = asNumber(value, Number.NaN);
    if (!Number.isFinite(amount)) continue;
    total += amount * (percentAttrs.has(key) ? 100 : 1) * (values[key] ?? 0);
  }
  return total;
}

function rarityMultiplier(config: ScoringConfig, rarity: unknown): number {
  const id = asString(rarity).trim();
  return config.rarities.find((row) => row.key === id)?.multiplier ?? 1;
}

function slotMultiplier(config: ScoringConfig, slot: unknown): number {
  const id = asString(slot).trim();
  return config.slots.find((row) => row.key === id)?.multiplier ?? 1;
}

function enemyTierMultiplier(config: ScoringConfig, type: unknown): number {
  const id = asString(type || 'normal').trim();
  return config.enemyAttributeTemplates.tiers.find((row) => row.key === id)?.scoreMult ?? 1;
}

function enemyRewardMultiplier(config: ScoringConfig, type: unknown): number {
  const id = asString(type || 'normal').trim();
  return config.enemyAttributeTemplates.tiers.find((row) => row.key === id)?.rewardMult ?? 1;
}

export function formulaBreakdown(raw: unknown, config: ScoringConfig, level = 1): { score: number; items: ScoreBreakdownItem[] } {
  if (!isRecord(raw)) return { score: 0, items: [] };
  const items: ScoreBreakdownItem[] = [];
  let total = asNumber(raw.baseAdd, 0);
  if (total !== 0) items.push({ label: '基础值', score: round(total), meta: `baseAdd ${total}` });
  for (const term of asArray(raw.terms)) {
    if (!isRecord(term)) continue;
    const attrKey = asString(term.attrKey).trim();
    if (!attrKey) continue;
    const coeff = asNumber(term.coefficient, 0) + asNumber(term.perSkillLevel, 0) * Math.max(0, level - 1);
    const attrValue = valueMap(config)[attrKey] ?? 1;
    const contribution = coeff * attrValue;
    total += contribution;
    items.push({ label: attrKey, score: round(contribution), meta: `${round(coeff, 4)} * ${round(attrValue, 4)}` });
  }
  return { score: total, items };
}

function formulaScore(raw: unknown, config: ScoringConfig, level = 1): number {
  return formulaBreakdown(raw, config, level).score;
}

export function targetCountBreakdown(targeting: unknown, config: ScoringConfig, level = 1): { count: number; items: ScoreBreakdownItem[] } {
  const countNode = isRecord(targeting) && isRecord(targeting.targetCount) ? targeting.targetCount : {};
  const base = Math.max(1, asNumber(countNode.base, 1));
  const formula = formulaBreakdown({ baseAdd: 0, terms: asArray(countNode.formula) }, config, level);
  const maxRaw = Math.trunc(asNumber(countNode.max, Number.NaN));
  const uncapped = base + Math.max(0, formula.score);
  // When a max cap exists, score at the cap (worst-case / conservative balance principle)
  const count = Number.isFinite(maxRaw) && maxRaw > 0 ? maxRaw : uncapped;
  const items: ScoreBreakdownItem[] = [{ label: '基础目标数', score: round(base) }];
  if (formula.score !== 0) items.push({ label: '目标数公式', score: round(formula.score), meta: formula.items.map((item) => `${item.label}: ${item.score}`).join(', ') });
  if (Number.isFinite(maxRaw) && maxRaw > 0) items.push({ label: '最大目标数(评分取上限)', score: round(maxRaw) });
  return { count: Math.max(1, count), items };
}

function targetCountFactor(targeting: unknown, config: ScoringConfig, level = 1): number {
  return targetCountBreakdown(targeting, config, level).count;
}

export function targetingUtilityScore(targeting: unknown, config: ScoringConfig): number {
  if (!isRecord(targeting)) return 0;
  const positions = asArray(targeting.targetPositions).map((value) => asString(value)).filter(Boolean);
  const positionBudget = positions.reduce<number>((sum, position) => {
    if (position === 'rearguard') return sum + config.constants.rearTargetValue;
    if (position === 'midguard') return sum + config.constants.targetPositionValue;
    return sum;
  }, 0);
  const seek = asString(targeting.secondarySeek);
  const seekBudget = seek === 'rear_to_front'
    ? config.constants.seekStrategyValue
    : seek === 'random_alive'
      ? config.constants.seekStrategyValue * 0.45
      : 0;
  const rule = asString(targeting.selectRule);
  const ruleBudget = rule && rule !== 'random'
    ? (rule === 'boss_first' ? config.constants.selectRuleValue * 1.4 : config.constants.selectRuleValue)
    : 0;
  const fallbackBudget = targeting.fallbackSelfIfEmpty ? config.constants.fallbackSelfValue : 0;
  return positionBudget + seekBudget + ruleBudget + fallbackBudget;
}

function rearPressureRiskScore(targeting: unknown, config: ScoringConfig): number {
  if (!isRecord(targeting)) return 0;
  const positions = asArray(targeting.targetPositions).map((value) => asString(value)).filter(Boolean);
  const reachesRear = positions.includes('rearguard');
  const reachesMid = positions.includes('midguard');
  const seek = asString(targeting.secondarySeek);
  const rule = asString(targeting.selectRule);
  let risk = 0;
  if (reachesRear) risk += config.constants.rearTargetValue * 3.5;
  if (reachesRear && seek === 'random_alive') risk += config.constants.rearTargetValue * 2.5;
  if (seek === 'rear_to_front') risk += config.constants.seekStrategyValue * 2.5;
  if (reachesRear && rule && rule !== 'random' && rule !== 'first') risk += config.constants.selectRuleValue * 1.8;
  // 全排覆盖惩罚使用固定系数，不依赖 extraSkillTargetValue（该常量语义不同，不应混用）
  if (reachesRear && reachesMid && positions.includes('vanguard')) risk += config.constants.rearTargetValue * 0.83;
  return risk;
}

function damageSpecBreakdown(raw: unknown, config: ScoringConfig, level = 1): { score: number; items: ScoreBreakdownItem[] } {
  if (!isRecord(raw)) return { score: 0, items: [] };
  const amount = Math.abs(formulaScore(raw.amountFormula, config, level));
  const hitCount = isRecord(raw.hitCount) ? Math.max(1, asNumber(raw.hitCount.base, 1)) : 1;
  const hitDamageScaling = normalizeHitDamageScaling(raw.hitDamageScaling);
  const damageHitMultiplier = hitDamageScaling ? hitDamageEquivalentSum(hitDamageScaling, hitCount) : hitCount;
  const penetration =
    Math.abs(formulaScore(raw.flatDefensePenetrationFormula, config, level)) * 0.35 +
    Math.abs(formulaScore(raw.physicalFlatDefensePenetrationFormula, config, level)) * 0.35 +
    Math.abs(formulaScore(raw.magicalFlatDefensePenetrationFormula, config, level)) * 0.35 +
    Math.abs(formulaScore(raw.percentDefensePenetrationFormula, config, level)) * 35 +
    Math.abs(formulaScore(raw.physicalPercentDefensePenetrationFormula, config, level)) * 35 +
    Math.abs(formulaScore(raw.magicalPercentDefensePenetrationFormula, config, level)) * 35;
  const amp = Math.abs(formulaScore(raw.extraDamageAmplificationFormula, config, level)) * 45;
  const bonus = isRecord(raw.bonusAgainstTargetTypes)
    ? Object.values(raw.bonusAgainstTargetTypes).reduce<number>((sum, value) => sum + Math.abs(asNumber(value, 0)) * 18, 0)
    : 0;
  const score = amount * damageHitMultiplier + penetration + amp + bonus;
  const items: ScoreBreakdownItem[] = [];
  if (amount) items.push({
    label: 'DamageSpec数值',
    score: round(amount * damageHitMultiplier),
    meta: hitDamageScaling
      ? `hitCount ${round(hitCount)}, equivalentHits ${round(damageHitMultiplier, 4)}`
      : `hitCount ${round(hitCount)}`
  });
  if (penetration) items.push({ label: '穿透', score: round(penetration) });
  if (amp) items.push({ label: '增伤', score: round(amp) });
  if (bonus) items.push({ label: '目标类型加成', score: round(bonus) });
  return { score, items };
}

function damageSpecScore(raw: unknown, config: ScoringConfig, level = 1): number {
  return damageSpecBreakdown(raw, config, level).score;
}

export function effectBreakdown(effect: unknown, config: ScoringConfig, level = 1, label = '效果'): ScoreBreakdownSection {
  if (!isRecord(effect)) return { id: label, label, score: 0, items: [] };
  const kind = asString(effect.kind);
  const target = targetCountBreakdown(effect.targeting, config, level);
  const targetFactor = target.count;
  const targetUtility = targetingUtilityScore(effect.targeting, config);
  const items: ScoreBreakdownItem[] = [];
  let base = 0;
  if (kind === 'damage') {
    const spec = damageSpecBreakdown(effect.damageSpec, config, level);
    const amount = formulaBreakdown(effect.amountFormula, config, level);
    base = Math.max(spec.score, Math.abs(amount.score));
    items.push({ label: '数值公式', score: round(Math.abs(amount.score)), meta: amount.items.map((item) => `${item.label}: ${item.score}`).join(', ') });
    for (const item of amount.items) items.push({ label: `公式/${item.label}`, score: round(Math.abs(item.score)), meta: item.meta });
    for (const item of spec.items) items.push(item);
  } else if (kind === 'heal') {
    const amount = formulaBreakdown(effect.amountFormula, config, level);
    base = Math.abs(amount.score) * 0.9;
    items.push({ label: '治疗公式', score: round(base), meta: amount.items.map((item) => `${item.label}: ${item.score}`).join(', ') });
    for (const item of amount.items) items.push({ label: `公式/${item.label}`, score: round(Math.abs(item.score) * 0.9), meta: item.meta });
  } else if (kind === 'resource') {
    const amount = formulaBreakdown(effect.amountFormula, config, level);
    const resource = asString(effect.resource, 'mp');
    const factor = resource === 'hp' ? 0.9 : 1.4;
    base = Math.abs(amount.score) * factor;
    items.push({ label: `${resource.toUpperCase()}恢复公式`, score: round(base), meta: amount.items.map((item) => `${item.label}: ${item.score}`).join(', ') });
    for (const item of amount.items) items.push({ label: `公式/${item.label}`, score: round(Math.abs(item.score) * factor), meta: item.meta });
  } else if (kind === 'status') {
    const payload = isRecord(effect.statusPayload) ? effect.statusPayload : {};
    const count = asArray(payload.statusIds).length || 1;
    const duration = asNumber(payload.durationBaseTurns, 1) + asNumber(payload.durationPerSkillLevelTurns, 0) * Math.max(0, level - 1);
    base = count * Math.max(1, duration) * 12;
    items.push({ label: '状态基础值', score: round(base), meta: `${count}个状态 * ${round(duration)}回合` });
  }
  const multiTargetFactor = config.constants.basicAttackMultiTargetFactor ?? 0.35;
  const effectiveTargetMult = 1 + (targetFactor - 1) * multiTargetFactor;
  items.push({ label: '多目标效果', score: round(base * effectiveTargetMult), meta: `基础x${round(effectiveTargetMult, 2)} (${round(targetFactor, 1)}目标 × ${round(multiTargetFactor, 2)}折扣)` });
  for (const item of target.items) items.push({ ...item, label: `目标/${item.label}` });
  if (targetUtility) items.push({ label: '目标策略', score: round(targetUtility) });
  const rearRisk = kind === 'damage' ? rearPressureRiskScore(effect.targeting, config) : 0;
  if (rearRisk) {
    items.push({
      label: '后排压力风险',
      score: -round(rearRisk),
      meta: '伤害效果不应直接命中后排；推荐只选前排，次要索敌 front_to_rear'
    });
  }
  return { id: label, label, score: round(base * effectiveTargetMult + targetUtility - rearRisk), items };
}

function effectScore(effect: unknown, config: ScoringConfig, level = 1): number {
  return effectBreakdown(effect, config, level).score;
}

function effectsListScore(raw: unknown, config: ScoringConfig, level = 1): number {
  return asArray(raw).reduce<number>((sum, effect) => sum + effectScore(effect, config, level), 0);
}

function effectsBreakdown(raw: unknown, config: ScoringConfig, level = 1, labelPrefix = '效果'): ScoreBreakdownSection[] {
  return asArray(raw).map((effect, index) => effectBreakdown(effect, config, level, `${labelPrefix} #${index + 1}`));
}

function rearPressurePenaltyFromSections(sections: ScoreBreakdownSection[]): number {
  return Math.abs(sections.reduce<number>((sum, section) => {
    return sum + (section.items ?? [])
      .filter((item) => item.label === '后排压力风险')
      .reduce<number>((itemSum, item) => itemSum + item.score, 0);
  }, 0));
}

function withGold(score: number, detail: Record<string, number>, config: ScoringConfig, sections: ScoreBreakdownSection[] = []): ScoreResult {
  const safe = Math.max(0, score);
  return {
    score: round(safe),
    goldValue: round(safe * config.constants.goldPerBudget),
    detail,
    sections
  };
}

export function scoreRealSkill(row: DataRow, config: ScoringConfig): ScoreResult {
  const level = Math.max(1, asNumber(row.level, 1));
  const baseBudget = levelBudget(level, config.constants);
  const effectSections = effectsBreakdown(row.effects, config, level, '技能效果');
  const effectBudget = effectSections.reduce((sum, section) => sum + section.score, 0);
  const rearPressurePenalty = rearPressurePenaltyFromSections(effectSections);
  const targetingBudget = baseBudget * (targetCountFactor(row.targeting, config, level) - 1) * 0.35;
  const mpPenalty = asNumber(row.mpCost, 0) * (valueMap(config).mp ?? 0.05) * config.constants.mpCostPenalty;
  const cooldownPenalty = baseBudget * asNumber(row.cooldown, 0) * config.constants.cooldownPenaltyPerTurn;
  const requirementBudget = scoreAttrMap(row.statusRequirement, config) * 0.08;
  const score = effectBudget + targetingBudget + requirementBudget - mpPenalty - cooldownPenalty;
  return withGold(score, {
    baseBudget: round(baseBudget),
    effectBudget: round(effectBudget),
    targetingBudget: round(targetingBudget),
    rearPressurePenalty: round(rearPressurePenalty),
    requirementBudget: round(requirementBudget),
    mpPenalty: round(mpPenalty),
    cooldownPenalty: round(cooldownPenalty)
  }, config, effectSections);
}

function curveScore(raw: unknown, config: ScoringConfig, maxLevel = 0): number {
  if (!isRecord(raw)) return 0;
  const base = Math.abs(scoreAttrMap(raw.base, config));
  const step = Math.abs(scoreAttrMap(raw.step, config)) * Math.max(0, maxLevel);
  const overrides = isRecord(raw.overrides)
    ? Object.values(raw.overrides).reduce<number>((sum, value) => sum + Math.abs(scoreAttrMap(value, config)), 0)
    : 0;
  return base + step + overrides;
}

function curveScoreAtLevel(raw: unknown, config: ScoringConfig, level = 0): number {
  if (!isRecord(raw) || level <= 0) return 0;
  const base = Math.abs(scoreAttrMap(raw.base, config));
  const step = Math.abs(scoreAttrMap(raw.step, config)) * level;
  const overrides = isRecord(raw.overrides)
    ? Object.entries(raw.overrides).reduce<number>((sum, [rawLevel, value]) => {
        const overrideLevel = Math.trunc(asNumber(rawLevel, Number.NaN));
        if (!Number.isFinite(overrideLevel) || overrideLevel > level) return sum;
        return sum + Math.abs(scoreAttrMap(value, config));
      }, 0)
    : 0;
  return base + step + overrides;
}

function effectsTargetingScore(raw: unknown, config: ScoringConfig): number {
  return asArray(raw).reduce<number>((sum, effect) => (isRecord(effect) ? sum + targetingUtilityScore(effect.targeting, config) : sum), 0);
}

export function scoreRealEquipment(row: DataRow, config: ScoringConfig): ScoreResult {
  const maxLevel = Math.max(0, asNumber(row.enhanceMaxLevel, 0));
  const baseAttrs = Math.abs(scoreAttrMap(row.baseAttrs, config));
  const enhanceAttrs = curveScore(row.enhanceAttrCurve, config, maxLevel);
  const enhanceMin = curveScoreAtLevel(row.enhanceAttrCurve, config, Math.min(1, maxLevel));
  const enhanceMax = curveScoreAtLevel(row.enhanceAttrCurve, config, maxLevel);
  const costBudget = curveScore(row.enhanceCostCurve, config, maxLevel) * 0.08;
  const attackSections = effectsBreakdown(row.basicAttackEffects, config, 1, '普攻效果');
  const basicAttack = attackSections.reduce((sum, section) => sum + section.score, 0);
  const rearPressurePenalty = rearPressurePenaltyFromSections(attackSections);
  const targetingUtility = effectsTargetingScore(row.basicAttackEffects, config);
  const hooks = asArray(row.battleHooks).length * config.constants.battleHookValue;
  const affixSlots = asNumber(row.affixSlotCount, 0) * config.constants.affixSlotBaseBudget;
  const slotFactor = slotMultiplier(config, row.slot);
  const score = (baseAttrs + basicAttack + hooks + affixSlots) * slotFactor;
  return withGold(score, {
    baseAttrs: round(baseAttrs),
    enhanceAttrs: round(enhanceAttrs),
    enhanceMin: round(enhanceMin),
    enhanceMax: round(enhanceMax),
    basicAttack: round(basicAttack),
    rearPressurePenalty: round(rearPressurePenalty),
    targetingUtility: round(targetingUtility),
    battleHooks: round(hooks),
    affixSlots: round(affixSlots),
    slotFactor: round(slotFactor),
    costBudget: round(costBudget)
  }, config, attackSections);
}

function scoreAttrBonusAffixRange(effectParams: unknown, config: ScoringConfig): { min: number; max: number } {
  if (!isRecord(effectParams) || !isRecord(effectParams.attrMods)) return { min: 0, max: 0 };
  let min = Number.POSITIVE_INFINITY;
  let max = 0;
  for (const rarityPayload of Object.values(effectParams.attrMods)) {
    if (!isRecord(rarityPayload)) continue;
    const minAttrs: Record<string, number> = {};
    const maxAttrs: Record<string, number> = {};
    for (const [attr, range] of Object.entries(rarityPayload)) {
      if (isRecord(range)) {
        minAttrs[attr] = asNumber(range.min, asNumber(range.max, 0));
        maxAttrs[attr] = asNumber(range.max, asNumber(range.min, 0));
      } else {
        minAttrs[attr] = asNumber(range, 0);
        maxAttrs[attr] = asNumber(range, 0);
      }
    }
    const minScore = Math.abs(scoreAttrMap(minAttrs, config));
    const maxScore = Math.abs(scoreAttrMap(maxAttrs, config));
    min = Math.min(min, minScore);
    max = Math.max(max, maxScore);
  }
  return { min: Number.isFinite(min) ? min : 0, max };
}

function scoreEffectMutationAffix(effectParams: unknown, config: ScoringConfig): number {
  if (!isRecord(effectParams)) return 0;
  let total = 0;
  for (const mutation of asArray(effectParams.effectMutations)) {
    if (!isRecord(mutation) || !isRecord(mutation.ops)) continue;
    total += Math.abs(asNumber(mutation.ops.amountBaseAddDelta, 0));
    // 每增加1个目标约值12分，不依赖 extraSkillTargetValue
    total += Math.abs(asNumber(mutation.ops.targetCountDelta, 0)) * 12;
    total += asArray(mutation.ops.amountTermsAppend).reduce<number>((sum, term) => {
      if (!isRecord(term)) return sum;
      const attrKey = asString(term.attrKey);
      return sum + Math.abs(asNumber(term.coefficient, 0)) * (valueMap(config)[attrKey] ?? 1);
    }, 0);
    total += asArray(mutation.ops.statusIdsAppend).length * 10;
    total += asArray(mutation.ops.statusIdsRemove).length * 8;
  }
  return total;
}

function buffDslEffectScore(effect: unknown): number {
  if (!isRecord(effect)) return 0;
  const kind = asString(effect.kind || effect.type).toLowerCase();
  if (kind === 'turn_action_suppressed') return 42;
  if (kind === 'skill_action_suppressed') return 30;
  if (kind === 'basic_attack_suppressed') return 24;
  if (kind === 'treat_all_as_enemy') return 52;
  if (kind === 'camouflage_targeting_remap') return 28;
  if (kind === 'prefer_as_enemy_target') return 22;
  if (kind === 'avoid_as_enemy_target') return 20;
  if (kind === 'prefer_as_ally_target') return 18;
  if (kind === 'avoid_as_ally_target') return 18;
  if (kind === 'untargetable_by_ally') return 24;
  if (kind === 'frontline_fixation') return 32;
  if (kind === 'position_override') return 26;
  if (kind === 'damage_redirect_to_source') return 48 * Math.max(0, asNumber(effect.percent ?? effect.value, 1));
  if (kind === 'reduce_damage_percent') return Math.abs(asNumber(effect.value ?? effect.percent, 0)) * 180;
  if (kind === 'add_damage_percent' || kind === 'set_damage_multiplier') return Math.abs(asNumber(effect.value ?? effect.percent, 0)) * 150;
  if (kind === 'heal') return Math.abs(asNumber(effect.value, 0)) * 0.18;
  if (kind === 'resource') return Math.abs(asNumber(effect.value, 0)) * 0.7;
  if (kind === 'add_buff') {
    const targetMode = asString(effect.targetMode || effect.target).toLowerCase();
    return 18 + Math.max(0, asNumber(effect.durationTurns, 0)) * 4 + (targetMode === 'source' ? 6 : 0);
  }
  if (kind === 'remove_buff') return 16;
  if (kind === 'add_stat') return Math.abs(asNumber(effect.value, 0)) * 0.35;
  if (kind === 'add_stat_percent') return Math.abs(asNumber(effect.value ?? effect.percent, 0)) * 120;
  return 10;
}

export function scoreRealAffix(row: DataRow, config: ScoringConfig): ScoreResult {
  const effectType = asString(row.effectType).toLowerCase();
  const manualScore = Math.max(0, asNumber(row.manualScore, 0));
  const attrRange = scoreAttrBonusAffixRange(row.effectParams, config);
  const attrBonus = attrRange.max;
  const mutation = scoreEffectMutationAffix(row.effectParams, config);
  const hooks = isRecord(row.effectParams) ? asArray(row.effectParams.battleHooks).length * config.constants.battleHookValue : 0;
  const rarity = asArray(row.allowedRarities).reduce<number>((sum, value) => sum + rarityMultiplier(config, value), 0) / Math.max(1, asArray(row.allowedRarities).length);
  const limitPenalty = Math.max(0.55, 1 - asNumber(row.maxEffectivePerBody, 0) * 0.04 - asNumber(row.maxEffectivePerEquipment, 0) * 0.03);
  const base = effectType.includes('mutation') ? mutation : effectType.includes('battle') ? hooks : attrBonus;
  const rangeBase = effectType.includes('mutation') ? mutation : effectType.includes('battle') ? hooks : 0;
  const affixMin = (attrRange.min + rangeBase + manualScore) * Math.max(1, rarity) * limitPenalty;
  const affixMax = (attrRange.max + rangeBase + manualScore) * Math.max(1, rarity) * limitPenalty;
  return withGold((base + manualScore) * Math.max(1, rarity) * limitPenalty, {
    manualScore: round(manualScore),
    attrBonus: round(attrBonus),
    affixMin: round(affixMin),
    affixMax: round(affixMax),
    mutation: round(mutation),
    battleHooks: round(hooks),
    rarityFactor: round(Math.max(1, rarity)),
    limitPenalty: round(limitPenalty)
  }, config);
}

export function scoreRealBuff(row: DataRow, catalog: Catalog, config: ScoringConfig): ScoreResult {
  const id = asString(row.id);
  const effects = catalog.buff_effects.filter((effect) => asString(effect.buffId) === id);
  const hookBudget = effects.reduce((sum, effect) => {
    const effectBudget = asArray(effect.effects).reduce<number>((effectSum, item) => effectSum + buffDslEffectScore(item), 0);
    return sum + effectBudget + asArray(effect.conditions).length * 4;
  }, 0);
  const duration = asNumber(row.permanent, 0) ? 8 : Math.max(1, asNumber(row.durationTurns, 1));
  const durationFactor = 1 + Math.max(0, duration - 1) * config.constants.extraTurnValue;
  const dispelPenalty = row.dispellable === false ? 1 : 0.92;
  return withGold(hookBudget * durationFactor * dispelPenalty, {
    hookBudget: round(hookBudget),
    durationFactor: round(durationFactor),
    dispelPenalty
  }, config);
}

export function scoreRealEnemy(row: DataRow, config: ScoringConfig, catalog?: Catalog): ScoreResult {
  const attrs: Record<string, number> = {};
  for (const attr of config.attributes) {
    const amount = asNumber(row[attr.key], Number.NaN);
    if (Number.isFinite(amount)) attrs[attr.key] = amount;
  }
  const attrBudget = Math.abs(scoreAttrMap(attrs, config));
  const level = Math.max(1, asNumber(row.level, 1));
  const levelFloor = levelBudget(level, config.constants) * 0.18;
  const skillBudget = catalog
    ? asArray(row.skillIds).reduce<number>((sum, skillID) => {
        const skill = catalog.skills.find((entry) => asString(entry.id) === asString(skillID));
        return sum + (skill ? scoreRealSkill(skill, config).score * config.constants.enemySkillScoreMultiplier : 12);
      }, 0)
    : asArray(row.skillIds).length * 12;
  const equipmentBudget = catalog
    ? asArray(row.equipment).reduce<number>((sum, entry) => {
        if (!isRecord(entry)) return sum;
        const template = catalog.equipment_templates.find((equipment) => asString(equipment.id) === asString(entry.templateItemId));
        if (!template) return sum;
        const templateScore = scoreRealEquipment(template, config);
        const baseAttrs = Math.abs(scoreAttrMap(isRecord(entry.baseSnapshot) && Object.keys(entry.baseSnapshot).length ? entry.baseSnapshot : template.baseAttrs, config));
        const enhanceAttrs = curveScoreAtLevel(template.enhanceAttrCurve, config, Math.max(0, asNumber(entry.enhanceLevel, 0)));
        const affixAttrs = asArray(entry.affixes).reduce<number>((affixSum, affix) => {
          if (!isRecord(affix)) return affixSum;
          return affixSum + Math.abs(scoreAttrMap(affix.attrMods, config));
        }, 0);
        return sum + baseAttrs + enhanceAttrs + affixAttrs + asNumber(templateScore.detail.basicAttack, 0) + asNumber(templateScore.detail.battleHooks, 0);
      }, 0)
    : 0;
  const dropBudget = isRecord(row.drops)
    ? Object.values(row.drops).reduce<number>((sum, drop) => {
        if (!isRecord(drop)) return sum;
        return sum + asNumber(drop.percent, 0) * 100 * Math.max(1, asNumber(isRecord(drop.range) ? drop.range.max : 1, 1));
      }, 0)
    : 0;
  const tier = enemyTierMultiplier(config, row.type);
  const reward = enemyRewardMultiplier(config, row.type);
  const score = (attrBudget + levelFloor + skillBudget + equipmentBudget + dropBudget) * tier;
  return withGold(score, {
    attrBudget: round(attrBudget),
    levelFloor: round(levelFloor),
    skillBudget: round(skillBudget),
    equipmentBudget: round(equipmentBudget),
    dropBudget: round(dropBudget),
    tierFactor: round(tier),
    rewardGold: round(score * config.constants.enemyGoldRewardRate * reward),
    rewardExp: round(score * config.constants.enemyExpRewardRate * reward)
  }, config);
}

export function scoreRealWorld(row: DataRow, catalog: Catalog, config: ScoringConfig): ScoreResult {
  const enemyScores = new Map(catalog.enemies.map((enemy) => [asString(enemy.id), scoreRealEnemy(enemy, config, catalog).score]));
  const enemyPool = asStringList(row.enemyIds).reduce<number>((sum, id) => sum + (enemyScores.get(asString(id)) ?? 0), 0);
  const specialPool = asStringList(row.specialEnemyIds).reduce<number>((sum, id) => sum + (enemyScores.get(asString(id)) ?? 0), 0);
  const lineupBudget = asArray(row.customLineupConf).reduce<number>((sum, conf) => {
    if (!isRecord(conf)) return sum;
    const percent = asNumber(conf.percent, 0);
    const lineup = asStringList(conf.lineup).reduce<number>((lineupSum, id) => lineupSum + (enemyScores.get(asString(id)) ?? 0), 0);
    return sum + lineup * Math.max(0, percent);
  }, 0);
  return withGold(enemyPool + specialPool * 1.4 + lineupBudget, {
    enemyPool: round(enemyPool),
    specialPool: round(specialPool),
    lineupBudget: round(lineupBudget)
  }, config);
}

export function scoreRealBattleUsableItem(row: DataRow, catalog: Catalog, config: ScoringConfig): ScoreResult {
  const level = Math.max(1, asNumber(row.level, 1));
  const effectSections = effectsBreakdown(row.effects, config, level, '战斗物品效果');
  const effectBudget = effectSections.reduce((sum, section) => sum + section.score, 0);
  const itemID = asString(row.itemId).trim();
  const item = catalog.items.find((candidate) => asString(candidate.id).trim() === itemID);
  const rarity = rarityMultiplier(config, item?.rarity);
  const cooldownTurns = Math.max(0, asNumber(row.cooldownTurns, 0));
  const cooldownPenalty = effectBudget * cooldownTurns * config.constants.cooldownPenaltyPerTurn;
  const score = effectBudget * rarity - cooldownPenalty;
  return withGold(score, {
    effectBudget: round(effectBudget),
    rarityFactor: round(rarity),
    cooldownTurns: round(cooldownTurns),
    cooldownPenalty: round(cooldownPenalty)
  }, config, effectSections);
}

export function scoreRow(table: string, row: DataRow, catalog: Catalog, config: ScoringConfig): ScoreResult {
  switch (table) {
    case 'skills':
      return scoreRealSkill(row, config);
    case 'equipment_templates':
      return scoreRealEquipment(row, config);
    case 'affix_definitions':
      return scoreRealAffix(row, config);
    case 'buff':
      return scoreRealBuff(row, catalog, config);
    case 'enemies':
      return scoreRealEnemy(row, config, catalog);
    case 'world':
      return scoreRealWorld(row, catalog, config);
    case 'battle_usable_items':
      return scoreRealBattleUsableItem(row, catalog, config);
    default:
      return emptyScore();
  }
}
