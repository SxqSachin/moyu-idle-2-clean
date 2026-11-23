package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"mysecondwebgame/server/internal/store"

	"github.com/gofiber/fiber/v2"
)

func TestFishingSaleHTTPEntryPointsRejectMalformedBody(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		handler func(*Server, *fiber.Ctx) error
	}{
		{name: "quote", path: "/quote", handler: func(server *Server, c *fiber.Ctx) error { return server.handleFishingSaleQuote(c) }},
		{name: "sell", path: "/sell", handler: func(server *Server, c *fiber.Ctx) error { return server.handleFishingSale(c) }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := &Server{App: fiber.New()}
			server.Post(test.path, func(c *fiber.Ctx) error {
				c.Locals("authUserID", int64(42))
				return test.handler(server, c)
			})

			request := httptest.NewRequest(http.MethodPost, test.path, nil)
			response, err := server.Test(request)
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			defer response.Body.Close()

			if response.StatusCode != fiber.StatusBadRequest {
				t.Fatalf("status = %d, want %d", response.StatusCode, fiber.StatusBadRequest)
			}
			var payload struct {
				OK    bool `json:"ok"`
				Error struct {
					Code string `json:"code"`
				} `json:"error"`
			}
			if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if payload.OK || payload.Error.Code != "FISHING_SALE_INVALID" {
				t.Fatalf("unexpected response payload: %+v", payload)
			}
		})
	}
}

func TestFishingSaleQuoteChangeReturnsConflict(t *testing.T) {
	server := &Server{App: fiber.New()}
	server.Post("/quote-changed", func(c *fiber.Ctx) error {
		return server.writeFishingError(c, store.ErrFishingSaleQuoteChanged, "sell fish failed")
	})

	response, err := server.Test(httptest.NewRequest(http.MethodPost, "/quote-changed", nil))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != fiber.StatusConflict {
		t.Fatalf("status = %d, want %d", response.StatusCode, fiber.StatusConflict)
	}

	var payload struct {
		OK    bool `json:"ok"`
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.OK || payload.Error.Code != "FISHING_SALE_QUOTE_CHANGED" {
		t.Fatalf("unexpected response payload: %+v", payload)
	}
}
