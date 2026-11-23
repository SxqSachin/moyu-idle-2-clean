package httpapi

import (
	"time"

	"mysecondwebgame/server/internal/store"
)

type battleComboPlan struct {
	Count    int       `json:"count"`
	Indexes  [4]int    `json:"indexes"`
	SkillIDs [4]string `json:"skillIds"`
}

type equipmentSkillView struct {
	SkillID      string                     `json:"skillId"`
	ItemID       string                     `json:"itemId"`
	EnhanceLevel int                        `json:"enhanceLevel"`
	Capacity     int                        `json:"capacity"`
	Config       store.EquipmentSkillConfig `json:"config"`
	Reason       string                     `json:"reason,omitempty"`
}

func (s *Server) equipmentSkillViews(equipped []store.EquippedItem, configs map[string]store.EquipmentSkillConfig) []equipmentSkillView {
	var out []equipmentSkillView
	for _, item := range equipped {
		if item.EquipType != "battle" {
			continue
		}
		if item.TemplateItemID != "domination_seal" && item.TemplateItemID != "hundred_battle_badge" {
			continue
		}
		template := s.deps.GameData.EquipmentTemplateByID(item.TemplateItemID)
		if template == nil || template.GrantedSkill == nil {
			continue
		}
		grant := template.GrantedSkill
		if !store.IsEquipmentSkill(grant.SkillID) {
			continue
		}
		cfg, ok := configs[grant.SkillID]
		if !ok {
			cfg = store.EquipmentSkillConfig{Version: 1, SkillIDs: []string{}}
		}
		view := equipmentSkillView{SkillID: grant.SkillID, ItemID: item.TemplateItemID, EnhanceLevel: item.EnhanceLevel, Capacity: grant.Capacity(item.EnhanceLevel), Config: cfg}
		duplicate := -1
		for i := range out {
			if out[i].SkillID == view.SkillID {
				duplicate = i
				break
			}
		}
		if duplicate < 0 {
			out = append(out, view)
		} else if view.Capacity > out[duplicate].Capacity {
			out[duplicate] = view
		}
	}
	return out
}

func (s *Server) equipmentSkillConfigReason(view equipmentSkillView, skills []battleSkillState, custom map[string]map[string]any) string {
	cfg := view.Config
	if err := store.ValidateEquipmentSkillConfig(view.SkillID, cfg); err != nil {
		return "INVALID_CONFIG"
	}
	if len(cfg.SkillIDs) > view.Capacity {
		return "CAPACITY_EXCEEDED"
	}
	if len(cfg.SkillIDs) == 0 {
		return "NOT_CONFIGURED"
	}
	if view.SkillID == store.EquipmentDominationSkill {
		for _, id := range cfg.SkillIDs {
			raw, ok := s.battleSkillRawByIDWithCustom(id, nil)
			if !ok || !store.EquipmentDominationEligible(raw) {
				return "SKILL_NOT_INTERRUPTIBLE"
			}
		}
		return ""
	}
	if len(cfg.SkillIDs) < 2 {
		return "NOT_CONFIGURED"
	}
	for _, id := range cfg.SkillIDs {
		raw, ok := s.battleSkillRawByIDWithCustom(id, custom)
		if !ok || !store.EquipmentComboEligible(raw) {
			return "SKILL_NOT_CARRIED_OR_PHYSICAL"
		}
		found := false
		for _, skill := range skills {
			if skill.ID == id && s.battleSkillRulesetEnabled(skill) {
				found = true
				break
			}
		}
		if !found {
			return "SKILL_NOT_CARRIED_OR_PHYSICAL"
		}
	}
	return ""
}

func (s *Server) applyEquipmentSkillSnapshot(unit *battleUnitState, equipped []store.EquippedItem, rows []store.PlayerSkill, custom map[string]map[string]any) {
	configs := equipmentSkillConfigsFromRows(rows)
	for _, view := range s.equipmentSkillViews(equipped, configs) {
		if !view.Config.Enabled || s.equipmentSkillConfigReason(view, unit.Skills, custom) != "" {
			continue
		}
		if view.SkillID == store.EquipmentDominationSkill {
			unit.DominationSkills = append([]string(nil), view.Config.SkillIDs...)
			continue
		}
		combo := s.battleSkillByID(view.SkillID)
		combo.Combo = &battleComboPlan{Count: len(view.Config.SkillIDs)}
		insert := 0
		if view.Config.BeforeSkillID != "" {
			// A removed priority anchor falls back to first, never behind basic attack.
			for i := range unit.Skills {
				if unit.Skills[i].ID == view.Config.BeforeSkillID {
					insert = i
					break
				}
			}
		}
		unit.Skills = append(unit.Skills, battleSkillState{})
		copy(unit.Skills[insert+1:], unit.Skills[insert:])
		unit.Skills[insert] = combo
		for n, id := range view.Config.SkillIDs {
			for i := range unit.Skills {
				if unit.Skills[i].ID != id {
					continue
				}
				combo.Combo.Indexes[n], combo.Combo.SkillIDs[n] = i, id
				unit.Skills[i].ComboOnly = !view.Config.AllowStandalone
				break
			}
		}
		unit.SkillIDs = append(unit.SkillIDs, combo.ID)
		if len(view.Config.Groups) > 0 {
			if unit.SkillRules == nil {
				unit.SkillRules = map[string]battleSkillRuleState{}
			}
			unit.SkillRules[combo.ID] = battleSkillRuleState{Enabled: true, Groups: view.Config.Groups}
		}
	}
}

// Indices are resolved once from the frozen snapshot; no skill scans or JSON in a turn.
func battleComboChild(unit *battleUnitState, plan *battleComboPlan, n int) (battleSkillState, int, bool) {
	if plan == nil || n < 0 || n >= plan.Count || plan.Count < 2 || plan.Count > 4 {
		return battleSkillState{}, -1, false
	}
	i := plan.Indexes[n]
	if i < 0 || i >= len(unit.Skills) || unit.Skills[i].ID != plan.SkillIDs[n] || unit.Skills[i].Combo != nil || store.IsEquipmentSkill(unit.Skills[i].ID) {
		return battleSkillState{}, -1, false
	}
	return unit.Skills[i], i, true
}

func (s *Server) battleComboReady(state *battleRuntimeState, unit *battleUnitState, plan *battleComboPlan) bool {
	if plan == nil || plan.Count < 2 || plan.Count > 4 {
		return false
	}
	hp, mp := 0.0, 0.0
	for n := 0; n < plan.Count; n++ {
		child, _, ok := battleComboChild(unit, plan, n)
		if !ok || !s.battleSkillRulesetEnabled(child) || child.CooldownRemain > 0 {
			return false
		}
		child = unit.applyBattleSkillScalarMutations("skill", child)
		if !s.battleSkillCastGuardsPassed(state, unit, child) {
			return false
		}
		hp += child.HPCost
		mp += child.MPCost
	}
	return unit.HP > hp && unit.MP >= mp
}

func indexBattleDominationUnit(state *battleRuntimeState, unit *battleUnitState) {
	if len(unit.DominationSkills) == 0 {
		return
	}
	if state.dominationBySkill == nil {
		state.dominationBySkill = map[string][]string{}
	}
	for _, id := range unit.DominationSkills {
		state.dominationBySkill[id] = append(state.dominationBySkill[id], unit.UnitID)
	}
}

func (s *Server) interceptBattleSkill(state *battleRuntimeState, actor *battleUnitState, skill battleSkillState) (battleSkillExecution, bool) {
	if skill.Uninterruptible || skill.ID == "basic_attack" || store.IsEquipmentSkill(skill.ID) {
		return battleSkillExecution{}, false
	}
	for _, id := range state.dominationBySkill[skill.ID] {
		owner := state.unitIndexByID[id]
		// Interception follows current allegiance, not panic/camouflage targeting remaps.
		if owner == nil || !owner.Alive || owner.HP <= 0 || owner.UnitID == actor.UnitID || battleUnitPerspectiveSide(owner) == battleUnitPerspectiveSide(actor) {
			continue
		}
		hpCost, mpCost := maxFloat(1, float64(int64(owner.MaxHP*.15))), maxFloat(1, float64(int64(owner.MaxMP*.15)))
		if owner.HP <= hpCost || owner.MP < mpCost {
			continue
		}
		beforeHP, beforeMP := owner.HP, owner.MP
		owner.HP -= hpCost
		owner.MP -= mpCost
		return battleSkillExecution{
			Targets: []map[string]any{}, Effects: []map[string]any{},
			Actions: []map[string]any{
				{"kind": "resource", "sourceId": owner.UnitID, "targetId": owner.UnitID, "skillId": store.EquipmentDominationSkill, "resource": "hp", "amount": -hpCost, "beforeHp": beforeHP, "afterHp": owner.HP, "afterAlive": true},
				{"kind": "resource", "sourceId": owner.UnitID, "targetId": owner.UnitID, "skillId": store.EquipmentDominationSkill, "resource": "mp", "amount": -mpCost, "beforeMp": beforeMP, "afterMp": owner.MP},
				{"kind": "trigger", "sourceId": owner.UnitID, "targetId": actor.UnitID, "skillId": store.EquipmentDominationSkill, "meta": map[string]any{"interruptedSkillId": skill.ID, "interruptedActorId": actor.UnitID}},
			},
			Meta: map[string]any{"interrupted": true, "reason": "equipment_domination"},
		}, true
	}
	return battleSkillExecution{}, false
}

func (s *Server) executeEquipmentCombo(state *battleRuntimeState, now time.Time, actor *battleUnitState, combo battleSkillState) battleSkillExecution {
	out := battleSkillExecution{Targets: []map[string]any{}, Effects: []map[string]any{}, Actions: []map[string]any{}, Meta: map[string]any{"equipmentCombo": true}}
	if state.executingEquipmentCombo || combo.Combo == nil {
		return out
	}
	state.executingEquipmentCombo = true
	defer func() { state.executingEquipmentCombo = false }()
	actorID := actor.UnitID
	for n := 0; n < combo.Combo.Count; n++ {
		actor = battleUnitByID(state, actorID)
		if actor == nil || !actor.Alive || actor.HP <= 0 {
			break
		}
		actionSuppressed, skillSuppressed, _ := s.battleUnitCheckTurnSuppressions(actor)
		if actionSuppressed || skillSuppressed {
			break
		}
		child, idx, ok := battleComboChild(actor, combo.Combo, n)
		if !ok || !s.battleSkillRulesetEnabled(child) || child.CooldownRemain > 0 {
			break
		}
		child = actor.applyBattleSkillScalarMutations("skill", child)
		if actor.HP <= child.HPCost || actor.MP < child.MPCost || !s.battleSkillCastGuardsPassed(state, actor, child) {
			break
		}
		context := &battleEquipmentHookContext{SkillID: child.ID, SkillCategory: child.Category, CastKind: "combo_child", MPCost: child.MPCost, HPCost: child.HPCost}
		out.Actions = append(out.Actions, s.battleTriggerUnitEquipmentHooksWithContext(state, now, actor, nil, nil, "on_before_skill_cast", 0, "", context)...)
		actor = battleUnitByID(state, actorID)
		if actor == nil || !actor.Alive || actor.HP <= child.HPCost || actor.MP < child.MPCost {
			break
		}
		actionSuppressed, skillSuppressed, _ = s.battleUnitCheckTurnSuppressions(actor)
		if actionSuppressed || skillSuppressed || !s.battleSkillCastGuardsPassed(state, actor, child) {
			break
		}
		child.CooldownTurns = maxInt(0, child.CooldownTurns+context.CooldownDelta)
		actor.HP -= child.HPCost
		actor.MP -= child.MPCost
		out.Actions = append(out.Actions, map[string]any{"kind": "cast", "sourceId": actorID, "targetId": actorID, "skillId": child.ID, "afterHp": actor.HP, "afterMp": actor.MP, "afterAlive": actor.Alive, "meta": map[string]any{"comboSkillId": combo.ID, "comboIndex": n + 1}})
		execution := s.executeBattleSkill(state, now, actor, child, "combo_child", nil)
		out.Actions = append(out.Actions, execution.Actions...)
		out.Targets = append(out.Targets, execution.Targets...)
		out.Effects = append(out.Effects, execution.Effects...)
		actor = battleUnitByID(state, actorID)
		if actor == nil {
			break
		}
		if child.CooldownTurns > 0 {
			actor.Skills[idx].CooldownRemain = child.CooldownTurns + 1
		}
		if execution.Meta["interrupted"] != true {
			state.recordAchievementAction(actorID, child.ID, nil)
			out.Actions = append(out.Actions, s.battleTriggerUnitEquipmentHooksWithContext(state, now, actor, nil, nil, "on_after_skill_cast", 0, "", context)...)
		}
		out.Actions = append(out.Actions, s.battleFlushPendingUnitDeathEquipmentHooks(state, now)...)
	}
	return out
}

func equipmentSkillConfigsFromRows(rows []store.PlayerSkill) map[string]store.EquipmentSkillConfig {
	var out map[string]store.EquipmentSkillConfig
	for _, row := range rows {
		if row.EquipmentConfig == nil || !store.IsEquipmentSkill(row.SkillID) {
			continue
		}
		if out == nil {
			out = map[string]store.EquipmentSkillConfig{}
		}
		out[row.SkillID] = *row.EquipmentConfig
	}
	return out
}
