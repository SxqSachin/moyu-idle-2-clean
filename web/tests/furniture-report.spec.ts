import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { createApp, h, nextTick, reactive, type App } from 'vue'
import { createPinia, setActivePinia } from 'pinia'
import { createI18n } from 'vue-i18n'
import ElementPlus from 'element-plus'
import FurnitureReport from '../src/components/game/panel/cattery/FurnitureReport.vue'
import { useGameStore } from '../src/stores/game'
import { furnitureZhCN } from '../src/locales/game/cattery-furniture'
const api=vi.hoisted(()=>({get:vi.fn(),ack:vi.fn()}))
vi.mock('@services/cattery-furniture',()=>({getFurnitureReport:api.get,acknowledgeFurnitureReport:api.ack}))
vi.mock('../src/components/game/panel/cattery/FurnitureContributionItems.vue',()=>({default:{props:['row'],template:'<pre>{{ JSON.stringify(row) }}</pre>'}}))
let app:App|undefined,container:HTMLDivElement
const flush=async()=>{await Promise.resolve();await nextTick();await Promise.resolve();await nextTick()}
beforeEach(()=>{api.get.mockReset();api.ack.mockReset()})
afterEach(()=>{app?.unmount();container?.remove()})
async function mount(active=true){const pinia=createPinia();setActivePinia(pinia);const game=useGameStore();game.token='account-a';const props=reactive({active});container=document.createElement('div');document.body.append(container);app=createApp({render:()=>h(FurnitureReport,props)}).use(pinia).use(ElementPlus).use(createI18n({legacy:false,locale:'zh-CN',messages:{'zh-CN':{game:{furniture:furnitureZhCN}}}}));app.mount(container);await flush();return {props,game}}
const report={token:'report-one',through:'2026-09-06T00:00:00Z',rows:{study_desk:{experience:{fishing:12.5}}},names:{study_desk:'game.furniture.studyDesk'}}
it('reads only while the offline dialog is active and explicitly acknowledges the exact report',async()=>{
 api.get.mockResolvedValue(report);api.ack.mockResolvedValue({})
 const {props}=await mount(false);expect(api.get).not.toHaveBeenCalled()
 props.active=true;await flush();expect(api.get).toHaveBeenCalledTimes(1);expect(container.textContent).toContain('12.5')
 container.querySelector('button')!.click();await flush();expect(api.ack).toHaveBeenCalledWith('account-a','report-one');expect(container.textContent).toContain('已查看')
 props.active=false;await flush();expect(container.textContent).toBe('')
 props.active=true;await flush();expect(api.get).toHaveBeenCalledTimes(2)
})
it('does not expose a late report from the previous account',async()=>{
 let resolve!:(v:unknown)=>void;api.get.mockReturnValueOnce(new Promise(r=>resolve=r)).mockResolvedValueOnce({token:'',rows:{}})
 const {game}=await mount();game.token='account-b';await flush();resolve(report);await flush();expect(container.textContent).not.toContain('12.5');expect(api.get).toHaveBeenLastCalledWith('account-b')
})
it('preserves retry access when acknowledgment has an uncertain result',async()=>{
 api.get.mockResolvedValue(report);api.ack.mockRejectedValue(new Error('lost response'))
 await mount();container.querySelector('button')!.click();await flush();expect(container.textContent).toContain('暂时无法读取')
 container.querySelector('button')!.click();await flush();expect(api.get).toHaveBeenCalledTimes(2);expect(container.textContent).toContain('12.5');expect(api.ack).toHaveBeenCalledTimes(1)
})
