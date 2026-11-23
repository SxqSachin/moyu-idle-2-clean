package catteryfurniture

import (
	"encoding/json"
	"mysecondwebgame/server/internal/operationeffect"
	"testing"
	"time"
)

func TestFurnitureJSONRejectsUnknownFields(t *testing.T) {
	base := `{"id":"desk","version":"v1","nameKey":"name","iconId":"item.desk","installCosts":{"wood":10},"refillCosts":{"wood":2},"capacitySeconds":120000,"refillSeconds":36000,"rules":[{"kind":"owner_experience","domain":"life_action","rateBps":1500}]}`
	var d Definition
	if err := json.Unmarshal([]byte(base), &d); err != nil {
		t.Fatal(err)
	}
	var raw map[string]any
	if err := json.Unmarshal([]byte(base), &raw); err != nil {
		t.Fatal(err)
	}
	raw["script"] = "not supported"
	b, _ := json.Marshal(raw)
	if json.Unmarshal(b, &d) == nil {
		t.Fatal("unknown definition field accepted")
	}
	delete(raw, "script")
	raw["rules"].([]any)[0].(map[string]any)["ratePercent"] = 15
	b, _ = json.Marshal(raw)
	if json.Unmarshal(b, &d) == nil {
		t.Fatal("unknown effect field accepted")
	}
}

func TestFurnitureDefinitionAndMaintenanceClock(t *testing.T) {
	d := Definition{ID: "desk", Version: "v1", NameKey: "name", IconID: "item.desk", InstallCosts: map[string]int64{"wood": 10}, RefillCosts: map[string]int64{"wood": 2}, CapacitySeconds: 120000, RefillSeconds: 36000, Rules: []operationeffect.PassiveRule{{Kind: operationeffect.KindOwnerExperience, Domain: operationeffect.DomainLifeAction, RateBPS: 1500}}}
	if err := d.Validate(); err != nil {
		t.Fatal(err)
	}
	at := time.Unix(100, 0)
	until := at.Add(time.Hour)
	if Remaining(9, &until, at) != 3600000 || Remaining(9, nil, at) != 9 || Remaining(9, &until, until) != 0 || Remaining(9, &until, until.Add(time.Hour)) != 0 {
		t.Fatal("maintenance must be elapsed time, paused balance or zero")
	}
	d.RefillCosts["wood"] = 0
	if d.Validate() == nil {
		t.Fatal("free maintenance accepted")
	}
}
