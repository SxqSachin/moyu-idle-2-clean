package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"mysecondwebgame/server/internal/store"

	"github.com/gofiber/fiber/v2"
)

type fishingSaleRequest struct {
	InstanceIDs       []int64 `json:"instanceIds"`
	ExpectedTotalGold int64   `json:"expectedTotalGold"`
	IdempotencyKey    string  `json:"idempotencyKey"`
}

type fishingTournamentClaimRequest struct {
	TournamentID string `json:"tournamentId"`
	Division     string `json:"division"`
}

type fishingPondFishRequest struct {
	InstanceID int64 `json:"instanceId"`
}

type fishingPondFishBatchRequest struct {
	InstanceIDs []int64 `json:"instanceIds"`
}

type fishingPondFeedRequest struct {
	ItemID   string `json:"itemId"`
	Quantity int64  `json:"quantity"`
}

type fishingPondPairRequest struct {
	FemaleInstanceID int64 `json:"femaleInstanceId"`
	MaleInstanceID   int64 `json:"maleInstanceId"`
}

func (s *Server) handleFishingOverview(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	recoveryCtx, recoveryCancel := context.WithTimeout(c.Context(), actionCommandTimeout)
	recovered, recoveryErr := s.deps.Store.RecoverOrphanedFishingTrip(recoveryCtx, userID)
	recoveryCancel()
	if recoveryErr != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "FISHING_PROGRESS_FAILED", "repair fishing trip failed", recoveryErr.Error())
	}
	if recovered {
		s.syncAfterMutation(mutationSyncScope{
			UserIDs: []int64{userID}, PlayerState: true, Inventory: true, Queue: true,
		})
		s.pushFishingState(userID)
	}
	if !s.hasOfflineFastForwardReady(userID) {
		ctx, cancel := context.WithTimeout(c.Context(), actionCommandTimeout)
		advance, advanceErr := s.actionRuntime().AdvanceDue(ctx, userID)
		cancel()
		if advanceErr != nil {
			return s.writeAPIError(c, fiber.StatusInternalServerError, "FISHING_PROGRESS_FAILED", "update fishing trip failed", advanceErr.Error())
		}
		s.syncActionRuntime(userID, advance.Specs, advance.Queue, advance.State, advance.Attr, advance.Summary)
	}
	waters, spots, species, pondSpecies, tackle := s.deps.Store.FishingCatalog()
	anglers, err := s.deps.Store.ListFishingAnglers(c.Context(), userID)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "FISHING_LEVEL_FAILED", "load fishing anglers failed", err.Error())
	}
	level := int64(1)
	if len(anglers) > 0 {
		level = anglers[0].FishingLevel
	}
	forecast, err := s.deps.Store.GetFishingForecast(c.Context(), userID, time.Now())
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "FISHING_FORECAST_FAILED", "load fishing forecast failed", err.Error())
	}
	ownedTackle, err := s.deps.Store.ListOwnedFishingTackle(c.Context(), userID)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "FISHING_TACKLE_FAILED", "load fishing tackle failed", err.Error())
	}
	trip, err := s.deps.Store.GetCurrentFishingTrip(c.Context(), userID)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "FISHING_TRIP_FAILED", "load fishing trip failed", err.Error())
	}
	latestTrip, err := s.deps.Store.GetLatestFishingTrip(c.Context(), userID)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "FISHING_TRIP_FAILED", "load fishing trip failed", err.Error())
	}
	spotCounts, err := s.deps.Store.FishingSpotCounts(c.Context())
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "FISHING_SPOT_COUNTS_FAILED", "load fishing spot activity failed", err.Error())
	}
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{
		"level": level, "anglers": anglers, "waters": waters, "spots": spots, "species": species, "pondSpecies": pondSpecies,
		"tackle": tackle, "ownedTackle": ownedTackle, "forecast": forecast, "mutations": s.deps.Store.FishingMutationCatalog(),
		"currentTrip": trip, "latestTrip": latestTrip,
		"spotCounts":     spotCounts,
		"rulesetVersion": store.FishingRulesetVersion,
	})
}

func (s *Server) handleFishingTripStop(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	ctx, cancel := context.WithTimeout(c.Context(), actionCommandTimeout)
	defer cancel()
	if !s.hasOfflineFastForwardReady(userID) {
		advance, advanceErr := s.actionRuntime().AdvanceDue(ctx, userID)
		if advanceErr != nil {
			return s.writeAPIError(c, fiber.StatusInternalServerError, "FISHING_PROGRESS_FAILED", "update fishing trip failed", advanceErr.Error())
		}
		s.syncActionRuntime(userID, advance.Specs, advance.Queue, advance.State, advance.Attr, advance.Summary)
	}
	trip, err := s.deps.Store.GetCurrentFishingTrip(ctx, userID)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "FISHING_TRIP_FAILED", "load fishing trip failed", err.Error())
	}
	if trip == nil {
		return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{"ok": true, "stopped": false})
	}
	payload, err := json.Marshal(fiber.Map{
		"queueId": trip.QueueID, "actionId": store.FishingTripActionID,
		"actorType": trip.ActorType, "actorId": trip.ActorID,
	})
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "FISHING_STOP_FAILED", "stop fishing trip failed", err.Error())
	}
	result, actionErr := s.actionRuntime().CancelAction(ctx, userID, payload)
	if actionErr != nil {
		// A trip may outlive an action row after a deploy or an older client flow.
		// Keep the dedicated stop endpoint usable without asking the client to know
		// queue internals, while preserving the ordinary cancellation path above.
		fallbackCtx, fallbackCancel := context.WithTimeout(c.Context(), actionCommandTimeout)
		defer fallbackCancel()
		stopped, stopErr := s.deps.Store.StopStaleFishingTrip(fallbackCtx, userID)
		if stopErr != nil {
			return s.writeFishingError(c, stopErr, "stop fishing trip failed")
		}
		if !stopped {
			remaining, lookupErr := s.deps.Store.GetCurrentFishingTrip(fallbackCtx, userID)
			if lookupErr != nil {
				return s.writeFishingError(c, lookupErr, "stop fishing trip failed")
			}
			if remaining != nil {
				return s.writeAPIError(c, actionErrorHTTPStatus(actionErr.Code), actionErr.Code, actionErr.Message, actionErr.Debug, actionErr.InstanceState)
			}
			return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{"ok": true, "stopped": true})
		}
		advance, advanceErr := s.actionRuntime().AdvanceDue(fallbackCtx, userID)
		if advanceErr != nil {
			return s.writeAPIError(c, fiber.StatusInternalServerError, "FISHING_PROGRESS_FAILED", "continue action queue failed", advanceErr.Error())
		}
		advance.Summary.FishingChanged = true
		s.syncActionRuntime(userID, advance.Specs, advance.Queue, advance.State, advance.Attr, advance.Summary)
		return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{"ok": true, "stopped": true})
	}
	result.Summary.FishingChanged = true
	s.syncActionRuntime(userID, result.Specs, result.Queue, result.State, result.Attr, result.Summary)
	remaining, err := s.deps.Store.GetCurrentFishingTrip(ctx, userID)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "FISHING_TRIP_FAILED", "load fishing trip failed", err.Error())
	}
	if remaining != nil && remaining.TripID == trip.TripID {
		stopped, stopErr := s.deps.Store.StopStaleFishingTrip(ctx, userID)
		if stopErr != nil {
			return s.writeFishingError(c, stopErr, "stop fishing trip failed")
		}
		if stopped {
			advance, advanceErr := s.actionRuntime().AdvanceDue(ctx, userID)
			if advanceErr != nil {
				return s.writeAPIError(c, fiber.StatusInternalServerError, "FISHING_PROGRESS_FAILED", "continue action queue failed", advanceErr.Error())
			}
			advance.Summary.FishingChanged = true
			s.syncActionRuntime(userID, advance.Specs, advance.Queue, advance.State, advance.Attr, advance.Summary)
		}
	}
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{"ok": true, "stopped": true})
}

func (s *Server) handleFishingTripHistory(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	items, err := s.deps.Store.ListRecentFishingTrips(c.Context(), userID)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "FISHING_TRIP_HISTORY_FAILED", "load fishing trip history failed", err.Error())
	}
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{
		"items": items,
		"limit": store.FishingTripHistoryLimit,
	})
}

func (s *Server) handleFishingTripEvents(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	cursor := int64(0)
	if raw := strings.TrimSpace(c.Query("cursor")); raw != "" {
		cursor, err = strconv.ParseInt(raw, 10, 64)
		if err != nil || cursor < 0 {
			return s.writeFishingError(c, store.ErrFishingArgsInvalid, "load fishing trip events failed")
		}
	}
	page, err := s.deps.Store.ListFishingTripEvents(c.Context(), userID, c.Params("tripID"), cursor)
	if err != nil {
		return s.writeFishingError(c, err, "load fishing trip events failed")
	}
	return s.writeAPISuccess(c, fiber.StatusOK, page)
}

func (s *Server) handleFishingStarterClaim(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	result, err := s.deps.Store.ClaimFishingStarter(c.Context(), userID)
	if err != nil {
		return s.writeFishingError(c, err, "prepare starter tackle failed")
	}
	if result.Granted || result.EquipmentChanged {
		s.syncAfterMutation(mutationSyncScope{
			UserIDs: []int64{userID}, PlayerState: true, Inventory: true,
			Equipment: true, Attributes: true,
		})
	}
	return s.writeAPISuccess(c, fiber.StatusOK, result)
}

func (s *Server) handleFishingLoadoutsList(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	rows, err := s.deps.Store.ListFishingLoadouts(c.Context(), userID)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "FISHING_LOADOUTS_FAILED", "load fishing tackle presets failed", err.Error())
	}
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{"items": rows})
}

func (s *Server) handleFishingLoadoutSave(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	var input store.FishingLoadoutInput
	if err := c.BodyParser(&input); err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "FISHING_LOADOUT_INVALID", "invalid tackle preset", err.Error())
	}
	row, err := s.deps.Store.SaveFishingLoadout(c.Context(), userID, input)
	if err != nil {
		return s.writeFishingError(c, err, "save tackle preset failed")
	}
	return s.writeAPISuccess(c, fiber.StatusOK, row)
}

func (s *Server) handleFishingLoadoutDelete(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	if err := s.deps.Store.DeleteFishingLoadout(c.Context(), userID, c.Params("loadoutID")); err != nil {
		return s.writeFishingError(c, err, "delete tackle preset failed")
	}
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{"ok": true})
}

func (s *Server) handleFishingCatchesList(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	cursor, _ := strconv.ParseInt(strings.TrimSpace(c.Query("cursor")), 10, 64)
	limit, _ := strconv.Atoi(strings.TrimSpace(c.Query("limit")))
	kept := strings.EqualFold(c.Query("kept"), "true") || c.Query("kept") == "1"
	page, err := s.deps.Store.ListFishingCatches(c.Context(), userID, cursor, limit, c.Query("speciesId"), kept)
	if err != nil {
		return s.writeFishingError(c, err, "load catches failed")
	}
	return s.writeAPISuccess(c, fiber.StatusOK, page)
}

func (s *Server) handleFishingJournal(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	rows, err := s.deps.Store.FishingJournal(c.Context(), userID)
	if err != nil {
		return s.writeFishingError(c, err, "load fishing journal failed")
	}
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{"items": rows})
}

func (s *Server) handleFishingLeaderboard(c *fiber.Ctx) error {
	if _, err := s.authUserID(c); err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	limit, _ := strconv.Atoi(strings.TrimSpace(c.Query("limit")))
	rows, err := s.deps.Store.FishingLeaderboard(c.Context(), c.Query("speciesId"), c.Query("metric"), limit)
	if err != nil {
		return s.writeFishingError(c, err, "load fishing records failed")
	}
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{"items": rows, "rulesetVersion": store.FishingRulesetVersion})
}

func (s *Server) handleFishingPondGet(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	overview, err := s.deps.Store.GetFishingPond(c.Context(), userID, time.Now())
	if err != nil {
		return s.writeFishingError(c, err, "load fishing pond failed")
	}
	return s.writeAPISuccess(c, fiber.StatusOK, overview)
}

func (s *Server) handleFishingPondAvailableFish(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	page, err := s.deps.Store.ListAvailableFishingPondFish(c.Context(), userID, int64(c.QueryInt("cursor", 0)))
	if err != nil {
		return s.writeFishingError(c, err, "load available pond fish failed")
	}
	return s.writeAPISuccess(c, fiber.StatusOK, page)
}

func (s *Server) handleFishingPondFishAdd(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	var request fishingPondFishRequest
	if err := c.BodyParser(&request); err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "FISHING_POND_INVALID", "invalid pond fish", err.Error())
	}
	overview, err := s.deps.Store.AddFishingPondFish(c.Context(), userID, request.InstanceID, time.Now())
	return s.writeFishingPondMutation(c, userID, overview, err, "stock fishing pond failed")
}

func (s *Server) handleFishingPondFishBatchAdd(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	var request fishingPondFishBatchRequest
	if err := c.BodyParser(&request); err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "FISHING_POND_INVALID", "invalid pond fish", err.Error())
	}
	overview, err := s.deps.Store.AddFishingPondFishBatch(c.Context(), userID, request.InstanceIDs, time.Now())
	return s.writeFishingPondMutation(c, userID, overview, err, "stock fishing pond failed")
}

func (s *Server) handleFishingPondFishRemove(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	instanceID, parseErr := strconv.ParseInt(c.Params("instanceID"), 10, 64)
	if parseErr != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "FISHING_POND_INVALID", "invalid pond fish", parseErr.Error())
	}
	overview, err := s.deps.Store.RemoveFishingPondFish(c.Context(), userID, instanceID, time.Now())
	return s.writeFishingPondMutation(c, userID, overview, err, "remove pond fish failed")
}

func (s *Server) handleFishingPondFishMoveToMain(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	instanceID, parseErr := strconv.ParseInt(c.Params("instanceID"), 10, 64)
	if parseErr != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "FISHING_POND_INVALID", "invalid pond fish", parseErr.Error())
	}
	overview, err := s.deps.Store.MoveFishingPondFishToMain(c.Context(), userID, instanceID, time.Now())
	return s.writeFishingPondMutation(c, userID, overview, err, "move nursery fish to main pond failed")
}

func (s *Server) handleFishingPondFeedAdd(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	var request fishingPondFeedRequest
	if err := c.BodyParser(&request); err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "FISHING_POND_INVALID", "invalid fish feed", err.Error())
	}
	overview, err := s.deps.Store.AddFishingPondFeed(c.Context(), userID, request.ItemID, request.Quantity, time.Now())
	return s.writeFishingPondMutation(c, userID, overview, err, "fill pond feeder failed")
}

func (s *Server) handleFishingPondFeedManage(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	var request struct {
		ItemID    string `json:"itemId"`
		Operation string `json:"operation"`
		Quantity  int64  `json:"quantity"`
	}
	if err := c.BodyParser(&request); err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "FISHING_POND_INVALID", "invalid fish feed operation", err.Error())
	}
	overview, err := s.deps.Store.ManageFishingPondFeed(c.Context(), userID, request.ItemID, request.Operation, request.Quantity, time.Now())
	return s.writeFishingPondMutation(c, userID, overview, err, "manage pond feeder failed")
}

func (s *Server) handleFishingPondTargetedFeed(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	instanceID, err := strconv.ParseInt(c.Params("instanceID"), 10, 64)
	var request struct {
		ItemID           string `json:"itemId"`
		ExpectedRevision *int64 `json:"expectedRevision"`
	}
	if err != nil || instanceID <= 0 || c.BodyParser(&request) != nil || request.ExpectedRevision == nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "FISHING_POND_INVALID", "invalid targeted feed", "")
	}
	overview, err := s.deps.Store.UseFishingPondTargetedFeed(c.Context(), userID, instanceID, request.ItemID, *request.ExpectedRevision, time.Now())
	return s.writeFishingPondMutation(c, userID, overview, err, "targeted pond feed failed")
}

func (s *Server) handleFishingPondPairSet(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	var request fishingPondPairRequest
	if err := c.BodyParser(&request); err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "FISHING_POND_INVALID", "invalid breeding pair", err.Error())
	}
	overview, err := s.deps.Store.SetFishingPondPair(c.Context(), userID, request.FemaleInstanceID, request.MaleInstanceID, time.Now())
	return s.writeFishingPondMutation(c, userID, overview, err, "set breeding pair failed")
}

func (s *Server) handleFishingPondPairDelete(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	slot, parseErr := strconv.ParseInt(c.Params("slotIndex"), 10, 64)
	if parseErr != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "FISHING_POND_INVALID", "invalid breeding pair", parseErr.Error())
	}
	overview, err := s.deps.Store.DeleteFishingPondPair(c.Context(), userID, slot, time.Now())
	return s.writeFishingPondMutation(c, userID, overview, err, "remove breeding pair failed")
}

func (s *Server) handleFishingPondMashApply(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	slot, parseErr := strconv.ParseInt(c.Params("slotIndex"), 10, 64)
	if parseErr != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "FISHING_POND_INVALID", "invalid breeding pair", parseErr.Error())
	}
	overview, err := s.deps.Store.ApplyFishingPondMash(c.Context(), userID, slot, time.Now())
	return s.writeFishingPondMutation(c, userID, overview, err, "apply breeding feed failed")
}

func (s *Server) handleFishingPondGenomeReveal(c *fiber.Ctx) error {
	return s.handleFishingPondFishAction(c, "reveal")
}

func (s *Server) handleFishingPondFishProcess(c *fiber.Ctx) error {
	return s.handleFishingPondFishAction(c, "process")
}

func (s *Server) handleFishingPondFishAction(c *fiber.Ctx, action string) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	instanceID, parseErr := strconv.ParseInt(c.Params("instanceID"), 10, 64)
	if parseErr != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "FISHING_POND_INVALID", "invalid pond fish", parseErr.Error())
	}
	var overview store.FishingPondOverview
	if action == "reveal" {
		overview, err = s.deps.Store.RevealFishingPondGenome(c.Context(), userID, instanceID, time.Now())
	} else {
		overview, err = s.deps.Store.ProcessFishingPondFish(c.Context(), userID, instanceID, time.Now())
	}
	return s.writeFishingPondMutation(c, userID, overview, err, "update pond fish failed")
}

func (s *Server) writeFishingPondMutation(c *fiber.Ctx, userID int64, overview store.FishingPondOverview, err error, fallback string) error {
	if err != nil {
		return s.writeFishingError(c, err, fallback)
	}
	s.syncAfterMutation(mutationSyncScope{UserIDs: []int64{userID}, Inventory: true})
	return s.writeAPISuccess(c, fiber.StatusOK, overview)
}

func (s *Server) handleFishingSaleQuote(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	var request fishingSaleRequest
	if err := c.BodyParser(&request); err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "FISHING_SALE_INVALID", "invalid fish sale", err.Error())
	}
	quote, err := s.deps.Store.QuoteFishingSale(c.Context(), userID, request.InstanceIDs, time.Now())
	if err != nil {
		return s.writeFishingError(c, err, "quote fish sale failed")
	}
	return s.writeAPISuccess(c, fiber.StatusOK, quote)
}

func (s *Server) handleFishingSale(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	var request fishingSaleRequest
	if err := c.BodyParser(&request); err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "FISHING_SALE_INVALID", "invalid fish sale", err.Error())
	}
	quote, err := s.deps.Store.SellFishingCatches(c.Context(), userID, request.InstanceIDs, request.ExpectedTotalGold, request.IdempotencyKey, time.Now())
	if err != nil {
		return s.writeFishingError(c, err, "sell fish failed")
	}
	s.syncAfterMutation(mutationSyncScope{UserIDs: []int64{userID}, Inventory: true})
	return s.writeAPISuccess(c, fiber.StatusOK, quote)
}

func (s *Server) handleFishingTournaments(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	overview, err := s.deps.Store.GetFishingTournamentOverview(c.Context(), userID, time.Now())
	if err != nil {
		return s.writeFishingError(c, err, "load fishing tournament failed")
	}
	return s.writeAPISuccess(c, fiber.StatusOK, overview)
}

func (s *Server) handleFishingPondTournamentSubmit(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	var request fishingPondFishRequest
	if err := c.BodyParser(&request); err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "FISHING_TOURNAMENT_INVALID", "invalid tournament fish", err.Error())
	}
	entry, err := s.deps.Store.SubmitFishingPondTournamentFish(c.Context(), userID, request.InstanceID, time.Now())
	if err != nil {
		return s.writeFishingError(c, err, "submit cultured fish failed")
	}
	return s.writeAPISuccess(c, fiber.StatusOK, entry)
}

func (s *Server) handleFishingTournamentRewardClaim(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	var request fishingTournamentClaimRequest
	if err := c.BodyParser(&request); err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "FISHING_REWARD_INVALID", "invalid reward", err.Error())
	}
	reward, err := s.deps.Store.ClaimFishingTournamentReward(c.Context(), userID, request.TournamentID, request.Division)
	if err != nil {
		return s.writeFishingError(c, err, "claim fishing reward failed")
	}
	s.syncAfterMutation(mutationSyncScope{
		UserIDs:   []int64{userID},
		Inventory: reward.ItemID != "",
		Wardrobe:  reward.CosmeticKey != "",
	})
	return s.writeAPISuccess(c, fiber.StatusOK, reward)
}

func (s *Server) handleFishingTournamentCatchUpRedeem(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	if err := s.deps.Store.RedeemFishingCatchUpRod(c.Context(), userID, time.Now()); err != nil {
		return s.writeFishingError(c, err, "redeem fishing tackle failed")
	}
	s.syncAfterMutation(mutationSyncScope{UserIDs: []int64{userID}, Inventory: true})
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{"ok": true})
}

func (s *Server) writeFishingError(c *fiber.Ctx, err error, fallback string) error {
	var validation *store.FishingValidationError
	switch {
	case errors.As(err, &validation):
		return s.writeAPIError(c, fiber.StatusBadRequest, validation.Code(), validation.Error(), "")
	case errors.Is(err, store.ErrFishingLoadoutNotFound), errors.Is(err, store.ErrFishingSpotNotFound), errors.Is(err, store.ErrFishingTripNotFound), errors.Is(err, store.ErrFishingCatchNotFound):
		return s.writeAPIError(c, fiber.StatusNotFound, "FISHING_NOT_FOUND", fallback, err.Error())
	case errors.Is(err, store.ErrFishingArgsInvalid), errors.Is(err, store.ErrFishingLoadoutInvalid), errors.Is(err, store.ErrFishingSaleInvalid):
		return s.writeAPIError(c, fiber.StatusBadRequest, "FISHING_INVALID", fallback, err.Error())
	case errors.Is(err, store.ErrFishingPondFishInvalid), errors.Is(err, store.ErrFishingPondFeedInvalid), errors.Is(err, store.ErrFishingPondPairInvalid):
		return s.writeAPIError(c, fiber.StatusBadRequest, "FISHING_POND_INVALID", fallback, err.Error())
	case errors.Is(err, store.ErrFishingSaleQuoteChanged):
		return s.writeAPIError(c, fiber.StatusConflict, "FISHING_SALE_QUOTE_CHANGED", fallback, err.Error())
	case errors.Is(err, store.ErrFishingSaleUnavailable):
		return s.writeAPIError(c, fiber.StatusLocked, "FISHING_SALE_UNAVAILABLE", fallback, err.Error())
	case errors.Is(err, store.ErrFishingPondLineageReagentMissing):
		return s.writeAPIError(c, fiber.StatusConflict, "FISHING_POND_REAGENT_MISSING", fallback, err.Error())
	case errors.Is(err, store.ErrFishingPondBroodstockMashMissing):
		return s.writeAPIError(c, fiber.StatusConflict, "FISHING_POND_MASH_MISSING", fallback, err.Error())
	case errors.Is(err, store.ErrFishingWaterLocked), errors.Is(err, store.ErrFishingTackleMissing), errors.Is(err, store.ErrFishingTackleInUse), errors.Is(err, store.ErrInsufficientItem), errors.Is(err, store.ErrFishingRewardUnavailable), errors.Is(err, store.ErrFishingTournamentEntryUnavailable), errors.Is(err, store.ErrStatusRequireNotMet), errors.Is(err, store.ErrFishingPondNotBuilt), errors.Is(err, store.ErrFishingPondFull), errors.Is(err, store.ErrFishingPondBiomassFull), errors.Is(err, store.ErrFishingPondFeederFull), errors.Is(err, store.ErrFishingPondPairFull):
		return s.writeAPIError(c, fiber.StatusConflict, "FISHING_UNAVAILABLE", fallback, err.Error())
	default:
		return s.writeAPIError(c, fiber.StatusInternalServerError, "FISHING_FAILED", fallback, err.Error())
	}
}

func (s *Server) pushFishingState(userID int64) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	trip, err := s.deps.Store.GetCurrentFishingTrip(ctx, userID)
	if err != nil {
		return
	}
	if trip == nil {
		trip, err = s.deps.Store.GetLatestFishingTrip(ctx, userID)
		if err != nil {
			return
		}
	}
	s.pushUserEventClusterOnce(userID, "game.fishing.trip", fiber.Map{"trip": trip})
}
