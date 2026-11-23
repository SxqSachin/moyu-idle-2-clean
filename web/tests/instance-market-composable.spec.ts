import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"
import { createApp, defineComponent, h, nextTick, reactive, ref, type App } from "vue"
import { useInstanceMarket } from "@/composables/game/market/useInstanceMarket"
import type { MarketListing, MarketQuote } from "@services/instance-market"
vi.mock('@services/market-catalog', () => ({ matchesMarketCatalog: (_id: string, row: { family?: string }, values: string) => !values || row?.family === values }))

const mocks = vi.hoisted(() => ({ read: vi.fn(), write: vi.fn(), initialize: vi.fn(), success: vi.fn(), warning: vi.fn(), error: vi.fn(), game: {} as { token: string; userId: number } }))
vi.mock("@stores/game", () => ({ useGameStore: () => mocks.game }))
vi.mock("@stores/inventory", () => ({ useInventoryStore: () => ({ instances: [], initialize: mocks.initialize }) }))
vi.mock("vue-i18n", () => ({ useI18n: () => ({ t: (key: string) => key }) }))
vi.mock("element-plus", () => ({ ElMessage: { success: mocks.success, warning: mocks.warning, error: mocks.error } }))
vi.mock("@services/instance-market", async () => ({ ...await vi.importActual<object>("@services/instance-market"), marketRead: mocks.read, marketWrite: mocks.write }))

let app: App
let root: HTMLDivElement
const item = "creamfin_dual_fish_soup"
const listing = { id: 1, itemId: item, price: 500, quantity: 10, groupKey: "a".repeat(64), isOwn: false, canBuy: true, status: 'active', fields: [] } as unknown as MarketListing
const flush = async () => { for (let i = 0; i < 6; i++) await nextTick() }
function mountMarket() {
  const id = ref(item)
  const catalogValues = ref('')
  let market!: ReturnType<typeof useInstanceMarket>
  root = document.createElement("div")
  document.body.append(root)
  app = createApp(defineComponent({ setup() { market = useInstanceMarket(id, ref(false), ref('mine'), catalogValues); return () => h("div") } }))
  app.mount(root)
  return { market, id, catalogValues }
}
beforeEach(() => {
  vi.clearAllMocks()
  mocks.read.mockReset().mockResolvedValue({ items: [], hasMore: false })
  mocks.write.mockReset()
  mocks.initialize.mockReset().mockResolvedValue(undefined)
  mocks.game = reactive({ token: "account-a", userId: 42 })
})
afterEach(() => { app?.unmount(); root?.remove() })

describe("instance market mutation and refresh lifecycle", () => {
  it('passes catalog scope before pagination and discards responses from the previous catalog', async () => {
    const { market, catalogValues } = mountMarket()
    await flush()
    let finish!: (value: unknown) => void
    mocks.read.mockReturnValueOnce(new Promise(resolve => { finish = resolve }))
    catalogValues.value = '["one"]'
    await flush()
    expect(mocks.read.mock.lastCall?.[2]).toMatchObject({ catalogValues: '["one"]', offset: 0 })
    catalogValues.value = '["two"]'
    await flush()
    finish({ items: [{ snapshot: { family: '["one"]' } }], hasMore: true })
    await flush()
    expect(market.rows.value).toEqual([])
    expect(market.hasMore.value).toBe(false)
    expect(market.selectedGroup.value).toBeNull()
  })
  it("only opens the authoritative cheapest offer, including beyond the first page", async () => {
    const { market } = mountMarket()
    await flush()
    const later = { ...listing, id: 2, canBuy: false }
    const own = { ...listing, id: 3, isOwn: true, canBuy: false }
    market.view.value = 'listings'; market.rows.value = [own, listing, later]
    market.openBuy(later)
    expect(market.buyTarget.value).toBeNull()
    market.openBuy(own)
    expect(market.buyTarget.value).toBeNull()
    market.offset.value = 30
    market.buyFirst()
    expect(market.buyTarget.value?.id).toBe(listing.id)
  })
  it("retries an uncertain purchase with its exact original payload and key", async () => {
    const { market } = mountMarket()
    await flush()
    market.view.value = 'listings'; await flush(); market.rows.value = [listing]
    market.openBuy(listing)
    market.buyQuantity.value = 2
    mocks.write.mockRejectedValueOnce(new Error("connection lost"))
    await market.confirmBuy()
    const first = mocks.write.mock.calls[0]
    expect(market.pending.value).not.toBeNull()
    market.buyQuantity.value = 4
    mocks.write.mockResolvedValueOnce({ orderId: 7 })
    await market.confirmBuy()
    expect(mocks.write.mock.calls[1]).toEqual(first)
    expect(first[2].quantity).toBe(2)
    expect(market.pending.value).toBeNull()
    expect(mocks.success).toHaveBeenCalledOnce()
  })

  it("does not turn a committed purchase into a failed or repeatable write when refresh fails", async () => {
    const { market } = mountMarket()
    await flush()
    market.view.value = 'listings'; await flush(); market.rows.value = [listing]
    market.openBuy(listing)
    mocks.write.mockResolvedValue({ orderId: 7 })
    mocks.read.mockRejectedValueOnce(new Error("read unavailable"))
    await market.confirmBuy()
    expect(mocks.success).toHaveBeenCalledOnce()
    expect(mocks.error).not.toHaveBeenCalled()
    expect(mocks.warning).toHaveBeenCalledWith("game.instanceMarket.refreshFailed")
    expect(market.pending.value).toBeNull()
    expect(market.buyTarget.value).toBeNull()
    expect(mocks.write).toHaveBeenCalledOnce()
  })

  it("discards a quote after switching item or account", async () => {
    const { market, id } = mountMarket()
    await flush()
    let resolve!: (value: { items: MarketQuote[] }) => void
    mocks.write.mockReturnValueOnce(new Promise(r => { resolve = r }))
    market.selectedIds.value = [1]
    const request = market.preview()
    id.value = "fish_catch"
    await flush()
    resolve({ items: [{ instanceId: 1, itemId: item, groupKey: "old" }] as MarketQuote[] })
    await request
    expect(market.quoteGroups.value).toEqual([])
    expect(market.busy.value).toBe(false)

    market.view.value = 'listings'; await flush(); market.rows.value = [listing]
    market.openBuy(listing)
    let finish!: (value: unknown) => void
    mocks.write.mockReturnValueOnce(new Promise(r => { finish = r }))
    const purchase = market.confirmBuy()
    mocks.game.token = "account-b"; mocks.game.userId = 43
    await flush()
    finish({ orderId: 1 }); await purchase
    expect(mocks.success).not.toHaveBeenCalled()
    expect(mocks.initialize).not.toHaveBeenCalled()
    expect(market.pending.value).toBeNull()
  })

  it("keeps cancellation retries tied to the original listing", async () => {
    const { market } = mountMarket()
    await flush()
    mocks.write.mockRejectedValueOnce(new Error("connection lost"))
    await market.cancelListing(listing)
    await market.cancelListing({ ...listing, id: 2 })
    expect(mocks.write).toHaveBeenCalledOnce()
    mocks.write.mockResolvedValueOnce({ ok: true })
    await market.retryPending()
    expect(mocks.write.mock.calls[1]).toEqual(mocks.write.mock.calls[0])
  })
})
