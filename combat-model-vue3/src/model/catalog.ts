export const COMBAT_TABLES = [
  'actions',
  'buildings',
  'skills',
  'skill_type',
  'buff',
  'buff_type',
  'buff_effects',
  'battle_usable_items',
  'operation_effects',
  'cattery_furniture',
  'market_profiles',
  'fish_food_recipes',
  'equipment_templates',
  'weapon_classes',
  'affix_definitions',
  'affix_enchant_rules',
  'affix_carrier_rules',
  'affix_smelt_rules',
  'alchemy_groups',
  'item_traits',
  'alchemy_combo_recipes',
  'alchemy_transmute_rules',
  'alchemy_inherit_mediums',
  'alchemy_mediums',
  'enemies',
  'world',
  'arena_formats',
  'combat_trial_metrics',
  'combat_trial_formats',
  'attributes',
  'specialization_trees',
  'rarity_definitions',
  'kitty_personalities',
  'kitty_journal_personality_grants',
  'items',
  'cat_village_commissions',
  'collectibles',
  'title_categories',
  'title_prefixes',
  'title_suffixes',
  'achievement_categories',
  'achievements',
  'fishing_species',
  'collectible_categories'
] as const;

export type CombatTable = (typeof COMBAT_TABLES)[number];
export type JsonPrimitive = string | number | boolean | null;
export type JsonValue = JsonPrimitive | JsonObject | JsonValue[];
export type JsonObject = { [key: string]: JsonValue };
export type DataRow = Record<string, unknown>;
export type Catalog = Record<CombatTable, DataRow[]>;

export const EDITABLE_TABLES = COMBAT_TABLES;

export const TABLE_LABELS: Record<CombatTable, string> = {
  actions: '动作',
  buildings: '建筑',
  skills: '技能',
  skill_type: '技能类型',
  buff: 'Buff',
  buff_type: 'Buff类型',
  buff_effects: 'Buff效果',
  battle_usable_items: '战斗可用物品',
  operation_effects: '全局操作效果',
  cattery_furniture: '猫舍家具',
  market_profiles: '市场规格规则',
  fish_food_recipes: '鱼料理配方',
  equipment_templates: '装备',
  weapon_classes: '武器类别',
  affix_definitions: '词条',
  affix_enchant_rules: '附魔规则',
  affix_carrier_rules: '词条载体',
  affix_smelt_rules: '词条熔炉规则',
  alchemy_groups: '炼金转换组',
  item_traits: '物品炼金特质',
  alchemy_combo_recipes: '炼金组合配方',
  alchemy_transmute_rules: '点金规则',
  alchemy_inherit_mediums: '炼金融合介质',
  alchemy_mediums: '炼金介质',
  enemies: '敌人',
  world: '区域',
  arena_formats: '联赛赛制',
  combat_trial_metrics: '试炼指标',
  combat_trial_formats: '试炼项目',
  attributes: '属性',
  specialization_trees: '专精树',
  rarity_definitions: '稀有度',
  kitty_personalities: '猫咪性格',
  kitty_journal_personality_grants: '随记特性授予',
  items: '物品',
  cat_village_commissions: '猫猫村委托',
  collectibles: '收藏品',
  title_categories: '称号分类',
  title_prefixes: '称号前缀',
  title_suffixes: '称号后缀',
  achievement_categories: '成就分类',
  achievements: '成就',
  fishing_species: '钓鱼鱼种',
  collectible_categories: '收藏品分类'
};

export function createEmptyCatalog(): Catalog {
  return Object.fromEntries(COMBAT_TABLES.map((table) => [table, []])) as unknown as Catalog;
}

export function clone<T>(value: T): T {
  return JSON.parse(JSON.stringify(value)) as T;
}

export function isRecord(value: unknown): value is Record<string, unknown> {
  return Boolean(value) && typeof value === 'object' && !Array.isArray(value);
}

export function asArray(value: unknown): unknown[] {
  return Array.isArray(value) ? value : [];
}

export function asNumber(value: unknown, fallback = 0): number {
  const parsed = Number(value);
  return Number.isFinite(parsed) ? parsed : fallback;
}

export function asString(value: unknown, fallback = ''): string {
  return typeof value === 'string' ? value : value == null ? fallback : String(value);
}

export function rowId(row: DataRow | null | undefined, fallback = ''): string {
  const id = asString(row?.id).trim();
  return id || fallback;
}

export function sortObjectDeep<T>(value: T): T {
  if (Array.isArray(value)) return value.map((item) => sortObjectDeep(item)) as T;
  if (!isRecord(value)) return value;
  const out: Record<string, unknown> = {};
  for (const key of Object.keys(value).sort((a, b) => a.localeCompare(b))) {
    out[key] = sortObjectDeep(value[key]);
  }
  return out as T;
}

export function makeDefaultRow(table: CombatTable): DataRow {
  const id = `${table}_new_${Date.now()}`;
  const withDisable = (row: DataRow): DataRow => ({ disabled: false, ...row });
  switch (table) {
    case 'market_profiles':
      return withDisable({
        id, schemaVersion: 1, groupingVersion: 1, catalogKeys: [],
        groupKeys: [{ source: 'instance', path: '/baseSnapshot', compare: 'exact' }],
        uniqueByInstance: false, presentation: []
      });
    case 'actions':
      return withDisable({
        id,
        iconId: `action.${id}`,
        nameKey: `action.${id}.name`,
        descKey: `action.${id}.desc`,
        mainType: 'cooking',
        menu: 'cooking',
        subType: 'common',
        durationS: 1,
        costs: {},
        materialInstanceRequirements: {},
        statusRequirement: {},
        itemReward: {},
        statusExpReward: {},
        economyIndustryTier: 1,
        economyDifficultyMultiplier: 1,
        rarity: 'common',
        buildingRequirement: {},
        mainOutput: ''
      });
    case 'operation_effects':
      return {
        id,
        rulesetVersion: 'operation-effect-v1',
        domain: 'life_action',
        kind: 'life_action.extra_main_output_chance',
        nameKey: `operation_effect.${id}.name`,
        descKey: `operation_effect.${id}.desc`,
        iconId: '',
        priority: 0,
        allowedTargets: ['player'],
        lifetime: { kind: 'charges', charges: 10 },
        stacking: { key: id, mode: 'add_charges', maxCharges: 100 },
        selector: {},
        params: { chance: 0.5, quantity: 1 }
      };
    case 'cattery_furniture':
      return {
        id, version: 'furniture-v1', nameKey: `game.furniture.${id}`, iconId: 'item.kitty_training_bench',
        service: 'permanent', installCosts: { wood_blank: 420, iron: 28 }, refillCosts: {}, capacitySeconds: 0, refillSeconds: 0,
        rules: [{ kind: 'action_efficiency_add', domain: 'action_start', rateBps: 2000, selector: { mainTypes: ['collecting','mining','farming_animal','crafting','sewing','forging','alchemy','cooking'] } }],
        levels: [],
      };
    case 'fish_food_recipes':
      return {
        id,
        rulesetVersion: 'fish-food-v2',
        actionId: '',
        outputItemId: '',
        kind: 'grilled_fish',
        qualityRulesetVersion: 'fish-food-quality-v2',
        parameterScaling: { rate: { length: 0.15, weight: 0.4, traits: 0.15, min: 0.005, max: 0.15 } },
        legacyEffectIds: [],
        replacedEffectIds: [],
        mutationLifetimeBonuses: {},
        allowSpecificIngredients: true,
        fishIngredients: [
          { speciesId: '', quantity: 1 },
          { speciesId: '', quantity: 1 }
        ]
      };
    case 'skills':
      return withDisable({
        id,
        name: '',
        icon: '',
        category: '',
        hpCost: 0,
        mpCost: 0,
        cooldown: 0,
        enemyOnly: false,
        statusRequirement: {},
        weaponRequirement: [],
        castGuards: [],
        effects: [{ ...defaultEffect('damage'), id: `${id}_damage` }]
      });
    case 'buildings':
      return withDisable({
        id,
        iconId: 'item.craftsmans_hammer',
        nameKey: `building.${id}.name`,
        descKey: `building.${id}.desc`,
        order: 1,
        maxLevel: 1,
        baseDurationS: 300,
        stepDurationS: 150,
        baseCosts: {},
        stepCosts: {},
        baseStatusRequirement: {},
        stepStatusRequirement: {}
      });
    case 'equipment_templates':
      return withDisable({
        id,
        equipType: 'battle',
        slot: 'right_hand',
        equipTargets: [],
        occupySlots: [],
        weaponClass: [],
        equipmentSecondaryClass: '',
        statusRequirement: {},
        accessoryGroup: '',
        affixSlotCount: 0,
        affixAllowedIds: [],
        baseAttrs: {},
        maxDurability: 100,
        decayRate: 0,
        enhanceMaxLevel: 0,
        enhanceDurationS: 1,
        enhanceExp: 0,
        enhanceProtectItemId: [],
        enhanceCostCurve: { base: {}, step: {}, overrides: {} },
        enhanceAttrCurve: { base: {}, step: {}, overrides: {} },
        basicAttackEffects: [defaultEffect('damage')],
        battleHooks: []
      });
    case 'affix_definitions':
      return withDisable({
        id,
        nameKey: `affix.${id}.name`,
        descKey: `affix.${id}.desc`,
        effectType: 'attr_bonus',
        effectParams: { attrMods: {} },
        valuePrecision: 0,
        manualScore: 0,
        allowedRarities: ['uncommon'],
        active: true,
        type: 'battle',
        maxEffectivePerEquipment: 0,
        maxEffectivePerBody: 0
      });
    case 'buff':
      return withDisable({
        id,
        nameKey: `buff.${id}.name`,
        descKey: `buff.${id}.desc`,
        icon: '',
        showInBattleCard: false,
        durationTurns: 1,
        permanent: false,
        kind: 'buff',
        stackPolicy: 'refresh_duration',
        dispellable: true,
        type: ''
      });
    case 'buff_effects':
      return withDisable({
        id,
        buffId: '',
        trigger: 'whileActive',
        priority: 0,
        conditions: [],
        effects: []
      });
    case 'enemies':
      return withDisable({
        id,
        name: '',
        position: 'vanguard',
        level: 1,
        type: 'normal',
        exp: 0,
        drops: {},
        equipment: [],
        skillIds: ['basic_attack']
      });
    case 'world':
      return withDisable({
        id,
        enemyIds: [],
        specialEnemyIds: [],
        specialWaves: [],
        waveSchedules: [],
        normalEnemyCount: { min: 1, max: 1 },
        extraEnemyPerPlayer: { min: 0, max: 0 },
        entryDurationSeconds: 1,
        entryCosts: {},
        customLineupConf: []
      });
    case 'arena_formats':
      return {
        id,
        baseFormatId: 'standard',
        schemaVersion: 1,
        rulesetVersion: 'arena-format-v1',
        balanceRevision: 1,
        nameKey: `game.arena.formats.${id}.name`,
        shortDescKey: `game.arena.formats.${id}.shortDesc`,
        descKey: `game.arena.formats.${id}.desc`,
        icon: 'game-icons:cat',
        sortOrder: 100,
        enabled: true,
        triggers: []
      };
    case 'combat_trial_metrics':
      return {
        id,
        schemaVersion: 2,
        nameKey: `game.combatTrial.metrics.${id}.name`,
        shortNameKey: `game.combatTrial.metrics.${id}.shortName`,
        descKey: `game.combatTrial.metrics.${id}.desc`,
        eventType: 'damage',
        aggregation: 'sum',
        sourceScope: 'player_side',
        targetScope: 'objective_units',
        ranked: true,
        rankDirection: 'desc',
        eligibleEndReasons: [],
        valueFormat: 'number',
        sortOrder: 100,
        enabled: true
      };
    case 'combat_trial_formats':
      return {
        id,
        baseFormatId: '',
        schemaVersion: 2,
        rulesetVersion: 'combat-trial-format-v2',
        balanceRevision: 1,
        nameKey: `game.combatTrial.formats.${id}.name`,
        shortDescKey: `game.combatTrial.formats.${id}.shortDesc`,
        descKey: `game.combatTrial.formats.${id}.desc`,
        icon: 'lucide:target',
        sortOrder: 100,
        enabled: true,
        lineupRules: { positionPolicy: 'shared' },
        metricIds: [],
        limits: [{ kind: 'player_main_actions', value: 20 }],
        encounter: { kind: 'unit_roster', units: [] },
        endConditions: [
          { kind: 'player_team_defeated', reason: 'team_defeated', labelKey: 'game.combatTrial.endReason.teamDefeated' },
          { kind: 'player_main_actions_reached', reason: 'action_budget_reached', labelKey: 'game.combatTrial.endReason.actionBudgetReached' }
        ],
        triggers: []
      };
    case 'attributes':
      return withDisable({
        id,
        icon: '',
        iconId: '',
        emojiIcon: '',
        baseDefault: 0,
        growthType: 'static',
        unit: 'number',
        order: 1,
        category: 'battle',
        expProfileId: '',
        allowBattlePointLevelUp: false,
        relatedActionMainType: '',
        relatedActionEffect: ''
      });
    case 'specialization_trees':
      return {
        id: 'player_combat_specialization',
        schemaVersion: 2,
        rulesetVersion: 'specialization-v6',
        balanceRevision: 1,
        ownerType: 'player',
        nameKey: 'specialization.tree.player_combat_specialization.name',
        descKey: 'specialization.tree.player_combat_specialization.desc',
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
        schools: [
          { id: 'strength', nameKey: 'specialization.school.strength.name', descKey: 'specialization.school.strength.desc', attributeKey: 'strength', ui: { column: 0, color: '#c2413b' } },
          { id: 'dexterity', nameKey: 'specialization.school.dexterity.name', descKey: 'specialization.school.dexterity.desc', attributeKey: 'dexterity', ui: { column: 2, color: '#2f8f6b' } },
          { id: 'intelligence', nameKey: 'specialization.school.intelligence.name', descKey: 'specialization.school.intelligence.desc', attributeKey: 'intelligence', ui: { column: 4, color: '#7656a8' } },
          { id: 'perception', nameKey: 'specialization.school.perception.name', descKey: 'specialization.school.perception.desc', attributeKey: 'perception', ui: { column: 6, color: '#2d7ea3' } },
          { id: 'stamina', nameKey: 'specialization.school.stamina.name', descKey: 'specialization.school.stamina.desc', attributeKey: 'stamina', ui: { column: 8, color: '#9a6d28' } }
        ],
        nodes: [],
        keywords: [],
        ui: { layout: 'five_spoke', schoolOrder: ['strength', 'dexterity', 'intelligence', 'perception', 'stamina'] },
        i18n: { 'zh-CN': {}, 'en-US': {} }
      };
    case 'rarity_definitions':
      return withDisable({
        id,
        order: 1,
        color: '#909399',
        economyMultiplier: 1
      });
    case 'items':
      return withDisable({
        id,
        type: ['loot'],
        rarity: 'common',
        skillForgeEligible: false,
        category: 'loot',
        isEquipment: false,
        baseAttrs: {},
        stackable: true,
        carryableInCombat: false,
        marketListable: true,
        iconId: `item.${id}`,
        emojiIcon: '',
        nameKey: `item.${id}.name`,
        descKey: `item.${id}.desc`,
        effectDescKey: `item.${id}.effect`,
        economyBaseValue: 0,
        useGrantSkillId: '',
        useOperationEffectId: '',
        battleUsableItemId: '',
        useSkillExpGain: 0,
        kittyFeed: {},
        pondFeed: {},
        alchemy: {}
      });
    case 'kitty_personalities':
      return withDisable({
        id,
        nameKey: `kitty.personality.${id}.name`,
        descKey: `kitty.personality.${id}.desc`,
        weight: 1,
        effects: []
      });
    case 'kitty_journal_personality_grants':
      return {
        id,
        matchType: 'category',
        matchValue: '',
        personalityIds: [],
        chance: 0.05
      };
    case 'battle_usable_items':
      return withDisable({
        id,
        itemId: '',
        isTrigger: false,
        cooldownTurns: 1,
        effects: [defaultEffect('heal')]
      });
    case 'skill_type':
    case 'buff_type':
      return withDisable({ id, color: '#909399' });
    case 'weapon_classes':
      return withDisable({ id, icon: '', color: '#909399', order: 1 });
    case 'affix_enchant_rules':
      return withDisable({
        id,
        toolItemId: '',
        consumeCount: 1,
        costs: {},
        failChance: 0,
        rarityWeights: {},
        allowedAffixIds: [],
        allowedAffixScopes: [],
        active: true
      });
    case 'affix_carrier_rules':
      return withDisable({ id, carrierItemId: '', allowedRarityIds: [], maxCustomAffixes: 0, allowedTraitKind: 'both', active: true });
    case 'alchemy_groups':
      return withDisable({
        id,
        nameKey: `alchemy.group.${id}.name`,
        descKey: `alchemy.group.${id}.desc`,
        minAlchemyLevel: 1,
        goldCost: 80,
        durationSec: 8,
        alchemyExp: 1,
        failChanceBase: 0.15,
        minFailChance: 0.15,
        upgradeChanceBase: 0,
        upgradeEntries: [],
        entries: [],
        disabled: false
      });
    case 'item_traits':
      return withDisable({
        id,
        itemId: '',
        traits: [
          {
            traitId: id,
            kind: 'combat',
            family: '',
            effectType: 'add_stat',
            effects: [
              { attr: '', valuePerUnit: { min: 0, max: 0 } }
            ],
            unitBasis: 1,
            maxBasisMultiplier: 500,
            minAlchemyLevel: 1,
            failBaseChance: 0.15,
            failChancePerLevelAbove: 0.005
          }
        ],
        disabled: false
      });
    case 'alchemy_mediums':
      return withDisable({
        id,
        itemId: '',
        nameKey: `alchemy.medium.${id}.name`,
        descKey: `alchemy.medium.${id}.desc`,
        consumePerCraft: 1,
        minAlchemyLevel: 1,
        modifiers: {
          valueMultiplier: 1,
          negativeMultiplier: 1,
          failChanceDelta: 0,
          successFloor: 0,
          maxTraitSlots: 0,
          familyAffinity: {}
        },
        active: true,
        disabled: false
      });
    case 'alchemy_inherit_mediums':
      return withDisable({
        id,
        itemId: '',
        nameKey: `alchemy.inheritMedium.${id}.name`,
        descKey: `alchemy.inheritMedium.${id}.desc`,
        consumePerCraft: 1,
        minAlchemyLevel: 1,
        modifiers: {
          inheritSlots: 3,
          suppressSlots: 1,
          ignoreKeepChance: 0.1,
          positiveIgnoreKeepChanceBase: 0.5,
          positiveIgnoreKeepChanceBonus: 0.1,
          negativeIgnoreKeepChanceBase: 0.9,
          negativeIgnoreSuppressChanceBonus: 0,
          inheritCost: { gold: 200, items: {} },
          suppressCost: { gold: 500, items: {} },
          hybridMergeDivisor: 1.6,
          inbreedMutationChance: 0.8,
          inbreedGoodChance: 0.05,
          inbreedGoodMultiplierRange: { min: 1.1, max: 1.35 },
          inbreedBadMultiplierRange: { min: 0.35, max: 0.8 },
          mutationChance: 0.08,
          precisionScaling: {
            hybridDivisorReductionPerPoint: 0.01,
            hybridDivisorReductionCap: 0.12
          },
          luckyScaling: {
            ignoreKeepChancePerPoint: 0.01,
            ignoreKeepChanceCap: 0.1,
            inbreedGoodChancePerPoint: 0.003,
            inbreedGoodChanceCap: 0.05
          },
          stabilityScaling: {
            inbreedMutationChanceReductionPerPoint: 0.01,
            inbreedMutationChanceReductionCap: 0.12,
            badMultiplierFloorBonusPerPoint: 0.01,
            badMultiplierFloorBonusCap: 0.12
          }
        },
        active: true,
        disabled: false
      });
    case 'alchemy_combo_recipes':
      return withDisable({
        id,
        nameKey: `alchemy.recipe.${id}.name`,
        descKey: `alchemy.recipe.${id}.desc`,
        minAlchemyLevel: 1,
        kind: 'combat',
        inputs: {},
        outputAffix: {
          effectType: 'add_stat',
          params: { attr: '' },
          valueRange: { min: 1, max: 1 }
        },
        outputRarity: null,
        failBaseChance: 0.35,
        failChancePerLevelAbove: 0.04,
        priority: 100,
        disabled: false
      });
    case 'alchemy_transmute_rules':
      return withDisable({
        id,
        itemIds: [],
        priority: 0,
        default: false,
        returnRatio: { minRatio: 0.8, maxRatio: 0.95 },
        failChanceBase: 0.18,
        minFailChance: 0.1,
        luckyChanceBase: 0.05,
        durationSec: 8,
        alchemyExp: 1.25,
        luckyTiers: [],
        minAlchemyLevel: 1,
        disabled: false
      });
    case 'cat_village_commissions':
      return withDisable({
        id,
        nameKey: `catVillage.commission.${id}.name`,
        descKey: `catVillage.commission.${id}.desc`,
        isRumor: false,
        rumorBadge: '',
        rumorMutexGroup: '',
        oncePerAccount: false,
        notRefreshable: false,
        notDeletable: false,
        clearCompletionIds: [],
        prereqRumorIds: [],
        costs: {},
        rewardPool: {},
        reputationMin: 5,
        reputationMax: 15,
        weight: 10,
        minRepLevel: 0
      });
    case 'collectibles':
      return withDisable({
        id,
        itemId: '',
        nameKey: `collectible.${id}.name`,
        descKey: `collectible.${id}.desc`,
        playerMods: {},
        kittyMods: {},
        rarity: 'common',
        categoryId: ''
      });
    case 'title_categories':
      return withDisable({
        id,
        nameKey: `title_category.${id}.name`,
        order: 1
      });
    case 'title_prefixes':
      return withDisable({
        id,
        nameKey: `title_prefix.${id}.name`,
        rarity: 'common',
        category: '',
        order: 1
      });
    case 'title_suffixes':
      return withDisable({
        id,
        nameKey: `title_suffix.${id}.name`,
        rarity: 'common',
        category: '',
        order: 1
      });
    case 'achievement_categories':
      return withDisable({
        id,
        nameKey: `achievement_category.${id}.name`,
        iconId: `achievement.${id}`,
        order: 1,
        hidden: false
      });
    case 'collectible_categories':
      return withDisable({
        id,
        nameKey: `collectible_category.${id}.name`,
        order: 1
      });
    case 'achievements':
      return withDisable({
        id,
        categoryId: 'action',
        nameKey: '',
        descKey: '',
        rarity: 'common',
        iconId: `achievement.${id}`,
        order: 1,
        hidden: false,
        stageProgressMode: 'sequential',
        prereqAchievementIds: [],
        condition: { kind: 'complete_action', actorType: 'player', actionId: '', count: 1 },
        rewards: []
      });
    case 'fishing_species':
      return withDisable({
        id,
        waterId: '',
        rarity: 'common',
        names: {}
      });
    default:
      return withDisable({ id });
  }
}

export function defaultTargeting(targetScope: 'ally' | 'enemy' | 'all' = 'enemy'): DataRow {
  return {
    targetPositions: ['vanguard'],
    secondarySeek: 'front_to_rear',
    targetScope,
    selectRule: 'random',
    selectAttrKey: '',
    fallbackSelfIfEmpty: false,
    targetCount: { base: 1, formula: [], max: 0 },
    targetPredicates: []
  };
}

export function defaultFormula(): DataRow {
  return { baseAdd: 0, terms: [] };
}

export function defaultDamageSpec(damageType = 'physical'): DataRow {
  return {
    damageType,
    amountFormula: defaultFormula(),
    hitCount: { base: 1, formula: [] },
    flatDefensePenetrationFormula: defaultFormula(),
    percentDefensePenetrationFormula: defaultFormula(),
    physicalFlatDefensePenetrationFormula: defaultFormula(),
    physicalPercentDefensePenetrationFormula: defaultFormula(),
    magicalFlatDefensePenetrationFormula: defaultFormula(),
    magicalPercentDefensePenetrationFormula: defaultFormula(),
    extraDamageAmplificationFormula: defaultFormula(),
    bonusAgainstTargetTypes: {}
  };
}

export function defaultEffect(kind: 'damage' | 'heal' | 'resource' | 'status' | 'summon' | 'corpse_explosion' | 'adjust_skill_cooldown' | 'weighted_random_per_target' = 'damage'): DataRow {
  if (kind === 'status') {
    return {
      kind,
      targeting: defaultTargeting('ally'),
      statusPayload: {
        statusIds: [],
        durationBaseTurns: 1,
        durationPerSkillLevelTurns: 0,
        durationFormula: defaultFormula()
      }
    };
  }
  if (kind === 'resource') {
    return {
      kind,
      resource: 'mp',
      targeting: defaultTargeting('ally'),
      amountFormula: defaultFormula()
    };
  }
  if (kind === 'adjust_skill_cooldown') {
    return {
      kind,
      targeting: { ...defaultTargeting('ally'), targetScope: 'self', selectRule: 'self', fallbackSelfIfEmpty: true },
      amount: -1,
      includeBasic: false,
      skillIds: [],
      excludeSkillIds: []
    };
  }
  if (kind === 'weighted_random_per_target') {
    return {
      kind,
      targeting: defaultTargeting('enemy'),
      choices: [
        { id: 'status_1', weight: 1, statusId: '', durationTurns: 1 }
      ]
    };
  }
  if (kind === 'summon') {
    return {
      kind,
      targeting: defaultTargeting('enemy'),
      summonPayload: {
        unitId: '',
        side: 'player',
        countFormula: { baseAdd: 1, terms: [] },
        durationFormula: { baseAdd: 0, terms: [] },
        durationTurns: 0,
        maxCountPerUnit: 0,
        persistAcrossBattles: false,
        onCasterDeath: 'keep_until_duration_end',
        onCapOverflow: 'replace_oldest',
        onDispel: 'despawn_immediately',
        onDurationEnd: 'despawn_immediately',
        statsFormula: {}
      }
    };
  }
  if (kind === 'corpse_explosion') {
    return {
      kind,
      targeting: {
        ...defaultTargeting('enemy'),
        targetPositions: ['vanguard', 'midguard', 'rearguard'],
        selectRule: 'first',
        targetCount: { base: 4, formula: [], max: 4 }
      },
      corpseTargeting: {
        ...defaultTargeting('all'),
        targetPositions: ['vanguard', 'midguard', 'rearguard'],
        selectRule: 'attr_max',
        selectAttrKey: 'hp',
        targetCount: { base: 1, formula: [], max: 1 }
      },
      consumeCorpse: true,
      damageType: 'magical',
      amountFormula: {
        baseAdd: 0,
        terms: [
          { attrKey: 'corpse_max_hp', coefficient: 0.31, perSkillLevel: 0.01 },
          { attrKey: 'perception', coefficient: 0.31, perSkillLevel: 0.01 }
        ]
      }
    };
  }
  const damageType = kind === 'heal' ? 'magical' : 'physical';
  return {
    kind,
    targeting: defaultTargeting(kind === 'heal' ? 'ally' : 'enemy'),
    damageType,
    amountFormula: defaultFormula(),
    ...(kind === 'damage' ? { damageSpec: defaultDamageSpec(damageType), consumeStatusesOnHit: [] } : {})
  };
}
