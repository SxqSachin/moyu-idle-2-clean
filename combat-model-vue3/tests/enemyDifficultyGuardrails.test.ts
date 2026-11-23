import assert from 'node:assert/strict';
import test from 'node:test';
import {
  applyEnemyDifficultyGuardrails,
  buildEnemyDifficultyGuardrailPatch,
  parseEnemyDifficultyAuthoringGuardrails,
  projectEnemyDefensiveStats
} from '../src/model/enemyDifficultyGuardrails.ts';

test('guardrail parsing preserves safe defaults and clamps retain ratios', () => {
  const parsed = parseEnemyDifficultyAuthoringGuardrails({
    defenseRetainRatio: 4,
    critDamageResistRetainRatio: -2,
    defenseBaseCapsByEnemyType: { boss: 75, elite: -1 },
    critDamageResistBaseFloorsByEnemyType: { boss: 3, elite: -1 },
    critDamageResistBaseCapsByEnemyType: { boss: 0.13 }
  });

  assert.equal(parsed.defenseRetainRatio, 1);
  assert.equal(parsed.critDamageResistRetainRatio, 0);
  assert.equal(parsed.minEnemyLevel, 40);
  assert.equal(parsed.defenseBaseCapsByEnemyType.boss, 75);
  assert.equal(parsed.defenseBaseCapsByEnemyType.elite, 25);
  assert.equal(parsed.critDamageResistBaseFloorsByEnemyType.boss, 1);
  assert.equal(parsed.critDamageResistBaseFloorsByEnemyType.elite, 0);
  assert.equal(parsed.critDamageResistBaseCapsByEnemyType.boss, 1);
});

test('boss guardrails author explicit corrections without changing runtime formulas', () => {
  const guardrails = parseEnemyDifficultyAuthoringGuardrails(null);
  const enemy = {
    type: 'boss',
    level: 290,
    strength: 357,
    perception: 459,
    stamina: 552.5,
    physics_defence: 50,
    magical_defence: 75,
    crit_damage_resist: 0
  };

  applyEnemyDifficultyGuardrails(enemy, guardrails);
  const projected = projectEnemyDefensiveStats(enemy);

  assert.ok(Math.abs(projected.physicsDefence - 30) < 0.000001);
  assert.ok(Math.abs(projected.magicalDefence - 30) < 0.000001);
  assert.ok(Math.abs(projected.critDamageResist - 0.133333) < 0.000001);
  assert.deepEqual(buildEnemyDifficultyGuardrailPatch(enemy, guardrails), {
    physics_defence: enemy.physics_defence,
    magical_defence: enemy.magical_defence,
    crit_damage_resist: enemy.crit_damage_resist
  });
});

test('role variation survives below the authored caps', () => {
  const guardrails = parseEnemyDifficultyAuthoringGuardrails(null);
  const enemy = {
    type: 'normal',
    level: 180,
    strength: 40,
    perception: 70,
    stamina: 60
  };
  const natural = projectEnemyDefensiveStats(enemy);
  applyEnemyDifficultyGuardrails(enemy, guardrails);
  const projected = projectEnemyDefensiveStats(enemy);

  assert.ok(Math.abs(projected.physicsDefence - natural.physicsDefence * 0.12) < 0.000001);
  assert.ok(Math.abs(projected.magicalDefence - natural.magicalDefence * 0.12) < 0.000001);
  assert.ok(Math.abs(projected.critDamageResist - natural.critDamageResist * 0.25) < 0.000001);
});

test('low-level enemies remain on the original progression', () => {
  const guardrails = parseEnemyDifficultyAuthoringGuardrails(null);
  const enemy = {
    type: 'normal',
    level: 20,
    strength: 40,
    perception: 70,
    stamina: 60
  };

  assert.deepEqual(buildEnemyDifficultyGuardrailPatch(enemy, guardrails), {});
});

test('bosses keep modest base crit resistance even below the normal guardrail level', () => {
  const guardrails = parseEnemyDifficultyAuthoringGuardrails(null);
  const boss = {
    type: 'boss',
    level: 20,
    strength: 40,
    perception: 30,
    stamina: 50
  };

  applyEnemyDifficultyGuardrails(boss, guardrails);
  const projected = projectEnemyDefensiveStats(boss);

  assert.ok(Math.abs(projected.physicsDefence - 3.42) < 0.000001);
  assert.ok(Math.abs(projected.magicalDefence - 3.9) < 0.000001);
  assert.ok(Math.abs(projected.critDamageResist - 0.12) < 0.000001);
});

test('unconfigured enemy types are not rewritten', () => {
  const guardrails = parseEnemyDifficultyAuthoringGuardrails(null);
  const summon = {
    type: 'summon',
    level: 130,
    strength: 100,
    perception: 100,
    stamina: 100
  };

  assert.deepEqual(buildEnemyDifficultyGuardrailPatch(summon, guardrails), {});
});
