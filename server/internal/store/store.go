package store

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"mysecondwebgame/server/internal/catalog"
	"mysecondwebgame/server/internal/config"
	"mysecondwebgame/server/internal/operationeffect"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	db                          *pgxpool.Pool
	config                      RuntimeConfig
	gameData                    catalog.Data
	enhanceCache                *EnhanceRuntimeCache
	attributeExpProfile         *attributeExpProfileResolver
	specializationCacheOnce     sync.Once
	specializationCache         *specializationTreeCache
	specializationCacheErr      error
	auditMu                     sync.Mutex
	auditBuffers                map[pgx.Tx]*inventoryAuditBuffer
	achievementNotify           func(context.Context)
	achievementEnqueue          func(ctx context.Context, userID int64, events []AchievementEvent) error
	achievementDefs             []achievementDefinition
	achievementDefsByCounterKey map[string][]achievementDefinition
	operationEffectRegistry     *operationeffect.Registry
	operationEffectDefinitions  map[string]operationeffect.Definition
	furnitureEffects            furnitureEffectCache
	furniturePolicy             *furnitureRuntimePolicy
	furnitureChanged            func(context.Context, int64, int64)
	// battleStateMachineV2 enables normalization of historical v1 rooms when
	// they reach an explicit user/admin transition. Runtime dispatch is V2-only.
	battleStateMachineV2 bool
}

// SetBattleStateMachineV2 controls historical-room normalization.
func (s *Store) SetBattleStateMachineV2(enabled bool) {
	s.battleStateMachineV2 = enabled
}

type RuntimeConfig struct {
	WarehouseDefaultCols                     int
	WarehouseDefaultRows                     int
	WarehouseLogDefaultLookback              time.Duration
	OfflineProgressMaxDuration               time.Duration
	OfflineSettleExecBudget                  time.Duration
	ActionQueueFastForwardMaxSteps           int
	OrdinarySettlementEnabled                bool
	ActionQueueFastForwardMaxExec            time.Duration
	ActionQueueLaneLimit                     int
	KittyActionQueueLaneLimit                int
	KittyStatusExpMultiplier                 float64
	KittyBattleEquipSlots                    []string
	KittyBattleAttrExpMultiplier             float64
	KittyBattleSkillExpMultiplier            float64
	KittyBattleLevelCapDefault               int64
	KittyBattleLevelCapPerKittyLevel         int64
	KittyMaxPersonalities                    int
	KittyRemovePersonalityItemID             string
	KittyRemovePersonalityCount              int64
	KittyDefaultPersonalityID                string
	KittyPersonalityProfiles                 map[string]KittyPersonalityProfile
	KittyPersonalityOrder                    []string
	KittyMaxLevel                            int
	KittyLevelExp                            map[int]int64
	KittyLevelExpBase                        int64
	KittyLevelExpPerLevel                    int64
	KittyLevelExpPower                       float64
	AuctionFeeRateBps                        int64
	AuctionMinBidIncrement                   int64
	AuctionAntiSnipeTriggerSeconds           int64
	AuctionAntiSnipeExtendSeconds            int64
	AuctionMinRepLevel                       int64
	ItemRenameCost                           map[string]int64
	ItemRenameMaxNameRunes                   int
	ItemRenameLabelColors                    []string
	PlayerUsernameRenameCostItemID           string
	PlayerUsernameRenameCostItemCount        int64
	ActionEnhanceInheritanceRatio            float64
	ItemRecycleGoldCoefficient               float64
	ItemRecycleAffixMismatchExtraCoefficient float64
	SkillLevelUpBaseExp                      int64
	SkillLevelUpPerLevelExtraExp             int64
	AchievementsEnabled                      bool
	CatVillageRepLevels                      []config.CatVillageRepLevel
	Battle                                   BattleRuntimeConfig
	DerivedStatsFormula                      DerivedStatsFormulaConfig
	Alchemy                                  AlchemyRuntimeConfig
	Fishing                                  config.FishingSettings
	SkillForge                               config.SkillForgeSettings
}

type AlchemyRuntimeConfig struct {
	InfusionBaseGoldCost           int64
	InfusionBaseDurationSec        int
	InfusionBaseExp                float64
	DurationDefaultSec             int
	DurationConvertSec             int
	DurationBenchSec               int
	DurationTransmuteSec           int
	DurationSmeltSec               int
	TransmuteDurationByRarityIndex []int
	LevelDeficitFailChancePerLevel float64
	LevelDeficitFailChanceCap      float64
	Inherit                        AlchemyInheritRuntimeConfig
}

type AlchemyInheritRuntimeConfig struct {
	BaseInheritSlots             int
	BaseSuppressSlots            int
	InheritSlotPrecisionScale    float64
	SuppressSlotStabilityBonus   float64
	PositiveIgnoreKeepChanceBase float64
	NegativeIgnoreKeepChanceBase float64
	HybridMergeDivisor           float64
	InbreedMutationChance        float64
	InbreedGoodChance            float64
	InbreedGoodMultiplierRange   catalog.AlchemyInheritMultiplierRange
	InbreedBadMultiplierRange    catalog.AlchemyInheritMultiplierRange
	MutationChance               float64
	NumericValueCapMultiplier    float64
}

func (c AlchemyRuntimeConfig) DurationForAction(actionID string) int {
	switch actionID {
	case "alchemy.convert":
		if c.DurationConvertSec > 0 {
			return c.DurationConvertSec
		}
	case "alchemy.bench":
		if c.DurationBenchSec > 0 {
			return c.DurationBenchSec
		}
	case "alchemy.transmute":
		if c.DurationTransmuteSec > 0 {
			return c.DurationTransmuteSec
		}
	case "alchemy.smelt":
		if c.DurationSmeltSec > 0 {
			return c.DurationSmeltSec
		}
	case "alchemy.inherit":
		return 60
	}
	if c.DurationDefaultSec > 0 {
		return c.DurationDefaultSec
	}
	return 12
}

func (c AlchemyRuntimeConfig) TransmuteDurationByRarity(rarityOrder int) int {
	if len(c.TransmuteDurationByRarityIndex) == 0 {
		return c.DurationForAction("alchemy.transmute")
	}
	idx := rarityOrder
	if idx < 0 {
		idx = 0
	}
	if idx >= len(c.TransmuteDurationByRarityIndex) {
		idx = len(c.TransmuteDurationByRarityIndex) - 1
	}
	if d := c.TransmuteDurationByRarityIndex[idx]; d > 0 {
		return d
	}
	return c.DurationForAction("alchemy.transmute")
}

type BattleRuntimeConfig struct {
	DropLuckFactor        float64
	ExpBonusStart         time.Duration
	ExpBonusPerSecond     float64
	ExpBonusMaxMultiplier float64
	ResetItemID           string
	ResetItemCount        int64
}

type DerivedStatsFormulaConfig struct {
	Version   string
	Operators []string
	Formulas  []DerivedStatFormula
}

type DerivedStatFormula struct {
	TargetAttr  string
	IncludeBase bool
	Terms       []DerivedStatFormulaTerm
}

type DerivedStatFormulaTerm struct {
	AttrKey     string
	Coefficient float64
}

type KittyPersonalityProfile struct {
	ID      string
	NameKey string
	Weight  float64
	Effects []KittyPersonalityEffect
}

type KittyPersonalityEffect struct {
	Kind            string
	Attrs           map[string]float64
	Position        string
	BattleHooks     []map[string]any
	EffectMutations []map[string]any
}

type BuyListingResult struct {
	OrderID      int64 `json:"orderId"`
	Created      bool  `json:"created"`
	FeeRateBps   int64 `json:"feeRateBps"`
	FeeAmount    int64 `json:"feeAmount"`
	SellerAmount int64 `json:"sellerAmount"`
	SellerID     int64 `json:"sellerId,omitempty"`
	BuyerID      int64 `json:"buyerId,omitempty"`
}

type MarketListing struct {
	ID          int64     `json:"id"`
	SellerID    int64     `json:"sellerId"`
	ItemID      string    `json:"itemId"`
	ListingKind string    `json:"listingKind"`
	InstanceID  *int64    `json:"instanceId,omitempty"`
	Price       int64     `json:"price"`
	Quantity    int64     `json:"quantity"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"createdAt"`
}

type MarketBuyOrder struct {
	ID                int64     `json:"id"`
	BuyerID           int64     `json:"buyerId"`
	ItemID            string    `json:"itemId"`
	Price             int64     `json:"price"`
	Quantity          int64     `json:"quantity"`
	RemainingQuantity int64     `json:"remainingQuantity"`
	Status            string    `json:"status"`
	CreatedAt         time.Time `json:"createdAt"`
	UpdatedAt         time.Time `json:"updatedAt"`
}

type AuctionListing struct {
	ID                    int64                `json:"id"`
	SellerID              int64                `json:"sellerId"`
	SellerUsername        string               `json:"sellerUsername,omitempty"`
	ListingKind           string               `json:"listingKind"`
	ItemID                string               `json:"itemId"`
	InstanceID            *int64               `json:"instanceId,omitempty"`
	InstanceCustomData    ItemCustomization    `json:"instanceCustomData,omitempty"`
	FishData              *FishInstanceData    `json:"fishData,omitempty"`
	InstanceEnhanceLevel  *int64               `json:"instanceEnhanceLevel,omitempty"`
	InstanceBaseSnapshot  map[string]any       `json:"instanceBaseSnapshot,omitempty"`
	InstanceAffixes       []EquipmentAffixSlot `json:"instanceAffixes,omitempty"`
	InstanceDurability    *int64               `json:"instanceDurability,omitempty"`
	InstanceMaxDurability *int64               `json:"instanceMaxDurability,omitempty"`
	Quantity              int64                `json:"quantity"`
	StartPrice            int64                `json:"startPrice"`
	CurrentPrice          int64                `json:"currentPrice"`
	MinBidIncrement       int64                `json:"minBidIncrement"`
	MinNextBid            int64                `json:"minNextBid"`
	CurrentBidderID       *int64               `json:"currentBidderId,omitempty"`
	CurrentBidderUsername string               `json:"currentBidderUsername,omitempty"`
	BuyoutPrice           *int64               `json:"buyoutPrice,omitempty"`
	FeeRateBps            int64                `json:"feeRateBps"`
	FeeAmount             int64                `json:"feeAmount"`
	SellerAmount          int64                `json:"sellerAmount"`
	AntiSnipeExtendCount  int                  `json:"antiSnipeExtendCount"`
	Status                string               `json:"status"`
	EndsAt                time.Time            `json:"endsAt"`
	CreatedAt             time.Time            `json:"createdAt"`
	UpdatedAt             time.Time            `json:"updatedAt"`
}

type AuctionBid struct {
	ID             int64     `json:"id"`
	AuctionID      int64     `json:"auctionId"`
	BidderID       int64     `json:"bidderId"`
	BidderUsername string    `json:"bidderUsername,omitempty"`
	BidAmount      int64     `json:"bidAmount"`
	CreatedAt      time.Time `json:"createdAt"`
}

type AuctionBidResult struct {
	AuctionID        int64  `json:"auctionId"`
	SellerID         int64  `json:"sellerId"`
	BidderID         int64  `json:"bidderId"`
	PreviousBidderID *int64 `json:"previousBidderId,omitempty"`
	BuyoutFinalized  bool   `json:"buyoutFinalized"`
}

func New(db *pgxpool.Pool, cfg RuntimeConfig, gameData catalog.Data) *Store {
	if cfg.SkillForge.CompilerVersion <= 0 {
		cfg.SkillForge = config.DefaultGlobalSettings().SkillForge
	}
	if cfg.WarehouseDefaultCols <= 0 {
		cfg.WarehouseDefaultCols = 25
	}
	if cfg.WarehouseDefaultRows <= 0 {
		cfg.WarehouseDefaultRows = 10
	}
	if cfg.WarehouseLogDefaultLookback <= 0 {
		cfg.WarehouseLogDefaultLookback = 7 * 24 * time.Hour
	}
	if cfg.OfflineProgressMaxDuration <= 0 {
		cfg.OfflineProgressMaxDuration = 24 * time.Hour
	}
	if cfg.OfflineSettleExecBudget <= 0 {
		cfg.OfflineSettleExecBudget = 2 * time.Second
	}
	if cfg.ActionQueueFastForwardMaxSteps <= 0 {
		cfg.ActionQueueFastForwardMaxSteps = defaultActionQueueFastForwardMaxSteps
	}
	if cfg.ActionQueueFastForwardMaxExec <= 0 {
		cfg.ActionQueueFastForwardMaxExec = defaultActionQueueFastForwardMaxExec
	}
	if cfg.ActionQueueLaneLimit <= 0 {
		cfg.ActionQueueLaneLimit = 30
	}
	if cfg.KittyActionQueueLaneLimit <= 0 {
		cfg.KittyActionQueueLaneLimit = 20
	}
	if cfg.KittyStatusExpMultiplier < 0 {
		cfg.KittyStatusExpMultiplier = 0
	}
	if cfg.KittyBattleAttrExpMultiplier < 0 {
		cfg.KittyBattleAttrExpMultiplier = 0
	}
	if cfg.KittyBattleSkillExpMultiplier < 0 {
		cfg.KittyBattleSkillExpMultiplier = 0
	}
	if len(cfg.KittyBattleEquipSlots) == 0 {
		cfg.KittyBattleEquipSlots = []string{
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
		}
	}
	if cfg.KittyPersonalityProfiles == nil {
		cfg.KittyPersonalityProfiles = map[string]KittyPersonalityProfile{}
	}
	if strings.TrimSpace(cfg.KittyRemovePersonalityItemID) == "" {
		cfg.KittyRemovePersonalityItemID = "gold_coin"
	}
	if cfg.KittyRemovePersonalityCount <= 0 {
		cfg.KittyRemovePersonalityCount = 1
	}
	if cfg.AuctionFeeRateBps < 0 || cfg.AuctionFeeRateBps > 10_000 {
		cfg.AuctionFeeRateBps = 500
	}
	if cfg.AuctionMinBidIncrement <= 0 {
		cfg.AuctionMinBidIncrement = 1
	}
	if cfg.AuctionAntiSnipeTriggerSeconds < 0 {
		cfg.AuctionAntiSnipeTriggerSeconds = 30 * 60
	}
	if cfg.AuctionAntiSnipeExtendSeconds < 0 {
		cfg.AuctionAntiSnipeExtendSeconds = 10 * 60
	}
	if cfg.AuctionMinRepLevel < 0 {
		cfg.AuctionMinRepLevel = 10
	}
	if cfg.ItemRenameMaxNameRunes <= 0 {
		cfg.ItemRenameMaxNameRunes = 16
	}
	if len(cfg.ItemRenameCost) == 0 {
		cfg.ItemRenameCost = map[string]int64{"harmonic_embryo": 1}
	} else {
		cost := make(map[string]int64, len(cfg.ItemRenameCost))
		for itemID, count := range cfg.ItemRenameCost {
			itemID = strings.TrimSpace(itemID)
			if itemID == "" || count <= 0 {
				continue
			}
			cost[itemID] += count
		}
		if len(cost) == 0 {
			cost["harmonic_embryo"] = 1
		}
		cfg.ItemRenameCost = cost
	}
	cfg.ItemRenameLabelColors = normalizeItemRenameLabelColors(cfg.ItemRenameLabelColors)
	cfg.PlayerUsernameRenameCostItemID = strings.TrimSpace(cfg.PlayerUsernameRenameCostItemID)
	if cfg.ActionEnhanceInheritanceRatio < 0 || cfg.ActionEnhanceInheritanceRatio > 1 {
		cfg.ActionEnhanceInheritanceRatio = 0.5
	}
	if cfg.Alchemy.Inherit.BaseInheritSlots <= 0 {
		cfg.Alchemy.Inherit.BaseInheritSlots = 3
	}
	if cfg.Alchemy.Inherit.BaseSuppressSlots < 0 {
		cfg.Alchemy.Inherit.BaseSuppressSlots = 0
	}
	if cfg.Alchemy.Inherit.InheritSlotPrecisionScale < 0 {
		cfg.Alchemy.Inherit.InheritSlotPrecisionScale = 1
	}
	if cfg.Alchemy.Inherit.SuppressSlotStabilityBonus < 0 {
		cfg.Alchemy.Inherit.SuppressSlotStabilityBonus = 1
	}
	if cfg.Alchemy.Inherit.PositiveIgnoreKeepChanceBase < 0 || cfg.Alchemy.Inherit.PositiveIgnoreKeepChanceBase > 1 {
		cfg.Alchemy.Inherit.PositiveIgnoreKeepChanceBase = 0.4
	}
	if cfg.Alchemy.Inherit.NegativeIgnoreKeepChanceBase < 0 || cfg.Alchemy.Inherit.NegativeIgnoreKeepChanceBase > 1 {
		cfg.Alchemy.Inherit.NegativeIgnoreKeepChanceBase = 1
	}
	if cfg.Alchemy.Inherit.HybridMergeDivisor < 1 {
		cfg.Alchemy.Inherit.HybridMergeDivisor = 1.9
	}
	if cfg.Alchemy.Inherit.InbreedMutationChance < 0 || cfg.Alchemy.Inherit.InbreedMutationChance > 1 {
		cfg.Alchemy.Inherit.InbreedMutationChance = 0.8
	}
	if cfg.Alchemy.Inherit.InbreedGoodChance < 0 || cfg.Alchemy.Inherit.InbreedGoodChance > 1 {
		cfg.Alchemy.Inherit.InbreedGoodChance = 0.05
	}
	if cfg.Alchemy.Inherit.InbreedGoodMultiplierRange.Min <= 0 {
		cfg.Alchemy.Inherit.InbreedGoodMultiplierRange = catalog.AlchemyInheritMultiplierRange{Min: 1.1, Max: 1.35}
	}
	if cfg.Alchemy.Inherit.InbreedBadMultiplierRange.Min <= 0 {
		cfg.Alchemy.Inherit.InbreedBadMultiplierRange = catalog.AlchemyInheritMultiplierRange{Min: 0.35, Max: 0.8}
	}
	if cfg.Alchemy.Inherit.MutationChance < 0 || cfg.Alchemy.Inherit.MutationChance > 1 {
		cfg.Alchemy.Inherit.MutationChance = 0.05
	}
	if cfg.Alchemy.Inherit.NumericValueCapMultiplier < 1 {
		cfg.Alchemy.Inherit.NumericValueCapMultiplier = 3
	}
	if cfg.SkillLevelUpBaseExp <= 0 {
		cfg.SkillLevelUpBaseExp = 100
	}
	if cfg.SkillLevelUpPerLevelExtraExp <= 0 {
		cfg.SkillLevelUpPerLevelExtraExp = 100
	}
	if cfg.ItemRecycleGoldCoefficient <= 0 || math.IsNaN(cfg.ItemRecycleGoldCoefficient) || math.IsInf(cfg.ItemRecycleGoldCoefficient, 0) {
		cfg.ItemRecycleGoldCoefficient = 0.5
	}
	if cfg.ItemRecycleAffixMismatchExtraCoefficient <= 0 || math.IsNaN(cfg.ItemRecycleAffixMismatchExtraCoefficient) || math.IsInf(cfg.ItemRecycleAffixMismatchExtraCoefficient, 0) {
		cfg.ItemRecycleAffixMismatchExtraCoefficient = 0.6
	}
	if cfg.ItemRecycleAffixMismatchExtraCoefficient > 1 {
		cfg.ItemRecycleAffixMismatchExtraCoefficient = 1
	}
	if cfg.ItemRecycleGoldCoefficient+cfg.ItemRecycleAffixMismatchExtraCoefficient <= 1 {
		cfg.ItemRecycleAffixMismatchExtraCoefficient = math.Min(1, 1-cfg.ItemRecycleGoldCoefficient+0.01)
	}
	if cfg.Battle.ExpBonusMaxMultiplier < 1 {
		cfg.Battle.ExpBonusMaxMultiplier = 1
	}
	if len(cfg.DerivedStatsFormula.Formulas) == 0 {
		cfg.DerivedStatsFormula = defaultDerivedStatsFormulaConfig()
	} else {
		cfg.DerivedStatsFormula = normalizeDerivedStatsFormulaConfig(cfg.DerivedStatsFormula)
	}
	enhanceCache, err := buildEnhanceRuntimeCache(gameData)
	if err != nil {
		panic(err)
	}
	attributeExpProfile, err := buildAttributeExpProfileResolver(gameData)
	if err != nil {
		panic(err)
	}
	achievementDefs := buildAchievementDefinitions(gameData)
	if len(gameData.CatteryFurniture) > 32 {
		panic("furniture catalog exceeds source bound")
	}
	furnitureIDs := map[string]bool{}
	for _, definition := range gameData.CatteryFurniture {
		if err := definition.Validate(); err != nil {
			panic(err)
		}
		if furnitureIDs[definition.ID] {
			panic("duplicate furniture definition")
		}
		furnitureIDs[definition.ID] = true
		costSets := []map[string]int64{definition.InstallCosts, definition.RefillCosts}
		for _, level := range definition.Levels {
			costSets = append(costSets, level.UpgradeCosts, level.RefillCostAdd)
		}
		for _, costs := range costSets {
			for id := range costs {
				item := gameData.ItemByID(id)
				if item == nil || item.Disabled || !item.Stackable || item.IsEquipment {
					panic("furniture cost must reference an enabled stackable material: " + id)
				}
			}
		}
	}
	operationEffectRegistry, operationEffectDefinitions, err := buildOperationEffectRuntime(gameData)
	if err != nil {
		panic(err)
	}
	st := &Store{
		db:                          db,
		config:                      cfg,
		gameData:                    gameData,
		enhanceCache:                enhanceCache,
		attributeExpProfile:         attributeExpProfile,
		auditBuffers:                make(map[pgx.Tx]*inventoryAuditBuffer),
		achievementDefs:             achievementDefs,
		achievementDefsByCounterKey: buildAchievementDefinitionsByCounterKey(achievementDefs),
		operationEffectRegistry:     operationEffectRegistry,
		operationEffectDefinitions:  operationEffectDefinitions,
	}
	if _, err := st.specializationTrees(); err != nil {
		panic(err)
	}
	return st
}

func (s *Store) Health(ctx context.Context) error {
	return s.db.Ping(ctx)
}

func (s *Store) CreateUser(ctx context.Context, username, email, passwordHash, policyVersion string) (int64, error) {
	var id int64
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `
			INSERT INTO users(username, email, password_hash)
			VALUES($1, $2, $3)
			RETURNING id
		`, username, email, passwordHash).Scan(&id); err != nil {
			return err
		}

		if _, err := tx.Exec(ctx, `
			INSERT INTO user_settings(user_id, language, theme, extra_jsonb, created_at, updated_at)
			VALUES($1, 'zh-CN', 'light', jsonb_build_object('simpleMode', false), now(), now())
			ON CONFLICT (user_id) DO NOTHING
		`, id); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO user_policy_acceptances(user_id, policy_key, policy_version, source, ip, accepted_at)
			VALUES($1, 'terms_of_service', $2, 'register', '', now())
			ON CONFLICT (user_id, policy_key, policy_version)
			DO UPDATE SET accepted_at = EXCLUDED.accepted_at, source = EXCLUDED.source
		`, id, policyVersion); err != nil {
			return err
		}

		if _, err := tx.Exec(ctx, `
			INSERT INTO player_item_instances(
				user_id, template_item_id, enhance_level, roll_seed, base_snapshot, affixes,
				durability, max_durability, quantity, state, created_at, updated_at
			)
			VALUES
				($1, 'gold_coin', 0, 0, '{}'::jsonb, '[]'::jsonb, 100, 100, 100, 'inventory', now(), now())
		`, id); err != nil {
			return err
		}
		basicSkillID := s.defaultBattleBasicSkillID()
		if basicSkillID != "" {
			if _, err := tx.Exec(ctx, `
				INSERT INTO player_skills(
					user_id, skill_id, enabled, cast_order,
					rule_enabled, rule_groups_jsonb, level, exp, updated_at
				)
				VALUES
					($1, $2, false, 0, false, '[]'::jsonb, 1, 0, now())
				ON CONFLICT (user_id, skill_id)
				DO UPDATE SET updated_at = now()
			`, id, basicSkillID); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO player_derived_stats_cache(user_id, stats_jsonb, version, updated_at)
			VALUES($1, '{}'::jsonb, 1, now())
			ON CONFLICT (user_id) DO NOTHING
		`, id); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO player_item_instances(user_id, template_item_id, enhance_level, roll_seed, base_snapshot, affixes, state, created_at, updated_at)
			VALUES($1, 'wood_sword', 0, 0, '{}'::jsonb, '[]'::jsonb, 'inventory', now(), now())
		`, id); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO player_cat_village_task_state(user_id, next_refresh_at, updated_at)
			VALUES($1, now(), now())
			ON CONFLICT (user_id) DO NOTHING
		`, id); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO player_presence(user_id, status, last_seen_at, session_version, updated_at)
			VALUES($1, 'offline', now(), 1, now())
			ON CONFLICT (user_id) DO NOTHING
		`, id); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			if strings.Contains(pgErr.ConstraintName, "email") {
				return 0, ErrDuplicateEmail
			}
			return 0, ErrDuplicateUser
		}
		return 0, err
	}
	return id, nil
}

func (s *Store) GetUserAuth(ctx context.Context, username string) (int64, string, error) {
	var id int64
	var hash string
	err := s.db.QueryRow(ctx, `
		SELECT id, password_hash FROM users WHERE username = $1
	`, username).Scan(&id, &hash)
	return id, hash, err
}

type UserAuthState struct {
	UserID         int64
	PasswordHash   string
	BanReason      string
	BanUntil       *time.Time
	BanIsPermanent bool
	EmailVerified  bool
	Email          string
}

func isUserBanActive(reason string, banUntil *time.Time, isPermanent bool, now time.Time) bool {
	if strings.TrimSpace(reason) == "" {
		return false
	}
	if isPermanent {
		return true
	}
	return banUntil != nil && banUntil.After(now)
}

func (s *Store) IsUserBanned(ctx context.Context, userID int64) (bool, error) {
	if userID <= 0 {
		return false, ErrUserNotFound
	}
	var reason string
	var banUntil *time.Time
	var isPermanent bool
	err := s.db.QueryRow(ctx, `
		SELECT ban_reason, ban_until, ban_is_permanent
		FROM users
		WHERE id = $1
	`, userID).Scan(&reason, &banUntil, &isPermanent)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, ErrUserNotFound
	}
	if err != nil {
		return false, err
	}
	return isUserBanActive(reason, banUntil, isPermanent, time.Now().UTC()), nil
}

// Keep ban updates serialized without conflicting with foreign-key key-share
// locks taken by risk graph inserts that reference the same user.
const assertUserNotBannedTxSQL = `
	SELECT ban_reason, ban_until, ban_is_permanent
	FROM users
	WHERE id = $1
	FOR NO KEY UPDATE
`

func (s *Store) assertUserNotBannedTx(ctx context.Context, tx pgx.Tx, userID int64) error {
	var reason string
	var banUntil *time.Time
	var isPermanent bool
	err := tx.QueryRow(ctx, assertUserNotBannedTxSQL, userID).Scan(&reason, &banUntil, &isPermanent)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrUserNotFound
	}
	if err != nil {
		return err
	}
	if isUserBanActive(reason, banUntil, isPermanent, time.Now().UTC()) {
		return ErrUserBanned
	}
	return nil
}

func (s *Store) GetUserAuthState(ctx context.Context, username string) (UserAuthState, error) {
	var state UserAuthState
	err := s.db.QueryRow(ctx, `
		SELECT id, password_hash, ban_reason, ban_until, ban_is_permanent, email_verified, email
		FROM users
		WHERE username = $1
	`, username).Scan(&state.UserID, &state.PasswordHash, &state.BanReason, &state.BanUntil, &state.BanIsPermanent, &state.EmailVerified, &state.Email)
	if err != nil {
		return UserAuthState{}, err
	}
	state.BanReason = strings.TrimSpace(state.BanReason)
	state.Email = strings.TrimSpace(state.Email)
	return state, nil
}

func (s *Store) GetUserIDByUsername(ctx context.Context, username string) (int64, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return 0, ErrUserNotFound
	}
	var userID int64
	err := s.db.QueryRow(ctx, `
		SELECT id
		FROM users
		WHERE username = $1
	`, username).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrUserNotFound
	}
	if err != nil {
		return 0, err
	}
	return userID, nil
}

func (s *Store) GetUsernameByUserID(ctx context.Context, userID int64) (string, error) {
	var username string
	err := s.db.QueryRow(ctx, `
		SELECT username
		FROM users
		WHERE id = $1
	`, userID).Scan(&username)
	return strings.TrimSpace(username), err
}

type AdminUserBanInput struct {
	UserID      int64
	Reason      string
	BanUntil    *time.Time
	IsPermanent bool
}

func (s *Store) AdminBanUser(ctx context.Context, in AdminUserBanInput) error {
	if in.UserID <= 0 {
		return ErrUserNotFound
	}
	banTryCleanup(ctx, func() error { return s.cleanupBannedUserListings(ctx, in.UserID) })
	banTryCleanup(ctx, func() error { return s.cleanupBannedUserBuyOrders(ctx, in.UserID) })
	banTryCleanup(ctx, func() error { return s.cleanupBannedUserAuctions(ctx, in.UserID) })
	banTryCleanup(ctx, func() error { return s.cleanupBannedUserKittyCommissions(ctx, in.UserID) })
	banTryCleanup(ctx, func() error { return s.cleanupBannedUserRuntime(ctx, in.UserID) })
	reason := strings.TrimSpace(in.Reason)
	return s.withTx(ctx, func(tx pgx.Tx) error {
		var existsUserID int64
		if err := tx.QueryRow(ctx, `
			SELECT id
			FROM users
			WHERE id = $1
			FOR UPDATE
		`, in.UserID).Scan(&existsUserID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrUserNotFound
			}
			return err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO player_presence(user_id, status, last_seen_at, session_version, updated_at)
			VALUES($1, 'offline', now(), 1, now())
			ON CONFLICT (user_id)
			DO UPDATE SET session_version = player_presence.session_version + 1, updated_at = now()
		`, in.UserID); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `
			UPDATE users
			SET ban_reason = $2,
			    ban_until = $3,
			    ban_is_permanent = $4
			WHERE id = $1
		`, in.UserID, reason, in.BanUntil, in.IsPermanent)
		return err
	})
}

func banTryCleanup(ctx context.Context, fn func() error) {
	_ = fn()
}

func (s *Store) cleanupBannedUserListings(ctx context.Context, userID int64) error {
	return s.withTx(ctx, func(tx pgx.Tx) error {
		return s.cleanupBannedUserListingsTx(ctx, tx, userID)
	})
}

func (s *Store) cleanupBannedUserBuyOrders(ctx context.Context, userID int64) error {
	return s.withTx(ctx, func(tx pgx.Tx) error {
		return s.cleanupBannedUserBuyOrdersTx(ctx, tx, userID)
	})
}

func (s *Store) cleanupBannedUserAuctions(ctx context.Context, userID int64) error {
	return s.withTx(ctx, func(tx pgx.Tx) error {
		return s.cleanupBannedUserAuctionsTx(ctx, tx, userID)
	})
}

func (s *Store) cleanupBannedUserRuntime(ctx context.Context, userID int64) error {
	return s.withTx(ctx, func(tx pgx.Tx) error {
		return s.cleanupBannedUserRuntimeStateTx(ctx, tx, userID)
	})
}

func (s *Store) cleanupBannedUserRuntimeStateTx(ctx context.Context, tx pgx.Tx, userID int64) error {
	if _, err := tx.Exec(ctx, `
		DELETE FROM player_action_queue
		WHERE user_id = $1
		  AND status IN ('pending', 'running')
	`, userID); err != nil {
		return err
	}

	var roomID int64
	var ownerID int64
	var status string
	var currentRunID *int64
	err := tx.QueryRow(ctx, `
		SELECT m.room_id, r.owner_id, r.status, r.current_run_id
		FROM battle_room_members m
		JOIN battle_rooms r ON r.id = m.room_id
		WHERE m.user_id = $1
		FOR UPDATE OF m, r
	`, userID).Scan(&roomID, &ownerID, &status, &currentRunID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}

	if _, err := tx.Exec(ctx, `
		DELETE FROM battle_room_members
		WHERE room_id = $1 AND user_id = $2
	`, roomID, userID); err != nil {
		return err
	}

	var memberCount int
	if err := tx.QueryRow(ctx, `
		SELECT COUNT(1)
		FROM battle_room_members
		WHERE room_id = $1
	`, roomID).Scan(&memberCount); err != nil {
		return err
	}
	if memberCount <= 0 {
		_, err = tx.Exec(ctx, `DELETE FROM battle_rooms WHERE id = $1`, roomID)
		return err
	}

	nextOwnerID := ownerID
	if ownerID == userID {
		if err := tx.QueryRow(ctx, `
			SELECT user_id
			FROM battle_room_members
			WHERE room_id = $1
			ORDER BY joined_at ASC, user_id ASC
			LIMIT 1
		`, roomID).Scan(&nextOwnerID); err != nil {
			return err
		}
	}

	normalizedStatus := strings.TrimSpace(strings.ToLower(status))
	if normalizedStatus == BattleRoomStatusEntering || normalizedStatus == BattleRoomStatusPlaying || normalizedStatus == BattleRoomStatusRecovering {
		if currentRunID != nil && *currentRunID > 0 {
			if _, err := tx.Exec(ctx, `
				UPDATE battle_runs
				SET status = $2,
				    ended_at = COALESCE(ended_at, now()),
				    updated_at = now()
				WHERE id = $1
				  AND status IN ($3, $4, $5)
			`, *currentRunID, BattleRunStatusInterrupted, BattleRunStatusCompiled, BattleRunStatusRunning, BattleRunStatusSettling); err != nil {
				return err
			}
		}
		_, err = tx.Exec(ctx, `
			UPDATE battle_rooms
			SET owner_id = $2,
			    status = $3,
			    current_run_id = NULL,
			    entering_started_at = NULL,
			    recovering_started_at = NULL,
			    recovery_ends_at = NULL,
			    owner_worker_id = '',
			    lease_until = NULL,
			    next_tick_at = now(),
			    phase = '',
			    schema_version = CASE WHEN $4 = true AND schema_version < 2 THEN 2 ELSE schema_version END,
			    updated_at = now()
			WHERE id = $1
		`, roomID, nextOwnerID, BattleRoomStatusPreparing, s.battleStateMachineV2)
		return err
	}

	_, err = tx.Exec(ctx, `
		UPDATE battle_rooms
		SET owner_id = $2,
		    updated_at = now()
		WHERE id = $1
	`, roomID, nextOwnerID)
	return err
}

func (s *Store) cleanupBannedUserMarketStateTx(ctx context.Context, tx pgx.Tx, userID int64) error {
	if err := s.cleanupBannedUserListingsTx(ctx, tx, userID); err != nil {
		return err
	}
	if err := s.cleanupBannedUserBuyOrdersTx(ctx, tx, userID); err != nil {
		return err
	}
	return s.cleanupBannedUserAuctionsTx(ctx, tx, userID)
}

func (s *Store) cleanupBannedUserListingsTx(ctx context.Context, tx pgx.Tx, userID int64) error {
	rows, err := tx.Query(ctx, `
		SELECT id, item_id, listing_kind, instance_id, quantity
		FROM market_listings
		WHERE seller_id = $1
		  AND status = 'active'
		FOR UPDATE
	`, userID)
	if err != nil {
		return err
	}
	defer rows.Close()

	type listingRow struct {
		id         int64
		itemID     string
		kind       string
		instanceID *int64
		quantity   int64
	}
	listings := make([]listingRow, 0, 8)
	for rows.Next() {
		var row listingRow
		if err := rows.Scan(&row.id, &row.itemID, &row.kind, &row.instanceID, &row.quantity); err != nil {
			return err
		}
		listings = append(listings, row)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, listing := range listings {
		if strings.TrimSpace(strings.ToLower(listing.kind)) == "instance" {
			if listing.instanceID == nil || *listing.instanceID <= 0 {
				return ErrInvalidInstanceState
			}
			if err := s.LedgerSetStateTx(ctx, tx, *listing.instanceID, userID, "market_listing", "inventory", listing.itemID, 1, InventoryLedgerMeta{
				EventType:     WarehouseChangeEventSystemToWarehouse,
				SourceSurface: "market_listing",
				TargetSurface: "inventory",
				Details: map[string]any{
					"type":      "market_listing_ban_cancel",
					"listingId": listing.id,
				},
			}); err != nil {
				return err
			}
		} else if listing.quantity > 0 {
			if err := s.LedgerGrantTemplateTx(ctx, tx, userID, listing.itemID, listing.quantity, InventoryLedgerMeta{
				SourceSurface: "market_listing",
				TargetSurface: "inventory",
				Details: map[string]any{
					"type":      "market_listing_ban_cancel",
					"listingId": listing.id,
				},
			}); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(ctx, `
			UPDATE market_listings
			SET status = 'cancelled'
			WHERE id = $1
		`, listing.id); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) cleanupBannedUserBuyOrdersTx(ctx context.Context, tx pgx.Tx, userID int64) error {
	rows, err := tx.Query(ctx, `
		SELECT id, item_id, price, remaining_quantity
		FROM market_buy_orders
		WHERE buyer_id = $1
		  AND status = 'active'
		FOR UPDATE
	`, userID)
	if err != nil {
		return err
	}
	defer rows.Close()

	type buyOrderRow struct {
		id        int64
		itemID    string
		price     int64
		remaining int64
	}
	orders := make([]buyOrderRow, 0, 8)
	for rows.Next() {
		var row buyOrderRow
		if err := rows.Scan(&row.id, &row.itemID, &row.price, &row.remaining); err != nil {
			return err
		}
		orders = append(orders, row)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, order := range orders {
		if order.remaining > 0 {
			refund, mulErr := checkedMulInt64(order.price, order.remaining)
			if mulErr != nil {
				return mulErr
			}
			if refund > 0 {
				if err := s.LedgerGrantTemplateTx(ctx, tx, userID, GoldCoinItemID, refund, InventoryLedgerMeta{
					SourceSurface: "market_buy_order",
					TargetSurface: "inventory",
					Details: map[string]any{
						"type":    "market_buy_order_ban_cancel_refund",
						"orderId": order.id,
						"itemId":  order.itemID,
					},
				}); err != nil {
					return err
				}
			}
		}
		if _, err := tx.Exec(ctx, `
			UPDATE market_buy_orders
			SET status = 'cancelled',
			    remaining_quantity = 0,
			    updated_at = now()
			WHERE id = $1
		`, order.id); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) cleanupBannedUserAuctionsTx(ctx context.Context, tx pgx.Tx, userID int64) error {
	rows, err := tx.Query(ctx, `
		SELECT id, item_id, listing_kind, instance_id, quantity, current_bidder_id, current_price
		FROM market_auctions
		WHERE seller_id = $1
		  AND status = 'active'
		FOR UPDATE
	`, userID)
	if err != nil {
		return err
	}
	defer rows.Close()

	type auctionRow struct {
		id              int64
		itemID          string
		listingKind     string
		instanceID      *int64
		quantity        int64
		currentBidderID *int64
		currentPrice    int64
	}
	auctions := make([]auctionRow, 0, 4)
	for rows.Next() {
		var row auctionRow
		if err := rows.Scan(&row.id, &row.itemID, &row.listingKind, &row.instanceID, &row.quantity, &row.currentBidderID, &row.currentPrice); err != nil {
			return err
		}
		auctions = append(auctions, row)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, auction := range auctions {
		if auction.currentBidderID != nil && *auction.currentBidderID > 0 && auction.currentPrice > 0 {
			if err := s.LedgerGrantTemplateTx(ctx, tx, *auction.currentBidderID, GoldCoinItemID, auction.currentPrice, InventoryLedgerMeta{
				SourceSurface: "auction_bid_hold",
				TargetSurface: "inventory",
				Details: map[string]any{
					"auctionId": auction.id,
					"type":      "auction_ban_cancel_refund",
				},
			}); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(ctx, `
			UPDATE market_auctions
			SET status = 'cancelled',
			    instance_id = NULL,
			    fee_rate_bps = 0,
			    fee_amount = 0,
			    seller_amount = 0,
			    updated_at = now()
			WHERE id = $1
		`, auction.id); err != nil {
			return err
		}

		if strings.TrimSpace(strings.ToLower(auction.listingKind)) == "instance" {
			if auction.instanceID == nil || *auction.instanceID <= 0 {
				return ErrInvalidInstanceState
			}
			if err := s.LedgerSetStateTx(ctx, tx, *auction.instanceID, userID, "auction", "inventory", auction.itemID, 1, InventoryLedgerMeta{
				EventType:     WarehouseChangeEventSystemToWarehouse,
				SourceSurface: "auction",
				TargetSurface: "inventory",
				Details: map[string]any{
					"auctionId": auction.id,
					"type":      "auction_ban_cancel_return",
				},
			}); err != nil {
				return err
			}
		} else {
			quantity := auction.quantity
			if quantity <= 0 {
				quantity = 1
			}
			if err := s.LedgerGrantTemplateTx(ctx, tx, userID, auction.itemID, quantity, InventoryLedgerMeta{
				SourceSurface: "auction",
				TargetSurface: "inventory",
				Details: map[string]any{
					"auctionId": auction.id,
					"type":      "auction_ban_cancel_return_stack",
				},
			}); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Store) cleanupBannedUserKittyCommissions(ctx context.Context, userID int64) error {
	return s.withTx(ctx, func(tx pgx.Tx) error {
		type commissionRow struct {
			id      int64
			kittyID int64
			costs   map[string]int64
		}
		rows, err := tx.Query(ctx, `
			SELECT id, kitty_id, costs_jsonb
			FROM player_kitty_commissions
			WHERE user_id = $1 AND status = 'running'
			FOR UPDATE
		`, userID)
		if err != nil {
			return err
		}
		defer rows.Close()
		var commissions []commissionRow
		for rows.Next() {
			var row commissionRow
			var costsRaw []byte
			if err := rows.Scan(&row.id, &row.kittyID, &costsRaw); err != nil {
				return err
			}
			if err := json.Unmarshal(costsRaw, &row.costs); err != nil {
				return err
			}
			commissions = append(commissions, row)
		}
		if err := rows.Err(); err != nil {
			return err
		}
		for _, c := range commissions {
			for itemID, qty := range c.costs {
				if itemID == "" || qty <= 0 {
					continue
				}
				if err := s.LedgerGrantTemplateTx(ctx, tx, userID, itemID, qty, InventoryLedgerMeta{
					SourceSurface: "kitty_commission",
					TargetSurface: "inventory",
					Details: map[string]any{
						"type":    "ban_commission_refund",
						"kittyId": c.kittyID,
					},
				}); err != nil {
					return err
				}
			}
			if _, err := tx.Exec(ctx, `
				UPDATE player_kitty_commissions
				SET status = 'cancelled'
				WHERE id = $1
			`, c.id); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `
				UPDATE player_kitties
				SET status = $3, updated_at = now()
				WHERE id = $1 AND user_id = $2 AND status = $4
			`, c.kittyID, userID, KittyStatusIdle, KittyStatusInLifeQueue); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *Store) AdminUnbanUser(ctx context.Context, userID int64) error {
	if userID <= 0 {
		return ErrUserNotFound
	}
	tag, err := s.db.Exec(ctx, `
		UPDATE users
		SET ban_reason = '',
		    ban_until = NULL,
		    ban_is_permanent = FALSE
		WHERE id = $1
	`, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrUserNotFound
	}
	return nil
}

func (s *Store) RotateSessionVersion(ctx context.Context, userID int64) (int64, error) {
	var sessionVersion int64
	err := s.db.QueryRow(ctx, `
		INSERT INTO player_presence(user_id, status, last_seen_at, session_version, updated_at)
		VALUES($1, 'offline', now(), 1, now())
		ON CONFLICT (user_id)
		DO UPDATE SET session_version = player_presence.session_version + 1, updated_at = now()
		RETURNING session_version
	`, userID).Scan(&sessionVersion)
	return sessionVersion, err
}

// CheckIPTrust returns (ipCount, hasAnyIP, error).
// ipCount: how many times userID has logged in from ip (0 = never).
// hasAnyIP: whether userID has any recorded IPs at all.
func (s *Store) CheckIPTrust(ctx context.Context, userID int64, ip string) (int, bool, error) {
	var ipCount int
	var totalIPs int
	err := s.db.QueryRow(ctx, `
		SELECT
			COALESCE(SUM(CASE WHEN ip = $2 THEN use_count ELSE 0 END), 0),
			COUNT(1)
		FROM user_known_ips
		WHERE user_id = $1
	`, userID, strings.TrimSpace(ip)).Scan(&ipCount, &totalIPs)
	if err != nil {
		return 0, false, err
	}
	return ipCount, totalIPs > 0, nil
}

func (s *Store) RecordIPLogin(ctx context.Context, userID int64, ip string) error {
	_, err := s.db.Exec(ctx, `
		INSERT INTO user_known_ips (user_id, ip, use_count, last_seen_at)
		VALUES ($1, $2, 1, NOW())
		ON CONFLICT (user_id, ip) DO UPDATE
		  SET use_count    = user_known_ips.use_count + 1,
		      last_seen_at = NOW()
	`, userID, strings.TrimSpace(ip))
	return err
}

func (s *Store) IsTrustedDevice(ctx context.Context, userID int64, deviceHash string) (bool, error) {
	var count int
	err := s.db.QueryRow(ctx, `
		SELECT COUNT(1) FROM user_trusted_devices
		WHERE user_id = $1 AND device_hash = $2 AND expires_at > NOW()
	`, userID, deviceHash).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (s *Store) TrustDevice(ctx context.Context, userID int64, deviceHash string, ttl time.Duration) error {
	seconds := int64(ttl.Seconds())
	_, err := s.db.Exec(ctx, `
		INSERT INTO user_trusted_devices (user_id, device_hash, last_seen_at, expires_at)
		VALUES ($1, $2, NOW(), NOW() + ($3 * interval '1 second'))
		ON CONFLICT (user_id, device_hash) DO UPDATE
		  SET last_seen_at = NOW(),
		      expires_at   = NOW() + ($3 * interval '1 second')
	`, userID, deviceHash, seconds)
	return err
}

func (s *Store) SessionVersionValid(ctx context.Context, userID, tokenSessionVersion int64) (bool, error) {
	if tokenSessionVersion <= 0 {
		return false, nil
	}
	var sessionVersion int64
	err := s.db.QueryRow(ctx, `
		SELECT session_version
		FROM player_presence
		WHERE user_id = $1
	`, userID).Scan(&sessionVersion)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return sessionVersion == tokenSessionVersion, nil
}

func (s *Store) SetPresenceStatus(ctx context.Context, userID int64, status string) error {
	return s.SetPresenceStatusAt(ctx, userID, status, time.Now())
}

func (s *Store) SetPresenceStatusAt(ctx context.Context, userID int64, status string, at time.Time) error {
	if at.IsZero() {
		at = time.Now()
	}
	_, err := s.db.Exec(ctx, `
		INSERT INTO player_presence(user_id, status, last_seen_at, session_version, updated_at)
		VALUES($1, $2, $3, 1, now())
		ON CONFLICT (user_id)
		DO UPDATE SET status = EXCLUDED.status, last_seen_at = EXCLUDED.last_seen_at, updated_at = now()
	`, userID, status, at)
	return err
}

func (s *Store) TouchPresence(ctx context.Context, userID int64) error {
	_, err := s.db.Exec(ctx, `
		INSERT INTO player_presence(user_id, status, last_seen_at, session_version, updated_at)
		VALUES($1, 'online', now(), 1, now())
		ON CONFLICT (user_id)
		DO UPDATE SET status = 'online', last_seen_at = now(), updated_at = now()
	`, userID)
	return err
}

type OnlinePresenceRow struct {
	UserID         int64
	SessionVersion int64
	LastSeenAt     time.Time
}

type playerAttributeDefaultRow struct {
	AttrKey       string
	SourcePayload string
	FinalValue    float64
}

// ListOnlinePresenceUsers returns online users after afterUserID using keyset pagination.
func (s *Store) ListOnlinePresenceUsers(ctx context.Context, limit int, afterUserID int64) ([]OnlinePresenceRow, error) {
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	if afterUserID < 0 {
		afterUserID = 0
	}
	rows, err := s.db.Query(ctx, `
		SELECT user_id, session_version, last_seen_at
		FROM player_presence
		WHERE status = 'online'
		  AND user_id > $2
		ORDER BY user_id
		LIMIT $1
	`, limit, afterUserID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]OnlinePresenceRow, 0, limit)
	for rows.Next() {
		var row OnlinePresenceRow
		if err := rows.Scan(&row.UserID, &row.SessionVersion, &row.LastSeenAt); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (s *Store) GetPresence(ctx context.Context, userID int64) (PresenceState, error) {
	var state PresenceState
	err := s.db.QueryRow(ctx, `
		SELECT user_id, status, last_seen_at, session_version, updated_at
		FROM player_presence
		WHERE user_id = $1
	`, userID).Scan(&state.UserID, &state.Status, &state.LastSeenAt, &state.SessionVersion, &state.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return PresenceState{
			UserID:         userID,
			Status:         "offline",
			LastSeenAt:     time.Time{},
			SessionVersion: 0,
			UpdatedAt:      time.Time{},
		}, nil
	}
	return state, err
}

func (s *Store) GetPlayerState(ctx context.Context, userID int64) (PlayerState, error) {
	buildings, err := s.ListPlayerBuildings(ctx, userID)
	if err != nil {
		return PlayerState{}, err
	}
	kitties, err := s.ListPlayerKitties(ctx, userID)
	if err != nil {
		return PlayerState{}, err
	}
	totalRep, err := s.GetCatVillageReputation(ctx, userID)
	if err != nil {
		return PlayerState{}, err
	}
	return PlayerState{
		UserID:               userID,
		Buildings:            buildings,
		Kitties:              kitties,
		CatVillageReputation: ComputeRepLevel(totalRep, s.config.CatVillageRepLevels),
		UpdatedAt:            time.Now(),
	}, nil
}

func (s *Store) EnsurePlayerAttributes(ctx context.Context, userID int64, seeds []PlayerAttributeSeed) error {
	if userID <= 0 || len(seeds) == 0 {
		return nil
	}
	rows, err := playerAttributeDefaultRowsFromSeeds(seeds)
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return nil
	}
	return s.withTx(ctx, func(tx pgx.Tx) error {
		return insertPlayerAttributeDefaultRowsTx(ctx, tx, userID, rows)
	})
}

func playerAttributeDefaultRowsFromSeeds(seeds []PlayerAttributeSeed) ([]playerAttributeDefaultRow, error) {
	rows := make([]playerAttributeDefaultRow, 0, len(seeds))
	seen := make(map[string]struct{}, len(seeds))
	for _, seed := range seeds {
		row, ok, err := playerAttributeDefaultRowFromSeed(strings.TrimSpace(seed.AttrKey), seed.BaseDefault, seed.BaseDefault)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		if _, exists := seen[row.AttrKey]; exists {
			continue
		}
		seen[row.AttrKey] = struct{}{}
		rows = append(rows, row)
	}
	return rows, nil
}

func playerAttributeDefaultRowFromSeed(attrKey string, baseDefault float64, finalValue float64) (playerAttributeDefaultRow, bool, error) {
	attrKey = strings.TrimSpace(attrKey)
	if attrKey == "" {
		return playerAttributeDefaultRow{}, false, nil
	}
	roundedDefault := roundStatValue(baseDefault)
	sourceMods := map[string]any{
		"version": "v2",
		"base": map[string]any{
			"value":       roundedDefault,
			"baseDefault": roundedDefault,
			"levelBonus":  0,
			"excluded":    false,
		},
		"summary": map[string]any{"add": 0, "mul": 1},
	}
	payload, err := json.Marshal(sourceMods)
	if err != nil {
		return playerAttributeDefaultRow{}, false, err
	}
	return playerAttributeDefaultRow{
		AttrKey:       attrKey,
		SourcePayload: string(payload),
		FinalValue:    finalValue,
	}, true, nil
}

func insertPlayerAttributeDefaultRowsTx(ctx context.Context, tx pgx.Tx, userID int64, rows []playerAttributeDefaultRow) error {
	if userID <= 0 || len(rows) == 0 {
		return nil
	}
	sort.SliceStable(rows, func(i, j int) bool {
		return rows[i].AttrKey < rows[j].AttrKey
	})
	attrKeys := make([]string, 0, len(rows))
	sourcePayloads := make([]string, 0, len(rows))
	finalValues := make([]float64, 0, len(rows))
	for _, row := range rows {
		attrKey := strings.TrimSpace(row.AttrKey)
		if attrKey == "" {
			continue
		}
		attrKeys = append(attrKeys, attrKey)
		sourcePayloads = append(sourcePayloads, row.SourcePayload)
		finalValues = append(finalValues, row.FinalValue)
	}
	if len(attrKeys) == 0 {
		return nil
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO player_attributes(
			user_id, attr_key, base_level, skill_point_level, attr_exp, source_mods_jsonb, final_value, lock_version, updated_at
		)
		SELECT $1, seed.attr_key, 1, 0, 0, seed.source_mods::jsonb, seed.final_value, 1, now()
		FROM unnest($2::text[], $3::text[], $4::double precision[]) AS seed(attr_key, source_mods, final_value)
		ON CONFLICT (user_id, attr_key) DO NOTHING
	`, userID, attrKeys, sourcePayloads, finalValues)
	return err
}

func (s *Store) ListPlayerAttributes(ctx context.Context, userID int64, seeds []PlayerAttributeSeed) ([]PlayerAttributeState, error) {
	rows, err := s.db.Query(ctx, `
		SELECT attr_key, base_level, skill_point_level, attr_exp, source_mods_jsonb, final_value, lock_version
		FROM player_attributes
		WHERE user_id = $1
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	seedMap := make(map[string]PlayerAttributeSeed, len(seeds))
	for _, seed := range seeds {
		key := strings.TrimSpace(seed.AttrKey)
		if key == "" {
			continue
		}
		seedMap[key] = seed
	}
	out := make([]PlayerAttributeState, 0, len(seeds))
	for rows.Next() {
		var item PlayerAttributeState
		var raw []byte
		if err := rows.Scan(&item.AttrKey, &item.BaseLevel, &item.SkillPointLevel, &item.AttrExp, &raw, &item.FinalValue, &item.LockVersion); err != nil {
			return nil, err
		}
		item.SourceMods = map[string]any{}
		_ = json.Unmarshal(raw, &item.SourceMods)
		if seed, ok := seedMap[item.AttrKey]; ok {
			item.BaseDefault = seed.BaseDefault
			item.GrowthType = seed.GrowthType
			item.Unit = seed.Unit
			item.Category = seed.Category
			item.Order = seed.Order
			item.NameKey = seed.NameKey
			item.DescKey = seed.DescKey
			item.IconID = seed.IconID
			item.EmojiIcon = seed.EmojiIcon
			item.AllowBattlePointLevelUp = seed.AllowBattlePointLevelUp
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) ApplyIdleTick(ctx context.Context, userID int64, now time.Time) (PlayerState, error) {
	_ = ctx
	return PlayerState{
		UserID:    userID,
		UpdatedAt: now,
	}, nil
}

// Terminal rows remain in the database for audit and domain history, but do not
// belong in the player-facing inventory snapshot.
const listItemInstancesSnapshotSQL = `
	SELECT i.instance_id, i.user_id, i.template_item_id, COALESCE(i.custom_data, '{}'::jsonb), COALESCE(i.fish_data, '{}'::jsonb), i.quantity, i.enhance_level, i.base_snapshot, i.affixes, i.durability, i.max_durability, i.state, i.archived_at, i.created_at, i.updated_at,
	       COALESCE(b.id,''),COALESCE(b.custom_skill_id,''),COALESCE(b.revision_id,''),COALESCE(r.source_skill_id,''),COALESCE(b.status,''),COALESCE(b.target_kitty_id,0),
	       COALESCE(r.ruleset_version,0),COALESCE(r.compiled_skill_jsonb->'nameParams','{}'::jsonb)
	FROM player_item_instances i
	LEFT JOIN player_forged_skillbooks b ON b.instance_id=i.instance_id
	LEFT JOIN player_custom_skill_revisions r ON r.id=b.revision_id
	WHERE i.user_id = $1
	  AND i.state NOT IN ('consumed', 'destroyed')
	ORDER BY i.instance_id DESC
`

func (s *Store) ListItemInstances(ctx context.Context, userID int64) ([]ItemInstance, error) {
	rows, err := s.db.Query(ctx, listItemInstancesSnapshotSQL, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]ItemInstance, 0, 16)
	for rows.Next() {
		var it ItemInstance
		var baseRaw []byte
		var affixRaw []byte
		var customDataRaw []byte
		var fishDataRaw []byte
		var forgedNameParamsRaw []byte
		var forged ForgedSkillbookSummary
		if err := rows.Scan(&it.InstanceID, &it.UserID, &it.TemplateItemID, &customDataRaw, &fishDataRaw, &it.Quantity, &it.EnhanceLevel, &baseRaw, &affixRaw, &it.Durability, &it.MaxDurability, &it.State, &it.ArchivedAt, &it.CreatedAt, &it.UpdatedAt, &forged.ID, &forged.CustomSkillID, &forged.RevisionID, &forged.SourceSkillID, &forged.Status, &forged.TargetKittyID, &forged.RulesetVersion, &forgedNameParamsRaw); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(baseRaw, &it.BaseSnapshot)
		it.CustomData = decodeItemCustomization(customDataRaw)
		if string(fishDataRaw) != "{}" {
			var fish FishInstanceData
			if err := json.Unmarshal(fishDataRaw, &fish); err == nil && strings.TrimSpace(fish.SpeciesID) != "" {
				it.FishData = &fish
			}
		}
		it.Affixes = decodeAffixSlots(affixRaw)
		if forged.ID != "" {
			forged.InstanceID = it.InstanceID
			forged.Usable = forged.Status == "inventory" && s.config.SkillForge.RulesetEnabled(forged.RulesetVersion)
			_ = json.Unmarshal(forgedNameParamsRaw, &forged.NameParams)
			it.ForgedSkillbook = &forged
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

func (s *Store) GetItemInstance(ctx context.Context, userID, instanceID int64) (ItemInstance, string, error) {
	var it ItemInstance
	var baseRaw []byte
	var affixRaw []byte
	var customDataRaw []byte
	var fishDataRaw []byte
	var equippedSlot string
	var forgedRevisionID string
	err := s.db.QueryRow(ctx, `
		SELECT i.instance_id, i.user_id, i.template_item_id, COALESCE(i.custom_data, '{}'::jsonb), COALESCE(i.fish_data, '{}'::jsonb), i.quantity, i.enhance_level, i.base_snapshot, i.affixes, i.durability, i.max_durability, i.state, i.archived_at, i.created_at, i.updated_at,
		       COALESCE((
		       	SELECT string_agg(slot, ',' ORDER BY slot)
		       	FROM player_equip_slots s
		       	WHERE s.user_id = i.user_id AND s.instance_id = i.instance_id
		       ), ''),COALESCE(b.revision_id,'')
		FROM player_item_instances i
		LEFT JOIN player_forged_skillbooks b ON b.instance_id=i.instance_id
		WHERE i.user_id = $1 AND i.instance_id = $2
	`, userID, instanceID).Scan(
		&it.InstanceID,
		&it.UserID,
		&it.TemplateItemID,
		&customDataRaw,
		&fishDataRaw,
		&it.Quantity,
		&it.EnhanceLevel,
		&baseRaw,
		&affixRaw,
		&it.Durability,
		&it.MaxDurability,
		&it.State,
		&it.ArchivedAt,
		&it.CreatedAt,
		&it.UpdatedAt,
		&equippedSlot,
		&forgedRevisionID,
	)
	if err != nil {
		return ItemInstance{}, "", err
	}
	_ = json.Unmarshal(baseRaw, &it.BaseSnapshot)
	it.CustomData = decodeItemCustomization(customDataRaw)
	if string(fishDataRaw) != "{}" {
		var fish FishInstanceData
		if err := json.Unmarshal(fishDataRaw, &fish); err == nil && strings.TrimSpace(fish.SpeciesID) != "" {
			it.FishData = &fish
		}
	}
	it.Affixes = decodeAffixSlots(affixRaw)
	if forgedRevisionID != "" {
		it.ForgedSkillbook, _ = s.GetForgedSkillbookByInstance(ctx, userID, instanceID)
	}
	return it, equippedSlot, nil
}

func listEquippedItemsQuery() string {
	return `
		SELECT i.instance_id, i.template_item_id, COALESCE(i.custom_data, '{}'::jsonb), s.equip_type, i.enhance_level, i.affixes, i.durability, i.max_durability, i.state,
		       COALESCE(string_agg(s.slot, ',' ORDER BY s.slot), '') AS equipped_slot,
		       i.updated_at
		FROM player_equip_slots s
		JOIN player_item_instances i ON i.instance_id = s.instance_id AND i.user_id = s.user_id
		WHERE s.user_id = $1 AND i.state = 'equipped'
		GROUP BY i.instance_id, i.template_item_id, i.custom_data, s.equip_type, i.enhance_level, i.affixes, i.durability, i.max_durability, i.state, i.updated_at
		ORDER BY i.instance_id DESC, s.equip_type ASC
	`
}

func (s *Store) ListEquippedItems(ctx context.Context, userID int64) ([]EquippedItem, error) {
	if userID <= 0 {
		return []EquippedItem{}, nil
	}
	out := make([]EquippedItem, 0, 16)
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		if err := s.lockUserTx(ctx, tx, userID); err != nil {
			return err
		}
		battleProfile, err := s.resolvePlayerBattleSlotProfileTx(ctx, tx, userID)
		if err != nil {
			return err
		}
		rows, err := tx.Query(ctx, listEquippedItemsQuery(), userID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var item EquippedItem
			var affixRaw []byte
			var customDataRaw []byte
			if err := rows.Scan(
				&item.InstanceID,
				&item.TemplateItemID,
				&customDataRaw,
				&item.EquipType,
				&item.EnhanceLevel,
				&affixRaw,
				&item.Durability,
				&item.MaxDurability,
				&item.State,
				&item.EquippedSlot,
				&item.UpdatedAt,
			); err != nil {
				return err
			}
			if normalizeEquipType(item.EquipType) == "battle" {
				template := s.gameData.EquipmentTemplateByID(strings.TrimSpace(item.TemplateItemID))
				valid := template != nil
				for _, slot := range strings.Split(item.EquippedSlot, ",") {
					slot = strings.TrimSpace(slot)
					if slot == "" || !equipmentTemplateAllowsEquipSlotWithProfile(template, "battle", slot, battleProfile) || (slot == "right_hand_2" && template != nil && len(template.OccupySlots) > 0) {
						valid = false
						break
					}
				}
				if !valid {
					continue
				}
			}
			item.CustomData = decodeItemCustomization(customDataRaw)
			item.Affixes = decodeAffixSlots(affixRaw)
			out = append(out, item)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Store) EquipItem(ctx context.Context, userID, instanceID int64, equipType, slot string, occupySlots []string, accessoryTemplateIDs []string) error {
	if slot == "" || len(occupySlots) == 0 {
		return ErrEquipSlotNotAllowed
	}
	return s.withTx(ctx, func(tx pgx.Tx) error {
		return s.EquipTx(ctx, tx, userID, instanceID, equipType, occupySlots, accessoryTemplateIDs)
	})
}

func (s *Store) EquipItemToSlot(ctx context.Context, userID, instanceID int64, equipType, slot string) error {
	if strings.TrimSpace(slot) == "" {
		return ErrEquipSlotNotAllowed
	}
	return s.withTx(ctx, func(tx pgx.Tx) error {
		return s.EquipSelectedSlotTx(ctx, tx, userID, instanceID, equipType, slot)
	})
}

func (s *Store) UnequipItem(ctx context.Context, userID int64, equipType, slot string) error {
	return s.withTx(ctx, func(tx pgx.Tx) error {
		return s.UnequipBySlotTx(ctx, tx, userID, equipType, slot)
	})
}

func (s *Store) UnequipInstance(ctx context.Context, userID, instanceID int64) (string, error) {
	var slot string
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		var txErr error
		slot, txErr = s.UnequipByInstanceTx(ctx, tx, userID, instanceID)
		return txErr
	})
	if err != nil {
		return "", err
	}
	return slot, nil
}

func (s *Store) GetPlayerStats(ctx context.Context, userID int64) (PlayerStats, error) {
	var out PlayerStats
	var cacheRaw []byte
	err := s.db.QueryRow(ctx, `
		SELECT stats_jsonb, version, updated_at
		FROM player_derived_stats_cache
		WHERE user_id = $1
	`, userID).Scan(&cacheRaw, &out.CacheVer, &out.UpdatedAt)
	if err != nil {
		return out, err
	}
	_ = json.Unmarshal(cacheRaw, &out.Derived)
	return out, nil
}

func (s *Store) RecomputeDerivedStats(ctx context.Context, userID int64) error {
	return s.withTx(ctx, func(tx pgx.Tx) error {
		if err := s.lockUserTx(ctx, tx, userID); err != nil {
			return err
		}
		return s.recomputeDerivedStatsTx(ctx, tx, userID)
	})
}

func (s *Store) recomputeDerivedStatsTx(ctx context.Context, tx pgx.Tx, userID int64) error {
	if err := s.lockUserTx(ctx, tx, userID); err != nil {
		return err
	}
	if _, err := s.cleanupPlayerEquipSlotsForStatsTx(ctx, tx, userID); err != nil {
		return err
	}
	if err := s.ensureAllPlayerAttributesTx(ctx, tx, userID); err != nil {
		return err
	}
	levelMap := map[string]int64{}
	skillPointLevelMap := map[string]int64{}
	attrRows, err := tx.Query(ctx, `
		SELECT attr_key, base_level, skill_point_level
		FROM player_attributes
		WHERE user_id = $1
		FOR UPDATE
	`, userID)
	if err != nil {
		return err
	}
	for attrRows.Next() {
		var key string
		var baseLevel int64
		var skillPointLevel int64
		if err := attrRows.Scan(&key, &baseLevel, &skillPointLevel); err != nil {
			attrRows.Close()
			return err
		}
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		levelMap[key] = baseLevel
		skillPointLevelMap[key] = skillPointLevel
	}
	if err := attrRows.Err(); err != nil {
		attrRows.Close()
		return err
	}
	attrRows.Close()

	baseDefaultMap := make(map[string]float64, len(s.gameData.Attributes))
	for _, row := range s.gameData.Attributes {
		key := strings.TrimSpace(row.ID)
		if key == "" {
			continue
		}
		baseDefaultMap[key] = row.BaseDefault
	}
	formulaTargetSet := s.derivedFormulaTargetSet()

	attrTotals := make(map[string]float64, len(levelMap))
	baseValues := make(map[string]float64, len(levelMap))
	levelBonusMap := make(map[string]float64, len(levelMap))
	baseLevelBonusMap := make(map[string]float64, len(levelMap))
	skillPointBonusMap := make(map[string]float64, len(levelMap))
	baseExcludedMap := make(map[string]bool, len(levelMap))
	for key, baseLevel := range levelMap {
		baseDefault := baseDefaultMap[key]
		baseLevelBonus := float64(0)
		if baseLevel > 1 {
			baseLevelBonus = float64(baseLevel - 1)
		}
		skillPointBonus := float64(0)
		if skillPointLevel, ok := skillPointLevelMap[key]; ok && skillPointLevel > 0 {
			skillPointBonus += float64(skillPointLevel)
		}
		levelBonus := baseLevelBonus + skillPointBonus
		baseValue := baseDefault + levelBonus
		if includeBase, isFormulaDerived := formulaTargetSet[key]; isFormulaDerived && !includeBase {
			baseExcludedMap[key] = true
			baseValue = 0
		}
		baseValues[key] = roundStatValue(baseValue)
		baseLevelBonusMap[key] = roundStatValue(baseLevelBonus)
		skillPointBonusMap[key] = roundStatValue(skillPointBonus)
		levelBonusMap[key] = roundStatValue(levelBonus)
		attrTotals[key] = roundStatValue(baseValue)
	}

	equipmentByAttr := map[string][]equipmentAttrContribution{}
	var convItems []equippedItemForConv
	equipmentRows, err := s.listValidPlayerEquippedItemsTx(ctx, tx, userID)
	if err != nil {
		return err
	}

	bodyAffixSeen := map[string]int{}
	for _, row := range equipmentRows {
		instanceID := row.InstanceID
		templateItemID := row.TemplateItemID
		affixRaw := row.AffixRaw
		enhanceLevel := row.EnhanceLevel
		if tmpl := s.gameData.EquipmentTemplateByID(strings.TrimSpace(templateItemID)); tmpl != nil && len(tmpl.AttrConversions) > 0 {
			convItems = append(convItems, equippedItemForConv{templateID: strings.TrimSpace(templateItemID), enhanceLevel: enhanceLevel})
		}
		baseAttrs := map[string]float64{}
		if tmpl := s.gameData.EquipmentTemplateByID(strings.TrimSpace(templateItemID)); tmpl != nil {
			for key, value := range tmpl.BaseAttrs {
				normalizedKey := strings.TrimSpace(key)
				if normalizedKey == "" {
					continue
				}
				baseAttrs[normalizedKey] = value
			}
		}
		enhanceByAttr := map[string]float64{}
		if s.enhanceCache != nil && enhanceLevel > 0 {
			for key, value := range s.enhanceCache.CumulativeAttrs(strings.TrimSpace(templateItemID), enhanceLevel) {
				normalizedKey := strings.TrimSpace(key)
				if normalizedKey == "" {
					continue
				}
				enhanceByAttr[normalizedKey] += value
			}
		}
		affixSlots := FilterEffectiveAffixSlotsByLimit(decodeAffixSlots(affixRaw), s.gameData, bodyAffixSeen)
		affixByAttr := map[string][]equipmentAffixContribution{}
		for _, slot := range affixSlots {
			affixID := strings.TrimSpace(slot.AffixID)
			for attrKey, rawValue := range slot.AttrMods {
				attrKey = strings.TrimSpace(attrKey)
				if attrKey == "" {
					continue
				}
				value := roundStatValue(rawValue)
				if value == 0 {
					continue
				}
				affixByAttr[attrKey] = append(affixByAttr[attrKey], equipmentAffixContribution{
					AffixID:         affixID,
					Value:           value,
					DisplayNameKey:  strings.TrimSpace(slot.DisplayNameKey),
					DisplayNameArgs: slot.DisplayNameArgs,
				})
			}
		}
		seenAttr := map[string]struct{}{}
		for attrKey := range baseAttrs {
			seenAttr[attrKey] = struct{}{}
		}
		for attrKey := range enhanceByAttr {
			seenAttr[attrKey] = struct{}{}
		}
		for attrKey := range affixByAttr {
			seenAttr[attrKey] = struct{}{}
		}
		for attrKey := range seenAttr {
			baseValue := roundStatValue(baseAttrs[attrKey])
			enhanceValue := roundStatValue(enhanceByAttr[attrKey])
			affixItems := append([]equipmentAffixContribution{}, affixByAttr[attrKey]...)
			sort.Slice(affixItems, func(i, j int) bool {
				if affixItems[i].AffixID != affixItems[j].AffixID {
					return affixItems[i].AffixID < affixItems[j].AffixID
				}
				return affixItems[i].Value > affixItems[j].Value
			})
			total := baseValue + enhanceValue
			for _, item := range affixItems {
				total += item.Value
			}
			total = roundStatValue(total)
			if total == 0 {
				continue
			}
			entry := equipmentAttrContribution{
				InstanceID:     instanceID,
				TemplateItemID: strings.TrimSpace(templateItemID),
				Base:           baseValue,
				Enhance:        enhanceValue,
				Affixes:        affixItems,
			}
			equipmentByAttr[attrKey] = append(equipmentByAttr[attrKey], entry)
			attrTotals[attrKey] = roundStatValue(attrTotals[attrKey] + total)
		}
	}

	for attrKey := range equipmentByAttr {
		sort.Slice(equipmentByAttr[attrKey], func(i, j int) bool {
			if equipmentByAttr[attrKey][i].TemplateItemID != equipmentByAttr[attrKey][j].TemplateItemID {
				return equipmentByAttr[attrKey][i].TemplateItemID < equipmentByAttr[attrKey][j].TemplateItemID
			}
			return equipmentByAttr[attrKey][i].InstanceID < equipmentByAttr[attrKey][j].InstanceID
		})
	}

	collectiblesByAttr, collectiblePlayerMods, err := s.loadCollectiblePlayerModsTx(ctx, tx, userID)
	if err != nil {
		return err
	}
	for attrKey, modValue := range collectiblePlayerMods {
		key := strings.TrimSpace(attrKey)
		if key == "" || modValue == 0 {
			continue
		}
		attrTotals[key] = roundStatValue(attrTotals[key] + modValue)
	}
	if err := s.applyCharacterSpecializationModifiersTx(ctx, tx, userID, attrTotals); err != nil {
		return err
	}

	formulaByAttr := map[string]formulaContribution{}
	for _, rawFormula := range s.config.DerivedStatsFormula.Formulas {
		targetAttr := strings.TrimSpace(rawFormula.TargetAttr)
		if targetAttr == "" {
			continue
		}
		terms := make([]formulaTerm, 0, len(rawFormula.Terms))
		for _, rawTerm := range rawFormula.Terms {
			attrKey := strings.TrimSpace(rawTerm.AttrKey)
			if attrKey == "" {
				continue
			}
			terms = append(terms, formulaTerm{
				AttrKey:     attrKey,
				Attribute:   roundStatValue(attrTotals[attrKey]),
				Coefficient: roundStatValue(rawTerm.Coefficient),
			})
		}
		if len(terms) == 0 {
			continue
		}
		applyFormulaContribution(attrTotals, formulaByAttr, targetAttr, buildDerivedStatFormulaExpression(rawFormula.Terms), terms)
	}

	// Apply equipment attr-conversion effects (e.g. crit overflow ring).
	// Must run after all formula contributions so we work on the fully-derived totals.
	conversionByAttr := map[string]float64{}
	conversionSourcesByAttr := map[string][]attrConversionSource{}
	for _, item := range convItems {
		tmpl := s.gameData.EquipmentTemplateByID(item.templateID)
		if tmpl == nil {
			continue
		}
		for _, node := range tmpl.AttrConversions {
			fromAttr := attrConvString(node["fromAttr"])
			toAttr := attrConvString(node["toAttr"])
			if fromAttr == "" || toAttr == "" || fromAttr == toAttr {
				continue
			}
			threshold := attrConvFloat(node["threshold"], 1.0)
			ratio := evaluateAttrConvRatio(node["ratioFormula"], node["ratio"], item.enhanceLevel, attrTotals)
			if ratio <= 0 {
				continue
			}
			currentFrom := attrTotals[fromAttr] + conversionByAttr[fromAttr]
			excess := roundStatValue(currentFrom - threshold)
			if excess <= 0 {
				continue
			}
			converted := roundStatValue(excess * ratio)
			conversionByAttr[fromAttr] = roundStatValue(conversionByAttr[fromAttr] - excess)
			conversionByAttr[toAttr] = roundStatValue(conversionByAttr[toAttr] + converted)
			src := attrConversionSource{
				TemplateItemID: item.templateID,
				FromAttr:       fromAttr,
				ToAttr:         toAttr,
				Excess:         excess,
				Ratio:          roundStatValue(ratio),
				Converted:      converted,
			}
			conversionSourcesByAttr[fromAttr] = append(conversionSourcesByAttr[fromAttr], src)
			conversionSourcesByAttr[toAttr] = append(conversionSourcesByAttr[toAttr], src)
		}
	}
	for attrKey, delta := range conversionByAttr {
		if delta != 0 {
			attrTotals[attrKey] = roundStatValue(attrTotals[attrKey] + delta)
		}
	}

	derived := make(map[string]float64, len(attrTotals))
	for key, value := range attrTotals {
		derived[key] = roundStatValue(value)
	}

	updateAttrKeys := make([]string, 0, len(levelMap))
	updateFinalValues := make([]float64, 0, len(levelMap))
	updateSourcePayloads := make([]string, 0, len(levelMap))
	levelKeys := make([]string, 0, len(levelMap))
	for key := range levelMap {
		levelKeys = append(levelKeys, key)
	}
	sort.Strings(levelKeys)
	for _, key := range levelKeys {
		finalValue := roundStatValue(attrTotals[key])
		attrTotals[key] = finalValue
		sourcePayload := buildAttributeSourcePayload(attributeSourceBuildInput{
			AttrKey:                 key,
			BaseDefault:             roundStatValue(baseDefaultMap[key]),
			BaseLevelBonus:          baseLevelBonusMap[key],
			SkillPointBonus:         skillPointBonusMap[key],
			LevelBonus:              levelBonusMap[key],
			BaseValue:               baseValues[key],
			BaseExcluded:            baseExcludedMap[key],
			FinalValue:              finalValue,
			EquipmentByAttr:         equipmentByAttr,
			FormulaByAttr:           formulaByAttr,
			BaseValuesByAttr:        baseValues,
			CollectiblesByAttr:      collectiblesByAttr,
			ConversionByAttr:        conversionByAttr,
			ConversionSourcesByAttr: conversionSourcesByAttr,
		})
		sourceRaw, err := json.Marshal(sourcePayload)
		if err != nil {
			return err
		}
		updateAttrKeys = append(updateAttrKeys, key)
		updateFinalValues = append(updateFinalValues, finalValue)
		updateSourcePayloads = append(updateSourcePayloads, string(sourceRaw))
	}
	if len(updateAttrKeys) > 0 {
		if _, err := tx.Exec(ctx, `
			UPDATE player_attributes AS attr
			SET final_value = updates.final_value,
			    source_mods_jsonb = updates.source_mods::jsonb,
			    lock_version = attr.lock_version + 1,
			    updated_at = now()
			FROM unnest($2::text[], $3::double precision[], $4::text[]) AS updates(attr_key, final_value, source_mods)
			WHERE attr.user_id = $1 AND attr.attr_key = updates.attr_key
		`, userID, updateAttrKeys, updateFinalValues, updateSourcePayloads); err != nil {
			return err
		}
	}

	payload, err := json.Marshal(derived)
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `
		INSERT INTO player_derived_stats_cache(user_id, stats_jsonb, version, updated_at)
		VALUES($1, $2::jsonb, 1, now())
		ON CONFLICT (user_id)
		DO UPDATE SET stats_jsonb = EXCLUDED.stats_jsonb, version = player_derived_stats_cache.version + 1, updated_at = now()
	`, userID, string(payload)); err != nil {
		return err
	}
	return nil
}

func (s *Store) loadPlayerBaseAttributeStateTx(ctx context.Context, tx pgx.Tx, userID int64) (
	map[string]int64,
	map[string]int64,
	map[string]float64,
	map[string]float64,
	map[string]float64,
	map[string]float64,
	map[string]bool,
	map[string]float64,
	error,
) {
	if err := s.lockUserTx(ctx, tx, userID); err != nil {
		return nil, nil, nil, nil, nil, nil, nil, nil, err
	}
	if err := s.ensureAllPlayerAttributesTx(ctx, tx, userID); err != nil {
		return nil, nil, nil, nil, nil, nil, nil, nil, err
	}
	levelMap := map[string]int64{}
	skillPointLevelMap := map[string]int64{}
	attrRows, err := tx.Query(ctx, `
		SELECT attr_key, base_level, skill_point_level
		FROM player_attributes
		WHERE user_id = $1
		FOR UPDATE
	`, userID)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, nil, nil, err
	}
	for attrRows.Next() {
		var key string
		var baseLevel int64
		var skillPointLevel int64
		if err := attrRows.Scan(&key, &baseLevel, &skillPointLevel); err != nil {
			attrRows.Close()
			return nil, nil, nil, nil, nil, nil, nil, nil, err
		}
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		levelMap[key] = baseLevel
		skillPointLevelMap[key] = skillPointLevel
	}
	if err := attrRows.Err(); err != nil {
		attrRows.Close()
		return nil, nil, nil, nil, nil, nil, nil, nil, err
	}
	attrRows.Close()
	baseDefaultMap := make(map[string]float64, len(s.gameData.Attributes))
	for _, row := range s.gameData.Attributes {
		key := strings.TrimSpace(row.ID)
		if key == "" {
			continue
		}
		baseDefaultMap[key] = row.BaseDefault
	}
	formulaTargetSet := s.derivedFormulaTargetSet()
	attrTotals := make(map[string]float64, len(levelMap))
	baseValues := make(map[string]float64, len(levelMap))
	levelBonusMap := make(map[string]float64, len(levelMap))
	baseLevelBonusMap := make(map[string]float64, len(levelMap))
	skillPointBonusMap := make(map[string]float64, len(levelMap))
	baseExcludedMap := make(map[string]bool, len(levelMap))
	for key, baseLevel := range levelMap {
		baseDefault := baseDefaultMap[key]
		baseLevelBonus := float64(0)
		if baseLevel > 1 {
			baseLevelBonus = float64(baseLevel - 1)
		}
		skillPointBonus := float64(0)
		if skillPointLevel, ok := skillPointLevelMap[key]; ok && skillPointLevel > 0 {
			skillPointBonus += float64(skillPointLevel)
		}
		levelBonus := baseLevelBonus + skillPointBonus
		baseValue := baseDefault + levelBonus
		if includeBase, isFormulaDerived := formulaTargetSet[key]; isFormulaDerived && !includeBase {
			baseExcludedMap[key] = true
			baseValue = 0
		}
		baseValues[key] = roundStatValue(baseValue)
		baseLevelBonusMap[key] = roundStatValue(baseLevelBonus)
		skillPointBonusMap[key] = roundStatValue(skillPointBonus)
		levelBonusMap[key] = roundStatValue(levelBonus)
		attrTotals[key] = roundStatValue(baseValue)
	}
	return levelMap, skillPointLevelMap, baseDefaultMap, baseValues, baseLevelBonusMap, skillPointBonusMap, baseExcludedMap, attrTotals, nil
}

func (s *Store) onPlayerAttributesChangedTx(ctx context.Context, tx pgx.Tx, userID int64) error {
	if err := s.recomputeDerivedStatsTx(ctx, tx, userID); err != nil {
		return err
	}
	if err := s.refreshQueueDurationsTx(ctx, tx, userID); err != nil {
		return err
	}
	return s.EnqueueAchievementEventsTx(ctx, tx, userID, []AchievementEvent{{
		Kind:      AchievementEventStateReconcile,
		StateKind: "player_attr_reached",
		SourceKey: "player_attr_changed:" + strconv.FormatInt(time.Now().UnixNano(), 10),
	}})
}

func (s *Store) computePlayerAttributeTotalsTx(ctx context.Context, tx pgx.Tx, userID int64, overrideWork []virtualEquipmentItem) (map[string]float64, map[string]float64, error) {
	_, _, _, _, _, _, _, attrTotals, err := s.loadPlayerBaseAttributeStateTx(ctx, tx, userID)
	if err != nil {
		return nil, nil, err
	}
	baseTotals := map[string]float64{}
	for key, value := range attrTotals {
		baseTotals[key] = value
	}
	useOverride := len(overrideWork) > 0
	rows, err := s.listValidPlayerEquippedItemsTx(ctx, tx, userID)
	if err != nil {
		return nil, nil, err
	}
	current := make([]validPlayerEquippedItemRow, 0, len(rows))
	for _, row := range rows {
		if useOverride && normalizeEquipType(row.EquipType) == "work" {
			continue
		}
		current = append(current, row)
	}
	bodyAffixSeen := map[string]int{}
	applyItem := func(templateItemID string, enhanceLevel int, affixes []EquipmentAffixSlot) {
		if tmpl := s.gameData.EquipmentTemplateByID(strings.TrimSpace(templateItemID)); tmpl != nil {
			for key, value := range tmpl.BaseAttrs {
				key = strings.TrimSpace(key)
				if key == "" {
					continue
				}
				attrTotals[key] = roundStatValue(attrTotals[key] + value)
			}
		}
		if s.enhanceCache != nil && enhanceLevel > 0 {
			for key, value := range s.enhanceCache.CumulativeAttrs(strings.TrimSpace(templateItemID), enhanceLevel) {
				key = strings.TrimSpace(key)
				if key == "" {
					continue
				}
				attrTotals[key] = roundStatValue(attrTotals[key] + value)
			}
		}
		for _, slot := range FilterEffectiveAffixSlotsByLimit(affixes, s.gameData, bodyAffixSeen) {
			for key, value := range slot.AttrMods {
				key = strings.TrimSpace(key)
				if key == "" {
					continue
				}
				attrTotals[key] = roundStatValue(attrTotals[key] + roundStatValue(value))
			}
		}
	}
	for _, row := range current {
		applyItem(row.TemplateItemID, row.EnhanceLevel, decodeAffixSlots(row.AffixRaw))
	}
	if useOverride {
		for _, row := range overrideWork {
			applyItem(row.TemplateItemID, row.EnhanceLevel, row.Affixes)
		}
	}
	if err := s.applyCharacterSpecializationModifiersTx(ctx, tx, userID, attrTotals); err != nil {
		return nil, nil, err
	}
	for _, rawFormula := range s.config.DerivedStatsFormula.Formulas {
		targetAttr := strings.TrimSpace(rawFormula.TargetAttr)
		if targetAttr == "" {
			continue
		}
		terms := make([]formulaTerm, 0, len(rawFormula.Terms))
		for _, rawTerm := range rawFormula.Terms {
			attrKey := strings.TrimSpace(rawTerm.AttrKey)
			if attrKey == "" {
				continue
			}
			terms = append(terms, formulaTerm{
				AttrKey:     attrKey,
				Attribute:   roundStatValue(attrTotals[attrKey]),
				Coefficient: roundStatValue(rawTerm.Coefficient),
			})
		}
		if len(terms) == 0 {
			continue
		}
		applyFormulaContribution(attrTotals, map[string]formulaContribution{}, targetAttr, buildDerivedStatFormulaExpression(rawFormula.Terms), terms)
	}
	deltas := map[string]float64{}
	for key, value := range attrTotals {
		delta := roundStatValue(value - baseTotals[key])
		if delta != 0 {
			deltas[key] = delta
		}
	}
	return attrTotals, deltas, nil
}

func (s *Store) ensureAllPlayerAttributesTx(ctx context.Context, tx pgx.Tx, userID int64) error {
	rows := make([]playerAttributeDefaultRow, 0, len(s.gameData.Attributes))
	for _, seed := range s.gameData.Attributes {
		row, ok, err := playerAttributeDefaultRowFromSeed(seed.ID, seed.BaseDefault, roundStatValue(seed.BaseDefault))
		if err != nil {
			return err
		}
		if ok {
			rows = append(rows, row)
		}
	}
	return insertPlayerAttributeDefaultRowsTx(ctx, tx, userID, rows)
}

type equipmentAffixContribution struct {
	AffixID         string
	Value           float64
	DisplayNameKey  string
	DisplayNameArgs map[string]any
}

type equipmentAttrContribution struct {
	InstanceID     int64
	TemplateItemID string
	Base           float64
	Enhance        float64
	Affixes        []equipmentAffixContribution
}

func (c equipmentAttrContribution) Total() float64 {
	total := c.Base + c.Enhance
	for _, affix := range c.Affixes {
		total += affix.Value
	}
	return roundStatValue(total)
}

type formulaTerm struct {
	AttrKey     string
	Attribute   float64
	Coefficient float64
}

type formulaContribution struct {
	Expression string
	Value      float64
	Terms      []formulaTerm
}

type personalityAttrContribution struct {
	PersonalityID string
	NameKey       string
	Value         float64
}

type collectibleAttrContribution struct {
	CollectibleID string
	NameKey       string
	Value         float64
}

func applyFormulaContribution(attrTotals map[string]float64, formulaByAttr map[string]formulaContribution, targetAttr string, expression string, terms []formulaTerm) {
	total := float64(0)
	normalizedTerms := make([]formulaTerm, 0, len(terms))
	for _, term := range terms {
		term.Attribute = roundStatValue(term.Attribute)
		term.Coefficient = roundStatValue(term.Coefficient)
		normalizedTerms = append(normalizedTerms, term)
		total += term.Attribute * term.Coefficient
	}
	total = roundStatValue(total)
	formulaByAttr[targetAttr] = formulaContribution{
		Expression: expression,
		Value:      total,
		Terms:      normalizedTerms,
	}
	attrTotals[targetAttr] = roundStatValue(attrTotals[targetAttr] + total)
}

func defaultDerivedStatsFormulaConfig() DerivedStatsFormulaConfig {
	return DerivedStatsFormulaConfig{
		Version:   "v2",
		Operators: []string{"+", "-", "*", "/"},
		Formulas:  []DerivedStatFormula{
			// {
			// 	TargetAttr:  "physics_damage",
			// 	IncludeBase: true,
			// 	Terms: []DerivedStatFormulaTerm{
			// 		{AttrKey: "strength", Coefficient: 0.6},
			// 		{AttrKey: "dexterity", Coefficient: 0.4},
			// 	},
			// },
			// {
			// 	TargetAttr:  "attack_speed",
			// 	IncludeBase: true,
			// 	Terms: []DerivedStatFormulaTerm{
			// 		{AttrKey: "dexterity", Coefficient: 1},
			// 	},
			// },
			// {
			// 	TargetAttr:  "magical_damage",
			// 	IncludeBase: true,
			// 	Terms: []DerivedStatFormulaTerm{
			// 		{AttrKey: "intelligence", Coefficient: 1.2},
			// 	},
			// },
			// {
			// 	TargetAttr:  "hp",
			// 	IncludeBase: true,
			// 	Terms: []DerivedStatFormulaTerm{
			// 		{AttrKey: "stamina", Coefficient: 10},
			// 	},
			// },
			// {
			// 	TargetAttr:  "hp_regen",
			// 	IncludeBase: true,
			// 	Terms: []DerivedStatFormulaTerm{
			// 		{AttrKey: "stamina", Coefficient: 1},
			// 	},
			// },
			// {
			// 	TargetAttr:  "mp",
			// 	IncludeBase: true,
			// 	Terms: []DerivedStatFormulaTerm{
			// 		{AttrKey: "intelligence", Coefficient: 5},
			// 	},
			// },
			// {
			// 	TargetAttr:  "mp_regen",
			// 	IncludeBase: true,
			// 	Terms: []DerivedStatFormulaTerm{
			// 		{AttrKey: "intelligence", Coefficient: 0.5},
			// 	},
			// },
		},
	}
}

func normalizeDerivedStatsFormulaConfig(cfg DerivedStatsFormulaConfig) DerivedStatsFormulaConfig {
	cfg.Version = strings.TrimSpace(cfg.Version)
	if cfg.Version == "" {
		cfg.Version = "v2"
	}
	normalizedOps := make([]string, 0, len(cfg.Operators))
	for _, operator := range cfg.Operators {
		operator = strings.TrimSpace(operator)
		if operator == "" {
			continue
		}
		normalizedOps = append(normalizedOps, operator)
	}
	if len(normalizedOps) == 0 {
		normalizedOps = []string{"+", "-", "*", "/"}
	}
	cfg.Operators = normalizedOps
	normalizedFormulas := make([]DerivedStatFormula, 0, len(cfg.Formulas))
	for _, rawFormula := range cfg.Formulas {
		targetAttr := strings.TrimSpace(rawFormula.TargetAttr)
		if targetAttr == "" {
			continue
		}
		terms := make([]DerivedStatFormulaTerm, 0, len(rawFormula.Terms))
		for _, rawTerm := range rawFormula.Terms {
			attrKey := strings.TrimSpace(rawTerm.AttrKey)
			if attrKey == "" {
				continue
			}
			coefficient := roundStatValue(rawTerm.Coefficient)
			if coefficient == 0 || math.IsNaN(coefficient) || math.IsInf(coefficient, 0) {
				continue
			}
			terms = append(terms, DerivedStatFormulaTerm{
				AttrKey:     attrKey,
				Coefficient: coefficient,
			})
		}
		if len(terms) == 0 {
			continue
		}
		normalizedFormulas = append(normalizedFormulas, DerivedStatFormula{
			TargetAttr:  targetAttr,
			IncludeBase: rawFormula.IncludeBase,
			Terms:       terms,
		})
	}
	if len(normalizedFormulas) == 0 {
		return defaultDerivedStatsFormulaConfig()
	}
	cfg.Formulas = normalizedFormulas
	return cfg
}

func buildDerivedStatFormulaExpression(terms []DerivedStatFormulaTerm) string {
	parts := make([]string, 0, len(terms))
	for _, term := range terms {
		attrKey := strings.TrimSpace(term.AttrKey)
		if attrKey == "" {
			continue
		}
		parts = append(parts, attrKey+"*"+trimmedFormulaNumber(term.Coefficient))
	}
	return strings.Join(parts, " + ")
}

func trimmedFormulaNumber(value float64) string {
	value = roundStatValue(value)
	return strings.TrimRight(strings.TrimRight(strconv.FormatFloat(value, 'f', 4, 64), "0"), ".")
}

func (s *Store) derivedFormulaTargetSet() map[string]bool {
	targetSet := make(map[string]bool, len(s.config.DerivedStatsFormula.Formulas))
	for _, formula := range s.config.DerivedStatsFormula.Formulas {
		targetAttr := strings.TrimSpace(formula.TargetAttr)
		if targetAttr == "" {
			continue
		}
		targetSet[targetAttr] = formula.IncludeBase
	}
	return targetSet
}

func decodeNumericJSONMap(raw []byte) map[string]float64 {
	if len(raw) == 0 {
		return map[string]float64{}
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return map[string]float64{}
	}
	out := make(map[string]float64, len(decoded))
	for key, value := range decoded {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		num, ok := asFloat64(value)
		if !ok {
			continue
		}
		num = roundStatValue(num)
		if num == 0 {
			continue
		}
		out[key] = num
	}
	return out
}

func asFloat64(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		if math.IsNaN(typed) || math.IsInf(typed, 0) {
			return 0, false
		}
		return typed, true
	case float32:
		v := float64(typed)
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return 0, false
		}
		return v, true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case int32:
		return float64(typed), true
	case int16:
		return float64(typed), true
	case int8:
		return float64(typed), true
	case uint:
		return float64(typed), true
	case uint64:
		return float64(typed), true
	case uint32:
		return float64(typed), true
	case uint16:
		return float64(typed), true
	case uint8:
		return float64(typed), true
	default:
		return 0, false
	}
}

func roundStatValue(value float64) float64 {
	return math.Round(value*10000) / 10000
}

type attrConversionSource struct {
	TemplateItemID string  `json:"templateItemId"`
	FromAttr       string  `json:"fromAttr"`
	ToAttr         string  `json:"toAttr"`
	Excess         float64 `json:"excess"`
	Ratio          float64 `json:"ratio"`
	Converted      float64 `json:"converted"`
}

type attributeSourceBuildInput struct {
	AttrKey                 string
	BaseDefault             float64
	BaseLevelBonus          float64
	SkillPointBonus         float64
	LevelBonus              float64
	BaseValue               float64
	BaseExcluded            bool
	FinalValue              float64
	CollectiblesByAttr      map[string][]collectibleAttrContribution
	EquipmentByAttr         map[string][]equipmentAttrContribution
	PersonalityByAttr       map[string][]personalityAttrContribution
	FormulaByAttr           map[string]formulaContribution
	BaseValuesByAttr        map[string]float64
	ConversionByAttr        map[string]float64                // net delta per attr
	ConversionSourcesByAttr map[string][]attrConversionSource // detailed sources per attr
}

// attrConvFloat converts any JSON-decoded value to float64 with a fallback.
func attrConvFloat(v any, fallback float64) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case float32:
		return float64(x)
	case int:
		return float64(x)
	case int64:
		return float64(x)
	case int32:
		return float64(x)
	case json.Number:
		if f, err := x.Float64(); err == nil {
			return f
		}
	}
	return fallback
}

// attrConvString safely casts any to string.
func attrConvString(v any) string {
	s, _ := v.(string)
	return strings.ToLower(strings.TrimSpace(s))
}

// evaluateAttrConvRatio evaluates the ratio from a ratioFormula node or a flat ratio value.
// For equipment_enhance_level terms the item's enhance level is used directly.
// For all other attrKey terms the player's current attribute total is used.
// perSkillLevel acts as an extra coefficient per enhance level, consistent with battle formulas.
func evaluateAttrConvRatio(rawFormula any, rawRatio any, enhanceLevel int, attrTotals map[string]float64) float64 {
	var ratio float64
	if rawFormula != nil {
		if m, ok := rawFormula.(map[string]any); ok {
			ratio = attrConvFloat(m["baseAdd"], 0)
			if terms, ok := m["terms"].([]any); ok {
				for _, rawTerm := range terms {
					term, ok := rawTerm.(map[string]any)
					if !ok {
						continue
					}
					attrKey := attrConvString(term["attrKey"])
					if attrKey == "" {
						continue
					}
					coefficient := attrConvFloat(term["coefficient"], 0)
					perSkillLevel := attrConvFloat(term["perSkillLevel"], 0)
					effectiveCoeff := coefficient + perSkillLevel*float64(enhanceLevel)
					if attrKey == "equipment_enhance_level" {
						ratio += float64(enhanceLevel) * effectiveCoeff
					} else {
						ratio += attrTotals[attrKey] * effectiveCoeff
					}
				}
			}
		}
	} else {
		ratio = attrConvFloat(rawRatio, 0)
	}
	if math.IsNaN(ratio) || math.IsInf(ratio, 0) {
		return 0
	}
	return ratio
}

// equippedItemForConv holds the minimal info needed to evaluate attrConversions.
type equippedItemForConv struct {
	templateID   string
	enhanceLevel int
}

func buildAttributeSourcePayload(input attributeSourceBuildInput) map[string]any {
	equipmentLines := buildEquipmentBreakdownLines(input.EquipmentByAttr[input.AttrKey])
	personalityLines := buildPersonalityBreakdownLines(input.PersonalityByAttr[input.AttrKey])
	formulaLine := map[string]any(nil)
	if formula, ok := input.FormulaByAttr[input.AttrKey]; ok {
		termLines := make([]map[string]any, 0, len(formula.Terms))
		for _, term := range formula.Terms {
			contribution := roundStatValue(term.Attribute * term.Coefficient)
			termLine := map[string]any{
				"attrKey":        term.AttrKey,
				"attributeValue": roundStatValue(term.Attribute),
				"coefficient":    roundStatValue(term.Coefficient),
				"value":          contribution,
			}
			baseValue := roundStatValue(input.BaseValuesByAttr[term.AttrKey])
			sourceEquipments := buildEquipmentBreakdownLines(input.EquipmentByAttr[term.AttrKey])
			sourcePersonalities := buildPersonalityBreakdownLines(input.PersonalityByAttr[term.AttrKey])
			if baseValue != 0 || len(sourceEquipments) > 0 || len(sourcePersonalities) > 0 {
				termLine["sourceBreakdown"] = map[string]any{
					"base": map[string]any{
						"value": baseValue,
					},
					"equipment":   sourceEquipments,
					"personality": sourcePersonalities,
					"summary": map[string]any{
						"add": roundStatValue(term.Attribute - baseValue),
						"mul": 1,
					},
				}
			}
			termLines = append(termLines, termLine)
		}
		formulaLine = map[string]any{
			"expression": formula.Expression,
			"value":      roundStatValue(formula.Value),
			"sources":    termLines,
		}
	}
	payload := map[string]any{
		"version": "v2",
		"final":   roundStatValue(input.FinalValue),
		"base": map[string]any{
			"value":           roundStatValue(input.BaseValue),
			"baseDefault":     roundStatValue(input.BaseDefault),
			"baseLevelBonus":  roundStatValue(input.BaseLevelBonus),
			"skillPointBonus": roundStatValue(input.SkillPointBonus),
			"levelBonus":      roundStatValue(input.LevelBonus),
			"excluded":        input.BaseExcluded,
		},
		"equipment":   equipmentLines,
		"personality": personalityLines,
		"collectible": buildCollectibleBreakdownLines(input.CollectiblesByAttr[input.AttrKey]),
		"summary": map[string]any{
			"add": roundStatValue(input.FinalValue - input.BaseValue),
			"mul": 1,
		},
	}
	if formulaLine != nil {
		payload["formula"] = formulaLine
	}
	if convDelta, ok := input.ConversionByAttr[input.AttrKey]; ok && convDelta != 0 {
		convSrcRaw := input.ConversionSourcesByAttr[input.AttrKey]
		srcPayload := make([]map[string]any, 0, len(convSrcRaw))
		for _, src := range convSrcRaw {
			srcPayload = append(srcPayload, map[string]any{
				"templateItemId": src.TemplateItemID,
				"fromAttr":       src.FromAttr,
				"toAttr":         src.ToAttr,
				"excess":         src.Excess,
				"ratio":          src.Ratio,
				"converted":      src.Converted,
			})
		}
		payload["conversion"] = map[string]any{
			"value":   roundStatValue(convDelta),
			"sources": srcPayload,
		}
	}
	return payload
}

func buildPersonalityBreakdownLines(entries []personalityAttrContribution) []map[string]any {
	lines := make([]map[string]any, 0, len(entries))
	for _, entry := range entries {
		value := roundStatValue(entry.Value)
		if value == 0 {
			continue
		}
		lines = append(lines, map[string]any{
			"personalityId": strings.TrimSpace(entry.PersonalityID),
			"nameKey":       strings.TrimSpace(entry.NameKey),
			"total":         value,
		})
	}
	return lines
}

func buildCollectibleBreakdownLines(entries []collectibleAttrContribution) []map[string]any {
	lines := make([]map[string]any, 0, len(entries))
	for _, entry := range entries {
		value := roundStatValue(entry.Value)
		if value == 0 {
			continue
		}
		lines = append(lines, map[string]any{
			"collectibleId": strings.TrimSpace(entry.CollectibleID),
			"nameKey":       strings.TrimSpace(entry.NameKey),
			"total":         value,
		})
	}
	return lines
}

func buildEquipmentBreakdownLines(entries []equipmentAttrContribution) []map[string]any {
	equipmentLines := make([]map[string]any, 0, len(entries))
	for _, entry := range entries {
		affixLines := make([]map[string]any, 0, len(entry.Affixes))
		affixTotal := float64(0)
		for _, affix := range entry.Affixes {
			affixValue := roundStatValue(affix.Value)
			affixTotal += affixValue
			line := map[string]any{
				"affixId": strings.TrimSpace(affix.AffixID),
				"value":   affixValue,
			}
			if affix.DisplayNameKey != "" {
				line["displayNameKey"] = affix.DisplayNameKey
			}
			if len(affix.DisplayNameArgs) > 0 {
				line["displayNameArgs"] = affix.DisplayNameArgs
			}
			affixLines = append(affixLines, line)
		}
		total := entry.Total()
		equipmentLines = append(equipmentLines, map[string]any{
			"instanceId":     entry.InstanceID,
			"templateItemId": entry.TemplateItemID,
			"total":          total,
			"base":           roundStatValue(entry.Base),
			"enhance":        roundStatValue(entry.Enhance),
			"affixTotal":     roundStatValue(affixTotal),
			"affixes":        affixLines,
		})
	}
	return equipmentLines
}
