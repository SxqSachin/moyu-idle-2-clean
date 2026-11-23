import { readdirSync, readFileSync } from "node:fs"
import { fileURLToPath } from "node:url"
import { expect, test, type Page } from "@playwright/test"

const sourceDir = fileURLToPath(new URL("../../packages/game-data/source/json/", import.meta.url))
function sourceFileForSpecializationE2E(name: string, text: string) {
  if (name !== "equipment_templates.json") return text
  const parsed = JSON.parse(text)
  const unrelatedTemplate = parsed.equipment_templates.find((row: { id?: string }) => row.id === "last_light_of_the_parade")
  const unrelatedHook = unrelatedTemplate?.battleHooks?.find((hook: { id?: string }) => hook.id === "last_light_of_the_parade_fourth_turn")
  if (unrelatedHook) unrelatedHook.conditions = []
  return JSON.stringify(parsed)
}
const sourceFiles = Object.fromEntries(
  readdirSync(sourceDir)
    .filter((name) => name.endsWith(".json"))
    .map((name) => [name, sourceFileForSpecializationE2E(name, readFileSync(`${sourceDir}/${name}`, "utf8"))])
)
const persistedSpecializationKey = "combat-model-e2e:specialization_trees.json"
const persistedItemsKey = "combat-model-e2e:items.json"

async function installGameDataDirectory(page: Page) {
  await page.addInitScript((files) => {
    const notFound = () => new DOMException("Not found", "NotFoundError")
    const jsonDirectory = {
      name: "json",
      async getFileHandle(name: string, options?: { create?: boolean }) {
        const sourceText = files[name]
        const persistedKey = name === "specialization_trees.json" || name === "items.json" ? `combat-model-e2e:${name}` : ""
        const text = persistedKey ? window.localStorage.getItem(persistedKey) ?? sourceText : sourceText
        if (typeof text !== "string" && !options?.create) throw notFound()
        return {
          name,
          async getFile() {
            return new File([text ?? ""], name, { type: "application/json" })
          },
          async createWritable() {
            let pending = ""
            return {
              async write(data: string) { pending = String(data) },
              async close() {
                if (persistedKey) window.localStorage.setItem(persistedKey, pending)
              }
            }
          }
        }
      }
    }
    const sourceDirectory = {
      name: "source",
      async getDirectoryHandle(name: string) {
        if (name !== "json") throw notFound()
        return jsonDirectory
      }
    }
    const root = {
      name: "game-data",
      async queryPermission() { return "granted" as PermissionState },
      async requestPermission() { return "granted" as PermissionState },
      async getDirectoryHandle(name: string) {
        if (name !== "source") throw notFound()
        return sourceDirectory
      }
    }
    Object.defineProperty(window, "showDirectoryPicker", {
      configurable: true,
      value: async () => root
    })
  }, sourceFiles)
}

async function openSpecializationDesigner(page: Page) {
  await installGameDataDirectory(page)
  await page.goto("/")
  await page.getByRole("button", { name: /授权并加载 game-data/ }).click()
  const specializationNav = page.getByRole("button", { name: /专精树 1/ })
  await expect(specializationNav).toBeVisible()
  await specializationNav.click()
  await expect(page.locator(".specialization-workspace")).toBeVisible()
  await expect(page.locator(".spec-node")).toHaveCount(70)
  await expect(page.locator(".spec-node.hybrid")).toHaveCount(20)
  await expect(page.locator(".issue-list header span")).toHaveText("0")
}

test("authors the canonical v6 ranks, root-route limit, keyword copy, and topology", async ({ page }) => {
  test.setTimeout(90_000)
  await page.setViewportSize({ width: 1728, height: 1000 })
  const pageErrors: string[] = []
  page.on("pageerror", (error) => pageErrors.push(error.message))
  await openSpecializationDesigner(page)

  const canonicalTable = JSON.parse(sourceFiles["specialization_trees.json"]!)
  const canonicalTree = canonicalTable.specialization_trees[0]
  const canonicalRootCopy = canonicalTree.i18n["zh-CN"].nodes.strength_root_break_stance

  await expect(page.locator(".spec-node strong", { hasText: "蓄势重击" })).toHaveCount(1)
  await expect(page.locator(".spec-node strong", { hasText: "双持架势" })).toHaveCount(1)
  await expect(page.locator(".spec-node strong", { hasText: "strength_root_break_stance" })).toHaveCount(0)
  await expect(page.getByLabel("schemaVersion", { exact: true })).toHaveValue("2")
  await expect(page.getByLabel("rulesetVersion", { exact: true })).toHaveValue("specialization-v6")
  await expect(page.getByLabel("点数上限", { exact: true })).toHaveValue("65")
  await expect(page.getByLabel("根路线上限", { exact: true })).toHaveValue("3")
  await page.getByLabel("规则编辑").check()

  await page.locator(".spec-node", { hasText: "绝境战意" }).click()
  const enCopy = page.locator("fieldset.node-locale-editor").filter({ hasText: "en-US 玩家文案" })
  await expect(enCopy.locator(".summary-line")).toHaveCount(3)
  await expect(enCopy.locator(".summary-line textarea").nth(0)).toHaveValue(/^Trigger:/)
  await expect(enCopy.locator(".summary-line textarea").nth(1)).toHaveValue(/^Condition:/)
  await expect(enCopy.locator(".summary-line textarea").nth(2)).toHaveValue(/^Effect:/)
  await expect(page.locator(".issue-list header span")).toHaveText("0")

  await page.locator(".spec-node", { hasText: "蓄势重击" }).click()
  await expect(page.locator(".inspector-head")).toContainText("6 级")
  const zhCopy = page.locator("fieldset.node-locale-editor").filter({ hasText: "zh-CN 玩家文案" })
  const zhName = zhCopy.getByLabel("名称", { exact: true })
  const zhDesc = zhCopy.getByLabel("风味", { exact: true })
  const editedName = "蓄势重击·往返验证"
  const editedDesc = "先积蓄力量，把爆发留给真正关键的一击。"
  const editedSummary = "[[keyword:break_momentum]] 每完成两次物理命中，下一次物理伤害会兑现攒下的力气。"
  await zhName.fill(editedName)
  await zhName.press("Tab")
  await zhDesc.fill(editedDesc)
  await zhDesc.press("Tab")
  await zhCopy.locator(".summary-line textarea").first().fill(editedSummary)
  await zhCopy.locator(".summary-line textarea").first().press("Tab")
  await expect(page.locator(".spec-node strong", { hasText: editedName })).toHaveCount(1)

  await page.getByRole("button", { name: "保存 source/json" }).click()
  await expect(page.getByText("已保存到 source/json", { exact: true })).toBeVisible()
  const persisted = await page.evaluate((key) => JSON.parse(window.localStorage.getItem(key) || "null"), persistedSpecializationKey)
  const persistedTree = persisted.specialization_trees[0]
  expect(persistedTree.schemaVersion).toBe(2)
  expect(persistedTree.rulesetVersion).toBe("specialization-v6")
  expect(persistedTree.maxPoints).toBe(65)
  expect(persistedTree.maxActiveRootNodes).toBe(3)
  expect(persistedTree.i18n["zh-CN"].nodes.strength_root_break_stance.name).toBe(editedName)
  expect(persistedTree.i18n["zh-CN"].nodes.strength_root_break_stance.desc).toBe(editedDesc)
  expect(persistedTree.i18n["zh-CN"].nodes.strength_root_break_stance.effectSummary[0]).toBe(editedSummary)
  expect(persistedTree.i18n["zh-CN"].nodes.strength_root_break_stance.levelEffectSummary).toHaveLength(6)
  expect(persistedTree.i18n["zh-CN"].nodes.strength_root_break_stance.levelEffectSummary
    .every((rank: string[]) => rank.some((line) => line.includes("[[keyword:break_momentum]]")))).toBe(true)
  expect(persistedTree.keywords).toHaveLength(23)
  expect("migrations" in persistedTree).toBe(false)

  await openSpecializationDesigner(page)
  await page.getByLabel("规则编辑").check()
  await page.locator(".spec-node", { hasText: editedName }).click()
  const reloadedZhCopy = page.locator("fieldset.node-locale-editor").filter({ hasText: "zh-CN 玩家文案" })
  await expect(reloadedZhCopy.getByLabel("名称", { exact: true })).toHaveValue(editedName)
  await expect(reloadedZhCopy.getByLabel("风味", { exact: true })).toHaveValue(editedDesc)
  await expect(reloadedZhCopy.locator(".summary-line textarea").first()).toHaveValue(editedSummary)

  await reloadedZhCopy.getByLabel("名称", { exact: true }).fill(canonicalRootCopy.name)
  await reloadedZhCopy.getByLabel("名称", { exact: true }).press("Tab")
  await reloadedZhCopy.getByLabel("风味", { exact: true }).fill(canonicalRootCopy.desc)
  await reloadedZhCopy.getByLabel("风味", { exact: true }).press("Tab")
  await reloadedZhCopy.locator(".summary-line textarea").first().fill(canonicalRootCopy.effectSummary[0])
  await reloadedZhCopy.locator(".summary-line textarea").first().press("Tab")
  await page.getByRole("button", { name: "保存 source/json" }).click()
  await expect(page.getByText("已保存到 source/json", { exact: true })).toBeVisible()
  await page.screenshot({ path: "output/playwright/combat-model-specialization-editor-copy.png", fullPage: true })

  await page.locator(".spec-node", { hasText: "不屈防线" }).click()
  await page.getByRole("button", { name: "前置" }).click()
  await expect(page.locator(".requirements-editor")).toContainText("pure_school_points_gte")
  const pureSchoolGatePoints = await page.locator(".requirements-editor .requirement-row").evaluateAll((rows) => {
    const row = rows.find((candidate) => (candidate.querySelector("select") as HTMLSelectElement | null)?.value === "pure_school_points_gte")
    return (row?.querySelector("input[type=number]") as HTMLInputElement | null)?.value ?? ""
  })
  expect(pureSchoolGatePoints).toBe("11")
  const terminalParentRanks = await page.locator(".requirements-editor .requirement-row").evaluateAll((rows) => rows
    .filter((candidate) => (candidate.querySelector("select") as HTMLSelectElement | null)?.value === "node_rank")
    .map((candidate) => (candidate.querySelector("input[type=number]") as HTMLInputElement | null)?.value ?? ""))
  expect(terminalParentRanks).toEqual(["4", "4"])

  await page.locator(".spec-node", { hasText: "血铸强袭" }).click()
  await page.getByRole("button", { name: "前置" }).click()
  const rootPrerequisiteRank = await page.locator(".requirements-editor .requirement-row").evaluateAll((rows) => {
    const row = rows.find((candidate) => (candidate.querySelector("select") as HTMLSelectElement | null)?.value === "node_rank")
    return (row?.querySelector("input[type=number]") as HTMLInputElement | null)?.value ?? ""
  })
  expect(rootPrerequisiteRank).toBe("6")
  await page.getByRole("button", { name: "Levels", exact: true }).click()
  await expect(page.locator(".level-row")).toHaveCount(4)
  await page.locator(".level-row").nth(3).getByTitle("删除等级").click()
  await expect(page.locator(".level-row")).toHaveCount(3)
  await page.getByRole("button", { name: "新增等级" }).click()
  await expect(page.locator(".level-row")).toHaveCount(4)
  await expect(page.locator(".level-row").nth(3)).toContainText("1 grants")
  await page.getByRole("button", { name: "Grants" }).click()
  await expect(page.locator(".grant-block input").first()).toHaveValue("grant_strength_tech_rupture")
  await page.getByRole("button", { name: "撤销" }).click()
  await page.getByRole("button", { name: "Levels", exact: true }).click()
  await expect(page.locator(".level-row")).toHaveCount(3)
  await page.getByRole("button", { name: "撤销" }).click()
  await expect(page.locator(".level-row")).toHaveCount(4)
  await expect(page.locator(".issue-list header span")).toHaveText("0")

  await page.locator(".spec-node", { hasText: "双持架势" }).click()
  await page.getByRole("button", { name: "槽位变换" }).click()
  await expect(page.locator(".topology-preview")).toContainText("饰品")
  await expect(page.locator(".topology-preview")).toContainText("右手")

  await page.getByRole("button", { name: "+节点" }).click()
  await expect(page.locator(".spec-node")).toHaveCount(71)
  await page.getByRole("button", { name: "撤销" }).click()
  await expect(page.locator(".spec-node")).toHaveCount(70)
  await expect(page.locator(".issue-list header span")).toHaveText("0")

  const overflow = await page.locator("html").evaluate((element) => element.scrollWidth - element.clientWidth)
  expect(overflow).toBeLessThanOrEqual(1)
  expect(pageErrors).toEqual([])
  await page.screenshot({ path: "output/playwright/combat-model-specialization-editor-desktop.png", fullPage: true })

  await page.setViewportSize({ width: 900, height: 1000 })
  await expect(page.locator(".specialization-workspace")).toBeVisible()
  await page.screenshot({ path: "output/playwright/combat-model-specialization-editor-narrow.png", fullPage: true })
})

test("edits targeted longevity feed without feeder-only fields", async ({ page }, testInfo) => {
  test.setTimeout(90_000)
  await page.setViewportSize({ width: 1400, height: 1000 })
  await installGameDataDirectory(page)
  await page.goto('/')
  await page.getByRole('button', { name: /授权并加载 game-data/ }).click()
  await page.getByRole('button', { name: /^物品\s+\d+$/ }).click()
  await page.getByPlaceholder('过滤 id / 名称 / nameKey / JSON 内容').fill('pond_feed_sacred_longevity')
  await page.locator('.row-main').filter({ hasText: 'pond_feed_sacred_longevity' }).click()
  const editor = page.locator('.pond-feed-editor')
  await expect(editor.getByRole('combobox', { name: /投喂方式/ })).toHaveValue('targeted')
  await expect(editor.locator('.effect-row')).toHaveCount(2)
  await expect(editor.getByRole('spinbutton', { name: '每鱼每小时最多份数', exact: true })).toHaveCount(0)
  await editor.locator('.effect-row').first().getByLabel('每份值').fill('730')
  await editor.screenshot({ path: testInfo.outputPath('pond-longevity-editor.png') })
  await page.getByRole('button', { name: '保存 source/json' }).click()
  await expect(page.getByText('已保存到 source/json', { exact: true })).toBeVisible()
  const persisted = await page.evaluate(key => JSON.parse(window.localStorage.getItem(key) || 'null'), persistedItemsKey)
  const feed = persisted.items.find((item: { id: string }) => item.id === 'pond_feed_sacred_longevity').pondFeed
  expect(feed.delivery).toBe('targeted')
  expect(feed.effects[0].valuePerPortion).toBe(730)
  expect(feed.effects[1].valuePerPortion).toBe(1095)
  expect(feed.maxPortionsPerFishHour).toBeUndefined()
  expect(feed.nutrition).toBe(0)
})

test("edits pond-feed-v2 conditions and effects with the structured item editor", async ({ page }, testInfo) => {
  test.setTimeout(90_000)
  await page.setViewportSize({ width: 1728, height: 1000 })
  await installGameDataDirectory(page)
  await page.goto("/")
  await page.getByRole("button", { name: /授权并加载 game-data/ }).click()

  await page.getByRole("button", { name: /^物品\s+\d+$/ }).click()
  await page.getByPlaceholder("过滤 id / 名称 / nameKey / JSON 内容").fill("pond_feed_forced_growth")
  const itemRow = page.locator(".row-main").filter({ hasText: "pond_feed_forced_growth" })
  await expect(itemRow).toHaveCount(1)
  await itemRow.click()

  const editor = page.locator(".pond-feed-editor")
  await expect(editor).toBeVisible()
  await expect(editor.getByText("启用 pond-feed-v2 组合规则", { exact: true })).toBeVisible()
  await expect(editor.getByRole('spinbutton', { name: '消耗优先级', exact: true })).toHaveValue("80")
  await expect(editor.getByRole('spinbutton', { name: '每鱼每小时最多份数', exact: true })).toHaveValue("0.5")
  await expect(editor.locator(".condition-row")).toHaveCount(1)
  await expect(editor.locator(".effect-row")).toHaveCount(3)
  await expect(editor.locator(".effect-row").nth(0)).toContainText("bonus_band")
  await expect(editor.locator(".effect-row").nth(2)).toContainText("flat")
  await editor.screenshot({ path: testInfo.outputPath("pond-feed-dsl-editor.png") })

  await editor.getByRole('spinbutton', { name: '消耗优先级', exact: true }).fill("79")
  await editor.getByRole('spinbutton', { name: '每鱼每小时最多份数', exact: true }).fill("0.4")
  await editor.locator(".condition-row input[type=number]").fill("0.4")
  await editor.locator(".effect-row").nth(0).getByLabel("每份值").fill("0.045")
  await page.getByRole("button", { name: "保存 source/json" }).click()
  await expect(page.getByText("已保存到 source/json", { exact: true })).toBeVisible()
  const persisted = await page.evaluate((key) => JSON.parse(window.localStorage.getItem(key) || "null"), persistedItemsKey)
  const savedFeed = persisted.items.find((item: { id?: string }) => item.id === "pond_feed_forced_growth").pondFeed
  expect(savedFeed.consumePriority).toBe(79)
  expect(savedFeed.maxPortionsPerFishHour).toBe(0.4)
  expect(savedFeed.conditions[0].value).toBe(0.4)
  expect(savedFeed.effects[0].valuePerPortion).toBe(0.045)
})
