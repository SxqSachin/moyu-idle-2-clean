import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { APIClientError } from '../src/services/api'
import { useCatteryFurnitureStore } from '../src/stores/cattery-furniture'
import { furnitureAt, furnitureRemaining, furnitureRefillLimit, furnitureEffectGroups, furniturePreviousRate, furnitureIsActive, furnitureWorkEfficiency, type FurnitureView, type FurnitureProjection, type FurnitureCommand } from '../src/services/cattery-furniture'

const api = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn() }))
vi.mock('../src/services/cattery-furniture', async (original) => ({ ...await original<object>(), getFurniture: api.get, commandFurniture: api.post }))
const at = Date.parse('2026-09-06T00:00:00Z')
function item(): FurnitureView { return { installed: true, remainingMs: 36000000, definition: { id: 'desk', version: 'v1', iconId: 'item.study_desk', nameKey: 'name', capacitySeconds: 120000, refillSeconds: 36000, installCosts: { wood: 10 }, refillCosts: { wood: 2 }, rules: [{ kind: 'owner_experience', domain: 'life_action', rateBps: 1500 }] } } }
const projection = (revision = 1): FurnitureProjection => ({ revision, serverNow: new Date(at).toISOString(), available: true, inventory: { wood: 100 }, items: [item()] })
const command: FurnitureCommand = { requestId: 'refill-00001', furnitureId: 'desk', version: 'v1', revision: 1, action: 'refill', count: 1 }

describe('furniture lifecycle projection', () => {
  beforeEach(() => { setActivePinia(createPinia()); api.get.mockReset(); api.post.mockReset() })
  it('keeps old paid service until activation and then uses the scheduled paused state', () => {
    const desk = item(); desk.enabledUntil = new Date(at + 36000000).toISOString()
    desk.pending = { definition: {...desk.definition, level: 2}, installed: true, remainingMs: 35940000, effectiveAt: new Date(at + 60000).toISOString() }
    expect(furnitureIsActive(desk, at + 59999)).toBe(true)
    expect(furnitureRemaining(desk, at + 59999)).toBe(35940001)
    expect(furnitureRefillLimit(desk, {wood:1000}, at)).toBe(0)
    expect(furnitureIsActive(desk, at + 60000)).toBe(false)
    expect(furnitureRemaining(desk, at + 1e9)).toBe(35940000)
    expect(furnitureAt(desk, at + 60000).definition.level).toBe(2)
  })
  it('applies permanent training at the exact deadline, including action previews', () => {
    const bench = item(); bench.definition.service = 'permanent'; bench.installed = false
    bench.definition.rules = [{kind:'action_efficiency_add',domain:'action_start',rateBps:2000}]
    bench.pending = {definition:{...bench.definition,level:1},installed:true,remainingMs:0,effectiveAt:new Date(at+60000).toISOString()}
    expect(furnitureWorkEfficiency([bench], 'collecting','','logging',at+59999)).toBe(0)
    expect(furnitureWorkEfficiency([bench], 'collecting','','logging',at+60000)).toBe(.2)
    expect(furnitureWorkEfficiency([bench], 'fishing','','logging',at+60000)).toBe(0)
  })
  it('reuses local summary reads but loads fresh quotes on demand', async () => {
    const store=useCatteryFurnitureStore()
    api.get.mockResolvedValue({...projection(),summary:true,inventory:{}})
    await store.refresh('a'); await store.refresh('a'); await store.refresh('a')
    expect(api.get).toHaveBeenCalledTimes(1)
    expect(api.get).toHaveBeenLastCalledWith('a',false)
    api.get.mockResolvedValue(projection())
    await store.refresh('a',{details:true,force:true})
    expect(api.get).toHaveBeenCalledTimes(2)
    expect(api.get).toHaveBeenLastCalledWith('a',true)
    await store.refresh('a'); expect(api.get).toHaveBeenCalledTimes(2)
    await store.refresh('a',{force:true}); expect(api.get).toHaveBeenCalledTimes(3)
  })
  it('upgrades an in-flight summary request to details without crossing accounts', async () => {
    const store=useCatteryFurnitureStore()
    let resolve!:(p:FurnitureProjection)=>void
    api.get.mockReturnValueOnce(new Promise(r=>{resolve=r})).mockResolvedValue(projection())
    const summary=store.refresh('a'), detail=store.refresh('a',{force:true,details:true})
    resolve({...projection(),summary:true}); await summary; await detail
    expect(api.get).toHaveBeenCalledTimes(2)
    expect(store.projection?.summary).not.toBe(true)
    api.get.mockReturnValueOnce(new Promise(r=>{resolve=r}))
    const old=store.refresh('a',{force:true}), oldDetail=store.refresh('a',{force:true,details:true})
    store.bind('b');resolve({...projection(),summary:true}); await old;await oldDetail
    expect(store.projection).toBeNull(); expect(api.get).toHaveBeenCalledTimes(3)
  })
  it('treats installed permanent service as active without fake paid time', () => {
    const bench = item(); bench.definition.service = 'permanent'; bench.definition.capacitySeconds = 0; bench.definition.refillSeconds = 0; bench.definition.refillCosts = {}; bench.remainingMs = 0
    expect(furnitureIsActive(bench, at + 1e12)).toBe(true)
    expect(furnitureRefillLimit(bench, {wood:100000}, at)).toBe(0)
    bench.installed = false
    expect(furnitureIsActive(bench, at)).toBe(false)
  })
  it('matches training scopes, de-duplicates overlapping rules, and adds independent sources', () => {
    const bench = item(); bench.definition.service = 'permanent'
    bench.definition.rules = [{kind:'action_efficiency_add',domain:'action_start',rateBps:14000,selector:{mainTypes:['collecting'],actionIds:['logging']}}]
    bench.definition.rules.push({...bench.definition.rules[0]})
    expect(furnitureWorkEfficiency([bench], 'collecting','','logging',at)).toBe(1.4)
    expect(furnitureWorkEfficiency([bench], 'collecting','','other',at)).toBe(0)
    expect(furnitureWorkEfficiency([bench], 'fishing','','logging',at)).toBe(0)
    expect(furnitureWorkEfficiency([bench], 'equipment','','logging',at)).toBe(0)
    const another = structuredClone(bench); another.definition.id = 'other'; another.definition.rules[0].rateBps=2000; another.definition.rules.splice(1)
    expect(furnitureWorkEfficiency([bench,another], 'collecting','','logging',at)).toBe(1.6)
  })
  it('presents combined DSL effects without mixing rates or selector scopes', () => {
    const groups = furnitureEffectGroups([
      { kind: 'owner_experience', domain: 'life_action', rateBps: 1500 },
      { kind: 'owner_experience', domain: 'fishing_cast', rateBps: 1500 },
      { kind: 'kitty_extra_loot', domain: 'battle_reward', rateBps: 2000 },
      { kind: 'owner_experience', domain: 'life_action', rateBps: 3000, selector: { actionIds: ['logging'] } }
    ])
    expect(groups).toHaveLength(3)
    expect(groups[0].domains).toEqual(['life_action', 'fishing_cast'])
    expect(groups.map(x => x.rateBps)).toEqual([1500, 2000, 3000])
    expect(groups[2].selector.actionIds).toEqual(['logging'])
    expect(furniturePreviousRate([{kind:'kitty_extra_loot',domain:'battle_reward',rateBps:500},...item().definition.rules],groups[1])).toBe(500)
    expect(furniturePreviousRate(item().definition.rules,groups[2])).toBe(0)
  })
  it('preserves paused service and expires enabled service at its exact boundary', () => {
    const paused = item(); expect(furnitureRemaining(paused, at + 1e12)).toBe(36000000)
    const active = { ...paused, enabledUntil: new Date(at + 1000).toISOString() }
    expect(furnitureRemaining(active, at)).toBe(1000)
    expect(furnitureRemaining(active, at + 1000)).toBe(0)
    expect(furnitureRemaining(active, at + 10000)).toBe(0)
    expect(furnitureRefillLimit(paused, { wood: 100 }, at)).toBe(2)
    expect(furnitureRefillLimit(paused, { wood: 1 }, at)).toBe(0)
  })
  it('retains an uncertain command identity and will not submit a new debit', async () => {
    const store = useCatteryFurnitureStore(); store.bind('a')
    api.post.mockRejectedValueOnce(new Error('connection lost'))
    expect(await store.submit(command)).toBe(false)
    expect(store.pending).toEqual(command)
    expect(await store.submit({ ...command, requestId: 'different-id' })).toBe(false)
    expect(api.post).toHaveBeenCalledTimes(1)
    api.post.mockResolvedValueOnce(projection(2))
    expect(await store.submit(command)).toBe(true)
    expect(store.pending).toBeNull()
    expect(store.projection?.revision).toBe(2)
  })
  it('rejects stale reads after a confirmed write', async () => {
    const store = useCatteryFurnitureStore(); store.bind('a')
    let resolve!: (p: FurnitureProjection) => void
    api.get.mockReturnValueOnce(new Promise(r => { resolve = r }))
    const reading = store.refresh('a')
    api.post.mockResolvedValueOnce(projection(3)); expect(await store.submit(command)).toBe(true)
    resolve(projection(1)); await reading
    expect(store.projection?.revision).toBe(3)
  })
  it('does not leak a response or a pending command across accounts', async () => {
    const store = useCatteryFurnitureStore(); store.bind('a')
    let resolve!: (p: FurnitureProjection) => void
    api.post.mockReturnValueOnce(new Promise(r => { resolve = r }))
    const writing = store.submit(command); store.bind('b'); resolve(projection(2))
    expect(await writing).toBe(false); expect(store.projection).toBeNull(); expect(store.pending).toBeNull(); expect(store.busy).toBe(false)
  })
  it('does not automatically retry a definite conflict', async () => {
    const store = useCatteryFurnitureStore(); store.bind('a')
    api.post.mockRejectedValueOnce(new APIClientError('conflict', { status: 409, code: 'FURNITURE_CONFLICT' }))
    expect(await store.submit(command)).toBe(false)
    expect(store.pending).toBeNull(); expect(store.errorCode).toBe('FURNITURE_CONFLICT'); expect(api.post).toHaveBeenCalledTimes(1)
  })
  it('retries the identical upgrade quote after an uncertain response, without repricing', async () => {
    const store=useCatteryFurnitureStore();store.bind('a')
    const upgrade: FurnitureCommand={...command,action:'upgrade',count:0,upgradeQuote:{token:'bound-quote',expiresAt:new Date(at).toISOString(),remainingMs:120000000}}
    api.post.mockRejectedValueOnce(new Error('lost response'))
    expect(await store.submit(upgrade)).toBe(false)
    api.get.mockResolvedValueOnce(projection(2));await store.refresh('a')
    expect(store.pending).toEqual(upgrade)
    api.post.mockResolvedValueOnce(projection(2));expect(await store.submit(store.pending!)).toBe(true)
    expect(api.post).toHaveBeenLastCalledWith('a',upgrade)
  })
})

it('registers complete furniture messages under game in every supported locale', async () => {
  const locales = await Promise.all([
    import('../src/locales/game/zh-CN'), import('../src/locales/game/en-US'), import('../src/locales/game/zh-TW'),
    import('../src/locales/game/ja'), import('../src/locales/game/ko'), import('../src/locales/game/ru')
  ])
  const keys = Object.keys(locales[0].default.game.furniture).sort()
  for (const locale of locales) {
    expect(Object.keys(locale.default.game.furniture).sort()).toEqual(keys)
    expect(locale.default.game.furniture.addTime).toContain('{minutes}')
    expect(locale.default.game.furniture.errors.FURNITURE_UNKNOWN).toBeTruthy()
  }
})
