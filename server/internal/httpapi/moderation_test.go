package httpapi

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"mysecondwebgame/server/internal/app"
	"mysecondwebgame/server/internal/config"
	"mysecondwebgame/server/internal/moderation"
)

func TestRejectModeratedAPIBlocksUsername(t *testing.T) {
	svc, err := moderation.New(moderation.Config{
		Enabled:            true,
		VocabularyJSONPath: "../../../packages/Sensitive-lexicon-1.2/compiled/sensitive-words.json",
		BlockMessageKey:    "error.contentModeration.blocked",
		Targets:            moderation.TargetConfig{Username: true},
	})
	if err != nil {
		t.Fatalf("new moderation: %v", err)
	}
	s := &Server{
		App: fiber.New(),
		deps: app.Dependencies{
			Config:     config.Config{AppEnv: "development"},
			Moderation: svc,
		},
	}
	s.Post("/test", func(c *fiber.Ctx) error {
		return s.rejectModeratedAPI(c, moderationRuleUsername, "xjp", 42)
	})

	resp, err := s.Test(httptest.NewRequest("POST", "/test", nil))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("status=%d, want %d", resp.StatusCode, fiber.StatusBadRequest)
	}
	payload := decodeAPIError(t, resp)
	if payload.Code != codeAuthUsernameBlocked {
		t.Fatalf("code=%s, want %s", payload.Code, codeAuthUsernameBlocked)
	}
	if payload.Error.MessageKey != "error.contentModeration.blocked" {
		t.Fatalf("messageKey=%q", payload.Error.MessageKey)
	}
}

func TestModerationWSBlockedChat(t *testing.T) {
	svc, err := moderation.New(moderation.Config{
		Enabled:            true,
		VocabularyJSONPath: "../../../packages/Sensitive-lexicon-1.2/compiled/sensitive-words.json",
		BlockMessageKey:    "error.contentModeration.blocked",
		Targets:            moderation.TargetConfig{Chat: true},
	})
	if err != nil {
		t.Fatalf("new moderation: %v", err)
	}
	s := &Server{deps: app.Dependencies{Moderation: svc}}
	code, message, messageKey, blocked := s.moderationWSBlocked(moderationRuleChat, "这里有爱液", 7)
	if !blocked {
		t.Fatalf("expected ws chat moderation block")
	}
	if code != "WS_CHAT_CONTENT_BLOCKED" {
		t.Fatalf("code=%s", code)
	}
	if message == "" {
		t.Fatalf("expected block message")
	}
	if messageKey != "error.contentModeration.blocked" {
		t.Fatalf("messageKey=%q", messageKey)
	}
}
