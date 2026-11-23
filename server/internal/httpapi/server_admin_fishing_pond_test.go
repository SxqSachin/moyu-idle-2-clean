package httpapi

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"mysecondwebgame/server/internal/app"
	"mysecondwebgame/server/internal/config"
	"mysecondwebgame/server/internal/store"

	"github.com/gofiber/fiber/v2"
)

func newAdminFishingPondTestServer() *Server {
	s := &Server{
		App:               fiber.New(),
		deps:              app.Dependencies{Config: config.Config{AppEnv: "development", ReleaseAnnounceToken: "admin-token"}},
		mutationSyncQueue: make(chan mutationSyncScope, 8),
	}
	s.Post("/api/admin/fishing/pond/fish", s.releaseAdminMiddleware(), s.handleAdminFishingPondFishAdd)
	return s
}

func TestAdminFishingPondFishAddRequiresReleaseToken(t *testing.T) {
	s := newAdminFishingPondTestServer()
	req := httptest.NewRequest(http.MethodPost, "/api/admin/fishing/pond/fish", bytes.NewBufferString(`{"userId":1}`))
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	resp, err := s.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status=%d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestAdminFishingPondFishAddRejectsInvalidUser(t *testing.T) {
	s := newAdminFishingPondTestServer()
	req := httptest.NewRequest(http.MethodPost, "/api/admin/fishing/pond/fish", bytes.NewBufferString(`{"userId":0}`))
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	req.Header.Set("X-Release-Token", "admin-token")
	resp, err := s.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status=%d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestAdminFishingPondFishAddPassesOptionalOverrides(t *testing.T) {
	s := newAdminFishingPondTestServer()
	original := adminFishingPondFishAddExecutor
	t.Cleanup(func() { adminFishingPondFishAddExecutor = original })
	var captured store.AdminFishingPondFishInput
	adminFishingPondFishAddExecutor = func(_ *Server, _ context.Context, input store.AdminFishingPondFishInput) (store.AdminFishingPondFishResult, error) {
		captured = input
		return store.AdminFishingPondFishResult{InstanceID: 81, Fish: store.FishInstanceData{SpeciesID: input.SpeciesID, Genes: store.FishGenes{Mutations: input.Mutations}}}, nil
	}

	body := `{"userId":7,"speciesId":"common_carp","mutations":["prismatic","shimmering"],"ageDays":1095,"bioAgeFraction":0.25,"sex":"female","lengthCm":42.5,"weightKg":2.4,"condition":1.1,"health":0.9,"generation":3,"genes":{"sizePotential":0.5,"lifespan":0.8},"longevityLimitDays":12000,"genomeRevealed":true,"satiety":85,"preferredFeedHours":96,"name":"Starlight","labelColor":"sky"}`
	req := httptest.NewRequest(http.MethodPost, "/api/admin/fishing/pond/fish", bytes.NewBufferString(body))
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	req.Header.Set("X-Release-Token", "admin-token")
	resp, err := s.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d, want %d", resp.StatusCode, http.StatusOK)
	}
	if captured.UserID != 7 || captured.SpeciesID != "common_carp" || !reflect.DeepEqual(captured.Mutations, []string{"prismatic", "shimmering"}) {
		t.Fatalf("captured identity=%+v", captured)
	}
	if captured.AgeDays == nil || *captured.AgeDays != 1095 || captured.LengthCM == nil || *captured.LengthCM != 42.5 || captured.Genes == nil || captured.Genes.SizePotential == nil || *captured.Genes.SizePotential != 0.5 {
		t.Fatalf("captured optional overrides=%+v", captured)
	}
	select {
	case scope := <-s.mutationSyncQueue:
		if !scope.Inventory || !reflect.DeepEqual(scope.UserIDs, []int64{7}) {
			t.Fatalf("sync scope=%+v", scope)
		}
	default:
		t.Fatal("expected inventory synchronization")
	}
}

func TestAdminFishingPondFishAddMapsInvalidOverride(t *testing.T) {
	s := newAdminFishingPondTestServer()
	original := adminFishingPondFishAddExecutor
	t.Cleanup(func() { adminFishingPondFishAddExecutor = original })
	adminFishingPondFishAddExecutor = func(_ *Server, _ context.Context, _ store.AdminFishingPondFishInput) (store.AdminFishingPondFishResult, error) {
		return store.AdminFishingPondFishResult{}, store.ErrAdminFishingPondFishInvalid
	}
	req := httptest.NewRequest(http.MethodPost, "/api/admin/fishing/pond/fish", bytes.NewBufferString(`{"userId":7,"mutations":["unknown"]}`))
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	req.Header.Set("X-Release-Token", "admin-token")
	resp, err := s.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status=%d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}
