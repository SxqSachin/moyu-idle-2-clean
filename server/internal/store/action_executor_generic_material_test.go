package store

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestGenericExecutorValidateAndNormalizeArgs(t *testing.T) {
	executor := newGenericExecutor(&Store{})

	validRaw, err := json.Marshal(genericExecutorArgs{
		MaterialInstanceBindings: map[string][]int64{
			"copper_dagger": []int64{9, 7, 7},
		},
		InheritFromInstanceID: 7,
	})
	if err != nil {
		t.Fatalf("marshal valid args: %v", err)
	}
	argsAny, err := executor.ValidateAndNormalizeArgs(validRaw)
	if err != nil {
		t.Fatalf("expected valid args, got err=%v", err)
	}
	args := argsAny.(genericExecutorArgs)
	gotBindings := args.MaterialInstanceBindings["copper_dagger"]
	if len(gotBindings) != 2 || gotBindings[0] != 7 || gotBindings[1] != 9 {
		t.Fatalf("unexpected bindings: %+v", gotBindings)
	}
	if args.InheritFromInstanceID != 7 {
		t.Fatalf("unexpected inherit source: %d", args.InheritFromInstanceID)
	}

	if _, err := executor.ValidateAndNormalizeArgs(json.RawMessage(`{"materialInstanceBindings":{"copper_dagger":[9]},"inheritFromInstanceId":7}`)); err == nil {
		t.Fatalf("expected validation error when inherit source is not in bindings")
	}
}

func TestGenericDurationRefreshPreservesExecutorRuntimeContract(t *testing.T) {
	runtimeRaw, err := json.Marshal(genericExecutorRuntime{
		BaseDurationMs:      45_000,
		DurationMs:          30_000,
		ExtraAttrExp:        map[string]float64{"mining": 2},
		UseInstanceBindings: true,
	})
	if err != nil {
		t.Fatalf("marshal runtime: %v", err)
	}
	runContextRaw, err := json.Marshal(genericExecutorRunContext{
		UseInstanceBindings:      true,
		MaterialInstanceBindings: map[string][]int64{"ore": {17}},
		InheritFromInstanceID:    17,
	})
	if err != nil {
		t.Fatalf("marshal run context: %v", err)
	}
	payloadRaw, err := json.Marshal(queueRuntimeEnvelope{
		ActionID:   "work_mining",
		ExecutorID: "generic",
		Runtime:    runtimeRaw,
		RunContext: runContextRaw,
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	nextPayload, changed, err := newGenericExecutor(&Store{}).refreshQueueDurationTx(
		context.Background(), nil, 7,
		QueueRuntimeItem{
			ActorType:  ActionActorTypePlayer,
			ActionID:   "work_mining",
			Spec:       ActionSpec{ID: "work_mining", MainType: "mining", DurationS: 60},
			PayloadRaw: payloadRaw,
		},
	)
	if err != nil {
		t.Fatalf("refresh generic duration: %v", err)
	}
	if !changed {
		t.Fatal("duration change was not reported")
	}
	runtime, ok := parseGenericExecutorRuntimeEnvelope(nextPayload)
	if !ok {
		t.Fatal("refreshed generic runtime no longer parses")
	}
	if runtime.BaseDurationMs != 45_000 || runtime.Duration() != 45*time.Second || !runtime.UseInstanceBindings {
		t.Fatalf("refreshed runtime=%+v, want preserved base duration and instance-binding mode", runtime)
	}
	runContext, ok := parseGenericRunContext(nextPayload)
	if !ok || !runContext.UseInstanceBindings || runContext.InheritFromInstanceID != 17 || len(runContext.MaterialInstanceBindings["ore"]) != 1 {
		t.Fatalf("refreshed run context=%+v, want original binding state", runContext)
	}
}

func TestActionMaterialInstanceRequirementRejectsAlteredEquipment(t *testing.T) {
	store := &Store{}
	requirement := ActionMaterialInstanceRequirement{
		EnhanceLevel:           0,
		RequireTemplateAffixes: true,
		Durability:             "full",
	}
	base := LockedItemInstance{
		TemplateItem:  "cat_sword",
		Quantity:      1,
		EnhanceLevel:  0,
		Durability:    100,
		MaxDurability: 100,
		State:         "inventory",
	}
	if !store.instanceSatisfiesActionMaterialRequirement(base, requirement) {
		t.Fatal("unaltered full-durability equipment should satisfy the recipe requirement")
	}
	enhanced := base
	enhanced.EnhanceLevel = 1
	if store.instanceSatisfiesActionMaterialRequirement(enhanced, requirement) {
		t.Fatal("enhanced equipment must be rejected")
	}
	damaged := base
	damaged.Durability = 99
	if store.instanceSatisfiesActionMaterialRequirement(damaged, requirement) {
		t.Fatal("damaged equipment must be rejected")
	}
	customized := base
	customized.CustomData.Name = "mine"
	if store.instanceSatisfiesActionMaterialRequirement(customized, requirement) {
		t.Fatal("customized equipment must be rejected")
	}
}
