package httpapi

import (
	"encoding/json"
	"strings"

	"mysecondwebgame/server/internal/config"
	"mysecondwebgame/server/internal/store"
)

const (
	offlineFastForwardProtocolVersion          = 3
	offlineFastForwardProjectionPolicyTerminal = "terminal"
)

type offlineFastForwardRequest struct {
	AckReportKey            string                         `json:"ackReportKey"`
	ReportPolicy            string                         `json:"reportPolicy"`
	SyncedReportKey         string                         `json:"syncedReportKey"`
	AckTasks                []store.OfflineRecoveryReceipt `json:"ackTasks"`
	SyncedTaskID            string                         `json:"syncedTaskId"`
	SyncedRevision          int64                          `json:"syncedRevision"`
	SyncedCombatPendingRuns int                            `json:"syncedCombatPendingRuns"`
	AckTaskID               string                         `json:"ackTaskId"`
	AckRevision             int64                          `json:"ackRevision"`
	RetryTaskID             string                         `json:"retryTaskId"`
	RetryRevision           int64                          `json:"retryRevision"`
	ProtocolVersion         int                            `json:"protocolVersion"`
	ProjectionPolicy        string                         `json:"projectionPolicy"`
}

func (request offlineFastForwardRequest) hasSyncedRecoverySnapshot(taskID string, revision int64, pendingRuns int) bool {
	return request.SyncedTaskID == taskID && request.SyncedRevision == revision && request.SyncedCombatPendingRuns == pendingRuns
}

type offlineFastForwardResponse struct {
	ReportKey              string                             `json:"reportKey,omitempty"`
	ReportTasks            []store.OfflineRecoveryReceipt     `json:"reportTasks,omitempty"`
	TaskID                 string                             `json:"taskId,omitempty"`
	Revision               int64                              `json:"revision"`
	Cumulative             bool                               `json:"cumulative"`
	StartedAtMS            int64                              `json:"startedAtMS"`
	RetryAfterMS           int                                `json:"retryAfterMS"`
	CombatPendingRuns      int                                `json:"combatPendingRuns"`
	CombatLegacyLogs       bool                               `json:"combatLegacyLogs"`
	RecoveryErrorCode      string                             `json:"recoveryErrorCode,omitempty"`
	ProtocolVersion        int                                `json:"protocolVersion"`
	RequestID              string                             `json:"requestId"`
	Status                 string                             `json:"status"`
	SyncPending            bool                               `json:"syncPending"`
	SnapshotSynced         bool                               `json:"snapshotSynced"`
	InventorySync          *wsPatchMeta                       `json:"inventorySync,omitempty"`
	ProjectionDeferred     bool                               `json:"projectionDeferred"`
	CompletedSteps         int64                              `json:"completedSteps"`
	ItemRewards            map[string]int64                   `json:"itemRewards"`
	ItemCosts              map[string]int64                   `json:"itemCosts"`
	AttrExp                map[string]float64                 `json:"attrExp"`
	ActorSummaries         []store.ActionActorSummary         `json:"actorSummaries"`
	EnhanceResults         []store.EnhanceAttemptResult       `json:"enhanceResults"`
	WorkLoadoutUsages      []store.WorkLoadoutUsage           `json:"workLoadoutUsages"`
	FinalWorkLoadoutID     string                             `json:"finalWorkLoadoutId"`
	QueueCompleted         []int64                            `json:"queueCompleted"`
	QueueCanceled          []store.ActionQueueCancellation    `json:"queueCanceled"`
	OfflineDurationSeconds int64                              `json:"offlineDurationSeconds"`
	CombatItemRewards      map[string]int64                   `json:"combatItemRewards"`
	BudgetExhausted        bool                               `json:"budgetExhausted"`
	HasMoreDue             bool                               `json:"hasMoreDue"`
	YieldReason            string                             `json:"yieldReason"`
	StalledQueueID         int64                              `json:"stalledQueueId"`
	SettledUntilMS         int64                              `json:"settledUntilMS"`
	TargetAtMS             int64                              `json:"targetAtMS"`
	FishingCatches         map[string]store.FishingCatchTally `json:"fishingCatches"`
	FishingMutations       []config.FishingMutationSettings   `json:"fishingMutations"`
	ErrorCode              string                             `json:"errorCode"`
}

type offlineFastForwardResponseContext struct {
	RequestID              string
	OfflineDurationSeconds int64
	CombatItemRewards      map[string]int64
	FishingMutations       []config.FishingMutationSettings
	SyncPending            bool
	SnapshotSynced         bool
	ProjectionDeferred     bool
	ErrorCode              string
}

// Stalled tasks are not terminal receipts. Keep their retry identity/status visible
// without requiring reward or inventory projections to succeed first.
func compactOfflineRecoverySummary(summary store.ActionFastForwardSummary) store.ActionFastForwardSummary {
	return store.ActionFastForwardSummary{
		CompletedSteps: summary.CompletedSteps, HasMoreDue: summary.HasMoreDue,
		TargetAtMS: summary.TargetAtMS, SettledUntilMS: summary.SettledUntilMS,
		YieldReason: summary.YieldReason, StalledQueueID: summary.StalledQueueID,
	}
}

func newOfflineFastForwardResponse(summary store.ActionFastForwardSummary, context offlineFastForwardResponseContext) offlineFastForwardResponse {
	status := offlineFastForwardResultStatus(summary.HasMoreDue, context.SyncPending, summary.YieldReason)
	errorCode := strings.TrimSpace(context.ErrorCode)
	if errorCode != "" {
		status = "failed_before_commit"
	}
	return offlineFastForwardResponse{
		ProtocolVersion:        offlineFastForwardProtocolVersion,
		RequestID:              context.RequestID,
		Status:                 status,
		SyncPending:            context.SyncPending,
		SnapshotSynced:         context.SnapshotSynced,
		ProjectionDeferred:     context.ProjectionDeferred,
		CompletedSteps:         summary.CompletedSteps,
		ItemRewards:            summary.ItemRewards,
		ItemCosts:              summary.ItemCosts,
		AttrExp:                summary.AttrExp,
		ActorSummaries:         summary.ActorSummaries,
		EnhanceResults:         summary.EnhanceResults,
		WorkLoadoutUsages:      summary.WorkLoadoutUsages,
		FinalWorkLoadoutID:     summary.FinalWorkLoadoutID,
		QueueCompleted:         summary.QueueCompleted,
		QueueCanceled:          summary.QueueCanceled,
		OfflineDurationSeconds: context.OfflineDurationSeconds,
		CombatItemRewards:      context.CombatItemRewards,
		BudgetExhausted:        summary.BudgetExhausted,
		HasMoreDue:             offlineFastForwardClientHasMoreDue(summary),
		YieldReason:            summary.YieldReason,
		StalledQueueID:         summary.StalledQueueID,
		SettledUntilMS:         summary.SettledUntilMS,
		TargetAtMS:             summary.TargetAtMS,
		FishingCatches:         summary.FishingCatches,
		FishingMutations:       context.FishingMutations,
		ErrorCode:              errorCode,
	}
}

func parseOfflineFastForwardRequest(raw json.RawMessage) offlineFastForwardRequest {
	var request offlineFastForwardRequest
	if len(raw) == 0 || json.Unmarshal(raw, &request) != nil {
		return offlineFastForwardRequest{}
	}
	request.ProjectionPolicy = strings.TrimSpace(request.ProjectionPolicy)
	return request
}

func (request offlineFastForwardRequest) defersIntermediateProjection() bool {
	return request.ProtocolVersion == offlineFastForwardProtocolVersion &&
		request.ProjectionPolicy == offlineFastForwardProjectionPolicyTerminal
}

func shouldLoadOfflineFastForwardProjection(request offlineFastForwardRequest, summary store.ActionFastForwardSummary) bool {
	return !request.defersIntermediateProjection() ||
		!offlineFastForwardClientHasMoreDue(summary) ||
		offlineFastForwardRequiresImmediateProjection(summary)
}

func offlineFastForwardRequiresImmediateProjection(summary store.ActionFastForwardSummary) bool {
	if summary.RequiresImmediateProjection || summary.OperationEffectsChanged {
		return true
	}
	if summary.FishingChanged || len(summary.EnhanceResults) > 0 || len(summary.ActivatedWorkLoadouts) > 0 {
		return true
	}
	for _, actor := range summary.ActorSummaries {
		if actor.EquipmentChanged {
			return true
		}
		if actor.ActorType == store.ActionActorTypeKitty && len(actor.AttrExp) > 0 {
			return true
		}
	}
	return false
}

func offlineFastForwardResultStatus(hasMoreDue, syncPending bool, yieldReason string) string {
	if strings.TrimSpace(yieldReason) == store.ActionQueueYieldReasonNoProgress {
		return "stalled"
	}
	if syncPending {
		return "committed_sync_pending"
	}
	if hasMoreDue {
		return "progress"
	}
	return "complete"
}

func offlineFastForwardCanConsumeMarker(summary store.ActionFastForwardSummary, expectedTargetAtMS int64) bool {
	return expectedTargetAtMS > 0 &&
		summary.TargetAtMS == expectedTargetAtMS &&
		!summary.HasMoreDue &&
		strings.TrimSpace(summary.YieldReason) == ""
}

func offlineFastForwardClientHasMoreDue(summary store.ActionFastForwardSummary) bool {
	return summary.HasMoreDue && strings.TrimSpace(summary.YieldReason) == ""
}

func shouldIncludeOfflineFastForwardMutationCatalog(request offlineFastForwardRequest, summary store.ActionFastForwardSummary) bool {
	return !request.defersIntermediateProjection() || !offlineFastForwardClientHasMoreDue(summary)
}
