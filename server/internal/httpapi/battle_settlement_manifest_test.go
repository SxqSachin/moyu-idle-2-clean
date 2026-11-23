package httpapi

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"mysecondwebgame/server/internal/store"
)

type settlementManifestItemWriter struct {
	updates []store.BattleItemBagQuantityUpdate
}

func (w *settlementManifestItemWriter) SetBattleItemBagQuantities(_ context.Context, updates []store.BattleItemBagQuantityUpdate) error {
	w.updates = append(w.updates, updates...)
	return nil
}

func battleSettlementManifestTestFinalResult() map[string]any {
	return map[string]any{
		"outcome": store.BattleRunStatusEnded,
		"rewardPlan": []battleRewardGrant{
			{
				UserID:       101,
				ItemRewards:  map[string]int64{"slime_drop": 3},
				AttrExp:      map[string]float64{"strength": 1.25},
				KittyAttrExp: map[int64]map[string]float64{501: {"agility": 0.5}},
			},
		},
		"battleStats": map[string]any{
			"enemyKills": map[string]int64{"slime_001": 2},
			"skillCastsByUnit": map[string]map[string]int64{
				"player:101": {"skill_alpha": 2},
				"kitty:501":  {"skill_beta": 1},
			},
		},
		"compileTimedOut":  false,
		"compileStepLimit": false,
		"carryState": map[string]any{
			"players": []battleCarryUnitState{
				{
					UnitID:   "player:101",
					UserID:   101,
					UnitType: "player",
					Skills: []battleSkillState{
						{ID: "skill_alpha", Name: "large discarded skill payload", Level: 3},
					},
					BattleItems: []battleItemState{
						{ItemID: "potion_a", Name: "large discarded item payload", Quantity: 0},
					},
				},
				{
					UnitID:   "kitty:501",
					UserID:   101,
					UnitType: "kitty",
					KittyID:  501,
					Skills: []battleSkillState{
						{ID: "skill_beta", Level: 4},
					},
					BattleItems: []battleItemState{
						{ItemID: "kitty_snack", Quantity: 2},
					},
				},
				{
					UnitID:      "summon:persistent:1",
					Skills:      []battleSkillState{{ID: "summon_skill", Level: 1}},
					BattleItems: []battleItemState{},
				},
			},
		},
	}
}

func battleSettlementManifestTestSummary(t *testing.T) map[string]any {
	t.Helper()
	summary := battleRunResultSummary(battleSettlementManifestTestFinalResult())
	raw, err := json.Marshal(summary)
	if err != nil {
		t.Fatal(err)
	}
	decoded := map[string]any{}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	return decoded
}

func cloneSettlementManifestTestSummary(t *testing.T, summary map[string]any) map[string]any {
	t.Helper()
	raw, err := json.Marshal(summary)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]any{}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestBattleRunResultSummaryBuildsMinimalSettlementManifestFromCarryPlayers(t *testing.T) {
	summary := battleSettlementManifestTestSummary(t)
	manifest := normalizeAnyMap(summary["settlementManifestV1"])
	if got := toInt64Loose(manifest["version"], 0); got != battleSettlementManifestVersionV1 {
		t.Fatalf("manifest version=%d", got)
	}
	players := battleAnyMapSlice(manifest["players"])
	if len(players) != 3 {
		t.Fatalf("manifest players=%d, want all carry players", len(players))
	}
	wantPlayerKeys := map[string]struct{}{
		"unitId": {}, "userId": {}, "unitType": {}, "kittyId": {}, "skills": {}, "battleItems": {},
	}
	for _, player := range players {
		if len(player) != len(wantPlayerKeys) {
			t.Fatalf("manifest leaked non-settlement fields: %#v", player)
		}
		for key := range player {
			if _, ok := wantPlayerKeys[key]; !ok {
				t.Fatalf("unexpected manifest player field %q", key)
			}
		}
		for _, skill := range battleAnyMapSlice(player["skills"]) {
			if len(skill) != 2 || skill["id"] == nil || skill["level"] == nil {
				t.Fatalf("skill manifest is not minimal: %#v", skill)
			}
		}
		for _, item := range battleAnyMapSlice(player["battleItems"]) {
			if len(item) != 2 || item["itemId"] == nil || item["quantity"] == nil {
				t.Fatalf("item manifest is not minimal: %#v", item)
			}
		}
	}
}

func TestBattleSettlementManifestUsesCarryStateSummonFiltering(t *testing.T) {
	state := battleRuntimeState{Players: []battleUnitState{
		{UnitID: "player:1", UserID: 1, UnitType: "player", Skills: []battleSkillState{{ID: "skill_a", Level: 1}}},
		{
			UnitID: "summon:transient", Alive: true, HP: 10,
			SummonTemplateUnitID: "transient", SummonSourceUnitID: "player:1",
			Skills: []battleSkillState{{ID: "skill_b", Level: 1}},
		},
		{
			UnitID: "summon:persistent", Alive: true, HP: 10,
			SummonTemplateUnitID: "persistent", SummonSourceUnitID: "player:1", SummonPersistAcrossBattles: true,
			Skills: []battleSkillState{{ID: "skill_c", Level: 1}},
		},
	}}
	finalResult := battleSettlementManifestTestFinalResult()
	finalResult["carryState"] = battleCarryStatePayload(state, store.BattleRunStatusEnded)
	summary := battleRunResultSummary(finalResult)
	manifest, ok := buildBattleSettlementManifestV1(map[string]any{"carryState": finalResult["carryState"]})
	if !ok {
		t.Fatal("carry state should produce a settlement manifest")
	}
	if len(manifest.Players) != 2 || manifest.Players[0].UnitID != "player:1" || manifest.Players[1].UnitID != "summon:persistent" {
		t.Fatalf("manifest did not preserve carry filtering: %#v", manifest.Players)
	}
	if _, ok := summary["settlementManifestV1"]; !ok {
		t.Fatal("result summary omitted the valid filtered manifest")
	}
}

func TestBattleSettlementManifestPreservesRewardsActorsSkillsAndAbsoluteItems(t *testing.T) {
	finalResult, ok := battleSettlementFinalResultFromSummary(battleSettlementManifestTestSummary(t))
	if !ok {
		t.Fatal("valid settlement summary rejected")
	}
	rewards := decodeBattleRewardPlan(finalResult)
	wantReward := store.BattleWaveReward{
		ItemRewards:  map[string]int64{"slime_drop": 3},
		AttrExp:      map[string]float64{"strength": 1.25},
		KittyAttrExp: map[int64]map[string]float64{501: {"agility": 0.5}},
	}
	if !reflect.DeepEqual(rewards[101], wantReward) {
		t.Fatalf("reward plan changed: got=%#v want=%#v", rewards[101], wantReward)
	}
	killEvents := battleKillAchievementEventsFromResult(10, finalResult, nil)
	if len(killEvents) != 1 || killEvents[0].EnemyID != "slime_001" || killEvents[0].Count != 2 {
		t.Fatalf("kill achievements changed: %#v", killEvents)
	}

	skillRewards, authoritative := battleSkillExpRewardsFromStats(finalResult, func(string) int { return 200 })
	if !authoritative {
		t.Fatal("battleStats should remain authoritative")
	}
	if got := skillRewards[101]["skill_alpha"]; got != 2*store.BattleSkillExpPerUse {
		t.Fatalf("player skill exp=%d", got)
	}
	if got := skillRewards[-501]["skill_beta"]; got != store.BattleSkillExpPerUse {
		t.Fatalf("kitty skill exp=%d", got)
	}

	writer := &settlementManifestItemWriter{}
	stats, err := settleBattleItemBagFromFinalResultWithWriter(context.Background(), writer, finalResult)
	if err != nil {
		t.Fatal(err)
	}
	if stats.RowsUpdated != 2 {
		t.Fatalf("item rows=%d, want 2", stats.RowsUpdated)
	}
	wantItems := []store.BattleItemBagQuantityUpdate{
		{UserID: 101, ItemID: "potion_a", Quantity: 0},
		{UserID: 101, ItemID: "kitty_snack", Quantity: 2},
	}
	if !reflect.DeepEqual(writer.updates, wantItems) {
		t.Fatalf("absolute item quantities changed: got=%#v want=%#v", writer.updates, wantItems)
	}

	players := decodeBattlePlayersFromFinalResult(finalResult)
	if len(players) != 3 || players[1].KittyID != 501 || players[1].UserID != 101 || players[1].UnitType != "kitty" {
		t.Fatalf("player/kitty identity changed: %#v", players)
	}
}

func TestEncodeBattleRewardPlanEmitsCompleteEmptyRewardMaps(t *testing.T) {
	rows := encodeBattleRewardPlan(map[int64]store.BattleWaveReward{101: {}})
	if len(rows) != 1 || rows[0].ItemRewards == nil || rows[0].AttrExp == nil || rows[0].KittyAttrExp == nil {
		t.Fatalf("reward plan maps must be explicit empty objects: %#v", rows)
	}
	raw, err := json.Marshal(rows)
	if err != nil {
		t.Fatal(err)
	}
	decoded := []map[string]any{}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	if len(decoded) != 1 {
		t.Fatalf("decoded reward rows=%d", len(decoded))
	}
	for _, key := range []string{"itemRewards", "attrExp", "kittyAttrExp"} {
		if value, exists := decoded[0][key]; !exists || normalizeAnyMap(value) == nil {
			t.Fatalf("reward field %q is missing or null: %#v", key, decoded[0])
		}
	}
	if !battleSettlementRewardPlanComplete(decoded) {
		t.Fatal("explicit empty reward plan should be authoritative")
	}
}

func TestResolveBattleSettlementRunTreatsEmptyBattleStatsAsComplete(t *testing.T) {
	summary := battleSettlementManifestTestSummary(t)
	summary["battleStats"] = map[string]any{
		"enemyKills":       map[string]any{},
		"skillCastsByUnit": map[string]any{},
	}
	fallbackCalled := false
	payload, err := resolveBattleSettlementRun(store.BattleRun{ID: 11, FinalResult: summary}, func() (store.BattleRun, error) {
		fallbackCalled = true
		return store.BattleRun{}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if fallbackCalled || !payload.UsedManifest {
		t.Fatalf("empty authoritative stats unexpectedly fell back: called=%t payload=%+v", fallbackCalled, payload)
	}
	if got := battleKillAchievementEventsFromResult(11, payload.FinalResult, payload.Timeline); len(got) != 0 {
		t.Fatalf("empty authoritative kill stats produced events: %#v", got)
	}
	if got := battleSkillExpRewardsFromTimeline(payload.Timeline, payload.FinalResult, func(string) int { return 200 }); len(got) != 0 {
		t.Fatalf("empty authoritative skill stats produced exp: %#v", got)
	}
}

func TestResolveBattleSettlementRunFallsBackForOldRun(t *testing.T) {
	minimal := store.BattleRun{ID: 12, FinalResult: map[string]any{
		"outcome":    store.BattleRunStatusEnded,
		"rewardPlan": []any{},
	}}
	full := store.BattleRun{
		ID:          12,
		Timeline:    []map[string]any{{"seq": int64(7), "payload": map[string]any{"eventType": "battle.turn.action"}}},
		FinalResult: map[string]any{"source": "cold"},
	}
	fallbackCalled := false
	payload, err := resolveBattleSettlementRun(minimal, func() (store.BattleRun, error) {
		fallbackCalled = true
		return full, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !fallbackCalled || payload.UsedManifest || payload.FinalResult["source"] != "cold" || len(payload.Timeline) != 1 || payload.Timeline[0].Seq != 7 {
		t.Fatalf("old run did not use the complete fallback: called=%t payload=%+v", fallbackCalled, payload)
	}
}

func TestResolveBattleSettlementRunFallsBackForMalformedOrIncompleteSummary(t *testing.T) {
	valid := battleSettlementManifestTestSummary(t)
	tests := []struct {
		name   string
		mutate func(map[string]any)
	}{
		{name: "manifest missing", mutate: func(summary map[string]any) { delete(summary, "settlementManifestV1") }},
		{name: "unknown manifest version", mutate: func(summary map[string]any) {
			normalizeAnyMap(summary["settlementManifestV1"])["version"] = float64(2)
		}},
		{name: "manifest skills missing", mutate: func(summary map[string]any) {
			manifest := normalizeAnyMap(summary["settlementManifestV1"])
			delete(battleAnyMapSlice(manifest["players"])[0], "skills")
		}},
		{name: "manifest player identity field missing", mutate: func(summary map[string]any) {
			manifest := normalizeAnyMap(summary["settlementManifestV1"])
			delete(battleAnyMapSlice(manifest["players"])[0], "userId")
		}},
		{name: "manifest item quantity invalid", mutate: func(summary map[string]any) {
			manifest := normalizeAnyMap(summary["settlementManifestV1"])
			battleAnyMapSlice(battleAnyMapSlice(manifest["players"])[0]["battleItems"])[0]["quantity"] = float64(-1)
		}},
		{name: "battle stats incomplete", mutate: func(summary map[string]any) {
			delete(normalizeAnyMap(summary["battleStats"]), "skillCastsByUnit")
		}},
		{name: "reward plan malformed", mutate: func(summary map[string]any) { summary["rewardPlan"] = map[string]any{} }},
		{name: "reward plan field missing", mutate: func(summary map[string]any) {
			delete(battleAnyMapSlice(summary["rewardPlan"])[0], "attrExp")
		}},
		{name: "kitty attr exp field missing", mutate: func(summary map[string]any) {
			delete(battleAnyMapSlice(summary["rewardPlan"])[0], "kittyAttrExp")
		}},
		{name: "item rewards null", mutate: func(summary map[string]any) {
			battleAnyMapSlice(summary["rewardPlan"])[0]["itemRewards"] = nil
		}},
		{name: "attr exp null", mutate: func(summary map[string]any) {
			battleAnyMapSlice(summary["rewardPlan"])[0]["attrExp"] = nil
		}},
		{name: "kitty attr exp null", mutate: func(summary map[string]any) {
			battleAnyMapSlice(summary["rewardPlan"])[0]["kittyAttrExp"] = nil
		}},
		{name: "item reward zero", mutate: func(summary map[string]any) {
			battleAnyMapSlice(summary["rewardPlan"])[0]["itemRewards"] = map[string]any{"slime_drop": float64(0)}
		}},
		{name: "attribute experience negative", mutate: func(summary map[string]any) {
			battleAnyMapSlice(summary["rewardPlan"])[0]["attrExp"] = map[string]any{"strength": float64(-1)}
		}},
		{name: "kitty attribute experience zero", mutate: func(summary map[string]any) {
			battleAnyMapSlice(summary["rewardPlan"])[0]["kittyAttrExp"] = map[string]any{"501": map[string]any{"agility": float64(0)}}
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			summary := cloneSettlementManifestTestSummary(t, valid)
			tc.mutate(summary)
			fallbackCalled := false
			payload, err := resolveBattleSettlementRun(store.BattleRun{ID: 13, FinalResult: summary}, func() (store.BattleRun, error) {
				fallbackCalled = true
				return store.BattleRun{ID: 13, FinalResult: map[string]any{"source": "fallback"}}, nil
			})
			if err != nil {
				t.Fatal(err)
			}
			if !fallbackCalled || payload.UsedManifest || payload.FinalResult["source"] != "fallback" {
				t.Fatalf("unsafe summary bypassed fallback: called=%t payload=%+v", fallbackCalled, payload)
			}
		})
	}
}
