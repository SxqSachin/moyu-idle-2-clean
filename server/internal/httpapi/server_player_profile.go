package httpapi

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"mysecondwebgame/server/internal/store"
)

type playerProfileKittySummary struct {
	ID     int64   `json:"id"`
	Name   string  `json:"name"`
	Level  int     `json:"level"`
	Status string  `json:"status"`
	Mood   float64 `json:"mood"`
}

type playerProfileSkillSummary struct {
	SkillID   string `json:"skillId"`
	Level     int    `json:"level"`
	Enabled   bool   `json:"enabled"`
	CastOrder int    `json:"castOrder"`
}

type playerProfileNameHistoryEntry struct {
	OldName string `json:"oldName"`
}

type playerUsernameProfileSnapshotReader interface {
	GetPlayerUsernameProfileSnapshot(context.Context, int64, int) (store.PlayerUsernameProfileSnapshot, error)
}

const playerProfileNameHistoryLimit = 50

func loadPlayerProfileUsernameSnapshot(
	ctx context.Context,
	reader playerUsernameProfileSnapshotReader,
	userID int64,
) (store.PublicUsernameSnapshot, []playerProfileNameHistoryEntry, bool, error) {
	snapshot, err := reader.GetPlayerUsernameProfileSnapshot(ctx, userID, playerProfileNameHistoryLimit+1)
	if err != nil {
		return store.PublicUsernameSnapshot{}, nil, false, err
	}
	history := snapshot.History
	hasMore := len(history) > playerProfileNameHistoryLimit
	if hasMore {
		history = history[:playerProfileNameHistoryLimit]
	}
	result := make([]playerProfileNameHistoryEntry, 0, len(history))
	for _, entry := range history {
		result = append(result, playerProfileNameHistoryEntry{
			OldName: strings.TrimSpace(entry.OldUsername),
		})
	}
	return snapshot.PublicUsernameSnapshot, result, hasMore, nil
}

func (s *Server) lookupPlayerProfileCache(userID int64, now time.Time) (playerProfileCacheEntry, uint64, bool) {
	s.playerProfileMu.RLock()
	defer s.playerProfileMu.RUnlock()
	epoch := s.playerProfileEpoch[userID]
	entry, ok := s.playerProfileMem[userID]
	return entry, epoch, ok && now.Before(entry.ExpiresAt)
}

func (s *Server) cachePlayerProfileIfCurrent(userID int64, epoch uint64, entry playerProfileCacheEntry) bool {
	s.playerProfileMu.Lock()
	defer s.playerProfileMu.Unlock()
	if s.playerProfileEpoch[userID] != epoch {
		return false
	}
	if s.playerProfileMem == nil {
		s.playerProfileMem = make(map[int64]playerProfileCacheEntry)
	}
	s.playerProfileMem[userID] = entry
	return true
}

func (s *Server) handlePlayerProfileGet(c *fiber.Ctx) error {
	rawUserID := strings.TrimSpace(c.Params("userID"))
	userID, err := strconv.ParseInt(rawUserID, 10, 64)
	if err != nil || userID <= 0 {
		return s.writeAPIError(c, fiber.StatusBadRequest, "PLAYER_PROFILE_INVALID_USER_ID", "invalid user id", "")
	}

	now := time.Now()
	entry, profileEpoch, cacheHit := s.lookupPlayerProfileCache(userID, now)
	if cacheHit {
		return s.writeAPISuccess(c, fiber.StatusOK, entry.Payload)
	}

	usernameSnapshot, nameHistory, nameHistoryHasMore, err := loadPlayerProfileUsernameSnapshot(c.Context(), s.deps.Store, userID)
	if err != nil {
		if errors.Is(err, store.ErrUserNotFound) {
			return s.writeAPIError(c, fiber.StatusNotFound, "PLAYER_PROFILE_NOT_FOUND", "player not found", "")
		}
		return s.writeAPIError(c, fiber.StatusInternalServerError, "PLAYER_PROFILE_NAME_HISTORY_FAILED", "get player name history failed", err.Error())
	}

	prefixItemID, suffixItemID, _ := s.deps.Store.GetPlayerTitle(c.Context(), userID)

	kitties, err := s.deps.Store.ListPlayerKitties(c.Context(), userID)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "PLAYER_PROFILE_KITTIES_FAILED", "get kitties failed", err.Error())
	}
	kittiesSummary := make([]playerProfileKittySummary, 0, len(kitties))
	for _, k := range kitties {
		kittiesSummary = append(kittiesSummary, playerProfileKittySummary{
			ID:     k.ID,
			Name:   k.Name,
			Level:  k.Level,
			Status: k.Status,
			Mood:   k.Mood,
		})
	}

	skills, err := s.deps.Store.ListPlayerSkills(c.Context(), userID)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "PLAYER_PROFILE_SKILLS_FAILED", "get skills failed", err.Error())
	}
	skillsSummary := make([]playerProfileSkillSummary, 0, len(skills))
	for _, sk := range skills {
		skillsSummary = append(skillsSummary, playerProfileSkillSummary{
			SkillID:   sk.SkillID,
			Level:     sk.Level,
			Enabled:   sk.Enabled,
			CastOrder: sk.CastOrder,
		})
	}
	customSkills, err := s.deps.Store.ListPlayerCustomSkills(c.Context(), userID, false)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "PLAYER_PROFILE_CUSTOM_SKILLS_FAILED", "get custom skills failed", err.Error())
	}

	collectibles, err := s.deps.Store.GetCollectibles(c.Context(), userID)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "PLAYER_PROFILE_COLLECTIBLES_FAILED", "get collectibles failed", err.Error())
	}

	achievementsCompleted, achievementsTotal, err := s.deps.Store.CountPlayerAchievementsReadOnly(c.Context(), userID)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "PLAYER_PROFILE_ACHIEVEMENTS_FAILED", "get achievements count failed", err.Error())
	}

	reputation, err := s.deps.Store.GetCatVillageReputationState(c.Context(), userID)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "PLAYER_PROFILE_REPUTATION_FAILED", "get reputation failed", err.Error())
	}

	payload := fiber.Map{
		"userId":                userID,
		"username":              usernameSnapshot.Username,
		"usernameChangeId":      usernameSnapshot.ChangeID,
		"nameHistory":           nameHistory,
		"nameHistoryHasMore":    nameHistoryHasMore,
		"titlePrefixId":         prefixItemID,
		"titleSuffixId":         suffixItemID,
		"kitties":               kittiesSummary,
		"skills":                skillsSummary,
		"customSkills":          customSkills,
		"collectiblesCount":     len(collectibles),
		"achievementsCompleted": achievementsCompleted,
		"achievementsTotal":     achievementsTotal,
		"reputation":            reputation,
	}

	s.cachePlayerProfileIfCurrent(userID, profileEpoch, playerProfileCacheEntry{
		Payload:   payload,
		ExpiresAt: now.Add(playerProfileCacheTTL),
		CachedAt:  now,
	})

	return s.writeAPISuccess(c, fiber.StatusOK, payload)
}

func (s *Server) handlePlayerProfileCollectiblesGet(c *fiber.Ctx) error {
	rawUserID := strings.TrimSpace(c.Params("userID"))
	userID, err := strconv.ParseInt(rawUserID, 10, 64)
	if err != nil || userID <= 0 {
		return s.writeAPIError(c, fiber.StatusBadRequest, "PLAYER_PROFILE_COLLECTIBLES_INVALID_USER_ID", "invalid user id", "")
	}

	now := time.Now()
	s.playerCollectiblesMu.RLock()
	if entry, ok := s.playerCollectiblesMem[userID]; ok && now.Before(entry.ExpiresAt) {
		s.playerCollectiblesMu.RUnlock()
		return s.writeAPISuccess(c, fiber.StatusOK, entry.Payload)
	}
	s.playerCollectiblesMu.RUnlock()

	collectibles, err := s.deps.Store.GetCollectibles(c.Context(), userID)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "PLAYER_PROFILE_COLLECTIBLES_GET_FAILED", "get collectibles failed", err.Error())
	}

	payload := fiber.Map{
		"userId":       userID,
		"collectibles": collectibles,
	}

	s.playerCollectiblesMu.Lock()
	s.playerCollectiblesMem[userID] = playerProfileCacheEntry{
		Payload:   payload,
		ExpiresAt: now.Add(playerProfileCacheTTL),
		CachedAt:  now,
	}
	s.playerCollectiblesMu.Unlock()

	return s.writeAPISuccess(c, fiber.StatusOK, payload)
}

func (s *Server) handlePlayerProfileAchievementsGet(c *fiber.Ctx) error {
	rawUserID := strings.TrimSpace(c.Params("userID"))
	userID, err := strconv.ParseInt(rawUserID, 10, 64)
	if err != nil || userID <= 0 {
		return s.writeAPIError(c, fiber.StatusBadRequest, "PLAYER_PROFILE_ACHIEVEMENTS_INVALID_USER_ID", "invalid user id", "")
	}

	now := time.Now()
	s.playerAchievementsMu.RLock()
	if entry, ok := s.playerAchievementsMem[userID]; ok && now.Before(entry.ExpiresAt) {
		s.playerAchievementsMu.RUnlock()
		return s.writeAPISuccess(c, fiber.StatusOK, entry.Payload)
	}
	s.playerAchievementsMu.RUnlock()

	achievements, err := s.deps.Store.ListPlayerAchievementsReadOnly(c.Context(), userID)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "PLAYER_PROFILE_ACHIEVEMENTS_GET_FAILED", "get achievements failed", err.Error())
	}

	payload := fiber.Map{
		"userId":       userID,
		"achievements": achievements,
	}

	s.playerAchievementsMu.Lock()
	s.playerAchievementsMem[userID] = playerProfileCacheEntry{
		Payload:   payload,
		ExpiresAt: now.Add(playerProfileCacheTTL),
		CachedAt:  now,
	}
	s.playerAchievementsMu.Unlock()

	return s.writeAPISuccess(c, fiber.StatusOK, payload)
}
