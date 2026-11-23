package httpapi

import (
	"context"
	"errors"
	"strings"

	"github.com/gofiber/fiber/v2"
	"golang.org/x/crypto/bcrypt"
	"mysecondwebgame/server/internal/store"
)

type languageUpdateRequest struct {
	Language string `json:"language"`
}

type themeUpdateRequest struct {
	Theme string `json:"theme"`
}

type menuLayoutUpdateRequest struct {
	MenuLayout string `json:"menuLayout"`
}

type simpleModeUpdateRequest struct {
	SimpleMode bool `json:"simpleMode"`
}

type usernameUpdateRequest struct {
	Username                string  `json:"username"`
	ExpectedCurrentUsername *string `json:"expectedCurrentUsername"`
}

type playerUsernameRenamer interface {
	RenamePlayerUsername(context.Context, int64, string, string) (store.PlayerUsernameRenameResult, error)
}

const (
	codeProfileUsernameInsufficientItem = "PROFILE_USERNAME_INSUFFICIENT_ITEM"
	codeProfileUsernameConfigInvalid    = "PROFILE_USERNAME_RENAME_CONFIG_INVALID"
	codeProfileUsernameUserNotFound     = "PROFILE_USERNAME_USER_NOT_FOUND"
	codeProfileUsernameCurrentConflict  = "PROFILE_USERNAME_CURRENT_CONFLICT"
)

type emailUpdateRequest struct {
	Email string `json:"email"`
}

type passwordUpdateRequest struct {
	CurrentPassword string `json:"currentPassword"`
	NewPassword     string `json:"newPassword"`
}

func (s *Server) handlePlayerSettingsGet(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	settings, err := s.deps.Store.GetUserSettings(c.Context(), userID)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, codeSettingsReadFailed, "Failed to load player settings", err.Error())
	}
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{
		"userId":               settings.UserID,
		"username":             settings.Username,
		"usernameChangeId":     settings.UsernameChangeID,
		"email":                settings.Email,
		"emailVerified":        settings.EmailVerified,
		"language":             settings.Language,
		"theme":                settings.Theme,
		"menuLayout":           settings.MenuLayout,
		"simpleMode":           settings.SimpleMode,
		"disableIPLoginVerify": settings.DisableIPLoginVerify,
	})
}

func (s *Server) handlePlayerSettingsLanguage(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	var req languageUpdateRequest
	if err := c.BodyParser(&req); err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, codeProfileInvalidPayload, "Invalid request payload", err.Error())
	}
	normalized := store.NormalizeUserLanguage(req.Language)
	if normalized != strings.TrimSpace(req.Language) {
		return s.writeAPIError(c, fiber.StatusBadRequest, codeSettingsInvalidLanguage, "Invalid language value", "")
	}
	if err := s.deps.Store.UpsertUserLanguage(c.Context(), userID, normalized); err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, codeSettingsUpdateFailed, "Failed to update language", err.Error())
	}
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{"language": normalized})
}

func (s *Server) handlePlayerSettingsTheme(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	var req themeUpdateRequest
	if err := c.BodyParser(&req); err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, codeProfileInvalidPayload, "Invalid request payload", err.Error())
	}
	normalized := store.NormalizeUserTheme(req.Theme)
	if normalized != strings.ToLower(strings.TrimSpace(req.Theme)) {
		return s.writeAPIError(c, fiber.StatusBadRequest, codeSettingsInvalidTheme, "Invalid theme value", "")
	}
	if err := s.deps.Store.UpsertUserTheme(c.Context(), userID, normalized); err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, codeSettingsUpdateFailed, "Failed to update theme", err.Error())
	}
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{"theme": normalized})
}

func (s *Server) handlePlayerSettingsMenuLayout(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	var req menuLayoutUpdateRequest
	if err := c.BodyParser(&req); err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, codeProfileInvalidPayload, "Invalid request payload", err.Error())
	}
	normalized := store.NormalizeUserMenuLayout(req.MenuLayout)
	if normalized != strings.TrimSpace(strings.ToLower(req.MenuLayout)) {
		return s.writeAPIError(c, fiber.StatusBadRequest, codeSettingsInvalidMenuLayout, "Invalid menu layout value", "")
	}
	if err := s.deps.Store.UpsertUserMenuLayout(c.Context(), userID, normalized); err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, codeSettingsUpdateFailed, "Failed to update menu layout", err.Error())
	}
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{"menuLayout": normalized})
}

func (s *Server) handlePlayerSettingsSimpleMode(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	var req simpleModeUpdateRequest
	if err := c.BodyParser(&req); err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, codeProfileInvalidPayload, "Invalid request payload", err.Error())
	}
	if err := s.deps.Store.UpsertUserSimpleMode(c.Context(), userID, req.SimpleMode); err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, codeSettingsUpdateFailed, "Failed to update simple mode", err.Error())
	}
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{"simpleMode": req.SimpleMode})
}

func (s *Server) handlePlayerProfileUsername(c *fiber.Ctx) error {
	return s.handlePlayerProfileUsernameWithStore(c, s.deps.Store)
}

func (s *Server) handlePlayerProfileUsernameWithStore(c *fiber.Ctx, renamer playerUsernameRenamer) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	var req usernameUpdateRequest
	if err := c.BodyParser(&req); err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, codeProfileInvalidPayload, "Invalid request payload", err.Error())
	}
	nextUsername := strings.TrimSpace(req.Username)
	if usernameLength(nextUsername) < minUsernameLength {
		return s.writeAPIError(c, fiber.StatusBadRequest, codeProfileUsernameTooShort, "Username is too short", "")
	}
	if usernameLength(nextUsername) > maxUsernameLength {
		return s.writeAPIError(c, fiber.StatusBadRequest, codeProfileUsernameTooLong, "Username is too long", "")
	}
	if !isValidUsername(nextUsername) {
		return s.writeAPIError(c, fiber.StatusBadRequest, codeProfileUsernameFormat, "Username format is invalid", "")
	}
	if req.ExpectedCurrentUsername == nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, codeProfileInvalidPayload, "Invalid request payload", "expectedCurrentUsername is required")
	}
	expectedCurrentUsername := strings.TrimSpace(*req.ExpectedCurrentUsername)
	if expectedCurrentUsername == "" {
		return s.writeAPIError(c, fiber.StatusBadRequest, codeProfileInvalidPayload, "Invalid request payload", "expectedCurrentUsername must not be empty")
	}
	if err := s.rejectModeratedAPI(c, moderationRuleProfileUsername, nextUsername, userID); err != nil {
		return err
	}
	result, err := renamer.RenamePlayerUsername(c.Context(), userID, nextUsername, expectedCurrentUsername)
	if err != nil {
		switch {
		case errors.Is(err, store.ErrPlayerUsernameConflict):
			return s.writeAPIError(c, fiber.StatusConflict, codeProfileUsernameCurrentConflict, "Username changed since it was loaded", "")
		case errors.Is(err, store.ErrPlayerUsernameUnchanged):
			return s.writeAPIError(c, fiber.StatusBadRequest, codeProfileUsernameSame, "Username is unchanged", "")
		case errors.Is(err, store.ErrDuplicateUser):
			return s.writeAPIError(c, fiber.StatusConflict, codeProfileUsernameExists, "Username already exists", "")
		case errors.Is(err, store.ErrInsufficientItem):
			return s.writeAPIError(c, fiber.StatusConflict, codeProfileUsernameInsufficientItem, "Insufficient username rename item", "")
		case errors.Is(err, store.ErrPlayerUsernameInvalid):
			return s.writeAPIError(c, fiber.StatusBadRequest, codeProfileUsernameFormat, "Username format is invalid", "")
		case errors.Is(err, store.ErrPlayerUsernameRenameCostInvalid):
			return s.writeAPIError(c, fiber.StatusInternalServerError, codeProfileUsernameConfigInvalid, "Username rename configuration is invalid", err.Error())
		case errors.Is(err, store.ErrUserNotFound):
			return s.writeAPIError(c, fiber.StatusNotFound, codeProfileUsernameUserNotFound, "Player not found", "")
		default:
			return s.writeAPIError(c, fiber.StatusInternalServerError, codeProfileUpdateFailed, "Failed to update profile", err.Error())
		}
	}
	s.invalidatePlayerProfileCaches(userID)
	s.syncAfterMutation(mutationSyncScope{UserIDs: []int64{userID}, Inventory: true})
	s.broadcastPlayerProfileChanged(c.Context(), userID, result.Username, result.HistoryID)
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{
		"username":         result.Username,
		"previousUsername": result.PreviousUsername,
		"changedAt":        result.ChangedAt,
		"changeId":         result.HistoryID,
	})
}

func (s *Server) handlePlayerProfileEmail(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	var req emailUpdateRequest
	if err := c.BodyParser(&req); err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, codeProfileInvalidPayload, "Invalid request payload", err.Error())
	}
	nextEmail := strings.TrimSpace(strings.ToLower(req.Email))
	if nextEmail == "" || len(nextEmail) > maxEmailLength || !isValidEmail(nextEmail) {
		return s.writeAPIError(c, fiber.StatusBadRequest, codeProfileEmailInvalid, "Email is invalid", "")
	}
	if s.isEmailDomainBlocked(nextEmail) {
		return s.writeEmailDomainBlocked(c)
	}
	settings, err := s.deps.Store.GetUserSettings(c.Context(), userID)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, codeProfileUpdateFailed, "Failed to update profile", err.Error())
	}
	if strings.EqualFold(settings.Email, nextEmail) {
		return s.writeAPIError(c, fiber.StatusBadRequest, codeProfileEmailUnchanged, "Email is unchanged", "")
	}
	if err := s.deps.Store.UpdateUnverifiedEmail(c.Context(), userID, nextEmail); err != nil {
		if errors.Is(err, store.ErrDuplicateEmail) {
			return s.writeAPIError(c, fiber.StatusConflict, codeProfileEmailExists, "Email already exists", "")
		}
		return s.writeAPIError(c, fiber.StatusInternalServerError, codeProfileUpdateFailed, "Failed to update profile", err.Error())
	}
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{"email": nextEmail, "emailVerified": false})
}

func (s *Server) handlePlayerProfilePassword(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	var req passwordUpdateRequest
	if err := c.BodyParser(&req); err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, codeProfileInvalidPayload, "Invalid request payload", err.Error())
	}
	if len(req.NewPassword) < minPasswordLength {
		return s.writeAPIError(c, fiber.StatusBadRequest, codeProfilePasswordTooShort, "New password is too short", "")
	}
	if req.CurrentPassword == req.NewPassword {
		return s.writeAPIError(c, fiber.StatusBadRequest, codeProfilePasswordSame, "New password must be different", "")
	}
	currentHash, err := s.deps.Store.GetPasswordHashByUserID(c.Context(), userID)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, codeProfileUpdateFailed, "Failed to update profile", err.Error())
	}
	if err := bcrypt.CompareHashAndPassword([]byte(currentHash), []byte(req.CurrentPassword)); err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeProfilePasswordInvalidOld, "Current password is invalid", "")
	}
	nextHash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, codeProfileUpdateFailed, "Failed to update profile", err.Error())
	}
	if err := s.deps.Store.UpdatePasswordHash(c.Context(), userID, string(nextHash)); err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, codeProfileUpdateFailed, "Failed to update profile", err.Error())
	}
	sessionVersion, err := s.deps.Store.RotateSessionVersion(c.Context(), userID)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, codeAuthSignTokenFailed, "Failed to create session", err.Error())
	}
	token, err := s.deps.Auth.Sign(userID, sessionVersion)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, codeAuthSignTokenFailed, "Failed to create session", err.Error())
	}
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{
		"userId": userID,
		"token":  token,
	})
}

type disableIPLoginVerifyRequest struct {
	Disabled bool `json:"disableIPLoginVerify"`
}

func (s *Server) handlePlayerSettingsDisableIPLoginVerify(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	var req disableIPLoginVerifyRequest
	if err := c.BodyParser(&req); err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, codeProfileInvalidPayload, "Invalid request payload", err.Error())
	}
	if err := s.deps.Store.UpsertDisableIPLoginVerify(c.Context(), userID, req.Disabled); err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, codeSettingsUpdateFailed, "Failed to update setting", err.Error())
	}
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{"disableIPLoginVerify": req.Disabled})
}
