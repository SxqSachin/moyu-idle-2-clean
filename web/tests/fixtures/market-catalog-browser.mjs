// Install after installInstanceMarketFixture so no game request reaches the server.
export function installMarketCatalogFixture() {
  const baseFetch = window.fetch
  const specimens = ['snakehead', 'bluegill'].map((speciesId, index) => ({
    instanceId: 900 + index, templateItemId: 'fish_catch', quantity: 1, state: 'inventory',
    enhanceLevel: 0, affixes: [], baseSnapshot: {}, customData: {}, durability: 100, maxDurability: 100,
    fishData: { speciesId, lengthCm: index ? 29.4 : 90.6, weightKg: index ? .507 : 6.206, sex: 'female', lifeState: 'alive', origin: 'wild' }
  }))
  const groups = specimens.map((snapshot, index) => ({
    itemId: 'fish_catch', groupKey: String(index).repeat(64), catalogKey: String(index + 2).repeat(64),
    quantity: 1, sellerCount: 1, lowestPrice: 600, snapshot,
    fields: [
      { labelKey: 'game.instanceMarket.length', format: 'number', value: snapshot.fishData.lengthCm },
      { labelKey: 'game.instanceMarket.weight', format: 'number', value: snapshot.fishData.weightKg },
      { labelKey: 'game.instanceMarket.sex', format: 'text', value: 'female' }
    ]
  }))
  window.__catalogRequests = []
  window.fetch = async (input, options) => {
    const url = new URL(typeof input === 'string' ? input : input.url, location.href)
    if (!url.pathname.startsWith('/api/')) return baseFetch(input, options)
    window.__catalogRequests.push({ path: url.pathname, query: url.search })
    let data
    if (url.pathname === '/api/market/instances/groups') {
      const values = url.searchParams.get('catalogValues')
      data = { items: groups.filter(g => !values || JSON.stringify([g.snapshot.fishData.speciesId]) === values), hasMore: false }
    } else if (url.pathname === '/api/market/instances/listings') {
      const group = groups.find(g => g.groupKey === url.searchParams.get('groupKey'))
      data = { items: group ? [{ id: 11, isOwn: false, canBuy: true, itemId: group.itemId, groupKey: group.groupKey, price: 600, quantity: 1, initialQuantity: 1, fields: group.fields, status: 'active' }] : [], hasMore: false }
    } else if (url.pathname === '/api/inventory/instances') data = { items: specimens }
    else {
      const response = await baseFetch(input, options)
      if (url.pathname !== '/api/bootstrap/init') return response
      const payload = await response.json()
      payload.data.inventory = { items: specimens }
      data = payload.data
    }
    return new Response(JSON.stringify({ ok: true, ts: Date.now(), data }), { status: 200, headers: { 'Content-Type': 'application/json' } })
  }
}
