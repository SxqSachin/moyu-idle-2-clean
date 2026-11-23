package store

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const (
	FishingTripEventPageLimit      = 100
	FishingTripEventRetentionLimit = 10_000
)

type FishingTripEventPage struct {
	Items           []FishingTripEvent `json:"items"`
	Total           int64              `json:"total"`
	NextCursor      int64              `json:"nextCursor,omitempty"`
	HistoryComplete bool               `json:"historyComplete"`
}

func (s *Store) ListFishingTripEvents(ctx context.Context, userID int64, tripID string, cursor int64) (FishingTripEventPage, error) {
	tripID = strings.TrimSpace(tripID)
	if userID <= 0 || tripID == "" || len(tripID) > 120 || cursor < 0 {
		return FishingTripEventPage{}, ErrFishingArgsInvalid
	}

	var (
		summaryRaw      []byte
		total, min, max int64
	)
	err := s.db.QueryRow(ctx, `
		SELECT t.summary_jsonb, COUNT(e.sequence), COALESCE(MIN(e.sequence), 0), COALESCE(MAX(e.sequence), 0)
		FROM fishing_trips t
		LEFT JOIN fishing_trip_events e ON e.trip_id = t.id
		WHERE t.user_id = $1 AND t.id = $2 AND t.status IN ('completed', 'stopped')
		GROUP BY t.id, t.summary_jsonb
	`, userID, tripID).Scan(&summaryRaw, &total, &min, &max)
	if errors.Is(err, pgx.ErrNoRows) {
		return FishingTripEventPage{}, ErrFishingTripNotFound
	}
	if err != nil {
		return FishingTripEventPage{}, err
	}

	if total == 0 {
		var summary FishingTripSummary
		if len(summaryRaw) > 0 {
			if err := json.Unmarshal(summaryRaw, &summary); err != nil {
				return FishingTripEventPage{}, err
			}
		}
		return paginateLegacyFishingTripEvents(summary.RecentEvents, cursor), nil
	}

	rows, err := s.db.Query(ctx, `
		SELECT e.sequence, e.event_jsonb
		FROM fishing_trip_events e
		JOIN fishing_trips t ON t.id = e.trip_id
		WHERE t.user_id = $1 AND e.trip_id = $2 AND ($3::bigint = 0 OR e.sequence < $3)
		ORDER BY e.sequence DESC
		LIMIT $4
	`, userID, tripID, cursor, FishingTripEventPageLimit+1)
	if err != nil {
		return FishingTripEventPage{}, err
	}
	defer rows.Close()

	items := make([]FishingTripEvent, 0, FishingTripEventPageLimit+1)
	for rows.Next() {
		var (
			sequence int64
			raw      []byte
		)
		if err := rows.Scan(&sequence, &raw); err != nil {
			return FishingTripEventPage{}, err
		}
		var event FishingTripEvent
		if err := json.Unmarshal(raw, &event); err != nil {
			return FishingTripEventPage{}, err
		}
		event.Sequence = sequence
		items = append(items, event)
	}
	if err := rows.Err(); err != nil {
		return FishingTripEventPage{}, err
	}

	page := FishingTripEventPage{
		Items:           items,
		Total:           total,
		HistoryComplete: fishingTripEventHistoryComplete(total, min, max),
	}
	if len(page.Items) > FishingTripEventPageLimit {
		page.Items = page.Items[:FishingTripEventPageLimit]
		page.NextCursor = page.Items[len(page.Items)-1].Sequence
	}
	return page, nil
}

func paginateLegacyFishingTripEvents(events []FishingTripEvent, cursor int64) FishingTripEventPage {
	items := append([]FishingTripEvent(nil), events...)
	sort.Slice(items, func(i, j int) bool { return items[i].Sequence > items[j].Sequence })
	total := int64(len(items))
	min, max := int64(0), int64(0)
	if len(items) > 0 {
		max = items[0].Sequence
		min = items[len(items)-1].Sequence
	}
	if cursor > 0 {
		filtered := items[:0]
		for _, event := range items {
			if event.Sequence < cursor {
				filtered = append(filtered, event)
			}
		}
		items = filtered
	}
	page := FishingTripEventPage{
		Items:           items,
		Total:           total,
		HistoryComplete: fishingTripEventHistoryComplete(total, min, max),
	}
	if len(page.Items) > FishingTripEventPageLimit {
		page.Items = page.Items[:FishingTripEventPageLimit]
		page.NextCursor = page.Items[len(page.Items)-1].Sequence
	}
	return page
}

func fishingTripEventHistoryComplete(total, min, max int64) bool {
	return total > 0 && min == 1 && max == total
}

func (s *Store) persistFishingTripEventsTx(ctx context.Context, tx pgx.Tx, tripID string, events []FishingTripEvent) error {
	if strings.TrimSpace(tripID) == "" || len(events) == 0 {
		return nil
	}
	raw, err := json.Marshal(events)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO fishing_trip_events(trip_id, sequence, event_jsonb)
		SELECT $1, (event.value->>'sequence')::bigint, event.value
		FROM jsonb_array_elements($2::jsonb) AS event(value)
		WHERE jsonb_typeof(event.value) = 'object'
		  AND event.value->>'sequence' ~ '^[1-9][0-9]*$'
		ON CONFLICT (trip_id, sequence) DO NOTHING
	`, tripID, string(raw))
	if err != nil {
		return err
	}
	lastSequence := int64(0)
	for _, event := range events {
		if event.Sequence > lastSequence {
			lastSequence = event.Sequence
		}
	}
	if lastSequence <= FishingTripEventRetentionLimit {
		return nil
	}
	_, err = tx.Exec(ctx, `
		DELETE FROM fishing_trip_events
		WHERE trip_id = $1 AND sequence <= $2
	`, tripID, lastSequence-FishingTripEventRetentionLimit)
	return err
}

func (s *Store) pruneFishingTripEventsTx(ctx context.Context, tx pgx.Tx, userID int64) error {
	_, err := tx.Exec(ctx, `
		DELETE FROM fishing_trip_events event
		USING fishing_trips trip
		WHERE trip.id = event.trip_id
		  AND trip.user_id = $1
		  AND trip.status IN ('completed', 'stopped')
		  AND trip.id NOT IN (
			SELECT id
			FROM fishing_trips
			WHERE user_id = $1 AND status IN ('completed', 'stopped')
			ORDER BY started_at DESC, id DESC
			LIMIT $2
		  )
	`, userID, FishingTripHistoryLimit)
	return err
}

func (s *Store) appendFishingTripFinishedEventTx(ctx context.Context, tx pgx.Tx, tripID, reason string, at time.Time, summary FishingTripSummary) (FishingTripSummary, error) {
	var lastSequence int64
	if err := tx.QueryRow(ctx, `
		SELECT COALESCE(MAX(sequence), 0)
		FROM fishing_trip_events
		WHERE trip_id = $1
	`, tripID).Scan(&lastSequence); err != nil {
		return FishingTripSummary{}, err
	}
	runtime := fishingExecutorRuntime{TimelineSeq: lastSequence, Summary: summary}
	appendFishingTripEvent(&runtime, FishingTripEvent{Kind: "finished", Reason: reason, At: at})
	if err := s.persistFishingTripEventsTx(ctx, tx, tripID, runtime.pendingEvents); err != nil {
		return FishingTripSummary{}, err
	}
	return runtime.Summary, nil
}
