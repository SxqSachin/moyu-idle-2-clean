package httpapi

import (
	"context"
	"errors"
	"log"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"mysecondwebgame/server/internal/store"
)

type adminBattleRoomsReinitializeRequest struct {
	Statuses               []string                      `json:"statuses"`
	ExpectedSpecialization *specializationRuntimeVersion `json:"expectedSpecialization,omitempty"`
}

type adminBattleRoomsReinitializeCommand struct {
	Statuses []string
}

var errAdminBattleRoomsReinitializeUnavailable = errors.New("admin battle rooms reinitialize unavailable")

var adminBattleRoomsReinitializeExecutor = func(s *Server, ctx context.Context, cmd adminBattleRoomsReinitializeCommand) (int64, int64, error) {
	if s.deps.Store == nil {
		return 0, 0, errAdminBattleRoomsReinitializeUnavailable
	}
	recoveryDuration := time.Duration(s.deps.Config.GlobalSettings.Battle.RecoveryDurationMs) * time.Millisecond
	return s.deps.Store.AdminReinitializeBattleRooms(ctx, cmd.Statuses, time.Now(), recoveryDuration)
}

func normalizeAdminBattleRoomStatuses(raw []string) []string {
	if len(raw) == 0 {
		return nil
	}
	allowed := map[string]struct{}{
		store.BattleRoomStatusEntering:   {},
		store.BattleRoomStatusPlaying:    {},
		store.BattleRoomStatusRecovering: {},
	}
	out := make([]string, 0, len(raw))
	seen := map[string]struct{}{}
	for _, status := range raw {
		normalized := strings.TrimSpace(strings.ToLower(status))
		if normalized == "" {
			continue
		}
		if _, ok := allowed[normalized]; !ok {
			return []string{}
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}
	return out
}

func (s *Server) handleAdminBattleRoomsReinitialize(c *fiber.Ctx) error {
	var req adminBattleRoomsReinitializeRequest
	if err := c.BodyParser(&req); err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "ADMIN_BATTLE_REINIT_INVALID_PAYLOAD", "invalid request payload", err.Error())
	}
	statuses := normalizeAdminBattleRoomStatuses(req.Statuses)
	if len(req.Statuses) > 0 && len(statuses) == 0 {
		return s.writeAPIError(c, fiber.StatusBadRequest, "ADMIN_BATTLE_REINIT_INVALID_STATUSES", "invalid statuses", "")
	}
	loadedSpecialization, specializationAvailable := s.specializationRuntimeVersion()
	workerSnapshot := battleWorkerSpecializationSnapshot{Workers: []battleWorkerSpecializationHeartbeat{}}
	if expected := req.ExpectedSpecialization; expected != nil {
		expected.TreeID = strings.TrimSpace(expected.TreeID)
		expected.RulesetVersion = strings.TrimSpace(expected.RulesetVersion)
		if expected.TreeID == "" || expected.RulesetVersion == "" || expected.BalanceRevision <= 0 {
			return s.writeAPIError(c, fiber.StatusBadRequest, "ADMIN_BATTLE_REINIT_INVALID_SPECIALIZATION", "invalid expected specialization version", "")
		}
		if !specializationAvailable {
			return s.writeAPIError(c, fiber.StatusServiceUnavailable, "ADMIN_BATTLE_REINIT_SPECIALIZATION_UNAVAILABLE", "loaded specialization version is unavailable", "")
		}
		if *expected != loadedSpecialization {
			return s.writeAPIError(c, fiber.StatusConflict, "ADMIN_BATTLE_REINIT_SPECIALIZATION_CONFLICT", "loaded specialization version changed; refresh metrics before retrying", "")
		}
		var err error
		workerSnapshot, err = s.battleWorkerSpecializationSnapshot(c.Context(), loadedSpecialization)
		if err != nil || workerSnapshot.WorkerCount == 0 {
			return s.writeAPIError(c, fiber.StatusServiceUnavailable, "ADMIN_BATTLE_REINIT_WORKERS_UNAVAILABLE", "no versioned battle worker heartbeat is available", "")
		}
		if !workerSnapshot.Ready {
			return s.writeAPIError(c, fiber.StatusConflict, "ADMIN_BATTLE_REINIT_WORKERS_CONFLICT", "battle worker specialization versions do not match the server", "")
		}
	}
	appliedStatuses := statuses
	if len(appliedStatuses) == 0 {
		appliedStatuses = []string{
			store.BattleRoomStatusEntering,
			store.BattleRoomStatusPlaying,
			store.BattleRoomStatusRecovering,
		}
	}
	rooms, runs, err := adminBattleRoomsReinitializeExecutor(s, c.Context(), adminBattleRoomsReinitializeCommand{
		Statuses: statuses,
	})
	if err != nil {
		if errors.Is(err, errAdminBattleRoomsReinitializeUnavailable) {
			return s.writeAPIError(c, fiber.StatusServiceUnavailable, "ADMIN_BATTLE_REINIT_UNAVAILABLE", "store is unavailable", "")
		}
		return s.writeAPIError(c, fiber.StatusInternalServerError, "ADMIN_BATTLE_REINIT_FAILED", "battle rooms reinitialize failed", err.Error())
	}
	log.Printf(
		"admin battle rooms reinitialized rooms=%d interrupted_runs=%d statuses=%v specialization_tree=%s ruleset=%s balance_revision=%d matching_workers=%d",
		rooms,
		runs,
		appliedStatuses,
		loadedSpecialization.TreeID,
		loadedSpecialization.RulesetVersion,
		loadedSpecialization.BalanceRevision,
		workerSnapshot.MatchingWorkers,
	)
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{
		"ok":              true,
		"reinitRooms":     rooms,
		"interruptedRuns": runs,
		"runStatus":       store.BattleRunStatusInterrupted,
		"specialization": func() any {
			if specializationAvailable {
				return loadedSpecialization
			}
			return nil
		}(),
		"battleWorkers": workerSnapshot,
		"statuses":      appliedStatuses,
	})
}
