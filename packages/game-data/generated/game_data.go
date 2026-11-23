package generated

type Item struct {
	ID            string         `json:"id"`
	Type          []string       `json:"type"`
	Rarity        string         `json:"rarity"`
	SkillForgeEligible bool      `json:"skillForgeEligible"`
	Category      string         `json:"category"`
	IsEquipment   bool           `json:"isEquipment"`
	Disabled      bool           `json:"disabled"`
	BaseAttrs     map[string]float64 `json:"baseAttrs"`
	Stackable     bool           `json:"stackable"`
	CarryableInCombat bool       `json:"carryableInCombat"`
	MarketListable bool          `json:"marketListable"`
	MarketPolicy map[string]interface{} `json:"marketPolicy,omitempty"`
	IconID        string         `json:"iconId,omitempty"`
	EmojiIcon     string         `json:"emojiIcon,omitempty"`
	NameKey       string         `json:"nameKey,omitempty"`
	DescKey       string         `json:"descKey,omitempty"`
	EffectDescKey string         `json:"effectDescKey,omitempty"`
	EconomyBaseValue float64     `json:"economyBaseValue,omitempty"`
	UseGrantSkillID string       `json:"useGrantSkillId,omitempty"`
	UseGrantCollectibleID string `json:"useGrantCollectibleId,omitempty"`
	UseOperationEffectID string  `json:"useOperationEffectId,omitempty"`
	BattleUsableItemID string    `json:"battleUsableItemId,omitempty"`
	UseSkillExpGain int64        `json:"useSkillExpGain,omitempty"`
	KittyFeed map[string]int64   `json:"kittyFeed,omitempty"`
	PondFeed *PondFeedConfig    `json:"pondFeed,omitempty"`
	Alchemy *ItemAlchemyMeta    `json:"alchemy,omitempty"`
}

type PondFeedConfig struct {
	Delivery string `json:"delivery,omitempty"`
	Role string `json:"role,omitempty"`
	MaxPortionsPerFishHour float64 `json:"maxPortionsPerFishHour,omitempty"`
		Nutrition float64 `json:"nutrition"`
		FeedClass string `json:"feedClass"`
		RulesetVersion string `json:"rulesetVersion,omitempty"`
		ConsumePriority int `json:"consumePriority,omitempty"`
		Conditions []PondFeedCondition `json:"conditions,omitempty"`
		Effects []PondFeedEffect `json:"effects,omitempty"`
	}

	type PondFeedCondition struct {
		Field string `json:"field"`
		Op string `json:"op"`
		Value any `json:"value"`
	}

	type PondFeedEffect struct {
		Kind string `json:"kind"`
		Stat string `json:"stat"`
		Mode string `json:"mode"`
		ValuePerPortion float64 `json:"valuePerPortion"`
		MaxTargetMultiplier float64 `json:"maxTargetMultiplier,omitempty"`
		Floor *float64 `json:"floor,omitempty"`
		Ceiling *float64 `json:"ceiling,omitempty"`
	}

type ItemAlchemyMeta struct {
	NameAffix AlchemyNameAffix `json:"nameAffix"`
	ConvertExcluded bool `json:"convertExcluded,omitempty"`
	ConvertWeight float64 `json:"convertWeight,omitempty"`
	ConvertWeightMultiplier float64 `json:"convertWeightMultiplier,omitempty"`
}

type AlchemyNameAffix struct {
	PrefixKey string `json:"prefixKey"`
	SuffixKey string `json:"suffixKey"`
}

type Icon struct {
	ID   string `json:"id"`
	Path string `json:"path"`
}

type ActionDefinition struct {
	ID string `json:"id"`
	MainType string `json:"mainType"`
	SubType string `json:"subType"`
	Menu string `json:"menu"`
	Disabled bool `json:"disabled"`
	Rarity string `json:"rarity"`
	IconID string `json:"iconId,omitempty"`
	NameKey string `json:"nameKey"`
	DescKey string `json:"descKey,omitempty"`
	Queueable bool `json:"queueable"`
	DurationS int64 `json:"durationS"`
	Costs map[string]int64 `json:"costs"`
	MaterialInstanceRequirements map[string]ActionMaterialInstanceRequirement `json:"materialInstanceRequirements,omitempty"`
	StatusRequirement map[string]int64 `json:"statusRequirement,omitempty"`
	BuildingRequirement map[string]int64 `json:"buildingRequirement,omitempty"`
	ItemReward map[string]any `json:"itemReward"`
  MainOutput string `json:"mainOutput"`
	StatusExpReward map[string]float64 `json:"statusExpReward"`
	EconomyIndustryTier float64 `json:"economyIndustryTier,omitempty"`
	EconomyDifficultyMultiplier float64 `json:"economyDifficultyMultiplier,omitempty"`
}

type ActionMaterialInstanceRequirement struct {
	EnhanceLevel int64 `json:"enhanceLevel"`
	RequireTemplateAffixes bool `json:"requireTemplateAffixes"`
	Durability string `json:"durability"`
}

type BuildingDefinition struct {
	ID string `json:"id"`
	IconID string `json:"iconId"`
	NameKey string `json:"nameKey"`
	DescKey string `json:"descKey,omitempty"`
	Order int64 `json:"order"`
	MaxLevel int64 `json:"maxLevel"`
	BaseDurationS int64 `json:"baseDurationS"`
	StepDurationS int64 `json:"stepDurationS"`
	BaseCosts map[string]int64 `json:"baseCosts"`
	StepCosts map[string]int64 `json:"stepCosts"`
	BaseStatusRequirement map[string]int64 `json:"baseStatusRequirement"`
	StepStatusRequirement map[string]int64 `json:"stepStatusRequirement"`
	Levels []BuildingLevelDefinition `json:"levels,omitempty"`
}

type BuildingLevelDefinition struct {
	Level int64 `json:"level"`
	DurationS int64 `json:"durationS"`
	Costs map[string]int64 `json:"costs"`
	StatusRequirement map[string]int64 `json:"statusRequirement"`
	Bonuses map[string]int64 `json:"bonuses"`
}

type ActionMainTypeDefinition struct {
	ID string `json:"id"`
	NameKey string `json:"nameKey"`
	DescKey string `json:"descKey,omitempty"`
	Icon string `json:"icon"`
	Color string `json:"color"`
	Order int64 `json:"order"`
	Actions []string `json:"actions"`
}

type ActionSubTypeDefinition struct {
	ID string `json:"id"`
	NameKey string `json:"nameKey"`
	DescKey string `json:"descKey,omitempty"`
	Icon string `json:"icon"`
	Color string `json:"color"`
	Order int64 `json:"order"`
	Actions []string `json:"actions"`
}

type Catalog struct {
	Version int    `json:"version"`
	Items   []Item `json:"items"`
	Icons   []Icon `json:"icons"`
	Actions []ActionDefinition `json:"actions"`
	ActionMainTypes []ActionMainTypeDefinition `json:"action_main_types"`
	ActionSubTypes []ActionSubTypeDefinition `json:"action_sub_types"`
	Buildings []BuildingDefinition `json:"buildings"`
	AchievementCategories []map[string]any `json:"achievement_categories"`
	Achievements []map[string]any `json:"achievements"`
	ActivityMutexRules []map[string]any `json:"activity_mutex_rules"`
	AffixCarrierRules []map[string]any `json:"affix_carrier_rules"`
	AffixDefinitions []map[string]any `json:"affix_definitions"`
	AffixDetachRules []map[string]any `json:"affix_detach_rules"`
	AffixEnchantRules []map[string]any `json:"affix_enchant_rules"`
	AffixSmeltRules []map[string]any `json:"affix_smelt_rules"`
	AlchemyComboRecipes []map[string]any `json:"alchemy_combo_recipes"`
	AlchemyGroups []map[string]any `json:"alchemy_groups"`
	AlchemyInheritMediums []map[string]any `json:"alchemy_inherit_mediums"`
	AlchemyMediums []map[string]any `json:"alchemy_mediums"`
	AlchemyTransmuteRules []map[string]any `json:"alchemy_transmute_rules"`
	ArenaFormats []map[string]any `json:"arena_formats"`
	AttributeExpProfiles []map[string]any `json:"attribute_exp_profiles"`
	Attributes []map[string]any `json:"attributes"`
	BattleUsableItems []map[string]any `json:"battle_usable_items"`
	Buff []map[string]any `json:"buff"`
	BuffEffects []map[string]any `json:"buff_effects"`
	BuffType []map[string]any `json:"buff_type"`
	CatVillageCommissions []map[string]any `json:"cat_village_commissions"`
	CatteryFurniture []map[string]any `json:"cattery_furniture"`
	CollectibleCategories []map[string]any `json:"collectible_categories"`
	Collectibles []map[string]any `json:"collectibles"`
	CombatConfigRules []map[string]any `json:"combat_config_rules"`
	CombatTrialFormats []map[string]any `json:"combat_trial_formats"`
	CombatTrialMetrics []map[string]any `json:"combat_trial_metrics"`
	DropTables []map[string]any `json:"drop_tables"`
	Effects []map[string]any `json:"effects"`
	Enemies []map[string]any `json:"enemies"`
	EquipmentTemplates []map[string]any `json:"equipment_templates"`
	EventsWorld []map[string]any `json:"events_world"`
	FishFoodRecipes []map[string]any `json:"fish_food_recipes"`
	FishingInfusionProfiles []map[string]any `json:"fishing_infusion_profiles"`
	FishingPondSpecies []map[string]any `json:"fishing_pond_species"`
	FishingSpecies []map[string]any `json:"fishing_species"`
	FishingSpots []map[string]any `json:"fishing_spots"`
	FishingTackle []map[string]any `json:"fishing_tackle"`
	FishingWaters []map[string]any `json:"fishing_waters"`
	ItemRuleSets []map[string]any `json:"item_rule_sets"`
	ItemTraits []map[string]any `json:"item_traits"`
	ItemTypes []map[string]any `json:"item_types"`
	KittyJournalPersonalityGrants []map[string]any `json:"kitty_journal_personality_grants"`
	KittyPersonalities []map[string]any `json:"kitty_personalities"`
	MarketFeeTiers []map[string]any `json:"market_fee_tiers"`
	MarketNpcOffers []map[string]any `json:"market_npc_offers"`
	MarketProfiles []map[string]any `json:"market_profiles"`
	OperationEffects []map[string]any `json:"operation_effects"`
	RarityDefinitions []map[string]any `json:"rarity_definitions"`
	SkillAlchemyModules []map[string]any `json:"skill_alchemy_modules"`
	SkillRuleSets []map[string]any `json:"skill_rule_sets"`
	SkillType []map[string]any `json:"skill_type"`
	Skills []map[string]any `json:"skills"`
	SpecializationTrees []map[string]any `json:"specialization_trees"`
	TitleCategories []map[string]any `json:"title_categories"`
	TitlePrefixes []map[string]any `json:"title_prefixes"`
	TitleSuffixes []map[string]any `json:"title_suffixes"`
	WarehouseRules []map[string]any `json:"warehouse_rules"`
	WeaponClasses []map[string]any `json:"weapon_classes"`
	World []map[string]any `json:"world"`
}
