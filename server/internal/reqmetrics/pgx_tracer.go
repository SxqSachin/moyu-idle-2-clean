package reqmetrics

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type pgxQueryTracer struct{}

type pgxQueryStartKey struct{}
type pgxAcquireStartKey struct{}

type pgxQueryTraceState struct {
	startAt time.Time
	sql     string
}

const defaultPGSlowQueryLogThreshold = 200 * time.Millisecond

var pgSlowQueryLogThreshold = loadPGSlowQueryLogThreshold()

func NewPGXQueryTracer() pgx.QueryTracer {
	return pgxQueryTracer{}
}

func (pgxQueryTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	return context.WithValue(ctx, pgxQueryStartKey{}, pgxQueryTraceState{
		startAt: time.Now(),
		sql:     data.SQL,
	})
}

func (pgxQueryTracer) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryEndData) {
	state, ok := ctx.Value(pgxQueryStartKey{}).(pgxQueryTraceState)
	if !ok || state.startAt.IsZero() {
		return
	}
	elapsed := time.Since(state.startAt)
	if collector := FromContext(ctx); collector != nil {
		collector.AddPG(elapsed)
	}
	logSlowPGQuery(state.sql, elapsed, data.Err)
}

func (pgxQueryTracer) TraceAcquireStart(ctx context.Context, _ *pgxpool.Pool, _ pgxpool.TraceAcquireStartData) context.Context {
	return context.WithValue(ctx, pgxAcquireStartKey{}, time.Now())
}

func (pgxQueryTracer) TraceAcquireEnd(ctx context.Context, pool *pgxpool.Pool, _ pgxpool.TraceAcquireEndData) {
	collector := FromContext(ctx)
	if collector == nil {
		return
	}
	if startAt, ok := ctx.Value(pgxAcquireStartKey{}).(time.Time); ok && !startAt.IsZero() {
		collector.AddPGAcquire(time.Since(startAt))
	}
	if pool != nil {
		stat := pool.Stat()
		collector.SetPoolStats(stat.AcquiredConns(), stat.IdleConns(), stat.MaxConns())
	}
}

func loadPGSlowQueryLogThreshold() time.Duration {
	raw := strings.TrimSpace(os.Getenv("REQMETRICS_PG_SLOW_LOG_MS"))
	if raw == "" {
		return defaultPGSlowQueryLogThreshold
	}
	ms, err := strconv.ParseFloat(raw, 64)
	if err != nil || ms <= 0 {
		return 0
	}
	return time.Duration(ms * float64(time.Millisecond))
}

func logSlowPGQuery(sql string, elapsed time.Duration, err error) {
	if pgSlowQueryLogThreshold <= 0 || elapsed < pgSlowQueryLogThreshold {
		return
	}
	log.Printf(
		"pg query slow bucket=%s elapsed_ms=%.3f err=%s fingerprint=%q",
		classifyPGQuery(sql),
		float64(elapsed)/float64(time.Millisecond),
		formatPGTraceErr(err),
		fingerprintPGQuery(sql),
	)
}

func formatPGTraceErr(err error) string {
	if err == nil {
		return "nil"
	}
	return fmt.Sprintf("%T", err)
}

func classifyPGQuery(sql string) string {
	normalized := strings.ToLower(normalizePGQuery(sql))
	switch {
	case strings.Contains(normalized, "pg_advisory_xact_lock"):
		return "advisory_lock"
	case strings.Contains(normalized, "player_cat_village_tasks") ||
		strings.Contains(normalized, "player_cat_village_task_state") ||
		strings.Contains(normalized, "player_cat_village_reputation") ||
		strings.Contains(normalized, "player_rumor_pity") ||
		strings.Contains(normalized, "player_rumor_completion"):
		return "cat_village"
	case strings.Contains(normalized, "player_item_instances") && strings.Contains(normalized, "for update"):
		return "inventory_lock"
	case strings.Contains(normalized, "player_item_instances"):
		return "inventory"
	case strings.Contains(normalized, "player_warehouse_change_logs"):
		return "warehouse_audit"
	case strings.Contains(normalized, "achievement_events"):
		return "achievement_events"
	case strings.Contains(normalized, "player_presence"):
		return "presence"
	case strings.Contains(normalized, "battle_rooms") &&
		strings.Contains(normalized, "owner_worker_id") &&
		strings.Contains(normalized, "lease_until") &&
		strings.Contains(normalized, "for update"):
		return "battle_room_claim"
	case strings.Contains(normalized, "for update"):
		return "row_lock"
	default:
		return "other"
	}
}

func normalizePGQuery(sql string) string {
	fields := strings.Fields(sql)
	if len(fields) == 0 {
		return ""
	}
	return strings.Join(fields, " ")
}

func fingerprintPGQuery(sql string) string {
	fingerprint := normalizePGQuery(sql)
	const maxFingerprintBytes = 240
	if len(fingerprint) > maxFingerprintBytes {
		return fingerprint[:maxFingerprintBytes] + "..."
	}
	return fingerprint
}
