package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"mysecondwebgame/server/internal/store"
)

func TestFishingValidationHTTPResponseIncludesReasonWithoutDebug(t *testing.T) {
	s := &Server{App: fiber.New()}
	validation := &store.FishingValidationError{Reason: "COMPONENT_REQUIRED", Field: "leader"}
	s.Post("/validation", func(c *fiber.Ctx) error {
		return s.writeFishingError(c, errors.Join(validation, errors.New("private diagnostic")), "save failed")
	})
	response, err := s.Test(httptest.NewRequest(http.MethodPost, "/validation", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var payload struct {
		Error apiErrorBody `json:"error"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusBadRequest || payload.Error.Code != validation.Code() || payload.Error.Message != validation.Error() {
		t.Fatalf("status=%d error=%+v", response.StatusCode, payload.Error)
	}
	raw, _ := json.Marshal(payload)
	if strings.Contains(string(raw), "private") {
		t.Fatal("private diagnostic leaked")
	}
	for _, code := range []string{validation.Code(), "FISHING_SETTINGS_BOTTOM_LAYER_MISMATCH", "FISHING_SETTINGS_SUPPLY_REQUIRED_MAIN_LINE"} {
		if got := actionErrorHTTPStatus("WS_" + code); got != http.StatusBadRequest {
			t.Fatalf("%s status=%d", code, got)
		}
	}
}
