import { afterEach, expect, it, vi } from "vitest"
import { createApp, h, nextTick, type App } from "vue"
import { createPinia, setActivePinia } from "pinia"
import { createI18n } from "vue-i18n"
import ElementPlus from "element-plus"
import Toolbar from "@components/game/operation-effect/GlobalEffectToolbar.vue"
import EffectList from "@components/game/operation-effect/OperationEffectList.vue"
import { useOperationEffectsStore } from "@stores/operation-effects"
import zh from "@/locales/game/zh-CN"
import type { OperationEffectView } from "@services/operation-effects"

vi.mock("@/composables/game/useDevice",async()=>{const {ref}=await import("vue");return {useDevice:()=>({isMobile:ref(true)})}})
let app: App | undefined
let host: HTMLElement | undefined
afterEach(()=>{app?.unmount();host?.remove()})

it('uses a compact counter and omits narrative text without losing numerical effects or audience',async()=>{
  const pinia=createPinia();setActivePinia(pinia)
  const now=new Date().toISOString()
  const item:OperationEffectView={grantId:1,definitionId:'food',rulesetVersion:'v1',domain:'fishing_cast',kind:'fishing_cast.bite_bonus',nameKey:'game.fishCuisine.names.reedbank_soup',descKey:'game.fishCuisine.descriptions.reedbank_soup',targetKind:'player_and_kitties',lifetimeKind:'charges',validFrom:now,remainingCharges:12,initialCharges:12,selector:{actionIds:['fishing.trip']},params:{bonusRate:.08}}
  useOperationEffectsStore().applySnapshot({revision:1,serverNow:now,items:[item]})
  host=document.createElement('div');document.body.append(host)
  app=createApp({render:()=>h('div',[h(Toolbar),h(EffectList,{items:[item],nowMs:Date.now()})])})
  app.use(pinia).use(ElementPlus).use(createI18n({legacy:false,locale:'zh-CN',messages:{'zh-CN':zh}})).mount(host)
  await nextTick()
  expect(host.querySelector('.global-effect-toolbar__count')?.textContent).toBe('1')
  expect(host.querySelector('.el-badge')).toBeNull()
  expect(host.querySelector('.global-effect-toolbar__trigger')?.getAttribute('aria-expanded')).toBe('false')
  expect(host.textContent).toContain('上钩概率 +8%')
  expect(host.textContent).toContain('剩余 12 次')
  expect(host.textContent).toContain('玩家与所有猫咪')
  expect(host.textContent).not.toContain('不改变鱼种')
  expect(host.textContent).not.toContain('个百分点')
  expect(host.textContent).not.toContain('共享次数')
})
