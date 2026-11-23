import { getGameDataTable } from '@services/game-data-runtime'
import { getItemDefinition } from '@services/item-definition'
import type { MarketPreview } from '@services/instance-market'

type CatalogKey = { source: string; path: string; compare: string }
type CatalogProfile = {
  id: string
  disabled?: boolean
  catalogKeys: CatalogKey[]
  catalogSource?: { table: string; namesPath: string; valuePaths: string[] }
}
export type MarketCatalogEntry = {
  key: string
  itemId: string
  catalogValues: string
  names: Record<string, string>
  preview: MarketPreview
}

function parts(path: string): string[] {
  if (!path.startsWith('/') || path.length > 256) return []
  const keys = path.slice(1).split('/').map(key => key.replace(/~1/g, '/').replace(/~0/g, '~'))
  return keys.some(key => ['__proto__', 'prototype', 'constructor'].includes(key)) ? [] : keys
}
function read(value: unknown, path: string): unknown {
  const keys = parts(path)
  if (!keys.length) return undefined
  for (const key of keys) {
    if (!value || typeof value !== 'object' || !Object.hasOwn(value, key)) return undefined
    value = (value as Record<string, unknown>)[key]
  }
  return value
}
function profileFor(itemId: string) {
  const id = getItemDefinition(itemId)?.marketPolicy?.fixedPrice.profileId
  return getGameDataTable<CatalogProfile>('market_profiles').find(p => p.id === id && !p.disabled)
}

// Catalog entries exist independently of listings, so the first seller can
// enter an empty catalog and list an instance without creating a new template.
export function marketCatalogEntries(itemId: string): MarketCatalogEntry[] {
  const profile = profileFor(itemId)
  const source = profile?.catalogSource
  if (!source || source.valuePaths.length !== profile.catalogKeys.length) return []
  return getGameDataTable<Record<string, unknown>>(source.table).flatMap(row => {
    if (row.disabled) return []
    const values = source.valuePaths.map(path => read(row, path))
    const names = read(row, source.namesPath)
    if (values.some(v => typeof v !== 'string' || !v) || !names || typeof names !== 'object' || Array.isArray(names)) return []
    const preview: Record<string, unknown> = {}
    for (const [index, key] of profile.catalogKeys.entries()) {
      const keys = parts(key.path)
      if (key.source !== 'instance' || key.compare !== 'exact' || !keys.length) return []
      let node = preview
      for (const part of keys.slice(0, -1)) {
        node[part] ??= {}
        if (!node[part] || typeof node[part] !== 'object') return []
        node = node[part] as Record<string, unknown>
      }
      node[keys[keys.length - 1]] = values[index]
    }
    const catalogValues = JSON.stringify(values)
    return [{ key: JSON.stringify([itemId, values]), itemId, catalogValues, names: names as Record<string, string>, preview: preview as MarketPreview }]
  })
}
export function marketCatalogName(entry: MarketCatalogEntry, locale: string): string {
  return entry.names[locale] || entry.names['en-US'] || entry.names['zh-CN'] || Object.values(entry.names).find(Boolean) || ''
}
export function matchesMarketCatalog(itemId: string, instance: unknown, catalogValues: string): boolean {
  if (!catalogValues) return true
  const profile = profileFor(itemId)
  if (!profile?.catalogSource) return false
  return JSON.stringify(profile.catalogKeys.map(key => read(instance, key.path))) === catalogValues
}
export function marketCatalogForInstance(itemId: string, instance: unknown): MarketCatalogEntry | undefined {
  const profile = profileFor(itemId)
  if (!profile?.catalogSource) return undefined
  const values = JSON.stringify(profile.catalogKeys.map(key => read(instance, key.path)))
  return marketCatalogEntries(itemId).find(entry => entry.catalogValues === values)
}
