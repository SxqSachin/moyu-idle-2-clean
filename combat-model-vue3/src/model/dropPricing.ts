import { generateEnemyStats, type CombatBalanceModel } from './balance';
import { asArray, asNumber, asString, isRecord, rowId, type Catalog, type DataRow } from './catalog';
import { scoreRealEnemy, type ScoringConfig } from './realScoring';
import { round } from './scoring';

export interface DropPricingSourceRow {
  itemId: string;
  itemName: string;
  rarity: string;
  catalogRarity: string;
  pricingRarity: string;
  skillbookAnchorEnemyType: string;
  worldId: string;
  enemyId: string;
  enemyType: string;
  enemyLevel: number;
  enemyTypeWeight: number;
  enemyLevelPriceMultiplier: number;
  expectedEnemyCount: number;
  expectedDropCount: number;
  dropPercent: number;
  dropAverageCount: number;
  enemyActualScore: number;
  enemyTemplateScore: number;
  enemyTemplateDeviation: number | null;
  enemyBudgetGold: number;
  sourceUnitPrice: number;
  cappedUnitPrice: number;
  riskFlags: string[];
}

export interface DropPricingItemPrice {
  itemId: string;
  itemName: string;
  rarity: string;
  catalogRarity: string;
  pricingRarity: string;
  skillbookAnchorEnemyType: string;
  currentValue: number;
  suggestedValue: number;
  delta: number;
  deltaRatio: number | null;
  expectedDropCount: number;
  sourceCount: number;
  mainWorldId: string;
  mainEnemyId: string;
  mainEnemyLevel: number;
  mainEnemyTypeWeight: number;
  mainEnemyLevelPriceMultiplier: number;
  mainDropPercent: number;
  mainEnemyActualScore: number;
  mainEnemyTemplateScore: number;
  mainEnemyTemplateDeviation: number | null;
  riskFlags: string[];
}

export interface DropPricingFitGroup {
  key: string;
  count: number;
  mae: number;
  mape: number;
  overpricedCount: number;
}

export interface DropPricingFitSummary {
  pricedCount: number;
  comparableCount: number;
  placeholderCount: number;
  mae: number;
  mape: number;
  overpricedCount: number;
  groups: DropPricingFitGroup[];
}

export interface DropPricingReport {
  itemPrices: DropPricingItemPrice[];
  sourceRows: DropPricingSourceRow[];
  fitSummary: DropPricingFitSummary;
  riskRows: DropPricingItemPrice[];
}

type EnemyWeight = {
  enemyId: string;
  count: number;
};

function asStringList(value: unknown): string[] {
  if (Array.isArray(value)) return value.flatMap((entry) => asStringList(entry));
  return asString(value)
    .split(',')
    .map((entry) => entry.trim())
    .filter(Boolean);
}

function averageRange(value: unknown, fallback = 0): number {
  if (!isRecord(value)) return fallback;
  const min = Math.max(0, asNumber(value.min, fallback));
  const max = Math.max(min, asNumber(value.max, min));
  return (min + max) / 2;
}

function dropAverageCount(rule: unknown): number {
  if (typeof rule === 'number') return Math.max(0, rule);
  if (!isRecord(rule)) return 0;
  if (isRecord(rule.range)) return averageRange(rule.range, 0);
  return Math.max(0, asNumber(rule.count, asNumber(rule.value, 1)));
}

function dropPercent(rule: unknown): number {
  if (typeof rule === 'number') return rule > 0 ? 1 : 0;
  if (!isRecord(rule)) return 0;
  return Math.max(0, asNumber(rule.percent, 1));
}

function worldSortIndex(worldId: string, fallbackIndex: number): number {
  const baseMatch = worldId.match(/base[_-]?(\d+)/i);
  if (baseMatch) return Math.max(1, Number(baseMatch[1]) || fallbackIndex + 1);
  const numberMatch = worldId.match(/(\d+)/);
  if (numberMatch) return Math.max(1, Number(numberMatch[1]) || fallbackIndex + 1);
  return fallbackIndex + 1;
}

function specialWaveProbability(world: DataRow, horizon: number): number {
  const raw = world.specialWaves;
  if (!isRecord(raw)) {
    const fixed = asArray(raw).length;
    return fixed > 0 ? Math.min(1, fixed / Math.max(1, horizon)) : 0;
  }
  const every = Math.max(0, Math.trunc(asNumber(raw.every, 0)));
  if (every > 0) return Math.min(1, 1 / every);
  const fixedWaves = Object.keys(raw)
    .map((key) => Number(key))
    .filter((value) => Number.isFinite(value) && value > 0 && value <= horizon).length;
  return fixedWaves > 0 ? Math.min(1, fixedWaves / Math.max(1, horizon)) : 0;
}

function specialWaveAverageCount(world: DataRow): number {
  const raw = world.specialWaves;
  if (!isRecord(raw)) return 1;
  const min = Math.max(1, asNumber(raw.minCount, 1));
  const max = Math.max(min, asNumber(raw.maxCount, min));
  return (min + max) / 2;
}

function addEnemyWeight(map: Map<string, number>, enemyId: string, count: number) {
  if (!enemyId || count <= 0) return;
  map.set(enemyId, (map.get(enemyId) ?? 0) + count);
}

function customLineupProbability(conf: unknown): number {
  if (!isRecord(conf)) return 0;
  return Math.max(0, Math.min(1, asNumber(conf.percent, 0)));
}

function customLineupIDs(conf: unknown): string[] {
  if (!isRecord(conf)) return [];
  return asStringList(conf.lineup);
}

function customLineupIsSpecial(conf: unknown): boolean {
  return isRecord(conf) && Boolean(conf.isSpecialWaveLineup);
}

function expectedEnemyWeights(world: DataRow, enemyById: Map<string, DataRow>, config: ScoringConfig): EnemyWeight[] {
  const out = new Map<string, number>();
  const normalIds = asStringList(world.enemyIds).filter((id) => enemyById.get(id)?.type !== 'world_boss');
  const specialIds = asStringList(world.specialEnemyIds).filter((id) => enemyById.get(id)?.type !== 'world_boss');
  const maxTeamSize = Math.max(1, Math.trunc(asNumber(world.maxTeamSize, config.dropPricing.defaultPlayerCount)));
  const playerCount = Math.max(1, Math.min(maxTeamSize, Math.trunc(config.dropPricing.defaultPlayerCount)));
  const normalCount = Math.max(0, averageRange(world.normalEnemyCount, 1)) +
    Math.max(0, playerCount - 1) * Math.max(0, averageRange(world.extraEnemyPerPlayer, 0));
  const specialProbability = specialWaveProbability(world, config.dropPricing.specialWaveHorizon);
  const specialCount = specialWaveAverageCount(world) * specialProbability;
  let normalReplacementProbability = 0;
  let specialReplacementProbability = 0;

  for (const conf of asArray(world.customLineupConf)) {
    const ids = customLineupIDs(conf).filter((id) => enemyById.get(id)?.type !== 'world_boss');
    if (!ids.length) continue;
    const probability = customLineupProbability(conf);
    if (probability <= 0) continue;
    const weightedProbability = customLineupIsSpecial(conf) ? probability * specialProbability : probability * (1 - specialProbability);
    if (weightedProbability <= 0) continue;
    if (customLineupIsSpecial(conf)) specialReplacementProbability += probability;
    else normalReplacementProbability += probability;
    for (const id of ids) addEnemyWeight(out, id, weightedProbability / ids.length);
  }

  const normalBaseProbability = Math.max(0, 1 - specialProbability) * Math.max(0, 1 - Math.min(1, normalReplacementProbability));
  const specialBaseProbability = specialProbability * Math.max(0, 1 - Math.min(1, specialReplacementProbability));
  for (const id of normalIds) addEnemyWeight(out, id, normalBaseProbability * normalCount / Math.max(1, normalIds.length));
  for (const id of normalIds) addEnemyWeight(out, id, specialBaseProbability * normalCount / Math.max(1, normalIds.length));
  for (const id of specialIds) addEnemyWeight(out, id, specialBaseProbability * specialCount / Math.max(1, specialIds.length));

  return Array.from(out.entries()).map(([enemyId, count]) => ({ enemyId, count })).filter((row) => row.count > 0);
}

function itemName(item: DataRow | undefined, itemId: string): string {
  return asString(item?.name || item?.nameKey, itemId) || itemId;
}

function itemTypeIncludes(item: DataRow | undefined, target: string): boolean {
  return asArray(item?.type).map((value) => asString(value).trim()).includes(target);
}

function enemyTypeRank(type: string): number {
  const order: Record<string, number> = {
    normal: 1,
    elite: 2,
    boss: 3,
    world_boss: 4
  };
  return order[type] ?? 1;
}

function rarityForEnemyType(type: string): string {
  if (type === 'world_boss') return 'legendary';
  if (type === 'boss') return 'epic';
  if (type === 'elite') return 'rare';
  return 'common';
}

function enemyTypeWeight(config: ScoringConfig, type: string): number {
  return Math.max(0, config.dropPricing.enemyTypeWeights?.[type] ?? config.dropPricing.enemyTypeWeights?.normal ?? 1);
}

function enemyLevelPriceMultiplier(config: ScoringConfig, level: number): number {
  const raw = 1 + Math.max(0, Math.trunc(level) - 1) * Math.max(0, config.dropPricing.enemyLevelPriceGrowth);
  return Math.min(Math.max(1, config.dropPricing.enemyLevelPriceMaxMultiplier), raw);
}

function buildSkillbookAnchorMap(catalog: Catalog): Map<string, { enemyType: string; rarity: string }> {
  const out = new Map<string, { enemyType: string; rarity: string }>();
  for (const enemy of catalog.enemies) {
    const enemyType = asString(enemy.type, 'normal') || 'normal';
    if (enemyType === 'world_boss') continue;
    for (const itemId of Object.keys(isRecord(enemy.drops) ? enemy.drops : {})) {
      if (!itemId.startsWith('skillbook_')) continue;
      const current = out.get(itemId);
      if (!current || enemyTypeRank(enemyType) < enemyTypeRank(current.enemyType)) {
        out.set(itemId, { enemyType, rarity: rarityForEnemyType(enemyType) });
      }
    }
  }
  return out;
}

function rarityRule(config: ScoringConfig, rarity: string) {
  return config.dropPricing.rarityRules.find((row) => row.key === rarity) ?? {
    key: rarity,
    weight: 1,
    softFloor: 0,
    hardCap: Number.POSITIVE_INFINITY
  };
}

function clampUnitPrice(raw: number, rarity: string, worldIndex: number, levelMultiplier: number, config: ScoringConfig): number {
  const rule = rarityRule(config, rarity);
  const floorMultiplier = Math.min(
    Math.max(1, config.dropPricing.worldFloorMaxMultiplier),
    1 + Math.max(0, worldIndex - 1) * Math.max(0, config.dropPricing.worldFloorGrowth)
  );
  const floor = rule.softFloor * floorMultiplier * levelMultiplier;
  const cap = rule.hardCap > 0 ? rule.hardCap * levelMultiplier : Number.POSITIVE_INFINITY;
  return Math.min(cap, Math.max(floor, raw));
}

function enemyScoreWithoutDrops(enemy: DataRow, catalog: Catalog, config: ScoringConfig): number {
  return scoreRealEnemy({ ...enemy, drops: {} }, config, catalog).score;
}

function tierKeyForEnemy(enemy: DataRow, config: ScoringConfig): string {
  const type = asString(enemy.type, 'normal') || 'normal';
  return config.enemyAttributeTemplates.tiers.find((tier) => tier.enemyType === type || tier.key === type)?.key ?? type;
}

function templateScoreForEnemy(enemy: DataRow, catalog: Catalog, config: ScoringConfig, model: CombatBalanceModel): number {
  const level = Math.max(1, Math.trunc(asNumber(enemy.level, 1)));
  const tierKey = tierKeyForEnemy(enemy, config);
  const roleKey = asString(enemy.class).trim();
  const speciesKey = asString(enemy.race).trim();
  if (!roleKey || !speciesKey) return 0;
  const stats = generateEnemyStats(level, tierKey, roleKey, speciesKey, model, config);
  if (Object.keys(stats).length === 0) return 0;
  const templateRow: DataRow = {
    ...enemy,
    ...stats,
    level,
    type: asString(enemy.type, 'normal'),
    drops: {}
  };
  return scoreRealEnemy(templateRow, config, catalog).score;
}

function riskFlagsForSource(
  currentValue: number,
  suggestedValue: number,
  templateDeviation: number | null,
  config: ScoringConfig
): string[] {
  const flags: string[] = [];
  if (currentValue <= 0) flags.push('未定价');
  else if (currentValue <= config.dropPricing.ignoreExistingPriceBelow) flags.push('占位价');
  else if (suggestedValue > currentValue * config.dropPricing.overpricedWarnRatio) flags.push('建议过高');
  if (templateDeviation != null && Math.abs(templateDeviation) > config.dropPricing.enemyTemplateDeviationWarnRatio) flags.push('敌人偏离模板');
  return flags;
}

function fitSummary(rows: DropPricingItemPrice[], config: ScoringConfig): DropPricingFitSummary {
  const priced = rows.filter((row) => row.currentValue > 0);
  const comparable = priced.filter((row) => row.currentValue > config.dropPricing.ignoreExistingPriceBelow);
  const placeholderCount = priced.length - comparable.length;
  const groups = new Map<string, DropPricingItemPrice[]>();
  for (const row of comparable) {
    if (!groups.has(row.rarity)) groups.set(row.rarity, []);
    groups.get(row.rarity)!.push(row);
  }
  const summarize = (items: DropPricingItemPrice[], key: string): DropPricingFitGroup => {
    const count = items.length;
    const mae = count > 0 ? items.reduce((sum, row) => sum + Math.abs(row.suggestedValue - row.currentValue), 0) / count : 0;
    const mape = count > 0 ? items.reduce((sum, row) => sum + Math.abs(row.suggestedValue - row.currentValue) / Math.max(1, row.currentValue), 0) / count : 0;
    const overpricedCount = items.filter((row) => row.suggestedValue > row.currentValue * config.dropPricing.overpricedWarnRatio).length;
    return { key, count, mae: round(mae), mape: round(mape * 100, 1), overpricedCount };
  };
  const all = summarize(comparable, 'all');
  return {
    pricedCount: priced.length,
    comparableCount: comparable.length,
    placeholderCount,
    mae: all.mae,
    mape: all.mape,
    overpricedCount: all.overpricedCount,
    groups: Array.from(groups.entries()).map(([key, items]) => summarize(items, key)).sort((a, b) => a.key.localeCompare(b.key))
  };
}

export function buildDropPricingReport(catalog: Catalog, config: ScoringConfig, model: CombatBalanceModel): DropPricingReport {
  const itemById = new Map<string, DataRow>();
  for (const item of catalog.items) {
    const id = rowId(item);
    if (id) itemById.set(id, item);
  }
  const enemyById = new Map<string, DataRow>();
  for (const enemy of catalog.enemies) {
    const id = rowId(enemy);
    if (id) enemyById.set(id, enemy);
  }
  const skillbookAnchorByItem = buildSkillbookAnchorMap(catalog);
  const enemyScores = new Map<string, { actual: number; template: number; deviation: number | null }>();
  for (const enemy of catalog.enemies) {
    const id = rowId(enemy);
    if (!id) continue;
    const actual = enemyScoreWithoutDrops(enemy, catalog, config);
    const template = templateScoreForEnemy(enemy, catalog, config, model);
    enemyScores.set(id, {
      actual,
      template,
      deviation: template > 0 ? (actual - template) / template : null
    });
  }

  const sourceRows: DropPricingSourceRow[] = [];
  catalog.world.forEach((world, worldArrayIndex) => {
    const worldId = rowId(world, `world_${worldArrayIndex + 1}`);
    const weights = expectedEnemyWeights(world, enemyById, config);
    if (!weights.length) return;
    const weightedEnemyScore = weights.reduce((sum, row) => sum + row.count * Math.max(0, enemyScores.get(row.enemyId)?.actual ?? 0), 0);
    if (weightedEnemyScore <= 0) return;
    const weightedEnemyDropScore = weights.reduce((sum, row) => {
      const enemy = enemyById.get(row.enemyId);
      const type = asString(enemy?.type, 'normal') || 'normal';
      if (type === 'world_boss') return sum;
      return sum + row.count * Math.max(0, enemyScores.get(row.enemyId)?.actual ?? 0) * enemyTypeWeight(config, type);
    }, 0);
    if (weightedEnemyDropScore <= 0) return;
    const estimatedSeconds = Math.max(0, asNumber(world.entryDurationSeconds, 0)) +
      weightedEnemyScore / Math.max(0.0001, config.dropPricing.waveScorePerSecond);
    const waveBudget = estimatedSeconds * Math.max(0, config.dropPricing.targetGoldPerSecond) * Math.max(0, config.dropPricing.dropBudgetShare);
    const worldIndex = worldSortIndex(worldId, worldArrayIndex);
    for (const weight of weights) {
      const enemy = enemyById.get(weight.enemyId);
      if (!enemy || asString(enemy.type) === 'world_boss') continue;
      const type = asString(enemy.type, 'normal') || 'normal';
      const enemyLevel = Math.max(1, Math.trunc(asNumber(enemy.level, 1)));
      const typeWeight = enemyTypeWeight(config, type);
      const levelMultiplier = enemyLevelPriceMultiplier(config, enemyLevel);
      const enemyScore = Math.max(0, enemyScores.get(weight.enemyId)?.actual ?? 0);
      const enemyBudgetGold = waveBudget * (weight.count * enemyScore * typeWeight) / weightedEnemyDropScore;
      const dropEntries = Object.entries(isRecord(enemy.drops) ? enemy.drops : {})
        .filter(([itemId]) => {
          const item = itemById.get(itemId);
          return itemId !== 'gold_coin' && !itemTypeIncludes(item, 'currency');
        })
        .map(([itemId, rule]) => {
          const item = itemById.get(itemId);
          const catalogRarity = asString(item?.rarity, 'common') || 'common';
          const skillbookAnchor = skillbookAnchorByItem.get(itemId);
          const rarity = itemTypeIncludes(item, 'skillbook') && skillbookAnchor ? skillbookAnchor.rarity : catalogRarity;
          const percent = dropPercent(rule);
          const averageCount = dropAverageCount(rule);
          const expectedDropCount = percent * averageCount;
          const weighted = expectedDropCount * Math.max(0, rarityRule(config, rarity).weight);
          return { itemId, item, catalogRarity, rarity, skillbookAnchor, percent, averageCount, expectedDropCount, weighted };
        })
        .filter((drop) => drop.expectedDropCount > 0 && drop.weighted > 0);
      const dropWeightTotal = dropEntries.reduce((sum, drop) => sum + drop.weighted, 0);
      if (dropWeightTotal <= 0) continue;
      for (const drop of dropEntries) {
        const item = drop.item;
        const currentValue = Math.max(0, asNumber(item?.economyBaseValue, 0));
        const sourceGold = enemyBudgetGold * drop.weighted / dropWeightTotal;
        const sourceExpectedDropCount = drop.expectedDropCount * weight.count;
        const sourceUnitPrice = sourceGold / Math.max(0.000001, sourceExpectedDropCount);
        const cappedUnitPrice = clampUnitPrice(sourceUnitPrice, drop.rarity, worldIndex, levelMultiplier, config);
        const template = enemyScores.get(weight.enemyId);
        const riskFlags = riskFlagsForSource(currentValue, cappedUnitPrice, template?.deviation ?? null, config);
        sourceRows.push({
          itemId: drop.itemId,
          itemName: itemName(item, drop.itemId),
          rarity: drop.rarity,
          catalogRarity: drop.catalogRarity,
          pricingRarity: drop.rarity,
          skillbookAnchorEnemyType: drop.skillbookAnchor?.enemyType ?? '',
          worldId,
          enemyId: weight.enemyId,
          enemyType: type,
          enemyLevel,
          enemyTypeWeight: round(typeWeight, 4),
          enemyLevelPriceMultiplier: round(levelMultiplier, 4),
          expectedEnemyCount: round(weight.count, 4),
          expectedDropCount: round(sourceExpectedDropCount, 6),
          dropPercent: round(drop.percent, 6),
          dropAverageCount: round(drop.averageCount, 4),
          enemyActualScore: round(enemyScore),
          enemyTemplateScore: round(template?.template ?? 0),
          enemyTemplateDeviation: template?.deviation == null ? null : round(template.deviation, 4),
          enemyBudgetGold: round(enemyBudgetGold, 2),
          sourceUnitPrice: round(sourceUnitPrice, 2),
          cappedUnitPrice: round(cappedUnitPrice, 2),
          riskFlags
        });
      }
    }
  });

  const byItem = new Map<string, { rows: DropPricingSourceRow[]; expectedTotal: number }>();
  for (const row of sourceRows) {
    const bucket = byItem.get(row.itemId) ?? { rows: [], expectedTotal: 0 };
    bucket.rows.push(row);
    bucket.expectedTotal += row.expectedDropCount;
    byItem.set(row.itemId, bucket);
  }

  const itemPrices = Array.from(byItem.entries()).map(([itemId, bucket]) => {
    const item = itemById.get(itemId);
    const catalogRarity = asString(item?.rarity, bucket.rows[0]?.catalogRarity || 'common') || 'common';
    const skillbookAnchor = skillbookAnchorByItem.get(itemId);
    const rarity = itemTypeIncludes(item, 'skillbook') && skillbookAnchor ? skillbookAnchor.rarity : catalogRarity;
    const currentValue = Math.max(0, asNumber(item?.economyBaseValue, 0));
    const main = [...bucket.rows].sort((a, b) =>
      a.cappedUnitPrice - b.cappedUnitPrice ||
      enemyTypeRank(a.enemyType) - enemyTypeRank(b.enemyType) ||
      a.enemyLevel - b.enemyLevel ||
      b.expectedDropCount - a.expectedDropCount
    )[0];
    const suggestedValue = round(main?.cappedUnitPrice ?? 0);
    const mergedFlags = Array.from(new Set(main?.riskFlags ?? []));
    const delta = suggestedValue - currentValue;
    const deltaRatio = currentValue > 0 ? delta / currentValue : null;
    return {
      itemId,
      itemName: itemName(item, itemId),
      rarity,
      catalogRarity,
      pricingRarity: rarity,
      skillbookAnchorEnemyType: skillbookAnchor?.enemyType ?? '',
      currentValue: round(currentValue),
      suggestedValue,
      delta: round(delta),
      deltaRatio: deltaRatio == null ? null : round(deltaRatio, 4),
      expectedDropCount: round(bucket.expectedTotal, 6),
      sourceCount: bucket.rows.length,
      mainWorldId: main?.worldId ?? '',
      mainEnemyId: main?.enemyId ?? '',
      mainEnemyLevel: main?.enemyLevel ?? 0,
      mainEnemyTypeWeight: main?.enemyTypeWeight ?? 1,
      mainEnemyLevelPriceMultiplier: main?.enemyLevelPriceMultiplier ?? 1,
      mainDropPercent: main?.dropPercent ?? 0,
      mainEnemyActualScore: main?.enemyActualScore ?? 0,
      mainEnemyTemplateScore: main?.enemyTemplateScore ?? 0,
      mainEnemyTemplateDeviation: main?.enemyTemplateDeviation ?? null,
      riskFlags: mergedFlags
    } satisfies DropPricingItemPrice;
  }).sort((a, b) => {
    const riskDelta = b.riskFlags.length - a.riskFlags.length;
    if (riskDelta) return riskDelta;
    return b.suggestedValue - a.suggestedValue;
  });

  return {
    itemPrices,
    sourceRows,
    fitSummary: fitSummary(itemPrices, config),
    riskRows: itemPrices.filter((row) => row.riskFlags.length > 0)
  };
}
