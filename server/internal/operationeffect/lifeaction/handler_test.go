package lifeaction_test

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"mysecondwebgame/server/internal/operationeffect"
	"mysecondwebgame/server/internal/operationeffect/lifeaction"
)

func TestResolveIsDeterministicAndKeepsRulesSeparate(t *testing.T) {
	refundParams, _ := json.Marshal(map[string]any{"chance": 1.0, "refundRate": 0.5})
	extraParams, _ := json.Marshal(map[string]any{"chance": 1.0, "quantity": 1})
	snapshot := operationeffect.Snapshot{
		SchemaVersion: operationeffect.SnapshotSchemaVersion,
		OperationKey:  "action:42:9",
		StartedAt:     time.Now(),
		Rules: []operationeffect.RuleSnapshot{
			{GrantID: 11, DefinitionID: "refund", Domain: operationeffect.DomainLifeAction, Kind: lifeaction.KindMaterialRefundChance, Params: refundParams},
			{GrantID: 12, DefinitionID: "extra", Domain: operationeffect.DomainLifeAction, Kind: lifeaction.KindExtraMainOutputChance, Params: extraParams},
		},
	}
	want := lifeaction.Outcome{
		Refunds:      []lifeaction.Refund{{GrantID: 11, DefinitionID: "refund", Items: map[string]int64{"wood": 2}}},
		ExtraOutputs: []lifeaction.ExtraOutput{{GrantID: 12, DefinitionID: "extra", ItemID: "plank", Quantity: 1}},
	}
	first, err := lifeaction.Resolve(snapshot, map[string]int64{"wood": 5}, "plank")
	if err != nil {
		t.Fatal(err)
	}
	second, err := lifeaction.Resolve(snapshot, map[string]int64{"wood": 5}, "plank")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, want) || !reflect.DeepEqual(first, second) {
		t.Fatalf("unexpected deterministic outcome: first=%#v second=%#v", first, second)
	}
}
