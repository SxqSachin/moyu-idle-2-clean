import { expect, test, type Page } from "@playwright/test"

const inventoryItems = [
  {
    instanceId: 701,
    templateItemId: "iron_sword",
    customData: { name: "暮雪", labelColor: "sky" },
    quantity: 1,
    state: "inventory",
    enhanceLevel: 4,
    durability: 72,
    maxDurability: 100,
    baseSnapshot: { physics_attack: 18 },
    affixes: []
  },
  {
    instanceId: 702,
    templateItemId: "iron_ore",
    quantity: 36,
    state: "inventory",
    enhanceLevel: 0,
    durability: 0,
    maxDurability: 0,
    affixes: []
  },
  {
    instanceId: 703,
    templateItemId: "fish_catch",
    quantity: 1,
    state: "inventory",
    enhanceLevel: 0,
    durability: 0,
    maxDurability: 0,
    affixes: [],
    fishData: {
      speciesId: "common_carp",
      ageDays: 730,
      sex: "female",
      lengthCm: 42.5,
      weightKg: 1.85,
      condition: 0.9,
      health: 1,
      waterId: "sunmere_shallows",
      spotId: "old_willow",
      caughtAt: "2026-09-03T08:00:00Z",
      rulesetVersion: "fishing-v1"
    }
  }
]

const chatHistory = [
  {
    id: 801,
    channel: "world",
    userId: 9,
    username: "山外来客",
    content: "找的是这种矿石",
    ts: Date.UTC(2026, 8, 3, 1, 5),
    attachment: { type: "item", itemId: "iron_ore" }
  },
  {
    id: 802,
    channel: "world",
    userId: 10,
    username: "铸剑师",
    content: "看看这把",
    ts: Date.UTC(2026, 8, 3, 1, 6),
    attachment: {
      type: "item_instance",
      itemId: "iron_sword",
      instanceId: 812,
      quantity: 1,
      preview: {
        customData: { name: "霜痕", labelColor: "sky" },
        enhanceLevel: 6,
        durability: 86,
        maxDurability: 100,
        baseSnapshot: { physics_attack: 24 },
        affixes: []
      }
    }
  },
  {
    id: 803,
    channel: "world",
    userId: 11,
    username: "樵夫",
    content: "一起去？",
    ts: Date.UTC(2026, 8, 3, 1, 7),
    attachment: { type: "action", actionId: "logging" }
  }
]

async function mockChat(page: Page, onSend: (payload: Record<string, unknown>) => void) {
  await page.addInitScript(() => {
    localStorage.setItem("locale", "zh-CN")
    localStorage.setItem("token", "e2e-token")
    localStorage.setItem("userId", "42")
    localStorage.setItem("username", "聊天测试员")
    localStorage.setItem("player-settings:42", JSON.stringify({
      language: "zh-CN",
      theme: "light",
      menuLayout: "icon-rail",
      simpleMode: true,
      emailVerified: true
    }))
  })

  await page.routeWebSocket(/\/ws(?:\?|$)/, (socket) => {
    socket.onMessage((message) => {
      if (typeof message !== "string") return
      const event = JSON.parse(message) as Record<string, unknown>
      if (event.type === "auth.bind") {
        socket.send(JSON.stringify({ type: "auth.bound", ts: Date.now(), data: {} }))
      }
      if (event.type === "action.offline.rewards.get") {
        socket.send(JSON.stringify({ type: "action.offline.rewards.result", ts: Date.now(), data: { hasMoreDue: false, rewards: [] } }))
      }
      if (event.type === "chat.send") onSend(event)
    })
  })

  await page.route("**/api/**", async (route) => {
    const path = decodeURIComponent(new URL(route.request().url()).pathname)
    const fulfill = (data: unknown) => route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ ok: true, ts: Date.now(), data })
    })
    if (path === "/api/bootstrap/init") {
      return fulfill({
        playerStats: {},
        inventory: { items: inventoryItems },
        chatConfig: {
          maxContentLength: 300,
          defaultHistoryLimit: 30,
          defaultChannel: "world",
          channels: [{ id: "world", nameKey: "chat.channel.world", order: 10, playerSendable: true }]
        },
        chatHistory: { channel: "world", items: chatHistory, hasMore: false },
        playerSettings: {
          userId: 42,
          username: "聊天测试员",
          language: "zh-CN",
          theme: "light",
          menuLayout: "icon-rail",
          simpleMode: true,
          emailVerified: true
        },
        tutorialProgress: {},
        playerPresence: { status: "online" },
        systemVersion: {},
        recycleConfig: { goldCoefficient: 0.5, affixMismatchExtraCoefficient: 0.6 }
      })
    }
    if (path === "/api/inventory/instances") return fulfill({ items: inventoryItems })
    if (path === "/api/chat/config") {
      return fulfill({
        defaultChannel: "world",
        channels: [{ id: "world", nameKey: "chat.channel.world", order: 10, playerSendable: true }]
      })
    }
    if (path === "/api/chat/messages") return fulfill({ channel: "world", items: chatHistory, hasMore: false })
    return fulfill({})
  })
}

async function openChat(page: Page) {
  const notice = page.locator(".el-notification").filter({ hasText: "测试版本提示" })
  await notice.waitFor({ state: "visible", timeout: 5_000 }).catch(() => undefined)
  if (await notice.isVisible()) await notice.locator(".el-notification__closeBtn").click()
  await page.getByRole("button", { name: "频道聊天" }).click()
  await expect(page.getByText("频道聊天", { exact: true })).toBeVisible()
}

function itemChoice(page: Page, name: string) {
  return page.getByTestId("chat-attachment-picker")
    .locator(".labeled-frame-card", { hasText: name })
    .locator(".item-card")
    .first()
}

test("desktop chat inserts catalog items, inventory instances, and actions", async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 900 })
  let sent: Record<string, unknown> | null = null
  await mockChat(page, (payload) => { sent = payload })

  await page.goto("/game/home")
  await openChat(page)
  await expect(page.getByText("图鉴物品", { exact: true })).toBeVisible()
  await expect(page.getByText("背包物品 · +6", { exact: true })).toBeVisible()
  await expect(page.getByText("霜痕", { exact: true })).toHaveClass(/item-custom-label--sky/)
  await expect(page.getByText("霜痕", { exact: true })).toHaveCSS("color", "rgb(3, 105, 161)")
  await expect(page.locator(".chat-attachment-card", { hasText: "耐久" })).toHaveCount(0)
  await expect(page.getByText("动作 · 采集", { exact: true })).toBeVisible()

  await page.getByTestId("chat-insert-trigger").click()
  const picker = page.getByTestId("chat-attachment-picker")
  await expect(picker).toBeVisible()
  await picker.getByPlaceholder("输入名称搜索").first().fill("铁矿石")
  await expect(itemChoice(page, "铁矿石")).toBeVisible()
  await page.screenshot({ path: "../output/playwright/chat-attachments-desktop.png", fullPage: true })
  await itemChoice(page, "铁矿石").click()
  await expect(page.locator(".chat-composer").getByText("铁矿石", { exact: true })).toBeVisible()

  await page.getByTestId("chat-insert-trigger").click()
  await picker.getByRole("tab", { name: "我的背包" }).click()
  await itemChoice(page, "暮雪").click()
  const pending = page.locator(".chat-composer > .chat-attachment-card")
  await expect(pending.getByText("暮雪", { exact: true })).toBeVisible()
  await expect(pending).toContainText("背包物品 · +4")
  await expect(pending).not.toContainText("耐久")
  await expect(pending.getByText("暮雪", { exact: true })).toHaveClass(/item-custom-label--sky/)
  await expect(pending.getByText("暮雪", { exact: true })).toHaveCSS("color", "rgb(3, 105, 161)")

  await page.getByTestId("chat-insert-trigger").click()
  await picker.getByRole("tab", { name: "渔获" }).click()
  await expect(itemChoice(page, "鲤鱼")).toBeVisible()
  await itemChoice(page, "鲤鱼").click()
  await expect(pending.getByText("鲤鱼", { exact: true })).toBeVisible()

  await page.getByTestId("chat-insert-trigger").click()
  await picker.getByRole("tab", { name: "动作" }).click()
  await picker.getByPlaceholder("搜索动作名称").fill("伐木")
  await picker.locator(".labeled-frame-card", { hasText: "伐木" }).locator(".action-card").click()
  await expect(pending.getByText("伐木", { exact: true })).toBeVisible()

  await page.getByRole("button", { name: "发送" }).click()
  await expect.poll(() => sent).not.toBeNull()
  expect(sent).toMatchObject({
    type: "chat.send",
    payload: { channel: "world", content: "", attachment: { type: "action", actionId: "logging" } }
  })
})

test("mobile chat uses a bottom drawer and 44px composer targets", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 })
  await mockChat(page, () => undefined)

  await page.goto("/game/home")
  await openChat(page)
  await page.getByTestId("chat-insert-trigger").click()
  const picker = page.getByTestId("chat-attachment-picker")
  await expect(picker).toBeVisible()
  await picker.getByRole("tab", { name: "我的背包" }).click()
  await picker.getByRole("tab", { name: "渔获" }).click()
  await expect(itemChoice(page, "鲤鱼")).toBeVisible()

  const drawerBox = await page.locator(".chat-attachment-drawer").boundingBox()
  expect(drawerBox).not.toBeNull()
  expect(drawerBox!.x).toBeGreaterThanOrEqual(-1)
  expect(drawerBox!.x + drawerBox!.width).toBeLessThanOrEqual(391)
  await page.screenshot({ path: "../output/playwright/chat-attachments-mobile.png", fullPage: true })

  await itemChoice(page, "鲤鱼").click()
  await expect(picker).toBeHidden()
  const pending = page.locator(".chat-composer > .chat-attachment-card")
  await expect(pending.getByText("鲤鱼", { exact: true })).toBeVisible()
  await expect(pending).toContainText("背包物品")
  await expect(pending).not.toContainText("耐久")

  const insertBox = await page.getByTestId("chat-insert-trigger").boundingBox()
  const sendBox = await page.getByRole("button", { name: "发送" }).boundingBox()
  expect(insertBox?.height).toBeGreaterThanOrEqual(44)
  expect(sendBox?.height).toBeGreaterThanOrEqual(44)
  const pageWidth = await page.evaluate(() => document.documentElement.scrollWidth)
  expect(pageWidth).toBeLessThanOrEqual(390)
  await page.screenshot({ path: "../output/playwright/chat-attachments-mobile-selected.png", fullPage: true })
})
