package store

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"mysecondwebgame/server/internal/catalog"

	"github.com/jackc/pgx/v5"
)

type buildingExecutor struct {
	store *Store
}

type buildingExecutorArgs struct {
	BuildingID string `json:"buildingId"`
}

func (buildingExecutorArgs) RequiresSingleQueueRun() bool { return true }

type buildingExecutorRuntime struct {
	BuildingID        string           `json:"buildingId"`
	MaxLevel          int64            `json:"maxLevel"`
	FromLevel         int64            `json:"fromLevel,omitempty"`
	ToLevel           int64            `json:"toLevel,omitempty"`
	DurationMs        int64            `json:"durationMs,omitempty"`
	Costs             map[string]int64 `json:"costs,omitempty"`
	StatusRequirement map[string]int64 `json:"statusRequirement,omitempty"`
}

type buildingRunContext struct {
	BuildingID string           `json:"buildingId"`
	FromLevel  int64            `json:"fromLevel"`
	ToLevel    int64            `json:"toLevel"`
	Costs      map[string]int64 `json:"costs"`
}

func newBuildingExecutor(store *Store) *buildingExecutor {
	return &buildingExecutor{store: store}
}

func (e *buildingExecutor) ActionID() string {
	return "building_upgrade"
}

func (e *buildingExecutor) OfflineProjectionPolicy() QueueOfflineProjectionPolicy {
	return QueueOfflineProjectionTerminal
}

func (e *buildingExecutor) durationRefreshMode() queueDurationRefreshMode {
	return queueDurationRefreshPending
}

func (e *buildingExecutor) refreshQueueDurationTx(ctx context.Context, tx pgx.Tx, userID int64, item QueueRuntimeItem) (json.RawMessage, bool, error) {
	runtime, ok := parseBuildingExecutorRuntime(item.PayloadRaw)
	if !ok {
		return nil, false, errors.New("invalid building runtime payload")
	}
	def := e.store.gameData.BuildingDefinitionByID(runtime.BuildingID)
	if def == nil {
		return nil, false, ErrBuildingNotFound
	}
	currentLevel, err := e.store.getPlayerBuildingLevelTx(ctx, tx, userID, runtime.BuildingID)
	if err != nil {
		return nil, false, err
	}
	baseDurationS, _, _ := buildingLevelPlan(def, currentLevel)
	baseDuration := time.Duration(baseDurationS) * time.Second
	resolvedDuration, _, err := e.store.resolveDurationAndBonusTx(
		ctx, tx, userID, item.ActorType, item.ActorID, item.Spec, baseDuration,
	)
	if err != nil {
		return nil, false, err
	}
	runtime.DurationMs = int64(resolvedDuration / time.Millisecond)
	nextPayload, err := setQueueRuntimePayload(item.PayloadRaw, runtime)
	if err != nil {
		return nil, false, err
	}
	return nextPayload, string(nextPayload) != string(item.PayloadRaw), nil
}

func (e *buildingExecutor) ValidateAndNormalizeArgs(raw json.RawMessage) (any, error) {
	if len(raw) == 0 {
		return nil, ErrBuildingArgsInvalid
	}
	var args buildingExecutorArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, ErrBuildingArgsInvalid
	}
	args.BuildingID = strings.TrimSpace(args.BuildingID)
	if args.BuildingID == "" {
		return nil, ErrBuildingArgsInvalid
	}
	return args, nil
}

func (e *buildingExecutor) FreezeRunContextTx(ctx context.Context, tx pgx.Tx, userID int64, actorType string, actorID int64, spec ActionSpec, rawArgs any, _ time.Time) (json.RawMessage, error) {
	args, ok := rawArgs.(buildingExecutorArgs)
	if !ok {
		return nil, ErrBuildingArgsInvalid
	}
	def := e.store.gameData.BuildingDefinitionByID(args.BuildingID)
	if def == nil {
		return nil, ErrBuildingNotFound
	}
	currentLevel, err := e.store.getPlayerBuildingLevelTx(ctx, tx, userID, args.BuildingID)
	if err != nil {
		return nil, err
	}
	maxLevel := maxInt64(1, def.MaxLevel)
	if currentLevel >= maxLevel {
		return nil, ErrBuildingMaxLevel
	}
	toLevel := currentLevel + 1
	if toLevel > maxLevel {
		toLevel = maxLevel
	}
	baseDurationS, _, _ := buildingLevelPlan(def, currentLevel)
	resolvedDuration, _, err := e.store.resolveDurationAndBonusTx(
		ctx,
		tx,
		userID,
		actorType,
		actorID,
		spec,
		time.Duration(baseDurationS)*time.Second,
	)
	if err != nil {
		return nil, err
	}
	runtimeRaw, err := json.Marshal(buildingExecutorRuntime{
		BuildingID: args.BuildingID,
		MaxLevel:   maxLevel,
		FromLevel:  currentLevel,
		ToLevel:    toLevel,
		DurationMs: int64(resolvedDuration / time.Millisecond),
	})
	if err != nil {
		return nil, err
	}
	payload, err := json.Marshal(queueRuntimeEnvelope{
		ActionID:   spec.ID,
		ExecutorID: e.ActionID(),
		Runtime:    runtimeRaw,
	})
	if err != nil {
		return nil, err
	}
	return payload, nil
}

func (e *buildingExecutor) StartRunTx(ctx context.Context, tx pgx.Tx, userID int64, item QueueRuntimeItem, _ time.Time) (json.RawMessage, error) {
	runtime, ok := parseBuildingExecutorRuntime(item.PayloadRaw)
	if !ok {
		return nil, ErrBuildingArgsInvalid
	}
	def := e.store.gameData.BuildingDefinitionByID(runtime.BuildingID)
	if def == nil {
		return nil, ErrBuildingNotFound
	}
	currentLevel, err := lockPlayerBuildingLevelTx(ctx, tx, userID, runtime.BuildingID)
	if err != nil {
		return nil, err
	}
	maxLevel := maxInt64(1, def.MaxLevel)
	if currentLevel >= maxLevel {
		return nil, ErrBuildingMaxLevel
	}
	durationSeconds, costs, statusRequirement := buildingLevelPlan(def, currentLevel)
	if ok, err := e.store.meetStatusRequireTx(ctx, tx, userID, statusRequirement); err != nil {
		return nil, err
	} else if !ok {
		return nil, ErrStatusRequireNotMet
	}
	if err := e.store.applyActionCostsTx(ctx, tx, userID, costs); err != nil {
		return nil, err
	}
	toLevel := currentLevel + 1
	if toLevel > maxLevel {
		toLevel = maxLevel
	}
	runtime.FromLevel = currentLevel
	runtime.ToLevel = toLevel
	runtime.MaxLevel = maxLevel
	runtime.DurationMs = int64((time.Duration(durationSeconds) * time.Second) / time.Millisecond)
	runtime.Costs = costs
	runtime.StatusRequirement = statusRequirement
	runCtx := buildingRunContext{
		BuildingID: runtime.BuildingID,
		FromLevel:  currentLevel,
		ToLevel:    toLevel,
		Costs:      costs,
	}
	return setBuildingRuntimeAndContext(item.PayloadRaw, runtime, runCtx)
}

func (e *buildingExecutor) CancelRunTx(ctx context.Context, tx pgx.Tx, userID int64, item QueueRuntimeItem) error {
	runCtx, ok := parseBuildingRunContext(item.PayloadRaw)
	if !ok {
		return nil
	}
	return e.store.refundActionCostsTx(ctx, tx, userID, runCtx.Costs)
}

func (e *buildingExecutor) SettleNormalTx(ctx context.Context, tx pgx.Tx, userID int64, item QueueRuntimeItem, now time.Time) (SettleResult, error) {
	return e.SettleFastForwardTx(ctx, tx, userID, item, now, FastForwardBudget{MaxIterations: 1})
}

func (e *buildingExecutor) SettleFastForwardTx(ctx context.Context, tx pgx.Tx, userID int64, item QueueRuntimeItem, now time.Time, budget FastForwardBudget) (SettleResult, error) {
	runtime, ok := parseBuildingExecutorRuntime(item.PayloadRaw)
	if !ok {
		return SettleResult{
			CancelAfterSettle: true,
			CancelReason:      ActionQueueCancelReasonTargetInvalid,
			NextPayload:       item.PayloadRaw,
			ItemCosts:         map[string]int64{},
			ItemRewards:       map[string]int64{},
			AttrExp:           map[string]float64{},
		}, nil
	}
	def := e.store.gameData.BuildingDefinitionByID(runtime.BuildingID)
	if def == nil {
		return SettleResult{
			CancelAfterSettle: true,
			CancelReason:      ActionQueueCancelReasonTargetInvalid,
			NextPayload:       item.PayloadRaw,
			ItemCosts:         map[string]int64{},
			ItemRewards:       map[string]int64{},
			AttrExp:           map[string]float64{},
		}, nil
	}
	runCtx, ok := parseBuildingRunContext(item.PayloadRaw)
	if !ok {
		return SettleResult{
			CancelAfterSettle: true,
			CancelReason:      ActionQueueCancelReasonTargetInvalid,
			NextPayload:       item.PayloadRaw,
			ItemCosts:         map[string]int64{},
			ItemRewards:       map[string]int64{},
			AttrExp:           map[string]float64{},
		}, nil
	}
	duration := time.Duration(runtime.DurationMs) * time.Millisecond
	if duration <= 0 {
		baseDurationS, _, _ := buildingLevelPlan(def, runCtx.FromLevel)
		duration = time.Duration(baseDurationS) * time.Second
	}
	if duration <= 0 {
		duration = minimumActionQueueRunDuration
	}
	duration = normalizeActionQueueRunDuration(duration)
	timing := resolveQueueRunTiming(item, now, duration)
	completed, completedAt := completedRunsForQueueTiming(timing, now, item.TotalCount-item.DoneCount)
	if budget.MaxIterations > 0 && completed > budget.MaxIterations {
		completed = budget.MaxIterations
		completedAt = completedAtForQueueTiming(timing, completed)
	}
	if completed <= 0 {
		return SettleResult{}, nil
	}
	currentLevel, err := lockPlayerBuildingLevelTx(ctx, tx, userID, runtime.BuildingID)
	if err != nil {
		return SettleResult{}, err
	}
	maxLevel := maxInt64(1, def.MaxLevel)
	targetLevel := runCtx.ToLevel
	if targetLevel <= currentLevel {
		targetLevel = currentLevel + 1
	}
	if targetLevel > maxLevel {
		targetLevel = maxLevel
	}
	if err := e.store.setPlayerBuildingLevelTx(ctx, tx, userID, runtime.BuildingID, targetLevel); err != nil {
		return SettleResult{}, err
	}
	doneAfter := item.DoneCount + completed
	if doneAfter > item.TotalCount {
		doneAfter = item.TotalCount
	}
	return SettleResult{
		Completed:         completed,
		DoneAfter:         doneAfter,
		CompletedAt:       completedAt,
		NextRunStartAt:    completedAt,
		NextRunEndsAt:     completedAt.Add(timing.NextDuration),
		CancelAfterSettle: false,
		CancelReason:      "",
		NextPayload:       item.PayloadRaw,
		ItemCosts:         map[string]int64{},
		ItemRewards:       map[string]int64{},
		AttrExp:           map[string]float64{},
	}, nil
}

func (e *buildingExecutor) ClassifyStartError(err error) (bool, string) {
	switch {
	case errors.Is(err, ErrInsufficientItem):
		return true, ActionQueueCancelReasonCostInsufficient
	case errors.Is(err, ErrStatusRequireNotMet):
		return true, ActionQueueCancelReasonStatusRequirement
	case errors.Is(err, ErrBuildingRequireNotMet):
		return true, ActionQueueCancelReasonBuildingRequirement
	case errors.Is(err, ErrBuildingNotFound), errors.Is(err, ErrBuildingMaxLevel), errors.Is(err, ErrBuildingArgsInvalid):
		return true, ActionQueueCancelReasonTargetInvalid
	default:
		return false, ""
	}
}

func parseBuildingExecutorRuntime(payloadRaw []byte) (buildingExecutorRuntime, bool) {
	if len(payloadRaw) == 0 {
		return buildingExecutorRuntime{}, false
	}
	var envelope queueRuntimeEnvelope
	if err := json.Unmarshal(payloadRaw, &envelope); err != nil {
		return buildingExecutorRuntime{}, false
	}
	if envelope.ExecutorID != "building_upgrade" {
		return buildingExecutorRuntime{}, false
	}
	var runtime buildingExecutorRuntime
	if err := json.Unmarshal(envelope.Runtime, &runtime); err != nil {
		return buildingExecutorRuntime{}, false
	}
	runtime.BuildingID = strings.TrimSpace(runtime.BuildingID)
	if runtime.BuildingID == "" {
		return buildingExecutorRuntime{}, false
	}
	if runtime.Costs == nil {
		runtime.Costs = map[string]int64{}
	}
	if runtime.StatusRequirement == nil {
		runtime.StatusRequirement = map[string]int64{}
	}
	return runtime, true
}

func parseBuildingRunContext(payloadRaw []byte) (buildingRunContext, bool) {
	if len(payloadRaw) == 0 {
		return buildingRunContext{}, false
	}
	var envelope queueRuntimeEnvelope
	if err := json.Unmarshal(payloadRaw, &envelope); err != nil {
		return buildingRunContext{}, false
	}
	if envelope.ExecutorID != "building_upgrade" || len(envelope.RunContext) == 0 {
		return buildingRunContext{}, false
	}
	var runCtx buildingRunContext
	if err := json.Unmarshal(envelope.RunContext, &runCtx); err != nil {
		return buildingRunContext{}, false
	}
	runCtx.BuildingID = strings.TrimSpace(runCtx.BuildingID)
	if runCtx.BuildingID == "" {
		return buildingRunContext{}, false
	}
	if runCtx.Costs == nil {
		runCtx.Costs = map[string]int64{}
	}
	return runCtx, true
}

func setBuildingRuntimeAndContext(payloadRaw []byte, runtime buildingExecutorRuntime, runCtx buildingRunContext) (json.RawMessage, error) {
	var envelope queueRuntimeEnvelope
	if err := json.Unmarshal(payloadRaw, &envelope); err != nil {
		return nil, err
	}
	runtimeRaw, err := json.Marshal(runtime)
	if err != nil {
		return nil, err
	}
	runCtxRaw, err := json.Marshal(runCtx)
	if err != nil {
		return nil, err
	}
	envelope.Runtime = runtimeRaw
	envelope.RunContext = runCtxRaw
	nextPayload, err := json.Marshal(envelope)
	if err != nil {
		return nil, err
	}
	return nextPayload, nil
}

func lockPlayerBuildingLevelTx(ctx context.Context, tx pgx.Tx, userID int64, buildingID string) (int64, error) {
	normalizedID := strings.TrimSpace(buildingID)
	if normalizedID == "" {
		return 0, ErrBuildingArgsInvalid
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO player_buildings(user_id, building_id, level, updated_at)
		VALUES($1, $2, 0, now())
		ON CONFLICT (user_id, building_id) DO NOTHING
	`, userID, normalizedID); err != nil {
		return 0, err
	}
	var level int64
	if err := tx.QueryRow(ctx, `
		SELECT level
		FROM player_buildings
		WHERE user_id = $1 AND building_id = $2
		FOR UPDATE
	`, userID, normalizedID).Scan(&level); err != nil {
		return 0, err
	}
	if level < 0 {
		level = 0
	}
	return level, nil
}

func buildingCurveDuration(base int64, step int64, level int64) int64 {
	duration := base + step*level
	if duration < 1 {
		return 1
	}
	return duration
}

func buildingCurveMap(base map[string]int64, step map[string]int64, level int64) map[string]int64 {
	out := map[string]int64{}
	for key, baseValue := range base {
		out[key] = baseValue
	}
	for key, stepValue := range step {
		out[key] += stepValue * level
	}
	for key, value := range out {
		if value <= 0 {
			delete(out, key)
			continue
		}
		out[key] = value
	}
	return out
}

func buildingLevelPlan(def *catalog.BuildingDefinition, currentLevel int64) (int64, map[string]int64, map[string]int64) {
	if def != nil {
		toLevel := currentLevel + 1
		for _, level := range def.Levels {
			if level.Level != toLevel {
				continue
			}
			return maxInt64(1, level.DurationS), cloneInt64Map(level.Costs), cloneInt64Map(level.StatusRequirement)
		}
		return buildingCurveDuration(def.BaseDurationS, def.StepDurationS, currentLevel),
			buildingCurveMap(def.BaseCosts, def.StepCosts, currentLevel),
			buildingCurveMap(def.BaseStatusRequirement, def.StepStatusRequirement, currentLevel)
	}
	return 1, map[string]int64{}, map[string]int64{}
}

func buildingLevelBonuses(def *catalog.BuildingDefinition, level int64) map[string]int64 {
	if def == nil || level <= 0 {
		return map[string]int64{}
	}
	for _, entry := range def.Levels {
		if entry.Level == level {
			return cloneInt64Map(entry.Bonuses)
		}
	}
	return map[string]int64{}
}

func cloneInt64Map(source map[string]int64) map[string]int64 {
	out := make(map[string]int64, len(source))
	for key, value := range source {
		if value > 0 {
			out[key] = value
		}
	}
	return out
}

func maxInt64(a int64, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
