package store

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestClearOfflineWorkLoadoutIntentIsPlayerAndIntentScoped(t *testing.T) {
	tx := &captureExecTx{}
	if err := (&Store{}).clearOfflineWorkLoadoutIntentTx(context.Background(), tx, 42, "loadout-herbalism"); err != nil {
		t.Fatal(err)
	}
	for _, clause := range []string{
		"payload = payload - 'deferredWorkLoadoutId'",
		"user_id = $1",
		"actor_type = 'player'",
		"actor_id = 0",
		"status = 'running'",
		"payload->>'deferredWorkLoadoutId' = $2",
	} {
		if !strings.Contains(tx.sql, clause) {
			t.Fatalf("cleanup must preserve other users, lanes, intents and runtime fields: missing %q", clause)
		}
	}
	if want := []any{int64(42), "loadout-herbalism"}; !reflect.DeepEqual(tx.args, want) {
		t.Fatalf("args=%v, want %v", tx.args, want)
	}
}

type offlineIntentFailureTx struct {
	captureExecTx
	err error
}

func (tx *offlineIntentFailureTx) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, tx.err
}

func TestClearOfflineWorkLoadoutIntentPropagatesFailure(t *testing.T) {
	want := errors.New("cleanup failed")
	tx := &offlineIntentFailureTx{err: want}
	if err := (&Store{}).clearOfflineWorkLoadoutIntentTx(context.Background(), tx, 42, "loadout-herbalism"); !errors.Is(err, want) {
		t.Fatalf("cleanup failure must reach the transaction owner: got %v", err)
	}
}
