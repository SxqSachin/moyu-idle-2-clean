package store

import (
	"os"
	"strings"
	"testing"
)

func TestOfflineRecoveryPostgresOperationsSQL(t *testing.T) {
	s, ctx, _ := recoveryPostgresStore(t)
	if _, err := s.db.Exec(ctx, `INSERT INTO player_offline_recoveries
 (user_id,started_at,target_at,resumed_at,status,background,revision,attempts,error_code,summary,pending_run_ids)
 VALUES
 (970071,now()-interval '1 hour',now(),now(),'stalled',false,7,10,'OFFLINE_RECOVERY_RETRY','{"completedSteps":3,"itemRewards":{"wood":3}}',ARRAY[123::bigint]),
 (970072,now()-interval '1 hour',now(),now(),'stalled',true,9,0,'no_progress','{"stalledQueueId":55}',ARRAY[]::bigint[]),
 (970071,now(),now(),now(),'running',false,11,2,'OFFLINE_RECOVERY_RETRY','{}',ARRAY[]::bigint[]),
 (970072,now(),now(),now(),'complete',false,13,0,'','{}',ARRAY[]::bigint[])`); err != nil {
		t.Fatal(err)
	}
	snapshot := func(omitMutable bool) string {
		t.Helper()
		expression := "to_jsonb(r)"
		if omitMutable {
			expression += " - ARRAY['status','revision','attempts','error_code','available_at']"
		}
		var raw string
		if err := s.db.QueryRow(ctx, `SELECT jsonb_agg(`+expression+` ORDER BY id)::text FROM player_offline_recoveries r`).Scan(&raw); err != nil {
			t.Fatal(err)
		}
		return raw
	}
	before, preserved := snapshot(false), snapshot(true)
	for _, filename := range []string{"offline_recovery_scan.sql", "offline_recovery_retry_stalled.sql"} {
		sql, err := os.ReadFile("../../schema/operations/" + filename)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = s.db.Exec(ctx, string(sql)); err != nil {
			t.Fatal(filename, err)
		}
		if snapshot(false) != before {
			t.Fatal("scan or preview modified player data", filename)
		}
	}
	sql, err := os.ReadFile("../../schema/operations/offline_recovery_retry_stalled.sql")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(string(sql), "\nROLLBACK;\n") {
		t.Fatal("operations retry must default to rollback")
	}
	apply := strings.TrimSuffix(string(sql), "\nROLLBACK;\n") + "\nCOMMIT;\n"
	if _, err = s.db.Exec(ctx, apply); err != nil {
		t.Fatal(err)
	}
	var valid int
	if err = s.db.QueryRow(ctx, `SELECT count(*) FROM player_offline_recoveries WHERE
 (revision IN (8,10) AND status='running' AND attempts=0 AND error_code='') OR
 (revision=11 AND status='running' AND attempts=2 AND error_code='OFFLINE_RECOVERY_RETRY') OR
 (revision=13 AND status='complete' AND attempts=0 AND error_code='')`).Scan(&valid); err != nil || valid != 4 {
		t.Fatal("retry changed a running/complete task or failed to fence stalled tasks", valid, err)
	}
	if snapshot(true) != preserved {
		t.Fatal("retry modified immutable task data")
	}
	after := snapshot(false)
	if _, err = s.db.Exec(ctx, apply); err != nil {
		t.Fatal(err)
	}
	if snapshot(false) != after {
		t.Fatal("replaying operations retry modified an already restarted task")
	}
}

func TestOfflineRecoveryPostgresOperationsSQLSkipsBusyAndBoundsBatch(t *testing.T) {
	s, ctx, _ := recoveryPostgresStore(t)
	if _, err := s.db.Exec(ctx, `INSERT INTO player_offline_recoveries(user_id,started_at,target_at,resumed_at,status)
 SELECT 970071,now(),now(),now(),'stalled' FROM generate_series(1,103)`); err != nil {
		t.Fatal(err)
	}
	busy, err := s.db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer busy.Rollback(ctx)
	var id int64
	if err = busy.QueryRow(ctx, `SELECT id FROM player_offline_recoveries ORDER BY id LIMIT 1 FOR UPDATE`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	sql, err := os.ReadFile("../../schema/operations/offline_recovery_retry_stalled.sql")
	if err != nil {
		t.Fatal(err)
	}
	apply := strings.TrimSuffix(string(sql), "\nROLLBACK;\n") + "\nCOMMIT;\n"
	if _, err = s.db.Exec(ctx, apply); err != nil {
		t.Fatal(err)
	}
	var restarted, pending int
	if err = s.db.QueryRow(ctx, `SELECT count(*) FILTER(WHERE status='running' AND revision=1),count(*) FILTER(WHERE status='stalled' AND revision=0) FROM player_offline_recoveries`).Scan(&restarted, &pending); err != nil || restarted != 100 || pending != 3 {
		t.Fatal("retry did not respect bounded batch", restarted, pending, err)
	}
	var status string
	if err = busy.QueryRow(ctx, `SELECT status FROM player_offline_recoveries WHERE id=$1`, id).Scan(&status); err != nil || status != "stalled" {
		t.Fatal("retry did not skip busy task", status, err)
	}
}
