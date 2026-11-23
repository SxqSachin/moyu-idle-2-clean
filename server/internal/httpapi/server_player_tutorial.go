package httpapi

import (
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"mysecondwebgame/server/internal/store"
)

type tutorialProgressUpsertRequest struct {
	TutorialID string `json:"tutorialId"`
	Completed  bool   `json:"completed"`
	Version    int    `json:"version"`
}

type tutorialProgressResetRequest struct {
	TutorialID string `json:"tutorialId"`
}

// tutorialProgressEntryView is the wire shape returned to the client. It mirrors
// the frontend TutorialProgress type.
type tutorialProgressEntryView struct {
	Completed   bool  `json:"completed"`
	CompletedAt int64 `json:"completedAt"`
	Version     int   `json:"version"`
}

func (s *Server) handlePlayerTutorialProgressGet(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	progress, err := s.deps.Store.GetTutorialProgress(c.Context(), userID)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, codeSettingsReadFailed, "Failed to load tutorial progress", err.Error())
	}
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{
		"tutorialProgress": tutorialProgressView(progress),
	})
}

func (s *Server) handlePlayerTutorialProgressUpsert(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	var req tutorialProgressUpsertRequest
	if err := c.BodyParser(&req); err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, codeProfileInvalidPayload, "Invalid request payload", err.Error())
	}
	tutorialID := strings.TrimSpace(req.TutorialID)
	if !store.IsValidTutorialID(tutorialID) {
		return s.writeAPIError(c, fiber.StatusBadRequest, codeProfileInvalidPayload, "Invalid tutorial id", "")
	}
	version := max(req.Version, 1)
	entry := store.TutorialProgressEntry{
		Completed:   req.Completed,
		CompletedAt: time.Now().UnixMilli(),
		Version:     version,
	}
	if err := s.deps.Store.UpsertTutorialProgressEntry(c.Context(), userID, tutorialID, entry); err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, codeSettingsUpdateFailed, "Failed to save tutorial progress", err.Error())
	}
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{
		"tutorialId":  tutorialID,
		"completed":   entry.Completed,
		"completedAt": entry.CompletedAt,
		"version":     entry.Version,
	})
}

func (s *Server) handlePlayerTutorialProgressReset(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	var req tutorialProgressResetRequest
	if err := c.BodyParser(&req); err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, codeProfileInvalidPayload, "Invalid request payload", err.Error())
	}
	tutorialID := strings.TrimSpace(req.TutorialID)
	if !store.IsValidTutorialID(tutorialID) {
		return s.writeAPIError(c, fiber.StatusBadRequest, codeProfileInvalidPayload, "Invalid tutorial id", "")
	}
	if err := s.deps.Store.DeleteTutorialProgressEntry(c.Context(), userID, tutorialID); err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, codeSettingsUpdateFailed, "Failed to reset tutorial progress", err.Error())
	}
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{"tutorialId": tutorialID})
}

func tutorialProgressView(progress map[string]store.TutorialProgressEntry) map[string]tutorialProgressEntryView {
	view := make(map[string]tutorialProgressEntryView, len(progress))
	for id, entry := range progress {
		view[id] = tutorialProgressEntryView{
			Completed:   entry.Completed,
			CompletedAt: entry.CompletedAt,
			Version:     entry.Version,
		}
	}
	return view
}
