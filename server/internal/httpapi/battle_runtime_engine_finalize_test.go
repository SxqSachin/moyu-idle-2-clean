package httpapi

import (
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestBattleFinalizeRetryable(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "deadlock detected",
			err:  &pgconn.PgError{Code: "40P01"},
			want: true,
		},
		{
			name: "serialization failure",
			err:  &pgconn.PgError{Code: "40001"},
			want: true,
		},
		{
			name: "other postgres error",
			err:  &pgconn.PgError{Code: "23505"},
			want: false,
		},
		{
			name: "wrapped deadlock detected",
			err:  fmt.Errorf("wrapped: %w", &pgconn.PgError{Code: "40P01"}),
			want: true,
		},
		{
			name: "generic error",
			err:  errors.New("boom"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := battleFinalizeRetryable(tt.err); got != tt.want {
				t.Fatalf("battleFinalizeRetryable()=%v, want %v", got, tt.want)
			}
		})
	}
}
