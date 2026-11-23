package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
	"mysecondwebgame/server/internal/store"
)

type battleRoomCreateRequest struct {
	DungeonID           string `json:"dungeonId"`
	RoomName            string `json:"roomName"`
	AutoReenterOnDefeat bool   `json:"autoReenterOnDefeat"`
	IsPublic            bool   `json:"isPublic"`
	IsFogMode           bool   `json:"isFogMode"`
	Position            string `json:"position"`
	DifficultyLevel     int    `json:"difficultyLevel"`
}

func parseBattleMaxTeamSize(raw any) int {
	value := int(toInt64Loose(raw, 3))
	if value <= 0 {
		return 3
	}
	return value
}

func toBoolLoose(value any, fallback bool) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		switch strings.ToLower(strings.TrimSpace(typed)) {
		case "1", "true", "yes":
			return true
		case "0", "false", "no":
			return false
		default:
			return fallback
		}
	case int:
		return typed != 0
	case int32:
		return typed != 0
	case int64:
		return typed != 0
	case float32:
		return typed != 0
	case float64:
		return typed != 0
	default:
		return fallback
	}
}

func parseBattleCountRange(value any, fallback battleCountRange) battleCountRange {
	switch typed := value.(type) {
	case map[string]any:
		minCount := int(toInt64Loose(typed["min"], int64(fallback.Min)))
		maxCount := int(toInt64Loose(typed["max"], int64(fallback.Max)))
		if minCount < 0 {
			minCount = 0
		}
		if maxCount < minCount {
			maxCount = minCount
		}
		return battleCountRange{Min: minCount, Max: maxCount}
	case map[string]int:
		minCount := typed["min"]
		maxCount := typed["max"]
		if minCount < 0 {
			minCount = 0
		}
		if maxCount < minCount {
			maxCount = minCount
		}
		return battleCountRange{Min: minCount, Max: maxCount}
	case string:
		trimmed := strings.TrimSpace(typed)
		if trimmed == "" {
			return fallback
		}
		num, err := strconv.ParseInt(trimmed, 10, 64)
		if err == nil {
			count := int(num)
			if count < 0 {
				count = 0
			}
			return battleCountRange{Min: count, Max: count}
		}
		var node map[string]any
		if err := json.Unmarshal([]byte(trimmed), &node); err == nil {
			return parseBattleCountRange(node, fallback)
		}
		return fallback
	default:
		count := int(toInt64Loose(value, int64(fallback.Min)))
		if count < 0 {
			count = 0
		}
		return battleCountRange{Min: count, Max: count}
	}
}

func parseBattleDropRules(raw any) map[string]battleDropRule {
	out := map[string]battleDropRule{}
	node, ok := raw.(map[string]any)
	if !ok {
		return out
	}
	for itemID, payloadRaw := range node {
		itemID = strings.TrimSpace(itemID)
		if itemID == "" {
			continue
		}
		payload, ok := payloadRaw.(map[string]any)
		if !ok {
			continue
		}
		minValue := toInt64Loose(payload["min"], 0)
		maxValue := toInt64Loose(payload["max"], minValue)
		if rangeNode, ok := payload["range"].(map[string]any); ok {
			minValue = toInt64Loose(rangeNode["min"], minValue)
			maxValue = toInt64Loose(rangeNode["max"], maxValue)
		}
		if minValue < 0 {
			minValue = 0
		}
		if maxValue < minValue {
			maxValue = minValue
		}
		percent := toFloat64Loose(payload["percent"], 0)
		if percent < 0 {
			percent = 0
		}
		if percent > 1 {
			percent = 1
		}
		out[itemID] = battleDropRule{Min: minValue, Max: maxValue, Percent: percent}
	}
	return out
}

func parseBattleCustomLineupConf(value any) []battleCustomLineupConfig {
	items := make([]battleCustomLineupConfig, 0)
	pushNode := func(node map[string]any) {
		id, _ := stringFromAny(node["id"])
		id = strings.TrimSpace(id)
		if id == "" {
			return
		}
		dropsNode, _ := node["drops"].(map[string]any)
		items = append(items, battleCustomLineupConfig{
			ID:                  id,
			NameKey:             strings.TrimSpace(toString(node["nameKey"])),
			DescKey:             strings.TrimSpace(toString(node["descKey"])),
			Percent:             clampFloat(toFloat64Loose(node["percent"], 0), 0, 1),
			IsSpecialWaveLineup: toBoolLoose(node["isSpecialWaveLineup"], false),
			Lineup:              toStringSlice(node["lineup"]),
			Drops: battleCustomLineupDrops{
				Items: parseBattleDropRules(dropsNode["items"]),
			},
		})
	}
	switch typed := value.(type) {
	case []any:
		for _, entry := range typed {
			node, ok := entry.(map[string]any)
			if !ok {
				continue
			}
			pushNode(node)
		}
	case []map[string]any:
		for _, node := range typed {
			pushNode(node)
		}
	case string:
		trimmed := strings.TrimSpace(typed)
		if trimmed == "" {
			return items
		}
		var parsed []map[string]any
		if err := json.Unmarshal([]byte(trimmed), &parsed); err == nil {
			for _, node := range parsed {
				pushNode(node)
			}
		}
	}
	return items
}

func toString(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	default:
		return ""
	}
}

type battleRoomJoinRequest struct {
	Position string `json:"position"`
}

type battleRoomConfigRequest struct {
	DungeonID           string `json:"dungeonId"`
	RoomName            string `json:"roomName"`
	AutoReenterOnDefeat bool   `json:"autoReenterOnDefeat"`
	DisableEntryRound   bool   `json:"disableEntryRound"`
	IsPublic            bool   `json:"isPublic"`
	IsFogMode           bool   `json:"isFogMode"`
	DifficultyLevel     int    `json:"difficultyLevel"`
}

type battleRoomReadyRequest struct {
	Ready bool `json:"ready"`
}

type battleRoomPositionRequest struct {
	Position string `json:"position"`
}

type battleRoomKittyUnitRequest struct {
	KittyID  int64  `json:"kittyId"`
	Position string `json:"position"`
}

type battleRoomKickRequest struct {
	TargetUserID int64 `json:"targetUserId"`
}

type battleRoomTransferOwnerRequest struct {
	TargetUserID int64 `json:"targetUserId"`
}

type battlePointAllocateRequest struct {
	AttrKey string `json:"attrKey"`
	Points  int64  `json:"points"`
}

type battlePointAllocateBatchRequest struct {
	Allocations map[string]int64 `json:"allocations"`
}

func normalizeBattlePointAllocations(raw map[string]int64) (map[string]int64, error) {
	if len(raw) == 0 {
		return nil, store.ErrBattleRoomConfigInvalid
	}
	out := make(map[string]int64, len(raw))
	var total int64
	for key, points := range raw {
		attrKey := strings.TrimSpace(key)
		if attrKey == "" || points <= 0 {
			return nil, store.ErrBattleRoomConfigInvalid
		}
		if points > math.MaxInt64-out[attrKey] {
			return nil, store.ErrBattleRoomConfigInvalid
		}
		out[attrKey] += points
		if out[attrKey] <= 0 {
			return nil, store.ErrBattleRoomConfigInvalid
		}
		if points > math.MaxInt64-total {
			return nil, store.ErrBattleRoomConfigInvalid
		}
		total += points
	}
	if total <= 0 {
		return nil, store.ErrBattleRoomConfigInvalid
	}
	return out, nil
}

type battleWorldOption struct {
	ID                    string                         `json:"id"`
	EnemyIDs              []string                       `json:"enemyIds"`
	SpecialEnemyIDs       []string                       `json:"specialEnemyIds"`
	SpecialWaves          []int                          `json:"specialWaves"`
	WaveSchedules         []battleWaveSchedule           `json:"waveSchedules,omitempty"`
	SpecialWaveEvery      int                            `json:"specialWaveEvery,omitempty"`
	SpecialWaveMinCount   int                            `json:"specialWaveMinCount,omitempty"`
	SpecialWaveMaxCount   int                            `json:"specialWaveMaxCount,omitempty"`
	EntryDurationSeconds  int64                          `json:"entryDurationSeconds"`
	MaxTeamSize           int                            `json:"maxTeamSize"`
	EntryCosts            map[string]int64               `json:"entryCosts"`
	NormalEnemyCount      int                            `json:"normalEnemyCount"`
	NormalEnemyCountRange battleCountRange               `json:"normalEnemyCountRange"`
	ExtraEnemyPerPlayer   battleCountRange               `json:"extraEnemyPerPlayer"`
	CustomLineupConf      []battleCustomLineupConfig     `json:"customLineupConf,omitempty"`
	BattlefieldEffects    []battleWorldBattlefieldEffect `json:"battlefieldEffects,omitempty"`
}

type battleSpecialWaveConfig struct {
	FixedWaves map[int]struct{}
	Every      int
	MinCount   int
	MaxCount   int
}

type battleWaveSchedule struct {
	ID           string                          `json:"id"`
	NameKey      string                          `json:"nameKey,omitempty"`
	DescKey      string                          `json:"descKey,omitempty"`
	Every        int                             `json:"every"`
	Priority     int                             `json:"priority,omitempty"`
	EnemyChoices []battleWaveScheduleEnemyChoice `json:"enemyChoices"`
	MinCount     int                             `json:"minCount"`
	MaxCount     int                             `json:"maxCount"`
}

type battleWaveScheduleEnemyChoice struct {
	EnemyID string  `json:"enemyId"`
	Weight  float64 `json:"weight"`
}

func (s *Server) handleBattleWorlds(c *fiber.Ctx) error {
	_, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	enabledEnemyIDs := s.enabledEnemyIDSet()
	items := make([]battleWorldOption, 0, len(s.deps.GameData.World))
	for _, row := range s.deps.GameData.World {
		cfg, ok := normalizedBattleWorldConfigRow(row, enabledEnemyIDs)
		if !ok {
			continue
		}
		specialWaves := sortedSpecialWaves(cfg.SpecialWaves)
		option := battleWorldOption{
			ID:                    cfg.ID,
			EnemyIDs:              cfg.EnemyIDs,
			SpecialEnemyIDs:       cfg.SpecialEnemyIDs,
			SpecialWaves:          specialWaves,
			WaveSchedules:         append([]battleWaveSchedule(nil), cfg.WaveSchedules...),
			SpecialWaveEvery:      cfg.SpecialWaveEvery,
			SpecialWaveMinCount:   cfg.SpecialWaveMinCount,
			SpecialWaveMaxCount:   cfg.SpecialWaveMaxCount,
			EntryDurationSeconds:  cfg.EntryDurationSeconds,
			MaxTeamSize:           cfg.MaxTeamSize,
			EntryCosts:            cfg.EntryCosts,
			NormalEnemyCountRange: cfg.NormalEnemyCountRange,
			ExtraEnemyPerPlayer:   cfg.ExtraEnemyPerPlayer,
			CustomLineupConf:      cfg.CustomLineupConf,
			BattlefieldEffects:    cfg.BattlefieldEffects,
		}
		option.NormalEnemyCount = cfg.NormalEnemyCount
		items = append(items, option)
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].ID < items[j].ID
	})
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{"items": items})
}

func (s *Server) handleBattleRoomPublicList(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	dungeonID := strings.TrimSpace(c.Query("dungeonId"))
	rooms, err := s.deps.Store.ListPublicBattleRooms(c.Context(), dungeonID, 200)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "BATTLE_ROOM_LIST_FAILED", "list battle rooms failed", err.Error())
	}
	mine, mineErr := s.deps.Store.GetBattleRoomByUser(c.Context(), userID)
	hasMine := mineErr == nil
	if mineErr != nil && !errors.Is(mineErr, store.ErrBattleRoomNotFound) {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "BATTLE_ROOM_SELF_FAILED", "load my battle room failed", mineErr.Error())
	}
	data := fiber.Map{"items": rooms}
	if hasMine {
		data["mine"] = mine
	} else {
		data["mine"] = nil
	}
	return s.writeAPISuccess(c, fiber.StatusOK, data)
}

func (s *Server) handleBattleRoomSelf(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	detail, err := s.deps.Store.GetBattleRoomByUser(c.Context(), userID)
	if errors.Is(err, store.ErrBattleRoomNotFound) {
		return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{"room": nil})
	}
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "BATTLE_ROOM_SELF_FAILED", "load my battle room failed", err.Error())
	}
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{"room": detail})
}

func (s *Server) handleBattleRoomCreate(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	var req battleRoomCreateRequest
	if err := c.BodyParser(&req); err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "BATTLE_ROOM_CREATE_INVALID_PAYLOAD", "invalid payload", err.Error())
	}
	req.DungeonID = strings.TrimSpace(req.DungeonID)
	roomName, validName := s.normalizeBattleRoomName(req.RoomName)
	if !validName {
		return s.writeAPIError(c, fiber.StatusBadRequest, "BATTLE_ROOM_NAME_INVALID", "invalid room name", "")
	}
	if err := s.rejectModeratedAPI(c, moderationRuleBattleRoomName, roomName, userID); err != nil {
		return err
	}
	if _, ok := s.battleWorldConfigByID(req.DungeonID); !ok {
		return s.writeAPIError(c, fiber.StatusBadRequest, "BATTLE_ROOM_DUNGEON_NOT_FOUND", "invalid dungeon id", "")
	}
	difficultyLevel := req.DifficultyLevel
	if difficultyLevel < 0 {
		difficultyLevel = 1
	}
	if difficultyLevel > 0 {
		hasLevel := false
		for _, lvl := range s.deps.Config.SharedGlobalSettings.CombatDifficulty.Levels {
			if lvl.Level == difficultyLevel {
				hasLevel = true
				break
			}
		}
		if !hasLevel {
			difficultyLevel = 1
		}
	}
	detail, err := s.deps.Store.CreateBattleRoom(c.Context(), userID, req.DungeonID, roomName, req.AutoReenterOnDefeat, req.IsPublic, req.IsFogMode, req.Position, difficultyLevel, BattleRoomSchemaV2)
	if err != nil {
		return s.writeBattleRoomError(c, "BATTLE_ROOM_CREATE_FAILED", err)
	}
	s.pushBattleRoomUpdated(detail, c.Context())
	return s.writeAPISuccess(c, fiber.StatusOK, detail)
}

func (s *Server) handleBattleRoomDetail(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	roomID, parseErr := parseBattleRoomID(c)
	if parseErr != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "BATTLE_ROOM_INVALID_ID", "invalid room id", "")
	}
	detail, err := s.deps.Store.GetBattleRoomDetail(c.Context(), roomID)
	if err != nil {
		return s.writeBattleRoomError(c, "BATTLE_ROOM_GET_FAILED", err)
	}
	if !detail.Room.IsPublic && !battleRoomContainsUser(detail, userID) {
		return s.writeAPIError(c, fiber.StatusForbidden, "BATTLE_ROOM_FORBIDDEN", "battle room forbidden", "")
	}
	return s.writeAPISuccess(c, fiber.StatusOK, detail)
}

func (s *Server) handleBattleRoomJoin(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	roomID, parseErr := parseBattleRoomID(c)
	if parseErr != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "BATTLE_ROOM_INVALID_ID", "invalid room id", "")
	}
	var req battleRoomJoinRequest
	_ = c.BodyParser(&req)
	detail, err := s.deps.Store.JoinBattleRoom(c.Context(), roomID, userID, req.Position)
	if err != nil {
		return s.writeBattleRoomError(c, "BATTLE_ROOM_JOIN_FAILED", err)
	}
	s.pushBattleRoomUpdated(detail, c.Context())
	return s.writeAPISuccess(c, fiber.StatusOK, detail)
}

func (s *Server) handleBattleRoomLeave(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	roomID, parseErr := parseBattleRoomID(c)
	if parseErr != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "BATTLE_ROOM_INVALID_ID", "invalid room id", "")
	}
	detail, err := s.deps.Store.LeaveBattleRoom(c.Context(), roomID, userID)
	if errors.Is(err, store.ErrBattleRoomNotFound) {
		s.notifyAllKittyStatusesForUser(c.Context(), userID)
		s.pushBattleRoomSelfSnapshot(userID, c.Context())
		s.syncAfterMutation(mutationSyncScope{UserIDs: []int64{userID}, PlayerState: true})
		return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{"left": true, "room": nil})
	}
	if err != nil {
		return s.writeBattleRoomError(c, "BATTLE_ROOM_LEAVE_FAILED", err)
	}
	s.notifyAllKittyStatusesForUser(c.Context(), userID)
	s.pushBattleRoomUpdated(detail, c.Context())
	s.pushBattleRoomSelfSnapshot(userID, c.Context())
	s.syncAfterMutation(mutationSyncScope{UserIDs: []int64{userID}, PlayerState: true})
	return s.writeAPISuccess(c, fiber.StatusOK, detail)
}

func (s *Server) notifyAllKittyStatusesForUser(ctx context.Context, userID int64) {
	if userID <= 0 {
		return
	}
	kitties, err := s.deps.Store.ListPlayerKitties(ctx, userID)
	if err != nil {
		return
	}
	changes := make([]KittyStatusChange, 0, len(kitties))
	for _, kitty := range kitties {
		if kitty.ID <= 0 {
			continue
		}
		changes = append(changes, KittyStatusChange{
			UserID:  userID,
			KittyID: kitty.ID,
			Status:  kitty.Status,
		})
	}
	s.NotifyKittyStatusesChanged(changes)
}

func (s *Server) handleBattleRoomKick(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	roomID, parseErr := parseBattleRoomID(c)
	if parseErr != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "BATTLE_ROOM_INVALID_ID", "invalid room id", "")
	}
	var req battleRoomKickRequest
	if err := c.BodyParser(&req); err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "BATTLE_ROOM_KICK_INVALID_PAYLOAD", "invalid payload", err.Error())
	}
	if req.TargetUserID <= 0 {
		return s.writeAPIError(c, fiber.StatusBadRequest, "BATTLE_ROOM_KICK_TARGET_INVALID", "invalid target user", "")
	}
	if req.TargetUserID == userID {
		return s.writeAPIError(c, fiber.StatusBadRequest, "BATTLE_ROOM_KICK_SELF_FORBIDDEN", "owner cannot kick self", "")
	}
	detail, err := s.deps.Store.KickBattleRoomMember(c.Context(), roomID, userID, req.TargetUserID)
	if err != nil {
		return s.writeBattleRoomError(c, "BATTLE_ROOM_KICK_FAILED", err)
	}
	s.pushBattleRoomUpdated(detail, c.Context())
	s.pushUserEventCluster(req.TargetUserID, "battle.room.kicked", fiber.Map{
		"roomId":       roomID,
		"kickedBy":     userID,
		"kickedUserId": req.TargetUserID,
	}, c.Context())
	s.pushBattleRoomSelfSnapshot(req.TargetUserID, c.Context())
	s.syncAfterMutation(mutationSyncScope{UserIDs: []int64{req.TargetUserID}, PlayerState: true})
	return s.writeAPISuccess(c, fiber.StatusOK, detail)
}

func (s *Server) handleBattleRoomUpdateConfig(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	roomID, parseErr := parseBattleRoomID(c)
	if parseErr != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "BATTLE_ROOM_INVALID_ID", "invalid room id", "")
	}
	var req battleRoomConfigRequest
	if err := c.BodyParser(&req); err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "BATTLE_ROOM_CONFIG_INVALID_PAYLOAD", "invalid payload", err.Error())
	}
	req.DungeonID = strings.TrimSpace(req.DungeonID)
	roomName, validName := s.normalizeBattleRoomName(req.RoomName)
	if !validName {
		return s.writeAPIError(c, fiber.StatusBadRequest, "BATTLE_ROOM_NAME_INVALID", "invalid room name", "")
	}
	if err := s.rejectModeratedAPI(c, moderationRuleBattleRoomName, roomName, userID); err != nil {
		return err
	}
	if _, ok := s.battleWorldConfigByID(req.DungeonID); !ok {
		return s.writeAPIError(c, fiber.StatusBadRequest, "BATTLE_ROOM_DUNGEON_NOT_FOUND", "invalid dungeon id", "")
	}
	difficultyLevel := req.DifficultyLevel
	if difficultyLevel < 0 {
		difficultyLevel = 1
	}
	if difficultyLevel > 0 {
		hasLevel := false
		for _, lvl := range s.deps.Config.SharedGlobalSettings.CombatDifficulty.Levels {
			if lvl.Level == difficultyLevel {
				hasLevel = true
				break
			}
		}
		if !hasLevel {
			difficultyLevel = 1
		}
	}
	detail, err := s.deps.Store.UpdateBattleRoomConfig(c.Context(), roomID, userID, req.DungeonID, roomName, req.AutoReenterOnDefeat, req.DisableEntryRound, req.IsPublic, req.IsFogMode, difficultyLevel)
	if err != nil {
		return s.writeBattleRoomError(c, "BATTLE_ROOM_CONFIG_FAILED", err)
	}
	s.pushBattleRoomUpdated(detail, c.Context())
	return s.writeAPISuccess(c, fiber.StatusOK, detail)
}

func (s *Server) handleBattleRoomTransferOwner(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	roomID, parseErr := parseBattleRoomID(c)
	if parseErr != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "BATTLE_ROOM_INVALID_ID", "invalid room id", "")
	}
	var req battleRoomTransferOwnerRequest
	if err := c.BodyParser(&req); err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "BATTLE_ROOM_TRANSFER_OWNER_INVALID_PAYLOAD", "invalid payload", err.Error())
	}
	if req.TargetUserID <= 0 || req.TargetUserID == userID {
		return s.writeAPIError(c, fiber.StatusBadRequest, "BATTLE_ROOM_TRANSFER_OWNER_TARGET_INVALID", "invalid target user", "")
	}
	detail, err := s.deps.Store.TransferBattleRoomOwner(c.Context(), roomID, userID, req.TargetUserID)
	if err != nil {
		return s.writeBattleRoomError(c, "BATTLE_ROOM_TRANSFER_OWNER_FAILED", err)
	}
	s.pushBattleRoomUpdated(detail, c.Context())
	return s.writeAPISuccess(c, fiber.StatusOK, detail)
}

func (s *Server) handleBattleRoomReady(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	roomID, parseErr := parseBattleRoomID(c)
	if parseErr != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "BATTLE_ROOM_INVALID_ID", "invalid room id", "")
	}
	var req battleRoomReadyRequest
	if err := c.BodyParser(&req); err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "BATTLE_ROOM_READY_INVALID_PAYLOAD", "invalid payload", err.Error())
	}
	detail, allReady, err := s.deps.Store.SetBattleRoomReady(c.Context(), roomID, userID, req.Ready)
	if err != nil {
		return s.writeBattleRoomError(c, "BATTLE_ROOM_READY_FAILED", err)
	}
	s.pushBattleRoomUpdated(detail, c.Context())
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{
		"room":     detail,
		"allReady": allReady,
	})
}

func (s *Server) handleBattleRoomPosition(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	roomID, parseErr := parseBattleRoomID(c)
	if parseErr != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "BATTLE_ROOM_INVALID_ID", "invalid room id", "")
	}
	var req battleRoomPositionRequest
	if err := c.BodyParser(&req); err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "BATTLE_ROOM_POSITION_INVALID_PAYLOAD", "invalid payload", err.Error())
	}
	detail, err := s.deps.Store.UpdateBattleRoomMemberPosition(c.Context(), roomID, userID, req.Position)
	if err != nil {
		return s.writeBattleRoomError(c, "BATTLE_ROOM_POSITION_FAILED", err)
	}
	s.pushBattleRoomUpdated(detail, c.Context())
	return s.writeAPISuccess(c, fiber.StatusOK, detail)
}

func (s *Server) handleBattleRoomKittyAdd(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	roomID, parseErr := parseBattleRoomID(c)
	if parseErr != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "BATTLE_ROOM_INVALID_ID", "invalid room id", "")
	}
	var req battleRoomKittyUnitRequest
	if err := c.BodyParser(&req); err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "BATTLE_ROOM_KITTY_INVALID_PAYLOAD", "invalid payload", err.Error())
	}
	detail, err := s.deps.Store.AddBattleRoomKittyUnit(c.Context(), roomID, userID, req.KittyID, req.Position)
	if err != nil {
		return s.writeBattleRoomError(c, "BATTLE_ROOM_KITTY_ADD_FAILED", err)
	}
	s.NotifyKittyStatusChanged(userID, req.KittyID, store.KittyStatusInCombat)
	s.pushBattleRoomUpdated(detail, c.Context())
	s.syncAfterMutation(mutationSyncScope{UserIDs: []int64{userID}, PlayerState: true})
	return s.writeAPISuccess(c, fiber.StatusOK, detail)
}

func (s *Server) handleBattleRoomKittyRemove(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	roomID, parseErr := parseBattleRoomID(c)
	if parseErr != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "BATTLE_ROOM_INVALID_ID", "invalid room id", "")
	}
	var req battleRoomKittyUnitRequest
	if err := c.BodyParser(&req); err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "BATTLE_ROOM_KITTY_INVALID_PAYLOAD", "invalid payload", err.Error())
	}
	detail, err := s.deps.Store.RemoveBattleRoomKittyUnit(c.Context(), roomID, userID, req.KittyID)
	if err != nil {
		return s.writeBattleRoomError(c, "BATTLE_ROOM_KITTY_REMOVE_FAILED", err)
	}
	s.NotifyKittyStatusChanged(userID, req.KittyID, store.KittyStatusIdle)
	s.pushBattleRoomUpdated(detail, c.Context())
	s.syncAfterMutation(mutationSyncScope{UserIDs: []int64{userID}, PlayerState: true})
	return s.writeAPISuccess(c, fiber.StatusOK, detail)
}

func (s *Server) handleBattleRoomInterrupt(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	roomID, parseErr := parseBattleRoomID(c)
	if parseErr != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "BATTLE_ROOM_INVALID_ID", "invalid room id", "")
	}
	var runID int64
	if run, runErr := s.deps.Store.GetBattleRunByRoomLite(c.Context(), roomID); runErr == nil {
		runID = run.ID
	}
	detail, err := s.deps.Store.InterruptBattleRoom(c.Context(), roomID, userID, time.Now())
	if err != nil {
		return s.writeBattleRoomError(c, "BATTLE_ROOM_INTERRUPT_FAILED", err)
	}
	if runID > 0 {
		s.clearBattleRuntimeState(runID)
	}
	s.pushBattleRoomUpdated(detail, c.Context())
	s.pushBattleRoomEvent(detail.Room.ID, "battle.run.interrupted", fiber.Map{
		"roomId":        detail.Room.ID,
		"interruptedBy": userID,
		"status":        detail.Room.Status,
	}, c.Context())
	memberUserIDs := make([]int64, 0, len(detail.Members))
	kittyStatusChanges := make([]KittyStatusChange, 0, len(detail.Units))
	for _, member := range detail.Members {
		if member.UserID > 0 {
			memberUserIDs = append(memberUserIDs, member.UserID)
		}
	}
	for _, unit := range detail.Units {
		if unit.OwnerUserID <= 0 || unit.UnitType != "kitty" || unit.UnitID <= 0 {
			continue
		}
		kittyStatusChanges = append(kittyStatusChanges, KittyStatusChange{
			UserID:  unit.OwnerUserID,
			KittyID: unit.UnitID,
			Status:  store.KittyStatusIdle,
		})
	}
	s.NotifyKittyStatusesChanged(kittyStatusChanges)
	if len(memberUserIDs) > 0 {
		s.syncAfterMutation(mutationSyncScope{UserIDs: memberUserIDs, PlayerState: true})
	}
	return s.writeAPISuccess(c, fiber.StatusOK, detail)
}

func (s *Server) handleBattleRoomRun(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	roomID, parseErr := parseBattleRoomID(c)
	if parseErr != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "BATTLE_ROOM_INVALID_ID", "invalid room id", "")
	}
	detail, err := s.deps.Store.GetBattleRoomDetail(c.Context(), roomID)
	if err != nil {
		return s.writeBattleRoomError(c, "BATTLE_ROOM_GET_FAILED", err)
	}
	if !battleRoomContainsUser(detail, userID) {
		return s.writeAPIError(c, fiber.StatusForbidden, "BATTLE_ROOM_FORBIDDEN", "battle room forbidden", "")
	}
	run, err := s.deps.Store.GetBattleRunByRoomLite(c.Context(), roomID)
	if err != nil {
		if errors.Is(err, store.ErrBattleRoomNotFound) {
			return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{"run": nil})
		}
		return s.writeAPIError(c, fiber.StatusInternalServerError, "BATTLE_RUN_GET_FAILED", "load battle run failed", err.Error())
	}
	if liveState, ok := s.battleRuntimeState(run.ID); ok && run.Status == store.BattleRunStatusRunning {
		run.State = liveState.toMap()
	} else if run.Status == store.BattleRunStatusRunning {
		if state, ok := s.battlePlaybackSyncState(c.Context(), detail, run, time.Now()); ok {
			run.State = state
		}
	} else if run.ID > 0 {
		// The gateway may cache running playback state for "mid-join" snapshots.
		// Once the run is no longer running, eagerly clear local cache to avoid buildup.
		s.clearBattleRuntimeState(run.ID)
	}
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{"run": run})
}

func (s *Server) handleBattlePointsGet(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	state, err := s.deps.Store.GetBattlePointState(c.Context(), userID)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "BATTLE_POINT_GET_FAILED", "load battle points failed", err.Error())
	}
	return s.writeAPISuccess(c, fiber.StatusOK, s.battlePointStatePayload(state))
}

func (s *Server) handleBattlePointsAllocate(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	var req battlePointAllocateRequest
	if err := c.BodyParser(&req); err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "BATTLE_POINT_ALLOCATE_INVALID_PAYLOAD", "invalid payload", err.Error())
	}
	req.AttrKey = strings.TrimSpace(req.AttrKey)
	if req.AttrKey == "" || req.Points <= 0 {
		return s.writeAPIError(c, fiber.StatusBadRequest, "BATTLE_POINT_ALLOCATE_INVALID_REQUEST", "invalid allocate request", "")
	}
	allocations, normalizeErr := normalizeBattlePointAllocations(map[string]int64{req.AttrKey: req.Points})
	if normalizeErr != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "BATTLE_POINT_ALLOCATE_INVALID_REQUEST", "invalid allocate request", "")
	}
	state, err := s.deps.Store.AllocateBattlePointsBatch(c.Context(), userID, allocations)
	if err != nil {
		switch {
		case errors.Is(err, store.ErrBattlePointInsufficient):
			return s.writeAPIError(c, fiber.StatusConflict, "BATTLE_POINT_INSUFFICIENT", "insufficient battle points", "")
		case errors.Is(err, store.ErrBattlePointAttrNotAllowed):
			return s.writeAPIError(c, fiber.StatusBadRequest, "BATTLE_POINT_ATTR_NOT_ALLOWED", "attribute does not allow battle points", "")
		default:
			return s.writeAPIError(c, fiber.StatusInternalServerError, "BATTLE_POINT_ALLOCATE_FAILED", "allocate battle points failed", err.Error())
		}
	}
	s.syncAfterMutation(mutationSyncScope{
		UserIDs:    []int64{userID},
		Attributes: true,
	})
	return s.writeAPISuccess(c, fiber.StatusOK, s.battlePointStatePayload(state))
}

func (s *Server) handleBattlePointsAllocateBatch(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	var req battlePointAllocateBatchRequest
	if err := c.BodyParser(&req); err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "BATTLE_POINT_ALLOCATE_BATCH_INVALID_PAYLOAD", "invalid payload", err.Error())
	}
	if len(req.Allocations) == 0 {
		return s.writeAPIError(c, fiber.StatusBadRequest, "BATTLE_POINT_ALLOCATE_BATCH_INVALID_REQUEST", "invalid allocate request", "")
	}
	normalizedAllocations, normalizeErr := normalizeBattlePointAllocations(req.Allocations)
	if normalizeErr != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "BATTLE_POINT_ALLOCATE_BATCH_INVALID_REQUEST", "invalid allocate request", "")
	}
	state, err := s.deps.Store.AllocateBattlePointsBatch(c.Context(), userID, normalizedAllocations)
	if err != nil {
		switch {
		case errors.Is(err, store.ErrBattlePointInsufficient):
			return s.writeAPIError(c, fiber.StatusConflict, "BATTLE_POINT_INSUFFICIENT", "insufficient battle points", "")
		case errors.Is(err, store.ErrBattlePointAttrNotAllowed):
			return s.writeAPIError(c, fiber.StatusBadRequest, "BATTLE_POINT_ATTR_NOT_ALLOWED", "attribute does not allow battle points", "")
		case errors.Is(err, store.ErrBattleRoomConfigInvalid):
			return s.writeAPIError(c, fiber.StatusBadRequest, "BATTLE_POINT_ALLOCATE_BATCH_INVALID_REQUEST", "invalid allocate request", "")
		default:
			return s.writeAPIError(c, fiber.StatusInternalServerError, "BATTLE_POINT_ALLOCATE_BATCH_FAILED", "allocate battle points failed", err.Error())
		}
	}
	s.syncAfterMutation(mutationSyncScope{
		UserIDs:    []int64{userID},
		Attributes: true,
	})
	return s.writeAPISuccess(c, fiber.StatusOK, s.battlePointStatePayload(state))
}

func (s *Server) handleBattlePointsReset(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	state, err := s.deps.Store.ResetBattlePoints(c.Context(), userID)
	if err != nil {
		switch {
		case errors.Is(err, store.ErrInsufficientItem):
			return s.writeAPIError(c, fiber.StatusConflict, "BATTLE_POINT_RESET_ITEM_INSUFFICIENT", "insufficient reset item", "")
		default:
			return s.writeAPIError(c, fiber.StatusInternalServerError, "BATTLE_POINT_RESET_FAILED", "reset battle points failed", err.Error())
		}
	}
	s.syncAfterMutation(mutationSyncScope{
		UserIDs:    []int64{userID},
		Inventory:  true,
		Equipment:  true,
		Attributes: true,
	})
	return s.writeAPISuccess(c, fiber.StatusOK, s.battlePointStatePayload(state))
}

func (s *Server) battlePointStatePayload(state store.PlayerBattlePointState) fiber.Map {
	available := state.TotalPoints - state.SpentPoints
	if available < 0 {
		available = 0
	}
	resetRequirement := fiber.Map{
		"itemId": strings.TrimSpace(s.deps.Config.GlobalSettings.Battle.ResetItemID),
		"count":  s.deps.Config.GlobalSettings.Battle.ResetItemCount,
	}
	return fiber.Map{
		"userId":           state.UserID,
		"totalPoints":      state.TotalPoints,
		"spentPoints":      state.SpentPoints,
		"available":        available,
		"resetRequirement": resetRequirement,
	}
}

func (s *Server) writeBattleRoomError(c *fiber.Ctx, fallbackCode string, err error) error {
	var teamSizeErr *store.BattleRoomTeamSizeExceededError
	switch {
	case errors.Is(err, store.ErrBattleRoomNotFound):
		return s.writeAPIError(c, fiber.StatusNotFound, "BATTLE_ROOM_NOT_FOUND", "battle room not found", "")
	case errors.Is(err, store.ErrBattleRoomFull):
		return s.writeAPIError(c, fiber.StatusConflict, "BATTLE_ROOM_FULL", "battle room is full", "")
	case errors.Is(err, store.ErrBattleRoomForbidden):
		return s.writeAPIError(c, fiber.StatusForbidden, "BATTLE_ROOM_FORBIDDEN", "battle room forbidden", "")
	case errors.Is(err, store.ErrBattleRoomNotOwner):
		return s.writeAPIError(c, fiber.StatusForbidden, "BATTLE_ROOM_NOT_OWNER", "only owner can modify room", "")
	case errors.Is(err, store.ErrBattleRoomAlreadyMember):
		return s.writeAPIError(c, fiber.StatusConflict, "BATTLE_ROOM_ALREADY_MEMBER", "already in battle room", "")
	case errors.Is(err, store.ErrBattleRoomMemberMissing):
		return s.writeAPIError(c, fiber.StatusNotFound, "BATTLE_ROOM_MEMBER_MISSING", "battle room member missing", "")
	case errors.Is(err, store.ErrBattleRoomStateInvalid):
		return s.writeAPIError(c, fiber.StatusConflict, "BATTLE_ROOM_STATE_INVALID", "battle room state invalid", "")
	case errors.Is(err, store.ErrBattleRoomConfigInvalid):
		return s.writeAPIError(c, fiber.StatusBadRequest, "BATTLE_ROOM_CONFIG_INVALID", "battle room config invalid", "")
	case errors.Is(err, store.ErrBattleRoomTransferTargetInvalid):
		return s.writeAPIError(c, fiber.StatusBadRequest, "BATTLE_ROOM_TRANSFER_OWNER_TARGET_INVALID", "battle room transfer target invalid", "")
	case errors.Is(err, store.ErrKittyNotFound):
		return s.writeAPIError(c, fiber.StatusNotFound, "CATTERY_KITTY_NOT_FOUND", "kitty not found", "")
	case errors.Is(err, store.ErrKittyModeConflictLifeQueue):
		return s.writeAPIError(c, fiber.StatusConflict, "KITTY_MODE_CONFLICT_LIFE_QUEUE", "kitty is in combat", "")
	case errors.Is(err, store.ErrKittyModeConflictCombat):
		return s.writeAPIError(c, fiber.StatusConflict, "KITTY_MODE_CONFLICT_COMBAT", "kitty is in life queue", "")
	case errors.As(err, &teamSizeErr):
		return s.writeAPIError(
			c,
			fiber.StatusConflict,
			"BATTLE_ROOM_DUNGEON_TEAM_SIZE_EXCEEDED",
			"battle room member count exceeds dungeon max team size",
			fmt.Sprintf("member_count=%d,max_team_size=%d", teamSizeErr.MemberCount, teamSizeErr.MaxTeamSize),
		)
	case errors.Is(err, store.ErrInsufficientItem):
		return s.writeAPIError(c, fiber.StatusConflict, "BATTLE_ROOM_ENTRY_COST_INSUFFICIENT", "entry cost insufficient", "")
	default:
		return s.writeAPIError(c, fiber.StatusInternalServerError, fallbackCode, "battle room operation failed", err.Error())
	}
}

func (s *Server) normalizeBattleRoomName(raw string) (string, bool) {
	name := strings.TrimSpace(raw)
	maxChars := s.deps.Config.GlobalSettings.Battle.RoomNameMaxChars
	if maxChars <= 0 {
		maxChars = 12
	}
	if name == "" || len([]rune(name)) > maxChars {
		return "", false
	}
	for _, r := range name {
		if unicode.IsSpace(r) {
			return "", false
		}
		if r >= '0' && r <= '9' {
			continue
		}
		if unicode.In(r, unicode.Han, unicode.Latin, unicode.Hiragana, unicode.Katakana, unicode.Hangul) {
			continue
		}
		return "", false
	}
	return name, true
}

func parseBattleRoomID(c *fiber.Ctx) (int64, error) {
	return strconv.ParseInt(strings.TrimSpace(c.Params("roomID")), 10, 64)
}

func battleRoomContainsUser(detail store.BattleRoomDetail, userID int64) bool {
	for _, member := range detail.Members {
		if member.UserID == userID {
			return true
		}
	}
	return false
}

func (s *Server) pushBattleRoomUpdated(detail store.BattleRoomDetail, ctxOpt ...context.Context) {
	if detail.Room.ID <= 0 {
		return
	}
	s.invalidateBattleRoomDetailCache(detail.Room.ID)
	s.invalidateBattleRoomMemberIDsCache(detail.Room.ID)
	s.pushBattleRoomEventToUsers(battleRoomMemberUserIDs(detail.Members), "battle.room.updated", detail, ctxOpt...)
	for _, member := range detail.Members {
		s.pushUserEventCluster(member.UserID, "battle.room.self", detail, ctxOpt...)
	}
}

func (s *Server) pushBattleRoomSelfSnapshot(userID int64, ctxOpt ...context.Context) {
	if userID <= 0 {
		return
	}
	detail, err := s.deps.Store.GetBattleRoomByUser(context.Background(), userID)
	if err == nil {
		s.pushUserEventCluster(userID, "battle.room.self", detail, ctxOpt...)
		return
	}
	if errors.Is(err, store.ErrBattleRoomNotFound) {
		s.pushUserEventCluster(userID, "battle.room.self", fiber.Map{"room": nil}, ctxOpt...)
	}
}

func (s *Server) pushBattleRoomEvent(roomID int64, eventType string, payload any, ctxOpt ...context.Context) {
	detail, err := s.loadBattleRoomDetailHot(context.Background(), roomID)
	if err != nil {
		return
	}
	s.pushBattleRoomEventByMembers(detail.Members, eventType, payload, ctxOpt...)
}

func (s *Server) loadBattleRoomDetailHot(ctx context.Context, roomID int64) (store.BattleRoomDetail, error) {
	if roomID <= 0 {
		return store.BattleRoomDetail{}, store.ErrBattleRoomNotFound
	}
	now := time.Now()
	s.battleRoomDetailMu.RLock()
	cached, ok := s.battleRoomDetailMem[roomID]
	s.battleRoomDetailMu.RUnlock()
	if ok && now.Before(cached.ExpiresAt) && !battleRoomDetailCacheBypass(cached.Detail) {
		s.battleMetrics.AddRoomDetail(0, true)
		return cached.Detail, nil
	}
	return s.loadBattleRoomDetailFresh(ctx, roomID)
}

func (s *Server) loadBattleRoomDetailFresh(ctx context.Context, roomID int64) (store.BattleRoomDetail, error) {
	if roomID <= 0 {
		return store.BattleRoomDetail{}, store.ErrBattleRoomNotFound
	}
	startedAt := time.Now()
	detail, err := s.deps.Store.GetBattleRoomDetailHot(ctx, roomID)
	if err != nil {
		return store.BattleRoomDetail{}, err
	}
	if !battleRoomDetailCacheBypass(detail) {
		s.setBattleRoomDetailCache(roomID, detail)
	} else {
		s.invalidateBattleRoomDetailCache(roomID)
	}
	s.battleMetrics.AddRoomDetail(time.Since(startedAt), false)
	return detail, nil
}

func battleRoomDetailCacheBypass(detail store.BattleRoomDetail) bool {
	switch detail.Room.Status {
	case store.BattleRoomStatusEntering, store.BattleRoomStatusPlaying, store.BattleRoomStatusRecovering:
		return true
	default:
		return detail.Room.CurrentRunID != nil
	}
}

func (s *Server) setBattleRoomDetailCache(roomID int64, detail store.BattleRoomDetail) {
	if roomID <= 0 {
		return
	}
	s.battleRoomDetailMu.Lock()
	s.battleRoomDetailMem[roomID] = battleRoomDetailCacheEntry{
		Detail:    detail,
		CachedAt:  time.Now(),
		ExpiresAt: time.Now().Add(2 * time.Second),
	}
	s.battleRoomDetailMu.Unlock()
}

func (s *Server) invalidateBattleRoomDetailCache(roomID int64) {
	if roomID <= 0 {
		return
	}
	s.battleRoomDetailMu.Lock()
	delete(s.battleRoomDetailMem, roomID)
	s.battleRoomDetailMu.Unlock()
}

const battleRoomMemberIDsCacheTTL = 2 * time.Minute

func battleRoomMemberIDsCacheKey(roomID int64) string {
	return fmt.Sprintf("battle:room:%d:mids", roomID)
}

func (s *Server) loadBattleRoomMemberIDsCached(ctx context.Context, roomID int64) ([]int64, error) {
	if s.deps.Redis != nil {
		val, err := s.deps.Redis.Get(ctx, battleRoomMemberIDsCacheKey(roomID)).Result()
		if err == nil {
			var ids []int64
			if json.Unmarshal([]byte(val), &ids) == nil {
				return ids, nil
			}
		} else if !errors.Is(err, redis.Nil) {
			_ = err
		}
	}
	ids, err := s.deps.Store.GetBattleRoomMemberUserIDs(ctx, roomID)
	if err != nil {
		return nil, err
	}
	if s.deps.Redis != nil {
		if buf, merr := json.Marshal(ids); merr == nil {
			_ = s.deps.Redis.Set(ctx, battleRoomMemberIDsCacheKey(roomID), string(buf), battleRoomMemberIDsCacheTTL).Err()
		}
	}
	return ids, nil
}

func (s *Server) invalidateBattleRoomMemberIDsCache(roomID int64) {
	if roomID <= 0 || s.deps.Redis == nil {
		return
	}
	_ = s.deps.Redis.Del(context.Background(), battleRoomMemberIDsCacheKey(roomID)).Err()
}

func (s *Server) pushBattleRoomEventByMembers(members []store.BattleRoomMember, eventType string, payload any, ctxOpt ...context.Context) bool {
	return s.pushBattleRoomEventToUsers(battleRoomMemberUserIDs(members), eventType, payload, ctxOpt...)
}

func (s *Server) pushBattleRoomEventToUsers(userIDs []int64, eventType string, payload any, ctxOpt ...context.Context) bool {
	targets := compactUserIDs(userIDs)
	if len(targets) == 0 {
		return true
	}
	return s.publishWSTargetUsers(eventType, targets, payload, "", "", ctxOpt...)
}

func battleRoomMemberUserIDs(members []store.BattleRoomMember) []int64 {
	if len(members) == 0 {
		return nil
	}
	out := make([]int64, 0, len(members))
	seen := make(map[int64]struct{}, len(members))
	for _, member := range members {
		if member.UserID <= 0 {
			continue
		}
		if _, ok := seen[member.UserID]; ok {
			continue
		}
		seen[member.UserID] = struct{}{}
		out = append(out, member.UserID)
	}
	return out
}

type battleWorldDifficultySettings struct {
	MaxLevel     int                      `json:"maxLevel"`
	StatBonuses  []battleWorldStatBonus   `json:"statBonuses,omitempty"`
	SpecialDrops []battleWorldSpecialDrop `json:"specialDrops,omitempty"`
}

type battleWorldStatBonus struct {
	AttrKey      string  `json:"attrKey"`
	Base         float64 `json:"base"`
	PerLevelMult float64 `json:"perLevelMult"`
}

type battleWorldSpecialDrop struct {
	ItemID   string  `json:"itemId"`
	Percent  float64 `json:"percent"`
	RangeMin int64   `json:"rangeMin,omitempty"`
	RangeMax int64   `json:"rangeMax,omitempty"`
	MinLevel int     `json:"minLevel"`
}

type battleWorldConfig struct {
	ID                    string
	FogMapItemID          string
	EnemyIDs              []string
	SpecialEnemyIDs       []string
	SpecialWaves          map[int]struct{}
	WaveSchedules         []battleWaveSchedule
	SpecialWaveEvery      int
	SpecialWaveMinCount   int
	SpecialWaveMaxCount   int
	EntryDurationSeconds  int64
	MaxTeamSize           int
	EntryCosts            map[string]int64
	NormalEnemyCount      int
	NormalEnemyCountRange battleCountRange
	ExtraEnemyPerPlayer   battleCountRange
	CustomLineupConf      []battleCustomLineupConfig
	WaveDrops             map[string]battleDropRule // extra drops rolled once per wave clear, per player
	DifficultySettings    *battleWorldDifficultySettings
	BattlefieldEffects    []battleWorldBattlefieldEffect
}

type battleWorldBattlefieldEffect struct {
	ID                   string                                 `json:"id"`
	NameKey              string                                 `json:"nameKey,omitempty"`
	DescKey              string                                 `json:"descKey,omitempty"`
	Icon                 string                                 `json:"icon,omitempty"`
	Counter              battleWorldBattlefieldCounter          `json:"counter"`
	TriggerEffects       []battleWorldBattlefieldTriggerEffect  `json:"triggerEffects"`
	PostTriggerWindows   []battleWorldBattlefieldWindow         `json:"postTriggerWindows,omitempty"`
	ConditionalModifiers []battleWorldBattlefieldConditionalMod `json:"conditionalModifiers,omitempty"`
}

type battleWorldBattlefieldCounter struct {
	Source                   string                            `json:"source"`
	ActorFilter              battleWorldBattlefieldActorFilter `json:"actorFilter"`
	Aggregation              string                            `json:"aggregation"`
	CountSuppressedTurns     bool                              `json:"countSuppressedTurns"`
	Initial                  int                               `json:"initial"`
	ResetTo                  int                               `json:"resetTo"`
	CarryProgressAcrossWaves bool                              `json:"carryProgressAcrossWaves"`
}

type battleWorldBattlefieldActorFilter struct {
	Sides              []string `json:"sides"`
	UnitTypes          []string `json:"unitTypes"`
	IncludeSummons     bool     `json:"includeSummons"`
	TurnKinds          []string `json:"turnKinds"`
	IncludePreparation bool     `json:"includePreparation"`
}

type battleWorldBattlefieldTriggerEffect struct {
	Kind               string                                `json:"kind"`
	TargetScope        string                                `json:"targetScope"`
	IncludeSummons     bool                                  `json:"includeSummons"`
	DamageType         string                                `json:"damageType"`
	AmountByDifficulty battleWorldBattlefieldDifficultyCurve `json:"amountByDifficulty"`
	TargetFilter       battleWorldBattlefieldTargetFilter    `json:"targetFilter,omitempty"`
	TargetCount        battleWorldBattlefieldTargetCount     `json:"targetCount,omitempty"`
	Selection          string                                `json:"selection,omitempty"`
	Choices            []battleWorldBattlefieldChoice        `json:"choices,omitempty"`
}

type battleWorldBattlefieldTargetFilter struct {
	UnitTypes      []string `json:"unitTypes,omitempty"`
	IncludeSummons bool     `json:"includeSummons"`
}

type battleWorldBattlefieldTargetCount struct {
	Source   string `json:"source"`
	Divisor  int    `json:"divisor"`
	Rounding string `json:"rounding"`
	Min      int    `json:"min"`
	Max      int    `json:"max"`
}

type battleWorldBattlefieldChoice struct {
	ID     string                             `json:"id"`
	Weight float64                            `json:"weight"`
	Effect battleWorldBattlefieldChoiceEffect `json:"effect"`
}

type battleWorldBattlefieldChoiceEffect struct {
	Kind            string   `json:"kind"`
	StatusID        string   `json:"statusId,omitempty"`
	DurationTurns   int      `json:"durationTurns,omitempty"`
	Amount          int      `json:"amount,omitempty"`
	IncludeBasic    bool     `json:"includeBasic,omitempty"`
	ExcludeSkillIDs []string `json:"excludeSkillIds,omitempty"`
}

type battleWorldBattlefieldDifficultyCurve struct {
	Base         float64 `json:"base"`
	PerLevelMult float64 `json:"perLevelMult"`
}

type battleWorldBattlefieldWindow struct {
	ID       string `json:"id"`
	Duration int    `json:"duration"`
	Unit     string `json:"unit"`
}

type battleWorldBattlefieldConditionalMod struct {
	Kind                 string  `json:"kind"`
	TargetScope          string  `json:"targetScope"`
	Value                float64 `json:"value"`
	InactiveDuringWindow string  `json:"inactiveDuringWindow"`
}

type battleCountRange struct {
	Min int `json:"min"`
	Max int `json:"max"`
}

type battleCustomLineupConfig struct {
	ID                  string                  `json:"id"`
	NameKey             string                  `json:"nameKey,omitempty"`
	DescKey             string                  `json:"descKey,omitempty"`
	Percent             float64                 `json:"percent"`
	IsSpecialWaveLineup bool                    `json:"isSpecialWaveLineup"`
	Lineup              []string                `json:"lineup"`
	Drops               battleCustomLineupDrops `json:"drops"`
}

type battleCustomLineupDrops struct {
	Items map[string]battleDropRule `json:"items"`
}

func (s *Server) battleWorldConfigByID(dungeonID string) (battleWorldConfig, bool) {
	dungeonID = strings.TrimSpace(dungeonID)
	if dungeonID == "" {
		return battleWorldConfig{}, false
	}
	s.battleDataMu.RLock()
	if cached, ok := s.battleWorldCache[dungeonID]; ok {
		s.battleDataMu.RUnlock()
		return cached, true
	}
	s.battleDataMu.RUnlock()
	enabledEnemyIDs := s.enabledEnemyIDSet()
	for _, raw := range s.deps.GameData.World {
		cfg, ok := normalizedBattleWorldConfigRow(raw, enabledEnemyIDs)
		if !ok {
			continue
		}
		if cfg.ID != dungeonID {
			continue
		}
		s.battleDataMu.Lock()
		s.battleWorldCache[dungeonID] = cfg
		s.battleDataMu.Unlock()
		return cfg, true
	}
	return battleWorldConfig{}, false
}

func normalizedBattleWorldConfigRow(row map[string]any, enabledEnemyIDs map[string]struct{}) (battleWorldConfig, bool) {
	if isCatalogRowDisabled(row) {
		return battleWorldConfig{}, false
	}
	id, _ := stringFromAny(row["id"])
	id = strings.TrimSpace(id)
	if id == "" {
		return battleWorldConfig{}, false
	}
	specialCfg := parseBattleSpecialWaveConfig(row["specialWaves"])
	enemyIDs := filterEnabledIDs(toStringSlice(row["enemyIds"]), enabledEnemyIDs)
	specialEnemyIDs := filterEnabledIDs(toStringSlice(row["specialEnemyIds"]), enabledEnemyIDs)
	waveSchedules := parseBattleWaveSchedules(row["waveSchedules"], enabledEnemyIDs)
	customLineups := filterBattleCustomLineupConf(parseBattleCustomLineupConf(row["customLineupConf"]), enabledEnemyIDs)
	if len(enemyIDs) == 0 && len(specialEnemyIDs) == 0 && len(customLineups) == 0 {
		return battleWorldConfig{}, false
	}
	fogMapItemID, _ := stringFromAny(row["fogMapItemId"])
	cfg := battleWorldConfig{
		ID:                    id,
		FogMapItemID:          strings.TrimSpace(fogMapItemID),
		EnemyIDs:              enemyIDs,
		SpecialEnemyIDs:       specialEnemyIDs,
		SpecialWaves:          specialCfg.FixedWaves,
		WaveSchedules:         waveSchedules,
		SpecialWaveEvery:      specialCfg.Every,
		SpecialWaveMinCount:   specialCfg.MinCount,
		SpecialWaveMaxCount:   specialCfg.MaxCount,
		EntryDurationSeconds:  toInt64Loose(row["entryDurationSeconds"], 0),
		MaxTeamSize:           parseBattleMaxTeamSize(row["maxTeamSize"]),
		EntryCosts:            toInt64Map(row["entryCosts"]),
		NormalEnemyCountRange: parseBattleCountRange(row["normalEnemyCount"], battleCountRange{}),
		ExtraEnemyPerPlayer:   parseBattleCountRange(row["extraEnemyPerPlayer"], battleCountRange{}),
		DifficultySettings:    parseWorldDifficultySettings(row["difficultySettings"]),
		CustomLineupConf:      customLineups,
		WaveDrops:             parseBattleDropRules(row["waveDrops"]),
		BattlefieldEffects:    parseBattleWorldBattlefieldEffects(row["battlefieldEffects"]),
	}
	cfg.NormalEnemyCount = cfg.NormalEnemyCountRange.Min
	if cfg.NormalEnemyCount < 0 {
		cfg.NormalEnemyCount = 0
	}
	return cfg, true
}

func parseBattleWorldBattlefieldEffects(raw any) []battleWorldBattlefieldEffect {
	if raw == nil {
		return nil
	}
	buf, err := json.Marshal(raw)
	if err != nil {
		return nil
	}
	var parsed []battleWorldBattlefieldEffect
	if err := json.Unmarshal(buf, &parsed); err != nil {
		return nil
	}
	out := make([]battleWorldBattlefieldEffect, 0, len(parsed))
	seen := make(map[string]struct{}, len(parsed))
	for _, effect := range parsed {
		effect.ID = strings.TrimSpace(effect.ID)
		effect.Counter.Source = strings.ToLower(strings.TrimSpace(effect.Counter.Source))
		effect.Counter.Aggregation = strings.ToLower(strings.TrimSpace(effect.Counter.Aggregation))
		for index := range effect.TriggerEffects {
			trigger := &effect.TriggerEffects[index]
			trigger.Kind = strings.ToLower(strings.TrimSpace(trigger.Kind))
			trigger.TargetScope = strings.ToLower(strings.TrimSpace(trigger.TargetScope))
			trigger.Selection = strings.ToLower(strings.TrimSpace(trigger.Selection))
			trigger.TargetCount.Source = strings.ToLower(strings.TrimSpace(trigger.TargetCount.Source))
			trigger.TargetCount.Rounding = strings.ToLower(strings.TrimSpace(trigger.TargetCount.Rounding))
			for choiceIndex := range trigger.Choices {
				choice := &trigger.Choices[choiceIndex]
				choice.ID = strings.TrimSpace(choice.ID)
				choice.Effect.Kind = strings.ToLower(strings.TrimSpace(choice.Effect.Kind))
				choice.Effect.StatusID = strings.TrimSpace(choice.Effect.StatusID)
			}
		}
		if effect.ID == "" || effect.Counter.Source != "turn_consumed" || effect.Counter.Initial <= 0 || effect.Counter.ResetTo <= 0 {
			continue
		}
		if effect.Counter.Aggregation != "per_event" && effect.Counter.Aggregation != "alive_roster_cycle" {
			continue
		}
		if _, exists := seen[effect.ID]; exists {
			continue
		}
		seen[effect.ID] = struct{}{}
		out = append(out, effect)
	}
	return out
}

func sortedSpecialWaves(source map[int]struct{}) []int {
	specialWaves := make([]int, 0, len(source))
	for wave := range source {
		specialWaves = append(specialWaves, wave)
	}
	sort.Ints(specialWaves)
	return specialWaves
}

func stringFromAny(value any) (string, bool) {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed), true
	default:
		return "", false
	}
}

func toStringSlice(value any) []string {
	switch typed := value.(type) {
	case []any:
		out := make([]string, 0, len(typed))
		for _, entry := range typed {
			if str, ok := entry.(string); ok {
				str = strings.TrimSpace(str)
				if str != "" {
					out = append(out, str)
				}
			}
		}
		return out
	case []string:
		out := make([]string, 0, len(typed))
		for _, entry := range typed {
			entry = strings.TrimSpace(entry)
			if entry != "" {
				out = append(out, entry)
			}
		}
		return out
	case string:
		parts := strings.Split(typed, ",")
		out := make([]string, 0, len(parts))
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part != "" {
				out = append(out, part)
			}
		}
		return out
	default:
		return []string{}
	}
}

func isCatalogRowDisabled(row map[string]any) bool {
	if row == nil {
		return false
	}
	return toBoolLoose(row["disabled"], false)
}

func (s *Server) enabledEnemyIDSet() map[string]struct{} {
	out := make(map[string]struct{}, len(s.deps.GameData.Enemies))
	for _, raw := range s.deps.GameData.Enemies {
		if isCatalogRowDisabled(raw) {
			continue
		}
		id, _ := stringFromAny(raw["id"])
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		out[id] = struct{}{}
	}
	return out
}

func filterEnabledIDs(ids []string, enabled map[string]struct{}) []string {
	if len(ids) == 0 {
		return []string{}
	}
	if len(enabled) == 0 {
		return []string{}
	}
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		normalized := strings.TrimSpace(id)
		if normalized == "" {
			continue
		}
		if _, ok := enabled[normalized]; !ok {
			continue
		}
		out = append(out, normalized)
	}
	return out
}

func filterBattleCustomLineupConf(rows []battleCustomLineupConfig, enabled map[string]struct{}) []battleCustomLineupConfig {
	if len(rows) == 0 {
		return []battleCustomLineupConfig{}
	}
	out := make([]battleCustomLineupConfig, 0, len(rows))
	for _, row := range rows {
		filteredLineup := filterEnabledIDs(row.Lineup, enabled)
		if len(filteredLineup) == 0 {
			continue
		}
		next := row
		next.Lineup = filteredLineup
		out = append(out, next)
	}
	return out
}

func toIntSet(value any) map[int]struct{} {
	out := map[int]struct{}{}
	switch typed := value.(type) {
	case []any:
		for _, entry := range typed {
			n := int(toInt64Loose(entry, 0))
			if n > 0 {
				out[n] = struct{}{}
			}
		}
	case []int:
		for _, n := range typed {
			if n > 0 {
				out[n] = struct{}{}
			}
		}
	case []int64:
		for _, n := range typed {
			if n > 0 {
				out[int(n)] = struct{}{}
			}
		}
	case string:
		for _, part := range strings.Split(typed, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			n, _ := strconv.Atoi(part)
			if n > 0 {
				out[n] = struct{}{}
			}
		}
	}
	return out
}

func parseBattleSpecialWaveConfig(value any) battleSpecialWaveConfig {
	cfg := battleSpecialWaveConfig{
		FixedWaves: toIntSet(value),
	}
	node, ok := value.(map[string]any)
	if !ok {
		return cfg
	}
	every := int(toInt64Loose(node["every"], 0))
	if every <= 0 {
		return cfg
	}
	minCount := int(toInt64Loose(node["minCount"], 1))
	maxCount := int(toInt64Loose(node["maxCount"], int64(minCount)))
	if minCount <= 0 {
		minCount = 1
	}
	if maxCount < minCount {
		maxCount = minCount
	}
	cfg.Every = every
	cfg.MinCount = minCount
	cfg.MaxCount = maxCount
	return cfg
}

func parseBattleWaveSchedules(value any, enabledEnemyIDs map[string]struct{}) []battleWaveSchedule {
	if value == nil {
		return nil
	}
	buf, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	var parsed []battleWaveSchedule
	if err := json.Unmarshal(buf, &parsed); err != nil {
		return nil
	}
	out := make([]battleWaveSchedule, 0, len(parsed))
	seen := make(map[string]struct{}, len(parsed))
	for _, schedule := range parsed {
		schedule.ID = strings.TrimSpace(schedule.ID)
		schedule.NameKey = strings.TrimSpace(schedule.NameKey)
		schedule.DescKey = strings.TrimSpace(schedule.DescKey)
		if schedule.ID == "" || schedule.Every <= 0 {
			continue
		}
		if _, exists := seen[schedule.ID]; exists {
			continue
		}
		choices := make([]battleWaveScheduleEnemyChoice, 0, len(schedule.EnemyChoices))
		for _, choice := range schedule.EnemyChoices {
			choice.EnemyID = strings.TrimSpace(choice.EnemyID)
			if choice.EnemyID == "" || choice.Weight <= 0 {
				continue
			}
			if len(enabledEnemyIDs) > 0 {
				if _, enabled := enabledEnemyIDs[choice.EnemyID]; !enabled {
					continue
				}
			}
			choices = append(choices, choice)
		}
		if len(choices) == 0 {
			continue
		}
		if schedule.MinCount <= 0 {
			schedule.MinCount = 1
		}
		if schedule.MaxCount < schedule.MinCount {
			schedule.MaxCount = schedule.MinCount
		}
		schedule.EnemyChoices = choices
		seen[schedule.ID] = struct{}{}
		out = append(out, schedule)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Priority != out[j].Priority {
			return out[i].Priority > out[j].Priority
		}
		return out[i].ID < out[j].ID
	})
	return out
}

func toInt64Loose(value any, fallback int64) int64 {
	switch typed := value.(type) {
	case int:
		return int64(typed)
	case int32:
		return int64(typed)
	case int64:
		return typed
	case float32:
		return int64(typed)
	case float64:
		return int64(typed)
	case string:
		typed = strings.TrimSpace(typed)
		if typed == "" {
			return fallback
		}
		num, err := strconv.ParseInt(typed, 10, 64)
		if err != nil {
			return fallback
		}
		return num
	default:
		return fallback
	}
}

func (s *Server) battleDifficultyStatMultiplier(level int) float64 {
	for _, lvl := range s.deps.Config.SharedGlobalSettings.CombatDifficulty.Levels {
		if lvl.Level == level {
			return resolveCurveValue(lvl.StatMultiplierCurve, float64(lvl.Level))
		}
	}
	return 1.0
}

func resolveCurveValue(curve map[string]any, level float64) float64 {
	if curve == nil {
		return 1.0
	}
	base := toFloat64(curve["base"], 1.0)
	step := toFloat64(curve["step"], 0)
	if overrides, ok := curve["overrides"].(map[string]any); ok {
		key := strconv.Itoa(int(level))
		if overrideVal, ok := overrides[key]; ok {
			return toFloat64(overrideVal, base)
		}
	}
	return base + step*(level-1)
}

func parseWorldDifficultySettings(raw any) *battleWorldDifficultySettings {
	if raw == nil {
		return nil
	}
	m, ok := raw.(map[string]any)
	if !ok || len(m) == 0 {
		return nil
	}
	out := &battleWorldDifficultySettings{MaxLevel: 10}
	if ml, e := m["maxLevel"]; e {
		out.MaxLevel = int(toFloat64(ml, 10))
	}
	if out.MaxLevel < 1 || out.MaxLevel > 10 {
		out.MaxLevel = 10
	}
	// Parse statBonuses array
	if sbArr, e := m["statBonuses"]; e {
		if arr, ok := sbArr.([]any); ok {
			for _, entry := range arr {
				em, ok := entry.(map[string]any)
				if !ok {
					continue
				}
				attrKey := strings.TrimSpace(fmt.Sprint(em["attrKey"]))
				if attrKey == "" {
					continue
				}
				out.StatBonuses = append(out.StatBonuses, battleWorldStatBonus{
					AttrKey:      attrKey,
					Base:         toFloat64(em["base"], 0),
					PerLevelMult: toFloat64(em["perLevelMult"], 0),
				})
			}
		}
	}
	// Parse specialDrops array
	if sdArr, e := m["specialDrops"]; e {
		if arr, ok := sdArr.([]any); ok {
			for _, entry := range arr {
				em, ok := entry.(map[string]any)
				if !ok {
					continue
				}
				itemID := strings.TrimSpace(fmt.Sprint(em["itemId"]))
				if itemID == "" {
					continue
				}
				minLvl := int(toFloat64(em["minLevel"], 1))
				if minLvl < 1 {
					minLvl = 1
				}
				out.SpecialDrops = append(out.SpecialDrops, battleWorldSpecialDrop{
					ItemID:   itemID,
					Percent:  toFloat64(em["percent"], 0),
					RangeMin: toInt64Loose(em["rangeMin"], 1),
					RangeMax: toInt64Loose(em["rangeMax"], 1),
					MinLevel: minLvl,
				})
			}
		}
	}
	if len(out.StatBonuses) == 0 && len(out.SpecialDrops) == 0 {
		return nil
	}
	return out
}

func toInt64Map(value any) map[string]int64 {
	out := map[string]int64{}
	switch typed := value.(type) {
	case map[string]any:
		for key, raw := range typed {
			key = strings.TrimSpace(key)
			if key == "" {
				continue
			}
			num := toInt64Loose(raw, 0)
			if num > 0 {
				out[key] = num
			}
		}
	case map[string]int64:
		for key, raw := range typed {
			key = strings.TrimSpace(key)
			if key == "" || raw <= 0 {
				continue
			}
			out[key] = raw
		}
	}
	return out
}
