package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
	"mysecondwebgame/server/internal/actionruntime"
	"mysecondwebgame/server/internal/catalog"
	"mysecondwebgame/server/internal/store"
)

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

const actionCommandTimeout = 3 * time.Second

func (s *Server) actionRuntime() *actionruntime.Runtime {
	return actionruntime.New(s.deps.Store, s.deps.GameData)
}

func (s *Server) handleActionCatalog(c *fiber.Ctx) error {
	defs := s.deps.GameData.ActionDefinitions()
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{
		"items":     defs,
		"mainTypes": s.actionMainTypes(defs),
		"subTypes":  s.actionSubTypes(defs),
	})
}

func (s *Server) actionDefinitionByID(actionID string) (store.ActionSpec, bool) {
	specs := s.actionSpecMap()
	spec, ok := specs[actionID]
	return spec, ok
}

func (s *Server) actionSpecMap() map[string]store.ActionSpec {
	return store.BuildActionSpecMap(s.deps.GameData)
}

func (s *Server) isLevelableAttributeKey(attrKey string) bool {
	key := strings.TrimSpace(attrKey)
	if key == "" {
		return false
	}
	for _, row := range s.deps.GameData.Attributes {
		if strings.TrimSpace(row.ID) != key {
			continue
		}
		return strings.TrimSpace(row.GrowthType) == "levelable"
	}
	return false
}

func (s *Server) nonLevelableAttrExpKeys(spec store.ActionSpec) []string {
	keys := make([]string, 0)
	seen := map[string]struct{}{}
	for key, value := range spec.StatusExpReward {
		if value == 0 {
			continue
		}
		if s.isLevelableAttributeKey(key) {
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

func (s *Server) actionMainTypes(defs []catalog.ActionDefinition) []catalog.ActionMainTypeDefinition {
	if len(s.deps.GameData.ActionMainTypes) > 0 {
		return s.deps.GameData.ActionMainTypes
	}
	orderByType := make(map[string]int64)
	actionsByType := make(map[string][]string)
	for _, def := range defs {
		mainType := def.MainType
		if mainType == "" {
			mainType = def.Menu
		}
		if mainType == "" {
			continue
		}
		if _, ok := orderByType[mainType]; !ok {
			orderByType[mainType] = int64(len(orderByType))
		}
		actionsByType[mainType] = append(actionsByType[mainType], def.ID)
	}
	out := make([]catalog.ActionMainTypeDefinition, 0, len(orderByType))
	for mainType, order := range orderByType {
		out = append(out, catalog.ActionMainTypeDefinition{
			ID:      mainType,
			NameKey: "action_main_type." + mainType + ".name",
			DescKey: "action_main_type." + mainType + ".desc",
			Icon:    "",
			Color:   "#333",
			Order:   order,
			Actions: actionsByType[mainType],
		})
	}
	return out
}

func (s *Server) actionSubTypes(defs []catalog.ActionDefinition) []catalog.ActionSubTypeDefinition {
	if len(s.deps.GameData.ActionSubTypes) > 0 {
		return s.deps.GameData.ActionSubTypes
	}
	orderByType := make(map[string]int64)
	actionsByType := make(map[string][]string)
	for _, def := range defs {
		subType := def.SubType
		if subType == "" {
			subType = "common"
		}
		if _, ok := orderByType[subType]; !ok {
			orderByType[subType] = int64(len(orderByType))
		}
		actionsByType[subType] = append(actionsByType[subType], def.ID)
	}
	out := make([]catalog.ActionSubTypeDefinition, 0, len(orderByType))
	for subType, order := range orderByType {
		out = append(out, catalog.ActionSubTypeDefinition{
			ID:      subType,
			NameKey: "action_sub_type." + subType + ".name",
			DescKey: "action_sub_type." + subType + ".desc",
			Icon:    "",
			Color:   "#333",
			Order:   order,
			Actions: actionsByType[subType],
		})
	}
	return out
}

func (s *Server) runActionCommand(conn *websocket.Conn, userID int64, payload json.RawMessage) {
	ctx, cancel := context.WithTimeout(context.Background(), actionCommandTimeout)
	defer cancel()
	result, actionErr := s.actionRuntime().RunAction(ctx, userID, payload)
	if actionErr != nil {
		_ = s.writeWSError(conn, actionErr.Code, actionErr.Message, actionErr.Debug)
		return
	}
	s.syncActionRuntime(userID, result.Specs, result.Queue, result.State, result.Attr, result.Summary)
	s.pushWorkLoadoutHints(userID, result.Summary)
	s.pushEnhanceAttemptEvents(userID, result.Summary.EnhanceResults)
}

func actionErrorHTTPStatus(code string) int {
	if strings.HasPrefix(strings.TrimSpace(code), "WS_FISHING_SETTINGS_") {
		return fiber.StatusBadRequest
	}
	switch strings.TrimSpace(code) {
	case "WS_ACTION_NOT_FOUND", "WS_BUILDING_NOT_FOUND", "WS_FISHING_NOT_FOUND":
		return fiber.StatusNotFound
	case "WS_ACTION_ACTOR_FORBIDDEN":
		return fiber.StatusForbidden
	case "WS_ALCHEMY_FISH_UNAVAILABLE":
		return fiber.StatusLocked
	case "WS_ACTION_INSUFFICIENT_ITEM",
		"WS_FISHING_SUPPLY_INSUFFICIENT",
		"WS_FISHING_TACKLE_NOT_EQUIPPED",
		"WS_FISHING_TACKLE_IN_USE",
		"WS_FISHING_TACKLE_MISSING",
		"WS_ACTION_STATUS_REQUIREMENT_NOT_MET",
		"WS_ACTION_BUILDING_REQUIREMENT_NOT_MET",
		"WS_FISH_FOOD_INGREDIENT_INVALID",
		"WS_BUILDING_MAX_LEVEL",
		"WS_ACTION_QUEUE_LIMIT_REACHED",
		"WS_FISHING_TRIP_ACTIVE":
		return fiber.StatusConflict
	case "WS_ACTION_INVALID_PAYLOAD",
		"WS_ACTION_ID_REQUIRED",
		"WS_ACTION_INVALID_COUNT",
		"WS_ACTION_BATCH_TOO_MANY_STEPS",
		"WS_BUILDING_ARGS_INVALID",
		"WS_ALCHEMY_ARGS_INVALID",
		"WS_FISHING_INVALID",
		"WS_ENHANCE_ARGS_INVALID",
		"WS_ACTION_ACTOR_INVALID":
		return fiber.StatusBadRequest
	default:
		return fiber.StatusInternalServerError
	}
}

func (s *Server) handleActionQueueBatchEnqueue(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	ctx, cancel := context.WithTimeout(c.Context(), actionCommandTimeout)
	defer cancel()
	result, actionErr := s.actionRuntime().EnqueueActionBatch(ctx, userID, c.Body())
	if actionErr != nil {
		return s.writeAPIError(c, actionErrorHTTPStatus(actionErr.Code), actionErr.Code, actionErr.Message, actionErr.Debug, actionErr.InstanceState)
	}
	s.syncActionRuntime(userID, result.Specs, result.Queue, result.State, result.Attr, result.Summary)
	s.pushWorkLoadoutHints(userID, result.Summary)
	s.pushEnhanceAttemptEvents(userID, result.Summary.EnhanceResults)
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{
		"ok":            true,
		"enqueuedCount": result.BatchEnqueuedCount,
		"droppedCount":  result.BatchDroppedCount,
	})
}

func (s *Server) cancelActionCommand(conn *websocket.Conn, userID int64, payload json.RawMessage) {
	ctx, cancel := context.WithTimeout(context.Background(), actionCommandTimeout)
	defer cancel()
	result, actionErr := s.actionRuntime().CancelAction(ctx, userID, payload)
	if actionErr != nil {
		_ = s.writeWSError(conn, actionErr.Code, actionErr.Message, actionErr.Debug)
		return
	}
	result.Summary.FishingChanged = true
	s.syncActionRuntime(userID, result.Specs, result.Queue, result.State, result.Attr, result.Summary)
	s.pushWorkLoadoutHints(userID, result.Summary)
}

func (s *Server) moveQueueItemCommand(conn *websocket.Conn, userID int64, payload json.RawMessage) {
	ctx, cancel := context.WithTimeout(context.Background(), actionCommandTimeout)
	defer cancel()
	result, actionErr := s.actionRuntime().MoveQueueItem(ctx, userID, payload)
	if actionErr != nil {
		_ = s.writeWSError(conn, actionErr.Code, actionErr.Message, actionErr.Debug)
		return
	}
	s.syncActionRuntime(userID, result.Specs, result.Queue, result.State, result.Attr, result.Summary)
	s.pushWorkLoadoutHints(userID, result.Summary)
	s.pushQueueAutoCancelledHint(userID, result)
}

func (s *Server) runQueueItemNowCommand(conn *websocket.Conn, userID int64, payload json.RawMessage) {
	ctx, cancel := context.WithTimeout(context.Background(), actionCommandTimeout)
	defer cancel()
	result, actionErr := s.actionRuntime().RunQueueItemNow(ctx, userID, payload)
	if actionErr != nil {
		_ = s.writeWSError(conn, actionErr.Code, actionErr.Message, actionErr.Debug)
		return
	}
	s.syncActionRuntime(userID, result.Specs, result.Queue, result.State, result.Attr, result.Summary)
	s.pushWorkLoadoutHints(userID, result.Summary)
	s.pushQueueAutoCancelledHint(userID, result)
	s.pushEnhanceAttemptEvents(userID, result.Summary.EnhanceResults)
}

func (s *Server) stopActionPush(userID int64) {
	if userID <= 0 || s.deps.Redis == nil {
		return
	}
	_ = s.deps.Redis.ZRem(context.Background(), actionQueueDueKey(), strconv.FormatInt(userID, 10)).Err()
}

func (s *Server) syncActionRuntime(userID int64, specs map[string]store.ActionSpec, queue []store.ActionQueueItem, state store.PlayerState, _ store.AttributeExp, summary store.ActionFastForwardSummary) {
	s.pushUserEventClusterOnce(userID, "action.queue.updated", fiber.Map{"items": queue})
	s.pushPlayerAttributes(userID)
	if actionruntime.EquipmentChanged(summary) {
		s.pushEquipmentDomain(userID)
	}
	s.pushUserEventClusterOnce(userID, "game.state.patch", state)
	s.pushWarehouseItems(userID)
	if summary.FishingChanged || actionQueueContainsFishing(queue) {
		s.pushFishingState(userID)
	}
	if summary.OperationEffectsChanged {
		s.pushOperationEffectsSnapshot(userID)
	}
	if err := s.syncActionQueueKittyCombatProjection(context.Background(), userID, state.Kitties); err != nil {
		s.rebuildActionQueueKittyCombatProjection(context.Background(), userID)
	}
	s.syncKittyActionAttributes(userID, summary.ActorSummaries)
	s.scheduleActionPush(userID, specs, queue)
}

func (s *Server) syncKittyActionAttributes(userID int64, summaries []store.ActionActorSummary) {
	attributeKittyIDs := make([]int64, 0, len(summaries))
	equipmentKittyIDs := make([]int64, 0, len(summaries))
	for _, summary := range summaries {
		if summary.ActorType != store.ActionActorTypeKitty || summary.ActorID <= 0 {
			continue
		}
		if summary.EquipmentChanged {
			equipmentKittyIDs = append(equipmentKittyIDs, summary.ActorID)
			continue
		}
		if len(summary.AttrExp) > 0 {
			attributeKittyIDs = append(attributeKittyIDs, summary.ActorID)
		}
	}
	equipmentKittyIDs = compactInt64s(equipmentKittyIDs)
	if len(equipmentKittyIDs) > 0 {
		s.syncAfterMutation(mutationSyncScope{
			UserIDs:         []int64{userID},
			KittyIDs:        equipmentKittyIDs,
			KittyAttributes: true,
			KittyEquipment:  true,
		})
	}
	attributeKittyIDs = compactInt64s(attributeKittyIDs)
	if len(attributeKittyIDs) > 0 {
		s.syncAfterMutation(mutationSyncScope{
			UserIDs:         []int64{userID},
			KittyIDs:        attributeKittyIDs,
			KittyAttributes: true,
		})
	}
}

func actionQueueContainsFishing(queue []store.ActionQueueItem) bool {
	for _, item := range queue {
		if item.ActionID == store.FishingTripActionID {
			return true
		}
	}
	return false
}

func (s *Server) pushEnhanceAttemptEvents(userID int64, rows []store.EnhanceAttemptResult) {
	if len(rows) == 0 {
		return
	}
	for _, row := range rows {
		s.pushUserEventCluster(userID, "equipment.enhance.result", row)
	}
}

func (s *Server) pushWorkLoadoutHints(userID int64, summary store.ActionFastForwardSummary) {
	hasActivated := len(summary.ActivatedWorkLoadouts) > 0
	hasFailed := false
	for _, u := range summary.WorkLoadoutUsages {
		if !u.Applied && !u.Offline && strings.TrimSpace(u.FallbackReason) != "" && strings.TrimSpace(u.FallbackReason) != store.WorkLoadoutFallbackNotBound {
			hasFailed = true
			break
		}
	}
	if userID <= 0 || (!hasActivated && !hasFailed) {
		return
	}
	loadouts, err := s.deps.Store.GetPlayerLoadouts(context.Background(), userID)
	if err != nil {
		return
	}
	nameByID := make(map[string]string, len(loadouts.WorkLoadouts))
	for _, row := range loadouts.WorkLoadouts {
		id := strings.TrimSpace(row.ID)
		if id == "" {
			continue
		}
		nameByID[strings.ToLower(id)] = strings.TrimSpace(row.Name)
	}
	seen := map[string]struct{}{}
	for _, usage := range summary.ActivatedWorkLoadouts {
		loadoutID := strings.TrimSpace(usage.LoadoutID)
		if loadoutID == "" {
			continue
		}
		key := strings.ToLower(loadoutID)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		name := strings.TrimSpace(nameByID[key])
		if name == "" {
			name = loadoutID
		}
		s.pushUserEventCluster(userID, "player.work_loadout.activated", fiber.Map{
			"loadoutId":   loadoutID,
			"loadoutName": name,
			"actionId":    strings.TrimSpace(usage.ActionID),
		})
	}
	seenFailed := map[string]struct{}{}
	for _, usage := range summary.WorkLoadoutUsages {
		if usage.Applied || usage.Offline {
			continue
		}
		reason := strings.TrimSpace(usage.FallbackReason)
		if reason == "" || reason == store.WorkLoadoutFallbackNotBound {
			continue
		}
		loadoutID := strings.TrimSpace(usage.LoadoutID)
		if loadoutID == "" {
			continue
		}
		failKey := strings.ToLower(loadoutID) + ":" + strings.ToLower(usage.ActionID)
		if _, ok := seenFailed[failKey]; ok {
			continue
		}
		seenFailed[failKey] = struct{}{}
		name := strings.TrimSpace(nameByID[strings.ToLower(loadoutID)])
		if name == "" {
			name = loadoutID
		}
		s.pushUserEventCluster(userID, "player.work_loadout.failed", fiber.Map{
			"loadoutId":   loadoutID,
			"loadoutName": name,
			"actionId":    strings.TrimSpace(usage.ActionID),
			"reason":      reason,
		})
	}
}

func (s *Server) pushUserEventClusterOnce(userID int64, eventType string, payload any) {
	if userID <= 0 || strings.TrimSpace(eventType) == "" {
		return
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return
	}
	key := fmt.Sprintf("%d:%s", userID, eventType)
	now := time.Now()
	s.eventDedupMu.Lock()
	if now.Sub(s.eventDedupLastSweep) >= eventDedupSweepInterval || len(s.eventDedup) > eventDedupMaxEntries {
		s.sweepEventDedupLocked(now)
		s.eventDedupLastSweep = now
	}
	if last, ok := s.eventDedup[key]; ok && last.Payload == string(raw) {
		s.eventDedupMu.Unlock()
		return
	}
	s.eventDedup[key] = eventDedupEntry{
		Payload:   string(raw),
		UpdatedAt: now,
	}
	s.eventDedupMu.Unlock()
	s.maybeSweepPresenceAndPatchCaches(now)
	s.publishWSBroadcast(eventType, userID, payload)
}

func (s *Server) pushPlayerAttributes(userID int64) {
	seeds, err := s.playerAttributeSeeds()
	if err != nil {
		log.Printf("push player attributes failed to load seeds user_id=%d err=%v", userID, err)
		return
	}
	if err = s.deps.Store.EnsurePlayerAttributes(context.Background(), userID, seeds); err != nil {
		log.Printf("push player attributes failed to ensure attributes user_id=%d err=%v", userID, err)
		return
	}
	states, err := s.deps.Store.ListPlayerAttributes(context.Background(), userID, seeds)
	if err != nil {
		log.Printf("push player attributes failed to list attributes user_id=%d err=%v", userID, err)
		return
	}
	progress, err := s.attributeProgressMap(states)
	if err != nil {
		log.Printf("push player attributes failed to build progress user_id=%d err=%v", userID, err)
		return
	}
	s.pushPlayerAttributesPatch(userID, s.attributeMapFromStates(states), progress)
}

func (s *Server) pushKittyAttributes(userID int64, kittyID int64) {
	seeds, err := s.playerAttributeSeeds()
	if err != nil {
		log.Printf("push kitty attributes failed to load seeds user_id=%d kitty_id=%d err=%v", userID, kittyID, err)
		return
	}
	if err = s.deps.Store.EnsureKittyAttributes(context.Background(), userID, kittyID, seeds); err != nil {
		log.Printf("push kitty attributes failed to ensure attributes user_id=%d kitty_id=%d err=%v", userID, kittyID, err)
		return
	}
	if err = s.deps.Store.RecomputeKittyDerivedStats(context.Background(), userID, kittyID, seeds); err != nil {
		log.Printf("push kitty attributes failed to recompute stats user_id=%d kitty_id=%d err=%v", userID, kittyID, err)
		return
	}
	states, err := s.deps.Store.ListKittyAttributes(context.Background(), userID, kittyID, seeds)
	if err != nil {
		log.Printf("push kitty attributes failed to list attributes user_id=%d kitty_id=%d err=%v", userID, kittyID, err)
		return
	}
	progress, err := s.attributeProgressMap(states)
	if err != nil {
		log.Printf("push kitty attributes failed to build progress user_id=%d kitty_id=%d err=%v", userID, kittyID, err)
		return
	}
	s.pushKittyAttributesPatch(userID, kittyID, s.attributeMapFromStates(states), progress)
}

func (s *Server) pushKittyAttributesNow(ctx context.Context, userID int64, kittyID int64) (fiber.Map, bool, error) {
	seeds, err := s.playerAttributeSeeds()
	if err != nil {
		return nil, false, err
	}
	if err = s.deps.Store.EnsureKittyAttributes(ctx, userID, kittyID, seeds); err != nil {
		return nil, false, err
	}
	if err = s.deps.Store.RecomputeKittyDerivedStats(ctx, userID, kittyID, seeds); err != nil {
		return nil, false, err
	}
	states, err := s.deps.Store.ListKittyAttributes(ctx, userID, kittyID, seeds)
	if err != nil {
		return nil, false, err
	}
	progress, err := s.attributeProgressMap(states)
	if err != nil {
		return nil, false, err
	}
	payload, changed := s.pushKittyAttributesPatch(userID, kittyID, s.attributeMapFromStates(states), progress)
	return payload, changed, nil
}

func (s *Server) pushKittyEquipment(userID int64, kittyID int64) {
	items, err := s.deps.Store.ListKittyEquippedItems(context.Background(), userID, kittyID)
	if err != nil {
		log.Printf("push kitty equipment failed user_id=%d kitty_id=%d err=%v", userID, kittyID, err)
		return
	}
	s.pushKittyEquipmentPatch(userID, kittyID, items)
}

func (s *Server) pushKittyEquipmentNow(ctx context.Context, userID int64, kittyID int64) (fiber.Map, bool, error) {
	items, err := s.deps.Store.ListKittyEquippedItems(ctx, userID, kittyID)
	if err != nil {
		return nil, false, err
	}
	// Mutation responses must be authoritative even when this process has no
	// cached kitty baseline and therefore cannot derive removals or replacements.
	payload := s.pushKittyEquipmentSnapshot(userID, kittyID, items)
	return payload, payload != nil, nil
}

func (s *Server) pushRealtimeInit(userID int64) {
	s.pushActionQueueSnapshot(userID)
	s.pushBattleRoomSelfSnapshot(userID)
	s.pushPlayerAttributes(userID)
	s.pushEquipmentDomainSnapshot(userID)
	s.pushWarehouseItems(userID)
	s.pushOperationEffectsSnapshot(userID)
}

func (s *Server) pushOperationEffectsSnapshot(userID int64) {
	projection, err := s.deps.Store.ListOperationEffects(context.Background(), userID, time.Now())
	if err != nil {
		log.Printf("push operation effects failed user_id=%d err=%v", userID, err)
		return
	}
	s.pushUserEventCluster(userID, "operation.effects.updated", projection)
}

func (s *Server) pushActionQueueSnapshot(userID int64) {
	specs := s.actionSpecMap()
	queue, err := s.deps.Store.ListActionQueue(context.Background(), userID, 50)
	if err != nil {
		log.Printf("push action queue snapshot failed user_id=%d err=%v", userID, err)
		return
	}
	s.rebuildActionQueueKittyCombatProjection(context.Background(), userID)
	s.pushUserEventCluster(userID, "action.queue.updated", fiber.Map{"items": queue})
	s.scheduleActionPush(userID, specs, queue)
}

func (s *Server) pushQueueAutoCancelledHint(userID int64, result *actionruntime.Result) {
	if result == nil || result.AutoCancelledQueueID <= 0 {
		return
	}
	s.pushUserEventCluster(userID, "action.queue.auto_cancelled", fiber.Map{
		"queueId": result.AutoCancelledQueueID,
		"source":  result.AutoCancelledSource,
	})
}

func (s *Server) unmetStatusRequire(ctx context.Context, userID int64, require map[string]int64) ([]string, error) {
	if len(require) == 0 {
		return nil, nil
	}
	seeds, err := s.playerAttributeSeeds()
	if err != nil {
		return nil, err
	}
	if err := s.deps.Store.EnsurePlayerAttributes(ctx, userID, seeds); err != nil {
		return nil, err
	}
	states, err := s.deps.Store.ListPlayerAttributes(ctx, userID, seeds)
	if err != nil {
		return nil, err
	}
	attrMap := make(map[string]float64, len(states))
	for _, state := range states {
		key := strings.TrimSpace(state.AttrKey)
		if key == "" {
			continue
		}
		attrMap[key] = state.FinalValue
	}
	unmet := make([]string, 0)
	for key, minRequired := range require {
		current := attrMap[key]
		if current < float64(minRequired) {
			unmet = append(unmet, key)
		}
	}
	return unmet, nil
}
