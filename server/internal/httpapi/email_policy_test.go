package httpapi

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"mysecondwebgame/server/internal/emailpolicy"
)

func TestRegisterRejectsBlockedEmailBeforeCreatingUser(t *testing.T) {
	s, cleanup := newCaptchaTestServer(t, "development")
	defer cleanup()

	policy, err := emailpolicy.New(emailpolicy.Config{
		Enabled:        true,
		BlockedDomains: []string{"pugb.vip"},
	})
	if err != nil {
		t.Fatalf("new email policy: %v", err)
	}
	s.deps.EmailPolicy = policy
	s.Post("/api/register", s.handleRegister)

	const userAgent = "email-policy-test"
	token := issueTokenViaValidateEndpoint(t, s, userAgent)
	body := []byte(`{"username":"blocked_email_user","email":"player@sub.pugb.vip","password":"123456","acceptedTerms":true,"acceptedTermsVersion":"test","captchaToken":"` + token + `"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/register", bytes.NewReader(body))
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	req.Header.Set(fiber.HeaderUserAgent, userAgent)
	resp, err := s.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, fiber.StatusBadRequest)
	}
	payload := decodeAPIError(t, resp)
	if payload.Code != codeAuthEmailDomainBlocked {
		t.Fatalf("code = %q, want %q", payload.Code, codeAuthEmailDomainBlocked)
	}
	if payload.Error.MessageKey != emailDomainBlockedMessageKey {
		t.Fatalf("message key = %q, want %q", payload.Error.MessageKey, emailDomainBlockedMessageKey)
	}
}

func TestNilEmailPolicyPreservesExistingBehavior(t *testing.T) {
	s := &Server{}
	if s.isEmailDomainBlocked("player@pugb.vip") {
		t.Fatal("nil policy must not block email")
	}
}
