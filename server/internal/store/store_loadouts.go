package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"mysecondwebgame/server/internal/catalog"

	"github.com/jackc/pgx/v5"
)

const (
	WorkLoadoutFallbackNotBound = "NOT_BOUND"

	workLoadoutFallbackNotFound          = "LOADOUT_NOT_FOUND"
	workLoadoutFallbackItemNotOwned      = "ITEM_NOT_OWNED"
	workLoadoutFallbackItemMissing       = "ITEM_MISSING"
	workLoadoutFallbackItemStateInvalid  = "ITEM_STATE_INVALID"
	workLoadoutFallbackEquipTypeMismatch = "EQUIP_TYPE_MISMATCH"
	workLoadoutFallbackSlotNotAllowed    = "SLOT_NOT_ALLOWED"
	workLoadoutFallbackStatusRequirement = "STATUS_REQUIREMENT_NOT_MET"
	workLoadoutFallbackAccessoryConflict = "ACCESSORY_CONFLICT"
	workLoadoutFallbackAffixBodyLimit    = "AFFIX_BODY_LIMIT"
)

var (
	ErrLoadoutNotFound = errors.New("loadout not found")
)

type LoadoutValidationError struct {
	Reason string
	ItemID string
}

func (e *LoadoutValidationError) Error() string {
	if e.ItemID != "" {
		return e.Reason + ": " + e.ItemID
	}
	return e.Reason
}

type loadoutQueryer interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

type loadoutRowPayload struct {
	EquipmentSkills map[string]EquipmentSkillConfig `json:"equipmentSkills"`
	Equipment       []LoadoutEquipmentSlot          `json:"equipment"`
	SkillSlots      []string                        `json:"skillSlots,omitempty"`
	SkillRules      []LoadoutSkillRule              `json:"skillRules,omitempty"`
}

type battleLoadoutSkillEvaluation struct {
	SkillID string
	Reason  string
}

func normalizeLoadoutEquipment(rows []LoadoutEquipmentSlot) []LoadoutEquipmentSlot {
	seen := map[string]int{}
	out := make([]LoadoutEquipmentSlot, 0, len(rows))
	for _, row := range rows {
		slot := strings.TrimSpace(row.Slot)
		if slot == "" || row.InstanceID <= 0 {
			continue
		}
		key := fmt.Sprintf("%s:%d", slot, row.InstanceID)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = len(out)
		out = append(out, LoadoutEquipmentSlot{Slot: slot, InstanceID: row.InstanceID})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].InstanceID != out[j].InstanceID {
			return out[i].InstanceID < out[j].InstanceID
		}
		return out[i].Slot < out[j].Slot
	})
	return out
}

func normalizeLoadoutSkillRules(rows []LoadoutSkillRule) []LoadoutSkillRule {
	out := make([]LoadoutSkillRule, 0, len(rows))
	seen := map[string]struct{}{}
	for _, row := range rows {
		skillID := strings.TrimSpace(row.SkillID)
		if skillID == "" {
			continue
		}
		key := strings.ToLower(skillID)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, LoadoutSkillRule{
			SkillID: skillID,
			Enabled: row.Enabled,
			Groups:  row.Groups,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].SkillID < out[j].SkillID })
	return out
}

func normalizeBattleLoadout(row BattleLoadout) BattleLoadout {
	row.ID = strings.TrimSpace(row.ID)
	row.Name = strings.TrimSpace(row.Name)
	row.Equipment = normalizeLoadoutEquipment(row.Equipment)
	row.SkillRules = normalizeLoadoutSkillRules(row.SkillRules)
	row.SkillSlots = sanitizeSkillIDs(row.SkillSlots)
	return row
}

func loadoutToFloat64(value any, fallback float64) float64 {
	switch typed := value.(type) {
	case float64:
		return typed
	case float32:
		return float64(typed)
	case int:
		return float64(typed)
	case int64:
		return float64(typed)
	case int32:
		return float64(typed)
	case json.Number:
		if out, err := typed.Float64(); err == nil {
			return out
		}
	}
	return fallback
}

func normalizeWeaponClassID(raw string) string {
	return strings.ToLower(strings.TrimSpace(raw))
}

func normalizeSkillWeaponRequirement(raw map[string]any) []string {
	value, ok := raw["weaponRequirement"]
	if !ok {
		return nil
	}
	var candidates []string
	switch typed := value.(type) {
	case []any:
		for _, row := range typed {
			if text, ok := row.(string); ok {
				candidates = append(candidates, text)
			}
		}
	case []string:
		candidates = append(candidates, typed...)
	}
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

func normalizeSkillStatusRequirement(raw map[string]any) map[string]float64 {
	out := map[string]float64{}
	value, ok := raw["statusRequirement"]
	if !ok {
		return out
	}
	switch typed := value.(type) {
	case map[string]any:
		for key, val := range typed {
			attrID := strings.TrimSpace(key)
			required := loadoutToFloat64(val, 0)
			if attrID == "" || required <= 0 {
				continue
			}
			out[attrID] = required
		}
	}
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

func (s *Store) battleSkillRawByID(skillID string) (map[string]any, bool) {
	normalized := strings.ToLower(strings.TrimSpace(skillID))
	if normalized == "" {
		return nil, false
	}
	for _, raw := range s.gameData.Skills {
		id, _ := raw["id"].(string)
		if strings.ToLower(strings.TrimSpace(id)) == normalized {
			return raw, true
		}
	}
	return nil, false
}

func (s *Store) evaluateBattleLoadoutSkillSetTx(ctx context.Context, tx pgx.Tx, userID int64, loadout BattleLoadout) ([]string, []LoadoutSkillRule, []BattleLoadoutSkippedSkill, error) {
	items, valErr, err := s.validateLoadoutEquipmentTx(ctx, tx, userID, "battle", loadout.Equipment)
	if err != nil {
		return nil, nil, nil, err
	}
	if valErr != nil {
		return nil, nil, nil, valErr
	}
	attrTotals, _, err := s.computePlayerAttributeTotalsTx(ctx, tx, userID, items)
	if err != nil {
		return nil, nil, nil, err
	}
	weaponClasses := map[string]struct{}{}
	for _, item := range items {
		template := s.gameData.EquipmentTemplateByID(strings.TrimSpace(item.TemplateItemID))
		for _, weaponClass := range weaponClassesFromTemplate(template) {
			weaponClasses[weaponClass] = struct{}{}
		}
	}
	skipped := make([]BattleLoadoutSkippedSkill, 0)
	filteredSlots := make([]string, 0, len(loadout.SkillSlots))
	enabledSet := map[string]struct{}{}
	custom, err := s.loadoutCustomSkillDefinitionsTx(ctx, tx, userID, loadout.SkillSlots)
	if err != nil {
		return nil, nil, nil, err
	}
	for _, skillID := range sanitizeSkillIDs(loadout.SkillSlots) {
		if IsEquipmentSkill(skillID) {
			continue
		}
		// Don't strip basic_attack - let it pass through normal validation.
		// If it exists in catalog and meets requirements (it has none), it gets included.
		raw, ok := s.battleSkillRawByID(skillID)
		if strings.HasPrefix(skillID, "custom:") {
			raw, ok = custom[skillID]
			if !ok {
				skipped = append(skipped, BattleLoadoutSkippedSkill{SkillID: skillID, Reason: "SKILL_UNAVAILABLE"})
			}
		}
		if !ok {
			continue
		}
		skipReason := ""
		for attrID, required := range normalizeSkillStatusRequirement(raw) {
			if attrTotals[attrID] < required {
				skipReason = "STATUS_REQUIREMENT_NOT_MET"
				break
			}
		}
		if skipReason == "" {
			weaponRequirement := normalizeSkillWeaponRequirement(raw)
			if len(weaponRequirement) > 0 {
				matched := false
				for _, weaponClass := range weaponRequirement {
					if _, ok := weaponClasses[weaponClass]; ok {
						matched = true
						break
					}
				}
				if !matched {
					skipReason = "WEAPON_REQUIREMENT_NOT_MET"
				}
			}
		}
		if skipReason != "" {
			skipped = append(skipped, BattleLoadoutSkippedSkill{
				SkillID: skillID,
				Reason:  skipReason,
			})
			continue
		}
		filteredSlots = append(filteredSlots, skillID)
		enabledSet[strings.ToLower(strings.TrimSpace(skillID))] = struct{}{}
	}
	if len(custom) > 0 {
		seenSources := map[string]string{}
		for _, id := range filteredSlots {
			root := id
			if forge, ok := custom[id]["customForge"].(map[string]any); ok {
				if source := stringValue(forge["sourceSkillId"]); source != "" {
					root = source
				}
			}
			if previous, ok := seenSources[root]; ok && previous != id {
				return nil, nil, nil, ErrSkillForgeSameSource
			}
			seenSources[root] = id
		}
	}
	filteredRules := make([]LoadoutSkillRule, 0, len(loadout.SkillRules))
	for _, row := range normalizeLoadoutSkillRules(loadout.SkillRules) {
		if _, ok := enabledSet[strings.ToLower(strings.TrimSpace(row.SkillID))]; !ok {
			continue
		}
		filteredRules = append(filteredRules, row)
	}
	return filteredSlots, filteredRules, skipped, nil
}

// Resolve private combo components in one bounded read while applying a loadout,
// never in battle turns. A learned kitty skill is not player ownership.
func (s *Store) loadoutCustomSkillDefinitionsTx(ctx context.Context, tx pgx.Tx, userID int64, ids []string) (map[string]map[string]any, error) {
	var customIDs []string
	for _, id := range sanitizeSkillIDs(ids) {
		if strings.HasPrefix(id, "custom:") {
			customIDs = append(customIDs, id)
		}
	}
	if len(customIDs) == 0 {
		return nil, nil
	}
	rows, err := tx.Query(ctx, `SELECT cs.id,r.compiled_skill_jsonb,r.ruleset_version FROM player_custom_skills cs
		JOIN player_custom_skill_revisions r ON r.id=cs.current_revision_id AND r.result_status='accepted'
		JOIN player_skills ps ON ps.user_id=cs.user_id AND ps.skill_id=cs.id
		WHERE cs.user_id=$1 AND cs.id=ANY($2::text[]) AND cs.archived_at IS NULL`, userID, customIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]map[string]any, len(customIDs))
	for rows.Next() {
		var id string
		var raw []byte
		var version int
		if err := rows.Scan(&id, &raw, &version); err != nil {
			return nil, err
		}
		if !s.config.SkillForge.RulesetEnabled(version) {
			continue
		}
		var definition map[string]any
		if err := json.Unmarshal(raw, &definition); err != nil {
			return nil, err
		}
		out[id] = definition
	}
	return out, rows.Err()
}

func normalizeWorkLoadout(row WorkLoadout) WorkLoadout {
	row.ID = strings.TrimSpace(row.ID)
	row.Name = strings.TrimSpace(row.Name)
	row.Equipment = normalizeLoadoutEquipment(row.Equipment)
	return row
}

func normalizePlayerLoadouts(raw PlayerLoadouts) PlayerLoadouts {
	out := PlayerLoadouts{
		BattleLoadouts:     make([]BattleLoadout, 0, len(raw.BattleLoadouts)),
		WorkLoadouts:       make([]WorkLoadout, 0, len(raw.WorkLoadouts)),
		WorkActionBindings: map[string]string{},
	}
	seenBattle := map[string]struct{}{}
	for _, row := range raw.BattleLoadouts {
		row = normalizeBattleLoadout(row)
		if row.ID == "" {
			continue
		}
		key := strings.ToLower(row.ID)
		if _, ok := seenBattle[key]; ok {
			continue
		}
		seenBattle[key] = struct{}{}
		out.BattleLoadouts = append(out.BattleLoadouts, row)
	}
	seenWork := map[string]struct{}{}
	for _, row := range raw.WorkLoadouts {
		row = normalizeWorkLoadout(row)
		if row.ID == "" {
			continue
		}
		key := strings.ToLower(row.ID)
		if _, ok := seenWork[key]; ok {
			continue
		}
		seenWork[key] = struct{}{}
		out.WorkLoadouts = append(out.WorkLoadouts, row)
	}
	for rawMainType, rawLoadoutID := range raw.WorkActionBindings {
		mainType := strings.TrimSpace(rawMainType)
		loadoutID := strings.TrimSpace(rawLoadoutID)
		if mainType == "" || loadoutID == "" {
			continue
		}
		out.WorkActionBindings[mainType] = loadoutID
	}
	sort.Slice(out.BattleLoadouts, func(i, j int) bool { return out.BattleLoadouts[i].ID < out.BattleLoadouts[j].ID })
	sort.Slice(out.WorkLoadouts, func(i, j int) bool { return out.WorkLoadouts[i].ID < out.WorkLoadouts[j].ID })
	return out
}

func marshalLoadoutPayload(loadoutType string, battle BattleLoadout, work WorkLoadout) ([]byte, error) {
	switch strings.TrimSpace(loadoutType) {
	case "battle":
		return json.Marshal(loadoutRowPayload{
			EquipmentSkills: battle.EquipmentSkills,
			Equipment:       battle.Equipment,
			SkillSlots:      battle.SkillSlots,
			SkillRules:      battle.SkillRules,
		})
	case "work":
		return json.Marshal(loadoutRowPayload{
			Equipment: work.Equipment,
		})
	default:
		return nil, ErrLoadoutNotFound
	}
}

func decodeBattleLoadout(rowID string, name string, payload []byte) BattleLoadout {
	var raw loadoutRowPayload
	_ = json.Unmarshal(payload, &raw)
	return normalizeBattleLoadout(BattleLoadout{
		EquipmentSkills: raw.EquipmentSkills,
		ID:              rowID,
		Name:            name,
		Equipment:       raw.Equipment,
		SkillSlots:      raw.SkillSlots,
		SkillRules:      raw.SkillRules,
	})
}

func decodeWorkLoadout(rowID string, name string, payload []byte) WorkLoadout {
	var raw loadoutRowPayload
	_ = json.Unmarshal(payload, &raw)
	return normalizeWorkLoadout(WorkLoadout{
		ID:        rowID,
		Name:      name,
		Equipment: raw.Equipment,
	})
}

func (s *Store) getPlayerLoadoutsFrom(ctx context.Context, db loadoutQueryer, userID int64) (PlayerLoadouts, error) {
	out := PlayerLoadouts{
		BattleLoadouts:     []BattleLoadout{},
		WorkLoadouts:       []WorkLoadout{},
		WorkActionBindings: map[string]string{},
	}
	rows, err := db.Query(ctx, `
		SELECT loadout_id, type, name, payload_jsonb
		FROM player_loadouts
		WHERE user_id = $1
		ORDER BY type ASC, updated_at DESC, loadout_id ASC
	`, userID)
	if err != nil {
		return PlayerLoadouts{}, err
	}
	for rows.Next() {
		var loadoutID string
		var loadoutType string
		var name string
		var payload []byte
		if err := rows.Scan(&loadoutID, &loadoutType, &name, &payload); err != nil {
			rows.Close()
			return PlayerLoadouts{}, err
		}
		switch strings.TrimSpace(loadoutType) {
		case "battle":
			out.BattleLoadouts = append(out.BattleLoadouts, decodeBattleLoadout(loadoutID, name, payload))
		case "work":
			out.WorkLoadouts = append(out.WorkLoadouts, decodeWorkLoadout(loadoutID, name, payload))
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return PlayerLoadouts{}, err
	}
	rows.Close()
	bindingRows, err := db.Query(ctx, `
		SELECT action_main_type, loadout_id
		FROM player_work_loadout_bindings
		WHERE user_id = $1
		ORDER BY action_main_type ASC
	`, userID)
	if err != nil {
		return PlayerLoadouts{}, err
	}
	for bindingRows.Next() {
		var mainType string
		var loadoutID string
		if err := bindingRows.Scan(&mainType, &loadoutID); err != nil {
			bindingRows.Close()
			return PlayerLoadouts{}, err
		}
		mainType = strings.TrimSpace(mainType)
		loadoutID = strings.TrimSpace(loadoutID)
		if mainType == "" || loadoutID == "" {
			continue
		}
		out.WorkActionBindings[mainType] = loadoutID
	}
	if err := bindingRows.Err(); err != nil {
		bindingRows.Close()
		return PlayerLoadouts{}, err
	}
	bindingRows.Close()
	return normalizePlayerLoadouts(out), nil
}

func (s *Store) GetPlayerLoadouts(ctx context.Context, userID int64) (PlayerLoadouts, error) {
	return s.getPlayerLoadoutsFrom(ctx, s.db, userID)
}

func (s *Store) upsertLoadoutTx(ctx context.Context, tx pgx.Tx, userID int64, loadoutType string, loadoutID string, name string, payload []byte) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO player_loadouts(user_id, loadout_id, type, name, payload_jsonb, created_at, updated_at)
		VALUES($1, $2, $3, $4, $5::jsonb, now(), now())
		ON CONFLICT (user_id, loadout_id)
		DO UPDATE SET
			type = EXCLUDED.type,
			name = EXCLUDED.name,
			payload_jsonb = EXCLUDED.payload_jsonb,
			updated_at = now()
	`, userID, loadoutID, loadoutType, name, string(payload))
	return err
}

func (s *Store) UpsertBattleLoadout(ctx context.Context, userID int64, loadout BattleLoadout) (PlayerLoadouts, error) {
	if len(loadout.EquipmentSkills) > 2 {
		return PlayerLoadouts{}, ErrEquipmentSkillConfig
	}
	for id, cfg := range loadout.EquipmentSkills {
		if err := ValidateEquipmentSkillConfig(id, cfg); err != nil {
			return PlayerLoadouts{}, err
		}
	}
	loadout = normalizeBattleLoadout(loadout)
	if loadout.ID == "" {
		return PlayerLoadouts{}, ErrInvalidSkillID
	}
	var out PlayerLoadouts
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		payload, err := marshalLoadoutPayload("battle", loadout, WorkLoadout{})
		if err != nil {
			return err
		}
		if err := s.upsertLoadoutTx(ctx, tx, userID, "battle", loadout.ID, loadout.Name, payload); err != nil {
			return err
		}
		out, err = s.getPlayerLoadoutsFrom(ctx, tx, userID)
		return err
	})
	return out, err
}

func (s *Store) UpsertWorkLoadout(ctx context.Context, userID int64, loadout WorkLoadout) (PlayerLoadouts, error) {
	loadout = normalizeWorkLoadout(loadout)
	if loadout.ID == "" {
		return PlayerLoadouts{}, ErrInvalidSkillID
	}
	var out PlayerLoadouts
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		payload, err := marshalLoadoutPayload("work", BattleLoadout{}, loadout)
		if err != nil {
			return err
		}
		if err := s.upsertLoadoutTx(ctx, tx, userID, "work", loadout.ID, loadout.Name, payload); err != nil {
			return err
		}
		out, err = s.getPlayerLoadoutsFrom(ctx, tx, userID)
		return err
	})
	return out, err
}

func (s *Store) DeletePlayerLoadout(ctx context.Context, userID int64, loadoutType string, loadoutID string) (PlayerLoadouts, error) {
	loadoutType = strings.TrimSpace(strings.ToLower(loadoutType))
	loadoutID = strings.TrimSpace(loadoutID)
	if loadoutID == "" {
		return PlayerLoadouts{}, ErrLoadoutNotFound
	}
	var out PlayerLoadouts
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `
			DELETE FROM player_loadouts
			WHERE user_id = $1 AND loadout_id = $2 AND type = $3
		`, userID, loadoutID, loadoutType)
		if err != nil {
			return err
		}
		if tag.RowsAffected() <= 0 {
			return ErrLoadoutNotFound
		}
		if loadoutType == "work" {
			if _, err := tx.Exec(ctx, `
				DELETE FROM player_work_loadout_bindings
				WHERE user_id = $1 AND loadout_id = $2
			`, userID, loadoutID); err != nil {
				return err
			}
		}
		out, err = s.getPlayerLoadoutsFrom(ctx, tx, userID)
		return err
	})
	return out, err
}

func (s *Store) UpdateWorkActionBindings(ctx context.Context, userID int64, bindings map[string]string) (PlayerLoadouts, error) {
	var out PlayerLoadouts
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		workIDs := map[string]struct{}{}
		rows, err := tx.Query(ctx, `
			SELECT loadout_id
			FROM player_loadouts
			WHERE user_id = $1 AND type = 'work'
		`, userID)
		if err != nil {
			return err
		}
		for rows.Next() {
			var loadoutID string
			if err := rows.Scan(&loadoutID); err != nil {
				rows.Close()
				return err
			}
			workIDs[strings.ToLower(strings.TrimSpace(loadoutID))] = struct{}{}
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return err
		}
		rows.Close()
		if _, err := tx.Exec(ctx, `DELETE FROM player_work_loadout_bindings WHERE user_id = $1`, userID); err != nil {
			return err
		}
		for actionMainType, loadoutID := range bindings {
			actionMainType = strings.TrimSpace(actionMainType)
			loadoutID = strings.TrimSpace(loadoutID)
			if actionMainType == "" || loadoutID == "" {
				continue
			}
			if _, ok := workIDs[strings.ToLower(loadoutID)]; !ok {
				continue
			}
			if _, err := tx.Exec(ctx, `
				INSERT INTO player_work_loadout_bindings(user_id, action_main_type, loadout_id, updated_at)
				VALUES($1, $2, $3, now())
			`, userID, actionMainType, loadoutID); err != nil {
				return err
			}
		}
		out, err = s.getPlayerLoadoutsFrom(ctx, tx, userID)
		return err
	})
	return out, err
}

func (s *Store) findBattleLoadout(loadouts PlayerLoadouts, loadoutID string) (BattleLoadout, bool) {
	for _, row := range loadouts.BattleLoadouts {
		if strings.EqualFold(row.ID, strings.TrimSpace(loadoutID)) {
			return row, true
		}
	}
	return BattleLoadout{}, false
}

func (s *Store) findWorkLoadout(loadouts PlayerLoadouts, loadoutID string) (WorkLoadout, bool) {
	for _, row := range loadouts.WorkLoadouts {
		if strings.EqualFold(row.ID, strings.TrimSpace(loadoutID)) {
			return row, true
		}
	}
	return WorkLoadout{}, false
}

type groupedLoadoutEquipment struct {
	InstanceID int64
	Slots      []string
}

func groupLoadoutEquipment(rows []LoadoutEquipmentSlot) []groupedLoadoutEquipment {
	grouped := map[int64]map[string]struct{}{}
	for _, row := range rows {
		if row.InstanceID <= 0 {
			continue
		}
		slot := strings.TrimSpace(row.Slot)
		if slot == "" {
			continue
		}
		if _, ok := grouped[row.InstanceID]; !ok {
			grouped[row.InstanceID] = map[string]struct{}{}
		}
		grouped[row.InstanceID][slot] = struct{}{}
	}
	out := make([]groupedLoadoutEquipment, 0, len(grouped))
	for instanceID, slotsMap := range grouped {
		slots := make([]string, 0, len(slotsMap))
		for slot := range slotsMap {
			slots = append(slots, slot)
		}
		sort.Strings(slots)
		out = append(out, groupedLoadoutEquipment{InstanceID: instanceID, Slots: slots})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].InstanceID < out[j].InstanceID })
	return out
}

func groupedLoadoutEquipmentEqual(a []groupedLoadoutEquipment, b []groupedLoadoutEquipment) bool {
	if len(a) != len(b) {
		return false
	}
	for idx := range a {
		if a[idx].InstanceID != b[idx].InstanceID {
			return false
		}
		if len(a[idx].Slots) != len(b[idx].Slots) {
			return false
		}
		for slotIdx := range a[idx].Slots {
			if a[idx].Slots[slotIdx] != b[idx].Slots[slotIdx] {
				return false
			}
		}
	}
	return true
}

type virtualEquipmentItem struct {
	InstanceID     int64
	TemplateItemID string
	EquipType      string
	EnhanceLevel   int
	Affixes        []EquipmentAffixSlot
}

func loadoutAccessoryTemplateIDs(templateItemID string, slot string) []string {
	if strings.TrimSpace(slot) != "accessory" {
		return nil
	}
	if strings.TrimSpace(templateItemID) == "" {
		return nil
	}
	return []string{strings.TrimSpace(templateItemID)}
}

func (s *Store) validateLoadoutEquipmentTx(
	ctx context.Context,
	tx pgx.Tx,
	userID int64,
	equipType string,
	rows []LoadoutEquipmentSlot,
) ([]virtualEquipmentItem, *LoadoutValidationError, error) {
	battleProfile, err := s.resolvePlayerBattleSlotProfileTx(ctx, tx, userID)
	if err != nil {
		return nil, nil, err
	}
	grouped := groupLoadoutEquipment(rows)
	out := make([]virtualEquipmentItem, 0, len(grouped))
	accessoryTemplates := map[string]int{}
	bodyAffixSeen := map[string]int{}
	for _, row := range grouped {
		locked, err := s.LockInstanceTx(ctx, tx, row.InstanceID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, &LoadoutValidationError{Reason: workLoadoutFallbackItemMissing}, nil
			}
			return nil, nil, err
		}
		itemID := strings.TrimSpace(locked.TemplateItem)
		if locked.UserID != userID {
			return nil, &LoadoutValidationError{Reason: workLoadoutFallbackItemNotOwned, ItemID: itemID}, nil
		}
		if locked.State != "inventory" && locked.State != "equipped" {
			return nil, &LoadoutValidationError{Reason: workLoadoutFallbackItemStateInvalid, ItemID: itemID}, nil
		}
		template := s.gameData.EquipmentTemplateByID(itemID)
		if template == nil {
			return nil, &LoadoutValidationError{Reason: workLoadoutFallbackItemMissing, ItemID: itemID}, nil
		}
		if normalizeEquipType(strings.TrimSpace(template.EquipType)) != equipType {
			return nil, &LoadoutValidationError{Reason: workLoadoutFallbackEquipTypeMismatch, ItemID: itemID}, nil
		}
		if len(row.Slots) == 0 {
			return nil, &LoadoutValidationError{Reason: workLoadoutFallbackSlotNotAllowed, ItemID: itemID}, nil
		}
		for _, slot := range row.Slots {
			if !equipmentTemplateAllowsEquipSlotWithProfile(template, equipType, slot, battleProfile) {
				return nil, &LoadoutValidationError{Reason: workLoadoutFallbackSlotNotAllowed, ItemID: itemID}, nil
			}
			if slot == "right_hand_2" && len(template.OccupySlots) > 0 {
				return nil, &LoadoutValidationError{Reason: workLoadoutFallbackSlotNotAllowed, ItemID: itemID}, nil
			}
		}
		for attrKey, required := range template.StatusRequirement {
			var finalValue float64
			err := tx.QueryRow(ctx, `
				SELECT final_value
				FROM player_attributes
				WHERE user_id = $1 AND attr_key = $2
			`, userID, attrKey).Scan(&finalValue)
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, &LoadoutValidationError{Reason: workLoadoutFallbackStatusRequirement, ItemID: itemID}, nil
			}
			if err != nil {
				return nil, nil, err
			}
			if finalValue < float64(required) {
				return nil, &LoadoutValidationError{Reason: workLoadoutFallbackStatusRequirement, ItemID: itemID}, nil
			}
		}
		if strings.TrimSpace(template.Slot) == "accessory" {
			conflictKeys := []string{"template:" + itemID}
			if group := strings.TrimSpace(template.AccessoryGroup); group != "" {
				conflictKeys = append(conflictKeys, "group:"+group)
			}
			for _, conflictKey := range conflictKeys {
				accessoryTemplates[conflictKey]++
			}
			if accessoryTemplates["template:"+itemID] > 1 || (strings.TrimSpace(template.AccessoryGroup) != "" && accessoryTemplates["group:"+strings.TrimSpace(template.AccessoryGroup)] > 1) {
				return nil, &LoadoutValidationError{Reason: workLoadoutFallbackAccessoryConflict, ItemID: itemID}, nil
			}
		}
		affixes := decodeAffixSlots(locked.AffixesRaw)
		_ = FilterEffectiveAffixSlotsByLimit(affixes, s.gameData, bodyAffixSeen)
		for affixID, count := range bodyAffixSeen {
			def := s.gameData.AffixDefinitionByID(affixID)
			if limit := AffixMaxEffectivePerBody(def); limit > 0 && count > limit {
				return nil, &LoadoutValidationError{Reason: workLoadoutFallbackAffixBodyLimit, ItemID: itemID}, nil
			}
		}
		out = append(out, virtualEquipmentItem{
			InstanceID:     row.InstanceID,
			TemplateItemID: itemID,
			EquipType:      equipType,
			EnhanceLevel:   locked.EnhanceLevel,
			Affixes:        affixes,
		})
	}
	return out, nil, nil
}

func (s *Store) replaceEquipTypeTx(ctx context.Context, tx pgx.Tx, userID int64, equipType string, rows []LoadoutEquipmentSlot) error {
	if err := s.lockUserTx(ctx, tx, userID); err != nil {
		return err
	}
	if _, err := s.cleanupPlayerEquipSlotsTx(ctx, tx, userID); err != nil {
		return err
	}
	currentRows, err := tx.Query(ctx, `
		SELECT instance_id
		FROM player_equip_slots
		WHERE user_id = $1 AND equip_type = $2
		FOR UPDATE
	`, userID, equipType)
	if err != nil {
		return err
	}
	currentInstanceSet := map[int64]struct{}{}
	currentInstanceIDs := make([]int64, 0, 8)
	for currentRows.Next() {
		var instanceID int64
		if err := currentRows.Scan(&instanceID); err != nil {
			currentRows.Close()
			return err
		}
		if _, ok := currentInstanceSet[instanceID]; ok {
			continue
		}
		currentInstanceSet[instanceID] = struct{}{}
		currentInstanceIDs = append(currentInstanceIDs, instanceID)
	}
	if err := currentRows.Err(); err != nil {
		currentRows.Close()
		return err
	}
	currentRows.Close()
	if _, err := tx.Exec(ctx, `
		DELETE FROM player_equip_slots
		WHERE user_id = $1 AND equip_type = $2
	`, userID, equipType); err != nil {
		return err
	}
	for _, instanceID := range currentInstanceIDs {
		if err := s.setInstanceInventoryWhenFullyUnequippedTx(ctx, tx, userID, instanceID); err != nil {
			return err
		}
	}
	for _, row := range groupLoadoutEquipment(rows) {
		locked, err := s.LockInstanceTx(ctx, tx, row.InstanceID)
		if err != nil {
			return err
		}
		template := s.gameData.EquipmentTemplateByID(strings.TrimSpace(locked.TemplateItem))
		if template == nil {
			return ErrInvalidItemID
		}
		if err := s.EquipTx(ctx, tx, userID, row.InstanceID, equipType, row.Slots, loadoutAccessoryTemplateIDs(strings.TrimSpace(locked.TemplateItem), strings.TrimSpace(template.Slot))); err != nil {
			return err
		}
	}
	return s.onPlayerAttributesChangedTx(ctx, tx, userID)
}

func (s *Store) currentEquipTypeMatchesLoadoutTx(ctx context.Context, tx pgx.Tx, userID int64, equipType string, rows []LoadoutEquipmentSlot) (bool, error) {
	currentRows, err := tx.Query(ctx, `
		SELECT instance_id, slot
		FROM player_equip_slots
		WHERE user_id = $1 AND equip_type = $2
		FOR UPDATE
	`, userID, equipType)
	if err != nil {
		return false, err
	}
	current := []LoadoutEquipmentSlot{}
	for currentRows.Next() {
		var row LoadoutEquipmentSlot
		if err := currentRows.Scan(&row.InstanceID, &row.Slot); err != nil {
			currentRows.Close()
			return false, err
		}
		current = append(current, row)
	}
	if err := currentRows.Err(); err != nil {
		currentRows.Close()
		return false, err
	}
	currentRows.Close()
	return groupedLoadoutEquipmentEqual(groupLoadoutEquipment(current), groupLoadoutEquipment(rows)), nil
}

func (s *Store) applyWorkLoadoutTx(ctx context.Context, tx pgx.Tx, userID int64, loadoutID string) (bool, error) {
	if err := s.lockUserTx(ctx, tx, userID); err != nil {
		return false, err
	}
	loadouts, err := s.getPlayerLoadoutsFrom(ctx, tx, userID)
	if err != nil {
		return false, err
	}
	loadout, ok := s.findWorkLoadout(loadouts, loadoutID)
	if !ok {
		return false, ErrLoadoutNotFound
	}
	if _, valErr, err := s.validateLoadoutEquipmentTx(ctx, tx, userID, "work", loadout.Equipment); err != nil {
		return false, err
	} else if valErr != nil {
		return false, valErr
	}
	matches, err := s.currentEquipTypeMatchesLoadoutTx(ctx, tx, userID, "work", loadout.Equipment)
	if err != nil {
		return false, err
	}
	if matches {
		return false, nil
	}
	if err := s.replaceEquipTypeTx(ctx, tx, userID, "work", loadout.Equipment); err != nil {
		return false, err
	}
	return true, nil
}

func (s *Store) ApplyWorkLoadoutTx(ctx context.Context, tx pgx.Tx, userID int64, loadoutID string) error {
	_, err := s.applyWorkLoadoutTx(ctx, tx, userID, loadoutID)
	return err
}

func (s *Store) ApplyWorkLoadout(ctx context.Context, userID int64, loadoutID string) error {
	return s.withTx(ctx, func(tx pgx.Tx) error {
		return s.ApplyWorkLoadoutTx(ctx, tx, userID, loadoutID)
	})
}

func (s *Store) ResolveWorkLoadoutRuntime(ctx context.Context, tx pgx.Tx, userID int64, actionID string, mainTypeHint string) (WorkLoadoutRuntimeResolution, error) {
	loadouts, err := s.getPlayerLoadoutsFrom(ctx, tx, userID)
	if err != nil {
		return WorkLoadoutRuntimeResolution{}, err
	}
	normalizedActionID := strings.TrimSpace(actionID)
	loadoutID := ""
	for _, def := range s.gameData.ActionDefinitions() {
		if strings.TrimSpace(def.ID) != normalizedActionID {
			continue
		}
		mainType := strings.TrimSpace(def.MainType)
		if mainType == "" {
			mainType = strings.TrimSpace(def.Menu)
		}
		if mainType != "" {
			loadoutID = strings.TrimSpace(loadouts.WorkActionBindings[mainType])
		}
		break
	}
	if loadoutID == "" {
		if hint := strings.TrimSpace(mainTypeHint); hint != "" {
			loadoutID = strings.TrimSpace(loadouts.WorkActionBindings[hint])
		}
	}
	if loadoutID == "" {
		return WorkLoadoutRuntimeResolution{FallbackReason: WorkLoadoutFallbackNotBound}, nil
	}
	loadout, ok := s.findWorkLoadout(loadouts, loadoutID)
	if !ok {
		return WorkLoadoutRuntimeResolution{LoadoutID: loadoutID, FallbackReason: workLoadoutFallbackNotFound}, nil
	}
	items, valErr, err := s.validateLoadoutEquipmentTx(ctx, tx, userID, "work", loadout.Equipment)
	if err != nil {
		return WorkLoadoutRuntimeResolution{}, err
	}
	if valErr != nil {
		return WorkLoadoutRuntimeResolution{
			LoadoutID:         loadout.ID,
			FallbackReason:    valErr.Reason,
			EquipmentSnapshot: append([]LoadoutEquipmentSlot(nil), loadout.Equipment...),
		}, nil
	}
	attrTotals, attrDeltas, err := s.computePlayerAttributeTotalsTx(ctx, tx, userID, items)
	if err != nil {
		return WorkLoadoutRuntimeResolution{}, err
	}
	return WorkLoadoutRuntimeResolution{
		LoadoutID:         loadout.ID,
		Available:         true,
		AttributeTotals:   attrTotals,
		AttributeDeltas:   attrDeltas,
		EquipmentSnapshot: append([]LoadoutEquipmentSlot(nil), loadout.Equipment...),
	}, nil
}

func (s *Store) applyCombatConfigTx(ctx context.Context, tx pgx.Tx, userID int64, cfg CombatConfig) error {
	skillSlots := sanitizeSkillIDs(cfg.SkillSlots)
	filteredSlots := skillSlots
	for _, skillID := range filteredSlots {
		if _, err := tx.Exec(ctx, `
			INSERT INTO player_skills(
				user_id, skill_id, enabled, cast_order,
				rule_enabled, rule_groups_jsonb, level, exp, updated_at
			)
			VALUES($1, $2, FALSE, 0, FALSE, '[]'::jsonb, 1, 0, now())
			ON CONFLICT (user_id, skill_id)
			DO UPDATE SET updated_at = now()
		`, userID, skillID); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(ctx, `
		UPDATE player_skills
		SET enabled = FALSE, cast_order = 0, updated_at = now()
		WHERE user_id = $1
	`, userID); err != nil {
		return err
	}
	if len(filteredSlots) > 0 {
		if _, err := tx.Exec(ctx, `
			WITH ranked AS (
				SELECT skill_id, (ordinality - 1)::INT AS cast_order
				FROM unnest($2::text[]) WITH ORDINALITY AS t(skill_id, ordinality)
			)
			UPDATE player_skills ps
			SET enabled = TRUE, cast_order = ranked.cast_order, updated_at = now()
			FROM ranked
			WHERE ps.user_id = $1 AND ps.skill_id = ranked.skill_id
		`, userID, filteredSlots); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) applyLoadoutSkillRulesTx(ctx context.Context, tx pgx.Tx, userID int64, rows []LoadoutSkillRule) error {
	for _, row := range normalizeLoadoutSkillRules(rows) {
		raw, err := json.Marshal(row.Groups)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO player_skills(
				user_id, skill_id, enabled, cast_order,
				rule_enabled, rule_groups_jsonb, level, exp, updated_at
			)
			VALUES($1, $2, FALSE, 0, $3, $4::jsonb, 1, 0, now())
			ON CONFLICT (user_id, skill_id)
			DO UPDATE SET
				rule_enabled = EXCLUDED.rule_enabled,
				rule_groups_jsonb = EXCLUDED.rule_groups_jsonb,
				updated_at = now()
		`, userID, row.SkillID, row.Enabled, string(raw)); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) resetPlayerSkillRulesTx(ctx context.Context, tx pgx.Tx, userID int64) error {
	_, err := tx.Exec(ctx, `
		UPDATE player_skills
		SET rule_enabled = FALSE,
		    rule_groups_jsonb = '[]'::jsonb,
		    updated_at = now()
		WHERE user_id = $1
	`, userID)
	return err
}

func (s *Store) ApplyBattleLoadout(ctx context.Context, userID int64, loadoutID string) (BattleLoadoutApplyResult, error) {
	var result BattleLoadoutApplyResult
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		// Specialization allocation/reset takes the same user lock before touching
		// equipment rows. Keep one global lock order so the validated topology
		// cannot change before this transaction replaces the loadout.
		if err := s.lockUserTx(ctx, tx, userID); err != nil {
			return err
		}
		loadouts, err := s.getPlayerLoadoutsFrom(ctx, tx, userID)
		if err != nil {
			return err
		}
		loadout, ok := s.findBattleLoadout(loadouts, loadoutID)
		if !ok {
			return ErrLoadoutNotFound
		}
		if _, valErr, err := s.validateLoadoutEquipmentTx(ctx, tx, userID, "battle", loadout.Equipment); err != nil {
			return err
		} else if valErr != nil {
			return valErr
		}
		if err := s.replaceEquipTypeTx(ctx, tx, userID, "battle", loadout.Equipment); err != nil {
			return err
		}
		filteredSkillSlots, filteredSkillRules, skippedSkills, err := s.evaluateBattleLoadoutSkillSetTx(ctx, tx, userID, loadout)
		if err != nil {
			return err
		}
		if err := s.applyCombatConfigTx(ctx, tx, userID, CombatConfig{SkillSlots: filteredSkillSlots}); err != nil {
			return err
		}
		if loadout.EquipmentSkills != nil {
			for _, id := range []string{EquipmentDominationSkill, EquipmentMartialComboSkill} {
				cfg, ok := loadout.EquipmentSkills[id]
				if !ok {
					cfg = EquipmentSkillConfig{Version: 1, SkillIDs: []string{}}
				}
				if err := ValidateEquipmentSkillConfig(id, cfg); err != nil {
					return err
				}
				if err := saveEquipmentSkillConfigTx(ctx, tx, userID, id, &cfg, false); err != nil {
					return err
				}
			}
		}
		if err := s.resetPlayerSkillRulesTx(ctx, tx, userID); err != nil {
			return err
		}
		if err := s.applyLoadoutSkillRulesTx(ctx, tx, userID, filteredSkillRules); err != nil {
			return err
		}
		result.SkippedSkills = skippedSkills
		return nil
	})
	return result, err
}
