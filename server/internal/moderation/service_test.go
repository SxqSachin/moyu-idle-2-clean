package moderation

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewLoadsOnlyTXTTerms(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("苹果\nxjp,\n"), 0o600); err != nil {
		t.Fatalf("write txt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.txt-pass"), []byte("不应命中"), 0o600); err != nil {
		t.Fatalf("write txt-pass: %v", err)
	}

	if _, err := New(Config{
		Enabled:            true,
		Targets:            TargetConfig{Chat: true},
		VocabularyJSONPath: filepath.Join(dir, "compiled.json"),
	}); err == nil {
		t.Fatalf("expected json-only service to reject directory path")
	}
}

func TestMatchRespectsTargetAndSubstring(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "vocabulary.json")
	if err := os.WriteFile(path, []byte(`{"terms":["敏感词"]}`), 0o600); err != nil {
		t.Fatalf("write json: %v", err)
	}
	svc, err := New(Config{
		Enabled:            true,
		VocabularyJSONPath: path,
		Targets: TargetConfig{
			Chat:     true,
			Username: false,
		},
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	if !svc.Match(TargetChat, "这是敏感词内容") {
		t.Fatalf("expected substring match")
	}
	if svc.Match(TargetUsername, "敏感词用户") {
		t.Fatalf("did not expect disabled target to match")
	}
	if svc.Match(TargetChat, "正常内容") {
		t.Fatalf("did not expect normal content to match")
	}
}

func TestMatchStripsSeparatorsBeforeCompare(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "vocabulary.json")
	if err := os.WriteFile(path, []byte(`{"terms":["xjp","敏感词","covid-19"]}`), 0o600); err != nil {
		t.Fatalf("write json: %v", err)
	}
	svc, err := New(Config{
		Enabled:            true,
		VocabularyJSONPath: path,
		Targets:            TargetConfig{Chat: true},
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	cases := []string{
		"x j p",
		"x_j_p",
		"敏 感 词",
		"敏-感-词",
		"co vid_19",
		"co\u200bvid 19",
	}
	for _, text := range cases {
		if !svc.Match(TargetChat, text) {
			t.Fatalf("expected separator-stripped match for %q", text)
		}
	}
}

func TestNewLoadsCompiledJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vocabulary.json")
	raw := []byte(`{"terms":["苹果","xjp,","苹果"]}`)
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatalf("write json: %v", err)
	}
	svc, err := New(Config{
		Enabled:            true,
		VocabularyJSONPath: path,
		Targets:            TargetConfig{Chat: true},
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	if !svc.Match(TargetChat, "这是苹果") {
		t.Fatalf("expected json term match")
	}
	if !svc.Match(TargetChat, "hello xjp world") {
		t.Fatalf("expected trimmed json term match")
	}
}
