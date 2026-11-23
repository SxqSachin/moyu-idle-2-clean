package httpapi

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"mysecondwebgame/server/internal/app"
	"mysecondwebgame/server/internal/config"
)

func TestWriteAPIErrorIncludesDebugInDevelopment(t *testing.T) {
	s := &Server{
		App:  fiber.New(),
		deps: app.Dependencies{Config: config.Config{AppEnv: "development"}},
	}
	s.Get("/test", func(c *fiber.Ctx) error {
		return s.writeAPIError(c, fiber.StatusBadRequest, "AUTH_INVALID_PAYLOAD", "Invalid request payload", "json parse failed")
	})

	resp, err := s.Test(httptest.NewRequest("GET", "/test", nil))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	var payload apiErrorResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}

	if payload.Code != "AUTH_INVALID_PAYLOAD" {
		t.Fatalf("unexpected code: %s", payload.Code)
	}
	if payload.Debug == "" {
		t.Fatalf("expected debug in development response")
	}
}

func TestWriteAPIErrorHidesDebugOutsideDevelopment(t *testing.T) {
	s := &Server{
		App:  fiber.New(),
		deps: app.Dependencies{Config: config.Config{AppEnv: "production"}},
	}
	s.Get("/test", func(c *fiber.Ctx) error {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "AUTH_CREATE_USER_FAILED", "Failed to create user", "db timeout")
	})

	resp, err := s.Test(httptest.NewRequest("GET", "/test", nil))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	var payload map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}

	if _, ok := payload["debug"]; ok {
		t.Fatalf("did not expect debug in non-development response")
	}
}

func TestWriteAPISuccessExposesTimingFieldsWithoutCompressionEnvelope(t *testing.T) {
	s := &Server{
		App:  fiber.New(),
		deps: app.Dependencies{Config: config.Config{AppEnv: "development"}},
	}
	s.Get("/test", func(c *fiber.Ctx) error {
		return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{"status": "ok"})
	})

	resp, err := s.Test(httptest.NewRequest("GET", "/test", nil))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	var payload map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}

	if _, ok := payload["compressed"]; ok {
		t.Fatalf("did not expect compressed envelope in success response")
	}

	for _, key := range []string{"total", "auth", "pg", "nats", "redis", "server", "business"} {
		if _, ok := payload[key]; !ok {
			t.Fatalf("expected timing field %q in response", key)
		}
	}
}
