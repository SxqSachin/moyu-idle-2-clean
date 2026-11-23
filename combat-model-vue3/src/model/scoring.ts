import type {
  AffixModel,
  AttributeValue,
  BuffModel,
  Constants,
  EffectMap,
  EnemyModel,
  EquipmentModel,
  ModelState,
  ScoreResult,
  SkillModel
} from './types';

export function round(value: number, digits = 2): number {
  const p = 10 ** digits;
  return Math.round((Number.isFinite(value) ? value : 0) * p) / p;
}

export function clamp(value: number, min: number, max: number): number {
  return Math.min(max, Math.max(min, value));
}

export function parseEffectText(input = ''): EffectMap {
  const result: EffectMap = {};
  input
    .split(/[;,，\n]+/)
    .map((s) => s.trim())
    .filter(Boolean)
    .forEach((part) => {
      const match = part.match(/^([a-zA-Z0-9_\-]+)\s*[:=：]\s*([+\-]?\d+(?:\.\d+)?%?)$/);
      if (!match) return;
      const key = match[1].trim();
      const raw = match[2].trim();
      const value = raw.endsWith('%') ? Number(raw.slice(0, -1)) / 100 : Number(raw);
      if (Number.isFinite(value)) result[key] = round((result[key] ?? 0) + value, 6);
    });
  return result;
}

export function formatEffectMap(map: EffectMap): string {
  return Object.entries(map)
    .filter(([, value]) => value !== 0)
    .map(([key, value]) => `${key}=${round(value, 6)}`)
    .join(';');
}

export function parseMilestones(input = ''): Array<{ level: number; effects: EffectMap; raw: string }> {
  const rows: Array<{ level: number; effects: EffectMap; raw: string }> = [];
  const normalized = input.replace(/\r/g, '').trim();
  if (!normalized) return rows;
  const parts = normalized.split(/[|\n]+/).map((p) => p.trim()).filter(Boolean);
  for (const part of parts) {
    const match = part.match(/^(\d+)\s*[:：]\s*(.+)$/);
    if (!match) continue;
    rows.push({ level: Number(match[1]), effects: parseEffectText(match[2]), raw: part });
  }
  return rows;
}

export function getAttrValueMap(attributes: AttributeValue[]): Record<string, number> {
  return Object.fromEntries(attributes.map((a) => [a.key, Number(a.unitValue) || 0]));
}

export function effectsScore(effects: EffectMap, attributes: AttributeValue[]): number {
  const valueMap = getAttrValueMap(attributes);
  return Object.entries(effects).reduce((sum, [key, amount]) => {
    const unit = valueMap[key] ?? 0;
    return sum + amount * unit;
  }, 0);
}

export function levelBudget(level: number, constants: Constants): number {
  const safeLevel = Math.max(1, Number(level) || 1);
  return constants.levelBudgetBase + constants.levelBudgetGrowth * safeLevel ** constants.levelBudgetPower;
}

export function rarityMultiplier(state: ModelState, rarity: string): number {
  return state.rarities.find((r) => r.key === rarity)?.multiplier ?? 1;
}

export function slotMultiplier(state: ModelState, slot: string): number {
  return state.slots.find((s) => s.key === slot)?.multiplier ?? 1;
}

export function tierMultiplier(_state: ModelState, _tier: string): number {
  return 1;
}

export function rewardMultiplier(_state: ModelState, _tier: string): number {
  return 1;
}

export function targetFactor(count: number, extraTargetValue: number): number {
  const safeCount = Math.max(1, Number(count) || 1);
  return 1 + (safeCount - 1) * extraTargetValue;
}

function positionFactor(position: string): number {
  const table: Record<string, number> = {
    self: 1,
    front: 1.08,
    mid: 1,
    back: 0.95,
    any: 0.92,
    random: 0.88,
    all: 1.12
  };
  return table[position] ?? 1;
}

function damageTypeFactor(type: string): number {
  const table: Record<string, number> = {
    physical: 1,
    magical: 1.02,
    mixed: 1.08,
    true: 1.35,
    none: 0
  };
  return table[type] ?? 1;
}

export function scoreSkill(skill: SkillModel, state: ModelState): ScoreResult {
  const baseBudget = levelBudget(skill.level, state.constants) * rarityMultiplier(state, skill.rarity);
  const targetValue = targetFactor(skill.targetCount, state.constants.extraSkillTargetValue);
  const hitRate = clamp(Number(skill.hitRate) || 0, 0, 2);
  const damageBudget =
    baseBudget *
    Math.max(0, Number(skill.damageMultiplier) || 0) *
    targetValue *
    positionFactor(skill.targetPosition) *
    damageTypeFactor(skill.damageType) *
    hitRate;

  const effectRaw = effectsScore(parseEffectText(skill.effectsText), state.attributes);
  const durationFactor = skill.effectDurationTurns > 0 ? 1 + (skill.effectDurationTurns - 1) * state.constants.extraTurnValue : 1;
  const effectBudget = Math.abs(effectRaw) * targetValue * durationFactor;

  const mpValue = getAttrValueMap(state.attributes).mp ?? 0.05;
  const mpPenalty = Math.max(0, skill.mpCost) * mpValue * state.constants.mpCostPenalty;
  const cooldownPenalty = baseBudget * Math.max(0, skill.cooldownTurns) * state.constants.cooldownPenaltyPerTurn;
  const delayPenalty = baseBudget * Math.max(0, skill.castDelayTurns) * 0.1;
  const score = Math.max(0, damageBudget + effectBudget - mpPenalty - cooldownPenalty - delayPenalty);

  return {
    score: round(score),
    goldValue: round(score * state.constants.goldPerBudget),
    detail: {
      baseBudget: round(baseBudget),
      damageBudget: round(damageBudget),
      effectBudget: round(effectBudget),
      mpPenalty: round(mpPenalty),
      cooldownPenalty: round(cooldownPenalty),
      delayPenalty: round(delayPenalty),
      targetFactor: round(targetValue)
    }
  };
}

export function scoreBuff(buff: BuffModel, state: ModelState): ScoreResult {
  const effectRaw = Math.abs(effectsScore(parseEffectText(buff.effectsText), state.attributes));
  const durationFactor = 1 + Math.max(0, buff.durationTurns - 1) * state.constants.extraTurnValue;
  const targetValue = targetFactor(buff.targetCount, state.constants.extraBuffTargetValue);
  const stackFactor = 1 + Math.max(0, buff.stackLimit - 1) * 0.35;
  const dispelPenalty = buff.dispellable ? 0.92 : 1;
  const uptime = clamp(Number(buff.uptime) || 1, 0, 1.5);
  const score = effectRaw * durationFactor * targetValue * stackFactor * uptime * dispelPenalty * rarityMultiplier(state, buff.rarity);
  return {
    score: round(score),
    goldValue: round(score * state.constants.goldPerBudget),
    detail: {
      effectRaw: round(effectRaw),
      durationFactor: round(durationFactor),
      targetFactor: round(targetValue),
      stackFactor: round(stackFactor),
      uptime: round(uptime),
      dispelPenalty
    }
  };
}

export function scoreAffix(affix: AffixModel, state: ModelState): ScoreResult {
  const scopeFactor: Record<string, number> = {
    all: 1,
    weapon: 1.1,
    armor: 0.95,
    accessory: 1.05,
    skill: 1.18
  };
  const raw = Math.abs(effectsScore(parseEffectText(affix.effectsText), state.attributes));
  const score = raw * rarityMultiplier(state, affix.rarity) * (scopeFactor[affix.scope] ?? 1);
  return {
    score: round(score),
    goldValue: round(score * state.constants.goldPerBudget),
    detail: {
      raw: round(raw),
      rarityMultiplier: round(rarityMultiplier(state, affix.rarity)),
      scopeFactor: scopeFactor[affix.scope] ?? 1
    }
  };
}

export function scoreEquipment(equipment: EquipmentModel, state: ModelState): ScoreResult {
  const rarity = rarityMultiplier(state, equipment.rarity);
  const slot = slotMultiplier(state, equipment.slot);
  const level = levelBudget(equipment.level, state.constants);
  const baseRaw = Math.abs(effectsScore(parseEffectText(equipment.baseEffectsText), state.attributes));
  const baseBudget = baseRaw * rarity * slot;
  const perLevelRaw = Math.abs(effectsScore(parseEffectText(equipment.enhancePerLevelText), state.attributes));
  const enhanceBudget = perLevelRaw * Math.max(0, equipment.maxEnhanceLevel) * rarity * 0.72;
  const milestones = parseMilestones(equipment.milestonesText);
  const milestoneBudget = milestones.reduce((sum, row) => sum + Math.abs(effectsScore(row.effects, state.attributes)), 0) * rarity * state.constants.milestoneWeight;
  const affixSlotBudget = Math.max(0, equipment.affixSlots) * state.constants.affixSlotBaseBudget * rarity * slot;
  const levelFloorBudget = level * 0.15 * rarity * slot;
  const score = baseBudget + enhanceBudget + milestoneBudget + affixSlotBudget + levelFloorBudget;
  const enhanceCostGold = calcEnhanceCostGold(equipment.maxEnhanceLevel, rarity, slot, state.constants);

  return {
    score: round(score),
    goldValue: round(score * state.constants.goldPerBudget),
    detail: {
      baseRaw: round(baseRaw),
      baseBudget: round(baseBudget),
      enhanceBudget: round(enhanceBudget),
      milestoneBudget: round(milestoneBudget),
      affixSlotBudget: round(affixSlotBudget),
      levelFloorBudget: round(levelFloorBudget),
      enhanceCostGold: round(enhanceCostGold)
    }
  };
}

export function calcEnhanceCostGold(maxLevel: number, rarity: number, slot: number, constants: Constants): number {
  let total = 0;
  for (let i = 1; i <= Math.max(0, maxLevel); i += 1) {
    total += constants.enhanceBaseCostGold * i ** constants.enhanceCostPower * rarity * slot;
  }
  return total;
}

export function generatedEnemyStats(enemy: EnemyModel, state: ModelState): EffectMap {
  const custom = parseEffectText(enemy.customStatsText);
  const stats: EffectMap = {};
  for (const [key, value] of Object.entries(custom)) {
    stats[key] = round(value, isPercentAttr(state.attributes, key) ? 4 : 2);
  }
  return stats;
}

function isPercentAttr(attributes: AttributeValue[], key: string): boolean {
  return Boolean(attributes.find((a) => a.key === key)?.displayAsPercent);
}

export function scoreEnemy(enemy: EnemyModel, state: ModelState): ScoreResult {
  const stats = generatedEnemyStats(enemy, state);
  const statScore = Math.abs(effectsScore(stats, state.attributes));
  const tierReward = rewardMultiplier(state, enemy.tier);
  const waveFactor = Math.max(1, enemy.countInWave || 1) ** 0.72;
  const score = statScore * waveFactor;
  return {
    score: round(score),
    goldValue: round(score * state.constants.goldPerBudget),
    detail: {
      statScore: round(statScore),
      waveFactor: round(waveFactor),
      rewardGold: round(score * state.constants.enemyGoldRewardRate * tierReward),
      rewardExp: round(score * state.constants.enemyExpRewardRate * tierReward),
      tierReward
    }
  };
}

export function effectsToJson(text: string): string {
  return JSON.stringify(parseEffectText(text));
}

export function milestonesToJson(text: string): string {
  return JSON.stringify(parseMilestones(text).map((m) => ({ level: m.level, attributes: m.effects })));
}
