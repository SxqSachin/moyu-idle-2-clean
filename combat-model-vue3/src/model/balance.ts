import { asArray, asNumber, asString, isRecord, rowId, type Catalog, type CombatTable, type DataRow } from './catalog';
import { scoreRow, type ScoringConfig } from './realScoring';
import { round } from './scoring';

export interface CombatBalanceLevelBand {
  level: number;
  playerDpr: number;
  playerHp: number;
  playerMp: number;
  playerSpeed: number;
  expRate: number;
}

export interface CombatBalanceModel {
  version: number;
  maxBattleLevel: number;
  primaryPointCap: number;
  primaryPointAttrs: string[];
  defenseK: number;
  enemyHpRegenRatio: number;
  percentPenetrationCap: number;
  expPerPower: number;
  teamExtraEnemyScale: number;
  teamRewardEfficiencyCap: number;
  comboBudgetMinShare: number;
  comboBudgetMaxShare: number;
  levelBands: CombatBalanceLevelBand[];
}

export interface BalanceHint {
  severity: 'ok' | 'info' | 'warn' | 'danger';
  title: string;
  detail: string;
}

export interface EnemyBalanceSuggestion {
  powerScore: number;
  recommendedExp: number;
  expDeltaPct: number;
  recommendedHpMin: number;
  recommendedHpMax: number;
  estimatedRounds: number;
  pressurePct: number;
  defenseK: number;
}

export interface BalanceDiagnostics {
  table: CombatTable;
  id: string;
  score: number;
  hints: BalanceHint[];
  enemy?: EnemyBalanceSuggestion;
}

export const DEFAULT_COMBAT_BALANCE_MODEL: CombatBalanceModel = {
  version: 1,
  maxBattleLevel: 500,
  primaryPointCap: 500,
  primaryPointAttrs: ['strength', 'dexterity', 'intelligence', 'perception', 'stamina'],
  defenseK: 100,
  enemyHpRegenRatio: 0.34,
  percentPenetrationCap: 0.8,
  expPerPower: 0.018,
  teamExtraEnemyScale: 0.45,
  teamRewardEfficiencyCap: 1.15,
  comboBudgetMinShare: 0.08,
  comboBudgetMaxShare: 0.18,
  levelBands: [
    { level: 1,   playerDpr: 6,    playerHp: 85,    playerMp: 20,   playerSpeed: 5,   expRate: 1 },
    { level: 25,  playerDpr: 60,   playerHp: 700,   playerMp: 150,  playerSpeed: 15,  expRate: 1.1 },
    { level: 50,  playerDpr: 180,  playerHp: 2200,  playerMp: 400,  playerSpeed: 28,  expRate: 1.2 },
    { level: 75,  playerDpr: 380,  playerHp: 4200,  playerMp: 700,  playerSpeed: 42,  expRate: 1.35 },
    { level: 100, playerDpr: 650,  playerHp: 6800,  playerMp: 1050, playerSpeed: 58,  expRate: 1.55 },
    { level: 150, playerDpr: 1200, playerHp: 14000, playerMp: 2500, playerSpeed: 115, expRate: 4.2 },
    { level: 200, playerDpr: 1100, playerHp: 18000, playerMp: 3200, playerSpeed: 135, expRate: 4.5 },
    { level: 300, playerDpr: 2100, playerHp: 30000, playerMp: 5000, playerSpeed: 175, expRate: 5.0 },
    { level: 400, playerDpr: 3200, playerHp: 42000, playerMp: 6800, playerSpeed: 215, expRate: 5.5 },
    { level: 500, playerDpr: 4500, playerHp: 55000, playerMp: 8500, playerSpeed: 255, expRate: 6.0 }
  ]
};

function parseLevelBand(raw: unknown): CombatBalanceLevelBand | null {
  if (!isRecord(raw)) return null;
  const level = Math.round(asNumber(raw.level, 0));
  const playerDpr = asNumber(raw.playerDpr, 0);
  const playerHp = asNumber(raw.playerHp, 0);
  if (level <= 0 || playerDpr <= 0 || playerHp <= 0) return null;
  return {
    level,
    playerDpr,
    playerHp,
    playerMp: Math.max(0, asNumber(raw.playerMp, 0)),
    playerSpeed: Math.max(0, asNumber(raw.playerSpeed, 0)),
    expRate: Math.max(0.0001, asNumber(raw.expRate, 1))
  };
}

export function parseCombatBalanceModel(raw: unknown): CombatBalanceModel {
  const source = isRecord(raw) ? raw : {};
  const fallback = DEFAULT_COMBAT_BALANCE_MODEL;
  const levelBands = asArray(source.levelBands).map(parseLevelBand).filter(Boolean) as CombatBalanceLevelBand[];
  const comboBudgetMinShare = Math.max(0, asNumber(source.comboBudgetMinShare, fallback.comboBudgetMinShare));
  return {
    version: Math.max(1, Math.round(asNumber(source.version, fallback.version))),
    maxBattleLevel: Math.max(1, Math.round(asNumber(source.maxBattleLevel, fallback.maxBattleLevel))),
    primaryPointCap: Math.max(1, Math.round(asNumber(source.primaryPointCap, fallback.primaryPointCap))),
    primaryPointAttrs: asArray(source.primaryPointAttrs).map((value) => asString(value).trim()).filter(Boolean).length
      ? asArray(source.primaryPointAttrs).map((value) => asString(value).trim()).filter(Boolean)
      : [...fallback.primaryPointAttrs],
    defenseK: Math.max(1, asNumber(source.defenseK, fallback.defenseK)),
    enemyHpRegenRatio: Math.max(0, Math.min(1, asNumber(source.enemyHpRegenRatio, fallback.enemyHpRegenRatio))),
    percentPenetrationCap: Math.max(0, Math.min(1, asNumber(source.percentPenetrationCap, fallback.percentPenetrationCap))),
    expPerPower: Math.max(0.0001, asNumber(source.expPerPower, fallback.expPerPower)),
    teamExtraEnemyScale: Math.max(0, asNumber(source.teamExtraEnemyScale, fallback.teamExtraEnemyScale)),
    teamRewardEfficiencyCap: Math.max(1, asNumber(source.teamRewardEfficiencyCap, fallback.teamRewardEfficiencyCap)),
    comboBudgetMinShare,
    comboBudgetMaxShare: Math.max(comboBudgetMinShare, asNumber(source.comboBudgetMaxShare, fallback.comboBudgetMaxShare)),
    levelBands: (levelBands.length ? levelBands : fallback.levelBands).sort((a, b) => a.level - b.level)
  };
}

export function serializeCombatBalanceModel(model: CombatBalanceModel): CombatBalanceModel {
  return parseCombatBalanceModel(model);
}

export function baselineAtLevel(model: CombatBalanceModel, level: number): CombatBalanceLevelBand {
  const bands = model.levelBands;
  const safeLevel = Math.max(1, level || 1);
  if (safeLevel <= bands[0].level) return { ...bands[0], level: safeLevel };
  const last = bands[bands.length - 1];
  if (safeLevel >= last.level) {
    const prev = bands[Math.max(0, bands.length - 2)] ?? last;
    const span = Math.max(1, last.level - prev.level);
    const growth = Math.max(1.01, last.playerDpr / Math.max(1, prev.playerDpr)) ** (1 / span);
    const hpGrowth = Math.max(1.01, last.playerHp / Math.max(1, prev.playerHp)) ** (1 / span);
    const delta = safeLevel - last.level;
    return {
      level: safeLevel,
      playerDpr: round(last.playerDpr * growth ** delta),
      playerHp: round(last.playerHp * hpGrowth ** delta),
      playerMp: round(last.playerMp * growth ** (delta * 0.45)),
      playerSpeed: round(last.playerSpeed * growth ** (delta * 0.25)),
      expRate: round(last.expRate * growth ** (delta * 0.35), 4)
    };
  }
  const upper = bands.find((band) => band.level >= safeLevel) ?? last;
  const lower = bands[Math.max(0, bands.indexOf(upper) - 1)] ?? upper;
  const t = (safeLevel - lower.level) / Math.max(1, upper.level - lower.level);
  const lerp = (a: number, b: number) => round(a + (b - a) * t, 4);
  return {
    level: safeLevel,
    playerDpr: lerp(lower.playerDpr, upper.playerDpr),
    playerHp: lerp(lower.playerHp, upper.playerHp),
    playerMp: lerp(lower.playerMp, upper.playerMp),
    playerSpeed: lerp(lower.playerSpeed, upper.playerSpeed),
    expRate: lerp(lower.expRate, upper.expRate)
  };
}


function enemyTierReward(config: ScoringConfig, type: string): number {
  return config.enemyAttributeTemplates.tiers.find((tier) => tier.key === type)?.rewardMult ?? 1;
}

function enemyStats(row: DataRow): Record<string, number> {
  const out: Record<string, number> = {};
  for (const [key, value] of Object.entries(row)) {
    const parsed = Number(value);
    if (Number.isFinite(parsed)) out[key] = parsed;
  }
  return out;
}

function addHint(hints: BalanceHint[], severity: BalanceHint['severity'], title: string, detail: string) {
  hints.push({ severity, title, detail });
}

export function generateEnemyStats(
  level: number,
  tierKey: string,
  roleKey: string,
  speciesKey: string,
  model: CombatBalanceModel,
  scoring: ScoringConfig
): Record<string, number> {
  const baseline = baselineAtLevel(model, level);
  const tier = scoring.enemyAttributeTemplates.tiers.find((t) => t.key === tierKey);
  const role = scoring.enemyAttributeTemplates.roles.find((r) => r.key === roleKey);
  const species = scoring.enemyAttributeTemplates.species.find((s) => s.key === speciesKey);
  if (!tier || !role) return {};

  const merged: Record<string, number> = {};
  for (const [k, v] of Object.entries(role.weights)) merged[k] = Math.max(0, v);
  if (species) {
    for (const [k, v] of Object.entries(species.weights)) {
      merged[k] = Math.max(0, (merged[k] ?? 0) + v);
    }
  }

  // 计算基础属性分（受等级和 tier 倍率影响）
  const baseBudget = Math.max(1, Math.floor(Math.pow(level, 1.2)));
  const budgetMult = tier.budgetMultiplier ?? 1;
  const totalBudget = baseBudget * budgetMult;

  const attrUnitValues = Object.fromEntries(scoring.attributes.map((a) => [a.key, a.unitValue]));

  // 按权重分配属性点到各属性
  const result: Record<string, number> = {};
  for (const [stat, weight] of Object.entries(merged)) {
    if (['hp', 'physics_damage', 'magical_damage'].includes(stat)) continue;
    const unitValue = attrUnitValues[stat] ?? 1;
    result[stat] = Math.round(totalBudget * weight / unitValue);
  }

  // 应用 role/species 的 baseAttrs 和 multipliers
  if (role) {
    for (const [stat, base] of Object.entries(role.baseAttrs ?? {})) {
      result[stat] = (result[stat] ?? 0) + Math.round(base);
    }
    for (const [stat, mult] of Object.entries(role.multipliers ?? {})) {
      if (result[stat] != null) result[stat] = Math.round(result[stat] * mult);
    }
  }
  if (species) {
    for (const [stat, base] of Object.entries(species.baseAttrs ?? {})) {
      result[stat] = (result[stat] ?? 0) + Math.round(base);
    }
    for (const [stat, mult] of Object.entries(species.multipliers ?? {})) {
      if (result[stat] != null) result[stat] = Math.round(result[stat] * mult);
    }
  }

  // 计算派生值
  const str = result.strength ?? 0;
  const dex = result.dexterity ?? 0;
  const int = result.intelligence ?? 0;
  const per = result.perception ?? 0;
  const sta = result.stamina ?? 0;
  const derivedHp = sta * 9 + str * 1.2;
  const derivedPhysicsDamage = str * 0.55 + dex * 0.25;
  const derivedMagicalDamage = int * 0.7 + per * 0.3;

  // 计算目标 HP 和 DPS（用于确定基础 hp/damage 值）
  const midRounds = (tier.targetRounds.min + tier.targetRounds.max) / 2;
  const hpMult = tier.hpMultiplier ?? 1;
  const targetHp = baseline.playerDpr * midRounds * budgetMult * hpMult;
  const targetDps = baseline.playerHp * tier.pressurePct * budgetMult;
  const physW = merged.physics_damage ?? 0;
  const magW = merged.magical_damage ?? 0;
  const totalDpsW = physW + magW || 1;

  // 计算基础值（目标值 - 派生值）
  const targetPhysicsDamage = targetDps * physW / totalDpsW;
  const targetMagicalDamage = targetDps * magW / totalDpsW;
  result.physics_damage = Math.max(1, Math.round(targetPhysicsDamage - derivedPhysicsDamage));
  result.magical_damage = Math.max(1, Math.round(targetMagicalDamage - derivedMagicalDamage));
  result.hp = Math.max(1, Math.round(targetHp - derivedHp));

  // 应用派生属性最低值保证
  const minimums = scoring.enemyAttributeTemplates.derivedStatMinimums ?? {};
  for (const [stat, min] of Object.entries(minimums)) {
    if (result[stat] != null && result[stat] < min) {
      result[stat] = Math.round(min);
    }
  }
  return result;
}

export function diagnoseRowBalance(table: CombatTable, row: DataRow, catalog: Catalog, scoring: ScoringConfig, model: CombatBalanceModel): BalanceDiagnostics {
  const score = scoreRow(table, row, catalog, scoring);
  const hints: BalanceHint[] = [];
  const id = rowId(row, 'unknown');
  if (table === 'enemies') {
    const level = Math.max(1, asNumber(row.level, 1));
    const type = asString(row.type, 'normal') || 'normal';
    const baseline = baselineAtLevel(model, level);
    const tier = scoring.enemyAttributeTemplates.tiers.find((t) => t.key === type);
    const target = tier?.targetRounds ?? { min: 4, max: 8 };
    const pressureThreshold = (tier?.pressurePct ?? 0.18) * 1.5;
    const targetMid = (target.min + target.max) / 2;
    const stats = enemyStats(row);

    // 计算最终属性（基础 + 派生）
    const str = stats.strength ?? 0;
    const dex = stats.dexterity ?? 0;
    const int = stats.intelligence ?? 0;
    const per = stats.perception ?? 0;
    const sta = stats.stamina ?? 0;
    const baseHp = stats.hp ?? 0;
    const basePhysicsDamage = stats.physics_damage ?? 0;
    const baseMagicalDamage = stats.magical_damage ?? 0;

    const derivedHp = sta * 9 + str * 1.2;
    const derivedPhysicsDamage = str * 0.55 + dex * 0.25;
    const derivedMagicalDamage = int * 0.7 + per * 0.3;

    const finalHp = baseHp + derivedHp;
    const finalPhysicsDamage = basePhysicsDamage + derivedPhysicsDamage;
    const finalMagicalDamage = baseMagicalDamage + derivedMagicalDamage;

    const hp = Math.max(0, finalHp);
    const attack = Math.max(finalPhysicsDamage, finalMagicalDamage);
    const recommendedHpMin = baseline.playerDpr * target.min;
    const recommendedHpMax = baseline.playerDpr * target.max;
    const estimatedRounds = hp > 0 ? hp / Math.max(1, baseline.playerDpr) : 0;
    const pressurePct = attack > 0 ? attack / Math.max(1, baseline.playerHp) : 0;
    const recommendedExp = Math.max(1, Math.round(score.score * model.expPerPower * baseline.expRate * enemyTierReward(scoring, type)));
    const currentExp = asNumber(row.exp, 0);
    const expDeltaPct = recommendedExp > 0 ? (currentExp - recommendedExp) / recommendedExp : 0;
    if (estimatedRounds > 0 && (estimatedRounds < target.min || estimatedRounds > target.max)) {
      addHint(hints, 'warn', '战斗时长偏离', `预计 ${round(estimatedRounds, 1)} 回合，目标 ${target.min}-${target.max} 回合`);
    }
    if (Math.abs(expDeltaPct) > 0.35) {
      addHint(hints, Math.abs(expDeltaPct) > 1 ? 'danger' : 'warn', '经验偏离强度', `当前 ${currentExp}，建议 ${recommendedExp}，偏差 ${round(expDeltaPct * 100, 1)}%`);
    }
    if (level <= 1 && score.score > 1000) {
      addHint(hints, 'danger', '1级敌人强度异常', `强度分 ${Math.round(score.score)}，建议检查等级或迁移到对应等级段`);
    }
    if (pressurePct > pressureThreshold) {
      addHint(hints, 'warn', '单回合压力偏高', `主攻击约为玩家基准生命 ${round(pressurePct * 100, 1)}%，${type} 类型容忍上限 ${round(pressureThreshold * 100, 1)}%`);
    }
    if (hints.length === 0) addHint(hints, 'ok', '敌人预算正常', `经验、生命和压力未发现明显偏离；目标中位约 ${round(targetMid, 1)} 回合`);
    return {
      table,
      id,
      score: score.score,
      hints,
      enemy: {
        powerScore: score.score,
        recommendedExp,
        expDeltaPct,
        recommendedHpMin: round(recommendedHpMin),
        recommendedHpMax: round(recommendedHpMax),
        estimatedRounds: round(estimatedRounds, 2),
        pressurePct: round(pressurePct, 4),
        defenseK: model.defenseK
      }
    };
  }
  if (table === 'skills') {
    const level = Math.max(1, asNumber(row.level, 1));
    const baseline = baselineAtLevel(model, level);
    const rearPenalty = asNumber(score.detail.rearPressurePenalty, 0);
    const effectBudget = asNumber(score.detail.effectBudget, 0);
    const cooldownPenalty = asNumber(score.detail.cooldownPenalty, 0);
    const mpPenalty = asNumber(score.detail.mpPenalty, 0);
    const cooldown = asNumber(row.cooldown, 0);
    const dpsRate = effectBudget / Math.max(1, 1 + cooldown);
    const dpsRef = baseline.playerDpr * (scoring.constants.skillDpsRefPerRound ?? 0.85);
    if (rearPenalty > 0) addHint(hints, 'danger', '后排直压风险', `伤害目标策略产生 ${round(rearPenalty)} 分风险，优先改为 front_to_rear`);
    if (cooldownPenalty + mpPenalty > effectBudget * 0.45) addHint(hints, 'info', '资源/冷却成本较高', `成本约 ${round(cooldownPenalty + mpPenalty)} 分，适合作为爆发或策略技能`);
    if (effectBudget > 0 && cooldownPenalty + mpPenalty < effectBudget * 0.08) addHint(hints, 'warn', '释放成本偏低', '高收益技能缺少 MP/CD 约束，可能成为默认最优');
    if (dpsRef > 0 && dpsRate > dpsRef * 2.5) addHint(hints, 'warn', '技能DPS速率偏高', `每回合有效收益 ${round(dpsRate)} 约为基准 ${round(dpsRef)} 的 ${round(dpsRate / dpsRef, 1)}x`);
  }
  if (table === 'equipment_templates') {
    const base = asNumber(score.detail.baseAttrs, 0);
    const hooks = asNumber(score.detail.battleHooks, 0);
    const mutationLike = hooks + asNumber(score.detail.basicAttack, 0);
    const comboMax = score.score * model.comboBudgetMaxShare;
    if (mutationLike > comboMax && comboMax > 0) addHint(hints, 'warn', '机制/普攻预算偏高', `机制相关约 ${round(mutationLike)} 分，建议不超过 ${round(comboMax)} 分`);
    if (base <= 0 && score.score > 0) addHint(hints, 'info', '特色装备', '主要强度来自普攻/钩子/词条槽，请确认触发条件和适用场景');
    // DPR calibration: infer implied level from score and check attack power
    const c = scoring.constants;
    if (c.levelBudgetBase > 0 && c.levelBudgetGrowth > 0 && c.levelBudgetPower > 0 && score.score > c.levelBudgetBase) {
      const impliedLevel = 1 + Math.pow(
        Math.max(0, score.score - c.levelBudgetBase) / c.levelBudgetGrowth,
        1 / c.levelBudgetPower
      );
      const baseline = baselineAtLevel(model, impliedLevel);
      const basicAttackScore = asNumber(score.detail.basicAttack, 0);
      if (basicAttackScore > 0 && baseline.playerDpr > 0 && basicAttackScore / baseline.playerDpr > 2.0) {
        addHint(hints, 'warn', '普攻伤害系数偏高', `普攻评分 ${round(basicAttackScore)} 约为隐含等级 Lv${round(impliedLevel, 0)} 玩家 DPR ${round(baseline.playerDpr)} 的 ${round(basicAttackScore / baseline.playerDpr, 1)}x`);
      }
    }
  }
  if (table === 'affix_definitions') {
    const maxBody = asNumber(row.maxEffectivePerBody, 0);
    const maxEquipment = asNumber(row.maxEffectivePerEquipment, 0);
    const mechanism = asNumber(score.detail.mutation, 0) + asNumber(score.detail.battleHooks, 0) + asNumber(score.detail.manualScore, 0);
    const comboMax = Math.max(1, score.score) * model.comboBudgetMaxShare;
    if (mechanism > comboMax && maxBody <= 0 && maxEquipment <= 0) addHint(hints, 'warn', '机制词条缺少上限', `机制预算 ${round(mechanism)}，建议设置单装备/全身生效上限`);
    if (maxBody > 4) addHint(hints, 'info', '全身叠加较宽', `最多全身 ${maxBody} 个生效，注意 combo 不应变成唯一答案`);
  }
  if (hints.length === 0) addHint(hints, 'ok', '预算未见明显异常', `当前评分 ${round(score.score)}`);
  return { table, id, score: score.score, hints };
}

export function buildBalanceIssueList(catalog: Catalog, scoring: ScoringConfig, model: CombatBalanceModel): BalanceDiagnostics[] {
  const tables: CombatTable[] = ['skills', 'equipment_templates', 'affix_definitions', 'enemies'];
  const out: BalanceDiagnostics[] = [];
  for (const table of tables) {
    for (const row of catalog[table]) {
      if (row?.disabled === true) continue;
      const diag = diagnoseRowBalance(table, row, catalog, scoring, model);
      if (diag.hints.some((hint) => hint.severity === 'warn' || hint.severity === 'danger')) out.push(diag);
    }
  }
  const rank = { danger: 0, warn: 1, info: 2, ok: 3 };
  return out.sort((a, b) => {
    const ar = Math.min(...a.hints.map((hint) => rank[hint.severity]));
    const br = Math.min(...b.hints.map((hint) => rank[hint.severity]));
    return ar - br || b.score - a.score;
  });
}

/**
 * 根据敌人等级和类型自动估算 tier 参数。
 * 用于辅助设计，避免"无头苍蝇"式手动填数。
 */
export function estimateTierParams(level: number, tierType: 'normal' | 'elite' | 'boss' | 'world_boss'): {
  targetRounds: { min: number; max: number };
  pressurePct: number;
  budgetMultiplier: number;
  hpMultiplier: number;
  suggestedHp: number;
  suggestedDps: number;
} {
  const baseline = baselineAtLevel(DEFAULT_COMBAT_BALANCE_MODEL, level);

  // 根据类型设定战斗时长、压力、预算倍率和HP倍率
  const tierConfig = {
    normal:     { rounds: [1, 3],   pressure: 0.15, budget: 1.0, hpMult: 1.0 },
    elite:      { rounds: [3, 5],   pressure: 0.20, budget: 1.45, hpMult: 1.5 },
    boss:       { rounds: [5, 8],   pressure: 0.30, budget: 2.4, hpMult: 3.0 },
    world_boss: { rounds: [8, 12],  pressure: 0.50, budget: 5.0, hpMult: 5.0 }
  }[tierType];

  const midRounds = (tierConfig.rounds[0] + tierConfig.rounds[1]) / 2;
  const targetHp = Math.round(baseline.playerDpr * midRounds * tierConfig.budget * tierConfig.hpMult);
  const targetDps = Math.round(baseline.playerHp * tierConfig.pressure * tierConfig.budget);

  return {
    targetRounds: { min: tierConfig.rounds[0], max: tierConfig.rounds[1] },
    pressurePct: tierConfig.pressure,
    budgetMultiplier: tierConfig.budget,
    hpMultiplier: tierConfig.hpMult,
    suggestedHp: targetHp,
    suggestedDps: targetDps
  };
}
