package httpapi

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"mysecondwebgame/server/internal/store"

	"github.com/gofiber/fiber/v2"
)

var errCompetitiveEquipmentSnapshotUnstable = errors.New("equipment changed during competitive lineup snapshot assembly")

const competitiveAlchemyEligibilityPolicyVersion = "arena-alchemy-v1"

func competitiveLineupDigest(lineup []battleUnitState) (string, json.RawMessage, error) {
	raw, err := json.Marshal(lineup)
	if err != nil {
		return "", nil, err
	}
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:]), raw, nil
}

func randomCompetitiveBattleSeed() (int64, error) {
	var buf [8]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return 0, err
	}
	seed := int64(binary.BigEndian.Uint64(buf[:]) & uint64(^uint64(0)>>1))
	if seed == 0 {
		seed = 1
	}
	return seed, nil
}

func randomCompetitiveReplayShareToken() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw), nil
}

type competitiveLineupKittySelection struct {
	KittyID  int64  `json:"kittyId"`
	Position string `json:"position"`
}

type competitiveLineupSelection struct {
	PlayerPosition string                            `json:"playerPosition"`
	Kitties        []competitiveLineupKittySelection `json:"kitties"`
}

type competitiveLineupSnapshotOptions struct {
	SpecializationEnabled bool
}

type competitiveAlchemyEligibilityViolation struct {
	UnitType              string  `json:"unitType"`
	UnitID                int64   `json:"unitId"`
	UnitName              string  `json:"unitName"`
	EquipmentInstanceID   int64   `json:"equipmentInstanceId"`
	TemplateItemID        string  `json:"templateItemId"`
	EquipmentName         string  `json:"equipmentName,omitempty"`
	EquippedSlot          string  `json:"equippedSlot,omitempty"`
	AffixSlotIndex        int     `json:"affixSlotIndex"`
	AlchemyRulesetVersion string  `json:"alchemyRulesetVersion,omitempty"`
	Attribute             string  `json:"attribute"`
	ActualValue           float64 `json:"actualValue"`
	AllowedValue          float64 `json:"allowedValue"`
	Reason                string  `json:"reason"`
}

type competitiveAlchemyEligibilityError struct {
	Violations []competitiveAlchemyEligibilityViolation
}

func normalizeCompetitiveDefinitionID(raw string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	if value == "" || value == "all" || len(value) > 64 {
		return ""
	}
	for index, char := range value {
		if (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') || (index > 0 && (char == '_' || char == ':' || char == '-')) {
			continue
		}
		return ""
	}
	return value
}

func normalizeCompetitivePosition(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "vanguard", "midguard", "rearguard":
		return strings.ToLower(strings.TrimSpace(raw))
	default:
		return ""
	}
}

func (e *competitiveAlchemyEligibilityError) Error() string {
	return "competitive lineup contains restricted alchemy equipment"
}

func normalizeCompetitiveLineupSelection(selection competitiveLineupSelection) (competitiveLineupSelection, error) {
	selection.PlayerPosition = normalizeCompetitivePosition(selection.PlayerPosition)
	if selection.PlayerPosition == "" || len(selection.Kitties) > store.CompetitiveMaxKittyCount {
		return competitiveLineupSelection{}, store.ErrBattleRoomConfigInvalid
	}
	if selection.Kitties == nil {
		selection.Kitties = []competitiveLineupKittySelection{}
	}
	seenKitties := map[int64]struct{}{}
	for index := range selection.Kitties {
		selection.Kitties[index].Position = normalizeCompetitivePosition(selection.Kitties[index].Position)
		if selection.Kitties[index].KittyID <= 0 || selection.Kitties[index].Position == "" {
			return competitiveLineupSelection{}, store.ErrBattleRoomConfigInvalid
		}
		if _, exists := seenKitties[selection.Kitties[index].KittyID]; exists {
			return competitiveLineupSelection{}, store.ErrBattleRoomConfigInvalid
		}
		seenKitties[selection.Kitties[index].KittyID] = struct{}{}
	}
	return selection, nil
}

func appendCompetitiveAlchemyUnitViolations(target []competitiveAlchemyEligibilityViolation, unitType string, unitID int64, unitName string, violations []store.ArenaAlchemyEquipmentViolation) []competitiveAlchemyEligibilityViolation {
	for _, violation := range violations {
		target = append(target, competitiveAlchemyEligibilityViolation{
			UnitType:              unitType,
			UnitID:                unitID,
			UnitName:              strings.TrimSpace(unitName),
			EquipmentInstanceID:   violation.EquipmentInstanceID,
			TemplateItemID:        violation.TemplateItemID,
			EquipmentName:         violation.EquipmentName,
			EquippedSlot:          violation.EquippedSlot,
			AffixSlotIndex:        violation.AffixSlotIndex,
			AlchemyRulesetVersion: violation.AlchemyRulesetVersion,
			Attribute:             violation.Attribute,
			ActualValue:           violation.ActualValue,
			AllowedValue:          violation.AllowedValue,
			Reason:                violation.Reason,
		})
	}
	return target
}

func (s *Server) validateCompetitiveLineupAlchemyEquipment(lineup []battleUnitState) error {
	violations := make([]competitiveAlchemyEligibilityViolation, 0)
	for _, unit := range lineup {
		unitType := strings.ToLower(strings.TrimSpace(unit.UnitType))
		unitID := unit.UserID
		if unitType == "kitty" {
			unitID = unit.KittyID
		}
		violations = appendCompetitiveAlchemyUnitViolations(violations, unitType, unitID, unit.Name, s.deps.Store.ArenaAlchemyEquipmentViolations(unit.equipmentSnapshotItems))
	}
	if len(violations) > 0 {
		return &competitiveAlchemyEligibilityError{Violations: violations}
	}
	return nil
}

func competitiveEquipmentSnapshotDigest(items []store.EquippedItem) (string, error) {
	battleItems := make([]store.EquippedItem, 0, len(items))
	for _, item := range items {
		if strings.EqualFold(strings.TrimSpace(item.EquipType), "battle") {
			battleItems = append(battleItems, item)
		}
	}
	sort.SliceStable(battleItems, func(i, j int) bool {
		if battleItems[i].InstanceID != battleItems[j].InstanceID {
			return battleItems[i].InstanceID < battleItems[j].InstanceID
		}
		return battleItems[i].EquippedSlot < battleItems[j].EquippedSlot
	})
	raw, err := json.Marshal(battleItems)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:]), nil
}

func competitiveEquipmentSnapshotsMatch(before, captured []store.EquippedItem) bool {
	beforeDigest, err := competitiveEquipmentSnapshotDigest(before)
	if err != nil {
		return false
	}
	capturedDigest, err := competitiveEquipmentSnapshotDigest(captured)
	return err == nil && beforeDigest == capturedDigest
}

func (s *Server) buildCompetitivePlayerSnapshot(ctx context.Context, member store.BattleRoomMember, now time.Time, options competitiveLineupSnapshotOptions) (battleUnitState, error) {
	for attempt := 0; attempt < battlePlayerSnapshotMaxAttempts; attempt++ {
		before, err := s.deps.Store.ListEquippedItems(ctx, member.UserID)
		if err != nil {
			return battleUnitState{}, fmt.Errorf("load competitive player equipment fence: %w", err)
		}
		var unit battleUnitState
		if options.SpecializationEnabled {
			unit, err = s.buildBattlePlayerSnapshot(ctx, member, now)
		} else {
			unit, err = s.buildBattlePlayerSnapshotWithoutSpecialization(ctx, member, now)
		}
		if err != nil {
			return battleUnitState{}, err
		}
		if competitiveEquipmentSnapshotsMatch(before, unit.equipmentSnapshotItems) {
			return unit, nil
		}
	}
	return battleUnitState{}, errCompetitiveEquipmentSnapshotUnstable
}

func (s *Server) buildCompetitiveKittySnapshot(ctx context.Context, roomUnit store.BattleRoomUnit, now time.Time) (battleUnitState, error) {
	if roomUnit.KittyID == nil || *roomUnit.KittyID <= 0 {
		return battleUnitState{}, store.ErrKittyNotFound
	}
	for attempt := 0; attempt < battlePlayerSnapshotMaxAttempts; attempt++ {
		before, err := s.deps.Store.ListKittyEquippedItems(ctx, roomUnit.OwnerUserID, *roomUnit.KittyID)
		if err != nil {
			return battleUnitState{}, fmt.Errorf("load competitive kitty equipment fence: %w", err)
		}
		unit, err := s.buildBattleKittySnapshot(ctx, roomUnit, now)
		if err != nil {
			return battleUnitState{}, err
		}
		if competitiveEquipmentSnapshotsMatch(before, unit.equipmentSnapshotItems) {
			return unit, nil
		}
	}
	return battleUnitState{}, errCompetitiveEquipmentSnapshotUnstable
}

func (s *Server) buildCompetitiveLineupSnapshot(ctx context.Context, userID int64, selection competitiveLineupSelection, now time.Time, options competitiveLineupSnapshotOptions) (competitiveLineupSelection, []battleUnitState, error) {
	selection, err := normalizeCompetitiveLineupSelection(selection)
	if err != nil {
		return competitiveLineupSelection{}, nil, err
	}
	for index := range selection.Kitties {
		lockedPosition, _, _, err := s.deps.Store.GetOwnedKittyPositionLock(ctx, userID, selection.Kitties[index].KittyID)
		if err != nil {
			return competitiveLineupSelection{}, nil, err
		}
		if normalized := normalizeCompetitivePosition(lockedPosition); normalized != "" {
			selection.Kitties[index].Position = normalized
		}
	}
	selection, err = normalizeCompetitiveLineupSelection(selection)
	if err != nil {
		return competitiveLineupSelection{}, nil, err
	}
	member := store.BattleRoomMember{UserID: userID, Position: selection.PlayerPosition}
	player, err := s.buildCompetitivePlayerSnapshot(ctx, member, now, options)
	if err != nil {
		return competitiveLineupSelection{}, nil, err
	}
	player.RewardEligible = false
	player.BattleItems = []battleItemState{}
	player.ItemRules = map[string]battleSkillRuleState{}
	lineup := []battleUnitState{player}
	for _, selected := range selection.Kitties {
		kitty, err := s.deps.Store.GetPlayerKitty(ctx, userID, selected.KittyID)
		if err != nil {
			return competitiveLineupSelection{}, nil, err
		}
		kittyID := selected.KittyID
		unit, err := s.buildCompetitiveKittySnapshot(ctx, store.BattleRoomUnit{
			OwnerUserID: userID,
			UnitType:    "kitty",
			KittyID:     &kittyID,
			KittyName:   kitty.Name,
			Position:    selected.Position,
		}, now)
		if err != nil {
			return competitiveLineupSelection{}, nil, err
		}
		unit.RewardEligible = false
		unit.BattleItems = []battleItemState{}
		unit.ItemRules = map[string]battleSkillRuleState{}
		lineup = append(lineup, unit)
	}
	if err := s.validateCompetitiveLineupAlchemyEquipment(lineup); err != nil {
		return competitiveLineupSelection{}, nil, err
	}
	for index := range lineup {
		lineup[index].equipmentSnapshotItems = nil
	}
	return selection, lineup, nil
}

func competitiveRuntimeLineup(raw json.RawMessage, side, prefix string) ([]battleUnitState, error) {
	var units []battleUnitState
	if err := json.Unmarshal(raw, &units); err != nil {
		return nil, fmt.Errorf("decode competitive lineup: %w", err)
	}
	if len(units) < 1 || len(units) > 1+store.CompetitiveMaxKittyCount {
		return nil, fmt.Errorf("competitive lineup has %d units, want 1 to %d", len(units), 1+store.CompetitiveMaxKittyCount)
	}
	seenPlayers := 0
	seenKitties := 0
	seenIDs := make(map[string]struct{}, len(units))
	for index := range units {
		unit := &units[index]
		unitType := strings.ToLower(strings.TrimSpace(unit.UnitType))
		if battleUnitIsSummon(unit) || (unitType != "player" && unitType != "kitty") || unit.UserID <= 0 {
			return nil, fmt.Errorf("competitive lineup contains invalid formal unit")
		}
		if unitType == "player" {
			seenPlayers++
		} else {
			seenKitties++
			if unit.KittyID <= 0 {
				return nil, fmt.Errorf("competitive lineup kitty id is missing")
			}
		}
		identity := fmt.Sprintf("%s:%s:%d", prefix, unitType, unit.UserID)
		if unitType == "kitty" {
			identity = fmt.Sprintf("%s:kitty:%d", prefix, unit.KittyID)
		}
		if _, exists := seenIDs[identity]; exists {
			return nil, fmt.Errorf("competitive lineup contains duplicate unit")
		}
		seenIDs[identity] = struct{}{}
		unit.UnitID = identity
		unit.Side = side
		unit.OriginSide = side
		unit.BaseSide = side
		unit.PerspectiveSide = side
		unit.IsPlayerUnit = true
		unit.RewardEligible = false
		unit.BattleItems = []battleItemState{}
		unit.ItemRules = map[string]battleSkillRuleState{}
		unit.BattleLevel = maxInt64(1, unit.BattleLevel)
		unit.Alive = true
		unit.HP = unit.MaxHP
		unit.MP = unit.MaxMP
		normalizeBattleUnitState(unit)
	}
	if seenPlayers != 1 || seenKitties > store.CompetitiveMaxKittyCount {
		return nil, fmt.Errorf("competitive lineup must contain one player and up to %d kitties", store.CompetitiveMaxKittyCount)
	}
	return units, nil
}

func competitivePublicUnitSlice(rows []battleUnitState) []fiber.Map {
	out := battlePublicUnitSlice(rows)
	for index := range out {
		out[index]["unitType"] = rows[index].UnitType
		out[index]["kittyId"] = rows[index].KittyID
		out[index]["battleLevel"] = rows[index].BattleLevel
	}
	return out
}

func (s *Server) battleTriggerCompetitiveWaveStartHooks(state *battleRuntimeState, now time.Time) []map[string]any {
	if state == nil {
		return nil
	}
	state.SpecializationHookWaveCounts = map[string]int{}
	unitIDs := make([]string, 0, len(state.Players)+len(state.Enemies))
	for index := range state.Players {
		unitIDs = append(unitIDs, state.Players[index].UnitID)
	}
	for index := range state.Enemies {
		unitIDs = append(unitIDs, state.Enemies[index].UnitID)
	}
	actions := make([]map[string]any, 0, len(unitIDs))
	for _, unitID := range unitIDs {
		unit := battleUnitByID(state, unitID)
		if unit == nil || !unit.Alive || unit.HP <= 0 {
			continue
		}
		actions = append(actions, s.battleTriggerUnitEquipmentHooks(state, now, unit, nil, battleEquipmentTriggerWaveStart, float64(state.Wave), "")...)
	}
	return actions
}
