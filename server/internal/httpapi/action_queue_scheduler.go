package httpapi

import (
	"context"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"mysecondwebgame/server/internal/actionruntime"
	"mysecondwebgame/server/internal/store"
)

type KittyStatusChange struct {
	UserID  int64
	KittyID int64
	Status  string
}

func actionQueueKittyCombatBlockedKey(userID int64) string {
	return actionruntime.KittyCombatBlockedKey(userID)
}

func normalizeKittyStatus(status string) string {
	switch strings.TrimSpace(status) {
	case store.KittyStatusInCombat:
		return store.KittyStatusInCombat
	case store.KittyStatusInLifeQueue:
		return store.KittyStatusInLifeQueue
	default:
		return store.KittyStatusIdle
	}
}

func isKittyCombatBlockedStatus(status string) bool {
	return normalizeKittyStatus(status) == store.KittyStatusInCombat
}

func computeActionQueueNextDue(queue []store.ActionQueueItem, blocked map[int64]struct{}, now time.Time) (time.Time, bool) {
	return actionruntime.ComputeNextDue(queue, blocked, now)
}

func (s *Server) loadActionQueueCombatBlockedKitties(ctx context.Context, userID int64) map[int64]struct{} {
	if userID <= 0 || s.deps.Redis == nil {
		return nil
	}
	members, err := s.deps.Redis.SMembers(ctx, actionQueueKittyCombatBlockedKey(userID)).Result()
	if err != nil {
		log.Printf("load action queue kitty combat projection failed user_id=%d err=%v", userID, err)
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

func (s *Server) syncActionQueueKittyCombatProjection(ctx context.Context, userID int64, kitties []store.PlayerKitty) error {
	if userID <= 0 || s.deps.Redis == nil {
		return nil
	}
	key := actionQueueKittyCombatBlockedKey(userID)
	pipe := s.deps.Redis.TxPipeline()
	pipe.Del(ctx, key)
	blockedMembers := make([]any, 0, len(kitties))
	for _, kitty := range kitties {
		if kitty.ID <= 0 || !isKittyCombatBlockedStatus(kitty.Status) {
			continue
		}
		blockedMembers = append(blockedMembers, strconv.FormatInt(kitty.ID, 10))
	}
	if len(blockedMembers) > 0 {
		pipe.SAdd(ctx, key, blockedMembers...)
	}
	_, err := pipe.Exec(ctx)
	if err != nil {
		return err
	}
	return nil
}

func (s *Server) rebuildActionQueueKittyCombatProjection(ctx context.Context, userID int64) {
	if userID <= 0 || s.deps.Redis == nil {
		return
	}
	kitties, err := s.deps.Store.ListPlayerKitties(ctx, userID)
	if err != nil {
		log.Printf("rebuild action queue kitty combat projection failed user_id=%d err=%v", userID, err)
		return
	}
	if err := s.syncActionQueueKittyCombatProjection(ctx, userID, kitties); err != nil {
		log.Printf("sync action queue kitty combat projection failed user_id=%d err=%v", userID, err)
	}
}

func (s *Server) applyKittyStatusChangesProjection(ctx context.Context, changes []KittyStatusChange) []int64 {
	if len(changes) == 0 || s.deps.Redis == nil {
		return nil
	}
	uniqueUsers := make(map[int64]struct{}, len(changes))
	for _, change := range changes {
		if change.UserID <= 0 || change.KittyID <= 0 {
			continue
		}
		member := strconv.FormatInt(change.KittyID, 10)
		var err error
		if isKittyCombatBlockedStatus(change.Status) {
			err = s.deps.Redis.SAdd(ctx, actionQueueKittyCombatBlockedKey(change.UserID), member).Err()
		} else {
			err = s.deps.Redis.SRem(ctx, actionQueueKittyCombatBlockedKey(change.UserID), member).Err()
		}
		if err != nil {
			log.Printf("apply kitty combat projection failed user_id=%d kitty_id=%d status=%s err=%v", change.UserID, change.KittyID, change.Status, err)
			continue
		}
		uniqueUsers[change.UserID] = struct{}{}
	}
	userIDs := make([]int64, 0, len(uniqueUsers))
	for userID := range uniqueUsers {
		userIDs = append(userIDs, userID)
	}
	return userIDs
}

func (s *Server) NotifyKittyStatusChanged(userID, kittyID int64, status string) {
	s.NotifyKittyStatusesChanged([]KittyStatusChange{{
		UserID:  userID,
		KittyID: kittyID,
		Status:  status,
	}})
}

func (s *Server) NotifyKittyStatusesChanged(changes []KittyStatusChange) {
	userIDs := s.applyKittyStatusChangesProjection(context.Background(), changes)
	if len(userIDs) == 0 {
		return
	}
	s.syncAfterMutation(mutationSyncScope{
		UserIDs:     userIDs,
		PlayerState: true,
		Queue:       true,
	})
}

func (s *Server) scheduleActionPush(userID int64, specs map[string]store.ActionSpec, queue []store.ActionQueueItem) {
	_ = specs
	if userID <= 0 || s.deps.Redis == nil {
		return
	}
	blocked := s.loadActionQueueCombatBlockedKitties(context.Background(), userID)
	next, ok := computeActionQueueNextDue(queue, blocked, time.Now())
	if !ok {
		s.stopActionPush(userID)
		return
	}
	if err := s.deps.Redis.ZAdd(context.Background(), actionQueueDueKey(), redis.Z{
		Score:  float64(next.UnixMilli()),
		Member: strconv.FormatInt(userID, 10),
	}).Err(); err != nil {
		log.Printf("schedule action queue due failed user_id=%d err=%v", userID, err)
		return
	}
}
