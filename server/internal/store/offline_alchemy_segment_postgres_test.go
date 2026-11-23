package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"mysecondwebgame/server/internal/catalog"
	"mysecondwebgame/server/internal/operationeffect"
	"mysecondwebgame/server/internal/operationeffect/transmute"
)

func startAlchemySegmentFixture(t *testing.T, ctx context.Context, s *Store, tx pgx.Tx, action string, total, owned int, expGain float64, changeGrants ...func([]operationeffect.Grant) []operationeffect.Grant) (QueueRuntimeItem, *alchemyExecutor) {
	t.Helper()
	if _, err := tx.Exec(ctx, `ALTER SEQUENCE player_item_instances_instance_id_seq RESTART WITH 10000`); err != nil {
		t.Fatal(err)
	}
	if err := s.ensureAllPlayerAttributesTx(ctx, tx, 970071); err != nil {
		t.Fatal(err)
	}
	input := "wood"
	var args any = alchemyTransmuteArgs{ItemID: input}
	if action == "alchemy.convert" {
		input = "crystal_chunk"
		group := s.gameData.AlchemyGroupByItemID(input)
		group.AlchemyExp = expGain
		group.Entries = []catalog.AlchemyGroupEntry{{ItemID: input, Weight: 1}, {ItemID: "wood", Weight: 3}, {ItemID: "stone", Weight: 2}}
		group.UpgradeEntries = []catalog.AlchemyGroupEntry{{ItemID: "iron_ore", Weight: 1}}
		group.UpgradeChanceBase = 0.35
		args = alchemyConvertArgs{SourceItemID: input, GroupID: group.ID}
	} else {
		s.gameData.AlchemyTransmuteRuleForItemID(input).AlchemyExp = expGain
	}
	if err := s.GrantByTemplateTx(ctx, tx, 970071, input, int64(owned)); err != nil {
		t.Fatal(err)
	}
	if err := s.GrantByTemplateTx(ctx, tx, 970071, GoldCoinItemID, 1000000); err != nil {
		t.Fatal(err)
	}
	// Active and archived stacks exercise original consumption order; outputs
	// initially absent exercise first-occurrence instance ID ordering.
	if _, err := tx.Exec(ctx, `UPDATE player_item_instances SET quantity=quantity-7 WHERE user_id=970071 AND template_item_id=$1`, input); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO player_item_instances(user_id,template_item_id,quantity,state,archived_at) VALUES(970071,$1,7,'inventory','2026-01-01')`, input); err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC)
	spec := ActionSpec{ID: action, MainType: "alchemy", DurationS: 8}
	e := &alchemyExecutor{store: s, actionID: action}
	payload, err := e.FreezeRunContextTx(ctx, tx, 970071, ActionActorTypePlayer, 0, spec, args, at)
	if err != nil {
		t.Fatal(err)
	}
	def := operationEffectTestDefinition(operationeffect.LifetimeCharges)
	def.Domain, def.Kind, def.Params = operationeffect.DomainAlchemy, transmute.Speed, []byte(`{"durationSeconds":1}`)
	if action == "alchemy.convert" {
		def = s.operationEffectDefinitions["food_gentle_current_grill_v2"]
	}
	def.Selector = operationeffect.Selector{ActionIDs: []string{action}}
	def.Lifetime = operationeffect.LifetimeDefinition{Kind: operationeffect.LifetimeCharges, Charges: 2000}
	grants := []operationeffect.Grant{{ID: 71, Definition: def, Target: operationeffect.Target{Kind: operationeffect.TargetPlayer}, ValidFrom: at, InitialCharges: 2000, RemainingCharges: 2000}}
	if action == "alchemy.transmute" {
		gold := def
		gold.ID, gold.Kind, gold.Params = "segment_gold", transmute.Gold, []byte(`{"bonusRate":0.1}`)
		grants = append(grants, operationeffect.Grant{ID: 72, Definition: gold, Target: operationeffect.Target{Kind: operationeffect.TargetPlayer}, ValidFrom: at, InitialCharges: 2000, RemainingCharges: 2000})
	}
	for _, change := range changeGrants {
		grants = change(grants)
	}
	for _, grant := range grants {
		raw, err := json.Marshal(grant.Definition)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO player_operation_effect_grants(id,user_id,definition_id,ruleset_version,domain,effect_kind,stacking_key,priority,target_kind,lifetime_kind,valid_from,valid_until,initial_charges,remaining_charges,definition_snapshot_jsonb)
VALUES($1,970071,$2,$3,$4,$5,$6,$7,'player',$8,$9,$10,$11,$12,$13)`, grant.ID, grant.Definition.ID, grant.Definition.RulesetVersion, grant.Definition.Domain, grant.Definition.Kind, grant.Definition.Stacking.Key, grant.Definition.Priority, grant.Definition.Lifetime.Kind, grant.ValidFrom, grant.ValidUntil, grant.InitialCharges, grant.RemainingCharges, string(raw)); err != nil {
			t.Fatal(err)
		}
	}
	session, err := operationeffect.NewSession(s.operationEffectRegistry, grants)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO player_action_queue(id,user_id,action_id,total_count,status,payload) VALUES(91,970071,$1,$2,'pending',$3)`, action, total, string(payload)); err != nil {
		t.Fatal(err)
	}
	item := QueueRuntimeItem{QueueID: 91, ActorType: ActionActorTypePlayer, ActionID: action, Spec: spec, TotalCount: total, PayloadRaw: payload, EffectSession: session, PreserveScheduledTimeline: true,
		alchemyRandomSeed: func(ordinal int) int64 { return int64(ordinal)*91827 + 17 }}
	if err := s.startQueueRunTx(ctx, tx, 970071, &item, at, queueRunStartOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := tx.QueryRow(ctx, `SELECT done_count,payload,current_run_started_at,current_run_ends_at FROM player_action_queue WHERE id=91`).Scan(&item.DoneCount, &item.PayloadRaw, &item.RunStartAt, &item.RunEndsAt); err != nil {
		t.Fatal(err)
	}
	item.OperationEffects = nil
	if _, err := s.flushOperationEffectSessionTx(ctx, tx, 970071, session, at); err != nil {
		t.Fatal(err)
	}
	return item, e
}

func TestOfflineAlchemySegmentPostgresParity(t *testing.T) {
	for _, action := range []string{"alchemy.transmute", "alchemy.convert"} {
		for _, scenario := range []struct {
			name              string
			total, owned, due int
			exp               float64
		}{
			{"stable", 1000, 1000, 512, 0.0001},
			{"complete", 512, 512, 512, 0.0001},
			{"shortage", 1000, 217, 512, 0.0001},
			{"levels", 180, 180, 180, 1.25},
			{"charge_boundary", 1000, 1000, 512, 0.0001},
			{"duration_boundary", 1000, 1000, 512, 0.0001},
			{"future_effect", 1000, 1000, 512, 0.0001},
			{"partition", 1000, 1000, 512, 0.0001},
			{"zero_exp", 128, 128, 128, 0},
			{"capped_exp", 128, 128, 128, 0.0001},
			{"all_fail", 128, 128, 128, 0.0001},
		} {
			t.Run(action+"/"+scenario.name, func(t *testing.T) {
				fixture, baseCtx, counter := recoveryPostgresStore(t)
				s := New(fixture.db, fixture.config, fixture.gameData)
				s.config.AchievementsEnabled = true
				var want []byte
				var legacyQueries int64
				for _, batch := range []bool{false, true} {
					tx, err := s.db.Begin(baseCtx)
					if err != nil {
						t.Fatal(err)
					}
					defer tx.Rollback(context.Background())
					s.registerInventoryAuditBuffer(tx)
					defer s.discardInventoryAuditBuffer(tx)
					achievements := &offlineAchievementBatch{store: s, tx: tx, userID: 970071}
					ctx := context.WithValue(baseCtx, offlineAchievementBatchKey{}, achievements)
					item, executor := startAlchemySegmentFixture(t, ctx, s, tx, action, scenario.total, scenario.owned, scenario.exp, func(grants []operationeffect.Grant) []operationeffect.Grant {
						if scenario.name == "charge_boundary" {
							for i := range grants {
								grants[i].InitialCharges, grants[i].RemainingCharges, grants[i].Definition.Lifetime.Charges = 97, 97, 97
							}
						}
						if scenario.name == "duration_boundary" {
							i := len(grants) - 1
							end := grants[i].ValidFrom.Add(97 * time.Second)
							grants[i].ValidUntil, grants[i].InitialCharges, grants[i].RemainingCharges = &end, 0, 0
							grants[i].Definition.Lifetime = operationeffect.LifetimeDefinition{Kind: operationeffect.LifetimeDuration, DurationSeconds: 97}
							grants[i].Definition.Stacking.Mode = operationeffect.StackExtendDuration
							grants[i].Definition.Stacking.MaxCharges = 0
						}
						if scenario.name == "future_effect" {
							g := grants[len(grants)-1]
							g.ID, g.ValidFrom = 73, g.ValidFrom.Add(97*time.Second)
							g.Definition.Priority += 10
							if action == "alchemy.transmute" {
								grants[len(grants)-1].Definition.Params = []byte(`{"bonusRate":0.04}`)
							} else {
								g.Definition.Params = []byte(`{"lossReductionRate":0.1}`)
							}
							grants = append(grants, g)
						}
						return grants
					})
					if scenario.name == "capped_exp" {
						cap := s.attributeExpProfile.capTotalExp["alchemy_level"]
						if _, err := s.applyAttrExpLockedResultTx(ctx, tx, 970071, map[string]float64{"alchemy_level": cap}); err != nil {
							t.Fatal(err)
						}
					}
					if scenario.name == "all_fail" {
						item.alchemyRandomSeed = func(int) int64 { return 2 }
						if action == "alchemy.transmute" {
							rule := s.gameData.AlchemyTransmuteRuleForItemID("wood")
							rule.FailChanceBase, rule.MinFailChance = 1, 1
						} else {
							group := s.gameData.AlchemyGroupByItemID("crystal_chunk")
							group.FailChanceBase, group.MinFailChance = 1, 1
						}
					}
					timing := resolveQueueRunTiming(item, *item.RunEndsAt, parseQueueRuntimeDuration(item.PayloadRaw))
					now := completedAtForQueueTiming(timing, scenario.due)
					summary := ActionFastForwardSummary{ItemCosts: map[string]int64{}, ItemRewards: map[string]int64{}, AttrExp: map[string]float64{"alchemy_level": 0.1}}
					actor := map[string]*ActionActorSummary{}
					var last SettleResult
					segments := 0
					began, before := time.Now(), counter.count.Load()
					reads := &offlineAlchemyReads{}
					for loops := 0; loops < scenario.total+2; loops++ {
						if err := tx.QueryRow(ctx, `SELECT done_count,payload,current_run_started_at,current_run_ends_at FROM player_action_queue WHERE id=91`).Scan(&item.DoneCount, &item.PayloadRaw, &item.RunStartAt, &item.RunEndsAt); err != nil {
							t.Fatal(err)
						}
						if item.RunEndsAt.After(now) {
							break
						}
						item.alchemyReads = reads.forItem(item)
						var result SettleResult
						var handled bool
						if batch {
							limit := 1024
							if scenario.name == "partition" {
								limit = 37
							}
							result, handled, err = executor.trySettleExactSegmentTx(ctx, tx, 970071, item, now, FastForwardBudget{MaxIterations: limit})
							if handled {
								segments++
							}
						}
						if err != nil {
							t.Fatal(err)
						}
						if !handled {
							limit := 1024
							_, frozen := queueOperationEffectSnapshot(item.PayloadRaw)
							probe := enrichOperationEffectContext(executor, queueOperationEffectOperation(operationeffect.DomainAlchemy, item, *item.RunEndsAt, item.DoneCount+2, item.PayloadRaw), item.PayloadRaw)
							if frozen || item.EffectSession.HasMatching(probe, now) {
								limit = 1
							}
							result, err = executor.SettleFastForwardTx(ctx, tx, 970071, item, now, FastForwardBudget{MaxIterations: limit})
						}
						if err != nil {
							t.Fatal(err)
						}
						last = result
						mergeInt64Map(summary.ItemCosts, result.ItemCosts)
						mergeInt64Map(summary.ItemRewards, result.ItemRewards)
						accumulateSettlementAttrExp(summary.AttrExp, result, false)
						accumulateActionActorSummary(actor, item.ActorType, 0, result)
						summary.CompletedSteps += int64(result.Completed)
						if result.CancelAfterSettle || result.DoneAfter == item.TotalCount {
							break
						}
						if result.Completed < 1 {
							t.Fatal("stalled", result)
						}
						if _, err := tx.Exec(ctx, `UPDATE player_action_queue SET done_count=$1,payload=$2,current_run_started_at=$3,current_run_ends_at=$4 WHERE id=91`, result.DoneAfter, string(result.NextPayload), result.NextRunStartAt, result.NextRunEndsAt); err != nil {
							t.Fatal(err)
						}
					}
					if err := achievements.flush(ctx); err != nil {
						t.Fatal(err)
					}
					if _, err := s.flushOperationEffectSessionTx(ctx, tx, 970071, item.EffectSession, now); err != nil {
						t.Fatal(err)
					}
					queries := counter.count.Load() - before
					t.Logf("batch=%v completed=%d segments=%d elapsed=%s SQL=%d", batch, summary.CompletedSteps, segments, time.Since(began), queries)
					var inventory, attrs, discoveries, events, effects, receipts json.RawMessage
					for _, query := range []struct {
						sql string
						dst *json.RawMessage
					}{
						{`SELECT COALESCE(jsonb_agg(to_jsonb(i)-'created_at'-'updated_at' ORDER BY instance_id),'[]') FROM player_item_instances i WHERE user_id=970071`, &inventory},
						{`SELECT jsonb_agg(to_jsonb(a)-'updated_at' ORDER BY attr_key) FROM player_attributes a WHERE user_id=970071`, &attrs},
						{`SELECT COALESCE(jsonb_agg(jsonb_build_array(kind,id) ORDER BY kind,id),'[]') FROM alchemy_discoveries WHERE user_id=970071`, &discoveries},
						{`SELECT COALESCE(jsonb_agg(jsonb_build_array(kind,counter_key,delta,payload) ORDER BY kind,counter_key,delta,payload::text),'[]') FROM achievement_events WHERE user_id=970071`, &events},
						{`SELECT jsonb_agg(jsonb_build_array(id,remaining_charges) ORDER BY id) FROM player_operation_effect_grants`, &effects},
						{`SELECT jsonb_agg(jsonb_build_array(grant_id,operation_key,started_at) ORDER BY grant_id,operation_key) FROM player_operation_effect_charge_receipts`, &receipts},
					} {
						if err := tx.QueryRow(ctx, query.sql).Scan(query.dst); err != nil {
							t.Fatal(err)
						}
					}
					// Single completion fields are represented by the cumulative
					// summary. Compare every remaining continuation/cancellation field.
					last.Completed, last.ItemCosts, last.ItemRewards, last.AttrExp, last.exactSingleAttrExp = 0, nil, nil, nil, nil
					audits := s.takeInventoryAuditEvents(tx)
					for i := range audits {
						// The legacy map iteration chooses an arbitrary representative
						// for a multi-item reserve event. Its full Items map is exact.
						if audits[i].Details["type"] == "alchemy_reserve_cost" && len(audits[i].Items) > 1 {
							if audits[i].Items[audits[i].TemplateItem] <= 0 || audits[i].Items[fmt.Sprint(audits[i].Details["itemId"])] <= 0 {
								t.Fatal("invalid audit representative")
							}
							audits[i].TemplateItem = primaryItemID(audits[i].Items)
							audits[i].Details["itemId"] = audits[i].TemplateItem
						}
					}
					raw, err := json.Marshal([]any{summary, actor, last, inventory, attrs, discoveries, events, audits, item.EffectSession.ChargeReceipts(), effects, receipts})
					if err != nil {
						t.Fatal(err)
					}
					var canonical any
					if err := json.Unmarshal(raw, &canonical); err != nil {
						t.Fatal(err)
					}
					raw, _ = json.Marshal(canonical)
					if !batch {
						want, legacyQueries = raw, queries
					} else {
						if string(raw) != string(want) {
							t.Fatalf("state/continuation/ledger mismatch\nwant %s\ngot  %s", want, raw)
						}
						if (scenario.name == "stable" || scenario.name == "complete" || scenario.name == "shortage" || scenario.name == "partition") && (segments == 0 || queries >= legacyQueries/4) {
							t.Fatalf("not a real batch: %d vs %d SQL, %d segments", queries, legacyQueries, segments)
						}
					}
					if err := tx.Rollback(ctx); err != nil {
						t.Fatal(err)
					}
				}
			})
		}
	}
}

func TestOfflineAlchemySegmentPostgresRecoveryAtomicity(t *testing.T) {
	fixture, ctx, counter := recoveryPostgresStore(t)
	s := New(fixture.db, fixture.config, fixture.gameData)
	s.config.ActionQueueFastForwardMaxSteps = 512
	rule := s.gameData.AlchemyTransmuteRuleForItemID("wood")
	rule.FailChanceBase, rule.MinFailChance, rule.LuckyChanceBase, rule.MinAlchemyLevel = 0, 0, -1, 1
	rule.ReturnRatio = catalog.AlchemyReturnRatio{MinRatio: 1, MaxRatio: 1}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	s.registerInventoryAuditBuffer(tx)
	defer s.discardInventoryAuditBuffer(tx)
	item, _ := startAlchemySegmentFixture(t, ctx, s, tx, "alchemy.transmute", 1536, 1536, 0.0001)
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	s.config.AchievementsEnabled = true
	var notifications atomic.Int64
	s.achievementNotify = func(context.Context) { notifications.Add(1) }
	target := item.RunStartAt.Add(1536 * time.Second)
	recovery, err := s.EnsureOfflineRecovery(ctx, 970071, *item.RunStartAt, target, target)
	if err != nil {
		t.Fatal(err)
	}
	specs := map[string]ActionSpec{item.ActionID: item.Spec}
	began, queries := time.Now(), counter.count.Load()
	if err := s.AdvanceOfflineRecovery(ctx, recovery, specs); err != nil {
		t.Fatal(err)
	}
	first, err := s.GetOfflineRecovery(ctx, 970071)
	if err != nil {
		t.Fatal(err)
	}
	baseGold := max(int64(1), int64(math.Floor(s.gameData.ItemByID("wood").EconomyBaseValue)))
	gold := baseGold + transmute.BonusGold(baseGold, .1)
	if first.Revision != 1 || first.Summary.CompletedSteps != 512 || first.Summary.ItemRewards[GoldCoinItemID] != gold*512 || notifications.Load() != 1 {
		t.Fatal("wrong first checkpoint", first, notifications.Load())
	}
	fingerprint := func() string {
		t.Helper()
		state := ""
		for _, table := range []string{"player_item_instances", "player_attributes", "player_action_queue", "player_operation_effect_grants", "player_operation_effect_charge_receipts", "player_operation_effect_state", "achievement_events", "player_offline_recoveries"} {
			var hash string
			query := `SELECT md5(COALESCE(jsonb_agg(to_jsonb(t) ORDER BY to_jsonb(t)::text),'[]')::text) FROM ` + pgx.Identifier{table}.Sanitize() + ` t WHERE user_id=970071`
			if err := s.db.QueryRow(ctx, query).Scan(&hash); err != nil {
				t.Fatal(err)
			}
			state += hash
		}
		return state
	}
	baseline := fingerprint()
	for _, fault := range []struct{ table, check string }{
		{"player_offline_recoveries", "revision<2"},
		{"player_operation_effect_grants", "remaining_charges>=1487"},
		{"achievement_events", "kind<>'obtain_item'"},
		{"player_item_instances", fmt.Sprintf("template_item_id<>'gold_coin' OR quantity<=%d", 1000000+gold*512)},
	} {
		if _, err := s.db.Exec(ctx, `ALTER TABLE `+pgx.Identifier{fault.table}.Sanitize()+` ADD CONSTRAINT segment_injected_failure CHECK(`+fault.check+`) NOT VALID`); err != nil {
			t.Fatal(err)
		}
		if err := s.AdvanceOfflineRecovery(ctx, first, specs); err == nil {
			t.Fatal("fault was ignored", fault.table)
		}
		if fingerprint() != baseline || notifications.Load() != 1 {
			t.Fatal("partial settlement escaped rollback", fault.table)
		}
		if _, err := s.db.Exec(ctx, `ALTER TABLE `+pgx.Identifier{fault.table}.Sanitize()+` DROP CONSTRAINT segment_injected_failure`); err != nil {
			t.Fatal(err)
		}
	}
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() { defer wg.Done(); errs <- s.AdvanceOfflineRecovery(ctx, first, specs) }()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil && !errors.Is(err, ErrOfflineRecoveryBusy) {
			t.Fatal(err)
		}
	}
	second, err := s.GetOfflineRecovery(ctx, 970071)
	if err != nil || second.Revision != 2 || second.Summary.CompletedSteps != 1024 {
		t.Fatal("concurrent duplicate settlement", second, err)
	}
	baseline = fingerprint()
	if err := s.AdvanceOfflineRecovery(ctx, recovery, specs); err != nil {
		t.Fatal(err)
	}
	if fingerprint() != baseline {
		t.Fatal("stale revision replay changed state")
	}
	if err := s.AdvanceOfflineRecovery(ctx, second, specs); err != nil {
		t.Fatal(err)
	}
	complete, err := s.GetOfflineRecovery(ctx, 970071)
	if err != nil || complete.Status != "complete" || complete.Summary.CompletedSteps != 1536 || complete.Summary.ItemRewards[GoldCoinItemID] != gold*1536 {
		t.Fatal("wrong terminal receipt", complete, err)
	}
	var remaining, receipts, completedEvents, distinctSources, reserved, goldOwned int64
	if err := s.db.QueryRow(ctx, `SELECT min(remaining_charges) FROM player_operation_effect_grants`).Scan(&remaining); err != nil {
		t.Fatal(err)
	}
	if err := s.db.QueryRow(ctx, `SELECT count(*) FROM player_operation_effect_charge_receipts`).Scan(&receipts); err != nil {
		t.Fatal(err)
	}
	if err := s.db.QueryRow(ctx, `SELECT sum(delta),count(DISTINCT payload->>'SourceKey') FROM achievement_events WHERE kind='complete_action' AND delta=512 AND payload->>'SourceKey' LIKE 'offline_batch:recovery:%'`).Scan(&completedEvents, &distinctSources); err != nil {
		t.Fatal(err)
	}
	if err := s.db.QueryRow(ctx, `SELECT COALESCE(sum(quantity),0) FROM player_item_instances WHERE template_item_id='wood'`).Scan(&reserved); err != nil {
		t.Fatal(err)
	}
	if err := s.db.QueryRow(ctx, `SELECT sum(quantity) FROM player_item_instances WHERE template_item_id='gold_coin'`).Scan(&goldOwned); err != nil {
		t.Fatal(err)
	}
	if remaining != 464 || receipts != 3072 || completedEvents != 1536 || distinctSources != 3 || reserved != 0 || goldOwned != 1000000+gold*1536 {
		t.Fatal("terminal inventory/food/recovery achievement mismatch", remaining, receipts, completedEvents, distinctSources, reserved, goldOwned)
	}
	t.Logf("real recovery 1536 completions + 4 rollbacks + concurrent/stale retry: elapsed=%s SQL=%d revisions=%d", time.Since(began), counter.count.Load()-queries, complete.Revision)
}

func TestOfflineAlchemySegmentPostgresFallback(t *testing.T) {
	s, ctx, _ := recoveryPostgresStore(t)
	for _, name := range []string{"changed_effect", "shared_charges", "other_lane", "legacy_sampler", "one_step", "unknown_exp", "instance", "feedback", "invalid_reward"} {
		t.Run(name, func(t *testing.T) {
			tx, err := s.db.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(context.Background())
			s.registerInventoryAuditBuffer(tx)
			defer s.discardInventoryAuditBuffer(tx)
			action := "alchemy.transmute"
			if name == "feedback" || name == "invalid_reward" {
				action = "alchemy.convert"
			}
			item, executor := startAlchemySegmentFixture(t, ctx, s, tx, action, 100, 100, 0.0001)
			budget := FastForwardBudget{MaxIterations: 100}
			now := item.RunEndsAt.Add(24 * time.Hour)
			switch name {
			case "changed_effect":
				item.EffectSession = operationeffect.NewEmptySession()
			case "shared_charges":
				def := operationEffectTestDefinition(operationeffect.LifetimeCharges)
				def.Domain, def.Kind, def.Params = operationeffect.DomainAlchemy, transmute.Speed, []byte(`{"durationSeconds":1}`)
				def.Selector = operationeffect.Selector{ActionIDs: []string{action}}
				from := *item.RunStartAt
				def.AllowedTargets = []operationeffect.TargetKind{operationeffect.TargetPlayerAndKitties}
				session, err := operationeffect.NewSession(s.operationEffectRegistry, []operationeffect.Grant{{ID: 71, Definition: def, Target: operationeffect.Target{Kind: operationeffect.TargetPlayerAndKitties}, ValidFrom: from, InitialCharges: 100, RemainingCharges: 100}})
				if err != nil {
					t.Fatal(err)
				}
				item.EffectSession = session
			case "other_lane":
				_, err = tx.Exec(ctx, `INSERT INTO player_action_queue(user_id,action_id,total_count,status) VALUES(970071,'other',100,'running')`)
			case "legacy_sampler":
				var payload map[string]any
				if err := json.Unmarshal(item.PayloadRaw, &payload); err != nil {
					t.Fatal(err)
				}
				delete(payload, "operationEffects")
				item.PayloadRaw, err = json.Marshal(payload)
				item.EffectSession = nil
			case "one_step":
				budget.MaxIterations = 1
			case "unknown_exp":
				r := parseAlchemyRuntime(item.PayloadRaw)
				r.ExtraAttrExp["strength"] = 1
				item.PayloadRaw, err = setQueueRuntimePayload(item.PayloadRaw, alchemyRuntime{DurationMs: r.DurationMs, ExtraAttrExp: r.ExtraAttrExp})
			case "instance":
				item.PayloadRaw, err = setAlchemyReservedCosts(item.PayloadRaw, alchemyTransmuteArgs{ItemID: "wood", InstanceID: 10000}, map[string]int64{"wood": 1})
			case "feedback":
				group := s.gameData.AlchemyGroupByItemID("crystal_chunk")
				group.UpgradeEntries = []catalog.AlchemyGroupEntry{{ItemID: "crystal_chunk", Weight: 1}}
			case "invalid_reward":
				group := s.gameData.AlchemyGroupByItemID("crystal_chunk")
				group.FailChanceBase, group.MinFailChance = 0, 0
				group.Entries = []catalog.AlchemyGroupEntry{{ItemID: "crystal_chunk", Weight: 1}, {ItemID: "wood", Weight: 0}}
				group.UpgradeEntries = nil
			}
			if err != nil {
				t.Fatal(err)
			}
			state := func() string {
				t.Helper()
				var database json.RawMessage
				if err := tx.QueryRow(ctx, `SELECT jsonb_build_array(
				 (SELECT jsonb_agg(to_jsonb(i) ORDER BY instance_id) FROM player_item_instances i WHERE user_id=970071),
				 (SELECT jsonb_agg(to_jsonb(a) ORDER BY attr_key) FROM player_attributes a WHERE user_id=970071),
				 (SELECT jsonb_agg(to_jsonb(q) ORDER BY id) FROM player_action_queue q WHERE user_id=970071),
				 (SELECT jsonb_agg(to_jsonb(g) ORDER BY id) FROM player_operation_effect_grants g WHERE user_id=970071),
				 (SELECT jsonb_agg(to_jsonb(r) ORDER BY grant_id,operation_key) FROM player_operation_effect_charge_receipts r WHERE user_id=970071))`).Scan(&database); err != nil {
					t.Fatal(err)
				}
				raw, err := json.Marshal([]any{database, s.auditBuffers[tx].events, item.EffectSession.ChargeReceipts()})
				if err != nil {
					t.Fatal(err)
				}
				return string(raw)
			}
			before := state()
			_, handled, err := executor.trySettleExactSegmentTx(ctx, tx, 970071, item, now, budget)
			if err != nil || handled {
				t.Fatal("unsafe contract entered batch", handled, err)
			}
			if before != state() {
				t.Fatal("fallback changed authoritative state or ledger")
			}
		})
	}
}

func TestOfflineAlchemySegmentPostgresLaneBoundary(t *testing.T) {
	fixture, ctx, _ := recoveryPostgresStore(t)
	s := New(fixture.db, fixture.config, fixture.gameData)
	for _, offset := range []int{1, 2, 3, 37, 201} {
		t.Run(fmt.Sprint(offset), func(t *testing.T) {
			tx, err := s.db.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(context.Background())
			s.registerInventoryAuditBuffer(tx)
			defer s.discardInventoryAuditBuffer(tx)
			item, executor := startAlchemySegmentFixture(t, ctx, s, tx, "alchemy.transmute", 100, 100, .0001)
			boundary := item.RunStartAt.Add(time.Duration(offset) * time.Second)
			if _, err := tx.Exec(ctx, `INSERT INTO player_action_queue(user_id,action_id,total_count,status,current_run_ends_at) VALUES(970071,'other',100,'running',$1)`, boundary); err != nil {
				t.Fatal(err)
			}
			result, handled, err := executor.trySettleExactSegmentTx(ctx, tx, 970071, item, item.RunStartAt.Add(100*time.Second), FastForwardBudget{MaxIterations: 100})
			if err != nil {
				t.Fatal(err)
			}
			want := min(offset-1, 100)
			if want < 2 {
				if handled {
					t.Fatal("batched at/beyond another lane's first boundary")
				}
			} else if !handled || result.Completed != want || !result.CompletedAt.Before(boundary) {
				t.Fatal("segment changed scheduler ordering", offset, handled, result.Completed, result.CompletedAt, boundary)
			}
		})
	}
	t.Run("combat_paused_lane", func(t *testing.T) {
		tx, err := s.db.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(context.Background())
		s.registerInventoryAuditBuffer(tx)
		defer s.discardInventoryAuditBuffer(tx)
		item, executor := startAlchemySegmentFixture(t, ctx, s, tx, "alchemy.transmute", 100, 100, .0001)
		if _, err := tx.Exec(ctx, `INSERT INTO player_kitties(id,user_id,name,status) VALUES(55,970071,'segment-test','in_life_queue')`); err != nil {
			t.Fatal(err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO player_action_queue(user_id,actor_type,actor_id,action_id,total_count,status) VALUES(970071,'kitty',55,'other',100,'running')`); err != nil {
			t.Fatal(err)
		}
		now := item.RunStartAt.Add(100 * time.Second)
		if _, handled, err := executor.trySettleExactSegmentTx(ctx, tx, 970071, item, now, FastForwardBudget{MaxIterations: 100}); err != nil || handled {
			t.Fatal("active unknown-time lane was skipped", handled, err)
		}
		if _, err := tx.Exec(ctx, `UPDATE player_kitties SET status='in_combat' WHERE id=55`); err != nil {
			t.Fatal(err)
		}
		result, handled, err := executor.trySettleExactSegmentTx(ctx, tx, 970071, item, now, FastForwardBudget{MaxIterations: 100})
		if err != nil || !handled || result.Completed != 100 {
			t.Fatal("combat-paused lane blocked batching", handled, result.Completed, err)
		}
	})
}

func TestOfflineAlchemySegmentPostgresLargeRecovery(t *testing.T) {
	fixture, ctx, counter := recoveryPostgresStore(t)
	s := New(fixture.db, fixture.config, fixture.gameData)
	s.config.ActionQueueFastForwardMaxSteps = 10000
	rule := s.gameData.AlchemyTransmuteRuleForItemID("wood")
	rule.FailChanceBase, rule.MinFailChance, rule.LuckyChanceBase, rule.MinAlchemyLevel = 0, 0, -1, 1
	rule.ReturnRatio = catalog.AlchemyReturnRatio{MinRatio: 1, MaxRatio: 1}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	s.registerInventoryAuditBuffer(tx)
	defer s.discardInventoryAuditBuffer(tx)
	const count = 8192
	item, _ := startAlchemySegmentFixture(t, ctx, s, tx, "alchemy.transmute", count, count, .0001, func(grants []operationeffect.Grant) []operationeffect.Grant {
		for i := range grants {
			grants[i].InitialCharges, grants[i].RemainingCharges, grants[i].Definition.Lifetime.Charges = count+1, count+1, count+1
		}
		return grants
	})
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	s.config.AchievementsEnabled = true
	target := item.RunStartAt.Add(count * time.Second)
	recovery, err := s.EnsureOfflineRecovery(ctx, 970071, *item.RunStartAt, target, target)
	if err != nil {
		t.Fatal(err)
	}
	began, before := time.Now(), counter.count.Load()
	for range 8 {
		if err := s.AdvanceOfflineRecovery(ctx, recovery, map[string]ActionSpec{item.ActionID: item.Spec}); err != nil {
			t.Fatal(err)
		}
		recovery, err = s.GetOfflineRecovery(ctx, 970071)
		if err != nil {
			t.Fatal(err)
		}
		if recovery.Status == "complete" {
			break
		}
	}
	baseGold := max(int64(1), int64(math.Floor(s.gameData.ItemByID("wood").EconomyBaseValue)))
	wantGold := (baseGold + transmute.BonusGold(baseGold, .1)) * count
	if recovery.Status != "complete" || recovery.Summary.CompletedSteps != count || recovery.Summary.ItemRewards[GoldCoinItemID] != wantGold {
		t.Fatal("large recovery incomplete or incorrect", recovery.Status, recovery.Summary.CompletedSteps, recovery.Summary.ItemRewards)
	}
	t.Logf("8192 completions with food, achievements, receipts, COMMIT and audit: elapsed=%s core_SQL=%d chunks=%d", time.Since(began), counter.count.Load()-before, recovery.Revision)
}
