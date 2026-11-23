package httpapi

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestDecodeStrictSkillForgePayloadRejectsArbitraryDSL(t *testing.T) {
	raw := []byte(`{
		"sourceKind":"blank",
		"graph":{
			"version":1,
			"source":{"kind":"blank"},
			"nodes":[{"id":"source","kind":"source","position":{"x":0,"y":0},"customDsl":{"damage":999999}}],
			"edges":[]
		}
	}`)
	var request skillForgeCreateDraftRequest
	if err := decodeStrictJSON(raw, &request); err == nil {
		t.Fatal("expected arbitrary nested DSL field to be rejected")
	}
}

func TestDecodeStrictSkillForgePayloadAcceptsDomainGraph(t *testing.T) {
	raw := []byte(`{
		"sourceKind":"blank",
		"graph":{
			"version":1,
			"source":{"kind":"blank"},
			"nodes":[
				{"id":"source","kind":"source","position":{"x":0,"y":0}},
				{"id":"output","kind":"output","position":{"x":100,"y":0}}
			],
			"edges":[]
		}
	}`)
	var request skillForgeCreateDraftRequest
	if err := decodeStrictJSON(raw, &request); err != nil {
		t.Fatalf("decodeStrictJSON failed: %v", err)
	}
}

func TestSkillForgePathIDUnescapesNamespacedIDs(t *testing.T) {
	app := fiber.New()
	app.Get("/drafts/:draftId", func(c *fiber.Ctx) error {
		return c.SendString(skillForgePathID(c, "draftId"))
	})

	request, err := http.NewRequest(http.MethodGet, "http://example.test/drafts/draft%3Aa409829f-d425-4380-ba68-1c87ee89febc", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	response, err := app.Test(request)
	if err != nil {
		t.Fatalf("serve request: %v", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	if got, want := string(body), "draft:a409829f-d425-4380-ba68-1c87ee89febc"; got != want {
		t.Fatalf("decoded path id = %q, want %q", got, want)
	}
}

func TestSkillForgeIdempotencyKeyIsBoundedAndOpaque(t *testing.T) {
	valid := []string{"skill-forge-draft:abc-123", "retry_2.1"}
	for _, value := range valid {
		if !validSkillForgeIdempotencyKey(value) {
			t.Fatalf("expected %q to be valid", value)
		}
	}
	invalid := []string{"", "contains space", "contains/slash", string(make([]byte, 129))}
	for _, value := range invalid {
		if validSkillForgeIdempotencyKey(value) {
			t.Fatalf("expected %q to be invalid", value)
		}
	}
}

func TestSkillForgeForgePayloadRequiresRevisionAndGraphHash(t *testing.T) {
	validHash := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	var request skillForgeForgeRequest
	if err := decodeStrictJSON([]byte(`{"expectedRevision":7,"expectedGraphHash":"`+validHash+`"}`), &request); err != nil {
		t.Fatalf("decode forge request: %v", err)
	}
	if request.ExpectedRevision != 7 || !validSkillForgeGraphHash(request.ExpectedGraphHash) {
		t.Fatalf("unexpected forge expectation: %+v", request)
	}
	for _, value := range []string{"", "abc", strings.Repeat("g", 64), strings.Repeat("a", 63)} {
		if validSkillForgeGraphHash(value) {
			t.Fatalf("expected graph hash %q to be invalid", value)
		}
	}
	if err := decodeStrictJSON([]byte(`{"expectedRevision":7,"expectedGraphHash":"`+validHash+`","compiledSkill":{"damage":999}}`), &request); err == nil {
		t.Fatal("expected arbitrary forge payload fields to be rejected")
	}
}

func TestSkillForgeDurableIdempotencyRoutesBypassReplayGuard(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{path: "/api/alchemy/skill-forge/drafts/draft:1/forge", want: true},
		{path: "/api/alchemy/skill-forge/results/revision:1/accept", want: true},
		{path: "/api/alchemy/skill-forge/results/revision:1/discard", want: true},
		{path: "/api/alchemy/skill-forge/drafts", want: false},
		{path: "/api/alchemy/skill-forge/drafts/draft:1/preview", want: false},
		{path: "/api/alchemy/skill-forge/skills/custom:1/archive", want: false},
		{path: "/api/market/buy", want: false},
	}
	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			if got := hasDurableHTTPIdempotency(test.path); got != test.want {
				t.Fatalf("hasDurableHTTPIdempotency(%q) = %v, want %v", test.path, got, test.want)
			}
		})
	}
}

func TestInvalidatePlayerProfileCachesClearsPrivateAndPublicViews(t *testing.T) {
	const userID int64 = 42
	server := &Server{
		playerProfileMem: map[int64]playerProfileCacheEntry{userID: {}},
		playerPublicMem:  map[int64]playerProfileCacheEntry{userID: {}},
	}

	server.invalidatePlayerProfileCaches(userID)

	if _, exists := server.playerProfileMem[userID]; exists {
		t.Fatal("private profile cache was not invalidated")
	}
	if _, exists := server.playerPublicMem[userID]; exists {
		t.Fatal("public profile cache was not invalidated")
	}
}
