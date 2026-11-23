package httpapi

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"mysecondwebgame/server/internal/store"
)

func (s *Server) lookupPlayerPublicCache(userID int64, now time.Time) (playerProfileCacheEntry, uint64, bool) {
	s.playerPublicMu.RLock()
	defer s.playerPublicMu.RUnlock()
	epoch := s.playerPublicEpoch[userID]
	entry, ok := s.playerPublicMem[userID]
	return entry, epoch, ok && now.Before(entry.ExpiresAt)
}

func (s *Server) cachePlayerPublicIfCurrent(userID int64, epoch uint64, entry playerProfileCacheEntry) bool {
	s.playerPublicMu.Lock()
	defer s.playerPublicMu.Unlock()
	if s.playerPublicEpoch[userID] != epoch {
		return false
	}
	if s.playerPublicMem == nil {
		s.playerPublicMem = make(map[int64]playerProfileCacheEntry)
	}
	s.playerPublicMem[userID] = entry
	return true
}

func (s *Server) handlePlayerPublicProfileGet(c *fiber.Ctx) error {
	rawUserID := strings.TrimSpace(c.Params("userID"))
	userID, err := strconv.ParseInt(rawUserID, 10, 64)
	if err != nil || userID <= 0 {
		return s.writeAPIError(c, fiber.StatusBadRequest, "PLAYER_PUBLIC_INVALID_USER_ID", "invalid user id", "")
	}
	now := time.Now()
	entry, publicEpoch, cacheHit := s.lookupPlayerPublicCache(userID, now)
	if cacheHit {
		return s.writeAPISuccess(c, fiber.StatusOK, entry.Payload)
	}

	seeds, err := s.playerAttributeSeeds()
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "PLAYER_PUBLIC_ATTRIBUTE_SEEDS_INVALID", "player attribute seeds invalid", err.Error())
	}
	if err := s.ensurePlayerStatsSnapshot(c.Context(), userID, seeds); err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "PLAYER_PUBLIC_STATS_PREPARE_FAILED", "prepare player stats failed", err.Error())
	}
	usernameSnapshot, err := s.deps.Store.GetPublicUsernameSnapshot(c.Context(), userID)
	if err != nil {
		if errors.Is(err, store.ErrUserNotFound) {
			return s.writeAPIError(c, fiber.StatusNotFound, "PLAYER_PUBLIC_NOT_FOUND", "player not found", "")
		}
		return s.writeAPIError(c, fiber.StatusInternalServerError, "PLAYER_PUBLIC_GET_FAILED", "get player profile failed", err.Error())
	}
	stats, err := s.deps.Store.GetPlayerStats(c.Context(), userID)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "PLAYER_PUBLIC_STATS_GET_FAILED", "get stats failed", err.Error())
	}
	attributeStates, err := s.deps.Store.ListPlayerAttributes(c.Context(), userID, seeds)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "PLAYER_PUBLIC_ATTRIBUTES_GET_FAILED", "get attributes failed", err.Error())
	}
	progress, err := s.attributeProgressMap(attributeStates)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "PLAYER_PUBLIC_ATTRIBUTE_PROGRESS_FAILED", "build attribute progress failed", err.Error())
	}
	equipped, err := s.deps.Store.ListEquippedItems(c.Context(), userID)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "PLAYER_PUBLIC_EQUIPPED_LIST_FAILED", "get equipped items failed", err.Error())
	}
	attributes := s.attributeMapFromStates(attributeStates)
	basicAttack, err := s.playerBasicAttackPreview(c.Context(), userID, stats.Derived, attributeStates)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "PLAYER_PUBLIC_BASIC_ATTACK_PREVIEW_FAILED", "build basic attack preview failed", err.Error())
	}
	customSkills, err := s.deps.Store.ListPlayerCustomSkills(c.Context(), userID, false)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "PLAYER_PUBLIC_CUSTOM_SKILLS_FAILED", "get custom skills failed", err.Error())
	}
	payload := fiber.Map{
		"userId":            userID,
		"username":          usernameSnapshot.Username,
		"usernameChangeId":  usernameSnapshot.ChangeID,
		"derived":           stats.Derived,
		"cacheVersion":      stats.CacheVer,
		"updatedAt":         stats.UpdatedAt,
		"attributeProgress": progress,
		"attributes":        attributes,
		"equipped":          equipped,
		"basicAttack":       basicAttack,
		"customSkills":      customSkills,
		"formula":           s.playerFormulaSnapshot(),
	}
	s.cachePlayerPublicIfCurrent(userID, publicEpoch, playerProfileCacheEntry{Payload: payload, ExpiresAt: now.Add(playerProfileCacheTTL), CachedAt: now})
	return s.writeAPISuccess(c, fiber.StatusOK, payload)
}
