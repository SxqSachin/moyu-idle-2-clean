package httpapi

import (
	"math"
	"testing"
	"time"

	"mysecondwebgame/server/internal/app"
	"mysecondwebgame/server/internal/config"
	"mysecondwebgame/server/internal/store"
)

func TestProjectBattlePlayerStatsUsesRawBaseLayer(t *testing.T) {
	attrStates := []store.PlayerAttributeState{
		{
			AttrKey:    "dexterity",
			FinalValue: 97,
			SourceMods: map[string]any{
				"version": "v2",
				"base": map[string]any{
					"value": 97,
				},
			},
		},
		{
			AttrKey:    "crit_chance",
			FinalValue: 0.97,
			SourceMods: map[string]any{
				"version": "v2",
				"base": map[string]any{
					"value": 0,
				},
			},
		},
	}

	projection := projectBattlePlayerStats(attrStates, map[string]float64{
		"dexterity":   97,
		"crit_chance": 0.97,
	})

	if got := projection.BaseStats["dexterity"]; got != 97 {
		t.Fatalf("base dexterity=%v, want 97", got)
	}
	if got := projection.BaseStats["crit_chance"]; got != 0 {
		t.Fatalf("base crit_chance=%v, want 0", got)
	}
	if got := projection.Stats["crit_chance"]; got != 0.97 {
		t.Fatalf("stats crit_chance=%v, want 0.97", got)
	}
}

func TestProjectBattlePlayerStatsPrefersAttributeStateOverDerivedCache(t *testing.T) {
	attrStates := []store.PlayerAttributeState{
		{
			AttrKey:    "Intelligence",
			FinalValue: 15,
			SourceMods: map[string]any{
				"version": "v2",
				"base": map[string]any{
					"value": 10,
				},
			},
		},
	}

	projection := projectBattlePlayerStats(attrStates, map[string]float64{
		"intelligence": 10, // stale cache value
	})

	if got := projection.Stats["intelligence"]; got != 15 {
		t.Fatalf("stats intelligence=%v, want 15", got)
	}
	if got := projection.BaseStats["intelligence"]; got != 10 {
		t.Fatalf("base intelligence=%v, want 10", got)
	}
}

func TestProjectBattleKittyStatsPrefersNewestDuplicateAttrKey(t *testing.T) {
	attrStates := []store.PlayerAttributeState{
		{
			AttrKey:     "intelligence",
			FinalValue:  1,
			LockVersion: 3,
			SourceMods: map[string]any{
				"version": "v2",
				"base": map[string]any{
					"value": 1,
				},
			},
		},
		{
			AttrKey:     "Intelligence",
			FinalValue:  6,
			LockVersion: 8,
			SourceMods: map[string]any{
				"version": "v2",
				"base": map[string]any{
					"value": 1,
				},
			},
		},
	}

	projection := projectBattleKittyStats(attrStates, map[string]float64{
		"intelligence": 1,
	})

	if got := projection.Stats["intelligence"]; got != 6 {
		t.Fatalf("kitty stats intelligence=%v, want 6", got)
	}
}

func TestBattleRecomputeUnitStatsUsesModeToAvoidDoubleCounting(t *testing.T) {
	settings := config.DefaultGlobalSettings()
	settings.DerivedStats = config.DerivedStatsSettings{
		Version:   "v2",
		Operators: []string{"+", "-", "*", "/"},
		Formulas: []config.DerivedStatFormula{
			{
				TargetAttr:  "crit_chance",
				IncludeBase: true,
				Terms: []config.DerivedStatFormulaTerm{
					{AttrKey: "dexterity", Coefficient: 0.01},
				},
			},
		},
	}
	srv := &Server{
		deps: app.Dependencies{
			Config: config.Config{
				GlobalSettings: settings,
			},
		},
	}

	t.Run("projected player stats recalc once", func(t *testing.T) {
		unit := battleUnitState{
			BaseStats: map[string]float64{
				"dexterity":   97,
				"crit_chance": 0.97,
			},
			Stats: map[string]float64{
				"dexterity":   97,
				"crit_chance": 0.97,
			},
			StatMode: battleStatModeProjected,
		}

		srv.battleRecomputeUnitStats(&unit)

		if math.Abs(unit.Stats["crit_chance"]-0.97) > 0.0000001 {
			t.Fatalf("projected crit_chance=%v, want 0.97", unit.Stats["crit_chance"])
		}
	})

	t.Run("projected snapshot keeps non-derived defense", func(t *testing.T) {
		unit := battleUnitState{
			BaseStats: map[string]float64{
				"physics_defence": 12,
			},
			Stats: map[string]float64{
				"physics_defence": 12,
			},
			StatMode: battleStatModeProjected,
		}

		srv.battleRecomputeUnitStats(&unit)

		if math.Abs(unit.Stats["physics_defence"]-12) > 0.0000001 {
			t.Fatalf("projected physics_defence=%v, want 12", unit.Stats["physics_defence"])
		}
	})

	t.Run("projected derived only applies on runtime delta", func(t *testing.T) {
		srvWithBuff := &Server{
			deps: app.Dependencies{
				Config: config.Config{
					GlobalSettings: settings,
				},
			},
		}
		srvWithBuff.deps.GameData.BuffEffects = []map[string]any{
			{
				"id":      "test_stamina_delta",
				"buffId":  "test_buff",
				"trigger": "while_active",
				"effects": []map[string]any{
					{
						"kind":  "addstat",
						"attr":  "dexterity",
						"value": 5,
					},
				},
			},
		}

		unit := battleUnitState{
			BaseStats: map[string]float64{
				"dexterity":   97,
				"crit_chance": 0.97,
			},
			Stats: map[string]float64{
				"dexterity":   97,
				"crit_chance": 0.97,
			},
			Buffs: []battleBuffState{
				{BuffID: "test_buff", RemainTurns: 1},
			},
			StatMode: battleStatModeProjected,
		}

		srvWithBuff.battleRecomputeUnitStats(&unit)

		// base 0.97 + delta dexterity(5)*0.01 = +0.05
		if math.Abs(unit.Stats["crit_chance"]-1.02) > 0.0000001 {
			t.Fatalf("projected crit_chance=%v, want 1.02", unit.Stats["crit_chance"])
		}
	})

	t.Run("direct battle stats stay direct", func(t *testing.T) {
		unit := battleUnitState{
			BaseStats: map[string]float64{
				"dexterity":   97,
				"crit_chance": 0.97,
			},
			Stats: map[string]float64{
				"dexterity":   97,
				"crit_chance": 0.97,
			},
			StatMode: battleStatModeDirect,
		}

		srv.battleRecomputeUnitStats(&unit)

		if math.Abs(unit.Stats["crit_chance"]-0.97) > 0.0000001 {
			t.Fatalf("direct crit_chance=%v, want 0.97", unit.Stats["crit_chance"])
		}
	})

	t.Run("negative hp becomes zero max hp and dead", func(t *testing.T) {
		unit := battleUnitState{
			UnitID: "player:1",
			Side:   "player",
			UserID: 1,
			Alive:  true,
			HP:     0,
			MaxHP:  0,
			BaseStats: map[string]float64{
				"hp": -50,
			},
			Stats: map[string]float64{
				"hp": -50,
			},
			StatMode: battleStatModeProjected,
		}

		srv.battleRecomputeUnitStats(&unit)

		if unit.MaxHP != 0 {
			t.Fatalf("negative hp should produce MaxHP=0, got %v", unit.MaxHP)
		}
		if unit.HP != 0 {
			t.Fatalf("negative hp should clamp current HP to 0, got %v", unit.HP)
		}
		if unit.Alive {
			t.Fatalf("unit with zero max hp should be dead")
		}
	})
}

func TestBattleProjectDerivedFinalStatsForEnemySpawn(t *testing.T) {
	settings := config.DefaultGlobalSettings()
	settings.DerivedStats = config.DerivedStatsSettings{
		Version:   "v2",
		Operators: []string{"+", "-", "*", "/"},
		Formulas: []config.DerivedStatFormula{
			{
				TargetAttr:  "hp",
				IncludeBase: true,
				Terms: []config.DerivedStatFormulaTerm{
					{AttrKey: "stamina", Coefficient: 7},
				},
			},
		},
	}
	srv := &Server{
		deps: app.Dependencies{
			Config: config.Config{
				GlobalSettings: settings,
			},
		},
	}

	projected := srv.battleProjectDerivedFinalStats(map[string]float64{
		"hp":      10,
		"stamina": 5,
	})
	if math.Abs(projected["hp"]-45) > 0.0000001 {
		t.Fatalf("enemy projected hp=%v, want 45", projected["hp"])
	}
}

func TestBattleProjectDerivedFinalStatsSupportsMagicCritAndPenetration(t *testing.T) {
	settings := config.DefaultGlobalSettings()
	settings.DerivedStats = config.DerivedStatsSettings{
		Version:   "v2",
		Operators: []string{"+", "-", "*", "/"},
		Formulas: []config.DerivedStatFormula{
			{
				TargetAttr:  "magic_crit_chance",
				IncludeBase: true,
				Terms: []config.DerivedStatFormulaTerm{
					{AttrKey: "perception", Coefficient: 0.005},
				},
			},
			{
				TargetAttr:  "magic_crit_damage",
				IncludeBase: true,
				Terms: []config.DerivedStatFormulaTerm{
					{AttrKey: "perception", Coefficient: 0.005},
					{AttrKey: "intelligence", Coefficient: 0.001},
				},
			},
			{
				TargetAttr:  "crit_damage_resist",
				IncludeBase: true,
				Terms: []config.DerivedStatFormulaTerm{
					{AttrKey: "perception", Coefficient: 0.001},
					{AttrKey: "stamina", Coefficient: 0.002},
				},
			},
			{
				TargetAttr:  "magicial_flat_defense_penetration",
				IncludeBase: true,
				Terms: []config.DerivedStatFormulaTerm{
					{AttrKey: "perception", Coefficient: 0.15},
				},
			},
		},
	}
	srv := &Server{
		deps: app.Dependencies{
			Config: config.Config{
				GlobalSettings: settings,
			},
		},
	}
	projected := srv.battleProjectDerivedFinalStats(map[string]float64{
		"perception":   100,
		"intelligence": 50,
		"stamina":      40,
	})

	if math.Abs(projected["magic_crit_chance"]-0.5) > 0.0000001 {
		t.Fatalf("magic_crit_chance=%v, want 0.5", projected["magic_crit_chance"])
	}
	if math.Abs(projected["magic_crit_damage"]-0.55) > 0.0000001 {
		t.Fatalf("magic_crit_damage=%v, want 0.55", projected["magic_crit_damage"])
	}
	if math.Abs(projected["crit_damage_resist"]-0.18) > 0.0000001 {
		t.Fatalf("crit_damage_resist=%v, want 0.18", projected["crit_damage_resist"])
	}
	if math.Abs(projected["magicial_flat_defense_penetration"]-15) > 0.0000001 {
		t.Fatalf("magicial_flat_defense_penetration=%v, want 15", projected["magicial_flat_defense_penetration"])
	}
}

func TestBattleStatRecalcActionCarriesDeltaStats(t *testing.T) {
	settings := config.DefaultGlobalSettings()
	srv := &Server{
		deps: app.Dependencies{
			Config: config.Config{
				GlobalSettings: settings,
			},
		},
	}
	srv.deps.GameData.BuffEffects = []map[string]any{
		{
			"id":      "delta_stat_rule",
			"buffId":  "delta_buff",
			"trigger": "while_active",
			"effects": []map[string]any{
				{
					"kind":  "addstat",
					"attr":  "physics_defence",
					"value": 3,
				},
			},
		},
	}
	unit := battleUnitState{
		UnitID:    "player:1",
		BaseStats: map[string]float64{"physics_defence": 10, "hp": 100, "mp": 20},
		Stats:     map[string]float64{"physics_defence": 10, "hp": 100, "mp": 20},
		StatMode:  battleStatModeProjected,
		Buffs: []battleBuffState{
			{BuffID: "delta_buff", RemainTurns: 1},
		},
	}

	actions := srv.battleTriggerUnitBuffHooks(nil, time.Now(), &unit, "while_active")
	found := false
	for _, action := range actions {
		if action["kind"] != "stat_recalc" {
			continue
		}
		deltaStats, ok := action["deltaStats"].(map[string]float64)
		if !ok {
			t.Fatalf("stat_recalc missing deltaStats: %#v", action["deltaStats"])
		}
		if math.Abs(deltaStats["physics_defence"]-3) > 0.0000001 {
			t.Fatalf("deltaStats.physics_defence=%v, want 3", deltaStats["physics_defence"])
		}
		found = true
	}
	if !found {
		t.Fatalf("stat_recalc action not found")
	}
}

func TestApplyEnemyHpRegenRatio(t *testing.T) {
	sharedSettings := config.DefaultSharedGlobalSettings()
	sharedSettings.CombatBalanceModel.EnemyHpRegenRatio = 0.34
	srv := &Server{
		deps: app.Dependencies{
			Config: config.Config{
				SharedGlobalSettings: sharedSettings,
			},
		},
	}

	// Enemy unit should have hp_regen reduced
	enemy := &battleUnitState{
		UnitID:       "enemy:1",
		IsPlayerUnit: false,
		Stats:        map[string]float64{"hp_regen": 10, "hp": 100},
		BaseStats:    map[string]float64{"hp_regen": 10, "hp": 100},
	}
	srv.applyEnemyHpRegenRatio(enemy)
	if got := enemy.Stats["hp_regen"]; math.Abs(got-3.4) > 0.0001 {
		t.Fatalf("enemy hp_regen=%v, want 3.4", got)
	}
	if got := enemy.BaseStats["hp_regen"]; math.Abs(got-3.4) > 0.0001 {
		t.Fatalf("enemy baseStats hp_regen=%v, want 3.4", got)
	}
	// Other stats should be unchanged
	if got := enemy.Stats["hp"]; got != 100 {
		t.Fatalf("enemy hp=%v, want 100", got)
	}

	// Player unit should NOT have hp_regen reduced
	player := &battleUnitState{
		UnitID:       "player:1",
		IsPlayerUnit: true,
		Stats:        map[string]float64{"hp_regen": 10, "hp": 100},
		BaseStats:    map[string]float64{"hp_regen": 10, "hp": 100},
	}
	srv.applyEnemyHpRegenRatio(player)
	if got := player.Stats["hp_regen"]; got != 10 {
		t.Fatalf("player hp_regen=%v, want 10 (unchanged)", got)
	}
}

func TestBattleResolveEffectiveDefensePercentPenetrationCap(t *testing.T) {
	// battleResolveEffectiveDefense internally clamps percent to [0, 1] as safety net.
	// The actual gameplay cap (0.8 by default) is applied at the call site via config.
	// Test that 100% penetration fully ignores defense.
	defense := 100.0
	flatPen := 0.0

	// 100% percent pen (internal max) → effective defense = 0
	effective := battleResolveEffectiveDefense(defense, flatPen, 1.0)
	if effective != 0 {
		t.Fatalf("effective defense with 100%% pen=%v, want 0", effective)
	}

	// 80% percent pen → effective defense = defense * (1 - 0.8) = 20
	effective = battleResolveEffectiveDefense(defense, flatPen, 0.8)
	if math.Abs(effective-20) > 0.0001 {
		t.Fatalf("effective defense with 80%% pen=%v, want 20", effective)
	}

	// Negative percent pen → clamped to 0 → effective defense = defense
	effective = battleResolveEffectiveDefense(defense, flatPen, -0.5)
	if math.Abs(effective-defense) > 0.0001 {
		t.Fatalf("effective defense with negative pen=%v, want %v", effective, defense)
	}
}
