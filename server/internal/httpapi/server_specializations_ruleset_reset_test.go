package httpapi

import (
	"os"
	"strings"
	"testing"
)

func TestSpecializationGetSyncsDomainsAfterRulesetReset(t *testing.T) {
	raw, err := os.ReadFile("server_specializations.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	start := strings.Index(text, "func (s *Server) handlePlayerSpecializationGet")
	if start < 0 {
		t.Fatal("specialization get handler missing")
	}
	body := text[start:]
	if next := strings.Index(body, "\nfunc (s *Server)"); next >= 0 {
		body = body[:next]
	}
	resetAt := strings.Index(body, "view.RulesetReset != nil")
	syncAt := strings.Index(body, "s.syncAfterSpecializationMutation(userID)")
	responseAt := strings.Index(body, "s.writeAPISuccess")
	if resetAt < 0 || syncAt <= resetAt || responseAt <= syncAt {
		t.Fatal("ruleset reset GET no longer invalidates caches and synchronizes inventory, equipment, and attributes")
	}
}
