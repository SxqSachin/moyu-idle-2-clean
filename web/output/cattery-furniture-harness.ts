import { createApp, defineComponent, h, reactive } from 'vue'
import { createPinia } from 'pinia'
import ElementPlus from 'element-plus'
import i18n, { setAppLocale } from '@/i18n'
import '@/style.css'
import Furniture from '@/components/game/panel/cattery/CatteryFurnitureSection.vue'
import FurnitureReport from '@/components/game/panel/cattery/FurnitureReport.vue'
import GameDialog from '@/components/game/common/GameDialog.vue'
import { loadAllGameData } from '@/services/game-data-runtime'
import { useGameStore } from '@/stores/game'
import source from '../../packages/game-data/source/json/cattery_furniture.json'
await setAppLocale(new URLSearchParams(location.search).get('locale') || 'zh-CN')
await loadAllGameData()
const pinia = createPinia(), game = useGameStore(pinia)
game.token = 'isolated-furniture-qa'
// Item tooltips restore authentication; keep this fixture isolated from saved accounts.
game.restoreToken = () => { game.token = 'isolated-furniture-qa' }
const commands: unknown[] = []
const requests: string[] = []
const pendingChanges = new Map<string, { previous: any; target: any; effectiveAt: number }>()
const delayMs = Math.max(0, Number(new URLSearchParams(location.search).get('delay') || 0)) * 1000
const ui = reactive({ reportOpen: new URLSearchParams(location.search).has('report') })
const state = { revision:1, inventory:{} as Record<string,number>, items:Object.fromEntries(source.cattery_furniture.map(d=>[d.id,{level:1,installed:true,enabled:true,remainingMs:50000000}])) }
state.items.kitty_training_bench.level=Math.min(10,Math.max(1,Number(new URLSearchParams(location.search).get('level')||1)))
state.items.kitty_training_bench.installed=!new URLSearchParams(location.search).has('install')
for (const d of source.cattery_furniture) for (const costs of [d.installCosts,d.refillCosts,...d.levels.flatMap(l=>[l.upgradeCosts,l.refillCostAdd])]) for (const id of Object.keys(costs)) state.inventory[id]=20000
if (new URLSearchParams(location.search).has('shortage')) {state.inventory.copper=0;state.inventory.base2_cave_membrane=0}
const flatten=(d:any,level:number)=>{const out=structuredClone(d);out.level=level;out.levels=undefined;for(const l of d.levels.filter((x:any)=>x.level<=level)){for(const [id,n] of Object.entries(l.refillCostAdd))out.refillCosts[id]=(out.refillCosts[id]||0)+Number(n);out.rules=l.rules}return out}
const rows={study_desk:{experience:{woodcutting:1452.3456,fishing:302.5}},loot_cabinet:{items:{base2_cave_membrane:12,base2_ruin_core_shard:4},opportunities:120,triggered:7,empty:2}}
function projection() {
 const now=Date.now()
 return {activationDelaySeconds:delayMs/1000,revision:state.revision,available:true,serverNow:new Date(now).toISOString(),inventory:state.inventory,contributions:rows,items:source.cattery_furniture.map(d=>{
  const saved=state.items[d.id],definition=flatten(d,saved.level),target=saved.level<d.levels.length+1?flatten(d,saved.level+1):undefined
  const supplementCosts:Record<string,number>={}
  if(target)for(const[id,n]of Object.entries(target.refillCosts)) {
   const diff=Number(n)-(definition.refillCosts[id]||0)
   if(diff>0)supplementCosts[id]=Math.ceil(diff*saved.remainingMs/(definition.refillSeconds*1000))
  }
  const base=target?d.levels[saved.level-1].upgradeCosts:{},total:Record<string,number>={...base}
  for(const[id,n]of Object.entries(supplementCosts))total[id]=(total[id]||0)+n
  const permanent=definition.service==='permanent'
  const pending = pendingChanges.get(d.id)
  if (pending && pending.effectiveAt > now) return {...pending.previous,upgrade:undefined,refillAllowed:false,pending:{definition:pending.target.definition,installed:pending.target.installed,remainingMs:pending.target.remainingMs,enabledUntil:pending.target.enabledUntil,effectiveAt:new Date(pending.effectiveAt).toISOString()}}
  if (pending) pendingChanges.delete(d.id)
  return {definition,installed:saved.installed,maxLevel:d.levels.length+1,refillAllowed:!permanent,remainingMs:permanent?0:saved.remainingMs,enabledUntil:!permanent&&saved.enabled?new Date(now+saved.remainingMs).toISOString():undefined,upgrade:target&&saved.installed?{token:'visual-quote',expiresAt:new Date(now+30000).toISOString(),remainingMs:permanent?0:saved.remainingMs,target,upgradeCosts:base,supplementCosts,totalCosts:total}:undefined}
 })}
}
let reviewed=false
const realFetch=window.fetch.bind(window)
window.fetch=async(input,init)=>{
 const url=String(input),ok=(data:unknown)=>new Response(JSON.stringify({ok:true,data}),{headers:{'Content-Type':'application/json'}})
 if(url.includes('/api/cattery/furniture/report/ack')){reviewed=true;return ok({})}
 if(url.includes('/api/cattery/furniture/report'))return ok(reviewed?{token:'',rows:{}}:{token:'visual-report',through:new Date().toISOString(),rows,names:{study_desk:'game.furniture.studyDesk',loot_cabinet:'game.furniture.lootCabinet'}})
 if(url.endsWith('/api/cattery/furniture') || url.endsWith('/api/cattery/furniture?view=summary')) {
  requests.push(`${init?.method || 'GET'} ${url}`)
  if(init?.method==='POST') {
   const cmd=JSON.parse(String(init.body)),saved=state.items[cmd.furnitureId]
   const view=projection().items.find(x=>x.definition.id===cmd.furnitureId)!
   const costs=cmd.action==='upgrade'?view.upgrade!.totalCosts:cmd.action==='refill'?view.definition.refillCosts:cmd.action==='install'?view.definition.installCosts:{}
   for(const[id,n]of Object.entries(costs))state.inventory[id]-=Number(n)*(cmd.action==='refill'?cmd.count:1)
   commands.push(cmd);state.revision++
   if(cmd.action==='upgrade')saved.level++
   if(cmd.action==='install')saved.installed=true
   if(cmd.action==='pause')saved.enabled=false
   if(cmd.action==='enable')saved.enabled=true
   if(cmd.action==='refill')saved.remainingMs+=36000000*cmd.count
   if (delayMs) pendingChanges.set(cmd.furnitureId,{previous:view,target:projection().items.find(x=>x.definition.id===cmd.furnitureId),effectiveAt:Date.now()+delayMs})
  }
  const p=projection()
  return ok(url.endsWith('?view=summary') ? {...p,summary:true,inventory:{},contributions:undefined,items:p.items.map(item=>({...item,upgrade:undefined}))} : p)
 }
 if(url.includes('/api/'))return ok([])
 return realFetch(input,init)
}
Object.assign(window,{__furnitureQA:{state,ui,commands,requests}})
const Host=defineComponent({setup:()=>()=>h('main',{style:'max-width:1120px;margin:24px auto;padding:20px;background:var(--app-panel);border:1px solid var(--app-border);border-radius:6px'},[
 h(Furniture),h(GameDialog,{modelValue:ui.reportOpen,'onUpdate:modelValue':(v:boolean)=>ui.reportOpen=v,title:'离线结算',width:'720px'},()=>h(FurnitureReport,{active:ui.reportOpen}))
])})
createApp(Host).use(pinia).use(i18n).use(ElementPlus).mount('#app')
