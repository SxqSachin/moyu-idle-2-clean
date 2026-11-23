package store

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type adminBattleReinitializeRow struct {
	rooms int64
	runs  int64
	err   error
}

func (row adminBattleReinitializeRow) Scan(destinations ...any) error {
	if row.err != nil {
		return row.err
	}
	if len(destinations) != 2 {
		return errors.New("unexpected admin battle reinitialize scan shape")
	}
	rooms, roomsOK := destinations[0].(*int64)
	runs, runsOK := destinations[1].(*int64)
	if !roomsOK || !runsOK {
		return errors.New("unexpected admin battle reinitialize scan destinations")
	}
	*rooms = row.rooms
	*runs = row.runs
	return nil
}

type adminBattleReinitializeTx struct {
	query string
	args  []any
	row   pgx.Row
}

func (tx *adminBattleReinitializeTx) Begin(context.Context) (pgx.Tx, error) { return nil, nil }
func (tx *adminBattleReinitializeTx) Commit(context.Context) error          { return nil }
func (tx *adminBattleReinitializeTx) Rollback(context.Context) error        { return nil }
func (tx *adminBattleReinitializeTx) CopyFrom(context.Context, pgx.Identifier, []string, pgx.CopyFromSource) (int64, error) {
	return 0, nil
}
func (tx *adminBattleReinitializeTx) SendBatch(context.Context, *pgx.Batch) pgx.BatchResults {
	return nil
}
func (tx *adminBattleReinitializeTx) LargeObjects() pgx.LargeObjects { return pgx.LargeObjects{} }
func (tx *adminBattleReinitializeTx) Prepare(context.Context, string, string) (*pgconn.StatementDescription, error) {
	return nil, nil
}
func (tx *adminBattleReinitializeTx) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}
func (tx *adminBattleReinitializeTx) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, nil
}
func (tx *adminBattleReinitializeTx) QueryRow(_ context.Context, query string, args ...any) pgx.Row {
	tx.query = query
	tx.args = append([]any(nil), args...)
	return tx.row
}
func (tx *adminBattleReinitializeTx) Conn() *pgx.Conn { return nil }

func TestAdminReinitializeBattleRoomsInterruptsOnlyActiveRuns(t *testing.T) {
	now := time.UnixMilli(1_700_000_000_000)
	recoveryEndsAt := now.Add(30 * time.Second)
	statuses := []string{BattleRoomStatusEntering, BattleRoomStatusPlaying, BattleRoomStatusRecovering}
	tx := &adminBattleReinitializeTx{row: adminBattleReinitializeRow{rooms: 7, runs: 5}}

	rooms, runs, err := adminReinitializeBattleRoomsTx(context.Background(), tx, statuses, now, recoveryEndsAt, true)
	if err != nil {
		t.Fatal(err)
	}
	if rooms != 7 || runs != 5 {
		t.Fatalf("counts=(%d,%d), want (7,5)", rooms, runs)
	}
	if !strings.Contains(tx.query, "br.status IN ($4, $5, $6)") || strings.Contains(tx.query, "br.status NOT IN") {
		t.Fatalf("run update must target only active run states:\n%s", tx.query)
	}
	wantArgs := []any{
		statuses,
		BattleRunStatusInterrupted,
		now,
		BattleRunStatusCompiled,
		BattleRunStatusRunning,
		BattleRunStatusSettling,
		BattleRoomStatusRecovering,
		recoveryEndsAt,
		true,
	}
	if !reflect.DeepEqual(tx.args, wantArgs) {
		t.Fatalf("query args=%#v, want %#v", tx.args, wantArgs)
	}
	for _, status := range tx.args[3:6] {
		if status == BattleRunStatusFailed {
			t.Fatal("admin reinitialize must not rewrite an already failed run as interrupted")
		}
	}
}
