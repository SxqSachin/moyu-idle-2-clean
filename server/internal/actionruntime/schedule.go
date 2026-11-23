package actionruntime

import (
	"fmt"
	"strconv"
	"time"

	"mysecondwebgame/server/internal/store"
)

func QueueDueKey() string {
	return "aq:due_users"
}

func ClaimKey(userID int64) string {
	return "aq:claim:" + strconv.FormatInt(userID, 10)
}

func OfflineFastForwardReadyKey(userID int64) string {
	return "aq:offline_ff_ready:" + strconv.FormatInt(userID, 10)
}

func KittyCombatBlockedKey(userID int64) string {
	return "aq:kitty_combat_blocked:" + strconv.FormatInt(userID, 10)
}

func FailureCountKey(userID int64) string {
	return "aq:fail:" + strconv.FormatInt(userID, 10)
}

func FailureInfoKey(userID int64) string {
	return "aq:fail_info:" + strconv.FormatInt(userID, 10)
}

func DeadUsersKey() string {
	return "aq:dead_users"
}

func DeadInfoKey(userID int64) string {
	return "aq:dead_info:" + strconv.FormatInt(userID, 10)
}

func PendingSyncUsersKey() string {
	return "aq:sync_pending_users"
}

func PendingSyncID(userID, version int64) string {
	return strconv.FormatInt(userID, 10) + ":" + strconv.FormatInt(version, 10)
}

func PendingSyncInfoKey(syncID string) string {
	return "aq:sync_pending_info:" + syncID
}

func PendingSyncClaimKey(syncID string) string {
	return "aq:sync_claim:" + syncID
}

func PresenceSessionConnKey(userID, sessionVersion int64) string {
	return fmt.Sprintf("ws:presence:session:%d:%d:connections", userID, sessionVersion)
}

func ComputeNextDue(queue []store.ActionQueueItem, blocked map[int64]struct{}, now time.Time) (time.Time, bool) {
	return computeNextDue(queue, blocked, now, func(item store.ActionQueueItem) (string, int64, string, time.Time) {
		return item.ActorType, item.ActorID, item.Status, item.RunEndsAt
	})
}

func ComputeNextDueSchedule(queue []store.ActionQueueScheduleItem, blocked map[int64]struct{}, now time.Time) (time.Time, bool) {
	return computeNextDue(queue, blocked, now, func(item store.ActionQueueScheduleItem) (string, int64, string, time.Time) {
		return item.ActorType, item.ActorID, item.Status, item.RunEndsAt
	})
}

func computeNextDue[T any](queue []T, blocked map[int64]struct{}, now time.Time, fields func(T) (string, int64, string, time.Time)) (time.Time, bool) {
	var next time.Time
	hasSchedulablePending := false
	for _, item := range queue {
		actorType, actorID, status, runEndsAt := fields(item)
		if actorType == store.ActionActorTypeKitty {
			if _, ok := blocked[actorID]; ok {
				continue
			}
		}
		switch status {
		case "running":
			if runEndsAt.IsZero() {
				continue
			}
			if next.IsZero() || runEndsAt.Before(next) {
				next = runEndsAt
			}
		case "pending":
			hasSchedulablePending = true
		}
	}
	if !next.IsZero() {
		return next, true
	}
	if hasSchedulablePending {
		return now, true
	}
	return time.Time{}, false
}
