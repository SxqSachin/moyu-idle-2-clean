import assert from 'node:assert/strict';
import test from 'node:test';
import { isNotFoundError, parsePresentTableText, tableRowsFromParsed } from '../src/services/sourceTableState.ts';

test('an explicit empty source array remains present and authoritative', () => {
  assert.deepEqual(parsePresentTableText('[]', 'specialization_trees', 'source/json/specialization_trees.json'), {
    present: true,
    rows: []
  });
  assert.deepEqual(parsePresentTableText('{"specialization_trees":[]}', 'specialization_trees', 'source/json/specialization_trees.json'), {
    present: true,
    rows: []
  });
});

test('malformed JSON retains its source path and cannot masquerade as a missing file', () => {
  assert.throws(
    () => parsePresentTableText('{bad json', 'specialization_trees', 'source/json/specialization_trees.json'),
    /source\/json\/specialization_trees\.json:/
  );
  assert.equal(isNotFoundError(new SyntaxError('bad JSON')), false);
});

test('wrong top-level shapes fail closed while only NotFoundError is treated as absent', () => {
  assert.throws(
    () => tableRowsFromParsed({}, 'specialization_trees', 'source/json/specialization_trees.json'),
    /JSON 顶层必须是数组/
  );
  assert.equal(isNotFoundError({ name: 'NotFoundError' }), true);
  assert.equal(isNotFoundError({ name: 'NotAllowedError' }), false);
});
