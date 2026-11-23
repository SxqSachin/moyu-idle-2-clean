import { expect, test, type Page } from "@playwright/test"

const standardFormat = {
  id: "standard",
  schemaVersion: 2,
  rulesetVersion: "combat-trial-format-v2",
  balanceRevision: 4,
  nameKey: "game.combatTrial.formats.standard.name",
  shortDescKey: "game.combatTrial.formats.standard.shortDesc",
  descKey: "game.combatTrial.formats.standard.desc",
  icon: "lucide:target",
  sortOrder: 0,
  enabled: true,
  lineupRules: { positionPolicy: "shared" },
  actionBudget: 20,
  defenseK: 100,
  metricIds: ["damage_total", "damage_max_segment"],
  metrics: [
    {
      id: "damage_total", schemaVersion: 2,
      nameKey: "game.combatTrial.metrics.damageTotal.name", shortNameKey: "game.combatTrial.metrics.damageTotal.shortName",
      descKey: "game.combatTrial.metrics.damageTotal.desc", eventType: "damage", aggregation: "sum",
      sourceScope: "player_side", targetScope: "objective_units", ranked: true, rankDirection: "desc",
      eligibleEndReasons: ["action_budget_reached", "team_defeated"], valueFormat: "number", sortOrder: 0, enabled: true
    },
    {
      id: "damage_max_segment", schemaVersion: 2,
      nameKey: "game.combatTrial.metrics.damageMaxSegment.name", shortNameKey: "game.combatTrial.metrics.damageMaxSegment.shortName",
      descKey: "game.combatTrial.metrics.damageMaxSegment.desc", eventType: "damage", aggregation: "max",
      sourceScope: "player_side", targetScope: "objective_units", ranked: true, rankDirection: "desc",
      eligibleEndReasons: ["action_budget_reached", "team_defeated"], valueFormat: "number", sortOrder: 10, enabled: true
    }
  ],
  limits: [{ kind: "player_main_actions", value: 20 }],
  encounter: {
    kind: "unit_roster",
    units: ["vanguard", "midguard", "rearguard"].map((position, index) => ({
      id: `dummy_${index + 1}`, kind: "training_target", nameKey: `game.combatTrial.dummies.${position}`,
      position, behavior: "passive", healthMode: "immutable_full", objectiveTarget: true,
      defenseSource: "combat_balance_defense_k", defense: 100, maxHp: 1_000_000_000_000,
      damageResist: [0.2, 0.1, 0][index], criticalDamageResist: 0.5
    }))
  },
  endConditions: [
    { kind: "player_team_defeated", reason: "team_defeated" },
    { kind: "player_main_actions_reached", reason: "action_budget_reached" }
  ],
  triggers: [],
  definitionDigest: "standard-v2-digest"
}

const gauntletMetric = (id: "waves_cleared" | "actions_to_victory", valueSource: string, sortOrder: number) => {
  const actionMetric = id === "actions_to_victory"
  const key = actionMetric ? "actionsToVictory" : "wavesCleared"
  return {
    id, schemaVersion: 2,
    nameKey: `game.combatTrial.metrics.${key}.name`, shortNameKey: `game.combatTrial.metrics.${key}.shortName`, descKey: `game.combatTrial.metrics.${key}.desc`,
    eventType: "battle_end", aggregation: "final", valueSource, sourceScope: "player_side", targetScope: "objective_units",
    ranked: true, rankDirection: actionMetric ? "asc" : "desc",
    eligibleEndReasons: actionMetric ? ["all_waves_cleared"] : ["all_waves_cleared", "action_budget_reached", "team_defeated"],
    valueFormat: "integer", sortOrder, enabled: true
  }
}

const gauntletWaveIDs = ["meadowPatrol", "forestHunt", "snowfieldScouts", "blizzardLine", "snowfieldCrown"] as const
const gauntletEnemyIDs = [
  ["slime_001", "goblin_001", "bat_001"],
  ["wolf_001", "mushroom_001", "lizard_001"],
  ["ice_slime_001", "snow_rabbit_001", "ice_bat_001"],
  ["frost_goblin_001", "snow_wolf_001", "frost_spirit_001"],
  ["snowfield_boss_001", "snowfield_boss_002"]
]

const gauntletFormat = {
  ...standardFormat,
  id: "gauntlet",
  nameKey: "game.combatTrial.formats.gauntlet.name",
  shortDescKey: "game.combatTrial.formats.gauntlet.shortDesc",
  descKey: "game.combatTrial.formats.gauntlet.desc",
  icon: "lucide:repeat-2",
  sortOrder: 10,
  actionBudget: 400,
  metricIds: ["waves_cleared", "actions_to_victory"],
  metrics: [gauntletMetric("waves_cleared", "waves_cleared", 30), gauntletMetric("actions_to_victory", "player_main_actions", 20)],
  limits: [{ kind: "player_main_actions", value: 400 }],
  encounter: {
    kind: "wave_roster",
    waves: gauntletWaveIDs.map((waveID, waveIndex) => ({
      id: waveID,
      nameKey: `game.combatTrial.formats.gauntlet.waves.${waveID}`,
      units: gauntletEnemyIDs[waveIndex].map((enemyId, unitIndex) => ({
        id: `${waveID}_${unitIndex + 1}`, kind: "authored_enemy", enemyId, nameKey: `enemy.${enemyId}.name`,
        position: (["vanguard", "midguard", "rearguard"] as const)[unitIndex % 3], behavior: "active", healthMode: "normal", objectiveTarget: true, maxHp: 1000
      }))
    }))
  },
  endConditions: [
    { kind: "player_team_defeated", reason: "team_defeated" },
    { kind: "encounter_completed", reason: "all_waves_cleared", labelKey: "game.combatTrial.endReason.allWavesCleared" },
    { kind: "player_main_actions_reached", reason: "action_budget_reached" }
  ],
  definitionDigest: "gauntlet-v4-digest"
}

const recoveryFormat = {
  ...standardFormat,
  id: "recovery",
  nameKey: "game.combatTrial.formats.recovery.name",
  shortDescKey: "game.combatTrial.formats.recovery.shortDesc",
  descKey: "game.combatTrial.formats.recovery.desc",
  icon: "lucide:heart-pulse",
  sortOrder: 20,
  actionBudget: 20,
  metricIds: ["effective_healing", "objective_remaining_hp"],
  metrics: [
    { id: "effective_healing", schemaVersion: 2, nameKey: "game.combatTrial.metrics.effectiveHealing.name", shortNameKey: "game.combatTrial.metrics.effectiveHealing.shortName", descKey: "game.combatTrial.metrics.effectiveHealing.desc", eventType: "heal", aggregation: "sum", sourceScope: "player_side", targetScope: "objective_units", ranked: true, rankDirection: "desc", eligibleEndReasons: ["action_budget_reached", "team_defeated"], valueFormat: "number", sortOrder: 50, enabled: true },
    { id: "objective_remaining_hp", schemaVersion: 2, nameKey: "game.combatTrial.metrics.objectiveRemainingHp.name", shortNameKey: "game.combatTrial.metrics.objectiveRemainingHp.shortName", descKey: "game.combatTrial.metrics.objectiveRemainingHp.desc", eventType: "battle_end", aggregation: "final", valueSource: "objective_units_remaining_hp", sourceScope: "player_side", targetScope: "objective_units", ranked: true, rankDirection: "desc", eligibleEndReasons: ["action_budget_reached", "team_defeated"], valueFormat: "number", sortOrder: 60, enabled: true }
  ],
  encounter: {
    kind: "unit_roster",
    units: [
      { id: "wounded_patient", kind: "scripted_ally", nameKey: "game.combatTrial.objectives.woundedPatient", side: "player", position: "midguard", behavior: "passive", healthMode: "normal", objectiveTarget: true, initialHpRatio: 0.5, skipTurnOrder: true, initialBuffs: [{ buffId: "combat_trial_patient_deterioration", payload: { max_hp_loss_ratio: 0.05 } }], defense: 100, maxHp: 100_000_000, criticalDamageResist: 0.5 },
      { id: "recovery_clock_vanguard", kind: "training_target", nameKey: "game.combatTrial.objectives.recoveryClockVanguard", position: "vanguard", behavior: "passive", healthMode: "immutable_full", objectiveTarget: false, defense: 100, maxHp: 1_000_000_000_000, damageResist: 0.9, criticalDamageResist: 0.5 },
      { id: "recovery_clock_midguard", kind: "training_target", nameKey: "game.combatTrial.objectives.recoveryClockMidguard", position: "midguard", behavior: "passive", healthMode: "immutable_full", objectiveTarget: false, defense: 100, maxHp: 1_000_000_000_000, damageResist: 0.9, criticalDamageResist: 0.5 },
      { id: "recovery_clock_rearguard", kind: "training_target", nameKey: "game.combatTrial.objectives.recoveryClockRearguard", position: "rearguard", behavior: "passive", healthMode: "immutable_full", objectiveTarget: false, defense: 100, maxHp: 1_000_000_000_000, damageResist: 0.9, criticalDamageResist: 0.5 }
    ]
  },
  definitionDigest: "recovery-v2-digest"
}

function gauntletFormatWithWaveCount(rawCount: number) {
  const waveCount = Math.max(1, Math.trunc(rawCount))
  if (waveCount === gauntletFormat.encounter.waves.length) return gauntletFormat
  return {
    ...gauntletFormat,
    encounter: {
      ...gauntletFormat.encounter,
      waves: Array.from({ length: waveCount }, (_, index) => {
        const source = gauntletFormat.encounter.waves[index % gauntletFormat.encounter.waves.length]
        return {
          ...source,
          id: `${source.id}_${index + 1}`,
          nameKey: index < gauntletFormat.encounter.waves.length ? source.nameKey : "",
          units: source.units.map((unit) => ({ ...unit, id: `${unit.id}_${index + 1}` }))
        }
      })
    }
  }
}

async function mockCombatTrial(page: Page, options: { gauntletWaveCount?: number } = {}) {
  const gauntletFixture = gauntletFormatWithWaveCount(options.gauntletWaveCount ?? gauntletFormat.encounter.waves.length)
  await page.addInitScript(() => {
    localStorage.setItem("locale", "zh-CN")
    localStorage.setItem("token", "e2e-token")
    localStorage.setItem("userId", "42")
    localStorage.setItem("username", "试炼测试员")
    localStorage.setItem("player-settings:42", JSON.stringify({ language: "zh-CN", theme: "light", menuLayout: "icon-rail", simpleMode: true, emailVerified: true }))
  })
  await page.routeWebSocket(/\/ws(?:\?|$)/, (socket) => {
    socket.onMessage((message) => {
      if (typeof message !== "string") return
      const event = JSON.parse(message) as { type?: string }
      if (event.type === "auth.bind") socket.send(JSON.stringify({ type: "auth.bound", ts: Date.now(), data: {} }))
      if (event.type === "action.offline.rewards.get") socket.send(JSON.stringify({ type: "action.offline.rewards.result", ts: Date.now(), data: { hasMoreDue: false, rewards: [] } }))
    })
  })
  await page.route("**/api/**", async (route) => {
    const requestURL = new URL(route.request().url())
    const path = decodeURIComponent(requestURL.pathname)
    const fulfill = (data: unknown) => route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ ok: true, ts: Date.now(), data }) })
    if (path === "/api/bootstrap/init") return fulfill({
      playerStats: {}, inventory: { items: [] }, chatConfig: { channels: [], defaultChannel: "world" }, chatHistory: { items: [] },
      playerSettings: { username: "试炼测试员", language: "zh-CN", theme: "light", menuLayout: "icon-rail", simpleMode: true, emailVerified: true },
      tutorialProgress: {}, playerPresence: { status: "online" }, systemVersion: {}, recycleConfig: { goldCoefficient: 0.5, affixMismatchExtraCoefficient: 0.6 }
    })
    if (path === "/api/combat-trials/overview") return fulfill({
      battleKind: "combat_trial",
      selection: { playerPosition: "vanguard", formatId: "standard", kitties: [{ kittyId: 7, position: "midguard" }, { kittyId: 9, position: "rearguard" }] },
      kitties: [
        { id: 7, name: "kitttyy", level: 3, status: "idle", positionLocked: "midguard", positionLockedByPersonalityId: "calm", positionLockedByPersonalityKey: "" },
        { id: 9, name: "hallo", level: 1, status: "idle", positionLocked: "", positionLockedByPersonalityId: "", positionLockedByPersonalityKey: "" }
      ],
      formats: [standardFormat, gauntletFixture, recoveryFormat],
      personalBests: [
        { userId: 42, formatId: "standard", formatDefinitionDigest: "standard-v2-digest", metricId: "damage_total", value: 128_460, metrics: { damage_total: 128_460, damage_max_segment: 32_780 }, kittyCount: 2, achievedAt: "2026-09-01T01:20:00Z", runId: 33 },
        { userId: 42, formatId: "standard", formatDefinitionDigest: "standard-v2-digest", metricId: "damage_max_segment", value: 32_780, metrics: { damage_total: 128_460, damage_max_segment: 32_780 }, kittyCount: 2, achievedAt: "2026-09-01T01:20:00Z", runId: 33 }
      ],
      activeRun: null,
      rules: { historyKeepCount: 50, ranked: true, unlimitedRuns: true, maxKitties: 2 }
    })
    if (path === "/api/combat-trials/runs/34/replay") return fulfill({
      runId: 34,
      battleKind: "combat_trial",
      seed: 34,
      status: "completed",
      endReason: "all_waves_cleared",
      metrics: { waves_cleared: 5, actions_to_victory: 34 },
      actionCount: 34,
      actionBudget: 400,
      initialState: {
        roomId: 34,
        wave: 1,
        completedBattles: 5,
        seq: 0,
        players: [{ unitId: "player:42", unitType: "player", side: "player", name: "试炼测试员", position: "vanguard", hp: 100, maxHp: 100, mp: 20, maxMp: 20, alive: true }],
        enemies: [{ unitId: "combat_trial:opponent:snowfield_boss", unitType: "enemy", side: "enemy", name: "试炼目标", position: "midguard", hp: 0, maxHp: 280_000, mp: 0, maxMp: 0, alive: false }],
        combatTrialFormat: {
          id: gauntletFixture.id,
          nameKey: gauntletFixture.nameKey,
          shortDescKey: gauntletFixture.shortDescKey,
          descKey: gauntletFixture.descKey,
          icon: gauntletFixture.icon,
          actionBudget: gauntletFixture.actionBudget,
          definitionDigest: gauntletFixture.definitionDigest,
          metrics: gauntletFixture.metrics
        },
        combatTrial: { actionCount: 34, actionBudget: 400, endReason: "all_waves_cleared", currentWave: 5, waveCount: 5, wavesCleared: 5, objectiveUnitsDefeated: 14, metrics: { waves_cleared: 5, actions_to_victory: 34 } }
      },
      timeline: [],
      finalResult: {}
    })
    if (path === "/api/combat-trials/runs" && route.request().method() === "POST") {
      const body = route.request().postDataJSON() as { selection?: { formatId?: string; playerPosition?: string; kitties?: unknown[] } }
      if (!body.selection?.formatId || !body.selection.playerPosition || !Array.isArray(body.selection.kitties)) {
        return route.fulfill({ status: 400, contentType: "application/json", body: JSON.stringify({ ok: false, error: { code: "COMBAT_TRIAL_LINEUP_INVALID", message: "missing inline selection" } }) })
      }
      return fulfill({
        battleKind: "combat_trial",
        run: {
          id: 99, battleKind: "combat_trial", userId: 42, formatId: body.selection.formatId, formatDefinitionDigest: `${body.selection.formatId}-digest`,
          seed: 99, status: "queued", attempts: 0, metrics: {}, actionCount: 0, actionBudget: body.selection.formatId === "gauntlet" ? 400 : 20,
          endReason: "", kittyCount: body.selection.kitties.length, createdAt: "2026-09-01T01:30:00Z"
        }
      })
    }
    if (path === "/api/combat-trials/runs") return fulfill({
      battleKind: "combat_trial",
      runs: [{
        id: 34, battleKind: "combat_trial", userId: 42, formatId: "gauntlet", formatDefinitionDigest: "gauntlet-v4-digest",
        seed: 34, status: "completed", attempts: 1, metrics: { waves_cleared: 5, actions_to_victory: 34 }, actionCount: 34, actionBudget: 400,
        endReason: "all_waves_cleared", kittyCount: 2, createdAt: "2026-09-01T01:25:00Z", completedAt: "2026-09-01T01:26:00Z",
        format: {
          id: gauntletFixture.id, nameKey: gauntletFixture.nameKey, shortDescKey: gauntletFixture.shortDescKey, icon: gauntletFixture.icon,
          definitionDigest: gauntletFixture.definitionDigest, actionBudget: gauntletFixture.actionBudget, metrics: gauntletFixture.metrics, endConditions: gauntletFixture.endConditions
        }
      }],
      historyKeepCount: 50
    })
    if (path === "/api/combat-trials/leaderboard") {
      const formatId = requestURL.searchParams.get("formatId") || "standard"
      const format = formatId === "gauntlet" ? gauntletFixture : formatId === "recovery" ? recoveryFormat : standardFormat
      const metricId = requestURL.searchParams.get("metricId") || format.metricIds[0]
      const metrics = formatId === "gauntlet"
        ? { waves_cleared: 5, actions_to_victory: 34 }
        : formatId === "recovery" ? { effective_healing: 50_000_000, objective_remaining_hp: 95_000_000 } : { damage_total: 128_460, damage_max_segment: 32_780 }
      const value = Number(metrics[metricId as keyof typeof metrics] || 0)
      const entry = { rank: 1, username: "试炼测试员", metricId, value, metrics, kittyCount: 2, achievedAt: "2026-09-01T01:20:00Z" }
      return fulfill({ battleKind: "combat_trial", format, metricId, total: 1, limit: 100, entries: [entry], self: entry })
    }
    return fulfill({})
  })
}

async function dismissBuildNotice(page: Page) {
  const notice = page.locator(".el-notification").filter({ hasText: "测试版本提示" })
  await notice.waitFor({ state: "visible", timeout: 5_000 }).catch(() => undefined)
  if (await notice.isVisible()) {
    await notice.locator(".el-notification__closeBtn").click()
    await notice.waitFor({ state: "hidden", timeout: 3_000 })
  }
}

async function expectNoHorizontalOverflow(page: Page) {
  await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth <= document.documentElement.clientWidth)).toBe(true)
}

test("cat trial tutorial reaches projects, lineup, rankings, and records", async ({ page }) => {
  await page.setViewportSize({ width: 1440, height: 1000 })
  await mockCombatTrial(page)
  await page.goto("/game/combat-trial")
  await dismissBuildNotice(page)

  await page.getByRole("button", { name: "教程", exact: true }).click()
  const popover = page.locator(".driver-popover")
  const steps = [
    ["猫猫试炼总览", "[data-tutorial='combat-trial-page']"],
    ["试炼、排行与记录", "[data-tutorial='combat-trial-tabs']"],
    ["先选择试炼项目", "[data-tutorial='combat-trial-projects']"],
    ["读懂目标与计分规则", "[data-tutorial='combat-trial-rules']"],
    ["编排当前试炼小队", "[data-tutorial='combat-trial-lineup']"],
    ["开始试炼", "[data-tutorial='combat-trial-start']"],
    ["按项目指标比较成绩", "[data-tutorial='combat-trial-leaderboard']"],
    ["复盘最近试炼", "[data-tutorial='combat-trial-history']"],
  ] as const
  for (let index = 0; index < steps.length; index += 1) {
    const [title, selector] = steps[index]
    await expect(popover).toContainText(title)
    await expect(page.locator(selector)).toBeVisible()
    await popover.getByRole("button", { name: index === steps.length - 1 ? "完成" : "下一步", exact: true }).click()
  }
  await expect(popover).toBeHidden()
})

test("desktop trial keeps records prominent and switches DSL-driven format cards", async ({ page }) => {
  await page.setViewportSize({ width: 1440, height: 1000 })
  await mockCombatTrial(page)
  await page.goto("/game/combat-trial")
  await dismissBuildNotice(page)

  await expect(page.getByText("猫猫试炼").first()).toBeVisible()
  await expect(page.getByText("个人最佳")).toBeVisible()
  await expect(page.locator(".trial-project-card")).toHaveCount(3)
  await expect(page.getByText("出战小队")).toBeVisible()
  await expect(page.locator(".trial-party-row")).toHaveCount(3)
  const standardProject = page.locator(".trial-project-card").filter({ hasText: "伤害试炼" })
  const projectDetail = page.locator(".trial-project-detail")
  await expect(projectDetail.locator(".trial-project-target")).toHaveCount(3)
  await expect(projectDetail.getByText("单位可以位于同一排")).toBeVisible()
  await expect(page.locator(".trial-project-card__ranking")).toHaveCount(3)
  const projectCardHeights = await page.locator(".trial-project-card").evaluateAll((nodes) => nodes.map((node) => Math.round(node.getBoundingClientRect().height)))
  expect(projectCardHeights.every((height) => Math.abs(height - projectCardHeights[0]) <= 1 && height <= 72)).toBe(true)
  await expect(page.getByRole("button", { name: "保存阵容" })).toHaveCount(0)
  await expect(page.getByRole("button", { name: "开始试炼" })).toBeInViewport()
  await page.locator(".trial-party-row").first().getByText("中排", { exact: true }).click()
  await expect(page.getByRole("button", { name: "开始试炼" })).toBeEnabled()
  await expect(page.getByText("128,460")).toBeVisible()
  const bestTop = await page.locator(".trial-launch-summary").evaluate((node) => node.getBoundingClientRect().top)
  const partyTop = await page.locator(".trial-preparation").evaluate((node) => node.getBoundingClientRect().top)
  expect(bestTop).toBeLessThan(partyTop)
  await expectNoHorizontalOverflow(page)
  await page.screenshot({ path: "../output/playwright/combat-trial-v8-compact-projects-desktop.png", fullPage: true })

  await standardProject.locator(".trial-project-card__ranking").click()
  const formatSelect = page.locator(".trial-leaderboard-controls .el-select")
  const metricSegmented = page.locator(".trial-leaderboard-controls .el-segmented")
  await expect(formatSelect).toContainText("伤害试炼")
  const [formatBox, metricBox] = await Promise.all([formatSelect.boundingBox(), metricSegmented.boundingBox()])
  expect(formatBox).not.toBeNull()
  expect(metricBox).not.toBeNull()
  const formatRight = (formatBox?.x ?? 0) + (formatBox?.width ?? 0)
  const metricRight = (metricBox?.x ?? 0) + (metricBox?.width ?? 0)
  expect(Math.abs(formatRight - metricRight)).toBeLessThanOrEqual(1)
  await page.screenshot({ path: "../output/playwright/combat-trial-v5-desktop-leaderboard.png", fullPage: true })
  await page.getByRole("tab", { name: "试炼" }).click()

  await projectDetail.locator(".trial-project-target").first().click()
  const targetDialog = page.locator(".el-dialog").filter({ hasText: "前排木桩" })
  await expect(targetDialog.getByText("生命值")).toBeVisible()
  await expect(targetDialog.getByText("∞")).toBeVisible()
  await expect(targetDialog.getByText("不攻击", { exact: true })).toBeVisible()
  await expect(targetDialog.getByText("物理伤害减免", { exact: true })).toBeVisible()
  await expect(targetDialog.getByText("魔法伤害减免", { exact: true })).toBeVisible()
  await expect(targetDialog.getByText("20%", { exact: true })).toHaveCount(2)
  await expect(targetDialog.getByText("暴击伤害抵抗", { exact: true })).toBeVisible()
  await expect(targetDialog.getByText("50%", { exact: true })).toBeVisible()
  await targetDialog.screenshot({ path: "../output/playwright/combat-trial-v4-desktop-target-detail.png" })
  await targetDialog.locator(".el-dialog__headerbtn").click()

  const gauntletProject = page.locator(".trial-project-card").filter({ hasText: "连战冲关" })
  await gauntletProject.locator(".trial-project-card__copy").click()
  await expect(projectDetail.getByText("连战冲关", { exact: true })).toBeVisible()
  await expect(projectDetail.locator(".trial-project-wave")).toHaveCount(5)
  await expect(projectDetail.locator(".trial-project-wave").first()).toHaveAttribute("aria-pressed", "true")
  await expect(projectDetail.locator(".trial-wave-unit")).toHaveCount(3)
  await projectDetail.locator(".trial-wave-unit").first().click()
  await expect(page.locator(".el-dialog").getByText("主动敌人")).toBeVisible()
  await expect(page.locator(".el-dialog").getByText("可被击败")).toBeVisible()
  await page.locator(".el-dialog").locator(".el-dialog__headerbtn").click()
  await expect(page.locator(".trial-launch-best").getByText("通过波数")).toBeVisible()
  await gauntletProject.locator(".trial-project-card__ranking").click()
  await expect(page.getByRole("tab", { name: "排行榜" })).toHaveAttribute("aria-selected", "true")
  await expect(page.locator(".trial-leaderboard-controls .el-select")).toContainText("连战冲关")
  await page.getByRole("tab", { name: "试炼" }).click()

  const recoveryProject = page.locator(".trial-project-card").filter({ hasText: "妙手回春" })
  await recoveryProject.locator(".trial-project-card__copy").click()
  await expect(projectDetail.locator(".trial-project-target")).toHaveCount(1)
  await projectDetail.locator(".trial-project-target").click()
  const patientDialog = page.locator(".el-dialog").filter({ hasText: "负伤队员" })
  await expect(patientDialog.getByText("需要守护的友方")).toBeVisible()
  await expect(patientDialog.getByText("50,000,000 / 100,000,000", { exact: true })).toBeVisible()
  await patientDialog.screenshot({ path: "../output/playwright/combat-trial-v6-recovery-patient.png" })
  await patientDialog.locator(".el-dialog__headerbtn").click()
  await expect(page.locator(".trial-launch-best").getByText("有效治疗")).toBeVisible()
  await page.getByRole("button", { name: "开始试炼" }).click()
  await expect(page.getByText("试炼已进入队列")).toBeVisible()
})

test("ten-wave projects stay contained in a selectable horizontal navigator", async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 900 })
  await mockCombatTrial(page, { gauntletWaveCount: 10 })
  await page.goto("/game/combat-trial")
  await dismissBuildNotice(page)

  const gauntletProject = page.locator(".trial-project-card").filter({ hasText: "连战冲关" })
  await gauntletProject.locator(".trial-project-card__copy").click()
  const projectDetail = page.locator(".trial-project-detail")
  const waveTrack = projectDetail.locator(".trial-project-waves")
  const waves = projectDetail.locator(".trial-project-wave")
  await expect(waves).toHaveCount(10)
  await expect.poll(() => waveTrack.evaluate((node) => node.scrollWidth > node.clientWidth)).toBe(true)

  await waves.last().click()
  await expect(waves.last()).toHaveAttribute("aria-pressed", "true")
  await expect(projectDetail.locator(".trial-wave-browser__controls > strong")).toHaveText("第 10 / 10 波")
  await expect(projectDetail.locator(".trial-wave-detail__summary")).toContainText("第 10 波")
  await expect(projectDetail.locator(".trial-wave-unit")).toHaveCount(2)
  await expect.poll(() => waveTrack.evaluate((node) => node.scrollLeft > 0)).toBe(true)
  await expectNoHorizontalOverflow(page)
  await page.screenshot({ path: "../output/playwright/combat-trial-v8-ten-wave-detail.png", fullPage: true })
})

test("360px trial remains usable without horizontal overflow", async ({ page }) => {
  await page.setViewportSize({ width: 360, height: 800 })
  await mockCombatTrial(page)
  await page.goto("/game/combat-trial")
  await dismissBuildNotice(page)

  const projectDetail = page.locator(".trial-project-detail")
  await expect(projectDetail.locator(".trial-project-target")).toHaveCount(3)
  await expect(page.getByRole("button", { name: "开始试炼" })).toBeInViewport()
  await expectNoHorizontalOverflow(page)
  const clipped = await page.locator(".trial-launch-summary, .trial-projects, .trial-preparation").evaluateAll((nodes) => nodes
    .filter((node) => node.getBoundingClientRect().right > document.documentElement.clientWidth + 1)
    .map((node) => node.className))
  expect(clipped).toEqual([])
  await page.screenshot({ path: "../output/playwright/combat-trial-v8-compact-projects-mobile.png", fullPage: true })

  await page.locator(".trial-projects").scrollIntoViewIfNeeded()
  await expect(projectDetail.locator(".trial-project-target")).toHaveCount(3)
  await page.locator(".trial-project-card").filter({ hasText: "连战冲关" }).click()
  await expect(projectDetail.locator(".trial-project-wave")).toHaveCount(5)
  await expectNoHorizontalOverflow(page)
  await page.screenshot({ path: "../output/playwright/combat-trial-v8-mobile-project-detail.png", fullPage: true })

  await page.getByRole("tab", { name: "排行榜" }).click()
  await expect(page.locator(".trial-table tbody tr")).toHaveCount(1)
  await expect(page.getByText("#1").last()).toBeVisible()
  await expectNoHorizontalOverflow(page)

  await page.getByRole("tab", { name: "记录" }).click()
  const historyRow = page.locator(".trial-history-row")
  await expect(historyRow.getByText("全部波次已通过")).toBeVisible()
  await expect(historyRow.getByText("通过波数")).toBeVisible()
  await expect(historyRow.getByText("5", { exact: true })).toBeVisible()
  await expect(historyRow.locator(".trial-history-buttons svg")).toHaveCount(2)
  await expectNoHorizontalOverflow(page)
  await page.screenshot({ path: "../output/playwright/combat-trial-v3-mobile-history.png", fullPage: true })
})

test("trial replay uses objective terminology instead of League home and visiting teams", async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 900 })
  await mockCombatTrial(page)
  await page.goto("/game/combat-trial")
  await dismissBuildNotice(page)

  await page.getByRole("tab", { name: "记录" }).click()
  await page.getByRole("button", { name: "观看录像" }).click()
  const replayDialog = page.locator(".arena-replay-dialog")
  await expect(replayDialog).toBeVisible()
  await expect(replayDialog.getByText("试炼小队").first()).toBeVisible()
  await expect(replayDialog.getByText("试炼目标").first()).toBeVisible()
  await page.getByRole("tab", { name: "单位贡献" }).click()
  await expect(page.locator(".replay-side-comparison").getByText("试炼小队")).toBeVisible()
  await expect(page.locator(".replay-side-comparison").getByText("试炼目标")).toBeVisible()
  await expect(page.getByText("来访小队", { exact: true })).toHaveCount(0)
  await expect(page.getByText("主场小队", { exact: true })).toHaveCount(0)
  await page.screenshot({ path: "../output/playwright/combat-trial-v3-replay.png", fullPage: true })
})
