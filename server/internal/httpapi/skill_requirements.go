package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"mysecondwebgame/server/internal/catalog"
	"mysecondwebgame/server/internal/store"
)

type battleSkillStatusRequirementGap struct {
	AttrID   string
	Required float64
	Current  float64
}

type battleSkillEnableEvaluation struct {
	Eligible             bool
	UnmetStatus          []battleSkillStatusRequirementGap
	MissingWeaponClasses []string
}

type battleSkillEnableContext struct {
	Equipped      []store.EquippedItem
	Attributes    map[string]float64
	WeaponClasses map[string]struct{}
}

func normalizeWeaponClassID(raw string) string {
	return strings.ToLower(strings.TrimSpace(raw))
}

func normalizeSkillStatusRequirement(raw map[string]any) map[string]float64 {
	out := map[string]float64{}
	value := raw["statusRequirement"]
	switch typed := value.(type) {
	case map[string]any:
		for key, v := range typed {
			attrID := strings.TrimSpace(key)
			required := toFloat64(v, 0)
			if attrID == "" || required <= 0 {
				continue
			}
			out[attrID] = required
		}
	case map[string]int64:
		for key, v := range typed {
			attrID := strings.TrimSpace(key)
			if attrID == "" || v <= 0 {
				continue
			}
			out[attrID] = float64(v)
		}
	}
	return out
}

func normalizeSkillWeaponRequirement(raw map[string]any) []string {
	candidates := toStringSlice(raw["weaponRequirement"])
	if len(candidates) == 0 {
		return nil
	}
	out := make([]string, 0, len(candidates))
	seen := make(map[string]struct{}, len(candidates))
	for _, row := range candidates {
		value := normalizeWeaponClassID(row)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func inferWeaponClassesFromTemplateID(templateID string) []string {
	normalized := strings.ToLower(strings.TrimSpace(templateID))
	switch {
	case strings.Contains(normalized, "crossbow"):
		return []string{"crossbow"}
	case strings.Contains(normalized, "bow"):
		return []string{"bow"}
	case strings.Contains(normalized, "spear"):
		return []string{"spear"}
	case strings.Contains(normalized, "sword"):
		return []string{"sword"}
	case strings.Contains(normalized, "staff"):
		return []string{"staff"}
	default:
		return nil
	}
}

func weaponClassesFromTemplate(template *catalog.EquipmentTemplate) []string {
	if template == nil {
		return nil
	}
	values := make([]string, 0, len(template.WeaponClass))
	seen := make(map[string]struct{}, len(template.WeaponClass))
	for _, raw := range template.WeaponClass {
		value := normalizeWeaponClassID(raw)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		values = append(values, value)
	}
	if len(values) > 0 {
		sort.Strings(values)
		return values
	}
	return inferWeaponClassesFromTemplateID(template.ID)
}

func (s *Server) battleSkillRawByID(skillID string) (map[string]any, bool) {
	return s.battleSkillRawByIDWithCustom(skillID, nil)
}

func (s *Server) battleSkillRawByIDWithCustom(skillID string, custom map[string]map[string]any) (map[string]any, bool) {
	normalized := strings.ToLower(strings.TrimSpace(skillID))
	if normalized == "" {
		return nil, false
	}
	for id, raw := range custom {
		if strings.ToLower(strings.TrimSpace(id)) == normalized {
			if !s.validAcceptedCustomBattleSkillRaw(normalized, raw) {
				return nil, false
			}
			return raw, true
		}
	}
	for _, raw := range s.deps.GameData.Skills {
		id, _ := stringFromAny(raw["id"])
		if strings.ToLower(strings.TrimSpace(id)) == normalized {
			return raw, true
		}
	}
	return nil, false
}

func (s *Server) validAcceptedCustomBattleSkillRaw(skillID string, raw map[string]any) bool {
	if !strings.HasPrefix(skillID, "custom:") || raw == nil {
		return false
	}
	rawID, _ := stringFromAny(raw["id"])
	if !strings.EqualFold(strings.TrimSpace(rawID), skillID) {
		return false
	}
	maxBytes := s.deps.Config.GlobalSettings.SkillForge.MaxGraphBytes
	if maxBytes <= 0 {
		maxBytes = 128 * 1024
	}
	encoded, err := json.Marshal(raw)
	if err != nil || len(encoded) > maxBytes {
		return false
	}
	maxEffects := s.deps.Config.GlobalSettings.SkillForge.MaxEffects
	if maxEffects <= 0 {
		maxEffects = 3
	}
	effects, ok := acceptedCustomBattleEffects(raw["effects"])
	if !ok || len(effects) == 0 || len(effects) > maxEffects {
		return false
	}
	for _, effect := range effects {
		kind := strings.ToLower(strings.TrimSpace(fmt.Sprintf("%v", effect["kind"])))
		switch kind {
		case "damage", "heal", "status", "resource":
		default:
			return false
		}
	}
	return true
}

func acceptedCustomBattleEffects(raw any) ([]map[string]any, bool) {
	switch effects := raw.(type) {
	case []map[string]any:
		if effects == nil {
			return nil, false
		}
		return effects, true
	case []any:
		out := make([]map[string]any, 0, len(effects))
		for _, value := range effects {
			effect, ok := value.(map[string]any)
			if !ok {
				return nil, false
			}
			out = append(out, effect)
		}
		return out, true
	default:
		return nil, false
	}
}

func (s *Server) weaponClassSetFromEquippedItems(equipped []store.EquippedItem) map[string]struct{} {
	out := map[string]struct{}{}
	for _, row := range equipped {
		if strings.TrimSpace(row.EquipType) != "battle" {
			continue
		}
		template := s.findEquipmentTemplate(strings.TrimSpace(row.TemplateItemID))
		for _, weaponClass := range weaponClassesFromTemplate(template) {
			out[weaponClass] = struct{}{}
		}
	}
	return out
}

func battleWeaponClassListFromSet(set map[string]struct{}) []string {
	out := make([]string, 0, len(set))
	for weaponClass := range set {
		if normalized := normalizeWeaponClassID(weaponClass); normalized != "" {
			out = append(out, normalized)
		}
	}
	sort.Strings(out)
	return out
}

func (s *Server) loadBattleSkillEnableContext(ctx context.Context, userID int64) (battleSkillEnableContext, error) {
	seeds, err := s.playerAttributeSeeds()
	if err != nil {
		return battleSkillEnableContext{}, err
	}
	if err := s.ensurePlayerStatsSnapshot(ctx, userID, seeds); err != nil {
		return battleSkillEnableContext{}, err
	}
	attrStates, err := s.deps.Store.ListPlayerAttributes(ctx, userID, seeds)
	if err != nil {
		return battleSkillEnableContext{}, err
	}
	attrs := make(map[string]float64, len(attrStates))
	for _, row := range attrStates {
		attrID := strings.TrimSpace(row.AttrKey)
		if attrID == "" {
			continue
		}
		attrs[attrID] = row.FinalValue
	}
	equipped, err := s.deps.Store.ListEquippedItems(ctx, userID)
	if err != nil {
		return battleSkillEnableContext{}, err
	}
	return battleSkillEnableContext{
		Attributes:    attrs,
		Equipped:      equipped,
		WeaponClasses: s.weaponClassSetFromEquippedItems(equipped),
	}, nil
}

func evaluateBattleSkillRequirement(raw map[string]any, enableCtx battleSkillEnableContext) battleSkillEnableEvaluation {
	unmetStatus := make([]battleSkillStatusRequirementGap, 0, 2)
	for attrID, required := range normalizeSkillStatusRequirement(raw) {
		current := enableCtx.Attributes[attrID]
		if current >= required {
			continue
		}
		unmetStatus = append(unmetStatus, battleSkillStatusRequirementGap{
			AttrID:   attrID,
			Required: required,
			Current:  current,
		})
	}
	sort.SliceStable(unmetStatus, func(i, j int) bool {
		return unmetStatus[i].AttrID < unmetStatus[j].AttrID
	})

	weaponRequirement := normalizeSkillWeaponRequirement(raw)
	missingWeaponClasses := make([]string, 0, len(weaponRequirement))
	if len(weaponRequirement) > 0 {
		matched := false
		for _, weaponClass := range weaponRequirement {
			if _, exists := enableCtx.WeaponClasses[weaponClass]; exists {
				matched = true
				break
			}
		}
		if !matched {
			missingWeaponClasses = append(missingWeaponClasses, weaponRequirement...)
		}
	}

	return battleSkillEnableEvaluation{
		Eligible:             len(unmetStatus) == 0 && len(missingWeaponClasses) == 0,
		UnmetStatus:          unmetStatus,
		MissingWeaponClasses: missingWeaponClasses,
	}
}

func (s *Server) battleSkillEligibilityMapFromContext(enableCtx battleSkillEnableContext) map[string]battleSkillEnableEvaluation {
	out := make(map[string]battleSkillEnableEvaluation, len(s.deps.GameData.Skills))
	for _, raw := range s.deps.GameData.Skills {
		id, _ := stringFromAny(raw["id"])
		id = strings.ToLower(strings.TrimSpace(id))
		if id == "" {
			continue
		}
		out[id] = evaluateBattleSkillRequirement(raw, enableCtx)
	}
	return out
}

func (s *Server) loadCombatConfigOrDefault(ctx context.Context, userID int64) (store.CombatConfig, error) {
	config, err := s.deps.Store.GetCombatConfig(ctx, userID)
	if err != nil {
		if !store.IsCombatConfigMissing(err) {
			return store.CombatConfig{}, err
		}
		return store.CombatConfig{SkillSlots: []string{}}, nil
	}
	return config, nil
}

func skillSlotsEqualFold(left []string, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for idx := range left {
		if !strings.EqualFold(strings.TrimSpace(left[idx]), strings.TrimSpace(right[idx])) {
			return false
		}
	}
	return true
}

func (s *Server) reconcileCombatSkillConfigForRequirements(
	ctx context.Context,
	userID int64,
	defaultBasicSkillID string,
	ownedSkillMap map[string]string,
	slotLimit int,
) ([]string, bool, error) {
	config, err := s.loadCombatConfigOrDefault(ctx, userID)
	if err != nil {
		return nil, false, err
	}
	enableCtx, err := s.loadBattleSkillEnableContext(ctx, userID)
	if err != nil {
		return nil, false, err
	}
	return s.reconcileCombatSkillConfigForRequirementsWithContext(ctx, userID, defaultBasicSkillID, ownedSkillMap, slotLimit, config, enableCtx)
}

func (s *Server) reconcileCombatSkillConfigForRequirementsWithContext(
	ctx context.Context,
	userID int64,
	defaultBasicSkillID string,
	ownedSkillMap map[string]string,
	slotLimit int,
	config store.CombatConfig,
	enableCtx battleSkillEnableContext,
) ([]string, bool, error) {
	normalized := normalizeCombatSkillSlots(config.SkillSlots, ownedSkillMap, slotLimit, defaultBasicSkillID)
	customDefinitions, err := s.deps.Store.ListAcceptedCustomSkillDefinitions(ctx, userID)
	if err != nil {
		return nil, false, fmt.Errorf("load custom skill definitions: %w", err)
	}
	filtered := make([]string, 0, len(normalized))
	for _, skillID := range normalized {
		if strings.EqualFold(skillID, defaultBasicSkillID) {
			filtered = append(filtered, skillID)
			continue
		}
		raw, ok := s.battleSkillRawByIDWithCustom(skillID, customDefinitions)
		if !ok {
			continue
		}
		result := evaluateBattleSkillRequirement(raw, enableCtx)
		if result.Eligible {
			filtered = append(filtered, skillID)
		}
	}
	if skillSlotsEqualFold(normalized, filtered) {
		return filtered, false, nil
	}
	if err := s.deps.Store.UpsertCombatConfig(ctx, userID, store.CombatConfig{SkillSlots: filtered}); err != nil {
		return nil, false, err
	}
	return filtered, true, nil
}

func (s *Server) battleSkillEligibilityMap(ctx context.Context, userID int64) (map[string]battleSkillEnableEvaluation, error) {
	enableCtx, err := s.loadBattleSkillEnableContext(ctx, userID)
	if err != nil {
		return nil, err
	}
	return s.battleSkillEligibilityMapFromContext(enableCtx), nil
}
