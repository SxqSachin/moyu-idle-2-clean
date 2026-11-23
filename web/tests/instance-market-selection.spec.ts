import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createApp, defineComponent, h, nextTick, ref, type App } from 'vue'
import Panel from '../src/views/game/market/components/InstanceMarketPanel.vue'

const mock = vi.hoisted(() => ({ state: {} as Record<string, any> }))
vi.mock('@/composables/game/market/useInstanceMarket', () => ({ useInstanceMarket: () => mock.state }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key, locale: ref('zh-CN') }) }))
vi.mock('@composables/game/useDevice', () => ({ useDevice: () => ({ isMobile: ref(false), supportsHover: ref(true), isTouchLike: ref(false) }) }))
vi.mock('@components/game/item/Card.vue', () => ({ default: defineComponent({ setup: () => () => h('div', { class: 'test-card' }, 'Item') }) }))
vi.mock('@components/game/item/QuickRef.vue', () => ({ default: defineComponent({ setup: () => () => h('span') }) }))
vi.mock('../src/views/game/market/components/InstanceMarketFields.vue', () => ({ default: defineComponent({ setup: () => () => h('div') }) }))
vi.mock('@components/game/common/GameDialog.vue', () => ({ default: defineComponent({ props: ['modelValue'], setup: (props, { slots }) => () => props.modelValue ? h('div', slots.default?.()) : null }) }))
vi.mock('@services/market-catalog', () => ({ marketCatalogForInstance: () => undefined, marketCatalogName: () => '' }))
vi.mock('@services/item-label', () => ({ getLocalizedItemName: () => 'Item' }))

let app: App
let root: HTMLDivElement
beforeEach(() => {
  const items = [1, 101].map(instanceId => ({ instanceId, templateItemId: 'test' }))
  mock.state = {
    view: ref('groups'), rows: ref([]), loading: ref(false), error: ref(''), pending: ref(null), busy: ref(false),
    listingOpen: ref(true), quoteGroups: ref([]), selectedIds: ref<number[]>([]), eligible: ref(items), visibleInventory: ref(items),
    offset: ref(0), hasMore: ref(false), details: ref([]), detailOpen: ref(false), buyTarget: ref(null),
    toggle: (id: number, selected: boolean) => { mock.state.selectedIds.value = selected ? [...mock.state.selectedIds.value, id] : mock.state.selectedIds.value.filter((value: number) => value !== id) }
  }
  root = document.createElement('div'); document.body.append(root)
  app = createApp(Panel, { itemId: 'test' })
  app.component('el-button', defineComponent({ setup: (_, { slots }) => () => h('button', slots.default?.()) }))
  const passthrough = defineComponent({ setup: (_, { slots }) => () => h('div', slots.default?.()) })
  for (const name of ['el-tooltip', 'el-alert', 'el-table-column', 'el-table', 'el-card', 'el-tag', 'el-option', 'el-select', 'el-input-number']) app.component(name, passthrough)
  app.directive('loading', {})
  app.mount(root)
})
afterEach(() => { app.unmount(); root.remove() })

describe('instance market card selection', () => {
  it('selects the entire card with click or keyboard, without a separate checkbox', async () => {
    const card = root.querySelector<HTMLElement>('[role="checkbox"]')!
    expect(root.querySelector('input[type="checkbox"]')).toBeNull()
    card.querySelector<HTMLElement>('.test-card')!.click()
    await nextTick()
    expect(card.getAttribute('aria-checked')).toBe('true')
    expect(card.classList.contains('batch-card-wrapper--selected')).toBe(true)
    expect(card.querySelector('.batch-card-badge')).not.toBeNull()
    card.dispatchEvent(new KeyboardEvent('keydown', { key: ' ', bubbles: true }))
    await nextTick()
    expect(card.getAttribute('aria-checked')).toBe('false')
    card.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }))
    await nextTick()
    expect(mock.state.selectedIds.value).toEqual([1])
  })
  it('blocks new selections at 100 but permits deselection, and freezes while busy or pending', async () => {
    mock.state.selectedIds.value = Array.from({ length: 100 }, (_, i) => i + 1)
    await nextTick()
    const [selected, extra] = [...root.querySelectorAll<HTMLElement>('[role="checkbox"]')]
    expect(extra.getAttribute('aria-disabled')).toBe('true')
    extra.click(); expect(mock.state.selectedIds.value).toHaveLength(100)
    selected.click(); await nextTick()
    expect(extra.getAttribute('aria-disabled')).toBe('false')
    extra.click(); await nextTick()
    expect(mock.state.selectedIds.value).toContain(101)
    mock.state.busy.value = true; await nextTick()
    extra.click(); expect(mock.state.selectedIds.value).toContain(101)
    mock.state.busy.value = false; mock.state.pending.value = {}; await nextTick()
    extra.click(); expect(mock.state.selectedIds.value).toContain(101)
  })
})
