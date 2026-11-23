package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"mysecondwebgame/server/internal/store"

	"github.com/gofiber/fiber/v2"
)

type publicFishingPondReaderStub struct {
	requestedUserID int64
	result          store.FishingPondPublicOverview
	err             error
}

func (stub *publicFishingPondReaderStub) GetPublicFishingPond(_ context.Context, userID int64) (store.FishingPondPublicOverview, error) {
	stub.requestedUserID = userID
	return stub.result, stub.err
}

func TestPlayerProfilePondReadsTargetWithoutUsingViewerAsOwner(t *testing.T) {
	server := &Server{App: fiber.New()}
	reader := &publicFishingPondReaderStub{result: store.FishingPondPublicOverview{OwnerUserID: 99, Built: true}}
	server.Get("/player/profile/:userID/pond", func(c *fiber.Ctx) error {
		c.Locals("authUserID", int64(42))
		return server.servePlayerProfilePondGet(c, reader)
	})

	response, err := server.Test(httptest.NewRequest(http.MethodGet, "/player/profile/99/pond", nil))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != fiber.StatusOK {
		t.Fatalf("status=%d, want %d", response.StatusCode, fiber.StatusOK)
	}
	if reader.requestedUserID != 99 {
		t.Fatalf("reader target=%d, want profile owner 99", reader.requestedUserID)
	}
	var payload struct {
		OK   bool                            `json:"ok"`
		Data store.FishingPondPublicOverview `json:"data"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !payload.OK || payload.Data.OwnerUserID != 99 {
		t.Fatalf("unexpected response: %+v", payload)
	}
}

func TestPlayerProfilePondRejectsUnauthenticatedAndInvalidRequests(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		auth       bool
		readerErr  error
		wantStatus int
		wantCode   string
	}{
		{name: "unauthenticated", path: "/player/profile/99/pond", wantStatus: fiber.StatusUnauthorized, wantCode: codeAuthInvalidToken},
		{name: "invalid target", path: "/player/profile/nope/pond", auth: true, wantStatus: fiber.StatusBadRequest, wantCode: "PLAYER_PROFILE_POND_INVALID_USER_ID"},
		{name: "missing target", path: "/player/profile/99/pond", auth: true, readerErr: store.ErrUserNotFound, wantStatus: fiber.StatusNotFound, wantCode: "PLAYER_PROFILE_POND_NOT_FOUND"},
		{name: "read failure", path: "/player/profile/99/pond", auth: true, readerErr: errors.New("database unavailable"), wantStatus: fiber.StatusInternalServerError, wantCode: "PLAYER_PROFILE_POND_GET_FAILED"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := &Server{App: fiber.New()}
			reader := &publicFishingPondReaderStub{err: test.readerErr}
			server.Get("/player/profile/:userID/pond", func(c *fiber.Ctx) error {
				if test.auth {
					c.Locals("authUserID", int64(42))
				}
				return server.servePlayerProfilePondGet(c, reader)
			})

			response, err := server.Test(httptest.NewRequest(http.MethodGet, test.path, nil))
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			defer response.Body.Close()
			if response.StatusCode != test.wantStatus {
				t.Fatalf("status=%d, want %d", response.StatusCode, test.wantStatus)
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
			if payload.OK || payload.Error.Code != test.wantCode {
				t.Fatalf("unexpected response: %+v", payload)
			}
		})
	}
}
