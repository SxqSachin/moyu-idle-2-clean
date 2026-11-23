const $ = (selector) => document.querySelector(selector)

const rowsEl = $("#rows")
const statusEl = $("#status")
const addRowBtn = $("#add-row-btn")
const resetBtn = $("#reset-btn")
const copyTsvBtn = $("#copy-tsv-btn")
const copyExamplesBtn = $("#copy-examples-btn")
const copyEffectParamsBtn = $("#copy-effect-params-btn")

const modalEl = $("#editor-modal")
const modalTitleEl = $("#editor-title")
const modalBodyEl = $("#editor-body")
const modalSaveBtn = $("#save-editor-btn")

const RARITY_OPTIONS = ["common", "uncommon", "rare", "epic", "legendary", "mythic"]
const SCOPE_OPTIONS = ["basic_attack", "skill", "both"]
const SELECTOR_OPTIONS = ["all", "index"]
const ATTR_KEY_OPTIONS = [
  "battle",
  "strength",
  "dexterity",
  "intelligence",
  "stamina",
  "luck",
  "hp",
  "hp_regen",
  "mp",
  "mp_regen",
  "speed",
  "physics_damage",
  "physics_defence",
  "magical_damage",
  "magical_defence",
  "crit_chance",
  "crit_damage",
  "mining",
  "collecting",
  "farming_animal",
  "cooking",
  "crafting",
  "sewing",
  "forging",
  "enhance",
  "enhancement_success_rate"
]

/** @type {Array<any>} */
let rows = []
let selectedUID = ""
let modalCtx = null

const EXAMPLE_EFFECT_PARAMS = {
  effectMutations: [
    {
      scope: "skill",
      skillIds: ["fireball"],
      effectSelector: { mode: "index", indexes: [1] },
      ops: {
        amountBaseAddDelta: 10,
        targetCountDelta: -1,
        amountTermsAppend: [{ attrKey: "intelligence", coefficient: 0.2, perSkillLevel: 0 }]
      },
      constraints: { targetCountMin: 1 }
    }
  ]
}

function uid() {
  if (typeof crypto !== "undefined" && typeof crypto.randomUUID === "function") return crypto.randomUUID()
  return `affix-${Date.now()}-${Math.random().toString(16).slice(2)}`
}

function clone(value) {
  return JSON.parse(JSON.stringify(value))
}

function escapeHtml(value) {
  return String(value ?? "")
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
}

function normalizeStringArray(value) {
  if (!Array.isArray(value)) return []
  return Array.from(
    new Set(
      value
        .map((item) => String(item ?? "").trim())
        .filter(Boolean)
    )
  )
}

function normalizeMutation(raw) {
  const source = raw && typeof raw === "object" ? raw : {}
  const scope = SCOPE_OPTIONS.includes(source.scope) ? source.scope : "both"
  const selector = source.effectSelector && typeof source.effectSelector === "object" ? source.effectSelector : {}
  const mode = SELECTOR_OPTIONS.includes(selector.mode) ? selector.mode : "all"
  const ops = source.ops && typeof source.ops === "object" ? source.ops : {}
  const constraints = source.constraints && typeof source.constraints === "object" ? source.constraints : {}
  const terms = Array.isArray(ops.amountTermsAppend) ? ops.amountTermsAppend : []
  return {
    scope,
    skillIds: normalizeStringArray(source.skillIds),
    effectSelector: {
      mode,
      indexes:
        mode === "index"
          ? Array.from(
              new Set(
                (Array.isArray(selector.indexes) ? selector.indexes : [])
                  .map((item) => Number(item))
                  .filter((n) => Number.isInteger(n) && n > 0)
              )
            )
          : []
    },
    ops: {
      targetCountDelta: Number.isFinite(Number(ops.targetCountDelta)) ? Number(ops.targetCountDelta) : 0,
      amountBaseAddDelta: Number.isFinite(Number(ops.amountBaseAddDelta)) ? Number(ops.amountBaseAddDelta) : 0,
      amountTermsAppend: terms
        .map((term) => ({
          attrKey: String(term?.attrKey ?? "").trim(),
          coefficient: Number.isFinite(Number(term?.coefficient)) ? Number(term.coefficient) : 0,
          perSkillLevel: Number.isFinite(Number(term?.perSkillLevel)) ? Number(term.perSkillLevel) : 0
        }))
        .filter((term) => term.attrKey !== ""),
      statusIdsAppend: normalizeStringArray(ops.statusIdsAppend),
      statusIdsRemove: normalizeStringArray(ops.statusIdsRemove)
    },
    constraints: {
      targetCountMin: Number.isFinite(Number(constraints.targetCountMin)) ? Number(constraints.targetCountMin) : 1
    }
  }
}

function normalizeEffectParams(raw) {
  const source = raw && typeof raw === "object" && !Array.isArray(raw) ? raw : {}
  const list = Array.isArray(source.effectMutations) ? source.effectMutations : []
  return {
    effectMutations: list.map((item) => normalizeMutation(item))
  }
}

function defaultRow() {
  return {
    _uid: uid(),
    id: "",
    effectType: "effectMutation",
    allowedRarities: ["rare", "epic", "legendary"],
    valuePrecision: 0,
    active: true,
    effectParams: clone(EXAMPLE_EFFECT_PARAMS)
  }
}

function exampleRows() {
  return [
    {
      _uid: uid(),
      id: "skill_bonus_fireball",
      effectType: "effectMutation",
      allowedRarities: ["rare", "epic", "legendary"],
      valuePrecision: 0,
      active: true,
      effectParams: clone(EXAMPLE_EFFECT_PARAMS)
    }
  ]
}

function setStatus(text) {
  if (statusEl) statusEl.textContent = text
}

function findRow(uidValue) {
  return rows.find((row) => row._uid === uidValue)
}

function openModal(ctx) {
  modalCtx = ctx
  modalTitleEl.textContent = ctx.title
  modalBodyEl.innerHTML = ctx.render()
  modalEl.classList.remove("hidden")
  modalEl.setAttribute("aria-hidden", "false")
}

function closeModal() {
  modalCtx = null
  modalEl.classList.add("hidden")
  modalEl.setAttribute("aria-hidden", "true")
  modalBodyEl.innerHTML = ""
}

function renderAllowedRarityEditor(row) {
  const current = normalizeStringArray(row.allowedRarities)
  return `
    <div class="section-card">
      <h3 class="section-title">可用稀有度（allowedRarities）</h3>
      <div class="checkbox-grid">
        ${RARITY_OPTIONS.map(
          (rarity) => `
          <label>
            <input type="checkbox" data-rarity-item="${escapeHtml(rarity)}" ${current.includes(rarity) ? "checked" : ""} />
            <span>${escapeHtml(rarity)}</span>
          </label>
        `
        ).join("")}
      </div>
      <p class="hint">可多选，保存后会自动去重。</p>
    </div>
  `
}

function mutationCard(index, mutation) {
  const skillIdsText = mutation.skillIds.join(", ")
  const indexesText = mutation.effectSelector.indexes.join(", ")
  const statusAppendText = mutation.ops.statusIdsAppend.join(", ")
  const statusRemoveText = mutation.ops.statusIdsRemove.join(", ")
  return `
    <section class="mutation-card" data-mutation-index="${index}">
      <div class="form-grid">
        <div class="form-field">
          <label>作用范围</label>
          <select data-field="scope">
            ${SCOPE_OPTIONS.map((value) => {
              const label =
                value === "basic_attack" ? "普攻（basic_attack）" : value === "skill" ? "技能（skill）" : "普攻+技能（both）"
              return `<option value="${value}" ${mutation.scope === value ? "selected" : ""}>${label}</option>`
            }).join("")}
          </select>
          <p class="hint">basic_attack=普攻，skill=技能，both=都生效</p>
        </div>
        <div class="form-field">
          <label>命中方式</label>
          <select data-field="selectorMode">
            ${SELECTOR_OPTIONS.map((value) => {
              const label = value === "all" ? "全部 effect（all）" : "按序号（index）"
              return `<option value="${value}" ${mutation.effectSelector.mode === value ? "selected" : ""}>${label}</option>`
            }).join("")}
          </select>
          <p class="hint">all=命中所有 effect；index=只命中指定序号</p>
        </div>
        <div class="form-field">
          <label>技能 ID 列表（逗号分隔）</label>
          <input data-field="skillIds" value="${escapeHtml(skillIdsText)}" />
          <p class="hint">仅 scope=skill/both 时填写；scope=basic_attack 时留空</p>
        </div>
        <div class="form-field">
          <label>命中序号（从 1 开始，逗号分隔）</label>
          <input data-field="indexes" value="${escapeHtml(indexesText)}" />
          <p class="hint">命中方式为 all 时可留空</p>
        </div>
        <div class="form-field">
          <label>目标数量增量</label>
          <input data-field="targetCountDelta" type="number" step="1" value="${mutation.ops.targetCountDelta}" />
        </div>
        <div class="form-field">
          <label>基础数值增量</label>
          <input data-field="amountBaseAddDelta" type="number" step="0.01" value="${mutation.ops.amountBaseAddDelta}" />
        </div>
        <div class="form-field">
          <label>目标数量最小值</label>
          <input data-field="targetCountMin" type="number" step="1" min="1" value="${mutation.constraints.targetCountMin}" />
        </div>
        <div class="form-field">
          <label>追加状态 ID（逗号分隔）</label>
          <input data-field="statusIdsAppend" value="${escapeHtml(statusAppendText)}" />
        </div>
        <div class="form-field">
          <label>移除状态 ID（逗号分隔）</label>
          <input data-field="statusIdsRemove" value="${escapeHtml(statusRemoveText)}" />
        </div>
      </div>
      <div class="nested-card">
        <label>追加伤害系数（amountTermsAppend）</label>
        <div class="list-editor" data-terms-list>
          ${mutation.ops.amountTermsAppend
            .map(
              (term, termIndex) => `
            <div class="list-row term-row" data-term-index="${termIndex}">
              <div class="form-field">
                <label>属性键</label>
                <input data-term-field="attrKey" value="${escapeHtml(term.attrKey)}" list="attr-key-options" />
              </div>
              <div class="form-field">
                <label>系数</label>
                <input data-term-field="coefficient" type="number" step="0.01" value="${term.coefficient}" />
              </div>
              <div class="form-field">
                <label>每级增量</label>
                <input data-term-field="perSkillLevel" type="number" step="0.01" value="${term.perSkillLevel}" />
              </div>
              <button type="button" data-action="remove-term">删除</button>
            </div>
          `
            )
            .join("")}
        </div>
        <div class="row-actions">
          <button type="button" data-action="add-term">新增一条系数</button>
        </div>
      </div>
      <div class="row-actions">
        <button type="button" data-action="copy-mutation">复制该 mutation JSON</button>
        <button type="button" data-action="remove-mutation">删除 mutation</button>
      </div>
    </section>
  `
}

function renderEffectParamsEditor(row) {
  const params = normalizeEffectParams(row.effectParams)
  return `
    <div class="section-card">
      <h3 class="section-title">effectType（固定）</h3>
      <span class="effect-type-badge">effectMutation</span>
      <p class="hint">当使用 effectMutations 时，effectType 只能是 effectMutation。</p>
    </div>
    <div class="section-card">
      <h3 class="section-title">effectParams.effectMutations</h3>
      <div id="mutation-list">
        ${params.effectMutations.map((mutation, idx) => mutationCard(idx, mutation)).join("")}
      </div>
      <div class="row-actions">
        <button type="button" id="add-sewing10-template-btn">插入“普攻+缝纫*10”示例</button>
        <button type="button" id="add-basic-attack-template-btn">插入“普攻全作用”模板</button>
        <button type="button" id="add-mutation-btn">新增 mutation</button>
      </div>
    </div>
    <datalist id="attr-key-options">
      ${ATTR_KEY_OPTIONS.map((key) => `<option value="${key}"></option>`).join("")}
    </datalist>
  `
}

function parseCommaList(text) {
  return Array.from(
    new Set(
      String(text ?? "")
        .split(",")
        .map((item) => item.trim())
        .filter(Boolean)
    )
  )
}

function collectTermsFromCard(cardEl) {
  return Array.from(cardEl.querySelectorAll("[data-term-index]"))
    .map((termEl) => {
      const get = (field) => termEl.querySelector(`[data-term-field="${field}"]`)
      return {
        attrKey: String(get("attrKey")?.value || "").trim(),
        coefficient: Number(get("coefficient")?.value || 0),
        perSkillLevel: Number(get("perSkillLevel")?.value || 0)
      }
    })
    .filter((term) => term.attrKey !== "")
}

function collectMutationFromCard(cardEl) {
  const get = (field) => cardEl.querySelector(`[data-field="${field}"]`)
  const mode = String(get("selectorMode")?.value || "all")
  return normalizeMutation({
    scope: get("scope")?.value || "both",
    skillIds: parseCommaList(get("skillIds")?.value || ""),
    effectSelector: {
      mode,
      indexes: mode === "index" ? parseCommaList(get("indexes")?.value || "").map((item) => Number(item)) : []
    },
    ops: {
      targetCountDelta: Number(get("targetCountDelta")?.value || 0),
      amountBaseAddDelta: Number(get("amountBaseAddDelta")?.value || 0),
      amountTermsAppend: collectTermsFromCard(cardEl),
      statusIdsAppend: parseCommaList(get("statusIdsAppend")?.value || ""),
      statusIdsRemove: parseCommaList(get("statusIdsRemove")?.value || "")
    },
    constraints: {
      targetCountMin: Number(get("targetCountMin")?.value || 1)
    }
  })
}

function renderRows() {
  rowsEl.innerHTML = rows
    .map((row) => {
      const selectedClass = row._uid === selectedUID ? ' style="outline:2px solid #2563eb"' : ""
      const rarityPreview = row.allowedRarities.length ? row.allowedRarities.join(", ") : "(空)"
      const mutationCount = Array.isArray(row.effectParams?.effectMutations) ? row.effectParams.effectMutations.length : 0
      return `
      <tr data-uid="${row._uid}"${selectedClass}>
        <td><input data-field="id" value="${escapeHtml(row.id)}" /></td>
        <td>
          <select data-field="effectType">
            <option value="effectMutation" selected>effectMutation</option>
          </select>
          <div class="effect-type-badge">固定：effectMutation</div>
        </td>
        <td>
          <div>${escapeHtml(rarityPreview)}</div>
          <button type="button" data-action="edit-rarities">可视化编辑</button>
        </td>
        <td><input data-field="valuePrecision" type="number" min="0" step="1" value="${row.valuePrecision}" /></td>
        <td><label class="checkbox-inline"><input data-field="active" type="checkbox" ${row.active ? "checked" : ""} />启用</label></td>
        <td>
          <ul class="summary-list">
            <li>变更条目数：${mutationCount}</li>
          </ul>
          <button type="button" data-action="edit-effect-params">可视化编辑</button>
        </td>
        <td>
          <div class="row-actions">
            <button type="button" data-action="copy-effect-params">复制 effectParams</button>
            <button type="button" data-action="remove-row">删除</button>
          </div>
        </td>
      </tr>
    `
    })
    .join("")
}

function copyText(text, successMsg) {
  if (navigator.clipboard && navigator.clipboard.writeText) {
    navigator.clipboard.writeText(text).then(
      () => setStatus(successMsg),
      () => setStatus("复制失败，请检查浏览器权限")
    )
    return
  }
  const temp = document.createElement("textarea")
  temp.value = text
  document.body.appendChild(temp)
  temp.select()
  document.execCommand("copy")
  temp.remove()
  setStatus(successMsg)
}

function toTsvLine(values) {
  return values
    .map((value) => String(value ?? "").replaceAll("\t", " ").replaceAll("\r", " ").replaceAll("\n", " "))
    .join("\t")
}

function exportTsv() {
  const header = ["id", "effectType", "effectParams", "valuePrecision", "allowedRarities", "active"]
  const body = rows.map((row) =>
    toTsvLine([
      row.id.trim(),
      "effectMutation",
      JSON.stringify(normalizeEffectParams(row.effectParams)),
      Math.max(0, Number(row.valuePrecision) || 0),
      JSON.stringify(normalizeStringArray(row.allowedRarities)),
      row.active ? "true" : "false"
    ])
  )
  return [toTsvLine(header), ...body].join("\n")
}

function bindTableEvents() {
  rowsEl.addEventListener("input", (event) => {
    const target = event.target
    if (!(target instanceof HTMLElement)) return
    const tr = target.closest("tr[data-uid]")
    if (!tr) return
    const uidValue = tr.getAttribute("data-uid")
    if (!uidValue) return
    const row = findRow(uidValue)
    if (!row) return
    selectedUID = uidValue
    const field = target.getAttribute("data-field")
    if (!field) return
    if (field === "id" && target instanceof HTMLInputElement) row.id = target.value
    if (field === "valuePrecision" && target instanceof HTMLInputElement) {
      const parsed = Number(target.value)
      row.valuePrecision = Number.isFinite(parsed) && parsed >= 0 ? Math.round(parsed) : 0
    }
    if (field === "effectType") row.effectType = "effectMutation"
  })

  rowsEl.addEventListener("change", (event) => {
    const target = event.target
    if (!(target instanceof HTMLElement)) return
    const tr = target.closest("tr[data-uid]")
    if (!tr) return
    const uidValue = tr.getAttribute("data-uid")
    if (!uidValue) return
    const row = findRow(uidValue)
    if (!row) return
    selectedUID = uidValue
    const field = target.getAttribute("data-field")
    if (field === "active" && target instanceof HTMLInputElement) row.active = target.checked
  })

  rowsEl.addEventListener("click", (event) => {
    const target = event.target
    if (!(target instanceof HTMLElement)) return
    const tr = target.closest("tr[data-uid]")
    if (!tr) return
    const uidValue = tr.getAttribute("data-uid")
    if (!uidValue) return
    selectedUID = uidValue
    const row = findRow(uidValue)
    if (!row) return
    const action = target.getAttribute("data-action")
    if (!action) {
      renderRows()
      return
    }

    if (action === "remove-row") {
      rows = rows.filter((item) => item._uid !== uidValue)
      if (selectedUID === uidValue) selectedUID = rows[0]?._uid || ""
      renderRows()
      return
    }

    if (action === "copy-effect-params") {
      copyText(JSON.stringify(normalizeEffectParams(row.effectParams), null, 2), "已复制 effectParams JSON")
      return
    }

    if (action === "edit-rarities") {
      openModal({
        type: "rarities",
        uid: uidValue,
        title: "allowedRarities 可视化编辑",
        render: () => renderAllowedRarityEditor(row),
        save: () => {
          const checked = Array.from(modalBodyEl.querySelectorAll("[data-rarity-item]:checked"))
            .map((el) => el.getAttribute("data-rarity-item") || "")
            .filter(Boolean)
          row.allowedRarities = normalizeStringArray(checked)
          row.effectType = "effectMutation"
          row.effectParams = normalizeEffectParams(row.effectParams)
          setStatus("allowedRarities 已保存")
        }
      })
      return
    }

    if (action === "edit-effect-params") {
      openModal({
        type: "effectParams",
        uid: uidValue,
        title: "effectParams.effectMutations 可视化编辑",
        render: () => renderEffectParamsEditor(row),
        save: () => {
          const cards = Array.from(modalBodyEl.querySelectorAll(".mutation-card"))
          row.effectParams = {
            effectMutations: cards.map((card) => collectMutationFromCard(card)).filter((item) => item.effectSelector.mode === "all" || item.effectSelector.indexes.length > 0)
          }
          row.effectType = "effectMutation"
          setStatus("effectParams 已保存")
        }
      })
      return
    }
  })
}

function bindModalEvents() {
  modalBodyEl.addEventListener("click", (event) => {
    if (!modalCtx || modalCtx.type !== "effectParams") return
    const target = event.target
    if (!(target instanceof HTMLElement)) return
    if (target.id === "add-mutation-btn") {
      const list = modalBodyEl.querySelector("#mutation-list")
      if (!list) return
      const nextIndex = list.querySelectorAll(".mutation-card").length
      list.insertAdjacentHTML("beforeend", mutationCard(nextIndex, normalizeMutation({})))
      return
    }
    if (target.id === "add-basic-attack-template-btn") {
      const list = modalBodyEl.querySelector("#mutation-list")
      if (!list) return
      const nextIndex = list.querySelectorAll(".mutation-card").length
      list.insertAdjacentHTML(
        "beforeend",
        mutationCard(
          nextIndex,
          normalizeMutation({
            scope: "basic_attack",
            skillIds: [],
            effectSelector: { mode: "all" },
            ops: {
              targetCountDelta: 0,
              amountBaseAddDelta: 0,
              amountTermsAppend: [],
              statusIdsAppend: [],
              statusIdsRemove: []
            },
            constraints: { targetCountMin: 1 }
          })
        )
      )
      return
    }
    if (target.id === "add-sewing10-template-btn") {
      const list = modalBodyEl.querySelector("#mutation-list")
      if (!list) return
      const nextIndex = list.querySelectorAll(".mutation-card").length
      list.insertAdjacentHTML(
        "beforeend",
        mutationCard(
          nextIndex,
          normalizeMutation({
            scope: "basic_attack",
            skillIds: [],
            effectSelector: { mode: "all" },
            ops: {
              targetCountDelta: 0,
              amountBaseAddDelta: 0,
              amountTermsAppend: [{ attrKey: "sewing", coefficient: 10, perSkillLevel: 0 }],
              statusIdsAppend: [],
              statusIdsRemove: []
            },
            constraints: { targetCountMin: 1 }
          })
        )
      )
      return
    }
    const action = target.getAttribute("data-action")
    if (!action) return
    const card = target.closest(".mutation-card")
    if (!card) return
    if (action === "remove-mutation") {
      card.remove()
      return
    }
    if (action === "add-term") {
      const list = card.querySelector("[data-terms-list]")
      if (!list) return
      const nextIndex = list.querySelectorAll("[data-term-index]").length
      list.insertAdjacentHTML(
        "beforeend",
        `
        <div class="list-row term-row" data-term-index="${nextIndex}">
          <div class="form-field">
            <label>属性键</label>
            <input data-term-field="attrKey" value="" list="attr-key-options" />
          </div>
          <div class="form-field">
            <label>系数</label>
            <input data-term-field="coefficient" type="number" step="0.01" value="0" />
          </div>
          <div class="form-field">
            <label>每级增量</label>
            <input data-term-field="perSkillLevel" type="number" step="0.01" value="0" />
          </div>
          <button type="button" data-action="remove-term">删除</button>
        </div>
      `
      )
      return
    }
    if (action === "remove-term") {
      const termRow = target.closest("[data-term-index]")
      if (termRow) termRow.remove()
      return
    }
    if (action === "copy-mutation") {
      const mutation = collectMutationFromCard(card)
      copyText(JSON.stringify(mutation, null, 2), "已复制 mutation JSON")
    }
  })

  modalEl.addEventListener("click", (event) => {
    const target = event.target
    if (!(target instanceof HTMLElement)) return
    if (target.getAttribute("data-action") === "close") {
      closeModal()
    }
  })
  modalSaveBtn.addEventListener("click", () => {
    if (!modalCtx) return
    modalCtx.save()
    renderRows()
    closeModal()
  })
}

function bindToolbarEvents() {
  addRowBtn.addEventListener("click", () => {
    const row = defaultRow()
    rows.unshift(row)
    selectedUID = row._uid
    renderRows()
    setStatus("已新增一行")
  })

  resetBtn.addEventListener("click", () => {
    rows = exampleRows()
    selectedUID = rows[0]._uid
    renderRows()
    setStatus("已重置为示例")
  })

  copyTsvBtn.addEventListener("click", () => {
    copyText(exportTsv(), "已复制 affix_definitions TSV")
  })

  copyExamplesBtn.addEventListener("click", () => {
    copyText(JSON.stringify(EXAMPLE_EFFECT_PARAMS, null, 2), "已复制文档示例 JSON")
  })

  copyEffectParamsBtn.addEventListener("click", () => {
    const targetRow = findRow(selectedUID) || rows[0]
    if (!targetRow) {
      setStatus("暂无可复制行")
      return
    }
    copyText(JSON.stringify(normalizeEffectParams(targetRow.effectParams), null, 2), "已复制当前行 effectParams JSON")
  })
}

function init() {
  rows = exampleRows()
  selectedUID = rows[0]._uid
  renderRows()
  bindTableEvents()
  bindModalEvents()
  bindToolbarEvents()
  setStatus("准备就绪")
}

init()
