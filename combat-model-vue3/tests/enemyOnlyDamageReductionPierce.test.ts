import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import { defaultDamageSpec, makeDefaultRow } from '../src/model/catalog.ts';
import {
  configuredEnemyOnlyDamageReductionPierceFields,
  removeEnemyOnlyDamageReductionPierceFields,
  validateEnemyOnlyDamageReductionPierceSkill,
  validateEnemyOnlyTargetPrioritySkill,
  validatePlayerOwnedDamageSpecs,
  validatePlayerOwnedTargetPriority
} from '../src/model/enemyOnlyDamageReductionPierce.ts';

test('new skills start player-facing while ordinary damage specs stay equipment-safe', () => {
  assert.equal(makeDefaultRow('skills').enemyOnly, false);
  assert.deepEqual(configuredEnemyOnlyDamageReductionPierceFields(defaultDamageSpec()), []);
});

test('editor validation requires monster-only ownership and known status gates', () => {
  const effect = {
    kind: 'damage',
    damageSpec: {
      enemyOnlyDamageReductionPierce: 0.4,
      enemyOnlyDamageReductionPierceTargetStatuses: ['pressure_mark']
    }
  };
  assert.deepEqual(validateEnemyOnlyDamageReductionPierceSkill({ id: 'pressure', enemyOnly: true, effects: [effect] }, ['pressure_mark']), []);
  assert.match(
    validateEnemyOnlyDamageReductionPierceSkill({ id: 'pressure', enemyOnly: false, effects: [effect] }, ['pressure_mark']).join('\n'),
    /仅供怪物使用/
  );
  assert.match(
    validateEnemyOnlyDamageReductionPierceSkill({ id: 'pressure', enemyOnly: true, effects: [effect] }, []).join('\n'),
    /未定义 pressure_mark/
  );
});

test('editor removes both fields atomically and rejects them on player-owned attacks', () => {
  const spec = {
    damageType: 'physical',
    enemyOnlyDamageReductionPierce: 0.25,
    enemyOnlyDamageReductionPierceTargetStatuses: ['pressure_mark']
  };
  assert.deepEqual(removeEnemyOnlyDamageReductionPierceFields(spec), { damageType: 'physical' });
  assert.match(
    validatePlayerOwnedDamageSpecs([{ kind: 'damage', damageSpec: spec }], '装备 test: basicAttackEffects').join('\n'),
    /仅允许怪物专属技能使用/
  );
});

test('target-priority bypass is an explicit monster-only targeting option', () => {
  const effect = { kind: 'damage', targeting: { targetPositions: ['vanguard'], ignoreTargetPriority: true } };
  assert.deepEqual(validateEnemyOnlyTargetPrioritySkill({ id: 'row_lock', enemyOnly: true, effects: [effect] }), []);
  assert.match(
    validateEnemyOnlyTargetPrioritySkill({ id: 'row_lock', enemyOnly: false, effects: [effect] }).join('\n'),
    /仅供怪物使用/
  );
  assert.match(validatePlayerOwnedTargetPriority([effect], '装备 test: basicAttackEffects').join('\n'), /仅允许怪物专属技能使用/);
});

test('current authored pierce skills satisfy the editor contract', () => {
  const skillsPath = new URL('../../packages/game-data/source/json/skills.json', import.meta.url);
  const buffsPath = new URL('../../packages/game-data/source/json/buff.json', import.meta.url);
  const skills = JSON.parse(readFileSync(skillsPath, 'utf8')).skills as Array<Record<string, unknown>>;
  const buffs = JSON.parse(readFileSync(buffsPath, 'utf8')).buff as Array<Record<string, unknown>>;
  const buffIds = buffs.map((row) => String(row.id || '')).filter(Boolean);
  assert.deepEqual(skills.flatMap((skill) => [
    ...validateEnemyOnlyDamageReductionPierceSkill(skill, buffIds),
    ...validateEnemyOnlyTargetPrioritySkill(skill)
  ]), []);
});
