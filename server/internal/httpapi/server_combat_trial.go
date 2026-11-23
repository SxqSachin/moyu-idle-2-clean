package httpapi

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"mysecondwebgame/server/internal/store"

	"github.com/gofiber/fiber/v2"
)

const (
	battleKindCombatTrial      = "combat_trial"
	combatTrialDefaultFormatID = "standard"
)

type combatTrialLineupSelection struct {
	PlayerPosition string                            `json:"playerPosition"`
	Kitties        []competitiveLineupKittySelection `json:"kitties"`
	FormatID       string                            `json:"formatId"`
}

type combatTrialCreateRunRequest struct {
	Selection *combatTrialLineupSelection `json:"selection,omitempty"`
}

type combatTrialRunFormatSummary struct {
	ID               string                              `json:"id"`
	NameKey          string                              `json:"nameKey"`
	ShortDescKey     string                              `json:"shortDescKey"`
	Icon             string                              `json:"icon"`
	DefinitionDigest string                              `json:"definitionDigest"`
	ActionBudget     int                                 `json:"actionBudget"`
	Metrics          []combatTrialMetricDefinition       `json:"metrics"`
	EndConditions    []combatTrialEndConditionDefinition `json:"endConditions"`
}

type combatTrialPublicRunPayload struct {
	store.CombatTrialRun
	Format *combatTrialRunFormatSummary `json:"format,omitempty"`
}

func defaultCombatTrialSelection() combatTrialLineupSelection {
	return combatTrialLineupSelection{PlayerPosition: "vanguard", Kitties: []competitiveLineupKittySelection{}, FormatID: combatTrialDefaultFormatID}
}

func combatTrialPublicRun(run store.CombatTrialRun) combatTrialPublicRunPayload {
	var summary *combatTrialRunFormatSummary
	if format, err := decodeCombatTrialFormatSnapshot(run.FormatSnapshot); err == nil {
		summary = &combatTrialRunFormatSummary{
			ID: format.ID, NameKey: format.NameKey, ShortDescKey: format.ShortDescKey, Icon: format.Icon,
			DefinitionDigest: format.DefinitionDigest, ActionBudget: combatTrialPlayerMainActionBudget(format),
			Metrics:       append([]combatTrialMetricDefinition(nil), format.Metrics...),
			EndConditions: append([]combatTrialEndConditionDefinition(nil), format.EndConditions...),
		}
	}
	run.Lineup = nil
	run.LineupDigest = ""
	run.FormatSnapshot = nil
	return combatTrialPublicRunPayload{CombatTrialRun: run, Format: summary}
}

func combatTrialPublicLeaderboardEntry(entry store.CombatTrialLeaderboardEntry, publicMetrics map[string]combatTrialMetricDefinition) fiber.Map {
	metrics := make(map[string]float64, len(entry.Metrics))
	for metricID, value := range entry.Metrics {
		metric, allowed := publicMetrics[metricID]
		if allowed && combatTrialMetricEndReasonEligible(metric, entry.EndReason) {
			metrics[metricID] = value
		}
	}
	return fiber.Map{
		"rank":       entry.Rank,
		"username":   entry.Username,
		"metricId":   entry.MetricID,
		"value":      entry.Value,
		"metrics":    metrics,
		"kittyCount": entry.KittyCount,
		"achievedAt": entry.AchievedAt,
	}
}

func normalizeCombatTrialSelection(selection combatTrialLineupSelection) (combatTrialLineupSelection, error) {
	selection.FormatID = strings.ToLower(strings.TrimSpace(selection.FormatID))
	if selection.FormatID == "" {
		selection.FormatID = combatTrialDefaultFormatID
	}
	normalized, err := normalizeCompetitiveLineupSelection(competitiveLineupSelection{
		PlayerPosition: selection.PlayerPosition,
		Kitties:        append([]competitiveLineupKittySelection(nil), selection.Kitties...),
	})
	if err != nil {
		return combatTrialLineupSelection{}, err
	}
	return combatTrialLineupSelection{
		PlayerPosition: normalized.PlayerPosition,
		Kitties:        normalized.Kitties,
		FormatID:       selection.FormatID,
	}, nil
}

func validateCombatTrialSelectionForFormat(selection combatTrialLineupSelection, format *combatTrialFormatDefinition) error {
	if format == nil {
		return store.ErrBattleRoomConfigInvalid
	}
	if format.LineupRules == nil || format.LineupRules.PositionPolicy != "distinct" {
		return nil
	}
	seenPositions := map[string]struct{}{selection.PlayerPosition: {}}
	for _, kitty := range selection.Kitties {
		if _, occupied := seenPositions[kitty.Position]; occupied {
			return store.ErrBattleRoomConfigInvalid
		}
		seenPositions[kitty.Position] = struct{}{}
	}
	return nil
}

func (s *Server) buildCombatTrialLineupSnapshot(ctx *fiber.Ctx, userID int64, selection combatTrialLineupSelection, format *combatTrialFormatDefinition, now time.Time) (combatTrialLineupSelection, []battleUnitState, error) {
	selection, err := normalizeCombatTrialSelection(selection)
	if err != nil {
		return combatTrialLineupSelection{}, nil, err
	}
	resolved, lineup, err := s.buildCompetitiveLineupSnapshot(ctx.Context(), userID, competitiveLineupSelection{
		PlayerPosition: selection.PlayerPosition,
		Kitties:        append([]competitiveLineupKittySelection(nil), selection.Kitties...),
	}, now, competitiveLineupSnapshotOptions{SpecializationEnabled: true})
	if err != nil {
		return combatTrialLineupSelection{}, nil, err
	}
	resolvedTrial, err := normalizeCombatTrialSelection(combatTrialLineupSelection{
		PlayerPosition: resolved.PlayerPosition,
		Kitties:        resolved.Kitties,
		FormatID:       selection.FormatID,
	})
	if err != nil {
		return combatTrialLineupSelection{}, nil, err
	}
	if err := validateCombatTrialSelectionForFormat(resolvedTrial, format); err != nil {
		return combatTrialLineupSelection{}, nil, err
	}
	return resolvedTrial, lineup, nil
}

func (s *Server) combatTrialKittyPayload(ctx *fiber.Ctx, userID int64) ([]fiber.Map, error) {
	kitties, err := s.deps.Store.ListPlayerKitties(ctx.Context(), userID)
	if err != nil {
		return nil, err
	}
	out := make([]fiber.Map, 0, len(kitties))
	for _, kitty := range kitties {
		position, personalityID, personalityKey, err := s.deps.Store.GetOwnedKittyPositionLock(ctx.Context(), userID, kitty.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, fiber.Map{
			"id": kitty.ID, "name": kitty.Name, "level": kitty.Level, "status": kitty.Status,
			"positionLocked": position, "positionLockedByPersonalityId": personalityID, "positionLockedByPersonalityKey": personalityKey,
		})
	}
	return out, nil
}

func (s *Server) handleCombatTrialOverview(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	if !s.deps.Config.GlobalSettings.CombatTrial.Enabled {
		return s.writeAPIError(c, fiber.StatusNotFound, "COMBAT_TRIAL_DISABLED", "combat trial is disabled", "")
	}
	formats, err := s.combatTrialPublicFormats()
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "COMBAT_TRIAL_OVERVIEW_FAILED", "load combat trial formats failed", err.Error())
	}
	selection := defaultCombatTrialSelection()
	kitties, err := s.combatTrialKittyPayload(c, userID)
	if err != nil {
		return s.writeCombatTrialError(c, "COMBAT_TRIAL_OVERVIEW_FAILED", err)
	}
	currentFormat := formats[0]
	requestedFormatID := strings.ToLower(strings.TrimSpace(c.Query("formatId")))
	currentFormatID := selection.FormatID
	if requestedFormatID != "" {
		currentFormatID = requestedFormatID
	}
	formatAvailable := false
	for index := range formats {
		if formats[index].ID == currentFormatID {
			currentFormat = formats[index]
			formatAvailable = true
			break
		}
	}
	if !formatAvailable && requestedFormatID != "" {
		return s.writeAPIError(c, fiber.StatusBadRequest, "COMBAT_TRIAL_FORMAT_INVALID", "combat trial format is unavailable", "")
	}
	selection.FormatID = currentFormat.ID
	bests, err := s.deps.Store.ListCombatTrialPersonalBests(c.Context(), userID, currentFormat.DefinitionDigest)
	if err != nil {
		return s.writeCombatTrialError(c, "COMBAT_TRIAL_OVERVIEW_FAILED", err)
	}
	var activeRun any
	if run, loadErr := s.deps.Store.GetActiveCombatTrialRun(c.Context(), userID); loadErr == nil {
		activeRun = combatTrialPublicRun(run)
	} else if !errors.Is(loadErr, store.ErrCombatTrialRunNotFound) {
		return s.writeCombatTrialError(c, "COMBAT_TRIAL_OVERVIEW_FAILED", loadErr)
	}
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{
		"battleKind":    battleKindCombatTrial,
		"selection":     selection,
		"kitties":       kitties,
		"formats":       formats,
		"personalBests": bests,
		"activeRun":     activeRun,
		"rules": fiber.Map{
			"historyKeepCount": s.deps.Config.GlobalSettings.CombatTrial.HistoryKeepCount,
			"ranked":           true,
			"unlimitedRuns":    true,
			"maxKitties":       store.CompetitiveMaxKittyCount,
		},
	})
}

func (s *Server) handleCombatTrialRunCreate(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	if !s.deps.Config.GlobalSettings.CombatTrial.Enabled {
		return s.writeAPIError(c, fiber.StatusNotFound, "COMBAT_TRIAL_DISABLED", "combat trial is disabled", "")
	}
	request := combatTrialCreateRunRequest{}
	if len(c.Body()) > 0 {
		if err := c.BodyParser(&request); err != nil {
			return s.writeAPIError(c, fiber.StatusBadRequest, "COMBAT_TRIAL_RUN_INVALID", "invalid combat trial run", err.Error())
		}
	}
	selection := defaultCombatTrialSelection()
	if request.Selection != nil {
		selection, err = normalizeCombatTrialSelection(*request.Selection)
		if err != nil {
			return s.writeCombatTrialError(c, "COMBAT_TRIAL_RUN_CREATE_FAILED", err)
		}
	}
	selection, err = normalizeCombatTrialSelection(selection)
	if err != nil {
		return s.writeCombatTrialError(c, "COMBAT_TRIAL_RUN_CREATE_FAILED", err)
	}
	format, err := s.resolveCombatTrialFormat(selection.FormatID)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "COMBAT_TRIAL_FORMAT_INVALID", "combat trial format is unavailable", "")
	}
	selection, lineup, err := s.buildCombatTrialLineupSnapshot(c, userID, selection, format, time.Now())
	if err != nil {
		return s.writeCombatTrialError(c, "COMBAT_TRIAL_RUN_CREATE_FAILED", err)
	}
	lineupDigest, lineupRaw, err := competitiveLineupDigest(lineup)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "COMBAT_TRIAL_RUN_CREATE_FAILED", "create combat trial run failed", err.Error())
	}
	formatRaw, err := json.Marshal(format)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "COMBAT_TRIAL_RUN_CREATE_FAILED", "create combat trial run failed", err.Error())
	}
	seed, err := randomCompetitiveBattleSeed()
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "COMBAT_TRIAL_RUN_CREATE_FAILED", "create combat trial run failed", err.Error())
	}
	run, err := s.deps.Store.CreateCombatTrialRun(c.Context(), store.CreateCombatTrialRunInput{
		UserID: userID, FormatID: format.ID, FormatDefinitionDigest: format.DefinitionDigest, Seed: seed,
		Lineup: lineupRaw, LineupDigest: lineupDigest, FormatSnapshot: formatRaw,
		ActionBudget: format.ActionBudget, KittyCount: len(selection.Kitties),
	})
	if err != nil {
		return s.writeCombatTrialError(c, "COMBAT_TRIAL_RUN_CREATE_FAILED", err)
	}
	return s.writeAPISuccess(c, fiber.StatusAccepted, fiber.Map{"battleKind": battleKindCombatTrial, "run": combatTrialPublicRun(run)})
}

func (s *Server) handleCombatTrialRuns(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	offset, _ := strconv.Atoi(c.Query("offset", "0"))
	runs, err := s.deps.Store.ListCombatTrialRuns(c.Context(), userID, s.deps.Config.GlobalSettings.CombatTrial.HistoryKeepCount, offset)
	if err != nil {
		return s.writeCombatTrialError(c, "COMBAT_TRIAL_HISTORY_FAILED", err)
	}
	publicRuns := make([]combatTrialPublicRunPayload, 0, len(runs))
	for _, run := range runs {
		publicRuns = append(publicRuns, combatTrialPublicRun(run))
	}
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{"battleKind": battleKindCombatTrial, "runs": publicRuns, "historyKeepCount": s.deps.Config.GlobalSettings.CombatTrial.HistoryKeepCount})
}

func (s *Server) handleCombatTrialLeaderboard(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	format, err := s.resolveCombatTrialFormat(c.Query("formatId", combatTrialDefaultFormatID))
	if err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "COMBAT_TRIAL_FORMAT_INVALID", "combat trial format is unavailable", "")
	}
	metricID := strings.TrimSpace(c.Query("metricId"))
	if metricID == "" {
		for _, metric := range format.Metrics {
			if metric.Ranked {
				metricID = metric.ID
				break
			}
		}
	}
	var selectedMetric *combatTrialMetricDefinition
	for _, metric := range format.Metrics {
		if metric.ID == metricID && metric.Ranked {
			copy := metric
			selectedMetric = &copy
			break
		}
	}
	if selectedMetric == nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "COMBAT_TRIAL_METRIC_INVALID", "combat trial metric is unavailable", "")
	}
	page, err := s.deps.Store.ListCombatTrialLeaderboard(c.Context(), userID, format.DefinitionDigest, metricID, selectedMetric.RankDirection, store.CombatTrialLeaderboardLimit)
	if err != nil {
		return s.writeCombatTrialError(c, "COMBAT_TRIAL_LEADERBOARD_FAILED", err)
	}
	entries := make([]fiber.Map, 0, len(page.Entries))
	publicMetrics := make(map[string]combatTrialMetricDefinition, len(format.Metrics))
	for _, metric := range format.Metrics {
		if metric.Ranked {
			publicMetrics[metric.ID] = metric
		}
	}
	for _, entry := range page.Entries {
		entries = append(entries, combatTrialPublicLeaderboardEntry(entry, publicMetrics))
	}
	var self any
	if page.Self != nil {
		self = combatTrialPublicLeaderboardEntry(*page.Self, publicMetrics)
	}
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{
		"battleKind": battleKindCombatTrial, "format": combatTrialPublicFormat(*format), "metricId": metricID,
		"entries": entries, "self": self, "total": page.Total, "limit": store.CombatTrialLeaderboardLimit,
	})
}

func parseCombatTrialRunID(c *fiber.Ctx) (int64, error) {
	return strconv.ParseInt(strings.TrimSpace(c.Params("runID")), 10, 64)
}

func (s *Server) handleCombatTrialRunReplay(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	runID, err := parseCombatTrialRunID(c)
	if err != nil || runID <= 0 {
		return s.writeAPIError(c, fiber.StatusBadRequest, "COMBAT_TRIAL_RUN_INVALID", "invalid combat trial run", "")
	}
	payload, err := s.deps.Store.GetCombatTrialRunReplay(c.Context(), userID, runID)
	if err != nil {
		return s.writeCombatTrialError(c, "COMBAT_TRIAL_REPLAY_FAILED", err)
	}
	return s.writeAPISuccess(c, fiber.StatusOK, payload)
}

func (s *Server) handleCombatTrialRunShare(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	runID, err := parseCombatTrialRunID(c)
	if err != nil || runID <= 0 {
		return s.writeAPIError(c, fiber.StatusBadRequest, "COMBAT_TRIAL_RUN_INVALID", "invalid combat trial run", "")
	}
	token, err := randomCompetitiveReplayShareToken()
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "COMBAT_TRIAL_REPLAY_SHARE_FAILED", "share combat trial replay failed", err.Error())
	}
	share, err := s.deps.Store.CreateCombatTrialReplayShare(c.Context(), userID, runID, token)
	if err != nil {
		return s.writeCombatTrialError(c, "COMBAT_TRIAL_REPLAY_SHARE_FAILED", err)
	}
	return s.writeAPISuccess(c, fiber.StatusCreated, fiber.Map{"battleKind": battleKindCombatTrial, "share": share})
}

func (s *Server) handleCombatTrialSharedReplay(c *fiber.Ctx) error {
	if _, err := s.authUserID(c); err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	shared, err := s.deps.Store.GetCombatTrialSharedReplay(c.Context(), c.Params("token"))
	if err != nil {
		return s.writeCombatTrialError(c, "COMBAT_TRIAL_REPLAY_SHARED_FAILED", err)
	}
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{"battleKind": battleKindCombatTrial, "share": shared.Share, "payload": shared.Payload})
}

func (s *Server) writeCombatTrialError(c *fiber.Ctx, fallbackCode string, err error) error {
	var eligibilityErr *competitiveAlchemyEligibilityError
	if errors.As(err, &eligibilityErr) {
		return s.writeAPIErrorWithData(c, fiber.StatusConflict, "COMBAT_TRIAL_ALCHEMY_EQUIPMENT_RESTRICTED", "combat trial lineup contains alchemy equipment above the league limit", "", fiber.Map{
			"policyVersion":                 competitiveAlchemyEligibilityPolicyVersion,
			"requiredAlchemyRulesetVersion": store.AlchemyCurrentRulesetVersion,
			"violations":                    eligibilityErr.Violations,
		})
	}
	switch {
	case errors.Is(err, store.ErrCombatTrialRunActive):
		return s.writeAPIError(c, fiber.StatusConflict, "COMBAT_TRIAL_RUN_ACTIVE", "combat trial run already active", "")
	case errors.Is(err, store.ErrCombatTrialRunNotFound):
		return s.writeAPIError(c, fiber.StatusNotFound, "COMBAT_TRIAL_RUN_NOT_FOUND", "combat trial run not found", "")
	case errors.Is(err, store.ErrCombatTrialReplayUnavailable):
		return s.writeAPIError(c, fiber.StatusGone, "COMBAT_TRIAL_REPLAY_EXPIRED", "combat trial replay is no longer retained", "")
	case errors.Is(err, errCompetitiveEquipmentSnapshotUnstable):
		return s.writeAPIError(c, fiber.StatusConflict, "COMBAT_TRIAL_LINEUP_CHANGED", "equipment changed while the combat trial lineup was being prepared", "")
	case errors.Is(err, store.ErrKittyNotFound):
		return s.writeAPIError(c, fiber.StatusNotFound, "COMBAT_TRIAL_KITTY_NOT_FOUND", "combat trial kitty not found", "")
	case errors.Is(err, store.ErrBattleRoomConfigInvalid):
		return s.writeAPIError(c, fiber.StatusBadRequest, "COMBAT_TRIAL_LINEUP_INVALID", "invalid combat trial lineup", "")
	default:
		return s.writeAPIError(c, fiber.StatusInternalServerError, fallbackCode, "combat trial request failed", err.Error())
	}
}
