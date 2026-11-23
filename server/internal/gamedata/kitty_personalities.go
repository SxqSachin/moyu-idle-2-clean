package gamedata

import (
	"strings"

	"mysecondwebgame/server/internal/config"
	"mysecondwebgame/server/internal/store"
)

// KittyPersonalityConfigsFromCatalog converts raw catalog rows into typed personality configs.
func KittyPersonalityConfigsFromCatalog(rows []map[string]any) []config.SharedKittyPersonalityConfig {
	out := make([]config.SharedKittyPersonalityConfig, 0, len(rows))
	seen := map[string]struct{}{}
	for _, row := range rows {
		id := normalizedPersonalityValue(row["id"])
		if id == "" {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		effectsRaw, _ := row["effects"].([]any)
		effects := make([]config.SharedKittyPersonalityEffect, 0, len(effectsRaw))
		for _, effectRaw := range effectsRaw {
			effectMap, _ := effectRaw.(map[string]any)
			kind := normalizedPersonalityValue(effectMap["kind"])
			if kind == "" {
				continue
			}
			attrs := map[string]float64{}
			if attrMap, ok := effectMap["attrs"].(map[string]any); ok {
				for rawAttrKey, rawValue := range attrMap {
					attrKey := normalizedPersonalityValue(rawAttrKey)
					if attrKey == "" {
						continue
					}
					value := toFloat64(rawValue)
					if value == 0 {
						continue
					}
					attrs[attrKey] = value
				}
			}
			effects = append(effects, config.SharedKittyPersonalityEffect{
				Kind:            kind,
				Attrs:           attrs,
				Position:        normalizedPersonalityPosition(effectMap["position"]),
				BattleHooks:     cloneAnyMapSlice(effectMap["battleHooks"]),
				EffectMutations: cloneAnyMapSlice(effectMap["effectMutations"]),
			})
		}
		out = append(out, config.SharedKittyPersonalityConfig{
			ID:      id,
			NameKey: strings.TrimSpace(toStringValue(row["nameKey"])),
			DescKey: strings.TrimSpace(toStringValue(row["descKey"])),
			Weight:  nonNegativeFloat64(row["weight"]),
			Effects: effects,
		})
	}
	return out
}

// KittyPersonalityProfileMap converts typed configs to the store's runtime profile map.
func KittyPersonalityProfileMap(rows []config.SharedKittyPersonalityConfig) map[string]store.KittyPersonalityProfile {
	out := make(map[string]store.KittyPersonalityProfile, len(rows))
	for _, row := range rows {
		id := strings.ToLower(strings.TrimSpace(row.ID))
		if id == "" {
			continue
		}
		effects := make([]store.KittyPersonalityEffect, 0, len(row.Effects))
		for _, effect := range row.Effects {
			attrs := make(map[string]float64, len(effect.Attrs))
			for attrKey, value := range effect.Attrs {
				normalizedAttrKey := strings.ToLower(strings.TrimSpace(attrKey))
				if normalizedAttrKey == "" {
					continue
				}
				attrs[normalizedAttrKey] = value
			}
			effects = append(effects, store.KittyPersonalityEffect{
				Kind:            strings.ToLower(strings.TrimSpace(effect.Kind)),
				Attrs:           attrs,
				Position:        strings.ToLower(strings.TrimSpace(effect.Position)),
				BattleHooks:     cloneAnyMapSlice(effect.BattleHooks),
				EffectMutations: cloneAnyMapSlice(effect.EffectMutations),
			})
		}
		out[id] = store.KittyPersonalityProfile{
			ID:      id,
			NameKey: strings.TrimSpace(row.NameKey),
			Weight:  row.Weight,
			Effects: effects,
		}
	}
	return out
}

// KittyPersonalityIDOrder returns the IDs in catalog order (deduped, normalized).
func KittyPersonalityIDOrder(rows []config.SharedKittyPersonalityConfig) []string {
	out := make([]string, 0, len(rows))
	seen := make(map[string]struct{}, len(rows))
	for _, row := range rows {
		id := strings.ToLower(strings.TrimSpace(row.ID))
		if id == "" {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

// ResolveDefaultKittyPersonalityID picks the configured default or falls back to first row.
func ResolveDefaultKittyPersonalityID(configured string, rows []config.SharedKittyPersonalityConfig) string {
	if len(rows) == 0 {
		return ""
	}
	normalized := normalizedPersonalityValue(configured)
	if normalized != "" {
		for _, row := range rows {
			if row.ID == normalized {
				return normalized
			}
		}
	}
	return rows[0].ID
}

// ── private helpers ──────────────────────────────────────────────────────────

func normalizedPersonalityValue(raw any) string {
	return strings.ToLower(strings.TrimSpace(toStringValue(raw)))
}

func normalizedPersonalityPosition(raw any) string {
	switch normalizedPersonalityValue(raw) {
	case "midguard":
		return "midguard"
	case "rearguard":
		return "rearguard"
	case "vanguard":
		return "vanguard"
	default:
		return ""
	}
}

func toStringValue(raw any) string {
	if s, ok := raw.(string); ok {
		return s
	}
	return ""
}

func toFloat64(raw any) float64 {
	switch v := raw.(type) {
	case float64:
		return v
	case int:
		return float64(v)
	case int64:
		return float64(v)
	default:
		return 0
	}
}

func nonNegativeFloat64(raw any) float64 {
	v := toFloat64(raw)
	if v < 0 {
		return 0
	}
	return v
}

func cloneAnyMapSlice(raw any) []map[string]any {
	rows, ok := raw.([]any)
	if !ok {
		if typed, typedOK := raw.([]map[string]any); typedOK {
			out := make([]map[string]any, 0, len(typed))
			for _, row := range typed {
				if row == nil {
					continue
				}
				out = append(out, cloneAnyMap(row))
			}
			return out
		}
		return nil
	}
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		node, ok := row.(map[string]any)
		if !ok || node == nil {
			continue
		}
		out = append(out, cloneAnyMap(node))
	}
	return out
}

func cloneAnyMap(raw map[string]any) map[string]any {
	if raw == nil {
		return nil
	}
	out := make(map[string]any, len(raw))
	for key, value := range raw {
		out[key] = cloneAnyValue(value)
	}
	return out
}

func cloneAnyValue(raw any) any {
	switch typed := raw.(type) {
	case map[string]any:
		return cloneAnyMap(typed)
	case []any:
		out := make([]any, 0, len(typed))
		for _, value := range typed {
			out = append(out, cloneAnyValue(value))
		}
		return out
	case []map[string]any:
		out := make([]any, 0, len(typed))
		for _, value := range typed {
			out = append(out, cloneAnyMap(value))
		}
		return out
	default:
		return raw
	}
}
