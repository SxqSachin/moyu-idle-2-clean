import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import { makeDefaultRow } from '../src/model/catalog.ts';

test('new conversion groups do not expose an extra ultra-rare reward branch', () => {
  const row = makeDefaultRow('alchemy_groups');
  assert.equal('ultraRareChanceBase' in row, false);
  assert.equal('ultraRareEntries' in row, false);
});

test('authored conversion upgrades never produce mythic items', () => {
  const groupPath = new URL('../../packages/game-data/source/json/alchemy_groups.json', import.meta.url);
  const itemPath = new URL('../../packages/game-data/source/json/items.json', import.meta.url);
  const groups = JSON.parse(readFileSync(groupPath, 'utf8')).alchemy_groups as Array<Record<string, unknown>>;
  const items = JSON.parse(readFileSync(itemPath, 'utf8')).items as Array<Record<string, unknown>>;
  const rarityByItemId = new Map(items.map((item) => [String(item.id || ''), String(item.rarity || '')]));

  for (const group of groups) {
    assert.equal('ultraRareChanceBase' in group, false, `${String(group.id)} has ultraRareChanceBase`);
    assert.equal('ultraRareEntries' in group, false, `${String(group.id)} has ultraRareEntries`);
    for (const entry of (group.upgradeEntries || []) as Array<Record<string, unknown>>) {
      const itemId = String(entry.itemId || '');
      assert.notEqual(rarityByItemId.get(itemId), 'mythic', `${String(group.id)} upgrades to ${itemId}`);
    }
  }
});
