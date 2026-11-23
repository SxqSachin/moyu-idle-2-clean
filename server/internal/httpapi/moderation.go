package httpapi

import (
	"log"
	"strings"

	"github.com/gofiber/fiber/v2"
	"mysecondwebgame/server/internal/moderation"
)

type moderationRule struct {
	target         moderation.Target
	apiCode        string
	wsCode         string
	messageKey     string
	defaultMessage string
}

var (
	moderationRuleChat = moderationRule{
		target:         moderation.TargetChat,
		apiCode:        codeChatContentBlocked,
		wsCode:         "WS_CHAT_CONTENT_BLOCKED",
		messageKey:     "error.contentModeration.blocked",
		defaultMessage: "Content blocked",
	}
	moderationRuleUsername = moderationRule{
		target:         moderation.TargetUsername,
		apiCode:        codeAuthUsernameBlocked,
		messageKey:     "error.contentModeration.blocked",
		defaultMessage: "Content blocked",
	}
	moderationRuleProfileUsername = moderationRule{
		target:         moderation.TargetUsername,
		apiCode:        codeProfileUsernameBlocked,
		messageKey:     "error.contentModeration.blocked",
		defaultMessage: "Content blocked",
	}
	moderationRuleKittyName = moderationRule{
		target:         moderation.TargetKittyName,
		apiCode:        codeCatteryAdoptNameBlocked,
		messageKey:     "error.contentModeration.blocked",
		defaultMessage: "Content blocked",
	}
	moderationRuleBattleRoomName = moderationRule{
		target:         moderation.TargetBattleRoomName,
		apiCode:        codeBattleRoomNameBlocked,
		messageKey:     "error.contentModeration.blocked",
		defaultMessage: "Content blocked",
	}
	moderationRuleLoadoutName = moderationRule{
		target:         moderation.TargetLoadoutName,
		apiCode:        codePlayerLoadoutNameBlocked,
		messageKey:     "error.contentModeration.blocked",
		defaultMessage: "Content blocked",
	}
	moderationRuleItemCustomName = moderationRule{
		target:         moderation.TargetItemCustomName,
		apiCode:        codeItemRenameNameBlocked,
		messageKey:     "error.contentModeration.blocked",
		defaultMessage: "Content blocked",
	}
)

func (s *Server) moderationBlockMessageKey() string {
	if s.deps.Moderation != nil {
		if key := strings.TrimSpace(s.deps.Moderation.BlockMessageKey()); key != "" {
			return key
		}
	}
	return "error.contentModeration.blocked"
}

func (s *Server) isModerationBlocked(rule moderationRule, text string) bool {
	return s.deps.Moderation != nil && s.deps.Moderation.Match(rule.target, text)
}

func (s *Server) rejectModeratedAPI(c *fiber.Ctx, rule moderationRule, text string, userID int64) error {
	if !s.isModerationBlocked(rule, text) {
		return nil
	}
	log.Printf("content moderation blocked target=%s user_id=%d", rule.target, userID)
	return s.writeAPIErrorWithMessageKey(c, fiber.StatusBadRequest, rule.apiCode, rule.defaultMessage, s.moderationBlockMessageKey(), "")
}

func (s *Server) moderationWSBlocked(rule moderationRule, text string, userID int64) (string, string, string, bool) {
	if !s.isModerationBlocked(rule, text) {
		return "", "", "", false
	}
	log.Printf("content moderation blocked target=%s user_id=%d", rule.target, userID)
	return rule.wsCode, rule.defaultMessage, s.moderationBlockMessageKey(), true
}
