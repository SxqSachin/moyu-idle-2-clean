package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"mysecondwebgame/server/internal/store"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
)

const offlineFastForwardEnhanceEventLimit = 100

func offlineFastForwardEnhanceAttemptEvents(rows []store.EnhanceAttemptResult) []store.EnhanceAttemptResult {
	if len(rows) <= offlineFastForwardEnhanceEventLimit {
		return rows
	}
	return rows[len(rows)-offlineFastForwardEnhanceEventLimit:]
}

func (s *Server) handleOfflineFastForwardRequest(conn *websocket.Conn, userID int64, raw json.RawMessage, requestID string) {
	request := parseOfflineFastForwardRequest(raw)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	fail := func(err error) {
		log.Printf("offline recovery read failed user_id=%d err=%v", userID, err)
		response := newOfflineFastForwardResponse(store.ActionFastForwardSummary{}, offlineFastForwardResponseContext{RequestID: requestID, ErrorCode: "OFFLINE_RECOVERY_UNAVAILABLE"})
		response.Status = "retryable"
		response.RetryAfterMS = 1500
		_ = s.writeWSEvent(conn, "action.offline.rewards.result", response)
	}
	if request.ReportPolicy == "combined" && request.AckReportKey != "" {
		id, err := strconv.ParseInt(request.AckTaskID, 10, 64)
		if err == nil {
			err = s.deps.Store.AcknowledgeOfflineRecoveryReportKey(ctx, userID, id, request.AckReportKey)
		}
		if err != nil {
			fail(err)
			return
		}
	} else if request.ReportPolicy == "combined" && len(request.AckTasks) > 0 {
		if err := s.deps.Store.AcknowledgeOfflineRecoveryReport(ctx, userID, request.AckTasks); err != nil {
			fail(err)
			return
		}
	} else if id, err := strconv.ParseInt(request.AckTaskID, 10, 64); err == nil && id > 0 {
		if err = s.deps.Store.AcknowledgeOfflineRecovery(ctx, userID, id, request.AckRevision); err != nil {
			fail(err)
			return
		}
	}
	if id, err := strconv.ParseInt(request.RetryTaskID, 10, 64); err == nil && id > 0 {
		if err = s.deps.Store.RetryOfflineRecovery(ctx, userID, id, request.RetryRevision); err != nil {
			fail(err)
			return
		}
	}
	var report store.OfflineRecoveryReport
	readRecovery := func() (store.OfflineRecovery, error) {
		if request.ReportPolicy == "combined" {
			var err error
			report, err = s.deps.Store.GetOfflineRecoveryReport(ctx, userID)
			return report.Recovery, err
		}
		return s.deps.Store.GetOfflineRecovery(ctx, userID)
	}
	recovery, err := readRecovery()
	if errors.Is(err, pgx.ErrNoRows) {
		// Import an outstanding pre-upgrade marker once. The durable window dedupes
		// subsequent requests, even if the marker expires or the response is lost.
		if ready, state := s.peekOfflineFastForward(userID); ready {
			resumed := time.UnixMilli(state.CreatedAtMS)
			if resumed.Before(time.UnixMilli(state.OfflineTargetAtMS)) {
				resumed = time.UnixMilli(state.OfflineTargetAtMS)
			}
			if _, err = s.deps.Store.EnsureOfflineRecovery(ctx, userID, time.UnixMilli(state.OfflineStartedAtMS), time.UnixMilli(state.OfflineTargetAtMS), resumed); err != nil {
				fail(err)
				return
			}
			s.consumeOfflineFastForwardIfUnchanged(userID, state)
			recovery, err = readRecovery()
		}
	}
	if errors.Is(err, pgx.ErrNoRows) {
		_ = s.writeWSEvent(conn, "action.offline.rewards.result", newOfflineFastForwardResponse(store.ActionFastForwardSummary{}, offlineFastForwardResponseContext{RequestID: requestID}))
		return
	}
	if err != nil {
		fail(err)
		return
	}
	summary := recovery.Summary
	summary.TargetAtMS = recovery.TargetAt.UnixMilli()
	summary.HasMoreDue = recovery.Status == "running"
	summary.YieldReason = ""
	if recovery.Status == "stalled" {
		summary.YieldReason = store.ActionQueueYieldReasonNoProgress
	}
	terminal := recovery.Status == "complete"
	combat := store.OfflineCombatRewards{}
	if terminal {
		// The action offline cap must not hide battle rewards already earned
		// during the remainder of the actual absence.
		if request.ReportPolicy == "combined" {
			combat, err = s.deps.Store.GetOfflineRecoveryReportCombat(ctx, userID, report.Tasks)
		} else {
			combat, err = s.deps.Store.GetOfflineCombatRewards(ctx, userID, recovery.StartedAt, recovery.ResumedAt, recovery.PendingRunIDs)
		}
		if err != nil {
			fail(err)
			return
		}
	}
	// Progress is intentionally compact. The complete cumulative receipt remains
	// queryable in PostgreSQL; client refresh never reconstructs it from deltas.
	responseSummary := summary
	if !terminal {
		responseSummary = compactOfflineRecoverySummary(summary)
		if request.ProtocolVersion < 3 {
			responseSummary.CompletedSteps = 0
		}
	}
	response := newOfflineFastForwardResponse(responseSummary, offlineFastForwardResponseContext{
		RequestID: requestID, OfflineDurationSeconds: int64(recovery.TargetAt.Sub(recovery.StartedAt) / time.Second),
		CombatItemRewards: combat.Items, ProjectionDeferred: !terminal,
	})
	response.TaskID = strconv.FormatInt(recovery.ID, 10)
	if request.ReportPolicy == "combined" {
		response.ReportKey = report.Key
		response.ReportTasks = report.Tasks
		response.OfflineDurationSeconds = report.DurationSeconds
	}
	response.EnhanceResults = offlineFastForwardEnhanceAttemptEvents(response.EnhanceResults)
	response.Revision = recovery.Revision
	response.Cumulative = true
	response.StartedAtMS = recovery.StartedAt.UnixMilli()
	response.RetryAfterMS = 750
	response.CombatPendingRuns = combat.PendingRuns
	response.CombatLegacyLogs = combat.LegacyLogs
	response.RecoveryErrorCode = recovery.ErrorCode
	if terminal {
		response.FishingMutations = s.deps.Store.FishingMutationCatalog()
		if !request.hasSyncedRecoverySnapshot(response.TaskID, recovery.Revision, combat.PendingRuns) ||
			(request.ReportPolicy == "combined" && request.SyncedReportKey != report.Key) {
			if inventorySync, err := s.syncOfflineRecoveryDomains(ctx, userID, summary); err != nil {
				log.Printf("offline recovery projection deferred user_id=%d recovery_id=%d err=%v", userID, recovery.ID, err)
				response.SyncPending = true
			} else {
				response.SnapshotSynced = true
				response.InventorySync = &inventorySync
			}
		} else {
			response.SnapshotSynced = true
		}
	}
	_ = s.writeWSEvent(conn, "action.offline.rewards.result", response)
}

// Only terminal reads refresh domains not covered by game.state/action.queue.
// Use the request deadline; failure retains the receipt for a later retry.
func (s *Server) syncOfflineRecoveryDomains(ctx context.Context, userID int64, summary store.ActionFastForwardSummary) (wsPatchMeta, error) {
	// The bootstrap may predate the final committed chunk. game.state.get does
	// not include inventory; never acknowledge a stale cached warehouse view.
	items, err := s.listInventoryInstances(ctx, userID, false)
	if err != nil {
		return wsPatchMeta{}, err
	}
	meta := s.pushInventoryInstancesPatch(userID, items)
	if err := s.syncOfflineRecoveryPlayer(ctx, userID, summary); err != nil {
		return meta, err
	}
	if summary.OperationEffectsChanged {
		projection, err := s.deps.Store.ListOperationEffects(ctx, userID, time.Now())
		if err != nil {
			return meta, err
		}
		s.pushUserEventCluster(userID, "operation.effects.updated", projection)
	}
	if summary.FishingChanged {
		trip, err := s.deps.Store.GetCurrentFishingTrip(ctx, userID)
		if err != nil {
			return meta, err
		}
		if trip == nil {
			trip, err = s.deps.Store.GetLatestFishingTrip(ctx, userID)
			if err != nil {
				return meta, err
			}
		}
		s.pushUserEventClusterOnce(userID, "game.fishing.trip", fiber.Map{"trip": trip})
	}
	for _, actor := range summary.ActorSummaries {
		if actor.ActorType != store.ActionActorTypeKitty || actor.ActorID <= 0 {
			continue
		}
		if actor.EquipmentChanged {
			if _, _, err := s.pushKittyEquipmentNow(ctx, userID, actor.ActorID); err != nil {
				return meta, err
			}
		}
		if actor.EquipmentChanged || len(actor.AttrExp) > 0 {
			if _, _, err := s.pushKittyAttributesNow(ctx, userID, actor.ActorID); err != nil {
				return meta, err
			}
		}
	}
	return meta, nil
}

func offlineRecoveryPlayerProjection(summary store.ActionFastForwardSummary) (equipment, attributes bool) {
	equipment = len(summary.EnhanceResults) > 0 || len(summary.ActivatedWorkLoadouts) > 0
	attributes = len(summary.AttrExp) > 0
	for _, actor := range summary.ActorSummaries {
		if actor.ActorType == store.ActionActorTypePlayer {
			equipment = equipment || actor.EquipmentChanged
			attributes = attributes || len(actor.AttrExp) > 0
		}
		attributes = attributes || len(actor.OwnerAttrExp) > 0
	}
	return equipment, attributes || equipment
}

// Unlike the ordinary equipment refresh, this only projects committed state;
// it must not reconcile skill settings as a side effect of reading a receipt.
func (s *Server) syncOfflineRecoveryPlayer(ctx context.Context, userID int64, summary store.ActionFastForwardSummary) error {
	equipment, attributes := offlineRecoveryPlayerProjection(summary)
	if equipment {
		items, err := s.deps.Store.ListEquippedItems(ctx, userID)
		if err != nil {
			return err
		}
		s.pushEquipmentInstancesSnapshot(userID, items)
	}
	if !attributes {
		return nil
	}
	seeds, err := s.playerAttributeSeeds()
	if err != nil {
		return err
	}
	states, err := s.deps.Store.ListPlayerAttributes(ctx, userID, seeds)
	if err != nil {
		return err
	}
	progress, err := s.attributeProgressMap(states)
	if err != nil {
		return err
	}
	values := s.attributeMapFromStates(states)
	s.pushPlayerAttributesPatch(userID, values, progress)
	if equipment {
		stats, err := s.deps.Store.GetPlayerStats(ctx, userID)
		if err != nil {
			return err
		}
		s.pushUserEventCluster(userID, "equipment.stats.updated", fiber.Map{"attributes": values, "derived": stats.Derived, "updatedAt": stats.UpdatedAt})
	}
	return nil
}
