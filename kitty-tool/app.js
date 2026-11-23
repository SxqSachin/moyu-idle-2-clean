const $ = (selector) => document.querySelector(selector)

const contentEl = $("#content")
const statusEl = $("#status")
let activeTab = "leveling"

const defaultConfig = {
  version: "v1",
  leveling: {
    maxLevel: 30,
    fallbackFormula: { base: 100, perLevel: 50, power: 1.25 },
    levels: [
      { level: 1, nextExp: 80 },
      { level: 2, nextExp: 120 },
      { level: 3, nextExp: 180 },
      { level: 4, nextExp: 260 },
      { level: 5, nextExp: 360 }
    ],
    milestones: [{ level: 5, nameKey: "game.catteryKittyMilestone.level5.name", descKey: "game.catteryKittyMilestone.level5.desc", effects: { commissionSlotBonus: 1 } }]
  },
  journalRewards: {
    enabled: true,
    contentPoolMap: {},
    categoryPoolMap: { "diary.idle": "idle_default_pool", "diary.working": "working_default_pool" },
    pools: {
      idle_default_pool: {
        labelKey: "game.catteryRewardPool.idle_default_pool",
        minKittyLevel: 1,
        kittyExp: { min: 1, max: 3 },
        bondExp: { min: 1, max: 1 },
        moodDelta: 0.01,
        rolls: [{ chance: 0.2, mode: "weightedOne", entries: [{ itemId: "wood", weight: 1, min: 1, max: 2 }] }]
      },
      working_default_pool: {
        labelKey: "game.catteryRewardPool.working_default_pool",
        minKittyLevel: 1,
        kittyExp: { min: 2, max: 5 },
        bondExp: { min: 1, max: 2 },
        moodDelta: 0.01,
        rolls: [{ chance: 0.25, mode: "weightedOne", entries: [{ itemId: "gold_coin", weight: 1, min: 1, max: 3 }] }]
      }
    }
  },
  commissions: {
    enabled: true,
    refreshMode: "daily",
    visibleCount: 3,
    dailyLimitPerKitty: 6,
    pools: [{ id: "default_daily", minKittyLevel: 1, entries: [{ commissionId: "bring_back_wood", weight: 80 }, { commissionId: "find_gold_coin", weight: 35 }, { commissionId: "gather_silverleaf", weight: 10 }] }],
    items: [
      {
        id: "bring_back_wood",
        rarity: "common",
        nameKey: "game.catteryCommission.bring_back_wood.name",
        descKey: "game.catteryCommission.bring_back_wood.desc",
        resultKey: "game.catteryCommission.bring_back_wood.result",
        minKittyLevel: 1,
        durationSeconds: 1800,
        costs: { wood: 2 },
        rewardPoolId: "commission_wood_pool",
        kittyExp: 30,
        bondExp: 8,
        moodDelta: 0.03
      },
      {
        id: "find_gold_coin",
        rarity: "uncommon",
        nameKey: "game.catteryCommission.find_gold_coin.name",
        descKey: "game.catteryCommission.find_gold_coin.desc",
        resultKey: "game.catteryCommission.find_gold_coin.result",
        minKittyLevel: 2,
        durationSeconds: 3600,
        costs: { wood: 5 },
        rewardPoolId: "commission_coin_pool",
        kittyExp: 45,
        bondExp: 10,
        moodDelta: 0.04
      },
      {
        id: "gather_silverleaf",
        rarity: "rare",
        nameKey: "game.catteryCommission.gather_silverleaf.name",
        descKey: "game.catteryCommission.gather_silverleaf.desc",
        resultKey: "game.catteryCommission.gather_silverleaf.result",
        minKittyLevel: 5,
        durationSeconds: 7200,
        costs: { sturdy_wood: 2, gold_coin: 20 },
        rewardPoolId: "commission_silverleaf_pool",
        kittyExp: 90,
        bondExp: 18,
        moodDelta: 0.05
      }
    ],
    rewardPools: {
      commission_wood_pool: { labelKey: "game.catteryRewardPool.commission_wood_pool", minKittyLevel: 1, rolls: [{ chance: 1, mode: "weightedOne", entries: [{ itemId: "wood", weight: 1, min: 1, max: 4 }] }] },
      commission_coin_pool: { labelKey: "game.catteryRewardPool.commission_coin_pool", minKittyLevel: 1, rolls: [{ chance: 1, mode: "weightedOne", entries: [{ itemId: "gold_coin", weight: 1, min: 20, max: 45 }] }] },
      commission_silverleaf_pool: {
        labelKey: "game.catteryRewardPool.commission_silverleaf_pool",
        minKittyLevel: 5,
        rolls: [
          { chance: 1, mode: "weightedOne", entries: [{ itemId: "silverleaf_wood", weight: 1, min: 1, max: 2 }] },
          { chance: 0.35, mode: "weightedOne", entries: [{ itemId: "gold_coin", weight: 1, min: 30, max: 80 }] }
        ]
      }
    }
  },
  _i18nDraft: {
    "zh-CN": {},
    "en-US": {}
  }
}

let state = clone(defaultConfig)

function clone(value) {
  return JSON.parse(JSON.stringify(value))
}

function escapeHtml(value) {
  return String(value ?? "").replaceAll("&", "&amp;").replaceAll("<", "&lt;").replaceAll(">", "&gt;").replaceAll('"', "&quot;")
}

function setStatus(text, danger = false) {
  statusEl.textContent = text
  statusEl.classList.toggle("danger", danger)
}

function parseJSON(text, fallback) {
  try {
    return JSON.parse(text || "")
  } catch {
    return fallback
  }
}

function cleanConfig() {
  const out = clone(state)
  delete out._i18nDraft
  return out
}

function render() {
  const errors = validate()
  if (errors.length) setStatus(`${errors.length} 个配置问题：${errors[0]}`, true)
  else setStatus("配置校验通过")
  const views = {
    leveling: renderLeveling,
    commissions: renderCommissions,
    commissionPools: renderCommissionPools,
    journalRewards: renderJournalRewards,
    rewardPools: renderRewardPools,
    i18n: renderI18n,
    raw: renderRaw
  }
  contentEl.innerHTML = views[activeTab]()
}

function renderLeveling() {
  return `
    <h2 class="section-title">等级曲线</h2>
    <div class="form-grid">
      ${numberField("leveling.maxLevel", "最高等级", state.leveling.maxLevel)}
      ${numberField("leveling.fallbackFormula.base", "公式 base", state.leveling.fallbackFormula.base)}
      ${numberField("leveling.fallbackFormula.perLevel", "公式 perLevel", state.leveling.fallbackFormula.perLevel)}
      ${numberField("leveling.fallbackFormula.power", "公式 power", state.leveling.fallbackFormula.power, "0.01")}
    </div>
    <div class="card">
      <div class="row-actions"><button data-action="add-level">新增等级</button><button data-action="add-milestone">新增里程碑</button></div>
    </div>
    ${table(["level", "nextExp", "操作"], state.leveling.levels.map((row, i) => [
      input(`leveling.levels.${i}.level`, row.level, "number"),
      input(`leveling.levels.${i}.nextExp`, row.nextExp, "number"),
      `<button data-action="remove-level" data-index="${i}">删除</button>`
    ]))}
    <h3>里程碑</h3>
    ${table(["level", "nameKey", "descKey", "effects JSON", "操作"], state.leveling.milestones.map((row, i) => [
      input(`leveling.milestones.${i}.level`, row.level, "number"),
      input(`leveling.milestones.${i}.nameKey`, row.nameKey),
      input(`leveling.milestones.${i}.descKey`, row.descKey),
      textarea(`leveling.milestones.${i}.effects`, JSON.stringify(row.effects || {}, null, 2), 80),
      `<button data-action="remove-milestone" data-index="${i}">删除</button>`
    ]))}
  `
}

function renderCommissions() {
  return `
    <h2 class="section-title">委托模板</h2>
    <div class="form-grid">
      ${numberField("commissions.visibleCount", "每日可见数量", state.commissions.visibleCount)}
      ${numberField("commissions.dailyLimitPerKitty", "每日次数限制", state.commissions.dailyLimitPerKitty)}
    </div>
    <div class="card"><button data-action="add-commission">新增委托</button></div>
    ${table(["id/稀有度", "i18n keys", "等级/耗时", "消耗 JSON", "奖励池/成长", "操作"], state.commissions.items.map((row, i) => [
      `${input(`commissions.items.${i}.id`, row.id)}${raritySelect(`commissions.items.${i}.rarity`, row.rarity || "common")}`,
      `${input(`commissions.items.${i}.nameKey`, row.nameKey)}${input(`commissions.items.${i}.descKey`, row.descKey)}${input(`commissions.items.${i}.resultKey`, row.resultKey)}`,
      `${input(`commissions.items.${i}.minKittyLevel`, row.minKittyLevel, "number")}${input(`commissions.items.${i}.durationSeconds`, row.durationSeconds, "number")}`,
      textarea(`commissions.items.${i}.costs`, JSON.stringify(row.costs || {}, null, 2), 100),
      `${input(`commissions.items.${i}.rewardPoolId`, row.rewardPoolId)}${input(`commissions.items.${i}.kittyExp`, row.kittyExp, "number")}${input(`commissions.items.${i}.bondExp`, row.bondExp, "number")}${input(`commissions.items.${i}.moodDelta`, row.moodDelta, "number", "0.01")}`,
      `<button data-action="remove-commission" data-index="${i}">删除</button>`
    ]))}
  `
}

function renderCommissionPools() {
  return `
    <h2 class="section-title">委托发布池</h2>
    <div class="card"><button data-action="add-commission-pool">新增发布池</button></div>
    ${state.commissions.pools.map((pool, i) => `
      <div class="card">
        <div class="form-grid">
          ${field(`commissions.pools.${i}.id`, "池 ID", pool.id)}
          ${numberField(`commissions.pools.${i}.minKittyLevel`, "最低猫咪等级", pool.minKittyLevel)}
        </div>
        <button data-action="add-commission-pool-entry" data-index="${i}">新增条目</button>
        ${table(["commissionId", "weight", "操作"], (pool.entries || []).map((entry, j) => [
          input(`commissions.pools.${i}.entries.${j}.commissionId`, entry.commissionId),
          input(`commissions.pools.${i}.entries.${j}.weight`, entry.weight, "number"),
          `<button data-action="remove-commission-pool-entry" data-index="${i}" data-entry="${j}">删除</button>`
        ]))}
        <button data-action="remove-commission-pool" data-index="${i}">删除发布池</button>
      </div>
    `).join("")}
  `
}

function renderJournalRewards() {
  return `
    <h2 class="section-title">随记奖励绑定</h2>
    <div class="form-grid">
      <div class="field"><label>contentPoolMap JSON</label>${textarea("journalRewards.contentPoolMap", JSON.stringify(state.journalRewards.contentPoolMap || {}, null, 2), 220)}</div>
      <div class="field"><label>categoryPoolMap JSON</label>${textarea("journalRewards.categoryPoolMap", JSON.stringify(state.journalRewards.categoryPoolMap || {}, null, 2), 220)}</div>
    </div>
  `
}

function renderRewardPools() {
  return `
    <h2 class="section-title">奖励池</h2>
    <div class="card">
      <button data-action="add-journal-reward-pool">新增随记奖励池</button>
      <button data-action="add-commission-reward-pool">新增委托奖励池</button>
    </div>
    ${rewardPoolEditor("journalRewards.pools", state.journalRewards.pools)}
    ${rewardPoolEditor("commissions.rewardPools", state.commissions.rewardPools)}
  `
}

function rewardPoolEditor(path, pools) {
  return Object.entries(pools || {}).map(([id, pool]) => `
    <div class="card">
      <h3>${escapeHtml(path)} / ${escapeHtml(id)}</h3>
      <div class="form-grid">
        ${field(`${path}.${id}.labelKey`, "labelKey", pool.labelKey)}
        ${numberField(`${path}.${id}.minKittyLevel`, "最低等级", pool.minKittyLevel || 1)}
        ${numberField(`${path}.${id}.moodDelta`, "心情变化", pool.moodDelta || 0, "0.01")}
      </div>
      <div class="form-grid">
        <div class="field"><label>kittyExp JSON</label>${textarea(`${path}.${id}.kittyExp`, JSON.stringify(pool.kittyExp || null, null, 2), 80)}</div>
        <div class="field"><label>bondExp JSON</label>${textarea(`${path}.${id}.bondExp`, JSON.stringify(pool.bondExp || null, null, 2), 80)}</div>
        <div class="field"><label>rolls JSON</label>${textarea(`${path}.${id}.rolls`, JSON.stringify(pool.rolls || [], null, 2), 220)}</div>
      </div>
      <button data-action="remove-reward-pool" data-path="${escapeHtml(path)}" data-id="${escapeHtml(id)}">删除奖励池</button>
    </div>
  `).join("")
}

function renderI18n() {
  return `
    <h2 class="section-title">i18n 文案草稿</h2>
    <p class="hint">这里生成 patch 草稿，复制后手动合并到 locales 文件。</p>
    <div class="form-grid">
      <div class="field"><label>zh-CN patch</label>${textarea("_i18nDraft.zh-CN", JSON.stringify(state._i18nDraft?.["zh-CN"] || {}, null, 2), 260)}</div>
      <div class="field"><label>en-US patch</label>${textarea("_i18nDraft.en-US", JSON.stringify(state._i18nDraft?.["en-US"] || {}, null, 2), 260)}</div>
    </div>
    <div class="card"><button data-action="seed-i18n">根据配置生成空 key</button></div>
  `
}

function renderRaw() {
  return `<h2 class="section-title">原始 JSON</h2>${textarea("raw", JSON.stringify(cleanConfig(), null, 2), 560)}`
}

function table(headers, rows) {
  return `<table><thead><tr>${headers.map((h) => `<th>${escapeHtml(h)}</th>`).join("")}</tr></thead><tbody>${rows.map((row) => `<tr>${row.map((cell) => `<td>${cell}</td>`).join("")}</tr>`).join("")}</tbody></table>`
}

function field(path, label, value) {
  return `<div class="field"><label>${escapeHtml(label)}</label>${input(path, value)}</div>`
}

function numberField(path, label, value, step = "1") {
  return `<div class="field"><label>${escapeHtml(label)}</label>${input(path, value, "number", step)}</div>`
}

function input(path, value, type = "text", step = "1") {
  return `<input data-path="${escapeHtml(path)}" type="${type}" step="${step}" value="${escapeHtml(value ?? "")}" />`
}

function raritySelect(path, value) {
  const options = ["common", "uncommon", "rare", "epic", "legendary", "mythic"]
  return `<select data-path="${escapeHtml(path)}">${options.map((option) => `<option value="${option}" ${option === value ? "selected" : ""}>${option}</option>`).join("")}</select>`
}

function textarea(path, value, height = 180) {
  return `<textarea data-path="${escapeHtml(path)}" style="min-height:${height}px">${escapeHtml(value ?? "")}</textarea>`
}

function setByPath(path, value) {
  if (path === "raw") {
    state = { ...parseJSON(value, cleanConfig()), _i18nDraft: state._i18nDraft || { "zh-CN": {}, "en-US": {} } }
    return
  }
  const parts = path.split(".")
  let node = state
  for (let i = 0; i < parts.length - 1; i++) node = node[parts[i]]
  const key = parts.at(-1)
  if (path.endsWith(".costs") || path.endsWith(".effects") || path.endsWith(".rolls") || path.endsWith(".kittyExp") || path.endsWith(".bondExp") || path.includes("PoolMap") || path.startsWith("_i18nDraft.")) {
    node[key] = parseJSON(value, path.endsWith(".rolls") ? [] : {})
  } else if (typeof node[key] === "number") {
    node[key] = Number(value)
  } else {
    node[key] = value
  }
}

function validate() {
  const errors = []
  const ids = new Set()
  for (const item of state.commissions.items || []) {
    if (!item.id) errors.push("委托缺少 id")
    if (ids.has(item.id)) errors.push(`委托 id 重复：${item.id}`)
    ids.add(item.id)
    if (!state.commissions.rewardPools?.[item.rewardPoolId]) errors.push(`委托 ${item.id} 引用不存在的奖励池 ${item.rewardPoolId}`)
  }
  for (const pool of state.commissions.pools || []) {
    for (const entry of pool.entries || []) {
      if (!ids.has(entry.commissionId)) errors.push(`发布池 ${pool.id} 引用不存在的委托 ${entry.commissionId}`)
      if (!(Number(entry.weight) > 0)) errors.push(`发布池 ${pool.id} 权重无效`)
    }
  }
  for (const [scope, pools] of Object.entries({ journalRewards: state.journalRewards.pools, commissionRewards: state.commissions.rewardPools })) {
    for (const [id, pool] of Object.entries(pools || {})) {
      for (const roll of pool.rolls || []) {
        if (roll.chance < 0 || roll.chance > 1) errors.push(`${scope}.${id} chance 无效`)
        for (const entry of roll.entries || []) {
          if (!entry.itemId) errors.push(`${scope}.${id} 有空 itemId`)
          if (Number(entry.min) <= 0 || Number(entry.max) < Number(entry.min)) errors.push(`${scope}.${id} 数量范围无效`)
        }
      }
    }
  }
  return errors
}

function seedI18n() {
  state._i18nDraft ||= { "zh-CN": {}, "en-US": {} }
  for (const lang of ["zh-CN", "en-US"]) state._i18nDraft[lang] ||= {}
  for (const item of state.commissions.items || []) {
    for (const key of [item.nameKey, item.descKey, item.resultKey]) {
      if (key) {
        state._i18nDraft["zh-CN"][key] ||= ""
        state._i18nDraft["en-US"][key] ||= ""
      }
    }
  }
  for (const poolSet of [state.journalRewards.pools, state.commissions.rewardPools]) {
    for (const pool of Object.values(poolSet || {})) {
      if (pool.labelKey) {
        state._i18nDraft["zh-CN"][pool.labelKey] ||= ""
        state._i18nDraft["en-US"][pool.labelKey] ||= ""
      }
    }
  }
}

contentEl.addEventListener("input", (event) => {
  const target = event.target
  if (!target?.dataset?.path) return
  setByPath(target.dataset.path, target.value)
})

contentEl.addEventListener("change", () => render())

document.body.addEventListener("click", async (event) => {
  const target = event.target.closest("button")
  if (!target) return
  if (target.dataset.tab) {
    activeTab = target.dataset.tab
    document.querySelectorAll(".tabs button").forEach((btn) => btn.classList.toggle("active", btn.dataset.tab === activeTab))
    render()
    return
  }
  const action = target.dataset.action || target.id
  if (action === "add-level") state.leveling.levels.push({ level: state.leveling.levels.length + 1, nextExp: 100 })
  if (action === "remove-level") state.leveling.levels.splice(Number(target.dataset.index), 1)
  if (action === "add-milestone") state.leveling.milestones.push({ level: 1, nameKey: "", descKey: "", effects: {} })
  if (action === "remove-milestone") state.leveling.milestones.splice(Number(target.dataset.index), 1)
  if (action === "add-commission") state.commissions.items.push({ id: "new_commission", rarity: "common", nameKey: "game.catteryCommission.new_commission.name", descKey: "game.catteryCommission.new_commission.desc", resultKey: "game.catteryCommission.new_commission.result", minKittyLevel: 1, durationSeconds: 1800, costs: {}, rewardPoolId: "", kittyExp: 0, bondExp: 0, moodDelta: 0 })
  if (action === "remove-commission") state.commissions.items.splice(Number(target.dataset.index), 1)
  if (action === "add-commission-pool") state.commissions.pools.push({ id: "new_pool", minKittyLevel: 1, entries: [] })
  if (action === "remove-commission-pool") state.commissions.pools.splice(Number(target.dataset.index), 1)
  if (action === "add-commission-pool-entry") state.commissions.pools[Number(target.dataset.index)].entries.push({ commissionId: "", weight: 1 })
  if (action === "remove-commission-pool-entry") state.commissions.pools[Number(target.dataset.index)].entries.splice(Number(target.dataset.entry), 1)
  if (action === "add-journal-reward-pool") state.journalRewards.pools.new_journal_pool = { labelKey: "game.catteryRewardPool.new_journal_pool", minKittyLevel: 1, rolls: [] }
  if (action === "add-commission-reward-pool") state.commissions.rewardPools.new_commission_pool = { labelKey: "game.catteryRewardPool.new_commission_pool", minKittyLevel: 1, rolls: [] }
  if (action === "remove-reward-pool") {
    const root = target.dataset.path.split(".").reduce((node, key) => node[key], state)
    delete root[target.dataset.id]
  }
  if (action === "seed-i18n") seedI18n()
  if (action === "copy-config-btn") await navigator.clipboard.writeText(JSON.stringify(cleanConfig(), null, 2))
  if (action === "copy-i18n-btn") await navigator.clipboard.writeText(JSON.stringify(state._i18nDraft || {}, null, 2))
  if (action === "reset-btn") state = clone(defaultConfig)
  if (action === "format-btn") render()
  if (action === "import-config-btn") {
    const raw = prompt("粘贴 kitty-config.json")
    if (raw) state = { ...parseJSON(raw, cleanConfig()), _i18nDraft: state._i18nDraft || { "zh-CN": {}, "en-US": {} } }
  }
  render()
})

render()
