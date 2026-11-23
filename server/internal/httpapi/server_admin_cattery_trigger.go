package httpapi

import (
	"context"
	"errors"
	"math/rand"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"mysecondwebgame/server/internal/store"
)

const (
	adminCatteryTriggerTypeDiary     = "diary"
	adminCatteryTriggerTypeEncounter = "encounter"
)

type adminCatteryTriggerRequest struct {
	UserID        int64  `json:"userId"`
	KittyID       int64  `json:"kittyId"`
	TriggerType   string `json:"triggerType"`
	TargetUserID  int64  `json:"targetUserId"`
	TargetKittyID int64  `json:"targetKittyId"`
}

type adminCatteryTriggerCommand struct {
	UserID        int64
	KittyID       int64
	TriggerType   string
	TargetUserID  int64
	TargetKittyID int64
}

type adminCatteryTriggerResult struct {
	TriggerType     string `json:"triggerType"`
	UserID          int64  `json:"userId"`
	KittyID         int64  `json:"kittyId"`
	TargetUserID    int64  `json:"targetUserId,omitempty"`
	TargetKittyID   int64  `json:"targetKittyId,omitempty"`
	KittyName       string `json:"kittyName,omitempty"`
	TargetKittyName string `json:"targetKittyName,omitempty"`
}

var errAdminCatteryTriggerUnavailable = errors.New("admin cattery trigger unavailable")

var adminTriggerDiaryIdleVariantKeys = []string{
	"game.catteryJournal.diary.idleVariants.1",
	"game.catteryJournal.diary.idleVariants.2",
	"game.catteryJournal.diary.idleVariants.3",
	"game.catteryJournal.diary.idleVariants.4",
	"game.catteryJournal.diary.idleVariants.5",
	"game.catteryJournal.diary.idleVariants.6",
	"game.catteryJournal.diary.idleVariants.7",
	"game.catteryJournal.diary.idleVariants.8",
	"game.catteryJournal.diary.idleVariants.9",
	"game.catteryJournal.diary.idleVariants.10",
	"game.catteryJournal.diary.idleVariants.11",
	"game.catteryJournal.diary.idleVariants.12",
	"game.catteryJournal.diary.idleVariants.13",
	"game.catteryJournal.diary.idleVariants.14",
	"game.catteryJournal.diary.idleVariants.15",
	"game.catteryJournal.diary.idleVariants.16",
	"game.catteryJournal.diary.idleVariants.17",
	"game.catteryJournal.diary.idleVariants.18",
	"game.catteryJournal.diary.idleVariants.19",
	"game.catteryJournal.diary.idleVariants.20",
}

var adminTriggerDiaryWorkingVariantKeys = []string{
	"game.catteryJournal.diary.workingVariants.1",
	"game.catteryJournal.diary.workingVariants.2",
	"game.catteryJournal.diary.workingVariants.3",
	"game.catteryJournal.diary.workingVariants.4",
	"game.catteryJournal.diary.workingVariants.5",
	"game.catteryJournal.diary.workingVariants.6",
	"game.catteryJournal.diary.workingVariants.7",
	"game.catteryJournal.diary.workingVariants.8",
	"game.catteryJournal.diary.workingVariants.9",
	"game.catteryJournal.diary.workingVariants.10",
	"game.catteryJournal.diary.workingVariants.11",
	"game.catteryJournal.diary.workingVariants.12",
	"game.catteryJournal.diary.workingVariants.13",
	"game.catteryJournal.diary.workingVariants.14",
	"game.catteryJournal.diary.workingVariants.15",
	"game.catteryJournal.diary.workingVariants.16",
	"game.catteryJournal.diary.workingVariants.17",
	"game.catteryJournal.diary.workingVariants.18",
	"game.catteryJournal.diary.workingVariants.19",
	"game.catteryJournal.diary.workingVariants.20",
}

var adminTriggerEncounterMetVariantKeys = []string{
	"game.catteryJournal.encounter.metVariants.1",
	"game.catteryJournal.encounter.metVariants.2",
	"game.catteryJournal.encounter.metVariants.3",
	"game.catteryJournal.encounter.metVariants.4",
	"game.catteryJournal.encounter.metVariants.5",
	"game.catteryJournal.encounter.metVariants.6",
	"game.catteryJournal.encounter.metVariants.7",
	"game.catteryJournal.encounter.metVariants.8",
	"game.catteryJournal.encounter.metVariants.9",
	"game.catteryJournal.encounter.metVariants.10",
	"game.catteryJournal.encounter.metVariants.11",
	"game.catteryJournal.encounter.metVariants.12",
	"game.catteryJournal.encounter.metVariants.13",
	"game.catteryJournal.encounter.metVariants.14",
	"game.catteryJournal.encounter.metVariants.15",
	"game.catteryJournal.encounter.metVariants.16",
	"game.catteryJournal.encounter.metVariants.17",
	"game.catteryJournal.encounter.metVariants.18",
	"game.catteryJournal.encounter.metVariants.19",
	"game.catteryJournal.encounter.metVariants.20",
}

func adminPickJournalKey(rng *rand.Rand, keys []string, fallback string) string {
	if len(keys) == 0 {
		return fallback
	}
	return keys[rng.Intn(len(keys))]
}

var adminCatteryTriggerExecutor = func(s *Server, ctx context.Context, cmd adminCatteryTriggerCommand) (adminCatteryTriggerResult, error) {
	if s.deps.Store == nil {
		return adminCatteryTriggerResult{}, errAdminCatteryTriggerUnavailable
	}
	isBanned, banErr := s.deps.Store.IsUserBanned(ctx, cmd.UserID)
	if banErr != nil {
		return adminCatteryTriggerResult{}, banErr
	}
	if isBanned {
		return adminCatteryTriggerResult{}, store.ErrUserBanned
	}
	kitty, err := s.deps.Store.GetPlayerKitty(ctx, cmd.UserID, cmd.KittyID)
	if err != nil {
		return adminCatteryTriggerResult{}, err
	}
	maxEntries := s.deps.Config.GlobalSettings.KittyJournal.MaxEntriesPerKitty
	if maxEntries <= 0 {
		maxEntries = 200
	}
	rngAdmin := rand.New(rand.NewSource(time.Now().UnixNano()))
	switch cmd.TriggerType {
	case adminCatteryTriggerTypeDiary:
		actionID, actionErr := s.deps.Store.GetKittyRunningActionID(ctx, cmd.UserID, cmd.KittyID)
		if actionErr != nil {
			return adminCatteryTriggerResult{}, actionErr
		}
		contentKey := adminPickJournalKey(rngAdmin, adminTriggerDiaryIdleVariantKeys, "game.catteryJournal.diary.idle")
		contentParams := map[string]any{
			"kittyName": kitty.Name,
		}
		if strings.TrimSpace(actionID) != "" {
			contentKey = adminPickJournalKey(rngAdmin, adminTriggerDiaryWorkingVariantKeys, "game.catteryJournal.diary.working")
			contentParams["actionId"] = actionID
		}
		mood := 0.5
		if err := s.deps.Store.CreateKittyDiaryWithRewards(ctx, store.KittyJournalWriteInput{
			UserID:        cmd.UserID,
			KittyID:       cmd.KittyID,
			EntryType:     store.KittyJournalEntryTypeDiary,
			TitleKey:      "game.catteryJournal.title.diary",
			ContentKey:    contentKey,
			ContentParams: contentParams,
			Mood:          &mood,
		}, nil, maxEntries); err != nil {
			return adminCatteryTriggerResult{}, err
		}
		return adminCatteryTriggerResult{
			TriggerType: cmd.TriggerType,
			UserID:      cmd.UserID,
			KittyID:     cmd.KittyID,
			KittyName:   kitty.Name,
		}, nil
	case adminCatteryTriggerTypeEncounter:
		var target store.KittyCandidate
		if cmd.TargetUserID > 0 || cmd.TargetKittyID > 0 {
			if cmd.TargetUserID <= 0 || cmd.TargetKittyID <= 0 || cmd.TargetUserID == cmd.UserID {
				return adminCatteryTriggerResult{}, store.ErrKittyEncounterTargetNotFound
			}
			targetBanned, targetBanErr := s.deps.Store.IsUserBanned(ctx, cmd.TargetUserID)
			if targetBanErr != nil {
				return adminCatteryTriggerResult{}, targetBanErr
			}
			if targetBanned {
				return adminCatteryTriggerResult{}, store.ErrUserBanned
			}
			targetKitty, targetErr := s.deps.Store.GetPlayerKitty(ctx, cmd.TargetUserID, cmd.TargetKittyID)
			if targetErr != nil {
				return adminCatteryTriggerResult{}, targetErr
			}
			target = store.KittyCandidate{
				UserID: cmd.TargetUserID,
				Kitty:  targetKitty,
			}
		} else {
			targetCandidate, pickErr := s.deps.Store.PickRandomEncounterTarget(ctx, cmd.UserID, cmd.KittyID, rngAdmin)
			if pickErr != nil {
				return adminCatteryTriggerResult{}, pickErr
			}
			target = targetCandidate
		}
		encounterContentKey := adminPickJournalKey(rngAdmin, adminTriggerEncounterMetVariantKeys, "game.catteryJournal.encounter.met")
		if err := s.deps.Store.CreateKittyEncounterWithDualJournals(ctx, store.KittyEncounterWriteInput{
			FromUserID:  cmd.UserID,
			FromKittyID: cmd.KittyID,
			ToUserID:    target.UserID,
			ToKittyID:   target.Kitty.ID,
			ContentKey:  encounterContentKey,
			ContentParams: map[string]any{
				"kittyName":       kitty.Name,
				"targetKittyName": target.Kitty.Name,
			},
		}, store.KittyJournalWriteInput{
			UserID:     cmd.UserID,
			KittyID:    cmd.KittyID,
			EntryType:  store.KittyJournalEntryTypeEncounter,
			TitleKey:   "game.catteryJournal.title.encounter",
			ContentKey: encounterContentKey,
			ContentParams: map[string]any{
				"kittyName":       kitty.Name,
				"targetKittyName": target.Kitty.Name,
			},
		}, store.KittyJournalWriteInput{
			UserID:     target.UserID,
			KittyID:    target.Kitty.ID,
			EntryType:  store.KittyJournalEntryTypeEncounter,
			TitleKey:   "game.catteryJournal.title.encounter",
			ContentKey: encounterContentKey,
			ContentParams: map[string]any{
				"kittyName":       target.Kitty.Name,
				"targetKittyName": kitty.Name,
			},
		}, maxEntries); err != nil {
			return adminCatteryTriggerResult{}, err
		}
		return adminCatteryTriggerResult{
			TriggerType:     cmd.TriggerType,
			UserID:          cmd.UserID,
			KittyID:         cmd.KittyID,
			TargetUserID:    target.UserID,
			TargetKittyID:   target.Kitty.ID,
			KittyName:       kitty.Name,
			TargetKittyName: target.Kitty.Name,
		}, nil
	default:
		return adminCatteryTriggerResult{}, store.ErrKittyNotFound
	}
}

func normalizeAdminCatteryTriggerType(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", adminCatteryTriggerTypeDiary:
		return adminCatteryTriggerTypeDiary
	case adminCatteryTriggerTypeEncounter:
		return adminCatteryTriggerTypeEncounter
	default:
		return ""
	}
}

func (s *Server) handleAdminCatteryTrigger(c *fiber.Ctx) error {
	var req adminCatteryTriggerRequest
	if err := c.BodyParser(&req); err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "ADMIN_CATTERY_TRIGGER_INVALID_PAYLOAD", "invalid request payload", err.Error())
	}
	if req.UserID <= 0 || req.KittyID <= 0 {
		return s.writeAPIError(c, fiber.StatusBadRequest, "ADMIN_CATTERY_TRIGGER_INVALID_OWNER", "invalid userId or kittyId", "")
	}
	triggerType := normalizeAdminCatteryTriggerType(req.TriggerType)
	if triggerType == "" {
		return s.writeAPIError(c, fiber.StatusBadRequest, "ADMIN_CATTERY_TRIGGER_INVALID_TYPE", "invalid trigger type", "")
	}
	result, err := adminCatteryTriggerExecutor(s, c.Context(), adminCatteryTriggerCommand{
		UserID:        req.UserID,
		KittyID:       req.KittyID,
		TriggerType:   triggerType,
		TargetUserID:  req.TargetUserID,
		TargetKittyID: req.TargetKittyID,
	})
	if err != nil {
		switch {
		case errors.Is(err, errAdminCatteryTriggerUnavailable):
			return s.writeAPIError(c, fiber.StatusServiceUnavailable, "ADMIN_CATTERY_TRIGGER_UNAVAILABLE", "store is unavailable", "")
		case errors.Is(err, store.ErrKittyNotFound):
			return s.writeAPIError(c, fiber.StatusNotFound, "ADMIN_CATTERY_TRIGGER_KITTY_NOT_FOUND", "kitty not found", "")
		case errors.Is(err, store.ErrKittyEncounterTargetNotFound):
			return s.writeAPIError(c, fiber.StatusConflict, "ADMIN_CATTERY_TRIGGER_TARGET_NOT_FOUND", "encounter target not found", "")
		case errors.Is(err, store.ErrUserBanned):
			return s.writeAPIError(c, fiber.StatusConflict, "ADMIN_CATTERY_TRIGGER_USER_BANNED", "user is banned", "")
		default:
			return s.writeAPIError(c, fiber.StatusInternalServerError, "ADMIN_CATTERY_TRIGGER_FAILED", "admin cattery trigger failed", err.Error())
		}
	}
	s.syncAfterMutation(mutationSyncScope{
		UserIDs:     []int64{result.UserID, result.TargetUserID},
		PlayerState: true,
	})
	return s.writeAPISuccess(c, fiber.StatusOK, result)
}
