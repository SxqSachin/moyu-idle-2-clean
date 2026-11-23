package store

import (
	"encoding/json"
	"fmt"
	"math"
	"testing"
	"time"

	"mysecondwebgame/server/internal/operationeffect"
	alchemyeffect "mysecondwebgame/server/internal/operationeffect/alchemy"
)

func TestConversionFoodPostgresOfflineAudience(t *testing.T) {
	for _, form := range []string{"grill", "soup"} {
		for _, target := range []operationeffect.Target{
			{Kind: operationeffect.TargetPlayer},
			{Kind: operationeffect.TargetKitty, ActorID: 7},
			{Kind: operationeffect.TargetPlayerAndKitties},
		} {
			if form == "soup" && target.Kind == operationeffect.TargetPlayerAndKitties {
				continue
			}
			t.Run(form+"/"+string(target.Kind), func(t *testing.T) {
				ctx, tx := operationEffectPostgresTx(t)
				s := fishCuisineTestStore(t)
				at := time.Date(2026, 9, 5, 0, 0, 0, 0, time.UTC)
				def := s.operationEffectDefinitions["food_gentle_current_"+form+"_v2"]
				def.Lifetime.Charges = 0
				if form == "grill" {
					def.Lifetime.Charges = 2
				} else {
					def.Lifetime.DurationSeconds = 20
				}
				raw, _ := json.Marshal(def)
				if _, err := s.activateOperationEffectGrantTx(ctx, tx, 41, "gentle_current_"+form, def, target, raw, at); err != nil {
					t.Fatal(err)
				}
				// The persisted queue retains the historical start while wall time is far past expiry.
				if _, err := tx.Exec(ctx, `INSERT INTO player_action_queue VALUES(1,41,'running',$1,$2)`, at, at.Add(time.Second)); err != nil {
					t.Fatal(err)
				}
				session, err := s.loadOperationEffectSessionTx(ctx, tx, 41, at.Add(time.Hour))
				if err != nil || session == nil {
					t.Fatalf("historical session: %v", err)
				}
				spent := 0
				for i, actorID := range []int64{8, 0, 7, 8, 0, 7} {
					actor := operationeffect.ActorKitty
					if actorID == 0 {
						actor = operationeffect.ActorPlayer
					}
					op := operationeffect.Operation{Key: fmt.Sprintf("convert:%d", i), Domain: operationeffect.DomainAlchemy, ActionID: "alchemy.convert", ActorKind: actor, ActorID: actorID, StartedAt: at.Add(time.Duration(i) * 3 * time.Second)}
					eligible := target.Kind == operationeffect.TargetPlayerAndKitties || target.Kind == operationeffect.TargetPlayer && actorID == 0 || target.Kind == operationeffect.TargetKitty && actorID == 7
					want := eligible && (form == "soup" || spent < 2)
					frozen, err := session.Commit(session.Prepare(op))
					if err != nil {
						t.Fatal(err)
					}
					chance, err := alchemyeffect.ConversionFailureChance(.1865, frozen)
					if err != nil || (chance < .1865) != want {
						t.Fatalf("actor %d chance=%v want effect=%t err=%v", actorID, chance, want, err)
					}
					if want {
						spent++
						if math.Abs(chance-.177175) > 1e-12 {
							t.Fatal(chance)
						}
					}
					for retry := 0; retry < 2; retry++ {
						if _, err := session.Commit(session.Prepare(op)); err != nil {
							t.Fatal(err)
						}
						if _, err := s.flushOperationEffectSessionTx(ctx, tx, 41, session, at.Add(time.Hour)); err != nil {
							t.Fatal(err)
						}
					}
					// An already-started operation keeps its frozen probability after expiry.
					if replay, err := alchemyeffect.ConversionFailureChance(.1865, frozen); err != nil || replay != chance {
						t.Fatal("frozen result changed")
					}
					op.Key += ":expired"
					op.StartedAt = at.Add(20 * time.Second)
					if form == "soup" && len(session.Prepare(op).Snapshot().Rules) != 0 {
						t.Fatal("expired soup applied")
					}
				}
				var receipts int
				if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM player_operation_effect_charge_receipts`).Scan(&receipts); err != nil {
					t.Fatal(err)
				}
				wantReceipts := 0
				if form == "grill" {
					wantReceipts = 2
				}
				if receipts != wantReceipts {
					t.Fatalf("receipts=%d want=%d", receipts, wantReceipts)
				}
			})
		}
	}
}

func TestConversionFoodUsesPerStartSettlementOnlyWhenMatching(t *testing.T) {
	s := fishCuisineTestStore(t)
	at := time.Date(2026, 9, 5, 0, 0, 0, 0, time.UTC)
	def := s.operationEffectDefinitions["food_gentle_current_soup_v2"]
	until := at.Add(20 * time.Second)
	session, err := operationeffect.NewSession(s.operationEffectRegistry, []operationeffect.Grant{{ID: 1, Definition: def, Target: operationeffect.Target{Kind: operationeffect.TargetKitty, ActorID: 7}, ValidFrom: at, ValidUntil: &until}})
	if err != nil {
		t.Fatal(err)
	}
	executor := &alchemyExecutor{store: s, actionID: "alchemy.convert"}
	if executor.OperationEffectDomain() != operationeffect.DomainAlchemy || !executor.OperationEffectsRequirePerStartSettlement() {
		t.Fatal("conversion bypasses effect lifecycle")
	}
	timing := queueRunTiming{StartAt: at, CurrentDuration: 4 * time.Second, NextDuration: 4 * time.Second, HasCurrentRun: true}
	item := QueueRuntimeItem{QueueID: 1, ActionID: "alchemy.convert", ActorType: ActionActorTypeKitty, ActorID: 7, EffectSession: session, PreserveScheduledTimeline: true}
	if !executor.operationEffectsNeedPerStart(item, timing, at.Add(time.Hour)) {
		t.Fatal("offline conversion batched across expiry")
	}
	item.ActorID = 8
	if executor.operationEffectsNeedPerStart(item, timing, at.Add(time.Hour)) {
		t.Fatal("another actor forced through effect path")
	}
	item.ActorID = 7
	timing.StartAt = until
	if executor.operationEffectsNeedPerStart(item, timing, at.Add(time.Hour)) {
		t.Fatal("expired effect prevented batching")
	}
}

func TestConversionFoodPostgresInsufficientNextStartDoesNotSpend(t *testing.T) {
	ctx, tx := operationEffectPostgresTx(t)
	s := fishCuisineTestStore(t)
	_, err := tx.Exec(ctx, `CREATE TEMP TABLE player_item_instances (
	 instance_id bigint, user_id bigint, template_item_id text, custom_data jsonb DEFAULT '{}',
	 quantity bigint, enhance_level int DEFAULT 0, roll_seed bigint DEFAULT 0,
	 base_snapshot jsonb DEFAULT '{}', affixes jsonb DEFAULT '[]', durability int DEFAULT 0,
	 max_durability int DEFAULT 0, state text DEFAULT 'inventory', archived_at timestamptz);
	 INSERT INTO player_item_instances(instance_id,user_id,template_item_id,quantity) VALUES(1,41,'crystal_chunk',1);`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `UPDATE player_item_instances SET durability=$1,max_durability=$1`, s.defaultMaxDurability("crystal_chunk")); err != nil {
		t.Fatal(err)
	}
	owned, err := s.maxAffordableActionStartsTx(ctx, tx, 41, map[string]int64{"crystal_chunk": 1}, 1)
	if err != nil || owned != 1 {
		t.Fatal("invalid material fixture", owned, err)
	}
	at := time.Date(2026, 9, 5, 0, 0, 0, 0, time.UTC)
	def := s.operationEffectDefinitions["food_gentle_current_grill_v2"]
	session, err := operationeffect.NewSession(s.operationEffectRegistry, []operationeffect.Grant{{ID: 1, Definition: def, Target: operationeffect.Target{Kind: operationeffect.TargetPlayer}, ValidFrom: at, InitialCharges: 1, RemainingCharges: 1}})
	if err != nil {
		t.Fatal(err)
	}
	group := s.gameData.AlchemyGroupByItemID("crystal_chunk")
	if group == nil || group.GoldCost <= 0 {
		t.Fatal("missing conversion gold cost")
	}
	args := alchemyConvertArgs{SourceItemID: "crystal_chunk", GroupID: group.ID}
	runContext, err := marshalAlchemyRunContext(args, nil)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(queueRuntimeEnvelope{ActionID: "alchemy.convert", ExecutorID: "alchemy.convert", RunContext: runContext})
	if err != nil {
		t.Fatal(err)
	}
	item := QueueRuntimeItem{QueueID: 1, ActionID: "alchemy.convert", ActorType: ActionActorTypePlayer, PayloadRaw: payload, EffectSession: session}
	executor := &alchemyExecutor{store: s, actionID: "alchemy.convert"}
	for retry := 0; retry < 2; retry++ {
		_, stop, reason, err := executor.prepareNextAlchemyEffectRunTx(ctx, tx, 41, item, payload, at, 1, nil)
		if err != nil || !stop || reason != ActionQueueCancelReasonCostInsufficient {
			t.Fatalf("stop=%t reason=%s err=%v", stop, reason, err)
		}
	}
	var quantity int64
	if err := tx.QueryRow(ctx, `SELECT quantity FROM player_item_instances WHERE instance_id=1`).Scan(&quantity); err != nil {
		t.Fatal(err)
	}
	if quantity != 1 || len(session.ChargeReceipts()) != 0 {
		t.Fatal("failed start spent material or food")
	}
}
