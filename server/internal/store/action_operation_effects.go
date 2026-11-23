package store

import (
	"context"
	"encoding/json"
	"errors"
	"math/rand"
	"sort"
	"strconv"
	"strings"
	"time"

	"mysecondwebgame/server/internal/operationeffect"
	alchemyeffect "mysecondwebgame/server/internal/operationeffect/alchemy"
	"mysecondwebgame/server/internal/operationeffect/lifeaction"

	"github.com/jackc/pgx/v5"
)

// operationEffectAwareExecutor is opt-in: existing and future executors remain
// unaffected until their real atomic operation boundary is explicitly adapted.
type operationEffectAwareExecutor interface {
	OperationEffectDomain() operationeffect.Domain
}

type operationEffectContextEnricher interface {
	EnrichOperationEffectContext(operationeffect.Operation, []byte) operationeffect.Operation
}

// Optional authoritative eligibility checks run only at a real start, never
// during pure previews or fast-forward probes.
type operationEffectContextTxEnricher interface {
	EnrichOperationEffectContextTx(context.Context, pgx.Tx, operationeffect.Operation, []byte) (operationeffect.Operation, error)
}

func enrichOperationEffectContext(executor QueueActionExecutor, op operationeffect.Operation, payload []byte) operationeffect.Operation {
	if enricher, ok := executor.(operationEffectContextEnricher); ok {
		return enricher.EnrichOperationEffectContext(op, payload)
	}
	return op
}

// operationEffectPerStartExecutor opts out of bulk settlement only while a
// relevant effect may cross a logical start. Effect-free queues retain their
// existing fast path.
type operationEffectPerStartExecutor interface {
	OperationEffectsRequirePerStartSettlement() bool
}

func executorOperationEffectDomain(executor QueueActionExecutor) operationeffect.Domain {
	aware, ok := executor.(operationEffectAwareExecutor)
	if !ok {
		return operationeffect.DomainNone
	}
	return aware.OperationEffectDomain()
}

func (e *genericExecutor) OperationEffectDomain() operationeffect.Domain {
	return operationeffect.DomainLifeAction
}

func prepareGenericOperationEffectSnapshots(item QueueRuntimeItem, timing queueRunTiming, settledAt time.Time, completed, startsToCharge, doneAfter int, cancelAfterSettle bool) ([]operationeffect.Snapshot, json.RawMessage, error) {
	if completed <= 0 {
		return nil, item.PayloadRaw, nil
	}
	current, hasCurrentSnapshot := queueOperationEffectSnapshot(item.PayloadRaw)
	if !item.EffectSession.HasGrants() && (!hasCurrentSnapshot || len(current.Rules) == 0) {
		return nil, item.PayloadRaw, nil
	}
	snapshots := make([]operationeffect.Snapshot, completed)
	if hasCurrentSnapshot {
		snapshots[0] = current
	} else {
		// Missing means the operation predates this system. Freeze it as empty;
		// never resolve a newly eaten effect at settlement time.
		snapshots[0] = operationeffect.EmptySnapshot(queueOperationEffectOperation(operationeffect.DomainLifeAction, item, timing.StartAt, item.DoneCount+1, item.PayloadRaw))
	}
	nextPayload := json.RawMessage(item.PayloadRaw)
	operationTemplate := queueOperationEffectOperation(operationeffect.DomainLifeAction, item, timing.StartAt, item.DoneCount+1, item.PayloadRaw)
	for index := 0; index < startsToCharge; index++ {
		startedAt := operationEffectSubsequentStartAt(item, timing, settledAt, index+1)
		operation := operationTemplate
		operation.StartedAt = startedAt
		if operation.Domain != operationeffect.DomainNone {
			operation.Key = queueOperationEffectKey(item.QueueID, item.DoneCount+2+index, startedAt)
		}
		snapshot := operationeffect.EmptySnapshot(operation)
		if item.EffectSession != nil {
			var err error
			snapshot, err = item.EffectSession.Commit(item.EffectSession.Prepare(operation))
			if err != nil {
				return nil, nil, err
			}
		}
		if index+1 < completed {
			snapshots[index+1] = snapshot
		}
		if index == startsToCharge-1 && doneAfter < item.TotalCount && !cancelAfterSettle {
			var err error
			nextPayload, err = setQueueOperationEffectSnapshot(nextPayload, snapshot)
			if err != nil {
				return nil, nil, err
			}
		}
	}
	return snapshots, nextPayload, nil
}

func operationEffectSubsequentStartAt(item QueueRuntimeItem, timing queueRunTiming, settledAt time.Time, completed int) time.Time {
	if !item.PreserveScheduledTimeline && completed == 1 && !settledAt.IsZero() {
		return settledAt
	}
	return completedAtForQueueTiming(timing, completed)
}

func (e *genericExecutor) applyLifeActionOperationEffectsTx(ctx context.Context, tx pgx.Tx, userID int64, item QueueRuntimeItem, snapshots []operationeffect.Snapshot, phase string) (map[string]int64, error) {
	return e.applyLifeActionOperationEffectsWithMainOutputGrantTx(ctx, tx, userID, item, snapshots, phase, nil)
}

type lifeActionMainOutputGrant func(operationeffect.Snapshot, lifeaction.ExtraOutput) error

func (e *genericExecutor) applyLifeActionOperationEffectsWithMainOutputGrantTx(
	ctx context.Context,
	tx pgx.Tx,
	userID int64,
	item QueueRuntimeItem,
	snapshots []operationeffect.Snapshot,
	phase string,
	customMainOutputGrant lifeActionMainOutputGrant,
) (map[string]int64, error) {
	rewards := make(map[string]int64)
	type batchKey struct {
		rewardType   string
		grantID      int64
		definitionID string
		itemID       string
	}
	type batchValue struct {
		quantity          int64
		triggered         int64
		firstOperationKey string
		lastOperationKey  string
	}
	type affixGrant struct {
		snapshot operationeffect.Snapshot
		output   lifeaction.ExtraOutput
		spec     ActionItemRewardSpec
	}
	batches := make(map[batchKey]batchValue)
	affixGrants := make([]affixGrant, 0)
	addResolvedReward := func(itemID string, quantity int64) error {
		if quantity <= 0 {
			return nil
		}
		if rewards[itemID] > int64(1<<63-1)-quantity {
			return errors.New("operation effect reward total overflow")
		}
		rewards[itemID] += quantity
		return nil
	}
	addBatch := func(key batchKey, quantity int64, operationKey string) error {
		if quantity <= 0 {
			return nil
		}
		value := batches[key]
		if value.quantity > int64(1<<63-1)-quantity {
			return errors.New("operation effect reward quantity overflow")
		}
		if err := addResolvedReward(key.itemID, quantity); err != nil {
			return err
		}
		value.quantity += quantity
		value.triggered++
		if value.firstOperationKey == "" {
			value.firstOperationKey = operationKey
		}
		value.lastOperationKey = operationKey
		batches[key] = value
		return nil
	}
	resolver := lifeaction.Resolver{}
	state, stateErr := ordinaryState(item.PayloadRaw)
	if stateErr != nil {
		return nil, stateErr
	}
	ordinal := item.DoneCount
	if state != nil {
		random := state.random()
		resolver.Sample = func(rule operationeffect.RuleSnapshot, p float64) bool {
			return random.Hit(ordinaryEffectStream(rule.GrantID, rule.Kind), int64(ordinal), p)
		}
	}
	for index, snapshot := range snapshots {
		ordinal = item.DoneCount + index
		if len(snapshot.Rules) == 0 {
			continue
		}
		outcome, err := resolver.Resolve(snapshot, item.Spec.Costs, item.Spec.MainOutput)
		if err != nil {
			return nil, err
		}
		for _, refund := range outcome.Refunds {
			for itemID, quantity := range refund.Items {
				if err := addBatch(batchKey{
					rewardType:   "operation_effect_material_refund",
					grantID:      refund.GrantID,
					definitionID: refund.DefinitionID,
					itemID:       itemID,
				}, quantity, snapshot.OperationKey); err != nil {
					return nil, err
				}
			}
		}
		for _, extra := range outcome.ExtraOutputs {
			if extra.Quantity <= 0 || strings.TrimSpace(extra.ItemID) == "" {
				continue
			}
			if customMainOutputGrant != nil && strings.TrimSpace(extra.ItemID) == strings.TrimSpace(item.Spec.MainOutput) {
				if err := addResolvedReward(extra.ItemID, extra.Quantity); err != nil {
					return nil, err
				}
				if err := customMainOutputGrant(snapshot, extra); err != nil {
					return nil, err
				}
				continue
			}
			rewardSpec := item.Spec.ItemReward[extra.ItemID]
			if strings.TrimSpace(rewardSpec.GrantPolicy) == InventoryGrantPolicyRollDefaultAffixes {
				if err := addResolvedReward(extra.ItemID, extra.Quantity); err != nil {
					return nil, err
				}
				affixGrants = append(affixGrants, affixGrant{snapshot: snapshot, output: extra, spec: rewardSpec})
				continue
			}
			if err := addBatch(batchKey{
				rewardType:   "operation_effect_extra_main_output",
				grantID:      extra.GrantID,
				definitionID: extra.DefinitionID,
				itemID:       extra.ItemID,
			}, extra.Quantity, snapshot.OperationKey); err != nil {
				return nil, err
			}
		}
	}
	keys := make([]batchKey, 0, len(batches))
	for key := range batches {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].rewardType != keys[j].rewardType {
			return keys[i].rewardType < keys[j].rewardType
		}
		if keys[i].definitionID != keys[j].definitionID {
			return keys[i].definitionID < keys[j].definitionID
		}
		if keys[i].grantID != keys[j].grantID {
			return keys[i].grantID < keys[j].grantID
		}
		return keys[i].itemID < keys[j].itemID
	})
	for _, key := range keys {
		batch := batches[key]
		operationRangeKey := batch.firstOperationKey
		if batch.lastOperationKey != batch.firstOperationKey {
			operationRangeKey += ".." + batch.lastOperationKey
		}
		details := map[string]any{
			"type":                key.rewardType,
			"queueId":             item.QueueID,
			"grantId":             key.grantID,
			"phase":               phase,
			"triggeredOperations": batch.triggered,
			"firstOperationKey":   batch.firstOperationKey,
			"lastOperationKey":    batch.lastOperationKey,
			"operationRangeKey":   operationRangeKey,
		}
		if batch.triggered == 1 {
			details["operationKey"] = batch.firstOperationKey
		}
		direction := InventoryAuditDirectionGain
		if key.rewardType == "operation_effect_material_refund" {
			direction = InventoryAuditDirectionRefund
		}
		if err := e.store.LedgerGrantTemplateTx(ctx, tx, userID, key.itemID, batch.quantity, InventoryLedgerMeta{
			Category:      InventoryAuditCategoryOperationEffect,
			Direction:     direction,
			SubjectType:   "operation_effect",
			SubjectID:     key.definitionID,
			SourceSurface: "action_queue",
			TargetSurface: "inventory",
			Details:       details,
		}); err != nil {
			return nil, err
		}
	}
	for _, grant := range affixGrants {
		extra := grant.output
		rng := rand.New(rand.NewSource(operationeffect.DeterministicSeed(grant.snapshot.OperationKey, extra.GrantID, lifeaction.KindExtraMainOutputChance, "grant")))
		if err := e.store.LedgerGrantTemplateWithOptionsTx(ctx, tx, userID, extra.ItemID, extra.Quantity, InventoryLedgerMeta{
			Category:      InventoryAuditCategoryOperationEffect,
			Direction:     InventoryAuditDirectionGain,
			SubjectType:   "operation_effect",
			SubjectID:     extra.DefinitionID,
			SourceSurface: "action_queue",
			TargetSurface: "inventory",
			Details: map[string]any{
				"type":         "operation_effect_extra_main_output",
				"queueId":      item.QueueID,
				"operationKey": grant.snapshot.OperationKey,
				"grantId":      extra.GrantID,
				"phase":        phase,
			},
		}, inventoryGrantOptionsFromActionReward(grant.spec, rng)); err != nil {
			return nil, err
		}
	}
	return rewards, nil
}

func queueOperationEffectSnapshot(payloadRaw []byte) (operationeffect.Snapshot, bool) {
	if len(payloadRaw) == 0 {
		return operationeffect.Snapshot{}, false
	}
	var envelope queueRuntimeEnvelope
	if err := json.Unmarshal(payloadRaw, &envelope); err != nil || envelope.OperationEffects == nil {
		return operationeffect.Snapshot{}, false
	}
	if envelope.OperationEffects.SchemaVersion != operationeffect.SnapshotSchemaVersion || strings.TrimSpace(envelope.OperationEffects.OperationKey) == "" {
		return operationeffect.Snapshot{}, false
	}
	return *envelope.OperationEffects, true
}

func setQueueOperationEffectSnapshot(payloadRaw []byte, snapshot operationeffect.Snapshot) (json.RawMessage, error) {
	var envelope queueRuntimeEnvelope
	if err := json.Unmarshal(payloadRaw, &envelope); err != nil {
		return nil, err
	}
	if len(snapshot.Rules) == 0 {
		envelope.OperationEffects = nil
	} else {
		envelope.OperationEffects = &snapshot
	}
	return json.Marshal(envelope)
}

func queueOperationEffectOperation(domain operationeffect.Domain, item QueueRuntimeItem, startedAt time.Time, ordinal int, payloadRaw []byte) operationeffect.Operation {
	actorKind := operationeffect.ActorPlayer
	actorID := int64(0)
	if normalizeActorType(item.ActorType) == ActionActorTypeKitty {
		actorKind = operationeffect.ActorKitty
		actorID = item.ActorID
	}
	capabilities := make(map[string]bool)
	if domain == operationeffect.DomainLifeAction {
		runtime, isGeneric := parseGenericExecutorRuntimeEnvelope(payloadRaw)
		_, isFishFood := parseFishFoodExecutorRuntime(payloadRaw)
		// Food cannot amplify its own production, and instance-based upgrades
		// must not clone their inherited equipment or custom ingredient data.
		if isFishFood || (isGeneric && runtime.UseInstanceBindings) {
			return operationeffect.Operation{Key: "ineligible", Domain: operationeffect.DomainNone, StartedAt: startedAt}
		}
		if len(item.Spec.Costs) > 0 && ((isGeneric && !runtime.UseInstanceBindings) || isFishFood) {
			capabilities[lifeaction.CapabilityTemplateCosts] = true
		}
		mainOutput := strings.TrimSpace(item.Spec.MainOutput)
		if mainOutput != "" {
			if _, exists := item.Spec.ItemReward[mainOutput]; exists {
				capabilities[lifeaction.CapabilityMainOutput] = true
			}
		}
	} else if domain == operationeffect.DomainAlchemy && strings.TrimSpace(item.ActionID) == "alchemy.bench" {
		var args alchemyBenchArgs
		if err := parseAlchemyRunContext(payloadRaw, &args); err == nil && args.FishInstanceID <= 0 && len(args.Materials) > 0 {
			capabilities[alchemyeffect.CapabilityInfusion] = true
		}
	}
	return operationeffect.Operation{
		// Include the logical start instant so interrupting and later restarting
		// the same queue ordinal is a new chargeable operation, while retries of
		// an already-persisted run keep using its frozen snapshot.
		Key:          queueOperationEffectKey(item.QueueID, ordinal, startedAt),
		Domain:       domain,
		ActorKind:    actorKind,
		ActorID:      actorID,
		ActionID:     strings.TrimSpace(item.ActionID),
		MainType:     strings.TrimSpace(item.Spec.MainType),
		SubType:      strings.TrimSpace(item.Spec.SubType),
		StartedAt:    startedAt,
		Capabilities: capabilities,
	}
}

func queueOperationEffectKey(queueID int64, ordinal int, startedAt time.Time) string {
	return "action_queue:" + strconv.FormatInt(queueID, 10) + ":" + strconv.Itoa(ordinal) + ":" + strconv.FormatInt(startedAt.UnixNano(), 10)
}
