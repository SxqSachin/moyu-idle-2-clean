package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"mysecondwebgame/server/internal/store"

	"github.com/gofiber/fiber/v2"
)

func TestSpecializationPointBreakdownSurvivesGetMutationAndConfirmationEnvelopes(t *testing.T) {
	nextLevel := int64(500)
	status := store.SpecializationStatus{
		TreeID:          "player_combat_specialization",
		RulesetVersion:  "specialization-v6",
		BattleLevel:     499,
		FormulaPoints:   63,
		TotalPoints:     63,
		AvailablePoints: 63,
		PointBreakdown: store.SpecializationPointBreakdown{
			UnlockGrantPoints:    2,
			LevelMilestonePoints: 61,
			MaxPoints:            65,
			NextPointBattleLevel: &nextLevel,
			NextPointPoints:      2,
			MilestoneBands: []store.SpecializationPointMilestoneBand{{
				BandID:                   "level_cap",
				LabelKey:                 "game.specialization.pointSource.levelMilestone",
				FromBattleLevel:          500,
				ToBattleLevel:            500,
				EveryLevels:              1,
				PointsPerGrant:           2,
				NextMilestoneBattleLevel: &nextLevel,
			}},
			Sources: []store.SpecializationPointSource{{
				SourceID:            "level_cap:500",
				Kind:                "level_milestone",
				LabelKey:            "game.specialization.pointSource.levelMilestone",
				BandID:              "level_cap",
				RequiredBattleLevel: &nextLevel,
				Points:              2,
				Earned:              false,
			}},
		},
	}
	mutation := store.SpecializationMutationResult{
		RequiresConfirm: true,
		Status:          status,
	}
	s := &Server{App: fiber.New()}
	s.Get("/get", func(c *fiber.Ctx) error {
		return s.writeAPISuccess(c, fiber.StatusOK, store.SpecializationView{Status: status})
	})
	s.Post("/allocate", func(c *fiber.Ctx) error {
		return s.writeAPISuccess(c, fiber.StatusOK, mutation)
	})
	s.Post("/reset", func(c *fiber.Ctx) error {
		return s.writeSpecializationImpactConfirmation(c, mutation)
	})

	for _, test := range []struct {
		method string
		path   string
		status int
	}{
		{method: http.MethodGet, path: "/get", status: http.StatusOK},
		{method: http.MethodPost, path: "/allocate", status: http.StatusOK},
		{method: http.MethodPost, path: "/reset", status: http.StatusConflict},
	} {
		t.Run(test.path, func(t *testing.T) {
			resp, err := s.Test(httptest.NewRequest(test.method, test.path, nil))
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != test.status {
				t.Fatalf("status=%d, want %d", resp.StatusCode, test.status)
			}
			var payload map[string]any
			if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
				t.Fatal(err)
			}
			data, ok := payload["data"].(map[string]any)
			if !ok {
				t.Fatalf("missing data envelope: %#v", payload)
			}
			statusPayload, ok := data["status"].(map[string]any)
			if !ok {
				t.Fatalf("missing status envelope: %#v", data)
			}
			breakdown, ok := statusPayload["pointBreakdown"].(map[string]any)
			if !ok {
				t.Fatalf("missing point breakdown: %#v", statusPayload)
			}
			if breakdown["unlockGrantPoints"] != float64(2) || breakdown["levelMilestonePoints"] != float64(61) || breakdown["nextPointBattleLevel"] != float64(500) || breakdown["nextPointPoints"] != float64(2) || breakdown["maxPoints"] != float64(65) {
				t.Fatalf("unexpected point breakdown: %#v", breakdown)
			}
			bands, ok := breakdown["milestoneBands"].([]any)
			if !ok || len(bands) != 1 || bands[0].(map[string]any)["nextMilestoneBattleLevel"] != float64(500) {
				t.Fatalf("unexpected milestone band breakdown: %#v", breakdown["milestoneBands"])
			}
		})
	}
}

func TestSpecializationActiveRootLimitHasStableConflictCode(t *testing.T) {
	s := &Server{App: fiber.New()}
	s.Get("/root-limit", func(c *fiber.Ctx) error {
		return s.writePlayerSpecializationError(c, store.ErrSpecializationActiveRootLimit)
	})
	resp, err := s.Test(httptest.NewRequest(http.MethodGet, "/root-limit", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status=%d, want %d", resp.StatusCode, http.StatusConflict)
	}
	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body["code"] != "SPECIALIZATION_ACTIVE_ROOT_LIMIT" {
		t.Fatalf("unexpected root-limit error body: %#v", body)
	}
}
