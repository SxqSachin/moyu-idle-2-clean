package store

import (
	"context"
	"sort"
	"strings"
)

const auditSourceContextKey = "audit_source"

func WithAuditSource(ctx context.Context, source string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	source = strings.TrimSpace(source)
	if source == "" {
		return ctx
	}
	return context.WithValue(ctx, auditSourceContextKey, source)
}

func auditSourceFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if raw := ctx.Value(auditSourceContextKey); raw != nil {
		if value, ok := raw.(string); ok {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func inventoryAuditCategorySummary(events []InventoryAuditEvent) string {
	if len(events) == 0 {
		return ""
	}
	counts := make(map[string]int, len(events))
	for _, event := range events {
		category := strings.TrimSpace(string(event.Category))
		if category == "" {
			category = "unknown"
		}
		counts[category]++
	}
	keys := make([]string, 0, len(counts))
	for key := range counts {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+":"+itoa(counts[key]))
	}
	return strings.Join(parts, ",")
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	sign := ""
	if v < 0 {
		sign = "-"
		v = -v
	}
	buf := make([]byte, 0, 12)
	for v > 0 {
		buf = append(buf, byte('0'+v%10))
		v /= 10
	}
	for left, right := 0, len(buf)-1; left < right; left, right = left+1, right-1 {
		buf[left], buf[right] = buf[right], buf[left]
	}
	return sign + string(buf)
}
