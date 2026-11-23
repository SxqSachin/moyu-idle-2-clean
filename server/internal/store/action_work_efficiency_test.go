package store

import (
	"encoding/json"
	"testing"
	"time"

	"mysecondwebgame/server/internal/operationeffect"
)

func trainingSource(level int) operationeffect.PassiveSource {
	rate := int64(level * 2000)
	if level > 5 {
		rate = int64(10000 + (level-5)*800)
	}
	return operationeffect.PassiveSource{ID: "training", Version: "v1", Level: level, Rules: []operationeffect.PassiveRule{{Kind: operationeffect.KindActionEfficiency, Domain: operationeffect.DomainActionStart, RateBPS: rate, Selector: operationeffect.Selector{MainTypes: []string{"collecting", "alchemy", "cooking"}}}}}
}

func TestWorkEfficiencyLevelsScopeAndStacking(t *testing.T) {
	at := time.Unix(1000, 0)
	want := []int64{36000, 32000, 28000, 24000, 20000, 18518, 17241, 16129, 15151, 14285}
	for level := 1; level <= 10; level++ {
		p := &operationeffect.PassiveSession{Windows: []operationeffect.PassiveWindow{{From: at, Source: trainingSource(level)}}}
		for _, actor := range []operationeffect.ActorKind{operationeffect.ActorKitty, operationeffect.ActorPlayer} {
			for _, main := range []string{"collecting", "fishing", "equipment", "building"} {
				op := operationeffect.Operation{ActorKind: actor, MainType: main, Domain: operationeffect.DomainActionStart, StartedAt: at.Add(100 * 365 * 24 * time.Hour)}
				snapshot := p.Snapshot(op)
				if err := snapshot.Validate(); err != nil {
					t.Fatal(err)
				}
				if actor != operationeffect.ActorKitty || main != "collecting" {
					if snapshot.EfficiencyBPS() != 0 {
						t.Fatal("scope leaked")
					}
					continue
				}
				got := computeEffectiveActionDurationWithFloor(20*time.Second, -1+float64(snapshot.EfficiencyBPS())/10000, .5).Milliseconds()
				if got != want[level-1] {
					t.Fatalf("level %d: %d != %d", level, got, want[level-1])
				}
			}
		}
	}
	source := trainingSource(1)
	source.Rules = append(source.Rules, source.Rules[0])
	other := trainingSource(2)
	other.ID = "another"
	if (operationeffect.PassiveSnapshot{Sources: []operationeffect.PassiveSource{source, other}}).EfficiencyBPS() != 6000 {
		t.Fatal("overlapping selectors duplicated source")
	}
}

func TestWorkEfficiencyBatchBoundaryAndFrozenLegacy(t *testing.T) {
	at := time.Unix(1000, 0)
	effects, _ := operationeffect.NewSession(operationeffect.NewRegistry(), nil)
	effects.SetPassive(&operationeffect.PassiveSession{Windows: []operationeffect.PassiveWindow{{From: at.Add(95 * time.Second), Source: trainingSource(5)}}})
	item := QueueRuntimeItem{ActorType: ActionActorTypeKitty, Spec: ActionSpec{MainType: "collecting"}, TotalCount: 1000, PayloadRaw: []byte(`{"runtime":{"durationMs":40000}}`), EffectSession: effects, PreserveScheduledTimeline: true}
	timing := queueRunTiming{StartAt: at, CurrentDuration: 40 * time.Second, NextDuration: 40 * time.Second}
	limit, changed, err := workEfficiencyBatchLimit(item, timing, at.Add(time.Hour), 1000)
	if err != nil || changed || limit != 2 {
		t.Fatalf("must batch two old completions: %d %v %v", limit, changed, err)
	}
	timing.StartAt = at.Add(80 * time.Second)
	limit, changed, err = workEfficiencyBatchLimit(item, timing, at.Add(time.Hour), 1000)
	if err != nil || !changed || limit != 1 {
		t.Fatalf("must transition after the in-flight run: %d %v %v", limit, changed, err)
	}
	item.PayloadRaw, err = setWorkQueueRuntime(item, item.PayloadRaw, genericExecutorRuntime{DurationMs: 20000}, at.Add(120*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	timing = queueRunTiming{StartAt: at.Add(120 * time.Second), CurrentDuration: 20 * time.Second, NextDuration: 20 * time.Second}
	limit, changed, err = workEfficiencyBatchLimit(item, timing, at.Add(time.Hour), 1000)
	if err != nil || changed || limit != 1000 {
		t.Fatal("permanent effect disabled bulk settlement")
	}
	var envelope queueRuntimeEnvelope
	if err := json.Unmarshal(item.PayloadRaw, &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.TimingEffects.Sources[0].Level != 5 {
		t.Fatal("start did not freeze level")
	}
	// Current refresh must use the old snapshot even with a newer live session.
	effects.SetPassive(&operationeffect.PassiveSession{Windows: []operationeffect.PassiveWindow{{From: at, Source: trainingSource(10)}}})
	frozen, err := workTimingSnapshot(item, time.Time{})
	if err != nil || frozen.EfficiencyBPS() != 10000 {
		t.Fatal("refresh rewrote running contract")
	}
}

func BenchmarkWorkEfficiencyBatchPlan(b *testing.B) {
	at := time.Unix(1000, 0)
	effects, _ := operationeffect.NewSession(operationeffect.NewRegistry(), nil)
	effects.SetPassive(&operationeffect.PassiveSession{Windows: []operationeffect.PassiveWindow{{From: at, Source: trainingSource(10)}}})
	item := QueueRuntimeItem{ActorType: ActionActorTypeKitty, Spec: ActionSpec{MainType: "collecting"}, TotalCount: 10000, PayloadRaw: []byte(`{"runtime":{"durationMs":14285}}`), EffectSession: effects, PreserveScheduledTimeline: true}
	item.PayloadRaw, _ = setWorkQueueRuntime(item, item.PayloadRaw, genericExecutorRuntime{DurationMs: 14285}, at)
	timing := queueRunTiming{StartAt: at, CurrentDuration: 14285 * time.Millisecond, NextDuration: 14285 * time.Millisecond, HasCurrentRun: true}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if n, changed, err := workEfficiencyBatchLimit(item, timing, at.Add(24*time.Hour), 10000); err != nil || changed || n != 10000 {
			b.Fatal(n, changed, err)
		}
	}
}
