package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"mysecondwebgame/server/internal/operationeffect"
	"mysecondwebgame/server/internal/operationeffect/ratebonus"
)

func fishingBatchProofState(t *testing.T, ctx context.Context, tx pgx.Tx) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, table := range []string{"player_item_instances", "fishing_catches", "fishing_species_knowledge", "fishing_records", "fishing_trips", "fishing_trip_events", "fishing_spot_pressure", "fishing_tackle_wear", "fishing_tournaments", "fishing_tournament_entries", "player_attributes", "player_operation_effect_grants", "player_operation_effect_charge_receipts", "player_action_queue"} {
		var raw []byte
		// Table names are a fixed test-only allowlist. A/B runs share the same
		// transaction clock and restart sequences, so compare complete rows.
		if err := tx.QueryRow(ctx, `SELECT COALESCE(jsonb_agg(v ORDER BY v),'[]'::jsonb) FROM (SELECT to_jsonb(t) v FROM `+table+` t) rows`).Scan(&raw); err != nil {
			t.Fatal(table, err)
		}
		out[table] = string(raw)
	}
	var raw []byte
	if err := tx.QueryRow(ctx, `SELECT COALESCE(jsonb_agg(v ORDER BY v),'[]'::jsonb) FROM (SELECT jsonb_build_object('kind',kind,'counter',counter_key,'delta',delta,'payload',payload) v FROM achievement_events) rows`).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	out["achievement_events"] = string(raw)
	return out
}

func seedFishingBatchProof(t *testing.T, ctx context.Context, s *Store, tx pgx.Tx, mode string) QueueRuntimeItem {
	t.Helper()
	at := time.Date(2026, 9, 6, 22, 0, 0, 0, time.UTC)
	spot := s.gameData.FishingSpots[0]
	water := s.gameData.FishingWaters[0]
	for _, row := range s.gameData.FishingWaters {
		if row.ID == spot.WaterID {
			water = row
		}
	}
	rules := newFishingExecutor(s).rulesSnapshot(water, spot)
	// Ensure an overnight trip exercises dense catches and the 128-catch
	// boundary, while preserving all species, condition and fight algorithms.
	rules.Spot.BaseBiteRate = 2
	runtime := fishingExecutorRuntime{
		TripID: "batch-proof", ActorType: ActionActorTypePlayer, WaterID: water.ID, SpotID: spot.ID,
		Rules: rules, TournamentDivision: fishingTournamentDivisionOpen,
		Phase: fishingPhaseResting, DurationMS: 4000, PhaseStartedAtMS: at.Add(-4 * time.Second).UnixMilli(), PhaseEndsAtMS: at.UnixMilli(),
		StartedAtMS: at.UnixMilli(), PlannedEndAtMS: at.Add(24 * time.Hour).UnixMilli(), PlannedDurationSeconds: 86400,
		Seed: 424242, Strategy: "balanced", SafetyLoadPercent: 82, KeepRule: FishingKeepRule{Mode: mode},
		Loadout: FishingLoadoutSnapshot{Method: "float", MaxLoadKG: 100, Control: 50, Hooking: 50, Casting: 50, FishingLevel: 100, BiteMultiplier: 2, Efficiency: 2,
			Components: FishingLoadoutComponents{BaitItemID: "wood", MainLineItemID: "stone", LeaderItemID: "wood", HookItemID: "stone", FloatOrSinkerItemID: "wood"}},
		Supplies: FishingTripSupplies{Remaining: FishingSupplyCounts{Bait: 5000, MainLine: 100, Leader: 100, Hook: 100, FloatOrSinker: 100}},
	}
	if mode == "tackle_empty" {
		runtime.KeepRule.Mode = "keep_all"
		runtime.Loadout.MaxLoadKG = 0.01
		runtime.Supplies.Remaining.MainLine = 1
	}
	if mode == "bait_empty" {
		runtime.KeepRule.Mode = "keep_all"
		runtime.Supplies.Remaining.Bait = 3
	}
	if mode == "effects" {
		runtime.KeepRule.Mode = "keep_all"
		def := operationEffectTestDefinition(operationeffect.LifetimeCharges)
		def.Domain, def.Kind = operationeffect.DomainFishingCast, ratebonus.FishingBite
		def.Params = []byte(`{"bonusRate":0.05}`)
		def.Lifetime.Charges = 137
		def.Selector = operationeffect.Selector{ActionIDs: []string{FishingTripActionID}}
		raw, err := json.Marshal(def)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = s.activateOperationEffectGrantTx(ctx, tx, 970071, "proof-food", def, operationeffect.Target{Kind: operationeffect.TargetPlayer}, raw, at.Add(5*time.Minute)); err != nil {
			t.Fatal(err)
		}
	}
	payload, err := marshalFishingEnvelope(FishingTripActionID, runtime)
	if err != nil {
		t.Fatal(err)
	}
	var queueID int64
	err = tx.QueryRow(ctx, `INSERT INTO player_action_queue(user_id,action_id,total_count,status,current_run_started_at,current_run_ends_at,payload)
 VALUES(970071,$1,1,'running',$2,$3,$4) RETURNING id`, FishingTripActionID, at.Add(-4*time.Second), at, string(payload)).Scan(&queueID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO fishing_trips(id,user_id,queue_id,water_id,spot_id,loadout_id,strategy,safety_load_percent,phase_started_at,phase_ends_at,seed,started_at)
 VALUES('batch-proof',970071,$1,$2,$3,'proof','balanced',82,$4,$5,424242,$5)`, queueID, water.ID, spot.ID, at.Add(-4*time.Second), at); err != nil {
		t.Fatal(err)
	}
	if err = s.ensureAllPlayerAttributesTx(ctx, tx, 970071); err != nil {
		t.Fatal(err)
	}
	// The unchanged refund path ranges over a map. Existing stacks avoid
	// comparing incidental first-insert IDs from that unrelated map order.
	if _, err = tx.Exec(ctx, `INSERT INTO player_item_instances(instance_id,user_id,template_item_id,quantity) VALUES(1,970071,'wood',1),(2,970071,'stone',1)`); err != nil {
		t.Fatal(err)
	}
	for _, date := range []time.Time{at, at.Add(24 * time.Hour)} {
		tournament, err := s.ensureFishingTournamentTx(ctx, tx, date)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = tx.Exec(ctx, `UPDATE fishing_tournaments SET target_species_id=$2 WHERE id=$1`, tournament.ID, rules.Species[0].ID); err != nil {
			t.Fatal(err)
		}
	}
	return QueueRuntimeItem{QueueID: queueID, ActorType: ActionActorTypePlayer, ActionID: FishingTripActionID, TotalCount: 1, PayloadRaw: payload, PreserveScheduledTimeline: true, AuditPhase: "offline_fastforward"}
}

func TestOfflineFishingBatchPostgresFullReference(t *testing.T) {
	fixture, ctx, counter := recoveryPostgresStore(t)
	s := New(fixture.db, fixture.config, fixture.gameData)
	s.config.AchievementsEnabled = true
	for _, scenario := range []struct {
		mode  string
		limit int
	}{
		{"keep_all", 2000}, {"release_all", 2000}, {"record_only", 2000}, {"keep_all", 37}, {"effects", 37}, {"bait_empty", 3}, {"tackle_empty", 2000},
	} {
		t.Run(fmt.Sprintf("%s/%d", scenario.mode, scenario.limit), func(t *testing.T) {
			tx, err := s.db.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(context.Background())
			item := seedFishingBatchProof(t, ctx, s, tx, scenario.mode)
			if _, err = tx.Exec(ctx, `SAVEPOINT reference_start`); err != nil {
				t.Fatal(err)
			}
			var want map[string]string
			var wantResults []SettleResult
			var wantAudits []InventoryAuditEvent
			for _, optimized := range []bool{false, true} {
				if _, err = tx.Exec(ctx, `ALTER SEQUENCE fishing_catches_id_seq RESTART WITH 1; ALTER SEQUENCE player_item_instances_instance_id_seq RESTART WITH 100`); err != nil {
					t.Fatal(err)
				}
				s.registerInventoryAuditBuffer(tx)
				defer s.discardInventoryAuditBuffer(tx)
				batch := &offlineAchievementBatch{store: s, tx: tx, userID: 970071}
				runCtx := context.WithValue(ctx, offlineAchievementBatchKey{}, batch)
				current := item
				until := time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC)
				current.EffectSession, err = s.loadOperationEffectSessionTx(ctx, tx, 970071, until)
				if err != nil {
					t.Fatal(err)
				}
				var results []SettleResult
				before, began := counter.count.Load(), time.Now()
				for step := 0; step < 2000; step++ {
					var result SettleResult
					executor := newFishingExecutor(s)
					if optimized {
						result, err = executor.SettleFastForwardTx(runCtx, tx, 970071, current, until, FastForwardBudget{MaxIterations: scenario.limit})
					} else {
						result, err = executor.settleFishingReferenceTx(runCtx, tx, 970071, current, until, scenario.limit, FastForwardBudget{}, true)
					}
					if err != nil {
						t.Fatal(err)
					}
					if result.BudgetIterations <= 0 || result.BudgetIterations > scenario.limit || result.CancelAfterSettle {
						t.Fatalf("invalid progress: %+v", result)
					}
					results = append(results, result)
					current.PayloadRaw = result.NextPayload
					if result.Completed > 0 {
						break
					}
				}
				if len(results) == 0 || results[len(results)-1].Completed != 1 {
					t.Fatal("trip never completed")
				}
				if _, err = s.flushOperationEffectSessionTx(runCtx, tx, 970071, current.EffectSession, until); err != nil {
					t.Fatal(err)
				}
				if err = batch.flush(runCtx); err != nil {
					t.Fatal(err)
				}
				t.Logf("optimized=%t chunks=%d elapsed=%s direct_query_calls=%d", optimized, len(results), time.Since(began), counter.count.Load()-before)
				state := fishingBatchProofState(t, ctx, tx)
				audits := s.takeInventoryAuditEvents(tx)
				for i := range audits {
					// The legacy supply refund combines a map of template items;
					// its first representative is unspecified, not its item totals.
					if audits[i].Details["type"] == "fishing_supply_return" && len(audits[i].Items) > 1 {
						if audits[i].Items[audits[i].TemplateItem] <= 0 {
							t.Fatal("invalid refund audit representative")
						}
						audits[i].TemplateItem = primaryItemID(audits[i].Items)
					}
				}
				if !optimized {
					want, wantResults, wantAudits = state, results, audits
					if _, err = tx.Exec(ctx, `ROLLBACK TO SAVEPOINT reference_start`); err != nil {
						t.Fatal(err)
					}
				} else {
					for table, expected := range want {
						if state[table] != expected {
							t.Fatalf("%s state differs", table)
						}
					}
					if !reflect.DeepEqual(wantResults, results) {
						t.Fatal("per-chunk results or frozen checkpoints changed")
					}
					if !reflect.DeepEqual(wantAudits, audits) {
						t.Fatal("audit events changed")
					}
					runtime, _ := parseFishingExecutorRuntime(results[len(results)-1].NextPayload)
					if scenario.mode != "tackle_empty" && runtime.Summary.Landed == 0 {
						t.Fatal("fixture did not land fish")
					}
					if scenario.limit == 2000 && scenario.mode == "keep_all" && runtime.Summary.Landed < 128 {
						t.Fatal("fixture missed catch batch boundary")
					}
					if scenario.mode == "keep_all" && state["fishing_tournament_entries"] == "[]" {
						t.Fatal("fixture missed tournament entries")
					}
					if scenario.mode == "effects" && len(current.EffectSession.ChargeReceipts()) != 137 {
						t.Fatal("fixture missed charge exhaustion")
					}
					t.Logf("casts=%d landed=%d kept=%d scaleglow=%d stop=%s", runtime.Summary.Casts, runtime.Summary.Landed, runtime.Summary.Kept, runtime.Summary.ScaleglowFound, runtime.Summary.StopReason)
				}
			}
		})
	}
}

func TestOfflineFishingStatePostgresReferenceAndRollback(t *testing.T) {
	s, ctx, _ := recoveryPostgresStore(t)
	tx, err := s.db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	at := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	if _, err = tx.Exec(ctx, `INSERT INTO fishing_species_knowledge(user_id,species_id,encounters,catches,releases,first_caught_at,last_caught_at) VALUES(970071,'existing',7,3,1,$1,$1)`, at); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO fishing_records(user_id,species_id,ruleset_version,best_length_cm,best_weight_kg) VALUES(970071,'existing','proof',20,5)`); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, `SAVEPOINT state_start`); err != nil {
		t.Fatal(err)
	}
	batch := &fishingFastForwardCatchBatch{RulesetVersion: "proof", Knowledge: map[string]*fishingBatchKnowledgeState{
		"existing": {EncounterDelta: 2, CatchDelta: 1, ReleaseDelta: 1}, "new": {EncounterDelta: 1, FirstCaughtAt: &at, LastCaughtAt: &at}, "zero": {}, "nil": nil,
	}, Records: map[string]*fishingBatchRecordState{
		"existing": {Dirty: true, BestLengthCM: 20, BestWeightKG: 4}, "new": {Dirty: true, BestLengthCM: 1, BestWeightKG: 2}, "clean": {}, "nil": nil,
	}}
	var want map[string]string
	for _, optimized := range []bool{false, true} {
		if optimized {
			err = batch.flushState(ctx, tx, 970071)
		} else {
			err = batch.flushStateReference(ctx, tx, 970071)
		}
		if err != nil {
			t.Fatal(err)
		}
		state := fishingBatchProofState(t, ctx, tx)
		if !optimized {
			want = state
		} else if !reflect.DeepEqual(want, state) {
			t.Fatal("nullable timestamps, ties or non-dirty records changed")
		}
		if _, err = tx.Exec(ctx, `ROLLBACK TO SAVEPOINT state_start`); err != nil {
			t.Fatal(err)
		}
	}
	before := fishingBatchProofState(t, ctx, tx)
	batch.Records["new"].BestLengthCM = -1
	if err = batch.flushState(ctx, tx, 970071); err == nil {
		t.Fatal("expected second-statement constraint failure")
	}
	if _, err = tx.Exec(ctx, `ROLLBACK TO SAVEPOINT state_start`); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, fishingBatchProofState(t, ctx, tx)) {
		t.Fatal("failed record write leaked knowledge")
	}
	batch.Records["new"].BestLengthCM = 1
	if err = batch.flushState(ctx, tx, 970071); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(want, fishingBatchProofState(t, ctx, tx)) {
		t.Fatal("retry changed result")
	}
}

func TestOfflineFishingScaleglowPostgresExact(t *testing.T) {
	s, ctx, counter := recoveryPostgresStore(t)
	s.config.AchievementsEnabled = true
	definition := s.gameData.ItemByID("scaleglow_medium")
	if definition == nil {
		t.Fatal("missing scaleglow definition")
	}
	stackable := definition.Stackable
	defer func() { definition.Stackable = stackable }()
	for _, fallback := range []bool{false, true} {
		definition.Stackable = !fallback
		for _, count := range []int{0, 1, 128} {
			t.Run(fmt.Sprintf("fallback=%t/count=%d", fallback, count), func(t *testing.T) {
				tx, err := s.db.Begin(ctx)
				if err != nil {
					t.Fatal(err)
				}
				defer tx.Rollback(context.Background())
				if _, err = tx.Exec(ctx, `INSERT INTO player_item_instances(instance_id,user_id,template_item_id,quantity,archived_at,custom_data) VALUES
 (1,970071,'scaleglow_medium',7,NULL,'{}'),(2,970071,'scaleglow_medium',3,now(),'{}'),(3,970071,'scaleglow_medium',9,NULL,'{"name":"custom"}'),(4,970071,'scaleglow_medium',11,NULL,'{}')`); err != nil {
					t.Fatal(err)
				}
				if _, err = tx.Exec(ctx, `SAVEPOINT scaleglow_start`); err != nil {
					t.Fatal(err)
				}
				pending := make([]*fishingPendingCatch, count)
				for i := range pending {
					pending[i] = &fishingPendingCatch{CatchID: int64(i + 1), Scaleglow: count == 1 || i%3 != 0}
				}
				var want map[string]string
				var wantAudits []InventoryAuditEvent
				for _, optimized := range []bool{false, true} {
					if _, err = tx.Exec(ctx, `ALTER SEQUENCE player_item_instances_instance_id_seq RESTART WITH 100`); err != nil {
						t.Fatal(err)
					}
					s.registerInventoryAuditBuffer(tx)
					defer s.discardInventoryAuditBuffer(tx)
					batch := &offlineAchievementBatch{store: s, tx: tx, userID: 970071}
					runCtx := context.WithValue(ctx, offlineAchievementBatchKey{}, batch)
					began, before := time.Now(), counter.count.Load()
					if optimized {
						err = s.grantFishingScaleglowBatchTx(runCtx, tx, 970071, "proof", pending)
					} else {
						for _, row := range pending {
							if !row.Scaleglow {
								continue
							}
							err = s.LedgerGrantTemplateTx(runCtx, tx, 970071, "scaleglow_medium", 1, InventoryLedgerMeta{SourceSurface: "fishing", TargetSurface: "inventory", Details: map[string]any{"type": "fishing_scaleglow", "tripId": "proof", "catchId": row.CatchID}})
							if err != nil {
								break
							}
						}
					}
					if err != nil {
						t.Fatal(err)
					}
					if err = batch.flush(runCtx); err != nil {
						t.Fatal(err)
					}
					t.Logf("optimized=%t elapsed=%s direct_query_calls=%d", optimized, time.Since(began), counter.count.Load()-before)
					state, audits := fishingBatchProofState(t, ctx, tx), s.takeInventoryAuditEvents(tx)
					if !optimized {
						want, wantAudits = state, audits
					} else if !reflect.DeepEqual(want, state) || !reflect.DeepEqual(wantAudits, audits) {
						t.Fatal("stack identity, archive, customization, ledger or achievements changed")
					}
					if _, err = tx.Exec(ctx, `ROLLBACK TO SAVEPOINT scaleglow_start`); err != nil {
						t.Fatal(err)
					}
				}
			})
		}
	}
}

func TestOfflineFishingBatchPostgresRecoveryRollbackAndReplay(t *testing.T) {
	fixture, ctx, _ := recoveryPostgresStore(t)
	s := New(fixture.db, fixture.config, fixture.gameData)
	s.config.AchievementsEnabled = true
	tx, err := s.db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	s.registerInventoryAuditBuffer(tx)
	defer s.discardInventoryAuditBuffer(tx)
	item := seedFishingBatchProof(t, ctx, s, tx, "effects")
	if _, err = tx.Exec(ctx, `ALTER SEQUENCE player_item_instances_instance_id_seq RESTART WITH 100`); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	runtime, _ := parseFishingExecutorRuntime(item.PayloadRaw)
	start, target := time.UnixMilli(runtime.StartedAtMS), time.UnixMilli(runtime.PlannedEndAtMS)
	recovery, err := s.EnsureOfflineRecovery(ctx, 970071, start, target, target)
	if err != nil {
		t.Fatal(err)
	}
	specs := map[string]ActionSpec{FishingTripActionID: {ID: FishingTripActionID, DurationS: 1}}
	fingerprint := func() string {
		t.Helper()
		state := ""
		for _, table := range []string{"player_item_instances", "player_attributes", "player_action_queue", "player_operation_effect_grants", "player_operation_effect_charge_receipts", "player_operation_effect_state", "achievement_events", "player_offline_recoveries", "fishing_trips", "fishing_catches", "fishing_species_knowledge", "fishing_records", "fishing_tournament_entries"} {
			var hash string
			if err := s.db.QueryRow(ctx, `SELECT md5(COALESCE(jsonb_agg(to_jsonb(t) ORDER BY to_jsonb(t)::text),'[]')::text) FROM `+pgx.Identifier{table}.Sanitize()+` t WHERE user_id=970071`).Scan(&hash); err != nil {
				t.Fatal(err)
			}
			state += hash
		}
		return state
	}
	baseline := fingerprint()
	for _, fault := range []struct{ table, check string }{{"player_offline_recoveries", "revision<1"}, {"fishing_records", "best_length_cm<0"}, {"fishing_trips", "event_seq<1"}} {
		if _, err = s.db.Exec(ctx, `ALTER TABLE `+pgx.Identifier{fault.table}.Sanitize()+` ADD CONSTRAINT fishing_batch_fault CHECK(`+fault.check+`) NOT VALID`); err != nil {
			t.Fatal(err)
		}
		if err = s.AdvanceOfflineRecovery(ctx, recovery, specs); err == nil {
			t.Fatal("injected failure ignored", fault.table)
		}
		if fingerprint() != baseline {
			t.Fatal("partial fishing recovery escaped rollback", fault.table)
		}
		if _, err = s.db.Exec(ctx, `ALTER TABLE `+pgx.Identifier{fault.table}.Sanitize()+` DROP CONSTRAINT fishing_batch_fault`); err != nil {
			t.Fatal(err)
		}
	}
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() { defer wg.Done(); errs <- s.AdvanceOfflineRecovery(ctx, recovery, specs) }()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil && !errors.Is(err, ErrOfflineRecoveryBusy) {
			t.Fatal(err)
		}
	}
	first, err := s.GetOfflineRecovery(ctx, 970071)
	if err != nil || first.Revision != 1 || first.Summary.CompletedSteps != 0 {
		t.Fatal("phase budget confused with trip completions", first, err)
	}
	baseline = fingerprint()
	if err = s.AdvanceOfflineRecovery(ctx, recovery, specs); err != nil {
		t.Fatal(err)
	}
	if fingerprint() != baseline {
		t.Fatal("stale retry duplicated fish")
	}
	// Recreate process-local state, then resume only from durable checkpoints.
	s = New(fixture.db, s.config, fixture.gameData)
	for range 30 {
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
	complete, err := s.GetOfflineRecovery(ctx, 970071)
	if err != nil || complete.Status != "complete" || complete.Summary.CompletedSteps != 1 {
		t.Fatal("missing terminal trip receipt", complete, err)
	}
	var kept, instances, charges int64
	if err = s.db.QueryRow(ctx, `SELECT (SELECT count(*) FROM fishing_catches WHERE kept),(SELECT count(*) FROM player_item_instances WHERE template_item_id='fish_catch'),(SELECT count(*) FROM player_operation_effect_charge_receipts)`).Scan(&kept, &instances, &charges); err != nil {
		t.Fatal(err)
	}
	if kept < 128 || kept != instances || charges != 137 {
		t.Fatal("fish or effect receipts differ", kept, instances, charges)
	}
	t.Logf("durable recovery: fish=%d revisions=%d complete_steps=%d", kept, complete.Revision, complete.Summary.CompletedSteps)
}
