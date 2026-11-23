package main

import (
	"encoding/json"
	"flag"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"mysecondwebgame/server/internal/moderation"
)

type compiledVocabulary struct {
	Version     int      `json:"version"`
	GeneratedAt string   `json:"generatedAt"`
	SourcePath  string   `json:"sourcePath"`
	Patterns    []string `json:"patterns,omitempty"`
	TermCount   int      `json:"termCount"`
	Terms       []string `json:"terms"`
}

func main() {
	input := flag.String("input", "../packages/Sensitive-lexicon-1.2/Vocabulary", "source vocabulary directory or file")
	output := flag.String("output", "", "output json file path")
	patternsRaw := flag.String("patterns", "*.txt", "comma-separated glob patterns when input is a directory")
	flag.Parse()

	if strings.TrimSpace(*output) == "" {
		log.Fatal("missing -output")
	}
	patterns := splitCSV(*patternsRaw)
	if len(patterns) == 0 {
		log.Fatal("missing -patterns")
	}
	terms, err := moderation.ExportTerms(strings.TrimSpace(*input), patterns)
	if err != nil {
		log.Fatalf("load vocabulary: %v", err)
	}

	payload := compiledVocabulary{
		Version:     1,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		SourcePath:  strings.TrimSpace(*input),
		Patterns:    patterns,
		TermCount:   len(terms),
		Terms:       terms,
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		log.Fatalf("marshal vocabulary: %v", err)
	}
	outputPath := filepath.Clean(strings.TrimSpace(*output))
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		log.Fatalf("mkdir output dir: %v", err)
	}
	if err := os.WriteFile(outputPath, append(data, '\n'), 0o644); err != nil {
		log.Fatalf("write output: %v", err)
	}
	log.Printf("compiled %d terms to %s", len(terms), outputPath)
}

func splitCSV(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value == "" {
			continue
		}
		out = append(out, value)
	}
	return out
}
