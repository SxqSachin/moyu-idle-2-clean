import { createApp, defineComponent, h, reactive } from "vue"
import { createPinia } from "pinia"
import i18n, { setAppLocale } from "@/i18n"
import "@/style.css"
import ItemDestructiveRecycleDialog from "@/components/game/item/ItemDestructiveRecycleDialog.vue"
import { loadAllGameData } from "@/services/game-data-runtime"
import { getEquipmentTemplateDefinition } from "@/services/item-definition"
import { useGameStore } from "@/stores/game"

await setAppLocale(new URLSearchParams(location.search).get("locale") || "zh-CN")
await loadAllGameData()
const pinia = createPinia()
const store = useGameStore(pinia)
store.token = "isolated-visual-test"
const itemId = "coarse_linen_gatherer_trousers"
const template = getEquipmentTemplateDefinition(itemId)!
const affixes = [{slotIndex:0,affixId:"strength",rarityId:"epic",rollValue:0.8}, {slotIndex:1,affixId:"dexterity",rarityId:"legendary",rollValue:0.9}]
const rows = Array.from({length:3},(_,i)=>({instanceId:90001+i,templateItemId:itemId,quantity:1,enhanceLevel:i<2?10:0,state:"inventory",durability:template.maxDurability,maxDurability:template.maxDurability,affixes:i===1?[]:affixes}))
const originalFetch = window.fetch.bind(window)
// Visual fixture only. No authenticated request or live inventory mutation.
window.fetch = async (input, init) => {
  const url = String(input)
  if (url.includes("/api/items/recycle/destructive/quote")) return new Response(JSON.stringify({ok:true,data:{items:rows.map(row=>({...row,bodyGold:35,disposalFee:4,totalGold:31,quoteToken:"a".repeat(64)})),feePercent:10,bodyGold:105,disposalFee:12,totalGold:93}}),{headers:{"Content-Type":"application/json"}})
  if (url.includes("/api/")) return new Response(JSON.stringify({ok:false,error:{message:"Visual test: mutation disabled"}}),{status:400})
  return originalFetch(input,init)
}
const state = reactive({open:true,rows})
const Host = defineComponent({setup:()=>()=>h(ItemDestructiveRecycleDialog,{modelValue:state.open,items:state.rows,"onUpdate:modelValue":v=>state.open=v})})
createApp(Host).use(pinia).use(i18n).mount("#app")
