import { describe, expect, it } from 'vitest'
import { previewOperationEffectServings } from '@services/operation-effect-servings'
import { getFishFoodEffectDefinition, listFishFoodRecipes } from '@services/fish-food'
import type { OperationEffectDefinitionView, OperationEffectView } from '@services/operation-effects'

const now = Date.parse('2026-09-06T00:00:00Z')
const player = { targetKind: 'player' as const }
const definition = (form = 'soup') => getFishFoodEffectDefinition(listFishFoodRecipes().find(r => r.id === `field_honey_${form}`)!)!
function active(def: OperationEffectDefinitionView, overrides: Partial<OperationEffectView> = {}): OperationEffectView {
  return { grantId: 1, definitionId: def.id, rulesetVersion: def.rulesetVersion, domain: def.domain, kind: def.kind, nameKey: def.nameKey,
    descKey: def.descKey, params: def.params, stacking: def.stacking, lifetimeKind: def.lifetime.kind, targetKind: 'player',
    validFrom: new Date(now - 1000).toISOString(), validUntil: new Date(now + 900000).toISOString(), remainingCharges: 220, ...overrides }
}
describe('food reserve preview', () => {
  it('allows 96 standard soups or 100 standard grills without multiplying strength', () => {
    for (const form of ['soup', 'grill']) {
      const def = definition(form), quantity = form === 'soup' ? 96 : 100
      const p = previewOperationEffectServings(def, player, [], now, 200, quantity)
      expect(p.valid).toBe(true)
      expect(p.maxQuantity).toBe(quantity)
      expect(p.total).toBe(form === 'soup' ? 86400 : 22000)
      expect(previewOperationEffectServings(def, player, [], now, 200, quantity + 1).valid).toBe(false)
    }
  })
  it('accounts for current remaining time, stock and upgraded single servings', () => {
    const def = definition()
    expect(previewOperationEffectServings(def, player, [active(def)], now, 200, 95)).toMatchObject({ maxQuantity: 95, remaining: 900, total: 86400, valid: true })
    expect(previewOperationEffectServings(def, player, [], now, 3, 4)).toMatchObject({ maxQuantity: 3, valid: false })
    const upgraded = { ...def, lifetime: { kind: 'duration' as const, durationSeconds: 2000 } }
    expect(previewOperationEffectServings(upgraded, player, [], now, 200, 43)).toMatchObject({ maxQuantity: 43, total: 86000, valid: true })
  })
  it('isolates recipients but refuses overlapping exclusive strengths and modes', () => {
    const def = definition(), cat = { targetKind: 'kitty' as const, targetActorId: 7 }
    expect(previewOperationEffectServings(def, cat, [active(def)], now, 200, 96).valid).toBe(true)
    for (const existing of [active(def, { params: { chance: .02 } }), active(definition('grill')), active(def, { targetKind: 'player_and_kitties' })]) {
      expect(previewOperationEffectServings(def, player, [existing], now, 200, 1).conflict).toBe(true)
    }
    const other = active(def, { targetKind: 'kitty', targetActorId: 8 })
    expect(previewOperationEffectServings(def, cat, [other], now, 200, 96).valid).toBe(true)
  })
  it('ignores expired grants and fails closed for unavailable policy or invalid quantities', () => {
    const def = definition()
    expect(previewOperationEffectServings(def, player, [active(def, { validUntil: new Date(now).toISOString() })], now, 200, 96).valid).toBe(true)
    for (const q of [0, -1, 1.5, NaN, Infinity]) expect(previewOperationEffectServings(def, player, [], now, 200, q).valid).toBe(false)
    expect(previewOperationEffectServings({ ...def, stacking: undefined }, player, [], now, 200, 1).valid).toBe(false)
  })
})
