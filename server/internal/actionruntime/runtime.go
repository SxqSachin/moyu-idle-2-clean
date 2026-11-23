package actionruntime

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"mysecondwebgame/server/internal/catalog"
	"mysecondwebgame/server/internal/store"
)

type Runtime struct {
	store       *store.Store
	gameData    catalog.Data
	workerSpecs map[string]store.ActionSpec
}

type Result struct {
	Specs                map[string]store.ActionSpec
	Queue                []store.ActionQueueItem
	Schedule             []store.ActionQueueScheduleItem
	State                store.PlayerState
	Attr                 store.AttributeExp
	Summary              store.ActionFastForwardSummary
	BatchDroppedCount    int
	BatchEnqueuedCount   int
	AutoCancelledQueueID int64
	AutoCancelledSource  string
}

type ServiceError struct {
	Code          string
	Message       string
	Debug         string
	InstanceState string
}

type actionBatchStepPayload struct {
	ActionID string          `json:"actionId"`
	Count    int             `json:"count"`
	Args     json.RawMessage `json:"args"`
}

type actionBatchCommandPayload struct {
	Steps      []actionBatchStepPayload `json:"steps"`
	ActorType  string                   `json:"actorType"`
	ActorID    int64                    `json:"actorId"`
	InsertMode string                   `json:"insertMode"`
}

type actionCommandPayload struct {
	ActionID   string          `json:"actionId"`
	QueueID    int64           `json:"queueId"`
	Mode       string          `json:"mode"`
	Count      int             `json:"count"`
	Args       json.RawMessage `json:"args"`
	MoveTarget string          `json:"moveTarget"`
	ActorType  string          `json:"actorType"`
	ActorID    int64           `json:"actorId"`
}

func New(st *store.Store, gameData catalog.Data) *Runtime {
	return &Runtime{store: st, gameData: gameData}
}

func NewWorker(st *store.Store, gameData catalog.Data) *Runtime {
	return &Runtime{
		store:       st,
		gameData:    gameData,
		workerSpecs: store.BuildActionSpecMap(gameData),
	}
}

func (rt *Runtime) ActionSpecMap() map[string]store.ActionSpec {
	return store.BuildActionSpecMap(rt.gameData)
}

func (rt *Runtime) isLevelableAttributeKey(attrKey string) bool {
	key := strings.TrimSpace(attrKey)
	if key == "" {
		return false
	}
	for _, row := range rt.gameData.Attributes {
		if strings.TrimSpace(row.ID) != key {
			continue
		}
		return strings.TrimSpace(row.GrowthType) == "levelable"
	}
	return false
}

func (rt *Runtime) nonLevelableAttrExpKeys(spec store.ActionSpec) []string {
	keys := make([]string, 0)
	seen := map[string]struct{}{}
	for key, value := range spec.StatusExpReward {
		if value == 0 {
			continue
		}
		if rt.isLevelableAttributeKey(key) {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		keys = append(keys, key)
	}
	return keys
}

func ensureBuiltinActionSpec(specs map[string]store.ActionSpec, actionID string) {
	switch actionID {
	case "equipment_enhance":
		if _, ok := specs[actionID]; ok {
			return
		}
		specs[actionID] = store.ActionSpec{
			ID:                  "equipment_enhance",
			MainType:            "equipment",
			Menu:                "equipment",
			DurationS:           1,
			Costs:               map[string]int64{},
			StatusRequirement:   map[string]int64{},
			BuildingRequirement: map[string]int64{},
			ItemReward:          map[string]store.ActionItemRewardSpec{},
			StatusExpReward:     map[string]float64{},
		}
	case "building_upgrade":
		if _, ok := specs[actionID]; ok {
			return
		}
		specs[actionID] = store.ActionSpec{
			ID:                  "building_upgrade",
			MainType:            "building",
			SubType:             "building",
			Menu:                "building",
			DurationS:           1,
			Costs:               map[string]int64{},
			StatusRequirement:   map[string]int64{},
			BuildingRequirement: map[string]int64{},
			ItemReward:          map[string]store.ActionItemRewardSpec{},
			StatusExpReward:     map[string]float64{},
		}
	case store.FishingTripActionID:
		if _, ok := specs[actionID]; ok {
			return
		}
		specs[actionID] = store.ActionSpec{
			ID:                  store.FishingTripActionID,
			MainType:            "fishing",
			SubType:             "fishing_trip",
			Menu:                "fishing",
			DurationS:           30,
			Costs:               map[string]int64{},
			StatusRequirement:   map[string]int64{},
			BuildingRequirement: map[string]int64{},
			ItemReward:          map[string]store.ActionItemRewardSpec{},
			StatusExpReward:     map[string]float64{},
		}
	}
}

func (rt *Runtime) RunAction(ctx context.Context, userID int64, payload json.RawMessage) (*Result, *ServiceError) {
	var req actionCommandPayload
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, &ServiceError{Code: "WS_ACTION_INVALID_PAYLOAD", Message: "invalid payload", Debug: err.Error()}
	}
	req.ActionID = strings.TrimSpace(req.ActionID)
	if req.ActionID == "" {
		return nil, &ServiceError{Code: "WS_ACTION_ID_REQUIRED", Message: "actionId required"}
	}
	if req.Count <= 0 {
		req.Count = 1
	}
	req.Mode = strings.TrimSpace(req.Mode)
	if req.Mode != "run" && req.Mode != "queue" {
		req.Mode = "queue"
	}
	if req.ActionID == "equipment_enhance" && req.Count != 1 {
		return nil, &ServiceError{Code: "WS_ENHANCE_ARGS_INVALID", Message: "enhance count must be 1"}
	}
	if req.ActionID == "building_upgrade" && req.Count != 1 {
		return nil, &ServiceError{Code: "WS_BUILDING_ARGS_INVALID", Message: "building upgrade count must be 1"}
	}
	specs := rt.ActionSpecMap()
	ensureBuiltinActionSpec(specs, req.ActionID)
	spec, ok := specs[req.ActionID]
	if !ok {
		return nil, &ServiceError{Code: "WS_ACTION_NOT_FOUND", Message: "action not found"}
	}
	if invalidKeys := rt.nonLevelableAttrExpKeys(spec); len(invalidKeys) > 0 {
		return nil, &ServiceError{Code: "WS_ACTION_ATTR_EXP_INVALID", Message: "action contains non-levelable attribute exp"}
	}
	queue, state, attr, summary, err := rt.store.RunAction(ctx, userID, specs, req.ActionID, req.Mode, req.Count, req.Args, req.ActorType, req.ActorID)
	if err != nil {
		return nil, mapActionStoreError(err, "run action failed")
	}
	return &Result{Specs: specs, Queue: queue, State: state, Attr: attr, Summary: summary}, nil
}

func (rt *Runtime) EnqueueActionBatch(ctx context.Context, userID int64, payload json.RawMessage) (*Result, *ServiceError) {
	var req actionBatchCommandPayload
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, &ServiceError{Code: "WS_ACTION_INVALID_PAYLOAD", Message: "invalid payload", Debug: err.Error()}
	}
	if len(req.Steps) <= 0 {
		return nil, &ServiceError{Code: "WS_ACTION_INVALID_COUNT", Message: "invalid steps"}
	}
	if len(req.Steps) > 200 {
		return nil, &ServiceError{Code: "WS_ACTION_BATCH_TOO_MANY_STEPS", Message: "too many steps"}
	}
	specs := rt.ActionSpecMap()
	steps := make([]store.ActionBatchStep, 0, len(req.Steps))
	for _, step := range req.Steps {
		step.ActionID = strings.TrimSpace(step.ActionID)
		if step.ActionID == "" {
			return nil, &ServiceError{Code: "WS_ACTION_ID_REQUIRED", Message: "actionId required"}
		}
		if step.Count <= 0 {
			return nil, &ServiceError{Code: "WS_ACTION_INVALID_COUNT", Message: "invalid count"}
		}
		if step.ActionID == "equipment_enhance" && step.Count != 1 {
			return nil, &ServiceError{Code: "WS_ENHANCE_ARGS_INVALID", Message: "enhance count must be 1"}
		}
		if step.ActionID == "building_upgrade" && step.Count != 1 {
			return nil, &ServiceError{Code: "WS_BUILDING_ARGS_INVALID", Message: "building upgrade count must be 1"}
		}
		ensureBuiltinActionSpec(specs, step.ActionID)
		spec, ok := specs[step.ActionID]
		if !ok {
			return nil, &ServiceError{Code: "WS_ACTION_NOT_FOUND", Message: "action not found"}
		}
		if invalidKeys := rt.nonLevelableAttrExpKeys(spec); len(invalidKeys) > 0 {
			return nil, &ServiceError{Code: "WS_ACTION_ATTR_EXP_INVALID", Message: "action contains non-levelable attribute exp"}
		}
		steps = append(steps, store.ActionBatchStep{
			ActionID: step.ActionID,
			Count:    step.Count,
			ArgsRaw:  step.Args,
		})
	}
	queue, state, attr, summary, droppedCount, err := rt.store.EnqueueActionBatch(ctx, userID, specs, steps, req.ActorType, req.ActorID, req.InsertMode)
	if err != nil {
		return nil, mapActionStoreError(err, "enqueue action batch failed")
	}
	return &Result{
		Specs:              specs,
		Queue:              queue,
		State:              state,
		Attr:               attr,
		Summary:            summary,
		BatchDroppedCount:  droppedCount,
		BatchEnqueuedCount: len(steps),
	}, nil
}

func (rt *Runtime) CancelAction(ctx context.Context, userID int64, payload json.RawMessage) (*Result, *ServiceError) {
	var req actionCommandPayload
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, &ServiceError{Code: "WS_ACTION_INVALID_PAYLOAD", Message: "invalid payload", Debug: err.Error()}
	}
	specs := rt.ActionSpecMap()
	var (
		queue []store.ActionQueueItem
		state store.PlayerState
		attr  store.AttributeExp
		err   error
	)
	if req.QueueID > 0 {
		queue, state, attr, err = rt.store.CancelQueueItem(ctx, userID, specs, req.QueueID)
	} else {
		req.ActionID = strings.TrimSpace(req.ActionID)
		if req.ActionID == "" {
			return nil, &ServiceError{Code: "WS_ACTION_ID_REQUIRED", Message: "queueId or actionId required"}
		}
		queue, state, attr, err = rt.store.CancelActionGroup(ctx, userID, specs, req.ActionID, req.ActorType, req.ActorID)
	}
	if err != nil {
		return nil, &ServiceError{Code: "WS_ACTION_CANCEL_FAILED", Message: "cancel action failed", Debug: err.Error()}
	}
	return &Result{Specs: specs, Queue: queue, State: state, Attr: attr}, nil
}

func (rt *Runtime) MoveQueueItem(ctx context.Context, userID int64, payload json.RawMessage) (*Result, *ServiceError) {
	var req actionCommandPayload
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, &ServiceError{Code: "WS_ACTION_INVALID_PAYLOAD", Message: "invalid payload", Debug: err.Error()}
	}
	if req.QueueID <= 0 {
		return nil, &ServiceError{Code: "WS_ACTION_QUEUE_ID_REQUIRED", Message: "queueId required"}
	}
	specs := rt.ActionSpecMap()
	queue, state, attr, summary, err := rt.store.MoveQueueItemWithSummary(ctx, userID, specs, req.QueueID, strings.TrimSpace(req.MoveTarget))
	if err != nil {
		return nil, &ServiceError{Code: "WS_ACTION_QUEUE_MOVE_FAILED", Message: "move queue item failed", Debug: err.Error()}
	}
	result := &Result{Specs: specs, Queue: queue, State: state, Attr: attr, Summary: summary}
	if strings.TrimSpace(req.MoveTarget) == "up" && !containsQueueID(queue, req.QueueID) {
		result.AutoCancelledQueueID = req.QueueID
		result.AutoCancelledSource = "move_up"
	}
	return result, nil
}

func (rt *Runtime) RunQueueItemNow(ctx context.Context, userID int64, payload json.RawMessage) (*Result, *ServiceError) {
	var req actionCommandPayload
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, &ServiceError{Code: "WS_ACTION_INVALID_PAYLOAD", Message: "invalid payload", Debug: err.Error()}
	}
	if req.QueueID <= 0 {
		return nil, &ServiceError{Code: "WS_ACTION_QUEUE_ID_REQUIRED", Message: "queueId required"}
	}
	specs := rt.ActionSpecMap()
	queue, state, attr, summary, err := rt.store.RunQueueItemNowWithSummary(ctx, userID, specs, req.QueueID)
	if err != nil {
		switch {
		case errors.Is(err, store.ErrInsufficientItem):
			return nil, &ServiceError{Code: "WS_ACTION_INSUFFICIENT_ITEM", Message: "insufficient item"}
		case errors.Is(err, store.ErrFishFoodIngredientsInvalid):
			return nil, &ServiceError{Code: "WS_FISH_FOOD_INGREDIENT_INVALID", Message: "fish food ingredient is unavailable"}
		case errors.Is(err, store.ErrStatusRequireNotMet):
			return nil, &ServiceError{Code: "WS_ACTION_STATUS_REQUIREMENT_NOT_MET", Message: "status requirements not met"}
		case errors.Is(err, store.ErrBuildingRequireNotMet):
			return nil, &ServiceError{Code: "WS_ACTION_BUILDING_REQUIREMENT_NOT_MET", Message: "building requirements not met"}
		case errors.Is(err, store.ErrKittyModeConflictLifeQueue):
			return nil, &ServiceError{Code: "WS_ACTION_KITTY_IN_COMBAT", Message: "kitty is in combat"}
		case errors.Is(err, store.ErrKittyModeConflictCombat):
			return nil, &ServiceError{Code: "WS_ACTION_KITTY_IN_LIFE_QUEUE", Message: "kitty is in life queue"}
		case errors.Is(err, store.ErrQueueRunNowNotStarted):
			return nil, &ServiceError{Code: "WS_ACTION_QUEUE_RUN_NOW_FAILED", Message: "run now did not enter running state"}
		default:
			return nil, &ServiceError{Code: "WS_ACTION_QUEUE_RUN_NOW_FAILED", Message: "run queue item now failed", Debug: err.Error()}
		}
	}
	result := &Result{Specs: specs, Queue: queue, State: state, Attr: attr, Summary: summary}
	if !containsQueueID(queue, req.QueueID) {
		result.AutoCancelledQueueID = req.QueueID
		result.AutoCancelledSource = "run_now"
	}
	return result, nil
}

func (rt *Runtime) AdvanceDue(ctx context.Context, userID int64) (*Result, error) {
	specs := rt.ActionSpecMap()
	queue, state, attr, summary, err := rt.store.AdvanceActionQueueWithSummary(ctx, userID, specs)
	if err != nil {
		return nil, err
	}
	return &Result{Specs: specs, Queue: queue, State: state, Attr: attr, Summary: summary}, nil
}

func (rt *Runtime) AdvanceDueForWorker(ctx context.Context, userID int64) (*Result, error) {
	specs := rt.workerSpecs
	if specs == nil {
		specs = rt.ActionSpecMap()
	}
	schedule, summary, err := rt.store.AdvanceActionQueueForWorker(ctx, userID, specs)
	if err != nil {
		return nil, err
	}
	return &Result{Schedule: schedule, Summary: summary}, nil
}

func (rt *Runtime) AdvanceBackgroundOffline(ctx context.Context, userID int64, presence store.PresenceState, start, now time.Time) (time.Time, error) {
	specs := rt.workerSpecs
	if specs == nil {
		specs = rt.ActionSpecMap()
	}
	return rt.store.AdvanceBackgroundActions(ctx, userID, presence, start, now, specs)
}

func (rt *Runtime) FastForwardOffline(ctx context.Context, userID int64, targetAt time.Time) (*Result, error) {
	specs := rt.ActionSpecMap()
	summary, err := rt.store.AdvanceActionQueueOfflineChunk(ctx, userID, specs, targetAt, store.FastForwardBudget{})
	if err != nil {
		return nil, err
	}
	return &Result{Specs: specs, Summary: summary}, nil
}

// LoadSnapshot reads the queue and player-state projections used after a
// committed action mutation. Attribute broadcasts own a richer projection and
// intentionally load it later, so this boundary does not repeat that query.
// Keeping reads separate from FastForwardOffline prevents a failed refresh from
// turning a successful settlement into an apparent failure.
func (rt *Runtime) LoadSnapshot(ctx context.Context, userID int64, specs map[string]store.ActionSpec) (*Result, error) {
	if specs == nil {
		specs = rt.ActionSpecMap()
	}
	queue, err := rt.store.ListActionQueue(ctx, userID, 50)
	if err != nil {
		return nil, err
	}
	state, err := rt.store.GetPlayerState(ctx, userID)
	if err != nil {
		return nil, err
	}
	return &Result{Specs: specs, Queue: queue, State: state}, nil
}

func mapActionStoreError(err error, fallback string) *ServiceError {
	var fishingValidation *store.FishingValidationError
	var fishingSupplyShortage *store.FishingSupplyShortageError
	var fishingTackleNotEquipped *store.FishingTackleNotEquippedError
	switch {
	case errors.Is(err, store.ErrOfflineRecoveryPending):
		return &ServiceError{Code: "OFFLINE_RECOVERY_PENDING", Message: "offline progress is still settling"}
	case errors.As(err, &fishingValidation):
		return &ServiceError{Code: "WS_" + fishingValidation.Code(), Message: fishingValidation.Error()}
	case errors.Is(err, store.ErrActionActorInvalid):
		return &ServiceError{Code: "WS_ACTION_ACTOR_INVALID", Message: "action actor invalid"}
	case errors.Is(err, store.ErrActionActorForbidden):
		return &ServiceError{Code: "WS_ACTION_ACTOR_FORBIDDEN", Message: "action actor forbidden"}
	case errors.Is(err, store.ErrEnhanceArgsInvalid):
		return &ServiceError{Code: "WS_ENHANCE_ARGS_INVALID", Message: "enhance args invalid"}
	case errors.Is(err, store.ErrEnhanceTargetInvalid):
		return &ServiceError{Code: "WS_ENHANCE_TARGET_INVALID", Message: "enhance target invalid"}
	case errors.Is(err, store.ErrEnhanceNotInInventory):
		return &ServiceError{Code: "WS_ENHANCE_NOT_IN_INVENTORY", Message: "enhance target not in inventory"}
	case errors.Is(err, store.ErrEnhanceTemplateInvalid):
		return &ServiceError{Code: "WS_ENHANCE_TEMPLATE_INVALID", Message: "enhance template invalid"}
	case errors.Is(err, store.ErrEnhanceOverflow):
		return &ServiceError{Code: "WS_ENHANCE_OVERFLOW", Message: "enhance overflow"}
	case errors.Is(err, store.ErrAlchemyArgsInvalid):
		return &ServiceError{Code: "WS_ALCHEMY_ARGS_INVALID", Message: "alchemy args invalid"}
	case errors.Is(err, store.ErrFishingInfusionUnavailable):
		return &ServiceError{Code: "WS_ALCHEMY_FISH_UNAVAILABLE", Message: "fish infusion is not available yet"}
	case errors.Is(err, store.ErrAlchemyLevelInsufficient):
		return &ServiceError{Code: "WS_ACTION_STATUS_REQUIREMENT_NOT_MET", Message: "alchemy level requirement not met"}
	case errors.Is(err, store.ErrAlchemyMixedKind), errors.Is(err, store.ErrAlchemyKindMismatch), errors.Is(err, store.ErrAlchemyCarrierFull), errors.Is(err, store.ErrAlchemyMaterialOverflow), errors.Is(err, store.ErrAlchemyGroupNotFound), errors.Is(err, store.ErrAlchemyBasisInvalid):
		return &ServiceError{Code: "WS_ACTION_INVALID_PAYLOAD", Message: "invalid alchemy payload"}
	case errors.Is(err, store.ErrActionMaterialBindingInvalid), errors.Is(err, store.ErrActionMaterialAffixForbidden):
		return &ServiceError{Code: "WS_ACTION_INVALID_PAYLOAD", Message: "invalid payload"}
	case errors.Is(err, store.ErrFishFoodArgsInvalid):
		return &ServiceError{Code: "WS_ACTION_INVALID_PAYLOAD", Message: "invalid fish food payload"}
	case errors.Is(err, store.ErrFishFoodIngredientsInvalid):
		return &ServiceError{Code: "WS_FISH_FOOD_INGREDIENT_INVALID", Message: "fish food ingredient is unavailable"}
	case errors.Is(err, store.ErrActionQueueLaneLimitReached):
		return &ServiceError{Code: "WS_ACTION_QUEUE_LIMIT_REACHED", Message: "action queue lane is full"}
	case errors.Is(err, store.ErrBuildingArgsInvalid):
		return &ServiceError{Code: "WS_BUILDING_ARGS_INVALID", Message: "building args invalid"}
	case errors.Is(err, store.ErrBuildingNotFound):
		return &ServiceError{Code: "WS_BUILDING_NOT_FOUND", Message: "building not found"}
	case errors.Is(err, store.ErrBuildingMaxLevel):
		return &ServiceError{Code: "WS_BUILDING_MAX_LEVEL", Message: "building max level reached"}
	case errors.Is(err, store.ErrFishingArgsInvalid), errors.Is(err, store.ErrFishingLoadoutInvalid):
		return &ServiceError{Code: "WS_FISHING_INVALID", Message: "fishing settings invalid"}
	case errors.Is(err, store.ErrFishingSpotNotFound), errors.Is(err, store.ErrFishingLoadoutNotFound):
		return &ServiceError{Code: "WS_FISHING_NOT_FOUND", Message: "fishing setup not found"}
	case errors.Is(err, store.ErrFishingWaterLocked):
		return &ServiceError{Code: "WS_ACTION_STATUS_REQUIREMENT_NOT_MET", Message: "fishing level requirement not met"}
	case errors.As(err, &fishingSupplyShortage):
		return &ServiceError{Code: "WS_FISHING_SUPPLY_INSUFFICIENT", Message: "fishing supply insufficient", InstanceState: fishingSupplyShortage.ItemID}
	case errors.As(err, &fishingTackleNotEquipped):
		return &ServiceError{Code: "WS_FISHING_TACKLE_NOT_EQUIPPED", Message: "fishing tackle is not equipped", InstanceState: fishingTackleNotEquipped.ItemID}
	case errors.Is(err, store.ErrFishingTackleInUse):
		return &ServiceError{Code: "WS_FISHING_TACKLE_IN_USE", Message: "fishing tackle is in use"}
	case errors.Is(err, store.ErrFishingTackleMissing):
		return &ServiceError{Code: "WS_FISHING_TACKLE_MISSING", Message: "fishing tackle missing"}
	case errors.Is(err, store.ErrFishingTripActive):
		return &ServiceError{Code: "WS_FISHING_TRIP_ACTIVE", Message: "a fishing trip is already planned"}
	case errors.Is(err, store.ErrInsufficientItem):
		return &ServiceError{Code: "WS_ACTION_INSUFFICIENT_ITEM", Message: "insufficient item"}
	case errors.Is(err, store.ErrInvalidActionCount):
		return &ServiceError{Code: "WS_ACTION_INVALID_COUNT", Message: "invalid count"}
	case errors.Is(err, store.ErrStatusRequireNotMet):
		return &ServiceError{Code: "WS_ACTION_STATUS_REQUIREMENT_NOT_MET", Message: "status requirements not met"}
	case errors.Is(err, store.ErrBuildingRequireNotMet):
		return &ServiceError{Code: "WS_ACTION_BUILDING_REQUIREMENT_NOT_MET", Message: "building requirements not met"}
	case errors.Is(err, store.ErrUserBanned):
		return &ServiceError{Code: "WS_ACTION_ACCOUNT_BANNED", Message: "account is banned"}
	case errors.Is(err, store.ErrKittyModeConflictLifeQueue):
		return &ServiceError{Code: "WS_ACTION_KITTY_IN_COMBAT", Message: "kitty is in combat"}
	case errors.Is(err, store.ErrKittyModeConflictCombat):
		return &ServiceError{Code: "WS_ACTION_KITTY_IN_LIFE_QUEUE", Message: "kitty is in life queue"}
	case errors.Is(err, store.ErrQueueRunNowNotStarted):
		return &ServiceError{Code: "WS_ACTION_QUEUE_RUN_NOW_FAILED", Message: "run did not enter running state"}
	default:
		return &ServiceError{Code: "WS_ACTION_FAILED", Message: fallback, Debug: err.Error()}
	}
}

func containsQueueID(items []store.ActionQueueItem, queueID int64) bool {
	for _, item := range items {
		if item.ID == queueID {
			return true
		}
	}
	return false
}
