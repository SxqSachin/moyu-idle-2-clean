package store

import (
	"testing"

	"mysecondwebgame/server/internal/catalog"
)

func TestFilterRumorCandidatesMutexLocksOtherFirstStages(t *testing.T) {
	templates := []catalog.CatVillageCommission{
		{
			ID:              "cvc_rumor_harmonic_lunar_loom_thread",
			IsRumor:         true,
			OncePerAccount:  true,
			RumorMutexGroup: "harmonic_embryo_legend",
			Weight:          1,
			Costs:           map[string]catalog.CatVillageItemRange{"goldvine_fiber": {Min: 1, Max: 1}},
		},
		{
			ID:              "cvc_rumor_harmonic_lunar_loom_pattern",
			IsRumor:         true,
			OncePerAccount:  true,
			RumorMutexGroup: "harmonic_embryo_legend",
			PrereqRumorIDs:  []string{"cvc_rumor_harmonic_lunar_loom_thread"},
			Weight:          1,
			Costs:           map[string]catalog.CatVillageItemRange{"clear_resin": {Min: 70, Max: 70}},
		},
		{
			ID:              "cvc_rumor_harmonic_shadow_echo_print",
			IsRumor:         true,
			OncePerAccount:  true,
			RumorMutexGroup: "harmonic_embryo_legend",
			Weight:          1,
			Costs:           map[string]catalog.CatVillageItemRange{"shadow_paw_silk": {Min: 1, Max: 1}},
		},
	}

	candidates := filterRumorCandidates(
		templates,
		12,
		map[string]struct{}{"cvc_rumor_harmonic_lunar_loom_thread": {}},
		nil,
	)

	if len(candidates) != 1 || candidates[0].ID != "cvc_rumor_harmonic_lunar_loom_pattern" {
		t.Fatalf("candidates=%+v, want only lunar stage 2", candidates)
	}
}

func TestFilterRumorCandidatesActiveMutexBlocksOtherFirstStages(t *testing.T) {
	templates := []catalog.CatVillageCommission{
		{
			ID:              "cvc_rumor_harmonic_lunar_loom_thread",
			IsRumor:         true,
			OncePerAccount:  true,
			RumorMutexGroup: "harmonic_embryo_legend",
			Weight:          1,
			Costs:           map[string]catalog.CatVillageItemRange{"goldvine_fiber": {Min: 1, Max: 1}},
		},
		{
			ID:              "cvc_rumor_harmonic_shadow_echo_print",
			IsRumor:         true,
			OncePerAccount:  true,
			RumorMutexGroup: "harmonic_embryo_legend",
			Weight:          1,
			Costs:           map[string]catalog.CatVillageItemRange{"shadow_paw_silk": {Min: 1, Max: 1}},
		},
	}

	candidates := filterRumorCandidates(
		templates,
		12,
		map[string]struct{}{},
		[]openTaskRow{{commissionID: "cvc_rumor_harmonic_lunar_loom_thread"}},
	)

	if len(candidates) != 0 {
		t.Fatalf("candidates=%+v, want none while a mutex-group rumor is active", candidates)
	}
}

func TestFilterRumorCandidatesRepeatableChainAdvancesWithoutReopeningFirstStage(t *testing.T) {
	templates := []catalog.CatVillageCommission{
		{
			ID:             "cvc_rumor_repeatable_clue",
			IsRumor:        true,
			OncePerAccount: false,
			Weight:         1,
			Costs:          map[string]catalog.CatVillageItemRange{"clue_material": {Min: 1, Max: 1}},
		},
		{
			ID:                 "cvc_rumor_repeatable_invitation",
			IsRumor:            true,
			OncePerAccount:     false,
			PrereqRumorIDs:     []string{"cvc_rumor_repeatable_clue"},
			ClearCompletionIds: []string{"cvc_rumor_repeatable_clue", "cvc_rumor_repeatable_invitation"},
			Weight:             1,
			Costs:              map[string]catalog.CatVillageItemRange{"clue": {Min: 1, Max: 1}},
		},
	}

	candidates := filterRumorCandidates(
		templates,
		10,
		map[string]struct{}{"cvc_rumor_repeatable_clue": {}},
		nil,
	)

	if len(candidates) != 1 || candidates[0].ID != "cvc_rumor_repeatable_invitation" {
		t.Fatalf("candidates=%+v, want only repeatable stage 2", candidates)
	}
}

func TestFilterRumorCandidatesFinalClearReopensRepeatableChain(t *testing.T) {
	templates := []catalog.CatVillageCommission{
		{
			ID:             "cvc_rumor_repeatable_clue",
			IsRumor:        true,
			OncePerAccount: false,
			Weight:         1,
			Costs:          map[string]catalog.CatVillageItemRange{"clue_material": {Min: 1, Max: 1}},
		},
		{
			ID:             "cvc_rumor_repeatable_invitation",
			IsRumor:        true,
			OncePerAccount: false,
			PrereqRumorIDs: []string{"cvc_rumor_repeatable_clue"},
			Weight:         1,
			Costs:          map[string]catalog.CatVillageItemRange{"clue": {Min: 1, Max: 1}},
		},
	}

	candidates := filterRumorCandidates(templates, 10, map[string]struct{}{}, nil)
	if len(candidates) != 1 || candidates[0].ID != "cvc_rumor_repeatable_clue" {
		t.Fatalf("candidates=%+v, want stage 1 after terminal clear", candidates)
	}
}

func TestFilterRumorCandidatesStillExcludesCompletedOncePerAccountRumor(t *testing.T) {
	templates := []catalog.CatVillageCommission{
		{
			ID:             "cvc_rumor_once",
			IsRumor:        true,
			OncePerAccount: true,
			Weight:         1,
			Costs:          map[string]catalog.CatVillageItemRange{"clue": {Min: 1, Max: 1}},
		},
	}

	candidates := filterRumorCandidates(
		templates,
		10,
		map[string]struct{}{"cvc_rumor_once": {}},
		nil,
	)
	if len(candidates) != 0 {
		t.Fatalf("candidates=%+v, want completed one-time rumor excluded", candidates)
	}
}

func TestClassifyOpenCatVillageTasksCountsRumorIndependentlyOfDeletionFlag(t *testing.T) {
	templates := map[string]catalog.CatVillageCommission{
		"cvc_rumor_first_stage": {
			ID:      "cvc_rumor_first_stage",
			IsRumor: true,
		},
		"cvc_battle": {
			ID:       "cvc_battle",
			IsBattle: true,
		},
		"cvc_life": {
			ID: "cvc_life",
		},
	}

	rumors, battles, life := classifyOpenCatVillageTasks(
		[]openTaskRow{
			{commissionID: "cvc_rumor_first_stage"},
			{commissionID: "cvc_battle"},
			{commissionID: "cvc_life"},
		},
		templates,
	)

	if rumors != 1 || battles != 1 || life != 1 {
		t.Fatalf("counts=(%d,%d,%d), want (1,1,1)", rumors, battles, life)
	}
}

func TestRefreshCandidatesForExistingCommissionPreservesBattleCategory(t *testing.T) {
	templates := []catalog.CatVillageCommission{
		{ID: "cvc_life", Weight: 1, Costs: map[string]catalog.CatVillageItemRange{"wood": {Min: 1, Max: 1}}},
		{ID: "cvc_battle", IsBattle: true, Weight: 1, Costs: map[string]catalog.CatVillageItemRange{"fang": {Min: 1, Max: 1}}},
		{ID: "cvc_rumor", IsRumor: true, Weight: 1, Costs: map[string]catalog.CatVillageItemRange{"clue": {Min: 1, Max: 1}}},
	}

	candidates, ok := refreshCandidatesForExistingCommission(templates, "cvc_battle")
	if !ok {
		t.Fatal("expected battle commission refresh candidates")
	}
	if len(candidates) != 1 || candidates[0].ID != "cvc_battle" || !candidates[0].IsBattle {
		t.Fatalf("candidates=%+v, want only battle candidates", candidates)
	}
}

func TestRefreshCandidatesForExistingCommissionPreservesLifeCategory(t *testing.T) {
	templates := []catalog.CatVillageCommission{
		{ID: "cvc_life", Weight: 1, Costs: map[string]catalog.CatVillageItemRange{"wood": {Min: 1, Max: 1}}},
		{ID: "cvc_battle", IsBattle: true, Weight: 1, Costs: map[string]catalog.CatVillageItemRange{"fang": {Min: 1, Max: 1}}},
		{ID: "cvc_rumor", IsRumor: true, Weight: 1, Costs: map[string]catalog.CatVillageItemRange{"clue": {Min: 1, Max: 1}}},
	}

	candidates, ok := refreshCandidatesForExistingCommission(templates, "cvc_life")
	if !ok {
		t.Fatal("expected life commission refresh candidates")
	}
	if len(candidates) != 1 || candidates[0].ID != "cvc_life" || candidates[0].IsBattle || candidates[0].IsRumor {
		t.Fatalf("candidates=%+v, want only life candidates", candidates)
	}
}

func TestPlanCatVillageRefreshCategoriesPrefersFourLifeThenTwoBattle(t *testing.T) {
	got := planCatVillageRefreshCategories(6, 24, 10, true, true)
	want := []catVillageRefreshCategory{
		catVillageRefreshLife,
		catVillageRefreshLife,
		catVillageRefreshLife,
		catVillageRefreshLife,
		catVillageRefreshBattle,
		catVillageRefreshBattle,
	}

	if len(got) != len(want) {
		t.Fatalf("plan length=%d, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("plan[%d]=%s, want %s: %+v", i, got[i], want[i], got)
		}
	}
}

func TestPlanCatVillageRefreshCategoriesSpillsToBattleWhenLifeFull(t *testing.T) {
	got := planCatVillageRefreshCategories(6, 2, 10, true, true)
	want := []catVillageRefreshCategory{
		catVillageRefreshLife,
		catVillageRefreshLife,
		catVillageRefreshBattle,
		catVillageRefreshBattle,
		catVillageRefreshBattle,
		catVillageRefreshBattle,
	}

	if len(got) != len(want) {
		t.Fatalf("plan length=%d, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("plan[%d]=%s, want %s: %+v", i, got[i], want[i], got)
		}
	}
}

func TestPlanCatVillageRefreshCategoriesSpillsToLifeWhenBattleFull(t *testing.T) {
	got := planCatVillageRefreshCategories(6, 24, 1, true, true)
	want := []catVillageRefreshCategory{
		catVillageRefreshLife,
		catVillageRefreshLife,
		catVillageRefreshLife,
		catVillageRefreshLife,
		catVillageRefreshBattle,
		catVillageRefreshLife,
	}

	if len(got) != len(want) {
		t.Fatalf("plan length=%d, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("plan[%d]=%s, want %s: %+v", i, got[i], want[i], got)
		}
	}
}

func TestHasEligibleCatVillageCommissionRespectsDisabledWeightAndRepLevel(t *testing.T) {
	templates := []catalog.CatVillageCommission{
		{ID: "disabled", Disabled: true, Weight: 1},
		{ID: "zero_weight", Weight: 0},
		{ID: "locked", Weight: 1, MinRepLevel: 10},
	}
	if hasEligibleCatVillageCommission(templates, 3) {
		t.Fatal("expected no eligible commissions")
	}
	templates = append(templates, catalog.CatVillageCommission{ID: "eligible", Weight: 1, MinRepLevel: 3})
	if !hasEligibleCatVillageCommission(templates, 3) {
		t.Fatal("expected one eligible commission")
	}
}

func TestApplyCatVillageTaskTemplateFieldsAddsRefreshPayloadMetadata(t *testing.T) {
	task := &CatVillageTask{ID: 10, CommissionID: "cvc_battle", Refreshed: true, Status: "open"}
	tmpl := catalog.CatVillageCommission{
		ID:             "cvc_battle",
		IsBattle:       true,
		IsRumor:        true,
		RumorBadge:     "special",
		NotRefreshable: true,
		NotDeletable:   true,
		PityThreshold:  7,
	}

	applyCatVillageTaskTemplateFields(task, tmpl)

	if !task.IsBattle {
		t.Fatalf("IsBattle=%v, want true", task.IsBattle)
	}
	if !task.IsRumor || task.RumorBadge != "special" || !task.NotRefreshable || !task.NotDeletable || task.PityThreshold != 7 {
		t.Fatalf("task metadata=%+v, want template-derived metadata", task)
	}
}
