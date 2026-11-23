package store

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"
)

func TestOfflineCombatQueryPostgresEquivalenceAndScale(t *testing.T) {
	s, ctx, _ := recoveryPostgresStore(t)
	since := time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)
	until := since.Add(8 * time.Hour)
	if _, err := s.db.Exec(ctx, `INSERT INTO battle_reward_receipts(run_id,user_id,item_rewards,settled_at)
 SELECT n,970071,jsonb_build_object('wood',n%7+1,'gold_coin',n%11+1),$1::timestamptz+(n-45000)*interval '1 second' FROM generate_series(1,50000) n`, since); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(ctx, `INSERT INTO battle_reward_receipts(run_id,user_id,item_rewards,settled_at)
 SELECT n,970072,'{"wood":999999}'::jsonb,$1 FROM generate_series(1,50000) n`, since); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(ctx, `INSERT INTO player_offline_recoveries(user_id,started_at,target_at,resumed_at,status,pending_run_ids)
 VALUES(970071,$1::timestamptz-interval '1 day',$1,$1,'complete',ARRAY[45002,40000]::bigint[])`, since); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(ctx, `INSERT INTO player_warehouse_change_logs(user_id,event_type,template_item_id,item_totals,category,direction,details,created_at)
 VALUES(970071,'system_to_warehouse','wood','{"wood":999999}','battle','gain','{"runId":45001}',$1),
 (970071,'system_to_warehouse','wood','{"wood":2}','battle','gain','{"runId":999999}',$1)`, since); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"battle_reward_receipts", "player_offline_recoveries", "player_warehouse_change_logs"} {
		if _, err := s.db.Exec(ctx, "ANALYZE "+table); err != nil {
			t.Fatal(err)
		}
	}

	for _, pending := range [][]int64{{}, {40000, 45001, 45001, 45002, 44000}, {50000}} {
		selected := map[int64]bool{}
		for _, id := range pending {
			selected[id] = true
		}
		expected := map[string]int64{"wood": 2, "gold_coin": 0}
		for n := int64(1); n <= 50000; n++ {
			if n == 45002 || n == 40000 {
				continue
			}
			at := since.Add(time.Duration(n-45000) * time.Second)
			if (!at.Before(since) && at.Before(until)) || selected[n] {
				expected["wood"] += n%7 + 1
				expected["gold_coin"] += n%11 + 1
			}
		}
		var elapsed time.Duration
		for i := 0; i < 12; i++ {
			began := time.Now()
			got, err := s.GetOfflineCombatRewards(ctx, 970071, since, until, pending)
			if i > 1 {
				elapsed += time.Since(began)
			}
			if err != nil || !got.LegacyLogs || got.PendingRuns != 0 || !reflect.DeepEqual(expected, got.Items) {
				t.Fatal("combat accounting mismatch", expected, got, err)
			}
		}
		t.Logf("100000 receipts, pending=%v: query mean=%s", pending, elapsed/10)
		args := []any{int64(970071), since, until, pending, []string{BattleRunStatusCompiled, BattleRunStatusRunning, BattleRunStatusSettling}}
		var plan []byte
		if err := s.db.QueryRow(ctx, "EXPLAIN (ANALYZE,BUFFERS,FORMAT JSON) "+offlineCombatRewardsSQL, args...).Scan(&plan); err != nil {
			t.Fatal(err)
		}
		var parsed []map[string]any
		if err := json.Unmarshal(plan, &parsed); err != nil {
			t.Fatal(err)
		}
		indexes := map[string]bool{}
		var walk func(any)
		walk = func(v any) {
			switch v := v.(type) {
			case map[string]any:
				if name, ok := v["Index Name"].(string); ok {
					indexes[name] = true
				}
				for _, child := range v {
					walk(child)
				}
			case []any:
				for _, child := range v {
					walk(child)
				}
			}
		}
		walk(parsed[0])
		if !indexes["idx_battle_reward_receipt_user_time"] {
			t.Fatal("time range index not used", indexes)
		}
		t.Logf("execution_ms=%v indexes=%v", parsed[0]["Execution Time"], indexes)
	}
}
