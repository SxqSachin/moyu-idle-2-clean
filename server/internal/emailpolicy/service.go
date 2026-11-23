package emailpolicy

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"golang.org/x/net/idna"
	"golang.org/x/net/publicsuffix"
)

type Config struct {
	Enabled        bool
	BlocklistPath  string
	BlockedDomains []string
	AllowedDomains []string
}

type Service struct {
	enabled bool
	blocked map[string]struct{}
	allowed map[string]struct{}
}

func New(cfg Config) (*Service, error) {
	service := &Service{
		enabled: cfg.Enabled,
		blocked: make(map[string]struct{}),
		allowed: make(map[string]struct{}),
	}
	if !cfg.Enabled {
		return service, nil
	}

	for index, raw := range cfg.BlockedDomains {
		if err := addDomainRule(service.blocked, raw); err != nil {
			return nil, fmt.Errorf("blocked domain %d: %w", index, err)
		}
	}
	for index, raw := range cfg.AllowedDomains {
		if err := addDomainRule(service.allowed, raw); err != nil {
			return nil, fmt.Errorf("allowed domain %d: %w", index, err)
		}
	}

	blocklistPath := strings.TrimSpace(cfg.BlocklistPath)
	if blocklistPath != "" {
		if err := loadDomainRules(blocklistPath, service.blocked); err != nil {
			return nil, err
		}
	}
	if len(service.blocked) == 0 {
		return nil, fmt.Errorf("at least one blocked domain or blocklist path is required")
	}
	return service, nil
}

func (s *Service) IsBlocked(email string) bool {
	if s == nil || !s.enabled {
		return false
	}
	domain, err := domainFromEmail(email)
	if err != nil {
		return false
	}
	if matchesDomainRule(domain, s.allowed) {
		return false
	}
	return matchesDomainRule(domain, s.blocked)
}

func (s *Service) BlockedDomainCount() int {
	if s == nil {
		return 0
	}
	return len(s.blocked)
}

func domainFromEmail(email string) (string, error) {
	value := strings.TrimSpace(email)
	separator := strings.LastIndexByte(value, '@')
	if separator <= 0 || separator == len(value)-1 {
		return "", fmt.Errorf("email domain is missing")
	}
	return normalizeDomain(value[separator+1:])
}

func normalizeDomain(raw string) (string, error) {
	domain := strings.TrimSpace(strings.ToLower(raw))
	domain = strings.TrimPrefix(domain, "*.")
	domain = strings.TrimPrefix(domain, ".")
	domain = strings.TrimSuffix(domain, ".")
	if domain == "" || strings.ContainsAny(domain, "@/\\: \t\r\n") {
		return "", fmt.Errorf("invalid domain %q", raw)
	}
	ascii, err := idna.Lookup.ToASCII(domain)
	if err != nil {
		return "", fmt.Errorf("normalize domain %q: %w", raw, err)
	}
	ascii = strings.ToLower(strings.TrimSuffix(ascii, "."))
	if len(ascii) > 253 || !strings.Contains(ascii, ".") {
		return "", fmt.Errorf("invalid domain %q", raw)
	}
	if suffix, _ := publicsuffix.PublicSuffix(ascii); suffix == ascii {
		return "", fmt.Errorf("domain %q is a public suffix", raw)
	}
	for _, label := range strings.Split(ascii, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return "", fmt.Errorf("invalid domain %q", raw)
		}
		for _, char := range label {
			if (char < 'a' || char > 'z') && (char < '0' || char > '9') && char != '-' {
				return "", fmt.Errorf("invalid domain %q", raw)
			}
		}
	}
	return ascii, nil
}

func addDomainRule(target map[string]struct{}, raw string) error {
	domain, err := normalizeDomain(raw)
	if err != nil {
		return err
	}
	target[domain] = struct{}{}
	return nil
}

func loadDomainRules(path string, target map[string]struct{}) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open email domain blocklist %q: %w", path, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		fields := strings.Fields(scanner.Text())
		if len(fields) == 0 {
			continue
		}
		entry := strings.TrimPrefix(fields[0], "\ufeff")
		if strings.HasPrefix(entry, "#") || strings.HasPrefix(entry, "//") {
			continue
		}
		if err := addDomainRule(target, entry); err != nil {
			return fmt.Errorf("email domain blocklist %q line %d: %w", path, lineNumber, err)
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read email domain blocklist %q: %w", path, err)
	}
	return nil
}

func matchesDomainRule(domain string, rules map[string]struct{}) bool {
	for candidate := domain; candidate != ""; {
		if _, ok := rules[candidate]; ok {
			return true
		}
		separator := strings.IndexByte(candidate, '.')
		if separator < 0 {
			return false
		}
		candidate = candidate[separator+1:]
	}
	return false
}
