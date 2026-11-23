package store

import (
	"testing"

	"mysecondwebgame/server/internal/operationeffect"
	"mysecondwebgame/server/internal/operationeffect/lifeaction"
)

func TestPrimaryWorkEffectBatchEligibility(t *testing.T) {
	one := int64(1)
	base := QueueRuntimeItem{ActorType: ActionActorTypePlayer, PreserveScheduledTimeline: true,
		Spec:       ActionSpec{MainType: "collecting", ItemReward: map[string]ActionItemRewardSpec{"wood": {Count: &one}}, StatusExpReward: map[string]float64{"collecting": 2}},
		PayloadRaw: []byte(`{"executorId":"generic","runtime":{"durationMs":2000}}`)}
	if !primaryWorkEffectBatchEligible(base) {
		t.Fatal("plain player gathering not eligible")
	}
	for _, edit := range []func(*QueueRuntimeItem){
		func(v *QueueRuntimeItem) { v.ActorType = ActionActorTypeKitty },
		func(v *QueueRuntimeItem) { v.PreserveScheduledTimeline = false },
		func(v *QueueRuntimeItem) { v.Spec.Costs = map[string]int64{"wood": 1} },
		func(v *QueueRuntimeItem) {
			v.Spec.MaterialInstanceRequirements = map[string]ActionMaterialInstanceRequirement{"wood": {EnhanceLevel: 1}}
		},
		func(v *QueueRuntimeItem) {
			v.Spec.MainType = "alchemy"
			v.PayloadRaw = []byte(`{"executorId":"alchemy.bench","runtime":{"durationMs":1000}}`)
		},
		func(v *QueueRuntimeItem) { v.Spec.StatusExpReward = map[string]float64{"collecting_speed": 1} },
		func(v *QueueRuntimeItem) {
			v.Spec.ItemReward = map[string]ActionItemRewardSpec{"wood": {Range: &ActionItemRewardRange{Min: 1, Max: 2}}}
		},
		func(v *QueueRuntimeItem) {
			v.PayloadRaw = []byte(`{"executorId":"generic","runtime":{"durationMs":2000,"useInstanceBindings":true}}`)
		},
		func(v *QueueRuntimeItem) {
			v.PayloadRaw = []byte(`{"executorId":"generic","runtime":{"durationMs":2000},"runContext":{"inheritFromInstanceId":1}}`)
		},
		func(v *QueueRuntimeItem) {
			v.EffectSession = operationeffect.NewEmptySession()
			v.EffectSession.SetPassive(&operationeffect.PassiveSession{ProgressOrdering: true})
		},
	} {
		candidate := base
		edit(&candidate)
		if primaryWorkEffectBatchEligible(candidate) {
			t.Fatalf("unsafe batch accepted: %+v", candidate)
		}
	}
	if primaryWorkBatchSnapshotSupported(workTestSnapshot("future_unknown_effect", `{}`)) {
		t.Fatal("unknown effect must retain sequential path")
	}
}

func TestPrimaryWorkBatchStopsBeforeLevelChangeAndRoundsEachGain(t *testing.T) {
	progress := attributeProgressNode{Level: 1, Exp: 4, NextExp: 10}
	if got := primaryWorkBeforeLevelLimit(4, progress, 2, 2, 100); got != 2 {
		t.Fatalf("limit=%d want 2", got)
	}
	if got := primaryWorkBeforeLevelLimit(9, attributeProgressNode{Level: 1, Exp: 9, NextExp: 10}, 2, 2, 100); got != 1 {
		t.Fatalf("immediate level must use sequential path: %d", got)
	}
	meal := workTestSnapshot(lifeaction.KindWorkExperience, `{"experienceBonusRate":0.07123}`)
	spec := ActionSpec{MainType: "collecting", StatusExpReward: map[string]float64{"collecting": 0.0001}}
	gain, err := primaryWorkBatchRoundedExperience(spec, []operationeffect.Snapshot{meal, meal, meal})
	if err != nil || gain["collecting"] != 0.0003 {
		t.Fatal("per-operation rounding changed", gain, err)
	}
}
