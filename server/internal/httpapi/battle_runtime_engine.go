package httpapi

// TODO(phase1-extract): move battle runtime orchestration/state machine from httpapi
// into internal/battle/runtime and keep httpapi as protocol adapter only.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"maps"
	"math"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	battlecore "mysecondwebgame/server/internal/battle/core"
	battleskills "mysecondwebgame/server/internal/battle/skills"
	"mysecondwebgame/server/internal/operationeffect"
	"mysecondwebgame/server/internal/rules"
	"mysecondwebgame/server/internal/store"
	"mysecondwebgame/server/internal/wsbus"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/redis/go-redis/v9"
)

type battleRuntimeState struct {
	dominationBySkill       map[string][]string
	executingEquipmentCombo bool
	PassiveSources          map[int64]operationeffect.PassiveSnapshot `json:"passiveSources,omitempty"`
	PassiveRunID            int64                                     `json:"passiveRunId,omitempty"`

	BattleKind                   string                   `json:"battleKind,omitempty"`
	Seed                         int64                    `json:"seed"`
	RNGIndex                     int64                    `json:"rngIndex"`
	StartedAt                    int64                    `json:"startedAt"`
	LastActionAt                 int64                    `json:"lastActionAt"`
	LastPushedSeq                int64                    `json:"lastPushedSeq"`
	Wave                         int                      `json:"wave"`
	CompletedBattles             int                      `json:"completedBattles"`
	CurrentWaveCustomLineupID    string                   `json:"currentWaveCustomLineupId,omitempty"`
	TurnCursor                   int                      `json:"turnCursor"`
	Seq                          int64                    `json:"seq"`
	Players                      []battleUnitState        `json:"players"`
	Enemies                      []battleUnitState        `json:"enemies"`
	DifficultyLevel              int                      `json:"difficultyLevel,omitempty"`
	DifficultyMult               float64                  `json:"difficultyMult,omitempty"`
	Order                        []string                 `json:"order"`
	FullOrder                    []string                 `json:"fullOrder,omitempty"`
	PreparationOrder             []string                 `json:"preparationOrder,omitempty"`
	PreparationSeedOrder         []string                 `json:"preparationSeedOrder,omitempty"`
	ActedUnits                   []string                 `json:"actedUnits,omitempty"`
	ExtraTurnQueue               []battleQueuedExtraTurn  `json:"extraTurnQueue,omitempty"`
	BattlefieldEffects           []battlefieldEffectState `json:"battlefieldEffects,omitempty"`
	Events                       []battleEvent            `json:"events"`
	RetainCompleteEventHistory   bool                     `json:"-"`
	AchievementStats             battleAchievementStats   `json:"achievementStats,omitempty"`
	BattleHookTurnCounts         map[string]int           `json:"battleHookTurnCounts,omitempty"`
	BattleHookBattleCounts       map[string]int           `json:"battleHookBattleCounts,omitempty"`
	BattleHookCooldownUntilTurns map[string]int           `json:"battleHookCooldownUntilTurns,omitempty"`
	SpecializationHookWaveCounts map[string]int           `json:"specializationHookWaveCounts,omitempty"`
	ProbabilitySuccessCounts     map[string]int           `json:"probabilitySuccessCounts,omitempty"`
	CompletedTurnCounts          map[string]int           `json:"completedTurnCounts,omitempty"`
	// Release-rule turns are wave-local; equipment turn counters intentionally carry across waves.
	RuleTurnCounts map[string]int `json:"ruleTurnCounts,omitempty"`
	// Fog mode fields (not serialised to clients)
	IsFogMode                       bool   `json:"-"`
	RoomID                          int64  `json:"-"`
	DungeonID                       string `json:"-"`
	turnProfile                     battleTurnProfile
	unitIndexByID                   map[string]*battleUnitState
	unitIndexCount                  int
	otherUnitTurnEndHookUnitIDs     []string
	otherUnitTurnEndHookIndexBuilt  bool
	otherUnitTurnEndHookUnitCount   int
	afterSummonHookDepth            int `json:"-"`
	activeEquipmentHookKeys         map[string]int
	activeExtraTurnUnitIDs          map[string]int
	activeExtraTurnRequest          *battleQueuedExtraTurn
	activeExtraTurnChildCount       int
	probabilityHookDepth            int
	specializationDispatchScopes    int
	specializationDispatchDepth     int
	specializationDispatchEvents    int
	specializationDispatchAncestry  map[string]int
	derivedHookDispatchSuppression  int
	pendingUnitDeaths               []battlePendingUnitDeath
	battlefieldEffectDefinitions    []battleWorldBattlefieldEffect
	battlefieldSpecialEnemyIDs      map[string]struct{}
	nextBuffDurationTurnSerial      int64
	activeBuffDurationTurnSerial    int64
	ArenaFormat                     *arenaFormatDefinition                `json:"-"`
	arenaFormatBarriers             map[string]*arenaFormatBarrierRuntime `json:"-"`
	arenaFormatBarrierOrder         []string                              `json:"-"`
	pendingArenaFormatTriggers      []arenaPendingFormatTrigger           `json:"-"`
	CombatTrialFormat               *combatTrialFormatDefinition          `json:"-"`
	CombatTrialActionCount          int                                   `json:"-"`
	CombatTrialMetrics              map[string]float64                    `json:"-"`
	CombatTrialPlayerSourceIDs      map[string]struct{}                   `json:"-"`
	CombatTrialObjectiveTargetIDs   map[string]struct{}                   `json:"-"`
	CombatTrialSupportUnitIDs       map[string]struct{}                   `json:"-"`
	CombatTrialDefeatedObjectiveIDs map[string]struct{}                   `json:"-"`
	CombatTrialWaveIndex            int                                   `json:"-"`
	CombatTrialEncounterCompleted   bool                                  `json:"-"`
	CombatTrialEndReason            string                                `json:"-"`
}

type battlePendingUnitDeath struct {
	UnitID   string
	SourceID string
}

type battleQueuedExtraTurn struct {
	ActorID          string         `json:"actorId"`
	Trigger          string         `json:"trigger,omitempty"`
	HookID           string         `json:"hookId,omitempty"`
	SourceKind       string         `json:"sourceKind,omitempty"`
	SourceID         string         `json:"sourceId,omitempty"`
	SourceNameKey    string         `json:"sourceNameKey,omitempty"`
	SourceItemID     string         `json:"sourceItemId,omitempty"`
	SourceInstanceID int64          `json:"sourceInstanceId,omitempty"`
	SourceAffixID    string         `json:"sourceAffixId,omitempty"`
	SourceAffixSlot  int            `json:"sourceAffixSlot,omitempty"`
	EventTargetID    string         `json:"eventTargetId,omitempty"`
	EventTargetType  string         `json:"eventTargetType,omitempty"`
	TurnMode         string         `json:"turnMode"`
	DispatchHooks    bool           `json:"dispatchHooks"`
	ChainDepth       int            `json:"chainDepth,omitempty"`
	ChainHookCounts  map[string]int `json:"chainHookCounts,omitempty"`
}

type battlefieldEffectState struct {
	ID        string         `json:"id"`
	Remaining int            `json:"remaining"`
	Progress  int            `json:"progress"`
	Goal      int            `json:"goal"`
	Windows   map[string]int `json:"windows,omitempty"`
}

type battleAchievementStats struct {
	EnemyKills       map[string]int64            `json:"enemyKills,omitempty"`
	KilledEnemyUnits map[string]bool             `json:"killedEnemyUnits,omitempty"`
	SkillCasts       map[string]int64            `json:"skillCasts,omitempty"`
	SkillCastsByUnit map[string]map[string]int64 `json:"skillCastsByUnit,omitempty"`
	BasicAttacks     int64                       `json:"basicAttacks,omitempty"`
	Summons          int64                       `json:"summons,omitempty"`
	MaxSingleDamage  float64                     `json:"maxSingleDamage,omitempty"`
}

func compactBattleTimelineStepMeta(raw any) map[string]any {
	meta := normalizeAnyMap(raw)
	if len(meta) == 0 {
		return nil
	}
	out := map[string]any{}
	if runtimeAmp, ok := meta["runtimeDamageAmp"]; ok {
		parsed := roundTo4(toFloat64(runtimeAmp, 0))
		if parsed != 0 {
			out["ra"] = parsed
		}
	}
	if runtimeAmpSources, ok := meta["runtimeDamageAmpSources"]; ok {
		parsed := battleAnyMapSlice(runtimeAmpSources)
		if len(parsed) > 0 {
			out["rs"] = parsed
		}
	}
	if mutationSources, ok := meta["mutationSources"]; ok {
		parsed := battleAnyMapSlice(mutationSources)
		if len(parsed) > 0 {
			out["ms"] = parsed
		}
	}
	if criticalMultiplier, ok := meta["criticalMultiplier"]; ok {
		parsed := roundTo4(toFloat64(criticalMultiplier, 0))
		if parsed > 0 {
			out["cm"] = parsed
		}
	}
	if hitIndex := toInt64Loose(meta["hitIndex"], 0); hitIndex > 0 {
		out["hi"] = hitIndex
	}
	if hitCount := toInt64Loose(meta["hitCount"], 0); hitCount > 0 {
		out["hc"] = hitCount
	}
	if rawMultiplier, exists := meta["hitDamageMultiplier"]; exists {
		multiplier := toFloat64Loose(rawMultiplier, math.NaN())
		if !math.IsNaN(multiplier) && !math.IsInf(multiplier, 0) && multiplier >= 0 && multiplier <= 1 {
			out["hm"] = multiplier
		}
	}
	if chainID := strings.TrimSpace(fmt.Sprintf("%v", meta["chainId"])); chainID != "" {
		out["ci"] = chainID
	}
	if parseBooleanLoose(meta["corpseExplosion"], false) {
		out["ce"] = true
	}
	for _, key := range []string{"sourceKind", "sourceId", "nodeId", "sourceNameKey", "hookId", "hookDefinitionId", "mutationId"} {
		if value := strings.TrimSpace(fmt.Sprintf("%v", meta[key])); value != "" {
			out[key] = value
		}
	}
	if sourceInstanceID := toInt64Loose(meta["sourceInstanceId"], 0); sourceInstanceID > 0 {
		out["sourceInstanceId"] = sourceInstanceID
	}
	for _, key := range []string{"interruptedSkillId", "interruptedActorId"} {
		if value, ok := meta[key].(string); ok && value != "" {
			out[key] = value
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func expandBattlePositionCode(raw any) string {
	switch typed := raw.(type) {
	case int:
		switch typed {
		case 0:
			return "vanguard"
		case 1:
			return "midguard"
		case 2:
			return "rearguard"
		default:
			return ""
		}
	case int64:
		return expandBattlePositionCode(int(typed))
	case float64:
		return expandBattlePositionCode(int(typed))
	case string:
		trimmed := strings.TrimSpace(strings.ToLower(typed))
		if trimmed == "0" {
			return "vanguard"
		}
		if trimmed == "1" {
			return "midguard"
		}
		if trimmed == "2" {
			return "rearguard"
		}
		return normalizeBattleTargetPosition(trimmed)
	default:
		return normalizeBattleTargetPosition(strings.TrimSpace(fmt.Sprintf("%v", raw)))
	}
}

type battleRunPlaybackCache struct {
	Timeline []wsbus.BattleTimelineEvent
}

type battleUnitState struct {
	DominationSkills                    []string                        `json:"dominationSkills,omitempty"`
	UnitID                              string                          `json:"unitId"`
	IsPlayerUnit                        bool                            `json:"isPlayerUnit,omitempty"`
	Side                                string                          `json:"side"`
	OriginSide                          string                          `json:"originSide,omitempty"`
	BaseSide                            string                          `json:"baseSide,omitempty"`
	PerspectiveSide                     string                          `json:"perspectiveSide,omitempty"`
	UserID                              int64                           `json:"userId,omitempty"`
	UnitType                            string                          `json:"unitType,omitempty"`
	KittyID                             int64                           `json:"kittyId,omitempty"`
	EnemyID                             string                          `json:"enemyId,omitempty"`
	EnemyType                           string                          `json:"enemyType,omitempty"`
	EnemyLevel                          int                             `json:"enemyLevel,omitempty"`
	BattleLevel                         int64                           `json:"battleLevel,omitempty"`
	Name                                string                          `json:"name"`
	NameKey                             string                          `json:"nameKey,omitempty"`
	Position                            string                          `json:"position"`
	BasePosition                        string                          `json:"basePosition,omitempty"`
	Alive                               bool                            `json:"alive"`
	CorpseConsumed                      bool                            `json:"corpseConsumed,omitempty"`
	RewardEligible                      bool                            `json:"rewardEligible"`
	HP                                  float64                         `json:"hp"`
	MP                                  float64                         `json:"mp"`
	MaxHP                               float64                         `json:"maxHp"`
	MaxMP                               float64                         `json:"maxMp"`
	BaseStats                           map[string]float64              `json:"baseStats,omitempty"`
	Stats                               map[string]float64              `json:"stats"`
	StatModifiers                       map[string]float64              `json:"statModifiers,omitempty"`
	StatSources                         map[string]map[string]float64   `json:"statSources,omitempty"`
	StatMode                            string                          `json:"statMode,omitempty"`
	SkillSlots                          []string                        `json:"skillSlots"`
	SkillIDs                            []string                        `json:"skillIds,omitempty"`
	SkillRules                          map[string]battleSkillRuleState `json:"skillRules,omitempty"`
	ItemRules                           map[string]battleSkillRuleState `json:"itemRules,omitempty"`
	Skills                              []battleSkillState              `json:"skills"`
	BattleItems                         []battleItemState               `json:"battleItems,omitempty"`
	EquipmentDigest                     []string                        `json:"equipmentDigest,omitempty"`
	EquipmentIDs                        []int64                         `json:"equipmentInstanceIds,omitempty"`
	equipmentSnapshotItems              []store.EquippedItem
	SpecializationTreeID                string                    `json:"specializationTreeId,omitempty"`
	SpecializationRulesetVersion        string                    `json:"specializationRulesetVersion,omitempty"`
	SpecializationBalanceRevision       int                       `json:"specializationBalanceRevision,omitempty"`
	SpecializationRevision              int64                     `json:"specializationRevision,omitempty"`
	SpecializationDigest                string                    `json:"specializationDigest,omitempty"`
	WeaponClasses                       []string                  `json:"weaponClasses,omitempty"`
	BasicAttack                         *battleBasicAttackProfile `json:"basicAttack,omitempty"`
	EquipmentHooks                      []battleEquipmentHookRule `json:"equipmentHooks,omitempty"`
	SpecializationHooks                 []battleEquipmentHookRule `json:"specializationHooks,omitempty"`
	otherUnitTurnEndHooks               []*battleEquipmentHookRule
	otherUnitTurnEndHooksIndexed        bool
	otherUnitTurnEndEquipmentCount      int
	otherUnitTurnEndKittyCount          int
	otherUnitTurnEndSpecializationCount int
	EffectMutations                     []battleAffixEffectMutation `json:"effectMutations,omitempty"`
	SpecializationEffectMutations       []battleAffixEffectMutation `json:"specializationEffectMutations,omitempty"`
	KittyPersonalityHooks               []battleEquipmentHookRule   `json:"kittyPersonalityHooks"`
	KittyPersonalityEffectMutations     []battleAffixEffectMutation `json:"kittyPersonalityEffectMutations"`
	Buffs                               []battleBuffState           `json:"buffs,omitempty"`
	SummonRemainTurns                   int64                       `json:"summonRemainTurns,omitempty"`
	SummonTemplateUnitID                string                      `json:"summonTemplateUnitId,omitempty"`
	SummonSourceUnitID                  string                      `json:"summonSourceUnitId,omitempty"`
	SummonMaxCountPerUnit               int                         `json:"summonMaxCountPerUnit,omitempty"`
	SummonPersistAcrossBattles          bool                        `json:"summonPersistAcrossBattles,omitempty"`
	SummonOnCasterDeath                 string                      `json:"summonOnCasterDeath,omitempty"`
	SummonOnCapOverflow                 string                      `json:"summonOnCapOverflow,omitempty"`
	SummonOnDispel                      string                      `json:"summonOnDispel,omitempty"`
	SummonOnDurationEnd                 string                      `json:"summonOnDurationEnd,omitempty"`
	SummonSourceEnhanceLevel            int                         `json:"summonSourceEnhanceLevel,omitempty"`
	SummonOrder                         int64                       `json:"summonOrder,omitempty"`
	SummonNameSuffixKey                 string                      `json:"summonNameSuffixKey,omitempty"`
	SkipTurnOrder                       bool                        `json:"-"`
	TurnIneligible                      bool                        `json:"-"`
	HasSyncedSummonTrigger              bool                        `json:"-"`
	SyncedSummonTemplateIDs             map[string]struct{}         `json:"-"`
}

type battleBasicAttackProfile struct {
	TargetDirection    string              `json:"targetDirection"`
	TargetSeekStrategy string              `json:"targetSeekStrategy"`
	DamageType         string              `json:"damageType"`
	Formula            []battleAttackTerm  `json:"formula,omitempty"`
	DamageSpec         battleDamageSpec    `json:"damageSpec,omitempty"`
	WeaponItemID       string              `json:"weaponItemId,omitempty"`
	SourceEnhanceLevel int                 `json:"sourceEnhanceLevel,omitempty"`
	Effects            []battleSkillEffect `json:"effects,omitempty"`
}

type battleCarryUnitState struct {
	DominationSkills                []string                        `json:"dominationSkills,omitempty"`
	UnitID                          string                          `json:"unitId"`
	IsPlayerUnit                    bool                            `json:"isPlayerUnit,omitempty"`
	Side                            string                          `json:"side"`
	OriginSide                      string                          `json:"originSide,omitempty"`
	BaseSide                        string                          `json:"baseSide,omitempty"`
	PerspectiveSide                 string                          `json:"perspectiveSide,omitempty"`
	UserID                          int64                           `json:"userId,omitempty"`
	UnitType                        string                          `json:"unitType,omitempty"`
	KittyID                         int64                           `json:"kittyId,omitempty"`
	EnemyID                         string                          `json:"enemyId,omitempty"`
	EnemyType                       string                          `json:"enemyType,omitempty"`
	EnemyLevel                      int                             `json:"enemyLevel,omitempty"`
	Name                            string                          `json:"name"`
	NameKey                         string                          `json:"nameKey,omitempty"`
	Position                        string                          `json:"position"`
	BasePosition                    string                          `json:"basePosition,omitempty"`
	Alive                           bool                            `json:"alive"`
	CorpseConsumed                  bool                            `json:"corpseConsumed,omitempty"`
	RewardEligible                  bool                            `json:"rewardEligible"`
	HP                              float64                         `json:"hp"`
	MP                              float64                         `json:"mp"`
	MaxHP                           float64                         `json:"maxHp"`
	MaxMP                           float64                         `json:"maxMp"`
	BaseStats                       map[string]float64              `json:"baseStats,omitempty"`
	Stats                           map[string]float64              `json:"stats"`
	StatModifiers                   map[string]float64              `json:"statModifiers,omitempty"`
	StatSources                     map[string]map[string]float64   `json:"statSources,omitempty"`
	StatMode                        string                          `json:"statMode,omitempty"`
	SkillSlots                      []string                        `json:"skillSlots"`
	SkillIDs                        []string                        `json:"skillIds,omitempty"`
	SkillRules                      map[string]battleSkillRuleState `json:"skillRules,omitempty"`
	ItemRules                       map[string]battleSkillRuleState `json:"itemRules,omitempty"`
	Skills                          []battleSkillState              `json:"skills"`
	BattleItems                     []battleItemState               `json:"battleItems,omitempty"`
	EquipmentDigest                 []string                        `json:"equipmentDigest,omitempty"`
	EquipmentIDs                    []int64                         `json:"equipmentInstanceIds,omitempty"`
	SpecializationTreeID            string                          `json:"specializationTreeId,omitempty"`
	SpecializationRulesetVersion    string                          `json:"specializationRulesetVersion,omitempty"`
	SpecializationBalanceRevision   int                             `json:"specializationBalanceRevision,omitempty"`
	SpecializationRevision          int64                           `json:"specializationRevision,omitempty"`
	SpecializationDigest            string                          `json:"specializationDigest,omitempty"`
	WeaponClasses                   []string                        `json:"weaponClasses,omitempty"`
	BasicAttack                     *battleBasicAttackProfile       `json:"basicAttack,omitempty"`
	EquipmentHooks                  []battleEquipmentHookRule       `json:"equipmentHooks,omitempty"`
	SpecializationHooks             []battleEquipmentHookRule       `json:"specializationHooks,omitempty"`
	EffectMutations                 []battleAffixEffectMutation     `json:"effectMutations,omitempty"`
	SpecializationEffectMutations   []battleAffixEffectMutation     `json:"specializationEffectMutations,omitempty"`
	KittyPersonalityHooks           []battleEquipmentHookRule       `json:"kittyPersonalityHooks"`
	KittyPersonalityEffectMutations []battleAffixEffectMutation     `json:"kittyPersonalityEffectMutations"`
	Buffs                           []battleBuffState               `json:"buffs,omitempty"`
	SummonRemainTurns               int64                           `json:"summonRemainTurns,omitempty"`
	SummonTemplateUnitID            string                          `json:"summonTemplateUnitId,omitempty"`
	SummonSourceUnitID              string                          `json:"summonSourceUnitId,omitempty"`
	SummonMaxCountPerUnit           int                             `json:"summonMaxCountPerUnit,omitempty"`
	SummonPersistAcrossBattles      bool                            `json:"summonPersistAcrossBattles,omitempty"`
	SummonOnCasterDeath             string                          `json:"summonOnCasterDeath,omitempty"`
	SummonOnCapOverflow             string                          `json:"summonOnCapOverflow,omitempty"`
	SummonOnDispel                  string                          `json:"summonOnDispel,omitempty"`
	SummonOnDurationEnd             string                          `json:"summonOnDurationEnd,omitempty"`
	SummonSourceEnhanceLevel        int                             `json:"summonSourceEnhanceLevel,omitempty"`
	SummonOrder                     int64                           `json:"summonOrder,omitempty"`
	SummonNameSuffixKey             string                          `json:"summonNameSuffixKey,omitempty"`
}

type battleAttackTerm struct {
	AttrKey         string  `json:"attrKey"`
	Coefficient     float64 `json:"coefficient"`
	PerSkillLevel   float64 `json:"perSkillLevel,omitempty"`
	PerEnhanceLevel float64 `json:"perEnhanceLevel,omitempty"`
}

type battleSkillTargetCount struct {
	Base    int                `json:"base"`
	Formula []battleAttackTerm `json:"formula,omitempty"`
	Max     int                `json:"max,omitempty"`
	MaxSet  bool               `json:"maxSet,omitempty"`
}

type battleSkillTargetPredicate struct {
	Kind       string `json:"kind"`
	ActorAttr  string `json:"actorAttr,omitempty"`
	TargetAttr string `json:"targetAttr,omitempty"`
	Operator   string `json:"operator,omitempty"`
	EnemyType  string `json:"enemyType,omitempty"`
	StatusID   string `json:"statusId,omitempty"`
}

type battleSkillTargeting struct {
	TargetPositions      []string                     `json:"targetPositions"`
	SecondarySeek        string                       `json:"secondarySeek"`
	TargetScope          string                       `json:"targetScope"`
	SelectRule           string                       `json:"selectRule"`
	SelectAttrKey        string                       `json:"selectAttrKey,omitempty"`
	FallbackSelfIfEmpty  bool                         `json:"fallbackSelfIfEmpty,omitempty"`
	ExcludeSelf          bool                         `json:"excludeSelf,omitempty"`
	IgnoreTargetPriority bool                         `json:"ignoreTargetPriority,omitempty"`
	TargetCount          battleSkillTargetCount       `json:"targetCount"`
	TargetPredicates     []battleSkillTargetPredicate `json:"targetPredicates,omitempty"`
}

type battleSkillAmountFormula struct {
	BaseAdd float64            `json:"baseAdd"`
	Terms   []battleAttackTerm `json:"terms,omitempty"`
}

const battleHitDamageScalingKindPoweredExponentialDecay = "powered_exponential_decay"

type battleHitDamageScaling struct {
	Kind                string  `json:"kind"`
	SecondHitMultiplier float64 `json:"secondHitMultiplier"`
	CurvePower          float64 `json:"curvePower"`
}

type battleDamageSpec struct {
	DamageType                                   string                   `json:"damageType,omitempty"`
	AmountFormula                                battleSkillAmountFormula `json:"amountFormula,omitempty"`
	HitCount                                     battleSkillTargetCount   `json:"hitCount,omitempty"`
	HitDamageScaling                             *battleHitDamageScaling  `json:"hitDamageScaling,omitempty"`
	FlatDefensePenetrationFormula                battleSkillAmountFormula `json:"flatDefensePenetrationFormula,omitempty"`
	PercentDefensePenetrationFormula             battleSkillAmountFormula `json:"percentDefensePenetrationFormula,omitempty"`
	PhysicalFlatDefensePenetrationFormula        battleSkillAmountFormula `json:"physicalFlatDefensePenetrationFormula,omitempty"`
	PhysicalPercentDefensePenetrationFormula     battleSkillAmountFormula `json:"physicalPercentDefensePenetrationFormula,omitempty"`
	MagicalFlatDefensePenetrationFormula         battleSkillAmountFormula `json:"magicalFlatDefensePenetrationFormula,omitempty"`
	MagicalPercentDefensePenetrationFormula      battleSkillAmountFormula `json:"magicalPercentDefensePenetrationFormula,omitempty"`
	ExtraDamageAmplificationFormula              battleSkillAmountFormula `json:"extraDamageAmplificationFormula,omitempty"`
	BonusAgainstTargetTypes                      map[string]float64       `json:"bonusAgainstTargetTypes,omitempty"`
	EnemyOnlyDamageReductionPierce               float64                  `json:"enemyOnlyDamageReductionPierce,omitempty"`
	EnemyOnlyDamageReductionPierceTargetStatuses []string                 `json:"enemyOnlyDamageReductionPierceTargetStatuses,omitempty"`
}

type battleSkillStatusPayload struct {
	StatusIDs                  []string                            `json:"statusIds"`
	DurationBaseTurns          int                                 `json:"durationBaseTurns"`
	DurationPerSkillLevelTurns float64                             `json:"durationPerSkillLevelTurns"`
	DurationFormula            battleSkillAmountFormula            `json:"durationFormula,omitempty"`
	ApplyChanceBase            *float64                            `json:"applyChanceBase,omitempty"`
	ApplyChancePerSkillLevel   float64                             `json:"applyChancePerSkillLevel,omitempty"`
	ApplyChanceFormula         battleSkillAmountFormula            `json:"applyChanceFormula,omitempty"`
	ApplyChanceMin             *float64                            `json:"applyChanceMin,omitempty"`
	ApplyChanceMax             *float64                            `json:"applyChanceMax,omitempty"`
	Payload                    map[string]any                      `json:"payload,omitempty"`
	PayloadFormula             map[string]battleSkillAmountFormula `json:"payloadFormula,omitempty"`
}

type battleSkillSummonPayload struct {
	UnitID               string                              `json:"unitId"`
	Side                 string                              `json:"side,omitempty"`
	CountFormula         battleSkillAmountFormula            `json:"countFormula,omitempty"`
	DurationFormula      battleSkillAmountFormula            `json:"durationFormula,omitempty"`
	DurationTurns        int64                               `json:"durationTurns,omitempty"`
	MaxCountPerUnit      int                                 `json:"maxCountPerUnit,omitempty"`
	MaxCountFormula      battleSkillAmountFormula            `json:"maxCountFormula,omitempty"`
	PersistAcrossBattles bool                                `json:"persistAcrossBattles,omitempty"`
	OnCasterDeath        string                              `json:"onCasterDeath,omitempty"`
	OnCapOverflow        string                              `json:"onCapOverflow,omitempty"`
	OnDispel             string                              `json:"onDispel,omitempty"`
	OnDurationEnd        string                              `json:"onDurationEnd,omitempty"`
	StatsFormula         map[string]battleSkillAmountFormula `json:"statsFormula,omitempty"`
}

type battleWeightedStatusChoice struct {
	ID            string         `json:"id"`
	Weight        float64        `json:"weight"`
	StatusID      string         `json:"statusId"`
	DurationTurns int            `json:"durationTurns"`
	Payload       map[string]any `json:"payload,omitempty"`
}

type battleSkillEffect struct {
	ID                          string                       `json:"id,omitempty"`
	GeneratedBy                 string                       `json:"generatedBy,omitempty"`
	MutationSources             []battleEffectMutationSource `json:"mutationSources,omitempty"`
	AttackStatPolicy            string                       `json:"-"`
	Kind                        string                       `json:"kind"`
	DamageType                  string                       `json:"damageType,omitempty"`
	DamageTypePolicy            string                       `json:"damageTypePolicy,omitempty"`
	Resource                    string                       `json:"resource,omitempty"`
	Targeting                   battleSkillTargeting         `json:"targeting"`
	CorpseTargeting             battleSkillTargeting         `json:"corpseTargeting,omitempty"`
	OnlyWhenNoTargetFromEffects []int                        `json:"onlyWhenNoTargetFromEffects,omitempty"`
	ReuseTargetsFromEffect      int                          `json:"reuseTargetsFromEffect,omitempty"`
	TargetRowFromEffect         int                          `json:"targetRowFromEffect,omitempty"`
	ExcludeTargetsFromEffects   []int                        `json:"excludeTargetsFromEffects,omitempty"`
	IgnoreUntargetable          bool                         `json:"ignoreUntargetable,omitempty"`
	ConsumeCorpse               bool                         `json:"consumeCorpse,omitempty"`
	AmountFormula               battleSkillAmountFormula     `json:"amountFormula,omitempty"`
	DamageSpec                  battleDamageSpec             `json:"damageSpec,omitempty"`
	StatusPayload               battleSkillStatusPayload     `json:"statusPayload,omitempty"`
	SummonPayload               battleSkillSummonPayload     `json:"summonPayload,omitempty"`
	WeightedStatusChoices       []battleWeightedStatusChoice `json:"choices,omitempty"`
	BuffIDs                     []string                     `json:"buffIds,omitempty"`
	MaxRemoved                  int                          `json:"maxRemoved,omitempty"`
	ConsumeStatusesOnHit        []string                     `json:"consumeStatusesOnHit,omitempty"`
	ConsumeCasterStatusesOnHit  []string                     `json:"consumeCasterStatusesOnHit,omitempty"`
	RequiredCasterStatuses      []string                     `json:"requiredCasterStatuses,omitempty"`
	RequiredTargetStatuses      []string                     `json:"requiredTargetStatuses,omitempty"`
	ForbiddenCasterStatuses     []string                     `json:"forbiddenCasterStatuses,omitempty"`
	ForbiddenTargetStatuses     []string                     `json:"forbiddenTargetStatuses,omitempty"`
	MpCostFormula               battleSkillAmountFormula     `json:"mpCost,omitempty"`
	CooldownAmount              int                          `json:"amount,omitempty"`
	IncludeBasic                bool                         `json:"includeBasic,omitempty"`
	TargetSkillIDs              []string                     `json:"skillIds,omitempty"`
	ExcludeSkillIDs             []string                     `json:"excludeSkillIds,omitempty"`
	MaxHPRatio                  float64                      `json:"maxHpRatio,omitempty"`
	MinimumHP                   float64                      `json:"minimumHp,omitempty"`
	AmountRatio                 float64                      `json:"amountRatio,omitempty"`
	SecondaryTargetPolicy       string                       `json:"secondaryTargetPolicy,omitempty"`
	SecondaryTargetPosition     string                       `json:"secondaryTargetPosition,omitempty"`
	MarkID                      string                       `json:"markId,omitempty"`
	DurationTurns               int64                        `json:"durationTurns,omitempty"`
}

type battleEffectMutationSource struct {
	MutationID    string `json:"mutationId,omitempty"`
	SourceKind    string `json:"sourceKind,omitempty"`
	SourceID      string `json:"sourceId,omitempty"`
	SourceNameKey string `json:"sourceNameKey,omitempty"`
}

type battleSkillCastGuard struct {
	Kind       string `json:"kind"`
	EffectKind string `json:"effectKind,omitempty"`
}

type battleSkillRuleState struct {
	Enabled bool                   `json:"enabled"`
	Groups  []rules.ConditionGroup `json:"groups"`
}

type battleSkillSlotRule struct {
	BaseSkillSlots int
	IntPerExtra    int
	MinSkillSlots  int
	MaxSkillSlots  int
}

type battleSkillState struct {
	Combo           *battleComboPlan       `json:"combo,omitempty"`
	ComboOnly       bool                   `json:"comboOnly,omitempty"`
	Uninterruptible bool                   `json:"uninterruptible,omitempty"`
	ID              string                 `json:"id"`
	Name            string                 `json:"name"`
	Category        string                 `json:"category,omitempty"`
	Icon            string                 `json:"icon,omitempty"`
	RulesetVersion  int                    `json:"rulesetVersion,omitempty"`
	Level           int                    `json:"level"`
	Exp             int64                  `json:"exp"`
	HPCost          float64                `json:"hpCost"`
	MPCost          float64                `json:"mpCost"`
	CooldownTurns   int                    `json:"cooldownTurns"`
	CooldownRemain  int                    `json:"cooldownRemain"`
	Targeting       battleSkillTargeting   `json:"targeting"`
	Effects         []battleSkillEffect    `json:"effects,omitempty"`
	CastGuards      []battleSkillCastGuard `json:"castGuards,omitempty"`
}

type battleItemState struct {
	ID             string               `json:"id"`
	ItemID         string               `json:"itemId"`
	Name           string               `json:"name"`
	Icon           string               `json:"icon,omitempty"`
	Quantity       int64                `json:"quantity"`
	CooldownTurns  int                  `json:"cooldownTurns"`
	CooldownRemain int                  `json:"cooldownRemain"`
	Targeting      battleSkillTargeting `json:"targeting"`
	Effects        []battleSkillEffect  `json:"effects,omitempty"`
	Resolving      bool                 `json:"-"`
	// Trigger-item fields (isTrigger = true means passive, cannot be actively used)
	IsTrigger         bool     `json:"isTrigger,omitempty"`
	DungeonIDs        []string `json:"dungeonIds,omitempty"`
	SpawnEnemyIDs     []string `json:"spawnEnemyIds,omitempty"`
	MinWave           int      `json:"minWave,omitempty"`
	PerWaveChance     float64  `json:"perWaveChance,omitempty"`
	IntervalWaves     int      `json:"intervalWaves,omitempty"`
	LastSpawnWave     int      `json:"lastSpawnWave,omitempty"`
	PartyTriggerGroup string   `json:"partyTriggerGroup,omitempty"`
}

type battleBuffState struct {
	BuffID                string         `json:"buffId"`
	RemainTurns           int64          `json:"remainTurns"`
	LegacyRemainMS        int64          `json:"remainMs,omitempty"`
	Payload               map[string]any `json:"payload,omitempty"`
	SourceID              string         `json:"sourceId,omitempty"`
	Triggered             int64          `json:"triggered,omitempty"`
	AppliedBuffTurnSerial int64          `json:"-"`
}

type battleEquipmentHookRule struct {
	ID                     string                   `json:"id,omitempty"`
	DefinitionID           string                   `json:"definitionId,omitempty"`
	SourceKind             string                   `json:"sourceKind,omitempty"`
	SourceID               string                   `json:"sourceId,omitempty"`
	SourceNameKey          string                   `json:"sourceNameKey,omitempty"`
	SourceItemID           string                   `json:"sourceItemId,omitempty"`
	SourceInstanceID       int64                    `json:"sourceInstanceId,omitempty"`
	SourceEnhanceLevel     int                      `json:"sourceEnhanceLevel,omitempty"`
	SourceAffixID          string                   `json:"sourceAffixId,omitempty"`
	SourceAffixSlot        int                      `json:"sourceAffixSlot,omitempty"`
	Trigger                string                   `json:"trigger"`
	Priority               int                      `json:"priority,omitempty"`
	MaxTriggersPerTurn     int                      `json:"maxTriggersPerTurn,omitempty"`
	MaxTriggersPerBattle   int                      `json:"maxTriggersPerBattle,omitempty"`
	MaxTriggersPerWave     int                      `json:"maxTriggersPerWave,omitempty"`
	SharedTriggerGroup     string                   `json:"sharedTriggerGroup,omitempty"`
	ActivationCost         battleHookActivationCost `json:"activationCost,omitempty"`
	CooldownCompletedTurns int                      `json:"cooldownCompletedTurns,omitempty"`
	DispatchHooks          bool                     `json:"dispatchHooks,omitempty"`
	SubjectFilter          map[string]any           `json:"subjectFilter,omitempty"`
	EventFilter            map[string]any           `json:"eventFilter,omitempty"`
	Conditions             []map[string]any         `json:"conditions,omitempty"`
	Effects                []map[string]any         `json:"effects"`
}

type battleHookActivationCost struct {
	MaxHPRatio float64 `json:"maxHpRatio,omitempty"`
	MaxMPRatio float64 `json:"maxMpRatio,omitempty"`
}

const (
	battleMaxSpecializationDispatchDepth  = 8
	battleMaxSpecializationDispatchEvents = 64
)

type battleEquipmentHookContext struct {
	SkillID              string
	SkillCategory        string
	CastKind             string
	DamageType           string
	IsBasicAttack        bool
	BuffID               string
	BuffApplicationKind  string
	BuffKind             string
	TurnKind             string
	IsPreparation        bool
	ActionSuppressed     bool
	ProbabilityKind      string
	ProbabilitySourceIDs []string
	HitCount             int
	MPCost               float64
	HPCost               float64
	MPInsufficient       bool
	CooldownDelta        int
	AllowDebtCast        bool
	ConsumedMarks        map[string]bool
	ConsumedBuffs        map[string]bool
	RawDefense           float64
	FlatPenetration      float64
	PercentPenetration   float64
}

type battleAffixEffectMutation struct {
	ID                       string                    `json:"id,omitempty"`
	Priority                 int                       `json:"priority,omitempty"`
	SourceKind               string                    `json:"sourceKind,omitempty"`
	SourceID                 string                    `json:"sourceId,omitempty"`
	SourceNameKey            string                    `json:"sourceNameKey,omitempty"`
	Scope                    string                    `json:"scope"`
	SkillIDList              []string                  `json:"skillIds,omitempty"`
	SkillIDs                 map[string]struct{}       `json:"-"`
	SkillCategoryList        []string                  `json:"skillCategories,omitempty"`
	SkillCategories          map[string]struct{}       `json:"-"`
	WeaponClassList          []string                  `json:"weaponClasses,omitempty"`
	WeaponClasses            map[string]struct{}       `json:"-"`
	DamageTypeList           []string                  `json:"damageTypes,omitempty"`
	DamageTypes              map[string]struct{}       `json:"-"`
	SelectAll                bool                      `json:"selectAll"`
	Indexes                  []int                     `json:"indexes,omitempty"`
	EffectKindList           []string                  `json:"effectKinds,omitempty"`
	EffectKinds              map[string]struct{}       `json:"-"`
	EffectIDList             []string                  `json:"effectIds,omitempty"`
	EffectIDs                map[string]struct{}       `json:"-"`
	IncludeGeneratedEffects  bool                      `json:"includeGeneratedEffects,omitempty"`
	TargetCountDelta         *battleSkillAmountFormula `json:"targetCountDelta,omitempty"`
	TargetCountTermsAppend   []battleAttackTerm        `json:"targetCountTermsAppend,omitempty"`
	AmountBaseAddDelta       *battleSkillAmountFormula `json:"amountBaseAddDelta,omitempty"`
	AmountTermsAppend        []battleAttackTerm        `json:"amountTermsAppend,omitempty"`
	AmountMultiplier         *float64                  `json:"amountMultiplier,omitempty"`
	RuntimeDamageAmp         float64                   `json:"runtimeDamageAmp,omitempty"`
	HitCountDelta            int                       `json:"hitCountDelta,omitempty"`
	CooldownDelta            int                       `json:"cooldownDelta,omitempty"`
	CooldownFloor            int                       `json:"cooldownFloor,omitempty"`
	MPCostDelta              float64                   `json:"mpCostDelta,omitempty"`
	MPCostMultiplier         *float64                  `json:"mpCostMultiplier,omitempty"`
	AttackStatPolicy         string                    `json:"attackStatPolicy,omitempty"`
	DamageTypePolicy         string                    `json:"damageTypePolicy,omitempty"`
	TargetingPatch           map[string]any            `json:"targetingPatch,omitempty"`
	StatusAppend             []string                  `json:"statusAppend,omitempty"`
	StatusRemoveList         []string                  `json:"statusRemove,omitempty"`
	StatusRemove             map[string]struct{}       `json:"-"`
	ConstraintTargetCountMin *int                      `json:"constraintTargetCountMin,omitempty"`
	ConstraintTargetCountMax *int                      `json:"constraintTargetCountMax,omitempty"`
	AppendEffects            []battleSkillEffect       `json:"appendEffects,omitempty"`
}

type battlePersonalityProfile struct {
	ID              string
	NameKey         string
	BattleHooks     []map[string]any
	EffectMutations []map[string]any
}

type battleEvent struct {
	Seq  int64          `json:"seq"`
	Type string         `json:"type"`
	TS   int64          `json:"ts"`
	Data map[string]any `json:"data,omitempty"`
}

const battleRecentEventLimit = 200

type battleEnemyConfig struct {
	ID             string
	Name           string
	NameKey        string
	DescKey        string
	Position       string
	Level          int
	Type           string
	Class          string // e.g. "scout", "warrior", "mage" — used for fog stat generation
	Race           string // e.g. "beast", "humanoid" — used for fog stat generation
	Exp            float64
	Stats          map[string]float64
	Drops          map[string]battleDropRule
	DropIDs        []string
	SkillIDs       []string
	Equipment      []battleEnemyEquipment
	InitialBuffIds []string
}

type battleEnemyEquipment struct {
	store.EquippedItem
	BaseSnapshot map[string]float64
}

type battleDropRule struct {
	Min     int64
	Max     int64
	Percent float64
}

type battleWaveSettlement struct {
	Err     error
	Rewards map[int64]store.BattleWaveReward
	ByUser  map[int64]map[string]any
}

type battleRewardGrant struct {
	Contributions operationeffect.Contributions `json:"contributions,omitempty"`
	UserID        int64                         `json:"userId"`
	ItemRewards   map[string]int64              `json:"itemRewards"`
	AttrExp       map[string]float64            `json:"attrExp"`
	KittyAttrExp  map[int64]map[string]float64  `json:"kittyAttrExp"`
}

const (
	battleStatModeProjected                   = "projected"
	battleStatModeDirect                      = "direct"
	battleEffectPreferAsEnemyTarget           = "prefer_as_enemy_target"
	battleEffectAvoidAsEnemyTarget            = "avoid_as_enemy_target"
	battleEffectPreferAsAllyTarget            = "prefer_as_ally_target"
	battleEffectAvoidAsAllyTarget             = "avoid_as_ally_target"
	battleEffectUntargetableByAlly            = "untargetable_by_ally"
	battleEffectTurnActionSuppressed          = "turn_action_suppressed"
	battleEffectSkillActionSuppressed         = "skill_action_suppressed"
	battleEffectBasicAttackSuppressed         = "basic_attack_suppressed"
	battleEffectAllActionsSuppressed          = "all_actions_suppressed"
	battleEffectFrontlineFixation             = "frontline_fixation"
	battleEffectPositionOverride              = "position_override"
	battleEffectAdvancePosition               = "advance_position"
	battleEffectSideOverride                  = "side_override"
	battleEffectInvertRelationView            = "invert_relation_perspective"
	battleEffectPositionOverrideKey           = "positionoverride"
	battleEffectAdvancePositionKey            = "advanceposition"
	battleEffectSideOverrideKey               = "sideoverride"
	battleEffectInvertRelationViewKey         = "invertrelationperspective"
	battleEffectOverrideTargeting             = "override_targeting"
	battleEffectDamageRedirectSource          = "damage_redirect_to_source"
	battleEquipmentTriggerBattleStart         = "on_battle_start"
	battleEquipmentTriggerBattleEnd           = "on_battle_end"
	battleEquipmentTriggerWaveStart           = "on_wave_start"
	battleEquipmentTriggerOnKill              = "on_kill"
	battleEquipmentTriggerUnitDeath           = "on_unit_death"
	battleEquipmentTriggerAfterSummon         = "on_after_summon"
	battleEquipmentTriggerAfterApplyBuff      = "on_after_apply_buff"
	battleEquipmentTriggerOtherTurnEnd        = "on_other_unit_turn_end"
	battleEquipmentTriggerAllyAfterTakeDamage = "on_ally_after_take_damage"
	battleEquipmentEffectMarkedAttack         = "basic_attack_marked_target"
	battleMaxAfterSummonHookDepth             = 8
	battleSummonBindingBuffID                 = "summon_unit_bound"
	battleSummonOnCasterDeathKeep             = "keep_until_duration_end"
	battleSummonOnCasterDeathDespawn          = "despawn_immediately"
	battleSummonOnCapOverflowReplace          = "replace_oldest"
	battleSummonOnCapOverflowFail             = "fail_new_summon"
	battleSummonOnDispelDespawn               = "despawn_immediately"
	battleSummonOnDispelIgnore                = "ignore_dispel"
	battleSummonOnDurationEndDespawn          = "despawn_immediately"

	// Normalized form (battleNormalizeTrigger strips underscores).
	// JSON stores underscored: "skip_turn_order", "turn_ineligible", "trigger_synced_summon_turn".
	battleEffectSkipTurnOrder           = "skipturnorder"
	battleEffectTurnIneligible          = "turnineligible"
	battleEffectTriggerSyncedSummonTurn = "triggersyncedsummonturn"
	battleEffectSuppressMirrorTrigger   = "suppressmirrorcasttrigger"
	battleEffectExtraEnemyRefresh       = "extraenemyrefresh"
	battleExtraEnemyPoolCurrentWorld    = "currentworld"
	battleMaxExtraEnemyRefreshCount     = 64
)

// ensureSliceSpareCap prevents slice reallocation when hooks hold pointers and
// addBattleSummon appends new units. Without this, hook-held pointers become stale.
func ensureSliceSpareCap[T any](slice *[]T, spare int) {
	if slice == nil {
		return
	}
	if cap(*slice) >= len(*slice)+spare {
		return
	}
	next := make([]T, len(*slice), len(*slice)+spare)
	copy(next, *slice)
	*slice = next
}

func normalizeBattleSide(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "player", "enemy":
		return strings.ToLower(strings.TrimSpace(raw))
	default:
		return ""
	}
}

func battleOppositeSide(side string) string {
	if normalizeBattleSide(side) == "enemy" {
		return "player"
	}
	return "enemy"
}

func normalizeBattleSummonOnCasterDeath(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case battleSummonOnCasterDeathDespawn:
		return battleSummonOnCasterDeathDespawn
	default:
		return battleSummonOnCasterDeathKeep
	}
}

func normalizeBattleSummonOnCapOverflow(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case battleSummonOnCapOverflowFail:
		return battleSummonOnCapOverflowFail
	default:
		return battleSummonOnCapOverflowReplace
	}
}

func normalizeBattleSummonOnDispel(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case battleSummonOnDispelIgnore:
		return battleSummonOnDispelIgnore
	default:
		return battleSummonOnDispelDespawn
	}
}

func normalizeBattleSummonOnDurationEnd(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case battleSummonOnDurationEndDespawn:
		return battleSummonOnDurationEndDespawn
	default:
		return battleSummonOnDurationEndDespawn
	}
}

func battleUnitTeamSide(unit *battleUnitState) string {
	if unit == nil {
		return ""
	}
	if side := normalizeBattleSide(unit.OriginSide); side != "" {
		return side
	}
	if side := normalizeBattleSide(unit.BaseSide); side != "" {
		return side
	}
	return normalizeBattleSide(unit.Side)
}

func battleUnitPerspectiveSide(unit *battleUnitState) string {
	if unit == nil {
		return ""
	}
	if side := normalizeBattleSide(unit.PerspectiveSide); side != "" {
		return side
	}
	return battleUnitTeamSide(unit)
}

type battleStatProjection struct {
	BaseStats map[string]float64
	Stats     map[string]float64
}

func projectBattleStats(attrStates []store.PlayerAttributeState, derived map[string]float64, preferLatestDuplicate bool) battleStatProjection {
	baseStats := make(map[string]float64, len(attrStates))
	stats := make(map[string]float64, len(attrStates))
	latestLockVersionByAttr := make(map[string]int64, len(attrStates))
	for _, row := range attrStates {
		key := strings.TrimSpace(strings.ToLower(row.AttrKey))
		if key == "" {
			continue
		}
		if preferLatestDuplicate {
			if latest, exists := latestLockVersionByAttr[key]; exists && row.LockVersion < latest {
				continue
			}
			latestLockVersionByAttr[key] = row.LockVersion
		}
		finalValue := roundTo4(row.FinalValue)
		stats[key] = finalValue
		baseStats[key] = battleBaseStatFromSourceMods(row.SourceMods, finalValue)
	}
	for key, value := range derived {
		key = strings.TrimSpace(strings.ToLower(key))
		if key == "" {
			continue
		}
		if _, exists := stats[key]; exists {
			continue
		}
		stats[key] = roundTo4(value)
		if _, ok := baseStats[key]; !ok {
			baseStats[key] = battleBaseStatFromSourceMods(nil, value)
		}
	}
	for key, value := range stats {
		if _, ok := baseStats[key]; !ok {
			baseStats[key] = roundTo4(value)
		}
	}
	return battleStatProjection{BaseStats: baseStats, Stats: stats}
}

func projectBattlePlayerStats(attrStates []store.PlayerAttributeState, derived map[string]float64) battleStatProjection {
	return projectBattleStats(attrStates, derived, false)
}

func projectBattleKittyStats(attrStates []store.PlayerAttributeState, derived map[string]float64) battleStatProjection {
	return projectBattleStats(attrStates, derived, true)
}

func battleLevelFromAttributeStates(attrStates []store.PlayerAttributeState) int64 {
	level := int64(1)
	lockVersion := int64(-1)
	for _, row := range attrStates {
		if !strings.EqualFold(strings.TrimSpace(row.AttrKey), "battle") || row.LockVersion < lockVersion {
			continue
		}
		lockVersion = row.LockVersion
		level = maxInt64(1, row.BaseLevel)
	}
	return level
}

func (s *Server) battleProjectDerivedFinalStats(base map[string]float64) map[string]float64 {
	finalStats := cloneFloatMap(base)
	for _, formula := range s.deps.Config.GlobalSettings.DerivedStats.Formulas {
		targetAttr := strings.TrimSpace(strings.ToLower(formula.TargetAttr))
		if targetAttr == "" {
			continue
		}
		if !formula.IncludeBase {
			finalStats[targetAttr] = 0
		}
		total := 0.0
		for _, term := range formula.Terms {
			attrKey := strings.TrimSpace(strings.ToLower(term.AttrKey))
			if attrKey == "" {
				continue
			}
			total += finalStats[attrKey] * term.Coefficient
		}
		finalStats[targetAttr] = roundTo4(finalStats[targetAttr] + total)
	}
	return finalStats
}

func battleBaseStatFromSourceMods(sourceMods map[string]any, fallback float64) float64 {
	if len(sourceMods) == 0 {
		return roundTo4(fallback)
	}
	base := normalizeAnyMap(sourceMods["base"])
	if len(base) == 0 {
		return roundTo4(fallback)
	}
	return roundTo4(toFloat64(base["value"], fallback))
}

type battleRunCompileResult struct {
	CompileInput     map[string]any
	Wave             int
	Timeline         []map[string]any
	PlaybackTimeline []wsbus.BattleTimelineEvent
	FinalResult      map[string]any
	Checkpoint       map[string]any
	PlayerUnits      int
	MaxSteps         int
	TotalTurns       int64
	TotalSteps       int
	TimedOut         bool
	StepLimited      bool
}

type battleTickPipeline struct {
	room         store.BattleRoom
	detail       store.BattleRoomDetail
	run          store.BattleRun
	worldReady   bool
	worldCfg     battleWorldConfig
	enemyCatalog map[string]battleEnemyConfig
	now          time.Time
}

type battleCompileKickoffTask struct {
	RoomID      int64
	RunID       int64
	Source      string
	StartedAtMs int64
}

type battleCompileRetryState struct {
	Attempts  int
	NotBefore time.Time
}

var errBattlePlaybackFinished = errors.New("battle playback finished")

const battleFinalizeOperationTimeout = 10 * time.Second

const battleCompileRetryMaxDelay = 30 * time.Second
const battleCompileInvalidCarryRetryMaxDelay = 30 * time.Minute
const battleCompileRetryStateTTL = 10 * time.Minute
const battleCompileRetryStateMaxEntries = 4096
const battlePlaybackLeaseTTL = 4 * time.Second
const battlePlaybackLeaseRenewThreshold = 1 * time.Second
const battlePlaybackFirstEventDelay = 200 * time.Millisecond
const battleCompileSlowLogThreshold = 600 * time.Millisecond
const battleCompilePendingRetryDelay = 80 * time.Millisecond
const battleCompileBackpressureRetryDelay = 120 * time.Millisecond
const battleCompileQueueSlowLogThresholdMs = int64(1000)
const battleCompileTailStageSlowLogThresholdMs = int64(100)
const battleCompileTailPostSlowLogThresholdMs = int64(200)
const battleCompileTailTotalSlowLogThresholdMs = int64(500)
const battleRunFirstPushSlowLogThresholdMs = int64(2500)
const battleFinalizeSlowLogThreshold = 500 * time.Millisecond
const battlePlaybackCheckpointTTL = 30 * time.Minute
const battlePlaybackCacheEntryTTL = 10 * time.Minute
const battlePlaybackCacheSweepInterval = 30 * time.Second
const battlePlaybackCheckpointRedisLoadSlowThreshold = 250 * time.Millisecond
const battlePlaybackCheckpointRedisLoadSevereThreshold = 2 * time.Second

func (s *Server) processBattleRoomTick(ctx context.Context, workerID string, room store.BattleRoom, now time.Time) (battleTickRoomMetrics, time.Time, bool, error) {
	metrics := battleTickRoomMetrics{}
	pipe := battleTickPipeline{
		room: room,
		now:  now,
	}
	loadDetailStartedAt := time.Now()
	if err := s.loadBattleTickRoom(ctx, &pipe); err != nil {
		return metrics, time.Time{}, false, err
	}
	metrics.LoadDetailElapsed = time.Since(loadDetailStartedAt)
	ensureRunStartedAt := time.Now()
	if err := s.ensureBattleTickRun(ctx, &pipe); err != nil {
		return metrics, time.Time{}, false, err
	}
	metrics.EnsureRunElapsed = time.Since(ensureRunStartedAt)
	if pipe.detail.Room.CurrentRunID == nil {
		return metrics, now.Add(battleCompileBackpressureRetryDelay), false, nil
	}
	loadRuntimeStartedAt := time.Now()
	if err := s.loadBattleTickRuntime(ctx, &pipe); err != nil {
		return metrics, time.Time{}, false, err
	}
	metrics.LoadRuntimeElapsed = time.Since(loadRuntimeStartedAt)
	if !pipe.worldReady {
		return metrics, time.Time{}, false, nil
	}
	if pipe.run.Status == store.BattleRunStatusSettling {
		return metrics, now.Add(battlePlaybackLeaseTTL), false, nil
	}
	compileStartedAt := time.Now()
	compileMetrics, err := s.compileBattleTickRunIfNeeded(ctx, &pipe)
	if err != nil {
		return metrics, time.Time{}, false, err
	}
	if compileMetrics.CompiledRuns > 0 {
		metrics.CompileElapsed = time.Since(compileStartedAt)
	}
	metrics.CompiledRuns += compileMetrics.CompiledRuns
	metrics.CompiledTurns += compileMetrics.CompiledTurns
	metrics.CompileTimedOut = compileMetrics.CompileTimedOut
	metrics.CompileStepLimit = compileMetrics.CompileStepLimit
	plan := s.buildBattleTickExecutionPlan(&pipe, now)
	nextTickAt, asyncPlayback, err := s.executeBattleTickExecutionPlan(ctx, workerID, &pipe, plan, now, &metrics)
	if err != nil {
		return metrics, time.Time{}, false, err
	}
	return metrics, nextTickAt, asyncPlayback, nil
}

func (s *Server) handleBattleRunStarted(
	detail store.BattleRoomDetail,
	seed int64,
	now time.Time,
	startPath string,
	compileSource string,
	includeReadyWait bool,
	ctx context.Context,
) {
	s.pushBattleRoomEventByMembers(detail.Members, "battle.run.started", fiber.Map{
		"roomId": detail.Room.ID,
		"runId":  detail.Room.CurrentRunID,
		"seed":   seed,
	}, ctx)
	if detail.Room.CurrentRunID == nil {
		return
	}
	startedAtMs := now.UnixMilli()
	gapMs := s.popBattleRunStartGapMs(detail.Room.ID, startedAtMs)
	waitFromReadyMs := int64(-1)
	if includeReadyWait {
		waitFromReadyMs = s.popBattleRoomReadyWaitMs(detail.Room.ID, startedAtMs)
	}
	expectedGapMs := expectedBattleRunStartGapMs(
		startPath,
		s.deps.Config.GlobalSettings.Battle.InterBattleIntervalMs,
		s.deps.Config.GlobalSettings.Battle.RecoveryDurationMs,
	)
	gapLateMs := battleRunStartLateMs(gapMs, expectedGapMs)
	if shouldLogBattleRunStart(gapLateMs, waitFromReadyMs) {
		log.Printf(
			"battle run started slow room_id=%d run_id=%d started_at_ms=%d path=%s gap_since_prev_finish_ms=%d expected_gap_ms=%d gap_late_ms=%d wait_since_ready_ms=%d",
			detail.Room.ID,
			*detail.Room.CurrentRunID,
			startedAtMs,
			startPath,
			gapMs,
			expectedGapMs,
			gapLateMs,
			waitFromReadyMs,
		)
	}
	if !s.tryKickoffBattleCompile(detail, "", compileSource, startedAtMs) {
		s.enqueueBattleDueRoomForRoom(detail.Room, now)
	}
}

func (s *Server) markBattleRunFinished(roomID int64, finishedAtMs int64) {
	if roomID <= 0 || finishedAtMs <= 0 {
		return
	}
	s.battleGapMu.Lock()
	s.battleLastFinishMs[roomID] = finishedAtMs
	s.battleGapMu.Unlock()
}

func (s *Server) markBattleRoomReadyForNext(roomID int64, readyAtMs int64) {
	if roomID <= 0 || readyAtMs <= 0 {
		return
	}
	s.battleGapMu.Lock()
	s.battleNextReadyMs[roomID] = readyAtMs
	s.battleGapMu.Unlock()
}

func (s *Server) popBattleRoomReadyWaitMs(roomID int64, startedAtMs int64) int64 {
	if roomID <= 0 || startedAtMs <= 0 {
		return -1
	}
	s.battleGapMu.Lock()
	readyAtMs, ok := s.battleNextReadyMs[roomID]
	if ok {
		delete(s.battleNextReadyMs, roomID)
	}
	s.battleGapMu.Unlock()
	if !ok || readyAtMs <= 0 {
		return -1
	}
	return maxInt64(0, startedAtMs-readyAtMs)
}

func (s *Server) popBattleRunStartGapMs(roomID int64, startedAtMs int64) int64 {
	if roomID <= 0 || startedAtMs <= 0 {
		return -1
	}
	s.battleGapMu.Lock()
	finishedAtMs, ok := s.battleLastFinishMs[roomID]
	if ok {
		delete(s.battleLastFinishMs, roomID)
	}
	s.battleGapMu.Unlock()
	if !ok || finishedAtMs <= 0 {
		return -1
	}
	return maxInt64(0, startedAtMs-finishedAtMs)
}

func (s *Server) tryKickoffBattleCompile(detail store.BattleRoomDetail, _ string, source string, startedAtMs int64) bool {
	if detail.Room.CurrentRunID == nil || *detail.Room.CurrentRunID <= 0 {
		return false
	}
	roomID := detail.Room.ID
	runID := *detail.Room.CurrentRunID
	s.battleCompileRunMu.Lock()
	_, exists := s.battleCompileRuns[runID]
	s.battleCompileRunMu.Unlock()
	if exists {
		return true
	}
	if !s.battleCompileRetryReady(runID, time.Now()) {
		return false
	}
	if !s.battleCompileQueuePressure().allowsKickoff(source) {
		return false
	}
	s.battleCompileRunMu.Lock()
	if _, exists = s.battleCompileRuns[runID]; exists {
		s.battleCompileRunMu.Unlock()
		return true
	}
	s.battleCompileRuns[runID] = struct{}{}
	s.battleCompileRunMu.Unlock()
	task := battleCompileKickoffTask{
		RoomID:      roomID,
		RunID:       runID,
		Source:      normalizeBattleCompileSource(source),
		StartedAtMs: startedAtMs,
	}
	queue := s.battleCompileQueue
	queueClass := "regular"
	if isBattleCompileFirstSnapshotSource(task.Source) && s.battleCompileFirstQ != nil {
		queue = s.battleCompileFirstQ
		queueClass = "first_snapshot"
	}
	select {
	case queue <- task:
		return true
	default:
		s.battleCompileRunMu.Lock()
		delete(s.battleCompileRuns, runID)
		s.battleCompileRunMu.Unlock()
		log.Printf("battle compile kickoff queue full room_id=%d run_id=%d source=%s queue_class=%s", roomID, runID, source, queueClass)
		return false
	}
}

func battleCompileRetryDelay(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	delay := time.Second
	for step := 1; step < attempt && delay < battleCompileRetryMaxDelay; step++ {
		delay *= 2
	}
	if delay > battleCompileRetryMaxDelay {
		return battleCompileRetryMaxDelay
	}
	return delay
}

func battleCompileRetryDelayForError(attempt int, cause error) time.Duration {
	if !errors.Is(cause, errBattlePreviousWaveCarryInvalid) {
		return battleCompileRetryDelay(attempt)
	}
	if attempt < 1 {
		attempt = 1
	}
	delay := battleCompileRetryMaxDelay
	for step := 1; step < attempt && delay < battleCompileInvalidCarryRetryMaxDelay; step++ {
		delay *= 2
	}
	if delay > battleCompileInvalidCarryRetryMaxDelay {
		return battleCompileInvalidCarryRetryMaxDelay
	}
	return delay
}

func (s *Server) nextBattleCompileRetryDelay(runID int64, cause error) time.Duration {
	s.battleCompileRetryMu.Lock()
	defer s.battleCompileRetryMu.Unlock()
	if s.battleCompileRetries == nil {
		s.battleCompileRetries = make(map[int64]battleCompileRetryState)
	}
	now := time.Now()
	for candidateRunID, candidate := range s.battleCompileRetries {
		if now.After(candidate.NotBefore.Add(battleCompileRetryStateTTL)) {
			delete(s.battleCompileRetries, candidateRunID)
		}
	}
	if _, exists := s.battleCompileRetries[runID]; !exists && len(s.battleCompileRetries) >= battleCompileRetryStateMaxEntries {
		var (
			oldestRunID int64
			oldestAt    time.Time
		)
		for candidateRunID, candidate := range s.battleCompileRetries {
			if oldestRunID == 0 || candidate.NotBefore.Before(oldestAt) {
				oldestRunID = candidateRunID
				oldestAt = candidate.NotBefore
			}
		}
		delete(s.battleCompileRetries, oldestRunID)
	}
	retry := s.battleCompileRetries[runID]
	retry.Attempts++
	delay := battleCompileRetryDelayForError(retry.Attempts, cause)
	retry.NotBefore = now.Add(delay)
	s.battleCompileRetries[runID] = retry
	return delay
}

func (s *Server) battleCompileRetryReady(runID int64, now time.Time) bool {
	if runID <= 0 {
		return false
	}
	s.battleCompileRetryMu.Lock()
	defer s.battleCompileRetryMu.Unlock()
	retry, exists := s.battleCompileRetries[runID]
	return !exists || !now.Before(retry.NotBefore)
}

func (s *Server) clearBattleCompileRetry(runID int64) {
	if runID <= 0 {
		return
	}
	s.battleCompileRetryMu.Lock()
	delete(s.battleCompileRetries, runID)
	s.battleCompileRetryMu.Unlock()
}

func (s *Server) scheduleBattleCompileRetry(workerID string, task battleCompileKickoffTask, cause error) {
	if task.RoomID <= 0 || task.RunID <= 0 {
		return
	}
	delay := s.nextBattleCompileRetryDelay(task.RunID, cause)
	retryAt := time.Now().Add(delay)
	retryCtx, retryCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer retryCancel()
	scheduled, err := s.deps.Store.ScheduleBattleCompileRetry(retryCtx, task.RoomID, task.RunID, workerID, retryAt)
	if err != nil {
		log.Printf("battle compile retry schedule failed worker=%s room_id=%d run_id=%d source=%s retry_delay_ms=%d err=%v original_err=%v", workerID, task.RoomID, task.RunID, task.Source, delay.Milliseconds(), err, cause)
		return
	}
	if !scheduled {
		s.clearBattleCompileRetry(task.RunID)
		return
	}
	log.Printf("battle compile retry scheduled worker=%s room_id=%d run_id=%d source=%s retry_delay_ms=%d err=%v", workerID, task.RoomID, task.RunID, task.Source, delay.Milliseconds(), cause)
}

func (s *Server) startBattleCompileKickoffWorkers(workerID string) {
	workerCount := cap(s.battleCompileSlots)
	if workerCount <= 0 {
		workerCount = 1
	}
	for i := 0; i < workerCount; i++ {
		localWorkerID := fmt.Sprintf("%s:compile:%d", workerID, i)
		playbackLeaseWorkerID := strings.TrimSpace(workerID)
		preferFirst := battleCompileWorkerPrefersFirst(i, workerCount)
		s.launchBattleTask(func() {
			for {
				task, fromFirst, ok := s.takeBattleCompileTask(preferFirst)
				if !ok {
					return
				}
				select {
				case <-s.battleStopCh:
					return
				default:
				}
				if workerCount == 1 {
					preferFirst = !fromFirst
				}
				s.executeBattleCompileKickoff(playbackLeaseWorkerID, localWorkerID, task)
			}
		})
	}
}

func battleCompileWorkerPrefersFirst(workerIndex int, workerCount int) bool {
	if workerCount <= 1 {
		return true
	}
	// Keep one regular-preferring lane for durable retries; all other workers
	// minimize first-snapshot latency. Either class may borrow idle capacity.
	return workerIndex >= 0 && workerIndex < workerCount-1
}

func (s *Server) takeBattleCompileTask(preferFirst bool) (battleCompileKickoffTask, bool, bool) {
	preferred := s.battleCompileQueue
	fallback := s.battleCompileFirstQ
	preferredIsFirst := false
	if preferFirst {
		preferred = s.battleCompileFirstQ
		fallback = s.battleCompileQueue
		preferredIsFirst = true
	}
	select {
	case <-s.battleStopCh:
		return battleCompileKickoffTask{}, false, false
	default:
	}
	select {
	case task := <-preferred:
		return task, preferredIsFirst, true
	default:
	}
	select {
	case <-s.battleStopCh:
		return battleCompileKickoffTask{}, false, false
	case task := <-preferred:
		return task, preferredIsFirst, true
	case task := <-fallback:
		return task, !preferredIsFirst, true
	}
}

func (s *Server) executeBattleCompileKickoff(playbackLeaseWorkerID string, logWorkerID string, task battleCompileKickoffTask) {
	playbackLeaseWorkerID = strings.TrimSpace(playbackLeaseWorkerID)
	if playbackLeaseWorkerID == "" {
		playbackLeaseWorkerID = strings.TrimSpace(logWorkerID)
	}
	defer func() {
		s.battleCompileRunMu.Lock()
		delete(s.battleCompileRuns, task.RunID)
		s.battleCompileRunMu.Unlock()
	}()
	if task.RoomID <= 0 || task.RunID <= 0 {
		return
	}
	compileStartedAt := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	freshDetail, err := s.loadBattleRoomDetailFresh(ctx, task.RoomID)
	if err != nil {
		s.scheduleBattleCompileRetry(playbackLeaseWorkerID, task, fmt.Errorf("load room detail: %w", err))
		return
	}
	if freshDetail.Room.CurrentRunID == nil || *freshDetail.Room.CurrentRunID != task.RunID {
		s.clearBattleCompileRetry(task.RunID)
		log.Printf(
			"battle compile kickoff room detail mismatch worker=%s room_id=%d run_id=%d source=%s current_run_id=%v",
			logWorkerID,
			task.RoomID,
			task.RunID,
			task.Source,
			freshDetail.Room.CurrentRunID,
		)
		return
	}
	run, err := s.deps.Store.GetBattleRunByRoomLite(ctx, task.RoomID)
	if err != nil {
		s.scheduleBattleCompileRetry(playbackLeaseWorkerID, task, fmt.Errorf("load run: %w", err))
		return
	}
	if run.ID != task.RunID || run.Status != store.BattleRunStatusCompiled {
		s.clearBattleCompileRetry(task.RunID)
		return
	}
	worldCfg, ok := s.battleWorldConfigByID(freshDetail.Room.DungeonID)
	if !ok {
		s.scheduleBattleCompileRetry(playbackLeaseWorkerID, task, fmt.Errorf("battle world config %q is unavailable", freshDetail.Room.DungeonID))
		return
	}
	compiled, compileErr := s.compileBattleRun(ctx, freshDetail, run, worldCfg, s.battleEnemyCatalog(), time.Now())
	if compileErr != nil {
		log.Printf("battle compile kickoff failed worker=%s room_id=%d run_id=%d source=%s err=%v", logWorkerID, task.RoomID, task.RunID, task.Source, compileErr)
		s.scheduleBattleCompileRetry(playbackLeaseWorkerID, task, compileErr)
		return
	}
	if compiled.TimedOut || compiled.StepLimited {
		log.Printf(
			"battle run compile guard triggered room_id=%d run_id=%d timed_out=%t step_limited=%t total_steps=%d total_turns=%d budget_steps=%d player_units=%d",
			freshDetail.Room.ID,
			run.ID,
			compiled.TimedOut,
			compiled.StepLimited,
			compiled.TotalSteps,
			compiled.TotalTurns,
			compiled.MaxSteps,
			compiled.PlayerUnits,
		)
		log.Printf(
			"battle run compile suspicious room_id=%d run_id=%d dungeon_id=%s wave=%d timeline_events=%d final_seq=%d compile_elapsed_ms=%d budget_steps=%d player_units=%d",
			freshDetail.Room.ID,
			run.ID,
			freshDetail.Room.DungeonID,
			compiled.Wave,
			len(compiled.Timeline),
			toInt64Loose(compiled.Checkpoint["finalSeq"], 0),
			maxInt64(0, toInt64Loose(compiled.Checkpoint["compileFinishedAtMs"], 0)-toInt64Loose(compiled.Checkpoint["compileStartedAtMs"], 0)),
			compiled.MaxSteps,
			compiled.PlayerUnits,
		)
	}
	compileElapsedMs := maxInt64(0, toInt64Loose(compiled.Checkpoint["compileFinishedAtMs"], 0)-toInt64Loose(compiled.Checkpoint["compileStartedAtMs"], 0))
	turnProfileTurns := battleCheckpointTurnProfileMetric(compiled.Checkpoint, "turns")
	turnProfileTotalMs := battleCheckpointTurnProfileMetric(compiled.Checkpoint, "totalMs")
	turnProfilePreActionMs := battleCheckpointTurnProfileMetric(compiled.Checkpoint, "preActionMs")
	turnProfileExecuteMs := battleCheckpointTurnProfileMetric(compiled.Checkpoint, "executeMs")
	turnProfilePostActionMs := battleCheckpointTurnProfileMetric(compiled.Checkpoint, "postActionMs")
	if time.Duration(compileElapsedMs)*time.Millisecond >= battleCompileSlowLogThreshold {
		log.Printf(
			"battle run compile slow room_id=%d run_id=%d wave=%d compile_elapsed_ms=%d total_steps=%d total_turns=%d timeline_events=%d final_seq=%d timed_out=%t step_limited=%t budget_steps=%d player_units=%d turn_profile_turns=%d turn_profile_total_ms=%d turn_profile_pre_action_ms=%d turn_profile_execute_ms=%d turn_profile_post_action_ms=%d",
			freshDetail.Room.ID,
			run.ID,
			compiled.Wave,
			compileElapsedMs,
			compiled.TotalSteps,
			compiled.TotalTurns,
			len(compiled.Timeline),
			toInt64Loose(compiled.Checkpoint["finalSeq"], 0),
			compiled.TimedOut,
			compiled.StepLimited,
			compiled.MaxSteps,
			compiled.PlayerUnits,
			turnProfileTurns,
			turnProfileTotalMs,
			turnProfilePreActionMs,
			turnProfileExecuteMs,
			turnProfilePostActionMs,
		)
	}
	saveStartedAt := time.Now()
	timelineJSON, saveErr := marshalBattleTimelineForPersistence(compiled.Timeline, compiled.PlaybackTimeline)
	saved := false
	if saveErr == nil {
		saved, saveErr = s.deps.Store.SaveBattleRunCompiledWithTimelineJSONIfStatus(
			ctx,
			run.ID,
			compiled.Wave,
			store.BattleRunStatusRunning,
			compiled.CompileInput,
			compiled.Checkpoint,
			timelineJSON,
			battleRunResultSummary(compiled.FinalResult),
			normalizeAnyMap(compiled.FinalResult["carryState"]),
			compiled.FinalResult,
			store.BattleRunStatusCompiled,
		)
	}
	saveElapsedMs := time.Since(saveStartedAt).Milliseconds()
	if saveErr != nil || !saved {
		if saveErr != nil {
			log.Printf("battle compile kickoff save failed worker=%s room_id=%d run_id=%d source=%s err=%v", logWorkerID, task.RoomID, task.RunID, task.Source, saveErr)
			s.scheduleBattleCompileRetry(playbackLeaseWorkerID, task, saveErr)
		} else {
			s.clearBattleCompileRetry(task.RunID)
		}
		return
	}
	s.clearBattleCompileRetry(task.RunID)
	playbackCacheStartedAt := time.Now()
	s.setBattleRunPlaybackCache(run.ID, compiled.PlaybackTimeline)
	playbackCacheElapsedMs := time.Since(playbackCacheStartedAt).Milliseconds()
	playbackStartStartedAt := time.Now()
	if !s.tryStartBattlePlaybackAfterCompile(ctx, playbackLeaseWorkerID, freshDetail, run, compiled, time.Now()) {
		s.enqueueBattleDueRoomForRoom(freshDetail.Room, time.Now())
		log.Printf("battle compile playback handoff missed worker=%s room_id=%d run_id=%d source=%s", logWorkerID, task.RoomID, task.RunID, task.Source)
	} else {
		s.pushBattleRoomEventByMembers(freshDetail.Members, "battle.run.compiled", fiber.Map{
			"roomId": freshDetail.Room.ID,
			"runId":  run.ID,
		}, ctx)
	}
	playbackStartElapsedMs := time.Since(playbackStartStartedAt).Milliseconds()
	queueSinceStartMs := maxInt64(0, compileStartedAt.UnixMilli()-task.StartedAtMs)
	preArchiveTotalMs := time.Since(compileStartedAt).Milliseconds()
	archiveStartedAt := time.Now()
	s.captureBattleSlowRunArchive(ctx, logWorkerID, freshDetail, run, task, compiled, queueSinceStartMs, compileElapsedMs, preArchiveTotalMs)
	archiveElapsedMs := time.Since(archiveStartedAt).Milliseconds()
	totalMs := time.Since(compileStartedAt).Milliseconds()
	postCompileTailMs := maxInt64(0, totalMs-compileElapsedMs)
	if shouldLogBattleCompileTail(saveElapsedMs, archiveElapsedMs, playbackStartElapsedMs, postCompileTailMs, totalMs) {
		log.Printf(
			"battle compile tail slow worker=%s room_id=%d run_id=%d source=%s compile_elapsed_ms=%d save_ms=%d playback_cache_ms=%d playback_start_ms=%d archive_ms=%d post_compile_tail_ms=%d total_ms=%d timeline_events=%d final_seq=%d checkpoint_players=%d checkpoint_enemies=%d",
			logWorkerID,
			task.RoomID,
			task.RunID,
			task.Source,
			compileElapsedMs,
			saveElapsedMs,
			playbackCacheElapsedMs,
			playbackStartElapsedMs,
			archiveElapsedMs,
			postCompileTailMs,
			totalMs,
			len(compiled.Timeline),
			toInt64Loose(compiled.Checkpoint["finalSeq"], 0),
			len(battleAnyMapSlice(compiled.Checkpoint["players"])),
			len(battleAnyMapSlice(compiled.Checkpoint["enemies"])),
		)
	}
	if queueSinceStartMs >= battleCompileQueueSlowLogThresholdMs || time.Duration(compileElapsedMs)*time.Millisecond >= battleCompileSlowLogThreshold || totalMs >= battleCompileQueueSlowLogThresholdMs {
		log.Printf(
			"battle compile kickoff slow worker=%s room_id=%d run_id=%d source=%s queue_since_start_ms=%d compile_elapsed_ms=%d total_ms=%d budget_steps=%d player_units=%d turn_profile_turns=%d turn_profile_total_ms=%d turn_profile_pre_action_ms=%d turn_profile_execute_ms=%d turn_profile_post_action_ms=%d",
			logWorkerID,
			task.RoomID,
			task.RunID,
			task.Source,
			queueSinceStartMs,
			compileElapsedMs,
			totalMs,
			compiled.MaxSteps,
			compiled.PlayerUnits,
			turnProfileTurns,
			turnProfileTotalMs,
			turnProfilePreActionMs,
			turnProfileExecuteMs,
			turnProfilePostActionMs,
		)
	}
}

func shouldLogBattleCompileTail(saveElapsedMs int64, archiveElapsedMs int64, playbackStartElapsedMs int64, postCompileTailMs int64, totalMs int64) bool {
	return saveElapsedMs >= battleCompileTailStageSlowLogThresholdMs ||
		archiveElapsedMs >= battleCompileTailStageSlowLogThresholdMs ||
		playbackStartElapsedMs >= battleCompileTailStageSlowLogThresholdMs ||
		postCompileTailMs >= battleCompileTailPostSlowLogThresholdMs ||
		totalMs >= battleCompileTailTotalSlowLogThresholdMs
}

func (s *Server) tryStartBattlePlaybackAfterCompile(
	ctx context.Context,
	workerID string,
	detail store.BattleRoomDetail,
	run store.BattleRun,
	compiled battleRunCompileResult,
	now time.Time,
) bool {
	reservation, reserved := s.newBattlePlaybackWorkerReservation(detail.Room.ID, run.ID, workerID)
	if !reserved {
		return false
	}
	leaseUntil := now.Add(battlePlaybackLeaseTTL)
	handoffOK, err := s.deps.Store.HandoffBattleRoomPlaybackLease(ctx, detail.Room.ID, run.ID, workerID, reservation.OwnerToken, leaseUntil, now)
	if err != nil {
		log.Printf("battle compile playback handoff failed worker=%s room_id=%d run_id=%d err=%v", workerID, detail.Room.ID, run.ID, err)
		return false
	}
	if !handoffOK {
		return false
	}
	run.Status = store.BattleRunStatusRunning
	run.Wave = compiled.Wave
	run.State = compiled.Checkpoint
	if !s.activateBattlePlaybackWorkerForRun(detail, reservation, now) {
		log.Printf("battle compile playback local reservation superseded worker=%s room_id=%d run_id=%d generation=%d", workerID, detail.Room.ID, run.ID, reservation.Generation)
		return false
	}
	initialTickCtx, initialTickCancel := context.WithTimeout(context.WithoutCancel(ctx), battlePlaybackTickOperationTimeout)
	leaseUntil, tickLeaseErr := s.ensureBattlePlaybackLeaseForTick(initialTickCtx, reservation, leaseUntil)
	if tickLeaseErr != nil {
		initialTickCancel()
		s.releaseBattlePlaybackWorkerForRun(detail, reservation)
		log.Printf("battle compile playback lease unavailable before initial tick worker=%s room_id=%d run_id=%d generation=%d err=%v", workerID, detail.Room.ID, run.ID, reservation.Generation, tickLeaseErr)
		return false
	}
	s.seedBattleRunPlaybackCheckpointLocal(
		run.ID,
		decodeBattlePlaybackCheckpoint(compiled.Checkpoint, time.Now()),
	)
	nextTickAt, tickResult, tickErr := s.playbackBattleRunTick(initialTickCtx, detail, run, time.Now())
	initialTickCancel()
	if tickErr != nil {
		if errors.Is(tickErr, errBattlePlaybackFinished) {
			s.releaseBattlePlaybackWorkerForRun(detail, reservation)
			return true
		}
		if errors.Is(tickErr, errBattlePlaybackPublishUnavailable) {
			nextTickAt = time.Now().Add(time.Second)
			if !s.updateBattlePlaybackWorkerState(reservation, func(state *battlePlaybackWorkerState) {
				state.LastTickAt = time.Now()
				state.LastNextTickAt = nextTickAt
			}) {
				return false
			}
			log.Printf("battle compile initial playback publish delayed worker=%s room_id=%d run_id=%d retry_ms=%d err=%v", workerID, detail.Room.ID, run.ID, time.Until(nextTickAt).Milliseconds(), tickErr)
		} else {
			s.releaseBattlePlaybackWorkerForRun(detail, reservation)
			log.Printf("battle compile initial playback tick failed worker=%s room_id=%d run_id=%d err=%v", workerID, detail.Room.ID, run.ID, tickErr)
			return false
		}
	} else if !s.updateBattlePlaybackWorkerState(reservation, func(state *battlePlaybackWorkerState) {
		state.LastTickAt = time.Now()
		state.LastNextTickAt = nextTickAt
		state.LastNextEventIdx = tickResult.NextEventIdx
		state.LastPushedSeq = tickResult.LastPushedSeq
		state.LastProgressAt = time.Now()
	}) {
		return false
	}
	leaseUntil, leaseErr := s.ensureBattlePlaybackLeaseBeforeStart(ctx, reservation, nextTickAt, leaseUntil)
	if leaseErr != nil {
		s.releaseBattlePlaybackWorkerForRun(detail, reservation)
		log.Printf("battle compile playback lease unavailable before worker start worker=%s room_id=%d run_id=%d generation=%d err=%v", workerID, detail.Room.ID, run.ID, reservation.Generation, leaseErr)
		return false
	}
	s.startReservedBattlePlaybackWorker(detail, run, reservation, nextTickAt, leaseUntil)
	return true
}

func (s *Server) loadBattleTickRoom(_ context.Context, pipe *battleTickPipeline) error {
	pipe.detail.Room = pipe.room
	return nil
}

func (s *Server) loadBattleTickMembersIfNeeded(ctx context.Context, pipe *battleTickPipeline) error {
	if len(pipe.detail.Members) > 0 {
		return nil
	}
	ids, err := s.loadBattleRoomMemberIDsCached(ctx, pipe.room.ID)
	if err != nil {
		return err
	}
	members := make([]store.BattleRoomMember, 0, len(ids))
	for _, uid := range ids {
		members = append(members, store.BattleRoomMember{UserID: uid})
	}
	pipe.detail.Members = members
	return nil
}

func (s *Server) ensureBattleTickRun(ctx context.Context, pipe *battleTickPipeline) error {
	if pipe.detail.Room.CurrentRunID == nil {
		if s.battleCompileQueuePressured() {
			return nil
		}
		nextDetail, seed, startErr := s.deps.Store.StartNextBattleRoomRun(ctx, pipe.room.ID, pipe.now)
		if startErr != nil {
			return startErr
		}
		s.pushBattleRoomUpdated(nextDetail, ctx)
		s.handleBattleRunStarted(nextDetail, seed, pipe.now, battleCompileSourceEnsureRun, battleCompileSourceEnsureRun, true, ctx)
		pipe.detail = nextDetail
	}
	return nil
}

func (s *Server) loadBattleTickRuntime(ctx context.Context, pipe *battleTickPipeline) error {
	if pipe.detail.Room.CurrentRunID == nil {
		return nil
	}
	run, err := s.deps.Store.GetBattleRunByRoomLite(ctx, pipe.room.ID)
	if err != nil {
		return err
	}
	worldCfg, ok := s.battleWorldConfigByID(pipe.room.DungeonID)
	if !ok {
		return nil
	}
	pipe.run = run
	pipe.worldReady = true
	pipe.worldCfg = worldCfg
	pipe.enemyCatalog = s.battleEnemyCatalog()
	return nil
}

func (s *Server) compileBattleTickRunIfNeeded(ctx context.Context, pipe *battleTickPipeline) (battleTickRoomMetrics, error) {
	_ = ctx
	metrics := battleTickRoomMetrics{}
	if pipe.run.Status == store.BattleRunStatusCompiled {
		startedAtMs := pipe.run.StartedAt.UnixMilli()
		_ = s.tryKickoffBattleCompile(pipe.detail, "", battleCompileSourceTickRetry, startedAtMs)
	}
	return metrics, nil
}

func (s *Server) initBattleRuntimeState(
	ctx context.Context,
	detail store.BattleRoomDetail,
	run store.BattleRun,
	worldCfg battleWorldConfig,
	enemyCatalog map[string]battleEnemyConfig,
	now time.Time,
) (battleRuntimeState, error) {
	wave := run.Wave
	if wave <= 0 {
		wave = 1
	}
	var previousCarry *battlePreviousWaveCarryState
	if wave > 1 {
		decoded, err := s.loadBattlePreviousWaveCarryState(ctx, detail.Room.ID, run.ID, wave-1)
		if err != nil {
			return battleRuntimeState{}, err
		}
		previousCarry = &decoded
	}
	players, preparationOrder, err := s.buildBattleInitialPlayers(ctx, detail, run, now, previousCarry)
	if err != nil {
		return battleRuntimeState{}, err
	}
	if wave > 1 {
		var repairReport battleCarryRepairReport
		players, repairReport, err = battleRepairPreviousWavePlayers(detail, players, worldCfg.MaxTeamSize)
		if err != nil {
			return battleRuntimeState{}, err
		}
		preparationOrder = battleFilterPreparationOrder(preparationOrder, players)
		if repairReport.changed() {
			log.Printf(
				"battle carry repaired room_id=%d run_id=%d wave=%d empty_ids=%d duplicate_ids=%d unknown_formal=%d formal_over_cap=%d invalid_summons=%d non_persistent=%d excess_summons=%d restored_formal=%d restored_summon=%d",
				detail.Room.ID,
				run.ID,
				wave,
				repairReport.DroppedEmptyUnitIDs,
				repairReport.DroppedDuplicateUnitIDs,
				repairReport.DroppedUnknownFormalUnits,
				repairReport.DroppedFormalUnitsOverCap,
				repairReport.DroppedInvalidSummons,
				repairReport.DroppedNonPersistent,
				repairReport.DroppedExcessSummons,
				repairReport.RestoredFormalIdentity,
				repairReport.RestoredSummonMetadata,
			)
		}
		players = s.battleTickBattleStartBuffs(players)
	}

	state := battleRuntimeState{
		Seed:                         detail.Room.Seed,
		PassiveRunID:                 run.ID,
		RNGIndex:                     0,
		StartedAt:                    run.StartedAt.UnixMilli(),
		DifficultyLevel:              detail.Room.DifficultyLevel,
		DifficultyMult:               s.battleDifficultyStatMultiplier(detail.Room.DifficultyLevel),
		LastActionAt:                 0,
		Wave:                         wave,
		CompletedBattles:             0,
		TurnCursor:                   0,
		Seq:                          0,
		Players:                      players,
		Enemies:                      []battleUnitState{},
		Order:                        []string{},
		PreparationOrder:             preparationOrder,
		PreparationSeedOrder:         append([]string(nil), preparationOrder...),
		ActedUnits:                   []string{},
		BattlefieldEffects:           buildBattleInitialBattlefieldEffects(worldCfg, players, previousCarry),
		Events:                       []battleEvent{},
		IsFogMode:                    detail.Room.IsFogMode,
		RoomID:                       detail.Room.ID,
		DungeonID:                    detail.Room.DungeonID,
		battlefieldEffectDefinitions: append([]battleWorldBattlefieldEffect(nil), worldCfg.BattlefieldEffects...),
		battlefieldSpecialEnemyIDs:   stringSliceSet(worldCfg.SpecialEnemyIDs),
	}
	if previousCarry != nil {
		state.BattleHookBattleCounts = maps.Clone(previousCarry.BattleHookBattleCounts)
		state.BattleHookCooldownUntilTurns = maps.Clone(previousCarry.BattleHookCooldownUntilTurns)
		state.CompletedTurnCounts = maps.Clone(previousCarry.CompletedTurnCounts)
	}
	if raw, ok := run.State["passiveSources"]; ok {
		encoded, err := json.Marshal(raw)
		if err != nil {
			return battleRuntimeState{}, err
		}
		if err = json.Unmarshal(encoded, &state.PassiveSources); err != nil {
			return battleRuntimeState{}, err
		}
		for _, snapshot := range state.PassiveSources {
			if err = snapshot.Validate(); err != nil {
				return battleRuntimeState{}, err
			}
		}
	}
	s.spawnWaveEnemies(ctx, &state, worldCfg, enemyCatalog)
	// Ensure spare capacity: hooks may call addBattleSummon which appends to these slices.
	// Without this, appends can reallocate the backing array and invalidate hook-held pointers.
	ensureSliceSpareCap(&state.Players, 40)
	ensureSliceSpareCap(&state.Enemies, 16)
	battleStartActions := s.battleTriggerBattleStartEquipmentHooks(&state, now)
	waveStartActions := s.battleTriggerWaveStartEquipmentHooks(&state, now)
	battleStartActions = append(battleStartActions, waveStartActions...)
	battleStartActions = append(battleStartActions, s.battleFlushPendingUnitDeathEquipmentHooks(&state, now)...)
	for i := range state.Players {
		s.battleRecomputeUnitStats(&state.Players[i])
	}
	for i := range state.Enemies {
		s.battleRecomputeUnitStats(&state.Enemies[i])
	}
	s.rebuildBattleOrder(&state)
	state.emit("battle.wave.spawn", now, map[string]any{
		"wave":               state.Wave,
		"order":              append([]string(nil), state.Order...),
		"preparationOrder":   append([]string(nil), state.PreparationOrder...),
		"players":            battlePublicUnitSlice(state.Players),
		"enemies":            battlePublicUnitSlice(state.Enemies),
		"battlefieldEffects": battlePublicBattlefieldEffectStates(state.BattlefieldEffects),
		"battleStartActions": battleStartActions,
	})
	return state, nil
}

func stringSliceSet(values []string) map[string]struct{} {
	out := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out[value] = struct{}{}
		}
	}
	return out
}

func buildBattleInitialBattlefieldEffects(
	worldCfg battleWorldConfig,
	players []battleUnitState,
	previousCarry *battlePreviousWaveCarryState,
) []battlefieldEffectState {
	if len(worldCfg.BattlefieldEffects) == 0 {
		return nil
	}
	carriedByID := map[string]battlefieldEffectState{}
	if previousCarry != nil {
		for _, carried := range previousCarry.BattlefieldEffects {
			carriedByID[carried.ID] = carried
		}
	}
	out := make([]battlefieldEffectState, 0, len(worldCfg.BattlefieldEffects))
	for _, definition := range worldCfg.BattlefieldEffects {
		goal := battlefieldEffectEligibleAliveCount(definition, players, nil)
		if goal <= 0 {
			goal = 1
		}
		next := battlefieldEffectState{
			ID:        definition.ID,
			Remaining: definition.Counter.Initial,
			Progress:  0,
			Goal:      goal,
			Windows:   map[string]int{},
		}
		if definition.Counter.CarryProgressAcrossWaves {
			if carried, ok := carriedByID[definition.ID]; ok {
				next.Remaining = carried.Remaining
				if next.Remaining <= 0 {
					next.Remaining = definition.Counter.Initial
				}
				next.Progress = carried.Progress
				next.Windows = cloneBattlefieldWindows(carried.Windows)
			}
		}
		next.Progress = clampBattlefieldProgress(next.Progress, goal)
		out = append(out, next)
	}
	return out
}

func buildBattleMissingInitialBattlefieldEffects(
	worldCfg battleWorldConfig,
	players []battleUnitState,
	wave int,
	loadPreviousCarry func(expectedWave int) (battlePreviousWaveCarryState, error),
) []battlefieldEffectState {
	var previousCarry *battlePreviousWaveCarryState
	if wave > 1 && loadPreviousCarry != nil {
		if carried, err := loadPreviousCarry(wave - 1); err == nil {
			previousCarry = &carried
		}
	}
	return buildBattleInitialBattlefieldEffects(worldCfg, players, previousCarry)
}

func (s *Server) buildBattleInitialPlayers(ctx context.Context, detail store.BattleRoomDetail, run store.BattleRun, now time.Time, previousCarry *battlePreviousWaveCarryState) ([]battleUnitState, []string, error) {
	if run.Wave > 1 {
		if previousCarry == nil || len(previousCarry.Players) == 0 {
			return nil, nil, errBattlePreviousWaveCarryInvalid
		}
		carried := append([]battleUnitState(nil), previousCarry.Players...)
		for index := range carried {
			normalizeBattleUnitState(&carried[index])
		}
		if detail.Room.DisableEntryRound {
			return carried, nil, nil
		}
		return carried, battleFilterPreparationOrder(previousCarry.PreparationOrder, carried), nil
	}
	units := detail.Units
	if len(units) == 0 {
		for _, member := range detail.Members {
			units = append(units, store.BattleRoomUnit{
				RoomID:      member.RoomID,
				OwnerUserID: member.UserID,
				Username:    member.Username,
				UnitType:    "player",
				UnitID:      0,
				Position:    member.Position,
				IsReady:     member.IsReady,
				JoinedAt:    member.JoinedAt,
				UpdatedAt:   member.UpdatedAt,
			})
		}
	}
	players := make([]battleUnitState, 0, len(units))
	for _, roomUnit := range units {
		unit, err := s.buildBattleRoomUnitSnapshot(ctx, roomUnit, now)
		if err != nil {
			return nil, nil, err
		}
		players = append(players, unit)
	}
	return players, nil, nil
}

func (s *Server) buildBattleRoomUnitSnapshot(ctx context.Context, roomUnit store.BattleRoomUnit, now time.Time) (battleUnitState, error) {
	switch strings.TrimSpace(roomUnit.UnitType) {
	case "kitty":
		return s.buildBattleKittySnapshot(ctx, roomUnit, now)
	default:
		member := store.BattleRoomMember{
			RoomID:    roomUnit.RoomID,
			UserID:    roomUnit.OwnerUserID,
			Username:  roomUnit.Username,
			Position:  roomUnit.Position,
			IsReady:   roomUnit.IsReady,
			JoinedAt:  roomUnit.JoinedAt,
			UpdatedAt: roomUnit.UpdatedAt,
		}
		return s.buildBattlePlayerSnapshot(ctx, member, now)
	}
}

func playbackCheckpointNextTickAt(now time.Time, nextPushAtMs int64) time.Time {
	if nextPushAtMs <= 0 {
		return now.Add(battleLoopMinSleep)
	}
	nextTickAt := time.UnixMilli(nextPushAtMs)
	if nextTickAt.Before(now) {
		return now
	}
	return nextTickAt
}

func shouldSampleBattlePlaybackCadenceLog(runID int64, now time.Time) bool {
	if runID <= 0 {
		return false
	}
	const buckets int64 = 32
	slot := (now.Unix() / 10) % buckets
	if slot < 0 {
		slot = -slot
	}
	mod := runID % buckets
	if mod < 0 {
		mod = -mod
	}
	return mod == slot
}

func shouldSampleBattlePlaybackFailureLog(runID int64, now time.Time) bool {
	if !shouldSampleBattlePlaybackCadenceLog(runID, now) {
		return false
	}
	secondInBucket := now.Unix() % 10
	if secondInBucket < 0 {
		secondInBucket = -secondInBucket
	}
	return secondInBucket < 2
}

func collectPlaybackEventsAtCurrentOffset(timeline []wsbus.BattleTimelineEvent, startIdx int, maxEvents int) ([]wsbus.BattleTimelineEvent, int, bool) {
	if startIdx < 0 || startIdx >= len(timeline) {
		return nil, startIdx, false
	}
	offset := timeline[startIdx].OffsetMS
	endIdx := startIdx
	for endIdx < len(timeline) && timeline[endIdx].OffsetMS == offset {
		endIdx++
	}
	if maxEvents <= 0 {
		maxEvents = endIdx - startIdx
	}
	limitedEndIdx := startIdx + maxEvents
	if limitedEndIdx > endIdx {
		limitedEndIdx = endIdx
	}
	hasOffsetRemainder := limitedEndIdx < endIdx
	return append([]wsbus.BattleTimelineEvent(nil), timeline[startIdx:limitedEndIdx]...), limitedEndIdx, hasOffsetRemainder
}

func playbackNextDelayMs(baseIntervalMs int64, maxGapMs int64, timeline []wsbus.BattleTimelineEvent, nextEventIdx int, lastPushedOffsetMs int64) int64 {
	_ = maxGapMs
	_ = timeline
	_ = nextEventIdx
	_ = lastPushedOffsetMs
	if baseIntervalMs <= 0 {
		baseIntervalMs = 1
	}
	return baseIntervalMs
}

func playbackCheckpointAtTimelineEnd(checkpoint battlePlaybackCheckpoint, timeline []wsbus.BattleTimelineEvent) bool {
	return checkpoint.NextEventIdx >= len(timeline)
}

func (s *Server) battlePlaybackSyncState(ctx context.Context, detail store.BattleRoomDetail, run store.BattleRun, now time.Time) (map[string]any, bool) {
	s.maybeSweepBattlePlaybackLocalCache(now)
	timeline, err := s.loadBattleRunPlaybackCache(ctx, run.ID)
	if err != nil {
		return nil, false
	}
	checkpoint := s.loadBattleRunPlaybackCheckpoint(run.ID, run.State, now)
	checkpoint = s.rebuildBattlePlaybackCheckpoint(run.ID, checkpoint, run.State, timeline, now)
	checkpoint.Players = normalizePlaybackSnapshotUnits(checkpoint.Players)
	checkpoint.Enemies = normalizePlaybackSnapshotUnits(checkpoint.Enemies)
	return checkpoint.toMap(), true
}

func battlePlaybackCheckpointHasUnits(checkpoint battlePlaybackCheckpoint) bool {
	return len(checkpoint.Players) > 0 || len(checkpoint.Enemies) > 0
}

func battlePlaybackCheckpointWithFallback(
	checkpoint battlePlaybackCheckpoint,
	fallback map[string]any,
	timeline []wsbus.BattleTimelineEvent,
	now time.Time,
) battlePlaybackCheckpoint {
	if battlePlaybackCheckpointHasUnits(checkpoint) {
		return checkpoint
	}
	fallbackCheckpoint := decodeBattlePlaybackCheckpoint(fallback, now)
	if battlePlaybackCheckpointHasUnits(fallbackCheckpoint) {
		return battlePlaybackCheckpointFromFallback(fallbackCheckpoint, checkpoint, timeline)
	}
	return checkpoint
}

func battlePlaybackCheckpointFromFallback(
	base battlePlaybackCheckpoint,
	cursor battlePlaybackCheckpoint,
	timeline []wsbus.BattleTimelineEvent,
) battlePlaybackCheckpoint {
	if !battlePlaybackCheckpointHasUnits(base) {
		return cursor
	}
	original := cursor
	targetIdx := maxInt(base.NextEventIdx, cursor.NextEventIdx)
	if targetIdx < 0 {
		targetIdx = 0
	}
	if targetIdx > len(timeline) {
		targetIdx = len(timeline)
	}
	startIdx := locateNextTimelineIndexBySeq(timeline, base.Seq)
	if startIdx > targetIdx {
		targetIdx = startIdx
	}
	if startIdx < targetIdx {
		applyPlaybackEventsToCheckpoint(&base, timeline[startIdx:targetIdx])
	}
	base.PlaybackStartedAtMs = original.PlaybackStartedAtMs
	base.NextPushAtMs = original.NextPushAtMs
	base.LastDispatchAtMs = original.LastDispatchAtMs
	base.LastPushedSeq = original.LastPushedSeq
	base.LastPushedOffsetMs = original.LastPushedOffsetMs
	base.NextEventIdx = targetIdx
	base.SnapshotPushed = original.SnapshotPushed
	if base.LastPushedSeq <= 0 && targetIdx > 0 {
		base.LastPushedSeq = timeline[targetIdx-1].Seq
	}
	if base.LastPushedOffsetMs < 0 && targetIdx > 0 {
		base.LastPushedOffsetMs = timeline[targetIdx-1].OffsetMS
	}
	return base
}

func shouldDerivePlaybackCheckpointByTime(checkpoint battlePlaybackCheckpoint) bool {
	return !checkpoint.SnapshotPushed && checkpoint.LastPushedSeq <= 0 && checkpoint.NextEventIdx <= 0
}

func derivePlaybackCheckpointByTime(checkpoint battlePlaybackCheckpoint, timeline []wsbus.BattleTimelineEvent, _ map[string]any, now time.Time) battlePlaybackCheckpoint {
	if len(timeline) == 0 || checkpoint.PlaybackStartedAtMs <= 0 {
		return checkpoint
	}
	offset := now.UnixMilli() - checkpoint.PlaybackStartedAtMs
	if offset <= 0 {
		return checkpoint
	}
	targetIdx := sort.Search(len(timeline), func(i int) bool {
		return timeline[i].OffsetMS > offset
	})
	if targetIdx <= 0 {
		return checkpoint
	}
	applyPlaybackEventsToCheckpoint(&checkpoint, timeline[:targetIdx])
	checkpoint.NextEventIdx = targetIdx
	checkpoint.SnapshotPushed = true
	if targetIdx > 0 {
		checkpoint.LastPushedSeq = timeline[targetIdx-1].Seq
		checkpoint.LastPushedOffsetMs = timeline[targetIdx-1].OffsetMS
	}
	return checkpoint
}

func (s *Server) rebuildBattlePlaybackCheckpoint(
	runID int64,
	checkpoint battlePlaybackCheckpoint,
	fallback map[string]any,
	timeline []wsbus.BattleTimelineEvent,
	now time.Time,
) battlePlaybackCheckpoint {
	checkpoint.ensureTimelineCursor(timeline)
	restored := battlePlaybackCheckpointWithFallback(checkpoint, fallback, timeline, now)
	if !battlePlaybackCheckpointHasUnits(checkpoint) && battlePlaybackCheckpointHasUnits(restored) {
		s.setBattleRunPlaybackCheckpoint(runID, restored)
	}
	return restored
}

func cloneBattlefieldWindows(source map[string]int) map[string]int {
	if len(source) == 0 {
		return map[string]int{}
	}
	out := make(map[string]int, len(source))
	for id, remaining := range source {
		if id = strings.TrimSpace(id); id != "" && remaining > 0 {
			out[id] = remaining
		}
	}
	return out
}

func clampBattlefieldProgress(progress int, goal int) int {
	if goal <= 1 {
		return 0
	}
	if progress < 0 {
		return 0
	}
	if progress >= goal {
		return goal - 1
	}
	return progress
}

func battlefieldEffectActorMatches(definition battleWorldBattlefieldEffect, unit *battleUnitState) bool {
	if unit == nil {
		return false
	}
	filter := definition.Counter.ActorFilter
	if battleUnitIsSummon(unit) && !filter.IncludeSummons {
		return false
	}
	side := normalizeBattleSide(battleUnitPerspectiveSide(unit))
	sideMatched := false
	for _, allowed := range filter.Sides {
		if normalizeBattleSide(allowed) == side {
			sideMatched = true
			break
		}
	}
	if !sideMatched {
		return false
	}
	unitType := strings.ToLower(strings.TrimSpace(unit.UnitType))
	if battleUnitIsSummon(unit) {
		unitType = "summon"
	}
	for _, allowed := range filter.UnitTypes {
		if strings.ToLower(strings.TrimSpace(allowed)) == unitType {
			return true
		}
	}
	return false
}

func battlefieldEffectEligibleAliveCount(definition battleWorldBattlefieldEffect, players []battleUnitState, enemies []battleUnitState) int {
	goal := 0
	for index := range players {
		unit := &players[index]
		if unit.Alive && unit.HP > 0 && battlefieldEffectActorMatches(definition, unit) {
			goal++
		}
	}
	for index := range enemies {
		unit := &enemies[index]
		if unit.Alive && unit.HP > 0 && battlefieldEffectActorMatches(definition, unit) {
			goal++
		}
	}
	return goal
}

func battlefieldEffectTurnKindAllowed(definition battleWorldBattlefieldEffect, turnKind string, isPreparation bool) bool {
	if isPreparation && !definition.Counter.ActorFilter.IncludePreparation {
		return false
	}
	turnKind = strings.ToLower(strings.TrimSpace(turnKind))
	for _, allowed := range definition.Counter.ActorFilter.TurnKinds {
		if strings.ToLower(strings.TrimSpace(allowed)) == turnKind {
			return true
		}
	}
	return false
}

func battleActionWasSuppressed(payload map[string]any) bool {
	meta := normalizeAnyMap(payload["resultMeta"])
	reason := strings.TrimSpace(fmt.Sprintf("%v", meta["reason"]))
	return reason == battleEffectTurnActionSuppressed || reason == battleEffectSkillActionSuppressed || reason == battleEffectBasicAttackSuppressed || reason == battleEffectAllActionsSuppressed
}

func battlePublicBattlefieldEffectState(state battlefieldEffectState) map[string]any {
	return map[string]any{
		"id":        state.ID,
		"remaining": state.Remaining,
		"progress":  state.Progress,
		"goal":      state.Goal,
		"windows":   cloneBattlefieldWindows(state.Windows),
	}
}

func battlePublicBattlefieldEffectStates(states []battlefieldEffectState) []map[string]any {
	if len(states) == 0 {
		return nil
	}
	out := make([]map[string]any, 0, len(states))
	for _, state := range states {
		out = append(out, battlePublicBattlefieldEffectState(state))
	}
	return out
}

func (s *Server) battleApplyBattlefieldIncomingDamageModifiers(state *battleRuntimeState, target *battleUnitState, incoming float64) float64 {
	damage := maxFloat(0, incoming)
	if state == nil || target == nil || damage <= 0 || len(state.BattlefieldEffects) == 0 {
		return damage
	}
	if _, special := state.battlefieldSpecialEnemyIDs[strings.TrimSpace(target.EnemyID)]; !special {
		return damage
	}
	statesByID := make(map[string]battlefieldEffectState, len(state.BattlefieldEffects))
	for _, effectState := range state.BattlefieldEffects {
		statesByID[effectState.ID] = effectState
	}
	for _, definition := range state.battlefieldEffectDefinitions {
		effectState, ok := statesByID[definition.ID]
		if !ok {
			continue
		}
		for _, modifier := range definition.ConditionalModifiers {
			if modifier.Kind != "incoming_damage_reduction" || modifier.TargetScope != "special_enemies" {
				continue
			}
			if effectState.Windows[modifier.InactiveDuringWindow] > 0 {
				continue
			}
			damage *= maxFloat(0, 1-clampFloat(modifier.Value, 0, 1))
		}
	}
	return maxFloat(0, damage)
}

func (s *Server) battleAdvanceBattlefieldEffects(
	state *battleRuntimeState,
	actor *battleUnitState,
	isPreparation bool,
	actionPayload map[string]any,
	now time.Time,
) {
	if state == nil || actor == nil || len(state.BattlefieldEffects) == 0 {
		return
	}
	for index, definition := range state.battlefieldEffectDefinitions {
		if index >= len(state.BattlefieldEffects) || state.BattlefieldEffects[index].ID != definition.ID {
			continue
		}
		if !battlefieldEffectTurnKindAllowed(definition, "ordered", isPreparation) || !battlefieldEffectActorMatches(definition, actor) {
			continue
		}
		if !definition.Counter.CountSuppressedTurns && battleActionWasSuppressed(actionPayload) {
			continue
		}

		effectState := &state.BattlefieldEffects[index]
		goal := battlefieldEffectEligibleAliveCount(definition, state.Players, state.Enemies)
		if goal <= 0 {
			goal = 1
		}
		effectState.Goal = goal
		effectState.Progress = clampBattlefieldProgress(effectState.Progress, goal)
		playerRoundCompleted := false
		switch definition.Counter.Aggregation {
		case "per_event":
			effectState.Remaining--
		case "alive_roster_cycle":
			effectState.Progress++
			if effectState.Progress >= goal {
				effectState.Progress = 0
				effectState.Remaining--
				playerRoundCompleted = true
			}
		}
		if playerRoundCompleted {
			s.battleAdvanceBattlefieldWindows(state, definition, effectState, now)
		}
		state.emit("battle.field.tick", now, map[string]any{
			"effectId": effectState.ID,
			"state":    battlePublicBattlefieldEffectState(*effectState),
		})
		if effectState.Remaining <= 0 {
			s.battleTriggerBattlefieldEffect(state, definition, effectState, now)
		}
	}
}

func (s *Server) battleAdvanceBattlefieldWindows(state *battleRuntimeState, definition battleWorldBattlefieldEffect, effectState *battlefieldEffectState, now time.Time) {
	for _, window := range definition.PostTriggerWindows {
		remaining := effectState.Windows[window.ID]
		if remaining <= 0 || window.Unit != "player_round" {
			continue
		}
		remaining--
		status := "tick"
		if remaining <= 0 {
			delete(effectState.Windows, window.ID)
			status = "closed"
		} else {
			effectState.Windows[window.ID] = remaining
		}
		state.emit("battle.field.window", now, map[string]any{
			"effectId":  effectState.ID,
			"windowId":  window.ID,
			"remaining": remaining,
			"status":    status,
			"state":     battlePublicBattlefieldEffectState(*effectState),
		})
	}
}

func battlefieldWeightedTargetMatches(effect battleWorldBattlefieldTriggerEffect, unit *battleUnitState) bool {
	if unit == nil || !unit.Alive || unit.HP <= 0 {
		return false
	}
	isSummon := battleUnitIsSummon(unit)
	if isSummon && !effect.TargetFilter.IncludeSummons {
		return false
	}
	unitType := strings.ToLower(strings.TrimSpace(unit.UnitType))
	if isSummon {
		unitType = "summon"
	}
	for _, allowed := range effect.TargetFilter.UnitTypes {
		if strings.ToLower(strings.TrimSpace(allowed)) == unitType {
			return true
		}
	}
	return false
}

func battlefieldWeightedTargetCount(effect battleWorldBattlefieldTriggerEffect, eligible int) int {
	if eligible <= 0 {
		return 0
	}
	divisor := maxInt(1, effect.TargetCount.Divisor)
	ratio := float64(eligible) / float64(divisor)
	count := 0
	switch strings.ToLower(strings.TrimSpace(effect.TargetCount.Rounding)) {
	case "floor":
		count = int(math.Floor(ratio))
	case "round":
		count = int(math.Round(ratio))
	default:
		count = int(math.Ceil(ratio))
	}
	count = maxInt(maxInt(1, effect.TargetCount.Min), count)
	if effect.TargetCount.Max > 0 {
		count = minInt(count, effect.TargetCount.Max)
	}
	return minInt(count, eligible)
}

func battlefieldWeightedTargets(state *battleRuntimeState, effect battleWorldBattlefieldTriggerEffect) []*battleUnitState {
	if state == nil || effect.TargetScope != "alive_players" {
		return nil
	}
	eligible := make([]*battleUnitState, 0, len(state.Players))
	for index := range state.Players {
		if target := &state.Players[index]; battlefieldWeightedTargetMatches(effect, target) {
			eligible = append(eligible, target)
		}
	}
	count := battlefieldWeightedTargetCount(effect, len(eligible))
	for index := 0; index < count; index++ {
		remaining := len(eligible) - index
		selected := index + int(math.Floor(state.randFloat64()*float64(remaining)))
		if selected >= len(eligible) {
			selected = len(eligible) - 1
		}
		eligible[index], eligible[selected] = eligible[selected], eligible[index]
	}
	return eligible[:count]
}

func battlefieldWeightedChoice(state *battleRuntimeState, choices []battleWorldBattlefieldChoice) (battleWorldBattlefieldChoice, bool) {
	if state == nil || len(choices) == 0 {
		return battleWorldBattlefieldChoice{}, false
	}
	roll := state.randFloat64()
	cumulative := 0.0
	for _, choice := range choices {
		if choice.Weight <= 0 {
			continue
		}
		cumulative += choice.Weight
		if roll < cumulative {
			return choice, true
		}
	}
	return choices[len(choices)-1], true
}

func battleSkillCooldownState(unit *battleUnitState) map[string]int {
	if unit == nil || len(unit.Skills) == 0 {
		return map[string]int{}
	}
	out := make(map[string]int, len(unit.Skills))
	for _, skill := range unit.Skills {
		if skillID := strings.TrimSpace(skill.ID); skillID != "" {
			out[skillID] = maxInt(0, skill.CooldownRemain)
		}
	}
	return out
}

func battleEffectiveSkillCooldownTurns(unit *battleUnitState) map[string]int {
	if unit == nil || len(unit.Skills) == 0 {
		return map[string]int{}
	}
	out := make(map[string]int, len(unit.Skills))
	for _, skill := range unit.Skills {
		skillID := strings.TrimSpace(skill.ID)
		if skillID == "" {
			continue
		}
		scope := "skill"
		if strings.EqualFold(skillID, "basic_attack") {
			scope = "basic_attack"
		}
		effective := unit.applyBattleSkillScalarMutations(scope, skill)
		out[skillID] = maxInt(0, effective.CooldownTurns)
	}
	return out
}

func battleItemCooldownState(unit *battleUnitState) map[string]int {
	if unit == nil || len(unit.BattleItems) == 0 {
		return map[string]int{}
	}
	out := make(map[string]int, len(unit.BattleItems))
	for _, item := range unit.BattleItems {
		if itemID := strings.TrimSpace(item.ItemID); itemID != "" {
			out[itemID] = maxInt(0, item.CooldownRemain)
		}
	}
	return out
}

func (s *Server) battleApplyWeightedBattlefieldEffect(state *battleRuntimeState, definition battleWorldBattlefieldEffect, triggerEffect battleWorldBattlefieldTriggerEffect, now time.Time) []map[string]any {
	targets := battlefieldWeightedTargets(state, triggerEffect)
	results := make([]map[string]any, 0, len(targets))
	for _, target := range targets {
		choice, ok := battlefieldWeightedChoice(state, triggerEffect.Choices)
		if !ok || target == nil {
			continue
		}
		result := map[string]any{
			"unitId":     target.UnitID,
			"outcomeId":  choice.ID,
			"effectKind": choice.Effect.Kind,
		}
		switch choice.Effect.Kind {
		case "status":
			runtime := newBattleSkillActionRuntime(state, nil, now, s, nil)
			runtime.ApplyBuff(battleskills.BuffAction{
				SourceID:      "battlefield:" + definition.ID,
				TargetID:      target.UnitID,
				BuffID:        choice.Effect.StatusID,
				DurationTurns: int64(choice.Effect.DurationTurns),
				Reason:        "battlefield:" + definition.ID,
				Meta: map[string]any{
					"battlefieldEffectId": definition.ID,
					"outcomeId":           choice.ID,
				},
			})
			result["statusId"] = choice.Effect.StatusID
			result["durationTurns"] = choice.Effect.DurationTurns
			result["buffs"] = battlePublicBuffSlice(target.Buffs)
		case "adjust_skill_cooldown":
			action, changed := s.battleAdjustUnitSkillCooldowns(
				target,
				choice.Effect.Amount,
				choice.Effect.IncludeBasic,
				nil,
				choice.Effect.ExcludeSkillIDs,
				"battlefield:"+definition.ID,
				"battlefield:"+definition.ID,
				0,
				map[string]any{"battlefieldEffectId": definition.ID, "outcomeId": choice.ID},
			)
			result["amount"] = choice.Effect.Amount
			if changed {
				result["cooldownChanges"] = normalizeAnyMap(action["meta"])["affectedSkills"]
			} else {
				result["cooldownChanges"] = []map[string]any{}
			}
			result["skillCooldowns"] = battleSkillCooldownState(target)
		}
		results = append(results, result)
	}
	return results
}

func (s *Server) battleTriggerBattlefieldEffect(state *battleRuntimeState, definition battleWorldBattlefieldEffect, effectState *battlefieldEffectState, now time.Time) {
	if effectState.Windows == nil {
		effectState.Windows = map[string]int{}
	}
	resetTo := definition.Counter.ResetTo
	if resetTo <= 0 {
		resetTo = definition.Counter.Initial
	}
	effectState.Remaining = resetTo
	targetResults := make([]map[string]any, 0, len(state.Players))
	rawDamage := 0.0
	for _, triggerEffect := range definition.TriggerEffects {
		if triggerEffect.Kind == "weighted_random_per_target" {
			targetResults = append(targetResults, s.battleApplyWeightedBattlefieldEffect(state, definition, triggerEffect, now)...)
			continue
		}
		if triggerEffect.Kind != "damage" || triggerEffect.TargetScope != "all_alive_players" {
			continue
		}
		rawDamage = triggerEffect.AmountByDifficulty.Base * (1 + triggerEffect.AmountByDifficulty.PerLevelMult*float64(maxInt(1, state.DifficultyLevel)-1))
		for index := range state.Players {
			target := &state.Players[index]
			if !target.Alive || target.HP <= 0 || (battleUnitIsSummon(target) && !triggerEffect.IncludeSummons) {
				continue
			}
			beforeHP := target.HP
			damageType := normalizeBattleDamageType(triggerEffect.DamageType)
			profile := battleDamageChannelProfileForType(damageType)
			defenseK := s.battleDefenseKForState(state)
			damage, _ := battlecore.ComputeDamage(battlecore.DamageInput{
				Physical:   !profile.IsMagical,
				Attack:     rawDamage,
				Defense:    target.Stats[profile.DefenseStatKey],
				DefenseK:   defenseK,
				Coeff:      1,
				CritChance: 0,
				Rand:       1,
			})
			runtime := newBattleSkillActionRuntime(state, nil, now, s, nil)
			applied := runtime.ApplyDamage(battleskills.DamageAction{
				TargetID:   target.UnitID,
				Amount:     damage,
				DamageType: damageType,
				Reason:     "battlefield:" + definition.ID,
				Meta: map[string]any{
					"battlefieldEffectId": definition.ID,
					"difficultyLevel":     state.DifficultyLevel,
					"rawDamage":           roundTo4(rawDamage),
				},
			})
			targetResults = append(targetResults, map[string]any{
				"unitId":   target.UnitID,
				"damage":   roundTo4(applied.Amount),
				"beforeHp": roundTo4(beforeHP),
				"afterHp":  roundTo4(target.HP),
				"alive":    target.Alive,
			})
		}
	}
	goal := battlefieldEffectEligibleAliveCount(definition, state.Players, state.Enemies)
	if goal <= 0 {
		goal = 1
	}
	effectState.Goal = goal
	effectState.Progress = clampBattlefieldProgress(effectState.Progress, goal)
	deathHookActions := s.battleFlushPendingUnitDeathEquipmentHooks(state, now)
	state.emit("battle.field.trigger", now, map[string]any{
		"effectId":        effectState.ID,
		"difficultyLevel": state.DifficultyLevel,
		"rawDamage":       roundTo4(rawDamage),
		"targets":         targetResults,
		"actions":         deathHookActions,
		"state":           battlePublicBattlefieldEffectState(*effectState),
	})
	for _, window := range definition.PostTriggerWindows {
		if window.Duration <= 0 {
			continue
		}
		effectState.Windows[window.ID] = window.Duration
		state.emit("battle.field.window", now, map[string]any{
			"effectId":  effectState.ID,
			"windowId":  window.ID,
			"remaining": window.Duration,
			"status":    "opened",
			"state":     battlePublicBattlefieldEffectState(*effectState),
		})
	}
	battleCleanupSummonBindingForDeadUnits(state)
}

func (s *Server) processBattleTurn(
	ctx context.Context,
	state *battleRuntimeState,
	worldCfg battleWorldConfig,
	enemyCatalog map[string]battleEnemyConfig,
	now time.Time,
	rewardPlan map[int64]store.BattleWaveReward,
) (status string, advanced bool, err error) {
	defer battleRestoreCombatTrialImmutableTargets(state)
	isCombatTrial := battleCombatTrialActive(state)
	turnProfile := battleTurnProfileSample{}
	defer func() {
		state.commitTurnProfileSample(turnProfile, status, advanced, err)
	}()
	stageStartedAt := time.Now()
	if len(state.Enemies) == 0 {
		state.ExtraTurnQueue = nil
		s.spawnWaveEnemies(ctx, state, worldCfg, enemyCatalog)

		battleStartActions := s.battleTriggerBattleStartEquipmentHooks(state, now)
		waveStartActions := s.battleTriggerWaveStartEquipmentHooks(state, now)
		battleStartActions = append(battleStartActions, waveStartActions...)
		for i := range state.Players {
			s.battleRecomputeUnitStats(&state.Players[i])
		}
		for i := range state.Enemies {
			s.battleRecomputeUnitStats(&state.Enemies[i])
		}
		s.rebuildBattleOrder(state)
		state.emit("battle.wave.spawn", now, map[string]any{
			"wave":               state.Wave,
			"order":              append([]string(nil), state.Order...),
			"preparationOrder":   append([]string(nil), state.PreparationOrder...),
			"players":            battlePublicUnitSlice(state.Players),
			"enemies":            battlePublicUnitSlice(state.Enemies),
			"battlefieldEffects": battlePublicBattlefieldEffectStates(state.BattlefieldEffects),
			"battleStartActions": battleStartActions,
		})
	}
	if !battlePlayerMembersAlive(state) {
		state.ExtraTurnQueue = nil
		if !isCombatTrial {
			s.battleTriggerBattleEndEquipmentHooks(state, now)
			state.emit("battle.failed", now, map[string]any{"reason": "all_players_dead"})
		}
		turnProfile.PrecheckMs += battleDurationMs(stageStartedAt)
		status = store.BattleRunStatusFailed
		return
	}
	if !battleTeamAlive(state, "enemy") {
		state.ExtraTurnQueue = nil
		if isCombatTrial {
			turnProfile.PrecheckMs += battleDurationMs(stageStartedAt)
			status = store.BattleRunStatusEnded
			advanced = true
			return
		}
		s.battleTriggerBattleEndEquipmentHooks(state, now)
		settlement := s.resolveBattleWaveSettlement(*state, worldCfg, now)
		if settlement.Err != nil {
			return "", false, settlement.Err
		}
		if err := mergeBattleWaveRewards(rewardPlan, settlement.Rewards); err != nil {
			return "", false, err
		}
		state.CompletedBattles++
		state.emit("battle.wave.win", now, map[string]any{
			"wave":              state.Wave,
			"completedBattles":  state.CompletedBattles,
			"settlementSummary": settlement.ByUser,
		})
		state.emit("battle.ended", now, map[string]any{"reason": "wave_cleared"})
		turnProfile.PrecheckMs += battleDurationMs(stageStartedAt)
		status = store.BattleRunStatusEnded
		advanced = true
		return
	}

	var (
		actor             *battleUnitState
		side              string
		idx               int
		isPreparationTurn bool
	)
	for {
		actor, side, idx, isPreparationTurn = battleNextActor(state)
		if actor == nil {
			state.reseedOrderIfEmpty()
			turnProfile.PrecheckMs += battleDurationMs(stageStartedAt)
			return
		}
		if !s.battleUnitTurnsIneligible(state, actor) {
			break
		}
		actor.TurnIneligible = true
		actor.SkipTurnOrder = true
		state.removeUnitFromActionQueues(actor.UnitID)
	}
	turnProfile.PrecheckMs += battleDurationMs(stageStartedAt)

	isExtraTurn := idx == battleExtraTurnOrderIndex
	var extraTurnRequest battleQueuedExtraTurn
	if isExtraTurn {
		if state.activeExtraTurnRequest == nil {
			return
		}
		extraTurnRequest = *state.activeExtraTurnRequest
		extraActorID := strings.TrimSpace(extraTurnRequest.ActorID)
		state.activeExtraTurnChildCount = 0
		if state.activeExtraTurnUnitIDs == nil {
			state.activeExtraTurnUnitIDs = map[string]int{}
		}
		state.activeExtraTurnUnitIDs[extraActorID]++
		defer func() {
			state.activeExtraTurnRequest = nil
			state.activeExtraTurnChildCount = 0
			state.activeExtraTurnUnitIDs[extraActorID]--
			if state.activeExtraTurnUnitIDs[extraActorID] <= 0 {
				delete(state.activeExtraTurnUnitIDs, extraActorID)
			}
		}()
	}
	finishHookSuppression := battleSuppressDerivedHookDispatch(state, isExtraTurn && !extraTurnRequest.DispatchHooks)
	defer finishHookSuppression()

	actorID := strings.TrimSpace(actor.UnitID)
	actionPayload := s.executeUnitTurn(state, now, actor, side, isPreparationTurn)
	if isExtraTurn {
		battleDecorateQueuedExtraTurnPayload(actionPayload, extraTurnRequest)
	}
	s.finalizeTurnActionPayload(state, actionPayload)
	battleRecordCombatTrialMainAction(state, actor, isPreparationTurn, actionPayload)
	actor = battleUnitByID(state, actorID)
	if actor == nil {
		return
	}

	state.emit("battle.turn.action", now, actionPayload)
	mainActorSeq := state.Seq

	s.executeMirrorCastsForAction(state, now, actor, side, actionPayload, mainActorSeq)
	actor = battleUnitByID(state, actorID)
	if actor == nil {
		return
	}

	if actor.HasSyncedSummonTrigger {
		s.executeSyncedSummonTurns(state, now, actor, side, mainActorSeq)
	}

	if !isPreparationTurn {
		s.executeOtherUnitTurnEndEquipmentHooks(state, now, actor, mainActorSeq)
	}
	s.battleAdvanceBattlefieldEffects(state, actor, isPreparationTurn, actionPayload, now)
	battlePopulateTurnCooldownPayload(state, actionPayload)
	if !isPreparationTurn && !isExtraTurn {
		state.advanceTurnCursor(idx)
	}
	state.LastActionAt = now.UnixMilli()
	return
}

func (s *Server) finalizeTurnActionPayload(state *battleRuntimeState, payload map[string]any) {
	if state == nil || payload == nil {
		return
	}
	if casterDeathActions := battleApplySummonCasterDeathPolicy(state); len(casterDeathActions) > 0 {
		if actionLog, ok := payload["actionLog"].([]map[string]any); ok {
			payload["actionLog"] = append(actionLog, casterDeathActions...)
		}
	}
	battleCleanupSummonBindingForDeadUnits(state)
	payload["order"] = append([]string(nil), state.Order...)
	battlePopulateTurnCooldownPayload(state, payload)
}

func battlePopulateTurnCooldownPayload(state *battleRuntimeState, payload map[string]any) {
	if state == nil || payload == nil {
		return
	}
	actorID := strings.TrimSpace(fmt.Sprintf("%v", payload["actorId"]))
	if actor := battleUnitByID(state, actorID); actor != nil {
		payload["skillCooldowns"] = battleSkillCooldownState(actor)
		payload["skillCooldownTurns"] = battleEffectiveSkillCooldownTurns(actor)
		payload["itemCooldowns"] = battleItemCooldownState(actor)
	}
}

func (s *Server) executeUnitTurn(
	state *battleRuntimeState,
	now time.Time,
	actor *battleUnitState,
	side string,
	isPreparationTurn bool,
) map[string]any {
	state.nextBuffDurationTurnSerial++
	buffDurationTurnSerial := state.nextBuffDurationTurnSerial
	previousBuffDurationTurnSerial := state.activeBuffDurationTurnSerial
	state.activeBuffDurationTurnSerial = buffDurationTurnSerial
	defer func() {
		state.activeBuffDurationTurnSerial = previousBuffDurationTurnSerial
	}()
	state.beginBattleHookTurn()
	state.markUnitActed(actor.UnitID)
	turnKind := "ordered"
	if battleUnitHasActiveExtraTurn(state, actor) {
		turnKind = "extra"
	}
	supportStartHookActions := s.battleTriggerCombatTrialSupportTurnStartHooks(state, now, actor, isPreparationTurn)
	prevTurnHP := actor.HP
	prevTurnMP := actor.MP
	startHookContext := &battleEquipmentHookContext{TurnKind: turnKind, IsPreparation: isPreparationTurn}
	startHookActions := s.battleTriggerUnitBuffHooksWithContext(state, now, actor, nil, "on_turn_start", startHookContext)
	startEquipHookActions := s.battleTriggerUnitEquipmentHooksWithContext(
		state,
		now,
		actor,
		nil,
		nil,
		"on_turn_start",
		0,
		"",
		startHookContext,
	)
	preActionHookActions := append([]map[string]any{}, supportStartHookActions...)
	preActionHookActions = append(preActionHookActions, startHookActions...)
	preActionHookActions = append(preActionHookActions, startEquipHookActions...)
	applyTurnRegen(actor, s)
	battleCombatTrialRecordHeal(state, actor.UnitID, actor, actor, actor.HP-prevTurnHP)
	turnRegenActions := buildTurnRegenActionLog(actor, prevTurnHP, prevTurnMP)
	if len(startHookActions) > 0 {
		turnRegenActions = append(startHookActions, turnRegenActions...)
	}
	if len(turnRegenActions) > 0 {
		state.emit("battle.turn.regen", now, map[string]any{
			"wave":      state.Wave,
			"actorId":   actor.UnitID,
			"actorSide": actor.Side,
			"actions":   turnRegenActions,
			"actorHp":   roundTo4(actor.HP),
			"actorMp":   roundTo4(actor.MP),
			"alive":     actor.Alive,
		})
	}
	actionSuppressed, skillActionSuppressed, basicAttackSuppressed := s.battleUnitCheckTurnSuppressions(actor)
	if battleCombatTrialPassiveTarget(state, actor) {
		// Passive trial targets keep ordinary turns so their statuses settle, but never attack.
		actionSuppressed = true
	}
	s.tryExecuteBattleTurnItem(state, now, actor, side, actionSuppressed)
	skill, skillIdx := battleSkillState{}, -1
	if !actionSuppressed {
		skill, skillIdx = s.battleSelectSkillFiltered(state, *actor, side, !basicAttackSuppressed, !skillActionSuppressed)
	}
	var castHookContext *battleEquipmentHookContext
	if strings.TrimSpace(skill.ID) != "" && skill.Combo == nil {
		castKind := "skill"
		if strings.EqualFold(strings.TrimSpace(skill.ID), strings.TrimSpace(s.battleDefaultBasicSkillID())) {
			castKind = "basic_attack"
		}
		castHookContext = &battleEquipmentHookContext{
			SkillID:        skill.ID,
			SkillCategory:  skill.Category,
			CastKind:       castKind,
			IsBasicAttack:  castKind == "basic_attack",
			MPCost:         skill.MPCost,
			HPCost:         skill.HPCost,
			MPInsufficient: actor.MP < skill.MPCost,
		}
		beforeCastActions := s.battleTriggerUnitEquipmentHooksWithContext(
			state, now, actor, nil, nil, "on_before_skill_cast", 0, "", castHookContext,
		)
		preActionHookActions = append(preActionHookActions, beforeCastActions...)
		if castHookContext.MPInsufficient && !castHookContext.AllowDebtCast {
			skill = battleSkillState{}
			skillIdx = -1
			castHookContext = nil
		} else if castHookContext.CooldownDelta != 0 {
			skill.CooldownTurns = maxInt(0, skill.CooldownTurns+castHookContext.CooldownDelta)
		}
	}
	if strings.TrimSpace(skill.ID) != "" {
		beforeCostAlive := actor.Alive
		actor.HP = maxFloat(0, actor.HP-skill.HPCost)
		actor.MP = maxFloat(0, actor.MP-skill.MPCost)
		if actor.HP <= 0 {
			actor.Alive = false
		}
		if beforeCostAlive && !actor.Alive {
			battleQueueUnitDeath(state, actor, actor)
		}
	}
	actionPayload := map[string]any{
		"wave":             state.Wave,
		"actorId":          actor.UnitID,
		"actorSide":        actor.Side,
		"actorUserId":      actor.UserID,
		"actorEnemyId":     actor.EnemyID,
		"skillId":          skill.ID,
		"skillName":        skill.Name,
		"actorHp":          roundTo4(actor.HP),
		"actorMp":          roundTo4(actor.MP),
		"actorAlive":       actor.Alive,
		"targets":          []map[string]any{},
		"effects":          []map[string]any{},
		"actionLog":        []map[string]any{},
		"hooks":            []map[string]any{},
		"turnKind":         turnKind,
		"preparationTurn":  isPreparationTurn,
		"preparationOrder": append([]string(nil), state.PreparationOrder...),
	}
	execution := battleSkillExecution{}
	if strings.TrimSpace(skill.ID) != "" {
		execution = s.executeBattleSkill(state, now, actor, skill, "active", nil)
		if skill.Combo != nil {
			// A component can summon and reallocate the backing unit slice.
			if current := battleUnitByID(state, actor.UnitID); current != nil {
				actor = current
			}
		}
		if castHookContext != nil && execution.Meta["interrupted"] != true {
			afterCastActions := s.battleTriggerUnitEquipmentHooksWithContext(
				state, now, actor, nil, nil, "on_after_skill_cast", 0, "", castHookContext,
			)
			execution.Actions = append(execution.Actions, afterCastActions...)
		}
	} else if actionSuppressed {
		actionPayload["resultMeta"] = map[string]any{
			"idle":   true,
			"reason": battleEffectTurnActionSuppressed,
		}
	} else if skillActionSuppressed && basicAttackSuppressed {
		actionPayload["resultMeta"] = map[string]any{
			"idle":   true,
			"reason": battleEffectAllActionsSuppressed,
		}
	} else if skillActionSuppressed || basicAttackSuppressed {
		reason := battleEffectSkillActionSuppressed
		if basicAttackSuppressed {
			reason = battleEffectBasicAttackSuppressed
		}
		actionPayload["resultMeta"] = map[string]any{
			"idle":   true,
			"reason": reason,
		}
	} else {
		actionPayload["resultMeta"] = map[string]any{
			"idle":   true,
			"reason": "no_skill_available",
		}
	}
	actionPayload["targets"] = execution.Targets
	actionPayload["effects"] = execution.Effects
	actionPayload["actionLog"] = append(preActionHookActions, execution.Actions...)
	state.recordCompletedTurn(actor, isPreparationTurn)
	endHookContext := &battleEquipmentHookContext{
		TurnKind:         turnKind,
		IsPreparation:    isPreparationTurn,
		ActionSuppressed: battleActionWasSuppressed(actionPayload),
	}
	endHookActions := s.battleTriggerUnitBuffHooksWithContext(state, now, actor, nil, "on_turn_end", endHookContext)
	if len(endHookActions) > 0 {
		actionPayload["actionLog"] = append(actionPayload["actionLog"].([]map[string]any), endHookActions...)
	}
	endEquipHookActions := s.battleTriggerUnitEquipmentHooksWithContext(
		state,
		now,
		actor,
		nil,
		nil,
		"on_turn_end",
		0,
		"",
		endHookContext,
	)
	if len(endEquipHookActions) > 0 {
		actionPayload["actionLog"] = append(actionPayload["actionLog"].([]map[string]any), endEquipHookActions...)
	}
	settledHookActions := s.battleTriggerUnitBuffHooksWithContext(state, now, actor, nil, "on_turn_settled", endHookContext)
	if len(settledHookActions) > 0 {
		actionPayload["actionLog"] = append(actionPayload["actionLog"].([]map[string]any), settledHookActions...)
	}
	tickChanged, expiredBuffIDs := tickBattleBuffDurationsForTurn(actor, buffDurationTurnSerial)
	if tickChanged && len(expiredBuffIDs) == 0 {
		s.battleRecomputeUnitStats(actor)
	}
	if len(expiredBuffIDs) > 0 {
		expireDetachActions := buildBuffDetachActionLog(actor.UnitID, expiredBuffIDs, "duration_expired")
		actionPayload["actionLog"] = append(actionPayload["actionLog"].([]map[string]any), expireDetachActions...)
		expireHookActions := s.battleTriggerUnitBuffHooks(state, now, actor, "on_detach", expiredBuffIDs...)
		actionPayload["actionLog"] = append(actionPayload["actionLog"].([]map[string]any), expireHookActions...)
		beforeStats := cloneFloatMap(actor.Stats)
		beforeMaxHP := actor.MaxHP
		beforeMaxMP := actor.MaxMP
		beforePosition := actor.Position
		beforeSide := normalizeBattleSide(actor.Side)
		beforePerspective := battleUnitPerspectiveSide(actor)
		s.battleRecomputeUnitStats(actor)
		afterSide := normalizeBattleSide(actor.Side)
		afterPerspective := battleUnitPerspectiveSide(actor)
		if !battleFloatMapEqual(beforeStats, actor.Stats) || beforeMaxHP != actor.MaxHP || beforeMaxMP != actor.MaxMP || beforePosition != actor.Position || beforeSide != afterSide || beforePerspective != afterPerspective {
			actionPayload["actionLog"] = append(actionPayload["actionLog"].([]map[string]any), map[string]any{
				"kind":            "stat_recalc",
				"targetId":        actor.UnitID,
				"sourceId":        actor.UnitID,
				"reason":          "duration_expired",
				"stats":           cloneFloatMap(actor.Stats),
				"baseStats":       cloneFloatMap(actor.BaseStats),
				"deltaStats":      cloneFloatMap(actor.StatModifiers),
				"statModifiers":   cloneFloatMap(actor.StatModifiers),
				"statSources":     cloneNestedFloatMap(actor.StatSources),
				"buffs":           battlePublicBuffSlice(actor.Buffs),
				"maxHp":           roundTo4(actor.MaxHP),
				"maxMp":           roundTo4(actor.MaxMP),
				"fromPosition":    beforePosition,
				"position":        actor.Position,
				"basePosition":    actor.BasePosition,
				"side":            actor.Side,
				"baseSide":        actor.BaseSide,
				"perspectiveSide": actor.PerspectiveSide,
				"delayMs":         int64(0),
			})
		}
	}
	if _, expireActions := s.battleApplyBuffExpiry(actor, expiredBuffIDs); len(expireActions) > 0 {
		actionPayload["actionLog"] = append(actionPayload["actionLog"].([]map[string]any), expireActions...)
	}
	if summonExpireActions := s.battleTickSourceBoundSkipTurnSummons(state, now, actor, isPreparationTurn); len(summonExpireActions) > 0 {
		actionPayload["actionLog"] = append(actionPayload["actionLog"].([]map[string]any), summonExpireActions...)
	}
	if len(execution.Meta) > 0 {
		actionPayload["resultMeta"] = execution.Meta
	}
	statSkillID := skill.ID
	if skill.Combo != nil || execution.Meta["interrupted"] == true {
		statSkillID = ""
	}
	state.recordAchievementAction(actor.UnitID, statSkillID, actionPayload["actionLog"])
	if formatActions := s.battleDrainArenaFormatActions(state, now); len(formatActions) > 0 {
		actionPayload["actionLog"] = append(actionPayload["actionLog"].([]map[string]any), formatActions...)
	}
	populateTargetSlots(actionPayload["targets"].([]map[string]any), state, side)
	actionPayload["actorHp"] = roundTo4(actor.HP)
	actionPayload["actorMp"] = roundTo4(actor.MP)
	actionPayload["actorAlive"] = actor.Alive
	// 因为回合结束会立即减少所有技能1回合冷却，因此这里需要单独为这个技能加一回合冷却
	if skillIdx >= 0 && skillIdx < len(actor.Skills) && skill.CooldownTurns > 0 {
		actor.Skills[skillIdx].CooldownRemain = skill.CooldownTurns + 1
	}
	decrementBattleSkillCooldowns(actor)
	if actor.Alive && actor.HP > 0 && skillIdx >= 0 && skillIdx < len(actor.Skills) && strings.TrimSpace(skill.ID) != "" {
		postCooldownHookActions := s.battleTriggerUnitEquipmentHooksWithContext(
			state,
			now,
			actor,
			nil,
			nil,
			"on_after_skill_cooldown_tick",
			0,
			"",
			&battleEquipmentHookContext{
				SkillID:  skill.ID,
				CastKind: "active_turn",
			},
		)
		if len(postCooldownHookActions) > 0 {
			actionPayload["actionLog"] = append(actionPayload["actionLog"].([]map[string]any), postCooldownHookActions...)
		}
	}
	decrementBattleItemCooldowns(actor)
	if consequenceActions := s.battleDrainArenaFormatConsequences(state, now); len(consequenceActions) > 0 {
		actionPayload["actionLog"] = append(actionPayload["actionLog"].([]map[string]any), consequenceActions...)
	}
	if detachActions := s.battleFinalizeArenaFormatTurn(state); len(detachActions) > 0 {
		actionPayload["actionLog"] = append(actionPayload["actionLog"].([]map[string]any), detachActions...)
	}
	actionPayload["actorHp"] = roundTo4(actor.HP)
	actionPayload["actorMp"] = roundTo4(actor.MP)
	actionPayload["actorAlive"] = actor.Alive
	return actionPayload
}

func (state *battleRuntimeState) recordCompletedTurn(actor *battleUnitState, isPreparationTurn bool) {
	if state == nil || actor == nil || isPreparationTurn || strings.TrimSpace(actor.UnitID) == "" {
		return
	}
	if state.CompletedTurnCounts == nil {
		state.CompletedTurnCounts = map[string]int{}
	}
	if state.RuleTurnCounts == nil {
		state.RuleTurnCounts = map[string]int{}
	}
	state.CompletedTurnCounts[actor.UnitID]++
	state.RuleTurnCounts[actor.UnitID]++
}

func (state *battleRuntimeState) recordAchievementAction(actorID string, skillID string, actionLog any) {
	if state == nil {
		return
	}
	actorID = strings.TrimSpace(actorID)
	skillID = strings.TrimSpace(skillID)
	if skillID != "" {
		if state.AchievementStats.SkillCasts == nil {
			state.AchievementStats.SkillCasts = map[string]int64{}
		}
		state.AchievementStats.SkillCasts[skillID]++
		if actorID != "" {
			if state.AchievementStats.SkillCastsByUnit == nil {
				state.AchievementStats.SkillCastsByUnit = map[string]map[string]int64{}
			}
			if state.AchievementStats.SkillCastsByUnit[actorID] == nil {
				state.AchievementStats.SkillCastsByUnit[actorID] = map[string]int64{}
			}
			state.AchievementStats.SkillCastsByUnit[actorID][skillID]++
		}
		if skillID == "basic_attack" {
			state.AchievementStats.BasicAttacks++
		}
	}
	for _, row := range battleAnyMapSlice(actionLog) {
		switch strings.TrimSpace(fmt.Sprintf("%v", row["kind"])) {
		case "damage", "damage_redirect":
			state.AchievementStats.MaxSingleDamage = maxFloat(state.AchievementStats.MaxSingleDamage, toFloat64(row["amount"], 0))
			if parseBooleanLoose(row["beforeAlive"], false) && !parseBooleanLoose(row["afterAlive"], true) {
				state.recordKilledEnemyUnit(strings.TrimSpace(fmt.Sprintf("%v", row["targetId"])))
			}
		case "summon":
			state.AchievementStats.Summons++
		}
		if summon := normalizeAnyMap(row["summon"]); len(summon) > 0 {
			state.AchievementStats.Summons++
		}
	}
}

func (state *battleRuntimeState) recordKilledEnemyUnit(unitID string) {
	if state == nil {
		return
	}
	unitID = strings.TrimSpace(unitID)
	if unitID == "" || !strings.HasPrefix(unitID, "enemy:") {
		return
	}
	if state.AchievementStats.KilledEnemyUnits == nil {
		state.AchievementStats.KilledEnemyUnits = map[string]bool{}
	}
	if state.AchievementStats.KilledEnemyUnits[unitID] {
		return
	}
	enemyID := battleEnemyIDFromUnitID(unitID)
	if enemyID == "" {
		return
	}
	if state.AchievementStats.EnemyKills == nil {
		state.AchievementStats.EnemyKills = map[string]int64{}
	}
	state.AchievementStats.KilledEnemyUnits[unitID] = true
	state.AchievementStats.EnemyKills[enemyID]++
}

func (s *Server) executeSyncedSummonTurns(
	state *battleRuntimeState,
	now time.Time,
	summoner *battleUnitState,
	side string,
	parentSeq int64,
) {
	if s == nil || state == nil || summoner == nil {
		return
	}
	summonerID := strings.TrimSpace(summoner.UnitID)

	units := state.Players
	unitSide := "player"
	if side == "enemy" {
		units = state.Enemies
		unitSide = "enemy"
	}

	type syncedTurnCandidate struct {
		unitID string
		speed  float64
	}
	candidates := make([]syncedTurnCandidate, 0)
	for i := range units {
		unit := &units[i]
		if !unit.Alive || unit.HP <= 0 {
			continue
		}
		if !unit.SkipTurnOrder {
			continue
		}
		if s.battleUnitTurnsIneligible(state, unit) {
			continue
		}
		if strings.TrimSpace(unit.SummonSourceUnitID) != summonerID {
			continue
		}
		if !battleSyncedSummonTemplateAllowed(summoner, unit) {
			continue
		}
		candidates = append(candidates, syncedTurnCandidate{
			unitID: strings.TrimSpace(unit.UnitID),
			speed:  unit.Stats["speed"],
		})
	}
	if len(candidates) == 0 {
		return
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		si := candidates[i].speed
		sj := candidates[j].speed
		if si == sj {
			return candidates[i].unitID < candidates[j].unitID
		}
		return si > sj
	})

	for _, candidate := range candidates {
		unit := battleUnitByID(state, candidate.unitID)
		if unit == nil {
			continue
		}
		if !unit.Alive || unit.HP <= 0 {
			continue
		}
		if !unit.SkipTurnOrder || s.battleUnitTurnsIneligible(state, unit) || strings.TrimSpace(unit.SummonSourceUnitID) != summonerID {
			continue
		}
		if !battleSyncedSummonTemplateAllowed(summoner, unit) {
			continue
		}
		payload := s.executeUnitTurn(state, now, unit, unitSide, false)
		s.finalizeTurnActionPayload(state, payload)
		payload["parentSeq"] = parentSeq
		payload["batchGroup"] = fmt.Sprintf("sync:%d", parentSeq)
		state.emit("battle.turn.action", now, payload)
		unit = battleUnitByID(state, candidate.unitID)
		if unit != nil {
			s.executeMirrorCastsForAction(state, now, unit, unitSide, payload, state.Seq)
		}
	}
}

func battleSyncedSummonTemplateAllowed(summoner, summon *battleUnitState) bool {
	if summoner == nil || summon == nil || len(summoner.SyncedSummonTemplateIDs) == 0 {
		return true
	}
	templateID := strings.ToLower(strings.TrimSpace(summon.SummonTemplateUnitID))
	_, ok := summoner.SyncedSummonTemplateIDs[templateID]
	return ok
}

func (s *Server) executeMirrorCastsForAction(
	state *battleRuntimeState,
	now time.Time,
	source *battleUnitState,
	sourceSide string,
	sourcePayload map[string]any,
	parentSeq int64,
) {
	if s == nil || state == nil || source == nil || sourcePayload == nil || parentSeq <= 0 {
		return
	}
	if s.battleUnitHasWhileActiveEffect(source, battleEffectSuppressMirrorTrigger, nil) {
		return
	}
	skillID := strings.TrimSpace(fmt.Sprintf("%v", sourcePayload["skillId"]))
	if skillID == "" || store.IsEquipmentSkill(skillID) || normalizeAnyMap(sourcePayload["resultMeta"])["interrupted"] == true {
		return
	}
	defaultBasicID := strings.TrimSpace(s.battleDefaultBasicSkillID())
	sourceID := strings.TrimSpace(source.UnitID)
	if sourceID == "" {
		return
	}
	sourceWasMirrored := false
	if meta := normalizeAnyMap(sourcePayload["resultMeta"]); parseBooleanLoose(meta["mirrorCast"], false) {
		sourceWasMirrored = true
	}
	originalTargets := battleMirrorOriginalTargetIDs(sourcePayload)
	sourceSkill := battleMirrorSourceSkill(source, skillID)
	for _, mirror := range battleMirrorCastCandidateUnits(state) {
		if mirror == nil || !mirror.Alive || mirror.HP <= 0 {
			continue
		}
		if strings.TrimSpace(mirror.UnitID) == sourceID {
			continue
		}
		hooks := battleMirrorCastEquipmentHooks(mirror)
		if len(hooks) == 0 {
			continue
		}
		for _, hook := range hooks {
			conditionsPassed := true
			for _, condition := range hook.Conditions {
				if !s.battleEvaluateEquipmentHookCondition(state, condition, mirror, source, nil, 0, "", hook.SourceEnhanceLevel, nil) {
					conditionsPassed = false
					break
				}
			}
			if !conditionsPassed {
				continue
			}
			for _, effect := range hook.Effects {
				kind := battleNormalizeTrigger(fmt.Sprintf("%v", effect["kind"]))
				if kind == "" {
					kind = battleNormalizeTrigger(fmt.Sprintf("%v", effect["type"]))
				}
				if kind != battleNormalizeTrigger("mirror_cast_skill") {
					continue
				}
				includeBasic := parseBooleanLoose(firstNonNil(effect["includeBasicAttack"], effect["include_basic_attack"]), false)
				if !includeBasic && defaultBasicID != "" && strings.EqualFold(skillID, defaultBasicID) {
					continue
				}
				allowMirrored := parseBooleanLoose(firstNonNil(effect["allowMirroredCastTrigger"], effect["allow_mirrored_cast_trigger"]), false)
				if sourceWasMirrored && !allowMirrored {
					continue
				}
				if !battleMirrorTriggerScopeMatches(s, state, mirror, source, effect["triggerScope"]) {
					continue
				}
				if !battleMirrorSkillFilterMatches(sourceSkill, effect) {
					continue
				}
				payload, ok := s.executeMirrorCastSkill(state, now, mirror, sourceSide, skillID, effect, originalTargets, parentSeq, hook)
				if !ok {
					continue
				}
				state.emit("battle.turn.action", now, payload)
			}
		}
	}
}

func battleMirrorCastCandidateUnits(state *battleRuntimeState) []*battleUnitState {
	if state == nil {
		return nil
	}
	out := make([]*battleUnitState, 0, len(state.Players)+len(state.Enemies))
	for idx := range state.Players {
		out = append(out, &state.Players[idx])
	}
	for idx := range state.Enemies {
		out = append(out, &state.Enemies[idx])
	}
	return out
}

func battleMirrorCastEquipmentHooks(unit *battleUnitState) []battleEquipmentHookRule {
	if unit == nil || unit.equipmentHookCount() == 0 {
		return nil
	}
	triggerKey := battleNormalizeTrigger("on_other_unit_cast_skill")
	out := make([]battleEquipmentHookRule, 0, 1)
	for hookIndex := 0; hookIndex < unit.equipmentHookCount(); hookIndex++ {
		hook, ok := unit.equipmentHookAt(hookIndex)
		if !ok {
			continue
		}
		if battleNormalizeTrigger(hook.Trigger) == triggerKey {
			out = append(out, hook)
		}
	}
	if len(out) <= 1 {
		return out
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Priority != out[j].Priority {
			return out[i].Priority > out[j].Priority
		}
		return out[i].ID < out[j].ID
	})
	return out
}

func battleMirrorOriginalTargetIDs(payload map[string]any) []string {
	seen := map[string]struct{}{}
	out := []string{}
	appendID := func(raw any) {
		id := strings.TrimSpace(fmt.Sprintf("%v", raw))
		if id == "" || id == "<nil>" {
			return
		}
		if _, ok := seen[id]; ok {
			return
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	for _, row := range battleAnyMapSlice(payload["targets"]) {
		appendID(row["targetId"])
	}
	for _, row := range battleAnyMapSlice(payload["effects"]) {
		appendID(row["targetId"])
	}
	for _, row := range battleAnyMapSlice(payload["actionLog"]) {
		appendID(row["targetId"])
	}
	return out
}

func battleMirrorSourceSkill(source *battleUnitState, skillID string) battleSkillState {
	if source == nil {
		return battleSkillState{ID: skillID}
	}
	skill, _, ok := battleFindUnitSkillByID(source, skillID)
	if ok {
		return skill
	}
	return battleSkillState{ID: skillID}
}

func battleMirrorSkillFilterMatches(skill battleSkillState, effect map[string]any) bool {
	if effect == nil {
		return true
	}
	skillIDs := compactStringSlice(firstNonNil(effect["skillIds"], effect["skill_ids"]))
	if len(skillIDs) > 0 && !battleStringSliceContainsFold(skillIDs, skill.ID) {
		return false
	}
	categories := compactStringSlice(firstNonNil(
		effect["skillCategories"],
		effect["skill_categories"],
		effect["skillTypes"],
		effect["skill_types"],
		effect["categories"],
	))
	if len(categories) > 0 && !battleStringSliceContainsFold(categories, skill.Category) {
		return false
	}
	return true
}

func battleStringSliceContainsFold(values []string, target string) bool {
	target = strings.TrimSpace(target)
	if target == "" {
		return false
	}
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), target) {
			return true
		}
	}
	return false
}

func battleMirrorTriggerScopeMatches(s *Server, state *battleRuntimeState, mirror *battleUnitState, source *battleUnitState, rawScope any) bool {
	scope := strings.ToLower(strings.TrimSpace(fmt.Sprintf("%v", rawScope)))
	scope = strings.ReplaceAll(scope, "-", "_")
	if scope == "" || scope == "<nil>" {
		scope = "all"
	}
	switch scope {
	case "all", "any", "all_other":
		return true
	case "ally", "allies":
		return s.battleTargetRelationForActor(state, mirror, source) == "ally"
	case "enemy", "enemies":
		return s.battleTargetRelationForActor(state, mirror, source) == "enemy"
	default:
		return true
	}
}

func (s *Server) executeMirrorCastSkill(
	state *battleRuntimeState,
	now time.Time,
	actor *battleUnitState,
	sourceSide string,
	skillID string,
	effect map[string]any,
	originalTargets []string,
	parentSeq int64,
	hook battleEquipmentHookRule,
) (map[string]any, bool) {
	if store.IsEquipmentSkill(skillID) {
		return nil, false
	}
	if state == nil || actor == nil || !actor.Alive || actor.HP <= 0 {
		return nil, false
	}
	actionSuppressed, skillSuppressed, _ := s.battleUnitCheckTurnSuppressions(actor)
	if actionSuppressed || skillSuppressed {
		return nil, false
	}
	skill, skillIdx, ok := battleFindUnitSkillByID(actor, skillID)
	if !ok || skillIdx < 0 {
		return nil, false
	}
	if !s.battleSkillRulesetEnabled(skill) {
		return nil, false
	}
	mutationScope := "skill"
	if strings.EqualFold(strings.TrimSpace(skill.ID), strings.TrimSpace(s.battleDefaultBasicSkillID())) {
		mutationScope = "basic_attack"
	}
	skill = actor.applyBattleSkillScalarMutations(mutationScope, skill)
	derivedCastKind := "mirror_cast"
	resultKind := "mirror"
	if battleNormalizeTrigger(fmt.Sprintf("%v", firstNonNil(effect["kind"], effect["type"]))) == "castskill" {
		derivedCastKind = "specialization_cast"
		resultKind = "cast_skill"
	}
	amountRatio := 1.0
	if battleSpecializationHasValueFormula(effect) {
		amountRatio = battleSpecializationEffectScalar(actor, effect)
	} else {
		amountRatio = toFloat64(firstNonNil(effect["amountRatioBase"], effect["amount_ratio_base"], effect["amountRatio"], effect["amount_ratio"]), 1)
		amountRatio += float64(maxInt(0, hook.SourceEnhanceLevel)) * toFloat64(firstNonNil(effect["amountRatioPerEnhanceLevel"], effect["amount_ratio_per_enhance_level"]), 0)
	}
	if amountRatio < 0 {
		amountRatio = 0
	}
	if amountRatio != 1 {
		skill.Effects = cloneBattleSkillEffects(skill.Effects)
		for index := range skill.Effects {
			kind := battleNormalizeTrigger(skill.Effects[index].Kind)
			if kind != "damage" && kind != "heal" {
				continue
			}
			skill.Effects[index].AmountFormula = scaleBattleSkillAmountFormula(skill.Effects[index].AmountFormula, amountRatio)
			skill.Effects[index].DamageSpec.AmountFormula = scaleBattleSkillAmountFormula(skill.Effects[index].DamageSpec.AmountFormula, amountRatio)
		}
	}
	if skill.CooldownRemain > 0 {
		return nil, false
	}
	defaultMPCostMultiplier := 1.0
	if hook.SourceKind == "specialization" {
		// A specialization mirror is the original cast's echo, not a second
		// resource purchase. Explicit DSL can still opt back into a cost.
		defaultMPCostMultiplier = 0
	}
	mpCostMultiplier := toFloat64(firstNonNil(effect["mpCostMultiplier"], effect["mp_cost_multiplier"]), defaultMPCostMultiplier)
	if mpCostMultiplier < 0 {
		mpCostMultiplier = 0
	}
	hpCostMultiplier := toFloat64(firstNonNil(effect["hpCostMultiplier"], effect["hp_cost_multiplier"]), 1)
	if hpCostMultiplier < 0 {
		hpCostMultiplier = 0
	}
	skill.MPCost = math.Floor(skill.MPCost * mpCostMultiplier)
	skill.HPCost = math.Floor(skill.HPCost * hpCostMultiplier)
	activationHPCost, activationMPCost := battleEquipmentHookActivationAmounts(actor, hook)
	if !battleEquipmentHookActivationAllowed(state, actor, hook) || actor.MP < skill.MPCost+activationMPCost || actor.HP <= skill.HPCost+activationHPCost {
		return nil, false
	}
	actorSide := strings.TrimSpace(actor.Side)
	if actorSide == "" {
		actorSide = sourceSide
	}
	if !s.battleSkillRulePassed(state, *actor, actorSide, skill) {
		return nil, false
	}
	if !s.battleSkillCastGuardsPassed(state, actor, skill) {
		return nil, false
	}
	defaultDispatchHooks := hook.SourceKind != "specialization" || hook.DispatchHooks
	dispatchHooks := parseBooleanLoose(firstNonNil(effect["dispatchHooks"], effect["dispatch_hooks"], effect["dispatchEquipmentHooks"], effect["dispatch_equipment_hooks"]), defaultDispatchHooks)
	castHookContext := &battleEquipmentHookContext{
		SkillID:       skill.ID,
		SkillCategory: skill.Category,
		CastKind:      derivedCastKind,
		IsBasicAttack: mutationScope == "basic_attack",
		MPCost:        skill.MPCost,
		HPCost:        skill.HPCost,
	}
	derivedHookActions := make([]map[string]any, 0, 2)
	if hook.SourceKind != "specialization" {
		activationActions, activated := battleActivateEquipmentHook(state, actor, hook, "on_other_unit_cast_skill")
		if !activated {
			return nil, false
		}
		derivedHookActions = append(derivedHookActions, activationActions...)
	}
	if dispatchHooks {
		derivedHookActions = append(derivedHookActions, s.battleTriggerUnitEquipmentHooksWithContext(
			state, now, actor, nil, nil, "on_before_skill_cast", 0, "", castHookContext,
		)...)
		if castHookContext.CooldownDelta != 0 {
			skill.CooldownTurns = maxInt(0, skill.CooldownTurns+castHookContext.CooldownDelta)
		}
	}
	forcedTargets := []string(nil)
	switch normalizeBattleMirrorTargetingMode(effect["targetingMode"]) {
	case "reuse_original_targets", "reuse_then_reselect":
		forcedTargets = append([]string(nil), originalTargets...)
	}
	beforeCostAlive := actor.Alive
	actor.HP = maxFloat(0, actor.HP-skill.HPCost)
	actor.MP = maxFloat(0, actor.MP-skill.MPCost)
	if actor.HP <= 0 {
		actor.Alive = false
	}
	if beforeCostAlive && !actor.Alive {
		battleQueueUnitDeath(state, actor, actor)
	}
	actionPayload := map[string]any{
		"wave":             state.Wave,
		"actorId":          actor.UnitID,
		"actorSide":        actor.Side,
		"actorUserId":      actor.UserID,
		"actorEnemyId":     actor.EnemyID,
		"skillId":          skill.ID,
		"skillName":        skill.Name,
		"actorHp":          roundTo4(actor.HP),
		"actorMp":          roundTo4(actor.MP),
		"actorAlive":       actor.Alive,
		"targets":          []map[string]any{},
		"effects":          []map[string]any{},
		"actionLog":        []map[string]any{},
		"hooks":            []map[string]any{},
		"preparationTurn":  false,
		"preparationOrder": append([]string(nil), state.PreparationOrder...),
		"parentSeq":        parentSeq,
		"batchGroup":       fmt.Sprintf("%s:%d", resultKind, parentSeq),
	}
	finishHookSuppression := battleSuppressDerivedHookDispatch(state, !dispatchHooks)
	execution := s.executeBattleSkill(state, now, actor, skill, derivedCastKind, forcedTargets)
	finishHookSuppression()
	afterHookActions := []map[string]any(nil)
	if dispatchHooks && execution.Meta["interrupted"] != true {
		afterHookActions = s.battleTriggerUnitEquipmentHooksWithContext(
			state, now, actor, nil, nil, "on_after_skill_cast", 0, "", castHookContext,
		)
	}
	actionPayload["targets"] = execution.Targets
	actionPayload["effects"] = execution.Effects
	derivedHookActions = append(derivedHookActions, execution.Actions...)
	derivedHookActions = append(derivedHookActions, afterHookActions...)
	if hook.SourceKind == "specialization" {
		for actionIndex := range derivedHookActions {
			enrichBattleSpecializationAction(derivedHookActions[actionIndex], hook)
		}
	}
	actionPayload["actionLog"] = derivedHookActions
	resultMeta := cloneAnyMap(execution.Meta)
	resultMeta["mirrorCast"] = resultKind == "mirror"
	resultMeta["specializationCast"] = resultKind == "cast_skill"
	if resultKind == "mirror" {
		resultMeta["mirrorTrigger"] = "on_other_unit_cast_skill"
	}
	resultMeta["mirrorHookId"] = hook.ID
	resultMeta["mirrorSourceItemId"] = hook.SourceItemID
	resultMeta["mpCostMultiplier"] = roundTo4(mpCostMultiplier)
	resultMeta["hpCostMultiplier"] = roundTo4(hpCostMultiplier)
	resultMeta["amountRatio"] = roundTo4(amountRatio)
	resultMeta["dispatchHooks"] = dispatchHooks
	actionPayload["resultMeta"] = resultMeta
	populateTargetSlots(actionPayload["targets"].([]map[string]any), state, actorSide)
	actionPayload["actorHp"] = roundTo4(actor.HP)
	actionPayload["actorMp"] = roundTo4(actor.MP)
	actionPayload["actorAlive"] = actor.Alive
	applyCooldown := parseBooleanLoose(firstNonNil(effect["applyCooldown"], effect["apply_cooldown"], effect["setCooldown"], effect["set_cooldown"]), true)
	resultMeta["applyCooldown"] = applyCooldown
	if applyCooldown && skillIdx >= 0 && skillIdx < len(actor.Skills) && skill.CooldownTurns > 0 {
		actor.Skills[skillIdx].CooldownRemain = skill.CooldownTurns
	}
	if deathHookActions := s.battleFlushPendingUnitDeathEquipmentHooks(state, now); len(deathHookActions) > 0 {
		if hook.SourceKind == "specialization" {
			for actionIndex := range deathHookActions {
				enrichBattleSpecializationAction(deathHookActions[actionIndex], hook)
			}
		}
		actionPayload["actionLog"] = append(actionPayload["actionLog"].([]map[string]any), deathHookActions...)
	}
	return actionPayload, true
}

func battleFindUnitSkillByID(unit *battleUnitState, skillID string) (battleSkillState, int, bool) {
	if unit == nil {
		return battleSkillState{}, -1, false
	}
	skillID = strings.TrimSpace(skillID)
	if skillID == "" {
		return battleSkillState{}, -1, false
	}
	for idx, skill := range unit.Skills {
		if strings.EqualFold(strings.TrimSpace(skill.ID), skillID) {
			return skill, idx, true
		}
	}
	return battleSkillState{}, -1, false
}

func normalizeBattleMirrorTargetingMode(raw any) string {
	mode := strings.ToLower(strings.TrimSpace(fmt.Sprintf("%v", raw)))
	mode = strings.ReplaceAll(mode, "-", "_")
	if mode == "" || mode == "<nil>" {
		return "reselect"
	}
	switch mode {
	case "reuse_original_targets", "reuse_targets", "reuse":
		return "reuse_original_targets"
	case "reuse_then_reselect", "reuse_original_then_reselect":
		return "reuse_then_reselect"
	case "reselect":
		return "reselect"
	default:
		return "reselect"
	}
}

const battlePlayerSnapshotMaxAttempts = 3

var errBattlePlayerSnapshotUnstable = errors.New("battle player snapshot changed during assembly")

func (s *Server) buildBattlePlayerSnapshot(ctx context.Context, member store.BattleRoomMember, now time.Time) (battleUnitState, error) {
	return buildBattlePlayerSnapshotWithFence(
		func() (store.SpecializationBattleContribution, error) {
			return s.playerSpecializationBattleContribution(ctx, member.UserID)
		},
		func() (battleUnitState, error) {
			return s.buildBattlePlayerSnapshotAttempt(ctx, member, now, true)
		},
	)
}

func (s *Server) buildBattlePlayerSnapshotWithoutSpecialization(ctx context.Context, member store.BattleRoomMember, now time.Time) (battleUnitState, error) {
	return s.buildBattlePlayerSnapshotAttempt(ctx, member, now, false)
}

func buildBattlePlayerSnapshotWithFence(
	readContribution func() (store.SpecializationBattleContribution, error),
	buildAttempt func() (battleUnitState, error),
) (battleUnitState, error) {
	for attempt := 1; attempt <= battlePlayerSnapshotMaxAttempts; attempt++ {
		before, err := readContribution()
		if err != nil {
			return battleUnitState{}, fmt.Errorf("read specialization snapshot fence: %w", err)
		}
		unit, err := buildAttempt()
		if err != nil {
			return battleUnitState{}, err
		}
		after, err := readContribution()
		if err != nil {
			return battleUnitState{}, fmt.Errorf("verify specialization snapshot fence: %w", err)
		}
		if battlePlayerSnapshotSpecializationStable(unit, before, after) {
			return unit, nil
		}
	}
	return battleUnitState{}, errBattlePlayerSnapshotUnstable
}

func battlePlayerSnapshotSpecializationStable(unit battleUnitState, before, after store.SpecializationBattleContribution) bool {
	return before.TreeID == after.TreeID &&
		before.RulesetVersion == after.RulesetVersion &&
		before.BalanceRevision == after.BalanceRevision &&
		before.Revision == after.Revision &&
		before.Digest == after.Digest &&
		unit.SpecializationTreeID == before.TreeID &&
		unit.SpecializationRulesetVersion == before.RulesetVersion &&
		unit.SpecializationBalanceRevision == before.BalanceRevision &&
		unit.SpecializationRevision == before.Revision &&
		unit.SpecializationDigest == before.Digest
}

func (s *Server) buildBattlePlayerSnapshotAttempt(ctx context.Context, member store.BattleRoomMember, now time.Time, specializationEnabled bool) (battleUnitState, error) {
	seeds, err := s.playerAttributeSeeds()
	if err != nil {
		return battleUnitState{}, err
	}
	if err := s.deps.Store.EnsurePlayerAttributes(ctx, member.UserID, seeds); err != nil {
		return battleUnitState{}, err
	}
	if err := s.deps.Store.RecomputeDerivedStats(ctx, member.UserID); err != nil {
		return battleUnitState{}, err
	}
	attrStates, err := s.deps.Store.ListPlayerAttributes(ctx, member.UserID, seeds)
	if err != nil {
		return battleUnitState{}, err
	}
	derivedStats := map[string]float64{}
	derived, err := s.deps.Store.GetPlayerStats(ctx, member.UserID)
	if err == nil {
		for key, value := range derived.Derived {
			normalizedKey := strings.TrimSpace(key)
			if normalizedKey == "" {
				continue
			}
			derivedStats[normalizedKey] = value
		}
	}
	projection := projectBattlePlayerStats(attrStates, derivedStats)
	stats := projection.Stats
	specializationContribution := store.SpecializationBattleContribution{}
	if specializationEnabled {
		specializationContribution, err = s.playerSpecializationBattleContribution(ctx, member.UserID)
		if err != nil {
			return battleUnitState{}, fmt.Errorf("load specialization battle contribution: %w", err)
		}
		if err := applyBattleSpecializationStatModifiers(stats, specializationContribution.StatModifiers); err != nil {
			return battleUnitState{}, fmt.Errorf("apply specialization battle stats: %w", err)
		}
	}
	playerSkills, err := s.deps.Store.ListPlayerSkills(ctx, member.UserID)
	if err != nil {
		return battleUnitState{}, err
	}
	customSkillDefinitions, err := s.deps.Store.ListAcceptedCustomSkillDefinitions(ctx, member.UserID)
	if err != nil {
		return battleUnitState{}, fmt.Errorf("load custom skill definitions: %w", err)
	}
	battleBag, err := s.deps.Store.ListBattleItemBag(ctx, member.UserID)
	if err != nil {
		return battleUnitState{}, err
	}
	slotRule := s.battleSkillSlotRule()
	maxSkillSlots := battleAvailableSkillSlots(stats["intelligence"], slotRule)
	defaultBasicSkillID := s.battleDefaultBasicSkillID()
	enabledSkillSlots := make([]string, 0, len(playerSkills))
	seenEnabled := make(map[string]struct{}, len(playerSkills))
	filteredRules := map[string]battleSkillRuleState{}
	skillProgress := map[string]store.PlayerSkill{}
	for _, row := range playerSkills {
		skillID := strings.TrimSpace(row.SkillID)
		if skillID == "" || store.IsEquipmentSkill(skillID) {
			continue
		}
		if _, exists := s.battleSkillRawByIDWithCustom(skillID, customSkillDefinitions); !exists {
			continue
		}
		skillProgress[strings.ToLower(skillID)] = row
		buf, marshalErr := json.Marshal(row.RuleGroups)
		if marshalErr == nil {
			groups := []rules.ConditionGroup{}
			if unmarshalErr := json.Unmarshal(buf, &groups); unmarshalErr == nil && len(groups) > 0 {
				filteredRules[strings.ToLower(skillID)] = battleSkillRuleState{
					Enabled: true,
					Groups:  groups,
				}
			}
		}
		if !row.Enabled {
			continue
		}
		key := strings.ToLower(skillID)
		if _, exists := seenEnabled[key]; exists {
			continue
		}
		seenEnabled[key] = struct{}{}
		enabledSkillSlots = append(enabledSkillSlots, skillID)
	}
	if maxSkillSlots > 0 && len(enabledSkillSlots) > maxSkillSlots {
		enabledSkillSlots = enabledSkillSlots[:maxSkillSlots]
	}
	if defaultBasicSkillID != "" && s.battleSkillExists(defaultBasicSkillID) && !containsSkillID(enabledSkillSlots, defaultBasicSkillID) {
		if maxSkillSlots > 0 && len(enabledSkillSlots) >= maxSkillSlots {
			keep := maxSkillSlots - 1
			if keep < 0 {
				keep = 0
			}
			if keep < len(enabledSkillSlots) {
				enabledSkillSlots = enabledSkillSlots[:keep]
			}
		}
		enabledSkillSlots = append(enabledSkillSlots, defaultBasicSkillID)
	}
	equipped, err := s.deps.Store.ListEquippedItems(ctx, member.UserID)
	if err != nil {
		return battleUnitState{}, fmt.Errorf("load dynamically equipped items: %w", err)
	}
	weaponClasses := s.weaponClassSetFromEquippedItems(equipped)
	eligibleSkillSlots := make([]string, 0, len(enabledSkillSlots))
	enableCtx := battleSkillEnableContext{
		Attributes:    stats,
		WeaponClasses: weaponClasses,
	}
	for _, skillID := range enabledSkillSlots {
		if strings.EqualFold(skillID, defaultBasicSkillID) {
			eligibleSkillSlots = append(eligibleSkillSlots, skillID)
			continue
		}
		raw, ok := s.battleSkillRawByIDWithCustom(skillID, customSkillDefinitions)
		if !ok {
			continue
		}
		if evaluateBattleSkillRequirement(raw, enableCtx).Eligible {
			eligibleSkillSlots = append(eligibleSkillSlots, skillID)
		}
	}
	enabledSkillSlots = eligibleSkillSlots
	skills := make([]battleSkillState, 0, len(enabledSkillSlots))
	for _, skillID := range enabledSkillSlots {
		progress := skillProgress[strings.ToLower(skillID)]
		raw, _ := s.battleSkillRawByIDWithCustom(skillID, customSkillDefinitions)
		skills = append(skills, s.battleSkillByRawWithProgress(skillID, progress.Level, progress.Exp, raw))
	}
	itemRules := s.loadBattleItemRules(ctx, member.UserID)
	battleItems := s.buildBattleItemStatesFromBag(battleBag)
	eqDigest := make([]string, 0, len(equipped))
	eqIDs := make([]int64, 0, len(equipped))
	var basicAttackProfile *battleBasicAttackProfile
	equipmentHooks := s.resolveBattleEquipmentHooks(equipped)
	specializationHooks := []battleEquipmentHookRule{}
	if specializationEnabled {
		specializationHooks, err = s.resolveBattleSpecializationHooks(specializationContribution)
		if err != nil {
			return battleUnitState{}, fmt.Errorf("resolve specialization hooks: %w", err)
		}
	}
	sort.SliceStable(specializationHooks, func(i, j int) bool {
		if specializationHooks[i].Priority != specializationHooks[j].Priority {
			return specializationHooks[i].Priority > specializationHooks[j].Priority
		}
		return specializationHooks[i].ID < specializationHooks[j].ID
	})
	effectMutations := s.resolveBattleAffixEffectMutations(equipped)
	specializationEffectMutations := []battleAffixEffectMutation{}
	if specializationEnabled {
		specializationEffectMutations, err = s.resolveBattleSpecializationEffectMutations(specializationContribution)
		if err != nil {
			return battleUnitState{}, fmt.Errorf("resolve specialization effect mutations: %w", err)
		}
	}
	sort.SliceStable(specializationEffectMutations, func(i, j int) bool {
		if specializationEffectMutations[i].Priority != specializationEffectMutations[j].Priority {
			return specializationEffectMutations[i].Priority > specializationEffectMutations[j].Priority
		}
		return specializationEffectMutations[i].ID < specializationEffectMutations[j].ID
	})
	for _, eq := range equipped {
		if strings.TrimSpace(eq.EquipType) != "battle" {
			continue
		}
		eqDigest = append(eqDigest, fmt.Sprintf("%s+%d", eq.TemplateItemID, eq.EnhanceLevel))
		eqIDs = append(eqIDs, eq.InstanceID)
	}
	basicAttackProfile = s.resolveBattleBasicAttackProfile(equipped)
	playerName := fmt.Sprintf("P%d", member.UserID)
	if username, nameErr := s.deps.Store.GetUsernameByUserID(ctx, member.UserID); nameErr == nil && strings.TrimSpace(username) != "" {
		playerName = username
	}
	unit := battleUnitState{
		UnitID:                        fmt.Sprintf("player:%d", member.UserID),
		IsPlayerUnit:                  true,
		Side:                          "player",
		OriginSide:                    "player",
		BaseSide:                      "player",
		PerspectiveSide:               "player",
		UserID:                        member.UserID,
		UnitType:                      "player",
		BattleLevel:                   battleLevelFromAttributeStates(attrStates),
		Name:                          playerName,
		Position:                      member.Position,
		BasePosition:                  member.Position,
		Alive:                         true,
		RewardEligible:                true,
		HP:                            battleMaxHPFromStat(stats["hp"]),
		MP:                            maxFloat(0, stats["mp"]),
		MaxHP:                         battleMaxHPFromStat(stats["hp"]),
		MaxMP:                         maxFloat(0, stats["mp"]),
		Stats:                         cloneFloatMap(stats),
		StatMode:                      battleStatModeProjected,
		SkillSlots:                    append([]string(nil), enabledSkillSlots...),
		SkillIDs:                      append([]string(nil), enabledSkillSlots...),
		SkillRules:                    filteredRules,
		ItemRules:                     itemRules,
		Skills:                        skills,
		BattleItems:                   battleItems,
		EquipmentDigest:               eqDigest,
		EquipmentIDs:                  eqIDs,
		equipmentSnapshotItems:        equipped,
		SpecializationTreeID:          specializationContribution.TreeID,
		SpecializationRulesetVersion:  specializationContribution.RulesetVersion,
		SpecializationBalanceRevision: specializationContribution.BalanceRevision,
		SpecializationRevision:        specializationContribution.Revision,
		SpecializationDigest:          specializationContribution.Digest,
		WeaponClasses:                 battleWeaponClassListFromSet(weaponClasses),
		BasicAttack:                   basicAttackProfile,
		EquipmentHooks:                equipmentHooks,
		SpecializationHooks:           specializationHooks,
		EffectMutations:               effectMutations,
		SpecializationEffectMutations: specializationEffectMutations,
		Buffs:                         []battleBuffState{},
	}
	unit.BaseStats = cloneFloatMap(stats)
	s.applyEquipmentSkillSnapshot(&unit, equipped, playerSkills, customSkillDefinitions)
	s.battleApplyFormationBuff(&unit)
	s.battleApplyUnitEquipmentRuntime(&unit)
	if presence, err := s.deps.Store.GetPresence(ctx, member.UserID); err == nil {
		offlineMax := time.Duration(s.deps.Config.GlobalSettings.Activity.OfflineProgressMaxDurationSeconds) * time.Second
		unit.RewardEligible = now.Sub(presence.LastSeenAt) <= offlineMax
	}
	return unit, nil
}

func (s *Server) buildBattleKittySnapshot(ctx context.Context, roomUnit store.BattleRoomUnit, now time.Time) (battleUnitState, error) {
	if roomUnit.KittyID == nil || *roomUnit.KittyID <= 0 {
		return battleUnitState{}, store.ErrKittyNotFound
	}
	kittyID := *roomUnit.KittyID
	if _, err := s.deps.Store.GetPlayerKitty(ctx, roomUnit.OwnerUserID, kittyID); err != nil {
		return battleUnitState{}, err
	}
	seeds, err := s.playerAttributeSeeds()
	if err != nil {
		return battleUnitState{}, err
	}
	attrStates, err := s.deps.Store.ListKittyAttributes(ctx, roomUnit.OwnerUserID, kittyID, seeds)
	if err != nil {
		return battleUnitState{}, err
	}
	derivedStats := map[string]float64{}
	if derived, err := s.deps.Store.GetKittyStats(ctx, roomUnit.OwnerUserID, kittyID); err == nil {
		for key, value := range derived.Derived {
			key = strings.TrimSpace(key)
			if key != "" {
				derivedStats[key] = value
			}
		}
	}
	projection := projectBattleKittyStats(attrStates, derivedStats)
	stats := projection.Stats
	playerSkills, err := s.deps.Store.ListKittySkills(ctx, roomUnit.OwnerUserID, kittyID)
	if err != nil {
		return battleUnitState{}, err
	}
	customSkillDefinitions, err := s.deps.Store.ListAcceptedCustomSkillDefinitions(ctx, roomUnit.OwnerUserID)
	if err != nil {
		return battleUnitState{}, fmt.Errorf("load kitty custom skill definitions: %w", err)
	}
	slotRule := s.battleSkillSlotRule()
	maxSkillSlots := battleAvailableSkillSlots(stats["intelligence"], slotRule)
	defaultBasicSkillID := s.battleDefaultBasicSkillID()
	enabledSkillSlots := make([]string, 0, len(playerSkills)+1)
	seenEnabled := map[string]struct{}{}
	filteredRules := map[string]battleSkillRuleState{}
	skillProgress := map[string]store.PlayerSkill{}
	for _, row := range playerSkills {
		skillID := strings.TrimSpace(row.SkillID)
		if skillID == "" {
			continue
		}
		if _, exists := s.battleSkillRawByIDWithCustom(skillID, customSkillDefinitions); !exists {
			continue
		}
		skillProgress[strings.ToLower(skillID)] = row
		buf, marshalErr := json.Marshal(row.RuleGroups)
		if marshalErr == nil {
			groups := []rules.ConditionGroup{}
			if unmarshalErr := json.Unmarshal(buf, &groups); unmarshalErr == nil && len(groups) > 0 {
				filteredRules[strings.ToLower(skillID)] = battleSkillRuleState{Enabled: true, Groups: groups}
			}
		}
		if !row.Enabled {
			continue
		}
		key := strings.ToLower(skillID)
		if _, exists := seenEnabled[key]; exists {
			continue
		}
		seenEnabled[key] = struct{}{}
		enabledSkillSlots = append(enabledSkillSlots, skillID)
	}
	if maxSkillSlots > 0 && len(enabledSkillSlots) > maxSkillSlots {
		enabledSkillSlots = enabledSkillSlots[:maxSkillSlots]
	}
	if defaultBasicSkillID != "" && s.battleSkillExists(defaultBasicSkillID) && !containsSkillID(enabledSkillSlots, defaultBasicSkillID) {
		if maxSkillSlots > 0 && len(enabledSkillSlots) >= maxSkillSlots {
			keep := maxSkillSlots - 1
			if keep < 0 {
				keep = 0
			}
			if keep < len(enabledSkillSlots) {
				enabledSkillSlots = enabledSkillSlots[:keep]
			}
		}
		enabledSkillSlots = append(enabledSkillSlots, defaultBasicSkillID)
	}
	equipped, err := s.deps.Store.ListKittyEquippedItems(ctx, roomUnit.OwnerUserID, kittyID)
	if err != nil {
		return battleUnitState{}, fmt.Errorf("load dynamically equipped kitty items: %w", err)
	}
	weaponClasses := s.weaponClassSetFromEquippedItems(equipped)
	personalityProfiles := s.resolveBattleKittyPersonalityProfiles(ctx, roomUnit.OwnerUserID, kittyID)
	eligibleSkillSlots := make([]string, 0, len(enabledSkillSlots))
	enableCtx := battleSkillEnableContext{Attributes: stats, WeaponClasses: weaponClasses}
	for _, skillID := range enabledSkillSlots {
		if strings.EqualFold(skillID, defaultBasicSkillID) {
			eligibleSkillSlots = append(eligibleSkillSlots, skillID)
			continue
		}
		raw, ok := s.battleSkillRawByIDWithCustom(skillID, customSkillDefinitions)
		if !ok {
			continue
		}
		if evaluateBattleSkillRequirement(raw, enableCtx).Eligible {
			eligibleSkillSlots = append(eligibleSkillSlots, skillID)
		}
	}
	enabledSkillSlots = eligibleSkillSlots
	skills := make([]battleSkillState, 0, len(enabledSkillSlots))
	for _, skillID := range enabledSkillSlots {
		progress := skillProgress[strings.ToLower(skillID)]
		raw, _ := s.battleSkillRawByIDWithCustom(skillID, customSkillDefinitions)
		skills = append(skills, s.battleSkillByRawWithProgress(skillID, progress.Level, progress.Exp, raw))
	}
	eqDigest := make([]string, 0, len(equipped))
	eqIDs := make([]int64, 0, len(equipped))
	for _, eq := range equipped {
		if strings.TrimSpace(eq.EquipType) != "battle" {
			continue
		}
		eqDigest = append(eqDigest, fmt.Sprintf("%s+%d", eq.TemplateItemID, eq.EnhanceLevel))
		eqIDs = append(eqIDs, eq.InstanceID)
	}
	name := roomUnit.KittyName
	if strings.TrimSpace(name) == "" {
		name = fmt.Sprintf("Kitty%d", kittyID)
	}
	unit := battleUnitState{
		UnitID:                          fmt.Sprintf("kitty:%d", kittyID),
		IsPlayerUnit:                    true,
		Side:                            "player",
		OriginSide:                      "player",
		BaseSide:                        "player",
		PerspectiveSide:                 "player",
		UserID:                          roomUnit.OwnerUserID,
		UnitType:                        "kitty",
		BattleLevel:                     battleLevelFromAttributeStates(attrStates),
		KittyID:                         kittyID,
		Name:                            name,
		Position:                        roomUnit.Position,
		BasePosition:                    roomUnit.Position,
		Alive:                           true,
		RewardEligible:                  true,
		HP:                              battleMaxHPFromStat(stats["hp"]),
		MP:                              maxFloat(0, stats["mp"]),
		MaxHP:                           battleMaxHPFromStat(stats["hp"]),
		MaxMP:                           maxFloat(0, stats["mp"]),
		Stats:                           cloneFloatMap(stats),
		StatMode:                        battleStatModeProjected,
		SkillSlots:                      append([]string(nil), enabledSkillSlots...),
		SkillIDs:                        append([]string(nil), enabledSkillSlots...),
		SkillRules:                      filteredRules,
		Skills:                          skills,
		BattleItems:                     []battleItemState{},
		EquipmentDigest:                 eqDigest,
		EquipmentIDs:                    eqIDs,
		equipmentSnapshotItems:          equipped,
		WeaponClasses:                   battleWeaponClassListFromSet(weaponClasses),
		BasicAttack:                     s.resolveBattleBasicAttackProfile(equipped),
		EquipmentHooks:                  s.resolveBattleEquipmentHooks(equipped),
		EffectMutations:                 s.resolveBattleAffixEffectMutations(equipped),
		KittyPersonalityHooks:           kittyPersonalityHooksOrEmpty(s.resolveBattlePersonalityHooks(personalityProfiles)),
		KittyPersonalityEffectMutations: kittyPersonalityMutationsOrEmpty(s.resolveBattlePersonalityEffectMutations(personalityProfiles)),
		Buffs:                           []battleBuffState{},
	}
	unit.BaseStats = cloneFloatMap(stats)
	s.battleApplyFormationBuff(&unit)
	s.battleApplyUnitEquipmentRuntime(&unit)
	if presence, err := s.deps.Store.GetPresence(ctx, roomUnit.OwnerUserID); err == nil {
		offlineMax := time.Duration(s.deps.Config.GlobalSettings.Activity.OfflineProgressMaxDurationSeconds) * time.Second
		unit.RewardEligible = now.Sub(presence.LastSeenAt) <= offlineMax
	}
	return unit, nil
}

func (s *Server) resolveBattleKittyPersonalityProfiles(ctx context.Context, userID, kittyID int64) []battlePersonalityProfile {
	if s == nil || s.deps.Store == nil || userID <= 0 || kittyID <= 0 {
		return nil
	}
	profiles, err := s.deps.Store.ListOwnedKittyPersonalityProfiles(ctx, userID, kittyID)

	if err != nil || len(profiles) == 0 {
		return nil
	}

	out := make([]battlePersonalityProfile, 0, len(profiles))
	for _, profile := range profiles {
		hooks := make([]map[string]any, 0, 2)
		mutations := make([]map[string]any, 0, 2)
		for _, effect := range profile.Effects {
			switch strings.ToLower(strings.TrimSpace(effect.Kind)) {
			case "battle_hook":
				hooks = append(hooks, cloneNestedAnyMapSlice(effect.BattleHooks)...)
			case "effect_mutation":
				mutations = append(mutations, cloneNestedAnyMapSlice(effect.EffectMutations)...)
			}
		}
		if len(hooks) == 0 && len(mutations) == 0 {
			continue
		}
		out = append(out, battlePersonalityProfile{
			ID:              strings.TrimSpace(profile.ID),
			NameKey:         strings.TrimSpace(profile.NameKey),
			BattleHooks:     hooks,
			EffectMutations: mutations,
		})
	}
	return out
}

func applyBattleSpecializationStatModifiers(stats map[string]float64, modifiers []store.SpecializationStatModifier) error {
	for _, modifier := range modifiers {
		key := strings.TrimSpace(modifier.AttributeKey)
		if key == "" || math.IsNaN(modifier.Value) || math.IsInf(modifier.Value, 0) {
			return store.ErrSpecializationDefinitionInvalid
		}
		switch strings.ToLower(strings.TrimSpace(modifier.Operation)) {
		case "add":
			stats[key] = roundTo4(stats[key] + modifier.Value)
		case "multiply":
			stats[key] = roundTo4(stats[key] * modifier.Value)
		default:
			return store.ErrSpecializationDefinitionInvalid
		}
	}
	return nil
}

func validateBattleSpecializationCanonicalKeys(node map[string]any, label string, allowed ...string) error {
	allowedSet := make(map[string]struct{}, len(allowed))
	for _, key := range allowed {
		allowedSet[key] = struct{}{}
	}
	for key := range node {
		if _, ok := allowedSet[key]; !ok {
			return fmt.Errorf("%w: %s has unsupported field %q", store.ErrSpecializationDefinitionInvalid, label, key)
		}
	}
	return nil
}

func validateBattleSpecializationStringList(raw any, label string) error {
	switch typed := raw.(type) {
	case []string:
		for _, item := range typed {
			if strings.TrimSpace(item) == "" {
				return fmt.Errorf("%w: %s must not contain empty values", store.ErrSpecializationDefinitionInvalid, label)
			}
		}
		return nil
	case []any:
		for _, item := range typed {
			value, ok := item.(string)
			if !ok || strings.TrimSpace(value) == "" {
				return fmt.Errorf("%w: %s must be a non-empty string array", store.ErrSpecializationDefinitionInvalid, label)
			}
		}
		return nil
	default:
		return fmt.Errorf("%w: %s must be a string array", store.ErrSpecializationDefinitionInvalid, label)
	}
}

func validateBattleSpecializationSubjectFilter(filter map[string]any) error {
	if err := validateBattleSpecializationCanonicalKeys(filter, "specialization subjectFilter", "requireAlive", "includeUnitTypes", "excludeUnitTypes", "includeSummonTemplateIds", "excludeSummonTemplateIds"); err != nil {
		return err
	}
	if raw, exists := filter["requireAlive"]; exists {
		if _, ok := raw.(bool); !ok {
			return fmt.Errorf("%w: specialization subjectFilter.requireAlive must be boolean", store.ErrSpecializationDefinitionInvalid)
		}
	}
	for _, key := range []string{"includeUnitTypes", "excludeUnitTypes", "includeSummonTemplateIds", "excludeSummonTemplateIds"} {
		if raw, exists := filter[key]; exists {
			if err := validateBattleSpecializationStringList(raw, "specialization subjectFilter."+key); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateBattleSpecializationCondition(condition map[string]any) error {
	kind := battleNormalizeTrigger(fmt.Sprintf("%v", condition["kind"]))
	if !battleSpecializationConditionSupported(kind) {
		return fmt.Errorf("%w: unsupported specialization condition %q", store.ErrSpecializationDefinitionInvalid, kind)
	}
	allowed := []string{"kind"}
	switch kind {
	case "contextdamagetype", "contexthitcountgte", "selfhpratiolte", "selfmpratiogte":
		allowed = append(allowed, "value")
	case "selfweaponclass":
		allowed = append(allowed, "weaponClasses")
	case "countergte":
		allowed = append(allowed, "counterId", "value")
	case "targethasmark":
		allowed = append(allowed, "markId")
	case "targethasbuff":
		allowed = append(allowed, "buffId")
	case "targethasanybuff":
		allowed = append(allowed, "buffIds")
	case "triggerlimit":
		allowed = append(allowed, "perTurn", "perWave", "perBattle")
	}
	if err := validateBattleSpecializationCanonicalKeys(condition, "specialization condition "+kind, allowed...); err != nil {
		return err
	}
	if kind == "targethasbuff" {
		buffID, ok := condition["buffId"].(string)
		if !ok || strings.TrimSpace(buffID) == "" {
			return fmt.Errorf("%w: specialization condition target_has_buff buffId is required", store.ErrSpecializationDefinitionInvalid)
		}
	}
	if kind == "targethasanybuff" {
		if err := validateBattleSpecializationStringList(condition["buffIds"], "specialization condition target_has_any_buff.buffIds"); err != nil {
			return err
		}
		if len(compactStringSlice(condition["buffIds"])) == 0 {
			return fmt.Errorf("%w: specialization condition target_has_any_buff.buffIds must not be empty", store.ErrSpecializationDefinitionInvalid)
		}
	}
	if kind == "selfweaponclass" {
		if err := validateBattleSpecializationStringList(condition["weaponClasses"], "specialization condition self_weapon_class.weaponClasses"); err != nil {
			return err
		}
		if len(compactStringSlice(condition["weaponClasses"])) == 0 {
			return fmt.Errorf("%w: specialization condition self_weapon_class.weaponClasses must not be empty", store.ErrSpecializationDefinitionInvalid)
		}
	}
	return nil
}

func battleSpecializationRuntimeGrantID(treeID, rulesetVersion string, grant store.SpecializationRuntimeGrant) (string, error) {
	rulesetVersion = strings.TrimSpace(rulesetVersion)
	if rulesetVersion != "specialization-v5" && rulesetVersion != "specialization-v6" {
		return "", fmt.Errorf("%w: unsupported specialization ruleset %q", store.ErrSpecializationDefinitionInvalid, rulesetVersion)
	}
	contributionID := strings.TrimSpace(grant.ID)
	rawPayloadID, ok := grant.Payload["id"].(string)
	payloadID := strings.TrimSpace(rawPayloadID)
	if !ok || contributionID == "" || payloadID == "" {
		return "", fmt.Errorf("%w: specialization contribution and payload ids are required", store.ErrSpecializationDefinitionInvalid)
	}
	treeID = strings.TrimSpace(treeID)
	nodeID := strings.TrimSpace(grant.SourceNodeID)
	prefix := treeID + ":" + nodeID + ":"
	suffix := ":" + payloadID
	remainder := strings.TrimPrefix(contributionID, prefix)
	grantID := strings.TrimSuffix(remainder, suffix)
	if treeID == "" || nodeID == "" || !strings.HasPrefix(contributionID, prefix) || !strings.HasSuffix(remainder, suffix) ||
		!store.IsValidSpecializationRuntimeIDPart(rawPayloadID) || !store.IsValidSpecializationRuntimeIDPart(grantID) {
		return "", fmt.Errorf("%w: specialization contribution id must contain tree, node, grant, and payload ids", store.ErrSpecializationDefinitionInvalid)
	}
	return contributionID, nil
}

func (s *Server) resolveBattleSpecializationHooks(contribution store.SpecializationBattleContribution) ([]battleEquipmentHookRule, error) {
	out := make([]battleEquipmentHookRule, 0, len(contribution.Hooks))
	seenRuntimeIDs := make(map[string]struct{}, len(contribution.Hooks))
	for _, grant := range contribution.Hooks {
		hook := grant.Payload
		if len(hook) == 0 {
			return nil, fmt.Errorf("%w: specialization hook grant %s is empty", store.ErrSpecializationDefinitionInvalid, grant.ID)
		}
		if err := validateBattleSpecializationCanonicalKeys(hook, "specialization hook "+grant.ID, "id", "trigger", "priority", "conditions", "effects", "eventFilter", "limits", "dispatchHooks", "subjectFilter"); err != nil {
			return nil, err
		}
		trigger := battleNormalizeTrigger(fmt.Sprintf("%v", hook["trigger"]))
		effects := battleAnyMapSlice(hook["effects"])
		if trigger == "" || len(effects) == 0 {
			return nil, fmt.Errorf("%w: specialization hook grant %s requires trigger and effects", store.ErrSpecializationDefinitionInvalid, grant.ID)
		}
		if !battleSpecializationTriggerSupported(trigger) {
			return nil, fmt.Errorf("%w: specialization hook %s uses unsupported trigger %q", store.ErrSpecializationDefinitionInvalid, grant.ID, trigger)
		}
		rawDispatchHooks, exists := hook["dispatchHooks"]
		if !exists {
			return nil, fmt.Errorf("%w: specialization hook %s dispatchHooks is required", store.ErrSpecializationDefinitionInvalid, grant.ID)
		}
		dispatchHooks, ok := rawDispatchHooks.(bool)
		if !ok {
			return nil, fmt.Errorf("%w: specialization hook %s dispatchHooks must be boolean", store.ErrSpecializationDefinitionInvalid, grant.ID)
		}
		runtimeHookID, err := battleSpecializationRuntimeGrantID(contribution.TreeID, contribution.RulesetVersion, grant)
		if err != nil {
			return nil, err
		}
		if _, duplicate := seenRuntimeIDs[runtimeHookID]; duplicate {
			return nil, fmt.Errorf("%w: duplicate specialization hook contribution id %s", store.ErrSpecializationDefinitionInvalid, runtimeHookID)
		}
		seenRuntimeIDs[runtimeHookID] = struct{}{}
		conditions := make([]map[string]any, 0, len(battleAnyMapSlice(hook["conditions"])))
		maxPerTurn := 0
		maxPerWave := 0
		maxPerBattle := 0
		mergeLimit := func(current, next int) int {
			if next <= 0 {
				return current
			}
			if current <= 0 || next < current {
				return next
			}
			return current
		}
		if limitsRaw, exists := hook["limits"]; exists && limitsRaw != nil {
			limits := normalizeAnyMap(limitsRaw)
			if len(limits) == 0 {
				return nil, fmt.Errorf("%w: specialization hook %s limits must be an object", store.ErrSpecializationDefinitionInvalid, grant.ID)
			}
			for key, raw := range limits {
				normalizedKey := battleNormalizeTrigger(key)
				if normalizedKey != "perturn" && normalizedKey != "perwave" && normalizedKey != "perbattle" {
					return nil, fmt.Errorf("%w: specialization hook %s uses unsupported limit %q", store.ErrSpecializationDefinitionInvalid, grant.ID, key)
				}
				value := toFloat64(raw, math.NaN())
				if math.IsNaN(value) || math.IsInf(value, 0) || value <= 0 || value != math.Trunc(value) {
					return nil, fmt.Errorf("%w: specialization hook %s limit %s must be a positive integer", store.ErrSpecializationDefinitionInvalid, grant.ID, key)
				}
				switch normalizedKey {
				case "perturn":
					maxPerTurn = mergeLimit(maxPerTurn, int(value))
				case "perwave":
					maxPerWave = mergeLimit(maxPerWave, int(value))
				case "perbattle":
					maxPerBattle = mergeLimit(maxPerBattle, int(value))
				}
			}
		}
		for _, condition := range battleAnyMapSlice(hook["conditions"]) {
			if err := validateBattleSpecializationCondition(condition); err != nil {
				return nil, fmt.Errorf("specialization hook %s: %w", grant.ID, err)
			}
			if err := validateBattleSpecializationConditionTrigger(trigger, condition); err != nil {
				return nil, fmt.Errorf("specialization hook %s: %w", grant.ID, err)
			}
			kind := battleNormalizeTrigger(fmt.Sprintf("%v", condition["kind"]))
			if kind == "triggerlimit" {
				maxPerTurn = mergeLimit(maxPerTurn, int(toInt64Loose(firstNonNil(condition["perTurn"], condition["per_turn"]), 0)))
				maxPerWave = mergeLimit(maxPerWave, int(toInt64Loose(firstNonNil(condition["perWave"], condition["per_wave"]), 0)))
				maxPerBattle = mergeLimit(maxPerBattle, int(toInt64Loose(firstNonNil(condition["perBattle"], condition["per_battle"]), 0)))
				if maxPerTurn <= 0 && maxPerWave <= 0 && maxPerBattle <= 0 {
					return nil, fmt.Errorf("%w: specialization hook %s has empty trigger limit", store.ErrSpecializationDefinitionInvalid, grant.ID)
				}
				continue
			}
			conditions = append(conditions, condition)
		}
		for _, effect := range effects {
			if err := validateBattleSpecializationHookEffect(effect); err != nil {
				return nil, fmt.Errorf("specialization hook %s: %w", grant.ID, err)
			}
			if err := validateBattleSpecializationEffectTrigger(trigger, effect); err != nil {
				return nil, fmt.Errorf("specialization hook %s: %w", grant.ID, err)
			}
		}
		if err := validateBattleSpecializationV5ExtraTurnSafety(trigger, effects, maxPerWave, maxPerBattle); err != nil {
			return nil, fmt.Errorf("specialization hook %s: %w", grant.ID, err)
		}
		if err := validateBattleSpecializationV5RiskPolicy(battleAnyMapSlice(hook["conditions"]), effects, maxPerTurn, maxPerWave, maxPerBattle); err != nil {
			return nil, fmt.Errorf("specialization hook %s: %w", grant.ID, err)
		}
		eventFilter := normalizeAnyMap(firstNonNil(hook["eventFilter"], hook["event_filter"]))
		if err := validateBattleSpecializationEventFilter(eventFilter); err != nil {
			return nil, fmt.Errorf("specialization hook %s: %w", grant.ID, err)
		}
		if err := validateBattleSpecializationEventFilterForTrigger(trigger, eventFilter); err != nil {
			return nil, fmt.Errorf("specialization hook %s: %w", grant.ID, err)
		}
		subjectFilter := normalizeAnyMap(hook["subjectFilter"])
		if _, exists := hook["subjectFilter"]; exists && subjectFilter == nil {
			return nil, fmt.Errorf("%w: specialization hook %s subjectFilter must be an object", store.ErrSpecializationDefinitionInvalid, grant.ID)
		}
		if err := validateBattleSpecializationSubjectFilter(subjectFilter); err != nil {
			return nil, fmt.Errorf("specialization hook %s: %w", grant.ID, err)
		}
		out = append(out, battleEquipmentHookRule{
			ID:                   runtimeHookID,
			SourceKind:           "specialization",
			SourceID:             grant.SourceNodeID,
			SourceNameKey:        grant.SourceNameKey,
			SourceItemID:         "specialization:" + grant.SourceNodeID,
			Trigger:              trigger,
			Priority:             int(toInt64Loose(hook["priority"], 0)),
			MaxTriggersPerTurn:   maxPerTurn,
			MaxTriggersPerWave:   maxPerWave,
			MaxTriggersPerBattle: maxPerBattle,
			DispatchHooks:        dispatchHooks,
			SubjectFilter:        subjectFilter,
			EventFilter:          eventFilter,
			Conditions:           conditions,
			Effects:              effects,
		})
	}
	return out, nil
}

func battleSpecializationTriggerSupported(trigger string) bool {
	switch battleNormalizeTrigger(trigger) {
	case "onafterdealcrit", "onafterdealdamage", "onafterskillcast", "onaftertakedamage", "onallybeforetakedamage", "onbattlestart", "onbeforedealdamage", "onbeforeskillcast", "onbeforetakedamage", "onkill", "onturnend", "onturnstart", "onwavestart":
		return true
	default:
		return false
	}
}

func battleSpecializationTriggerHasSkillContext(trigger string) bool {
	switch battleNormalizeTrigger(trigger) {
	case "onbeforeskillcast", "onafterskillcast", "onbeforedealdamage", "onafterdealdamage", "onafterdealcrit", "onbeforetakedamage", "onaftertakedamage", "onallybeforetakedamage", "onkill":
		return true
	default:
		return false
	}
}

func battleSpecializationTriggerHasDamageContext(trigger string) bool {
	switch battleNormalizeTrigger(trigger) {
	case "onbeforedealdamage", "onafterdealdamage", "onafterdealcrit", "onbeforetakedamage", "onaftertakedamage", "onallybeforetakedamage", "onkill":
		return true
	default:
		return false
	}
}

func battleSpecializationTriggerHasEventSubject(trigger string) bool {
	return battleSpecializationTriggerHasDamageContext(trigger)
}

func validateBattleSpecializationConditionTrigger(trigger string, condition map[string]any) error {
	kind := battleNormalizeTrigger(fmt.Sprintf("%v", condition["kind"]))
	require := func(ok bool, contextName string) error {
		if ok {
			return nil
		}
		return fmt.Errorf("%w: specialization condition %s requires %s", store.ErrSpecializationDefinitionInvalid, kind, contextName)
	}
	switch kind {
	case "contextdamagetype", "contexthitcountgte":
		return require(battleSpecializationTriggerHasDamageContext(trigger), "a damage trigger")
	case "contextmpinsufficient":
		return require(battleNormalizeTrigger(trigger) == "onbeforeskillcast", "onBeforeSkillCast")
	case "contextskillnonbasic":
		return require(battleSpecializationTriggerHasSkillContext(trigger), "a skill or damage trigger")
	case "incomingdamagewouldbelethal":
		normalized := battleNormalizeTrigger(trigger)
		return require(normalized == "onbeforetakedamage" || normalized == "onallybeforetakedamage", "a before-take-damage trigger")
	case "sourceisenemy", "targethasanybuff", "targethasanyspecializationmark", "targethasbuff", "targethasmark", "targetislowesthpally":
		return require(battleSpecializationTriggerHasEventSubject(trigger), "an event subject")
	default:
		return nil
	}
}

func validateBattleSpecializationEffectTrigger(trigger string, effect map[string]any) error {
	trigger = battleNormalizeTrigger(trigger)
	kind := battleNormalizeTrigger(fmt.Sprintf("%v", effect["kind"]))
	allowed := func(values ...string) bool {
		for _, value := range values {
			if trigger == battleNormalizeTrigger(value) {
				return true
			}
		}
		return false
	}
	switch kind {
	case "consumecounterfordamageamp", "flatdefensepenetration", "runtimedamageamp":
		if !allowed("onBeforeDealDamage", "onBeforeTakeDamage") {
			return fmt.Errorf("%w: specialization effect %s requires a before-damage trigger", store.ErrSpecializationDefinitionInvalid, kind)
		}
	case "redirectdamage":
		if !allowed("onAllyBeforeTakeDamage") {
			return fmt.Errorf("%w: specialization effect redirect_damage requires onAllyBeforeTakeDamage", store.ErrSpecializationDefinitionInvalid)
		}
	case "preventlethal", "resourceshield":
		if !allowed("onBeforeTakeDamage") {
			return fmt.Errorf("%w: specialization effect %s requires onBeforeTakeDamage", store.ErrSpecializationDefinitionInvalid, kind)
		}
	case "resourcedebtcast":
		if !allowed("onBeforeSkillCast") {
			return fmt.Errorf("%w: specialization effect resource_debt_cast requires onBeforeSkillCast", store.ErrSpecializationDefinitionInvalid)
		}
	case "mirrorcastskill":
		if !allowed("onAfterSkillCast") {
			return fmt.Errorf("%w: specialization effect mirror_cast_skill requires onAfterSkillCast", store.ErrSpecializationDefinitionInvalid)
		}
	case "buffcycle", "markcycle":
		if !allowed("onAfterDealDamage") {
			return fmt.Errorf("%w: specialization effect %s requires onAfterDealDamage", store.ErrSpecializationDefinitionInvalid, kind)
		}
	case "repeatcontexteffect":
		if !allowed("onBeforeDealDamage", "onBeforeTakeDamage", "onAfterDealDamage", "onAfterTakeDamage", "onAfterDealCrit") {
			return fmt.Errorf("%w: specialization effect repeat_context_effect requires a damage trigger", store.ErrSpecializationDefinitionInvalid)
		}
	case "heal":
		if trigger == "onbeforetakedamage" && battleNormalizeTrigger(fmt.Sprintf("%v", effect["targetMode"])) != "self" {
			return fmt.Errorf("%w: specialization onBeforeTakeDamage heal targetMode must be self", store.ErrSpecializationDefinitionInvalid)
		}
	case "consumemark":
		if !battleSpecializationTriggerHasEventSubject(trigger) {
			return fmt.Errorf("%w: specialization effect consume_mark requires an event subject", store.ErrSpecializationDefinitionInvalid)
		}
	}
	if kind == "adjustskillcooldown" && battleNormalizeSkillCooldownSelectionMode(fmt.Sprintf("%v", effect["selectionMode"])) == "context_skill" && !battleSpecializationTriggerHasSkillContext(trigger) {
		return fmt.Errorf("%w: specialization adjust_skill_cooldown context_skill requires a skill or damage trigger", store.ErrSpecializationDefinitionInvalid)
	}
	targetMode := battleNormalizeTrigger(fmt.Sprintf("%v", effect["targetMode"]))
	if (targetMode == "source" || targetMode == "targeting") && !battleSpecializationTriggerHasEventSubject(trigger) {
		return fmt.Errorf("%w: specialization effect %s targetMode %s requires an event subject", store.ErrSpecializationDefinitionInvalid, kind, targetMode)
	}
	return nil
}

func battleSpecializationConditionSupported(kind string) bool {
	switch battleNormalizeTrigger(kind) {
	case "adjacentallyexists", "contextdamagetype", "contexthitcountgte", "contextmpinsufficient", "contextskillnonbasic", "countergte", "incomingdamagewouldbelethal", "selfhpratiolte", "selfmpratiogte", "selfweaponclass", "sourceisenemy", "targethasanybuff", "targethasanyspecializationmark", "targethasbuff", "targethasmark", "targetislowesthpally", "triggerlimit":
		return true
	default:
		return false
	}
}

func validateBattleSpecializationEventFilter(filter map[string]any) error {
	if err := validateBattleSpecializationCanonicalKeys(
		filter,
		"specialization eventFilter",
		"skillIds", "excludeSkillIds", "skillCategories", "damageTypes", "unitTypes",
		"probabilityKinds", "sourceIds", "excludeSourceIds",
		"buffIds", "excludeBuffIds", "buffKinds", "applicationKinds", "turnKinds",
		"includeBasicAttack", "excludeSelf", "firstBuffInstanceOnly", "includePreparation", "includeSuppressedTurns",
	); err != nil {
		return err
	}
	for key, raw := range filter {
		switch battleNormalizeTrigger(key) {
		case "skillids", "excludeskillids", "skillcategories", "damagetypes", "unittypes",
			"probabilitykinds", "sourceids", "excludesourceids",
			"buffids", "excludebuffids", "buffkinds", "applicationkinds", "turnkinds":
			validList := false
			switch typed := raw.(type) {
			case string, []string:
				validList = true
			case []any:
				validList = true
				for _, item := range typed {
					if _, ok := item.(string); !ok {
						validList = false
						break
					}
				}
			}
			if !validList {
				return fmt.Errorf("%w: eventFilter.%s must be a string list", store.ErrSpecializationDefinitionInvalid, key)
			}
		case "includebasicattack", "excludeself", "firstbuffinstanceonly", "includepreparation", "includesuppressedturns":
			if _, ok := raw.(bool); !ok {
				return fmt.Errorf("%w: eventFilter.%s must be boolean", store.ErrSpecializationDefinitionInvalid, key)
			}
		default:
			return fmt.Errorf("%w: unsupported eventFilter field %q", store.ErrSpecializationDefinitionInvalid, key)
		}
	}
	return nil
}

func validateBattleSpecializationEventFilterForTrigger(trigger string, filter map[string]any) error {
	if len(filter) == 0 {
		return nil
	}
	trigger = battleNormalizeTrigger(trigger)
	skillContext := battleSpecializationTriggerHasSkillContext(trigger)
	damageContext := battleSpecializationTriggerHasDamageContext(trigger)
	subjectContext := battleSpecializationTriggerHasEventSubject(trigger)
	for key := range filter {
		normalized := battleNormalizeTrigger(key)
		switch normalized {
		case "buffids", "excludebuffids", "buffkinds", "firstbuffinstanceonly", "applicationkinds":
			return fmt.Errorf("%w: specialization eventFilter.%s requires a buff trigger, which specializations do not expose", store.ErrSpecializationDefinitionInvalid, key)
		case "probabilitykinds", "sourceids", "excludesourceids":
			return fmt.Errorf("%w: specialization eventFilter.%s requires a probability trigger, which specializations do not expose", store.ErrSpecializationDefinitionInvalid, key)
		case "skillids", "excludeskillids", "skillcategories", "includebasicattack":
			if !skillContext {
				return fmt.Errorf("%w: specialization eventFilter.%s requires a skill or damage trigger", store.ErrSpecializationDefinitionInvalid, key)
			}
		case "damagetypes":
			if !damageContext {
				return fmt.Errorf("%w: specialization eventFilter.damageTypes requires a damage trigger", store.ErrSpecializationDefinitionInvalid)
			}
		case "unittypes", "excludeself":
			if !subjectContext {
				return fmt.Errorf("%w: specialization eventFilter.%s requires an event subject", store.ErrSpecializationDefinitionInvalid, key)
			}
		case "turnkinds", "includepreparation":
			if trigger != "onturnstart" && trigger != "onturnend" {
				return fmt.Errorf("%w: specialization eventFilter.%s requires onTurnStart or onTurnEnd", store.ErrSpecializationDefinitionInvalid, key)
			}
		case "includesuppressedturns":
			if trigger != "onturnend" {
				return fmt.Errorf("%w: specialization eventFilter.includeSuppressedTurns requires onTurnEnd", store.ErrSpecializationDefinitionInvalid)
			}
		}
	}
	return nil
}

func validateBattleSpecializationTargetMode(effect map[string]any, kind string, required bool, allowed ...string) error {
	raw, exists := effect["targetMode"]
	if !exists {
		if required {
			return fmt.Errorf("%w: specialization effect %s targetMode is required", store.ErrSpecializationDefinitionInvalid, kind)
		}
		return nil
	}
	value, ok := raw.(string)
	if !ok || strings.TrimSpace(value) == "" {
		return fmt.Errorf("%w: specialization effect %s targetMode must be a non-empty string", store.ErrSpecializationDefinitionInvalid, kind)
	}
	normalized := battleNormalizeTrigger(value)
	for _, candidate := range allowed {
		if normalized == battleNormalizeTrigger(candidate) {
			return nil
		}
	}
	return fmt.Errorf("%w: specialization effect %s uses unsupported targetMode %q", store.ErrSpecializationDefinitionInvalid, kind, value)
}

func validateBattleSpecializationHookEffect(effect map[string]any) error {
	kind := battleNormalizeTrigger(fmt.Sprintf("%v", effect["kind"]))
	allowedByKind := map[string][]string{
		"actionmetergain":            {"kind", "value", "valueFormula"},
		"addbuff":                    {"kind", "buffId", "durationTurns", "targetMode", "selectionMode", "dispatchHooks"},
		"adjustskillcooldown":        {"kind", "targetMode", "amount", "selectionMode", "maxSkills", "skillIds"},
		"castskill":                  {"kind", "skillId", "amountRatio", "dispatchHooks"},
		"consumecounterfordamageamp": {"kind", "counterId", "consume", "value", "percentDefensePenetration", "valueFormula"},
		"consumemark":                {"kind", "markId"},
		"cyclecounter":               {"kind", "counterId", "gain", "maxStacks", "atMax"},
		"damage":                     {"kind", "targetMode", "damageType", "amountFormula", "dispatchHooks"},
		"designateprioritytarget":    {"kind", "markId", "selectionMode", "rewriteTargeting"},
		"extraturn":                  {"kind", "targetMode", "turnMode", "procChanceBase", "procChanceMax", "abortHookOnFail", "dispatchHooks"},
		"flatdefensepenetration":     {"kind", "valueFormula"},
		"heal":                       {"kind", "targetMode", "maxHpRatio", "missingHpRatio", "valueFormula"},
		"buffcycle":                  {"kind", "buffId", "firstHit", "buffedHit"},
		"markcycle":                  {"kind", "markId", "firstHit", "markedHit"},
		"marktarget":                 {"kind", "markId", "selectionMode", "targetMode", "durationTurns"},
		"mirrorcastskill":            {"kind", "amountRatio", "applyCooldown", "allowMirroredCastTrigger", "skillIds", "dispatchHooks", "valueFormula"},
		"preventlethal":              {"kind", "remainingHpRatio", "valueFormula"},
		"redirectdamage":             {"kind", "targetMode", "ratio", "valueFormula"},
		"removebuff":                 {"kind", "buffId", "targetMode", "dispatchHooks"},
		"removebuffbykind":           {"kind", "buffKind", "selectionMode", "maxBuffs"},
		"repeatcontexteffect":        {"kind", "amountRatio", "targetMode", "dispatchHooks", "valueFormula"},
		"resourcedebtcast":           {"kind", "hpCostRatio", "cooldownDelta", "minimumHp", "valueFormula"},
		"resourceshield":             {"kind", "resource", "absorbRatio", "resourcePerDamage", "valueFormula"},
		"restoremp":                  {"kind", "maxMpRatio", "valueFormula"},
		"retaliate":                  {"kind", "targetMode", "strengthRatio", "staminaRatio", "dispatchHooks"},
		"runtimedamageamp":           {"kind", "value", "valueFormula"},
		"shareddamagereduction":      {"kind", "value", "durationTurns", "valueFormula"},
		"shiftposition":              {"kind", "policy", "maxSteps"},
	}
	allowed, supported := allowedByKind[kind]
	if !supported {
		return fmt.Errorf("%w: unsupported hook effect %q", store.ErrSpecializationDefinitionInvalid, kind)
	}
	if err := validateBattleSpecializationCanonicalKeys(effect, "specialization effect "+kind, allowed...); err != nil {
		return err
	}
	if raw, exists := effect["dispatchHooks"]; exists {
		if _, ok := raw.(bool); !ok {
			return fmt.Errorf("%w: specialization effect %s dispatchHooks must be boolean", store.ErrSpecializationDefinitionInvalid, kind)
		}
	}
	switch kind {
	case "addbuff", "castskill", "damage", "extraturn", "mirrorcastskill", "removebuff", "repeatcontexteffect", "retaliate":
		if _, exists := effect["dispatchHooks"]; !exists {
			return fmt.Errorf("%w: specialization effect %s dispatchHooks is required", store.ErrSpecializationDefinitionInvalid, kind)
		}
	}
	switch kind {
	case "addbuff", "removebuff":
		buffID := strings.TrimSpace(fmt.Sprintf("%v", effect["buffId"]))
		if buffID == "" || buffID == "<nil>" {
			return fmt.Errorf("%w: specialization effect %s buffId is required", store.ErrSpecializationDefinitionInvalid, kind)
		}
		if raw, exists := effect["durationTurns"]; exists && raw != nil {
			duration := toFloat64(raw, math.NaN())
			if math.IsNaN(duration) || math.IsInf(duration, 0) || duration < 1 || duration != math.Trunc(duration) {
				return fmt.Errorf("%w: specialization effect %s durationTurns must be a positive integer", store.ErrSpecializationDefinitionInvalid, kind)
			}
		}
		if kind == "removebuff" {
			if _, exists := effect["durationTurns"]; exists {
				return fmt.Errorf("%w: specialization effect remove_buff cannot use durationTurns", store.ErrSpecializationDefinitionInvalid)
			}
		}
		if err := validateBattleSpecializationTargetMode(effect, kind, false, "self", "source", "target", "enemy", "targeting", "lowest_hp_ally"); err != nil {
			return err
		}
		if raw, exists := effect["selectionMode"]; exists {
			selectionMode, ok := raw.(string)
			if !ok {
				return fmt.Errorf("%w: specialization effect add_buff selectionMode must be a string", store.ErrSpecializationDefinitionInvalid)
			}
			switch battleNormalizeTrigger(selectionMode) {
			case "highestattack", "highestthreat", "lowesthp", "random":
			default:
				return fmt.Errorf("%w: specialization effect add_buff uses unsupported selectionMode %q", store.ErrSpecializationDefinitionInvalid, selectionMode)
			}
			if kind != "addbuff" || battleNormalizeTrigger(fmt.Sprintf("%v", effect["targetMode"])) != "enemy" {
				return fmt.Errorf("%w: specialization effect add_buff selectionMode requires enemy targetMode", store.ErrSpecializationDefinitionInvalid)
			}
		}
	case "adjustskillcooldown":
		if err := validateBattleSpecializationTargetMode(effect, kind, false, "self", "source", "enemy", "marked_enemy", "targeting", "lowest_hp_ally"); err != nil {
			return err
		}
		selectionMode := ""
		if raw, exists := effect["selectionMode"]; exists {
			value, ok := raw.(string)
			if !ok {
				return fmt.Errorf("%w: specialization effect adjust_skill_cooldown selectionMode must be a string", store.ErrSpecializationDefinitionInvalid)
			}
			selectionMode = battleNormalizeSkillCooldownSelectionMode(value)
			normalized := battleNormalizeTrigger(value)
			if normalized != "maxremaining" && normalized != "contextskill" && normalized != "all" {
				return fmt.Errorf("%w: specialization effect adjust_skill_cooldown uses unsupported selectionMode %q", store.ErrSpecializationDefinitionInvalid, value)
			}
		}
		if selectionMode == "" {
			if _, exists := effect["skillIds"]; !exists {
				return fmt.Errorf("%w: specialization effect adjust_skill_cooldown requires selectionMode or skillIds", store.ErrSpecializationDefinitionInvalid)
			}
		}
		if raw, exists := effect["skillIds"]; exists {
			if err := validateBattleSpecializationStringList(raw, "specialization effect adjust_skill_cooldown.skillIds"); err != nil {
				return err
			}
		}
		if selectionMode == "context_skill" {
			if raw, exists := effect["maxSkills"]; exists {
				value := toFloat64(raw, math.NaN())
				if math.IsNaN(value) || math.IsInf(value, 0) || value != 1 {
					return fmt.Errorf("%w: specialization effect adjust_skill_cooldown context_skill maxSkills must be 1", store.ErrSpecializationDefinitionInvalid)
				}
			}
		}
	case "damage", "heal", "repeatcontexteffect":
		if err := validateBattleSpecializationTargetMode(effect, kind, true, "self", "source", "target", "enemy", "marked_enemy", "targeting", "lowest_hp_ally"); err != nil {
			return err
		}
	case "redirectdamage":
		if err := validateBattleSpecializationTargetMode(effect, kind, true, "self"); err != nil {
			return err
		}
	case "retaliate":
		if err := validateBattleSpecializationTargetMode(effect, kind, true, "source"); err != nil {
			return err
		}
	case "extraturn":
		if err := validateBattleSpecializationTargetMode(effect, kind, true, "self"); err != nil {
			return err
		}
		turnMode, ok := effect["turnMode"].(string)
		if !ok || battleNormalizeTrigger(turnMode) != "full" {
			return fmt.Errorf("%w: specialization effect extra_turn turnMode must be full", store.ErrSpecializationDefinitionInvalid)
		}
		_, hasChanceBase := effect["procChanceBase"]
		_, hasChanceMax := effect["procChanceMax"]
		chanceConfigured := hasChanceBase || hasChanceMax
		if chanceConfigured {
			if !hasChanceBase {
				return fmt.Errorf("%w: specialization effect extra_turn procChanceBase is required when probability is configured", store.ErrSpecializationDefinitionInvalid)
			}
			chanceBase := toFloat64(effect["procChanceBase"], math.NaN())
			if math.IsNaN(chanceBase) || math.IsInf(chanceBase, 0) || chanceBase < 0 || chanceBase > 1 {
				return fmt.Errorf("%w: specialization effect extra_turn procChanceBase must be between 0 and 1", store.ErrSpecializationDefinitionInvalid)
			}
			if hasChanceMax {
				chanceMax := toFloat64(effect["procChanceMax"], math.NaN())
				if math.IsNaN(chanceMax) || math.IsInf(chanceMax, 0) || chanceMax < chanceBase || chanceMax > 1 {
					return fmt.Errorf("%w: specialization effect extra_turn procChanceMax must be between procChanceBase and 1", store.ErrSpecializationDefinitionInvalid)
				}
			}
		}
		if raw, exists := effect["abortHookOnFail"]; exists {
			abortOnFail, ok := raw.(bool)
			if !ok {
				return fmt.Errorf("%w: specialization effect extra_turn abortHookOnFail must be boolean", store.ErrSpecializationDefinitionInvalid)
			}
			if abortOnFail && !chanceConfigured {
				return fmt.Errorf("%w: specialization effect extra_turn abortHookOnFail requires procChanceBase", store.ErrSpecializationDefinitionInvalid)
			}
		}
	case "shiftposition":
		policy, ok := effect["policy"].(string)
		if !ok || battleNormalizeTrigger(policy) != "towardlowesthpally" {
			return fmt.Errorf("%w: specialization effect shift_position uses unsupported policy %q", store.ErrSpecializationDefinitionInvalid, policy)
		}
		maxSteps := toFloat64(effect["maxSteps"], math.NaN())
		if math.IsNaN(maxSteps) || math.IsInf(maxSteps, 0) || maxSteps < 1 || maxSteps != math.Trunc(maxSteps) {
			return fmt.Errorf("%w: specialization effect shift_position maxSteps must be a positive integer", store.ErrSpecializationDefinitionInvalid)
		}
	case "castskill":
		skillID := strings.TrimSpace(fmt.Sprintf("%v", effect["skillId"]))
		if skillID == "" || skillID == "<nil>" {
			return fmt.Errorf("%w: specialization effect cast_skill skillId is required", store.ErrSpecializationDefinitionInvalid)
		}
		if raw, exists := effect["amountRatio"]; exists && raw != nil {
			ratio := toFloat64(raw, math.NaN())
			if math.IsNaN(ratio) || math.IsInf(ratio, 0) || ratio <= 0 {
				return fmt.Errorf("%w: specialization effect cast_skill amountRatio must be positive and finite", store.ErrSpecializationDefinitionInvalid)
			}
		}
	case "mirrorcastskill":
		if raw, exists := effect["allowMirroredCastTrigger"]; exists {
			if _, ok := raw.(bool); !ok {
				return fmt.Errorf("%w: specialization effect mirror_cast_skill allowMirroredCastTrigger must be boolean", store.ErrSpecializationDefinitionInvalid)
			}
		}
		if raw, exists := effect["skillIds"]; exists {
			values := compactStringSlice(raw)
			if len(values) == 0 {
				return fmt.Errorf("%w: specialization effect mirror_cast_skill skillIds must be a non-empty string array", store.ErrSpecializationDefinitionInvalid)
			}
		}
	case "marktarget", "designateprioritytarget":
		markID, ok := effect["markId"].(string)
		if !ok || strings.TrimSpace(markID) == "" {
			return fmt.Errorf("%w: specialization effect %s markId is required", store.ErrSpecializationDefinitionInvalid, kind)
		}
		selectionMode := ""
		if raw, exists := effect["selectionMode"]; exists {
			value, ok := raw.(string)
			if !ok {
				return fmt.Errorf("%w: specialization effect %s selectionMode must be a string", store.ErrSpecializationDefinitionInvalid, kind)
			}
			selectionMode = battleNormalizeTrigger(value)
			switch selectionMode {
			case "highestattack", "highestthreat", "lowesthp", "random":
			default:
				return fmt.Errorf("%w: specialization effect %s uses unsupported selectionMode %q", store.ErrSpecializationDefinitionInvalid, kind, value)
			}
		}
		if kind == "designateprioritytarget" && selectionMode == "" {
			return fmt.Errorf("%w: specialization effect designate_priority_target selectionMode is required", store.ErrSpecializationDefinitionInvalid)
		}
		targetMode := ""
		if raw, exists := effect["targetMode"]; exists {
			value, ok := raw.(string)
			if !ok {
				return fmt.Errorf("%w: specialization effect mark_target targetMode must be a string", store.ErrSpecializationDefinitionInvalid)
			}
			targetMode = battleNormalizeTrigger(value)
			switch targetMode {
			case "self", "source", "enemy", "markedenemy", "targeting", "lowesthpally":
			default:
				return fmt.Errorf("%w: specialization effect mark_target uses unsupported targetMode %q", store.ErrSpecializationDefinitionInvalid, value)
			}
		}
		if kind == "marktarget" && selectionMode == "" && targetMode == "" {
			return fmt.Errorf("%w: specialization effect mark_target requires selectionMode or targetMode", store.ErrSpecializationDefinitionInvalid)
		}
		if kind == "marktarget" && selectionMode != "" && targetMode != "" && targetMode != "enemy" && targetMode != "markedenemy" {
			return fmt.Errorf("%w: specialization effect mark_target selectionMode requires enemy or marked_enemy targetMode", store.ErrSpecializationDefinitionInvalid)
		}
	}
	if battleSpecializationHasValueFormula(effect) {
		switch kind {
		case "actionmetergain", "consumecounterfordamageamp", "flatdefensepenetration", "heal", "mirrorcastskill", "preventlethal", "redirectdamage", "repeatcontexteffect", "resourcedebtcast", "resourceshield", "restoremp", "runtimedamageamp", "shareddamagereduction":
		default:
			return fmt.Errorf("%w: specialization effect %s cannot use valueFormula", store.ErrSpecializationDefinitionInvalid, kind)
		}
		if err := validateBattleSpecializationValueFormula(effect); err != nil {
			return fmt.Errorf("%w: specialization effect %s: %v", store.ErrSpecializationDefinitionInvalid, kind, err)
		}
	}
	if kind == "cyclecounter" {
		atMax := normalizeAnyMap(firstNonNil(effect["atMax"], effect["at_max"]))
		if len(atMax) > 0 {
			nestedKind := battleNormalizeTrigger(fmt.Sprintf("%v", atMax["kind"]))
			var allowed []string
			switch nestedKind {
			case "empowernexthit":
				allowed = []string{"kind", "damageAmp", "consumeStacks", "valueFormula"}
			case "actionmetergain":
				allowed = []string{"kind", "value", "consumeStacks", "valueFormula"}
			case "extraturn", "immediateaction":
				allowed = []string{"kind", "targetMode", "turnMode", "dispatchHooks", "consumeStacks"}
			case "restoremp":
				allowed = []string{"kind", "mpRatio", "consumeStacks", "valueFormula"}
			case "restorempandtickcooldown":
				allowed = []string{"kind", "mpRatio", "cooldownAmount", "consumeStacks", "valueFormula"}
			case "guardlowesthpally":
				allowed = []string{"kind", "damageReduction", "durationTurns", "consumeStacks", "valueFormula"}
			default:
				return fmt.Errorf("%w: unsupported cycle counter terminal effect %q", store.ErrSpecializationDefinitionInvalid, nestedKind)
			}
			if err := validateBattleSpecializationCanonicalKeys(atMax, "specialization cycle counter terminal "+nestedKind, allowed...); err != nil {
				return err
			}
			if battleSpecializationHasValueFormula(atMax) {
				if err := validateBattleSpecializationValueFormula(atMax); err != nil {
					return fmt.Errorf("%w: specialization cycle counter terminal %s: %v", store.ErrSpecializationDefinitionInvalid, nestedKind, err)
				}
			}
			if nestedKind == "extraturn" || nestedKind == "immediateaction" {
				maxStacks := toFloat64(firstNonNil(effect["maxStacks"], effect["max_stacks"]), math.NaN())
				consumeStacks := toFloat64(firstNonNil(atMax["consumeStacks"], atMax["consume_stacks"]), math.NaN())
				if math.IsNaN(maxStacks) || math.IsInf(maxStacks, 0) || maxStacks < 3 || maxStacks != math.Trunc(maxStacks) ||
					math.IsNaN(consumeStacks) || math.IsInf(consumeStacks, 0) || consumeStacks < 3 || consumeStacks > maxStacks || consumeStacks != math.Trunc(consumeStacks) {
					return fmt.Errorf("%w: cycle counter extra_turn requires integer maxStacks and consumeStacks between 3 and maxStacks", store.ErrSpecializationDefinitionInvalid)
				}
				if err := validateBattleSpecializationTargetMode(atMax, "extra_turn", true, "self"); err != nil {
					return err
				}
				turnMode, ok := atMax["turnMode"].(string)
				if !ok || battleNormalizeTrigger(turnMode) != "full" {
					return fmt.Errorf("%w: cycle counter extra_turn turnMode must be full", store.ErrSpecializationDefinitionInvalid)
				}
				if _, ok := atMax["dispatchHooks"].(bool); !ok {
					return fmt.Errorf("%w: cycle counter extra_turn dispatchHooks must be boolean", store.ErrSpecializationDefinitionInvalid)
				}
			}
		}
	}
	if kind == "markcycle" {
		markedHit := normalizeAnyMap(firstNonNil(effect["markedHit"], effect["marked_hit"]))
		if battleNormalizeTrigger(fmt.Sprintf("%v", markedHit["kind"])) != "runtimedamageamp" {
			return fmt.Errorf("%w: mark_cycle requires a runtime_damage_amp markedHit", store.ErrSpecializationDefinitionInvalid)
		}
		if err := validateBattleSpecializationCanonicalKeys(markedHit, "specialization mark_cycle markedHit", "kind", "value", "consumeMark", "valueFormula"); err != nil {
			return err
		}
		if battleSpecializationHasValueFormula(markedHit) {
			if err := validateBattleSpecializationValueFormula(markedHit); err != nil {
				return fmt.Errorf("%w: specialization mark_cycle markedHit: %v", store.ErrSpecializationDefinitionInvalid, err)
			}
		}
	}
	if kind == "buffcycle" {
		buffID, ok := effect["buffId"].(string)
		if !ok || strings.TrimSpace(buffID) == "" {
			return fmt.Errorf("%w: buff_cycle buffId is required", store.ErrSpecializationDefinitionInvalid)
		}
		if battleNormalizeTrigger(fmt.Sprintf("%v", effect["firstHit"])) != "applybuff" {
			return fmt.Errorf("%w: buff_cycle firstHit must be apply_buff", store.ErrSpecializationDefinitionInvalid)
		}
		buffedHit := normalizeAnyMap(effect["buffedHit"])
		if battleNormalizeTrigger(fmt.Sprintf("%v", buffedHit["kind"])) != "runtimedamageamp" {
			return fmt.Errorf("%w: buff_cycle requires a runtime_damage_amp buffedHit", store.ErrSpecializationDefinitionInvalid)
		}
		if err := validateBattleSpecializationCanonicalKeys(buffedHit, "specialization buff_cycle buffedHit", "kind", "value", "consumeBuff", "valueFormula"); err != nil {
			return err
		}
		if _, ok := buffedHit["consumeBuff"].(bool); !ok {
			return fmt.Errorf("%w: specialization buff_cycle buffedHit.consumeBuff must be boolean", store.ErrSpecializationDefinitionInvalid)
		}
		if battleSpecializationHasValueFormula(buffedHit) {
			if err := validateBattleSpecializationValueFormula(buffedHit); err != nil {
				return fmt.Errorf("%w: specialization buff_cycle buffedHit: %v", store.ErrSpecializationDefinitionInvalid, err)
			}
		}
	}
	if kind == "designateprioritytarget" {
		rewrite := normalizeAnyMap(effect["rewriteTargeting"])
		if len(rewrite) == 0 {
			return fmt.Errorf("%w: designate_priority_target rewriteTargeting must be an object", store.ErrSpecializationDefinitionInvalid)
		}
		if err := validateBattleSpecializationCanonicalKeys(rewrite, "specialization designate_priority_target rewriteTargeting", "preferMarked", "fallback"); err != nil {
			return err
		}
	}
	return nil
}

func validateBattleSpecializationV5ExtraTurnSafety(trigger string, effects []map[string]any, maxPerWave, maxPerBattle int) error {
	trigger = battleNormalizeTrigger(trigger)
	for _, effect := range effects {
		kind := battleNormalizeTrigger(fmt.Sprintf("%v", firstNonNil(effect["kind"], effect["type"])))
		if kind != "extraturn" && kind != "immediateaction" {
			continue
		}
		if (maxPerWave > 0 && maxPerWave <= 1) || (maxPerBattle > 0 && maxPerBattle <= 1) {
			continue
		}
		return fmt.Errorf("%w: direct extra_turn on %s requires perWave=1 or perBattle=1", store.ErrSpecializationDefinitionInvalid, trigger)
	}
	return nil
}

func validateBattleSpecializationV5RiskPolicy(conditions, effects []map[string]any, maxPerTurn, maxPerWave, maxPerBattle int) error {
	hasWeaponGate := false
	hasNonBasicGate := false
	maxMPGate := 0.0
	for _, condition := range conditions {
		switch battleNormalizeTrigger(fmt.Sprintf("%v", condition["kind"])) {
		case "selfweaponclass":
			hasWeaponGate = len(compactStringSlice(condition["weaponClasses"])) > 0
		case "contextskillnonbasic":
			hasNonBasicGate = true
		case "selfmpratiogte":
			maxMPGate = maxFloat(maxMPGate, toFloat64(condition["value"], 0))
		}
	}
	hasOneShotLimit := maxPerWave == 1 || maxPerBattle == 1
	hasAnyLimit := maxPerTurn > 0 || maxPerWave > 0 || maxPerBattle > 0
	for _, effect := range effects {
		kind := battleNormalizeTrigger(fmt.Sprintf("%v", firstNonNil(effect["kind"], effect["type"])))
		if kind == "adjustskillcooldown" && toFloat64(effect["amount"], 0) < 0 {
			return fmt.Errorf("%w: runtime cooldown reduction is forbidden for specialization hooks", store.ErrSpecializationDefinitionInvalid)
		}
		if kind == "cyclecounter" {
			atMax := normalizeAnyMap(firstNonNil(effect["atMax"], effect["at_max"]))
			nestedKind := battleNormalizeTrigger(fmt.Sprintf("%v", firstNonNil(atMax["kind"], atMax["type"])))
			if nestedKind == "restorempandtickcooldown" && toFloat64(firstNonNil(atMax["cooldownAmount"], atMax["cooldown_amount"]), 0) < 0 {
				return fmt.Errorf("%w: counter payoff cooldown reduction is forbidden", store.ErrSpecializationDefinitionInvalid)
			}
			if nestedKind == "extraturn" || nestedKind == "immediateaction" {
				return fmt.Errorf("%w: nested counter extra turns are forbidden", store.ErrSpecializationDefinitionInvalid)
			}
		}
		if kind == "extraturn" || kind == "immediateaction" {
			baseChance := toFloat64(effect["procChanceBase"], math.NaN())
			maxChance := toFloat64(effect["procChanceMax"], math.NaN())
			if !hasWeaponGate || !hasOneShotLimit {
				return fmt.Errorf("%w: extra_turn requires a weapon gate and perWave=1 or perBattle=1", store.ErrSpecializationDefinitionInvalid)
			}
			if math.IsNaN(baseChance) || math.IsNaN(maxChance) || baseChance <= 0 || maxChance <= 0 || baseChance > maxChance || maxChance > 0.4 {
				return fmt.Errorf("%w: extra_turn probability must be explicit and capped at 40%%", store.ErrSpecializationDefinitionInvalid)
			}
			if effect["abortHookOnFail"] != true || effect["dispatchHooks"] != false {
				return fmt.Errorf("%w: extra_turn must abort on failure and disable derived hook dispatch", store.ErrSpecializationDefinitionInvalid)
			}
		}
		if kind == "mirrorcastskill" {
			if !hasWeaponGate || !hasOneShotLimit || !hasNonBasicGate || maxMPGate < 0.75 {
				return fmt.Errorf("%w: mirror_cast_skill requires weapon, non-basic, 75%% MP, and one-shot gates", store.ErrSpecializationDefinitionInvalid)
			}
			if effect["allowMirroredCastTrigger"] != false || effect["dispatchHooks"] != false || battleSpecializationHasValueFormula(effect) || toFloat64(effect["amountRatio"], 0) > 0.55 {
				return fmt.Errorf("%w: mirror_cast_skill must be non-recursive, fixed-ratio, and capped at 55%%", store.ErrSpecializationDefinitionInvalid)
			}
		}
		if kind == "repeatcontexteffect" {
			ratio := toFloat64(effect["amountRatio"], 0)
			if !hasWeaponGate || !hasAnyLimit {
				return fmt.Errorf("%w: repeat_context_effect requires a weapon gate and explicit limit", store.ErrSpecializationDefinitionInvalid)
			}
			if effect["dispatchHooks"] != false || battleSpecializationHasValueFormula(effect) || ratio <= 0 || ratio > 0.45 {
				return fmt.Errorf("%w: repeat_context_effect must be non-recursive, fixed-ratio, and capped at 45%%", store.ErrSpecializationDefinitionInvalid)
			}
		}
	}
	return nil
}

func battleSpecializationValueFormulaRaw(effect map[string]any) (any, bool) {
	if effect == nil {
		return nil, false
	}
	if raw, exists := effect["valueFormula"]; exists && raw != nil {
		return raw, true
	}
	if raw, exists := effect["value_formula"]; exists && raw != nil {
		return raw, true
	}
	return nil, false
}

func battleSpecializationHasValueFormula(effect map[string]any) bool {
	_, exists := battleSpecializationValueFormulaRaw(effect)
	return exists
}

func validateBattleSpecializationValueFormula(effect map[string]any) error {
	raw, exists := battleSpecializationValueFormulaRaw(effect)
	if !exists {
		return nil
	}
	node := normalizeAnyMap(raw)
	if len(node) == 0 {
		return errors.New("valueFormula must be an object")
	}
	for key := range node {
		if key != "baseAdd" && key != "terms" {
			return fmt.Errorf("valueFormula has unsupported field %q", key)
		}
	}
	baseRaw, hasBase := node["baseAdd"]
	if !hasBase {
		return errors.New("valueFormula.baseAdd is required")
	}
	base := toFloat64(baseRaw, math.NaN())
	if math.IsNaN(base) || math.IsInf(base, 0) {
		return errors.New("valueFormula.baseAdd must be finite")
	}
	termsRaw, hasTerms := node["terms"]
	if !hasTerms {
		return errors.New("valueFormula.terms is required")
	}
	items, ok := termsRaw.([]any)
	if !ok {
		if typed, typedOK := termsRaw.([]map[string]any); typedOK {
			items = make([]any, 0, len(typed))
			for _, item := range typed {
				items = append(items, item)
			}
		} else {
			return errors.New("valueFormula.terms must be an array")
		}
	}
	if len(items) == 0 {
		return errors.New("valueFormula.terms must not be empty")
	}
	allowedAttributes := map[string]struct{}{
		"strength": {}, "dexterity": {}, "intelligence": {}, "perception": {}, "stamina": {},
	}
	seenAttributes := map[string]struct{}{}
	for index, rawTerm := range items {
		term := normalizeAnyMap(rawTerm)
		for key := range term {
			if key != "attributeKey" && key != "coefficient" {
				return fmt.Errorf("valueFormula.terms[%d] has unsupported field %q", index, key)
			}
		}
		attributeKey := strings.TrimSpace(fmt.Sprintf("%v", term["attributeKey"]))
		if len(term) == 0 || attributeKey == "" || attributeKey == "<nil>" {
			return fmt.Errorf("valueFormula.terms[%d].attributeKey is required", index)
		}
		if _, allowed := allowedAttributes[attributeKey]; !allowed {
			return fmt.Errorf("valueFormula.terms[%d].attributeKey %q is not a specialization attribute", index, attributeKey)
		}
		if _, duplicate := seenAttributes[attributeKey]; duplicate {
			return fmt.Errorf("valueFormula.terms[%d] repeats attributeKey %q", index, attributeKey)
		}
		seenAttributes[attributeKey] = struct{}{}
		coefficient := toFloat64(term["coefficient"], math.NaN())
		if math.IsNaN(coefficient) || math.IsInf(coefficient, 0) || coefficient == 0 {
			return fmt.Errorf("valueFormula.terms[%d].coefficient must be finite and non-zero", index)
		}
	}
	return nil
}

func battleSpecializationEffectScalar(actor *battleUnitState, effect map[string]any, fallbackKeys ...string) float64 {
	if raw, exists := battleSpecializationValueFormulaRaw(effect); exists {
		return battleEvaluateAmountFormulaWithEnhance(actor, 1, parseBattleSkillAmountFormula(raw, nil), 0)
	}
	for _, key := range fallbackKeys {
		if raw, exists := effect[key]; exists && raw != nil {
			return toFloat64(raw, 0)
		}
	}
	return 0
}

func validateBattleSpecializationHookRule(hook battleEquipmentHookRule) error {
	if !battleSpecializationTriggerSupported(hook.Trigger) {
		return fmt.Errorf("%w: unsupported carried specialization trigger %q", store.ErrSpecializationDefinitionInvalid, hook.Trigger)
	}
	for _, condition := range hook.Conditions {
		kind := battleNormalizeTrigger(fmt.Sprintf("%v", condition["kind"]))
		// trigger_limit must be compiled into the rule's explicit counters before
		// the snapshot is persisted. Treating it as a runtime predicate would be
		// permissive and silently remove its cap.
		if kind == "triggerlimit" {
			return fmt.Errorf("%w: unsupported carried specialization condition %q", store.ErrSpecializationDefinitionInvalid, kind)
		}
		if err := validateBattleSpecializationCondition(condition); err != nil {
			return err
		}
		if err := validateBattleSpecializationConditionTrigger(hook.Trigger, condition); err != nil {
			return err
		}
	}
	if len(hook.Effects) == 0 {
		return fmt.Errorf("%w: carried specialization hook has no effects", store.ErrSpecializationDefinitionInvalid)
	}
	if err := validateBattleSpecializationEventFilter(hook.EventFilter); err != nil {
		return err
	}
	if err := validateBattleSpecializationEventFilterForTrigger(hook.Trigger, hook.EventFilter); err != nil {
		return err
	}
	if err := validateBattleSpecializationSubjectFilter(hook.SubjectFilter); err != nil {
		return err
	}
	for _, effect := range hook.Effects {
		if err := validateBattleSpecializationHookEffect(effect); err != nil {
			return err
		}
		if err := validateBattleSpecializationEffectTrigger(hook.Trigger, effect); err != nil {
			return err
		}
	}
	return nil
}

func battleSpecializationSourceMeta(hook battleEquipmentHookRule, current map[string]any) map[string]any {
	meta := cloneAnyMap(current)
	if meta == nil {
		meta = map[string]any{}
	}
	meta["sourceKind"] = "specialization"
	meta["sourceId"] = strings.TrimSpace(hook.SourceID)
	meta["nodeId"] = strings.TrimSpace(hook.SourceID)
	meta["sourceNameKey"] = strings.TrimSpace(hook.SourceNameKey)
	meta["hookId"] = strings.TrimSpace(hook.ID)
	return meta
}

func enrichBattleEquipmentInstanceAction(action map[string]any, hook battleEquipmentHookRule) {
	if action == nil || hook.SourceKind != "equipment" || hook.SourceInstanceID <= 0 {
		return
	}
	meta := cloneAnyMap(normalizeAnyMap(action["meta"]))
	if meta == nil {
		meta = map[string]any{}
	}
	meta["sourceKind"] = "equipment"
	meta["sourceItemId"] = strings.TrimSpace(hook.SourceItemID)
	meta["sourceInstanceId"] = hook.SourceInstanceID
	meta["hookId"] = strings.TrimSpace(hook.ID)
	if definitionID := strings.TrimSpace(hook.DefinitionID); definitionID != "" {
		meta["hookDefinitionId"] = definitionID
	}
	action["meta"] = meta
}

func enrichBattleSpecializationAction(row map[string]any, hook battleEquipmentHookRule) {
	if row == nil || hook.SourceKind != "specialization" {
		return
	}
	meta := cloneAnyMap(normalizeAnyMap(row["meta"]))
	if meta == nil {
		meta = map[string]any{}
	}
	hasNestedSource := false
	for _, key := range []string{"sourceKind", "nodeId", "sourceNameKey"} {
		value := strings.TrimSpace(fmt.Sprintf("%v", meta[key]))
		if value != "" && value != "<nil>" {
			hasNestedSource = true
			break
		}
	}
	if !hasNestedSource {
		meta = battleSpecializationSourceMeta(hook, meta)
	}
	if _, exists := row["sourceKind"]; !exists {
		row["sourceKind"] = firstNonNil(meta["sourceKind"], "specialization")
	}
	if _, exists := row["sourceNameKey"]; !exists {
		row["sourceNameKey"] = firstNonNil(meta["sourceNameKey"], strings.TrimSpace(hook.SourceNameKey))
	}
	if _, exists := row["nodeId"]; !exists {
		row["nodeId"] = firstNonNil(meta["nodeId"], strings.TrimSpace(hook.SourceID))
	}
	row["meta"] = meta
}

func battleBeginSpecializationDispatchScope(state *battleRuntimeState) func() {
	if state == nil {
		return func() {}
	}
	if state.specializationDispatchScopes == 0 {
		state.specializationDispatchDepth = 0
		state.specializationDispatchEvents = 0
		state.specializationDispatchAncestry = map[string]int{}
	}
	state.specializationDispatchScopes++
	return func() {
		state.specializationDispatchScopes--
		if state.specializationDispatchScopes < 0 {
			state.specializationDispatchScopes = 0
		}
	}
}

func battleSuppressDerivedHookDispatch(state *battleRuntimeState, suppress bool) func() {
	if state == nil || !suppress {
		return func() {}
	}
	state.derivedHookDispatchSuppression++
	return func() {
		state.derivedHookDispatchSuppression--
		if state.derivedHookDispatchSuppression < 0 {
			state.derivedHookDispatchSuppression = 0
		}
	}
}

func battleEnterSpecializationContribution(state *battleRuntimeState, hook battleEquipmentHookRule) (func(), string) {
	if hook.SourceKind != "specialization" {
		return func() {}, ""
	}
	if state == nil {
		return nil, "missing_runtime_state"
	}
	key := strings.TrimSpace(hook.ID)
	if key == "" {
		key = strings.TrimSpace(hook.SourceID)
	}
	if key == "" {
		return nil, "missing_contribution_id"
	}
	if state.specializationDispatchAncestry == nil {
		state.specializationDispatchAncestry = map[string]int{}
	}
	if state.specializationDispatchAncestry[key] > 0 {
		return nil, "ancestor_contribution"
	}
	if state.specializationDispatchDepth >= battleMaxSpecializationDispatchDepth {
		return nil, "depth_budget"
	}
	if state.specializationDispatchEvents >= battleMaxSpecializationDispatchEvents {
		return nil, "event_budget"
	}
	state.specializationDispatchAncestry[key]++
	state.specializationDispatchDepth++
	state.specializationDispatchEvents++
	return func() {
		state.specializationDispatchAncestry[key]--
		if state.specializationDispatchAncestry[key] <= 0 {
			delete(state.specializationDispatchAncestry, key)
		}
		state.specializationDispatchDepth--
		if state.specializationDispatchDepth < 0 {
			state.specializationDispatchDepth = 0
		}
	}, ""
}

func battleSpecializationDispatchBlockedAction(self *battleUnitState, trigger string, hook battleEquipmentHookRule, blockedBy string) map[string]any {
	unitID := ""
	if self != nil {
		unitID = strings.TrimSpace(self.UnitID)
	}
	return map[string]any{
		"kind": "specialization_dispatch_blocked", "targetId": unitID, "sourceId": unitID,
		"reason": trigger, "delayMs": int64(0),
		"meta": battleSpecializationSourceMeta(hook, map[string]any{"specialization": true, "blockedBy": blockedBy}),
	}
}

type battleSpecializationGeneratedEffect struct {
	ProducerMutationID string
	Kind               string
}

func (s *Server) battleSpecializationCatalogEffectKindsByID() map[string]string {
	out := map[string]string{}
	register := func(effect map[string]any) {
		effectID := strings.ToLower(strings.TrimSpace(fmt.Sprintf("%v", firstNonNil(effect["id"], effect["effectId"]))))
		kind := strings.TrimSpace(fmt.Sprintf("%v", effect["kind"]))
		if effectID != "" && effectID != "<nil>" && kind != "" && kind != "<nil>" {
			out[effectID] = kind
		}
	}
	if s == nil {
		return out
	}
	for _, skill := range s.deps.GameData.Skills {
		for _, effect := range battleAnyMapSlice(skill["effects"]) {
			register(effect)
		}
	}
	for _, equipment := range s.deps.GameData.EquipmentTemplates {
		for _, effect := range equipment.BasicAttackEffects {
			register(effect)
		}
	}
	return out
}

func (s *Server) battleSpecializationEquipmentEffectIDs() map[string]struct{} {
	out := map[string]struct{}{}
	if s == nil {
		return out
	}
	for _, equipment := range s.deps.GameData.EquipmentTemplates {
		for _, effect := range equipment.BasicAttackEffects {
			effectID := strings.ToLower(strings.TrimSpace(fmt.Sprintf("%v", firstNonNil(effect["id"], effect["effectId"]))))
			if effectID != "" && effectID != "<nil>" {
				out[effectID] = struct{}{}
			}
		}
	}
	return out
}

func (s *Server) resolveBattleSpecializationEffectMutations(contribution store.SpecializationBattleContribution) ([]battleAffixEffectMutation, error) {
	out := make([]battleAffixEffectMutation, 0, len(contribution.Mutations))
	staticEffectKinds := s.battleSpecializationCatalogEffectKindsByID()
	equipmentEffectIDs := s.battleSpecializationEquipmentEffectIDs()
	generatedEffects := make(map[string]battleSpecializationGeneratedEffect)
	opsByMutationID := make(map[string]map[string]any, len(contribution.Mutations))
	for _, grant := range contribution.Mutations {
		runtimeID, err := battleSpecializationRuntimeGrantID(contribution.TreeID, contribution.RulesetVersion, grant)
		if err != nil {
			return nil, err
		}
		if _, duplicate := opsByMutationID[runtimeID]; duplicate {
			return nil, fmt.Errorf("%w: duplicate specialization mutation contribution id %s", store.ErrSpecializationDefinitionInvalid, runtimeID)
		}
		ops := normalizeAnyMap(grant.Payload["ops"])
		opsByMutationID[runtimeID] = ops
		for _, raw := range battleAnyMapSlice(ops["appendEffects"]) {
			effectID := strings.TrimSpace(fmt.Sprintf("%v", raw["id"]))
			normalizedEffectID := strings.ToLower(effectID)
			if effectID == "" || effectID == "<nil>" {
				return nil, fmt.Errorf("%w: specialization grant %s appends an effect without a stable id", store.ErrSpecializationDefinitionInvalid, grant.ID)
			}
			if _, exists := staticEffectKinds[normalizedEffectID]; exists {
				return nil, fmt.Errorf("%w: specialization grant %s appended effect id %q collides with a catalog effect", store.ErrSpecializationDefinitionInvalid, grant.ID, effectID)
			}
			if previous, exists := generatedEffects[normalizedEffectID]; exists {
				return nil, fmt.Errorf(
					"%w: specialization grants %s and %s repeat appended effect id %q",
					store.ErrSpecializationDefinitionInvalid,
					previous.ProducerMutationID,
					runtimeID,
					effectID,
				)
			}
			generatedEffects[normalizedEffectID] = battleSpecializationGeneratedEffect{
				ProducerMutationID: runtimeID,
				Kind:               strings.TrimSpace(fmt.Sprintf("%v", raw["kind"])),
			}
		}
	}
	for _, grant := range contribution.Mutations {
		runtimeID, err := battleSpecializationRuntimeGrantID(contribution.TreeID, contribution.RulesetVersion, grant)
		if err != nil {
			return nil, err
		}
		mutation, err := parseBattleSpecializationEffectMutation(grant.Payload)
		if err != nil {
			return nil, fmt.Errorf("specialization grant %s: %w", grant.ID, err)
		}
		mutation.SourceKind = "specialization"
		mutation.SourceID = grant.SourceNodeID
		mutation.SourceNameKey = grant.SourceNameKey
		mutation.ID = runtimeID
		for effectIndex := range mutation.AppendEffects {
			mutation.AppendEffects[effectIndex].GeneratedBy = mutation.ID
		}
		mutation.Priority = int(toInt64Loose(grant.Payload["priority"], 0))
		out = append(out, mutation)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Priority != out[j].Priority {
			return out[i].Priority > out[j].Priority
		}
		return out[i].ID < out[j].ID
	})
	mutationIndex := make(map[string]int, len(out))
	for index := range out {
		mutationIndex[out[index].ID] = index
	}
	for index := range out {
		mutation := out[index]
		if len(mutation.EffectIDList) == 0 {
			continue
		}
		selectedKinds := make([]string, 0, len(mutation.EffectIDList))
		ops := opsByMutationID[mutation.ID]
		hasSkillScalarOps := ops["cooldownDelta"] != nil || ops["mpCostDelta"] != nil
		for _, effectID := range mutation.EffectIDList {
			normalizedEffectID := strings.ToLower(strings.TrimSpace(effectID))
			if generated, exists := generatedEffects[normalizedEffectID]; exists {
				if hasSkillScalarOps {
					return nil, fmt.Errorf("%w: mutation %s cannot use generated effect %q to select skill-level cooldown/mp-cost operations", store.ErrSpecializationDefinitionInvalid, mutation.ID, effectID)
				}
				if !mutation.IncludeGeneratedEffects {
					return nil, fmt.Errorf("%w: mutation %s selects generated effect %q without includeGenerated", store.ErrSpecializationDefinitionInvalid, mutation.ID, effectID)
				}
				producerIndex, producerActive := mutationIndex[generated.ProducerMutationID]
				if !producerActive || producerIndex >= index {
					return nil, fmt.Errorf("%w: mutation %s selects generated effect %q before its producer %s executes", store.ErrSpecializationDefinitionInvalid, mutation.ID, effectID, generated.ProducerMutationID)
				}
				switch battleNormalizeTrigger(generated.Kind) {
				case "splashdamage":
					selectedKinds = append(selectedKinds, "damage")
				case "marktarget":
					selectedKinds = append(selectedKinds, "status")
				default:
					return nil, fmt.Errorf("%w: mutation %s selects generated effect %q whose kind %q is not mutable", store.ErrSpecializationDefinitionInvalid, mutation.ID, effectID, generated.Kind)
				}
				continue
			}
			kind, exists := staticEffectKinds[normalizedEffectID]
			if !exists {
				return nil, fmt.Errorf("%w: mutation %s selects unknown or inactive effect id %q", store.ErrSpecializationDefinitionInvalid, mutation.ID, effectID)
			}
			if _, equipmentEffect := equipmentEffectIDs[normalizedEffectID]; equipmentEffect && hasSkillScalarOps {
				return nil, fmt.Errorf("%w: mutation %s cannot use equipment basic-attack effect %q to select skill-level cooldown/mp-cost operations", store.ErrSpecializationDefinitionInvalid, mutation.ID, effectID)
			}
			selectedKinds = append(selectedKinds, kind)
		}
		compatibilityMutation := mutation
		compatibilityMutation.EffectKindList = selectedKinds
		if err := validateBattleSpecializationMutationKindCompatibility(mutation.ID, compatibilityMutation, ops); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (s *Server) resolveBattlePersonalityHooks(profiles []battlePersonalityProfile) []battleEquipmentHookRule {
	if len(profiles) == 0 {
		return nil
	}
	out := make([]battleEquipmentHookRule, 0, len(profiles))
	for _, profile := range profiles {
		for idx, hook := range profile.BattleHooks {
			if hook == nil {
				continue
			}
			trigger := battleNormalizeTrigger(fmt.Sprintf("%v", hook["trigger"]))
			if trigger == "" {
				continue
			}
			effects := battleAnyMapSlice(hook["effects"])
			if len(effects) == 0 {
				continue
			}
			hookID := strings.TrimSpace(fmt.Sprintf("%v", hook["id"]))
			if hookID == "<nil>" {
				hookID = ""
			}
			if hookID == "" {
				hookID = fmt.Sprintf("personality:%s:battle_hook:%d", strings.TrimSpace(profile.ID), idx+1)
			}
			out = append(out, battleEquipmentHookRule{
				ID:                     hookID,
				SourceItemID:           fmt.Sprintf("personality:%s", strings.TrimSpace(profile.ID)),
				Trigger:                trigger,
				Priority:               int(toInt64Loose(hook["priority"], 0)),
				MaxTriggersPerTurn:     maxInt(0, int(toInt64Loose(firstNonNil(hook["maxTriggersPerTurn"], hook["max_triggers_per_turn"]), 0))),
				MaxTriggersPerWave:     maxInt(0, int(toInt64Loose(firstNonNil(hook["maxTriggersPerWave"], hook["max_triggers_per_wave"]), 0))),
				MaxTriggersPerBattle:   maxInt(0, int(toInt64Loose(firstNonNil(hook["maxTriggersPerBattle"], hook["max_triggers_per_battle"]), 0))),
				SharedTriggerGroup:     strings.TrimSpace(fmt.Sprintf("%v", firstNonNil(hook["sharedTriggerGroup"], hook["shared_trigger_group"]))),
				ActivationCost:         parseBattleHookActivationCost(firstNonNil(hook["activationCost"], hook["activation_cost"])),
				CooldownCompletedTurns: maxInt(0, int(toInt64Loose(firstNonNil(hook["cooldownCompletedTurns"], hook["cooldown_completed_turns"]), 0))),
				SubjectFilter:          normalizeAnyMap(firstNonNil(hook["subjectFilter"], hook["subject_filter"])),
				EventFilter:            normalizeAnyMap(firstNonNil(hook["eventFilter"], hook["event_filter"])),
				Conditions:             battleAnyMapSlice(hook["conditions"]),
				Effects:                effects,
			})
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func (s *Server) resolveBattlePersonalityEffectMutations(profiles []battlePersonalityProfile) []battleAffixEffectMutation {
	if len(profiles) == 0 {
		return nil
	}
	out := make([]battleAffixEffectMutation, 0, len(profiles))
	for _, profile := range profiles {
		for _, node := range profile.EffectMutations {
			if mutation, ok := parseBattleAffixEffectMutation(node); ok {
				out = append(out, mutation)
			}
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func cloneNestedAnyMapSlice(rows []map[string]any) []map[string]any {
	if len(rows) == 0 {
		return nil
	}
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		if row == nil {
			continue
		}
		out = append(out, cloneNestedAnyMap(row))
	}
	return out
}

func cloneNestedAnyMap(raw map[string]any) map[string]any {
	if raw == nil {
		return nil
	}
	out := make(map[string]any, len(raw))
	for key, value := range raw {
		out[key] = cloneNestedAnyValueForBattle(value)
	}
	return out
}

func cloneNestedAnyValueForBattle(raw any) any {
	switch typed := raw.(type) {
	case map[string]any:
		return cloneNestedAnyMap(typed)
	case []any:
		out := make([]any, 0, len(typed))
		for _, value := range typed {
			out = append(out, cloneNestedAnyValueForBattle(value))
		}
		return out
	case []map[string]any:
		out := make([]any, 0, len(typed))
		for _, value := range typed {
			out = append(out, cloneNestedAnyMap(value))
		}
		return out
	default:
		return raw
	}
}

func (s *Server) battleApplyUnitEquipmentRuntime(unit *battleUnitState) {
	if unit == nil {
		return
	}
	s.battleRecomputeUnitStats(unit)
}

func (s *Server) battleSkillByID(skillID string) battleSkillState {
	return s.battleSkillByIDWithProgress(skillID, 1, 0)
}

func (s *Server) battleSkillByIDWithProgress(skillID string, level int, exp int64) battleSkillState {
	return s.battleSkillByRawWithProgress(skillID, level, exp, nil)
}

func (s *Server) battleSkillByRawWithProgress(skillID string, level int, exp int64, rawOverride map[string]any) battleSkillState {
	skillID = strings.TrimSpace(skillID)
	if skillID == "" {
		skillID = s.battleDefaultBasicSkillID()
	}
	out := battleSkillState{
		ID:             skillID,
		Name:           skillID,
		Level:          maxInt(1, level),
		Exp:            maxInt64(0, exp),
		HPCost:         0,
		MPCost:         0,
		CooldownTurns:  0,
		CooldownRemain: 0,
		Targeting: battleSkillTargeting{
			TargetPositions: []string{"vanguard"},
			SecondarySeek:   "front_to_rear",
			TargetScope:     "enemy",
			SelectRule:      "random",
			TargetCount: battleSkillTargetCount{
				Base: 1,
			},
		},
		Effects: []battleSkillEffect{
			{
				Kind:       "damage",
				DamageType: "physical",
				Targeting: battleSkillTargeting{
					TargetPositions: []string{"vanguard"},
					SecondarySeek:   "front_to_rear",
					TargetScope:     "enemy",
					SelectRule:      "random",
					TargetCount: battleSkillTargetCount{
						Base: 1,
					},
				},
				AmountFormula: battleSkillAmountFormula{
					BaseAdd: 0,
					Terms: []battleAttackTerm{
						{AttrKey: "physics_damage", Coefficient: 1},
					},
				},
				DamageSpec: battleDamageSpec{
					DamageType: "physical",
					AmountFormula: battleSkillAmountFormula{
						BaseAdd: 0,
						Terms: []battleAttackTerm{
							{AttrKey: "physics_damage", Coefficient: 1},
						},
					},
					HitCount: battleSkillTargetCount{Base: 1},
				},
			},
		},
	}
	candidates := s.deps.GameData.Skills
	if rawOverride != nil {
		candidates = []map[string]any{rawOverride}
	}
	for _, raw := range candidates {
		id, _ := stringFromAny(raw["id"])
		if strings.TrimSpace(id) != skillID {
			continue
		}
		name, _ := stringFromAny(raw["name"])
		if name != "" {
			out.Name = name
		}
		category, _ := stringFromAny(raw["category"])
		out.Category = strings.TrimSpace(category)
		icon, _ := stringFromAny(raw["icon"])
		out.Icon = strings.TrimSpace(icon)
		out.Uninterruptible = parseBooleanLoose(raw["uninterruptible"], false)
		if strings.HasPrefix(strings.ToLower(skillID), "custom:") {
			customForge := normalizeAnyMap(raw["customForge"])
			out.RulesetVersion = int(toInt64Loose(customForge["rulesetVersion"], 1))
		}
		out.MPCost = maxFloat(0, float64(toInt64Loose(raw["mpCost"], 0)))
		out.HPCost = maxFloat(0, float64(toInt64Loose(raw["hpCost"], 0)))
		rawCD := toInt64Loose(raw["cooldown"], 0)
		if rawCD > 0 {
			out.CooldownTurns = int(rawCD)
		}
		out.Targeting = parseBattleSkillTargeting(raw["targeting"], "vanguard", "front_to_rear", 1, nil, 0, false, nil)
		out.Effects = parseBattleSkillEffects(raw["effects"], out.Targeting, "physical")
		out.CastGuards = parseBattleSkillCastGuards(raw["castGuards"])
		if len(out.Effects) > 0 {
			out.Targeting = out.Effects[0].Targeting
		}
		break
	}
	if strings.HasPrefix(strings.ToLower(skillID), "custom:") {
		s.sanitizeCustomBattleSkillState(&out)
	}
	if len(out.Effects) == 0 {
		out.Effects = []battleSkillEffect{
			{
				Kind:          "damage",
				DamageType:    "physical",
				Targeting:     out.Targeting,
				AmountFormula: battleSkillAmountFormula{BaseAdd: 0, Terms: []battleAttackTerm{{AttrKey: "physics_damage", Coefficient: 1}}},
				DamageSpec: battleDamageSpec{
					DamageType:    "physical",
					AmountFormula: battleSkillAmountFormula{BaseAdd: 0, Terms: []battleAttackTerm{{AttrKey: "physics_damage", Coefficient: 1}}},
					HitCount:      battleSkillTargetCount{Base: 1},
				},
			},
		}
	}
	return out
}

func (s *Server) sanitizeCustomBattleSkillState(skill *battleSkillState) {
	if skill == nil {
		return
	}
	settings := s.deps.Config.GlobalSettings.SkillForge
	maxEffects := settings.MaxEffects
	if maxEffects <= 0 {
		maxEffects = 3
	}
	maxTerms := settings.MaxFormulaTermsPerEffect
	if maxTerms <= 0 {
		maxTerms = 4
	}
	maxTargets := settings.MaxTargetCount
	if maxTargets <= 0 {
		maxTargets = 8
	}
	maxHits := settings.MaxHitCount
	if maxHits <= 0 {
		maxHits = 8
	}
	minCooldown := settings.MinCooldown
	if minCooldown < 1 {
		minCooldown = 1
	}
	maxCooldown := settings.MaxCooldown
	if maxCooldown < minCooldown {
		maxCooldown = 12
	}
	skill.CooldownTurns = minInt(maxCooldown, maxInt(minCooldown, skill.CooldownTurns))
	if len(skill.Effects) > maxEffects {
		skill.Effects = skill.Effects[:maxEffects]
	}
	sanitizeCustomBattleTargeting(&skill.Targeting, maxTargets, maxTerms)
	if len(skill.CastGuards) > maxEffects*2 {
		skill.CastGuards = skill.CastGuards[:maxEffects*2]
	}
	for idx := range skill.Effects {
		effect := &skill.Effects[idx]
		sanitizeCustomBattleTargeting(&effect.Targeting, maxTargets, maxTerms)
		sanitizeCustomBattleFormula(&effect.AmountFormula, maxTerms)
		sanitizeCustomBattleFormula(&effect.MpCostFormula, maxTerms)
		sanitizeCustomBattleFormula(&effect.DamageSpec.AmountFormula, maxTerms)
		sanitizeCustomBattleFormula(&effect.DamageSpec.FlatDefensePenetrationFormula, maxTerms)
		sanitizeCustomBattleFormula(&effect.DamageSpec.PercentDefensePenetrationFormula, maxTerms)
		sanitizeCustomBattleFormula(&effect.DamageSpec.PhysicalFlatDefensePenetrationFormula, maxTerms)
		sanitizeCustomBattleFormula(&effect.DamageSpec.PhysicalPercentDefensePenetrationFormula, maxTerms)
		sanitizeCustomBattleFormula(&effect.DamageSpec.MagicalFlatDefensePenetrationFormula, maxTerms)
		sanitizeCustomBattleFormula(&effect.DamageSpec.MagicalPercentDefensePenetrationFormula, maxTerms)
		sanitizeCustomBattleFormula(&effect.DamageSpec.ExtraDamageAmplificationFormula, maxTerms)
		effect.DamageSpec.BonusAgainstTargetTypes = sanitizeCustomBattleNumberMap(effect.DamageSpec.BonusAgainstTargetTypes, 8)
		sanitizeCustomBattleFormula(&effect.StatusPayload.DurationFormula, maxTerms)
		sanitizeCustomBattleFormula(&effect.StatusPayload.ApplyChanceFormula, maxTerms)
		effect.DamageSpec.HitCount.Base = minInt(maxHits, maxInt(1, effect.DamageSpec.HitCount.Base))
		effect.DamageSpec.HitCount.Max = maxHits
		effect.DamageSpec.HitCount.MaxSet = true
		sanitizeCustomBattleFormulaTerms(&effect.DamageSpec.HitCount.Formula, maxTerms)
		effect.StatusPayload.StatusIDs = sanitizeCustomBattleStrings(effect.StatusPayload.StatusIDs, 8)
		effect.StatusPayload.DurationBaseTurns = minInt(100, maxInt(0, effect.StatusPayload.DurationBaseTurns))
		effect.StatusPayload.DurationPerSkillLevelTurns = clampFloat(effect.StatusPayload.DurationPerSkillLevelTurns, 0, 100)
		effect.StatusPayload.ApplyChancePerSkillLevel = clampFloat(effect.StatusPayload.ApplyChancePerSkillLevel, -1, 1)
		effect.StatusPayload.ApplyChanceBase = sanitizeCustomBattleOptionalChance(effect.StatusPayload.ApplyChanceBase)
		effect.StatusPayload.ApplyChanceMin = sanitizeCustomBattleOptionalChance(effect.StatusPayload.ApplyChanceMin)
		effect.StatusPayload.ApplyChanceMax = sanitizeCustomBattleOptionalChance(effect.StatusPayload.ApplyChanceMax)
		effect.StatusPayload.Payload = nil
		effect.StatusPayload.PayloadFormula = nil
		effect.ConsumeStatusesOnHit = sanitizeCustomBattleStrings(effect.ConsumeStatusesOnHit, 8)
		effect.ConsumeCasterStatusesOnHit = sanitizeCustomBattleStrings(effect.ConsumeCasterStatusesOnHit, 8)
		effect.RequiredCasterStatuses = sanitizeCustomBattleStrings(effect.RequiredCasterStatuses, 8)
		effect.RequiredTargetStatuses = sanitizeCustomBattleStrings(effect.RequiredTargetStatuses, 8)
		effect.ForbiddenCasterStatuses = sanitizeCustomBattleStrings(effect.ForbiddenCasterStatuses, 8)
		effect.ForbiddenTargetStatuses = sanitizeCustomBattleStrings(effect.ForbiddenTargetStatuses, 8)
		effect.TargetSkillIDs = sanitizeCustomBattleStrings(effect.TargetSkillIDs, 8)
		effect.ExcludeSkillIDs = sanitizeCustomBattleStrings(effect.ExcludeSkillIDs, 8)
		effect.OnlyWhenNoTargetFromEffects = sanitizeCustomBattleEffectRefs(effect.OnlyWhenNoTargetFromEffects, idx)
		if effect.ReuseTargetsFromEffect < 0 || effect.ReuseTargetsFromEffect > idx {
			effect.ReuseTargetsFromEffect = 0
		}
		effect.CooldownAmount = minInt(maxCooldown, maxInt(-maxCooldown, effect.CooldownAmount))
	}
}

func sanitizeCustomBattleTargeting(targeting *battleSkillTargeting, maxTargets, maxTerms int) {
	if targeting == nil {
		return
	}
	normalizeBattleSkillTargeting(targeting)
	if len(targeting.TargetPositions) > 3 {
		targeting.TargetPositions = targeting.TargetPositions[:3]
	}
	targeting.TargetCount.Base = minInt(maxTargets, maxInt(1, targeting.TargetCount.Base))
	targeting.TargetCount.Max = maxTargets
	targeting.TargetCount.MaxSet = true
	sanitizeCustomBattleFormulaTerms(&targeting.TargetCount.Formula, maxTerms)
	if len(targeting.TargetPredicates) > 8 {
		targeting.TargetPredicates = targeting.TargetPredicates[:8]
	}
}

func sanitizeCustomBattleStrings(values []string, limit int) []string {
	if limit <= 0 {
		return nil
	}
	out := make([]string, 0, minInt(len(values), limit))
	seen := make(map[string]struct{}, minInt(len(values), limit))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
		if len(out) >= limit {
			break
		}
	}
	return out
}

func sanitizeCustomBattleEffectRefs(values []int, maxEffects int) []int {
	if maxEffects <= 0 {
		return nil
	}
	out := make([]int, 0, minInt(len(values), maxEffects))
	seen := map[int]struct{}{}
	for _, value := range values {
		if value < 0 || value >= maxEffects {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
		if len(out) >= maxEffects {
			break
		}
	}
	return out
}

func sanitizeCustomBattleOptionalChance(value *float64) *float64 {
	if value == nil || math.IsNaN(*value) || math.IsInf(*value, 0) {
		return nil
	}
	clean := clampFloat(*value, 0, 1)
	return &clean
}

func sanitizeCustomBattleNumberMap(values map[string]float64, limit int) map[string]float64 {
	if len(values) == 0 || limit <= 0 {
		return nil
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make(map[string]float64, minInt(len(keys), limit))
	for _, key := range keys {
		value := values[key]
		key = strings.TrimSpace(key)
		if key == "" || math.IsNaN(value) || math.IsInf(value, 0) {
			continue
		}
		out[key] = clampFloat(value, -1_000_000, 1_000_000)
		if len(out) >= limit {
			break
		}
	}
	return out
}

func sanitizeCustomBattleFormula(formula *battleSkillAmountFormula, maxTerms int) {
	if formula == nil {
		return
	}
	if math.IsNaN(formula.BaseAdd) || math.IsInf(formula.BaseAdd, 0) {
		formula.BaseAdd = 0
	}
	formula.BaseAdd = clampFloat(formula.BaseAdd, -1_000_000, 1_000_000)
	sanitizeCustomBattleFormulaTerms(&formula.Terms, maxTerms)
}

func sanitizeCustomBattleFormulaTerms(terms *[]battleAttackTerm, maxTerms int) {
	if terms == nil {
		return
	}
	if maxTerms <= 0 {
		maxTerms = 4
	}
	out := make([]battleAttackTerm, 0, minInt(len(*terms), maxTerms))
	for _, term := range *terms {
		term.AttrKey = strings.TrimSpace(term.AttrKey)
		if term.AttrKey == "" || math.IsNaN(term.Coefficient) || math.IsInf(term.Coefficient, 0) || math.IsNaN(term.PerSkillLevel) || math.IsInf(term.PerSkillLevel, 0) {
			continue
		}
		term.Coefficient = clampFloat(term.Coefficient, -1_000_000, 1_000_000)
		term.PerSkillLevel = clampFloat(term.PerSkillLevel, -1_000_000, 1_000_000)
		out = append(out, term)
		if len(out) >= maxTerms {
			break
		}
	}
	*terms = out
}

func (s *Server) resolveBattleEquipmentHooks(equipped []store.EquippedItem) []battleEquipmentHookRule {
	if len(equipped) == 0 {
		return nil
	}
	effectiveAffixesByInstance := s.resolveBattleEffectiveAffixesByInstance(equipped)
	out := make([]battleEquipmentHookRule, 0, 4)
	for _, eq := range equipped {
		if strings.TrimSpace(eq.EquipType) != "battle" {
			continue
		}
		template := s.deps.GameData.EquipmentTemplateByID(strings.TrimSpace(eq.TemplateItemID))
		if template != nil && len(template.BattleHooks) > 0 {
			for idx, hook := range template.BattleHooks {
				if hook == nil {
					continue
				}
				trigger := battleNormalizeTrigger(fmt.Sprintf("%v", hook["trigger"]))
				if trigger == "" {
					continue
				}
				effects := battleAnyMapSlice(hook["effects"])
				if len(effects) == 0 {
					continue
				}
				hookID := strings.TrimSpace(fmt.Sprintf("%v", hook["id"]))
				if hookID == "" {
					hookID = fmt.Sprintf("%s:battle_hook:%d", strings.TrimSpace(eq.TemplateItemID), idx+1)
				}
				definitionID := hookID
				if eq.InstanceID > 0 {
					hookID = fmt.Sprintf("%s@instance:%d", definitionID, eq.InstanceID)
				}
				out = append(out, battleEquipmentHookRule{
					ID:                     hookID,
					DefinitionID:           definitionID,
					SourceKind:             "equipment",
					SourceItemID:           strings.TrimSpace(eq.TemplateItemID),
					SourceInstanceID:       eq.InstanceID,
					SourceEnhanceLevel:     maxInt(0, eq.EnhanceLevel),
					Trigger:                trigger,
					Priority:               int(toInt64Loose(hook["priority"], 0)),
					MaxTriggersPerTurn:     maxInt(0, int(toInt64Loose(firstNonNil(hook["maxTriggersPerTurn"], hook["max_triggers_per_turn"]), 0))),
					MaxTriggersPerWave:     maxInt(0, int(toInt64Loose(firstNonNil(hook["maxTriggersPerWave"], hook["max_triggers_per_wave"]), 0))),
					MaxTriggersPerBattle:   maxInt(0, int(toInt64Loose(firstNonNil(hook["maxTriggersPerBattle"], hook["max_triggers_per_battle"]), 0))),
					SharedTriggerGroup:     strings.TrimSpace(fmt.Sprintf("%v", firstNonNil(hook["sharedTriggerGroup"], hook["shared_trigger_group"]))),
					ActivationCost:         parseBattleHookActivationCost(firstNonNil(hook["activationCost"], hook["activation_cost"])),
					CooldownCompletedTurns: maxInt(0, int(toInt64Loose(firstNonNil(hook["cooldownCompletedTurns"], hook["cooldown_completed_turns"]), 0))),
					SubjectFilter:          normalizeAnyMap(firstNonNil(hook["subjectFilter"], hook["subject_filter"])),
					EventFilter:            normalizeAnyMap(firstNonNil(hook["eventFilter"], hook["event_filter"])),
					Conditions:             battleAnyMapSlice(hook["conditions"]),
					Effects:                effects,
				})
			}
		}
		for slotIdx, affix := range effectiveAffixesByInstance[eq.InstanceID] {
			affixID := strings.TrimSpace(affix.AffixID)
			if affixID == "" {
				continue
			}
			def := s.deps.GameData.AffixDefinitionByID(affixID)
			effectType := strings.ToLower(strings.TrimSpace(affix.EffectType))
			if effectType == "" && def != nil {
				effectType = strings.ToLower(strings.TrimSpace(def.EffectType))
			}
			effectType = strings.ReplaceAll(effectType, "-", "_")
			params := affix.EffectParams
			if len(params) == 0 && def != nil && def.EffectParams != nil {
				params = def.EffectParams
			}
			if len(params) == 0 {
				continue
			}
			if effectType != "battle_trigger" && effectType != "battle_hook" && !battleAffixParamsContainHooks(params) {
				continue
			}
			rawHooks := any(params["battleHooks"])
			if rawHooks == nil {
				rawHooks = params["battle_hooks"]
			}
			if rawHooks == nil {
				rawHooks = params["hooks"]
			}
			hooks := battleAnyMapSlice(rawHooks)
			if len(hooks) == 0 {
				continue
			}
			for hookIdx, hook := range hooks {
				if hook == nil {
					continue
				}
				trigger := battleNormalizeTrigger(fmt.Sprintf("%v", hook["trigger"]))
				if trigger == "" {
					continue
				}
				effects := battleAnyMapSlice(hook["effects"])
				if len(effects) == 0 {
					continue
				}
				hookID := strings.TrimSpace(fmt.Sprintf("%v", hook["id"]))
				if hookID == "" {
					hookID = fmt.Sprintf("%s:affix:%s:%d:%d", strings.TrimSpace(eq.TemplateItemID), affixID, slotIdx, hookIdx+1)
				}
				definitionID := hookID
				if eq.InstanceID > 0 {
					hookID = fmt.Sprintf("%s@instance:%d", definitionID, eq.InstanceID)
				}
				out = append(out, battleEquipmentHookRule{
					ID:                     hookID,
					DefinitionID:           definitionID,
					SourceKind:             "equipment",
					SourceItemID:           strings.TrimSpace(eq.TemplateItemID),
					SourceInstanceID:       eq.InstanceID,
					SourceEnhanceLevel:     maxInt(0, eq.EnhanceLevel),
					SourceAffixID:          affixID,
					SourceAffixSlot:        slotIdx,
					Trigger:                trigger,
					Priority:               int(toInt64Loose(hook["priority"], 0)),
					MaxTriggersPerTurn:     maxInt(0, int(toInt64Loose(firstNonNil(hook["maxTriggersPerTurn"], hook["max_triggers_per_turn"]), 0))),
					MaxTriggersPerWave:     maxInt(0, int(toInt64Loose(firstNonNil(hook["maxTriggersPerWave"], hook["max_triggers_per_wave"]), 0))),
					MaxTriggersPerBattle:   maxInt(0, int(toInt64Loose(firstNonNil(hook["maxTriggersPerBattle"], hook["max_triggers_per_battle"]), 0))),
					SharedTriggerGroup:     strings.TrimSpace(fmt.Sprintf("%v", firstNonNil(hook["sharedTriggerGroup"], hook["shared_trigger_group"]))),
					ActivationCost:         parseBattleHookActivationCost(firstNonNil(hook["activationCost"], hook["activation_cost"])),
					CooldownCompletedTurns: maxInt(0, int(toInt64Loose(firstNonNil(hook["cooldownCompletedTurns"], hook["cooldown_completed_turns"]), 0))),
					SubjectFilter:          normalizeAnyMap(firstNonNil(hook["subjectFilter"], hook["subject_filter"])),
					EventFilter:            normalizeAnyMap(firstNonNil(hook["eventFilter"], hook["event_filter"])),
					Conditions:             battleAnyMapSlice(hook["conditions"]),
					Effects:                effects,
				})
			}
		}
	}
	if len(out) == 0 {
		return nil
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Priority != out[j].Priority {
			return out[i].Priority > out[j].Priority
		}
		return out[i].ID < out[j].ID
	})
	return out
}

func (s *Server) resolveBattleAffixEffectMutations(equipped []store.EquippedItem) []battleAffixEffectMutation {
	if len(equipped) == 0 {
		return nil
	}
	effectiveAffixesByInstance := s.resolveBattleEffectiveAffixesByInstance(equipped)
	out := make([]battleAffixEffectMutation, 0, 4)
	for _, eq := range equipped {
		if strings.TrimSpace(eq.EquipType) != "battle" {
			continue
		}
		for _, affix := range effectiveAffixesByInstance[eq.InstanceID] {
			affixID := strings.TrimSpace(affix.AffixID)
			if affixID == "" {
				continue
			}
			def := s.deps.GameData.AffixDefinitionByID(affixID)
			effectType := strings.ToLower(strings.TrimSpace(affix.EffectType))
			if effectType == "" && def != nil {
				effectType = strings.ToLower(strings.TrimSpace(def.EffectType))
			}
			effectType = strings.ReplaceAll(effectType, "-", "_")
			params := affix.EffectParams
			if len(params) == 0 && def != nil && def.EffectParams != nil {
				params = def.EffectParams
			}
			if len(params) == 0 {
				continue
			}
			if effectType != "effectmutation" && effectType != "effect_mutation" && !battleAffixParamsContainEffectMutations(params) {
				continue
			}
			rawMutations := any(params["effectMutations"])
			if rawMutations == nil {
				rawMutations = params["effect_mutations"]
			}
			if rawMutations == nil {
				continue
			}
			// print raw mutations in json
			for _, node := range battleAnyMapSlice(rawMutations) {
				if mutation, ok := parseBattleAffixEffectMutation(node); ok {
					out = append(out, mutation)
				}
			}
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func battleAffixParamsContainHooks(params map[string]any) bool {
	if len(params) == 0 {
		return false
	}
	for _, key := range []string{"battleHooks", "battle_hooks", "hooks"} {
		if len(battleAnyMapSlice(params[key])) > 0 {
			return true
		}
	}
	return false
}

func battleAffixParamsContainEffectMutations(params map[string]any) bool {
	if len(params) == 0 {
		return false
	}
	for _, key := range []string{"effectMutations", "effect_mutations"} {
		if len(battleAnyMapSlice(params[key])) > 0 {
			return true
		}
	}
	return false
}

func (s *Server) resolveBattleEffectiveAffixesByInstance(equipped []store.EquippedItem) map[int64][]store.EquipmentAffixSlot {
	if len(equipped) == 0 {
		return nil
	}
	sorted := append([]store.EquippedItem(nil), equipped...)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].InstanceID < sorted[j].InstanceID
	})
	bodySeen := map[string]int{}
	effectiveByInstance := make(map[int64][]store.EquipmentAffixSlot, len(sorted))
	for _, eq := range sorted {
		effectiveByInstance[eq.InstanceID] = store.FilterEffectiveAffixSlotsByLimit(eq.Affixes, s.deps.GameData, bodySeen)
	}
	return effectiveByInstance
}

func parseBattleAffixEffectMutation(node map[string]any) (battleAffixEffectMutation, bool) {
	if len(node) == 0 {
		return battleAffixEffectMutation{}, false
	}
	scope := strings.ToLower(strings.TrimSpace(fmt.Sprintf("%v", node["scope"])))

	if scope != "skill" && scope != "basic_attack" && scope != "both" {
		scope = "both"
	}
	mutation := battleAffixEffectMutation{
		ID:              strings.TrimSpace(fmt.Sprintf("%v", node["id"])),
		Priority:        int(toInt64Loose(node["priority"], 0)),
		Scope:           scope,
		SkillIDs:        map[string]struct{}{},
		SkillCategories: map[string]struct{}{},
		WeaponClasses:   map[string]struct{}{},
		DamageTypes:     map[string]struct{}{},
		EffectKinds:     map[string]struct{}{},
		EffectIDs:       map[string]struct{}{},
	}
	if mutation.ID == "<nil>" {
		mutation.ID = ""
	}
	skillSelector := normalizeAnyMap(firstNonNil(node["skillSelector"], node["skill_selector"]))
	for _, skillID := range parseBattleStringSlice(firstNonNil(skillSelector["skillIds"], skillSelector["skill_ids"], node["skillIds"], node["skill_ids"])) {
		normalized := strings.ToLower(strings.TrimSpace(skillID))
		if normalized == "" {
			continue
		}
		mutation.SkillIDList = append(mutation.SkillIDList, normalized)
		mutation.SkillIDs[normalized] = struct{}{}
	}
	mutation.SkillCategoryList = parseBattleStringSlice(firstNonNil(skillSelector["categories"], skillSelector["skillCategories"], node["skillCategories"], node["skill_categories"]))
	mutation.WeaponClassList = parseBattleStringSlice(firstNonNil(skillSelector["weaponClasses"], skillSelector["weapon_classes"], node["weaponClasses"], node["weapon_classes"]))
	mutation.DamageTypeList = parseBattleStringSlice(firstNonNil(skillSelector["damageTypes"], skillSelector["damage_types"], node["damageTypes"], node["damage_types"]))
	selector := normalizeAnyMap(node["effectSelector"])
	if len(selector) == 0 {
		selector = normalizeAnyMap(node["effect_selector"])
	}
	mode := strings.ToLower(strings.TrimSpace(fmt.Sprintf("%v", selector["mode"])))
	if mode == "" {
		mode = strings.ToLower(strings.TrimSpace(fmt.Sprintf("%v", node["effectSelectorMode"])))
	}
	switch mode {
	case "all":
		mutation.SelectAll = true
	case "index":
		mutation.Indexes = parseBattleEffectReferenceIndexes(selector["indexes"])
	case "kind":
		mutation.EffectKindList = parseBattleStringSlice(firstNonNil(selector["kinds"], selector["effectKinds"], selector["effect_kinds"]))
	case "effect_id", "effectid", "id":
		mutation.EffectIDList = parseBattleStringSlice(firstNonNil(selector["ids"], selector["effectIds"], selector["effect_ids"]))
	default:
		mutation.Indexes = parseBattleEffectReferenceIndexes(selector["indexes"])
		if len(mutation.Indexes) == 0 {
			mutation.SelectAll = true
		}
	}
	mutation.IncludeGeneratedEffects = parseBooleanLoose(firstNonNil(selector["includeGenerated"], selector["include_generated"], selector["includeGeneratedEffects"], selector["include_generated_effects"]), false)

	ops := normalizeAnyMap(node["ops"])
	if len(ops) == 0 {
		ops = normalizeAnyMap(node["operations"])
	}
	if len(ops) == 0 {
		ops = node
	}
	if formula, ok := parseBattleDynamicDeltaFormula(firstNonNil(ops["targetCountDelta"], ops["target_count_delta"])); ok {
		mutation.TargetCountDelta = &formula
	}
	mutation.TargetCountTermsAppend = parseBattleAttackTerms(firstNonNil(
		ops["targetCountFormulaTermsAppend"],
		ops["targetCountTermsAppend"],
		ops["target_count_formula_terms_append"],
		ops["target_count_terms_append"],
	))
	if formula, ok := parseBattleDynamicDeltaFormula(firstNonNil(
		ops["amountBaseAddDelta"],
		ops["amountFormulaBaseAddDelta"],
		ops["amount_base_add_delta"],
		ops["amount_formula_base_add_delta"],
	)); ok {
		mutation.AmountBaseAddDelta = &formula
	}
	mutation.AmountTermsAppend = parseBattleAttackTerms(firstNonNil(
		ops["amountTermsAppend"],
		ops["amountFormulaTermsAppend"],
		ops["amount_terms_append"],
		ops["amount_formula_terms_append"],
	))
	if rawMultiplier := firstNonNil(ops["amountMultiplier"], ops["amount_multiplier"]); rawMultiplier != nil {
		multiplier := toFloat64(rawMultiplier, math.NaN())
		if !math.IsNaN(multiplier) && !math.IsInf(multiplier, 0) {
			mutation.AmountMultiplier = &multiplier
		}
	}
	mutation.HitCountDelta = int(toInt64Loose(firstNonNil(ops["hitCountDelta"], ops["hit_count_delta"]), 0))
	mutation.CooldownDelta = int(toInt64Loose(firstNonNil(ops["cooldownDelta"], ops["cooldown_delta"]), 0))
	mutation.CooldownFloor = int(toInt64Loose(firstNonNil(ops["cooldownFloor"], ops["cooldown_floor"]), 0))
	mutation.MPCostDelta = toFloat64(firstNonNil(ops["mpCostDelta"], ops["mp_cost_delta"]), 0)
	if rawMultiplier := firstNonNil(ops["mpCostMultiplier"], ops["mp_cost_multiplier"]); rawMultiplier != nil {
		multiplier := toFloat64(rawMultiplier, math.NaN())
		mutation.MPCostMultiplier = &multiplier
	}
	mutation.AttackStatPolicy = strings.ToLower(strings.TrimSpace(fmt.Sprintf("%v", firstNonNil(ops["attackStatPolicy"], ops["attack_stat_policy"]))))
	if mutation.AttackStatPolicy == "<nil>" {
		mutation.AttackStatPolicy = ""
	}
	mutation.DamageTypePolicy = strings.ToLower(strings.TrimSpace(fmt.Sprintf("%v", firstNonNil(ops["damageTypePolicy"], ops["damage_type_policy"]))))
	if mutation.DamageTypePolicy == "<nil>" {
		mutation.DamageTypePolicy = ""
	}
	mutation.TargetingPatch = normalizeAnyMap(firstNonNil(ops["targetingPatch"], ops["targeting_patch"]))
	mutation.StatusAppend = parseBattleStringSlice(firstNonNil(
		ops["statusIdsAppend"],
		ops["statusAppend"],
		ops["status_ids_append"],
		ops["status_append"],
	))
	statusRemove := parseBattleStringSlice(firstNonNil(
		ops["statusIdsRemove"],
		ops["statusRemove"],
		ops["status_ids_remove"],
		ops["status_remove"],
	))
	if len(statusRemove) > 0 {
		mutation.StatusRemoveList = append([]string(nil), statusRemove...)
		mutation.StatusRemove = map[string]struct{}{}
		for _, statusID := range statusRemove {
			normalized := strings.TrimSpace(statusID)
			if normalized == "" {
				continue
			}
			mutation.StatusRemove[normalized] = struct{}{}
		}
	}
	constraints := normalizeAnyMap(node["constraints"])
	if len(constraints) > 0 {
		if min := int(toInt64Loose(firstNonNil(constraints["targetCountMin"], constraints["target_count_min"]), 0)); min > 0 {
			mutation.ConstraintTargetCountMin = &min
		}
		if max := int(toInt64Loose(firstNonNil(constraints["targetCountMax"], constraints["target_count_max"]), 0)); max > 0 {
			mutation.ConstraintTargetCountMax = &max
		}
	}
	if appendEffects := parseBattleSkillEffects(firstNonNil(ops["appendEffects"], ops["effectsAppend"], ops["append_effects"], ops["effects_append"]), parseBattleSkillTargeting(nil, "vanguard", "front_to_rear", 1, nil, 0, false, nil), "physical"); len(appendEffects) > 0 {
		mutation.AppendEffects = appendEffects
	}
	if !mutation.SelectAll && len(mutation.Indexes) == 0 && len(mutation.EffectKindList) == 0 && len(mutation.EffectIDList) == 0 {
		mutation.SelectAll = true
	}
	if mutation.ConstraintTargetCountMin == nil {
		min := 1
		mutation.ConstraintTargetCountMin = &min
	}
	normalizeBattleAffixEffectMutation(&mutation)
	return mutation, true
}

func validateBattleSpecializationTargetingPatch(patch map[string]any) error {
	if len(patch) == 0 {
		return fmt.Errorf("%w: targetingPatch must be a non-empty object", store.ErrSpecializationDefinitionInvalid)
	}
	allowed := map[string]struct{}{
		"targetPositions": {}, "secondarySeek": {}, "targetScope": {}, "selectRule": {},
		"selectAttrKey": {}, "fallbackSelfIfEmpty": {}, "targetCount": {},
	}
	for key := range patch {
		if _, ok := allowed[key]; !ok {
			return fmt.Errorf("%w: unsupported targetingPatch field %q", store.ErrSpecializationDefinitionInvalid, key)
		}
	}
	if raw, exists := patch["targetPositions"]; exists {
		if err := validateBattleSpecializationStringList(raw, "targetingPatch.targetPositions"); err != nil {
			return err
		}
		for _, position := range battleParseStringList(raw) {
			switch normalizeBattleTargetPosition(position) {
			case "vanguard", "midguard", "rearguard":
			default:
				return fmt.Errorf("%w: targetingPatch.targetPositions contains unsupported value %q", store.ErrSpecializationDefinitionInvalid, position)
			}
		}
	}
	validateEnum := func(key string, allowedValues ...string) error {
		raw, exists := patch[key]
		if !exists {
			return nil
		}
		value, ok := raw.(string)
		if !ok || strings.TrimSpace(value) == "" {
			return fmt.Errorf("%w: targetingPatch.%s must be a non-empty string", store.ErrSpecializationDefinitionInvalid, key)
		}
		normalized := strings.ToLower(strings.TrimSpace(value))
		for _, allowedValue := range allowedValues {
			if normalized == allowedValue {
				return nil
			}
		}
		return fmt.Errorf("%w: targetingPatch.%s has unsupported value %q", store.ErrSpecializationDefinitionInvalid, key, value)
	}
	if err := validateEnum("targetScope", "self", "ally", "enemy", "all"); err != nil {
		return err
	}
	if err := validateEnum("secondarySeek", "front_to_rear", "rear_to_front", "random_alive", "none"); err != nil {
		return err
	}
	if err := validateEnum("selectRule", "hp_max", "hp_min", "hp_percent_max", "hp_percent_min", "first", "boss_first", "attr_max", "attr_min", "random", "self"); err != nil {
		return err
	}
	selectRule, _ := patch["selectRule"].(string)
	selectAttrKey, hasSelectAttrKey := patch["selectAttrKey"].(string)
	if _, exists := patch["selectAttrKey"]; exists && (!hasSelectAttrKey || strings.TrimSpace(selectAttrKey) == "") {
		return fmt.Errorf("%w: targetingPatch.selectAttrKey must be a non-empty string", store.ErrSpecializationDefinitionInvalid)
	}
	isAttributeRule := strings.EqualFold(selectRule, "attr_max") || strings.EqualFold(selectRule, "attr_min")
	if isAttributeRule != hasSelectAttrKey {
		return fmt.Errorf("%w: targetingPatch selectRule attr_max/attr_min and selectAttrKey must be authored together", store.ErrSpecializationDefinitionInvalid)
	}
	if raw, exists := patch["fallbackSelfIfEmpty"]; exists {
		if _, ok := raw.(bool); !ok {
			return fmt.Errorf("%w: targetingPatch.fallbackSelfIfEmpty must be boolean", store.ErrSpecializationDefinitionInvalid)
		}
	}
	if raw, exists := patch["targetCount"]; exists {
		value := toFloat64(raw, math.NaN())
		if math.IsNaN(value) || math.IsInf(value, 0) || value < 1 || value != math.Trunc(value) {
			return fmt.Errorf("%w: targetingPatch.targetCount must be a positive integer", store.ErrSpecializationDefinitionInvalid)
		}
	}
	return nil
}

func validateBattleSpecializationMutationKindCompatibility(id string, mutation battleAffixEffectMutation, ops map[string]any) error {
	hasAnyOp := func(keys ...string) bool {
		for _, key := range keys {
			if _, exists := ops[key]; exists {
				return true
			}
		}
		return false
	}
	if len(mutation.EffectKindList) == 0 {
		return nil
	}
	requireKinds := func(label string, allowed ...string) error {
		allowedSet := make(map[string]struct{}, len(allowed))
		for _, value := range allowed {
			allowedSet[battleNormalizeTrigger(value)] = struct{}{}
		}
		for _, effectKind := range mutation.EffectKindList {
			if _, ok := allowedSet[battleNormalizeTrigger(effectKind)]; !ok {
				return fmt.Errorf("%w: mutation %s operation %s cannot target effect kind %q", store.ErrSpecializationDefinitionInvalid, id, label, effectKind)
			}
		}
		return nil
	}
	if hasAnyOp("amountBaseAddDelta", "amountFormulaBaseAddDelta", "amountTermsAppend", "amountFormulaTermsAppend", "amountMultiplier") {
		if err := requireKinds("amount", "damage", "heal"); err != nil {
			return err
		}
	}
	if hasAnyOp("hitCountDelta", "attackStatPolicy", "damageTypePolicy") {
		if err := requireKinds("damage", "damage"); err != nil {
			return err
		}
	}
	if hasAnyOp("statusIdsAppend", "statusAppend", "statusIdsRemove", "statusRemove") {
		if err := requireKinds("status", "status"); err != nil {
			return err
		}
	}
	for _, appended := range battleAnyMapSlice(firstNonNil(ops["appendEffects"], ops["effectsAppend"])) {
		switch battleNormalizeTrigger(fmt.Sprintf("%v", appended["kind"])) {
		case "runtimedamageamp", "splashdamage", "secondarydamage":
			if err := requireKinds("appendEffects", "damage"); err != nil {
				return err
			}
		}
	}
	return nil
}

func parseBattleSpecializationEffectMutation(node map[string]any) (battleAffixEffectMutation, error) {
	if len(node) == 0 {
		return battleAffixEffectMutation{}, fmt.Errorf("%w: empty effect mutation", store.ErrSpecializationDefinitionInvalid)
	}
	id := strings.TrimSpace(fmt.Sprintf("%v", node["id"]))
	if id == "" || id == "<nil>" {
		return battleAffixEffectMutation{}, fmt.Errorf("%w: effect mutation id is required", store.ErrSpecializationDefinitionInvalid)
	}
	scope := strings.ToLower(strings.TrimSpace(fmt.Sprintf("%v", node["scope"])))
	if scope != "skill" && scope != "basic_attack" && scope != "both" {
		return battleAffixEffectMutation{}, fmt.Errorf("%w: mutation %s has unsupported scope %q", store.ErrSpecializationDefinitionInvalid, id, scope)
	}
	allowedTopLevel := map[string]struct{}{
		"id": {}, "priority": {}, "scope": {}, "skillIds": {}, "skillCategories": {}, "weaponClasses": {}, "damageTypes": {},
		"effectSelector": {}, "ops": {},
	}
	for key := range node {
		if _, ok := allowedTopLevel[key]; !ok {
			return battleAffixEffectMutation{}, fmt.Errorf("%w: mutation %s has unsupported field %q", store.ErrSpecializationDefinitionInvalid, id, key)
		}
	}
	validateStringList := func(field string) error {
		raw, exists := node[field]
		if !exists {
			return nil
		}
		valid := false
		switch typed := raw.(type) {
		case []string:
			valid = true
		case []any:
			valid = true
			for _, item := range typed {
				if _, ok := item.(string); !ok {
					valid = false
					break
				}
			}
		}
		if !valid {
			return fmt.Errorf("%w: mutation %s field %s must be a string array", store.ErrSpecializationDefinitionInvalid, id, field)
		}
		return nil
	}
	for _, field := range []string{"skillIds", "skillCategories", "weaponClasses", "damageTypes"} {
		if err := validateStringList(field); err != nil {
			return battleAffixEffectMutation{}, err
		}
	}
	selector := normalizeAnyMap(firstNonNil(node["effectSelector"], node["effect_selector"]))
	for key, raw := range selector {
		switch key {
		case "mode", "kinds", "ids", "includeGenerated", "effectKinds", "effectIds", "includeGeneratedEffects":
		default:
			return battleAffixEffectMutation{}, fmt.Errorf("%w: mutation %s has unsupported effect selector field %q", store.ErrSpecializationDefinitionInvalid, id, key)
		}
		if (key == "includeGenerated" || key == "includeGeneratedEffects") && raw != nil {
			if _, ok := raw.(bool); !ok {
				return battleAffixEffectMutation{}, fmt.Errorf("%w: mutation %s %s must be boolean", store.ErrSpecializationDefinitionInvalid, id, key)
			}
		}
	}
	mode := strings.ToLower(strings.TrimSpace(fmt.Sprintf("%v", selector["mode"])))
	switch mode {
	case "all":
	case "kind":
		if len(parseBattleStringSlice(firstNonNil(selector["kinds"], selector["effectKinds"], selector["effect_kinds"]))) == 0 {
			return battleAffixEffectMutation{}, fmt.Errorf("%w: mutation %s kind selector is empty", store.ErrSpecializationDefinitionInvalid, id)
		}
	case "effect_id", "effectid", "id":
		if len(parseBattleStringSlice(firstNonNil(selector["ids"], selector["effectIds"], selector["effect_ids"]))) == 0 {
			return battleAffixEffectMutation{}, fmt.Errorf("%w: mutation %s effect id selector is empty", store.ErrSpecializationDefinitionInvalid, id)
		}
	case "index":
		return battleAffixEffectMutation{}, fmt.Errorf("%w: mutation %s cannot use index selectors", store.ErrSpecializationDefinitionInvalid, id)
	default:
		return battleAffixEffectMutation{}, fmt.Errorf("%w: mutation %s has unsupported selector mode %q", store.ErrSpecializationDefinitionInvalid, id, mode)
	}
	ops := normalizeAnyMap(firstNonNil(node["ops"], node["operations"]))
	if len(ops) == 0 {
		return battleAffixEffectMutation{}, fmt.Errorf("%w: mutation %s has no operations", store.ErrSpecializationDefinitionInvalid, id)
	}
	allowedOps := map[string]struct{}{
		"targetCountDelta": {}, "targetCountFormulaTermsAppend": {}, "targetCountTermsAppend": {},
		"amountBaseAddDelta": {}, "amountFormulaBaseAddDelta": {}, "amountTermsAppend": {}, "amountFormulaTermsAppend": {},
		"amountMultiplier": {}, "hitCountDelta": {}, "cooldownDelta": {}, "cooldownFloor": {},
		"mpCostDelta": {}, "mpCostMultiplier": {}, "attackStatPolicy": {}, "damageTypePolicy": {},
		"targetingPatch": {}, "statusIdsAppend": {}, "statusAppend": {}, "statusIdsRemove": {}, "statusRemove": {},
		"appendEffects": {}, "effectsAppend": {},
	}
	for key := range ops {
		if _, ok := allowedOps[key]; !ok {
			return battleAffixEffectMutation{}, fmt.Errorf("%w: mutation %s has unsupported operation %q", store.ErrSpecializationDefinitionInvalid, id, key)
		}
	}
	mutation, ok := parseBattleAffixEffectMutation(node)
	if !ok {
		return battleAffixEffectMutation{}, fmt.Errorf("%w: cannot parse mutation %s", store.ErrSpecializationDefinitionInvalid, id)
	}
	if mutation.AmountMultiplier != nil && (*mutation.AmountMultiplier < 0 || math.IsNaN(*mutation.AmountMultiplier) || math.IsInf(*mutation.AmountMultiplier, 0)) {
		return battleAffixEffectMutation{}, fmt.Errorf("%w: mutation %s has invalid amount multiplier", store.ErrSpecializationDefinitionInvalid, id)
	}
	if math.IsNaN(mutation.MPCostDelta) || math.IsInf(mutation.MPCostDelta, 0) {
		return battleAffixEffectMutation{}, fmt.Errorf("%w: mutation %s has invalid mp cost delta", store.ErrSpecializationDefinitionInvalid, id)
	}
	if mutation.MPCostMultiplier != nil && (*mutation.MPCostMultiplier <= 0 || math.IsNaN(*mutation.MPCostMultiplier) || math.IsInf(*mutation.MPCostMultiplier, 0)) {
		return battleAffixEffectMutation{}, fmt.Errorf("%w: mutation %s has invalid mp cost multiplier", store.ErrSpecializationDefinitionInvalid, id)
	}
	if mutation.CooldownDelta < 0 {
		if mutation.CooldownFloor < 1 {
			return battleAffixEffectMutation{}, fmt.Errorf("%w: mutation %s that reduces cooldown requires cooldownFloor >= 1", store.ErrSpecializationDefinitionInvalid, id)
		}
		if mutation.MPCostMultiplier == nil || *mutation.MPCostMultiplier < 3 {
			return battleAffixEffectMutation{}, fmt.Errorf("%w: mutation %s that reduces cooldown requires mpCostMultiplier >= 3", store.ErrSpecializationDefinitionInvalid, id)
		}
		if mutation.MPCostDelta < 0 {
			return battleAffixEffectMutation{}, fmt.Errorf("%w: mutation %s that reduces cooldown requires mpCostDelta >= 0", store.ErrSpecializationDefinitionInvalid, id)
		}
		if len(mutation.WeaponClasses) == 0 {
			return battleAffixEffectMutation{}, fmt.Errorf("%w: mutation %s that reduces cooldown requires weaponClasses", store.ErrSpecializationDefinitionInvalid, id)
		}
	}
	if mutation.HitCountDelta > 0 && len(mutation.WeaponClasses) == 0 {
		return battleAffixEffectMutation{}, fmt.Errorf("%w: mutation %s that adds hits requires weaponClasses", store.ErrSpecializationDefinitionInvalid, id)
	}
	if mutation.AttackStatPolicy != "" && mutation.SelectAll {
		return battleAffixEffectMutation{}, fmt.Errorf("%w: mutation %s attack stat policy requires a damage-only selector", store.ErrSpecializationDefinitionInvalid, id)
	}
	switch mutation.AttackStatPolicy {
	case "", "higher":
	default:
		return battleAffixEffectMutation{}, fmt.Errorf("%w: mutation %s has unsupported attack stat policy %q", store.ErrSpecializationDefinitionInvalid, id, mutation.AttackStatPolicy)
	}
	switch mutation.DamageTypePolicy {
	case "", "preserve", "physical", "magical", "higher_final_damage":
	default:
		return battleAffixEffectMutation{}, fmt.Errorf("%w: mutation %s has unsupported damage type policy %q", store.ErrSpecializationDefinitionInvalid, id, mutation.DamageTypePolicy)
	}
	if _, exists := ops["targetingPatch"]; exists {
		if err := validateBattleSpecializationTargetingPatch(mutation.TargetingPatch); err != nil {
			return battleAffixEffectMutation{}, fmt.Errorf("mutation %s: %w", id, err)
		}
	}
	mutation.AppendEffects = nil
	generatedIDs := map[string]struct{}{}
	secondaryDamageCount := 0
	hasSplashDamage := false
	for index, raw := range battleAnyMapSlice(firstNonNil(ops["appendEffects"], ops["effectsAppend"])) {
		kind := strings.ToLower(strings.TrimSpace(fmt.Sprintf("%v", raw["kind"])))
		generatedID := strings.TrimSpace(fmt.Sprintf("%v", raw["id"]))
		if generatedID == "" || generatedID == "<nil>" {
			return battleAffixEffectMutation{}, fmt.Errorf("%w: mutation %s appended effect %d requires a stable id", store.ErrSpecializationDefinitionInvalid, id, index+1)
		}
		normalizedGeneratedID := strings.ToLower(generatedID)
		if _, exists := generatedIDs[normalizedGeneratedID]; exists {
			return battleAffixEffectMutation{}, fmt.Errorf("%w: mutation %s repeats appended effect id %q", store.ErrSpecializationDefinitionInvalid, id, generatedID)
		}
		generatedIDs[normalizedGeneratedID] = struct{}{}
		effect := battleSkillEffect{ID: generatedID, GeneratedBy: id, Kind: kind}
		switch kind {
		case "consume_hp":
			effect.MaxHPRatio = toFloat64(firstNonNil(raw["maxHpRatio"], raw["max_hp_ratio"]), 0)
			effect.MinimumHP = toFloat64(firstNonNil(raw["minimumHp"], raw["minimum_hp"]), 1)
			if effect.MaxHPRatio <= 0 || effect.MaxHPRatio > 1 || effect.MinimumHP < 0 {
				return battleAffixEffectMutation{}, fmt.Errorf("%w: mutation %s has invalid consume_hp effect", store.ErrSpecializationDefinitionInvalid, id)
			}
		case "runtime_damage_amp":
			value := toFloat64(raw["value"], math.NaN())
			if math.IsNaN(value) || math.IsInf(value, 0) || value < -1 {
				return battleAffixEffectMutation{}, fmt.Errorf("%w: mutation %s has invalid runtime damage amp", store.ErrSpecializationDefinitionInvalid, id)
			}
			mutation.RuntimeDamageAmp += value
			continue
		case "splash_damage":
			hasSplashDamage = true
			effect.AmountRatio = toFloat64(firstNonNil(raw["amountRatio"], raw["amount_ratio"]), 0)
			effect.Targeting.TargetCount.Base = int(toInt64Loose(firstNonNil(raw["targetCount"], raw["target_count"]), 0))
			if effect.AmountRatio <= 0 || effect.Targeting.TargetCount.Base <= 0 {
				return battleAffixEffectMutation{}, fmt.Errorf("%w: mutation %s has invalid splash damage effect", store.ErrSpecializationDefinitionInvalid, id)
			}
		case "secondary_damage":
			secondaryDamageCount++
			if secondaryDamageCount > 1 {
				return battleAffixEffectMutation{}, fmt.Errorf("%w: mutation %s supports at most one secondary_damage", store.ErrSpecializationDefinitionInvalid, id)
			}
			effect.AmountRatio = toFloat64(raw["amountRatio"], 0)
			effect.SecondaryTargetPolicy = strings.ToLower(strings.TrimSpace(fmt.Sprintf("%v", raw["targetPolicy"])))
			effect.SecondaryTargetPosition = normalizeBattleTargetPosition(fmt.Sprintf("%v", raw["targetPosition"]))
			if effect.AmountRatio <= 0 || effect.AmountRatio > 0.5 {
				return battleAffixEffectMutation{}, fmt.Errorf("%w: mutation %s secondary_damage amountRatio must be within (0, 0.5]", store.ErrSpecializationDefinitionInvalid, id)
			}
			switch effect.SecondaryTargetPolicy {
			case "same_row", "all_rows":
				if effect.SecondaryTargetPosition != "" {
					return battleAffixEffectMutation{}, fmt.Errorf("%w: mutation %s secondary_damage targetPosition is only valid for fixed_row", store.ErrSpecializationDefinitionInvalid, id)
				}
			case "fixed_row":
				if effect.SecondaryTargetPosition == "" {
					return battleAffixEffectMutation{}, fmt.Errorf("%w: mutation %s secondary_damage fixed_row requires targetPosition", store.ErrSpecializationDefinitionInvalid, id)
				}
			default:
				return battleAffixEffectMutation{}, fmt.Errorf("%w: mutation %s has invalid secondary_damage targetPolicy %q", store.ErrSpecializationDefinitionInvalid, id, effect.SecondaryTargetPolicy)
			}
		case "mark_target":
			effect.MarkID = strings.TrimSpace(fmt.Sprintf("%v", firstNonNil(raw["markId"], raw["mark_id"])))
			effect.DurationTurns = toInt64Loose(firstNonNil(raw["durationTurns"], raw["duration_turns"]), 1)
			if effect.MarkID == "" || effect.MarkID == "<nil>" || effect.DurationTurns <= 0 {
				return battleAffixEffectMutation{}, fmt.Errorf("%w: mutation %s has invalid mark target effect", store.ErrSpecializationDefinitionInvalid, id)
			}
		default:
			parsed := parseBattleSkillEffects([]any{raw}, parseBattleSkillTargeting(nil, "vanguard", "front_to_rear", 1, nil, 0, false, nil), "physical")
			if len(parsed) != 1 {
				return battleAffixEffectMutation{}, fmt.Errorf("%w: mutation %s has unsupported appended effect %q", store.ErrSpecializationDefinitionInvalid, id, kind)
			}
			effect = parsed[0]
			effect.GeneratedBy = id
			if effect.ID == "" {
				effect.ID = generatedID
			}
		}
		mutation.AppendEffects = append(mutation.AppendEffects, effect)
	}
	hasPositiveTargetExpansion := false
	if mutation.TargetCountDelta != nil {
		hasPositiveTargetExpansion = mutation.TargetCountDelta.BaseAdd > 0
		for _, term := range mutation.TargetCountDelta.Terms {
			hasPositiveTargetExpansion = hasPositiveTargetExpansion || term.Coefficient > 0 || term.PerSkillLevel > 0 || term.PerEnhanceLevel > 0
		}
	}
	for _, term := range mutation.TargetCountTermsAppend {
		hasPositiveTargetExpansion = hasPositiveTargetExpansion || term.Coefficient > 0 || term.PerSkillLevel > 0 || term.PerEnhanceLevel > 0
	}
	if secondaryDamageCount == 1 && (hasSplashDamage || hasPositiveTargetExpansion || mutation.HitCountDelta > 0) {
		return battleAffixEffectMutation{}, fmt.Errorf("%w: mutation %s secondary_damage cannot be combined with another hit or target expansion", store.ErrSpecializationDefinitionInvalid, id)
	}
	normalizeBattleAffixEffectMutation(&mutation)
	if err := validateBattleSpecializationMutationKindCompatibility(id, mutation, ops); err != nil {
		return battleAffixEffectMutation{}, err
	}
	return mutation, nil
}

func normalizeBattleAffixEffectMutation(mutation *battleAffixEffectMutation) {
	if mutation == nil {
		return
	}
	scope := strings.ToLower(strings.TrimSpace(mutation.Scope))
	if scope != "skill" && scope != "basic_attack" && scope != "both" {
		scope = "both"
	}
	mutation.Scope = scope
	mutation.SkillIDs = map[string]struct{}{}
	mutation.SkillIDList = parseBattleStringSlice(mutation.SkillIDList)
	for _, skillID := range mutation.SkillIDList {
		normalized := strings.ToLower(strings.TrimSpace(skillID))
		if normalized == "" {
			continue
		}
		mutation.SkillIDs[normalized] = struct{}{}
	}
	normalizeSelector := func(values []string) ([]string, map[string]struct{}) {
		list := parseBattleStringSlice(values)
		set := make(map[string]struct{}, len(list))
		for index := range list {
			list[index] = strings.ToLower(strings.TrimSpace(list[index]))
			if list[index] != "" {
				set[list[index]] = struct{}{}
			}
		}
		return list, set
	}
	mutation.SkillCategoryList, mutation.SkillCategories = normalizeSelector(mutation.SkillCategoryList)
	mutation.WeaponClassList, mutation.WeaponClasses = normalizeSelector(mutation.WeaponClassList)
	mutation.DamageTypeList, mutation.DamageTypes = normalizeSelector(mutation.DamageTypeList)
	mutation.EffectKindList, mutation.EffectKinds = normalizeSelector(mutation.EffectKindList)
	mutation.EffectIDList, mutation.EffectIDs = normalizeSelector(mutation.EffectIDList)
	mutation.StatusAppend = parseBattleStringSlice(mutation.StatusAppend)
	mutation.StatusRemoveList = parseBattleStringSlice(mutation.StatusRemoveList)
	if len(mutation.StatusRemoveList) == 0 {
		mutation.StatusRemove = nil
	} else {
		mutation.StatusRemove = make(map[string]struct{}, len(mutation.StatusRemoveList))
		for _, statusID := range mutation.StatusRemoveList {
			normalized := strings.TrimSpace(statusID)
			if normalized == "" {
				continue
			}
			mutation.StatusRemove[normalized] = struct{}{}
		}
	}
	if !mutation.SelectAll && len(mutation.Indexes) == 0 && len(mutation.EffectKinds) == 0 && len(mutation.EffectIDs) == 0 {
		mutation.SelectAll = true
	}
	if mutation.ConstraintTargetCountMin == nil {
		min := 1
		mutation.ConstraintTargetCountMin = &min
	}
	for idx := range mutation.AppendEffects {
		normalizeBattleSkillEffect(&mutation.AppendEffects[idx])
	}
}

func normalizeBattleAffixEffectMutations(mutations []battleAffixEffectMutation) {
	for idx := range mutations {
		normalizeBattleAffixEffectMutation(&mutations[idx])
	}
}

func parseBattleStringSlice(raw any) []string {
	out := make([]string, 0, 4)
	appendItem := func(value string) {
		normalized := strings.TrimSpace(value)
		if normalized == "" {
			return
		}
		for _, existing := range out {
			if existing == normalized {
				return
			}
		}
		out = append(out, normalized)
	}
	switch typed := raw.(type) {
	case []string:
		for _, value := range typed {
			appendItem(value)
		}
	case []any:
		for _, value := range typed {
			appendItem(fmt.Sprintf("%v", value))
		}
	case string:
		for _, value := range strings.Split(typed, ",") {
			appendItem(value)
		}
	}
	return out
}

func parseBattleDynamicDeltaFormula(raw any) (battleSkillAmountFormula, bool) {
	switch typed := raw.(type) {
	case int:
		return battleSkillAmountFormula{BaseAdd: float64(typed)}, true
	case int64:
		return battleSkillAmountFormula{BaseAdd: float64(typed)}, true
	case float64:
		if math.IsNaN(typed) || math.IsInf(typed, 0) {
			return battleSkillAmountFormula{}, false
		}
		return battleSkillAmountFormula{BaseAdd: typed}, true
	case string:
		value := toFloat64(strings.TrimSpace(typed), math.NaN())
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return battleSkillAmountFormula{}, false
		}
		return battleSkillAmountFormula{BaseAdd: value}, true
	case map[string]any:
		formula := parseBattleSkillAmountFormula(typed, nil)
		return formula, true
	default:
		node := normalizeAnyMap(raw)
		if len(node) == 0 {
			return battleSkillAmountFormula{}, false
		}
		formula := parseBattleSkillAmountFormula(node, nil)
		return formula, true
	}
}

func (s *Server) resolveBattleBasicAttackProfile(equipped []store.EquippedItem) *battleBasicAttackProfile {
	selectByPriority := func(slot string) *store.EquippedItem {
		for idx := range equipped {
			row := &equipped[idx]
			if strings.TrimSpace(row.EquipType) != "battle" {
				continue
			}
			slots := strings.Split(strings.TrimSpace(row.EquippedSlot), ",")
			for _, item := range slots {
				if strings.TrimSpace(item) == slot {
					return row
				}
			}
		}
		return nil
	}
	selected := selectByPriority("weapon")
	if selected == nil {
		selected = selectByPriority("right_hand")
	}
	if selected == nil {
		selected = selectByPriority("right_hand_2")
	}
	if selected == nil {
		selected = selectByPriority("left_hand")
	}
	if selected == nil {
		return nil
	}
	template := s.deps.GameData.EquipmentTemplateByID(strings.TrimSpace(selected.TemplateItemID))
	if template == nil {
		return nil
	}
	targetDirection := normalizeBattleTargetPosition(template.BasicAttackTarget)
	if targetDirection == "" {
		targetDirection = "vanguard"
	}
	seekStrategy := normalizeBattleSecondarySeek(template.TargetSeekStrategy, "front_to_rear")
	legacyDamageType := normalizeBattleDamageType(template.BasicAttackFormula.DamageType)
	if strings.TrimSpace(template.BasicAttackFormula.DamageType) == "" {
		legacyDamageType = "physical"
	}
	legacyFormula := make([]battleAttackTerm, 0, len(template.BasicAttackFormula.Formula))
	for _, term := range template.BasicAttackFormula.Formula {
		normalizedKey := strings.TrimSpace(term.AttrKey)
		if normalizedKey == "" || math.IsNaN(term.Coefficient) || math.IsInf(term.Coefficient, 0) {
			continue
		}
		legacyFormula = append(legacyFormula, battleAttackTerm{
			AttrKey:     normalizedKey,
			Coefficient: term.Coefficient,
		})
	}
	legacySpec := parseBattleDamageSpec(
		template.BasicAttackFormula.DamageSpec,
		legacyDamageType,
		battleSkillAmountFormula{
			BaseAdd: 0,
			Terms:   append([]battleAttackTerm(nil), legacyFormula...),
		},
	)
	fallbackTargeting := battleSkillTargeting{
		TargetPositions: []string{targetDirection},
		SecondarySeek:   seekStrategy,
		TargetScope:     "enemy",
		SelectRule:      "random",
		TargetCount: battleSkillTargetCount{
			Base: 1,
		},
	}
	effects := parseBattleSkillEffects(template.BasicAttackEffects, fallbackTargeting, legacyDamageType)
	if len(effects) > 0 {
		effectDamageType := effects[0].DamageType
		if strings.TrimSpace(effectDamageType) == "" {
			effectDamageType = legacyDamageType
		}
		formulaForDisplay := append([]battleAttackTerm(nil), legacyFormula...)
		if len(formulaForDisplay) == 0 {
			formulaForDisplay = battleExtractBasicAttackFormulaTermsFromEffects(effects)
		}
		specForDisplay := legacySpec
		if effectSpec, ok := battleExtractBasicAttackDamageSpecFromEffects(effects, effectDamageType, formulaForDisplay); ok {
			specForDisplay = effectSpec
		}
		return &battleBasicAttackProfile{
			TargetDirection:    firstNonEmptyTargetPosition(effects[0].Targeting.TargetPositions),
			TargetSeekStrategy: normalizeBattleSecondarySeek(effects[0].Targeting.SecondarySeek, seekStrategy),
			DamageType:         effectDamageType,
			Formula:            formulaForDisplay,
			DamageSpec:         specForDisplay,
			WeaponItemID:       strings.TrimSpace(selected.TemplateItemID),
			SourceEnhanceLevel: maxInt(0, selected.EnhanceLevel),
			Effects:            effects,
		}
	}
	if len(legacyFormula) == 0 {
		return nil
	}
	return &battleBasicAttackProfile{
		TargetDirection:    targetDirection,
		TargetSeekStrategy: seekStrategy,
		DamageType:         legacyDamageType,
		Formula:            legacyFormula,
		DamageSpec:         legacySpec,
		WeaponItemID:       strings.TrimSpace(selected.TemplateItemID),
		SourceEnhanceLevel: maxInt(0, selected.EnhanceLevel),
	}
}

func battleExtractBasicAttackFormulaTermsFromEffects(effects []battleSkillEffect) []battleAttackTerm {
	for _, effect := range effects {
		if !strings.EqualFold(strings.TrimSpace(effect.Kind), "damage") {
			continue
		}
		if len(effect.AmountFormula.Terms) > 0 {
			return append([]battleAttackTerm(nil), effect.AmountFormula.Terms...)
		}
		if len(effect.DamageSpec.AmountFormula.Terms) > 0 {
			return append([]battleAttackTerm(nil), effect.DamageSpec.AmountFormula.Terms...)
		}
	}
	return nil
}

func battleExtractBasicAttackDamageSpecFromEffects(
	effects []battleSkillEffect,
	fallbackDamageType string,
	fallbackFormula []battleAttackTerm,
) (battleDamageSpec, bool) {
	for _, effect := range effects {
		if !strings.EqualFold(strings.TrimSpace(effect.Kind), "damage") {
			continue
		}
		fallbackAmount := effect.AmountFormula
		if len(fallbackAmount.Terms) == 0 {
			fallbackAmount.Terms = append([]battleAttackTerm(nil), fallbackFormula...)
		}
		spec := normalizeBattleDamageSpecForRuntime(effect.DamageSpec, effect.DamageType, fallbackAmount)
		spec = normalizeBattleDamageSpecForRuntime(spec, fallbackDamageType, fallbackAmount)
		return spec, true
	}
	return battleDamageSpec{}, false
}

func (s *Server) battleEnemyCatalog() map[string]battleEnemyConfig {
	s.battleDataMu.RLock()
	if s.battleEnemyLoaded {
		cached := s.battleEnemyCache
		s.battleDataMu.RUnlock()
		return cached
	}
	s.battleDataMu.RUnlock()
	out := map[string]battleEnemyConfig{}
	for _, raw := range s.deps.GameData.Enemies {
		id, ok := stringFromAny(raw["id"])
		if !ok || id == "" {
			continue
		}
		name, _ := stringFromAny(raw["name"])
		nameKey, _ := stringFromAny(raw["nameKey"])
		descKey, _ := stringFromAny(raw["descKey"])
		position, _ := stringFromAny(raw["position"])
		position = strings.ToLower(strings.TrimSpace(position))
		if position != "vanguard" && position != "midguard" && position != "rearguard" {
			position = "vanguard"
		}
		level := int(maxInt(1, int(toInt64Loose(raw["level"], 1))))
		enemyType := strings.ToLower(strings.TrimSpace(fmt.Sprintf("%v", raw["type"])))
		if enemyType != "normal" && enemyType != "elite" && enemyType != "boss" {
			enemyType = strings.ToLower(strings.TrimSpace(fmt.Sprintf("%v", raw["enemyType"])))
		}
		if enemyType != "normal" && enemyType != "elite" && enemyType != "boss" {
			if strings.Contains(strings.ToLower(id), "boss") {
				enemyType = "boss"
			} else if strings.Contains(strings.ToLower(id), "elite") {
				enemyType = "elite"
			} else {
				enemyType = "normal"
			}
		}
		stats := map[string]float64{}
		for key, value := range raw {
			if key == "id" || key == "name" || key == "position" || key == "drops" || key == "exp" || key == "__rowNum" {
				continue
			}
			num := toFloat64(value, math.NaN())
			if math.IsNaN(num) {
				continue
			}
			stats[strings.TrimSpace(key)] = num
		}
		drops := parseBattleDrops(raw["drops"])
		dropIDs := make([]string, 0, len(drops))
		for id := range drops {
			dropIDs = append(dropIDs, id)
		}
		sort.Strings(dropIDs)
		skillIDs := make([]string, 0, 4)
		for _, skillID := range toStringSlice(raw["skillIds"]) {
			normalized := strings.TrimSpace(skillID)
			if normalized == "" || !s.battleSkillExists(normalized) {
				continue
			}
			skillIDs = appendUniqueSkillID(skillIDs, normalized)
		}
		enemyClass, _ := stringFromAny(raw["class"])
		enemyRace, _ := stringFromAny(raw["race"])
		cfg := battleEnemyConfig{
			ID:        id,
			Name:      name,
			NameKey:   nameKey,
			DescKey:   descKey,
			Position:  position,
			Level:     level,
			Type:      enemyType,
			Class:     strings.TrimSpace(enemyClass),
			Race:      strings.TrimSpace(enemyRace),
			Exp:       toFloat64(raw["exp"], 0),
			Stats:     stats,
			Drops:     drops,
			DropIDs:   dropIDs,
			SkillIDs:  skillIDs,
			Equipment: s.parseBattleEnemyEquipment(raw["equipment"]),
		}
		if v, ok := raw["initialBuffIds"]; ok {
			if arr, ok := v.([]any); ok {
				for _, id := range arr {
					if s, ok := id.(string); ok && s != "" {
						cfg.InitialBuffIds = append(cfg.InitialBuffIds, s)
					}
				}
			}
		}
		if cfg.NameKey == "" {
			cfg.NameKey = fmt.Sprintf("enemy.%s.name", cfg.ID)
		}
		if cfg.Stats["hp"] <= 0 {
			cfg.Stats["hp"] = 30
		}
		if cfg.Stats["mp"] < 0 {
			cfg.Stats["mp"] = 0
		}
		out[cfg.ID] = cfg
	}
	s.battleDataMu.Lock()
	s.battleEnemyCache = out
	s.battleEnemyLoaded = true
	s.battleDataMu.Unlock()
	return out
}

func parseBattleDrops(value any) map[string]battleDropRule {
	out := map[string]battleDropRule{}
	rawMap, ok := value.(map[string]any)
	if !ok {
		return out
	}
	for itemID, raw := range rawMap {
		itemID = strings.TrimSpace(itemID)
		if itemID == "" {
			continue
		}
		node, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		rangeNode, _ := node["range"].(map[string]any)
		minVal := toInt64Loose(rangeNode["min"], 0)
		maxVal := toInt64Loose(rangeNode["max"], minVal)
		if minVal < 0 {
			minVal = 0
		}
		if maxVal < minVal {
			maxVal = minVal
		}
		percent := toFloat64(node["percent"], 0)
		if percent < 0 {
			percent = 0
		}
		if percent > 1 {
			percent = 1
		}
		out[itemID] = battleDropRule{Min: minVal, Max: maxVal, Percent: percent}
	}
	return out
}

func (s *Server) parseBattleEnemyEquipment(value any) []battleEnemyEquipment {
	rawList, ok := value.([]any)
	if !ok {
		return nil
	}
	out := make([]battleEnemyEquipment, 0, len(rawList))
	for idx, raw := range rawList {
		node := normalizeAnyMap(raw)
		if len(node) == 0 {
			continue
		}
		templateItemID := battleStringFromAny(firstNonNil(node["templateItemId"], node["itemId"], node["id"]))
		if templateItemID == "" {
			continue
		}
		template := s.deps.GameData.EquipmentTemplateByID(templateItemID)
		if template == nil {
			continue
		}
		equipType := battleStringFromAny(node["equipType"])
		if equipType == "" {
			equipType = strings.TrimSpace(template.EquipType)
		}
		if equipType == "" {
			equipType = "battle"
		}
		equippedSlot := battleStringFromAny(node["equippedSlot"])
		if equippedSlot == "" {
			equippedSlot = strings.TrimSpace(template.Slot)
		}
		maxDurability := int(template.MaxDurability)
		if maxDurability <= 0 {
			maxDurability = 1
		}
		durability := int(toInt64Loose(node["durability"], int64(maxDurability)))
		if durability <= 0 {
			durability = maxDurability
		}
		out = append(out, battleEnemyEquipment{
			EquippedItem: store.EquippedItem{
				InstanceID:     int64(-(idx + 1)),
				TemplateItemID: templateItemID,
				EquipType:      equipType,
				EnhanceLevel:   maxInt(0, int(toInt64Loose(node["enhanceLevel"], 0))),
				Affixes:        parseBattleEnemyEquipmentAffixes(node["affixes"]),
				Durability:     durability,
				MaxDurability:  maxDurability,
				State:          "equipped",
				EquippedSlot:   equippedSlot,
			},
			BaseSnapshot: parseBattleEnemyEquipmentBaseSnapshot(node["baseSnapshot"]),
		})
	}
	return out
}

func parseBattleEnemyEquipmentBaseSnapshot(raw any) map[string]float64 {
	node := normalizeAnyMap(raw)
	if len(node) == 0 {
		return nil
	}
	out := map[string]float64{}
	for key, value := range node {
		attrKey := strings.TrimSpace(key)
		if attrKey == "" {
			continue
		}
		num := toFloat64(value, math.NaN())
		if math.IsNaN(num) || math.IsInf(num, 0) || num == 0 {
			continue
		}
		out[attrKey] = num
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func battleStringFromAny(raw any) string {
	if raw == nil {
		return ""
	}
	if value, ok := raw.(string); ok {
		return strings.TrimSpace(value)
	}
	return strings.TrimSpace(fmt.Sprintf("%v", raw))
}

func parseBattleEnemyEquipmentAffixes(raw any) []store.EquipmentAffixSlot {
	rawList, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := make([]store.EquipmentAffixSlot, 0, len(rawList))
	for _, item := range rawList {
		node := normalizeAnyMap(item)
		if len(node) == 0 {
			continue
		}
		affixID := battleStringFromAny(node["affixId"])
		rarityID := battleStringFromAny(node["rarityId"])
		if affixID == "" {
			continue
		}
		out = append(out, store.EquipmentAffixSlot{
			SlotIndex:      int(toInt64Loose(node["slotIndex"], 0)),
			AffixID:        affixID,
			RarityID:       rarityID,
			RollValue:      toFloat64(node["rollValue"], 0),
			ValuePrecision: toInt64Loose(node["valuePrecision"], 0),
			AttrMods:       parseBattleEnemyEquipmentBaseSnapshot(node["attrMods"]),
			EffectType:     battleStringFromAny(node["effectType"]),
			EffectParams:   normalizeAnyMap(node["effectParams"]),
		})
	}
	return out
}

func battleEnemyStoreEquippedItems(equipment []battleEnemyEquipment) []store.EquippedItem {
	if len(equipment) == 0 {
		return nil
	}
	out := make([]store.EquippedItem, 0, len(equipment))
	for _, entry := range equipment {
		out = append(out, entry.EquippedItem)
	}
	return out
}

func (s *Server) applyBattleEnemyEquipmentStats(stats map[string]float64, equipment []battleEnemyEquipment) {
	if len(equipment) == 0 {
		return
	}
	equipped := battleEnemyStoreEquippedItems(equipment)
	effectiveAffixesByInstance := s.resolveBattleEffectiveAffixesByInstance(equipped)
	for _, entry := range equipment {
		eq := entry.EquippedItem
		if strings.TrimSpace(eq.EquipType) != "battle" {
			continue
		}
		template := s.deps.GameData.EquipmentTemplateByID(strings.TrimSpace(eq.TemplateItemID))
		baseAttrs := entry.BaseSnapshot
		if len(baseAttrs) == 0 && template != nil {
			baseAttrs = template.BaseAttrs
		}
		for attrKey, value := range baseAttrs {
			attrKey = strings.TrimSpace(attrKey)
			if attrKey == "" || math.IsNaN(value) || math.IsInf(value, 0) {
				continue
			}
			stats[attrKey] += value
		}
		if s.deps.Store != nil && eq.EnhanceLevel > 0 {
			for attrKey, value := range s.deps.Store.EquipmentEnhanceCumulativeAttrs(eq.TemplateItemID, eq.EnhanceLevel) {
				attrKey = strings.TrimSpace(attrKey)
				if attrKey == "" || math.IsNaN(value) || math.IsInf(value, 0) {
					continue
				}
				stats[attrKey] += value
			}
		}
		for _, affix := range effectiveAffixesByInstance[eq.InstanceID] {
			for attrKey, value := range affix.AttrMods {
				attrKey = strings.TrimSpace(attrKey)
				if attrKey == "" || math.IsNaN(value) || math.IsInf(value, 0) {
					continue
				}
				stats[attrKey] += value
			}
		}
	}
}

func (s *Server) buildBattleEnemyUnit(cfg battleEnemyConfig, unitID string, rewardEligible bool, defaultBasicSkillID string) battleUnitState {
	enemySkillIDs, enemySkills := s.battleEnemySkillStates(cfg.SkillIDs, defaultBasicSkillID)
	baseStats := cloneFloatMap(cfg.Stats)
	s.applyBattleEnemyEquipmentStats(baseStats, cfg.Equipment)
	projectedStats := s.battleProjectDerivedFinalStats(baseStats)
	equipped := battleEnemyStoreEquippedItems(cfg.Equipment)
	weaponClasses := s.weaponClassSetFromEquippedItems(equipped)
	eqDigest := make([]string, 0, len(equipped))
	eqIDs := make([]int64, 0, len(equipped))
	for _, eq := range equipped {
		if strings.TrimSpace(eq.EquipType) != "battle" {
			continue
		}
		eqDigest = append(eqDigest, fmt.Sprintf("%s+%d", eq.TemplateItemID, eq.EnhanceLevel))
		eqIDs = append(eqIDs, eq.InstanceID)
	}
	enemyUnit := battleUnitState{
		UnitID:          unitID,
		Side:            "enemy",
		OriginSide:      "enemy",
		BaseSide:        "enemy",
		PerspectiveSide: "enemy",
		EnemyID:         cfg.ID,
		EnemyType:       cfg.Type,
		EnemyLevel:      cfg.Level,
		Name:            cfg.Name,
		NameKey:         cfg.NameKey,
		Position:        cfg.Position,
		BasePosition:    cfg.Position,
		Alive:           true,
		RewardEligible:  rewardEligible,
		HP:              maxFloat(1, projectedStats["hp"]),
		MP:              maxFloat(0, projectedStats["mp"]),
		MaxHP:           maxFloat(1, projectedStats["hp"]),
		MaxMP:           maxFloat(0, projectedStats["mp"]),
		BaseStats:       cloneFloatMap(projectedStats),
		Stats:           cloneFloatMap(projectedStats),
		StatMode:        battleStatModeProjected,
		SkillIDs:        enemySkillIDs,
		Skills:          enemySkills,
		EquipmentDigest: eqDigest,
		EquipmentIDs:    eqIDs,
		WeaponClasses:   battleWeaponClassListFromSet(weaponClasses),
		BasicAttack:     s.resolveBattleBasicAttackProfile(equipped),
		EquipmentHooks:  s.resolveBattleEquipmentHooks(equipped),
		EffectMutations: s.resolveBattleAffixEffectMutations(equipped),
	}
	s.battleApplyUnitEquipmentRuntime(&enemyUnit)
	enemyUnit.HP = enemyUnit.MaxHP
	enemyUnit.MP = enemyUnit.MaxMP
	for _, buffID := range cfg.InitialBuffIds {
		dur := s.battleBuffDurationByIDTurns(buffID)
		if dur == 0 {
			continue
		}
		stackPolicy := s.battleBuffStackPolicyByID(buffID)
		battleApplyOrRefreshBuff(&enemyUnit, buffID, dur, nil, "initial", stackPolicy, s.battleBuffMaxStacksByID(buffID))
	}
	return enemyUnit
}

func (s *Server) applyEnemyHpRegenRatio(unit *battleUnitState) {
	if unit.IsPlayerUnit {
		return
	}
	ratio := s.deps.Config.SharedGlobalSettings.CombatBalanceModel.EnemyHpRegenRatio
	if ratio >= 1 {
		return
	}
	if regen, ok := unit.Stats["hp_regen"]; ok && regen > 0 {
		reduced := roundTo4(regen * ratio)
		unit.Stats["hp_regen"] = reduced
		unit.BaseStats["hp_regen"] = reduced
	}
}

func (s *Server) spawnWaveEnemies(ctx context.Context, state *battleRuntimeState, worldCfg battleWorldConfig, enemyCatalog map[string]battleEnemyConfig) {
	defaultBasicSkillID := s.battleDefaultBasicSkillID()
	defer s.appendBattleEquipmentExtraBossRefreshes(state, enemyCatalog, defaultBasicSkillID)
	defer s.appendBattleExtraEnemyRefreshes(state, worldCfg, enemyCatalog, defaultBasicSkillID)
	normalPool := worldCfg.EnemyIDs
	state.CurrentWaveCustomLineupID = ""

	// ── Fog mode: replace enemies with merged pool + player phantom encounter ──
	if state.IsFogMode {
		s.spawnFogWaveEnemies(ctx, state, enemyCatalog, worldCfg, defaultBasicSkillID)
		return
	}

	isSpecialWave := s.isBattleSpecialWave(state, worldCfg)
	if lineup := s.resolveCustomWaveLineup(state, worldCfg, isSpecialWave); len(lineup) > 0 {
		state.Enemies = make([]battleUnitState, 0, len(lineup))
		for i, enemyID := range lineup {
			cfg, ok := enemyCatalog[enemyID]
			if !ok {
				continue
			}
			enemyUnit := s.buildBattleEnemyUnit(cfg, fmt.Sprintf("enemy:%d:c%d:%s", state.Wave, i+1, cfg.ID), false, defaultBasicSkillID)
			s.applyEnemyHpRegenRatio(&enemyUnit)
			state.Enemies = append(state.Enemies, enemyUnit)
		}
		return
	}
	count := s.rollBattleCountRange(state, worldCfg.NormalEnemyCountRange)
	extraPerPlayer := s.rollBattleCountRange(state, worldCfg.ExtraEnemyPerPlayer)
	nonSummonPlayerCount := battleEnemyScalingPlayerCount(state.Players, worldCfg.MaxTeamSize)
	extraPlayers := maxInt(0, nonSummonPlayerCount-1)
	count += extraPlayers * extraPerPlayer
	if count <= 0 {
		count = 1
	}
	specialSpawnIDs := s.resolveSpecialWaveEnemyIDs(state, worldCfg)
	triggerSpawnIDs := s.resolveTriggerItemSpawns(state, worldCfg.ID)
	state.Enemies = make([]battleUnitState, 0, count+len(specialSpawnIDs)+len(triggerSpawnIDs))
	for i := 0; i < count && len(normalPool) > 0; i++ {
		idx := state.randInt(len(normalPool))
		enemyID := normalPool[idx]
		cfg, ok := enemyCatalog[enemyID]
		if !ok {
			continue
		}
		enemyUnit := s.buildBattleEnemyUnit(cfg, fmt.Sprintf("enemy:%d:%d:%s", state.Wave, i+1, cfg.ID), false, defaultBasicSkillID)
		s.applyEnemyHpRegenRatio(&enemyUnit)
		state.Enemies = append(state.Enemies, enemyUnit)
	}
	for index, enemyID := range specialSpawnIDs {
		cfg, exists := enemyCatalog[enemyID]
		if !exists {
			continue
		}
		enemyUnit := s.buildBattleEnemyUnit(cfg, fmt.Sprintf("enemy:%d:s%d:%s", state.Wave, index+1, cfg.ID), false, defaultBasicSkillID)
		s.applyEnemyHpRegenRatio(&enemyUnit)
		state.Enemies = append(state.Enemies, enemyUnit)
	}
	for index, enemyID := range triggerSpawnIDs {
		cfg, exists := enemyCatalog[enemyID]
		if !exists {
			continue
		}
		enemyUnit := s.buildBattleEnemyUnit(cfg, fmt.Sprintf("enemy:%d:t%d:%s", state.Wave, index+1, cfg.ID), false, defaultBasicSkillID)
		s.applyEnemyHpRegenRatio(&enemyUnit)
		state.Enemies = append(state.Enemies, enemyUnit)
	}
	for i := range state.Enemies {
		s.applyBattleWorldDifficultyToEnemyUnit(state, worldCfg, &state.Enemies[i])
	}
}

func (s *Server) applyBattleWorldDifficultyToEnemyUnit(state *battleRuntimeState, worldCfg battleWorldConfig, unit *battleUnitState) {
	if state == nil || unit == nil {
		return
	}
	mult := state.DifficultyMult
	if mult == 1.0 || mult <= 0 {
		return
	}
	var statBonuses []battleWorldStatBonus
	if worldCfg.DifficultySettings != nil {
		statBonuses = worldCfg.DifficultySettings.StatBonuses
	}
	statBonusMap := make(map[string]float64, len(statBonuses))
	for _, bonus := range statBonuses {
		if state.DifficultyLevel >= 1 {
			statBonusMap[bonus.AttrKey] = bonus.Base * (1 + bonus.PerLevelMult*float64(state.DifficultyLevel-1))
		}
	}
	for key, value := range unit.Stats {
		if isCombatStat(key) {
			unit.Stats[key] = (value * mult) + statBonusMap[key]
		}
	}
	for key, value := range unit.BaseStats {
		if isCombatStat(key) {
			unit.BaseStats[key] = (value * mult) + statBonusMap[key]
		}
	}
	unit.MaxHP = maxFloat(1, unit.Stats["hp"])
	unit.MaxMP = maxFloat(0, unit.Stats["mp"])
	unit.HP = unit.MaxHP
	unit.MP = unit.MaxMP
}

func (s *Server) appendBattleExtraEnemyRefreshes(
	state *battleRuntimeState,
	worldCfg battleWorldConfig,
	enemyCatalog map[string]battleEnemyConfig,
	defaultBasicSkillID string,
) {
	if s == nil || state == nil || len(worldCfg.EnemyIDs) == 0 {
		return
	}
	count := s.battleExtraEnemyRefreshCount(state)
	if count <= 0 {
		return
	}
	for index := 0; index < count; index++ {
		enemyID := worldCfg.EnemyIDs[state.randInt(len(worldCfg.EnemyIDs))]
		cfg, ok := enemyCatalog[enemyID]
		if !ok {
			continue
		}
		unitID := fmt.Sprintf("enemy:%d:extra%d:%s", state.Wave, index+1, cfg.ID)
		enemyUnit := s.buildBattleEnemyUnit(cfg, unitID, false, defaultBasicSkillID)
		s.applyEnemyHpRegenRatio(&enemyUnit)
		s.applyBattleWorldDifficultyToEnemyUnit(state, worldCfg, &enemyUnit)
		state.Enemies = append(state.Enemies, enemyUnit)
	}
}

func (s *Server) battleExtraEnemyRefreshCount(state *battleRuntimeState) int {
	if s == nil || state == nil {
		return 0
	}
	total := 0
	for playerIndex := range state.Players {
		player := &state.Players[playerIndex]
		if battleUnitIsSummon(player) || !player.Alive || player.HP <= 0 {
			continue
		}
		for buffIndex := range player.Buffs {
			buff := &player.Buffs[buffIndex]
			if buff.RemainTurns == 0 {
				continue
			}
			buffID := strings.ToLower(strings.TrimSpace(buff.BuffID))
			if buffID == "" {
				continue
			}
			chargesRemaining := battleBuffChargesRemaining(buff, s.battleBuffMaxCharges(buffID))
			for _, rule := range s.battleFindBuffRules(buffID, "on_enemy_refresh") {
				if !battleBuffRuleLimitAllowed(state, player, rule) ||
					!battleRuleConditionsPassed(rule, player, nil, float64(state.Wave), "", buff, chargesRemaining) {
					continue
				}
				ruleCount := 0
				for effectIndex, effect := range rule.Effects {
					kind := ""
					if effectIndex < len(rule.EffectKinds) {
						kind = rule.EffectKinds[effectIndex]
					} else {
						kind = battleNormalizeTrigger(fmt.Sprintf("%v", firstNonNil(effect["kind"], effect["type"])))
					}
					if kind != battleEffectExtraEnemyRefresh {
						continue
					}
					pool := battleNormalizeTrigger(fmt.Sprintf("%v", effect["enemyPool"]))
					if pool != battleExtraEnemyPoolCurrentWorld {
						continue
					}
					ruleCount += maxInt(0, int(toInt64Loose(effect["count"], 0)))
				}
				if ruleCount <= 0 {
					continue
				}
				battleMarkBuffRuleTriggered(state, player, rule)
				total = minInt(battleMaxExtraEnemyRefreshCount, total+ruleCount)
				if total >= battleMaxExtraEnemyRefreshCount {
					return total
				}
			}
		}
	}
	return total
}

// resolveTriggerItemSpawns scans all player BattleItems for trigger items (IsTrigger = true),
// rolls per-wave chance for each eligible one, and returns enemy IDs to spawn this wave.
// Trigger items are consumed (Quantity--) when they fire, matching the normal item-use settlement path.
func (s *Server) resolveTriggerItemSpawns(state *battleRuntimeState, dungeonID string) []string {
	if state == nil {
		return nil
	}
	var spawns []string
	grouped := map[string][]battleTriggerItemCandidate{}
	for playerIdx := range state.Players {
		player := &state.Players[playerIdx]
		if battleUnitIsSummon(player) {
			continue
		}
		for itemIdx := range player.BattleItems {
			item := &player.BattleItems[itemIdx]
			if !battleTriggerItemBaseEligible(item, state.Wave, dungeonID) {
				continue
			}
			group := strings.ToLower(strings.TrimSpace(item.PartyTriggerGroup))
			if group != "" {
				grouped[group] = append(grouped[group], battleTriggerItemCandidate{
					PlayerIndex: playerIdx,
					ItemIndex:   itemIdx,
					UserID:      player.UserID,
					ItemID:      item.ItemID,
				})
				continue
			}
			if item.IntervalWaves > 0 && state.Wave-item.LastSpawnWave < item.IntervalWaves {
				continue
			}
			if state.randFloat64() >= item.PerWaveChance {
				continue
			}
			spawns = append(spawns, item.SpawnEnemyIDs[state.randInt(len(item.SpawnEnemyIDs))])
			item.LastSpawnWave = state.Wave
			item.Quantity--
		}
	}

	groupIDs := make([]string, 0, len(grouped))
	for groupID := range grouped {
		groupIDs = append(groupIDs, groupID)
	}
	sort.Strings(groupIDs)
	for _, groupID := range groupIDs {
		candidates := grouped[groupID]
		sort.SliceStable(candidates, func(i, j int) bool {
			if candidates[i].UserID != candidates[j].UserID {
				return candidates[i].UserID < candidates[j].UserID
			}
			if candidates[i].ItemID != candidates[j].ItemID {
				return candidates[i].ItemID < candidates[j].ItemID
			}
			if candidates[i].PlayerIndex != candidates[j].PlayerIndex {
				return candidates[i].PlayerIndex < candidates[j].PlayerIndex
			}
			return candidates[i].ItemIndex < candidates[j].ItemIndex
		})
		winnerRef := candidates[0]
		winner := &state.Players[winnerRef.PlayerIndex].BattleItems[winnerRef.ItemIndex]
		groupLastSpawnWave := battleTriggerGroupLastSpawnWave(state, groupID)
		if winner.IntervalWaves > 0 && state.Wave-groupLastSpawnWave < winner.IntervalWaves {
			continue
		}
		if state.randFloat64() >= winner.PerWaveChance {
			continue
		}
		spawns = append(spawns, battlePickTriggerEnemyID(state, winner.SpawnEnemyIDs))
		winner.Quantity--
		battleMarkTriggerGroupSpawned(state, groupID, state.Wave)
	}
	return spawns
}

type battleTriggerItemCandidate struct {
	PlayerIndex int
	ItemIndex   int
	UserID      int64
	ItemID      string
}

func battleTriggerItemBaseEligible(item *battleItemState, wave int, dungeonID string) bool {
	if item == nil || !item.IsTrigger || item.Quantity <= 0 || len(item.SpawnEnemyIDs) == 0 || wave < item.MinWave {
		return false
	}
	for _, allowedDungeonID := range item.DungeonIDs {
		if strings.EqualFold(allowedDungeonID, dungeonID) {
			return true
		}
	}
	return false
}

func battlePickTriggerEnemyID(state *battleRuntimeState, enemyIDs []string) string {
	if len(enemyIDs) == 1 {
		return enemyIDs[0]
	}
	return enemyIDs[state.randInt(len(enemyIDs))]
}

func battleTriggerGroupLastSpawnWave(state *battleRuntimeState, groupID string) int {
	lastWave := 0
	for playerIdx := range state.Players {
		for itemIdx := range state.Players[playerIdx].BattleItems {
			item := &state.Players[playerIdx].BattleItems[itemIdx]
			if strings.EqualFold(strings.TrimSpace(item.PartyTriggerGroup), groupID) {
				lastWave = maxInt(lastWave, item.LastSpawnWave)
			}
		}
	}
	return lastWave
}

func battleMarkTriggerGroupSpawned(state *battleRuntimeState, groupID string, wave int) {
	for playerIdx := range state.Players {
		for itemIdx := range state.Players[playerIdx].BattleItems {
			item := &state.Players[playerIdx].BattleItems[itemIdx]
			if strings.EqualFold(strings.TrimSpace(item.PartyTriggerGroup), groupID) {
				item.LastSpawnWave = wave
			}
		}
	}
}

func (s *Server) rollBattleCountRange(state *battleRuntimeState, cfg battleCountRange) int {
	minCount := maxInt(0, cfg.Min)
	maxCount := maxInt(minCount, cfg.Max)
	if maxCount == minCount {
		return minCount
	}
	return minCount + state.randInt(maxCount-minCount+1)
}

func (s *Server) isBattleSpecialWave(state *battleRuntimeState, worldCfg battleWorldConfig) bool {
	if state == nil {
		return false
	}
	if _, ok := worldCfg.SpecialWaves[state.Wave]; ok {
		return true
	}
	if _, ok := matchingBattleWaveSchedule(state.Wave, worldCfg.WaveSchedules); ok {
		return true
	}
	return worldCfg.SpecialWaveEvery > 0 && state.Wave > 0 && state.Wave%worldCfg.SpecialWaveEvery == 0
}

func (s *Server) resolveCustomWaveLineup(state *battleRuntimeState, worldCfg battleWorldConfig, isSpecialWave bool) []string {
	if state == nil || len(worldCfg.CustomLineupConf) == 0 {
		return nil
	}
	matched := make([]battleCustomLineupConfig, 0, len(worldCfg.CustomLineupConf))
	for _, conf := range worldCfg.CustomLineupConf {
		if conf.IsSpecialWaveLineup != isSpecialWave || len(conf.Lineup) == 0 {
			continue
		}
		if state.randFloat64() > clampFloat(conf.Percent, 0, 1) {
			continue
		}
		matched = append(matched, conf)
	}
	if len(matched) == 0 {
		return nil
	}
	selected := matched[state.randInt(len(matched))]
	state.CurrentWaveCustomLineupID = selected.ID
	return append([]string(nil), selected.Lineup...)
}

func (s *Server) resolveSpecialWaveEnemyIDs(state *battleRuntimeState, worldCfg battleWorldConfig) []string {
	if state == nil {
		return nil
	}
	ids := make([]string, 0, len(worldCfg.SpecialEnemyIDs)+4)
	if len(worldCfg.SpecialEnemyIDs) > 0 {
		if _, ok := worldCfg.SpecialWaves[state.Wave]; ok {
			ids = append(ids, worldCfg.SpecialEnemyIDs...)
		}
	}
	if len(worldCfg.SpecialEnemyIDs) > 0 && worldCfg.SpecialWaveEvery > 0 && state.Wave > 0 && state.Wave%worldCfg.SpecialWaveEvery == 0 {
		minCount := maxInt(1, worldCfg.SpecialWaveMinCount)
		maxCount := maxInt(minCount, worldCfg.SpecialWaveMaxCount)
		count := minCount
		if maxCount > minCount {
			count += state.randInt(maxCount - minCount + 1)
		}
		for i := 0; i < count; i++ {
			ids = append(ids, worldCfg.SpecialEnemyIDs[state.randInt(len(worldCfg.SpecialEnemyIDs))])
		}
	}
	ids = append(ids, s.resolveWaveScheduleEnemyIDs(state, worldCfg)...)
	return ids
}

func (s *Server) battleEnemySkillStates(skillIDs []string, fallbackSkillID string) ([]string, []battleSkillState) {
	resolvedIDs := make([]string, 0, len(skillIDs)+1)
	for _, skillID := range skillIDs {
		normalized := strings.TrimSpace(skillID)
		if normalized == "" || !s.battleSkillExists(normalized) {
			continue
		}
		resolvedIDs = appendUniqueSkillID(resolvedIDs, normalized)
	}
	fallbackSkillID = strings.TrimSpace(fallbackSkillID)
	if len(resolvedIDs) == 0 && fallbackSkillID != "" {
		resolvedIDs = appendUniqueSkillID(resolvedIDs, fallbackSkillID)
	}
	if fallbackSkillID != "" && len(resolvedIDs) > 1 {
		others := make([]string, 0, len(resolvedIDs))
		hasFallback := false
		for _, id := range resolvedIDs {
			if id == fallbackSkillID {
				hasFallback = true
				continue
			}
			others = append(others, id)
		}
		if hasFallback {
			resolvedIDs = append(others, fallbackSkillID)
		}
	}
	out := make([]battleSkillState, 0, len(resolvedIDs))
	for _, skillID := range resolvedIDs {
		out = append(out, s.battleSkillByID(skillID))
	}
	return resolvedIDs, out
}

func (s *Server) resolveBattleWaveSettlement(state battleRuntimeState, worldCfg battleWorldConfig, now time.Time) battleWaveSettlement {
	rewardByUser := map[int64]store.BattleWaveReward{}
	summaryByUser := map[int64]map[string]any{}
	enemyCatalog := s.battleEnemyCatalog()
	totalEnemyExp := 0.0
	defeated := make([]battleUnitState, 0, len(state.Enemies))
	for _, enemy := range state.Enemies {
		if enemy.Alive {
			continue
		}
		defeated = append(defeated, enemy)
		if cfg, ok := enemyCatalog[enemy.EnemyID]; ok {
			totalEnemyExp += cfg.Exp
		}
	}
	if len(defeated) == 0 {
		return battleWaveSettlement{Rewards: rewardByUser, ByUser: summaryByUser}
	}
	elapsedSec := 0.0
	if state.StartedAt > 0 {
		elapsedSec = now.Sub(time.UnixMilli(state.StartedAt)).Seconds()
	}
	expMultiplier := s.battleExpMultiplier(elapsedSec)
	unitsByOwner := map[int64][]battleUnitState{}
	playerUnitByOwner := map[int64]battleUnitState{}
	for _, player := range state.Players {
		if player.UserID <= 0 || !player.RewardEligible {
			continue
		}
		unitsByOwner[player.UserID] = append(unitsByOwner[player.UserID], player)
		if player.UnitType != "kitty" {
			playerUnitByOwner[player.UserID] = player
		}
	}
	// Wave-level drops: rolled once for the whole room, then every player receives the same result.
	// In fog mode, waveDrops are intentionally skipped.
	roomWaveDrops := map[string]int64{}
	for itemID, rule := range worldCfg.WaveDrops {
		if state.IsFogMode {
			continue
		}
		if state.randFloat64() > clampFloat(rule.Percent, 0, 1) {
			continue
		}
		dropQty := rule.Min
		if rule.Max > rule.Min {
			dropQty += int64(state.randInt(int(rule.Max-rule.Min) + 1))
		}
		if dropQty > 0 {
			roomWaveDrops[itemID] = dropQty
		}
	}
	for userID, player := range playerUnitByOwner {
		playerReward := store.BattleWaveReward{
			ItemRewards:  map[string]int64{},
			AttrExp:      map[string]float64{},
			KittyAttrExp: map[int64]map[string]float64{},
		}
		dropBonusRate := player.Stats["battle_drop_rate_bonus"]
		expBonusRate := player.Stats["battle_exp_bonus"]
		difficultyDropBonusRate := 0.0
		difficultyExpBonusRate := 0.0
		if state.DifficultyLevel > 0 {
			for _, dl := range s.deps.Config.SharedGlobalSettings.CombatDifficulty.Levels {
				if dl.Level != state.DifficultyLevel {
					continue
				}
				difficultyDropBonusRate = dl.BattleDropRateBonus
				difficultyExpBonusRate = dl.BattleExpBonus
				break
			}
		}
		totalExpMult := maxFloat(0, 1.0+expBonusRate+difficultyExpBonusRate)
		luck := player.Stats["luck"]
		for _, enemy := range defeated {
			cfg, ok := enemyCatalog[enemy.EnemyID]
			if !ok {
				continue
			}
			for itemID, rule := range cfg.Drops {
				percent := battleSettlementDropPercent(rule.Percent, dropBonusRate, difficultyDropBonusRate, luck, s.deps.Config.GlobalSettings.Battle.DropLuckFactor)
				if state.randFloat64() > percent {
					continue
				}
				dropQty := rule.Min
				if rule.Max > rule.Min {
					dropQty += int64(state.randInt(int(rule.Max-rule.Min) + 1))
				}
				if dropQty <= 0 {
					continue
				}
				playerReward.ItemRewards[itemID] += dropQty
			}
		}
		if state.CurrentWaveCustomLineupID != "" {
			if lineupCfg, ok := s.findBattleCustomLineupByID(worldCfg, state.CurrentWaveCustomLineupID); ok {
				for itemID, rule := range lineupCfg.Drops.Items {
					percent := battleSettlementDropPercent(rule.Percent, dropBonusRate, difficultyDropBonusRate, luck, s.deps.Config.GlobalSettings.Battle.DropLuckFactor)
					if state.randFloat64() > percent {
						continue
					}
					dropQty := rule.Min
					if rule.Max > rule.Min {
						dropQty += int64(state.randInt(int(rule.Max-rule.Min) + 1))
					}
					if dropQty <= 0 {
						continue
					}
					playerReward.ItemRewards[itemID] += dropQty
				}
			}
		}
		// Distribute room-level wave drops (rolled once for the whole room above)
		for itemID, qty := range roomWaveDrops {
			playerReward.ItemRewards[itemID] += qty
		}
		expGain := totalEnemyExp * expMultiplier * totalExpMult
		if expGain > 0 {
			playerReward.AttrExp["battle"] = expGain
			kittyExpMultiplier := maxFloat(0, s.deps.Config.SharedGlobalSettings.Kitty.BattleAttrExpMultiplier)
			if kittyExpMultiplier > 0 {
				for _, unit := range unitsByOwner[userID] {
					if unit.UnitType != "kitty" || unit.KittyID <= 0 {
						continue
					}
					if _, ok := playerReward.KittyAttrExp[unit.KittyID]; !ok {
						playerReward.KittyAttrExp[unit.KittyID] = map[string]float64{}
					}
					playerReward.KittyAttrExp[unit.KittyID]["battle"] += expGain * kittyExpMultiplier
				}
			}
		}
		// World-specific special drops (independent of bonus rates)
		if state.DifficultyLevel > 0 && worldCfg.DifficultySettings != nil {
			for _, sd := range worldCfg.DifficultySettings.SpecialDrops {
				if state.DifficultyLevel < sd.MinLevel {
					continue
				}
				if state.randFloat64() < sd.Percent {
					dropQty := sd.RangeMin
					if sd.RangeMax > sd.RangeMin {
						dropQty += int64(state.randInt(int(sd.RangeMax-sd.RangeMin) + 1))
					}
					playerReward.ItemRewards[sd.ItemID] += dropQty
				}
			}
		}
		rewardByUser[userID] = playerReward
		summaryByUser[userID] = map[string]any{
			"items":          playerReward.ItemRewards,
			"attrExp":        playerReward.AttrExp,
			"kittyAttrExp":   playerReward.KittyAttrExp,
			"rewardEligible": true,
			"expMultiplier":  expMultiplier,
		}
	}

	// Fog player drops: awarded once per player when a player-phantom wave is defeated.
	if state.IsFogMode && isWaveFogPlayerPhantom(state.Enemies) {
		for _, rule := range s.deps.Config.SharedGlobalSettings.FogMode.FogPlayerDrops {
			if rule.ItemID == "" || rule.Percent <= 0 {
				continue
			}
			if state.randFloat64() >= rule.Percent {
				continue
			}
			qty := rule.Min
			if rule.Max > rule.Min {
				qty += int64(state.randInt(int(rule.Max - rule.Min + 1)))
			}
			if qty <= 0 {
				continue
			}
			for userID := range rewardByUser {
				r := rewardByUser[userID]
				r.ItemRewards[rule.ItemID] += qty
				rewardByUser[userID] = r
			}
		}
	}

	if err := s.appendBattlePassiveRewards(state, worldCfg, defeated, playerUnitByOwner, rewardByUser, summaryByUser); err != nil {
		return battleWaveSettlement{Err: err}
	}
	return battleWaveSettlement{Rewards: rewardByUser, ByUser: summaryByUser}
}

func battleSettlementDropPercent(basePercent, dropBonusRate, difficultyDropBonusRate, luck, dropLuckFactor float64) float64 {
	playerMultiplier := (1 + luck*dropLuckFactor) * maxFloat(0, 1+dropBonusRate)
	effectiveMultiplier := maxFloat(0, playerMultiplier+difficultyDropBonusRate)
	return clampFloat(basePercent*effectiveMultiplier, 0, 1)
}

func (s *Server) findBattleCustomLineupByID(worldCfg battleWorldConfig, lineupID string) (battleCustomLineupConfig, bool) {
	lineupID = strings.TrimSpace(lineupID)
	if lineupID == "" {
		return battleCustomLineupConfig{}, false
	}
	for _, conf := range worldCfg.CustomLineupConf {
		if strings.TrimSpace(conf.ID) == lineupID {
			return conf, true
		}
	}
	return battleCustomLineupConfig{}, false
}

func mergeBattleWaveRewards(target map[int64]store.BattleWaveReward, incoming map[int64]store.BattleWaveReward) error {
	if len(incoming) == 0 {
		return nil
	}
	for userID, reward := range incoming {
		if userID <= 0 {
			continue
		}
		merged, ok := target[userID]
		if !ok {
			merged = store.BattleWaveReward{
				ItemRewards:  map[string]int64{},
				AttrExp:      map[string]float64{},
				KittyAttrExp: map[int64]map[string]float64{},
			}
		}
		var err error
		merged.Contributions, err = operationeffect.MergeContributions(merged.Contributions, reward.Contributions)
		if err != nil {
			return err
		}
		for itemID, qty := range reward.ItemRewards {
			if qty <= 0 {
				continue
			}
			merged.ItemRewards[itemID] += qty
		}
		for attrKey, exp := range reward.AttrExp {
			if exp <= 0 {
				continue
			}
			merged.AttrExp[attrKey] += exp
		}
		if merged.KittyAttrExp == nil {
			merged.KittyAttrExp = map[int64]map[string]float64{}
		}
		for kittyID, attrs := range reward.KittyAttrExp {
			if kittyID <= 0 {
				continue
			}
			if _, ok := merged.KittyAttrExp[kittyID]; !ok {
				merged.KittyAttrExp[kittyID] = map[string]float64{}
			}
			for attrKey, exp := range attrs {
				if exp <= 0 {
					continue
				}
				merged.KittyAttrExp[kittyID][attrKey] += exp
			}
		}
		target[userID] = merged
	}
	return nil
}

func (s *Server) battleExpMultiplier(elapsedSec float64) float64 {
	cfg := s.deps.Config.GlobalSettings.Battle
	start := float64(cfg.ExpBonusStartSeconds)
	perSec := cfg.ExpBonusPerSecond
	maxM := cfg.ExpBonusMaxMultiplier
	if maxM < 1 {
		maxM = 1
	}
	if elapsedSec <= start || perSec <= 0 {
		return 1
	}
	extra := (elapsedSec - start) * perSec
	return clampFloat(1+extra, 1, maxM)
}

func (s *Server) updateBattlePlayerRewardEligibility(ctx context.Context, state *battleRuntimeState, now time.Time) {
	offlineMax := time.Duration(s.deps.Config.GlobalSettings.Activity.OfflineProgressMaxDurationSeconds) * time.Second
	if offlineMax <= 0 {
		offlineMax = 24 * time.Hour
	}
	for index := range state.Players {
		player := &state.Players[index]
		if player.UserID <= 0 {
			player.RewardEligible = false
			continue
		}
		presence, err := s.deps.Store.GetPresence(ctx, player.UserID)
		if err != nil {
			continue
		}
		player.RewardEligible = now.Sub(presence.LastSeenAt) <= offlineMax
	}
}

func (s *Server) saveBattleRuntimeState(ctx context.Context, runID int64, state battleRuntimeState, status string, endedAt *time.Time) error {
	if status == "" {
		status = store.BattleRunStatusRunning
	}
	payload := state.toMap()
	if endedAt != nil || status != store.BattleRunStatusRunning {
		payload = battleSettlementStatePayload(state, status)
	}
	return s.deps.Store.SaveBattleRunState(ctx, runID, state.Wave, status, payload, endedAt)
}

func (s *Server) battleRuntimeState(runID int64) (battleRuntimeState, bool) {
	if runID <= 0 {
		return battleRuntimeState{}, false
	}
	s.battleStateMu.RLock()
	state, ok := s.battleStateMem[runID]
	s.battleStateMu.RUnlock()
	if !ok {
		return battleRuntimeState{}, false
	}
	return cloneBattleRuntimeState(state), true
}

func (s *Server) touchBattlePlaybackLocalCache(runID int64, now time.Time) {
	if runID <= 0 {
		return
	}
	s.battlePlaybackTouch[runID] = now
}

func (s *Server) maybeSweepBattlePlaybackLocalCache(now time.Time) {
	if now.IsZero() {
		now = time.Now()
	}
	expired := make([]int64, 0)
	s.battlePlaybackMu.Lock()
	if now.Sub(s.battlePlaybackSweep) >= battlePlaybackCacheSweepInterval {
		for runID, touchedAt := range s.battlePlaybackTouch {
			if now.Sub(touchedAt) <= battlePlaybackCacheEntryTTL {
				continue
			}
			expired = append(expired, runID)
			delete(s.battlePlaybackTouch, runID)
			delete(s.battlePlaybackMem, runID)
		}
		s.battlePlaybackSweep = now
	}
	s.battlePlaybackMu.Unlock()
	if len(expired) == 0 {
		return
	}
	s.battleCheckpointMu.Lock()
	for _, runID := range expired {
		delete(s.battleCheckpointMem, runID)
	}
	s.battleCheckpointMu.Unlock()
}

func (s *Server) setBattleRuntimeState(runID int64, state battleRuntimeState) {
	if runID <= 0 {
		return
	}
	s.battleStateMu.Lock()
	s.battleStateMem[runID] = cloneBattleRuntimeState(state)
	s.battleStateMu.Unlock()
}

func (s *Server) clearBattleRuntimeState(runID int64) {
	if runID <= 0 {
		return
	}
	s.battleStateMu.Lock()
	delete(s.battleStateMem, runID)
	s.battleStateMu.Unlock()
	s.clearBattleRunPlaybackCache(runID)
}

func (s *Server) loadBattleRunPlaybackCache(ctx context.Context, runID int64) ([]wsbus.BattleTimelineEvent, error) {
	if runID <= 0 {
		return []wsbus.BattleTimelineEvent{}, nil
	}
	s.battlePlaybackMu.RLock()
	cached, ok := s.battlePlaybackMem[runID]
	s.battlePlaybackMu.RUnlock()
	if ok {
		s.battlePlaybackMu.Lock()
		s.touchBattlePlaybackLocalCache(runID, time.Now())
		s.battlePlaybackMu.Unlock()
		return cached.Timeline, nil
	}
	timelineRaw, err := s.deps.Store.GetBattleRunPlaybackTimelineByID(ctx, runID)
	if err != nil {
		return nil, err
	}
	decodedTimeline := decodeBattleTimeline(timelineRaw)
	s.battlePlaybackMu.Lock()
	s.battlePlaybackMem[runID] = battleRunPlaybackCache{
		Timeline: decodedTimeline,
	}
	s.touchBattlePlaybackLocalCache(runID, time.Now())
	s.battlePlaybackMu.Unlock()
	return decodedTimeline, nil
}

func (s *Server) clearBattleRunPlaybackCache(runID int64) {
	if runID <= 0 {
		return
	}
	s.battlePlaybackMu.Lock()
	delete(s.battlePlaybackMem, runID)
	delete(s.battlePlaybackTouch, runID)
	s.battlePlaybackMu.Unlock()
	s.clearBattleRunPlaybackCheckpoint(runID)
}

func (s *Server) setBattleRunPlaybackCache(runID int64, timeline []wsbus.BattleTimelineEvent) {
	if runID <= 0 {
		return
	}
	s.battlePlaybackMu.Lock()
	s.battlePlaybackMem[runID] = battleRunPlaybackCache{
		Timeline: timeline,
	}
	s.touchBattlePlaybackLocalCache(runID, time.Now())
	s.battlePlaybackMu.Unlock()
}

func (s *Server) loadBattleRunPlaybackCheckpoint(runID int64, fallback map[string]any, now time.Time) battlePlaybackCheckpoint {
	if runID <= 0 {
		return decodeBattlePlaybackCheckpoint(fallback, now)
	}
	s.battleCheckpointMu.RLock()
	checkpoint, ok := s.battleCheckpointMem[runID]
	s.battleCheckpointMu.RUnlock()
	if ok {
		s.battlePlaybackMu.Lock()
		s.touchBattlePlaybackLocalCache(runID, time.Now())
		s.battlePlaybackMu.Unlock()
		return checkpoint
	}
	if s.deps.Redis != nil {
		loadStartedAt := time.Now()
		raw, err := s.deps.Redis.Get(context.Background(), battlePlaybackCheckpointRedisKey(runID)).Result()
		loadElapsed := time.Since(loadStartedAt)
		if loadElapsed >= battlePlaybackCheckpointRedisLoadSlowThreshold &&
			(loadElapsed >= battlePlaybackCheckpointRedisLoadSevereThreshold || shouldSampleBattlePlaybackCadenceLog(runID, time.Now())) {
			log.Printf(
				"battle playback checkpoint redis load slow run_id=%d elapsed_ms=%d hit=%t err=%v",
				runID,
				loadElapsed.Milliseconds(),
				err == nil && strings.TrimSpace(raw) != "",
				err,
			)
		}
		if err == nil && strings.TrimSpace(raw) != "" {
			node := map[string]any{}
			if json.Unmarshal([]byte(raw), &node) == nil {
				checkpoint = decodeBattlePlaybackCheckpoint(node, now)
				s.setBattleRunPlaybackCheckpoint(runID, checkpoint)
				return checkpoint
			}
		} else if err != nil && !errors.Is(err, redis.Nil) {
			log.Printf("battle playback checkpoint redis load failed run_id=%d err=%v", runID, err)
		}
	}
	checkpoint = decodeBattlePlaybackCheckpoint(fallback, now)
	s.setBattleRunPlaybackCheckpoint(runID, checkpoint)
	return checkpoint
}

// Seed only the local cache after this process acquires the playback handoff,
// so the first snapshot does not wait on Redis. Never replace a cursor that a
// concurrent local dispatch may already have advanced.
func (s *Server) seedBattleRunPlaybackCheckpointLocal(runID int64, checkpoint battlePlaybackCheckpoint) {
	if runID <= 0 {
		return
	}
	checkpoint.RedisPersistedAtMs = 0
	s.battleCheckpointMu.Lock()
	if s.battleCheckpointMem == nil {
		s.battleCheckpointMem = make(map[int64]battlePlaybackCheckpoint)
	}
	if _, exists := s.battleCheckpointMem[runID]; !exists {
		s.battleCheckpointMem[runID] = checkpoint
	}
	s.battleCheckpointMu.Unlock()
	s.battlePlaybackMu.Lock()
	if s.battlePlaybackTouch == nil {
		s.battlePlaybackTouch = make(map[int64]time.Time)
	}
	s.touchBattlePlaybackLocalCache(runID, time.Now())
	s.battlePlaybackMu.Unlock()
}

func (s *Server) setBattleRunPlaybackCheckpoint(runID int64, checkpoint battlePlaybackCheckpoint) {
	if runID <= 0 {
		return
	}
	nowMs := time.Now().UnixMilli()
	s.battleCheckpointMu.Lock()
	previous, hadPrevious := s.battleCheckpointMem[runID]
	if checkpoint.RedisPersistedAtMs <= 0 {
		checkpoint.RedisPersistedAtMs = previous.RedisPersistedAtMs
	}
	shouldPersist := !hadPrevious ||
		checkpoint.RedisPersistedAtMs <= 0 ||
		checkpoint.LastPushedSeq != previous.LastPushedSeq ||
		checkpoint.NextEventIdx != previous.NextEventIdx ||
		playbackCheckpointAtTimelineEndCursor(checkpoint)
	if shouldPersist {
		checkpoint.RedisPersistedAtMs = nowMs
	}
	s.battleCheckpointMem[runID] = checkpoint
	s.battleCheckpointMu.Unlock()
	s.battlePlaybackMu.Lock()
	s.touchBattlePlaybackLocalCache(runID, time.Now())
	s.battlePlaybackMu.Unlock()
	if shouldPersist {
		s.persistBattleRunPlaybackCheckpoint(runID, checkpoint)
	}
}

func (s *Server) clearBattleRunPlaybackCheckpoint(runID int64) {
	if runID <= 0 {
		return
	}
	s.battleCheckpointMu.Lock()
	delete(s.battleCheckpointMem, runID)
	s.battleCheckpointMu.Unlock()
	if s.deps.Redis != nil && s.battleLoopEnabled {
		if err := s.deps.Redis.Del(context.Background(), battlePlaybackCheckpointRedisKey(runID)).Err(); err != nil {
			log.Printf("battle playback checkpoint redis clear failed run_id=%d err=%v", runID, err)
		}
	}
}

func (s *Server) persistBattleRunPlaybackCheckpoint(runID int64, checkpoint battlePlaybackCheckpoint) {
	if runID <= 0 || s.deps.Redis == nil || !s.battleLoopEnabled {
		return
	}
	raw, err := json.Marshal(checkpoint.cursorMap())
	if err != nil {
		return
	}
	if err := s.deps.Redis.Set(context.Background(), battlePlaybackCheckpointRedisKey(runID), string(raw), battlePlaybackCheckpointTTL).Err(); err != nil {
		log.Printf("battle playback checkpoint redis save failed run_id=%d err=%v", runID, err)
	}
}

func playbackCheckpointAtTimelineEndCursor(checkpoint battlePlaybackCheckpoint) bool {
	return checkpoint.NextEventIdx > 0 && checkpoint.NextPushAtMs <= 0
}

func battlePlaybackCheckpointRedisKey(runID int64) string {
	return "battle:playback:checkpoint:" + strconv.FormatInt(runID, 10)
}

func decodeBattleRuntimeState(raw map[string]any) battleRuntimeState {
	if len(raw) == 0 {
		return battleRuntimeState{}
	}
	buf, err := json.Marshal(raw)
	if err != nil {
		return battleRuntimeState{}
	}
	var state battleRuntimeState
	if err := json.Unmarshal(buf, &state); err != nil {
		return battleRuntimeState{}
	}
	if state.Wave <= 0 {
		state.Wave = 1
	}
	if state.Events == nil {
		state.Events = []battleEvent{}
	}
	if state.Players == nil {
		state.Players = []battleUnitState{}
	}
	if state.Enemies == nil {
		state.Enemies = []battleUnitState{}
	}
	if state.Order == nil {
		state.Order = []string{}
	}
	if state.PreparationOrder == nil {
		state.PreparationOrder = []string{}
	}
	if state.PreparationSeedOrder == nil {
		state.PreparationSeedOrder = []string{}
	}
	if state.ActedUnits == nil {
		state.ActedUnits = []string{}
	}
	state.ExtraTurnQueue = normalizeBattleQueuedExtraTurns(state.ExtraTurnQueue)
	for index := range state.BattlefieldEffects {
		state.BattlefieldEffects[index].Windows = cloneBattlefieldWindows(state.BattlefieldEffects[index].Windows)
	}
	for index := range state.Players {
		if state.Players[index].Stats == nil {
			state.Players[index].Stats = map[string]float64{}
		}
		normalizeBattleUnitState(&state.Players[index])
	}
	for index := range state.Enemies {
		if state.Enemies[index].Stats == nil {
			state.Enemies[index].Stats = map[string]float64{}
		}
		normalizeBattleUnitState(&state.Enemies[index])
	}
	return state
}

func cloneBattleRuntimeState(source battleRuntimeState) battleRuntimeState {
	buf, err := json.Marshal(source)
	if err != nil {
		return source
	}
	var cloned battleRuntimeState
	if err := json.Unmarshal(buf, &cloned); err != nil {
		return source
	}
	for index := range cloned.Players {
		if cloned.Players[index].Stats == nil {
			cloned.Players[index].Stats = map[string]float64{}
		}
		normalizeBattleUnitState(&cloned.Players[index])
	}
	for index := range cloned.Enemies {
		if cloned.Enemies[index].Stats == nil {
			cloned.Enemies[index].Stats = map[string]float64{}
		}
		normalizeBattleUnitState(&cloned.Enemies[index])
	}
	if cloned.Events == nil {
		cloned.Events = []battleEvent{}
	}
	if cloned.Order == nil {
		cloned.Order = []string{}
	}
	if cloned.PreparationOrder == nil {
		cloned.PreparationOrder = []string{}
	}
	if cloned.PreparationSeedOrder == nil {
		cloned.PreparationSeedOrder = []string{}
	}
	if cloned.ActedUnits == nil {
		cloned.ActedUnits = []string{}
	}
	cloned.ExtraTurnQueue = normalizeBattleQueuedExtraTurns(cloned.ExtraTurnQueue)
	for index := range cloned.BattlefieldEffects {
		cloned.BattlefieldEffects[index].Windows = cloneBattlefieldWindows(cloned.BattlefieldEffects[index].Windows)
	}
	cloned.battlefieldEffectDefinitions = append([]battleWorldBattlefieldEffect(nil), source.battlefieldEffectDefinitions...)
	cloned.battlefieldSpecialEnemyIDs = maps.Clone(source.battlefieldSpecialEnemyIDs)
	return cloned
}

func normalizeBattleQueuedExtraTurns(rows []battleQueuedExtraTurn) []battleQueuedExtraTurn {
	if len(rows) == 0 {
		return []battleQueuedExtraTurn{}
	}
	out := make([]battleQueuedExtraTurn, 0, len(rows))
	for _, request := range rows {
		request.ActorID = strings.TrimSpace(request.ActorID)
		request.TurnMode = strings.ToLower(strings.TrimSpace(request.TurnMode))
		if request.ActorID == "" || request.TurnMode != "full" {
			continue
		}
		request.Trigger = strings.TrimSpace(request.Trigger)
		request.HookID = strings.TrimSpace(request.HookID)
		request.SourceKind = strings.TrimSpace(request.SourceKind)
		request.SourceID = strings.TrimSpace(request.SourceID)
		request.SourceNameKey = strings.TrimSpace(request.SourceNameKey)
		request.SourceItemID = strings.TrimSpace(request.SourceItemID)
		request.SourceAffixID = strings.TrimSpace(request.SourceAffixID)
		request.EventTargetID = strings.TrimSpace(request.EventTargetID)
		request.EventTargetType = strings.TrimSpace(request.EventTargetType)
		request.ChainDepth = maxInt(1, minInt(battleMaxQueuedExtraTurnsPerChain, request.ChainDepth))
		counts := map[string]int{}
		for key, count := range request.ChainHookCounts {
			key = strings.TrimSpace(key)
			if key != "" && count > 0 {
				counts[key] = count
			}
		}
		request.ChainHookCounts = counts
		out = append(out, request)
		if len(out) >= battleMaxQueuedExtraTurnsPerChain {
			break
		}
	}
	return out
}

func battleSettlementStatePayload(state battleRuntimeState, status string) map[string]any {
	payload := map[string]any{
		"seed":               state.Seed,
		"wave":               state.Wave,
		"completedBattles":   state.CompletedBattles,
		"seq":                state.Seq,
		"status":             status,
		"preparationOrder":   append([]string(nil), state.PreparationOrder...),
		"players":            battlePublicUnitSlice(state.Players),
		"battlefieldEffects": battlePublicBattlefieldEffectStates(state.BattlefieldEffects),
	}
	if strings.TrimSpace(state.BattleKind) != "" {
		payload["battleKind"] = strings.TrimSpace(state.BattleKind)
	}
	return payload
}

func encodeBattleCarryUnitState(unit battleUnitState) battleCarryUnitState {
	return battleCarryUnitState{
		UnitID:                          unit.UnitID,
		IsPlayerUnit:                    unit.IsPlayerUnit,
		Side:                            unit.Side,
		OriginSide:                      unit.OriginSide,
		BaseSide:                        unit.BaseSide,
		PerspectiveSide:                 unit.PerspectiveSide,
		UserID:                          unit.UserID,
		UnitType:                        unit.UnitType,
		KittyID:                         unit.KittyID,
		EnemyID:                         unit.EnemyID,
		EnemyType:                       unit.EnemyType,
		EnemyLevel:                      unit.EnemyLevel,
		Name:                            unit.Name,
		NameKey:                         unit.NameKey,
		Position:                        unit.Position,
		BasePosition:                    unit.BasePosition,
		Alive:                           unit.Alive,
		RewardEligible:                  unit.RewardEligible,
		HP:                              unit.HP,
		MP:                              unit.MP,
		MaxHP:                           unit.MaxHP,
		MaxMP:                           unit.MaxMP,
		BaseStats:                       cloneFloatMap(unit.BaseStats),
		Stats:                           cloneFloatMap(unit.Stats),
		StatModifiers:                   cloneFloatMap(unit.StatModifiers),
		StatSources:                     cloneNestedFloatMap(unit.StatSources),
		StatMode:                        unit.StatMode,
		SkillSlots:                      append([]string(nil), unit.SkillSlots...),
		DominationSkills:                append([]string(nil), unit.DominationSkills...),
		SkillIDs:                        append([]string(nil), unit.SkillIDs...),
		SkillRules:                      cloneBattleSkillRuleStateMap(unit.SkillRules),
		ItemRules:                       cloneBattleSkillRuleStateMap(unit.ItemRules),
		Skills:                          cloneBattleSkillStates(unit.Skills),
		BattleItems:                     cloneBattleItemStates(unit.BattleItems),
		EquipmentDigest:                 append([]string(nil), unit.EquipmentDigest...),
		EquipmentIDs:                    append([]int64(nil), unit.EquipmentIDs...),
		SpecializationTreeID:            unit.SpecializationTreeID,
		SpecializationRulesetVersion:    unit.SpecializationRulesetVersion,
		SpecializationBalanceRevision:   unit.SpecializationBalanceRevision,
		SpecializationRevision:          unit.SpecializationRevision,
		SpecializationDigest:            unit.SpecializationDigest,
		WeaponClasses:                   append([]string(nil), unit.WeaponClasses...),
		BasicAttack:                     cloneBattleBasicAttackProfile(unit.BasicAttack),
		EquipmentHooks:                  cloneBattleEquipmentHooks(unit.EquipmentHooks),
		SpecializationHooks:             cloneBattleEquipmentHooks(unit.SpecializationHooks),
		EffectMutations:                 cloneBattleAffixEffectMutations(unit.EffectMutations),
		SpecializationEffectMutations:   cloneBattleAffixEffectMutations(unit.SpecializationEffectMutations),
		KittyPersonalityHooks:           cloneBattleEquipmentHooks(unit.KittyPersonalityHooks),
		KittyPersonalityEffectMutations: cloneBattleAffixEffectMutations(unit.KittyPersonalityEffectMutations),
		Buffs:                           cloneBattleBuffStates(unit.Buffs),
		SummonRemainTurns:               unit.SummonRemainTurns,
		SummonTemplateUnitID:            unit.SummonTemplateUnitID,
		SummonSourceUnitID:              unit.SummonSourceUnitID,
		SummonMaxCountPerUnit:           unit.SummonMaxCountPerUnit,
		SummonPersistAcrossBattles:      unit.SummonPersistAcrossBattles,
		SummonOnCasterDeath:             unit.SummonOnCasterDeath,
		SummonOnCapOverflow:             unit.SummonOnCapOverflow,
		SummonOnDispel:                  unit.SummonOnDispel,
		SummonOnDurationEnd:             unit.SummonOnDurationEnd,
		SummonSourceEnhanceLevel:        unit.SummonSourceEnhanceLevel,
		SummonOrder:                     unit.SummonOrder,
		SummonNameSuffixKey:             unit.SummonNameSuffixKey,
	}
}

func battleCarryUnitSlice(rows []battleUnitState) []battleCarryUnitState {
	out := make([]battleCarryUnitState, 0, len(rows))
	for _, row := range rows {
		out = append(out, encodeBattleCarryUnitState(row))
	}
	return out
}

func cloneBattleSkillRuleStateMap(in map[string]battleSkillRuleState) map[string]battleSkillRuleState {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]battleSkillRuleState, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func cloneBattleSkillStates(in []battleSkillState) []battleSkillState {
	if len(in) == 0 {
		return nil
	}
	out := make([]battleSkillState, len(in))
	copy(out, in)
	return out
}

func cloneBattleItemStates(in []battleItemState) []battleItemState {
	if len(in) == 0 {
		return nil
	}
	out := make([]battleItemState, len(in))
	copy(out, in)
	return out
}

func cloneBattleBasicAttackProfile(in *battleBasicAttackProfile) *battleBasicAttackProfile {
	if in == nil {
		return nil
	}
	cloned := *in
	cloned.Formula = append([]battleAttackTerm(nil), in.Formula...)
	cloned.Effects = cloneBattleSkillEffects(in.Effects)
	return &cloned
}

func cloneBattleSkillEffects(in []battleSkillEffect) []battleSkillEffect {
	if len(in) == 0 {
		return nil
	}
	out := make([]battleSkillEffect, len(in))
	for idx, effect := range in {
		cloned := effect
		cloned.Targeting.TargetPositions = append([]string(nil), effect.Targeting.TargetPositions...)
		cloned.Targeting.TargetCount.Formula = append([]battleAttackTerm(nil), effect.Targeting.TargetCount.Formula...)
		cloned.Targeting.TargetPredicates = append([]battleSkillTargetPredicate(nil), effect.Targeting.TargetPredicates...)
		cloned.CorpseTargeting.TargetPositions = append([]string(nil), effect.CorpseTargeting.TargetPositions...)
		cloned.CorpseTargeting.TargetCount.Formula = append([]battleAttackTerm(nil), effect.CorpseTargeting.TargetCount.Formula...)
		cloned.CorpseTargeting.TargetPredicates = append([]battleSkillTargetPredicate(nil), effect.CorpseTargeting.TargetPredicates...)
		cloned.OnlyWhenNoTargetFromEffects = append([]int(nil), effect.OnlyWhenNoTargetFromEffects...)
		cloned.ExcludeTargetsFromEffects = append([]int(nil), effect.ExcludeTargetsFromEffects...)
		cloned.MutationSources = append([]battleEffectMutationSource(nil), effect.MutationSources...)
		cloned.AmountFormula.Terms = append([]battleAttackTerm(nil), effect.AmountFormula.Terms...)
		cloned.DamageSpec.AmountFormula.Terms = append([]battleAttackTerm(nil), effect.DamageSpec.AmountFormula.Terms...)
		cloned.DamageSpec.HitCount.Formula = append([]battleAttackTerm(nil), effect.DamageSpec.HitCount.Formula...)
		cloned.DamageSpec.FlatDefensePenetrationFormula.Terms = append([]battleAttackTerm(nil), effect.DamageSpec.FlatDefensePenetrationFormula.Terms...)
		cloned.DamageSpec.PercentDefensePenetrationFormula.Terms = append([]battleAttackTerm(nil), effect.DamageSpec.PercentDefensePenetrationFormula.Terms...)
		cloned.DamageSpec.PhysicalFlatDefensePenetrationFormula.Terms = append([]battleAttackTerm(nil), effect.DamageSpec.PhysicalFlatDefensePenetrationFormula.Terms...)
		cloned.DamageSpec.PhysicalPercentDefensePenetrationFormula.Terms = append([]battleAttackTerm(nil), effect.DamageSpec.PhysicalPercentDefensePenetrationFormula.Terms...)
		cloned.DamageSpec.MagicalFlatDefensePenetrationFormula.Terms = append([]battleAttackTerm(nil), effect.DamageSpec.MagicalFlatDefensePenetrationFormula.Terms...)
		cloned.DamageSpec.MagicalPercentDefensePenetrationFormula.Terms = append([]battleAttackTerm(nil), effect.DamageSpec.MagicalPercentDefensePenetrationFormula.Terms...)
		cloned.DamageSpec.ExtraDamageAmplificationFormula.Terms = append([]battleAttackTerm(nil), effect.DamageSpec.ExtraDamageAmplificationFormula.Terms...)
		cloned.DamageSpec.EnemyOnlyDamageReductionPierceTargetStatuses = append([]string(nil), effect.DamageSpec.EnemyOnlyDamageReductionPierceTargetStatuses...)
		if len(effect.DamageSpec.BonusAgainstTargetTypes) > 0 {
			cloned.DamageSpec.BonusAgainstTargetTypes = maps.Clone(effect.DamageSpec.BonusAgainstTargetTypes)
		}
		cloned.StatusPayload.StatusIDs = append([]string(nil), effect.StatusPayload.StatusIDs...)
		cloned.StatusPayload.DurationFormula.Terms = append([]battleAttackTerm(nil), effect.StatusPayload.DurationFormula.Terms...)
		cloned.StatusPayload.Payload = cloneAnyMap(effect.StatusPayload.Payload)
		cloned.SummonPayload.CountFormula.Terms = append([]battleAttackTerm(nil), effect.SummonPayload.CountFormula.Terms...)
		cloned.SummonPayload.DurationFormula.Terms = append([]battleAttackTerm(nil), effect.SummonPayload.DurationFormula.Terms...)
		cloned.SummonPayload.MaxCountFormula.Terms = append([]battleAttackTerm(nil), effect.SummonPayload.MaxCountFormula.Terms...)
		if len(effect.StatusPayload.PayloadFormula) > 0 {
			cloned.StatusPayload.PayloadFormula = make(map[string]battleSkillAmountFormula, len(effect.StatusPayload.PayloadFormula))
			for key, formula := range effect.StatusPayload.PayloadFormula {
				clonedFormula := formula
				clonedFormula.Terms = append([]battleAttackTerm(nil), formula.Terms...)
				cloned.StatusPayload.PayloadFormula[key] = clonedFormula
			}
		}
		if len(effect.SummonPayload.StatsFormula) > 0 {
			cloned.SummonPayload.StatsFormula = make(map[string]battleSkillAmountFormula, len(effect.SummonPayload.StatsFormula))
			for key, formula := range effect.SummonPayload.StatsFormula {
				clonedFormula := formula
				clonedFormula.Terms = append([]battleAttackTerm(nil), formula.Terms...)
				cloned.SummonPayload.StatsFormula[key] = clonedFormula
			}
		}
		cloned.ConsumeStatusesOnHit = append([]string(nil), effect.ConsumeStatusesOnHit...)
		cloned.ConsumeCasterStatusesOnHit = append([]string(nil), effect.ConsumeCasterStatusesOnHit...)
		cloned.RequiredCasterStatuses = append([]string(nil), effect.RequiredCasterStatuses...)
		cloned.RequiredTargetStatuses = append([]string(nil), effect.RequiredTargetStatuses...)
		cloned.ForbiddenCasterStatuses = append([]string(nil), effect.ForbiddenCasterStatuses...)
		cloned.ForbiddenTargetStatuses = append([]string(nil), effect.ForbiddenTargetStatuses...)
		out[idx] = cloned
	}
	return out
}

func normalizeBattleSkillTargetCount(count *battleSkillTargetCount) {
	if count == nil {
		return
	}
	if count.Base <= 0 {
		count.Base = 1
	}
	if count.MaxSet && count.Max <= 0 {
		count.MaxSet = false
		count.Max = 0
	}
	if !count.MaxSet && count.Max > 0 {
		count.MaxSet = true
	}
}

func normalizeBattleSkillTargeting(targeting *battleSkillTargeting) {
	if targeting == nil {
		return
	}
	if len(targeting.TargetPositions) == 0 {
		targeting.TargetPositions = []string{"vanguard"}
	}
	for idx, position := range targeting.TargetPositions {
		targeting.TargetPositions[idx] = normalizeBattleTargetPosition(position)
		if targeting.TargetPositions[idx] == "" {
			targeting.TargetPositions[idx] = "vanguard"
		}
	}
	targeting.SecondarySeek = normalizeBattleSecondarySeek(targeting.SecondarySeek, "front_to_rear")
	targeting.TargetScope = normalizeBattleTargetScope(targeting.TargetScope)
	targeting.SelectRule = normalizeBattleSelectRule(targeting.SelectRule)
	if targeting.SelectRule != "attr_max" && targeting.SelectRule != "attr_min" {
		targeting.SelectAttrKey = ""
	}
	normalizeBattleSkillTargetCount(&targeting.TargetCount)
}

func normalizeBattleSkillEffect(effect *battleSkillEffect) {
	if effect == nil {
		return
	}
	normalizeBattleSkillTargeting(&effect.Targeting)
	if effect.Kind == "corpse_explosion" {
		normalizeBattleSkillTargeting(&effect.CorpseTargeting)
	}
	effect.DamageType = normalizeBattleDamageType(effect.DamageType)
	effect.StatusPayload.StatusIDs = parseBattleStringSlice(effect.StatusPayload.StatusIDs)
	effect.ConsumeStatusesOnHit = parseBattleStringSlice(effect.ConsumeStatusesOnHit)
	effect.ConsumeCasterStatusesOnHit = parseBattleStringSlice(effect.ConsumeCasterStatusesOnHit)
	effect.RequiredCasterStatuses = parseBattleStringSlice(effect.RequiredCasterStatuses)
	effect.RequiredTargetStatuses = parseBattleStringSlice(effect.RequiredTargetStatuses)
}

func normalizeBattleBasicAttackProfile(profile *battleBasicAttackProfile) {
	if profile == nil {
		return
	}
	profile.TargetDirection = normalizeBattleTargetPosition(profile.TargetDirection)
	if profile.TargetDirection == "" {
		profile.TargetDirection = "vanguard"
	}
	profile.TargetSeekStrategy = normalizeBattleSecondarySeek(profile.TargetSeekStrategy, "front_to_rear")
	profile.DamageType = normalizeBattleDamageType(profile.DamageType)
	for idx := range profile.Effects {
		normalizeBattleSkillEffect(&profile.Effects[idx])
	}
}

func kittyPersonalityHooksOrEmpty(hooks []battleEquipmentHookRule) []battleEquipmentHookRule {
	if hooks == nil {
		return []battleEquipmentHookRule{}
	}
	return hooks
}

func kittyPersonalityMutationsOrEmpty(mutations []battleAffixEffectMutation) []battleAffixEffectMutation {
	if mutations == nil {
		return []battleAffixEffectMutation{}
	}
	return mutations
}

func (unit *battleUnitState) equipmentHookCount() int {
	if unit == nil {
		return 0
	}
	return len(unit.EquipmentHooks) + len(unit.KittyPersonalityHooks) + len(unit.SpecializationHooks)
}

// equipmentHookAt exposes the combined immutable runtime view without
// allocating a merged slice. Runtime source layers are fixed: equipment and
// affixes, kitty personality, then specialization.
func (unit *battleUnitState) equipmentHookAt(index int) (battleEquipmentHookRule, bool) {
	if unit == nil || index < 0 {
		return battleEquipmentHookRule{}, false
	}
	if index < len(unit.EquipmentHooks) {
		return unit.EquipmentHooks[index], true
	}
	index -= len(unit.EquipmentHooks)
	if index < len(unit.KittyPersonalityHooks) {
		return unit.KittyPersonalityHooks[index], true
	}
	index -= len(unit.KittyPersonalityHooks)
	if index < len(unit.SpecializationHooks) {
		return unit.SpecializationHooks[index], true
	}
	return battleEquipmentHookRule{}, false
}

func battleEquipmentHookActiveKey(unit *battleUnitState, trigger string, hook battleEquipmentHookRule) string {
	if unit == nil {
		return ""
	}
	unitID := strings.TrimSpace(unit.UnitID)
	if unitID == "" {
		return ""
	}
	hookID := strings.TrimSpace(hook.ID)
	if hookID == "" {
		hookID = strings.TrimSpace(hook.SourceItemID)
	}
	if hookID == "" {
		hookID = fmt.Sprintf("%p", &hook)
	}
	return unitID + ":" + battleNormalizeTrigger(trigger) + ":" + hookID
}

func battleEquipmentHookSubjectAllowed(hook battleEquipmentHookRule, subject *battleUnitState) bool {
	filter := hook.SubjectFilter
	if len(filter) == 0 {
		return true
	}
	if parseBooleanLoose(firstNonNil(filter["requireAlive"], filter["require_alive"]), false) {
		if subject == nil || !subject.Alive || subject.HP <= 0 {
			return false
		}
	}
	includeTypes := battleParseStringList(firstNonNil(filter["includeUnitTypes"], filter["include_unit_types"], filter["includeTypes"], filter["include_types"]))
	excludeTypes := battleParseStringList(firstNonNil(filter["excludeUnitTypes"], filter["exclude_unit_types"], filter["excludeTypes"], filter["exclude_types"]))
	subjectType := battleUnitFilterType(subject)
	if len(includeTypes) > 0 && !battleStringSliceContainsFold(includeTypes, subjectType) {
		return false
	}
	if len(excludeTypes) > 0 && battleStringSliceContainsFold(excludeTypes, subjectType) {
		return false
	}
	includeTemplates := battleParseStringList(firstNonNil(filter["includeSummonTemplateIds"], filter["include_summon_template_ids"], filter["includeTemplateIds"], filter["include_template_ids"]))
	excludeTemplates := battleParseStringList(firstNonNil(filter["excludeSummonTemplateIds"], filter["exclude_summon_template_ids"], filter["excludeTemplateIds"], filter["exclude_template_ids"]))
	templateID := ""
	if subject != nil {
		templateID = strings.TrimSpace(subject.SummonTemplateUnitID)
	}
	if len(includeTemplates) > 0 && !battleStringSliceContainsFold(includeTemplates, templateID) {
		return false
	}
	if len(excludeTemplates) > 0 && battleStringSliceContainsFold(excludeTemplates, templateID) {
		return false
	}
	return true
}

func battleEquipmentHookEventAllowed(hook battleEquipmentHookRule, self *battleUnitState, subject *battleUnitState, context *battleEquipmentHookContext) bool {
	filter := hook.EventFilter
	if len(filter) == 0 {
		return true
	}
	skillIDs := battleParseStringList(firstNonNil(filter["skillIds"], filter["skill_ids"]))
	if len(skillIDs) > 0 && (context == nil || !battleStringSliceContainsFold(skillIDs, context.SkillID)) {
		return false
	}
	excludedSkillIDs := battleParseStringList(firstNonNil(filter["excludeSkillIds"], filter["exclude_skill_ids"]))
	if len(excludedSkillIDs) > 0 && context != nil && battleStringSliceContainsFold(excludedSkillIDs, context.SkillID) {
		return false
	}
	skillCategories := battleParseStringList(firstNonNil(filter["skillCategories"], filter["skill_categories"]))
	if len(skillCategories) > 0 && (context == nil || !battleStringSliceContainsFold(skillCategories, context.SkillCategory)) {
		return false
	}
	damageTypes := battleParseStringList(firstNonNil(filter["damageTypes"], filter["damage_types"]))
	if len(damageTypes) > 0 && (context == nil || !battleStringSliceContainsFold(damageTypes, normalizeBattleDamageType(context.DamageType))) {
		return false
	}
	if raw, exists := filter["includeBasicAttack"]; exists && context != nil && context.IsBasicAttack && !parseBooleanLoose(raw, false) {
		return false
	}
	if raw, exists := filter["include_basic_attack"]; exists && context != nil && context.IsBasicAttack && !parseBooleanLoose(raw, false) {
		return false
	}
	unitTypes := battleParseStringList(firstNonNil(filter["unitTypes"], filter["unit_types"]))
	if len(unitTypes) > 0 && !battleStringSliceContainsFold(unitTypes, battleUnitFilterType(subject)) {
		return false
	}
	probabilityKinds := battleParseStringList(firstNonNil(filter["probabilityKinds"], filter["probability_kinds"], filter["probabilityKind"], filter["probability_kind"]))
	if len(probabilityKinds) > 0 {
		if context == nil || !battleProbabilityStringAllowed(probabilityKinds, context.ProbabilityKind) {
			return false
		}
	}
	probabilitySourceIDs := battleParseStringList(firstNonNil(filter["sourceIds"], filter["source_ids"], filter["probabilitySourceIds"], filter["probability_source_ids"]))
	if len(probabilitySourceIDs) > 0 {
		if context == nil || !battleProbabilitySourceAllowed(probabilitySourceIDs, context.ProbabilitySourceIDs) {
			return false
		}
	}
	excludedProbabilitySourceIDs := battleParseStringList(firstNonNil(filter["excludeSourceIds"], filter["exclude_source_ids"], filter["excludeProbabilitySourceIds"], filter["exclude_probability_source_ids"]))
	if len(excludedProbabilitySourceIDs) > 0 && context != nil && battleProbabilitySourceAllowed(excludedProbabilitySourceIDs, context.ProbabilitySourceIDs) {
		return false
	}
	if parseBooleanLoose(firstNonNil(filter["excludeSelf"], filter["exclude_self"]), false) {
		if self == nil || subject == nil || strings.TrimSpace(self.UnitID) == strings.TrimSpace(subject.UnitID) {
			return false
		}
	}
	buffIDs := battleParseStringList(firstNonNil(filter["buffIds"], filter["buff_ids"]))
	if len(buffIDs) > 0 {
		if context == nil || !battleStringSliceContainsFold(buffIDs, strings.TrimSpace(context.BuffID)) {
			return false
		}
	}
	excludedBuffIDs := battleParseStringList(firstNonNil(filter["excludeBuffIds"], filter["exclude_buff_ids"]))
	if len(excludedBuffIDs) > 0 && context != nil && battleStringSliceContainsFold(excludedBuffIDs, strings.TrimSpace(context.BuffID)) {
		return false
	}
	buffKinds := battleParseStringList(firstNonNil(filter["buffKinds"], filter["buff_kinds"]))
	if len(buffKinds) > 0 {
		if context == nil || !battleStringSliceContainsFold(buffKinds, strings.TrimSpace(context.BuffKind)) {
			return false
		}
	}
	if parseBooleanLoose(firstNonNil(filter["firstBuffInstanceOnly"], filter["first_buff_instance_only"]), false) {
		if context == nil || subject == nil || battleCountBuff(subject, context.BuffID) != 1 {
			return false
		}
	}
	applicationKinds := battleParseStringList(firstNonNil(filter["applicationKinds"], filter["application_kinds"]))
	if len(applicationKinds) > 0 {
		if context == nil || !battleStringSliceContainsFold(applicationKinds, strings.TrimSpace(context.BuffApplicationKind)) {
			return false
		}
	}
	turnKinds := battleParseStringList(firstNonNil(filter["turnKinds"], filter["turn_kinds"]))
	if len(turnKinds) > 0 {
		if context == nil || !battleStringSliceContainsFold(turnKinds, strings.TrimSpace(context.TurnKind)) {
			return false
		}
	}
	if context != nil && context.IsPreparation {
		if raw, ok := filter["includePreparation"]; ok && !parseBooleanLoose(raw, false) {
			return false
		}
		if raw, ok := filter["include_preparation"]; ok && !parseBooleanLoose(raw, false) {
			return false
		}
	}
	if context != nil && context.ActionSuppressed {
		if raw, ok := filter["includeSuppressedTurns"]; ok && !parseBooleanLoose(raw, false) {
			return false
		}
		if raw, ok := filter["include_suppressed_turns"]; ok && !parseBooleanLoose(raw, false) {
			return false
		}
	}
	return true
}

func (s *Server) battleTriggerAfterApplyBuffEquipmentHooks(
	state *battleRuntimeState,
	now time.Time,
	source *battleUnitState,
	target *battleUnitState,
	buffID string,
	isNewAttach bool,
) []map[string]any {
	if s == nil || state == nil || source == nil || target == nil || strings.TrimSpace(buffID) == "" {
		return nil
	}
	applicationKind := "refresh"
	if isNewAttach {
		applicationKind = "attach"
	}
	return s.battleTriggerUnitEquipmentHooksWithContext(
		state,
		now,
		source,
		target,
		target,
		battleEquipmentTriggerAfterApplyBuff,
		0,
		"",
		&battleEquipmentHookContext{
			BuffID:              strings.TrimSpace(buffID),
			BuffApplicationKind: applicationKind,
			BuffKind:            s.battleBuffKindByID(buffID),
		},
	)
}

func battleEquipmentHookNeedsReentryGuard(hook battleEquipmentHookRule) bool {
	for _, effect := range hook.Effects {
		kind := battleNormalizeTrigger(fmt.Sprintf("%v", effect["kind"]))
		if kind == "" {
			kind = battleNormalizeTrigger(fmt.Sprintf("%v", effect["type"]))
		}
		if kind == "extraturn" || kind == "immediateaction" || kind == "damage" || kind == "addbuff" {
			return true
		}
	}
	return false
}

const battleMaxQueuedExtraTurnsPerChain = 64

func battleEquipmentHookDeferredExtraTurnCount(hook battleEquipmentHookRule) int {
	count := 0
	for _, effect := range hook.Effects {
		kind := battleNormalizeTrigger(fmt.Sprintf("%v", firstNonNil(effect["kind"], effect["type"])))
		if kind != "extraturn" && kind != "immediateaction" {
			continue
		}
		targetMode := battleNormalizeTrigger(fmt.Sprintf("%v", firstNonNil(effect["targetMode"], effect["target_mode"])))
		if targetMode != "" && targetMode != "nil" && targetMode != "self" {
			continue
		}
		turnMode := strings.ToLower(strings.TrimSpace(fmt.Sprintf("%v", firstNonNil(effect["turnMode"], effect["turn_mode"]))))
		if turnMode != "" && turnMode != "<nil>" && turnMode != "full" {
			continue
		}
		count++
	}
	return count
}

func battleExtraTurnChainHookKey(unit *battleUnitState, hook battleEquipmentHookRule) string {
	unitID := "unknown"
	if unit != nil && strings.TrimSpace(unit.UnitID) != "" {
		unitID = strings.TrimSpace(unit.UnitID)
	}
	return battleRuntimeCounterKey(unitID, battleEquipmentHookCounterGroup(hook))
}

func battleCanQueueProbabilityExtraTurn(state *battleRuntimeState, unit *battleUnitState, hook battleEquipmentHookRule) bool {
	if state == nil || unit == nil || state.activeExtraTurnRequest == nil {
		return false
	}
	effectCount := battleEquipmentHookDeferredExtraTurnCount(hook)
	if effectCount == 0 {
		return true
	}
	if state.activeExtraTurnRequest.ChainDepth >= battleMaxQueuedExtraTurnsPerChain || len(state.ExtraTurnQueue)+effectCount > battleMaxQueuedExtraTurnsPerChain {
		return false
	}
	// An uncapped probability reward may extend an extra-turn chain once, but the
	// persisted ancestry prevents it from sustaining that chain forever.
	if hook.MaxTriggersPerBattle <= 0 && state.activeExtraTurnRequest.ChainHookCounts[battleExtraTurnChainHookKey(unit, hook)] > 0 {
		return false
	}
	return true
}

func battleUnitHasActiveExtraTurn(state *battleRuntimeState, unit *battleUnitState) bool {
	if state == nil || unit == nil || state.activeExtraTurnUnitIDs == nil {
		return false
	}
	unitID := strings.TrimSpace(unit.UnitID)
	if unitID == "" {
		return false
	}
	return state.activeExtraTurnUnitIDs[unitID] > 0
}

func battleUnitFilterType(unit *battleUnitState) string {
	if unit == nil {
		return ""
	}
	if battleUnitIsSummon(unit) {
		return "summon"
	}
	unitType := strings.ToLower(strings.TrimSpace(unit.UnitType))
	if unitType == "kitty" {
		return "kitty"
	}
	if battleUnitTeamSide(unit) == "enemy" {
		return "enemy"
	}
	return "player"
}

func (unit *battleUnitState) otherUnitTurnEndEquipmentHooks() []*battleEquipmentHookRule {
	if unit == nil {
		return nil
	}
	equipmentCount := len(unit.EquipmentHooks)
	kittyCount := len(unit.KittyPersonalityHooks)
	specializationCount := len(unit.SpecializationHooks)
	if unit.otherUnitTurnEndHooksIndexed &&
		unit.otherUnitTurnEndEquipmentCount == equipmentCount &&
		unit.otherUnitTurnEndKittyCount == kittyCount &&
		unit.otherUnitTurnEndSpecializationCount == specializationCount {
		return unit.otherUnitTurnEndHooks
	}

	unit.otherUnitTurnEndHooks = unit.otherUnitTurnEndHooks[:0]
	triggerKey := battleNormalizeTrigger(battleEquipmentTriggerOtherTurnEnd)
	for i := range unit.EquipmentHooks {
		if battleNormalizeTrigger(unit.EquipmentHooks[i].Trigger) == triggerKey {
			unit.otherUnitTurnEndHooks = append(unit.otherUnitTurnEndHooks, &unit.EquipmentHooks[i])
		}
	}
	for i := range unit.KittyPersonalityHooks {
		if battleNormalizeTrigger(unit.KittyPersonalityHooks[i].Trigger) == triggerKey {
			unit.otherUnitTurnEndHooks = append(unit.otherUnitTurnEndHooks, &unit.KittyPersonalityHooks[i])
		}
	}
	for i := range unit.SpecializationHooks {
		if battleNormalizeTrigger(unit.SpecializationHooks[i].Trigger) == triggerKey {
			unit.otherUnitTurnEndHooks = append(unit.otherUnitTurnEndHooks, &unit.SpecializationHooks[i])
		}
	}
	unit.otherUnitTurnEndHooksIndexed = true
	unit.otherUnitTurnEndEquipmentCount = equipmentCount
	unit.otherUnitTurnEndKittyCount = kittyCount
	unit.otherUnitTurnEndSpecializationCount = specializationCount
	return unit.otherUnitTurnEndHooks
}

func (state *battleRuntimeState) otherUnitTurnEndHookUnitIDsForRuntime() []string {
	if state == nil {
		return nil
	}
	unitCount := len(state.Players) + len(state.Enemies)
	if state.otherUnitTurnEndHookIndexBuilt && state.otherUnitTurnEndHookUnitCount == unitCount {
		return state.otherUnitTurnEndHookUnitIDs
	}

	state.otherUnitTurnEndHookUnitIDs = state.otherUnitTurnEndHookUnitIDs[:0]
	collect := func(units []battleUnitState) {
		for i := range units {
			unit := &units[i]
			unitID := strings.TrimSpace(unit.UnitID)
			if unitID == "" {
				continue
			}
			if len(unit.otherUnitTurnEndEquipmentHooks()) == 0 {
				continue
			}
			state.otherUnitTurnEndHookUnitIDs = append(state.otherUnitTurnEndHookUnitIDs, unitID)
		}
	}
	collect(state.Players)
	collect(state.Enemies)
	state.otherUnitTurnEndHookIndexBuilt = true
	state.otherUnitTurnEndHookUnitCount = unitCount
	return state.otherUnitTurnEndHookUnitIDs
}

func (unit *battleUnitState) effectMutationCount() int {
	if unit == nil {
		return 0
	}
	return len(unit.EffectMutations) + len(unit.KittyPersonalityEffectMutations) + len(unit.SpecializationEffectMutations)
}

func (unit *battleUnitState) effectMutationAt(index int) (battleAffixEffectMutation, bool) {
	if unit == nil || index < 0 {
		return battleAffixEffectMutation{}, false
	}
	if index < len(unit.EffectMutations) {
		return unit.EffectMutations[index], true
	}
	index -= len(unit.EffectMutations)
	if index < len(unit.KittyPersonalityEffectMutations) {
		return unit.KittyPersonalityEffectMutations[index], true
	}
	index -= len(unit.KittyPersonalityEffectMutations)
	if index < len(unit.SpecializationEffectMutations) {
		return unit.SpecializationEffectMutations[index], true
	}
	return battleAffixEffectMutation{}, false
}

func battleAttackStatPolicyMutations(in []battleAffixEffectMutation) []battleAffixEffectMutation {
	count := 0
	for _, mutation := range in {
		if strings.EqualFold(strings.TrimSpace(mutation.AttackStatPolicy), "higher") {
			count++
		}
	}
	if count == 0 {
		return nil
	}
	out := make([]battleAffixEffectMutation, 0, count)
	for _, mutation := range in {
		if strings.EqualFold(strings.TrimSpace(mutation.AttackStatPolicy), "higher") {
			out = append(out, mutation)
		}
	}
	return out
}

func battleAttackStatPolicyPreviewActor(unit battleUnitState) (battleUnitState, bool) {
	unit.EffectMutations = battleAttackStatPolicyMutations(unit.EffectMutations)
	unit.KittyPersonalityEffectMutations = battleAttackStatPolicyMutations(unit.KittyPersonalityEffectMutations)
	unit.SpecializationEffectMutations = battleAttackStatPolicyMutations(unit.SpecializationEffectMutations)
	return unit, unit.effectMutationCount() > 0
}

func cloneBattleEquipmentHooks(in []battleEquipmentHookRule) []battleEquipmentHookRule {
	if len(in) == 0 {
		return nil
	}
	out := make([]battleEquipmentHookRule, len(in))
	copy(out, in)
	return out
}

func cloneBattleAffixEffectMutations(in []battleAffixEffectMutation) []battleAffixEffectMutation {
	if len(in) == 0 {
		return nil
	}
	out := make([]battleAffixEffectMutation, len(in))
	for idx, mutation := range in {
		cloned := mutation
		cloned.SkillIDList = append([]string(nil), mutation.SkillIDList...)
		if len(mutation.SkillIDs) > 0 {
			cloned.SkillIDs = make(map[string]struct{}, len(mutation.SkillIDs))
			for key := range mutation.SkillIDs {
				cloned.SkillIDs[key] = struct{}{}
			}
		}
		cloned.SkillCategoryList = append([]string(nil), mutation.SkillCategoryList...)
		cloned.SkillCategories = cloneBattleStringSet(mutation.SkillCategories)
		cloned.WeaponClassList = append([]string(nil), mutation.WeaponClassList...)
		cloned.WeaponClasses = cloneBattleStringSet(mutation.WeaponClasses)
		cloned.DamageTypeList = append([]string(nil), mutation.DamageTypeList...)
		cloned.DamageTypes = cloneBattleStringSet(mutation.DamageTypes)
		cloned.EffectKindList = append([]string(nil), mutation.EffectKindList...)
		cloned.EffectKinds = cloneBattleStringSet(mutation.EffectKinds)
		cloned.EffectIDList = append([]string(nil), mutation.EffectIDList...)
		cloned.EffectIDs = cloneBattleStringSet(mutation.EffectIDs)
		cloned.Indexes = append([]int(nil), mutation.Indexes...)
		cloned.TargetCountTermsAppend = append([]battleAttackTerm(nil), mutation.TargetCountTermsAppend...)
		cloned.AmountTermsAppend = append([]battleAttackTerm(nil), mutation.AmountTermsAppend...)
		cloned.StatusAppend = append([]string(nil), mutation.StatusAppend...)
		cloned.StatusRemoveList = append([]string(nil), mutation.StatusRemoveList...)
		if len(mutation.StatusRemove) > 0 {
			cloned.StatusRemove = make(map[string]struct{}, len(mutation.StatusRemove))
			for key := range mutation.StatusRemove {
				cloned.StatusRemove[key] = struct{}{}
			}
		}
		cloned.AppendEffects = cloneBattleSkillEffects(mutation.AppendEffects)
		cloned.TargetingPatch = cloneAnyMap(mutation.TargetingPatch)
		out[idx] = cloned
	}
	return out
}

func cloneBattleStringSet(in map[string]struct{}) map[string]struct{} {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]struct{}, len(in))
	for key := range in {
		out[key] = struct{}{}
	}
	return out
}

func cloneBattleBuffStates(in []battleBuffState) []battleBuffState {
	if len(in) == 0 {
		return nil
	}
	out := make([]battleBuffState, 0, len(in))
	for _, buff := range in {
		cloned := buff
		cloned.Payload = cloneAnyMap(buff.Payload)
		out = append(out, cloned)
	}
	return out
}

func battleCarryStatePayload(state battleRuntimeState, status string) map[string]any {
	nextPlayers := make([]battleUnitState, 0, len(state.Players))
	for _, player := range state.Players {
		if !battleUnitIsSummon(&player) {
			nextPlayers = append(nextPlayers, player)
			continue
		}
		if player.SummonPersistAcrossBattles && player.Alive && player.HP > 0 {
			nextPlayers = append(nextPlayers, player)
		}
	}
	battleCounts, cooldownUntilTurns, completedTurnCounts := battleFilterCarryCounterMaps(
		nextPlayers,
		state.BattleHookBattleCounts,
		state.BattleHookCooldownUntilTurns,
		state.CompletedTurnCounts,
	)
	payload := map[string]any{
		"seed":                         state.Seed,
		"wave":                         state.Wave,
		"completedBattles":             state.CompletedBattles,
		"seq":                          state.Seq,
		"status":                       status,
		"order":                        append([]string(nil), state.Order...),
		"preparationOrder":             append([]string(nil), state.PreparationOrder...),
		"players":                      battleCarryUnitSlice(nextPlayers),
		"battleHookBattleCounts":       battleCounts,
		"battleHookCooldownUntilTurns": cooldownUntilTurns,
		"completedTurnCounts":          completedTurnCounts,
	}
	if status == store.BattleRunStatusEnded {
		payload["battlefieldEffects"] = battlePublicBattlefieldEffectStates(state.BattlefieldEffects)
	}
	return payload
}

// Drop a counter only when its owner is unambiguously wave-local and is not
// carried forward. Unknown formats and non-wave-local formal-unit counters
// remain untouched, while enemy and expired-summon growth stays bounded.
func battleFilterCarryCounterMaps(
	players []battleUnitState,
	battleCounts map[string]int,
	cooldownUntilTurns map[string]int,
	completedTurnCounts map[string]int,
) (map[string]int, map[string]int, map[string]int) {
	retainedUnitIDs := make(map[string]struct{}, len(players))
	retainedCounterUnitIDs := make(map[string]struct{}, len(players))
	for index := range players {
		unitID := players[index].UnitID
		if strings.TrimSpace(unitID) == "" {
			continue
		}
		retainedUnitIDs[unitID] = struct{}{}
		retainedCounterUnitIDs[strings.TrimSpace(unitID)] = struct{}{}
	}
	return filterBattleRuntimeCounterMap(battleCounts, retainedCounterUnitIDs),
		filterBattleRuntimeCounterMap(cooldownUntilTurns, retainedCounterUnitIDs),
		filterBattleCompletedTurnCountMap(completedTurnCounts, retainedUnitIDs)
}

func filterBattleRuntimeCounterMap(in map[string]int, retainedUnitIDs map[string]struct{}) map[string]int {
	out := make(map[string]int)
	for key, count := range in {
		unitID, ok := battleRuntimeCounterKeyUnitID(key)
		if !ok {
			// Unknown keys may have been written by an older runtime. Preserve
			// them rather than silently changing a legacy counter contract.
			out[key] = count
			continue
		}
		if _, ok := retainedUnitIDs[unitID]; ok || !battleCarryCounterOwnerIsWaveLocal(unitID) {
			out[key] = count
		}
	}
	return out
}

func filterBattleCompletedTurnCountMap(in map[string]int, retainedUnitIDs map[string]struct{}) map[string]int {
	out := make(map[string]int, len(retainedUnitIDs))
	for unitID, count := range in {
		if _, ok := retainedUnitIDs[unitID]; ok || !battleCarryCounterOwnerIsWaveLocal(unitID) {
			out[unitID] = count
		}
	}
	return out
}

func battleCarryCounterOwnerIsWaveLocal(unitID string) bool {
	unitID = strings.TrimSpace(unitID)
	return strings.HasPrefix(unitID, "enemy:") || strings.HasPrefix(unitID, "summon_")
}

type battleEventBatchWirePayload struct {
	K string                  `json:"k,omitempty"`
	M int64                   `json:"m"`
	R int64                   `json:"r"`
	F int64                   `json:"f"`
	T int64                   `json:"t"`
	S int64                   `json:"s"`
	E int64                   `json:"e"`
	V []battleEventBatchEvent `json:"v"`
	N map[string]any          `json:"n,omitempty"`
}

type battleEventBatchEvent struct {
	Q int64          `json:"q"`
	O int64          `json:"o"`
	E string         `json:"e,omitempty"`
	D map[string]any `json:"d,omitempty"`
}

func toBattleEventBatchWirePayload(roomID int64, payload wsbus.BattleEventBatchPayload) battleEventBatchWirePayload {
	out := battleEventBatchWirePayload{
		M: roomID,
		R: payload.RunID,
		F: payload.FromSeq,
		T: payload.ToSeq,
		S: payload.WindowStartMS,
		E: payload.WindowEndMS,
		V: make([]battleEventBatchEvent, 0, len(payload.Events)),
	}
	for _, event := range payload.Events {
		eventType, data := battleTimelineEventPayloadParts(event)
		wireEvent := battleEventBatchEvent{
			Q: event.Seq,
			O: event.OffsetMS,
			E: compactBattleEventType(eventType),
		}
		if event.ParentSeq != nil {
			data = cloneAnyMap(data)
			if data == nil {
				data = map[string]any{}
			}
			data["parentSeq"] = *event.ParentSeq
		}
		if strings.TrimSpace(event.GroupID) != "" {
			if event.ParentSeq == nil {
				data = cloneAnyMap(data)
			}
			if data == nil {
				data = map[string]any{}
			}
			data["batchGroup"] = strings.TrimSpace(event.GroupID)
		}
		if len(data) > 0 {
			wireEvent.D = data
		}
		out.V = append(out.V, wireEvent)
	}
	if len(payload.Snapshot) > 0 {
		out.N = compactBattleSnapshot(payload.Snapshot)
	}
	return out
}

func toArenaBattleEventBatchWirePayload(challengeID int64, payload wsbus.BattleEventBatchPayload) battleEventBatchWirePayload {
	out := toBattleEventBatchWirePayload(challengeID, payload)
	out.K = battleKindArena
	return out
}

func toCombatTrialBattleEventBatchWirePayload(runID int64, payload wsbus.BattleEventBatchPayload) battleEventBatchWirePayload {
	out := toBattleEventBatchWirePayload(runID, payload)
	out.K = battleKindCombatTrial
	return out
}

func unpackBattleTimelinePayload(raw json.RawMessage) (string, map[string]any) {
	if len(raw) == 0 {
		return "", nil
	}
	payload := map[string]any{}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return "", nil
	}
	eventType := strings.TrimSpace(fmt.Sprintf("%v", payload["eventType"]))
	data := normalizeAnyMap(payload["data"])
	return eventType, data
}

func battleTimelineEventPayloadParts(event wsbus.BattleTimelineEvent) (string, map[string]any) {
	eventType := strings.TrimSpace(event.EventType)
	if eventType != "" || event.Data != nil {
		return eventType, event.Data
	}
	return unpackBattleTimelinePayload(event.Payload)
}

func compactBattleSnapshot(snapshot map[string]any) map[string]any {
	if len(snapshot) == 0 {
		return nil
	}
	players := compactBattleSnapshotUnits(snapshot["players"])
	enemies := compactBattleSnapshotUnits(snapshot["enemies"])
	out := map[string]any{
		"w":  toInt64Loose(snapshot["wave"], 1),
		"c":  toInt64Loose(snapshot["completedBattles"], 0),
		"q":  toInt64Loose(snapshot["seq"], 0),
		"ro": compactStringSlice(firstNonNil(snapshot["order"], snapshot["ro"])),
		"bo": compactStringSlice(firstNonNil(snapshot["preparationOrder"], snapshot["bo"])),
		"p":  players,
		"e":  enemies,
	}
	if battlefieldEffects := compactBattlefieldEffectStates(snapshot["battlefieldEffects"]); len(battlefieldEffects) > 0 {
		out["bf"] = battlefieldEffects
	}
	if arenaFormat := normalizeAnyMap(firstNonNil(snapshot["arenaFormat"], snapshot["af"])); len(arenaFormat) > 0 {
		out["af"] = arenaFormat
	}
	if combatTrialFormat := normalizeAnyMap(firstNonNil(snapshot["combatTrialFormat"], snapshot["ctf"])); len(combatTrialFormat) > 0 {
		out["ctf"] = combatTrialFormat
	}
	if combatTrial := normalizeAnyMap(firstNonNil(snapshot["combatTrial"], snapshot["ct"])); len(combatTrial) > 0 {
		out["ct"] = combatTrial
	}
	return out
}

func compactBattlefieldEffectStates(raw any) []map[string]any {
	rows := battleAnyMapSlice(raw)
	if len(rows) == 0 {
		return nil
	}
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		id := strings.TrimSpace(fmt.Sprintf("%v", firstNonNil(row["id"], row["i"])))
		if id == "" {
			continue
		}
		item := map[string]any{
			"i": id,
			"r": toInt64Loose(firstNonNil(row["remaining"], row["r"]), 0),
			"p": toInt64Loose(firstNonNil(row["progress"], row["p"]), 0),
			"g": toInt64Loose(firstNonNil(row["goal"], row["g"]), 0),
		}
		if windows := normalizeAnyMap(firstNonNil(row["windows"], row["w"])); len(windows) > 0 {
			item["w"] = windows
		}
		out = append(out, item)
	}
	return out
}

func compactBattleSnapshotUnits(raw any) []map[string]any {
	rows := battleAnyMapSlice(raw)
	if len(rows) == 0 {
		return []map[string]any{}
	}
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		if row == nil {
			continue
		}
		unitID := strings.TrimSpace(fmt.Sprintf("%v", firstNonNil(row["unitId"], row["u"])))
		if unitID == "" {
			continue
		}
		item := map[string]any{
			"u": unitID,
			"h": roundTo4(toFloat64Loose(firstNonNil(row["hp"], row["h"]), 0)),
			"m": roundTo4(toFloat64Loose(firstNonNil(row["mp"], row["m"]), 0)),
			"x": roundTo4(toFloat64Loose(firstNonNil(row["maxHp"], row["x"]), 0)),
			"y": roundTo4(toFloat64Loose(firstNonNil(row["maxMp"], row["y"]), 0)),
		}
		if side := strings.TrimSpace(fmt.Sprintf("%v", firstNonNil(row["side"], row["s"]))); side != "" {
			item["s"] = compactBattleSideCode(side)
		}
		if name := strings.TrimSpace(fmt.Sprintf("%v", firstNonNil(row["name"], row["n"]))); name != "" {
			item["n"] = name
		}
		if unitTypeRaw := firstNonNil(row["unitType"], row["t"]); unitTypeRaw != nil {
			if unitType := strings.TrimSpace(fmt.Sprintf("%v", unitTypeRaw)); unitType != "" {
				item["t"] = unitType
			}
		}
		if position := strings.TrimSpace(fmt.Sprintf("%v", firstNonNil(row["position"], row["p"]))); position != "" {
			item["p"] = compactBattlePositionCode(position)
		}
		if alive, ok := firstNonNil(row["alive"], row["a"]).(bool); ok {
			item["a"] = alive
		}
		if skillIDs := compactStringSlice(firstNonNil(row["skillIds"], row["i"])); len(skillIDs) > 0 {
			item["i"] = skillIDs
		}
		if cooldowns := compactCooldownMap(firstNonNil(row["skillCooldowns"], row["c"])); len(cooldowns) > 0 {
			item["d"] = cooldowns
		}
		if cooldownTurns := compactAuthoritativeCooldownMap(firstNonNil(row["skillCooldownTurns"], row["st"])); len(cooldownTurns) > 0 {
			item["st"] = cooldownTurns
		}
		if levels := compactSkillLevelMap(firstNonNil(row["skillLevels"], row["g"])); len(levels) > 0 {
			item["g"] = levels
		}
		if itemCooldowns := compactCooldownMap(firstNonNil(row["itemCooldowns"], row["ic"])); len(itemCooldowns) > 0 {
			item["ic"] = itemCooldowns
		}
		if itemQuantities := compactQuantityMap(firstNonNil(row["itemQuantities"], row["iq"])); len(itemQuantities) > 0 {
			item["iq"] = itemQuantities
		}
		if battleItems := compactBattleSnapshotItems(firstNonNil(row["battleItems"], row["bi"])); len(battleItems) > 0 {
			item["bi"] = battleItems
		}
		if buffs := compactBattleSnapshotBuffs(firstNonNil(row["buffs"], row["f"])); len(buffs) > 0 {
			item["f"] = buffs
		}
		if stats := normalizeFloatMapAny(firstNonNil(row["stats"], row["attributes"], row["z"])); len(stats) > 0 {
			item["z"] = stats
		}
		out = append(out, item)
	}
	return out
}

func compactBattleSideCode(side string) int {
	if strings.EqualFold(strings.TrimSpace(side), "enemy") {
		return 1
	}
	return 0
}

func compactBattlePositionCode(position string) int {
	switch strings.ToLower(strings.TrimSpace(position)) {
	case "midguard":
		return 1
	case "rearguard":
		return 2
	default:
		return 0
	}
}

func compactBattleSnapshotBuffs(raw any) []map[string]any {
	rows := battleAnyMapSlice(raw)
	if len(rows) == 0 {
		return []map[string]any{}
	}
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		if row == nil {
			continue
		}
		buffID := strings.TrimSpace(fmt.Sprintf("%v", firstNonNil(row["buffId"], row["i"])))
		if buffID == "" {
			continue
		}
		item := map[string]any{"i": buffID}
		if remainRaw := firstNonNil(row["remainTurns"], row["t"]); remainRaw != nil {
			item["t"] = toInt64Loose(remainRaw, 0)
		}
		if triggered := toInt64Loose(firstNonNil(row["triggered"], row["g"]), 0); triggered > 0 {
			item["g"] = triggered
		}
		if payload := normalizeAnyMap(firstNonNil(row["payload"], row["p"])); len(payload) > 0 {
			item["p"] = payload
		}
		out = append(out, item)
	}
	return out
}

func compactStringSlice(raw any) []string {
	switch typed := raw.(type) {
	case []string:
		out := make([]string, 0, len(typed))
		for _, row := range typed {
			value := strings.TrimSpace(row)
			if value == "" {
				continue
			}
			out = append(out, value)
		}
		return out
	case []any:
		out := make([]string, 0, len(typed))
		for _, row := range typed {
			value := strings.TrimSpace(fmt.Sprintf("%v", row))
			if value == "" {
				continue
			}
			out = append(out, value)
		}
		return out
	default:
		return nil
	}
}

func compactCooldownMap(raw any) map[string]any {
	in := normalizeAnyMap(raw)
	if len(in) == 0 {
		return nil
	}
	out := map[string]any{}
	for key, value := range in {
		normalizedKey := strings.TrimSpace(key)
		if normalizedKey == "" {
			continue
		}
		cooldown := toInt64Loose(value, 0)
		if cooldown <= 0 {
			continue
		}
		out[normalizedKey] = cooldown
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func compactAuthoritativeCooldownMap(raw any) map[string]any {
	in := normalizeAnyMap(raw)
	out := map[string]any{}
	for key, value := range in {
		normalizedKey := strings.TrimSpace(key)
		if normalizedKey == "" {
			continue
		}
		out[normalizedKey] = maxInt64(0, toInt64Loose(value, 0))
	}
	return out
}

func compactSkillLevelMap(raw any) map[string]any {
	in := normalizeAnyMap(raw)
	if len(in) == 0 {
		return nil
	}
	out := map[string]any{}
	for key, value := range in {
		normalizedKey := strings.TrimSpace(key)
		if normalizedKey == "" {
			continue
		}
		level := toInt64Loose(value, 0)
		if level <= 0 {
			continue
		}
		out[normalizedKey] = level
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func compactQuantityMap(raw any) map[string]any {
	in := normalizeAnyMap(raw)
	if len(in) == 0 {
		return nil
	}
	out := map[string]any{}
	for key, value := range in {
		normalizedKey := strings.TrimSpace(key)
		if normalizedKey == "" {
			continue
		}
		quantity := toInt64Loose(value, 0)
		if quantity < 0 {
			quantity = 0
		}
		out[normalizedKey] = quantity
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func compactBattleSnapshotItems(raw any) []map[string]any {
	rows := battleAnyMapSlice(raw)
	if len(rows) == 0 {
		return nil
	}
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		if row == nil {
			continue
		}
		itemID := strings.TrimSpace(fmt.Sprintf("%v", firstNonNil(row["itemId"], row["i"])))
		if itemID == "" {
			continue
		}
		item := map[string]any{"i": itemID}
		if quantityRaw := firstNonNil(row["quantity"], row["q"]); quantityRaw != nil {
			quantity := toInt64Loose(quantityRaw, 0)
			if quantity < 0 {
				quantity = 0
			}
			item["q"] = quantity
		}
		if cooldownTurnsRaw := firstNonNil(row["cooldownTurns"], row["t"]); cooldownTurnsRaw != nil {
			cooldownTurns := toInt64Loose(cooldownTurnsRaw, 0)
			if cooldownTurns < 0 {
				cooldownTurns = 0
			}
			item["t"] = cooldownTurns
		}
		if cooldownRemainRaw := firstNonNil(row["cooldownRemain"], row["r"]); cooldownRemainRaw != nil {
			cooldownRemain := toInt64Loose(cooldownRemainRaw, 0)
			if cooldownRemain < 0 {
				cooldownRemain = 0
			}
			item["r"] = cooldownRemain
		}
		out = append(out, item)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func compactBattleEventType(eventType string) string {
	switch strings.TrimSpace(eventType) {
	case "battle.timeline.node":
		return "n"
	case "battle.wave.spawn":
		return "s"
	case "battle.wave.win":
		return "w"
	case "battle.turn.action":
		return "a"
	case "battle.turn.regen":
		return "r"
	case "battle.field.tick":
		return "ft"
	case "battle.field.trigger":
		return "fg"
	case "battle.field.window":
		return "fw"
	default:
		return eventType
	}
}

func (state battleRuntimeState) toMap() map[string]any {
	buf, err := json.Marshal(state)
	if err != nil {
		return map[string]any{}
	}
	out := map[string]any{}
	_ = json.Unmarshal(buf, &out)
	return out
}

func (state *battleRuntimeState) emit(eventType string, now time.Time, data map[string]any) {
	if strings.TrimSpace(state.BattleKind) != "" {
		if data == nil {
			data = map[string]any{}
		}
		data["battleKind"] = strings.TrimSpace(state.BattleKind)
	}
	if battleCombatTrialActive(state) {
		if data == nil {
			data = map[string]any{}
		}
		data["combatTrial"] = combatTrialProgressPayload(state)
	}
	state.Seq++
	event := battleEvent{
		Seq:  state.Seq,
		Type: eventType,
		TS:   now.UnixMilli(),
		Data: data,
	}
	state.Events = append(state.Events, event)
	if !state.RetainCompleteEventHistory && len(state.Events) > battleRecentEventLimit {
		overflow := len(state.Events) - battleRecentEventLimit
		copy(state.Events, state.Events[overflow:])
		clear(state.Events[len(state.Events)-overflow:])
		state.Events = state.Events[:battleRecentEventLimit]
	}
}

func (state *battleRuntimeState) nextEventBatch(runID int64, now time.Time, interval time.Duration) (wsbus.BattleEventBatchPayload, bool) {
	if runID <= 0 || len(state.Events) == 0 {
		return wsbus.BattleEventBatchPayload{}, false
	}
	start := 0
	for i := len(state.Events) - 1; i >= 0; i-- {
		if state.Events[i].Seq <= state.LastPushedSeq {
			start = i + 1
			break
		}
	}
	if start >= len(state.Events) {
		return wsbus.BattleEventBatchPayload{}, false
	}
	events := append([]battleEvent(nil), state.Events[start:]...)
	converted := make([]wsbus.BattleTimelineEvent, 0, len(events))
	for _, event := range events {
		rawPayload, _ := json.Marshal(map[string]any{
			"eventType": event.Type,
			"ts":        event.TS,
			"data":      event.Data,
		})
		offset := int64(0)
		if state.StartedAt > 0 && event.TS >= state.StartedAt {
			offset = event.TS - state.StartedAt
		}
		converted = append(converted, wsbus.BattleTimelineEvent{
			Seq:              event.Seq,
			OffsetMS:         offset,
			Lane:             "turn",
			Phase:            "main",
			OrderKey:         fmt.Sprintf("%020d:%020d", event.TS, event.Seq),
			ActionDurationMS: 0,
			Payload:          rawPayload,
		})
	}
	fromSeq := converted[0].Seq
	toSeq := converted[len(converted)-1].Seq
	state.LastPushedSeq = toSeq
	windowEnd := now.UnixMilli()
	windowStart := now.Add(-interval).UnixMilli()
	return wsbus.BattleEventBatchPayload{
		RunID:         runID,
		FromSeq:       fromSeq,
		ToSeq:         toSeq,
		WindowStartMS: windowStart,
		WindowEndMS:   windowEnd,
		Events:        converted,
		Snapshot: map[string]any{
			"seed":               state.Seed,
			"wave":               state.Wave,
			"completedBattles":   state.CompletedBattles,
			"seq":                state.Seq,
			"order":              append([]string(nil), state.Order...),
			"preparationOrder":   append([]string(nil), state.PreparationOrder...),
			"players":            battlePublicUnitSlice(state.Players),
			"enemies":            battlePublicUnitSlice(state.Enemies),
			"battlefieldEffects": battlePublicBattlefieldEffectStates(state.BattlefieldEffects),
		},
	}, true
}

type battlePlaybackCheckpoint struct {
	PlaybackStartedAtMs int64            `json:"playbackStartedAtMs"`
	CompileStartedAtMs  int64            `json:"compileStartedAtMs,omitempty"`
	CompileFinishedAtMs int64            `json:"compileFinishedAtMs,omitempty"`
	NextPushAtMs        int64            `json:"nextPushAtMs,omitempty"`
	LastDispatchAtMs    int64            `json:"lastDispatchAtMs,omitempty"`
	LastPushedSeq       int64            `json:"lastPushedSeq"`
	LastPushedOffsetMs  int64            `json:"lastPushedOffsetMs"`
	NextEventIdx        int              `json:"nextEventIdx,omitempty"`
	SnapshotPushed      bool             `json:"snapshotPushed"`
	Wave                int              `json:"wave"`
	CompletedBattles    int              `json:"completedBattles"`
	Seq                 int64            `json:"seq"`
	Order               []string         `json:"order,omitempty"`
	PreparationOrder    []string         `json:"preparationOrder,omitempty"`
	Players             []fiber.Map      `json:"players"`
	Enemies             []fiber.Map      `json:"enemies"`
	BattlefieldEffects  []map[string]any `json:"battlefieldEffects,omitempty"`
	Stage               string           `json:"stage,omitempty"`
	TotalEvents         int              `json:"totalEvents,omitempty"`
	FinalSeq            int64            `json:"finalSeq,omitempty"`
	RedisPersistedAtMs  int64            `json:"-"`
}

func (c battlePlaybackCheckpoint) toMap() map[string]any {
	return map[string]any{
		"playbackStartedAtMs": c.PlaybackStartedAtMs,
		"compileStartedAtMs":  c.CompileStartedAtMs,
		"compileFinishedAtMs": c.CompileFinishedAtMs,
		"nextPushAtMs":        c.NextPushAtMs,
		"lastDispatchAtMs":    c.LastDispatchAtMs,
		"lastPushedSeq":       c.LastPushedSeq,
		"lastPushedOffsetMs":  c.LastPushedOffsetMs,
		"nextEventIdx":        c.NextEventIdx,
		"snapshotPushed":      c.SnapshotPushed,
		"wave":                c.Wave,
		"completedBattles":    c.CompletedBattles,
		"seq":                 c.Seq,
		"order":               append([]string(nil), c.Order...),
		"preparationOrder":    append([]string(nil), c.PreparationOrder...),
		"players":             c.Players,
		"enemies":             c.Enemies,
		"battlefieldEffects":  c.BattlefieldEffects,
		"stage":               c.Stage,
		"totalEvents":         c.TotalEvents,
		"finalSeq":            c.FinalSeq,
		"events":              []battleEvent{},
	}
}

func (c battlePlaybackCheckpoint) cursorMap() map[string]any {
	return map[string]any{
		"playbackStartedAtMs": c.PlaybackStartedAtMs,
		"compileStartedAtMs":  c.CompileStartedAtMs,
		"compileFinishedAtMs": c.CompileFinishedAtMs,
		"nextPushAtMs":        c.NextPushAtMs,
		"lastDispatchAtMs":    c.LastDispatchAtMs,
		"lastPushedSeq":       c.LastPushedSeq,
		"lastPushedOffsetMs":  c.LastPushedOffsetMs,
		"nextEventIdx":        c.NextEventIdx,
		"snapshotPushed":      c.SnapshotPushed,
		"wave":                c.Wave,
		"completedBattles":    c.CompletedBattles,
		"seq":                 c.Seq,
		"order":               append([]string(nil), c.Order...),
		"preparationOrder":    append([]string(nil), c.PreparationOrder...),
		"battlefieldEffects":  c.BattlefieldEffects,
		"stage":               c.Stage,
		"totalEvents":         c.TotalEvents,
		"finalSeq":            c.FinalSeq,
	}
}

func (c battlePlaybackCheckpoint) snapshot(roomID int64) map[string]any {
	return map[string]any{
		"roomId":             roomID,
		"wave":               c.Wave,
		"completedBattles":   c.CompletedBattles,
		"seq":                c.Seq,
		"order":              append([]string(nil), c.Order...),
		"preparationOrder":   append([]string(nil), c.PreparationOrder...),
		"players":            normalizePlaybackSnapshotUnits(c.Players),
		"enemies":            normalizePlaybackSnapshotUnits(c.Enemies),
		"battlefieldEffects": c.BattlefieldEffects,
	}
}

func normalizePlaybackSnapshotUnits(rows []fiber.Map) []fiber.Map {
	if len(rows) == 0 {
		return []fiber.Map{}
	}
	out := make([]fiber.Map, 0, len(rows))
	for _, row := range rows {
		cloned := cloneAnyMap(row)
		hp, hpOK := playbackSnapshotNumber(cloned["hp"])
		maxHp, maxHpOK := playbackSnapshotNumber(cloned["maxHp"])
		mp, mpOK := playbackSnapshotNumber(cloned["mp"])
		maxMp, maxMpOK := playbackSnapshotNumber(cloned["maxMp"])
		if !hpOK && maxHpOK {
			hp = maxHp
			hpOK = true
		}
		if !mpOK {
			if maxMpOK {
				mp = maxMp
			} else {
				mp = 0
			}
			mpOK = true
		}
		if hpOK {
			cloned["hp"] = hp
		}
		if mpOK {
			cloned["mp"] = mp
		}
		if maxHpOK {
			cloned["maxHp"] = maxHp
		}
		if maxMpOK {
			cloned["maxMp"] = maxMp
		}
		if hpOK {
			cloned["alive"] = hp > 0
		} else if alive, ok := cloned["alive"].(bool); ok {
			cloned["alive"] = alive
		} else {
			cloned["alive"] = true
		}
		out = append(out, fiber.Map(cloned))
	}
	return out
}

func playbackSnapshotNumber(raw any) (float64, bool) {
	switch typed := raw.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int32:
		return float64(typed), true
	case int64:
		return float64(typed), true
	default:
		return 0, false
	}
}

func decodeBattlePlaybackCheckpoint(raw map[string]any, now time.Time) battlePlaybackCheckpoint {
	checkpoint := battlePlaybackCheckpoint{
		PlaybackStartedAtMs: now.UnixMilli(),
		CompileStartedAtMs:  0,
		CompileFinishedAtMs: 0,
		NextPushAtMs:        0,
		LastDispatchAtMs:    0,
		LastPushedSeq:       0,
		LastPushedOffsetMs:  -1,
		NextEventIdx:        0,
		SnapshotPushed:      false,
		Wave:                1,
		CompletedBattles:    0,
		Seq:                 0,
		Order:               []string{},
		PreparationOrder:    []string{},
		Players:             []fiber.Map{},
		Enemies:             []fiber.Map{},
		Stage:               "compiled",
		TotalEvents:         0,
		FinalSeq:            0,
	}
	if len(raw) == 0 {
		return checkpoint
	}
	if value := toInt64Loose(raw["playbackStartedAtMs"], 0); value > 0 {
		checkpoint.PlaybackStartedAtMs = value
	}
	checkpoint.CompileStartedAtMs = toInt64Loose(raw["compileStartedAtMs"], 0)
	checkpoint.CompileFinishedAtMs = toInt64Loose(raw["compileFinishedAtMs"], 0)
	checkpoint.NextPushAtMs = toInt64Loose(raw["nextPushAtMs"], 0)
	checkpoint.LastDispatchAtMs = toInt64Loose(raw["lastDispatchAtMs"], 0)
	checkpoint.LastPushedSeq = toInt64Loose(raw["lastPushedSeq"], 0)
	checkpoint.LastPushedOffsetMs = toInt64Loose(raw["lastPushedOffsetMs"], -1)
	checkpoint.NextEventIdx = int(toInt64Loose(raw["nextEventIdx"], 0))
	if checkpoint.NextEventIdx < 0 {
		checkpoint.NextEventIdx = 0
	}
	if value, ok := raw["snapshotPushed"]; ok {
		switch typed := value.(type) {
		case bool:
			checkpoint.SnapshotPushed = typed
		case string:
			parsed, err := strconv.ParseBool(strings.TrimSpace(typed))
			checkpoint.SnapshotPushed = err == nil && parsed
		default:
			checkpoint.SnapshotPushed = false
		}
	} else {
		checkpoint.SnapshotPushed = checkpoint.LastPushedSeq > 0
	}
	checkpoint.Wave = int(toInt64Loose(raw["wave"], 1))
	if checkpoint.Wave <= 0 {
		checkpoint.Wave = 1
	}
	checkpoint.CompletedBattles = int(toInt64Loose(raw["completedBattles"], 0))
	checkpoint.Seq = toInt64Loose(raw["seq"], 0)
	checkpoint.Order = decodeBattleStringSlice(firstNonNil(raw["order"], raw["ro"]))
	checkpoint.PreparationOrder = decodeBattleStringSlice(firstNonNil(raw["preparationOrder"], raw["bo"]))
	checkpoint.Players = decodeFiberMapSlice(raw["players"])
	checkpoint.Enemies = decodeFiberMapSlice(raw["enemies"])
	checkpoint.BattlefieldEffects = battleAnyMapSlice(firstNonNil(raw["battlefieldEffects"], raw["bf"]))
	checkpoint.Stage = strings.TrimSpace(strings.ToLower(fmt.Sprintf("%v", raw["stage"])))
	if checkpoint.Stage == "" {
		checkpoint.Stage = "compiled"
	}
	checkpoint.TotalEvents = int(toInt64Loose(raw["totalEvents"], 0))
	checkpoint.FinalSeq = toInt64Loose(raw["finalSeq"], 0)
	return checkpoint
}

func (c *battlePlaybackCheckpoint) ensureTimelineCursor(timeline []wsbus.BattleTimelineEvent) {
	if c == nil {
		return
	}
	if c.NextEventIdx < 0 {
		c.NextEventIdx = 0
	}
	if c.NextEventIdx > len(timeline) {
		c.NextEventIdx = len(timeline)
	}
	if c.LastPushedSeq <= 0 || c.NextEventIdx > 0 {
		return
	}
	c.NextEventIdx = locateNextTimelineIndexBySeq(timeline, c.LastPushedSeq)
}

func locateNextTimelineIndexBySeq(timeline []wsbus.BattleTimelineEvent, lastSeq int64) int {
	if len(timeline) == 0 {
		return 0
	}
	left := 0
	right := len(timeline)
	for left < right {
		mid := left + (right-left)/2
		if timeline[mid].Seq <= lastSeq {
			left = mid + 1
		} else {
			right = mid
		}
	}
	return left
}

func decodeFiberMapSlice(raw any) []fiber.Map {
	out := []fiber.Map{}
	appendRow := func(row any) bool {
		cloned, ok := cloneBattlePlaybackFiberMap(row, 0)
		if !ok {
			return false
		}
		out = append(out, cloned)
		return true
	}
	switch typed := raw.(type) {
	case []map[string]any:
		out = make([]fiber.Map, 0, len(typed))
		for _, row := range typed {
			if !appendRow(row) {
				return []fiber.Map{}
			}
		}
	case []fiber.Map:
		out = make([]fiber.Map, 0, len(typed))
		for _, row := range typed {
			if !appendRow(row) {
				return []fiber.Map{}
			}
		}
	case []any:
		out = make([]fiber.Map, 0, len(typed))
		for _, row := range typed {
			if !appendRow(row) {
				return []fiber.Map{}
			}
		}
	default:
		return []fiber.Map{}
	}
	return out
}

const battlePlaybackJSONCloneMaxDepth = 64

func cloneBattlePlaybackFiberMap(raw any, depth int) (fiber.Map, bool) {
	if raw == nil {
		return nil, true
	}
	switch typed := raw.(type) {
	case fiber.Map:
		return cloneBattlePlaybackAnyMapFast(typed, depth)
	case map[string]any:
		cloned, ok := cloneBattlePlaybackAnyMapFast(typed, depth)
		return fiber.Map(cloned), ok
	}
	return cloneBattlePlaybackFiberMapReflect(raw, depth)
}

func cloneBattlePlaybackFiberMapReflect(raw any, depth int) (fiber.Map, bool) {
	if raw == nil {
		return nil, true
	}
	value := reflect.ValueOf(raw)
	for value.Kind() == reflect.Interface {
		if value.IsNil() {
			return nil, true
		}
		value = value.Elem()
	}
	if depth > battlePlaybackJSONCloneMaxDepth || value.Kind() != reflect.Map || value.Type().Key().Kind() != reflect.String {
		return nil, false
	}
	if value.IsNil() {
		return nil, true
	}
	out := make(fiber.Map, value.Len())
	iter := value.MapRange()
	for iter.Next() {
		cloned, ok := cloneBattlePlaybackJSONValue(iter.Value(), depth+1)
		if !ok {
			return nil, false
		}
		out[iter.Key().String()] = cloned.Interface()
	}
	return out, true
}

func cloneBattlePlaybackAnyMapFast[M ~map[string]any](raw M, depth int) (M, bool) {
	if depth > battlePlaybackJSONCloneMaxDepth {
		return nil, false
	}
	if raw == nil {
		return nil, true
	}
	out := make(M, len(raw))
	for key, value := range raw {
		cloned, ok := cloneBattlePlaybackJSONInterfaceFast(value, depth+1)
		if !ok {
			return nil, false
		}
		out[key] = cloned
	}
	return out, true
}

// Values stored in map[string]any and []any consume an interface level in the
// legacy reflection walk. Keep that depth accounting so the safety limit has
// exactly the same boundary on the fast path.
func cloneBattlePlaybackJSONInterfaceFast(raw any, depth int) (any, bool) {
	if depth > battlePlaybackJSONCloneMaxDepth {
		return nil, false
	}
	if raw == nil {
		return nil, true
	}
	return cloneBattlePlaybackJSONConcreteFast(raw, depth+1)
}

func cloneBattlePlaybackJSONConcreteFast(raw any, depth int) (any, bool) {
	if depth > battlePlaybackJSONCloneMaxDepth {
		return nil, false
	}
	switch typed := raw.(type) {
	case fiber.Map:
		return cloneBattlePlaybackAnyMapFast(typed, depth)
	case map[string]any:
		return cloneBattlePlaybackAnyMapFast(typed, depth)
	case []fiber.Map:
		if typed == nil {
			return typed, true
		}
		out := make([]fiber.Map, len(typed))
		for idx := range typed {
			cloned, ok := cloneBattlePlaybackAnyMapFast(typed[idx], depth+1)
			if !ok {
				return nil, false
			}
			out[idx] = cloned
		}
		return out, true
	case []map[string]any:
		if typed == nil {
			return typed, true
		}
		out := make([]map[string]any, len(typed))
		for idx := range typed {
			cloned, ok := cloneBattlePlaybackAnyMapFast(typed[idx], depth+1)
			if !ok {
				return nil, false
			}
			out[idx] = cloned
		}
		return out, true
	case []any:
		if typed == nil {
			return typed, true
		}
		out := make([]any, len(typed))
		for idx, value := range typed {
			cloned, ok := cloneBattlePlaybackJSONInterfaceFast(value, depth+1)
			if !ok {
				return nil, false
			}
			out[idx] = cloned
		}
		return out, true
	case map[string]float64:
		if typed == nil {
			return typed, true
		}
		if len(typed) > 0 && depth+1 > battlePlaybackJSONCloneMaxDepth {
			return nil, false
		}
		out := make(map[string]float64, len(typed))
		for key, value := range typed {
			if math.IsNaN(value) || math.IsInf(value, 0) {
				return nil, false
			}
			out[key] = value
		}
		return out, true
	case map[string]string:
		return cloneBattlePlaybackScalarMap(typed, depth)
	case map[string]bool:
		return cloneBattlePlaybackScalarMap(typed, depth)
	case map[string]int:
		return cloneBattlePlaybackScalarMap(typed, depth)
	case map[string]int64:
		return cloneBattlePlaybackScalarMap(typed, depth)
	case []string:
		return cloneBattlePlaybackScalarSlice(typed, depth)
	case []int:
		return cloneBattlePlaybackScalarSlice(typed, depth)
	case []int64:
		return cloneBattlePlaybackScalarSlice(typed, depth)
	case []uint64:
		return cloneBattlePlaybackScalarSlice(typed, depth)
	case bool, string, json.Number,
		int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64:
		return typed, true
	case float32:
		if math.IsNaN(float64(typed)) || math.IsInf(float64(typed), 0) {
			return nil, false
		}
		return typed, true
	case float64:
		if math.IsNaN(typed) || math.IsInf(typed, 0) {
			return nil, false
		}
		return typed, true
	default:
		cloned, ok := cloneBattlePlaybackJSONValue(reflect.ValueOf(raw), depth)
		if !ok {
			return nil, false
		}
		return cloned.Interface(), true
	}
}

type battlePlaybackJSONScalar interface {
	~bool | ~string |
		~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64
}

func cloneBattlePlaybackScalarMap[T battlePlaybackJSONScalar](raw map[string]T, depth int) (map[string]T, bool) {
	if raw == nil {
		return nil, true
	}
	if len(raw) > 0 && depth+1 > battlePlaybackJSONCloneMaxDepth {
		return nil, false
	}
	out := make(map[string]T, len(raw))
	for key, value := range raw {
		out[key] = value
	}
	return out, true
}

func cloneBattlePlaybackScalarSlice[T battlePlaybackJSONScalar](raw []T, depth int) ([]T, bool) {
	if raw == nil {
		return nil, true
	}
	if len(raw) > 0 && depth+1 > battlePlaybackJSONCloneMaxDepth {
		return nil, false
	}
	out := make([]T, len(raw))
	copy(out, raw)
	return out, true
}

func cloneBattlePlaybackJSONValue(value reflect.Value, depth int) (reflect.Value, bool) {
	if depth > battlePlaybackJSONCloneMaxDepth || !value.IsValid() {
		return reflect.Value{}, false
	}
	switch value.Kind() {
	case reflect.Interface:
		out := reflect.New(value.Type()).Elem()
		if value.IsNil() {
			return out, true
		}
		cloned, ok := cloneBattlePlaybackJSONValue(value.Elem(), depth+1)
		if !ok || (!cloned.Type().AssignableTo(value.Type()) && !cloned.Type().Implements(value.Type())) {
			return reflect.Value{}, false
		}
		out.Set(cloned)
		return out, true
	case reflect.Pointer:
		if value.IsNil() {
			return reflect.Zero(value.Type()), true
		}
		cloned, ok := cloneBattlePlaybackJSONValue(value.Elem(), depth+1)
		if !ok {
			return reflect.Value{}, false
		}
		out := reflect.New(value.Type().Elem())
		out.Elem().Set(cloned)
		return out, true
	case reflect.Map:
		if value.Type().Key().Kind() != reflect.String {
			return reflect.Value{}, false
		}
		if value.IsNil() {
			return reflect.Zero(value.Type()), true
		}
		out := reflect.MakeMapWithSize(value.Type(), value.Len())
		iter := value.MapRange()
		for iter.Next() {
			cloned, ok := cloneBattlePlaybackJSONValue(iter.Value(), depth+1)
			if !ok {
				return reflect.Value{}, false
			}
			out.SetMapIndex(iter.Key(), cloned)
		}
		return out, true
	case reflect.Slice:
		if value.IsNil() {
			return reflect.Zero(value.Type()), true
		}
		out := reflect.MakeSlice(value.Type(), value.Len(), value.Len())
		for idx := 0; idx < value.Len(); idx++ {
			cloned, ok := cloneBattlePlaybackJSONValue(value.Index(idx), depth+1)
			if !ok {
				return reflect.Value{}, false
			}
			out.Index(idx).Set(cloned)
		}
		return out, true
	case reflect.Array:
		out := reflect.New(value.Type()).Elem()
		for idx := 0; idx < value.Len(); idx++ {
			cloned, ok := cloneBattlePlaybackJSONValue(value.Index(idx), depth+1)
			if !ok {
				return reflect.Value{}, false
			}
			out.Index(idx).Set(cloned)
		}
		return out, true
	case reflect.Struct:
		out := reflect.New(value.Type()).Elem()
		for idx := 0; idx < value.NumField(); idx++ {
			if !out.Field(idx).CanSet() {
				return reflect.Value{}, false
			}
			cloned, ok := cloneBattlePlaybackJSONValue(value.Field(idx), depth+1)
			if !ok {
				return reflect.Value{}, false
			}
			out.Field(idx).Set(cloned)
		}
		return out, true
	case reflect.Float32, reflect.Float64:
		if number := value.Float(); math.IsNaN(number) || math.IsInf(number, 0) {
			return reflect.Value{}, false
		}
		return value, true
	case reflect.Bool, reflect.String,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return value, true
	default:
		return reflect.Value{}, false
	}
}

func encodeBattleTimeline(events []battleEvent, startedAtMs int64, nodeIntervalMs int64) []map[string]any {
	timeline, _ := encodeBattleTimelineWithPlayback(events, startedAtMs, nodeIntervalMs)
	return timeline
}

func encodeBattleTimelineWithPlayback(events []battleEvent, startedAtMs int64, nodeIntervalMs int64) ([]map[string]any, []wsbus.BattleTimelineEvent) {
	if nodeIntervalMs <= 0 {
		nodeIntervalMs = 1000
	}
	out := make([]map[string]any, 0, len(events))
	playback := make([]wsbus.BattleTimelineEvent, 0, len(events))
	pendingRegenByActor := map[string]map[string]any{}
	pendingItemByActor := map[string]map[string]any{}
	timelineSeqByEventSeq := map[int64]int64{}
	timelineOffsetByEventSeq := map[int64]int64{}
	nextSeq := int64(0)
	nextNodeOffset := nodeIntervalMs
	appendTimelineEvent := func(event battleEvent, eventType string, ts int64, data map[string]any, lane string, phase string, offsetOverride int64) int {
		nextSeq++
		eventType = strings.TrimSpace(eventType)
		lane = strings.TrimSpace(lane)
		phase = strings.TrimSpace(phase)
		offset := int64(0)
		if offsetOverride >= 0 {
			offset = offsetOverride
		} else if startedAtMs > 0 && ts >= startedAtMs {
			offset = ts - startedAtMs
		}
		payload := map[string]any{
			"eventType": eventType,
			"ts":        ts,
			"data":      data,
		}
		payloadRaw, _ := json.Marshal(payload)
		item := map[string]any{
			"seq":              nextSeq,
			"offsetMs":         offset,
			"lane":             lane,
			"phase":            phase,
			"orderKey":         fmt.Sprintf("%020d:%020d", ts, nextSeq),
			"actionDurationMs": int64(0),
			"payload":          payload,
		}
		out = append(out, item)
		playback = append(playback, wsbus.BattleTimelineEvent{
			Seq:              nextSeq,
			OffsetMS:         offset,
			Lane:             lane,
			Phase:            phase,
			OrderKey:         fmt.Sprintf("%020d:%020d", ts, nextSeq),
			ActionDurationMS: int64(0),
			Payload:          payloadRaw,
			EventType:        eventType,
			Data:             data,
		})
		if event.Seq > 0 {
			timelineSeqByEventSeq[event.Seq] = nextSeq
			timelineOffsetByEventSeq[event.Seq] = offset
		}
		return len(out) - 1
	}
	for _, event := range events {
		eventType := strings.TrimSpace(event.Type)
		switch eventType {
		case "battle.wave.spawn":
			appendTimelineEvent(event, eventType, event.TS, event.Data, "turn", "main", -1)
			startActions := battleAnyMapSlice(event.Data["battleStartActions"])
			if len(startActions) == 0 {
				break
			}
			nodeData := buildBattleTimelineActionRowsNodeData(event.Data["wave"], startActions)
			if len(battleAnyMapSlice(nodeData["p"])) == 0 {
				break
			}
			synthetic := battleEvent{Type: "battle.timeline.node", TS: event.TS}
			appendTimelineEvent(synthetic, synthetic.Type, event.TS, nodeData, "system", "node", 0)
			nextNodeOffset = maxInt64(nextNodeOffset, battleTimelineNodeDurationMs(nodeData)+nodeIntervalMs)
		case "battle.turn.regen":
			data := cloneAnyMap(event.Data)
			actorID := strings.TrimSpace(fmt.Sprintf("%v", data["actorId"]))
			if actorID != "" {
				pendingRegenByActor[actorID] = data
			}
		case "battle.turn.item_use":
			data := cloneAnyMap(event.Data)
			actorID := strings.TrimSpace(fmt.Sprintf("%v", data["actorId"]))
			if actorID == "" {
				nodeData := buildBattleTimelineNodeData(event, map[string]any{
					"wave":       data["wave"],
					"actorId":    data["actorId"],
					"skillId":    data["itemId"],
					"actorHp":    data["actorHp"],
					"actorMp":    data["actorMp"],
					"actorAlive": data["actorAlive"],
					"actionLog":  data["actionLog"],
					"hooks":      []map[string]any{},
				}, nil, data)
				appendTimelineEvent(event, "battle.timeline.node", event.TS, nodeData, "turn", "node", nextNodeOffset)
				nextNodeOffset += battleTimelineNodeDurationMs(nodeData) + nodeIntervalMs
				break
			}
			pendingItemByActor[actorID] = data
		case "battle.turn.action":
			data := cloneAnyMap(event.Data)
			actorID := strings.TrimSpace(fmt.Sprintf("%v", data["actorId"]))
			regenData := pendingRegenByActor[actorID]
			itemData := pendingItemByActor[actorID]
			delete(pendingRegenByActor, actorID)
			delete(pendingItemByActor, actorID)
			nodeData := buildBattleTimelineNodeData(event, data, regenData, itemData)
			parentEventSeq := toInt64Loose(data["parentSeq"], 0)
			offset := nextNodeOffset
			if parentEventSeq > 0 {
				if parentOffset, ok := timelineOffsetByEventSeq[parentEventSeq]; ok {
					offset = parentOffset
				}
			}
			itemIndex := appendTimelineEvent(event, "battle.timeline.node", event.TS, nodeData, "turn", "node", offset)
			if parentEventSeq > 0 {
				parentTimelineSeq := parentEventSeq
				if mappedSeq, ok := timelineSeqByEventSeq[parentEventSeq]; ok && mappedSeq > 0 {
					parentTimelineSeq = mappedSeq
				}
				out[itemIndex]["parentSeq"] = parentTimelineSeq
				playback[itemIndex].ParentSeq = &parentTimelineSeq
				groupID := strings.TrimSpace(fmt.Sprintf("%v", data["batchGroup"]))
				if groupID == "" || groupID == "<nil>" {
					groupID = fmt.Sprintf("sync:%d", parentTimelineSeq)
				}
				out[itemIndex]["groupId"] = groupID
				playback[itemIndex].GroupID = groupID
			} else {
				nextNodeOffset += battleTimelineNodeDurationMs(nodeData) + nodeIntervalMs
			}
		default:
			appendTimelineEvent(event, eventType, event.TS, event.Data, "turn", "main", -1)
		}
	}
	return out, playback
}

func battleTimelineNodeDurationMs(nodeData map[string]any) int64 {
	if len(nodeData) == 0 {
		return 0
	}
	steps := battleAnyMapSlice(nodeData["p"])
	if len(steps) == 0 {
		steps = battleAnyMapSlice(nodeData["steps"])
	}
	if len(steps) == 0 {
		return 0
	}
	total := int64(0)
	for _, step := range steps {
		delay := toInt64Loose(step["d"], toInt64Loose(step["delayMs"], 0))
		total += maxInt64(0, delay)
	}
	return total
}

func decodeBattleTimeline(raw []map[string]any) []wsbus.BattleTimelineEvent {
	if len(raw) == 0 {
		return []wsbus.BattleTimelineEvent{}
	}
	out := make([]wsbus.BattleTimelineEvent, 0, len(raw))
	for _, item := range raw {
		ev := wsbus.BattleTimelineEvent{
			Seq:              toInt64Loose(item["seq"], 0),
			OffsetMS:         toInt64Loose(item["offsetMs"], 0),
			Lane:             anyToString(item["lane"]),
			Phase:            anyToString(item["phase"]),
			OrderKey:         anyToString(item["orderKey"]),
			ActionDurationMS: toInt64Loose(item["actionDurationMs"], 0),
			GroupID:          anyToString(item["groupId"]),
			ActorID:          anyToString(item["actorId"]),
			ActorUserID:      toInt64Loose(item["actorUserId"], 0),
			HookID:           anyToString(item["hookId"]),
		}
		if v := item["parentSeq"]; v != nil {
			ps := toInt64Loose(v, -1)
			if ps >= 0 {
				ev.ParentSeq = &ps
			}
		}
		if payload := item["payload"]; payload != nil {
			payloadMap := normalizeAnyMap(payload)
			if len(payloadMap) > 0 {
				ev.EventType = strings.TrimSpace(fmt.Sprintf("%v", payloadMap["eventType"]))
				ev.Data = normalizeAnyMap(payloadMap["data"])
			}
			if pb, err := json.Marshal(payload); err == nil {
				ev.Payload = json.RawMessage(pb)
			}
		}
		out = append(out, ev)
	}
	return out
}

func anyToString(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}

func buildBattleTimelineNodeData(event battleEvent, actionData map[string]any, regenData map[string]any, itemData map[string]any) map[string]any {
	actorID := strings.TrimSpace(fmt.Sprintf("%v", actionData["actorId"]))
	steps := make([]map[string]any, 0, 16)
	unitUpdates := map[string]map[string]any{}
	appendStep := func(step map[string]any) {
		if step == nil {
			return
		}
		steps = append(steps, step)
	}
	if regenData != nil {
		for _, row := range battleAnyMapSlice(regenData["actions"]) {
			collectBattleUnitUpdateFromActionRow(row, unitUpdates)
			appendStep(battleTimelineStepWithPhase(battleTimelineStepFromActionRow(row), "regen"))
			appendStep(battleTimelineStepWithPhase(battleTimelinePositionMoveStepFromActionRow(row), "regen"))
		}
	}
	if itemData != nil {
		appendStep(withBattleTimelineStepPhase(map[string]any{
			"k":  "c",
			"s":  actorID,
			"i":  strings.TrimSpace(fmt.Sprintf("%v", itemData["itemId"])),
			"ph": "i",
		}, "item"))
		for _, row := range battleAnyMapSlice(itemData["actionLog"]) {
			collectBattleUnitUpdateFromActionRow(row, unitUpdates)
			appendStep(battleTimelineStepWithPhase(battleTimelineStepFromActionRow(row), "item"))
			appendStep(battleTimelineStepWithPhase(battleTimelinePositionMoveStepFromActionRow(row), "item"))
		}
	}
	appendStep(map[string]any{
		"k":  "c",
		"s":  actorID,
		"i":  strings.TrimSpace(fmt.Sprintf("%v", actionData["skillId"])),
		"ph": "a",
	})
	for _, row := range battleAnyMapSlice(actionData["actionLog"]) {
		collectBattleUnitUpdateFromActionRow(row, unitUpdates)
		appendStep(battleTimelineStepWithPhase(battleTimelineStepFromActionRow(row), "action"))
		appendStep(battleTimelineStepWithPhase(battleTimelinePositionMoveStepFromActionRow(row), "action"))
	}
	for _, hook := range battleAnyMapSlice(actionData["hooks"]) {
		sourceID := strings.TrimSpace(fmt.Sprintf("%v", hook["sourceId"]))
		targetID := strings.TrimSpace(fmt.Sprintf("%v", hook["targetId"]))
		appendStep(map[string]any{
			"k": "t",
			"s": sourceID,
			"t": []map[string]any{
				{"u": targetID},
			},
			"i":  strings.TrimSpace(fmt.Sprintf("%v", hook["skillId"])),
			"d":  toInt64Loose(hook["delayMs"], 0),
			"ph": "a",
		})
		appendStep(map[string]any{
			"k":  "c",
			"s":  sourceID,
			"i":  strings.TrimSpace(fmt.Sprintf("%v", hook["skillId"])),
			"ph": "a",
		})
		for _, effect := range battleAnyMapSlice(hook["effects"]) {
			collectBattleUnitUpdateFromActionRow(effect, unitUpdates)
			appendStep(battleTimelineStepWithPhase(battleTimelineStepFromActionRow(effect), "action"))
			appendStep(battleTimelineStepWithPhase(battleTimelinePositionMoveStepFromActionRow(effect), "action"))
		}
	}
	nodeData := map[string]any{
		"w": int(toInt64Loose(actionData["wave"], int64(toInt64Loose(regenData["wave"], 1)))),
		"o": actorID,
		"p": steps,
		"a": map[string]any{
			"h": toFloat64(actionData["actorHp"], 0),
			"m": toFloat64(actionData["actorMp"], 0),
			"l": actionData["actorAlive"],
		},
	}
	if len(unitUpdates) > 0 {
		nodeData["u"] = compactBattleUnitUpdates(unitUpdates)
	}
	if value, ok := actionData["preparationTurn"]; ok {
		nodeData["pr"] = parseBooleanLoose(value, false)
	}
	if value, ok := actionData["preparationOrder"]; ok {
		nodeData["po"] = decodeBattleStringSlice(value)
	}
	if value, ok := actionData["order"]; ok {
		nodeData["ro"] = decodeBattleStringSlice(value)
	}
	if value, ok := actionData["skillCooldowns"]; ok {
		nodeData["sc"] = compactAuthoritativeCooldownMap(value)
	}
	if value, ok := actionData["skillCooldownTurns"]; ok {
		nodeData["st"] = compactAuthoritativeCooldownMap(value)
	}
	if value, ok := actionData["itemCooldowns"]; ok {
		nodeData["ic"] = compactAuthoritativeCooldownMap(value)
	}
	if parseBooleanLoose(actionData["extraTurn"], false) {
		nodeData["et"] = true
	}
	if turnKind := strings.TrimSpace(fmt.Sprintf("%v", actionData["turnKind"])); turnKind != "" && turnKind != "<nil>" {
		nodeData["tk"] = turnKind
	}
	if value, ok := actionData["combatTrial"]; ok {
		nodeData["combatTrial"] = normalizeAnyMap(value)
	}
	return nodeData
}

func buildBattleTimelineActionRowsNodeData(wave any, actions []map[string]any) map[string]any {
	steps := make([]map[string]any, 0, len(actions)*2)
	unitUpdates := map[string]map[string]any{}
	originID := ""
	for _, row := range actions {
		if originID == "" {
			originID = strings.TrimSpace(fmt.Sprintf("%v", row["sourceId"]))
			if originID == "<nil>" {
				originID = ""
			}
		}
		collectBattleUnitUpdateFromActionRow(row, unitUpdates)
		if step := battleTimelineStepWithPhase(battleTimelineStepFromActionRow(row), "action"); step != nil {
			steps = append(steps, step)
		}
		if step := battleTimelineStepWithPhase(battleTimelinePositionMoveStepFromActionRow(row), "action"); step != nil {
			steps = append(steps, step)
		}
	}
	nodeData := map[string]any{
		"w": int(toInt64Loose(wave, 1)),
		"o": originID,
		"p": steps,
	}
	if len(unitUpdates) > 0 {
		nodeData["u"] = compactBattleUnitUpdates(unitUpdates)
	}
	return nodeData
}

func battleTimelineStepWithPhase(step map[string]any, phase string) map[string]any {
	return withBattleTimelineStepPhase(step, phase)
}

func withBattleTimelineStepPhase(step map[string]any, phase string) map[string]any {
	if step == nil {
		return nil
	}
	switch strings.TrimSpace(strings.ToLower(phase)) {
	case "regen":
		step["ph"] = "r"
	case "item":
		step["ph"] = "i"
	case "action":
		step["ph"] = "a"
	}
	return step
}

func battleTimelinePositionMoveStepFromActionRow(row map[string]any) map[string]any {
	if len(row) == 0 {
		return nil
	}
	kind := strings.ToLower(strings.TrimSpace(fmt.Sprintf("%v", row["kind"])))
	if kind != "stat_recalc" && kind != "position_move" {
		return nil
	}
	targetID := strings.TrimSpace(fmt.Sprintf("%v", firstNonNil(row["targetId"], row["u"])))
	if targetID == "" {
		return nil
	}
	fromPosition := normalizeBattleTargetPosition(strings.TrimSpace(fmt.Sprintf("%v", firstNonNil(row["fromPosition"], row["beforePosition"], row["f"]))))
	toPosition := normalizeBattleTargetPosition(strings.TrimSpace(fmt.Sprintf("%v", firstNonNil(row["toPosition"], row["position"], row["t"], row["p"]))))
	if fromPosition == "" || toPosition == "" || fromPosition == toPosition {
		return nil
	}
	target := map[string]any{
		"u": targetID,
		"f": compactBattlePositionCode(fromPosition),
		"t": compactBattlePositionCode(toPosition),
	}
	if basePosition := normalizeBattleTargetPosition(strings.TrimSpace(fmt.Sprintf("%v", firstNonNil(row["basePosition"], row["bp"])))); basePosition != "" {
		target["bp"] = compactBattlePositionCode(basePosition)
	}
	step := map[string]any{
		"k": "pm",
		"t": []map[string]any{target},
	}
	if sourceID := strings.TrimSpace(fmt.Sprintf("%v", row["sourceId"])); sourceID != "" {
		step["s"] = sourceID
	}
	if delay := toInt64Loose(row["delayMs"], 0); delay > 0 {
		step["d"] = delay
	}
	return step
}

func battleTimelineStepFromActionRow(row map[string]any) map[string]any {
	kind := strings.ToLower(strings.TrimSpace(fmt.Sprintf("%v", row["kind"])))
	sourceID := strings.TrimSpace(fmt.Sprintf("%v", firstNonNil(row["visualSourceId"], row["lineSourceId"], row["sourceId"])))
	targetID := strings.TrimSpace(fmt.Sprintf("%v", row["targetId"]))
	buffID := strings.TrimSpace(fmt.Sprintf("%v", firstNonNil(row["buffId"], row["stateId"])))
	step := map[string]any{
		"k": battleTimelineKindCode(kind),
	}
	if id, ok := row["skillId"].(string); ok && id != "" {
		step["i"] = id
	}
	if resource, ok := row["resource"].(string); ok && (resource == "hp" || resource == "mp") {
		step["resource"] = resource
	}
	if sourceID != "" {
		step["s"] = sourceID
	}
	if buffID != "" {
		step["bi"] = buffID
	}
	if value, ok := row["buffDurationTurns"]; ok {
		turns := toInt64Loose(value, 0)
		if turns != 0 {
			step["bt"] = turns
		}
	}
	if value, ok := row["buffTriggered"]; ok {
		triggered := toInt64Loose(value, 0)
		if triggered >= 0 {
			step["bg"] = triggered
		}
	}
	if value := normalizeAnyMap(row["buffPayload"]); len(value) > 0 {
		step["bp"] = cloneAnyMap(value)
	}
	delay := toInt64Loose(row["delayMs"], 0)
	if delay > 0 {
		step["d"] = delay
	}
	if amount, ok := row["amount"]; ok {
		parsed := roundTo4(toFloat64(amount, 0))
		if parsed != 0 {
			step["v"] = parsed
		}
	}
	if rawCrit, ok := row["crit"]; ok {
		step["c"] = parseBooleanLoose(rawCrit, false)
	}
	if damageType := strings.TrimSpace(strings.ToLower(fmt.Sprintf("%v", row["damageType"]))); damageType != "" {
		step["dt"] = damageType
	}
	if meta := compactBattleTimelineStepMeta(row["meta"]); len(meta) > 0 {
		step["me"] = meta
	}
	if summon := normalizeAnyMap(row["summon"]); len(summon) > 0 {
		step["q"] = compactBattleTimelineSummon(summon)
		targetID = strings.TrimSpace(fmt.Sprintf("%v", firstNonNil(summon["unitId"], summon["u"])))
	}
	if targetID != "" {
		target := map[string]any{
			"u": targetID,
		}
		if position := normalizeBattleTargetPosition(strings.TrimSpace(fmt.Sprintf("%v", firstNonNil(row["position"], row["toPosition"], row["p"])))); position != "" {
			target["p"] = compactBattlePositionCode(position)
		}
		if basePosition := normalizeBattleTargetPosition(strings.TrimSpace(fmt.Sprintf("%v", firstNonNil(row["basePosition"], row["bp"])))); basePosition != "" {
			target["bp"] = compactBattlePositionCode(basePosition)
		}
		if fromPosition := normalizeBattleTargetPosition(strings.TrimSpace(fmt.Sprintf("%v", firstNonNil(row["fromPosition"], row["beforePosition"], row["f"])))); fromPosition != "" {
			target["f"] = compactBattlePositionCode(fromPosition)
		}
		if toPosition := normalizeBattleTargetPosition(strings.TrimSpace(fmt.Sprintf("%v", firstNonNil(row["toPosition"], row["position"], row["t"], row["p"])))); toPosition != "" {
			target["t"] = compactBattlePositionCode(toPosition)
		}
		if side := normalizeBattleSide(strings.TrimSpace(fmt.Sprintf("%v", firstNonNil(row["side"], row["s"])))); side != "" {
			target["s"] = side
		}
		if baseSide := normalizeBattleSide(strings.TrimSpace(fmt.Sprintf("%v", firstNonNil(row["baseSide"], row["bs"])))); baseSide != "" {
			target["bs"] = baseSide
		}
		if perspectiveSide := normalizeBattleSide(strings.TrimSpace(fmt.Sprintf("%v", firstNonNil(row["perspectiveSide"], row["ps"])))); perspectiveSide != "" {
			target["ps"] = perspectiveSide
		}
		if value, ok := row["beforeHp"]; ok {
			target["b"] = roundTo4(toFloat64(value, 0))
		}
		if value, ok := row["afterHp"]; ok {
			target["h"] = roundTo4(toFloat64(value, 0))
		}
		if value, ok := row["afterMp"]; ok {
			target["m"] = roundTo4(toFloat64(value, 0))
		}
		if value, ok := row["beforeMp"]; ok {
			target["beforeMp"] = roundTo4(toFloat64(value, 0))
		}
		if value, ok := row["maxHpAfter"]; ok {
			target["x"] = roundTo4(toFloat64(value, 0))
		}
		if value, ok := row["maxMpAfter"]; ok {
			target["y"] = roundTo4(toFloat64(value, 0))
		}
		if alive, ok := row["afterAlive"].(bool); ok {
			target["l"] = alive
		}
		if consumed, ok := row["corpseConsumed"].(bool); ok {
			target["cc"] = consumed
		}
		if rawCrit, ok := row["crit"]; ok {
			target["c"] = parseBooleanLoose(rawCrit, false)
		}
		if value := normalizeFloatMapAny(row["stats"]); len(value) > 0 {
			target["stats"] = value
		}
		if value := normalizeFloatMapAny(row["baseStats"]); len(value) > 0 {
			target["baseStats"] = value
		}
		if value := normalizeFloatMapAny(firstNonNil(row["deltaStats"], row["statModifiers"])); len(value) > 0 {
			target["statModifiers"] = value
		}
		if value := normalizeAnyMap(row["statSources"]); len(value) > 0 {
			target["statSources"] = cloneAnyMap(value)
		}
		if value := battleAnyMapSlice(row["buffs"]); len(value) > 0 {
			target["buffs"] = value
		}
		step["t"] = []map[string]any{target}
	}
	return step
}

func collectBattleUnitUpdateFromActionRow(row map[string]any, updates map[string]map[string]any) {
	if len(row) == 0 || updates == nil {
		return
	}
	targetID := strings.TrimSpace(fmt.Sprintf("%v", firstNonNil(row["targetId"], row["u"])))
	if summon := normalizeAnyMap(row["summon"]); len(summon) > 0 {
		targetID = strings.TrimSpace(fmt.Sprintf("%v", firstNonNil(summon["unitId"], summon["u"], targetID)))
	}
	if targetID == "" {
		return
	}
	update, ok := updates[targetID]
	if !ok {
		update = map[string]any{"u": targetID}
		updates[targetID] = update
	}
	if value, ok := row["afterHp"]; ok {
		update["h"] = roundTo4(toFloat64(value, 0))
	}
	if value, ok := row["afterMp"]; ok {
		update["m"] = roundTo4(toFloat64(value, 0))
	}
	if alive, ok := row["afterAlive"].(bool); ok {
		update["l"] = alive
	}
	if consumed, ok := row["corpseConsumed"].(bool); ok {
		update["cc"] = consumed
	}
	if value, ok := row["maxHpAfter"]; ok {
		update["x"] = roundTo4(toFloat64(value, 0))
	}
	if value, ok := row["maxMpAfter"]; ok {
		update["y"] = roundTo4(toFloat64(value, 0))
	}
	if value, ok := row["maxHp"]; ok {
		update["x"] = roundTo4(toFloat64(value, 0))
	}
	if value, ok := row["maxMp"]; ok {
		update["y"] = roundTo4(toFloat64(value, 0))
	}
	if position := normalizeBattleTargetPosition(strings.TrimSpace(fmt.Sprintf("%v", firstNonNil(row["position"], row["toPosition"], row["p"])))); position != "" {
		update["p"] = compactBattlePositionCode(position)
	}
	if basePosition := normalizeBattleTargetPosition(strings.TrimSpace(fmt.Sprintf("%v", firstNonNil(row["basePosition"], row["bp"])))); basePosition != "" {
		update["bp"] = compactBattlePositionCode(basePosition)
	}
	if side := normalizeBattleSide(strings.TrimSpace(fmt.Sprintf("%v", firstNonNil(row["side"], row["s"])))); side != "" {
		update["s"] = side
	}
	if baseSide := normalizeBattleSide(strings.TrimSpace(fmt.Sprintf("%v", firstNonNil(row["baseSide"], row["bs"])))); baseSide != "" {
		update["bs"] = baseSide
	}
	if perspectiveSide := normalizeBattleSide(strings.TrimSpace(fmt.Sprintf("%v", firstNonNil(row["perspectiveSide"], row["ps"])))); perspectiveSide != "" {
		update["ps"] = perspectiveSide
	}
	if value := normalizeFloatMapAny(row["stats"]); len(value) > 0 {
		update["stats"] = value
	}
	if value := normalizeFloatMapAny(row["baseStats"]); len(value) > 0 {
		update["baseStats"] = value
	}
	if value := normalizeFloatMapAny(firstNonNil(row["deltaStats"], row["statModifiers"])); len(value) > 0 {
		update["statModifiers"] = value
	}
	if value := normalizeAnyMap(row["statSources"]); len(value) > 0 {
		update["statSources"] = cloneAnyMap(value)
	}
	if value := battleAnyMapSlice(row["buffs"]); len(value) > 0 {
		update["buffs"] = value
	}
}

func compactBattleUnitUpdates(updates map[string]map[string]any) []map[string]any {
	if len(updates) == 0 {
		return nil
	}
	out := make([]map[string]any, 0, len(updates))
	for _, row := range updates {
		unitID := strings.TrimSpace(fmt.Sprintf("%v", row["u"]))
		if unitID == "" {
			continue
		}
		item := map[string]any{"u": unitID}
		if value, ok := row["h"]; ok {
			item["h"] = value
		}
		if value, ok := row["m"]; ok {
			item["m"] = value
		}
		if value, ok := row["l"]; ok {
			item["l"] = value
		}
		if value, ok := row["cc"]; ok {
			item["cc"] = value
		}
		if value, ok := row["x"]; ok {
			item["x"] = value
		}
		if value, ok := row["y"]; ok {
			item["y"] = value
		}
		if value, ok := row["p"]; ok {
			item["p"] = value
		}
		if value, ok := row["bp"]; ok {
			item["bp"] = value
		}
		if value, ok := row["s"]; ok {
			item["s"] = value
		}
		if value, ok := row["bs"]; ok {
			item["bs"] = value
		}
		if value, ok := row["ps"]; ok {
			item["ps"] = value
		}
		if value, ok := row["stats"]; ok {
			item["stats"] = value
		}
		if value, ok := row["baseStats"]; ok {
			item["baseStats"] = value
		}
		if value, ok := row["statModifiers"]; ok {
			item["statModifiers"] = value
		}
		if value, ok := row["statSources"]; ok {
			item["statSources"] = value
		}
		if value, ok := row["buffs"]; ok {
			item["buffs"] = value
		}
		out = append(out, item)
	}
	return out
}

func normalizeFloatMapAny(raw any) map[string]any {
	switch typed := raw.(type) {
	case map[string]float64:
		out := make(map[string]any, len(typed))
		for key, value := range typed {
			normalized := strings.TrimSpace(key)
			if normalized == "" {
				continue
			}
			out[normalized] = roundTo4(value)
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, value := range typed {
			normalized := strings.TrimSpace(key)
			if normalized == "" {
				continue
			}
			out[normalized] = roundTo4(toFloat64(value, 0))
		}
		return out
	default:
		return nil
	}
}

func compactBattleTimelineSummon(summon map[string]any) map[string]any {
	if len(summon) == 0 {
		return nil
	}
	out := map[string]any{
		"u": strings.TrimSpace(fmt.Sprintf("%v", firstNonNil(summon["unitId"], summon["u"]))),
		"s": strings.TrimSpace(strings.ToLower(fmt.Sprintf("%v", firstNonNil(summon["side"], summon["s"])))),
		"n": strings.TrimSpace(fmt.Sprintf("%v", firstNonNil(summon["name"], summon["n"]))),
		"k": strings.TrimSpace(fmt.Sprintf("%v", firstNonNil(summon["nameKey"], summon["k"]))),
		"p": strings.TrimSpace(fmt.Sprintf("%v", firstNonNil(summon["position"], summon["p"]))),
		"h": roundTo4(toFloat64Loose(firstNonNil(summon["hp"], summon["h"]), 0)),
		"m": roundTo4(toFloat64Loose(firstNonNil(summon["mp"], summon["m"]), 0)),
		"x": roundTo4(toFloat64Loose(firstNonNil(summon["maxHp"], summon["x"]), 0)),
		"y": roundTo4(toFloat64Loose(firstNonNil(summon["maxMp"], summon["y"]), 0)),
	}
	if out["u"] == "" {
		return nil
	}
	return out
}

func battleTimelineKindCode(kind string) string {
	switch strings.TrimSpace(strings.ToLower(kind)) {
	case "cast":
		return "c"
	case "damage":
		return "d"
	case "heal":
		return "h"
	case "resource":
		return "r"
	case "buff":
		return "b"
	case "buff_charge":
		return "bc"
	case "summon":
		return "s"
	case "summon_expire":
		return "se"
	case "corpse_focus":
		return "cf"
	case "corpse_consume":
		return "cc"
	case "trigger":
		return "t"
	case "position_move":
		return "pm"
	default:
		return kind
	}
}

func battleTimelineKindFromCode(code string) string {
	switch strings.TrimSpace(strings.ToLower(code)) {
	case "c":
		return "cast"
	case "d":
		return "damage"
	case "h":
		return "heal"
	case "r":
		return "resource"
	case "b":
		return "buff"
	case "bc":
		return "buff_charge"
	case "s":
		return "summon"
	case "se":
		return "summon_expire"
	case "cf":
		return "corpse_focus"
	case "cc":
		return "corpse_consume"
	case "t":
		return "trigger"
	case "pm":
		return "position_move"
	default:
		return code
	}
}

func battleAnyMapSlice(raw any) []map[string]any {
	switch typed := raw.(type) {
	case []map[string]any:
		out := make([]map[string]any, 0, len(typed))
		out = append(out, typed...)
		return out
	case []fiber.Map:
		out := make([]map[string]any, 0, len(typed))
		for _, row := range typed {
			if row == nil {
				continue
			}
			out = append(out, row)
		}
		return out
	case []any:
		out := make([]map[string]any, 0, len(typed))
		for _, row := range typed {
			item := normalizeAnyMap(row)
			if len(item) == 0 {
				continue
			}
			out = append(out, item)
		}
		return out
	default:
		return nil
	}
}

func normalizeAnyMap(raw any) map[string]any {
	if raw == nil {
		return nil
	}
	if typed, ok := raw.(map[string]any); ok {
		return typed
	}
	if typed, ok := raw.(fiber.Map); ok {
		return typed
	}
	buf, err := json.Marshal(raw)
	if err != nil {
		return nil
	}
	out := map[string]any{}
	if err := json.Unmarshal(buf, &out); err != nil {
		return nil
	}
	return out
}

func applyPlaybackEventsToCheckpoint(checkpoint *battlePlaybackCheckpoint, events []wsbus.BattleTimelineEvent) {
	for _, event := range events {
		if event.Seq > checkpoint.Seq {
			checkpoint.Seq = event.Seq
		}
		eventType, data := battleTimelineEventPayloadParts(event)
		switch strings.TrimSpace(eventType) {
		case "battle.wave.spawn":
			if data != nil {
				if wave := int(toInt64Loose(data["wave"], int64(checkpoint.Wave))); wave > 0 {
					checkpoint.Wave = wave
				}
				if prepRaw, ok := data["preparationOrder"]; ok {
					checkpoint.PreparationOrder = decodeBattleStringSlice(prepRaw)
				}
				if orderRaw, ok := data["order"]; ok {
					checkpoint.Order = decodeBattleStringSlice(orderRaw)
				}
				if players := decodeFiberMapSlice(data["players"]); len(players) > 0 {
					checkpoint.Players = players
				}
				if enemies := decodeFiberMapSlice(data["enemies"]); len(enemies) > 0 {
					checkpoint.Enemies = enemies
				}
				if battlefieldEffects := battleAnyMapSlice(data["battlefieldEffects"]); len(battlefieldEffects) > 0 {
					checkpoint.BattlefieldEffects = battlefieldEffects
				}
			}
		case "battle.wave.win":
			if data != nil {
				checkpoint.CompletedBattles = int(toInt64Loose(data["completedBattles"], int64(checkpoint.CompletedBattles)))
				if wave := int(toInt64Loose(data["wave"], int64(checkpoint.Wave))); wave > 0 {
					checkpoint.Wave = wave
				}
			}
		case "battle.timeline.node":
			applyPlaybackTimelineNode(checkpoint, data)
		case "battle.turn.item_use":
			if applyPlaybackActionLog(checkpoint, data["actionLog"]) {
				break
			}
			applyPlaybackTargetSummary(checkpoint, data["targets"])
		case "battle.turn.action":
			if orderRaw, ok := data["order"]; ok {
				checkpoint.Order = decodeBattleStringSlice(orderRaw)
			}
			if applyPlaybackActionLog(checkpoint, data["actionLog"]) {
				break
			}
			applyPlaybackTargetSummary(checkpoint, data["targets"])
		case "battle.turn.regen":
			if applyPlaybackActionLog(checkpoint, data["actions"]) {
				break
			}
		case "battle.field.tick", "battle.field.window":
			applyPlaybackBattlefieldEffectState(checkpoint, data["state"])
		case "battle.field.trigger":
			applyPlaybackBattlefieldEffectState(checkpoint, data["state"])
			applyPlaybackBattlefieldTargets(checkpoint, data["targets"])
		}
	}
}

func applyPlaybackTimelineNode(checkpoint *battlePlaybackCheckpoint, data map[string]any) {
	if checkpoint == nil || data == nil {
		return
	}
	if wave := int(toInt64Loose(data["w"], toInt64Loose(data["wave"], int64(checkpoint.Wave)))); wave > 0 {
		checkpoint.Wave = wave
	}
	if prepRaw, ok := data["po"]; ok {
		checkpoint.PreparationOrder = decodeBattleStringSlice(prepRaw)
	} else if prepRaw, ok := data["preparationOrder"]; ok {
		checkpoint.PreparationOrder = decodeBattleStringSlice(prepRaw)
	}
	if orderRaw, ok := data["ro"]; ok {
		checkpoint.Order = decodeBattleStringSlice(orderRaw)
	} else if orderRaw, ok := data["order"]; ok {
		checkpoint.Order = decodeBattleStringSlice(orderRaw)
	}
	steps := battleAnyMapSlice(data["p"])
	if len(steps) == 0 {
		steps = battleAnyMapSlice(data["steps"])
	}
	for _, step := range steps {
		kind := battleTimelineKindFromCode(strings.TrimSpace(fmt.Sprintf("%v", firstNonNil(step["k"], step["kind"]))))
		switch kind {
		case "summon":
			applyPlaybackSummonStep(checkpoint, step)
		case "summon_expire":
			applyPlaybackSummonExpireStep(checkpoint, step)
		}
	}
	applyPlaybackTimelineCooldowns(checkpoint, data, steps)
	unitUpdates := battleAnyMapSlice(data["u"])
	if len(unitUpdates) == 0 {
		unitUpdates = battleAnyMapSlice(data["unitUpdates"])
	}
	for _, update := range unitUpdates {
		applyPlaybackStepTarget(checkpoint, update)
	}
}

func applyPlaybackTimelineCooldowns(checkpoint *battlePlaybackCheckpoint, data map[string]any, steps []map[string]any) {
	if checkpoint == nil || data == nil {
		return
	}
	ownerID := strings.TrimSpace(fmt.Sprintf("%v", firstNonNil(data["o"], data["turnOwnerId"], data["actorId"])))
	if ownerID == "" {
		if owner := normalizeAnyMap(data["turnOwner"]); len(owner) > 0 {
			ownerID = strings.TrimSpace(fmt.Sprintf("%v", firstNonNil(owner["unitId"], owner["u"])))
		}
	}
	if ownerID == "" {
		return
	}
	var unit fiber.Map
	if matched, ok := playbackFindUnitMap(checkpoint.Players, ownerID); ok {
		unit = matched
	} else if matched, ok := playbackFindUnitMap(checkpoint.Enemies, ownerID); ok {
		unit = matched
	}
	if unit == nil {
		return
	}
	if rawItemCooldowns, ok := data["ic"]; ok {
		applyPlaybackAuthoritativeItemCooldowns(unit, rawItemCooldowns)
	} else if rawItemCooldowns, ok := data["itemCooldowns"]; ok {
		applyPlaybackAuthoritativeItemCooldowns(unit, rawItemCooldowns)
	}
	rawSkillCooldowns, hasAuthoritativeSkillCooldowns := data["sc"]
	if !hasAuthoritativeSkillCooldowns {
		rawSkillCooldowns, hasAuthoritativeSkillCooldowns = data["skillCooldowns"]
	}
	if hasAuthoritativeSkillCooldowns {
		rawTotals := firstNonNil(data["st"], data["skillCooldownTurns"])
		applyPlaybackAuthoritativeSkillCooldowns(unit, rawSkillCooldowns, rawTotals)
		return
	}
	castSkillID := ""
	for _, step := range steps {
		kind := battleTimelineKindFromCode(strings.TrimSpace(fmt.Sprintf("%v", firstNonNil(step["k"], step["kind"]))))
		if kind != "cast" {
			continue
		}
		castSkillID = strings.TrimSpace(fmt.Sprintf("%v", firstNonNil(step["i"], step["skillId"])))
		if castSkillID != "" {
			break
		}
	}
	cooldowns := playbackUnitCooldownMap(unit)
	if castSkillID != "" {
		cooldowns[castSkillID] = playbackSkillCooldownTurns(unit, castSkillID)
	}
	for skillID, remain := range cooldowns {
		if remain > 0 {
			cooldowns[skillID] = remain - 1
		}
	}
	unit["skillCooldowns"] = cooldowns
	skills := battleAnyMapSlice(unit["skills"])
	if len(skills) > 0 {
		for _, skill := range skills {
			skillID := strings.TrimSpace(fmt.Sprintf("%v", skill["id"]))
			if skillID == "" {
				continue
			}
			skill["cooldownRemain"] = cooldowns[skillID]
		}
		rows := make([]any, 0, len(skills))
		for _, skill := range skills {
			rows = append(rows, skill)
		}
		unit["skills"] = rows
	}
}

func applyPlaybackAuthoritativeSkillCooldowns(unit fiber.Map, rawCooldowns any, rawTotals any) {
	if unit == nil {
		return
	}
	cooldowns := map[string]int64{}
	for key, value := range normalizeAnyMap(rawCooldowns) {
		skillID := strings.TrimSpace(key)
		if skillID != "" {
			cooldowns[skillID] = maxInt64(0, toInt64Loose(value, 0))
		}
	}
	totals := map[string]int64{}
	for key, value := range normalizeAnyMap(rawTotals) {
		skillID := strings.TrimSpace(key)
		if skillID != "" {
			totals[skillID] = maxInt64(0, toInt64Loose(value, 0))
		}
	}
	unit["skillCooldowns"] = cooldowns
	if rawTotals != nil {
		unit["skillCooldownTurns"] = totals
	}
	skills := battleAnyMapSlice(unit["skills"])
	if len(skills) == 0 {
		return
	}
	for _, skill := range skills {
		skillID := strings.TrimSpace(fmt.Sprintf("%v", skill["id"]))
		if skillID == "" {
			continue
		}
		if remain, ok := cooldowns[skillID]; ok {
			skill["cooldownRemain"] = remain
		}
		if total, ok := totals[skillID]; ok {
			skill["cooldown"] = total
			skill["cooldownTurns"] = total
		}
	}
	rows := make([]any, 0, len(skills))
	for _, skill := range skills {
		rows = append(rows, skill)
	}
	unit["skills"] = rows
}

func applyPlaybackAuthoritativeItemCooldowns(unit fiber.Map, rawCooldowns any) {
	if unit == nil {
		return
	}
	cooldowns := map[string]int64{}
	for key, value := range normalizeAnyMap(rawCooldowns) {
		itemID := strings.TrimSpace(key)
		if itemID != "" {
			cooldowns[itemID] = maxInt64(0, toInt64Loose(value, 0))
		}
	}
	unit["itemCooldowns"] = cooldowns
	items := battleAnyMapSlice(unit["battleItems"])
	if len(items) == 0 {
		return
	}
	for _, item := range items {
		itemID := strings.TrimSpace(fmt.Sprintf("%v", firstNonNil(item["itemId"], item["id"])))
		if remain, ok := cooldowns[itemID]; ok {
			item["cooldownRemain"] = remain
		}
	}
	rows := make([]any, 0, len(items))
	for _, item := range items {
		rows = append(rows, item)
	}
	unit["battleItems"] = rows
}

func playbackUnitCooldownMap(unit fiber.Map) map[string]int64 {
	out := map[string]int64{}
	for key, value := range normalizeAnyMap(unit["skillCooldowns"]) {
		skillID := strings.TrimSpace(key)
		if skillID != "" {
			out[skillID] = maxInt64(0, toInt64Loose(value, 0))
		}
	}
	for _, skill := range battleAnyMapSlice(unit["skills"]) {
		skillID := strings.TrimSpace(fmt.Sprintf("%v", skill["id"]))
		if skillID == "" {
			continue
		}
		if _, exists := out[skillID]; exists {
			continue
		}
		out[skillID] = maxInt64(0, toInt64Loose(skill["cooldownRemain"], 0))
	}
	return out
}

func playbackSkillCooldownTurns(unit fiber.Map, skillID string) int64 {
	for _, skill := range battleAnyMapSlice(unit["skills"]) {
		if strings.TrimSpace(fmt.Sprintf("%v", skill["id"])) != skillID {
			continue
		}
		return maxInt64(0, toInt64Loose(firstNonNil(skill["cooldownTurns"], skill["cooldown"]), 0))
	}
	return 0
}

func applyPlaybackStepTarget(checkpoint *battlePlaybackCheckpoint, target map[string]any) {
	targetID := strings.TrimSpace(fmt.Sprintf("%v", firstNonNil(target["u"], target["unitId"])))
	if targetID == "" {
		targetID = strings.TrimSpace(fmt.Sprintf("%v", target["targetId"]))
	}
	if targetID == "" {
		return
	}
	if unit, ok := playbackFindUnitMap(checkpoint.Players, targetID); ok {
		applyPlaybackStepTargetToUnit(unit, target)
		playbackRelocateCheckpointUnitBySide(checkpoint, targetID)
		return
	}
	if unit, ok := playbackFindUnitMap(checkpoint.Enemies, targetID); ok {
		applyPlaybackStepTargetToUnit(unit, target)
		playbackRelocateCheckpointUnitBySide(checkpoint, targetID)
	}
}

func applyPlaybackStepTargetToUnit(unit fiber.Map, target map[string]any) {
	if unit == nil {
		return
	}
	if value, ok := target["h"]; ok {
		unit["hp"] = toFloat64Loose(value, toFloat64Loose(unit["hp"], 0))
	} else if value, ok := target["hpAfter"]; ok {
		unit["hp"] = toFloat64Loose(value, toFloat64Loose(unit["hp"], 0))
	}
	if value, ok := target["m"]; ok {
		unit["mp"] = toFloat64Loose(value, toFloat64Loose(unit["mp"], 0))
	} else if value, ok := target["mpAfter"]; ok {
		unit["mp"] = toFloat64Loose(value, toFloat64Loose(unit["mp"], 0))
	}
	if alive, ok := target["l"].(bool); ok {
		unit["alive"] = alive
	} else if alive, ok := target["aliveAfter"].(bool); ok {
		unit["alive"] = alive
	}
	if consumed, ok := target["cc"].(bool); ok {
		unit["corpseConsumed"] = consumed
	} else if consumed, ok := target["corpseConsumed"].(bool); ok {
		unit["corpseConsumed"] = consumed
	}
	if value, ok := target["maxHpAfter"]; ok {
		unit["maxHp"] = toFloat64Loose(value, toFloat64Loose(unit["maxHp"], 0))
	} else if value, ok := target["x"]; ok {
		unit["maxHp"] = toFloat64Loose(value, toFloat64Loose(unit["maxHp"], 0))
	}
	if value, ok := target["maxMpAfter"]; ok {
		unit["maxMp"] = toFloat64Loose(value, toFloat64Loose(unit["maxMp"], 0))
	} else if value, ok := target["y"]; ok {
		unit["maxMp"] = toFloat64Loose(value, toFloat64Loose(unit["maxMp"], 0))
	}
	if value, ok := target["maxHp"]; ok {
		unit["maxHp"] = toFloat64Loose(value, toFloat64Loose(unit["maxHp"], 0))
	}
	if value, ok := target["maxMp"]; ok {
		unit["maxMp"] = toFloat64Loose(value, toFloat64Loose(unit["maxMp"], 0))
	}
	if position := expandBattlePositionCode(firstNonNil(target["p"], target["toPosition"], target["t"], target["position"])); position != "" {
		unit["position"] = position
	}
	if basePosition := expandBattlePositionCode(firstNonNil(target["bp"], target["basePosition"])); basePosition != "" {
		unit["basePosition"] = basePosition
	}
	if side := normalizeBattleSide(strings.TrimSpace(fmt.Sprintf("%v", firstNonNil(target["s"], target["side"])))); side != "" {
		unit["side"] = side
	}
	if baseSide := normalizeBattleSide(strings.TrimSpace(fmt.Sprintf("%v", firstNonNil(target["bs"], target["baseSide"])))); baseSide != "" {
		unit["baseSide"] = baseSide
	}
	if perspectiveSide := normalizeBattleSide(strings.TrimSpace(fmt.Sprintf("%v", firstNonNil(target["ps"], target["perspectiveSide"])))); perspectiveSide != "" {
		unit["perspectiveSide"] = perspectiveSide
	}
	if value, ok := target["stats"].(map[string]any); ok {
		unit["stats"] = cloneAnyMap(value)
		unit["attributes"] = cloneAnyMap(value)
	}
	if value, ok := target["baseStats"].(map[string]any); ok {
		unit["baseStats"] = cloneAnyMap(value)
	}
	if value, ok := target["statModifiers"].(map[string]any); ok {
		unit["statModifiers"] = cloneAnyMap(value)
	}
	if value, ok := target["deltaStats"].(map[string]any); ok {
		unit["deltaStats"] = cloneAnyMap(value)
		unit["statModifiers"] = cloneAnyMap(value)
	}
	if value, ok := target["statSources"].(map[string]any); ok {
		unit["statSources"] = cloneAnyMap(value)
	}
	if value, ok := target["buffs"].([]any); ok {
		unit["buffs"] = value
	}
	if value, ok := target["buffs"].([]map[string]any); ok {
		rows := make([]any, 0, len(value))
		for _, row := range value {
			rows = append(rows, row)
		}
		unit["buffs"] = rows
	}
}

func playbackRelocateCheckpointUnitBySide(checkpoint *battlePlaybackCheckpoint, unitID string) {
	unitID = strings.TrimSpace(unitID)
	if checkpoint == nil || unitID == "" {
		return
	}
	side := ""
	var unit fiber.Map
	var foundInPlayers bool
	for idx := range checkpoint.Players {
		candidate := checkpoint.Players[idx]
		if strings.TrimSpace(fmt.Sprintf("%v", candidate["unitId"])) != unitID {
			continue
		}
		unit = candidate
		side = normalizeBattleSide(strings.TrimSpace(fmt.Sprintf("%v", candidate["side"])))
		foundInPlayers = true
		break
	}
	if unit == nil {
		for idx := range checkpoint.Enemies {
			candidate := checkpoint.Enemies[idx]
			if strings.TrimSpace(fmt.Sprintf("%v", candidate["unitId"])) != unitID {
				continue
			}
			unit = candidate
			side = normalizeBattleSide(strings.TrimSpace(fmt.Sprintf("%v", candidate["side"])))
			break
		}
	}
	if unit == nil || side == "" {
		return
	}
	if side == "enemy" && foundInPlayers {
		checkpoint.Players = playbackRemoveUnitByID(checkpoint.Players, unitID)
		if _, exists := playbackFindUnitMap(checkpoint.Enemies, unitID); !exists {
			checkpoint.Enemies = append(checkpoint.Enemies, unit)
		}
		return
	}
	if side == "player" && !foundInPlayers {
		checkpoint.Enemies = playbackRemoveUnitByID(checkpoint.Enemies, unitID)
		if _, exists := playbackFindUnitMap(checkpoint.Players, unitID); !exists {
			checkpoint.Players = append(checkpoint.Players, unit)
		}
	}
}

func playbackRemoveUnitByID(rows []fiber.Map, unitID string) []fiber.Map {
	if len(rows) == 0 {
		return rows
	}
	unitID = strings.TrimSpace(unitID)
	out := make([]fiber.Map, 0, len(rows))
	for _, row := range rows {
		if strings.TrimSpace(fmt.Sprintf("%v", row["unitId"])) == unitID {
			continue
		}
		out = append(out, row)
	}
	return out
}

func applyPlaybackSummonStep(checkpoint *battlePlaybackCheckpoint, step map[string]any) {
	summon := normalizeAnyMap(firstNonNil(step["q"], step["summon"]))
	if len(summon) == 0 {
		return
	}
	unitID := strings.TrimSpace(fmt.Sprintf("%v", firstNonNil(summon["unitId"], summon["u"])))
	if unitID == "" {
		return
	}
	side := strings.ToLower(strings.TrimSpace(fmt.Sprintf("%v", firstNonNil(summon["side"], summon["s"]))))
	row := fiber.Map{
		"unitId":   unitID,
		"side":     side,
		"name":     strings.TrimSpace(fmt.Sprintf("%v", firstNonNil(summon["name"], summon["n"]))),
		"nameKey":  strings.TrimSpace(fmt.Sprintf("%v", firstNonNil(summon["nameKey"], summon["k"]))),
		"position": strings.TrimSpace(fmt.Sprintf("%v", firstNonNil(summon["position"], summon["p"]))),
		"hp":       toFloat64Loose(firstNonNil(summon["hp"], summon["h"]), 0),
		"mp":       toFloat64Loose(firstNonNil(summon["mp"], summon["m"]), 0),
		"maxHp":    toFloat64Loose(firstNonNil(summon["maxHp"], summon["x"]), 0),
		"maxMp":    toFloat64Loose(firstNonNil(summon["maxMp"], summon["y"]), 0),
		"alive":    true,
		"skills":   []fiber.Map{},
		"buffs":    []fiber.Map{},
	}
	if side == "enemy" {
		if _, exists := playbackFindUnitMap(checkpoint.Enemies, unitID); !exists {
			checkpoint.Enemies = append(checkpoint.Enemies, row)
		}
		return
	}
	if _, exists := playbackFindUnitMap(checkpoint.Players, unitID); !exists {
		checkpoint.Players = append(checkpoint.Players, row)
	}
}

func applyPlaybackSummonExpireStep(checkpoint *battlePlaybackCheckpoint, step map[string]any) {
	if checkpoint == nil {
		return
	}
	targetID := strings.TrimSpace(fmt.Sprintf("%v", firstNonNil(step["u"], step["targetId"])))
	if targetID == "" {
		targets := battleAnyMapSlice(step["t"])
		if len(targets) > 0 {
			targetID = strings.TrimSpace(fmt.Sprintf("%v", firstNonNil(targets[0]["u"], targets[0]["targetId"])))
		}
	}
	if targetID == "" {
		return
	}
	checkpoint.Players = playbackRemoveUnitByID(checkpoint.Players, targetID)
	checkpoint.Enemies = playbackRemoveUnitByID(checkpoint.Enemies, targetID)
}

func firstNonNil(values ...any) any {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func applyPlaybackActionLog(checkpoint *battlePlaybackCheckpoint, raw any) bool {
	actions, _ := raw.([]any)
	if len(actions) == 0 {
		return false
	}
	for _, row := range actions {
		action, _ := row.(map[string]any)
		targetID, _ := action["targetId"].(string)
		targetID = strings.TrimSpace(targetID)
		kind := strings.ToLower(strings.TrimSpace(fmt.Sprintf("%v", action["kind"])))
		switch kind {
		case "damage", "heal", "resource", "buff", "corpse_consume":
			if targetID == "" {
				continue
			}
			if unit, ok := playbackFindUnitMap(checkpoint.Players, targetID); ok {
				applyPlaybackActionToUnit(unit, action)
				continue
			}
			if unit, ok := playbackFindUnitMap(checkpoint.Enemies, targetID); ok {
				applyPlaybackActionToUnit(unit, action)
			}
		case "summon_expire":
			if targetID == "" {
				continue
			}
			checkpoint.Players = playbackRemoveUnitByID(checkpoint.Players, targetID)
			checkpoint.Enemies = playbackRemoveUnitByID(checkpoint.Enemies, targetID)
		case "summon":
			summon, _ := action["summon"].(map[string]any)
			summonID := strings.TrimSpace(fmt.Sprintf("%v", summon["unitId"]))
			if summonID == "" {
				continue
			}
			side := strings.ToLower(strings.TrimSpace(fmt.Sprintf("%v", summon["side"])))
			row := fiber.Map{
				"unitId":   summonID,
				"side":     side,
				"name":     strings.TrimSpace(fmt.Sprintf("%v", summon["name"])),
				"nameKey":  strings.TrimSpace(fmt.Sprintf("%v", summon["nameKey"])),
				"position": strings.TrimSpace(fmt.Sprintf("%v", summon["position"])),
				"hp":       toFloat64Loose(summon["hp"], 0),
				"mp":       toFloat64Loose(summon["mp"], 0),
				"maxHp":    toFloat64Loose(summon["maxHp"], 0),
				"maxMp":    toFloat64Loose(summon["maxMp"], 0),
				"alive":    true,
				"skills":   []fiber.Map{},
				"buffs":    []fiber.Map{},
			}
			if side == "enemy" {
				if _, exists := playbackFindUnitMap(checkpoint.Enemies, summonID); !exists {
					checkpoint.Enemies = append(checkpoint.Enemies, row)
				}
				continue
			}
			if _, exists := playbackFindUnitMap(checkpoint.Players, summonID); !exists {
				checkpoint.Players = append(checkpoint.Players, row)
			}
		}
	}
	return true
}

func applyPlaybackActionToUnit(unit fiber.Map, action map[string]any) {
	if unit == nil {
		return
	}
	if afterHP, ok := action["afterHp"]; ok {
		unit["hp"] = toFloat64Loose(afterHP, toFloat64Loose(unit["hp"], 0))
	}
	if afterMP, ok := action["afterMp"]; ok {
		unit["mp"] = toFloat64Loose(afterMP, toFloat64Loose(unit["mp"], 0))
	}
	if afterAlive, ok := action["afterAlive"].(bool); ok {
		unit["alive"] = afterAlive
	}
	if consumed, ok := action["corpseConsumed"].(bool); ok {
		unit["corpseConsumed"] = consumed
	}
}

func applyPlaybackTargetSummary(checkpoint *battlePlaybackCheckpoint, raw any) {
	targets, _ := raw.([]any)
	for _, row := range targets {
		target, _ := row.(map[string]any)
		targetID, _ := target["targetId"].(string)
		targetID = strings.TrimSpace(targetID)
		if targetID == "" {
			continue
		}
		if unit, ok := playbackFindUnitMap(checkpoint.Players, targetID); ok {
			unit["hp"] = toFloat64Loose(target["hp"], toFloat64Loose(unit["hp"], 0))
			unit["mp"] = toFloat64Loose(target["mp"], toFloat64Loose(unit["mp"], 0))
			if alive, ok := target["alive"].(bool); ok {
				unit["alive"] = alive
			}
			continue
		}
		if unit, ok := playbackFindUnitMap(checkpoint.Enemies, targetID); ok {
			unit["hp"] = toFloat64Loose(target["hp"], toFloat64Loose(unit["hp"], 0))
			unit["mp"] = toFloat64Loose(target["mp"], toFloat64Loose(unit["mp"], 0))
			if alive, ok := target["alive"].(bool); ok {
				unit["alive"] = alive
			}
		}
	}
}

func applyPlaybackBattlefieldEffectState(checkpoint *battlePlaybackCheckpoint, raw any) {
	if checkpoint == nil {
		return
	}
	state := normalizeAnyMap(raw)
	id := strings.TrimSpace(fmt.Sprintf("%v", firstNonNil(state["id"], state["i"])))
	if id == "" {
		return
	}
	for index := range checkpoint.BattlefieldEffects {
		currentID := strings.TrimSpace(fmt.Sprintf("%v", firstNonNil(checkpoint.BattlefieldEffects[index]["id"], checkpoint.BattlefieldEffects[index]["i"])))
		if currentID == id {
			checkpoint.BattlefieldEffects[index] = state
			return
		}
	}
	checkpoint.BattlefieldEffects = append(checkpoint.BattlefieldEffects, state)
}

func applyPlaybackBattlefieldTargets(checkpoint *battlePlaybackCheckpoint, raw any) {
	if checkpoint == nil {
		return
	}
	for _, target := range battleAnyMapSlice(raw) {
		unitID := strings.TrimSpace(fmt.Sprintf("%v", firstNonNil(target["unitId"], target["targetId"])))
		if unitID == "" {
			continue
		}
		unit, ok := playbackFindUnitMap(checkpoint.Players, unitID)
		if !ok {
			unit, ok = playbackFindUnitMap(checkpoint.Enemies, unitID)
		}
		if !ok {
			continue
		}
		unit["hp"] = toFloat64Loose(firstNonNil(target["afterHp"], target["hp"]), toFloat64Loose(unit["hp"], 0))
		if alive, ok := target["alive"].(bool); ok {
			unit["alive"] = alive
		}
	}
}

func playbackFindUnitMap(rows []fiber.Map, unitID string) (fiber.Map, bool) {
	for _, row := range rows {
		if strings.TrimSpace(fmt.Sprintf("%v", row["unitId"])) == unitID {
			return row, true
		}
	}
	return nil, false
}

func toFloat64Loose(value any, fallback float64) float64 {
	switch typed := value.(type) {
	case float64:
		return typed
	case float32:
		return float64(typed)
	case int:
		return float64(typed)
	case int32:
		return float64(typed)
	case int64:
		return float64(typed)
	case json.Number:
		if parsed, err := typed.Float64(); err == nil {
			return parsed
		}
	}
	return fallback
}

func encodeBattleRewardPlan(plan map[int64]store.BattleWaveReward) []battleRewardGrant {
	if len(plan) == 0 {
		return []battleRewardGrant{}
	}
	out := make([]battleRewardGrant, 0, len(plan))
	for userID, reward := range plan {
		if userID <= 0 {
			continue
		}
		kittyAttrExp := make(map[int64]map[string]float64, len(reward.KittyAttrExp))
		for kittyID, attrs := range reward.KittyAttrExp {
			kittyAttrExp[kittyID] = cloneFloatMap(attrs)
		}
		out = append(out, battleRewardGrant{
			Contributions: reward.Contributions,
			UserID:        userID,
			ItemRewards:   cloneInt64Map(reward.ItemRewards),
			AttrExp:       cloneFloatMap(reward.AttrExp),
			KittyAttrExp:  kittyAttrExp,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UserID < out[j].UserID })
	return out
}

func battleAchievementStatsFromState(state battleRuntimeState) map[string]any {
	enemyKills := cloneInt64Map(state.AchievementStats.EnemyKills)
	killedUnits := map[string]struct{}{}
	for unitID, killed := range state.AchievementStats.KilledEnemyUnits {
		if killed {
			killedUnits[strings.TrimSpace(unitID)] = struct{}{}
		}
	}
	for _, enemy := range state.Enemies {
		if enemy.Alive || strings.TrimSpace(enemy.EnemyID) == "" {
			continue
		}
		unitID := strings.TrimSpace(enemy.UnitID)
		if unitID != "" {
			if _, exists := killedUnits[unitID]; exists {
				inferredEnemyID := battleEnemyIDFromUnitID(unitID)
				actualEnemyID := strings.TrimSpace(enemy.EnemyID)
				if actualEnemyID != "" && inferredEnemyID != "" && actualEnemyID != inferredEnemyID && enemyKills[inferredEnemyID] > 0 {
					enemyKills[inferredEnemyID]--
					if enemyKills[inferredEnemyID] <= 0 {
						delete(enemyKills, inferredEnemyID)
					}
					enemyKills[actualEnemyID]++
				}
				continue
			}
			killedUnits[unitID] = struct{}{}
		}
		enemyKills[strings.TrimSpace(enemy.EnemyID)]++
	}
	return map[string]any{
		"enemyKills":       enemyKills,
		"skillCasts":       cloneInt64Map(state.AchievementStats.SkillCasts),
		"skillCastsByUnit": cloneNestedInt64Map(state.AchievementStats.SkillCastsByUnit),
		"basicAttacks":     state.AchievementStats.BasicAttacks,
		"summons":          state.AchievementStats.Summons,
		"maxSingleDamage":  roundTo4(state.AchievementStats.MaxSingleDamage),
	}
}

func cloneInt64Map(in map[string]int64) map[string]int64 {
	if len(in) == 0 {
		return map[string]int64{}
	}
	out := make(map[string]int64, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func cloneNestedInt64Map(in map[string]map[string]int64) map[string]map[string]int64 {
	if len(in) == 0 {
		return map[string]map[string]int64{}
	}
	out := make(map[string]map[string]int64, len(in))
	for key, child := range in {
		if len(child) == 0 {
			out[key] = map[string]int64{}
			continue
		}
		next := make(map[string]int64, len(child))
		for childKey, value := range child {
			next[childKey] = value
		}
		out[key] = next
	}
	return out
}

func decodeBattleRewardPlan(finalResult map[string]any) map[int64]store.BattleWaveReward {
	planRaw, ok := finalResult["rewardPlan"]
	if !ok {
		return map[int64]store.BattleWaveReward{}
	}
	buf, err := json.Marshal(planRaw)
	if err != nil {
		return map[int64]store.BattleWaveReward{}
	}
	decoded := []battleRewardGrant{}
	if err := json.Unmarshal(buf, &decoded); err != nil {
		return map[int64]store.BattleWaveReward{}
	}
	out := make(map[int64]store.BattleWaveReward, len(decoded))
	for _, row := range decoded {
		if row.UserID <= 0 {
			continue
		}
		out[row.UserID] = store.BattleWaveReward{
			Contributions: row.Contributions,
			ItemRewards:   row.ItemRewards,
			AttrExp:       row.AttrExp,
			KittyAttrExp:  row.KittyAttrExp,
		}
	}
	return out
}

func battleOutcomeFromFinalResult(finalResult map[string]any) string {
	outcome, _ := finalResult["outcome"].(string)
	outcome = strings.TrimSpace(strings.ToLower(outcome))
	switch outcome {
	case store.BattleRunStatusEnded, store.BattleRunStatusFailed, store.BattleRunStatusInterrupted:
		return outcome
	default:
		return store.BattleRunStatusEnded
	}
}

func battleFinalResultFlag(finalResult map[string]any, key string) bool {
	if len(finalResult) == 0 {
		return false
	}
	value, ok := finalResult[key]
	if !ok {
		return false
	}
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		normalized := strings.TrimSpace(strings.ToLower(typed))
		return normalized == "true" || normalized == "1"
	case int:
		return typed != 0
	case int64:
		return typed != 0
	case float64:
		return typed != 0
	default:
		return false
	}
}

func battleFailureReasonFromFinalResult(finalResult map[string]any) string {
	if battleFinalResultFlag(finalResult, "compileTimedOut") {
		return "compile_timeout"
	}
	if battleFinalResultFlag(finalResult, "compileStepLimit") {
		return "compile_step_limit"
	}
	return ""
}

func battleFinalizeRetryable(err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	return pgErr.Code == "40P01" || pgErr.Code == "40001"
}

func battlePublicUnitSlice(rows []battleUnitState) []fiber.Map {
	out := make([]fiber.Map, 0, len(rows))
	mutationServer := &Server{}
	for _, row := range rows {
		attackStatPreviewActor, hasAttackStatPreview := battleAttackStatPolicyPreviewActor(row)
		basicAttackDisplay := battleBasicAttackProfileForDisplay(row)
		stats := map[string]float64{}
		for key, value := range row.Stats {
			normalizedKey := strings.TrimSpace(key)
			if normalizedKey == "" {
				continue
			}
			stats[normalizedKey] = roundTo4(value)
		}
		skills := make([]fiber.Map, 0, len(row.Skills))
		skillCooldowns := map[string]int{}
		skillCooldownTurns := map[string]int{}
		skillLevels := map[string]int{}
		itemCooldowns := map[string]int{}
		itemQuantities := map[string]int64{}
		for _, skill := range row.Skills {
			normalizedSkillID := strings.TrimSpace(skill.ID)
			mutationScope := "skill"
			if strings.EqualFold(normalizedSkillID, "basic_attack") {
				mutationScope = "basic_attack"
			}
			effectiveSkill := (&row).applyBattleSkillScalarMutations(mutationScope, skill)
			if hasAttackStatPreview {
				effectiveSkill.Effects = mutationServer.applyBattleAffixEffectMutations(&attackStatPreviewActor, mutationScope, effectiveSkill.ID, effectiveSkill.Level, effectiveSkill.Targeting, effectiveSkill.Effects)
				if len(effectiveSkill.Effects) > 0 {
					effectiveSkill.Targeting = effectiveSkill.Effects[0].Targeting
				}
			}
			if normalizedSkillID != "" {
				skillCooldowns[normalizedSkillID] = maxInt(0, skill.CooldownRemain)
				skillCooldownTurns[normalizedSkillID] = maxInt(0, effectiveSkill.CooldownTurns)
				skillLevels[normalizedSkillID] = maxInt(1, skill.Level)
			}
			skills = append(skills, fiber.Map{
				"id":             skill.ID,
				"name":           skill.Name,
				"icon":           skill.Icon,
				"level":          maxInt(1, skill.Level),
				"exp":            maxInt64(0, skill.Exp),
				"hpCost":         roundTo4(effectiveSkill.HPCost),
				"mpCost":         roundTo4(effectiveSkill.MPCost),
				"cooldown":       effectiveSkill.CooldownTurns,
				"cooldownTurns":  effectiveSkill.CooldownTurns,
				"cooldownRemain": skill.CooldownRemain,
				"targeting":      effectiveSkill.Targeting,
				"effects":        effectiveSkill.Effects,
			})
		}
		battleItems := make([]fiber.Map, 0, len(row.BattleItems))
		for _, item := range row.BattleItems {
			normalizedItemID := strings.TrimSpace(item.ItemID)
			if normalizedItemID != "" {
				itemCooldowns[normalizedItemID] = maxInt(0, item.CooldownRemain)
				itemQuantities[normalizedItemID] = maxInt64(0, item.Quantity)
			}
			battleItems = append(battleItems, fiber.Map{
				"id":             item.ID,
				"itemId":         item.ItemID,
				"name":           item.Name,
				"icon":           item.Icon,
				"quantity":       maxInt64(0, item.Quantity),
				"cooldownTurns":  maxInt(0, item.CooldownTurns),
				"cooldownRemain": maxInt(0, item.CooldownRemain),
				"targeting":      item.Targeting,
				"effects":        item.Effects,
			})
		}
		buffs := make([]fiber.Map, 0, len(row.Buffs))
		for _, buff := range row.Buffs {
			buffID := strings.TrimSpace(buff.BuffID)
			if buffID == "" {
				continue
			}
			payload := map[string]any{}
			for key, value := range buff.Payload {
				normalizedKey := strings.TrimSpace(key)
				if normalizedKey == "" {
					continue
				}
				payload[normalizedKey] = value
			}
			buffs = append(buffs, fiber.Map{
				"buffId":      buffID,
				"remainTurns": buff.RemainTurns,
				"payload":     payload,
				"sourceId":    strings.TrimSpace(buff.SourceID),
			})
		}
		entry := fiber.Map{
			"unitId":               row.UnitID,
			"side":                 row.Side,
			"originSide":           row.OriginSide,
			"baseSide":             row.BaseSide,
			"perspectiveSide":      row.PerspectiveSide,
			"userId":               row.UserID,
			"enemyId":              row.EnemyID,
			"name":                 row.Name,
			"nameKey":              row.NameKey,
			"position":             row.Position,
			"basePosition":         row.BasePosition,
			"alive":                row.Alive,
			"corpseConsumed":       row.CorpseConsumed,
			"rewardEligible":       row.RewardEligible,
			"hp":                   roundTo4(row.HP),
			"mp":                   roundTo4(row.MP),
			"maxHp":                roundTo4(row.MaxHP),
			"maxMp":                roundTo4(row.MaxMP),
			"baseStats":            cloneFloatMap(row.BaseStats),
			"stats":                stats,
			"statModifiers":        cloneFloatMap(row.StatModifiers),
			"statSources":          cloneNestedFloatMap(row.StatSources),
			"skillSlots":           append([]string(nil), row.SkillSlots...),
			"skillIds":             append([]string(nil), row.SkillIDs...),
			"skillCooldowns":       skillCooldowns,
			"skillCooldownTurns":   skillCooldownTurns,
			"skillLevels":          skillLevels,
			"itemCooldowns":        itemCooldowns,
			"itemQuantities":       itemQuantities,
			"skills":               skills,
			"battleItems":          battleItems,
			"equipmentDigest":      append([]string(nil), row.EquipmentDigest...),
			"equipmentInstanceIds": append([]int64(nil), row.EquipmentIDs...),
			"basicAttack":          basicAttackDisplay,
			"buffs":                buffs,
		}
		if row.SpecializationTreeID != "" {
			entry["specializationTreeId"] = row.SpecializationTreeID
			entry["specializationRulesetVersion"] = row.SpecializationRulesetVersion
			entry["specializationBalanceRevision"] = row.SpecializationBalanceRevision
			entry["specializationRevision"] = row.SpecializationRevision
			entry["specializationDigest"] = row.SpecializationDigest
		}
		if t := strings.TrimSpace(row.SummonTemplateUnitID); t != "" {
			entry["summonTemplateUnitId"] = t
		}
		out = append(out, entry)
	}
	return out
}

func battleBasicAttackProfileForDisplay(unit battleUnitState) *battleBasicAttackProfile {
	profile := cloneBattleBasicAttackProfile(unit.BasicAttack)
	if profile == nil {
		return nil
	}
	normalizeBattleBasicAttackProfile(profile)
	fallbackTargeting := battleSkillTargeting{
		TargetPositions: []string{profile.TargetDirection},
		SecondarySeek:   profile.TargetSeekStrategy,
		TargetScope:     "enemy",
		SelectRule:      "random",
		TargetCount: battleSkillTargetCount{
			Base: 1,
		},
	}
	if len(profile.Effects) == 0 {
		spec := resolveBattleBasicAttackDamageSpec(profile)
		profile.Effects = []battleSkillEffect{
			{
				Kind:          "damage",
				DamageType:    spec.DamageType,
				Targeting:     fallbackTargeting,
				AmountFormula: spec.AmountFormula,
				DamageSpec:    spec,
			},
		}
	}
	server := &Server{}
	profile.Effects = server.applyBattleAffixEffectMutations(&unit, "basic_attack", "basic_attack", 1, fallbackTargeting, profile.Effects)
	for idx := range profile.Effects {
		normalizeBattleSkillEffect(&profile.Effects[idx])
	}
	if len(profile.Effects) > 0 {
		profile.TargetDirection = firstNonEmptyTargetPosition(profile.Effects[0].Targeting.TargetPositions)
		profile.TargetSeekStrategy = normalizeBattleSecondarySeek(profile.Effects[0].Targeting.SecondarySeek, profile.TargetSeekStrategy)
		if strings.TrimSpace(profile.Effects[0].DamageType) != "" {
			profile.DamageType = normalizeBattleDamageType(profile.Effects[0].DamageType)
		}
	}
	return profile
}

func decodeBattlePlayersFromFinalResult(raw map[string]any) []battleUnitState {
	if len(raw) == 0 {
		return nil
	}
	if players := decodeBattlePlayersFromFinalResultNode(raw["carryState"]); len(players) > 0 {
		return players
	}
	finalStateRaw, ok := raw["finalState"]
	if !ok || finalStateRaw == nil {
		return nil
	}
	players := decodeBattlePlayersFromFinalResultNode(finalStateRaw)
	for index := range players {
		unitType, formalID, err := battleSettlementFormalUnitIdentity(players[index])
		if err != nil {
			continue
		}
		players[index].UnitType = unitType
		if unitType == "kitty" {
			players[index].KittyID = formalID
		}
	}
	return players
}

var errBattlePreviousWaveCarryInvalid = errors.New("previous battle wave carry state is missing or invalid")

type battlePreviousWaveCarryState struct {
	Players                      []battleUnitState
	PreparationOrder             []string
	BattleHookBattleCounts       map[string]int
	BattleHookCooldownUntilTurns map[string]int
	CompletedTurnCounts          map[string]int
	BattlefieldEffects           []battlefieldEffectState
}

type battlePreviousWaveCarryPayload struct {
	Status                       string            `json:"status"`
	Players                      []battleUnitState `json:"players"`
	PreparationOrder             json.RawMessage   `json:"preparationOrder"`
	BattleHookBattleCounts       json.RawMessage   `json:"battleHookBattleCounts"`
	BattleHookCooldownUntilTurns json.RawMessage   `json:"battleHookCooldownUntilTurns"`
	CompletedTurnCounts          json.RawMessage   `json:"completedTurnCounts"`
	BattlefieldEffects           json.RawMessage   `json:"battlefieldEffects"`
}

func loadRequiredBattlePreviousWaveCarry(load func() (store.BattlePreviousWaveCarry, error), expectedWave int) (battlePreviousWaveCarryState, error) {
	if load == nil {
		return battlePreviousWaveCarryState{}, fmt.Errorf("%w: previous wave loader is required", errBattlePreviousWaveCarryInvalid)
	}
	previous, err := load()
	if err != nil {
		return battlePreviousWaveCarryState{}, fmt.Errorf("load previous wave carry: %w", err)
	}
	return decodeRequiredBattlePreviousWaveCarry(previous, expectedWave)
}

func (s *Server) loadBattlePreviousWaveCarryState(ctx context.Context, roomID, currentRunID int64, expectedWave int) (battlePreviousWaveCarryState, error) {
	return loadRequiredBattlePreviousWaveCarry(func() (store.BattlePreviousWaveCarry, error) {
		return s.deps.Store.GetBattlePreviousWaveCarry(ctx, roomID, currentRunID, expectedWave)
	}, expectedWave)
}

func decodeRequiredBattlePreviousWaveCarry(previous store.BattlePreviousWaveCarry, expectedWave int) (battlePreviousWaveCarryState, error) {
	if expectedWave <= 0 || previous.Wave != expectedWave {
		return battlePreviousWaveCarryState{}, fmt.Errorf("%w: expected wave %d, got %d", errBattlePreviousWaveCarryInvalid, expectedWave, previous.Wave)
	}
	if status := strings.TrimSpace(previous.Status); status != store.BattleRunStatusEnded {
		return battlePreviousWaveCarryState{}, fmt.Errorf("%w: previous run status %q is not ended", errBattlePreviousWaveCarryInvalid, status)
	}
	var carry battlePreviousWaveCarryPayload
	if err := json.Unmarshal(previous.CarryStateRaw, &carry); err != nil {
		return battlePreviousWaveCarryState{}, fmt.Errorf("%w: decode carryState: %v", errBattlePreviousWaveCarryInvalid, err)
	}
	if status := strings.TrimSpace(carry.Status); status != store.BattleRunStatusEnded {
		return battlePreviousWaveCarryState{}, fmt.Errorf("%w: previous wave status %q is not ended", errBattlePreviousWaveCarryInvalid, status)
	}
	if len(carry.Players) == 0 {
		return battlePreviousWaveCarryState{}, fmt.Errorf("%w: players must be a non-empty array", errBattlePreviousWaveCarryInvalid)
	}
	for index := range carry.Players {
		normalizeBattleUnitState(&carry.Players[index])
	}
	battleCounts, err := normalizeRequiredBattleCarryCounts(carry.BattleHookBattleCounts, "battleHookBattleCounts")
	if err != nil {
		return battlePreviousWaveCarryState{}, err
	}
	cooldownUntilTurns, err := normalizeRequiredBattleCarryCounts(carry.BattleHookCooldownUntilTurns, "battleHookCooldownUntilTurns")
	if err != nil {
		return battlePreviousWaveCarryState{}, err
	}
	turnCounts, err := normalizeRequiredBattleCarryCounts(carry.CompletedTurnCounts, "completedTurnCounts")
	if err != nil {
		return battlePreviousWaveCarryState{}, err
	}
	return battlePreviousWaveCarryState{
		Players:                      carry.Players,
		PreparationOrder:             decodeBattleCarryStringSliceRaw(carry.PreparationOrder),
		BattleHookBattleCounts:       battleCounts,
		BattleHookCooldownUntilTurns: cooldownUntilTurns,
		CompletedTurnCounts:          turnCounts,
		BattlefieldEffects:           decodeBattlefieldEffectStatesRaw(carry.BattlefieldEffects),
	}, nil
}

func normalizeRequiredBattleCarryCounts(raw json.RawMessage, key string) (map[string]int, error) {
	if len(raw) == 0 || strings.TrimSpace(string(raw)) == "null" {
		return map[string]int{}, nil
	}
	values := map[string]any{}
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil, fmt.Errorf("%w: decode %s: %v", errBattlePreviousWaveCarryInvalid, key, err)
	}
	out := make(map[string]int, len(values))
	for countKey, rawCount := range values {
		count := toFloat64(rawCount, math.NaN())
		if strings.TrimSpace(countKey) == "" || math.IsNaN(count) || math.IsInf(count, 0) || count < 0 || count != math.Trunc(count) {
			return nil, fmt.Errorf("%w: %s contains an invalid count", errBattlePreviousWaveCarryInvalid, key)
		}
		if count > 0 {
			out[countKey] = int(count)
		}
	}
	return out, nil
}

func decodeBattleCarryStringSliceRaw(raw json.RawMessage) []string {
	if len(raw) == 0 || strings.TrimSpace(string(raw)) == "null" {
		return []string{}
	}
	var values any
	if err := json.Unmarshal(raw, &values); err != nil {
		return []string{}
	}
	return decodeBattleStringSlice(values)
}

func decodeBattlefieldEffectStatesRaw(raw json.RawMessage) []battlefieldEffectState {
	if len(raw) == 0 || strings.TrimSpace(string(raw)) == "null" {
		return nil
	}
	out := []battlefieldEffectState{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil
	}
	for index := range out {
		out[index].ID = strings.TrimSpace(out[index].ID)
		out[index].Windows = cloneBattlefieldWindows(out[index].Windows)
	}
	return out
}

func decodeBattleStringSlice(raw any) []string {
	rows := compactStringSlice(raw)
	if len(rows) == 0 {
		return []string{}
	}
	return rows
}

func decodeBattlePlayersFromFinalResultNode(finalStateRaw any) []battleUnitState {
	finalState, ok := finalStateRaw.(map[string]any)
	if !ok || len(finalState) == 0 {
		return nil
	}
	playersRaw, ok := finalState["players"]
	if !ok || playersRaw == nil {
		return nil
	}
	buf, err := json.Marshal(playersRaw)
	if err != nil {
		return nil
	}
	out := []battleUnitState{}
	if err := json.Unmarshal(buf, &out); err != nil {
		return nil
	}
	for idx := range out {
		normalizeBattleUnitState(&out[idx])
	}
	return out
}

func battleFilterPreparationOrder(order []string, players []battleUnitState) []string {
	if len(order) == 0 || len(players) == 0 {
		return []string{}
	}
	alivePlayers := map[string]struct{}{}
	for _, player := range players {
		unitID := strings.TrimSpace(player.UnitID)
		if unitID == "" || !player.Alive || player.HP <= 0 {
			continue
		}
		if battleUnitIsSummon(&player) || player.TurnIneligible {
			continue
		}
		alivePlayers[unitID] = struct{}{}
	}
	if len(alivePlayers) == 0 {
		return []string{}
	}
	out := make([]string, 0, len(order))
	seen := map[string]struct{}{}
	for _, row := range order {
		unitID := strings.TrimSpace(row)
		if unitID == "" {
			continue
		}
		if _, ok := alivePlayers[unitID]; !ok {
			continue
		}
		if _, exists := seen[unitID]; exists {
			continue
		}
		seen[unitID] = struct{}{}
		out = append(out, unitID)
	}
	return out
}

func (state *battleRuntimeState) randInt(max int) int {
	if max <= 1 {
		return 0
	}
	return int(math.Floor(state.randFloat64() * float64(max)))
}

func (state *battleRuntimeState) randFloat64() float64 {
	seed := uint64(state.Seed)
	seed ^= (uint64(state.RNGIndex) + uint64(0x9e3779b97f4a7c15)) + (seed << 6) + (seed >> 2)
	seed ^= seed >> 33
	seed *= 0xff51afd7ed558ccd
	seed ^= seed >> 33
	seed *= 0xc4ceb9fe1a85ec53
	seed ^= seed >> 33
	state.RNGIndex++
	return float64(seed%1_000_000) / 1_000_000
}

func (state *battleRuntimeState) rebuildOrderIfNeeded() {
	if len(state.Order) == 0 || !state.orderMatchesAliveUnits() {
		state.reseedOrder()
	}
}

func (state *battleRuntimeState) markUnitActed(unitID string) {
	if state == nil {
		return
	}
	normalizedID := strings.TrimSpace(unitID)
	if normalizedID == "" {
		return
	}
	for _, row := range state.ActedUnits {
		if strings.TrimSpace(row) == normalizedID {
			return
		}
	}
	state.ActedUnits = append(state.ActedUnits, normalizedID)
}

func (state battleRuntimeState) actedUnitSet() map[string]struct{} {
	out := map[string]struct{}{}
	for _, row := range state.ActedUnits {
		unitID := strings.TrimSpace(row)
		if unitID == "" {
			continue
		}
		out[unitID] = struct{}{}
	}
	return out
}

func battleNextPreparationOrderFromState(state battleRuntimeState) []string {
	playerAlive := map[string]struct{}{}
	for _, player := range state.Players {
		unitID := strings.TrimSpace(player.UnitID)
		if unitID == "" || !player.Alive || player.HP <= 0 {
			continue
		}
		if battleUnitIsSummon(&player) || player.TurnIneligible {
			continue
		}
		playerAlive[unitID] = struct{}{}
	}
	if len(playerAlive) == 0 {
		return []string{}
	}
	acted := state.actedUnitSet()
	if preparation := battleRemainingPreparationOrder(state.PreparationSeedOrder, playerAlive, acted); len(preparation) > 0 {
		return preparation
	}
	return battleNextPreparationOrderFromNormalTurns(state, playerAlive, acted)
}

func battleRemainingPreparationOrder(seed []string, playerAlive map[string]struct{}, acted map[string]struct{}) []string {
	if len(seed) == 0 || len(playerAlive) == 0 {
		return []string{}
	}
	out := make([]string, 0, len(seed))
	seen := map[string]struct{}{}
	for _, row := range seed {
		unitID := strings.TrimSpace(row)
		if unitID == "" {
			continue
		}
		if _, exists := seen[unitID]; exists {
			continue
		}
		if _, ok := playerAlive[unitID]; !ok {
			continue
		}
		if _, hasActed := acted[unitID]; hasActed {
			continue
		}
		seen[unitID] = struct{}{}
		out = append(out, unitID)
	}
	return out
}

func battleNextPreparationOrderFromNormalTurns(state battleRuntimeState, playerAlive map[string]struct{}, acted map[string]struct{}) []string {
	if len(state.Order) == 0 {
		return []string{}
	}
	start := state.TurnCursor
	if start < 0 {
		start = 0
	}
	if len(state.Order) > 0 {
		start %= len(state.Order)
	}
	out := make([]string, 0, len(state.Order))
	seen := map[string]struct{}{}
	for i := 0; i < len(state.Order); i++ {
		unitID := strings.TrimSpace(state.Order[(start+i)%len(state.Order)])
		if unitID == "" {
			continue
		}
		if _, exists := seen[unitID]; exists {
			continue
		}
		if _, ok := playerAlive[unitID]; !ok {
			continue
		}
		if _, hasActed := acted[unitID]; hasActed {
			continue
		}
		seen[unitID] = struct{}{}
		out = append(out, unitID)
	}
	return out
}

const (
	battlePreparationTurnOrderIndex = -1
	battleExtraTurnOrderIndex       = -2
)

func (state *battleRuntimeState) popNextExtraTurnActor() (*battleUnitState, string, bool) {
	if state == nil {
		return nil, "", false
	}
	state.activeExtraTurnRequest = nil
	for len(state.ExtraTurnQueue) > 0 {
		request := state.ExtraTurnQueue[0]
		state.ExtraTurnQueue = state.ExtraTurnQueue[1:]
		actor := battleUnitByID(state, strings.TrimSpace(request.ActorID))
		if actor == nil || !actor.Alive || actor.HP <= 0 || actor.TurnIneligible {
			continue
		}
		side := battleUnitTeamSide(actor)
		if side == "" {
			side = normalizeBattleSide(actor.Side)
		}
		if side == "" {
			continue
		}
		request.ActorID = strings.TrimSpace(actor.UnitID)
		state.activeExtraTurnRequest = &request
		return actor, side, true
	}
	return nil, "", false
}

func (state *battleRuntimeState) popNextPreparationActor() (*battleUnitState, string, bool) {
	if state == nil {
		return nil, "", false
	}
	for len(state.PreparationOrder) > 0 {
		unitID := strings.TrimSpace(state.PreparationOrder[0])
		state.PreparationOrder = state.PreparationOrder[1:]
		if unitID == "" {
			continue
		}
		for index := range state.Players {
			if state.Players[index].UnitID == unitID && state.Players[index].Alive && state.Players[index].HP > 0 && !state.Players[index].TurnIneligible {
				return &state.Players[index], "player", true
			}
		}
	}
	return nil, "", false
}

func (state *battleRuntimeState) reseedOrderIfEmpty() {
	if len(state.Order) > 0 {
		return
	}
	state.reseedOrder()
}

func (state *battleRuntimeState) reseedOrder() {
	units := make([]battleUnitState, 0, len(state.Players)+len(state.Enemies))
	for _, unit := range state.Players {
		if unit.Alive && unit.HP > 0 {
			units = append(units, unit)
		}
	}
	for _, unit := range state.Enemies {
		if unit.Alive && unit.HP > 0 {
			units = append(units, unit)
		}
	}
	sort.SliceStable(units, func(i, j int) bool {
		ai := units[i].Stats["speed"]
		aj := units[j].Stats["speed"]
		if ai == aj {
			return units[i].UnitID < units[j].UnitID
		}
		return ai > aj
	})
	state.FullOrder = make([]string, 0, len(units))
	state.Order = make([]string, 0, len(units))
	for _, unit := range units {
		if !unit.TurnIneligible {
			state.FullOrder = append(state.FullOrder, unit.UnitID)
		}
		if !unit.SkipTurnOrder && !unit.TurnIneligible {
			state.Order = append(state.Order, unit.UnitID)
		}
	}
	state.TurnCursor = 0
}

func (state *battleRuntimeState) orderMatchesAliveUnits() bool {
	// Avoid map allocation: linear scan is faster for small N (typical battle: 2-8 units).
	aliveCount := 0
	for i := range state.Players {
		u := &state.Players[i]
		if u.Alive && u.HP > 0 && strings.TrimSpace(u.UnitID) != "" && !u.SkipTurnOrder && !u.TurnIneligible {
			aliveCount++
		}
	}
	for i := range state.Enemies {
		u := &state.Enemies[i]
		if u.Alive && u.HP > 0 && strings.TrimSpace(u.UnitID) != "" && !u.SkipTurnOrder && !u.TurnIneligible {
			aliveCount++
		}
	}
	if aliveCount == 0 {
		return len(state.Order) == 0
	}
	matchCount := 0
	for _, orderID := range state.Order {
		orderID = strings.TrimSpace(orderID)
		if orderID == "" {
			continue
		}
		for i := range state.Players {
			u := &state.Players[i]
			if u.Alive && u.HP > 0 && strings.TrimSpace(u.UnitID) == orderID {
				matchCount++
				goto nextOrder
			}
		}
		for i := range state.Enemies {
			u := &state.Enemies[i]
			if u.Alive && u.HP > 0 && strings.TrimSpace(u.UnitID) == orderID {
				matchCount++
				goto nextOrder
			}
		}
	nextOrder:
	}
	return matchCount == aliveCount
}

func (s *Server) rebuildBattleOrder(state *battleRuntimeState) {
	if state == nil {
		return
	}
	if s != nil {
		for index := range state.Players {
			s.battleRecomputeUnitStats(&state.Players[index])
		}
		for index := range state.Enemies {
			s.battleRecomputeUnitStats(&state.Enemies[index])
		}
	}
	state.PreparationOrder = battleFilterPreparationOrder(state.PreparationOrder, state.Players)
	state.PreparationSeedOrder = battleFilterPreparationOrder(state.PreparationSeedOrder, state.Players)
	state.Order = []string{}
	state.rebuildOrderIfNeeded()
}

func (state *battleRuntimeState) removeUnitFromActionQueues(unitID string) {
	if state == nil {
		return
	}
	unitID = strings.TrimSpace(unitID)
	if unitID == "" {
		return
	}
	filter := func(rows []string) []string {
		out := rows[:0]
		for _, row := range rows {
			if strings.TrimSpace(row) != unitID {
				out = append(out, row)
			}
		}
		return out
	}
	state.PreparationOrder = filter(state.PreparationOrder)
	state.PreparationSeedOrder = filter(state.PreparationSeedOrder)
	state.FullOrder = filter(state.FullOrder)
	state.Order = filter(state.Order)
	if len(state.ExtraTurnQueue) > 0 {
		queued := state.ExtraTurnQueue[:0]
		for _, request := range state.ExtraTurnQueue {
			if strings.TrimSpace(request.ActorID) != unitID {
				queued = append(queued, request)
			}
		}
		state.ExtraTurnQueue = queued
	}
	if len(state.Order) == 0 {
		state.TurnCursor = 0
	} else {
		state.TurnCursor %= len(state.Order)
	}
}

func battleTeamAlive(state *battleRuntimeState, teamSide string) bool {
	if state == nil {
		return false
	}
	teamSide = normalizeBattleSide(teamSide)
	if teamSide == "" {
		return false
	}
	for i := range state.Players {
		u := &state.Players[i]
		if u.Alive && u.HP > 0 && battleUnitTeamSide(u) == teamSide {
			return true
		}
	}
	for i := range state.Enemies {
		u := &state.Enemies[i]
		if u.Alive && u.HP > 0 && battleUnitTeamSide(u) == teamSide {
			return true
		}
	}
	return false
}

func battlePlayerMembersAlive(state *battleRuntimeState) bool {
	if state == nil {
		return false
	}
	for i := range state.Players {
		u := &state.Players[i]
		if u.UserID > 0 && u.Alive && u.HP > 0 && battleUnitTeamSide(u) == "player" {
			return true
		}
	}
	for i := range state.Enemies {
		u := &state.Enemies[i]
		if u.UserID > 0 && u.Alive && u.HP > 0 && battleUnitTeamSide(u) == "player" {
			return true
		}
	}
	return false
}

func battleNextActor(state *battleRuntimeState) (*battleUnitState, string, int, bool) {
	if extraActor, extraSide, ok := state.popNextExtraTurnActor(); ok {
		return extraActor, extraSide, battleExtraTurnOrderIndex, false
	}
	if prepActor, prepSide, ok := state.popNextPreparationActor(); ok {
		return prepActor, prepSide, battlePreparationTurnOrderIndex, true
	}
	state.rebuildOrderIfNeeded()
	if len(state.Order) == 0 {
		return nil, "", 0, false
	}
	for i := 0; i < len(state.Order); i++ {
		index := (state.TurnCursor + i) % len(state.Order)
		id := state.Order[index]
		for p := range state.Players {
			if state.Players[p].UnitID == id && state.Players[p].Alive && state.Players[p].HP > 0 && !state.Players[p].TurnIneligible {
				state.TurnCursor = index
				return &state.Players[p], "player", index, false
			}
		}
		for e := range state.Enemies {
			if state.Enemies[e].UnitID == id && state.Enemies[e].Alive && state.Enemies[e].HP > 0 && !state.Enemies[e].TurnIneligible {
				state.TurnCursor = index
				return &state.Enemies[e], "enemy", index, false
			}
		}
	}
	return nil, "", 0, false
}

func (state *battleRuntimeState) advanceTurnCursor(currentIndex int) {
	if len(state.Order) == 0 {
		state.TurnCursor = 0
		return
	}
	state.TurnCursor = (currentIndex + 1) % len(state.Order)
}

func applyTurnRegen(unit *battleUnitState, server *Server) {
	if unit == nil {
		return
	}
	if !unit.Alive || unit.HP <= 0 {
		unit.HP = 0
		unit.Alive = false
		return
	}
	hpRegen := maxFloat(0, unit.Stats["hp_regen"])
	if server != nil && hpRegen > 0 {
		hpRegen, _ = server.battleApplyOutgoingBuffModifiers(unit, "heal", hpRegen)
	}
	mpRegen := maxFloat(0, unit.Stats["mp_regen"])
	unit.HP = clampFloat(unit.HP+hpRegen, 0, unit.MaxHP)
	unit.MP = clampFloat(unit.MP+mpRegen, 0, unit.MaxMP)
}

func buildTurnRegenActionLog(unit *battleUnitState, beforeHP, beforeMP float64) []map[string]any {
	if unit == nil {
		return nil
	}
	hpDelta := roundTo4(unit.HP - beforeHP)
	mpDelta := roundTo4(unit.MP - beforeMP)
	if hpDelta <= 0 && mpDelta <= 0 {
		return nil
	}
	actions := make([]map[string]any, 0, 2)
	if hpDelta > 0 {
		actions = append(actions, map[string]any{
			"kind":        "heal",
			"targetId":    unit.UnitID,
			"sourceId":    unit.UnitID,
			"amount":      hpDelta,
			"reason":      "turn_regen",
			"beforeHp":    roundTo4(beforeHP),
			"afterHp":     roundTo4(unit.HP),
			"beforeAlive": beforeHP > 0,
			"afterAlive":  unit.Alive,
			"delayMs":     int64(0),
		})
	}
	if mpDelta > 0 {
		actions = append(actions, map[string]any{
			"kind":     "resource",
			"targetId": unit.UnitID,
			"sourceId": unit.UnitID,
			"resource": "mp",
			"amount":   mpDelta,
			"reason":   "turn_regen",
			"beforeMp": roundTo4(beforeMP),
			"afterMp":  roundTo4(unit.MP),
			"delayMs":  int64(0),
		})
	}
	return actions
}

func buildBuffDetachActionLog(unitID string, buffIDs []string, reason string) []map[string]any {
	if len(buffIDs) == 0 {
		return nil
	}
	actions := make([]map[string]any, 0, len(buffIDs))
	for _, rawID := range buffIDs {
		buffID := strings.TrimSpace(rawID)
		if buffID == "" {
			continue
		}
		actions = append(actions, map[string]any{
			"kind":     "state_detach",
			"targetId": unitID,
			"sourceId": unitID,
			"buffId":   buffID,
			"reason":   reason,
			"delayMs":  int64(0),
		})
	}
	return actions
}

func populateTargetSlots(targets []map[string]any, state *battleRuntimeState, actorSide string) {
	if len(targets) == 0 || state == nil {
		return
	}
	targetPool := state.Enemies
	if strings.EqualFold(strings.TrimSpace(actorSide), "enemy") {
		targetPool = state.Players
	}
	indexMap := make(map[string]int, len(targetPool))
	for idx, unit := range targetPool {
		indexMap[strings.TrimSpace(unit.UnitID)] = idx
	}
	for _, target := range targets {
		targetID, _ := target["targetId"].(string)
		targetID = strings.TrimSpace(targetID)
		if targetID == "" {
			continue
		}
		if slot, ok := indexMap[targetID]; ok {
			target["slot"] = slot
		}
	}
}

func decrementBattleSkillCooldowns(unit *battleUnitState) {
	if unit == nil {
		return
	}
	for idx := range unit.Skills {
		if unit.Skills[idx].CooldownRemain > 0 {
			unit.Skills[idx].CooldownRemain--
		}
	}
}

func battleDurationTurnsFromLegacyMS(durationMS int64, turnIntervalMS int64) int64 {
	if durationMS < 0 {
		return -1
	}
	if durationMS == 0 {
		return 0
	}
	if turnIntervalMS <= 0 {
		turnIntervalMS = 1000
	}
	turns := durationMS / turnIntervalMS
	if durationMS%turnIntervalMS != 0 {
		turns += 1
	}
	if turns <= 0 {
		return 1
	}
	return turns
}

func tickBattleBuffDurations(unit *battleUnitState) (bool, []string) {
	return tickBattleBuffDurationsForTurn(unit, 0)
}

func tickBattleBuffDurationsForTurn(unit *battleUnitState, turnSerial int64) (bool, []string) {
	if unit == nil || len(unit.Buffs) == 0 {
		return false, nil
	}
	next := make([]battleBuffState, 0, len(unit.Buffs))
	changed := false
	var expired []string
	for _, buff := range unit.Buffs {
		if unit.SkipTurnOrder && strings.EqualFold(strings.TrimSpace(buff.BuffID), battleSummonBindingBuffID) {
			buff.AppliedBuffTurnSerial = 0
			next = append(next, buff)
			continue
		}
		if turnSerial > 0 && buff.AppliedBuffTurnSerial == turnSerial {
			buff.AppliedBuffTurnSerial = 0
			next = append(next, buff)
			continue
		}
		buff.AppliedBuffTurnSerial = 0
		if buff.RemainTurns == 0 {
			changed = true
			if id := strings.TrimSpace(buff.BuffID); id != "" {
				expired = append(expired, id)
			}
			continue
		}
		if buff.RemainTurns > 0 {
			buff.RemainTurns -= 1
			changed = true
			if buff.RemainTurns <= 0 {
				if id := strings.TrimSpace(buff.BuffID); id != "" {
					expired = append(expired, id)
				}
				continue
			}
		}
		next = append(next, buff)
	}
	if len(next) != len(unit.Buffs) {
		changed = true
	}
	unit.Buffs = next
	return changed, expired
}

func battleDespawnSummonedUnit(unit *battleUnitState) (bool, float64) {
	if unit == nil || !unit.Alive || unit.HP <= 0 {
		return false, 0
	}
	beforeHP := unit.HP
	unit.HP = 0
	unit.MP = 0
	unit.Alive = false
	unit.SummonRemainTurns = 0
	unit.Buffs = battleRemoveBuffByID(unit.Buffs, battleSummonBindingBuffID)
	return true, beforeHP
}

func battleRemoveBuffByID(buffs []battleBuffState, buffID string) []battleBuffState {
	normalized := strings.ToLower(strings.TrimSpace(buffID))
	if normalized == "" || len(buffs) == 0 {
		return buffs
	}
	next := make([]battleBuffState, 0, len(buffs))
	for _, buff := range buffs {
		if strings.ToLower(strings.TrimSpace(buff.BuffID)) == normalized {
			continue
		}
		next = append(next, buff)
	}
	return next
}

func battleCleanupSummonBindingForDeadUnits(state *battleRuntimeState) {
	if state == nil {
		return
	}
	cleanup := func(unit *battleUnitState) {
		if unit == nil || unit.Alive || unit.HP > 0 {
			return
		}
		if strings.TrimSpace(unit.SummonTemplateUnitID) == "" || strings.TrimSpace(unit.SummonSourceUnitID) == "" {
			return
		}
		unit.Buffs = battleRemoveBuffByID(unit.Buffs, battleSummonBindingBuffID)
		unit.SummonRemainTurns = 0
	}
	for i := range state.Players {
		cleanup(&state.Players[i])
	}
	for i := range state.Enemies {
		cleanup(&state.Enemies[i])
	}
}

func battleApplySummonCasterDeathPolicy(state *battleRuntimeState) []map[string]any {
	if state == nil {
		return nil
	}
	casterAlive := map[string]struct{}{}
	for i := range state.Players {
		unit := &state.Players[i]
		if unit != nil && unit.Alive && unit.HP > 0 {
			casterAlive[strings.TrimSpace(unit.UnitID)] = struct{}{}
		}
	}
	for i := range state.Enemies {
		unit := &state.Enemies[i]
		if unit != nil && unit.Alive && unit.HP > 0 {
			casterAlive[strings.TrimSpace(unit.UnitID)] = struct{}{}
		}
	}
	actions := make([]map[string]any, 0, 2)
	apply := func(unit *battleUnitState) {
		if unit == nil || !unit.Alive || unit.HP <= 0 {
			return
		}
		if strings.TrimSpace(unit.SummonTemplateUnitID) == "" || strings.TrimSpace(unit.SummonSourceUnitID) == "" {
			return
		}
		if normalizeBattleSummonOnCasterDeath(unit.SummonOnCasterDeath) != battleSummonOnCasterDeathDespawn {
			return
		}
		if _, alive := casterAlive[strings.TrimSpace(unit.SummonSourceUnitID)]; alive {
			return
		}
		if despawned, beforeHP := battleDespawnSummonedUnit(unit); despawned {
			actions = append(actions, map[string]any{
				"kind":       "summon_expire",
				"targetId":   unit.UnitID,
				"sourceId":   unit.SummonSourceUnitID,
				"reason":     "summon_caster_dead",
				"beforeHp":   roundTo4(beforeHP),
				"afterHp":    roundTo4(unit.HP),
				"afterMp":    roundTo4(unit.MP),
				"afterAlive": unit.Alive,
				"delayMs":    int64(0),
			})
		}
	}
	for i := range state.Players {
		apply(&state.Players[i])
	}
	for i := range state.Enemies {
		apply(&state.Enemies[i])
	}
	return actions
}

func battleSummonBindingOnDispel(unit *battleUnitState) bool {
	if unit == nil {
		return false
	}
	if strings.TrimSpace(unit.SummonTemplateUnitID) == "" || strings.TrimSpace(unit.SummonSourceUnitID) == "" {
		return false
	}
	return normalizeBattleSummonOnDispel(unit.SummonOnDispel) == battleSummonOnDispelDespawn
}

func (s *Server) battleTickBattleStartBuffs(players []battleUnitState) []battleUnitState {
	if s == nil || len(players) == 0 {
		return players
	}
	s.battleParseBuffRules() // ensure cache is built
	result := make([]battleUnitState, 0, len(players))
	for i := range players {
		unit := players[i]
		expiredBuffIDs := s.tickUnitBattleStartBuffs(&unit)
		shouldRemove, _ := s.battleApplyBuffExpiry(&unit, expiredBuffIDs)
		if shouldRemove {
			continue // drop — no corpse, no DB record
		}
		result = append(result, unit)
	}
	return result
}

func (s *Server) tickUnitBattleStartBuffs(unit *battleUnitState) []string {
	if unit == nil || len(unit.Buffs) == 0 {
		return nil
	}
	var expired []string
	next := make([]battleBuffState, 0, len(unit.Buffs))
	for _, buff := range unit.Buffs {
		buffID := strings.ToLower(strings.TrimSpace(buff.BuffID))
		if !s.buffTickOnBattleStartCache[buffID] {
			next = append(next, buff)
			continue
		}
		if buff.RemainTurns <= 0 {
			// Permanent (< 0) — keep unchanged; already expired (0) — keep to not re-trigger
			next = append(next, buff)
			continue
		}
		buff.RemainTurns -= 1
		if buff.RemainTurns <= 0 {
			expired = append(expired, buff.BuffID)
			continue // remove from buffs
		}
		next = append(next, buff)
	}
	unit.Buffs = next
	return expired
}

func battleApplyOrRefreshBuff(unit *battleUnitState, buffID string, remainTurns int64, payload map[string]any, sourceID string, stackPolicy string, maxStacksOpt ...int) bool {
	return battleApplyOrRefreshBuffWithTurnSerial(unit, buffID, remainTurns, payload, sourceID, stackPolicy, 0, maxStacksOpt...)
}

func battleApplyOrRefreshBuffForTurn(unit *battleUnitState, buffID string, remainTurns int64, payload map[string]any, sourceID string, stackPolicy string, maxStacks int, turnSerial int64) bool {
	return battleApplyOrRefreshBuffWithTurnSerial(unit, buffID, remainTurns, payload, sourceID, stackPolicy, turnSerial, maxStacks)
}

func battleApplyOrRefreshBuffWithTurnSerial(unit *battleUnitState, buffID string, remainTurns int64, payload map[string]any, sourceID string, stackPolicy string, turnSerial int64, maxStacksOpt ...int) bool {
	if unit == nil {
		return false
	}
	normalizedBuffID := strings.TrimSpace(buffID)
	if normalizedBuffID == "" {
		return false
	}
	maxStacks := 0
	if len(maxStacksOpt) > 0 {
		maxStacks = maxInt(0, maxStacksOpt[0])
	}
	normalizedStackPolicy := battleNormalizeBuffStackPolicy(stackPolicy)
	if normalizedStackPolicy != "stack" {
		for idx := range unit.Buffs {
			if strings.TrimSpace(unit.Buffs[idx].BuffID) != normalizedBuffID {
				continue
			}
			unit.Buffs[idx].BuffID = normalizedBuffID
			unit.Buffs[idx].RemainTurns = remainTurns
			unit.Buffs[idx].LegacyRemainMS = 0
			unit.Buffs[idx].Payload = cloneAnyMap(payload)
			unit.Buffs[idx].SourceID = strings.TrimSpace(sourceID)
			unit.Buffs[idx].AppliedBuffTurnSerial = maxInt64(0, turnSerial)
			return false
		}
	} else if maxStacks > 0 {
		firstMatchingIndex := -1
		stackCount := 0
		for idx := range unit.Buffs {
			if strings.TrimSpace(unit.Buffs[idx].BuffID) != normalizedBuffID {
				continue
			}
			if firstMatchingIndex < 0 {
				firstMatchingIndex = idx
			}
			stackCount++
		}
		if stackCount >= maxStacks {
			if firstMatchingIndex >= 0 {
				unit.Buffs[firstMatchingIndex].RemainTurns = remainTurns
				unit.Buffs[firstMatchingIndex].LegacyRemainMS = 0
				unit.Buffs[firstMatchingIndex].Payload = cloneAnyMap(payload)
				unit.Buffs[firstMatchingIndex].SourceID = strings.TrimSpace(sourceID)
				unit.Buffs[firstMatchingIndex].AppliedBuffTurnSerial = maxInt64(0, turnSerial)
			}
			return false
		}
	}
	unit.Buffs = append(unit.Buffs, battleBuffState{
		BuffID:                normalizedBuffID,
		RemainTurns:           remainTurns,
		Payload:               cloneAnyMap(payload),
		SourceID:              strings.TrimSpace(sourceID),
		AppliedBuffTurnSerial: maxInt64(0, turnSerial),
	})
	return true
}

func battleBuffApplicationTurnSerial(state *battleRuntimeState, trigger string) int64 {
	if state == nil || battleNormalizeTrigger(trigger) == "onturnstart" {
		return 0
	}
	return maxInt64(0, state.activeBuffDurationTurnSerial)
}

func (s *Server) battleSelectSkill(state *battleRuntimeState, unit battleUnitState, actorSide string) (battleSkillState, int) {
	return s.battleSelectSkillFiltered(state, unit, actorSide, true, true)
}

func (s *Server) battleSkillRulesetEnabled(skill battleSkillState) bool {
	if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(skill.ID)), "custom:") {
		return true
	}
	if s == nil {
		return false
	}
	version := skill.RulesetVersion
	if version <= 0 {
		// Battle snapshots created before rulesetVersion existed belong to V1.
		version = 1
	}
	return s.deps.Config.GlobalSettings.SkillForge.RulesetEnabled(version)
}

func (s *Server) battleSelectSkillFiltered(state *battleRuntimeState, unit battleUnitState, actorSide string, allowBasic bool, allowNonBasic bool) (battleSkillState, int) {
	if !unit.Alive {
		return battleSkillState{}, -1
	}
	basicSkillID := s.battleDefaultBasicSkillID()
	basicSkillPresent := false
	// Lazy eval context: built at most once per call, shared across all skill rule checks.
	// Safe because cooldown is checked before any rule evaluation, so CooldownReady is always 1.
	cachedCtxNeedsUnitContext := battleRuleStatesNeedUnitContext(unit.SkillRules)
	var cachedCtx *rules.EvalContext
	for idx, skill := range unit.Skills {
		if skill.ComboOnly {
			continue
		}
		if !s.battleSkillRulesetEnabled(skill) {
			continue
		}
		isBasicSkill := basicSkillID != "" && strings.EqualFold(strings.TrimSpace(skill.ID), basicSkillID)
		mutationScope := "skill"
		if isBasicSkill {
			mutationScope = "basic_attack"
		}
		skill = (&unit).applyBattleSkillScalarMutations(mutationScope, skill)
		if isBasicSkill {
			basicSkillPresent = true
		}
		if (isBasicSkill && !allowBasic) || (!isBasicSkill && !allowNonBasic) {
			continue
		}
		if skill.CooldownRemain > 0 {
			continue
		}
		if skill.Combo != nil && !s.battleComboReady(state, &unit, skill.Combo) {
			continue
		}
		if unit.MP < skill.MPCost && !s.battleCanPaySpecializationResourceDebt(state, &unit, skill) {
			continue
		}
		if unit.HP <= skill.HPCost {
			continue
		}
		if !s.battleSkillRulePassedCached(state, unit, actorSide, skill, cachedCtxNeedsUnitContext, &cachedCtx) {
			continue
		}
		if !s.battleSkillCastGuardsPassed(state, &unit, skill) {
			continue
		}
		return skill, idx
	}
	// The basic skill has already been evaluated from unit.Skills. If it failed
	// (rule/cooldown/resource), do not bypass those checks via unconditional fallback.
	if basicSkillPresent {
		return battleSkillState{}, -1
	}
	if !allowBasic {
		return battleSkillState{}, -1
	}
	fallback := s.battleSkillByID(basicSkillID)
	fallback = (&unit).applyBattleSkillScalarMutations("basic_attack", fallback)
	return fallback, -1
}

func (s *Server) battleCanPaySpecializationResourceDebt(state *battleRuntimeState, unit *battleUnitState, skill battleSkillState) bool {
	if s == nil || unit == nil || strings.TrimSpace(skill.ID) == "" || unit.MP >= skill.MPCost {
		return false
	}
	context := &battleEquipmentHookContext{
		SkillID:        skill.ID,
		SkillCategory:  skill.Category,
		CastKind:       "skill",
		IsBasicAttack:  strings.EqualFold(strings.TrimSpace(skill.ID), strings.TrimSpace(s.battleDefaultBasicSkillID())),
		MPCost:         skill.MPCost,
		HPCost:         skill.HPCost,
		MPInsufficient: true,
	}
	for _, hook := range unit.SpecializationHooks {
		if battleNormalizeTrigger(hook.Trigger) != "onbeforeskillcast" || validateBattleSpecializationHookRule(hook) != nil {
			continue
		}
		if !battleEquipmentHookSubjectAllowed(hook, unit) || !battleEquipmentHookEventAllowed(hook, unit, nil, context) {
			continue
		}
		if !battleEquipmentHookLimitAllowed(state, unit, hook) {
			continue
		}
		conditionsPassed := true
		for _, condition := range hook.Conditions {
			if !s.battleEvaluateEquipmentHookCondition(state, condition, unit, nil, nil, 0, "", hook.SourceEnhanceLevel, context) {
				conditionsPassed = false
				break
			}
		}
		if !conditionsPassed {
			continue
		}
		for _, effect := range hook.Effects {
			if battleNormalizeTrigger(fmt.Sprintf("%v", firstNonNil(effect["kind"], effect["type"]))) != "resourcedebtcast" {
				continue
			}
			minimumHP := maxFloat(0, toFloat64(firstNonNil(effect["minimumHp"], effect["minimum_hp"]), 1))
			cost := math.Floor(unit.MaxHP * clampFloat(battleSpecializationEffectScalar(unit, effect, "hpCostRatio", "hp_cost_ratio"), 0, 1))
			return cost > 0 && unit.HP-cost-skill.HPCost >= minimumHP
		}
	}
	return false
}

func (s *Server) battleSkillRulePassed(state *battleRuntimeState, actor battleUnitState, actorSide string, skill battleSkillState) bool {
	key := strings.ToLower(strings.TrimSpace(skill.ID))
	if key == "" {
		return true
	}
	ruleState, ok := actor.SkillRules[key]
	if !ok || len(ruleState.Groups) == 0 {
		return true
	}
	ctx := s.buildBattleRuleEvalContext(state, actor, actorSide, skill, battleRuleNeedsUnitContext(ruleState.Groups))
	return rules.Evaluate(ruleState.Groups, ctx)
}

func (s *Server) battleSkillRulePassedCached(state *battleRuntimeState, actor battleUnitState, actorSide string, skill battleSkillState, needsUnitContext bool, cachedCtx **rules.EvalContext) bool {
	key := strings.ToLower(strings.TrimSpace(skill.ID))
	if key == "" {
		return true
	}
	ruleState, ok := actor.SkillRules[key]
	if !ok || len(ruleState.Groups) == 0 {
		return true
	}
	if *cachedCtx == nil {
		ctx := s.buildBattleRuleEvalContext(state, actor, actorSide, skill, needsUnitContext)
		*cachedCtx = &ctx
	}
	return rules.Evaluate(ruleState.Groups, **cachedCtx)
}

func (s *Server) battleSkillCastGuardsPassed(state *battleRuntimeState, actor *battleUnitState, skill battleSkillState) bool {
	if s == nil || state == nil || actor == nil || len(skill.CastGuards) == 0 {
		return true
	}
	actorID := strings.TrimSpace(actor.UnitID)
	hasEnemyEffect := func(units []battleUnitState, effectKind string) bool {
		for i := range units {
			unit := &units[i]
			if strings.TrimSpace(unit.UnitID) == actorID {
				continue
			}
			if !unit.Alive || unit.HP <= 0 {
				continue
			}
			if s.battleTargetRelationForActor(state, actor, unit) == "enemy" && s.battleUnitHasWhileActiveEffect(unit, effectKind, actor) {
				return true
			}
		}
		return false
	}
	hasAllySideNonFullHP := func(units []battleUnitState) bool {
		for i := range units {
			unit := &units[i]
			if !unit.Alive || unit.HP <= 0 {
				continue
			}
			if s.battleTargetRelationForActor(state, actor, unit) != "ally" {
				continue
			}
			if unit.HP < unit.MaxHP {
				return true
			}
		}
		return false
	}
	for _, guard := range skill.CastGuards {
		switch guard.Kind {
		case "enemy_side_not_has_effect":
			effectKind := battleNormalizeTrigger(guard.EffectKind)
			if effectKind == "" {
				continue
			}
			if hasEnemyEffect(state.Players, effectKind) || hasEnemyEffect(state.Enemies, effectKind) {
				return false
			}
		case "ally_side_has_non_full_hp":
			if !hasAllySideNonFullHP(state.Players) && !hasAllySideNonFullHP(state.Enemies) {
				return false
			}
		}
	}
	return true
}

func (s *Server) buildBattleRuleEvalContext(state *battleRuntimeState, actor battleUnitState, actorSide string, skill battleSkillState, needsUnitContext bool) rules.EvalContext {
	skillCooldowns := make(map[string]float64, len(actor.Skills))
	itemCooldowns := make(map[string]float64, len(actor.BattleItems))
	stateStacks := make(map[string]float64, len(actor.Buffs))
	states := make(map[string]float64, len(actor.Buffs))
	buffKinds := map[string]string{}
	if needsUnitContext {
		buffKinds = s.battleRuleBuffKindMap()
	}
	for _, row := range actor.Skills {
		key := strings.ToLower(strings.TrimSpace(row.ID))
		if key == "" {
			continue
		}
		skillCooldowns[key] = float64(maxInt(0, row.CooldownRemain))
	}
	for _, row := range actor.BattleItems {
		key := strings.ToLower(strings.TrimSpace(row.ItemID))
		if key == "" {
			continue
		}
		itemCooldowns[key] = float64(maxInt(0, row.CooldownRemain))
	}
	for _, buff := range actor.Buffs {
		buffID := strings.ToLower(strings.TrimSpace(buff.BuffID))
		if buffID == "" {
			continue
		}
		stateStacks[buffID] += 1
		states[buffID] = 1
	}
	unitAlive := make(map[string]float64)
	var selfUnit rules.UnitContext
	if needsUnitContext {
		selfUnit = s.battleRuleUnitContext(actor, buffKinds)
	}
	allies := []rules.UnitContext{}
	enemies := []rules.UnitContext{}
	allUnits := []rules.UnitContext{}
	selfSummonedUnits := []rules.UnitContext{}
	actorID := strings.TrimSpace(actor.UnitID)
	actorSide = normalizeBattleSide(actorSide)
	if actorSide == "" {
		actorSide = "player"
	}
	accumulateUnit := func(unit battleUnitState) {
		key := strings.ToLower(strings.TrimSpace(unit.UnitID))
		if key != "" {
			unitAlive[key] = battleReadyFlag(unit.Alive)
		}
		templateKey := strings.ToLower(strings.TrimSpace(unit.EnemyID))
		if templateKey != "" {
			unitAlive[templateKey] = maxFloat(unitAlive[templateKey], battleReadyFlag(unit.Alive))
		}
		summonTemplateKey := strings.ToLower(strings.TrimSpace(unit.SummonTemplateUnitID))
		if summonTemplateKey != "" {
			unitAlive[summonTemplateKey] = maxFloat(unitAlive[summonTemplateKey], battleReadyFlag(unit.Alive))
		}
		if !needsUnitContext {
			return
		}
		if !unit.Alive || unit.HP <= 0 {
			return
		}
		ctxUnit := s.battleRuleUnitContext(unit, buffKinds)
		allUnits = append(allUnits, ctxUnit)
		if battleUnitPerspectiveSide(&unit) == actorSide {
			allies = append(allies, ctxUnit)
		} else {
			enemies = append(enemies, ctxUnit)
		}
		if actorID != "" && strings.TrimSpace(unit.SummonSourceUnitID) == actorID {
			selfSummonedUnits = append(selfSummonedUnits, ctxUnit)
		}
	}
	if state != nil {
		unitAlive = make(map[string]float64, len(state.Players)+len(state.Enemies))
		for _, unit := range state.Players {
			accumulateUnit(unit)
		}
		for _, unit := range state.Enemies {
			accumulateUnit(unit)
		}
	}
	if needsUnitContext && len(allies) == 0 && actor.Alive && actor.HP > 0 {
		allies = append(allies, selfUnit)
		allUnits = append(allUnits, selfUnit)
	}
	enemyHasBoss := false
	if state != nil {
		for _, unit := range state.Enemies {
			if battleUnitIsBoss(&unit) {
				enemyHasBoss = true
				break
			}
		}
	}
	enemyCount := float64(battleAliveCountForSide(state, actorSide))
	metrics := map[string]float64{}
	if state != nil {
		metrics = buildBattleAliveCountMetrics(state, actorSide, enemyCount)
	}
	passedTurn := 1.0
	if state != nil {
		passedTurn += float64(maxInt(0, state.RuleTurnCounts[strings.TrimSpace(actor.UnitID)]))
	}
	ctx := rules.EvalContext{
		SelfHPPercent:  s.battleSelfHPPercent(actor),
		CooldownReady:  battleReadyFlag(skill.CooldownRemain <= 0),
		HasBuff:        battleReadyFlag(len(actor.Buffs) > 0),
		TargetTagMatch: 0,
		InventoryCount: 0,
		EnemyCount:     enemyCount,
		Distance:       1,
		TimeWindow: float64(func() int {
			if state == nil {
				return 0
			}
			return state.Wave
		}()),
		PassedTurn:     passedTurn,
		SkillCooldowns: skillCooldowns,
		ItemCooldowns:  itemCooldowns,
		StateStacks:    stateStacks,
		States:         states,
		UnitAlive:      unitAlive,
		SpecialFlags: map[string]float64{
			"enemyHasBoss":    battleReadyFlag(enemyHasBoss),
			"enemyNotHasBoss": battleReadyFlag(!enemyHasBoss),
		},
		Metrics:           metrics,
		Allies:            allies,
		Enemies:           enemies,
		AllUnits:          allUnits,
		SelfSummonedUnits: selfSummonedUnits,
	}
	if needsUnitContext {
		ctx.Self = &selfUnit
	}
	return ctx
}

func battleRuleNeedsUnitContext(groups []rules.ConditionGroup) bool {
	for _, group := range groups {
		for _, condition := range group.Conditions {
			if battleRuleConditionNeedsUnitContext(condition) {
				return true
			}
		}
	}
	return false
}

func battleRuleStatesNeedUnitContext(ruleStates map[string]battleSkillRuleState) bool {
	for _, ruleState := range ruleStates {
		if battleRuleNeedsUnitContext(ruleState.Groups) {
			return true
		}
	}
	return false
}

func battleRuleConditionNeedsUnitContext(condition rules.Condition) bool {
	switch strings.ToLower(strings.TrimSpace(condition.CompareType)) {
	case "value":
		dataType := condition.DataType
		if strings.TrimSpace(dataType) == "" {
			dataType = condition.Field
		}
		switch normalizeBattleRuleToken(dataType) {
		case "hp", "mp", "losthp", "lostmp", "totalhp", "count", "buffcount", "debuffcount":
			return true
		default:
			return false
		}
	}
	switch normalizeBattleRuleToken(condition.Target) {
	case "allies", "enemies", "eachallies", "eachenemies", "all", "selfsummonedunit":
		return true
	}
	return false
}

func normalizeBattleRuleToken(raw string) string {
	replacer := strings.NewReplacer("_", "", "-", "", " ", "")
	return replacer.Replace(strings.ToLower(strings.TrimSpace(raw)))
}

func (s *Server) battleRuleBuffKindMap() map[string]string {
	out := map[string]string{}
	if s == nil || s.deps.GameData.Buff == nil {
		return out
	}
	for _, raw := range s.deps.GameData.Buff {
		id := strings.ToLower(strings.TrimSpace(toString(raw["id"])))
		if id == "" {
			continue
		}
		kind := strings.ToLower(strings.TrimSpace(toString(raw["kind"])))
		if kind != "debuff" {
			kind = "buff"
		}
		out[id] = kind
	}
	return out
}

func (s *Server) battleRuleUnitContext(unit battleUnitState, buffKinds map[string]string) rules.UnitContext {
	skillCooldowns := make(map[string]float64, len(unit.Skills))
	for _, row := range unit.Skills {
		key := strings.ToLower(strings.TrimSpace(row.ID))
		if key != "" {
			skillCooldowns[key] = float64(maxInt(0, row.CooldownRemain))
		}
	}
	itemCooldowns := make(map[string]float64, len(unit.BattleItems))
	for _, row := range unit.BattleItems {
		key := strings.ToLower(strings.TrimSpace(row.ItemID))
		if key != "" {
			itemCooldowns[key] = float64(maxInt(0, row.CooldownRemain))
		}
	}
	stateStacks := make(map[string]float64, len(unit.Buffs))
	stateKinds := make(map[string]string, len(unit.Buffs))
	for _, buff := range unit.Buffs {
		key := strings.ToLower(strings.TrimSpace(buff.BuffID))
		if key == "" {
			continue
		}
		stateStacks[key] += 1
		if kind := strings.TrimSpace(buffKinds[key]); kind != "" {
			stateKinds[key] = kind
		}
	}
	templateID := strings.TrimSpace(unit.SummonTemplateUnitID)
	if templateID == "" {
		templateID = strings.TrimSpace(unit.EnemyID)
	}
	return rules.UnitContext{
		UnitID:             strings.TrimSpace(unit.UnitID),
		TemplateUnitID:     templateID,
		HP:                 unit.HP,
		MP:                 unit.MP,
		MaxHP:              unit.MaxHP,
		MaxMP:              unit.MaxMP,
		IsBoss:             battleUnitIsBoss(&unit),
		SkillCooldowns:     skillCooldowns,
		ItemCooldowns:      itemCooldowns,
		StateStacks:        stateStacks,
		StateKinds:         stateKinds,
		SummonSourceUnitID: strings.TrimSpace(unit.SummonSourceUnitID),
	}
}

func (s *Server) battleSelfHPPercent(actor battleUnitState) float64 {
	if actor.MaxHP <= 0 {
		return 0
	}
	return clampFloat((actor.HP/actor.MaxHP)*100, 0, 100)
}

func battleReadyFlag(ok bool) float64 {
	if ok {
		return 1
	}
	return 0
}

func battleAliveCountForSide(state *battleRuntimeState, actorSide string) int {
	if state == nil {
		return 0
	}
	actorSide = normalizeBattleSide(actorSide)
	if actorSide == "" {
		actorSide = "player"
	}
	alive := 0
	for i := range state.Players {
		unit := &state.Players[i]
		if !unit.Alive || unit.HP <= 0 {
			continue
		}
		if battleUnitPerspectiveSide(unit) == actorSide {
			continue
		}
		alive++
	}
	for i := range state.Enemies {
		unit := &state.Enemies[i]
		if !unit.Alive || unit.HP <= 0 {
			continue
		}
		if battleUnitPerspectiveSide(unit) == actorSide {
			continue
		}
		alive++
	}
	return alive
}

func normalizeBattleRuleTargetScope(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "allies", "all", "self", "none":
		return strings.ToLower(strings.TrimSpace(raw))
	default:
		return "enemies"
	}
}

// buildBattleAliveCountMetrics computes all 20 enemy_count.* metrics in a single pass,
// replacing 20 separate calls to battleAliveCountByRuleScope.
func buildBattleAliveCountMetrics(state *battleRuntimeState, actorSide string, enemyCount float64) map[string]float64 {
	// Counters indexed by [scopeIdx][posIdx]
	// scope: 0=enemies, 1=allies/self, 2=all, 3=none(always 0)
	// pos:   0=all, 1=vanguard, 2=midguard, 3=rearguard
	var counts [4][4]int
	actorSide = normalizeBattleSide(actorSide)
	if actorSide == "" {
		actorSide = "player"
	}
	accumulate := func(unit *battleUnitState) {
		if unit == nil || !unit.Alive || unit.HP <= 0 {
			return
		}
		isAlly := battleUnitPerspectiveSide(unit) == actorSide
		sAlly, sEnemy := 1, 0
		if !isAlly {
			sAlly, sEnemy = 0, 1
		}
		counts[0][0] += sEnemy // enemies.all
		counts[1][0] += sAlly  // allies.all
		counts[2][0]++         // all.all
		pos := strings.ToLower(strings.TrimSpace(unit.Position))
		var pIdx int
		switch pos {
		case "vanguard":
			pIdx = 1
		case "midguard":
			pIdx = 2
		case "rearguard":
			pIdx = 3
		default:
			return
		}
		counts[0][pIdx] += sEnemy
		counts[1][pIdx] += sAlly
		counts[2][pIdx]++
	}
	for i := range state.Players {
		accumulate(&state.Players[i])
	}
	for i := range state.Enemies {
		accumulate(&state.Enemies[i])
	}
	// scope indices: 0=enemies, 1=allies(=self), 2=all, 3=none
	scopes := [4]string{"enemies", "allies", "all", "none"}
	positions := [3]string{"vanguard", "midguard", "rearguard"}
	metrics := make(map[string]float64, 26)
	metrics["enemy_count"] = enemyCount
	for si := 0; si < 4; si++ {
		scope := scopes[si]
		total := float64(counts[si][0])
		metrics["enemy_count."+scope] = total
		metrics["enemy_count."+scope+".all"] = total
		for pi, pos := range positions {
			metrics["enemy_count."+scope+"."+pos] = float64(counts[si][pi+1])
		}
	}
	// "self" scope behaves identically to "allies" in battleAliveCountByRuleScope.
	selfTotal := float64(counts[1][0])
	metrics["enemy_count.self"] = selfTotal
	metrics["enemy_count.self.all"] = selfTotal
	for pi, pos := range positions {
		metrics["enemy_count.self."+pos] = float64(counts[1][pi+1])
	}
	return metrics
}

func normalizeBattleRulePosition(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "vanguard", "midguard", "rearguard":
		return strings.ToLower(strings.TrimSpace(raw))
	default:
		return "all"
	}
}

func battleAliveCountByRuleScope(state *battleRuntimeState, actorSide string, targetScope string, position string) int {
	if state == nil {
		return 0
	}
	targetScope = normalizeBattleRuleTargetScope(targetScope)
	position = normalizeBattleRulePosition(position)
	actorSide = normalizeBattleSide(actorSide)
	if actorSide == "" {
		actorSide = "player"
	}
	count := 0
	accumulate := func(unit *battleUnitState) {
		if unit == nil {
			return
		}
		if !unit.Alive || unit.HP <= 0 {
			return
		}
		if position != "all" && strings.ToLower(strings.TrimSpace(unit.Position)) != position {
			return
		}
		relation := "enemy"
		if battleUnitPerspectiveSide(unit) == actorSide {
			relation = "ally"
		}
		switch targetScope {
		case "none":
			return
		case "all":
			count++
		case "allies", "self":
			if relation == "ally" {
				count++
			}
		default:
			if relation == "enemy" {
				count++
			}
		}
	}
	for i := range state.Players {
		accumulate(&state.Players[i])
	}
	for i := range state.Enemies {
		accumulate(&state.Enemies[i])
	}
	return count
}

func evaluateBattleSkillFormula(actor *battleUnitState, formula []battleAttackTerm, level int) float64 {
	return evaluateBattleSkillFormulaWithExtra(actor, formula, level, nil)
}

func evaluateBattleSkillFormulaWithExtra(actor *battleUnitState, formula []battleAttackTerm, level int, extra map[string]float64) float64 {
	if actor == nil || len(formula) == 0 {
		return 0
	}
	levelOffset := maxFloat(0, float64(level-1))
	total := 0.0
	for _, term := range formula {
		attrKey := strings.TrimSpace(term.AttrKey)
		if attrKey == "" {
			continue
		}
		coefficient := term.Coefficient + term.PerSkillLevel*levelOffset
		if math.IsNaN(coefficient) || math.IsInf(coefficient, 0) {
			continue
		}
		attrValue := actor.Stats[attrKey]
		switch attrKey {
		case "skill_level":
			attrValue = float64(maxInt(1, level))
		case "current_hp":
			attrValue = actor.HP
		case "current_mp":
			attrValue = actor.MP
		case "max_hp":
			attrValue = actor.MaxHP
		case "max_mp":
			attrValue = actor.MaxMP
		}
		if extra != nil {
			if value, ok := extra[attrKey]; ok {
				attrValue = value
			}
		}
		total += attrValue * coefficient
	}
	return total
}

func resolveBattleTemplateTargetCount(actor *battleUnitState, level int, config battleSkillTargeting) int {
	base := maxInt(1, config.TargetCount.Base)
	bonus := evaluateBattleSkillFormula(actor, config.TargetCount.Formula, level)
	count := int(math.Floor(float64(base) + bonus))
	if count < 1 {
		count = 1
	}
	if config.TargetCount.MaxSet && count > config.TargetCount.Max {
		count = config.TargetCount.Max
	}
	if count < 1 {
		return 1
	}
	return count
}

func (s *Server) battleTemplateTargetsByScope(state *battleRuntimeState, actor *battleUnitState, scope string) []*battleUnitState {
	if state == nil || actor == nil {
		return nil
	}
	scope = strings.ToLower(strings.TrimSpace(scope))
	if scope == "self" {
		if actor.Alive && actor.HP > 0 {
			return []*battleUnitState{actor}
		}
		return nil
	}
	targetScope := scope
	if targetScope != "ally" && targetScope != "all" {
		targetScope = "enemy"
	}
	shouldInclude := func(unit *battleUnitState) bool {
		if unit == nil || !unit.Alive || unit.HP <= 0 {
			return false
		}
		if targetScope == "all" {
			return true
		}
		relationship := s.battleTargetRelationForActor(state, actor, unit)
		if targetScope == "ally" {
			return relationship == "ally"
		}
		return relationship == "enemy"
	}
	out := make([]*battleUnitState, 0, len(state.Players)+len(state.Enemies))
	for idx := range state.Players {
		unit := &state.Players[idx]
		if shouldInclude(unit) {
			out = append(out, unit)
		}
	}
	for idx := range state.Enemies {
		unit := &state.Enemies[idx]
		if shouldInclude(unit) {
			out = append(out, unit)
		}
	}
	return out
}

func (s *Server) battleHasRealAlly(state *battleRuntimeState, actor *battleUnitState) bool {
	if s == nil || state == nil || actor == nil {
		return false
	}
	actorID := strings.TrimSpace(actor.UnitID)
	actorSide := battleUnitPerspectiveSide(actor)
	allUnits := [][]battleUnitState{state.Players, state.Enemies}
	for _, rows := range allUnits {
		for idx := range rows {
			unit := &rows[idx]
			if unit == nil || !unit.Alive || unit.HP <= 0 {
				continue
			}
			if strings.TrimSpace(unit.UnitID) == actorID {
				continue
			}
			if battleUnitPerspectiveSide(unit) != actorSide {
				continue
			}
			return true
		}
	}
	return false
}

func (s *Server) battleHasRealEnemyWithoutCamouflage(state *battleRuntimeState, actor *battleUnitState) bool {
	if s == nil || state == nil || actor == nil {
		return false
	}
	actorSide := battleUnitPerspectiveSide(actor)
	allUnits := [][]battleUnitState{state.Players, state.Enemies}
	for _, rows := range allUnits {
		for idx := range rows {
			unit := &rows[idx]
			if unit == nil || !unit.Alive || unit.HP <= 0 {
				continue
			}
			if battleUnitPerspectiveSide(unit) == actorSide {
				continue
			}
			if s.battleUnitHasWhileActiveEffect(unit, "camouflage_targeting_remap", actor) {
				continue
			}
			return true
		}
	}
	return false
}

func (s *Server) battleTargetRelationForActor(state *battleRuntimeState, actor *battleUnitState, target *battleUnitState) string {
	if actor == nil || target == nil {
		return "enemy"
	}
	if s != nil && s.battleUnitHasWhileActiveEffect(actor, "treat_all_as_enemy", nil) {
		return "enemy"
	}
	actorSide := battleUnitPerspectiveSide(actor)
	targetSide := battleUnitPerspectiveSide(target)
	if actorSide == targetSide {
		return "ally"
	}
	if s == nil || state == nil {
		return "enemy"
	}
	if !s.battleUnitHasWhileActiveEffect(target, "camouflage_targeting_remap", actor) {
		return "enemy"
	}
	if !s.battleHasRealAlly(state, actor) {
		return "enemy"
	}
	if !s.battleHasRealEnemyWithoutCamouflage(state, actor) {
		return "enemy"
	}
	return "ally"
}

func (s *Server) battleRemapTargetScopeForActor(actor *battleUnitState, scope string) (string, bool) {
	normalized := normalizeBattleTargetScope(scope)
	if normalized != "enemy" {
		return normalized, false
	}
	if !s.battleUnitHasWhileActiveEffect(actor, "treat_all_as_enemy", nil) {
		return normalized, true
	}
	return "all", true
}

func battleParseStringList(raw any) []string {
	if raw == nil {
		return nil
	}
	values := []string{}
	appendValue := func(value any) {
		if value == nil {
			return
		}
		normalized := strings.ToLower(strings.TrimSpace(fmt.Sprintf("%v", value)))
		if normalized == "" || normalized == "<nil>" {
			return
		}
		values = append(values, normalized)
	}
	switch typed := raw.(type) {
	case []string:
		for _, item := range typed {
			appendValue(item)
		}
	case []any:
		for _, item := range typed {
			appendValue(item)
		}
	default:
		appendValue(raw)
	}
	if len(values) == 0 {
		return nil
	}
	return values
}

func battleTargetingOverrideSource(effect map[string]any) map[string]any {
	if effect == nil {
		return nil
	}
	if targetingNode, ok := effect["targeting"].(map[string]any); ok && len(targetingNode) > 0 {
		return targetingNode
	}
	return effect
}

func battleApplyTargetingOverride(base battleSkillTargeting, override map[string]any) battleSkillTargeting {
	if len(override) == 0 {
		return base
	}
	out := base
	if _, hasPositions := override["targetPositions"]; hasPositions {
		out.TargetPositions = parseBattleSkillTargetPositions(override["targetPositions"], firstNonEmptyTargetPosition(out.TargetPositions))
	} else if _, hasPosition := override["targetPosition"]; hasPosition {
		out.TargetPositions = parseBattleSkillTargetPositions(override["targetPosition"], firstNonEmptyTargetPosition(out.TargetPositions))
	} else if _, hasDirection := override["targetDirection"]; hasDirection {
		out.TargetPositions = parseBattleSkillTargetPositions(override["targetDirection"], firstNonEmptyTargetPosition(out.TargetPositions))
	}
	if rawSeek, exists := override["secondarySeek"]; exists {
		out.SecondarySeek = normalizeBattleSecondarySeek(fmt.Sprintf("%v", rawSeek), out.SecondarySeek)
	} else if rawSeek, exists := override["targetSeekStrategy"]; exists {
		out.SecondarySeek = normalizeBattleSecondarySeek(fmt.Sprintf("%v", rawSeek), out.SecondarySeek)
	}
	if rawScope, exists := override["targetScope"]; exists {
		out.TargetScope = normalizeBattleTargetScope(fmt.Sprintf("%v", rawScope))
	}
	if rawRule, exists := override["selectRule"]; exists {
		out.SelectRule = normalizeBattleSelectRule(fmt.Sprintf("%v", rawRule))
		if out.SelectRule != "attr_max" && out.SelectRule != "attr_min" {
			out.SelectAttrKey = ""
		}
	}
	if rawAttr, exists := override["selectAttrKey"]; exists {
		out.SelectAttrKey = strings.TrimSpace(fmt.Sprintf("%v", rawAttr))
	}
	if rawFallback, exists := override["fallbackSelfIfEmpty"]; exists {
		out.FallbackSelfIfEmpty = parseBooleanLoose(rawFallback, out.FallbackSelfIfEmpty)
	}
	if countNode, ok := override["targetCount"].(map[string]any); ok && len(countNode) > 0 {
		if rawBase, exists := countNode["base"]; exists {
			baseCount := int(toInt64Loose(rawBase, int64(out.TargetCount.Base)))
			if baseCount > 0 {
				out.TargetCount.Base = baseCount
			}
		}
		if rawFormula, exists := countNode["formula"]; exists {
			if formula := parseBattleAttackTerms(rawFormula); len(formula) > 0 {
				out.TargetCount.Formula = formula
			}
		}
		if rawMax, exists := countNode["max"]; exists {
			maxValue := int(toInt64Loose(rawMax, 0))
			if maxValue > 0 {
				out.TargetCount.Max = maxValue
				out.TargetCount.MaxSet = true
			} else {
				out.TargetCount.Max = 0
				out.TargetCount.MaxSet = false
			}
		}
	}
	return out
}

func (s *Server) battleTargetingOverrideApplies(effect map[string]any, isBasicAttack bool, skillID string) bool {
	if len(effect) == 0 {
		return false
	}
	applyToBasicAttack := parseBooleanLoose(effect["applyToBasicAttack"], true)
	applyToSkills := parseBooleanLoose(effect["applyToSkills"], true)
	if isBasicAttack {
		return applyToBasicAttack
	}
	if !applyToSkills {
		return false
	}
	allowedSkillIDs := battleParseStringList(effect["skillIds"])
	if len(allowedSkillIDs) == 0 {
		return true
	}
	normalizedSkillID := strings.ToLower(strings.TrimSpace(skillID))
	for _, allowed := range allowedSkillIDs {
		if allowed == normalizedSkillID {
			return true
		}
	}
	return false
}

func (s *Server) battleApplyBuffTargetingOverrides(actor *battleUnitState, skill battleSkillState, isBasicAttack bool) (battleSkillState, bool) {
	if s == nil || actor == nil || len(actor.Buffs) == 0 {
		return skill, false
	}
	overridden := false
	effectsCloned := false
	for i := range actor.Buffs {
		buffID := strings.TrimSpace(strings.ToLower(actor.Buffs[i].BuffID))
		if buffID == "" {
			continue
		}
		maxCharges := s.battleBuffMaxCharges(buffID)
		chargesRemaining := battleBuffChargesRemaining(&actor.Buffs[i], maxCharges)
		for _, rule := range s.battleFindBuffRules(buffID, "while_active") {
			if !battleRuleConditionsPassed(rule, actor, nil, 0, "", &actor.Buffs[i], chargesRemaining) {
				continue
			}
			for _, effect := range rule.Effects {
				kind := battleNormalizeTrigger(fmt.Sprintf("%v", effect["kind"]))
				if kind == "" {
					kind = battleNormalizeTrigger(fmt.Sprintf("%v", effect["type"]))
				}
				if kind != battleNormalizeTrigger(battleEffectOverrideTargeting) || !s.battleTargetingOverrideApplies(effect, isBasicAttack, skill.ID) {
					continue
				}
				overrideNode := battleTargetingOverrideSource(effect)
				skill.Targeting = battleApplyTargetingOverride(skill.Targeting, overrideNode)
				if len(skill.Effects) > 0 && !effectsCloned {
					skill.Effects = append([]battleSkillEffect(nil), skill.Effects...)
					effectsCloned = true
				}
				for idx := range skill.Effects {
					updatedEffect := skill.Effects[idx]
					updatedEffect.Targeting = battleApplyTargetingOverride(updatedEffect.Targeting, overrideNode)
					skill.Effects[idx] = updatedEffect
				}
				overridden = true
			}
		}
	}
	return skill, overridden
}

func (s *Server) battleFilterUntargetableCandidates(actor *battleUnitState, candidates []*battleUnitState, enemyPerspective bool, allyPerspective bool, ignoreUntargetable bool) []*battleUnitState {
	if (!enemyPerspective && !allyPerspective) || ignoreUntargetable || len(candidates) == 0 {
		return candidates
	}
	out := make([]*battleUnitState, 0, len(candidates))
	for _, target := range candidates {
		if target == nil {
			continue
		}
		if enemyPerspective && s.battleUnitHasWhileActiveEffect(target, "untargetable_by_enemy", actor) {
			continue
		}
		if allyPerspective && strings.TrimSpace(target.UnitID) != strings.TrimSpace(actor.UnitID) && s.battleUnitHasWhileActiveEffect(target, battleEffectUntargetableByAlly, actor) {
			continue
		}
		out = append(out, target)
	}
	return out
}

func battleTemplateFilterByPositions(candidates []*battleUnitState, positions []string) []*battleUnitState {
	if len(candidates) == 0 {
		return nil
	}
	allowed := make(map[string]struct{}, len(positions))
	for _, row := range positions {
		normalized := normalizeBattleTargetPosition(row)
		if normalized == "" {
			continue
		}
		allowed[normalized] = struct{}{}
	}
	if len(allowed) == 0 {
		return candidates
	}
	out := make([]*battleUnitState, 0, len(candidates))
	for _, unit := range candidates {
		if unit == nil {
			continue
		}
		position := normalizeBattleTargetPosition(unit.Position)
		if _, ok := allowed[position]; !ok {
			continue
		}
		out = append(out, unit)
	}
	return out
}

func battleTemplateFilterBossTargets(candidates []*battleUnitState) []*battleUnitState {
	if len(candidates) == 0 {
		return nil
	}
	out := make([]*battleUnitState, 0, len(candidates))
	for _, unit := range candidates {
		if unit == nil {
			continue
		}
		if !battleUnitIsBoss(unit) {
			continue
		}
		out = append(out, unit)
	}
	if len(out) == 0 {
		return candidates
	}
	return out
}

func battleUnitIsBoss(unit *battleUnitState) bool {
	if unit == nil {
		return false
	}
	enemyType := strings.ToLower(strings.TrimSpace(unit.EnemyType))
	if enemyType == "boss" || enemyType == "world_boss" {
		return true
	}
	enemyID := strings.ToLower(strings.TrimSpace(unit.EnemyID))
	return strings.Contains(enemyID, "boss")
}

func battleTemplateFallbackTargets(candidates []*battleUnitState, strategy string, randFn func() float64) []*battleUnitState {
	if len(candidates) == 0 {
		return nil
	}
	strategy = strings.ToLower(strings.TrimSpace(strategy))
	if strategy == "none" {
		return nil
	}
	if strategy == "random_alive" {
		out := append([]*battleUnitState(nil), candidates...)
		for idx := len(out) - 1; idx > 0; idx-- {
			pick := int(math.Floor(clampFloat(randFn(), 0, 0.999999) * float64(idx+1)))
			out[idx], out[pick] = out[pick], out[idx]
		}
		return out
	}
	order := []string{"vanguard", "midguard", "rearguard"}
	if strategy == "rear_to_front" {
		order = []string{"rearguard", "midguard", "vanguard"}
	}
	for _, position := range order {
		out := make([]*battleUnitState, 0, len(candidates))
		for _, unit := range candidates {
			if unit == nil {
				continue
			}
			if normalizeBattleTargetPosition(unit.Position) != position {
				continue
			}
			out = append(out, unit)
		}
		if len(out) > 0 {
			return out
		}
	}
	return nil
}

func battleTemplateSortTargets(candidates []*battleUnitState, selectRule string, selectAttrKey string, randFn func() float64) {
	selectRule = strings.ToLower(strings.TrimSpace(selectRule))
	selectAttrKey = strings.TrimSpace(selectAttrKey)
	if selectRule == "first" || selectRule == "boss_first" || selectRule == "self" {
		return
	}
	if selectRule == "random" {
		for idx := len(candidates) - 1; idx > 0; idx-- {
			pick := int(math.Floor(clampFloat(randFn(), 0, 0.999999) * float64(idx+1)))
			candidates[idx], candidates[pick] = candidates[pick], candidates[idx]
		}
		return
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		left := candidates[i]
		right := candidates[j]
		if left == nil || right == nil {
			return false
		}
		leftValue := left.HP
		rightValue := right.HP
		switch selectRule {
		case "hp_max", "hp_min":
			leftValue = left.HP
			rightValue = right.HP
		case "hp_percent_max", "hp_percent_min":
			if left.MaxHP > 0 {
				leftValue = left.HP / left.MaxHP
			}
			if right.MaxHP > 0 {
				rightValue = right.HP / right.MaxHP
			}
		case "attr_max", "attr_min":
			leftValue = battleTemplateTargetAttributeValue(left, selectAttrKey)
			rightValue = battleTemplateTargetAttributeValue(right, selectAttrKey)
		}
		if leftValue == rightValue {
			return left.UnitID < right.UnitID
		}
		if selectRule == "hp_max" || selectRule == "attr_max" {
			return leftValue > rightValue
		}
		return leftValue < rightValue
	})
}

func battleTemplateTargetAttributeValue(unit *battleUnitState, attrKey string) float64 {
	if unit == nil {
		return 0
	}
	switch strings.ToLower(strings.TrimSpace(attrKey)) {
	case "hp":
		return unit.HP
	case "mp":
		return unit.MP
	default:
		return unit.Stats[strings.TrimSpace(attrKey)]
	}
}

func (s *Server) battleSelectTemplateTargets(state *battleRuntimeState, actor *battleUnitState, skill battleSkillState, targeting battleSkillTargeting, ignoreUntargetable bool, randFn func() float64) []*battleUnitState {
	if state == nil || actor == nil {
		return nil
	}
	if randFn == nil {
		randFn = func() float64 { return 0.5 }
	}
	targetScope, enemyPerspective := s.battleRemapTargetScopeForActor(actor, targeting.TargetScope)
	allyPerspective := targetScope == "ally"
	if enemyPerspective && s.battleUnitHasWhileActiveEffect(actor, battleEffectFrontlineFixation, nil) {
		targeting.TargetPositions = []string{"vanguard"}
		targeting.SecondarySeek = "front_to_rear"
	}
	candidates := s.battleTemplateTargetsByScope(state, actor, targetScope)
	if targeting.ExcludeSelf {
		withoutSelf := make([]*battleUnitState, 0, len(candidates))
		for _, candidate := range candidates {
			if candidate == nil || strings.TrimSpace(candidate.UnitID) == strings.TrimSpace(actor.UnitID) {
				continue
			}
			withoutSelf = append(withoutSelf, candidate)
		}
		candidates = withoutSelf
	}
	if len(candidates) == 0 {
		return nil
	}
	if targetScope == "self" {
		return candidates
	}
	filteredCandidates := s.battleFilterUntargetableCandidates(actor, candidates, enemyPerspective, allyPerspective, ignoreUntargetable)
	filteredCandidates = battleFilterTargetsByPredicates(actor, filteredCandidates, targeting.TargetPredicates)
	if len(filteredCandidates) == 0 && targeting.FallbackSelfIfEmpty && actor != nil {
		return []*battleUnitState{actor}
	}
	if len(filteredCandidates) == 0 {
		return nil
	}
	count := resolveBattleTemplateTargetCount(actor, skill.Level, targeting)
	if (enemyPerspective || allyPerspective) && !targeting.IgnoreTargetPriority {
		prioritizedCandidates, preferredCount, hasPriorityOrdering := s.battlePrioritizeTargetCandidates(actor, filteredCandidates, enemyPerspective, allyPerspective)
		if preferredCount > 0 {
			preferred := append([]*battleUnitState(nil), prioritizedCandidates[:preferredCount]...)
			others := append([]*battleUnitState(nil), prioritizedCandidates[preferredCount:]...)
			if strings.EqualFold(strings.TrimSpace(targeting.SelectRule), "boss_first") {
				preferred = battleTemplateFilterBossTargets(preferred)
				if len(others) > 0 {
					others = battleTemplateFilterBossTargets(others)
				}
			}
			battleTemplateSortTargets(preferred, targeting.SelectRule, targeting.SelectAttrKey, randFn)
			if len(others) > 0 {
				battleTemplateSortTargets(others, targeting.SelectRule, targeting.SelectAttrKey, randFn)
			}
			out := append(preferred, others...)
			if count >= len(out) {
				return out
			}
			return append([]*battleUnitState(nil), out[:count]...)
		}
		if hasPriorityOrdering {
			filteredCandidates = prioritizedCandidates
		}
	}
	selectedPool := battleTemplateFilterByPositions(filteredCandidates, targeting.TargetPositions)
	if len(selectedPool) == 0 {
		selectedPool = battleTemplateFallbackTargets(filteredCandidates, targeting.SecondarySeek, randFn)
	}
	if len(selectedPool) == 0 && targeting.FallbackSelfIfEmpty && actor != nil {
		return []*battleUnitState{actor}
	}
	if len(selectedPool) == 0 {
		return nil
	}
	selectedPool = battleFilterTargetsByPredicates(actor, selectedPool, targeting.TargetPredicates)
	if len(selectedPool) == 0 && targeting.FallbackSelfIfEmpty && actor != nil {
		return []*battleUnitState{actor}
	}
	if len(selectedPool) == 0 {
		return nil
	}
	if strings.EqualFold(strings.TrimSpace(targeting.SelectRule), "boss_first") {
		selectedPool = battleTemplateFilterBossTargets(selectedPool)
	}
	if strings.EqualFold(strings.TrimSpace(targeting.SelectRule), "self") && actor != nil {
		out := make([]*battleUnitState, 0, len(selectedPool))
		out = append(out, actor)
		for _, target := range selectedPool {
			if target == nil || strings.TrimSpace(target.UnitID) == strings.TrimSpace(actor.UnitID) {
				continue
			}
			out = append(out, target)
		}
		if count := resolveBattleTemplateTargetCount(actor, skill.Level, targeting); count < len(out) {
			return append([]*battleUnitState(nil), out[:count]...)
		}
		return out
	}
	out := append([]*battleUnitState(nil), selectedPool...)
	battleTemplateSortTargets(out, targeting.SelectRule, targeting.SelectAttrKey, randFn)
	normalizedSelectRule := strings.ToLower(strings.TrimSpace(targeting.SelectRule))
	isHPBasedRule := normalizedSelectRule == "hp_percent_min" || normalizedSelectRule == "hp_percent_max" ||
		normalizedSelectRule == "hp_min" || normalizedSelectRule == "hp_max"
	if (enemyPerspective || allyPerspective) && count < len(out) && !isHPBasedRule {
		prioritized, _, hasPriorityOrdering := s.battlePrioritizeTargetCandidates(actor, out, enemyPerspective, allyPerspective)
		if hasPriorityOrdering {
			out = prioritized
		}
	}
	if count >= len(out) {
		return out
	}
	return append([]*battleUnitState(nil), out[:count]...)
}

func (s *Server) battleSelectCorpseTargets(state *battleRuntimeState, actor *battleUnitState, skill battleSkillState, targeting battleSkillTargeting, randFn func() float64) []*battleUnitState {
	if state == nil || actor == nil {
		return nil
	}
	if randFn == nil {
		randFn = func() float64 { return 0.5 }
	}
	targetScope, _ := s.battleRemapTargetScopeForActor(actor, targeting.TargetScope)
	candidates := make([]*battleUnitState, 0, len(state.Players)+len(state.Enemies))
	appendIfMatches := func(unit *battleUnitState) {
		if unit == nil || unit.CorpseConsumed || unit.Alive || unit.HP > 0 {
			return
		}
		if targetScope == "self" {
			if strings.TrimSpace(unit.UnitID) == strings.TrimSpace(actor.UnitID) {
				candidates = append(candidates, unit)
			}
			return
		}
		if targetScope == "all" {
			candidates = append(candidates, unit)
			return
		}
		relationship := s.battleTargetRelationForActor(state, actor, unit)
		if targetScope == "ally" && relationship == "ally" {
			candidates = append(candidates, unit)
			return
		}
		if targetScope != "ally" && relationship == "enemy" {
			candidates = append(candidates, unit)
		}
	}
	for idx := range state.Players {
		appendIfMatches(&state.Players[idx])
	}
	for idx := range state.Enemies {
		appendIfMatches(&state.Enemies[idx])
	}
	if len(candidates) == 0 {
		return nil
	}
	count := resolveBattleTemplateTargetCount(actor, skill.Level, targeting)
	selectedPool := battleTemplateFilterByPositions(candidates, targeting.TargetPositions)
	if len(selectedPool) == 0 {
		selectedPool = battleTemplateFallbackTargets(candidates, targeting.SecondarySeek, randFn)
	}
	if len(selectedPool) == 0 {
		return nil
	}
	selectedPool = battleFilterTargetsByPredicates(actor, selectedPool, targeting.TargetPredicates)
	if len(selectedPool) == 0 {
		return nil
	}
	out := append([]*battleUnitState(nil), selectedPool...)
	battleTemplateSortTargets(out, targeting.SelectRule, targeting.SelectAttrKey, randFn)
	if count >= len(out) {
		return out
	}
	return append([]*battleUnitState(nil), out[:count]...)
}

func (s *Server) battlePrioritizeEnemyTargetCandidates(actor *battleUnitState, candidates []*battleUnitState) ([]*battleUnitState, int) {
	out, preferredCount, _ := s.battlePrioritizeTargetCandidates(actor, candidates, true, false)
	return out, preferredCount
}

func (s *Server) battlePrioritizeTargetCandidates(actor *battleUnitState, candidates []*battleUnitState, enemyPerspective bool, allyPerspective bool) ([]*battleUnitState, int, bool) {
	if s == nil || len(candidates) == 0 {
		return candidates, 0, false
	}
	preferred := make([]*battleUnitState, 0, len(candidates))
	neutral := make([]*battleUnitState, 0, len(candidates))
	avoided := make([]*battleUnitState, 0, len(candidates))
	others := make([]*battleUnitState, 0, len(candidates))
	for _, unit := range candidates {
		if unit == nil {
			continue
		}
		preferEffect := ""
		avoidEffect := ""
		if enemyPerspective {
			preferEffect = battleEffectPreferAsEnemyTarget
			avoidEffect = battleEffectAvoidAsEnemyTarget
			if battleUnitHasPreferredSpecializationMark(unit) {
				preferred = append(preferred, unit)
				continue
			}
		}
		if allyPerspective {
			preferEffect = battleEffectPreferAsAllyTarget
			avoidEffect = battleEffectAvoidAsAllyTarget
		}
		if preferEffect != "" && s.battleUnitHasWhileActiveEffect(unit, preferEffect, actor) {
			preferred = append(preferred, unit)
			continue
		}
		if avoidEffect != "" && s.battleUnitHasWhileActiveEffect(unit, avoidEffect, actor) {
			avoided = append(avoided, unit)
			continue
		}
		neutral = append(neutral, unit)
	}
	if len(preferred) == 0 && len(avoided) == 0 {
		return candidates, 0, false
	}
	out := make([]*battleUnitState, 0, len(candidates))
	out = append(out, preferred...)
	out = append(out, neutral...)
	out = append(out, avoided...)
	out = append(out, others...)
	return out, len(preferred), true
}

func battleUnitHasPreferredSpecializationMark(unit *battleUnitState) bool {
	if unit == nil {
		return false
	}
	for _, buff := range unit.Buffs {
		if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(buff.BuffID)), "specialization:mark:") {
			continue
		}
		if parseBooleanLoose(buff.Payload["preferMarked"], false) {
			return true
		}
	}
	return false
}

func battleSpecializationCounterBuffID(counterID string) string {
	return "specialization:counter:" + strings.ToLower(strings.TrimSpace(counterID))
}

func battleSpecializationMarkBuffID(markID string) string {
	return "specialization:mark:" + strings.ToLower(strings.TrimSpace(markID))
}

func battleSpecializationCounterStacks(unit *battleUnitState, counterID string) int {
	buff := findBuffByID(unit, battleSpecializationCounterBuffID(counterID))
	if buff == nil {
		return 0
	}
	return maxInt(0, int(toInt64Loose(buff.Payload["stacks"], 0)))
}

func battleSetSpecializationCounter(unit *battleUnitState, counterID string, stacks int) {
	if unit == nil {
		return
	}
	buffID := battleSpecializationCounterBuffID(counterID)
	if stacks <= 0 {
		battleRemoveBuff(unit, buffID)
		return
	}
	battleApplyOrRefreshBuff(unit, buffID, -1, map[string]any{"stacks": stacks}, unit.UnitID, "refresh_duration")
}

func battleUnitHasAnySpecializationMark(unit *battleUnitState) bool {
	if unit == nil {
		return false
	}
	for _, buff := range unit.Buffs {
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(buff.BuffID)), "specialization:mark:") {
			return true
		}
	}
	return false
}

func battlePositionOrdinal(position string) int {
	switch normalizeBattleTargetPosition(position) {
	case "vanguard":
		return 0
	case "midguard":
		return 1
	case "rearguard":
		return 2
	default:
		return -1
	}
}

func battleAlliesForSpecialization(state *battleRuntimeState, self *battleUnitState) []*battleUnitState {
	if state == nil || self == nil {
		return nil
	}
	out := make([]*battleUnitState, 0, len(state.Players)+len(state.Enemies))
	appendUnit := func(unit *battleUnitState) {
		if unit == nil || !unit.Alive || unit.HP <= 0 || strings.TrimSpace(unit.UnitID) == strings.TrimSpace(self.UnitID) {
			return
		}
		if battleUnitTeamSide(unit) == battleUnitTeamSide(self) {
			out = append(out, unit)
		}
	}
	for index := range state.Players {
		appendUnit(&state.Players[index])
	}
	for index := range state.Enemies {
		appendUnit(&state.Enemies[index])
	}
	return out
}

func battleLowestHPAlly(state *battleRuntimeState, self *battleUnitState) *battleUnitState {
	allies := battleAlliesForSpecialization(state, self)
	sort.SliceStable(allies, func(i, j int) bool {
		leftRatio, rightRatio := 1.0, 1.0
		if allies[i].MaxHP > 0 {
			leftRatio = allies[i].HP / allies[i].MaxHP
		}
		if allies[j].MaxHP > 0 {
			rightRatio = allies[j].HP / allies[j].MaxHP
		}
		if leftRatio == rightRatio {
			return allies[i].UnitID < allies[j].UnitID
		}
		return leftRatio < rightRatio
	})
	if len(allies) == 0 {
		return nil
	}
	return allies[0]
}

func battleIsLowestHPAlly(state *battleRuntimeState, self, target *battleUnitState) bool {
	lowest := battleLowestHPAlly(state, self)
	return lowest != nil && target != nil && strings.TrimSpace(lowest.UnitID) == strings.TrimSpace(target.UnitID)
}

func battleRealPlayerPartyContainsUnit(state *battleRuntimeState, unit *battleUnitState) bool {
	if state == nil || unit == nil || battleUnitIsSummon(unit) {
		return false
	}
	unitID := strings.TrimSpace(unit.UnitID)
	if unitID == "" {
		return false
	}
	for index := range state.Players {
		candidate := &state.Players[index]
		if strings.TrimSpace(candidate.UnitID) == unitID && !battleUnitIsSummon(candidate) {
			return true
		}
	}
	return false
}

func battlePlayerPartyEquippedUnitCount(state *battleRuntimeState, itemID string) int {
	if state == nil {
		return 0
	}
	itemID = strings.TrimSpace(itemID)
	if itemID == "" {
		return 0
	}
	count := 0
	for index := range state.Players {
		unit := &state.Players[index]
		if battleUnitIsSummon(unit) {
			continue
		}
		for _, hook := range unit.EquipmentHooks {
			if strings.EqualFold(strings.TrimSpace(hook.SourceItemID), itemID) {
				count++
				break
			}
		}
	}
	return count
}

func battleAdjacentLowestHPAlly(state *battleRuntimeState, self *battleUnitState) *battleUnitState {
	if self == nil {
		return nil
	}
	selfPosition := battlePositionOrdinal(self.Position)
	allies := battleAlliesForSpecialization(state, self)
	filtered := allies[:0]
	for _, ally := range allies {
		position := battlePositionOrdinal(ally.Position)
		if selfPosition >= 0 && position >= 0 && int(math.Abs(float64(position-selfPosition))) <= 1 {
			filtered = append(filtered, ally)
		}
	}
	if len(filtered) == 0 {
		return nil
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		left, right := filtered[i].HP/maxFloat(1, filtered[i].MaxHP), filtered[j].HP/maxFloat(1, filtered[j].MaxHP)
		if left == right {
			return filtered[i].UnitID < filtered[j].UnitID
		}
		return left < right
	})
	return filtered[0]
}

func (s *Server) battleEvaluateEquipmentHookCondition(
	state *battleRuntimeState,
	condition map[string]any,
	self *battleUnitState,
	source *battleUnitState,
	target *battleUnitState,
	incomingDamage float64,
	incomingDamageType string,
	sourceEnhanceLevel int,
	context *battleEquipmentHookContext,
) bool {
	if len(condition) == 0 {
		return true
	}
	condType := battleNormalizeTrigger(fmt.Sprintf("%v", firstNonNil(condition["type"], condition["kind"])))
	switch condType {
	case "contextdamagetype":
		return normalizeBattleDamageType(fmt.Sprintf("%v", condition["value"])) == normalizeBattleDamageType(incomingDamageType)
	case "contexthitcountgte":
		return context != nil && context.HitCount >= int(toInt64Loose(condition["value"], 0))
	case "contextmpinsufficient":
		return context != nil && context.MPInsufficient
	case "contextskillnonbasic":
		return context != nil && strings.TrimSpace(context.SkillID) != "" && !strings.EqualFold(strings.TrimSpace(context.SkillID), strings.TrimSpace(s.battleDefaultBasicSkillID()))
	case "countergte":
		return battleSpecializationCounterStacks(self, fmt.Sprintf("%v", firstNonNil(condition["counterId"], condition["counter_id"]))) >= int(toInt64Loose(condition["value"], 0))
	case "incomingdamagewouldbelethal":
		return self != nil && incomingDamage >= self.HP
	case "selfhpratiolte":
		return self != nil && self.MaxHP > 0 && self.HP/self.MaxHP <= toFloat64(condition["value"], 0)
	case "selfmpratiogte":
		return self != nil && self.MaxMP > 0 && self.MP/self.MaxMP >= toFloat64(condition["value"], 0)
	case "sourceisenemy":
		return state != nil && self != nil && source != nil && s.battleTargetRelationForActor(state, self, source) == "enemy"
	case "targethasmark":
		return battleHasBuff(target, battleSpecializationMarkBuffID(fmt.Sprintf("%v", firstNonNil(condition["markId"], condition["mark_id"]))))
	case "targethasanyspecializationmark":
		return battleUnitHasAnySpecializationMark(target)
	case "targethasbuff":
		return target != nil && battleHasBuff(target, strings.TrimSpace(fmt.Sprintf("%v", condition["buffId"])))
	case "targethasanybuff":
		if target == nil {
			return false
		}
		for _, buffID := range compactStringSlice(condition["buffIds"]) {
			if battleHasBuff(target, buffID) {
				return true
			}
		}
		return false
	case "targetislowesthpally":
		return state != nil && self != nil && target != nil && battleIsLowestHPAlly(state, self, target)
	case "adjacentallyexists":
		return state != nil && self != nil && battleAdjacentLowestHPAlly(state, self) != nil
	case "equipmentenhancelevelgte", "sourceequipmentenhancelevelgte":
		return sourceEnhanceLevel >= int(toInt64Loose(condition["value"], 0))
	case "playerpartyequippedunitcounteq":
		if !battleRealPlayerPartyContainsUnit(state, self) {
			return false
		}
		itemID := strings.TrimSpace(fmt.Sprintf("%v", firstNonNil(condition["itemId"], condition["item_id"])))
		expected := int(toInt64Loose(condition["value"], 0))
		return itemID != "" && expected >= 0 && battlePlayerPartyEquippedUnitCount(state, itemID) == expected
	case "selfcompletedturncountmodeq":
		if state == nil || self == nil {
			return false
		}
		divisor := int(toInt64Loose(condition["divisor"], 0))
		remainder := int(toInt64Loose(condition["remainder"], 0))
		if divisor <= 0 || remainder < 0 || remainder >= divisor {
			return false
		}
		return state.CompletedTurnCounts[self.UnitID]%divisor == remainder
	case "selfposition", "selfisinposition":
		if self == nil {
			return false
		}
		positions := battleParseStringList(firstNonNil(condition["positions"], condition["position"], condition["value"]))
		if len(positions) == 0 {
			return false
		}
		current := normalizeBattleTargetPosition(self.Position)
		for _, position := range positions {
			if current == normalizeBattleTargetPosition(position) {
				return true
			}
		}
		return false
	case "allysidehaspositions", "allysidecoverspositions":
		return battleRuntimeSideHasLivingPositions(state, battleUnitTeamSide(self), battleParseStringList(firstNonNil(condition["positions"], condition["position"], condition["value"])))
	case "targetdistinctsourcebuffcountgte":
		if s == nil || self == nil || target == nil {
			return false
		}
		wantedKind := strings.TrimSpace(strings.ToLower(fmt.Sprintf("%v", condition["buffKind"])))
		if wantedKind == "" {
			return false
		}
		distinct := map[string]struct{}{}
		for _, buff := range target.Buffs {
			buffID := strings.TrimSpace(strings.ToLower(buff.BuffID))
			if buffID == "" || strings.TrimSpace(buff.SourceID) != strings.TrimSpace(self.UnitID) || s.battleBuffKindByID(buffID) != wantedKind {
				continue
			}
			distinct[buffID] = struct{}{}
		}
		return len(distinct) >= int(toInt64Loose(condition["value"], 0))
	default:
		return battleEvaluateBuffConditionWithTarget(condition, self, source, target, incomingDamage, incomingDamageType, nil, 0)
	}
}

func (s *Server) battleBuffDefinesWhileActiveEffect(buffID string, effectKind string) bool {
	effectKind = battleNormalizeTrigger(effectKind)
	if s == nil || strings.TrimSpace(buffID) == "" || effectKind == "" {
		return false
	}
	for _, rule := range s.battleFindBuffRules(buffID, "while_active") {
		for index, effect := range rule.Effects {
			kind := ""
			if index < len(rule.EffectKinds) {
				kind = rule.EffectKinds[index]
			} else {
				kind = battleNormalizeTrigger(fmt.Sprintf("%v", firstNonNil(effect["kind"], effect["type"])))
			}
			if kind == effectKind {
				return true
			}
		}
	}
	return false
}

func (s *Server) battleUnitTurnsIneligible(state *battleRuntimeState, unit *battleUnitState) bool {
	if s == nil || unit == nil {
		return false
	}
	if battleCombatTrialActive(state) {
		if _, scriptedSupport := state.CombatTrialSupportUnitIDs[strings.TrimSpace(unit.UnitID)]; scriptedSupport {
			return true
		}
	}
	if unit.TurnIneligible || s.battleUnitHasWhileActiveEffect(unit, battleEffectTurnIneligible, nil) {
		return true
	}
	// Opening hooks can attempt an extra turn before their own passive buff has
	// been attached. Treat an eligible opening turn-lock as active immediately.
	for _, hook := range unit.EquipmentHooks {
		trigger := battleNormalizeTrigger(hook.Trigger)
		if trigger != battleNormalizeTrigger(battleEquipmentTriggerBattleStart) && trigger != battleNormalizeTrigger(battleEquipmentTriggerWaveStart) {
			continue
		}
		conditionsPassed := true
		for _, condition := range hook.Conditions {
			if !s.battleEvaluateEquipmentHookCondition(state, condition, unit, nil, nil, 0, "", hook.SourceEnhanceLevel, nil) {
				conditionsPassed = false
				break
			}
		}
		if !conditionsPassed {
			continue
		}
		for _, effect := range hook.Effects {
			if battleNormalizeTrigger(fmt.Sprintf("%v", firstNonNil(effect["kind"], effect["type"]))) != "addbuff" {
				continue
			}
			if targetMode := battleNormalizeTrigger(fmt.Sprintf("%v", firstNonNil(effect["targetMode"], effect["target"]))); targetMode != "self" {
				continue
			}
			buffID := strings.TrimSpace(fmt.Sprintf("%v", effect["buffId"]))
			if s.battleBuffDurationByIDTurns(buffID) < 0 && s.battleBuffDefinesWhileActiveEffect(buffID, battleEffectTurnIneligible) {
				return true
			}
		}
	}
	return false
}

func battleRuntimeSideHasLivingPositions(state *battleRuntimeState, side string, positions []string) bool {
	if state == nil {
		return false
	}
	side = normalizeBattleSide(side)
	if side == "" || len(positions) == 0 {
		return false
	}
	required := map[string]struct{}{}
	for _, position := range positions {
		normalized := normalizeBattleTargetPosition(position)
		if normalized != "" {
			required[normalized] = struct{}{}
		}
	}
	if len(required) == 0 {
		return false
	}
	check := func(unit *battleUnitState) {
		if unit == nil || !unit.Alive || unit.HP <= 0 {
			return
		}
		if battleUnitTeamSide(unit) != side {
			return
		}
		delete(required, normalizeBattleTargetPosition(unit.Position))
	}
	for idx := range state.Players {
		check(&state.Players[idx])
	}
	for idx := range state.Enemies {
		check(&state.Enemies[idx])
	}
	return len(required) == 0
}

func (s *Server) battleTriggerUnitEquipmentHooks(
	state *battleRuntimeState,
	now time.Time,
	self *battleUnitState,
	source *battleUnitState,
	trigger string,
	incomingDamage float64,
	incomingDamageType string,
) []map[string]any {
	return s.battleTriggerUnitEquipmentHooksWithTarget(state, now, self, source, nil, trigger, incomingDamage, incomingDamageType)
}

func (s *Server) battleTriggerUnitEquipmentHooksWithTarget(
	state *battleRuntimeState,
	now time.Time,
	self *battleUnitState,
	source *battleUnitState,
	target *battleUnitState,
	trigger string,
	incomingDamage float64,
	incomingDamageType string,
) []map[string]any {
	return s.battleTriggerUnitEquipmentHooksWithContext(state, now, self, source, target, trigger, incomingDamage, incomingDamageType, nil)
}

func (s *Server) battleTriggerUnitEquipmentHooksWithContext(
	state *battleRuntimeState,
	now time.Time,
	self *battleUnitState,
	source *battleUnitState,
	target *battleUnitState,
	trigger string,
	incomingDamage float64,
	incomingDamageType string,
	context *battleEquipmentHookContext,
) []map[string]any {
	if s == nil || self == nil || self.equipmentHookCount() == 0 || (state != nil && state.derivedHookDispatchSuppression > 0) {
		return nil
	}
	finishDispatchScope := battleBeginSpecializationDispatchScope(state)
	defer finishDispatchScope()
	triggerKey := battleNormalizeTrigger(trigger)
	if triggerKey == "" {
		return nil
	}
	actions := make([]map[string]any, 0, 2)
	affectedUnits := map[string]*battleUnitState{}
	for hookIndex := 0; hookIndex < self.equipmentHookCount(); hookIndex++ {
		hook, ok := self.equipmentHookAt(hookIndex)
		if !ok {
			continue
		}
		if hook.SourceKind == "specialization" {
			if err := validateBattleSpecializationHookRule(hook); err != nil {
				actions = append(actions, map[string]any{
					"kind": "specialization_definition_invalid", "targetId": self.UnitID, "sourceId": self.UnitID,
					"reason": "specialization_fail_closed", "meta": map[string]any{"hookId": hook.ID, "error": err.Error()},
				})
				continue
			}
		}
		if battleNormalizeTrigger(hook.Trigger) != triggerKey {
			continue
		}
		subject := target
		if hook.SourceKind == "specialization" && subject == nil {
			subject = self
		}
		if !battleEquipmentHookSubjectAllowed(hook, subject) {
			continue
		}
		if !battleEquipmentHookEventAllowed(hook, self, source, context) {
			continue
		}
		if !battleEquipmentHookActivationAllowed(state, self, hook) {
			continue
		}
		// Specialization hooks use the contribution ancestry guard below so every
		// blocked recursive dispatch is explicit in the authoritative action log.
		// Keep the legacy active-hook guard for equipment/personality effects.
		needsExtraTurnGuard := hook.SourceKind != "specialization" && battleEquipmentHookNeedsReentryGuard(hook)
		if needsExtraTurnGuard && battleUnitHasActiveExtraTurn(state, self) {
			if !battleIsProbabilityOutcomeTrigger(triggerKey) || !battleCanQueueProbabilityExtraTurn(state, self, hook) {
				continue
			}
		}
		hookKey := ""
		if needsExtraTurnGuard {
			hookKey = battleEquipmentHookActiveKey(self, triggerKey, hook)
		}
		if state != nil && hookKey != "" {
			if state.activeEquipmentHookKeys != nil && state.activeEquipmentHookKeys[hookKey] > 0 {
				continue
			}
			if state.activeEquipmentHookKeys == nil {
				state.activeEquipmentHookKeys = map[string]int{}
			}
			state.activeEquipmentHookKeys[hookKey]++
			defer func(key string) {
				if state.activeEquipmentHookKeys == nil {
					return
				}
				state.activeEquipmentHookKeys[key]--
				if state.activeEquipmentHookKeys[key] <= 0 {
					delete(state.activeEquipmentHookKeys, key)
				}
			}(hookKey)
		}
		conditionsPassed := true
		for _, condition := range hook.Conditions {
			if !s.battleEvaluateEquipmentHookCondition(state, condition, self, source, target, incomingDamage, incomingDamageType, hook.SourceEnhanceLevel, context) {
				conditionsPassed = false
				break
			}
		}
		if !conditionsPassed {
			continue
		}
		leaveContribution := func() {}
		if hook.SourceKind == "specialization" {
			var blockedBy string
			leaveContribution, blockedBy = battleEnterSpecializationContribution(state, hook)
			if blockedBy != "" {
				actions = append(actions, battleSpecializationDispatchBlockedAction(self, trigger, hook, blockedBy))
				continue
			}
		}
		hookActionStart := len(actions)
		specializationEffectApplied := false
		markBeforeEffects := hook.SourceKind != "specialization"
		if markBeforeEffects {
			activationActions, activated := battleActivateEquipmentHook(state, self, hook, trigger)
			if !activated {
				leaveContribution()
				continue
			}
			actions = append(actions, activationActions...)
		}
		resolveTargets := func(targetMode string, effect map[string]any) []*battleUnitState {
			targets := make([]*battleUnitState, 0, 1)
			switch targetMode {
			case "source":
				if source != nil {
					targets = append(targets, source)
				}
			case "target":
				if target != nil {
					targets = append(targets, target)
				}
			case "targeting":
				if hook.SourceKind == "specialization" && target != nil && normalizeAnyMap(effect["targeting"]) == nil {
					targets = append(targets, target)
					break
				}
				if state == nil {
					break
				}
				targeting := parseBattleSkillTargeting(effect["targeting"], "vanguard", "front_to_rear", 1, nil, 0, false, nil)
				targets = s.battleSelectTemplateTargets(state, self, battleSkillState{Level: 1}, targeting, false, state.randFloat64)
			case "enemy", "markedenemy":
				if state == nil {
					break
				}
				candidates := s.battleTemplateTargetsByScope(state, self, "enemy")
				if targetMode == "markedenemy" {
					marked := candidates[:0]
					for _, candidate := range candidates {
						if battleUnitHasAnySpecializationMark(candidate) {
							marked = append(marked, candidate)
						}
					}
					candidates = marked
				}
				selectionMode := fmt.Sprintf("%v", firstNonNil(effect["selectionMode"], effect["selection_mode"]))
				if selected := battleSelectSpecializationTarget(state, candidates, selectionMode); selected != nil {
					targets = append(targets, selected)
				}
			case "lowesthpally":
				if ally := battleLowestHPAlly(state, self); ally != nil {
					targets = append(targets, ally)
				}
			default:
				targets = append(targets, self)
			}
			return targets
		}
		stopCurrentHook := false
		for _, effect := range hook.Effects {
			effectActionStart := len(actions)
			kind := battleNormalizeTrigger(fmt.Sprintf("%v", effect["kind"]))
			if kind == "" {
				kind = battleNormalizeTrigger(fmt.Sprintf("%v", effect["type"]))
			}
			if hook.SourceKind == "specialization" {
				if specializationActions, handled := s.executeBattleSpecializationHookEffect(state, now, self, source, target, trigger, incomingDamage, incomingDamageType, hook, effect, context); handled {
					actions = append(actions, specializationActions...)
					for _, specializationAction := range specializationActions {
						actionKind := battleNormalizeTrigger(fmt.Sprintf("%v", specializationAction["kind"]))
						if actionKind != "buff" && actionKind != "statedetach" {
							continue
						}
						targetID := strings.TrimSpace(fmt.Sprintf("%v", specializationAction["targetId"]))
						affected := battleUnitByID(state, targetID)
						if affected == nil {
							for _, candidate := range []*battleUnitState{self, source, target} {
								if candidate != nil && candidate.UnitID == targetID {
									affected = candidate
									break
								}
							}
						}
						if affected != nil {
							affectedUnits[affected.UnitID] = affected
						}
					}
					if len(specializationActions) > 0 {
						specializationEffectApplied = true
					}
					continue
				}
			}
			switch kind {
			case "damage":
				targetMode := battleNormalizeTrigger(fmt.Sprintf("%v", effect["targetMode"]))
				if targetMode == "" {
					targetMode = battleNormalizeTrigger(fmt.Sprintf("%v", effect["target"]))
				}
				if targetMode == "" {
					targetMode = "targeting"
				}
				targets := resolveTargets(targetMode, effect)
				if len(targets) == 0 {
					continue
				}
				damageType := normalizeBattleDamageType(fmt.Sprintf("%v", effect["damageType"]))
				profile := battleDamageChannelProfileForType(damageType)
				procChance, procChanceConfigured := battleResolveEquipmentHookProcChance(self, hook, effect)
				defaultDispatchHooks := hook.SourceKind != "specialization" || hook.DispatchHooks
				dispatchEquipmentHooks := parseBooleanLoose(firstNonNil(effect["dispatchHooks"], effect["dispatch_hooks"], effect["dispatchEquipmentHooks"], effect["dispatch_equipment_hooks"]), defaultDispatchHooks)
				relationMode := battleNormalizeTrigger(fmt.Sprintf("%v", firstNonNil(effect["relationMode"], effect["relation_mode"])))
				relationFormulas := normalizeAnyMap(firstNonNil(effect["amountFormulaByRelation"], effect["amount_formula_by_relation"]))
				simultaneous := parseBooleanLoose(effect["simultaneous"], false)
				defaultFormula := parseBattleSkillAmountFormula(firstNonNil(effect["amountFormula"], effect["amount_formula"]), nil)
				defenseK := s.battleDefenseKForState(state)
				type pendingEquipmentDamage struct {
					target            *battleUnitState
					targetID          string
					relation          string
					rawDamage         float64
					damage            float64
					probabilityCheck  battleProbabilityCheck
					probabilityResult battleProbabilityResult
				}
				batchDamage := simultaneous || len(relationFormulas) > 0
				sourceUnitID := self.UnitID
				pending := make([]pendingEquipmentDamage, 0, len(targets))
				applyResult := func(result pendingEquipmentDamage, resolveByID bool) {
					damageTarget := result.target
					damageSource := self
					if resolveByID && state != nil {
						damageSource = battleUnitByID(state, sourceUnitID)
						damageTarget = battleUnitByID(state, result.targetID)
					}
					if damageSource == nil || damageTarget == nil || !damageTarget.Alive || damageTarget.HP <= 0 {
						return
					}
					hookMeta := map[string]any{
						"sourceItemId":           hook.SourceItemID,
						"sourceEnhanceLevel":     hook.SourceEnhanceLevel,
						"hookId":                 hook.ID,
						"targetMode":             targetMode,
						"rawDamage":              roundTo4(result.rawDamage),
						"dispatchEquipmentHooks": dispatchEquipmentHooks,
					}
					if result.relation != "" {
						hookMeta["relation"] = result.relation
						hookMeta["relationMode"] = "team_side"
					}
					if simultaneous {
						hookMeta["simultaneous"] = true
					}
					if context != nil && strings.TrimSpace(context.BuffID) != "" {
						hookMeta["triggerBuffId"] = strings.TrimSpace(context.BuffID)
						hookMeta["buffApplicationKind"] = strings.TrimSpace(context.BuffApplicationKind)
					}
					if procChanceConfigured {
						hookMeta["procChance"] = roundTo4(result.probabilityResult.FinalChance)
					}
					if strings.TrimSpace(hook.SourceAffixID) != "" {
						hookMeta["sourceAffixId"] = hook.SourceAffixID
						hookMeta["sourceAffixSlot"] = hook.SourceAffixSlot
					}
					runtime := newBattleSkillActionRuntime(state, damageSource, now, s, nil)
					runtime.ApplyDamage(battleskills.DamageAction{
						SourceID:               damageSource.UnitID,
						TargetID:               damageTarget.UnitID,
						Amount:                 result.damage,
						Crit:                   false,
						DamageType:             damageType,
						SuppressEquipmentHooks: !dispatchEquipmentHooks,
						Reason:                 "equipment_hook:" + hook.ID,
						Meta:                   hookMeta,
					})
					actions = append(actions, runtime.ActionLog()...)
					if procChanceConfigured {
						actions = append(actions, s.battleFinalizeProbabilityResult(state, now, result.probabilityCheck, result.probabilityResult)...)
					}
				}
				for _, damageTarget := range targets {
					if damageTarget == nil || !damageTarget.Alive || damageTarget.HP <= 0 {
						continue
					}
					probabilityCheck := battleProbabilityCheck{}
					probabilityResult := battleProbabilityResult{}
					if procChanceConfigured {
						probabilityCheck = battleEquipmentProbabilityCheck(self, damageTarget, hook, effect, procChance)
						probabilityResult = s.resolveBattleProbability(state, probabilityCheck)
						if !probabilityResult.Established {
							continue
						}
						if !probabilityResult.Succeeded {
							actions = append(actions, s.battleFinalizeProbabilityResult(state, now, probabilityCheck, probabilityResult)...)
							continue
						}
					}
					relation := ""
					formula := defaultFormula
					if len(relationFormulas) > 0 && relationMode == "teamside" {
						relation = "enemy"
						if battleUnitTeamSide(damageTarget) == battleUnitTeamSide(self) {
							relation = "ally"
						}
						formula = parseBattleSkillAmountFormula(relationFormulas[relation], nil)
					}
					rawDamage := math.Floor(battleEvaluateAmountFormulaWithEnhance(self, 1, formula, hook.SourceEnhanceLevel))
					if rawDamage <= 0 {
						continue
					}
					damage, _ := battlecore.ComputeDamage(battlecore.DamageInput{
						Physical:   !profile.IsMagical,
						Attack:     rawDamage,
						Defense:    damageTarget.Stats[profile.DefenseStatKey],
						DefenseK:   defenseK,
						Coeff:      1,
						CritChance: 0,
						Rand:       1,
					})
					result := pendingEquipmentDamage{
						target:            damageTarget,
						targetID:          damageTarget.UnitID,
						relation:          relation,
						rawDamage:         rawDamage,
						damage:            damage,
						probabilityCheck:  probabilityCheck,
						probabilityResult: probabilityResult,
					}
					if batchDamage {
						pending = append(pending, result)
					} else {
						applyResult(result, false)
					}
				}
				for _, result := range pending {
					applyResult(result, true)
				}
			case "addbuff":
				buffID := strings.TrimSpace(fmt.Sprintf("%v", effect["buffId"]))
				if buffID == "" {
					continue
				}
				targetMode := battleNormalizeTrigger(fmt.Sprintf("%v", effect["targetMode"]))
				if targetMode == "" {
					targetMode = battleNormalizeTrigger(fmt.Sprintf("%v", effect["target"]))
				}
				targets := resolveTargets(targetMode, effect)
				if len(targets) == 0 {
					continue
				}
				durationTurns := toInt64Loose(effect["durationTurns"], 0)
				if durationTurns == 0 {
					durationTurns = s.battleBuffDurationByIDTurns(buffID)
				}
				procChance, procChanceConfigured := battleResolveEquipmentHookProcChance(self, hook, effect)
				payload := battleResolveEquipmentHookPayload(self, hook, effect)
				defaultDispatchHooks := hook.SourceKind != "specialization" || hook.DispatchHooks
				dispatchHooks := parseBooleanLoose(firstNonNil(effect["dispatchHooks"], effect["dispatch_hooks"], effect["dispatchEquipmentHooks"], effect["dispatch_equipment_hooks"]), defaultDispatchHooks)
				for _, target := range targets {
					if target == nil || !target.Alive {
						continue
					}
					probabilityCheck := battleProbabilityCheck{}
					probabilityResult := battleProbabilityResult{}
					if procChanceConfigured {
						probabilityCheck = battleEquipmentProbabilityCheck(self, target, hook, effect, procChance)
						probabilityResult = s.resolveBattleProbability(state, probabilityCheck)
						if !probabilityResult.Established {
							continue
						}
						if !probabilityResult.Succeeded {
							actions = append(actions, s.battleFinalizeProbabilityResult(state, now, probabilityCheck, probabilityResult)...)
							continue
						}
					}
					stackPolicy := "refresh_duration"
					maxStacks := 0
					if s != nil {
						stackPolicy = s.battleBuffStackPolicyByID(buffID)
						maxStacks = s.battleBuffMaxStacksByID(buffID)
					}
					isNewAttach := battleApplyOrRefreshBuffForTurn(
						target,
						buffID,
						durationTurns,
						payload,
						self.UnitID,
						stackPolicy,
						maxStacks,
						battleBuffApplicationTurnSerial(state, trigger),
					)
					affectedUnits[target.UnitID] = target
					hookMeta := map[string]any{
						"sourceItemId":       hook.SourceItemID,
						"sourceEnhanceLevel": hook.SourceEnhanceLevel,
						"hookId":             hook.ID,
						"targetMode":         targetMode,
						"dispatchHooks":      dispatchHooks,
					}
					if procChanceConfigured {
						hookMeta["procChance"] = roundTo4(probabilityResult.FinalChance)
					}
					if strings.TrimSpace(hook.SourceAffixID) != "" {
						hookMeta["sourceAffixId"] = hook.SourceAffixID
						hookMeta["sourceAffixSlot"] = hook.SourceAffixSlot
					}
					actions = append(actions, map[string]any{
						"kind":              "buff",
						"targetId":          target.UnitID,
						"sourceId":          self.UnitID,
						"buffId":            buffID,
						"buffDurationTurns": durationTurns,
						"buffPayload":       payload,
						"reason":            trigger,
						"meta":              hookMeta,
					})
					if dispatchHooks {
						afterApplyActions := s.battleTriggerAfterApplyBuffEquipmentHooks(state, now, self, target, buffID, isNewAttach)
						if len(afterApplyActions) > 0 {
							actions = append(actions, afterApplyActions...)
						}
					}
					if procChanceConfigured {
						actions = append(actions, s.battleFinalizeProbabilityResult(state, now, probabilityCheck, probabilityResult)...)
					}
				}
			case "removebuff":
				buffID := strings.TrimSpace(fmt.Sprintf("%v", firstNonNil(effect["buffId"], effect["stateId"])))
				if buffID == "" {
					continue
				}
				targetMode := battleNormalizeTrigger(fmt.Sprintf("%v", effect["targetMode"]))
				if targetMode == "" {
					targetMode = battleNormalizeTrigger(fmt.Sprintf("%v", effect["target"]))
				}
				targets := resolveTargets(targetMode, effect)
				if len(targets) == 0 {
					continue
				}
				removeOneStack := toFloat64(firstNonNil(effect["removeStack"], effect["remove_stack"], effect["stackOnly"], effect["stack_only"]), 0) > 0
				defaultDispatchHooks := hook.SourceKind != "specialization" || hook.DispatchHooks
				dispatchHooks := parseBooleanLoose(firstNonNil(effect["dispatchHooks"], effect["dispatch_hooks"], effect["dispatchEquipmentHooks"], effect["dispatch_equipment_hooks"]), defaultDispatchHooks)
				for _, target := range targets {
					if target == nil || len(target.Buffs) == 0 {
						continue
					}
					result := battleBuffDispelResult{}
					if removeOneStack {
						result.Changed = battleRemoveBuffOneStack(target, buffID)
						result.Removed = result.Changed
					} else {
						result = s.battleDispelBuff(target, buffID)
					}
					if !result.Changed {
						continue
					}
					affectedUnits[target.UnitID] = target
					hookMeta := map[string]any{
						"sourceItemId":       hook.SourceItemID,
						"sourceEnhanceLevel": hook.SourceEnhanceLevel,
						"hookId":             hook.ID,
						"targetMode":         targetMode,
						"dispatchHooks":      dispatchHooks,
					}
					if strings.TrimSpace(hook.SourceAffixID) != "" {
						hookMeta["sourceAffixId"] = hook.SourceAffixID
						hookMeta["sourceAffixSlot"] = hook.SourceAffixSlot
					}
					if removeOneStack {
						actions = append(actions, map[string]any{
							"kind":     "state_detach",
							"targetId": target.UnitID,
							"sourceId": self.UnitID,
							"buffId":   buffID,
							"reason":   trigger,
							"meta":     hookMeta,
						})
					} else {
						actions = append(actions, buildBattleBuffDispelAction(self.UnitID, target.UnitID, buffID, trigger, result, hookMeta))
					}
					if dispatchHooks && !removeOneStack && result.Removed {
						_, expiryActions := s.battleApplyBuffExpiry(target, []string{buffID})
						actions = append(actions, expiryActions...)
					}
				}
			case "removebuffbykind":
				buffKind := strings.TrimSpace(strings.ToLower(fmt.Sprintf("%v", effect["buffKind"])))
				if buffKind == "" {
					continue
				}
				targetMode := battleNormalizeTrigger(fmt.Sprintf("%v", firstNonNil(effect["targetMode"], effect["target"])))
				if (targetMode == "" || targetMode == "<nil>") && hook.SourceKind == "specialization" && target != nil {
					targetMode = "target"
				}
				targets := resolveTargets(targetMode, effect)
				selectionMode := battleNormalizeTrigger(fmt.Sprintf("%v", firstNonNil(effect["selectionMode"], effect["selection_mode"])))
				maxBuffs := maxInt(1, int(toInt64Loose(firstNonNil(effect["maxBuffs"], effect["max_buffs"]), 1)))
				dispellableOnly := parseBooleanLoose(firstNonNil(effect["dispellableOnly"], effect["dispellable_only"]), false)
				procChance, procChanceConfigured := battleResolveEquipmentHookProcChance(self, hook, effect)
				for _, target := range targets {
					if target == nil || len(target.Buffs) == 0 {
						continue
					}
					probabilityCheck := battleProbabilityCheck{}
					probabilityResult := battleProbabilityResult{}
					if procChanceConfigured {
						probabilityCheck = battleEquipmentProbabilityCheck(self, target, hook, effect, procChance)
						probabilityResult = s.resolveBattleProbability(state, probabilityCheck)
						if !probabilityResult.Established {
							continue
						}
						if !probabilityResult.Succeeded {
							actions = append(actions, s.battleFinalizeProbabilityResult(state, now, probabilityCheck, probabilityResult)...)
							continue
						}
					}
					type candidate struct {
						id     string
						remain int64
					}
					byID := map[string]int64{}
					for _, buff := range target.Buffs {
						buffID := strings.TrimSpace(strings.ToLower(buff.BuffID))
						if buffID == "" || s.battleBuffKindByID(buffID) != buffKind || (dispellableOnly && !s.battleBuffDispellableByID(buffID)) {
							continue
						}
						if current, ok := byID[buffID]; !ok || buff.RemainTurns > current {
							byID[buffID] = buff.RemainTurns
						}
					}
					candidates := make([]candidate, 0, len(byID))
					for buffID, remain := range byID {
						candidates = append(candidates, candidate{id: buffID, remain: remain})
					}
					sort.SliceStable(candidates, func(i, j int) bool {
						if selectionMode == "maxremaining" && candidates[i].remain != candidates[j].remain {
							return candidates[i].remain > candidates[j].remain
						}
						return candidates[i].id < candidates[j].id
					})
					if len(candidates) > maxBuffs {
						candidates = candidates[:maxBuffs]
					}
					removedAny := false
					for _, picked := range candidates {
						result := s.battleDispelBuff(target, picked.id)
						if !result.Changed {
							continue
						}
						removedAny = true
						affectedUnits[target.UnitID] = target
						meta := map[string]any{
							"sourceItemId":       hook.SourceItemID,
							"sourceEnhanceLevel": hook.SourceEnhanceLevel,
							"hookId":             hook.ID,
							"targetMode":         targetMode,
							"buffKind":           buffKind,
						}
						if procChanceConfigured {
							meta["procChance"] = roundTo4(probabilityResult.FinalChance)
						}
						actions = append(actions, buildBattleBuffDispelAction(self.UnitID, target.UnitID, picked.id, trigger, result, meta))
					}
					if procChanceConfigured && removedAny {
						actions = append(actions, s.battleFinalizeProbabilityResult(state, now, probabilityCheck, probabilityResult)...)
					}
				}
			case "heal":
				targetMode := battleNormalizeTrigger(fmt.Sprintf("%v", effect["targetMode"]))
				if targetMode == "" {
					targetMode = battleNormalizeTrigger(fmt.Sprintf("%v", effect["target"]))
				}
				var targets []*battleUnitState
				if targetMode == "targeting" || targetMode == "" {
					if state == nil {
						continue
					}
					targeting := parseBattleSkillTargeting(effect["targeting"], "vanguard", "front_to_rear", 1, nil, 0, false, nil)
					targets = s.battleSelectTemplateTargets(state, self, battleSkillState{Level: 1}, targeting, false, state.randFloat64)
				} else {
					targets = resolveTargets(targetMode, effect)
				}
				if len(targets) == 0 {
					continue
				}
				formula := parseBattleSkillAmountFormula(firstNonNil(effect["amountFormula"], effect["amount_formula"]), nil)
				healValue := battleEvaluateAmountFormulaWithEnhance(self, 1, formula, hook.SourceEnhanceLevel)
				if flatVal := toFloat64(effect["value"], 0); flatVal > 0 && healValue <= 0 {
					healValue = flatVal
				}
				if healValue <= 0 {
					continue
				}
				healValue, outputSources := s.battleApplyOutgoingBuffModifiers(self, "heal", healValue)
				if healValue <= 0 {
					continue
				}
				procChance, procChanceConfigured := battleResolveEquipmentHookProcChance(self, hook, effect)
				for _, target := range targets {
					if target == nil || !target.Alive {
						continue
					}
					probabilityCheck := battleProbabilityCheck{}
					probabilityResult := battleProbabilityResult{}
					if procChanceConfigured {
						probabilityCheck = battleEquipmentProbabilityCheck(self, target, hook, effect, procChance)
						probabilityResult = s.resolveBattleProbability(state, probabilityCheck)
						if !probabilityResult.Established {
							continue
						}
						if !probabilityResult.Succeeded {
							actions = append(actions, s.battleFinalizeProbabilityResult(state, now, probabilityCheck, probabilityResult)...)
							continue
						}
					}
					beforeHP := target.HP
					beforeAlive := target.Alive
					target.HP = clampFloat(target.HP+healValue, 0, target.MaxHP)
					effectiveHeal := maxFloat(0, target.HP-beforeHP)
					if target.HP > 0 {
						target.Alive = true
					}
					battleCombatTrialRecordHeal(state, self.UnitID, self, target, effectiveHeal)
					affectedUnits[target.UnitID] = target
					hookMeta := map[string]any{
						"sourceItemId":       hook.SourceItemID,
						"sourceEnhanceLevel": hook.SourceEnhanceLevel,
						"hookId":             hook.ID,
						"targetMode":         targetMode,
					}
					if len(outputSources) > 0 {
						hookMeta["outgoingBuffAdjusted"] = true
						hookMeta["outgoingBuffSources"] = outputSources
					}
					if procChanceConfigured {
						hookMeta["procChance"] = roundTo4(probabilityResult.FinalChance)
					}
					if strings.TrimSpace(hook.SourceAffixID) != "" {
						hookMeta["sourceAffixId"] = hook.SourceAffixID
						hookMeta["sourceAffixSlot"] = hook.SourceAffixSlot
					}
					actions = append(actions, map[string]any{
						"kind":        "heal",
						"targetId":    target.UnitID,
						"sourceId":    self.UnitID,
						"amount":      roundTo4(healValue),
						"reason":      trigger,
						"beforeHp":    roundTo4(beforeHP),
						"afterHp":     roundTo4(target.HP),
						"beforeAlive": beforeAlive,
						"afterAlive":  target.Alive,
						"meta":        hookMeta,
					})
					// Propagate heal to on_after_deal_heal hooks (e.g. snow_oath_codex).
					// Mirror the condition in battleSkillActionRuntime.ApplyHeal: skip self-heals.
					if state != nil && effectiveHeal > 0 && strings.TrimSpace(self.UnitID) != strings.TrimSpace(target.UnitID) {
						dealHealHooks := s.battleTriggerUnitEquipmentHooks(state, now, self, target, "on_after_deal_heal", effectiveHeal, "")
						if len(dealHealHooks) > 0 {
							actions = append(actions, dealHealHooks...)
						}
					}
					if procChanceConfigured {
						actions = append(actions, s.battleFinalizeProbabilityResult(state, now, probabilityCheck, probabilityResult)...)
					}
				}
			case "summon":
				payload := parseBattleSkillSummonPayload(firstNonNil(effect["summonPayload"], effect["summon"], effect))
				if strings.TrimSpace(payload.UnitID) == "" {
					continue
				}
				procChance, procChanceConfigured := battleResolveEquipmentHookProcChance(self, hook, effect)
				targetMode := battleNormalizeTrigger(fmt.Sprintf("%v", effect["targetMode"]))
				if targetMode == "" {
					targetMode = battleNormalizeTrigger(fmt.Sprintf("%v", effect["target"]))
				}
				if targetMode == "" {
					targetMode = "self"
				}
				targets := resolveTargets(targetMode, effect)
				if len(targets) == 0 {
					continue
				}
				for _, target := range targets {
					if target == nil {
						continue
					}
					probabilityCheck := battleProbabilityCheck{}
					probabilityResult := battleProbabilityResult{}
					if procChanceConfigured {
						probabilityCheck = battleEquipmentProbabilityCheck(self, target, hook, effect, procChance)
						probabilityResult = s.resolveBattleProbability(state, probabilityCheck)
						if !probabilityResult.Established {
							continue
						}
						if !probabilityResult.Succeeded {
							actions = append(actions, s.battleFinalizeProbabilityResult(state, now, probabilityCheck, probabilityResult)...)
							continue
						}
					}
					summonCount := int(math.Floor(battleEvaluateAmountFormulaWithEnhance(target, 1, payload.CountFormula, hook.SourceEnhanceLevel)))
					if summonCount <= 0 {
						continue
					}
					resolvedMaxCount := battleResolveSummonMaxCount(target, 1, payload, hook.SourceEnhanceLevel)
					summonedAny := false
					for summonIndex := 0; summonIndex < summonCount; summonIndex++ {
						expireActions, hasCapacity := battlePrepareSummonCapacity(
							state,
							payload.UnitID,
							target.UnitID,
							resolvedMaxCount,
							payload.OnCapOverflow,
						)
						actions = append(actions, expireActions...)
						if !hasCapacity {
							break
						}
						summon, ok := s.buildBattleSkillSummonRuntime(state, target, payload, 1, hook.SourceEnhanceLevel)
						if !ok {
							continue
						}
						if !addBattleSummon(state, target.Side, &summon, s, now) {
							continue
						}
						summonedAny = true
						hookMeta := map[string]any{
							"sourceItemId":         hook.SourceItemID,
							"sourceEnhanceLevel":   hook.SourceEnhanceLevel,
							"hookId":               hook.ID,
							"targetMode":           targetMode,
							"persistAcrossBattles": payload.PersistAcrossBattles,
							"templateUnitId":       payload.UnitID,
							"onCasterDeath":        normalizeBattleSummonOnCasterDeath(payload.OnCasterDeath),
							"onCapOverflow":        normalizeBattleSummonOnCapOverflow(payload.OnCapOverflow),
							"onDispel":             normalizeBattleSummonOnDispel(payload.OnDispel),
							"onDurationEnd":        normalizeBattleSummonOnDurationEnd(payload.OnDurationEnd),
						}
						if resolvedMaxCount > 0 {
							hookMeta["maxCountPerUnit"] = resolvedMaxCount
						}
						if procChanceConfigured {
							hookMeta["procChance"] = roundTo4(probabilityResult.FinalChance)
						}
						if strings.TrimSpace(hook.SourceAffixID) != "" {
							hookMeta["sourceAffixId"] = hook.SourceAffixID
							hookMeta["sourceAffixSlot"] = hook.SourceAffixSlot
						}
						actions = append(actions, map[string]any{
							"kind":     "summon",
							"sourceId": self.UnitID,
							"summon":   summon,
							"reason":   trigger,
							"meta":     hookMeta,
						})
					}
					if procChanceConfigured && summonedAny {
						actions = append(actions, s.battleFinalizeProbabilityResult(state, now, probabilityCheck, probabilityResult)...)
					}
				}
			case "playerclone":
				clonePayload := parseBattlePlayerClonePayload(effect)
				cloneFormulaSource := battleArenaSummonFormulaSource(state, self)
				procChance, procChanceConfigured := battleResolveEquipmentHookProcChance(self, hook, effect)
				probabilityCheck := battleProbabilityCheck{}
				probabilityResult := battleProbabilityResult{}
				if procChanceConfigured {
					probabilityCheck = battleEquipmentProbabilityCheck(self, self, hook, effect, procChance)
					probabilityResult = s.resolveBattleProbability(state, probabilityCheck)
					if !probabilityResult.Established {
						continue
					}
					if !probabilityResult.Succeeded {
						actions = append(actions, s.battleFinalizeProbabilityResult(state, now, probabilityCheck, probabilityResult)...)
						continue
					}
				}
				expireActions, hasCapacity := battlePrepareSummonCapacity(
					state,
					battlePlayerCloneTemplateID,
					self.UnitID,
					clonePayload.MaxCountPerUnit,
					clonePayload.OnCapOverflow,
				)
				actions = append(actions, expireActions...)
				if !hasCapacity {
					continue
				}
				durationTurns := clonePayload.DurationTurns
				if clonePayload.DurationFormula.BaseAdd != 0 || len(clonePayload.DurationFormula.Terms) > 0 {
					computed := int64(math.Floor(battleEvaluateAmountFormulaWithEnhance(cloneFormulaSource, 1, clonePayload.DurationFormula, hook.SourceEnhanceLevel)))
					if computed >= 1 {
						durationTurns = computed
					}
				}
				// Determine base stats based on inherit.stats
				inh := clonePayload.Inherit
				var cloneStats map[string]float64
				switch {
				case !inh.Defined || inh.Stats == "all":
					cloneStats = cloneFloatMap(self.Stats)
				case inh.Stats == "none":
					cloneStats = map[string]float64{}
				case inh.Stats == "select":
					cloneStats = map[string]float64{}
					keysSet := make(map[string]bool, len(inh.StatsKeys))
					for _, k := range inh.StatsKeys {
						keysSet[k] = true
					}
					for k, v := range self.Stats {
						if keysSet[k] {
							cloneStats[k] = v
						}
					}
				default:
					cloneStats = cloneFloatMap(self.Stats)
				}
				cloneStats = battleArenaSummonBaseStats(state, self, cloneStats)
				for attrKey, delta := range clonePayload.StatModifiers {
					attr := strings.TrimSpace(attrKey)
					if attr == "" {
						continue
					}
					if math.IsNaN(delta) || math.IsInf(delta, 0) {
						continue
					}
					cloneStats[attr] += delta
				}
				for attrKey, formula := range clonePayload.StatsFormula {
					attr := strings.TrimSpace(attrKey)
					if attr == "" {
						continue
					}
					delta := battleEvaluateAmountFormulaWithEnhance(cloneFormulaSource, 1, formula, hook.SourceEnhanceLevel)
					if math.IsNaN(delta) || math.IsInf(delta, 0) {
						continue
					}
					cloneStats[attr] += delta
				}
				cloneHP := maxFloat(1, cloneStats["hp"])
				cloneMP := maxFloat(0, cloneStats["mp"])
				cloneStats["hp"] = cloneHP
				cloneStats["mp"] = cloneMP
				cloneUnitID := battleNextSummonUnitID(state, self.UnitID, battlePlayerCloneTemplateID)
				// Skills inheritance
				var cloneSkillIDs []string
				var cloneSkills []battleSkillState
				if !inh.Defined || inh.Skills {
					for _, sk := range self.Skills {
						if store.IsEquipmentSkill(sk.ID) {
							continue
						}
						fresh := sk
						fresh.ComboOnly = false
						fresh.CooldownRemain = 0
						cloneSkillIDs = append(cloneSkillIDs, fresh.ID)
						cloneSkills = append(cloneSkills, fresh)
					}
				} else {
					defaultSkillID := s.battleDefaultBasicSkillID()
					cloneSkillIDs = []string{defaultSkillID}
					cloneSkills = []battleSkillState{s.battleSkillByID(defaultSkillID)}
				}
				// Basic attack inheritance
				var cloneBasicAttack *battleBasicAttackProfile
				if (!inh.Defined || inh.BasicAttack) && self.BasicAttack != nil {
					cp := *self.BasicAttack
					cp.Effects = append([]battleSkillEffect(nil), self.BasicAttack.Effects...)
					cloneBasicAttack = &cp
				}
				// Effect mutations inheritance
				var cloneEffectMutations []battleAffixEffectMutation
				if !inh.Defined || inh.EffectMutations {
					cloneEffectMutations = append([]battleAffixEffectMutation(nil), self.EffectMutations...)
				}
				cloneSeparator := clonePayload.NameSeparator
				if cloneSeparator == "" {
					cloneSeparator = " - "
				}
				cloneName := self.Name
				if clonePayload.NameSuffix != "" {
					cloneName = self.Name + cloneSeparator + clonePayload.NameSuffix
				}
				cloneNameKey := ""
				cloneUnit := battleUnitState{
					UnitID:                     cloneUnitID,
					Side:                       self.Side,
					OriginSide:                 self.Side,
					BaseSide:                   self.Side,
					PerspectiveSide:            self.Side,
					Name:                       cloneName,
					NameKey:                    cloneNameKey,
					Position:                   self.Position,
					BasePosition:               self.BasePosition,
					Alive:                      true,
					RewardEligible:             false,
					HP:                         cloneHP,
					MP:                         cloneMP,
					MaxHP:                      cloneHP,
					MaxMP:                      cloneMP,
					Stats:                      cloneStats,
					BaseStats:                  cloneFloatMap(cloneStats),
					StatMode:                   battleStatModeDirect,
					SkillSlots:                 append([]string(nil), self.SkillSlots...),
					SkillIDs:                   cloneSkillIDs,
					SkillRules:                 map[string]battleSkillRuleState{},
					ItemRules:                  map[string]battleSkillRuleState{},
					Skills:                     cloneSkills,
					BattleItems:                []battleItemState{},
					WeaponClasses:              append([]string(nil), self.WeaponClasses...),
					BasicAttack:                cloneBasicAttack,
					EffectMutations:            cloneEffectMutations,
					SummonRemainTurns:          maxInt64(0, durationTurns),
					SummonTemplateUnitID:       battlePlayerCloneTemplateID,
					SummonSourceUnitID:         self.UnitID,
					SummonMaxCountPerUnit:      clonePayload.MaxCountPerUnit,
					SummonPersistAcrossBattles: clonePayload.PersistAcrossBattles,
					SummonOnCasterDeath:        normalizeBattleSummonOnCasterDeath(clonePayload.OnCasterDeath),
					SummonOnCapOverflow:        normalizeBattleSummonOnCapOverflow(clonePayload.OnCapOverflow),
					SummonOnDispel:             normalizeBattleSummonOnDispel(clonePayload.OnDispel),
					SummonOnDurationEnd:        normalizeBattleSummonOnDurationEnd(clonePayload.OnDurationEnd),
					SummonSourceEnhanceLevel:   hook.SourceEnhanceLevel,
					SummonOrder:                state.Seq + 1,
					SummonNameSuffixKey:        clonePayload.NameSuffixKey,
				}
				remainTurns := durationTurns
				if remainTurns == 0 {
					remainTurns = -1
				}
				battleApplyOrRefreshBuff(&cloneUnit, battleSummonBindingBuffID, remainTurns, map[string]any{
					"templateUnitId":       battlePlayerCloneTemplateID,
					"sourceUnitId":         self.UnitID,
					"maxCountPerUnit":      cloneUnit.SummonMaxCountPerUnit,
					"persistAcrossBattles": cloneUnit.SummonPersistAcrossBattles,
					"onCasterDeath":        cloneUnit.SummonOnCasterDeath,
					"onCapOverflow":        cloneUnit.SummonOnCapOverflow,
					"onDispel":             cloneUnit.SummonOnDispel,
					"onDurationEnd":        cloneUnit.SummonOnDurationEnd,
					"sourceEnhanceLevel":   cloneUnit.SummonSourceEnhanceLevel,
					"summonOrder":          cloneUnit.SummonOrder,
				}, self.UnitID, "refresh_duration")
				appendToPlayers := normalizeBattleSide(cloneUnit.Side) != "enemy"
				if appendToPlayers {
					state.Players = append(state.Players, cloneUnit)
				} else {
					state.Enemies = append(state.Enemies, cloneUnit)
				}
				sourceUnit := battleUnitByID(state, strings.TrimSpace(self.UnitID))
				if sourceUnit == nil {
					sourceUnit = self
				}
				if appendToPlayers {
					s.battleTriggerAfterSummonHooks(state, now, sourceUnit, &state.Players[len(state.Players)-1])
				} else {
					s.battleTriggerAfterSummonHooks(state, now, sourceUnit, &state.Enemies[len(state.Enemies)-1])
				}
				state.Order = []string{}
				state.rebuildOrderIfNeeded()
				hookMeta := map[string]any{
					"sourceItemId":         hook.SourceItemID,
					"sourceEnhanceLevel":   hook.SourceEnhanceLevel,
					"hookId":               hook.ID,
					"targetMode":           "self",
					"persistAcrossBattles": clonePayload.PersistAcrossBattles,
					"templateUnitId":       battlePlayerCloneTemplateID,
					"onCasterDeath":        cloneUnit.SummonOnCasterDeath,
					"onCapOverflow":        cloneUnit.SummonOnCapOverflow,
					"onDispel":             cloneUnit.SummonOnDispel,
					"onDurationEnd":        cloneUnit.SummonOnDurationEnd,
				}
				if procChanceConfigured {
					hookMeta["procChance"] = roundTo4(probabilityResult.FinalChance)
				}
				if strings.TrimSpace(hook.SourceAffixID) != "" {
					hookMeta["sourceAffixId"] = hook.SourceAffixID
					hookMeta["sourceAffixSlot"] = hook.SourceAffixSlot
				}
				actions = append(actions, map[string]any{
					"kind":     "summon",
					"sourceId": self.UnitID,
					"summon": map[string]any{
						"unitId":             cloneUnitID,
						"side":               cloneUnit.Side,
						"name":               cloneUnit.Name,
						"nameKey":            cloneUnit.NameKey,
						"nameSuffixKey":      cloneUnit.SummonNameSuffixKey,
						"position":           cloneUnit.Position,
						"hp":                 roundTo4(cloneUnit.HP),
						"mp":                 roundTo4(cloneUnit.MP),
						"maxHp":              roundTo4(cloneUnit.MaxHP),
						"maxMp":              roundTo4(cloneUnit.MaxMP),
						"stats":              cloneFloatMap(cloneUnit.Stats),
						"durationTurns":      durationTurns,
						"templateUnitId":     battlePlayerCloneTemplateID,
						"sourceUnitId":       self.UnitID,
						"sourceEnhanceLevel": hook.SourceEnhanceLevel,
						"skillIds":           cloneUnit.SkillIDs,
					},
					"reason": trigger,
					"meta":   hookMeta,
				})
				if procChanceConfigured {
					actions = append(actions, s.battleFinalizeProbabilityResult(state, now, probabilityCheck, probabilityResult)...)
				}
			case "execute":
				// Instant-kill if target HP% < threshold. hpPercentThreshold: base (e.g. 0.10).
				// hpPercentPerEnhanceLevel: bonus per enhance level (e.g. 0.005 = +0.5%/level).
				// Defaults targetMode to "source" (the unit you just damaged in on_after_deal_damage).
				threshold := toFloat64(effect["hpPercentThreshold"], 0.10)
				perEnhance := toFloat64(effect["hpPercentPerEnhanceLevel"], 0.005)
				effectiveThreshold := threshold + perEnhance*float64(hook.SourceEnhanceLevel)
				executeTargetMode := battleNormalizeTrigger(fmt.Sprintf("%v", effect["targetMode"]))
				if executeTargetMode == "" {
					executeTargetMode = "source"
				}
				executeTargets := resolveTargets(executeTargetMode, effect)
				for _, execTarget := range executeTargets {
					if execTarget == nil || !execTarget.Alive || battleCombatTrialImmutableTarget(state, execTarget) {
						continue
					}
					hpPct := 0.0
					if execTarget.MaxHP > 0 {
						hpPct = execTarget.HP / execTarget.MaxHP
					}
					if hpPct >= effectiveThreshold {
						continue
					}
					execBeforeHP := execTarget.HP
					execTarget.HP = 0
					execTarget.Alive = false
					battleCombatTrialRecordDamage(state, self.UnitID, self, execTarget, execBeforeHP)
					battleQueueUnitDeath(state, execTarget, self)
					executeMeta := map[string]any{
						"sourceItemId":       hook.SourceItemID,
						"sourceEnhanceLevel": hook.SourceEnhanceLevel,
						"hookId":             hook.ID,
						"executeThreshold":   roundTo4(effectiveThreshold),
					}
					if strings.TrimSpace(hook.SourceAffixID) != "" {
						executeMeta["sourceAffixId"] = hook.SourceAffixID
						executeMeta["sourceAffixSlot"] = hook.SourceAffixSlot
					}
					actions = append(actions, map[string]any{
						"kind":        "damage",
						"targetId":    execTarget.UnitID,
						"sourceId":    self.UnitID,
						"amount":      roundTo4(execBeforeHP),
						"damageType":  "physical",
						"reason":      "execute",
						"beforeHp":    roundTo4(execBeforeHP),
						"afterHp":     0.0,
						"beforeAlive": true,
						"afterAlive":  false,
						"delayMs":     int64(0),
						"meta":        executeMeta,
					})
				}
			case "extraturn", "immediateaction":
				targetMode := battleNormalizeTrigger(fmt.Sprintf("%v", effect["targetMode"]))
				if targetMode == "" {
					targetMode = "self"
				}
				if targetMode != "self" {
					continue
				}
				if self == nil || !self.Alive || self.HP <= 0 {
					continue
				}
				turnMode := strings.ToLower(strings.TrimSpace(fmt.Sprintf("%v", firstNonNil(effect["turnMode"], effect["turn_mode"]))))
				if turnMode == "" || turnMode == "<nil>" {
					turnMode = "full"
				}
				defaultDispatchHooks := hook.SourceKind != "specialization" || hook.DispatchHooks
				dispatchHooks := parseBooleanLoose(firstNonNil(effect["dispatchHooks"], effect["dispatch_hooks"], effect["dispatchEquipmentHooks"], effect["dispatch_equipment_hooks"]), defaultDispatchHooks)
				abortHookOnFail := parseBooleanLoose(firstNonNil(effect["abortHookOnFail"], effect["abort_hook_on_fail"]), false)
				probabilityCheck, probabilityResult, procChanceConfigured := s.battleResolveEquipmentEffectProbability(state, self, self, hook, effect)
				if procChanceConfigured {
					if !probabilityResult.Established {
						continue
					}
					if !probabilityResult.Succeeded {
						actions = append(actions, s.battleFinalizeProbabilityResult(state, now, probabilityCheck, probabilityResult)...)
						if abortHookOnFail {
							stopCurrentHook = true
						}
						break
					}
				}
				_, ok := battleRunProbabilityExtraTurn(state, func() (map[string]any, bool) {
					return s.executeEquipmentExtraTurn(state, now, self, trigger, hook, target, turnMode, dispatchHooks)
				})
				if !ok {
					if abortHookOnFail {
						stopCurrentHook = true
						break
					}
					continue
				}
				hookMeta := map[string]any{
					"sourceItemId":       hook.SourceItemID,
					"sourceEnhanceLevel": hook.SourceEnhanceLevel,
					"hookId":             hook.ID,
					"targetMode":         targetMode,
					"turnMode":           turnMode,
				}
				if target != nil {
					hookMeta["eventTargetId"] = target.UnitID
					hookMeta["eventTargetType"] = battleUnitFilterType(target)
				}
				if procChanceConfigured {
					hookMeta["procChance"] = roundTo4(probabilityResult.FinalChance)
				}
				if strings.TrimSpace(hook.SourceAffixID) != "" {
					hookMeta["sourceAffixId"] = hook.SourceAffixID
					hookMeta["sourceAffixSlot"] = hook.SourceAffixSlot
				}
				actions = append(actions, map[string]any{
					"kind":     "extra_turn",
					"targetId": self.UnitID,
					"sourceId": self.UnitID,
					"reason":   trigger,
					"queued":   true,
					"turnMode": turnMode,
					"meta":     hookMeta,
				})
				if hook.SourceKind == "specialization" {
					specializationEffectApplied = true
				}
				if procChanceConfigured {
					actions = append(actions, s.battleFinalizeProbabilityResult(state, now, probabilityCheck, probabilityResult)...)
				}
			case "adjustskillcooldown":
				amount := int(toInt64Loose(firstNonNil(effect["amount"], effect["turns"], effect["value"]), 0))
				if amount == 0 {
					continue
				}
				targetMode := battleNormalizeTrigger(fmt.Sprintf("%v", firstNonNil(effect["targetMode"], effect["target"])))
				if targetMode == "" {
					targetMode = "self"
				}
				selector := battleParseSkillCooldownSelector(effect)
				if selector.SelectionMode == "context_skill" {
					if context == nil || strings.TrimSpace(context.SkillID) == "" {
						continue
					}
					if len(selector.SkillIDs) > 0 && !battleStringSliceContainsFold(selector.SkillIDs, context.SkillID) {
						continue
					}
					selector.SkillIDs = []string{context.SkillID}
					selector.MaxSkills = 1
				}
				for _, cooldownTarget := range resolveTargets(targetMode, effect) {
					action, changed := s.battleAdjustUnitSkillCooldownsWithSelector(
						cooldownTarget,
						amount,
						selector,
						self.UnitID,
						trigger,
						0,
						map[string]any{
							"sourceItemId":       hook.SourceItemID,
							"sourceEnhanceLevel": hook.SourceEnhanceLevel,
							"hookId":             hook.ID,
							"targetMode":         targetMode,
						},
					)
					if changed {
						actions = append(actions, action)
					}
				}
			case "transferskillcooldown":
				if context == nil || strings.TrimSpace(context.SkillID) == "" {
					continue
				}
				targetMode := battleNormalizeTrigger(fmt.Sprintf("%v", firstNonNil(effect["targetMode"], effect["target"])))
				if targetMode == "" {
					targetMode = "self"
				}
				if targetMode != "self" {
					continue
				}
				action, consumedWaiverBuffID, changed := s.battleTransferSkillCooldown(
					self,
					context.SkillID,
					effect,
					self.UnitID,
					trigger,
					map[string]any{
						"sourceItemId":       hook.SourceItemID,
						"sourceEnhanceLevel": hook.SourceEnhanceLevel,
						"hookId":             hook.ID,
						"targetMode":         targetMode,
						"castKind":           context.CastKind,
					},
				)
				if !changed {
					continue
				}
				actions = append(actions, action)
				if consumedWaiverBuffID != "" {
					affectedUnits[self.UnitID] = self
					actions = append(actions, map[string]any{
						"kind":     "state_detach",
						"targetId": self.UnitID,
						"sourceId": self.UnitID,
						"buffId":   consumedWaiverBuffID,
						"reason":   trigger,
						"meta": map[string]any{
							"sourceItemId":       hook.SourceItemID,
							"sourceEnhanceLevel": hook.SourceEnhanceLevel,
							"hookId":             hook.ID,
						},
					})
				}
			case "consumemp":
				formula := parseBattleSkillAmountFormula(firstNonNil(effect["amountFormula"], effect["amount_formula"]), nil)
				amount := battleEvaluateAmountFormulaWithEnhance(self, 1, formula, hook.SourceEnhanceLevel)
				if flatVal := toFloat64(effect["value"], 0); flatVal > 0 && amount <= 0 {
					amount = flatVal
				}
				if amount <= 0 {
					continue
				}
				requireFullAmount := parseBooleanLoose(firstNonNil(effect["requireFullAmount"], effect["require_full_amount"], effect["strict"]), false)
				abortHookOnFail := parseBooleanLoose(firstNonNil(effect["abortHookOnFail"], effect["abort_hook_on_fail"]), false)
				if requireFullAmount && self.MP < amount {
					if abortHookOnFail {
						stopCurrentHook = true
						break
					}
					continue
				}
				beforeMP := self.MP
				self.MP = maxFloat(0, self.MP-amount)
				consumeMpMeta := map[string]any{
					"sourceItemId":       hook.SourceItemID,
					"sourceEnhanceLevel": hook.SourceEnhanceLevel,
					"hookId":             hook.ID,
				}
				if strings.TrimSpace(hook.SourceAffixID) != "" {
					consumeMpMeta["sourceAffixId"] = hook.SourceAffixID
					consumeMpMeta["sourceAffixSlot"] = hook.SourceAffixSlot
				}
				actions = append(actions, map[string]any{
					"kind":     "resource",
					"resource": "mp",
					"targetId": self.UnitID,
					"sourceId": self.UnitID,
					"amount":   roundTo4(self.MP - beforeMP),
					"reason":   trigger,
					"beforeMp": roundTo4(beforeMP),
					"afterMp":  roundTo4(self.MP),
					"delayMs":  int64(0),
					"meta":     consumeMpMeta,
				})
			}
			if hook.SourceKind == "specialization" && kind != "extraturn" && kind != "immediateaction" && len(actions) > effectActionStart {
				specializationEffectApplied = true
			}
			if stopCurrentHook {
				break
			}
		}
		if hook.SourceKind == "specialization" {
			for actionIndex := hookActionStart; actionIndex < len(actions); actionIndex++ {
				enrichBattleSpecializationAction(actions[actionIndex], hook)
			}
			// Failed mirrors, debt casts, and other conditional settlements must not
			// consume a trigger limit. Every successful specialization effect emits
			// at least one authoritative action row.
			if specializationEffectApplied {
				battleMarkEquipmentHookTriggered(state, self, hook)
			}
		} else if hook.SourceKind == "equipment" {
			for actionIndex := hookActionStart; actionIndex < len(actions); actionIndex++ {
				enrichBattleEquipmentInstanceAction(actions[actionIndex], hook)
			}
		}
		leaveContribution()
	}
	if len(actions) == 0 {
		return nil
	}
	for _, unit := range affectedUnits {
		if unit == nil {
			continue
		}
		beforeStats := cloneFloatMap(unit.Stats)
		beforeMaxHP := unit.MaxHP
		beforeMaxMP := unit.MaxMP
		beforePosition := unit.Position
		beforeSide := normalizeBattleSide(unit.Side)
		beforePerspective := battleUnitPerspectiveSide(unit)
		s.battleRecomputeUnitStats(unit)
		afterSide := normalizeBattleSide(unit.Side)
		afterPerspective := battleUnitPerspectiveSide(unit)
		if battleFloatMapEqual(beforeStats, unit.Stats) && beforeMaxHP == unit.MaxHP && beforeMaxMP == unit.MaxMP && beforePosition == unit.Position && beforeSide == afterSide && beforePerspective == afterPerspective {
			continue
		}
		actions = append(actions, map[string]any{
			"kind":            "stat_recalc",
			"targetId":        unit.UnitID,
			"sourceId":        self.UnitID,
			"reason":          trigger,
			"stats":           cloneFloatMap(unit.Stats),
			"baseStats":       cloneFloatMap(unit.BaseStats),
			"deltaStats":      cloneFloatMap(unit.StatModifiers),
			"statModifiers":   cloneFloatMap(unit.StatModifiers),
			"statSources":     cloneNestedFloatMap(unit.StatSources),
			"buffs":           battlePublicBuffSlice(unit.Buffs),
			"maxHp":           roundTo4(unit.MaxHP),
			"maxMp":           roundTo4(unit.MaxMP),
			"fromPosition":    beforePosition,
			"position":        unit.Position,
			"basePosition":    unit.BasePosition,
			"side":            unit.Side,
			"baseSide":        unit.BaseSide,
			"perspectiveSide": unit.PerspectiveSide,
			"meta": fiber.Map{
				"trigger": trigger,
				"ts":      now.UnixMilli(),
			},
		})
	}
	return actions
}

func battleSelectSpecializationTarget(state *battleRuntimeState, candidates []*battleUnitState, selectionMode string) *battleUnitState {
	if len(candidates) == 0 {
		return nil
	}
	selectionMode = battleNormalizeTrigger(selectionMode)
	if selectionMode == "" || selectionMode == "nil" {
		selectionMode = "highestattack"
	}
	if selectionMode == "random" {
		// Normalize the candidate order before consuming battle RNG so replay output
		// does not depend on the backing player/enemy slice order.
		sort.SliceStable(candidates, func(i, j int) bool {
			return candidates[i].UnitID < candidates[j].UnitID
		})
		if state == nil {
			return candidates[0]
		}
		return candidates[state.randInt(len(candidates))]
	}
	if selectionMode != "highestattack" && selectionMode != "highestthreat" && selectionMode != "lowesthp" {
		return nil
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		left := candidates[i].Stats["attack"]
		right := candidates[j].Stats["attack"]
		switch selectionMode {
		case "highestthreat":
			left += candidates[i].Stats["magic_attack"] + candidates[i].MaxHP*0.05
			right += candidates[j].Stats["magic_attack"] + candidates[j].MaxHP*0.05
		case "lowesthp":
			left = candidates[i].HP
			right = candidates[j].HP
		}
		if left == right {
			return candidates[i].UnitID < candidates[j].UnitID
		}
		if selectionMode == "lowesthp" {
			return left < right
		}
		return left > right
	})
	return candidates[0]
}

func (s *Server) executeBattleSpecializationHookEffect(
	state *battleRuntimeState,
	now time.Time,
	self, source, target *battleUnitState,
	trigger string,
	incomingDamage float64,
	incomingDamageType string,
	hook battleEquipmentHookRule,
	effect map[string]any,
	context *battleEquipmentHookContext,
) ([]map[string]any, bool) {
	kind := battleNormalizeTrigger(fmt.Sprintf("%v", firstNonNil(effect["kind"], effect["type"])))
	meta := battleSpecializationSourceMeta(hook, map[string]any{"specialization": true})
	action := func(kind, targetID string, extra map[string]any) map[string]any {
		row := map[string]any{
			"kind": kind, "targetId": targetID, "sourceId": self.UnitID,
			"reason": trigger, "delayMs": int64(0), "meta": meta,
		}
		for key, value := range extra {
			row[key] = value
		}
		return row
	}
	resolveEventTargets := func(mode string) []*battleUnitState {
		switch battleNormalizeTrigger(mode) {
		case "source":
			if source != nil {
				return []*battleUnitState{source}
			}
		case "target", "targeting":
			if target != nil {
				return []*battleUnitState{target}
			}
		case "lowesthpally":
			if ally := battleLowestHPAlly(state, self); ally != nil {
				return []*battleUnitState{ally}
			}
		case "enemy", "markedenemy":
			candidates := s.battleTemplateTargetsByScope(state, self, "enemy")
			if battleNormalizeTrigger(mode) == "markedenemy" {
				marked := candidates[:0]
				for _, candidate := range candidates {
					if battleUnitHasAnySpecializationMark(candidate) {
						marked = append(marked, candidate)
					}
				}
				candidates = marked
			}
			if selected := battleSelectSpecializationTarget(state, candidates, "highest_attack"); selected != nil {
				return []*battleUnitState{selected}
			}
		default:
			if self != nil {
				return []*battleUnitState{self}
			}
		}
		return nil
	}
	resolveEventTarget := func(mode string) *battleUnitState {
		targets := resolveEventTargets(mode)
		if len(targets) == 0 {
			return nil
		}
		return targets[0]
	}
	applyMark := func(markTarget *battleUnitState, markID string, duration int64) []map[string]any {
		if markTarget == nil || strings.TrimSpace(markID) == "" {
			return nil
		}
		if duration == 0 {
			duration = -1
		}
		buffID := battleSpecializationMarkBuffID(markID)
		payload := battleSpecializationSourceMeta(hook, map[string]any{"markId": markID})
		battleApplyOrRefreshBuffForTurn(markTarget, buffID, duration, payload, self.UnitID, "refresh_duration", 0, battleBuffApplicationTurnSerial(state, trigger))
		return []map[string]any{action("buff", markTarget.UnitID, map[string]any{"buffId": buffID, "buffDurationTurns": duration})}
	}
	applyBuff := func(buffTarget *battleUnitState, buffID string, duration int64) ([]map[string]any, bool) {
		buffID = strings.TrimSpace(buffID)
		if buffTarget == nil || buffID == "" {
			return nil, false
		}
		if duration == 0 {
			duration = s.battleBuffDurationByIDTurns(buffID)
		}
		stackPolicy := s.battleBuffStackPolicyByID(buffID)
		maxStacks := s.battleBuffMaxStacksByID(buffID)
		payload := battleSpecializationSourceMeta(hook, map[string]any{"specializationBuff": true})
		isNewAttach := battleApplyOrRefreshBuffForTurn(buffTarget, buffID, duration, payload, self.UnitID, stackPolicy, maxStacks, battleBuffApplicationTurnSerial(state, trigger))
		return []map[string]any{action("buff", buffTarget.UnitID, map[string]any{"buffId": buffID, "buffDurationTurns": duration, "buffPayload": payload})}, isNewAttach
	}
	switch kind {
	case "actionmetergain":
		value := clampFloat(battleSpecializationEffectScalar(self, effect, "value"), 0, 1)
		if value <= 0 {
			return nil, true
		}
		buffID := "specialization:action_meter"
		before := 0.0
		if buff := findBuffByID(self, buffID); buff != nil {
			before = toFloat64(buff.Payload["value"], 0)
		}
		after := before + value
		wholeTurns := int(math.Floor(after))
		after -= float64(wholeTurns)
		if after > 0 {
			battleApplyOrRefreshBuff(self, buffID, -1, map[string]any{"value": after}, self.UnitID, "refresh_duration")
		} else {
			battleRemoveBuff(self, buffID)
		}
		rows := []map[string]any{action("action_meter", self.UnitID, map[string]any{"amount": roundTo4(value), "before": roundTo4(before), "after": roundTo4(after)})}
		for index := 0; index < wholeTurns; index++ {
			_, ok := battleRunProbabilityExtraTurn(state, func() (map[string]any, bool) {
				return s.executeEquipmentExtraTurn(state, now, self, trigger, hook, target, "full", hook.DispatchHooks)
			})
			if !ok {
				break
			}
			rows = append(rows, action("extra_turn", self.UnitID, map[string]any{"queued": true, "turnMode": "full"}))
		}
		return rows, true
	case "cyclecounter":
		counterID := strings.TrimSpace(fmt.Sprintf("%v", firstNonNil(effect["counterId"], effect["counter_id"])))
		gain := maxInt(0, int(toInt64Loose(effect["gain"], 1)))
		maxStacks := maxInt(1, int(toInt64Loose(firstNonNil(effect["maxStacks"], effect["max_stacks"]), 1)))
		before := battleSpecializationCounterStacks(self, counterID)
		after := minInt(maxStacks, before+gain)
		battleSetSpecializationCounter(self, counterID, after)
		rows := []map[string]any{action("specialization_counter", self.UnitID, map[string]any{"counterId": counterID, "before": before, "after": after})}
		if after >= maxStacks {
			atMax := normalizeAnyMap(firstNonNil(effect["atMax"], effect["at_max"]))
			consume := maxInt(0, int(toInt64Loose(firstNonNil(atMax["consumeStacks"], atMax["consume_stacks"]), 0)))
			if consume > 0 {
				battleSetSpecializationCounter(self, counterID, maxInt(0, after-consume))
			}
			nestedKind := battleNormalizeTrigger(fmt.Sprintf("%v", firstNonNil(atMax["kind"], atMax["type"])))
			switch nestedKind {
			case "empowernexthit":
				buffID := "specialization:empower:" + strings.ToLower(counterID)
				value := maxFloat(0, battleSpecializationEffectScalar(self, atMax, "damageAmp", "damage_amp"))
				battleApplyOrRefreshBuff(self, buffID, -1, battleSpecializationSourceMeta(hook, map[string]any{"damageAmp": value}), self.UnitID, "refresh_duration")
				rows = append(rows, action("buff", self.UnitID, map[string]any{"buffId": buffID, "buffDurationTurns": int64(-1)}))
			case "actionmetergain":
				nested := cloneAnyMap(atMax)
				nested["kind"] = "action_meter_gain"
				nestedRows, _ := s.executeBattleSpecializationHookEffect(state, now, self, source, target, trigger, incomingDamage, incomingDamageType, hook, nested, context)
				rows = append(rows, nestedRows...)
			case "extraturn", "immediateaction":
				targetMode := battleNormalizeTrigger(fmt.Sprintf("%v", firstNonNil(atMax["targetMode"], atMax["target_mode"])))
				if targetMode == "" || targetMode == "nil" {
					targetMode = "self"
				}
				turnMode := strings.ToLower(strings.TrimSpace(fmt.Sprintf("%v", firstNonNil(atMax["turnMode"], atMax["turn_mode"]))))
				if turnMode == "" || turnMode == "<nil>" {
					turnMode = "full"
				}
				if targetMode == "self" && turnMode == "full" {
					dispatchHooks := parseBooleanLoose(firstNonNil(atMax["dispatchHooks"], atMax["dispatch_hooks"]), hook.DispatchHooks)
					_, ok := battleRunProbabilityExtraTurn(state, func() (map[string]any, bool) {
						return s.executeEquipmentExtraTurn(state, now, self, trigger, hook, target, turnMode, dispatchHooks)
					})
					if ok {
						rows = append(rows, action("extra_turn", self.UnitID, map[string]any{"queued": true, "turnMode": turnMode}))
					}
				}
			case "restoremp":
				restore := cloneAnyMap(atMax)
				restore["kind"] = "restore_mp"
				restore["maxMpRatio"] = firstNonNil(atMax["mpRatio"], atMax["mp_ratio"])
				restoreRows, _ := s.executeBattleSpecializationHookEffect(state, now, self, source, target, trigger, incomingDamage, incomingDamageType, hook, restore, context)
				rows = append(rows, restoreRows...)
			case "restorempandtickcooldown":
				restore := cloneAnyMap(atMax)
				restore["kind"] = "restore_mp"
				restore["maxMpRatio"] = firstNonNil(atMax["mpRatio"], atMax["mp_ratio"])
				restoreRows, _ := s.executeBattleSpecializationHookEffect(state, now, self, source, target, trigger, incomingDamage, incomingDamageType, hook, restore, context)
				rows = append(rows, restoreRows...)
				selector := map[string]any{"amount": atMax["cooldownAmount"], "selectionMode": "max_remaining", "maxSkills": 1}
				amount := int(toInt64Loose(selector["amount"], 0))
				if amount != 0 {
					cooldownAction, changed := s.battleAdjustUnitSkillCooldownsWithSelector(
						self,
						amount,
						battleParseSkillCooldownSelector(selector),
						self.UnitID,
						trigger,
						0,
						battleSpecializationSourceMeta(hook, map[string]any{"targetMode": "self"}),
					)
					if changed {
						rows = append(rows, cooldownAction)
					}
				}
			case "guardlowesthpally":
				ally := battleLowestHPAlly(state, self)
				if ally != nil {
					buffID := "specialization:damage_reduction:" + hook.ID
					duration := toInt64Loose(atMax["durationTurns"], 1)
					value := clampFloat(battleSpecializationEffectScalar(self, atMax, "damageReduction", "damage_reduction"), 0, 1)
					battleApplyOrRefreshBuffForTurn(ally, buffID, duration, battleSpecializationSourceMeta(hook, map[string]any{"value": value}), self.UnitID, "refresh_duration", 0, battleBuffApplicationTurnSerial(state, trigger))
					rows = append(rows, action("buff", ally.UnitID, map[string]any{"buffId": buffID, "buffDurationTurns": duration}))
				}
			}
		}
		return rows, true
	case "consumemark":
		markTarget := target
		markID := strings.TrimSpace(fmt.Sprintf("%v", firstNonNil(effect["markId"], effect["mark_id"])))
		buffID := battleSpecializationMarkBuffID(markID)
		if markTarget != nil && battleRemoveBuff(markTarget, buffID) {
			return []map[string]any{action("state_detach", markTarget.UnitID, map[string]any{"buffId": buffID})}, true
		}
		return nil, true
	case "markcycle":
		if target == nil {
			return nil, true
		}
		markID := strings.TrimSpace(fmt.Sprintf("%v", firstNonNil(effect["markId"], effect["mark_id"])))
		if context != nil && context.ConsumedMarks[strings.ToLower(markID)] {
			return nil, true
		}
		if !battleHasBuff(target, battleSpecializationMarkBuffID(markID)) {
			return applyMark(target, markID, -1), true
		}
		return nil, true
	case "buffcycle":
		if target == nil {
			return nil, true
		}
		buffID := strings.TrimSpace(fmt.Sprintf("%v", firstNonNil(effect["buffId"], effect["buff_id"])))
		if context != nil && context.ConsumedBuffs[strings.ToLower(buffID)] {
			return nil, true
		}
		if !battleHasBuff(target, buffID) {
			rows, _ := applyBuff(target, buffID, 0)
			return rows, true
		}
		return nil, true
	case "addbuff":
		buffID := strings.TrimSpace(fmt.Sprintf("%v", effect["buffId"]))
		mode := strings.TrimSpace(fmt.Sprintf("%v", firstNonNil(effect["targetMode"], effect["target_mode"])))
		if mode == "" || mode == "<nil>" {
			mode = "self"
		}
		targets := resolveEventTargets(mode)
		if battleNormalizeTrigger(mode) == "enemy" {
			candidates := s.battleTemplateTargetsByScope(state, self, "enemy")
			selectionMode := fmt.Sprintf("%v", firstNonNil(effect["selectionMode"], effect["selection_mode"]))
			if selected := battleSelectSpecializationTarget(state, candidates, selectionMode); selected != nil {
				targets = []*battleUnitState{selected}
			}
		}
		duration := toInt64Loose(firstNonNil(effect["durationTurns"], effect["duration_turns"]), 0)
		dispatchHooks := parseBooleanLoose(firstNonNil(effect["dispatchHooks"], effect["dispatch_hooks"]), hook.DispatchHooks)
		rows := make([]map[string]any, 0, len(targets)+1)
		for _, buffTarget := range targets {
			appliedRows, isNewAttach := applyBuff(buffTarget, buffID, duration)
			rows = append(rows, appliedRows...)
			if dispatchHooks && buffTarget != nil {
				rows = append(rows, s.battleTriggerAfterApplyBuffEquipmentHooks(state, now, self, buffTarget, buffID, isNewAttach)...)
			}
		}
		return rows, true
	case "removebuff":
		buffID := strings.TrimSpace(fmt.Sprintf("%v", effect["buffId"]))
		mode := strings.TrimSpace(fmt.Sprintf("%v", firstNonNil(effect["targetMode"], effect["target_mode"])))
		if mode == "" || mode == "<nil>" {
			mode = "self"
		}
		rows := make([]map[string]any, 0, 1)
		for _, buffTarget := range resolveEventTargets(mode) {
			if buffTarget == nil {
				continue
			}
			result := s.battleDispelBuff(buffTarget, buffID)
			if !result.Changed {
				continue
			}
			rows = append(rows, buildBattleBuffDispelAction(self.UnitID, buffTarget.UnitID, buffID, trigger, result, meta))
			if result.Removed && parseBooleanLoose(firstNonNil(effect["dispatchHooks"], effect["dispatch_hooks"]), hook.DispatchHooks) {
				_, expiryRows := s.battleApplyBuffExpiry(buffTarget, []string{buffID})
				rows = append(rows, expiryRows...)
			}
		}
		return rows, true
	case "marktarget", "designateprioritytarget":
		mode := strings.TrimSpace(fmt.Sprintf("%v", firstNonNil(effect["targetMode"], effect["target_mode"])))
		if (mode == "" || mode == "<nil>") && firstNonNil(effect["selectionMode"], effect["selection_mode"]) != nil {
			mode = "enemy"
		}
		markTarget := resolveEventTarget(mode)
		targetMode := battleNormalizeTrigger(mode)
		if targetMode == "enemy" || targetMode == "markedenemy" || kind == "designateprioritytarget" {
			candidates := s.battleTemplateTargetsByScope(state, self, "enemy")
			if targetMode == "markedenemy" {
				marked := candidates[:0]
				for _, candidate := range candidates {
					if battleUnitHasAnySpecializationMark(candidate) {
						marked = append(marked, candidate)
					}
				}
				candidates = marked
			}
			selectionMode := battleNormalizeTrigger(fmt.Sprintf("%v", firstNonNil(effect["selectionMode"], effect["selection_mode"])))
			markTarget = battleSelectSpecializationTarget(state, candidates, selectionMode)
		}
		markID := fmt.Sprintf("%v", firstNonNil(effect["markId"], effect["mark_id"]))
		rows := applyMark(markTarget, markID, toInt64Loose(firstNonNil(effect["durationTurns"], effect["duration_turns"]), -1))
		if markTarget != nil {
			rewrite := normalizeAnyMap(firstNonNil(effect["rewriteTargeting"], effect["rewrite_targeting"]))
			if buff := findBuffByID(markTarget, battleSpecializationMarkBuffID(markID)); buff != nil && len(rewrite) > 0 {
				buff.Payload["preferMarked"] = parseBooleanLoose(firstNonNil(rewrite["preferMarked"], rewrite["prefer_marked"]), false)
				buff.Payload["targetingFallback"] = strings.TrimSpace(fmt.Sprintf("%v", rewrite["fallback"]))
			}
		}
		return rows, true
	case "restoremp":
		amount := 0.0
		if battleSpecializationHasValueFormula(effect) {
			amount = self.MaxMP * clampFloat(battleSpecializationEffectScalar(self, effect), 0, 1)
		} else {
			amount = toFloat64(effect["value"], 0)
			amount += self.MaxMP * toFloat64(firstNonNil(effect["maxMpRatio"], effect["max_mp_ratio"]), 0)
		}
		before := self.MP
		self.MP = clampFloat(self.MP+amount, 0, self.MaxMP)
		return []map[string]any{action("resource", self.UnitID, map[string]any{"resource": "mp", "amount": roundTo4(self.MP - before), "beforeMp": roundTo4(before), "afterMp": roundTo4(self.MP)})}, true
	case "heal":
		healTarget := resolveEventTarget(fmt.Sprintf("%v", firstNonNil(effect["targetMode"], effect["target_mode"])))
		if healTarget == nil {
			return nil, true
		}
		amount := 0.0
		if battleSpecializationHasValueFormula(effect) {
			value := battleSpecializationEffectScalar(self, effect)
			if firstNonNil(effect["missingHpRatio"], effect["missing_hp_ratio"]) != nil {
				amount = maxFloat(0, healTarget.MaxHP-healTarget.HP) * clampFloat(value, 0, 1)
			} else if firstNonNil(effect["maxHpRatio"], effect["max_hp_ratio"]) != nil {
				amount = healTarget.MaxHP * clampFloat(value, 0, 1)
			} else {
				amount = value
			}
		} else {
			amount = battleEvaluateAmountFormulaWithEnhance(self, 1, parseBattleSkillAmountFormula(firstNonNil(effect["amountFormula"], effect["amount_formula"]), nil), 0)
			amount += healTarget.MaxHP * toFloat64(firstNonNil(effect["maxHpRatio"], effect["max_hp_ratio"]), 0)
			amount += maxFloat(0, healTarget.MaxHP-healTarget.HP) * toFloat64(firstNonNil(effect["missingHpRatio"], effect["missing_hp_ratio"]), 0)
		}
		if amount <= 0 {
			return nil, true
		}
		runtime := newBattleSkillActionRuntime(state, self, now, s, nil)
		runtime.ApplyHeal(battleskills.HealAction{SourceID: self.UnitID, TargetID: healTarget.UnitID, Amount: amount, Reason: "specialization_hook:" + hook.ID, Meta: meta})
		return runtime.ActionLog(), true
	case "retaliate":
		damageTarget := source
		if damageTarget == nil {
			return nil, true
		}
		amount := self.Stats["strength"]*toFloat64(firstNonNil(effect["strengthRatio"], effect["strength_ratio"]), 0) + self.Stats["stamina"]*toFloat64(firstNonNil(effect["staminaRatio"], effect["stamina_ratio"]), 0)
		dispatchHooks := parseBooleanLoose(firstNonNil(effect["dispatchHooks"], effect["dispatch_hooks"], effect["dispatchEquipmentHooks"], effect["dispatch_equipment_hooks"]), hook.DispatchHooks)
		return s.battleApplySpecializationDamage(state, now, self, damageTarget, amount, "physical", hook, "retaliate", dispatchHooks), true
	case "repeatcontexteffect":
		repeatTarget := resolveEventTarget(fmt.Sprintf("%v", firstNonNil(effect["targetMode"], effect["target_mode"])))
		if repeatTarget == nil || incomingDamage <= 0 {
			return nil, true
		}
		dispatchHooks := parseBooleanLoose(firstNonNil(effect["dispatchHooks"], effect["dispatch_hooks"], effect["dispatchEquipmentHooks"], effect["dispatch_equipment_hooks"]), hook.DispatchHooks)
		return s.battleApplySpecializationDamage(state, now, self, repeatTarget, incomingDamage*maxFloat(0, battleSpecializationEffectScalar(self, effect, "amountRatio", "amount_ratio")), incomingDamageType, hook, "repeat_context_effect", dispatchHooks), true
	case "resourcedebtcast":
		if context == nil || !context.MPInsufficient {
			return nil, true
		}
		minimumHP := maxFloat(0, toFloat64(firstNonNil(effect["minimumHp"], effect["minimum_hp"]), 1))
		cost := math.Floor(self.MaxHP * clampFloat(battleSpecializationEffectScalar(self, effect, "hpCostRatio", "hp_cost_ratio"), 0, 1))
		if cost <= 0 || self.HP-cost-context.HPCost < minimumHP {
			return nil, true
		}
		before := self.HP
		self.HP -= cost
		context.AllowDebtCast = true
		context.CooldownDelta += int(toInt64Loose(firstNonNil(effect["cooldownDelta"], effect["cooldown_delta"]), 0))
		return []map[string]any{action("resource", self.UnitID, map[string]any{"resource": "hp", "amount": roundTo4(-cost), "beforeHp": roundTo4(before), "afterHp": roundTo4(self.HP)})}, true
	case "shiftposition":
		ally := battleLowestHPAlly(state, self)
		if ally == nil {
			return nil, true
		}
		before := normalizeBattleTargetPosition(self.Position)
		from, toward := battlePositionOrdinal(before), battlePositionOrdinal(ally.Position)
		maxSteps := maxInt(1, int(toInt64Loose(firstNonNil(effect["maxSteps"], effect["max_steps"]), 1)))
		if from < toward {
			from = minInt(toward, from+maxSteps)
		} else if from > toward {
			from = maxInt(toward, from-maxSteps)
		}
		positions := []string{"vanguard", "midguard", "rearguard"}
		if from >= 0 && from < len(positions) {
			self.Position = positions[from]
		}
		return []map[string]any{action("position_shift", self.UnitID, map[string]any{"fromPosition": before, "position": self.Position})}, true
	case "shareddamagereduction":
		value := clampFloat(battleSpecializationEffectScalar(self, effect, "value"), 0, 1)
		duration := toInt64Loose(firstNonNil(effect["durationTurns"], effect["duration_turns"]), 1)
		units := []*battleUnitState{self}
		if ally := battleAdjacentLowestHPAlly(state, self); ally != nil {
			units = append(units, ally)
		}
		rows := make([]map[string]any, 0, len(units))
		for _, unit := range units {
			buffID := "specialization:damage_reduction:" + hook.ID
			battleApplyOrRefreshBuffForTurn(unit, buffID, duration, battleSpecializationSourceMeta(hook, map[string]any{"value": value}), self.UnitID, "refresh_duration", 0, battleBuffApplicationTurnSerial(state, trigger))
			rows = append(rows, action("buff", unit.UnitID, map[string]any{"buffId": buffID, "buffDurationTurns": duration}))
		}
		return rows, true
	case "mirrorcastskill":
		if context == nil || strings.TrimSpace(context.SkillID) == "" {
			return nil, true
		}
		sourceSkill := battleMirrorSourceSkill(self, context.SkillID)
		if strings.TrimSpace(sourceSkill.Category) == "" {
			sourceSkill.Category = strings.TrimSpace(context.SkillCategory)
		}
		if !battleMirrorSkillFilterMatches(sourceSkill, effect) {
			return nil, true
		}
		castKind := battleNormalizeTrigger(context.CastKind)
		if (castKind == "mirrorcast" || castKind == "specializationmirrorcast") &&
			!parseBooleanLoose(firstNonNil(effect["allowMirroredCastTrigger"], effect["allow_mirrored_cast_trigger"]), false) {
			return nil, true
		}
		payload, ok := s.executeMirrorCastSkill(state, now, self, battleUnitTeamSide(self), context.SkillID, effect, nil, state.Seq, hook)
		if !ok {
			return nil, true
		}
		return []map[string]any{action("mirror_cast", self.UnitID, map[string]any{"turn": payload})}, true
	case "castskill":
		skillID := strings.TrimSpace(fmt.Sprintf("%v", effect["skillId"]))
		payload, ok := s.executeMirrorCastSkill(state, now, self, battleUnitTeamSide(self), skillID, effect, nil, state.Seq, hook)
		if !ok {
			return nil, true
		}
		return []map[string]any{action("cast_skill", self.UnitID, map[string]any{"turn": payload})}, true
	case "adjustskillcooldown", "damage", "extraturn", "removebuffbykind":
		return nil, false
	case "consumecounterfordamageamp", "flatdefensepenetration", "preventlethal", "redirectdamage", "resourceshield", "runtimedamageamp":
		// These settle in the pre-damage adjustment path where they can change
		// the authoritative damage amount before HP is committed.
		return nil, true
	default:
		return nil, false
	}
}

func selectorWithKind(values map[string]any, kind string) map[string]any {
	out := cloneAnyMap(values)
	out["kind"] = kind
	return out
}

func (s *Server) battleApplySpecializationDamage(state *battleRuntimeState, now time.Time, source, target *battleUnitState, amount float64, damageType string, hook battleEquipmentHookRule, reason string, dispatchHooks bool) []map[string]any {
	if state == nil || source == nil || target == nil || amount <= 0 || !target.Alive || target.HP <= 0 {
		return nil
	}
	profile := battleDamageChannelProfileForType(damageType)
	defenseK := s.battleDefenseKForState(state)
	damage, _ := battlecore.ComputeDamage(battlecore.DamageInput{Physical: !profile.IsMagical, Attack: amount, Defense: target.Stats[profile.DefenseStatKey], DefenseK: defenseK, Coeff: 1, Rand: 1})
	runtime := newBattleSkillActionRuntime(state, source, now, s, nil)
	runtime.ApplyDamage(battleskills.DamageAction{SourceID: source.UnitID, TargetID: target.UnitID, Amount: damage, DamageType: profile.ID, SuppressEquipmentHooks: !dispatchHooks, Reason: "specialization_" + reason, Meta: battleSpecializationSourceMeta(hook, map[string]any{"specialization": true, "dispatchHooks": dispatchHooks})})
	return runtime.ActionLog()
}

func battleResolveEquipmentHookProcChance(actor *battleUnitState, hook battleEquipmentHookRule, effect map[string]any) (float64, bool) {
	if len(effect) == 0 {
		return 0, false
	}
	baseRaw := firstNonNil(effect["procChanceBase"], effect["proc_chance_base"], effect["procChance"], effect["proc_chance"])
	formulaRaw := firstNonNil(effect["procChanceFormula"], effect["proc_chance_formula"])
	minRaw := firstNonNil(effect["procChanceMin"], effect["proc_chance_min"])
	maxRaw := firstNonNil(effect["procChanceMax"], effect["proc_chance_max"])
	configured := baseRaw != nil || formulaRaw != nil || minRaw != nil || maxRaw != nil
	if !configured {
		return 0, false
	}
	chance := toFloat64(baseRaw, 0)
	if formulaRaw != nil {
		chance += battleEvaluateEquipmentHookProcChanceFormula(actor, hook, parseBattleSkillAmountFormula(formulaRaw, nil))
	}
	minChance := 0.0
	if minRaw != nil {
		minChance = toFloat64(minRaw, 0)
	}
	maxChance := 1.0
	if maxRaw != nil {
		maxChance = toFloat64(maxRaw, 1)
	}
	if maxChance < minChance {
		maxChance = minChance
	}
	return clampFloat(chance, minChance, maxChance), true
}

func battleEvaluateEquipmentHookProcChanceFormula(actor *battleUnitState, hook battleEquipmentHookRule, formula battleSkillAmountFormula) float64 {
	value := formula.BaseAdd
	for _, term := range formula.Terms {
		attrKey := strings.TrimSpace(term.AttrKey)
		if attrKey == "" {
			continue
		}
		coefficient := term.Coefficient
		if math.IsNaN(coefficient) || math.IsInf(coefficient, 0) {
			continue
		}
		attrValue := 0.0
		if attrKey == "equipment_enhance_level" {
			attrValue = float64(hook.SourceEnhanceLevel)
		} else if actor != nil && actor.Stats != nil {
			attrValue = actor.Stats[attrKey]
		}
		value += attrValue * coefficient
	}
	return value
}

func battleResolveEquipmentHookPayload(actor *battleUnitState, hook battleEquipmentHookRule, effect map[string]any) map[string]any {
	payload := normalizeAnyMap(effect["payload"])
	formulas := normalizeAnyMap(firstNonNil(effect["payloadFormula"], effect["payloadFormulas"], effect["payload_formula"], effect["payload_formulas"]))
	if len(formulas) == 0 {
		return payload
	}
	if payload == nil {
		payload = map[string]any{}
	}
	for key, rawFormula := range formulas {
		normalizedKey := strings.TrimSpace(key)
		if normalizedKey == "" {
			continue
		}
		payload[normalizedKey] = roundTo4(battleEvaluateAmountFormulaWithEnhance(actor, 1, parseBattleSkillAmountFormula(rawFormula, nil), hook.SourceEnhanceLevel))
	}
	return payload
}

func (s *Server) battleCollectRuntimeDamageAmpFromEquipmentHooks(
	state *battleRuntimeState,
	now time.Time,
	self *battleUnitState,
	source *battleUnitState,
	trigger string,
	incomingDamage float64,
	incomingDamageType string,
) (float64, []map[string]any, []map[string]any) {
	return s.battleCollectRuntimeDamageAmpFromEquipmentHooksWithContext(state, now, self, source, trigger, incomingDamage, incomingDamageType, nil)
}

func (s *Server) battleCollectRuntimeDamageAmpFromEquipmentHooksWithContext(
	state *battleRuntimeState,
	now time.Time,
	self *battleUnitState,
	source *battleUnitState,
	trigger string,
	incomingDamage float64,
	incomingDamageType string,
	context *battleEquipmentHookContext,
) (float64, []map[string]any, []map[string]any) {
	total, actions, sources, pending := s.battleCollectRuntimeDamageAmpFromEquipmentHooksDeferred(
		state,
		now,
		self,
		source,
		trigger,
		incomingDamage,
		incomingDamageType,
		context,
	)
	actions = append(actions, s.battleFinalizePendingProbabilityOutcomes(state, now, pending)...)
	if len(actions) == 0 {
		actions = nil
	}
	return total, actions, sources
}

func (s *Server) battleCollectRuntimeDamageAmpFromEquipmentHooksDeferred(
	state *battleRuntimeState,
	now time.Time,
	self *battleUnitState,
	source *battleUnitState,
	trigger string,
	incomingDamage float64,
	incomingDamageType string,
	context *battleEquipmentHookContext,
) (float64, []map[string]any, []map[string]any, []battlePendingProbabilityOutcome) {
	if s == nil || self == nil || self.equipmentHookCount() == 0 || (state != nil && state.derivedHookDispatchSuppression > 0) {
		return 0, nil, nil, nil
	}
	finishDispatchScope := battleBeginSpecializationDispatchScope(state)
	defer finishDispatchScope()
	triggerKey := battleNormalizeTrigger(trigger)
	if triggerKey == "" {
		return 0, nil, nil, nil
	}
	total := 0.0
	actions := make([]map[string]any, 0, 2)
	sources := make([]map[string]any, 0, 2)
	pending := make([]battlePendingProbabilityOutcome, 0, 1)
	for hookIndex := 0; hookIndex < self.equipmentHookCount(); hookIndex++ {
		hook, ok := self.equipmentHookAt(hookIndex)
		if !ok {
			continue
		}
		if hook.SourceKind == "specialization" {
			if err := validateBattleSpecializationHookRule(hook); err != nil {
				actions = append(actions, map[string]any{
					"kind": "specialization_definition_invalid", "targetId": self.UnitID, "sourceId": self.UnitID,
					"reason": "specialization_fail_closed", "meta": map[string]any{"hookId": hook.ID, "error": err.Error()},
				})
				continue
			}
		}
		if battleNormalizeTrigger(hook.Trigger) != triggerKey {
			continue
		}
		if hook.SourceKind == "specialization" {
			if !battleEquipmentHookSubjectAllowed(hook, source) || !battleEquipmentHookEventAllowed(hook, self, source, context) {
				continue
			}
		}
		if !battleEquipmentHookLimitAllowed(state, self, hook) {
			continue
		}
		conditionsPassed := true
		for _, condition := range hook.Conditions {
			if !s.battleEvaluateEquipmentHookCondition(state, condition, self, source, source, incomingDamage, incomingDamageType, hook.SourceEnhanceLevel, context) {
				conditionsPassed = false
				break
			}
		}
		if !conditionsPassed {
			continue
		}
		if hook.SourceKind == "specialization" {
			hasPreDamageEffect := false
			for _, effect := range hook.Effects {
				switch battleNormalizeTrigger(fmt.Sprintf("%v", firstNonNil(effect["kind"], effect["type"]))) {
				case "runtimedamageamp", "consumecounterfordamageamp", "flatdefensepenetration":
					hasPreDamageEffect = true
				}
			}
			if !hasPreDamageEffect {
				continue
			}
		}
		leaveContribution := func() {}
		if hook.SourceKind == "specialization" {
			var blockedBy string
			leaveContribution, blockedBy = battleEnterSpecializationContribution(state, hook)
			if blockedBy != "" {
				actions = append(actions, battleSpecializationDispatchBlockedAction(self, trigger, hook, blockedBy))
				continue
			}
		}
		hookActionStart := len(actions)
		battleMarkEquipmentHookTriggered(state, self, hook)
		for _, effect := range hook.Effects {
			kind := battleNormalizeTrigger(fmt.Sprintf("%v", effect["kind"]))
			if kind == "" {
				kind = battleNormalizeTrigger(fmt.Sprintf("%v", effect["type"]))
			}
			if hook.SourceKind == "specialization" {
				switch kind {
				case "consumecounterfordamageamp":
					counterID := strings.TrimSpace(fmt.Sprintf("%v", firstNonNil(effect["counterId"], effect["counter_id"])))
					consume := maxInt(0, int(toInt64Loose(effect["consume"], 0)))
					stacks := battleSpecializationCounterStacks(self, counterID)
					if consume <= 0 || stacks < consume {
						continue
					}
					battleSetSpecializationCounter(self, counterID, stacks-consume)
					value := battleSpecializationEffectScalar(self, effect, "value")
					value += s.battleEquivalentRuntimePenetrationAmp(state, self, source, incomingDamageType, 0, toFloat64(firstNonNil(effect["percentDefensePenetration"], effect["percent_defense_penetration"]), 0), context)
					total += value
					actions = append(actions, map[string]any{"kind": "specialization_counter", "targetId": self.UnitID, "sourceId": self.UnitID, "reason": trigger, "meta": map[string]any{"specialization": true, "hookId": hook.ID, "counterId": counterID, "before": stacks, "after": stacks - consume, "runtimeDamageAmp": roundTo4(value)}})
					continue
				case "flatdefensepenetration":
					flat := battleSpecializationEffectScalar(self, effect, "value")
					value := s.battleEquivalentRuntimePenetrationAmp(state, self, source, incomingDamageType, flat, 0, context)
					total += value
					actions = append(actions, map[string]any{"kind": "trigger", "targetId": self.UnitID, "sourceId": self.UnitID, "reason": trigger, "meta": map[string]any{"specialization": true, "hookId": hook.ID, "flatDefensePenetration": roundTo4(flat), "runtimeDamageAmp": roundTo4(value)}})
					continue
				}
			}
			if kind != "runtimedamageamp" && kind != "addruntimedamageamp" && kind != "adddamageamp" && kind != "adddamagepercent" {
				continue
			}
			if !battleConditionDamageTypeAllowed(effect, incomingDamageType) {
				continue
			}
			value := 0.0
			if hook.SourceKind == "specialization" {
				value = battleSpecializationEffectScalar(self, effect, "value", "percent")
			} else {
				value = toFloat64(firstNonNil(effect["value"], effect["percent"]), 0)
				if rawFormula, configured := battleSpecializationValueFormulaRaw(effect); configured {
					value += battleEvaluateAmountFormulaWithEnhance(
						self,
						1,
						parseBattleSkillAmountFormula(rawFormula, nil),
						hook.SourceEnhanceLevel,
					)
				}
			}
			if minRaw := firstNonNil(effect["valueMin"], effect["value_min"], effect["min"]); minRaw != nil {
				value = maxFloat(value, toFloat64(minRaw, value))
			}
			if maxRaw := firstNonNil(effect["valueMax"], effect["value_max"], effect["max"]); maxRaw != nil {
				if maxValue := toFloat64(maxRaw, value); value > maxValue {
					value = maxValue
				}
			}
			if value == 0 {
				continue
			}
			probabilityCheck, probabilityResult, procChanceConfigured := s.battleResolveEquipmentEffectProbability(state, self, source, hook, effect)
			if procChanceConfigured {
				if !probabilityResult.Established {
					continue
				}
				if !probabilityResult.Succeeded {
					pending = append(pending, battlePendingProbabilityOutcome{Check: probabilityCheck, Result: probabilityResult})
					continue
				}
			}
			total += value
			hookMeta := map[string]any{
				"sourceItemId":       hook.SourceItemID,
				"sourceEnhanceLevel": hook.SourceEnhanceLevel,
				"hookId":             hook.ID,
				"runtimeDamageAmp":   roundTo4(value),
				"incomingDamageType": normalizeBattleDamageType(incomingDamageType),
				"ts":                 now.UnixMilli(),
			}
			if hook.SourceInstanceID > 0 {
				hookMeta["sourceInstanceId"] = hook.SourceInstanceID
				hookMeta["hookDefinitionId"] = hook.DefinitionID
			}
			if procChanceConfigured {
				hookMeta["procChance"] = roundTo4(probabilityResult.FinalChance)
			}
			if strings.TrimSpace(hook.SourceAffixID) != "" {
				hookMeta["sourceAffixId"] = hook.SourceAffixID
				hookMeta["sourceAffixSlot"] = hook.SourceAffixSlot
				sources = append(sources, map[string]any{
					"kind":               "affix",
					"id":                 hook.SourceAffixID,
					"sourceItemId":       hook.SourceItemID,
					"sourceEnhanceLevel": hook.SourceEnhanceLevel,
					"sourceAffixSlot":    hook.SourceAffixSlot,
					"sourceInstanceId":   hook.SourceInstanceID,
					"hookId":             hook.ID,
					"runtimeDamageAmp":   roundTo4(value),
					"sourceId":           self.UnitID,
				})
			}
			if hook.SourceKind == "specialization" {
				sources = append(sources, map[string]any{
					"kind": "specialization", "id": hook.SourceID, "nodeId": hook.SourceID,
					"sourceNameKey": hook.SourceNameKey, "hookId": hook.ID,
					"runtimeDamageAmp": roundTo4(value), "sourceId": self.UnitID,
				})
			} else if strings.TrimSpace(hook.SourceItemID) != "" {
				sources = append(sources, map[string]any{
					"kind":               "equipment",
					"id":                 hook.SourceItemID,
					"sourceEnhanceLevel": hook.SourceEnhanceLevel,
					"sourceAffixId":      strings.TrimSpace(hook.SourceAffixID),
					"sourceAffixSlot":    hook.SourceAffixSlot,
					"sourceInstanceId":   hook.SourceInstanceID,
					"hookId":             hook.ID,
					"runtimeDamageAmp":   roundTo4(value),
					"sourceId":           self.UnitID,
				})
			}
			actions = append(actions, map[string]any{
				"kind":     "trigger",
				"targetId": self.UnitID,
				"sourceId": self.UnitID,
				"reason":   trigger,
				"meta":     hookMeta,
			})
			if procChanceConfigured {
				pending = append(pending, battlePendingProbabilityOutcome{Check: probabilityCheck, Result: probabilityResult})
			}
		}
		if hook.SourceKind == "specialization" {
			for actionIndex := hookActionStart; actionIndex < len(actions); actionIndex++ {
				enrichBattleSpecializationAction(actions[actionIndex], hook)
			}
		} else if hook.SourceKind == "equipment" {
			for actionIndex := hookActionStart; actionIndex < len(actions); actionIndex++ {
				enrichBattleEquipmentInstanceAction(actions[actionIndex], hook)
			}
		}
		leaveContribution()
	}
	if triggerKey == "onbeforedealdamage" {
		deferredTotal, deferredActions := s.battleCollectSpecializationDeferredPreDamage(state, self, source, trigger, incomingDamage, incomingDamageType, context)
		total += deferredTotal
		actions = append(actions, deferredActions...)
	}
	if len(actions) == 0 {
		return total, nil, sources, pending
	}
	return total, actions, sources, pending
}

func (s *Server) battleCollectSpecializationDeferredPreDamage(
	state *battleRuntimeState,
	self, source *battleUnitState,
	trigger string,
	incomingDamage float64,
	incomingDamageType string,
	context *battleEquipmentHookContext,
) (float64, []map[string]any) {
	if s == nil || self == nil {
		return 0, nil
	}
	total := 0.0
	actions := make([]map[string]any, 0, 2)
	for index := len(self.Buffs) - 1; index >= 0; index-- {
		buff := self.Buffs[index]
		buffID := strings.ToLower(strings.TrimSpace(buff.BuffID))
		if !strings.HasPrefix(buffID, "specialization:empower:") {
			continue
		}
		value := toFloat64(buff.Payload["damageAmp"], 0)
		total += value
		self.Buffs = append(self.Buffs[:index], self.Buffs[index+1:]...)
		meta := cloneAnyMap(buff.Payload)
		meta["specialization"] = true
		meta["runtimeDamageAmp"] = roundTo4(value)
		actions = append(actions, map[string]any{
			"kind": "state_detach", "targetId": self.UnitID, "sourceId": self.UnitID,
			"buffId": buffID, "reason": trigger, "meta": meta,
		})
	}
	if source == nil {
		return total, actions
	}
	for _, hook := range self.SpecializationHooks {
		if battleNormalizeTrigger(hook.Trigger) != "onafterdealdamage" || validateBattleSpecializationHookRule(hook) != nil {
			continue
		}
		if !battleEquipmentHookSubjectAllowed(hook, source) || !battleEquipmentHookEventAllowed(hook, self, source, context) || !battleEquipmentHookLimitAllowed(state, self, hook) {
			continue
		}
		conditionsPassed := true
		for _, condition := range hook.Conditions {
			if !s.battleEvaluateEquipmentHookCondition(state, condition, self, source, source, incomingDamage, incomingDamageType, hook.SourceEnhanceLevel, context) {
				conditionsPassed = false
				break
			}
		}
		if !conditionsPassed {
			continue
		}
		settledHook := false
		for _, cycle := range hook.Effects {
			cycleKind := battleNormalizeTrigger(fmt.Sprintf("%v", firstNonNil(cycle["kind"], cycle["type"])))
			if cycleKind != "buffcycle" && cycleKind != "markcycle" {
				continue
			}
			stateID := ""
			buffID := ""
			hit := map[string]any(nil)
			consume := false
			if cycleKind == "buffcycle" {
				stateID = strings.TrimSpace(fmt.Sprintf("%v", firstNonNil(cycle["buffId"], cycle["buff_id"])))
				buffID = stateID
				hit = normalizeAnyMap(firstNonNil(cycle["buffedHit"], cycle["buffed_hit"]))
				consume = parseBooleanLoose(firstNonNil(hit["consumeBuff"], hit["consume_buff"]), false)
			} else {
				stateID = strings.TrimSpace(fmt.Sprintf("%v", firstNonNil(cycle["markId"], cycle["mark_id"])))
				buffID = battleSpecializationMarkBuffID(stateID)
				hit = normalizeAnyMap(firstNonNil(cycle["markedHit"], cycle["marked_hit"]))
				consume = parseBooleanLoose(firstNonNil(hit["consumeMark"], hit["consume_mark"]), false)
			}
			if stateID == "" || !battleHasBuff(source, buffID) {
				continue
			}
			value := battleSpecializationEffectScalar(self, hit, "value", "percent")
			total += value
			if consume {
				battleRemoveBuff(source, buffID)
				if context != nil {
					if cycleKind == "buffcycle" {
						if context.ConsumedBuffs == nil {
							context.ConsumedBuffs = map[string]bool{}
						}
						context.ConsumedBuffs[strings.ToLower(stateID)] = true
					} else {
						if context.ConsumedMarks == nil {
							context.ConsumedMarks = map[string]bool{}
						}
						context.ConsumedMarks[strings.ToLower(stateID)] = true
					}
				}
			}
			settledHook = true
			actions = append(actions, map[string]any{
				"kind": "state_detach", "targetId": source.UnitID, "sourceId": self.UnitID,
				"buffId": buffID, "reason": trigger,
				"meta": battleSpecializationSourceMeta(hook, map[string]any{"specialization": true, "runtimeDamageAmp": roundTo4(value)}),
			})
		}
		if settledHook {
			battleMarkEquipmentHookTriggered(state, self, hook)
		}
	}
	return total, actions
}

func (s *Server) battleEquivalentRuntimePenetrationAmp(state *battleRuntimeState, source, target *battleUnitState, damageType string, extraFlat, extraPercent float64, context *battleEquipmentHookContext) float64 {
	if source == nil || target == nil || (extraFlat <= 0 && extraPercent <= 0) {
		return 0
	}
	profile := battleDamageChannelProfileForType(damageType)
	defense := maxFloat(0, target.Stats[profile.DefenseStatKey])
	existingFlat := 0.0
	existingPercent := 0.0
	if context != nil {
		if context.RawDefense > 0 {
			defense = context.RawDefense
		}
		existingFlat = maxFloat(0, context.FlatPenetration)
		existingPercent = maxFloat(0, context.PercentPenetration)
	}
	percentCap := s.deps.Config.SharedGlobalSettings.CombatBalanceModel.PercentPenetrationCap
	if percentCap <= 0 || percentCap > 1 {
		percentCap = 0.8
	}
	existingPercent = clampFloat(existingPercent, 0, percentCap)
	defenseK := s.battleDefenseKForState(state)
	baseEffective := battleResolveEffectiveDefense(defense, existingFlat, existingPercent)
	nextEffective := battleResolveEffectiveDefense(defense, existingFlat+extraFlat, clampFloat(existingPercent+extraPercent, 0, percentCap))
	baseEffective = maxFloat(baseEffective, -defenseK/2)
	nextEffective = maxFloat(nextEffective, -defenseK/2)
	baseMultiplier := defenseK / (defenseK + baseEffective)
	nextMultiplier := defenseK / (defenseK + nextEffective)
	if baseMultiplier <= 0 {
		return 0
	}
	return maxFloat(0, nextMultiplier/baseMultiplier-1)
}

func (s *Server) executeOtherUnitTurnEndEquipmentHooks(state *battleRuntimeState, now time.Time, completedUnit *battleUnitState, parentSeq int64) {
	if s == nil || state == nil || completedUnit == nil || !completedUnit.Alive || completedUnit.HP <= 0 {
		return
	}
	completedID := strings.TrimSpace(completedUnit.UnitID)
	if completedID == "" {
		return
	}
	hookUnitIDs := state.otherUnitTurnEndHookUnitIDsForRuntime()
	if len(hookUnitIDs) == 0 {
		return
	}
	triggerKey := battleNormalizeTrigger(battleEquipmentTriggerOtherTurnEnd)
	effectKey := battleNormalizeTrigger(battleEquipmentEffectMarkedAttack)
	processHooks := func(unit *battleUnitState, hooks []*battleEquipmentHookRule) {
		for _, hookRef := range hooks {
			if hookRef == nil {
				continue
			}
			hook := *hookRef
			if battleNormalizeTrigger(hook.Trigger) != triggerKey {
				continue
			}
			if !battleEquipmentHookLimitAllowed(state, unit, hook) {
				continue
			}
			conditionsPassed := true
			for _, condition := range hook.Conditions {
				if !s.battleEvaluateEquipmentHookCondition(state, condition, unit, completedUnit, nil, 0, "", hook.SourceEnhanceLevel, nil) {
					conditionsPassed = false
					break
				}
			}
			if !conditionsPassed {
				continue
			}
			battleMarkEquipmentHookTriggered(state, unit, hook)
			for _, effect := range hook.Effects {
				kind := battleNormalizeTrigger(fmt.Sprintf("%v", effect["kind"]))
				if kind == "" {
					kind = battleNormalizeTrigger(fmt.Sprintf("%v", effect["type"]))
				}
				if kind != effectKey {
					continue
				}
				procChance, procChanceConfigured := battleResolveEquipmentHookProcChance(unit, hook, effect)
				probabilityCheck := battleProbabilityCheck{}
				probabilityResult := battleProbabilityResult{}
				if procChanceConfigured {
					probabilityCheck = battleEquipmentProbabilityCheck(unit, nil, hook, effect, procChance)
					probabilityResult = s.resolveBattleProbability(state, probabilityCheck)
					if !probabilityResult.Established {
						continue
					}
					if !probabilityResult.Succeeded {
						// Failure hooks settle even when the proc never selects a target.
						failureActions := s.battleFinalizeProbabilityResult(state, now, probabilityCheck, probabilityResult)
						if len(failureActions) > 0 {
							state.emit("battle.turn.action", now, map[string]any{"wave": state.Wave, "actorId": unit.UnitID, "actionLog": failureActions, "parentSeq": parentSeq})
						}
						continue
					}
				}
				target := s.battleSelectRandomMarkedTarget(state, unit)
				if target == nil {
					continue
				}
				if procChanceConfigured {
					probabilityCheck.Target = target
					probabilityResult.TargetID = target.UnitID
					procChance = probabilityResult.FinalChance
				}
				payload, ok := s.executeMarkedTargetBasicAttack(state, now, unit, target, parentSeq, hook, procChance, procChanceConfigured)
				if !ok {
					continue
				}
				if procChanceConfigured {
					if actionLog, ok := payload["actionLog"].([]map[string]any); ok {
						payload["actionLog"] = append(actionLog, s.battleFinalizeProbabilityResult(state, now, probabilityCheck, probabilityResult)...)
					}
				}
				state.emit("battle.turn.action", now, payload)
			}
		}
	}
	for _, hookUnitID := range hookUnitIDs {
		hookUnitID = strings.TrimSpace(hookUnitID)
		if hookUnitID == "" || hookUnitID == completedID {
			continue
		}
		unit := battleUnitByID(state, hookUnitID)
		if unit == nil {
			continue
		}
		unitID := strings.TrimSpace(unit.UnitID)
		if unitID == "" || unitID == completedID {
			continue
		}
		if unit == nil || !unit.Alive || unit.HP <= 0 {
			continue
		}
		processHooks(unit, unit.otherUnitTurnEndEquipmentHooks())
	}
}

func (s *Server) battleSelectRandomMarkedTarget(state *battleRuntimeState, actor *battleUnitState) *battleUnitState {
	if s == nil || state == nil || actor == nil {
		return nil
	}
	candidates := make([]*battleUnitState, 0, 2)
	collect := func(units []battleUnitState) {
		for i := range units {
			unit := &units[i]
			if unit == nil || !unit.Alive || unit.HP <= 0 {
				continue
			}
			if strings.TrimSpace(unit.UnitID) == strings.TrimSpace(actor.UnitID) {
				continue
			}
			if s.battleTargetRelationForActor(state, actor, unit) != "enemy" {
				continue
			}
			if !s.battleUnitHasWhileActiveEffect(unit, "marked_target", actor) {
				continue
			}
			candidates = append(candidates, unit)
		}
	}
	collect(state.Players)
	collect(state.Enemies)
	if len(candidates) == 0 {
		return nil
	}
	idx := int(math.Floor(state.randFloat64() * float64(len(candidates))))
	if idx < 0 {
		idx = 0
	}
	if idx >= len(candidates) {
		idx = len(candidates) - 1
	}
	return candidates[idx]
}

func (s *Server) executeMarkedTargetBasicAttack(
	state *battleRuntimeState,
	now time.Time,
	actor *battleUnitState,
	target *battleUnitState,
	parentSeq int64,
	hook battleEquipmentHookRule,
	procChance float64,
	procChanceConfigured bool,
) (map[string]any, bool) {
	if s == nil || state == nil || actor == nil || target == nil || !actor.Alive || actor.HP <= 0 || !target.Alive || target.HP <= 0 {
		return nil, false
	}
	skillID := s.battleDefaultBasicSkillID()
	if strings.TrimSpace(skillID) == "" {
		return nil, false
	}
	skill := s.battleSkillByID(skillID)
	if strings.TrimSpace(skill.ID) == "" {
		return nil, false
	}
	execution := s.executeBattleSkill(state, now, actor, skill, "equipment_follow_up", []string{target.UnitID})
	resultMeta := cloneAnyMap(execution.Meta)
	if resultMeta == nil {
		resultMeta = map[string]any{}
	}
	resultMeta["markedTargetFollowUp"] = true
	resultMeta["followUpTrigger"] = "on_other_unit_turn_end"
	resultMeta["followUpHookId"] = hook.ID
	resultMeta["followUpSourceItemId"] = hook.SourceItemID
	resultMeta["markedTargetId"] = target.UnitID
	if procChanceConfigured {
		resultMeta["procChance"] = roundTo4(procChance)
	}
	payload := map[string]any{
		"wave":             state.Wave,
		"actorId":          actor.UnitID,
		"actorSide":        actor.Side,
		"actorUserId":      actor.UserID,
		"actorEnemyId":     actor.EnemyID,
		"skillId":          skill.ID,
		"skillName":        skill.Name,
		"actorHp":          roundTo4(actor.HP),
		"actorMp":          roundTo4(actor.MP),
		"actorAlive":       actor.Alive,
		"targets":          execution.Targets,
		"effects":          execution.Effects,
		"actionLog":        execution.Actions,
		"hooks":            []map[string]any{},
		"preparationTurn":  false,
		"preparationOrder": append([]string(nil), state.PreparationOrder...),
		"parentSeq":        parentSeq,
		"batchGroup":       fmt.Sprintf("followup:%d", parentSeq),
		"resultMeta":       resultMeta,
	}
	if deathHookActions := s.battleFlushPendingUnitDeathEquipmentHooks(state, now); len(deathHookActions) > 0 {
		payload["actionLog"] = append(payload["actionLog"].([]map[string]any), deathHookActions...)
	}
	populateTargetSlots(payload["targets"].([]map[string]any), state, strings.TrimSpace(fmt.Sprintf("%v", payload["actorSide"])))
	state.recordAchievementAction(actor.UnitID, skill.ID, payload["actionLog"])
	return payload, true
}

func (s *Server) executeEquipmentExtraTurn(
	state *battleRuntimeState,
	_ time.Time,
	actor *battleUnitState,
	trigger string,
	hook battleEquipmentHookRule,
	eventTarget *battleUnitState,
	turnMode string,
	dispatchHooksOpt ...bool,
) (map[string]any, bool) {
	if s == nil || state == nil || actor == nil || !actor.Alive || actor.HP <= 0 || s.battleUnitTurnsIneligible(state, actor) {
		return nil, false
	}
	if turnMode != "full" {
		return nil, false
	}
	dispatchHooks := true
	if len(dispatchHooksOpt) > 0 {
		dispatchHooks = dispatchHooksOpt[0]
	}
	actorID := strings.TrimSpace(actor.UnitID)
	if actorID == "" {
		return nil, false
	}
	isActiveExtraTurnActor := battleUnitHasActiveExtraTurn(state, actor)
	if isActiveExtraTurnActor && hook.SourceKind != "specialization" {
		if !battleIsProbabilityOutcomeTrigger(trigger) || !battleCanQueueProbabilityExtraTurn(state, actor, hook) {
			return nil, false
		}
	}
	if len(state.ExtraTurnQueue) >= battleMaxQueuedExtraTurnsPerChain {
		return nil, false
	}
	chainDepth := 1
	chainHookCounts := map[string]int{}
	withinActiveExtraTurn := state.activeExtraTurnRequest != nil
	if withinActiveExtraTurn {
		chainDepth = state.activeExtraTurnRequest.ChainDepth + 1
		if chainDepth > battleMaxQueuedExtraTurnsPerChain {
			return nil, false
		}
		chainHookCounts = maps.Clone(state.activeExtraTurnRequest.ChainHookCounts)
		if chainHookCounts == nil {
			chainHookCounts = map[string]int{}
		}
	}
	chainHookCounts[battleExtraTurnChainHookKey(actor, hook)]++
	eventTargetID := ""
	eventTargetType := ""
	if eventTarget != nil {
		eventTargetID = strings.TrimSpace(eventTarget.UnitID)
		eventTargetType = battleUnitFilterType(eventTarget)
	}
	request := battleQueuedExtraTurn{
		ActorID:          actorID,
		Trigger:          strings.TrimSpace(trigger),
		HookID:           strings.TrimSpace(hook.ID),
		SourceKind:       strings.TrimSpace(hook.SourceKind),
		SourceID:         strings.TrimSpace(hook.SourceID),
		SourceNameKey:    strings.TrimSpace(hook.SourceNameKey),
		SourceItemID:     strings.TrimSpace(hook.SourceItemID),
		SourceInstanceID: hook.SourceInstanceID,
		SourceAffixID:    strings.TrimSpace(hook.SourceAffixID),
		SourceAffixSlot:  hook.SourceAffixSlot,
		EventTargetID:    eventTargetID,
		EventTargetType:  eventTargetType,
		TurnMode:         turnMode,
		DispatchHooks:    dispatchHooks,
		ChainDepth:       chainDepth,
		ChainHookCounts:  chainHookCounts,
	}
	if withinActiveExtraTurn {
		insertAt := minInt(state.activeExtraTurnChildCount, len(state.ExtraTurnQueue))
		state.ExtraTurnQueue = append(state.ExtraTurnQueue, battleQueuedExtraTurn{})
		copy(state.ExtraTurnQueue[insertAt+1:], state.ExtraTurnQueue[insertAt:])
		state.ExtraTurnQueue[insertAt] = request
		state.activeExtraTurnChildCount++
	} else {
		state.ExtraTurnQueue = append(state.ExtraTurnQueue, request)
	}
	return battleQueuedExtraTurnPayload(request), true
}

func battleQueuedExtraTurnPayload(request battleQueuedExtraTurn) map[string]any {
	payload := map[string]any{
		"actorId":       request.ActorID,
		"queued":        true,
		"turnMode":      request.TurnMode,
		"dispatchHooks": request.DispatchHooks,
	}
	for key, value := range map[string]string{
		"trigger":         request.Trigger,
		"hookId":          request.HookID,
		"sourceKind":      request.SourceKind,
		"sourceId":        request.SourceID,
		"sourceNameKey":   request.SourceNameKey,
		"sourceItemId":    request.SourceItemID,
		"sourceAffixId":   request.SourceAffixID,
		"eventTargetId":   request.EventTargetID,
		"eventTargetType": request.EventTargetType,
	} {
		if strings.TrimSpace(value) != "" {
			payload[key] = value
		}
	}
	if request.SourceInstanceID > 0 {
		payload["sourceInstanceId"] = request.SourceInstanceID
	}
	if request.SourceAffixSlot > 0 {
		payload["sourceAffixSlot"] = request.SourceAffixSlot
	}
	return payload
}

func battleDecorateQueuedExtraTurnPayload(payload map[string]any, request battleQueuedExtraTurn) {
	if payload == nil {
		return
	}
	resultMeta := cloneAnyMap(normalizeAnyMap(payload["resultMeta"]))
	if resultMeta == nil {
		resultMeta = map[string]any{}
	}
	resultMeta["extraTurn"] = true
	resultMeta["extraTurnTrigger"] = request.Trigger
	resultMeta["extraTurnHookId"] = request.HookID
	resultMeta["extraTurnSourceKind"] = request.SourceKind
	resultMeta["extraTurnSourceId"] = request.SourceID
	resultMeta["extraTurnSourceNameKey"] = request.SourceNameKey
	resultMeta["extraTurnSourceItemId"] = request.SourceItemID
	resultMeta["extraTurnMode"] = request.TurnMode
	resultMeta["dispatchHooks"] = request.DispatchHooks
	if request.SourceInstanceID > 0 {
		resultMeta["extraTurnSourceInstanceId"] = request.SourceInstanceID
	}
	if request.SourceAffixID != "" {
		resultMeta["extraTurnSourceAffixId"] = request.SourceAffixID
		resultMeta["extraTurnSourceAffixSlot"] = request.SourceAffixSlot
	}
	if request.EventTargetID != "" {
		resultMeta["extraTurnEventTargetId"] = request.EventTargetID
	}
	if request.EventTargetType != "" {
		resultMeta["extraTurnEventTargetType"] = request.EventTargetType
	}
	payload["resultMeta"] = resultMeta
	payload["extraTurn"] = true
	payload["turnKind"] = "extra"
}

func (s *Server) battleTriggerAllyAfterTakeDamageEquipmentHooks(
	state *battleRuntimeState,
	now time.Time,
	damaged *battleUnitState,
	damageSource *battleUnitState,
	incomingDamage float64,
	incomingDamageType string,
) []map[string]any {
	if s == nil || state == nil || damaged == nil || strings.TrimSpace(damaged.UnitID) == "" {
		return nil
	}
	if !damaged.Alive || damaged.HP <= 0 {
		return nil
	}
	damagedTeam := battleUnitTeamSide(damaged)
	if damagedTeam == "" {
		return nil
	}
	damagedID := strings.TrimSpace(damaged.UnitID)
	actions := make([]map[string]any, 0, 2)
	collect := func(units []battleUnitState) {
		for i := range units {
			unit := &units[i]
			unitID := strings.TrimSpace(unit.UnitID)
			if unitID == "" || unitID == damagedID {
				continue
			}
			if !unit.Alive || unit.HP <= 0 {
				continue
			}
			if battleUnitTeamSide(unit) != damagedTeam {
				continue
			}
			rows := s.battleTriggerUnitEquipmentHooksWithTarget(state, now, unit, damageSource, damaged, battleEquipmentTriggerAllyAfterTakeDamage, incomingDamage, incomingDamageType)
			if len(rows) > 0 {
				actions = append(actions, rows...)
			}
		}
	}
	collect(state.Players)
	collect(state.Enemies)
	if len(actions) == 0 {
		return nil
	}
	return actions
}

type battleSpecializationPreDamageResult struct {
	Amount              float64
	DeferredHeal        float64
	DeferredHealSources []map[string]any
	Actions             []map[string]any
}

func (s *Server) battleApplySpecializationPreDamageAdjustments(
	state *battleRuntimeState,
	now time.Time,
	target *battleUnitState,
	source *battleUnitState,
	amount float64,
	damageType string,
	context *battleEquipmentHookContext,
) battleSpecializationPreDamageResult {
	result := battleSpecializationPreDamageResult{Amount: maxFloat(0, amount)}
	if s == nil || state == nil || target == nil || result.Amount <= 0 || state.derivedHookDispatchSuppression > 0 {
		return result
	}
	finishDispatchScope := battleBeginSpecializationDispatchScope(state)
	defer finishDispatchScope()
	appendAction := func(kind, targetID, sourceID string, sourceMeta map[string]any, extra map[string]any) {
		meta := cloneAnyMap(sourceMeta)
		if meta == nil {
			meta = map[string]any{}
		}
		meta["specialization"] = true
		for key, value := range normalizeAnyMap(extra["meta"]) {
			meta[key] = value
		}
		row := map[string]any{
			"kind": kind, "targetId": targetID, "sourceId": sourceID,
			"reason": "specialization_pre_damage", "delayMs": int64(0),
			"meta": meta,
		}
		for key, value := range extra {
			if key == "meta" {
				continue
			}
			row[key] = value
		}
		result.Actions = append(result.Actions, row)
	}

	// Ally interception settles first. The protected unit's own mitigation then
	// applies only to the damage that still reaches it.
	allies := battleAlliesForSpecialization(state, target)
	sort.SliceStable(allies, func(i, j int) bool { return allies[i].UnitID < allies[j].UnitID })
	for _, protector := range allies {
		if protector == nil || !protector.Alive || protector.HP <= 0 || result.Amount <= 0 {
			continue
		}
		for _, hook := range protector.SpecializationHooks {
			if battleNormalizeTrigger(hook.Trigger) != "onallybeforetakedamage" || validateBattleSpecializationHookRule(hook) != nil {
				continue
			}
			if !battleEquipmentHookSubjectAllowed(hook, target) || !battleEquipmentHookEventAllowed(hook, protector, source, context) {
				continue
			}
			if !battleEquipmentHookLimitAllowed(state, protector, hook) {
				continue
			}
			conditionsPassed := true
			for _, condition := range hook.Conditions {
				if !s.battleEvaluateEquipmentHookCondition(state, condition, protector, source, target, result.Amount, damageType, hook.SourceEnhanceLevel, context) {
					conditionsPassed = false
					break
				}
			}
			if !conditionsPassed {
				continue
			}
			hasAdjustment := false
			for _, effect := range hook.Effects {
				if battleNormalizeTrigger(fmt.Sprintf("%v", firstNonNil(effect["kind"], effect["type"]))) == "redirectdamage" {
					hasAdjustment = true
					break
				}
			}
			if !hasAdjustment {
				continue
			}
			leaveContribution, blockedBy := battleEnterSpecializationContribution(state, hook)
			if blockedBy != "" {
				result.Actions = append(result.Actions, battleSpecializationDispatchBlockedAction(protector, hook.Trigger, hook, blockedBy))
				continue
			}
			battleMarkEquipmentHookTriggered(state, protector, hook)
			for _, effect := range hook.Effects {
				kind := battleNormalizeTrigger(fmt.Sprintf("%v", firstNonNil(effect["kind"], effect["type"])))
				switch kind {
				case "redirectdamage":
					ratio := clampFloat(battleSpecializationEffectScalar(protector, effect, "ratio"), 0, 1)
					redirected := math.Min(result.Amount, result.Amount*ratio)
					if redirected <= 0 {
						continue
					}
					result.Amount = maxFloat(0, result.Amount-redirected)
					dispatchHooks := parseBooleanLoose(firstNonNil(effect["dispatchHooks"], effect["dispatch_hooks"], effect["dispatchEquipmentHooks"], effect["dispatch_equipment_hooks"]), hook.DispatchHooks)
					damageSource := source
					if damageSource == nil {
						damageSource = target
					}
					runtime := newBattleSkillActionRuntime(state, damageSource, now, s, nil)
					runtime.skillID = "specialization_redirect"
					runtime.ApplyDamage(battleskills.DamageAction{
						SourceID: damageSource.UnitID, TargetID: protector.UnitID, Amount: redirected,
						DamageType: damageType, SuppressEquipmentHooks: !dispatchHooks,
						Reason: "specialization_redirect_damage",
						Meta: battleSpecializationSourceMeta(hook, map[string]any{
							"specialization": true, "protectedId": target.UnitID, "dispatchHooks": dispatchHooks,
							"skipFinalDamageModifiers": true, "suppressSourceHooks": true,
						}),
					})
					for _, row := range runtime.ActionLog() {
						enrichBattleSpecializationAction(row, hook)
						if row["kind"] == "damage" {
							row["kind"] = "damage_redirect"
						}
						result.Actions = append(result.Actions, row)
					}
				case "marktarget", "addbuff":
					rows, _ := s.executeBattleSpecializationHookEffect(state, now, protector, source, target, hook.Trigger, result.Amount, damageType, hook, effect, context)
					result.Actions = append(result.Actions, rows...)
				}
			}
			leaveContribution()
		}
	}

	// Specialization guard buffs are real runtime mitigation, not presentation
	// markers. Multiple independent guards compose multiplicatively.
	for _, buff := range target.Buffs {
		if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(buff.BuffID)), "specialization:damage_reduction:") {
			continue
		}
		value := clampFloat(toFloat64(buff.Payload["value"], 0), 0, 1)
		if value <= 0 {
			continue
		}
		before := result.Amount
		result.Amount *= 1 - value
		guardMeta := cloneAnyMap(buff.Payload)
		guardMeta["hookId"] = strings.TrimPrefix(buff.BuffID, "specialization:damage_reduction:")
		appendAction("damage_reduction", target.UnitID, strings.TrimSpace(buff.SourceID), guardMeta, map[string]any{
			"amount": roundTo4(before - result.Amount), "beforeDamage": roundTo4(before), "afterDamage": roundTo4(result.Amount),
		})
	}

	for _, hook := range target.SpecializationHooks {
		if battleNormalizeTrigger(hook.Trigger) != "onbeforetakedamage" || validateBattleSpecializationHookRule(hook) != nil {
			continue
		}
		if !battleEquipmentHookSubjectAllowed(hook, target) || !battleEquipmentHookEventAllowed(hook, target, source, context) {
			continue
		}
		if !battleEquipmentHookLimitAllowed(state, target, hook) {
			continue
		}
		conditionsPassed := true
		for _, condition := range hook.Conditions {
			if !s.battleEvaluateEquipmentHookCondition(state, condition, target, source, target, result.Amount, damageType, hook.SourceEnhanceLevel, context) {
				conditionsPassed = false
				break
			}
		}
		if !conditionsPassed {
			continue
		}
		hasAdjustment := false
		for _, effect := range hook.Effects {
			switch battleNormalizeTrigger(fmt.Sprintf("%v", firstNonNil(effect["kind"], effect["type"]))) {
			case "heal", "preventlethal", "resourceshield":
				hasAdjustment = true
			}
		}
		if !hasAdjustment {
			continue
		}
		leaveContribution, blockedBy := battleEnterSpecializationContribution(state, hook)
		if blockedBy != "" {
			result.Actions = append(result.Actions, battleSpecializationDispatchBlockedAction(target, hook.Trigger, hook, blockedBy))
			continue
		}
		battleMarkEquipmentHookTriggered(state, target, hook)
		for _, effect := range hook.Effects {
			kind := battleNormalizeTrigger(fmt.Sprintf("%v", firstNonNil(effect["kind"], effect["type"])))
			switch kind {
			case "resourceshield":
				if battleNormalizeTrigger(fmt.Sprintf("%v", effect["resource"])) != "mp" {
					continue
				}
				absorbRatio := clampFloat(battleSpecializationEffectScalar(target, effect, "absorbRatio", "absorb_ratio"), 0, 1)
				resourcePerDamage := toFloat64(firstNonNil(effect["resourcePerDamage"], effect["resource_per_damage"]), 0)
				if absorbRatio <= 0 || resourcePerDamage <= 0 || target.MP <= 0 {
					continue
				}
				absorbed := math.Min(result.Amount*absorbRatio, target.MP/resourcePerDamage)
				if absorbed <= 0 {
					continue
				}
				beforeMP := target.MP
				target.MP = maxFloat(0, target.MP-absorbed*resourcePerDamage)
				result.Amount = maxFloat(0, result.Amount-absorbed)
				appendAction("resource_shield", target.UnitID, target.UnitID, battleSpecializationSourceMeta(hook, nil), map[string]any{
					"resource": "mp", "absorbed": roundTo4(absorbed), "beforeMp": roundTo4(beforeMP), "afterMp": roundTo4(target.MP),
				})
			case "preventlethal":
				remaining := clampFloat(battleSpecializationEffectScalar(target, effect, "remainingHpRatio", "remaining_hp_ratio"), 0, 1) * target.MaxHP
				allowedDamage := maxFloat(0, target.HP-remaining)
				if result.Amount <= allowedDamage {
					continue
				}
				prevented := result.Amount - allowedDamage
				result.Amount = allowedDamage
				appendAction("prevent_lethal", target.UnitID, target.UnitID, battleSpecializationSourceMeta(hook, nil), map[string]any{
					"amount": roundTo4(prevented), "remainingHp": roundTo4(remaining),
				})
			case "heal":
				amount := 0.0
				if battleSpecializationHasValueFormula(effect) {
					value := battleSpecializationEffectScalar(target, effect)
					if firstNonNil(effect["missingHpRatio"], effect["missing_hp_ratio"]) != nil {
						amount = maxFloat(0, target.MaxHP-target.HP) * clampFloat(value, 0, 1)
					} else if firstNonNil(effect["maxHpRatio"], effect["max_hp_ratio"]) != nil {
						amount = target.MaxHP * clampFloat(value, 0, 1)
					} else {
						amount = value
					}
				} else {
					amount = battleEvaluateAmountFormulaWithEnhance(target, 1, parseBattleSkillAmountFormula(firstNonNil(effect["amountFormula"], effect["amount_formula"]), nil), 0)
					amount += target.MaxHP * toFloat64(firstNonNil(effect["maxHpRatio"], effect["max_hp_ratio"]), 0)
					amount += maxFloat(0, target.MaxHP-target.HP) * toFloat64(firstNonNil(effect["missingHpRatio"], effect["missing_hp_ratio"]), 0)
				}
				amount = maxFloat(0, amount)
				result.DeferredHeal += amount
				if amount > 0 {
					result.DeferredHealSources = append(result.DeferredHealSources, battleSpecializationSourceMeta(hook, map[string]any{"amount": roundTo4(amount)}))
				}
			}
		}
		leaveContribution()
	}
	return result
}

func battleQueueUnitDeath(state *battleRuntimeState, unit *battleUnitState, source *battleUnitState) {
	if state == nil || unit == nil || unit.Alive || unit.HP > 0 {
		return
	}
	if battleCombatTrialImmutableTarget(state, unit) {
		battleRestoreCombatTrialImmutableTarget(state, unit)
		return
	}
	unitID := strings.TrimSpace(unit.UnitID)
	if unitID == "" {
		return
	}
	for _, pending := range state.pendingUnitDeaths {
		if strings.EqualFold(strings.TrimSpace(pending.UnitID), unitID) {
			return
		}
	}
	sourceID := ""
	if source != nil {
		sourceID = strings.TrimSpace(source.UnitID)
	}
	state.pendingUnitDeaths = append(state.pendingUnitDeaths, battlePendingUnitDeath{
		UnitID:   unitID,
		SourceID: sourceID,
	})
}

func (s *Server) battleFlushPendingUnitDeathEquipmentHooks(state *battleRuntimeState, now time.Time) []map[string]any {
	if s == nil || state == nil || len(state.pendingUnitDeaths) == 0 {
		return nil
	}
	actions := make([]map[string]any, 0, len(state.pendingUnitDeaths))
	for len(state.pendingUnitDeaths) > 0 {
		pending := state.pendingUnitDeaths[0]
		state.pendingUnitDeaths = state.pendingUnitDeaths[1:]
		dead := battleUnitByID(state, pending.UnitID)
		if dead == nil || dead.Alive || dead.HP > 0 {
			continue
		}
		wearerIDs := make([]string, 0, len(state.Players)+len(state.Enemies))
		collectWearers := func(units []battleUnitState) {
			for index := range units {
				unit := &units[index]
				if unit == nil || !unit.Alive || unit.HP <= 0 || strings.TrimSpace(unit.UnitID) == "" {
					continue
				}
				for hookIndex := 0; hookIndex < unit.equipmentHookCount(); hookIndex++ {
					hook, ok := unit.equipmentHookAt(hookIndex)
					if !ok {
						continue
					}
					if battleNormalizeTrigger(hook.Trigger) == battleNormalizeTrigger(battleEquipmentTriggerUnitDeath) {
						wearerIDs = append(wearerIDs, unit.UnitID)
						break
					}
				}
			}
		}
		collectWearers(state.Players)
		collectWearers(state.Enemies)
		for _, wearerID := range wearerIDs {
			wearer := battleUnitByID(state, wearerID)
			dead = battleUnitByID(state, pending.UnitID)
			if wearer == nil || !wearer.Alive || wearer.HP <= 0 || dead == nil || dead.Alive || dead.HP > 0 {
				continue
			}
			source := battleUnitByID(state, pending.SourceID)
			rows := s.battleTriggerUnitEquipmentHooksWithTarget(
				state,
				now,
				wearer,
				source,
				dead,
				battleEquipmentTriggerUnitDeath,
				0,
				"",
			)
			if len(rows) > 0 {
				actions = append(actions, rows...)
			}
		}
	}
	return actions
}

func (s *Server) battleTriggerBattleStartEquipmentHooks(state *battleRuntimeState, now time.Time) []map[string]any {
	if s == nil || state == nil {
		return nil
	}
	actions := make([]map[string]any, 0, len(state.Players)+len(state.Enemies))
	// Snapshot unit IDs first; resolve via battleUnitByID to get fresh
	// pointers even if state.Players was reallocated by addBattleSummon.
	playerIDs := make([]string, len(state.Players))
	for i := range state.Players {
		playerIDs[i] = state.Players[i].UnitID
	}
	for _, id := range playerIDs {
		unit := battleUnitByID(state, id)
		if unit == nil || !unit.Alive || unit.HP <= 0 {
			continue
		}
		hooks := s.battleTriggerUnitEquipmentHooks(state, now, unit, nil, battleEquipmentTriggerBattleStart, 0, "")
		if len(hooks) > 0 {
			actions = append(actions, hooks...)
		}
	}
	for idx := range state.Enemies {
		unit := &state.Enemies[idx]
		if unit == nil || !unit.Alive || unit.HP <= 0 {
			continue
		}
		hooks := s.battleTriggerUnitEquipmentHooks(state, now, unit, nil, battleEquipmentTriggerBattleStart, 0, "")
		if len(hooks) > 0 {
			actions = append(actions, hooks...)
		}
	}
	return actions
}

func (s *Server) battleTriggerWaveStartEquipmentHooks(state *battleRuntimeState, now time.Time) []map[string]any {
	if s == nil || state == nil {
		return nil
	}
	state.SpecializationHookWaveCounts = map[string]int{}
	waveNumber := float64(state.Wave)
	actions := make([]map[string]any, 0, len(state.Players))
	// Snapshot unit IDs first; resolve via battleUnitByID for fresh pointers.
	playerIDs := make([]string, len(state.Players))
	for i := range state.Players {
		playerIDs[i] = state.Players[i].UnitID
	}
	for _, id := range playerIDs {
		unit := battleUnitByID(state, id)
		if unit == nil || !unit.Alive || unit.HP <= 0 {
			continue
		}
		hooks := s.battleTriggerUnitEquipmentHooks(state, now, unit, nil, battleEquipmentTriggerWaveStart, waveNumber, "")
		if len(hooks) > 0 {
			actions = append(actions, hooks...)
		}
	}
	return actions
}

func (s *Server) battleTriggerBattleEndEquipmentHooks(state *battleRuntimeState, now time.Time) []map[string]any {
	if s == nil || state == nil {
		return nil
	}
	actions := make([]map[string]any, 0, len(state.Players)+len(state.Enemies))
	for idx := range state.Players {
		unit := &state.Players[idx]
		if unit == nil {
			continue
		}
		hooks := s.battleTriggerUnitEquipmentHooks(state, now, unit, nil, battleEquipmentTriggerBattleEnd, 0, "")
		if len(hooks) > 0 {
			actions = append(actions, hooks...)
		}
	}
	for idx := range state.Enemies {
		unit := &state.Enemies[idx]
		if unit == nil {
			continue
		}
		hooks := s.battleTriggerUnitEquipmentHooks(state, now, unit, nil, battleEquipmentTriggerBattleEnd, 0, "")
		if len(hooks) > 0 {
			actions = append(actions, hooks...)
		}
	}
	// Safety net: always recompute final battle stats at battle end so any stale
	// temporary modifiers are cleared even if the originating buff was already detached.
	recomputeAll := func(unit *battleUnitState) {
		if unit == nil {
			return
		}
		beforeStats := cloneFloatMap(unit.Stats)
		beforeMaxHP := unit.MaxHP
		beforeMaxMP := unit.MaxMP
		beforePosition := unit.Position
		beforeSide := normalizeBattleSide(unit.Side)
		beforePerspective := battleUnitPerspectiveSide(unit)
		s.battleRecomputeUnitStats(unit)
		afterSide := normalizeBattleSide(unit.Side)
		afterPerspective := battleUnitPerspectiveSide(unit)
		if battleFloatMapEqual(beforeStats, unit.Stats) && beforeMaxHP == unit.MaxHP && beforeMaxMP == unit.MaxMP && beforePosition == unit.Position && beforeSide == afterSide && beforePerspective == afterPerspective {
			return
		}
		actions = append(actions, map[string]any{
			"kind":            "stat_recalc",
			"targetId":        unit.UnitID,
			"sourceId":        unit.UnitID,
			"reason":          "battle_end_cleanup",
			"stats":           cloneFloatMap(unit.Stats),
			"baseStats":       cloneFloatMap(unit.BaseStats),
			"deltaStats":      cloneFloatMap(unit.StatModifiers),
			"statModifiers":   cloneFloatMap(unit.StatModifiers),
			"statSources":     cloneNestedFloatMap(unit.StatSources),
			"buffs":           battlePublicBuffSlice(unit.Buffs),
			"maxHp":           roundTo4(unit.MaxHP),
			"maxMp":           roundTo4(unit.MaxMP),
			"fromPosition":    beforePosition,
			"position":        unit.Position,
			"basePosition":    unit.BasePosition,
			"side":            unit.Side,
			"baseSide":        unit.BaseSide,
			"perspectiveSide": unit.PerspectiveSide,
			"meta": fiber.Map{
				"trigger": battleEquipmentTriggerBattleEnd,
				"ts":      now.UnixMilli(),
			},
		})
	}
	for idx := range state.Players {
		recomputeAll(&state.Players[idx])
	}
	for idx := range state.Enemies {
		recomputeAll(&state.Enemies[idx])
	}
	return actions
}

type battleSkillActionRuntime struct {
	state           *battleRuntimeState
	actorID         string
	actorSide       string
	now             time.Time
	server          *Server
	forcedTargetIDs map[string]struct{}
	effects         []battleskills.SkillEffect
	actions         []map[string]any
	isBasicAttack   bool
	skillID         string
}

func newBattleSkillActionRuntime(
	state *battleRuntimeState,
	actor *battleUnitState,
	now time.Time,
	server *Server,
	forcedTargets []string,
) *battleSkillActionRuntime {
	forced := make(map[string]struct{}, len(forcedTargets))
	for _, targetID := range forcedTargets {
		targetID = strings.TrimSpace(targetID)
		if targetID == "" {
			continue
		}
		forced[targetID] = struct{}{}
	}
	actorID := ""
	actorSide := ""
	if actor != nil {
		actorID = strings.TrimSpace(actor.UnitID)
		actorSide = strings.TrimSpace(actor.Side)
	}
	return &battleSkillActionRuntime{
		state:           state,
		actorID:         actorID,
		actorSide:       actorSide,
		now:             now,
		server:          server,
		forcedTargetIDs: forced,
		effects:         []battleskills.SkillEffect{},
		actions:         []map[string]any{},
	}
}

func (r *battleSkillActionRuntime) Memory() battleskills.BattleRuntime {
	if r == nil || r.state == nil {
		return battleskills.BattleRuntime{}
	}
	return battleskills.BattleRuntime{
		Wave:             r.state.Wave,
		CompletedBattles: r.state.CompletedBattles,
		TurnCursor:       r.state.TurnCursor,
		Seq:              r.state.Seq,
		RNGIndex:         r.state.RNGIndex,
		StartedAtMS:      r.state.StartedAt,
		TimestampMS:      r.now.UnixMilli(),
		Players:          toBattleSkillUnitRuntimeList(r.state.Players),
		Enemies:          toBattleSkillUnitRuntimeList(r.state.Enemies),
	}
}

func (r *battleSkillActionRuntime) SelectSkillTargets(skill battleskills.SkillRuntime) []battleskills.UnitRuntime {
	if r == nil || r.state == nil {
		return nil
	}
	if len(r.forcedTargetIDs) > 0 {
		out := make([]battleskills.UnitRuntime, 0, len(r.forcedTargetIDs))
		for targetID := range r.forcedTargetIDs {
			target := battleUnitByID(r.state, targetID)
			if target == nil {
				continue
			}
			out = append(out, toBattleSkillUnitRuntime(*target))
		}
		return out
	}
	actor := battleUnitByID(r.state, r.actorID)
	if actor == nil {
		return nil
	}
	targeting := battleSkillTargeting{
		TargetPositions:     append([]string(nil), skill.TargetPositions...),
		SecondarySeek:       skill.SecondarySeek,
		TargetScope:         skill.TargetScope,
		SelectRule:          skill.SelectRule,
		SelectAttrKey:       skill.SelectAttrKey,
		FallbackSelfIfEmpty: skill.FallbackSelfIfEmpty,
		TargetCount: battleSkillTargetCount{
			Base: maxInt(1, skill.TargetCount),
		},
	}
	if len(targeting.TargetPositions) == 0 {
		targeting.TargetPositions = []string{"vanguard"}
	}
	if strings.TrimSpace(targeting.SecondarySeek) == "" {
		targeting.SecondarySeek = "front_to_rear"
	}
	if strings.TrimSpace(targeting.TargetScope) == "" {
		targeting.TargetScope = "enemy"
	}
	if strings.TrimSpace(targeting.SelectRule) == "" {
		targeting.SelectRule = "random"
	}
	targets := r.server.battleSelectTemplateTargets(r.state, actor, battleSkillState{Level: maxInt(1, skill.SkillLevel)}, targeting, skill.IgnoreUntargetable, r.state.randFloat64)
	out := make([]battleskills.UnitRuntime, 0, len(targets))
	for _, target := range targets {
		if target == nil {
			continue
		}
		out = append(out, toBattleSkillUnitRuntime(*target))
	}
	return out
}

func (r *battleSkillActionRuntime) SelectTargets(query battleskills.TargetQuery) []battleskills.UnitRuntime {
	if r == nil || r.state == nil {
		return nil
	}
	candidates := r.queryCandidates(query)
	sortBy := strings.ToLower(strings.TrimSpace(query.SortBy))
	order := strings.ToLower(strings.TrimSpace(query.SortOrder))
	sort.SliceStable(candidates, func(i, j int) bool {
		left := candidates[i]
		right := candidates[j]
		if left == nil || right == nil {
			return false
		}
		leftValue := targetSortValue(left, sortBy)
		rightValue := targetSortValue(right, sortBy)
		if leftValue == rightValue {
			return left.UnitID < right.UnitID
		}
		if order == "desc" {
			return leftValue > rightValue
		}
		return leftValue < rightValue
	})
	limit := query.Limit
	if limit <= 0 || limit > len(candidates) {
		limit = len(candidates)
	}
	side := strings.ToLower(strings.TrimSpace(query.Side))
	if side == "" {
		side = "enemy"
	}
	if r.server != nil && (side == "enemy" || side == "enemies") && limit < len(candidates) {
		actor := battleUnitByID(r.state, r.actorID)
		prioritized, preferredCount := r.server.battlePrioritizeEnemyTargetCandidates(actor, candidates)
		if preferredCount > 0 {
			candidates = prioritized
		}
	}
	out := make([]battleskills.UnitRuntime, 0, limit)
	for idx := 0; idx < limit; idx++ {
		if candidates[idx] == nil {
			continue
		}
		out = append(out, toBattleSkillUnitRuntime(*candidates[idx]))
	}
	return out
}

func (r *battleSkillActionRuntime) queryCandidates(query battleskills.TargetQuery) []*battleUnitState {
	exclude := make(map[string]struct{}, len(query.ExcludeUnitIDs))
	for _, targetID := range query.ExcludeUnitIDs {
		targetID = strings.TrimSpace(targetID)
		if targetID == "" {
			continue
		}
		exclude[targetID] = struct{}{}
	}
	include := make(map[string]struct{}, len(query.IncludeUnitIDs))
	for _, targetID := range query.IncludeUnitIDs {
		targetID = strings.TrimSpace(targetID)
		if targetID == "" {
			continue
		}
		include[targetID] = struct{}{}
	}
	side := strings.ToLower(strings.TrimSpace(query.Side))
	if side == "" {
		side = "enemy"
	}
	actor := battleUnitByID(r.state, r.actorID)
	enemyPerspective := side == "enemy" || side == "enemies"
	relationScope := ""
	if side == "ally" || side == "allies" {
		relationScope = "ally"
	}
	if side == "enemy" || side == "enemies" {
		relationScope = "enemy"
	}
	pool := []*battleUnitState{}
	appendRows := func(rows []battleUnitState) {
		for idx := range rows {
			pool = append(pool, &rows[idx])
		}
	}
	switch side {
	case "self":
		unit := battleUnitByID(r.state, r.actorID)
		if unit != nil {
			pool = append(pool, unit)
		}
	case "ally", "allies":
		appendRows(r.state.Players)
		appendRows(r.state.Enemies)
	case "player", "players":
		appendRows(r.state.Players)
	case "enemy", "enemies":
		appendRows(r.state.Players)
		appendRows(r.state.Enemies)
	case "any", "all":
		appendRows(r.state.Players)
		appendRows(r.state.Enemies)
	default:
		if r.actorSide == "enemy" {
			appendRows(r.state.Players)
		} else {
			appendRows(r.state.Enemies)
		}
	}
	out := make([]*battleUnitState, 0, len(pool))
	for _, unit := range pool {
		if unit == nil {
			continue
		}
		unitID := strings.TrimSpace(unit.UnitID)
		if unitID == "" {
			continue
		}
		if len(include) > 0 {
			if _, ok := include[unitID]; !ok {
				continue
			}
		}
		if _, blocked := exclude[unitID]; blocked {
			continue
		}
		if query.AliveOnly && (!unit.Alive || unit.HP <= 0) {
			continue
		}
		if relationScope != "" {
			if r.server != nil {
				if r.server.battleTargetRelationForActor(r.state, actor, unit) != relationScope {
					continue
				}
			} else {
				unitSide := battleUnitPerspectiveSide(unit)
				actorSide := normalizeBattleSide(r.actorSide)
				if actorSide == "" {
					actorSide = "player"
				}
				if relationScope == "ally" && unitSide != actorSide {
					continue
				}
				if relationScope == "enemy" && unitSide == actorSide {
					continue
				}
			}
		}
		if r.server != nil && enemyPerspective && !query.IgnoreUntargetable {
			if r.server.battleUnitHasWhileActiveEffect(unit, "untargetable_by_enemy", actor) {
				continue
			}
		}
		out = append(out, unit)
	}
	return out
}

func targetSortValue(unit *battleUnitState, sortBy string) float64 {
	if unit == nil {
		return 0
	}
	switch sortBy {
	case "hp_percent", "hpPercent":
		if unit.MaxHP <= 0 {
			return 0
		}
		return unit.HP / unit.MaxHP
	case "mp", "mana":
		return unit.MP
	case "mp_percent", "mpPercent":
		if unit.MaxMP <= 0 {
			return 0
		}
		return unit.MP / unit.MaxMP
	case "speed":
		return unit.Stats["speed"]
	case "hp", "":
		fallthrough
	default:
		return unit.HP
	}
}

func (r *battleSkillActionRuntime) ApplyDamage(action battleskills.DamageAction) battleskills.SkillEffect {
	finishDispatchScope := battleBeginSpecializationDispatchScope(r.state)
	defer finishDispatchScope()
	delayMS := resolveActionDelayMS(action.DelayMS, action.Meta)
	effect := battleskills.SkillEffect{
		Kind:       "damage",
		TargetID:   strings.TrimSpace(action.TargetID),
		SourceID:   defaultSkillSourceID(strings.TrimSpace(action.SourceID), r.actorID),
		Amount:     maxFloat(0, action.Amount),
		Crit:       action.Crit,
		DamageType: normalizeBattleDamageType(action.DamageType),
		Reason:     strings.TrimSpace(action.Reason),
		Meta:       mergeActionMeta(cloneAnyMap(action.Meta), map[string]any{"delayMs": delayMS}),
		Applied:    true,
	}
	target := battleUnitByID(r.state, effect.TargetID)
	source := battleUnitByID(r.state, effect.SourceID)
	arenaGuarded := false
	if r.server != nil && target != nil {
		var guardRows []map[string]any
		arenaGuarded, guardRows = r.server.battleApplyArenaFormatDamageGuard(r.state, target, effect.Amount)
		if arenaGuarded {
			effect.Amount = 0
			effect.Meta = mergeActionMeta(effect.Meta, map[string]any{
				"arenaFormatBlocked": true,
				"arenaFormatId":      r.state.ArenaFormat.ID,
				"arenaFormatNameKey": r.state.ArenaFormat.NameKey,
			})
			r.actions = append(r.actions, guardRows...)
		}
	}
	dispatchEquipmentHooks := !arenaGuarded && !action.SuppressEquipmentHooks && (r.state == nil || r.state.derivedHookDispatchSuppression == 0)
	dispatchBuffHooks := !arenaGuarded && !parseBooleanLoose(effect.Meta["suppressBuffHooks"], false)
	dispatchSourceHooks := dispatchEquipmentHooks && !parseBooleanLoose(effect.Meta["suppressSourceHooks"], false)
	skipFinalDamageModifiers := arenaGuarded || parseBooleanLoose(effect.Meta["skipFinalDamageModifiers"], false)
	combatTrialImmutable := battleCombatTrialImmutableTarget(r.state, target)
	suppressDamageRedirect := arenaGuarded || combatTrialImmutable || parseBooleanLoose(effect.Meta["suppressDamageRedirect"], false)
	beforeHP := 0.0
	beforeAlive := false
	redirectActionRows := []map[string]any{}
	specializationPostDamageRows := []map[string]any{}
	deferredSpecializationHeal := 0.0
	deferredSpecializationHealSources := []map[string]any{}
	pendingProbabilityOutcomes := make([]battlePendingProbabilityOutcome, 0, 2)
	hookContext := &battleEquipmentHookContext{
		SkillID:            strings.TrimSpace(r.skillID),
		CastKind:           "skill",
		DamageType:         effect.DamageType,
		IsBasicAttack:      r.isBasicAttack,
		HitCount:           maxInt(1, int(toInt64Loose(effect.Meta["hitCount"], 1))),
		RawDefense:         toFloat64(effect.Meta["rawDefense"], 0),
		FlatPenetration:    toFloat64(effect.Meta["flatDefensePenetration"], 0),
		PercentPenetration: toFloat64(effect.Meta["percentDefensePenetration"], 0),
	}
	if r.isBasicAttack {
		hookContext.CastKind = "basic_attack"
	}
	if source != nil {
		if skill, _, ok := battleFindUnitSkillByID(source, r.skillID); ok {
			hookContext.SkillCategory = skill.Category
		}
	}
	if target != nil {
		runtimeDamageAmp := 0.0
		runtimeDamageAmpSources := make([]map[string]any, 0, 4)
		if trialAmp := battleCombatTrialDamageAmplification(r.state, effect.SourceID, source, effect.DamageType); trialAmp != 0 {
			action.ExtraDamageAmplification = (1+action.ExtraDamageAmplification)*(1+trialAmp) - 1
			effect.Meta = mergeActionMeta(effect.Meta, map[string]any{
				"combatTrialDamageAmplification": roundTo4(trialAmp),
			})
		}
		if r.server != nil && dispatchSourceHooks {
			sourceAmp, sourceHookRows, sourceAmpSources, sourceProbabilityOutcomes := r.server.battleCollectRuntimeDamageAmpFromEquipmentHooksDeferred(r.state, r.now, source, target, "on_before_deal_damage", effect.Amount, effect.DamageType, hookContext)
			runtimeDamageAmp += sourceAmp
			if len(sourceHookRows) > 0 {
				r.actions = append(r.actions, sourceHookRows...)
			}
			runtimeDamageAmpSources = append(runtimeDamageAmpSources, sourceAmpSources...)
			pendingProbabilityOutcomes = append(pendingProbabilityOutcomes, sourceProbabilityOutcomes...)
		}
		if r.server != nil && dispatchEquipmentHooks {
			targetAmp, targetHookRows, targetAmpSources, targetProbabilityOutcomes := r.server.battleCollectRuntimeDamageAmpFromEquipmentHooksDeferred(r.state, r.now, target, source, "on_before_take_damage", effect.Amount, effect.DamageType, hookContext)
			runtimeDamageAmp += targetAmp
			if len(targetHookRows) > 0 {
				r.actions = append(r.actions, targetHookRows...)
			}
			runtimeDamageAmpSources = append(runtimeDamageAmpSources, targetAmpSources...)
			pendingProbabilityOutcomes = append(pendingProbabilityOutcomes, targetProbabilityOutcomes...)
		}
		if runtimeDamageAmp != 0 {
			action.ExtraDamageAmplification += runtimeDamageAmp
			effect.Meta = mergeActionMeta(effect.Meta, map[string]any{
				"runtimeDamageAmp": roundTo4(runtimeDamageAmp),
			})
		}
		if effect.Crit && source != nil {
			_, critDamage, critDamageResist := battleCritStatsForDamageType(source, target, effect.DamageType)
			criticalMultiplier := 1 + critDamage*maxFloat(0, 1-clampFloat(critDamageResist, 0, 1))
			if criticalMultiplier > 0 {
				effect.Meta = mergeActionMeta(effect.Meta, map[string]any{
					"criticalMultiplier": roundTo4(criticalMultiplier),
				})
			}
		}
		baseRuntimeDamageAmp := toFloat64(effect.Meta["runtimeDamageAmp"], 0)
		damageBeforeBuffHooks := effect.Amount
		if skipFinalDamageModifiers {
			effect.Amount *= maxFloat(0, 1+action.ExtraDamageAmplification)
		} else {
			damageReductionPierce := battleEnemyOnlyDamageReductionPierce(
				source,
				target,
				action.EnemyOnlyDamageReductionPierce,
				action.EnemyOnlyDamageReductionPierceTargetStatuses,
			)
			effect.Amount = applyBattleDamageStatModifiersWithExtraAmplificationAndResistPierce(
				source,
				target,
				effect.Amount,
				effect.DamageType,
				action.ExtraDamageAmplification,
				damageReductionPierce,
			)
			if damageReductionPierce > 0 {
				effect.Meta = mergeActionMeta(effect.Meta, map[string]any{
					"enemyOnlyDamageReductionPierceApplied": roundTo4(damageReductionPierce),
				})
			}
		}
		if r.server != nil && !skipFinalDamageModifiers {
			effect.Amount = r.server.battleApplyBattlefieldIncomingDamageModifiers(r.state, target, effect.Amount)
			if dispatchSourceHooks {
				adjustedBySource, sourceHookRows, sourceAmpSources := r.server.battleApplyBeforeDealDamageBuffHooks(r.state, r.now, source, target, effect.Amount, effect.DamageType)
				if adjustedBySource != effect.Amount {
					effect.Amount = adjustedBySource
					effect.Meta = mergeActionMeta(effect.Meta, map[string]any{"adjustedBySourceBuff": true})
				}
				runtimeDamageAmpSources = append(runtimeDamageAmpSources, sourceAmpSources...)
				for _, hook := range sourceHookRows {
					r.actions = append(r.actions, map[string]any{
						"kind":     "trigger",
						"targetId": strings.TrimSpace(fmt.Sprintf("%v", hook["targetId"])),
						"sourceId": strings.TrimSpace(fmt.Sprintf("%v", hook["sourceId"])),
						"reason":   "on_before_deal_damage",
						"meta":     hook,
						"delayMs":  int64(0),
					})
				}
			}
			if source != nil && effect.Amount > 0 {
				adjustedByOutput, outputSources := r.server.battleApplyOutgoingBuffModifiers(source, "damage", effect.Amount)
				if adjustedByOutput != effect.Amount {
					effect.Amount = adjustedByOutput
					effect.Meta = mergeActionMeta(effect.Meta, map[string]any{
						"outgoingBuffAdjusted": true,
						"outgoingBuffSources":  outputSources,
					})
				}
			}
			adjusted, hookRows := r.server.battleApplyBeforeTakeDamageBuffHooksByPhase(
				r.state, r.now, source, target, effect.Amount, effect.Amount, effect.DamageType, false,
			)
			if adjusted != effect.Amount {
				effect.Amount = adjusted
				effect.Meta = mergeActionMeta(effect.Meta, map[string]any{"adjustedByBuff": true})
			}
			for _, hook := range hookRows {
				r.actions = append(r.actions, map[string]any{
					"kind":     "trigger",
					"targetId": strings.TrimSpace(fmt.Sprintf("%v", hook["targetId"])),
					"sourceId": strings.TrimSpace(fmt.Sprintf("%v", hook["sourceId"])),
					"reason":   "on_before_take_damage",
					"meta":     hook,
					"delayMs":  int64(0),
				})
			}
			if effect.Amount != damageBeforeBuffHooks && damageBeforeBuffHooks > 0 {
				buffDamageAmp := effect.Amount/damageBeforeBuffHooks - 1
				totalDamageAmp := (1+baseRuntimeDamageAmp)*(1+buffDamageAmp) - 1
				if totalDamageAmp != 0 {
					effect.Meta = mergeActionMeta(effect.Meta, map[string]any{
						"runtimeDamageAmp": roundTo4(totalDamageAmp),
					})
				}
			}
			if len(runtimeDamageAmpSources) > 0 {
				effect.Meta = mergeActionMeta(effect.Meta, map[string]any{
					"runtimeDamageAmpSources": runtimeDamageAmpSources,
				})
			}
		}
		if r.server != nil && dispatchEquipmentHooks && effect.Amount > 0 {
			adjustment := r.server.battleApplySpecializationPreDamageAdjustments(r.state, r.now, target, source, effect.Amount, effect.DamageType, hookContext)
			if adjustment.Amount != effect.Amount {
				effect.Meta = mergeActionMeta(effect.Meta, map[string]any{
					"specializationAdjustedDamage": roundTo4(adjustment.Amount),
				})
			}
			effect.Amount = adjustment.Amount
			r.actions = append(r.actions, adjustment.Actions...)
			if adjustment.DeferredHeal > 0 {
				deferredSpecializationHeal += adjustment.DeferredHeal
				deferredSpecializationHealSources = append(deferredSpecializationHealSources, adjustment.DeferredHealSources...)
			}
		}
		if r.server != nil && !skipFinalDamageModifiers && effect.Amount > 0 {
			// Lethal catches settle after ordinary mitigation and against only the
			// damage that will remain on the protected target after redirection.
			projectedDamage := r.server.battleProjectedDamageAfterRedirect(r.state, target, effect.Amount)
			adjusted, hookRows := r.server.battleApplyBeforeTakeDamageBuffHooksByPhase(
				r.state, r.now, source, target, effect.Amount, projectedDamage, effect.DamageType, true,
			)
			if adjusted != effect.Amount {
				effect.Amount = adjusted
				effect.Meta = mergeActionMeta(effect.Meta, map[string]any{"adjustedByBuff": true})
			}
			for _, hook := range hookRows {
				r.actions = append(r.actions, map[string]any{
					"kind":     "trigger",
					"targetId": strings.TrimSpace(fmt.Sprintf("%v", hook["targetId"])),
					"sourceId": strings.TrimSpace(fmt.Sprintf("%v", hook["sourceId"])),
					"reason":   "on_before_take_damage",
					"meta":     hook,
					"delayMs":  int64(0),
				})
			}
		}
		beforeHP = target.HP
		beforeAlive = target.Alive
		if r.server != nil && !suppressDamageRedirect {
			if redirectTarget, redirectAmount, redirectBuffID, redirectRuleID := r.server.battleFindDamageRedirectToSource(r.state, target, effect.Amount); redirectTarget != nil && redirectAmount > 0 {
				if redirectAmount > effect.Amount {
					redirectAmount = effect.Amount
				}
				redirectShare := redirectAmount
				adjustedRedirect, redirectHookRows := r.server.battleApplyBeforeTakeDamageBuffHooksByPhase(
					r.state, r.now, source, redirectTarget, redirectAmount, redirectAmount, effect.DamageType, true,
				)
				redirectAmount = adjustedRedirect
				for _, hook := range redirectHookRows {
					r.actions = append(r.actions, map[string]any{
						"kind":     "trigger",
						"targetId": strings.TrimSpace(fmt.Sprintf("%v", hook["targetId"])),
						"sourceId": strings.TrimSpace(fmt.Sprintf("%v", hook["sourceId"])),
						"reason":   "on_before_take_damage",
						"meta":     hook,
						"delayMs":  int64(0),
					})
				}
				redirectBeforeHP := redirectTarget.HP
				redirectBeforeAlive := redirectTarget.Alive
				redirectTarget.HP = maxFloat(0, redirectTarget.HP-redirectAmount)
				if redirectTarget.HP <= 0 {
					redirectTarget.Alive = false
				}
				battleCombatTrialRecordDamage(r.state, effect.SourceID, source, redirectTarget, redirectAmount)
				battleRestoreCombatTrialImmutableTarget(r.state, redirectTarget)
				effect.Amount = maxFloat(0, effect.Amount-redirectShare)
				effect.Meta = mergeActionMeta(effect.Meta, map[string]any{
					"redirectedByBuff": redirectBuffID,
					"redirectTargetId": redirectTarget.UnitID,
					"redirectAmount":   roundTo4(redirectShare),
					"redirectDamage":   roundTo4(redirectAmount),
				})
				redirectActionRows = append(redirectActionRows, map[string]any{
					"kind":        "damage_redirect",
					"targetId":    redirectTarget.UnitID,
					"sourceId":    target.UnitID,
					"amount":      roundTo4(redirectAmount),
					"damageType":  effect.DamageType,
					"reason":      battleEffectDamageRedirectSource,
					"beforeHp":    roundTo4(redirectBeforeHP),
					"afterHp":     roundTo4(redirectTarget.HP),
					"beforeAlive": redirectBeforeAlive,
					"afterAlive":  redirectTarget.Alive,
					"delayMs":     delayMS,
					"meta": map[string]any{
						"buffId":        redirectBuffID,
						"ruleId":        redirectRuleID,
						"protectedId":   target.UnitID,
						"originalFrom":  effect.SourceID,
						"redirectShare": roundTo4(redirectShare),
					},
				})
				if redirectBeforeAlive && !redirectTarget.Alive {
					battleQueueUnitDeath(r.state, redirectTarget, source)
				}
			}
		}
		battleCombatTrialRecordDamage(r.state, effect.SourceID, source, target, effect.Amount)
		if combatTrialImmutable {
			target.HP = target.MaxHP
			target.Alive = true
		} else {
			target.HP = maxFloat(0, target.HP-effect.Amount)
			if target.HP <= 0 {
				target.Alive = false
			}
		}
		if deferredSpecializationHeal > 0 {
			if r.server != nil {
				deferredSpecializationHeal, _ = r.server.battleApplyOutgoingBuffModifiers(target, "heal", deferredSpecializationHeal)
			}
			beforeHeal := target.HP
			target.HP = clampFloat(target.HP+deferredSpecializationHeal, 0, target.MaxHP)
			if target.HP > 0 {
				target.Alive = true
			}
			battleCombatTrialRecordHeal(r.state, target.UnitID, target, target, target.HP-beforeHeal)
			healMeta := map[string]any{"specialization": true, "sources": deferredSpecializationHealSources}
			if len(deferredSpecializationHealSources) > 0 {
				for _, key := range []string{"sourceKind", "sourceId", "nodeId", "sourceNameKey", "hookId"} {
					if value := deferredSpecializationHealSources[0][key]; value != nil {
						healMeta[key] = value
					}
				}
			}
			healRow := map[string]any{
				"kind": "heal", "targetId": target.UnitID, "sourceId": target.UnitID,
				"amount": roundTo4(target.HP - beforeHeal), "beforeHp": roundTo4(beforeHeal), "afterHp": roundTo4(target.HP),
				"reason": "specialization_prevent_lethal_recovery", "delayMs": int64(0),
				"meta": healMeta,
			}
			for _, key := range []string{"sourceKind", "nodeId", "sourceNameKey"} {
				if value := healMeta[key]; value != nil {
					healRow[key] = value
				}
			}
			specializationPostDamageRows = append(specializationPostDamageRows, healRow)
		}
		if r.server != nil {
			if dispatchEquipmentHooks {
				equipmentHookRows := r.server.battleTriggerUnitEquipmentHooksWithContext(r.state, r.now, target, source, target, "on_after_take_damage", effect.Amount, effect.DamageType, hookContext)
				if len(equipmentHookRows) > 0 {
					r.actions = append(r.actions, equipmentHookRows...)
				}
			}
			if dispatchBuffHooks {
				targetHookRows := r.server.battleTriggerUnitBuffHooksWithSource(r.state, r.now, target, source, "on_after_take_damage")
				if len(targetHookRows) > 0 {
					r.actions = append(r.actions, targetHookRows...)
				}
			}
			if dispatchEquipmentHooks {
				allyHookRows := r.server.battleTriggerAllyAfterTakeDamageEquipmentHooks(r.state, r.now, target, source, effect.Amount, effect.DamageType)
				if len(allyHookRows) > 0 {
					r.actions = append(r.actions, allyHookRows...)
				}
			}
			if source != nil {
				if dispatchSourceHooks {
					sourceEquipmentHookRows := r.server.battleTriggerUnitEquipmentHooksWithContext(r.state, r.now, source, target, target, "on_after_deal_damage", effect.Amount, effect.DamageType, hookContext)
					if len(sourceEquipmentHookRows) > 0 {
						r.actions = append(r.actions, sourceEquipmentHookRows...)
					}
				}
				if dispatchSourceHooks {
					sourceHookRows := r.server.battleTriggerUnitBuffHooks(r.state, r.now, source, "on_after_deal_damage")
					if len(sourceHookRows) > 0 {
						r.actions = append(r.actions, sourceHookRows...)
					}
					critTrigger := "on_after_deal_no_crit"
					if effect.Crit {
						critTrigger = "on_after_deal_crit"
					}
					sourceCritHookRows := r.server.battleTriggerUnitEquipmentHooksWithContext(r.state, r.now, source, target, target, critTrigger, effect.Amount, effect.DamageType, hookContext)
					if len(sourceCritHookRows) > 0 {
						r.actions = append(r.actions, sourceCritHookRows...)
					}
					if r.isBasicAttack {
						basicAtkHookRows := r.server.battleTriggerUnitEquipmentHooksWithContext(r.state, r.now, source, target, target, "on_after_basic_attack_deal_damage", effect.Amount, effect.DamageType, hookContext)
						if len(basicAtkHookRows) > 0 {
							r.actions = append(r.actions, basicAtkHookRows...)
						}
					}
				}
			}
		}
	}
	// Fire on_kill after all after_damage hooks (covers both regular kills and execute kills).
	if source != nil && beforeAlive && target != nil && !target.Alive {
		if r.server != nil {
			if dispatchSourceHooks {
				killHookRows := r.server.battleTriggerUnitEquipmentHooksWithContext(r.state, r.now, source, target, target, battleEquipmentTriggerOnKill, effect.Amount, effect.DamageType, hookContext)
				if len(killHookRows) > 0 {
					r.actions = append(r.actions, killHookRows...)
				}
			}
			if dispatchSourceHooks && dispatchBuffHooks {
				killBuffHookRows := r.server.battleTriggerUnitBuffHooks(r.state, r.now, source, battleEquipmentTriggerOnKill)
				if len(killBuffHookRows) > 0 {
					r.actions = append(r.actions, killBuffHookRows...)
				}
			}
		}
	}
	if beforeAlive && target != nil && !target.Alive {
		battleQueueUnitDeath(r.state, target, source)
	}
	r.recordEffect(effect)
	actionRow := map[string]any{
		"kind":        "damage",
		"targetId":    effect.TargetID,
		"sourceId":    effect.SourceID,
		"amount":      roundTo4(effect.Amount),
		"crit":        effect.Crit,
		"damageType":  effect.DamageType,
		"reason":      effect.Reason,
		"beforeHp":    roundTo4(beforeHP),
		"afterHp":     roundTo4(readTargetHP(target, beforeHP)),
		"beforeAlive": beforeAlive,
		"afterAlive":  readTargetAlive(target, beforeAlive),
		"delayMs":     delayMS,
	}
	if displaySourceID := strings.TrimSpace(action.DisplaySourceID); displaySourceID != "" {
		actionRow["visualSourceId"] = displaySourceID
	}
	if action.ExtraDamageAmplification != 0 {
		actionRow["extraDamageAmplification"] = roundTo4(action.ExtraDamageAmplification)
	}
	if len(effect.Meta) > 0 {
		actionRow["meta"] = effect.Meta
	}
	if len(redirectActionRows) > 0 {
		r.actions = append(r.actions, redirectActionRows...)
	}
	r.actions = append(r.actions, actionRow)
	if len(specializationPostDamageRows) > 0 {
		r.actions = append(r.actions, specializationPostDamageRows...)
	}
	if r.server != nil {
		r.actions = append(r.actions, r.server.battleFinalizePendingProbabilityOutcomes(r.state, r.now, pendingProbabilityOutcomes)...)
	}
	return effect
}

func (r *battleSkillActionRuntime) ApplyHeal(action battleskills.HealAction) battleskills.SkillEffect {
	delayMS := resolveActionDelayMS(action.DelayMS, action.Meta)
	effect := battleskills.SkillEffect{
		Kind:     "heal",
		TargetID: strings.TrimSpace(action.TargetID),
		SourceID: defaultSkillSourceID(strings.TrimSpace(action.SourceID), r.actorID),
		Amount:   maxFloat(0, action.Amount),
		Reason:   strings.TrimSpace(action.Reason),
		Meta:     mergeActionMeta(cloneAnyMap(action.Meta), map[string]any{"delayMs": delayMS}),
		Applied:  true,
	}
	target := battleUnitByID(r.state, effect.TargetID)
	source := battleUnitByID(r.state, effect.SourceID)
	beforeHP := 0.0
	beforeAlive := false
	effectiveHeal := 0.0
	if target != nil {
		if r.server != nil {
			adjustedBySource, outputSources := r.server.battleApplyOutgoingBuffModifiers(source, "heal", effect.Amount)
			if adjustedBySource != effect.Amount {
				effect.Amount = adjustedBySource
				effect.Meta = mergeActionMeta(effect.Meta, map[string]any{
					"outgoingBuffAdjusted": true,
					"outgoingBuffSources":  outputSources,
				})
			}
			adjusted, hookRows := r.server.battleApplyBeforeHealBuffHooks(r.state, r.now, source, target, effect.Amount)
			if adjusted != effect.Amount {
				effect.Amount = adjusted
				effect.Meta = mergeActionMeta(effect.Meta, map[string]any{"adjustedByBuff": true})
			}
			for _, hook := range hookRows {
				r.actions = append(r.actions, map[string]any{
					"kind":     "trigger",
					"targetId": strings.TrimSpace(fmt.Sprintf("%v", hook["targetId"])),
					"sourceId": strings.TrimSpace(fmt.Sprintf("%v", hook["sourceId"])),
					"reason":   "on_before_heal",
					"meta":     hook,
					"delayMs":  int64(0),
				})
			}
		}
		beforeHP = target.HP
		beforeAlive = target.Alive
		target.HP = clampFloat(target.HP+effect.Amount, 0, target.MaxHP)
		effectiveHeal = maxFloat(0, target.HP-beforeHP)
		if target.HP > 0 {
			target.Alive = true
		}
	}
	battleCombatTrialRecordHeal(r.state, effect.SourceID, source, target, effectiveHeal)
	r.recordEffect(effect)
	row := map[string]any{
		"kind":        "heal",
		"targetId":    effect.TargetID,
		"sourceId":    effect.SourceID,
		"amount":      roundTo4(effect.Amount),
		"reason":      effect.Reason,
		"beforeHp":    roundTo4(beforeHP),
		"afterHp":     roundTo4(readTargetHP(target, beforeHP)),
		"beforeAlive": beforeAlive,
		"afterAlive":  readTargetAlive(target, beforeAlive),
		"delayMs":     delayMS,
	}
	if len(effect.Meta) > 0 {
		row["meta"] = effect.Meta
	}
	r.actions = append(r.actions, row)
	if r.server != nil && effectiveHeal > 0 && effect.SourceID != "" && effect.TargetID != "" && effect.SourceID != effect.TargetID {
		healer := battleUnitByID(r.state, effect.SourceID)
		healTarget := battleUnitByID(r.state, effect.TargetID)
		if healer != nil && healTarget != nil {
			dealHealHookRows := r.server.battleTriggerUnitEquipmentHooks(r.state, r.now, healer, healTarget, "on_after_deal_heal", effectiveHeal, "")
			if len(dealHealHookRows) > 0 {
				r.actions = append(r.actions, dealHealHookRows...)
			}
		}
	}
	return effect
}

func (r *battleSkillActionRuntime) ApplyResource(action battleskills.ResourceAction) battleskills.SkillEffect {
	delayMS := resolveActionDelayMS(action.DelayMS, action.Meta)
	resource := strings.ToLower(strings.TrimSpace(action.Resource))
	effect := battleskills.SkillEffect{
		Kind:     "resource",
		TargetID: strings.TrimSpace(action.TargetID),
		SourceID: defaultSkillSourceID(strings.TrimSpace(action.SourceID), r.actorID),
		Resource: resource,
		Amount:   action.Amount,
		Reason:   strings.TrimSpace(action.Reason),
		Meta:     mergeActionMeta(cloneAnyMap(action.Meta), map[string]any{"delayMs": delayMS}),
		Applied:  true,
	}
	target := battleUnitByID(r.state, effect.TargetID)
	source := battleUnitByID(r.state, effect.SourceID)
	beforeHP := 0.0
	afterHP := 0.0
	beforeMP := 0.0
	afterMP := 0.0
	beforeAlive := false
	afterAlive := false
	if resource == "hp" && effect.Amount > 0 && r.server != nil {
		adjusted, outputSources := r.server.battleApplyOutgoingBuffModifiers(source, "heal", effect.Amount)
		if adjusted != effect.Amount {
			effect.Amount = adjusted
			effect.Meta = mergeActionMeta(effect.Meta, map[string]any{
				"outgoingBuffAdjusted": true,
				"outgoingBuffSources":  outputSources,
			})
		}
	}
	if target != nil {
		beforeHP = target.HP
		beforeMP = target.MP
		beforeAlive = target.Alive
		switch resource {
		case "hp":
			target.HP = clampFloat(target.HP+effect.Amount, 0, target.MaxHP)
			if target.HP <= 0 {
				target.Alive = false
			} else {
				target.Alive = true
			}
		case "mp":
			target.MP = clampFloat(target.MP+effect.Amount, 0, target.MaxMP)
		}
		battleRestoreCombatTrialImmutableTarget(r.state, target)
		afterHP = target.HP
		afterMP = target.MP
		afterAlive = target.Alive
		if beforeAlive && !afterAlive {
			battleQueueUnitDeath(r.state, target, battleUnitByID(r.state, effect.SourceID))
		}
		if resource == "hp" && effect.Amount > 0 {
			battleCombatTrialRecordHeal(r.state, effect.SourceID, source, target, maxFloat(0, afterHP-beforeHP))
		}
	}
	r.recordEffect(effect)
	row := map[string]any{
		"kind":        "resource",
		"targetId":    effect.TargetID,
		"sourceId":    effect.SourceID,
		"resource":    effect.Resource,
		"amount":      roundTo4(effect.Amount),
		"reason":      effect.Reason,
		"beforeHp":    roundTo4(beforeHP),
		"afterHp":     roundTo4(afterHP),
		"beforeMp":    roundTo4(beforeMP),
		"afterMp":     roundTo4(afterMP),
		"beforeAlive": beforeAlive,
		"afterAlive":  afterAlive,
		"delayMs":     delayMS,
	}
	if len(effect.Meta) > 0 {
		row["meta"] = effect.Meta
	}
	r.actions = append(r.actions, row)
	return effect
}

func (r *battleSkillActionRuntime) ApplyBuff(action battleskills.BuffAction) battleskills.SkillEffect {
	delayMS := resolveActionDelayMS(action.DelayMS, action.Meta)
	effect := battleskills.SkillEffect{
		Kind:              "buff",
		TargetID:          strings.TrimSpace(action.TargetID),
		SourceID:          defaultSkillSourceID(strings.TrimSpace(action.SourceID), r.actorID),
		BuffID:            strings.TrimSpace(action.BuffID),
		BuffDurationTurns: action.DurationTurns,
		BuffPayload:       action.Payload,
		Reason:            strings.TrimSpace(action.Reason),
		Meta:              mergeActionMeta(cloneAnyMap(action.Meta), map[string]any{"delayMs": delayMS}),
		Applied:           true,
	}
	if effect.BuffDurationTurns <= 0 && effect.BuffID != "" && r.server != nil {
		effect.BuffDurationTurns = r.server.battleBuffDurationByIDTurns(effect.BuffID)
	}
	target := battleUnitByID(r.state, effect.TargetID)
	battleCombatTrialRegisterPlayerSource(r.state, battleUnitByID(r.state, effect.SourceID))
	isNewAttach := false
	buffApplied := false
	if target != nil && effect.BuffID != "" {
		stackPolicy := "refresh_duration"
		maxStacks := 0
		if r.server != nil {
			stackPolicy = r.server.battleBuffStackPolicyByID(effect.BuffID)
			maxStacks = r.server.battleBuffMaxStacksByID(effect.BuffID)
		}
		isNewAttach = battleApplyOrRefreshBuffForTurn(
			target,
			effect.BuffID,
			effect.BuffDurationTurns,
			effect.BuffPayload,
			effect.SourceID,
			stackPolicy,
			maxStacks,
			battleBuffApplicationTurnSerial(r.state, effect.Reason),
		)
		buffApplied = true
		if r.server != nil {
			beforeStats := cloneFloatMap(target.Stats)
			beforeMaxHP := target.MaxHP
			beforeMaxMP := target.MaxMP
			beforePosition := target.Position
			beforeSide := normalizeBattleSide(target.Side)
			beforePerspective := battleUnitPerspectiveSide(target)
			r.server.battleRecomputeUnitStats(target)
			battleRestoreCombatTrialImmutableTarget(r.state, target)
			afterSide := normalizeBattleSide(target.Side)
			afterPerspective := battleUnitPerspectiveSide(target)
			if !battleFloatMapEqual(beforeStats, target.Stats) || beforeMaxHP != target.MaxHP || beforeMaxMP != target.MaxMP || beforePosition != target.Position || beforeSide != afterSide || beforePerspective != afterPerspective {
				r.actions = append(r.actions, map[string]any{
					"kind":            "stat_recalc",
					"targetId":        target.UnitID,
					"sourceId":        effect.SourceID,
					"reason":          firstNonNil(effect.Reason, "buff_attach"),
					"stats":           cloneFloatMap(target.Stats),
					"baseStats":       cloneFloatMap(target.BaseStats),
					"deltaStats":      cloneFloatMap(target.StatModifiers),
					"statModifiers":   cloneFloatMap(target.StatModifiers),
					"statSources":     cloneNestedFloatMap(target.StatSources),
					"buffs":           battlePublicBuffSlice(target.Buffs),
					"maxHp":           roundTo4(target.MaxHP),
					"maxMp":           roundTo4(target.MaxMP),
					"fromPosition":    beforePosition,
					"position":        target.Position,
					"basePosition":    target.BasePosition,
					"side":            target.Side,
					"baseSide":        target.BaseSide,
					"perspectiveSide": target.PerspectiveSide,
					"delayMs":         delayMS,
					"meta":            cloneAnyMap(effect.Meta),
				})
			}
			if isNewAttach {
				hookActions := r.server.battleTriggerUnitBuffHooks(r.state, r.now, target, "on_attach", effect.BuffID)
				if len(hookActions) > 0 {
					r.actions = append(r.actions, hookActions...)
				}
			}
		}
	}
	r.recordEffect(effect)
	row := map[string]any{
		"kind":              "buff",
		"targetId":          effect.TargetID,
		"sourceId":          effect.SourceID,
		"buffId":            effect.BuffID,
		"buffDurationTurns": effect.BuffDurationTurns,
		"buffPayload":       effect.BuffPayload,
		"reason":            effect.Reason,
		"delayMs":           delayMS,
	}
	if len(effect.Meta) > 0 {
		row["meta"] = effect.Meta
	}
	r.actions = append(r.actions, row)
	if buffApplied && r.server != nil {
		source := battleUnitByID(r.state, effect.SourceID)
		if source != nil {
			afterApplyActions := r.server.battleTriggerAfterApplyBuffEquipmentHooks(r.state, r.now, source, target, effect.BuffID, isNewAttach)
			if len(afterApplyActions) > 0 {
				r.actions = append(r.actions, afterApplyActions...)
			}
		}
	}
	return effect
}

func (r *battleSkillActionRuntime) ApplySummon(action battleskills.SummonAction) battleskills.SkillEffect {
	delayMS := resolveActionDelayMS(action.DelayMS, action.Meta)
	summon := action.Summon
	if strings.TrimSpace(summon.Side) == "" {
		summon.Side = r.actorSide
	}
	effect := battleskills.SkillEffect{
		Kind:     "summon",
		SourceID: defaultSkillSourceID(strings.TrimSpace(action.SourceID), r.actorID),
		Summon:   &summon,
		Reason:   strings.TrimSpace(action.Reason),
		Meta:     mergeActionMeta(cloneAnyMap(action.Meta), map[string]any{"delayMs": delayMS}),
		Applied:  true,
	}
	applied := addBattleSummon(r.state, r.actorSide, &summon, r.server, r.now)
	if !applied {
		effect.Meta = mergeActionMeta(effect.Meta, map[string]any{"applied": false})
	} else {
		effect.Meta = mergeActionMeta(effect.Meta, map[string]any{"applied": true})
		battleCombatTrialRegisterPlayerSource(r.state, battleUnitByID(r.state, summon.UnitID))
	}
	r.recordEffect(effect)
	row := map[string]any{
		"kind":     "summon",
		"sourceId": effect.SourceID,
		"summon":   summon,
		"reason":   effect.Reason,
		"delayMs":  delayMS,
	}
	if len(effect.Meta) > 0 {
		row["meta"] = effect.Meta
	}
	r.actions = append(r.actions, row)
	return effect
}

func (r *battleSkillActionRuntime) AppliedEffects() []battleskills.SkillEffect {
	if len(r.effects) == 0 {
		return nil
	}
	out := make([]battleskills.SkillEffect, 0, len(r.effects))
	out = append(out, r.effects...)
	return out
}

func (r *battleSkillActionRuntime) ActionLog() []map[string]any {
	if len(r.actions) == 0 {
		return nil
	}
	out := make([]map[string]any, 0, len(r.actions))
	for _, row := range r.actions {
		out = append(out, cloneAnyMap(row))
	}
	return out
}

func (r *battleSkillActionRuntime) recordEffect(effect battleskills.SkillEffect) {
	r.effects = append(r.effects, effect)
}

func readTargetHP(target *battleUnitState, fallback float64) float64 {
	if target == nil {
		return fallback
	}
	return target.HP
}

func readTargetAlive(target *battleUnitState, fallback bool) bool {
	if target == nil {
		return fallback
	}
	return target.Alive
}

func defaultSkillSourceID(sourceID, fallback string) string {
	if strings.TrimSpace(sourceID) != "" {
		return strings.TrimSpace(sourceID)
	}
	return strings.TrimSpace(fallback)
}

func resolveActionDelayMS(explicit int64, meta map[string]any) int64 {
	if explicit > 0 {
		return explicit
	}
	if len(meta) == 0 {
		return 0
	}
	return maxInt64(0, toInt64Loose(meta["delayMs"], 0))
}

func mergeActionMeta(base map[string]any, patch map[string]any) map[string]any {
	if len(base) == 0 && len(patch) == 0 {
		return nil
	}
	out := cloneAnyMap(base)
	if out == nil {
		out = map[string]any{}
	}
	for key, value := range patch {
		out[key] = value
	}
	return out
}

func battleActionRowFromEffect(effect battleskills.SkillEffect, target *battleUnitState) map[string]any {
	row := map[string]any{
		"kind":       strings.TrimSpace(effect.Kind),
		"targetId":   strings.TrimSpace(effect.TargetID),
		"sourceId":   strings.TrimSpace(effect.SourceID),
		"amount":     roundTo4(effect.Amount),
		"crit":       effect.Crit,
		"damageType": strings.TrimSpace(effect.DamageType),
		"resource":   strings.TrimSpace(effect.Resource),
		"buffId":     strings.TrimSpace(effect.BuffID),
		"reason":     strings.TrimSpace(effect.Reason),
	}
	if len(effect.Meta) > 0 {
		row["delayMs"] = maxInt64(0, toInt64Loose(effect.Meta["delayMs"], 0))
	}
	if effect.BuffDurationTurns != 0 {
		row["buffDurationTurns"] = effect.BuffDurationTurns
	}
	if len(effect.BuffPayload) > 0 {
		row["buffPayload"] = effect.BuffPayload
	}
	if effect.Summon != nil {
		row["summon"] = effect.Summon
	}
	if len(effect.Meta) > 0 {
		row["meta"] = effect.Meta
	}
	if target != nil {
		row["afterHp"] = roundTo4(target.HP)
		row["afterMp"] = roundTo4(target.MP)
		row["afterAlive"] = target.Alive
	}
	return row
}

type battleSkillExecution struct {
	Targets []map[string]any
	Effects []map[string]any
	Actions []map[string]any
	Meta    map[string]any
}

func normalizeBattleDamageSpecForRuntime(spec battleDamageSpec, fallbackDamageType string, fallbackAmount battleSkillAmountFormula) battleDamageSpec {
	out := spec
	if strings.TrimSpace(out.DamageType) == "" {
		out.DamageType = normalizeBattleDamageType(fallbackDamageType)
	} else {
		out.DamageType = normalizeBattleDamageType(out.DamageType)
	}
	if out.HitCount.Base <= 0 {
		out.HitCount.Base = 1
	}
	if math.IsNaN(out.AmountFormula.BaseAdd) || math.IsInf(out.AmountFormula.BaseAdd, 0) {
		out.AmountFormula.BaseAdd = 0
	}
	if len(out.AmountFormula.Terms) == 0 {
		out.AmountFormula = battleSkillAmountFormula{
			BaseAdd: fallbackAmount.BaseAdd,
			Terms:   append([]battleAttackTerm(nil), fallbackAmount.Terms...),
		}
	}
	out.HitDamageScaling = normalizeBattleHitDamageScaling(out.HitDamageScaling)
	out.BonusAgainstTargetTypes = normalizeBattleTargetTypeBonuses(out.BonusAgainstTargetTypes)
	out.EnemyOnlyDamageReductionPierce = normalizeBattleDamageReductionPierce(out.EnemyOnlyDamageReductionPierce)
	out.EnemyOnlyDamageReductionPierceTargetStatuses = parseBattleStringSlice(out.EnemyOnlyDamageReductionPierceTargetStatuses)
	return out
}

func normalizeBattleHitDamageScaling(raw *battleHitDamageScaling) *battleHitDamageScaling {
	if !validBattleHitDamageScaling(raw) {
		return nil
	}
	out := *raw
	return &out
}

func validBattleHitDamageScaling(raw *battleHitDamageScaling) bool {
	if raw == nil || raw.Kind != battleHitDamageScalingKindPoweredExponentialDecay {
		return false
	}
	if math.IsNaN(raw.SecondHitMultiplier) || math.IsInf(raw.SecondHitMultiplier, 0) ||
		raw.SecondHitMultiplier <= 0 || raw.SecondHitMultiplier >= 1 {
		return false
	}
	if math.IsNaN(raw.CurvePower) || math.IsInf(raw.CurvePower, 0) || raw.CurvePower < 1 || raw.CurvePower > 3 {
		return false
	}
	return true
}

func battleResolveHitDamageMultiplier(raw *battleHitDamageScaling, hitIndex int) float64 {
	if !validBattleHitDamageScaling(raw) || hitIndex <= 1 {
		return 1
	}
	multiplier := math.Pow(raw.SecondHitMultiplier, math.Pow(float64(hitIndex-1), raw.CurvePower))
	if math.IsNaN(multiplier) || math.IsInf(multiplier, 0) || multiplier < 0 || multiplier > 1 {
		return 0
	}
	return multiplier
}

func battleEvaluateAmountFormula(actor *battleUnitState, skillLevel int, formula battleSkillAmountFormula) float64 {
	return battleEvaluateAmountFormulaWithExtra(actor, skillLevel, formula, nil)
}

func battleEvaluateAmountFormulaWithExtra(actor *battleUnitState, skillLevel int, formula battleSkillAmountFormula, extra map[string]float64) float64 {
	base := formula.BaseAdd + evaluateBattleSkillFormulaWithExtra(actor, formula.Terms, skillLevel, extra)
	if math.IsNaN(base) || math.IsInf(base, 0) {
		return 0
	}
	return base
}

func battleEvaluateAmountFormulaWithEnhance(actor *battleUnitState, skillLevel int, formula battleSkillAmountFormula, sourceEnhanceLevel int) float64 {
	base := formula.BaseAdd
	for _, term := range formula.Terms {
		attrKey := strings.TrimSpace(term.AttrKey)
		if attrKey == "" {
			continue
		}
		coefficient := term.Coefficient + term.PerSkillLevel*float64(skillLevel-1) + term.PerEnhanceLevel*float64(sourceEnhanceLevel)
		if math.IsNaN(coefficient) || math.IsInf(coefficient, 0) {
			continue
		}
		attrValue := 0.0
		if attrKey == "equipment_enhance_level" {
			attrValue = float64(sourceEnhanceLevel)
		} else if attrKey == "skill_level" {
			attrValue = float64(maxInt(1, skillLevel))
		} else if attrKey == "current_hp" && actor != nil {
			attrValue = actor.HP
		} else if attrKey == "current_mp" && actor != nil {
			attrValue = actor.MP
		} else if attrKey == "max_hp" && actor != nil {
			attrValue = actor.MaxHP
		} else if attrKey == "max_mp" && actor != nil {
			attrValue = actor.MaxMP
		} else if actor != nil && actor.Stats != nil {
			attrValue = actor.Stats[attrKey]
		}
		base += attrValue * coefficient
	}
	if math.IsNaN(base) || math.IsInf(base, 0) {
		return 0
	}
	return base
}

func battleResolveDamageHitCount(actor *battleUnitState, skillLevel int, spec battleDamageSpec) int {
	base := maxInt(1, spec.HitCount.Base)
	extra := evaluateBattleSkillFormula(actor, spec.HitCount.Formula, skillLevel)
	count := int(math.Floor(float64(base) + extra))
	if count < 1 {
		return 1
	}
	if spec.HitCount.MaxSet && spec.HitCount.Max > 0 && count > spec.HitCount.Max {
		count = spec.HitCount.Max
	}
	if count > 64 {
		count = 64
	}
	return count
}

func normalizeBattleTargetTypeBonuses(raw map[string]float64) map[string]float64 {
	if len(raw) == 0 {
		return nil
	}
	out := make(map[string]float64, len(raw))
	for key, value := range raw {
		normalized := strings.ToLower(strings.TrimSpace(key))
		if normalized == "" || math.IsNaN(value) || math.IsInf(value, 0) {
			continue
		}
		out[normalized] = value
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func battleTargetTypeBonus(target *battleUnitState, spec battleDamageSpec) float64 {
	if target == nil || len(spec.BonusAgainstTargetTypes) == 0 {
		return 0
	}
	targetType := strings.ToLower(strings.TrimSpace(target.EnemyType))
	if targetType == "" {
		targetType = "normal"
	}
	if bonus, ok := spec.BonusAgainstTargetTypes[targetType]; ok {
		return bonus
	}
	return 0
}

type battleDamageChannelProfile struct {
	ID                         string
	IsMagical                  bool
	DefenseStatKey             string
	DamageAmplificationStatKey string
	DamageResistStatKey        string
	CritChanceStatKey          string
	CritDamageStatKey          string
	FlatPenStatKeys            []string
	PercentPenStatKeys         []string
	FlatPenSourceLabel         string
	PercentPenSourceLabel      string
	SpecFlatPenFormula         func(spec battleDamageSpec) battleSkillAmountFormula
	SpecPercentPenFormula      func(spec battleDamageSpec) battleSkillAmountFormula
}

var battlePhysicalDamageChannelProfile = battleDamageChannelProfile{
	ID:                         "physical",
	IsMagical:                  false,
	DefenseStatKey:             "physics_defence",
	DamageAmplificationStatKey: "physics_damage_amplification",
	DamageResistStatKey:        "physics_damage_resist",
	CritChanceStatKey:          "crit_chance",
	CritDamageStatKey:          "crit_damage",
	FlatPenStatKeys:            []string{"physics_flat_defense_penetration"},
	PercentPenStatKeys:         []string{"physics_percent_defense_penetration"},
	FlatPenSourceLabel:         "physics_flat_defense_penetration",
	PercentPenSourceLabel:      "physics_percent_defense_penetration",
	SpecFlatPenFormula: func(spec battleDamageSpec) battleSkillAmountFormula {
		return spec.PhysicalFlatDefensePenetrationFormula
	},
	SpecPercentPenFormula: func(spec battleDamageSpec) battleSkillAmountFormula {
		return spec.PhysicalPercentDefensePenetrationFormula
	},
}

var battleMagicalDamageChannelProfile = battleDamageChannelProfile{
	ID:                         "magical",
	IsMagical:                  true,
	DefenseStatKey:             "magical_defence",
	DamageAmplificationStatKey: "magical_damage_amplification",
	DamageResistStatKey:        "magical_damage_resist",
	CritChanceStatKey:          "magic_crit_chance",
	CritDamageStatKey:          "magic_crit_damage",
	FlatPenStatKeys:            []string{"magicial_flat_defense_penetration", "magical_flat_defense_penetration"},
	PercentPenStatKeys:         []string{"magicial_percent_defense_penetration", "magical_percent_defense_penetration"},
	SpecFlatPenFormula: func(spec battleDamageSpec) battleSkillAmountFormula {
		return spec.MagicalFlatDefensePenetrationFormula
	},
	SpecPercentPenFormula: func(spec battleDamageSpec) battleSkillAmountFormula {
		return spec.MagicalPercentDefensePenetrationFormula
	},
}

func battleDamageChannelProfileForType(damageType string) battleDamageChannelProfile {
	if normalizeBattleDamageType(damageType) == "magical" {
		return battleMagicalDamageChannelProfile
	}
	return battlePhysicalDamageChannelProfile
}

func battleDamageChannelProfileByIsMagical(isMagical bool) battleDamageChannelProfile {
	if isMagical {
		return battleMagicalDamageChannelProfile
	}
	return battlePhysicalDamageChannelProfile
}

func battleCollectPenetrationWithSource(stats map[string]float64, keys []string, defaultLabel string) (float64, string) {
	if len(keys) == 0 {
		return 0, ""
	}
	total := 0.0
	labels := make([]string, 0, len(keys))
	for _, key := range keys {
		value := stats[key]
		total += value
		if value != 0 {
			labels = append(labels, key)
		}
	}
	if len(labels) == 0 {
		return total, defaultLabel
	}
	if len(labels) == 1 {
		return total, labels[0]
	}
	return total, strings.Join(labels, "+")
}

func battleResolveAttackerPenetration(actor *battleUnitState, damageType string) (flat float64, percent float64, flatSource string, percentSource string) {
	return battleResolveAttackerPenetrationByProfile(actor, battleDamageChannelProfileForType(damageType))
}

func battleResolveAttackerPenetrationNormalized(actor *battleUnitState, isMagical bool) (flat float64, percent float64, flatSource string, percentSource string) {
	return battleResolveAttackerPenetrationByProfile(actor, battleDamageChannelProfileByIsMagical(isMagical))
}

func battleResolveAttackerPenetrationByProfile(actor *battleUnitState, profile battleDamageChannelProfile) (flat float64, percent float64, flatSource string, percentSource string) {
	if actor == nil {
		return 0, 0, "", ""
	}
	flat, flatSource = battleCollectPenetrationWithSource(actor.Stats, profile.FlatPenStatKeys, profile.FlatPenSourceLabel)
	percent, percentSource = battleCollectPenetrationWithSource(actor.Stats, profile.PercentPenStatKeys, profile.PercentPenSourceLabel)
	return flat, percent, flatSource, percentSource
}

func battleResolveSpecPenetration(spec battleDamageSpec, actor *battleUnitState, skillLevel int, damageType string) (float64, float64) {
	return battleResolveSpecPenetrationByProfile(spec, actor, skillLevel, battleDamageChannelProfileForType(damageType))
}

func battleResolveSpecPenetrationNormalized(spec battleDamageSpec, actor *battleUnitState, skillLevel int, isMagical bool) (float64, float64) {
	return battleResolveSpecPenetrationByProfile(spec, actor, skillLevel, battleDamageChannelProfileByIsMagical(isMagical))
}

func battleResolveSpecPenetrationByProfile(spec battleDamageSpec, actor *battleUnitState, skillLevel int, profile battleDamageChannelProfile) (float64, float64) {
	flat := battleEvaluateAmountFormula(actor, skillLevel, spec.FlatDefensePenetrationFormula)
	percent := battleEvaluateAmountFormula(actor, skillLevel, spec.PercentDefensePenetrationFormula)
	if profile.SpecFlatPenFormula != nil {
		flat += battleEvaluateAmountFormula(actor, skillLevel, profile.SpecFlatPenFormula(spec))
	}
	if profile.SpecPercentPenFormula != nil {
		percent += battleEvaluateAmountFormula(actor, skillLevel, profile.SpecPercentPenFormula(spec))
	}
	return flat, percent
}

// battleResolveEffectiveDefense applies penetration in order: flat pen first, then percent pen.
// effectiveDefense = (defense - flatPen) * (1 - percentPen)
// Result can be negative when flat pen exceeds defense — negative effective defense causes
// damage amplification via the K formula: multiplier = K / (K + effectiveDefense).
// ComputeDamage caps the floor at -K/2, giving a maximum amplification of 2x.
func battleResolveEffectiveDefense(targetDefense float64, totalFlatPen float64, totalPercentPen float64) float64 {
	defenseBase := maxFloat(0, targetDefense)
	flatPen := maxFloat(0, totalFlatPen)
	percentPen := clampFloat(totalPercentPen, 0, 1)
	return (defenseBase - flatPen) * (1 - percentPen)
}

func battleLegacyAmountFormulaFromBasicAttack(profile *battleBasicAttackProfile) battleSkillAmountFormula {
	terms := make([]battleAttackTerm, 0, len(profile.Formula))
	for _, term := range profile.Formula {
		attrKey := strings.TrimSpace(term.AttrKey)
		if attrKey == "" || math.IsNaN(term.Coefficient) || math.IsInf(term.Coefficient, 0) {
			continue
		}
		terms = append(terms, battleAttackTerm{
			AttrKey:       attrKey,
			Coefficient:   term.Coefficient,
			PerSkillLevel: 0,
		})
	}
	return battleSkillAmountFormula{
		BaseAdd: 0,
		Terms:   terms,
	}
}

func resolveBattleBasicAttackDamageSpec(profile *battleBasicAttackProfile) battleDamageSpec {
	if profile == nil {
		return battleDamageSpec{
			DamageType: "physical",
			HitCount: battleSkillTargetCount{
				Base: 1,
			},
		}
	}
	return normalizeBattleDamageSpecForRuntime(profile.DamageSpec, profile.DamageType, battleLegacyAmountFormulaFromBasicAttack(profile))
}

func battleMutationEffectSelected(mutation battleAffixEffectMutation, effect battleSkillEffect) bool {
	if effect.GeneratedBy != "" && !mutation.IncludeGeneratedEffects {
		return false
	}
	if len(mutation.EffectKinds) > 0 {
		if _, ok := mutation.EffectKinds[strings.ToLower(strings.TrimSpace(effect.Kind))]; !ok {
			return false
		}
	}
	if len(mutation.EffectIDs) > 0 {
		if _, ok := mutation.EffectIDs[strings.ToLower(strings.TrimSpace(effect.ID))]; !ok {
			return false
		}
	}
	if len(mutation.DamageTypes) > 0 {
		damageType := strings.ToLower(strings.TrimSpace(effect.DamageType))
		if damageType == "" {
			damageType = strings.ToLower(strings.TrimSpace(effect.DamageSpec.DamageType))
		}
		if _, ok := mutation.DamageTypes[damageType]; !ok {
			return false
		}
	}
	return true
}

func battleSkillEffectWithMutationSource(effect battleSkillEffect, mutation battleAffixEffectMutation) battleSkillEffect {
	source := battleEffectMutationSource{
		MutationID:    strings.TrimSpace(mutation.ID),
		SourceKind:    strings.TrimSpace(mutation.SourceKind),
		SourceID:      strings.TrimSpace(mutation.SourceID),
		SourceNameKey: strings.TrimSpace(mutation.SourceNameKey),
	}
	if source.MutationID == "" && source.SourceKind == "" && source.SourceID == "" && source.SourceNameKey == "" {
		return effect
	}
	for _, existing := range effect.MutationSources {
		if existing == source {
			return effect
		}
	}
	effect.MutationSources = append(append([]battleEffectMutationSource(nil), effect.MutationSources...), source)
	return effect
}

func battleSkillEffectSourceMeta(effect battleSkillEffect, base map[string]any) map[string]any {
	meta := cloneAnyMap(base)
	if meta == nil {
		meta = map[string]any{}
	}
	if len(effect.MutationSources) == 0 {
		return meta
	}
	sources := make([]map[string]any, 0, len(effect.MutationSources))
	for _, source := range effect.MutationSources {
		row := map[string]any{}
		if source.MutationID != "" {
			row["mutationId"] = source.MutationID
		}
		if source.SourceKind != "" {
			row["sourceKind"] = source.SourceKind
		}
		if source.SourceID != "" {
			row["sourceId"] = source.SourceID
			if strings.EqualFold(source.SourceKind, "specialization") {
				row["nodeId"] = source.SourceID
			}
		}
		if source.SourceNameKey != "" {
			row["sourceNameKey"] = source.SourceNameKey
		}
		if len(row) > 0 {
			sources = append(sources, row)
		}
	}
	if len(sources) == 0 {
		return meta
	}
	meta["mutationSources"] = sources
	primary := sources[len(sources)-1]
	for index := len(sources) - 1; index >= 0; index-- {
		if strings.EqualFold(strings.TrimSpace(fmt.Sprintf("%v", sources[index]["sourceKind"])), "specialization") {
			primary = sources[index]
			meta["specialization"] = true
			break
		}
	}
	for _, key := range []string{"mutationId", "sourceKind", "sourceId", "nodeId", "sourceNameKey"} {
		if value, ok := primary[key]; ok {
			meta[key] = value
		}
	}
	return meta
}

func scaleBattleSkillAmountFormula(formula battleSkillAmountFormula, multiplier float64) battleSkillAmountFormula {
	formula.Terms = append([]battleAttackTerm(nil), formula.Terms...)
	formula.BaseAdd = roundTo4(formula.BaseAdd * multiplier)
	for index := range formula.Terms {
		formula.Terms[index].Coefficient = roundTo4(formula.Terms[index].Coefficient * multiplier)
		formula.Terms[index].PerSkillLevel = roundTo4(formula.Terms[index].PerSkillLevel * multiplier)
		formula.Terms[index].PerEnhanceLevel = roundTo4(formula.Terms[index].PerEnhanceLevel * multiplier)
	}
	return formula
}

func applyBattleAttackStatPolicy(actor *battleUnitState, policy string, formula battleSkillAmountFormula) battleSkillAmountFormula {
	if actor == nil || actor.Stats == nil || strings.ToLower(strings.TrimSpace(policy)) != "higher" || len(formula.Terms) == 0 {
		return formula
	}

	hasPhysicalAttack := false
	hasMagicalAttack := false
	for _, term := range formula.Terms {
		switch strings.ToLower(strings.TrimSpace(term.AttrKey)) {
		case "physics_damage":
			hasPhysicalAttack = true
		case "magical_damage":
			hasMagicalAttack = true
		}
	}
	// Hybrid formulas are intentional; only replace a single authored attack family.
	if hasPhysicalAttack == hasMagicalAttack {
		return formula
	}
	authoredStatKey, alternativeStatKey := "physics_damage", "magical_damage"
	if hasMagicalAttack {
		authoredStatKey, alternativeStatKey = alternativeStatKey, authoredStatKey
	}
	authoredValue, authoredExists := actor.Stats[authoredStatKey]
	alternativeValue, alternativeExists := actor.Stats[alternativeStatKey]
	if !authoredExists || !alternativeExists || math.IsNaN(authoredValue) || math.IsInf(authoredValue, 0) || math.IsNaN(alternativeValue) || math.IsInf(alternativeValue, 0) || alternativeValue <= authoredValue {
		return formula
	}

	formula.Terms = append([]battleAttackTerm(nil), formula.Terms...)
	for index := range formula.Terms {
		if strings.EqualFold(strings.TrimSpace(formula.Terms[index].AttrKey), authoredStatKey) {
			formula.Terms[index].AttrKey = alternativeStatKey
		}
	}
	return formula
}

func applyBattleTargetingPatch(targeting battleSkillTargeting, patch map[string]any) battleSkillTargeting {
	if len(patch) == 0 {
		return targeting
	}
	if raw, ok := patch["targetPositions"]; ok {
		targeting.TargetPositions = battleParseStringList(raw)
	}
	if raw, ok := patch["secondarySeek"]; ok {
		targeting.SecondarySeek = normalizeBattleSecondarySeek(strings.TrimSpace(fmt.Sprintf("%v", raw)), targeting.SecondarySeek)
	}
	if raw, ok := patch["targetScope"]; ok {
		targeting.TargetScope = strings.ToLower(strings.TrimSpace(fmt.Sprintf("%v", raw)))
	}
	if raw, ok := patch["selectRule"]; ok {
		targeting.SelectRule = strings.ToLower(strings.TrimSpace(fmt.Sprintf("%v", raw)))
	}
	if raw, ok := patch["selectAttrKey"]; ok {
		targeting.SelectAttrKey = strings.TrimSpace(fmt.Sprintf("%v", raw))
	}
	if raw, ok := patch["fallbackSelfIfEmpty"]; ok {
		targeting.FallbackSelfIfEmpty = parseBooleanLoose(raw, targeting.FallbackSelfIfEmpty)
	}
	if raw, ok := patch["excludeSelf"]; ok {
		targeting.ExcludeSelf = parseBooleanLoose(raw, targeting.ExcludeSelf)
	}
	if raw, ok := patch["targetCount"]; ok {
		if node := normalizeAnyMap(raw); len(node) > 0 {
			if base, exists := node["base"]; exists {
				targeting.TargetCount.Base = maxInt(1, int(toInt64Loose(base, int64(targeting.TargetCount.Base))))
			}
			if maxRaw, exists := node["max"]; exists {
				targeting.TargetCount.Max = maxInt(1, int(toInt64Loose(maxRaw, int64(targeting.TargetCount.Max))))
				targeting.TargetCount.MaxSet = true
			}
		} else {
			targeting.TargetCount.Base = maxInt(1, int(toInt64Loose(raw, int64(targeting.TargetCount.Base))))
			targeting.TargetCount.Max = targeting.TargetCount.Base
			targeting.TargetCount.MaxSet = true
		}
	}
	return targeting
}

func battleMutationSkillSelected(actor *battleUnitState, mutation battleAffixEffectMutation, scope string, skill battleSkillState) bool {
	normalizedScope := strings.ToLower(strings.TrimSpace(scope))
	mutationScope := strings.ToLower(strings.TrimSpace(mutation.Scope))
	if mutationScope == "" {
		mutationScope = "both"
	}
	if mutationScope != "both" && mutationScope != normalizedScope {
		return false
	}
	if len(mutation.SkillIDs) > 0 {
		if _, ok := mutation.SkillIDs[strings.ToLower(strings.TrimSpace(skill.ID))]; !ok {
			return false
		}
	}
	category := strings.ToLower(strings.TrimSpace(skill.Category))
	if normalizedScope == "basic_attack" && category == "" {
		category = "basic_attack"
	}
	if len(mutation.SkillCategories) > 0 {
		if _, ok := mutation.SkillCategories[category]; !ok {
			return false
		}
	}
	if len(mutation.WeaponClasses) > 0 {
		matched := false
		if actor != nil {
			for _, weaponClass := range actor.WeaponClasses {
				if _, ok := mutation.WeaponClasses[strings.ToLower(strings.TrimSpace(weaponClass))]; ok {
					matched = true
					break
				}
			}
		}
		if !matched {
			return false
		}
	}
	if len(mutation.DamageTypes) > 0 {
		matched := false
		for _, effect := range skill.Effects {
			damageType := strings.ToLower(strings.TrimSpace(effect.DamageType))
			if damageType == "" {
				damageType = strings.ToLower(strings.TrimSpace(effect.DamageSpec.DamageType))
			}
			if _, ok := mutation.DamageTypes[damageType]; ok {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	return true
}

func battleMutationSkillHasSelectedEffect(mutation battleAffixEffectMutation, skill battleSkillState) bool {
	if mutation.SelectAll && len(mutation.EffectKinds) == 0 && len(mutation.EffectIDs) == 0 {
		return true
	}
	for _, effect := range skill.Effects {
		if battleMutationEffectSelected(mutation, effect) {
			return true
		}
	}
	return false
}

func (unit *battleUnitState) applyBattleSkillScalarMutations(scope string, skill battleSkillState) battleSkillState {
	if unit == nil || skill.Combo != nil {
		return skill
	}
	for index := 0; index < unit.effectMutationCount(); index++ {
		mutation, ok := unit.effectMutationAt(index)
		if !ok || !battleMutationSkillSelected(unit, mutation, scope, skill) || !battleMutationSkillHasSelectedEffect(mutation, skill) {
			continue
		}
		previousCooldown := skill.CooldownTurns
		skill.CooldownTurns = maxInt(0, previousCooldown+mutation.CooldownDelta)
		if previousCooldown > 0 && mutation.CooldownDelta < 0 && mutation.CooldownFloor > 0 {
			skill.CooldownTurns = maxInt(mutation.CooldownFloor, skill.CooldownTurns)
		}
		skill.MPCost = maxFloat(0, skill.MPCost+mutation.MPCostDelta)
		if mutation.MPCostMultiplier != nil {
			skill.MPCost = maxFloat(0, skill.MPCost**mutation.MPCostMultiplier)
		}
	}
	return skill
}

func (s *Server) applyBattleAffixEffectMutations(
	actor *battleUnitState,
	scope string,
	skillID string,
	skillLevel int,
	fallbackTargeting battleSkillTargeting,
	effects []battleSkillEffect,
) []battleSkillEffect {
	if actor == nil || len(effects) == 0 {
		return effects
	}
	mutationCount := actor.effectMutationCount()
	if mutationCount == 0 {
		return effects
	}
	out := append([]battleSkillEffect(nil), effects...)
	normalizedScope := strings.ToLower(strings.TrimSpace(scope))
	normalizedSkillID := strings.ToLower(strings.TrimSpace(skillID))
	skillCategory := ""
	for _, candidate := range actor.Skills {
		if strings.EqualFold(strings.TrimSpace(candidate.ID), normalizedSkillID) {
			skillCategory = strings.ToLower(strings.TrimSpace(candidate.Category))
			break
		}
	}
	if normalizedScope == "basic_attack" && skillCategory == "" {
		skillCategory = "basic_attack"
	}
	for mutationIndex := 0; mutationIndex < mutationCount; mutationIndex++ {
		mutation, ok := actor.effectMutationAt(mutationIndex)
		if !ok {
			continue
		}
		mutationScope := strings.ToLower(strings.TrimSpace(mutation.Scope))
		if mutationScope == "" {
			mutationScope = "both"
		}
		scopeMatched := mutationScope == "both" || mutationScope == normalizedScope
		if !scopeMatched {
			continue
		}
		if len(mutation.SkillIDs) > 0 {
			if _, ok := mutation.SkillIDs[normalizedSkillID]; !ok {
				continue
			}
		}
		if len(mutation.SkillCategories) > 0 {
			if _, ok := mutation.SkillCategories[skillCategory]; !ok {
				continue
			}
		}
		if len(mutation.WeaponClasses) > 0 {
			matched := false
			for _, weaponClass := range actor.WeaponClasses {
				if _, ok := mutation.WeaponClasses[strings.ToLower(strings.TrimSpace(weaponClass))]; ok {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}
		selectedIndexes := make([]int, 0, len(out))
		if mutation.SelectAll {
			for idx := range out {
				if battleMutationEffectSelected(mutation, out[idx]) {
					selectedIndexes = append(selectedIndexes, idx)
				}
			}
		} else if len(mutation.EffectKinds) > 0 || len(mutation.EffectIDs) > 0 {
			for idx := range out {
				if battleMutationEffectSelected(mutation, out[idx]) {
					selectedIndexes = append(selectedIndexes, idx)
				}
			}
		} else {
			for _, ref := range mutation.Indexes {
				idx := ref - 1
				if idx < 0 || idx >= len(out) {
					continue
				}
				selectedIndexes = append(selectedIndexes, idx)
			}
		}
		for _, idx := range selectedIndexes {
			if idx < 0 || idx >= len(out) {
				continue
			}
			effect := out[idx]
			if len(mutation.TargetingPatch) > 0 {
				effect.Targeting = applyBattleTargetingPatch(effect.Targeting, mutation.TargetingPatch)
			}
			if mutation.TargetCountDelta != nil {
				baseDelta := int(math.Floor(battleEvaluateAmountFormula(actor, skillLevel, *mutation.TargetCountDelta)))
				effect.Targeting.TargetCount.Base += baseDelta
				if effect.Targeting.TargetCount.MaxSet {
					effect.Targeting.TargetCount.Max += baseDelta
				}
			}
			if len(mutation.TargetCountTermsAppend) > 0 {
				effect.Targeting.TargetCount.Formula = append(effect.Targeting.TargetCount.Formula, mutation.TargetCountTermsAppend...)
			}
			if mutation.ConstraintTargetCountMin != nil && effect.Targeting.TargetCount.Base < *mutation.ConstraintTargetCountMin {
				effect.Targeting.TargetCount.Base = *mutation.ConstraintTargetCountMin
			}
			if mutation.ConstraintTargetCountMax != nil && effect.Targeting.TargetCount.Base > *mutation.ConstraintTargetCountMax {
				effect.Targeting.TargetCount.Base = *mutation.ConstraintTargetCountMax
			}
			if mutation.AmountBaseAddDelta != nil && (effect.Kind == "damage" || effect.Kind == "heal") {
				baseAddDelta := battleEvaluateAmountFormula(actor, skillLevel, *mutation.AmountBaseAddDelta)
				effect.AmountFormula.BaseAdd = roundTo4(effect.AmountFormula.BaseAdd + baseAddDelta)
				effect.DamageSpec.AmountFormula.BaseAdd = roundTo4(effect.DamageSpec.AmountFormula.BaseAdd + baseAddDelta)
			}
			if len(mutation.AmountTermsAppend) > 0 && (effect.Kind == "damage" || effect.Kind == "heal") {
				effect.AmountFormula.Terms = append(effect.AmountFormula.Terms, mutation.AmountTermsAppend...)
				effect.DamageSpec.AmountFormula.Terms = append(effect.DamageSpec.AmountFormula.Terms, mutation.AmountTermsAppend...)
			}
			if mutation.AmountMultiplier != nil && (effect.Kind == "damage" || effect.Kind == "heal") {
				effect.AmountFormula = scaleBattleSkillAmountFormula(effect.AmountFormula, *mutation.AmountMultiplier)
				effect.DamageSpec.AmountFormula = scaleBattleSkillAmountFormula(effect.DamageSpec.AmountFormula, *mutation.AmountMultiplier)
			}
			if mutation.RuntimeDamageAmp != 0 && effect.Kind == "damage" {
				multiplier := maxFloat(0, 1+mutation.RuntimeDamageAmp)
				effect.AmountFormula = scaleBattleSkillAmountFormula(effect.AmountFormula, multiplier)
				effect.DamageSpec.AmountFormula = scaleBattleSkillAmountFormula(effect.DamageSpec.AmountFormula, multiplier)
			}
			if mutation.AttackStatPolicy != "" && effect.Kind == "damage" {
				effect.AttackStatPolicy = mutation.AttackStatPolicy
			}
			if mutation.HitCountDelta != 0 && effect.Kind == "damage" {
				effect.DamageSpec = normalizeBattleDamageSpecForRuntime(effect.DamageSpec, effect.DamageType, effect.AmountFormula)
				effect.DamageSpec.HitCount.Base = maxInt(1, effect.DamageSpec.HitCount.Base+mutation.HitCountDelta)
				if effect.DamageSpec.HitCount.MaxSet {
					effect.DamageSpec.HitCount.Max = maxInt(1, effect.DamageSpec.HitCount.Max+mutation.HitCountDelta)
				}
			}
			if mutation.DamageTypePolicy != "" && effect.Kind == "damage" {
				effect.DamageTypePolicy = mutation.DamageTypePolicy
			}
			if effect.Kind == "status" {
				statusIDs := make([]string, 0, len(effect.StatusPayload.StatusIDs)+len(mutation.StatusAppend))
				seen := map[string]struct{}{}
				for _, statusID := range effect.StatusPayload.StatusIDs {
					normalized := strings.TrimSpace(statusID)
					if normalized == "" {
						continue
					}
					if mutation.StatusRemove != nil {
						if _, removed := mutation.StatusRemove[normalized]; removed {
							continue
						}
					}
					if _, exists := seen[normalized]; exists {
						continue
					}
					seen[normalized] = struct{}{}
					statusIDs = append(statusIDs, normalized)
				}
				for _, statusID := range mutation.StatusAppend {
					normalized := strings.TrimSpace(statusID)
					if normalized == "" {
						continue
					}
					if mutation.StatusRemove != nil {
						if _, removed := mutation.StatusRemove[normalized]; removed {
							continue
						}
					}
					if _, exists := seen[normalized]; exists {
						continue
					}
					seen[normalized] = struct{}{}
					statusIDs = append(statusIDs, normalized)
				}
				effect.StatusPayload.StatusIDs = statusIDs
			}
			if effect.Kind == "damage" {
				effect.DamageSpec = normalizeBattleDamageSpecForRuntime(effect.DamageSpec, effect.DamageType, effect.AmountFormula)
				effect.DamageType = effect.DamageSpec.DamageType
			}
			effect = battleSkillEffectWithMutationSource(effect, mutation)
			out[idx] = effect
		}
		if len(mutation.AppendEffects) > 0 && len(selectedIndexes) > 0 {
			for _, effect := range mutation.AppendEffects {
				next := battleSkillEffectWithMutationSource(effect, mutation)
				switch next.Kind {
				case "splash_damage":
					for _, selectedIndex := range selectedIndexes {
						if selectedIndex < 0 || selectedIndex >= len(out) || out[selectedIndex].Kind != "damage" {
							continue
						}
						generated := out[selectedIndex]
						generated.ID = next.ID
						generated.GeneratedBy = mutation.ID
						generated.AmountFormula = scaleBattleSkillAmountFormula(generated.AmountFormula, next.AmountRatio)
						generated.DamageSpec.AmountFormula = scaleBattleSkillAmountFormula(generated.DamageSpec.AmountFormula, next.AmountRatio)
						generated.Targeting.TargetCount.Base = maxInt(1, next.Targeting.TargetCount.Base)
						generated.Targeting.TargetCount.Max = generated.Targeting.TargetCount.Base
						generated.Targeting.TargetCount.MaxSet = true
						out = append(out, generated)
					}
					continue
				case "secondary_damage":
					sourceIndex := -1
					for _, selectedIndex := range selectedIndexes {
						if selectedIndex >= 0 && selectedIndex < len(out) && out[selectedIndex].Kind == "damage" {
							sourceIndex = selectedIndex
							break
						}
					}
					if sourceIndex < 0 {
						continue
					}
					generated := out[sourceIndex]
					generated.ID = next.ID
					generated.GeneratedBy = mutation.ID
					generated.AmountFormula = scaleBattleSkillAmountFormula(generated.AmountFormula, next.AmountRatio)
					generated.DamageSpec.AmountFormula = scaleBattleSkillAmountFormula(generated.DamageSpec.AmountFormula, next.AmountRatio)
					generated.Targeting.TargetCount = battleSkillTargetCount{Base: 1, Max: 1, MaxSet: true}
					generated.ExcludeTargetsFromEffects = []int{sourceIndex + 1}
					switch next.SecondaryTargetPolicy {
					case "same_row":
						generated.TargetRowFromEffect = sourceIndex + 1
						generated.Targeting.SecondarySeek = "none"
					case "fixed_row":
						generated.Targeting.TargetPositions = []string{next.SecondaryTargetPosition}
						generated.Targeting.SecondarySeek = "none"
					case "all_rows":
						generated.Targeting.TargetPositions = []string{"vanguard", "midguard", "rearguard"}
						generated.Targeting.SecondarySeek = "front_to_rear"
					}
					out = append(out, generated)
					continue
				case "mark_target":
					next.Kind = "status"
					next.StatusPayload.StatusIDs = []string{"specialization:mark:" + strings.TrimSpace(next.MarkID)}
					next.StatusPayload.DurationBaseTurns = int(next.DurationTurns)
				}
				if len(next.Targeting.TargetPositions) == 0 {
					next.Targeting = fallbackTargeting
				}
				if next.Kind == "damage" {
					next.DamageSpec = normalizeBattleDamageSpecForRuntime(next.DamageSpec, next.DamageType, next.AmountFormula)
					next.DamageType = next.DamageSpec.DamageType
				}
				out = append(out, next)
			}
		}
	}
	for index := range out {
		if out[index].Kind != "damage" || out[index].AttackStatPolicy == "" {
			continue
		}
		out[index].AmountFormula = applyBattleAttackStatPolicy(actor, out[index].AttackStatPolicy, out[index].AmountFormula)
		out[index].DamageSpec.AmountFormula = applyBattleAttackStatPolicy(actor, out[index].AttackStatPolicy, out[index].DamageSpec.AmountFormula)
	}
	return out
}

func battleEvaluateTargetPredicate(actor *battleUnitState, target *battleUnitState, predicate battleSkillTargetPredicate) bool {
	if actor == nil || target == nil {
		return false
	}
	switch predicate.Kind {
	case "compare_attr":
		left := actor.Stats[strings.TrimSpace(predicate.ActorAttr)]
		right := target.Stats[strings.TrimSpace(predicate.TargetAttr)]
		switch predicate.Operator {
		case ">", "gt":
			return left > right
		case "gte":
			return left >= right
		case "<", "lt":
			return left < right
		case "lte":
			return left <= right
		case "=", "==", "eq":
			return left == right
		case "!=", "ne":
			return left != right
		default:
			return left > right
		}
	case "exclude_enemy_type":
		return !strings.EqualFold(strings.TrimSpace(target.EnemyType), strings.TrimSpace(predicate.EnemyType))
	case "require_target_has_status":
		return battleHasBuff(target, predicate.StatusID)
	case "exclude_target_has_status":
		return !battleHasBuff(target, predicate.StatusID)
	case "target_hp_not_full":
		return target.HP < target.MaxHP
	default:
		return true
	}
}

func battleFilterTargetsByPredicates(actor *battleUnitState, targets []*battleUnitState, predicates []battleSkillTargetPredicate) []*battleUnitState {
	if len(targets) == 0 || len(predicates) == 0 {
		return targets
	}
	filtered := make([]*battleUnitState, 0, len(targets))
	for _, target := range targets {
		if target == nil {
			continue
		}
		pass := true
		for _, predicate := range predicates {
			if !battleEvaluateTargetPredicate(actor, target, predicate) {
				pass = false
				break
			}
		}
		if pass {
			filtered = append(filtered, target)
		}
	}
	return filtered
}

func (s *Server) battleConsumeStatusesAfterHit(runtime *battleSkillActionRuntime, actor *battleUnitState, target *battleUnitState, statusIDs []string, reason string) {
	if s == nil || runtime == nil || actor == nil || target == nil || len(statusIDs) == 0 {
		return
	}
	removedIDs := make([]string, 0, len(statusIDs))
	for _, statusID := range statusIDs {
		if battleRemoveBuff(target, statusID) {
			removedIDs = append(removedIDs, statusID)
			runtime.actions = append(runtime.actions, map[string]any{
				"kind":     "state_detach",
				"targetId": target.UnitID,
				"sourceId": actor.UnitID,
				"buffId":   strings.TrimSpace(statusID),
				"reason":   reason,
				"meta": map[string]any{
					"consumedOnHit": true,
				},
			})
		}
	}
	if len(removedIDs) == 0 {
		return
	}
	beforeStats := cloneFloatMap(target.Stats)
	beforeMaxHP := target.MaxHP
	beforeMaxMP := target.MaxMP
	beforePosition := target.Position
	beforeSide := normalizeBattleSide(target.Side)
	beforePerspective := battleUnitPerspectiveSide(target)
	s.battleRecomputeUnitStats(target)
	afterSide := normalizeBattleSide(target.Side)
	afterPerspective := battleUnitPerspectiveSide(target)
	if !battleFloatMapEqual(beforeStats, target.Stats) || beforeMaxHP != target.MaxHP || beforeMaxMP != target.MaxMP || beforePosition != target.Position || beforeSide != afterSide || beforePerspective != afterPerspective {
		runtime.actions = append(runtime.actions, map[string]any{
			"kind":            "stat_recalc",
			"targetId":        target.UnitID,
			"sourceId":        actor.UnitID,
			"reason":          reason,
			"stats":           cloneFloatMap(target.Stats),
			"baseStats":       cloneFloatMap(target.BaseStats),
			"deltaStats":      cloneFloatMap(target.StatModifiers),
			"statModifiers":   cloneFloatMap(target.StatModifiers),
			"statSources":     cloneNestedFloatMap(target.StatSources),
			"buffs":           battlePublicBuffSlice(target.Buffs),
			"maxHp":           roundTo4(target.MaxHP),
			"maxMp":           roundTo4(target.MaxMP),
			"fromPosition":    beforePosition,
			"position":        target.Position,
			"basePosition":    target.BasePosition,
			"side":            target.Side,
			"baseSide":        target.BaseSide,
			"perspectiveSide": target.PerspectiveSide,
		})
	}
}

func (s *Server) battleRemoveSkillStatuses(runtime *battleSkillActionRuntime, actor, target *battleUnitState, statusIDs []string, maxRemoved int, reason string, meta map[string]any) {
	if s == nil || runtime == nil || actor == nil || target == nil || len(statusIDs) == 0 || maxRemoved <= 0 {
		return
	}
	beforeStats := cloneFloatMap(target.Stats)
	beforeMaxHP := target.MaxHP
	beforeMaxMP := target.MaxMP
	beforePosition := target.Position
	beforeSide := normalizeBattleSide(target.Side)
	beforePerspective := battleUnitPerspectiveSide(target)
	removed := 0
	for _, statusID := range statusIDs {
		if removed >= maxRemoved {
			break
		}
		buffID := strings.TrimSpace(statusID)
		if buffID == "" {
			continue
		}
		result := s.battleDispelBuff(target, buffID)
		if !result.Changed {
			continue
		}
		removed++
		runtime.actions = append(runtime.actions, buildBattleBuffDispelAction(actor.UnitID, target.UnitID, buffID, reason, result, meta))
	}
	if removed == 0 {
		return
	}
	s.battleRecomputeUnitStats(target)
	afterSide := normalizeBattleSide(target.Side)
	afterPerspective := battleUnitPerspectiveSide(target)
	if !battleFloatMapEqual(beforeStats, target.Stats) || beforeMaxHP != target.MaxHP || beforeMaxMP != target.MaxMP || beforePosition != target.Position || beforeSide != afterSide || beforePerspective != afterPerspective {
		runtime.actions = append(runtime.actions, map[string]any{
			"kind": "stat_recalc", "targetId": target.UnitID, "sourceId": actor.UnitID, "reason": reason,
			"stats": cloneFloatMap(target.Stats), "baseStats": cloneFloatMap(target.BaseStats),
			"deltaStats": cloneFloatMap(target.StatModifiers), "statModifiers": cloneFloatMap(target.StatModifiers),
			"statSources": cloneNestedFloatMap(target.StatSources), "buffs": battlePublicBuffSlice(target.Buffs),
			"maxHp": roundTo4(target.MaxHP), "maxMp": roundTo4(target.MaxMP), "fromPosition": beforePosition,
			"position": target.Position, "basePosition": target.BasePosition, "side": target.Side,
			"baseSide": target.BaseSide, "perspectiveSide": target.PerspectiveSide,
			"meta": cloneAnyMap(meta),
		})
	}
}

func (s *Server) executeBattleTemplateSkillEffects(
	state *battleRuntimeState,
	now time.Time,
	actor *battleUnitState,
	skill battleSkillState,
	runtime *battleSkillActionRuntime,
) battleskills.SkillResult {
	if state == nil || actor == nil || runtime == nil {
		return battleskills.SkillResult{}
	}
	for _, effect := range skill.Effects {
		if effect.Kind != "consume_hp" || effect.MaxHPRatio <= 0 {
			continue
		}
		minimumHP := maxFloat(0, effect.MinimumHP)
		cost := math.Floor(maxFloat(0, actor.MaxHP) * effect.MaxHPRatio)
		cost = math.Min(cost, maxFloat(0, actor.HP-minimumHP))
		if cost <= 0 {
			continue
		}
		runtime.ApplyResource(battleskills.ResourceAction{
			SourceID: actor.UnitID,
			TargetID: actor.UnitID,
			Resource: "hp",
			Amount:   -cost,
			Reason:   "specialization_consume_hp",
			Meta: battleSkillEffectSourceMeta(effect, map[string]any{
				"specialization": true,
				"effectId":       effect.ID,
			}),
		})
	}
	levelOffset := maxFloat(0, float64(skill.Level-1))
	selectedTargetsByEffect := make([][]*battleUnitState, len(skill.Effects))
	for effectIndex, effect := range skill.Effects {
		if effect.Kind == "consume_hp" {
			continue
		}
		if !battleUnitHasAllStatuses(actor, effect.RequiredCasterStatuses) || battleUnitHasAnyStatus(actor, effect.ForbiddenCasterStatuses) {
			selectedTargetsByEffect[effectIndex] = nil
			continue
		}
		shouldExecute := true
		for _, ref := range effect.OnlyWhenNoTargetFromEffects {
			refIndex := ref - 1
			if refIndex < 0 || refIndex >= effectIndex {
				continue
			}
			if len(selectedTargetsByEffect[refIndex]) > 0 {
				shouldExecute = false
				break
			}
		}
		if !shouldExecute {
			selectedTargetsByEffect[effectIndex] = nil
			continue
		}
		if effect.Kind == "corpse_explosion" {
			corpses := s.battleSelectCorpseTargets(state, actor, skill, effect.CorpseTargeting, state.randFloat64)
			if len(corpses) == 0 || corpses[0] == nil {
				selectedTargetsByEffect[effectIndex] = nil
				continue
			}
			corpse := corpses[0]
			targets := s.battleSelectTemplateTargets(state, actor, skill, effect.Targeting, effect.IgnoreUntargetable, state.randFloat64)
			selectedTargetsByEffect[effectIndex] = targets
			if len(targets) == 0 {
				continue
			}
			extraFormulaAttrs := map[string]float64{
				"corpse_max_hp": maxFloat(0, corpse.MaxHP),
			}
			amount := math.Floor(battleEvaluateAmountFormulaWithExtra(actor, skill.Level, effect.AmountFormula, extraFormulaAttrs))
			if amount <= 0 {
				continue
			}
			damageType := normalizeBattleDamageType(effect.DamageType)
			if damageType == "" {
				damageType = "magical"
			}
			runtime.actions = append(runtime.actions, map[string]any{
				"kind":     "corpse_focus",
				"targetId": corpse.UnitID,
				"sourceId": actor.UnitID,
				"reason":   "corpse_explosion_focus",
				"delayMs":  int64(0),
				"meta":     battleSkillEffectSourceMeta(effect, map[string]any{"corpseExplosion": true}),
			})
			for _, target := range targets {
				if target == nil {
					continue
				}
				if !battleUnitHasAllStatuses(target, effect.RequiredTargetStatuses) {
					continue
				}
				runtime.ApplyDamage(battleskills.DamageAction{
					SourceID:        actor.UnitID,
					DisplaySourceID: corpse.UnitID,
					TargetID:        target.UnitID,
					Amount:          amount,
					DamageType:      damageType,
					Reason:          "corpse_explosion",
					Meta: battleSkillEffectSourceMeta(effect, map[string]any{
						"template":        true,
						"corpseExplosion": true,
						"corpseUnitId":    corpse.UnitID,
						"corpseMaxHp":     roundTo4(corpse.MaxHP),
						"baseDamage":      roundTo4(amount),
					}),
				})
			}
			if effect.ConsumeCorpse {
				beforeHP := corpse.HP
				corpse.HP = 0
				corpse.Alive = false
				corpse.CorpseConsumed = true
				runtime.actions = append(runtime.actions, map[string]any{
					"kind":           "corpse_consume",
					"targetId":       corpse.UnitID,
					"sourceId":       actor.UnitID,
					"reason":         "corpse_explosion_consume",
					"beforeHp":       roundTo4(beforeHP),
					"afterHp":        roundTo4(corpse.HP),
					"afterMp":        roundTo4(corpse.MP),
					"afterAlive":     corpse.Alive,
					"corpseConsumed": corpse.CorpseConsumed,
					"delayMs":        int64(0),
					"meta":           battleSkillEffectSourceMeta(effect, map[string]any{"corpseExplosion": true}),
				})
			}
			continue
		}
		var targets []*battleUnitState
		if effect.ReuseTargetsFromEffect > 0 {
			refIndex := effect.ReuseTargetsFromEffect - 1
			if refIndex >= 0 && refIndex < effectIndex {
				targets = selectedTargetsByEffect[refIndex]
			}
		}
		if len(targets) == 0 {
			targets = s.battleSelectTemplateEffectTargets(state, actor, skill, effect, selectedTargetsByEffect, state.randFloat64)
		}
		targets = battleFilterEffectStatusConditions(targets, effect.RequiredTargetStatuses, effect.ForbiddenTargetStatuses)
		selectedTargetsByEffect[effectIndex] = targets
		if (effect.MpCostFormula.BaseAdd != 0 || len(effect.MpCostFormula.Terms) > 0) && len(targets) > 0 {
			cost := math.Floor(effect.MpCostFormula.BaseAdd + evaluateBattleSkillFormula(actor, effect.MpCostFormula.Terms, skill.Level))
			if cost > 0 {
				if actor.MP < cost {
					targets = nil
					selectedTargetsByEffect[effectIndex] = nil
				} else {
					runtime.ApplyResource(battleskills.ResourceAction{
						SourceID: actor.UnitID,
						TargetID: actor.UnitID,
						Resource: "mp",
						Amount:   -cost,
						Reason:   "basic_attack_mp_cost",
						Meta:     battleSkillEffectSourceMeta(effect, map[string]any{"template": true, "skillId": skill.ID}),
					})
				}
			}
		}
		if len(targets) == 0 {
			continue
		}
		switch effect.Kind {
		case "damage":
			spec := normalizeBattleDamageSpecForRuntime(effect.DamageSpec, effect.DamageType, effect.AmountFormula)
			hitCount := battleResolveDamageHitCount(actor, skill.Level, spec)
			for _, target := range targets {
				if target == nil {
					continue
				}
				if !battleUnitHasAllStatuses(target, effect.RequiredTargetStatuses) {
					continue
				}
				resolvedDamageType := spec.DamageType
				switch strings.ToLower(strings.TrimSpace(effect.DamageTypePolicy)) {
				case "physical", "magical":
					resolvedDamageType = strings.ToLower(strings.TrimSpace(effect.DamageTypePolicy))
				case "higher_final_damage":
					resolvedDamageType = s.battleChooseHigherFinalDamageType(state, actor, target, spec, skill.Level)
				}
				damageChannel := battleDamageChannelProfileForType(resolvedDamageType)
				isMagicalDamage := damageChannel.IsMagical
				chainID := fmt.Sprintf("%s:%s:%d:%s", actor.UnitID, skill.ID, effectIndex+1, target.UnitID)
				for hitIndex := 0; hitIndex < hitCount; hitIndex++ {
					targetDefense := target.Stats[damageChannel.DefenseStatKey]
					attackerFlatPen, attackerPercentPen, flatSource, percentSource := battleResolveAttackerPenetrationNormalized(actor, isMagicalDamage)
					specFlatPen, specPercentPen := battleResolveSpecPenetrationNormalized(spec, actor, skill.Level, isMagicalDamage)
					targetTypeBonus := battleTargetTypeBonus(target, spec)
					extraDamageAmplification := battleEvaluateAmountFormula(actor, skill.Level, spec.ExtraDamageAmplificationFormula) + targetTypeBonus
					totalFlatPen := maxFloat(0, attackerFlatPen+specFlatPen)
					percentPenCap := s.deps.Config.SharedGlobalSettings.CombatBalanceModel.PercentPenetrationCap
					if percentPenCap <= 0 || percentPenCap > 1 {
						percentPenCap = 0.8
					}
					totalPercentPen := clampFloat(attackerPercentPen+specPercentPen, 0, percentPenCap)
					effectiveDefense := battleResolveEffectiveDefense(targetDefense, totalFlatPen, totalPercentPen)
					baseAttack := math.Floor(battleEvaluateAmountFormula(actor, skill.Level, spec.AmountFormula))
					defenseK := s.battleDefenseKForState(state)
					damage, crit := battlecore.ComputeDamage(battlecore.DamageInput{
						Physical:         !isMagicalDamage,
						Attack:           baseAttack,
						Defense:          effectiveDefense,
						DefenseK:         defenseK,
						Strength:         actor.Stats["strength"],
						Dexterity:        actor.Stats["dexterity"],
						Coeff:            1,
						CritChance:       battleCritChanceForDamageTypeNormalized(actor, isMagicalDamage),
						CritDamage:       battleCritDamageForDamageTypeNormalized(actor, isMagicalDamage),
						CritDamageResist: targetCritDamageResist(target),
						Rand:             state.randFloat64(),
					})
					hitDamageMultiplier := battleResolveHitDamageMultiplier(spec.HitDamageScaling, hitIndex+1)
					damage *= hitDamageMultiplier
					cappedEffectiveDefense := maxFloat(effectiveDefense, -defenseK/2)
					meta := map[string]any{
						"template":                  true,
						"chainId":                   chainID,
						"hitIndex":                  hitIndex + 1,
						"hitCount":                  hitCount,
						"baseAttack":                roundTo4(baseAttack),
						"rawDefense":                roundTo4(targetDefense),
						"effectiveDefense":          roundTo4(cappedEffectiveDefense),
						"flatDefensePenetration":    roundTo4(totalFlatPen),
						"percentDefensePenetration": roundTo4(totalPercentPen),
						"extraDamageAmplification":  roundTo4(extraDamageAmplification),
					}
					if spec.HitDamageScaling != nil {
						meta["hitDamageMultiplier"] = hitDamageMultiplier
					}
					if targetTypeBonus != 0 {
						meta["targetTypeBonus"] = roundTo4(targetTypeBonus)
					}
					if isMagicalDamage {
						if strings.TrimSpace(flatSource) != "" {
							meta["flatPenetrationSource"] = flatSource
						}
						if strings.TrimSpace(percentSource) != "" {
							meta["percentPenetrationSource"] = percentSource
						}
					}
					meta = battleSkillEffectSourceMeta(effect, meta)
					runtime.ApplyDamage(battleskills.DamageAction{
						SourceID:                       actor.UnitID,
						TargetID:                       target.UnitID,
						Amount:                         damage,
						Crit:                           crit,
						DamageType:                     resolvedDamageType,
						ExtraDamageAmplification:       extraDamageAmplification,
						EnemyOnlyDamageReductionPierce: spec.EnemyOnlyDamageReductionPierce,
						EnemyOnlyDamageReductionPierceTargetStatuses: spec.EnemyOnlyDamageReductionPierceTargetStatuses,
						Reason: "template_damage",
						Meta:   meta,
					})
					if !target.Alive || target.HP <= 0 {
						break
					}
				}
				if len(effect.ConsumeStatusesOnHit) > 0 {
					s.battleConsumeStatusesAfterHit(runtime, actor, target, effect.ConsumeStatusesOnHit, "template_damage_consume")
				}
				if len(effect.ConsumeCasterStatusesOnHit) > 0 {
					s.battleConsumeStatusesAfterHit(runtime, actor, actor, effect.ConsumeCasterStatusesOnHit, "template_damage_consume_caster")
				}
			}
		case "heal":
			base := effect.AmountFormula.BaseAdd + evaluateBattleSkillFormula(actor, effect.AmountFormula.Terms, skill.Level)
			amount := math.Floor(base)
			for _, target := range targets {
				if target == nil {
					continue
				}
				if !battleUnitHasAllStatuses(target, effect.RequiredTargetStatuses) {
					continue
				}
				runtime.ApplyHeal(battleskills.HealAction{
					SourceID: actor.UnitID,
					TargetID: target.UnitID,
					Amount:   amount,
					Reason:   "template_heal",
					Meta: battleSkillEffectSourceMeta(effect, map[string]any{
						"template": true,
					}),
				})
			}
		case "resource":
			resource := strings.ToLower(strings.TrimSpace(effect.Resource))
			if resource == "" {
				resource = "mp"
			}
			base := effect.AmountFormula.BaseAdd + evaluateBattleSkillFormula(actor, effect.AmountFormula.Terms, skill.Level)
			amount := math.Floor(base)
			for _, target := range targets {
				if target == nil {
					continue
				}
				if !battleUnitHasAllStatuses(target, effect.RequiredTargetStatuses) {
					continue
				}
				runtime.ApplyResource(battleskills.ResourceAction{
					SourceID: actor.UnitID,
					TargetID: target.UnitID,
					Resource: resource,
					Amount:   amount,
					Reason:   "template_resource",
					Meta: battleSkillEffectSourceMeta(effect, map[string]any{
						"template": true,
						"resource": resource,
					}),
				})
			}
		case "adjust_skill_cooldown":
			for _, target := range targets {
				if target == nil {
					continue
				}
				action, changed := s.battleAdjustUnitSkillCooldowns(
					target,
					effect.CooldownAmount,
					effect.IncludeBasic,
					effect.TargetSkillIDs,
					effect.ExcludeSkillIDs,
					actor.UnitID,
					"template_adjust_skill_cooldown",
					0,
					battleSkillEffectSourceMeta(effect, map[string]any{"template": true, "skillId": skill.ID}),
				)
				if changed {
					runtime.actions = append(runtime.actions, action)
				}
			}
		case "remove_buff":
			for _, target := range targets {
				if target == nil {
					continue
				}
				s.battleRemoveSkillStatuses(runtime, actor, target, effect.BuffIDs, effect.MaxRemoved, "template_remove_buff", battleSkillEffectSourceMeta(effect, map[string]any{"template": true, "skillId": skill.ID}))
			}
		case "weighted_random_per_target":
			applyChance := battleResolveSkillStatusApplyChance(actor, skill.Level, effect.StatusPayload)
			for _, target := range targets {
				if target == nil {
					continue
				}
				check, result := s.battleResolveStatusApplyProbability(state, actor, target, applyChance, skill.ID)
				if !result.Established {
					continue
				}
				if !result.Succeeded {
					runtime.actions = append(runtime.actions, s.battleFinalizeProbabilityResult(state, now, check, result)...)
					continue
				}
				choice, ok := battleChooseWeightedStatus(state, effect.WeightedStatusChoices)
				if !ok {
					runtime.actions = append(runtime.actions, s.battleFinalizeProbabilityResult(state, now, check, result)...)
					continue
				}
				runtime.ApplyBuff(battleskills.BuffAction{
					SourceID: actor.UnitID, TargetID: target.UnitID, BuffID: choice.StatusID,
					DurationTurns: int64(choice.DurationTurns), Payload: cloneAnyMap(choice.Payload),
					Reason: "template_weighted_status",
					Meta: battleSkillEffectSourceMeta(effect, map[string]any{
						"template": true, "outcomeId": choice.ID, "applyChance": roundTo4(result.FinalChance),
					}),
				})
				runtime.actions = append(runtime.actions, s.battleFinalizeProbabilityResult(state, now, check, result)...)
			}
		case "status":
			durationTurns := int(math.Floor(
				float64(effect.StatusPayload.DurationBaseTurns) +
					effect.StatusPayload.DurationPerSkillLevelTurns*levelOffset +
					battleEvaluateAmountFormula(actor, skill.Level, effect.StatusPayload.DurationFormula),
			))
			durationTurns = maxInt(0, durationTurns)
			payload := cloneAnyMap(effect.StatusPayload.Payload)
			applyChance := battleResolveSkillStatusApplyChance(actor, skill.Level, effect.StatusPayload)
			if len(effect.StatusPayload.PayloadFormula) > 0 {
				sourceEnhanceLevel := 0
				if runtime != nil && runtime.isBasicAttack && actor.BasicAttack != nil {
					sourceEnhanceLevel = maxInt(0, actor.BasicAttack.SourceEnhanceLevel)
				}
				if payload == nil {
					payload = map[string]any{}
				}
				for key, formula := range effect.StatusPayload.PayloadFormula {
					normalizedKey := strings.TrimSpace(key)
					if normalizedKey == "" {
						continue
					}
					payload[normalizedKey] = roundTo4(battleEvaluateAmountFormulaWithEnhance(actor, skill.Level, formula, sourceEnhanceLevel))
				}
			}
			for _, target := range targets {
				if target == nil {
					continue
				}
				if !battleUnitHasAllStatuses(target, effect.RequiredTargetStatuses) {
					continue
				}
				for _, statusID := range effect.StatusPayload.StatusIDs {
					buffID := strings.TrimSpace(statusID)
					if buffID == "" {
						continue
					}
					probabilityCheck, probabilityResult := s.battleResolveStatusApplyProbability(state, actor, target, applyChance, skill.ID, buffID)
					if !probabilityResult.Established {
						continue
					}
					if !probabilityResult.Succeeded {
						runtime.actions = append(runtime.actions, s.battleFinalizeProbabilityResult(state, now, probabilityCheck, probabilityResult)...)
						continue
					}
					runtime.ApplyBuff(battleskills.BuffAction{
						SourceID:      actor.UnitID,
						TargetID:      target.UnitID,
						BuffID:        buffID,
						DurationTurns: int64(durationTurns),
						Payload:       payload,
						Reason:        "template_status",
						Meta: battleSkillEffectSourceMeta(effect, map[string]any{
							"template":    true,
							"turns":       durationTurns,
							"applyChance": roundTo4(probabilityResult.FinalChance),
						}),
					})
					runtime.actions = append(runtime.actions, s.battleFinalizeProbabilityResult(state, now, probabilityCheck, probabilityResult)...)
				}
			}
		case "summon":
			payload := effect.SummonPayload
			if strings.TrimSpace(payload.UnitID) == "" {
				continue
			}
			sourceEnhanceLevel := 0
			if runtime != nil && runtime.isBasicAttack && actor.BasicAttack != nil {
				sourceEnhanceLevel = maxInt(0, actor.BasicAttack.SourceEnhanceLevel)
			}
			resolvedCount := int(math.Floor(battleEvaluateAmountFormulaWithEnhance(actor, skill.Level, payload.CountFormula, sourceEnhanceLevel)))
			if resolvedCount <= 0 {
				continue
			}
			resolvedMaxCount := battleResolveSummonMaxCount(actor, skill.Level, payload, sourceEnhanceLevel)
			for summonIndex := 0; summonIndex < resolvedCount; summonIndex++ {
				expireActions, hasCapacity := battlePrepareSummonCapacity(
					state,
					payload.UnitID,
					actor.UnitID,
					resolvedMaxCount,
					payload.OnCapOverflow,
				)
				runtime.actions = append(runtime.actions, expireActions...)
				if !hasCapacity {
					break
				}
				summon, ok := s.buildBattleSkillSummonRuntime(state, actor, payload, skill.Level, sourceEnhanceLevel)
				if !ok {
					continue
				}
				meta := map[string]any{
					"template":             true,
					"templateUnitId":       payload.UnitID,
					"persistAcrossBattles": payload.PersistAcrossBattles,
					"onCasterDeath":        normalizeBattleSummonOnCasterDeath(payload.OnCasterDeath),
					"onCapOverflow":        normalizeBattleSummonOnCapOverflow(payload.OnCapOverflow),
					"onDispel":             normalizeBattleSummonOnDispel(payload.OnDispel),
					"onDurationEnd":        normalizeBattleSummonOnDurationEnd(payload.OnDurationEnd),
				}
				if resolvedMaxCount > 0 {
					meta["maxCountPerUnit"] = resolvedMaxCount
				}
				meta = battleSkillEffectSourceMeta(effect, meta)
				runtime.ApplySummon(battleskills.SummonAction{
					SourceID: actor.UnitID,
					Summon:   summon,
					Reason:   "template_summon",
					Meta:     meta,
				})
			}
		}
	}
	return battleskills.SkillResult{
		Meta: map[string]any{
			"template": true,
		},
	}
}

func (s *Server) battleSelectTemplateEffectTargets(
	state *battleRuntimeState,
	actor *battleUnitState,
	skill battleSkillState,
	effect battleSkillEffect,
	selectedTargetsByEffect [][]*battleUnitState,
	randFn func() float64,
) []*battleUnitState {
	targeting := effect.Targeting
	if refIndex := effect.TargetRowFromEffect - 1; refIndex >= 0 && refIndex < len(selectedTargetsByEffect) {
		for _, target := range selectedTargetsByEffect[refIndex] {
			if target == nil {
				continue
			}
			if position := normalizeBattleTargetPosition(target.Position); position != "" {
				targeting.TargetPositions = []string{position}
				targeting.SecondarySeek = "none"
			}
			break
		}
	}

	excludedTargetIDs := map[string]struct{}{}
	for _, ref := range effect.ExcludeTargetsFromEffects {
		refIndex := ref - 1
		if refIndex < 0 || refIndex >= len(selectedTargetsByEffect) {
			continue
		}
		for _, target := range selectedTargetsByEffect[refIndex] {
			if target == nil {
				continue
			}
			if targetID := strings.TrimSpace(target.UnitID); targetID != "" {
				excludedTargetIDs[targetID] = struct{}{}
			}
		}
	}

	desiredCount := resolveBattleTemplateTargetCount(actor, skill.Level, targeting)
	if len(excludedTargetIDs) > 0 {
		targeting.TargetCount = battleSkillTargetCount{
			Base:   desiredCount + len(excludedTargetIDs),
			Max:    desiredCount + len(excludedTargetIDs),
			MaxSet: true,
		}
	}
	targets := s.battleSelectTemplateTargets(state, actor, skill, targeting, effect.IgnoreUntargetable, randFn)
	if len(excludedTargetIDs) == 0 || len(targets) == 0 {
		return targets
	}
	filtered := make([]*battleUnitState, 0, minInt(desiredCount, len(targets)))
	for _, target := range targets {
		if target == nil {
			continue
		}
		if _, excluded := excludedTargetIDs[strings.TrimSpace(target.UnitID)]; excluded {
			continue
		}
		filtered = append(filtered, target)
		if len(filtered) >= desiredCount {
			break
		}
	}
	return filtered
}

func (s *Server) applyBattleBasicAttackProfile(skill battleSkillState, actor *battleUnitState) battleSkillState {
	if actor == nil {
		return skill
	}
	defaultBasicSkillID := strings.ToLower(strings.TrimSpace(s.battleDefaultBasicSkillID()))
	if defaultBasicSkillID == "" || strings.ToLower(strings.TrimSpace(skill.ID)) != defaultBasicSkillID {
		return skill
	}
	if actor.BasicAttack == nil {
		return skill
	}
	profile := actor.BasicAttack
	if len(profile.Effects) > 0 {
		skill.Targeting = profile.Effects[0].Targeting
		skill.Effects = append([]battleSkillEffect(nil), profile.Effects...)
		skill.Effects = s.applyBattleAffixEffectMutations(actor, "basic_attack", skill.ID, skill.Level, skill.Targeting, skill.Effects)
		if len(skill.Effects) > 0 {
			skill.Targeting = skill.Effects[0].Targeting
		}
		return skill
	}
	targetDirection := normalizeBattleTargetPosition(profile.TargetDirection)
	if targetDirection == "" {
		targetDirection = "vanguard"
	}
	seekStrategy := normalizeBattleSecondarySeek(profile.TargetSeekStrategy, "front_to_rear")
	targeting := battleSkillTargeting{
		TargetPositions: []string{targetDirection},
		SecondarySeek:   seekStrategy,
		TargetScope:     "enemy",
		SelectRule:      "random",
		TargetCount: battleSkillTargetCount{
			Base: 1,
		},
	}
	spec := resolveBattleBasicAttackDamageSpec(profile)
	skill.Targeting = targeting
	skill.Effects = []battleSkillEffect{
		{
			Kind:          "damage",
			DamageType:    spec.DamageType,
			Targeting:     targeting,
			AmountFormula: spec.AmountFormula,
			DamageSpec:    spec,
		},
	}
	skill.Effects = s.applyBattleAffixEffectMutations(actor, "basic_attack", skill.ID, skill.Level, skill.Targeting, skill.Effects)
	if len(skill.Effects) > 0 {
		skill.Targeting = skill.Effects[0].Targeting
	}
	return skill
}

func (s *Server) executeBattleSkill(
	state *battleRuntimeState,
	now time.Time,
	actor *battleUnitState,
	skill battleSkillState,
	trigger string,
	forcedTargets []string,
) battleSkillExecution {
	if state == nil || actor == nil {
		return battleSkillExecution{}
	}
	if skill.Combo != nil {
		return s.executeEquipmentCombo(state, now, actor, skill)
	}
	if !s.battleSkillRulesetEnabled(skill) {
		return battleSkillExecution{Meta: map[string]any{
			"blocked": true,
			"reason":  "skill_forge_ruleset_disabled",
		}}
	}
	battleRuntimeUnitIndex(state)
	if state.dominationBySkill != nil {
		if result, interrupted := s.interceptBattleSkill(state, actor, skill); interrupted {
			return result
		}
	}
	_ = trigger
	skill = s.applyBattleBasicAttackProfile(skill, actor)
	isBasicAttack := strings.EqualFold(strings.TrimSpace(skill.ID), strings.TrimSpace(s.battleDefaultBasicSkillID()))
	if !isBasicAttack {
		skill.Effects = s.applyBattleAffixEffectMutations(actor, "skill", skill.ID, skill.Level, skill.Targeting, skill.Effects)
	}
	if len(skill.Effects) > 0 {
		skill.Targeting = skill.Effects[0].Targeting
	}
	skill, _ = s.battleApplyBuffTargetingOverrides(actor, skill, isBasicAttack)
	if len(skill.Effects) > 0 {
		skill.Targeting = skill.Effects[0].Targeting
	}
	runtime := newBattleSkillActionRuntime(state, actor, now, s, forcedTargets)
	runtime.isBasicAttack = isBasicAttack
	runtime.skillID = strings.TrimSpace(skill.ID)
	primaryTargeting := skill.Targeting
	if len(skill.Effects) > 0 {
		primaryTargeting = skill.Effects[0].Targeting
	}
	primaryIgnoreUntargetable := false
	if len(skill.Effects) > 0 {
		primaryIgnoreUntargetable = skill.Effects[0].IgnoreUntargetable
	}
	initialTargets := s.battleSelectTemplateTargets(state, actor, skill, primaryTargeting, primaryIgnoreUntargetable, state.randFloat64)
	result := s.executeBattleTemplateSkillEffects(state, now, actor, skill, runtime)
	collectedEffects := runtime.AppliedEffects()
	effects := make([]battleskills.SkillEffect, 0, len(collectedEffects)+len(result.Effects))
	effects = append(effects, collectedEffects...)
	effects = append(effects, result.Effects...)
	actionRows := runtime.ActionLog()
	unitIndexByID := battleBuildUnitIndexByID(state)
	targetSummaryByID := make(map[string]map[string]any, len(effects))
	targetOrder := make([]string, 0, len(effects))
	for _, target := range initialTargets {
		targetID := strings.TrimSpace(target.UnitID)
		if targetID == "" {
			continue
		}
		targetSummaryByID[targetID] = map[string]any{
			"targetId":      targetID,
			"targetSide":    strings.TrimSpace(target.Side),
			"targetUserId":  target.UserID,
			"targetEnemyId": strings.TrimSpace(target.EnemyID),
			"damage":        0.0,
			"heal":          0.0,
			"hp":            roundTo4(target.HP),
			"mp":            roundTo4(target.MP),
			"alive":         target.Alive,
			"effects":       []map[string]any{},
		}
		targetOrder = append(targetOrder, targetID)
	}
	effectRows := make([]map[string]any, 0, len(effects))
	for _, effect := range effects {
		kind := strings.TrimSpace(effect.Kind)
		targetID := strings.TrimSpace(effect.TargetID)
		sourceID := strings.TrimSpace(effect.SourceID)
		damageType := strings.TrimSpace(effect.DamageType)
		resource := strings.TrimSpace(effect.Resource)
		buffID := strings.TrimSpace(effect.BuffID)
		reason := strings.TrimSpace(effect.Reason)
		row := map[string]any{
			"kind":       kind,
			"targetId":   targetID,
			"sourceId":   sourceID,
			"amount":     roundTo4(effect.Amount),
			"crit":       effect.Crit,
			"damageType": damageType,
			"resource":   resource,
			"buffId":     buffID,
		}
		if effect.BuffDurationTurns != 0 {
			row["buffDurationTurns"] = effect.BuffDurationTurns
		}
		if len(effect.BuffPayload) > 0 {
			row["buffPayload"] = effect.BuffPayload
		}
		if effect.Summon != nil {
			row["summon"] = effect.Summon
		}
		if reason != "" {
			row["reason"] = reason
		}
		if len(effect.Meta) > 0 {
			row["meta"] = effect.Meta
		}
		effectRows = append(effectRows, row)
		target := unitIndexByID[targetID]
		summary, ok := targetSummaryByID[targetID]
		if !ok && target != nil {
			summary = map[string]any{
				"targetId":      target.UnitID,
				"targetSide":    target.Side,
				"targetUserId":  target.UserID,
				"targetEnemyId": target.EnemyID,
				"damage":        0.0,
				"heal":          0.0,
				"hp":            roundTo4(target.HP),
				"mp":            roundTo4(target.MP),
				"alive":         target.Alive,
				"effects":       []map[string]any{},
			}
			targetSummaryByID[targetID] = summary
			targetOrder = append(targetOrder, targetID)
			ok = true
		}
		if !ok {
			continue
		}
		switch strings.ToLower(kind) {
		case "damage":
			summary["damage"] = roundTo4(summary["damage"].(float64) + effect.Amount)
			if effect.Crit {
				summary["crit"] = true
			}
		case "heal":
			summary["heal"] = roundTo4(summary["heal"].(float64) + effect.Amount)
		}
		if target != nil {
			summary["hp"] = roundTo4(target.HP)
			summary["mp"] = roundTo4(target.MP)
			summary["alive"] = target.Alive
		}
		summary["effects"] = append(summary["effects"].([]map[string]any), row)
	}
	targetRows := make([]map[string]any, 0, len(targetOrder))
	for _, targetID := range targetOrder {
		if row, ok := targetSummaryByID[targetID]; ok {
			targetRows = append(targetRows, row)
		}
	}
	return battleSkillExecution{
		Targets: targetRows,
		Effects: effectRows,
		Actions: actionRows,
		Meta:    result.Meta,
	}
}

func battleUnitByID(state *battleRuntimeState, unitID string) *battleUnitState {
	if state == nil {
		return nil
	}
	unitID = strings.TrimSpace(unitID)
	if unitID == "" {
		return nil
	}
	return battleRuntimeUnitIndex(state)[unitID]
}

func battleBuildUnitIndexByID(state *battleRuntimeState) map[string]*battleUnitState {
	return battleRuntimeUnitIndex(state)
}

func battleRuntimeUnitIndex(state *battleRuntimeState) map[string]*battleUnitState {
	index := map[string]*battleUnitState{}
	if state == nil {
		return index
	}
	totalUnits := len(state.Players) + len(state.Enemies)
	if state.unitIndexByID != nil && state.unitIndexCount == totalUnits {
		return state.unitIndexByID
	}
	index = make(map[string]*battleUnitState, totalUnits)
	state.dominationBySkill = nil
	for idx := range state.Players {
		unitID := strings.TrimSpace(state.Players[idx].UnitID)
		if unitID == "" {
			continue
		}
		index[unitID] = &state.Players[idx]
		indexBattleDominationUnit(state, &state.Players[idx])
	}
	for idx := range state.Enemies {
		unitID := strings.TrimSpace(state.Enemies[idx].UnitID)
		if unitID == "" {
			continue
		}
		index[unitID] = &state.Enemies[idx]
		indexBattleDominationUnit(state, &state.Enemies[idx])
	}
	state.unitIndexByID = index
	state.unitIndexCount = totalUnits
	return index
}

func toBattleSkillRuntime(skill battleSkillState) battleskills.SkillRuntime {
	targetCount := resolveBattleTemplateTargetCount(nil, skill.Level, skill.Targeting)
	return battleskills.SkillRuntime{
		SkillID:             strings.TrimSpace(skill.ID),
		SkillLevel:          maxInt(1, skill.Level),
		SkillExp:            maxInt64(0, skill.Exp),
		DamageType:          "physical",
		DamageCoeff:         1,
		HPCost:              skill.HPCost,
		MPCost:              skill.MPCost,
		TargetDirection:     firstNonEmptyTargetPosition(skill.Targeting.TargetPositions),
		TargetSeekStrategy:  strings.TrimSpace(skill.Targeting.SecondarySeek),
		TargetMode:          "single",
		TargetCount:         maxInt(1, targetCount),
		TargetPositions:     append([]string(nil), skill.Targeting.TargetPositions...),
		SecondarySeek:       strings.TrimSpace(skill.Targeting.SecondarySeek),
		TargetScope:         strings.TrimSpace(skill.Targeting.TargetScope),
		SelectRule:          strings.TrimSpace(skill.Targeting.SelectRule),
		SelectAttrKey:       strings.TrimSpace(skill.Targeting.SelectAttrKey),
		FallbackSelfIfEmpty: skill.Targeting.FallbackSelfIfEmpty,
		IgnoreUntargetable:  len(skill.Effects) > 0 && skill.Effects[0].IgnoreUntargetable,
		CooldownTurns:       skill.CooldownTurns,
		CooldownRemain:      skill.CooldownRemain,
	}
}

func toBattleSkillUnitRuntime(unit battleUnitState) battleskills.UnitRuntime {
	buffs := make([]battleskills.BuffRuntime, 0, len(unit.Buffs))
	for _, buff := range unit.Buffs {
		buffs = append(buffs, battleskills.BuffRuntime{
			BuffID:      strings.TrimSpace(buff.BuffID),
			RemainTurns: buff.RemainTurns,
			Payload:     buff.Payload,
		})
	}
	return battleskills.UnitRuntime{
		UnitID:             strings.TrimSpace(unit.UnitID),
		Side:               strings.TrimSpace(unit.Side),
		UserID:             unit.UserID,
		EnemyID:            strings.TrimSpace(unit.EnemyID),
		EnemyType:          strings.TrimSpace(unit.EnemyType),
		EnemyLevel:         unit.EnemyLevel,
		Position:           strings.TrimSpace(unit.Position),
		Alive:              unit.Alive,
		HP:                 unit.HP,
		MP:                 unit.MP,
		MaxHP:              unit.MaxHP,
		MaxMP:              unit.MaxMP,
		Stats:              cloneFloatMap(unit.Stats),
		Buffs:              buffs,
		BasicAttackProfile: toBattleSkillBasicAttackProfile(unit.BasicAttack),
	}
}

func toBattleSkillBasicAttackProfile(profile *battleBasicAttackProfile) *battleskills.BasicAttackProfile {
	if profile == nil {
		return nil
	}
	return &battleskills.BasicAttackProfile{
		TargetDirection:    strings.TrimSpace(profile.TargetDirection),
		TargetSeekStrategy: strings.TrimSpace(profile.TargetSeekStrategy),
		DamageType:         normalizeBattleDamageType(profile.DamageType),
		Formula:            toBattleSkillBasicAttackFormula(profile.Formula),
		DamageSpec:         battleDamageSpecToAnyMap(profile.DamageSpec),
	}
}

func battleDamageSpecToAnyMap(spec battleDamageSpec) map[string]any {
	spec = normalizeBattleDamageSpecForRuntime(spec, spec.DamageType, spec.AmountFormula)
	out := map[string]any{
		"damageType": spec.DamageType,
		"amountFormula": map[string]any{
			"baseAdd": spec.AmountFormula.BaseAdd,
			"terms":   spec.AmountFormula.Terms,
		},
		"hitCount": map[string]any{
			"base":    spec.HitCount.Base,
			"formula": spec.HitCount.Formula,
		},
		"flatDefensePenetrationFormula": map[string]any{
			"baseAdd": spec.FlatDefensePenetrationFormula.BaseAdd,
			"terms":   spec.FlatDefensePenetrationFormula.Terms,
		},
		"percentDefensePenetrationFormula": map[string]any{
			"baseAdd": spec.PercentDefensePenetrationFormula.BaseAdd,
			"terms":   spec.PercentDefensePenetrationFormula.Terms,
		},
		"physicalFlatDefensePenetrationFormula": map[string]any{
			"baseAdd": spec.PhysicalFlatDefensePenetrationFormula.BaseAdd,
			"terms":   spec.PhysicalFlatDefensePenetrationFormula.Terms,
		},
		"physicalPercentDefensePenetrationFormula": map[string]any{
			"baseAdd": spec.PhysicalPercentDefensePenetrationFormula.BaseAdd,
			"terms":   spec.PhysicalPercentDefensePenetrationFormula.Terms,
		},
		"magicalFlatDefensePenetrationFormula": map[string]any{
			"baseAdd": spec.MagicalFlatDefensePenetrationFormula.BaseAdd,
			"terms":   spec.MagicalFlatDefensePenetrationFormula.Terms,
		},
		"magicalPercentDefensePenetrationFormula": map[string]any{
			"baseAdd": spec.MagicalPercentDefensePenetrationFormula.BaseAdd,
			"terms":   spec.MagicalPercentDefensePenetrationFormula.Terms,
		},
		"extraDamageAmplificationFormula": map[string]any{
			"baseAdd": spec.ExtraDamageAmplificationFormula.BaseAdd,
			"terms":   spec.ExtraDamageAmplificationFormula.Terms,
		},
		"bonusAgainstTargetTypes": spec.BonusAgainstTargetTypes,
	}
	if spec.HitDamageScaling != nil {
		out["hitDamageScaling"] = map[string]any{
			"kind":                spec.HitDamageScaling.Kind,
			"secondHitMultiplier": spec.HitDamageScaling.SecondHitMultiplier,
			"curvePower":          spec.HitDamageScaling.CurvePower,
		}
	}
	return out
}

func toBattleSkillBasicAttackFormula(formula []battleAttackTerm) []battleskills.BasicAttackFormulaTerm {
	out := make([]battleskills.BasicAttackFormulaTerm, 0, len(formula))
	for _, term := range formula {
		attrKey := strings.TrimSpace(term.AttrKey)
		if attrKey == "" || math.IsNaN(term.Coefficient) || math.IsInf(term.Coefficient, 0) {
			continue
		}
		out = append(out, battleskills.BasicAttackFormulaTerm{
			AttrKey:     attrKey,
			Coefficient: term.Coefficient,
		})
	}
	return out
}

func toCoreAttackTerms(formula []battleAttackTerm) []battlecore.AttackTerm {
	out := make([]battlecore.AttackTerm, 0, len(formula))
	for _, term := range formula {
		attrKey := strings.TrimSpace(term.AttrKey)
		if attrKey == "" || math.IsNaN(term.Coefficient) || math.IsInf(term.Coefficient, 0) || math.IsNaN(term.PerSkillLevel) || math.IsInf(term.PerSkillLevel, 0) {
			continue
		}
		out = append(out, battlecore.AttackTerm{
			AttrKey:       attrKey,
			Coefficient:   term.Coefficient,
			PerSkillLevel: term.PerSkillLevel,
		})
	}
	return out
}

func fromCoreAttackTerms(formula []battlecore.AttackTerm) []battleAttackTerm {
	out := make([]battleAttackTerm, 0, len(formula))
	for _, term := range formula {
		attrKey := strings.TrimSpace(term.AttrKey)
		if attrKey == "" || math.IsNaN(term.Coefficient) || math.IsInf(term.Coefficient, 0) || math.IsNaN(term.PerSkillLevel) || math.IsInf(term.PerSkillLevel, 0) {
			continue
		}
		out = append(out, battleAttackTerm{
			AttrKey:       attrKey,
			Coefficient:   term.Coefficient,
			PerSkillLevel: term.PerSkillLevel,
		})
	}
	return out
}

func parseBattleAttackTerms(raw any) []battleAttackTerm {
	var items []any
	switch typed := raw.(type) {
	case []any:
		items = typed
	case []map[string]any:
		items = make([]any, 0, len(typed))
		for _, row := range typed {
			items = append(items, row)
		}
	case []battleAttackTerm:
		out := make([]battleAttackTerm, 0, len(typed))
		for _, term := range typed {
			attrKey := strings.TrimSpace(term.AttrKey)
			if attrKey == "" || math.IsNaN(term.Coefficient) || math.IsInf(term.Coefficient, 0) || math.IsNaN(term.PerSkillLevel) || math.IsInf(term.PerSkillLevel, 0) || math.IsNaN(term.PerEnhanceLevel) || math.IsInf(term.PerEnhanceLevel, 0) {
				continue
			}
			out = append(out, battleAttackTerm{
				AttrKey:         attrKey,
				Coefficient:     term.Coefficient,
				PerSkillLevel:   term.PerSkillLevel,
				PerEnhanceLevel: term.PerEnhanceLevel,
			})
		}
		if len(out) == 0 {
			return nil
		}
		return out
	default:
		return nil
	}
	if len(items) == 0 {
		return nil
	}
	out := make([]battleAttackTerm, 0, len(items))
	for _, item := range items {
		termMap := normalizeAnyMap(item)
		if len(termMap) == 0 {
			continue
		}
		attrKey, _ := stringFromAny(firstNonNil(termMap["attrKey"], termMap["attributeKey"]))
		attrKey = strings.TrimSpace(attrKey)
		if attrKey == "" {
			continue
		}
		coefficient := toFloat64(termMap["coefficient"], 0)
		if math.IsNaN(coefficient) || math.IsInf(coefficient, 0) {
			continue
		}
		perSkillLevel := toFloat64(termMap["perSkillLevel"], 0)
		if math.IsNaN(perSkillLevel) || math.IsInf(perSkillLevel, 0) {
			continue
		}
		perEnhanceLevel := toFloat64(termMap["perEnhanceLevel"], 0)
		if math.IsNaN(perEnhanceLevel) || math.IsInf(perEnhanceLevel, 0) {
			continue
		}
		out = append(out, battleAttackTerm{
			AttrKey:         attrKey,
			Coefficient:     coefficient,
			PerSkillLevel:   perSkillLevel,
			PerEnhanceLevel: perEnhanceLevel,
		})
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func normalizeBattleTargetPosition(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "vanguard", "midguard", "rearguard":
		return strings.ToLower(strings.TrimSpace(raw))
	default:
		return ""
	}
}

func normalizeBattleSelectRule(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "hp_max", "hp_min", "hp_percent_max", "hp_percent_min", "first", "boss_first", "attr_max", "attr_min", "random", "self":
		return strings.ToLower(strings.TrimSpace(raw))
	case "self_first":
		return "self"
	default:
		return "random"
	}
}

func normalizeBattleTargetScope(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "ally", "enemy", "all", "self":
		return strings.ToLower(strings.TrimSpace(raw))
	default:
		return "enemy"
	}
}

func normalizeBattleSecondarySeek(raw string, fallback string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "rear_to_front", "front_to_rear", "random_alive", "none":
		return strings.ToLower(strings.TrimSpace(raw))
	}
	switch strings.ToLower(strings.TrimSpace(fallback)) {
	case "rear_to_front", "front_to_rear", "random_alive", "none":
		return strings.ToLower(strings.TrimSpace(fallback))
	default:
		return "front_to_rear"
	}
}

func normalizeBattleDamageType(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "magical", "magic", "mage", "魔法":
		return "magical"
	case "physical", "physic", "物理":
		return "physical"
	default:
		return "physical"
	}
}

func parseBattleSkillTargetPositions(raw any, fallback string) []string {
	out := make([]string, 0, 3)
	appendValue := func(value string) {
		normalized := normalizeBattleTargetPosition(value)
		if normalized == "" {
			return
		}
		for _, existing := range out {
			if existing == normalized {
				return
			}
		}
		out = append(out, normalized)
	}
	switch typed := raw.(type) {
	case []any:
		for _, value := range typed {
			appendValue(fmt.Sprintf("%v", value))
		}
	case []string:
		for _, value := range typed {
			appendValue(value)
		}
	case string:
		for _, part := range strings.Split(typed, ",") {
			appendValue(part)
		}
	}
	if len(out) > 0 {
		return out
	}
	if value := normalizeBattleTargetPosition(fallback); value != "" {
		return []string{value}
	}
	return []string{"vanguard"}
}

func parseBattleSkillTargetPredicates(raw any) []battleSkillTargetPredicate {
	items := make([]any, 0, 4)
	switch typed := raw.(type) {
	case []any:
		items = append(items, typed...)
	case []map[string]any:
		for _, row := range typed {
			items = append(items, row)
		}
	}
	if len(items) == 0 {
		return nil
	}
	out := make([]battleSkillTargetPredicate, 0, len(items))
	for _, item := range items {
		node, ok := item.(map[string]any)
		if !ok || len(node) == 0 {
			continue
		}
		kind := strings.ToLower(strings.TrimSpace(fmt.Sprintf("%v", node["kind"])))
		predicate := battleSkillTargetPredicate{Kind: kind}
		switch kind {
		case "compare_attr":
			predicate.ActorAttr = strings.TrimSpace(fmt.Sprintf("%v", firstNonNil(node["actorAttr"], node["actor_attr"])))
			predicate.TargetAttr = strings.TrimSpace(fmt.Sprintf("%v", firstNonNil(node["targetAttr"], node["target_attr"])))
			op := strings.ToLower(strings.TrimSpace(fmt.Sprintf("%v", firstNonNil(node["operator"], node["op"]))))
			switch op {
			case ">", "gt", "gte", "<", "lt", "lte", "=", "==", "eq", "!=", "ne":
				predicate.Operator = op
			default:
				predicate.Operator = ">"
			}
			if predicate.ActorAttr == "" || predicate.TargetAttr == "" {
				continue
			}
		case "exclude_enemy_type":
			predicate.EnemyType = strings.ToLower(strings.TrimSpace(fmt.Sprintf("%v", firstNonNil(node["enemyType"], node["enemy_type"]))))
			if predicate.EnemyType == "" {
				continue
			}
		case "require_target_has_status", "exclude_target_has_status":
			predicate.StatusID = strings.TrimSpace(fmt.Sprintf("%v", firstNonNil(node["statusId"], node["status_id"], node["buffId"], node["buff_id"])))
			if predicate.StatusID == "" {
				continue
			}
		default:
			continue
		}
		out = append(out, predicate)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func parseBattleSkillTargeting(raw any, fallbackDirection string, fallbackSeek string, fallbackBaseCount int, fallbackFormula []battleAttackTerm, fallbackMax int, fallbackMaxSet bool, fallbackPredicates []battleSkillTargetPredicate) battleSkillTargeting {
	targeting := battleSkillTargeting{
		TargetPositions:     parseBattleSkillTargetPositions(nil, fallbackDirection),
		SecondarySeek:       normalizeBattleSecondarySeek("", fallbackSeek),
		TargetScope:         "enemy",
		SelectRule:          "random",
		FallbackSelfIfEmpty: false,
		TargetCount: battleSkillTargetCount{
			Base:    maxInt(1, fallbackBaseCount),
			Formula: append([]battleAttackTerm(nil), fallbackFormula...),
			Max:     fallbackMax,
			MaxSet:  fallbackMaxSet,
		},
		TargetPredicates: append([]battleSkillTargetPredicate(nil), fallbackPredicates...),
	}
	node, ok := raw.(map[string]any)
	if !ok || len(node) == 0 {
		return targeting
	}
	targeting.TargetPositions = parseBattleSkillTargetPositions(node["targetPositions"], fallbackDirection)
	targeting.SecondarySeek = normalizeBattleSecondarySeek(fmt.Sprintf("%v", node["secondarySeek"]), fallbackSeek)
	targeting.TargetScope = normalizeBattleTargetScope(fmt.Sprintf("%v", node["targetScope"]))
	targeting.SelectRule = normalizeBattleSelectRule(fmt.Sprintf("%v", node["selectRule"]))
	targeting.SelectAttrKey = strings.TrimSpace(fmt.Sprintf("%v", node["selectAttrKey"]))
	targeting.FallbackSelfIfEmpty = parseBooleanLoose(node["fallbackSelfIfEmpty"], targeting.FallbackSelfIfEmpty)
	targeting.ExcludeSelf = parseBooleanLoose(firstNonNil(node["excludeSelf"], node["exclude_self"]), false)
	targeting.IgnoreTargetPriority = parseBooleanLoose(firstNonNil(node["ignoreTargetPriority"], node["ignore_target_priority"]), false)
	targeting.TargetPredicates = parseBattleSkillTargetPredicates(node["targetPredicates"])
	if targeting.SelectRule != "attr_max" && targeting.SelectRule != "attr_min" {
		targeting.SelectAttrKey = ""
	}
	if countNode, ok := node["targetCount"].(map[string]any); ok {
		base := int(toInt64Loose(countNode["base"], int64(targeting.TargetCount.Base)))
		if base > 0 {
			targeting.TargetCount.Base = base
		}
		if formula := parseBattleAttackTerms(countNode["formula"]); len(formula) > 0 {
			targeting.TargetCount.Formula = formula
		}
		if rawMax, exists := countNode["max"]; exists {
			maxValue := int(toInt64Loose(rawMax, 0))
			if maxValue > 0 {
				targeting.TargetCount.Max = maxValue
				targeting.TargetCount.MaxSet = true
			} else {
				targeting.TargetCount.Max = 0
				targeting.TargetCount.MaxSet = false
			}
		}
	}
	return targeting
}

func parseBattleSkillAmountFormula(raw any, fallbackFormula []battleAttackTerm) battleSkillAmountFormula {
	out := battleSkillAmountFormula{
		BaseAdd: 0,
		Terms:   append([]battleAttackTerm(nil), fallbackFormula...),
	}
	node, ok := raw.(map[string]any)
	if !ok || len(node) == 0 {
		return out
	}
	out.BaseAdd = toFloat64(node["baseAdd"], 0)
	if terms := parseBattleAttackTerms(node["terms"]); len(terms) > 0 {
		out.Terms = terms
	}
	return out
}

func parseBattleHitDamageScaling(raw any) *battleHitDamageScaling {
	node, ok := raw.(map[string]any)
	if !ok || len(node) != 3 {
		return nil
	}
	kind, ok := node["kind"].(string)
	if !ok || kind != battleHitDamageScalingKindPoweredExponentialDecay {
		return nil
	}
	secondHitMultiplier, ok := battleStrictFiniteNumber(node["secondHitMultiplier"])
	if !ok {
		return nil
	}
	curvePower, ok := battleStrictFiniteNumber(node["curvePower"])
	if !ok {
		return nil
	}
	return normalizeBattleHitDamageScaling(&battleHitDamageScaling{
		Kind:                kind,
		SecondHitMultiplier: secondHitMultiplier,
		CurvePower:          curvePower,
	})
}

func battleStrictFiniteNumber(raw any) (float64, bool) {
	var value float64
	switch typed := raw.(type) {
	case float64:
		value = typed
	case float32:
		value = float64(typed)
	case int:
		value = float64(typed)
	case int8:
		value = float64(typed)
	case int16:
		value = float64(typed)
	case int32:
		value = float64(typed)
	case int64:
		value = float64(typed)
	case uint:
		value = float64(typed)
	case uint8:
		value = float64(typed)
	case uint16:
		value = float64(typed)
	case uint32:
		value = float64(typed)
	case uint64:
		value = float64(typed)
	case json.Number:
		parsed, err := typed.Float64()
		if err != nil {
			return 0, false
		}
		value = parsed
	default:
		return 0, false
	}
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, false
	}
	return value, true
}

func parseBattleDamageSpec(raw any, fallbackDamageType string, fallbackAmount battleSkillAmountFormula) battleDamageSpec {
	out := battleDamageSpec{
		DamageType: normalizeBattleDamageType(fallbackDamageType),
		AmountFormula: battleSkillAmountFormula{
			BaseAdd: fallbackAmount.BaseAdd,
			Terms:   append([]battleAttackTerm(nil), fallbackAmount.Terms...),
		},
		HitCount: battleSkillTargetCount{
			Base: 1,
		},
		BonusAgainstTargetTypes: map[string]float64{},
	}
	node, ok := raw.(map[string]any)
	if !ok || len(node) == 0 {
		return out
	}
	if value := strings.TrimSpace(fmt.Sprintf("%v", node["damageType"])); value != "" {
		out.DamageType = normalizeBattleDamageType(value)
	}
	if amountNode, ok := node["amountFormula"].(map[string]any); ok && len(amountNode) > 0 {
		out.AmountFormula = parseBattleSkillAmountFormula(amountNode, out.AmountFormula.Terms)
	}
	if countNode, ok := node["hitCount"].(map[string]any); ok && len(countNode) > 0 {
		base := int(toInt64Loose(countNode["base"], int64(out.HitCount.Base)))
		if base > 0 {
			out.HitCount.Base = base
		}
		if formula := parseBattleAttackTerms(countNode["formula"]); len(formula) > 0 {
			out.HitCount.Formula = formula
		}
	}
	out.HitDamageScaling = parseBattleHitDamageScaling(node["hitDamageScaling"])
	if bonusNode, ok := node["bonusAgainstTargetTypes"].(map[string]any); ok && len(bonusNode) > 0 {
		bonuses := map[string]float64{}
		for key, value := range bonusNode {
			normalized := strings.ToLower(strings.TrimSpace(key))
			if normalized == "" {
				continue
			}
			amount := toFloat64(value, math.NaN())
			if math.IsNaN(amount) || math.IsInf(amount, 0) {
				continue
			}
			bonuses[normalized] = amount
		}
		out.BonusAgainstTargetTypes = bonuses
	}
	out.FlatDefensePenetrationFormula = parseBattleSkillAmountFormula(node["flatDefensePenetrationFormula"], nil)
	out.PercentDefensePenetrationFormula = parseBattleSkillAmountFormula(node["percentDefensePenetrationFormula"], nil)
	out.PhysicalFlatDefensePenetrationFormula = parseBattleSkillAmountFormula(node["physicalFlatDefensePenetrationFormula"], nil)
	out.PhysicalPercentDefensePenetrationFormula = parseBattleSkillAmountFormula(node["physicalPercentDefensePenetrationFormula"], nil)
	out.MagicalFlatDefensePenetrationFormula = parseBattleSkillAmountFormula(node["magicalFlatDefensePenetrationFormula"], nil)
	out.MagicalPercentDefensePenetrationFormula = parseBattleSkillAmountFormula(node["magicalPercentDefensePenetrationFormula"], nil)
	out.ExtraDamageAmplificationFormula = parseBattleSkillAmountFormula(node["extraDamageAmplificationFormula"], nil)
	out.EnemyOnlyDamageReductionPierce = normalizeBattleDamageReductionPierce(
		toFloat64(firstNonNil(node["enemyOnlyDamageReductionPierce"], node["enemy_only_damage_reduction_pierce"]), 0),
	)
	out.EnemyOnlyDamageReductionPierceTargetStatuses = parseBattleStringSlice(firstNonNil(
		node["enemyOnlyDamageReductionPierceTargetStatuses"],
		node["enemy_only_damage_reduction_pierce_target_statuses"],
	))
	out.BonusAgainstTargetTypes = normalizeBattleTargetTypeBonuses(out.BonusAgainstTargetTypes)
	if out.HitCount.Base <= 0 {
		out.HitCount.Base = 1
	}
	return out
}

func parseBattleSkillStatusPayload(raw any) battleSkillStatusPayload {
	out := battleSkillStatusPayload{
		StatusIDs:                  []string{},
		DurationBaseTurns:          0,
		DurationPerSkillLevelTurns: 0,
		DurationFormula:            battleSkillAmountFormula{BaseAdd: 0, Terms: nil},
	}
	node, ok := raw.(map[string]any)
	if !ok || len(node) == 0 {
		return out
	}
	ids := make([]string, 0, 4)
	switch typed := node["statusIds"].(type) {
	case []any:
		for _, value := range typed {
			id := strings.TrimSpace(fmt.Sprintf("%v", value))
			if id == "" {
				continue
			}
			ids = append(ids, id)
		}
	case []string:
		for _, value := range typed {
			id := strings.TrimSpace(value)
			if id == "" {
				continue
			}
			ids = append(ids, id)
		}
	case string:
		for _, part := range strings.Split(typed, ",") {
			id := strings.TrimSpace(part)
			if id == "" {
				continue
			}
			ids = append(ids, id)
		}
	}
	unique := make([]string, 0, len(ids))
	for _, id := range ids {
		duplicated := false
		for _, existing := range unique {
			if existing == id {
				duplicated = true
				break
			}
		}
		if duplicated {
			continue
		}
		unique = append(unique, id)
	}
	out.StatusIDs = unique
	baseTurns := int(toInt64Loose(node["durationBaseTurns"], 0))
	if baseTurns > 0 {
		out.DurationBaseTurns = baseTurns
	}
	out.DurationPerSkillLevelTurns = toFloat64(node["durationPerSkillLevelTurns"], 0)
	out.DurationFormula = parseBattleSkillAmountFormula(node["durationFormula"], nil)
	chanceBaseRaw := firstNonNil(node["applyChanceBase"], node["apply_chance_base"], node["applyChance"], node["apply_chance"])
	chanceFormulaRaw := firstNonNil(node["applyChanceFormula"], node["apply_chance_formula"])
	chancePerLevelRaw := firstNonNil(node["applyChancePerSkillLevel"], node["apply_chance_per_skill_level"])
	chanceMinRaw := firstNonNil(node["applyChanceMin"], node["apply_chance_min"])
	chanceMaxRaw := firstNonNil(node["applyChanceMax"], node["apply_chance_max"])
	if chanceBaseRaw != nil || chanceFormulaRaw != nil || chancePerLevelRaw != nil || chanceMinRaw != nil || chanceMaxRaw != nil {
		base := toFloat64(chanceBaseRaw, 0)
		out.ApplyChanceBase = &base
		out.ApplyChancePerSkillLevel = toFloat64(chancePerLevelRaw, 0)
		out.ApplyChanceFormula = parseBattleSkillAmountFormula(chanceFormulaRaw, nil)
		if chanceMinRaw != nil {
			minChance := toFloat64(chanceMinRaw, 0)
			out.ApplyChanceMin = &minChance
		}
		if chanceMaxRaw != nil {
			maxChance := toFloat64(chanceMaxRaw, 1)
			out.ApplyChanceMax = &maxChance
		}
	}
	out.Payload = normalizeAnyMap(node["payload"])
	if rawPayloadFormula := firstNonNil(node["payloadFormula"], node["payloadFormulas"], node["payload_formula"], node["payload_formulas"]); rawPayloadFormula != nil {
		if formulas := normalizeAnyMap(rawPayloadFormula); len(formulas) > 0 {
			out.PayloadFormula = make(map[string]battleSkillAmountFormula, len(formulas))
			for key, value := range formulas {
				normalizedKey := strings.TrimSpace(key)
				if normalizedKey == "" {
					continue
				}
				out.PayloadFormula[normalizedKey] = parseBattleSkillAmountFormula(value, nil)
			}
		}
	}
	return out
}

func battleResolveSkillStatusApplyChance(actor *battleUnitState, skillLevel int, payload battleSkillStatusPayload) float64 {
	if payload.ApplyChanceBase == nil {
		return 1
	}
	levelOffset := maxFloat(0, float64(skillLevel-1))
	chance := *payload.ApplyChanceBase + payload.ApplyChancePerSkillLevel*levelOffset + battleEvaluateAmountFormula(actor, skillLevel, payload.ApplyChanceFormula)
	minChance := 0.0
	if payload.ApplyChanceMin != nil {
		minChance = *payload.ApplyChanceMin
	}
	maxChance := 1.0
	if payload.ApplyChanceMax != nil {
		maxChance = *payload.ApplyChanceMax
	}
	if maxChance < minChance {
		maxChance = minChance
	}
	return clampFloat(chance, minChance, maxChance)
}

func parseBattleSkillSummonStatFormulas(raw any) map[string]battleSkillAmountFormula {
	node := normalizeAnyMap(raw)
	if len(node) == 0 {
		return nil
	}
	out := map[string]battleSkillAmountFormula{}
	for attrKey, value := range node {
		normalized := strings.TrimSpace(attrKey)
		if normalized == "" {
			continue
		}
		formula, ok := parseBattleDynamicDeltaFormula(value)
		if !ok {
			continue
		}
		out[normalized] = formula
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

const battlePlayerCloneTemplateID = "_player_clone"

type battlePlayerCloneInherit struct {
	Defined         bool
	Skills          bool
	BasicAttack     bool
	EffectMutations bool
	// Stats: "all" copies all caster stats; "none" starts from zero; "select" copies only StatsKeys
	Stats     string
	StatsKeys []string
}

type battlePlayerClonePayload struct {
	MaxCountPerUnit      int
	PersistAcrossBattles bool
	OnCasterDeath        string
	OnCapOverflow        string
	OnDispel             string
	OnDurationEnd        string
	DurationTurns        int64
	DurationFormula      battleSkillAmountFormula
	StatModifiers        map[string]float64
	StatsFormula         map[string]battleSkillAmountFormula
	Inherit              battlePlayerCloneInherit
	NameSuffix           string
	NameSuffixKey        string
	NameSeparator        string
}

func parseBattlePlayerCloneInherit(raw any) battlePlayerCloneInherit {
	node := normalizeAnyMap(raw)
	if len(node) == 0 {
		return battlePlayerCloneInherit{Defined: false}
	}
	out := battlePlayerCloneInherit{
		Defined:         true,
		Skills:          parseBooleanLoose(node["skills"], true),
		BasicAttack:     parseBooleanLoose(node["basicAttack"], true),
		EffectMutations: parseBooleanLoose(node["effectMutations"], true),
	}
	statsRaw := fmt.Sprintf("%v", node["stats"])
	switch strings.ToLower(strings.TrimSpace(statsRaw)) {
	case "none":
		out.Stats = "none"
	case "select":
		out.Stats = "select"
		if arr, ok := node["stats"].([]any); ok {
			for _, v := range arr {
				key := strings.TrimSpace(fmt.Sprintf("%v", v))
				if key != "" {
					out.StatsKeys = append(out.StatsKeys, key)
				}
			}
		}
	default:
		// if stats is an array of strings, treat as "select"
		if arr, ok := node["stats"].([]any); ok {
			out.Stats = "select"
			for _, v := range arr {
				key := strings.TrimSpace(fmt.Sprintf("%v", v))
				if key != "" {
					out.StatsKeys = append(out.StatsKeys, key)
				}
			}
		} else {
			out.Stats = "all"
		}
	}
	return out
}

func parseBattlePlayerClonePayload(effect map[string]any) battlePlayerClonePayload {
	out := battlePlayerClonePayload{
		MaxCountPerUnit:      maxInt(0, int(toInt64Loose(effect["maxCountPerUnit"], 0))),
		PersistAcrossBattles: parseBooleanLoose(effect["persistAcrossBattles"], false),
		OnCasterDeath:        normalizeBattleSummonOnCasterDeath(fmt.Sprintf("%v", firstNonNil(effect["onCasterDeath"], "keep_until_duration_end"))),
		OnCapOverflow:        normalizeBattleSummonOnCapOverflow(fmt.Sprintf("%v", firstNonNil(effect["onCapOverflow"], "replace_oldest"))),
		OnDispel:             normalizeBattleSummonOnDispel(fmt.Sprintf("%v", firstNonNil(effect["onDispel"], "despawn_immediately"))),
		OnDurationEnd:        normalizeBattleSummonOnDurationEnd(fmt.Sprintf("%v", firstNonNil(effect["onDurationEnd"], "despawn_immediately"))),
		DurationTurns:        maxInt64(0, toInt64Loose(effect["durationTurns"], 0)),
		DurationFormula:      parseBattleSkillAmountFormula(effect["durationFormula"], nil),
		StatModifiers:        map[string]float64{},
		StatsFormula:         parseBattleSkillSummonStatFormulas(effect["statsFormula"]),
		Inherit:              parseBattlePlayerCloneInherit(effect["inherit"]),
		NameSuffix:           strings.TrimSpace(fmt.Sprintf("%v", firstNonNil(effect["nameSuffix"], ""))),
		NameSuffixKey:        strings.TrimSpace(fmt.Sprintf("%v", firstNonNil(effect["nameSuffixKey"], ""))),
		NameSeparator:        strings.TrimSpace(fmt.Sprintf("%v", firstNonNil(effect["nameSeparator"], ""))),
	}
	if rawMods := normalizeAnyMap(effect["statModifiers"]); len(rawMods) > 0 {
		for k, v := range rawMods {
			attr := strings.TrimSpace(k)
			if attr == "" {
				continue
			}
			out.StatModifiers[attr] = toFloat64(v, 0)
		}
	}
	if out.DurationTurns == 0 && out.DurationFormula.BaseAdd == 0 && len(out.DurationFormula.Terms) == 0 {
		out.DurationTurns = 5
	}
	return out
}

func parseBattleSkillSummonPayload(raw any) battleSkillSummonPayload {
	node := normalizeAnyMap(raw)
	if len(node) == 0 {
		return battleSkillSummonPayload{
			CountFormula:  battleSkillAmountFormula{BaseAdd: 1},
			OnCasterDeath: battleSummonOnCasterDeathKeep,
			OnCapOverflow: battleSummonOnCapOverflowReplace,
			OnDispel:      battleSummonOnDispelDespawn,
			OnDurationEnd: battleSummonOnDurationEndDespawn,
		}
	}
	out := battleSkillSummonPayload{
		UnitID:               battleStringFromAny(firstNonNil(node["unitId"], node["enemyId"])),
		Side:                 normalizeBattleSide(fmt.Sprintf("%v", node["side"])),
		CountFormula:         parseBattleSkillAmountFormula(node["countFormula"], nil),
		DurationFormula:      parseBattleSkillAmountFormula(node["durationFormula"], nil),
		DurationTurns:        maxInt64(0, toInt64Loose(node["durationTurns"], 0)),
		MaxCountPerUnit:      maxInt(0, int(toInt64Loose(node["maxCountPerUnit"], 0))),
		MaxCountFormula:      parseBattleSkillAmountFormula(node["maxCountFormula"], nil),
		PersistAcrossBattles: parseBooleanLoose(node["persistAcrossBattles"], false),
		OnCasterDeath:        normalizeBattleSummonOnCasterDeath(fmt.Sprintf("%v", node["onCasterDeath"])),
		OnCapOverflow:        normalizeBattleSummonOnCapOverflow(fmt.Sprintf("%v", node["onCapOverflow"])),
		OnDispel:             normalizeBattleSummonOnDispel(fmt.Sprintf("%v", node["onDispel"])),
		OnDurationEnd:        normalizeBattleSummonOnDurationEnd(fmt.Sprintf("%v", node["onDurationEnd"])),
		StatsFormula:         parseBattleSkillSummonStatFormulas(node["statsFormula"]),
	}
	if out.CountFormula.BaseAdd == 0 && len(out.CountFormula.Terms) == 0 {
		out.CountFormula.BaseAdd = 1
	}
	return out
}

func battleResolveSummonMaxCount(actor *battleUnitState, skillLevel int, payload battleSkillSummonPayload, sourceEnhanceLevel int) int {
	fallback := maxInt(0, payload.MaxCountPerUnit)
	formula := payload.MaxCountFormula
	if formula.BaseAdd == 0 && len(formula.Terms) == 0 {
		return fallback
	}
	resolved := math.Floor(battleEvaluateAmountFormulaWithEnhance(actor, skillLevel, formula, sourceEnhanceLevel))
	if math.IsNaN(resolved) || math.IsInf(resolved, 0) {
		return fallback
	}
	if resolved < 1 {
		return 1
	}
	if resolved > float64(math.MaxInt32) {
		return math.MaxInt32
	}
	return int(resolved)
}

func parseBattleStatusIDList(raw any) []string {
	ids := make([]string, 0, 4)
	appendID := func(value string) {
		id := strings.TrimSpace(value)
		if id == "" {
			return
		}
		for _, existing := range ids {
			if existing == id {
				return
			}
		}
		ids = append(ids, id)
	}
	switch typed := raw.(type) {
	case []any:
		for _, value := range typed {
			appendID(fmt.Sprintf("%v", value))
		}
	case []string:
		for _, value := range typed {
			appendID(value)
		}
	case string:
		for _, part := range strings.Split(typed, ",") {
			appendID(part)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	return ids
}

func parseBattleSkillCastGuards(raw any) []battleSkillCastGuard {
	items := make([]any, 0, 4)
	switch typed := raw.(type) {
	case []any:
		items = append(items, typed...)
	case []map[string]any:
		for _, row := range typed {
			items = append(items, row)
		}
	}
	if len(items) == 0 {
		return nil
	}
	out := make([]battleSkillCastGuard, 0, len(items))
	for _, item := range items {
		node, ok := item.(map[string]any)
		if !ok || len(node) == 0 {
			continue
		}
		kind := strings.ToLower(strings.TrimSpace(fmt.Sprintf("%v", node["kind"])))
		guard := battleSkillCastGuard{Kind: kind}
		switch kind {
		case "enemy_side_not_has_effect":
			guard.EffectKind = battleNormalizeTrigger(fmt.Sprintf("%v", firstNonNil(node["effectKind"], node["effect"])))
			if guard.EffectKind == "" {
				continue
			}
		case "ally_side_has_non_full_hp":
			// no extra fields
		default:
			continue
		}
		out = append(out, guard)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func parseBattleEffectReferenceIndexes(raw any) []int {
	refs := make([]int, 0, 4)
	appendRef := func(value int) {
		if value <= 0 {
			return
		}
		for _, existing := range refs {
			if existing == value {
				return
			}
		}
		refs = append(refs, value)
	}
	switch typed := raw.(type) {
	case []any:
		for _, value := range typed {
			appendRef(int(toInt64Loose(value, 0)))
		}
	case []int:
		for _, value := range typed {
			appendRef(value)
		}
	case []int64:
		for _, value := range typed {
			appendRef(int(value))
		}
	case string:
		for _, part := range strings.Split(typed, ",") {
			appendRef(int(toInt64Loose(strings.TrimSpace(part), 0)))
		}
	default:
		appendRef(int(toInt64Loose(raw, 0)))
	}
	if len(refs) == 0 {
		return nil
	}
	return refs
}

func parseBattleSkillEffects(raw any, fallbackTargeting battleSkillTargeting, fallbackDamageType string) []battleSkillEffect {
	var items []any
	switch typed := raw.(type) {
	case []any:
		items = typed
	case []map[string]any:
		items = make([]any, 0, len(typed))
		for _, row := range typed {
			items = append(items, row)
		}
	default:
		return nil
	}
	if len(items) == 0 {
		return nil
	}
	out := make([]battleSkillEffect, 0, len(items))
	for itemIndex, item := range items {
		node, ok := item.(map[string]any)
		if !ok || len(node) == 0 {
			continue
		}
		kind := strings.ToLower(strings.TrimSpace(fmt.Sprintf("%v", node["kind"])))
		if kind == "remove_status" {
			kind = "remove_buff"
		}
		if kind != "damage" && kind != "heal" && kind != "resource" && kind != "status" && kind != "summon" && kind != "corpse_explosion" && kind != "adjust_skill_cooldown" && kind != "weighted_random_per_target" && kind != "remove_buff" {
			continue
		}
		effect := battleSkillEffect{
			ID:   strings.TrimSpace(fmt.Sprintf("%v", node["id"])),
			Kind: kind,
			Targeting: parseBattleSkillTargeting(
				node["targeting"],
				firstNonEmptyTargetPosition(fallbackTargeting.TargetPositions),
				fallbackTargeting.SecondarySeek,
				maxInt(1, fallbackTargeting.TargetCount.Base),
				fallbackTargeting.TargetCount.Formula,
				fallbackTargeting.TargetCount.Max,
				fallbackTargeting.TargetCount.MaxSet,
				fallbackTargeting.TargetPredicates,
			),
			IgnoreUntargetable: parseBooleanLoose(node["ignoreUntargetable"], false),
		}
		if effect.ID == "<nil>" {
			effect.ID = ""
		}
		if effect.ID == "" {
			effect.ID = fmt.Sprintf("effect_%d", itemIndex+1)
		}
		if kind == "corpse_explosion" {
			effect.CorpseTargeting = parseBattleSkillTargeting(
				firstNonNil(node["corpseTargeting"], node["corpse_targeting"]),
				firstNonEmptyTargetPosition(effect.Targeting.TargetPositions),
				effect.Targeting.SecondarySeek,
				1,
				nil,
				1,
				true,
				nil,
			)
			effect.ConsumeCorpse = parseBooleanLoose(node["consumeCorpse"], true)
		}
		effect.OnlyWhenNoTargetFromEffects = parseBattleEffectReferenceIndexes(node["onlyWhenNoTargetFromEffects"])
		effect.ReuseTargetsFromEffect = int(toInt64Loose(node["reuseTargetsFromEffect"], 0))
		effect.ConsumeStatusesOnHit = parseBattleStatusIDList(node["consumeStatusesOnHit"])
		effect.ConsumeCasterStatusesOnHit = parseBattleStatusIDList(firstNonNil(node["consumeCasterStatusesOnHit"], node["consumeSourceStatusesOnHit"]))
		effect.RequiredCasterStatuses = parseBattleStatusIDList(firstNonNil(node["requiredCasterStatuses"], node["requireCasterStatuses"], node["casterStatuses"]))
		effect.RequiredTargetStatuses = parseBattleStatusIDList(firstNonNil(node["requiredTargetStatuses"], node["requireTargetStatuses"], node["targetStatuses"]))
		effect.ForbiddenCasterStatuses = parseBattleStatusIDList(firstNonNil(node["forbiddenCasterStatuses"], node["excludeCasterStatuses"]))
		effect.ForbiddenTargetStatuses = parseBattleStatusIDList(firstNonNil(node["forbiddenTargetStatuses"], node["excludeTargetStatuses"]))
		effect.MpCostFormula = parseBattleSkillAmountFormula(node["mpCost"], nil)
		if effect.ReuseTargetsFromEffect <= 0 {
			effect.ReuseTargetsFromEffect = 0
		}
		if kind == "damage" || kind == "heal" || kind == "resource" || kind == "corpse_explosion" {
			effect.DamageType = normalizeBattleDamageType(firstNonEmptyString(fmt.Sprintf("%v", node["damageType"]), fallbackDamageType))
			effect.AmountFormula = parseBattleSkillAmountFormula(node["amountFormula"], nil)
			if kind == "resource" {
				effect.Resource = strings.ToLower(strings.TrimSpace(fmt.Sprintf("%v", firstNonNil(node["resource"], "mp"))))
				if effect.Resource != "hp" && effect.Resource != "mp" {
					effect.Resource = "mp"
				}
			}
			if kind != "resource" && kind != "corpse_explosion" {
				effect.DamageSpec = parseBattleDamageSpec(node["damageSpec"], effect.DamageType, effect.AmountFormula)
			}
			if kind == "damage" {
				effect.DamageType = effect.DamageSpec.DamageType
			}
		}
		if kind == "status" {
			effect.StatusPayload = parseBattleSkillStatusPayload(node["statusPayload"])
		}
		if kind == "weighted_random_per_target" {
			effect.StatusPayload = parseBattleWeightedStatusChance(node)
			effect.WeightedStatusChoices = parseBattleWeightedStatusChoices(node["choices"])
			if len(effect.WeightedStatusChoices) == 0 {
				continue
			}
		}
		if kind == "remove_buff" {
			effect.BuffIDs = parseBattleStatusIDList(firstNonNil(node["buffIds"], node["statusIds"], node["buffId"], node["statusId"]))
			effect.MaxRemoved = int(toInt64Loose(node["maxRemoved"], int64(len(effect.BuffIDs))))
			if effect.MaxRemoved <= 0 {
				effect.MaxRemoved = len(effect.BuffIDs)
			}
			if len(effect.BuffIDs) == 0 {
				continue
			}
		}
		if kind == "summon" {
			effect.SummonPayload = parseBattleSkillSummonPayload(firstNonNil(node["summonPayload"], node["summon"], node))
		}
		if kind == "adjust_skill_cooldown" {
			effect.CooldownAmount = int(toInt64Loose(firstNonNil(node["amount"], node["turns"], node["value"]), 0))
			effect.IncludeBasic = parseBooleanLoose(firstNonNil(node["includeBasic"], node["include_basic"]), false)
			effect.TargetSkillIDs = battleParseStringList(firstNonNil(node["skillIds"], node["skills"]))
			effect.ExcludeSkillIDs = battleParseStringList(firstNonNil(node["excludeSkillIds"], node["excludeSkills"]))
		}
		out = append(out, effect)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func parseBattleWeightedStatusChance(node map[string]any) battleSkillStatusPayload {
	raw := normalizeAnyMap(node["statusPayload"])
	if raw == nil {
		raw = map[string]any{}
	}
	for _, key := range []string{"applyChanceBase", "applyChancePerSkillLevel", "applyChanceFormula", "applyChanceMin", "applyChanceMax"} {
		if value, exists := node[key]; exists {
			raw[key] = value
		}
	}
	return parseBattleSkillStatusPayload(raw)
}

func parseBattleWeightedStatusChoices(raw any) []battleWeightedStatusChoice {
	items, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := make([]battleWeightedStatusChoice, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		node := normalizeAnyMap(item)
		id := strings.TrimSpace(fmt.Sprintf("%v", node["id"]))
		statusID := strings.TrimSpace(fmt.Sprintf("%v", firstNonNil(node["statusId"], node["buffId"])))
		weight := toFloat64(node["weight"], 0)
		durationTurns := int(toInt64Loose(node["durationTurns"], 0))
		if id == "" || id == "<nil>" || statusID == "" || statusID == "<nil>" || weight <= 0 || durationTurns <= 0 {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, battleWeightedStatusChoice{
			ID: id, Weight: weight, StatusID: statusID, DurationTurns: durationTurns, Payload: normalizeAnyMap(node["payload"]),
		})
	}
	return out
}

func battleChooseWeightedStatus(state *battleRuntimeState, choices []battleWeightedStatusChoice) (battleWeightedStatusChoice, bool) {
	if state == nil || len(choices) == 0 {
		return battleWeightedStatusChoice{}, false
	}
	total := 0.0
	for _, choice := range choices {
		if choice.Weight > 0 {
			total += choice.Weight
		}
	}
	if total <= 0 {
		return battleWeightedStatusChoice{}, false
	}
	roll := state.randFloat64() * total
	cumulative := 0.0
	for _, choice := range choices {
		if choice.Weight <= 0 {
			continue
		}
		cumulative += choice.Weight
		if roll < cumulative {
			return choice, true
		}
	}
	return choices[len(choices)-1], true
}

func firstNonEmptyTargetPosition(values []string) string {
	for _, value := range values {
		if normalized := normalizeBattleTargetPosition(value); normalized != "" {
			return normalized
		}
	}
	return "vanguard"
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func toBattleSkillUnitRuntimeList(units []battleUnitState) []battleskills.UnitRuntime {
	out := make([]battleskills.UnitRuntime, 0, len(units))
	for _, unit := range units {
		out = append(out, toBattleSkillUnitRuntime(unit))
	}
	return out
}

func cloneFloatMap(source map[string]float64) map[string]float64 {
	out := make(map[string]float64, len(source))
	for key, value := range source {
		out[key] = value
	}
	return out
}

func cloneAnyMap(source map[string]any) map[string]any {
	if len(source) == 0 {
		return nil
	}
	out := make(map[string]any, len(source))
	for key, value := range source {
		out[key] = value
	}
	return out
}

func battleSummonIDToken(raw string) string {
	trimmed := strings.ToLower(strings.TrimSpace(raw))
	if trimmed == "" {
		return "unknown"
	}
	var builder strings.Builder
	for _, ch := range trimmed {
		if (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') {
			builder.WriteRune(ch)
			continue
		}
		builder.WriteRune('_')
	}
	token := strings.Trim(builder.String(), "_")
	if token == "" {
		return "unknown"
	}
	return token
}

func battleSummonUnitIDPrefix(sourceUnitID, templateUnitID string) string {
	return fmt.Sprintf("summon_%s_%s_", battleSummonIDToken(sourceUnitID), battleSummonIDToken(templateUnitID))
}

func battleUnitIsSummonedBy(unit *battleUnitState, templateUnitID string, sourceUnitID string) bool {
	if unit == nil || !unit.Alive || unit.HP <= 0 {
		return false
	}
	templateID := strings.TrimSpace(templateUnitID)
	sourceID := strings.TrimSpace(sourceUnitID)
	if templateID == "" || sourceID == "" {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(unit.SummonTemplateUnitID), templateID) &&
		strings.EqualFold(strings.TrimSpace(unit.SummonSourceUnitID), sourceID) {
		return true
	}
	return strings.HasPrefix(strings.TrimSpace(unit.UnitID), battleSummonUnitIDPrefix(sourceID, templateID))
}

func battleUnitIsSummon(unit *battleUnitState) bool {
	if unit == nil {
		return false
	}
	if strings.TrimSpace(unit.SummonTemplateUnitID) != "" && strings.TrimSpace(unit.SummonSourceUnitID) != "" {
		return true
	}
	return strings.HasPrefix(strings.TrimSpace(unit.UnitID), "summon_")
}

func battleCountSummonsByTemplateAndSource(state *battleRuntimeState, templateUnitID string, sourceUnitID string) int {
	if state == nil || strings.TrimSpace(templateUnitID) == "" || strings.TrimSpace(sourceUnitID) == "" {
		return 0
	}
	count := 0
	for i := range state.Players {
		if battleUnitIsSummonedBy(&state.Players[i], templateUnitID, sourceUnitID) {
			count++
		}
	}
	for i := range state.Enemies {
		if battleUnitIsSummonedBy(&state.Enemies[i], templateUnitID, sourceUnitID) {
			count++
		}
	}
	return count
}

func battleFindOldestSummonByTemplateAndSource(state *battleRuntimeState, templateUnitID string, sourceUnitID string) *battleUnitState {
	if state == nil {
		return nil
	}
	var oldest *battleUnitState
	chooseOldest := func(candidate *battleUnitState) {
		if !battleUnitIsSummonedBy(candidate, templateUnitID, sourceUnitID) {
			return
		}
		if oldest == nil {
			oldest = candidate
			return
		}
		if candidate.SummonOrder > 0 && oldest.SummonOrder > 0 {
			if candidate.SummonOrder < oldest.SummonOrder {
				oldest = candidate
			}
			return
		}
		if strings.Compare(strings.TrimSpace(candidate.UnitID), strings.TrimSpace(oldest.UnitID)) < 0 {
			oldest = candidate
		}
	}
	for i := range state.Players {
		chooseOldest(&state.Players[i])
	}
	for i := range state.Enemies {
		chooseOldest(&state.Enemies[i])
	}
	return oldest
}

func battleNextSummonUnitID(state *battleRuntimeState, sourceUnitID, templateUnitID string) string {
	prefix := battleSummonUnitIDPrefix(sourceUnitID, templateUnitID)
	for seq := 1; seq < 1000000; seq++ {
		candidate := fmt.Sprintf("%s%d", prefix, seq)
		if battleUnitByID(state, candidate) == nil {
			return candidate
		}
	}
	return fmt.Sprintf("%s%d", prefix, time.Now().UnixNano())
}

func (s *Server) buildBattleSkillSummonRuntime(
	state *battleRuntimeState,
	actor *battleUnitState,
	payload battleSkillSummonPayload,
	skillLevel int,
	sourceEnhanceLevel int,
) (battleskills.SummonRuntime, bool) {
	if s == nil || state == nil || actor == nil {
		return battleskills.SummonRuntime{}, false
	}
	templateUnitID := strings.TrimSpace(payload.UnitID)
	if templateUnitID == "" {
		return battleskills.SummonRuntime{}, false
	}
	enemyCfg, ok := s.battleEnemyCatalog()[templateUnitID]
	if !ok {
		return battleskills.SummonRuntime{}, false
	}
	baseUnit := s.buildBattleEnemyUnit(enemyCfg, battleNextSummonUnitID(state, actor.UnitID, templateUnitID), false, s.battleDefaultBasicSkillID())
	side := normalizeBattleSide(payload.Side)
	if side == "" {
		side = normalizeBattleSide(actor.Side)
	}
	if side != "player" && side != "enemy" {
		return battleskills.SummonRuntime{}, false
	}
	baseUnit.Side = side
	baseUnit.OriginSide = side
	baseUnit.BaseSide = side
	baseUnit.PerspectiveSide = side
	baseUnit.IsPlayerUnit = actor.IsPlayerUnit
	s.applyEnemyHpRegenRatio(&baseUnit)
	stats := cloneFloatMap(baseUnit.Stats)
	formulaActor := battleArenaSummonFormulaSource(state, actor)
	if len(payload.StatsFormula) > 0 {
		for attrKey, formula := range payload.StatsFormula {
			attr := strings.TrimSpace(attrKey)
			if attr == "" {
				continue
			}
			delta := battleEvaluateAmountFormulaWithEnhance(formulaActor, skillLevel, formula, sourceEnhanceLevel)
			if math.IsNaN(delta) || math.IsInf(delta, 0) {
				continue
			}
			stats[attr] += delta
		}
	}
	durationTurns := payload.DurationTurns
	if payload.DurationFormula.BaseAdd != 0 || len(payload.DurationFormula.Terms) > 0 {
		durationTurns = int64(math.Floor(battleEvaluateAmountFormulaWithEnhance(formulaActor, skillLevel, payload.DurationFormula, sourceEnhanceLevel)))
		if durationTurns < 1 {
			durationTurns = 1
		}
	}
	hp := maxFloat(1, stats["hp"])
	mp := maxFloat(0, stats["mp"])
	stats["hp"] = hp
	stats["mp"] = mp
	return battleskills.SummonRuntime{
		UnitID:               baseUnit.UnitID,
		Side:                 side,
		Name:                 baseUnit.Name,
		NameKey:              baseUnit.NameKey,
		Position:             baseUnit.Position,
		HP:                   hp,
		MP:                   mp,
		MaxHP:                hp,
		MaxMP:                mp,
		DurationTurns:        durationTurns,
		TemplateUnitID:       templateUnitID,
		SourceUnitID:         actor.UnitID,
		MaxCountPerUnit:      battleResolveSummonMaxCount(formulaActor, skillLevel, payload, sourceEnhanceLevel),
		PersistAcrossBattles: payload.PersistAcrossBattles,
		OnCasterDeath:        normalizeBattleSummonOnCasterDeath(payload.OnCasterDeath),
		OnCapOverflow:        normalizeBattleSummonOnCapOverflow(payload.OnCapOverflow),
		OnDispel:             normalizeBattleSummonOnDispel(payload.OnDispel),
		OnDurationEnd:        normalizeBattleSummonOnDurationEnd(payload.OnDurationEnd),
		SourceEnhanceLevel:   sourceEnhanceLevel,
		Stats:                stats,
		SkillIDs:             append([]string(nil), baseUnit.SkillIDs...),
		InitialBuffIDs:       append([]string(nil), enemyCfg.InitialBuffIds...),
		AllowNoSkills:        len(enemyCfg.SkillIDs) == 0,
	}, true
}

func addBattleSummon(state *battleRuntimeState, fallbackSide string, summon *battleskills.SummonRuntime, server *Server, now time.Time) bool {
	if state == nil || summon == nil || server == nil {
		return false
	}
	unitID := strings.TrimSpace(summon.UnitID)
	if unitID == "" {
		return false
	}
	if battleUnitByID(state, unitID) != nil {
		return false
	}
	side := strings.ToLower(strings.TrimSpace(summon.Side))
	if side != "player" && side != "enemy" {
		side = strings.ToLower(strings.TrimSpace(fallbackSide))
	}
	if side != "player" && side != "enemy" {
		return false
	}
	maxHP := maxFloat(maxFloat(1, summon.MaxHP), summon.HP)
	maxMP := maxFloat(0, maxFloat(summon.MaxMP, summon.MP))
	skillIDs := make([]string, 0, len(summon.SkillIDs)+1)
	for _, skillID := range summon.SkillIDs {
		skillIDs = appendUniqueSkillID(skillIDs, skillID)
	}
	if len(skillIDs) == 0 && !summon.AllowNoSkills {
		skillIDs = appendUniqueSkillID(skillIDs, server.battleDefaultBasicSkillID())
	}
	skills := make([]battleSkillState, 0, len(skillIDs))
	for _, skillID := range skillIDs {
		skills = append(skills, server.battleSkillByID(skillID))
	}
	unit := battleUnitState{
		UnitID:                     unitID,
		Side:                       side,
		OriginSide:                 side,
		BaseSide:                   side,
		PerspectiveSide:            side,
		Name:                       strings.TrimSpace(summon.Name),
		NameKey:                    strings.TrimSpace(summon.NameKey),
		Position:                   strings.TrimSpace(summon.Position),
		BasePosition:               strings.TrimSpace(summon.Position),
		Alive:                      true,
		RewardEligible:             false,
		HP:                         clampFloat(summon.HP, 0, maxHP),
		MP:                         clampFloat(summon.MP, 0, maxMP),
		MaxHP:                      maxHP,
		MaxMP:                      maxMP,
		Stats:                      cloneFloatMap(summon.Stats),
		StatMode:                   battleStatModeDirect,
		SkillSlots:                 append([]string(nil), skillIDs...),
		SkillIDs:                   append([]string(nil), skillIDs...),
		SkillRules:                 map[string]battleSkillRuleState{},
		ItemRules:                  map[string]battleSkillRuleState{},
		Skills:                     skills,
		BattleItems:                []battleItemState{},
		SummonRemainTurns:          maxInt64(0, summon.DurationTurns),
		SummonTemplateUnitID:       strings.TrimSpace(summon.TemplateUnitID),
		SummonSourceUnitID:         strings.TrimSpace(summon.SourceUnitID),
		SummonMaxCountPerUnit:      maxInt(0, summon.MaxCountPerUnit),
		SummonPersistAcrossBattles: summon.PersistAcrossBattles,
		SummonOnCasterDeath:        normalizeBattleSummonOnCasterDeath(summon.OnCasterDeath),
		SummonOnCapOverflow:        normalizeBattleSummonOnCapOverflow(summon.OnCapOverflow),
		SummonOnDispel:             normalizeBattleSummonOnDispel(summon.OnDispel),
		SummonOnDurationEnd:        normalizeBattleSummonOnDurationEnd(summon.OnDurationEnd),
		SummonSourceEnhanceLevel:   summon.SourceEnhanceLevel,
		SummonOrder:                state.Seq + 1,
	}
	for _, buffID := range summon.InitialBuffIDs {
		buffID = strings.TrimSpace(buffID)
		if buffID == "" {
			continue
		}
		durationTurns := server.battleBuffDurationByIDTurns(buffID)
		if durationTurns == 0 {
			continue
		}
		battleApplyOrRefreshBuff(&unit, buffID, durationTurns, nil, "initial", server.battleBuffStackPolicyByID(buffID), server.battleBuffMaxStacksByID(buffID))
	}
	if !unit.Alive || unit.HP <= 0 {
		unit.HP = 0
		unit.Alive = false
	}
	if strings.TrimSpace(unit.SummonTemplateUnitID) == "" {
		unit.SummonTemplateUnitID = strings.TrimSpace(summon.TemplateUnitID)
	}
	if strings.TrimSpace(unit.SummonSourceUnitID) == "" {
		unit.SummonSourceUnitID = strings.TrimSpace(summon.SourceUnitID)
	}
	if strings.TrimSpace(unit.SummonTemplateUnitID) != "" && strings.TrimSpace(unit.SummonSourceUnitID) != "" {
		remainTurns := summon.DurationTurns
		if remainTurns == 0 {
			remainTurns = -1
		}
		summonPayload := map[string]any{
			"templateUnitId":       unit.SummonTemplateUnitID,
			"sourceUnitId":         unit.SummonSourceUnitID,
			"maxCountPerUnit":      unit.SummonMaxCountPerUnit,
			"persistAcrossBattles": unit.SummonPersistAcrossBattles,
			"onCasterDeath":        unit.SummonOnCasterDeath,
			"onCapOverflow":        unit.SummonOnCapOverflow,
			"onDispel":             unit.SummonOnDispel,
			"onDurationEnd":        unit.SummonOnDurationEnd,
			"sourceEnhanceLevel":   unit.SummonSourceEnhanceLevel,
			"summonOrder":          unit.SummonOrder,
		}
		battleApplyOrRefreshBuff(&unit, battleSummonBindingBuffID, remainTurns, summonPayload, unit.SummonSourceUnitID, "refresh_duration")
	}
	if side == "player" {
		state.Players = append(state.Players, unit)
		if sourceUnit := battleUnitByID(state, strings.TrimSpace(unit.SummonSourceUnitID)); sourceUnit != nil {
			server.battleTriggerAfterSummonHooks(state, now, sourceUnit, &state.Players[len(state.Players)-1])
		}
		server.battleRecomputeUnitStats(&state.Players[len(state.Players)-1])
	} else {
		state.Enemies = append(state.Enemies, unit)
		if sourceUnit := battleUnitByID(state, strings.TrimSpace(unit.SummonSourceUnitID)); sourceUnit != nil {
			server.battleTriggerAfterSummonHooks(state, now, sourceUnit, &state.Enemies[len(state.Enemies)-1])
		}
		server.battleRecomputeUnitStats(&state.Enemies[len(state.Enemies)-1])
	}
	state.Order = []string{}
	state.rebuildOrderIfNeeded()
	return true
}

func (s *Server) battleTriggerAfterSummonHooks(
	state *battleRuntimeState,
	now time.Time,
	sourceUnit *battleUnitState,
	summonedUnit *battleUnitState,
) {
	if s == nil || state == nil || sourceUnit == nil || summonedUnit == nil {
		return
	}
	battleCombatTrialRegisterPlayerSource(state, summonedUnit)
	s.battleQueueArenaFormatAfterSummon(state, sourceUnit, summonedUnit)
	if state.afterSummonHookDepth >= battleMaxAfterSummonHookDepth {
		return
	}
	state.afterSummonHookDepth++
	defer func() {
		state.afterSummonHookDepth--
	}()
	s.battleTriggerUnitEquipmentHooksWithTarget(state, now, sourceUnit, summonedUnit, summonedUnit, battleEquipmentTriggerAfterSummon, 0, "")
}

func toCoreSkillState(skill battleSkillState) battlecore.SkillState {
	targetDirection := firstNonEmptyTargetPosition(skill.Targeting.TargetPositions)
	targetSeek := normalizeBattleSecondarySeek(skill.Targeting.SecondarySeek, "front_to_rear")
	targetCount := maxInt(1, resolveBattleTemplateTargetCount(nil, skill.Level, skill.Targeting))
	return battlecore.SkillState{
		ID:                 skill.ID,
		HPCost:             skill.HPCost,
		MPCost:             skill.MPCost,
		CooldownTurns:      skill.CooldownTurns,
		CooldownRemain:     skill.CooldownRemain,
		DamageCoeff:        1,
		DamageType:         "physical",
		TargetDirection:    targetDirection,
		TargetSeekStrategy: targetSeek,
		TargetMode:         "single",
		TargetBaseCount:    maxInt(1, skill.Targeting.TargetCount.Base),
		TargetFormula:      toCoreAttackTerms(skill.Targeting.TargetCount.Formula),
		TargetCount:        targetCount,
	}
}

func fromCoreSkillState(skill battlecore.SkillState) battleSkillState {
	targeting := battleSkillTargeting{
		TargetPositions: []string{normalizeBattleTargetPosition(skill.TargetDirection)},
		SecondarySeek:   normalizeBattleSecondarySeek(skill.TargetSeekStrategy, "front_to_rear"),
		TargetScope:     "enemy",
		SelectRule:      "random",
		TargetCount: battleSkillTargetCount{
			Base:    maxInt(1, skill.TargetBaseCount),
			Formula: fromCoreAttackTerms(skill.TargetFormula),
		},
	}
	if targeting.TargetPositions[0] == "" {
		targeting.TargetPositions[0] = "vanguard"
	}
	return battleSkillState{
		ID:             skill.ID,
		Name:           skill.ID,
		HPCost:         skill.HPCost,
		MPCost:         skill.MPCost,
		CooldownTurns:  skill.CooldownTurns,
		CooldownRemain: skill.CooldownRemain,
		Targeting:      targeting,
	}
}

func toCoreUnitState(unit battleUnitState) battlecore.UnitState {
	skills := make([]battlecore.SkillState, 0, len(unit.Skills))
	for _, skill := range unit.Skills {
		skills = append(skills, toCoreSkillState(skill))
	}
	return battlecore.UnitState{
		UnitID:   unit.UnitID,
		Position: unit.Position,
		Alive:    unit.Alive,
		HP:       unit.HP,
		MP:       unit.MP,
		MaxHP:    unit.MaxHP,
		MaxMP:    unit.MaxMP,
		Stats:    unit.Stats,
		Skills:   skills,
	}
}

func roundTo4(value float64) float64 {
	return math.Round(value*10000) / 10000
}

func toFloat64(value any, fallback float64) float64 {
	switch typed := value.(type) {
	case float64:
		return typed
	case float32:
		return float64(typed)
	case int:
		return float64(typed)
	case int64:
		return float64(typed)
	case int32:
		return float64(typed)
	case string:
		typed = strings.TrimSpace(typed)
		if typed == "" {
			return fallback
		}
		parsed, err := strconv.ParseFloat(typed, 64)
		if err != nil {
			return fallback
		}
		return parsed
	default:
		return fallback
	}
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func battleMaxHPFromStat(hp float64) float64 {
	return maxFloat(0, hp)
}

func clampFloat(value, min, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func applyBattleDamageStatModifiers(source *battleUnitState, target *battleUnitState, incoming float64, damageType string) float64 {
	return applyBattleDamageStatModifiersWithExtraAmplification(source, target, incoming, damageType, 0)
}

const battleDamageResistCap = 0.999

func battleCritStatsForDamageType(source *battleUnitState, target *battleUnitState, damageType string) (float64, float64, float64) {
	return battleCritStatsForDamageTypeByProfile(source, target, battleDamageChannelProfileForType(damageType))
}

func battleCritStatsForDamageTypeNormalized(source *battleUnitState, target *battleUnitState, isMagical bool) (float64, float64, float64) {
	return battleCritStatsForDamageTypeByProfile(source, target, battleDamageChannelProfileByIsMagical(isMagical))
}

func battleCritStatsForDamageTypeByProfile(source *battleUnitState, target *battleUnitState, profile battleDamageChannelProfile) (float64, float64, float64) {
	if source == nil {
		return 0, 0, 0
	}
	return source.Stats[profile.CritChanceStatKey], source.Stats[profile.CritDamageStatKey], targetCritDamageResist(target)
}

func battleCritChanceForDamageType(source *battleUnitState, damageType string) float64 {
	critChance, _, _ := battleCritStatsForDamageType(source, nil, damageType)
	return critChance
}

func battleCritDamageForDamageType(source *battleUnitState, damageType string) float64 {
	_, critDamage, _ := battleCritStatsForDamageType(source, nil, damageType)
	return critDamage
}

func battleCritChanceForDamageTypeNormalized(source *battleUnitState, isMagical bool) float64 {
	critChance, _, _ := battleCritStatsForDamageTypeByProfile(source, nil, battleDamageChannelProfileByIsMagical(isMagical))
	return critChance
}

func battleCritDamageForDamageTypeNormalized(source *battleUnitState, isMagical bool) float64 {
	_, critDamage, _ := battleCritStatsForDamageTypeByProfile(source, nil, battleDamageChannelProfileByIsMagical(isMagical))
	return critDamage
}

func targetCritDamageResist(target *battleUnitState) float64 {
	if target == nil {
		return 0
	}
	return target.Stats["crit_damage_resist"]
}

func applyBattleDamageStatModifiersWithExtraAmplification(source *battleUnitState, target *battleUnitState, incoming float64, damageType string, extraAmplification float64) float64 {
	return applyBattleDamageStatModifiersWithExtraAmplificationAndResistPierce(source, target, incoming, damageType, extraAmplification, 0)
}

func normalizeBattleDamageReductionPierce(value float64) float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0
	}
	return clampFloat(value, 0, 1)
}

func battleEnemyOnlyDamageReductionPierce(source, target *battleUnitState, configured float64, targetStatusIDs []string) float64 {
	if source == nil || target == nil {
		return 0
	}
	sourceBaseSide := normalizeBattleSide(source.BaseSide)
	if sourceBaseSide == "" {
		sourceBaseSide = normalizeBattleSide(source.OriginSide)
	}
	if sourceBaseSide == "" {
		sourceBaseSide = normalizeBattleSide(source.Side)
	}
	if sourceBaseSide != "enemy" {
		return 0
	}
	pierce := normalizeBattleDamageReductionPierce(configured)
	if pierce <= 0 {
		return 0
	}
	statuses := parseBattleStringSlice(targetStatusIDs)
	if len(statuses) > 0 && !battleUnitHasAnyStatus(target, statuses) {
		return 0
	}
	return pierce
}

func applyBattleDamageStatModifiersWithExtraAmplificationAndResistPierce(
	source *battleUnitState,
	target *battleUnitState,
	incoming float64,
	damageType string,
	extraAmplification float64,
	damageReductionPierce float64,
) float64 {
	damage := maxFloat(0, incoming)
	if target == nil || damage <= 0 {
		return damage
	}
	profile := battleDamageChannelProfileForType(damageType)
	amp := 0.0
	if source != nil {
		amp = source.Stats[profile.DamageAmplificationStatKey]
	}
	resist := target.Stats[profile.DamageResistStatKey]
	if resist > battleDamageResistCap {
		resist = battleDamageResistCap
	}
	if resist > 0 {
		resist *= 1 - normalizeBattleDamageReductionPierce(damageReductionPierce)
	}
	amp += extraAmplification
	damage = damage * maxFloat(0, 1+amp)
	damage = damage * maxFloat(0, 1-resist)
	return maxFloat(0, damage)
}

func (s *Server) battleChooseHigherFinalDamageType(state *battleRuntimeState, source, target *battleUnitState, spec battleDamageSpec, skillLevel int) string {
	if source == nil || target == nil {
		return normalizeBattleDamageType(spec.DamageType)
	}
	baseAttack := math.Floor(battleEvaluateAmountFormula(source, skillLevel, spec.AmountFormula))
	defenseK := s.battleDefenseKForState(state)
	percentPenCap := s.deps.Config.SharedGlobalSettings.CombatBalanceModel.PercentPenetrationCap
	if percentPenCap <= 0 || percentPenCap > 1 {
		percentPenCap = 0.8
	}
	targetTypeBonus := battleTargetTypeBonus(target, spec)
	extraAmp := battleEvaluateAmountFormula(source, skillLevel, spec.ExtraDamageAmplificationFormula) + targetTypeBonus
	damageReductionPierce := battleEnemyOnlyDamageReductionPierce(
		source,
		target,
		spec.EnemyOnlyDamageReductionPierce,
		spec.EnemyOnlyDamageReductionPierceTargetStatuses,
	)
	preview := func(profile battleDamageChannelProfile) float64 {
		flatPen, percentPen, _, _ := battleResolveAttackerPenetrationNormalized(source, profile.IsMagical)
		specFlatPen, specPercentPen := battleResolveSpecPenetrationNormalized(spec, source, skillLevel, profile.IsMagical)
		effectiveDefense := battleResolveEffectiveDefense(
			target.Stats[profile.DefenseStatKey],
			maxFloat(0, flatPen+specFlatPen),
			clampFloat(percentPen+specPercentPen, 0, percentPenCap),
		)
		effectiveDefense = maxFloat(effectiveDefense, -defenseK/2)
		baseDamage := baseAttack * defenseK / (defenseK + effectiveDefense)
		return applyBattleDamageStatModifiersWithExtraAmplificationAndResistPierce(source, target, baseDamage, profile.ID, extraAmp, damageReductionPierce)
	}
	physical := preview(battlePhysicalDamageChannelProfile)
	magical := preview(battleMagicalDamageChannelProfile)
	if magical > physical {
		return "magical"
	}
	return "physical"
}

func normalizeBattleUnitState(unit *battleUnitState) {
	if unit == nil {
		return
	}
	if unit.Stats == nil {
		unit.Stats = map[string]float64{}
	}
	if unit.BaseStats == nil {
		unit.BaseStats = cloneFloatMap(unit.Stats)
	}
	if unit.StatModifiers == nil {
		unit.StatModifiers = map[string]float64{}
	}
	if unit.StatSources == nil {
		unit.StatSources = map[string]map[string]float64{}
	}
	if unit.Skills == nil {
		unit.Skills = []battleSkillState{}
	}
	if unit.SkillSlots == nil {
		unit.SkillSlots = []string{}
	}
	if unit.SkillIDs == nil {
		unit.SkillIDs = []string{}
	}
	if unit.SkillRules == nil {
		unit.SkillRules = map[string]battleSkillRuleState{}
	}
	if unit.EquipmentDigest == nil {
		unit.EquipmentDigest = []string{}
	}
	if unit.EquipmentIDs == nil {
		unit.EquipmentIDs = []int64{}
	}
	if unit.WeaponClasses == nil {
		unit.WeaponClasses = []string{}
	}
	if unit.Buffs == nil {
		unit.Buffs = []battleBuffState{}
	}
	if unit.UnitType == "kitty" && unit.KittyPersonalityHooks == nil {
		unit.KittyPersonalityHooks = []battleEquipmentHookRule{}
	}
	if unit.UnitType == "kitty" && unit.KittyPersonalityEffectMutations == nil {
		unit.KittyPersonalityEffectMutations = []battleAffixEffectMutation{}
	}
	normalizeBattleBasicAttackProfile(unit.BasicAttack)
	normalizeBattleAffixEffectMutations(unit.EffectMutations)
	normalizeBattleAffixEffectMutations(unit.KittyPersonalityEffectMutations)
	normalizeBattleAffixEffectMutations(unit.SpecializationEffectMutations)
	unit.Position = normalizeBattleTargetPosition(unit.Position)
	unit.BasePosition = normalizeBattleTargetPosition(unit.BasePosition)
	unit.Side = normalizeBattleSide(unit.Side)
	unit.OriginSide = normalizeBattleSide(unit.OriginSide)
	unit.BaseSide = normalizeBattleSide(unit.BaseSide)
	unit.PerspectiveSide = normalizeBattleSide(unit.PerspectiveSide)
	if unit.Position == "" {
		unit.Position = "vanguard"
	}
	if unit.BasePosition == "" {
		unit.BasePosition = unit.Position
	}
	if unit.Side == "" {
		unit.Side = "player"
	}
	if unit.OriginSide == "" {
		unit.OriginSide = unit.Side
	}
	if unit.BaseSide == "" {
		unit.BaseSide = unit.OriginSide
	}
	if unit.PerspectiveSide == "" {
		unit.PerspectiveSide = unit.OriginSide
	}
	for idx := range unit.Buffs {
		buff := &unit.Buffs[idx]
		if buff.RemainTurns == 0 && buff.LegacyRemainMS != 0 {
			buff.RemainTurns = battleDurationTurnsFromLegacyMS(buff.LegacyRemainMS, 1000)
		}
		if buff.RemainTurns < 0 {
			buff.RemainTurns = -1
		}
		buff.LegacyRemainMS = 0
	}
	if len(unit.SkillIDs) == 0 && len(unit.Skills) > 0 {
		unit.SkillIDs = make([]string, 0, len(unit.Skills))
		for _, skill := range unit.Skills {
			unit.SkillIDs = appendUniqueSkillID(unit.SkillIDs, skill.ID)
		}
	}
	if len(unit.SkillSlots) == 0 && len(unit.SkillIDs) > 0 {
		unit.SkillSlots = append([]string(nil), unit.SkillIDs...)
	}
	if unit.MaxHP < 0 {
		unit.MaxHP = 0
	}
	if unit.MaxMP < 0 {
		unit.MaxMP = 0
	}
	unit.HP = clampFloat(unit.HP, 0, unit.MaxHP)
	unit.MP = clampFloat(unit.MP, 0, unit.MaxMP)
	if unit.HP > 0 {
		unit.Alive = true
	} else {
		unit.HP = 0
		unit.Alive = false
	}
}

func appendUniqueSkillID(ids []string, skillID string) []string {
	skillID = strings.TrimSpace(skillID)
	if skillID == "" {
		return ids
	}
	for _, row := range ids {
		if strings.EqualFold(strings.TrimSpace(row), skillID) {
			return ids
		}
	}
	return append(ids, skillID)
}

func containsSkillID(ids []string, skillID string) bool {
	skillID = strings.TrimSpace(strings.ToLower(skillID))
	if skillID == "" {
		return false
	}
	for _, row := range ids {
		if strings.TrimSpace(strings.ToLower(row)) == skillID {
			return true
		}
	}
	return false
}

func canonicalSkillID(raw string, owned map[string]string, fallback string) string {
	id := strings.TrimSpace(raw)
	if id == "" {
		id = fallback
	}
	key := strings.ToLower(id)
	if mapped, ok := owned[key]; ok {
		return mapped
	}
	return strings.TrimSpace(fallback)
}

func normalizeCombatSkillSlots(raw []string, owned map[string]string, limit int, defaultBasicSkillID string) []string {
	out := make([]string, 0, len(raw))
	seen := make(map[string]struct{}, len(raw))
	for _, row := range raw {
		key := strings.ToLower(strings.TrimSpace(row))
		if key == "" {
			continue
		}
		canonical, ok := owned[key]
		if !ok {
			continue
		}
		if _, duplicated := seen[key]; duplicated {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, canonical)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	basicKey := strings.ToLower(strings.TrimSpace(defaultBasicSkillID))
	if basicKey != "" {
		if basicCanonical, ok := owned[basicKey]; ok {
			if _, exists := seen[basicKey]; !exists {
				out = append(out, basicCanonical)
				seen[basicKey] = struct{}{}
			}
		}
	}
	if limit > 0 && len(out) > limit {
		if basicKey == "" {
			return out[:limit]
		}
		basicIndex := -1
		for idx, skillID := range out {
			if strings.EqualFold(skillID, defaultBasicSkillID) {
				basicIndex = idx
				break
			}
		}
		if basicIndex < 0 || basicIndex < limit {
			return out[:limit]
		}
		trimmed := make([]string, 0, limit)
		for _, skillID := range out {
			if strings.EqualFold(skillID, defaultBasicSkillID) {
				continue
			}
			trimmed = append(trimmed, skillID)
			if len(trimmed) >= limit-1 {
				break
			}
		}
		trimmed = append(trimmed, defaultBasicSkillID)
		return trimmed
	}
	return out
}

func (s *Server) battleDefaultBasicSkillID() string {
	for _, raw := range s.deps.GameData.CombatConfigRules {
		value, _ := stringFromAny(raw["default_basic_skill_id"])
		skillID := strings.TrimSpace(value)
		if skillID != "" && s.battleSkillExists(skillID) {
			return skillID
		}
	}
	for _, raw := range s.deps.GameData.Skills {
		value, _ := stringFromAny(raw["id"])
		skillID := strings.TrimSpace(value)
		if skillID != "" {
			return skillID
		}
	}
	return ""
}

func (s *Server) battleSkillSlotRule() battleSkillSlotRule {
	rule := battleSkillSlotRule{
		BaseSkillSlots: 4,
		IntPerExtra:    10,
		MinSkillSlots:  1,
		MaxSkillSlots:  8,
	}
	if len(s.deps.GameData.CombatConfigRules) > 0 {
		raw := s.deps.GameData.CombatConfigRules[0]
		if base := int(toInt64Loose(raw["base_skill_slots"], int64(rule.BaseSkillSlots))); base > 0 {
			rule.BaseSkillSlots = base
		}
		if intPer := int(toInt64Loose(raw["int_per_extra_slot"], int64(rule.IntPerExtra))); intPer > 0 {
			rule.IntPerExtra = intPer
		}
		if minSlots := int(toInt64Loose(raw["min_skill_slots"], int64(rule.MinSkillSlots))); minSlots > 0 {
			rule.MinSkillSlots = minSlots
		}
		if maxSlots := int(toInt64Loose(raw["max_skill_slots"], int64(rule.MaxSkillSlots))); maxSlots > 0 {
			rule.MaxSkillSlots = maxSlots
		}
	}
	if rule.MaxSkillSlots < rule.MinSkillSlots {
		rule.MaxSkillSlots = rule.MinSkillSlots
	}
	if rule.BaseSkillSlots < rule.MinSkillSlots {
		rule.BaseSkillSlots = rule.MinSkillSlots
	}
	if rule.BaseSkillSlots > rule.MaxSkillSlots {
		rule.BaseSkillSlots = rule.MaxSkillSlots
	}
	return rule
}

func battleAvailableSkillSlots(intelligence float64, rule battleSkillSlotRule) int {
	base := rule.BaseSkillSlots
	if base <= 0 {
		base = 1
	}
	extra := 0
	if rule.IntPerExtra > 0 {
		extra = int(math.Floor(maxFloat(0, intelligence) / float64(rule.IntPerExtra)))
	}
	total := base + extra
	if total < rule.MinSkillSlots {
		total = rule.MinSkillSlots
	}
	if total > rule.MaxSkillSlots {
		total = rule.MaxSkillSlots
	}
	return total
}

func (s *Server) loadBattleSkillRules(ctx context.Context, userID int64) map[string]battleSkillRuleState {
	sets, err := s.deps.Store.ListRuleSets(ctx, userID, "skill")
	if err != nil {
		return map[string]battleSkillRuleState{}
	}
	out := make(map[string]battleSkillRuleState, len(sets))
	for _, set := range sets {
		slotKey := strings.ToLower(strings.TrimSpace(set.SlotKey))
		if slotKey == "" {
			continue
		}
		buf, err := json.Marshal(set.Groups)
		if err != nil {
			continue
		}
		groups := []rules.ConditionGroup{}
		if err := json.Unmarshal(buf, &groups); err != nil {
			continue
		}
		out[slotKey] = battleSkillRuleState{
			Enabled: true,
			Groups:  groups,
		}
	}
	return out
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func isCombatStat(key string) bool {
	switch key {
	case "hp", "mp", "hp_regen", "mp_regen",
		"strength", "dexterity", "intelligence", "stamina", "luck", "perception",
		"physics_damage", "magical_damage",
		"physics_defence", "magical_defence",
		"speed",
		"crit_chance", "crit_damage", "magic_crit_chance", "magic_crit_damage", "crit_damage_resist",
		"physics_damage_amplification", "magical_damage_amplification",
		"physics_damage_resist", "magical_damage_resist",
		"physics_flat_defense_penetration", "magicial_flat_defense_penetration",
		"physics_percent_defense_penetration", "magicial_percent_defense_penetration":
		return true
	}
	return false
}
