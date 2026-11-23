import assert from 'node:assert/strict';
import test from 'node:test';
import {
  DEFAULT_POWERED_EXPONENTIAL_DECAY,
  approximateHitDamageEquivalentCap,
  hitDamageEquivalentSum,
  hitDamageMultiplier,
  normalizeHitDamageScaling,
  validateHitDamageScaling
} from '../src/model/hitDamageScaling.ts';

const scaling = DEFAULT_POWERED_EXPONENTIAL_DECAY;

test('powered exponential decay matches the canonical golden vectors', () => {
  assert.equal(hitDamageMultiplier(scaling, 1), 1);
  assert.equal(hitDamageMultiplier(scaling, 2), 0.998);
  assert.ok(Math.abs(hitDamageMultiplier(scaling, 12) - 0.6942975551370444) < 1e-12);
  assert.ok(Math.abs(hitDamageMultiplier(scaling, 36) - 0.011095759646342131) < 1e-12);
  assert.ok(Math.abs(hitDamageEquivalentSum(scaling, 36) - 15.969712553678615) < 1e-12);
  const cap = approximateHitDamageEquivalentCap(scaling);
  assert.equal(cap?.converged, true);
  assert.ok(Math.abs((cap?.equivalentHits || 0) - 16.00049329726089) < 1e-9);
});

test('normalization accepts only the canonical kind and safe parameter ranges', () => {
  assert.deepEqual(normalizeHitDamageScaling(scaling), scaling);
  assert.equal(normalizeHitDamageScaling({ ...scaling, kind: 'linear' }), null);
  assert.equal(normalizeHitDamageScaling({ ...scaling, kind: ' powered_exponential_decay ' }), null);
  assert.equal(normalizeHitDamageScaling({ ...scaling, secondHitMultiplier: '0.998' }), null);
  assert.equal(normalizeHitDamageScaling({ ...scaling, curvePower: '2.1708' }), null);
  assert.equal(normalizeHitDamageScaling({ ...scaling, floor: 0.01 }), null);
  assert.equal(normalizeHitDamageScaling({ ...scaling, secondHitMultiplier: 1 }), null);
  assert.equal(normalizeHitDamageScaling({ ...scaling, secondHitMultiplier: 0 }), null);
  assert.equal(normalizeHitDamageScaling({ ...scaling, curvePower: 0.99 }), null);
  assert.equal(normalizeHitDamageScaling({ ...scaling, curvePower: 3.01 }), null);
  assert.deepEqual(validateHitDamageScaling(scaling, 'damageSpec'), []);
  assert.match(validateHitDamageScaling({ ...scaling, floor: 0.01 }, 'damageSpec').join('\n'), /不支持的字段 floor/);
  assert.match(validateHitDamageScaling({ ...scaling, secondHitMultiplier: 1 }, 'damageSpec').join('\n'), /严格大于 0 且小于 1/);
  assert.match(validateHitDamageScaling({ ...scaling, curvePower: 3.01 }, 'damageSpec').join('\n'), /1 到 3/);
});

test('equivalent-hit summaries use the runtime 64-hit ceiling', () => {
  assert.equal(hitDamageEquivalentSum(scaling, 100), hitDamageEquivalentSum(scaling, 64));
});
