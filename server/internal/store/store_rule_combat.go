package store

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
)

func (s *Store) UpsertRuleSet(ctx context.Context, userID int64, ruleKind, slotKey string, enabled bool, groups any) error {
	if ruleKind != "skill" && ruleKind != "item" {
		return ErrInvalidRuleKind
	}
	slotKey = strings.TrimSpace(slotKey)
	if slotKey == "" {
		return ErrInvalidSkillID
	}
	raw, err := json.Marshal(groups)
	if err != nil {
		return err
	}
	if ruleKind == "item" {
		_, err = s.db.Exec(ctx, `
			INSERT INTO player_rule_sets(user_id, rule_kind, slot_key, enabled, groups_jsonb, updated_at)
			VALUES($1, $2, $3, $4, $5::jsonb, now())
			ON CONFLICT (user_id, rule_kind, slot_key)
			DO UPDATE SET enabled = EXCLUDED.enabled, groups_jsonb = EXCLUDED.groups_jsonb, updated_at = now()
		`, userID, ruleKind, slotKey, enabled, string(raw))
		return err
	}
	return s.withTx(ctx, func(tx pgx.Tx) error {
		if enabled {
			rows, err := tx.Query(ctx, `SELECT skill_id FROM player_skills WHERE user_id=$1 AND (enabled OR rule_enabled) AND skill_id<>$2`, userID, slotKey)
			if err != nil {
				return err
			}
			ids := []string{slotKey}
			for rows.Next() {
				var id string
				if err := rows.Scan(&id); err != nil {
					rows.Close()
					return err
				}
				ids = append(ids, id)
			}
			rows.Close()
			if err := s.validateSkillForgeSourceMutexTx(ctx, tx, userID, ids); err != nil {
				return err
			}
		}
		_, err := tx.Exec(ctx, `
			INSERT INTO player_skills(user_id,skill_id,enabled,cast_order,rule_enabled,rule_groups_jsonb,level,exp,updated_at)
			VALUES($1,$2,FALSE,0,$3,$4::jsonb,1,0,now())
			ON CONFLICT (user_id,skill_id) DO UPDATE SET rule_enabled=EXCLUDED.rule_enabled,rule_groups_jsonb=EXCLUDED.rule_groups_jsonb,updated_at=now()
		`, userID, slotKey, enabled, string(raw))
		return err
	})
}

func (s *Store) ListRuleSets(ctx context.Context, userID int64, ruleKind string) ([]RuleSet, error) {
	if ruleKind == "" || ruleKind == "item" {
		rows, err := s.db.Query(ctx, `
			SELECT rule_kind, slot_key, enabled, groups_jsonb
			FROM player_rule_sets
			WHERE user_id = $1 AND ($2 = '' OR rule_kind = $2)
			ORDER BY rule_kind, slot_key
		`, userID, ruleKind)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		out := make([]RuleSet, 0, 16)
		for rows.Next() {
			var rs RuleSet
			var raw []byte
			if err := rows.Scan(&rs.RuleKind, &rs.SlotKey, &rs.Enabled, &raw); err != nil {
				return nil, err
			}
			var groups any
			_ = json.Unmarshal(raw, &groups)
			rs.Groups = groups
			out = append(out, rs)
		}
		return out, rows.Err()
	}
	if ruleKind != "skill" {
		return nil, ErrInvalidRuleKind
	}
	rows, err := s.db.Query(ctx, `
		SELECT skill_id, rule_enabled, rule_groups_jsonb
		FROM player_skills
		WHERE user_id = $1
		  AND jsonb_typeof(rule_groups_jsonb) = 'array'
		  AND jsonb_array_length(rule_groups_jsonb) > 0
		ORDER BY skill_id
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]RuleSet, 0, 16)
	for rows.Next() {
		var (
			skillID string
			enabled bool
			raw     []byte
		)
		if err := rows.Scan(&skillID, &enabled, &raw); err != nil {
			return nil, err
		}
		var groups any
		_ = json.Unmarshal(raw, &groups)
		out = append(out, RuleSet{
			RuleKind: "skill",
			SlotKey:  strings.TrimSpace(skillID),
			Enabled:  true,
			Groups:   groups,
		})
	}
	return out, rows.Err()
}

func (s *Store) ListPlayerSkills(ctx context.Context, userID int64) ([]PlayerSkill, error) {
	rows, err := s.db.Query(ctx, `
		SELECT skill_id, enabled, cast_order, rule_enabled, rule_groups_jsonb, level, exp, updated_at, equipment_config_jsonb
		FROM player_skills
		WHERE user_id = $1
		ORDER BY cast_order ASC, skill_id ASC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]PlayerSkill, 0, 12)
	for rows.Next() {
		var (
			row       PlayerSkill
			raw       []byte
			configRaw []byte
		)
		if err := rows.Scan(
			&row.SkillID,
			&row.Enabled,
			&row.CastOrder,
			&row.RuleEnabled,
			&raw,
			&row.Level,
			&row.Exp,
			&row.UpdatedAt,
			&configRaw,
		); err != nil {
			return nil, err
		}
		var groups any
		_ = json.Unmarshal(raw, &groups)
		row.RuleGroups = groups
		if len(configRaw) > 0 {
			if err := json.Unmarshal(configRaw, &row.EquipmentConfig); err != nil {
				return nil, err
			}
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (s *Store) ListOwnedSkills(ctx context.Context, userID int64) ([]OwnedSkill, error) {
	rows, err := s.db.Query(ctx, `
		SELECT skill_id, level, exp
		FROM player_skills
		WHERE user_id = $1
		  AND skill_id NOT IN ('equipment_domination', 'equipment_martial_combo')
		ORDER BY skill_id
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]OwnedSkill, 0, 8)
	for rows.Next() {
		var skill OwnedSkill
		if err := rows.Scan(&skill.SkillID, &skill.Level, &skill.Exp); err != nil {
			return nil, err
		}
		out = append(out, skill)
	}
	return out, rows.Err()
}

func (s *Store) GetCombatConfig(ctx context.Context, userID int64) (CombatConfig, error) {
	playerSkills, err := s.ListPlayerSkills(ctx, userID)
	if err != nil {
		return CombatConfig{}, err
	}
	if len(playerSkills) == 0 {
		return CombatConfig{}, pgx.ErrNoRows
	}
	cfg := CombatConfig{
		SkillSlots: []string{},
		UpdatedAt:  playerSkills[0].UpdatedAt,
	}
	for _, row := range playerSkills {
		if row.UpdatedAt.After(cfg.UpdatedAt) {
			cfg.UpdatedAt = row.UpdatedAt
		}
	}
	enabled := make([]PlayerSkill, 0, len(playerSkills))
	for _, row := range playerSkills {
		if IsEquipmentSkill(row.SkillID) {
			if row.EquipmentConfig != nil {
				if cfg.EquipmentSkills == nil {
					cfg.EquipmentSkills = map[string]EquipmentSkillConfig{}
				}
				cfg.EquipmentSkills[row.SkillID] = *row.EquipmentConfig
			}
			continue
		}
		if !row.Enabled {
			continue
		}
		// Include basic_attack so frontend can display and reorder it.
		enabled = append(enabled, row)
	}
	sort.SliceStable(enabled, func(i, j int) bool {
		if enabled[i].CastOrder == enabled[j].CastOrder {
			return enabled[i].SkillID < enabled[j].SkillID
		}
		return enabled[i].CastOrder < enabled[j].CastOrder
	})
	cfg.SkillSlots = make([]string, 0, len(enabled))
	for _, row := range enabled {
		cfg.SkillSlots = append(cfg.SkillSlots, strings.TrimSpace(row.SkillID))
	}
	return cfg, nil
}

func (s *Store) UpsertCombatConfig(ctx context.Context, userID int64, cfg CombatConfig) error {
	skillSlots := sanitizeSkillIDs(cfg.SkillSlots)
	filteredSlots := make([]string, 0, len(skillSlots))
	for _, skillID := range skillSlots {
		if IsEquipmentSkill(skillID) {
			return ErrInvalidSkillID
		}
		// Keep basic_attack in the list so its cast_order is preserved.
		// Users can drag basic_attack to set priority; stripping it here
		// caused battle_runtime_engine to always append it at the end.
		filteredSlots = append(filteredSlots, skillID)
	}
	return s.withTx(ctx, func(tx pgx.Tx) error {
		if err := s.lockUserTx(ctx, tx, userID); err != nil {
			return err
		}
		mutexIDs := append([]string(nil), filteredSlots...)
		rows, err := tx.Query(ctx, `SELECT skill_id FROM player_skills WHERE user_id=$1 AND rule_enabled`, userID)
		if err != nil {
			return err
		}
		for rows.Next() {
			var skillID string
			if err := rows.Scan(&skillID); err != nil {
				rows.Close()
				return err
			}
			mutexIDs = append(mutexIDs, skillID)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return err
		}
		rows.Close()
		if err := s.validateSkillForgeSourceMutexTx(ctx, tx, userID, sanitizeSkillIDs(mutexIDs)); err != nil {
			return err
		}
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
	})
}

func (s *Store) GrantOwnedSkill(ctx context.Context, userID int64, skillID string) error {
	skillID = strings.TrimSpace(skillID)
	if skillID == "" || IsEquipmentSkill(skillID) {
		return ErrInvalidSkillID
	}
	return s.withTx(ctx, func(tx pgx.Tx) error {
		_, err := s.grantOwnedSkillTx(ctx, tx, userID, skillID)
		return err
	})
}

func (s *Store) RevokeOwnedSkill(ctx context.Context, userID int64, skillID string) error {
	skillID = strings.TrimSpace(skillID)
	if skillID == "" {
		return ErrInvalidSkillID
	}
	if strings.EqualFold(skillID, s.defaultBattleBasicSkillID()) {
		return ErrBasicSkillLocked
	}
	return s.withTx(ctx, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, `
			SELECT 1
			FROM player_skills
			WHERE user_id = $1 AND skill_id = $2
		`, userID, skillID).Scan(new(int))
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `
			DELETE FROM player_skills
			WHERE user_id = $1 AND skill_id = $2
		`, userID, skillID)
		return err
	})
}

func (s *Store) grantOwnedSkillTx(ctx context.Context, tx pgx.Tx, userID int64, skillID string) (bool, error) {
	if IsEquipmentSkill(skillID) {
		return false, ErrInvalidSkillID
	}
	tag, err := tx.Exec(ctx, `
		INSERT INTO player_skills(
			user_id, skill_id, enabled, cast_order,
			rule_enabled, rule_groups_jsonb, level, exp, updated_at
		)
		VALUES($1, $2, FALSE, 0, FALSE, '[]'::jsonb, 1, 0, now())
		ON CONFLICT (user_id, skill_id) DO NOTHING
	`, userID, skillID)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

func (s *Store) skillLevelUpRequirement(level int) int64 {
	if level < 1 {
		level = 1
	}
	base := s.config.SkillLevelUpBaseExp
	step := s.config.SkillLevelUpPerLevelExtraExp
	required := base + int64(level-1)*step
	if required <= 0 {
		return base
	}
	return required
}

func (s *Store) addSkillExpTx(ctx context.Context, tx pgx.Tx, userID int64, skillID string, gained int64) (int, int64, error) {
	if gained < 0 {
		gained = 0
	}
	if _, err := s.grantOwnedSkillTx(ctx, tx, userID, skillID); err != nil {
		return 0, 0, err
	}
	var level int
	var exp int64
	if err := tx.QueryRow(ctx, `
		SELECT level, exp
		FROM player_skills
		WHERE user_id = $1 AND skill_id = $2
		FOR UPDATE
	`, userID, skillID).Scan(&level, &exp); err != nil {
		return 0, 0, err
	}
	nextExp, err := checkedSkillExpTotal(exp, gained)
	if err != nil {
		return 0, 0, err
	}
	exp = nextExp
	for {
		required := s.skillLevelUpRequirement(level)
		if exp < required {
			break
		}
		exp -= required
		level++
	}
	if _, err := tx.Exec(ctx, `
		UPDATE player_skills
		SET level = $3, exp = $4, updated_at = now()
		WHERE user_id = $1 AND skill_id = $2
	`, userID, skillID, level, exp); err != nil {
		return 0, 0, err
	}
	return level, exp, nil
}

func (s *Store) addKittySkillExpTx(ctx context.Context, tx pgx.Tx, kittyID int64, skillID string, gained int64) (bool, int, int64, error) {
	skillID = strings.TrimSpace(skillID)
	if kittyID <= 0 || skillID == "" {
		return false, 0, 0, ErrInvalidSkillID
	}
	if gained < 0 {
		gained = 0
	}
	tag, err := tx.Exec(ctx, `
		INSERT INTO player_kitty_skills(kitty_id, skill_id, enabled, cast_order, rule_enabled, rule_groups_jsonb, level, exp, updated_at)
		VALUES($1, $2, FALSE, 0, FALSE, '[]'::jsonb, 1, 0, now())
		ON CONFLICT (kitty_id, skill_id) DO NOTHING
	`, kittyID, skillID)
	if err != nil {
		return false, 0, 0, err
	}
	gainedSkill := tag.RowsAffected() > 0
	var level int
	var exp int64
	if err := tx.QueryRow(ctx, `
		SELECT level, exp
		FROM player_kitty_skills
		WHERE kitty_id = $1 AND skill_id = $2
		FOR UPDATE
	`, kittyID, skillID).Scan(&level, &exp); err != nil {
		return false, 0, 0, err
	}
	exp, err = checkedSkillExpTotal(exp, gained)
	if err != nil {
		return false, 0, 0, err
	}
	for {
		required := s.skillLevelUpRequirement(level)
		if exp < required {
			break
		}
		exp -= required
		level++
	}
	if _, err := tx.Exec(ctx, `
		UPDATE player_kitty_skills
		SET level = $3, exp = $4, updated_at = now()
		WHERE kitty_id = $1 AND skill_id = $2
	`, kittyID, skillID, level, exp); err != nil {
		return false, 0, 0, err
	}
	return gainedSkill, level, exp, nil
}

func checkedSkillExpTotal(current, gained int64) (int64, error) {
	if current < 0 || gained < 0 || gained > math.MaxInt64-current {
		return 0, ErrSkillExpOverflow
	}
	return current + gained, nil
}

func (s *Store) UseSkillbook(ctx context.Context, userID, instanceID int64, skillID string, skillExpGain int64, count int64) (SkillbookUseResult, error) {
	skillID = strings.TrimSpace(skillID)
	if skillID == "" {
		return SkillbookUseResult{}, ErrInvalidSkillID
	}
	if count < 1 {
		count = 1
	}
	var (
		gainedSkill bool
		gainedExp   int64
		level       int
		exp         int64
	)
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		row, err := s.LockInstanceTx(ctx, tx, instanceID)
		if err != nil {
			return err
		}
		if row.UserID != userID {
			return ErrInstanceNotOwned
		}
		if row.State != "inventory" {
			return ErrInvalidInstanceState
		}
		if err = s.DecrementOrDeleteTx(ctx, tx, instanceID, userID, count); err != nil {
			return err
		}
		gainedSkill, err = s.grantOwnedSkillTx(ctx, tx, userID, skillID)
		if err != nil {
			return err
		}
		// First use grants the skill (0 exp). Each additional use adds skillExpGain.
		// If skill was already owned, all count uses add exp.
		if gainedSkill {
			gainedExp = skillExpGain * (count - 1)
		} else {
			gainedExp = skillExpGain * count
		}
		level, exp, err = s.addSkillExpTx(ctx, tx, userID, skillID, gainedExp)
		return err
	})
	if err != nil {
		return SkillbookUseResult{}, err
	}
	return SkillbookUseResult{
		SkillID:     skillID,
		GainedSkill: gainedSkill,
		GainedExp:   gainedExp,
		Level:       level,
		Exp:         exp,
	}, nil
}

func (s *Store) UseKittySkillbook(ctx context.Context, userID, kittyID, instanceID int64, skillID string, skillExpGain int64, count int64) (SkillbookUseResult, error) {
	skillID = strings.TrimSpace(skillID)
	if skillID == "" {
		return SkillbookUseResult{}, ErrInvalidSkillID
	}
	if kittyID <= 0 {
		return SkillbookUseResult{}, ErrKittyNotFound
	}
	if count < 1 {
		count = 1
	}
	var (
		gainedSkill bool
		gainedExp   int64
		level       int
		exp         int64
	)
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		if _, err := s.getOwnedKittyStatusTx(ctx, tx, userID, kittyID, true); err != nil {
			return err
		}
		row, err := s.LockInstanceTx(ctx, tx, instanceID)
		if err != nil {
			return err
		}
		if row.UserID != userID {
			return ErrInstanceNotOwned
		}
		if row.State != "inventory" {
			return ErrInvalidInstanceState
		}
		if err = s.DecrementOrDeleteTx(ctx, tx, instanceID, userID, count); err != nil {
			return err
		}
		// Grant skill with 0 exp first to detect if newly acquired.
		gainedSkill, level, exp, err = s.addKittySkillExpTx(ctx, tx, kittyID, skillID, 0)
		if err != nil {
			return err
		}
		if gainedSkill {
			gainedExp = skillExpGain * (count - 1)
		} else {
			gainedExp = skillExpGain * count
		}
		if gainedExp > 0 {
			_, level, exp, err = s.addKittySkillExpTx(ctx, tx, kittyID, skillID, gainedExp)
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return SkillbookUseResult{}, err
	}
	return SkillbookUseResult{
		SkillID:     skillID,
		GainedSkill: gainedSkill,
		GainedExp:   gainedExp,
		Level:       level,
		Exp:         exp,
	}, nil
}

func sanitizeSkillIDs(input []string) []string {
	out := make([]string, 0, len(input))
	seen := make(map[string]struct{}, len(input))
	for _, row := range input {
		id := strings.TrimSpace(row)
		if id == "" {
			continue
		}
		key := strings.ToLower(id)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, id)
	}
	return out
}

func (s *Store) defaultBattleBasicSkillID() string {
	for _, raw := range s.gameData.CombatConfigRules {
		value, _ := raw["default_basic_skill_id"].(string)
		id := strings.TrimSpace(value)
		if id != "" {
			return id
		}
	}
	for _, raw := range s.gameData.Skills {
		value, _ := raw["id"].(string)
		id := strings.TrimSpace(value)
		if id != "" {
			return id
		}
	}
	return ""
}

func IsCombatConfigMissing(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}
