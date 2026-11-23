import { createApp, h, nextTick } from 'vue'
import { expect, it, vi } from 'vitest'
import FurnitureCosts from '../src/components/game/panel/cattery/FurnitureCosts.vue'

vi.mock('../src/components/game/item/QuickRefWithRequire.vue', () => ({ default: {
  props: ['itemId','requiredCount','availableCount'],
  setup: (props: {itemId:string;requiredCount:number;availableCount:number}) => () => h('span', {'data-item':props.itemId}, `${props.requiredCount}(${props.availableCount})`),
} }))

it('keeps requirement and inventory in one reference with no separate shortage row', async () => {
  const target = document.createElement('div'); document.body.append(target)
  const app = createApp(FurnitureCosts, {costs:{iron:80,wood:120},inventory:{iron:7,wood:68}})
  try {
    app.mount(target); await nextTick()
    expect(target.textContent).toBe('80(7)120(68)')
    expect(target.querySelectorAll('.cost-entry')).toHaveLength(2)
    expect(target.querySelector('small,.shortage')).toBeNull()
  } finally { app.unmount(); target.remove() }
})
