// Isolated browser fixture. Never forwards game API requests or game sockets.
export function installInstanceMarketFixture(definition) {
  localStorage.setItem("locale", "zh-CN")
  localStorage.setItem("token", "isolated-market-ui-test")
  localStorage.setItem("userId", "42")
  localStorage.setItem("username", "Market QA")
  const settings = { language: "zh-CN", theme: "light", menuLayout: "icon-rail", simpleMode: true, emailVerified: true }
  localStorage.setItem("player-settings:42", JSON.stringify(settings))
  window.__marketRequests = []
  const itemId = "creamfin_dual_fish_soup"
  const fields = (rate) => [{ labelKey: "game.instanceMarket.effect", format: "operation_effect", value: { ...definition, params: { bonusRate: rate }, lifetime: { kind: "duration", durationSeconds: 3600 } } }]
  const snapshot = (rate) => ({ customData: {}, enhanceLevel: 0, affixes: [], durability: 100, maxDurability: 100, baseSnapshot: { operationEffect: { schemaVersion: 2, definitionId: definition.id, rulesetVersion: definition.rulesetVersion, params: { bonusRate: rate }, lifetime: { kind: "duration", durationSeconds: 3600 } } } })
  const inv = [801, 802, 803].map(instanceId => ({ instanceId, templateItemId: itemId, quantity: 1, state: "inventory", ...snapshot(instanceId === 803 ? .108 : .104) }))
  const listings = [
    { id: 1, sellerId: 99, sellerName: "Player A", itemId, groupKey: "a".repeat(64), price: 500, quantity: 10, initialQuantity: 10, status: "active", fields: fields(.104) },
    { id: 2, sellerId: 98, sellerName: "Player B", itemId, groupKey: "a".repeat(64), price: 520, quantity: 6, initialQuantity: 6, status: "active", fields: fields(.104) },
    { id: 3, sellerId: 98, sellerName: "Player B", itemId, groupKey: "b".repeat(64), price: 600, quantity: 3, initialQuantity: 3, status: "active", fields: fields(.108) }
  ]
  const originalFetch = window.fetch.bind(window)
  window.fetch = async (input, options = {}) => {
    const url = new URL(typeof input === "string" ? input : input.url, location.href)
    if (!url.pathname.startsWith("/api/")) return originalFetch(input, options)
    const path = url.pathname
    const body = options.body ? JSON.parse(options.body) : {}
    window.__marketRequests.push({ path, body, headers: options.headers })
    let data = { items: [], hasMore: false }
    if (path === "/api/bootstrap/init") data = {
      playerStats: { stats: {}, attributes: {}, buildings: { cattery: 2 }, kitties: [] }, inventory: { items: inv },
      chatConfig: { channels: [], defaultChannel: "world" }, chatHistory: { items: [] }, playerSettings: { username: "Market QA", ...settings },
      tutorialProgress: {}, playerPresence: { status: "online" }, systemVersion: {}, recycleConfig: { goldCoefficient: .5, affixMismatchExtraCoefficient: .6 }
    }
    if (path === "/api/player/stats") data = { stats: {}, attributes: {}, buildings: { cattery: 2 }, kitties: [] }
    if (path === "/api/inventory/instances") data = { items: inv.filter(i => i.state === "inventory") }
    if (path === "/api/market/snapshot") data = { myListings: [], myBuyOrders: [], publicListings: [], publicBuyOrders: [] }
    if (path === "/api/market/instances/groups") {
      const groups = new Map()
      for (const l of listings.filter(l => l.status === "active")) {
        let g = groups.get(l.groupKey)
        if (!g) { g = { itemId, groupKey: l.groupKey, catalogKey: "c".repeat(64), quantity: 0, sellerCount: 0, lowestPrice: l.price, fields: l.fields }; groups.set(l.groupKey, g) }
        g.quantity += l.quantity; g.sellerCount++; g.lowestPrice = Math.min(g.lowestPrice, l.price)
      }
      data = { items: [...groups.values()], hasMore: false }
    }
    if (path === "/api/market/instances/listings" || path === "/api/market/instances/mine") {
      const mine = path.endsWith('/mine')
      const offers = listings.filter(l => mine ? l.sellerId === 42 : l.groupKey === url.searchParams.get("groupKey") && l.status === "active").sort((a,b) => a.price-b.price || a.id-b.id)
      const best = offers.find(l => l.sellerId !== 42)
      data = { items: offers.map(({sellerId,sellerName,...l}) => ({...l,isOwn:sellerId===42,canBuy:!mine && l.id===best?.id})), hasMore: false }
    }
    if (path === "/api/market/instances/quote") data = { items: body.items.map(({ instanceId }) => ({ instanceId, itemId, groupKey: (instanceId === 803 ? "b" : "a").repeat(64), quoteToken: String(instanceId).padStart(64, "0"), fields: fields(instanceId === 803 ? .108 : .104), snapshot: inv.find(i => i.instanceId === instanceId) })) }
    if (path === "/api/market/instances/list") {
      listings.push({ id: 4, sellerId: 42, sellerName: "Market QA", itemId, groupKey: "a".repeat(64), price: body.price, quantity: body.items.length, initialQuantity: body.items.length, status: "active", fields: fields(.104) })
      for (const entry of body.items) inv.find(i => i.instanceId === entry.instanceId).state = "market"
      data = { listingId: 4 }
    }
    if (path === "/api/market/instances/buy") {
      const l = listings.find(l => l.id === body.listingId); l.quantity -= body.quantity
      if (!l.quantity) l.status = "sold"
      data = { orderId: 1, created: true }
    }
    if (path === "/api/market/instances/cancel") { const l = listings.find(l => l.id === body.listingId); l.quantity = 0; l.status = "cancelled"; data = { ok: true } }
    if (path === "/api/market/instances/details") data = { items: [{ instanceId: 901, state: "listed", snapshot: snapshot(.104) }] }
    return new Response(JSON.stringify({ ok: true, ts: Date.now(), data }), { status: 200, headers: { "Content-Type": "application/json" } })
  }
  const WS = window.WebSocket
  window.WebSocket = class extends EventTarget {
    static OPEN = 1
    static CLOSED = 3
    constructor(url, protocol) {
      super()
      if (!new URL(url, location.href).pathname.startsWith("/ws")) return new WS(url, protocol)
      this.readyState = 1
      setTimeout(() => { const e = new Event("open"); this.dispatchEvent(e); this.onopen?.(e) }, 50)
    }
    send(raw) {
      const event = JSON.parse(raw)
      const responses = { "auth.bind": ["auth.bound", {}], "action.queue.get": ["action.queue.updated", { items: [] }], "action.offline.rewards.get": ["action.offline.rewards.result", { hasMoreDue: false, rewards: [] }] }
      const entry = responses[event.type]
      if (entry) setTimeout(() => { const e = new MessageEvent("message", { data: JSON.stringify({ type: entry[0], data: entry[1], ts: Date.now() }) }); this.dispatchEvent(e); this.onmessage?.(e) }, 5)
    }
    close() { this.readyState = 3 }
  }
}
