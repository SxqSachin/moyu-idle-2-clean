package config

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	maxArenaChallengeChargeCapacity       = 100
	maxArenaChallengeTimingSeconds  int64 = 7 * 24 * 60 * 60
)

type GlobalSettings struct {
	Warehouse            WarehouseSettings            `json:"warehouse"`
	Activity             ActivitySettings             `json:"activity"`
	ActionQueue          ActionQueueSettings          `json:"actionQueue"`
	KittyJournal         KittyJournalSettings         `json:"kittyJournal"`
	Release              ReleaseSettings              `json:"release"`
	Admin                AdminSettings                `json:"admin"`
	Transport            TransportSettings            `json:"transport"`
	RateLimit            RateLimitSettings            `json:"rateLimit"`
	EmailDomainPolicy    EmailDomainPolicySettings    `json:"emailDomainPolicy"`
	Chat                 ChatSettings                 `json:"chat"`
	ContentModeration    ContentModerationSettings    `json:"contentModeration"`
	Battle               BattleSettings               `json:"battle"`
	Arena                ArenaSettings                `json:"arena"`
	CombatTrial          CombatTrialSettings          `json:"combatTrial"`
	Economy              EconomySettings              `json:"economy"`
	Rule                 RuleSettings                 `json:"rule"`
	Skill                SkillSettings                `json:"skill"`
	SkillForge           SkillForgeSettings           `json:"skillForge"`
	DerivedStats         DerivedStatsSettings         `json:"derivedStats"`
	CatVillageCommission CatVillageCommissionSettings `json:"catVillageCommission"`
	Alchemy              AlchemySettings              `json:"alchemy"`
	Fishing              FishingSettings              `json:"fishing"`
}

type AlchemySettings struct {
	InfusionBaseGoldCost           int64               `json:"infusionBaseGoldCost"`
	InfusionBaseDurationSec        int                 `json:"infusionBaseDurationSec"`
	InfusionBaseExp                float64             `json:"infusionBaseExp"`
	DurationsSec                   AlchemyDurationsSec `json:"durationsSec"`
	TransmuteDurationByRarityIndex []int               `json:"transmuteDurationByRarityIndex"`
}

type AlchemyDurationsSec struct {
	Default   int `json:"default"`
	Convert   int `json:"convert"`
	Bench     int `json:"bench"`
	Transmute int `json:"transmute"`
	Smelt     int `json:"smelt"`
}

type FishingSettings struct {
	Mutations []FishingMutationSettings `json:"mutations"`
}

type FishingMutationSettings struct {
	ID                  string                         `json:"id"`
	NameKey             string                         `json:"nameKey"`
	DescriptionKey      string                         `json:"descriptionKey"`
	VisualEffect        string                         `json:"visualEffect"`
	BadgeIcon           string                         `json:"badgeIcon"`
	AppearanceChance    float64                        `json:"appearanceChance"`
	InheritanceChance   float64                        `json:"inheritanceChance"`
	LifespanMultiplier  float64                        `json:"lifespanMultiplier"`
	SaleValueMultiplier float64                        `json:"saleValueMultiplier"`
	Alchemy             FishingMutationAlchemySettings `json:"alchemy"`
}

type FishingMutationAlchemySettings struct {
	PrimaryValueMultiplier float64                                `json:"primaryValueMultiplier"`
	BonusAttributes        []FishingMutationAlchemyBonusAttribute `json:"bonusAttributes"`
}

type FishingMutationAlchemyBonusAttribute struct {
	AttrKey string  `json:"attrKey"`
	Min     float64 `json:"min"`
	Max     float64 `json:"max"`
}

func (s AlchemySettings) DurationForAction(actionID string) int {
	switch actionID {
	case "alchemy.convert":
		if s.DurationsSec.Convert > 0 {
			return s.DurationsSec.Convert
		}
	case "alchemy.bench":
		if s.DurationsSec.Bench > 0 {
			return s.DurationsSec.Bench
		}
	case "alchemy.transmute":
		if s.DurationsSec.Transmute > 0 {
			return s.DurationsSec.Transmute
		}
	case "alchemy.smelt":
		if s.DurationsSec.Smelt > 0 {
			return s.DurationsSec.Smelt
		}
	}
	if s.DurationsSec.Default > 0 {
		return s.DurationsSec.Default
	}
	return 12
}

func (s AlchemySettings) TransmuteDurationByRarity(rarityOrder int) int {
	idx := rarityOrder
	if idx < 0 {
		idx = 0
	}
	if len(s.TransmuteDurationByRarityIndex) == 0 {
		return s.DurationForAction("alchemy.transmute")
	}
	if idx >= len(s.TransmuteDurationByRarityIndex) {
		idx = len(s.TransmuteDurationByRarityIndex) - 1
	}
	if d := s.TransmuteDurationByRarityIndex[idx]; d > 0 {
		return d
	}
	return s.DurationForAction("alchemy.transmute")
}

type CatVillageCommissionSettings struct {
	RefreshIntervalHours int     `json:"refreshIntervalHours"`
	RefreshBatchSize     int     `json:"refreshBatchSize"`
	MaxActiveTasks       int     `json:"maxActiveTasks"`
	MaxBattleTasks       int     `json:"maxBattleTasks"`
	RumorChancePerBatch  float64 `json:"rumorChancePerBatch"`
	MaxRumorsActive      int     `json:"maxRumorsActive"`
}

type TransportSettings struct {
	Compression CompressionSettings `json:"compression"`
}

type CompressionSettings struct {
	Enabled  bool   `json:"enabled"`
	MinBytes int    `json:"minBytes"`
	Level    int    `json:"level"`
	Format   string `json:"format"`
}

type WarehouseSettings struct {
	DefaultCols            int `json:"defaultCols"`
	DefaultRows            int `json:"defaultRows"`
	LogDefaultLookbackDays int `json:"logDefaultLookbackDays"`
}

type ActivitySettings struct {
	OfflineProgressMaxDurationSeconds int64 `json:"offlineProgressMaxDurationSeconds"`
}

type ActionQueueSettings struct {
	FastForward ActionQueueFastForwardSettings `json:"fastForward"`
}

type ActionQueueFastForwardSettings struct {
	MaxSteps  int   `json:"maxSteps"`
	MaxExecMs int64 `json:"maxExecMs"`
}

type KittyJournalSettings struct {
	Enabled              bool    `json:"enabled"`
	IntervalSeconds      int64   `json:"intervalSeconds"`
	JitterSeconds        int64   `json:"jitterSeconds"`
	DiaryProbability     float64 `json:"diaryProbability"`
	EncounterProbability float64 `json:"encounterProbability"`
	MaxEntriesPerKitty   int     `json:"maxEntriesPerKitty"`
}

type ReleaseSettings struct {
	BuildID                   string `json:"buildId"`
	DefaultReloadDelaySeconds int    `json:"defaultReloadDelaySeconds"`
}

type AdminSettings struct {
	UserBanReasons []AdminBanReason `json:"userBanReasons"`
}

type AdminBanReason struct {
	Key     string `json:"key"`
	NameKey string `json:"nameKey"`
}

type RateLimitSettings struct {
	API           FixedWindowRateLimitSettings `json:"api"`
	Register      FixedWindowRateLimitSettings `json:"register"`
	Login         FixedWindowRateLimitSettings `json:"login"`
	WSConnect     FixedWindowRateLimitSettings `json:"wsConnect"`
	ChatSend      FixedWindowRateLimitSettings `json:"chatSend"`
	LoginAttempt  LoginAttemptSettings         `json:"loginAttempt"`
	WSAuthAttempt LoginAttemptSettings         `json:"wsAuthAttempt"`
}

type FixedWindowRateLimitSettings struct {
	Max           int   `json:"max"`
	WindowSeconds int64 `json:"windowSeconds"`
}

type LoginAttemptSettings struct {
	MaxFailures          int   `json:"maxFailures"`
	LockDurationSeconds  int64 `json:"lockDurationSeconds"`
	FailureWindowSeconds int64 `json:"failureWindowSeconds"`
}

type EmailDomainPolicySettings struct {
	Enabled        bool     `json:"enabled"`
	BlocklistPath  string   `json:"blocklistPath"`
	BlockedDomains []string `json:"blockedDomains"`
	AllowedDomains []string `json:"allowedDomains"`
}

type ChatSettings struct {
	MaxContentLength    int           `json:"maxContentLength"`
	DefaultHistoryLimit int           `json:"defaultHistoryLimit"`
	Channels            []ChatChannel `json:"channels"`
}

type ChatChannel struct {
	ID             string `json:"id"`
	NameKey        string `json:"nameKey"`
	Order          int    `json:"order"`
	PlayerSendable bool   `json:"playerSendable"`
}

type ContentModerationSettings struct {
	Enabled            bool                            `json:"enabled"`
	VocabularyJSONPath string                          `json:"vocabularyJsonPath"`
	BlockMessageKey    string                          `json:"blockMessageKey"`
	Targets            ContentModerationTargetSettings `json:"targets"`
}

type ContentModerationTargetSettings struct {
	Chat           bool `json:"chat"`
	Username       bool `json:"username"`
	KittyName      bool `json:"kittyName"`
	BattleRoomName bool `json:"battleRoomName"`
	LoadoutName    bool `json:"loadoutName"`
	ItemCustomName bool `json:"itemCustomName"`
}

type BattleSettings struct {
	DropLuckFactor                     float64                       `json:"dropLuckFactor"`
	TickShardWorkers                   int                           `json:"tickShardWorkers"`
	TickClaimRoomsMin                  int                           `json:"tickClaimRoomsMin"`
	TickClaimRoomsMax                  int                           `json:"tickClaimRoomsMax"`
	TickTargetLeaseUtilPct             int                           `json:"tickTargetLeaseUtilPct"`
	CompileKickoffSlots                int                           `json:"compileKickoffSlots"`
	CompileTurnsPerPlayerUnit          int                           `json:"compileTurnsPerPlayerUnit"`
	CompileMaxTurns                    int                           `json:"compileMaxTurns"`
	CompileMaxExecMs                   int64                         `json:"compileMaxExecMs"`
	TimelinePushIntervalMs             int64                         `json:"timelinePushIntervalMs"`
	PlaybackFixedIntervalMs            int64                         `json:"playbackFixedIntervalMs"`
	PlaybackIntervalJitterTolerancePct int                           `json:"playbackIntervalJitterTolerancePct"`
	SettlementRequirePlaybackFinished  bool                          `json:"settlementRequirePlaybackFinished"`
	PlaybackMaxEventsBatch             int                           `json:"playbackMaxEventsBatch"`
	PlaybackMaxGapMs                   int64                         `json:"playbackMaxGapMs"`
	SettlementWorkers                  int                           `json:"settlementWorkers"`
	SkillExpWorkers                    int                           `json:"skillExpWorkers"`
	AttrExpWorkers                     int                           `json:"attrExpWorkers"`
	FinalizeSlots                      int                           `json:"finalizeSlots"`
	InterBattleIntervalMs              int64                         `json:"interBattleIntervalMs"`
	RecoveryDurationMs                 int64                         `json:"recoveryDurationMs"`
	RunHistoryKeepCount                int                           `json:"runHistoryKeepCount"`
	RoomNameMaxChars                   int                           `json:"roomNameMaxChars"`
	WorkerHeartbeat                    BattleWorkerHeartbeatSettings `json:"workerHeartbeat"`
	ExpBonusStartSeconds               int64                         `json:"expBonusStartSeconds"`
	ExpBonusPerSecond                  float64                       `json:"expBonusPerSecond"`
	ExpBonusMaxMultiplier              float64                       `json:"expBonusMaxMultiplier"`
	ResetItemID                        string                        `json:"resetItemId"`
	ResetItemCount                     int64                         `json:"resetItemCount"`
	// StateMachineV2 is retained as an explicit deployment assertion. V1
	// dispatch is retired, so configuration loading rejects false.
	StateMachineV2 bool `json:"stateMachineV2"`
}

type BattleWorkerHeartbeatSettings struct {
	TTLSeconds             int64 `json:"ttlSeconds"`
	RefreshIntervalSeconds int64 `json:"refreshIntervalSeconds"`
}

type ArenaSettings struct {
	Enabled                          bool                     `json:"enabled"`
	HistoryKeepCount                 int                      `json:"historyKeepCount"`
	TargetCount                      int                      `json:"targetCount"`
	NearbyTargetCount                int                      `json:"nearbyTargetCount"`
	RelaxedTargetCount               int                      `json:"relaxedTargetCount"`
	ChallengeChargeCapacity          int                      `json:"challengeChargeCapacity"`
	ChallengeChargeRecoverySeconds   int64                    `json:"challengeChargeRecoverySeconds"`
	OpponentChallengeCooldownSeconds int64                    `json:"opponentChallengeCooldownSeconds"`
	RecommendationRefreshSeconds     int64                    `json:"recommendationRefreshCooldownSeconds"`
	WorkerCount                      int                      `json:"workerCount"`
	WorkerPollIntervalMs             int64                    `json:"workerPollIntervalMs"`
	WorkerLeaseSeconds               int64                    `json:"workerLeaseSeconds"`
	MaxAttempts                      int                      `json:"maxAttempts"`
	DefaultRating                    int64                    `json:"defaultRating"`
	AttackerWinTokens                int64                    `json:"attackerWinTokens"`
	AttackerLossTokens               int64                    `json:"attackerLossTokens"`
	DefenderWinTokens                int64                    `json:"defenderWinTokens"`
	BountyPerDefenseWin              int64                    `json:"bountyPerDefenseWin"`
	BountyCap                        int64                    `json:"bountyCap"`
	AttackerWinRating                int64                    `json:"attackerWinRating"`
	AttackerLossRatingPenalty        int64                    `json:"attackerLossRatingPenalty"`
	DefenderWinRating                int64                    `json:"defenderWinRating"`
	DefenderLossRatingPenalty        int64                    `json:"defenderLossRatingPenalty"`
	ShopOffers                       []ArenaShopOfferSettings `json:"shopOffers"`
}

type CombatTrialSettings struct {
	Enabled              bool `json:"enabled"`
	HistoryKeepCount     int  `json:"historyKeepCount"`
	WorkerCount          int  `json:"workerCount"`
	WorkerPollIntervalMs int  `json:"workerPollIntervalMs"`
	WorkerLeaseSeconds   int  `json:"workerLeaseSeconds"`
	MaxAttempts          int  `json:"maxAttempts"`
}

const (
	minCombatTrialWorkerLeaseSeconds = 30
	maxCombatTrialWorkerLeaseSeconds = 3600
	maxCombatTrialWorkerPollMS       = 60_000
)

type ArenaShopOfferSettings struct {
	ID       string `json:"id"`
	ItemID   string `json:"itemId"`
	Quantity int64  `json:"quantity"`
	Cost     int64  `json:"cost"`
}

type EconomySettings struct {
	ItemRecycleGoldCoefficient               float64 `json:"itemRecycleGoldCoefficient"`
	ItemRecycleAffixMismatchExtraCoefficient float64 `json:"itemRecycleAffixMismatchExtraCoefficient"`
}

type RuleSettings struct {
	MaxGroups             int `json:"maxGroups"`
	MaxConditionsPerGroup int `json:"maxConditionsPerGroup"`
}

type SkillSettings struct {
	LevelUpBaseExp          int64 `json:"levelUpBaseExp"`
	LevelUpPerLevelExtraExp int64 `json:"levelUpPerLevelExtraExp"`
}

type SkillForgeSettings struct {
	CompilerVersion           int     `json:"compilerVersion"`
	RulesetVersion            int     `json:"rulesetVersion"`
	DisabledRulesetVersions   []int   `json:"disabledRulesetVersions"`
	BaseSuccessChance         float64 `json:"baseSuccessChance"`
	SourceSkillbookMinCount   int64   `json:"sourceSkillbookMinCount"`
	SourceSkillbookMaxCount   int64   `json:"sourceSkillbookMaxCount"`
	SourceSkillbookCountStep  int64   `json:"sourceSkillbookCountStep"`
	SourceSkillbookBonusStep  float64 `json:"sourceSkillbookBonusPerStep"`
	SourceSkillbookMaxBonus   float64 `json:"sourceSkillbookMaxBonus"`
	MaxCustomSkills           int     `json:"maxCustomSkills"`
	MaxDrafts                 int     `json:"maxDrafts"`
	MaxPendingResultsPerDraft int     `json:"maxPendingResultsPerDraft"`
	RecentHistoryLimit        int     `json:"recentHistoryLimit"`
	IdempotencyRetentionDays  int     `json:"idempotencyRetentionDays"`
	MaxNodes                  int     `json:"maxNodes"`
	MaxEdges                  int     `json:"maxEdges"`
	MaxEffects                int     `json:"maxEffects"`
	MaxFormulaTermsPerEffect  int     `json:"maxFormulaTermsPerEffect"`
	MaxTargetCount            int     `json:"maxTargetCount"`
	MaxHitCount               int     `json:"maxHitCount"`
	MinCooldown               int     `json:"minCooldown"`
	MaxCooldown               int     `json:"maxCooldown"`
	MaxGraphBytes             int     `json:"maxGraphBytes"`
	MaxTradeoffRefundRatio    float64 `json:"maxTradeoffRefundRatio"`
	StatusBaseApplyChance     float64 `json:"statusBaseApplyChance"`
	StatusApplyChancePerStep  float64 `json:"statusApplyChancePerStep"`
	CustomizedNodeSealItemID  string  `json:"customizedNodeSealItemId"`
	CustomizedNodeSealCount   int64   `json:"customizedNodeSealCount"`
	ForgeCatalystItemID       string  `json:"forgeCatalystItemId"`
	ForgeCatalystCount        int64   `json:"forgeCatalystCount"`
	GoldCostItemID            string  `json:"goldCostItemId"`
	GoldCostBase              int64   `json:"goldCostBase"`
	GoldCostPerMaterial       int64   `json:"goldCostPerMaterial"`
	GoldCostPerBudget         int64   `json:"goldCostPerBudget"`
}

// RulesetEnabled is the single policy gate for player-authored skill releases.
// Compiler versions describe format compatibility; ruleset versions can be
// disabled independently when a live balance or runtime issue is discovered.
func (s SkillForgeSettings) RulesetEnabled(version int) bool {
	if version <= 0 {
		return false
	}
	for _, disabled := range s.DisabledRulesetVersions {
		if disabled == version {
			return false
		}
	}
	return true
}

type DerivedStatsSettings struct {
	Version   string               `json:"version"`
	Operators []string             `json:"operators"`
	Formulas  []DerivedStatFormula `json:"formulas"`
}

type DerivedStatFormula struct {
	TargetAttr  string                   `json:"targetAttr"`
	IncludeBase bool                     `json:"includeBase"`
	Terms       []DerivedStatFormulaTerm `json:"terms"`
}

type DerivedStatFormulaTerm struct {
	AttrKey     string  `json:"attrKey"`
	Coefficient float64 `json:"coefficient"`
}

func DefaultGlobalSettings() GlobalSettings {
	return GlobalSettings{
		Warehouse: WarehouseSettings{
			DefaultCols:            25,
			DefaultRows:            10,
			LogDefaultLookbackDays: 7,
		},
		Activity: ActivitySettings{
			OfflineProgressMaxDurationSeconds: int64((24 * time.Hour) / time.Second),
		},
		ActionQueue: ActionQueueSettings{
			FastForward: ActionQueueFastForwardSettings{
				MaxSteps:  10_000,
				MaxExecMs: 1_000,
			},
		},
		KittyJournal: KittyJournalSettings{
			Enabled:              true,
			IntervalSeconds:      int64((30 * time.Minute) / time.Second),
			JitterSeconds:        int64((10 * time.Minute) / time.Second),
			DiaryProbability:     0.1,
			EncounterProbability: 0.1,
			MaxEntriesPerKitty:   200,
		},
		Release: ReleaseSettings{
			BuildID:                   "dev-local",
			DefaultReloadDelaySeconds: 120,
		},
		Admin: AdminSettings{
			UserBanReasons: []AdminBanReason{
				{Key: "abnormal_trade", NameKey: "admin.banReason.abnormalTrade"},
				{Key: "multi_account", NameKey: "admin.banReason.multiAccount"},
			},
		},
		Transport: TransportSettings{
			Compression: CompressionSettings{
				Enabled:  true,
				MinBytes: 512,
				Level:    1,
				Format:   "zlib-base64",
			},
		},
		RateLimit: RateLimitSettings{
			API: FixedWindowRateLimitSettings{
				Max:           120,
				WindowSeconds: int64(time.Minute / time.Second),
			},
			Register: FixedWindowRateLimitSettings{
				Max:           6,
				WindowSeconds: int64(time.Minute / time.Second),
			},
			Login: FixedWindowRateLimitSettings{
				Max:           12,
				WindowSeconds: int64(time.Minute / time.Second),
			},
			WSConnect: FixedWindowRateLimitSettings{
				Max:           30,
				WindowSeconds: int64(time.Minute / time.Second),
			},
			ChatSend: FixedWindowRateLimitSettings{
				Max:           10,
				WindowSeconds: 10,
			},
			LoginAttempt: LoginAttemptSettings{
				MaxFailures:          5,
				LockDurationSeconds:  int64((10 * time.Minute) / time.Second),
				FailureWindowSeconds: int64((10 * time.Minute) / time.Second),
			},
			WSAuthAttempt: LoginAttemptSettings{
				MaxFailures:          8,
				LockDurationSeconds:  int64((10 * time.Minute) / time.Second),
				FailureWindowSeconds: int64((10 * time.Minute) / time.Second),
			},
		},
		EmailDomainPolicy: EmailDomainPolicySettings{
			Enabled:        false,
			BlockedDomains: []string{},
			AllowedDomains: []string{},
		},
		Chat: ChatSettings{
			MaxContentLength:    300,
			DefaultHistoryLimit: 30,
			Channels: []ChatChannel{
				{ID: "world", NameKey: "chat.channel.world", Order: 10, PlayerSendable: true},
				{ID: "system", NameKey: "chat.channel.system", Order: 20, PlayerSendable: false},
				{ID: "life", NameKey: "chat.channel.life", Order: 30, PlayerSendable: true},
				{ID: "battle", NameKey: "chat.channel.battle", Order: 40, PlayerSendable: true},
			},
		},
		ContentModeration: ContentModerationSettings{
			Enabled:            false,
			VocabularyJSONPath: "../packages/Sensitive-lexicon-1.2/compiled/sensitive-words.json",
			BlockMessageKey:    "error.contentModeration.blocked",
			Targets: ContentModerationTargetSettings{
				Chat:           true,
				Username:       true,
				KittyName:      true,
				BattleRoomName: true,
				LoadoutName:    true,
				ItemCustomName: true,
			},
		},
		Battle: BattleSettings{
			DropLuckFactor:                     0,
			TickShardWorkers:                   6,
			TickClaimRoomsMin:                  16,
			TickClaimRoomsMax:                  32,
			TickTargetLeaseUtilPct:             60,
			CompileKickoffSlots:                4,
			CompileTurnsPerPlayerUnit:          100,
			CompileMaxTurns:                    300,
			CompileMaxExecMs:                   2500,
			TimelinePushIntervalMs:             900,
			PlaybackFixedIntervalMs:            900,
			PlaybackIntervalJitterTolerancePct: 10,
			SettlementRequirePlaybackFinished:  true,
			PlaybackMaxEventsBatch:             120,
			PlaybackMaxGapMs:                   3000,
			SettlementWorkers:                  4,
			SkillExpWorkers:                    2,
			AttrExpWorkers:                     4,
			FinalizeSlots:                      6,
			InterBattleIntervalMs:              3000,
			RecoveryDurationMs:                 3000,
			RunHistoryKeepCount:                10,
			RoomNameMaxChars:                   12,
			WorkerHeartbeat: BattleWorkerHeartbeatSettings{
				TTLSeconds:             30,
				RefreshIntervalSeconds: 5,
			},
			ExpBonusStartSeconds:  300,
			ExpBonusPerSecond:     0.001,
			ExpBonusMaxMultiplier: 3,
			ResetItemID:           "reset_scroll",
			ResetItemCount:        1,
			StateMachineV2:        true,
		},
		Arena: ArenaSettings{
			Enabled:                          true,
			HistoryKeepCount:                 50,
			TargetCount:                      12,
			NearbyTargetCount:                7,
			RelaxedTargetCount:               4,
			ChallengeChargeCapacity:          10,
			ChallengeChargeRecoverySeconds:   600,
			OpponentChallengeCooldownSeconds: 900,
			RecommendationRefreshSeconds:     300,
			WorkerCount:                      4,
			WorkerPollIntervalMs:             250,
			WorkerLeaseSeconds:               30,
			MaxAttempts:                      3,
			DefaultRating:                    1000,
			AttackerWinTokens:                20,
			AttackerLossTokens:               5,
			DefenderWinTokens:                5,
			BountyPerDefenseWin:              10,
			BountyCap:                        100,
			AttackerWinRating:                20,
			AttackerLossRatingPenalty:        5,
			DefenderWinRating:                10,
			DefenderLossRatingPenalty:        10,
			ShopOffers:                       []ArenaShopOfferSettings{},
		},
		CombatTrial: CombatTrialSettings{
			Enabled:              true,
			HistoryKeepCount:     50,
			WorkerCount:          2,
			WorkerPollIntervalMs: 250,
			WorkerLeaseSeconds:   30,
			MaxAttempts:          3,
		},
		Economy: EconomySettings{
			ItemRecycleGoldCoefficient:               0.5,
			ItemRecycleAffixMismatchExtraCoefficient: 0.6,
		},
		Rule: RuleSettings{
			MaxGroups:             6,
			MaxConditionsPerGroup: 6,
		},
		Skill: SkillSettings{
			LevelUpBaseExp:          100,
			LevelUpPerLevelExtraExp: 100,
		},
		SkillForge: SkillForgeSettings{
			CompilerVersion:           10,
			RulesetVersion:            1,
			DisabledRulesetVersions:   []int{},
			BaseSuccessChance:         0.30,
			SourceSkillbookMinCount:   200,
			SourceSkillbookMaxCount:   1000,
			SourceSkillbookCountStep:  100,
			SourceSkillbookBonusStep:  0.10,
			SourceSkillbookMaxBonus:   0.80,
			MaxCustomSkills:           12,
			MaxDrafts:                 4,
			MaxPendingResultsPerDraft: 1,
			RecentHistoryLimit:        50,
			IdempotencyRetentionDays:  30,
			MaxNodes:                  64,
			MaxEdges:                  96,
			MaxEffects:                3,
			MaxFormulaTermsPerEffect:  4,
			MaxTargetCount:            8,
			MaxHitCount:               8,
			MinCooldown:               1,
			MaxCooldown:               12,
			MaxGraphBytes:             128 * 1024,
			MaxTradeoffRefundRatio:    0.35,
			StatusBaseApplyChance:     0.25,
			StatusApplyChancePerStep:  0.10,
			CustomizedNodeSealItemID:  "spell_seal_lock_plate",
			CustomizedNodeSealCount:   10,
			ForgeCatalystItemID:       "final_volume_key_sigil",
			ForgeCatalystCount:        5,
			GoldCostItemID:            "gold_coin",
			GoldCostBase:              10_000,
			GoldCostPerMaterial:       20,
			GoldCostPerBudget:         5_000,
		},
		Alchemy: AlchemySettings{
			InfusionBaseGoldCost:    32,
			InfusionBaseDurationSec: 16,
			InfusionBaseExp:         2,
			DurationsSec: AlchemyDurationsSec{
				Default:   16,
				Convert:   14,
				Bench:     16,
				Transmute: 8,
				Smelt:     24,
			},
			TransmuteDurationByRarityIndex: []int{8, 9, 10, 11, 12, 12},
		},
		Fishing: FishingSettings{
			Mutations: []FishingMutationSettings{
				{
					ID: "prismatic", NameKey: "game.fishing.mutation_prismatic", DescriptionKey: "game.fishing.mutation_prismaticDesc", VisualEffect: "prismatic", BadgeIcon: "lucide:gem",
					AppearanceChance: 0.0025, InheritanceChance: 0.06, LifespanMultiplier: 1.8, SaleValueMultiplier: 2,
					Alchemy: FishingMutationAlchemySettings{PrimaryValueMultiplier: 1.5, BonusAttributes: []FishingMutationAlchemyBonusAttribute{{AttrKey: "alchemy_precision", Min: 0.018, Max: 0.032}}},
				},
				{
					ID: "rainbow", NameKey: "game.fishing.mutation_rainbow", DescriptionKey: "game.fishing.mutation_rainbowDesc", VisualEffect: "rainbow", BadgeIcon: "lucide:rainbow",
					AppearanceChance: 0.0015, InheritanceChance: 0.04, LifespanMultiplier: 1.6, SaleValueMultiplier: 2.5,
					Alchemy: FishingMutationAlchemySettings{PrimaryValueMultiplier: 1.35, BonusAttributes: []FishingMutationAlchemyBonusAttribute{{AttrKey: "alchemy_lucky_chance", Min: 0.012, Max: 0.025}}},
				},
				{
					ID: "shimmering", NameKey: "game.fishing.mutation_shimmering", DescriptionKey: "game.fishing.mutation_shimmeringDesc", VisualEffect: "shimmering", BadgeIcon: "lucide:sparkles",
					AppearanceChance: 0.005, InheritanceChance: 0.1, LifespanMultiplier: 1.35, SaleValueMultiplier: 1.5,
					Alchemy: FishingMutationAlchemySettings{PrimaryValueMultiplier: 1.25, BonusAttributes: []FishingMutationAlchemyBonusAttribute{{AttrKey: "battle_drop_rate_bonus", Min: 0.008, Max: 0.016}}},
				},
			},
		},
		DerivedStats: DerivedStatsSettings{
			Version:   "v2",
			Operators: []string{"+", "-", "*", "/"},
			Formulas: []DerivedStatFormula{
				{
					TargetAttr:  "physics_damage",
					IncludeBase: true,
					Terms: []DerivedStatFormulaTerm{
						{AttrKey: "strength", Coefficient: 0.6},
						{AttrKey: "dexterity", Coefficient: 0.4},
					},
				},
				{
					TargetAttr:  "magical_damage",
					IncludeBase: true,
					Terms: []DerivedStatFormulaTerm{
						{AttrKey: "intelligence", Coefficient: 1.2},
					},
				},
				{
					TargetAttr:  "speed",
					IncludeBase: true,
					Terms: []DerivedStatFormulaTerm{
						{AttrKey: "dexterity", Coefficient: 1},
					},
				},
				{
					TargetAttr:  "crit_damage",
					IncludeBase: true,
					Terms: []DerivedStatFormulaTerm{
						{AttrKey: "dexterity", Coefficient: 0.01},
					},
				},
				{
					TargetAttr:  "crit_chance",
					IncludeBase: true,
					Terms: []DerivedStatFormulaTerm{
						{AttrKey: "dexterity", Coefficient: 0.0025},
						{AttrKey: "luck", Coefficient: 0.005},
					},
				},
				{
					TargetAttr:  "hp",
					IncludeBase: true,
					Terms: []DerivedStatFormulaTerm{
						{AttrKey: "stamina", Coefficient: 10},
					},
				},
				{
					TargetAttr:  "hp_regen",
					IncludeBase: true,
					Terms: []DerivedStatFormulaTerm{
						{AttrKey: "stamina", Coefficient: 1},
					},
				},
				{
					TargetAttr:  "mp",
					IncludeBase: true,
					Terms: []DerivedStatFormulaTerm{
						{AttrKey: "intelligence", Coefficient: 5},
					},
				},
				{
					TargetAttr:  "mp_regen",
					IncludeBase: true,
					Terms: []DerivedStatFormulaTerm{
						{AttrKey: "intelligence", Coefficient: 0.5},
					},
				},
			},
		},
	}
}

func LoadGlobalSettings(path string) (GlobalSettings, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return GlobalSettings{}, fmt.Errorf("read global config %q: %w", path, err)
	}
	settings := DefaultGlobalSettings()
	if err = json.Unmarshal(raw, &settings); err != nil {
		return GlobalSettings{}, fmt.Errorf("parse global config %q: %w", path, err)
	}
	if settings.Warehouse.DefaultCols <= 0 {
		return GlobalSettings{}, fmt.Errorf("invalid warehouse.defaultCols in %q", path)
	}
	if settings.Warehouse.DefaultRows <= 0 {
		return GlobalSettings{}, fmt.Errorf("invalid warehouse.defaultRows in %q", path)
	}
	if settings.Warehouse.LogDefaultLookbackDays <= 0 {
		return GlobalSettings{}, fmt.Errorf("invalid warehouse.logDefaultLookbackDays in %q", path)
	}
	if settings.Activity.OfflineProgressMaxDurationSeconds <= 0 {
		return GlobalSettings{}, fmt.Errorf("invalid activity.offlineProgressMaxDurationSeconds in %q", path)
	}
	if settings.ActionQueue.FastForward.MaxSteps <= 0 {
		return GlobalSettings{}, fmt.Errorf("invalid actionQueue.fastForward.maxSteps in %q", path)
	}
	if settings.ActionQueue.FastForward.MaxExecMs <= 0 {
		return GlobalSettings{}, fmt.Errorf("invalid actionQueue.fastForward.maxExecMs in %q", path)
	}
	if settings.KittyJournal.IntervalSeconds <= 0 {
		return GlobalSettings{}, fmt.Errorf("invalid kittyJournal.intervalSeconds in %q", path)
	}
	if settings.KittyJournal.JitterSeconds < 0 {
		return GlobalSettings{}, fmt.Errorf("invalid kittyJournal.jitterSeconds in %q", path)
	}
	if settings.KittyJournal.DiaryProbability < 0 || settings.KittyJournal.DiaryProbability > 1 || math.IsNaN(settings.KittyJournal.DiaryProbability) || math.IsInf(settings.KittyJournal.DiaryProbability, 0) {
		return GlobalSettings{}, fmt.Errorf("invalid kittyJournal.diaryProbability in %q", path)
	}
	if settings.KittyJournal.EncounterProbability < 0 || settings.KittyJournal.EncounterProbability > 1 || math.IsNaN(settings.KittyJournal.EncounterProbability) || math.IsInf(settings.KittyJournal.EncounterProbability, 0) {
		return GlobalSettings{}, fmt.Errorf("invalid kittyJournal.encounterProbability in %q", path)
	}
	if settings.KittyJournal.MaxEntriesPerKitty <= 0 {
		return GlobalSettings{}, fmt.Errorf("invalid kittyJournal.maxEntriesPerKitty in %q", path)
	}
	settings.Release.BuildID = strings.TrimSpace(settings.Release.BuildID)
	if settings.Release.BuildID == "" {
		return GlobalSettings{}, fmt.Errorf("invalid release.buildId in %q", path)
	}
	if settings.Release.DefaultReloadDelaySeconds <= 0 || settings.Release.DefaultReloadDelaySeconds > 900 {
		return GlobalSettings{}, fmt.Errorf("invalid release.defaultReloadDelaySeconds in %q", path)
	}
	if len(settings.Admin.UserBanReasons) == 0 {
		return GlobalSettings{}, fmt.Errorf("invalid admin.userBanReasons in %q: empty", path)
	}
	adminBanReasonSeen := make(map[string]struct{}, len(settings.Admin.UserBanReasons))
	for idx := range settings.Admin.UserBanReasons {
		reason := &settings.Admin.UserBanReasons[idx]
		reason.Key = strings.TrimSpace(strings.ToLower(reason.Key))
		reason.NameKey = strings.TrimSpace(reason.NameKey)
		if reason.Key == "" {
			return GlobalSettings{}, fmt.Errorf("invalid admin.userBanReasons[%d].key in %q", idx, path)
		}
		if reason.NameKey == "" {
			return GlobalSettings{}, fmt.Errorf("invalid admin.userBanReasons[%d].nameKey in %q", idx, path)
		}
		if _, exists := adminBanReasonSeen[reason.Key]; exists {
			return GlobalSettings{}, fmt.Errorf("duplicate admin.userBanReasons key %q in %q", reason.Key, path)
		}
		adminBanReasonSeen[reason.Key] = struct{}{}
	}
	settings.Transport.Compression.Format = strings.TrimSpace(strings.ToLower(settings.Transport.Compression.Format))
	if settings.Transport.Compression.Format == "" {
		settings.Transport.Compression.Format = "zlib-base64"
	}
	if settings.Transport.Compression.Format != "zlib-base64" {
		return GlobalSettings{}, fmt.Errorf("invalid transport.compression.format in %q", path)
	}
	if settings.Transport.Compression.MinBytes <= 0 {
		return GlobalSettings{}, fmt.Errorf("invalid transport.compression.minBytes in %q", path)
	}
	if settings.Transport.Compression.Level < -2 || settings.Transport.Compression.Level > 9 {
		return GlobalSettings{}, fmt.Errorf("invalid transport.compression.level in %q", path)
	}
	if settings.RateLimit.API.Max <= 0 || settings.RateLimit.API.WindowSeconds <= 0 {
		return GlobalSettings{}, fmt.Errorf("invalid rateLimit.api in %q", path)
	}
	if settings.RateLimit.Register.Max <= 0 || settings.RateLimit.Register.WindowSeconds <= 0 {
		return GlobalSettings{}, fmt.Errorf("invalid rateLimit.register in %q", path)
	}
	if settings.RateLimit.Login.Max <= 0 || settings.RateLimit.Login.WindowSeconds <= 0 {
		return GlobalSettings{}, fmt.Errorf("invalid rateLimit.login in %q", path)
	}
	if settings.RateLimit.WSConnect.Max <= 0 || settings.RateLimit.WSConnect.WindowSeconds <= 0 {
		return GlobalSettings{}, fmt.Errorf("invalid rateLimit.wsConnect in %q", path)
	}
	if settings.RateLimit.ChatSend.Max <= 0 || settings.RateLimit.ChatSend.Max > 100 ||
		settings.RateLimit.ChatSend.WindowSeconds <= 0 || settings.RateLimit.ChatSend.WindowSeconds > 300 {
		return GlobalSettings{}, fmt.Errorf("invalid rateLimit.chatSend in %q", path)
	}
	if settings.RateLimit.LoginAttempt.MaxFailures <= 0 {
		return GlobalSettings{}, fmt.Errorf("invalid rateLimit.loginAttempt.maxFailures in %q", path)
	}
	if settings.RateLimit.LoginAttempt.LockDurationSeconds <= 0 {
		return GlobalSettings{}, fmt.Errorf("invalid rateLimit.loginAttempt.lockDurationSeconds in %q", path)
	}
	if settings.RateLimit.LoginAttempt.FailureWindowSeconds <= 0 {
		return GlobalSettings{}, fmt.Errorf("invalid rateLimit.loginAttempt.failureWindowSeconds in %q", path)
	}
	if settings.RateLimit.WSAuthAttempt.MaxFailures <= 0 {
		return GlobalSettings{}, fmt.Errorf("invalid rateLimit.wsAuthAttempt.maxFailures in %q", path)
	}
	if settings.RateLimit.WSAuthAttempt.LockDurationSeconds <= 0 {
		return GlobalSettings{}, fmt.Errorf("invalid rateLimit.wsAuthAttempt.lockDurationSeconds in %q", path)
	}
	if settings.RateLimit.WSAuthAttempt.FailureWindowSeconds <= 0 {
		return GlobalSettings{}, fmt.Errorf("invalid rateLimit.wsAuthAttempt.failureWindowSeconds in %q", path)
	}
	settings.EmailDomainPolicy.BlocklistPath = strings.TrimSpace(settings.EmailDomainPolicy.BlocklistPath)
	settings.EmailDomainPolicy.BlockedDomains = normalizeEmailDomainEntries(settings.EmailDomainPolicy.BlockedDomains)
	settings.EmailDomainPolicy.AllowedDomains = normalizeEmailDomainEntries(settings.EmailDomainPolicy.AllowedDomains)
	if settings.EmailDomainPolicy.Enabled && settings.EmailDomainPolicy.BlocklistPath == "" && len(settings.EmailDomainPolicy.BlockedDomains) == 0 {
		return GlobalSettings{}, fmt.Errorf("invalid emailDomainPolicy in %q: no blocklist configured", path)
	}
	if settings.Chat.MaxContentLength <= 0 || settings.Chat.MaxContentLength > 2_000 {
		return GlobalSettings{}, fmt.Errorf("invalid chat.maxContentLength in %q", path)
	}
	if settings.Chat.DefaultHistoryLimit <= 0 || settings.Chat.DefaultHistoryLimit > 500 {
		return GlobalSettings{}, fmt.Errorf("invalid chat.defaultHistoryLimit in %q", path)
	}
	settings.ContentModeration.VocabularyJSONPath = strings.TrimSpace(settings.ContentModeration.VocabularyJSONPath)
	settings.ContentModeration.BlockMessageKey = strings.TrimSpace(settings.ContentModeration.BlockMessageKey)
	if settings.ContentModeration.Enabled {
		if settings.ContentModeration.VocabularyJSONPath == "" {
			return GlobalSettings{}, fmt.Errorf("invalid contentModeration.vocabularyJsonPath in %q", path)
		}
		if settings.ContentModeration.BlockMessageKey == "" {
			return GlobalSettings{}, fmt.Errorf("invalid contentModeration.blockMessageKey in %q", path)
		}
	}
	if settings.Battle.ExpBonusStartSeconds < 0 {
		return GlobalSettings{}, fmt.Errorf("invalid battle.expBonusStartSeconds in %q", path)
	}
	if settings.Battle.TickShardWorkers <= 0 || settings.Battle.TickShardWorkers > 128 {
		return GlobalSettings{}, fmt.Errorf("invalid battle.tickShardWorkers in %q", path)
	}
	if settings.Battle.TickClaimRoomsMin <= 0 || settings.Battle.TickClaimRoomsMin > 10_000 {
		return GlobalSettings{}, fmt.Errorf("invalid battle.tickClaimRoomsMin in %q", path)
	}
	if settings.Battle.TickClaimRoomsMax < settings.Battle.TickClaimRoomsMin || settings.Battle.TickClaimRoomsMax > 10_000 {
		return GlobalSettings{}, fmt.Errorf("invalid battle.tickClaimRoomsMax in %q", path)
	}
	if settings.Battle.TickTargetLeaseUtilPct <= 0 || settings.Battle.TickTargetLeaseUtilPct >= 100 {
		return GlobalSettings{}, fmt.Errorf("invalid battle.tickTargetLeaseUtilPct in %q", path)
	}
	if settings.Battle.CompileMaxTurns <= 0 || settings.Battle.CompileMaxTurns > 10_000 {
		return GlobalSettings{}, fmt.Errorf("invalid battle.compileMaxTurns in %q", path)
	}
	if settings.Battle.CompileTurnsPerPlayerUnit <= 0 || settings.Battle.CompileTurnsPerPlayerUnit > 10_000 {
		return GlobalSettings{}, fmt.Errorf("invalid battle.compileTurnsPerPlayerUnit in %q", path)
	}
	if settings.Battle.CompileKickoffSlots <= 0 || settings.Battle.CompileKickoffSlots > 128 {
		return GlobalSettings{}, fmt.Errorf("invalid battle.compileKickoffSlots in %q", path)
	}
	if settings.Battle.CompileMaxExecMs <= 0 || settings.Battle.CompileMaxExecMs > 60_000 {
		return GlobalSettings{}, fmt.Errorf("invalid battle.compileMaxExecMs in %q", path)
	}
	if settings.Battle.TimelinePushIntervalMs <= 0 {
		return GlobalSettings{}, fmt.Errorf("invalid battle.timelinePushIntervalMs in %q", path)
	}
	if settings.Battle.PlaybackFixedIntervalMs <= 0 {
		settings.Battle.PlaybackFixedIntervalMs = settings.Battle.TimelinePushIntervalMs
	}
	if settings.Battle.PlaybackFixedIntervalMs <= 0 {
		return GlobalSettings{}, fmt.Errorf("invalid battle.playbackFixedIntervalMs in %q", path)
	}
	if settings.Battle.PlaybackIntervalJitterTolerancePct < 0 || settings.Battle.PlaybackIntervalJitterTolerancePct > 100 {
		return GlobalSettings{}, fmt.Errorf("invalid battle.playbackIntervalJitterTolerancePct in %q", path)
	}
	if !settings.Battle.SettlementRequirePlaybackFinished {
		return GlobalSettings{}, fmt.Errorf("invalid battle.settlementRequirePlaybackFinished in %q: must be true", path)
	}
	if settings.Battle.PlaybackMaxEventsBatch <= 0 || settings.Battle.PlaybackMaxEventsBatch > 10_000 {
		return GlobalSettings{}, fmt.Errorf("invalid battle.playbackMaxEventsBatch in %q", path)
	}
	if settings.Battle.PlaybackMaxGapMs <= 0 {
		return GlobalSettings{}, fmt.Errorf("invalid battle.playbackMaxGapMs in %q", path)
	}
	if settings.Battle.SettlementWorkers <= 0 || settings.Battle.SettlementWorkers > 64 {
		return GlobalSettings{}, fmt.Errorf("invalid battle.settlementWorkers in %q", path)
	}
	if settings.Battle.SkillExpWorkers <= 0 || settings.Battle.SkillExpWorkers > 64 {
		return GlobalSettings{}, fmt.Errorf("invalid battle.skillExpWorkers in %q", path)
	}
	if settings.Battle.AttrExpWorkers <= 0 || settings.Battle.AttrExpWorkers > 64 {
		return GlobalSettings{}, fmt.Errorf("invalid battle.attrExpWorkers in %q", path)
	}
	if !settings.Battle.StateMachineV2 {
		return GlobalSettings{}, fmt.Errorf("invalid battle.stateMachineV2 in %q: v1 dispatch is retired", path)
	}
	if settings.Battle.FinalizeSlots <= 0 || settings.Battle.FinalizeSlots > 128 {
		return GlobalSettings{}, fmt.Errorf("invalid battle.finalizeSlots in %q", path)
	}
	if settings.Battle.InterBattleIntervalMs < 0 {
		return GlobalSettings{}, fmt.Errorf("invalid battle.interBattleIntervalMs in %q", path)
	}
	if settings.Battle.RecoveryDurationMs < 0 {
		return GlobalSettings{}, fmt.Errorf("invalid battle.recoveryDurationMs in %q", path)
	}
	if settings.Battle.RunHistoryKeepCount <= 0 {
		return GlobalSettings{}, fmt.Errorf("invalid battle.runHistoryKeepCount in %q", path)
	}
	if settings.Battle.RoomNameMaxChars <= 0 || settings.Battle.RoomNameMaxChars > 64 {
		return GlobalSettings{}, fmt.Errorf("invalid battle.roomNameMaxChars in %q", path)
	}
	if settings.Battle.WorkerHeartbeat.TTLSeconds <= 0 {
		return GlobalSettings{}, fmt.Errorf("invalid battle.workerHeartbeat.ttlSeconds in %q", path)
	}
	if settings.Battle.WorkerHeartbeat.RefreshIntervalSeconds <= 0 {
		return GlobalSettings{}, fmt.Errorf("invalid battle.workerHeartbeat.refreshIntervalSeconds in %q", path)
	}
	if settings.Battle.WorkerHeartbeat.RefreshIntervalSeconds > settings.Battle.WorkerHeartbeat.TTLSeconds {
		return GlobalSettings{}, fmt.Errorf("invalid battle.workerHeartbeat interval > ttl in %q", path)
	}
	if settings.Arena.HistoryKeepCount <= 0 || settings.Arena.HistoryKeepCount > 50 {
		return GlobalSettings{}, fmt.Errorf("invalid arena.historyKeepCount in %q", path)
	}
	if settings.Arena.TargetCount <= 0 || settings.Arena.TargetCount > 50 ||
		settings.Arena.NearbyTargetCount < 0 || settings.Arena.RelaxedTargetCount < 0 ||
		settings.Arena.NearbyTargetCount+settings.Arena.RelaxedTargetCount > settings.Arena.TargetCount {
		return GlobalSettings{}, fmt.Errorf("invalid arena target counts in %q", path)
	}
	if settings.Arena.ChallengeChargeCapacity <= 0 || settings.Arena.ChallengeChargeCapacity > maxArenaChallengeChargeCapacity ||
		settings.Arena.ChallengeChargeRecoverySeconds <= 0 || settings.Arena.ChallengeChargeRecoverySeconds > maxArenaChallengeTimingSeconds ||
		settings.Arena.OpponentChallengeCooldownSeconds < 0 || settings.Arena.OpponentChallengeCooldownSeconds > maxArenaChallengeTimingSeconds ||
		settings.Arena.RecommendationRefreshSeconds <= 0 || settings.Arena.RecommendationRefreshSeconds > maxArenaChallengeTimingSeconds ||
		settings.Arena.WorkerCount <= 0 || settings.Arena.WorkerCount > 32 || settings.Arena.WorkerPollIntervalMs <= 0 || settings.Arena.WorkerLeaseSeconds <= 0 || settings.Arena.MaxAttempts <= 0 {
		return GlobalSettings{}, fmt.Errorf("invalid arena worker or cooldown settings in %q", path)
	}
	if settings.Arena.DefaultRating < 0 {
		return GlobalSettings{}, fmt.Errorf("invalid arena default rating in %q", path)
	}
	if settings.Arena.AttackerWinTokens < 0 || settings.Arena.AttackerLossTokens < 0 || settings.Arena.DefenderWinTokens < 0 || settings.Arena.BountyPerDefenseWin < 0 || settings.Arena.BountyCap < settings.Arena.BountyPerDefenseWin {
		return GlobalSettings{}, fmt.Errorf("invalid arena token rewards in %q", path)
	}
	if settings.Arena.AttackerWinRating < 0 || settings.Arena.AttackerLossRatingPenalty < 0 || settings.Arena.DefenderWinRating < 0 || settings.Arena.DefenderLossRatingPenalty < 0 {
		return GlobalSettings{}, fmt.Errorf("invalid arena rating rewards in %q", path)
	}
	if settings.CombatTrial.HistoryKeepCount <= 0 || settings.CombatTrial.HistoryKeepCount > 50 ||
		settings.CombatTrial.WorkerCount <= 0 || settings.CombatTrial.WorkerCount > 32 ||
		settings.CombatTrial.WorkerPollIntervalMs <= 0 || settings.CombatTrial.WorkerPollIntervalMs > maxCombatTrialWorkerPollMS ||
		settings.CombatTrial.WorkerLeaseSeconds < minCombatTrialWorkerLeaseSeconds || settings.CombatTrial.WorkerLeaseSeconds > maxCombatTrialWorkerLeaseSeconds ||
		settings.CombatTrial.MaxAttempts <= 0 {
		return GlobalSettings{}, fmt.Errorf("invalid combatTrial worker or history settings in %q", path)
	}
	seenArenaOffers := make(map[string]struct{}, len(settings.Arena.ShopOffers))
	for _, offer := range settings.Arena.ShopOffers {
		id := strings.TrimSpace(offer.ID)
		itemID := strings.TrimSpace(offer.ItemID)
		if id == "" || itemID == "" || offer.Quantity <= 0 || offer.Cost <= 0 {
			return GlobalSettings{}, fmt.Errorf("invalid arena shop offer in %q", path)
		}
		if _, exists := seenArenaOffers[id]; exists {
			return GlobalSettings{}, fmt.Errorf("duplicate arena shop offer %q in %q", id, path)
		}
		seenArenaOffers[id] = struct{}{}
	}
	if settings.Battle.ExpBonusPerSecond < 0 || math.IsNaN(settings.Battle.ExpBonusPerSecond) || math.IsInf(settings.Battle.ExpBonusPerSecond, 0) {
		return GlobalSettings{}, fmt.Errorf("invalid battle.expBonusPerSecond in %q", path)
	}
	if settings.Battle.ExpBonusMaxMultiplier < 1 || math.IsNaN(settings.Battle.ExpBonusMaxMultiplier) || math.IsInf(settings.Battle.ExpBonusMaxMultiplier, 0) {
		return GlobalSettings{}, fmt.Errorf("invalid battle.expBonusMaxMultiplier in %q", path)
	}
	if settings.Economy.ItemRecycleGoldCoefficient <= 0 || math.IsNaN(settings.Economy.ItemRecycleGoldCoefficient) || math.IsInf(settings.Economy.ItemRecycleGoldCoefficient, 0) {
		return GlobalSettings{}, fmt.Errorf("invalid economy.itemRecycleGoldCoefficient in %q", path)
	}
	if settings.Economy.ItemRecycleAffixMismatchExtraCoefficient <= 0 || math.IsNaN(settings.Economy.ItemRecycleAffixMismatchExtraCoefficient) || math.IsInf(settings.Economy.ItemRecycleAffixMismatchExtraCoefficient, 0) {
		return GlobalSettings{}, fmt.Errorf("invalid economy.itemRecycleAffixMismatchExtraCoefficient in %q", path)
	}
	if settings.Economy.ItemRecycleAffixMismatchExtraCoefficient > 1 {
		return GlobalSettings{}, fmt.Errorf("invalid economy.itemRecycleAffixMismatchExtraCoefficient > 1 in %q", path)
	}
	if settings.Economy.ItemRecycleGoldCoefficient+settings.Economy.ItemRecycleAffixMismatchExtraCoefficient <= 1 {
		return GlobalSettings{}, fmt.Errorf("invalid economy.itemRecycleGoldCoefficient + itemRecycleAffixMismatchExtraCoefficient <= 1 in %q", path)
	}
	settings.Battle.ResetItemID = strings.TrimSpace(settings.Battle.ResetItemID)
	if settings.Battle.ResetItemID == "" {
		return GlobalSettings{}, fmt.Errorf("invalid battle.resetItemId in %q", path)
	}
	if settings.Battle.ResetItemCount <= 0 {
		return GlobalSettings{}, fmt.Errorf("invalid battle.resetItemCount in %q", path)
	}
	if settings.Rule.MaxGroups <= 0 {
		return GlobalSettings{}, fmt.Errorf("invalid rule.maxGroups in %q", path)
	}
	if settings.Rule.MaxConditionsPerGroup <= 0 {
		return GlobalSettings{}, fmt.Errorf("invalid rule.maxConditionsPerGroup in %q", path)
	}
	if settings.Skill.LevelUpBaseExp <= 0 {
		return GlobalSettings{}, fmt.Errorf("invalid skill.levelUpBaseExp in %q", path)
	}
	if settings.Skill.LevelUpPerLevelExtraExp <= 0 {
		return GlobalSettings{}, fmt.Errorf("invalid skill.levelUpPerLevelExtraExp in %q", path)
	}
	forge := settings.SkillForge
	if forge.CompilerVersion <= 0 || forge.RulesetVersion <= 0 || forge.MaxCustomSkills <= 0 || forge.MaxDrafts <= 0 || forge.MaxPendingResultsPerDraft <= 0 ||
		forge.RecentHistoryLimit <= 0 || forge.IdempotencyRetentionDays <= 0 ||
		math.IsNaN(forge.BaseSuccessChance) || math.IsInf(forge.BaseSuccessChance, 0) || forge.BaseSuccessChance <= 0 || forge.BaseSuccessChance >= 1 ||
		forge.SourceSkillbookMinCount <= 0 || forge.SourceSkillbookMaxCount < forge.SourceSkillbookMinCount || forge.SourceSkillbookCountStep <= 0 ||
		(forge.SourceSkillbookMaxCount-forge.SourceSkillbookMinCount)%forge.SourceSkillbookCountStep != 0 ||
		math.IsNaN(forge.SourceSkillbookBonusStep) || math.IsInf(forge.SourceSkillbookBonusStep, 0) || forge.SourceSkillbookBonusStep < 0 ||
		math.IsNaN(forge.SourceSkillbookMaxBonus) || math.IsInf(forge.SourceSkillbookMaxBonus, 0) || forge.SourceSkillbookMaxBonus < 0 || forge.SourceSkillbookMaxBonus > 1 ||
		forge.MaxNodes <= 0 || forge.MaxEdges <= 0 || forge.MaxEffects <= 0 || forge.MaxFormulaTermsPerEffect <= 0 ||
		forge.MaxTargetCount <= 0 || forge.MaxHitCount <= 0 || forge.MinCooldown < 1 || forge.MaxCooldown < forge.MinCooldown || forge.MaxGraphBytes <= 0 ||
		math.IsNaN(forge.StatusBaseApplyChance) || math.IsInf(forge.StatusBaseApplyChance, 0) || forge.StatusBaseApplyChance <= 0 || forge.StatusBaseApplyChance > 1 ||
		math.IsNaN(forge.StatusApplyChancePerStep) || math.IsInf(forge.StatusApplyChancePerStep, 0) || forge.StatusApplyChancePerStep <= 0 || forge.StatusApplyChancePerStep > 1 ||
		strings.TrimSpace(forge.CustomizedNodeSealItemID) == "" || forge.CustomizedNodeSealCount <= 0 ||
		strings.TrimSpace(forge.ForgeCatalystItemID) == "" || forge.ForgeCatalystCount <= 0 ||
		strings.TrimSpace(forge.GoldCostItemID) == "" || forge.GoldCostBase < 0 || forge.GoldCostPerMaterial < 0 || forge.GoldCostPerBudget < 0 ||
		math.IsNaN(forge.MaxTradeoffRefundRatio) || math.IsInf(forge.MaxTradeoffRefundRatio, 0) ||
		forge.MaxTradeoffRefundRatio < 0 || forge.MaxTradeoffRefundRatio > 1 {
		return GlobalSettings{}, fmt.Errorf("invalid skillForge limits in %q", path)
	}
	disabledRulesets := make(map[int]struct{}, len(forge.DisabledRulesetVersions))
	for _, version := range forge.DisabledRulesetVersions {
		if version <= 0 {
			return GlobalSettings{}, fmt.Errorf("invalid skillForge.disabledRulesetVersions in %q", path)
		}
		if _, exists := disabledRulesets[version]; exists {
			return GlobalSettings{}, fmt.Errorf("duplicate skillForge.disabledRulesetVersions entry %d in %q", version, path)
		}
		disabledRulesets[version] = struct{}{}
	}
	if len(settings.Chat.Channels) == 0 {
		return GlobalSettings{}, fmt.Errorf("invalid chat.channels in %q: empty", path)
	}
	channelIDs := make(map[string]struct{}, len(settings.Chat.Channels))
	playerSendableCount := 0
	for i := range settings.Chat.Channels {
		ch := &settings.Chat.Channels[i]
		ch.ID = strings.TrimSpace(strings.ToLower(ch.ID))
		ch.NameKey = strings.TrimSpace(ch.NameKey)
		if ch.ID == "" {
			return GlobalSettings{}, fmt.Errorf("invalid chat.channels[%d].id in %q", i, path)
		}
		if ch.NameKey == "" {
			return GlobalSettings{}, fmt.Errorf("invalid chat.channels[%d].nameKey in %q", i, path)
		}
		if _, exists := channelIDs[ch.ID]; exists {
			return GlobalSettings{}, fmt.Errorf("duplicate chat.channels id %q in %q", ch.ID, path)
		}
		channelIDs[ch.ID] = struct{}{}
		if ch.PlayerSendable {
			playerSendableCount++
		}
	}
	if playerSendableCount == 0 {
		return GlobalSettings{}, fmt.Errorf("invalid chat.channels in %q: no playerSendable channel", path)
	}
	sort.Slice(settings.Chat.Channels, func(i, j int) bool {
		if settings.Chat.Channels[i].Order != settings.Chat.Channels[j].Order {
			return settings.Chat.Channels[i].Order < settings.Chat.Channels[j].Order
		}
		return settings.Chat.Channels[i].ID < settings.Chat.Channels[j].ID
	})
	if err := normalizeFishingSettings(&settings.Fishing, path); err != nil {
		return GlobalSettings{}, err
	}
	settings.DerivedStats.Version = strings.TrimSpace(settings.DerivedStats.Version)
	if settings.DerivedStats.Version == "" {
		return GlobalSettings{}, fmt.Errorf("invalid derivedStats.version in %q", path)
	}
	if len(settings.DerivedStats.Operators) == 0 {
		return GlobalSettings{}, fmt.Errorf("invalid derivedStats.operators in %q: empty", path)
	}
	operators := make([]string, 0, len(settings.DerivedStats.Operators))
	for i, operator := range settings.DerivedStats.Operators {
		operator = strings.TrimSpace(operator)
		if operator == "" {
			return GlobalSettings{}, fmt.Errorf("invalid derivedStats.operators[%d] in %q", i, path)
		}
		operators = append(operators, operator)
	}
	settings.DerivedStats.Operators = operators
	if len(settings.DerivedStats.Formulas) == 0 {
		return GlobalSettings{}, fmt.Errorf("invalid derivedStats.formulas in %q: empty", path)
	}
	targetSeen := make(map[string]struct{}, len(settings.DerivedStats.Formulas))
	for i := range settings.DerivedStats.Formulas {
		formula := &settings.DerivedStats.Formulas[i]
		formula.TargetAttr = strings.TrimSpace(formula.TargetAttr)
		if formula.TargetAttr == "" {
			return GlobalSettings{}, fmt.Errorf("invalid derivedStats.formulas[%d].targetAttr in %q", i, path)
		}
		if _, exists := targetSeen[formula.TargetAttr]; exists {
			return GlobalSettings{}, fmt.Errorf("duplicate derivedStats.formulas targetAttr %q in %q", formula.TargetAttr, path)
		}
		targetSeen[formula.TargetAttr] = struct{}{}
		if len(formula.Terms) == 0 {
			return GlobalSettings{}, fmt.Errorf("invalid derivedStats.formulas[%d].terms in %q: empty", i, path)
		}
		for j := range formula.Terms {
			term := &formula.Terms[j]
			term.AttrKey = strings.TrimSpace(term.AttrKey)
			if term.AttrKey == "" {
				return GlobalSettings{}, fmt.Errorf("invalid derivedStats.formulas[%d].terms[%d].attrKey in %q", i, j, path)
			}
			if math.IsNaN(term.Coefficient) || math.IsInf(term.Coefficient, 0) || term.Coefficient == 0 {
				return GlobalSettings{}, fmt.Errorf("invalid derivedStats.formulas[%d].terms[%d].coefficient in %q", i, j, path)
			}
		}
	}
	return settings, nil
}

func normalizeEmailDomainEntries(values []string) []string {
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, raw := range values {
		value := strings.ToLower(strings.TrimSpace(raw))
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func normalizeFishingSettings(settings *FishingSettings, path string) error {
	if settings == nil {
		return nil
	}
	seen := make(map[string]struct{}, len(settings.Mutations))
	for i := range settings.Mutations {
		mutation := &settings.Mutations[i]
		mutation.ID = strings.ToLower(strings.TrimSpace(mutation.ID))
		mutation.NameKey = strings.TrimSpace(mutation.NameKey)
		mutation.DescriptionKey = strings.TrimSpace(mutation.DescriptionKey)
		mutation.VisualEffect = strings.ToLower(strings.TrimSpace(mutation.VisualEffect))
		mutation.BadgeIcon = strings.ToLower(strings.TrimSpace(mutation.BadgeIcon))
		if mutation.BadgeIcon == "" {
			mutation.BadgeIcon = "lucide:sparkles"
		}
		if mutation.SaleValueMultiplier == 0 {
			mutation.SaleValueMultiplier = 1
		}
		if !isFishingMutationIdentifier(mutation.ID) {
			return fmt.Errorf("invalid fishing.mutations[%d].id in %q", i, path)
		}
		if _, exists := seen[mutation.ID]; exists {
			return fmt.Errorf("duplicate fishing.mutations id %q in %q", mutation.ID, path)
		}
		seen[mutation.ID] = struct{}{}
		if mutation.NameKey == "" || mutation.DescriptionKey == "" || !isFishingMutationIdentifier(mutation.VisualEffect) || !isFishingMutationIcon(mutation.BadgeIcon) {
			return fmt.Errorf("invalid fishing.mutations[%d] presentation in %q", i, path)
		}
		if !finiteProbability(mutation.AppearanceChance) || !finiteProbability(mutation.InheritanceChance) {
			return fmt.Errorf("invalid fishing.mutations[%d] probability in %q", i, path)
		}
		if !finiteInRange(mutation.LifespanMultiplier, 1, 10) || !finiteInRange(mutation.SaleValueMultiplier, 1, 100) || !finiteInRange(mutation.Alchemy.PrimaryValueMultiplier, 1, 10) {
			return fmt.Errorf("invalid fishing.mutations[%d] multiplier in %q", i, path)
		}
		for j := range mutation.Alchemy.BonusAttributes {
			bonus := &mutation.Alchemy.BonusAttributes[j]
			bonus.AttrKey = strings.ToLower(strings.TrimSpace(bonus.AttrKey))
			if !isFishingMutationIdentifier(bonus.AttrKey) || !finiteInRange(bonus.Min, 0, 1_000_000) || !finiteInRange(bonus.Max, bonus.Min, 1_000_000) {
				return fmt.Errorf("invalid fishing.mutations[%d].alchemy.bonusAttributes[%d] in %q", i, j, path)
			}
		}
	}
	return nil
}

func finiteProbability(value float64) bool {
	return finiteInRange(value, 0, 1)
}

func finiteInRange(value, minimum, maximum float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= minimum && value <= maximum
}

func isFishingMutationIdentifier(value string) bool {
	if value == "" {
		return false
	}
	for _, char := range value {
		if (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') || char == '_' || char == '-' {
			continue
		}
		return false
	}
	return true
}

func isFishingMutationIcon(value string) bool {
	if len(value) > 80 {
		return false
	}
	parts := strings.Split(value, ":")
	return len(parts) == 2 && isFishingMutationIdentifier(parts[0]) && isFishingMutationIdentifier(parts[1])
}

func BuildDerivedFormulaExpression(terms []DerivedStatFormulaTerm) string {
	parts := make([]string, 0, len(terms))
	for _, term := range terms {
		attrKey := strings.TrimSpace(term.AttrKey)
		if attrKey == "" {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s*%s", attrKey, formatFormulaNumber(term.Coefficient)))
	}
	return strings.Join(parts, " + ")
}

func formatFormulaNumber(value float64) string {
	value = math.Round(value*10000) / 10000
	return strconv.FormatFloat(value, 'f', -1, 64)
}
