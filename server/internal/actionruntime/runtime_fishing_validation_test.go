package actionruntime

import (
	"errors"
	"strings"
	"testing"

	"mysecondwebgame/server/internal/store"
)

func TestMapActionStoreErrorFishingValidationPreservesPublicReason(t *testing.T) {
	validation := &store.FishingValidationError{Reason: "COMPONENT_INCOMPATIBLE", Field: "rod"}
	err := errors.Join(validation, errors.New("private database diagnostic"))
	got := mapActionStoreError(err, "fallback")
	if got.Code != "WS_FISHING_SETTINGS_COMPONENT_INCOMPATIBLE_ROD" || got.Message != validation.Error() || got.Debug != "" {
		t.Fatalf("mapping = %+v", got)
	}
	if strings.Contains(got.Message, "private") {
		t.Fatal("private diagnostic leaked")
	}
}

func TestMapActionStoreErrorFishingLegacyFallbackRemainsCompatible(t *testing.T) {
	for _, err := range []error{store.ErrFishingArgsInvalid, store.ErrFishingLoadoutInvalid} {
		if got := mapActionStoreError(err, "fallback"); got.Code != "WS_FISHING_INVALID" {
			t.Fatalf("legacy error = %+v", got)
		}
	}
}
