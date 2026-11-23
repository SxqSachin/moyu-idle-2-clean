package store

import (
	"context"
	"encoding/json"
	"math/rand"
	"reflect"
	"testing"
	"time"

	"mysecondwebgame/server/internal/catalog"
	"mysecondwebgame/server/internal/operationeffect"
	"mysecondwebgame/server/internal/operationeffect/transmute"
)

func TestOfflineAlchemyReadsPostgresSequentialParity(t *testing.T) {
	for _, action := range []string{"alchemy.transmute", "alchemy.convert"} {
		t.Run(action, func(t *testing.T) {
			fixture, ctx, counter := recoveryPostgresStore(t)
			s := New(fixture.db, fixture.config, fixture.gameData)
			// Fixed reward fixtures let the whole original executor run unchanged,
			// including its wall-clock RNG. Seeded random-outcome parity is below.
			input := "wood"
			var args any = alchemyTransmuteArgs{ItemID: input}
			if action == "alchemy.transmute" {
				rule := s.gameData.AlchemyTransmuteRuleForItemID(input)
				rule.FailChanceBase, rule.MinFailChance, rule.LuckyChanceBase, rule.MinAlchemyLevel = 0, 0, -1, 1
				rule.ReturnRatio = catalog.AlchemyReturnRatio{MinRatio: 1, MaxRatio: 1}
			} else {
				input = "crystal_chunk"
				group := s.gameData.AlchemyGroupByItemID(input)
				group.FailChanceBase, group.MinFailChance, group.MinAlchemyLevel = 0, 0, 1
				group.Entries = []catalog.AlchemyGroupEntry{{ItemID: input, Weight: 1}, {ItemID: "wood", Weight: 1}}
				group.UpgradeEntries = nil
				args = alchemyConvertArgs{SourceItemID: input, GroupID: group.ID}
			}
			var wantResults []SettleResult
			var wantInventory, wantAttributes []byte
			var wantReceipts []operationeffect.ChargeReceipt
			var wantQueries int64
			for _, cached := range []bool{false, true} {
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
				if err = s.GrantByTemplateTx(ctx, tx, 970071, input, 512); err != nil {
					t.Fatal(err)
				}
				if err = s.GrantByTemplateTx(ctx, tx, 970071, GoldCoinItemID, 1000000); err != nil {
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
				def.Lifetime = operationeffect.LifetimeDefinition{Kind: operationeffect.LifetimeCharges, Charges: 97}
				session, err := operationeffect.NewSession(s.operationEffectRegistry, []operationeffect.Grant{{ID: 71, Definition: def, Target: operationeffect.Target{Kind: operationeffect.TargetPlayer}, ValidFrom: at, InitialCharges: 97, RemainingCharges: 97}})
				if err != nil {
					t.Fatal(err)
				}
				if _, err = tx.Exec(ctx, `INSERT INTO player_action_queue(id,user_id,action_id,total_count,status,payload) VALUES(91,970071,$1,1000,'pending',$2)`, action, string(payload)); err != nil {
					t.Fatal(err)
				}
				item := QueueRuntimeItem{QueueID: 91, ActorType: ActionActorTypePlayer, ActionID: action, Spec: spec, TotalCount: 1000, PayloadRaw: payload, EffectSession: session, PreserveScheduledTimeline: true}
				if err = s.startQueueRunTx(ctx, tx, 970071, &item, at, queueRunStartOptions{}); err != nil {
					t.Fatal(err)
				}
				reads := &offlineAlchemyReads{}
				before, began := counter.count.Load(), time.Now()
				var results []SettleResult
				for i := 0; i < 513; i++ {
					// Like the real queue loop, each iteration reloads its checkpoint.
					if err = tx.QueryRow(ctx, `SELECT done_count,payload,current_run_started_at,current_run_ends_at FROM player_action_queue WHERE id=91`).Scan(&item.DoneCount, &item.PayloadRaw, &item.RunStartAt, &item.RunEndsAt); err != nil {
						t.Fatal(err)
					}
					item.WorkLoadoutRuntime = nil
					if cached {
						item.alchemyReads = reads.forItem(item)
					}
					result, err := e.SettleFastForwardTx(ctx, tx, 970071, item, *item.RunEndsAt, FastForwardBudget{MaxIterations: 1})
					if err != nil {
						t.Fatal(err)
					}
					results = append(results, result)
					if result.CancelAfterSettle {
						if result.DoneAfter != 512 || result.CancelReason != ActionQueueCancelReasonCostInsufficient {
							t.Fatal("wrong shortage boundary", result)
						}
						break
					}
					if _, err = tx.Exec(ctx, `UPDATE player_action_queue SET done_count=$1,payload=$2,current_run_started_at=$3,current_run_ends_at=$4 WHERE id=91`, result.DoneAfter, string(result.NextPayload), result.NextRunStartAt, result.NextRunEndsAt); err != nil {
						t.Fatal(err)
					}
				}
				queries := counter.count.Load() - before
				t.Logf("cached=%v completions=512 elapsed=%s SQL=%d", cached, time.Since(began), queries)
				var inventory, attributes []byte
				if err = tx.QueryRow(ctx, `SELECT jsonb_object_agg(template_item_id,qty) FROM (SELECT template_item_id,sum(quantity) qty FROM player_item_instances WHERE user_id=970071 GROUP BY template_item_id) q`).Scan(&inventory); err != nil {
					t.Fatal(err)
				}
				if err = tx.QueryRow(ctx, `SELECT jsonb_agg(jsonb_build_array(attr_key,attr_exp,base_level,final_value,lock_version) ORDER BY attr_key) FROM player_attributes WHERE user_id=970071`).Scan(&attributes); err != nil {
					t.Fatal(err)
				}
				if len(results) < 512 {
					t.Fatal("fixture did not settle the full queue")
				}
				if !cached {
					wantResults, wantInventory, wantAttributes, wantReceipts, wantQueries = results, inventory, attributes, session.ChargeReceipts(), queries
				} else if !reflect.DeepEqual(wantResults, results) || string(wantInventory) != string(inventory) || string(wantAttributes) != string(attributes) || !reflect.DeepEqual(wantReceipts, session.ChargeReceipts()) || queries >= wantQueries-1000 {
					t.Fatal("sequential settlement, attributes, receipts or query reduction changed")
				}
				if err = tx.Rollback(ctx); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

func TestOfflineAlchemyReadsPostgresSeededRewards(t *testing.T) {
	fixture, ctx, _ := recoveryPostgresStore(t)
	s := New(fixture.db, fixture.config, fixture.gameData)
	for _, action := range []string{"alchemy.transmute", "alchemy.convert"} {
		var want []byte
		for _, cached := range []bool{false, true} {
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
			input := "wood"
			var args any = alchemyTransmuteArgs{ItemID: input}
			if action == "alchemy.convert" {
				input = "crystal_chunk"
				args = alchemyConvertArgs{SourceItemID: input, GroupID: s.gameData.AlchemyGroupByItemID(input).ID}
			}
			if err = s.GrantByTemplateTx(ctx, tx, 970071, input, 128); err != nil {
				t.Fatal(err)
			}
			if err = s.GrantByTemplateTx(ctx, tx, 970071, GoldCoinItemID, 1000000); err != nil {
				t.Fatal(err)
			}
			e := &alchemyExecutor{store: s, actionID: action}
			payload, err := e.FreezeRunContextTx(ctx, tx, 970071, ActionActorTypePlayer, 0, ActionSpec{ID: action, MainType: "alchemy", DurationS: 8}, args, time.Now())
			if err != nil {
				t.Fatal(err)
			}
			item := QueueRuntimeItem{QueueID: 91, ActorType: ActionActorTypePlayer, ActionID: action, PayloadRaw: payload}
			if cached {
				item.alchemyReads = (&offlineAlchemyReads{}).forItem(item)
			}
			rng := rand.New(rand.NewSource(91827))
			var rewards []map[string]int64
			for i := 0; i < 128; i++ {
				item.DoneCount = i
				_, reward, err := e.settleOnceTx(ctx, tx, 970071, item, rng, false)
				if err != nil {
					t.Fatal(err)
				}
				rewards = append(rewards, reward)
			}
			raw, err := json.Marshal(rewards)
			if err != nil {
				t.Fatal(err)
			}
			if !cached {
				want = raw
			} else if string(want) != string(raw) {
				t.Fatal("RNG outcomes changed", action)
			}
			if err = tx.Rollback(ctx); err != nil {
				t.Fatal(err)
			}
		}
	}
}
