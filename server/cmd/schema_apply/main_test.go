package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunRejectsInvalidInputBeforeConnecting(t *testing.T) {
	if err := run("", []string{"migration.sql"}); err == nil || !strings.Contains(err.Error(), "DATABASE_URL") {
		t.Fatalf("missing database URL error = %v", err)
	}
	if err := run("postgres://unused", nil); err == nil || !strings.Contains(err.Error(), "SQL file") {
		t.Fatalf("missing SQL files error = %v", err)
	}

	emptyPath := filepath.Join(t.TempDir(), "empty.sql")
	if err := os.WriteFile(emptyPath, []byte(" \n\t"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := run("postgres://unused", []string{emptyPath}); err == nil || !strings.Contains(err.Error(), "is empty") {
		t.Fatalf("empty SQL file error = %v", err)
	}
	if err := run("postgres://unused", []string{filepath.Join(t.TempDir(), "missing.sql")}); err == nil || !strings.Contains(err.Error(), "read missing.sql") {
		t.Fatalf("missing SQL file error = %v", err)
	}
}
