package config

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strings"
)

type SharedGlobalSettings struct {
	Market                        SharedMarketSettings               `json:"market"`
	Auction                       SharedAuctionSettings              `json:"auction"`
	ItemRename                    SharedItemRenameSettings           `json:"itemRename"`
	PlayerUsernameRename          SharedPlayerUsernameRenameSettings `json:"playerUsernameRename"`
	Kitty                         SharedKittySettings                `json:"kitty"`
	Presence                      SharedPresenceSettings             `json:"presence"`
	Action                        SharedActionSettings               `json:"action"`
	CatVillageCommission          SharedCatVillageCommissionConfig   `json:"catVillageCommission"`
	BattleItemBagMaxKinds         int64                              `json:"battleItemBagMaxKinds"`
	BattleItemBagMaxTotalQuantity int64                              `json:"battleItemBagMaxTotalQuantity"`
	IntervalBetweenItemAndAction  int64                              `json:"intervalBetweenItemAndAction"`
	CombatDifficulty              CombatDifficultySettings           `json:"combatDifficulty"`
	CombatBalanceModel            CombatBalanceModelSettings         `json:"combatBalanceModel"`
	CombatScoring                 CombatScoringSettings              `json:"combatScoring"`
	FogMode                       FogModeSettings                    `json:"fogMode"`
	Alchemy                       SharedAlchemySettings              `json:"alchemy"`
}

// CombatScoringSettings holds the enemy attribute template data used for fog-mode dynamic stat generation.
// Only the fields needed by the server are included; the full scoring system lives in the frontend editor.
type CombatScoringSettings struct {
	AttributeUnitValues     map[string]float64      `json:"attributeUnitValues"`
	EnemyAttributeTemplates EnemyAttributeTemplates `json:"enemyAttributeTemplates"`
}

type EnemyAttributeTemplates struct {
	Tiers               []EnemyTierTemplate    `json:"tiers"`
	Roles               []EnemyRoleTemplate    `json:"roles"`
	Species             []EnemySpeciesTemplate `json:"species"`
	DerivedStatMinimums map[string]float64     `json:"derivedStatMinimums"`
}

type EnemyTierTemplate struct {
	Key              string              `json:"key"`
	BudgetMultiplier float64             `json:"budgetMultiplier"`
	PressurePct      float64             `json:"pressurePct"`
	HpMultiplier     float64             `json:"hpMultiplier"`
	TargetRounds     EnemyTierRoundRange `json:"targetRounds"`
}

type EnemyTierRoundRange struct {
	Min float64 `json:"min"`
	Max float64 `json:"max"`
}

type EnemyRoleTemplate struct {
	Key         string             `json:"key"`
	Weights     map[string]float64 `json:"weights"`
	BaseAttrs   map[string]float64 `json:"baseAttrs"`
	Multipliers map[string]float64 `json:"multipliers"`
}

type EnemySpeciesTemplate struct {
	Key         string             `json:"key"`
	Weights     map[string]float64 `json:"weights"`
	BaseAttrs   map[string]float64 `json:"baseAttrs"`
	Multipliers map[string]float64 `json:"multipliers"`
}

// FogModeSettings configures fog-mode dungeon rooms.
type FogModeSettings struct {
	// PlayerEncounterChance is the per-wave probability of replacing enemies with a player phantom team.
	PlayerEncounterChance float64 `json:"playerEncounterChance"`
	// SpecialWaveEvery overrides the dungeon's own specialWaveEvery in fog mode.
	SpecialWaveEvery int `json:"specialWaveEvery"`
	// FogPlayerDrops is the loot table applied when a player phantom team is defeated.
	FogPlayerDrops []FogPlayerDropRule `json:"fogPlayerDrops"`
}

// FogPlayerDropRule is one entry in the fog-player loot table.
type FogPlayerDropRule struct {
	ItemID  string  `json:"itemId"`
	Min     int64   `json:"min"`
	Max     int64   `json:"max"`
	Percent float64 `json:"percent"`
}

type SharedCatVillageCommissionConfig struct {
	RefreshCost map[string]int64 `json:"refreshCost"`
}

type SharedMarketSettings struct {
	PriceSteps []SharedMarketPriceStep `json:"priceSteps"`
}

type SharedMarketPriceStep struct {
	Min  int64  `json:"min"`
	Max  *int64 `json:"max"`
	Step int64  `json:"step"`
}

type SharedKittySettings struct {
	ActionDurationMultiplier    float64  `json:"actionDurationMultiplier"`
	StatusExpMultiplier         float64  `json:"statusExpMultiplier"`
	BattleAttrExpMultiplier     float64  `json:"battleAttrExpMultiplier"`
	BattleSkillExpMultiplier    float64  `json:"battleSkillExpMultiplier"`
	BattleLevelCapDefault       int64    `json:"battleLevelCapDefault"`
	BattleLevelCapPerKittyLevel int64    `json:"battleLevelCapPerKittyLevel"`
	BattleEquipSlots            []string `json:"battleEquipSlots"`
	RemovePersonalityItemID     string   `json:"removePersonalityItemId"`
	RemovePersonalityCount      int64    `json:"removePersonalityCount"`
	DefaultPersonalityID        string   `json:"defaultPersonalityId"`
	MaxPersonalities            int      `json:"maxPersonalities"`
}

type SharedKittyPersonalityConfig struct {
	ID      string                         `json:"id"`
	NameKey string                         `json:"nameKey"`
	DescKey string                         `json:"descKey"`
	Weight  float64                        `json:"weight"`
	Effects []SharedKittyPersonalityEffect `json:"effects"`
}

type SharedKittyPersonalityEffect struct {
	Kind            string             `json:"kind"`
	Attrs           map[string]float64 `json:"attrs,omitempty"`
	Position        string             `json:"position,omitempty"`
	BattleHooks     []map[string]any   `json:"battleHooks,omitempty"`
	EffectMutations []map[string]any   `json:"effectMutations,omitempty"`
}

type SharedAuctionSettings struct {
	FeeRateBps              int64 `json:"feeRateBps"`
	MinBidIncrement         int64 `json:"minBidIncrement"`
	AntiSnipeTriggerSeconds int64 `json:"antiSnipeTriggerSeconds"`
	AntiSnipeExtendSeconds  int64 `json:"antiSnipeExtendSeconds"`
	MinRepLevel             int64 `json:"minRepLevel"`
}

type SharedItemRenameSettings struct {
	Cost         map[string]int64 `json:"cost"`
	MaxNameRunes int              `json:"maxNameRunes"`
	LabelColors  []string         `json:"labelColors"`
}

type SharedPlayerUsernameRenameSettings struct {
	CostItemID    string `json:"costItemId"`
	CostItemCount int64  `json:"costItemCount"`
}

func ValidatePlayerUsernameRenameItemID(settings SharedPlayerUsernameRenameSettings, itemIDs map[string]bool) error {
	if len(itemIDs) == 0 {
		return nil
	}
	itemID := strings.TrimSpace(settings.CostItemID)
	if !itemIDs[itemID] {
		return fmt.Errorf("playerUsernameRename.costItemId not found: %s", itemID)
	}
	return nil
}

type SharedPresenceSettings struct {
	VisibilityOfflineDelaySeconds int64 `json:"visibilityOfflineDelaySeconds"`
}

type SharedActionSettings struct {
	EnhanceInheritanceRatio float64 `json:"enhanceInheritanceRatio"`
	QueueLaneLimit          int64   `json:"queueLaneLimit"`
	KittyQueueLaneLimit     int64   `json:"kittyQueueLaneLimit"`
}

type SharedAlchemySettings struct {
	InfusionBaseGoldCost           int64                                `json:"infusionBaseGoldCost"`
	InfusionBaseDurationSec        int                                  `json:"infusionBaseDurationSec"`
	InfusionBaseExp                float64                              `json:"infusionBaseExp"`
	DurationsSec                   SharedAlchemyDurationsSec            `json:"durationsSec"`
	TransmuteDurationByRarityIndex []int                                `json:"transmuteDurationByRarityIndex"`
	LevelDeficitFailChancePerLevel float64                              `json:"levelDeficitFailChancePerLevel"`
	LevelDeficitFailChanceCap      float64                              `json:"levelDeficitFailChanceCap"`
	Inherit                        SharedAlchemyInheritSettings         `json:"inherit"`
	ConvertGroupEconomyByRarity    map[string]SharedAlchemyGroupEconomy `json:"convertGroupEconomyByRarity"`
}

type SharedAlchemyInheritSettings struct {
	BaseInheritSlots             int                                 `json:"baseInheritSlots"`
	BaseSuppressSlots            int                                 `json:"baseSuppressSlots"`
	InheritSlotPrecisionScale    float64                             `json:"inheritSlotPrecisionScalePerPoint"`
	SuppressSlotStabilityBonus   float64                             `json:"suppressSlotStabilityBonusPerPoint"`
	PositiveIgnoreKeepChanceBase float64                             `json:"positiveIgnoreKeepChanceBase"`
	NegativeIgnoreKeepChanceBase float64                             `json:"negativeIgnoreKeepChanceBase"`
	HybridMergeDivisor           float64                             `json:"hybridMergeDivisor"`
	InbreedMutationChance        float64                             `json:"inbreedMutationChance"`
	InbreedGoodChance            float64                             `json:"inbreedGoodChance"`
	InbreedGoodMultiplierRange   SharedAlchemyInheritMultiplierRange `json:"inbreedGoodMultiplierRange"`
	InbreedBadMultiplierRange    SharedAlchemyInheritMultiplierRange `json:"inbreedBadMultiplierRange"`
	MutationChance               float64                             `json:"mutationChance"`
	NumericValueCapMultiplier    float64                             `json:"numericValueCapMultiplier"`
}

type SharedAlchemyInheritMultiplierRange struct {
	Min float64 `json:"min"`
	Max float64 `json:"max"`
}

type SharedAlchemyDurationsSec struct {
	Default   int `json:"default"`
	Convert   int `json:"convert"`
	Bench     int `json:"bench"`
	Transmute int `json:"transmute"`
	Smelt     int `json:"smelt"`
}

type SharedAlchemyGroupEconomy struct {
	Gold     int64   `json:"gold"`
	Duration float64 `json:"duration"`
	Exp      float64 `json:"exp"`
	Upgrade  float64 `json:"upgrade"`
}

type CombatDifficultySettings struct {
	Levels []CombatDifficultyLevel `json:"levels"`
}

type CombatDifficultyLevel struct {
	Level               int            `json:"level"`
	LabelKey            string         `json:"labelKey"`
	StatMultiplierCurve map[string]any `json:"statMultiplierCurve"`
	BattleExpBonus      float64        `json:"battleExpBonus"`
	BattleDropRateBonus float64        `json:"battleDropRateBonus"`
}

type CombatBalanceModelSettings struct {
	Version                 int                      `json:"version"`
	MaxBattleLevel          int                      `json:"maxBattleLevel"`
	PrimaryPointCap         int                      `json:"primaryPointCap"`
	PrimaryPointAttrs       []string                 `json:"primaryPointAttrs"`
	DefenseK                float64                  `json:"defenseK"`
	EnemyHpRegenRatio       float64                  `json:"enemyHpRegenRatio"`
	PercentPenetrationCap   float64                  `json:"percentPenetrationCap"`
	ExpPerPower             float64                  `json:"expPerPower"`
	TeamExtraEnemyScale     float64                  `json:"teamExtraEnemyScale"`
	TeamRewardEfficiencyCap float64                  `json:"teamRewardEfficiencyCap"`
	ComboBudgetMinShare     float64                  `json:"comboBudgetMinShare"`
	ComboBudgetMaxShare     float64                  `json:"comboBudgetMaxShare"`
	LevelBands              []CombatBalanceLevelBand `json:"levelBands"`
}

type CombatBalanceLevelBand struct {
	Level       int     `json:"level"`
	PlayerDPR   float64 `json:"playerDpr"`
	PlayerHP    float64 `json:"playerHp"`
	PlayerMP    float64 `json:"playerMp"`
	PlayerSpeed float64 `json:"playerSpeed"`
	ExpRate     float64 `json:"expRate"`
}

func DefaultDifficultyLevels() []CombatDifficultyLevel {
	// 属性倍率: 1 位小数(L1=1.15 例外), 平滑凸递增 1.15 → 10.0
	// 奖励: 1 位小数, 超 1 位则以 5 结尾 (0.15, 0.45, 1.05 等), 经验/掉落同曲线
	return []CombatDifficultyLevel{
		{Level: 1, LabelKey: "game.difficulty.level1", StatMultiplierCurve: map[string]any{"base": 1.15, "step": 0.0, "overrides": map[string]any{}}, BattleExpBonus: 0.10, BattleDropRateBonus: 0.10},
		{Level: 2, LabelKey: "game.difficulty.level2", StatMultiplierCurve: map[string]any{"base": 1.40, "step": 0.0, "overrides": map[string]any{}}, BattleExpBonus: 0.15, BattleDropRateBonus: 0.15},
		{Level: 3, LabelKey: "game.difficulty.level3", StatMultiplierCurve: map[string]any{"base": 1.80, "step": 0.0, "overrides": map[string]any{}}, BattleExpBonus: 0.20, BattleDropRateBonus: 0.20},
		{Level: 4, LabelKey: "game.difficulty.level4", StatMultiplierCurve: map[string]any{"base": 2.40, "step": 0.0, "overrides": map[string]any{}}, BattleExpBonus: 0.30, BattleDropRateBonus: 0.30},
		{Level: 5, LabelKey: "game.difficulty.level5", StatMultiplierCurve: map[string]any{"base": 3.20, "step": 0.0, "overrides": map[string]any{}}, BattleExpBonus: 0.45, BattleDropRateBonus: 0.45},
		{Level: 6, LabelKey: "game.difficulty.level6", StatMultiplierCurve: map[string]any{"base": 4.20, "step": 0.0, "overrides": map[string]any{}}, BattleExpBonus: 0.60, BattleDropRateBonus: 0.60},
		{Level: 7, LabelKey: "game.difficulty.level7", StatMultiplierCurve: map[string]any{"base": 5.40, "step": 0.0, "overrides": map[string]any{}}, BattleExpBonus: 0.80, BattleDropRateBonus: 0.80},
		{Level: 8, LabelKey: "game.difficulty.level8", StatMultiplierCurve: map[string]any{"base": 6.70, "step": 0.0, "overrides": map[string]any{}}, BattleExpBonus: 1.05, BattleDropRateBonus: 1.05},
		{Level: 9, LabelKey: "game.difficulty.level9", StatMultiplierCurve: map[string]any{"base": 8.30, "step": 0.0, "overrides": map[string]any{}}, BattleExpBonus: 1.30, BattleDropRateBonus: 1.30},
		{Level: 10, LabelKey: "game.difficulty.level10", StatMultiplierCurve: map[string]any{"base": 10.00, "step": 0.0, "overrides": map[string]any{}}, BattleExpBonus: 1.60, BattleDropRateBonus: 1.60},
	}
}

func DefaultSharedGlobalSettings() SharedGlobalSettings {
	return SharedGlobalSettings{
		Market: SharedMarketSettings{
			PriceSteps: []SharedMarketPriceStep{
				{Min: 0, Max: int64Ptr(100), Step: 1},
				{Min: 100, Max: int64Ptr(1000), Step: 10},
				{Min: 1000, Max: int64Ptr(10000), Step: 50},
				{Min: 10000, Max: int64Ptr(100000), Step: 100},
				{Min: 100000, Max: int64Ptr(1000000), Step: 1000},
				{Min: 1000000, Max: int64Ptr(10000000), Step: 10000},
				{Min: 10000000, Max: nil, Step: 100000},
			},
		},
		Kitty: SharedKittySettings{
			ActionDurationMultiplier:    1.5,
			StatusExpMultiplier:         0,
			BattleAttrExpMultiplier:     1,
			BattleSkillExpMultiplier:    1,
			BattleLevelCapDefault:       100,
			BattleLevelCapPerKittyLevel: 25,
			BattleEquipSlots: []string{
				"cloak",
				"head",
				"left_hand",
				"body",
				"right_hand",
				"arm",
				"legs",
				"feet",
				"accessory_1",
				"accessory_2",
				"accessory_3",
				"accessory_4",
			},
			RemovePersonalityItemID: "gold_coin",
			RemovePersonalityCount:  1,
			DefaultPersonalityID:    "balanced",
			MaxPersonalities:        4,
		},
		Auction: SharedAuctionSettings{
			FeeRateBps:              500,
			MinBidIncrement:         1,
			AntiSnipeTriggerSeconds: 30 * 60,
			AntiSnipeExtendSeconds:  10 * 60,
			MinRepLevel:             10,
		},
		ItemRename: SharedItemRenameSettings{
			Cost: map[string]int64{
				"harmonic_embryo": 1,
			},
			MaxNameRunes: 16,
			LabelColors:  []string{"gold", "rose", "violet", "amber", "sky", "emerald", "fuchsia"},
		},
		PlayerUsernameRename: SharedPlayerUsernameRenameSettings{
			CostItemID:    "harmonic_embryo",
			CostItemCount: 1,
		},
		Presence: SharedPresenceSettings{
			VisibilityOfflineDelaySeconds: 120,
		},
		Action: SharedActionSettings{
			EnhanceInheritanceRatio: 0.5,
			QueueLaneLimit:          30,
			KittyQueueLaneLimit:     20,
		},
		BattleItemBagMaxKinds:         4,
		BattleItemBagMaxTotalQuantity: 1000,
		IntervalBetweenItemAndAction:  100,
		CombatDifficulty: CombatDifficultySettings{
			Levels: DefaultDifficultyLevels(),
		},
		CombatBalanceModel: DefaultCombatBalanceModelSettings(),
		FogMode: FogModeSettings{
			PlayerEncounterChance: 0.20,
			SpecialWaveEvery:      10,
			FogPlayerDrops:        []FogPlayerDropRule{},
		},
		Alchemy: defaultSharedAlchemySettings(),
	}
}

func defaultSharedAlchemySettings() SharedAlchemySettings {
	return SharedAlchemySettings{
		InfusionBaseGoldCost:    32,
		InfusionBaseDurationSec: 16,
		InfusionBaseExp:         2,
		DurationsSec: SharedAlchemyDurationsSec{
			Default:   16,
			Convert:   14,
			Bench:     16,
			Transmute: 8,
			Smelt:     24,
		},
		TransmuteDurationByRarityIndex: []int{8, 9, 10, 11, 12, 12},
		LevelDeficitFailChancePerLevel: 0.06,
		LevelDeficitFailChanceCap:      0.85,
		Inherit: SharedAlchemyInheritSettings{
			BaseInheritSlots:             3,
			BaseSuppressSlots:            0,
			InheritSlotPrecisionScale:    1,
			SuppressSlotStabilityBonus:   1,
			PositiveIgnoreKeepChanceBase: 0.4,
			NegativeIgnoreKeepChanceBase: 1,
			HybridMergeDivisor:           1.9,
			InbreedMutationChance:        0.8,
			InbreedGoodChance:            0.05,
			InbreedGoodMultiplierRange:   SharedAlchemyInheritMultiplierRange{Min: 1.1, Max: 1.35},
			InbreedBadMultiplierRange:    SharedAlchemyInheritMultiplierRange{Min: 0.35, Max: 0.8},
			MutationChance:               0.05,
			NumericValueCapMultiplier:    3,
		},
		ConvertGroupEconomyByRarity: map[string]SharedAlchemyGroupEconomy{
			"common":    {Gold: 45, Duration: 14, Exp: 2.0, Upgrade: 0.020},
			"uncommon":  {Gold: 105, Duration: 27, Exp: 3.5, Upgrade: 0.018},
			"rare":      {Gold: 245, Duration: 39, Exp: 5.0, Upgrade: 0.016},
			"epic":      {Gold: 550, Duration: 52, Exp: 6.5, Upgrade: 0.014},
			"legendary": {Gold: 1200, Duration: 64, Exp: 8.0, Upgrade: 0.000},
		},
	}
}

func DefaultCombatBalanceModelSettings() CombatBalanceModelSettings {
	return CombatBalanceModelSettings{
		Version:                 1,
		MaxBattleLevel:          500,
		PrimaryPointCap:         500,
		PrimaryPointAttrs:       []string{"strength", "dexterity", "intelligence", "perception", "stamina"},
		DefenseK:                100,
		EnemyHpRegenRatio:       0.34,
		PercentPenetrationCap:   0.8,
		ExpPerPower:             0.018,
		TeamExtraEnemyScale:     0.45,
		TeamRewardEfficiencyCap: 1.15,
		ComboBudgetMinShare:     0.08,
		ComboBudgetMaxShare:     0.18,
		LevelBands: []CombatBalanceLevelBand{
			{Level: 1, PlayerDPR: 12, PlayerHP: 115, PlayerMP: 28, PlayerSpeed: 7, ExpRate: 1},
			{Level: 50, PlayerDPR: 140, PlayerHP: 1500, PlayerMP: 110, PlayerSpeed: 28, ExpRate: 1.2},
			{Level: 100, PlayerDPR: 330, PlayerHP: 3500, PlayerMP: 190, PlayerSpeed: 48, ExpRate: 1.55},
			{Level: 200, PlayerDPR: 950, PlayerHP: 9000, PlayerMP: 360, PlayerSpeed: 85, ExpRate: 2.2},
			{Level: 300, PlayerDPR: 1900, PlayerHP: 16000, PlayerMP: 540, PlayerSpeed: 120, ExpRate: 2.85},
			{Level: 400, PlayerDPR: 3400, PlayerHP: 26000, PlayerMP: 760, PlayerSpeed: 155, ExpRate: 3.5},
			{Level: 500, PlayerDPR: 5600, PlayerHP: 40000, PlayerMP: 1000, PlayerSpeed: 190, ExpRate: 4.2},
		},
	}
}

func LoadSharedGlobalSettings(path string) (SharedGlobalSettings, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return SharedGlobalSettings{}, fmt.Errorf("read shared global config %q: %w", path, err)
	}
	settings := DefaultSharedGlobalSettings()
	if err = json.Unmarshal(raw, &settings); err != nil {
		return SharedGlobalSettings{}, fmt.Errorf("parse shared global config %q: %w", path, err)
	}
	if len(settings.Market.PriceSteps) == 0 {
		return SharedGlobalSettings{}, fmt.Errorf("invalid market.priceSteps in %q: empty", path)
	}
	for i := range settings.Market.PriceSteps {
		step := &settings.Market.PriceSteps[i]
		if step.Min < 0 {
			return SharedGlobalSettings{}, fmt.Errorf("invalid market.priceSteps[%d].min in %q", i, path)
		}
		if step.Step <= 0 {
			return SharedGlobalSettings{}, fmt.Errorf("invalid market.priceSteps[%d].step in %q", i, path)
		}
		if step.Max != nil && *step.Max <= step.Min {
			return SharedGlobalSettings{}, fmt.Errorf("invalid market.priceSteps[%d].max in %q", i, path)
		}
		if i > 0 {
			prev := settings.Market.PriceSteps[i-1]
			if prev.Max == nil {
				return SharedGlobalSettings{}, fmt.Errorf("invalid market.priceSteps order in %q: open-ended segment must be last", path)
			}
			if prev.Min > step.Min {
				return SharedGlobalSettings{}, fmt.Errorf("invalid market.priceSteps order in %q: min not ascending", path)
			}
		}
	}
	for i := 1; i < len(settings.Market.PriceSteps); i++ {
		prev := settings.Market.PriceSteps[i-1]
		curr := settings.Market.PriceSteps[i]
		if prev.Max == nil {
			return SharedGlobalSettings{}, fmt.Errorf("invalid market.priceSteps in %q: open-ended segment must be last", path)
		}
		if *prev.Max > curr.Min {
			return SharedGlobalSettings{}, fmt.Errorf("invalid market.priceSteps overlap in %q: idx=%d", path, i)
		}
	}
	if settings.Kitty.ActionDurationMultiplier <= 0 {
		return SharedGlobalSettings{}, fmt.Errorf("invalid kitty.actionDurationMultiplier in %q", path)
	}
	if settings.Kitty.StatusExpMultiplier < 0 {
		return SharedGlobalSettings{}, fmt.Errorf("invalid kitty.statusExpMultiplier in %q", path)
	}
	if settings.Kitty.BattleAttrExpMultiplier < 0 || settings.Kitty.BattleAttrExpMultiplier > 1 {
		return SharedGlobalSettings{}, fmt.Errorf("invalid kitty.battleAttrExpMultiplier in %q", path)
	}
	if settings.Kitty.BattleSkillExpMultiplier < 0 || settings.Kitty.BattleSkillExpMultiplier > 1 {
		return SharedGlobalSettings{}, fmt.Errorf("invalid kitty.battleSkillExpMultiplier in %q", path)
	}
	if settings.Kitty.BattleLevelCapDefault <= 0 {
		return SharedGlobalSettings{}, fmt.Errorf("invalid kitty.battleLevelCapDefault in %q", path)
	}
	if settings.Kitty.BattleLevelCapPerKittyLevel < 0 {
		return SharedGlobalSettings{}, fmt.Errorf("invalid kitty.battleLevelCapPerKittyLevel in %q", path)
	}
	settings.Kitty.BattleEquipSlots = uniqueNormalizedStrings(settings.Kitty.BattleEquipSlots)
	if len(settings.Kitty.BattleEquipSlots) == 0 {
		return SharedGlobalSettings{}, fmt.Errorf("invalid kitty.battleEquipSlots in %q: empty", path)
	}
	allowedKittySlots := map[string]struct{}{
		"cloak":       {},
		"head":        {},
		"left_hand":   {},
		"body":        {},
		"right_hand":  {},
		"arm":         {},
		"legs":        {},
		"feet":        {},
		"accessory_1": {},
		"accessory_2": {},
		"accessory_3": {},
		"accessory_4": {},
	}
	for _, slot := range settings.Kitty.BattleEquipSlots {
		if _, ok := allowedKittySlots[slot]; !ok {
			return SharedGlobalSettings{}, fmt.Errorf("invalid kitty.battleEquipSlots in %q: %s", path, slot)
		}
	}
	settings.Kitty.RemovePersonalityItemID = strings.TrimSpace(settings.Kitty.RemovePersonalityItemID)
	if settings.Kitty.RemovePersonalityItemID == "" {
		return SharedGlobalSettings{}, fmt.Errorf("invalid kitty.removePersonalityItemId in %q", path)
	}
	if settings.Kitty.RemovePersonalityCount <= 0 {
		return SharedGlobalSettings{}, fmt.Errorf("invalid kitty.removePersonalityCount in %q", path)
	}
	if settings.Kitty.MaxPersonalities <= 0 {
		return SharedGlobalSettings{}, fmt.Errorf("invalid kitty.maxPersonalities in %q", path)
	}
	settings.Kitty.DefaultPersonalityID = normalizedID(settings.Kitty.DefaultPersonalityID)
	if settings.Kitty.DefaultPersonalityID == "" {
		return SharedGlobalSettings{}, fmt.Errorf("invalid kitty.defaultPersonalityId in %q", path)
	}
	if settings.Auction.FeeRateBps < 0 || settings.Auction.FeeRateBps > 10_000 {
		return SharedGlobalSettings{}, fmt.Errorf("invalid auction.feeRateBps in %q", path)
	}
	if settings.Auction.MinBidIncrement <= 0 {
		return SharedGlobalSettings{}, fmt.Errorf("invalid auction.minBidIncrement in %q", path)
	}
	if settings.Auction.AntiSnipeTriggerSeconds < 0 {
		return SharedGlobalSettings{}, fmt.Errorf("invalid auction.antiSnipeTriggerSeconds in %q", path)
	}
	if settings.Auction.AntiSnipeExtendSeconds < 0 {
		return SharedGlobalSettings{}, fmt.Errorf("invalid auction.antiSnipeExtendSeconds in %q", path)
	}
	if settings.Auction.MinRepLevel < 0 {
		return SharedGlobalSettings{}, fmt.Errorf("invalid auction.minRepLevel in %q", path)
	}
	if settings.ItemRename.MaxNameRunes <= 0 {
		return SharedGlobalSettings{}, fmt.Errorf("invalid itemRename.maxNameRunes in %q", path)
	}
	settings.ItemRename.LabelColors = normalizeItemRenameLabelColors(settings.ItemRename.LabelColors)
	if len(settings.ItemRename.LabelColors) == 0 {
		return SharedGlobalSettings{}, fmt.Errorf("invalid itemRename.labelColors in %q", path)
	}
	if len(settings.ItemRename.Cost) == 0 {
		return SharedGlobalSettings{}, fmt.Errorf("invalid itemRename.cost in %q: empty", path)
	}
	normalizedRenameCost := make(map[string]int64, len(settings.ItemRename.Cost))
	for rawItemID, count := range settings.ItemRename.Cost {
		itemID := strings.TrimSpace(rawItemID)
		if itemID == "" || count <= 0 {
			return SharedGlobalSettings{}, fmt.Errorf("invalid itemRename.cost in %q", path)
		}
		normalizedRenameCost[itemID] += count
	}
	settings.ItemRename.Cost = normalizedRenameCost
	settings.PlayerUsernameRename.CostItemID = strings.TrimSpace(settings.PlayerUsernameRename.CostItemID)
	if settings.PlayerUsernameRename.CostItemID == "" {
		return SharedGlobalSettings{}, fmt.Errorf("invalid playerUsernameRename.costItemId in %q", path)
	}
	if settings.PlayerUsernameRename.CostItemCount <= 0 {
		return SharedGlobalSettings{}, fmt.Errorf("invalid playerUsernameRename.costItemCount in %q", path)
	}
	if settings.Presence.VisibilityOfflineDelaySeconds < 0 {
		return SharedGlobalSettings{}, fmt.Errorf("invalid presence.visibilityOfflineDelaySeconds in %q", path)
	}
	if settings.Action.EnhanceInheritanceRatio < 0 || settings.Action.EnhanceInheritanceRatio > 1 {
		return SharedGlobalSettings{}, fmt.Errorf("invalid action.enhanceInheritanceRatio in %q", path)
	}
	if settings.Action.QueueLaneLimit <= 0 {
		return SharedGlobalSettings{}, fmt.Errorf("invalid action.queueLaneLimit in %q", path)
	}
	if settings.Action.KittyQueueLaneLimit <= 0 {
		return SharedGlobalSettings{}, fmt.Errorf("invalid action.kittyQueueLaneLimit in %q", path)
	}
	if settings.BattleItemBagMaxKinds <= 0 {
		return SharedGlobalSettings{}, fmt.Errorf("invalid battleItemBagMaxKinds in %q", path)
	}
	if settings.BattleItemBagMaxTotalQuantity <= 0 {
		return SharedGlobalSettings{}, fmt.Errorf("invalid battleItemBagMaxTotalQuantity in %q", path)
	}
	if settings.IntervalBetweenItemAndAction < 0 {
		return SharedGlobalSettings{}, fmt.Errorf("invalid intervalBetweenItemAndAction in %q", path)
	}
	if err := validateSharedAlchemySettings(path, settings.Alchemy); err != nil {
		return SharedGlobalSettings{}, err
	}
	if len(settings.CombatDifficulty.Levels) == 0 {
		return SharedGlobalSettings{}, fmt.Errorf("invalid combatDifficulty.levels in %q: empty", path)
	}
	seenLevel := make(map[int]struct{}, len(settings.CombatDifficulty.Levels))
	for i, lvl := range settings.CombatDifficulty.Levels {
		if lvl.Level < 1 {
			return SharedGlobalSettings{}, fmt.Errorf("invalid combatDifficulty.levels[%d].level in %q", i, path)
		}
		if _, exists := seenLevel[lvl.Level]; exists {
			return SharedGlobalSettings{}, fmt.Errorf("duplicate combatDifficulty.levels[%d].level in %q", i, path)
		}
		seenLevel[lvl.Level] = struct{}{}
		if lvl.BattleExpBonus < 0 {
			return SharedGlobalSettings{}, fmt.Errorf("invalid combatDifficulty.levels[%d].battleExpBonus in %q", i, path)
		}
		if lvl.BattleDropRateBonus < 0 {
			return SharedGlobalSettings{}, fmt.Errorf("invalid combatDifficulty.levels[%d].battleDropRateBonus in %q", i, path)
		}
		if lvl.StatMultiplierCurve == nil {
			return SharedGlobalSettings{}, fmt.Errorf("invalid combatDifficulty.levels[%d].statMultiplierCurve in %q", i, path)
		}
	}
	if settings.CombatBalanceModel.Version <= 0 {
		settings.CombatBalanceModel.Version = 1
	}
	if settings.CombatBalanceModel.MaxBattleLevel <= 0 {
		return SharedGlobalSettings{}, fmt.Errorf("invalid combatBalanceModel.maxBattleLevel in %q", path)
	}
	if settings.CombatBalanceModel.PrimaryPointCap <= 0 {
		return SharedGlobalSettings{}, fmt.Errorf("invalid combatBalanceModel.primaryPointCap in %q", path)
	}
	settings.CombatBalanceModel.PrimaryPointAttrs = uniqueNormalizedStrings(settings.CombatBalanceModel.PrimaryPointAttrs)
	if len(settings.CombatBalanceModel.PrimaryPointAttrs) == 0 {
		return SharedGlobalSettings{}, fmt.Errorf("invalid combatBalanceModel.primaryPointAttrs in %q: empty", path)
	}
	if settings.CombatBalanceModel.DefenseK <= 0 {
		return SharedGlobalSettings{}, fmt.Errorf("invalid combatBalanceModel.defenseK in %q", path)
	}
	if settings.CombatBalanceModel.EnemyHpRegenRatio < 0 || settings.CombatBalanceModel.EnemyHpRegenRatio > 1 {
		return SharedGlobalSettings{}, fmt.Errorf("invalid combatBalanceModel.enemyHpRegenRatio in %q", path)
	}
	if settings.CombatBalanceModel.PercentPenetrationCap < 0 || settings.CombatBalanceModel.PercentPenetrationCap > 1 {
		return SharedGlobalSettings{}, fmt.Errorf("invalid combatBalanceModel.percentPenetrationCap in %q", path)
	}
	if settings.CombatBalanceModel.ExpPerPower <= 0 {
		return SharedGlobalSettings{}, fmt.Errorf("invalid combatBalanceModel.expPerPower in %q", path)
	}
	if settings.CombatBalanceModel.TeamExtraEnemyScale < 0 {
		return SharedGlobalSettings{}, fmt.Errorf("invalid combatBalanceModel.teamExtraEnemyScale in %q", path)
	}
	if settings.CombatBalanceModel.TeamRewardEfficiencyCap < 1 {
		return SharedGlobalSettings{}, fmt.Errorf("invalid combatBalanceModel.teamRewardEfficiencyCap in %q", path)
	}
	if settings.CombatBalanceModel.ComboBudgetMinShare < 0 || settings.CombatBalanceModel.ComboBudgetMaxShare < settings.CombatBalanceModel.ComboBudgetMinShare {
		return SharedGlobalSettings{}, fmt.Errorf("invalid combatBalanceModel combo budget shares in %q", path)
	}
	if len(settings.CombatBalanceModel.LevelBands) == 0 {
		return SharedGlobalSettings{}, fmt.Errorf("invalid combatBalanceModel.levelBands in %q: empty", path)
	}
	for i, band := range settings.CombatBalanceModel.LevelBands {
		if band.Level <= 0 || band.PlayerDPR <= 0 || band.PlayerHP <= 0 || band.ExpRate <= 0 {
			return SharedGlobalSettings{}, fmt.Errorf("invalid combatBalanceModel.levelBands[%d] in %q", i, path)
		}
	}
	return settings, nil
}

func validateSharedAlchemySettings(path string, settings SharedAlchemySettings) error {
	if settings.InfusionBaseGoldCost < 0 {
		return fmt.Errorf("invalid alchemy.infusionBaseGoldCost in %q", path)
	}
	if settings.InfusionBaseDurationSec <= 0 {
		return fmt.Errorf("invalid alchemy.infusionBaseDurationSec in %q", path)
	}
	if !isFiniteNonNegative(settings.InfusionBaseExp) {
		return fmt.Errorf("invalid alchemy.infusionBaseExp in %q", path)
	}
	durations := map[string]int{
		"default":   settings.DurationsSec.Default,
		"convert":   settings.DurationsSec.Convert,
		"bench":     settings.DurationsSec.Bench,
		"transmute": settings.DurationsSec.Transmute,
		"smelt":     settings.DurationsSec.Smelt,
	}
	for key, value := range durations {
		if value <= 0 {
			return fmt.Errorf("invalid alchemy.durationsSec.%s in %q", key, path)
		}
	}
	if len(settings.TransmuteDurationByRarityIndex) == 0 {
		return fmt.Errorf("invalid alchemy.transmuteDurationByRarityIndex in %q: empty", path)
	}
	for i, value := range settings.TransmuteDurationByRarityIndex {
		if value <= 0 {
			return fmt.Errorf("invalid alchemy.transmuteDurationByRarityIndex[%d] in %q", i, path)
		}
	}
	if !isFiniteNonNegative(settings.LevelDeficitFailChancePerLevel) || settings.LevelDeficitFailChancePerLevel > 1 {
		return fmt.Errorf("invalid alchemy.levelDeficitFailChancePerLevel in %q", path)
	}
	if !isFiniteNonNegative(settings.LevelDeficitFailChanceCap) || settings.LevelDeficitFailChanceCap > 1 {
		return fmt.Errorf("invalid alchemy.levelDeficitFailChanceCap in %q", path)
	}
	if settings.Inherit.BaseInheritSlots < 0 {
		return fmt.Errorf("invalid alchemy.inherit.baseInheritSlots in %q", path)
	}
	if settings.Inherit.BaseSuppressSlots < 0 {
		return fmt.Errorf("invalid alchemy.inherit.baseSuppressSlots in %q", path)
	}
	if !isFiniteNonNegative(settings.Inherit.InheritSlotPrecisionScale) {
		return fmt.Errorf("invalid alchemy.inherit.inheritSlotPrecisionScalePerPoint in %q", path)
	}
	if !isFiniteNonNegative(settings.Inherit.SuppressSlotStabilityBonus) {
		return fmt.Errorf("invalid alchemy.inherit.suppressSlotStabilityBonusPerPoint in %q", path)
	}
	if !isFiniteNonNegative(settings.Inherit.PositiveIgnoreKeepChanceBase) || settings.Inherit.PositiveIgnoreKeepChanceBase > 1 {
		return fmt.Errorf("invalid alchemy.inherit.positiveIgnoreKeepChanceBase in %q", path)
	}
	if !isFiniteNonNegative(settings.Inherit.NegativeIgnoreKeepChanceBase) || settings.Inherit.NegativeIgnoreKeepChanceBase > 1 {
		return fmt.Errorf("invalid alchemy.inherit.negativeIgnoreKeepChanceBase in %q", path)
	}
	if !isFiniteNonNegative(settings.Inherit.HybridMergeDivisor) || settings.Inherit.HybridMergeDivisor < 1 {
		return fmt.Errorf("invalid alchemy.inherit.hybridMergeDivisor in %q", path)
	}
	if !isFiniteNonNegative(settings.Inherit.InbreedMutationChance) || settings.Inherit.InbreedMutationChance > 1 {
		return fmt.Errorf("invalid alchemy.inherit.inbreedMutationChance in %q", path)
	}
	if !isFiniteNonNegative(settings.Inherit.InbreedGoodChance) || settings.Inherit.InbreedGoodChance > 1 {
		return fmt.Errorf("invalid alchemy.inherit.inbreedGoodChance in %q", path)
	}
	if !isFiniteNonNegative(settings.Inherit.InbreedGoodMultiplierRange.Min) || !isFiniteNonNegative(settings.Inherit.InbreedGoodMultiplierRange.Max) || settings.Inherit.InbreedGoodMultiplierRange.Min <= 0 || settings.Inherit.InbreedGoodMultiplierRange.Max < settings.Inherit.InbreedGoodMultiplierRange.Min {
		return fmt.Errorf("invalid alchemy.inherit.inbreedGoodMultiplierRange in %q", path)
	}
	if !isFiniteNonNegative(settings.Inherit.InbreedBadMultiplierRange.Min) || !isFiniteNonNegative(settings.Inherit.InbreedBadMultiplierRange.Max) || settings.Inherit.InbreedBadMultiplierRange.Min <= 0 || settings.Inherit.InbreedBadMultiplierRange.Max < settings.Inherit.InbreedBadMultiplierRange.Min {
		return fmt.Errorf("invalid alchemy.inherit.inbreedBadMultiplierRange in %q", path)
	}
	if !isFiniteNonNegative(settings.Inherit.MutationChance) || settings.Inherit.MutationChance > 1 {
		return fmt.Errorf("invalid alchemy.inherit.mutationChance in %q", path)
	}
	if !isFiniteNonNegative(settings.Inherit.NumericValueCapMultiplier) || settings.Inherit.NumericValueCapMultiplier < 1 {
		return fmt.Errorf("invalid alchemy.inherit.numericValueCapMultiplier in %q", path)
	}
	requiredRarities := []string{"common", "uncommon", "rare", "epic", "legendary"}
	for _, rarity := range requiredRarities {
		row, ok := settings.ConvertGroupEconomyByRarity[rarity]
		if !ok {
			return fmt.Errorf("invalid alchemy.convertGroupEconomyByRarity in %q: missing %s", path, rarity)
		}
		if row.Gold < 0 || row.Duration <= 0 || !isFiniteNonNegative(row.Exp) || !isFiniteNonNegative(row.Upgrade) || row.Upgrade > 1 {
			return fmt.Errorf("invalid alchemy.convertGroupEconomyByRarity.%s in %q", rarity, path)
		}
	}
	return nil
}

func isFiniteNonNegative(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= 0
}

func int64Ptr(value int64) *int64 {
	return &value
}

func normalizedID(raw string) string {
	return normalizedString(raw)
}

func normalizedString(raw string) string {
	return lowerTrimmed(raw)
}

func lowerTrimmed(raw string) string {
	return strings.ToLower(strings.TrimSpace(raw))
}

func uniqueNormalizedStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, raw := range values {
		normalized := normalizedString(raw)
		if normalized == "" {
			continue
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}
	return out
}

func normalizeItemRenameLabelColors(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range uniqueNormalizedStrings(values) {
		if !isSupportedItemRenameLabelColor(value) {
			continue
		}
		out = append(out, value)
	}
	if len(out) == 0 {
		out = []string{"gold", "rose", "violet", "amber", "sky", "emerald", "fuchsia"}
	}
	return out
}

func isSupportedItemRenameLabelColor(value string) bool {
	switch value {
	case "gold", "rose", "violet", "amber", "sky", "emerald", "fuchsia":
		return true
	default:
		return false
	}
}

func normalizedBattlePosition(raw string) string {
	switch lowerTrimmed(raw) {
	case "midguard":
		return "midguard"
	case "rearguard":
		return "rearguard"
	case "vanguard":
		return "vanguard"
	default:
		return ""
	}
}
