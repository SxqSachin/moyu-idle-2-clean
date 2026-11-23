package store

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"mysecondwebgame/server/internal/operationeffect"
	"mysecondwebgame/server/internal/operationeffect/lifeaction"
)

type primaryWorkProof struct {
	Done       int
	Start, End time.Time
	Snapshot   operationeffect.Snapshot
	Inventory  map[string]int64
	Experience float64
	Receipts   []operationeffect.ChargeReceipt
	Calls      int
	Queries    int64
	Elapsed    time.Duration
}

func seedPrimaryWorkProofGrant(t *testing.T, ctx context.Context, s *Store, at time.Time, kind, params string, duration, charges int64) {
	t.Helper()
	def := operationEffectTestDefinition(operationeffect.LifetimeDuration)
	def.ID = fmt.Sprintf("proof-%s-%d", kind, at.UnixNano())
	def.Kind = kind
	def.Params = []byte(params)
	def.Selector = operationeffect.Selector{MainTypes: []string{"collecting"}}
	def.Lifetime = operationeffect.LifetimeDefinition{Kind: operationeffect.LifetimeDuration, DurationSeconds: duration}
	def.Stacking = operationeffect.StackingDefinition{Key: def.ID, Mode: operationeffect.StackExtendDuration}
	if lifeaction.IsWorkMeal(kind) {
		def.Stacking.Key = "food.primary_work"
		def.Stacking.Exclusive = true
	}
	if charges > 0 {
		def.Lifetime = operationeffect.LifetimeDefinition{Kind: operationeffect.LifetimeCharges, Charges: charges}
		def.Stacking.Mode = operationeffect.StackAddCharges
	}
	raw, err := json.Marshal(def)
	if err != nil {
		t.Fatal(err)
	}
	err = s.withTx(ctx, func(tx pgx.Tx) error {
		_, err := s.activateOperationEffectGrantTx(ctx, tx, 970071, "proof-food", def, operationeffect.Target{Kind: operationeffect.TargetPlayer}, raw, at)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestPrimaryWorkEffectBatchPostgresMatchesSequential(t *testing.T) {
	for _, scenario := range []string{"overnight_5000_percent", "expiry_charges_levels_legacy"} {
		t.Run(scenario, func(t *testing.T) {
			fixture, _, counter := recoveryPostgresStore(t)
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
			defer cancel()
			s := New(fixture.db, fixture.config, fixture.gameData)
			s.config.OrdinarySettlementEnabled = true
			at := time.Now().Add(-9 * time.Hour).Truncate(time.Second)
			until := at.Add(8 * time.Hour)
			legacy := scenario != "overnight_5000_percent"
			if legacy {
				until = at.Add(30 * time.Minute)
			}
			if err := s.withTx(ctx, func(tx pgx.Tx) error { return s.ensureAllPlayerAttributesTx(ctx, tx, 970071) }); err != nil {
				t.Fatal(err)
			}
			if !legacy {
				exp := s.attributeExpProfile.capTotalExp["collecting"]
				progress, err := s.resolveAttributeProgress("collecting", exp)
				if err != nil {
					t.Fatal(err)
				}
				if _, err = s.db.Exec(ctx, `UPDATE player_attributes SET attr_exp=$1,base_level=$2,final_value=$3 WHERE user_id=970071 AND attr_key='collecting'`, exp, progress.Level, float64(progress.Level)); err != nil {
					t.Fatal(err)
				}
				seedPrimaryWorkProofGrant(t, ctx, s, at, lifeaction.KindFocusedHarvest, `{"chance":0.0272,"durationReductionRate":0.04}`, 28800, 0)
			} else {
				seedPrimaryWorkProofGrant(t, ctx, s, at, lifeaction.KindPatientHarvest, `{"chance":0.08,"durationPenaltyRate":0.04}`, 21, 0)
				seedPrimaryWorkProofGrant(t, ctx, s, at.Add(25*time.Second), lifeaction.KindWorkExperience, `{"experienceBonusRate":0.123456}`, 180, 0)
				seedPrimaryWorkProofGrant(t, ctx, s, at.Add(17*time.Second), lifeaction.KindExtraMainOutputChance, `{"chance":0.37,"quantity":1}`, 0, 15)
			}
			if _, err := s.db.Exec(ctx, `UPDATE player_attributes SET final_value=50 WHERE user_id=970071 AND attr_key='collecting_speed'`); err != nil {
				t.Fatal(err)
			}
			var queueID int64
			if err := s.db.QueryRow(ctx, `INSERT INTO player_action_queue(user_id,action_id,total_count,status,current_run_started_at,current_run_ends_at,payload)
 VALUES(970071,'logging',20000,'running',$1,$2,'{"executorId":"generic","runtime":{"durationMs":2000}}') RETURNING id`, at, at.Add(2*time.Second)).Scan(&queueID); err != nil {
				t.Fatal(err)
			}
			spec := BuildActionSpecMap(s.gameData)["logging"]
			var sharedStart *ordinarySettlementState
			run := func(limit int, commit bool) primaryWorkProof {
				t.Helper()
				tx, err := s.db.Begin(ctx)
				if err != nil {
					t.Fatal(err)
				}
				defer tx.Rollback(context.Background())
				s.registerInventoryAuditBuffer(tx)
				defer s.discardInventoryAuditBuffer(tx)
				if err = s.lockUserTx(ctx, tx, 970071); err != nil {
					t.Fatal(err)
				}
				effects, err := s.loadOperationEffectSessionTx(ctx, tx, 970071, until)
				if err != nil {
					t.Fatal(err)
				}
				executor := newGenericExecutor(s)
				item := QueueRuntimeItem{QueueID: queueID, ActorType: ActionActorTypePlayer, ActionID: "logging", Spec: spec, TotalCount: 20000, EffectSession: effects, PreserveScheduledTimeline: true, AuditPhase: "offline_fastforward"}
				item.PayloadRaw, err = executor.FreezeRunContextTx(ctx, tx, 970071, item.ActorType, 0, spec, genericExecutorArgs{}, at)
				if err != nil {
					t.Fatal(err)
				}
				op := queueOperationEffectOperation(operationeffect.DomainLifeAction, item, at, 1, item.PayloadRaw)
				first := operationeffect.EmptySnapshot(op)
				if !legacy {
					first, err = effects.Commit(effects.Prepare(op))
					if err != nil {
						t.Fatal(err)
					}
				}
				item.OperationEffects = &first
				item.PayloadRaw, err = executor.StartRunTx(ctx, tx, 970071, item, at)
				if err != nil {
					t.Fatal(err)
				}
				// Compare partitions of one committed random stream, not separate
				// queue creations, which deliberately receive different entropy.
				var started queueRuntimeEnvelope
				if err = json.Unmarshal(item.PayloadRaw, &started); err != nil {
					t.Fatal(err)
				}
				if sharedStart == nil {
					sharedStart = started.OrdinarySettlement
				} else {
					started.OrdinarySettlement = sharedStart
				}
				item.PayloadRaw, err = json.Marshal(started)
				if err != nil {
					t.Fatal(err)
				}
				item.PayloadRaw, err = setQueueOperationEffectSnapshot(item.PayloadRaw, first)
				if err != nil {
					t.Fatal(err)
				}
				start, end := at, at.Add(parseQueueRuntimeDuration(item.PayloadRaw))
				item.RunStartAt = &start
				item.RunEndsAt = &end
				proof := primaryWorkProof{Inventory: map[string]int64{}}
				began := time.Now()
				before := counter.count.Load()
				for item.DoneCount < item.TotalCount && !item.RunEndsAt.After(until) {
					result, err := executor.SettleFastForwardTx(ctx, tx, 970071, item, until, FastForwardBudget{MaxIterations: limit})
					if err != nil {
						t.Fatal(err)
					}
					if result.Completed <= 0 {
						t.Fatal("no progress")
					}
					proof.Calls++
					item.DoneCount = result.DoneAfter
					item.PayloadRaw = result.NextPayload
					a, b := result.NextRunStartAt, result.NextRunEndsAt
					item.RunStartAt = &a
					item.RunEndsAt = &b
					if _, err = tx.Exec(ctx, `UPDATE player_action_queue SET done_count=$2,payload=$3,current_run_started_at=$4,current_run_ends_at=$5 WHERE id=$1`, queueID, item.DoneCount, string(item.PayloadRaw), a, b); err != nil {
						t.Fatal(err)
					}
				}
				if _, err = s.flushOperationEffectSessionTx(ctx, tx, 970071, effects, until); err != nil {
					t.Fatal(err)
				}
				proof.Elapsed = time.Since(began)
				proof.Queries = counter.count.Load() - before
				proof.Done = item.DoneCount
				proof.Start = *item.RunStartAt
				proof.End = *item.RunEndsAt
				proof.Snapshot, _ = queueOperationEffectSnapshot(item.PayloadRaw)
				proof.Receipts = effects.ChargeReceipts()
				rows, err := tx.Query(ctx, `SELECT template_item_id,sum(quantity)::bigint FROM player_item_instances WHERE user_id=970071 GROUP BY template_item_id`)
				if err != nil {
					t.Fatal(err)
				}
				for rows.Next() {
					var id string
					var qty int64
					if err = rows.Scan(&id, &qty); err != nil {
						t.Fatal(err)
					}
					proof.Inventory[id] = qty
				}
				if err = rows.Err(); err != nil {
					t.Fatal(err)
				}
				rows.Close()
				if err = tx.QueryRow(ctx, `SELECT attr_exp FROM player_attributes WHERE user_id=970071 AND attr_key='collecting'`).Scan(&proof.Experience); err != nil {
					t.Fatal(err)
				}
				if commit {
					if err = tx.Commit(ctx); err != nil {
						t.Fatal(err)
					}
				}
				return proof
			}
			sequential := run(1, false)
			batch := run(10000, true)
			if sequential.Done != batch.Done || !sequential.Start.Equal(batch.Start) || !sequential.End.Equal(batch.End) || !reflect.DeepEqual(sequential.Inventory, batch.Inventory) || math.Abs(sequential.Experience-batch.Experience) > 1e-8 || !reflect.DeepEqual(sequential.Snapshot, batch.Snapshot) || !reflect.DeepEqual(sequential.Receipts, batch.Receipts) {
				t.Fatalf("sequential mismatch\nsequential=%+v\nbatch=%+v", sequential, batch)
			}
			if batch.Calls >= sequential.Calls || batch.Queries >= sequential.Queries {
				t.Fatalf("batch did not reduce work: sequential=%+v batch=%+v", sequential, batch)
			}
			if !legacy && batch.Done != 14400 {
				t.Fatalf("overnight count=%d", batch.Done)
			}
			t.Logf("%s: steps=%d sequential=%s/%d SQL/%d calls batch=%s/%d SQL/%d calls speedup=%.1fx", scenario, batch.Done, sequential.Elapsed, sequential.Queries, sequential.Calls, batch.Elapsed, batch.Queries, batch.Calls, float64(sequential.Elapsed)/float64(batch.Elapsed))
		})
	}
}

func TestPrimaryWorkEffectBatchPostgresRecoveryRollbackAndReplay(t *testing.T) {
	fixture, ctx, _ := recoveryPostgresStore(t)
	s := New(fixture.db, fixture.config, fixture.gameData)
	start := time.Now().Add(-8 * time.Hour).Truncate(time.Second)
	target := start.Add(8 * time.Hour)
	if err := s.withTx(ctx, func(tx pgx.Tx) error { return s.ensureAllPlayerAttributesTx(ctx, tx, 970071) }); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(ctx, `UPDATE player_attributes SET final_value=50 WHERE user_id=970071 AND attr_key='collecting_speed'`); err != nil {
		t.Fatal(err)
	}
	seedPrimaryWorkProofGrant(t, ctx, s, start, lifeaction.KindExtraMainOutputChance, `{"chance":1,"quantity":1}`, 0, 2000)
	if _, err := s.db.Exec(ctx, `INSERT INTO player_action_queue(user_id,action_id,total_count,status,current_run_started_at,current_run_ends_at,payload)
 VALUES(970071,'logging',20000,'running',$1,$2,'{"actionId":"logging","executorId":"generic","runtime":{"baseDurationMs":16000,"durationMs":2000}}')`, start, start.Add(2*time.Second)); err != nil {
		t.Fatal(err)
	}
	spec := BuildActionSpecMap(s.gameData)["logging"]
	spec.StatusExpReward = nil
	specs := map[string]ActionSpec{"logging": spec}
	recovery, err := s.EnsureOfflineRecovery(ctx, 970071, start, target, target)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.AdvanceOfflineRecovery(ctx, recovery, specs); err != nil {
		t.Fatal(err)
	}
	first, err := s.GetOfflineRecovery(ctx, 970071)
	if err != nil {
		t.Fatal(err)
	}
	assert := func(steps, quantity, charges, receipts int64) {
		t.Helper()
		var gotSteps, gotQuantity, gotCharges, gotReceipts int64
		err := s.db.QueryRow(ctx, `SELECT
 (SELECT done_count FROM player_action_queue WHERE user_id=970071),
 (SELECT sum(quantity)::bigint FROM player_item_instances WHERE user_id=970071 AND template_item_id='wood'),
 (SELECT remaining_charges FROM player_operation_effect_grants WHERE user_id=970071),
 (SELECT count(*) FROM player_operation_effect_charge_receipts)`).Scan(&gotSteps, &gotQuantity, &gotCharges, &gotReceipts)
		if err != nil || gotSteps != steps || gotQuantity != quantity || gotCharges != charges || gotReceipts != receipts {
			t.Fatalf("state=(%d,%d,%d,%d) want=(%d,%d,%d,%d) err=%v", gotSteps, gotQuantity, gotCharges, gotReceipts, steps, quantity, charges, receipts, err)
		}
	}
	assert(1000, 1999, 1000, 1000)
	if _, err = s.db.Exec(ctx, `ALTER TABLE player_offline_recoveries ADD CONSTRAINT injected_batch_failure CHECK(revision<2)`); err != nil {
		t.Fatal(err)
	}
	if err = s.AdvanceOfflineRecovery(ctx, first, specs); err == nil {
		t.Fatal("expected checkpoint failure")
	}
	assert(1000, 1999, 1000, 1000)
	if _, err = s.db.Exec(ctx, `ALTER TABLE player_offline_recoveries DROP CONSTRAINT injected_batch_failure`); err != nil {
		t.Fatal(err)
	}
	// Recreate all in-process store state before retrying the durable checkpoint.
	s = New(fixture.db, fixture.config, fixture.gameData)
	if err = s.AdvanceOfflineRecovery(ctx, first, specs); err != nil {
		t.Fatal(err)
	}
	assert(2000, 3999, 0, 2000)
	if err = s.AdvanceOfflineRecovery(ctx, first, specs); err != nil {
		t.Fatal(err)
	}
	assert(2000, 3999, 0, 2000)
	for range 20 {
		current, err := s.GetOfflineRecovery(ctx, 970071)
		if err != nil {
			t.Fatal(err)
		}
		if current.Status == "complete" {
			break
		}
		if err = s.AdvanceOfflineRecovery(ctx, current, specs); err != nil {
			t.Fatal(err)
		}
	}
	assert(14400, 16400, 0, 2000)
	completed, err := s.GetOfflineRecovery(ctx, 970071)
	if err != nil || completed.Status != "complete" || completed.Summary.ItemRewards["wood"] != 16400 {
		t.Fatal("recovery summary disagrees with credited inventory", completed, err)
	}
}
