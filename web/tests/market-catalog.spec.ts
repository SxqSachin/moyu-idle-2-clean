import { describe, expect, it, vi } from 'vitest'
import profiles from '../../packages/game-data/source/json/market_profiles.json'
import species from '../../packages/game-data/source/json/fishing_species.json'
vi.mock('@services/item-definition', () => ({ getItemDefinition: (id: string) => ({ marketPolicy: id === 'fish_catch' ? { fixedPrice: { profileId: 'individual_specimen_v1' } } : undefined }) }))
vi.mock('@services/game-data-runtime', () => ({ getGameDataTable: (table: string) => table === 'market_profiles' ? profiles.market_profiles : table === 'fishing_species' ? species.fishing_species : [] }))
import { marketCatalogEntries, marketCatalogForInstance, marketCatalogName, matchesMarketCatalog } from '@services/market-catalog'

describe('DSL-driven market catalog', () => {
  it('expands every configured species without depending on inventory or listings', () => {
    const entries = marketCatalogEntries('fish_catch')
    expect(entries).toHaveLength(species.fishing_species.length)
    expect(new Set(entries.map(e => e.key)).size).toBe(entries.length)
    const entry = entries.find(e => e.catalogValues === '["bluegill"]')!
    expect(marketCatalogName(entry, 'zh-CN')).toBe('蓝鳃太阳鱼')
    expect(marketCatalogName(entry, 'en-US')).toBe('Bluegill')
    expect(entry.preview).toEqual({ fishData: { speciesId: 'bluegill' } })
  })
  it('keeps species separate but does not split catalog entries by individual measurements', () => {
    const specimen = { fishData: { speciesId: 'bluegill', lengthCm: 29.4 } }
    expect(matchesMarketCatalog('fish_catch', specimen, '["bluegill"]')).toBe(true)
    expect(matchesMarketCatalog('fish_catch', specimen, '["snakehead"]')).toBe(false)
    expect(marketCatalogForInstance('fish_catch', specimen)?.catalogValues).toBe('["bluegill"]')
    expect(marketCatalogForInstance('fish_catch', { fishData: { speciesId: 'bluegill', lengthCm: 12 } })?.key).toBe(marketCatalogForInstance('fish_catch', specimen)?.key)
  })
  it('preserves ordinary templates and fails closed for missing or malformed selection data', () => {
    expect(marketCatalogEntries('wood')).toEqual([])
    expect(matchesMarketCatalog('wood', {}, '')).toBe(true)
    expect(matchesMarketCatalog('wood', {}, '["bluegill"]')).toBe(false)
    expect(matchesMarketCatalog('fish_catch', {}, '["bluegill"]')).toBe(false)
    expect(matchesMarketCatalog('fish_catch', {}, 'null')).toBe(false)
  })
})
