package httpapi

import (
	"math"
	"testing"
	"time"

	battleskills "mysecondwebgame/server/internal/battle/skills"
)

func TestEnemyOnlyDamageReductionPierceRequiresEnemyBaseSideAndMatchingStatus(t *testing.T) {
	tests := []struct {
		name             string
		sourceSide       string
		sourceBaseSide   string
		sourceOriginSide string
		sourceUnitType   string
		targetBuffs      []battleBuffState
		damageType       string
		resistStat       string
		statuses         []string
		wantDamage       float64
		wantApplied      bool
	}{
		{
			name:           "enemy skill pierces matching physical resist",
			sourceSide:     "enemy",
			sourceBaseSide: "enemy",
			targetBuffs:    []battleBuffState{{BuffID: "candy_poison", RemainTurns: 2}},
			damageType:     "physical",
			resistStat:     "physics_damage_resist",
			statuses:       []string{"candy_poison"},
			wantDamage:     60,
			wantApplied:    true,
		},
		{
			name:           "enemy skill pierces matching magical resist",
			sourceSide:     "enemy",
			sourceBaseSide: "enemy",
			targetBuffs:    []battleBuffState{{BuffID: "ghostly_rider_preyscent", RemainTurns: 2}},
			damageType:     "magical",
			resistStat:     "magical_damage_resist",
			statuses:       []string{"ghostly_rider_preyscent"},
			wantDamage:     60,
			wantApplied:    true,
		},
		{
			name:           "missing mark keeps full resist",
			sourceSide:     "enemy",
			sourceBaseSide: "enemy",
			damageType:     "physical",
			resistStat:     "physics_damage_resist",
			statuses:       []string{"candy_poison"},
			wantDamage:     20,
		},
		{
			name:           "player cannot obtain monster pierce",
			sourceSide:     "player",
			sourceBaseSide: "player",
			targetBuffs:    []battleBuffState{{BuffID: "candy_poison", RemainTurns: 2}},
			damageType:     "physical",
			resistStat:     "physics_damage_resist",
			statuses:       []string{"candy_poison"},
			wantDamage:     20,
		},
		{
			name:             "player side summon cannot obtain monster pierce",
			sourceSide:       "player",
			sourceBaseSide:   "player",
			sourceOriginSide: "player",
			sourceUnitType:   "summon",
			targetBuffs:      []battleBuffState{{BuffID: "candy_poison", RemainTurns: 2}},
			damageType:       "physical",
			resistStat:       "physics_damage_resist",
			statuses:         []string{"candy_poison"},
			wantDamage:       20,
		},
		{
			name:             "perspective inversion does not remove authored enemy pierce",
			sourceSide:       "player",
			sourceBaseSide:   "enemy",
			sourceOriginSide: "enemy",
			targetBuffs:      []battleBuffState{{BuffID: "candy_poison", RemainTurns: 2}},
			damageType:       "physical",
			resistStat:       "physics_damage_resist",
			statuses:         []string{"candy_poison"},
			wantDamage:       60,
			wantApplied:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source := battleUnitState{
				UnitID:     "source",
				Side:       tt.sourceSide,
				BaseSide:   tt.sourceBaseSide,
				OriginSide: tt.sourceOriginSide,
				UnitType:   tt.sourceUnitType,
				Alive:      true,
				HP:         100,
				MaxHP:      100,
				Stats:      map[string]float64{},
			}
			target := battleUnitState{
				UnitID:   "target",
				Side:     battleOppositeSide(tt.sourceSide),
				BaseSide: battleOppositeSide(tt.sourceBaseSide),
				Alive:    true,
				HP:       1000,
				MaxHP:    1000,
				Stats:    map[string]float64{tt.resistStat: 0.8},
				Buffs:    tt.targetBuffs,
			}
			state := &battleRuntimeState{Seed: 17}
			if normalizeBattleSide(source.Side) == "enemy" {
				state.Enemies = []battleUnitState{source}
				state.Players = []battleUnitState{target}
			} else {
				state.Players = []battleUnitState{source}
				state.Enemies = []battleUnitState{target}
			}
			sourceRef := battleUnitByID(state, source.UnitID)
			targetRef := battleUnitByID(state, target.UnitID)
			runtime := newBattleSkillActionRuntime(state, sourceRef, time.UnixMilli(1), nil, nil)
			result := runtime.ApplyDamage(battleskills.DamageAction{
				SourceID:                       source.UnitID,
				TargetID:                       target.UnitID,
				Amount:                         100,
				DamageType:                     tt.damageType,
				EnemyOnlyDamageReductionPierce: 0.5,
				EnemyOnlyDamageReductionPierceTargetStatuses: tt.statuses,
				Reason: "enemy_only_pierce_test",
			})

			if math.Abs(result.Amount-tt.wantDamage) > 0.0000001 {
				t.Fatalf("damage=%v, want %v", result.Amount, tt.wantDamage)
			}
			if got := toFloat64(result.Meta["enemyOnlyDamageReductionPierceApplied"], 0) > 0; got != tt.wantApplied {
				t.Fatalf("pierce applied=%v, want %v; meta=%#v", got, tt.wantApplied, result.Meta)
			}
			if got := targetRef.HP; math.Abs(got-(1000-tt.wantDamage)) > 0.0000001 {
				t.Fatalf("target hp=%v, want %v", got, 1000-tt.wantDamage)
			}
			if state.RNGIndex != 0 {
				t.Fatalf("pierce must not consume RNG, index=%d", state.RNGIndex)
			}
		})
	}
}

func TestParseBattleDamageSpecNormalizesEnemyOnlyPierce(t *testing.T) {
	spec := parseBattleDamageSpec(map[string]any{
		"enemyOnlyDamageReductionPierce": 1.5,
		"enemyOnlyDamageReductionPierceTargetStatuses": []any{
			"candy_poison",
			"candy_poison",
			"ghostly_rider_preyscent",
		},
	}, "physical", battleSkillAmountFormula{BaseAdd: 10})

	if spec.EnemyOnlyDamageReductionPierce != 1 {
		t.Fatalf("pierce=%v, want runtime clamp 1", spec.EnemyOnlyDamageReductionPierce)
	}
	if len(spec.EnemyOnlyDamageReductionPierceTargetStatuses) != 2 ||
		spec.EnemyOnlyDamageReductionPierceTargetStatuses[0] != "candy_poison" ||
		spec.EnemyOnlyDamageReductionPierceTargetStatuses[1] != "ghostly_rider_preyscent" {
		t.Fatalf("statuses=%#v", spec.EnemyOnlyDamageReductionPierceTargetStatuses)
	}
	invalid := parseBattleDamageSpec(map[string]any{
		"enemyOnlyDamageReductionPierce": math.NaN(),
	}, "physical", battleSkillAmountFormula{BaseAdd: 10})
	if invalid.EnemyOnlyDamageReductionPierce != 0 {
		t.Fatalf("non-finite pierce=%v, want 0", invalid.EnemyOnlyDamageReductionPierce)
	}
}
