package httpapi

import (
	"encoding/json"
	"testing"
	"time"

	"mysecondwebgame/server/internal/app"
	"mysecondwebgame/server/internal/catalog"
	"mysecondwebgame/server/internal/store"
)

func TestApplyBattleAffixEffectMutations_IndexClampAndAmount(t *testing.T) {
	s := &Server{}
	actor := &battleUnitState{
		UnitID: "player:1",
		Stats: map[string]float64{
			"magical_damage": 100,
		},
		EffectMutations: []battleAffixEffectMutation{
			{
				Scope:     "skill",
				SelectAll: false,
				Indexes:   []int{1},
				TargetCountDelta: &battleSkillAmountFormula{
					BaseAdd: -3,
				},
				AmountBaseAddDelta: &battleSkillAmountFormula{
					BaseAdd: 10,
				},
				AmountTermsAppend: []battleAttackTerm{
					{AttrKey: "magical_damage", Coefficient: 0.2},
				},
				ConstraintTargetCountMin: intPtr(1),
			},
		},
	}
	effects := []battleSkillEffect{
		{
			Kind: "damage",
			Targeting: battleSkillTargeting{
				TargetPositions: []string{"vanguard"},
				SecondarySeek:   "front_to_rear",
				TargetScope:     "enemy",
				SelectRule:      "first",
				TargetCount: battleSkillTargetCount{
					Base: 2,
				},
			},
			AmountFormula: battleSkillAmountFormula{
				BaseAdd: 1,
			},
			DamageSpec: battleDamageSpec{
				DamageType: "magical",
				AmountFormula: battleSkillAmountFormula{
					BaseAdd: 1,
				},
				HitCount: battleSkillTargetCount{Base: 1},
			},
		},
		{
			Kind: "status",
			Targeting: battleSkillTargeting{
				TargetPositions: []string{"vanguard"},
				SecondarySeek:   "front_to_rear",
				TargetScope:     "enemy",
				SelectRule:      "first",
				TargetCount: battleSkillTargetCount{
					Base: 1,
				},
			},
			StatusPayload: battleSkillStatusPayload{
				StatusIDs: []string{"bleed"},
			},
		},
	}
	out := s.applyBattleAffixEffectMutations(actor, "skill", "fireball", 1, effects[0].Targeting, effects)
	if len(out) != 2 {
		t.Fatalf("effects len=%d, want 2", len(out))
	}
	if out[0].Targeting.TargetCount.Base != 1 {
		t.Fatalf("targetCount=%d, want 1", out[0].Targeting.TargetCount.Base)
	}
	if out[0].AmountFormula.BaseAdd != 11 {
		t.Fatalf("amount baseAdd=%v, want 11", out[0].AmountFormula.BaseAdd)
	}
	if len(out[0].AmountFormula.Terms) != 1 {
		t.Fatalf("amount terms len=%d, want 1", len(out[0].AmountFormula.Terms))
	}
	if out[1].StatusPayload.StatusIDs[0] != "bleed" {
		t.Fatalf("second effect should stay unchanged")
	}
}

func TestApplyBattleAffixEffectMutations_AllStatusMergeAndAppendEffect(t *testing.T) {
	s := &Server{}
	actor := &battleUnitState{
		UnitID: "player:1",
		EffectMutations: []battleAffixEffectMutation{
			{
				Scope:     "basic_attack",
				SelectAll: true,
				StatusAppend: []string{
					"burn",
					"bleed",
				},
				StatusRemove: map[string]struct{}{
					"bleed": {},
				},
				AppendEffects: []battleSkillEffect{
					{
						Kind: "status",
						StatusPayload: battleSkillStatusPayload{
							StatusIDs: []string{"weakness"},
						},
					},
				},
				ConstraintTargetCountMin: intPtr(1),
			},
		},
	}
	effects := []battleSkillEffect{
		{
			Kind: "status",
			Targeting: battleSkillTargeting{
				TargetPositions: []string{"vanguard"},
				SecondarySeek:   "front_to_rear",
				TargetScope:     "enemy",
				SelectRule:      "first",
				TargetCount: battleSkillTargetCount{
					Base: 1,
				},
			},
			StatusPayload: battleSkillStatusPayload{
				StatusIDs: []string{"bleed", "poison"},
			},
		},
	}
	out := s.applyBattleAffixEffectMutations(actor, "basic_attack", "basic_attack", 1, effects[0].Targeting, effects)
	if len(out) != 2 {
		t.Fatalf("effects len=%d, want 2", len(out))
	}
	if got := out[0].StatusPayload.StatusIDs; len(got) != 2 || got[0] != "poison" || got[1] != "burn" {
		t.Fatalf("merged status ids=%v, want [poison burn]", got)
	}
	if out[1].Kind != "status" || len(out[1].StatusPayload.StatusIDs) != 1 || out[1].StatusPayload.StatusIDs[0] != "weakness" {
		t.Fatalf("append effect mismatch: %#v", out[1])
	}
}

func TestParseBattleAffixEffectMutation_AmountTermsAppendSupportsMapSlice(t *testing.T) {
	mutation, ok := parseBattleAffixEffectMutation(map[string]any{
		"scope": "basic_attack",
		"ops": map[string]any{
			"amountTermsAppend": []map[string]any{
				{
					"attrKey":       "sewing",
					"coefficient":   5,
					"perSkillLevel": 0,
				},
			},
		},
	})
	if !ok {
		t.Fatalf("expected mutation to parse")
	}
	if len(mutation.AmountTermsAppend) != 1 {
		t.Fatalf("amountTermsAppend len=%d, want 1", len(mutation.AmountTermsAppend))
	}
	term := mutation.AmountTermsAppend[0]
	if term.AttrKey != "sewing" || term.Coefficient != 5 || term.PerSkillLevel != 0 {
		t.Fatalf("unexpected amount term: %#v", term)
	}
}

func TestParseBattleAffixEffectMutation_SnakeCaseKeys(t *testing.T) {
	mutation, ok := parseBattleAffixEffectMutation(map[string]any{
		"scope":     "basic_attack",
		"skill_ids": []any{"basic_attack"},
		"ops": map[string]any{
			"amount_terms_append": []any{
				map[string]any{
					"attrKey":       "sewing",
					"coefficient":   3,
					"perSkillLevel": 0,
				},
			},
			"status_ids_append": []any{"burn"},
		},
		"constraints": map[string]any{
			"target_count_min": 2,
		},
	})
	if !ok {
		t.Fatalf("expected mutation to parse")
	}
	if len(mutation.AmountTermsAppend) != 1 {
		t.Fatalf("amountTermsAppend len=%d, want 1", len(mutation.AmountTermsAppend))
	}
	if mutation.AmountTermsAppend[0].AttrKey != "sewing" || mutation.AmountTermsAppend[0].Coefficient != 3 {
		t.Fatalf("unexpected amount term: %#v", mutation.AmountTermsAppend[0])
	}
	if len(mutation.StatusAppend) != 1 || mutation.StatusAppend[0] != "burn" {
		t.Fatalf("status append mismatch: %#v", mutation.StatusAppend)
	}
	if mutation.ConstraintTargetCountMin == nil || *mutation.ConstraintTargetCountMin != 2 {
		t.Fatalf("target_count_min not parsed: %#v", mutation.ConstraintTargetCountMin)
	}
}

func TestExecuteBattleSkill_AppliesBasicAttackMutationFromOriginalEffects(t *testing.T) {
	s := &Server{}
	s.deps.GameData.Skills = []map[string]any{
		{"id": "basic_attack"},
	}
	actor := battleUnitState{
		UnitID:   "player:1",
		Side:     "player",
		Position: "vanguard",
		Alive:    true,
		HP:       100,
		MaxHP:    100,
		Stats: map[string]float64{
			"physics_damage": 10,
			"hp":             100,
		},
		EffectMutations: []battleAffixEffectMutation{
			{
				Scope:   "basic_attack",
				Indexes: []int{1},
				TargetCountDelta: &battleSkillAmountFormula{
					BaseAdd: 1,
				},
			},
		},
		BasicAttack: &battleBasicAttackProfile{
			Effects: []battleSkillEffect{
				{
					Kind: "damage",
					Targeting: battleSkillTargeting{
						TargetPositions: []string{"vanguard"},
						SecondarySeek:   "front_to_rear",
						TargetScope:     "enemy",
						SelectRule:      "first",
						TargetCount: battleSkillTargetCount{
							Base: 1, // original value; mutation (delta=1) applied at execution → 2
						},
					},
					DamageType: "physical",
					AmountFormula: battleSkillAmountFormula{
						Terms: []battleAttackTerm{{AttrKey: "physics_damage", Coefficient: 1}},
					},
					DamageSpec: battleDamageSpec{
						DamageType: "physical",
						AmountFormula: battleSkillAmountFormula{
							Terms: []battleAttackTerm{{AttrKey: "physics_damage", Coefficient: 1}},
						},
						HitCount: battleSkillTargetCount{Base: 1},
					},
				},
			},
		},
	}
	state := &battleRuntimeState{
		Seed: 1,
		Wave: 1,
		Players: []battleUnitState{
			actor,
		},
		Enemies: []battleUnitState{
			{UnitID: "enemy:1", Side: "enemy", Position: "vanguard", Alive: true, HP: 100, MaxHP: 100, Stats: map[string]float64{"hp": 100}},
			{UnitID: "enemy:2", Side: "enemy", Position: "vanguard", Alive: true, HP: 100, MaxHP: 100, Stats: map[string]float64{"hp": 100}},
			{UnitID: "enemy:3", Side: "enemy", Position: "rear", Alive: true, HP: 100, MaxHP: 100, Stats: map[string]float64{"hp": 100}},
		},
	}
	skill := battleSkillState{
		ID:    "basic_attack",
		Name:  "basic_attack",
		Level: 1,
	}
	execution := s.executeBattleSkill(state, time.Unix(0, 0), &state.Players[0], skill, "active", nil)
	targets := map[string]struct{}{}
	for _, effect := range execution.Effects {
		targetID, _ := effect["targetId"].(string)
		if targetID == "" {
			continue
		}
		targets[targetID] = struct{}{}
	}
	if len(targets) != 2 {
		t.Fatalf("unique targets=%d, want 2", len(targets))
	}
}

func TestApplyBattleAffixEffectMutations_TargetCountDeltaCanBreakTargetCountMax(t *testing.T) {
	s := &Server{}
	actor := &battleUnitState{
		UnitID: "player:1",
		Stats: map[string]float64{
			"intelligence": 4,
		},
		EffectMutations: []battleAffixEffectMutation{
			{
				Scope:   "skill",
				Indexes: []int{1},
				TargetCountDelta: &battleSkillAmountFormula{
					BaseAdd: 2,
				},
				ConstraintTargetCountMin: intPtr(1),
			},
		},
	}
	effects := []battleSkillEffect{
		{
			Kind: "damage",
			Targeting: battleSkillTargeting{
				TargetPositions: []string{"vanguard"},
				SecondarySeek:   "front_to_rear",
				TargetScope:     "enemy",
				SelectRule:      "first",
				TargetCount: battleSkillTargetCount{
					Base:    2,
					Formula: []battleAttackTerm{{AttrKey: "intelligence", Coefficient: 1}},
					Max:     3,
					MaxSet:  true,
				},
			},
			AmountFormula: battleSkillAmountFormula{
				BaseAdd: 1,
			},
			DamageSpec: battleDamageSpec{
				DamageType: "magical",
				AmountFormula: battleSkillAmountFormula{
					BaseAdd: 1,
				},
				HitCount: battleSkillTargetCount{Base: 1},
			},
		},
	}

	before := resolveBattleTemplateTargetCount(actor, 1, effects[0].Targeting)
	if before != 3 {
		t.Fatalf("before mutation targetCount=%d, want 3", before)
	}
	out := s.applyBattleAffixEffectMutations(actor, "skill", "fireball", 1, effects[0].Targeting, effects)
	if len(out) != 1 {
		t.Fatalf("effects len=%d, want 1", len(out))
	}
	after := resolveBattleTemplateTargetCount(actor, 1, out[0].Targeting)
	if after != 5 {
		t.Fatalf("after mutation targetCount=%d, want 5", after)
	}
}

func TestBattleAffixEffectMutation_JSONRoundTripPreservesRuntimeFields(t *testing.T) {
	original := battleAffixEffectMutation{
		Scope:       "basic_attack",
		SkillIDList: []string{"basic_attack"},
		SkillIDs: map[string]struct{}{
			"basic_attack": {},
		},
		Indexes: []int{1},
		TargetCountDelta: &battleSkillAmountFormula{
			BaseAdd: 1,
		},
		AmountTermsAppend: []battleAttackTerm{
			{AttrKey: "physics_damage", Coefficient: 0.25},
		},
		StatusAppend:     []string{"burn"},
		StatusRemoveList: []string{"bleed"},
		StatusRemove: map[string]struct{}{
			"bleed": {},
		},
		AppendEffects: []battleSkillEffect{
			{
				Kind: "status",
				Targeting: battleSkillTargeting{
					TargetPositions: []string{"vanguard"},
					SecondarySeek:   "front_to_rear",
					TargetScope:     "enemy",
					SelectRule:      "random",
					TargetCount: battleSkillTargetCount{
						Base:   1,
						Max:    1,
						MaxSet: true,
					},
				},
				StatusPayload: battleSkillStatusPayload{
					StatusIDs: []string{"weakness"},
				},
			},
		},
		ConstraintTargetCountMin: intPtr(1),
		ConstraintTargetCountMax: intPtr(3),
	}
	buf, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	var decoded battleAffixEffectMutation
	if err := json.Unmarshal(buf, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	normalizeBattleAffixEffectMutation(&decoded)
	if decoded.TargetCountDelta == nil || decoded.TargetCountDelta.BaseAdd != 1 {
		t.Fatalf("targetCountDelta lost after round trip: %#v", decoded.TargetCountDelta)
	}
	if len(decoded.AmountTermsAppend) != 1 || decoded.AmountTermsAppend[0].AttrKey != "physics_damage" {
		t.Fatalf("amountTermsAppend lost after round trip: %#v", decoded.AmountTermsAppend)
	}
	if _, ok := decoded.SkillIDs["basic_attack"]; !ok {
		t.Fatalf("skillIDs runtime lookup not restored: %#v", decoded.SkillIDs)
	}
	if _, ok := decoded.StatusRemove["bleed"]; !ok {
		t.Fatalf("statusRemove runtime lookup not restored: %#v", decoded.StatusRemove)
	}
	if len(decoded.AppendEffects) != 1 || !decoded.AppendEffects[0].Targeting.TargetCount.MaxSet {
		t.Fatalf("appendEffects targetCount maxSet lost after round trip: %#v", decoded.AppendEffects)
	}
}

func TestDecodeBattlePlayersFromFinalResultNode_RestoresCarriedMutationState(t *testing.T) {
	state := map[string]any{
		"players": []map[string]any{
			{
				"unitId":   "player:1",
				"side":     "player",
				"position": "vanguard",
				"alive":    true,
				"hp":       100.0,
				"maxHp":    100.0,
				"mp":       0.0,
				"maxMp":    0.0,
				"stats": map[string]any{
					"physics_damage": 10.0,
					"hp":             100.0,
				},
				"effectMutations": []map[string]any{
					{
						"scope":            "basic_attack",
						"indexes":          []any{1},
						"targetCountDelta": map[string]any{"baseAdd": 1.0},
						"amountTermsAppend": []map[string]any{
							{"attrKey": "physics_damage", "coefficient": 0.25, "perSkillLevel": 0.0},
						},
						"constraintTargetCountMin": 1,
					},
				},
				"basicAttack": map[string]any{
					"targetDirection":    "vanguard",
					"targetSeekStrategy": "front_to_rear",
					"damageType":         "physical",
					"effects": []map[string]any{
						{
							"kind":       "damage",
							"damageType": "physical",
							"targeting": map[string]any{
								"targetPositions": []any{"vanguard"},
								"secondarySeek":   "front_to_rear",
								"targetScope":     "enemy",
								"selectRule":      "random",
								"targetCount": map[string]any{
									"base": 1,
								},
							},
							"amountFormula": map[string]any{
								"terms": []map[string]any{
									{"attrKey": "physics_damage", "coefficient": 1.0, "perSkillLevel": 0.0},
								},
							},
							"damageSpec": map[string]any{
								"damageType": "physical",
								"amountFormula": map[string]any{
									"terms": []map[string]any{
										{"attrKey": "physics_damage", "coefficient": 1.0, "perSkillLevel": 0.0},
									},
								},
								"hitCount": map[string]any{"base": 1},
							},
						},
					},
				},
			},
		},
	}
	players := decodeBattlePlayersFromFinalResultNode(state)
	if len(players) != 1 {
		t.Fatalf("players len=%d, want 1", len(players))
	}
	player := players[0]
	if len(player.EffectMutations) != 1 {
		t.Fatalf("effectMutations len=%d, want 1", len(player.EffectMutations))
	}
	mutation := player.EffectMutations[0]
	if mutation.TargetCountDelta == nil || mutation.TargetCountDelta.BaseAdd != 1 {
		t.Fatalf("targetCountDelta not restored: %#v", mutation.TargetCountDelta)
	}
	if len(mutation.AmountTermsAppend) != 1 || mutation.AmountTermsAppend[0].AttrKey != "physics_damage" {
		t.Fatalf("amountTermsAppend not restored: %#v", mutation.AmountTermsAppend)
	}
	s := &Server{}
	s.deps.GameData.Skills = []map[string]any{
		{"id": "basic_attack"},
	}
	skill := battleSkillState{ID: "basic_attack", Level: 1}
	skill = s.applyBattleBasicAttackProfile(skill, &player)
	if len(skill.Effects) != 1 {
		t.Fatalf("basic attack effects len=%d, want 1", len(skill.Effects))
	}
	if skill.Effects[0].Targeting.TargetCount.Base != 2 {
		t.Fatalf("carried basic attack target count=%d, want 2", skill.Effects[0].Targeting.TargetCount.Base)
	}
	if got := len(skill.Effects[0].AmountFormula.Terms); got != 2 {
		t.Fatalf("carried basic attack amount terms len=%d, want 2", got)
	}
}

func TestBattlePublicUnitSlice_ExportsBasicAttackDisplayWithAppliedMutations(t *testing.T) {
	units := []battleUnitState{
		{
			UnitID:   "player:1",
			Side:     "player",
			Position: "vanguard",
			Alive:    true,
			HP:       100,
			MaxHP:    100,
			Stats: map[string]float64{
				"physics_damage": 10,
				"hp":             100,
			},
			EffectMutations: []battleAffixEffectMutation{
				{
					Scope:   "basic_attack",
					Indexes: []int{1},
					TargetCountDelta: &battleSkillAmountFormula{
						BaseAdd: 1,
					},
					AmountTermsAppend: []battleAttackTerm{
						{AttrKey: "physics_damage", Coefficient: 0.25},
					},
					ConstraintTargetCountMin: intPtr(1),
				},
			},
			BasicAttack: &battleBasicAttackProfile{
				TargetDirection:    "vanguard",
				TargetSeekStrategy: "front_to_rear",
				DamageType:         "physical",
				Effects: []battleSkillEffect{
					{
						Kind:       "damage",
						DamageType: "physical",
						Targeting: battleSkillTargeting{
							TargetPositions: []string{"vanguard"},
							SecondarySeek:   "front_to_rear",
							TargetScope:     "enemy",
							SelectRule:      "random",
							TargetCount: battleSkillTargetCount{
								Base: 1,
							},
						},
						AmountFormula: battleSkillAmountFormula{
							Terms: []battleAttackTerm{{AttrKey: "physics_damage", Coefficient: 1}},
						},
						DamageSpec: battleDamageSpec{
							DamageType: "physical",
							AmountFormula: battleSkillAmountFormula{
								Terms: []battleAttackTerm{{AttrKey: "physics_damage", Coefficient: 1}},
							},
							HitCount: battleSkillTargetCount{Base: 1},
						},
					},
				},
			},
		},
	}
	publicUnits := battlePublicUnitSlice(units)
	if len(publicUnits) != 1 {
		t.Fatalf("publicUnits len=%d, want 1", len(publicUnits))
	}
	basicAttack, ok := publicUnits[0]["basicAttack"].(*battleBasicAttackProfile)
	if !ok || basicAttack == nil {
		t.Fatalf("basicAttack export type mismatch: %#v", publicUnits[0]["basicAttack"])
	}
	if len(basicAttack.Effects) != 1 {
		t.Fatalf("basicAttack effects len=%d, want 1", len(basicAttack.Effects))
	}
	if basicAttack.Effects[0].Targeting.TargetCount.Base != 2 {
		t.Fatalf("exported basic attack target count=%d, want 2", basicAttack.Effects[0].Targeting.TargetCount.Base)
	}
	if got := len(basicAttack.Effects[0].AmountFormula.Terms); got != 2 {
		t.Fatalf("exported basic attack amount terms len=%d, want 2", got)
	}
}

func intPtr(value int) *int { return &value }

func TestResolveBattleAffixEffectMutations_AppliesMaxEffectiveLimits(t *testing.T) {
	s := &Server{
		deps: app.Dependencies{
			GameData: catalog.Data{
				AffixDefinitions: []catalog.AffixDefinition{
					{
						ID:                       "affix.mutate",
						EffectType:               "effectMutation",
						MaxEffectivePerEquipment: 2,
						MaxEffectivePerBody:      3,
						EffectParams: map[string]any{
							"effectMutations": []map[string]any{
								{"scope": "both"},
							},
						},
						Active: true,
					},
				},
			},
		},
	}
	equipped := []store.EquippedItem{
		{
			InstanceID: 2,
			EquipType:  "battle",
			Affixes: []store.EquipmentAffixSlot{
				{SlotIndex: 1, AffixID: "affix.mutate", EffectType: "effectMutation", EffectParams: map[string]any{"effectMutations": []map[string]any{{"scope": "both"}}}},
				{SlotIndex: 0, AffixID: "affix.mutate", EffectType: "effectMutation", EffectParams: map[string]any{"effectMutations": []map[string]any{{"scope": "both"}}}},
			},
		},
		{
			InstanceID: 1,
			EquipType:  "battle",
			Affixes: []store.EquipmentAffixSlot{
				{SlotIndex: 0, AffixID: "affix.mutate", EffectType: "effectMutation", EffectParams: map[string]any{"effectMutations": []map[string]any{{"scope": "both"}}}},
				{SlotIndex: 1, AffixID: "affix.mutate", EffectType: "effectMutation", EffectParams: map[string]any{"effectMutations": []map[string]any{{"scope": "both"}}}},
			},
		},
	}
	mutations := s.resolveBattleAffixEffectMutations(equipped)
	if len(mutations) != 3 {
		t.Fatalf("mutation len=%d, want 3 (equipment cap 2 + body cap 3)", len(mutations))
	}
}
