package store

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestOfflineAchievementBatchPostgresParity(t *testing.T) {
	s, ctx, counter := recoveryPostgresStore(t)
	s.config.AchievementsEnabled = true
	var want []byte
	var referenceQueries int64
	for _, batched := range []bool{false, true} {
		tx, err := s.db.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(context.Background())
		batch := &offlineAchievementBatch{store: s, tx: tx, userID: 970071}
		runCtx := ctx
		if batched {
			runCtx = context.WithValue(ctx, offlineAchievementBatchKey{}, batch)
		}
		notifications := 0
		s.achievementNotify = func(context.Context) { notifications++ }
		before, began := counter.count.Load(), time.Now()
		for i := 0; i < 2048; i++ {
			events := []AchievementEvent{{Kind: AchievementEventObtainItem, ItemID: "wood", SubjectID: "wood", Count: 3, SourceKey: fmt.Sprintf("proof:%d", i)}, {Kind: AchievementEventStateReconcile, StateKind: "own_item", ItemID: "wood", SubjectID: "wood", SourceKey: fmt.Sprintf("state:%d", i)}}
			if err = s.EnqueueAchievementEventsTx(runCtx, tx, 970071, events); err != nil {
				t.Fatal(err)
			}
			if i%11 == 0 {
				if err = s.EnqueueAchievementEventsTx(runCtx, tx, 970071, events); err != nil {
					t.Fatal(err)
				}
			}
			if len(batch.rows) >= achievementEventInsertBatchSize {
				t.Fatal("event buffer exceeded its bound")
			}
		}
		if batched {
			if err = batch.flush(runCtx); err != nil {
				t.Fatal(err)
			}
			if notifications != 0 || !batch.inserted {
				t.Fatal("notification escaped uncommitted recovery", notifications, batch.inserted)
			}
		}
		queries := counter.count.Load() - before
		t.Logf("batched=%v events=4096 plus duplicate replays elapsed=%s SQL=%d", batched, time.Since(began), queries)
		var got []byte
		if err = tx.QueryRow(ctx, `SELECT jsonb_agg(to_jsonb(e)-'available_at'-'created_at'-'updated_at' ORDER BY event_id) FROM achievement_events e`).Scan(&got); err != nil {
			t.Fatal(err)
		}
		if !batched {
			want, referenceQueries = got, queries
		} else if string(want) != string(got) || queries >= referenceQueries/100 {
			t.Fatal("event identities, payloads, deduplication or batching changed")
		}
		if err = tx.Rollback(ctx); err != nil {
			t.Fatal(err)
		}
		var count int
		if err = s.db.QueryRow(ctx, `SELECT count(*) FROM achievement_events`).Scan(&count); err != nil || count != 0 {
			t.Fatal("events escaped rollback", count, err)
		}
	}
}

func TestOfflineAchievementBatchPostgresOwnership(t *testing.T) {
	s, ctx, _ := recoveryPostgresStore(t)
	s.config.AchievementsEnabled = true
	tx, err := s.db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	batch := &offlineAchievementBatch{store: s, tx: tx, userID: 970071}
	ctx = context.WithValue(ctx, offlineAchievementBatchKey{}, batch)
	event := []AchievementEvent{{Kind: AchievementEventObtainItem, ItemID: "wood", Count: 1, SourceKey: "proof"}}
	if err = s.EnqueueAchievementEventsTx(ctx, tx, 970071, event); err != nil {
		t.Fatal(err)
	}
	if err = s.EnqueueAchievementEventsTx(ctx, tx, 970072, event); err != nil {
		t.Fatal(err)
	}
	var count int
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM achievement_events`).Scan(&count); err != nil || count != 1 || len(batch.rows) != 1 {
		t.Fatal("user boundary was not respected", count, len(batch.rows), err)
	}
	other, err := s.db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Rollback(context.Background())
	if err = s.EnqueueAchievementEventsTx(ctx, other, 970071, event); err != nil {
		t.Fatal(err)
	}
	if err = other.QueryRow(ctx, `SELECT count(*) FROM achievement_events`).Scan(&count); err != nil || count != 1 || len(batch.rows) != 1 {
		t.Fatal("transaction boundary was not respected", count, len(batch.rows), err)
	}
}
