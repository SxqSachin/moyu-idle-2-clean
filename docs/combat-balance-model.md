# Combat Balance Model

This document is the source-of-truth design note for combat number tuning. It complements the editable `combatBalanceModel` block in `packages/game-config/source/shared-global-config.json` and the diagnostics in `combat-model-vue3`.

## Goals

- Player battle level cap is 500.
- Battle level grants 500 allocatable primary-stat points in total.
- Allocatable battle primary stats are strength, dexterity, intelligence, perception, and stamina.
- Luck is not manually allocatable. It should stay a secondary variance/reward stat from gear, affixes, buffs, titles, or special content.
- Single-player plus kitties must be viable. Multiplayer may be slightly more efficient and more forgiving, but must not be mandatory.
- Different professions, equipment identities, and combo effects should create multiple good answers, not one spreadsheet-solved answer.

## Level And Attribute Model

The combat model treats player strength as two layers:

- Battle level: provides the point budget and the long-term growth pacing.
- Build budget: how the player spends points, equips gear, chooses skills, and uses affixes/combo effects.

At level 500, the model assumes 500 total primary points. A balanced character is not expected to put 100 points into each stat; instead, the model should evaluate several archetypes:

- Physical striker: strength and dexterity lead, stamina secondary.
- Magical striker: intelligence and perception lead, stamina secondary.
- Defender: stamina leads, strength/perception secondary for mixed defense.
- Tempo/crit build: dexterity leads, but its value must be capped by encounter length and resource pressure.
- Support/summon build: intelligence/perception lead, with MP and trigger windows as constraints.

Current derived formulas make the primary stats mean:

- Strength: physical damage, physical defense, HP, physical penetration, some crit damage.
- Dexterity: speed, physical damage, physical crit chance, physical crit damage.
- Intelligence: magical damage, MP, MP regen, magic penetration, magic crit damage.
- Perception: magical damage, magical defense, magic crit chance/damage, crit damage resist, magic penetration.
- Stamina: HP, HP regen, both defenses, crit damage resist.
- Luck: crit chances, drop/variance bonuses through non-point sources.

Design boundary:

- Speed should be worth roughly 7%-18% of a build's effective combat budget. If speed only sorts once per round, keep its coefficients modest or add more speed-sensitive hooks; if speed changes action frequency, cap its budget harder.
- Perception should not be "magic dexterity plus better defense" without cost. Its magic crit and defense value should be balanced against intelligence's raw spell damage and MP.
- Luck should improve variance and reward texture, not replace dexterity/perception as the best crit stat.

## Baseline Curve

`combatBalanceModel.levelBands` defines the expected single-unit baseline. The current v1 targets are:

| Level | DPR | HP | MP | Speed | Exp Rate |
| ---: | ---: | ---: | ---: | ---: | ---: |
| 1 | 12 | 115 | 28 | 7 | 1.00 |
| 50 | 140 | 1500 | 110 | 28 | 1.20 |
| 100 | 330 | 3500 | 190 | 48 | 1.55 |
| 200 | 950 | 9000 | 360 | 85 | 2.20 |
| 300 | 1900 | 16000 | 540 | 120 | 2.85 |
| 400 | 3400 | 26000 | 760 | 155 | 3.50 |
| 500 | 5600 | 40000 | 1000 | 190 | 4.20 |

These numbers are not raw naked-stat values. They represent an expected usable player unit after normal gear, skills, and primary-point allocation for that level band.

Defense uses diminishing mitigation with a fixed saturation constant K:

```text
effectiveDefense = max(0, (defense - flatPen) * (1 - percentPen))
mitigationRatio  = effectiveDefense / (K + effectiveDefense)
damageBeforeCrit = rawDamage * (1 - mitigationRatio)
```

`K = defenseKBase` (fixed, default 100). At K=100: 100 defense → 50% mitigation, 300 defense → 75%.
Flat penetration is applied first (directly reduces the defense value), then percent penetration scales the remainder.

## Enemy And Experience Model

Enemy numbers should be derived from target experience, not filled by feel:

```text
targetRounds:
  normal: 4-8
  elite: 8-15
  boss: 15-35
  world_boss: 35-60

recommendedHp = playerDpr(level) * targetRounds * teamScale
recommendedExp = enemyPowerScore * expPerPower * levelExpRate * tierRewardMultiplier
```

Enemy power score includes attributes, equipment, skills, drops, and tier multipliers. It is okay for a mechanically interesting enemy to have lower raw stats, but the total score should still explain its exp.

Important current issue:

- The generated `attribute_exp_profiles` base curve currently reaches level 199, not 500.
- A level-500 cap needs either 500 rows in the source workbook/extra table or a build-time formula generator.
- Until that source is extended, combat-model should treat 200-500 as design targets rather than confirmed runtime progression.

Recommended progression policy:

- Early levels should come quickly enough to give several point-allocation choices in the first sessions.
- Mid levels should slow enough that gear/skill upgrades matter between point gains.
- Late levels should be long-tail prestige growth, not required for normal content completion.
- Enemy exp should target a stable "time to next battle point" band, then difficulty bonuses can shorten that time modestly.

## Difficulty Rewards

Difficulty should increase danger faster than rewards:

| Difficulty | Enemy Stat Mult | Exp Bonus | Drop Bonus |
| ---: | ---: | ---: | ---: |
| 1 | 1.00 | 0% | 0% |
| 2 | 1.18 | 8% | 5% |
| 3 | 1.38 | 18% | 10% |
| 4 | 1.62 | 30% | 16% |
| 5 | 1.90 | 45% | 24% |
| 6 | 2.22 | 65% | 34% |
| 7 | 2.58 | 90% | 46% |
| 8 | 2.98 | 120% | 60% |
| 9 | 3.42 | 155% | 78% |
| 10 | 3.90 | 200% | 100% |

This makes high difficulty attractive for strong builds, but not mandatory for efficient progression. A player who clears lower difficulty faster should remain competitive.

## Team Scaling

The default multiplayer policy is:

- Extra players add enemies or enemy pressure at a sublinear rate.
- Team reward efficiency should cap near 1.15x per unit time.
- Multiplayer should feel safer and more expressive because roles combine, not because it prints vastly more exp.
- Player plus kitties should be balanced as a normal small team, not as a second-class fallback.

`teamExtraEnemyScale=0.45` means each extra non-summon player should add less than a full solo workload. Existing `extraEnemyPerPlayer` world data should be checked against this.

## Equipment, Affixes, And Combo Effects

Equipment budget should split into:

- Base attributes.
- Enhance attributes.
- Basic attack identity.
- Battle hooks and effect mutations.
- Affix slots.
- Durability and cost constraints.

Combo/set effects should be treated as a soft budget, not free power:

- Healthy combo contribution: 8%-18% of the effective build budget.
- Above 18% should require a real constraint: trigger condition, cooldown, resource cost, encounter type, stack cap, or opportunity cost.
- Same-equipment affix combo should usually be narrower than cross-equipment combo.
- Combo should create "this is good here" rather than "always assemble this or lose".

## Combat Model Diagnostics

`combat-model-vue3` should flag:

- Enemy HP outside target rounds.
- Enemy exp more than 35% away from power-derived recommendation.
- Level-1 enemies with high power score.
- Skills with rear-line damage pressure.
- Skills with high effect score and near-zero MP/CD cost.
- Equipment whose identity budget is dominated by hooks/mutations without constraints.
- Affixes with mechanism budget but no body/equipment effective cap.
- Attribute model risks, especially speed, perception, and luck exceeding their intended roles.

## Data Migration Policy

Do not bulk rewrite existing enemies/equipment/skills immediately.

First produce audits:

1. Current value.
2. Recommended value.
3. Percent deviation.
4. Reason.
5. Suggested migration band.

Then rebalance by dungeon/level band, starting with early game enemies and battle exp, because those determine player point distribution and long-term progression.
