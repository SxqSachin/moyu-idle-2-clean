package httpapi

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestDestructiveRecycleHTTPRejectsUnsafeRequests(t *testing.T) {
	for _, confirm := range []bool{false, true} {
		for _, body := range []string{
			``, `{`, `{}`, `{"items":[]}`,
			`{"items":[{"instanceId":11,"quantity":2}]}`,
			`{"items":[{"instanceId":11,"totalGold":999999}]}`,
			`{"items":[{"instanceId":11}],"ignoreAffixes":true}`,
			`{"items":[{"instanceId":0}]}`,
			`{"items":[{"instanceId":11},{"instanceId":11}]}`,
		} {
			t.Run(body, func(t *testing.T) {
				s := &Server{App: fiber.New()}
				s.Post("/test", func(c *fiber.Ctx) error {
					c.Locals("authUserID", int64(42))
					return s.handleDestructiveRecycleRequest(c, confirm)
				})
				req := httptest.NewRequest("POST", "/test", strings.NewReader(body))
				req.Header.Set("Content-Type", "application/json")
				res, err := s.Test(req)
				if err != nil {
					t.Fatal(err)
				}
				defer res.Body.Close()
				if res.StatusCode != 400 && res.StatusCode != 409 {
					t.Fatalf("status=%d", res.StatusCode)
				}
			})
		}
	}
}

func TestDestructiveRecycleHTTPRequiresConfirmationToken(t *testing.T) {
	s := &Server{App: fiber.New()}
	s.Post("/test", func(c *fiber.Ctx) error { c.Locals("authUserID", int64(42)); return s.handleDestructiveRecycle(c) })
	req := httptest.NewRequest("POST", "/test", strings.NewReader(`{"items":[{"instanceId":11}]}`))
	req.Header.Set("Content-Type", "application/json")
	res, err := s.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 409 {
		t.Fatalf("status=%d", res.StatusCode)
	}
	var payload struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if payload.Error.Code != "ITEM_DESTRUCTIVE_RECYCLE_CHANGED" {
		t.Fatalf("code=%s", payload.Error.Code)
	}
}

func TestDestructiveRecycleHTTPRequiresAuthentication(t *testing.T) {
	s := &Server{App: fiber.New()}
	s.Post("/test", s.handleDestructiveRecycleQuote)
	res, err := s.Test(httptest.NewRequest("POST", "/test", strings.NewReader(`{"items":[{"instanceId":11}]}`)))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 401 {
		t.Fatalf("status=%d", res.StatusCode)
	}
}
