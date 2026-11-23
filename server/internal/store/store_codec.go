package store

import (
	"encoding/json"
	"strings"
)

func decodeAffixSlots(raw []byte) []EquipmentAffixSlot {
	if len(raw) == 0 {
		return []EquipmentAffixSlot{}
	}
	var out []EquipmentAffixSlot
	if err := json.Unmarshal(raw, &out); err == nil {
		for idx := range out {
			if strings.TrimSpace(out[idx].SourceKind) == "" {
				out[idx].SourceKind = "template"
			}
		}
		return out
	}
	return []EquipmentAffixSlot{}
}

func encodeAffixSlots(slots []EquipmentAffixSlot) []byte {
	if slots == nil {
		slots = []EquipmentAffixSlot{}
	}
	raw, err := json.Marshal(slots)
	if err != nil {
		return []byte("[]")
	}
	return raw
}

// affixesJSONEmptyRaw treats [] and {} as empty for historical compatibility.
func affixesJSONEmptyRaw(raw []byte) bool {
	if len(raw) == 0 {
		return true
	}
	var node any
	if err := json.Unmarshal(raw, &node); err != nil {
		return false
	}
	switch typed := node.(type) {
	case nil:
		return true
	case []any:
		return len(typed) == 0
	case map[string]any:
		return len(typed) == 0
	default:
		return false
	}
}

func countEquippedAffixes(raw []byte) int {
	slots := decodeAffixSlots(raw)
	count := 0
	for _, s := range slots {
		if strings.TrimSpace(s.AffixID) != "" {
			count++
		}
	}
	return count
}

func decodeMilestoneAffixClaims(raw []byte) map[string]bool {
	out := map[string]bool{}
	if len(raw) == 0 {
		return out
	}
	var node map[string]any
	if err := json.Unmarshal(raw, &node); err != nil {
		return out
	}
	for key, value := range node {
		if key == "" {
			continue
		}
		asBool, ok := value.(bool)
		if ok {
			out[key] = asBool
			continue
		}
		// Backward/forward compatibility for numeric flags.
		asNum, ok := value.(float64)
		if ok {
			out[key] = asNum != 0
		}
	}
	return out
}

func encodeMilestoneAffixClaims(claims map[string]bool) []byte {
	if claims == nil {
		claims = map[string]bool{}
	}
	raw, err := json.Marshal(claims)
	if err != nil {
		return []byte("{}")
	}
	return raw
}
