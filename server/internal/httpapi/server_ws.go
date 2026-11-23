package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"
	"unicode/utf8"

	"mysecondwebgame/server/internal/actionruntime"
	"mysecondwebgame/server/internal/auth"
	"mysecondwebgame/server/internal/reqmetrics"
	"mysecondwebgame/server/internal/store"
	"mysecondwebgame/server/internal/ws"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
)

type bindPayload struct {
	Token string `json:"token"`
}

type chatPayload struct {
	Channel    string         `json:"channel"`
	Content    string         `json:"content"`
	Attachment map[string]any `json:"attachment"`
}

type battleStreamSubscribePayload struct {
	Enabled bool   `json:"enabled"`
	Kind    string `json:"kind"`
}

type wsMessage struct {
	Type  string        `json:"type"`
	OK    bool          `json:"ok"`
	TS    int64         `json:"ts"`
	Data  any           `json:"data,omitempty"`
	Error *apiErrorBody `json:"error,omitempty"`
}

const wsAuthAttemptKey = "bind"
const wsRequestIDMaxBytes = 128

func shouldDeduplicatePreauthenticatedWSBind(preauthenticated bool, clientUserID, clientSessionVersion, presenceUserID, presenceSessionVersion, nextUserID, nextSessionVersion int64) bool {
	return preauthenticated && nextUserID > 0 && nextSessionVersion > 0 &&
		clientUserID == nextUserID && clientSessionVersion == nextSessionVersion &&
		presenceUserID == nextUserID && presenceSessionVersion == nextSessionVersion
}

func isValidWSRequestID(value string) bool {
	if value == "" || len(value) > wsRequestIDMaxBytes {
		return false
	}
	for _, char := range value {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') {
			continue
		}
		switch char {
		case '-', '_', '.', ':':
			continue
		default:
			return false
		}
	}
	return true
}

func isChatWSMessage(typ string) bool {
	return typ == "chat.send" || typ == "chat.global.send"
}

var (
	errAuthSessionRevoked      = errors.New("session revoked")
	errAuthAccountBanned       = errors.New("account banned")
	errAuthBackendBusy         = errors.New("auth backend busy")
	parseAuthClaimsWithSession = func(s *Server, token string) (auth.Claims, error) {
		return s.deps.Auth.ParseWithSession(token)
	}
	loadAuthGateState = func(s *Server, ctx context.Context, userID int64) (store.AuthGateState, error) {
		return s.deps.AuthGate.GetAuthGateState(ctx, userID)
	}
)

func decodeWSJSONEnvelope(raw []byte) (map[string]json.RawMessage, error) {
	var decoded map[string]json.RawMessage
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return nil, fmt.Errorf("parse ws payload failed: %w", err)
	}
	return decoded, nil
}

func (s *Server) decodeWSIncomingFrame(messageType int, payload []byte) (map[string]json.RawMessage, error) {
	switch messageType {
	case websocket.TextMessage:
		return decodeWSJSONEnvelope(payload)
	case websocket.BinaryMessage:
		plain, err := decodeWSCompressedFrame(payload)
		if err != nil {
			return nil, err
		}
		return decodeWSJSONEnvelope(plain)
	default:
		return nil, fmt.Errorf("unsupported ws message type: %d", messageType)
	}
}

func (s *Server) writeWSEvent(conn *websocket.Conn, typ string, payload any) error {
	msg := wsMessage{
		Type: typ,
		OK:   true,
		TS:   time.Now().UnixMilli(),
		Data: payload,
	}
	return s.enqueueWSMessage(conn, msg)
}

func (s *Server) writeWSError(conn *websocket.Conn, code string, message string, debug ...string) error {
	return s.writeWSErrorWithMessageKey(conn, code, message, "", debug...)
}

func (s *Server) writeWSErrorWithMessageKey(conn *websocket.Conn, code string, message string, messageKey string, debug ...string) error {
	body := apiErrorBody{
		Code:       code,
		Message:    message,
		MessageKey: strings.TrimSpace(messageKey),
	}
	if s.isDevEnv() && len(debug) > 0 && strings.TrimSpace(debug[0]) != "" {
		body.Debug = strings.TrimSpace(debug[0])
	}
	msg := wsMessage{
		Type:  "error",
		OK:    false,
		TS:    time.Now().UnixMilli(),
		Error: &body,
	}
	return s.enqueueWSMessage(conn, msg)
}

func (s *Server) tryCompressWSMessage(payload any) (any, error) {
	compressionCfg := s.deps.Config.GlobalSettings.Transport.Compression
	if !compressionCfg.Enabled {
		return payload, nil
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal ws payload failed: %w", err)
	}
	if len(raw) < compressionCfg.MinBytes {
		return payload, nil
	}
	return encodeWSCompressedFrame(raw, compressionCfg.Level)
}

func (s *Server) enqueueWSMessage(conn *websocket.Conn, payload any) error {
	wirePayload, compressionErr := s.tryCompressWSMessage(payload)
	if compressionErr != nil {
		wirePayload = wsMessage{
			Type: "error",
			OK:   false,
			TS:   time.Now().UnixMilli(),
			Error: &apiErrorBody{
				Code:    "WS_COMPRESS_RESPONSE_FAILED",
				Message: "compress ws response failed",
			},
		}
	}
	sendErr := s.deps.Hub.Send(conn, wirePayload)
	if sendErr == nil {
		return compressionErr
	}
	if errors.Is(sendErr, ws.ErrClientQueueFull) {
		log.Printf("ws send queue full, closing slow connection")
		s.recordWSPressure(time.Now(), func(counters *wsPressureCounters) {
			counters.QueueFullCloses++
		})
		_ = conn.Close()
	}
	return errors.Join(compressionErr, sendErr)
}

func (s *Server) runWSWritePump(client *ws.Client) {
	if err := s.deps.Hub.WritePump(client); err != nil {
		log.Printf("ws write pump stopped err=%v", err)
	}
	_ = client.Conn.Close()
	s.deps.Hub.Unregister(client.Conn)
}

func (s *Server) handleWS(conn *websocket.Conn) {
	conn.SetReadLimit(wsIncomingMessageMaxBytes)
	client := s.deps.Hub.Register(conn)
	wsIP := s.wsClientIP(conn)
	var presenceBoundUserID int64
	var presenceBoundSessionVersion int64
	var deduplicateNextAuthBind bool
	var chatRateLimitedUntil time.Time
	go s.runWSWritePump(client)
	defer func() {
		s.deps.Hub.Unregister(conn)
		s.clearBattleStreamSubscribed(client.ID)
		_ = conn.Close()
		if presenceBoundUserID != 0 && presenceBoundSessionVersion > 0 {
			s.markUserOfflineByWS(presenceBoundUserID, presenceBoundSessionVersion)
		}
		if client.UserID > 0 {
			s.decrGlobalConnectionCount(client.UserID)
			s.clearUserTransientCachesIfInactive(client.UserID)
		}
	}()
	if rawToken := conn.Locals("wsToken"); rawToken != nil {
		if token, ok := rawToken.(string); ok && token != "" {
			if locked, _, lockErr := s.wsAuthGuard.IsLocked(wsIP, wsAuthAttemptKey, time.Now()); lockErr != nil {
				_ = s.writeWSError(conn, "WS_AUTH_GUARD_UNAVAILABLE", "ws auth guard backend unavailable")
				return
			} else if locked {
				_ = s.writeWSError(conn, "WS_AUTH_LOCKED", "too many failed auth attempts, please retry later")
				return
			}
			if uid, sessionVersion, err := s.authUserWithSession(context.Background(), token); err == nil {
				if guardErr := s.wsAuthGuard.Reset(wsIP, wsAuthAttemptKey); guardErr != nil {
					_ = s.writeWSError(conn, "WS_AUTH_GUARD_UNAVAILABLE", "ws auth guard backend unavailable")
					return
				}
				if presence, pErr := s.deps.Store.GetPresence(context.Background(), uid); pErr == nil {
					s.setUserPresenceCache(uid, presence.Status)
					now := time.Now()
					if _, err := s.markOfflineFastForwardFromPresence(uid, presence, now); err != nil {
						_ = s.writeWSError(conn, "OFFLINE_RECOVERY_UNAVAILABLE", "could not preserve offline recovery; reconnect to retry")
						return
					}
				} else {
					_ = s.writeWSError(conn, "OFFLINE_RECOVERY_UNAVAILABLE", "could not load offline window; reconnect to retry")
					return
				}
				client.UserID = uid
				client.SessionVersion = sessionVersion
				s.incrGlobalConnectionCount(uid)
				s.enforceConnectionLimit(uid, s.deps.Config.MaxConnectionsPerUser, client.ID)
				_ = s.writeWSEvent(conn, "auth.bound", fiber.Map{"userId": client.UserID})
				presenceBoundUserID = uid
				presenceBoundSessionVersion = sessionVersion
				deduplicateNextAuthBind = true
				s.markUserOnlineByWS(client.UserID, sessionVersion)
				s.resetRealtimePatchState(client.UserID)
				s.pushRealtimeInit(client.UserID)
			} else if err != nil && errors.Is(err, errAuthSessionRevoked) {
				_ = s.wsAuthGuard.RecordFailure(wsIP, wsAuthAttemptKey, time.Now())
				_ = s.writeWSError(conn, "WS_SESSION_REVOKED", "session revoked")
				return
			} else if err != nil && errors.Is(err, errAuthAccountBanned) {
				_ = s.wsAuthGuard.RecordFailure(wsIP, wsAuthAttemptKey, time.Now())
				_ = s.writeWSError(conn, "WS_ACCOUNT_BANNED", "account is banned")
				_ = conn.Close()
				return
			} else if err != nil && errors.Is(err, errAuthBackendBusy) {
				_ = s.writeWSError(conn, "WS_AUTH_BACKEND_BUSY", "auth backend busy")
				return
			} else {
				_ = s.wsAuthGuard.RecordFailure(wsIP, wsAuthAttemptKey, time.Now())
				_ = s.writeWSError(conn, "WS_INVALID_TOKEN", "invalid token")
			}
		}
	}

	for {
		messageType, payload, err := conn.ReadMessage()
		if err != nil {
			return
		}
		env, err := s.decodeWSIncomingFrame(messageType, payload)
		if err != nil {
			_ = s.writeWSError(conn, "WS_INVALID_COMPRESSED_PAYLOAD", "invalid compressed payload")
			continue
		}
		var typ string
		var requestID string
		_ = json.Unmarshal(env["type"], &typ)
		_ = json.Unmarshal(env["requestId"], &requestID)
		if typ == "" {
			_ = s.writeWSError(conn, "WS_INVALID_MESSAGE_TYPE", "missing message type")
			continue
		}
		if client.UserID != 0 {
			s.touchPresenceDB(client.UserID)
			s.refreshUserPresenceHeartbeat(client.UserID, client.SessionVersion)
		}
		if typ != "auth.bind" && client.UserID == 0 {
			_ = s.writeWSError(conn, "WS_UNAUTHORIZED", "bind first")
			continue
		}
		if client.UserID > 0 && isMutatingWSMessage(typ) {
			if strings.HasPrefix(typ, "action.") && typ != "action.offline.rewards.get" && s.hasOfflineFastForwardReady(client.UserID) {
				_ = s.writeWSError(conn, "OFFLINE_RECOVERY_PENDING", "offline progress is still settling; retry after recovery")
				continue
			}
			requestID = strings.TrimSpace(requestID)
			if !isValidWSRequestID(requestID) {
				_ = s.writeWSError(conn, "WS_REQUEST_ID_REQUIRED", "valid requestId required")
				continue
			}
			if isChatWSMessage(typ) {
				now := time.Now()
				if now.Before(chatRateLimitedUntil) {
					_ = s.writeWSError(conn, "RATE_LIMITED", "rate limit exceeded")
					continue
				}
				limited, retryAfter, limitErr := s.hitChatSendRateLimit(context.Background(), client.UserID, wsIP)
				if limitErr != nil {
					_ = s.writeWSError(conn, "RATE_LIMIT_BACKEND_UNAVAILABLE", "rate limit backend unavailable")
					return
				}
				if limited {
					if retryAfter <= 0 {
						retryAfter = time.Second
					}
					chatRateLimitedUntil = now.Add(retryAfter)
					_ = s.writeWSError(conn, "RATE_LIMITED", "rate limit exceeded")
					continue
				}
			}
			duplicate, replayErr := false, error(nil)
			if typ != "action.offline.rewards.get" {
				duplicate, replayErr = s.markReplay(context.Background(), "ws:"+typ, client.UserID, requestID, 2*time.Minute)
			}
			if replayErr != nil {
				_ = s.writeWSError(conn, "WS_REPLAY_GUARD_FAILED", "request replay guard failed")
				continue
			}
			if duplicate {
				_ = s.writeWSError(conn, "WS_DUPLICATE_REQUEST", "duplicate request")
				continue
			}
		}

		switch typ {
		case "auth.bind":
			deduplicatePreauthenticatedBind := deduplicateNextAuthBind
			deduplicateNextAuthBind = false
			var p bindPayload
			_ = json.Unmarshal(env["payload"], &p)
			if locked, _, lockErr := s.wsAuthGuard.IsLocked(wsIP, wsAuthAttemptKey, time.Now()); lockErr != nil {
				_ = s.writeWSError(conn, "WS_AUTH_GUARD_UNAVAILABLE", "ws auth guard backend unavailable")
				continue
			} else if locked {
				_ = s.writeWSError(conn, "WS_AUTH_LOCKED", "too many failed auth attempts, please retry later")
				continue
			}
			uid, sessionVersion, err := s.authUserWithSession(context.Background(), p.Token)
			if err != nil {
				if errors.Is(err, errAuthSessionRevoked) {
					_ = s.wsAuthGuard.RecordFailure(wsIP, wsAuthAttemptKey, time.Now())
					_ = s.writeWSError(conn, "WS_SESSION_REVOKED", "session revoked")
					_ = s.writeWSEvent(conn, "auth.kicked", fiber.Map{"reason": "SIGNED_IN_ELSEWHERE"})
					_ = conn.Close()
					return
				}
				if errors.Is(err, errAuthAccountBanned) {
					_ = s.wsAuthGuard.RecordFailure(wsIP, wsAuthAttemptKey, time.Now())
					_ = s.writeWSError(conn, "WS_ACCOUNT_BANNED", "account is banned")
					_ = s.writeWSEvent(conn, "auth.kicked", fiber.Map{"reason": "ACCOUNT_BANNED"})
					_ = conn.Close()
					return
				}
				if errors.Is(err, errAuthBackendBusy) {
					_ = s.writeWSError(conn, "WS_AUTH_BACKEND_BUSY", "auth backend busy")
					continue
				}
				_ = s.wsAuthGuard.RecordFailure(wsIP, wsAuthAttemptKey, time.Now())
				_ = s.writeWSError(conn, "WS_INVALID_TOKEN", "invalid token")
				continue
			}
			if guardErr := s.wsAuthGuard.Reset(wsIP, wsAuthAttemptKey); guardErr != nil {
				_ = s.writeWSError(conn, "WS_AUTH_GUARD_UNAVAILABLE", "ws auth guard backend unavailable")
				continue
			}
			if shouldDeduplicatePreauthenticatedWSBind(deduplicatePreauthenticatedBind, client.UserID, client.SessionVersion, presenceBoundUserID, presenceBoundSessionVersion, uid, sessionVersion) {
				continue
			}
			if presence, pErr := s.deps.Store.GetPresence(context.Background(), uid); pErr == nil {
				s.setUserPresenceCache(uid, presence.Status)
				now := time.Now()
				if _, err := s.markOfflineFastForwardFromPresence(uid, presence, now); err != nil {
					_ = s.writeWSError(conn, "OFFLINE_RECOVERY_UNAVAILABLE", "could not preserve offline recovery; reconnect to retry")
					continue
				}
			} else {
				_ = s.writeWSError(conn, "OFFLINE_RECOVERY_UNAVAILABLE", "could not load offline window; reconnect to retry")
				continue
			}
			client.UserID = uid
			client.SessionVersion = sessionVersion
			if presenceBoundUserID != 0 && (presenceBoundUserID != uid || presenceBoundSessionVersion != sessionVersion) {
				s.markUserOfflineByWS(presenceBoundUserID, presenceBoundSessionVersion)
				s.decrGlobalConnectionCount(presenceBoundUserID)
				presenceBoundUserID = 0
				presenceBoundSessionVersion = 0
			}
			if presenceBoundUserID == 0 {
				s.incrGlobalConnectionCount(uid)
				s.enforceConnectionLimit(uid, s.deps.Config.MaxConnectionsPerUser, client.ID)
				presenceBoundUserID = uid
				presenceBoundSessionVersion = sessionVersion
				s.markUserOnlineByWS(client.UserID, sessionVersion)
			}
			_ = s.writeWSEvent(conn, "auth.bound", fiber.Map{"userId": client.UserID})
			s.resetRealtimePatchState(client.UserID)
			s.pushRealtimeInit(client.UserID)
		case "game.state.get":
			state, err := s.deps.Game.State(context.Background(), client.UserID)
			if err != nil {
				_ = s.writeWSError(conn, "WS_GAME_STATE_FAILED", err.Error())
				continue
			}
			_ = s.writeWSEvent(conn, "game.state.patch", state)
		case "idle.tick.request":
			state, err := s.deps.Game.Tick(context.Background(), client.UserID)
			if err != nil {
				_ = s.writeWSError(conn, "WS_TICK_FAILED", err.Error())
				continue
			}
			_ = s.writeWSEvent(conn, "idle.tick.result", state)
		case "chat.send", "chat.global.send":
			var p chatPayload
			if err := json.Unmarshal(env["payload"], &p); err != nil {
				_ = s.writeWSError(conn, "WS_CHAT_INVALID_PAYLOAD", "invalid chat payload")
				continue
			}
			p.Content = strings.TrimSpace(p.Content)
			attachment, attachmentErr := s.resolveChatAttachment(context.Background(), client.UserID, p.Attachment)
			if attachmentErr != nil {
				if errors.Is(attachmentErr, store.ErrArenaReplayShareUnavailable) {
					_ = s.writeWSError(conn, "WS_ARENA_REPLAY_EXPIRED", "arena replay is no longer retained")
				} else if errors.Is(attachmentErr, store.ErrCombatTrialReplayUnavailable) {
					_ = s.writeWSError(conn, "WS_COMBAT_TRIAL_REPLAY_EXPIRED", "combat trial replay is no longer retained")
				} else if errors.Is(attachmentErr, errChatItemInstanceUnavailable) {
					_ = s.writeWSError(conn, "WS_CHAT_ITEM_INSTANCE_UNAVAILABLE", "item is no longer available in inventory")
				} else {
					_ = s.writeWSError(conn, "WS_CHAT_ATTACHMENT_INVALID", "invalid chat attachment")
				}
				continue
			}
			if p.Content == "" && len(attachment) == 0 {
				continue
			}
			channelRaw := p.Channel
			if typ == "chat.global.send" && strings.TrimSpace(channelRaw) == "" {
				channelRaw = fallbackChatChannelID
			}
			if roomID, roomChannel := parseBattleRoomChatChannelID(channelRaw); roomChannel {
				detail, allowed, accessErr := s.battleRoomChatAccess(context.Background(), client.UserID, roomID)
				if accessErr != nil {
					_ = s.writeWSError(conn, "WS_CHAT_CHANNEL_ACCESS_FAILED", "chat channel access check failed")
					continue
				}
				if !allowed {
					_ = s.writeWSError(conn, "WS_CHAT_CHANNEL_FORBIDDEN", "chat channel forbidden")
					continue
				}
				channelID := buildBattleRoomChatChannelID(roomID)
				if channelID == "" {
					_ = s.writeWSError(conn, "WS_CHAT_CHANNEL_INVALID", "invalid chat channel")
					continue
				}
				maxContentLength := s.deps.Config.GlobalSettings.Chat.MaxContentLength
				if maxContentLength <= 0 {
					maxContentLength = 300
				}
				if utf8.RuneCountInString(p.Content) > maxContentLength {
					_ = s.writeWSError(conn, "WS_CHAT_TOO_LONG", "chat too long")
					continue
				}
				if p.Content != "" {
					if code, message, messageKey, blocked := s.moderationWSBlocked(moderationRuleChat, p.Content, client.UserID); blocked {
						_ = s.writeWSErrorWithMessageKey(conn, code, message, messageKey)
						continue
					}
				}
				cosmeticMeta := s.resolveChatCosmeticMeta(context.Background(), client.UserID)
				cosmeticMeta.Attachment = attachment
				message, saveErr := s.deps.Store.SaveChatMessage(context.Background(), client.UserID, channelID, p.Content, cosmeticMeta)
				if saveErr != nil {
					_ = s.writeWSError(conn, "WS_CHAT_SEND_FAILED", "chat send failed")
					continue
				}
				payload := mapChatMessage(message)
				if strings.TrimSpace(detail.Room.RoomName) != "" {
					payload["channelLabel"] = strings.TrimSpace(detail.Room.RoomName)
				}
				s.pushBattleRoomEventToUsers(battleRoomMemberUserIDs(detail.Members), "chat.message", payload)
				continue
			}
			channel, ok := s.resolveChatChannel(channelRaw)
			if !ok {
				_ = s.writeWSError(conn, "WS_CHAT_CHANNEL_INVALID", "invalid chat channel")
				continue
			}
			if !channel.PlayerSendable {
				_ = s.writeWSError(conn, "WS_CHAT_CHANNEL_READONLY", "chat channel is read only")
				continue
			}
			maxContentLength := s.deps.Config.GlobalSettings.Chat.MaxContentLength
			if maxContentLength <= 0 {
				maxContentLength = 300
			}
			if utf8.RuneCountInString(p.Content) > maxContentLength {
				_ = s.writeWSError(conn, "WS_CHAT_TOO_LONG", "chat too long")
				continue
			}
			if p.Content != "" {
				if code, message, messageKey, blocked := s.moderationWSBlocked(moderationRuleChat, p.Content, client.UserID); blocked {
					_ = s.writeWSErrorWithMessageKey(conn, code, message, messageKey)
					continue
				}
			}
			cosmeticMeta := s.resolveChatCosmeticMeta(context.Background(), client.UserID)
			cosmeticMeta.Attachment = attachment
			message, saveErr := s.deps.Store.SaveChatMessage(context.Background(), client.UserID, channel.ID, p.Content, cosmeticMeta)
			if saveErr != nil {
				_ = s.writeWSError(conn, "WS_CHAT_SEND_FAILED", "chat send failed")
				continue
			}
			s.broadcastWSCluster("chat.message", mapChatMessage(message))
			if channel.ID == fallbackChatChannelID {
				s.broadcastWSCluster("chat.global.message", fiber.Map{
					"id":            message.ID,
					"channel":       message.Channel,
					"userId":        message.UserID,
					"username":      message.Username,
					"titlePrefixId": message.TitlePrefixID,
					"titleSuffixId": message.TitleSuffixID,
					"displayMeta":   message.DisplayMeta,
					"attachment":    publicChatAttachment(message.Attachment),
					"content":       message.Content,
					"ts":            message.SentAt.UnixMilli(),
				})
			}
		case "action.run":
			s.runActionCommand(conn, client.UserID, env["payload"])
		case "action.enqueue":
			s.runActionCommand(conn, client.UserID, env["payload"])
		case "action.cancel":
			s.cancelActionCommand(conn, client.UserID, env["payload"])
		case "action.queue.move":
			s.moveQueueItemCommand(conn, client.UserID, env["payload"])
		case "action.queue.run_now":
			s.runQueueItemNowCommand(conn, client.UserID, env["payload"])
		case "action.queue.get":
			specs := s.actionRuntime().ActionSpecMap()
			var (
				queue       []store.ActionQueueItem
				state       store.PlayerState
				attr        store.AttributeExp
				syncSummary store.ActionFastForwardSummary
				err         error
			)
			if s.hasOfflineFastForwardReady(client.UserID) {
				queue, err = s.deps.Store.ListActionQueue(context.Background(), client.UserID, 50)
				if err == nil {
					state, err = s.deps.Store.GetPlayerState(context.Background(), client.UserID)
				}
				if err == nil {
					attr, err = s.deps.Store.GetAttributeExp(context.Background(), client.UserID)
				}
			} else {
				ctx, cancel := context.WithTimeout(context.Background(), actionCommandTimeout)
				result, advanceErr := s.actionRuntime().AdvanceDue(ctx, client.UserID)
				cancel()
				if advanceErr != nil {
					err = advanceErr
				} else {
					specs = result.Specs
					queue = result.Queue
					state = result.State
					attr = result.Attr
					syncSummary = result.Summary
				}
			}
			if err != nil {
				log.Printf("ws action.queue.get failed user_id=%d err=%v", client.UserID, err)
				var listErr error
				var stateErr error
				var attrErr error
				queue, listErr = s.deps.Store.ListActionQueue(context.Background(), client.UserID, 50)
				state, stateErr = s.deps.Store.GetPlayerState(context.Background(), client.UserID)
				attr, attrErr = s.deps.Store.GetAttributeExp(context.Background(), client.UserID)
				if listErr != nil || stateErr != nil || attrErr != nil {
					_ = s.writeWSError(conn, "WS_ACTION_QUEUE_FAILED", "load action queue failed")
					continue
				}
			}
			syncSummary.FishingChanged = syncSummary.FishingChanged || actionQueueContainsFishing(queue)
			if !actionruntime.EquipmentChanged(syncSummary) {
				s.pushEquipmentDomain(client.UserID)
			}
			s.syncActionRuntime(client.UserID, specs, queue, state, attr, syncSummary)
		case "action.offline.rewards.get":
			s.handleOfflineFastForwardRequest(conn, client.UserID, env["payload"], requestID)
		case "presence.ping":
			_ = s.writeWSEvent(conn, "presence.pong", fiber.Map{"serverTs": time.Now().UnixMilli()})
		case "battle.stream.set":
			var p battleStreamSubscribePayload
			_ = json.Unmarshal(env["payload"], &p)
			kind := normalizeBattleStreamKind(p.Kind)
			s.setBattleStreamKindSubscribed(client.ID, kind, p.Enabled)
			_ = s.writeWSEvent(conn, "battle.stream.state", fiber.Map{"kind": kind, "enabled": p.Enabled})
		default:
			_ = s.writeWSError(conn, "WS_UNKNOWN_MESSAGE_TYPE", "unknown message type")
		}
	}
}

func (s *Server) wsClientIP(conn *websocket.Conn) string {
	if conn == nil {
		return ""
	}
	if raw := conn.Locals("wsIP"); raw != nil {
		if ip, ok := raw.(string); ok {
			ip = strings.TrimSpace(ip)
			if ip != "" {
				return ip
			}
		}
	}
	return ""
}

func (s *Server) authUserID(c *fiber.Ctx) (int64, error) {
	startAt := time.Now()
	defer func() {
		if collector := reqmetrics.FromContext(c.Context()); collector != nil {
			collector.AddAuth(time.Since(startAt))
		}
	}()
	if fromLocal := c.Locals("authUserID"); fromLocal != nil {
		if userID, ok := fromLocal.(int64); ok {
			return userID, nil
		}
	}
	raw := c.Get("Authorization")
	token := extractBearerToken(raw)
	if token == "" {
		return 0, errors.New("missing bearer token")
	}
	userID, _, err := s.authUserWithSession(c.Context(), token)
	if err != nil {
		if errors.Is(err, auth.ErrInvalidToken) {
			return 0, errors.New("invalid token")
		}
		if errors.Is(err, errAuthSessionRevoked) {
			return 0, errAuthSessionRevoked
		}
		if errors.Is(err, errAuthAccountBanned) {
			return 0, errAuthAccountBanned
		}
		if errors.Is(err, errAuthBackendBusy) {
			return 0, errAuthBackendBusy
		}
		return 0, err
	}
	return userID, nil
}

func (s *Server) authUserWithSession(ctx context.Context, token string) (int64, int64, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	parseStart := time.Now()
	claims, err := parseAuthClaimsWithSession(s, token)
	if collector := reqmetrics.FromContext(ctx); collector != nil {
		collector.AddAuthParse(time.Since(parseStart))
	}
	if err != nil {
		return 0, 0, err
	}

	authCtx := ctx
	var cancel context.CancelFunc
	if timeout := time.Duration(s.deps.Config.AuthDBTimeoutMS) * time.Millisecond; timeout > 0 {
		authCtx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	dbStart := time.Now()
	authState, err := loadAuthGateState(s, authCtx, claims.UserID)
	if collector := reqmetrics.FromContext(ctx); collector != nil {
		collector.AddAuthDB(time.Since(dbStart))
	}
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(authCtx.Err(), context.DeadlineExceeded) {
			return 0, 0, errAuthBackendBusy
		}
		if errors.Is(err, store.ErrUserNotFound) {
			return 0, 0, errAuthSessionRevoked
		}
		return 0, 0, err
	}
	if !authState.SessionVersionValid(claims.SessionVersion) {
		return 0, 0, errAuthSessionRevoked
	}
	if authState.IsBanned(time.Now()) {
		return 0, 0, errAuthAccountBanned
	}
	return claims.UserID, claims.SessionVersion, nil
}

func extractBearerToken(raw string) string {
	if !strings.HasPrefix(raw, "Bearer ") {
		return ""
	}
	return strings.TrimPrefix(raw, "Bearer ")
}
