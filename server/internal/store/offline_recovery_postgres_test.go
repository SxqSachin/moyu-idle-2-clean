package store

import (
	"context"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type recoveryQueryCounter struct{ count atomic.Int64 }

func (q *recoveryQueryCounter) TraceQueryStart(ctx context.Context, _ *pgx.Conn, _ pgx.TraceQueryStartData) context.Context {
	q.count.Add(1)
	return ctx
}
func (q *recoveryQueryCounter) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}

func recoveryPostgresStore(t *testing.T) (*Store, context.Context, *recoveryQueryCounter) {
	t.Helper()
	dsn := os.Getenv("OFFLINE_RECOVERY_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set OFFLINE_RECOVERY_TEST_DATABASE_URL for isolated recovery proof")
	}
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	if host := cfg.ConnConfig.Host; host != "localhost" && host != "127.0.0.1" && host != "::1" {
		t.Fatal("loopback database required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)
	schema := fmt.Sprintf("offline_recovery_test_%d", time.Now().UnixNano())
	admin, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = admin.Exec(ctx, `CREATE DATABASE `+pgx.Identifier{schema}.Sanitize()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, err := admin.Exec(context.Background(), `DROP DATABASE `+pgx.Identifier{schema}.Sanitize())
		if err != nil {
			t.Error(err)
		}
		admin.Close(context.Background())
	})
	cfg.ConnConfig.Database = schema
	counter := &recoveryQueryCounter{}
	cfg.ConnConfig.Tracer = counter
	cfg.MaxConns = 4
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	baseline, err := os.ReadFile("../../schema/init.sql")
	if err != nil {
		t.Fatal(err)
	}
	source := string(baseline)
	source = source[strings.Index(source, "-- === Identity"):]
	if _, err = pool.Exec(ctx, source); err != nil {
		t.Fatal(err)
	}
	migration, err := os.ReadFile("../../schema/migrations/20260907_offline_recovery.sql")
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if _, err = pool.Exec(ctx, string(migration)); err != nil {
			t.Fatal(err)
		}
	}
	backgroundMigration, err := os.ReadFile("../../schema/migrations/20260908_background_offline_recovery.sql")
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if _, err = pool.Exec(ctx, string(backgroundMigration)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = pool.Exec(ctx, `INSERT INTO users(id,username,password_hash) VALUES(970071,'recovery-one','test'),(970072,'recovery-two','test')`); err != nil {
		t.Fatal(err)
	}
	st := fishCuisineTestStore(t)
	st.db = pool
	st.gameData.CatteryFurniture = nil
	st.config.ActionQueueFastForwardMaxSteps = 1000
	st.config.ActionQueueFastForwardMaxExec = time.Second
	st.config.AchievementsEnabled = false
	return st, ctx, counter
}

func TestOfflineRecoveryPostgresAtomicRefreshAndReplay(t *testing.T) {
	s, ctx, counter := recoveryPostgresStore(t)
	s.config.AchievementsEnabled = true
	var notifications atomic.Int64
	var notifiedBeforeCommit atomic.Bool
	s.achievementNotify = func(ctx context.Context) {
		var visible bool
		if err := s.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM player_offline_recoveries WHERE user_id=970071 AND revision>0)`).Scan(&visible); err != nil || !visible {
			notifiedBeforeCommit.Store(true)
		}
		notifications.Add(1)
	}
	const userID int64 = 970071
	target := time.Now().Truncate(time.Second)
	start := target.Add(-8 * time.Hour)
	one := int64(1)
	spec := ActionSpec{ID: "recovery_test", DurationS: 2, ItemReward: map[string]ActionItemRewardSpec{"wood": {Count: &one}}}
	specs := map[string]ActionSpec{spec.ID: spec}
	if _, err := s.db.Exec(ctx, `INSERT INTO player_action_queue(user_id,action_id,total_count,status,current_run_started_at,current_run_ends_at,payload)
 VALUES($1,$2,20000,'running',$3,$4,'{"actionId":"recovery_test","executorId":"generic","runtime":{"durationMs":2000}}')`, userID, spec.ID, start, start.Add(2*time.Second)); err != nil {
		t.Fatal(err)
	}
	r, err := s.EnsureOfflineRecovery(ctx, userID, start, target, target)
	if err != nil {
		t.Fatal(err)
	}
	again, err := s.EnsureOfflineRecovery(ctx, userID, start, target.Add(time.Minute), target.Add(time.Minute))
	if err != nil || again.ID != r.ID || !again.TargetAt.Equal(target) {
		t.Fatal("refresh replaced original window", again, err)
	}
	if _, _, _, _, err = s.AdvanceActionQueueWithSummary(ctx, userID, specs); !errors.Is(err, ErrOfflineRecoveryPending) {
		t.Fatal("online worker bypassed recovery", err)
	}
	started := time.Now()
	before := counter.count.Load()
	if err = s.AdvanceOfflineRecovery(ctx, r, specs); err != nil {
		t.Fatal(err)
	}
	first, err := s.GetOfflineRecovery(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	if first.Revision != 1 || first.Summary.CompletedSteps != 1000 || first.Summary.ItemRewards["wood"] != 1000 {
		t.Fatalf("bad first receipt: %+v", first)
	}
	inventory := func() int64 {
		t.Helper()
		var qty int64
		if err := s.db.QueryRow(ctx, `SELECT COALESCE(sum(quantity),0)::bigint FROM player_item_instances WHERE user_id=$1 AND template_item_id='wood'`, userID).Scan(&qty); err != nil {
			t.Fatal(err)
		}
		return qty
	}
	if inventory() != 1000 {
		t.Fatal("first credit missing")
	}
	var firstEventCount int
	if err = s.db.QueryRow(ctx, `SELECT count(*) FROM achievement_events`).Scan(&firstEventCount); err != nil || firstEventCount == 0 || notifications.Load() != 1 || notifiedBeforeCommit.Load() {
		t.Fatal("committed recovery did not persist/notify its events", firstEventCount, notifications.Load(), err)
	}
	// A fault at the final checkpoint must roll back earlier grants and progress.
	if _, err = s.db.Exec(ctx, `ALTER TABLE player_offline_recoveries ADD CONSTRAINT injected_failure CHECK(revision<2)`); err != nil {
		t.Fatal(err)
	}
	if err = s.AdvanceOfflineRecovery(ctx, first, specs); err == nil {
		t.Fatal("expected checkpoint failure")
	}
	if inventory() != 1000 {
		t.Fatal("grant escaped failed transaction")
	}
	var afterFailureEventCount int
	if err = s.db.QueryRow(ctx, `SELECT count(*) FROM achievement_events`).Scan(&afterFailureEventCount); err != nil || afterFailureEventCount != firstEventCount || notifications.Load() != 1 {
		t.Fatal("achievement events/notification escaped checkpoint rollback", afterFailureEventCount, notifications.Load(), err)
	}
	if _, err = s.db.Exec(ctx, `ALTER TABLE player_offline_recoveries DROP CONSTRAINT injected_failure`); err != nil {
		t.Fatal(err)
	}
	if _, err = s.db.Exec(ctx, `ALTER TABLE achievement_events ADD CONSTRAINT injected_event_failure CHECK(delta<1000) NOT VALID`); err != nil {
		t.Fatal(err)
	}
	if err = s.AdvanceOfflineRecovery(ctx, first, specs); err == nil {
		t.Fatal("event batch persistence failure was swallowed")
	}
	if inventory() != 1000 || notifications.Load() != 1 {
		t.Fatal("event batch failure did not roll back settlement")
	}
	if _, err = s.db.Exec(ctx, `ALTER TABLE achievement_events DROP CONSTRAINT injected_event_failure`); err != nil {
		t.Fatal(err)
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
	if inventory() != 2000 {
		t.Fatal("simultaneous refresh duplicated grant")
	}
	if err = s.AdvanceOfflineRecovery(ctx, r, specs); err != nil {
		t.Fatal(err)
	}
	if inventory() != 2000 {
		t.Fatal("stale revision replayed")
	}
	for range 20 {
		current, err := s.GetOfflineRecovery(ctx, userID)
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
	complete, err := s.GetOfflineRecovery(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	if complete.Status != "complete" || complete.Summary.CompletedSteps != 14400 || inventory() != 14400 {
		t.Fatal("overnight totals mismatch", complete.Status, complete.Summary.CompletedSteps, inventory())
	}
	if err = s.RetryOfflineRecoveryLater(ctx, r, context.DeadlineExceeded); err != nil {
		t.Fatal(err)
	}
	retry, err := s.GetOfflineRecovery(ctx, userID)
	if err != nil || retry.Status != "complete" || retry.Revision != complete.Revision {
		t.Fatal("late commit error rewrote receipt", retry, err)
	}
	var ends time.Time
	if err = s.db.QueryRow(ctx, `SELECT current_run_ends_at FROM player_action_queue WHERE user_id=$1`, userID).Scan(&ends); err != nil || !ends.Equal(target.Add(2*time.Second)) {
		t.Fatal("processing time erased", ends, err)
	}
	t.Logf("8h/14400 actions incl rollback+replay: elapsed=%s queries=%d committed_chunks=%d", time.Since(started), counter.count.Load()-before, complete.Revision)
	if err = s.AcknowledgeOfflineRecovery(ctx, 970072, complete.ID, complete.Revision); err != nil {
		t.Fatal(err)
	}
	if _, err = s.GetOfflineRecovery(ctx, userID); err != nil {
		t.Fatal("foreign acknowledgement hid receipt", err)
	}
	if err = s.AcknowledgeOfflineRecovery(ctx, userID, complete.ID, complete.Revision); err != nil {
		t.Fatal(err)
	}
	if _, err = s.GetOfflineRecovery(ctx, userID); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatal("acknowledgement not persisted", err)
	}
	ackReplay, err := s.EnsureOfflineRecovery(ctx, userID, start, target.Add(time.Minute), target.Add(time.Minute))
	if err != nil || ackReplay.ID != complete.ID || ackReplay.Status != "complete" {
		t.Fatal("acknowledged window replayed", ackReplay, err)
	}
}

func TestOfflineRecoveryPostgresBattleReceiptAndLateSettlement(t *testing.T) {
	s, ctx, _ := recoveryPostgresStore(t)
	const userID int64 = 970071
	until := time.Now().Add(time.Second)
	since := until.Add(-time.Hour)
	var roomID int64
	if err := s.db.QueryRow(ctx, `INSERT INTO battle_rooms(owner_id,dungeon_id) VALUES($1,'test') RETURNING id`, userID).Scan(&roomID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(ctx, `INSERT INTO battle_runs(id,room_id,status) VALUES(123,$1,$2)`, roomID, BattleRunStatusRunning); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(ctx, `INSERT INTO battle_room_members(room_id,user_id) VALUES($1,$2)`, roomID, userID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(ctx, `UPDATE battle_rooms SET current_run_id=123 WHERE id=$1`, roomID); err != nil {
		t.Fatal(err)
	}
	// A current battle can start after the action queue's capped cutoff.
	recovery, err := s.EnsureOfflineRecovery(ctx, userID, since, until.Add(-30*time.Minute), until)
	if err != nil || len(recovery.PendingRunIDs) != 1 || recovery.PendingRunIDs[0] != 123 {
		t.Fatal("playing run not captured", recovery.PendingRunIDs, err)
	}
	pending, err := s.GetOfflineCombatRewards(ctx, userID, since, until, recovery.PendingRunIDs)
	if err != nil || pending.PendingRuns != 1 {
		t.Fatal("playing run not pending", pending, err)
	}
	if err = s.AdvanceOfflineRecovery(ctx, recovery, map[string]ActionSpec{}); err != nil {
		t.Fatal(err)
	}
	complete, err := s.GetOfflineRecovery(ctx, userID)
	if err != nil || complete.Status != "complete" {
		t.Fatal("action receipt did not complete", complete, err)
	}
	if err = s.AcknowledgeOfflineRecovery(ctx, userID, complete.ID, complete.Revision); err != nil {
		t.Fatal(err)
	}
	if _, err = s.GetOfflineRecovery(ctx, userID); err != nil {
		t.Fatal("early ACK discarded pending battle rewards", err)
	}
	rewards := map[string]int64{"wood": 7}
	grant := func(fail bool) error {
		return s.withTx(ctx, func(tx pgx.Tx) error {
			if err := s.lockUserTx(ctx, tx, userID); err != nil {
				return err
			}
			fresh, err := s.recordBattleRewardReceiptTx(ctx, tx, 123, userID, rewards)
			if err != nil || !fresh {
				return err
			}
			if err = s.GrantByTemplateTx(ctx, tx, userID, "wood", 7); err != nil {
				return err
			}
			if fail {
				return errors.New("injected grant failure")
			}
			return nil
		})
	}
	if err := grant(true); err == nil {
		t.Fatal("expected rollback")
	}
	var count int
	if err := s.db.QueryRow(ctx, `SELECT count(*) FROM battle_reward_receipts`).Scan(&count); err != nil || count != 0 {
		t.Fatal("receipt escaped rollback", err)
	}
	if err := grant(false); err != nil {
		t.Fatal(err)
	}
	if err := grant(false); err != nil {
		t.Fatal(err)
	}
	var stock int64
	if err := s.db.QueryRow(ctx, `SELECT sum(quantity)::bigint FROM player_item_instances WHERE user_id=$1`, userID).Scan(&stock); err != nil || stock != 7 {
		t.Fatal("repeated receipt duplicated grant", stock, err)
	}
	result, err := s.GetOfflineCombatRewards(ctx, userID, since, until, nil)
	if err != nil || result.Items["wood"] != 7 || result.LegacyLogs {
		t.Fatal("warehouse-log-independent result", result, err)
	}
	// A post-cutoff receipt remains attributed to the captured in-flight run.
	if _, err = s.db.Exec(ctx, `UPDATE battle_reward_receipts SET settled_at=$1`, until.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	result, err = s.GetOfflineCombatRewards(ctx, userID, since, until, []int64{123})
	if err != nil || result.Items["wood"] != 7 || result.PendingRuns != 0 {
		t.Fatal("late settlement disappeared", result, err)
	}
	if _, err = s.db.Exec(ctx, `INSERT INTO player_warehouse_change_logs(user_id,event_type,template_item_id,item_totals,category,direction,details,created_at)
 VALUES($1,'system_to_warehouse','wood','{"wood":7}','battle','gain','{"runId":123}',$2),
 ($1,'system_to_warehouse','wood','{"wood":2}','battle','gain','{"runId":99}',$2)`, userID, since.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	result, err = s.GetOfflineCombatRewards(ctx, userID, since, until, recovery.PendingRunIDs)
	if err != nil || result.Items["wood"] != 9 || !result.LegacyLogs {
		t.Fatal("legacy receipt overlap double counted", result, err)
	}
	result, err = s.GetOfflineCombatRewards(ctx, userID, until, until.Add(time.Hour), recovery.PendingRunIDs)
	if err != nil || result.Items["wood"] != 0 || result.PendingRuns != 0 {
		t.Fatal("late run reported in two absences", result, err)
	}
	err = s.withTx(ctx, func(tx pgx.Tx) error {
		_, err := s.recordBattleRewardReceiptTx(ctx, tx, 123, userID, map[string]int64{"wood": 8})
		return err
	})
	if err == nil {
		t.Fatal("conflicting reward plan accepted")
	}
	if err = s.AcknowledgeOfflineRecovery(ctx, userID, complete.ID, complete.Revision); err != nil {
		t.Fatal(err)
	}
	if _, err = s.GetOfflineRecovery(ctx, userID); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatal("committed battle receipt did not release ACK", err)
	}
}

func TestOfflineRecoveryPostgresRetryQueueAndSchedule(t *testing.T) {
	s, ctx, _ := recoveryPostgresStore(t)
	now := time.Now().Truncate(time.Second)
	first, err := s.EnsureOfflineRecovery(ctx, 970071, now.Add(-time.Hour), now, now)
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.EnsureOfflineRecovery(ctx, 970071, now.Add(time.Second), now.Add(time.Minute), now.Add(time.Minute))
	if err != nil || second.ID == first.ID {
		t.Fatal("new absence overwritten", err)
	}
	for range 10 {
		if err = s.RetryOfflineRecoveryLater(ctx, first, context.DeadlineExceeded); err != nil {
			t.Fatal(err)
		}
	}
	retrying, err := s.GetOfflineRecovery(ctx, 970071)
	if err != nil || retrying.Status != "running" || retrying.Attempts != 10 {
		t.Fatal("transient failure permanently stopped recovery", retrying, err)
	}
	jobs, err := s.ListDueOfflineRecoveries(ctx, 4)
	if err != nil || len(jobs) != 0 {
		t.Fatal("later job bypassed retrying predecessor", jobs, err)
	}
	if _, err = s.db.Exec(ctx, `UPDATE player_offline_recoveries SET status='stalled' WHERE id=$1`, first.ID); err != nil {
		t.Fatal(err)
	}
	if err = s.RetryOfflineRecovery(ctx, 970072, first.ID, first.Revision); err != nil {
		t.Fatal(err)
	}
	if err = s.RetryOfflineRecovery(ctx, 970071, first.ID, first.Revision); err != nil {
		t.Fatal(err)
	}
	first, err = s.GetOfflineRecovery(ctx, 970071)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.AdvanceOfflineRecovery(ctx, first, map[string]ActionSpec{}); err != nil {
		t.Fatal(err)
	}
	schedules, err := s.OfflineRecoverySchedules(ctx)
	if err != nil || schedules[first.ID] != 970071 {
		t.Fatal("completion lost schedule intent", schedules, err)
	}
	if err = s.MarkOfflineRecoveryScheduled(ctx, first.ID); err != nil {
		t.Fatal(err)
	}
	schedules, err = s.OfflineRecoverySchedules(ctx)
	if err != nil || len(schedules) != 0 {
		t.Fatal("schedule acknowledgement not durable", schedules, err)
	}
	jobs, err = s.ListDueOfflineRecoveries(ctx, 4)
	if err != nil || len(jobs) != 1 || jobs[0].ID != second.ID {
		t.Fatal("queued absence did not resume", jobs, err)
	}
}

func TestOfflineRecoveryPostgresManualRetryFencesOldWorkers(t *testing.T) {
	s, ctx, _ := recoveryPostgresStore(t)
	const userID int64 = 970071
	target := time.Now().Truncate(time.Second)
	start := target.Add(-14 * time.Second)
	one := int64(1)
	spec := ActionSpec{ID: "retry_test", DurationS: 2, ItemReward: map[string]ActionItemRewardSpec{"wood": {Count: &one}}}
	specs := map[string]ActionSpec{spec.ID: spec}
	s.config.ActionQueueFastForwardMaxSteps = 3
	if _, err := s.db.Exec(ctx, `INSERT INTO player_action_queue(user_id,action_id,total_count,status,current_run_started_at,current_run_ends_at,payload)
 VALUES($1,$2,20,'running',$3,$4,'{"actionId":"retry_test","executorId":"generic","runtime":{"durationMs":2000}}')`, userID, spec.ID, start, start.Add(2*time.Second)); err != nil {
		t.Fatal(err)
	}
	r, err := s.EnsureOfflineRecovery(ctx, userID, start, target, target)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.AdvanceOfflineRecovery(ctx, r, specs); err != nil {
		t.Fatal(err)
	}
	old, err := s.GetOfflineRecovery(ctx, userID)
	if err != nil || old.Summary.CompletedSteps != 3 {
		t.Fatal("missing initial checkpoint", old, err)
	}
	for range 10 {
		if err = s.RetryOfflineRecoveryLater(ctx, old, errors.New("executor dependency missing")); err != nil {
			t.Fatal(err)
		}
	}
	stalled, err := s.GetOfflineRecovery(ctx, userID)
	if err != nil || stalled.Status != "stalled" {
		t.Fatal("expected deterministic failure to stop", stalled, err)
	}
	for _, request := range [][2]int64{{970072, stalled.Revision}, {userID, stalled.Revision - 1}} {
		if err = s.RetryOfflineRecovery(ctx, request[0], stalled.ID, request[1]); err != nil {
			t.Fatal(err)
		}
	}
	unchanged, err := s.GetOfflineRecovery(ctx, userID)
	if err != nil || !reflect.DeepEqual(stalled, unchanged) {
		t.Fatal("wrong owner or stale retry changed the task", unchanged, err)
	}
	var wg sync.WaitGroup
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := s.RetryOfflineRecovery(ctx, userID, stalled.ID, stalled.Revision); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	retried, err := s.GetOfflineRecovery(ctx, userID)
	if err != nil || retried.Status != "running" || retried.Revision != stalled.Revision+1 || retried.Attempts != 0 || retried.ErrorCode != "" {
		t.Fatal("retry did not create exactly one attempt generation", retried, err)
	}
	if !reflect.DeepEqual(retried.Summary, stalled.Summary) || !retried.StartedAt.Equal(stalled.StartedAt) || !retried.TargetAt.Equal(stalled.TargetAt) || !retried.ResumedAt.Equal(stalled.ResumedAt) {
		t.Fatal("retry rewrote the original checkpoint/window")
	}
	for range 10 {
		if err = s.RetryOfflineRecoveryLater(ctx, old, errors.New("late old failure")); err != nil {
			t.Fatal(err)
		}
	}
	if err = s.AdvanceOfflineRecovery(ctx, old, specs); err != nil {
		t.Fatal(err)
	}
	unchanged, err = s.GetOfflineRecovery(ctx, userID)
	if err != nil || !reflect.DeepEqual(retried, unchanged) {
		t.Fatal("old worker crossed retry fence", unchanged, err)
	}
	for range 5 {
		if retried.Status == "complete" {
			break
		}
		if err = s.AdvanceOfflineRecovery(ctx, retried, specs); err != nil {
			t.Fatal(err)
		}
		retried, err = s.GetOfflineRecovery(ctx, userID)
		if err != nil {
			t.Fatal(err)
		}
	}
	var quantity int64
	if err = s.db.QueryRow(ctx, `SELECT COALESCE(sum(quantity),0)::bigint FROM player_item_instances WHERE user_id=$1 AND template_item_id='wood'`, userID).Scan(&quantity); err != nil {
		t.Fatal(err)
	}
	if retried.Status != "complete" || retried.Summary.CompletedSteps != 7 || retried.Summary.ItemRewards["wood"] != 7 || quantity != 7 {
		t.Fatal("retry lost or duplicated rewards", retried, quantity)
	}
}
