import { describe, expect, it } from 'vitest'
import { operationEffectFoodTargets } from '@services/operation-effect-food-targets'
import type { OperationEffectDefinitionView } from '@services/operation-effects'
import { getFishFoodEffectDefinition, listFishFoodRecipes } from '@services/fish-food'

const cats = [{ id: 7, name: 'Momo' }, { id: 8, name: ' ' }]
const t = (key: string, params?: Record<string, string | number>) => params?.name ? `${key}:${params.name}` : key
describe('food recipient options', () => {
  it('offers only individual actors for every soup, including stale shared definitions', () => {
    for (const recipe of listFishFoodRecipes()) {
      const def = getFishFoodEffectDefinition(recipe)!
      if (def.lifetime.kind !== 'duration') continue
      expect(operationEffectFoodTargets(def, cats, t).map(row => row.key)).toEqual(['player', 'kitty:7', 'kitty:8'])
      const stale: OperationEffectDefinitionView = { ...def, allowedTargets: ['player', 'kitty', 'all_kitties', 'player_and_kitties'] }
      expect(operationEffectFoodTargets(stale, cats, t).map(row => row.key)).toEqual(['player', 'kitty:7', 'kitty:8'])
    }
  })
  it('keeps shared uses for grilled fish and the player-only enchantment exception', () => {
    for (const recipe of listFishFoodRecipes()) {
      const def = getFishFoodEffectDefinition(recipe)!
      if (def.lifetime.kind !== 'charges') continue
      const options = operationEffectFoodTargets(def, cats, t)
      expect(options.map(row => row.key)).toEqual(def.domain === 'enchantment' ? ['player'] : ['player', 'kitty:7', 'kitty:8', 'player_and_kitties'])
    }
  })
  it('preserves exact kitty IDs and handles missing or malformed recipients', () => {
    const def = { allowedTargets: ['player', 'kitty'] as const, lifetime: { kind: 'duration' as const, durationSeconds: 600 } }
    const definition = { ...def, allowedTargets: [...def.allowedTargets] }
    expect(operationEffectFoodTargets(definition, [], t).map(row => row.key)).toEqual(['player'])
    const options = operationEffectFoodTargets(definition, [...cats, { id: 0 }, { id: 1.5 }, { id: NaN }], t)
    expect(options[1]).toMatchObject({ targetKind: 'kitty', targetActorId: 7 })
    expect(options[2].label).toContain('#8')
    expect(options).toHaveLength(3)
    expect(operationEffectFoodTargets(null, cats, t)).toEqual([])
  })
})
