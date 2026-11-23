package store

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

func TestFishingTripEventHistoryCompletenessRequiresContiguousSequence(t *testing.T) {
	if FishingTripEventRetentionLimit < 10_000 {
		t.Fatalf("event retention limit=%d, want enough for a full 24-hour trip", FishingTripEventRetentionLimit)
	}
	tests := []struct {
		name            string
		total, min, max int64
		want            bool
	}{
		{name: "complete", total: 35, min: 1, max: 35, want: true},
		{name: "legacy tail", total: 24, min: 12, max: 35, want: false},
		{name: "gap", total: 34, min: 1, max: 35, want: false},
		{name: "empty", want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := fishingTripEventHistoryComplete(test.total, test.min, test.max); got != test.want {
				t.Fatalf("complete=%t, want %t", got, test.want)
			}
		})
	}
}

func TestLegacyFishingTripEventsAreNewestFirstAndPaged(t *testing.T) {
	events := make([]FishingTripEvent, FishingTripEventPageLimit+7)
	for index := range events {
		events[index] = FishingTripEvent{
			Sequence: int64(index + 1),
			Kind:     "cast",
			At:       time.Unix(int64(index+1), 0),
		}
	}
	first := paginateLegacyFishingTripEvents(events, 0)
	if len(first.Items) != FishingTripEventPageLimit || first.NextCursor != 8 {
		t.Fatalf("first page len=%d cursor=%d", len(first.Items), first.NextCursor)
	}
	if first.Items[0].Sequence != int64(len(events)) || first.Items[len(first.Items)-1].Sequence != 8 {
		t.Fatalf("first page range=%d..%d", first.Items[0].Sequence, first.Items[len(first.Items)-1].Sequence)
	}
	if !first.HistoryComplete || first.Total != int64(len(events)) {
		t.Fatalf("first page metadata=%+v", first)
	}
	second := paginateLegacyFishingTripEvents(events, first.NextCursor)
	if len(second.Items) != 7 || second.NextCursor != 0 || second.Items[0].Sequence != 7 || second.Items[6].Sequence != 1 {
		t.Fatalf("second page=%+v", second)
	}
}

func TestFishingTripEventHistoryMigrationBackfillsOnlyRetainedTrips(t *testing.T) {
	raw, err := os.ReadFile("../../schema/migrations/20260903_fishing_trip_event_history.sql")
	if err != nil {
		t.Fatal(err)
	}
	compact := strings.Join(strings.Fields(strings.ToLower(string(raw))), " ")
	for _, required := range []string{
		"create table if not exists fishing_trip_events",
		"primary key(trip_id, sequence)",
		"recent_rank <= 20",
		"status = 'running'",
		"jsonb_array_elements",
		"on conflict (trip_id, sequence) do nothing",
	} {
		if !strings.Contains(compact, required) {
			t.Fatalf("event history migration missing %s", fmt.Sprintf("%q", required))
		}
	}
	down, err := os.ReadFile("../../schema/migrations/20260903_fishing_trip_event_history_down.sql")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.ToLower(string(down)), "drop table if exists fishing_trip_events") {
		t.Fatal("event history down migration must drop fishing_trip_events")
	}
}
