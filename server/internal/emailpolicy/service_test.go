package emailpolicy

import (
	"os"
	"path/filepath"
	"testing"
)

func TestServiceBlocksExactAndSubdomainsWithoutSuffixFalsePositive(t *testing.T) {
	service, err := New(Config{
		Enabled:        true,
		BlockedDomains: []string{"PUGB.VIP"},
	})
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	tests := []struct {
		email   string
		blocked bool
	}{
		{email: "player@pugb.vip", blocked: true},
		{email: "player@PUGB.VIP.", blocked: true},
		{email: "player@inbox.pugb.vip", blocked: true},
		{email: "player@notpugb.vip", blocked: false},
		{email: "player@example.com", blocked: false},
	}
	for _, test := range tests {
		if got := service.IsBlocked(test.email); got != test.blocked {
			t.Fatalf("IsBlocked(%q) = %v, want %v", test.email, got, test.blocked)
		}
	}
}

func TestServiceAllowlistOverridesMoreGeneralBlock(t *testing.T) {
	service, err := New(Config{
		Enabled:        true,
		BlockedDomains: []string{"pugb.vip"},
		AllowedDomains: []string{"trusted.pugb.vip"},
	})
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	if service.IsBlocked("player@trusted.pugb.vip") {
		t.Fatal("allowlisted domain was blocked")
	}
	if service.IsBlocked("player@mail.trusted.pugb.vip") {
		t.Fatal("allowlisted subdomain was blocked")
	}
	if !service.IsBlocked("player@other.pugb.vip") {
		t.Fatal("non-allowlisted subdomain was allowed")
	}
}

func TestServiceLoadsBlocklistAndNormalizesInternationalDomain(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "domains.txt")
	raw := []byte("# confirmed disposable providers\n*.pugb.vip\nB\u00dcCHER.example # IDN\n")
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatalf("write blocklist: %v", err)
	}
	service, err := New(Config{Enabled: true, BlocklistPath: path})
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	if !service.IsBlocked("player@mail.pugb.vip") {
		t.Fatal("file-backed rule did not block subdomain")
	}
	if !service.IsBlocked("player@b\u00fccher.example") {
		t.Fatal("international domain was not normalized")
	}
	if service.BlockedDomainCount() != 2 {
		t.Fatalf("BlockedDomainCount() = %d, want 2", service.BlockedDomainCount())
	}
}

func TestServiceDisabledDoesNotReadOrBlock(t *testing.T) {
	service, err := New(Config{
		Enabled:        false,
		BlocklistPath:  "/missing/domains.txt",
		BlockedDomains: []string{"pugb.vip"},
	})
	if err != nil {
		t.Fatalf("disabled New failed: %v", err)
	}
	if service.IsBlocked("player@pugb.vip") {
		t.Fatal("disabled policy blocked email")
	}
}

func TestServiceRejectsUnsafeConfiguration(t *testing.T) {
	tests := []Config{
		{Enabled: true},
		{Enabled: true, BlockedDomains: []string{"pugb"}},
		{Enabled: true, BlockedDomains: []string{"co.uk"}},
		{Enabled: true, BlockedDomains: []string{"bad@example.com"}},
		{Enabled: true, BlocklistPath: "/missing/domains.txt"},
	}
	for _, cfg := range tests {
		if _, err := New(cfg); err == nil {
			t.Fatalf("New(%+v) unexpectedly succeeded", cfg)
		}
	}
}
