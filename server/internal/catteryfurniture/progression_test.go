package catteryfurniture

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestAuthoredProgression(t *testing.T) {
	raw, err := os.ReadFile("../../../packages/game-data/source/json/cattery_furniture.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		Rows []Definition `json:"cattery_furniture"`
	}
	if err = json.Unmarshal(raw, &catalog); err != nil {
		t.Fatal(err)
	}
	for _, root := range catalog.Rows {
		for level := 1; level <= len(root.Levels)+1; level++ {
			d, err := root.AtLevel(level)
			if err != nil {
				t.Fatal(err)
			}
			expected := int64(1400 + level*100)
			if root.ID == "loot_cabinet" {
				expected = int64(300 + level*200)
			}
			if root.ID == "kitty_training_bench" {
				expected = int64(level * 2000)
				if level > 5 {
					expected = int64(10000 + (level-5)*800)
				}
				if !d.Permanent() || d.CapacitySeconds != 0 || len(d.RefillCosts) != 0 {
					t.Fatal("permanent furniture has upkeep")
				}
			}
			if d.Rules[0].RateBPS != expected || d.CurrentLevel() != level || len(d.Levels) != 0 {
				t.Fatalf("bad level: %+v", d)
			}
			for id, n := range root.RefillCosts {
				if d.RefillCosts[id] < n {
					t.Fatalf("removed base cost %s", id)
				}
			}
			for _, stage := range root.Levels {
				if stage.Level <= level {
					for id, n := range stage.RefillCostAdd {
						if d.RefillCosts[id] < n {
							t.Fatalf("removed tier cost %s", id)
						}
					}
				}
			}
			if d.Source().Level != level {
				t.Fatal("snapshot lost level")
			}
		}
		if _, err := root.AtLevel(len(root.Levels) + 2); err == nil {
			t.Fatal("level beyond catalog accepted")
		}
	}
}

func TestUpgradeSupplementExactAndMonotone(t *testing.T) {
	old := Definition{RefillSeconds: 36000, RefillCosts: map[string]int64{"wood": 2, "iron": 3}}
	next := Definition{RefillSeconds: 36000, RefillCosts: map[string]int64{"wood": 2, "iron": 4, "silver": 2}}
	for _, tc := range []struct {
		ms   int64
		want map[string]int64
	}{
		{0, map[string]int64{}}, {1, map[string]int64{"iron": 1, "silver": 1}},
		{36000000, map[string]int64{"iron": 1, "silver": 2}},
		{36000001, map[string]int64{"iron": 2, "silver": 3}},
		{120000000, map[string]int64{"iron": 4, "silver": 7}},
	} {
		got, err := UpgradeSupplement(old, next, tc.ms)
		if err != nil || !reflect.DeepEqual(got, tc.want) {
			t.Fatalf("%d: %v %v", tc.ms, got, err)
		}
	}
	next.RefillSeconds = 72000
	got, err := UpgradeSupplement(old, next, 36000000)
	if err != nil || !reflect.DeepEqual(got, map[string]int64{"silver": 1}) {
		t.Fatalf("rate difference: %v %v", got, err)
	}
	if _, err := UpgradeSupplement(old, next, -1); err == nil {
		t.Fatal("negative time accepted")
	}
}
