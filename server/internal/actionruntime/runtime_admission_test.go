package actionruntime

import (
	"context"
	"encoding/json"
	"testing"

	"mysecondwebgame/server/internal/catalog"
	"mysecondwebgame/server/internal/store"
)

func TestSpecialQueueCommandsRejectIncompleteRequests(t *testing.T) {
	data := catalog.Data{}
	cases := []struct {
		id   string
		code string
	}{
		{store.FishingTripActionID, "WS_FISHING_SETTINGS_SPOT_REQUIRED"},
		{"equipment_enhance", "WS_ENHANCE_ARGS_INVALID"},
		{"building_upgrade", "WS_BUILDING_ARGS_INVALID"},
		{"alchemy.convert", "WS_ALCHEMY_ARGS_INVALID"},
		{"alchemy.bench", "WS_ALCHEMY_ARGS_INVALID"},
		{"alchemy.transmute", "WS_ALCHEMY_ARGS_INVALID"},
		{"alchemy.smelt", "WS_ALCHEMY_ARGS_INVALID"},
		{"alchemy.inherit", "WS_ALCHEMY_ARGS_INVALID"},
	}
	for _, tc := range cases {
		data.Actions = append(data.Actions, catalog.ActionDefinition{ID: tc.id})
	}
	rt := New(&store.Store{}, data)
	for _, tc := range cases {
		t.Run(tc.id, func(t *testing.T) {
			for _, batch := range []bool{false, true} {
				for _, actor := range []string{"player", "kitty"} {
					// Forged runtime fields do not replace the executor's required args.
					step := map[string]any{"actionId": " " + tc.id + " ", "count": 1, "args": map[string]any{
						"executorId": "generic", "runtime": map[string]any{"durationMs": 1}, "runContext": map[string]any{},
					}}
					command := map[string]any{"actionId": step["actionId"], "count": 1, "args": step["args"], "mode": "run", "actorType": actor, "actorId": 42}
					if batch {
						command = map[string]any{"steps": []any{step}, "insertMode": "run_now_like_prepend", "actorType": actor, "actorId": 42}
					}
					payload, err := json.Marshal(command)
					if err != nil {
						t.Fatal(err)
					}
					var result *Result
					var serviceErr *ServiceError
					if batch {
						result, serviceErr = rt.EnqueueActionBatch(context.Background(), 7, payload)
					} else {
						result, serviceErr = rt.RunAction(context.Background(), 7, payload)
					}
					if result != nil || serviceErr == nil || serviceErr.Code != tc.code || serviceErr.Debug != "" {
						t.Fatalf("batch=%v actor=%s: result=%+v error=%+v, want %s", batch, actor, result, serviceErr, tc.code)
					}
				}
			}
		})
	}
}
