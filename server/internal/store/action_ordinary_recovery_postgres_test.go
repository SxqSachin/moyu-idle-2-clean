package store

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestOrdinaryPostgresRecovery50000EquipmentAndProjection(t *testing.T) {
	fixture, _, counter := recoveryPostgresStore(t)
	s := New(fixture.db, fixture.config, fixture.gameData)
	s.config.OrdinarySettlementEnabled = true
	s.config.AchievementsEnabled = true
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	start := time.Now().Add(-30 * time.Hour).Truncate(time.Second)
	target := start.Add(100000 * time.Second)
	// A legacy prepaid head proves the migration occurs AFTER that completion.
	spec := ActionSpec{ID: "logging", MainType: "collecting", DurationS: 2, ItemReward: BuildActionSpecMap(s.gameData)["craft_base2_mistwarden_charm"].ItemReward, MainOutput: "base2_mistwarden_charm"}
	specs := map[string]ActionSpec{spec.ID: spec}
	var queueID int64
	if err := s.db.QueryRow(ctx, `INSERT INTO player_action_queue(user_id,action_id,total_count,status,current_run_started_at,current_run_ends_at,payload)
	 VALUES(970071,'logging',50000,'running',$1,$2,'{"actionId":"logging","executorId":"generic","runtime":{"baseDurationMs":2000,"durationMs":2000}}') RETURNING id`, start, start.Add(2*time.Second)).Scan(&queueID); err != nil {
		t.Fatal(err)
	}
	job, err := s.EnsureOfflineRecovery(ctx, 970071, start, target, target)
	if err != nil {
		t.Fatal(err)
	}
	before, began := counter.count.Load(), time.Now()
	if err := s.AdvanceOfflineRecovery(ctx, job, specs); err != nil {
		t.Fatal(err)
	}
	first, err := s.GetOfflineRecovery(ctx, 970071)
	if err != nil {
		t.Fatal(err)
	}
	var raw []byte
	var done int
	if err := s.db.QueryRow(ctx, `SELECT payload,done_count FROM player_action_queue WHERE id=$1`, queueID).Scan(&raw, &done); err != nil {
		t.Fatal(err)
	}
	state, err := ordinaryState(raw)
	if err != nil || state == nil || state.FirstOrdinal != 1 || done != 1000 {
		t.Fatalf("migration boundary: state=%+v done=%d err=%v", state, done, err)
	}
	seed := state.Seed
	if _, err := s.db.Exec(ctx, `CREATE FUNCTION fail_ordinary_checkpoint() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'ordinary checkpoint injected failure'; END $$;
	 CREATE TRIGGER fail_ordinary_checkpoint BEFORE UPDATE ON player_action_queue FOR EACH ROW EXECUTE FUNCTION fail_ordinary_checkpoint()`); err != nil {
		t.Fatal(err)
	}
	if err := s.AdvanceOfflineRecovery(ctx, first, specs); err == nil {
		t.Fatal("injected failure did not roll back")
	}
	var count int
	if err := s.db.QueryRow(ctx, `SELECT count(*) FROM player_item_instances WHERE user_id=970071`).Scan(&count); err != nil || count != 1000 {
		t.Fatal("uncommitted equipment escaped", count, err)
	}
	if err := s.db.QueryRow(ctx, `SELECT payload FROM player_action_queue WHERE id=$1`, queueID).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	rolledBack, err := ordinaryState(raw)
	if err != nil || rolledBack.Seed != seed {
		t.Fatal("retry changed random stream", err)
	}
	if _, err := s.db.Exec(ctx, `DROP TRIGGER fail_ordinary_checkpoint ON player_action_queue`); err != nil {
		t.Fatal(err)
	}
	// Closing admission must NOT switch existing versioned work back to legacy.
	s.config.OrdinarySettlementEnabled = false
	for i := 0; i < 100; i++ {
		current, err := s.GetOfflineRecovery(ctx, 970071)
		if err != nil {
			t.Fatal(err)
		}
		if current.Status == "complete" {
			break
		}
		if err := s.AdvanceOfflineRecovery(ctx, current, specs); err != nil {
			t.Fatal(err)
		}
	}
	complete, err := s.GetOfflineRecovery(ctx, 970071)
	if err != nil || complete.Status != "complete" || complete.Summary.CompletedSteps != 50000 {
		t.Fatalf("incomplete equipment recovery: %+v err=%v", complete, err)
	}
	settled := time.Since(began)
	if err := s.AdvanceOfflineRecovery(ctx, first, specs); err != nil {
		t.Fatal(err)
	}
	if err := s.AdvanceOfflineRecovery(ctx, complete, specs); err != nil {
		t.Fatal(err)
	}
	projectedAt := time.Now()
	items, err := s.ListItemInstances(ctx, 970071)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 50000 {
		t.Fatalf("duplicate/missing equipment: %d", len(items))
	}
	seen := map[int64]bool{}
	for _, item := range items {
		if item.Quantity != 1 || seen[item.InstanceID] {
			t.Fatal("invalid independent equipment identity")
		}
		seen[item.InstanceID] = true
	}
	projection, err := json.Marshal(items)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.AcknowledgeOfflineRecovery(ctx, 970071, complete.ID, complete.Revision); err != nil {
		t.Fatal(err)
	}
	t.Logf("50000 independent equipment: settlement=%s SQL=%d; authoritative inventory read+JSON=%s bytes=%d", settled, counter.count.Load()-before, time.Since(projectedAt), len(projection))
}
