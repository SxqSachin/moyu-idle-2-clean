package store

import (
	"encoding/json"
	"testing"
	"time"

	"mysecondwebgame/server/internal/operationeffect"
	"mysecondwebgame/server/internal/operationeffect/lifeaction"
)

func TestPrepareGenericOperationEffectSnapshotsUsesEachOfflineStart(t *testing.T) {
	now := time.Date(2026, time.September, 4, 12, 0, 0, 0, time.UTC)
	session := operationEffectTestSession(t, operationeffect.Grant{
		ID:               17,
		Definition:       operationEffectTestDefinition(operationeffect.LifetimeCharges),
		Target:           operationeffect.Target{Kind: operationeffect.TargetPlayer},
		ValidFrom:        now,
		InitialCharges:   2,
		RemainingCharges: 2,
		CreatedAt:        now,
	})
	item := operationEffectQueueItem(t, now, session)
	item.PreserveScheduledTimeline = true
	timing := queueRunTiming{
		StartAt:         now,
		CurrentDuration: time.Minute,
		NextDuration:    time.Minute,
		HasCurrentRun:   true,
	}

	snapshots, nextPayload, err := prepareGenericOperationEffectSnapshots(item, timing, now.Add(3*time.Minute), 3, 3, 3, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshots) != 3 || len(snapshots[0].Rules) != 0 || len(snapshots[1].Rules) != 1 || len(snapshots[2].Rules) != 1 {
		t.Fatalf("offline snapshots did not preserve per-start charge order: %#v", snapshots)
	}
	if got := len(session.ChargeReceipts()); got != 2 {
		t.Fatalf("charge receipts=%d, want 2", got)
	}
	if next, ok := queueOperationEffectSnapshot(nextPayload); ok {
		t.Fatalf("empty next-operation snapshot should be omitted, got %#v", next)
	}
}

func TestPrepareGenericOperationEffectSnapshotsIsNoOpWhenFeatureIsDisabled(t *testing.T) {
	now := time.Date(2026, time.September, 4, 12, 0, 0, 0, time.UTC)
	item := operationEffectQueueItem(t, now, nil)
	var envelope queueRuntimeEnvelope
	if err := json.Unmarshal(item.PayloadRaw, &envelope); err != nil {
		t.Fatal(err)
	}
	envelope.OperationEffects = nil
	payload, err := json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	item.PayloadRaw = payload
	timing := queueRunTiming{
		StartAt:         now,
		CurrentDuration: time.Minute,
		NextDuration:    time.Minute,
		HasCurrentRun:   true,
	}

	snapshots, nextPayload, err := prepareGenericOperationEffectSnapshots(item, timing, now.Add(100*time.Minute), 100, 100, 100, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshots) != 0 || string(nextPayload) != string(item.PayloadRaw) {
		t.Fatal("disabled operation effects changed generic queue settlement")
	}
}

func TestPrepareGenericOperationEffectSnapshotsUsesLogicalTimeOffline(t *testing.T) {
	now := time.Date(2026, time.September, 4, 12, 0, 0, 0, time.UTC)
	until := now.Add(90 * time.Second)
	definition := operationEffectTestDefinition(operationeffect.LifetimeDuration)
	definition.Lifetime = operationeffect.LifetimeDefinition{Kind: operationeffect.LifetimeDuration, DurationSeconds: 90}
	definition.Stacking = operationeffect.StackingDefinition{Key: "duration-extra", Mode: operationeffect.StackExtendDuration}
	session := operationEffectTestSession(t, operationeffect.Grant{
		ID:         18,
		Definition: definition,
		Target:     operationeffect.Target{Kind: operationeffect.TargetPlayer},
		ValidFrom:  now,
		ValidUntil: &until,
		CreatedAt:  now,
	})
	item := operationEffectQueueItem(t, now, session)
	item.PreserveScheduledTimeline = true
	timing := queueRunTiming{
		StartAt:         now,
		CurrentDuration: time.Minute,
		NextDuration:    time.Minute,
		HasCurrentRun:   true,
	}

	snapshots, _, err := prepareGenericOperationEffectSnapshots(item, timing, now.Add(3*time.Minute), 3, 2, 3, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshots) != 3 || len(snapshots[0].Rules) != 0 || len(snapshots[1].Rules) != 1 || len(snapshots[2].Rules) != 0 {
		t.Fatalf("duration effect was not evaluated at logical starts: %#v", snapshots)
	}
}

func TestPrepareGenericOperationEffectSnapshotsUsesActualOnlineNextStart(t *testing.T) {
	startedAt := time.Date(2026, time.September, 4, 12, 0, 0, 0, time.UTC)
	settledAt := startedAt.Add(2 * time.Minute)
	validFrom := startedAt.Add(90 * time.Second)
	validUntil := validFrom.Add(time.Hour)
	definition := operationEffectTestDefinition(operationeffect.LifetimeDuration)
	definition.Lifetime = operationeffect.LifetimeDefinition{Kind: operationeffect.LifetimeDuration, DurationSeconds: 3600}
	definition.Stacking = operationeffect.StackingDefinition{Key: "online-duration", Mode: operationeffect.StackExtendDuration}
	session := operationEffectTestSession(t, operationeffect.Grant{
		ID:         19,
		Definition: definition,
		Target:     operationeffect.Target{Kind: operationeffect.TargetPlayer},
		ValidFrom:  validFrom,
		ValidUntil: &validUntil,
		CreatedAt:  validFrom,
	})
	item := operationEffectQueueItem(t, startedAt, session)
	item.PreserveScheduledTimeline = false
	timing := queueRunTiming{
		StartAt:         startedAt,
		CurrentDuration: time.Minute,
		NextDuration:    time.Minute,
		HasCurrentRun:   true,
	}

	_, nextPayload, err := prepareGenericOperationEffectSnapshots(item, timing, settledAt, 1, 1, 1, false)
	if err != nil {
		t.Fatal(err)
	}
	next, ok := queueOperationEffectSnapshot(nextPayload)
	if !ok || len(next.Rules) != 1 || !next.StartedAt.Equal(settledAt) {
		t.Fatalf("online next operation did not use actual settlement start: %#v", next)
	}
}

func TestQueueOperationEffectOperationTreatsRestartAsNewStart(t *testing.T) {
	now := time.Date(2026, time.September, 4, 12, 0, 0, 0, time.UTC)
	item := operationEffectQueueItem(t, now, operationeffect.NewEmptySession())
	first := queueOperationEffectOperation(operationeffect.DomainLifeAction, item, now, 1, item.PayloadRaw)
	restarted := queueOperationEffectOperation(operationeffect.DomainLifeAction, item, now.Add(time.Second), 1, item.PayloadRaw)

	if first.Key == restarted.Key {
		t.Fatalf("restarted queue ordinal reused operation key %q", first.Key)
	}
}

func TestQueueOperationEffectOperationOnlyMarksMaterialAlchemyAsInfusion(t *testing.T) {
	now := time.Date(2026, time.September, 4, 12, 0, 0, 0, time.UTC)
	runContext, err := marshalAlchemyRunContext(alchemyBenchArgs{
		CarrierInstanceID: 7,
		Materials:         map[string]int64{"wood": 8},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(queueRuntimeEnvelope{
		ActionID:   "alchemy.bench",
		ExecutorID: "alchemy.bench",
		Runtime:    json.RawMessage(`{"durationMs":60000}`),
		RunContext: runContext,
	})
	if err != nil {
		t.Fatal(err)
	}
	item := QueueRuntimeItem{QueueID: 45, ActorType: ActionActorTypePlayer, ActionID: "alchemy.bench", PayloadRaw: payload}
	operation := queueOperationEffectOperation(operationeffect.DomainAlchemy, item, now, 1, payload)
	if !operation.Capabilities["infusion"] {
		t.Fatal("material alchemy bench was not marked as an infusion")
	}

	runContext, err = marshalAlchemyRunContext(alchemyBenchArgs{CarrierInstanceID: 7, FishInstanceID: 9}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var envelope queueRuntimeEnvelope
	if err := json.Unmarshal(payload, &envelope); err != nil {
		t.Fatal(err)
	}
	envelope.RunContext = runContext
	payload, err = json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	operation = queueOperationEffectOperation(operationeffect.DomainAlchemy, item, now, 1, payload)
	if operation.Capabilities["infusion"] {
		t.Fatal("legacy fish infusion unexpectedly matched material infusion effects")
	}
}

func operationEffectTestSession(t *testing.T, grant operationeffect.Grant) *operationeffect.Session {
	t.Helper()
	registry := operationeffect.NewRegistry()
	if err := lifeaction.Register(registry); err != nil {
		t.Fatal(err)
	}
	session, err := operationeffect.NewSession(registry, []operationeffect.Grant{grant})
	if err != nil {
		t.Fatal(err)
	}
	return session
}

func operationEffectTestDefinition(lifetime operationeffect.LifetimeKind) operationeffect.Definition {
	params, _ := json.Marshal(map[string]any{"chance": 1.0, "quantity": 1})
	definition := operationeffect.Definition{
		ID:             "test-extra-output",
		RulesetVersion: "test-v1",
		Domain:         operationeffect.DomainLifeAction,
		Kind:           lifeaction.KindExtraMainOutputChance,
		NameKey:        "operation_effect.test.name",
		DescKey:        "operation_effect.test.desc",
		AllowedTargets: []operationeffect.TargetKind{operationeffect.TargetPlayer},
		Lifetime:       operationeffect.LifetimeDefinition{Kind: lifetime, Charges: 2},
		Stacking:       operationeffect.StackingDefinition{Key: "charge-extra", Mode: operationeffect.StackAddCharges},
		Params:         params,
	}
	return definition
}

func operationEffectQueueItem(t *testing.T, now time.Time, session *operationeffect.Session) QueueRuntimeItem {
	t.Helper()
	payload, err := json.Marshal(queueRuntimeEnvelope{
		ActionID:   "work_logging",
		ExecutorID: "generic",
		Runtime:    json.RawMessage(`{"durationMs":60000}`),
		OperationEffects: &operationeffect.Snapshot{
			SchemaVersion: operationeffect.SnapshotSchemaVersion,
			OperationKey:  "action_queue:44:1",
			StartedAt:     now,
			Rules:         []operationeffect.RuleSnapshot{},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return QueueRuntimeItem{
		QueueID:       44,
		ActorType:     ActionActorTypePlayer,
		ActionID:      "work_logging",
		Spec:          ActionSpec{MainType: "logging", SubType: "common", MainOutput: "wood", ItemReward: map[string]ActionItemRewardSpec{"wood": {}}},
		TotalCount:    4,
		DoneCount:     0,
		PayloadRaw:    payload,
		RunStartAt:    &now,
		EffectSession: session,
	}
}
