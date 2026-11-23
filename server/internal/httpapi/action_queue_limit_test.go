package httpapi

import (
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestActionErrorHTTPStatusQueueLimitReached(t *testing.T) {
	if got := actionErrorHTTPStatus("WS_ACTION_QUEUE_LIMIT_REACHED"); got != fiber.StatusConflict {
		t.Fatalf("actionErrorHTTPStatus() = %d, want %d", got, fiber.StatusConflict)
	}
}

func TestActionErrorHTTPStatusFishingTripActive(t *testing.T) {
	if got := actionErrorHTTPStatus("WS_FISHING_TRIP_ACTIVE"); got != fiber.StatusConflict {
		t.Fatalf("actionErrorHTTPStatus() = %d, want %d", got, fiber.StatusConflict)
	}
}

func TestActionErrorHTTPStatusSpecialActionValidation(t *testing.T) {
	for _, code := range []string{
		"WS_ALCHEMY_ARGS_INVALID", "WS_BUILDING_ARGS_INVALID", "WS_ENHANCE_ARGS_INVALID",
		"WS_ACTION_INVALID_COUNT", "WS_FISHING_SETTINGS_SPOT_REQUIRED", "WS_FISHING_SETTINGS_SUPPLY_REQUIRED_MAIN_LINE",
	} {
		if got := actionErrorHTTPStatus(code); got != fiber.StatusBadRequest {
			t.Errorf("actionErrorHTTPStatus(%q)=%d, want %d", code, got, fiber.StatusBadRequest)
		}
	}
}
