package store

import (
	"os"
	"strings"
	"testing"
)

func normalizeInventorySnapshotSQL(raw string) string {
	return strings.Join(strings.Fields(strings.ToLower(raw)), " ")
}

func TestListItemInstancesSnapshotExcludesOnlyTerminalHistory(t *testing.T) {
	query := normalizeInventorySnapshotSQL(listItemInstancesSnapshotSQL)
	if !strings.Contains(query, "where i.user_id = $1 and i.state not in ('consumed', 'destroyed')") {
		t.Fatalf("inventory snapshot must exclude consumed and destroyed history: %s", query)
	}
	if strings.Contains(query, "archived_at is null") {
		t.Fatal("inventory snapshot must retain archived inventory items")
	}
	if strings.Contains(query, "i.state = 'inventory'") {
		t.Fatal("inventory snapshot must retain equipped and other non-terminal domain states")
	}
	if !strings.Contains(query, "order by i.instance_id desc") {
		t.Fatal("inventory snapshot ordering changed")
	}
}

func TestActiveInventorySnapshotIndexMatchesQueryPredicate(t *testing.T) {
	const indexName = "idx_player_item_instances_user_active_instance_desc"
	for name, path := range map[string]string{
		"fresh schema": "../../schema/init.sql",
		"migration":    "../../schema/migrations/20260902_player_item_instance_active_snapshot_index.sql",
	} {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		sql := normalizeInventorySnapshotSQL(string(raw))
		if !strings.Contains(sql, indexName) || !strings.Contains(sql, "on player_item_instances(user_id, instance_id desc) where state not in ('consumed', 'destroyed')") {
			t.Fatalf("%s does not define the matching partial index", name)
		}
	}

	migrationRaw, err := os.ReadFile("../../schema/migrations/20260902_player_item_instance_active_snapshot_index.sql")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(normalizeInventorySnapshotSQL(string(migrationRaw)), "create index concurrently if not exists") {
		t.Fatal("production index migration must avoid blocking writes")
	}

	downRaw, err := os.ReadFile("../../schema/migrations/20260902_player_item_instance_active_snapshot_index_down.sql")
	if err != nil {
		t.Fatal(err)
	}
	downSQL := normalizeInventorySnapshotSQL(string(downRaw))
	if !strings.Contains(downSQL, "drop index concurrently if exists "+indexName) {
		t.Fatal("down migration must drop the matching index without blocking writes")
	}
}
