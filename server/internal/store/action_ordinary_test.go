package store

import (
	"encoding/json"
	"math"
	"reflect"
	"strings"
	"testing"
	"time"

	"mysecondwebgame/server/internal/actionsettlement"
	"mysecondwebgame/server/internal/operationeffect"
	"mysecondwebgame/server/internal/operationeffect/lifeaction"
)

func ordinaryUnitState(t *testing.T) (*Store, QueueRuntimeItem, *ordinarySettlementState) {
	t.Helper()
	s := fishCuisineTestStore(t)
	s.config.OrdinarySettlementEnabled = true
	spec := BuildActionSpecMap(s.gameData)["craft_base2_mistwarden_charm"]
	raw, err := json.Marshal(queueRuntimeEnvelope{ActionID: spec.ID, ExecutorID: "generic", Runtime: json.RawMessage(`{"durationMs":2000}`)})
	if err != nil {
		t.Fatal(err)
	}
	item := QueueRuntimeItem{QueueID: 77, ActionID: spec.ID, Spec: spec, DoneCount: 19, PayloadRaw: raw}
	if state, err := ordinaryState(raw); err != nil || state != nil {
		t.Fatal("legacy must remain unversioned until start")
	}
	item.PayloadRaw, err = s.freezeOrdinarySettlement(item, raw)
	if err != nil {
		t.Fatal(err)
	}
	state, err := ordinaryState(item.PayloadRaw)
	if err != nil {
		t.Fatal(err)
	}
	return s, item, state
}

func TestOrdinaryCheckpointFreezesRulesAndSurvivesSetters(t *testing.T) {
	s, item, state := ordinaryUnitState(t)
	if state.FirstOrdinal != 19 || state.Version != ordinarySettlementVersion || len(state.Seed) != 64 {
		t.Fatalf("bad checkpoint: %+v", state)
	}
	first := state.Seed
	item.Spec.Costs = map[string]int64{"changed": 123456}
	raw, err := s.freezeOrdinarySettlement(item, item.PayloadRaw)
	if err != nil {
		t.Fatal(err)
	}
	raw, err = setQueueRuntimePayload(raw, genericExecutorRuntime{DurationMs: 1777})
	if err != nil {
		t.Fatal(err)
	}
	raw, err = setGenericRunContext(raw, genericExecutorRunContext{})
	if err != nil {
		t.Fatal(err)
	}
	raw, err = setQueueOperationEffectSnapshot(raw, operationeffect.EmptySnapshot(operationeffect.Operation{Key: "probe", StartedAt: time.Now()}))
	if err != nil {
		t.Fatal(err)
	}
	after, err := ordinaryState(raw)
	if err != nil {
		t.Fatal(err)
	}
	if after.Seed != first || !reflect.DeepEqual(state, after) {
		t.Fatal("checkpoint changed while refreshing runtime")
	}
	var envelope queueRuntimeEnvelope
	if err := json.Unmarshal(raw, &envelope); err != nil {
		t.Fatal(err)
	}
	envelope.OrdinarySettlement.Version = "future-version"
	raw, err = json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ordinaryState(raw); err == nil {
		t.Fatal("unknown version silently fell back")
	}
}

func TestOrdinaryAdmissionGateAndLegacyIdentity(t *testing.T) {
	s, item, _ := ordinaryUnitState(t)
	legacy := json.RawMessage(`{"executorId":"generic","runtime":{"durationMs":2000}}`)
	s.config.OrdinarySettlementEnabled = false
	raw, err := s.freezeOrdinarySettlement(item, legacy)
	if err != nil || string(raw) != string(legacy) {
		t.Fatal("closed gate altered legacy work", err)
	}
	s.config.OrdinarySettlementEnabled = true
	raw, err = s.freezeOrdinarySettlement(item, legacy)
	if err != nil {
		t.Fatal(err)
	}
	state, err := ordinaryState(raw)
	if err != nil || state == nil || state.Spec.ID != item.ActionID {
		t.Fatal("legacy payload without duplicated action ID cannot resume", err)
	}
	encoded, err := json.Marshal(buildActionQueueDisplay(item.ActionID, raw))
	if err != nil || strings.Contains(string(encoded), state.Seed) || strings.Contains(string(encoded), "ordinarySettlement") {
		t.Fatal("private random checkpoint leaked into player display", err)
	}
}

func TestOrdinaryInventoryPreservesPrefixAndArchivedOrder(t *testing.T) {
	old := &ordinaryInventoryRow{id: 1, item: "ore", quantity: 2, commodity: true}
	archived := &ordinaryInventoryRow{id: 2, item: "ore", quantity: 5, archived: true, commodity: true}
	inv := &ordinaryInventory{rows: map[string][]*ordinaryInventoryRow{"ore": {old, archived}}, rules: map[string]ordinaryGrantRule{"ore": {Stackable: true, Commodity: true, Durability: 100}}, random: actionsettlement.NewRandom([32]byte{})}
	if err := inv.consume(map[string]int64{"ore": 3}); err != nil {
		t.Fatal(err)
	}
	if old.quantity != 0 || archived.quantity != 4 {
		t.Fatal("wrong input preference")
	}
	if err := inv.grant("ore", 4, 0, "refund", false, false); err != nil {
		t.Fatal(err)
	}
	if old.quantity != 0 || len(inv.created) != 1 || inv.created[0] == old {
		t.Fatal("depleted stack resurrected")
	}
	if err := inv.consume(map[string]int64{"ore": 5}); err != nil {
		t.Fatal(err)
	}
	if inv.created[0].quantity != 0 || archived.quantity != 3 {
		t.Fatal("new unarchived stack consumed after archived")
	}
	if inv.affordable(map[string]int64{"ore": 4}) {
		t.Fatal("borrowed unavailable future refunds")
	}
	if err := inv.consume(map[string]int64{"ore": 4}); err != ErrInsufficientItem || archived.quantity != 3 {
		t.Fatal("failed reservation partially consumed input")
	}
}

func TestOrdinaryEquipmentIsIndependentAndFrozen(t *testing.T) {
	_, _, state := ordinaryUnitState(t)
	state.Seed = strings.Repeat("21", 32)
	id := state.Spec.MainOutput
	inv := &ordinaryInventory{rows: map[string][]*ordinaryInventoryRow{}, rules: state.Grants, random: state.random()}
	if err := inv.grant(id, 2, 19, "base", true, false); err != nil {
		t.Fatal(err)
	}
	if err := inv.grant(id, 1, 19, "extra:88", true, false); err != nil {
		t.Fatal(err)
	}
	if len(inv.created) != 3 {
		t.Fatal("equipment was stacked")
	}
	for i, row := range inv.created {
		if row.quantity != 1 || row.commodity {
			t.Fatal("rolled equipment became a template cost")
		}
		for j := 0; j < i; j++ {
			if reflect.DeepEqual(row.affixes, inv.created[j].affixes) {
				t.Fatal("extra equipment cloned an earlier roll")
			}
		}
	}
}

func TestOrdinaryRefundCorrelationAndRounding(t *testing.T) {
	rule := operationeffect.RuleSnapshot{GrantID: 8, Domain: operationeffect.DomainLifeAction, Kind: lifeaction.KindMaterialRefundChance, Params: json.RawMessage(`{"chance":0.5,"refundRate":0.5}`)}
	rolls := 0
	resolver := lifeaction.Resolver{Sample: func(operationeffect.RuleSnapshot, float64) bool { rolls++; return true }}
	out, err := resolver.Resolve(operationeffect.Snapshot{Rules: []operationeffect.RuleSnapshot{rule}}, map[string]int64{"a": 3, "b": 5}, "output")
	if err != nil {
		t.Fatal(err)
	}
	if rolls != 1 || len(out.Refunds) != 1 || out.Refunds[0].Items["a"] != 1 || out.Refunds[0].Items["b"] != 2 {
		t.Fatalf("lost correlated refund or floor: %+v rolls=%d", out, rolls)
	}
	if err := addOrdinaryQuantity(map[string]int64{"a": math.MaxInt64}, "a", 1); err != ErrInventoryQuantityOverflow {
		t.Fatal("overflow accepted")
	}
}
