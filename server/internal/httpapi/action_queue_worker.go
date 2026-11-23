package httpapi

import (
	"context"
	"errors"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"mysecondwebgame/server/internal/actionruntime"
	"mysecondwebgame/server/internal/store"
)

var releaseClaimScript = redis.NewScript(`
if redis.call("GET", KEYS[1]) == ARGV[1] then
	return redis.call("DEL", KEYS[1])
end
return 0
`)

func (s *Server) startActionQueueWorker() {
	if s.deps.Redis == nil {
		return
	}
	go s.actionQueueWorkerLoop()
}

func (s *Server) actionQueueWorkerLoop() {
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	var scanAfter int64
	var scanAt time.Time
	for {
		select {
		case <-s.actionWorkerStop:
			return
		case <-ticker.C:
			if time.Now().After(scanAt) {
				ctx, cancel := context.WithTimeout(context.Background(), time.Second)
				page, err := s.deps.Store.ListBackgroundActionUsers(ctx, scanAfter, 256)
				if err == nil {
					pipe := s.deps.Redis.Pipeline()
					for _, id := range page.UserIDs {
						pipe.ZAddNX(ctx, actionQueueDueKey(), redis.Z{Member: strconv.FormatInt(id, 10), Score: float64(time.Now().Add(time.Duration(id%5000) * time.Millisecond).UnixMilli())})
					}
					if _, err = pipe.Exec(ctx); err == nil {
						scanAfter = page.NextAfter
					}
				}
				if err != nil {
					log.Printf("background schedule recovery failed err=%v", err)
				}
				cancel()
				scanAt = time.Now().Add(5 * time.Second)
			}
			s.processDueActionQueueUsers(store.WithAuditSource(context.Background(), "action_queue_worker"), 32)
		}
	}
}

func (s *Server) processDueActionQueueUsers(ctx context.Context, batch int64) {
	if s.deps.Redis == nil {
		return
	}
	nowMs := time.Now().UnixMilli()
	users, err := s.deps.Redis.ZRangeByScore(ctx, actionQueueDueKey(), &redis.ZRangeBy{
		Min:    "-inf",
		Max:    strconv.FormatInt(nowMs, 10),
		Offset: 0,
		Count:  batch,
	}).Result()
	if err != nil {
		log.Printf("action queue due fetch failed err=%v", err)
		return
	}
	if len(users) == 0 {
		return
	}
	for _, member := range users {
		userID, parseErr := strconv.ParseInt(strings.TrimSpace(member), 10, 64)
		if parseErr != nil || userID <= 0 {
			_ = s.deps.Redis.ZRem(ctx, actionQueueDueKey(), member).Err()
			continue
		}
		token := strconv.FormatInt(time.Now().UnixNano(), 10)
		claimKey := actionQueueClaimKey(userID)
		claimed, claimErr := s.deps.Redis.SetNX(ctx, claimKey, token, 5*time.Second).Result()
		if claimErr != nil || !claimed {
			continue
		}
		presence, presenceErr := s.deps.Store.GetPresence(ctx, userID)
		if presenceErr != nil {
			log.Printf("action queue worker get presence failed user_id=%d err=%v", userID, presenceErr)
			_, _ = releaseClaimScript.Run(ctx, s.deps.Redis, []string{claimKey}, token).Result()
			continue
		}
		if s.hasOfflineFastForwardReady(userID) {
			_ = s.deps.Redis.ZRem(ctx, actionQueueDueKey(), member).Err()
			_, _ = releaseClaimScript.Run(ctx, s.deps.Redis, []string{claimKey}, token).Result()
			continue
		}
		active := false
		if presence.SessionVersion > 0 {
			count, err := s.deps.Redis.Get(ctx, actionruntime.PresenceSessionConnKey(userID, presence.SessionVersion)).Int64()
			if err != nil && !errors.Is(err, redis.Nil) {
				log.Printf("action queue worker presence backend unavailable user_id=%d err=%v", userID, err)
				_, _ = releaseClaimScript.Run(ctx, s.deps.Redis, []string{claimKey}, token).Result()
				continue
			}
			active = count > 0
		}
		if !active {
			now := time.Now()
			start := presence.LastSeenAt
			if strings.EqualFold(strings.TrimSpace(presence.Status), "online") {
				start = start.Add(s.visibilityOfflineDelay())
			}
			next := now.Add(store.OfflineBackgroundInterval)
			if !start.IsZero() && now.After(start.Add(store.OfflineBackgroundInterval)) {
				workCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
				var err error
				next, err = s.actionRuntime().AdvanceBackgroundOffline(workCtx, userID, presence, start, now)
				cancel()
				if err != nil {
					log.Printf("background action recovery failed user_id=%d err=%v", userID, err)
					next = now.Add(store.OfflineBackgroundInterval)
				}
			}
			if next.IsZero() {
				_ = s.deps.Redis.ZRem(ctx, actionQueueDueKey(), member).Err()
			} else {
				_ = s.deps.Redis.ZAdd(ctx, actionQueueDueKey(), redis.Z{Member: member, Score: float64(next.UnixMilli())}).Err()
			}
			_, _ = releaseClaimScript.Run(ctx, s.deps.Redis, []string{claimKey}, token).Result()
			continue
		}
		result, ffErr := s.actionRuntime().AdvanceDue(ctx, userID)
		if ffErr != nil {
			if errors.Is(ffErr, store.ErrUserBanned) {
				_ = s.deps.Redis.ZRem(ctx, actionQueueDueKey(), member).Err()
				_, _ = releaseClaimScript.Run(ctx, s.deps.Redis, []string{claimKey}, token).Result()
				continue
			}
			log.Printf("action queue worker advance failed user_id=%d err=%v", userID, ffErr)
		} else {
			s.syncActionRuntime(userID, result.Specs, result.Queue, result.State, result.Attr, result.Summary)
			s.pushEnhanceAttemptEvents(userID, result.Summary.EnhanceResults)
			s.pushWorkLoadoutHints(userID, result.Summary)
		}
		_, _ = releaseClaimScript.Run(ctx, s.deps.Redis, []string{claimKey}, token).Result()
	}
}
