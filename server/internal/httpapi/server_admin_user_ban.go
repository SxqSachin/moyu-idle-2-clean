package httpapi

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"mysecondwebgame/server/internal/store"
)

type adminUserBanRequest struct {
	UserID      int64  `json:"userId"`
	Username    string `json:"username"`
	Reason      string `json:"reason"`
	BanUntil    string `json:"banUntil"`
	IsPermanent bool   `json:"isPermanent"`
}

type adminUserUnbanRequest struct {
	UserID   int64  `json:"userId"`
	Username string `json:"username"`
}

type adminUserBanCommand struct {
	UserID      int64
	Reason      string
	BanUntil    *time.Time
	IsPermanent bool
}

var errAdminUserBanUnavailable = errors.New("admin user ban unavailable")

var adminUserBanExecutor = func(s *Server, ctx context.Context, cmd adminUserBanCommand) error {
	if s.deps.Store == nil {
		return errAdminUserBanUnavailable
	}
	return s.deps.Store.AdminBanUser(ctx, store.AdminUserBanInput{
		UserID:      cmd.UserID,
		Reason:      cmd.Reason,
		BanUntil:    cmd.BanUntil,
		IsPermanent: cmd.IsPermanent,
	})
}

var adminUserUnbanExecutor = func(s *Server, ctx context.Context, userID int64) error {
	if s.deps.Store == nil {
		return errAdminUserBanUnavailable
	}
	return s.deps.Store.AdminUnbanUser(ctx, userID)
}

var adminUserKickExecutor = func(s *Server, userID int64) {
	s.kickUserSockets(userID, "ACCOUNT_BANNED")
}

func configuredAdminBanReasons(s *Server) []struct {
	Key     string
	NameKey string
} {
	reasons := make([]struct {
		Key     string
		NameKey string
	}, 0, len(s.deps.Config.GlobalSettings.Admin.UserBanReasons))
	for _, reason := range s.deps.Config.GlobalSettings.Admin.UserBanReasons {
		key := strings.TrimSpace(strings.ToLower(reason.Key))
		nameKey := strings.TrimSpace(reason.NameKey)
		if key == "" || nameKey == "" {
			continue
		}
		reasons = append(reasons, struct {
			Key     string
			NameKey string
		}{Key: key, NameKey: nameKey})
	}
	return reasons
}

func configuredAdminBanReasonSet(s *Server) map[string]struct{} {
	reasons := configuredAdminBanReasons(s)
	out := make(map[string]struct{}, len(reasons))
	for _, reason := range reasons {
		out[reason.Key] = struct{}{}
	}
	return out
}

func normalizeAdminBanReason(raw string, allowed map[string]struct{}) string {
	reason := strings.TrimSpace(strings.ToLower(raw))
	if reason == "" {
		return ""
	}
	if _, ok := allowed[reason]; !ok {
		return ""
	}
	return reason
}

func resolveAdminTargetUserID(ctx context.Context, st *store.Store, userID int64, username string) (int64, error) {
	username = strings.TrimSpace(username)
	if userID <= 0 && username == "" {
		return 0, store.ErrUserNotFound
	}
	if username == "" {
		return userID, nil
	}
	if st == nil {
		return 0, errAdminUserBanUnavailable
	}
	resolvedByName, err := st.GetUserIDByUsername(ctx, username)
	if err != nil {
		return 0, err
	}
	if userID > 0 && resolvedByName != userID {
		return 0, store.ErrUserNotFound
	}
	return resolvedByName, nil
}

var adminUserResolveTargetExecutor = resolveAdminTargetUserID

func parseAdminBanUntil(raw string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(raw))
	if err != nil {
		return time.Time{}, err
	}
	return parsed.UTC(), nil
}

func (s *Server) handleAdminUserBan(c *fiber.Ctx) error {
	var req adminUserBanRequest
	if err := c.BodyParser(&req); err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "ADMIN_USER_BAN_INVALID_PAYLOAD", "invalid request payload", err.Error())
	}
	targetUserID, err := adminUserResolveTargetExecutor(c.Context(), s.deps.Store, req.UserID, req.Username)
	if err != nil {
		switch {
		case errors.Is(err, errAdminUserBanUnavailable):
			return s.writeAPIError(c, fiber.StatusServiceUnavailable, "ADMIN_USER_BAN_UNAVAILABLE", "store is unavailable", "")
		case errors.Is(err, store.ErrUserNotFound):
			return s.writeAPIError(c, fiber.StatusNotFound, "ADMIN_USER_BAN_TARGET_NOT_FOUND", "target user not found", "")
		default:
			return s.writeAPIError(c, fiber.StatusInternalServerError, "ADMIN_USER_BAN_FAILED", "admin user ban failed", err.Error())
		}
	}
	allowedReasons := configuredAdminBanReasonSet(s)
	reason := normalizeAdminBanReason(req.Reason, allowedReasons)
	if reason == "" {
		return s.writeAPIError(c, fiber.StatusBadRequest, "ADMIN_USER_BAN_INVALID_REASON", "invalid ban reason", "")
	}
	var banUntil *time.Time
	if !req.IsPermanent {
		parsedUntil, parseErr := parseAdminBanUntil(req.BanUntil)
		if parseErr != nil {
			return s.writeAPIError(c, fiber.StatusBadRequest, "ADMIN_USER_BAN_INVALID_BAN_UNTIL", "invalid banUntil, RFC3339 required", "")
		}
		if !parsedUntil.After(time.Now().UTC()) {
			return s.writeAPIError(c, fiber.StatusBadRequest, "ADMIN_USER_BAN_INVALID_BAN_UNTIL", "banUntil must be in the future", "")
		}
		banUntil = &parsedUntil
	}
	adminUserKickExecutor(s, targetUserID)
	if err := adminUserBanExecutor(s, c.Context(), adminUserBanCommand{
		UserID:      targetUserID,
		Reason:      reason,
		BanUntil:    banUntil,
		IsPermanent: req.IsPermanent,
	}); err != nil {
		switch {
		case errors.Is(err, errAdminUserBanUnavailable):
			return s.writeAPIError(c, fiber.StatusServiceUnavailable, "ADMIN_USER_BAN_UNAVAILABLE", "store is unavailable", "")
		case errors.Is(err, store.ErrUserNotFound):
			return s.writeAPIError(c, fiber.StatusNotFound, "ADMIN_USER_BAN_TARGET_NOT_FOUND", "target user not found", "")
		default:
			return s.writeAPIError(c, fiber.StatusInternalServerError, "ADMIN_USER_BAN_FAILED", "admin user ban failed", err.Error())
		}
	}
	resp := fiber.Map{
		"ok":          true,
		"userId":      targetUserID,
		"reason":      reason,
		"isPermanent": req.IsPermanent,
	}
	if banUntil != nil {
		resp["banUntil"] = banUntil.UTC().Format(time.RFC3339)
	}
	return s.writeAPISuccess(c, fiber.StatusOK, resp)
}

func (s *Server) handleAdminUserUnban(c *fiber.Ctx) error {
	var req adminUserUnbanRequest
	if err := c.BodyParser(&req); err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "ADMIN_USER_UNBAN_INVALID_PAYLOAD", "invalid request payload", err.Error())
	}
	targetUserID, err := adminUserResolveTargetExecutor(c.Context(), s.deps.Store, req.UserID, req.Username)
	if err != nil {
		switch {
		case errors.Is(err, errAdminUserBanUnavailable):
			return s.writeAPIError(c, fiber.StatusServiceUnavailable, "ADMIN_USER_UNBAN_UNAVAILABLE", "store is unavailable", "")
		case errors.Is(err, store.ErrUserNotFound):
			return s.writeAPIError(c, fiber.StatusNotFound, "ADMIN_USER_UNBAN_TARGET_NOT_FOUND", "target user not found", "")
		default:
			return s.writeAPIError(c, fiber.StatusInternalServerError, "ADMIN_USER_UNBAN_FAILED", "admin user unban failed", err.Error())
		}
	}
	if err := adminUserUnbanExecutor(s, c.Context(), targetUserID); err != nil {
		switch {
		case errors.Is(err, errAdminUserBanUnavailable):
			return s.writeAPIError(c, fiber.StatusServiceUnavailable, "ADMIN_USER_UNBAN_UNAVAILABLE", "store is unavailable", "")
		case errors.Is(err, store.ErrUserNotFound):
			return s.writeAPIError(c, fiber.StatusNotFound, "ADMIN_USER_UNBAN_TARGET_NOT_FOUND", "target user not found", "")
		default:
			return s.writeAPIError(c, fiber.StatusInternalServerError, "ADMIN_USER_UNBAN_FAILED", "admin user unban failed", err.Error())
		}
	}
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{
		"ok":     true,
		"userId": targetUserID,
	})
}

func (s *Server) handleAdminUserBanReasons(c *fiber.Ctx) error {
	configured := configuredAdminBanReasons(s)
	items := make([]fiber.Map, 0, len(configured))
	for _, reason := range configured {
		items = append(items, fiber.Map{
			"key":     reason.Key,
			"nameKey": reason.NameKey,
		})
	}
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{
		"ok":      true,
		"reasons": items,
	})
}
