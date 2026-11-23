package moderation

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode"
)

type Target string

const (
	TargetChat           Target = "chat"
	TargetUsername       Target = "username"
	TargetKittyName      Target = "kitty_name"
	TargetBattleRoomName Target = "battle_room_name"
	TargetLoadoutName    Target = "loadout_name"
	TargetItemCustomName Target = "item_custom_name"
)

type Config struct {
	Enabled            bool
	VocabularyJSONPath string
	BlockMessageKey    string
	Targets            TargetConfig
}

type TargetConfig struct {
	Chat           bool
	Username       bool
	KittyName      bool
	BattleRoomName bool
	LoadoutName    bool
	ItemCustomName bool
}

type Service struct {
	enabled         bool
	blockMessageKey string
	targets         TargetConfig
	root            *node
}

type node struct {
	children map[rune]*node
	terminal bool
}

func New(cfg Config) (*Service, error) {
	if !cfg.Enabled {
		return &Service{
			enabled:         false,
			blockMessageKey: strings.TrimSpace(cfg.BlockMessageKey),
			targets:         cfg.Targets,
			root:            &node{children: map[rune]*node{}},
		}, nil
	}
	vocabularyPath := strings.TrimSpace(cfg.VocabularyJSONPath)
	if vocabularyPath == "" {
		return nil, fmt.Errorf("vocabulary json path is required")
	}
	terms, err := loadTermsFromJSONFile(vocabularyPath)
	if err != nil {
		return nil, err
	}
	root := &node{children: map[rune]*node{}}
	for _, term := range terms {
		insert(root, []rune(normalize(term)))
	}
	return &Service{
		enabled:         true,
		blockMessageKey: strings.TrimSpace(cfg.BlockMessageKey),
		targets:         cfg.Targets,
		root:            root,
	}, nil
}

func (s *Service) Enabled() bool {
	return s != nil && s.enabled
}

func (s *Service) BlockMessageKey() string {
	if s == nil {
		return ""
	}
	return s.blockMessageKey
}

func (s *Service) TargetEnabled(target Target) bool {
	if s == nil || !s.enabled {
		return false
	}
	switch target {
	case TargetChat:
		return s.targets.Chat
	case TargetUsername:
		return s.targets.Username
	case TargetKittyName:
		return s.targets.KittyName
	case TargetBattleRoomName:
		return s.targets.BattleRoomName
	case TargetLoadoutName:
		return s.targets.LoadoutName
	case TargetItemCustomName:
		return s.targets.ItemCustomName
	default:
		return false
	}
}

func (s *Service) Match(target Target, text string) bool {
	if !s.TargetEnabled(target) {
		return false
	}
	runes := []rune(normalize(text))
	for start := 0; start < len(runes); start++ {
		cur := s.root
		for idx := start; idx < len(runes); idx++ {
			next, ok := cur.children[runes[idx]]
			if !ok {
				break
			}
			cur = next
			if cur.terminal {
				return true
			}
		}
	}
	return false
}

func loadTermsFromJSONFile(path string) ([]string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %q: %w", path, err)
	}
	return loadTermsFromJSON(raw)
}

func loadTermsFromDir(vocabularyDir string, patterns []string) ([]string, error) {
	if len(patterns) == 0 {
		return nil, fmt.Errorf("include patterns are required for directory vocabulary")
	}
	fileSet := map[string]struct{}{}
	for _, pattern := range patterns {
		matches, err := filepath.Glob(filepath.Join(vocabularyDir, pattern))
		if err != nil {
			return nil, fmt.Errorf("glob %q: %w", pattern, err)
		}
		for _, match := range matches {
			info, err := os.Stat(match)
			if err != nil {
				return nil, fmt.Errorf("stat %q: %w", match, err)
			}
			if info.IsDir() {
				continue
			}
			fileSet[match] = struct{}{}
		}
	}
	files := make([]string, 0, len(fileSet))
	for file := range fileSet {
		files = append(files, file)
	}
	slices.Sort(files)

	termSet := map[string]struct{}{}
	for _, file := range files {
		raw, err := os.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("read %q: %w", file, err)
		}
		for _, term := range loadTermsFromText(raw) {
			termSet[term] = struct{}{}
		}
	}
	terms := make([]string, 0, len(termSet))
	for term := range termSet {
		terms = append(terms, term)
	}
	slices.Sort(terms)
	return terms, nil
}

func loadTermsFromText(raw []byte) []string {
	termSet := map[string]struct{}{}
	for _, line := range strings.Split(string(raw), "\n") {
		term := normalizeTermLine(line)
		if term == "" {
			continue
		}
		termSet[term] = struct{}{}
	}
	terms := make([]string, 0, len(termSet))
	for term := range termSet {
		terms = append(terms, term)
	}
	slices.Sort(terms)
	return terms
}

func loadTermsFromJSON(raw []byte) ([]string, error) {
	var asArray []string
	if err := json.Unmarshal(raw, &asArray); err == nil {
		return dedupeTerms(asArray), nil
	}
	var payload struct {
		Terms []string `json:"terms"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("parse json vocabulary: %w", err)
	}
	return dedupeTerms(payload.Terms), nil
}

func dedupeTerms(input []string) []string {
	termSet := map[string]struct{}{}
	for _, term := range input {
		normalized := normalizeTermLine(term)
		if normalized == "" {
			continue
		}
		termSet[normalized] = struct{}{}
	}
	terms := make([]string, 0, len(termSet))
	for term := range termSet {
		terms = append(terms, term)
	}
	slices.Sort(terms)
	return terms
}

func ExportTerms(vocabularyDir string, patterns []string) ([]string, error) {
	return loadTermsFromDir(strings.TrimSpace(vocabularyDir), patterns)
}

func normalizeTermLine(line string) string {
	trimmed := strings.TrimSpace(line)
	trimmed = strings.TrimRight(trimmed, ",，")
	trimmed = strings.TrimSpace(trimmed)
	if trimmed == "" {
		return ""
	}
	return trimmed
}

func normalize(text string) string {
	trimmed := strings.TrimSpace(strings.ToLower(text))
	if trimmed == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(trimmed))
	for _, r := range trimmed {
		switch {
		case unicode.IsSpace(r):
			continue
		case unicode.IsPunct(r):
			continue
		case unicode.IsSymbol(r):
			continue
		case unicode.In(r, unicode.Cc, unicode.Cf, unicode.Cs):
			continue
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func insert(root *node, word []rune) {
	if len(word) == 0 {
		return
	}
	cur := root
	for _, r := range word {
		if cur.children == nil {
			cur.children = map[rune]*node{}
		}
		child, ok := cur.children[r]
		if !ok {
			child = &node{children: map[rune]*node{}}
			cur.children[r] = child
		}
		cur = child
	}
	cur.terminal = true
}
