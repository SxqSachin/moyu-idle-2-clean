package catalog

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

func TestLoadSpecializationCatalogSource(t *testing.T) {
	t.Parallel()

	path := filepath.Join("..", "..", "..", "packages", "game-data", "source", "json", "specialization_trees.json")
	data, err := Load(path)
	if err != nil {
		t.Fatalf("load specialization source catalog: %v", err)
	}
	tree := data.SpecializationTreeByID("player_combat_specialization")
	if tree == nil {
		t.Fatal("player specialization tree missing")
	}
	if tree.SchemaVersion != 2 || tree.RulesetVersion != "specialization-v6" || tree.BalanceRevision != 3 {
		t.Fatalf("unexpected specialization versions: schema=%d ruleset=%q balance=%d", tree.SchemaVersion, tree.RulesetVersion, tree.BalanceRevision)
	}
	if tree.MaxPoints != 65 || tree.MaxActiveRootNodes != 3 {
		t.Fatalf("unexpected specialization choice budget: maxPoints=%d maxActiveRootNodes=%d", tree.MaxPoints, tree.MaxActiveRootNodes)
	}
	if len(tree.Schools) != 5 || len(tree.Nodes) != 70 {
		t.Fatalf("unexpected specialization shape: schools=%d nodes=%d", len(tree.Schools), len(tree.Nodes))
	}
	totalTreeCost := 0
	kindCounts := map[string]int{}
	kindCosts := map[string]int{}
	for _, node := range tree.Nodes {
		kindCounts[node.Kind]++
		nodeCost := 0
		for _, level := range node.Levels {
			nodeCost += level.Cost
		}
		totalTreeCost += nodeCost
		kindCosts[node.Kind] += nodeCost
		switch node.Kind {
		case "root":
			if len(node.Levels) != 6 || node.Levels[0].Cost != 2 || nodeCost != 7 {
				t.Fatalf("root %s no longer uses the frozen [2,1,1,1,1,1] cost curve: %#v", node.ID, node.Levels)
			}
		case "technique":
			if len(node.Levels) != 4 || node.Levels[0].Cost != 2 || nodeCost != 5 {
				t.Fatalf("technique %s no longer uses the frozen [2,1,1,1] cost curve: %#v", node.ID, node.Levels)
			}
		case "terminal":
			switch node.ID {
			case "strength_terminal_three_armed":
				if len(node.Levels) != 1 || nodeCost != 22 {
					t.Fatalf("three-armed topology must remain a late 22-point commitment: %#v", node.Levels)
				}
			case "dexterity_terminal_shadow_dance", "intelligence_terminal_archmage", "stamina_terminal_absolute_defense":
				if len(node.Levels) != 4 || node.Levels[3].Cost != 20 || nodeCost != 28 {
					t.Fatalf("composite topology terminal %s lost its late 20-point final rank: %#v", node.ID, node.Levels)
				}
			default:
				if len(node.Levels) != 3 || nodeCost != 8 || node.Levels[0].Cost != 4 || node.Levels[1].Cost != 2 || node.Levels[2].Cost != 2 {
					t.Fatalf("ordinary terminal %s no longer uses [4,2,2]: %#v", node.ID, node.Levels)
				}
			}
		case "confluence":
			if nodeCost != 10 || (node.ID != "confluence_intelligence_perception_crownless_astrolabe" && (len(node.Levels) != 3 || node.Levels[0].Cost != 6 || node.Levels[1].Cost != 2 || node.Levels[2].Cost != 2)) {
				t.Fatalf("confluence %s no longer has total cost 10 and ordinary [6,2,2] ranks: %#v", node.ID, node.Levels)
			}
		}
	}
	if totalTreeCost != 604 || kindCounts["root"] != 10 || kindCounts["technique"] != 20 || kindCounts["terminal"] != 20 || kindCounts["confluence"] != 20 || kindCosts["root"] != 70 || kindCosts["technique"] != 100 || kindCosts["terminal"] != 234 || kindCosts["confluence"] != 200 {
		t.Fatalf("unexpected frozen specialization budget: total=%d counts=%v costs=%v", totalTreeCost, kindCounts, kindCosts)
	}
	for _, node := range tree.Nodes {
		nodeRankRequirements := []SpecializationRequirement{}
		pureSchoolRequirements := []SpecializationRequirement{}
		for _, requirement := range node.Requirements.All {
			switch requirement.Kind {
			case "node_rank":
				nodeRankRequirements = append(nodeRankRequirements, requirement)
			case "pure_school_points_gte":
				pureSchoolRequirements = append(pureSchoolRequirements, requirement)
			}
		}
		for _, group := range node.Requirements.AnyOf {
			for _, requirement := range group {
				if requirement.Kind == "node_rank" {
					nodeRankRequirements = append(nodeRankRequirements, requirement)
				}
			}
		}
		switch node.Kind {
		case "technique":
			if len(nodeRankRequirements) != 1 || nodeRankRequirements[0].MinRank != 6 {
				t.Fatalf("technique %s must require one max-rank root: %#v", node.ID, node.Requirements)
			}
			parent := tree.NodeByID(nodeRankRequirements[0].NodeID)
			if parent == nil || parent.Kind != "root" {
				t.Fatalf("technique %s prerequisite is not a root: %#v", node.ID, nodeRankRequirements[0])
			}
		case "terminal":
			if len(nodeRankRequirements) == 0 || len(pureSchoolRequirements) != 1 || pureSchoolRequirements[0].Points != 11 {
				t.Fatalf("terminal %s lost its max-technique and 11-point school gates: %#v", node.ID, node.Requirements)
			}
			for _, requirement := range nodeRankRequirements {
				parent := tree.NodeByID(requirement.NodeID)
				if requirement.MinRank != 4 || parent == nil || parent.Kind != "technique" {
					t.Fatalf("terminal %s has an invalid technique gate: %#v", node.ID, requirement)
				}
			}
		case "confluence":
			wantMinRank, wantSchoolPoints := 2, 9
			if node.ID == "confluence_intelligence_perception_crownless_astrolabe" {
				wantMinRank, wantSchoolPoints = 4, 11
			}
			if len(node.Requirements.AnyOf) != 2 || len(pureSchoolRequirements) != 2 {
				t.Fatalf("confluence %s must expose one technique-choice group per school: %#v", node.ID, node.Requirements)
			}
			for _, requirement := range pureSchoolRequirements {
				if requirement.Points != wantSchoolPoints {
					t.Fatalf("confluence %s has an invalid school-point gate: %#v", node.ID, requirement)
				}
			}
			for _, requirement := range nodeRankRequirements {
				parent := tree.NodeByID(requirement.NodeID)
				if requirement.MinRank != wantMinRank || parent == nil || parent.Kind != "technique" {
					t.Fatalf("confluence %s has an invalid technique gate: %#v", node.ID, requirement)
				}
			}
		}
	}
	if len(tree.Keywords) != 24 {
		t.Fatalf("specialization v6 glossary was truncated: keywords=%d", len(tree.Keywords))
	}
	breakMomentum := tree.KeywordByID("break_momentum")
	if breakMomentum == nil || breakMomentum.MechanicKind == "" || len(breakMomentum.RuntimeIDs) == 0 || len(breakMomentum.SourceNodeIDs) == 0 {
		t.Fatalf("break-momentum keyword contract was truncated: %#v", breakMomentum)
	}
	if got := tree.I18n["zh-CN"].Keywords["break_momentum"]; got.Name == "" || got.PlayerSummary == "" || got.MechanicDetails == "" || got.SourceHint == "" {
		t.Fatalf("break-momentum localized glossary was truncated: %#v", got)
	}
	if got := tree.I18n["zh-CN"].Nodes["strength_root_break_stance"].Name; got != "蓄势重击" {
		t.Fatalf("specialization source locale was not retained: %q", got)
	}
	if got := tree.I18n["zh-CN"].Nodes["strength_root_break_stance"].EffectSummary; len(got) == 0 {
		t.Fatal("specialization source effect summary was not retained")
	}
	if got := tree.I18n["zh-CN"].Nodes["strength_root_break_stance"].LevelEffectSummary; len(got) != 6 || len(got[1]) < 2 {
		t.Fatalf("specialization per-rank effect summaries were not retained: %#v", got)
	}

	pointsByLevel := map[int]int{
		9:   0,
		10:  2,
		15:  3,
		90:  18,
		99:  18,
		100: 19,
		120: 23,
		195: 38,
		200: 40,
		210: 42,
		300: 60,
		349: 60,
		350: 61,
		450: 63,
		499: 63,
		500: 65,
	}
	for level, want := range pointsByLevel {
		if got := tree.PointsAtBattleLevel(level); got != want {
			t.Fatalf("PointsAtBattleLevel(%d)=%d, want %d", level, got, want)
		}
	}

	threeArmed := tree.NodeByID("strength_terminal_three_armed")
	if threeArmed == nil || len(threeArmed.Levels) != 1 || len(threeArmed.Levels[0].Grants) != 1 {
		t.Fatal("three-armed terminal grant missing")
	}
	grant := threeArmed.Levels[0].Grants[0]
	if grant.Kind != "equipment_slot_capacity" || grant.EquipType != "battle" || grant.Deltas["accessory"] != -2 || grant.Deltas["right_hand"] != 1 {
		t.Fatalf("unexpected three-armed topology grant: %#v", grant)
	}
	crownless := tree.NodeByID("confluence_intelligence_perception_crownless_astrolabe")
	if crownless == nil || len(crownless.Levels) != 1 || len(crownless.Levels[0].Grants) != 1 {
		t.Fatal("crownless astrolabe confluence grant missing")
	}
	crownlessGrant := crownless.Levels[0].Grants[0]
	if crownlessGrant.Kind != "equipment_slot_capacity" || crownlessGrant.Deltas["head"] != -1 || crownlessGrant.Deltas["accessory"] != 2 {
		t.Fatalf("unexpected crownless topology grant: %#v", crownlessGrant)
	}
	if got := 4 + grant.Deltas["accessory"] + crownlessGrant.Deltas["accessory"]; got != 4 {
		t.Fatalf("combined accessory capacity=%d, want 4", got)
	}
	if got := 1 + grant.Deltas["right_hand"] + crownlessGrant.Deltas["right_hand"]; got != 2 {
		t.Fatalf("combined right-hand capacity=%d, want 2", got)
	}
	if got := 1 + grant.Deltas["head"] + crownlessGrant.Deltas["head"]; got != 0 {
		t.Fatalf("combined head capacity=%d, want 0", got)
	}

	root := tree.NodeByID("strength_root_break_stance")
	if root == nil || len(root.Levels) != 6 || len(root.Levels[0].Grants) < 1 {
		t.Fatal("strength root loop grants missing")
	}
	if len(root.KeywordIDs) == 0 || root.KeywordIDs[0] != "break_momentum" {
		t.Fatalf("strength root keyword ids were not retained: %#v", root.KeywordIDs)
	}
	for _, level := range root.Levels {
		var retainedHook bool
		for _, rootGrant := range level.Grants {
			if rootGrant.Kind != "battle_hook" || len(rootGrant.BattleHooks) == 0 {
				continue
			}
			if rootGrant.BattleHooks[0]["trigger"] == "onAfterDealDamage" {
				retainedHook = true
			}
		}
		if !retainedHook {
			t.Fatalf("rank %d dropped the specialization battle hook payload", level.Level)
		}
	}
}

func TestSpecializationRequirementRoundTripRetainsSchoolPointContract(t *testing.T) {
	raw := []byte(`{"kind":"pure_school_points_gte","schoolId":"strength","points":3}`)
	var requirement SpecializationRequirement
	if err := json.Unmarshal(raw, &requirement); err != nil {
		t.Fatal(err)
	}
	if requirement.Kind != "pure_school_points_gte" || requirement.SchoolID != "strength" || requirement.Points != 3 {
		t.Fatalf("school-point requirement was truncated by the catalog projection: %#v", requirement)
	}
	roundTrip, err := json.Marshal(requirement)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(roundTrip, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["schoolId"] != "strength" || decoded["points"] != float64(3) {
		t.Fatalf("school-point requirement was lost before the API projection: %s", roundTrip)
	}
}
