const TRIGGER_OPTIONS = [
  { value: "while_active", label: "持续生效（while_active）" },
  { value: "on_attach", label: "挂载时（on_attach）" },
  { value: "on_detach", label: "卸载时（on_detach）" },
  { value: "on_turn_start", label: "回合开始（on_turn_start）" },
  { value: "on_turn_end", label: "回合结束（on_turn_end）" },
  { value: "on_before_take_damage", label: "受击前（on_before_take_damage）" },
  { value: "on_after_take_damage", label: "受击后（on_after_take_damage）" },
  { value: "on_before_heal", label: "受治疗前（on_before_heal）" },
  { value: "on_before_deal_damage", label: "造成伤害前（on_before_deal_damage）" },
  { value: "on_after_deal_damage", label: "造成伤害后（on_after_deal_damage）" }
]

const BUFF_KIND_OPTIONS = [
  { value: "buff", label: "增益（buff）" },
  { value: "debuff", label: "减益（debuff）" }
]

const BOOL_OPTIONS = [
  { value: "false", label: "否（false）" },
  { value: "true", label: "是（true）" }
]

const RESOURCE_OPTIONS = [
  { value: "hp", label: "生命（hp）" },
  { value: "mp", label: "法力（mp）" }
]

const ATTR_OPTIONS = [
  "strength",
  "agility",
  "intelligence",
  "stamina",
  "hp_max",
  "mp_max",
  "physics_damage",
  "magical_damage",
  "defense",
  "crit_chance",
  "crit_damage",
  "speed"
].map((value) => ({ value, label: `${value}` }))

const CONDITION_DEFS = [
  { value: "self_hp_percent_lte", label: "自身生命占比 <= value", params: [{ key: "value", type: "number" }] },
  { value: "self_hp_percent_gte", label: "自身生命占比 >= value", params: [{ key: "value", type: "number" }] },
  { value: "incoming_damage_gte", label: "受到伤害 >= value", params: [{ key: "value", type: "number" }] },
  { value: "incoming_damage_lte", label: "受到伤害 <= value", params: [{ key: "value", type: "number" }] },
  { value: "incoming_heal_gte", label: "受到治疗 >= value", params: [{ key: "value", type: "number" }] },
  { value: "incoming_heal_lte", label: "受到治疗 <= value", params: [{ key: "value", type: "number" }] },
  { value: "has_buff", label: "自身拥有指定状态", params: [{ key: "buffId", type: "buff" }] },
  { value: "source_has_buff", label: "来源拥有指定状态", params: [{ key: "buffId", type: "buff" }] }
]

const EFFECT_DEFS = [
  { value: "add_stat", label: "增加属性值", params: [{ key: "attr", type: "attr" }, { key: "value", type: "number" }] },
  { value: "add_stat_percent", label: "增加属性百分比", params: [{ key: "attr", type: "attr" }, { key: "value", type: "number" }] },
  { value: "heal", label: "治疗", params: [{ key: "value", type: "number" }] },
  { value: "set_heal_value", label: "受治疗固定数值", params: [{ key: "value", type: "number" }] },
  { value: "resource", label: "资源变化", params: [{ key: "resource", type: "resource" }, { key: "value", type: "number" }] },
  { value: "add_buff", label: "附加状态", params: [{ key: "buffId", type: "buff" }, { key: "durationMs", type: "number" }, { key: "payload", type: "json-object" }] },
  { value: "remove_buff", label: "移除状态", params: [{ key: "buffId", type: "buff" }] },
  { value: "reduce_damage_percent", label: "受击伤害降低%", params: [{ key: "value", type: "number" }] },
  { value: "set_damage_multiplier", label: "受击倍率设置", params: [{ key: "value", type: "number" }] },
  { value: "set_damage_value", label: "受击固定伤害值", params: [{ key: "value", type: "number" }] },
  { value: "consume_charge", label: "消耗充能并移除状态", params: [{ key: "buffId", type: "buff" }] },
  { value: "add_damage_percent", label: "造成伤害增加%", params: [{ key: "value", type: "number" }] },
  { value: "add_damage_flat", label: "造成伤害固定增加", params: [{ key: "value", type: "number" }] },
  { value: "untargetable_by_enemy", label: "敌方点选不可选", params: [] },
  { value: "untargetable_by_ally", label: "友方点选不可选", params: [] },
  { value: "treat_all_as_enemy", label: "敌方目标视作全体敌人", params: [] },
  { value: "camouflage_targeting_remap", label: "伪装索敌重映射", params: [] },
  { value: "prefer_as_enemy_target", label: "吸引敌方索敌", params: [] },
  { value: "avoid_as_enemy_target", label: "回避敌方索敌", params: [] },
  { value: "prefer_as_ally_target", label: "吸引友方索敌", params: [] },
  { value: "avoid_as_ally_target", label: "回避友方索敌", params: [] },
  { value: "turn_action_suppressed", label: "禁止行动", params: [] },
  { value: "skill_action_suppressed", label: "禁止主动技能，仍可普攻", params: [] },
  { value: "basic_attack_suppressed", label: "禁止普攻", params: [] },
  { value: "frontline_fixation", label: "对敌索敌固定从前排开始", params: [] },
  { value: "position_override", label: "临时改变站位", params: [{ key: "position", type: "select", options: ["vanguard", "midguard", "rearguard"] }] },
  { value: "damage_redirect_to_source", label: "伤害转移至状态来源", params: [{ key: "percent", type: "number", defaultValue: 1 }] }
]

const state = {
  buffTypes: [],
  buffs: [],
  buffEffects: [],
  editor: null
}

const statusEl = document.querySelector("#status")
const statusBodyEl = document.querySelector("#status-body")
const modalEl = document.querySelector("#editor-modal")
const editorTitleEl = document.querySelector("#editor-title")
const editorContentEl = document.querySelector("#editor-content")

init()

function init() {
  resetAll()
  bindEvents()
}

function bindEvents() {
  document.querySelector("#add-status-btn").addEventListener("click", () => openEditor("create", null))
  document.querySelector("#copy-all-btn").addEventListener("click", copyAllTsv)
  document.querySelector("#download-all-btn").addEventListener("click", downloadAllTsv)
  document.querySelector("#reset-all-btn").addEventListener("click", resetAll)

  statusBodyEl.addEventListener("click", handleStatusTableClick)
  modalEl.addEventListener("click", handleModalClick)
  editorContentEl.addEventListener("input", handleModalInput)
  editorContentEl.addEventListener("change", handleModalInput)
}

function resetAll() {
  state.buffTypes = [
    {
      id: "utility",
      nameKey: "buff_type.utility.name",
      descKey: "buff_type.utility.desc",
      color: "#409EFF"
    },
    {
      id: "control",
      nameKey: "buff_type.control.name",
      descKey: "buff_type.control.desc",
      color: "#E6A23C"
    }
  ]
  state.buffs = [
    {
      id: "stealth",
      nameKey: "buff.stealth.name",
      descKey: "buff.stealth.desc",
      durationTurns: 2,
      permanent: false,
      kind: "buff",
      dispellable: true,
      type: "utility"
    },
    {
      id: "panic",
      nameKey: "buff.panic.name",
      descKey: "buff.panic.desc",
      durationTurns: 2,
      permanent: false,
      kind: "debuff",
      dispellable: true,
      type: "control"
    }
  ]
  state.buffEffects = [
    {
      id: "stealth_untargetable_while_active",
      buffId: "stealth",
      trigger: "while_active",
      priority: 0,
      conditions: [],
      effects: [{ kind: "untargetable_by_enemy" }]
    },
    {
      id: "panic_treat_all_as_enemy_while_active",
      buffId: "panic",
      trigger: "while_active",
      priority: 0,
      conditions: [],
      effects: [{ kind: "treat_all_as_enemy" }]
    }
  ]
  renderStatusTable()
  toast("已重置示例")
}

function renderStatusTable() {
  const rows = state.buffs
    .map((buff) => {
      const ruleCount = state.buffEffects.filter((row) => row.buffId === buff.id).length
      return `
        <tr>
          <td>${escapeHtml(buff.id)}</td>
          <td>${escapeHtml(mapBuffKind(buff.kind))}</td>
          <td>${escapeHtml(buff.type || "空")}</td>
          <td>${buff.durationTurns}</td>
          <td>${buff.permanent ? "是" : "否"}</td>
          <td>${buff.dispellable ? "是" : "否"}</td>
          <td>${ruleCount}</td>
          <td class="actions">
            <button data-action="edit-status" data-id="${escapeHtml(buff.id)}">编辑</button>
            <button data-action="duplicate-status" data-id="${escapeHtml(buff.id)}">复制</button>
            <button data-action="delete-status" data-id="${escapeHtml(buff.id)}">删除</button>
          </td>
        </tr>
      `
    })
    .join("")
  statusBodyEl.innerHTML = rows || '<tr><td colspan="8" class="empty-cell">暂无状态，点击“新增状态”开始</td></tr>'
}

function handleStatusTableClick(event) {
  const button = event.target.closest("button[data-action]")
  if (!button) return
  const action = button.dataset.action
  const id = button.dataset.id
  if (!action || !id) return

  if (action === "edit-status") {
    openEditor("edit", id)
    return
  }
  if (action === "duplicate-status") {
    duplicateStatus(id)
    return
  }
  if (action === "delete-status") {
    deleteStatus(id)
  }
}

function duplicateStatus(id) {
  const source = state.buffs.find((row) => row.id === id)
  if (!source) return
  const copyId = `${source.id}_copy`
  if (state.buffs.some((row) => row.id === copyId)) {
    toast(`复制失败：${copyId} 已存在`)
    return
  }
  const next = { ...source, id: copyId, nameKey: `buff.${copyId}.name`, descKey: `buff.${copyId}.desc` }
  state.buffs.push(next)
  const linked = state.buffEffects.filter((row) => row.buffId === source.id)
  linked.forEach((rule, index) => {
    state.buffEffects.push({
      ...rule,
      id: `${copyId}_rule_${index + 1}`,
      buffId: copyId,
      conditions: structuredClone(rule.conditions || []),
      effects: structuredClone(rule.effects || [])
    })
  })
  renderStatusTable()
  toast(`已复制状态 ${id} -> ${copyId}`)
}

function deleteStatus(id) {
  state.buffs = state.buffs.filter((row) => row.id !== id)
  state.buffEffects = state.buffEffects.filter((row) => row.buffId !== id)
  renderStatusTable()
  toast(`已删除状态 ${id}`)
}

function openEditor(mode, id) {
  const isCreate = mode === "create"
  const defaultId = isCreate ? uniqueBuffId("new_buff") : id
  const buff = isCreate
    ? {
        id: defaultId,
        nameKey: `buff.${defaultId}.name`,
        descKey: `buff.${defaultId}.desc`,
        durationTurns: 2,
        maxCharges: 0,
        permanent: false,
        kind: "buff",
        dispellable: true,
        type: state.buffTypes[0]?.id || "utility"
      }
    : structuredClone(state.buffs.find((row) => row.id === id))
  if (!buff) return

  const currentType = state.buffTypes.find((row) => row.id === buff.type)
  const typeDraft = currentType
    ? structuredClone(currentType)
    : {
        id: buff.type || uniqueTypeId("new_type"),
        nameKey: `buff_type.${buff.type || "new_type"}.name`,
        descKey: `buff_type.${buff.type || "new_type"}.desc`,
        color: "#409EFF"
      }
  const rules = isCreate
    ? [makeDefaultRule(buff.id)]
    : state.buffEffects.filter((row) => row.buffId === id).map((row) => structuredClone(row))
  state.editor = {
    mode,
    originalId: id,
    buff,
    type: typeDraft,
    rules: rules.length ? rules : [makeDefaultRule(buff.id)]
  }
  renderEditor()
  modalEl.classList.remove("hidden")
  modalEl.setAttribute("aria-hidden", "false")
}

function renderEditor() {
  if (!state.editor) return
  editorTitleEl.textContent = state.editor.mode === "create" ? "新增状态（含分类与规则）" : `编辑状态：${state.editor.originalId}`
  editorContentEl.innerHTML = `
    <section class="panel modal-section">
      <h3>状态基础（buff）</h3>
      <div class="editor-grid">
        ${renderInputField("状态ID", "editor.buff.id", state.editor.buff.id, "text")}
        ${renderInputField("名称Key", "editor.buff.nameKey", state.editor.buff.nameKey, "text")}
        ${renderInputField("描述Key", "editor.buff.descKey", state.editor.buff.descKey, "text")}
        ${renderInputField("持续回合", "editor.buff.durationTurns", state.editor.buff.durationTurns, "number")}
        ${renderInputField("触发次数上限（maxCharges）", "editor.buff.maxCharges", state.editor.buff.maxCharges ?? 0, "number")}
        ${renderSelectField("状态类型", "editor.buff.kind", BUFF_KIND_OPTIONS, state.editor.buff.kind)}
        ${renderSelectField("永久状态", "editor.buff.permanent", BOOL_OPTIONS, String(Boolean(state.editor.buff.permanent)))}
        ${renderSelectField("可驱散", "editor.buff.dispellable", BOOL_OPTIONS, String(Boolean(state.editor.buff.dispellable)))}
        ${renderSelectField("分类ID（引用 buff_type）", "editor.buff.type", typeOptionList(), state.editor.buff.type)}
      </div>
    </section>

    <section class="panel modal-section">
      <h3>分类信息（buff_type）</h3>
      <div class="editor-grid">
        ${renderInputField("分类ID", "editor.type.id", state.editor.type.id, "text")}
        ${renderInputField("分类名称Key", "editor.type.nameKey", state.editor.type.nameKey, "text")}
        ${renderInputField("分类描述Key", "editor.type.descKey", state.editor.type.descKey, "text")}
        ${renderInputField("颜色（HEX）", "editor.type.color", state.editor.type.color, "text")}
      </div>
    </section>

    <section class="panel modal-section">
      <header class="section-head">
        <h3>状态规则（buff_effects）</h3>
        <button type="button" data-action="add-rule">新增规则</button>
      </header>
      <div class="rules-wrap">
        ${state.editor.rules.map((rule, ruleIndex) => renderRuleCard(rule, ruleIndex)).join("")}
      </div>
    </section>
  `
}

function renderRuleCard(rule, ruleIndex) {
  return `
    <article class="rule-card">
      <header class="rule-head">
        <strong>规则 #${ruleIndex + 1}</strong>
        <button type="button" data-action="remove-rule" data-rule-index="${ruleIndex}">删除规则</button>
      </header>
      <div class="editor-grid">
        ${renderInputField("规则ID", `editor.rules.${ruleIndex}.id`, rule.id, "text")}
        ${renderSelectField("触发时机", `editor.rules.${ruleIndex}.trigger`, TRIGGER_OPTIONS, rule.trigger)}
        ${renderInputField("优先级", `editor.rules.${ruleIndex}.priority`, rule.priority, "number")}
      </div>
      <div class="visual-grid">
        <section>
          <div class="visual-title-row">
            <strong>条件（conditions）</strong>
            <button type="button" data-action="add-condition" data-rule-index="${ruleIndex}">新增条件</button>
          </div>
          ${(rule.conditions || []).map((item, idx) => renderConditionItem(ruleIndex, idx, item)).join("") || '<div class="empty-cell">无条件（默认总是触发）</div>'}
        </section>
        <section>
          <div class="visual-title-row">
            <strong>效果（effects）</strong>
            <button type="button" data-action="add-effect" data-rule-index="${ruleIndex}">新增效果</button>
          </div>
          ${(rule.effects || []).map((item, idx) => renderEffectItem(ruleIndex, idx, item)).join("")}
        </section>
      </div>
      <details class="json-preview">
        <summary>查看 JSON 预览（只读）</summary>
        <div class="code-grid">
          <label>conditions</label>
          <textarea readonly rows="4">${escapeHtml(JSON.stringify(normalizeConditionList(rule.conditions), null, 2))}</textarea>
          <label>effects</label>
          <textarea readonly rows="6">${escapeHtml(JSON.stringify(normalizeEffectList(rule.effects), null, 2))}</textarea>
        </div>
      </details>
    </article>
  `
}

function renderConditionItem(ruleIndex, conditionIndex, condition) {
  const kind = condition?.kind || CONDITION_DEFS[0].value
  const def = CONDITION_DEFS.find((item) => item.value === kind) || CONDITION_DEFS[0]
  return `
    <div class="visual-item">
      <div class="visual-item-head">
        <span>${escapeHtml(def.label)}</span>
        <button type="button" data-action="remove-condition" data-rule-index="${ruleIndex}" data-condition-index="${conditionIndex}">删除</button>
      </div>
      <div class="editor-grid">
        ${renderSelectField("条件类型", `editor.rules.${ruleIndex}.conditions.${conditionIndex}.kind`, CONDITION_DEFS, kind)}
        ${def.params.map((param) => renderParamField(`editor.rules.${ruleIndex}.conditions.${conditionIndex}.${param.key}`, param, condition?.[param.key])).join("")}
      </div>
    </div>
  `
}

function renderEffectItem(ruleIndex, effectIndex, effect) {
  const kind = effect?.kind || EFFECT_DEFS[0].value
  const def = EFFECT_DEFS.find((item) => item.value === kind) || EFFECT_DEFS[0]
  return `
    <div class="visual-item">
      <div class="visual-item-head">
        <span>${escapeHtml(def.label)}</span>
        <button type="button" data-action="remove-effect" data-rule-index="${ruleIndex}" data-effect-index="${effectIndex}">删除</button>
      </div>
      <div class="editor-grid">
        ${renderSelectField("效果类型", `editor.rules.${ruleIndex}.effects.${effectIndex}.kind`, EFFECT_DEFS, kind)}
        ${def.params.map((param) => renderParamField(`editor.rules.${ruleIndex}.effects.${effectIndex}.${param.key}`, param, effect?.[param.key])).join("")}
      </div>
    </div>
  `
}

function renderParamField(path, param, value) {
  if (param.type === "number") {
    return renderInputField(param.key, path, value ?? 0, "number")
  }
  if (param.type === "attr") {
    return renderSelectField(param.key, path, ATTR_OPTIONS, value || ATTR_OPTIONS[0].value)
  }
  if (param.type === "resource") {
    return renderSelectField(param.key, path, RESOURCE_OPTIONS, value || "hp")
  }
  if (param.type === "buff") {
    return renderSelectField(param.key, path, buffOptionList(), value || "")
  }
  if (param.type === "json-object") {
    return renderInputField(param.key, path, value || "{}", "text")
  }
  return renderInputField(param.key, path, value || "", "text")
}

function renderInputField(label, bind, value, type) {
  return `
    <label class="editor-field">
      <span>${escapeHtml(label)}</span>
      <input type="${type}" data-bind="${bind}" value="${escapeHtml(String(value ?? ""))}" />
    </label>
  `
}

function renderSelectField(label, bind, options, selected) {
  return `
    <label class="editor-field">
      <span>${escapeHtml(label)}</span>
      <select data-bind="${bind}">
        ${options
          .map((option) => {
            const item = typeof option === "string" ? { value: option, label: option } : option
            const isSelected = String(item.value) === String(selected) ? "selected" : ""
            return `<option value="${escapeHtml(item.value)}" ${isSelected}>${escapeHtml(item.label)}</option>`
          })
          .join("")}
      </select>
    </label>
  `
}

function typeOptionList() {
  const list = state.buffTypes.map((row) => ({ value: row.id, label: `${row.id}（已存在）` }))
  if (state.editor?.type?.id && !list.some((row) => row.value === state.editor.type.id)) {
    list.unshift({ value: state.editor.type.id, label: `${state.editor.type.id}（当前）` })
  }
  return list
}

function buffOptionList() {
  const allIds = Array.from(new Set([...state.buffs.map((row) => row.id), state.editor?.buff?.id].filter(Boolean)))
  return [{ value: "", label: "空" }, ...allIds.map((id) => ({ value: id, label: id }))]
}

function handleModalInput(event) {
  const target = event.target
  if (!(target instanceof HTMLElement) || !state.editor) return
  const bind = target.dataset.bind
  if (!bind) return
  const value = target.value
  setByPath(state, bind, value)

  if (bind === "editor.buff.id") {
    syncIdsWithBuffId()
  }
  if (bind === "editor.buff.type") {
    const match = state.buffTypes.find((row) => row.id === value)
    if (match) {
      state.editor.type = structuredClone(match)
      renderEditor()
      return
    }
  }
  if (bind.endsWith(".kind")) {
    normalizeKindShape(bind)
    renderEditor()
  }
}

function handleModalClick(event) {
  const actionEl = event.target.closest("[data-action]")
  if (!actionEl || !state.editor) return
  const action = actionEl.dataset.action
  if (!action) return

  if (action === "close-editor") {
    closeEditor()
    return
  }
  if (action === "save-editor") {
    saveEditor()
    return
  }

  const ruleIndex = Number(actionEl.dataset.ruleIndex)
  if (action === "add-rule") {
    state.editor.rules.push(makeDefaultRule(state.editor.buff.id))
    renderEditor()
    return
  }
  if (action === "remove-rule" && Number.isInteger(ruleIndex)) {
    state.editor.rules.splice(ruleIndex, 1)
    if (state.editor.rules.length === 0) state.editor.rules.push(makeDefaultRule(state.editor.buff.id))
    renderEditor()
    return
  }
  if (action === "add-condition" && Number.isInteger(ruleIndex)) {
    state.editor.rules[ruleIndex].conditions.push(makeDefaultCondition())
    renderEditor()
    return
  }
  if (action === "remove-condition" && Number.isInteger(ruleIndex)) {
    const conditionIndex = Number(actionEl.dataset.conditionIndex)
    state.editor.rules[ruleIndex].conditions.splice(conditionIndex, 1)
    renderEditor()
    return
  }
  if (action === "add-effect" && Number.isInteger(ruleIndex)) {
    state.editor.rules[ruleIndex].effects.push(makeDefaultEffect())
    renderEditor()
    return
  }
  if (action === "remove-effect" && Number.isInteger(ruleIndex)) {
    const effectIndex = Number(actionEl.dataset.effectIndex)
    state.editor.rules[ruleIndex].effects.splice(effectIndex, 1)
    if (state.editor.rules[ruleIndex].effects.length === 0) state.editor.rules[ruleIndex].effects.push(makeDefaultEffect())
    renderEditor()
  }
}

function syncIdsWithBuffId() {
  if (!state.editor) return
  const id = asId(state.editor.buff.id)
  state.editor.buff.id = id
  if (!asId(state.editor.buff.nameKey)) state.editor.buff.nameKey = `buff.${id}.name`
  if (!asId(state.editor.buff.descKey)) state.editor.buff.descKey = `buff.${id}.desc`
  state.editor.rules.forEach((rule, index) => {
    rule.buffId = id
    if (!asId(rule.id) || rule.id.startsWith(`${state.editor.originalId || "new_buff"}_`)) {
      rule.id = `${id}_rule_${index + 1}`
    }
  })
}

function normalizeKindShape(path) {
  const value = getByPath(state, path)
  if (!path.includes(".conditions.") && !path.includes(".effects.")) return
  const listPath = path.includes(".conditions.") ? path.split(".kind")[0] : path.split(".kind")[0]
  const current = getByPath(state, listPath)
  const defs = path.includes(".conditions.") ? CONDITION_DEFS : EFFECT_DEFS
  const match = defs.find((item) => item.value === value) || defs[0]
  const seed = { kind: match.value }
  match.params.forEach((param) => {
    seed[param.key] = defaultParamValue(param)
  })
  setByPath(state, listPath, seed)
}

function saveEditor() {
  if (!state.editor) return
  const draft = state.editor

  draft.buff.id = asId(draft.buff.id)
  if (!draft.buff.id) {
    toast("保存失败：状态ID不能为空")
    return
  }
  draft.type.id = asId(draft.type.id || draft.buff.type)
  if (!draft.type.id) {
    toast("保存失败：分类ID不能为空")
    return
  }

  draft.buff.type = draft.type.id
  draft.buff.durationTurns = toNumber(draft.buff.durationTurns)
  draft.buff.maxCharges = toNumber(draft.buff.maxCharges)
  draft.buff.permanent = toBool(draft.buff.permanent)
  draft.buff.dispellable = toBool(draft.buff.dispellable)
  if (!asId(draft.buff.nameKey)) draft.buff.nameKey = `buff.${draft.buff.id}.name`
  if (!asId(draft.buff.descKey)) draft.buff.descKey = `buff.${draft.buff.id}.desc`
  if (!asId(draft.type.nameKey)) draft.type.nameKey = `buff_type.${draft.type.id}.name`
  if (!asId(draft.type.descKey)) draft.type.descKey = `buff_type.${draft.type.id}.desc`
  if (!/^#[0-9a-fA-F]{6}$/.test(draft.type.color || "")) draft.type.color = "#409EFF"

  const duplicateId = state.buffs.some((row) => row.id === draft.buff.id && row.id !== draft.originalId)
  if (duplicateId) {
    toast(`保存失败：状态ID ${draft.buff.id} 已存在`)
    return
  }

  const normalizedRules = draft.rules.map((rule, index) => ({
    id: asId(rule.id) || `${draft.buff.id}_rule_${index + 1}`,
    buffId: draft.buff.id,
    trigger: normalizeTrigger(rule.trigger),
    priority: toNumber(rule.priority),
    conditions: normalizeConditionList(rule.conditions),
    effects: normalizeEffectList(rule.effects)
  }))
  if (normalizedRules.some((row) => row.effects.length === 0)) {
    toast("保存失败：每条规则至少需要一个效果")
    return
  }

  upsertType(draft.type)
  upsertBuff(draft.buff, draft.originalId)
  state.buffEffects = state.buffEffects.filter((row) => row.buffId !== draft.originalId && row.buffId !== draft.buff.id)
  state.buffEffects.push(...normalizedRules)

  closeEditor()
  renderStatusTable()
  toast(`已保存状态 ${draft.buff.id}`)
}

function upsertType(typeDraft) {
  const index = state.buffTypes.findIndex((row) => row.id === typeDraft.id)
  if (index >= 0) state.buffTypes[index] = structuredClone(typeDraft)
  else state.buffTypes.push(structuredClone(typeDraft))
}

function upsertBuff(buffDraft, originalId) {
  if (originalId && originalId !== buffDraft.id) {
    const oldIndex = state.buffs.findIndex((row) => row.id === originalId)
    if (oldIndex >= 0) state.buffs.splice(oldIndex, 1)
  }
  const existing = state.buffs.findIndex((row) => row.id === buffDraft.id)
  if (existing >= 0) state.buffs[existing] = structuredClone(buffDraft)
  else state.buffs.push(structuredClone(buffDraft))
}

function closeEditor() {
  state.editor = null
  modalEl.classList.add("hidden")
  modalEl.setAttribute("aria-hidden", "true")
  editorContentEl.innerHTML = ""
}

function copyAllTsv() {
  const content = ["### buff_type", buildBuffTypeTsv(), "", "### buff", buildBuffTsv(), "", "### buff_effects", buildBuffEffectsTsv()].join("\n")
  copyText(content)
  toast("已复制全部 TSV（分段）")
}

function downloadAllTsv() {
  downloadText("buff_type.tsv", buildBuffTypeTsv())
  downloadText("buff.tsv", buildBuffTsv())
  downloadText("buff_effects.tsv", buildBuffEffectsTsv())
  toast("已下载 3 个 TSV 文件")
}

function buildBuffTypeTsv() {
  const lines = ["id\tnameKey\tdescKey\tcolor"]
  for (const row of state.buffTypes) {
    lines.push([row.id, row.nameKey, row.descKey, row.color].map(sanitizeCell).join("\t"))
  }
  return lines.join("\n")
}

function buildBuffTsv() {
  const lines = ["id\tnameKey\tdescKey\tdurationTurns\tmaxCharges\tpermanent\tkind\tdispellable\ttype"]
  for (const row of state.buffs) {
    lines.push(
      [
        row.id,
        row.nameKey,
        row.descKey,
        row.durationTurns,
        row.maxCharges ?? 0,
        row.permanent ? "true" : "false",
        row.kind,
        row.dispellable ? "true" : "false",
        row.type
      ]
        .map(sanitizeCell)
        .join("\t")
    )
  }
  return lines.join("\n")
}

function buildBuffEffectsTsv() {
  const lines = ["id\tbuffId\ttrigger\tpriority\tconditions\teffects"]
  for (const row of state.buffEffects) {
    lines.push(
      [
        row.id,
        row.buffId,
        row.trigger,
        row.priority,
        JSON.stringify(normalizeConditionList(row.conditions)),
        JSON.stringify(normalizeEffectList(row.effects))
      ]
        .map(sanitizeCell)
        .join("\t")
    )
  }
  return lines.join("\n")
}

function sanitizeCell(value) {
  return String(value ?? "").replace(/\t/g, "    ").replace(/\r?\n/g, "\\n")
}

function makeDefaultRule(buffId) {
  return {
    id: `${buffId || "new_buff"}_rule_1`,
    buffId: buffId || "new_buff",
    trigger: "while_active",
    priority: 0,
    conditions: [],
    effects: [makeDefaultEffect()]
  }
}

function makeDefaultCondition() {
  return { kind: "self_hp_percent_lte", value: 0.5 }
}

function makeDefaultEffect() {
  return { kind: "add_stat", attr: "strength", value: 1 }
}

function normalizeConditionList(conditions) {
  if (!Array.isArray(conditions)) return []
  return conditions
    .map((item) => {
      const kind = asId(item?.kind)
      const def = CONDITION_DEFS.find((row) => row.value === kind)
      if (!def) return null
      const next = { kind: def.value }
      def.params.forEach((param) => {
        const value = item?.[param.key]
        if (param.type === "number") next[param.key] = toNumber(value)
        else if (param.type === "buff" && asId(value)) next[param.key] = asId(value)
      })
      return next
    })
    .filter(Boolean)
}

function normalizeEffectList(effects) {
  if (!Array.isArray(effects)) return []
  return effects
    .map((item) => {
      const kind = asId(item?.kind)
      const def = EFFECT_DEFS.find((row) => row.value === kind)
      if (!def) return null
      const next = { kind: def.value }
      def.params.forEach((param) => {
        const value = item?.[param.key]
        if (param.type === "number") {
          if (String(value ?? "").trim() !== "") next[param.key] = toNumber(value)
        } else if (param.type === "attr") {
          if (asId(value)) next[param.key] = asId(value)
        } else if (param.type === "resource") {
          if (asId(value)) next[param.key] = asId(value)
        } else if (param.type === "buff") {
          if (asId(value)) next[param.key] = asId(value)
        } else if (param.type === "json-object") {
          const normalized = normalizeJsonObjectText(value)
          if (normalized) next[param.key] = normalized
        }
      })
      return next
    })
    .filter(Boolean)
}

function normalizeJsonObjectText(value) {
  const raw = String(value ?? "").trim()
  if (!raw || raw === "{}") return undefined
  try {
    const parsed = JSON.parse(raw)
    if (parsed && typeof parsed === "object" && !Array.isArray(parsed)) return parsed
  } catch {}
  return undefined
}

function normalizeTrigger(value) {
  const normalized = asId(value)
  return TRIGGER_OPTIONS.some((row) => row.value === normalized) ? normalized : "while_active"
}

function mapBuffKind(value) {
  const hit = BUFF_KIND_OPTIONS.find((row) => row.value === value)
  return hit ? hit.label : value || ""
}

function uniqueBuffId(base) {
  let id = base
  let index = 1
  while (state.buffs.some((row) => row.id === id)) {
    id = `${base}_${index}`
    index += 1
  }
  return id
}

function uniqueTypeId(base) {
  let id = base
  let index = 1
  while (state.buffTypes.some((row) => row.id === id)) {
    id = `${base}_${index}`
    index += 1
  }
  return id
}

function setByPath(root, path, value) {
  const segments = path.split(".")
  let current = root
  for (let i = 0; i < segments.length - 1; i += 1) {
    const key = maybeNumber(segments[i])
    current = current[key]
  }
  const lastKey = maybeNumber(segments[segments.length - 1])
  current[lastKey] = value
}

function getByPath(root, path) {
  return path.split(".").reduce((acc, key) => (acc == null ? undefined : acc[maybeNumber(key)]), root)
}

function maybeNumber(value) {
  return /^\d+$/.test(value) ? Number(value) : value
}

function toNumber(value) {
  const num = Number(value)
  return Number.isFinite(num) ? num : 0
}

function toBool(value) {
  return String(value) === "true" || value === true
}

function asId(value) {
  return String(value ?? "").trim()
}

function defaultParamValue(param) {
  if (param.type === "number") return 0
  if (param.type === "attr") return ATTR_OPTIONS[0]?.value || ""
  if (param.type === "resource") return "hp"
  if (param.type === "buff") return ""
  if (param.type === "json-object") return "{}"
  return ""
}

function copyText(text) {
  if (navigator.clipboard?.writeText) {
    navigator.clipboard.writeText(text).catch(() => fallbackCopy(text))
    return
  }
  fallbackCopy(text)
}

function fallbackCopy(text) {
  const textarea = document.createElement("textarea")
  textarea.value = text
  document.body.appendChild(textarea)
  textarea.select()
  document.execCommand("copy")
  document.body.removeChild(textarea)
}

function downloadText(filename, content) {
  const blob = new Blob([content], { type: "text/tab-separated-values;charset=utf-8" })
  const url = URL.createObjectURL(blob)
  const anchor = document.createElement("a")
  anchor.href = url
  anchor.download = filename
  anchor.click()
  URL.revokeObjectURL(url)
}

function toast(message) {
  statusEl.textContent = message
}

function escapeHtml(value) {
  return String(value ?? "")
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
}
