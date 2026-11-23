package config

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
)

type KittySettings struct {
	Version        string                     `json:"version"`
	Leveling       KittyLevelingSettings      `json:"leveling"`
	JournalRewards KittyJournalRewardSettings `json:"journalRewards"`
	Commissions    KittyCommissionSettings    `json:"commissions"`
}

type KittyLevelingSettings struct {
	MaxLevel        int                          `json:"maxLevel"`
	FallbackFormula KittyLevelFallbackFormula    `json:"fallbackFormula"`
	Levels          []KittyLevelSetting          `json:"levels"`
	Milestones      []KittyLevelMilestoneSetting `json:"milestones"`
}

type KittyLevelFallbackFormula struct {
	Base     int64   `json:"base"`
	PerLevel int64   `json:"perLevel"`
	Power    float64 `json:"power"`
}

type KittyLevelSetting struct {
	Level   int   `json:"level"`
	NextExp int64 `json:"nextExp"`
}

type KittyLevelMilestoneSetting struct {
	Level   int            `json:"level"`
	NameKey string         `json:"nameKey"`
	DescKey string         `json:"descKey"`
	Effects map[string]any `json:"effects"`
}

type KittyJournalRewardSettings struct {
	Enabled                bool                                  `json:"enabled"`
	ContentPoolMap         map[string]string                     `json:"contentPoolMap"`
	CategoryPoolMap        map[string]string                     `json:"categoryPoolMap"`
	Pools                  map[string]KittyRewardPool            `json:"pools"`
	ScheduledAffection     KittyScheduledAffectionRewardSettings `json:"scheduledAffection"`
	CuteEquipmentDrop      KittyCuteEquipmentDropSettings        `json:"cuteEquipmentDrop"`
	RareItemDrops          []KittyRareItemDropSettings           `json:"rareItemDrops"`
	EncounterRareItemDrops []KittyRareItemDropSettings           `json:"encounterRareItemDrops"`
}

type KittyScheduledAffectionRewardSettings struct {
	Enabled       bool     `json:"enabled"`
	Timezone      string   `json:"timezone"`
	Times         []string `json:"times"`
	RewardKey     string   `json:"rewardKey"`
	RewardItemID  string   `json:"rewardItemId"`
	Quantity      int64    `json:"quantity"`
	ContentKey    string   `json:"contentKey"`
	MaxMissedDays int      `json:"maxMissedDays"`
}

type KittyCuteEquipmentDropSettings struct {
	Enabled bool     `json:"enabled"`
	Chance  float64  `json:"chance"`
	ItemIDs []string `json:"itemIds"`
}

type KittyRareItemDropSettings struct {
	Enabled     bool     `json:"enabled"`
	Chance      float64  `json:"chance"`
	ItemID      string   `json:"itemId"`
	Quantity    int64    `json:"quantity"`
	ContentKey  string   `json:"contentKey"`
	ContentKeys []string `json:"contentKeys"`
}

type KittyCommissionSettings struct {
	Enabled            bool                       `json:"enabled"`
	RefreshMode        string                     `json:"refreshMode"`
	VisibleCount       int                        `json:"visibleCount"`
	DailyLimitPerKitty int                        `json:"dailyLimitPerKitty"`
	Pools              []KittyCommissionPool      `json:"pools"`
	Items              []KittyCommissionItem      `json:"items"`
	RewardPools        map[string]KittyRewardPool `json:"rewardPools"`
}

type KittyCommissionPool struct {
	ID            string                     `json:"id"`
	MinKittyLevel int                        `json:"minKittyLevel"`
	Entries       []KittyCommissionPoolEntry `json:"entries"`
}

type KittyCommissionPoolEntry struct {
	CommissionID string  `json:"commissionId"`
	Weight       float64 `json:"weight"`
}

type KittyCommissionItem struct {
	ID              string           `json:"id"`
	Rarity          string           `json:"rarity"`
	NameKey         string           `json:"nameKey"`
	DescKey         string           `json:"descKey"`
	ResultKey       string           `json:"resultKey"`
	MinKittyLevel   int              `json:"minKittyLevel"`
	DurationSeconds int64            `json:"durationSeconds"`
	Costs           map[string]int64 `json:"costs"`
	RewardPoolID    string           `json:"rewardPoolId"`
	KittyExp        int64            `json:"kittyExp"`
	BondExp         int64            `json:"bondExp"`
	MoodDelta       float64          `json:"moodDelta"`
}

type KittyRewardPool struct {
	LabelKey      string            `json:"labelKey"`
	MinKittyLevel int               `json:"minKittyLevel"`
	Rolls         []KittyRewardRoll `json:"rolls"`
	KittyExp      *KittyIntRange    `json:"kittyExp,omitempty"`
	BondExp       *KittyIntRange    `json:"bondExp,omitempty"`
	MoodDelta     float64           `json:"moodDelta"`
}

type KittyRewardRoll struct {
	Chance  float64            `json:"chance"`
	Mode    string             `json:"mode"`
	Entries []KittyRewardEntry `json:"entries"`
}

type KittyRewardEntry struct {
	ItemID string  `json:"itemId"`
	Weight float64 `json:"weight,omitempty"`
	Chance float64 `json:"chance,omitempty"`
	Min    int64   `json:"min"`
	Max    int64   `json:"max"`
}

type KittyIntRange struct {
	Min int64 `json:"min"`
	Max int64 `json:"max"`
}

func DefaultKittySettings() KittySettings {
	return KittySettings{
		Version: "v1",
		Leveling: KittyLevelingSettings{
			MaxLevel:        30,
			FallbackFormula: KittyLevelFallbackFormula{Base: 100, PerLevel: 50, Power: 1.25},
			Levels: []KittyLevelSetting{
				{Level: 1, NextExp: 80},
				{Level: 2, NextExp: 120},
				{Level: 3, NextExp: 180},
				{Level: 4, NextExp: 260},
				{Level: 5, NextExp: 360},
			},
		},
		JournalRewards: KittyJournalRewardSettings{
			Enabled:        true,
			ContentPoolMap: map[string]string{},
			CategoryPoolMap: map[string]string{
				"diary.idle":    "idle_default_pool",
				"diary.working": "working_default_pool",
			},
			ScheduledAffection: KittyScheduledAffectionRewardSettings{
				Enabled:      true,
				Timezone:     "Asia/Shanghai",
				Times:        []string{"09:00", "18:00"},
				RewardKey:    "scheduled_affection",
				RewardItemID: "kitty_affection",
				Quantity:     2,
				ContentKey:   "game.catteryJournal.diary.affectionGrant",
			},
			CuteEquipmentDrop: KittyCuteEquipmentDropSettings{
				Enabled: true,
				Chance:  0.005,
				ItemIDs: []string{"cute_hat", "cute_clothes", "cute_gloves", "cute_shoes", "cute_cape"},
			},
			RareItemDrops:          []KittyRareItemDropSettings{},
			EncounterRareItemDrops: []KittyRareItemDropSettings{},
			Pools: map[string]KittyRewardPool{
				"idle_default_pool": {
					LabelKey: "game.catteryRewardPool.idle_default_pool",
					Rolls: []KittyRewardRoll{{Chance: 0.2, Mode: "weightedOne", Entries: []KittyRewardEntry{
						{ItemID: "wood", Weight: 1, Min: 1, Max: 2},
					}}},
					KittyExp:  &KittyIntRange{Min: 1, Max: 3},
					BondExp:   &KittyIntRange{Min: 1, Max: 1},
					MoodDelta: 0.01,
				},
				"working_default_pool": {
					LabelKey: "game.catteryRewardPool.working_default_pool",
					Rolls: []KittyRewardRoll{{Chance: 0.25, Mode: "weightedOne", Entries: []KittyRewardEntry{
						{ItemID: "gold_coin", Weight: 1, Min: 1, Max: 3},
					}}},
					KittyExp:  &KittyIntRange{Min: 2, Max: 5},
					BondExp:   &KittyIntRange{Min: 1, Max: 2},
					MoodDelta: 0.01,
				},
			},
		},
		Commissions: KittyCommissionSettings{
			Enabled:            true,
			RefreshMode:        "daily",
			VisibleCount:       3,
			DailyLimitPerKitty: 6,
			Pools: []KittyCommissionPool{{
				ID:            "default_daily",
				MinKittyLevel: 1,
				Entries: []KittyCommissionPoolEntry{
					{CommissionID: "bring_back_wood", Weight: 80},
					{CommissionID: "find_gold_coin", Weight: 35},
					{CommissionID: "gather_silverleaf", Weight: 10},
				},
			}},
			Items: []KittyCommissionItem{
				{
					ID: "bring_back_wood", NameKey: "game.catteryCommission.bring_back_wood.name", DescKey: "game.catteryCommission.bring_back_wood.desc", ResultKey: "game.catteryCommission.bring_back_wood.result",
					Rarity: "common", MinKittyLevel: 1, DurationSeconds: 1800, Costs: map[string]int64{"wood": 2}, RewardPoolID: "commission_wood_pool", KittyExp: 30, BondExp: 8, MoodDelta: 0.03,
				},
				{
					ID: "find_gold_coin", NameKey: "game.catteryCommission.find_gold_coin.name", DescKey: "game.catteryCommission.find_gold_coin.desc", ResultKey: "game.catteryCommission.find_gold_coin.result",
					Rarity: "uncommon", MinKittyLevel: 2, DurationSeconds: 3600, Costs: map[string]int64{"wood": 5}, RewardPoolID: "commission_coin_pool", KittyExp: 45, BondExp: 10, MoodDelta: 0.04,
				},
				{
					ID: "gather_silverleaf", NameKey: "game.catteryCommission.gather_silverleaf.name", DescKey: "game.catteryCommission.gather_silverleaf.desc", ResultKey: "game.catteryCommission.gather_silverleaf.result",
					Rarity: "rare", MinKittyLevel: 5, DurationSeconds: 7200, Costs: map[string]int64{"sturdy_wood": 2, "gold_coin": 20}, RewardPoolID: "commission_silverleaf_pool", KittyExp: 90, BondExp: 18, MoodDelta: 0.05,
				},
			},
			RewardPools: map[string]KittyRewardPool{
				"commission_wood_pool":       {LabelKey: "game.catteryRewardPool.commission_wood_pool", Rolls: []KittyRewardRoll{{Chance: 1, Mode: "weightedOne", Entries: []KittyRewardEntry{{ItemID: "wood", Weight: 1, Min: 1, Max: 4}}}}},
				"commission_coin_pool":       {LabelKey: "game.catteryRewardPool.commission_coin_pool", Rolls: []KittyRewardRoll{{Chance: 1, Mode: "weightedOne", Entries: []KittyRewardEntry{{ItemID: "gold_coin", Weight: 1, Min: 20, Max: 45}}}}},
				"commission_silverleaf_pool": {LabelKey: "game.catteryRewardPool.commission_silverleaf_pool", Rolls: []KittyRewardRoll{{Chance: 1, Mode: "weightedOne", Entries: []KittyRewardEntry{{ItemID: "silverleaf_wood", Weight: 1, Min: 1, Max: 2}}}, {Chance: 0.35, Mode: "weightedOne", Entries: []KittyRewardEntry{{ItemID: "gold_coin", Weight: 1, Min: 30, Max: 80}}}}},
			},
		},
	}
}

func LoadKittySettings(path string) (KittySettings, error) {
	settings := DefaultKittySettings()
	raw, err := os.ReadFile(path)
	if err != nil {
		return KittySettings{}, fmt.Errorf("read kitty config %q: %w", path, err)
	}
	if err := json.Unmarshal(raw, &settings); err != nil {
		return KittySettings{}, fmt.Errorf("parse kitty config %q: %w", path, err)
	}
	if err := ValidateKittySettings(settings); err != nil {
		return KittySettings{}, fmt.Errorf("invalid kitty config %q: %w", path, err)
	}
	return settings, nil
}

func ValidateKittySettings(settings KittySettings) error {
	if strings.TrimSpace(settings.Version) == "" {
		return fmt.Errorf("version is required")
	}
	if settings.Leveling.MaxLevel < 1 {
		return fmt.Errorf("leveling.maxLevel invalid")
	}
	if settings.Leveling.FallbackFormula.Base <= 0 || settings.Leveling.FallbackFormula.PerLevel < 0 || settings.Leveling.FallbackFormula.Power <= 0 {
		return fmt.Errorf("leveling.fallbackFormula invalid")
	}
	seenLevels := map[int]bool{}
	for _, row := range settings.Leveling.Levels {
		if row.Level < 1 || row.Level > settings.Leveling.MaxLevel || row.NextExp <= 0 {
			return fmt.Errorf("leveling.levels invalid: level=%d", row.Level)
		}
		if seenLevels[row.Level] {
			return fmt.Errorf("leveling.levels duplicate: %d", row.Level)
		}
		seenLevels[row.Level] = true
	}
	if err := validateRewardPoolMap(settings.JournalRewards.Pools, "journalRewards.pools"); err != nil {
		return err
	}
	if err := validateScheduledAffectionSettings(settings.JournalRewards.ScheduledAffection); err != nil {
		return err
	}
	if err := validateCuteEquipmentDropSettings(settings.JournalRewards.CuteEquipmentDrop); err != nil {
		return err
	}
	if err := validateRareItemDropSettings(settings.JournalRewards.RareItemDrops, "journalRewards.rareItemDrops"); err != nil {
		return err
	}
	if err := validateRareItemDropSettings(settings.JournalRewards.EncounterRareItemDrops, "journalRewards.encounterRareItemDrops"); err != nil {
		return err
	}
	for key, poolID := range settings.JournalRewards.ContentPoolMap {
		if strings.TrimSpace(key) == "" || settings.JournalRewards.Pools[strings.TrimSpace(poolID)].LabelKey == "" {
			return fmt.Errorf("journalRewards.contentPoolMap invalid: %s", key)
		}
	}
	for key, poolID := range settings.JournalRewards.CategoryPoolMap {
		if strings.TrimSpace(key) == "" || settings.JournalRewards.Pools[strings.TrimSpace(poolID)].LabelKey == "" {
			return fmt.Errorf("journalRewards.categoryPoolMap invalid: %s", key)
		}
	}
	if settings.Commissions.RefreshMode == "" {
		settings.Commissions.RefreshMode = "daily"
	}
	if settings.Commissions.RefreshMode != "daily" {
		return fmt.Errorf("commissions.refreshMode unsupported: %s", settings.Commissions.RefreshMode)
	}
	if settings.Commissions.VisibleCount <= 0 {
		return fmt.Errorf("commissions.visibleCount invalid")
	}
	if settings.Commissions.DailyLimitPerKitty <= 0 {
		return fmt.Errorf("commissions.dailyLimitPerKitty invalid")
	}
	if err := validateRewardPoolMap(settings.Commissions.RewardPools, "commissions.rewardPools"); err != nil {
		return err
	}
	itemIDs := map[string]bool{}
	for _, item := range settings.Commissions.Items {
		id := strings.TrimSpace(item.ID)
		if id == "" || itemIDs[id] {
			return fmt.Errorf("commissions.items id invalid: %s", id)
		}
		itemIDs[id] = true
		if strings.TrimSpace(item.Rarity) == "" {
			return fmt.Errorf("commissions.items rarity invalid: %s", id)
		}
		if strings.TrimSpace(item.NameKey) == "" || strings.TrimSpace(item.DescKey) == "" || strings.TrimSpace(item.ResultKey) == "" {
			return fmt.Errorf("commissions.items i18n key invalid: %s", id)
		}
		if item.MinKittyLevel < 1 || item.DurationSeconds <= 0 || item.KittyExp < 0 || item.BondExp < 0 || math.IsNaN(item.MoodDelta) || math.IsInf(item.MoodDelta, 0) {
			return fmt.Errorf("commissions.items value invalid: %s", id)
		}
		if _, ok := settings.Commissions.RewardPools[strings.TrimSpace(item.RewardPoolID)]; !ok {
			return fmt.Errorf("commissions.items rewardPoolId not found: %s.%s", id, item.RewardPoolID)
		}
		for costID, qty := range item.Costs {
			if strings.TrimSpace(costID) == "" || qty <= 0 {
				return fmt.Errorf("commissions.items costs invalid: %s", id)
			}
		}
	}
	for _, pool := range settings.Commissions.Pools {
		if strings.TrimSpace(pool.ID) == "" || pool.MinKittyLevel < 1 || len(pool.Entries) == 0 {
			return fmt.Errorf("commissions.pools invalid: %s", pool.ID)
		}
		for _, entry := range pool.Entries {
			if !itemIDs[strings.TrimSpace(entry.CommissionID)] || entry.Weight <= 0 || math.IsNaN(entry.Weight) || math.IsInf(entry.Weight, 0) {
				return fmt.Errorf("commissions.pools entry invalid: %s.%s", pool.ID, entry.CommissionID)
			}
		}
	}
	return nil
}

func ValidateKittySettingsItemIDs(settings KittySettings, itemIDs map[string]bool) error {
	if len(itemIDs) == 0 {
		return nil
	}
	checkPools := func(pools map[string]KittyRewardPool, path string) error {
		for poolID, pool := range pools {
			for i, roll := range pool.Rolls {
				for j, entry := range roll.Entries {
					itemID := strings.TrimSpace(entry.ItemID)
					if !itemIDs[itemID] {
						return fmt.Errorf("%s.%s.rolls[%d].entries[%d].itemId not found: %s", path, poolID, i, j, itemID)
					}
				}
			}
		}
		return nil
	}
	if err := checkPools(settings.JournalRewards.Pools, "journalRewards.pools"); err != nil {
		return err
	}
	if scheduled := settings.JournalRewards.ScheduledAffection; scheduled.Enabled {
		itemID := strings.TrimSpace(scheduled.RewardItemID)
		if !itemIDs[itemID] {
			return fmt.Errorf("journalRewards.scheduledAffection.rewardItemId not found: %s", itemID)
		}
	}
	if drop := settings.JournalRewards.CuteEquipmentDrop; drop.Enabled {
		for i, itemID := range drop.ItemIDs {
			normalized := strings.TrimSpace(itemID)
			if !itemIDs[normalized] {
				return fmt.Errorf("journalRewards.cuteEquipmentDrop.itemIds[%d] not found: %s", i, normalized)
			}
		}
	}
	for i, drop := range settings.JournalRewards.RareItemDrops {
		if !drop.Enabled {
			continue
		}
		itemID := strings.TrimSpace(drop.ItemID)
		if !itemIDs[itemID] {
			return fmt.Errorf("journalRewards.rareItemDrops[%d].itemId not found: %s", i, itemID)
		}
	}
	for i, drop := range settings.JournalRewards.EncounterRareItemDrops {
		if !drop.Enabled {
			continue
		}
		itemID := strings.TrimSpace(drop.ItemID)
		if !itemIDs[itemID] {
			return fmt.Errorf("journalRewards.encounterRareItemDrops[%d].itemId not found: %s", i, itemID)
		}
	}
	if err := checkPools(settings.Commissions.RewardPools, "commissions.rewardPools"); err != nil {
		return err
	}
	for _, item := range settings.Commissions.Items {
		for itemID := range item.Costs {
			if !itemIDs[strings.TrimSpace(itemID)] {
				return fmt.Errorf("commissions.items.%s.costs itemId not found: %s", item.ID, itemID)
			}
		}
	}
	return nil
}

func ValidateKittySettingsRarityIDs(settings KittySettings, rarityIDs map[string]bool) error {
	if len(rarityIDs) == 0 {
		return nil
	}
	for _, item := range settings.Commissions.Items {
		rarity := strings.TrimSpace(item.Rarity)
		if !rarityIDs[rarity] {
			return fmt.Errorf("commissions.items.%s.rarity not found: %s", item.ID, rarity)
		}
	}
	return nil
}

func validateScheduledAffectionSettings(settings KittyScheduledAffectionRewardSettings) error {
	if !settings.Enabled {
		return nil
	}
	if strings.TrimSpace(settings.Timezone) == "" {
		return fmt.Errorf("journalRewards.scheduledAffection.timezone invalid")
	}
	if len(settings.Times) == 0 {
		return fmt.Errorf("journalRewards.scheduledAffection.times empty")
	}
	seen := map[string]bool{}
	for _, timeKey := range settings.Times {
		normalized := strings.TrimSpace(timeKey)
		if normalized == "" || seen[normalized] {
			return fmt.Errorf("journalRewards.scheduledAffection.times invalid: %s", timeKey)
		}
		seen[normalized] = true
		parts := strings.Split(normalized, ":")
		if len(parts) != 2 {
			return fmt.Errorf("journalRewards.scheduledAffection.times invalid: %s", timeKey)
		}
		hour := parseSmallInt(parts[0], -1)
		minute := parseSmallInt(parts[1], -1)
		if hour < 0 || hour > 23 || minute < 0 || minute > 59 {
			return fmt.Errorf("journalRewards.scheduledAffection.times invalid: %s", timeKey)
		}
	}
	if strings.TrimSpace(settings.RewardKey) == "" || strings.TrimSpace(settings.RewardItemID) == "" || settings.Quantity <= 0 {
		return fmt.Errorf("journalRewards.scheduledAffection reward invalid")
	}
	if strings.TrimSpace(settings.ContentKey) == "" {
		return fmt.Errorf("journalRewards.scheduledAffection.contentKey invalid")
	}
	if settings.MaxMissedDays < 0 {
		return fmt.Errorf("journalRewards.scheduledAffection.maxMissedDays invalid")
	}
	return nil
}

func validateCuteEquipmentDropSettings(settings KittyCuteEquipmentDropSettings) error {
	if !settings.Enabled {
		return nil
	}
	if settings.Chance < 0 || settings.Chance > 1 || math.IsNaN(settings.Chance) || math.IsInf(settings.Chance, 0) {
		return fmt.Errorf("journalRewards.cuteEquipmentDrop.chance invalid")
	}
	if len(settings.ItemIDs) == 0 {
		return fmt.Errorf("journalRewards.cuteEquipmentDrop.itemIds empty")
	}
	seen := map[string]bool{}
	for _, itemID := range settings.ItemIDs {
		normalized := strings.TrimSpace(itemID)
		if normalized == "" || seen[normalized] {
			return fmt.Errorf("journalRewards.cuteEquipmentDrop.itemIds invalid: %s", itemID)
		}
		seen[normalized] = true
	}
	return nil
}

func validateRareItemDropSettings(settings []KittyRareItemDropSettings, path string) error {
	seen := map[string]bool{}
	for i, drop := range settings {
		if !drop.Enabled {
			continue
		}
		itemID := strings.TrimSpace(drop.ItemID)
		if itemID == "" || seen[itemID] {
			return fmt.Errorf("%s[%d].itemId invalid", path, i)
		}
		seen[itemID] = true
		if drop.Chance < 0 || drop.Chance > 1 || math.IsNaN(drop.Chance) || math.IsInf(drop.Chance, 0) {
			return fmt.Errorf("%s[%d].chance invalid", path, i)
		}
		if drop.Quantity <= 0 {
			return fmt.Errorf("%s[%d].quantity invalid", path, i)
		}
		if strings.TrimSpace(drop.ContentKey) == "" && len(drop.ContentKeys) == 0 {
			return fmt.Errorf("%s[%d].contentKey invalid", path, i)
		}
		for j, key := range drop.ContentKeys {
			if strings.TrimSpace(key) == "" {
				return fmt.Errorf("%s[%d].contentKeys[%d] invalid", path, i, j)
			}
		}
	}
	return nil
}

func parseSmallInt(raw string, fallback int) int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fallback
	}
	value := 0
	for _, ch := range raw {
		if ch < '0' || ch > '9' {
			return fallback
		}
		value = value*10 + int(ch-'0')
	}
	return value
}

func validateRewardPoolMap(pools map[string]KittyRewardPool, path string) error {
	keys := make([]string, 0, len(pools))
	for key := range pools {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, id := range keys {
		pool := pools[id]
		if strings.TrimSpace(id) == "" || strings.TrimSpace(pool.LabelKey) == "" {
			return fmt.Errorf("%s id/labelKey invalid: %s", path, id)
		}
		if pool.MinKittyLevel < 0 {
			return fmt.Errorf("%s minKittyLevel invalid: %s", path, id)
		}
		if pool.KittyExp != nil && (pool.KittyExp.Min < 0 || pool.KittyExp.Max < pool.KittyExp.Min) {
			return fmt.Errorf("%s kittyExp invalid: %s", path, id)
		}
		if pool.BondExp != nil && (pool.BondExp.Min < 0 || pool.BondExp.Max < pool.BondExp.Min) {
			return fmt.Errorf("%s bondExp invalid: %s", path, id)
		}
		if math.IsNaN(pool.MoodDelta) || math.IsInf(pool.MoodDelta, 0) {
			return fmt.Errorf("%s moodDelta invalid: %s", path, id)
		}
		for i, roll := range pool.Rolls {
			if roll.Chance < 0 || roll.Chance > 1 || math.IsNaN(roll.Chance) || math.IsInf(roll.Chance, 0) {
				return fmt.Errorf("%s rolls[%d].chance invalid: %s", path, i, id)
			}
			if roll.Mode != "weightedOne" && roll.Mode != "independent" {
				return fmt.Errorf("%s rolls[%d].mode invalid: %s", path, i, id)
			}
			if len(roll.Entries) == 0 {
				return fmt.Errorf("%s rolls[%d].entries empty: %s", path, i, id)
			}
			for j, entry := range roll.Entries {
				if strings.TrimSpace(entry.ItemID) == "" || entry.Min <= 0 || entry.Max < entry.Min {
					return fmt.Errorf("%s rolls[%d].entries[%d] invalid: %s", path, i, j, id)
				}
				if roll.Mode == "weightedOne" && (entry.Weight <= 0 || math.IsNaN(entry.Weight) || math.IsInf(entry.Weight, 0)) {
					return fmt.Errorf("%s rolls[%d].entries[%d].weight invalid: %s", path, i, j, id)
				}
				if roll.Mode == "independent" && (entry.Chance < 0 || entry.Chance > 1 || math.IsNaN(entry.Chance) || math.IsInf(entry.Chance, 0)) {
					return fmt.Errorf("%s rolls[%d].entries[%d].chance invalid: %s", path, i, j, id)
				}
			}
		}
	}
	return nil
}
