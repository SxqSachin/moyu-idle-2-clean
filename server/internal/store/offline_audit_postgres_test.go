package store

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func offlineAuditProofEvents(count int) []InventoryAuditEvent {
	events := make([]InventoryAuditEvent, 0, count+1)
	for i := 0; i < count; i++ {
		events = append(events, normalizeInventoryAuditEvent(InventoryAuditEvent{
			UserID: 970071, Category: InventoryAuditCategoryAlchemy, Direction: InventoryAuditDirectionGain,
			SubjectType: "alchemy", SubjectID: "91", TemplateItem: GoldCoinItemID, Quantity: int64(i%7 + 1),
			SourceSurface: "alchemy_transmute", TargetSurface: "inventory",
			Details: map[string]any{"phase": "offline_fastforward", "runId": fmt.Sprint(i), "proof": "initial"},
		}))
	}
	if count > 0 {
		// Same-key UPSERT must remain ordered even within a pipeline.
		duplicate := events[0]
		duplicate.Items = copyInt64Map(duplicate.Items)
		duplicate.Details = map[string]any{"phase": "offline_fastforward", "runId": "0", "proof": "last"}
		events = append(events, duplicate)
	}
	return events
}

func TestOfflineAuditPostgresPipelineParity(t *testing.T) {
	s, ctx, _ := recoveryPostgresStore(t)
	var database string
	if err := s.db.QueryRow(ctx, `SELECT current_database()`).Scan(&database); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(database, "offline_recovery_test_") {
		t.Fatal("audit proof may reset only its newly created disposable database", database)
	}
	for _, count := range []int{0, 1, 128, 129, 2048} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			var want []byte
			for _, pipelined := range []bool{false, true} {
				if _, err := s.db.Exec(ctx, `TRUNCATE player_warehouse_change_logs RESTART IDENTITY`); err != nil {
					t.Fatal(err)
				}
				events := offlineAuditProofEvents(count)
				began := time.Now()
				if pipelined {
					if err := s.writeInventoryAuditEvents(ctx, events); err != nil {
						t.Fatal(err)
					}
				} else {
					for _, event := range events {
						if err := s.insertInventoryAuditEvent(ctx, event); err != nil {
							t.Fatal(err)
						}
					}
				}
				t.Logf("pipelined=%v events=%d elapsed=%s", pipelined, len(events), time.Since(began))
				var got []byte
				if err := s.db.QueryRow(ctx, `SELECT COALESCE(jsonb_agg(to_jsonb(q)-'created_at' ORDER BY id),'[]'::jsonb) FROM player_warehouse_change_logs q`).Scan(&got); err != nil {
					t.Fatal(err)
				}
				if !pipelined {
					want = got
				} else if string(want) != string(got) {
					t.Fatal("audit rows, identities, order or UPSERT changed")
				}
			}
		})
	}
}

func TestOfflineAuditPostgresPipelineFailure(t *testing.T) {
	s, ctx, _ := recoveryPostgresStore(t)
	events := offlineAuditProofEvents(129)
	events[128].UserID = 999999
	if err := s.writeInventoryAuditEvents(ctx, events); err == nil {
		t.Fatal("foreign key failure was swallowed")
	}
	var rows int
	if err := s.db.QueryRow(ctx, `SELECT count(*) FROM player_warehouse_change_logs`).Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if rows != 128 {
		t.Fatal("failed pipeline was not bounded/atomic", rows)
	}
	if err := s.writeInventoryAuditEvents(ctx, offlineAuditProofEvents(129)); err != nil {
		t.Fatal("connection was not released after failure", err)
	}
	if err := s.db.QueryRow(ctx, `SELECT count(*) FROM player_warehouse_change_logs`).Scan(&rows); err != nil || rows != 129 {
		t.Fatal("retry duplicated audit events", rows, err)
	}
}
