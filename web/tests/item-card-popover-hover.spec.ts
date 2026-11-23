import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"
import { createApp, h, nextTick, type App } from "vue"
import { createI18n } from "vue-i18n"
import { createPinia, setActivePinia } from "pinia"
import ElementPlus from "element-plus"

vi.mock("@components/game/item/ItemPopoverContent.vue", () => ({
  default: {
    name: "ItemPopoverContentStub",
    props: ["active"],
    template: `<div data-testid="item-popover-content" :data-active="active ? 'true' : 'false'"></div>`
  }
}))

vi.mock("@components/game/icon/TypeIcon.vue", () => ({
  default: {
    name: "TypeIconStub",
    props: ["alt"],
    template: `<span data-testid="type-icon">{{ alt }}</span>`
  }
}))

vi.mock("@iconify/vue", () => ({
  Icon: {
    name: "IconStub",
    template: `<span data-testid="icon"></span>`
  }
}))

const mountedApps: Array<{ app: App; container: HTMLElement }> = []
const gameDataModules = import.meta.glob("@game-data/chunks/*.json", { eager: true }) as Record<string, { default: unknown }>
const gameDataChunks: Record<string, unknown> = {}

for (const [path, module] of Object.entries(gameDataModules)) {
  const match = path.match(/chunks\/([^/]+)\.json$/)
  if (!match) continue
  gameDataChunks[match[1]] = module.default
}

beforeEach(async () => {
  vi.useFakeTimers()
  vi.setSystemTime(1_000)
  vi.resetModules()
  setHoverCapableDevice()
  await installGameDataForCurrentModules()
})

afterEach(() => {
  for (const view of mountedApps.splice(0)) {
    view.app.unmount()
    view.container.remove()
  }
  document.body.innerHTML = ""
  vi.useRealTimers()
})

function setHoverCapableDevice() {
  Object.defineProperty(window, "matchMedia", {
    configurable: true,
    writable: true,
    value: (query: string) => ({
      matches: query === "(hover: hover) and (pointer: fine)",
      media: query,
      onchange: null,
      addListener: () => {},
      removeListener: () => {},
      addEventListener: () => {},
      removeEventListener: () => {},
      dispatchEvent: () => false
    })
  })
  Object.defineProperty(navigator, "maxTouchPoints", {
    configurable: true,
    value: 0
  })
}

async function installGameDataForCurrentModules() {
  const { installGameDataForTest } = await import("../src/services/game-data-runtime")
  installGameDataForTest(gameDataChunks)
}

async function mountItemCard(overrides: Record<string, unknown> = {}) {
  const { default: ItemCard } = await import("../src/components/game/item/Card.vue")
  const pinia = createPinia()
  setActivePinia(pinia)
  const i18n = createI18n({
    legacy: false,
    locale: "zh-CN",
    missingWarn: false,
    fallbackWarn: false,
    messages: {
      "zh-CN": {
        game: {
          unnamedItem: "未知物品"
        }
      }
    }
  })

  const container = document.createElement("div")
  document.body.appendChild(container)
  const app = createApp({
    render: () => h(ItemCard, { itemId: "wood_sword", quantity: 1, ...overrides })
  })
  app.use(pinia)
  app.use(i18n)
  app.use(ElementPlus)
  app.mount(container)
  mountedApps.push({ app, container })
  await flushVue()

  const card = container.querySelector(".item-card")
  if (!(card instanceof HTMLElement)) {
    throw new Error("ItemCard reference element was not rendered")
  }
  return { card, container }
}

async function flushVue() {
  await nextTick()
  await Promise.resolve()
  await nextTick()
}

function activePopover() {
  return document.body.querySelector('[data-testid="item-popover-content"][data-active="true"]')
}

async function advancePopoverDelay() {
  vi.advanceTimersByTime(400)
  await flushVue()
}

function dispatchMouseEnter(element: HTMLElement) {
  element.dispatchEvent(new MouseEvent("mouseenter", { bubbles: false, clientX: 12, clientY: 12 }))
}

function dispatchMouseMove(element: HTMLElement) {
  element.dispatchEvent(new MouseEvent("mousemove", { bubbles: true, clientX: 16, clientY: 16 }))
}

describe("item card popover hover intent", () => {
  it("uses two-line names without changing the full label", async () => {
    const { container } = await mountItemCard()
    const label = container.querySelector(".item-card-name")

    expect(label?.classList.contains("item-card-name--two-lines")).toBe(true)
    expect(label?.textContent).toBe(container.querySelector('[data-testid="type-icon"]')?.textContent)
  })

  it("does not reserve label space when the label is hidden", async () => {
    const { container } = await mountItemCard({ hideLabel: true })

    expect(container.querySelector(".item-card-name")).toBeNull()
  })

  it("can render an explicit zero quantity for remaining-supply views", async () => {
    const { card } = await mountItemCard({ quantity: 0, showZeroQuantity: true })

    expect(card.textContent).toContain("0")
  })

  it("does not open from a scroll-induced mouseenter until the mouse actually moves", async () => {
    const { card } = await mountItemCard()

    window.dispatchEvent(new Event("scroll"))
    dispatchMouseEnter(card)
    await advancePopoverDelay()
    expect(activePopover()).toBeNull()

    dispatchMouseMove(card)
    await advancePopoverDelay()
    expect(activePopover()).not.toBeNull()
  })

  it("closes an open popover when an outer scroll happens", async () => {
    const { card } = await mountItemCard()

    dispatchMouseEnter(card)
    dispatchMouseMove(card)
    await advancePopoverDelay()
    expect(activePopover()).not.toBeNull()

    window.dispatchEvent(new Event("scroll"))
    await flushVue()
    expect(activePopover()).toBeNull()
  })

  it("cancels a pending popover when scrolling during the show delay", async () => {
    const { card } = await mountItemCard()

    dispatchMouseEnter(card)
    dispatchMouseMove(card)
    vi.advanceTimersByTime(200)
    window.dispatchEvent(new Event("scroll"))
    await advancePopoverDelay()
    expect(activePopover()).toBeNull()
  })

  it("keeps keyboard focus able to open the popover", async () => {
    const { card } = await mountItemCard()

    card.dispatchEvent(new FocusEvent("focusin", { bubbles: true }))
    await advancePopoverDelay()
    expect(activePopover()).not.toBeNull()
  })
})
