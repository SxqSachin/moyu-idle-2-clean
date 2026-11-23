package store

import (
	"fmt"
	"sort"
	"strings"

	"mysecondwebgame/server/internal/catalog"
)

const attributeExpEpsilon = 1e-9

type attributeProgressNode struct {
	Level   int64
	Exp     float64
	NextExp float64
}

type AttributeProgress struct {
	Level   int64
	Exp     float64
	NextExp float64
}

type attributeExpProfileResolver struct {
	profileByAttr map[string]string
	curves        map[string]map[int64]float64
	capTotalExp   map[string]float64
}

func buildAttributeExpProfileResolver(data catalog.Data) (*attributeExpProfileResolver, error) {
	curves := make(map[string]map[int64]float64)
	for _, row := range data.AttributeExpRules {
		profileID := strings.TrimSpace(row.ID)
		if profileID == "" {
			continue
		}
		level := row.Level
		if level <= 0 {
			return nil, fmt.Errorf("invalid attribute exp profile level: id=%s level=%d", profileID, level)
		}
		expRequired := float64(row.ExpRequired)
		if expRequired < 0 {
			return nil, fmt.Errorf("invalid attribute exp requirement: id=%s level=%d expRequired=%d", profileID, level, row.ExpRequired)
		}
		curve, ok := curves[profileID]
		if !ok {
			curve = make(map[int64]float64)
			curves[profileID] = curve
		}
		curve[level] = expRequired
	}

	profileByAttr := make(map[string]string)
	capTotalExpByAttr := make(map[string]float64)
	for _, row := range data.Attributes {
		attrKey := strings.TrimSpace(row.ID)
		if attrKey == "" || strings.TrimSpace(row.GrowthType) != "levelable" {
			continue
		}
		profileID := strings.TrimSpace(row.ExpProfileID)
		if profileID == "" {
			return nil, fmt.Errorf("missing expProfileId for levelable attribute: %s", attrKey)
		}
		curve, ok := curves[profileID]
		if !ok {
			return nil, fmt.Errorf("missing attribute exp profile: attr=%s profile=%s", attrKey, profileID)
		}
		capTotal, err := computeProfileCapTotalExp(curve)
		if err != nil {
			return nil, fmt.Errorf("invalid attribute exp profile: attr=%s profile=%s err=%w", attrKey, profileID, err)
		}
		profileByAttr[attrKey] = profileID
		capTotalExpByAttr[attrKey] = capTotal
	}

	return &attributeExpProfileResolver{
		profileByAttr: profileByAttr,
		curves:        curves,
		capTotalExp:   capTotalExpByAttr,
	}, nil
}

func computeProfileCapTotalExp(curve map[int64]float64) (float64, error) {
	if len(curve) == 0 {
		return 0, fmt.Errorf("profile has no rows")
	}
	levels := make([]int64, 0, len(curve))
	for level := range curve {
		levels = append(levels, level)
	}
	sort.Slice(levels, func(i, j int) bool { return levels[i] < levels[j] })
	if levels[0] != 1 {
		return 0, fmt.Errorf("profile must start at level 1")
	}
	total := float64(0)
	expected := int64(1)
	for _, level := range levels {
		if level != expected {
			break
		}
		required := curve[level]
		if required < 0 {
			return 0, fmt.Errorf("expRequired must be >= 0 at level %d", level)
		}
		if required == 0 {
			if level != levels[len(levels)-1] {
				return 0, fmt.Errorf("terminal expRequired=0 must be the last contiguous level, got level %d", level)
			}
			return total, nil
		}
		total += required
		expected++
	}
	return total, nil
}

func (r *attributeExpProfileResolver) progress(attrKey string, totalExp float64) (attributeProgressNode, error) {
	key := strings.TrimSpace(attrKey)
	profileID := r.profileByAttr[key]
	if profileID == "" {
		return attributeProgressNode{}, fmt.Errorf("attribute exp profile not found for attr: %s", key)
	}
	curve := r.curves[profileID]
	if curve == nil {
		return attributeProgressNode{}, fmt.Errorf("attribute exp curve missing for profile: %s", profileID)
	}
	if totalExp < 0 {
		totalExp = 0
	}
	remaining := totalExp
	level := int64(1)
	for {
		required, ok := curve[level]
		if !ok {
			return attributeProgressNode{Level: level, Exp: 0, NextExp: 0}, nil
		}
		if required <= 0 {
			return attributeProgressNode{Level: level, Exp: 0, NextExp: 0}, nil
		}
		if remaining+attributeExpEpsilon < required {
			if remaining < 0 {
				remaining = 0
			}
			return attributeProgressNode{Level: level, Exp: remaining, NextExp: required}, nil
		}
		remaining -= required
		if remaining < 0 && remaining > -attributeExpEpsilon {
			remaining = 0
		}
		level++
	}
}

func (r *attributeExpProfileResolver) clampTotalExp(attrKey string, totalExp float64) (float64, error) {
	key := strings.TrimSpace(attrKey)
	capTotal, ok := r.capTotalExp[key]
	if !ok {
		return 0, fmt.Errorf("attribute exp cap not found for attr: %s", key)
	}
	if totalExp < 0 {
		return 0, nil
	}
	if totalExp > capTotal {
		return capTotal, nil
	}
	return totalExp, nil
}

func (r *attributeExpProfileResolver) totalExpBeforeLevel(attrKey string, level int64) (float64, error) {
	key := strings.TrimSpace(attrKey)
	profileID := r.profileByAttr[key]
	if profileID == "" {
		return 0, fmt.Errorf("attribute exp profile not found for attr: %s", key)
	}
	if level <= 1 {
		return 0, nil
	}
	curve := r.curves[profileID]
	if curve == nil {
		return 0, fmt.Errorf("attribute exp curve missing for profile: %s", profileID)
	}
	capTotal, ok := r.capTotalExp[key]
	if !ok {
		return 0, fmt.Errorf("attribute exp cap not found for attr: %s", key)
	}
	total := float64(0)
	for current := int64(1); current < level; current++ {
		required, exists := curve[current]
		if !exists {
			return capTotal, nil
		}
		if required <= 0 {
			return capTotal, nil
		}
		total += required
		if total >= capTotal {
			return capTotal, nil
		}
	}
	return total, nil
}

func (s *Store) resolveAttributeProgress(attrKey string, totalExp float64) (attributeProgressNode, error) {
	return s.attributeExpProfile.progress(attrKey, totalExp)
}

func (s *Store) ResolveAttributeProgress(attrKey string, totalExp float64) (AttributeProgress, error) {
	progress, err := s.resolveAttributeProgress(attrKey, totalExp)
	if err != nil {
		return AttributeProgress{}, err
	}
	return AttributeProgress{
		Level:   progress.Level,
		Exp:     progress.Exp,
		NextExp: progress.NextExp,
	}, nil
}
