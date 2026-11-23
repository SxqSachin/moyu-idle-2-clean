package httpapi

import (
	"fmt"
	"sort"
	"strings"

	"mysecondwebgame/server/internal/store"
)

type battleCarryRepairReport struct {
	DroppedEmptyUnitIDs       int
	DroppedDuplicateUnitIDs   int
	DroppedUnknownFormalUnits int
	DroppedFormalUnitsOverCap int
	DroppedInvalidSummons     int
	DroppedNonPersistent      int
	DroppedExcessSummons      int
	RestoredFormalIdentity    int
	RestoredSummonMetadata    int
}

func (report battleCarryRepairReport) changed() bool {
	return report.DroppedEmptyUnitIDs > 0 ||
		report.DroppedDuplicateUnitIDs > 0 ||
		report.DroppedUnknownFormalUnits > 0 ||
		report.DroppedFormalUnitsOverCap > 0 ||
		report.DroppedInvalidSummons > 0 ||
		report.DroppedNonPersistent > 0 ||
		report.DroppedExcessSummons > 0 ||
		report.RestoredFormalIdentity > 0 ||
		report.RestoredSummonMetadata > 0
}

type battleCarryFormalIdentity struct {
	UnitID       string
	UnitType     string
	UserID       int64
	KittyID      int64
	WithinRoster bool
}

func battleCarryFormalRoster(detail store.BattleRoomDetail, maxTeamSize int) map[string]battleCarryFormalIdentity {
	roster := map[string]battleCarryFormalIdentity{}
	accepted := 0
	appendIdentity := func(identity battleCarryFormalIdentity) {
		key := strings.ToLower(strings.TrimSpace(identity.UnitID))
		if key == "" {
			return
		}
		if _, exists := roster[key]; exists {
			return
		}
		identity.WithinRoster = maxTeamSize <= 0 || accepted < maxTeamSize
		roster[key] = identity
		accepted++
	}

	for _, roomUnit := range detail.Units {
		if strings.EqualFold(strings.TrimSpace(roomUnit.UnitType), "kitty") {
			kittyID := roomUnit.UnitID
			if roomUnit.KittyID != nil {
				kittyID = *roomUnit.KittyID
			}
			if kittyID <= 0 {
				continue
			}
			appendIdentity(battleCarryFormalIdentity{
				UnitID:   fmt.Sprintf("kitty:%d", kittyID),
				UnitType: "kitty",
				UserID:   roomUnit.OwnerUserID,
				KittyID:  kittyID,
			})
			continue
		}
		if roomUnit.OwnerUserID <= 0 {
			continue
		}
		appendIdentity(battleCarryFormalIdentity{
			UnitID:   fmt.Sprintf("player:%d", roomUnit.OwnerUserID),
			UnitType: "player",
			UserID:   roomUnit.OwnerUserID,
		})
	}
	if len(roster) > 0 {
		return roster
	}
	for _, member := range detail.Members {
		if member.UserID <= 0 {
			continue
		}
		appendIdentity(battleCarryFormalIdentity{
			UnitID:   fmt.Sprintf("player:%d", member.UserID),
			UnitType: "player",
			UserID:   member.UserID,
		})
	}
	return roster
}

func battleRestoreSummonBindingMetadata(unit *battleUnitState) bool {
	if unit == nil {
		return false
	}
	changed := false
	for index := range unit.Buffs {
		binding := &unit.Buffs[index]
		if !strings.EqualFold(strings.TrimSpace(binding.BuffID), battleSummonBindingBuffID) {
			continue
		}
		payload := binding.Payload
		if strings.TrimSpace(unit.SummonTemplateUnitID) == "" {
			unit.SummonTemplateUnitID = strings.TrimSpace(battleStringFromAny(payload["templateUnitId"]))
			changed = changed || unit.SummonTemplateUnitID != ""
		}
		if strings.TrimSpace(unit.SummonSourceUnitID) == "" {
			unit.SummonSourceUnitID = strings.TrimSpace(battleStringFromAny(payload["sourceUnitId"]))
			if unit.SummonSourceUnitID == "" {
				unit.SummonSourceUnitID = strings.TrimSpace(binding.SourceID)
			}
			changed = changed || unit.SummonSourceUnitID != ""
		}
		if unit.SummonMaxCountPerUnit <= 0 {
			unit.SummonMaxCountPerUnit = maxInt(0, int(toInt64Loose(payload["maxCountPerUnit"], 0)))
			changed = changed || unit.SummonMaxCountPerUnit > 0
		}
		if !unit.SummonPersistAcrossBattles && parseBooleanLoose(payload["persistAcrossBattles"], false) {
			unit.SummonPersistAcrossBattles = true
			changed = true
		}
		if strings.TrimSpace(unit.SummonOnCasterDeath) == "" && payload["onCasterDeath"] != nil {
			unit.SummonOnCasterDeath = normalizeBattleSummonOnCasterDeath(battleStringFromAny(payload["onCasterDeath"]))
			changed = true
		}
		if strings.TrimSpace(unit.SummonOnCapOverflow) == "" && payload["onCapOverflow"] != nil {
			unit.SummonOnCapOverflow = normalizeBattleSummonOnCapOverflow(battleStringFromAny(payload["onCapOverflow"]))
			changed = true
		}
		if strings.TrimSpace(unit.SummonOnDispel) == "" && payload["onDispel"] != nil {
			unit.SummonOnDispel = normalizeBattleSummonOnDispel(battleStringFromAny(payload["onDispel"]))
			changed = true
		}
		if strings.TrimSpace(unit.SummonOnDurationEnd) == "" && payload["onDurationEnd"] != nil {
			unit.SummonOnDurationEnd = normalizeBattleSummonOnDurationEnd(battleStringFromAny(payload["onDurationEnd"]))
			changed = true
		}
		if unit.SummonSourceEnhanceLevel <= 0 {
			unit.SummonSourceEnhanceLevel = maxInt(0, int(toInt64Loose(payload["sourceEnhanceLevel"], 0)))
		}
		if unit.SummonOrder <= 0 {
			unit.SummonOrder = maxInt64(0, toInt64Loose(payload["summonOrder"], 0))
			changed = changed || unit.SummonOrder > 0
		}
		if unit.SummonRemainTurns == 0 && binding.RemainTurns > 0 {
			unit.SummonRemainTurns = binding.RemainTurns
			changed = true
		}
		break
	}
	unit.SummonTemplateUnitID = strings.TrimSpace(unit.SummonTemplateUnitID)
	unit.SummonSourceUnitID = strings.TrimSpace(unit.SummonSourceUnitID)
	return changed
}

func battlePersistentSummonCapsForSource(source *battleUnitState) map[string]int {
	if source == nil {
		return nil
	}
	caps := map[string]int{}
	applyHooks := func(hooks []battleEquipmentHookRule) {
		for _, hook := range hooks {
			for _, effect := range hook.Effects {
				kind := battleNormalizeTrigger(battleStringFromAny(firstNonNil(effect["kind"], effect["type"])))
				templateID := ""
				cap := 0
				switch kind {
				case "summon":
					payload := parseBattleSkillSummonPayload(firstNonNil(effect["summonPayload"], effect["summon"], effect))
					if !payload.PersistAcrossBattles {
						continue
					}
					templateID = payload.UnitID
					cap = battleResolveSummonMaxCount(source, 1, payload, hook.SourceEnhanceLevel)
				case "playerclone":
					payload := parseBattlePlayerClonePayload(effect)
					if !payload.PersistAcrossBattles {
						continue
					}
					templateID = battlePlayerCloneTemplateID
					cap = payload.MaxCountPerUnit
				}
				key := strings.ToLower(strings.TrimSpace(templateID))
				if key != "" && cap > caps[key] {
					caps[key] = cap
				}
			}
		}
	}
	applyHooks(source.EquipmentHooks)
	applyHooks(source.SpecializationHooks)
	applyHooks(source.KittyPersonalityHooks)
	applyEffects := func(effects []battleSkillEffect, skillLevel int, sourceEnhanceLevel int) {
		for _, effect := range effects {
			if battleNormalizeTrigger(effect.Kind) != "summon" || !effect.SummonPayload.PersistAcrossBattles {
				continue
			}
			key := strings.ToLower(strings.TrimSpace(effect.SummonPayload.UnitID))
			cap := battleResolveSummonMaxCount(source, maxInt(1, skillLevel), effect.SummonPayload, sourceEnhanceLevel)
			if key != "" && cap > caps[key] {
				caps[key] = cap
			}
		}
	}
	for _, skill := range source.Skills {
		applyEffects(skill.Effects, skill.Level, 0)
	}
	for _, item := range source.BattleItems {
		applyEffects(item.Effects, 1, 0)
	}
	if source.BasicAttack != nil {
		applyEffects(source.BasicAttack.Effects, 1, source.BasicAttack.SourceEnhanceLevel)
	}
	if len(caps) == 0 {
		return nil
	}
	return caps
}

func battleSyncSummonCapMetadata(unit *battleUnitState, cap int) {
	if unit == nil || cap <= 0 {
		return
	}
	unit.SummonMaxCountPerUnit = cap
	for index := range unit.Buffs {
		binding := &unit.Buffs[index]
		if !strings.EqualFold(strings.TrimSpace(binding.BuffID), battleSummonBindingBuffID) {
			continue
		}
		if binding.Payload == nil {
			binding.Payload = map[string]any{}
		}
		binding.Payload["maxCountPerUnit"] = cap
		return
	}
}

func battleRepairPreviousWavePlayers(
	detail store.BattleRoomDetail,
	players []battleUnitState,
	maxTeamSize int,
) ([]battleUnitState, battleCarryRepairReport, error) {
	report := battleCarryRepairReport{}
	roster := battleCarryFormalRoster(detail, maxTeamSize)
	if len(roster) == 0 {
		return nil, report, fmt.Errorf("%w: battle room formal roster is empty", errBattlePreviousWaveCarryInvalid)
	}

	repaired := make([]battleUnitState, 0, len(players))
	seenUnitIDs := make(map[string]struct{}, len(players))
	formalCount := 0
	for index := range players {
		unit := players[index]
		normalizeBattleUnitState(&unit)
		unitID := strings.TrimSpace(unit.UnitID)
		if unitID == "" {
			report.DroppedEmptyUnitIDs++
			continue
		}
		unit.UnitID = unitID
		unitKey := strings.ToLower(unitID)
		if _, duplicate := seenUnitIDs[unitKey]; duplicate {
			report.DroppedDuplicateUnitIDs++
			continue
		}

		summonMetadataRestored := battleRestoreSummonBindingMetadata(&unit)
		isSummon := battleUnitIsSummon(&unit) || strings.HasPrefix(unitKey, "summon_")
		if isSummon {
			if summonMetadataRestored {
				report.RestoredSummonMetadata++
			}
			if unit.SummonTemplateUnitID == "" || unit.SummonSourceUnitID == "" || !unit.Alive || unit.HP <= 0 {
				report.DroppedInvalidSummons++
				continue
			}
			if !unit.SummonPersistAcrossBattles {
				report.DroppedNonPersistent++
				continue
			}
			unit.IsPlayerUnit = false
			unit.UnitType = ""
			unit.UserID = 0
			unit.KittyID = 0
			unit.RewardEligible = false
			repaired = append(repaired, unit)
			seenUnitIDs[unitKey] = struct{}{}
			continue
		}

		identity, exists := roster[unitKey]
		if !exists {
			report.DroppedUnknownFormalUnits++
			continue
		}
		if !identity.WithinRoster {
			report.DroppedFormalUnitsOverCap++
			continue
		}
		if unit.UnitID != identity.UnitID || !unit.IsPlayerUnit || unit.UnitType != identity.UnitType ||
			unit.UserID != identity.UserID || unit.KittyID != identity.KittyID {
			report.RestoredFormalIdentity++
		}
		unit.UnitID = identity.UnitID
		unit.IsPlayerUnit = true
		unit.UnitType = identity.UnitType
		unit.UserID = identity.UserID
		unit.KittyID = identity.KittyID
		repaired = append(repaired, unit)
		seenUnitIDs[unitKey] = struct{}{}
		formalCount++
	}
	if formalCount == 0 {
		return nil, report, fmt.Errorf("%w: carry state has no room roster units", errBattlePreviousWaveCarryInvalid)
	}

	sources := make(map[string]*battleUnitState, formalCount)
	for index := range repaired {
		unit := &repaired[index]
		if !battleUnitIsSummon(unit) {
			sources[strings.ToLower(strings.TrimSpace(unit.UnitID))] = unit
		}
	}
	declaredCaps := make(map[string]map[string]int, len(sources))
	for sourceID, source := range sources {
		declaredCaps[sourceID] = battlePersistentSummonCapsForSource(source)
	}

	type summonGroup struct {
		indices []int
		cap     int
	}
	groups := map[string]*summonGroup{}
	for index := range repaired {
		unit := &repaired[index]
		if !battleUnitIsSummon(unit) {
			continue
		}
		sourceID := strings.ToLower(strings.TrimSpace(unit.SummonSourceUnitID))
		templateID := strings.ToLower(strings.TrimSpace(unit.SummonTemplateUnitID))
		groupKey := sourceID + "\x00" + templateID
		group := groups[groupKey]
		if group == nil {
			group = &summonGroup{}
			groups[groupKey] = group
		}
		group.indices = append(group.indices, index)
		if cap := declaredCaps[sourceID][templateID]; cap > 0 {
			group.cap = cap
		} else if unit.SummonMaxCountPerUnit > group.cap {
			group.cap = unit.SummonMaxCountPerUnit
		}
	}

	drop := make(map[int]struct{})
	for _, group := range groups {
		if group.cap <= 0 {
			continue
		}
		for _, index := range group.indices {
			battleSyncSummonCapMetadata(&repaired[index], group.cap)
		}
		if len(group.indices) <= group.cap {
			continue
		}
		newestFirst := append([]int(nil), group.indices...)
		sort.SliceStable(newestFirst, func(i, j int) bool {
			left := repaired[newestFirst[i]].SummonOrder
			right := repaired[newestFirst[j]].SummonOrder
			if left != right {
				return left > right
			}
			return newestFirst[i] > newestFirst[j]
		})
		for _, index := range newestFirst[group.cap:] {
			drop[index] = struct{}{}
			report.DroppedExcessSummons++
		}
	}
	if len(drop) == 0 {
		return repaired, report, nil
	}
	bounded := make([]battleUnitState, 0, len(repaired)-len(drop))
	for index := range repaired {
		if _, shouldDrop := drop[index]; shouldDrop {
			continue
		}
		bounded = append(bounded, repaired[index])
	}
	return bounded, report, nil
}
