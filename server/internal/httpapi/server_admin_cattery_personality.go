package httpapi

import (
	"errors"
	"math/rand"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"mysecondwebgame/server/internal/store"
)

type adminKittyPersonalityRequest struct {
	UserID        int64  `json:"userId"`
	KittyID       int64  `json:"kittyId"`
	PersonalityID string `json:"personalityId"`
}

func (s *Server) handleAdminKittyPersonalityAdd(c *fiber.Ctx) error {
	var req adminKittyPersonalityRequest
	if err := c.BodyParser(&req); err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "ADMIN_KITTY_PERSONALITY_ADD_INVALID_PAYLOAD", "invalid payload", err.Error())
	}
	if req.UserID <= 0 || req.KittyID <= 0 {
		return s.writeAPIError(c, fiber.StatusBadRequest, "ADMIN_KITTY_PERSONALITY_ADD_INVALID_TARGET", "invalid target", "")
	}
	personalityID := strings.TrimSpace(req.PersonalityID)
	kitty, err := s.deps.Store.AddKittyPersonality(c.Context(), req.UserID, req.KittyID, personalityID)
	if err != nil {
		switch {
		case errors.Is(err, store.ErrKittyPersonalityInvalid):
			return s.writeAPIError(c, fiber.StatusBadRequest, "ADMIN_KITTY_PERSONALITY_INVALID", "invalid kitty personality", "")
		case errors.Is(err, store.ErrKittyPersonalityExists):
			return s.writeAPIError(c, fiber.StatusConflict, "ADMIN_KITTY_PERSONALITY_EXISTS", "kitty personality already exists", "")
		case errors.Is(err, store.ErrKittyNotFound):
			return s.writeAPIError(c, fiber.StatusNotFound, "ADMIN_KITTY_NOT_FOUND", "kitty not found", "")
		default:
			return s.writeAPIError(c, fiber.StatusInternalServerError, "ADMIN_KITTY_PERSONALITY_ADD_FAILED", "add kitty personality failed", err.Error())
		}
	}
	s.syncAfterMutation(mutationSyncScope{
		UserIDs:     []int64{req.UserID},
		PlayerState: true,
	})
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{
		"ok":          true,
		"userId":      req.UserID,
		"kittyId":     req.KittyID,
		"personality": personalityID,
		"kitty":       kitty,
	})
}

// handleAdminKittyPersonalityJournalTrigger grants a personality to a kitty (if not already owned)
// and writes a diary journal entry with extra.acquiredPersonality set, for testing the UI flow.
func (s *Server) handleAdminKittyPersonalityJournalTrigger(c *fiber.Ctx) error {
	var req adminKittyPersonalityRequest
	if err := c.BodyParser(&req); err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "ADMIN_KITTY_PERSONALITY_JOURNAL_INVALID_PAYLOAD", "invalid payload", err.Error())
	}
	if req.UserID <= 0 || req.KittyID <= 0 {
		return s.writeAPIError(c, fiber.StatusBadRequest, "ADMIN_KITTY_PERSONALITY_JOURNAL_INVALID_TARGET", "invalid userId or kittyId", "")
	}
	personalityID := strings.ToLower(strings.TrimSpace(req.PersonalityID))
	if personalityID == "" {
		return s.writeAPIError(c, fiber.StatusBadRequest, "ADMIN_KITTY_PERSONALITY_JOURNAL_INVALID_PERSONALITY", "personalityId is required", "")
	}

	// Step 1: grant the personality (tolerate already-owned)
	kitty, addErr := s.deps.Store.AddKittyPersonality(c.Context(), req.UserID, req.KittyID, personalityID)
	alreadyOwned := errors.Is(addErr, store.ErrKittyPersonalityExists)
	if addErr != nil && !alreadyOwned {
		switch {
		case errors.Is(addErr, store.ErrKittyPersonalityInvalid):
			return s.writeAPIError(c, fiber.StatusBadRequest, "ADMIN_KITTY_PERSONALITY_JOURNAL_INVALID_PERSONALITY", "unknown personalityId", "")
		case errors.Is(addErr, store.ErrKittyNotFound):
			return s.writeAPIError(c, fiber.StatusNotFound, "ADMIN_KITTY_NOT_FOUND", "kitty not found", "")
		default:
			return s.writeAPIError(c, fiber.StatusInternalServerError, "ADMIN_KITTY_PERSONALITY_JOURNAL_GRANT_FAILED", "grant personality failed", addErr.Error())
		}
	}
	if alreadyOwned {
		// fetch kitty separately to get the name for the journal
		fetched, fetchErr := s.deps.Store.GetPlayerKitty(c.Context(), req.UserID, req.KittyID)
		if fetchErr != nil {
			return s.writeAPIError(c, fiber.StatusNotFound, "ADMIN_KITTY_NOT_FOUND", "kitty not found", fetchErr.Error())
		}
		kitty = fetched
	}

	// Step 2: write a diary entry with acquiredPersonality in extra
	maxEntries := s.deps.Config.GlobalSettings.KittyJournal.MaxEntriesPerKitty
	if maxEntries <= 0 {
		maxEntries = 200
	}
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	contentKey := adminPickJournalKey(rng, adminTriggerDiaryIdleVariantKeys, "game.catteryJournal.diary.idle")
	mood := 0.5
	if err := s.deps.Store.CreateKittyDiaryWithRewards(c.Context(), store.KittyJournalWriteInput{
		UserID:        req.UserID,
		KittyID:       req.KittyID,
		EntryType:     store.KittyJournalEntryTypeDiary,
		TitleKey:      "game.catteryJournal.title.diary",
		ContentKey:    contentKey,
		ContentParams: map[string]any{"kittyName": kitty.Name},
		Mood:          &mood,
		Extra:         map[string]any{"acquiredPersonality": personalityID},
	}, nil, maxEntries); err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "ADMIN_KITTY_PERSONALITY_JOURNAL_WRITE_FAILED", "write journal failed", err.Error())
	}

	s.syncAfterMutation(mutationSyncScope{
		UserIDs:     []int64{req.UserID},
		PlayerState: true,
	})
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{
		"ok":           true,
		"userId":       req.UserID,
		"kittyId":      req.KittyID,
		"kittyName":    kitty.Name,
		"personalityId": personalityID,
		"alreadyOwned": alreadyOwned,
	})
}

func (s *Server) handleAdminKittyPersonalityRemove(c *fiber.Ctx) error {
	var req adminKittyPersonalityRequest
	if err := c.BodyParser(&req); err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "ADMIN_KITTY_PERSONALITY_REMOVE_INVALID_PAYLOAD", "invalid payload", err.Error())
	}
	if req.UserID <= 0 || req.KittyID <= 0 {
		return s.writeAPIError(c, fiber.StatusBadRequest, "ADMIN_KITTY_PERSONALITY_REMOVE_INVALID_TARGET", "invalid target", "")
	}
	personalityID := strings.TrimSpace(req.PersonalityID)
	kitty, err := s.deps.Store.RemoveKittyPersonality(c.Context(), req.UserID, req.KittyID, personalityID)
	if err != nil {
		switch {
		case errors.Is(err, store.ErrKittyPersonalityInvalid):
			return s.writeAPIError(c, fiber.StatusBadRequest, "ADMIN_KITTY_PERSONALITY_INVALID", "invalid kitty personality", "")
		case errors.Is(err, store.ErrKittyPersonalityNotFound):
			return s.writeAPIError(c, fiber.StatusBadRequest, "ADMIN_KITTY_PERSONALITY_NOT_FOUND", "kitty personality not found", "")
		case errors.Is(err, store.ErrKittyNotFound):
			return s.writeAPIError(c, fiber.StatusNotFound, "ADMIN_KITTY_NOT_FOUND", "kitty not found", "")
		default:
			return s.writeAPIError(c, fiber.StatusInternalServerError, "ADMIN_KITTY_PERSONALITY_REMOVE_FAILED", "remove kitty personality failed", err.Error())
		}
	}
	s.syncAfterMutation(mutationSyncScope{
		UserIDs:     []int64{req.UserID},
		PlayerState: true,
	})
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{
		"ok":          true,
		"userId":      req.UserID,
		"kittyId":     req.KittyID,
		"personality": personalityID,
		"kitty":       kitty,
	})
}
