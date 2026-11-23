package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
	"mysecondwebgame/server/internal/config"
	"mysecondwebgame/server/internal/store"
)

const fallbackChatChannelID = "world"
const battleRoomChatChannelPrefix = "battle_room:"
const chatAttachmentArenaReplay = "arena_replay"
const chatAttachmentCombatTrialReplay = "combat_trial_replay"
const chatAttachmentItem = "item"
const chatAttachmentItemInstance = "item_instance"
const chatAttachmentAction = "action"
const chatAttachmentIdentifierMaxBytes = 128
const chatReplayTokenMaxBytes = 256
const chatAttachmentSnapshotAttributeLimit = 64
const chatAttachmentAffixLimit = 16

var (
	errChatAttachmentInvalid       = errors.New("invalid chat attachment")
	errChatItemInstanceUnavailable = errors.New("chat item instance unavailable")
	chatInternalActionIDs          = map[string]struct{}{
		"alchemy.convert":   {},
		"alchemy.bench":     {},
		"alchemy.inherit":   {},
		"alchemy.transmute": {},
		"alchemy.smelt":     {},
	}
)

func normalizeChatChannelID(raw string) string {
	return strings.TrimSpace(strings.ToLower(raw))
}

func buildBattleRoomChatChannelID(roomID int64) string {
	if roomID <= 0 {
		return ""
	}
	return battleRoomChatChannelPrefix + strconv.FormatInt(roomID, 10)
}

func parseBattleRoomChatChannelID(raw string) (int64, bool) {
	normalized := normalizeChatChannelID(raw)
	if !strings.HasPrefix(normalized, battleRoomChatChannelPrefix) {
		return 0, false
	}
	roomPart := strings.TrimSpace(strings.TrimPrefix(normalized, battleRoomChatChannelPrefix))
	if roomPart == "" {
		return 0, false
	}
	roomID, err := strconv.ParseInt(roomPart, 10, 64)
	if err != nil || roomID <= 0 {
		return 0, false
	}
	return roomID, true
}

func (s *Server) battleRoomChatAccess(ctx context.Context, userID int64, roomID int64) (store.BattleRoomDetail, bool, error) {
	if userID <= 0 || roomID <= 0 {
		return store.BattleRoomDetail{}, false, nil
	}
	detail, err := s.deps.Store.GetBattleRoomByUser(ctx, userID)
	if errors.Is(err, store.ErrBattleRoomNotFound) {
		return store.BattleRoomDetail{}, false, nil
	}
	if err != nil {
		return store.BattleRoomDetail{}, false, err
	}
	if detail.Room.ID != roomID {
		return store.BattleRoomDetail{}, false, nil
	}
	return detail, true, nil
}

func (s *Server) chatChannels() []config.ChatChannel {
	channels := s.deps.Config.GlobalSettings.Chat.Channels
	if len(channels) > 0 {
		return channels
	}
	return []config.ChatChannel{{
		ID:             fallbackChatChannelID,
		NameKey:        "chat.channel.world",
		Order:          10,
		PlayerSendable: true,
	}}
}

func (s *Server) defaultChatChannel() string {
	channels := s.chatChannels()
	for _, ch := range channels {
		if ch.PlayerSendable {
			return ch.ID
		}
	}
	if len(channels) > 0 {
		return channels[0].ID
	}
	return fallbackChatChannelID
}

func (s *Server) resolveChatChannel(raw string) (config.ChatChannel, bool) {
	channelID := normalizeChatChannelID(raw)
	if channelID == "" {
		channelID = s.defaultChatChannel()
	}
	for _, channel := range s.chatChannels() {
		if channel.ID == channelID {
			return channel, true
		}
	}
	return config.ChatChannel{}, false
}

func mapChatMessage(msg store.ChatMessage) fiber.Map {
	ts := msg.SentAt
	if ts.IsZero() {
		ts = time.Now()
	}
	displayMeta := msg.DisplayMeta
	if displayMeta == nil {
		displayMeta = map[string]any{}
	}
	attachment := publicChatAttachment(msg.Attachment)
	if attachment == nil {
		attachment = map[string]any{}
	}
	return fiber.Map{
		"id":            msg.ID,
		"channel":       msg.Channel,
		"userId":        msg.UserID,
		"username":      msg.Username,
		"titlePrefixId": msg.TitlePrefixID,
		"titleSuffixId": msg.TitleSuffixID,
		"displayMeta":   displayMeta,
		"attachment":    attachment,
		"content":       msg.Content,
		"sentAt":        ts,
		"ts":            ts.UnixMilli(),
	}
}

func (s *Server) resolveChatAttachment(ctx context.Context, userID int64, raw map[string]any) (map[string]any, error) {
	if len(raw) == 0 {
		return map[string]any{}, nil
	}
	kind, ok := parseChatAttachmentText(raw["type"], 32)
	if !ok {
		return nil, errChatAttachmentInvalid
	}
	kind = strings.ToLower(kind)
	switch kind {
	case chatAttachmentArenaReplay:
		token, ok := parseChatAttachmentText(raw["token"], chatReplayTokenMaxBytes)
		if !ok {
			return nil, errChatAttachmentInvalid
		}
		share, err := s.deps.Store.GetArenaReplayShare(ctx, token, s.deps.Config.GlobalSettings.Arena.HistoryKeepCount)
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"type":          chatAttachmentArenaReplay,
			"battleKind":    battleKindArena,
			"token":         share.Token,
			"challengeId":   share.ChallengeID,
			"createdBy":     share.CreatedBy,
			"createdByName": share.CreatedByName,
			"outcome":       share.Outcome,
		}, nil
	case chatAttachmentCombatTrialReplay:
		token, ok := parseChatAttachmentText(raw["token"], chatReplayTokenMaxBytes)
		if !ok {
			return nil, errChatAttachmentInvalid
		}
		share, err := s.deps.Store.GetCombatTrialReplayShare(ctx, token)
		if err != nil {
			return nil, err
		}
		attachment := map[string]any{
			"type":          chatAttachmentCombatTrialReplay,
			"battleKind":    battleKindCombatTrial,
			"token":         share.Token,
			"runId":         share.RunID,
			"createdBy":     share.CreatedBy,
			"createdByName": share.CreatedByName,
			"outcome":       share.EndReason,
			"metrics":       share.Metrics,
		}
		if labelKey := combatTrialReplayOutcomeLabelKey(share.FormatSnapshot, share.EndReason); labelKey != "" {
			attachment["outcomeLabelKey"] = labelKey
		}
		return attachment, nil
	case chatAttachmentItem:
		itemID, ok := parseChatAttachmentText(raw["itemId"], chatAttachmentIdentifierMaxBytes)
		if !ok {
			return nil, errChatAttachmentInvalid
		}
		item := s.deps.GameData.ItemByID(itemID)
		if item == nil || item.Disabled {
			return nil, errChatAttachmentInvalid
		}
		return map[string]any{
			"type":   chatAttachmentItem,
			"itemId": item.ID,
		}, nil
	case chatAttachmentItemInstance:
		instanceID, ok := parsePositiveChatAttachmentID(raw["instanceId"])
		if !ok || userID <= 0 || s.deps.Store == nil {
			return nil, errChatItemInstanceUnavailable
		}
		instance, _, err := s.deps.Store.GetItemInstance(ctx, userID, instanceID)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errChatItemInstanceUnavailable
		}
		if err != nil {
			return nil, fmt.Errorf("load chat item instance: %w", err)
		}
		item := s.deps.GameData.ItemByID(instance.TemplateItemID)
		if item == nil || item.Disabled {
			return nil, errChatItemInstanceUnavailable
		}
		return s.buildChatItemInstanceAttachment(instance)
	case chatAttachmentAction:
		actionID, ok := parseChatAttachmentText(raw["actionId"], chatAttachmentIdentifierMaxBytes)
		if !ok {
			return nil, errChatAttachmentInvalid
		}
		if _, hidden := chatInternalActionIDs[actionID]; hidden {
			return nil, errChatAttachmentInvalid
		}
		for idx := range s.deps.GameData.Actions {
			action := &s.deps.GameData.Actions[idx]
			if action.Disabled || action.ID != actionID {
				continue
			}
			return map[string]any{
				"type":     chatAttachmentAction,
				"actionId": action.ID,
			}, nil
		}
		return nil, errChatAttachmentInvalid
	default:
		return nil, errChatAttachmentInvalid
	}
}

func parseChatAttachmentText(raw any, maxBytes int) (string, bool) {
	value, ok := raw.(string)
	if !ok {
		return "", false
	}
	value = strings.TrimSpace(value)
	if value == "" || maxBytes <= 0 || len(value) > maxBytes {
		return "", false
	}
	return value, true
}

func (s *Server) hitChatSendRateLimit(ctx context.Context, userID int64, ip string) (bool, time.Duration, error) {
	if userID <= 0 {
		return true, 0, nil
	}
	settings := s.deps.Config.GlobalSettings.RateLimit.ChatSend
	if settings.Max <= 0 || settings.Max > 100 || settings.WindowSeconds <= 0 || settings.WindowSeconds > 300 {
		settings = config.FixedWindowRateLimitSettings{Max: 10, WindowSeconds: 10}
	}
	window := time.Duration(settings.WindowSeconds) * time.Second
	userLimiter := ipRateLimiter{max: settings.Max, window: window, prefix: "ws_chat_user", redis: s.deps.Redis}
	limited, retryAfter, err := userLimiter.hit(ctx, strconv.FormatInt(userID, 10), time.Now())
	if err != nil || limited {
		return limited, retryAfter, err
	}
	ip = strings.TrimSpace(ip)
	if ip == "" {
		return false, 0, nil
	}
	ipDigest := sha256.Sum256([]byte(ip))
	ipLimiter := ipRateLimiter{max: settings.Max * 4, window: window, prefix: "ws_chat_ip", redis: s.deps.Redis}
	return ipLimiter.hit(ctx, fmt.Sprintf("%x", ipDigest[:16]), time.Now())
}

func parsePositiveChatAttachmentID(raw any) (int64, bool) {
	var value int64
	switch typed := raw.(type) {
	case int:
		value = int64(typed)
	case int32:
		value = int64(typed)
	case int64:
		value = typed
	case float32:
		floatValue := float64(typed)
		if math.Trunc(floatValue) != floatValue || floatValue < math.MinInt64 || floatValue > math.MaxInt64 {
			return 0, false
		}
		value = int64(floatValue)
	case float64:
		if math.Trunc(typed) != typed || typed < math.MinInt64 || typed > math.MaxInt64 {
			return 0, false
		}
		value = int64(typed)
	case json.Number:
		parsed, err := typed.Int64()
		if err != nil {
			return 0, false
		}
		value = parsed
	case string:
		parsed, err := strconv.ParseInt(strings.TrimSpace(typed), 10, 64)
		if err != nil {
			return 0, false
		}
		value = parsed
	default:
		return 0, false
	}
	if value <= 0 {
		return 0, false
	}
	return value, true
}

func (s *Server) buildChatItemInstanceAttachment(instance store.ItemInstance) (map[string]any, error) {
	if instance.InstanceID <= 0 || strings.TrimSpace(instance.TemplateItemID) == "" || instance.Quantity <= 0 ||
		strings.TrimSpace(instance.State) != "inventory" || instance.ArchivedAt != nil {
		return nil, errChatItemInstanceUnavailable
	}
	preview := map[string]any{"enhanceLevel": maxInt(0, instance.EnhanceLevel)}
	if instance.CustomData.Name != "" || instance.CustomData.LabelColor != "" {
		customData := map[string]any{}
		if name := strings.TrimSpace(instance.CustomData.Name); name != "" && len(name) <= chatAttachmentIdentifierMaxBytes {
			customData["name"] = name
		}
		if color := strings.TrimSpace(strings.ToLower(instance.CustomData.LabelColor)); color != "" && len(color) <= 32 {
			customData["labelColor"] = color
		}
		if len(customData) > 0 {
			preview["customData"] = customData
		}
	}
	if instance.FishData != nil {
		preview["fishData"] = s.buildChatFishInstancePreview(*instance.FishData)
	}
	if snapshot := buildPublicChatNumericMap(instance.BaseSnapshot, chatAttachmentSnapshotAttributeLimit); len(snapshot) > 0 {
		preview["baseSnapshot"] = snapshot
	}
	if affixes := buildPublicChatAffixPreview(instance.Affixes); len(affixes) > 0 {
		preview["affixes"] = affixes
	}
	return map[string]any{
		"type":    chatAttachmentItemInstance,
		"itemId":  strings.TrimSpace(instance.TemplateItemID),
		"preview": preview,
	}, nil
}

func buildPublicChatNumericMap(raw map[string]any, limit int) map[string]float64 {
	if len(raw) == 0 || limit <= 0 {
		return nil
	}
	keys := make([]string, 0, len(raw))
	for key := range raw {
		if key != "" && key == strings.TrimSpace(key) && len(key) <= chatAttachmentIdentifierMaxBytes {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	if len(keys) > limit {
		keys = keys[:limit]
	}
	out := make(map[string]float64, len(keys))
	for _, key := range keys {
		value := toFloat64(raw[key], math.NaN())
		if math.IsNaN(value) || math.IsInf(value, 0) {
			continue
		}
		out[key] = value
	}
	return out
}

func buildPublicChatAffixPreview(raw []store.EquipmentAffixSlot) []map[string]any {
	if len(raw) == 0 {
		return nil
	}
	limit := minInt(len(raw), chatAttachmentAffixLimit)
	out := make([]map[string]any, 0, limit)
	for _, affix := range raw[:limit] {
		affixID := strings.TrimSpace(affix.AffixID)
		if affix.SlotIndex < 0 || affix.SlotIndex > 63 || affixID == "" || len(affixID) > chatAttachmentIdentifierMaxBytes {
			continue
		}
		entry := map[string]any{
			"slotIndex": affix.SlotIndex,
			"affixId":   affixID,
		}
		if rarityID := strings.TrimSpace(affix.RarityID); rarityID != "" && len(rarityID) <= 64 {
			entry["rarityId"] = rarityID
		}
		if sourceKind := strings.TrimSpace(affix.SourceKind); sourceKind == "craft" || sourceKind == "inherit" || sourceKind == "inherit_orphan" {
			entry["sourceKind"] = sourceKind
		}
		if displayNameKey := strings.TrimSpace(affix.DisplayNameKey); displayNameKey != "" && len(displayNameKey) <= chatAttachmentIdentifierMaxBytes {
			entry["displayNameKey"] = displayNameKey
			if args := buildPublicChatDisplayNameArgs(affix.DisplayNameArgs); len(args) > 0 {
				entry["displayNameArgs"] = args
			}
		}
		if !math.IsNaN(affix.RollValue) && !math.IsInf(affix.RollValue, 0) {
			entry["rollValue"] = affix.RollValue
		}
		if affix.ValuePrecision >= 0 && affix.ValuePrecision <= 8 {
			entry["valuePrecision"] = affix.ValuePrecision
		}
		if attrMods := buildPublicChatFloatMap(affix.AttrMods, 32); len(attrMods) > 0 {
			entry["attrMods"] = attrMods
		}
		if effectType := strings.TrimSpace(affix.EffectType); effectType != "" && len(effectType) <= 64 {
			entry["effectType"] = effectType
		}
		out = append(out, entry)
	}
	return out
}

func buildPublicChatFloatMap(raw map[string]float64, limit int) map[string]float64 {
	if len(raw) == 0 || limit <= 0 {
		return nil
	}
	keys := make([]string, 0, len(raw))
	for key := range raw {
		if key != "" && key == strings.TrimSpace(key) && len(key) <= chatAttachmentIdentifierMaxBytes {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	if len(keys) > limit {
		keys = keys[:limit]
	}
	out := make(map[string]float64, len(keys))
	for _, key := range keys {
		value := raw[key]
		if math.IsNaN(value) || math.IsInf(value, 0) {
			continue
		}
		out[key] = value
	}
	return out
}

func buildPublicChatDisplayNameArgs(raw map[string]any) map[string]any {
	if len(raw) == 0 {
		return nil
	}
	keys := make([]string, 0, len(raw))
	for key := range raw {
		if key != "" && key == strings.TrimSpace(key) && len(key) <= 64 {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	if len(keys) > 16 {
		keys = keys[:16]
	}
	out := make(map[string]any, len(keys))
	for _, key := range keys {
		switch value := raw[key].(type) {
		case string:
			value = strings.TrimSpace(value)
			if value != "" && len(value) <= chatAttachmentIdentifierMaxBytes {
				out[key] = value
			}
		case bool:
			out[key] = value
		case float64:
			if !math.IsNaN(value) && !math.IsInf(value, 0) {
				out[key] = value
			}
		case int:
			out[key] = value
		case int64:
			out[key] = value
		case []string:
			values := make([]string, 0, minInt(len(value), 8))
			for _, entry := range value {
				entry = strings.TrimSpace(entry)
				if entry != "" && len(entry) <= chatAttachmentIdentifierMaxBytes {
					values = append(values, entry)
					if len(values) == 8 {
						break
					}
				}
			}
			if len(values) > 0 {
				out[key] = values
			}
		case []any:
			values := make([]string, 0, minInt(len(value), 8))
			for _, rawEntry := range value {
				entry, ok := rawEntry.(string)
				if !ok {
					continue
				}
				entry = strings.TrimSpace(entry)
				if entry != "" && len(entry) <= chatAttachmentIdentifierMaxBytes {
					values = append(values, entry)
					if len(values) == 8 {
						break
					}
				}
			}
			if len(values) > 0 {
				out[key] = values
			}
		}
	}
	return out
}

func publicChatAttachment(raw map[string]any) map[string]any {
	if len(raw) == 0 || strings.TrimSpace(fmt.Sprint(raw["type"])) != chatAttachmentItemInstance {
		return raw
	}
	out := make(map[string]any, len(raw))
	for key, value := range raw {
		switch key {
		case "instanceId", "quantity":
			continue
		default:
			out[key] = value
		}
	}
	previewRaw, ok := raw["preview"].(map[string]any)
	if !ok {
		return out
	}
	preview := make(map[string]any, len(previewRaw))
	for key, value := range previewRaw {
		switch key {
		case "durability", "maxDurability":
			continue
		default:
			preview[key] = value
		}
	}
	out["preview"] = preview
	return out
}

func (s *Server) buildChatFishInstancePreview(fish store.FishInstanceData) map[string]any {
	preview := map[string]any{
		"speciesId": fish.SpeciesID,
		"ageDays":   maxInt(0, fish.AgeDays),
		"sex":       fish.Sex,
		"lengthCm":  math.Max(0, fish.LengthCM),
		"weightKg":  math.Max(0, fish.WeightKG),
		"condition": math.Max(0, fish.Condition),
		"health":    math.Max(0, fish.Health),
	}
	if fish.WaterID != "" {
		preview["waterId"] = fish.WaterID
	}
	if fish.SpotID != "" {
		preview["spotId"] = fish.SpotID
	}
	if !fish.CaughtAt.IsZero() {
		preview["caughtAt"] = fish.CaughtAt
	}
	if fish.Origin != "" {
		preview["origin"] = fish.Origin
	}
	if fish.LifeState != "" {
		preview["lifeState"] = fish.LifeState
	}
	if fish.Generation > 0 {
		preview["generation"] = fish.Generation
	}
	if mutations := s.chatVisibleFishMutationIDs(fish.Genes.Mutations); len(mutations) > 0 {
		preview["genes"] = map[string]any{"mutations": mutations}
	}
	return preview
}

func (s *Server) chatVisibleFishMutationIDs(ids []string) []string {
	wanted := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		if normalized := strings.TrimSpace(id); normalized != "" {
			wanted[normalized] = struct{}{}
		}
	}
	visible := make([]string, 0, len(wanted))
	for _, mutation := range s.deps.Config.GlobalSettings.Fishing.Mutations {
		id := strings.TrimSpace(mutation.ID)
		if _, ok := wanted[id]; ok {
			visible = append(visible, id)
		}
	}
	return visible
}

func combatTrialReplayOutcomeLabelKey(snapshot json.RawMessage, endReason string) string {
	endReason = strings.TrimSpace(endReason)
	if endReason == "" || len(snapshot) == 0 {
		return ""
	}
	format, err := decodeCombatTrialFormatSnapshot(snapshot)
	if err != nil {
		return ""
	}
	for _, condition := range format.EndConditions {
		if strings.TrimSpace(condition.Reason) == endReason {
			return strings.TrimSpace(condition.LabelKey)
		}
	}
	return ""
}

func (s *Server) handleChatConfig(c *fiber.Ctx) error {
	if _, err := s.authUserID(c); err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
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
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{
		"maxContentLength":    settings.MaxContentLength,
		"defaultHistoryLimit": settings.DefaultHistoryLimit,
		"defaultChannel":      s.defaultChatChannel(),
		"channels":            items,
	})
}

func parseChatHistoryLimit(raw string, fallback int) (int, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fallback, true
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil || parsed <= 0 {
		return 0, false
	}
	if parsed > 200 {
		parsed = 200
	}
	return parsed, true
}

func parseChatBeforeID(raw string) (int64, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, true
	}
	parsed, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || parsed < 0 {
		return 0, false
	}
	return parsed, true
}

func (s *Server) handleChatMessages(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	requestedChannel := c.Query("channel")
	channelID := ""
	channelLabel := ""
	if roomID, roomChannel := parseBattleRoomChatChannelID(requestedChannel); roomChannel {
		detail, allowed, accessErr := s.battleRoomChatAccess(c.Context(), userID, roomID)
		if accessErr != nil {
			return s.writeAPIError(c, fiber.StatusInternalServerError, "CHAT_CHANNEL_ACCESS_FAILED", "chat channel access check failed", accessErr.Error())
		}
		if !allowed {
			return s.writeAPIError(c, fiber.StatusForbidden, "CHAT_CHANNEL_FORBIDDEN", "chat channel forbidden", "")
		}
		channelID = buildBattleRoomChatChannelID(roomID)
		channelLabel = detail.Room.RoomName
	} else {
		channel, ok := s.resolveChatChannel(requestedChannel)
		if !ok {
			return s.writeAPIError(c, fiber.StatusBadRequest, "CHAT_CHANNEL_INVALID", "invalid chat channel", "")
		}
		channelID = channel.ID
	}
	settings := s.deps.Config.GlobalSettings.Chat
	limit, ok := parseChatHistoryLimit(c.Query("limit"), settings.DefaultHistoryLimit)
	if !ok {
		return s.writeAPIError(c, fiber.StatusBadRequest, "CHAT_LIMIT_INVALID", "invalid chat history limit", "")
	}
	beforeID, ok := parseChatBeforeID(c.Query("beforeId"))
	if !ok {
		return s.writeAPIError(c, fiber.StatusBadRequest, "CHAT_BEFORE_ID_INVALID", "invalid beforeId", "")
	}
	items, err := s.deps.Store.ListChatMessages(c.Context(), channelID, limit, beforeID)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "CHAT_MESSAGES_QUERY_FAILED", "query chat messages failed", err.Error())
	}
	messages := make([]fiber.Map, 0, len(items))
	var nextBeforeID int64
	for idx, item := range items {
		messages = append(messages, mapChatMessage(item))
		if idx == len(items)-1 {
			nextBeforeID = item.ID
		}
	}
	resp := fiber.Map{
		"channel":      channelID,
		"items":        messages,
		"nextBeforeId": nextBeforeID,
		"hasMore":      len(items) >= limit,
	}
	if strings.TrimSpace(channelLabel) != "" {
		resp["channelLabel"] = strings.TrimSpace(channelLabel)
	}
	return s.writeAPISuccess(c, fiber.StatusOK, resp)
}
