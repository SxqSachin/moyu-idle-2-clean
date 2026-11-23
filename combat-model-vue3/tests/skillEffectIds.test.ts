import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import {
  createStableSkillEffectId,
  stableSkillEffectId,
  validateGlobalSkillEffectStableIds,
  validateSkillEffectStableIds
} from '../src/model/skillEffectIds.ts';

test('every skill effect requires a stable ID while the legacy effectId alias remains readable', () => {
  assert.match(validateSkillEffectStableIds('arcane_missile', [{ kind: 'damage' }]).join('\n'), /id 必须是非空字符串/);
  assert.deepEqual(validateSkillEffectStableIds('arcane_missile', [{ effectId: 'arcane_missile:damage', kind: 'damage' }]), []);
});

test('current skill source has a valid stable ID for every top-level effect', () => {
  const path = new URL('../../packages/game-data/source/json/skills.json', import.meta.url);
  const skills = JSON.parse(readFileSync(path, 'utf8')).skills as Array<Record<string, unknown>>;
  const effects = skills.flatMap((skill) => Array.isArray(skill.effects) ? skill.effects : []);
  assert.ok(effects.length > 0);
  assert.equal(effects.filter((effect) => stableSkillEffectId(effect)).length, effects.length);
  assert.deepEqual(skills.flatMap((skill) => validateSkillEffectStableIds(skill.id, Array.isArray(skill.effects) ? skill.effects : [])), []);
});

test('current equipment basic attacks also persist stable effect IDs', () => {
  const path = new URL('../../packages/game-data/source/json/equipment_templates.json', import.meta.url);
  const templates = JSON.parse(readFileSync(path, 'utf8')).equipment_templates as Array<Record<string, unknown>>;
  const rows = templates.filter((template) => Array.isArray(template.basicAttackEffects));
  const effects = rows.flatMap((template) => template.basicAttackEffects as unknown[]);
  assert.ok(effects.length > 0);
  assert.equal(effects.filter((effect) => stableSkillEffectId(effect)).length, effects.length);
  assert.deepEqual(rows.flatMap((template) => validateSkillEffectStableIds(`equipment:${template.id}`, template.basicAttackEffects as unknown[])), []);
});

test('new stable effect IDs are semantic and remain independent of array position', () => {
  const effects = [
    { id: 'arcane_missile_damage', kind: 'damage' },
    { id: 'arcane_missile_damage_2', kind: 'damage' }
  ];
  assert.equal(createStableSkillEffectId('arcane-missile', 'damage', effects), 'arcane_missile_damage_3');
  assert.equal(createStableSkillEffectId('arcane-missile', 'status', [...effects].reverse()), 'arcane_missile_status');
  assert.equal(stableSkillEffectId({ effectId: 'legacy:damage' }), 'legacy:damage');
});

test('configured stable IDs reject blanks, conflicts, uppercase, invalid characters, and case-folded duplicates', () => {
  const issues = validateSkillEffectStableIds('arcane_missile', [
    { id: '', kind: 'damage' },
    { id: 'arcane_missile.damage', effectId: 'legacy_damage', kind: 'damage' },
    { id: 'Uppercase_Damage', kind: 'damage' },
    { id: 'bad id', kind: 'status' },
    { id: 'duplicate', kind: 'damage' },
    { effectId: 'DUPLICATE', kind: 'heal' }
  ]).join('\n');
  assert.match(issues, /id 必须是非空字符串/);
  assert.match(issues, /id 与旧 effectId 不一致/);
  assert.match(issues, /只能包含小写字母/);
  assert.match(issues, /稳定 ID 重复 DUPLICATE/);
});

test('stable effect IDs remain globally unique across skills', () => {
  assert.match(validateGlobalSkillEffectStableIds([
    { id: 'first', effects: [{ id: 'shared_damage', kind: 'damage' }] },
    { id: 'second', effects: [{ id: 'SHARED_DAMAGE', kind: 'damage' }] }
  ]).join('\n'), /全局重复 SHARED_DAMAGE/);
  assert.deepEqual(validateGlobalSkillEffectStableIds([
    { id: 'first', effects: [{ id: 'first_damage', kind: 'damage' }] },
    { id: 'second', effects: [{ id: 'second_damage', kind: 'damage' }] }
  ]), []);
});
