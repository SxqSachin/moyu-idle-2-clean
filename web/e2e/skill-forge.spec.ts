import { expect, test, type Page } from "@playwright/test"

const now = "2026-08-14T08:00:00Z"

function graphWithMaterial(sourceKind: "blank" | "official" = "blank") {
  return {
    version: 1,
    source: { kind: sourceKind, ...(sourceKind === "official" ? { skillId: "power_strike", definitionHash: "hash:power_strike" } : {}) },
    nodes: [
      { id: "source", kind: "source", position: { x: 40, y: 180 } },
      { id: "effect-1", kind: "effect", effectKind: "damage", effectIndex: 0, position: { x: 300, y: 120 } },
      { id: "material-1", kind: "material", moduleId: "skill_forge.silkwood", quantity: 200, effectIndex: 0, position: { x: 560, y: 120 } },
      { id: "output", kind: "output", position: { x: 820, y: 180 } }
    ],
    edges: [
      { id: "edge-1", source: "source", sourceHandle: "effect", target: "effect-1", targetHandle: "in" },
      { id: "edge-2", source: "effect-1", sourceHandle: "out", target: "material-1", targetHandle: "in" },
      { id: "edge-3", source: "material-1", sourceHandle: "out", target: "output", targetHandle: "effect" }
    ]
  }
}

function moduleDefinition() {
  return {
    id: "skill_forge.silkwood",
    itemId: "silkwood",
    nodeKind: "formula",
    typedPorts: { inputs: [{ id: "in", type: "effect" }], outputs: [{ id: "out", type: "effect" }] },
    compileOp: { kind: "formula_term", effectKind: "damage", attrKey: "dexterity" },
    valuePerUnit: { min: 0.0004, max: 0.00088 },
    unitBasis: 200,
    maxBasisMultiplier: 20,
    minAlchemyLevel: 15,
    failBaseChance: 0.16,
    failChancePerLevelAbove: 0.006,
    successChanceModifier: -0.04,
    budgetCreditPerUnit: 0.0112,
    nameParts: { coreKey: "item.silkwood.name", suffixKey: "skillForge.name.damage" },
    icon: "item.silkwood"
  }
}

function conditionModuleDefinition() {
  return {
    id: "skill_forge.silver_vein_silk",
    itemId: "silver_vein_silk",
    nodeKind: "condition",
    typedPorts: { inputs: [{ id: "in", type: "condition" }], outputs: [{ id: "true", type: "effect" }, { id: "false", type: "effect" }] },
    compileOp: { kind: "target_status_condition", buffId: "armor_break" },
    valuePerUnit: { min: 0, max: 0 },
    unitBasis: 200,
    maxBasisMultiplier: 6,
    minAlchemyLevel: 15,
    failBaseChance: 0.18,
    failChancePerLevelAbove: 0.006,
    successChanceModifier: -0.05,
    budgetCreditPerUnit: 0.0072,
    nameParts: { coreKey: "item.silver_vein_silk.name", suffixKey: "skillForge.name.conditional" },
    icon: "item.silver_vein_silk"
  }
}

function effectSlotModuleDefinition() {
  return {
    id: "skill_forge.harmonic_embryo",
    itemId: "harmonic_embryo",
    nodeKind: "structure",
    typedPorts: { inputs: [{ id: "in", type: "skill" }], outputs: [{ id: "out", type: "skill" }] },
    compileOp: { kind: "effect_slot" },
    valuePerUnit: { min: 0, max: 0 },
    unitBasis: 200,
    maxBasisMultiplier: 2,
    minAlchemyLevel: 45,
    failBaseChance: 0.36,
    failChancePerLevelAbove: 0.009,
    successChanceModifier: -0.10,
    budgetCreditPerUnit: 0.015,
    nameParts: { coreKey: "item.harmonic_embryo.name", suffixKey: "skillForge.name.chain" },
    icon: "item.harmonic_embryo"
  }
}

function statusApplyModuleDefinition() {
  return {
    id: "skill_forge.sunward_resin",
    itemId: "sunward_resin",
    nodeKind: "status",
    typedPorts: { inputs: [{ id: "in", type: "effect" }], outputs: [{ id: "out", type: "effect" }] },
    compileOp: { kind: "status_apply", buffId: "frozen_action_suppressed", targetScope: "enemy", durationBaseTurns: 1 },
    valuePerUnit: { min: 0.1, max: 0.1 },
    unitBasis: 200,
    maxBasisMultiplier: 4,
    minAlchemyLevel: 30,
    failBaseChance: 0.2,
    failChancePerLevelAbove: 0.006,
    successChanceModifier: -0.06,
    budgetCreditPerUnit: 0.01,
    nameParts: { coreKey: "item.sunward_resin.name", suffixKey: "skillForge.name.blessing" },
    icon: "item.sunward_resin"
  }
}

function statusDurationModuleDefinition() {
  return {
    id: "skill_forge.frost_magic_scroll",
    itemId: "frost_magic_scroll",
    nodeKind: "status",
    typedPorts: { inputs: [{ id: "in", type: "effect" }], outputs: [{ id: "out", type: "effect" }] },
    compileOp: { kind: "status_duration" },
    valuePerUnit: { min: 1, max: 1 },
    unitBasis: 200,
    maxBasisMultiplier: 3,
    minAlchemyLevel: 30,
    failBaseChance: 0.2,
    failChancePerLevelAbove: 0.006,
    successChanceModifier: -0.06,
    budgetCreditPerUnit: 0.01,
    nameParts: { coreKey: "item.frost_magic_scroll.name", suffixKey: "skillForge.name.lingering" },
    icon: "item.frost_magic_scroll"
  }
}

async function mockSkillForge(page: Page, official = false, forgeFails = true, duplicateForgeOnce = false, includeSecondDraft = false) {
  let draft = {
    id: "draft:1",
    sourceKind: official ? "official" : "blank",
    sourceSkillId: official ? "power_strike" : undefined,
    sourceSkillbookItemId: official ? "skillbook_power_strike" : undefined,
    sourceSkillbookCount: official ? 700 : 0,
    status: "active",
    graph: graphWithMaterial(official ? "official" : "blank"),
    revisionNo: 1,
    createdAt: now,
    updatedAt: now
  } as Record<string, unknown>
  let secondDraft = {
    ...draft,
    id: "draft:2",
    sourceKind: "blank",
    sourceSkillId: undefined,
    sourceSkillbookItemId: undefined,
    sourceSkillbookCount: 0,
    graph: graphWithMaterial("blank"),
    revisionNo: 1,
    updatedAt: new Date(Date.parse(now) - 1000).toISOString()
  } as Record<string, unknown>
  let pending: Array<Record<string, unknown>> = []
  const skills: Array<Record<string, unknown>> = []
  let updateCount = 0
  let draftDeleted = false
  let acceptedInventoryItem: Record<string, unknown> | null = null
  let inventoryFetchCount = 0
  let forgeRequestCount = 0
  const forgeKeys: string[] = []
  const updatedDraftIDs: string[] = []
  let rejectDraftUpdates = false

  const preview = () => ({
    valid: true,
    blockingReasons: [],
    compiledSkill: {
      id: "custom:pending",
      nameKey: "skillForge.customName",
      descKey: "skillForge.customDesc",
      nameParams: {
        sourceSkillId: official ? "power_strike" : "",
        materialKey: "item.silkwood.name",
        effectKey: "skillForge.name.damage",
        traitKey: "skillForge.name.damage"
      },
      category: "alchemy",
      icon: "item.silkwood",
      hpCost: 0,
      mpCost: 2,
      cooldown: 1,
      statusRequirement: {},
      weaponRequirement: [],
      effects: [{ kind: "damage", targeting: { targetScope: "enemy", targetPositions: ["vanguard"], targetCount: { base: 1, formula: [] } }, amountFormula: { baseAdd: 0, terms: [{ attrKey: "dexterity", coefficient: 0.13, perSkillLevel: 0 }] } }]
    },
    ranges: [{ nodeId: "material-1", kind: "formula_term", baseMin: 0.08, baseMax: 0.176, min: 0.1, max: 0.22 }],
    recipe: { silkwood: 200, spell_seal_lock_plate: 10, final_volume_key_sigil: 5, gold_coin: 50000 },
    budgetUsed: 0.88,
    budgetLimit: 1.12,
    budgetRefund: 0,
    budgetMultiplier: 1,
    failChance: official ? 0.24 : 0.74,
    baseSuccessChance: 0.30,
    materialSuccessModifier: -0.04,
    baseFailChance: 0.74,
    skillbookBonus: official ? 0.50 : 0,
    finalFailChance: official ? 0.24 : 0.74,
    goldCost: 50000,
    goldCostBreakdown: { base: 10000, materials: 4000, budget: 36000 },
    compilerVersion: 10,
    rulesetVersion: 1,
    graphHash: "a".repeat(64)
  })

  await page.addInitScript(() => {
    localStorage.setItem("locale", "zh-CN")
    localStorage.setItem("token", "e2e-token")
    localStorage.setItem("userId", "42")
    localStorage.setItem("username", "炼金测试员")
    localStorage.setItem("player-settings:42", JSON.stringify({ language: "zh-CN", theme: "light", menuLayout: "icon-rail", simpleMode: true, emailVerified: true }))
  })

  await page.routeWebSocket(/\/ws(?:\?|$)/, (socket) => {
    socket.onMessage((message) => {
      if (typeof message !== "string") return
      const event = JSON.parse(message) as { type?: string }
      if (event.type === "auth.bind") {
        socket.send(JSON.stringify({ type: "auth.bound", ts: Date.now(), data: {} }))
      }
      if (event.type === "action.offline.rewards.get") {
        socket.send(JSON.stringify({ type: "action.offline.rewards.result", ts: Date.now(), data: { hasMoreDue: false, rewards: [] } }))
      }
    })
  })

  await page.route("**/api/**", async (route) => {
    const request = route.request()
    const url = new URL(request.url())
    const path = decodeURIComponent(url.pathname)
    const fulfill = (data: unknown, status = 200) => route.fulfill({ status, contentType: "application/json", body: JSON.stringify({ ok: true, ts: Date.now(), data }) })
    if (path === "/api/bootstrap/init") {
      return fulfill({
        playerStats: {}, inventory: { items: [] },
        chatConfig: { channels: [], defaultChannel: "world" }, chatHistory: { items: [] },
        playerSettings: { username: "炼金测试员", language: "zh-CN", theme: "light", menuLayout: "icon-rail", simpleMode: true, emailVerified: true },
        tutorialProgress: {}, playerPresence: { status: "online" }, systemVersion: {},
        recycleConfig: { goldCoefficient: 0.5, affixMismatchExtraCoefficient: 0.6 }
      })
    }
    if (path === "/api/alchemy/discoveries") return fulfill({ groups: [], traits: [], combos: [] })
    if (path === "/api/alchemy/skill-forge/state") {
      return fulfill({
        modules: [moduleDefinition(), conditionModuleDefinition(), effectSlotModuleDefinition(), statusApplyModuleDefinition(), statusDurationModuleDefinition()],
        limits: { compilerVersion: 10, rulesetVersion: 1, disabledRulesetVersions: [], baseSuccessChance: 0.30, sourceSkillbookMinCount: 200, sourceSkillbookMaxCount: 1000, sourceSkillbookCountStep: 100, sourceSkillbookBonusPerStep: 0.10, sourceSkillbookMaxBonus: 0.80, statusBaseApplyChance: 0.25, statusApplyChancePerStep: 0.1, maxCustomSkills: 12, maxDrafts: 4, maxPendingResultsPerDraft: 1, maxNodes: 64, maxEdges: 96, maxEffects: 3, maxFormulaTermsPerEffect: 4, maxTargetCount: 8, maxHitCount: 8, minCooldown: 1, maxCooldown: 12, maxGraphBytes: 131072, maxTradeoffRefundRatio: 0.35, customizedNodeSealItemId: "spell_seal_lock_plate", customizedNodeSealCount: 10, forgeCatalystItemId: "final_volume_key_sigil", forgeCatalystCount: 5, goldCostItemId: "gold_coin", goldCostBase: 10000, goldCostPerMaterial: 20, goldCostPerBudget: 5000 },
        derived: { level: 30, precision: 0.25, stability: 0.2 },
        inventory: [{ itemId: "silkwood", quantity: 800 }, { itemId: "silver_vein_silk", quantity: 300 }, { itemId: "harmonic_embryo", quantity: 200 }, { itemId: "sunward_resin", quantity: 800 }, { itemId: "frost_magic_scroll", quantity: 600 }, { itemId: "spell_seal_lock_plate", quantity: 30 }, { itemId: "final_volume_key_sigil", quantity: 10 }, { itemId: "gold_coin", quantity: 100000 }], drafts: draftDeleted ? [] : [draft, ...(includeSecondDraft ? [secondDraft] : [])], skills, pendingRevisions: pending, ownedSourceIds: ["power_strike"],
        sourceSkillbooks: [{ itemId: "skillbook_power_strike", skillId: "power_strike", rarity: "common", iconId: "item.skillbook", quantity: 1000 }]
      })
    }
    if (path.endsWith("/preview")) return fulfill(preview())
    if (path.includes("/drafts/draft:1") && request.method() === "DELETE") {
      draftDeleted = true
      return fulfill({ ok: true })
    }
    if (path.includes("/drafts/draft:") && request.method() === "PUT") {
      const body = request.postDataJSON() as { graph: unknown; sourceSkillbookCount?: number }
      const targetID = path.includes("draft:2") ? "draft:2" : "draft:1"
      updatedDraftIDs.push(targetID)
      updateCount += 1
      if (rejectDraftUpdates) {
        return route.fulfill({ status: 409, contentType: "application/json", body: JSON.stringify({ ok: false, error: { code: "SKILL_FORGE_REVISION_CONFLICT", message: "save rejected" } }) })
      }
      if (targetID === "draft:2") {
        secondDraft = { ...secondDraft, graph: body.graph, sourceSkillbookCount: body.sourceSkillbookCount || 0, revisionNo: Number(secondDraft.revisionNo) + 1, updatedAt: now }
        return fulfill(secondDraft)
      }
      draft = { ...draft, graph: body.graph, sourceSkillbookCount: body.sourceSkillbookCount || 0, revisionNo: Number(draft.revisionNo) + 1, updatedAt: now }
      return fulfill(draft)
    }
    if (path.endsWith("/forge")) {
      forgeRequestCount += 1
      forgeKeys.push(request.headers()["x-idempotency-key"] || "")
      const body = request.postDataJSON() as { expectedRevision?: number; expectedGraphHash?: string }
      if (body.expectedRevision !== draft.revisionNo || body.expectedGraphHash !== preview().graphHash) {
        return route.fulfill({ status: 409, contentType: "application/json", body: JSON.stringify({ ok: false, error: { code: "SKILL_FORGE_REVISION_CONFLICT", message: "stale draft" } }) })
      }
      if (duplicateForgeOnce && forgeRequestCount === 1) {
        return route.fulfill({
          status: 409,
          contentType: "application/json",
          body: JSON.stringify({ ok: false, ts: Date.now(), error: { code: "REQUEST_DUPLICATE", message: "duplicate request" } })
        })
      }
      const result = {
        id: "revision:1", customSkillId: "custom:11111111-1111-4111-8111-111111111111", draftId: "draft:1", revisionNo: 1,
        compiledSkill: preview().compiledSkill, recipe: { silkwood: 200, skillbook_power_strike: 700 }, requestedRecipe: { silkwood: 200, skillbook_power_strike: 700 }, consumedRecipe: { silkwood: 100, skillbook_power_strike: 350 }, refundedRecipe: { silkwood: 100, skillbook_power_strike: 350 }, budget: { used: 0.88, limit: 1.12, refund: 0, failChance: 0.24 },
        rolls: [{ nodeId: "material-1", kind: "formula_term", baseMin: 0.08, baseMax: 0.176, min: 0.1, max: 0.22, rolled: 0.1 }],
        failed: forgeFails, resultStatus: forgeFails ? "failed" : "pending", compilerVersion: 10, createdAt: now
      }
      if (!forgeFails) {
        pending = [result]
        draft = { ...draft, customSkillId: result.customSkillId, pendingRevisionId: result.id }
      }
      return fulfill(result)
    }
    if (path.endsWith("/accept")) {
      const accepted = { ...pending[0], resultStatus: "accepted", forgedSkillbook: { id: "forged-book:1", instanceId: 99, customSkillId: pending[0]?.customSkillId, revisionId: pending[0]?.id, sourceSkillId: "power_strike", status: "inventory", compiledSkill: pending[0]?.compiledSkill, budget: pending[0]?.budget } }
      acceptedInventoryItem = {
        instanceId: 99,
        templateItemId: "forged_skillbook",
        quantity: 1,
        enhanceLevel: 0,
        durability: 0,
        maxDurability: 0,
        state: "inventory",
        forgedSkillBook: accepted.forgedSkillbook
      }
      pending = []
      draft = { ...draft, pendingRevisionId: undefined }
      skills.push({ id: accepted.customSkillId, sourceKind: "blank", currentRevisionId: accepted.id, archived: false, compiledSkill: accepted.compiledSkill, budget: accepted.budget, createdAt: now, updatedAt: now })
      return fulfill(accepted)
    }
    if (path === "/api/inventory/instances") {
      inventoryFetchCount += 1
      return fulfill({ items: acceptedInventoryItem ? [acceptedInventoryItem] : [] })
    }
    return fulfill({})
  })

  return {
    updateCount: () => updateCount,
    draftDeleted: () => draftDeleted,
    inventoryFetchCount: () => inventoryFetchCount,
    forgeRequestCount: () => forgeRequestCount,
    forgeKeys: () => forgeKeys.slice(),
    updatedDraftIDs: () => updatedDraftIDs.slice(),
    rejectDraftUpdates: (reject: boolean) => { rejectDraftUpdates = reject }
  }
}

test("edits, autosaves, undoes, reloads, and keeps a failed skillbook draft", async ({ page }) => {
  await page.setViewportSize({ width: 1440, height: 900 })
  const pageErrors: string[] = []
  page.on("pageerror", (error) => pageErrors.push(error.message))
  const api = await mockSkillForge(page, true)
  await page.goto("/game/alchemy?tab=skill-forge")
  await expect(page.getByTestId("skill-forge-panel")).toBeVisible({ timeout: 20_000 })
  await expect(page.getByTestId("forge-node-material")).toHaveCount(1)
  await expect(page.locator(".preview-metrics > div").filter({ hasText: "基础成功率" })).toContainText("30.0%")
  await expect(page.locator(".preview-metrics > div").filter({ hasText: "素材修正" })).toContainText("-4.0%")
  await expect(page.locator(".preview-metrics > div").filter({ hasText: "技能书加成" })).toContainText("+50.0%")
  await expect(page.locator(".preview-metrics > div").filter({ hasText: "最终成功率" })).toContainText("76.0%")
  await expect(page.locator(".module-row__chance").first()).toHaveText("每份成功率 -4.0%")
  await expect(page.getByText("0.08–0.18 → 0.10–0.22")).toBeVisible()
  await expect(page.getByText("0.13×敏捷")).toBeVisible()
  await expect(page.getByRole("heading", { name: "最终技能效果" })).toBeVisible()
  await expect(page.locator(".final-skill-preview").getByText("冷却:", { exact: true })).toBeVisible()
  await expect(page.locator(".final-skill-preview .skill-detail-row.is-changed").first()).toBeVisible()
  await expect(page.locator(".final-skill-preview").getByText("原值", { exact: true }).first()).toBeVisible()
  await expect(page.locator(".final-skill-preview").getByText("修改后", { exact: true }).first()).toBeVisible()
  await page.getByText("素材消耗", { exact: true }).first().click()
  await expect(page.getByText("每项参数修改额外消耗 10 个封术锁片；每次炼成至少消耗 5 个终卷钥印。金币按素材数量和硬预算结算，最终冷却不会低于 1 回合。")).toBeVisible()
  await expect(page.getByText("金币消耗")).toBeVisible()
  await expect(page.locator(".recipe-list .item-quick-ref")).toHaveCount(5)
  await page.locator(".recipe-list > div").filter({ hasText: "金币" }).locator(".recipe-usage-help").hover()
  await expect(page.getByText("金币构成：基础 10,000 + 素材数量 4,000 + 硬预算 36,000")).toBeVisible()
  const materialBox = await page.locator(".module-panel").boundingBox()
  const graphBox = await page.locator(".graph-panel").boundingBox()
  const previewBox = await page.locator(".preview-panel").boundingBox()
  expect(materialBox?.x).toBeLessThan(graphBox?.x || 0)
  expect(previewBox?.x).toBeGreaterThan(graphBox?.x || 0)

  const pageScroller = page.locator(".game-content-scroll")
  const previewPanel = page.locator(".preview-panel")
  const outerOverflow = await pageScroller.evaluate((element) => element.scrollHeight - element.clientHeight)
  expect(outerOverflow).toBeLessThanOrEqual(1)
  await previewPanel.hover()
  await page.mouse.wheel(0, 900)
  await expect.poll(() => previewPanel.evaluate((element) => element.scrollTop)).toBeGreaterThan(0)
  expect(await pageScroller.evaluate((element) => element.scrollTop)).toBe(0)
  await expect(page.locator(".game-page-header")).toBeInViewport()
  await expect(page.locator(".skill-forge__toolbar")).toBeInViewport()
  await previewPanel.evaluate((element) => { element.scrollTop = 0 })
  if (process.env.SKILL_FORGE_SCREENSHOT_DIR) {
    await page.screenshot({ path: `${process.env.SKILL_FORGE_SCREENSHOT_DIR}/skill-forge-desktop.png`, fullPage: true })
  }

  const moduleRow = page.locator(".module-row").first()
  const canvas = page.locator(".graph-panel")
  await moduleRow.evaluate((source) => {
    const transfer = new DataTransfer()
    source.dispatchEvent(new DragEvent("dragstart", { bubbles: true, dataTransfer: transfer }))
    const canvas = document.querySelector<HTMLElement>(".graph-panel")
    if (!canvas) throw new Error("skill forge canvas is missing")
    canvas.dispatchEvent(new DragEvent("dragover", { bubbles: true, cancelable: true, dataTransfer: transfer }))
    canvas.dispatchEvent(new DragEvent("drop", { bubbles: true, cancelable: true, dataTransfer: transfer, clientX: 640, clientY: 420 }))
  })
  expect(pageErrors).toEqual([])
  await expect(page.getByTestId("forge-node-material")).toHaveCount(2)
  await expect.poll(api.updateCount).toBeGreaterThan(0)
  await page.locator(".forge-node--material .forge-node__remove").last().click()
  await expect(page.getByTestId("forge-node-material")).toHaveCount(1)
  await page.getByRole("button", { name: "撤销" }).click()
  await expect(page.getByTestId("forge-node-material")).toHaveCount(2)
  await page.getByRole("button", { name: "撤销" }).click()
  await expect(page.getByTestId("forge-node-material")).toHaveCount(1)
  await expect.poll(api.updateCount).toBeGreaterThan(1)

  await page.reload()
  await expect(page.getByTestId("forge-node-material")).toHaveCount(1)
  await page.getByRole("button", { name: "立即炼成" }).click()
  await expect(page.getByRole("heading", { name: "优化失败" })).toBeVisible()
  await expect(page.getByText("消耗 350 / 返还 350")).toBeVisible()
  await expect(page.getByRole("button", { name: "接纳并铸书" })).toHaveCount(0)
})

test("picks official source books from the item browser and allows deleting the current design", async ({ page }) => {
  const api = await mockSkillForge(page, true)
  await page.goto("/game/alchemy?tab=skill-forge")
  await expect(page.getByTestId("skill-forge-panel")).toBeVisible({ timeout: 20_000 })

  await page.getByRole("button", { name: "选择技能来源" }).click()
  const dialog = page.getByRole("dialog", { name: "选择技能来源" })
  await dialog.getByText("技能书优化", { exact: true }).click()
  await expect(dialog.locator(".source-book-picker")).toBeVisible()
  await expect(dialog.locator(".source-book-tile")).toHaveCount(1)
  await expect(dialog.getByText("强力打击技能书", { exact: true })).toBeVisible()
  await dialog.locator(".source-book-tile").click()
  await expect(dialog.locator(".source-book-tile.is-selected")).toHaveCount(1)
  await expect(dialog.locator(".source-book-tile")).toContainText("强力打击")
  await dialog.getByRole("button", { name: "取消" }).click()

  await page.getByRole("button", { name: "删除设计" }).click()
  await page.getByRole("button", { name: "确认" }).click()
  await expect.poll(api.draftDeleted).toBe(true)
  await expect(page.getByText("选择空白设计，或投入官方技能书开始优化。", { exact: true })).toBeVisible()
})

test("keeps the node editor usable on a mobile viewport", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 })
  await mockSkillForge(page)
  await page.goto("/game/alchemy?tab=skill-forge")
  await expect(page.getByTestId("skill-forge-panel")).toBeVisible({ timeout: 20_000 })
  await expect(page.locator(".graph-panel")).toBeVisible()
  await expect(page.locator(".module-panel")).toBeHidden()
  await expect(page.locator(".preview-panel")).toBeHidden()
  await expect(page.locator(".forge-node--parameter")).toHaveCount(15)
  await expect(page.locator(".forge-node--parameter").filter({ hasText: "施加状态与概率" })).toBeVisible()
  await expect(page.locator(".forge-node--parameter").filter({ hasText: "索敌方向" })).toBeVisible()
  await page.getByText("选素材", { exact: true }).click()
  await expect(page.locator(".module-panel")).toBeVisible()
  await expect(page.locator(".graph-panel")).toBeHidden()
  await page.getByText("看结果", { exact: true }).click()
  await expect(page.locator(".preview-panel")).toBeVisible()
  await expect(page.getByText("空白设计仅用于编辑与权威预览，不能提交炼成。")).toBeVisible()
  await expect(page.getByRole("button", { name: "立即炼成" })).toHaveCount(0)
  if (process.env.SKILL_FORGE_SCREENSHOT_DIR) {
    await page.screenshot({ path: `${process.env.SKILL_FORGE_SCREENSHOT_DIR}/skill-forge-mobile.png`, fullPage: true })
  }
})

test("accepts a successful result as a bound skillbook instead of learning it", async ({ page }) => {
  const api = await mockSkillForge(page, true, false)
  await page.goto("/game/alchemy?tab=skill-forge")
  await expect(page.getByTestId("skill-forge-panel")).toBeVisible({ timeout: 20_000 })
  const inventoryFetchesBeforeAccept = api.inventoryFetchCount()
  await expect(page.getByText("技能书加成", { exact: true })).toBeVisible()
  await page.getByRole("button", { name: "立即炼成" }).click()
  await expect(page.getByRole("heading", { name: "炼成成功" })).toBeVisible()
  await page.getByRole("button", { name: "接纳并铸书" }).click()
  await expect(page.getByRole("heading", { name: "改造技能书已铸成" })).toBeVisible()
  await expect(page.getByText("成品已进入背包，可选择玩家或一只所属猫咪学习。")).toBeVisible()
  await expect.poll(api.inventoryFetchCount).toBeGreaterThan(inventoryFetchesBeforeAccept)
  await page.getByText("仓库", { exact: true }).first().click()
  await expect(page.locator(".item-card--skillbook")).toBeVisible()
  await expect(page.getByText(/强力打击.*技能书/, { exact: true })).toBeVisible()
  await expect(page.getByText("绑定改造技能书", { exact: true })).toHaveCount(0)
})

test("recovers from a replay-guard duplicate without an unhandled error", async ({ page }) => {
  const pageErrors: string[] = []
  page.on("pageerror", (error) => pageErrors.push(error.message))
  const api = await mockSkillForge(page, true, false, true)
  await page.goto("/game/alchemy?tab=skill-forge")
  await expect(page.getByTestId("skill-forge-panel")).toBeVisible({ timeout: 20_000 })

  await page.getByRole("button", { name: "立即炼成" }).click()
  await expect(page.getByText("请求已处理，已同步最新炼成结果")).toBeVisible()
  await expect.poll(api.forgeRequestCount).toBe(1)

  await page.getByRole("button", { name: "立即炼成" }).click()
  await expect(page.getByRole("heading", { name: "炼成成功" })).toBeVisible()
  await expect.poll(api.forgeRequestCount).toBe(2)
  expect(api.forgeKeys()[0]).not.toBe(api.forgeKeys()[1])
  expect(pageErrors).toEqual([])
})

test("flushes the original draft snapshot before switching during autosave debounce", async ({ page }) => {
  const api = await mockSkillForge(page, true, true, false, true)
  await page.goto("/game/alchemy?tab=skill-forge")
  await expect(page.getByTestId("skill-forge-panel")).toBeVisible({ timeout: 20_000 })

  await page.locator(".forge-node--material .forge-node__remove").first().dispatchEvent("click")
  await expect(page.getByText("等待自动保存", { exact: true })).toBeVisible()
  await page.locator(".draft-select").click()
  await page.getByRole("option", { name: "空白设计" }).click()

  await expect.poll(() => api.updatedDraftIDs()[0]).toBe("draft:1")
  await expect(page.locator(".draft-select")).toContainText("空白设计")
})

test("blocks forging after an autosave failure until the draft is saved", async ({ page }) => {
  const api = await mockSkillForge(page, true, true)
  api.rejectDraftUpdates(true)
  await page.goto("/game/alchemy?tab=skill-forge")
  await expect(page.getByTestId("skill-forge-panel")).toBeVisible({ timeout: 20_000 })

  await page.locator(".forge-node--material .forge-node__remove").first().dispatchEvent("click")
  await expect(page.getByText("设计保存失败", { exact: true })).toBeVisible({ timeout: 5_000 })
  await expect(page.getByRole("button", { name: "立即炼成" })).toBeDisabled()
  expect(api.forgeRequestCount()).toBe(0)

  api.rejectDraftUpdates(false)
  await page.getByRole("button", { name: "重试保存" }).click()
  await expect(page.getByText("已自动保存", { exact: true })).toBeVisible()
})

test("shows condition materials as explicit true and false branch nodes", async ({ page }) => {
  await mockSkillForge(page)
  await page.goto("/game/alchemy?tab=skill-forge")
  await expect(page.getByTestId("skill-forge-panel")).toBeVisible({ timeout: 20_000 })
  const conditionRow = page.locator(".module-row").filter({ hasText: "银纹绸" })
  await conditionRow.hover()
  await expect(page.locator(".module-inspector").getByText("判断目标状态 · 护甲破碎 · 分出满足/否则两路", { exact: true })).toBeVisible()

  await conditionRow.evaluate((source) => {
    const transfer = new DataTransfer()
    source.dispatchEvent(new DragEvent("dragstart", { bubbles: true, dataTransfer: transfer }))
    const canvas = document.querySelector<HTMLElement>(".graph-panel")
    if (!canvas) throw new Error("skill forge canvas is missing")
    canvas.dispatchEvent(new DragEvent("dragover", { bubbles: true, cancelable: true, dataTransfer: transfer }))
    canvas.dispatchEvent(new DragEvent("drop", { bubbles: true, cancelable: true, dataTransfer: transfer, clientX: 600, clientY: 430 }))
  })
  await expect(page.getByText("满足时", { exact: true })).toBeVisible()
  await expect(page.getByText("否则", { exact: true })).toBeVisible()
})

test("filters compatible materials and keeps stage and material chains connected", async ({ page }) => {
  await mockSkillForge(page, true)
  await page.goto("/game/alchemy?tab=skill-forge")
  await expect(page.getByTestId("skill-forge-panel")).toBeVisible({ timeout: 20_000 })
  await expect(page.locator(".vue-flow__edge").first()).toBeVisible()
  await expect(page.locator(".forge-node--parameter").filter({ hasText: "索敌方向" })).toBeVisible()
  await expect(page.locator(".forge-node--parameter").filter({ hasText: "最大目标数量" })).toBeVisible()
  const edgeStroke = await page.locator(".vue-flow__edge-path").first().evaluate((path) => getComputedStyle(path).stroke)
  expect(edgeStroke).not.toBe("none")
  expect(edgeStroke).not.toBe("transparent")

  await page.locator(".forge-node--parameter").filter({ hasText: "数值公式" }).dispatchEvent("click")
  await expect(page.getByText("已只显示可修正此参数的素材；点击素材即挂载，点击画布空白处恢复全部。")).toBeVisible()
  await expect(page.locator(".module-row")).toHaveCount(1)
  await expect(page.locator(".module-row").filter({ hasText: "丝纹木" })).toBeVisible()
  await expect(page.locator(".module-row").filter({ hasText: "银脉丝" })).toHaveCount(0)

  await page.locator(".module-row").click()
  await expect(page.getByTestId("forge-node-material")).toHaveCount(2)
  const materialEdge = page.getByRole("group", { name: /Edge from material-1 to material-/ })
  await expect(materialEdge).toHaveCount(1)
  await expect(materialEdge.locator("path.vue-flow__edge-path")).toHaveAttribute("d", /\S+/)
})

test("uses a highest-rarity material to unlock effect two and three without an add-effect command", async ({ page }) => {
  await mockSkillForge(page, true)
  await page.goto("/game/alchemy?tab=skill-forge")
  await expect(page.getByTestId("skill-forge-panel")).toBeVisible({ timeout: 20_000 })
  await expect(page.getByRole("button", { name: "添加效果" })).toHaveCount(0)

  const slotRow = page.locator(".module-row").filter({ hasText: "调和核心" })
  await slotRow.hover()
  await expect(page.locator(".module-inspector").getByText("增加一个效果槽", { exact: true })).toBeVisible()

  for (const y of [380, 560]) {
    await slotRow.evaluate((source, dropY) => {
      const transfer = new DataTransfer()
      source.dispatchEvent(new DragEvent("dragstart", { bubbles: true, dataTransfer: transfer }))
      const canvas = document.querySelector<HTMLElement>(".graph-panel")
      if (!canvas) throw new Error("skill forge canvas is missing")
      canvas.dispatchEvent(new DragEvent("dragover", { bubbles: true, cancelable: true, dataTransfer: transfer }))
      canvas.dispatchEvent(new DragEvent("drop", { bubbles: true, cancelable: true, dataTransfer: transfer, clientX: 620, clientY: dropY }))
    }, y)
  }

  await expect(page.getByTestId("forge-node-effect")).toHaveCount(3)
  await expect(page.getByTestId("forge-node-material")).toHaveCount(2)
  await expect(page.getByTestId("forge-node-parameter").filter({ hasText: "目标阵营" })).toHaveCount(3)
  await expect(page.getByTestId("forge-node-parameter").filter({ hasText: "索敌方向" })).toHaveCount(3)
  await expect(page.getByTestId("forge-node-parameter").filter({ hasText: "索敌规则" })).toHaveCount(3)

	const secondEffect = page.getByTestId("forge-node-effect").nth(1)
	const secondStatusApply = page.getByTestId("forge-node-parameter").filter({ hasText: "施加状态与概率" }).first()
	const secondStatusDuration = page.getByTestId("forge-node-parameter").filter({ hasText: "状态持续" }).first()
	await expect(secondStatusApply).toContainText("可挂 1 种素材")
	await expect(secondStatusDuration).toContainText("可挂 1 种素材")
	await secondStatusApply.dispatchEvent("click")
	const statusMaterial = page.locator(".module-row").filter({ hasText: /sunward_resin|向阳树脂/ })
	await expect(statusMaterial).toBeVisible()
	await statusMaterial.click()
	await expect(secondEffect.locator(".effect-kind-select")).toContainText("状态")
	await expect(page.getByTestId("forge-node-material")).toHaveCount(3)

	const thirdEffect = page.getByTestId("forge-node-effect").nth(2)
	await thirdEffect.locator(".effect-kind-select").evaluate((select) => {
    const trigger = select.querySelector<HTMLElement>(".el-select__wrapper")
    if (!trigger) throw new Error("effect kind selector trigger is missing")
    trigger.click()
  })
  await page.getByRole("option", { name: "治疗" }).click()
  await expect(thirdEffect.locator(".effect-kind-select")).toContainText("治疗")
})
