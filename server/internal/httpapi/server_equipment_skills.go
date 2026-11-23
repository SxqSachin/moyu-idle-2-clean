package httpapi

import (
	"errors"
	"github.com/gofiber/fiber/v2"
	"mysecondwebgame/server/internal/store"
)

func (s *Server) equipmentSkillPreview(ctx battleSkillEnableContext, cfg store.CombatConfig, owned []store.OwnedSkill, custom map[string]map[string]any) []equipmentSkillView {
	views := s.equipmentSkillViews(ctx.Equipped, cfg.EquipmentSkills)
	if len(views) == 0 {
		return nil
	}
	levels := make(map[string]store.OwnedSkill, len(owned))
	for _, row := range owned {
		levels[row.SkillID] = row
	}
	skills := make([]battleSkillState, 0, len(cfg.SkillSlots))
	for _, id := range cfg.SkillSlots {
		raw, ok := s.battleSkillRawByIDWithCustom(id, custom)
		if !ok || !evaluateBattleSkillRequirement(raw, ctx).Eligible {
			continue
		}
		progress := levels[id]
		skills = append(skills, s.battleSkillByRawWithProgress(id, progress.Level, progress.Exp, raw))
	}
	for i := range views {
		views[i].Reason = s.equipmentSkillConfigReason(views[i], skills, custom)
	}
	return views
}

func (s *Server) handleEquipmentSkillConfigUpdate(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", "")
	}
	var req struct {
		SkillID string                     `json:"skillId"`
		Config  store.EquipmentSkillConfig `json:"config"`
	}
	if len(c.Body()) > 18000 || c.BodyParser(&req) != nil || store.ValidateEquipmentSkillConfig(req.SkillID, req.Config) != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "EQUIPMENT_SKILL_CONFIG_INVALID", "invalid equipment skill configuration", "")
	}
	saved, err := s.deps.Store.SaveEquipmentSkillConfig(c.Context(), userID, req.SkillID, req.Config)
	if errors.Is(err, store.ErrEquipmentSkillRevision) {
		return s.writeAPIError(c, fiber.StatusConflict, "EQUIPMENT_SKILL_CONFIG_CHANGED", "configuration changed; reload before saving", "")
	}
	if errors.Is(err, store.ErrEquipmentSkillConfig) || errors.Is(err, store.ErrSkillForgeSameSource) {
		return s.writeAPIError(c, fiber.StatusConflict, "EQUIPMENT_SKILL_CONFIG_INVALID", "check equipment, capacity and carried skills", "")
	}
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "EQUIPMENT_SKILL_CONFIG_SAVE_FAILED", "save equipment skill configuration failed", err.Error())
	}
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{"config": saved})
}
