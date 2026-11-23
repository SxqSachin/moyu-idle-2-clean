package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"mysecondwebgame/server/internal/store"

	"github.com/gofiber/fiber/v2"
)

func TestFishingPondResourceErrorsRemainSpecific(t *testing.T) {
	tests := []struct {
		name string
		err  error
		code string
	}{
		{name: "lineage reagent", err: store.ErrFishingPondLineageReagentMissing, code: "FISHING_POND_REAGENT_MISSING"},
		{name: "broodstock mash", err: store.ErrFishingPondBroodstockMashMissing, code: "FISHING_POND_MASH_MISSING"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := &Server{App: fiber.New()}
			server.Post("/pond-error", func(c *fiber.Ctx) error {
				return server.writeFishingError(c, test.err, "pond action failed")
			})

			response, err := server.Test(httptest.NewRequest(http.MethodPost, "/pond-error", nil))
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
			if payload.OK || payload.Error.Code != test.code {
				t.Fatalf("unexpected response payload: %+v", payload)
			}
		})
	}
}

func TestFishingPondBatchRejectsMalformedBody(t *testing.T) {
	server := &Server{App: fiber.New()}
	server.Post("/pond/fish/batch", func(c *fiber.Ctx) error {
		c.Locals("authUserID", int64(42))
		return server.handleFishingPondFishBatchAdd(c)
	})

	response, err := server.Test(httptest.NewRequest(http.MethodPost, "/pond/fish/batch", nil))
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
	if payload.OK || payload.Error.Code != "FISHING_POND_INVALID" {
		t.Fatalf("unexpected response payload: %+v", payload)
	}
}
