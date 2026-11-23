package store

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"mysecondwebgame/server/internal/catalog"
)

type EnhanceAttemptPreview struct {
	NextLevel   int                `json:"nextLevel"`
	SuccessRate float64            `json:"successRate"`
	Costs       map[string]int64   `json:"costs"`
	DeltaAttrs  map[string]float64 `json:"deltaAttrs"`
}

type EnhanceTemplateRuntime struct {
	TemplateID          string                                           `json:"templateId"`
	RarityID            string                                           `json:"rarityId"`
	MaxLevel            int                                              `json:"maxLevel"`
	Duration            time.Duration                                    `json:"duration"`
	EnhanceExp          float64                                          `json:"enhanceExp"`
	ProtectItemIDs      []string                                         `json:"protectItemIds,omitempty"`
	AffixMilestonePools map[int][]catalog.EnhanceAffixMilestonePoolEntry `json:"affixMilestonePools,omitempty"`
	CumulativeByLevel   map[int]map[string]float64                       `json:"cumulativeByLevel"`
	NextPreviewByLevel  map[int]EnhanceAttemptPreview                    `json:"nextPreviewByLevel"`
}

type EnhanceRuntimeCache struct {
	byTemplateID map[string]EnhanceTemplateRuntime
}

func buildEnhanceRuntimeCache(data catalog.Data) (*EnhanceRuntimeCache, error) {
	cache := &EnhanceRuntimeCache{
		byTemplateID: make(map[string]EnhanceTemplateRuntime, len(data.EquipmentTemplates)),
	}
	for _, template := range data.EquipmentTemplates {
		templateID := strings.TrimSpace(template.ID)
		if templateID == "" {
			continue
		}
		rarityID := "common"
		if item := data.ItemByID(templateID); item != nil {
			rarityID = normalizeEnhanceRarity(item.Rarity)
		}
		runtime, err := buildEnhanceTemplateRuntime(template, rarityID)
		if err != nil {
			return nil, fmt.Errorf("build enhance runtime failed template=%s: %w", templateID, err)
		}
		cache.byTemplateID[templateID] = runtime
	}
	return cache, nil
}

func buildEnhanceTemplateRuntime(template catalog.EquipmentTemplate, rarityID string) (EnhanceTemplateRuntime, error) {
	maxLevel := int(template.EnhanceMaxLevel)
	if maxLevel < 0 {
		return EnhanceTemplateRuntime{}, fmt.Errorf("enhanceMaxLevel must be >= 0")
	}
	if maxLevel > 1000 {
		return EnhanceTemplateRuntime{}, fmt.Errorf("enhanceMaxLevel exceeds cap: %d", maxLevel)
	}
	duration := time.Duration(template.EnhanceDurationS) * time.Second
	if template.EnhanceDurationS <= 0 {
		return EnhanceTemplateRuntime{}, fmt.Errorf("enhanceDurationS must be > 0")
	}
	runtime := EnhanceTemplateRuntime{
		TemplateID:          strings.TrimSpace(template.ID),
		RarityID:            normalizeEnhanceRarity(rarityID),
		MaxLevel:            maxLevel,
		Duration:            duration,
		EnhanceExp:          normalizeEnhanceExp(template.EnhanceExp),
		ProtectItemIDs:      normalizeProtectItemIDs(template.EnhanceProtectItem),
		AffixMilestonePools: make(map[int][]catalog.EnhanceAffixMilestonePoolEntry, len(template.EnhanceAffixMilestonePools)),
		CumulativeByLevel:   make(map[int]map[string]float64, maxLevel+1),
		NextPreviewByLevel:  make(map[int]EnhanceAttemptPreview, maxLevel+1),
	}
	for levelKey, pool := range template.EnhanceAffixMilestonePools {
		level, err := strconv.Atoi(strings.TrimSpace(levelKey))
		if err != nil || level <= 0 {
			return EnhanceTemplateRuntime{}, fmt.Errorf("enhanceAffixMilestonePools level=%s: invalid", levelKey)
		}
		if maxLevel > 0 && level > maxLevel {
			return EnhanceTemplateRuntime{}, fmt.Errorf("enhanceAffixMilestonePools level=%s exceeds maxLevel", levelKey)
		}
		normalizedPool := make([]catalog.EnhanceAffixMilestonePoolEntry, 0, len(pool))
		for idx, entry := range pool {
			affixID := strings.TrimSpace(entry.AffixID)
			rarityID := strings.TrimSpace(entry.RarityID)
			if affixID == "" || rarityID == "" {
				return EnhanceTemplateRuntime{}, fmt.Errorf("enhanceAffixMilestonePools entry invalid level=%s index=%d", levelKey, idx)
			}
			var rollValue *float64
			if entry.RollValue != nil {
				val := *entry.RollValue
				if math.IsNaN(val) || math.IsInf(val, 0) || val < 0 || val > 1 {
					return EnhanceTemplateRuntime{}, fmt.Errorf("enhanceAffixMilestonePools rollValue invalid level=%s index=%d", levelKey, idx)
				}
				rollValue = &val
			}
			normalizedPool = append(normalizedPool, catalog.EnhanceAffixMilestonePoolEntry{
				AffixID:   affixID,
				RarityID:  rarityID,
				RollValue: rollValue,
			})
		}
		if len(normalizedPool) > 0 {
			runtime.AffixMilestonePools[level] = normalizedPool
		}
	}
	runtime.CumulativeByLevel[0] = map[string]float64{}
	for level := 1; level <= maxLevel; level++ {
		increment, err := resolveEnhanceAttrIncrement(template.EnhanceAttrCurve, level, maxLevel)
		if err != nil {
			return EnhanceTemplateRuntime{}, fmt.Errorf("enhanceAttrCurve level=%d: %w", level, err)
		}
		runtime.CumulativeByLevel[level] = mergeAttrMaps(runtime.CumulativeByLevel[level-1], increment)
	}
	for current := 0; current <= maxLevel; current++ {
		nextLevel := current + 1
		if nextLevel > maxLevel {
			runtime.NextPreviewByLevel[current] = EnhanceAttemptPreview{
				NextLevel:   nextLevel,
				SuccessRate: 0,
				Costs:       map[string]int64{},
				DeltaAttrs:  map[string]float64{},
			}
			continue
		}
		successRate := resolveEnhanceSuccessRate(template, runtime.RarityID, nextLevel, maxLevel)
		costs, err := resolveEnhanceCosts(template.EnhanceCostCurve, nextLevel, maxLevel)
		if err != nil {
			return EnhanceTemplateRuntime{}, fmt.Errorf("enhanceCostCurve level=%d: %w", nextLevel, err)
		}
		deltaAttrs, err := subtractAttrMaps(runtime.CumulativeByLevel[nextLevel], runtime.CumulativeByLevel[current])
		if err != nil {
			return EnhanceTemplateRuntime{}, fmt.Errorf("enhanceAttrCurve level=%d: %w", nextLevel, err)
		}
		runtime.NextPreviewByLevel[current] = EnhanceAttemptPreview{
			NextLevel:   nextLevel,
			SuccessRate: successRate,
			Costs:       costs,
			DeltaAttrs:  deltaAttrs,
		}
	}
	return runtime, nil
}

func (c *EnhanceRuntimeCache) Template(templateID string) (EnhanceTemplateRuntime, bool) {
	if c == nil {
		return EnhanceTemplateRuntime{}, false
	}
	runtime, ok := c.byTemplateID[strings.TrimSpace(templateID)]
	return runtime, ok
}

func (c *EnhanceRuntimeCache) CumulativeAttrs(templateID string, level int) map[string]float64 {
	runtime, ok := c.Template(templateID)
	if !ok {
		return map[string]float64{}
	}
	if level < 0 {
		level = 0
	}
	if level > runtime.MaxLevel {
		level = runtime.MaxLevel
	}
	return cloneAttrMap(runtime.CumulativeByLevel[level])
}

func (s *Store) EquipmentEnhanceCumulativeAttrs(templateID string, level int) map[string]float64 {
	if s == nil || s.enhanceCache == nil {
		return map[string]float64{}
	}
	return s.enhanceCache.CumulativeAttrs(templateID, level)
}

func (c *EnhanceRuntimeCache) NextPreview(templateID string, level int) (EnhanceAttemptPreview, bool) {
	runtime, ok := c.Template(templateID)
	if !ok {
		return EnhanceAttemptPreview{}, false
	}
	if level < 0 {
		level = 0
	}
	if level > runtime.MaxLevel {
		level = runtime.MaxLevel
	}
	preview, exists := runtime.NextPreviewByLevel[level]
	if !exists {
		return EnhanceAttemptPreview{}, false
	}
	return EnhanceAttemptPreview{
		NextLevel:   preview.NextLevel,
		SuccessRate: preview.SuccessRate,
		Costs:       cloneCostMap(preview.Costs),
		DeltaAttrs:  cloneAttrMap(preview.DeltaAttrs),
	}, true
}

func (c *EnhanceRuntimeCache) AffixMilestonePool(templateID string, level int) ([]catalog.EnhanceAffixMilestonePoolEntry, bool) {
	runtime, ok := c.Template(templateID)
	if !ok {
		return nil, false
	}
	if level < 0 {
		level = 0
	}
	if level > runtime.MaxLevel {
		level = runtime.MaxLevel
	}
	pool, exists := runtime.AffixMilestonePools[level]
	if !exists || len(pool) == 0 {
		return nil, false
	}
	return pool, true
}

func resolveEnhanceCosts(curve catalog.EnhanceCostCurve, level int, maxLevel int) (map[string]int64, error) {
	if level <= 0 {
		return nil, fmt.Errorf("level must be positive")
	}
	if maxLevel > 0 && level > maxLevel {
		return nil, fmt.Errorf("level exceeds enhanceMaxLevel")
	}
	override, hasOverride, err := resolveOverrideCostMap(curve.Overrides, level, maxLevel)
	if err != nil {
		return nil, err
	}
	if hasOverride {
		if err := validateCostMap(override); err != nil {
			return nil, err
		}
		return cloneCostMap(override), nil
	}
	out := make(map[string]int64, len(curve.Base)+len(curve.Step))
	for itemID, base := range curve.Base {
		normalizedItemID := strings.TrimSpace(itemID)
		if normalizedItemID == "" {
			return nil, fmt.Errorf("cost item id empty")
		}
		step := curve.Step[normalizedItemID]
		value, err := safeInt64Linear(base, step, level-1)
		if err != nil {
			return nil, fmt.Errorf("cost overflow item=%s: %w", normalizedItemID, err)
		}
		if value < 0 {
			return nil, fmt.Errorf("cost must be >= 0 item=%s", normalizedItemID)
		}
		out[normalizedItemID] = value
	}
	for itemID, step := range curve.Step {
		normalizedItemID := strings.TrimSpace(itemID)
		if normalizedItemID == "" {
			return nil, fmt.Errorf("cost item id empty")
		}
		if _, exists := out[normalizedItemID]; exists {
			continue
		}
		value, err := safeInt64Linear(0, step, level-1)
		if err != nil {
			return nil, fmt.Errorf("cost overflow item=%s: %w", normalizedItemID, err)
		}
		if value < 0 {
			return nil, fmt.Errorf("cost must be >= 0 item=%s", normalizedItemID)
		}
		out[normalizedItemID] = value
	}
	return out, nil
}

func resolveEnhanceAttrIncrement(curve catalog.EnhanceAttrCurve, level int, maxLevel int) (map[string]float64, error) {
	if level <= 0 {
		return nil, fmt.Errorf("level must be positive")
	}
	if maxLevel > 0 && level > maxLevel {
		return nil, fmt.Errorf("level exceeds enhanceMaxLevel")
	}
	override, hasOverride, err := resolveOverrideAttrMap(curve.Overrides, level, maxLevel)
	if err != nil {
		return nil, err
	}
	if hasOverride {
		if err := validateAttrMap(override); err != nil {
			return nil, err
		}
		return cloneAttrMap(override), nil
	}
	out := make(map[string]float64, len(curve.Base)+len(curve.Step))
	for attrKey, base := range curve.Base {
		normalizedKey := strings.TrimSpace(attrKey)
		if normalizedKey == "" {
			return nil, fmt.Errorf("attr key empty")
		}
		value := base
		if level > 1 {
			value = curve.Step[normalizedKey]
		}
		if !isFiniteNumber(value) {
			return nil, fmt.Errorf("attr increment not finite attr=%s", normalizedKey)
		}
		out[normalizedKey] = value
	}
	for attrKey, step := range curve.Step {
		normalizedKey := strings.TrimSpace(attrKey)
		if normalizedKey == "" {
			return nil, fmt.Errorf("attr key empty")
		}
		if _, exists := out[normalizedKey]; exists {
			continue
		}
		value := float64(0)
		if level > 1 {
			value = step
		}
		if !isFiniteNumber(value) {
			return nil, fmt.Errorf("attr increment not finite attr=%s", normalizedKey)
		}
		out[normalizedKey] = value
	}
	return out, nil
}

func resolveOverrideCostMap(overrides map[string]map[string]int64, level int, maxLevel int) (map[string]int64, bool, error) {
	for key, value := range overrides {
		parsedLevel, err := parseEnhanceLevelKey(key, maxLevel)
		if err != nil {
			return nil, false, err
		}
		if parsedLevel != level {
			continue
		}
		return value, true, nil
	}
	return nil, false, nil
}

func resolveOverrideAttrMap(overrides map[string]map[string]float64, level int, maxLevel int) (map[string]float64, bool, error) {
	for key, value := range overrides {
		parsedLevel, err := parseEnhanceLevelKey(key, maxLevel)
		if err != nil {
			return nil, false, err
		}
		if parsedLevel != level {
			continue
		}
		return value, true, nil
	}
	return nil, false, nil
}

func parseEnhanceLevelKey(raw string, maxLevel int) (int, error) {
	normalized := strings.TrimSpace(raw)
	if normalized == "" {
		return 0, fmt.Errorf("enhance override key empty")
	}
	level, err := strconv.Atoi(normalized)
	if err != nil || level <= 0 {
		return 0, fmt.Errorf("enhance override key invalid: %s", raw)
	}
	if maxLevel > 0 && level > maxLevel {
		return 0, fmt.Errorf("enhance override key exceeds max level: %s", raw)
	}
	return level, nil
}

func validateCostMap(value map[string]int64) error {
	for itemID, qty := range value {
		if strings.TrimSpace(itemID) == "" {
			return fmt.Errorf("cost item id empty")
		}
		if qty < 0 {
			return fmt.Errorf("cost must be >= 0 item=%s", itemID)
		}
	}
	return nil
}

func validateAttrMap(value map[string]float64) error {
	for attrKey, delta := range value {
		if strings.TrimSpace(attrKey) == "" {
			return fmt.Errorf("attr key empty")
		}
		if !isFiniteNumber(delta) {
			return fmt.Errorf("attr delta not finite attr=%s", attrKey)
		}
	}
	return nil
}

func safeInt64Linear(base int64, step int64, multiplier int) (int64, error) {
	if multiplier < 0 {
		return 0, fmt.Errorf("multiplier must be >= 0")
	}
	product := float64(step) * float64(multiplier)
	if !isFiniteNumber(product) || product > float64(math.MaxInt64) || product < float64(math.MinInt64) {
		return 0, fmt.Errorf("overflow in multiplication")
	}
	value := float64(base) + product
	if !isFiniteNumber(value) || value > float64(math.MaxInt64) || value < float64(math.MinInt64) {
		return 0, fmt.Errorf("overflow in addition")
	}
	return int64(value), nil
}

func isFiniteNumber(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

func mergeAttrMaps(base map[string]float64, delta map[string]float64) map[string]float64 {
	out := cloneAttrMap(base)
	for key, value := range delta {
		out[key] += value
	}
	return out
}

func cloneAttrMap(value map[string]float64) map[string]float64 {
	if len(value) == 0 {
		return map[string]float64{}
	}
	out := make(map[string]float64, len(value))
	for key, item := range value {
		out[key] = item
	}
	return out
}

func cloneCostMap(value map[string]int64) map[string]int64 {
	if len(value) == 0 {
		return map[string]int64{}
	}
	out := make(map[string]int64, len(value))
	for key, item := range value {
		out[key] = item
	}
	return out
}

var enhanceRarityFailMultiplier = map[string]float64{
	"common":    1,
	"uncommon":  1.06,
	"rare":      1.12,
	"epic":      1.1,
	"legendary": 1.26,
	"mythic":    1.4,
}

func resolveEnhanceSuccessRateByFormula(rarityID string, level int, maxLevel int) float64 {
	if maxLevel <= 1 {
		return 1
	}
	if level < 1 {
		level = 1
	}
	if level > maxLevel {
		level = maxLevel
	}
	// minSuccessRate := clampFloat(0.50-(float64(maxLevel)-15)*(0.55/35), 0.15, 0.50)
	// curvePower := clampFloat(1.25-(float64(maxLevel)-15)*(0.50/35), 0.75, 1.25)

	minSuccessRate := clampFloat(0.45-(float64(maxLevel)-15)*(0.7/35), 0.15, 0.45)
	curvePower := clampFloat(1-(float64(maxLevel)-15)*(0.4/35), 0.4, 1)
	progress := float64(level-1) / float64(maxLevel-1)
	baseRate := minSuccessRate + (1-minSuccessRate)*(1-math.Pow(progress, curvePower))
	failMultiplier := enhanceRarityFailMultiplier[normalizeEnhanceRarity(rarityID)]
	if failMultiplier <= 0 {
		failMultiplier = 1
	}
	successRate := 1 - (1-baseRate)*failMultiplier
	return roundEnhanceRate(clampFloat(successRate, 0.01, 1))
}

func resolveEnhanceSuccessRate(template catalog.EquipmentTemplate, rarityID string, level int, maxLevel int) float64 {
	if template.EnhanceSuccessRateOverride == nil {
		return resolveEnhanceSuccessRateByFormula(rarityID, level, maxLevel)
	}
	return roundEnhanceRate(clampFloat(*template.EnhanceSuccessRateOverride, 0.01, 1))
}

func normalizeEnhanceRarity(raw string) string {
	value := strings.TrimSpace(strings.ToLower(raw))
	if _, ok := enhanceRarityFailMultiplier[value]; ok {
		return value
	}
	return "common"
}

func normalizeEnhanceExp(raw float64) float64 {
	if !isFiniteNumber(raw) || raw <= 0 {
		return 0
	}
	return roundEnhanceRate(raw)
}

func roundEnhanceRate(value float64) float64 {
	if !isFiniteNumber(value) {
		return 0
	}
	return math.Round(value*10000) / 10000
}

func clampFloat(value float64, min float64, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func subtractAttrMaps(next map[string]float64, current map[string]float64) (map[string]float64, error) {
	keys := make(map[string]struct{}, len(next)+len(current))
	for key := range next {
		keys[key] = struct{}{}
	}
	for key := range current {
		keys[key] = struct{}{}
	}
	out := make(map[string]float64, len(keys))
	for key := range keys {
		diff := next[key] - current[key]
		if !isFiniteNumber(diff) {
			return nil, fmt.Errorf("attr delta not finite attr=%s", key)
		}
		out[key] = diff
	}
	return out, nil
}

func normalizeProtectItemIDs(raw []string) []string {
	if len(raw) == 0 {
		return []string{}
	}
	seen := make(map[string]struct{}, len(raw))
	out := make([]string, 0, len(raw))
	for _, itemID := range raw {
		normalized := strings.TrimSpace(itemID)
		if normalized == "" {
			continue
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}
	return out
}

func (s *Store) EnhanceTemplateRuntime(templateID string) (EnhanceTemplateRuntime, bool) {
	if s == nil || s.enhanceCache == nil {
		return EnhanceTemplateRuntime{}, false
	}
	return s.enhanceCache.Template(templateID)
}

func (s *Store) EnhanceCumulativeAttrs(templateID string, level int) map[string]float64 {
	if s == nil || s.enhanceCache == nil {
		return map[string]float64{}
	}
	return s.enhanceCache.CumulativeAttrs(templateID, level)
}

func (s *Store) EnhanceNextPreview(templateID string, level int) (EnhanceAttemptPreview, bool) {
	if s == nil || s.enhanceCache == nil {
		return EnhanceAttemptPreview{}, false
	}
	return s.enhanceCache.NextPreview(templateID, level)
}
