package store

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
	"time"

	"mysecondwebgame/server/internal/operationeffect"
	alchemyeffect "mysecondwebgame/server/internal/operationeffect/alchemy"
	"mysecondwebgame/server/internal/operationeffect/lifeaction"
)

func offlineEffectRegistry(t *testing.T) *operationeffect.Registry {
	t.Helper()
	registry := operationeffect.NewRegistry()
	if err := lifeaction.Register(registry); err != nil {
		t.Fatal(err)
	}
	if err := alchemyeffect.Register(registry); err != nil {
		t.Fatal(err)
	}
	return registry
}

func offlineEffectGrants(start time.Time) []operationeffect.Grant {
	charges := operationEffectTestDefinition(operationeffect.LifetimeCharges)
	charges.Lifetime.Charges = 6
	charges.Params = json.RawMessage(`{"chance":0.47,"quantity":1}`)
	duration := operationEffectTestDefinition(operationeffect.LifetimeDuration)
	duration.ID = "offline-duration"
	duration.Lifetime = operationeffect.LifetimeDefinition{Kind: operationeffect.LifetimeDuration, DurationSeconds: 240}
	duration.Stacking = operationeffect.StackingDefinition{Key: "offline-duration", Mode: operationeffect.StackExtendDuration}
	duration.Params = json.RawMessage(`{"chance":0.61,"quantity":2}`)
	until := start.Add(6 * time.Minute)
	return []operationeffect.Grant{
		{ID: 17, Definition: charges, Target: operationeffect.Target{Kind: operationeffect.TargetPlayer}, ValidFrom: start, InitialCharges: 6, RemainingCharges: 6},
		{ID: 18, Definition: duration, Target: operationeffect.Target{Kind: operationeffect.TargetPlayer}, ValidFrom: start.Add(2 * time.Minute), ValidUntil: &until},
	}
}

type offlineEffectTrace struct {
	Rules    [][]operationeffect.RuleSnapshot
	Outcomes []lifeaction.Outcome
	Receipts []operationeffect.ChargeReceipt
}

// Recreate the session from persisted remaining charges at every chunk. This
// deliberately does not let in-memory operation-key dedup hide a retry bug.
func simulateOfflineEffectChunks(t *testing.T, chunkSize int) offlineEffectTrace {
	t.Helper()
	start := time.Date(2026, 9, 5, 1, 0, 0, 0, time.UTC)
	grants := offlineEffectGrants(start)
	item := operationEffectQueueItem(t, start, nil)
	item.TotalCount = 12
	item.PreserveScheduledTimeline = true
	trace := offlineEffectTrace{}
	for item.DoneCount < item.TotalCount {
		session, err := operationeffect.NewSession(offlineEffectRegistry(t), grants)
		if err != nil {
			t.Fatal(err)
		}
		item.EffectSession = session
		count := min(chunkSize, item.TotalCount-item.DoneCount)
		doneAfter := item.DoneCount + count
		timing := queueRunTiming{StartAt: *item.RunStartAt, CurrentDuration: time.Minute, NextDuration: time.Minute, HasCurrentRun: true}
		snapshots, payload, err := prepareGenericOperationEffectSnapshots(item, timing, start.Add(12*time.Minute), count, computeAdditionalStarts(count, doneAfter, item.TotalCount), doneAfter, false)
		if err != nil {
			t.Fatal(err)
		}
		for _, snapshot := range snapshots {
			outcome, err := lifeaction.Resolve(snapshot, nil, "wood")
			if err != nil {
				t.Fatal(err)
			}
			trace.Rules = append(trace.Rules, snapshot.Rules)
			trace.Outcomes = append(trace.Outcomes, outcome)
		}
		for _, receipt := range session.ChargeReceipts() {
			for index := range grants {
				if grants[index].ID == receipt.GrantID {
					grants[index].RemainingCharges--
				}
			}
			trace.Receipts = append(trace.Receipts, receipt)
		}
		nextStart := timing.StartAt.Add(time.Duration(count) * time.Minute)
		item.RunStartAt, item.DoneCount, item.PayloadRaw = &nextStart, doneAfter, payload
	}
	return trace
}

func TestOfflineEffectsAreInvariantAcrossChunkSizes(t *testing.T) {
	sequential := simulateOfflineEffectChunks(t, 1)
	if len(sequential.Receipts) != 6 {
		t.Fatalf("six charges must cover exactly six successful starts, got %d", len(sequential.Receipts))
	}
	for _, size := range []int{2, 3, 5, 7, 12} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			if actual := simulateOfflineEffectChunks(t, size); !reflect.DeepEqual(actual, sequential) {
				t.Fatalf("chunk size %d changed snapshots, rolls or charges\nactual=%+v\nsequential=%+v", size, actual, sequential)
			}
		})
	}
	for index, rules := range sequential.Rules {
		hasDuration := false
		for _, rule := range rules {
			hasDuration = hasDuration || rule.GrantID == 18
		}
		if hasDuration != (index >= 2 && index < 6) {
			t.Fatalf("start %d violated duration interval [2m,6m)", index)
		}
	}
	if len(sequential.Rules[0]) != 0 {
		t.Fatal("legacy running action acquired an effect retroactively")
	}
}

func TestOfflineAlchemyFrozenChargeSurvivesExhaustionButDoesNotSpread(t *testing.T) {
	start := time.Date(2026, 9, 5, 1, 0, 0, 0, time.UTC)
	def := operationeffect.Definition{
		ID: "test-infusion", RulesetVersion: "test-v1", Domain: operationeffect.DomainAlchemy,
		Kind:    alchemyeffect.KindInfusionCapacityFloorTradeoff,
		NameKey: "effect.test.name", DescKey: "effect.test.desc",
		AllowedTargets: []operationeffect.TargetKind{operationeffect.TargetPlayer},
		Lifetime:       operationeffect.LifetimeDefinition{Kind: operationeffect.LifetimeCharges, Charges: 1},
		Stacking:       operationeffect.StackingDefinition{Key: "test-infusion", Mode: operationeffect.StackAddCharges},
		Params:         json.RawMessage(`{"extraMaterialQuantity":40,"positiveFloorReductionRate":0.12}`),
	}
	session, err := operationeffect.NewSession(offlineEffectRegistry(t), []operationeffect.Grant{{
		ID: 29, Definition: def, Target: operationeffect.Target{Kind: operationeffect.TargetPlayer},
		ValidFrom: start, InitialCharges: 1, RemainingCharges: 1,
	}})
	if err != nil {
		t.Fatal(err)
	}
	contextRaw, err := marshalAlchemyRunContext(alchemyBenchArgs{CarrierInstanceID: 7, Materials: map[string]int64{"wood": 8}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(queueRuntimeEnvelope{ActionID: "alchemy.bench", ExecutorID: "alchemy.bench", Runtime: json.RawMessage(`{"durationMs":60000}`), RunContext: contextRaw})
	if err != nil {
		t.Fatal(err)
	}
	item := QueueRuntimeItem{QueueID: 88, ActorType: ActionActorTypePlayer, ActionID: "alchemy.bench", PayloadRaw: payload, EffectSession: session, PreserveScheduledTimeline: true}
	first := queueOperationEffectOperation(operationeffect.DomainAlchemy, item, start, 1, payload)
	snapshot, err := session.Commit(session.Prepare(first))
	if err != nil {
		t.Fatal(err)
	}
	item.PayloadRaw, err = setQueueOperationEffectSnapshot(payload, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	// The persisted grant is exhausted on the next request; the running action
	// still owns its already-paid snapshot and must not become a bulk template.
	item.EffectSession = nil
	executor := newAlchemyExecutor(nil, "alchemy.bench")
	timing := queueRunTiming{StartAt: start, CurrentDuration: time.Minute, NextDuration: time.Minute, HasCurrentRun: true}
	if !executor.operationEffectsNeedPerStart(item, timing, start.Add(time.Hour)) {
		t.Fatal("last charged snapshot could be reused for every remaining alchemy run")
	}
	modifiers, err := alchemyOperationEffectModifiers(item)
	if err != nil || modifiers.ExtraMaterialQuantity != 40 {
		t.Fatalf("already-paid operation lost its effect: %+v %v", modifiers, err)
	}
	next := queueOperationEffectOperation(operationeffect.DomainAlchemy, item, start.Add(time.Minute), 2, payload)
	nextSnapshot, err := session.Commit(session.Prepare(next))
	if err != nil || len(nextSnapshot.Rules) != 0 || len(session.ChargeReceipts()) != 1 {
		t.Fatalf("exhausted grant leaked to next operation: %+v %v", nextSnapshot, err)
	}
}
