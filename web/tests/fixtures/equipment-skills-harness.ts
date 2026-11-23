import { createApp, defineComponent, h, reactive } from "vue"
import { createPinia } from "pinia"
import i18n, { setAppLocale } from "@/i18n"
import "@/style.css"
import EquipmentSkillSettings from "@/components/game/skill/EquipmentSkillSettings.vue"
import { loadAllGameData } from "@/services/game-data-runtime"
import { listSkillDefinitions } from "@/services/skill-definition"
import { emptyEquipmentSkillConfig, type EquipmentSkillView, type EquipmentSkillConfig } from "@/services/equipment-skill-config"

await setAppLocale("zh-CN")
await loadAllGameData()
const definitions = listSkillDefinitions()
const physical = definitions.filter(row => row.id !== "basic_attack" && row.effects?.some(effect => effect.kind === "damage" && effect.damageType === "physical"))
const ids = physical.slice(0, 4).map(row => row.id)
const draftMode = new URLSearchParams(location.search).has("draft")
const views = reactive<EquipmentSkillView[]>([
  {skillId:"equipment_domination",itemId:"domination_seal",enhanceLevel:20,capacity:3,config:{...emptyEquipmentSkillConfig(),enabled:true,skillIds:[ids[0]]}},
  {skillId:"equipment_martial_combo",itemId:"hundred_battle_badge",enhanceLevel:20,capacity:4,config:{...emptyEquipmentSkillConfig(),enabled:true,skillIds:ids.slice(0,2),beforeSkillId:draftMode ? "removed_skill" : ""}}
])
const catalog = definitions.map(row => ({id:row.id,category:row.category,hpCost:row.hpCost||0,mpCost:row.mpCost||0,cooldown:row.cooldown||0,eligible:!draftMode,comboEligible:physical.some(skill=>skill.id===row.id),dominationEligible:!row.id.startsWith("equipment_") && row.id!=="basic_attack"}))
const qa = {views,ids,draftMode,failNext:false,requests:[] as unknown[]}
Object.assign(window,{equipmentSkillQA:qa})
const originalFetch = window.fetch.bind(window)
window.fetch = async (input, init) => {
  if (String(input).endsWith("/api/battle/skills/equipment-config")) {
    const body = JSON.parse(String(init?.body)); qa.requests.push(body)
    if (qa.failNext) { qa.failNext=false; return new Response(JSON.stringify({ok:false,error:{code:"EQUIPMENT_SKILL_CONFIG_CHANGED"}}),{status:409}) }
    return new Response(JSON.stringify({ok:true,data:{config:{...body.config,revision:body.config.revision+1}}}),{status:200})
  }
  return originalFetch(input,init)
}
const Host = defineComponent({setup:()=>()=>h("main",{style:"max-width:960px;margin:24px auto;padding:12px"},[
  h(EquipmentSkillSettings,{token:"isolated-fixture",views,catalog,draftMode,carried:ids,owned:ids.map(skillId=>({skillId,level:20})),itemOptions:[],onSaved:(id:string,cfg:EquipmentSkillConfig)=>{views.find(row=>row.skillId===id)!.config=cfg}})
])})
createApp(Host).use(createPinia()).use(i18n).mount("#app")
