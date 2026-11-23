package store

import (
	"context"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type queuePerformanceExecTx struct {
	execCount int
	sql       string
}

func (tx *queuePerformanceExecTx) Begin(context.Context) (pgx.Tx, error) { return nil, nil }
func (tx *queuePerformanceExecTx) Commit(context.Context) error          { return nil }
func (tx *queuePerformanceExecTx) Rollback(context.Context) error        { return nil }
func (tx *queuePerformanceExecTx) CopyFrom(context.Context, pgx.Identifier, []string, pgx.CopyFromSource) (int64, error) {
	return 0, nil
}
func (tx *queuePerformanceExecTx) SendBatch(context.Context, *pgx.Batch) pgx.BatchResults {
	return nil
}
func (tx *queuePerformanceExecTx) LargeObjects() pgx.LargeObjects { return pgx.LargeObjects{} }
func (tx *queuePerformanceExecTx) Prepare(context.Context, string, string) (*pgconn.StatementDescription, error) {
	return nil, nil
}
func (tx *queuePerformanceExecTx) Exec(_ context.Context, sql string, _ ...any) (pgconn.CommandTag, error) {
	tx.execCount++
	tx.sql = sql
	return pgconn.NewCommandTag("UPDATE 1"), nil
}
func (tx *queuePerformanceExecTx) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, nil
}
func (tx *queuePerformanceExecTx) QueryRow(context.Context, string, ...any) pgx.Row { return nil }
func (tx *queuePerformanceExecTx) Conn() *pgx.Conn                                  { return nil }

func TestTransactionUserLockIsAcquiredOncePerUser(t *testing.T) {
	store := &Store{auditBuffers: make(map[pgx.Tx]*inventoryAuditBuffer)}
	tx := &queuePerformanceExecTx{}
	store.registerInventoryAuditBuffer(tx)
	t.Cleanup(func() { store.discardInventoryAuditBuffer(tx) })

	if err := store.lockUserTx(context.Background(), tx, 7); err != nil {
		t.Fatalf("first lock: %v", err)
	}
	if err := store.lockUserTx(context.Background(), tx, 7); err != nil {
		t.Fatalf("second lock: %v", err)
	}
	if tx.execCount != 1 {
		t.Fatalf("advisory lock executions=%d, want 1", tx.execCount)
	}
	if err := store.lockUserTx(context.Background(), tx, 8); err != nil {
		t.Fatalf("different user lock: %v", err)
	}
	if tx.execCount != 2 {
		t.Fatalf("advisory lock executions=%d after second user, want 2", tx.execCount)
	}
}

func TestUserBanCheckDoesNotConflictWithForeignKeyKeyShareLocks(t *testing.T) {
	normalized := strings.ToUpper(strings.Join(strings.Fields(assertUserNotBannedTxSQL), " "))
	if !strings.Contains(normalized, "FOR NO KEY UPDATE") {
		t.Fatalf("ban check must preserve update serialization with a no-key row lock: %s", normalized)
	}
	if strings.Contains(normalized, "FOR UPDATE") {
		t.Fatalf("ban check uses FOR UPDATE and will conflict with foreign-key key-share locks: %s", normalized)
	}
}

func TestNormalizeQueueOrderUpdatesOnlyChangedRows(t *testing.T) {
	tx := &queuePerformanceExecTx{}
	if err := (&Store{}).normalizeQueueOrderTx(context.Background(), tx, 7); err != nil {
		t.Fatalf("normalize queue order: %v", err)
	}
	if !strings.Contains(tx.sql, "q.queue_order IS DISTINCT FROM ordered.next_order") {
		t.Fatalf("normalization SQL still rewrites unchanged rows:\n%s", tx.sql)
	}
}

func TestReconcileKittyLaneStatusesUsesOneConditionalUpdate(t *testing.T) {
	tx := &queuePerformanceExecTx{}
	if err := (&Store{}).reconcileKittyLaneStatusesTx(context.Background(), tx, 7); err != nil {
		t.Fatalf("reconcile kitty lane status: %v", err)
	}
	if tx.execCount != 1 {
		t.Fatalf("reconcile executions=%d, want 1", tx.execCount)
	}
	for _, required := range []string{"WITH desired AS MATERIALIZED", "SELECT DISTINCT actor_id", "LEFT JOIN", "IS DISTINCT FROM desired.status"} {
		if !strings.Contains(tx.sql, required) {
			t.Fatalf("reconcile SQL missing %q:\n%s", required, tx.sql)
		}
	}
}

func TestWorkerScheduleProjectionDoesNotReadPayload(t *testing.T) {
	if strings.Contains(strings.ToLower(actionQueueScheduleSelectSQL), "payload") {
		t.Fatalf("worker schedule projection unexpectedly reads payload:\n%s", actionQueueScheduleSelectSQL)
	}
	for _, required := range []string{"actor_type", "actor_id", "status", "current_run_ends_at", "LIMIT $2"} {
		if !strings.Contains(actionQueueScheduleSelectSQL, required) {
			t.Fatalf("worker schedule projection missing %q", required)
		}
	}
}
