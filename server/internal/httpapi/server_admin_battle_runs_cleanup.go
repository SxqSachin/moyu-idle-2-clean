package httpapi

import (
	"context"
	"errors"

	"github.com/gofiber/fiber/v2"
)

type adminBattleRunsCleanupRequest struct {
	KeepCount int `json:"keepCount"`
	BatchSize int `json:"batchSize"`
	Rounds    int `json:"rounds"`
}

type adminBattleRunsCleanupCommand struct {
	KeepCount int
	BatchSize int
	Rounds    int
}

var errAdminBattleRunsCleanupUnavailable = errors.New("admin battle runs cleanup unavailable")

var adminBattleRunsCleanupExecutor = func(s *Server, ctx context.Context, cmd adminBattleRunsCleanupCommand) (int64, int, error) {
	if s.deps.Store == nil {
		return 0, 0, errAdminBattleRunsCleanupUnavailable
	}
	keepCount := cmd.KeepCount
	if keepCount <= 0 {
		keepCount = s.deps.Config.GlobalSettings.Battle.RunHistoryKeepCount
	}
	if keepCount <= 0 {
		keepCount = 10
	}
	batchSize := cmd.BatchSize
	if batchSize <= 0 {
		batchSize = 2000
	}
	rounds := cmd.Rounds
	if rounds <= 0 {
		rounds = 1
	}
	var totalDeleted int64
	executed := 0
	for idx := 0; idx < rounds; idx++ {
		deleted, err := s.deps.Store.PurgeBattleRunHistory(ctx, keepCount, batchSize)
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

func (s *Server) handleAdminBattleRunsCleanup(c *fiber.Ctx) error {
	var req adminBattleRunsCleanupRequest
	if err := c.BodyParser(&req); err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "ADMIN_BATTLE_RUNS_CLEANUP_INVALID_PAYLOAD", "invalid request payload", err.Error())
	}
	if req.KeepCount < 0 || req.KeepCount > 200 {
		return s.writeAPIError(c, fiber.StatusBadRequest, "ADMIN_BATTLE_RUNS_CLEANUP_INVALID_KEEP_COUNT", "invalid keepCount", "")
	}
	if req.BatchSize < 0 || req.BatchSize > 20_000 {
		return s.writeAPIError(c, fiber.StatusBadRequest, "ADMIN_BATTLE_RUNS_CLEANUP_INVALID_BATCH_SIZE", "invalid batchSize", "")
	}
	if req.Rounds < 0 || req.Rounds > 500 {
		return s.writeAPIError(c, fiber.StatusBadRequest, "ADMIN_BATTLE_RUNS_CLEANUP_INVALID_ROUNDS", "invalid rounds", "")
	}
	totalDeleted, executed, err := adminBattleRunsCleanupExecutor(s, c.Context(), adminBattleRunsCleanupCommand{
		KeepCount: req.KeepCount,
		BatchSize: req.BatchSize,
		Rounds:    req.Rounds,
	})
	if err != nil {
		if errors.Is(err, errAdminBattleRunsCleanupUnavailable) {
			return s.writeAPIError(c, fiber.StatusServiceUnavailable, "ADMIN_BATTLE_RUNS_CLEANUP_UNAVAILABLE", "store is unavailable", "")
		}
		return s.writeAPIError(c, fiber.StatusInternalServerError, "ADMIN_BATTLE_RUNS_CLEANUP_FAILED", "battle runs cleanup failed", err.Error())
	}
	keepCount := req.KeepCount
	if keepCount <= 0 {
		keepCount = s.deps.Config.GlobalSettings.Battle.RunHistoryKeepCount
		if keepCount <= 0 {
			keepCount = 10
		}
	}
	batchSize := req.BatchSize
	if batchSize <= 0 {
		batchSize = 2000
	}
	rounds := req.Rounds
	if rounds <= 0 {
		rounds = 1
	}
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{
		"ok":        true,
		"keepCount": keepCount,
		"batchSize": batchSize,
		"rounds":    rounds,
		"executed":  executed,
		"deleted":   totalDeleted,
	})
}
