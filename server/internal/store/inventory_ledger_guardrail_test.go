package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWarehouseLogWritesAreCentralized(t *testing.T) {
	root := "."
	allowed := map[string]struct{}{
		"inventory_ledger.go": {},
		"warehouse_logs.go":   {},
	}

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		base := filepath.Base(path)
		if _, ok := allowed[base]; ok {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if strings.Contains(string(raw), "insertWarehouseChangeLogTx(") {
			t.Fatalf("direct warehouse log write found in %s; route through inventory_ledger.go", path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("scan store files failed: %v", err)
	}
}
