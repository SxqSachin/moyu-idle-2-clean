export type EffectMap = Record<string, number>;

export interface Constants {
  goldPerSecond: number;
  expSeconds: number;
  goldPerBudget: number;
  levelBudgetBase: number;
  levelBudgetGrowth: number;
  levelBudgetPower: number;
  affixSlotBaseBudget: number;
  enhanceBaseCostGold: number;
  enhanceCostPower: number;
  milestoneWeight: number;
  extraSkillTargetValue: number;
  extraBuffTargetValue: number;
  extraTurnValue: number;
  cooldownPenaltyPerTurn: number;
  mpCostPenalty: number;
  targetPositionValue: number;
  rearTargetValue: number;
  seekStrategyValue: number;
  selectRuleValue: number;
  fallbackSelfValue: number;
  battleHookValue: number;
  effectMutationValue: number;
  enhanceAttrScoreMultiplier: number;
  affixInstanceScoreMultiplier: number;
  enemySkillScoreMultiplier: number;
  enemyGoldRewardRate: number;
  enemyExpRewardRate: number;
  basicAttackMultiTargetFactor: number;
  skillDpsRefPerRound: number;
  skillBalanceWarnRatio: number;
  skillBalanceOvertuneRatio: number;
}

export interface AttributeValue {
  key: string;
  label: string;
  category: string;
  unitValue: number;
  displayAsPercent?: boolean;
  description?: string;
}

export interface RarityRule {
  key: string;
  label: string;
  multiplier: number;
  affixRollMin: number;
  affixRollMax: number;
  description?: string;
}

export interface SlotRule {
  key: string;
  label: string;
  multiplier: number;
  defaultAffixSlots: number;
  defaultMaxEnhance: number;
}

export interface EnemyTemplateRule {
  key: string;
  label: string;
  weights: Record<string, number>;
  baseAttrs?: Record<string, number>;
  multipliers?: Record<string, number>;
  preferredSkillTypes?: string[];
  description?: string;
}

export interface EnemyTierTemplateRule {
  key: string;
  label: string;
  enemyType: string;
  budgetMultiplier: number;
  scoreMult: number;
  rewardMult: number;
  pressurePct: number;
  targetRounds: { min: number; max: number };
  hpMultiplier?: number;
  description?: string;
}

export interface CombatDifficultyLevel {
  level: number
  labelKey: string
  statMultiplierCurve: Record<string, unknown>
  battleExpBonus: number
  battleDropRateBonus: number
}

export interface EnemyDifficultyAuthoringGuardrails {
  minEnemyLevel: number
  defenseRetainRatio: number
  defenseBaseCapsByEnemyType: Record<string, number>
  critDamageResistRetainRatio: number
  critDamageResistBaseFloorsByEnemyType: Record<string, number>
  critDamageResistBaseCapsByEnemyType: Record<string, number>
}

export interface WorldStatBonus {
  attrKey: string
  base: number
  perLevelMult: number
}

export interface WorldSpecialDrop {
  itemId: string
  percent: number
  rangeMin: number
  rangeMax: number
  minLevel: number
}

export interface WorldDifficultySettings {
  maxLevel: number
  statBonuses: WorldStatBonus[]
  specialDrops: WorldSpecialDrop[]
}

export interface EnemyAttributeTemplates {
  tiers: EnemyTierTemplateRule[];
  roles: EnemyTemplateRule[];
  species: EnemyTemplateRule[];
  derivedStatMinimums?: Record<string, number>;
}

export interface DropPricingRarityRule {
  key: string;
  weight: number;
  softFloor: number;
  hardCap: number;
}

export interface DropPricingConfig {
  targetGoldPerSecond: number;
  dropBudgetShare: number;
  defaultPlayerCount: number;
  waveScorePerSecond: number;
  specialWaveHorizon: number;
  worldFloorGrowth: number;
  worldFloorMaxMultiplier: number;
  enemyLevelPriceGrowth: number;
  enemyLevelPriceMaxMultiplier: number;
  ignoreExistingPriceBelow: number;
  overpricedWarnRatio: number;
  enemyTemplateDeviationWarnRatio: number;
  enemyTypeWeights: Record<string, number>;
  rarityRules: DropPricingRarityRule[];
}

export interface SkillModel {
  id: string;
  name: string;
  level: number;
  rarity: string;
  role: string;
  targetSide: 'enemy' | 'ally' | 'self';
  targetCount: number;
  targetPosition: string;
  damageType: 'physical' | 'magical' | 'mixed' | 'true' | 'none';
  damageMultiplier: number;
  hitRate: number;
  mpCost: number;
  cooldownTurns: number;
  castDelayTurns: number;
  effectDurationTurns: number;
  effectsText: string;
  tags: string;
}

export interface BuffModel {
  id: string;
  name: string;
  icon: string;
  rarity: string;
  targetSide: 'enemy' | 'ally' | 'self';
  targetCount: number;
  durationTurns: number;
  uptime: number;
  stackLimit: number;
  dispellable: boolean;
  showInBattleCard: boolean;
  effectsText: string;
  tags: string;
}

export interface EquipmentModel {
  id: string;
  name: string;
  level: number;
  slot: string;
  rarity: string;
  archetype: string;
  affixSlots: number;
  maxEnhanceLevel: number;
  baseEffectsText: string;
  enhancePerLevelText: string;
  milestonesText: string;
  tags: string;
}

export interface AffixModel {
  id: string;
  name: string;
  rarity: string;
  scope: string;
  weight: number;
  conflictGroup: string;
  effectsText: string;
  tags: string;
}

export interface EnemyModel {
  id: string;
  name: string;
  level: number;
  tier: string;
  role: string;
  countInWave: number;
  skillPressure: number;
  customStatsText: string;
  tags: string;
}

export interface ModelState {
  constants: Constants;
  attributes: AttributeValue[];
  rarities: RarityRule[];
  slots: SlotRule[];
  skills: SkillModel[];
  buffs: BuffModel[];
  equipments: EquipmentModel[];
  affixes: AffixModel[];
  enemies: EnemyModel[];
}

export interface ScoreResult {
  score: number;
  goldValue: number;
  detail: Record<string, number>;
  sections?: ScoreBreakdownSection[];
}

export interface ScoreBreakdownSection {
  id: string;
  label: string;
  score: number;
  items?: ScoreBreakdownItem[];
}

export interface ScoreBreakdownItem {
  label: string;
  score: number;
  meta?: string;
}
