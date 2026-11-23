import { apiGet, apiPost } from '@services/api'

export type FurnitureSelector = { actionIds?: string[]; mainTypes?: string[]; subTypes?: string[] }
export type FurnitureRule = { kind: 'owner_experience' | 'kitty_extra_loot' | 'action_efficiency_add'; domain: string; rateBps: number; selector?: FurnitureSelector }
export type FurnitureEffectGroup = { key: string; kind: FurnitureRule['kind']; rateBps: number; domains: string[]; selector: FurnitureSelector }
export type FurnitureDefinition = {
	service?: 'prepaid' | 'permanent'
	level?: number
  id: string; version: string; nameKey: string; iconId: string
  installCosts: Record<string, number>; refillCosts: Record<string, number>
  capacitySeconds: number; refillSeconds: number; rules: FurnitureRule[]
}
export type FurnitureQuoteIdentity = { token: string; expiresAt: string; remainingMs: number }
export type FurnitureUpgradeQuote = FurnitureQuoteIdentity & { target: FurnitureDefinition; upgradeCosts: Record<string,number>; supplementCosts: Record<string,number>; totalCosts: Record<string,number> }
export type FurnitureContribution = { experience?: Record<string,number>; items?: Record<string,number>; opportunities?: number; triggered?: number; empty?: number }
export type FurnitureReport = { token: string; through: string; rows: Record<string,FurnitureContribution>; names?: Record<string,string> }
export type FurnitureView = { definition: FurnitureDefinition; installed: boolean; refillAllowed?: boolean; remainingMs: number; enabledUntil?: string; maxLevel?: number; upgrade?: FurnitureUpgradeQuote; pending?: { definition: FurnitureDefinition; installed: boolean; remainingMs: number; enabledUntil?: string; effectiveAt: string } }
export type FurnitureProjection = { activationDelaySeconds?: number; summary?: boolean; revision: number; serverNow: string; available: boolean; items: FurnitureView[]; inventory: Record<string, number>; contributions?: Record<string,FurnitureContribution> }
export type FurnitureCommand = { requestId: string; furnitureId: string; version: string; revision: number; action: 'install' | 'refill' | 'enable' | 'pause' | 'upgrade'; count: number; upgradeQuote?: FurnitureQuoteIdentity }
export const getFurniture = (token: string, details = false) => apiGet<FurnitureProjection>(`/api/cattery/furniture${details ? '' : '?view=summary'}`, token)
export const commandFurniture = (token: string, command: FurnitureCommand) => apiPost<FurnitureProjection>('/api/cattery/furniture', token, command)
export const getFurnitureReport = (token: string) => apiGet<FurnitureReport>('/api/cattery/furniture/report', token)
export const acknowledgeFurnitureReport = (token: string, reportToken: string) => apiPost<FurnitureReport>('/api/cattery/furniture/report/ack', token, { token: reportToken })
export function furnitureAt(item: FurnitureView, now: number): FurnitureView {
  if (!item.pending || Date.parse(item.pending.effectiveAt) > now) return item
  return { ...item, ...item.pending, enabledUntil: item.pending.enabledUntil, pending: undefined, upgrade: undefined }
}
export function furnitureRemaining(item: FurnitureView, now: number) {
  item = furnitureAt(item, now)
  return Math.max(0, item.enabledUntil ? Date.parse(item.enabledUntil) - now : item.remainingMs)
}
export function furnitureRefillLimit(item: FurnitureView, inventory: Record<string, number>, now: number) {
  item = furnitureAt(item, now)
  if (item.pending) return 0
  if (item.definition.service === 'permanent' || item.refillAllowed === false) return 0
  const capacity = Math.floor((item.definition.capacitySeconds * 1000 - furnitureRemaining(item, now)) / (item.definition.refillSeconds * 1000))
  return Math.max(0, Math.min(capacity, ...Object.entries(item.definition.refillCosts).map(([id, n]) => Math.floor((inventory[id] || 0) / n))))
}

export const furnitureIsPermanent = (item: FurnitureView) => item.definition.service === 'permanent'
export const furnitureIsActive = (item: FurnitureView, now: number) => { item = furnitureAt(item, now); return item.installed && (furnitureIsPermanent(item) || (!!item.enabledUntil && furnitureRemaining(item, now) > 0)) }
export const furnitureEffectLabel = (kind: FurnitureRule['kind']) => ({ owner_experience: 'workLabel', kitty_extra_loot: 'lootLabel', action_efficiency_add: 'efficiencyLabel' })[kind]
export const furnitureEffectDescription = (kind: FurnitureRule['kind']) => ({ owner_experience: 'workEffect', kitty_extra_loot: 'lootEffect', action_efficiency_add: 'efficiencyEffect' })[kind]
export const furnitureEffectScope = (kind: FurnitureRule['kind']) => ({ owner_experience: 'workScope', kitty_extra_loot: 'lootScope', action_efficiency_add: 'efficiencyScope' })[kind]

export function furnitureWorkEfficiency(items: FurnitureView[], mainType: string, subType: string, actionID: string, now: number): number {
  if (!['collecting','mining','farming_animal','crafting','sewing','forging','alchemy','cooking'].includes(mainType)) return 0
  let total = 0
  for (const original of items) {
    const item = furnitureAt(original, now)
    if (!furnitureIsActive(item, now)) continue
    let best = 0
    for (const rule of item.definition.rules) {
      if (rule.kind !== 'action_efficiency_add' || rule.domain !== 'action_start') continue
      const s = rule.selector
      if (s?.mainTypes?.length && !s.mainTypes.includes(mainType) || s?.subTypes?.length && !s.subTypes.includes(subType) || s?.actionIds?.length && !s.actionIds.includes(actionID)) continue
      best = Math.max(best, rule.rateBps)
    }
    total += best
  }
  return total / 10000
}

export function furnitureEffectGroups(rules: FurnitureRule[]): FurnitureEffectGroup[] {
  const groups = new Map<string, FurnitureEffectGroup>()
  for (const rule of rules) {
    const selector = Object.fromEntries(['actionIds', 'mainTypes', 'subTypes'].map(key => [key, [...(rule.selector?.[key as keyof FurnitureSelector] || [])].sort()]))
    const key = JSON.stringify([rule.kind, rule.rateBps, selector])
    let group = groups.get(key)
    if (!group) { group = { key, kind: rule.kind, rateBps: rule.rateBps, domains: [], selector }; groups.set(key, group) }
    if (!group.domains.includes(rule.domain)) group.domains.push(rule.domain)
  }
  return [...groups.values()]
}

export function furniturePreviousRate(rules: FurnitureRule[], next: FurnitureEffectGroup): number {
  const scope = (e: FurnitureEffectGroup) => JSON.stringify([e.kind, e.selector, [...e.domains].sort()])
  return furnitureEffectGroups(rules).find(e => scope(e) === scope(next))?.rateBps || 0
}
