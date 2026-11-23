package store

import (
	"container/list"
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"mysecondwebgame/server/internal/operationeffect"
)

type furnitureRuntimePolicy struct{ lease, delay time.Duration }

func (s *Store) furnitureRuntimePolicy() furnitureRuntimePolicy {
	if s.furniturePolicy != nil {
		return *s.furniturePolicy
	}
	return furnitureRuntimePolicy{lease: 30 * time.Second, delay: 60 * time.Second}
}

const furnitureCacheMaxBytes = 32 << 20
const furnitureCacheMaxEntries = 10000

type furnitureEffectEntry struct {
	userID                 int64
	from, through, expires time.Time
	windows                []operationeffect.PassiveWindow
	bytes                  int
}

type furnitureEffectFlight struct {
	done chan struct{}
	err  error
}

// Entries own immutable parsed windows. Mutable reward accounting never enters
// this cache. Cold readers and writers use a separate, furniture-only lock.
type furnitureEffectCache struct {
	mu      sync.Mutex
	entries map[int64]*list.Element
	lru     list.List
	flights map[int64]*furnitureEffectFlight
	bytes   int
}

func (c *furnitureEffectCache) remove(el *list.Element) {
	entry := el.Value.(*furnitureEffectEntry)
	delete(c.entries, entry.userID)
	c.bytes -= entry.bytes
	c.lru.Remove(el)
}

func (s *Store) InvalidateFurnitureEffects(userID int64) {
	c := &s.furnitureEffects
	c.mu.Lock()
	defer c.mu.Unlock()
	if el := c.entries[userID]; el != nil {
		c.remove(el)
	}
}

// A source-only lock never waits for a user/room/inventory lock. The loader
// takes it before its SELECT, in a separate statement, to obtain a post-commit
// READ COMMITTED snapshot even when a furniture command was still committing.
func lockFurnitureSourcesTx(ctx context.Context, tx pgx.Tx, ids []int64, shared bool) error {
	fn := "pg_advisory_xact_lock"
	if shared {
		fn += "_shared"
	}
	_, err := tx.Exec(ctx, `SELECT `+fn+`(hashtextextended('cattery-source:' || id::text,0)) FROM unnest($1::bigint[]) AS id ORDER BY id`, ids)
	return err
}

func (s *Store) furnitureWindowsTx(ctx context.Context, tx pgx.Tx, ids []int64, from, through time.Time) (map[int64][]operationeffect.PassiveWindow, error) {
	ids = append([]int64(nil), ids...)
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	ids = uniqueFurnitureUsers(ids)
	result := make(map[int64][]operationeffect.PassiveWindow, len(ids))
	c, policy := &s.furnitureEffects, s.furnitureRuntimePolicy()
	for len(result) < len(ids) {
		started := time.Now()
		missing := []int64{}
		var wait *furnitureEffectFlight
		c.mu.Lock()
		if c.entries == nil {
			c.entries = map[int64]*list.Element{}
			c.flights = map[int64]*furnitureEffectFlight{}
		}
		for _, id := range ids {
			if _, ok := result[id]; ok {
				continue
			}
			if el := c.entries[id]; el != nil {
				e := el.Value.(*furnitureEffectEntry)
				if started.Before(e.expires) && !from.Before(e.from) && !through.After(e.through) {
					result[id] = e.windows
					c.lru.MoveToFront(el)
					continue
				}
			}
			if flight := c.flights[id]; flight != nil {
				wait = flight
				continue
			}
			missing = append(missing, id)
		}
		for _, id := range missing {
			c.flights[id] = &furnitureEffectFlight{done: make(chan struct{})}
		}
		c.mu.Unlock()
		if len(missing) == 0 {
			if wait == nil {
				break
			}
			select {
			case <-wait.done:
				if wait.err != nil {
					return nil, wait.err
				}
				continue
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		loaded, err := loadFurnitureWindowsTx(ctx, tx, missing, from)
		c.mu.Lock()
		for _, id := range missing {
			if err == nil {
				e := loaded[id]
				e.expires = started.Add(policy.lease)
				e.through = e.expires
				result[id] = e.windows
				if el := c.entries[id]; el != nil {
					c.remove(el)
				}
				if time.Now().Before(e.expires) && e.bytes <= furnitureCacheMaxBytes/4 {
					for c.bytes+e.bytes > furnitureCacheMaxBytes || len(c.entries) >= furnitureCacheMaxEntries {
						c.remove(c.lru.Back())
					}
					c.entries[id] = c.lru.PushFront(e)
					c.bytes += e.bytes
				}
			}
			c.flights[id].err = err
			close(c.flights[id].done)
			delete(c.flights, id)
		}
		c.mu.Unlock()
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

func uniqueFurnitureUsers(ids []int64) []int64 {
	out := ids[:0]
	for _, id := range ids {
		if len(out) == 0 || out[len(out)-1] != id {
			out = append(out, id)
		}
	}
	return out
}

func loadFurnitureWindowsTx(ctx context.Context, tx pgx.Tx, ids []int64, from time.Time) (map[int64]*furnitureEffectEntry, error) {
	if err := lockFurnitureSourcesTx(ctx, tx, ids, true); err != nil {
		return nil, err
	}
	// Include scheduled future windows; entry leases bound how far callers may
	// trust the absence of a later revision. Bound each user's history separately.
	rows, err := tx.Query(ctx, `SELECT u.id,p.valid_from,p.valid_until,p.source FROM unnest($1::bigint[]) u(id)
	LEFT JOIN LATERAL (SELECT valid_from,valid_until,source FROM player_cattery_furniture_periods
	 WHERE user_id=u.id AND (valid_until IS NULL OR valid_until>$2)
	 ORDER BY valid_from,id LIMIT 4097) p ON true ORDER BY u.id,p.valid_from`, ids, from)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[int64]*furnitureEffectEntry, len(ids))
	for _, id := range ids {
		out[id] = &furnitureEffectEntry{userID: id, from: from, bytes: 256}
	}
	for rows.Next() {
		var id int64
		var start, end *time.Time
		var raw []byte
		if err = rows.Scan(&id, &start, &end, &raw); err != nil {
			return nil, err
		}
		if start == nil {
			continue
		}
		var source operationeffect.PassiveSource
		if err = json.Unmarshal(raw, &source); err != nil {
			return nil, err
		}
		if err = (operationeffect.PassiveSnapshot{Version: 1, Sources: []operationeffect.PassiveSource{source}}).Validate(); err != nil {
			return nil, err
		}
		e := out[id]
		window := operationeffect.PassiveWindow{From: *start, Source: source}
		if end != nil {
			window.Until = *end
		}
		e.windows = append(e.windows, window)
		e.bytes += len(raw)*4 + 256
		if len(e.windows) > 4096 {
			return nil, fmt.Errorf("furniture recovery interval budget exceeded")
		}
	}
	return out, rows.Err()
}
