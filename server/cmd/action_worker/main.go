package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"mysecondwebgame/server/internal/actionruntime"
	"mysecondwebgame/server/internal/catalog"
	"mysecondwebgame/server/internal/config"
	"mysecondwebgame/server/internal/gamedata"
	"mysecondwebgame/server/internal/mq"
	"mysecondwebgame/server/internal/reqmetrics"
	"mysecondwebgame/server/internal/store"
)

const (
	actionWorkerAuditLabel        = "action_worker"
	actionWorkerDeadRecoveryLimit = 1000
)

var releaseClaimScript = redis.NewScript(`
if redis.call("GET", KEYS[1]) == ARGV[1] then
	return redis.call("DEL", KEYS[1])
end
return 0
`)

type workerConfig struct {
	batchSize        int64
	concurrency      int
	userTimeout      time.Duration
	pollInterval     time.Duration
	claimTTL         time.Duration
	stuckExitAfter   time.Duration
	healthLogPeriod  time.Duration
	retryBaseDelay   time.Duration
	retryMaxDelay    time.Duration
	failureTTL       time.Duration
	deadAfterFailure int64
	deadRetryDelay   time.Duration
	deadRetryMax     time.Duration
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	dbConfig, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("parse postgres config: %v", err)
	}
	dbConfig.ConnConfig.Tracer = reqmetrics.NewPGXQueryTracer()
	db, err := pgxpool.NewWithConfig(ctx, dbConfig)
	if err != nil {
		log.Fatalf("connect postgres: %v", err)
	}
	defer db.Close()
	if err := store.CheckOfflineRecoverySchema(ctx, db); err != nil {
		log.Fatal(err)
	}

	rd := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})
	rd.AddHook(reqmetrics.NewRedisHook())
	if err := rd.Ping(ctx).Err(); err != nil {
		log.Fatalf("connect redis: %v", err)
	}
	defer rd.Close()

	publisher, err := mq.NewNatsPublisher(cfg.NATSURL, cfg.MQTopicPrefix)
	if err != nil {
		log.Fatalf("connect nats: %v", err)
	}
	defer publisher.Close()

	gameData, err := catalog.Load(cfg.GameDataPath)
	if err != nil {
		log.Fatalf("load game data: %v", err)
	}

	st := store.New(db, storeRuntimeConfig(cfg, gameData), gameData)
	if err := st.BindFurnitureInvalidations(publisher); err != nil {
		log.Fatalf("subscribe furniture cache: %v", err)
	}
	runtime := actionruntime.NewWorker(st, gameData)
	worker := &actionWorker{
		redis:                  rd,
		mq:                     publisher,
		store:                  st,
		runtime:                runtime,
		settings:               loadWorkerConfig(),
		visibilityOfflineDelay: time.Duration(cfg.SharedGlobalSettings.Presence.VisibilityOfflineDelaySeconds) * time.Second,
	}

	log.Printf(
		"action worker started nats=%s batch=%d concurrency=%d user_timeout=%s",
		cfg.NATSURL,
		worker.settings.batchSize,
		worker.settings.concurrency,
		worker.settings.userTimeout,
	)
	if err := worker.run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		log.Fatalf("action worker stopped with error: %v", err)
	}
	log.Printf("action worker stopped")
}

type actionWorker struct {
	redis                  *redis.Client
	mq                     mq.Publisher
	store                  *store.Store
	runtime                *actionruntime.Runtime
	settings               workerConfig
	lastBatchUsers         int
	lastBatchAdvanced      int
	lastBatchDuration      time.Duration
	backgroundScanAfter    int64
	backgroundScanAt       time.Time
	visibilityOfflineDelay time.Duration
}

func (w *actionWorker) run(ctx context.Context) error {
	pollTimer := time.NewTimer(0)
	defer pollTimer.Stop()
	healthTicker := time.NewTicker(w.settings.healthLogPeriod)
	defer healthTicker.Stop()

	lastHealthyPoll := time.Now()
	lastProgress := time.Now()
	consecutiveFailures := 0
	needsDeadRecovery := false
	if recovered, err := w.recoverDeadUsers(ctx); err != nil {
		needsDeadRecovery = true
		log.Printf("action worker initial dead-user recovery failed err=%v", err)
	} else if recovered > 0 {
		log.Printf("action worker initial dead-user recovery recovered=%d", recovered)
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-healthTicker.C:
			log.Printf(
				"action worker health last_healthy_poll_age_ms=%d last_progress_age_ms=%d consecutive_failures=%d batch_users=%d batch_advanced=%d batch_duration_ms=%d batch_full=%t",
				time.Since(lastHealthyPoll).Milliseconds(),
				time.Since(lastProgress).Milliseconds(),
				consecutiveFailures,
				w.lastBatchUsers,
				w.lastBatchAdvanced,
				w.lastBatchDuration.Milliseconds(),
				w.settings.batchSize > 0 && int64(w.lastBatchUsers) >= w.settings.batchSize,
			)
		case <-pollTimer.C:
			regularPollAt := time.Now().Add(w.pollInterval())
			advanced, err := w.processBatch(ctx)
			w.lastBatchAdvanced = advanced
			if err != nil {
				consecutiveFailures++
				log.Printf("action worker batch failed err=%v consecutive_failures=%d", err, consecutiveFailures)
			} else {
				if needsDeadRecovery {
					recovered, recoveryErr := w.recoverDeadUsers(ctx)
					if recoveryErr != nil {
						log.Printf("action worker dead-user recovery failed err=%v", recoveryErr)
					} else {
						needsDeadRecovery = false
						if recovered > 0 {
							log.Printf("action worker dead-user recovery recovered=%d", recovered)
						}
					}
				}
				lastHealthyPoll = time.Now()
				consecutiveFailures = 0
			}
			if advanced > 0 {
				lastProgress = time.Now()
			}
			if w.settings.stuckExitAfter > 0 && consecutiveFailures > 0 && time.Since(lastHealthyPoll) > w.settings.stuckExitAfter {
				return errors.New("action worker stuck without healthy polls")
			}
			// Borrow idle time for a known deadline, but keep the old batch cadence
			// after any due work so backlog cannot raise database throughput.
			allowDeadlineWake := err == nil && w.lastBatchUsers == 0
			pollTimer.Reset(w.nextPollDelay(ctx, regularPollAt, allowDeadlineWake))
		}
	}
}

func (w *actionWorker) pollInterval() time.Duration {
	if w.settings.pollInterval > 0 {
		return w.settings.pollInterval
	}
	return 200 * time.Millisecond
}

func (w *actionWorker) nextPollDelay(ctx context.Context, regularPollAt time.Time, allowDeadlineWake bool) time.Duration {
	now := time.Now()
	regularDelay := max(regularPollAt.Sub(now), 0)
	if regularDelay == 0 {
		return 0
	}
	if !allowDeadlineWake {
		return regularDelay
	}
	if w.redis == nil {
		return regularDelay
	}
	rows, err := w.redis.ZRangeWithScores(ctx, actionruntime.QueueDueKey(), 0, 0).Result()
	now = time.Now()
	regularDelay = max(regularPollAt.Sub(now), 0)
	if err != nil || len(rows) == 0 {
		return regularDelay
	}
	// Redis stores millisecond scores while PostgreSQL timestamps can retain
	// sub-millisecond precision. Wake one millisecond after the score so a known
	// deadline cannot be observed just before the authoritative timestamp.
	nextDueAt := actionWorkerDueScoreWakeAt(rows[0].Score)
	return actionWorkerPollDelay(now, nextDueAt, regularDelay, true)
}

func actionWorkerDueScoreWakeAt(score float64) time.Time {
	return time.UnixMilli(int64(score)).Add(time.Millisecond)
}

func actionWorkerPollDelay(now, nextDueAt time.Time, regularDelay time.Duration, allowDeadlineWake bool) time.Duration {
	regularDelay = max(regularDelay, 0)
	if nextDueAt.IsZero() || !allowDeadlineWake {
		return regularDelay
	}
	untilDue := nextDueAt.Sub(now)
	if untilDue <= 0 {
		return 0
	}
	if untilDue < regularDelay {
		return untilDue
	}
	return regularDelay
}

func (w *actionWorker) processBatch(ctx context.Context) (int, error) {
	startedAt := time.Now()
	w.lastBatchUsers = 0
	defer func() {
		w.lastBatchDuration = time.Since(startedAt)
	}()
	nowMs := time.Now().UnixMilli()
	if time.Now().After(w.backgroundScanAt) {
		scanCtx, cancel := context.WithTimeout(ctx, time.Second)
		err := w.recoverBackgroundSchedules(scanCtx)
		cancel()
		w.backgroundScanAt = time.Now().Add(5 * time.Second)
		if err != nil {
			log.Printf("action worker background schedule recovery failed err=%v", err)
		}
	}
	users, err := w.redis.ZRangeByScore(ctx, actionruntime.QueueDueKey(), &redis.ZRangeBy{
		Min:    "-inf",
		Max:    strconv.FormatInt(nowMs, 10),
		Offset: 0,
		Count:  w.settings.batchSize,
	}).Result()
	if err != nil {
		return 0, err
	}
	w.lastBatchUsers = len(users)
	if w.settings.concurrency <= 1 {
		advanced := 0
		failures := 0
		validUsers := 0
		var firstErr error
		for _, member := range users {
			userID, parseErr := strconv.ParseInt(strings.TrimSpace(member), 10, 64)
			if parseErr != nil || userID <= 0 {
				_ = w.redis.ZRem(ctx, actionruntime.QueueDueKey(), member).Err()
				continue
			}
			validUsers++
			ok, processErr := w.processUser(ctx, userID)
			if processErr != nil {
				failures++
				if firstErr == nil {
					firstErr = processErr
				}
				log.Printf("action worker user failed user_id=%d err=%v", userID, processErr)
				w.recordUserFailure(ctx, userID, processErr)
				continue
			}
			if ok {
				advanced++
			}
		}
		if failures > 0 && failures == validUsers {
			return advanced, errors.Join(errors.New("all action queue users in batch failed"), firstErr)
		}
		return advanced, nil
	}

	userIDs := make([]int64, 0, len(users))
	for _, member := range users {
		userID, parseErr := strconv.ParseInt(strings.TrimSpace(member), 10, 64)
		if parseErr != nil || userID <= 0 {
			_ = w.redis.ZRem(ctx, actionruntime.QueueDueKey(), member).Err()
			continue
		}
		userIDs = append(userIDs, userID)
	}
	results := processActionUsers(ctx, userIDs, w.settings.concurrency, w.processUser)
	advanced := 0
	failures := 0
	var firstErr error
	for _, result := range results {
		if result.err != nil {
			failures++
			if firstErr == nil {
				firstErr = result.err
			}
			log.Printf("action worker user failed user_id=%d err=%v", result.userID, result.err)
			w.recordUserFailure(ctx, result.userID, result.err)
			continue
		}
		if result.advanced {
			advanced++
		}
	}
	if failures > 0 && failures == len(results) {
		return advanced, errors.Join(errors.New("all action queue users in batch failed"), firstErr)
	}
	return advanced, nil
}

type actionUserProcessResult struct {
	userID   int64
	advanced bool
	err      error
}

func processActionUsers(
	ctx context.Context,
	userIDs []int64,
	concurrency int,
	process func(context.Context, int64) (bool, error),
) []actionUserProcessResult {
	if len(userIDs) == 0 || process == nil {
		return nil
	}
	if concurrency <= 0 {
		concurrency = 1
	}
	if concurrency > len(userIDs) {
		concurrency = len(userIDs)
	}
	if concurrency == 1 {
		out := make([]actionUserProcessResult, 0, len(userIDs))
		for _, userID := range userIDs {
			advanced, err := process(ctx, userID)
			out = append(out, actionUserProcessResult{userID: userID, advanced: advanced, err: err})
		}
		return out
	}
	jobs := make(chan int64)
	results := make(chan actionUserProcessResult, len(userIDs))
	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for userID := range jobs {
				advanced, err := process(ctx, userID)
				results <- actionUserProcessResult{userID: userID, advanced: advanced, err: err}
			}
		}()
	}
	for _, userID := range userIDs {
		jobs <- userID
	}
	close(jobs)
	wg.Wait()
	close(results)

	out := make([]actionUserProcessResult, 0, len(userIDs))
	for result := range results {
		out = append(out, result)
	}
	return out
}

func (w *actionWorker) processUser(ctx context.Context, userID int64) (bool, error) {
	token := strconv.FormatInt(time.Now().UnixNano(), 10)
	claimKey := actionruntime.ClaimKey(userID)
	claimed, err := w.redis.SetNX(ctx, claimKey, token, w.settings.claimTTL).Result()
	if err != nil || !claimed {
		return false, err
	}
	defer func() {
		_, _ = releaseClaimScript.Run(context.Background(), w.redis, []string{claimKey}, token).Result()
	}()

	if skip, retryAt, err := w.shouldSkipUser(ctx, userID); skip || err != nil {
		if err != nil {
			return false, err
		}
		member := strconv.FormatInt(userID, 10)
		if retryAt.IsZero() {
			_ = w.redis.ZRem(ctx, actionruntime.QueueDueKey(), member).Err()
		} else {
			_ = w.redis.ZAdd(ctx, actionruntime.QueueDueKey(), redis.Z{
				Score:  float64(retryAt.UnixMilli()),
				Member: member,
			}).Err()
		}
		w.clearUserFailure(ctx, userID)
		return false, nil
	}

	userCtx, cancel := context.WithTimeout(store.WithAuditSource(ctx, actionWorkerAuditLabel), w.settings.userTimeout)
	defer cancel()
	result, err := w.runtime.AdvanceDueForWorker(userCtx, userID)
	if err != nil {
		if errors.Is(err, store.ErrUserBanned) {
			_ = w.redis.ZRem(ctx, actionruntime.QueueDueKey(), strconv.FormatInt(userID, 10)).Err()
			w.clearUserFailure(ctx, userID)
			return false, nil
		}
		return false, err
	}
	w.scheduleNextDue(ctx, userID, result.Schedule)
	w.clearUserFailure(ctx, userID)
	if err := w.publishResult(ctx, userID, result); err != nil {
		log.Printf("action worker publish failed user_id=%d err=%v", userID, err)
	}
	return true, nil
}

func (w *actionWorker) shouldSkipUser(ctx context.Context, userID int64) (bool, time.Time, error) {
	if userID <= 0 {
		return true, time.Time{}, nil
	}
	presence, err := w.store.GetPresence(ctx, userID)
	if err != nil {
		return false, time.Time{}, err
	}
	active, err := w.activePresenceSession(ctx, userID, presence.SessionVersion)
	if err != nil {
		return false, time.Time{}, err
	}
	if !active {
		now := time.Now()
		start := presence.LastSeenAt
		if strings.EqualFold(strings.TrimSpace(presence.Status), "online") {
			start = start.Add(w.visibilityOfflineDelay)
		}
		if start.IsZero() || !now.After(start.Add(store.OfflineBackgroundInterval)) {
			return true, now.Add(store.OfflineBackgroundInterval), nil
		}
		workCtx, cancel := context.WithTimeout(store.WithAuditSource(ctx, "offline_background"), w.settings.userTimeout)
		defer cancel()
		next, err := w.runtime.AdvanceBackgroundOffline(workCtx, userID, presence, start, now)
		return true, next, err
	}
	ready, err := w.store.HasOfflineRecovery(ctx, userID)
	if err != nil {
		return false, time.Time{}, err
	}
	if ready {
		return true, time.Now().Add(time.Second), nil
	}
	return false, time.Time{}, nil
}

func (w *actionWorker) recoverBackgroundSchedules(ctx context.Context) error {
	page, err := w.store.ListBackgroundActionUsers(ctx, w.backgroundScanAfter, 256)
	if err != nil {
		return err
	}
	pipe := w.redis.Pipeline()
	for _, id := range page.UserIDs {
		pipe.ZAddNX(ctx, actionruntime.QueueDueKey(), redis.Z{Member: strconv.FormatInt(id, 10), Score: float64(time.Now().Add(time.Duration(id%5000) * time.Millisecond).UnixMilli())})
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return err
	}
	w.backgroundScanAfter = page.NextAfter
	return nil
}

func (w *actionWorker) activePresenceSession(ctx context.Context, userID, version int64) (bool, error) {
	if userID <= 0 || version <= 0 {
		return false, nil
	}
	count, err := w.redis.Get(ctx, actionruntime.PresenceSessionConnKey(userID, version)).Int64()
	if errors.Is(err, redis.Nil) {
		return false, nil
	}
	return count > 0, err
}

func (w *actionWorker) scheduleNextDue(ctx context.Context, userID int64, queue []store.ActionQueueScheduleItem) {
	var blocked map[int64]struct{}
	if actionScheduleHasKitty(queue) {
		blocked = w.loadCombatBlockedKitties(ctx, userID)
	}
	next, ok := actionruntime.ComputeNextDueSchedule(queue, blocked, time.Now())
	member := strconv.FormatInt(userID, 10)
	if !ok {
		_ = w.redis.ZRem(ctx, actionruntime.QueueDueKey(), member).Err()
		return
	}
	if err := w.redis.ZAdd(ctx, actionruntime.QueueDueKey(), redis.Z{
		Score:  float64(next.UnixMilli()),
		Member: member,
	}).Err(); err != nil {
		log.Printf("action worker schedule due failed user_id=%d err=%v", userID, err)
	}
}

func actionScheduleHasKitty(queue []store.ActionQueueScheduleItem) bool {
	for _, item := range queue {
		if item.ActorType == store.ActionActorTypeKitty {
			return true
		}
	}
	return false
}

func (w *actionWorker) recoverDeadUsers(ctx context.Context) (int64, error) {
	if w.redis == nil {
		return 0, nil
	}
	members, err := w.redis.ZRange(ctx, actionruntime.DeadUsersKey(), 0, actionWorkerDeadRecoveryLimit-1).Result()
	if err != nil || len(members) == 0 {
		return 0, err
	}
	if len(members) == actionWorkerDeadRecoveryLimit {
		log.Printf("action worker dead-user startup recovery capped limit=%d", actionWorkerDeadRecoveryLimit)
	}
	validMembers := make([]string, 0, len(members))
	for _, member := range members {
		userID, parseErr := strconv.ParseInt(strings.TrimSpace(member), 10, 64)
		if parseErr != nil || userID <= 0 {
			continue
		}
		validMembers = append(validMembers, member)
	}
	if len(validMembers) == 0 {
		return 0, nil
	}
	recoveryWindow := w.settings.deadRetryDelay
	if recoveryWindow <= 0 {
		recoveryWindow = 5 * time.Minute
	}
	if w.settings.deadRetryMax > recoveryWindow {
		recoveryWindow = w.settings.deadRetryMax
	}
	step := recoveryWindow / time.Duration(len(validMembers))
	now := time.Now()
	dueMembers := make([]redis.Z, 0, len(validMembers))
	for i, member := range validMembers {
		dueMembers = append(dueMembers, redis.Z{
			Score:  float64(now.Add(time.Duration(i) * step).UnixMilli()),
			Member: member,
		})
	}
	return w.redis.ZAddArgs(ctx, actionruntime.QueueDueKey(), redis.ZAddArgs{
		NX:      true,
		Members: dueMembers,
	}).Result()
}

func (w *actionWorker) recordUserFailure(ctx context.Context, userID int64, cause error) {
	if userID <= 0 {
		return
	}
	member := strconv.FormatInt(userID, 10)
	count, err := w.redis.Incr(ctx, actionruntime.FailureCountKey(userID)).Result()
	if err != nil {
		log.Printf("action worker failure record incr failed user_id=%d err=%v", userID, err)
		return
	}
	if w.settings.failureTTL > 0 {
		_ = w.redis.Expire(ctx, actionruntime.FailureCountKey(userID), w.settings.failureTTL).Err()
		_ = w.redis.Expire(ctx, actionruntime.FailureInfoKey(userID), w.settings.failureTTL).Err()
	}
	lastError := strings.TrimSpace(cause.Error())
	if lastError == "" {
		lastError = "unknown"
	}
	now := time.Now()
	if count >= w.settings.deadAfterFailure {
		deadRetryDelay := w.deadRetryDelayFor(count)
		nextDue := now.Add(deadRetryDelay)
		pipe := w.redis.TxPipeline()
		pipe.ZAdd(ctx, actionruntime.QueueDueKey(), redis.Z{Score: float64(nextDue.UnixMilli()), Member: member})
		pipe.ZAdd(ctx, actionruntime.DeadUsersKey(), redis.Z{Score: float64(now.UnixMilli()), Member: member})
		pipe.HSet(ctx, actionruntime.DeadInfoKey(userID), map[string]any{
			"user_id":       userID,
			"failures":      count,
			"last_error":    lastError,
			"dead_at_ms":    now.UnixMilli(),
			"next_retry_ms": nextDue.UnixMilli(),
			"worker_source": actionWorkerAuditLabel,
		})
		if _, execErr := pipe.Exec(ctx); execErr != nil {
			log.Printf("action worker dead-letter failed user_id=%d failures=%d err=%v", userID, count, execErr)
			return
		}
		log.Printf("action worker user circuit opened user_id=%d failures=%d retry_in_ms=%d last_error=%s", userID, count, deadRetryDelay.Milliseconds(), lastError)
		return
	}
	delay := w.retryDelay(count)
	nextDue := now.Add(delay)
	pipe := w.redis.TxPipeline()
	pipe.ZAdd(ctx, actionruntime.QueueDueKey(), redis.Z{Score: float64(nextDue.UnixMilli()), Member: member})
	pipe.HSet(ctx, actionruntime.FailureInfoKey(userID), map[string]any{
		"user_id":       userID,
		"failures":      count,
		"last_error":    lastError,
		"next_retry_ms": nextDue.UnixMilli(),
		"worker_source": actionWorkerAuditLabel,
	})
	if w.settings.failureTTL > 0 {
		pipe.Expire(ctx, actionruntime.FailureInfoKey(userID), w.settings.failureTTL)
	}
	if _, execErr := pipe.Exec(ctx); execErr != nil {
		log.Printf("action worker retry schedule failed user_id=%d failures=%d err=%v", userID, count, execErr)
		return
	}
	log.Printf("action worker user retry scheduled user_id=%d failures=%d delay_ms=%d", userID, count, delay.Milliseconds())
}

func (w *actionWorker) retryDelay(failures int64) time.Duration {
	if failures <= 1 {
		return w.settings.retryBaseDelay
	}
	delay := w.settings.retryBaseDelay
	for i := int64(1); i < failures; i++ {
		delay *= 2
		if delay >= w.settings.retryMaxDelay {
			return w.settings.retryMaxDelay
		}
	}
	if delay <= 0 {
		return w.settings.retryBaseDelay
	}
	return delay
}

func (w *actionWorker) deadRetryDelayFor(failures int64) time.Duration {
	delay := w.settings.deadRetryDelay
	if delay <= 0 {
		delay = 5 * time.Minute
	}
	maxDelay := w.settings.deadRetryMax
	if maxDelay < delay {
		maxDelay = delay
	}
	extraFailures := failures - w.settings.deadAfterFailure
	for i := int64(0); i < extraFailures; i++ {
		if delay >= maxDelay/2 {
			return maxDelay
		}
		delay *= 2
	}
	return min(delay, maxDelay)
}

func (w *actionWorker) clearUserFailure(ctx context.Context, userID int64) {
	if userID <= 0 {
		return
	}
	member := strconv.FormatInt(userID, 10)
	if err := w.redis.Del(ctx, actionruntime.FailureCountKey(userID), actionruntime.FailureInfoKey(userID), actionruntime.DeadInfoKey(userID)).Err(); err != nil {
		log.Printf("action worker clear failure keys failed user_id=%d err=%v", userID, err)
	}
	if err := w.redis.ZRem(ctx, actionruntime.DeadUsersKey(), member).Err(); err != nil {
		log.Printf("action worker clear dead user failed user_id=%d err=%v", userID, err)
	}
}

func (w *actionWorker) loadCombatBlockedKitties(ctx context.Context, userID int64) map[int64]struct{} {
	members, err := w.redis.SMembers(ctx, actionruntime.KittyCombatBlockedKey(userID)).Result()
	if err != nil {
		log.Printf("action worker load kitty combat projection failed user_id=%d err=%v", userID, err)
		return nil
	}
	blocked := make(map[int64]struct{}, len(members))
	for _, member := range members {
		kittyID, parseErr := strconv.ParseInt(strings.TrimSpace(member), 10, 64)
		if parseErr != nil || kittyID <= 0 {
			continue
		}
		blocked[kittyID] = struct{}{}
	}
	return blocked
}

func (w *actionWorker) publishResult(ctx context.Context, userID int64, result *actionruntime.Result) error {
	if result == nil {
		return nil
	}
	pendingID, pendingAtMS, equipmentChanged, err := w.recordPendingSync(ctx, userID, result)
	if err != nil {
		log.Printf("action worker pending sync record failed user_id=%d err=%v", userID, err)
		equipmentChanged = actionruntime.EquipmentChanged(result.Summary)
	}
	return w.mq.PublishJSON(ctx, actionruntime.SettledTopic, actionruntime.SettledEvent{
		UserID:           userID,
		Source:           actionWorkerAuditLabel,
		PendingID:        pendingID,
		PendingAtMS:      pendingAtMS,
		EquipmentChanged: equipmentChanged,
		Summary:          result.Summary,
	})
}

func (w *actionWorker) recordPendingSync(ctx context.Context, userID int64, result *actionruntime.Result) (string, int64, bool, error) {
	if result == nil || userID <= 0 {
		return "", 0, false, nil
	}
	now := time.Now()
	pendingAtMS := now.UnixMilli()
	pendingID := actionruntime.PendingSyncID(userID, now.UnixNano())
	equipmentChanged := actionruntime.EquipmentChanged(result.Summary)
	summaryJSON, err := json.Marshal(result.Summary)
	if err != nil {
		return "", 0, false, err
	}
	pipe := w.redis.TxPipeline()
	pipe.ZAdd(ctx, actionruntime.PendingSyncUsersKey(), redis.Z{
		Score:  float64(pendingAtMS),
		Member: pendingID,
	})
	pipe.HSet(ctx, actionruntime.PendingSyncInfoKey(pendingID), map[string]any{
		"user_id":           userID,
		"pending_id":        pendingID,
		"pending_at_ms":     pendingAtMS,
		"equipment_changed": equipmentChanged,
		"summary":           string(summaryJSON),
		"source":            actionWorkerAuditLabel,
	})
	_, err = pipe.Exec(ctx)
	if err != nil {
		return "", 0, false, err
	}
	return pendingID, pendingAtMS, equipmentChanged, nil
}

func loadWorkerConfig() workerConfig {
	return workerConfig{
		batchSize:        int64(min(envInt("ACTION_WORKER_BATCH_SIZE", 8), 256)),
		concurrency:      min(envInt("ACTION_WORKER_CONCURRENCY", 1), 4),
		userTimeout:      time.Duration(envInt("ACTION_WORKER_USER_TIMEOUT_MS", 2000)) * time.Millisecond,
		pollInterval:     time.Duration(envInt("ACTION_WORKER_POLL_MS", 200)) * time.Millisecond,
		claimTTL:         time.Duration(envInt("ACTION_WORKER_CLAIM_TTL_MS", 5000)) * time.Millisecond,
		stuckExitAfter:   time.Duration(envInt("ACTION_WORKER_STUCK_EXIT_MS", 120000)) * time.Millisecond,
		healthLogPeriod:  time.Duration(envInt("ACTION_WORKER_HEALTH_LOG_MS", 30000)) * time.Millisecond,
		retryBaseDelay:   time.Duration(envInt("ACTION_WORKER_RETRY_BASE_MS", 1000)) * time.Millisecond,
		retryMaxDelay:    time.Duration(envInt("ACTION_WORKER_RETRY_MAX_MS", 30000)) * time.Millisecond,
		failureTTL:       time.Duration(envInt("ACTION_WORKER_FAILURE_TTL_MS", 86400000)) * time.Millisecond,
		deadAfterFailure: int64(envInt("ACTION_WORKER_DEAD_AFTER_FAILURES", 10)),
		deadRetryDelay:   time.Duration(envInt("ACTION_WORKER_DEAD_RETRY_MS", 300000)) * time.Millisecond,
		deadRetryMax:     time.Duration(envInt("ACTION_WORKER_DEAD_RETRY_MAX_MS", 3600000)) * time.Millisecond,
	}
}

func envInt(key string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func storeRuntimeConfig(cfg config.Config, gameData catalog.Data) store.RuntimeConfig {
	kittyPersonalityRows := gamedata.KittyPersonalityConfigsFromCatalog(gameData.KittyPersonalities)
	kittyDefaultPersonalityID := gamedata.ResolveDefaultKittyPersonalityID(cfg.SharedGlobalSettings.Kitty.DefaultPersonalityID, kittyPersonalityRows)
	derivedFormulas := make([]store.DerivedStatFormula, 0, len(cfg.GlobalSettings.DerivedStats.Formulas))
	for _, formula := range cfg.GlobalSettings.DerivedStats.Formulas {
		terms := make([]store.DerivedStatFormulaTerm, 0, len(formula.Terms))
		for _, term := range formula.Terms {
			terms = append(terms, store.DerivedStatFormulaTerm{
				AttrKey:     term.AttrKey,
				Coefficient: term.Coefficient,
			})
		}
		derivedFormulas = append(derivedFormulas, store.DerivedStatFormula{
			TargetAttr:  formula.TargetAttr,
			IncludeBase: formula.IncludeBase,
			Terms:       terms,
		})
	}
	return store.RuntimeConfig{
		AchievementsEnabled:              cfg.AchievementsEnabled,
		WarehouseDefaultCols:             cfg.GlobalSettings.Warehouse.DefaultCols,
		WarehouseDefaultRows:             cfg.GlobalSettings.Warehouse.DefaultRows,
		WarehouseLogDefaultLookback:      time.Duration(cfg.GlobalSettings.Warehouse.LogDefaultLookbackDays) * 24 * time.Hour,
		OfflineProgressMaxDuration:       time.Duration(cfg.GlobalSettings.Activity.OfflineProgressMaxDurationSeconds) * time.Second,
		OfflineSettleExecBudget:          time.Duration(cfg.GlobalSettings.ActionQueue.FastForward.MaxExecMs) * time.Millisecond,
		ActionQueueFastForwardMaxSteps:   cfg.GlobalSettings.ActionQueue.FastForward.MaxSteps,
		OrdinarySettlementEnabled:        cfg.OrdinarySettlementEnabled,
		ActionQueueFastForwardMaxExec:    time.Duration(cfg.GlobalSettings.ActionQueue.FastForward.MaxExecMs) * time.Millisecond,
		ActionQueueLaneLimit:             int(cfg.SharedGlobalSettings.Action.QueueLaneLimit),
		KittyActionQueueLaneLimit:        int(cfg.SharedGlobalSettings.Action.KittyQueueLaneLimit),
		KittyBattleLevelCapDefault:       cfg.SharedGlobalSettings.Kitty.BattleLevelCapDefault,
		KittyBattleLevelCapPerKittyLevel: cfg.SharedGlobalSettings.Kitty.BattleLevelCapPerKittyLevel,
		KittyMaxPersonalities:            cfg.SharedGlobalSettings.Kitty.MaxPersonalities,
		KittyDefaultPersonalityID:        kittyDefaultPersonalityID,
		KittyPersonalityProfiles:         gamedata.KittyPersonalityProfileMap(kittyPersonalityRows),
		KittyPersonalityOrder:            gamedata.KittyPersonalityIDOrder(kittyPersonalityRows),
		KittyMaxLevel:                    cfg.KittySettings.Leveling.MaxLevel,
		KittyLevelExp:                    kittyLevelExpMap(cfg.KittySettings),
		KittyLevelExpBase:                cfg.KittySettings.Leveling.FallbackFormula.Base,
		KittyLevelExpPerLevel:            cfg.KittySettings.Leveling.FallbackFormula.PerLevel,
		KittyLevelExpPower:               cfg.KittySettings.Leveling.FallbackFormula.Power,
		AuctionFeeRateBps:                cfg.SharedGlobalSettings.Auction.FeeRateBps,
		AuctionMinBidIncrement:           cfg.SharedGlobalSettings.Auction.MinBidIncrement,
		AuctionAntiSnipeTriggerSeconds:   cfg.SharedGlobalSettings.Auction.AntiSnipeTriggerSeconds,
		AuctionAntiSnipeExtendSeconds:    cfg.SharedGlobalSettings.Auction.AntiSnipeExtendSeconds,
		AuctionMinRepLevel:               cfg.SharedGlobalSettings.Auction.MinRepLevel,
		ItemRenameCost:                   cfg.SharedGlobalSettings.ItemRename.Cost,
		ItemRenameMaxNameRunes:           cfg.SharedGlobalSettings.ItemRename.MaxNameRunes,
		ItemRenameLabelColors:            cfg.SharedGlobalSettings.ItemRename.LabelColors,
		ActionEnhanceInheritanceRatio:    cfg.SharedGlobalSettings.Action.EnhanceInheritanceRatio,
		ItemRecycleGoldCoefficient:       cfg.GlobalSettings.Economy.ItemRecycleGoldCoefficient,
		SkillLevelUpBaseExp:              cfg.GlobalSettings.Skill.LevelUpBaseExp,
		SkillLevelUpPerLevelExtraExp:     cfg.GlobalSettings.Skill.LevelUpPerLevelExtraExp,
		Battle: store.BattleRuntimeConfig{
			DropLuckFactor:        cfg.GlobalSettings.Battle.DropLuckFactor,
			ExpBonusStart:         time.Duration(cfg.GlobalSettings.Battle.ExpBonusStartSeconds) * time.Second,
			ExpBonusPerSecond:     cfg.GlobalSettings.Battle.ExpBonusPerSecond,
			ExpBonusMaxMultiplier: cfg.GlobalSettings.Battle.ExpBonusMaxMultiplier,
			ResetItemID:           cfg.GlobalSettings.Battle.ResetItemID,
			ResetItemCount:        cfg.GlobalSettings.Battle.ResetItemCount,
		},
		DerivedStatsFormula: store.DerivedStatsFormulaConfig{
			Version:   cfg.GlobalSettings.DerivedStats.Version,
			Operators: append([]string(nil), cfg.GlobalSettings.DerivedStats.Operators...),
			Formulas:  derivedFormulas,
		},
		Alchemy: store.AlchemyRuntimeConfig{
			InfusionBaseGoldCost:           cfg.SharedGlobalSettings.Alchemy.InfusionBaseGoldCost,
			InfusionBaseDurationSec:        cfg.SharedGlobalSettings.Alchemy.InfusionBaseDurationSec,
			InfusionBaseExp:                cfg.SharedGlobalSettings.Alchemy.InfusionBaseExp,
			DurationDefaultSec:             cfg.SharedGlobalSettings.Alchemy.DurationsSec.Default,
			DurationConvertSec:             cfg.SharedGlobalSettings.Alchemy.DurationsSec.Convert,
			DurationBenchSec:               cfg.SharedGlobalSettings.Alchemy.DurationsSec.Bench,
			DurationTransmuteSec:           cfg.SharedGlobalSettings.Alchemy.DurationsSec.Transmute,
			DurationSmeltSec:               cfg.SharedGlobalSettings.Alchemy.DurationsSec.Smelt,
			TransmuteDurationByRarityIndex: append([]int(nil), cfg.SharedGlobalSettings.Alchemy.TransmuteDurationByRarityIndex...),
			LevelDeficitFailChancePerLevel: cfg.SharedGlobalSettings.Alchemy.LevelDeficitFailChancePerLevel,
			LevelDeficitFailChanceCap:      cfg.SharedGlobalSettings.Alchemy.LevelDeficitFailChanceCap,
			Inherit: store.AlchemyInheritRuntimeConfig{
				BaseInheritSlots:             cfg.SharedGlobalSettings.Alchemy.Inherit.BaseInheritSlots,
				BaseSuppressSlots:            cfg.SharedGlobalSettings.Alchemy.Inherit.BaseSuppressSlots,
				InheritSlotPrecisionScale:    cfg.SharedGlobalSettings.Alchemy.Inherit.InheritSlotPrecisionScale,
				SuppressSlotStabilityBonus:   cfg.SharedGlobalSettings.Alchemy.Inherit.SuppressSlotStabilityBonus,
				PositiveIgnoreKeepChanceBase: cfg.SharedGlobalSettings.Alchemy.Inherit.PositiveIgnoreKeepChanceBase,
				NegativeIgnoreKeepChanceBase: cfg.SharedGlobalSettings.Alchemy.Inherit.NegativeIgnoreKeepChanceBase,
				HybridMergeDivisor:           cfg.SharedGlobalSettings.Alchemy.Inherit.HybridMergeDivisor,
				InbreedMutationChance:        cfg.SharedGlobalSettings.Alchemy.Inherit.InbreedMutationChance,
				InbreedGoodChance:            cfg.SharedGlobalSettings.Alchemy.Inherit.InbreedGoodChance,
				InbreedGoodMultiplierRange:   catalog.AlchemyInheritMultiplierRange{Min: cfg.SharedGlobalSettings.Alchemy.Inherit.InbreedGoodMultiplierRange.Min, Max: cfg.SharedGlobalSettings.Alchemy.Inherit.InbreedGoodMultiplierRange.Max},
				InbreedBadMultiplierRange:    catalog.AlchemyInheritMultiplierRange{Min: cfg.SharedGlobalSettings.Alchemy.Inherit.InbreedBadMultiplierRange.Min, Max: cfg.SharedGlobalSettings.Alchemy.Inherit.InbreedBadMultiplierRange.Max},
				MutationChance:               cfg.SharedGlobalSettings.Alchemy.Inherit.MutationChance,
				NumericValueCapMultiplier:    cfg.SharedGlobalSettings.Alchemy.Inherit.NumericValueCapMultiplier,
			},
		},
		Fishing: cfg.GlobalSettings.Fishing,
	}
}

func kittyLevelExpMap(settings config.KittySettings) map[int]int64 {
	out := make(map[int]int64, len(settings.Leveling.Levels))
	for _, row := range settings.Leveling.Levels {
		if row.Level > 0 && row.NextExp > 0 {
			out[row.Level] = row.NextExp
		}
	}
	return out
}
