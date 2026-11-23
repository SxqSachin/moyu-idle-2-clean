package store

import (
	"context"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type captureExecTx struct {
	sql  string
	args []any
}

func (tx *captureExecTx) Begin(context.Context) (pgx.Tx, error) { return nil, nil }
func (tx *captureExecTx) Commit(context.Context) error          { return nil }
func (tx *captureExecTx) Rollback(context.Context) error        { return nil }
func (tx *captureExecTx) CopyFrom(context.Context, pgx.Identifier, []string, pgx.CopyFromSource) (int64, error) {
	return 0, nil
}
func (tx *captureExecTx) SendBatch(context.Context, *pgx.Batch) pgx.BatchResults { return nil }
func (tx *captureExecTx) LargeObjects() pgx.LargeObjects                         { return pgx.LargeObjects{} }
func (tx *captureExecTx) Prepare(context.Context, string, string) (*pgconn.StatementDescription, error) {
	return nil, nil
}
func (tx *captureExecTx) Exec(_ context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	tx.sql = sql
	tx.args = append([]any(nil), arguments...)
	return pgconn.NewCommandTag("UPDATE 1"), nil
}
func (tx *captureExecTx) Query(context.Context, string, ...any) (pgx.Rows, error) { return nil, nil }
func (tx *captureExecTx) QueryRow(context.Context, string, ...any) pgx.Row        { return nil }
func (tx *captureExecTx) Conn() *pgx.Conn                                         { return nil }

func TestTransferOwnershipAndStateClearsArchivedAtOnlyWhenOwnerChanges(t *testing.T) {
	tx := &captureExecTx{}
	err := (&Store{}).TransferOwnershipAndStateTx(context.Background(), tx, 99, 10, 20, "auction", "inventory")
	if err != nil {
		t.Fatalf("TransferOwnershipAndStateTx returned err: %v", err)
	}

	required := []string{
		"archived_at = CASE WHEN $1 <> $4 THEN NULL ELSE archived_at END",
		"WHERE instance_id = $3 AND user_id = $4 AND state = $5",
	}
	for _, needle := range required {
		if !strings.Contains(tx.sql, needle) {
			t.Fatalf("transfer SQL missing %q:\n%s", needle, tx.sql)
		}
	}

	wantArgs := []any{int64(20), "inventory", int64(99), int64(10), "auction"}
	if len(tx.args) != len(wantArgs) {
		t.Fatalf("args len=%d, want %d (%v)", len(tx.args), len(wantArgs), tx.args)
	}
	for idx := range wantArgs {
		if tx.args[idx] != wantArgs[idx] {
			t.Fatalf("arg[%d]=%v, want %v", idx, tx.args[idx], wantArgs[idx])
		}
	}
}

func TestSetStatePreservesArchivedAt(t *testing.T) {
	tx := &captureExecTx{}
	err := (&Store{}).SetStateTx(context.Background(), tx, 99, 10, "auction", "inventory")
	if err != nil {
		t.Fatalf("SetStateTx returned err: %v", err)
	}
	if strings.Contains(tx.sql, "archived_at") {
		t.Fatalf("state-only SQL should preserve archived_at:\n%s", tx.sql)
	}
	required := []string{
		"SET state = $1, updated_at = now()",
		"WHERE instance_id = $2 AND user_id = $3 AND state = $4",
	}
	for _, needle := range required {
		if !strings.Contains(tx.sql, needle) {
			t.Fatalf("state-only SQL missing %q:\n%s", needle, tx.sql)
		}
	}

	wantArgs := []any{"inventory", int64(99), int64(10), "auction"}
	if len(tx.args) != len(wantArgs) {
		t.Fatalf("args len=%d, want %d (%v)", len(tx.args), len(wantArgs), tx.args)
	}
	for idx := range wantArgs {
		if tx.args[idx] != wantArgs[idx] {
			t.Fatalf("arg[%d]=%v, want %v", idx, tx.args[idx], wantArgs[idx])
		}
	}
}
