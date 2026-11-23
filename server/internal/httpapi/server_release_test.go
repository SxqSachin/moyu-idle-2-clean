package httpapi

import (
	"testing"

	"mysecondwebgame/server/internal/app"
	"mysecondwebgame/server/internal/config"
)

func TestNormalizeReleaseKind(t *testing.T) {
	if got := normalizeReleaseKind("force"); got != "force" {
		t.Fatalf("normalizeReleaseKind(force)=%q, want force", got)
	}
	if got := normalizeReleaseKind("SOFT"); got != "soft" {
		t.Fatalf("normalizeReleaseKind(SOFT)=%q, want soft", got)
	}
	if got := normalizeReleaseKind("unknown"); got != "soft" {
		t.Fatalf("normalizeReleaseKind(unknown)=%q, want soft", got)
	}
}

func TestClampReleaseDelay(t *testing.T) {
	if got := clampReleaseDelay(0, 0); got != defaultReleaseReloadDelayS {
		t.Fatalf("clampReleaseDelay(0,0)=%d, want %d", got, defaultReleaseReloadDelayS)
	}
	if got := clampReleaseDelay(9999, 120); got != maxReleaseReloadDelayS {
		t.Fatalf("clampReleaseDelay(9999,120)=%d, want %d", got, maxReleaseReloadDelayS)
	}
	if got := clampReleaseDelay(30, 120); got != 30 {
		t.Fatalf("clampReleaseDelay(30,120)=%d, want 30", got)
	}
}

func TestLoadReleaseAnnouncementFallback(t *testing.T) {
	s := &Server{
		deps: app.Dependencies{
			Config: config.Config{
				ReleaseBuildID:        "release-x",
				ReleaseReloadDelaySec: 75,
			},
		},
	}
	state := s.loadReleaseAnnouncement(t.Context())
	if state.ReleaseID != "release-x" {
		t.Fatalf("release id=%q, want release-x", state.ReleaseID)
	}
	if state.RecommendedDelayS != 75 {
		t.Fatalf("delay=%d, want 75", state.RecommendedDelayS)
	}
	if state.Kind != "soft" {
		t.Fatalf("kind=%q, want soft", state.Kind)
	}
}
