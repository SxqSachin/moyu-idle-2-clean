package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"mysecondwebgame/server/internal/rules"
)

const EquipmentDominationSkill = "equipment_domination"
const EquipmentMartialComboSkill = "equipment_martial_combo"

var ErrEquipmentSkillConfig = errors.New("equipment skill configuration invalid")
var ErrEquipmentSkillRevision = errors.New("equipment skill configuration changed")

// Configuration is carried by the existing skill read. No combat-time query is needed.
type EquipmentSkillConfig struct {
	Version         int                    `json:"version"`
	Revision        int64                  `json:"revision"`
	Enabled         bool                   `json:"enabled"`
	SkillIDs        []string               `json:"skillIds"`
	AllowStandalone bool                   `json:"allowStandalone"`
	BeforeSkillID   string                 `json:"beforeSkillId,omitempty"`
	Groups          []rules.ConditionGroup `json:"groups,omitempty"`
}

func IsEquipmentSkill(id string) bool {
	return id == EquipmentDominationSkill || id == EquipmentMartialComboSkill
}

func ValidateEquipmentSkillConfig(id string, cfg EquipmentSkillConfig) error {
	if !IsEquipmentSkill(id) || cfg.Version != 1 || cfg.Revision < 0 || cfg.Revision >= 9007199254740991 || len(cfg.SkillIDs) > 4 || len(cfg.Groups) > 6 || len(cfg.BeforeSkillID) > 160 {
		return ErrEquipmentSkillConfig
	}
	seen := make(map[string]bool, len(cfg.SkillIDs))
	for _, skillID := range cfg.SkillIDs {
		if skillID == "" || skillID != strings.TrimSpace(skillID) || len(skillID) > 160 || seen[skillID] || IsEquipmentSkill(skillID) || skillID == "basic_attack" {
			return ErrEquipmentSkillConfig
		}
		seen[skillID] = true
	}
	if id == EquipmentDominationSkill && (len(cfg.SkillIDs) > 3 || len(cfg.Groups) > 0 || cfg.AllowStandalone || cfg.BeforeSkillID != "") {
		return ErrEquipmentSkillConfig
	}
	for _, group := range cfg.Groups {
		if len(group.Conditions) == 0 || len(group.Conditions) > 6 {
			return ErrEquipmentSkillConfig
		}
		for _, condition := range group.Conditions {
			if len(condition.CooldownSkillIDs) > 16 || len(condition.CooldownItemIDs) > 16 {
				return ErrEquipmentSkillConfig
			}
		}
	}
	if cfg.Enabled && (len(cfg.SkillIDs) == 0 || (id == EquipmentMartialComboSkill && len(cfg.SkillIDs) < 2)) {
		return ErrEquipmentSkillConfig
	}
	raw, err := json.Marshal(cfg)
	if err != nil || len(raw) > 16384 {
		return ErrEquipmentSkillConfig
	}
	return nil
}

// Eligibility uses effect data, not translated names or weapon-school labels.
func EquipmentComboEligible(raw map[string]any) bool {
	if raw == nil || raw["disabled"] == true || raw["equipmentOnly"] == true || raw["passive"] == true || raw["enemyOnly"] == true {
		return false
	}
	if raw["id"] == "basic_attack" {
		return false
	}
	physical := false
	effects, ok := raw["effects"].([]any)
	if !ok {
		return false
	}
	for _, value := range effects {
		effect, ok := value.(map[string]any)
		if !ok {
			return false
		}
		if effect["kind"] != "damage" {
			continue
		}
		damageType, _ := effect["damageType"].(string)
		if spec, ok := effect["damageSpec"].(map[string]any); ok {
			if value, ok := spec["damageType"].(string); ok {
				damageType = value
			}
		}
		if damageType != "physical" {
			return false
		}
		physical = true
	}
	return physical
}

func EquipmentDominationEligible(raw map[string]any) bool {
	id, _ := raw["id"].(string)
	return id != "" && id != "basic_attack" && !strings.HasPrefix(id, "custom:") && !IsEquipmentSkill(id) && raw["disabled"] != true && raw["passive"] != true && raw["uninterruptible"] != true && raw["equipmentOnly"] != true
}

func (s *Store) SaveEquipmentSkillConfig(ctx context.Context, userID int64, id string, cfg EquipmentSkillConfig) (EquipmentSkillConfig, error) {
	if err := ValidateEquipmentSkillConfig(id, cfg); err != nil {
		return cfg, err
	}
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		if err := s.lockUserTx(ctx, tx, userID); err != nil {
			return err
		}
		// The same user lock serializes equipment, enhancement and skill changes.
		rows, err := tx.Query(ctx, `SELECT DISTINCT i.template_item_id,i.enhance_level
			FROM player_equip_slots e JOIN player_item_instances i ON i.instance_id=e.instance_id AND i.user_id=e.user_id
			WHERE e.user_id=$1 AND e.equip_type='battle' AND i.state='equipped'`, userID)
		if err != nil {
			return err
		}
		capacity := 0
		for rows.Next() {
			var templateID string
			var enhance int
			if err := rows.Scan(&templateID, &enhance); err != nil {
				rows.Close()
				return err
			}
			template := s.gameData.EquipmentTemplateByID(templateID)
			if template != nil && template.GrantedSkill != nil && template.GrantedSkill.SkillID == id {
				if value := template.GrantedSkill.Capacity(enhance); value > capacity {
					capacity = value
				}
			}
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		if capacity == 0 || len(cfg.SkillIDs) > capacity {
			return fmt.Errorf("%w: EQUIPMENT_OR_CAPACITY", ErrEquipmentSkillConfig)
		}
		definitions := make(map[string]map[string]any, len(s.gameData.Skills))
		for _, raw := range s.gameData.Skills {
			if key, ok := raw["id"].(string); ok {
				definitions[key] = raw
			}
		}
		if id == EquipmentMartialComboSkill {
			rows, err := tx.Query(ctx, `SELECT ps.skill_id,ps.enabled,r.compiled_skill_jsonb,r.ruleset_version
				FROM player_skills ps
				LEFT JOIN player_custom_skills cs ON cs.id=ps.skill_id AND cs.user_id=ps.user_id AND cs.archived_at IS NULL
				LEFT JOIN player_custom_skill_revisions r ON r.id=cs.current_revision_id AND r.result_status='accepted'
				WHERE ps.user_id=$1 AND ps.skill_id=ANY($2::text[])`, userID, cfg.SkillIDs)
			if err != nil {
				return err
			}
			owned := make(map[string]bool, len(cfg.SkillIDs))
			for rows.Next() {
				var key string
				var enabled bool
				var raw []byte
				var version *int
				if err := rows.Scan(&key, &enabled, &raw, &version); err != nil {
					rows.Close()
					return err
				}
				owned[key] = enabled
				if strings.HasPrefix(key, "custom:") {
					if len(raw) == 0 || version == nil || !s.config.SkillForge.RulesetEnabled(*version) {
						owned[key] = false
						continue
					}
					var definition map[string]any
					if err := json.Unmarshal(raw, &definition); err != nil {
						rows.Close()
						return err
					}
					definitions[key] = definition
				}
			}
			err = rows.Err()
			rows.Close()
			if err != nil {
				return err
			}
			for _, key := range cfg.SkillIDs {
				if !owned[key] || !EquipmentComboEligible(definitions[key]) {
					return fmt.Errorf("%w: SKILL_NOT_CARRIED_OR_PHYSICAL", ErrEquipmentSkillConfig)
				}
			}
			if err := s.validateSkillForgeSourceMutexTx(ctx, tx, userID, cfg.SkillIDs); err != nil {
				return err
			}
		} else {
			for _, key := range cfg.SkillIDs {
				if !EquipmentDominationEligible(definitions[key]) {
					return fmt.Errorf("%w: SKILL_NOT_INTERRUPTIBLE", ErrEquipmentSkillConfig)
				}
			}
		}
		return saveEquipmentSkillConfigTx(ctx, tx, userID, id, &cfg, true)
	})
	return cfg, err
}

func saveEquipmentSkillConfigTx(ctx context.Context, tx pgx.Tx, userID int64, id string, cfg *EquipmentSkillConfig, compare bool) error {
	expected := cfg.Revision
	cfg.Revision++
	raw, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	var saved []byte
	err = tx.QueryRow(ctx, `INSERT INTO player_skills(user_id,skill_id,equipment_config_jsonb)
		SELECT $1,$2,$3::jsonb WHERE NOT $5 OR $4=0 OR EXISTS(SELECT 1 FROM player_skills WHERE user_id=$1 AND skill_id=$2)
		ON CONFLICT(user_id,skill_id) DO UPDATE SET equipment_config_jsonb=jsonb_set(EXCLUDED.equipment_config_jsonb,'{revision}',to_jsonb(COALESCE((player_skills.equipment_config_jsonb->>'revision')::bigint,0)+1)),updated_at=now()
		WHERE NOT $5 OR COALESCE((player_skills.equipment_config_jsonb->>'revision')::bigint,0)=$4
		RETURNING equipment_config_jsonb`, userID, id, raw, expected, compare).Scan(&saved)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrEquipmentSkillRevision
	}
	if err != nil {
		return err
	}
	return json.Unmarshal(saved, cfg)
}
