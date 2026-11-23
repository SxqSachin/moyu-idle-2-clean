import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { createApp, defineComponent, h, nextTick, reactive, type App } from 'vue'
import { createPinia } from 'pinia'
import { createI18n } from 'vue-i18n'
import ElementPlus from 'element-plus'
import Furniture from '../src/components/game/panel/cattery/CatteryFurnitureSection.vue'
import { furnitureZhCN } from '../src/locales/game/cattery-furniture'
import type { FurnitureProjection } from '../src/services/cattery-furniture'

const mocks = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn(), game: vi.fn(), initialize: vi.fn() }))
vi.mock('../src/services/cattery-furniture', async original => ({ ...await original<object>(), getFurniture: mocks.get, commandFurniture: mocks.post }))
vi.mock('../src/stores/game', () => ({ useGameStore: mocks.game }))
vi.mock('../src/stores/inventory', () => ({ useInventoryStore: () => ({ initialize: mocks.initialize }) }))
vi.mock('../src/components/game/panel/cattery/FurnitureCosts.vue', () => ({ default: { props: ['costs'], setup: (props: { costs: object }) => () => h('div', { class: 'costs' }, JSON.stringify(props.costs)) } }))
vi.mock('../src/components/game/panel/cattery/FurnitureContributionItems.vue', () => ({ default: { render: () => h('div', 'contributions') } }))
vi.mock('../src/components/game/common/GameDialog.vue', () => ({ default: defineComponent({
  props: ['modelValue', 'title'], emits: ['update:modelValue'],
  setup: (props, { slots, emit }) => () => props.modelValue ? h('div', { role: 'dialog' }, [
    h('h2', props.title), h('button', { class: 'close-dialog', onClick: () => emit('update:modelValue', false) }, 'close'),
    slots.default?.(), h('footer', slots.footer?.()),
  ]) : null,
}) }))

let app: App, target: HTMLDivElement, game: { token: string }, state: FurnitureProjection
async function settle() { for (let i = 0; i < 12; i++) await nextTick() }
async function press(selector: string) { (target.querySelector(selector) as HTMLButtonElement).click(); await settle() }
async function command(label: string, scope = '[role=dialog]') {
  const button = [...target.querySelectorAll<HTMLButtonElement>(`${scope} button`)].find(el => el.textContent?.trim() === label)
  expect(button, label).toBeTruthy(); button!.click(); await settle()
}
beforeEach(async () => {
  vi.clearAllMocks()
  game = reactive({ token: 'a' }); mocks.game.mockReturnValue(game)
  const definition = { id: 'bench', version: 'v1', nameKey: 'game.furniture.trainingBench', iconId: 'item.kitty_training_bench', service: 'permanent' as const, level: 1, installCosts: { wood: 100 }, refillCosts: {}, capacitySeconds: 0, refillSeconds: 0, rules: [{ kind: 'action_efficiency_add' as const, domain: 'action_start', rateBps: 2000 }] }
  state = { revision: 1, serverNow: new Date().toISOString(), available: true, inventory: { wood: 1000 }, items: [
    { installed: false, remainingMs: 0, maxLevel: 10, definition },
    { installed: true, remainingMs: 36000000, maxLevel: 6, definition: { ...definition, id: 'desk', nameKey: 'game.furniture.studyDesk', service: 'prepaid', capacitySeconds: 120000, refillSeconds: 36000, refillCosts: { wood: 10 }, rules: [{ kind: 'owner_experience', domain: 'life_action', rateBps: 1500 }] } },
  ] }
  mocks.get.mockImplementation(async () => structuredClone(state))
  mocks.initialize.mockResolvedValue(undefined)
  target = document.createElement('div'); document.body.append(target)
  app = createApp(Furniture).use(createPinia()).use(createI18n({ legacy: false, locale: 'zh-CN', messages: { 'zh-CN': { game: { furniture: furnitureZhCN } } } })).use(ElementPlus)
  app.mount(target); await settle()
})
afterEach(() => { app.unmount(); target.remove() })

it('starts with summary cards only and opens the selected furniture on demand', async () => {
  expect(target.querySelectorAll('.furniture-card')).toHaveLength(2)
  expect(target.querySelector('[role=dialog]')).toBeNull()
  expect(target.querySelector('.costs')).toBeNull()
  expect(target.querySelector('[data-furniture=bench]')?.textContent).toContain('Lv.0 / 10')
  expect(target.querySelector('[data-furniture=bench]')?.textContent).toContain('安置后效果')
  await press('[data-furniture=desk]')
  expect(target.querySelector('[role=dialog] h2')?.textContent).toBe('猫咪书桌')
  expect(target.querySelector('.furniture-detail')?.textContent).toContain('维护耗材')
  expect(mocks.post).not.toHaveBeenCalled()
})

it('uses one dialog for details and confirmation, debits only after confirmation, and returns to updated details', async () => {
  await press('[data-furniture=bench]'); await command('安置')
  expect(target.querySelectorAll('[role=dialog]')).toHaveLength(1)
  expect(target.querySelector('.confirmation-body')).not.toBeNull()
  expect(mocks.post).not.toHaveBeenCalled()
  await command('返回详情', 'footer')
  expect(target.querySelector('.furniture-detail')).not.toBeNull()
  await command('安置')
  mocks.post.mockImplementation(async () => { state.revision++; state.items[0].installed = true; state.inventory.wood -= 100; return structuredClone(state) })
  await command('安置', 'footer')
  expect(mocks.post).toHaveBeenCalledTimes(1)
  expect(mocks.post.mock.calls[0][1]).toMatchObject({ action: 'install', furnitureId: 'bench', revision: 1 })
  expect(target.querySelector('.confirmation-body')).toBeNull()
  expect(target.querySelector('.furniture-detail')?.textContent).toContain('永久生效')
  expect(target.querySelector('[data-furniture=bench]')?.textContent).toContain('Lv.1 / 10')
})

it('does not reopen a dismissed dialog when its price refresh finishes', async () => {
  await press('[data-furniture=bench]')
  let resolve!: (state: FurnitureProjection) => void
  mocks.get.mockReturnValueOnce(new Promise(r => { resolve = r }))
  await command('安置'); await press('.close-dialog')
  resolve(structuredClone(state)); await settle()
  expect(target.querySelector('[role=dialog]')).toBeNull()
  expect(mocks.post).not.toHaveBeenCalled()
})

it('discards the old account detail and confirmation view on account change', async () => {
  await press('[data-furniture=bench]'); await command('安置')
  game.token = 'b'; await settle()
  expect(target.querySelector('[role=dialog]')).toBeNull()
  expect(mocks.post).not.toHaveBeenCalled()
})

it('shows scheduled activation and removes debit buttons while the furniture is pending', async () => {
  await press('[data-furniture=bench]'); await command('安置')
  mocks.post.mockImplementation(async () => {
    state.revision++; state.inventory.wood-=100
    state.items[0].pending={definition:state.items[0].definition,installed:true,remainingMs:0,effectiveAt:new Date(Date.now()+60000).toISOString()}
    return structuredClone(state)
  })
  await command('安置','footer')
  expect(target.querySelector('[data-furniture=bench]')?.textContent).toContain('等待生效')
  expect(target.querySelector('.pending-change')?.textContent).toContain('变更已提交')
  expect([...target.querySelectorAll('[role=dialog] button')].some(b=>b.textContent?.trim()==='安置')).toBe(false)
  expect(mocks.post).toHaveBeenCalledTimes(1)
})
