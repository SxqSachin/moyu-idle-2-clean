package httpapi

import (
	"context"
	"errors"
	"log"
	"math/rand"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
	"mysecondwebgame/server/internal/config"
	"mysecondwebgame/server/internal/kittylogic"
	"mysecondwebgame/server/internal/store"
)

const (
	minKittyNameLength = 1
	maxKittyNameLength = 24
)

var kittyFishingEquipSlots = []string{"fishing_rod", "fishing_reel"}

type catteryAdoptRequest struct {
	Name string `json:"name"`
}

type catteryPagingQuery struct {
	KittyID  int64
	Page     int
	PageSize int
}

type catteryCommissionStartRequest struct {
	KittyID      int64  `json:"kittyId"`
	CommissionID string `json:"commissionId"`
}

type catteryCommissionClaimRequest struct {
	RunID int64 `json:"runId"`
}

type catteryCombatConfigRequest struct {
	SkillSlots []string `json:"skillSlots"`
}

type catteryKittyEquipRequest struct {
	InstanceID int64  `json:"instanceId"`
	Slot       string `json:"slot"`
	EquipType  string `json:"equipType"`
}

type catteryKittyUnequipRequest struct {
	InstanceID int64 `json:"instanceId"`
}

type catteryKittyPersonalityRemoveRequest struct {
	PersonalityID string `json:"personalityId"`
}

type catteryKittyFeedRequest struct {
	InstanceID int64 `json:"instanceId"`
}

type catteryBattlePointStatePayload struct {
	KittyID          int64     `json:"kittyId"`
	TotalPoints      int64     `json:"totalPoints"`
	SpentPoints      int64     `json:"spentPoints"`
	Available        int64     `json:"available"`
	ResetRequirement fiber.Map `json:"resetRequirement"`
}

func (s *Server) handleCatteryCombatConfigGet(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	kittyID, errResp := s.parseKittyIDParam(c)
	if errResp != nil {
		return errResp
	}
	payload, err := s.buildCatteryCombatConfigPayload(c, userID, kittyID, false)
	if err != nil {
		if errors.Is(err, store.ErrKittyNotFound) {
			return s.writeAPIError(c, fiber.StatusNotFound, "CATTERY_KITTY_NOT_FOUND", "kitty not found", "")
		}
		return s.writeAPIError(c, fiber.StatusInternalServerError, "CATTERY_COMBAT_CONFIG_GET_FAILED", "load kitty combat config failed", err.Error())
	}
	return s.writeAPISuccess(c, fiber.StatusOK, payload)
}

func (s *Server) handleCatterySkillRuleConfigUpdate(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	kittyID, errResp := s.parseKittyIDParam(c)
	if errResp != nil {
		return errResp
	}
	var req ruleConfigUpsertRequest
	if err := c.BodyParser(&req); err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "CATTERY_KITTY_RULE_INVALID_PAYLOAD", "invalid payload", err.Error())
	}
	if strings.TrimSpace(req.RuleKind) != "" && strings.TrimSpace(req.RuleKind) != "skill" {
		return s.writeAPIError(c, fiber.StatusBadRequest, "CATTERY_KITTY_RULE_KIND_INVALID", "invalid kitty rule kind", "")
	}
	if err := s.deps.Store.UpsertKittySkillRule(c.Context(), userID, kittyID, req.SlotKey, req.Enabled, req.Groups); err != nil {
		if errors.Is(err, store.ErrKittyNotFound) {
			return s.writeAPIError(c, fiber.StatusNotFound, "CATTERY_KITTY_NOT_FOUND", "kitty not found", "")
		}
		if errors.Is(err, store.ErrSkillForgeSameSource) {
			return s.writeAPIError(c, fiber.StatusConflict, "COMBAT_CONFIG_SKILL_LINEAGE_CONFLICT", "source skill and derived skill are mutually exclusive", err.Error())
		}
		return s.writeAPIError(c, fiber.StatusInternalServerError, "CATTERY_KITTY_RULE_SAVE_FAILED", "save kitty rule failed", err.Error())
	}
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{"ok": true})
}

func (s *Server) handleCatteryCombatProfile(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	kittyID, errResp := s.parseKittyIDParam(c)
	if errResp != nil {
		return errResp
	}
	seeds, err := s.playerAttributeSeeds()
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "CATTERY_COMBAT_PROFILE_FAILED", "load attribute seeds failed", err.Error())
	}
	profile, err := s.deps.Store.GetKittyCombatProfile(c.Context(), userID, kittyID, seeds)
	if err != nil {
		if errors.Is(err, store.ErrKittyNotFound) {
			return s.writeAPIError(c, fiber.StatusNotFound, "CATTERY_KITTY_NOT_FOUND", "kitty not found", "")
		}
		return s.writeAPIError(c, fiber.StatusInternalServerError, "CATTERY_COMBAT_PROFILE_FAILED", "load kitty combat profile failed", err.Error())
	}
	return s.writeAPISuccess(c, fiber.StatusOK, s.kittyCombatProfilePayload(profile))
}

func (s *Server) kittyCombatProfilePayload(profile store.KittyCombatProfile) fiber.Map {
	return fiber.Map{
		"kitty":        profile.Kitty,
		"attributes":   profile.Attributes,
		"stats":        profile.Stats,
		"skills":       profile.Skills,
		"equipment":    profile.Equipment,
		"capabilities": s.kittyRoleCapabilities(),
	}
}

func (s *Server) kittyRoleCapabilities() fiber.Map {
	visibleSlots := append([]string{}, s.deps.Config.SharedGlobalSettings.Kitty.BattleEquipSlots...)
	visibleSlots = append(visibleSlots, kittyFishingEquipSlots...)
	return fiber.Map{
		"attributeScenes": []string{"battle", "work"},
		"equipmentTypes":  []string{"battle", "work"},
		"visibleSlots":    visibleSlots,
		"battlePoints":    true,
		"skills":          true,
		"battleItemBag":   false,
		"loadouts":        false,
		"sets":            false,
	}
}

func (s *Server) attachKittyAttributePatch(ctx context.Context, resp fiber.Map, action string, userID int64, kittyID int64) {
	if resp == nil {
		return
	}
	patch, changed, err := s.pushKittyAttributesNow(ctx, userID, kittyID)
	if err != nil {
		log.Printf("kitty %s immediate attribute sync failed user_id=%d kitty_id=%d err=%v", action, userID, kittyID, err)
		return
	}
	if changed {
		resp["roleAttributePatch"] = patch
	}
}

func (s *Server) attachKittyEquipmentPatch(ctx context.Context, resp fiber.Map, action string, userID int64, kittyID int64) {
	if resp == nil {
		return
	}
	patch, changed, err := s.pushKittyEquipmentNow(ctx, userID, kittyID)
	if err != nil {
		log.Printf("kitty %s immediate equipment sync failed user_id=%d kitty_id=%d err=%v", action, userID, kittyID, err)
		return
	}
	if changed {
		resp["roleEquipmentPatch"] = patch
	}
}

func (s *Server) handleCatteryBattlePointsGet(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	kittyID, errResp := s.parseKittyIDParam(c)
	if errResp != nil {
		return errResp
	}
	state, err := s.deps.Store.GetKittyBattlePointState(c.Context(), userID, kittyID)
	if err != nil {
		return s.writeCatteryBattlePointError(c, "CATTERY_BATTLE_POINT_GET_FAILED", err)
	}
	return s.writeAPISuccess(c, fiber.StatusOK, s.catteryBattlePointStatePayload(state))
}

func (s *Server) handleCatteryBattlePointsAllocateBatch(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	kittyID, errResp := s.parseKittyIDParam(c)
	if errResp != nil {
		return errResp
	}
	var req battlePointAllocateBatchRequest
	if err := c.BodyParser(&req); err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "CATTERY_BATTLE_POINT_ALLOCATE_INVALID_PAYLOAD", "invalid payload", err.Error())
	}
	if len(req.Allocations) == 0 {
		return s.writeAPIError(c, fiber.StatusBadRequest, "CATTERY_BATTLE_POINT_ALLOCATE_INVALID_REQUEST", "invalid allocate request", "")
	}
	allocations, normalizeErr := normalizeBattlePointAllocations(req.Allocations)
	if normalizeErr != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "CATTERY_BATTLE_POINT_ALLOCATE_INVALID_REQUEST", "invalid allocate request", "")
	}
	state, err := s.deps.Store.AllocateKittyBattlePointsBatch(c.Context(), userID, kittyID, allocations)
	if err != nil {
		return s.writeCatteryBattlePointError(c, "CATTERY_BATTLE_POINT_ALLOCATE_FAILED", err)
	}
	resp := fiber.Map{
		"battlePoints": s.catteryBattlePointStatePayload(state),
	}
	s.attachKittyAttributePatch(c.Context(), resp, "battle point allocate", userID, kittyID)
	s.syncAfterMutation(mutationSyncScope{
		UserIDs:         []int64{userID},
		KittyIDs:        []int64{kittyID},
		KittyAttributes: true,
	})
	return s.writeAPISuccess(c, fiber.StatusOK, resp)
}

func (s *Server) handleCatteryBattlePointsReset(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	kittyID, errResp := s.parseKittyIDParam(c)
	if errResp != nil {
		return errResp
	}
	state, err := s.deps.Store.ResetKittyBattlePoints(c.Context(), userID, kittyID)
	if err != nil {
		return s.writeCatteryBattlePointError(c, "CATTERY_BATTLE_POINT_RESET_FAILED", err)
	}
	resp := fiber.Map{
		"battlePoints": s.catteryBattlePointStatePayload(state),
	}
	s.attachKittyAttributePatch(c.Context(), resp, "battle point reset", userID, kittyID)
	s.attachKittyEquipmentPatch(c.Context(), resp, "battle point reset", userID, kittyID)
	s.pushInventoryInstances(userID)
	s.syncAfterMutation(mutationSyncScope{
		UserIDs:         []int64{userID},
		KittyIDs:        []int64{kittyID},
		Inventory:       true,
		KittyEquipment:  true,
		KittyAttributes: true,
		PlayerState:     true,
	})
	return s.writeAPISuccess(c, fiber.StatusOK, resp)
}

func (s *Server) handleCatteryCombatConfigUpdate(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	kittyID, errResp := s.parseKittyIDParam(c)
	if errResp != nil {
		return errResp
	}
	var req catteryCombatConfigRequest
	if err := c.BodyParser(&req); err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "CATTERY_COMBAT_CONFIG_INVALID_PAYLOAD", "invalid payload", err.Error())
	}
	if err := s.deps.Store.UpsertKittyCombatConfig(c.Context(), userID, kittyID, store.CombatConfig{SkillSlots: req.SkillSlots}); err != nil {
		if errors.Is(err, store.ErrKittyNotFound) {
			return s.writeAPIError(c, fiber.StatusNotFound, "CATTERY_KITTY_NOT_FOUND", "kitty not found", "")
		}
		if errors.Is(err, store.ErrSkillForgeSameSource) {
			return s.writeAPIError(c, fiber.StatusConflict, "COMBAT_CONFIG_SKILL_LINEAGE_CONFLICT", "source skill and derived skill are mutually exclusive", err.Error())
		}
		return s.writeAPIError(c, fiber.StatusInternalServerError, "CATTERY_COMBAT_CONFIG_FAILED", "save kitty combat config failed", err.Error())
	}
	payload, payloadErr := s.buildCatteryCombatConfigPayload(c, userID, kittyID, true)
	if payloadErr != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "CATTERY_COMBAT_CONFIG_FAILED", "reload kitty combat config failed", payloadErr.Error())
	}
	s.syncAfterMutation(mutationSyncScope{UserIDs: []int64{userID}, PlayerState: true})
	return s.writeAPISuccess(c, fiber.StatusOK, payload)
}

func (s *Server) buildCatteryCombatConfigPayload(c *fiber.Ctx, userID, kittyID int64, skipDefaultInit bool) (fiber.Map, error) {
	seeds, err := s.playerAttributeSeeds()
	if err != nil {
		return nil, err
	}
	profile, err := s.deps.Store.GetKittyCombatProfile(c.Context(), userID, kittyID, seeds)
	if err != nil {
		return nil, err
	}
	defaultBasicSkillID := s.battleDefaultBasicSkillID()
	if !skipDefaultInit && len(profile.Skills) == 0 && defaultBasicSkillID != "" {
		if err := s.deps.Store.UpsertKittyCombatConfig(c.Context(), userID, kittyID, store.CombatConfig{SkillSlots: []string{defaultBasicSkillID}}); err != nil {
			return nil, err
		}
		profile, err = s.deps.Store.GetKittyCombatProfile(c.Context(), userID, kittyID, seeds)
		if err != nil {
			return nil, err
		}
	}
	ownedSkills := make([]store.OwnedSkill, 0, len(profile.Skills)+1)
	ownedSkillMap := make(map[string]string, len(profile.Skills)+1)
	for _, skill := range profile.Skills {
		id := strings.TrimSpace(skill.SkillID)
		if id == "" {
			continue
		}
		ownedSkillMap[strings.ToLower(id)] = id
		ownedSkills = append(ownedSkills, store.OwnedSkill{SkillID: id, Level: skill.Level, Exp: skill.Exp})
	}
	if defaultBasicSkillID != "" {
		if _, exists := ownedSkillMap[strings.ToLower(defaultBasicSkillID)]; !exists {
			ownedSkillMap[strings.ToLower(defaultBasicSkillID)] = defaultBasicSkillID
			ownedSkills = append(ownedSkills, store.OwnedSkill{SkillID: defaultBasicSkillID, Level: 1, Exp: 0})
		}
	}
	sort.SliceStable(ownedSkills, func(i, j int) bool {
		return ownedSkills[i].SkillID < ownedSkills[j].SkillID
	})
	stats := map[string]float64{}
	lockVersionByAttr := map[string]int64{}
	for _, attr := range profile.Attributes {
		key := strings.TrimSpace(strings.ToLower(attr.AttrKey))
		if key != "" {
			if prevVersion, exists := lockVersionByAttr[key]; exists && attr.LockVersion < prevVersion {
				continue
			}
			stats[key] = attr.FinalValue
			lockVersionByAttr[key] = attr.LockVersion
		}
	}
	currentSlots := battleAvailableSkillSlots(stats["intelligence"], s.battleSkillSlotRule())
	configSlots := make([]string, 0, len(profile.Skills))
	for _, skill := range profile.Skills {
		if skill.Enabled {
			configSlots = append(configSlots, strings.TrimSpace(skill.SkillID))
		}
	}
	configSlots = normalizeCombatSkillSlots(configSlots, ownedSkillMap, currentSlots, defaultBasicSkillID)
	enableCtx := battleSkillEnableContext{
		Attributes:    stats,
		WeaponClasses: s.weaponClassSetFromEquippedItems(profile.Equipment),
	}
	skillEligibility := s.battleSkillEligibilityMapFromContext(enableCtx)
	catalog := s.battleSkillCatalogRows(skillEligibility)
	sort.SliceStable(catalog, func(i, j int) bool {
		left, _ := catalog[i]["id"].(string)
		right, _ := catalog[j]["id"].(string)
		return left < right
	})
	rulesList, err := s.deps.Store.ListKittySkillRuleSets(c.Context(), userID, kittyID)
	if err != nil {
		return nil, err
	}
	return fiber.Map{
		"config":              store.CombatConfig{SkillSlots: configSlots},
		"defaultBasicSkillId": defaultBasicSkillID,
		"ownedSkills":         ownedSkills,
		"rules":               rulesList,
		"skillCatalog":        catalog,
		"slotRule": fiber.Map{
			"baseSkillSlots": s.battleSkillSlotRule().BaseSkillSlots,
			"intPerExtra":    s.battleSkillSlotRule().IntPerExtra,
			"minSkillSlots":  s.battleSkillSlotRule().MinSkillSlots,
			"maxSkillSlots":  s.battleSkillSlotRule().MaxSkillSlots,
			"current":        currentSlots,
		},
		"ruleLimits": fiber.Map{
			"maxGroups":             s.deps.Config.GlobalSettings.Rule.MaxGroups,
			"maxConditionsPerGroup": s.deps.Config.GlobalSettings.Rule.MaxConditionsPerGroup,
		},
	}, nil
}

func (s *Server) handleCatteryKittyEquip(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	kittyID, errResp := s.parseKittyIDParam(c)
	if errResp != nil {
		return errResp
	}
	var req catteryKittyEquipRequest
	if err := c.BodyParser(&req); err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "CATTERY_KITTY_EQUIP_INVALID_PAYLOAD", "invalid payload", err.Error())
	}
	if req.InstanceID <= 0 || strings.TrimSpace(req.Slot) == "" {
		return s.writeAPIError(c, fiber.StatusBadRequest, "CATTERY_KITTY_EQUIP_INVALID_REQUEST", "invalid equip request", "")
	}
	req.EquipType = normalizedEquipType(req.EquipType)
	if req.EquipType == "" {
		req.EquipType = "battle"
	}
	if req.EquipType != "battle" && req.EquipType != "work" {
		return s.writeAPIError(c, fiber.StatusBadRequest, "CATTERY_KITTY_EQUIP_TYPE_MISMATCH", "equip type mismatch", "")
	}
	if req.EquipType == "work" && !containsString(kittyFishingEquipSlots, req.Slot) {
		return s.writeAPIError(c, fiber.StatusBadRequest, "CATTERY_KITTY_EQUIP_SLOT_NOT_ALLOWED", "slot not allowed", "")
	}
	instance, _, getErr := s.deps.Store.GetItemInstance(c.Context(), userID, req.InstanceID)
	if getErr != nil {
		return s.writeAPIError(c, fiber.StatusNotFound, "CATTERY_KITTY_EQUIP_INSTANCE_NOT_FOUND", "item instance not found", "")
	}
	itemDef := s.findCatalogItem(instance.TemplateItemID)
	template := s.findEquipmentTemplate(instance.TemplateItemID)
	if !isEquipmentItem(itemDef, template) {
		return s.writeAPIError(c, fiber.StatusBadRequest, "CATTERY_KITTY_EQUIP_NOT_EQUIPMENT", "item is not equipment", "")
	}
	if templateEquipType(template) != req.EquipType {
		return s.writeAPIError(c, fiber.StatusBadRequest, "CATTERY_KITTY_EQUIP_TYPE_MISMATCH", "equip type mismatch", "")
	}
	if !isEquipTargetAllowed(template, "kitty") {
		return s.writeAPIError(c, fiber.StatusBadRequest, "CATTERY_KITTY_EQUIP_TARGET_NOT_ALLOWED", "equipment target not allowed", "")
	}
	allowedSlots := equipAllowedSlots(template)
	if req.EquipType == "battle" {
		allowedSlots = intersectSlots(allowedSlots, s.deps.Config.SharedGlobalSettings.Kitty.BattleEquipSlots)
	} else {
		allowedSlots = intersectSlots(allowedSlots, kittyFishingEquipSlots)
	}
	if len(allowedSlots) == 0 || !containsString(allowedSlots, req.Slot) {
		return s.writeAPIError(c, fiber.StatusBadRequest, "CATTERY_KITTY_EQUIP_SLOT_NOT_ALLOWED", "slot not allowed", "")
	}
	unmet, unmetErr := s.unmetKittyStatusRequire(c.Context(), userID, kittyID, template.StatusRequirement)
	if unmetErr != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "CATTERY_KITTY_EQUIP_FAILED", "equip kitty item failed", unmetErr.Error())
	}
	if len(unmet) > 0 {
		return s.writeAPIError(c, fiber.StatusConflict, "EQUIP_STATUS_REQUIREMENT_NOT_MET", "status requirements not met", "")
	}
	occupySlots := equipOccupySlots(template, req.Slot)
	if len(occupySlots) == 0 {
		return s.writeAPIError(c, fiber.StatusBadRequest, "CATTERY_KITTY_EQUIP_SLOT_NOT_ALLOWED", "slot not allowed", "")
	}
	if req.EquipType == "battle" {
		for _, slot := range occupySlots {
			if !containsString(s.deps.Config.SharedGlobalSettings.Kitty.BattleEquipSlots, slot) {
				return s.writeAPIError(c, fiber.StatusBadRequest, "CATTERY_KITTY_EQUIP_SLOT_NOT_ALLOWED", "slot not allowed", "")
			}
		}
	} else {
		for _, slot := range occupySlots {
			if !containsString(kittyFishingEquipSlots, slot) {
				return s.writeAPIError(c, fiber.StatusBadRequest, "CATTERY_KITTY_EQUIP_SLOT_NOT_ALLOWED", "slot not allowed", "")
			}
		}
	}
	if err := s.deps.Store.EquipKittyItem(c.Context(), userID, kittyID, req.InstanceID, req.EquipType, occupySlots); err != nil {
		return s.writeCatteryKittyCombatError(c, err)
	}
	resp := fiber.Map{"ok": true, "kittyId": kittyID}
	s.attachKittyEquipmentPatch(c.Context(), resp, "equip", userID, kittyID)
	s.attachKittyAttributePatch(c.Context(), resp, "equip", userID, kittyID)
	s.pushInventoryInstances(userID)
	s.syncAfterMutation(mutationSyncScope{
		UserIDs:         []int64{userID},
		KittyIDs:        []int64{kittyID},
		Inventory:       true,
		Equipment:       true,
		Attributes:      true,
		KittyEquipment:  true,
		KittyAttributes: true,
		PlayerState:     true,
	})
	return s.writeAPISuccess(c, fiber.StatusOK, resp)
}

func (s *Server) unmetKittyStatusRequire(ctx context.Context, userID, kittyID int64, require map[string]int64) ([]string, error) {
	if len(require) == 0 {
		return nil, nil
	}
	seeds, err := s.playerAttributeSeeds()
	if err != nil {
		return nil, err
	}
	if err := s.deps.Store.EnsureKittyAttributes(ctx, userID, kittyID, seeds); err != nil {
		return nil, err
	}
	states, err := s.deps.Store.ListKittyAttributes(ctx, userID, kittyID, seeds)
	if err != nil {
		return nil, err
	}
	attrMap := make(map[string]float64, len(states))
	for _, state := range states {
		key := strings.TrimSpace(state.AttrKey)
		if key == "" {
			continue
		}
		attrMap[key] = state.FinalValue
	}
	return unmetStatusRequireFromFinalValues(attrMap, require), nil
}

func unmetStatusRequireFromFinalValues(attrMap map[string]float64, require map[string]int64) []string {
	unmet := make([]string, 0)
	for key, minRequired := range require {
		normalizedKey := strings.TrimSpace(key)
		if normalizedKey == "" || minRequired <= 0 {
			continue
		}
		current := attrMap[normalizedKey]
		if current < float64(minRequired) {
			unmet = append(unmet, normalizedKey)
		}
	}
	sort.Strings(unmet)
	return unmet
}

func (s *Server) handleCatteryKittyUnequip(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	kittyID, errResp := s.parseKittyIDParam(c)
	if errResp != nil {
		return errResp
	}
	var req catteryKittyUnequipRequest
	if err := c.BodyParser(&req); err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "CATTERY_KITTY_UNEQUIP_INVALID_PAYLOAD", "invalid payload", err.Error())
	}
	if err := s.deps.Store.UnequipKittyInstance(c.Context(), userID, kittyID, req.InstanceID); err != nil {
		return s.writeCatteryKittyCombatError(c, err)
	}
	resp := fiber.Map{"ok": true, "kittyId": kittyID}
	s.attachKittyEquipmentPatch(c.Context(), resp, "unequip", userID, kittyID)
	s.attachKittyAttributePatch(c.Context(), resp, "unequip", userID, kittyID)
	s.pushInventoryInstances(userID)
	s.syncAfterMutation(mutationSyncScope{
		UserIDs:         []int64{userID},
		KittyIDs:        []int64{kittyID},
		Inventory:       true,
		KittyEquipment:  true,
		KittyAttributes: true,
		PlayerState:     true,
	})
	return s.writeAPISuccess(c, fiber.StatusOK, resp)
}

func (s *Server) handleCatteryKittyFeed(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	kittyID, errResp := s.parseKittyIDParam(c)
	if errResp != nil {
		return errResp
	}
	var req catteryKittyFeedRequest
	if err := c.BodyParser(&req); err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "CATTERY_KITTY_FEED_INVALID_PAYLOAD", "invalid payload", err.Error())
	}
	if req.InstanceID <= 0 {
		return s.writeAPIError(c, fiber.StatusBadRequest, "CATTERY_KITTY_FEED_INVALID_REQUEST", "invalid feed request", "")
	}
	instance, _, getErr := s.deps.Store.GetItemInstance(c.Context(), userID, req.InstanceID)
	if getErr != nil {
		if errors.Is(getErr, pgx.ErrNoRows) {
			return s.writeAPIError(c, fiber.StatusNotFound, "CATTERY_KITTY_FEED_INSTANCE_NOT_FOUND", "item instance not found", "")
		}
		return s.writeAPIError(c, fiber.StatusInternalServerError, "CATTERY_KITTY_FEED_LOAD_FAILED", "load item instance failed", getErr.Error())
	}
	itemDef := s.findCatalogItem(instance.TemplateItemID)
	battleLevelCapGain := kittyFeedBattleLevelCapGain(itemDef)
	if battleLevelCapGain <= 0 {
		return s.writeAPIError(c, fiber.StatusBadRequest, "CATTERY_KITTY_FEED_INVALID_ITEM", "item cannot feed kitty battle level cap", "")
	}
	result, feedErr := s.deps.Store.FeedKittyBattleLevelCap(c.Context(), userID, kittyID, req.InstanceID, battleLevelCapGain)
	if feedErr != nil {
		switch {
		case errors.Is(feedErr, store.ErrKittyNotFound):
			return s.writeAPIError(c, fiber.StatusNotFound, "CATTERY_KITTY_NOT_FOUND", "kitty not found", "")
		case errors.Is(feedErr, store.ErrInstanceNotOwned):
			return s.writeAPIError(c, fiber.StatusForbidden, "CATTERY_KITTY_FEED_INSTANCE_NOT_OWNED", "instance not owned", "")
		case errors.Is(feedErr, store.ErrInvalidInstanceState):
			return s.writeAPIError(c, fiber.StatusConflict, "CATTERY_KITTY_FEED_INSTANCE_STATE_INVALID", "instance state invalid", "")
		case errors.Is(feedErr, store.ErrKittyFeedItemInvalid):
			return s.writeAPIError(c, fiber.StatusBadRequest, "CATTERY_KITTY_FEED_INVALID_ITEM", "invalid kitty feed item", "")
		default:
			return s.writeAPIError(c, fiber.StatusInternalServerError, "CATTERY_KITTY_FEED_FAILED", "feed kitty failed", feedErr.Error())
		}
	}
	kitty, kittyErr := s.deps.Store.GetPlayerKitty(c.Context(), userID, kittyID)
	if kittyErr != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "CATTERY_KITTY_FEED_FAILED", "reload kitty failed", kittyErr.Error())
	}
	kitties, listErr := s.deps.Store.ListPlayerKitties(c.Context(), userID)
	if listErr != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "CATTERY_LIST_FAILED", "load kitty list failed", listErr.Error())
	}
	s.syncAfterMutation(mutationSyncScope{
		UserIDs:     []int64{userID},
		Inventory:   true,
		PlayerState: true,
	})
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{
		"kitty":   kitty,
		"kitties": kitties,
		"gain": fiber.Map{
			"battleLevelCapGain": result.BattleLevelCapGain,
			"battleLevelCap":     result.BattleLevelCap,
		},
	})
}

func (s *Server) handleCatteryKittyPersonalityRemove(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	kittyID, errResp := s.parseKittyIDParam(c)
	if errResp != nil {
		return errResp
	}
	var req catteryKittyPersonalityRemoveRequest
	if err := c.BodyParser(&req); err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "CATTERY_KITTY_PERSONALITY_REMOVE_INVALID_PAYLOAD", "invalid payload", err.Error())
	}
	personalityID := strings.TrimSpace(req.PersonalityID)
	kitty, removeErr := s.deps.Store.RemoveKittyPersonalityWithCost(
		c.Context(),
		userID,
		kittyID,
		personalityID,
		s.deps.Config.SharedGlobalSettings.Kitty.RemovePersonalityItemID,
		s.deps.Config.SharedGlobalSettings.Kitty.RemovePersonalityCount,
	)
	if removeErr != nil {
		switch {
		case errors.Is(removeErr, store.ErrKittyPersonalityNotFound):
			return s.writeAPIError(c, fiber.StatusBadRequest, "CATTERY_KITTY_PERSONALITY_NOT_FOUND", "kitty personality not found", "")
		case errors.Is(removeErr, store.ErrKittyPersonalityInvalid):
			return s.writeAPIError(c, fiber.StatusBadRequest, "CATTERY_KITTY_PERSONALITY_INVALID", "invalid kitty personality", "")
		case errors.Is(removeErr, store.ErrInsufficientItem):
			return s.writeAPIError(c, fiber.StatusConflict, "CATTERY_KITTY_PERSONALITY_REMOVE_COST_INSUFFICIENT", "insufficient personality remove item", "")
		case errors.Is(removeErr, store.ErrKittyNotFound):
			return s.writeAPIError(c, fiber.StatusNotFound, "CATTERY_KITTY_NOT_FOUND", "kitty not found", "")
		default:
			return s.writeAPIError(c, fiber.StatusInternalServerError, "CATTERY_KITTY_PERSONALITY_REMOVE_FAILED", "remove kitty personality failed", removeErr.Error())
		}
	}
	kitties, listErr := s.deps.Store.ListPlayerKitties(c.Context(), userID)
	if listErr != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "CATTERY_LIST_FAILED", "load kitty list failed", listErr.Error())
	}
	s.syncAfterMutation(mutationSyncScope{
		UserIDs:     []int64{userID},
		Inventory:   true,
		PlayerState: true,
	})
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{
		"kitty":   kitty,
		"kitties": kitties,
		"cost": fiber.Map{
			"itemId": s.deps.Config.SharedGlobalSettings.Kitty.RemovePersonalityItemID,
			"count":  s.deps.Config.SharedGlobalSettings.Kitty.RemovePersonalityCount,
		},
	})
}

func (s *Server) handleCatteryAdopt(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	var req catteryAdoptRequest
	if err := c.BodyParser(&req); err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "CATTERY_ADOPT_INVALID_PAYLOAD", "invalid payload", err.Error())
	}
	normalizedName := strings.TrimSpace(req.Name)
	nameLen := usernameLength(normalizedName)
	if nameLen < minKittyNameLength || nameLen > maxKittyNameLength || !isValidUsername(normalizedName) {
		return s.writeAPIError(c, fiber.StatusBadRequest, "CATTERY_ADOPT_NAME_INVALID", "invalid kitty name", "")
	}
	if err := s.rejectModeratedAPI(c, moderationRuleKittyName, normalizedName, userID); err != nil {
		return err
	}
	kitty, err := s.deps.Store.AdoptKitty(c.Context(), userID, normalizedName)
	if err != nil {
		switch {
		case errors.Is(err, store.ErrCatteryUnavailable):
			return s.writeAPIError(c, fiber.StatusConflict, "CATTERY_NOT_BUILT", "cattery not built", "")
		case errors.Is(err, store.ErrKittyAdoptLimitReached):
			return s.writeAPIError(c, fiber.StatusConflict, "CATTERY_CAPACITY_REACHED", "cattery capacity reached", "")
		case errors.Is(err, store.ErrKittyNameInvalid):
			return s.writeAPIError(c, fiber.StatusBadRequest, "CATTERY_ADOPT_NAME_INVALID", "invalid kitty name", "")
		default:
			return s.writeAPIError(c, fiber.StatusInternalServerError, "CATTERY_ADOPT_FAILED", "adopt kitty failed", err.Error())
		}
	}
	kitties, listErr := s.deps.Store.ListPlayerKitties(c.Context(), userID)
	if listErr != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "CATTERY_LIST_FAILED", "load kitty list failed", listErr.Error())
	}
	s.syncAfterMutation(mutationSyncScope{
		UserIDs:     []int64{userID},
		PlayerState: true,
	})
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{
		"kitty":   kitty,
		"kitties": kitties,
	})
}

func (s *Server) handleCatteryCommissionCatalog(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	kittyID, errResp := s.parseKittyIDQuery(c)
	if errResp != nil {
		return errResp
	}
	kitty, err := s.deps.Store.GetPlayerKitty(c.Context(), userID, kittyID)
	if err != nil {
		if errors.Is(err, store.ErrKittyNotFound) {
			return s.writeAPIError(c, fiber.StatusNotFound, "CATTERY_KITTY_NOT_FOUND", "kitty not found", "")
		}
		return s.writeAPIError(c, fiber.StatusInternalServerError, "CATTERY_KITTY_QUERY_FAILED", "load kitty failed", err.Error())
	}
	now := time.Now()
	items := kittylogic.VisibleCommissions(s.deps.Config.KittySettings, userID, kitty, now)
	visible := make([]fiber.Map, 0, len(items))
	for _, item := range items {
		visible = append(visible, visibleCommissionPayload(item))
	}
	used, _ := s.deps.Store.CountKittyCommissionsSince(c.Context(), userID, kittyID, dayStart(now))
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{
		"items":      visible,
		"period":     kittylogic.PeriodKey(now, s.deps.Config.KittySettings.Commissions.RefreshMode),
		"dailyLimit": s.deps.Config.KittySettings.Commissions.DailyLimitPerKitty,
		"usedToday":  used,
	})
}

func (s *Server) handleCatteryCommissionRuns(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	kittyID, errResp := s.parseKittyIDQuery(c)
	if errResp != nil {
		return errResp
	}
	if _, err := s.deps.Store.GetPlayerKitty(c.Context(), userID, kittyID); err != nil {
		if errors.Is(err, store.ErrKittyNotFound) {
			return s.writeAPIError(c, fiber.StatusNotFound, "CATTERY_KITTY_NOT_FOUND", "kitty not found", "")
		}
		return s.writeAPIError(c, fiber.StatusInternalServerError, "CATTERY_KITTY_QUERY_FAILED", "load kitty failed", err.Error())
	}
	runs, err := s.deps.Store.ListPlayerKittyCommissionRuns(c.Context(), userID, kittyID, 20)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "CATTERY_COMMISSIONS_QUERY_FAILED", "load kitty commissions failed", err.Error())
	}
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{"items": runs})
}

func (s *Server) handleCatteryCommissionStart(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	var req catteryCommissionStartRequest
	if err := c.BodyParser(&req); err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "CATTERY_COMMISSION_INVALID_PAYLOAD", "invalid payload", err.Error())
	}
	kitty, err := s.deps.Store.GetPlayerKitty(c.Context(), userID, req.KittyID)
	if err != nil {
		if errors.Is(err, store.ErrKittyNotFound) {
			return s.writeAPIError(c, fiber.StatusNotFound, "CATTERY_KITTY_NOT_FOUND", "kitty not found", "")
		}
		return s.writeAPIError(c, fiber.StatusInternalServerError, "CATTERY_KITTY_QUERY_FAILED", "load kitty failed", err.Error())
	}
	item, ok := s.findVisibleCommission(userID, kitty, req.CommissionID, time.Now())
	if !ok {
		return s.writeAPIError(c, fiber.StatusConflict, "CATTERY_COMMISSION_NOT_VISIBLE", "commission is not visible", "")
	}
	run, err := s.deps.Store.StartKittyCommission(c.Context(), store.KittyCommissionStartInput{
		UserID:          userID,
		KittyID:         kitty.ID,
		CommissionID:    item.ID,
		MinKittyLevel:   item.MinKittyLevel,
		DurationSeconds: item.DurationSeconds,
		Costs:           item.Costs,
		DailyLimit:      s.deps.Config.KittySettings.Commissions.DailyLimitPerKitty,
		DayStart:        dayStart(time.Now()),
	})
	if err != nil {
		return s.writeCatteryCommissionError(c, err)
	}
	s.NotifyKittyStatusChanged(userID, kitty.ID, store.KittyStatusInLifeQueue)
	kitties, _ := s.deps.Store.ListPlayerKitties(c.Context(), userID)
	s.syncAfterMutation(mutationSyncScope{UserIDs: []int64{userID}, PlayerState: true})
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{"run": run, "kitties": kitties})
}

func (s *Server) handleCatteryCommissionClaim(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	var req catteryCommissionClaimRequest
	if err := c.BodyParser(&req); err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "CATTERY_COMMISSION_INVALID_PAYLOAD", "invalid payload", err.Error())
	}
	run, err := s.deps.Store.GetKittyCommissionRun(c.Context(), userID, req.RunID)
	if err != nil {
		return s.writeCatteryCommissionError(c, err)
	}
	kitty, err := s.deps.Store.GetPlayerKitty(c.Context(), userID, run.KittyID)
	if err != nil {
		return s.writeCatteryCommissionError(c, err)
	}
	item, ok := findCommissionItem(s.deps.Config.KittySettings, run.CommissionID)
	if !ok {
		return s.writeAPIError(c, fiber.StatusConflict, "CATTERY_COMMISSION_CONFIG_NOT_FOUND", "commission config not found", "")
	}
	pool, ok := s.deps.Config.KittySettings.Commissions.RewardPools[strings.TrimSpace(item.RewardPoolID)]
	if !ok {
		return s.writeAPIError(c, fiber.StatusConflict, "CATTERY_COMMISSION_REWARD_POOL_NOT_FOUND", "commission reward pool not found", "")
	}
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	rolled := kittylogic.RollRewardPool(item.RewardPoolID, pool, kitty.Level, rng)
	rolled.KittyExp += item.KittyExp
	rolled.BondExp += item.BondExp
	rolled.MoodDelta += item.MoodDelta
	rewardPayload := kittylogic.RewardPayload(rolled)
	rewardPayload["resultKey"] = item.ResultKey
	claimed, err := s.deps.Store.ClaimKittyCommission(c.Context(), store.KittyCommissionClaimInput{
		UserID:        userID,
		RunID:         req.RunID,
		Rewards:       rolled.Items,
		RewardPayload: rewardPayload,
		KittyExp:      rolled.KittyExp,
		BondExp:       rolled.BondExp,
		MoodDelta:     rolled.MoodDelta,
	})
	if err != nil {
		return s.writeCatteryCommissionError(c, err)
	}
	kitties, _ := s.deps.Store.ListPlayerKitties(c.Context(), userID)
	for _, kittyState := range kitties {
		if kittyState.ID != claimed.KittyID {
			continue
		}
		s.NotifyKittyStatusChanged(userID, claimed.KittyID, kittyState.Status)
		break
	}
	s.syncAfterMutation(mutationSyncScope{UserIDs: []int64{userID}, PlayerState: true})
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{"run": claimed, "reward": rewardPayload, "kitties": kitties})
}

func (s *Server) handleCatteryJournals(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	paging, errResp := s.parseCatteryPagingQuery(c)
	if errResp != nil {
		return errResp
	}
	if _, err := s.deps.Store.GetPlayerKitty(c.Context(), userID, paging.KittyID); err != nil {
		if errors.Is(err, store.ErrKittyNotFound) {
			return s.writeAPIError(c, fiber.StatusNotFound, "CATTERY_KITTY_NOT_FOUND", "kitty not found", "")
		}
		return s.writeAPIError(c, fiber.StatusInternalServerError, "CATTERY_KITTY_QUERY_FAILED", "load kitty failed", err.Error())
	}
	items, total, err := s.deps.Store.ListPlayerKittyJournals(c.Context(), userID, paging.KittyID, paging.Page, paging.PageSize)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "CATTERY_JOURNALS_QUERY_FAILED", "load kitty journals failed", err.Error())
	}
	for i := range items {
		items[i].Reward = kittylogic.SanitizeRewardPayload(items[i].Reward)
	}
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{
		"items":    items,
		"total":    total,
		"page":     paging.Page,
		"pageSize": paging.PageSize,
	})
}

func (s *Server) handleCatteryEncounters(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	paging, errResp := s.parseCatteryPagingQuery(c)
	if errResp != nil {
		return errResp
	}
	if _, err := s.deps.Store.GetPlayerKitty(c.Context(), userID, paging.KittyID); err != nil {
		if errors.Is(err, store.ErrKittyNotFound) {
			return s.writeAPIError(c, fiber.StatusNotFound, "CATTERY_KITTY_NOT_FOUND", "kitty not found", "")
		}
		return s.writeAPIError(c, fiber.StatusInternalServerError, "CATTERY_KITTY_QUERY_FAILED", "load kitty failed", err.Error())
	}
	items, total, err := s.deps.Store.ListPlayerKittyEncounters(c.Context(), userID, paging.KittyID, paging.Page, paging.PageSize)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "CATTERY_ENCOUNTERS_QUERY_FAILED", "load kitty encounters failed", err.Error())
	}
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{
		"items":    items,
		"total":    total,
		"page":     paging.Page,
		"pageSize": paging.PageSize,
	})
}

func (s *Server) parseCatteryPagingQuery(c *fiber.Ctx) (catteryPagingQuery, error) {
	rawKittyID := strings.TrimSpace(c.Query("kittyId"))
	kittyID, err := strconv.ParseInt(rawKittyID, 10, 64)
	if rawKittyID == "" || err != nil || kittyID <= 0 {
		return catteryPagingQuery{}, s.writeAPIError(c, fiber.StatusBadRequest, "CATTERY_QUERY_INVALID_KITTY_ID", "invalid kitty id", "")
	}
	page := 1
	if rawPage := strings.TrimSpace(c.Query("page")); rawPage != "" {
		parsed, parseErr := strconv.Atoi(rawPage)
		if parseErr != nil || parsed <= 0 {
			return catteryPagingQuery{}, s.writeAPIError(c, fiber.StatusBadRequest, "CATTERY_QUERY_INVALID_PAGE", "invalid page", "")
		}
		page = parsed
	}
	pageSize := 20
	if rawPageSize := strings.TrimSpace(c.Query("pageSize")); rawPageSize != "" {
		parsed, parseErr := strconv.Atoi(rawPageSize)
		if parseErr != nil || parsed <= 0 {
			return catteryPagingQuery{}, s.writeAPIError(c, fiber.StatusBadRequest, "CATTERY_QUERY_INVALID_PAGE_SIZE", "invalid page size", "")
		}
		pageSize = parsed
	}
	return catteryPagingQuery{
		KittyID:  kittyID,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

func (s *Server) parseKittyIDQuery(c *fiber.Ctx) (int64, error) {
	rawKittyID := strings.TrimSpace(c.Query("kittyId"))
	kittyID, err := strconv.ParseInt(rawKittyID, 10, 64)
	if rawKittyID == "" || err != nil || kittyID <= 0 {
		return 0, s.writeAPIError(c, fiber.StatusBadRequest, "CATTERY_QUERY_INVALID_KITTY_ID", "invalid kitty id", "")
	}
	return kittyID, nil
}

func (s *Server) parseKittyIDParam(c *fiber.Ctx) (int64, error) {
	rawKittyID := strings.TrimSpace(c.Params("kittyID"))
	kittyID, err := strconv.ParseInt(rawKittyID, 10, 64)
	if rawKittyID == "" || err != nil || kittyID <= 0 {
		return 0, s.writeAPIError(c, fiber.StatusBadRequest, "CATTERY_QUERY_INVALID_KITTY_ID", "invalid kitty id", "")
	}
	return kittyID, nil
}

func (s *Server) findVisibleCommission(userID int64, kitty store.PlayerKitty, commissionID string, now time.Time) (config.KittyCommissionItem, bool) {
	for _, item := range kittylogic.VisibleCommissions(s.deps.Config.KittySettings, userID, kitty, now) {
		if strings.TrimSpace(item.ID) == strings.TrimSpace(commissionID) {
			return item, true
		}
	}
	return config.KittyCommissionItem{}, false
}

func findCommissionItem(settings config.KittySettings, commissionID string) (config.KittyCommissionItem, bool) {
	commissionID = strings.TrimSpace(commissionID)
	for _, item := range settings.Commissions.Items {
		if strings.TrimSpace(item.ID) == commissionID {
			return item, true
		}
	}
	return config.KittyCommissionItem{}, false
}

func visibleCommissionPayload(item config.KittyCommissionItem) fiber.Map {
	return fiber.Map{
		"id":              strings.TrimSpace(item.ID),
		"rarity":          strings.TrimSpace(item.Rarity),
		"nameKey":         strings.TrimSpace(item.NameKey),
		"descKey":         strings.TrimSpace(item.DescKey),
		"resultKey":       strings.TrimSpace(item.ResultKey),
		"minKittyLevel":   item.MinKittyLevel,
		"durationSeconds": item.DurationSeconds,
		"costs":           item.Costs,
	}
}

func dayStart(now time.Time) time.Time {
	y, m, d := now.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, now.Location())
}

func (s *Server) writeCatteryCommissionError(c *fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, store.ErrKittyNotFound):
		return s.writeAPIError(c, fiber.StatusNotFound, "CATTERY_KITTY_NOT_FOUND", "kitty not found", "")
	case errors.Is(err, store.ErrKittyCommissionNotFound):
		return s.writeAPIError(c, fiber.StatusNotFound, "CATTERY_COMMISSION_NOT_FOUND", "commission not found", "")
	case errors.Is(err, store.ErrKittyCommissionLimitReached):
		return s.writeAPIError(c, fiber.StatusConflict, "CATTERY_COMMISSION_LIMIT_REACHED", "commission limit reached", "")
	case errors.Is(err, store.ErrKittyLevelTooLow):
		return s.writeAPIError(c, fiber.StatusConflict, "CATTERY_KITTY_LEVEL_TOO_LOW", "kitty level too low", "")
	case errors.Is(err, store.ErrInsufficientItem):
		return s.writeAPIError(c, fiber.StatusConflict, "CATTERY_COMMISSION_COST_NOT_ENOUGH", "not enough items", "")
	case errors.Is(err, store.ErrKittyCommissionNotReady):
		return s.writeAPIError(c, fiber.StatusConflict, "CATTERY_COMMISSION_NOT_READY", "commission not ready", "")
	case errors.Is(err, store.ErrKittyCommissionNotRunning):
		return s.writeAPIError(c, fiber.StatusConflict, "CATTERY_COMMISSION_NOT_RUNNING", "commission is not running", "")
	case errors.Is(err, store.ErrKittyModeConflictLifeQueue):
		return s.writeAPIError(c, fiber.StatusConflict, "KITTY_MODE_CONFLICT_LIFE_QUEUE", "kitty is in combat", "")
	default:
		return s.writeAPIError(c, fiber.StatusInternalServerError, "CATTERY_COMMISSION_FAILED", "kitty commission failed", err.Error())
	}
}

func (s *Server) writeCatteryKittyCombatError(c *fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, store.ErrKittyNotFound):
		return s.writeAPIError(c, fiber.StatusNotFound, "CATTERY_KITTY_NOT_FOUND", "kitty not found", "")
	case errors.Is(err, store.ErrInstanceNotOwned):
		return s.writeAPIError(c, fiber.StatusForbidden, "CATTERY_KITTY_EQUIP_INSTANCE_NOT_OWNED", "instance not owned", "")
	case errors.Is(err, store.ErrEquipSlotNotFound):
		return s.writeAPIError(c, fiber.StatusNotFound, "CATTERY_KITTY_EQUIP_SLOT_NOT_FOUND", "equip slot not found", "")
	case errors.Is(err, store.ErrEquipSlotNotAllowed):
		return s.writeAPIError(c, fiber.StatusBadRequest, "CATTERY_KITTY_EQUIP_SLOT_NOT_ALLOWED", "slot not allowed", "")
	case errors.Is(err, store.ErrInvalidInstanceState):
		return s.writeAPIError(c, fiber.StatusConflict, "CATTERY_KITTY_EQUIP_INSTANCE_STATE_INVALID", "instance state invalid", "")
	case errors.Is(err, store.ErrFishingTackleInUse):
		return s.writeAPIError(c, fiber.StatusConflict, "CATTERY_KITTY_EQUIP_FISHING_TACKLE_IN_USE", "fishing tackle is in use", "")
	case errors.Is(err, store.ErrAffixBodyMaxEffectiveExceeded):
		affixID := ""
		var limitErr *store.AffixLimitExceededError
		if errors.As(err, &limitErr) {
			affixID = strings.TrimSpace(limitErr.AffixID)
		}
		return s.writeAPIError(c, fiber.StatusConflict, "EQUIP_AFFIX_BODY_MAX_EFFECTIVE_EXCEEDED", "affix exceeds body max effective count", "", affixID)
	default:
		return s.writeAPIError(c, fiber.StatusInternalServerError, "CATTERY_KITTY_COMBAT_FAILED", "kitty combat operation failed", err.Error())
	}
}

func (s *Server) writeCatteryBattlePointError(c *fiber.Ctx, fallbackCode string, err error) error {
	switch {
	case errors.Is(err, store.ErrKittyNotFound):
		return s.writeAPIError(c, fiber.StatusNotFound, "CATTERY_KITTY_NOT_FOUND", "kitty not found", "")
	case errors.Is(err, store.ErrBattlePointInsufficient):
		return s.writeAPIError(c, fiber.StatusConflict, "CATTERY_BATTLE_POINT_INSUFFICIENT", "insufficient battle points", "")
	case errors.Is(err, store.ErrBattlePointAttrNotAllowed):
		return s.writeAPIError(c, fiber.StatusBadRequest, "CATTERY_BATTLE_POINT_ATTR_NOT_ALLOWED", "attribute does not allow battle points", "")
	case errors.Is(err, store.ErrInsufficientItem):
		return s.writeAPIError(c, fiber.StatusConflict, "CATTERY_BATTLE_POINT_RESET_ITEM_INSUFFICIENT", "insufficient reset item", "")
	case errors.Is(err, store.ErrBattleRoomConfigInvalid):
		return s.writeAPIError(c, fiber.StatusBadRequest, "CATTERY_BATTLE_POINT_INVALID_REQUEST", "invalid request", "")
	default:
		return s.writeAPIError(c, fiber.StatusInternalServerError, fallbackCode, "kitty battle point operation failed", err.Error())
	}
}

func (s *Server) catteryBattlePointStatePayload(state store.KittyBattlePointState) catteryBattlePointStatePayload {
	available := state.TotalPoints - state.SpentPoints
	if available < 0 {
		available = 0
	}
	resetRequirement := fiber.Map{
		"itemId": strings.TrimSpace(s.deps.Config.GlobalSettings.Battle.ResetItemID),
		"count":  s.deps.Config.GlobalSettings.Battle.ResetItemCount,
	}
	return catteryBattlePointStatePayload{
		KittyID:          state.KittyID,
		TotalPoints:      state.TotalPoints,
		SpentPoints:      state.SpentPoints,
		Available:        available,
		ResetRequirement: resetRequirement,
	}
}

func (s *Server) handleCatteryPublicKittyGet(c *fiber.Ctx) error {
	rawUserID := strings.TrimSpace(c.Query("userId"))
	rawKittyID := strings.TrimSpace(c.Query("kittyId"))
	userID, userErr := strconv.ParseInt(rawUserID, 10, 64)
	kittyID, kittyErr := strconv.ParseInt(rawKittyID, 10, 64)
	if userErr != nil || kittyErr != nil || userID <= 0 || kittyID <= 0 {
		return s.writeAPIError(c, fiber.StatusBadRequest, "CATTERY_PUBLIC_KITTY_INVALID_QUERY", "invalid userId or kittyId", "")
	}
	profile, err := s.deps.Store.GetPublicKittyProfile(c.Context(), userID, kittyID)
	if err != nil {
		if errors.Is(err, store.ErrKittyNotFound) {
			return s.writeAPIError(c, fiber.StatusNotFound, "CATTERY_PUBLIC_KITTY_NOT_FOUND", "kitty not found", "")
		}
		return s.writeAPIError(c, fiber.StatusInternalServerError, "CATTERY_PUBLIC_KITTY_GET_FAILED", "get public kitty failed", err.Error())
	}
	return s.writeAPISuccess(c, fiber.StatusOK, profile)
}
