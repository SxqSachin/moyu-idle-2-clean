import { expect, test, type Page } from "@playwright/test"

const inventoryItems = [
  {
    instanceId: 601,
    templateItemId: "engrave_stone",
    quantity: 2,
    state: "inventory",
    enhanceLevel: 0,
    durability: 0,
    maxDurability: 0,
    affixes: []
  },
  {
    instanceId: 602,
    templateItemId: "scaleglow_medium",
    quantity: 4,
    state: "inventory",
    enhanceLevel: 0,
    durability: 0,
    maxDurability: 0,
    affixes: []
  },
  {
    instanceId: 603,
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
      lengthCm: 100,
      weightKg: 5,
      condition: 1.05,
      health: 0.98,
      waterId: "sunmere_shallows",
      spotId: "old_willow",
      caughtAt: "2026-09-01T06:30:00Z",
      rulesetVersion: "fishing-v1",
      genes: { mutations: ["prismatic", "rainbow"] }
    }
  },
  {
    instanceId: 605,
    templateItemId: "fish_catch",
    quantity: 1,
    state: "inventory",
    enhanceLevel: 0,
    durability: 0,
    maxDurability: 0,
    affixes: [],
    fishData: {
      speciesId: "common_carp",
      ageDays: 610,
      sex: "male",
      lengthCm: 82,
      weightKg: 3.8,
      condition: 0.98,
      health: 0.96,
      waterId: "sunmere_shallows",
      spotId: "sunmere_old_pier",
      caughtAt: "2026-09-01T06:20:00Z",
      rulesetVersion: "fishing-v1"
    }
  },
  {
    instanceId: 606,
    templateItemId: "fish_catch",
    quantity: 1,
    state: "inventory",
    enhanceLevel: 0,
    durability: 0,
    maxDurability: 0,
    affixes: [],
    fishData: {
      speciesId: "common_carp",
      ageDays: 540,
      sex: "female",
      lengthCm: 74,
      weightKg: 2.9,
      condition: 1.02,
      health: 0.97,
      waterId: "sunmere_shallows",
      spotId: "sunmere_reed_edge",
      caughtAt: "2026-09-01T06:10:00Z",
      rulesetVersion: "fishing-v1"
    }
  },
  {
    instanceId: 607,
    templateItemId: "fish_catch",
    quantity: 1,
    state: "inventory",
    enhanceLevel: 0,
    durability: 0,
    maxDurability: 0,
    affixes: [],
    fishData: {
      speciesId: "common_carp",
      ageDays: 490,
      sex: "male",
      lengthCm: 69,
      weightKg: 2.4,
      condition: 0.95,
      health: 0.93,
      waterId: "sunmere_shallows",
      spotId: "sunmere_south_bank",
      caughtAt: "2026-09-01T06:00:00Z",
      rulesetVersion: "fishing-v1"
    }
  },
  {
    instanceId: 604,
    templateItemId: "gold_coin",
    quantity: 100000,
    state: "inventory",
    enhanceLevel: 0,
    durability: 0,
    maxDurability: 0,
    affixes: []
  }
]

async function mockAlchemy(page: Page, onAction: (payload: Record<string, unknown>) => void) {
  await page.addInitScript(() => {
    localStorage.setItem("locale", "zh-CN")
    localStorage.setItem("token", "e2e-token")
    localStorage.setItem("userId", "42")
    localStorage.setItem("username", "灌注测试员")
    localStorage.setItem("player-settings:42", JSON.stringify({ language: "zh-CN", theme: "light", menuLayout: "icon-rail", simpleMode: true, emailVerified: true }))
  })
  await page.routeWebSocket(/\/ws(?:\?|$)/, (socket) => {
    socket.onMessage((message) => {
      if (typeof message !== "string") return
      const event = JSON.parse(message) as Record<string, unknown>
      if (event.type === "auth.bind") socket.send(JSON.stringify({ type: "auth.bound", ts: Date.now(), data: {} }))
      if (event.type === "action.offline.rewards.get") socket.send(JSON.stringify({ type: "action.offline.rewards.result", ts: Date.now(), data: { hasMoreDue: false, rewards: [] } }))
      if (event.type === "action.enqueue") onAction(event)
    })
  })
  await page.route("**/api/**", async (route) => {
    const path = decodeURIComponent(new URL(route.request().url()).pathname)
    const fulfill = (data: unknown) => route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ ok: true, ts: Date.now(), data }) })
    if (path === "/api/bootstrap/init") return fulfill({
      playerStats: {},
      inventory: { items: inventoryItems },
      chatConfig: { channels: [], defaultChannel: "world" },
      chatHistory: { items: [] },
      playerSettings: { username: "灌注测试员", language: "zh-CN", theme: "light", menuLayout: "icon-rail", simpleMode: true, emailVerified: true },
      tutorialProgress: {},
      playerPresence: { status: "online" },
      systemVersion: {},
      recycleConfig: { goldCoefficient: 0.5, affixMismatchExtraCoefficient: 0.6 }
    })
    if (path === "/api/inventory/instances") return fulfill({ items: inventoryItems })
    if (path === "/api/alchemy/discoveries") return fulfill({ groups: [], traits: [], combos: [] })
    if (path === "/api/alchemy/bench/fish-preview") return fulfill({
      fishInstanceId: 603,
      speciesId: "common_carp",
      traitKind: "combat",
      attribute: "physics_damage",
      lengthCm: 100,
      weightKg: 5,
      power: 231.25,
      mutations: ["prismatic", "rainbow"],
      mutationPowerMultiplier: 1.85,
      mutationBonusAttributes: [
        { attribute: "alchemy_precision", mutationIds: ["prismatic"], baseRange: { min: 0.018, max: 0.032 }, effectiveRange: { min: 0.018, max: 0.032 }, failureFloor: 0.018 },
        { attribute: "alchemy_lucky_chance", mutationIds: ["rainbow"], baseRange: { min: 0.012, max: 0.025 }, effectiveRange: { min: 0.012, max: 0.025 }, failureFloor: 0.012 }
      ],
      baseRange: { min: 150.31, max: 231.25 },
      effectiveRange: { min: 150.31, max: 231.25 },
      failureFloor: 150.31,
      failChance: 0.15,
      goldCost: 200,
      mediumItemId: "scaleglow_medium",
      mediumCount: 1,
      rulesetVersion: "fish-infusion-v3"
    })
    return fulfill({})
  })
}

async function dismissBuildNotice(page: Page) {
  const notice = page.locator(".el-notification").filter({ hasText: "测试版本提示" })
  await notice.waitFor({ state: "visible", timeout: 5_000 }).catch(() => undefined)
  if (await notice.isVisible()) await notice.locator(".el-notification__closeBtn").click()
}

test("fish infusion previews up to four fish while submission stays closed", async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 900 })
  let submittedAction: Record<string, unknown> | null = null
  await mockAlchemy(page, (payload) => { submittedAction = payload })

  await page.goto("/game/alchemy?tab=bench")
  await dismissBuildNotice(page)
  await page.getByText("鱼类", { exact: true }).click()
  await expect(page.getByText("鳞辉灵媒", { exact: true })).toBeVisible()
  await expect(page.getByText("每次使用 1 枚", { exact: true })).toBeVisible()
  await expect(page.locator(".alchemy-submit__planner")).toHaveCount(0)

  await page.locator('[data-tutorial="alchemy-bench-carrier"] .item-card').click()
  await expect(page.getByRole("dialog", { name: "选择载体" })).toBeVisible()
  await page.locator(".inventory-instance-selector-dialog .item-card").first().click()

  await expect(page.locator(".alchemy-fish-choice")).toHaveCount(4)
  await page.getByRole("button", { name: /选择鱼类素材/ }).first().click()
  const fishDialog = page.getByRole("dialog", { name: "选择鱼类素材" })
  await expect(fishDialog).toBeVisible()
  await expect(fishDialog.locator(".alchemy-fish-picker button")).toHaveCount(4)
  const largestFish = fishDialog.locator(".alchemy-fish-picker button").first()
  await expect(largestFish.locator('[data-fish-size-metric="length"]')).toHaveAttribute("data-arrow-count", "3")
  await expect(largestFish.locator('[data-fish-size-metric="weight"]')).toHaveAttribute("data-arrow-count", "3")
  for (const fish of await fishDialog.locator(".alchemy-fish-picker button").all()) await fish.click()
  await expect(fishDialog.getByText("已选 4 / 4 尾")).toBeVisible()
  await fishDialog.getByRole("button", { name: "确认" }).click()
  await expect(fishDialog).toBeHidden()

  await expect(page.getByText("物理攻击", { exact: true }).first()).toBeVisible()
  await expect(page.getByText("150 ~ 231", { exact: false }).first()).toBeVisible()
  await expect(page.locator(".alchemy-fish-preview__mutations").first().getByText("棱彩", { exact: true })).toBeVisible()
  await expect(page.locator(".alchemy-fish-preview__mutations").first().getByText("彩虹", { exact: true })).toBeVisible()
  await expect(page.locator(".alchemy-fish-preview__values").first()).toContainText("异变增幅")
  await expect(page.locator(".alchemy-fish-preview__values").first()).toContainText("+85%")
  await expect(page.locator(".alchemy-fish-preview__values").first()).toContainText("炼金精度")
  await expect(page.locator(".alchemy-fish-preview__values").first()).toContainText("炼金幸运")
  await expect(page.locator(".alchemy-fish-choice .has-stacked-fish-mutations").first()).toBeVisible()
  await expect(page.locator(".alchemy-fish-preview__values")).toHaveCount(4)
  await expect(page.getByText("同种鱼词条每件装备最多 1 条，全身最多 2 条。")).toBeVisible()
  await expect(page.getByText("鱼类灌注仍在观察中，暂时不能开始制作")).toBeVisible()
  await expect(page.getByRole("button", { name: "立即执行" })).toBeDisabled()
  await expect(page.getByRole("button", { name: "加入队列" })).toBeDisabled()
  await page.screenshot({ path: "../output/playwright/alchemy-fish-infusion-desktop.png", fullPage: true })

  await page.waitForTimeout(250)
  expect(submittedAction).toBeNull()
})
