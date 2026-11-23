package reqmetrics

import (
	"strings"
	"testing"
	"time"
)

func TestCollectorSeparatesAcquireQueryAndAuthCosts(t *testing.T) {
	collector := New(time.Now())
	collector.AddPGAcquire(40 * time.Millisecond)
	collector.AddPG(5 * time.Millisecond)
	collector.AddAuthParse(2 * time.Millisecond)
	collector.AddAuthDB(7 * time.Millisecond)
	collector.AddAuth(10 * time.Millisecond)
	collector.SetPoolStats(3, 1, 4)

	cost := collector.Snapshot(time.Now().Add(50 * time.Millisecond))
	if cost.PGAcquire != 40 {
		t.Fatalf("PGAcquire=%v, want 40", cost.PGAcquire)
	}
	if cost.PG != 5 {
		t.Fatalf("PG=%v, want 5", cost.PG)
	}
	if cost.AuthParse != 2 || cost.AuthDB != 7 || cost.Auth != 10 {
		t.Fatalf("auth costs = parse:%v db:%v total:%v, want 2/7/10", cost.AuthParse, cost.AuthDB, cost.Auth)
	}
	if cost.PoolAcquired != 3 || cost.PoolIdle != 1 || cost.PoolMax != 4 {
		t.Fatalf("pool stats = %d/%d/%d, want 3/1/4", cost.PoolAcquired, cost.PoolIdle, cost.PoolMax)
	}
}

func TestCollectorKeepsMostUtilizedPoolStats(t *testing.T) {
	collector := New(time.Now())

	collector.SetPoolStats(3, 1, 4)
	collector.SetPoolStats(10, 14, 24)
	collector.SetPoolStats(4, 0, 4)
	collector.SetPoolStats(2, 2, 4)

	cost := collector.Snapshot(time.Now())
	if cost.PoolAcquired != 4 || cost.PoolIdle != 0 || cost.PoolMax != 4 {
		t.Fatalf("pool stats = %d/%d/%d, want 4/0/4", cost.PoolAcquired, cost.PoolIdle, cost.PoolMax)
	}
}

func TestLoadPGSlowQueryLogThreshold(t *testing.T) {
	t.Run("default", func(t *testing.T) {
		t.Setenv("REQMETRICS_PG_SLOW_LOG_MS", "")

		if got := loadPGSlowQueryLogThreshold(); got != defaultPGSlowQueryLogThreshold {
			t.Fatalf("loadPGSlowQueryLogThreshold() = %s, want %s", got, defaultPGSlowQueryLogThreshold)
		}
	})

	t.Run("override", func(t *testing.T) {
		t.Setenv("REQMETRICS_PG_SLOW_LOG_MS", "75.5")

		if got := loadPGSlowQueryLogThreshold(); got != 75500*time.Microsecond {
			t.Fatalf("loadPGSlowQueryLogThreshold() = %s, want 75.5ms", got)
		}
	})

	t.Run("disabled", func(t *testing.T) {
		t.Setenv("REQMETRICS_PG_SLOW_LOG_MS", "0")

		if got := loadPGSlowQueryLogThreshold(); got != 0 {
			t.Fatalf("loadPGSlowQueryLogThreshold() = %s, want 0", got)
		}
	})
}

func TestClassifyPGQuery(t *testing.T) {
	cases := []struct {
		name string
		sql  string
		want string
	}{
		{
			name: "advisory lock",
			sql:  `SELECT pg_advisory_xact_lock($1)`,
			want: "advisory_lock",
		},
		{
			name: "cat village",
			sql:  `SELECT id FROM player_cat_village_tasks WHERE user_id = $1 AND status = 'open' ORDER BY id DESC`,
			want: "cat_village",
		},
		{
			name: "inventory lock",
			sql:  `SELECT instance_id FROM player_item_instances WHERE user_id = $1 ORDER BY instance_id FOR UPDATE`,
			want: "inventory_lock",
		},
		{
			name: "warehouse audit",
			sql:  `INSERT INTO player_warehouse_change_logs(user_id, audit_key) VALUES($1, $2)`,
			want: "warehouse_audit",
		},
		{
			name: "achievement events",
			sql:  `INSERT INTO achievement_events(event_id, user_id, kind) VALUES($1, $2, $3)`,
			want: "achievement_events",
		},
		{
			name: "presence",
			sql:  `SELECT user_id, session_version FROM player_presence WHERE status = 'online' AND user_id > $2 ORDER BY user_id LIMIT $1`,
			want: "presence",
		},
		{
			name: "battle room claim after fingerprint truncation",
			sql: `WITH candidates AS (
				SELECT r.id FROM battle_rooms r
				WHERE r.status = 'playing' AND r.schema_version = 1
				AND r.current_run_id IS NULL AND r.next_tick_at <= $1
				AND (r.lease_until IS NULL OR r.lease_until < $1 OR r.owner_worker_id = $2)
				ORDER BY r.next_tick_at ASC, r.updated_at ASC, r.id ASC
				LIMIT $3 FOR UPDATE OF r SKIP LOCKED
			) UPDATE battle_rooms AS r SET owner_worker_id = $2, lease_until = $4
			FROM candidates c WHERE r.id = c.id RETURNING r.id`,
			want: "battle_room_claim",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := classifyPGQuery(tc.sql); got != tc.want {
				t.Fatalf("classifyPGQuery() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestFingerprintPGQueryCollapsesWhitespaceAndTruncates(t *testing.T) {
	got := fingerprintPGQuery("\n\tSELECT   id,\n commission_id   FROM player_cat_village_tasks\tWHERE user_id = $1\n")
	want := "SELECT id, commission_id FROM player_cat_village_tasks WHERE user_id = $1"
	if got != want {
		t.Fatalf("fingerprintPGQuery() = %q, want %q", got, want)
	}

	longQuery := strings.Repeat("x", 300)
	got = fingerprintPGQuery(longQuery)
	if len(got) != 243 || got[240:] != "..." {
		t.Fatalf("truncated fingerprint length/suffix = %d/%q, want 243/... ", len(got), got[len(got)-3:])
	}
}
