package store

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
	"time"

	"mysecondwebgame/server/internal/operationeffect"
)

func TestOfflineGenericSnapshotBatchReference(t *testing.T) {
	at := time.Date(2026, 9, 8, 0, 0, 0, 123000000, time.UTC)
	for _, actor := range []string{ActionActorTypePlayer, ActionActorTypeKitty} {
		for _, variant := range []string{"legacy", "frozen", "bound", "food", "cancel", "online", "finished"} {
			t.Run(actor+"/"+variant, func(t *testing.T) {
				var want []operationeffect.Snapshot
				var wantPayload []byte
				var wantReceipts []operationeffect.ChargeReceipt
				for _, optimized := range []bool{false, true} {
					def := operationEffectTestDefinition(operationeffect.LifetimeCharges)
					def.Lifetime.Charges = 137
					def.AllowedTargets = []operationeffect.TargetKind{operationeffect.TargetPlayerAndKitties}
					session := operationEffectTestSession(t, operationeffect.Grant{ID: 17, Definition: def, Target: operationeffect.Target{Kind: operationeffect.TargetPlayerAndKitties}, ValidFrom: at.Add(70 * time.Second), InitialCharges: 137, RemainingCharges: 137})
					item := operationEffectQueueItem(t, at, session)
					item.TotalCount, item.DoneCount, item.ActorType = 1000, 19, actor
					if actor == ActionActorTypeKitty {
						item.ActorID = 2333
					}
					item.PreserveScheduledTimeline = variant != "online"
					var envelope queueRuntimeEnvelope
					if err := json.Unmarshal(item.PayloadRaw, &envelope); err != nil {
						t.Fatal(err)
					}
					switch variant {
					case "legacy":
						envelope.OperationEffects = nil
					case "bound":
						envelope.Runtime = []byte(`{"durationMs":60000,"useInstanceBindings":true}`)
					case "food":
						envelope.ExecutorID = "fish_food"
						envelope.Runtime = []byte(`{"durationMs":60000}`)
					}
					item.PayloadRaw, _ = json.Marshal(envelope)
					count, starts := 512, 512
					if variant == "online" {
						count, starts = 1, 1
					}
					if variant == "cancel" || variant == "finished" {
						starts--
					}
					if variant == "finished" {
						item.TotalCount = item.DoneCount + count
					}
					timing := queueRunTiming{StartAt: at, CurrentDuration: 30 * time.Second, NextDuration: time.Minute, HasCurrentRun: true}
					var snapshots []operationeffect.Snapshot
					var payload json.RawMessage
					var err error
					if optimized {
						snapshots, payload, err = prepareGenericOperationEffectSnapshots(item, timing, at.Add(24*time.Hour), count, starts, item.DoneCount+count, variant == "cancel")
					} else {
						snapshots, payload, err = prepareGenericOperationEffectSnapshotsReference(item, timing, at.Add(24*time.Hour), count, starts, item.DoneCount+count, variant == "cancel")
					}
					if err != nil {
						t.Fatal(err)
					}
					if !optimized {
						want, wantPayload, wantReceipts = snapshots, payload, session.ChargeReceipts()
					} else if !reflect.DeepEqual(want, snapshots) || string(wantPayload) != string(payload) || !reflect.DeepEqual(wantReceipts, session.ChargeReceipts()) {
						t.Fatal("snapshot, metadata or receipts changed")
					}
				}
			})
		}
	}
}

func BenchmarkOfflineGenericSnapshotBatch(b *testing.B) {
	at := time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC)
	item := QueueRuntimeItem{QueueID: 77, ActorType: ActionActorTypePlayer, ActionID: "logging", TotalCount: 10000, PreserveScheduledTimeline: true,
		Spec:       ActionSpec{MainType: "collecting", MainOutput: "wood", ItemReward: map[string]ActionItemRewardSpec{"wood": {}}},
		PayloadRaw: []byte(`{"executorId":"generic","runtime":{"durationMs":2000},"operationEffects":{"schemaVersion":1,"operationKey":"first","rules":[{"domain":"life_action","kind":"life_action.extra_main_output_chance","grantId":17,"params":{"chance":0.3,"quantity":1}}]}}`),
	}
	timing := queueRunTiming{StartAt: at, CurrentDuration: 2 * time.Second, NextDuration: 2 * time.Second, HasCurrentRun: true}
	for _, optimized := range []bool{false, true} {
		b.Run(fmt.Sprint(optimized), func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				var err error
				if optimized {
					_, _, err = prepareGenericOperationEffectSnapshots(item, timing, at.Add(time.Hour), 512, 512, 512, false)
				} else {
					_, _, err = prepareGenericOperationEffectSnapshotsReference(item, timing, at.Add(time.Hour), 512, 512, 512, false)
				}
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
