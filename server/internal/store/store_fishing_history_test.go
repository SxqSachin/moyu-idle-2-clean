package store

import (
	"os"
	"strings"
	"testing"
)

func TestFishingTripHistoryIsBoundedAndExcludesRunningTrips(t *testing.T) {
	if FishingTripHistoryLimit != 20 {
		t.Fatalf("FishingTripHistoryLimit = %d, want 20", FishingTripHistoryLimit)
	}

	raw, err := os.ReadFile("store_fishing.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(raw)
	start := strings.Index(body, "func (s *Store) ListRecentFishingTrips")
	end := strings.Index(body, "const fishingTripQueueMissingStopReason")
	if start < 0 || end <= start {
		t.Fatal("fishing trip history function boundaries not found")
	}
	query := body[start:end]
	for _, required := range []string{
		"status IN ('completed', 'stopped')",
		"ORDER BY started_at DESC, id DESC",
		"LIMIT $2",
		"FishingTripHistoryLimit",
		"scanFishingTripState(rows)",
		"rows.Err()",
	} {
		if !strings.Contains(query, required) {
			t.Fatalf("fishing trip history query missing %q", required)
		}
	}
}
