import { afterEach, describe, expect, it, vi } from "vitest"
import { createApp, h, nextTick, reactive, type App } from "vue"
import { createPinia, setActivePinia } from "pinia"
import { createI18n } from "vue-i18n"
import ElementPlus from "element-plus"
import { useGameStore } from "../src/stores/game"
import { useRoleStateStore } from "../src/stores/role-state"
import ItemDetailPanel from "../src/components/game/item/ItemDetailPanel.vue"

vi.hoisted(() => {
  Object.defineProperty(window, "matchMedia", { configurable: true, value: () => ({
    matches: false, addEventListener() {}, removeEventListener() {}
  }) })
})

vi.mock("@services/api", async (importOriginal) => ({
  ...await importOriginal<typeof import("../src/services/api")>(),
  apiGet: vi.fn(async () => ({ item: { id: "silkwood_search_rod", type: ["equipment"] }, operations: {} }))
}))
vi.mock("@components/game/item/ItemPopoverContent.vue", () => ({
  default: {
    props: ["requirementAttributes"],
    template: '<div data-testid="requirements">{{ JSON.stringify(requirementAttributes) }}</div>'
  }
}))
vi.mock("vue-router", () => ({ useRouter: () => ({ push: vi.fn() }) }))

let app: App | undefined
let container: HTMLElement
afterEach(() => {
  app?.unmount()
  container?.remove()
})

async function flush() {
  await Promise.resolve()
  await nextTick()
  await Promise.resolve()
  await nextTick()
}

describe("item detail target requirements", () => {
  it("renders the target kitty's attributes, reacts to switching and preserves explicit overrides", async () => {
    const pinia = createPinia()
    setActivePinia(pinia)
    useGameStore().token = "test-token"
    const roles = useRoleStateStore()
    function patch(roleType: "kitty" | "player", roleId: number | string, fishing: number, version = 1) {
      roles.patchRoleAttributesPayload({ roleType, roleId, upsert: {
        fishing: { value: fishing, category: "work", unit: "number", growthType: "levelable", order: 1 }
      }, meta: { version } })
    }
    patch("player", "self", 1)
    patch("kitty", 9, 18)
    patch("kitty", 10, 3)
    const props = reactive<{
      itemId: string
      operationTargetType: "kitty" | "player"
      operationTargetKittyId: number
      requirementAttributes: Record<string, number> | null
    }>({ itemId: "silkwood_search_rod", operationTargetType: "kitty", operationTargetKittyId: 9, requirementAttributes: null })
    container = document.createElement("div")
    document.body.appendChild(container)
    app = createApp({ render: () => h(ItemDetailPanel, props) })
    app.use(pinia)
    app.use(createI18n({ legacy: false, locale: "en-US", missingWarn: false, fallbackWarn: false, messages: { "en-US": {} } }))
    app.use(ElementPlus)
    app.mount(container)
    await flush()
    const displayed = () => JSON.parse(container.querySelector('[data-testid="requirements"]')?.textContent || "null")
    expect(displayed()).toEqual({ fishing: 18 })
    props.operationTargetKittyId = 10
    await flush()
    expect(displayed()).toEqual({ fishing: 3 })
    patch("kitty", 10, 24, 2)
    await flush()
    expect(displayed()).toEqual({ fishing: 24 })
    props.operationTargetKittyId = 11
    await flush()
    expect(displayed()).toEqual({})
    props.operationTargetType = "player"
    await flush()
    expect(displayed()).toEqual({ fishing: 1 })
    props.requirementAttributes = { fishing: 45 }
    await flush()
    expect(displayed()).toEqual({ fishing: 45 })
  })
})
