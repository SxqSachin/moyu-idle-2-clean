import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"
import { createApp, h, nextTick, ref, type App } from "vue"
import { createPinia, setActivePinia } from "pinia"
import { createI18n } from "vue-i18n"
import OperationEffectNotice from "@components/game/operation-effect/OperationEffectNotice.vue"
import { operationEffectMatchesQuery, operationEffectEligibility, type OperationEffectContext } from "@services/operation-effect-context"
import type { OperationEffectView } from "@services/operation-effects"
import { useOperationEffects } from "@/composables/game/useOperationEffects"
import { useOperationEffectsStore } from "@stores/operation-effects"
import zhCN from "@/locales/game/zh-CN"

const at = "2026-09-05T00:00:00.000Z"
function effect(overrides: Partial<OperationEffectView> = {}): OperationEffectView {
  return {grantId:1,definitionId:"capacity",rulesetVersion:"test",domain:"alchemy",kind:"alchemy.infusion_bulk_capacity_floor_tradeoff",nameKey:"game.fishCuisine.names.mossfire_dual_fish_grill",descKey:"",targetKind:"player",lifetimeKind:"charges",validFrom:at,initialCharges:1,remainingCharges:1,selector:{actionIds:["alchemy.bench"]},params:{extraMaterialQuantity:50,positiveFloorReductionRate:0.12},...overrides}
}

let app: App | undefined
let host: HTMLElement | undefined
beforeEach(() => { setActivePinia(createPinia()); vi.useFakeTimers(); vi.setSystemTime(new Date(at)); useOperationEffects().reset() })
afterEach(() => { app?.unmount(); host?.remove(); app=undefined; vi.clearAllTimers(); vi.restoreAllMocks(); vi.useRealTimers() })

describe("operation context eligibility", () => {
  it('allows success food for all material rarities, but not fish or other alchemy', () => {
    const food=effect({kind:'alchemy.infusion_success_bonus',params:{bonusRate:.03},targetKind:'player_and_kitties'})
    const context: OperationEffectContext={domain:'alchemy',actionId:'alchemy.bench',infusionMode:'materials',actorKind:'kitty',actorId:2}
    expect(operationEffectEligibility(food,context)).toEqual({applicable:false,reason:'materialsRequired'})
    for(const rarity of ['common','uncommon','rare','epic','legendary','mythic']) expect(operationEffectEligibility(food,{...context,materialRarities:[rarity]})).toEqual({applicable:true})
    expect(operationEffectEligibility(food,{...context,infusionMode:'fish',materialRarities:['common']})).toEqual({applicable:false,reason:'fishInfusion'})
    expect(operationEffectMatchesQuery(food,{...context,actionId:'alchemy.convert'})).toBe(false)
  })
  it("matches domain, selector and exact target without leaking player effects to cats", () => {
    const e = effect()
    expect(operationEffectMatchesQuery(e,{domain:"alchemy",actionId:"alchemy.bench"})).toBe(true)
    expect(operationEffectMatchesQuery(e,{domain:"alchemy",actionId:"alchemy.convert"})).toBe(false)
    expect(operationEffectMatchesQuery(e,{domain:"enhancement",actionId:"alchemy.bench"})).toBe(false)
    expect(operationEffectMatchesQuery(e,{domain:"alchemy",actionId:"alchemy.bench",actorKind:"kitty",actorId:1})).toBe(false)
    expect(operationEffectMatchesQuery(effect({targetKind:"kitty",targetActorId:2}),{domain:"alchemy",actionId:"alchemy.bench",actorKind:"kitty",actorId:1})).toBe(false)
    expect(operationEffectMatchesQuery(effect({targetKind:"all_kitties"}),{domain:"alchemy",actionId:"alchemy.bench",actorKind:"kitty",actorId:1})).toBe(true)
  })
  it("distinguishes missing materials, fish infusion and rarity restrictions", () => {
    const base: OperationEffectContext = {domain:"alchemy",actionId:"alchemy.bench",infusionMode:"materials"}
    expect(operationEffectEligibility(effect(),base)).toEqual({applicable:false,reason:"materialsRequired"})
    expect(operationEffectEligibility(effect(),{...base,infusionMode:"fish",materialRarities:["common"]})).toEqual({applicable:false,reason:"fishInfusion"})
    expect(operationEffectEligibility(effect(),{...base,materialRarities:["legendary","mythic"]})).toEqual({applicable:false,reason:"materialRarity"})
    expect(operationEffectEligibility(effect(),{...base,materialRarities:["mythic","epic"]})).toEqual({applicable:true})
    expect(operationEffectEligibility(effect({kind:"future.effect"}),base)).toEqual({applicable:false,reason:"unsupported"})
  })
  it("keeps the enhancement probability preview scoped to the selected actor", () => {
    const cache=useOperationEffects()
    cache.hydrate({revision:1,serverNow:at,items:[effect({domain:"enhancement",kind:"enhancement.success_bonus",selector:{actionIds:["equipment_enhance"]},params:{bonusRate:0.025}})]})
    expect(cache.enhancementSuccessBonusFor({actorKind:"player"})).toBe(.025)
    expect(cache.enhancementSuccessBonusFor({actorKind:"kitty",actorId:1})).toBe(0)
  })
  it("reacts to materials, expiry, stale state and account reset without fetching or spending", async () => {
    const pinia=createPinia();setActivePinia(pinia)
    const cache=useOperationEffects()
    const precision=effect({grantId:2,definitionId:"precision",nameKey:"game.fishCuisine.names.starlight_silverfish_soup",kind:"alchemy.infusion_precision_ceiling_bonus",lifetimeKind:"duration",validUntil:"2026-09-05T00:00:02.000Z",params:{positiveCeilingBonusRate:.03}})
    const items=[effect(),precision]
    const interval = vi.spyOn(window,"setInterval")
    const clearInterval = vi.spyOn(window,"clearInterval")
    cache.hydrate({revision:1,serverNow:at,items})
    const context=ref<OperationEffectContext>({domain:"alchemy",actionId:"alchemy.bench",infusionMode:"materials",materialRarities:["mythic"]})
    host=document.createElement("div");document.body.append(host)
    app=createApp({render:()=>h(OperationEffectNotice,{context:context.value})});app.use(pinia);app.use(createI18n({legacy:false,locale:"zh-CN",messages:{"zh-CN":zhCN}}));app.mount(host)
    expect(host.querySelectorAll('.operation-effect-list__item')).toHaveLength(1)
    expect(host.querySelector('details')?.open).toBe(false)
    const disclosure = host.querySelector<HTMLDetailsElement>('.operation-effect-notice__disclosure')!
    disclosure.querySelector('summary')!.click()
    expect(disclosure.open).toBe(true)
    disclosure.querySelector('summary')!.click()
    expect(disclosure.open).toBe(false)
    expect(host.textContent).toContain('需要投入至少一种史诗及以下素材')
    context.value={...context.value,materialRarities:["epic"]};await nextTick()
    expect(host.querySelectorAll('.operation-effect-list__item')).toHaveLength(2)
    expect(host.textContent).toContain('+50')
    expect(host.textContent).toContain('排队不预留次数或时间')
    vi.advanceTimersByTime(2100);await nextTick()
    expect(host.querySelectorAll('.operation-effect-list__item')).toHaveLength(1)
    expect(useOperationEffectsStore().items[0].remainingCharges).toBe(1)
    useOperationEffectsStore().markStale();await nextTick()
    expect(host.textContent).toContain('效果数据待更新')
    expect(host.querySelector('.operation-effect-list')).toBeNull()
    cache.reset();await nextTick()
    expect(host.textContent).toContain('效果数据尚未同步')
    app.unmount();app=undefined
    expect(interval).toHaveBeenCalledTimes(1)
    expect(clearInterval).toHaveBeenCalledWith(interval.mock.results[0].value)
  })
})
