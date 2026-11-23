package store

import (
	"context"
	"reflect"
	"testing"
	"time"

	"mysecondwebgame/server/internal/operationeffect"
	"mysecondwebgame/server/internal/operationeffect/transmute"
)

func TestOfflineAlchemyTimingPostgresEquivalent(t *testing.T) {
	for _, actionID := range []string{"alchemy.transmute", "alchemy.convert"} {
		t.Run(actionID, func(t *testing.T) { testOfflineAlchemyTimingPostgresEquivalent(t, actionID) })
	}
}

func testOfflineAlchemyTimingPostgresEquivalent(t *testing.T, actionID string) {
	fixture, ctx, counter := recoveryPostgresStore(t)
	s := New(fixture.db, fixture.config, fixture.gameData)
	var wantPayload []byte
	var wantReceipts []operationeffect.ChargeReceipt
	var wantStock int64
	var wantQueries int64
	var wantStarts []string
	for _, optimized := range []bool{false, true} {
		tx, err := s.db.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(context.Background())
		s.registerInventoryAuditBuffer(tx)
		defer s.discardInventoryAuditBuffer(tx)
		if err = s.ensureAllPlayerAttributesTx(ctx, tx, 970071); err != nil {
			t.Fatal(err)
		}
		inputID := "wood"
		var args any = alchemyTransmuteArgs{ItemID: inputID}
		if actionID == "alchemy.convert" {
			inputID = "crystal_chunk"
			group := s.gameData.AlchemyGroupByItemID(inputID)
			if group == nil {
				t.Fatal("missing conversion fixture")
			}
			args = alchemyConvertArgs{SourceItemID: inputID, GroupID: group.ID}
			if err = s.GrantByTemplateTx(ctx, tx, 970071, GoldCoinItemID, group.GoldCost*128+1); err != nil {
				t.Fatal(err)
			}
		}
		if err = s.GrantByTemplateTx(ctx, tx, 970071, inputID, 128); err != nil {
			t.Fatal(err)
		}
		at := time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)
		spec := ActionSpec{ID: actionID, MainType: "alchemy", DurationS: 8}
		e := &alchemyExecutor{store: s, actionID: spec.ID}
		payload, err := e.FreezeRunContextTx(ctx, tx, 970071, ActionActorTypePlayer, 0, spec, args, at)
		if err != nil {
			t.Fatal(err)
		}
		def := operationEffectTestDefinition(operationeffect.LifetimeCharges)
		def.Domain, def.Kind, def.Params = operationeffect.DomainAlchemy, transmute.Speed, []byte(`{"durationSeconds":1}`)
		if actionID == "alchemy.convert" {
			def = s.operationEffectDefinitions["food_gentle_current_grill_v2"]
		}
		def.Lifetime.Charges = 7
		def.Selector = operationeffect.Selector{ActionIDs: []string{spec.ID}}
		session, err := operationeffect.NewSession(s.operationEffectRegistry, []operationeffect.Grant{{ID: 71, Definition: def, Target: operationeffect.Target{Kind: operationeffect.TargetPlayer}, ValidFrom: at, InitialCharges: 7, RemainingCharges: 7}})
		if err != nil {
			t.Fatal(err)
		}
		item := QueueRuntimeItem{QueueID: 91, ActorType: ActionActorTypePlayer, ActionID: spec.ID, Spec: spec, TotalCount: 1000, PayloadRaw: payload, EffectSession: session, PreserveScheduledTimeline: true}
		began := time.Now()
		before := counter.count.Load()
		stopped := false
		var starts []string
		for i := 0; i < 200; i++ {
			// Exercise fresh attributes after a boundary; no timing may leak between starts.
			if i == 17 {
				if _, err = tx.Exec(ctx, `UPDATE player_attributes SET final_value=0.234567 WHERE user_id=970071 AND attr_key='alchemy_speed'`); err != nil {
					t.Fatal(err)
				}
			}
			item.PayloadRaw = e.clearReservedPayload(item.PayloadRaw)
			refreshed, _, err := e.refreshNextRuntimePayloadTx(ctx, tx, 970071, item, item.PayloadRaw, at)
			if err != nil {
				t.Fatal(err)
			}
			var timing *parsedAlchemyRuntime
			if optimized {
				v := parseAlchemyRuntime(refreshed)
				timing = &v
			}
			var stop bool
			var reason string
			item.PayloadRaw, stop, reason, err = e.prepareNextAlchemyEffectRunTx(ctx, tx, 970071, item, refreshed, at, i+1, timing)
			if err != nil {
				t.Fatal(err)
			}
			starts = append(starts, string(item.PayloadRaw))
			if stop {
				if reason != ActionQueueCancelReasonCostInsufficient || i != 128 {
					t.Fatal("unexpected stop", i, reason)
				}
				stopped = true
				break
			}
			duration := parseQueueRuntimeDuration(item.PayloadRaw)
			if (actionID == "alchemy.transmute" && i < 7 && duration != time.Second) || ((actionID != "alchemy.transmute" || i >= 7) && duration < 2*time.Second) {
				t.Fatal("food timing changed", i, duration)
			}
			at = at.Add(duration)
		}
		if !stopped {
			t.Fatal("did not stop at exhausted materials")
		}
		queries := counter.count.Load() - before
		t.Logf("optimized=%v 128 starts plus insufficient-material boundary elapsed=%s queries=%d", optimized, time.Since(began), queries)
		var stock int64
		if err = tx.QueryRow(ctx, `SELECT COALESCE(sum(quantity),0)::bigint FROM player_item_instances WHERE user_id=970071`).Scan(&stock); err != nil {
			t.Fatal(err)
		}
		if !optimized {
			wantPayload, wantReceipts, wantStock = item.PayloadRaw, session.ChargeReceipts(), stock
			wantStarts, wantQueries = starts, queries
		} else if string(wantPayload) != string(item.PayloadRaw) || !reflect.DeepEqual(wantStarts, starts) || !reflect.DeepEqual(wantReceipts, session.ChargeReceipts()) || wantStock != stock || queries != wantQueries-128 {
			t.Fatal("timing, effects or material consumption changed")
		}
		if err = tx.Rollback(ctx); err != nil {
			t.Fatal(err)
		}
	}
}
