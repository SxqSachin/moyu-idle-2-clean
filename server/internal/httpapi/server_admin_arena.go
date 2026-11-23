package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/gofiber/fiber/v2"
	"mysecondwebgame/server/internal/store"
)

const (
	adminArenaDefenseInitializationWindow    = 14 * 24 * time.Hour
	adminArenaDefenseInitializationBatchSize = 50
)

type adminArenaDefenseInitializeResult struct {
	ActiveSince      time.Time `json:"activeSince"`
	ActiveWithinDays int       `json:"activeWithinDays"`
	BatchSize        int       `json:"batchSize"`
	Batches          int       `json:"batches"`
	Scanned          int       `json:"scanned"`
	Initialized      int       `json:"initialized"`
	SkippedExisting  int       `json:"skippedExisting"`
}

type adminArenaDefenseCandidatePageFunc func(context.Context, time.Time, int64, int) ([]int64, bool, error)

type adminArenaDefenseInitializeUserFunc func(context.Context, int64, time.Time) (bool, error)

type arenaDefaultKittyCandidate struct {
	KittyID        int64
	PositionLocked string
}

var errAdminArenaDefenseInitializeUnavailable = errors.New("admin arena defense initialize unavailable")

var errAdminArenaLeaderboardResetUnavailable = errors.New("admin arena leaderboard reset unavailable")

var adminArenaDefenseInitializeExecutor = func(s *Server, ctx context.Context) (adminArenaDefenseInitializeResult, error) {
	if s.deps.Store == nil {
		return adminArenaDefenseInitializeResult{}, errAdminArenaDefenseInitializeUnavailable
	}
	return runAdminArenaDefenseInitialization(
		ctx,
		time.Now().UTC(),
		s.deps.Store.ListArenaDefenseInitializationCandidates,
		s.initializeDefaultArenaDefense,
	)
}

var adminArenaLeaderboardResetExecutor = func(s *Server, ctx context.Context, defaultRating int64) (store.ArenaLeaderboardResetResult, error) {
	if s.deps.Store == nil {
		return store.ArenaLeaderboardResetResult{}, errAdminArenaLeaderboardResetUnavailable
	}
	return s.deps.Store.AdminResetArenaLeaderboard(ctx, defaultRating)
}

func runAdminArenaDefenseInitialization(
	ctx context.Context,
	now time.Time,
	listCandidates adminArenaDefenseCandidatePageFunc,
	initializeUser adminArenaDefenseInitializeUserFunc,
) (adminArenaDefenseInitializeResult, error) {
	activeSince := now.Add(-adminArenaDefenseInitializationWindow)
	result := adminArenaDefenseInitializeResult{
		ActiveSince:      activeSince,
		ActiveWithinDays: int(adminArenaDefenseInitializationWindow / (24 * time.Hour)),
		BatchSize:        adminArenaDefenseInitializationBatchSize,
	}
	afterUserID := int64(0)
	for {
		userIDs, hasMore, err := listCandidates(ctx, activeSince, afterUserID, adminArenaDefenseInitializationBatchSize)
		if err != nil {
			return result, fmt.Errorf("list arena initialization candidates after user_id=%d: %w", afterUserID, err)
		}
		if len(userIDs) == 0 {
			if hasMore {
				return result, errors.New("arena initialization candidate page is empty while more candidates are reported")
			}
			break
		}
		result.Batches++
		for _, userID := range userIDs {
			result.Scanned++
			created, err := initializeUser(ctx, userID, now)
			if err != nil {
				return result, fmt.Errorf("initialize arena defense for user_id=%d: %w", userID, err)
			}
			if created {
				result.Initialized++
			} else {
				result.SkippedExisting++
			}
		}
		if !hasMore {
			break
		}
		nextAfterUserID := userIDs[len(userIDs)-1]
		if nextAfterUserID <= afterUserID {
			return result, fmt.Errorf("arena initialization candidate cursor did not advance: current=%d next=%d", afterUserID, nextAfterUserID)
		}
		afterUserID = nextAfterUserID
	}
	return result, nil
}

func (s *Server) handleAdminArenaDefensesInitialize(c *fiber.Ctx) error {
	result, err := adminArenaDefenseInitializeExecutor(s, c.Context())
	if err != nil {
		if errors.Is(err, errAdminArenaDefenseInitializeUnavailable) {
			return s.writeAPIError(c, fiber.StatusServiceUnavailable, "ADMIN_ARENA_INITIALIZE_UNAVAILABLE", "arena initialization is unavailable", "")
		}
		log.Printf(
			"admin arena defenses initialization failed batches=%d scanned=%d initialized=%d skipped_existing=%d error=%v",
			result.Batches,
			result.Scanned,
			result.Initialized,
			result.SkippedExisting,
			err,
		)
		return s.writeAPIError(c, fiber.StatusInternalServerError, "ADMIN_ARENA_INITIALIZE_FAILED", "initialize arena defenses failed", err.Error())
	}
	log.Printf(
		"admin arena defenses initialized active_since=%s batches=%d scanned=%d initialized=%d skipped_existing=%d",
		result.ActiveSince.Format(time.RFC3339),
		result.Batches,
		result.Scanned,
		result.Initialized,
		result.SkippedExisting,
	)
	return s.writeAPISuccess(c, fiber.StatusOK, result)
}

func (s *Server) handleAdminArenaLeaderboardReset(c *fiber.Ctx) error {
	defaultRating := s.deps.Config.GlobalSettings.Arena.DefaultRating
	result, err := adminArenaLeaderboardResetExecutor(s, c.Context(), defaultRating)
	if err != nil {
		if errors.Is(err, errAdminArenaLeaderboardResetUnavailable) {
			return s.writeAPIError(c, fiber.StatusServiceUnavailable, "ADMIN_ARENA_LEADERBOARD_RESET_UNAVAILABLE", "arena leaderboard reset is unavailable", "")
		}
		log.Printf("admin arena leaderboard reset failed default_rating=%d error=%v", defaultRating, err)
		return s.writeAPIError(c, fiber.StatusInternalServerError, "ADMIN_ARENA_LEADERBOARD_RESET_FAILED", "reset arena leaderboard failed", err.Error())
	}
	log.Printf(
		"admin arena leaderboard reset reset_players=%d default_rating=%d reset_at=%s",
		result.ResetPlayers,
		result.DefaultRating,
		result.ResetAt.Format(time.RFC3339),
	)
	return s.writeAPISuccess(c, fiber.StatusOK, result)
}

func (s *Server) initializeDefaultArenaDefense(ctx context.Context, userID int64, now time.Time) (bool, error) {
	if _, err := s.deps.Store.GetArenaDefense(ctx, userID); err == nil {
		return false, nil
	} else if !errors.Is(err, store.ErrArenaDefenseNotFound) {
		return false, fmt.Errorf("check existing arena defense: %w", err)
	}
	kitties, err := s.deps.Store.ListPlayerKitties(ctx, userID)
	if err != nil {
		return false, fmt.Errorf("list player kitties: %w", err)
	}
	candidates := make([]arenaDefaultKittyCandidate, 0, len(kitties))
	for _, kitty := range kitties {
		lockedPosition, _, _ := s.deps.Store.ResolveKittyPositionLock(kitty.Extra)
		candidates = append(candidates, arenaDefaultKittyCandidate{
			KittyID:        kitty.ID,
			PositionLocked: lockedPosition,
		})
	}
	selection := defaultArenaLineupSelection(candidates)
	format, err := s.resolveArenaFormatForPublish(arenaFormatStandardID)
	if err != nil {
		return false, fmt.Errorf("resolve standard arena format: %w", err)
	}
	selection.FormatID = format.ID
	selection.FormatSnapshot = format
	selection, lineup, err := s.buildArenaLineupSnapshot(ctx, userID, selection, now, format)
	if err != nil {
		return false, fmt.Errorf("build arena lineup snapshot: %w", err)
	}
	digest, lineupRaw, err := arenaLineupDigest(lineup)
	if err != nil {
		return false, fmt.Errorf("encode arena lineup snapshot: %w", err)
	}
	selectionRaw, err := json.Marshal(selection)
	if err != nil {
		return false, fmt.Errorf("encode arena lineup selection: %w", err)
	}
	kittyIDs := make([]int64, 0, len(selection.Kitties))
	for _, kitty := range selection.Kitties {
		kittyIDs = append(kittyIDs, kitty.KittyID)
	}
	if _, err := s.deps.Store.EnsureArenaPlayerState(ctx, userID, s.deps.Config.GlobalSettings.Arena.DefaultRating, s.deps.Config.GlobalSettings.Arena.ChallengeChargeCapacity); err != nil {
		return false, fmt.Errorf("ensure arena player state: %w", err)
	}
	_, created, err := s.deps.Store.CreateArenaDefenseIfAbsent(ctx, store.PublishArenaDefenseInput{
		UserID:       userID,
		BattleLevel:  lineup[0].BattleLevel,
		KittyIDs:     kittyIDs,
		Selection:    selectionRaw,
		Lineup:       lineupRaw,
		LineupDigest: digest,
	})
	if err != nil {
		return false, fmt.Errorf("persist initialized arena defense: %w", err)
	}
	return created, nil
}

func defaultArenaLineupSelection(candidates []arenaDefaultKittyCandidate) arenaLineupSelection {
	selected := selectDefaultArenaKitties(candidates)
	selection := arenaLineupSelection{
		PlayerPosition: "vanguard",
		Kitties:        make([]arenaLineupKittySelection, len(selected)),
		FormatID:       arenaFormatStandardID,
	}
	defaultKittyPositions := []string{"midguard", "rearguard"}
	for index, candidate := range selected {
		position := defaultKittyPositions[index]
		if locked := normalizeArenaPosition(candidate.PositionLocked); locked != "" {
			position = locked
		}
		selection.Kitties[index] = arenaLineupKittySelection{KittyID: candidate.KittyID, Position: position}
	}
	return selection
}

func selectDefaultArenaKitties(candidates []arenaDefaultKittyCandidate) []arenaDefaultKittyCandidate {
	valid := make([]arenaDefaultKittyCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.KittyID > 0 {
			valid = append(valid, candidate)
		}
	}
	if len(valid) > store.ArenaMaxKittyCount {
		valid = valid[:store.ArenaMaxKittyCount]
	}
	return append([]arenaDefaultKittyCandidate(nil), valid...)
}
