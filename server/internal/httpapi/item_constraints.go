package httpapi

import (
	"strings"

	"mysecondwebgame/server/internal/catalog"
)

func normalizeEquipTargets(targets []string) map[string]struct{} {
	out := make(map[string]struct{}, len(targets))
	for _, raw := range targets {
		switch strings.ToLower(strings.TrimSpace(raw)) {
		case "player":
			out["player"] = struct{}{}
		case "kitty":
			out["kitty"] = struct{}{}
		}
	}
	return out
}

func isEquipTargetAllowed(template *catalog.EquipmentTemplate, target string) bool {
	if template == nil {
		return false
	}
	normalized := strings.ToLower(strings.TrimSpace(target))
	if normalized != "player" && normalized != "kitty" {
		return false
	}
	targets := normalizeEquipTargets(template.EquipTargets)
	if len(targets) == 0 || len(targets) >= 2 {
		return true
	}
	_, ok := targets[normalized]
	return ok
}

func kittyFeedBattleLevelCapGain(item *catalog.Item) int64 {
	if item == nil {
		return 0
	}
	gain := item.KittyFeed.BattleLevelCapGain
	if gain <= 0 {
		return 0
	}
	return gain
}
