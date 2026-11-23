package store

import "testing"

func TestFormulaDerivedValues(t *testing.T) {
	attrTotals := map[string]float64{
		"strength":     10,
		"dexterity":    5,
		"intelligence": 8,
	}
	formulaByAttr := map[string]formulaContribution{}
	applyFormulaContribution(attrTotals, formulaByAttr, "physics_damage", "strength*0.6 + dexterity*0.4", []formulaTerm{{AttrKey: "strength", Attribute: attrTotals["strength"], Coefficient: 0.6}, {AttrKey: "dexterity", Attribute: attrTotals["dexterity"], Coefficient: 0.4}})
	applyFormulaContribution(attrTotals, formulaByAttr, "attack_speed", "dexterity*1", []formulaTerm{{AttrKey: "dexterity", Attribute: attrTotals["dexterity"], Coefficient: 1}})
	applyFormulaContribution(attrTotals, formulaByAttr, "magical_damage", "intelligence*1.2", []formulaTerm{{AttrKey: "intelligence", Attribute: attrTotals["intelligence"], Coefficient: 1.2}})

	if got := attrTotals["physics_damage"]; got != 8 {
		t.Fatalf("physics_damage=%v, want 8", got)
	}
	if got := attrTotals["attack_speed"]; got != 5 {
		t.Fatalf("attack_speed=%v, want 5", got)
	}
	if got := attrTotals["magical_damage"]; got != 9.6 {
		t.Fatalf("magical_damage=%v, want 9.6", got)
	}
}

func TestBuildAttributeSourcePayloadIncludesEquipmentAndFormula(t *testing.T) {
	payload := buildAttributeSourcePayload(attributeSourceBuildInput{
		AttrKey:      "attack_speed",
		BaseDefault:  1,
		LevelBonus:   0,
		BaseValue:    0,
		BaseExcluded: true,
		FinalValue:   3.5,
		EquipmentByAttr: map[string][]equipmentAttrContribution{
			"attack_speed": {
				{
					InstanceID:     101,
					TemplateItemID: "wood_sword",
					Base:           1,
					Affixes: []equipmentAffixContribution{
						{AffixID: "speed_rune", Value: 0.5},
					},
				},
			},
			"dexterity": {
				{
					InstanceID:     202,
					TemplateItemID: "wood_sword",
					Base:           1,
					Affixes: []equipmentAffixContribution{
						{AffixID: "speed_rune", Value: 0.5},
					},
				},
			},
		},
		FormulaByAttr: map[string]formulaContribution{
			"attack_speed": {
				Expression: "dexterity*1",
				Value:      2,
				Terms: []formulaTerm{
					{AttrKey: "dexterity", Attribute: 2, Coefficient: 1},
				},
			},
		},
		BaseValuesByAttr: map[string]float64{
			"attack_speed": 0,
			"dexterity":    1,
		},
	})

	formulaRaw, ok := payload["formula"]
	if !ok {
		t.Fatalf("payload.formula should exist")
	}
	formula, ok := formulaRaw.(map[string]any)
	if !ok {
		t.Fatalf("payload.formula type invalid")
	}
	if got, _ := formula["expression"].(string); got != "dexterity*1" {
		t.Fatalf("formula.expression=%q, want dexterity*1", got)
	}
	formulaSources, ok := formula["sources"].([]map[string]any)
	if !ok || len(formulaSources) != 1 {
		t.Fatalf("formula.sources should have 1 source")
	}
	breakdown, ok := formulaSources[0]["sourceBreakdown"].(map[string]any)
	if !ok {
		t.Fatalf("formula source should include sourceBreakdown")
	}
	baseRaw, ok := breakdown["base"].(map[string]any)
	if !ok {
		t.Fatalf("sourceBreakdown.base should exist")
	}
	if got, _ := baseRaw["value"].(float64); got != 1 {
		t.Fatalf("sourceBreakdown.base.value=%v, want 1", got)
	}
	breakdownEquip, ok := breakdown["equipment"].([]map[string]any)
	if !ok || len(breakdownEquip) != 1 {
		t.Fatalf("sourceBreakdown.equipment should have 1 row")
	}
	if got, _ := breakdownEquip[0]["total"].(float64); got != 1.5 {
		t.Fatalf("sourceBreakdown.equipment[0].total=%v, want 1.5", got)
	}
	equipmentRaw, ok := payload["equipment"]
	if !ok {
		t.Fatalf("payload.equipment should exist")
	}
	equipment, ok := equipmentRaw.([]map[string]any)
	if !ok {
		t.Fatalf("payload.equipment type invalid")
	}
	if len(equipment) != 1 {
		t.Fatalf("equipment len=%d, want 1", len(equipment))
	}
	if got, _ := equipment[0]["templateItemId"].(string); got != "wood_sword" {
		t.Fatalf("equipment.templateItemId=%q, want wood_sword", got)
	}
	if got, _ := equipment[0]["total"].(float64); got != 1.5 {
		t.Fatalf("equipment.total=%v, want 1.5", got)
	}
}

func TestFormulaDerivedValuesForDexterityAndLuck(t *testing.T) {
	attrTotals := map[string]float64{
		"dexterity": 10,
		"luck":      4,
	}
	formulaByAttr := map[string]formulaContribution{}
	applyFormulaContribution(attrTotals, formulaByAttr, "speed", "dexterity*1", []formulaTerm{
		{AttrKey: "dexterity", Attribute: attrTotals["dexterity"], Coefficient: 1},
	})
	applyFormulaContribution(attrTotals, formulaByAttr, "crit_damage", "dexterity*0.01", []formulaTerm{
		{AttrKey: "dexterity", Attribute: attrTotals["dexterity"], Coefficient: 0.01},
	})
	applyFormulaContribution(attrTotals, formulaByAttr, "crit_chance", "dexterity*0.0025 + luck*0.005", []formulaTerm{
		{AttrKey: "dexterity", Attribute: attrTotals["dexterity"], Coefficient: 0.0025},
		{AttrKey: "luck", Attribute: attrTotals["luck"], Coefficient: 0.005},
	})

	if got := attrTotals["speed"]; got != 10 {
		t.Fatalf("speed=%v, want 10", got)
	}
	if got := attrTotals["crit_damage"]; got != 0.1 {
		t.Fatalf("crit_damage=%v, want 0.1", got)
	}
	if got := attrTotals["crit_chance"]; got != 0.045 {
		t.Fatalf("crit_chance=%v, want 0.045", got)
	}
}
