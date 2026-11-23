import { afterAll, afterEach, expect, it, vi } from "vitest"
import { createApp, nextTick, type App } from "vue"
import { createPinia, setActivePinia } from "pinia"
import { createI18n } from "vue-i18n"
import ElementPlus from "element-plus"
import Bench from "@components/game/alchemy/AlchemyBenchPanel.vue"
import { useInventoryStore } from "@stores/inventory"
import { usePlayerStore } from "@stores/player"
import { useOperationEffectsStore } from "@stores/operation-effects"
import type { InventoryInstance } from "@/types/equipment"
import zh from "@/locales/game/zh-CN"
import data from "@/locales/game-data/zh-CN"
import { getFishFoodEffectDefinition, getFishFoodRecipeByActionId } from "@services/fish-food"
import type { OperationEffectView } from "@services/operation-effects"

vi.hoisted(() => {
  vi.stubGlobal('matchMedia', vi.fn(() => ({matches:false,addEventListener:vi.fn(),removeEventListener:vi.fn()})))
  vi.stubGlobal('ResizeObserver', class { observe() {} unobserve() {} disconnect() {} })
})
afterAll(() => vi.unstubAllGlobals())

let app: App | undefined
let host: HTMLElement | undefined
afterEach(() => { app?.unmount(); host?.remove() })

it('shows the success-only food beside real controls without changing quantity or ranges', async () => {
  const pinia=createPinia();setActivePinia(pinia)
  const inventory=useInventoryStore()
  inventory.instances=[{instanceId:999001,templateItemId:'base4_cursed_wing_membrane',quantity:1000,state:'inventory',customData:{}} as InventoryInstance]
  const player=usePlayerStore()
  const attribute=(value:number): NonNullable<typeof player.attributes>[string]=>({value,unit:'number',category:'work',growthType:'static',order:0})
  player.attributes={alchemy_level:attribute(30),alchemy_craft_success:attribute(.6),alchemy_precision:attribute(.6)}
  const effects=useOperationEffectsStore()
  const now=new Date().toISOString()
  effects.applySnapshot({revision:1,serverNow:now,items:[]})
  host=document.createElement('div');document.body.append(host)
  app=createApp(Bench)
  app.use(pinia).use(ElementPlus).use(createI18n({legacy:false,locale:'zh-CN',messages:{'zh-CN':{...data,...zh}}}))
  const vm=app.mount(host)
  const state=(vm as unknown as {$:{setupState:{handleAddMaterial(id:number):void;materials:Record<string,number>;aggregatedSuccessPct:number;benchMode:string}}}).$.setupState
  state.handleAddMaterial(999001);state.materials.base4_cursed_wing_membrane=48
  await nextTick()
  const input=()=>host!.querySelector<HTMLInputElement>('.alchemy-material-quantity input')!
  const capacity=input().max
  const before=state.aggregatedSuccessPct
  const def=getFishFoodEffectDefinition(getFishFoodRecipeByActionId('cook_creamfin_dual_fish_grill'))!
  const food={...def,grantId:1,definitionId:def.id,targetKind:'player_and_kitties',lifetimeKind:'charges',validFrom:now,initialCharges:6,remainingCharges:6} as OperationEffectView
  effects.applySnapshot({revision:2,serverNow:now,items:[food]})
  await nextTick()
  expect(state.aggregatedSuccessPct).toBeCloseTo(Math.min(1,before+.05))
  expect(host.textContent).toContain('料理成功率 +5.00%')
  expect(input().max).toBe(capacity)
  expect(host.textContent).not.toContain('料理上限')
  expect(host.textContent).not.toContain('料理下限')
  expect(effects.items[0].remainingCharges).toBe(6)
  effects.applySnapshot({revision:3,serverNow:now,items:[]})
  await nextTick()
  expect(state.aggregatedSuccessPct).toBeCloseTo(before)
  expect(host.textContent).not.toContain('料理成功率')
  effects.applySnapshot({revision:4,serverNow:now,items:[food]})
  state.benchMode='fish'
  await nextTick()
  expect(host.textContent).toContain('不适用于鱼类灌注')
  expect(host.textContent).not.toContain('料理成功率')
})

it('uses the current three recipe tiers in real controls without compounding capacity', async () => {
  const pinia=createPinia();setActivePinia(pinia)
  const inventory=useInventoryStore()
  inventory.instances=['base2_moss_spore','goldvine_wood','moonwood'].map((templateItemId,i)=>({instanceId:9000+i,templateItemId,quantity:100000,state:'inventory',customData:{}} as InventoryInstance))
  const player=usePlayerStore()
  const attribute=(value:number): NonNullable<typeof player.attributes>[string]=>({value,unit:'number',category:'work',growthType:'static',order:0})
  player.attributes={alchemy_level:attribute(80),alchemy_craft_success:attribute(1000000),alchemy_precision:attribute(.6)}
  const effects=useOperationEffectsStore()
  const now=new Date().toISOString()
  const items=['mossfire','lotusflame','frostmoon'].map((series,i)=>{
    const def=getFishFoodEffectDefinition(getFishFoodRecipeByActionId('cook_'+series+'_dual_fish_grill'))!
    return {...def,grantId:i+1,definitionId:def.id,targetKind:'player_and_kitties',lifetimeKind:'charges',validFrom:now,initialCharges:1,remainingCharges:1}
  }) as OperationEffectView[]
  effects.applySnapshot({revision:1,serverNow:now,items})
  host=document.createElement('div');document.body.append(host)
  app=createApp(Bench)
  app.use(pinia).use(ElementPlus).use(createI18n({legacy:false,locale:'zh-CN',messages:{'zh-CN':{...data,...zh}}}))
  const vm=app.mount(host)
  const state=(vm as unknown as {$:{setupState:{handleAddMaterial(id:number):void;activeSlotIndex:number|null}}}).$.setupState
  for(let i=0;i<3;i++){state.activeSlotIndex=null;state.handleAddMaterial(9000+i)}
  await nextTick()
  const maxima=()=>[...host!.querySelectorAll<HTMLInputElement>('.alchemy-material-quantity input')].map(input=>Number(input.max))
  expect(maxima()).toEqual([6820,6840,6840])
  expect(host.textContent).toContain('料理成功率 -7.50%')
  effects.applySnapshot({revision:2,serverNow:now,items:items.map((item,i)=>({...item,params:{...item.params,quantityLimitBonusRate:[.20,.15,.12][i]}}))})
  await nextTick()
  expect(maxima()).toEqual([7640,7350,7160])
  expect(effects.items.map(item=>item.remainingCharges)).toEqual([1,1,1])
  effects.applySnapshot({revision:3,serverNow:now,items:[]})
  await nextTick()
  expect(maxima()).toEqual([6380,6395,6396])
  expect(host.textContent).not.toContain('料理上限')
})

it('updates the real legendary quantity control from precious food and removes it on cache refresh', async () => {
  const pinia = createPinia()
  setActivePinia(pinia)
  const inventory = useInventoryStore()
  inventory.instances = [{instanceId:987655,templateItemId:'moonwood',quantity:1000,state:'inventory',customData:{}} as InventoryInstance]
  const player = usePlayerStore()
  const attribute = (value: number): NonNullable<typeof player.attributes>[string] => ({value,unit:'number',category:'work',growthType:'static',order:0})
  player.attributes = {alchemy_level:attribute(50),alchemy_craft_success:attribute(.6),alchemy_precision:attribute(.6)}
  const effects = useOperationEffectsStore()
  const now = new Date().toISOString()
  effects.applySnapshot({revision:1,serverNow:now,items:[{grantId:2,definitionId:'food_frostmoon_dual_fish_grill_v1',rulesetVersion:'food-frostmoon-v1',nameKey:'game.fishCuisine.names.frostmoon_dual_fish_grill',descKey:'',kind:'alchemy.infusion_precious_capacity_bonus',domain:'alchemy',targetKind:'player_and_kitties',lifetimeKind:'charges',validFrom:now,initialCharges:1,remainingCharges:1,selector:{actionIds:['alchemy.bench']},params:{extraMaterialQuantity:25}}]})
  host = document.createElement('div'); document.body.append(host)
  app = createApp(Bench)
  app.use(pinia).use(ElementPlus).use(createI18n({legacy:false,locale:'zh-CN',messages:{'zh-CN':{...data,...zh}}}))
  const vm = app.mount(host)
  const state = (vm as unknown as { $: { setupState: { handleAddMaterial(id: number): void } } }).$.setupState
  state.handleAddMaterial(987655)
  await nextTick()
  expect(host.textContent).toContain('料理上限 +24')
  expect(host.textContent).not.toContain('料理下限')
  expect(host.querySelector<HTMLDetailsElement>('.operation-effect-notice__disclosure')?.open).toBe(false)
  const input = () => host!.querySelector<HTMLInputElement>('.alchemy-material-quantity input')!
  const boosted = Number(input().max)
  expect(effects.items[0].remainingCharges).toBe(1)
  effects.applySnapshot({revision:2,serverNow:now,items:[]})
  await nextTick()
  expect(Number(input().max)).toBe(boosted - 24)
  expect(host.textContent).not.toContain('料理上限')
  player.attributes.alchemy_craft_success.value = 1_000_000
  inventory.instances[0].quantity = 100_000
  effects.applySnapshot({revision:3,serverNow:now,items:[{grantId:3,definitionId:'food_frostmoon_dual_fish_grill_v3',rulesetVersion:'food-capacity-risk-v1',nameKey:'game.fishCuisine.names.frostmoon_dual_fish_grill',descKey:'',kind:'alchemy.infusion_precious_capacity_bonus',domain:'alchemy',targetKind:'player_and_kitties',lifetimeKind:'charges',validFrom:now,initialCharges:1,remainingCharges:1,selector:{actionIds:['alchemy.bench']},params:{extraMaterialQuantity:91,quantityLimitBonusRate:.17,successRatePenalty:.03}}]})
  await nextTick()
  expect(Number(input().max)).toBeGreaterThan(6400)
  expect(host.textContent).toContain('无料理理论上界 6400 → 含料理 7488')
  expect(host.textContent).toContain('料理成功率 -3.00%')
  expect(host.textContent).toContain('97.00%')
  effects.applySnapshot({revision:4,serverNow:now,items:[]})
  await nextTick()
  expect(Number(input().max)).toBeLessThanOrEqual(6400)
  expect(host.textContent).not.toContain('料理成功率')
})

it('renders the actual material controls with separate capacity, inventory and food range feedback', async () => {
  const pinia = createPinia()
  setActivePinia(pinia)
  const inventory = useInventoryStore()
  inventory.instances = [{instanceId:987654,templateItemId:'base4_cursed_wing_membrane',quantity:50,state:'inventory',customData:{}} as InventoryInstance]
  const player = usePlayerStore()
  const attribute = (value: number): NonNullable<typeof player.attributes>[string] => ({value,unit:'number',category:'work',growthType:'static',order:0})
  player.attributes = {alchemy_level:attribute(30),alchemy_craft_success:attribute(.6),alchemy_precision:attribute(.6)}
  const effects = useOperationEffectsStore()
  const now = new Date().toISOString()
  const food = {grantId:1,definitionId:'food_mossfire_dual_fish_grill_v4',rulesetVersion:'test',nameKey:'game.fishCuisine.names.mossfire_dual_fish_grill',descKey:'',kind:'alchemy.infusion_bulk_capacity_floor_tradeoff',domain:'alchemy',targetKind:'player_and_kitties',lifetimeKind:'charges',validFrom:now,initialCharges:2,remainingCharges:2,selector:{actionIds:['alchemy.bench']},params:{extraMaterialQuantity:50,positiveFloorReductionRate:.12}} as const
  effects.applySnapshot({revision:1,serverNow:now,items:[{...food,selector:{actionIds:['alchemy.bench']}}]})
  host = document.createElement('div');document.body.append(host)
  app = createApp(Bench)
  app.use(pinia).use(ElementPlus).use(createI18n({legacy:false,locale:'zh-CN',messages:{'zh-CN':{...data,...zh}}}))
  const vm = app.mount(host)
  // Exercise the same material-selection handler as the inventory picker.
  const state = (vm as unknown as { $: { setupState: { handleAddMaterial(id: number): void; materials: Record<string, number> } } }).$.setupState
  state.handleAddMaterial(987654)
  state.materials.base4_cursed_wing_membrane = 48
  await nextTick()
  expect(host.querySelector<HTMLDetailsElement>('.operation-effect-notice__disclosure')?.open).toBe(false)
  expect(host.textContent).toContain('用量 8 ~ 368')
  expect(host.textContent).toContain('料理上限 +48')
  expect(host.textContent).toContain('库存 50 件，每组 8 件，当前可投入 48 件')
  expect(host.textContent).toContain('料理下限 -0.071')
  const input = () => host!.querySelector<HTMLInputElement>('.alchemy-material-quantity input')!
  expect(input().max).toBe('48')
  inventory.instances[0].quantity = 1000
  await nextTick()
  expect(input().max).toBe('368')
  expect(effects.items[0].remainingCharges).toBe(2)
  effects.applySnapshot({revision:2,serverNow:now,items:[]})
  await nextTick()
  expect(input().max).toBe('320')
  expect(host.textContent).not.toContain('料理下限')
  expect(host.textContent).not.toContain('料理上限')
  inventory.instances[0].quantity = 7
  await nextTick()
  await vi.waitFor(() => expect(input().disabled).toBe(true))
  expect(input().max).toBe('0')
  expect(input().min).toBe('0')
})
