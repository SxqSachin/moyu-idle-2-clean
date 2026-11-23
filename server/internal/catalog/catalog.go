package catalog

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"mysecondwebgame/server/internal/catteryfurniture"
	"mysecondwebgame/server/internal/marketpolicy"
)

type Item struct {
	ID                    string               `json:"id"`
	Type                  []string             `json:"type"`
	Rarity                string               `json:"rarity"`
	SkillForgeEligible    bool                 `json:"skillForgeEligible"`
	Category              string               `json:"category"`
	IsEquipment           bool                 `json:"isEquipment"`
	Disabled              bool                 `json:"disabled"`
	IconID                string               `json:"iconId"`
	EmojiIcon             string               `json:"emojiIcon"`
	NameKey               string               `json:"nameKey"`
	DescKey               string               `json:"descKey"`
	EffectDescKey         string               `json:"effectDescKey"`
	EconomyBaseValue      float64              `json:"economyBaseValue"`
	BaseAttrs             map[string]float64   `json:"baseAttrs"`
	Stackable             bool                 `json:"stackable"`
	CarryableInCombat     bool                 `json:"carryableInCombat"`
	MarketListable        bool                 `json:"marketListable"`
	MarketPolicy          *marketpolicy.Policy `json:"marketPolicy,omitempty"`
	UseGrantSkillID       string               `json:"useGrantSkillId"`
	UseGrantCollectibleID string               `json:"useGrantCollectibleId"`
	UseOperationEffectID  string               `json:"useOperationEffectId"`
	BattleUsableItemID    string               `json:"battleUsableItemId"`
	UseSkillExpGain       int64                `json:"useSkillExpGain"`
	KittyFeed             KittyFeedConfig      `json:"kittyFeed,omitempty"`
	PondFeed              *PondFeedConfig      `json:"pondFeed,omitempty"`
	Alchemy               *ItemAlchemyMeta     `json:"alchemy,omitempty"`
}

type KittyFeedConfig struct {
	BattleLevelCapGain int64 `json:"battleLevelCapGain,omitempty"`
}

type PondFeedConfig struct {
	Delivery               string              `json:"delivery,omitempty"`
	Role                   string              `json:"role,omitempty"`
	MaxPortionsPerFishHour float64             `json:"maxPortionsPerFishHour,omitempty"`
	Nutrition              float64             `json:"nutrition"`
	FeedClass              string              `json:"feedClass"`
	RulesetVersion         string              `json:"rulesetVersion,omitempty"`
	ConsumePriority        int                 `json:"consumePriority,omitempty"`
	Conditions             []PondFeedCondition `json:"conditions,omitempty"`
	Effects                []PondFeedEffect    `json:"effects,omitempty"`
}

type PondFeedCondition struct {
	Field string `json:"field"`
	Op    string `json:"op"`
	Value any    `json:"value"`
}

type PondFeedEffect struct {
	Kind                string   `json:"kind"`
	Stat                string   `json:"stat"`
	Mode                string   `json:"mode"`
	ValuePerPortion     float64  `json:"valuePerPortion"`
	MaxTargetMultiplier float64  `json:"maxTargetMultiplier,omitempty"`
	Floor               *float64 `json:"floor,omitempty"`
	Ceiling             *float64 `json:"ceiling,omitempty"`
}

// ItemAlchemyMeta holds alchemy-namespaced metadata that belongs to the item
// itself (i.e., does not depend on which trait the item carries). Keep this
// minimal — purely descriptive item-of-noun fields. Trait DSL (effect/value/
// failure) stays in item_traits.json.
type ItemAlchemyMeta struct {
	NameAffix               AlchemyNameAffix `json:"nameAffix"`
	ConvertExcluded         bool             `json:"convertExcluded,omitempty"`
	ConvertWeight           float64          `json:"convertWeight,omitempty"`
	ConvertWeightMultiplier float64          `json:"convertWeightMultiplier,omitempty"`
}

type Data struct {
	Version                       int                            `json:"version"`
	Items                         []Item                         `json:"items"`
	Skills                        []map[string]any               `json:"skills"`
	SkillTypes                    []map[string]any               `json:"skill_type"`
	Buff                          []map[string]any               `json:"buff"`
	BuffTypes                     []map[string]any               `json:"buff_type"`
	BuffEffects                   []map[string]any               `json:"buff_effects"`
	Enemies                       []map[string]any               `json:"enemies"`
	World                         []map[string]any               `json:"world"`
	ArenaFormats                  []map[string]any               `json:"arena_formats"`
	CombatTrialMetrics            []map[string]any               `json:"combat_trial_metrics"`
	CombatTrialFormats            []map[string]any               `json:"combat_trial_formats"`
	CombatConfigRules             []map[string]any               `json:"combat_config_rules"`
	BattleUsableItems             []map[string]any               `json:"battle_usable_items"`
	OperationEffects              []OperationEffectDefinition    `json:"operation_effects"`
	CatteryFurniture              []catteryfurniture.Definition  `json:"cattery_furniture"`
	MarketProfiles                []marketpolicy.Profile         `json:"market_profiles"`
	FishFoodRecipes               []FishFoodRecipe               `json:"fish_food_recipes"`
	Actions                       []ActionDefinition             `json:"actions"`
	ActionMainTypes               []ActionMainTypeDefinition     `json:"action_main_types"`
	ActionSubTypes                []ActionSubTypeDefinition      `json:"action_sub_types"`
	Buildings                     []BuildingDefinition           `json:"buildings"`
	Attributes                    []AttributeDefinition          `json:"attributes"`
	AttributeExpRules             []AttributeExpProfile          `json:"attribute_exp_profiles"`
	KittyPersonalities            []map[string]any               `json:"kitty_personalities"`
	KittyJournalPersonalityGrants []KittyJournalPersonalityGrant `json:"kitty_journal_personality_grants"`
	RarityDefinitions             []RarityDefinition             `json:"rarity_definitions"`
	CatVillageCommissions         []CatVillageCommission         `json:"cat_village_commissions"`
	Collectibles                  []Collectible                  `json:"collectibles"`
	EquipmentTemplates            []EquipmentTemplate            `json:"equipment_templates"`
	AffixDefinitions              []AffixDefinition              `json:"affix_definitions"`
	AffixDetachRules              []AffixDetachRule              `json:"affix_detach_rules"`
	AffixEnchantRules             []AffixEnchantRule             `json:"affix_enchant_rules"`
	AffixCarrierRules             []AffixCarrierRule             `json:"affix_carrier_rules"`
	AffixSmeltRules               []AffixSmeltRule               `json:"affix_smelt_rules"`
	AlchemyGroups                 []AlchemyGroup                 `json:"alchemy_groups"`
	ItemTraits                    []ItemTraitDefinition          `json:"item_traits"`
	AlchemyComboRecipes           []AlchemyComboRecipe           `json:"alchemy_combo_recipes"`
	AlchemyTransmuteRules         []AlchemyTransmuteRule         `json:"alchemy_transmute_rules"`
	AlchemyInheritMediums         []AlchemyInheritMedium         `json:"alchemy_inherit_mediums"`
	AlchemyMediums                []AlchemyMedium                `json:"alchemy_mediums"`
	SkillAlchemyModules           []SkillAlchemyModule           `json:"skill_alchemy_modules"`
	SpecializationTrees           []SpecializationTree           `json:"specialization_trees"`
	TitleCategories               []map[string]any               `json:"title_categories"`
	TitlePrefixes                 []map[string]any               `json:"title_prefixes"`
	TitleSuffixes                 []map[string]any               `json:"title_suffixes"`
	AchievementCategories         []map[string]any               `json:"achievement_categories"`
	Achievements                  []map[string]any               `json:"achievements"`
	CollectibleCategories         []map[string]any               `json:"collectible_categories"`
	FishingWaters                 []FishingWater                 `json:"fishing_waters"`
	FishingSpots                  []FishingSpot                  `json:"fishing_spots"`
	FishingSpecies                []FishingSpecies               `json:"fishing_species"`
	FishingPondSpecies            []FishingPondSpecies           `json:"fishing_pond_species"`
	FishingTackle                 []FishingTackleDefinition      `json:"fishing_tackle"`
	FishingInfusionProfiles       []FishingInfusionProfile       `json:"fishing_infusion_profiles"`
}

type FishingLocalizedText map[string]string

type FishFoodRecipe struct {
	ID                       string                                   `json:"id"`
	RulesetVersion           string                                   `json:"rulesetVersion"`
	ActionID                 string                                   `json:"actionId"`
	OutputItemID             string                                   `json:"outputItemId"`
	Kind                     string                                   `json:"kind"`
	QualityRulesetVersion    string                                   `json:"qualityRulesetVersion"`
	AllowSpecificIngredients bool                                     `json:"allowSpecificIngredients"`
	FishIngredients          []FishFoodIngredient                     `json:"fishIngredients"`
	ParameterScaling         map[string]FishFoodParameterScale        `json:"parameterScaling,omitempty"`
	LegacyEffectIDs          []string                                 `json:"legacyEffectIds,omitempty"`
	ReplacedEffectIDs        []string                                 `json:"replacedEffectIds,omitempty"`
	MutationLifetimeBonuses  map[string]FishFoodMutationLifetimeBonus `json:"mutationLifetimeBonuses,omitempty"`
	LifetimeScaling          *FishFoodLifetimeScaling                 `json:"lifetimeScaling,omitempty"`
}

type FishFoodLifetimeScaling struct {
	Charges         *FishFoodParameterScale `json:"charges,omitempty"`
	DurationSeconds *FishFoodParameterScale `json:"durationSeconds,omitempty"`
}

type FishFoodMutationLifetimeBonus struct {
	Charges         int64 `json:"charges,omitempty"`
	DurationSeconds int64 `json:"durationSeconds,omitempty"`
}

type FishFoodParameterScale struct {
	Curve             string  `json:"curve,omitempty"`
	MaxBeforeMutation float64 `json:"maxBeforeMutation,omitempty"`
	Length            float64 `json:"length"`
	Weight            float64 `json:"weight"`
	Traits            float64 `json:"traits"`
	Inverse           bool    `json:"inverse,omitempty"`
	Integer           bool    `json:"integer,omitempty"`
	Min               float64 `json:"min"`
	Max               float64 `json:"max"`
}

type FishFoodIngredient struct {
	SpeciesID string `json:"speciesId"`
	Quantity  int    `json:"quantity"`
}

type FishingWater struct {
	ID                   string               `json:"id"`
	Order                int                  `json:"order"`
	RequiredFishingLevel int64                `json:"requiredFishingLevel"`
	Names                FishingLocalizedText `json:"names"`
	Descriptions         FishingLocalizedText `json:"descriptions"`
	BaseTemperatureC     float64              `json:"baseTemperatureC"`
	WaterKind            string               `json:"waterKind"`
}

type FishingSpot struct {
	ID             string               `json:"id"`
	WaterID        string               `json:"waterId"`
	Order          int                  `json:"order"`
	Names          FishingLocalizedText `json:"names"`
	Habitat        string               `json:"habitat"`
	DepthM         float64              `json:"depthM"`
	Oxygen         float64              `json:"oxygen"`
	BaseBiteRate   float64              `json:"baseBiteRate"`
	Methods        []string             `json:"methods"`
	SpeciesWeights map[string]float64   `json:"speciesWeights"`
}

type FishingSpecies struct {
	ID                     string               `json:"id"`
	WaterID                string               `json:"waterId"`
	Rarity                 string               `json:"rarity"`
	Fantasy                bool                 `json:"fantasy"`
	Names                  FishingLocalizedText `json:"names"`
	MinLengthCM            float64              `json:"minLengthCm"`
	MaxLengthCM            float64              `json:"maxLengthCm"`
	MaturityLengthCM       float64              `json:"maturityLengthCm"`
	MaturityAgeDays        int                  `json:"maturityAgeDays"`
	LifespanDays           int                  `json:"lifespanDays"`
	ShapeFactor            float64              `json:"shapeFactor"`
	BaseValue              float64              `json:"baseValue"`
	FightPower             float64              `json:"fightPower"`
	CastingChallenge       float64              `json:"castingChallenge"`
	HookingChallenge       float64              `json:"hookingChallenge"`
	MainLineWearMultiplier float64              `json:"mainLineWearMultiplier"`
	LeaderWearMultiplier   float64              `json:"leaderWearMultiplier"`
	PreferredMethods       []string             `json:"preferredMethods"`
	PreferredLayers        []string             `json:"preferredLayers"`
	Depth                  string               `json:"depth"`
	ActiveHours            []int                `json:"activeHours"`
	WindowWeather          string               `json:"windowWeather,omitempty"`
	IconPath               string               `json:"iconPath"`
}

type FishingPondSpecies struct {
	ID                   string                `json:"id"`
	SpeciesID            string                `json:"speciesId"`
	DietClass            string                `json:"dietClass"`
	SecondaryFeedClasses []string              `json:"secondaryFeedClasses"`
	MaturityAgeDays      int                   `json:"maturityAgeDays"`
	LifespanDays         int                   `json:"lifespanDays"`
	Predation            *FishingPondPredation `json:"predation,omitempty"`
}

type FishingPondPredation struct {
	MaxPreyLengthRatio float64 `json:"maxPreyLengthRatio"`
	HuntBelowSatiety   float64 `json:"huntBelowSatiety"`
	DailyHuntChance    float64 `json:"dailyHuntChance"`
}

type FishingTackleDefinition struct {
	ItemID                   string             `json:"itemId"`
	Slot                     string             `json:"slot"`
	Methods                  []string           `json:"methods"`
	Control                  float64            `json:"control,omitempty"`
	Sensitivity              float64            `json:"sensitivity,omitempty"`
	Casting                  float64            `json:"casting,omitempty"`
	Hooking                  float64            `json:"hooking,omitempty"`
	Efficiency               float64            `json:"efficiency,omitempty"`
	MaxLoadKG                float64            `json:"maxLoadKg,omitempty"`
	WearPerFight             float64            `json:"wearPerFight,omitempty"`
	BiteMultiplier           float64            `json:"biteMultiplier,omitempty"`
	MutationChanceMultiplier float64            `json:"mutationChanceMultiplier,omitempty"`
	WaterBiteModifiers       map[string]float64 `json:"waterBiteModifiers,omitempty"`
	SpeciesWeightModifiers   map[string]float64 `json:"speciesWeightModifiers,omitempty"`
	Terminal                 bool               `json:"terminal,omitempty"`
	TournamentReward         bool               `json:"tournamentReward,omitempty"`
}

type FishingInfusionProfile struct {
	SpeciesID string  `json:"speciesId"`
	TraitKind string  `json:"traitKind"`
	AttrKey   string  `json:"attrKey"`
	UnitScale float64 `json:"unitScale"`
}

type SkillAlchemyModule struct {
	ID                      string                 `json:"id"`
	ItemID                  string                 `json:"itemId"`
	NodeKind                string                 `json:"nodeKind"`
	TypedPorts              SkillAlchemyTypedPorts `json:"typedPorts"`
	CompileOp               map[string]any         `json:"compileOp"`
	ValuePerUnit            SkillAlchemyValueRange `json:"valuePerUnit"`
	UnitBasis               int64                  `json:"unitBasis"`
	MaxBasisMultiplier      int64                  `json:"maxBasisMultiplier"`
	MinAlchemyLevel         int64                  `json:"minAlchemyLevel"`
	FailBaseChance          float64                `json:"failBaseChance"`
	FailChancePerLevelAbove float64                `json:"failChancePerLevelAbove"`
	SuccessChanceModifier   float64                `json:"successChanceModifier"`
	BudgetCreditPerUnit     float64                `json:"budgetCreditPerUnit"`
	NameParts               map[string]string      `json:"nameParts"`
	Icon                    string                 `json:"icon"`
	Disabled                bool                   `json:"disabled"`
}

type SkillAlchemyTypedPorts struct {
	Inputs  []SkillAlchemyPort `json:"inputs"`
	Outputs []SkillAlchemyPort `json:"outputs"`
}

type SkillAlchemyPort struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}

type SkillAlchemyValueRange struct {
	Min float64 `json:"min"`
	Max float64 `json:"max"`
}

type CatVillageCommission struct {
	ID                 string                           `json:"id"`
	Disabled           bool                             `json:"disabled"`
	NameKey            string                           `json:"nameKey"`
	DescKey            string                           `json:"descKey"`
	Costs              map[string]CatVillageItemRange   `json:"costs"`
	CostOptions        []map[string]CatVillageItemRange `json:"costOptions"`
	RewardPool         map[string]CatVillageItemRange   `json:"rewardPool"`
	RewardOptions      []map[string]CatVillageItemRange `json:"rewardOptions"`
	ReputationMin      int64                            `json:"reputationMin"`
	ReputationMax      int64                            `json:"reputationMax"`
	Weight             int                              `json:"weight"`
	MinRepLevel        int                              `json:"minRepLevel"`
	IsRumor            bool                             `json:"isRumor"`
	IsBattle           bool                             `json:"isBattle"`
	RumorBadge         string                           `json:"rumorBadge"`
	RumorMutexGroup    string                           `json:"rumorMutexGroup"`
	OncePerAccount     bool                             `json:"oncePerAccount"`
	NotRefreshable     bool                             `json:"notRefreshable"`
	NotDeletable       bool                             `json:"notDeletable"`
	PrereqRumorIDs     []string                         `json:"prereqRumorIds"`
	PityThreshold      int                              `json:"pityThreshold"`      // 0 = no pity; N = guaranteed after N auto-generated slots
	ClearCompletionIds []string                         `json:"clearCompletionIds"` // on completion, delete these commission IDs from player_rumor_completion and reset their pity counters
}

type Collectible struct {
	ID             string             `json:"id"`
	ItemID         string             `json:"itemId"`
	NameKey        string             `json:"nameKey"`
	DescKey        string             `json:"descKey"`
	PlayerMods     map[string]float64 `json:"playerMods"`
	KittyMods      map[string]float64 `json:"kittyMods"`
	Rarity         string             `json:"rarity"`
	CategoryID     string             `json:"categoryId"`
	Disabled       bool               `json:"disabled"`
	Upgradeable    bool               `json:"upgradeable"`
	LevelMaxLevel  int                `json:"levelMaxLevel"`
	LevelAttrCurve EnhanceAttrCurve   `json:"levelAttrCurve"`
}

type CatVillageItemRange struct {
	Min int64 `json:"min"`
	Max int64 `json:"max"`
}

type KittyJournalPersonalityGrant struct {
	ID             string   `json:"id"`
	MatchType      string   `json:"matchType"`
	MatchValue     string   `json:"matchValue"`
	PersonalityIDs []string `json:"personalityIds"`
	Chance         float64  `json:"chance"`
}

type RarityDefinition struct {
	ID                string  `json:"id"`
	NameKey           string  `json:"nameKey"`
	Order             int64   `json:"order"`
	Color             string  `json:"color"`
	EconomyMultiplier float64 `json:"economyMultiplier"`
}

type EquipmentTemplate struct {
	GrantedSkill               *EquipmentGrantedSkill                      `json:"grantedSkill,omitempty"`
	ID                         string                                      `json:"id"`
	EquipType                  string                                      `json:"equipType"`
	Slot                       string                                      `json:"slot"`
	OccupySlots                []string                                    `json:"occupySlots"`
	EquipTargets               []string                                    `json:"equipTargets,omitempty"`
	WeaponClass                []string                                    `json:"weaponClass"`
	StatusRequirement          map[string]int64                            `json:"statusRequirement"`
	AccessoryGroup             string                                      `json:"accessoryGroup"`
	AffixSlotCount             int64                                       `json:"affixSlotCount"`
	AffixAllowedIDs            []string                                    `json:"affixAllowedIds"`
	BaseAttrs                  map[string]float64                          `json:"baseAttrs"`
	MaxDurability              int64                                       `json:"maxDurability"`
	DecayRate                  float64                                     `json:"decayRate"`
	EnhanceMaxLevel            int64                                       `json:"enhanceMaxLevel"`
	EnhanceDurationS           int64                                       `json:"enhanceDurationS"`
	EnhanceExp                 float64                                     `json:"enhanceExp"`
	EnhanceSuccessRateOverride *float64                                    `json:"enhanceSuccessRateOverride,omitempty"`
	EnhanceProtectItem         []string                                    `json:"enhanceProtectItemId"`
	EnhanceCostCurve           EnhanceCostCurve                            `json:"enhanceCostCurve"`
	EnhanceAttrCurve           EnhanceAttrCurve                            `json:"enhanceAttrCurve"`
	EnhanceAffixMilestonePools map[string][]EnhanceAffixMilestonePoolEntry `json:"enhanceAffixMilestonePools,omitempty"`
	BasicAttackTarget          string                                      `json:"basicAttackTargetDirection"`
	TargetSeekStrategy         string                                      `json:"targetSeekStrategy"`
	BasicAttackFormula         BasicAttackFormula                          `json:"basicAttackFormula"`
	BasicAttackEffects         []map[string]any                            `json:"basicAttackEffects,omitempty"`
	BattleHooks                []map[string]any                            `json:"battleHooks,omitempty"`
	AttrConversions            []map[string]any                            `json:"attrConversions,omitempty"`
	DefaultAffixSlots          []DefaultAffixSlot                          `json:"defaultAffixSlots,omitempty"`
}

type EquipmentGrantedSkill struct {
	SkillID            string `json:"skillId"`
	BaseCapacity       int    `json:"baseCapacity"`
	CapacityMilestones []int  `json:"capacityMilestones"`
}

func (g EquipmentGrantedSkill) Capacity(enhance int) int {
	capacity := g.BaseCapacity
	for _, milestone := range g.CapacityMilestones {
		if enhance >= milestone {
			capacity++
		}
	}
	return capacity
}

type DefaultAffixSlot struct {
	SlotIndex int     `json:"slotIndex"`
	AffixID   string  `json:"affixId"`
	RarityID  string  `json:"rarityId"`
	RollValue float64 `json:"rollValue,omitempty"`
}

type BasicAttackFormula struct {
	DamageType string                   `json:"damageType"`
	Formula    []BasicAttackFormulaTerm `json:"formula"`
	DamageSpec map[string]any           `json:"damageSpec,omitempty"`
}

type BasicAttackFormulaTerm struct {
	AttrKey     string  `json:"attrKey"`
	Coefficient float64 `json:"coefficient"`
}

type EnhanceCostCurve struct {
	Base      map[string]int64            `json:"base"`
	Step      map[string]int64            `json:"step"`
	Overrides map[string]map[string]int64 `json:"overrides"`
}

type EnhanceAttrCurve struct {
	Base      map[string]float64            `json:"base"`
	Step      map[string]float64            `json:"step"`
	Overrides map[string]map[string]float64 `json:"overrides"`
}

type EnhanceAffixMilestonePoolEntry struct {
	AffixID   string   `json:"affixId"`
	RarityID  string   `json:"rarityId"`
	RollValue *float64 `json:"rollValue,omitempty"`
}

type AffixDefinition struct {
	ID                       string                             `json:"id"`
	NameKey                  string                             `json:"nameKey"`
	DescKey                  string                             `json:"descKey"`
	EffectType               string                             `json:"effectType"`
	EffectParams             map[string]any                     `json:"effectParams"`
	Type                     string                             `json:"type"`
	ValuePrecision           int64                              `json:"valuePrecision"`
	MaxEffectivePerEquipment int64                              `json:"maxEffectivePerEquipment"`
	MaxEffectivePerBody      int64                              `json:"maxEffectivePerBody"`
	AllowedRarities          []string                           `json:"allowedRarities"`
	Active                   bool                               `json:"active"`
	SmeltOverride            map[string][]AffixSmeltRewardEntry `json:"smeltOverride,omitempty"`
}

type AffixSmeltRewardEntry struct {
	ItemID   string `json:"itemId"`
	MinCount int64  `json:"minCount"`
	MaxCount int64  `json:"maxCount"`
}

type AffixSmeltRule struct {
	ID            string                             `json:"id"`
	RarityRewards map[string][]AffixSmeltRewardEntry `json:"rarityRewards"`
	Active        bool                               `json:"active"`
}

type AffixDetachRule struct {
	ID                          string  `json:"id"`
	ToolItemID                  string  `json:"toolItemId"`
	DestroyChanceBase           float64 `json:"destroyChanceBase"`
	LuckImpactFactor            float64 `json:"luckImpactFactor"`
	AffixDropBeforeDestroyCheck bool    `json:"affixDropBeforeDestroyCheck"`
	Active                      bool    `json:"active"`
}

type AffixEnchantRule struct {
	ID                 string                 `json:"id"`
	ToolItemID         string                 `json:"toolItemId"`
	ConsumeCount       int64                  `json:"consumeCount"`
	Costs              map[string]int64       `json:"costs"`
	FailChance         float64                `json:"failChance"`
	RarityWeights      map[string]float64     `json:"rarityWeights"`
	AllowedAffix       []string               `json:"allowedAffixIds"`
	AllowedAffixScopes []AffixEnchantScopeDef `json:"allowedAffixScopes"`
	Active             bool                   `json:"active"`
}

type AffixEnchantScopeDef struct {
	EquipType     string   `json:"equipType"`
	Slots         []string `json:"slots"`
	WeaponClasses []string `json:"weaponClasses"`
	AllowedAffix  []string `json:"allowedAffixIds"`
}

type AffixCarrierRule struct {
	ID               string   `json:"id"`
	CarrierItemID    string   `json:"carrierItemId"`
	AllowedRarityIDs []string `json:"allowedRarityIds"`
	MaxCustomAffixes int64    `json:"maxCustomAffixes"`
	AllowedTraitKind string   `json:"allowedTraitKind"`
	Active           bool     `json:"active"`
}

type AlchemyGroup struct {
	ID                string              `json:"id"`
	NameKey           string              `json:"nameKey"`
	DescKey           string              `json:"descKey"`
	MinAlchemyLevel   int64               `json:"minAlchemyLevel"`
	Entries           []AlchemyGroupEntry `json:"entries"`
	GoldCost          int64               `json:"goldCost"`
	DurationSec       float64             `json:"durationSec"`
	AlchemyExp        float64             `json:"alchemyExp"`
	FailChanceBase    float64             `json:"failChanceBase"`
	MinFailChance     float64             `json:"minFailChance"`
	UpgradeChanceBase float64             `json:"upgradeChanceBase"`
	UpgradeEntries    []AlchemyGroupEntry `json:"upgradeEntries"`
	Disabled          bool                `json:"disabled"`
}

type AlchemyGroupEntry struct {
	ItemID string  `json:"itemId"`
	Weight float64 `json:"weight"`
}

// ItemTraitDefinition is the alchemy DSL row for one trait-bearing item.
// It only contains the dynamic side of the trait (effect, value, failure).
// Static item-of-noun naming lives on Item.Alchemy in items.json, accessed via
// Data.ItemAlchemyNameAffix(itemID).
type ItemTraitDefinition struct {
	ItemID string              `json:"itemId"`
	Traits []AlchemyTraitEntry `json:"traits"`
}

type AlchemyNameAffix struct {
	PrefixKey string `json:"prefixKey"`
	SuffixKey string `json:"suffixKey"`
}

// AlchemyTraitEntry is one trait offered by a material.
//
// A trait packages together:
//   - Identity:    TraitID (stable id), Kind ("life"|"combat"), optional Family.
//   - Gating:      MinAlchemyLevel (player level required).
//   - Cost shape:  UnitBasis (qty step), MaxBasisMultiplier (max units per craft).
//   - Risk:        FailBaseChance + FailChancePerLevelAbove + quantity pressure.
//   - Effects:     []AlchemyTraitEffect — multiple per-attribute deltas, each
//     with its own per-unit value range. Values can be NEGATIVE
//     (side-effect/penalty). All effects of one trait share the
//     same UnitBasis/Max/Fail params.
//
// Extension anchors (forward-compatible, no migration needed):
//   - EffectType is a discriminator string. Current MVP only supports
//     "add_stat" (each Effect adds linearly to its target attr). New types
//     can be added; consumers switch on EffectType and ignore unknown values.
//   - Family: optional grouping (e.g., "regen", "offense") for filtering or
//     combo rules that match "any regen trait".
type AlchemyTraitEntry struct {
	TraitID                  string               `json:"traitId"`
	Family                   string               `json:"family,omitempty"`
	Kind                     string               `json:"kind"`
	EffectType               string               `json:"effectType"`
	Effects                  []AlchemyTraitEffect `json:"effects"`
	UnitBasis                int64                `json:"unitBasis"`
	MaxBasisMultiplier       int64                `json:"maxBasisMultiplier"`
	MinAlchemyLevel          int64                `json:"minAlchemyLevel"`
	FailBaseChance           float64              `json:"failBaseChance"`
	FailChancePerLevelAbove  float64              `json:"failChancePerLevelAbove"`
	FailChancePerExtraBasis  float64              `json:"failChancePerExtraBasis"`
	FailChanceQuantityCap    float64              `json:"failChanceQuantityCap"`
	GoldCostPerUnit          int64                `json:"goldCostPerUnit"`
	DurationSecPerUnit       float64              `json:"durationSecPerUnit"`
	AlchemyExpPerUnit        float64              `json:"alchemyExpPerUnit"`
	MaxEffectivePerEquipment int64                `json:"maxEffectivePerEquipment"`
	MaxEffectivePerBody      int64                `json:"maxEffectivePerBody"`
}

// AlchemyTraitEffect is one (attr, per-unit value range) pair within a trait.
// Negative ValuePerUnit encodes a penalty.
type AlchemyTraitEffect struct {
	Attr         string            `json:"attr"`
	ValuePerUnit AlchemyValueRange `json:"valuePerUnit"`
}

type AlchemyValueRange struct {
	Min float64 `json:"min"`
	Max float64 `json:"max"`
}

// AlchemyMedium is an optional craft catalyst consumed by the alchemy bench.
// Mediums apply orthogonal modifiers to the craft outcome, e.g. boosting
// positive values, dampening penalties, biasing failure rate, or amplifying
// effects that match a specific trait family.
//
// Cross-domain reference: ItemID may point to an item that is ALSO used as
// an enchant tool (e.g. moon_essence appears in affix_enchant_rules). Each
// subsystem owns its own rules; the shared item is just the resource.
type AlchemyMedium struct {
	ID              string                 `json:"id"`
	ItemID          string                 `json:"itemId"`
	NameKey         string                 `json:"nameKey"`
	DescKey         string                 `json:"descKey"`
	ConsumePerCraft int64                  `json:"consumePerCraft"`
	MinAlchemyLevel int64                  `json:"minAlchemyLevel"`
	Modifiers       AlchemyMediumModifiers `json:"modifiers"`
	Active          bool                   `json:"active"`
	Disabled        bool                   `json:"disabled"`
}

// AlchemyMediumModifiers describes how a medium reshapes one craft.
//   - ValueMultiplier  : positive effect values get ×this. Default 1.
//   - NegativeMultiplier: negative effect values get ×this. Default 1.
//   - FailChanceDelta  : added to final failChance (negative reduces risk).
//   - SuccessFloor     : final success rate is clamped ≥ this.
//   - FamilyAffinity   : extra multiplicative bonus to positive effects when
//     the trait.family matches a key. E.g. {"offense": 0.20} = +20% on offense.
//   - MaxTraitSlots    : how many distinct material slots this medium unlocks
//     in the bench UI. 0 means "use the engine default" (DefaultBenchMaterialSlots).
//
// Future: rarityBoost, allowedTraitKinds, etc. — add fields, leave defaults safe.
type AlchemyMediumModifiers struct {
	ValueMultiplier    float64            `json:"valueMultiplier"`
	NegativeMultiplier float64            `json:"negativeMultiplier"`
	FailChanceDelta    float64            `json:"failChanceDelta"`
	SuccessFloor       float64            `json:"successFloor"`
	FamilyAffinity     map[string]float64 `json:"familyAffinity"`
	MaxTraitSlots      int                `json:"maxTraitSlots"`
}

type AlchemyInheritMedium struct {
	ID              string                  `json:"id"`
	ItemID          string                  `json:"itemId"`
	NameKey         string                  `json:"nameKey"`
	DescKey         string                  `json:"descKey"`
	ConsumePerCraft int64                   `json:"consumePerCraft"`
	MinAlchemyLevel int64                   `json:"minAlchemyLevel"`
	Modifiers       AlchemyInheritModifiers `json:"modifiers"`
	Active          bool                    `json:"active"`
	Disabled        bool                    `json:"disabled"`
}

type AlchemyInheritModifiers struct {
	InheritSlots                      int                            `json:"inheritSlots"`
	SuppressSlots                     int                            `json:"suppressSlots"`
	IgnoreKeepChance                  float64                        `json:"ignoreKeepChance"`
	PositiveIgnoreKeepChanceBase      float64                        `json:"positiveIgnoreKeepChanceBase"`
	PositiveIgnoreKeepChanceBonus     float64                        `json:"positiveIgnoreKeepChanceBonus"`
	NegativeIgnoreKeepChanceBase      float64                        `json:"negativeIgnoreKeepChanceBase"`
	NegativeIgnoreSuppressChanceBonus float64                        `json:"negativeIgnoreSuppressChanceBonus"`
	InheritCost                       AlchemyInheritCost             `json:"inheritCost"`
	SuppressCost                      AlchemyInheritCost             `json:"suppressCost"`
	HybridMergeDivisor                float64                        `json:"hybridMergeDivisor"`
	InbreedMutationChance             float64                        `json:"inbreedMutationChance"`
	InbreedGoodChance                 float64                        `json:"inbreedGoodChance"`
	InbreedGoodMultiplierRange        AlchemyInheritMultiplierRange  `json:"inbreedGoodMultiplierRange"`
	InbreedBadMultiplierRange         AlchemyInheritMultiplierRange  `json:"inbreedBadMultiplierRange"`
	MutationChance                    float64                        `json:"mutationChance"`
	PrecisionScaling                  AlchemyInheritPrecisionScaling `json:"precisionScaling"`
	LuckyScaling                      AlchemyInheritLuckyScaling     `json:"luckyScaling"`
	StabilityScaling                  AlchemyInheritStabilityScaling `json:"stabilityScaling"`
}

type AlchemyInheritCost struct {
	Gold  int64            `json:"gold"`
	Items map[string]int64 `json:"items"`
}

type AlchemyInheritMultiplierRange struct {
	Min float64 `json:"min"`
	Max float64 `json:"max"`
}

type AlchemyInheritPrecisionScaling struct {
	HybridDivisorReductionPerPoint float64 `json:"hybridDivisorReductionPerPoint"`
	HybridDivisorReductionCap      float64 `json:"hybridDivisorReductionCap"`
}

type AlchemyInheritLuckyScaling struct {
	IgnoreKeepChancePerPoint  float64 `json:"ignoreKeepChancePerPoint"`
	IgnoreKeepChanceCap       float64 `json:"ignoreKeepChanceCap"`
	InbreedGoodChancePerPoint float64 `json:"inbreedGoodChancePerPoint"`
	InbreedGoodChanceCap      float64 `json:"inbreedGoodChanceCap"`
}

type AlchemyInheritStabilityScaling struct {
	InbreedMutationChanceReductionPerPoint float64 `json:"inbreedMutationChanceReductionPerPoint"`
	InbreedMutationChanceReductionCap      float64 `json:"inbreedMutationChanceReductionCap"`
	BadMultiplierFloorBonusPerPoint        float64 `json:"badMultiplierFloorBonusPerPoint"`
	BadMultiplierFloorBonusCap             float64 `json:"badMultiplierFloorBonusCap"`
}

// DefaultBenchMaterialSlots is the cap on distinct trait-materials in one
// craft when no medium is used. Mediums may raise this via MaxTraitSlots.
const DefaultBenchMaterialSlots = 3

// EffectiveBenchSlots returns the trait-material slot cap for a given medium.
// Returns DefaultBenchMaterialSlots when m is nil or its MaxTraitSlots is 0/<=0.
func EffectiveBenchSlots(m *AlchemyMedium) int {
	if m != nil && m.Modifiers.MaxTraitSlots > 0 {
		return m.Modifiers.MaxTraitSlots
	}
	return DefaultBenchMaterialSlots
}

type AlchemyComboRecipe struct {
	ID                      string                  `json:"id"`
	NameKey                 string                  `json:"nameKey"`
	DescKey                 string                  `json:"descKey"`
	SummaryKey              string                  `json:"summaryKey,omitempty"`
	MinAlchemyLevel         int64                   `json:"minAlchemyLevel"`
	Kind                    string                  `json:"kind"`
	Inputs                  map[string]int64        `json:"inputs"`
	OutputAffix             AlchemyComboOutputAffix `json:"outputAffix"`
	OutputRarity            *string                 `json:"outputRarity"`
	ExtraGoldCost           int64                   `json:"extraGoldCost"`
	ExtraDurationSec        float64                 `json:"extraDurationSec"`
	FailBaseChance          float64                 `json:"failBaseChance"`
	FailChancePerLevelAbove float64                 `json:"failChancePerLevelAbove"`
	Priority                int64                   `json:"priority"`
	Disabled                bool                    `json:"disabled"`
}

type AlchemyComboOutputAffix struct {
	EffectType string            `json:"effectType"`
	Params     map[string]any    `json:"params"`
	ValueRange AlchemyValueRange `json:"valueRange"`
}

type AlchemyTransmuteRule struct {
	ID                                string                 `json:"id"`
	ItemIDs                           []string               `json:"itemIds"`
	Priority                          int64                  `json:"priority"`
	Default                           bool                   `json:"default"`
	ReturnRatio                       AlchemyReturnRatio     `json:"returnRatio"`
	FailChanceBase                    float64                `json:"failChanceBase"`
	MinFailChance                     float64                `json:"minFailChance"`
	LuckyChanceBase                   float64                `json:"luckyChanceBase"`
	DurationSec                       float64                `json:"durationSec"`
	RarityDurationAddSecByRarityIndex []float64              `json:"rarityDurationAddSecByRarityIndex"`
	AlchemyExp                        float64                `json:"alchemyExp"`
	LuckyTiers                        []AlchemyTransmuteTier `json:"luckyTiers"`
	MinAlchemyLevel                   int64                  `json:"minAlchemyLevel"`
	Disabled                          bool                   `json:"disabled"`
}

type AlchemyReturnRatio struct {
	MinRatio float64 `json:"minRatio"`
	MaxRatio float64 `json:"maxRatio"`
}

type AlchemyTransmuteTier struct {
	ID     string  `json:"id"`
	Weight float64 `json:"weight"`
	Kind   string  `json:"kind"`
	Value  float64 `json:"value"`
	ItemID string  `json:"itemId"`
	Min    int64   `json:"min"`
	Max    int64   `json:"max"`
}

type AttributeDefinition struct {
	ID                      string  `json:"id"`
	IconID                  string  `json:"iconId,omitempty"`
	EmojiIcon               string  `json:"emojiIcon,omitempty"`
	NameKey                 string  `json:"nameKey"`
	DescKey                 string  `json:"descKey"`
	BaseDefault             float64 `json:"baseDefault"`
	GrowthType              string  `json:"growthType"`
	ExpProfileID            string  `json:"expProfileId,omitempty"`
	Unit                    string  `json:"unit"`
	Order                   int64   `json:"order"`
	Category                string  `json:"category"`
	AllowBattlePointLevelUp bool    `json:"allowBattlePointLevelUp"`
	RelatedActionMainType   string  `json:"relatedActionMainType,omitempty"`
	RelatedActionEffect     string  `json:"relatedActionEffect,omitempty"`
}

type AttributeExpProfile struct {
	ID          string `json:"id"`
	Attr        string `json:"attr"`
	Level       int64  `json:"level"`
	ExpRequired int64  `json:"expRequired"`
}

type ActionDefinition struct {
	ID                           string                                       `json:"id"`
	MainType                     string                                       `json:"mainType"`
	SubType                      string                                       `json:"subType"`
	Menu                         string                                       `json:"menu"`
	Disabled                     bool                                         `json:"disabled"`
	IconID                       string                                       `json:"iconId,omitempty"`
	NameKey                      string                                       `json:"nameKey"`
	DescKey                      string                                       `json:"descKey,omitempty"`
	Queueable                    bool                                         `json:"queueable"`
	DurationS                    int64                                        `json:"durationS"`
	Costs                        map[string]int64                             `json:"costs"`
	MaterialInstanceRequirements map[string]ActionMaterialInstanceRequirement `json:"materialInstanceRequirements,omitempty"`
	StatusRequirement            map[string]int64                             `json:"statusRequirement"`
	BuildingRequirement          map[string]int64                             `json:"buildingRequirement"`
	ItemReward                   map[string]ActionItemReward                  `json:"itemReward"`
	MainOutput                   string                                       `json:"mainOutput"`
	StatusExpReward              map[string]float64                           `json:"statusExpReward"`
	EconomyIndustryTier          float64                                      `json:"economyIndustryTier"`
	EconomyDifficultyMultiplier  float64                                      `json:"economyDifficultyMultiplier"`
}

type OperationEffectDefinition struct {
	ID             string                  `json:"id"`
	RulesetVersion string                  `json:"rulesetVersion"`
	Domain         string                  `json:"domain"`
	Kind           string                  `json:"kind"`
	NameKey        string                  `json:"nameKey"`
	DescKey        string                  `json:"descKey"`
	IconID         string                  `json:"iconId,omitempty"`
	Priority       int                     `json:"priority,omitempty"`
	AllowedTargets []string                `json:"allowedTargets"`
	Lifetime       OperationEffectLifetime `json:"lifetime"`
	Stacking       OperationEffectStacking `json:"stacking"`
	Selector       OperationEffectSelector `json:"selector,omitempty"`
	Params         json.RawMessage         `json:"params"`
}

type OperationEffectLifetime struct {
	Kind            string `json:"kind"`
	DurationSeconds int64  `json:"durationSeconds,omitempty"`
	Charges         int64  `json:"charges,omitempty"`
}

type OperationEffectStacking struct {
	Exclusive          bool   `json:"exclusive,omitempty"`
	Key                string `json:"key"`
	Mode               string `json:"mode"`
	MaxDurationSeconds int64  `json:"maxDurationSeconds,omitempty"`
	MaxCharges         int64  `json:"maxCharges,omitempty"`
}

type OperationEffectSelector struct {
	ActionIDs []string `json:"actionIds,omitempty"`
	MainTypes []string `json:"mainTypes,omitempty"`
	SubTypes  []string `json:"subTypes,omitempty"`
}

type ActionMaterialInstanceRequirement struct {
	EnhanceLevel           int64  `json:"enhanceLevel"`
	RequireTemplateAffixes bool   `json:"requireTemplateAffixes"`
	Durability             string `json:"durability"`
}

type BuildingDefinition struct {
	ID                    string                    `json:"id"`
	IconID                string                    `json:"iconId"`
	NameKey               string                    `json:"nameKey"`
	DescKey               string                    `json:"descKey"`
	Order                 int64                     `json:"order"`
	MaxLevel              int64                     `json:"maxLevel"`
	BaseDurationS         int64                     `json:"baseDurationS"`
	StepDurationS         int64                     `json:"stepDurationS"`
	BaseCosts             map[string]int64          `json:"baseCosts"`
	StepCosts             map[string]int64          `json:"stepCosts"`
	BaseStatusRequirement map[string]int64          `json:"baseStatusRequirement"`
	StepStatusRequirement map[string]int64          `json:"stepStatusRequirement"`
	Levels                []BuildingLevelDefinition `json:"levels,omitempty"`
}

type BuildingLevelDefinition struct {
	Level             int64            `json:"level"`
	DurationS         int64            `json:"durationS"`
	Costs             map[string]int64 `json:"costs"`
	StatusRequirement map[string]int64 `json:"statusRequirement"`
	Bonuses           map[string]int64 `json:"bonuses"`
}

type ActionItemReward struct {
	Count                     *int64             `json:"count,omitempty"`
	Range                     *ActionRewardRange `json:"range,omitempty"`
	Percent                   *float64           `json:"percent,omitempty"`
	GrantPolicy               string             `json:"grantPolicy,omitempty"`
	DefaultAffixRarityWeights map[string]float64 `json:"defaultAffixRarityWeights,omitempty"`
}

type ActionRewardRange struct {
	Min int64 `json:"min"`
	Max int64 `json:"max"`
}

func (r *ActionItemReward) UnmarshalJSON(data []byte) error {
	if len(data) == 0 || string(data) == "null" {
		*r = ActionItemReward{}
		return nil
	}
	var direct int64
	if err := json.Unmarshal(data, &direct); err == nil {
		r.Count = &direct
		r.Range = nil
		r.Percent = nil
		return nil
	}
	type alias ActionItemReward
	var parsed alias
	if err := json.Unmarshal(data, &parsed); err != nil {
		return err
	}
	*r = ActionItemReward(parsed)
	return nil
}

func (r ActionItemReward) MarshalJSON() ([]byte, error) {
	if r.Count != nil && r.Range == nil && r.Percent == nil && r.GrantPolicy == "" && len(r.DefaultAffixRarityWeights) == 0 {
		return json.Marshal(*r.Count)
	}
	type alias ActionItemReward
	return json.Marshal(alias(r))
}

type ActionMainTypeDefinition struct {
	ID      string   `json:"id"`
	NameKey string   `json:"nameKey"`
	DescKey string   `json:"descKey,omitempty"`
	Icon    string   `json:"icon"`
	Color   string   `json:"color"`
	Order   int64    `json:"order"`
	Actions []string `json:"actions"`
}

type ActionSubTypeDefinition struct {
	ID      string   `json:"id"`
	NameKey string   `json:"nameKey"`
	DescKey string   `json:"descKey,omitempty"`
	Icon    string   `json:"icon"`
	Color   string   `json:"color"`
	Order   int64    `json:"order"`
	Actions []string `json:"actions"`
}

func Load(path string) (Data, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Data{}, err
	}
	var d Data
	if err := json.Unmarshal(b, &d); err != nil {
		return Data{}, err
	}
	if err := d.ValidateMarketPolicies(); err != nil {
		return Data{}, err
	}
	templateIDs := make(map[string]struct{}, len(d.EquipmentTemplates))
	itemRarityByID := make(map[string]string, len(d.Items))
	for idx := range d.EquipmentTemplates {
		row := &d.EquipmentTemplates[idx]
		if row.ID == "" {
			continue
		}
		if strings.TrimSpace(strings.ToLower(row.EquipType)) == "work" {
			row.EquipType = "work"
		} else {
			row.EquipType = "battle"
		}
		templateIDs[row.ID] = struct{}{}
		if row.BaseAttrs == nil {
			row.BaseAttrs = map[string]float64{}
		}
		if row.OccupySlots == nil {
			row.OccupySlots = []string{}
		}
		if row.StatusRequirement == nil {
			row.StatusRequirement = map[string]int64{}
		}
		if row.AffixAllowedIDs == nil {
			row.AffixAllowedIDs = []string{}
		}
		if row.MaxDurability <= 0 {
			row.MaxDurability = 100
		}
		if row.DecayRate < 0 {
			row.DecayRate = 0
		}
		if row.EnhanceMaxLevel < 0 {
			row.EnhanceMaxLevel = 0
		}
		if row.EnhanceDurationS <= 0 {
			row.EnhanceDurationS = 1
		}
		if row.EnhanceExp < 0 {
			row.EnhanceExp = 0
		}
		if row.EnhanceCostCurve.Base == nil {
			row.EnhanceCostCurve.Base = map[string]int64{}
		}
		if row.EnhanceCostCurve.Step == nil {
			row.EnhanceCostCurve.Step = map[string]int64{}
		}
		if row.EnhanceCostCurve.Overrides == nil {
			row.EnhanceCostCurve.Overrides = map[string]map[string]int64{}
		}
		if row.EnhanceAttrCurve.Base == nil {
			row.EnhanceAttrCurve.Base = map[string]float64{}
		}
		if row.EnhanceAttrCurve.Step == nil {
			row.EnhanceAttrCurve.Step = map[string]float64{}
		}
		if row.EnhanceAttrCurve.Overrides == nil {
			row.EnhanceAttrCurve.Overrides = map[string]map[string]float64{}
		}
	}
	for idx := range d.Items {
		item := &d.Items[idx]
		if item.Type == nil {
			item.Type = []string{}
		}
		_, ok := templateIDs[item.ID]
		item.IsEquipment = ok
		if strings.TrimSpace(item.Category) == "" {
			if item.IsEquipment {
				item.Category = "equipment"
			} else if len(item.Type) > 0 {
				item.Category = strings.TrimSpace(item.Type[0])
			}
		}
		if item.EconomyBaseValue < 0 {
			item.EconomyBaseValue = 0
		}
		itemRarityByID[item.ID] = strings.TrimSpace(strings.ToLower(item.Rarity))
	}
	for idx := range d.RarityDefinitions {
		row := &d.RarityDefinitions[idx]
		if row.EconomyMultiplier <= 0 {
			row.EconomyMultiplier = 1
		}
	}
	for idx := range d.AffixDefinitions {
		row := &d.AffixDefinitions[idx]
		if row.EffectParams == nil {
			row.EffectParams = map[string]any{}
		}
		if row.AllowedRarities == nil {
			row.AllowedRarities = []string{}
		}
		if row.ID == "" {
			continue
		}
		if strings.HasPrefix(row.ID, "affix.") {
			row.NameKey = row.ID + ".name"
			row.DescKey = row.ID + ".desc"
		} else {
			row.NameKey = "affix." + row.ID + ".name"
			row.DescKey = "affix." + row.ID + ".desc"
		}
		if strings.TrimSpace(strings.ToLower(row.Type)) == "work" {
			row.Type = "work"
		} else {
			row.Type = "battle"
		}
		if row.MaxEffectivePerEquipment < 0 {
			row.MaxEffectivePerEquipment = 0
		}
		if row.MaxEffectivePerBody < 0 {
			row.MaxEffectivePerBody = 0
		}
	}
	for idx := range d.AffixEnchantRules {
		row := &d.AffixEnchantRules[idx]
		if row.ConsumeCount <= 0 {
			row.ConsumeCount = 1
		}
		if row.FailChance < 0 {
			row.FailChance = 0
		}
		if row.FailChance > 1 {
			row.FailChance = 1
		}
		if row.RarityWeights == nil {
			row.RarityWeights = map[string]float64{}
		}
		if row.AllowedAffix == nil {
			row.AllowedAffix = []string{}
		}
		if row.AllowedAffixScopes == nil {
			row.AllowedAffixScopes = []AffixEnchantScopeDef{}
			continue
		}
		for scopeIdx := range row.AllowedAffixScopes {
			scope := &row.AllowedAffixScopes[scopeIdx]
			scope.EquipType = strings.TrimSpace(scope.EquipType)
			if scope.Slots == nil {
				scope.Slots = []string{}
			}
			for slotIdx := range scope.Slots {
				scope.Slots[slotIdx] = strings.TrimSpace(scope.Slots[slotIdx])
			}
			if scope.WeaponClasses == nil {
				scope.WeaponClasses = []string{}
			}
			for classIdx := range scope.WeaponClasses {
				scope.WeaponClasses[classIdx] = strings.TrimSpace(scope.WeaponClasses[classIdx])
			}
			if scope.AllowedAffix == nil {
				scope.AllowedAffix = []string{}
			}
			for affixIdx := range scope.AllowedAffix {
				scope.AllowedAffix[affixIdx] = strings.TrimSpace(scope.AllowedAffix[affixIdx])
			}
		}
	}
	for idx := range d.AffixCarrierRules {
		row := &d.AffixCarrierRules[idx]
		if row.AllowedRarityIDs == nil {
			row.AllowedRarityIDs = []string{}
		}
		if row.AllowedTraitKind == "" {
			row.AllowedTraitKind = "both"
		}
	}
	for idx := range d.ItemTraits {
		row := &d.ItemTraits[idx]
		for traitIdx := range row.Traits {
			if row.Traits[traitIdx].Effects == nil {
				row.Traits[traitIdx].Effects = []AlchemyTraitEffect{}
			}
			if row.Traits[traitIdx].UnitBasis <= 0 {
				row.Traits[traitIdx].UnitBasis = 1
			}
			if row.Traits[traitIdx].MaxBasisMultiplier <= 0 {
				row.Traits[traitIdx].MaxBasisMultiplier = 1
			}
			if row.Traits[traitIdx].GoldCostPerUnit < 0 {
				row.Traits[traitIdx].GoldCostPerUnit = 0
			}
			if row.Traits[traitIdx].DurationSecPerUnit < 0 {
				row.Traits[traitIdx].DurationSecPerUnit = 0
			}
			if row.Traits[traitIdx].AlchemyExpPerUnit < 0 {
				row.Traits[traitIdx].AlchemyExpPerUnit = 0
			}
		}
	}
	for idx := range d.AlchemyGroups {
		row := &d.AlchemyGroups[idx]
		if row.UpgradeEntries == nil {
			row.UpgradeEntries = []AlchemyGroupEntry{}
		}
		for _, entry := range row.UpgradeEntries {
			itemID := strings.TrimSpace(entry.ItemID)
			if itemRarityByID[itemID] == "mythic" {
				return Data{}, fmt.Errorf("alchemy group %q upgrade entry %q cannot produce mythic items", row.ID, itemID)
			}
		}
		if row.GoldCost < 0 {
			row.GoldCost = 0
		}
		if row.DurationSec < 0 {
			row.DurationSec = 0
		}
		if row.AlchemyExp < 0 {
			row.AlchemyExp = 0
		}
		if row.FailChanceBase < 0 {
			row.FailChanceBase = 0
		}
		if row.FailChanceBase > 1 {
			row.FailChanceBase = 1
		}
		if row.MinFailChance < 0 {
			row.MinFailChance = 0
		}
		if row.MinFailChance > 1 {
			row.MinFailChance = 1
		}
		if row.UpgradeChanceBase < 0 {
			row.UpgradeChanceBase = 0
		}
		if row.UpgradeChanceBase > 1 {
			row.UpgradeChanceBase = 1
		}
	}
	for idx := range d.AlchemyComboRecipes {
		row := &d.AlchemyComboRecipes[idx]
		if row.Inputs == nil {
			row.Inputs = map[string]int64{}
		}
		if row.OutputAffix.Params == nil {
			row.OutputAffix.Params = map[string]any{}
		}
		if row.ExtraGoldCost < 0 {
			row.ExtraGoldCost = 0
		}
		if row.ExtraDurationSec < 0 {
			row.ExtraDurationSec = 0
		}
	}
	for idx := range d.AlchemyTransmuteRules {
		row := &d.AlchemyTransmuteRules[idx]
		if row.ItemIDs == nil {
			row.ItemIDs = []string{}
		}
		if row.FailChanceBase < 0 {
			row.FailChanceBase = 0
		}
		if row.FailChanceBase > 1 {
			row.FailChanceBase = 1
		}
		if row.MinFailChance < 0 {
			row.MinFailChance = 0
		}
		if row.MinFailChance > 1 {
			row.MinFailChance = 1
		}
		if row.LuckyChanceBase < 0 {
			row.LuckyChanceBase = 0
		}
		if row.LuckyChanceBase > 1 {
			row.LuckyChanceBase = 1
		}
		if row.DurationSec < 0 {
			row.DurationSec = 0
		}
		for addIdx := range row.RarityDurationAddSecByRarityIndex {
			if row.RarityDurationAddSecByRarityIndex[addIdx] < 0 {
				row.RarityDurationAddSecByRarityIndex[addIdx] = 0
			}
		}
		if row.AlchemyExp < 0 {
			row.AlchemyExp = 0
		}
	}
	for idx := range d.AlchemyMediums {
		row := &d.AlchemyMediums[idx]
		if row.Modifiers.FamilyAffinity == nil {
			row.Modifiers.FamilyAffinity = map[string]float64{}
		}
		if row.Modifiers.ValueMultiplier == 0 {
			row.Modifiers.ValueMultiplier = 1
		}
		if row.Modifiers.NegativeMultiplier == 0 {
			row.Modifiers.NegativeMultiplier = 1
		}
		if row.ConsumePerCraft <= 0 {
			row.ConsumePerCraft = 1
		}
	}
	for idx := range d.AlchemyInheritMediums {
		row := &d.AlchemyInheritMediums[idx]
		if row.Modifiers.InheritSlots < 0 {
			row.Modifiers.InheritSlots = 0
		}
		if row.Modifiers.SuppressSlots < 0 {
			row.Modifiers.SuppressSlots = 0
		}
		row.Modifiers.IgnoreKeepChance = clampCatalogChance(row.Modifiers.IgnoreKeepChance)
		if row.Modifiers.InheritCost.Gold < 0 {
			row.Modifiers.InheritCost.Gold = 0
		}
		if row.Modifiers.InheritCost.Items == nil {
			row.Modifiers.InheritCost.Items = map[string]int64{}
		}
		for itemID, qty := range row.Modifiers.InheritCost.Items {
			if strings.TrimSpace(itemID) == "" || qty <= 0 {
				delete(row.Modifiers.InheritCost.Items, itemID)
			}
		}
		if row.Modifiers.SuppressCost.Gold < 0 {
			row.Modifiers.SuppressCost.Gold = 0
		}
		if row.Modifiers.SuppressCost.Items == nil {
			row.Modifiers.SuppressCost.Items = map[string]int64{}
		}
		for itemID, qty := range row.Modifiers.SuppressCost.Items {
			if strings.TrimSpace(itemID) == "" || qty <= 0 {
				delete(row.Modifiers.SuppressCost.Items, itemID)
			}
		}
		if row.Modifiers.HybridMergeDivisor <= 0 {
			row.Modifiers.HybridMergeDivisor = 1
		}
		row.Modifiers.InbreedMutationChance = clampCatalogChance(row.Modifiers.InbreedMutationChance)
		row.Modifiers.InbreedGoodChance = clampCatalogChance(row.Modifiers.InbreedGoodChance)
		if row.Modifiers.InbreedGoodMultiplierRange.Min <= 0 {
			row.Modifiers.InbreedGoodMultiplierRange.Min = 1
		}
		if row.Modifiers.InbreedGoodMultiplierRange.Max < row.Modifiers.InbreedGoodMultiplierRange.Min {
			row.Modifiers.InbreedGoodMultiplierRange.Max = row.Modifiers.InbreedGoodMultiplierRange.Min
		}
		if row.Modifiers.InbreedBadMultiplierRange.Min <= 0 {
			row.Modifiers.InbreedBadMultiplierRange.Min = 1
		}
		if row.Modifiers.InbreedBadMultiplierRange.Max < row.Modifiers.InbreedBadMultiplierRange.Min {
			row.Modifiers.InbreedBadMultiplierRange.Max = row.Modifiers.InbreedBadMultiplierRange.Min
		}
		if row.Modifiers.MutationChance < 0 {
			row.Modifiers.MutationChance = 0
		}
		if row.Modifiers.MutationChance > 1 {
			row.Modifiers.MutationChance = 1
		}
		if row.ConsumePerCraft <= 0 {
			row.ConsumePerCraft = 1
		}
	}
	if err := validateFishingPondBuilding(d.Buildings); err != nil {
		return Data{}, err
	}
	return d, nil
}

func validateFishingPondBuilding(buildings []BuildingDefinition) error {
	for _, building := range buildings {
		if strings.TrimSpace(building.ID) != "fish_pond" {
			continue
		}
		if building.MaxLevel <= 0 || int64(len(building.Levels)) < building.MaxLevel {
			return fmt.Errorf("building %q must define every level through maxLevel", building.ID)
		}
		levels := make(map[int64]BuildingLevelDefinition, len(building.Levels))
		for _, level := range building.Levels {
			levels[level.Level] = level
		}
		for level := int64(1); level <= building.MaxLevel; level++ {
			definition, ok := levels[level]
			if !ok {
				return fmt.Errorf("building %q is missing level %d", building.ID, level)
			}
			if definition.Bonuses["fishCapacity"] <= 0 {
				return fmt.Errorf("building %q level %d must define a positive fishCapacity", building.ID, level)
			}
			if definition.Bonuses["nurseryCapacity"] <= 0 {
				return fmt.Errorf("building %q level %d must define a positive nurseryCapacity", building.ID, level)
			}
		}
		return nil
	}
	return nil
}

// AlchemyMediumByID returns an enabled medium by id, or nil.
func (d Data) AlchemyMediumByID(id string) *AlchemyMedium {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil
	}
	for idx := range d.AlchemyMediums {
		row := &d.AlchemyMediums[idx]
		if row.Disabled || !row.Active {
			continue
		}
		if strings.TrimSpace(row.ID) == id {
			return row
		}
	}
	return nil
}

// AlchemyMediumByItemID returns an enabled medium by underlying itemId.
// Useful when the client passes the item id (e.g., "moon_essence") rather
// than the medium id ("medium.moon_essence").
func (d Data) AlchemyMediumByItemID(itemID string) *AlchemyMedium {
	itemID = strings.TrimSpace(itemID)
	if itemID == "" {
		return nil
	}
	for idx := range d.AlchemyMediums {
		row := &d.AlchemyMediums[idx]
		if row.Disabled || !row.Active {
			continue
		}
		if strings.TrimSpace(row.ItemID) == itemID {
			return row
		}
	}
	return nil
}

func (d Data) AlchemyInheritMediumByItemID(itemID string) *AlchemyInheritMedium {
	itemID = strings.TrimSpace(itemID)
	if itemID == "" {
		return nil
	}
	for idx := range d.AlchemyInheritMediums {
		row := &d.AlchemyInheritMediums[idx]
		if row.Disabled || !row.Active {
			continue
		}
		if strings.TrimSpace(row.ItemID) == itemID {
			return row
		}
	}
	return nil
}

func (d Data) itemByID(id string) *Item {
	for idx := range d.Items {
		if d.Items[idx].ID == id {
			return &d.Items[idx]
		}
	}
	return nil
}

func (d Data) ItemByID(id string) *Item {
	return d.itemByID(id)
}

func (i Item) HasType(target string) bool {
	for _, entry := range i.Type {
		if entry == target {
			return true
		}
	}
	return false
}

func (d Data) EquipmentTemplateByID(id string) *EquipmentTemplate {
	for idx := range d.EquipmentTemplates {
		if d.EquipmentTemplates[idx].ID == id {
			return &d.EquipmentTemplates[idx]
		}
	}
	return nil
}

func (d Data) AffixDefinitionByID(id string) *AffixDefinition {
	for idx := range d.AffixDefinitions {
		if d.AffixDefinitions[idx].ID == id {
			return &d.AffixDefinitions[idx]
		}
	}
	return nil
}

func (d Data) AffixDetachRuleByToolID(toolItemID string) *AffixDetachRule {
	for idx := range d.AffixDetachRules {
		if d.AffixDetachRules[idx].ToolItemID == toolItemID {
			return &d.AffixDetachRules[idx]
		}
	}
	return nil
}

func (d Data) AffixEnchantRuleByToolID(toolItemID string) *AffixEnchantRule {
	for idx := range d.AffixEnchantRules {
		if d.AffixEnchantRules[idx].ToolItemID == toolItemID {
			return &d.AffixEnchantRules[idx]
		}
	}
	return nil
}

func (d Data) AffixCarrierRuleByItemID(carrierItemID string) *AffixCarrierRule {
	for idx := range d.AffixCarrierRules {
		if d.AffixCarrierRules[idx].CarrierItemID == carrierItemID {
			return &d.AffixCarrierRules[idx]
		}
	}
	return nil
}

func (d Data) AlchemyGroupByID(id string) *AlchemyGroup {
	id = strings.TrimSpace(id)
	for idx := range d.AlchemyGroups {
		if !d.AlchemyGroups[idx].Disabled && strings.TrimSpace(d.AlchemyGroups[idx].ID) == id {
			return &d.AlchemyGroups[idx]
		}
	}
	return nil
}

func (d Data) AlchemyGroupByItemID(itemID string) *AlchemyGroup {
	itemID = strings.TrimSpace(itemID)
	for idx := range d.AlchemyGroups {
		group := &d.AlchemyGroups[idx]
		if group.Disabled {
			continue
		}
		for _, entry := range group.Entries {
			if strings.TrimSpace(entry.ItemID) == itemID {
				return group
			}
		}
	}
	return nil
}

func (d Data) ItemTraitByItemID(itemID string) *ItemTraitDefinition {
	itemID = strings.TrimSpace(itemID)
	for idx := range d.ItemTraits {
		if strings.TrimSpace(d.ItemTraits[idx].ItemID) == itemID {
			return &d.ItemTraits[idx]
		}
	}
	return nil
}

// ItemAlchemyNameAffix returns the prefix/suffix i18n keys an item contributes
// to alchemy custom-affix naming, or nil if the item has no alchemy metadata.
// Source of truth: items.json `alchemy.nameAffix`.
func (d Data) ItemAlchemyNameAffix(itemID string) *AlchemyNameAffix {
	item := d.ItemByID(itemID)
	if item == nil || item.Alchemy == nil {
		return nil
	}
	if strings.TrimSpace(item.Alchemy.NameAffix.PrefixKey) == "" && strings.TrimSpace(item.Alchemy.NameAffix.SuffixKey) == "" {
		return nil
	}
	return &item.Alchemy.NameAffix
}

func (d Data) AlchemyComboRecipeByID(id string) *AlchemyComboRecipe {
	id = strings.TrimSpace(id)
	for idx := range d.AlchemyComboRecipes {
		if !d.AlchemyComboRecipes[idx].Disabled && strings.TrimSpace(d.AlchemyComboRecipes[idx].ID) == id {
			return &d.AlchemyComboRecipes[idx]
		}
	}
	return nil
}

func (d Data) DefaultAlchemyTransmuteRule() *AlchemyTransmuteRule {
	var selected *AlchemyTransmuteRule
	for idx := range d.AlchemyTransmuteRules {
		rule := &d.AlchemyTransmuteRules[idx]
		if rule.Disabled || !rule.Default {
			continue
		}
		if selected == nil || rule.Priority > selected.Priority || (rule.Priority == selected.Priority && rule.ID < selected.ID) {
			selected = rule
		}
	}
	if selected != nil {
		return selected
	}
	// Legacy catalogs predate explicit default/itemIds selectors. Preserve their
	// first-active-rule behavior when loading old generated data or saves.
	for idx := range d.AlchemyTransmuteRules {
		rule := &d.AlchemyTransmuteRules[idx]
		if !rule.Disabled && len(rule.ItemIDs) == 0 {
			return rule
		}
	}
	for idx := range d.AlchemyTransmuteRules {
		if !d.AlchemyTransmuteRules[idx].Disabled {
			return &d.AlchemyTransmuteRules[idx]
		}
	}
	return nil
}

func (d Data) AlchemyTransmuteRuleForItemID(itemID string) *AlchemyTransmuteRule {
	itemID = strings.TrimSpace(itemID)
	var selected *AlchemyTransmuteRule
	for idx := range d.AlchemyTransmuteRules {
		rule := &d.AlchemyTransmuteRules[idx]
		if rule.Disabled || rule.Default {
			continue
		}
		matched := false
		for _, candidate := range rule.ItemIDs {
			if strings.TrimSpace(candidate) == itemID {
				matched = true
				break
			}
		}
		if matched && (selected == nil || rule.Priority > selected.Priority || (rule.Priority == selected.Priority && rule.ID < selected.ID)) {
			selected = rule
		}
	}
	if selected != nil {
		return selected
	}
	return d.DefaultAlchemyTransmuteRule()
}

func (d Data) DefaultAffixSmeltRule() *AffixSmeltRule {
	for idx := range d.AffixSmeltRules {
		if d.AffixSmeltRules[idx].Active && d.AffixSmeltRules[idx].ID == "default" {
			return &d.AffixSmeltRules[idx]
		}
	}
	return nil
}

func (d Data) CollectibleByItemID(itemID string) *Collectible {
	for idx := range d.Collectibles {
		if !d.Collectibles[idx].Disabled && d.Collectibles[idx].ItemID == itemID {
			return &d.Collectibles[idx]
		}
	}
	return nil
}

func (d Data) CollectibleByID(id string) *Collectible {
	for idx := range d.Collectibles {
		if !d.Collectibles[idx].Disabled && d.Collectibles[idx].ID == id {
			return &d.Collectibles[idx]
		}
	}
	return nil
}

func (d Data) BuildingDefinitionByID(id string) *BuildingDefinition {
	normalized := strings.TrimSpace(id)
	if normalized == "" {
		return nil
	}
	for idx := range d.Buildings {
		if strings.TrimSpace(d.Buildings[idx].ID) == normalized {
			return &d.Buildings[idx]
		}
	}
	return nil
}

func (d Data) ActionDefinitions() []ActionDefinition {
	return normalizeActionDefinitions(d.Actions)
}

func (d Data) FishingWaterByID(id string) *FishingWater {
	normalized := strings.TrimSpace(id)
	for index := range d.FishingWaters {
		if strings.TrimSpace(d.FishingWaters[index].ID) == normalized {
			return &d.FishingWaters[index]
		}
	}
	return nil
}

func (d Data) FishingSpotByID(id string) *FishingSpot {
	normalized := strings.TrimSpace(id)
	for index := range d.FishingSpots {
		if strings.TrimSpace(d.FishingSpots[index].ID) == normalized {
			return &d.FishingSpots[index]
		}
	}
	return nil
}

func (d Data) FishingSpeciesByID(id string) *FishingSpecies {
	normalized := strings.TrimSpace(id)
	for index := range d.FishingSpecies {
		if strings.TrimSpace(d.FishingSpecies[index].ID) == normalized {
			return &d.FishingSpecies[index]
		}
	}
	return nil
}

func (d Data) FishFoodRecipeByActionID(actionID string) *FishFoodRecipe {
	normalized := strings.TrimSpace(actionID)
	for index := range d.FishFoodRecipes {
		if strings.TrimSpace(d.FishFoodRecipes[index].ActionID) == normalized {
			return &d.FishFoodRecipes[index]
		}
	}
	return nil
}

func (d Data) FishFoodRecipeByOutputItemID(itemID string) *FishFoodRecipe {
	normalized := strings.TrimSpace(itemID)
	for index := range d.FishFoodRecipes {
		if strings.TrimSpace(d.FishFoodRecipes[index].OutputItemID) == normalized {
			return &d.FishFoodRecipes[index]
		}
	}
	return nil
}

func (d Data) FishingPondSpeciesByID(id string) *FishingPondSpecies {
	normalized := strings.TrimSpace(id)
	for index := range d.FishingPondSpecies {
		if d.FishingPondSpecies[index].SpeciesID == normalized {
			return &d.FishingPondSpecies[index]
		}
	}
	return nil
}

func (d Data) FishingTackleByItemID(itemID string) *FishingTackleDefinition {
	normalized := strings.TrimSpace(itemID)
	for index := range d.FishingTackle {
		if strings.TrimSpace(d.FishingTackle[index].ItemID) == normalized {
			return &d.FishingTackle[index]
		}
	}
	return nil
}

func (d Data) FishingInfusionProfileBySpeciesID(speciesID string) *FishingInfusionProfile {
	normalized := strings.TrimSpace(speciesID)
	for index := range d.FishingInfusionProfiles {
		if strings.TrimSpace(d.FishingInfusionProfiles[index].SpeciesID) == normalized {
			return &d.FishingInfusionProfiles[index]
		}
	}
	return nil
}

func normalizeActionDefinitions(rows []ActionDefinition) []ActionDefinition {
	out := make([]ActionDefinition, 0, len(rows))
	for _, row := range rows {
		if row.ID == "" {
			continue
		}
		if row.Disabled {
			continue
		}
		mainType := row.MainType
		if mainType == "" {
			mainType = row.Menu
		}
		menu := row.Menu
		if menu == "" {
			menu = mainType
		}
		iconID := row.IconID
		if iconID == "" {
			iconID = "action." + row.ID
		}
		nameKey := row.NameKey
		if nameKey == "" {
			nameKey = "action." + row.ID + ".name"
		}
		descKey := row.DescKey
		if descKey == "" {
			descKey = "action." + row.ID + ".desc"
		}
		row.MainType = mainType
		subType := row.SubType
		if subType == "" {
			subType = "common"
		}
		row.SubType = subType
		row.Menu = menu
		row.IconID = iconID
		row.NameKey = nameKey
		row.DescKey = descKey
		if row.EconomyIndustryTier <= 0 {
			row.EconomyIndustryTier = 1
		}
		if row.EconomyDifficultyMultiplier <= 0 {
			row.EconomyDifficultyMultiplier = 1
		}
		out = append(out, row)
	}
	return out
}

// TitleCategoryEntry is a typed view for a title_category row.
type TitleCategoryEntry struct {
	ID       string
	NameKey  string
	Order    int
	Disabled bool
}

// TitlePrefixEntry is a typed view for a title_prefix row.
type TitlePrefixEntry struct {
	ID       string
	NameKey  string
	Rarity   string
	Category string
	Order    int
	Disabled bool
}

// TitleSuffixEntry is a typed view for a title_suffix row.
type TitleSuffixEntry struct {
	ID       string
	NameKey  string
	Rarity   string
	Category string
	Order    int
	Disabled bool
}

func titleEntryFromMap(m map[string]any) (id, nameKey, rarity, category string, order int, disabled bool) {
	if v, ok := m["id"].(string); ok {
		id = v
	}
	if v, ok := m["nameKey"].(string); ok {
		nameKey = v
	}
	if v, ok := m["rarity"].(string); ok {
		rarity = v
	}
	if v, ok := m["category"].(string); ok {
		category = v
	}
	if v, ok := m["order"].(float64); ok {
		order = int(v)
	}
	if v, ok := m["disabled"].(bool); ok {
		disabled = v
	}
	return
}

func titleCategoryFromMap(m map[string]any) (id, nameKey string, order int, disabled bool) {
	if v, ok := m["id"].(string); ok {
		id = v
	}
	if v, ok := m["nameKey"].(string); ok {
		nameKey = v
	}
	if v, ok := m["order"].(float64); ok {
		order = int(v)
	}
	if v, ok := m["disabled"].(bool); ok {
		disabled = v
	}
	return
}

func (d Data) TitleCategoryByID(id string) *TitleCategoryEntry {
	trimmed := strings.TrimSpace(id)
	if trimmed == "" {
		return nil
	}
	for _, row := range d.TitleCategories {
		rowID, nameKey, order, disabled := titleCategoryFromMap(row)
		if rowID == trimmed {
			return &TitleCategoryEntry{ID: rowID, NameKey: nameKey, Order: order, Disabled: disabled}
		}
	}
	return nil
}

// CollectibleCategoryEntry is a typed view for a collectible_category row.
type CollectibleCategoryEntry struct {
	ID       string
	NameKey  string
	Order    int
	Disabled bool
}

func collectibleCategoryFromMap(m map[string]any) (id, nameKey string, order int, disabled bool) {
	if v, ok := m["id"].(string); ok {
		id = v
	}
	if v, ok := m["nameKey"].(string); ok {
		nameKey = v
	}
	if v, ok := m["order"].(float64); ok {
		order = int(v)
	}
	if v, ok := m["disabled"].(bool); ok {
		disabled = v
	}
	return
}

func (d Data) CollectibleCategoryByID(id string) *CollectibleCategoryEntry {
	trimmed := strings.TrimSpace(id)
	if trimmed == "" {
		return nil
	}
	for _, row := range d.CollectibleCategories {
		rowID, nameKey, order, disabled := collectibleCategoryFromMap(row)
		if rowID == trimmed {
			return &CollectibleCategoryEntry{ID: rowID, NameKey: nameKey, Order: order, Disabled: disabled}
		}
	}
	return nil
}

func (d Data) TitlePrefixByID(id string) *TitlePrefixEntry {
	trimmed := strings.TrimSpace(id)
	if trimmed == "" {
		return nil
	}
	for _, row := range d.TitlePrefixes {
		rowID, nameKey, rarity, category, order, disabled := titleEntryFromMap(row)
		if rowID == trimmed {
			return &TitlePrefixEntry{ID: rowID, NameKey: nameKey, Rarity: rarity, Category: category, Order: order, Disabled: disabled}
		}
	}
	return nil
}

func (d Data) TitleSuffixByID(id string) *TitleSuffixEntry {
	trimmed := strings.TrimSpace(id)
	if trimmed == "" {
		return nil
	}
	for _, row := range d.TitleSuffixes {
		rowID, nameKey, rarity, category, order, disabled := titleEntryFromMap(row)
		if rowID == trimmed {
			return &TitleSuffixEntry{ID: rowID, NameKey: nameKey, Rarity: rarity, Category: category, Order: order, Disabled: disabled}
		}
	}
	return nil
}

func clampCatalogChance(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}
