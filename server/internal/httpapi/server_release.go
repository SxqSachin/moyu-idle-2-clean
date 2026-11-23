package httpapi

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

const (
	releaseStateRedisKey       = "system:release:state"
	defaultReleaseReloadDelayS = 120
	maxReleaseReloadDelayS     = 900
)

type releaseAnnouncement struct {
	ReleaseID         string `json:"releaseId"`
	Kind              string `json:"kind"`
	MessageI18nKey    string `json:"messageI18nKey,omitempty"`
	RecommendedDelayS int    `json:"recommendedDelaySec"`
	ForceAtMS         int64  `json:"forceAtMs,omitempty"`
	MinAcceptedBuild  string `json:"minAcceptedBuild,omitempty"`
	AnnouncedAtMS     int64  `json:"announcedAtMs"`
}

type releaseAnnounceRequest struct {
	ReleaseID         string `json:"releaseId"`
	Kind              string `json:"kind"`
	MessageI18nKey    string `json:"messageI18nKey"`
	RecommendedDelayS int    `json:"recommendedDelaySec"`
	ForceAtMS         int64  `json:"forceAtMs"`
	MinAcceptedBuild  string `json:"minAcceptedBuild"`
}

func normalizeReleaseKind(raw string) string {
	kind := strings.TrimSpace(strings.ToLower(raw))
	if kind == "force" {
		return "force"
	}
	return "soft"
}

func clampReleaseDelay(seconds int, fallback int) int {
	if fallback <= 0 {
		fallback = defaultReleaseReloadDelayS
	}
	if seconds <= 0 {
		seconds = fallback
	}
	if seconds > maxReleaseReloadDelayS {
		seconds = maxReleaseReloadDelayS
	}
	return seconds
}

func (s *Server) releaseAdminMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		expected := strings.TrimSpace(s.deps.Config.ReleaseAnnounceToken)
		if expected == "" {
			return s.writeAPIError(c, fiber.StatusServiceUnavailable, "RELEASE_ANNOUNCE_DISABLED", "release announce token is not configured", "")
		}
		provided := strings.TrimSpace(c.Get("X-Release-Token"))
		if provided == "" || provided != expected {
			return s.writeAPIError(c, fiber.StatusUnauthorized, "RELEASE_ANNOUNCE_UNAUTHORIZED", "invalid release announce token", "")
		}
		return c.Next()
	}
}

func (s *Server) defaultReleaseAnnouncement() releaseAnnouncement {
	nowMS := time.Now().UnixMilli()
	releaseID := strings.TrimSpace(s.deps.Config.ReleaseBuildID)
	if releaseID == "" {
		releaseID = "runtime-" + time.Now().UTC().Format("20060102-150405")
	}
	delay := clampReleaseDelay(0, s.deps.Config.ReleaseReloadDelaySec)
	return releaseAnnouncement{
		ReleaseID:         releaseID,
		Kind:              "soft",
		MessageI18nKey:    "game.clientUpdateAvailable",
		RecommendedDelayS: delay,
		AnnouncedAtMS:     nowMS,
	}
}

func (s *Server) saveReleaseAnnouncement(ctx context.Context, announcement releaseAnnouncement) {
	if s.deps.Redis == nil {
		return
	}
	raw, err := json.Marshal(announcement)
	if err != nil {
		return
	}
	_ = s.deps.Redis.Set(ctx, releaseStateRedisKey, string(raw), 0).Err()
}

func (s *Server) loadReleaseAnnouncement(ctx context.Context) releaseAnnouncement {
	defaultState := s.defaultReleaseAnnouncement()
	if s.deps.Redis == nil {
		return defaultState
	}
	raw, err := s.deps.Redis.Get(ctx, releaseStateRedisKey).Result()
	if err != nil || strings.TrimSpace(raw) == "" {
		return defaultState
	}
	var state releaseAnnouncement
	if err := json.Unmarshal([]byte(raw), &state); err != nil {
		return defaultState
	}
	state.ReleaseID = strings.TrimSpace(state.ReleaseID)
	if state.ReleaseID == "" {
		return defaultState
	}
	state.Kind = normalizeReleaseKind(state.Kind)
	state.MessageI18nKey = strings.TrimSpace(state.MessageI18nKey)
	if state.MessageI18nKey == "" {
		state.MessageI18nKey = "game.clientUpdateAvailable"
	}
	state.RecommendedDelayS = clampReleaseDelay(state.RecommendedDelayS, s.deps.Config.ReleaseReloadDelaySec)
	if state.AnnouncedAtMS <= 0 {
		state.AnnouncedAtMS = defaultState.AnnouncedAtMS
	}
	if state.Kind != "force" {
		state.ForceAtMS = 0
	}
	return state
}

func (s *Server) handleSystemVersionGet(c *fiber.Ctx) error {
	announcement := s.loadReleaseAnnouncement(c.Context())
	return s.writeAPISuccess(c, fiber.StatusOK, announcement)
}

func (s *Server) handleReleaseAnnounce(c *fiber.Ctx) error {
	var req releaseAnnounceRequest
	if err := c.BodyParser(&req); err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "RELEASE_ANNOUNCE_INVALID_PAYLOAD", "invalid request payload", err.Error())
	}
	releaseID := strings.TrimSpace(req.ReleaseID)
	if releaseID == "" {
		return s.writeAPIError(c, fiber.StatusBadRequest, "RELEASE_ANNOUNCE_RELEASE_ID_REQUIRED", "releaseId is required", "")
	}
	kind := normalizeReleaseKind(req.Kind)
	recommendedDelay := clampReleaseDelay(req.RecommendedDelayS, s.deps.Config.ReleaseReloadDelaySec)
	forceAtMS := req.ForceAtMS
	if kind == "force" && forceAtMS <= 0 {
		forceAtMS = time.Now().Add(time.Duration(recommendedDelay) * time.Second).UnixMilli()
	}
	if kind != "force" {
		forceAtMS = 0
	}
	announcement := releaseAnnouncement{
		ReleaseID:         releaseID,
		Kind:              kind,
		MessageI18nKey:    strings.TrimSpace(req.MessageI18nKey),
		RecommendedDelayS: recommendedDelay,
		ForceAtMS:         forceAtMS,
		MinAcceptedBuild:  strings.TrimSpace(req.MinAcceptedBuild),
		AnnouncedAtMS:     time.Now().UnixMilli(),
	}
	if announcement.MessageI18nKey == "" {
		announcement.MessageI18nKey = "game.clientUpdateAvailable"
	}
	s.saveReleaseAnnouncement(c.Context(), announcement)
	s.broadcastWSCluster("system.client.update", announcement, c.Context())
	return s.writeAPISuccess(c, fiber.StatusOK, announcement)
}
