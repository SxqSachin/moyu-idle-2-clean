package achievement

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"

	"mysecondwebgame/server/internal/store"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

type fakeAchievementStore struct {
	claimed        bool
	processed      []store.QueuedAchievementEvent
	processedUsers []int64
	processErr     error
}

func (f *fakeAchievementStore) ClaimQueuedAchievementEvents(context.Context, string, int, time.Duration) ([]store.QueuedAchievementEvent, error) {
	if f.claimed {
		return nil, nil
	}
	f.claimed = true
	return []store.QueuedAchievementEvent{
		{EventID: "1", UserID: 42},
		{EventID: "2", UserID: 42},
		{EventID: "3", UserID: 77},
	}, nil
}

func (f *fakeAchievementStore) ProcessQueuedAchievementEvent(context.Context, store.QueuedAchievementEvent, string) error {
	return nil
}

func (f *fakeAchievementStore) ProcessQueuedAchievementEventsBatch(_ context.Context, events []store.QueuedAchievementEvent, _ string) ([]int64, error) {
	f.processed = append(f.processed, events...)
	return append([]int64(nil), f.processedUsers...), f.processErr
}

func (f *fakeAchievementStore) CleanupAchievementEvents(context.Context) error {
	return nil
}

func TestDrainNotifiesProcessedUsers(t *testing.T) {
	fakeStore := &fakeAchievementStore{processedUsers: []int64{42}}
	service := NewService(fakeStore, Options{BatchSize: 10})
	var notified []int64
	service.SetProcessedUsersCallback(func(userIDs []int64) {
		notified = append(notified, userIDs...)
	})

	service.drain(context.Background(), "worker-1")

	if len(fakeStore.processed) != 3 {
		t.Fatalf("processed events=%d, want 3", len(fakeStore.processed))
	}
	if want := []int64{42}; !reflect.DeepEqual(notified, want) {
		t.Fatalf("notified users=%v, want %v", notified, want)
	}
}

func TestDrainDoesNotNotifyWhenBatchProcessingFails(t *testing.T) {
	fakeStore := &fakeAchievementStore{
		processedUsers: []int64{42},
		processErr:     errors.New("process failed"),
	}
	service := NewService(fakeStore, Options{BatchSize: 10})
	notified := false
	service.SetProcessedUsersCallback(func([]int64) {
		notified = true
	})

	service.drain(context.Background(), "worker-1")

	if notified {
		t.Fatal("failed batch must not notify achievement users")
	}
}

type fakeBulkEnqueuer struct {
	err     error
	batches []store.AchievementEventBatch
}

func (f *fakeBulkEnqueuer) BulkEnqueueAchievementEvents(_ context.Context, batches []store.AchievementEventBatch) error {
	if f.err != nil {
		return f.err
	}
	f.batches = append(f.batches, batches...)
	return nil
}

func TestFlushRedisBufferKeepsItemsWhenBulkFails(t *testing.T) {
	redisServer := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	defer client.Close()
	pushBufferedAchievementBatch(t, client, 42)
	bulk := &fakeBulkEnqueuer{err: errors.New("db unavailable")}
	service := NewService(&fakeAchievementStore{}, Options{Redis: client})
	service.SetBulkEnqueuer(bulk)

	service.flushRedisBuffer(context.Background())

	if got := client.LLen(context.Background(), redisBufferKey).Val(); got != 1 {
		t.Fatalf("redis buffer len=%d, want 1", got)
	}
}

func TestFlushRedisBufferTrimsItemsAfterBulkSuccess(t *testing.T) {
	redisServer := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	defer client.Close()
	pushBufferedAchievementBatch(t, client, 42)
	bulk := &fakeBulkEnqueuer{}
	service := NewService(&fakeAchievementStore{}, Options{Redis: client})
	service.SetBulkEnqueuer(bulk)

	service.flushRedisBuffer(context.Background())

	if got := client.LLen(context.Background(), redisBufferKey).Val(); got != 0 {
		t.Fatalf("redis buffer len=%d, want 0", got)
	}
	if len(bulk.batches) != 1 || bulk.batches[0].UserID != 42 {
		t.Fatalf("flushed batches=%#v, want one user 42 batch", bulk.batches)
	}
}

func TestFlushRedisBufferDrainsMultipleBatchesWithinBudget(t *testing.T) {
	redisServer := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	defer client.Close()
	for i := 0; i < redisFlushBatch+1; i++ {
		pushBufferedAchievementBatch(t, client, int64(1000+i))
	}
	bulk := &fakeBulkEnqueuer{}
	service := NewService(&fakeAchievementStore{}, Options{Redis: client})
	service.SetBulkEnqueuer(bulk)

	service.flushRedisBuffer(context.Background())

	if got := client.LLen(context.Background(), redisBufferKey).Val(); got != 0 {
		t.Fatalf("redis buffer len=%d, want 0", got)
	}
	if len(bulk.batches) != redisFlushBatch+1 {
		t.Fatalf("flushed batches=%d, want %d", len(bulk.batches), redisFlushBatch+1)
	}
}

func pushBufferedAchievementBatch(t *testing.T, client *redis.Client, userID int64) {
	t.Helper()
	raw, err := json.Marshal(bufferedAchievementBatch{
		UserID: userID,
		Events: []store.AchievementEvent{{
			Kind:      store.AchievementEventCompleteAction,
			ActionID:  "collect_clear_resin",
			ActorType: store.ActionActorTypePlayer,
			Count:     1,
			SourceKey: "test-source",
		}},
	})
	if err != nil {
		t.Fatalf("marshal buffered batch: %v", err)
	}
	if err := client.RPush(context.Background(), redisBufferKey, string(raw)).Err(); err != nil {
		t.Fatalf("push redis batch: %v", err)
	}
}
