package store

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"mysecondwebgame/server/internal/catalog"
	"mysecondwebgame/server/internal/catteryfurniture"
	"mysecondwebgame/server/internal/operationeffect"
)

type furnitureCacheTestTx struct {
	pgx.Tx
	queries, locks atomic.Int64
	gate, entered  chan struct{}
	fail           atomic.Bool
}

type furnitureCacheWaitContext struct {
	context.Context
	entered chan struct{}
	once    sync.Once
}

func (c *furnitureCacheWaitContext) Done() <-chan struct{} {
	c.once.Do(func() { close(c.entered) })
	return c.Context.Done()
}

type furnitureCacheTestRow struct {
	id   int64
	from *time.Time
	raw  []byte
}
type furnitureCacheTestRows struct {
	pgx.Rows
	rows []furnitureCacheTestRow
	i    int
}

func (r *furnitureCacheTestRows) Next() bool { r.i++; return r.i < len(r.rows) }
func (r *furnitureCacheTestRows) Close()     {}
func (r *furnitureCacheTestRows) Err() error { return nil }
func (r *furnitureCacheTestRows) Scan(dest ...any) error {
	x := r.rows[r.i]
	*dest[0].(*int64) = x.id
	*dest[1].(**time.Time) = x.from
	*dest[2].(**time.Time) = nil
	*dest[3].(*[]byte) = x.raw
	return nil
}
func (tx *furnitureCacheTestTx) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	tx.locks.Add(1)
	return pgconn.NewCommandTag("SELECT 1"), nil
}
func (tx *furnitureCacheTestTx) Query(ctx context.Context, _ string, args ...any) (pgx.Rows, error) {
	n := tx.queries.Add(1)
	if n == 1 && tx.entered != nil {
		close(tx.entered)
	}
	if tx.gate != nil {
		select {
		case <-tx.gate:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if tx.fail.Load() {
		return nil, errors.New("database unavailable")
	}
	rows := &furnitureCacheTestRows{i: -1}
	for _, id := range args[0].([]int64) {
		row := furnitureCacheTestRow{id: id}
		if id == 1 {
			at := time.Unix(1, 0)
			row.from = &at
			row.raw, _ = json.Marshal(operationeffect.PassiveSource{ID: "cabinet", Version: "v1", Rules: []operationeffect.PassiveRule{{Kind: operationeffect.KindExtraLoot, Domain: operationeffect.DomainBattleReward, RateBPS: 500}}})
		}
		rows.rows = append(rows.rows, row)
	}
	return rows, nil
}

func TestFurnitureCacheWarmReadsBatchMissesAndNegativeEntries(t *testing.T) {
	s := &Store{}
	tx := &furnitureCacheTestTx{}
	at := time.Now()
	for i := 0; i < 100; i++ {
		rows, err := s.furnitureWindowsTx(context.Background(), tx, []int64{2, 1, 1}, at, at)
		if err != nil || len(rows[1]) != 1 || len(rows[2]) != 0 {
			t.Fatalf("rows=%v err=%v", rows, err)
		}
	}
	if tx.queries.Load() != 1 || tx.locks.Load() != 1 {
		t.Fatalf("warm cache queried database: queries=%d locks=%d", tx.queries.Load(), tx.locks.Load())
	}
	s.InvalidateFurnitureEffects(1)
	if _, err := s.furnitureWindowsTx(context.Background(), tx, []int64{1, 2}, at, at); err != nil {
		t.Fatal(err)
	}
	if tx.queries.Load() != 2 {
		t.Fatal("invalidation did not reload")
	}
}

func TestFurnitureCacheBattleStartsReuseRosterAndSources(t *testing.T) {
	s := &Store{gameData: catalog.Data{CatteryFurniture: []catteryfurniture.Definition{{ID: "cabinet"}}}}
	tx := &furnitureCacheTestTx{}
	for i := 0; i < 100; i++ {
		raw, _, err := s.freezeBattlePassiveTx(context.Background(), tx, 4, 1, 2)
		if err != nil || raw == "{}" {
			t.Fatalf("battle snapshot: %s %v", raw, err)
		}
	}
	if tx.queries.Load() != 1 || tx.locks.Load() != 1 {
		t.Fatal("warm battle starts reloaded roster or furniture")
	}
}

func TestFurnitureCacheLeaseHistoryAndErrors(t *testing.T) {
	s := &Store{}
	tx := &furnitureCacheTestTx{}
	at := time.Now()
	load := func(from time.Time) error {
		_, err := s.furnitureWindowsTx(context.Background(), tx, []int64{1}, from, at)
		return err
	}
	if err := load(at); err != nil {
		t.Fatal(err)
	}
	if err := load(at.Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	if tx.queries.Load() != 2 {
		t.Fatal("missing history treated as complete")
	}
	s.furnitureEffects.mu.Lock()
	s.furnitureEffects.entries[1].Value.(*furnitureEffectEntry).expires = time.Now().Add(-time.Second)
	s.furnitureEffects.mu.Unlock()
	tx.fail.Store(true)
	if err := load(at); err == nil {
		t.Fatal("expired source silently used after DB failure")
	}
	tx.fail.Store(false)
	if err := load(at); err != nil {
		t.Fatal(err)
	}
	if tx.queries.Load() != 4 {
		t.Fatal("failed load was cached")
	}
}

func TestFurnitureCacheConcurrentMissesAndCancellation(t *testing.T) {
	s := &Store{}
	tx := &furnitureCacheTestTx{gate: make(chan struct{}), entered: make(chan struct{})}
	at := time.Now()
	var wg sync.WaitGroup
	for i := 0; i < 24; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := s.furnitureWindowsTx(context.Background(), tx, []int64{1, 2}, at, at); err != nil {
				t.Error(err)
			}
		}()
	}
	<-tx.entered
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.furnitureWindowsTx(ctx, tx, []int64{1}, at, at); !errors.Is(err, context.Canceled) {
		t.Fatalf("wait cancellation: %v", err)
	}
	close(tx.gate)
	wg.Wait()
	if tx.queries.Load() != 1 {
		t.Fatalf("concurrent loads=%d", tx.queries.Load())
	}
}

func TestFurnitureCacheSharesFailedLoadsWithWaiters(t *testing.T) {
	s := &Store{}
	tx := &furnitureCacheTestTx{gate: make(chan struct{}), entered: make(chan struct{})}
	tx.fail.Store(true)
	at := time.Now()
	errorsOut := make(chan error, 2)
	load := func(ctx context.Context) {
		_, err := s.furnitureWindowsTx(ctx, tx, []int64{1}, at, at)
		errorsOut <- err
	}
	go load(context.Background())
	<-tx.entered
	waiting := &furnitureCacheWaitContext{Context: context.Background(), entered: make(chan struct{})}
	go load(waiting)
	<-waiting.entered
	close(tx.gate)
	for i := 0; i < 2; i++ {
		if err := <-errorsOut; err == nil {
			t.Fatal("load error lost")
		}
	}
	if tx.queries.Load() != 1 {
		t.Fatal("waiting request retried a shared database failure")
	}
}

func TestFurnitureCacheProductionPolicyAndEviction(t *testing.T) {
	s := &Store{}
	policy := s.furnitureRuntimePolicy()
	if policy.lease != 30*time.Second || policy.delay < 2*policy.lease {
		t.Fatal("unsafe activation/cache policy")
	}
	tx := &furnitureCacheTestTx{}
	at := time.Now()
	for id := int64(1); id <= furnitureCacheMaxEntries+1; id++ {
		if _, err := s.furnitureWindowsTx(context.Background(), tx, []int64{id}, at, at); err != nil {
			t.Fatal(err)
		}
	}
	if len(s.furnitureEffects.entries) > furnitureCacheMaxEntries || s.furnitureEffects.bytes > furnitureCacheMaxBytes || s.furnitureEffects.entries[1] != nil {
		t.Fatal("cache not bounded/LRU")
	}
}

func BenchmarkFurnitureEffectCacheWarm(b *testing.B) {
	s := &Store{}
	tx := &furnitureCacheTestTx{}
	at := time.Now()
	ids := []int64{1, 2, 3, 4}
	if _, err := s.furnitureWindowsTx(context.Background(), tx, ids, at, at); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := s.furnitureWindowsTx(context.Background(), tx, ids, at, at); err != nil {
			b.Fatal(err)
		}
	}
	b.StopTimer()
	if tx.queries.Load() != 1 {
		b.Fatal("cache hit issued SQL")
	}
}
