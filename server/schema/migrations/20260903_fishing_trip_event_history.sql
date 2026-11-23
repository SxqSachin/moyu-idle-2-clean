-- Keep complete event timelines for the most recent fishing trips without
-- inflating the action queue runtime payload.
CREATE TABLE IF NOT EXISTS fishing_trip_events (
  trip_id     TEXT   NOT NULL REFERENCES fishing_trips(id) ON DELETE CASCADE,
  sequence    BIGINT NOT NULL CHECK (sequence > 0),
  event_jsonb JSONB  NOT NULL CHECK (jsonb_typeof(event_jsonb) = 'object'),
  PRIMARY KEY(trip_id, sequence)
);

-- Older trips only carried a short recentEvents window. Preserve what remains
-- for the latest 20 completed trips and any trip that is currently running.
WITH ranked_trips AS (
  SELECT id, summary_jsonb,
         row_number() OVER (PARTITION BY user_id ORDER BY started_at DESC, id DESC) AS recent_rank
  FROM fishing_trips
  WHERE status IN ('completed', 'stopped')
), retained_trips AS (
  SELECT id, summary_jsonb
  FROM ranked_trips
  WHERE recent_rank <= 20
  UNION ALL
  SELECT id, summary_jsonb
  FROM fishing_trips
  WHERE status = 'running'
)
INSERT INTO fishing_trip_events(trip_id, sequence, event_jsonb)
SELECT retained.id,
       CASE
         WHEN event.value->>'sequence' ~ '^[1-9][0-9]*$' THEN (event.value->>'sequence')::BIGINT
         ELSE event.ordinality::BIGINT
       END,
       event.value
FROM retained_trips retained
CROSS JOIN LATERAL jsonb_array_elements(
  CASE
    WHEN jsonb_typeof(retained.summary_jsonb->'recentEvents') = 'array'
      THEN retained.summary_jsonb->'recentEvents'
    ELSE '[]'::jsonb
  END
) WITH ORDINALITY AS event(value, ordinality)
WHERE jsonb_typeof(event.value) = 'object'
ON CONFLICT (trip_id, sequence) DO NOTHING;
