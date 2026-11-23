package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"mysecondwebgame/server/internal/operationeffect"
	"mysecondwebgame/server/internal/operationeffect/transmute"
	"testing"
	"time"
)

func TestOfflineBackgroundPostgresOvernightTail(t *testing.T) {
	for _, duration := range []int64{2} {
		t.Run(fmt.Sprintf("%ds", duration), func(t *testing.T) {
			s, ctx, _ := recoveryPostgresStore(t)
			s.config.OfflineProgressMaxDuration = 8 * time.Hour
			start := time.Now().Add(-8 * time.Hour).Truncate(time.Second)
			if err := s.SetPresenceStatusAt(ctx, 970071, "offline", start); err != nil {
				t.Fatal(err)
			}
			presence, err := s.GetPresence(ctx, 970071)
			if err != nil {
				t.Fatal(err)
			}
			one := int64(1)
			spec := ActionSpec{ID: "overnight_background", DurationS: duration, ItemReward: map[string]ActionItemRewardSpec{"wood": {Count: &one}}}
			specs := map[string]ActionSpec{spec.ID: spec}
			payload := fmt.Sprintf(`{"actionId":"overnight_background","executorId":"generic","runtime":{"durationMs":%d}}`, duration*1000)
			if _, err = s.db.Exec(ctx, `INSERT INTO player_action_queue(user_id,action_id,total_count,status,current_run_started_at,current_run_ends_at,payload) VALUES(970071,$1,40000,'running',$2,$3,$4)`, spec.ID, start, start.Add(time.Duration(duration)*time.Second), payload); err != nil {
				t.Fatal(err)
			}
			begun := time.Now()
			for elapsed := 30 * time.Second; elapsed < 8*time.Hour; elapsed += 30 * time.Second {
				if _, err = s.AdvanceBackgroundActions(ctx, 970071, presence, start, start.Add(elapsed), specs); err != nil {
					t.Fatal(elapsed, err)
				}
			}
			backgroundElapsed := time.Since(begun)
			resumed := start.Add(8 * time.Hour)
			r, err := s.EnsureOfflineRecovery(ctx, 970071, start, resumed, resumed)
			if err != nil {
				t.Fatal(err)
			}
			before := r.Summary.CompletedSteps
			tailStart := time.Now()
			if err = s.AdvanceOfflineRecovery(ctx, r, specs); err != nil {
				t.Fatal(err)
			}
			result, err := s.GetOfflineRecovery(ctx, 970071)
			if err != nil {
				t.Fatal(err)
			}
			tailElapsed := time.Since(tailStart)
			want := int64(28800) / duration
			if result.Status != "complete" || result.Summary.CompletedSteps != want || result.Summary.ItemRewards["wood"] != want || result.Summary.CompletedSteps-before != 30/duration {
				t.Fatal("overnight return did not finish only its tail", result, before)
			}
			if pending, err := s.HasOfflineRecovery(ctx, 970071); err != nil || pending {
				t.Fatal("return left gameplay blocked", pending, err)
			}
			var qty int64
			if err = s.db.QueryRow(ctx, `SELECT COALESCE(sum(quantity),0)::bigint FROM player_item_instances WHERE user_id=970071 AND template_item_id='wood'`).Scan(&qty); err != nil || qty != want {
				t.Fatal("overnight inventory mismatch", qty, err)
			}
			t.Logf("duration=%ds steps=%d background_chunks=959 total_background=%s return_steps=%d return_store_time=%s", duration, want, backgroundElapsed, result.Summary.CompletedSteps-before, tailElapsed)
		})
	}
}

func TestOfflineBackgroundPostgresAlchemyOneSecond(t *testing.T) {
	fixture, ctx, _ := recoveryPostgresStore(t)
	s := New(fixture.db, fixture.config, fixture.gameData)
	start := time.Now().Add(-3 * time.Minute).Truncate(time.Second)
	spec := ActionSpec{ID: "alchemy.transmute", MainType: "alchemy", DurationS: 8}
	specs := map[string]ActionSpec{spec.ID: spec}
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		if err := s.ensureAllPlayerAttributesTx(ctx, tx, 970071); err != nil {
			return err
		}
		if err := s.GrantByTemplateTx(ctx, tx, 970071, "wood", 500); err != nil {
			return err
		}
		def := operationEffectTestDefinition(operationeffect.LifetimeCharges)
		def.Domain, def.Kind, def.Params = operationeffect.DomainAlchemy, transmute.Speed, []byte(`{"durationSeconds":1}`)
		def.Lifetime.Charges = 200
		def.Selector = operationeffect.Selector{ActionIDs: []string{spec.ID}}
		raw, err := json.Marshal(def)
		if err != nil {
			return err
		}
		if _, err = s.activateOperationEffectGrantTx(ctx, tx, 970071, "proof-food", def, operationeffect.Target{Kind: operationeffect.TargetPlayer}, raw, start); err != nil {
			return err
		}
		e := &alchemyExecutor{store: s, actionID: spec.ID}
		payload, err := e.FreezeRunContextTx(ctx, tx, 970071, ActionActorTypePlayer, 0, spec, alchemyTransmuteArgs{ItemID: "wood"}, start)
		if err != nil {
			return err
		}
		var id int64
		if err = tx.QueryRow(ctx, `INSERT INTO player_action_queue(user_id,action_id,total_count,status,payload) VALUES(970071,$1,500,'pending',$2) RETURNING id`, spec.ID, string(payload)).Scan(&id); err != nil {
			return err
		}
		session, err := s.loadOperationEffectSessionTx(ctx, tx, 970071, start)
		if err != nil {
			return err
		}
		item := QueueRuntimeItem{QueueID: id, ActorType: ActionActorTypePlayer, ActionID: spec.ID, Spec: spec, TotalCount: 500, PayloadRaw: payload, EffectSession: session, PreserveScheduledTimeline: true}
		if err = s.startQueueRunTx(ctx, tx, 970071, &item, start, queueRunStartOptions{}); err != nil {
			return err
		}
		_, err = s.flushOperationEffectSessionTx(ctx, tx, 970071, session, start)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if err = s.SetPresenceStatusAt(ctx, 970071, "offline", start); err != nil {
		t.Fatal(err)
	}
	presence, err := s.GetPresence(ctx, 970071)
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= 3; i++ {
		if _, err = s.AdvanceBackgroundActions(ctx, 970071, presence, start, start.Add(time.Duration(i)*30*time.Second), specs); err != nil {
			t.Fatal(err)
		}
	}
	r, err := s.EnsureOfflineRecovery(ctx, 970071, start, start.Add(2*time.Minute), start.Add(2*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if r.Summary.CompletedSteps != 90 {
		t.Fatal("background alchemy changed 1s food duration", r.Summary.CompletedSteps)
	}
	began := time.Now()
	if err = s.AdvanceOfflineRecovery(ctx, r, specs); err != nil {
		t.Fatal(err)
	}
	final, err := s.GetOfflineRecovery(ctx, 970071)
	if err != nil || final.Status != "complete" || final.Summary.CompletedSteps != 120 {
		t.Fatal("alchemy tail incomplete", final, err)
	}
	t.Logf("one-second transmute: return_steps=30 store_time=%s", time.Since(began))
	if err = s.AdvanceOfflineRecovery(context.Background(), r, specs); err != nil {
		t.Fatal(err)
	}
	var receipts, stock int64
	if err = s.db.QueryRow(ctx, `SELECT (SELECT count(*) FROM player_operation_effect_charge_receipts WHERE user_id=970071),(SELECT sum(quantity)::bigint FROM player_item_instances WHERE user_id=970071 AND template_item_id='wood')`).Scan(&receipts, &stock); err != nil {
		t.Fatal(err)
	}
	if receipts != 121 || stock != 379 {
		t.Fatal("food or prepaid costs duplicated", receipts, stock)
	}
}

func TestOfflineBackgroundPostgresCapSealAndReplay(t *testing.T) {
	s, ctx, _ := recoveryPostgresStore(t)
	s.config.OfflineProgressMaxDuration = 2 * time.Minute
	start := time.Now().Add(-8 * time.Hour).Truncate(time.Second)
	one := int64(1)
	spec := ActionSpec{ID: "background_test", DurationS: 2, ItemReward: map[string]ActionItemRewardSpec{"wood": {Count: &one}}}
	specs := map[string]ActionSpec{spec.ID: spec}
	for _, user := range []int64{970071, 970072} {
		if err := s.SetPresenceStatusAt(ctx, user, "offline", start); err != nil {
			t.Fatal(err)
		}
		if _, err := s.db.Exec(ctx, `INSERT INTO player_action_queue(user_id,action_id,total_count,status,current_run_started_at,current_run_ends_at,payload)
 VALUES($1,'background_test',20000,'running',$2,$3,'{"actionId":"background_test","executorId":"generic","runtime":{"durationMs":2000}}')`, user, start, start.Add(2*time.Second)); err != nil {
			t.Fatal(err)
		}
	}
	presence, err := s.GetPresence(ctx, 970071)
	if err != nil {
		t.Fatal(err)
	}
	var stale OfflineRecovery
	for i := 1; i <= 4; i++ {
		now := start.Add(time.Duration(i) * 30 * time.Second)
		r, ready, err := s.PrepareBackgroundOfflineRecovery(ctx, 970071, presence, start, now)
		if err != nil || !ready {
			t.Fatal(r, ready, err)
		}
		if i == 1 {
			stale = r
		}
		if i == 2 {
			if _, err = s.db.Exec(ctx, `ALTER TABLE player_offline_recoveries ADD CONSTRAINT background_fail_commit CHECK(revision<=2)`); err != nil {
				t.Fatal(err)
			}
			if err = s.AdvanceOfflineRecovery(ctx, r, specs); err == nil {
				t.Fatal("checkpoint fault did not roll back")
			}
			var qty int64
			if err = s.db.QueryRow(ctx, `SELECT COALESCE(sum(quantity),0)::bigint FROM player_item_instances WHERE user_id=970071 AND template_item_id='wood'`).Scan(&qty); err != nil || qty != 15 {
				t.Fatal("failed background chunk leaked rewards", qty, err)
			}
			if _, err = s.db.Exec(ctx, `ALTER TABLE player_offline_recoveries DROP CONSTRAINT background_fail_commit`); err != nil {
				t.Fatal(err)
			}
		}
		if err = s.AdvanceOfflineRecovery(ctx, r, specs); err != nil {
			t.Fatal(err)
		}
		if err = s.AdvanceOfflineRecovery(ctx, r, specs); err != nil {
			t.Fatal(err)
		}
		var qty int64
		if err = s.db.QueryRow(ctx, `SELECT COALESCE(sum(quantity),0)::bigint FROM player_item_instances WHERE user_id=970071 AND template_item_id='wood'`).Scan(&qty); err != nil {
			t.Fatal(err)
		}
		if qty != int64(i*15) {
			t.Fatalf("interval %d credited %d", i, qty)
		}
		if _, err = s.GetOfflineRecovery(ctx, 970071); !errors.Is(err, pgx.ErrNoRows) {
			t.Fatal("unsealed receipt exposed", err)
		}
		if pending, err := s.HasOfflineRecovery(ctx, 970071); err != nil || !pending {
			t.Fatal("unsealed absence allowed mutations ahead of its tail", pending, err)
		}
	}
	if schedules, err := s.OfflineRecoverySchedules(ctx); err != nil || len(schedules) != 0 {
		t.Fatal("background completion scheduled online execution", schedules, err)
	}
	users, err := s.ListBackgroundActionUsers(ctx, 0, 256)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range users.UserIDs {
		if id == 970071 {
			t.Fatal("capped user rediscovered")
		}
	}
	firstPage, err := s.ListBackgroundActionUsers(ctx, 0, 1)
	if err != nil || len(firstPage.UserIDs) != 0 || firstPage.NextAfter != 970071 {
		t.Fatal("capped page lost scan cursor", firstPage, err)
	}
	secondPage, err := s.ListBackgroundActionUsers(ctx, firstPage.NextAfter, 1)
	if err != nil || len(secondPage.UserIDs) != 1 || secondPage.UserIDs[0] != 970072 {
		t.Fatal("capped page starved next player", secondPage, err)
	}
	next, err := s.AdvanceBackgroundActions(ctx, 970071, presence, start, start.Add(4*time.Hour), specs)
	if err != nil || !next.IsZero() {
		t.Fatal("cap did not park background work", next, err)
	}
	resumed := start.Add(8 * time.Hour)
	r, err := s.EnsureOfflineRecovery(ctx, 970071, start, start.Add(2*time.Minute), resumed)
	if err != nil || r.Background || r.ID != stale.ID {
		t.Fatal("reconnect replaced absence", r, err)
	}
	if err = s.AdvanceOfflineRecovery(ctx, stale, specs); err != nil {
		t.Fatal(err)
	}
	if _, ready, err := s.PrepareBackgroundOfflineRecovery(ctx, 970071, presence, start, resumed); err != nil || ready {
		t.Fatal("stale worker reopened sealed absence", ready, err)
	}
	if err = s.AdvanceOfflineRecovery(ctx, r, specs); err != nil {
		t.Fatal(err)
	}
	final, err := s.GetOfflineRecovery(ctx, 970071)
	if err != nil || final.Status != "complete" || final.Summary.CompletedSteps != 60 || final.Summary.ItemRewards["wood"] != 60 {
		t.Fatal("invalid final receipt", final, err)
	}
	reference, err := s.EnsureOfflineRecovery(ctx, 970072, start, start.Add(2*time.Minute), resumed)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.AdvanceOfflineRecovery(ctx, reference, specs); err != nil {
		t.Fatal(err)
	}
	var first, second time.Time
	if err = s.db.QueryRow(ctx, `SELECT current_run_ends_at FROM player_action_queue WHERE user_id=970071`).Scan(&first); err != nil {
		t.Fatal(err)
	}
	if err = s.db.QueryRow(ctx, `SELECT current_run_ends_at FROM player_action_queue WHERE user_id=970072`).Scan(&second); err != nil {
		t.Fatal(err)
	}
	if !first.Equal(second) || !first.Equal(resumed.Add(2*time.Second)) {
		t.Fatal("cap shifted historical windows more than once", first, second)
	}
	if err = s.AdvanceOfflineRecovery(ctx, r, specs); err != nil {
		t.Fatal(err)
	}
	again, err := s.EnsureOfflineRecovery(ctx, 970071, start, resumed, resumed.Add(time.Second))
	if err != nil || again.ID != r.ID || again.Revision != final.Revision {
		t.Fatal("refresh repeated settlement", again, err)
	}
	if err := CheckOfflineRecoverySchema(ctx, s.db); err != nil {
		t.Fatal(err)
	}
	if _, err = s.db.Exec(ctx, `ALTER TABLE player_offline_recoveries DROP COLUMN background`); err != nil {
		t.Fatal(err)
	}
	if err := CheckOfflineRecoverySchema(ctx, s.db); err == nil {
		t.Fatal("startup accepted missing schema")
	}
}

func TestOfflineBackgroundPostgresPresenceFenceAndRollback(t *testing.T) {
	s, ctx, _ := recoveryPostgresStore(t)
	start := time.Now().Add(-time.Minute).Truncate(time.Second)
	if err := s.SetPresenceStatusAt(ctx, 970071, "offline", start); err != nil {
		t.Fatal(err)
	}
	presence, err := s.GetPresence(ctx, 970071)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.SetPresenceStatusAt(ctx, 970071, "online", start.Add(30*time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, ready, err := s.PrepareBackgroundOfflineRecovery(ctx, 970071, presence, start, start.Add(time.Minute)); err != nil || ready {
		t.Fatal("stale presence created background work", ready, err)
	}
}

func TestOfflineBackgroundPostgresRejectsMissingBattleDependency(t *testing.T) {
	s, ctx, _ := recoveryPostgresStore(t)
	if err := CheckOfflineRecoverySchema(ctx, s.db); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(ctx, `ALTER TABLE player_skills DROP COLUMN equipment_config_jsonb`); err != nil {
		t.Fatal(err)
	}
	if err := CheckOfflineRecoverySchema(ctx, s.db); err == nil {
		t.Fatal("startup accepted a schema that strands pending battle rewards")
	}
}

func TestOfflineBackgroundPostgresHonorsRetryDeadline(t *testing.T) {
	s, ctx, _ := recoveryPostgresStore(t)
	const userID int64 = 970071
	now := time.Now().Truncate(time.Millisecond)
	start := now.Add(-time.Minute)
	if err := s.SetPresenceStatusAt(ctx, userID, "offline", start); err != nil {
		t.Fatal(err)
	}
	presence, err := s.GetPresence(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	r, ready, err := s.PrepareBackgroundOfflineRecovery(ctx, userID, presence, start, now)
	if err != nil || !ready {
		t.Fatal("prepare", ready, err)
	}
	retryAt := now.Add(time.Minute)
	if _, err = s.db.Exec(ctx, `UPDATE player_offline_recoveries SET available_at=$2,attempts=3 WHERE id=$1`, r.ID, retryAt); err != nil {
		t.Fatal(err)
	}
	next, err := s.AdvanceBackgroundActions(ctx, userID, presence, start, now, map[string]ActionSpec{})
	if err != nil || !next.Equal(retryAt) {
		t.Fatal("worker ignored durable retry deadline", next, err)
	}
	var revision int64
	if err = s.db.QueryRow(ctx, `SELECT revision FROM player_offline_recoveries WHERE id=$1`, r.ID).Scan(&revision); err != nil || revision != r.Revision {
		t.Fatal("early retry executed a chunk", revision, err)
	}
	if _, err = s.AdvanceBackgroundActions(ctx, userID, presence, start, retryAt, map[string]ActionSpec{}); err != nil {
		t.Fatal(err)
	}
	var status string
	if err = s.db.QueryRow(ctx, `SELECT status FROM player_offline_recoveries WHERE id=$1`, r.ID).Scan(&status); err != nil || status != "complete" {
		t.Fatal("due retry did not finish", status, err)
	}
	if err = s.AcknowledgeOfflineRecovery(ctx, userID, r.ID, r.Revision+1); err != nil {
		t.Fatal(err)
	}
	var acknowledged bool
	if err = s.db.QueryRow(ctx, `SELECT acknowledged_at IS NOT NULL FROM player_offline_recoveries WHERE id=$1`, r.ID).Scan(&acknowledged); err != nil || acknowledged {
		t.Fatal("open background absence was acknowledged", acknowledged, err)
	}
}
