import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"
import { createApp, defineComponent, h, nextTick, reactive, type App } from "vue"
import { createI18n } from "vue-i18n"
import { ElAlert, ElButton, ElCheckbox, ElPagination } from "element-plus"
import type { InventoryInstance } from "../src/types/equipment"
import { destructiveRecycleZhCN } from "../src/locales/game/destructive-recycle"

const mocks = vi.hoisted(() => ({api:vi.fn(),store:vi.fn()}))
vi.mock("@services/api", async (importOriginal) => ({...await importOriginal<typeof import("@services/api")>(),apiPost:mocks.api}))
vi.mock("@stores/game", () => ({useGameStore:mocks.store}))
vi.mock("@services/item-label", () => ({getLocalizedItemName: () => "Wooden sword"}))
vi.mock("@components/game/common/GameDialog.vue", () => ({default:defineComponent({props:["modelValue"],setup:(props,{slots})=>()=>props.modelValue?h("section",[slots.default?.(),slots.footer?.()]):null})}))
vi.mock("@components/game/item/Card.vue", () => ({default:defineComponent({setup:()=>()=>h("span","icon")})}))
vi.mock("@components/game/item/QuickRef.vue", () => ({default:defineComponent({props:["count"],setup:props=>()=>h("span",String(props.count))})}))
vi.mock("@components/game/affix/AffixQuickRef.vue", () => ({default:defineComponent({props:["affixId"],setup:props=>()=>h("span",props.affixId)})}))
import ItemDestructiveRecycleDialog from "../src/components/game/item/ItemDestructiveRecycleDialog.vue"

let app: App | undefined
let host: HTMLDivElement
let store: {token:string}
let state: {open:boolean;items:InventoryInstance[]}
let done: ReturnType<typeof vi.fn>
const quote = {items:[{instanceId:11,templateItemId:"wooden_sword",enhanceLevel:10,affixes:[{slotIndex:0,affixId:"affix.test",rarityId:"epic"}],bodyGold:35,disposalFee:4,totalGold:31,quoteToken:"a".repeat(64)}],feePercent:10,bodyGold:35,disposalFee:4,totalGold:31}
function deferred() { let resolve!: (value:any)=>void; let reject!: (err:Error)=>void; const promise = new Promise((yes,no)=>{resolve=yes;reject=no}); return {promise,resolve,reject} }
async function settle() { await Promise.resolve(); await nextTick(); await Promise.resolve(); await nextTick() }
function button(text: string) { return [...host.querySelectorAll("button")].find(el=>el.textContent?.includes(text))! }
async function acknowledge() { host.querySelector<HTMLInputElement>('input[type="checkbox"]')!.click(); await settle() }
function mount() {
  host = document.createElement("div"); document.body.append(host)
  app = createApp({render:()=>h(ItemDestructiveRecycleDialog,{modelValue:state.open,items:state.items,"onUpdate:modelValue":(v:boolean)=>state.open=v,onDone:done})})
  app.use(createI18n({legacy:false,locale:"zh-CN",messages:{"zh-CN":{game:{destructiveRecycle:destructiveRecycleZhCN,cancel:"取消",recycleTotalGoldLabel:"回收总价",batchRecycleSuccess:"回收成功 {gold}",recycleFailed:"回收失败"}}}}))
  app.component("el-button",ElButton); app.component("el-checkbox",ElCheckbox); app.component("el-alert",ElAlert)
  app.component("el-pagination",ElPagination)
  app.mount(host)
}
beforeEach(()=>{
  mocks.api.mockReset()
  store = reactive({token:"account-a"}); mocks.store.mockReturnValue(store)
  state = reactive({open:true,items:[{instanceId:11,templateItemId:"wooden_sword",quantity:1,state:"inventory",enhanceLevel:0,durability:100,maxDurability:100,affixes:[{slotIndex:0,affixId:"affix.test",rarityId:"epic"}]}]})
  done=vi.fn()
})
afterEach(()=>{app?.unmount();host?.remove();vi.clearAllMocks()})

describe("destructive recycling consent",()=>{
  it("uses server prices and requires unchecked consent, prevents double submission",async()=>{
    const commit=deferred()
    mocks.api.mockResolvedValueOnce(quote).mockReturnValueOnce(commit.promise)
    mount(); await settle()
    expect(host.textContent).toContain("35"); expect(host.textContent).toContain("31")
    expect(host.textContent).toContain("+10"); expect(host.textContent).toContain("1 件已强化")
    expect(button("确认折价回收").disabled).toBe(true)
    await acknowledge(); button("确认折价回收").click(); button("确认折价回收").click(); await settle()
    expect(mocks.api).toHaveBeenCalledTimes(2)
    expect(mocks.api.mock.calls[1]).toEqual(["/api/items/recycle/destructive","account-a",{items:[{instanceId:11,quoteToken:"a".repeat(64)}]}])
    expect(button("取消").disabled).toBe(true)
    commit.resolve(quote); await settle()
    expect(done).toHaveBeenCalledTimes(1); expect(state.open).toBe(false)
  })
  it("invalidates consent on inventory changes and does not automatically re-quote",async()=>{
    mocks.api.mockResolvedValue(quote); mount(); await settle(); await acknowledge()
    state.items[0].affixes![0].rarityId="mythic"; await settle()
    expect(host.textContent).toContain(destructiveRecycleZhCN.changed)
    expect(button("确认折价回收")).toBeUndefined()
    expect(mocks.api).toHaveBeenCalledTimes(1)
    button("重新核价").click(); await settle()
    expect(button("确认折价回收").disabled).toBe(true)
  })
  it("ignores quotes from a previous account and after closure",async()=>{
    const pending=deferred(); mocks.api.mockReturnValue(pending.promise); mount(); await settle()
    store.token="account-b"; await settle(); pending.resolve(quote); await settle()
    expect(button("确认折价回收")).toBeUndefined()
    expect(done).not.toHaveBeenCalled()
    state.open=false; await settle(); expect(host.textContent).toBe("")
  })
  it("requires new preview and consent after an uncertain mutation response",async()=>{
    mocks.api.mockResolvedValueOnce(quote).mockRejectedValueOnce(new Error("network"))
    mount(); await settle(); await acknowledge(); button("确认折价回收").click(); await settle()
    expect(button("确认折价回收")).toBeUndefined()
    expect(button("重新核价")).toBeDefined()
    expect(mocks.api).toHaveBeenCalledTimes(2); expect(done).not.toHaveBeenCalled()
  })
  it("never carries confirmation across reopen",async()=>{
    mocks.api.mockResolvedValue(quote); mount(); await settle(); await acknowledge()
    state.open=false; await settle(); state.open=true; await settle()
    expect(button("确认折价回收").disabled).toBe(true)
  })
  it("ignores late completion after the dialog is closed and reopened",async()=>{
    const commit=deferred()
    mocks.api.mockResolvedValueOnce(quote).mockReturnValueOnce(commit.promise)
    mount(); await settle(); await acknowledge(); button("确认折价回收").click(); await settle()
    state.open=false; await settle(); state.open=true; await settle()
    commit.resolve(quote); await settle()
    expect(done).not.toHaveBeenCalled(); expect(state.open).toBe(true)
  })
  it("bounds rendered equipment rows without shrinking the confirmed selection",async()=>{
    mocks.api.mockResolvedValue({...quote,items:Array.from({length:45},(_,i)=>({...quote.items[0],instanceId:100+i}))})
    mount(); await settle()
    expect(host.querySelectorAll(".destructive-recycle__item")).toHaveLength(20)
    expect(host.textContent).toContain("45 件装备")
    expect(button("确认折价回收").disabled).toBe(true)
  })
})
