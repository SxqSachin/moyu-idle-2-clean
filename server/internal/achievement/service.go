package achievement

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"mysecondwebgame/server/internal/store"

	"github.com/redis/go-redis/v9"
)

const (
	redisBufferKey     = "achievement:events:buf"
	redisFlushLockKey  = "achievement:events:flush_lock"
	redisFlushInterval = 2 * time.Second
	redisFlushBatch    = 500
	redisFlushBudget   = 500 * time.Millisecond
	redisFlushLockTTL  = 30 * time.Second
)

var releaseRedisFlushLockScript = redis.NewScript(`
if redis.call("GET", KEYS[1]) == ARGV[1] then
	return redis.call("DEL", KEYS[1])
end
return 0
`)

type Store interface {
	ClaimQueuedAchievementEvents(ctx context.Context, workerID string, limit int, lease time.Duration) ([]store.QueuedAchievementEvent, error)
	ProcessQueuedAchievementEvent(ctx context.Context, event store.QueuedAchievementEvent, workerID string) error
	ProcessQueuedAchievementEventsBatch(ctx context.Context, events []store.QueuedAchievementEvent, workerID string) ([]int64, error)
	CleanupAchievementEvents(ctx context.Context) error
}

type BulkEnqueuer interface {
	BulkEnqueueAchievementEvents(ctx context.Context, batches []store.AchievementEventBatch) error
}

type Options struct {
	Enabled         bool
	WorkerID        string
	Concurrency     int
	BatchSize       int
	PollInterval    time.Duration
	LeaseDuration   time.Duration
	CleanupInterval time.Duration
	Redis           *redis.Client
}

type bufferedAchievementBatch struct {
	UserID int64                    `json:"u"`
	Events []store.AchievementEvent `json:"e"`
}

type Service struct {
	store            Store
	bulk             BulkEnqueuer
	opts             Options
	wake             chan struct{}
	onProcessedUsers func([]int64)
}

func NewService(st Store, opts Options) *Service {
	if opts.WorkerID == "" {
		opts.WorkerID = "achievement-worker"
	}
	if opts.Concurrency <= 0 {
		opts.Concurrency = 1
	}
	if opts.BatchSize <= 0 {
		opts.BatchSize = 200
	}
	if opts.PollInterval <= 0 {
		opts.PollInterval = time.Second
	}
	if opts.LeaseDuration <= 0 {
		opts.LeaseDuration = 30 * time.Second
	}
	if opts.CleanupInterval <= 0 {
		opts.CleanupInterval = time.Hour
	}
	return &Service{
		store: st,
		opts:  opts,
		wake:  make(chan struct{}, 1),
	}
}

func (s *Service) SetBulkEnqueuer(e BulkEnqueuer) {
	s.bulk = e
}

func (s *Service) SetProcessedUsersCallback(fn func([]int64)) {
	s.onProcessedUsers = fn
}

// RedisEnqueue pushes achievement events to the Redis write-behind buffer.
// Called by Store.EnqueueAchievementEvents when the enqueuer hook is set.
func (s *Service) RedisEnqueue(ctx context.Context, userID int64, events []store.AchievementEvent) error {
	if s.opts.Redis == nil {
		return fmt.Errorf("achievement redis enqueue: redis unavailable")
	}
	data, err := json.Marshal(bufferedAchievementBatch{UserID: userID, Events: events})
	if err != nil {
		return fmt.Errorf("achievement redis enqueue: %w", err)
	}
	return s.opts.Redis.RPush(ctx, redisBufferKey, data).Err()
}

func (s *Service) Wake() {
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

func (s *Service) Start(ctx context.Context) {
	if s == nil || !s.opts.Enabled || s.store == nil {
		return
	}
	var wg sync.WaitGroup
	for i := 0; i < s.opts.Concurrency; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			s.runWorker(ctx, index)
		}(i)
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		s.runCleanup(ctx)
	}()
	if s.opts.Redis != nil && s.bulk != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.runFlush(ctx)
		}()
	}
	go func() {
		<-ctx.Done()
		wg.Wait()
	}()
}

func (s *Service) runWorker(ctx context.Context, index int) {
	workerID := fmt.Sprintf("%s-%d-%d-%d", s.opts.WorkerID, os.Getpid(), time.Now().UnixNano(), index)
	ticker := time.NewTicker(s.opts.PollInterval)
	defer ticker.Stop()
	for {
		s.drain(ctx, workerID)
		select {
		case <-ctx.Done():
			return
		case <-s.wake:
		case <-ticker.C:
		}
	}
}

func (s *Service) drain(ctx context.Context, workerID string) {
	for {
		events, err := s.store.ClaimQueuedAchievementEvents(ctx, workerID, s.opts.BatchSize, s.opts.LeaseDuration)
		if err != nil {
			log.Printf("achievement worker claim failed: %v", err)
			return
		}
		if len(events) == 0 {
			return
		}
		processedUsers, err := s.store.ProcessQueuedAchievementEventsBatch(ctx, events, workerID)
		if err != nil {
			log.Printf("achievement worker batch process failed: %v", err)
		} else {
			s.notifyProcessedUsers(processedUsers)
		}
		if len(events) < s.opts.BatchSize {
			return
		}
	}
}

func (s *Service) notifyProcessedUsers(userIDs []int64) {
	if s.onProcessedUsers == nil || len(userIDs) == 0 {
		return
	}
	s.onProcessedUsers(userIDs)
}

func (s *Service) runCleanup(ctx context.Context) {
	ticker := time.NewTicker(s.opts.CleanupInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.store.CleanupAchievementEvents(ctx); err != nil {
				log.Printf("achievement event cleanup failed: %v", err)
			}
		}
	}
}

func (s *Service) runFlush(ctx context.Context) {
	ticker := time.NewTicker(redisFlushInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.flushRedisBuffer(ctx)
		}
	}
}

func (s *Service) flushRedisBuffer(ctx context.Context) {
	if s.opts.Redis == nil || s.bulk == nil {
		return
	}
	token := fmt.Sprintf("%d-%d", os.Getpid(), time.Now().UnixNano())
	locked, err := s.opts.Redis.SetNX(ctx, redisFlushLockKey, token, redisFlushLockTTL).Result()
	if err != nil || !locked {
		return
	}
	defer func() {
		_, _ = releaseRedisFlushLockScript.Run(ctx, s.opts.Redis, []string{redisFlushLockKey}, token).Result()
	}()

	deadline := time.Now().Add(redisFlushBudget)
	for {
		keepGoing := s.flushRedisBufferBatch(ctx)
		if !keepGoing || time.Now().After(deadline) {
			return
		}
	}
}

func (s *Service) flushRedisBufferBatch(ctx context.Context) bool {
	items, err := s.opts.Redis.LRange(ctx, redisBufferKey, 0, redisFlushBatch-1).Result()
	if err != nil || len(items) == 0 {
		return false
	}
	batches := make([]store.AchievementEventBatch, 0, len(items))
	for _, item := range items {
		var b bufferedAchievementBatch
		if jsonErr := json.Unmarshal([]byte(item), &b); jsonErr != nil {
			log.Printf("achievement flush decode: %v", jsonErr)
			continue
		}
		if b.UserID > 0 && len(b.Events) > 0 {
			batches = append(batches, store.AchievementEventBatch{UserID: b.UserID, Events: b.Events})
		}
	}
	if len(batches) > 0 {
		if err := s.bulk.BulkEnqueueAchievementEvents(ctx, batches); err != nil {
			log.Printf("achievement flush bulk enqueue: %v", err)
			return false
		}
	}
	if err := s.opts.Redis.LTrim(ctx, redisBufferKey, int64(len(items)), -1).Err(); err != nil {
		log.Printf("achievement flush trim: %v", err)
		return false
	}
	return len(items) >= redisFlushBatch
}
