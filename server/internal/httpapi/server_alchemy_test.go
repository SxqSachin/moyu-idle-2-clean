package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
	"mysecondwebgame/server/internal/app"
)

func TestAlchemyInheritImmediateCooldownUsesRedis(t *testing.T) {
	mr := miniredis.RunT(t)
	rd := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rd.Close()

	s := &Server{
		App:  fiber.New(),
		deps: app.Dependencies{Redis: rd},
	}
	s.Post("/cooldown", func(c *fiber.Ctx) error {
		if retryAfter := s.checkAlchemyInheritImmediateCooldown(c, 42); retryAfter > 0 {
			c.Set("Retry-After", strconv.Itoa(ceilDurationSeconds(retryAfter)))
			return c.SendStatus(fiber.StatusTooManyRequests)
		}
		return c.SendStatus(fiber.StatusOK)
	})

	first := performAlchemyCooldownRequest(t, s)
	if first.StatusCode != fiber.StatusOK {
		t.Fatalf("first status=%d, want %d", first.StatusCode, fiber.StatusOK)
	}
	_ = first.Body.Close()

	second := performAlchemyCooldownRequest(t, s)
	if second.StatusCode != fiber.StatusTooManyRequests {
		t.Fatalf("second status=%d, want %d", second.StatusCode, fiber.StatusTooManyRequests)
	}
	if got := second.Header.Get("Retry-After"); got == "" {
		t.Fatalf("second Retry-After is empty")
	}
	_ = second.Body.Close()

	mr.FastForward(alchemyInheritImmediateCooldown)
	third := performAlchemyCooldownRequest(t, s)
	if third.StatusCode != fiber.StatusOK {
		t.Fatalf("third status=%d, want %d", third.StatusCode, fiber.StatusOK)
	}
	_ = third.Body.Close()
}

func performAlchemyCooldownRequest(t *testing.T, s *Server) *http.Response {
	t.Helper()
	resp, err := s.Test(httptest.NewRequest(http.MethodPost, "/cooldown", nil))
	if err != nil {
		t.Fatalf("request cooldown route: %v", err)
	}
	return resp
}
