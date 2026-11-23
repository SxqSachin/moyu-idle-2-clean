package httpapi

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"mysecondwebgame/server/internal/store"
)

type adminBattleSlowRunsCleanupRequest struct {
	KeepDays  int `json:"keepDays"`
	BatchSize int `json:"batchSize"`
	Rounds    int `json:"rounds"`
}

type adminBattleSlowRunsCleanupCommand struct {
	KeepDays  int
	BatchSize int
	Rounds    int
}

type battleSlowRunAnalysis struct {
	PrimaryCause        string `json:"primaryCause"`
	DominantTurnPhase   string `json:"dominantTurnPhase"`
	DominantTurnPhaseMs int64  `json:"dominantTurnPhaseMs"`
}

var errAdminBattleSlowRunsUnavailable = errors.New("admin battle slow runs unavailable")

var adminBattleSlowRunsListExecutor = func(s *Server, ctx context.Context, filter store.BattleSlowRunArchiveFilter) ([]store.BattleSlowRunArchiveSummary, error) {
	if s.deps.Store == nil {
		return nil, errAdminBattleSlowRunsUnavailable
	}
	return s.deps.Store.ListBattleSlowRunArchives(ctx, filter)
}

var adminBattleSlowRunsGetExecutor = func(s *Server, ctx context.Context, id int64) (store.BattleSlowRunArchive, error) {
	if s.deps.Store == nil {
		return store.BattleSlowRunArchive{}, errAdminBattleSlowRunsUnavailable
	}
	return s.deps.Store.GetBattleSlowRunArchiveByID(ctx, id)
}

var adminBattleSlowRunsCleanupExecutor = func(s *Server, ctx context.Context, cmd adminBattleSlowRunsCleanupCommand) (int64, int, error) {
	if s.deps.Store == nil {
		return 0, 0, errAdminBattleSlowRunsUnavailable
	}
	keepDays := cmd.KeepDays
	if keepDays <= 0 {
		keepDays = battleSlowRunArchiveKeepDaysDefault
	}
	batchSize := cmd.BatchSize
	if batchSize <= 0 {
		batchSize = 1000
	}
	rounds := cmd.Rounds
	if rounds <= 0 {
		rounds = 1
	}
	var totalDeleted int64
	executed := 0
	for idx := 0; idx < rounds; idx++ {
		deleted, err := s.deps.Store.PurgeBattleSlowRunArchives(ctx, keepDays, batchSize)
		if err != nil {
			return totalDeleted, executed, err
		}
		totalDeleted += deleted
		executed++
		if deleted == 0 {
			break
		}
	}
	return totalDeleted, executed, nil
}

func parseAdminBattleSlowRunLimit(raw string) (int, bool) {
	normalized := strings.TrimSpace(raw)
	if normalized == "" {
		return 20, true
	}
	limit, err := strconv.Atoi(normalized)
	if err != nil || limit < 0 {
		return 0, false
	}
	if limit == 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	return limit, true
}

func parseAdminBattleSlowRunQueryID(raw string) (int64, bool) {
	normalized := strings.TrimSpace(raw)
	if normalized == "" {
		return 0, true
	}
	value, err := strconv.ParseInt(normalized, 10, 64)
	if err != nil || value < 0 {
		return 0, false
	}
	return value, true
}

func parseAdminBattleSlowRunPathID(raw string) (int64, bool) {
	value, ok := parseAdminBattleSlowRunQueryID(raw)
	return value, ok && value > 0
}

func battleSlowRunFlag(raw map[string]any, key string) bool {
	if len(raw) == 0 {
		return false
	}
	value, ok := raw[key]
	if !ok {
		return false
	}
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		return strings.EqualFold(strings.TrimSpace(typed), "true") || strings.TrimSpace(typed) == "1"
	case int:
		return typed != 0
	case int64:
		return typed != 0
	case float64:
		return typed != 0
	default:
		return false
	}
}

func battleSlowRunAnalysisFromSummary(triggerFlags, turnProfile map[string]any, guardTimedOut, guardStepLimited bool) battleSlowRunAnalysis {
	queueSlow := battleSlowRunFlag(triggerFlags, "queueSlow")
	compileSlow := battleSlowRunFlag(triggerFlags, "compileSlow")
	guardTriggered := guardTimedOut || guardStepLimited || battleSlowRunFlag(triggerFlags, "guardTimedOut") || battleSlowRunFlag(triggerFlags, "guardStepLimited")
	primaryCause := "compile_execution"
	switch {
	case guardTriggered && (queueSlow || compileSlow):
		primaryCause = "mixed"
	case queueSlow && (compileSlow || guardTriggered):
		primaryCause = "mixed"
	case queueSlow:
		primaryCause = "queue_backlog"
	case guardTriggered:
		primaryCause = "compile_guard"
	case compileSlow:
		primaryCause = "compile_execution"
	}
	preActionMs := battleCheckpointTurnProfileMetric(map[string]any{"turnProfileMs": turnProfile}, "preActionMs")
	executeMs := battleCheckpointTurnProfileMetric(map[string]any{"turnProfileMs": turnProfile}, "executeMs")
	postActionMs := battleCheckpointTurnProfileMetric(map[string]any{"turnProfileMs": turnProfile}, "postActionMs")
	dominantPhase := ""
	dominantMs := int64(0)
	if preActionMs > dominantMs {
		dominantPhase = "preAction"
		dominantMs = preActionMs
	}
	if executeMs > dominantMs {
		dominantPhase = "execute"
		dominantMs = executeMs
	}
	if postActionMs > dominantMs {
		dominantPhase = "postAction"
		dominantMs = postActionMs
	}
	return battleSlowRunAnalysis{
		PrimaryCause:        primaryCause,
		DominantTurnPhase:   dominantPhase,
		DominantTurnPhaseMs: dominantMs,
	}
}

func mapBattleSlowRunArchiveSummary(item store.BattleSlowRunArchiveSummary) fiber.Map {
	return fiber.Map{
		"id":                item.ID,
		"roomId":            item.RoomID,
		"runId":             item.RunID,
		"dungeonId":         item.DungeonID,
		"workerId":          item.WorkerID,
		"source":            item.Source,
		"wave":              item.Wave,
		"outcome":           item.Outcome,
		"queueSinceStartMs": item.QueueSinceStartMs,
		"compileElapsedMs":  item.CompileElapsedMs,
		"totalMs":           item.TotalMs,
		"totalSteps":        item.TotalSteps,
		"totalTurns":        item.TotalTurns,
		"timelineEvents":    item.TimelineEvents,
		"finalSeq":          item.FinalSeq,
		"playerUnits":       item.PlayerUnits,
		"guardTimedOut":     item.GuardTimedOut,
		"guardStepLimited":  item.GuardStepLimited,
		"triggerFlags":      item.TriggerFlags,
		"capturedAt":        item.CapturedAt,
		"runStartedAt":      item.RunStartedAt,
		"analysis":          battleSlowRunAnalysisFromSummary(item.TriggerFlags, item.TurnProfile, item.GuardTimedOut, item.GuardStepLimited),
	}
}

func mapBattleSlowRunArchive(item store.BattleSlowRunArchive) fiber.Map {
	return fiber.Map{
		"id":                item.ID,
		"roomId":            item.RoomID,
		"runId":             item.RunID,
		"dungeonId":         item.DungeonID,
		"workerId":          item.WorkerID,
		"source":            item.Source,
		"wave":              item.Wave,
		"outcome":           item.Outcome,
		"queueSinceStartMs": item.QueueSinceStartMs,
		"compileElapsedMs":  item.CompileElapsedMs,
		"totalMs":           item.TotalMs,
		"totalSteps":        item.TotalSteps,
		"totalTurns":        item.TotalTurns,
		"timelineEvents":    item.TimelineEvents,
		"finalSeq":          item.FinalSeq,
		"playerUnits":       item.PlayerUnits,
		"guardTimedOut":     item.GuardTimedOut,
		"guardStepLimited":  item.GuardStepLimited,
		"triggerFlags":      item.TriggerFlags,
		"turnProfile":       item.TurnProfile,
		"compileInput":      item.CompileInput,
		"checkpoint":        item.Checkpoint,
		"timeline":          item.Timeline,
		"finalResult":       item.FinalResult,
		"capturedAt":        item.CapturedAt,
		"runStartedAt":      item.RunStartedAt,
		"updatedAt":         item.UpdatedAt,
		"analysis":          battleSlowRunAnalysisFromSummary(item.TriggerFlags, item.TurnProfile, item.GuardTimedOut, item.GuardStepLimited),
	}
}

func (s *Server) handleAdminBattleSlowRunsList(c *fiber.Ctx) error {
	roomID, ok := parseAdminBattleSlowRunQueryID(c.Query("roomId"))
	if !ok {
		return s.writeAPIError(c, fiber.StatusBadRequest, "ADMIN_BATTLE_SLOW_RUNS_INVALID_ROOM_ID", "invalid roomId", "")
	}
	runID, ok := parseAdminBattleSlowRunQueryID(c.Query("runId"))
	if !ok {
		return s.writeAPIError(c, fiber.StatusBadRequest, "ADMIN_BATTLE_SLOW_RUNS_INVALID_RUN_ID", "invalid runId", "")
	}
	beforeID, ok := parseAdminBattleSlowRunQueryID(c.Query("beforeId"))
	if !ok {
		return s.writeAPIError(c, fiber.StatusBadRequest, "ADMIN_BATTLE_SLOW_RUNS_INVALID_BEFORE_ID", "invalid beforeId", "")
	}
	limit, ok := parseAdminBattleSlowRunLimit(c.Query("limit"))
	if !ok {
		return s.writeAPIError(c, fiber.StatusBadRequest, "ADMIN_BATTLE_SLOW_RUNS_INVALID_LIMIT", "invalid limit", "")
	}
	items, err := adminBattleSlowRunsListExecutor(s, c.Context(), store.BattleSlowRunArchiveFilter{
		RoomID:   roomID,
		RunID:    runID,
		BeforeID: beforeID,
		Limit:    limit,
	})
	if err != nil {
		if errors.Is(err, errAdminBattleSlowRunsUnavailable) {
			return s.writeAPIError(c, fiber.StatusServiceUnavailable, "ADMIN_BATTLE_SLOW_RUNS_UNAVAILABLE", "store is unavailable", "")
		}
		return s.writeAPIError(c, fiber.StatusInternalServerError, "ADMIN_BATTLE_SLOW_RUNS_LIST_FAILED", "battle slow runs query failed", err.Error())
	}
	respItems := make([]fiber.Map, 0, len(items))
	var nextBeforeID int64
	for idx, item := range items {
		respItems = append(respItems, mapBattleSlowRunArchiveSummary(item))
		if idx == len(items)-1 {
			nextBeforeID = item.ID
		}
	}
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{
		"items":        respItems,
		"nextBeforeId": nextBeforeID,
		"hasMore":      len(items) >= limit,
	})
}

func (s *Server) handleAdminBattleSlowRunGet(c *fiber.Ctx) error {
	archiveID, ok := parseAdminBattleSlowRunPathID(c.Params("archiveID"))
	if !ok {
		return s.writeAPIError(c, fiber.StatusBadRequest, "ADMIN_BATTLE_SLOW_RUN_INVALID_ID", "invalid archive id", "")
	}
	item, err := adminBattleSlowRunsGetExecutor(s, c.Context(), archiveID)
	if err != nil {
		if errors.Is(err, errAdminBattleSlowRunsUnavailable) {
			return s.writeAPIError(c, fiber.StatusServiceUnavailable, "ADMIN_BATTLE_SLOW_RUN_UNAVAILABLE", "store is unavailable", "")
		}
		if errors.Is(err, store.ErrBattleSlowRunArchiveNotFound) {
			return s.writeAPIError(c, fiber.StatusNotFound, "ADMIN_BATTLE_SLOW_RUN_NOT_FOUND", "battle slow run archive not found", "")
		}
		return s.writeAPIError(c, fiber.StatusInternalServerError, "ADMIN_BATTLE_SLOW_RUN_GET_FAILED", "battle slow run archive query failed", err.Error())
	}
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{
		"archive": mapBattleSlowRunArchive(item),
	})
}

func (s *Server) handleAdminBattleSlowRunsCleanup(c *fiber.Ctx) error {
	var req adminBattleSlowRunsCleanupRequest
	if err := c.BodyParser(&req); err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "ADMIN_BATTLE_SLOW_RUNS_CLEANUP_INVALID_PAYLOAD", "invalid request payload", err.Error())
	}
	if req.KeepDays < 0 || req.KeepDays > 3650 {
		return s.writeAPIError(c, fiber.StatusBadRequest, "ADMIN_BATTLE_SLOW_RUNS_CLEANUP_INVALID_KEEP_DAYS", "invalid keepDays", "")
	}
	if req.BatchSize < 0 || req.BatchSize > 20_000 {
		return s.writeAPIError(c, fiber.StatusBadRequest, "ADMIN_BATTLE_SLOW_RUNS_CLEANUP_INVALID_BATCH_SIZE", "invalid batchSize", "")
	}
	if req.Rounds < 0 || req.Rounds > 500 {
		return s.writeAPIError(c, fiber.StatusBadRequest, "ADMIN_BATTLE_SLOW_RUNS_CLEANUP_INVALID_ROUNDS", "invalid rounds", "")
	}
	totalDeleted, executed, err := adminBattleSlowRunsCleanupExecutor(s, c.Context(), adminBattleSlowRunsCleanupCommand{
		KeepDays:  req.KeepDays,
		BatchSize: req.BatchSize,
		Rounds:    req.Rounds,
	})
	if err != nil {
		if errors.Is(err, errAdminBattleSlowRunsUnavailable) {
			return s.writeAPIError(c, fiber.StatusServiceUnavailable, "ADMIN_BATTLE_SLOW_RUNS_CLEANUP_UNAVAILABLE", "store is unavailable", "")
		}
		return s.writeAPIError(c, fiber.StatusInternalServerError, "ADMIN_BATTLE_SLOW_RUNS_CLEANUP_FAILED", "battle slow runs cleanup failed", err.Error())
	}
	keepDays := req.KeepDays
	if keepDays <= 0 {
		keepDays = battleSlowRunArchiveKeepDaysDefault
	}
	batchSize := req.BatchSize
	if batchSize <= 0 {
		batchSize = 1000
	}
	rounds := req.Rounds
	if rounds <= 0 {
		rounds = 1
	}
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{
		"ok":        true,
		"keepDays":  keepDays,
		"batchSize": batchSize,
		"rounds":    rounds,
		"executed":  executed,
		"deleted":   totalDeleted,
	})
}
