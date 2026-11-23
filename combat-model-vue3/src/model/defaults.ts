import type { DropPricingConfig, EnemyAttributeTemplates, ModelState } from './types';
import type { AttrDerivation } from './realScoring';

export function createDefaultEnemyAttributeTemplates(): EnemyAttributeTemplates {
  return {
    tiers: [
      { key: 'normal',    label: '普通',   enemyType: 'normal',    budgetMultiplier: 1,    scoreMult: 1,    rewardMult: 1,    pressurePct: 0.15, targetRounds: { min: 4,  max: 8  }, hpMultiplier: 1 },
      { key: 'elite',     label: '精英',   enemyType: 'elite',     budgetMultiplier: 1.45, scoreMult: 2.3,  rewardMult: 1.65, pressurePct: 0.20, targetRounds: { min: 8,  max: 15 }, hpMultiplier: 1.5 },
      { key: 'boss',      label: '首领',   enemyType: 'boss',      budgetMultiplier: 2.4,  scoreMult: 6.5,  rewardMult: 3.5,  pressurePct: 0.30, targetRounds: { min: 15, max: 35 }, hpMultiplier: 3.0 },
      { key: 'world_boss',label: '世界首领',enemyType: 'world_boss',budgetMultiplier: 5,    scoreMult: 28,   rewardMult: 7,    pressurePct: 0.50, targetRounds: { min: 35, max: 60 }, hpMultiplier: 5.0 }
    ],
    roles: [
      { key: 'tank',       label: '坦克', preferredSkillTypes: ['physical', 'taunt', 'heal'],         weights: { stamina: 0.35, strength: 0.20, perception: 0.12, dexterity: 0.10, intelligence: 0.08, hp: 0.08, physics_defence: 0.07 } },
      { key: 'warrior',    label: '战士', preferredSkillTypes: ['physical', 'melee', 'aoe'],          weights: { strength: 0.40, stamina: 0.22, dexterity: 0.15, perception: 0.08, intelligence: 0.05, hp: 0.05, physics_damage: 0.05 } },
      { key: 'mage',       label: '法师', preferredSkillTypes: ['magical', 'aoe', 'debuff'],          weights: { intelligence: 0.42, perception: 0.28, stamina: 0.10, dexterity: 0.08, strength: 0.05, hp: 0.04, mp: 0.03 } },
      { key: 'assassin',   label: '刺客', preferredSkillTypes: ['physical', 'single', 'crit', 'pierce'], weights: { dexterity: 0.38, strength: 0.28, perception: 0.16, stamina: 0.10, intelligence: 0.04, hp: 0.04 }, baseAttrs: { physics_damage: 8, crit_chance: 0.03 } },
      { key: 'support',    label: '辅助', preferredSkillTypes: ['heal', 'buff', 'magical'],           weights: { intelligence: 0.32, perception: 0.30, stamina: 0.18, dexterity: 0.10, strength: 0.05, hp: 0.03, mp: 0.02 } },
      { key: 'magic_tank', label: '法坦', preferredSkillTypes: ['magical', 'taunt', 'debuff'],        weights: { stamina: 0.30, intelligence: 0.25, perception: 0.22, strength: 0.12, dexterity: 0.06, hp: 0.05 } },
      { key: 'priest',     label: '牧师', preferredSkillTypes: ['heal', 'magical', 'buff'],           weights: { perception: 0.40, intelligence: 0.28, stamina: 0.18, dexterity: 0.06, strength: 0.04, hp: 0.04 }, baseAttrs: { magical_defence: 12, hp_regen: 3, mp_regen: 2 } },
      { key: 'archer',     label: '弓手', preferredSkillTypes: ['physical', 'single', 'pierce'],      weights: { dexterity: 0.42, strength: 0.24, perception: 0.15, stamina: 0.10, intelligence: 0.04, hp: 0.05 }, baseAttrs: { physics_damage: 6, crit_chance: 0.02 } },
      { key: 'berserker',  label: '狂战士', preferredSkillTypes: ['physical', 'melee', 'aoe'],        weights: { strength: 0.48, stamina: 0.24, dexterity: 0.12, perception: 0.08, intelligence: 0.04, hp: 0.04 }, baseAttrs: { physics_damage: 10, crit_damage: 0.05 } },
      { key: 'scout',      label: '游侠', preferredSkillTypes: ['physical', 'single', 'debuff'],      weights: { dexterity: 0.34, perception: 0.28, stamina: 0.16, strength: 0.10, intelligence: 0.08, hp: 0.04 } }
    ],
    species: [
      { key: 'beast',     label: '野兽',   weights: { dexterity: 0.04, strength: 0.03, stamina: 0.02, perception: 0.01 } },
      { key: 'humanoid',  label: '人形',   weights: { strength: 0.03, dexterity: 0.03, intelligence: 0.02, perception: 0.02 } },
      { key: 'undead',    label: '亡灵',   weights: { stamina: 0.05, strength: 0.03, perception: 0.02 }, baseAttrs: { hp: 15 } },
      { key: 'construct', label: '构装体', weights: { stamina: 0.05, strength: 0.03, perception: 0.02 }, multipliers: { physics_defence: 1.12, speed: 0.88 } },
      { key: 'spirit',    label: '灵体',   weights: { intelligence: 0.05, perception: 0.04, stamina: 0.01 }, baseAttrs: { magical_damage: 4 } },
      { key: 'plant',     label: '植物',   weights: { stamina: 0.06, perception: 0.03, strength: 0.01 }, baseAttrs: { hp_regen: 2 } },
      { key: 'insect',    label: '虫群',   weights: { dexterity: 0.06, perception: 0.03, strength: 0.01 }, multipliers: { speed: 1.08 } },
      { key: 'shadow',    label: '暗影',   weights: { dexterity: 0.05, perception: 0.04, strength: 0.01 }, baseAttrs: { crit_chance: 0.02 } },
      { key: 'elemental', label: '元素',   weights: { intelligence: 0.05, perception: 0.03, stamina: 0.02 }, baseAttrs: { magical_damage: 3 } },
      { key: 'feline',    label: '猫族',   weights: { dexterity: 0.05, perception: 0.03, strength: 0.02 }, baseAttrs: { speed: 6, crit_chance: 0.02 } },
      { key: 'dragon',    label: '龙族',   weights: { strength: 0.03, stamina: 0.03, intelligence: 0.02, perception: 0.02 }, baseAttrs: { hp: 25, physics_damage: 4, magical_damage: 4 } }
    ],
    derivedStatMinimums: {
      hp: 1,
      physics_damage: 1,
      physics_defence: 1,
      magical_damage: 1,
      magical_defence: 1
    }
  };
}

export const STORAGE_KEY = 'combat-model-tuner-v1';

export function createDefaultDropPricingConfig(): DropPricingConfig {
  return {
    targetGoldPerSecond: 1,
    dropBudgetShare: 0.3,
    defaultPlayerCount: 1,
    waveScorePerSecond: 12,
    specialWaveHorizon: 30,
    worldFloorGrowth: 0.25,
    worldFloorMaxMultiplier: 2.25,
    enemyLevelPriceGrowth: 0.01,
    enemyLevelPriceMaxMultiplier: 1.8,
    ignoreExistingPriceBelow: 1,
    overpricedWarnRatio: 1.5,
    enemyTemplateDeviationWarnRatio: 0.5,
    enemyTypeWeights: {
      normal: 1,
      elite: 1.12,
      boss: 1.25,
      world_boss: 1.5
    },
    rarityRules: [
      { key: 'common', weight: 1, softFloor: 20, hardCap: 80 },
      { key: 'uncommon', weight: 1.2, softFloor: 24, hardCap: 120 },
      { key: 'rare', weight: 1.45, softFloor: 32, hardCap: 240 },
      { key: 'epic', weight: 1.75, softFloor: 48, hardCap: 320 },
      { key: 'legendary', weight: 2.2, softFloor: 120, hardCap: 500 }
    ]
  };
}

export function createDefaultState(): ModelState {
  return {
    constants: {
      goldPerSecond: 1,
      expSeconds: 8,
      goldPerBudget: 12,
      levelBudgetBase: 8,
      levelBudgetGrowth: 2.8,
      levelBudgetPower: 1.18,
      affixSlotBaseBudget: 18,
      enhanceBaseCostGold: 24,
      enhanceCostPower: 1.48,
      milestoneWeight: 1,
      extraSkillTargetValue: 0.62,
      extraBuffTargetValue: 0.55,
      extraTurnValue: 0.45,
      cooldownPenaltyPerTurn: 0.08,
      mpCostPenalty: 0.55,
      targetPositionValue: 4,
      rearTargetValue: 12,
      seekStrategyValue: 6,
      selectRuleValue: 5,
      fallbackSelfValue: 3,
      battleHookValue: 18,
      effectMutationValue: 18,
      enhanceAttrScoreMultiplier: 0.72,
      affixInstanceScoreMultiplier: 1,
      enemySkillScoreMultiplier: 0.45,
      enemyGoldRewardRate: 0.18,
      enemyExpRewardRate: 0.018,
      basicAttackMultiTargetFactor: 0.35,
      skillDpsRefPerRound: 0.85,
      skillBalanceWarnRatio: 1.0,
      skillBalanceOvertuneRatio: 1.5
    },
    attributes: [
      { key: 'hp', label: '生命值', category: '生存', unitValue: 0.1, description: 'derivedStats: stamina / 9' },
      { key: 'mp', label: '魔法值', category: '资源', unitValue: 0.25, description: 'derivedStats: intelligence / 4' },
      { key: 'hp_regen', label: '生命恢复', category: '恢复', unitValue: 4.5, description: 'derivedStats: stamina / 0.2' },
      { key: 'mp_regen', label: '魔法恢复', category: '恢复', unitValue: 10, description: 'derivedStats: intelligence / 0.1' },
      { key: 'strength', label: '力量', category: '主属性', unitValue: 1.0 },
      { key: 'dexterity', label: '敏捷', category: '主属性', unitValue: 1.0 },
      { key: 'intelligence', label: '智力', category: '主属性', unitValue: 1.0 },
      { key: 'stamina', label: '耐力', category: '主属性', unitValue: 0.9 },
      { key: 'luck', label: '幸运', category: '特殊', unitValue: 0.75 },
      { key: 'perception', label: '感知', category: '主属性', unitValue: 1.0 },
      { key: 'physics_damage', label: '物理伤害', category: '输出', unitValue: 1.8182 },
      { key: 'magical_damage', label: '魔法伤害', category: '输出', unitValue: 1.0 },
      { key: 'physics_defence', label: '物理防御', category: '生存', unitValue: 2 },
      { key: 'magical_defence', label: '魔法防御', category: '生存', unitValue: 2.2222 },
      { key: 'crit_chance', label: '暴击率', category: '爆发', unitValue: 1.0714, displayAsPercent: true, description: '填0.05代表5%' },
      { key: 'crit_damage', label: '暴击伤害', category: '爆发', unitValue: 1.1111, displayAsPercent: true, description: '填0.20代表20%' },
      { key: 'magic_crit_chance', label: '法术暴击率', category: '爆发', unitValue: 1.0714, displayAsPercent: true, description: '填0.05代表5%' },
      { key: 'magic_crit_damage', label: '法术暴击伤害', category: '爆发', unitValue: 1.1111, displayAsPercent: true, description: '填0.20代表20%' },
      { key: 'crit_damage_resist', label: '暴击伤害抵抗', category: '爆发', unitValue: 1.1111, displayAsPercent: true, description: '填0.20代表20%' },
      { key: 'speed', label: '速度', category: '节奏', unitValue: 2.2222, description: 'derivedStats: dexterity / 0.45' },
      { key: 'physics_damage_amplification', label: '物理增伤', category: '乘区', unitValue: 60, displayAsPercent: true },
      { key: 'magical_damage_amplification', label: '魔法增伤', category: '乘区', unitValue: 60, displayAsPercent: true },
      { key: 'physics_damage_resist', label: '物理减伤', category: '乘区', unitValue: 58, displayAsPercent: true },
      { key: 'magical_damage_resist', label: '魔法减伤', category: '乘区', unitValue: 58, displayAsPercent: true },
      { key: 'physics_flat_defense_penetration', label: '物理固定穿透', category: '穿透', unitValue: 5 },
      { key: 'magicial_flat_defense_penetration', label: '魔法固定穿透', category: '穿透', unitValue: 5 },
      { key: 'physics_percent_defense_penetration', label: '物理百分比穿透', category: '穿透', unitValue: 48, displayAsPercent: true },
      { key: 'magicial_percent_defense_penetration', label: '魔法百分比穿透', category: '穿透', unitValue: 48, displayAsPercent: true }
    ],
    rarities: [
      { key: 'common', label: '普通', multiplier: 1, affixRollMin: 0.9, affixRollMax: 1.05 },
      { key: 'uncommon', label: '优秀', multiplier: 1.18, affixRollMin: 1.0, affixRollMax: 1.2 },
      { key: 'rare', label: '稀有', multiplier: 1.45, affixRollMin: 1.15, affixRollMax: 1.45 },
      { key: 'epic', label: '史诗', multiplier: 1.82, affixRollMin: 1.35, affixRollMax: 1.85 },
      { key: 'legendary', label: '传说', multiplier: 2.35, affixRollMin: 1.7, affixRollMax: 2.4 }
    ],
    slots: [
      { key: 'weapon', label: '武器', multiplier: 1.35, defaultAffixSlots: 2, defaultMaxEnhance: 15 },
      { key: 'helmet', label: '头部', multiplier: 0.9, defaultAffixSlots: 1, defaultMaxEnhance: 12 },
      { key: 'armor', label: '衣服', multiplier: 1.05, defaultAffixSlots: 2, defaultMaxEnhance: 12 },
      { key: 'gloves', label: '手套', multiplier: 0.85, defaultAffixSlots: 1, defaultMaxEnhance: 12 },
      { key: 'pants', label: '裤子', multiplier: 0.95, defaultAffixSlots: 1, defaultMaxEnhance: 12 },
      { key: 'boots', label: '鞋子', multiplier: 0.8, defaultAffixSlots: 1, defaultMaxEnhance: 12 },
      { key: 'accessory', label: '饰品', multiplier: 0.95, defaultAffixSlots: 2, defaultMaxEnhance: 10 }
    ],
    skills: [
      { id: 'power_slash', name: '强力斩击', level: 10, rarity: 'common', role: '战士单体', targetSide: 'enemy', targetCount: 1, targetPosition: 'front', damageType: 'physical', damageMultiplier: 1.35, hitRate: 1, mpCost: 8, cooldownTurns: 1, castDelayTurns: 0, effectDurationTurns: 0, effectsText: '', tags: 'single,physical' },
      { id: 'arcane_burst', name: '奥术爆裂', level: 18, rarity: 'rare', role: '法师AOE', targetSide: 'enemy', targetCount: 3, targetPosition: 'any', damageType: 'magical', damageMultiplier: 0.82, hitRate: 0.95, mpCost: 28, cooldownTurns: 3, castDelayTurns: 0, effectDurationTurns: 0, effectsText: 'magical_damage_resist=-8%', tags: 'aoe,debuff' }
    ],
    buffs: [
      { id: 'guard_stance', name: '守护姿态', icon: '', rarity: 'common', targetSide: 'self', targetCount: 1, durationTurns: 3, uptime: 1, stackLimit: 1, dispellable: true, showInBattleCard: false, effectsText: 'physics_damage_resist=8%;magical_damage_resist=8%', tags: 'defense' },
      { id: 'cat_wind', name: '猫风祝福', icon: '', rarity: 'rare', targetSide: 'ally', targetCount: 3, durationTurns: 2, uptime: 0.85, stackLimit: 1, dispellable: true, showInBattleCard: false, effectsText: 'speed=18;crit_chance=3%', tags: 'support' }
    ],
    equipments: [
      { id: 'iron_sword_10', name: '铁质长剑', level: 10, slot: 'weapon', rarity: 'common', archetype: '战士均衡', affixSlots: 1, maxEnhanceLevel: 10, baseEffectsText: 'physics_damage=12;strength=3', enhancePerLevelText: 'physics_damage=1.2', milestonesText: '5:crit_chance=2%|10:physics_damage=6', tags: 'weapon,physical' },
      { id: 'moon_robe_20', name: '月影法袍', level: 20, slot: 'armor', rarity: 'rare', archetype: '法师AOE', affixSlots: 2, maxEnhanceLevel: 12, baseEffectsText: 'magical_damage=18;mp=45;intelligence=5', enhancePerLevelText: 'magical_damage=1.5;mp=4', milestonesText: '6:magical_damage_amplification=4%|12:mp_regen=1', tags: 'armor,magic' }
    ],
    affixes: [
      { id: 'sharp', name: '锋利', rarity: 'common', scope: 'weapon', weight: 100, conflictGroup: 'damage_prefix', effectsText: 'physics_damage=6', tags: 'prefix' },
      { id: 'swift', name: '迅捷', rarity: 'rare', scope: 'all', weight: 60, conflictGroup: 'speed_suffix', effectsText: 'speed=18;crit_chance=2%', tags: 'suffix' }
    ],
    enemies: [
      { id: 'plain_mouse_10', name: '平原小鼠', level: 10, tier: 'normal', role: 'burst', countInWave: 3, skillPressure: 0.08, customStatsText: '', tags: 'plain' },
      { id: 'plain_guard_cat_20', name: '平原守护猫', level: 20, tier: 'boss', role: 'tank', countInWave: 1, skillPressure: 0.2, customStatsText: 'physics_damage_resist=5%', tags: 'boss' }
    ]
  };
}

/**
 * 主属性 → 派生属性换算率默认表。
 * 含义：1点主属性点分配到该属性后，可获得的各派生属性数值。
 * 用于将技能/装备公式中的派生属性还原为主属性等价消耗，从而评估公式权重是否合理。
 *
 * 多来源情况（如生命值同时来自力量和耐力），各主属性按实际换算率独立填写；
 * 公式权重取最优来源（换算率最高 = 每主属性点获得最多派生属性 = 单位成本最低）。
 *
 * 请根据游戏实际公式调整这些数值。
 */
export function createDefaultAttrDerivations(): AttrDerivation[] {
  return [
    {
      // 力量：偏向物理输出 + 少量生存
      primaryKey: 'strength',
      rates: {
        hp: 1.2,
        physics_flat_defense_penetration: 0.2,
        physics_damage: 0.55,
        physics_defence: 0.15,
        crit_damage: 0.003,       // 0.3%
        crit_damage_resist: 0.001 // 0.1%
      }
    },
    {
      // 敏捷：速度、暴击、少量物理
      primaryKey: 'dexterity',
      rates: {
        speed: 0.45,
        crit_chance: 0.002,  // 0.2%（请根据实际调整）
        crit_damage: 0.002   // 0.2%（请根据实际调整）
      }
    },
    {
      // 智力：法术伤害、魔法资源（mp来自 intelligence/4，mp_regen来自 intelligence/0.1）
      primaryKey: 'intelligence',
      rates: {
        magical_damage: 1.0,
        mp: 4,
        mp_regen: 0.1
      }
    },
    {
      // 耐力：生命值（hp来自 stamina/9）、生命恢复（hp_regen来自 stamina/0.2）、少量防御
      primaryKey: 'stamina',
      rates: {
        hp: 9,
        hp_regen: 0.2,
        physics_defence: 0.3,  // 请根据实际调整
        magical_defence: 0.2   // 请根据实际调整
      }
    },
    {
      // 感知：法术输出、法防为主（请根据实际调整）
      primaryKey: 'perception',
      rates: {
        magical_damage: 0.5,
        magical_defence: 0.4,
        magic_crit_chance: 0.003  // 0.3%（请根据实际调整）
      }
    },
    {
      // 幸运：少量暴击率（请根据实际调整）
      primaryKey: 'luck',
      rates: {
        crit_chance: 0.002,       // 0.2%
        magic_crit_chance: 0.002  // 0.2%
      }
    }
  ];
}
