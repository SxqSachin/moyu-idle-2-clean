package store

import (
	"context"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

type pondCandidateQuery struct {
	t            *testing.T
	total        int64
	invalidAbove int64
}

func (q pondCandidateQuery) Query(_ context.Context, sql string, args ...any) (pgx.Rows, error) {
	q.t.Helper()
	for _, guard := range []string{"user_id=$1", "instance_id<$2", "state='inventory'", "archived_at IS NULL", "quantity=1", "ORDER BY instance_id DESC LIMIT $3"} {
		if !strings.Contains(sql, guard) {
			q.t.Fatalf("missing query guard %s", guard)
		}
	}
	if args[0] != int64(7) || args[2] != 201 {
		q.t.Fatalf("unexpected arguments %v", args)
	}
	start := min(q.total, args[1].(int64)-1)
	return &pondCandidateRows{next: start, remaining: 201, invalidAbove: q.invalidAbove}, nil
}

type pondCandidateRows struct {
	pgx.Rows
	next, current, invalidAbove int64
	remaining                   int
}

func (r *pondCandidateRows) Next() bool {
	if r.next <= 0 || r.remaining <= 0 {
		return false
	}
	r.current = r.next
	r.next--
	r.remaining--
	return true
}
func (r *pondCandidateRows) Close()     {}
func (r *pondCandidateRows) Err() error { return nil }
func (r *pondCandidateRows) Scan(dest ...any) error {
	*dest[0].(*int64) = r.current
	*dest[1].(*[]byte) = []byte(`{}`)
	raw := `{"speciesId":"test","lifeState":"alive"}`
	if r.invalidAbove > 0 && r.current > r.invalidAbove {
		raw = `{"speciesId":"test","lifeState":"dead"}`
	}
	*dest[2].(*[]byte) = []byte(raw)
	return nil
}

func TestFishingPondCandidatesPaginateBeyondLimit(t *testing.T) {
	s := pondFeedTestStore()
	for _, total := range []int64{0, 200, 201, 650} {
		cursor, count := int64(0), 0
		seen := map[int64]bool{}
		for {
			page, err := s.listAvailablePondFishPage(context.Background(), pondCandidateQuery{t: t, total: total}, 7, cursor)
			if err != nil {
				t.Fatal(err)
			}
			if len(page.Items) > 200 {
				t.Fatal("unbounded page")
			}
			for _, row := range page.Items {
				if seen[row.InstanceID] {
					t.Fatal("duplicate candidate")
				}
				seen[row.InstanceID] = true
				count++
			}
			if page.NextCursor == 0 {
				break
			}
			if cursor > 0 && page.NextCursor >= cursor {
				t.Fatal("cursor did not advance")
			}
			cursor = page.NextCursor
		}
		if int64(count) != total {
			t.Fatalf("got %d candidates, want %d", count, total)
		}
	}
}

func TestFishingPondCandidatesEmptyPageStillAdvances(t *testing.T) {
	s := pondFeedTestStore()
	q := pondCandidateQuery{t: t, total: 401, invalidAbove: 1}
	page, err := s.listAvailablePondFishPage(context.Background(), q, 7, 0)
	if err != nil || len(page.Items) != 0 || page.NextCursor != 202 {
		t.Fatalf("first page %+v, %v", page, err)
	}
	page, err = s.listAvailablePondFishPage(context.Background(), q, 7, page.NextCursor)
	if err != nil || len(page.Items) != 0 || page.NextCursor != 2 {
		t.Fatalf("second page %+v, %v", page, err)
	}
	page, err = s.listAvailablePondFishPage(context.Background(), q, 7, page.NextCursor)
	if err != nil || len(page.Items) != 1 || page.Items[0].InstanceID != 1 || page.NextCursor != 0 {
		t.Fatalf("last page %+v, %v", page, err)
	}
}
