package store

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"math/rand"
	"sort"
	"strings"
	"time"

	"mysecondwebgame/server/internal/catalog"
	"mysecondwebgame/server/internal/config"
	"mysecondwebgame/server/internal/operationeffect"
	fishingeffect "mysecondwebgame/server/internal/operationeffect/fishing"
	"mysecondwebgame/server/internal/operationeffect/ratebonus"

	"github.com/jackc/pgx/v5"
)

const (
	fishingMinimumTripSeconds    = int64(10 * 60)
	fishingMaximumTripSeconds    = int64(24 * 60 * 60)
	fishingMaxFastForwardPhases  = 2_000
	fishingMaxFastForwardCatches = 128
	fishingDefaultSafetyLoad     = 82
	fishingTargetLayerAuto       = "auto"
	fishingTargetLayerSurface    = "surface"
	fishingTargetLayerMidwater   = "midwater"
	fishingTargetLayerNearBottom = "near_bottom"
	fishingPacePatient           = "patient"
	fishingPaceBalanced          = "balanced"
	fishingPaceSearch            = "search"
	fishingPhaseWaiting          = "waiting"
	fishingPhaseBite             = "bite"
	fishingPhaseFighting         = "fighting"
	fishingPhaseLanded           = "landed"
	fishingPhaseResting          = "resting"
	fishingPhaseFinished         = "finished"
	fishingRecentEventLimit      = 24
)

type fishingExecutor struct {
	store *Store
}

type fishingExecutorArgs struct {
	SpotID             string              `json:"spotId"`
	LoadoutID          string              `json:"loadoutId"`
	TournamentDivision string              `json:"tournamentDivision,omitempty"`
	DurationSeconds    int64               `json:"durationSeconds"`
	Strategy           string              `json:"strategy,omitempty"`
	SafetyLoadPercent  int                 `json:"safetyLoadPercent,omitempty"`
	TargetLayer        string              `json:"targetLayer,omitempty"`
	Pace               string              `json:"pace,omitempty"`
	KeepRule           FishingKeepRule     `json:"keepRule"`
	Supplies           FishingSupplyCounts `json:"supplies"`
}

func (fishingExecutorArgs) RequiresSingleQueueRun() bool { return true }

type fishingExecutorRuntime struct {
	CastPassive *operationeffect.PassiveSnapshot `json:"castPassive,omitempty"`

	CastEffects            *operationeffect.Snapshot `json:"castEffects,omitempty"`
	effectSession          *operationeffect.Session  `json:"-"`
	DurationMS             int64                     `json:"durationMs"`
	TripID                 string                    `json:"tripId"`
	ActorType              string                    `json:"actorType"`
	ActorID                int64                     `json:"actorId"`
	WaterID                string                    `json:"waterId"`
	SpotID                 string                    `json:"spotId"`
	TournamentDivision     string                    `json:"tournamentDivision,omitempty"`
	Rules                  fishingRulesSnapshot      `json:"rules"`
	Loadout                FishingLoadoutSnapshot    `json:"loadout"`
	Strategy               string                    `json:"strategy"`
	SafetyLoadPercent      int                       `json:"safetyLoadPercent"`
	TargetLayer            string                    `json:"targetLayer,omitempty"`
	Pace                   string                    `json:"pace,omitempty"`
	KeepRule               FishingKeepRule           `json:"keepRule"`
	PlannedDurationSeconds int64                     `json:"plannedDurationSeconds"`
	PlannedEndAtMS         int64                     `json:"plannedEndAtMs,omitempty"`
	StartedAtMS            int64                     `json:"startedAtMs,omitempty"`
	Phase                  string                    `json:"phase"`
	PhaseStartedAtMS       int64                     `json:"phaseStartedAtMs,omitempty"`
	PhaseEndsAtMS          int64                     `json:"phaseEndsAtMs,omitempty"`
	Seed                   int64                     `json:"seed"`
	EventSeq               int64                     `json:"eventSeq"`
	TimelineSeq            int64                     `json:"timelineSeq,omitempty"`
	Pressure               float64                   `json:"pressure"`
	ReportedBaitUsed       int64                     `json:"reportedBaitUsed,omitempty"`
	CurrentFish            *FishInstanceData         `json:"currentFish,omitempty"`
	CurrentSpeciesRarity   string                    `json:"currentSpeciesRarity,omitempty"`
	CurrentLineLoad        float64                   `json:"currentLineLoad,omitempty"`
	Summary                FishingTripSummary        `json:"summary"`
	Supplies               FishingTripSupplies       `json:"supplies"`
	EquipmentKittyIDs      []int64                   `json:"equipmentKittyIds,omitempty"`
	pendingEvents          []FishingTripEvent        `json:"-"`
	batchRNG               *rand.Rand                `json:"-"`
}

type fishingRulesSnapshot struct {
	RulesetVersion string                           `json:"rulesetVersion"`
	Water          catalog.FishingWater             `json:"water"`
	Spot           catalog.FishingSpot              `json:"spot"`
	Species        []catalog.FishingSpecies         `json:"species"`
	Mutations      []config.FishingMutationSettings `json:"mutations,omitempty"`
}

type FishingConditions struct {
	GameHour     int     `json:"gameHour"`
	Season       string  `json:"season"`
	Weather      string  `json:"weather"`
	Temperature  float64 `json:"temperatureC"`
	Oxygen       float64 `json:"oxygen"`
	Activity     float64 `json:"activity"`
	ActivityBand string  `json:"activityBand"`
	Trend        string  `json:"trend"`
}

type fishingFightResult struct {
	Landed        bool
	LineBreak     bool
	Chance        float64
	LineLoad      float64
	FailureReason string
}

// Offline batching preserves each fish and its deterministic outcome while
// collapsing the per-catch database round trips into bounded writes.
type fishingBatchKnowledgeState struct {
	Exists         bool
	BaseCatches    int64
	EncounterDelta int64
	CatchDelta     int64
	ReleaseDelta   int64
	FirstCaughtAt  *time.Time
	LastCaughtAt   *time.Time
}

type fishingBatchRecordState struct {
	BestLengthCM      float64
	BestLengthCatchID *int64
	BestWeightKG      float64
	BestWeightCatchID *int64
	Dirty             bool
}

type fishingPendingCatch struct {
	Species             catalog.FishingSpecies
	Fish                FishInstanceData
	Record              FishingRecordFlag
	IsNew               bool
	Kept                bool
	Gain                float64
	CaughtAt            time.Time
	Scaleglow           bool
	CatchID             int64
	InventoryInstanceID *int64
}

type fishingFastForwardCatchBatch struct {
	RulesetVersion     string
	TournamentDivision string
	Loadout            FishingLoadoutSnapshot
	Species            []catalog.FishingSpecies
	Knowledge          map[string]*fishingBatchKnowledgeState
	Records            map[string]*fishingBatchRecordState
	Pending            []*fishingPendingCatch
	Loaded             bool
}

func newFishingExecutor(store *Store) *fishingExecutor {
	return &fishingExecutor{store: store}
}

func (e *fishingExecutor) ActionID() string {
	return "fishing_trip"
}

func (e *fishingExecutor) OfflineProjectionPolicy() QueueOfflineProjectionPolicy {
	return QueueOfflineProjectionImmediate
}

func (e *fishingExecutor) ValidateAndNormalizeArgs(raw json.RawMessage) (any, error) {
	if len(raw) == 0 {
		return nil, fishingArgsError("PAYLOAD_INVALID", "")
	}
	var args fishingExecutorArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, fishingArgsError("PAYLOAD_INVALID", "")
	}
	args.SpotID = strings.TrimSpace(args.SpotID)
	args.LoadoutID = strings.TrimSpace(args.LoadoutID)
	if args.SpotID == "" {
		return nil, fishingArgsError("SPOT_REQUIRED", "")
	}
	if args.LoadoutID == "" {
		return nil, fishingArgsError("LOADOUT_REQUIRED", "")
	}
	args.TournamentDivision = normalizeFishingTournamentDivision(args.TournamentDivision)
	if args.TournamentDivision == "" {
		return nil, fishingArgsError("DIVISION_INVALID", "")
	}
	if args.DurationSeconds != 0 && (args.DurationSeconds < fishingMinimumTripSeconds || args.DurationSeconds > fishingMaximumTripSeconds) {
		return nil, fishingArgsError("DURATION_INVALID", "")
	}
	if strings.TrimSpace(args.Strategy) != "" {
		strategy := strings.ToLower(strings.TrimSpace(args.Strategy))
		if strategy != "conservative" && strategy != "balanced" && strategy != "aggressive" {
			return nil, fishingArgsError("STRATEGY_INVALID", "")
		}
		args.Strategy = strategy
	}
	if args.SafetyLoadPercent != 0 && (args.SafetyLoadPercent < 50 || args.SafetyLoadPercent > 100) {
		return nil, fishingArgsError("SAFETY_INVALID", "")
	}
	if strings.TrimSpace(args.TargetLayer) == "" {
		args.TargetLayer = fishingTargetLayerAuto
	} else {
		args.TargetLayer = normalizeFishingTargetLayer(args.TargetLayer)
		if args.TargetLayer == "" {
			return nil, fishingArgsError("LAYER_INVALID", "")
		}
	}
	if strings.TrimSpace(args.Pace) != "" {
		args.Pace = normalizeFishingPace(args.Pace)
		if args.Pace == "" {
			return nil, fishingArgsError("PACE_INVALID", "")
		}
	}
	args.KeepRule = normalizeFishingKeepRule(args.KeepRule)
	if err := validateFishingSupplyCounts(args.Supplies); err != nil {
		return nil, err
	}
	return args, nil
}

func (e *fishingExecutor) FreezeRunContextTx(ctx context.Context, tx pgx.Tx, userID int64, actorType string, actorID int64, _ ActionSpec, rawArgs any, now time.Time) (json.RawMessage, error) {
	args, ok := rawArgs.(fishingExecutorArgs)
	actorType = normalizeActorType(actorType)
	if !ok || (actorType != ActionActorTypePlayer && actorType != ActionActorTypeKitty) {
		return nil, fishingArgsError("PAYLOAD_INVALID", "")
	}
	if actorType == ActionActorTypePlayer {
		actorID = 0
	} else if actorID <= 0 {
		return nil, ErrActionActorInvalid
	}
	if err := e.store.lockUserTx(ctx, tx, userID); err != nil {
		return nil, err
	}
	var hasActiveTrip bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1
			FROM player_action_queue
			WHERE user_id = $1
			  AND action_id = $2
			  AND status IN ('pending', 'running')
		)
	`, userID, FishingTripActionID).Scan(&hasActiveTrip); err != nil {
		return nil, err
	}
	if hasActiveTrip {
		return nil, ErrFishingTripActive
	}
	spot := e.store.gameData.FishingSpotByID(args.SpotID)
	if spot == nil {
		return nil, ErrFishingSpotNotFound
	}
	water := e.store.gameData.FishingWaterByID(spot.WaterID)
	if water == nil {
		return nil, ErrFishingSpotNotFound
	}
	loadout, err := e.store.getFishingLoadoutTx(ctx, tx, userID, args.LoadoutID)
	if err != nil {
		return nil, err
	}
	snapshot, err := e.store.previewFishingLoadoutForActorTx(ctx, tx, userID, actorType, actorID, loadout)
	if err != nil {
		return nil, err
	}
	if err := validateFishingSuppliesForLoadout(args.Supplies, snapshot.Method); err != nil {
		return nil, err
	}
	if err := e.validateFishingSupplyAvailabilityTx(ctx, tx, userID, snapshot, args.Supplies); err != nil {
		return nil, err
	}
	if snapshot.FishingLevel < water.RequiredFishingLevel {
		return nil, ErrFishingWaterLocked
	}
	if !containsFishingMethod(spot.Methods, snapshot.Method) {
		return nil, fishingLoadoutError("SPOT_METHOD_MISMATCH", "")
	}
	if snapshot.Method == "bottom" && args.TargetLayer != fishingTargetLayerAuto && args.TargetLayer != fishingTargetLayerNearBottom {
		return nil, fishingLoadoutError("BOTTOM_LAYER_MISMATCH", "")
	}
	if args.TournamentDivision == fishingTournamentDivisionStandard {
		tournament, err := e.store.ensureFishingTournamentTx(ctx, tx, now)
		if err != nil {
			return nil, err
		}
		if err := e.store.validateFishingTournamentSelection(tournament, args.TournamentDivision, snapshot); err != nil {
			return nil, err
		}
	}
	strategy := snapshot.Strategy
	if args.Strategy != "" {
		strategy = args.Strategy
	}
	safety := snapshot.SafetyLoadPercent
	if safety <= 0 {
		safety = fishingDefaultSafetyLoad
	}
	if args.SafetyLoadPercent > 0 {
		safety = args.SafetyLoadPercent
	} else if args.Strategy != "" && args.Pace != "" {
		safety = fishingSafetyLoadForStrategy(strategy)
	}
	pressure, err := e.store.fishingPressureTx(ctx, tx, userID, spot.ID, now)
	if err != nil {
		return nil, err
	}
	runtime := fishingExecutorRuntime{
		DurationMS: 1_000, TripID: newFishingID("trip"), ActorType: actorType, ActorID: actorID, WaterID: water.ID, SpotID: spot.ID,
		TournamentDivision: args.TournamentDivision, Rules: e.rulesSnapshot(*water, *spot),
		Loadout: snapshot, Strategy: strategy, SafetyLoadPercent: safety, TargetLayer: args.TargetLayer, Pace: args.Pace,
		KeepRule: args.KeepRule, Supplies: FishingTripSupplies{Initial: args.Supplies}, PlannedDurationSeconds: args.DurationSeconds,
		Phase: "pending", Seed: int64(stableFishingHash(newFishingID("seed"))) ^ now.UnixNano() ^ userID,
		Pressure: pressure, Summary: FishingTripSummary{NewSpecies: []string{}, Records: []FishingRecordFlag{}},
	}
	return marshalFishingEnvelope(FishingTripActionID, runtime)
}

func (e *fishingExecutor) StartRunTx(ctx context.Context, tx pgx.Tx, userID int64, item QueueRuntimeItem, now time.Time) (json.RawMessage, error) {
	runtime, ok := parseFishingExecutorRuntime(item.PayloadRaw)
	if !ok || runtime.Phase != "pending" {
		return nil, fishingArgsError("RUNTIME_INVALID", "")
	}
	item.ActorType = normalizeActorType(item.ActorType)
	if item.ActorType == ActionActorTypePlayer {
		item.ActorID = 0
	}
	if runtime.ActorType == "" {
		runtime.ActorType = item.ActorType
		runtime.ActorID = item.ActorID
	}
	if normalizeActorType(runtime.ActorType) != item.ActorType || runtime.ActorID != item.ActorID {
		return nil, ErrActionActorInvalid
	}
	if err := e.store.lockUserTx(ctx, tx, userID); err != nil {
		return nil, err
	}
	var hasOtherRunningTrip bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1
			FROM fishing_trips
			WHERE user_id = $1
			  AND status = 'running'
			  AND (queue_id IS NULL OR queue_id <> $2)
		)
	`, userID, item.QueueID).Scan(&hasOtherRunningTrip); err != nil {
		return nil, err
	}
	if hasOtherRunningTrip {
		return nil, ErrFishingTripActive
	}
	loadout, err := e.store.getFishingLoadoutTx(ctx, tx, userID, runtime.Loadout.LoadoutID)
	if err != nil {
		return nil, err
	}
	equipmentKittyIDs, err := e.store.fishingEquipmentKittyIDsTx(ctx, tx, runtime.ActorType, runtime.ActorID, loadout)
	if err != nil {
		return nil, err
	}
	if err := e.store.applyFishingLoadoutEquipmentForActorTx(ctx, tx, userID, runtime.ActorType, runtime.ActorID, loadout); err != nil {
		return nil, err
	}
	snapshot, err := e.store.freezeFishingLoadoutForActorTx(ctx, tx, userID, runtime.ActorType, runtime.ActorID, loadout, true)
	if err != nil {
		return nil, err
	}
	spot := e.store.gameData.FishingSpotByID(runtime.SpotID)
	water := e.store.gameData.FishingWaterByID(runtime.WaterID)
	if spot == nil || water == nil {
		return nil, ErrFishingSpotNotFound
	}
	if snapshot.FishingLevel < water.RequiredFishingLevel {
		return nil, ErrFishingWaterLocked
	}
	if !containsFishingMethod(spot.Methods, snapshot.Method) {
		return nil, fishingLoadoutError("SPOT_METHOD_MISMATCH", "")
	}
	if runtime.TournamentDivision == fishingTournamentDivisionStandard {
		tournament, err := e.store.ensureFishingTournamentTx(ctx, tx, now)
		if err != nil {
			return nil, err
		}
		if err := e.store.validateFishingTournamentSelection(tournament, runtime.TournamentDivision, snapshot); err != nil {
			return nil, err
		}
	}
	// Queueing only reserves the chosen preset. When the trip reaches the front,
	// the selected angler equips it in this transaction before values are frozen.
	runtime.Loadout = snapshot
	runtime.EquipmentKittyIDs = equipmentKittyIDs
	runtime.Rules = e.rulesSnapshot(*water, *spot)
	supplies, err := e.reserveFishingSuppliesTx(ctx, tx, userID, runtime.Loadout, runtime.Supplies.Initial)
	if err != nil {
		return nil, err
	}
	runtime.Supplies = supplies
	if now.IsZero() {
		now = time.Now()
	}
	pressure, err := e.store.fishingPressureTx(ctx, tx, userID, runtime.SpotID, now)
	if err != nil {
		return nil, err
	}
	runtime.Pressure = pressure
	runtime.StartedAtMS = now.UnixMilli()
	if runtime.PlannedDurationSeconds > 0 {
		runtime.PlannedEndAtMS = now.Add(time.Duration(runtime.PlannedDurationSeconds) * time.Second).UnixMilli()
	}
	runtime.Phase = fishingPhaseWaiting
	runtime.PhaseStartedAtMS = now.UnixMilli()
	if ok, err := e.beginFishingCastTx(ctx, tx, userID, &runtime, now); err != nil {
		return nil, err
	} else if !ok {
		return nil, ErrInsufficientItem
	}
	if item.OperationEffects != nil && len(item.OperationEffects.Rules) > 0 {
		runtime.CastEffects = item.OperationEffects
	}
	passive := item.EffectSession.Passive().Snapshot(passiveOperation(item, operationeffect.DomainFishingCast, now))
	if len(passive.Sources) > 0 {
		runtime.CastPassive = &passive
	}
	loadoutRaw, err := json.Marshal(runtime.Loadout)
	if err != nil {
		return nil, err
	}
	keepRuleRaw, err := json.Marshal(runtime.KeepRule)
	if err != nil {
		return nil, err
	}
	summaryRaw, err := json.Marshal(runtime.Summary)
	if err != nil {
		return nil, err
	}
	suppliesRaw, err := json.Marshal(runtime.Supplies)
	if err != nil {
		return nil, err
	}
	var plannedEnd *time.Time
	if runtime.PlannedEndAtMS > 0 {
		value := time.UnixMilli(runtime.PlannedEndAtMS)
		plannedEnd = &value
	}
	phaseEnd := time.UnixMilli(runtime.PhaseEndsAtMS)
	if _, err := tx.Exec(ctx, `
		INSERT INTO fishing_trips(
			id, user_id, queue_id, actor_type, actor_id, water_id, spot_id, loadout_id, loadout_snapshot_jsonb,
			strategy, safety_load_percent, keep_rule_jsonb, status, phase, phase_started_at,
				phase_ends_at, planned_end_at, current_fish_jsonb, summary_jsonb, supplies_jsonb, stop_reason,
			ruleset_version, seed, event_seq, started_at, updated_at
		)
			VALUES($1, $2, $3, $4, $5, $6, $7, $8, $9::jsonb, $10, $11, $12::jsonb, 'running', $13, $14,
			       $15, $16, '{}'::jsonb, $17::jsonb, $18::jsonb, '', $19, $20, $21, $14, now())
		`, runtime.TripID, userID, item.QueueID, runtime.ActorType, runtime.ActorID, runtime.WaterID, runtime.SpotID, runtime.Loadout.LoadoutID,
		string(loadoutRaw), runtime.Strategy, runtime.SafetyLoadPercent, string(keepRuleRaw), runtime.Phase,
		now, phaseEnd, plannedEnd, string(summaryRaw), string(suppliesRaw), runtime.Rules.RulesetVersion, runtime.Seed, runtime.EventSeq); err != nil {
		return nil, err
	}
	if err := e.store.persistFishingTripEventsTx(ctx, tx, runtime.TripID, runtime.pendingEvents); err != nil {
		return nil, err
	}
	return setFishingExecutorRuntime(item.PayloadRaw, runtime)
}

func (e *fishingExecutor) CancelRunTx(ctx context.Context, tx pgx.Tx, userID int64, item QueueRuntimeItem) error {
	_, err := e.store.stopFishingTripForQueueTx(ctx, tx, userID, item.QueueID, "manual_stop", time.Now())
	return err
}

func (e *fishingExecutor) SettleNormalTx(ctx context.Context, tx pgx.Tx, userID int64, item QueueRuntimeItem, now time.Time) (SettleResult, error) {
	return e.settleFishingTx(ctx, tx, userID, item, now, 1, FastForwardBudget{}, false)
}

func (e *fishingExecutor) SettleFastForwardTx(ctx context.Context, tx pgx.Tx, userID int64, item QueueRuntimeItem, now time.Time, budget FastForwardBudget) (SettleResult, error) {
	return e.settleFishingTx(ctx, tx, userID, item, now, fishingFastForwardPhaseLimit(budget), budget, true)
}

func fishingFastForwardPhaseLimit(budget FastForwardBudget) int {
	limit := fishingMaxFastForwardPhases
	if budget.MaxIterations > 0 && budget.MaxIterations < limit {
		limit = budget.MaxIterations
	}
	return limit
}

func shouldYieldFishingFastForwardBudget(startedAt time.Time, budget FastForwardBudget, transitions int) bool {
	if transitions <= 0 || budget.MaxExec <= 0 || startedAt.IsZero() {
		return false
	}
	return time.Since(startedAt) >= budget.MaxExec
}

func newFishingFastForwardCatchBatch(rules fishingRulesSnapshot) *fishingFastForwardCatchBatch {
	rulesetVersion := strings.TrimSpace(rules.RulesetVersion)
	if rulesetVersion == "" {
		rulesetVersion = FishingRulesetVersion
	}
	return &fishingFastForwardCatchBatch{
		RulesetVersion: rulesetVersion,
		Species:        append([]catalog.FishingSpecies(nil), rules.Species...),
		Knowledge:      make(map[string]*fishingBatchKnowledgeState, len(rules.Species)),
		Records:        make(map[string]*fishingBatchRecordState, len(rules.Species)),
		Pending:        make([]*fishingPendingCatch, 0, fishingMaxFastForwardCatches),
	}
}

const fishingFastForwardKnowledgeLoadSQL = `
	SELECT species_id, catches
	FROM fishing_species_knowledge
	WHERE user_id = $1 AND species_id = ANY($2)
	FOR UPDATE
`

const fishingFastForwardRecordsLoadSQL = `
	SELECT species_id, best_length_cm, best_length_catch_id, best_weight_kg, best_weight_catch_id
	FROM fishing_records
	WHERE user_id = $1 AND ruleset_version = $2 AND species_id = ANY($3)
	FOR UPDATE
`

func newFishingFastForwardStateLoadBatch(userID int64, rulesetVersion string, speciesIDs []string) *pgx.Batch {
	loadBatch := &pgx.Batch{}
	loadBatch.Queue(fishingFastForwardKnowledgeLoadSQL, userID, speciesIDs)
	loadBatch.Queue(fishingFastForwardRecordsLoadSQL, userID, rulesetVersion, speciesIDs)
	return loadBatch
}

func (batch *fishingFastForwardCatchBatch) ensureLoaded(ctx context.Context, tx pgx.Tx, userID int64) error {
	if batch == nil || batch.Loaded {
		return nil
	}
	speciesSet := make(map[string]struct{}, len(batch.Species))
	for _, species := range batch.Species {
		speciesID := strings.TrimSpace(species.ID)
		if speciesID == "" {
			continue
		}
		speciesSet[speciesID] = struct{}{}
		batch.Knowledge[speciesID] = &fishingBatchKnowledgeState{}
		batch.Records[speciesID] = &fishingBatchRecordState{}
	}
	speciesIDs := make([]string, 0, len(speciesSet))
	for speciesID := range speciesSet {
		speciesIDs = append(speciesIDs, speciesID)
	}
	sort.Strings(speciesIDs)
	if len(speciesIDs) == 0 {
		batch.Loaded = true
		return nil
	}

	results := tx.SendBatch(ctx, newFishingFastForwardStateLoadBatch(userID, batch.RulesetVersion, speciesIDs))
	rows, err := results.Query()
	if err != nil {
		_ = results.Close()
		return err
	}
	for rows.Next() {
		var speciesID string
		var catches int64
		if err := rows.Scan(&speciesID, &catches); err != nil {
			rows.Close()
			_ = results.Close()
			return err
		}
		state := batch.Knowledge[speciesID]
		if state == nil {
			state = &fishingBatchKnowledgeState{}
			batch.Knowledge[speciesID] = state
		}
		state.Exists = true
		state.BaseCatches = catches
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		_ = results.Close()
		return err
	}

	recordRows, err := results.Query()
	if err != nil {
		_ = results.Close()
		return err
	}
	for recordRows.Next() {
		var speciesID string
		state := &fishingBatchRecordState{}
		if err := recordRows.Scan(&speciesID, &state.BestLengthCM, &state.BestLengthCatchID, &state.BestWeightKG, &state.BestWeightCatchID); err != nil {
			recordRows.Close()
			_ = results.Close()
			return err
		}
		batch.Records[speciesID] = state
	}
	recordRows.Close()
	if err := recordRows.Err(); err != nil {
		_ = results.Close()
		return err
	}
	if err := results.Close(); err != nil {
		return err
	}
	batch.Loaded = true
	return nil
}

func (batch *fishingFastForwardCatchBatch) recordEncounter(speciesID string) {
	if batch == nil {
		return
	}
	speciesID = strings.TrimSpace(speciesID)
	state := batch.Knowledge[speciesID]
	if state == nil {
		state = &fishingBatchKnowledgeState{}
		batch.Knowledge[speciesID] = state
	}
	state.EncounterDelta++
}

func (batch *fishingFastForwardCatchBatch) prepareCatch(species catalog.FishingSpecies, fish FishInstanceData, rule FishingKeepRule, caughtAt time.Time) *fishingPendingCatch {
	if batch == nil {
		return nil
	}
	speciesID := strings.TrimSpace(fish.SpeciesID)
	knowledge := batch.Knowledge[speciesID]
	if knowledge == nil {
		knowledge = &fishingBatchKnowledgeState{}
		batch.Knowledge[speciesID] = knowledge
	}
	if !knowledge.Exists && knowledge.EncounterDelta == 0 {
		// A saved fight normally already has an encounter row. Preserve the
		// historical fallback if an older or repaired runtime reaches us without one.
		knowledge.EncounterDelta = 1
	}
	recordState := batch.Records[speciesID]
	if recordState == nil {
		recordState = &fishingBatchRecordState{}
		batch.Records[speciesID] = recordState
	}
	rulesetVersion := strings.TrimSpace(fish.RulesetVersion)
	if rulesetVersion == "" {
		rulesetVersion = batch.RulesetVersion
		fish.RulesetVersion = rulesetVersion
	}
	record := FishingRecordFlag{
		SpeciesID: speciesID,
		Length:    fish.LengthCM > recordState.BestLengthCM,
		Weight:    fish.WeightKG > recordState.BestWeightKG,
	}
	if record.Length {
		fish.RecordFlags = append(fish.RecordFlags, "length")
		recordState.BestLengthCM = fish.LengthCM
		recordState.Dirty = true
	}
	if record.Weight {
		fish.RecordFlags = append(fish.RecordFlags, "weight")
		recordState.BestWeightKG = fish.WeightKG
		recordState.Dirty = true
	}
	isNew := knowledge.BaseCatches+knowledge.CatchDelta == 0
	kept := fishingShouldKeep(rule, fish, record.Length || record.Weight)
	knowledge.CatchDelta++
	if !kept {
		knowledge.ReleaseDelta++
	}
	if knowledge.FirstCaughtAt == nil {
		firstCaughtAt := caughtAt
		knowledge.FirstCaughtAt = &firstCaughtAt
	}
	lastCaughtAt := caughtAt
	knowledge.LastCaughtAt = &lastCaughtAt
	pending := &fishingPendingCatch{
		Species:  species,
		Fish:     fish,
		Record:   record,
		IsNew:    isNew,
		Kept:     kept,
		Gain:     fishingCatchExpGain(species, isNew, record.Length || record.Weight),
		CaughtAt: caughtAt,
	}
	batch.Pending = append(batch.Pending, pending)
	return pending
}

func (batch *fishingFastForwardCatchBatch) full() bool {
	return batch != nil && len(batch.Pending) >= fishingMaxFastForwardCatches
}

func fishingCatchExpGain(species catalog.FishingSpecies, isNew, isRecord bool) float64 {
	gain := 3.0
	switch species.Rarity {
	case "uncommon":
		gain += 1.2
	case "rare":
		gain += 3
	case "epic":
		gain += 6
	case "legendary":
		gain += 10
	}
	if isNew {
		gain += 15
	}
	if isRecord {
		gain += 8
	}
	return roundFloat(gain, 3)
}

func (batch *fishingFastForwardCatchBatch) persist(ctx context.Context, tx pgx.Tx, executor *fishingExecutor, userID int64, tripID string) (*FishingCatchView, error) {
	if batch == nil || !batch.Loaded {
		return nil, nil
	}
	if err := insertFishingCatchBatchTx(ctx, tx, userID, tripID, batch.Pending); err != nil {
		return nil, err
	}
	if err := executor.store.grantFishingScaleglowBatchTx(ctx, tx, userID, tripID, batch.Pending); err != nil {
		return nil, err
	}

	var lastCatch *FishingCatchView
	for _, pending := range batch.Pending {
		recordState := batch.Records[pending.Fish.SpeciesID]
		if recordState != nil {
			if pending.Record.Length {
				recordState.BestLengthCatchID = &pending.CatchID
			}
			if pending.Record.Weight {
				recordState.BestWeightCatchID = &pending.CatchID
			}
		}
		view := FishingCatchView{
			CatchID: pending.CatchID, TripID: tripID, SpeciesID: pending.Fish.SpeciesID,
			Fish: pending.Fish, Kept: pending.Kept, InventoryInstanceID: pending.InventoryInstanceID,
			CaughtAt: pending.CaughtAt, Record: pending.Record,
		}
		lastCatch = &view
	}
	if err := batch.persistTournamentEntries(ctx, tx, executor.store, userID); err != nil {
		return nil, err
	}
	if err := batch.flushState(ctx, tx, userID); err != nil {
		return nil, err
	}
	if len(batch.Pending) > 0 {
		if err := executor.store.pruneFishingCatchHistoryTx(ctx, tx, userID); err != nil {
			return nil, err
		}
	}
	return lastCatch, nil
}

func (batch *fishingFastForwardCatchBatch) persistTournamentEntries(ctx context.Context, tx pgx.Tx, store *Store, userID int64) error {
	if batch == nil || store == nil || batch.RulesetVersion != FishingRulesetVersion || len(batch.Pending) == 0 {
		return nil
	}
	type tournamentPendingGroup struct {
		At      time.Time
		Catches []*fishingPendingCatch
	}
	groups := map[string]*tournamentPendingGroup{}
	for _, pending := range batch.Pending {
		if pending == nil {
			continue
		}
		weekStart := fishingWeekStart(pending.CaughtAt)
		weekKey := weekStart.Format("2006-01-02")
		group := groups[weekKey]
		if group == nil {
			group = &tournamentPendingGroup{At: pending.CaughtAt}
			groups[weekKey] = group
		}
		group.Catches = append(group.Catches, pending)
	}
	weekKeys := make([]string, 0, len(groups))
	for weekKey := range groups {
		weekKeys = append(weekKeys, weekKey)
	}
	sort.Strings(weekKeys)
	for _, weekKey := range weekKeys {
		group := groups[weekKey]
		tournament, err := store.ensureFishingTournamentTx(ctx, tx, group.At)
		if err != nil {
			return err
		}
		eligible := fishingTournamentEligiblePendingCatches(tournament, group.Catches)
		if len(eligible) == 0 {
			continue
		}
		if fishingTournamentUsesStandardLoadout(tournament) {
			division := normalizeFishingTournamentDivision(batch.TournamentDivision)
			if division == fishingTournamentDivisionStandard {
				standard, loadoutErr := store.validatedFishingTournamentStandardLoadout()
				if loadoutErr != nil {
					return loadoutErr
				}
				if !fishingLoadoutMatchesTournamentStandard(batch.Loadout, standard) {
					continue
				}
			}
			if division != fishingTournamentDivisionOpen && division != fishingTournamentDivisionStandard {
				continue
			}
			var best *fishingPendingCatch
			for _, pending := range eligible {
				if betterFishingTournamentCatch(pending, best) {
					best = pending
				}
			}
			if best != nil {
				if err := store.upsertFishingTournamentCatchEntryTx(ctx, tx, userID, tournament.ID, division, best.CatchID); err != nil {
					return err
				}
			}
			continue
		}
		var openBest, standardBest *fishingPendingCatch
		for _, pending := range eligible {
			if betterFishingTournamentCatch(pending, openBest) {
				openBest = pending
			}
			if fishingStandardTournamentQualifies(tournament.ID, pending.CatchID, pending.Species, pending.Fish) && betterFishingTournamentCatch(pending, standardBest) {
				standardBest = pending
			}
		}
		for _, entry := range []struct {
			Division string
			Catch    *fishingPendingCatch
		}{{Division: fishingTournamentDivisionOpen, Catch: openBest}, {Division: fishingTournamentDivisionStandard, Catch: standardBest}} {
			if entry.Catch == nil {
				continue
			}
			if err := store.upsertFishingTournamentCatchEntryTx(ctx, tx, userID, tournament.ID, entry.Division, entry.Catch.CatchID); err != nil {
				return err
			}
		}
	}
	return nil
}

func betterFishingTournamentCatch(candidate, current *fishingPendingCatch) bool {
	if candidate == nil {
		return false
	}
	if current == nil {
		return true
	}
	return candidate.Fish.WeightKG > current.Fish.WeightKG ||
		(candidate.Fish.WeightKG == current.Fish.WeightKG && candidate.Fish.LengthCM > current.Fish.LengthCM)
}

func fishingTournamentEligiblePendingCatches(tournament FishingTournament, pending []*fishingPendingCatch) []*fishingPendingCatch {
	eligible := make([]*fishingPendingCatch, 0, len(pending))
	for _, candidate := range pending {
		if candidate != nil && fishingTournamentCatchEligible(tournament, candidate.Fish.SpeciesID, candidate.CaughtAt) {
			eligible = append(eligible, candidate)
		}
	}
	return eligible
}

func (e *fishingExecutor) settleFishingTx(ctx context.Context, tx pgx.Tx, userID int64, item QueueRuntimeItem, now time.Time, maxPhases int, budget FastForwardBudget, batchCatches bool) (SettleResult, error) {
	runtime, ok := parseFishingExecutorRuntime(item.PayloadRaw)
	runtime.effectSession = item.EffectSession
	if !ok || !fishingRuntimeCanSettle(runtime, item) {
		// CancelAfterSettle authorizes queue deletion, so close the persisted trip
		// and return supplies in this transaction before granting that permission.
		tripID, err := e.store.stopFishingTripForQueueTx(ctx, tx, userID, item.QueueID, fishingTripRuntimeInvalidStopReason, time.Now())
		if err != nil {
			return SettleResult{}, err
		}
		log.Printf("fishing runtime rejected user_id=%d queue_id=%d trip_id=%s parsed=%t payload_sha256=%x cleanup=pending_commit", userID, item.QueueID, tripID, ok, sha256.Sum256(item.PayloadRaw))
		return SettleResult{CancelAfterSettle: true, CancelReason: ActionQueueCancelReasonTargetInvalid}, nil
	}
	if maxPhases <= 0 {
		maxPhases = 1
	}
	if now.IsZero() {
		now = time.Now()
	}
	if err := e.ensureRulesSnapshot(&runtime); err != nil {
		return SettleResult{}, err
	}
	lastEventAt := time.Time{}
	attrExp := 0.0
	ownerExp := map[string]float64{}
	itemCosts := map[string]int64{}
	itemRewards := map[string]int64{}
	fishingCatches := map[string]FishingCatchTally{}
	var catchBatch *fishingFastForwardCatchBatch
	if batchCatches {
		runtime.batchRNG = rand.New(rand.NewSource(1))
		catchBatch = newFishingFastForwardCatchBatch(runtime.Rules)
		catchBatch.TournamentDivision = runtime.TournamentDivision
		catchBatch.Loadout = runtime.Loadout
	}
	completed := false
	budgetStartedAt := budget.StartedAt
	if budgetStartedAt.IsZero() {
		budgetStartedAt = time.Now()
	}
	transitions := 0
	for transitions < maxPhases {
		if shouldYieldFishingFastForwardBudget(budgetStartedAt, budget, transitions) {
			break
		}
		phaseEnd := time.UnixMilli(runtime.PhaseEndsAtMS)
		if phaseEnd.After(now) {
			break
		}
		transitions++
		lastEventAt = phaseEnd
		beforeExp := attrExp
		beforePassive := runtime.CastPassive
		switch runtime.Phase {
		case fishingPhaseWaiting:
			if runtime.PlannedEndAtMS > 0 && phaseEnd.UnixMilli() >= runtime.PlannedEndAtMS {
				attrExp += 0.18
				runtime.Summary.StopReason = "time_reached"
				completed = true
				break
			}
			spot := runtime.Rules.Spot
			water := runtime.Rules.Water
			pressureElapsed := phaseEnd.Sub(time.UnixMilli(runtime.PhaseStartedAtMS))
			if pressureElapsed > 0 {
				runtime.Pressure = decayFishingPressure(runtime.Pressure, pressureElapsed)
			}
			conditions := fishingConditionsAt(water, spot, phaseEnd)
			rng := fishingRuntimeRNG(&runtime)
			targetLayer := effectiveFishingTargetLayer(runtime.TargetLayer)
			layerBiteMultiplier := fishingLayerBiteMultiplier(runtime.Rules.Species, spot, runtime.Loadout, conditions, targetLayer)
			biteChance := fishingBiteChance(spot, runtime.Loadout, effectiveFishingPace(runtime.Pace, runtime.Strategy), runtime.Pressure, conditions, layerBiteMultiplier)
			if runtime.CastEffects != nil {
				bonus, err := ratebonus.Resolve(*runtime.CastEffects, ratebonus.FishingBite)
				if err != nil {
					return SettleResult{}, err
				}
				biteChance = clampFloat(biteChance+bonus, 0, 0.97)
			}
			runtime.Pressure = clampFloat(runtime.Pressure+0.006, 0, 1)
			if rng.Float64() >= biteChance {
				attrExp += 0.18
				appendFishingTripEvent(&runtime, FishingTripEvent{
					Kind: "quiet", At: phaseEnd, CastNumber: runtime.Summary.Casts,
				})
				if ok, err := e.beginFishingCastTx(ctx, tx, userID, &runtime, phaseEnd); err != nil {
					return SettleResult{}, err
				} else if !ok {
					runtime.Summary.StopReason = "bait_empty"
					completed = true
				}
				break
			}
			species := selectFishingSpecies(runtime.Rules.Species, spot, runtime.Loadout, conditions, targetLayer, rng)
			if species == nil {
				attrExp += 0.18
				appendFishingTripEvent(&runtime, FishingTripEvent{
					Kind: "quiet", At: phaseEnd, CastNumber: runtime.Summary.Casts,
				})
				if ok, err := e.beginFishingCastTx(ctx, tx, userID, &runtime, phaseEnd); err != nil {
					return SettleResult{}, err
				} else if !ok {
					runtime.Summary.StopReason = "bait_empty"
					completed = true
				}
				break
			}
			fish := generateFishingIndividual(*species, runtime.WaterID, runtime.SpotID, runtime.Rules.RulesetVersion, phaseEnd, runtime.Pressure, rng)
			mutationSeed := fmt.Sprintf("%d:%d:%d:%d:%s", runtime.Seed, runtime.Summary.Casts, runtime.EventSeq, phaseEnd.UnixMilli(), fish.SpeciesID)
			fish.Genes.Mutations = rollFishingMutations(runtime.Rules.Mutations, mutationSeed, runtime.Loadout.MutationChanceMultiplier)
			runtime.CurrentFish = &fish
			runtime.CurrentSpeciesRarity = species.Rarity
			runtime.Summary.Bites++
			runtime.Pressure = clampFloat(runtime.Pressure+0.012, 0, 1)
			if catchBatch != nil {
				if err := catchBatch.ensureLoaded(ctx, tx, userID); err != nil {
					return SettleResult{}, err
				}
				catchBatch.recordEncounter(fish.SpeciesID)
			} else if err := e.recordFishingEncounterTx(ctx, tx, userID, fish.SpeciesID); err != nil {
				return SettleResult{}, err
			}
			runtime.Phase = fishingPhaseBite
			runtime.PhaseStartedAtMS = phaseEnd.UnixMilli()
			runtime.DurationMS = 3_000
			runtime.PhaseEndsAtMS = phaseEnd.Add(3 * time.Second).UnixMilli()
			appendFishingTripEvent(&runtime, FishingTripEvent{
				Kind: "bite", At: phaseEnd, CastNumber: runtime.Summary.Casts,
				SizeClass: fishingFishSizeClass(runtime.Rules, fish),
			})

		case fishingPhaseBite:
			if runtime.CurrentFish == nil {
				return SettleResult{}, ErrFishingArgsInvalid
			}
			species := runtime.Rules.speciesByID(runtime.CurrentFish.SpeciesID)
			if species == nil {
				return SettleResult{}, ErrFishingSpotNotFound
			}
			duration := fishingFightDuration(*species, *runtime.CurrentFish, runtime.Loadout, runtime.Strategy)
			runtime.Phase = fishingPhaseFighting
			runtime.PhaseStartedAtMS = phaseEnd.UnixMilli()
			runtime.DurationMS = duration.Milliseconds()
			runtime.PhaseEndsAtMS = phaseEnd.Add(duration).UnixMilli()
			runtime.CurrentLineLoad = roundFloat(fishingLineLoadForRuleset(*species, *runtime.CurrentFish, runtime.Strategy, runtime.Rules.RulesetVersion), 3)
			appendFishingTripEvent(&runtime, FishingTripEvent{
				Kind: "hooked", At: phaseEnd, CastNumber: runtime.Summary.Casts,
				SizeClass: fishingFishSizeClass(runtime.Rules, *runtime.CurrentFish),
			})

		case fishingPhaseFighting:
			if runtime.CurrentFish == nil {
				return SettleResult{}, ErrFishingArgsInvalid
			}
			species := runtime.Rules.speciesByID(runtime.CurrentFish.SpeciesID)
			if species == nil {
				return SettleResult{}, ErrFishingSpotNotFound
			}
			rng := fishingRuntimeRNG(&runtime)
			var modifiers fishingeffect.Modifiers
			if runtime.CastEffects != nil {
				var err error
				modifiers, err = fishingeffect.Resolve(*runtime.CastEffects)
				if err != nil {
					return SettleResult{}, err
				}
			}
			fight := resolveFishingFightWithEffects(*species, *runtime.CurrentFish, runtime.Loadout, runtime.Strategy, runtime.SafetyLoadPercent, runtime.Rules.RulesetVersion, modifiers, rng)
			runtime.CurrentLineLoad = fight.LineLoad
			if fight.LineBreak {
				runtime.Summary.LineBreaks++
				runtime.Summary.Escaped++
				appendFishingTripEvent(&runtime, FishingTripEvent{
					Kind: "line_break", Reason: fight.FailureReason, At: phaseEnd,
					CastNumber: runtime.Summary.Casts, SizeClass: fishingFishSizeClass(runtime.Rules, *runtime.CurrentFish),
				})
				for _, itemID := range fishingLineBreakCostIDs(runtime.Loadout) {
					itemCosts[itemID]++
				}
				consumeFishingTerminalSupplies(&runtime.Supplies, runtime.Loadout, true)
			} else {
				for itemID, quantity := range applyFishingTackleWear(&runtime.Supplies, runtime.Loadout, *species) {
					itemCosts[itemID] += quantity
				}
				if fight.Landed {
					var catchView FishingCatchView
					var record FishingRecordFlag
					var isNew, kept bool
					var gain float64
					var pendingCatch *fishingPendingCatch
					if catchBatch != nil {
						if err := catchBatch.ensureLoaded(ctx, tx, userID); err != nil {
							return SettleResult{}, err
						}
						pendingCatch = catchBatch.prepareCatch(*species, *runtime.CurrentFish, runtime.KeepRule, phaseEnd)
						if pendingCatch == nil {
							return SettleResult{}, ErrFishingArgsInvalid
						}
						record, isNew, kept, gain = pendingCatch.Record, pendingCatch.IsNew, pendingCatch.Kept, pendingCatch.Gain
					} else {
						var err error
						catchView, record, isNew, kept, gain, err = e.landFishingCatchTx(ctx, tx, userID, runtime.TripID, runtime.TournamentDivision, runtime.Loadout, *species, *runtime.CurrentFish, runtime.KeepRule, phaseEnd)
						if err != nil {
							return SettleResult{}, err
						}
					}
					runtime.Summary.Landed++
					if kept {
						runtime.Summary.Kept++
					} else {
						runtime.Summary.Released++
					}
					disposition := "released"
					if kept {
						disposition = "kept"
					}
					runtime.Summary.TotalWeight = roundFloat(runtime.Summary.TotalWeight+runtime.CurrentFish.WeightKG, 3)
					if catchBatch == nil {
						runtime.Summary.LastCatch = &catchView
					}
					addFishingCatchTally(fishingCatches, runtime.CurrentFish.SpeciesID, kept, runtime.CurrentFish.WeightKG, runtime.CurrentFish.Genes.Mutations)
					appendFishingTripEvent(&runtime, FishingTripEvent{
						Kind: "landed", At: phaseEnd, CastNumber: runtime.Summary.Casts,
						SizeClass: fishingFishSizeClass(runtime.Rules, *runtime.CurrentFish), SpeciesID: runtime.CurrentFish.SpeciesID,
						LengthCM: runtime.CurrentFish.LengthCM, WeightKG: runtime.CurrentFish.WeightKG,
						Mutations: append([]string(nil), runtime.CurrentFish.Genes.Mutations...), Disposition: disposition,
					})
					if isNew {
						runtime.Summary.NewSpecies = appendUniqueFishingString(runtime.Summary.NewSpecies, runtime.CurrentFish.SpeciesID)
					}
					if record.Length || record.Weight {
						runtime.Summary.Records = append(runtime.Summary.Records, record)
					}
					attrExp += gain
					if fishingScaleglowDrop(runtime.Seed, runtime.Summary.Landed) {
						if pendingCatch != nil {
							pendingCatch.Scaleglow = true
						} else {
							if err := e.store.LedgerGrantTemplateTx(ctx, tx, userID, "scaleglow_medium", 1, InventoryLedgerMeta{
								SourceSurface: "fishing", TargetSurface: "inventory",
								Details: map[string]any{"type": "fishing_scaleglow", "tripId": runtime.TripID, "catchId": catchView.CatchID},
							}); err != nil {
								return SettleResult{}, err
							}
						}
						runtime.Summary.ScaleglowFound++
						itemRewards["scaleglow_medium"]++
					}
				} else {
					runtime.Summary.Escaped++
					attrExp += 0.7
					appendFishingTripEvent(&runtime, FishingTripEvent{
						Kind: "escaped", Reason: fight.FailureReason, At: phaseEnd,
						CastNumber: runtime.Summary.Casts, SizeClass: fishingFishSizeClass(runtime.Rules, *runtime.CurrentFish),
					})
				}
			}
			runtime.CurrentFish = nil
			runtime.CurrentSpeciesRarity = ""
			runtime.Phase = fishingPhaseResting
			if fight.Landed {
				runtime.Phase = fishingPhaseLanded
			}
			runtime.PhaseStartedAtMS = phaseEnd.UnixMilli()
			runtime.DurationMS = 4_000
			runtime.PhaseEndsAtMS = phaseEnd.Add(4 * time.Second).UnixMilli()

		case fishingPhaseLanded, fishingPhaseResting:
			if runtime.PlannedEndAtMS > 0 && phaseEnd.UnixMilli() >= runtime.PlannedEndAtMS {
				runtime.Summary.StopReason = "time_reached"
				completed = true
				break
			}
			if ok, err := e.beginFishingCastTx(ctx, tx, userID, &runtime, phaseEnd); err != nil {
				return SettleResult{}, err
			} else if !ok {
				runtime.Summary.StopReason = "tackle_empty"
				completed = true
			}

		case fishingPhaseFinished:
			completed = true
		default:
			return SettleResult{}, ErrFishingArgsInvalid
		}
		if delta := attrExp - beforeExp; delta > 0 && beforePassive != nil && runtime.ActorType == ActionActorTypeKitty {
			gain, err := runtime.effectSession.Passive().Share(*beforePassive, map[string]float64{"fishing": roundFloat(delta, 4)})
			if err != nil {
				return SettleResult{}, err
			}
			ownerExp = mergeAttrExp(ownerExp, gain)
		}
		if completed || catchBatch.full() {
			break
		}
	}
	if catchBatch != nil {
		lastCatch, err := catchBatch.persist(ctx, tx, e, userID, runtime.TripID)
		if err != nil {
			return SettleResult{}, err
		}
		if lastCatch != nil {
			runtime.Summary.LastCatch = lastCatch
		}
	}
	if baitUsed := runtime.Summary.BaitUsed - runtime.ReportedBaitUsed; baitUsed > 0 {
		if baitID := strings.TrimSpace(runtime.Loadout.Components.BaitItemID); baitID != "" {
			itemCosts[baitID] += int64(baitUsed)
		}
	}
	runtime.ReportedBaitUsed = runtime.Summary.BaitUsed
	if len(ownerExp) > 0 {
		if err := e.store.applyAttributedExperienceTx(ctx, tx, userID, ownerExp, ownerExp, runtime.effectSession); err != nil {
			return SettleResult{}, err
		}
	}
	if attrExp > 0 {
		attrExp = roundFloat(attrExp, 4)
		if err := e.applyFishingAttrExpTx(ctx, tx, userID, runtime.ActorType, runtime.ActorID, attrExp); err != nil {
			return SettleResult{}, err
		}
		runtime.Summary.FishingExp = roundFloat(runtime.Summary.FishingExp+attrExp, 4)
	}
	if !lastEventAt.IsZero() {
		if err := e.store.saveFishingPressureTx(ctx, tx, userID, runtime.SpotID, runtime.Pressure, lastEventAt); err != nil {
			return SettleResult{}, err
		}
	}
	if completed {
		runtime.Phase = fishingPhaseFinished
		if lastEventAt.IsZero() {
			lastEventAt = now
		}
		appendFishingTripEvent(&runtime, FishingTripEvent{
			Kind: "finished", Reason: runtime.Summary.StopReason, At: lastEventAt,
		})
		runtime.PhaseStartedAtMS = lastEventAt.UnixMilli()
		runtime.PhaseEndsAtMS = lastEventAt.UnixMilli()
		runtime.DurationMS = 1_000
		if err := e.returnFishingSuppliesTx(ctx, tx, userID, runtime.Loadout, &runtime.Supplies); err != nil {
			return SettleResult{}, err
		}
		if err := e.saveFishingTripRuntimeTx(ctx, tx, userID, item.QueueID, runtime, true, lastEventAt); err != nil {
			return SettleResult{}, err
		}
		nextPayload, err := setFishingExecutorRuntime(item.PayloadRaw, runtime)
		if err != nil {
			return SettleResult{}, err
		}
		return SettleResult{
			Completed: 1, BudgetIterations: transitions, DoneAfter: item.TotalCount, CompletedAt: lastEventAt,
			NextRunStartAt: lastEventAt, NextRunEndsAt: lastEventAt.Add(time.Second),
			NextPayload: nextPayload, ItemCosts: itemCosts, ItemRewards: itemRewards,
			AttrExp: map[string]float64{"fishing": attrExp}, OwnerAttrExp: ownerExp, FishingCatches: fishingCatches,
		}, nil
	}
	if lastEventAt.IsZero() {
		return SettleResult{}, nil
	}
	if err := e.saveFishingTripRuntimeTx(ctx, tx, userID, item.QueueID, runtime, false, lastEventAt); err != nil {
		return SettleResult{}, err
	}
	nextPayload, err := setFishingExecutorRuntime(item.PayloadRaw, runtime)
	if err != nil {
		return SettleResult{}, err
	}
	return SettleResult{
		Progressed: true, BudgetIterations: transitions, CompletedAt: lastEventAt,
		NextRunStartAt: time.UnixMilli(runtime.PhaseStartedAtMS), NextRunEndsAt: time.UnixMilli(runtime.PhaseEndsAtMS),
		NextPayload: nextPayload, ItemCosts: itemCosts, ItemRewards: itemRewards,
		AttrExp: map[string]float64{"fishing": attrExp}, OwnerAttrExp: ownerExp, FishingCatches: fishingCatches,
	}, nil
}

func (e *fishingExecutor) ClassifyStartError(err error) (bool, string) {
	switch {
	case errors.Is(err, ErrInsufficientItem), errors.Is(err, ErrFishingTackleMissing):
		return true, ActionQueueCancelReasonCostInsufficient
	case errors.Is(err, ErrFishingWaterLocked):
		return true, ActionQueueCancelReasonStatusRequirement
	case errors.Is(err, ErrStatusRequireNotMet):
		return true, ActionQueueCancelReasonStatusRequirement
	case errors.Is(err, ErrFishingArgsInvalid),
		errors.Is(err, ErrFishingSpotNotFound),
		errors.Is(err, ErrFishingLoadoutNotFound),
		errors.Is(err, ErrFishingLoadoutInvalid),
		errors.Is(err, ErrFishingTripActive),
		errors.Is(err, ErrFishingTackleInUse),
		errors.Is(err, ErrInstanceNotOwned),
		errors.Is(err, ErrInvalidInstanceState),
		errors.Is(err, ErrEquipTargetNotAllowed),
		errors.Is(err, ErrEquipSlotNotAllowed),
		errors.Is(err, ErrActionActorInvalid),
		errors.Is(err, ErrKittyNotFound):
		return true, ActionQueueCancelReasonTargetInvalid
	default:
		return false, ""
	}
}

func (e *fishingExecutor) rulesSnapshot(water catalog.FishingWater, spot catalog.FishingSpot) fishingRulesSnapshot {
	species := make([]catalog.FishingSpecies, 0, 8)
	for _, row := range e.store.gameData.FishingSpecies {
		if row.WaterID == water.ID {
			species = append(species, row)
		}
	}
	return fishingRulesSnapshot{
		RulesetVersion: FishingRulesetVersion,
		Water:          water,
		Spot:           spot,
		Species:        species,
		Mutations:      e.store.FishingMutationCatalog(),
	}
}

func (e *fishingExecutor) ensureRulesSnapshot(runtime *fishingExecutorRuntime) error {
	if runtime == nil {
		return ErrFishingArgsInvalid
	}
	if runtime.Rules.Water.ID != "" && runtime.Rules.Spot.ID != "" && len(runtime.Rules.Species) > 0 {
		if strings.TrimSpace(runtime.Rules.RulesetVersion) == "" {
			runtime.Rules.RulesetVersion = FishingRulesetVersion
		}
		return nil
	}
	spot := e.store.gameData.FishingSpotByID(runtime.SpotID)
	if spot == nil {
		return ErrFishingSpotNotFound
	}
	water := e.store.gameData.FishingWaterByID(runtime.WaterID)
	if water == nil || spot.WaterID != water.ID {
		return ErrFishingSpotNotFound
	}
	runtime.Rules = e.rulesSnapshot(*water, *spot)
	return nil
}

func (rules fishingRulesSnapshot) speciesByID(speciesID string) *catalog.FishingSpecies {
	for index := range rules.Species {
		if rules.Species[index].ID == speciesID {
			return &rules.Species[index]
		}
	}
	return nil
}

func (e *fishingExecutor) beginFishingCastTx(ctx context.Context, tx pgx.Tx, userID int64, runtime *fishingExecutorRuntime, at time.Time) (bool, error) {
	if runtime == nil {
		return false, ErrFishingArgsInvalid
	}
	if runtime.PlannedEndAtMS > 0 && at.UnixMilli() >= runtime.PlannedEndAtMS {
		return false, nil
	}
	if !hasFishingCastSupplies(runtime.Supplies.Remaining, runtime.Loadout.Method) {
		return false, nil
	}
	if runtime.Loadout.Method != "lure" {
		runtime.Supplies.Remaining.Bait--
		runtime.Supplies.Consumed.Bait++
		runtime.Summary.BaitUsed++
	}
	runtime.Summary.Casts++
	runtime.Phase = fishingPhaseWaiting
	runtime.PhaseStartedAtMS = at.UnixMilli()
	appendFishingTripEvent(runtime, FishingTripEvent{
		Kind: "cast", At: at, CastNumber: runtime.Summary.Casts,
	})
	if err := e.ensureRulesSnapshot(runtime); err != nil {
		return false, err
	}
	spot := runtime.Rules.Spot
	water := runtime.Rules.Water
	conditions := fishingConditionsAt(water, spot, at)
	rng := fishingRuntimeRNG(runtime)
	duration := fishingWaitDuration(spot, runtime.Loadout, effectiveFishingPace(runtime.Pace, runtime.Strategy), runtime.Pressure, conditions, rng)
	endAt := at.Add(duration)
	if runtime.PlannedEndAtMS > 0 && endAt.UnixMilli() > runtime.PlannedEndAtMS {
		endAt = time.UnixMilli(runtime.PlannedEndAtMS)
	}
	runtime.DurationMS = maxInt64(1_000, endAt.Sub(at).Milliseconds())
	runtime.PhaseEndsAtMS = endAt.UnixMilli()
	if err := freezeFishingCastEffects(runtime, at); err != nil {
		return false, err
	}
	return true, nil
}

func (e *fishingExecutor) validateFrozenEquipmentTx(ctx context.Context, tx pgx.Tx, userID int64, loadout FishingLoadoutSnapshot) error {
	for _, component := range loadout.FrozenComponents {
		if component.InstanceID > 0 {
			var exists int
			err := tx.QueryRow(ctx, `
				SELECT 1 FROM player_item_instances
				WHERE user_id = $1 AND instance_id = $2 AND template_item_id = $3 AND state IN ('inventory', 'equipped')
			`, userID, component.InstanceID, component.ItemID).Scan(&exists)
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrFishingTackleMissing
			}
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (e *fishingExecutor) applyFishingAttrExpTx(ctx context.Context, tx pgx.Tx, userID int64, actorType string, actorID int64, exp float64) error {
	if exp <= 0 {
		return nil
	}
	if normalizeActorType(actorType) == ActionActorTypeKitty {
		seeds := e.store.attributeSeedsFromGameData()
		return e.store.applyKittyAttrExpLockedTx(ctx, tx, userID, actorID, map[string]float64{"fishing": exp}, seeds)
	}
	return e.store.applyAttrExpTx(ctx, tx, userID, map[string]float64{"fishing": exp})
}

func fishingLineBreakCostIDs(loadout FishingLoadoutSnapshot) []string {
	terminalIDs := []string{strings.TrimSpace(loadout.Components.MainLineItemID), strings.TrimSpace(loadout.Components.LeaderItemID)}
	if loadout.Method == "lure" {
		terminalIDs = append(terminalIDs, strings.TrimSpace(loadout.Components.LureItemID))
	} else {
		terminalIDs = append(
			terminalIDs,
			strings.TrimSpace(loadout.Components.HookItemID),
			strings.TrimSpace(loadout.Components.FloatOrSinkerItemID),
		)
	}
	out := terminalIDs[:0]
	for _, itemID := range terminalIDs {
		if itemID != "" {
			out = append(out, itemID)
		}
	}
	return out
}

type fishingSupplyCountEntry struct {
	slot  string
	count int64
}

func fishingSupplyCountEntries(counts FishingSupplyCounts) []fishingSupplyCountEntry {
	return []fishingSupplyCountEntry{
		{"main_line", counts.MainLine}, {"leader", counts.Leader},
		{"hook", counts.Hook}, {"float_or_sinker", counts.FloatOrSinker},
		{"lure", counts.Lure}, {"bait", counts.Bait},
	}
}

func validateFishingSupplyCounts(counts FishingSupplyCounts) error {
	for _, entry := range fishingSupplyCountEntries(counts) {
		if entry.count < 0 || entry.count > 100_000 {
			return fishingArgsError("SUPPLY_RANGE", entry.slot)
		}
	}
	if counts.MainLine <= 0 {
		return fishingArgsError("SUPPLY_REQUIRED", "main_line")
	}
	if counts.Leader <= 0 {
		return fishingArgsError("SUPPLY_REQUIRED", "leader")
	}
	return nil
}

func validateFishingSuppliesForLoadout(counts FishingSupplyCounts, method string) error {
	if err := validateFishingSupplyCounts(counts); err != nil {
		return err
	}
	for _, entry := range fishingSupplyCountEntries(counts)[2:] {
		required := entry.slot != "lure"
		if method == "lure" {
			required = entry.slot == "lure"
		}
		if required && entry.count <= 0 {
			return fishingArgsError("SUPPLY_REQUIRED", entry.slot)
		}
		if !required && entry.count != 0 {
			return fishingArgsError("SUPPLY_UNEXPECTED", entry.slot)
		}
	}
	return nil
}

func hasFishingCastSupplies(counts FishingSupplyCounts, method string) bool {
	if counts.MainLine <= 0 || counts.Leader <= 0 {
		return false
	}
	if method == "lure" {
		return counts.Lure > 0
	}
	return counts.Hook > 0 && counts.FloatOrSinker > 0 && counts.Bait > 0
}

func fishingSupplyItems(loadout FishingLoadoutSnapshot, counts FishingSupplyCounts) map[string]int64 {
	out := map[string]int64{}
	add := func(itemID string, quantity int64) {
		itemID = strings.TrimSpace(itemID)
		if itemID != "" && quantity > 0 {
			out[itemID] += quantity
		}
	}
	add(loadout.Components.MainLineItemID, counts.MainLine)
	add(loadout.Components.LeaderItemID, counts.Leader)
	add(loadout.Components.HookItemID, counts.Hook)
	add(loadout.Components.FloatOrSinkerItemID, counts.FloatOrSinker)
	add(loadout.Components.LureItemID, counts.Lure)
	add(loadout.Components.BaitItemID, counts.Bait)
	return out
}

func sortedFishingSupplyEntries(loadout FishingLoadoutSnapshot, counts FishingSupplyCounts) []actionCostEntry {
	items := fishingSupplyItems(loadout, counts)
	entries := make([]actionCostEntry, 0, len(items))
	for itemID, quantity := range items {
		entries = append(entries, actionCostEntry{ItemID: itemID, Quantity: quantity})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].ItemID < entries[j].ItemID
	})
	return entries
}

func firstFishingSupplyShortage(entries []actionCostEntry, ownedByItem map[string]int64) *FishingSupplyShortageError {
	for _, entry := range entries {
		available := maxInt64(0, ownedByItem[entry.ItemID])
		if available < entry.Quantity {
			return &FishingSupplyShortageError{ItemID: entry.ItemID, Required: entry.Quantity, Available: available}
		}
	}
	return nil
}

func validFishingWearProgress(value float64) bool {
	return value >= 0 && value < 1 && !math.IsNaN(value) && !math.IsInf(value, 0)
}

func (e *fishingExecutor) loadFishingSupplyWearTx(ctx context.Context, tx pgx.Tx, userID int64, loadout FishingLoadoutSnapshot, supplies *FishingTripSupplies) error {
	if supplies == nil {
		return ErrFishingArgsInvalid
	}
	if err := e.store.lockUserTx(ctx, tx, userID); err != nil {
		return fmt.Errorf("lock fishing tackle wear owner: %w", err)
	}
	mainLineItemID := strings.TrimSpace(loadout.Components.MainLineItemID)
	leaderItemID := strings.TrimSpace(loadout.Components.LeaderItemID)
	if mainLineItemID == "" || leaderItemID == "" {
		return ErrFishingLoadoutInvalid
	}
	itemIDs := []string{mainLineItemID, leaderItemID}
	sort.Strings(itemIDs)
	rows, err := tx.Query(ctx, `
		SELECT item_id, wear
		FROM fishing_tackle_wear
		WHERE user_id = $1 AND item_id = ANY($2::text[])
		FOR UPDATE
	`, userID, itemIDs)
	if err != nil {
		return fmt.Errorf("load fishing tackle wear: %w", err)
	}
	defer rows.Close()
	wearByItem := make(map[string]float64, len(itemIDs))
	for rows.Next() {
		var itemID string
		var wear float64
		if err := rows.Scan(&itemID, &wear); err != nil {
			return fmt.Errorf("scan fishing tackle wear: %w", err)
		}
		if !validFishingWearProgress(wear) {
			return fmt.Errorf("invalid fishing tackle wear for %s: %v", itemID, wear)
		}
		wearByItem[itemID] = wear
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate fishing tackle wear: %w", err)
	}
	supplies.Wear.MainLine = wearByItem[mainLineItemID]
	supplies.Wear.Leader = wearByItem[leaderItemID]
	return nil
}

func (e *fishingExecutor) saveFishingSupplyWearTx(ctx context.Context, tx pgx.Tx, userID int64, loadout FishingLoadoutSnapshot, supplies FishingTripSupplies) error {
	if err := e.store.lockUserTx(ctx, tx, userID); err != nil {
		return fmt.Errorf("lock fishing tackle wear owner: %w", err)
	}
	itemIDs := []string{
		strings.TrimSpace(loadout.Components.MainLineItemID),
		strings.TrimSpace(loadout.Components.LeaderItemID),
	}
	wearValues := []float64{supplies.Wear.MainLine, supplies.Wear.Leader}
	for index := range itemIDs {
		if itemIDs[index] == "" {
			return ErrFishingLoadoutInvalid
		}
		if !validFishingWearProgress(wearValues[index]) {
			return fmt.Errorf("invalid fishing tackle wear for %s: %v", itemIDs[index], wearValues[index])
		}
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO fishing_tackle_wear(user_id, item_id, wear, updated_at)
		SELECT $1, wear_rows.item_id, wear_rows.wear, now()
		FROM unnest($2::text[], $3::double precision[]) AS wear_rows(item_id, wear)
		ON CONFLICT (user_id, item_id) DO UPDATE
		SET wear = EXCLUDED.wear, updated_at = now()
	`, userID, itemIDs, wearValues); err != nil {
		return fmt.Errorf("save fishing tackle wear: %w", err)
	}
	return nil
}

func (e *fishingExecutor) validateFishingSupplyAvailabilityTx(ctx context.Context, tx pgx.Tx, userID int64, loadout FishingLoadoutSnapshot, counts FishingSupplyCounts) error {
	entries := sortedFishingSupplyEntries(loadout, counts)
	ownedByItem, err := e.store.lockConsumableQuantitiesByTemplateIDsTx(ctx, tx, userID, actionCostItemIDs(entries))
	if err != nil {
		return err
	}
	if shortage := firstFishingSupplyShortage(entries, ownedByItem); shortage != nil {
		return shortage
	}
	return nil
}

func (e *fishingExecutor) reserveFishingSuppliesTx(ctx context.Context, tx pgx.Tx, userID int64, loadout FishingLoadoutSnapshot, counts FishingSupplyCounts) (FishingTripSupplies, error) {
	if err := validateFishingSuppliesForLoadout(counts, loadout.Method); err != nil {
		return FishingTripSupplies{}, err
	}
	if err := e.validateFishingSupplyAvailabilityTx(ctx, tx, userID, loadout, counts); err != nil {
		return FishingTripSupplies{}, err
	}
	for _, entry := range sortedFishingSupplyEntries(loadout, counts) {
		if err := e.store.LedgerConsumeTemplateTx(ctx, tx, userID, entry.ItemID, entry.Quantity, InventoryLedgerMeta{
			SourceSurface: "inventory", TargetSurface: "fishing_pack",
			Details: map[string]any{"type": "fishing_supply_reserve"},
		}); err != nil {
			return FishingTripSupplies{}, err
		}
	}
	supplies := FishingTripSupplies{Initial: counts, Remaining: counts}
	if err := e.loadFishingSupplyWearTx(ctx, tx, userID, loadout, &supplies); err != nil {
		return FishingTripSupplies{}, err
	}
	return supplies, nil
}

func (e *fishingExecutor) returnFishingSuppliesTx(ctx context.Context, tx pgx.Tx, userID int64, loadout FishingLoadoutSnapshot, supplies *FishingTripSupplies) error {
	if supplies == nil || supplies.Returned {
		return nil
	}
	if err := e.saveFishingSupplyWearTx(ctx, tx, userID, loadout, *supplies); err != nil {
		return err
	}
	for itemID, quantity := range fishingSupplyItems(loadout, supplies.Remaining) {
		if err := e.store.LedgerGrantTemplateTx(ctx, tx, userID, itemID, quantity, InventoryLedgerMeta{
			SourceSurface: "fishing_pack", TargetSurface: "inventory",
			Details: map[string]any{"type": "fishing_supply_return"},
		}); err != nil {
			return err
		}
	}
	supplies.Returned = true
	return nil
}

func fishingComponentWearPerFight(loadout FishingLoadoutSnapshot, slot string) float64 {
	for _, component := range loadout.FrozenComponents {
		if component.Slot == slot && component.WearPerFight > 0 && !math.IsNaN(component.WearPerFight) && !math.IsInf(component.WearPerFight, 0) {
			return component.WearPerFight
		}
	}
	return 0
}

func fishingWearMultiplier(value float64) float64 {
	if value <= 0 || math.IsNaN(value) || math.IsInf(value, 0) {
		return 1
	}
	return clampFloat(value, 0.1, 2)
}

func consumeFishingWear(remaining *int64, consumed *int64, progress *float64, amount float64) int64 {
	if remaining == nil || consumed == nil || progress == nil || *remaining <= 0 || amount <= 0 || math.IsNaN(amount) || math.IsInf(amount, 0) {
		return 0
	}
	current := *progress
	if !validFishingWearProgress(current) {
		current = 0
	}
	next := current + clampFloat(amount, 0, 0.95)
	used := int64(0)
	if next+1e-9 >= 1 {
		used = 1
		*remaining = *remaining - 1
		*consumed = *consumed + 1
		next -= 1
		if *remaining == 0 {
			next = 0
		}
	}
	*progress = roundFloat(clampFloat(next, 0, 0.999999), 6)
	return used
}

func applyFishingTackleWear(supplies *FishingTripSupplies, loadout FishingLoadoutSnapshot, species catalog.FishingSpecies) map[string]int64 {
	costs := map[string]int64{}
	if supplies == nil {
		return costs
	}
	mainLineUsed := consumeFishingWear(
		&supplies.Remaining.MainLine,
		&supplies.Consumed.MainLine,
		&supplies.Wear.MainLine,
		fishingComponentWearPerFight(loadout, "main_line")*fishingWearMultiplier(species.MainLineWearMultiplier),
	)
	if mainLineUsed > 0 {
		if itemID := strings.TrimSpace(loadout.Components.MainLineItemID); itemID != "" {
			costs[itemID] += mainLineUsed
		}
	}
	leaderUsed := consumeFishingWear(
		&supplies.Remaining.Leader,
		&supplies.Consumed.Leader,
		&supplies.Wear.Leader,
		fishingComponentWearPerFight(loadout, "leader")*fishingWearMultiplier(species.LeaderWearMultiplier),
	)
	if leaderUsed > 0 {
		if itemID := strings.TrimSpace(loadout.Components.LeaderItemID); itemID != "" {
			costs[itemID] += leaderUsed
		}
	}
	return costs
}

func consumeFishingTerminalSupplies(supplies *FishingTripSupplies, loadout FishingLoadoutSnapshot, includeMainLine bool) {
	if supplies == nil {
		return
	}
	consume := func(remaining *int64, consumed *int64) {
		if *remaining > 0 {
			*remaining = *remaining - 1
			*consumed = *consumed + 1
		}
	}
	if includeMainLine {
		consume(&supplies.Remaining.MainLine, &supplies.Consumed.MainLine)
		supplies.Wear.MainLine = 0
	}
	consume(&supplies.Remaining.Leader, &supplies.Consumed.Leader)
	supplies.Wear.Leader = 0
	if loadout.Method == "lure" {
		consume(&supplies.Remaining.Lure, &supplies.Consumed.Lure)
		return
	}
	consume(&supplies.Remaining.Hook, &supplies.Consumed.Hook)
	consume(&supplies.Remaining.FloatOrSinker, &supplies.Consumed.FloatOrSinker)
}

func fishingScaleglowDrop(seed, landedOrdinal int64) bool {
	streamSeed := seed ^ int64(stableFishingHash(fmt.Sprintf("scaleglow:%d", landedOrdinal)))
	return rand.New(rand.NewSource(streamSeed)).Float64() < 0.002
}

func (e *fishingExecutor) recordFishingEncounterTx(ctx context.Context, tx pgx.Tx, userID int64, speciesID string) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO fishing_species_knowledge(user_id, species_id, encounters, catches, releases, updated_at)
		VALUES($1, $2, 1, 0, 0, now())
		ON CONFLICT (user_id, species_id) DO UPDATE
		SET encounters = fishing_species_knowledge.encounters + 1, updated_at = now()
	`, userID, speciesID)
	return err
}

func (e *fishingExecutor) landFishingCatchTx(ctx context.Context, tx pgx.Tx, userID int64, tripID, tournamentDivision string, loadout FishingLoadoutSnapshot, species catalog.FishingSpecies, fish FishInstanceData, rule FishingKeepRule, caughtAt time.Time) (FishingCatchView, FishingRecordFlag, bool, bool, float64, error) {
	rulesetVersion := strings.TrimSpace(fish.RulesetVersion)
	if rulesetVersion == "" {
		rulesetVersion = FishingRulesetVersion
		fish.RulesetVersion = rulesetVersion
	}
	var previousCatches int64
	err := tx.QueryRow(ctx, `
		SELECT catches FROM fishing_species_knowledge WHERE user_id = $1 AND species_id = $2 FOR UPDATE
	`, userID, fish.SpeciesID).Scan(&previousCatches)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return FishingCatchView{}, FishingRecordFlag{}, false, false, 0, err
	}
	var bestLength, bestWeight float64
	err = tx.QueryRow(ctx, `
		SELECT best_length_cm, best_weight_kg
		FROM fishing_records
		WHERE user_id = $1 AND species_id = $2 AND ruleset_version = $3
		FOR UPDATE
	`, userID, fish.SpeciesID, rulesetVersion).Scan(&bestLength, &bestWeight)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return FishingCatchView{}, FishingRecordFlag{}, false, false, 0, err
	}
	record := FishingRecordFlag{SpeciesID: fish.SpeciesID, Length: fish.LengthCM > bestLength, Weight: fish.WeightKG > bestWeight}
	if record.Length {
		fish.RecordFlags = append(fish.RecordFlags, "length")
	}
	if record.Weight {
		fish.RecordFlags = append(fish.RecordFlags, "weight")
	}
	kept := fishingShouldKeep(rule, fish, record.Length || record.Weight)
	var instanceID *int64
	if kept {
		id, err := e.store.addFishInstanceTx(ctx, tx, userID, fish)
		if err != nil {
			return FishingCatchView{}, FishingRecordFlag{}, false, false, 0, err
		}
		instanceID = &id
	}
	fishRaw, err := json.Marshal(fish)
	if err != nil {
		return FishingCatchView{}, FishingRecordFlag{}, false, false, 0, err
	}
	var catchID int64
	if err := tx.QueryRow(ctx, `
		INSERT INTO fishing_catches(trip_id, user_id, species_id, fish_data_jsonb, kept, inventory_instance_id, caught_at, created_at)
		VALUES($1, $2, $3, $4::jsonb, $5, $6, $7, now())
		RETURNING id
	`, tripID, userID, fish.SpeciesID, string(fishRaw), kept, instanceID, caughtAt).Scan(&catchID); err != nil {
		return FishingCatchView{}, FishingRecordFlag{}, false, false, 0, err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO fishing_records(user_id, species_id, ruleset_version, best_length_cm, best_length_catch_id, best_weight_kg, best_weight_catch_id, updated_at)
		VALUES($1, $2, $3, $4, $5, $6, $5, now())
		ON CONFLICT (user_id, species_id, ruleset_version) DO UPDATE
		SET best_length_cm = GREATEST(fishing_records.best_length_cm, EXCLUDED.best_length_cm),
		    best_length_catch_id = CASE WHEN EXCLUDED.best_length_cm > fishing_records.best_length_cm THEN EXCLUDED.best_length_catch_id ELSE fishing_records.best_length_catch_id END,
		    best_weight_kg = GREATEST(fishing_records.best_weight_kg, EXCLUDED.best_weight_kg),
		    best_weight_catch_id = CASE WHEN EXCLUDED.best_weight_kg > fishing_records.best_weight_kg THEN EXCLUDED.best_weight_catch_id ELSE fishing_records.best_weight_catch_id END,
		    updated_at = CASE WHEN EXCLUDED.best_length_cm > fishing_records.best_length_cm OR EXCLUDED.best_weight_kg > fishing_records.best_weight_kg THEN now() ELSE fishing_records.updated_at END
	`, userID, fish.SpeciesID, rulesetVersion, fish.LengthCM, catchID, fish.WeightKG); err != nil {
		return FishingCatchView{}, FishingRecordFlag{}, false, false, 0, err
	}
	releaseInc := int64(0)
	if !kept {
		releaseInc = 1
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO fishing_species_knowledge(user_id, species_id, encounters, catches, releases, first_caught_at, last_caught_at, updated_at)
		VALUES($1, $2, 1, 1, $3, $4, $4, now())
		ON CONFLICT (user_id, species_id) DO UPDATE
		SET catches = fishing_species_knowledge.catches + 1,
		    releases = fishing_species_knowledge.releases + EXCLUDED.releases,
		    first_caught_at = COALESCE(fishing_species_knowledge.first_caught_at, EXCLUDED.first_caught_at),
		    last_caught_at = EXCLUDED.last_caught_at,
		    updated_at = now()
	`, userID, fish.SpeciesID, releaseInc, caughtAt); err != nil {
		return FishingCatchView{}, FishingRecordFlag{}, false, false, 0, err
	}
	if rulesetVersion == FishingRulesetVersion {
		if err := e.store.recordFishingTournamentEntryTx(ctx, tx, userID, catchID, tournamentDivision, loadout, species, fish, caughtAt); err != nil {
			return FishingCatchView{}, FishingRecordFlag{}, false, false, 0, err
		}
	}
	if err := e.store.pruneFishingCatchHistoryTx(ctx, tx, userID); err != nil {
		return FishingCatchView{}, FishingRecordFlag{}, false, false, 0, err
	}
	isNew := previousCatches == 0
	gain := fishingCatchExpGain(species, isNew, record.Length || record.Weight)
	view := FishingCatchView{CatchID: catchID, TripID: tripID, SpeciesID: fish.SpeciesID, Fish: fish, Kept: kept, InventoryInstanceID: instanceID, CaughtAt: caughtAt, Record: record}
	return view, record, isNew, kept, gain, nil
}

func (s *Store) recordFishingTournamentEntryTx(ctx context.Context, tx pgx.Tx, userID, catchID int64, requestedDivision string, loadout FishingLoadoutSnapshot, species catalog.FishingSpecies, fish FishInstanceData, caughtAt time.Time) error {
	tournament, err := s.ensureFishingTournamentTx(ctx, tx, caughtAt)
	if err != nil {
		return err
	}
	if !fishingTournamentCatchEligible(tournament, fish.SpeciesID, caughtAt) {
		return nil
	}
	divisions := make([]string, 0, 2)
	if fishingTournamentUsesStandardLoadout(tournament) {
		requestedDivision = normalizeFishingTournamentDivision(requestedDivision)
		if requestedDivision == fishingTournamentDivisionOpen {
			divisions = append(divisions, fishingTournamentDivisionOpen)
		} else if requestedDivision == fishingTournamentDivisionStandard {
			standard, loadoutErr := s.validatedFishingTournamentStandardLoadout()
			if loadoutErr != nil {
				return loadoutErr
			}
			if fishingLoadoutMatchesTournamentStandard(loadout, standard) {
				divisions = append(divisions, fishingTournamentDivisionStandard)
			}
		}
	} else {
		divisions = append(divisions, fishingTournamentDivisionOpen)
		if fishingStandardTournamentQualifies(tournament.ID, catchID, species, fish) {
			divisions = append(divisions, fishingTournamentDivisionStandard)
		}
	}
	for _, division := range divisions {
		if err := s.upsertFishingTournamentCatchEntryTx(ctx, tx, userID, tournament.ID, division, catchID); err != nil {
			return err
		}
	}
	return nil
}

func fishingTournamentCatchEligible(tournament FishingTournament, speciesID string, caughtAt time.Time) bool {
	return strings.TrimSpace(speciesID) != "" &&
		tournament.TargetSpeciesID == speciesID &&
		!caughtAt.Before(tournament.StartsAt) &&
		caughtAt.Before(tournament.EndsAt)
}

func (s *Store) upsertFishingTournamentCatchEntryTx(ctx context.Context, tx pgx.Tx, userID int64, tournamentID, division string, catchID int64) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO fishing_tournament_entries(tournament_id, user_id, division, catch_id, fish_instance_id, source_kind, length_cm, weight_kg, submitted_at)
		SELECT t.id, c.user_id, $3, c.id, NULL, 'catch',
		       (c.fish_data_jsonb->>'lengthCm')::double precision,
		       ROUND((c.fish_data_jsonb->>'weightKg')::numeric, 3)::double precision,
		       c.caught_at
		FROM fishing_tournaments t
		JOIN fishing_catches c ON c.id = $4 AND c.user_id = $2
		WHERE t.id = $1
		  AND c.species_id = t.target_species_id
		  AND c.fish_data_jsonb->>'speciesId' = t.target_species_id
		  AND c.caught_at >= t.starts_at
		  AND c.caught_at < t.ends_at
		ON CONFLICT (tournament_id, user_id, division) DO UPDATE
		SET catch_id = EXCLUDED.catch_id, fish_instance_id = NULL, source_kind = 'catch', length_cm = EXCLUDED.length_cm, weight_kg = EXCLUDED.weight_kg, submitted_at = EXCLUDED.submitted_at
		WHERE EXCLUDED.weight_kg > fishing_tournament_entries.weight_kg
		   OR (EXCLUDED.weight_kg = fishing_tournament_entries.weight_kg AND EXCLUDED.length_cm > fishing_tournament_entries.length_cm)
	`, tournamentID, userID, division, catchID)
	return err
}

func fishingStandardTournamentQualifies(tournamentID string, catchID int64, species catalog.FishingSpecies, fish FishInstanceData) bool {
	return fishingStandardTournamentQualifiesForRuleset(tournamentID, catchID, species, fish, FishingRulesetVersion)
}

func fishingStandardTournamentQualifiesForRuleset(tournamentID string, catchID int64, species catalog.FishingSpecies, fish FishInstanceData, rulesetVersion string) bool {
	standardRules := fishingStandardTournamentRules()
	seed := int64(stableFishingHash(fmt.Sprintf("%s:%d:standard", tournamentID, catchID)))
	result := resolveFishingFightForRuleset(species, fish, standardRules.loadoutSnapshot(), standardRules.Strategy, fishingDefaultSafetyLoad, rulesetVersion, rand.New(rand.NewSource(seed)))
	return result.Landed && !result.LineBreak
}

func (e *fishingExecutor) saveFishingTripRuntimeTx(ctx context.Context, tx pgx.Tx, userID, queueID int64, runtime fishingExecutorRuntime, finished bool, at time.Time) error {
	summaryRaw, err := json.Marshal(runtime.Summary)
	if err != nil {
		return err
	}
	suppliesRaw, err := json.Marshal(runtime.Supplies)
	if err != nil {
		return err
	}
	currentRaw := []byte(`{}`)
	if runtime.CurrentFish != nil {
		sizeClass := fishingFishSizeClass(runtime.Rules, *runtime.CurrentFish)
		stamina := 100.0
		if runtime.Phase == fishingPhaseLanded || runtime.Phase == fishingPhaseResting {
			stamina = 0
		}
		currentRaw, err = json.Marshal(map[string]any{
			"sizeClass": sizeClass, "stamina": stamina,
			"lineLoadKg": runtime.CurrentLineLoad,
			"lineState":  fishingLineState(runtime.CurrentLineLoad, runtime.Loadout.MaxLoadKG, runtime.SafetyLoadPercent),
		})
		if err != nil {
			return err
		}
	}
	status := "running"
	stopReason := ""
	var endedAt *time.Time
	if finished {
		status = "completed"
		stopReason = runtime.Summary.StopReason
		endedAt = &at
	}
	tag, err := tx.Exec(ctx, `
		UPDATE fishing_trips
		SET status = $3, phase = $4, phase_started_at = $5, phase_ends_at = $6,
		    current_fish_jsonb = $7::jsonb, summary_jsonb = $8::jsonb,
		    supplies_jsonb = $9::jsonb, stop_reason = $10,
		    event_seq = $11, ended_at = $12, updated_at = now()
		WHERE id = $1 AND user_id = $2 AND status = 'running'
		  AND queue_id = $13 AND actor_type = $14 AND actor_id = $15
	`, runtime.TripID, userID, status, runtime.Phase, time.UnixMilli(runtime.PhaseStartedAtMS),
		time.UnixMilli(runtime.PhaseEndsAtMS), string(currentRaw), string(summaryRaw), string(suppliesRaw),
		stopReason, runtime.EventSeq, endedAt, queueID, runtime.ActorType, runtime.ActorID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return fmt.Errorf("fishing runtime checkpoint rejected user_id=%d queue_id=%d trip_id=%s", userID, queueID, runtime.TripID)
	}
	if err := e.store.persistFishingTripEventsTx(ctx, tx, runtime.TripID, runtime.pendingEvents); err != nil {
		return err
	}
	if finished {
		return e.store.pruneFishingTripEventsTx(ctx, tx, userID)
	}
	return nil
}

func marshalFishingEnvelope(actionID string, runtime fishingExecutorRuntime) (json.RawMessage, error) {
	runtimeRaw, err := json.Marshal(runtime)
	if err != nil {
		return nil, err
	}
	return json.Marshal(queueRuntimeEnvelope{ActionID: actionID, ExecutorID: "fishing_trip", Runtime: runtimeRaw})
}

func fishingRuntimeCanSettle(runtime fishingExecutorRuntime, item QueueRuntimeItem) bool {
	actorType := normalizeActorType(item.ActorType)
	actorID := item.ActorID
	if actorType == ActionActorTypePlayer {
		actorID = 0
	}
	if runtime.ActorType != actorType || runtime.ActorID != actorID || runtime.Supplies.Returned ||
		runtime.PhaseStartedAtMS <= 0 || runtime.PhaseEndsAtMS <= runtime.PhaseStartedAtMS {
		return false
	}
	switch runtime.Phase {
	case fishingPhaseWaiting, fishingPhaseBite, fishingPhaseFighting, fishingPhaseLanded, fishingPhaseResting:
		return true
	default:
		return false
	}
}

func parseFishingExecutorRuntime(payloadRaw []byte) (fishingExecutorRuntime, bool) {
	if len(payloadRaw) == 0 {
		return fishingExecutorRuntime{}, false
	}
	var envelope queueRuntimeEnvelope
	if err := json.Unmarshal(payloadRaw, &envelope); err != nil || envelope.ExecutorID != "fishing_trip" {
		return fishingExecutorRuntime{}, false
	}
	var runtime fishingExecutorRuntime
	if err := json.Unmarshal(envelope.Runtime, &runtime); err != nil {
		return fishingExecutorRuntime{}, false
	}
	if strings.TrimSpace(runtime.TripID) == "" || strings.TrimSpace(runtime.SpotID) == "" || runtime.DurationMS <= 0 {
		return fishingExecutorRuntime{}, false
	}
	runtime.ActorType = normalizeActorType(runtime.ActorType)
	if runtime.ActorType == ActionActorTypePlayer {
		runtime.ActorID = 0
	}
	runtime.TournamentDivision = normalizeFishingTournamentDivision(runtime.TournamentDivision)
	if runtime.TournamentDivision == "" {
		return fishingExecutorRuntime{}, false
	}
	return runtime, true
}

func setFishingExecutorRuntime(payloadRaw []byte, runtime fishingExecutorRuntime) (json.RawMessage, error) {
	var envelope queueRuntimeEnvelope
	if err := json.Unmarshal(payloadRaw, &envelope); err != nil {
		return nil, err
	}
	runtimeRaw, err := json.Marshal(runtime)
	if err != nil {
		return nil, err
	}
	envelope.Runtime = runtimeRaw
	envelope.ExecutorID = "fishing_trip"
	return json.Marshal(envelope)
}

func fishingRuntimeRNG(runtime *fishingExecutorRuntime) *rand.Rand {
	runtime.EventSeq++
	seed := runtime.Seed + runtime.EventSeq*(-7046029254386353131)
	if runtime.batchRNG != nil {
		// The generator is phase-local in the legacy path. Reset every phase,
		// including its cached read state, rather than continuing its stream.
		runtime.batchRNG.Seed(seed)
		return runtime.batchRNG
	}
	return rand.New(rand.NewSource(seed))
}

func fishingConditionsAt(water catalog.FishingWater, spot catalog.FishingSpot, at time.Time) FishingConditions {
	gameMinute := int((at.Unix() / 60) % 240)
	if gameMinute < 0 {
		gameMinute += 240
	}
	gameHour := gameMinute / 10
	seasonIndex := int((at.Unix() / int64((7 * 24 * time.Hour).Seconds())) % 4)
	if seasonIndex < 0 {
		seasonIndex += 4
	}
	seasons := []string{"spring", "summer", "autumn", "winter"}
	season := seasons[seasonIndex]
	bucket := at.Unix() / int64((30 * time.Minute).Seconds())
	weatherRoll := stableFishingHash(water.ID+":"+strconvInt64(bucket)) % 100
	weather := "clear"
	switch water.WaterKind {
	case "cold_lake", "cold_deep_lake":
		switch {
		case weatherRoll < 14:
			weather = "snow"
		case weatherRoll < 29:
			weather = "mist"
		case weatherRoll < 50:
			weather = "cloud"
		default:
			weather = "clear"
		}
	case "river", "temperate_river":
		switch {
		case weatherRoll < 9:
			weather = "storm"
		case weatherRoll < 30:
			weather = "rain"
		case weatherRoll < 48:
			weather = "cloud"
		default:
			weather = "clear"
		}
	default:
		switch {
		case weatherRoll < 16:
			weather = "mist"
		case weatherRoll < 28:
			weather = "rain"
		case weatherRoll < 55:
			weather = "cloud"
		case weatherRoll < 76:
			weather = "sunny"
		default:
			weather = "clear"
		}
	}
	seasonOffset := map[string]float64{"spring": 1, "summer": 6, "autumn": -1, "winter": -6}[season]
	diurnal := math.Sin((float64(gameHour)-6)/24*2*math.Pi) * 2.8
	temperature := water.BaseTemperatureC + seasonOffset + diurnal
	oxygen := clampFloat(spot.Oxygen+(12-temperature)*0.006, 0.48, 1)
	activity := 0.88 + oxygen*0.2
	if gameHour >= 5 && gameHour <= 8 || gameHour >= 17 && gameHour <= 20 {
		activity += 0.18
	}
	switch weather {
	case "rain", "mist", "cloud":
		activity += 0.08
	case "storm":
		activity -= 0.12
	}
	activity = clampFloat(activity, 0.55, 1.38)
	band := "steady"
	if activity >= 1.12 {
		band = "busy"
	} else if activity < 0.83 {
		band = "quiet"
	}
	nextHour := (gameHour + 1) % 24
	trend := "steady"
	if (nextHour >= 5 && nextHour <= 8) || (nextHour >= 17 && nextHour <= 20) {
		if !(gameHour >= 5 && gameHour <= 8 || gameHour >= 17 && gameHour <= 20) {
			trend = "rising"
		}
	} else if gameHour == 8 || gameHour == 20 {
		trend = "falling"
	}
	return FishingConditions{
		GameHour: gameHour, Season: season, Weather: weather,
		Temperature: roundFloat(temperature, 1), Oxygen: roundFloat(oxygen, 2),
		Activity: roundFloat(activity, 3), ActivityBand: band, Trend: trend,
	}
}

func fishingWaitDuration(spot catalog.FishingSpot, loadout FishingLoadoutSnapshot, pace string, pressure float64, conditions FishingConditions, rng *rand.Rand) time.Duration {
	base := 52 + rng.Float64()*92
	waterMultiplier := clampFloat(1+loadout.WaterBiteModifiers[spot.WaterID], 0.55, 1.5)
	multiplier := 1 / math.Max(0.35, spot.BaseBiteRate*conditions.Activity*loadout.BiteMultiplier*waterMultiplier)
	multiplier *= 1 + clampFloat(pressure, 0, 1)*0.75
	multiplier *= 1 / (1 + clampFloat(loadout.Efficiency, 0, 1.5))
	switch pace {
	case fishingPacePatient:
		multiplier *= 1.15
	case fishingPaceSearch:
		multiplier *= 0.82
	case "legacy_conservative":
		multiplier *= 1.12
	case "legacy_aggressive":
		multiplier *= 0.88
	}
	seconds := clampFloat(base*multiplier, 28, 210)
	return time.Duration(math.Round(seconds)) * time.Second
}

func fishingBiteChance(spot catalog.FishingSpot, loadout FishingLoadoutSnapshot, pace string, pressure float64, conditions FishingConditions, layerMultiplier float64) float64 {
	waterMultiplier := clampFloat(1+loadout.WaterBiteModifiers[spot.WaterID], 0.55, 1.5)
	chance := 0.48 * spot.BaseBiteRate * conditions.Activity * loadout.BiteMultiplier * waterMultiplier
	chance *= 1 - clampFloat(pressure, 0, 1)*0.55
	chance *= 1 + loadout.Sensitivity*0.025
	chance *= clampFloat(layerMultiplier, 0.45, 1.15)
	switch pace {
	case fishingPacePatient:
		chance *= 1.08
	case fishingPaceSearch:
		chance *= 0.9
	case "legacy_conservative":
		chance *= 0.96
	case "legacy_aggressive":
		chance *= 1.05
	}
	return clampFloat(chance, 0.08, 0.9)
}

func selectFishingSpecies(species []catalog.FishingSpecies, spot catalog.FishingSpot, loadout FishingLoadoutSnapshot, conditions FishingConditions, targetLayer string, rng *rand.Rand) *catalog.FishingSpecies {
	type candidate struct {
		index  int
		weight float64
	}
	candidates := make([]candidate, 0, len(spot.SpeciesWeights))
	total := 0.0
	for index := range species {
		row := species[index]
		baseWeight := spot.SpeciesWeights[row.ID]
		if baseWeight <= 0 || row.WaterID != spot.WaterID {
			continue
		}
		active := containsInt(row.ActiveHours, conditions.GameHour)
		if row.Fantasy {
			if !active || (row.WindowWeather != "" && row.WindowWeather != conditions.Weather) {
				continue
			}
		} else if active {
			baseWeight *= 1.35
		} else {
			baseWeight *= 0.72
		}
		if containsFishingMethod(row.PreferredMethods, loadout.Method) {
			baseWeight *= 1.32
		} else {
			baseWeight *= 0.52
		}
		casting := loadout.Casting + float64(loadout.FishingLevel)*0.02
		castingFactor := clampFloat(0.92+(casting-row.CastingChallenge)*0.12, 0.55, 1.35)
		baseWeight *= castingFactor
		baseWeight *= fishingSpeciesLayerWeight(row, targetLayer)
		baseWeight *= clampFloat(1+loadout.SpeciesWeightModifiers[row.ID], 0.4, 1.8)
		if baseWeight <= 0 {
			continue
		}
		total += baseWeight
		candidates = append(candidates, candidate{index: index, weight: baseWeight})
	}
	if total <= 0 || len(candidates) == 0 {
		return nil
	}
	roll := rng.Float64() * total
	for _, row := range candidates {
		roll -= row.weight
		if roll <= 0 {
			copy := species[row.index]
			return &copy
		}
	}
	copy := species[candidates[len(candidates)-1].index]
	return &copy
}

func normalizeFishingTargetLayer(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case fishingTargetLayerAuto, fishingTargetLayerSurface, fishingTargetLayerMidwater, fishingTargetLayerNearBottom:
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return ""
	}
}

func normalizeFishingPace(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case fishingPacePatient, fishingPaceBalanced, fishingPaceSearch:
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return ""
	}
}

func effectiveFishingPace(pace, legacyStrategy string) string {
	if normalized := normalizeFishingPace(pace); normalized != "" {
		return normalized
	}
	switch normalizeFishingStrategy(legacyStrategy) {
	case "conservative":
		return "legacy_conservative"
	case "aggressive":
		return "legacy_aggressive"
	default:
		return fishingPaceBalanced
	}
}

func effectiveFishingTargetLayer(targetLayer string) string {
	if strings.TrimSpace(targetLayer) == "" {
		return ""
	}
	return normalizeFishingTargetLayer(targetLayer)
}

func fishingSpeciesLayerWeight(species catalog.FishingSpecies, targetLayer string) float64 {
	if targetLayer == "" || targetLayer == fishingTargetLayerAuto {
		return 1
	}
	for _, preferred := range fishingSpeciesLayers(species) {
		if preferred == targetLayer {
			return 1.45
		}
	}
	return 0.58
}

func fishingLayerBiteMultiplier(species []catalog.FishingSpecies, spot catalog.FishingSpot, loadout FishingLoadoutSnapshot, conditions FishingConditions, targetLayer string) float64 {
	if targetLayer == "" || targetLayer == fishingTargetLayerAuto {
		return 1
	}
	totalWeight := 0.0
	matchingWeight := 0.0
	for _, row := range species {
		weight := spot.SpeciesWeights[row.ID]
		if weight <= 0 || row.WaterID != spot.WaterID {
			continue
		}
		active := containsInt(row.ActiveHours, conditions.GameHour)
		if row.Fantasy {
			if !active || (row.WindowWeather != "" && row.WindowWeather != conditions.Weather) {
				continue
			}
		} else if active {
			weight *= 1.35
		} else {
			weight *= 0.72
		}
		if containsFishingMethod(row.PreferredMethods, loadout.Method) {
			weight *= 1.32
		} else {
			weight *= 0.52
		}
		weight *= clampFloat(1+loadout.SpeciesWeightModifiers[row.ID], 0.4, 1.8)
		totalWeight += weight
		for _, preferred := range fishingSpeciesLayers(row) {
			if preferred == targetLayer {
				matchingWeight += weight
				break
			}
		}
	}
	if totalWeight <= 0 {
		return 1
	}
	return clampFloat(0.62+matchingWeight/totalWeight*0.48, 0.62, 1.1)
}

func fishingSpeciesLayers(species catalog.FishingSpecies) []string {
	if len(species.PreferredLayers) > 0 {
		return species.PreferredLayers
	}
	switch strings.ToLower(strings.TrimSpace(species.Depth)) {
	case "shallow":
		return []string{fishingTargetLayerSurface}
	case "mid":
		return []string{fishingTargetLayerMidwater}
	case "bottom", "deep":
		return []string{fishingTargetLayerNearBottom}
	default:
		return nil
	}
}

func fishingSafetyLoadForStrategy(strategy string) int {
	switch normalizeFishingStrategy(strategy) {
	case "conservative":
		return 94
	case "aggressive":
		return 70
	default:
		return fishingDefaultSafetyLoad
	}
}

func generateFishingIndividual(species catalog.FishingSpecies, waterID, spotID, rulesetVersion string, caughtAt time.Time, pressure float64, rng *rand.Rand) FishInstanceData {
	rulesetVersion = strings.TrimSpace(rulesetVersion)
	if rulesetVersion == "" {
		rulesetVersion = FishingRulesetVersion
	}
	exponent := 1.85 + clampFloat(pressure, 0, 1)*0.65
	if rulesetVersion == fishingLegacyRulesetVersion {
		exponent = 2.05 + clampFloat(pressure, 0, 1)*0.7
	}
	ratio := math.Pow(rng.Float64(), exponent)
	length := species.MinLengthCM + (species.MaxLengthCM-species.MinLengthCM)*ratio
	condition := 0.82 + rng.Float64()*0.3
	health := clampFloat(0.86+rng.Float64()*0.14-(condition-1)*0.04, 0.78, 1)
	weight := species.ShapeFactor * math.Pow(length, 3) / 100_000 * condition
	ageDays := 0
	if rulesetVersion == fishingLegacyRulesetVersion || species.MaturityLengthCM <= species.MinLengthCM || species.MaturityAgeDays <= 0 || species.LifespanDays <= species.MaturityAgeDays {
		ageDays = int(math.Round(120 + math.Pow((length-species.MinLengthCM)/math.Max(1, species.MaxLengthCM-species.MinLengthCM), 1.65)*3_400 + rng.Float64()*180))
	} else {
		minimumAge := math.Max(30, float64(species.MaturityAgeDays)*0.12)
		maturityLength := clampFloat(species.MaturityLengthCM, species.MinLengthCM+0.1, species.MaxLengthCM-0.1)
		minimumBiologicalAge := minimumAge
		if length <= maturityLength {
			progress := (length - species.MinLengthCM) / math.Max(0.1, maturityLength-species.MinLengthCM)
			ageDays = int(math.Round(minimumAge + (float64(species.MaturityAgeDays)-minimumAge)*math.Pow(clampFloat(progress, 0, 1), 1.25)))
		} else {
			minimumBiologicalAge = float64(species.MaturityAgeDays)
			progress := (length - maturityLength) / math.Max(0.1, species.MaxLengthCM-maturityLength)
			ageDays = int(math.Round(float64(species.MaturityAgeDays) + (float64(species.LifespanDays)*0.9-float64(species.MaturityAgeDays))*math.Pow(clampFloat(progress, 0, 1), 1.7)))
		}
		ageDays += int(math.Round((rng.Float64() - 0.5) * float64(species.MaturityAgeDays) * 0.08))
		ageDays = int(clampFloat(float64(ageDays), minimumBiologicalAge, float64(species.LifespanDays)*0.95))
	}
	sex := "female"
	if rng.Float64() < 0.5 {
		sex = "male"
	}
	return FishInstanceData{
		SpeciesID: species.ID, AgeDays: ageDays, Sex: sex,
		LengthCM: roundFloat(length, 1), WeightKG: roundFloat(math.Max(0.01, weight), 3),
		Condition: roundFloat(condition, 3), Health: roundFloat(health, 3),
		WaterID: waterID, SpotID: spotID, CaughtAt: caughtAt, RulesetVersion: rulesetVersion,
		Origin: "wild", LifeState: "alive",
	}
}

func fishingLineLoad(species catalog.FishingSpecies, fish FishInstanceData, strategy string) float64 {
	return fishingLineLoadForRuleset(species, fish, strategy, FishingRulesetVersion)
}

func fishingLineLoadForRuleset(species catalog.FishingSpecies, fish FishInstanceData, strategy, rulesetVersion string) float64 {
	multiplier := 1.0
	switch normalizeFishingStrategy(strategy) {
	case "conservative":
		multiplier = 0.78
	case "aggressive":
		multiplier = 1.22
	}
	if strings.TrimSpace(rulesetVersion) == fishingLegacyRulesetVersion {
		return math.Max(0.15, fish.WeightKG*(0.62+species.FightPower*0.42)*multiplier)
	}
	// A balanced fight is deliberately close to the fish's displayed weight.
	// Fight power and strategy add a modest, legible peak-tension premium.
	return math.Max(0.15, fish.WeightKG*(0.82+0.10*math.Sqrt(math.Max(0, species.FightPower)))*multiplier)
}

func fishingFightDuration(species catalog.FishingSpecies, fish FishInstanceData, loadout FishingLoadoutSnapshot, strategy string) time.Duration {
	seconds := 11 + species.FightPower*7 + math.Sqrt(math.Max(0.01, fish.WeightKG))*4
	seconds /= 1 + (loadout.Control+float64(loadout.FishingLevel)*0.035)*0.055
	switch normalizeFishingStrategy(strategy) {
	case "conservative":
		seconds *= 1.28
	case "aggressive":
		seconds *= 0.78
	}
	return time.Duration(math.Round(clampFloat(seconds, 8, 95))) * time.Second
}

func resolveFishingFight(species catalog.FishingSpecies, fish FishInstanceData, loadout FishingLoadoutSnapshot, strategy string, safetyLoadPercent int, rng *rand.Rand) fishingFightResult {
	return resolveFishingFightForRuleset(species, fish, loadout, strategy, safetyLoadPercent, FishingRulesetVersion, rng)
}

func resolveFishingFightForRuleset(species catalog.FishingSpecies, fish FishInstanceData, loadout FishingLoadoutSnapshot, strategy string, safetyLoadPercent int, rulesetVersion string, rng *rand.Rand) fishingFightResult {
	return resolveFishingFightWithEffects(species, fish, loadout, strategy, safetyLoadPercent, rulesetVersion, fishingeffect.Modifiers{}, rng)
}

func resolveFishingFightWithEffects(species catalog.FishingSpecies, fish FishInstanceData, loadout FishingLoadoutSnapshot, strategy string, safetyLoadPercent int, rulesetVersion string, modifiers fishingeffect.Modifiers, rng *rand.Rand) fishingFightResult {
	lineLoad := fishingLineLoadForRuleset(species, fish, strategy, rulesetVersion)
	lineLoad *= 1 - clampFloat(modifiers.LineLoadReductionRate, 0, fishingeffect.MaxRate)
	ratedLoad := math.Max(0, loadout.MaxLoadKG)
	overloadThreshold := ratedLoad
	breakChance := 0.0
	if strings.TrimSpace(rulesetVersion) == fishingLegacyRulesetVersion {
		safeLoad := ratedLoad * clampFloat(float64(safetyLoadPercent)/100, 0.5, 1)
		overloadThreshold = safeLoad
		breakChance = 0.01
		if lineLoad > safeLoad {
			breakChance += 0.16 + (lineLoad-safeLoad)/math.Max(0.2, safeLoad)*0.62
		}
		if normalizeFishingStrategy(strategy) == "aggressive" {
			breakChance += 0.025
		}
		breakChance = clampFloat(breakChance, 0.005, 0.86)
	} else if lineLoad > ratedLoad {
		overloadRatio := (lineLoad - ratedLoad) / math.Max(0.2, ratedLoad)
		breakChance = clampFloat(0.08+overloadRatio*0.78, 0.08, 0.92)
	}
	if rng.Float64() < breakChance {
		reason := "line_break"
		if lineLoad > overloadThreshold {
			reason = "line_overload"
		}
		return fishingFightResult{LineBreak: true, Chance: 0, LineLoad: roundFloat(lineLoad, 3), FailureReason: reason}
	}
	control := loadout.Control + float64(loadout.FishingLevel)*0.035
	challenge := species.FightPower * (0.8 + math.Sqrt(math.Max(0.01, fish.WeightKG))*0.18)
	hooking := loadout.Hooking + float64(loadout.FishingLevel)*0.02
	chance := 0.64 + control*0.048 + loadout.Sensitivity*0.018 + hooking*0.04 - challenge*0.105 - species.HookingChallenge*0.035
	switch normalizeFishingStrategy(strategy) {
	case "conservative":
		chance += 0.09
	case "aggressive":
		chance -= 0.075
	}
	chance = clampFloat(chance, 0.18, 0.97)
	chance = math.Min(0.97, chance+clampFloat(modifiers.LandingBonusRate, 0, fishingeffect.MaxRate))
	landed := rng.Float64() < chance
	return fishingFightResult{
		Landed: landed, Chance: chance, LineLoad: roundFloat(lineLoad, 3),
		FailureReason: fishingEscapeReason(species, fish, loadout, strategy),
	}
}

func appendFishingTripEvent(runtime *fishingExecutorRuntime, event FishingTripEvent) {
	if runtime == nil || event.Kind == "" || event.At.IsZero() {
		return
	}
	if count := len(runtime.Summary.RecentEvents); count > 0 {
		lastSequence := runtime.Summary.RecentEvents[count-1].Sequence
		if runtime.TimelineSeq < lastSequence {
			runtime.TimelineSeq = lastSequence
		}
	}
	runtime.TimelineSeq++
	event.Sequence = runtime.TimelineSeq
	runtime.pendingEvents = append(runtime.pendingEvents, event)
	runtime.Summary.RecentEvents = append(runtime.Summary.RecentEvents, event)
	if overflow := len(runtime.Summary.RecentEvents) - fishingRecentEventLimit; overflow > 0 {
		copy(runtime.Summary.RecentEvents, runtime.Summary.RecentEvents[overflow:])
		runtime.Summary.RecentEvents = runtime.Summary.RecentEvents[:fishingRecentEventLimit]
	}
}

func fishingFishSizeClass(rules fishingRulesSnapshot, fish FishInstanceData) string {
	sizeClass := "small"
	if species := rules.speciesByID(fish.SpeciesID); species != nil {
		ratio := (fish.LengthCM - species.MinLengthCM) / math.Max(1, species.MaxLengthCM-species.MinLengthCM)
		if ratio > 0.72 {
			return "large"
		}
		if ratio > 0.38 {
			return "medium"
		}
	}
	return sizeClass
}

func fishingEscapeReason(species catalog.FishingSpecies, fish FishInstanceData, loadout FishingLoadoutSnapshot, strategy string) string {
	control := loadout.Control + float64(loadout.FishingLevel)*0.035
	challenge := species.FightPower * (0.8 + math.Sqrt(math.Max(0.01, fish.WeightKG))*0.18)
	controlPressure := challenge*0.105 - control*0.048 - loadout.Sensitivity*0.018
	switch normalizeFishingStrategy(strategy) {
	case "conservative":
		controlPressure -= 0.09
	case "aggressive":
		controlPressure += 0.075
	}
	hookHold := loadout.Hooking + float64(loadout.FishingLevel)*0.02
	hookPressure := species.HookingChallenge*0.035 - hookHold*0.04
	if hookPressure > controlPressure {
		return "hook_slipped"
	}
	return "lost_control"
}

func fishingLineState(lineLoad, maxLoad float64, safetyLoadPercent int) string {
	safe := maxLoad * clampFloat(float64(safetyLoadPercent)/100, 0.5, 1)
	if safe <= 0 {
		return "steady"
	}
	ratio := lineLoad / safe
	if ratio >= 1 {
		return "critical"
	}
	if ratio >= 0.72 {
		return "tight"
	}
	return "steady"
}

func containsInt(values []int, value int) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}

func appendUniqueFishingString(values []string, value string) []string {
	for _, candidate := range values {
		if candidate == value {
			return values
		}
	}
	return append(values, value)
}

func strconvInt64(value int64) string {
	if value == 0 {
		return "0"
	}
	negative := value < 0
	if negative {
		value = -value
	}
	var buffer [24]byte
	index := len(buffer)
	for value > 0 {
		index--
		buffer[index] = byte('0' + value%10)
		value /= 10
	}
	if negative {
		index--
		buffer[index] = '-'
	}
	return string(buffer[index:])
}

func sortFishingRecords(records []FishingRecordFlag) {
	sort.SliceStable(records, func(i, j int) bool { return records[i].SpeciesID < records[j].SpeciesID })
}
