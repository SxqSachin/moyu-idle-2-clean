import { readFileSync } from "node:fs"
import { fileURLToPath } from "node:url"
import { expect, test, type Page } from "@playwright/test"

const sourcePath = fileURLToPath(new URL("../../packages/game-data/source/json/specialization_trees.json", import.meta.url))
const specializationTree = JSON.parse(readFileSync(sourcePath, "utf8")).specialization_trees[0]
const sourceNodes = specializationTree.nodes as Array<{
  id: string
  kind: string
  schoolIds: string[]
  requirements: {
    all: Array<{ kind: string; nodeId?: string; minRank?: number }>
    anyOf: Array<Array<{ kind: string; nodeId?: string; minRank?: number }>>
  }
  levels: Array<{ cost: number }>
}>
const strengthRouteNodes = sourceNodes.filter((node) => node.schoolIds.length === 1 && node.schoolIds[0] === "strength")
const strengthFocusNodes = strengthRouteNodes.filter((node) => node.kind === "root" || node.kind === "technique")
const highlightedConfluence = sourceNodes.find((node) => node.id === "confluence_strength_perception_pressure_point")
  || sourceNodes.find((node) => node.kind === "confluence" && node.schoolIds.includes("strength"))!
const strengthRootLocale = specializationTree.i18n["zh-CN"].nodes.strength_root_break_stance as {
  name: string
  desc: string
  effectSummary?: string[]
  levelEffectSummary?: string[][]
}
function ancestorIds(nodeId: string) {
  const byId = new Map(sourceNodes.map((node) => [node.id, node]))
  const result = new Set<string>()
  const visit = (id: string) => {
    const node = byId.get(id)
    if (!node) return
    const prerequisites = [...node.requirements.all, ...node.requirements.anyOf.flat()]
      .filter((requirement) => requirement.kind === "node_rank" && requirement.nodeId)
    for (const requirement of prerequisites) {
      if (!requirement.nodeId || result.has(requirement.nodeId)) continue
      result.add(requirement.nodeId)
      visit(requirement.nodeId)
    }
  }
  visit(nodeId)
  return result
}

const baseSlots = [
  "cloak", "head", "left_hand", "body", "right_hand", "arm", "legs", "feet",
  "accessory_1", "accessory_2", "accessory_3", "accessory_4"
]

type MockSpecializationAllocation = { nodeId: string; rank: number; paidCost: number }

function specializationSummary(
  dualHand = false,
  extraAllocations: MockSpecializationAllocation[] = []
) {
  const baseAllocations: MockSpecializationAllocation[] = dualHand
      ? [
        { nodeId: "strength_root_break_stance", rank: 6, paidCost: 7 },
        { nodeId: "strength_tech_heavy_momentum", rank: 4, paidCost: 5 },
        { nodeId: "strength_terminal_three_armed", rank: 1, paidCost: 22 }
      ]
    : [
        { nodeId: "strength_root_break_stance", rank: 1, paidCost: 2 },
        { nodeId: "dexterity_root_shift_rhythm", rank: 1, paidCost: 2 }
      ]
  const allocations = [...baseAllocations, ...extraAllocations]
  const allocationByNode = new Map(allocations.map((row) => [row.nodeId, row]))
  const spentPoints = allocations.reduce((sum, row) => sum + row.paidCost, 0)
  return {
    tree: specializationTree,
    status: {
      treeId: specializationTree.id,
      rulesetVersion: specializationTree.rulesetVersion,
      balanceRevision: specializationTree.balanceRevision,
      revision: 4,
      battleLevel: 500,
      formulaPoints: 65,
      bonusPoints: 0,
      totalPoints: 65,
      spentPoints,
      availablePoints: Math.max(0, 65 - spentPoints),
      pointBreakdown: {
        unlockGrantPoints: 2,
        levelMilestonePoints: 63,
        rewardPoints: 0,
        countedRewardPoints: 0,
        cappedPoints: 0,
        maxPoints: 65,
        atCap: true,
        nextPointBattleLevel: null,
        nextPointPoints: 0,
        milestoneBands: [
          {
            bandId: "early_growth",
            labelKey: "game.specialization.pointSource.levelMilestone",
            fromBattleLevel: 15,
            toBattleLevel: 90,
            everyLevels: 5,
            pointsPerGrant: 1,
            earnedGrantCount: 16,
            earnedPoints: 16,
            countedPoints: 16,
            firstEarnedBattleLevel: 15,
            lastEarnedBattleLevel: 90,
            nextMilestoneBattleLevel: null
          },
          {
            bandId: "rapid_growth",
            labelKey: "game.specialization.pointSource.levelMilestone",
            fromBattleLevel: 100,
            toBattleLevel: 195,
            everyLevels: 5,
            pointsPerGrant: 1,
            earnedGrantCount: 20,
            earnedPoints: 20,
            countedPoints: 20,
            firstEarnedBattleLevel: 100,
            lastEarnedBattleLevel: 195,
            nextMilestoneBattleLevel: null
          },
          {
            bandId: "level_200",
            labelKey: "game.specialization.pointSource.levelMilestone",
            fromBattleLevel: 200,
            toBattleLevel: 200,
            everyLevels: 1,
            pointsPerGrant: 2,
            earnedGrantCount: 1,
            earnedPoints: 2,
            countedPoints: 2,
            firstEarnedBattleLevel: 200,
            lastEarnedBattleLevel: 200,
            nextMilestoneBattleLevel: null
          },
          {
            bandId: "mastery_growth",
            labelKey: "game.specialization.pointSource.levelMilestone",
            fromBattleLevel: 210,
            toBattleLevel: 300,
            everyLevels: 10,
            pointsPerGrant: 2,
            earnedGrantCount: 10,
            earnedPoints: 20,
            countedPoints: 20,
            firstEarnedBattleLevel: 210,
            lastEarnedBattleLevel: 300,
            nextMilestoneBattleLevel: null
          },
          {
            bandId: "late_polish",
            labelKey: "game.specialization.pointSource.levelMilestone",
            fromBattleLevel: 350,
            toBattleLevel: 450,
            everyLevels: 50,
            pointsPerGrant: 1,
            earnedGrantCount: 3,
            earnedPoints: 3,
            countedPoints: 3,
            firstEarnedBattleLevel: 350,
            lastEarnedBattleLevel: 450,
            nextMilestoneBattleLevel: null
          },
          {
            bandId: "level_cap",
            labelKey: "game.specialization.pointSource.levelMilestone",
            fromBattleLevel: 500,
            toBattleLevel: 500,
            everyLevels: 1,
            pointsPerGrant: 2,
            earnedGrantCount: 1,
            earnedPoints: 2,
            countedPoints: 2,
            firstEarnedBattleLevel: 500,
            lastEarnedBattleLevel: 500,
            nextMilestoneBattleLevel: null
          }
        ],
        sources: [
          {
            sourceId: "unlock",
            kind: "unlock_grant",
            labelKey: "game.specialization.pointSource.unlockGrant",
            requiredBattleLevel: 10,
            points: 2,
            countedPoints: 2,
            earned: true
          }
        ]
      },
      allocations,
      nodeStates: specializationTree.nodes.map((node: { id: string; levels: Array<{ cost: number }> }) => ({
        nodeId: node.id,
        rank: allocationByNode.get(node.id)?.rank || 0,
        maxRank: node.levels.length,
        nextCost: (allocationByNode.get(node.id)?.rank || 0) >= node.levels.length ? 0 : node.levels[allocationByNode.get(node.id)?.rank || 0]?.cost || 0,
        canAllocate: (allocationByNode.get(node.id)?.rank || 0) < node.levels.length,
        unmetReasonCodes: []
      })),
      resetCost: { itemId: "gold_coin", count: 5000 }
    },
    characterAttributes: { strength: 28, dexterity: 24, intelligence: 22, perception: 23, stamina: 26 },
    effectiveSlots: [
      ...baseSlots.map((slot) => ({
        slot,
        family: slot.startsWith("accessory_") ? "accessory" : slot,
        ordinal: slot.startsWith("accessory_") ? Number(slot.split("_")[1]) : 1,
        enabled: !dualHand || !["accessory_3", "accessory_4"].includes(slot),
        accepts: [slot.startsWith("accessory_") ? "accessory" : slot]
      })),
      { slot: "head_2", family: "head", ordinal: 2, enabled: false, accepts: ["head"] },
      { slot: "body_2", family: "body", ordinal: 2, enabled: false, accepts: ["body"] },
      { slot: "right_hand_2", family: "right_hand", ordinal: 2, enabled: dualHand, accepts: ["right_hand", "weapon"] },
      { slot: "arm_2", family: "arm", ordinal: 2, enabled: false, accepts: ["arm"] },
      { slot: "feet_2", family: "feet", ordinal: 2, enabled: false, accepts: ["feet"] },
      { slot: "accessory_5", family: "accessory", ordinal: 5, enabled: false, accepts: ["accessory"] },
      { slot: "accessory_6", family: "accessory", ordinal: 6, enabled: false, accepts: ["accessory"] },
      { slot: "accessory_7", family: "accessory", ordinal: 7, enabled: false, accepts: ["accessory"] },
      { slot: "accessory_8", family: "accessory", ordinal: 8, enabled: false, accepts: ["accessory"] }
    ],
    slotCapacities: { cloak: 1, head: 1, left_hand: 1, body: 1, right_hand: dualHand ? 2 : 1, arm: 1, legs: 1, feet: 1, accessory: dualHand ? 2 : 4 }
  }
}

async function mockCharacterPage(page: Page, options: {
  dualHandReset?: boolean
  sendResetRealtime?: boolean
  extraAllocations?: MockSpecializationAllocation[]
} = {}) {
  let dualHand = options.dualHandReset === true
  const startsWithExtraHandItem = options.dualHandReset === true
  const resetRequests: Array<{ body: Record<string, unknown>; idempotencyKey: string }> = []
  const inventoryItem = {
    instanceId: 91,
    templateItemId: "cat_sword",
    quantity: 1,
    state: startsWithExtraHandItem ? "equipped" : "inventory",
    equippedSlot: startsWithExtraHandItem ? "right_hand_2" : "",
    equipType: "battle",
    enhanceLevel: 0,
    durability: 100,
    maxDurability: 100,
    createdAt: "2026-08-27T00:00:00Z",
    updatedAt: "2026-08-27T00:00:00Z"
  }
  let inventoryRequests = 0
  let specializationGets = 0
  let sendRealtime: ((type: string, data: unknown) => void) | null = null
  await page.addInitScript(() => {
    localStorage.setItem("locale", "zh-CN")
    localStorage.setItem("token", "e2e-token")
    localStorage.setItem("userId", "42")
    localStorage.setItem("username", "专精测试员")
    localStorage.setItem("player-settings:42", JSON.stringify({
      language: "zh-CN",
      theme: "light",
      menuLayout: "icon-rail",
      simpleMode: true,
      emailVerified: true
    }))
  })

  await page.routeWebSocket(/\/ws(?:\?|$)/, (socket) => {
    sendRealtime = (type, data) => socket.send(JSON.stringify({ type, ts: Date.now(), data }))
    socket.onMessage((message) => {
      if (typeof message !== "string") return
      const event = JSON.parse(message) as { type?: string }
      if (event.type === "auth.bind") {
        sendRealtime?.("auth.bound", {})
        if (options.dualHandReset) sendRealtime?.("equipment.instances.updated", { items: [inventoryItem] })
      }
      if (event.type === "action.offline.rewards.get") {
        socket.send(JSON.stringify({ type: "action.offline.rewards.result", ts: Date.now(), data: { hasMoreDue: false, rewards: [] } }))
      }
    })
  })

  await page.route("**/api/**", async (route) => {
    const url = new URL(route.request().url())
    const path = decodeURIComponent(url.pathname)
    const fulfill = (data: unknown) => route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ ok: true, ts: Date.now(), data })
    })
    if (path === "/api/player/specializations/player_combat_specialization/reset" && route.request().method() === "POST") {
      const body = (route.request().postDataJSON() || {}) as Record<string, unknown>
      resetRequests.push({
        body,
        idempotencyKey: route.request().headers()["x-idempotency-key"] || ""
      })
      if (body.impactDigest !== "reset-impact-91") {
        const preview = specializationSummary(true)
        return route.fulfill({
          status: 409,
          contentType: "application/json",
          body: JSON.stringify({
            ok: false,
            ts: Date.now(),
            error: {
              code: "SPECIALIZATION_IMPACT_CONFIRMATION_REQUIRED",
              message: "specialization change requires equipment impact confirmation"
            },
            data: {
              committed: false,
              requiresConfirmation: true,
              status: preview.status,
              effectiveSlots: preview.effectiveSlots,
              slotCapacities: preview.slotCapacities,
              characterAttributes: preview.characterAttributes,
              impact: {
                digest: "reset-impact-91",
                disabledSlots: ["right_hand_2"],
                enabledSlots: [],
                autoUnequipped: [{
                  instanceId: 91,
                  templateItemId: "cat_sword",
                  fromSlots: ["right_hand_2"],
                  reason: "slot_disabled"
                }],
                disabledSkills: []
              }
            }
          })
        })
      }
      dualHand = false
      inventoryItem.state = "inventory"
      inventoryItem.equippedSlot = ""
      if (options.sendResetRealtime !== false) {
        sendRealtime?.("equipment.instances.updated", { items: [] })
      }
      const next = specializationSummary(false, options.extraAllocations)
      next.status.revision = 5
      return fulfill({
        committed: true,
        requiresConfirmation: false,
        status: next.status,
        effectiveSlots: next.effectiveSlots,
        slotCapacities: next.slotCapacities,
        characterAttributes: next.characterAttributes,
        impact: { digest: "", disabledSlots: ["right_hand_2"], enabledSlots: [], autoUnequipped: [], disabledSkills: [] }
      })
    }
    if (path === "/api/bootstrap/init") {
      return fulfill({
        playerStats: {},
        inventory: {
          items: []
        },
        chatConfig: { channels: [], defaultChannel: "world" },
        chatHistory: { items: [] },
        playerSettings: { username: "专精测试员", language: "zh-CN", theme: "light", menuLayout: "icon-rail", simpleMode: true, emailVerified: true },
        tutorialProgress: {},
        playerPresence: { status: "online" },
        systemVersion: {},
        recycleConfig: { goldCoefficient: 0.5, affixMismatchExtraCoefficient: 0.6 }
      })
    }
    if (path === "/api/player/stats") {
      const attributes = Object.fromEntries(["strength", "dexterity", "intelligence", "perception", "stamina"].map((key, index) => [key, {
        value: 20 + index,
        baseValue: 20 + index,
        skillPointLevel: 0,
        allowBattlePointLevelUp: true,
        unit: "number",
        category: "battle",
        growthType: "levelable",
        order: index + 1,
        nameKey: `attr.${key}.name`
      }]))
      return fulfill({ stats: {}, attributes, attributeProgress: {}, buildings: {}, kitties: [], formula: { version: "v2", operators: [], derived: {} } })
    }
    if (path === "/api/inventory/instances") {
      inventoryRequests += 1
      const items = startsWithExtraHandItem ? [inventoryItem] : []
      return fulfill({ items })
    }
    if (path === "/api/battle/points") return fulfill({ totalPoints: 24, spentPoints: 0, available: 24, resetRequirement: { itemId: "gold_coin", count: 1 } })
    if (path === "/api/player/specializations/player_combat_specialization") {
      specializationGets += 1
      return fulfill(specializationSummary(dualHand, options.extraAllocations))
    }
    return fulfill({})
  })
  return {
    resetRequests,
    inventoryRequestCount: () => inventoryRequests,
    specializationGetCount: () => specializationGets
  }
}

async function openSpecialization(page: Page) {
  await openCharacterPage(page)
  await page.getByRole("tab", { name: "专精" }).click()
  await expect(page.locator("[data-tutorial='specialization-panel']")).toBeVisible({ timeout: 20_000 })
  await expect(page.locator("[data-testid^='specialization-node-']")).toHaveCount(sourceNodes.length)
  await page.waitForTimeout(300)
}

async function openCharacterPage(page: Page) {
  await page.goto("/game/attributes")
  const noticeClose = page.locator(".el-notification__closeBtn")
  await noticeClose.waitFor({ state: "visible", timeout: 2_000 }).catch(() => undefined)
  if (await noticeClose.isVisible()) await noticeClose.click()
}

async function selectSpecializationNode(page: Page, nodeId: string) {
  await page.getByTestId(`specialization-node-${nodeId}`).dispatchEvent("click")
}

async function dragSpecializationCanvas(
  page: Page,
  start: { x: number; y: number },
  delta: { x: number; y: number }
) {
  const viewport = page.locator(".vue-flow__transformationpane")
  const before = await viewport.getAttribute("style")
  await page.mouse.move(start.x, start.y)
  await page.mouse.down()
  await page.mouse.move(start.x + delta.x, start.y + delta.y, { steps: 6 })
  await page.mouse.up()
  await expect.poll(() => viewport.getAttribute("style")).not.toBe(before)
  await expect(page.locator(".vue-flow__nodesselection-rect")).toHaveCount(0)
}

async function blankCanvasPointNearNode(page: Page, nodeId: string) {
  const node = await page.getByTestId(`specialization-node-${nodeId}`).boundingBox()
  const canvas = await page.locator(".vue-flow__pane").boundingBox()
  if (!node || !canvas) return null
  return page.evaluate(({ node, canvas }) => {
    const candidates = [
      { x: node.x + node.width + 18, y: node.y + node.height / 2 },
      { x: node.x - 18, y: node.y + node.height / 2 },
      { x: node.x + node.width / 2, y: node.y + node.height + 18 },
      { x: node.x + node.width / 2, y: node.y - 18 }
    ]
    for (const yFraction of [0.18, 0.36, 0.58, 0.78]) {
      for (const xFraction of [0.16, 0.34, 0.58, 0.8]) {
        candidates.push({
          x: canvas.x + canvas.width * xFraction,
          y: canvas.y + canvas.height * yFraction
        })
      }
    }
    return candidates.find(({ x, y }) => {
      if (x <= canvas.x + 4 || y <= canvas.y + 4 || x >= canvas.x + canvas.width - 4 || y >= canvas.y + canvas.height - 4) return false
      const target = document.elementFromPoint(x, y)
      return !!target && !target.closest("button, .specialization-node, .el-popper, .vue-flow__controls, .vue-flow__minimap")
    }) || null
  }, { node, canvas })
}

test("renders the five-spoke specialization board without desktop overlap", async ({ page }) => {
  await page.setViewportSize({ width: 1440, height: 900 })
  const pageErrors: string[] = []
  page.on("pageerror", (error) => pageErrors.push(error.message))
  const mockState = await mockCharacterPage(page)
  await openSpecialization(page)
  const specializationGetsBeforeReentry = mockState.specializationGetCount()
  await page.getByRole("tab", { name: "属性", exact: true }).click()
  await page.getByRole("tab", { name: "专精", exact: true }).click()
  await expect.poll(() => mockState.specializationGetCount()).toBe(specializationGetsBeforeReentry + 1)
  await expect(page.locator(".specialization-ledger__routes")).toHaveText("根路线 2 / 3")

  const desktopPageScroll = await page.locator(".game-content-scroll").evaluate((element) => ({
    overflowY: getComputedStyle(element).overflowY,
    overflow: element.scrollHeight - element.clientHeight
  }))
  expect(desktopPageScroll.overflowY).toBe("hidden")
  expect(desktopPageScroll.overflow).toBeLessThanOrEqual(1)
  const initialDesktopWorkspace = await page.locator(".specialization-workspace").boundingBox()
  expect(initialDesktopWorkspace?.height || 0).toBeGreaterThan(430)

  await expect(page.locator(".specialization-node--confluence")).toHaveCount(
    sourceNodes.filter((node) => node.kind === "confluence").length
  )
  await expect(page.locator(".specialization-route-nav")).toHaveCount(0)
  await expect(page.locator(".specialization-school-key")).toContainText("虚线任选一条")
  const pointGuideTrigger = page.getByTestId("specialization-point-guide-trigger")
  const pointGuide = page.getByTestId("specialization-point-guide")
  await expect(pointGuideTrigger).toBeVisible()
  await expect(pointGuide).toBeHidden()
  await pointGuideTrigger.hover()
  await expect(pointGuide).toBeVisible()
  await expect(pointGuide).toContainText("战斗等级")
  await expect(pointGuide).toContainText("500")
  await expect(pointGuide).toContainText("当前 65 点的来源")
  await expect(pointGuide).toContainText("10 级解锁 +2")
  await expect(pointGuide).toContainText("15–90 级每 5 级 +1（已获 16）")
  await expect(pointGuide).toContainText("100–195 级每 5 级 +1（已获 20）")
  await expect(pointGuide).toContainText("200 级里程碑 +2")
  await expect(pointGuide).toContainText("210–300 级每 10 级 +2（已获 20）")
  await expect(pointGuide).toContainText("350–450 级每 50 级 +1（已获 3）")
  await expect(pointGuide).toContainText("500 级里程碑 +2")
  await expect(pointGuide).toContainText("已全部获得")
  await expect(page.locator("[data-testid^='specialization-node-']")).toHaveCount(sourceNodes.length)
  const hoverNode = page.getByTestId("specialization-node-strength_root_break_stance")
  await hoverNode.hover()
  await expect(page.getByTestId("specialization-tooltip-card")).toBeVisible()
  await page.mouse.move(8, 892)
  await expect(page.getByTestId("specialization-tooltip-card")).toHaveCount(0)
  await hoverNode.focus()
  await expect(page.getByTestId("specialization-tooltip-card")).toBeVisible()
  await page.getByRole("button", { name: "刷新" }).focus()
  await expect(page.getByTestId("specialization-tooltip-card")).toHaveCount(0)
  await selectSpecializationNode(page, "dexterity_terminal_shadow_dance")
  await page.waitForTimeout(100)
  const directIncomingEdges = await page.locator(".vue-flow__edge").evaluateAll((edges) => edges
    .map((edge) => edge.getAttribute("data-id") || "")
    .filter((id) => id.endsWith("->dexterity_terminal_shadow_dance")))
  expect(directIncomingEdges).toEqual(["dexterity_tech_rapid_strike->dexterity_terminal_shadow_dance"])
  await selectSpecializationNode(page, "dexterity_terminal_stolen_beat")
  await page.waitForTimeout(100)
  const alternativeIncomingEdges = await page.locator(".vue-flow__edge").evaluateAll((edges) => edges
    .map((edge) => edge.getAttribute("data-id") || "")
    .filter((id) => id.endsWith("->dexterity_terminal_stolen_beat"))
    .sort())
  expect(alternativeIncomingEdges).toEqual([
    "dexterity_tech_chain_steps->dexterity_terminal_stolen_beat",
    "dexterity_tech_critical_reprise->dexterity_terminal_stolen_beat"
  ])
  for (const edgeId of alternativeIncomingEdges) {
    const dashStyle = await page.locator(`.vue-flow__edge[data-id='${edgeId}'] .vue-flow__edge-path`).getAttribute("style")
    expect(dashStyle).toContain("stroke-dasharray: 7, 5")
  }
  const edgePaths = await page.locator(".vue-flow__edge-path").evaluateAll((paths) =>
    paths.map((path) => path.getAttribute("d") || "").filter(Boolean)
  )
  expect(edgePaths.length).toBeGreaterThan(0)
  for (const path of edgePaths) expect(path).not.toMatch(/[ACST]/i)
  await expect(page.locator(".specialization-inspector")).toHaveCount(0)
  await expect(page.getByTestId("specialization-tooltip-card")).toContainText("敏捷已投入 2 / 11 点")
  const nodeBoxes = await page.locator("[data-testid^='specialization-node-']").evaluateAll((nodes) => nodes.map((node) => {
    const rect = node.getBoundingClientRect()
    return { id: node.getAttribute("data-testid") || "", x: rect.x, y: rect.y, width: rect.width, height: rect.height }
  }))
  for (let leftIndex = 0; leftIndex < nodeBoxes.length; leftIndex += 1) {
    for (let rightIndex = leftIndex + 1; rightIndex < nodeBoxes.length; rightIndex += 1) {
      const left = nodeBoxes[leftIndex]
      const right = nodeBoxes[rightIndex]
      if (!left || !right) continue
      const overlapWidth = Math.min(left.x + left.width, right.x + right.width) - Math.max(left.x, right.x)
      const overlapHeight = Math.min(left.y + left.height, right.y + right.height) - Math.max(left.y, right.y)
      expect(overlapWidth > 1 && overlapHeight > 1, `${left.id} overlaps ${right.id}`).toBe(false)
    }
  }
  const desktopRenderedHeadingSize = await page.getByTestId(`specialization-node-${strengthRouteNodes[0]!.id}`).evaluate((node) => {
    const pane = node.closest(".vue-flow__transformationpane")
    const heading = node.querySelector("h3")
    const scale = pane ? new DOMMatrix(getComputedStyle(pane).transform).a : 1
    return heading ? parseFloat(getComputedStyle(heading).fontSize) * scale : 0
  })
  expect(desktopRenderedHeadingSize).toBeGreaterThanOrEqual(10)
  const routeAncestors = ancestorIds(highlightedConfluence.id)
  await selectSpecializationNode(page, highlightedConfluence.id)
  await page.waitForTimeout(300)
  for (const ancestorId of routeAncestors) {
    const routeNode = page.getByTestId(`specialization-node-${ancestorId}`)
    await expect(routeNode).toBeVisible()
    await expect(routeNode).toHaveClass(/is-route-ancestor/)
  }
  const selectedRouteHeadingSize = await page.getByTestId(`specialization-node-${highlightedConfluence.id}`).evaluate((node) => {
    const pane = node.closest(".vue-flow__transformationpane")
    const heading = node.querySelector("h3")
    const scale = pane ? new DOMMatrix(getComputedStyle(pane).transform).a : 1
    return heading ? parseFloat(getComputedStyle(heading).fontSize) * scale : 0
  })
  expect(selectedRouteHeadingSize).toBeGreaterThanOrEqual(10)
  await expect(page.getByTestId(`specialization-node-${highlightedConfluence.id}`)).toHaveClass(/is-route-selected/)
  await expect(page.locator(".specialization-node.is-route-muted").first()).toBeVisible()
  expect(await page.locator(".vue-flow__edge.specialization-edge--route").count()).toBeGreaterThan(0)
  const nodeTooltip = page.getByTestId("specialization-tooltip-card")
  await expect(nodeTooltip).toBeVisible()
  await expect(nodeTooltip.locator("h2")).not.toBeEmpty()
  await expect(nodeTooltip).not.toContainText("机制关键词")
  await expect(nodeTooltip).not.toContainText("实战效果")
  await expect(nodeTooltip).not.toContainText("未知效果")
  await expect(nodeTooltip).toContainText("力量路线：")
  await expect(nodeTooltip).toContainText("感知路线：")
  await expect(page.getByTestId("specialization-technical-details")).not.toHaveAttribute("open", "")
  const desktopTooltipBox = await page.locator(".specialization-node-popper:visible").boundingBox()
  expect(desktopTooltipBox).toBeTruthy()
  const popperRoundingTolerance = 1
  expect(desktopTooltipBox!.x).toBeGreaterThanOrEqual(-popperRoundingTolerance)
  expect(desktopTooltipBox!.x + desktopTooltipBox!.width).toBeLessThanOrEqual(1440 + popperRoundingTolerance)
  expect(desktopTooltipBox!.y).toBeGreaterThanOrEqual(-popperRoundingTolerance)
  expect(desktopTooltipBox!.y + desktopTooltipBox!.height).toBeLessThanOrEqual(900 + popperRoundingTolerance)
  await page.screenshot({ path: "output/playwright/specialization-desktop.png", fullPage: true })
  const workspace = await page.locator(".specialization-workspace").boundingBox()
  const canvas = await page.locator(".specialization-canvas-wrap").boundingBox()
  expect(canvas && workspace && Math.abs(canvas.width - workspace.width) <= 1).toBeTruthy()
  expect(workspace?.width).toBeLessThanOrEqual(1440)
  await selectSpecializationNode(page, "strength_root_break_stance")
  const rootSummary = page.getByTestId("specialization-effect-summary")
  await expect(rootSummary).toBeVisible()
  await expect(rootSummary).toContainText("当前 1 级全部效果")
  await expect(rootSummary).not.toContainText("力量 +")
  await expect(rootSummary).toContainText("获得 1 层破势，最多 2 层")
  await expect(rootSummary).toContainText("本次伤害提高 6.9802% + 力量×0.02599%")
  await expect(rootSummary).toContainText("升到 2 级后的全部效果")
  await expect(rootSummary).toContainText("本次伤害提高 11.1683% + 力量×0.0416%")
  const technicalDetails = page.getByTestId("specialization-technical-details")
  await expect(technicalDetails).not.toHaveAttribute("open", "")
  await technicalDetails.locator("summary").click()
  await expect(technicalDetails).toHaveAttribute("open", "")
  await expect(technicalDetails.locator("li").first()).toBeVisible()
  await expect(technicalDetails).not.toContainText("未知效果")

  const firstStrengthTechnique = sourceNodes.find((node) => node.id === "strength_tech_heavy_momentum")!
  const rootGate = firstStrengthTechnique.requirements.all.find((requirement) => requirement.kind === "node_rank")!
  expect(rootGate.minRank).toBe(6)
  await selectSpecializationNode(page, firstStrengthTechnique.id)
  const strengthRootName = String(specializationTree.i18n["zh-CN"].nodes[rootGate.nodeId!].name)
  await expect(page.getByTestId("specialization-tooltip-card")).toContainText(`${strengthRootName}达到 6 级`)

  const selectedRoot = page.getByTestId("specialization-node-strength_root_break_stance")
  await selectSpecializationNode(page, "strength_root_break_stance")
  await expect(selectedRoot).toHaveClass(/is-route-selected/)
  await selectSpecializationNode(page, "strength_root_break_stance")
  await expect(selectedRoot).not.toHaveClass(/is-route-selected/)
  await expect(selectedRoot).not.toHaveClass(/\bis-selected\b/)

  await selectSpecializationNode(page, highlightedConfluence.id)
  const pane = page.locator(".vue-flow__pane")
  const paneBox = await pane.boundingBox()
  expect(paneBox).toBeTruthy()
  const nearbyBlank = await blankCanvasPointNearNode(page, highlightedConfluence.id)
  expect(nearbyBlank, "there should be draggable canvas beside the selected node").toBeTruthy()
  await dragSpecializationCanvas(page, nearbyBlank!, { x: 56, y: 34 })
  await expect(page.getByTestId(`specialization-node-${highlightedConfluence.id}`)).toHaveClass(/is-route-selected/)

  const movedPaneBox = await pane.boundingBox()
  expect(movedPaneBox).toBeTruthy()
  const movedNearbyBlank = await blankCanvasPointNearNode(page, highlightedConfluence.id)
  expect(movedNearbyBlank, "there should still be draggable canvas after panning").toBeTruthy()
  await dragSpecializationCanvas(page, movedNearbyBlank!, { x: -96, y: 72 })
  await expect(page.getByTestId(`specialization-node-${highlightedConfluence.id}`)).toHaveClass(/is-route-selected/)
  await expect(page.locator(".vue-flow__nodesselection-rect")).toHaveCount(0)
  const deselectPoint = await blankCanvasPointNearNode(page, highlightedConfluence.id)
  expect(deselectPoint, "there should be blank canvas available to clear the selection").toBeTruthy()
  await page.mouse.click(deselectPoint!.x, deselectPoint!.y)
  await expect(page.getByTestId(`specialization-node-${highlightedConfluence.id}`)).not.toHaveClass(/is-route-selected/)
  await expect(page.getByTestId(`specialization-node-${highlightedConfluence.id}`)).not.toHaveClass(/\bis-selected\b/)
  await selectSpecializationNode(page, highlightedConfluence.id)
  await page.keyboard.press("Escape")
  await expect(page.getByTestId(`specialization-node-${highlightedConfluence.id}`)).not.toHaveClass(/is-route-selected/)
  await expect(page.getByTestId(`specialization-node-${highlightedConfluence.id}`)).not.toHaveClass(/\bis-selected\b/)
  expect(pageErrors).toEqual([])
  await selectSpecializationNode(page, "dexterity_terminal_shadow_dance")
  await page.waitForTimeout(300)
  await expect(page.getByTestId("specialization-technical-details")).not.toHaveAttribute("open", "")
})

test("shows complete rank snapshots and renders specialization debuffs as BuffQuickRef", async ({ page }) => {
  await page.setViewportSize({ width: 1440, height: 900 })
  await mockCharacterPage(page, {
    extraAllocations: [
      { nodeId: "dexterity_tech_chain_steps", rank: 4, paidCost: 5 },
      { nodeId: "intelligence_tech_cooldown_ledger", rank: 4, paidCost: 5 }
    ]
  })
  await openSpecialization(page)

  await selectSpecializationNode(page, "dexterity_tech_chain_steps")
  const chainSummary = page.getByTestId("specialization-effect-summary")
  await expect(chainSummary).toContainText("当前 4 级全部效果")
  await expect(chainSummary).toContainText("装备镰刀")
  await expect(chainSummary).toContainText("额外命中 1 次")
  await expect(chainSummary).toContainText("每段数值变为原来的 80%")
  await expect(chainSummary).not.toContainText("敏捷 +")
  await page.waitForTimeout(260)
  await page.screenshot({ path: "output/playwright/specialization-complete-rank.png", fullPage: true })
  let details = page.getByTestId("specialization-technical-details")
  await details.locator("summary").click()
  await expect(details).toContainText("按当前属性：")
  await expect(details).not.toContainText("生效对象：上述技能的所有效果")
  await page.getByTestId("specialization-tooltip-card").screenshot({
    path: "output/playwright/specialization-complete-rank-snapshot.png"
  })

  await selectSpecializationNode(page, "dexterity_tech_adapt_on_hit")
  const spearSummary = page.getByTestId("specialization-effect-summary")
  await expect(spearSummary).toContainText("装备矛")
  await expect(spearSummary).toContainText("反刺伤害来源")
  await expect(spearSummary).toContainText("6 + 敏捷×5%")
  await page.waitForTimeout(260)
  await page.screenshot({ path: "output/playwright/specialization-weapon-flavor.png", fullPage: true })

  await selectSpecializationNode(page, "dexterity_terminal_reaping_sweep")
  const sweepSummary = page.getByTestId("specialization-effect-summary")
  await expect(sweepSummary).toContainText("触发：普通攻击或主动技能的首个伤害效果命中后")
  await expect(sweepSummary).toContainText("条件：装备镰刀，且原目标同排还有其他敌人")
  await expect(sweepSummary).toContainText("同排 1 个其他敌人造成原效果 30%")
  await expect(sweepSummary).toContainText("最多追加 1 个目标")
  await expect(sweepSummary).not.toContainText(/匕首|刀扇/)
  await page.waitForTimeout(260)
  await page.screenshot({ path: "output/playwright/specialization-scythe-sweep.png", fullPage: true })

  await selectSpecializationNode(page, "perception_terminal_foresight")
  const spearDepthSummary = page.getByTestId("specialization-effect-summary")
  await expect(spearDepthSummary).toContainText("触发：长矛普通攻击结算前排和中排伤害后")
  await expect(spearDepthSummary).toContainText("条件：装备长矛，且后排有存活敌人")
  await expect(spearDepthSummary).toContainText("后排 1 个敌人造成首段伤害 25%")
  await expect(spearDepthSummary).toContainText("最多命中前、中、后排各 1 个目标")
  await page.waitForTimeout(260)
  await page.screenshot({ path: "output/playwright/specialization-spear-depth.png", fullPage: true })

  await selectSpecializationNode(page, "confluence_strength_dexterity_swift_force")
  const longswordSummary = page.getByTestId("specialization-effect-summary")
  await expect(longswordSummary).toContainText("触发：主动刀剑技能的首个物理伤害效果命中后")
  await expect(longswordSummary).toContainText("条件：装备长剑")
  await expect(longswordSummary).toContainText("任意排位 1 个其他敌人造成原效果 24%")
  await expect(longswordSummary).toContainText("普通攻击不受影响")
  await page.waitForTimeout(260)
  await page.screenshot({ path: "output/playwright/specialization-longsword-arc.png", fullPage: true })

  await selectSpecializationNode(page, "strength_terminal_titan_combo")
  const greatbladeSummary = page.getByTestId("specialization-effect-summary")
  await expect(greatbladeSummary).toContainText("触发：每回合第一次造成物理伤害前")
  await expect(greatbladeSummary).toContainText("条件：装备大刀，且自身生命不高于 55%")
  await expect(greatbladeSummary).toContainText("本次伤害提高 12%")
  await expect(greatbladeSummary).toContainText("每回合最多触发 1 次")
  await page.waitForTimeout(260)
  await page.screenshot({ path: "output/playwright/specialization-greatblade-brink.png", fullPage: true })

  await selectSpecializationNode(page, "intelligence_tech_cooldown_ledger")
  const cooldownSummary = page.getByTestId("specialization-effect-summary")
  await expect(cooldownSummary).toContainText("当前 4 级全部效果")
  await expect(cooldownSummary).toContainText("所有技能冷却 -1")
  await expect(cooldownSummary).toContainText("最低仍为 1 回合")
  await expect(cooldownSummary).toContainText("技能法力消耗变为原来的 300%")
  await page.waitForTimeout(260)
  await page.screenshot({ path: "output/playwright/specialization-cooldown-safeguard.png", fullPage: true })

  for (const [nodeId, buffName] of [
    ["dexterity_tech_critical_reprise", "弓弦节拍"],
    ["stamina_terminal_reactive_bulwark", "盾墙架势"],
    ["strength_tech_overpower_pursuit", "收刀定势"],
    ["dexterity_terminal_shadow_dance", "猎镰余势"],
    ["perception_tech_relentless_pursuit", "快装箭"]
  ] as const) {
    await selectSpecializationNode(page, nodeId)
    await expect(page.getByTestId("specialization-effect-summary")
      .locator(".buff-quick-ref-tag").filter({ hasText: buffName }).first()).toBeVisible()
  }

  await selectSpecializationNode(page, "stamina_root_hold_line")
  const guardSummary = page.getByTestId("specialization-effect-summary")
  await expect(guardSummary.locator(".game-keyword-tag").filter({ hasText: "守势" }).first()).toBeVisible()
  await expect(guardSummary.locator(".buff-quick-ref-tag").filter({ hasText: "守势" })).toHaveCount(0)

  await selectSpecializationNode(page, "confluence_dexterity_intelligence_quick_formula")
  const quickFormulaSummary = page.getByTestId("specialization-effect-summary")
  await expect(quickFormulaSummary).toContainText("最低仍为 1 回合")
  await expect(quickFormulaSummary).toContainText("法力消耗变为原来的 400%")

  for (const nodeId of ["strength_terminal_executioner", "stamina_terminal_last_line"]) {
    await selectSpecializationNode(page, nodeId)
    details = page.getByTestId("specialization-technical-details")
    await details.locator("summary").click()
    await expect(details.locator("li").first()).toContainText("触发：")
    await expect(details.locator("li").filter({ hasText: "效果：" }).first()).toBeVisible()
    await expect(details).not.toContainText("暂无效果")
  }

  await selectSpecializationNode(page, "intelligence_terminal_first_echo")
  const echoTooltip = page.getByTestId("specialization-tooltip-card")
  await expect(echoTooltip.getByTestId("specialization-keywords")).toHaveCount(0)
  const echoSummary = echoTooltip.getByTestId("specialization-effect-summary")
  await expect(echoSummary).toContainText("复诵")
  await echoSummary.locator(".game-keyword-tag").filter({ hasText: "复诵" }).first().click()
  const keywordCard = page.locator(".game-keyword-card:visible")
  await expect(keywordCard).toContainText("再次施放")
  await expect(keywordCard).toContainText("如何获得")
  await page.waitForTimeout(220)
  await page.screenshot({ path: "output/playwright/specialization-keyword-tooltip.png", fullPage: true })

  await selectSpecializationNode(page, "confluence_perception_stamina_sentinel_link")
  const interceptTooltip = page.getByTestId("specialization-tooltip-card")
  await expect(interceptTooltip).toContainText(/生命比例最低|最低生命/)
  const interceptSummary = interceptTooltip.getByTestId("specialization-effect-summary")
  await expect(interceptSummary).toContainText(/替代承担|替目标承担/)
  await expect(interceptSummary.locator(".buff-quick-ref-tag").filter({ hasText: "哨卫锁定" })).toBeVisible()

  await selectSpecializationNode(page, "perception_tech_reveal_weakness")
  const exposedTooltip = page.getByTestId("specialization-tooltip-card")
  const exposedSummary = exposedTooltip.getByTestId("specialization-effect-summary")
  await expect(exposedSummary).toContainText("破绽")
  const exposedBuffRef = exposedSummary.locator(".buff-quick-ref-tag").filter({ hasText: "破绽" }).first()
  await expect(exposedBuffRef).toBeVisible()
  await exposedBuffRef.hover()
  const exposedBuffCard = page.locator(".buff-quick-ref-popper:visible")
  await expect(exposedBuffCard).toContainText("伤害技能留下的可驱散减益")
  await page.screenshot({ path: "output/playwright/specialization-debuff-tooltip.png", fullPage: true })

  await selectSpecializationNode(page, "confluence_strength_perception_pressure_point")
  details = page.getByTestId("specialization-technical-details")
  await details.locator("summary").click()
  const recognizedBuffRefs = details.locator(".buff-quick-ref-tag")
  await expect(recognizedBuffRefs).toHaveCount(8)
  for (const name of ["观察", "重点目标", "破绽", "锁定", "抢先锁定", "连击锁定", "裂镰创口", "哨卫锁定"]) {
    await expect(recognizedBuffRefs.filter({ hasText: name }).first()).toBeVisible()
  }
  await page.waitForTimeout(260)
  await page.screenshot({ path: "output/playwright/specialization-recognized-debuffs.png", fullPage: true })

  await selectSpecializationNode(page, "perception_terminal_preemptive_strike")
  const preemptiveSummary = page.getByTestId("specialization-effect-summary")
  await expect(preemptiveSummary).toContainText("每波开始时为威胁最高的敌人施加")
  await expect(preemptiveSummary).toContainText("本波首次攻击该目标")
  await expect(preemptiveSummary).toContainText("本次伤害提高 10%")
  await expect(preemptiveSummary).not.toContainText("给自己附加")
  const preemptiveBuffRef = preemptiveSummary.locator(".buff-quick-ref-tag").filter({ hasText: "抢先锁定" }).first()
  await expect(preemptiveBuffRef).toBeVisible()
  await preemptiveBuffRef.hover()
  const preemptiveBuffCard = page.locator(".buff-quick-ref-popper:visible")
  await expect(preemptiveBuffCard).toContainText("物理、魔法抗性各降低 15 个百分点")
  await preemptiveBuffCard.screenshot({ path: "output/playwright/specialization-preemptive-lock-buff-card.png" })
  await page.screenshot({ path: "output/playwright/specialization-preemptive-lock.png", fullPage: true })

  await selectSpecializationNode(page, "dexterity_root_deadly_strike")
  details = page.getByTestId("specialization-technical-details")
  await details.locator("summary").click()
  await expect(details).not.toContainText("指定减益")

  await selectSpecializationNode(page, "confluence_dexterity_perception_hunt_sequence")
  details = page.getByTestId("specialization-technical-details")
  await details.locator("summary").click()
  const orderedTechnicalLines = await details.locator("li").allInnerTexts()
  const triggerIndex = orderedTechnicalLines.findIndex((line) => line.startsWith("触发："))
  const conditionIndex = orderedTechnicalLines.findIndex((line) => line.startsWith("条件："))
  const effectIndex = orderedTechnicalLines.findIndex((line) => line.startsWith("效果："))
  expect(triggerIndex).toBeGreaterThanOrEqual(0)
  expect(conditionIndex).toBeGreaterThan(triggerIndex)
  expect(effectIndex).toBeGreaterThan(conditionIndex)

  await selectSpecializationNode(page, "dexterity_terminal_stolen_beat")
  details = page.getByTestId("specialization-technical-details")
  await details.locator("summary").click()
  await expect(details).toContainText("触发概率：20%（最高 40%）")
  await expect(details).toContainText("每个敌人刷新波次最多触发 1 次")
  await page.screenshot({ path: "output/playwright/specialization-probability-order.png", fullPage: true })

  await selectSpecializationNode(page, "confluence_intelligence_perception_mind_lock")
  details = page.getByTestId("specialization-technical-details")
  await details.locator("summary").click()
  await expect(details).toContainText("目标阵营：敌方")
  await expect(details).toContainText("选择规则：生命比例最低")
  await expect(details).toContainText("次要寻敌：前排到后排")
  await expect(details).toContainText("无可用目标时改选自身：否")
  await expect(details).not.toContainText(/fallbackSelfIfEmpty|secondarySeek|selectRule|targetScope/)
  await page.screenshot({ path: "output/playwright/specialization-targeting-i18n.png", fullPage: true })

  await selectSpecializationNode(page, "strength_root_break_stance")
  const strengthTooltip = page.getByTestId("specialization-tooltip-card")
  await expect(strengthTooltip.getByTestId("specialization-effect-summary")).toContainText(/破势|蓄力/)
  await expect(strengthTooltip).not.toContainText(strengthRootLocale.desc)
})

test("opens compact node tooltips on mobile without a drawer", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 })
  const pageErrors: string[] = []
  page.on("pageerror", (error) => pageErrors.push(error.message))
  await mockCharacterPage(page)
  await openSpecialization(page)

  const pointGuideTrigger = page.getByTestId("specialization-point-guide-trigger")
  const pointGuide = page.getByTestId("specialization-point-guide")
  await expect(pointGuide).toBeHidden()
  await pointGuideTrigger.click()
  await expect(pointGuide).toBeVisible()
  await expect(pointGuide).toContainText("当前 65 点的来源")
  await expect(pointGuide).toContainText("已全部获得")
  const pointSourceRows = pointGuide.locator(".specialization-point-guide__source li")
  await expect(pointSourceRows).toHaveCount(7)
  const pointSourceRowTops = await pointSourceRows.evaluateAll((rows) => rows.map((row) => Math.round(row.getBoundingClientRect().top)))
  expect(new Set(pointSourceRowTops).size).toBe(7)
  await pointGuideTrigger.click()
  await expect(pointGuide).toBeHidden()
  const mobileCanvas = await page.locator(".specialization-flow").boundingBox()
  const mobileNodeBoxes = await page.locator("[data-testid^='specialization-node-']").evaluateAll((nodes) => nodes.map((node) => {
    const rect = node.getBoundingClientRect()
    return { id: node.getAttribute("data-testid") || "", x: rect.x, y: rect.y, width: rect.width, height: rect.height }
  }))
  const mobileViewportHeight = await page.evaluate(() => window.innerHeight)
  const chatButton = await page.locator(".pointer-events-none.fixed .el-badge").boundingBox()
  const fullyVisibleNodes = mobileNodeBoxes.filter((node) => node && mobileCanvas
    && node.x >= mobileCanvas.x - 1
    && node.y >= mobileCanvas.y - 1
    && node.x + node.width <= mobileCanvas.x + mobileCanvas.width + 1
    && node.y + node.height <= mobileCanvas.y + mobileCanvas.height + 1
    && node.y + node.height <= mobileViewportHeight - 1)
  const visibleStrengthFocusNodes = fullyVisibleNodes.filter((node) =>
    strengthFocusNodes.some((candidate) => node.id === `specialization-node-${candidate.id}`)
  )
  expect(visibleStrengthFocusNodes.length, "the initial mobile view should contain a readable strength route start").toBeGreaterThanOrEqual(2)
  for (const node of fullyVisibleNodes) {
    if (chatButton) {
      const overlapsChat = node.x < chatButton.x + chatButton.width
        && node.x + node.width > chatButton.x
        && node.y < chatButton.y + chatButton.height
        && node.y + node.height > chatButton.y
      expect(overlapsChat, `${node.id} is covered by the chat button`).toBe(false)
    }
  }
  const firstVisibleStrengthNodeId = visibleStrengthFocusNodes[0]!.id.replace("specialization-node-", "")
  const mobileRenderedHeadingSize = await page.getByTestId(`specialization-node-${firstVisibleStrengthNodeId}`).evaluate((node) => {
    const pane = node.closest(".vue-flow__transformationpane")
    const heading = node.querySelector("h3")
    const scale = pane ? new DOMMatrix(getComputedStyle(pane).transform).a : 1
    return heading ? parseFloat(getComputedStyle(heading).fontSize) * scale : 0
  })
  expect(mobileRenderedHeadingSize).toBeGreaterThanOrEqual(11)
  await expect(page.locator(".specialization-route-nav")).toHaveCount(0)
  const mobilePageScroll = await page.locator(".game-content-scroll").evaluate((element) => ({
    overflowY: getComputedStyle(element).overflowY,
    overflow: element.scrollHeight - element.clientHeight
  }))
  expect(mobilePageScroll.overflowY).toBe("hidden")
  expect(mobilePageScroll.overflow).toBeLessThanOrEqual(1)
  expect(mobileCanvas?.height || 0).toBeGreaterThan(280)
  await page.screenshot({ path: "output/playwright/specialization-mobile-board.png", fullPage: true })
  await selectSpecializationNode(page, "strength_root_break_stance")
  await expect(page.locator(".el-drawer.btt")).toHaveCount(0)
  const tooltip = page.getByTestId("specialization-tooltip-card")
  await expect(tooltip).toBeVisible()
  await expect(tooltip.locator("h2")).toHaveText(strengthRootLocale.name)
  const mobileRootSummary = tooltip.getByTestId("specialization-effect-summary")
  await expect(mobileRootSummary).toBeVisible()
  await expect(mobileRootSummary).toContainText("当前 1 级全部效果")
  await expect(mobileRootSummary).toContainText("本次伤害提高 6.9802% + 力量×0.02599%")
  await expect(mobileRootSummary).toContainText("升到 2 级后的全部效果")
  await expect(mobileRootSummary).toContainText("本次伤害提高 11.1683% + 力量×0.0416%")
  await expect(tooltip).toContainText("从这里开启力量路线")
  const mobileTooltipBox = await page.locator(".specialization-node-popper:visible").boundingBox()
  expect(mobileTooltipBox).toBeTruthy()
  expect(mobileTooltipBox!.x).toBeGreaterThanOrEqual(42)
  expect(mobileTooltipBox!.x + mobileTooltipBox!.width).toBeLessThanOrEqual(390)
  expect(mobileTooltipBox!.y).toBeGreaterThanOrEqual(0)
  expect(mobileTooltipBox!.y + mobileTooltipBox!.height).toBeLessThanOrEqual(844)
  await page.waitForTimeout(220)
  await page.screenshot({ path: "output/playwright/specialization-mobile-tooltip.png", fullPage: true })
  const technicalDetails = tooltip.getByTestId("specialization-technical-details")
  await expect(technicalDetails).not.toHaveAttribute("open", "")
  await technicalDetails.locator("summary").click()
  await expect(technicalDetails.locator("li").first()).toBeVisible()
  await technicalDetails.locator("summary").click()
  await expect(technicalDetails).not.toHaveAttribute("open", "")
  await tooltip.getByRole("button", { name: "关闭专精详情" }).click()
  await expect(page.getByTestId("specialization-tooltip-card")).toHaveCount(0)
  const viewportOverflow = await page.locator("html").evaluate((element) => element.scrollWidth - element.clientWidth)
  expect(viewportOverflow).toBeLessThanOrEqual(1)
  expect(pageErrors).toEqual([])
  await page.screenshot({ path: "output/playwright/specialization-mobile.png", fullPage: true })
})

test("HTTP reset refreshes equipment without a realtime equipment patch", async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 900 })
  const mockState = await mockCharacterPage(page, { dualHandReset: true, sendResetRealtime: false })
  await page.goto("/game/attributes")
  const noticeClose = page.locator(".el-notification__closeBtn")
  await noticeClose.waitFor({ state: "visible", timeout: 2_000 }).catch(() => undefined)
  if (await noticeClose.isVisible()) await noticeClose.click()

  await page.getByRole("tab", { name: "装备", exact: true }).click()
  await page.getByRole("tab", { name: "战斗", exact: true }).click()
  const activeEquipmentPane = page.locator(".equipment-tabs .el-tab-pane[aria-hidden='false']")
  const extraHand = activeEquipmentPane.locator("[data-tutorial='equipment-specialization-slots']")
  await expect(extraHand).toBeVisible()
  await expect(extraHand.getByText("副右手", { exact: true })).toBeVisible()
  await expect(extraHand.locator(".lp-suppress")).toHaveCount(1)

  await page.getByRole("tab", { name: "专精", exact: true }).click()
  await expect(page.locator("[data-tutorial='specialization-panel']")).toBeVisible()
  await page.getByRole("button", { name: "重置专精", exact: true }).click()
  const confirm = page.locator(".el-message-box")
  await expect(confirm).toBeVisible()
  await confirm.getByRole("button", { name: "确认", exact: true }).click()

  const impactDialog = page.locator(".el-dialog:visible").filter({ hasText: "确认构筑变化" })
  await expect(impactDialog).toBeVisible()
  await expect(impactDialog).toContainText("副右手")
  await expect(impactDialog).toContainText("猫猫剑")
  const previewExtraHand = activeEquipmentPane.locator("[data-tutorial='equipment-specialization-slots']")
  await expect(previewExtraHand).toHaveCount(1)
  await expect(previewExtraHand.locator(".lp-suppress")).toHaveCount(1)
  expect(mockState.resetRequests).toHaveLength(1)
  expect(mockState.resetRequests[0]?.body.impactDigest).toBeUndefined()
  await impactDialog.getByRole("button", { name: "确认并提交", exact: true }).click()
  await expect(impactDialog).toBeHidden()
  expect(mockState.resetRequests).toHaveLength(2)
  expect(mockState.resetRequests[1]?.body.impactDigest).toBe("reset-impact-91")
  expect(mockState.resetRequests[1]?.idempotencyKey).toBe(mockState.resetRequests[0]?.idempotencyKey)

  await page.getByRole("tab", { name: "装备", exact: true }).click()
  await page.getByRole("tab", { name: "战斗", exact: true }).click()
  await expect(activeEquipmentPane.locator("[data-tutorial='equipment-specialization-slots']")).toHaveCount(0)
  await expect(activeEquipmentPane.getByText("副右手", { exact: true })).toHaveCount(0)

  await activeEquipmentPane.locator("[data-tutorial='equipment-main-slots']").getByRole("button", { name: /右手/ }).click()
  await expect(page.locator(".slot-dropdown-panel")).toContainText("猫猫剑")
})
