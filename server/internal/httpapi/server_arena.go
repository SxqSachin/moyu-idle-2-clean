package httpapi

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"sort"
	"strconv"
	"strings"
	"time"

	"mysecondwebgame/server/internal/config"
	"mysecondwebgame/server/internal/store"

	"github.com/gofiber/fiber/v2"
)

const (
	arenaLeaderboardLimit       = 25
	arenaMoreTargetsLimit       = 100
	arenaMoreTargetsPageSize    = 20
	arenaRecommendationTokenTTL = 30 * time.Minute
	// Bump whenever published defenses must pass a new eligibility evaluator.
	arenaAlchemyEligibilityPolicyVersion = competitiveAlchemyEligibilityPolicyVersion

	arenaRecommendationTierNearby  = "nearby"
	arenaRecommendationTierRelaxed = "relaxed"
	arenaRecommendationTierRandom  = "random"
)

type arenaLineupKittySelection = competitiveLineupKittySelection

type arenaLineupSelection struct {
	PlayerPosition           string                      `json:"playerPosition"`
	Kitties                  []arenaLineupKittySelection `json:"kitties"`
	FormatID                 string                      `json:"formatId"`
	FormatSnapshot           *arenaFormatDefinition      `json:"formatSnapshot,omitempty"`
	EligibilityPolicyVersion string                      `json:"eligibilityPolicyVersion,omitempty"`
}

type arenaPublishDefenseRequest struct {
	Selection arenaLineupSelection `json:"selection"`
}

type arenaCreateChallengeRequest struct {
	DefenderID          int64  `json:"defenderId"`
	RecommendationToken string `json:"recommendationToken"`
}

type arenaRecommendedTarget struct {
	Target store.ArenaTarget
	Tier   string
}

type arenaShopBuyRequest struct {
	OfferID string `json:"offerId"`
}

func normalizeArenaPosition(raw string) string {
	return normalizeCompetitivePosition(raw)
}

func normalizeArenaSelection(selection arenaLineupSelection) (arenaLineupSelection, error) {
	selection.FormatID = normalizeArenaFormatID(selection.FormatID)
	if selection.FormatID == "" {
		return arenaLineupSelection{}, store.ErrBattleRoomConfigInvalid
	}
	base, err := normalizeCompetitiveLineupSelection(competitiveLineupSelection{
		PlayerPosition: selection.PlayerPosition,
		Kitties:        append([]competitiveLineupKittySelection(nil), selection.Kitties...),
	})
	if err != nil {
		return arenaLineupSelection{}, err
	}
	selection.PlayerPosition = base.PlayerPosition
	selection.Kitties = base.Kitties
	return selection, nil
}

func arenaSelectionEligibilityPolicyCurrent(selection arenaLineupSelection) bool {
	return strings.TrimSpace(selection.EligibilityPolicyVersion) == arenaAlchemyEligibilityPolicyVersion
}

func arenaDefenseEligibilityPolicyCurrent(defense store.ArenaDefense) bool {
	var selection arenaLineupSelection
	return json.Unmarshal(defense.Selection, &selection) == nil && arenaSelectionEligibilityPolicyCurrent(selection)
}

func filterArenaTargetsByEligibilityPolicy(rows []store.ArenaTarget) []store.ArenaTarget {
	filtered := make([]store.ArenaTarget, 0, len(rows))
	for _, row := range rows {
		if arenaDefenseEligibilityPolicyCurrent(row.ArenaDefense) {
			filtered = append(filtered, row)
		}
	}
	return filtered
}

func (s *Server) buildArenaLineupSnapshot(ctx context.Context, userID int64, selection arenaLineupSelection, now time.Time, format *arenaFormatDefinition) (arenaLineupSelection, []battleUnitState, error) {
	selection, err := normalizeArenaSelection(selection)
	if err != nil {
		return arenaLineupSelection{}, nil, err
	}
	resolved, lineup, err := s.buildCompetitiveLineupSnapshot(ctx, userID, competitiveLineupSelection{
		PlayerPosition: selection.PlayerPosition,
		Kitties:        append([]competitiveLineupKittySelection(nil), selection.Kitties...),
	}, now, competitiveLineupSnapshotOptions{SpecializationEnabled: arenaFormatSpecializationEnabled(format)})
	if err != nil {
		return arenaLineupSelection{}, nil, err
	}
	selection.PlayerPosition = resolved.PlayerPosition
	selection.Kitties = resolved.Kitties
	selection.EligibilityPolicyVersion = arenaAlchemyEligibilityPolicyVersion
	return selection, lineup, nil
}

func arenaLineupDigest(lineup []battleUnitState) (string, json.RawMessage, error) {
	return competitiveLineupDigest(lineup)
}

func randomArenaSeed() (int64, error) {
	return randomCompetitiveBattleSeed()
}

func randomArenaReplayShareToken() (string, error) {
	return randomCompetitiveReplayShareToken()
}

func (s *Server) arenaDefensePayload(defense store.ArenaDefense) fiber.Map {
	var selection arenaLineupSelection
	if json.Unmarshal(defense.Selection, &selection) == nil {
		if normalized, err := normalizeArenaSelection(selection); err == nil {
			selection = normalized
		}
	}
	format, _ := arenaFormatFromSelection(selection)
	return fiber.Map{
		"userId":                   defense.UserID,
		"revision":                 defense.Revision,
		"active":                   defense.Active,
		"battleLevel":              defense.BattleLevel,
		"selection":                arenaPublicSelection(selection),
		"format":                   arenaPublicFormatFromSelection(selection),
		"lineup":                   s.arenaPublicLineup(defense.Lineup, format),
		"lineupDigest":             defense.LineupDigest,
		"bountyTokens":             defense.BountyTokens,
		"successfulDefenses":       defense.SuccessfulDefenses,
		"challengeCount":           defense.ChallengeCount,
		"publishedAt":              defense.PublishedAt,
		"updatedAt":                defense.UpdatedAt,
		"eligibilityPolicyVersion": strings.TrimSpace(selection.EligibilityPolicyVersion),
		"eligibilityPolicyCurrent": arenaSelectionEligibilityPolicyCurrent(selection),
	}
}

func arenaPublicSelection(selection arenaLineupSelection) fiber.Map {
	return fiber.Map{
		"playerPosition": selection.PlayerPosition,
		"kitties":        selection.Kitties,
		"formatId":       normalizeArenaFormatID(selection.FormatID),
	}
}

func arenaPublicFormatFromSelection(selection arenaLineupSelection) arenaFormatPublic {
	definition, err := arenaFormatFromSelection(selection)
	if err != nil {
		return arenaPublicFormat(nil)
	}
	return arenaPublicFormat(definition)
}

func arenaChallengePayload(challenge store.ArenaChallenge) fiber.Map {
	var definition *arenaFormatDefinition
	if decoded, err := decodeArenaFormatSnapshot(challenge.FormatSnapshot); err == nil {
		definition = decoded
	}
	return fiber.Map{
		"id": challenge.ID, "battleKind": challenge.BattleKind,
		"attackerId": challenge.AttackerID, "defenderId": challenge.DefenderID,
		"opponentId": challenge.OpponentID, "opponentName": challenge.OpponentName, "role": challenge.Role,
		"defenseRevision": challenge.DefenseRevision, "seed": challenge.Seed,
		"status": challenge.Status, "outcome": challenge.Outcome, "attempts": challenge.Attempts,
		"ratingEligible":       challenge.RatingEligible,
		"attackerRewardTokens": challenge.AttackerRewardTokens, "defenderRewardTokens": challenge.DefenderRewardTokens,
		"bountyRewardTokens":  challenge.BountyRewardTokens,
		"attackerRatingDelta": challenge.AttackerRatingDelta, "defenderRatingDelta": challenge.DefenderRatingDelta,
		"createdAt": challenge.CreatedAt, "startedAt": challenge.StartedAt, "completedAt": challenge.CompletedAt,
		"format": arenaPublicFormat(definition),
	}
}

func (s *Server) arenaPublicLineup(raw json.RawMessage, _ *arenaFormatDefinition) []fiber.Map {
	var units []battleUnitState
	if err := json.Unmarshal(raw, &units); err != nil {
		return []fiber.Map{}
	}
	out := make([]fiber.Map, 0, len(units))
	for _, unit := range units {
		unitType := strings.ToLower(strings.TrimSpace(unit.UnitType))
		if unitType != "player" && unitType != "kitty" {
			continue
		}
		out = append(out, fiber.Map{
			"unitType":        unitType,
			"kittyId":         unit.KittyID,
			"name":            unit.Name,
			"position":        unit.Position,
			"battleLevel":     unit.BattleLevel,
			"maxHp":           roundTo4(unit.MaxHP),
			"skillIds":        append([]string(nil), unit.SkillIDs...),
			"equipmentDigest": append([]string(nil), unit.EquipmentDigest...),
		})
	}
	return out
}

func arenaChallengeChargePolicy(cfg config.ArenaSettings) store.ArenaChallengeChargePolicy {
	return store.ArenaChallengeChargePolicy{
		Capacity: cfg.ChallengeChargeCapacity,
		Recovery: time.Duration(cfg.ChallengeChargeRecoverySeconds) * time.Second,
	}
}

func arenaChallengeChargePayload(cfg config.ArenaSettings, state store.ArenaChallengeChargeState) fiber.Map {
	return fiber.Map{
		"serverTime":              state.ServerTime,
		"remaining":               state.Remaining,
		"capacity":                state.Capacity,
		"recoverySeconds":         state.RecoverySeconds,
		"nextRecoveryAt":          state.NextRecoveryAt,
		"opponentCooldownSeconds": cfg.OpponentChallengeCooldownSeconds,
	}
}

func arenaOpponentCooldownPayload(cfg config.ArenaSettings, state store.ArenaOpponentCooldownState) fiber.Map {
	return fiber.Map{
		"serverTime": state.ServerTime,
		"seconds":    cfg.OpponentChallengeCooldownSeconds,
	}
}

func arenaOpponentCooldownUntil(state store.ArenaOpponentCooldownState, opponentID int64) *time.Time {
	until, exists := state.OpponentUntilByUserID[opponentID]
	if !exists {
		return nil
	}
	return &until
}

func (s *Server) arenaTargetPayload(target store.ArenaTarget, cooldownState store.ArenaOpponentCooldownState, recommendationTier, recommendationToken string) fiber.Map {
	payload := s.arenaDefensePayload(target.ArenaDefense)
	payload["username"] = target.Username
	payload["rating"] = target.Rating
	payload["estimatedRewardTokens"] = s.deps.Config.GlobalSettings.Arena.AttackerWinTokens + target.BountyTokens
	payload["opponentCooldownUntil"] = arenaOpponentCooldownUntil(cooldownState, target.UserID)
	payload["ratingEligible"] = recommendationTier != "" && recommendationToken != ""
	payload["recommendationTier"] = recommendationTier
	if recommendationToken != "" {
		payload["recommendationToken"] = recommendationToken
	}
	return payload
}

func (s *Server) arenaTargetsPayload(ctx context.Context, userID int64, rows []store.ArenaTarget, cfg config.ArenaSettings, recommendationTiers map[int64]string, now time.Time) ([]fiber.Map, store.ArenaOpponentCooldownState, error) {
	opponentIDs := make([]int64, 0, len(rows))
	for _, row := range rows {
		opponentIDs = append(opponentIDs, row.UserID)
	}
	opponentCooldown := time.Duration(cfg.OpponentChallengeCooldownSeconds) * time.Second
	cooldownState, err := s.deps.Store.GetArenaOpponentCooldownState(ctx, userID, opponentIDs, opponentCooldown)
	if err != nil {
		return nil, store.ArenaOpponentCooldownState{}, err
	}
	payload := make([]fiber.Map, 0, len(rows))
	for _, row := range rows {
		tier := recommendationTiers[row.UserID]
		token := ""
		if tier != "" {
			token = arenaRecommendationToken(s.deps.Config.JWTSecret, userID, row.UserID, row.Revision, now)
		}
		payload = append(payload, s.arenaTargetPayload(row, cooldownState, tier, token))
	}
	return payload, cooldownState, nil
}

func normalizeArenaTargetScope(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "more", "all":
		return "more"
	default:
		return "suggested"
	}
}

func arenaTargetRotationDay(now time.Time) string {
	return now.UTC().Format("2006-01-02")
}

func arenaTargetRotationScore(requesterID, targetID int64, now time.Time, refreshedAt *time.Time) uint64 {
	h := fnv.New64a()
	if refreshedAt == nil {
		_, _ = fmt.Fprintf(h, "%d:%d:%s", requesterID, targetID, arenaTargetRotationDay(now))
	} else {
		_, _ = fmt.Fprintf(h, "%d:%d:%s:%s", requesterID, targetID, arenaTargetRotationDay(now), refreshedAt.UTC().Format(time.RFC3339Nano))
	}
	return h.Sum64()
}

func selectArenaRecommendationPool(rows []store.ArenaTarget, count int, requesterID int64, now time.Time, refreshedAt *time.Time) []store.ArenaTarget {
	pool := append([]store.ArenaTarget(nil), rows...)
	sort.SliceStable(pool, func(i, j int) bool {
		left := arenaTargetRotationScore(requesterID, pool[i].UserID, now, refreshedAt)
		right := arenaTargetRotationScore(requesterID, pool[j].UserID, now, refreshedAt)
		if left != right {
			return left < right
		}
		return pool[i].UserID < pool[j].UserID
	})
	return pool[:minInt(count, len(pool))]
}

func selectArenaRecommendedTargets(rows []store.ArenaTarget, cfg config.ArenaSettings, requesterID int64, now time.Time, refreshedAt *time.Time) []arenaRecommendedTarget {
	targetCount := minInt(cfg.TargetCount, len(rows))
	nearbyCount := minInt(cfg.NearbyTargetCount, targetCount)
	relaxedCount := minInt(cfg.RelaxedTargetCount, targetCount-nearbyCount)
	selected := make([]arenaRecommendedTarget, 0, targetCount)

	nearbyEnd := nearbyCount
	relaxedEnd := nearbyCount + relaxedCount
	if refreshedAt != nil && len(rows) > targetCount {
		extraCandidates := len(rows) - targetCount
		weightedCount := nearbyCount + relaxedCount
		nearbyExtra := 0
		if weightedCount > 0 {
			nearbyExtra = minInt(extraCandidates*nearbyCount/weightedCount, nearbyCount*2)
		}
		relaxedExtra := minInt(extraCandidates-nearbyExtra, relaxedCount*3)
		nearbyEnd += nearbyExtra
		relaxedEnd = nearbyEnd + relaxedCount + relaxedExtra
	}

	nearbyRows := rows[:nearbyEnd]
	if refreshedAt != nil {
		nearbyRows = selectArenaRecommendationPool(nearbyRows, nearbyCount, requesterID, now, refreshedAt)
	}
	for _, row := range nearbyRows[:minInt(nearbyCount, len(nearbyRows))] {
		selected = append(selected, arenaRecommendedTarget{Target: row, Tier: arenaRecommendationTierNearby})
	}
	relaxedRows := rows[nearbyEnd:relaxedEnd]
	if refreshedAt != nil {
		relaxedRows = selectArenaRecommendationPool(relaxedRows, relaxedCount, requesterID, now, refreshedAt)
	}
	for _, row := range relaxedRows[:minInt(relaxedCount, len(relaxedRows))] {
		selected = append(selected, arenaRecommendedTarget{Target: row, Tier: arenaRecommendationTierRelaxed})
	}

	rest := selectArenaRecommendationPool(rows[relaxedEnd:], targetCount-len(selected), requesterID, now, refreshedAt)
	for _, row := range rest {
		if len(selected) >= targetCount {
			break
		}
		selected = append(selected, arenaRecommendedTarget{Target: row, Tier: arenaRecommendationTierRandom})
	}
	return selected
}

func arenaRecommendationRefreshPayload(cfg config.ArenaSettings, state arenaRecommendationRefreshState) fiber.Map {
	return fiber.Map{
		"serverTime":      state.ServerTime,
		"availableAt":     state.AvailableAt,
		"cooldownSeconds": cfg.RecommendationRefreshSeconds,
	}
}

func arenaMoreTargetsOffset(raw string) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, nil
	}
	offset, err := strconv.Atoi(raw)
	if err != nil || offset < 0 || offset > arenaMoreTargetsLimit {
		return 0, store.ErrBadQuantity
	}
	return offset, nil
}

func paginateArenaMoreTargets(rows []store.ArenaTarget, offset int) ([]store.ArenaTarget, int, bool) {
	if offset < 0 {
		offset = 0
	}
	if offset >= len(rows) {
		return []store.ArenaTarget{}, offset, false
	}
	end := minInt(offset+arenaMoreTargetsPageSize, len(rows))
	return rows[offset:end], end, end < len(rows)
}

func arenaRecommendationToken(secret string, requesterID, targetID, revision int64, now time.Time) string {
	if strings.TrimSpace(secret) == "" || requesterID <= 0 || targetID <= 0 || revision <= 0 || now.IsZero() {
		return ""
	}
	expiresAt := now.Add(arenaRecommendationTokenTTL).Unix()
	payload := []byte(fmt.Sprintf("%d:%d:%d:%d", requesterID, targetID, revision, expiresAt))
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte("arena-recommendation-v1\x00"))
	_, _ = mac.Write(payload)
	return base64.RawURLEncoding.EncodeToString(payload) + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func validArenaRecommendationToken(secret, token string, requesterID, targetID, revision int64, now time.Time) bool {
	if strings.TrimSpace(secret) == "" || len(token) == 0 || len(token) > 512 || requesterID <= 0 || targetID <= 0 || revision <= 0 || now.IsZero() {
		return false
	}
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return false
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil || len(payload) == 0 || len(payload) > 160 {
		return false
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte("arena-recommendation-v1\x00"))
	_, _ = mac.Write(payload)
	if !hmac.Equal(signature, mac.Sum(nil)) {
		return false
	}
	fields := strings.Split(string(payload), ":")
	if len(fields) != 4 {
		return false
	}
	values := make([]int64, len(fields))
	for index, field := range fields {
		value, parseErr := strconv.ParseInt(field, 10, 64)
		if parseErr != nil {
			return false
		}
		values[index] = value
	}
	return values[0] == requesterID && values[1] == targetID && values[2] == revision && now.Unix() <= values[3]
}

func (s *Server) handleArenaOverview(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	cfg := s.deps.Config.GlobalSettings.Arena
	if !cfg.Enabled {
		return s.writeAPIError(c, fiber.StatusNotFound, "ARENA_DISABLED", "arena is disabled", "")
	}
	state, chargeState, err := s.deps.Store.RefreshArenaPlayerState(c.Context(), userID, cfg.DefaultRating, arenaChallengeChargePolicy(cfg))
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "ARENA_OVERVIEW_FAILED", "load arena overview failed", err.Error())
	}
	var defense any
	if row, err := s.deps.Store.GetArenaDefense(c.Context(), userID); err == nil {
		defense = s.arenaDefensePayload(row)
	} else if !errors.Is(err, store.ErrArenaDefenseNotFound) {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "ARENA_OVERVIEW_FAILED", "load arena defense failed", err.Error())
	}
	kitties, err := s.deps.Store.ListPlayerKitties(c.Context(), userID)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "ARENA_OVERVIEW_FAILED", "load arena kitties failed", err.Error())
	}
	kittyPayload := make([]fiber.Map, 0, len(kitties))
	for _, kitty := range kitties {
		position, personalityID, personalityKey, lockErr := s.deps.Store.GetArenaKittyPositionLock(c.Context(), userID, kitty.ID)
		if lockErr != nil {
			return s.writeAPIError(c, fiber.StatusInternalServerError, "ARENA_OVERVIEW_FAILED", "load kitty position failed", lockErr.Error())
		}
		kittyPayload = append(kittyPayload, fiber.Map{
			"id": kitty.ID, "name": kitty.Name, "level": kitty.Level, "status": kitty.Status,
			"positionLocked": position, "positionLockedByPersonalityId": personalityID, "positionLockedByPersonalityKey": personalityKey,
		})
	}
	formats, err := s.arenaPublicFormats()
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "ARENA_OVERVIEW_FAILED", "load arena formats failed", err.Error())
	}
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{
		"battleKind":       battleKindArena,
		"state":            state,
		"defense":          defense,
		"kitties":          kittyPayload,
		"formats":          formats,
		"challengeEntries": arenaChallengeChargePayload(cfg, chargeState),
		"rules": fiber.Map{
			"historyKeepCount":  cfg.HistoryKeepCount,
			"attackerWinTokens": cfg.AttackerWinTokens, "attackerLossTokens": cfg.AttackerLossTokens, "defenderWinTokens": cfg.DefenderWinTokens,
			"bountyPerDefenseWin": cfg.BountyPerDefenseWin, "bountyCap": cfg.BountyCap,
		},
	})
}

func (s *Server) handleArenaTargets(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	cfg := s.deps.Config.GlobalSettings.Arena
	if !cfg.Enabled {
		return s.writeAPIError(c, fiber.StatusNotFound, "ARENA_DISABLED", "arena is disabled", "")
	}
	scope := normalizeArenaTargetScope(c.Query("scope", "suggested"))
	formatFilter := strings.ToLower(strings.TrimSpace(c.Query("format", "all")))
	if formatFilter != "all" {
		normalized := normalizeArenaFormatID(formatFilter)
		_, formatsByID, catalogErr := s.arenaFormatCatalog()
		if catalogErr != nil {
			return s.writeAPIError(c, fiber.StatusInternalServerError, "ARENA_TARGETS_FAILED", "load arena formats failed", catalogErr.Error())
		}
		definition, exists := formatsByID[normalized]
		if normalized == "" || !exists || !definition.Enabled {
			return s.writeAPIError(c, fiber.StatusBadRequest, "ARENA_TARGETS_FILTER_INVALID", "invalid arena target filter", "")
		}
		formatFilter = normalized
	}
	rotationNow := time.Now()
	if scope == "more" {
		offsetRaw := strings.TrimSpace(c.Query("offset"))
		offset, err := arenaMoreTargetsOffset(offsetRaw)
		if err != nil {
			return s.writeAPIError(c, fiber.StatusBadRequest, "ARENA_TARGETS_OFFSET_INVALID", "invalid arena target offset", "")
		}
		rows, err := s.deps.Store.ListArenaMoreTargets(c.Context(), userID, cfg.DefaultRating, arenaTargetRotationDay(rotationNow), arenaMoreTargetsLimit, formatFilter, arenaAlchemyEligibilityPolicyVersion)
		if err != nil {
			return s.writeAPIError(c, fiber.StatusInternalServerError, "ARENA_TARGETS_FAILED", "load arena targets failed", err.Error())
		}
		rows = filterArenaTargetsByEligibilityPolicy(rows)
		page := rows
		nextOffset := len(rows)
		hasMore := false
		pageSize := arenaMoreTargetsLimit
		if offsetRaw != "" {
			page, nextOffset, hasMore = paginateArenaMoreTargets(rows, offset)
			pageSize = arenaMoreTargetsPageSize
		}
		payload, cooldownState, err := s.arenaTargetsPayload(c.Context(), userID, page, cfg, nil, rotationNow)
		if err != nil {
			return s.writeAPIError(c, fiber.StatusInternalServerError, "ARENA_TARGETS_FAILED", "load arena cooldown failed", err.Error())
		}
		return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{
			"battleKind": battleKindArena, "targets": payload, "offset": offset, "limit": pageSize,
			"nextOffset": nextOffset, "hasMore": hasMore,
			"opponentCooldown": arenaOpponentCooldownPayload(cfg, cooldownState),
		})
	}
	battleLevel, err := s.deps.Store.GetPlayerBaseBattleLevel(c.Context(), userID)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "ARENA_TARGETS_FAILED", "load arena targets failed", err.Error())
	}
	refreshCooldown := time.Duration(cfg.RecommendationRefreshSeconds) * time.Second
	refreshState, err := s.getArenaRecommendationRefreshState(c.Context(), userID, refreshCooldown)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "ARENA_TARGETS_FAILED", "load arena recommendation refresh state failed", err.Error())
	}
	rotationNow = refreshState.ServerTime
	rows, err := s.deps.Store.ListArenaTargets(c.Context(), userID, battleLevel, cfg.DefaultRating, 100, 0, true, formatFilter, arenaAlchemyEligibilityPolicyVersion)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "ARENA_TARGETS_FAILED", "load arena targets failed", err.Error())
	}
	rows = filterArenaTargetsByEligibilityPolicy(rows)
	recommendations := selectArenaRecommendedTargets(rows, cfg, userID, rotationNow, refreshState.RefreshedAt)
	selected := make([]store.ArenaTarget, 0, len(recommendations))
	recommendationTiers := make(map[int64]string, len(recommendations))
	for _, recommendation := range recommendations {
		selected = append(selected, recommendation.Target)
		recommendationTiers[recommendation.Target.UserID] = recommendation.Tier
	}
	payload, cooldownState, err := s.arenaTargetsPayload(c.Context(), userID, selected, cfg, recommendationTiers, rotationNow)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "ARENA_TARGETS_FAILED", "load arena cooldown failed", err.Error())
	}
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{
		"battleKind": battleKindArena, "targets": payload,
		"opponentCooldown":      arenaOpponentCooldownPayload(cfg, cooldownState),
		"recommendationRefresh": arenaRecommendationRefreshPayload(cfg, refreshState),
	})
}

func (s *Server) handleArenaTargetsRefresh(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	cfg := s.deps.Config.GlobalSettings.Arena
	if !cfg.Enabled {
		return s.writeAPIError(c, fiber.StatusNotFound, "ARENA_DISABLED", "arena is disabled", "")
	}
	refreshState, err := s.refreshArenaRecommendations(c.Context(), userID, time.Duration(cfg.RecommendationRefreshSeconds)*time.Second)
	if errors.Is(err, errArenaRecommendationRefreshCooldown) {
		return s.writeAPIErrorWithData(c, fiber.StatusConflict, "ARENA_RECOMMENDATION_REFRESH_COOLDOWN", "arena recommendations are on refresh cooldown", "", fiber.Map{
			"recommendationRefresh": arenaRecommendationRefreshPayload(cfg, refreshState),
		})
	}
	if err != nil {
		return s.writeArenaError(c, "ARENA_RECOMMENDATION_REFRESH_FAILED", err)
	}
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{
		"battleKind":            battleKindArena,
		"recommendationRefresh": arenaRecommendationRefreshPayload(cfg, refreshState),
	})
}

func (s *Server) handleArenaLeaderboard(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	if !s.deps.Config.GlobalSettings.Arena.Enabled {
		return s.writeAPIError(c, fiber.StatusNotFound, "ARENA_DISABLED", "arena is disabled", "")
	}
	page, err := s.deps.Store.ListArenaLeaderboard(c.Context(), userID, arenaLeaderboardLimit, 0)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "ARENA_LEADERBOARD_FAILED", "load arena leaderboard failed", err.Error())
	}
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{
		"battleKind": battleKindArena,
		"entries":    page.Entries,
		"self":       page.Self,
		"offset":     0,
		"limit":      arenaLeaderboardLimit,
		"total":      page.Total,
		"hasMore":    false,
	})
}

func (s *Server) handleArenaDefensePublish(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	if !s.deps.Config.GlobalSettings.Arena.Enabled {
		return s.writeAPIError(c, fiber.StatusNotFound, "ARENA_DISABLED", "arena is disabled", "")
	}
	var req arenaPublishDefenseRequest
	if err := c.BodyParser(&req); err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "ARENA_LINEUP_INVALID", "invalid arena lineup", err.Error())
	}
	selection, err := normalizeArenaSelection(req.Selection)
	if err != nil {
		return s.writeArenaError(c, "ARENA_DEFENSE_PUBLISH_FAILED", err)
	}
	format, err := s.resolveArenaFormatForPublish(selection.FormatID)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "ARENA_FORMAT_INVALID", "selected arena format is unavailable", "")
	}
	selection.FormatSnapshot = format
	selection.FormatID = format.ID
	selection, lineup, err := s.buildArenaLineupSnapshot(c.Context(), userID, selection, time.Now(), format)
	if err != nil {
		return s.writeArenaError(c, "ARENA_DEFENSE_PUBLISH_FAILED", err)
	}
	digest, lineupRaw, err := arenaLineupDigest(lineup)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "ARENA_DEFENSE_PUBLISH_FAILED", "publish arena defense failed", err.Error())
	}
	selectionRaw, _ := json.Marshal(selection)
	kittyIDs := make([]int64, 0, len(selection.Kitties))
	for _, kitty := range selection.Kitties {
		kittyIDs = append(kittyIDs, kitty.KittyID)
	}
	if _, err := s.deps.Store.EnsureArenaPlayerState(c.Context(), userID, s.deps.Config.GlobalSettings.Arena.DefaultRating, s.deps.Config.GlobalSettings.Arena.ChallengeChargeCapacity); err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "ARENA_DEFENSE_PUBLISH_FAILED", "publish arena defense failed", err.Error())
	}
	defense, err := s.deps.Store.PublishArenaDefense(c.Context(), store.PublishArenaDefenseInput{
		UserID: userID, BattleLevel: lineup[0].BattleLevel, KittyIDs: kittyIDs, Selection: selectionRaw, Lineup: lineupRaw, LineupDigest: digest,
	})
	if err != nil {
		return s.writeArenaError(c, "ARENA_DEFENSE_PUBLISH_FAILED", err)
	}
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{"battleKind": battleKindArena, "defense": s.arenaDefensePayload(defense)})
}

func (s *Server) handleArenaDefenseUnpublish(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	if !s.deps.Config.GlobalSettings.Arena.Enabled {
		return s.writeAPIError(c, fiber.StatusNotFound, "ARENA_DISABLED", "arena is disabled", "")
	}
	if err := s.deps.Store.UnpublishArenaDefense(c.Context(), userID); err != nil {
		return s.writeArenaError(c, "ARENA_DEFENSE_UNPUBLISH_FAILED", err)
	}
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{"battleKind": battleKindArena, "active": false})
}

func (s *Server) handleArenaChallengeCreate(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	cfg := s.deps.Config.GlobalSettings.Arena
	if !cfg.Enabled {
		return s.writeAPIError(c, fiber.StatusNotFound, "ARENA_DISABLED", "arena is disabled", "")
	}
	var req arenaCreateChallengeRequest
	if err := c.BodyParser(&req); err != nil || req.DefenderID <= 0 || req.DefenderID == userID {
		return s.writeAPIError(c, fiber.StatusBadRequest, "ARENA_CHALLENGE_INVALID", "invalid arena challenge", "")
	}
	attackerDefense, err := s.deps.Store.GetArenaDefense(c.Context(), userID)
	if err != nil || !attackerDefense.Active {
		if err == nil {
			err = store.ErrArenaDefenseNotFound
		}
		return s.writeArenaError(c, "ARENA_CHALLENGE_CREATE_FAILED", err)
	}
	defenderDefense, err := s.deps.Store.GetArenaDefense(c.Context(), req.DefenderID)
	if err != nil || !defenderDefense.Active {
		if err == nil {
			err = store.ErrArenaDefenseNotFound
		}
		return s.writeArenaError(c, "ARENA_CHALLENGE_CREATE_FAILED", err)
	}
	ratingEligible := false
	if strings.TrimSpace(req.RecommendationToken) != "" {
		if !validArenaRecommendationToken(s.deps.Config.JWTSecret, req.RecommendationToken, userID, req.DefenderID, defenderDefense.Revision, time.Now()) {
			return s.writeAPIError(c, fiber.StatusConflict, "ARENA_RECOMMENDATION_EXPIRED", "arena recommendation expired", "")
		}
		ratingEligible = true
	}
	chargePolicy := arenaChallengeChargePolicy(cfg)
	_, chargeState, err := s.deps.Store.RefreshArenaPlayerState(c.Context(), userID, cfg.DefaultRating, chargePolicy)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "ARENA_CHALLENGE_CREATE_FAILED", "load arena challenge entries failed", err.Error())
	}
	if chargeState.Remaining <= 0 {
		return s.writeArenaError(c, "ARENA_CHALLENGE_CREATE_FAILED", store.ErrArenaChallengeChargesExhausted)
	}
	opponentCooldown := time.Duration(cfg.OpponentChallengeCooldownSeconds) * time.Second
	cooldownState, err := s.deps.Store.GetArenaOpponentCooldownState(c.Context(), userID, []int64{req.DefenderID}, opponentCooldown)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "ARENA_CHALLENGE_CREATE_FAILED", "load arena cooldown failed", err.Error())
	}
	if _, cooling := cooldownState.OpponentUntilByUserID[req.DefenderID]; cooling {
		return s.writeArenaError(c, "ARENA_CHALLENGE_CREATE_FAILED", store.ErrArenaChallengeOpponentCooldown)
	}
	var attackerSelection arenaLineupSelection
	if err := json.Unmarshal(attackerDefense.Selection, &attackerSelection); err != nil {
		return s.writeAPIError(c, fiber.StatusConflict, "ARENA_LINEUP_INVALID", "published arena lineup is invalid", err.Error())
	}
	var defenderSelection arenaLineupSelection
	if err := json.Unmarshal(defenderDefense.Selection, &defenderSelection); err != nil {
		return s.writeAPIError(c, fiber.StatusConflict, "ARENA_LINEUP_INVALID", "target arena lineup is invalid", err.Error())
	}
	if !arenaSelectionEligibilityPolicyCurrent(attackerSelection) || !arenaSelectionEligibilityPolicyCurrent(defenderSelection) {
		return s.writeAPIError(c, fiber.StatusConflict, "ARENA_DEFENSE_POLICY_OUTDATED", "arena defense must be republished under the current equipment policy", "")
	}
	defenderSelection, err = normalizeArenaSelection(defenderSelection)
	if err != nil {
		return s.writeArenaError(c, "ARENA_CHALLENGE_CREATE_FAILED", err)
	}
	format, err := arenaFormatFromSelection(defenderSelection)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusConflict, "ARENA_FORMAT_SNAPSHOT_INVALID", "target arena format snapshot is invalid", err.Error())
	}
	_, lineup, err := s.buildArenaLineupSnapshot(c.Context(), userID, attackerSelection, time.Now(), format)
	if err != nil {
		return s.writeArenaError(c, "ARENA_CHALLENGE_CREATE_FAILED", err)
	}
	_, lineupRaw, err := arenaLineupDigest(lineup)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "ARENA_CHALLENGE_CREATE_FAILED", "create arena challenge failed", err.Error())
	}
	seed, err := randomArenaSeed()
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "ARENA_CHALLENGE_CREATE_FAILED", "create arena challenge failed", err.Error())
	}
	formatRaw, err := json.Marshal(format)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "ARENA_CHALLENGE_CREATE_FAILED", "create arena challenge failed", err.Error())
	}
	challenge, updatedChargeState, err := s.deps.Store.CreateArenaChallenge(c.Context(), store.CreateArenaChallengeInput{
		AttackerID: userID, DefenderID: req.DefenderID, ExpectedDefenseRevision: defenderDefense.Revision, Seed: seed, AttackerLineup: lineupRaw,
		FormatSnapshot:   formatRaw,
		RatingEligible:   ratingEligible,
		DefaultRating:    cfg.DefaultRating,
		ChargePolicy:     chargePolicy,
		OpponentCooldown: opponentCooldown,
	})
	if err != nil {
		return s.writeArenaError(c, "ARENA_CHALLENGE_CREATE_FAILED", err)
	}
	var opponentCooldownUntil *time.Time
	if opponentCooldown > 0 {
		until := challenge.CreatedAt.Add(opponentCooldown)
		opponentCooldownUntil = &until
	}
	return s.writeAPISuccess(c, fiber.StatusAccepted, fiber.Map{
		"battleKind":            battleKindArena,
		"challenge":             arenaChallengePayload(challenge),
		"challengeEntries":      arenaChallengeChargePayload(cfg, updatedChargeState),
		"opponentCooldownUntil": opponentCooldownUntil,
	})
}

func (s *Server) handleArenaChallengeHistory(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	if !s.deps.Config.GlobalSettings.Arena.Enabled {
		return s.writeAPIError(c, fiber.StatusNotFound, "ARENA_DISABLED", "arena is disabled", "")
	}
	offset, _ := strconv.Atoi(c.Query("offset", "0"))
	rows, err := s.deps.Store.ListArenaChallengeHistory(c.Context(), userID, c.Query("role", "all"), s.deps.Config.GlobalSettings.Arena.HistoryKeepCount, offset)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "ARENA_HISTORY_FAILED", "load arena history failed", err.Error())
	}
	payload := make([]fiber.Map, 0, len(rows))
	for _, row := range rows {
		payload = append(payload, arenaChallengePayload(row))
	}
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{"battleKind": battleKindArena, "challenges": payload, "historyKeepCount": s.deps.Config.GlobalSettings.Arena.HistoryKeepCount})
}

func (s *Server) handleArenaChallengeReplay(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	if !s.deps.Config.GlobalSettings.Arena.Enabled {
		return s.writeAPIError(c, fiber.StatusNotFound, "ARENA_DISABLED", "arena is disabled", "")
	}
	challengeID, err := strconv.ParseInt(strings.TrimSpace(c.Params("challengeID")), 10, 64)
	if err != nil || challengeID <= 0 {
		return s.writeAPIError(c, fiber.StatusBadRequest, "ARENA_CHALLENGE_INVALID", "invalid arena challenge", "")
	}
	payload, err := s.deps.Store.GetArenaChallengeReplay(c.Context(), userID, challengeID, s.deps.Config.GlobalSettings.Arena.HistoryKeepCount)
	if err != nil {
		return s.writeArenaError(c, "ARENA_REPLAY_FAILED", err)
	}
	return s.writeAPISuccess(c, fiber.StatusOK, payload)
}

func (s *Server) handleArenaChallengeShare(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	if !s.deps.Config.GlobalSettings.Arena.Enabled {
		return s.writeAPIError(c, fiber.StatusNotFound, "ARENA_DISABLED", "arena is disabled", "")
	}
	challengeID, err := strconv.ParseInt(strings.TrimSpace(c.Params("challengeID")), 10, 64)
	if err != nil || challengeID <= 0 {
		return s.writeAPIError(c, fiber.StatusBadRequest, "ARENA_CHALLENGE_INVALID", "invalid arena challenge", "")
	}
	token, err := randomArenaReplayShareToken()
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "ARENA_REPLAY_SHARE_FAILED", "share arena replay failed", err.Error())
	}
	share, err := s.deps.Store.CreateArenaReplayShare(c.Context(), userID, challengeID, token, s.deps.Config.GlobalSettings.Arena.HistoryKeepCount)
	if err != nil {
		return s.writeArenaError(c, "ARENA_REPLAY_SHARE_FAILED", err)
	}
	return s.writeAPISuccess(c, fiber.StatusCreated, fiber.Map{
		"battleKind": battleKindArena,
		"share":      share,
	})
}

func (s *Server) handleArenaSharedReplay(c *fiber.Ctx) error {
	if _, err := s.authUserID(c); err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	if !s.deps.Config.GlobalSettings.Arena.Enabled {
		return s.writeAPIError(c, fiber.StatusNotFound, "ARENA_DISABLED", "arena is disabled", "")
	}
	shared, err := s.deps.Store.GetArenaSharedReplay(c.Context(), c.Params("token"), s.deps.Config.GlobalSettings.Arena.HistoryKeepCount)
	if err != nil {
		return s.writeArenaError(c, "ARENA_REPLAY_SHARED_FAILED", err)
	}
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{
		"battleKind": battleKindArena,
		"share":      shared.Share,
		"payload":    shared.Payload,
	})
}

func (s *Server) arenaShopOffersPayload() []fiber.Map {
	offers := s.deps.Config.GlobalSettings.Arena.ShopOffers
	out := make([]fiber.Map, 0, len(offers))
	for _, offer := range offers {
		out = append(out, fiber.Map{"id": offer.ID, "itemId": offer.ItemID, "quantity": offer.Quantity, "cost": offer.Cost})
	}
	return out
}

func (s *Server) handleArenaShopGet(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	if !s.deps.Config.GlobalSettings.Arena.Enabled {
		return s.writeAPIError(c, fiber.StatusNotFound, "ARENA_DISABLED", "arena is disabled", "")
	}
	state, err := s.deps.Store.EnsureArenaPlayerState(c.Context(), userID, s.deps.Config.GlobalSettings.Arena.DefaultRating, s.deps.Config.GlobalSettings.Arena.ChallengeChargeCapacity)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "ARENA_SHOP_FAILED", "load arena shop failed", err.Error())
	}
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{"battleKind": battleKindArena, "tokens": state.Tokens, "offers": s.arenaShopOffersPayload()})
}

func (s *Server) handleArenaShopBuy(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	if !s.deps.Config.GlobalSettings.Arena.Enabled {
		return s.writeAPIError(c, fiber.StatusNotFound, "ARENA_DISABLED", "arena is disabled", "")
	}
	var req arenaShopBuyRequest
	if err := c.BodyParser(&req); err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "ARENA_SHOP_OFFER_INVALID", "invalid arena shop offer", err.Error())
	}
	var selected *config.ArenaShopOfferSettings
	for index := range s.deps.Config.GlobalSettings.Arena.ShopOffers {
		offer := &s.deps.Config.GlobalSettings.Arena.ShopOffers[index]
		if strings.EqualFold(strings.TrimSpace(offer.ID), strings.TrimSpace(req.OfferID)) {
			selected = offer
			break
		}
	}
	if selected == nil {
		return s.writeAPIError(c, fiber.StatusNotFound, "ARENA_SHOP_OFFER_NOT_FOUND", "arena shop offer not found", "")
	}
	result, err := s.deps.Store.BuyArenaShopOffer(c.Context(), userID, s.deps.Config.GlobalSettings.Arena.DefaultRating, selected.ID, selected.ItemID, selected.Quantity, selected.Cost, strings.TrimSpace(c.Get("X-Idempotency-Key")))
	if err != nil {
		return s.writeArenaError(c, "ARENA_SHOP_BUY_FAILED", err)
	}
	s.syncAfterMutation(mutationSyncScope{UserIDs: []int64{userID}, Inventory: true})
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{"battleKind": battleKindArena, "purchase": result})
}

func (s *Server) writeArenaError(c *fiber.Ctx, fallbackCode string, err error) error {
	var eligibilityErr *competitiveAlchemyEligibilityError
	if errors.As(err, &eligibilityErr) {
		return s.writeAPIErrorWithData(c, fiber.StatusConflict, "ARENA_ALCHEMY_EQUIPMENT_RESTRICTED", "arena lineup contains alchemy equipment above the current league limit", "", fiber.Map{
			"policyVersion":                 arenaAlchemyEligibilityPolicyVersion,
			"requiredAlchemyRulesetVersion": store.AlchemyCurrentRulesetVersion,
			"violations":                    eligibilityErr.Violations,
		})
	}
	switch {
	case errors.Is(err, store.ErrArenaDefenseNotFound):
		return s.writeAPIError(c, fiber.StatusNotFound, "ARENA_DEFENSE_NOT_FOUND", "arena defense not found", "")
	case errors.Is(err, store.ErrArenaDefenseBusy):
		return s.writeAPIError(c, fiber.StatusConflict, "ARENA_DEFENSE_BUSY", "arena defense has active challenges", "")
	case errors.Is(err, store.ErrArenaChallengeNotFound):
		return s.writeAPIError(c, fiber.StatusNotFound, "ARENA_CHALLENGE_NOT_FOUND", "arena challenge not found", "")
	case errors.Is(err, store.ErrArenaChallengeActive):
		return s.writeAPIError(c, fiber.StatusConflict, "ARENA_CHALLENGE_ACTIVE", "arena challenge already active", "")
	case errors.Is(err, store.ErrArenaChallengeChargesExhausted):
		return s.writeAPIError(c, fiber.StatusConflict, "ARENA_CHALLENGE_CHARGES_EXHAUSTED", "arena challenge charges are exhausted", "")
	case errors.Is(err, store.ErrArenaChallengeOpponentCooldown):
		return s.writeAPIError(c, fiber.StatusConflict, "ARENA_CHALLENGE_OPPONENT_COOLDOWN", "arena opponent is on cooldown", "")
	case errors.Is(err, errCompetitiveEquipmentSnapshotUnstable):
		return s.writeAPIError(c, fiber.StatusConflict, "ARENA_LINEUP_CHANGED", "equipment changed while the arena lineup was being prepared", "")
	case errors.Is(err, store.ErrArenaReplayShareUnavailable):
		return s.writeAPIError(c, fiber.StatusGone, "ARENA_REPLAY_SHARE_EXPIRED", "arena replay is no longer retained", "")
	case errors.Is(err, store.ErrArenaTokensInsufficient):
		return s.writeAPIError(c, fiber.StatusConflict, "ARENA_TOKENS_INSUFFICIENT", "arena tokens insufficient", "")
	case errors.Is(err, store.ErrKittyNotFound):
		return s.writeAPIError(c, fiber.StatusNotFound, "ARENA_KITTY_NOT_FOUND", "arena kitty not found", "")
	case errors.Is(err, store.ErrBattleRoomConfigInvalid):
		return s.writeAPIError(c, fiber.StatusBadRequest, "ARENA_LINEUP_INVALID", "invalid arena lineup", "")
	default:
		return s.writeAPIError(c, fiber.StatusInternalServerError, fallbackCode, "arena request failed", err.Error())
	}
}
