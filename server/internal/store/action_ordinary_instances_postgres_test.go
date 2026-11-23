package store

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"mysecondwebgame/server/internal/operationeffect"
	"mysecondwebgame/server/internal/operationeffect/lifeaction"
)

func TestOrdinaryPostgresInheritedEquipmentAndExtra(t *testing.T) {
	fixture, ctx, _ := recoveryPostgresStore(t)
	s := New(fixture.db, fixture.config, fixture.gameData)
	s.config.OrdinarySettlementEnabled = true
	s.config.ActionEnhanceInheritanceRatio = 0.5
	tx, err := s.db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	s.registerInventoryAuditBuffer(tx)
	defer s.discardInventoryAuditBuffer(tx)
	if err := s.ensureAllPlayerAttributesTx(ctx, tx, 970071); err != nil {
		t.Fatal(err)
	}
	input, err := s.AddInventoryEnhancedInstanceTx(ctx, tx, 970071, "iron_bracers", 10)
	if err != nil {
		t.Fatal(err)
	}
	spec := BuildActionSpecMap(s.gameData)["craft_base2_mistwarden_charm"]
	spec.Costs = map[string]int64{"iron_bracers": 1}
	spec.StatusExpReward = nil
	args := genericExecutorArgs{MaterialInstanceBindings: map[string][]int64{"iron_bracers": {input}}, InheritFromInstanceID: input}
	at := time.Now().Add(-time.Hour).Truncate(time.Second)
	e := newGenericExecutor(s)
	item := QueueRuntimeItem{QueueID: 87, ActionID: spec.ID, Spec: spec, ActorType: ActionActorTypePlayer, TotalCount: 1, EffectSession: operationeffect.NewEmptySession(), PreserveScheduledTimeline: true}
	item.PayloadRaw, err = e.FreezeRunContextTx(ctx, tx, 970071, item.ActorType, 0, spec, args, at)
	if err != nil {
		t.Fatal(err)
	}
	item.PayloadRaw, err = e.StartRunTx(ctx, tx, 970071, item, at)
	if err != nil {
		t.Fatal(err)
	}
	op := queueOperationEffectOperation(operationeffect.DomainLifeAction, item, at, 1, item.PayloadRaw)
	snapshot := operationeffect.EmptySnapshot(op)
	snapshot.Rules = []operationeffect.RuleSnapshot{{GrantID: 99, DefinitionID: "proof-extra", RulesetVersion: "proof-v1", Domain: operationeffect.DomainLifeAction, Kind: lifeaction.KindExtraMainOutputChance, Params: json.RawMessage(`{"chance":1,"quantity":1}`)}}
	item.PayloadRaw, err = setQueueOperationEffectSnapshot(item.PayloadRaw, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	end := at.Add(parseQueueRuntimeDuration(item.PayloadRaw))
	item.RunStartAt, item.RunEndsAt = &at, &end
	state, err := ordinaryState(item.PayloadRaw)
	if err != nil {
		t.Fatal(err)
	}
	if state.Grants[spec.MainOutput].EnhanceLevel != 5 {
		t.Fatal("inheritance not frozen at accepted start")
	}
	// A subsequent catalog change cannot alter this operation's output template.
	template := s.gameData.EquipmentTemplateByID(spec.MainOutput)
	template.EnhanceMaxLevel = 0
	template.BaseAttrs = map[string]float64{"attack": 999999}
	result, err := e.SettleFastForwardTx(ctx, tx, 970071, item, end, FastForwardBudget{MaxIterations: 2000})
	if err != nil {
		t.Fatal(err)
	}
	if result.Completed != 1 || result.ItemRewards[spec.MainOutput] != 2 {
		t.Fatalf("bad reward result: %+v", result)
	}
	rows, err := tx.Query(ctx, `SELECT enhance_level,quantity,base_snapshot,affixes FROM player_item_instances WHERE user_id=970071 ORDER BY enhance_level`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	levels := []int{}
	for rows.Next() {
		var level, qty int
		var raw, affixes []byte
		if err := rows.Scan(&level, &qty, &raw, &affixes); err != nil {
			t.Fatal(err)
		}
		if qty != 1 {
			t.Fatal("equipment stacked")
		}
		levels = append(levels, level)
		if level == 5 {
			var base map[string]float64
			if json.Unmarshal(raw, &base) != nil || len(base) == 0 || base["attack"] == 999999 {
				t.Fatal("lost frozen inherited snapshot")
			}
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(levels) != 2 || levels[0] != 0 || levels[1] != 5 {
		t.Fatalf("extra equipment incorrectly inherited enhancement: %v", levels)
	}
}
