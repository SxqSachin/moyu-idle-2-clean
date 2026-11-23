package httpapi

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v2"
)

func (s *Server) handleBootstrapInit(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	payload, loadErr := s.buildBootstrapInitPayload(c.Context(), userID)
	if loadErr != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, loadErr.code, loadErr.message, loadErr.debug)
	}
	return s.writeAPISuccessMaybeCompressed(c, fiber.StatusOK, payload, 1024)
}

type bootstrapLoadError struct {
	code    string
	message string
	debug   string
}

func (s *Server) buildBootstrapInitPayload(ctx context.Context, userID int64) (fiber.Map, *bootstrapLoadError) {
	inventoryItems, err := s.listInventoryInstances(ctx, userID, true)
	if err != nil {
		return nil, &bootstrapLoadError{
			code:    "BOOTSTRAP_INVENTORY_LIST_FAILED",
			message: "load bootstrap inventory failed",
			debug:   err.Error(),
		}
	}
	equipmentItems, err := s.deps.Store.ListEquippedItems(ctx, userID)
	if err != nil {
		return nil, &bootstrapLoadError{
			code:    "BOOTSTRAP_EQUIPMENT_LIST_FAILED",
			message: "load bootstrap equipment failed",
			debug:   err.Error(),
		}
	}
	playerStats, statsErr := s.bootstrapPlayerStatsSnapshot(ctx, userID)
	if statsErr != nil {
		return nil, statsErr
	}
	chatConfig, chatConfigErr := s.bootstrapChatConfigSnapshot()
	if chatConfigErr != nil {
		return nil, chatConfigErr
	}
	chatHistory, chatHistoryErr := s.bootstrapChatHistorySnapshot(ctx)
	if chatHistoryErr != nil {
		return nil, chatHistoryErr
	}
	settings, err := s.deps.Store.GetUserSettings(ctx, userID)
	if err != nil {
		return nil, &bootstrapLoadError{
			code:    "BOOTSTRAP_PLAYER_SETTINGS_FAILED",
			message: "load bootstrap player settings failed",
			debug:   err.Error(),
		}
	}
	presence, err := s.reconcilePresenceState(ctx, userID)
	if err != nil {
		return nil, &bootstrapLoadError{
			code:    "BOOTSTRAP_PLAYER_PRESENCE_FAILED",
			message: "load bootstrap player presence failed",
			debug:   err.Error(),
		}
	}
	tutorialProgress, err := s.deps.Store.GetTutorialProgress(ctx, userID)
	if err != nil {
		return nil, &bootstrapLoadError{
			code:    "BOOTSTRAP_TUTORIAL_PROGRESS_FAILED",
			message: "load bootstrap tutorial progress failed",
			debug:   err.Error(),
		}
	}
	operationEffects, err := s.deps.Store.ListOperationEffects(ctx, userID, time.Now())
	if err != nil {
		return nil, &bootstrapLoadError{
			code:    "BOOTSTRAP_OPERATION_EFFECTS_FAILED",
			message: "load operation effects failed",
			debug:   err.Error(),
		}
	}
	return fiber.Map{
		"playerStats": playerStats,
		"inventory": fiber.Map{
			"items": inventoryItems,
		},
		"equipment": fiber.Map{
			"items": equipmentItems,
		},
		"chatConfig": chatConfig,
		"chatHistory": fiber.Map{
			"channel":      chatHistory.channelID,
			"items":        chatHistory.messages,
			"nextBeforeId": chatHistory.nextBeforeID,
			"hasMore":      chatHistory.hasMore,
		},
		"playerSettings": fiber.Map{
			"userId":           settings.UserID,
			"username":         settings.Username,
			"usernameChangeId": settings.UsernameChangeID,
			"email":            settings.Email,
			"language":         settings.Language,
			"theme":            settings.Theme,
			"menuLayout":       settings.MenuLayout,
			"simpleMode":       settings.SimpleMode,
		},
		"playerPresence":   presence,
		"tutorialProgress": tutorialProgressView(tutorialProgress),
		"operationEffects": operationEffects,
		"systemVersion":    s.loadReleaseAnnouncement(ctx),
		"recycleConfig": fiber.Map{
			"goldCoefficient":               s.deps.Config.GlobalSettings.Economy.ItemRecycleGoldCoefficient,
			"affixMismatchExtraCoefficient": s.deps.Config.GlobalSettings.Economy.ItemRecycleAffixMismatchExtraCoefficient,
		},
	}, nil
}

func (s *Server) bootstrapPlayerStatsSnapshot(ctx context.Context, userID int64) (fiber.Map, *bootstrapLoadError) {
	seeds, err := s.playerAttributeSeeds()
	if err != nil {
		return nil, &bootstrapLoadError{
			code:    "BOOTSTRAP_PLAYER_ATTRIBUTE_SEEDS_INVALID",
			message: "player attribute seeds invalid",
			debug:   err.Error(),
		}
	}
	if err := s.ensurePlayerStatsSnapshot(ctx, userID, seeds); err != nil {
		return nil, &bootstrapLoadError{
			code:    "BOOTSTRAP_PLAYER_STATS_PREPARE_FAILED",
			message: "prepare bootstrap player stats failed",
			debug:   err.Error(),
		}
	}
	stats, err := s.deps.Store.GetPlayerStats(ctx, userID)
	if err != nil {
		return nil, &bootstrapLoadError{
			code:    "BOOTSTRAP_PLAYER_STATS_GET_FAILED",
			message: "load bootstrap player stats failed",
			debug:   err.Error(),
		}
	}
	attributeStates, err := s.deps.Store.ListPlayerAttributes(ctx, userID, seeds)
	if err != nil {
		return nil, &bootstrapLoadError{
			code:    "BOOTSTRAP_PLAYER_ATTRIBUTES_GET_FAILED",
			message: "load bootstrap player attributes failed",
			debug:   err.Error(),
		}
	}
	progress, err := s.attributeProgressMap(attributeStates)
	if err != nil {
		return nil, &bootstrapLoadError{
			code:    "BOOTSTRAP_PLAYER_ATTRIBUTE_PROGRESS_FAILED",
			message: "build bootstrap attribute progress failed",
			debug:   err.Error(),
		}
	}
	attributes := s.attributeMapFromStates(attributeStates)
	battlePointState, err := s.deps.Store.GetBattlePointState(ctx, userID)
	if err != nil {
		return nil, &bootstrapLoadError{
			code:    "BOOTSTRAP_PLAYER_BATTLE_POINTS_FAILED",
			message: "load bootstrap battle points failed",
			debug:   err.Error(),
		}
	}
	availableBattlePoints := battlePointState.TotalPoints - battlePointState.SpentPoints
	if availableBattlePoints < 0 {
		availableBattlePoints = 0
	}
	buildings, err := s.deps.Store.ListPlayerBuildings(ctx, userID)
	if err != nil {
		return nil, &bootstrapLoadError{
			code:    "BOOTSTRAP_PLAYER_BUILDINGS_GET_FAILED",
			message: "load bootstrap player buildings failed",
			debug:   err.Error(),
		}
	}
	kitties, err := s.deps.Store.ListPlayerKitties(ctx, userID)
	if err != nil {
		return nil, &bootstrapLoadError{
			code:    "BOOTSTRAP_PLAYER_KITTIES_GET_FAILED",
			message: "load bootstrap player kitties failed",
			debug:   err.Error(),
		}
	}
	return fiber.Map{
		"derived":           stats.Derived,
		"cacheVersion":      stats.CacheVer,
		"updatedAt":         stats.UpdatedAt,
		"attributeProgress": progress,
		"attributes":        attributes,
		"buildings":         buildings,
		"kitties":           kitties,
		"battlePoints": fiber.Map{
			"totalPoints": battlePointState.TotalPoints,
			"spentPoints": battlePointState.SpentPoints,
			"available":   availableBattlePoints,
		},
		"formula": s.playerFormulaSnapshot(),
	}, nil
}

func (s *Server) bootstrapChatConfigSnapshot() (fiber.Map, *bootstrapLoadError) {
	channels := s.chatChannels()
	items := make([]fiber.Map, 0, len(channels))
	for _, channel := range channels {
		items = append(items, fiber.Map{
			"id":             channel.ID,
			"nameKey":        channel.NameKey,
			"order":          channel.Order,
			"playerSendable": channel.PlayerSendable,
		})
	}
	settings := s.deps.Config.GlobalSettings.Chat
	return fiber.Map{
		"maxContentLength":    settings.MaxContentLength,
		"defaultHistoryLimit": settings.DefaultHistoryLimit,
		"defaultChannel":      s.defaultChatChannel(),
		"channels":            items,
	}, nil
}

type bootstrapChatHistorySnapshot struct {
	channelID    string
	messages     []fiber.Map
	nextBeforeID int64
	hasMore      bool
}

func (s *Server) bootstrapChatHistorySnapshot(ctx context.Context) (*bootstrapChatHistorySnapshot, *bootstrapLoadError) {
	settings := s.deps.Config.GlobalSettings.Chat
	channelID := s.defaultChatChannel()
	limit := settings.DefaultHistoryLimit
	if limit <= 0 {
		limit = 30
	}
	if limit > 200 {
		limit = 200
	}
	items, err := s.deps.Store.ListChatMessages(ctx, channelID, limit, 0)
	if err != nil {
		return nil, &bootstrapLoadError{
			code:    "BOOTSTRAP_CHAT_MESSAGES_QUERY_FAILED",
			message: "load bootstrap chat messages failed",
			debug:   err.Error(),
		}
	}
	messages := make([]fiber.Map, 0, len(items))
	var nextBeforeID int64
	for idx, item := range items {
		messages = append(messages, mapChatMessage(item))
		if idx == len(items)-1 {
			nextBeforeID = item.ID
		}
	}
	return &bootstrapChatHistorySnapshot{
		channelID:    channelID,
		messages:     messages,
		nextBeforeID: nextBeforeID,
		hasMore:      len(items) >= limit,
	}, nil
}
