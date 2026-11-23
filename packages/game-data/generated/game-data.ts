export interface Item {
  id: string
  type: string[]
  rarity: string
  skillForgeEligible: boolean
  category: string
  isEquipment: boolean
    disabled: boolean
    baseAttrs: Record<string, number>
  stackable: boolean
  carryableInCombat: boolean
  marketListable: boolean
  marketPolicy?: { fixedPrice: { mode: "instance_variants"; profileId: string }; buyOrders: false; auction: boolean }
  iconId?: string
  emojiIcon?: string
  nameKey?: string
  descKey?: string
  effectDescKey?: string
  economyBaseValue?: number
  useGrantSkillId?: string
  useGrantCollectibleId?: string
  useOperationEffectId?: string
  battleUsableItemId?: string
  useSkillExpGain?: number
  kittyFeed?: {
    battleLevelCapGain: number
  }
	  pondFeed?: {
	    nutrition: number
	    feedClass: string
	    rulesetVersion?: string
	    consumePriority?: number
	    role?: "staple" | "functional"
	    maxPortionsPerFishHour?: number
	    delivery?: "feeder" | "targeted"
	    conditions?: Array<{
	      field: string
	      op: string
	      value: string | number
	    }>
	    effects?: Array<{
	      kind: string
	      stat: string
	      mode: string
	      valuePerPortion: number
	      maxTargetMultiplier?: number
	      floor?: number
	      ceiling?: number
	    }>
	  }
  alchemy?: {
    convertExcluded?: boolean
    convertWeight?: number
    convertWeightMultiplier?: number
    nameAffix?: {
      prefixKey: string
      suffixKey: string
    }
  }
}

export interface Icon {
  id: string
  path: string
}

export interface ActionDefinition {
  id: string
  mainType: string
  subType: string
  menu: string
  disabled: boolean
  rarity: string
  iconId?: string
  nameKey: string
  descKey?: string
  queueable: boolean
  durationS: number
  costs: Record<string, number>
  materialInstanceRequirements?: Record<string, ActionMaterialInstanceRequirement>
  statusRequirement?: Record<string, number>
  buildingRequirement?: Record<string, number>
  itemReward: Record<string, number | ActionItemRewardRule>
  mainOutput?: string
  statusExpReward: Record<string, number>
  economyIndustryTier?: number
  economyDifficultyMultiplier?: number
}

export interface ActionMaterialInstanceRequirement {
  enhanceLevel: number
  requireTemplateAffixes: boolean
  durability: "full"
}

export interface BuildingDefinition {
  id: string
  iconId: string
  nameKey: string
  descKey?: string
  order: number
  maxLevel: number
  baseDurationS: number
  stepDurationS: number
  baseCosts: Record<string, number>
  stepCosts: Record<string, number>
  baseStatusRequirement: Record<string, number>
  stepStatusRequirement: Record<string, number>
  levels?: BuildingLevelDefinition[]
}

export interface BuildingLevelDefinition {
  level: number
  durationS: number
  costs: Record<string, number>
  statusRequirement: Record<string, number>
  bonuses: Record<string, number>
}

export interface ActionItemRewardRule {
  count?: number
  range?: {
    min: number
    max: number
  }
  percent?: number
  grantPolicy?: "rollDefaultAffixes"
  defaultAffixRarityWeights?: Record<string, number>
}

export interface ActionMainTypeDefinition {
  id: string
  nameKey: string
  descKey?: string
  icon: string
  color: string
  order: number
  actions: string[]
}

export interface ActionSubTypeDefinition {
  id: string
  nameKey: string
  descKey?: string
  icon: string
  color: string
  order: number
  actions: string[]
}

export interface BattlefieldEffectStateDefinition {
  id: string
  nameKey: string
  descKey: string
  icon?: string
  displayMode?: "countdown" | "progress"
  counter: {
    source: "turn_consumed"
    actorFilter: {
      sides: Array<"player" | "enemy">
      unitTypes: Array<"player" | "kitty" | "enemy" | "summon">
      includeSummons: boolean
      turnKinds: Array<"ordered" | "extra">
      includePreparation: boolean
    }
    aggregation: "per_event" | "alive_roster_cycle"
    countSuppressedTurns: boolean
    initial: number
    resetTo: number
    carryProgressAcrossWaves: boolean
  }
  triggerEffects: Array<
    | {
        kind: "damage"
        targetScope: "all_alive_players"
        includeSummons: boolean
        damageType: "physical" | "magical"
        amountByDifficulty: { base: number; perLevelMult: number }
      }
    | {
        kind: "weighted_random_per_target"
        targetScope: "alive_players"
        targetFilter: { unitTypes: Array<"player" | "kitty" | "summon">; includeSummons: boolean }
        targetCount: { source: "eligible_count"; divisor: number; rounding: "ceil" | "floor" | "round"; min: number; max: number }
        selection: "uniform_without_replacement"
        choices: Array<{
          id: string
          weight: number
          effect:
            | { kind: "status"; statusId: string; durationTurns: number }
            | { kind: "adjust_skill_cooldown"; amount: number; includeBasic: boolean; excludeSkillIds?: string[] }
        }>
      }
  >
  postTriggerWindows?: Array<{ id: string; duration: number; unit: "player_round" }>
  conditionalModifiers?: Array<{
    kind: "incoming_damage_reduction"
    targetScope: "special_enemies"
    value: number
    inactiveDuringWindow: string
  }>
}

export interface WorldDefinition extends Record<string, unknown> {
  id: string
  nameKey: string
  descKey: string
  battlefieldEffects?: BattlefieldEffectStateDefinition[]
}

export interface Catalog {
  version: number
  items: Item[]
  icons: Icon[]
  actions: ActionDefinition[]
  action_main_types: ActionMainTypeDefinition[]
  action_sub_types: ActionSubTypeDefinition[]
  buildings: BuildingDefinition[]
  achievement_categories: Record<string, unknown>[]
  achievements: Record<string, unknown>[]
  activity_mutex_rules: Record<string, unknown>[]
  affix_carrier_rules: Record<string, unknown>[]
  affix_definitions: Record<string, unknown>[]
  affix_detach_rules: Record<string, unknown>[]
  affix_enchant_rules: Record<string, unknown>[]
  affix_smelt_rules: Record<string, unknown>[]
  alchemy_combo_recipes: Record<string, unknown>[]
  alchemy_groups: Record<string, unknown>[]
  alchemy_inherit_mediums: Record<string, unknown>[]
  alchemy_mediums: Record<string, unknown>[]
  alchemy_transmute_rules: Record<string, unknown>[]
  arena_formats: Record<string, unknown>[]
  attribute_exp_profiles: Record<string, unknown>[]
  attributes: Record<string, unknown>[]
  battle_usable_items: Record<string, unknown>[]
  buff: Record<string, unknown>[]
  buff_effects: Record<string, unknown>[]
  buff_type: Record<string, unknown>[]
  cat_village_commissions: Record<string, unknown>[]
  cattery_furniture: Record<string, unknown>[]
  collectible_categories: Record<string, unknown>[]
  collectibles: Record<string, unknown>[]
  combat_config_rules: Record<string, unknown>[]
  combat_trial_formats: Record<string, unknown>[]
  combat_trial_metrics: Record<string, unknown>[]
  drop_tables: Record<string, unknown>[]
  effects: Record<string, unknown>[]
  enemies: Record<string, unknown>[]
  equipment_templates: Record<string, unknown>[]
  events_world: Record<string, unknown>[]
  fish_food_recipes: Record<string, unknown>[]
  fishing_infusion_profiles: Record<string, unknown>[]
  fishing_pond_species: Record<string, unknown>[]
  fishing_species: Record<string, unknown>[]
  fishing_spots: Record<string, unknown>[]
  fishing_tackle: Record<string, unknown>[]
  fishing_waters: Record<string, unknown>[]
  item_rule_sets: Record<string, unknown>[]
  item_traits: Record<string, unknown>[]
  item_types: Record<string, unknown>[]
  kitty_journal_personality_grants: Record<string, unknown>[]
  kitty_personalities: Record<string, unknown>[]
  market_fee_tiers: Record<string, unknown>[]
  market_npc_offers: Record<string, unknown>[]
  market_profiles: Record<string, unknown>[]
  operation_effects: Record<string, unknown>[]
  rarity_definitions: Record<string, unknown>[]
  skill_alchemy_modules: Record<string, unknown>[]
  skill_rule_sets: Record<string, unknown>[]
  skill_type: Record<string, unknown>[]
  skills: Record<string, unknown>[]
  specialization_trees: Record<string, unknown>[]
  title_categories: Record<string, unknown>[]
  title_prefixes: Record<string, unknown>[]
  title_suffixes: Record<string, unknown>[]
  warehouse_rules: Record<string, unknown>[]
  weapon_classes: Record<string, unknown>[]
  world: WorldDefinition[]
  [key: string]: unknown
}
