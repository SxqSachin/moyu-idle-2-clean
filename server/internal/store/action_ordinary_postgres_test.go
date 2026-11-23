package store

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"mysecondwebgame/server/internal/operationeffect"
	"mysecondwebgame/server/internal/operationeffect/lifeaction"
)

type ordinaryProof struct {
	Done                 int
	Canceled             bool
	Start, End           time.Time
	Costs, Rewards       map[string]int64
	Inventory            []string
	Experience, Versions map[string]float64
	Receipts             []operationeffect.ChargeReceipt
	Calls                int
	Queries              int64
	Elapsed              time.Duration
}

func ordinaryProofEffects(t *testing.T, at time.Time, refund bool) *operationeffect.Session {
	t.Helper()
	registry := operationeffect.NewRegistry()
	if err := lifeaction.Register(registry); err != nil {
		t.Fatal(err)
	}
	var grants []operationeffect.Grant
	for i, kind := range []string{lifeaction.KindExtraMainOutputChance, lifeaction.KindMaterialRefundChance} {
		if !refund {
			break
		}
		def := operationEffectTestDefinition(operationeffect.LifetimeCharges)
		def.ID = fmt.Sprintf("ordinary-food-%d", i)
		def.Kind = kind
		def.Selector = operationeffect.Selector{}
		def.Stacking = operationeffect.StackingDefinition{Key: def.ID, Mode: operationeffect.StackAddCharges}
		def.Lifetime = operationeffect.LifetimeDefinition{Kind: operationeffect.LifetimeCharges, Charges: 1500}
		def.Params = json.RawMessage(`{"chance":0.37,"quantity":1}`)
		if i == 1 {
			def.Params = json.RawMessage(`{"chance":0.5,"refundRate":1}`)
		}
		grants = append(grants, operationeffect.Grant{ID: int64(800 + i), Definition: def, Target: operationeffect.Target{Kind: operationeffect.TargetPlayer}, ValidFrom: at, InitialCharges: 1500, RemainingCharges: 1500})
	}
	session, err := operationeffect.NewSession(registry, grants)
	if err != nil {
		t.Fatal(err)
	}
	return session
}

func runOrdinaryProof(t *testing.T, s *Store, counter *recoveryQueryCounter, spec ActionSpec, total, limit int, stock int64, food bool, untilRuns int) ordinaryProof {
	t.Helper()
	s.config.OrdinarySettlementEnabled = true
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	tx, err := s.db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	s.registerInventoryAuditBuffer(tx)
	defer s.discardInventoryAuditBuffer(tx)
	if err := s.lockUserTx(ctx, tx, 970071); err != nil {
		t.Fatal(err)
	}
	if err := s.ensureAllPlayerAttributesTx(ctx, tx, 970071); err != nil {
		t.Fatal(err)
	}
	for id := range spec.Costs {
		if err := s.AddInventoryItemTx(ctx, tx, 970071, id, stock); err != nil {
			t.Fatal(err)
		}
	}
	at := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	effects := ordinaryProofEffects(t, at, food)
	var executor QueueActionExecutor = newGenericExecutor(s)
	var args any = genericExecutorArgs{}
	if recipe := s.gameData.FishFoodRecipeByActionID(spec.ID); recipe != nil {
		executor = newFishFoodExecutor(s, *recipe)
		args = fishFoodExecutorArgs{IngredientMode: fishFoodIngredientModeAuto, FishIngredientInstances: map[string][]int64{}}
		for _, requirement := range recipe.FishIngredients {
			for i := 0; i < (total+4)*requirement.Quantity; i++ {
				fish := FishInstanceData{SpeciesID: requirement.SpeciesID, LengthCM: float64(20 + i%31), WeightKG: float64(1+i%7) / 3}
				if i%5 == 0 {
					fish.Genes.Mutations = []string{"shiny"}
				}
				raw, err := json.Marshal(fish)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := tx.Exec(ctx, `INSERT INTO player_item_instances(user_id,template_item_id,quantity,state,durability,max_durability,fish_data) VALUES(970071,$1,1,'inventory',100,100,$2::jsonb)`, fishFoodCatchTemplateItemID, string(raw)); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	item := QueueRuntimeItem{QueueID: 881, ActorType: ActionActorTypePlayer, ActionID: spec.ID, Spec: spec, TotalCount: total, EffectSession: effects, PreserveScheduledTimeline: true, AuditPhase: "offline_fastforward"}
	item.PayloadRaw, err = executor.FreezeRunContextTx(ctx, tx, 970071, item.ActorType, 0, spec, args, at)
	if err != nil {
		t.Fatal(err)
	}
	op := queueOperationEffectOperation(operationeffect.DomainLifeAction, item, at, 1, item.PayloadRaw)
	first, err := effects.Commit(effects.Prepare(op))
	if err != nil {
		t.Fatal(err)
	}
	item.OperationEffects = &first
	item.PayloadRaw, err = executor.StartRunTx(ctx, tx, 970071, item, at)
	if err != nil {
		t.Fatal(err)
	}
	item.PayloadRaw, err = setQueueOperationEffectSnapshot(item.PayloadRaw, first)
	if err != nil {
		t.Fatal(err)
	}
	var envelope queueRuntimeEnvelope
	if err := json.Unmarshal(item.PayloadRaw, &envelope); err != nil {
		t.Fatal(err)
	}
	// Both executions represent the SAME committed start, not two new queues.
	envelope.OrdinarySettlement.Seed = strings.Repeat("43", 32)
	item.PayloadRaw, err = json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	start, end := at, at.Add(parseQueueRuntimeDuration(item.PayloadRaw))
	item.RunStartAt, item.RunEndsAt = &start, &end
	until := at.Add(time.Duration(untilRuns)*parseQueueRuntimeDuration(item.PayloadRaw) + time.Millisecond)
	proof := ordinaryProof{Costs: map[string]int64{}, Rewards: map[string]int64{}, Experience: map[string]float64{}, Versions: map[string]float64{}}
	before, began := counter.count.Load(), time.Now()
	for item.DoneCount < total && !item.RunEndsAt.After(until) {
		batchLimit := limit
		if limit < 0 {
			batchLimit = 1 + (proof.Calls*73+19)%137
		}
		result, err := executor.SettleFastForwardTx(ctx, tx, 970071, item, until, FastForwardBudget{MaxIterations: batchLimit})
		if err != nil {
			t.Fatal(err)
		}
		if result.Completed <= 0 {
			t.Fatal("no progress")
		}
		proof.Calls++
		proof.Done = result.DoneAfter
		proof.Canceled = result.CancelAfterSettle
		for id, n := range result.ItemCosts {
			proof.Costs[id] += n
		}
		for id, n := range result.ItemRewards {
			proof.Rewards[id] += n
		}
		item.DoneCount = result.DoneAfter
		if item.DoneCount >= total || result.CancelAfterSettle {
			break
		}
		// Round-trip the checkpoint, as PostgreSQL/recovery workers do.
		item.PayloadRaw = append([]byte(nil), result.NextPayload...)
		a, b := result.NextRunStartAt, result.NextRunEndsAt
		item.RunStartAt, item.RunEndsAt = &a, &b
	}
	proof.Start, proof.End = *item.RunStartAt, *item.RunEndsAt
	proof.Queries = counter.count.Load() - before
	proof.Elapsed = time.Since(began)
	proof.Receipts = effects.ChargeReceipts()
	rows, err := tx.Query(ctx, `SELECT template_item_id,quantity,enhance_level,base_snapshot::text,affixes::text,durability,max_durability,state,COALESCE(fish_data,'{}'::jsonb)::text FROM player_item_instances WHERE user_id=970071`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var id, base, affixes, state, fish string
		var qty, level, dur, maxDur int64
		if err := rows.Scan(&id, &qty, &level, &base, &affixes, &dur, &maxDur, &state, &fish); err != nil {
			t.Fatal(err)
		}
		proof.Inventory = append(proof.Inventory, fmt.Sprintf("%s|%d|%d|%s|%s|%d|%d|%s|%s", id, qty, level, base, affixes, dur, maxDur, state, fish))
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	rows.Close()
	sort.Strings(proof.Inventory)
	rows, err = tx.Query(ctx, `SELECT attr_key,attr_exp,lock_version FROM player_attributes WHERE user_id=970071`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var key string
		var exp, version float64
		if err := rows.Scan(&key, &exp, &version); err != nil {
			t.Fatal(err)
		}
		proof.Experience[key] = exp
		proof.Versions[key] = version
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	rows.Close()
	return proof
}

func TestOrdinaryPostgresPartitions(t *testing.T) {
	for _, scenario := range []string{"fixed_cost", "refund_prefix", "independent_equipment", "partial_prepaid", "level_boundary", "fish_auto"} {
		t.Run(scenario, func(t *testing.T) {
			s, _, counter := recoveryPostgresStore(t)
			s = New(s.db, s.config, s.gameData)
			n := int64(1)
			spec := ActionSpec{ID: "logging", MainType: "collecting", DurationS: 2, Costs: map[string]int64{"proof_input": 2}, ItemReward: map[string]ActionItemRewardSpec{"proof_output": {Count: &n}}, MainOutput: "proof_output"}
			total, until, stock, food := 1200, 1200, int64(10000), false
			switch scenario {
			case "refund_prefix":
				stock = 23
				food = true
				spec.Costs["proof_second"] = 3
			case "independent_equipment":
				spec.ItemReward = BuildActionSpecMap(s.gameData)["craft_base2_mistwarden_charm"].ItemReward
				spec.MainOutput = "base2_mistwarden_charm"
				food = true
				total = 320
				until = 320
			case "partial_prepaid":
				until = 173
				food = true
			case "level_boundary":
				spec.StatusExpReward = map[string]float64{"collecting": 0.123456}
				total = 90
				until = 90
			case "fish_auto":
				spec = BuildActionSpecMap(s.gameData)[s.gameData.FishFoodRecipes[0].ActionID]
				spec.StatusExpReward = nil
				total = 80
				until = 73
				food = true
			}
			one := runOrdinaryProof(t, s, counter, spec, total, 1, stock, food, until)
			for _, limit := range []int{2048, -1} {
				batch := runOrdinaryProof(t, s, counter, spec, total, limit, stock, food, until)
				t.Logf("limit=%d done=%d calls=%d queries=%d elapsed=%s; single calls=%d queries=%d elapsed=%s", limit, batch.Done, batch.Calls, batch.Queries, batch.Elapsed, one.Calls, one.Queries, one.Elapsed)
				if batch.Done != one.Done || batch.Canceled != one.Canceled || !reflect.DeepEqual(batch.Costs, one.Costs) || !reflect.DeepEqual(batch.Rewards, one.Rewards) || !reflect.DeepEqual(batch.Inventory, one.Inventory) || !reflect.DeepEqual(batch.Experience, one.Experience) || !reflect.DeepEqual(batch.Versions, one.Versions) || !reflect.DeepEqual(batch.Receipts, one.Receipts) {
					t.Fatalf("partition mismatch: done=%d/%d cancel=%v/%v costs=%v/%v rewards=%v/%v inventoryEqual=%v xpEqual=%v versionsEqual=%v receiptsEqual=%v", batch.Done, one.Done, batch.Canceled, one.Canceled, batch.Costs, one.Costs, batch.Rewards, one.Rewards, reflect.DeepEqual(batch.Inventory, one.Inventory), reflect.DeepEqual(batch.Experience, one.Experience), reflect.DeepEqual(batch.Versions, one.Versions), reflect.DeepEqual(batch.Receipts, one.Receipts))
				}
				if scenario == "partial_prepaid" && (!batch.Start.Equal(one.Start) || !batch.End.Equal(one.End)) {
					t.Fatal("partial start window changed")
				}
				if scenario == "fixed_cost" && limit == 2048 && batch.Queries > one.Queries/10 {
					t.Fatal("ordinary fast path did not batch database work")
				}
			}
		})
	}
}

func TestOrdinaryPostgres50000(t *testing.T) {
	s, _, counter := recoveryPostgresStore(t)
	s = New(s.db, s.config, s.gameData)
	n := int64(1)
	p := 0.04
	spec := ActionSpec{ID: "logging", MainType: "collecting", DurationS: 2, Costs: map[string]int64{"proof_input": 2}, ItemReward: map[string]ActionItemRewardSpec{"proof_output": {Count: &n, Percent: &p}}, MainOutput: "proof_output"}
	proof := runOrdinaryProof(t, s, counter, spec, 50000, 2048, 100000, false, 50000)
	if proof.Done != 50000 || proof.Canceled || proof.Costs["proof_input"] != 99998 || proof.Rewards["proof_output"] <= 0 {
		t.Fatalf("invalid 50000 settlement: %+v", proof)
	}
	if proof.Calls > 30 || proof.Queries > 700 {
		t.Fatalf("unbounded SQL: calls=%d queries=%d", proof.Calls, proof.Queries)
	}
	t.Logf("50000 actions: %d calls, %d queries, %s, output=%d", proof.Calls, proof.Queries, proof.Elapsed, proof.Rewards["proof_output"])
}
