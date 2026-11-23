package httpapi

import (
	"encoding/json"
	"math"
	"strings"

	"mysecondwebgame/server/internal/store"
	"mysecondwebgame/server/internal/wsbus"
)

const battleSettlementManifestVersionV1 = 1

type battleSettlementManifestV1 struct {
	Version int                                `json:"version"`
	Players []battleSettlementManifestPlayerV1 `json:"players"`
}

type battleSettlementManifestPlayerV1 struct {
	UnitID      string                            `json:"unitId"`
	UserID      int64                             `json:"userId"`
	UnitType    string                            `json:"unitType"`
	KittyID     int64                             `json:"kittyId"`
	Skills      []battleSettlementManifestSkillV1 `json:"skills"`
	BattleItems []battleSettlementManifestItemV1  `json:"battleItems"`
}

type battleSettlementManifestSkillV1 struct {
	ID    string `json:"id"`
	Level int    `json:"level"`
}

type battleSettlementManifestItemV1 struct {
	ItemID   string `json:"itemId"`
	Quantity int64  `json:"quantity"`
}

type battleSettlementRunPayload struct {
	Run          store.BattleRun
	Timeline     []wsbus.BattleTimelineEvent
	FinalResult  map[string]any
	UsedManifest bool
}

func buildBattleSettlementManifestV1(finalResult map[string]any) (battleSettlementManifestV1, bool) {
	carryState, ok := finalResult["carryState"].(map[string]any)
	if !ok || len(carryState) == 0 {
		return battleSettlementManifestV1{}, false
	}
	carryPlayers, ok := carryState["players"].([]battleCarryUnitState)
	if !ok || len(carryPlayers) == 0 {
		return battleSettlementManifestV1{}, false
	}
	manifest := battleSettlementManifestV1{
		Version: battleSettlementManifestVersionV1,
		Players: make([]battleSettlementManifestPlayerV1, 0, len(carryPlayers)),
	}
	for _, player := range carryPlayers {
		row := battleSettlementManifestPlayerV1{
			UnitID:      player.UnitID,
			UserID:      player.UserID,
			UnitType:    player.UnitType,
			KittyID:     player.KittyID,
			Skills:      make([]battleSettlementManifestSkillV1, 0, len(player.Skills)),
			BattleItems: make([]battleSettlementManifestItemV1, 0, len(player.BattleItems)),
		}
		for _, skill := range player.Skills {
			row.Skills = append(row.Skills, battleSettlementManifestSkillV1{ID: skill.ID, Level: skill.Level})
		}
		for _, item := range player.BattleItems {
			row.BattleItems = append(row.BattleItems, battleSettlementManifestItemV1{ItemID: item.ItemID, Quantity: item.Quantity})
		}
		manifest.Players = append(manifest.Players, row)
	}
	return manifest, true
}

func battleSettlementFinalResultFromSummary(summary map[string]any) (map[string]any, bool) {
	players, ok := decodeBattleSettlementManifestV1(summary["settlementManifestV1"])
	if !ok || !battleSettlementSummaryComplete(summary) {
		return nil, false
	}
	finalResult := make(map[string]any, len(summary)+1)
	for key, value := range summary {
		finalResult[key] = value
	}
	finalResult["carryState"] = map[string]any{"players": players}
	return finalResult, true
}

func battleSettlementSummaryComplete(summary map[string]any) bool {
	if len(summary) == 0 {
		return false
	}
	outcome, ok := summary["outcome"].(string)
	if !ok {
		return false
	}
	switch strings.TrimSpace(strings.ToLower(outcome)) {
	case store.BattleRunStatusEnded, store.BattleRunStatusFailed, store.BattleRunStatusInterrupted:
	default:
		return false
	}
	if !battleSettlementRewardPlanComplete(summary["rewardPlan"]) ||
		!battleSettlementStatsComplete(summary["battleStats"]) {
		return false
	}
	if _, ok := summary["compileTimedOut"].(bool); !ok {
		return false
	}
	if _, ok := summary["compileStepLimit"].(bool); !ok {
		return false
	}
	return true
}

func battleSettlementRewardPlanComplete(raw any) bool {
	encoded, err := json.Marshal(raw)
	if err != nil {
		return false
	}
	genericRows := []any(nil)
	if err := json.Unmarshal(encoded, &genericRows); err != nil || genericRows == nil {
		return false
	}
	for _, rawRow := range genericRows {
		row := normalizeAnyMap(rawRow)
		if row == nil {
			return false
		}
		for _, key := range []string{"userId", "itemRewards", "attrExp", "kittyAttrExp"} {
			if _, exists := row[key]; !exists {
				return false
			}
		}
	}
	rows := []battleRewardGrant{}
	if err := json.Unmarshal(encoded, &rows); err != nil || rows == nil {
		return false
	}
	seenUsers := make(map[int64]struct{}, len(rows))
	for _, row := range rows {
		if row.UserID <= 0 || row.ItemRewards == nil || row.AttrExp == nil || row.KittyAttrExp == nil {
			return false
		}
		if _, duplicate := seenUsers[row.UserID]; duplicate {
			return false
		}
		seenUsers[row.UserID] = struct{}{}
		if !battleSettlementRewardGrantValuesComplete(row) {
			return false
		}
	}
	return true
}

func battleSettlementRewardGrantValuesComplete(row battleRewardGrant) bool {
	if row.Contributions.Validate() != nil {
		return false
	}
	contributed := map[string]int64{}
	for _, r := range row.Contributions {
		if len(r.Experience) > 0 {
			return false
		}
		for id, n := range r.Items {
			contributed[id] += n
			if contributed[id] > row.ItemRewards[id] {
				return false
			}
		}
	}
	for itemID, quantity := range row.ItemRewards {
		if strings.TrimSpace(itemID) == "" || quantity <= 0 {
			return false
		}
	}
	if !battleSettlementFloatMapComplete(row.AttrExp) {
		return false
	}
	for kittyID, attrs := range row.KittyAttrExp {
		if kittyID <= 0 || !battleSettlementFloatMapComplete(attrs) {
			return false
		}
	}
	return true
}

func battleSettlementFloatMapComplete(values map[string]float64) bool {
	for key, value := range values {
		if strings.TrimSpace(key) == "" || value <= 0 || math.IsNaN(value) || math.IsInf(value, 0) {
			return false
		}
	}
	return true
}

func battleSettlementStatsComplete(raw any) bool {
	stats := normalizeAnyMap(raw)
	if stats == nil {
		return false
	}
	enemyKillsRaw, hasEnemyKills := stats["enemyKills"]
	skillCastsRaw, hasSkillCasts := stats["skillCastsByUnit"]
	if !hasEnemyKills || !hasSkillCasts {
		return false
	}
	if !battleSettlementCountMapComplete(enemyKillsRaw) {
		return false
	}
	castsByUnit := normalizeAnyMap(skillCastsRaw)
	if castsByUnit == nil {
		return false
	}
	for unitID, rawPerSkill := range castsByUnit {
		if strings.TrimSpace(unitID) == "" || !battleSettlementCountMapComplete(rawPerSkill) {
			return false
		}
	}
	return true
}

func battleSettlementCountMapComplete(raw any) bool {
	values := normalizeAnyMap(raw)
	if values == nil {
		return false
	}
	for key, rawCount := range values {
		if strings.TrimSpace(key) == "" {
			return false
		}
		if _, ok := battleSettlementNonnegativeInt64(rawCount); !ok {
			return false
		}
	}
	return true
}

func battleSettlementNonnegativeInt64(raw any) (int64, bool) {
	switch value := raw.(type) {
	case int:
		if value < 0 {
			return 0, false
		}
		return int64(value), true
	case int32:
		if value < 0 {
			return 0, false
		}
		return int64(value), true
	case int64:
		return value, value >= 0
	case float64:
		if value < 0 || math.IsNaN(value) || math.IsInf(value, 0) || value != math.Trunc(value) || value >= float64(math.MaxInt64) {
			return 0, false
		}
		return int64(value), true
	case json.Number:
		parsed, err := value.Int64()
		return parsed, err == nil && parsed >= 0
	default:
		return 0, false
	}
}

func decodeBattleSettlementManifestV1(raw any) ([]battleUnitState, bool) {
	if raw == nil {
		return nil, false
	}
	encoded, err := json.Marshal(raw)
	if err != nil {
		return nil, false
	}
	if !battleSettlementManifestShapeComplete(encoded) {
		return nil, false
	}
	manifest := battleSettlementManifestV1{}
	if err := json.Unmarshal(encoded, &manifest); err != nil || manifest.Version != battleSettlementManifestVersionV1 {
		return nil, false
	}
	players, ok := battleUnitsFromSettlementManifestV1(manifest)
	if !ok {
		return nil, false
	}
	return players, true
}

func battleSettlementManifestShapeComplete(encoded []byte) bool {
	root := map[string]any{}
	if err := json.Unmarshal(encoded, &root); err != nil {
		return false
	}
	if _, exists := root["version"]; !exists {
		return false
	}
	rawPlayers, ok := root["players"].([]any)
	if !ok || len(rawPlayers) == 0 {
		return false
	}
	for _, rawPlayer := range rawPlayers {
		player := normalizeAnyMap(rawPlayer)
		if player == nil {
			return false
		}
		for _, key := range []string{"unitId", "userId", "unitType", "kittyId", "skills", "battleItems"} {
			if _, exists := player[key]; !exists {
				return false
			}
		}
		rawSkills, ok := player["skills"].([]any)
		if !ok {
			return false
		}
		for _, rawSkill := range rawSkills {
			skill := normalizeAnyMap(rawSkill)
			if skill == nil {
				return false
			}
			if _, exists := skill["id"]; !exists {
				return false
			}
			if _, exists := skill["level"]; !exists {
				return false
			}
		}
		rawItems, ok := player["battleItems"].([]any)
		if !ok {
			return false
		}
		for _, rawItem := range rawItems {
			item := normalizeAnyMap(rawItem)
			if item == nil {
				return false
			}
			if _, exists := item["itemId"]; !exists {
				return false
			}
			if _, exists := item["quantity"]; !exists {
				return false
			}
		}
	}
	return true
}

func battleUnitsFromSettlementManifestV1(manifest battleSettlementManifestV1) ([]battleUnitState, bool) {
	if manifest.Version != battleSettlementManifestVersionV1 || len(manifest.Players) == 0 {
		return nil, false
	}
	players := make([]battleUnitState, 0, len(manifest.Players))
	seenUnits := make(map[string]struct{}, len(manifest.Players))
	for _, row := range manifest.Players {
		unitID := strings.TrimSpace(row.UnitID)
		unitType := strings.TrimSpace(row.UnitType)
		if unitID == "" || unitID != row.UnitID || unitType != row.UnitType || row.UserID < 0 || row.KittyID < 0 || row.Skills == nil || row.BattleItems == nil {
			return nil, false
		}
		if _, duplicate := seenUnits[unitID]; duplicate {
			return nil, false
		}
		seenUnits[unitID] = struct{}{}
		if unitType == "kitty" && (row.UserID <= 0 || row.KittyID <= 0) {
			return nil, false
		}
		player := battleUnitState{
			UnitID:      unitID,
			UserID:      row.UserID,
			UnitType:    unitType,
			KittyID:     row.KittyID,
			Skills:      make([]battleSkillState, 0, len(row.Skills)),
			BattleItems: make([]battleItemState, 0, len(row.BattleItems)),
		}
		for _, skill := range row.Skills {
			skillID := strings.TrimSpace(skill.ID)
			if skillID == "" || skillID != skill.ID || skill.Level <= 0 {
				return nil, false
			}
			player.Skills = append(player.Skills, battleSkillState{ID: skillID, Level: skill.Level})
		}
		for _, item := range row.BattleItems {
			itemID := strings.TrimSpace(item.ItemID)
			if itemID == "" || itemID != item.ItemID || item.Quantity < 0 {
				return nil, false
			}
			player.BattleItems = append(player.BattleItems, battleItemState{ItemID: itemID, Quantity: item.Quantity})
		}
		players = append(players, player)
	}
	return players, true
}

func resolveBattleSettlementRun(
	minimal store.BattleRun,
	loadFull func() (store.BattleRun, error),
) (battleSettlementRunPayload, error) {
	if finalResult, ok := battleSettlementFinalResultFromSummary(minimal.FinalResult); ok {
		minimal.FinalResult = finalResult
		return battleSettlementRunPayload{
			Run:          minimal,
			Timeline:     []wsbus.BattleTimelineEvent{},
			FinalResult:  finalResult,
			UsedManifest: true,
		}, nil
	}
	full, err := loadFull()
	if err != nil {
		return battleSettlementRunPayload{}, err
	}
	finalResult := full.FinalResult
	if finalResult == nil {
		finalResult = map[string]any{}
	}
	return battleSettlementRunPayload{
		Run:         full,
		Timeline:    decodeBattleTimeline(full.Timeline),
		FinalResult: finalResult,
	}, nil
}
