const columns = [
  { key: "id", label: "id", type: "text" },
  { key: "name", label: "name", type: "text" },
  { key: "category", label: "category", type: "text" },
  { key: "hpCost", label: "hpCost", type: "number" },
  { key: "mpCost", label: "mpCost", type: "number" },
  { key: "cooldown", label: "cooldown", type: "number" },
  { key: "damageType", label: "damageType", type: "select", options: ["physical", "magical"] },
  { key: "targetDirection", label: "targetDirection", type: "select", options: ["vanguard", "midguard", "rearguard"] },
  { key: "targetMode", label: "targetMode", type: "select", options: ["single", "multi", "all"] },
  { key: "targetBaseCount", label: "targetBaseCount", type: "number" },
  {
    key: "targetSeekStrategy",
    label: "targetSeekStrategy",
    type: "select",
    options: ["front_to_rear", "rear_to_front", "random_alive", "none"]
  },
  { key: "statusRequirement", label: "statusRequirement", type: "json" },
  { key: "weaponRequirement", label: "weaponRequirement", type: "json" },
  { key: "damageFormula", label: "damageFormula", type: "json" },
  { key: "targetFormula", label: "targetFormula", type: "json" },
  { key: "targeting", label: "targeting", type: "visual-targeting" },
  { key: "effects", label: "effects", type: "visual-effects" }
]

const templates = {
  damage: {
    name: "伤害模板",
    row: () => ({
      id: "damage_template",
      name: "",
      category: "",
      hpCost: 0,
      mpCost: 0,
      cooldown: 2,
      damageType: "physical",
      targetDirection: "vanguard",
      targetMode: "single",
      targetBaseCount: 1,
      targetSeekStrategy: "front_to_rear",
      statusRequirement: "{}",
      weaponRequirement: "[]",
      damageFormula: '[{"attrKey":"physics_damage","coefficient":1.2,"perSkillLevel":0}]',
      targetFormula: "[]",
      targeting: JSON.stringify(
        {
          targetPositions: ["vanguard"],
          secondarySeek: "front_to_rear",
          targetScope: "enemy",
          selectRule: "first",
          selectAttrKey: "",
          fallbackSelfIfEmpty: false,
          targetCount: { base: 1, formula: [] }
        },
        null,
        2
      ),
      effects: JSON.stringify(
        [
          {
            kind: "damage",
            damageType: "physical",
            targeting: {
              targetPositions: ["vanguard"],
              secondarySeek: "front_to_rear",
              targetScope: "enemy",
              selectRule: "first",
              selectAttrKey: "",
              fallbackSelfIfEmpty: false,
              targetCount: { base: 1, formula: [] }
            },
            amountFormula: {
              baseAdd: 0,
              terms: [{ attrKey: "physics_damage", coefficient: 1.2, perSkillLevel: 0 }]
            }
          }
        ],
        null,
        2
      )
    })
  },
  heal: {
    name: "治疗模板",
    row: () => ({
      id: "heal_template",
      name: "",
      category: "",
      hpCost: 0,
      mpCost: 10,
      cooldown: 3,
      damageType: "magical",
      targetDirection: "vanguard",
      targetMode: "single",
      targetBaseCount: 1,
      targetSeekStrategy: "front_to_rear",
      statusRequirement: "{}",
      weaponRequirement: "[]",
      damageFormula: '[{"attrKey":"magical_damage","coefficient":1.2,"perSkillLevel":0},{"attrKey":"intelligence","coefficient":0.6,"perSkillLevel":0}]',
      targetFormula: "[]",
      targeting: JSON.stringify(
        {
          targetPositions: ["vanguard", "midguard", "rearguard"],
          secondarySeek: "none",
          targetScope: "ally",
          selectRule: "hp_percent_min",
          selectAttrKey: "",
          fallbackSelfIfEmpty: true,
          targetCount: { base: 1, formula: [] }
        },
        null,
        2
      ),
      effects: JSON.stringify(
        [
          {
            kind: "heal",
            damageType: "magical",
            targeting: {
              targetPositions: ["vanguard", "midguard", "rearguard"],
              secondarySeek: "none",
              targetScope: "ally",
              selectRule: "hp_percent_min",
              selectAttrKey: "",
              fallbackSelfIfEmpty: true,
              targetCount: { base: 1, formula: [] }
            },
            amountFormula: {
              baseAdd: 0,
              terms: [
                { attrKey: "magical_damage", coefficient: 1.2, perSkillLevel: 0 },
                { attrKey: "intelligence", coefficient: 0.6, perSkillLevel: 0 }
              ]
            }
          }
        ],
        null,
        2
      )
    })
  },
  status: {
    name: "状态模板",
    row: () => ({
      id: "status_template",
      name: "",
      category: "",
      hpCost: 0,
      mpCost: 15,
      cooldown: 4,
      damageType: "magical",
      targetDirection: "vanguard",
      targetMode: "single",
      targetBaseCount: 1,
      targetSeekStrategy: "front_to_rear",
      statusRequirement: "{}",
      weaponRequirement: "[]",
      damageFormula: "[]",
      targetFormula: "[]",
      targeting: JSON.stringify(
        {
          targetPositions: ["vanguard", "midguard", "rearguard"],
          secondarySeek: "none",
          targetScope: "ally",
          selectRule: "hp_percent_min",
          selectAttrKey: "",
          fallbackSelfIfEmpty: true,
          targetCount: { base: 999, formula: [] }
        },
        null,
        2
      ),
      effects: JSON.stringify(
        [
          {
            kind: "status",
            targeting: {
              targetPositions: ["vanguard", "midguard", "rearguard"],
              secondarySeek: "none",
              targetScope: "ally",
              selectRule: "hp_percent_min",
              selectAttrKey: "",
              fallbackSelfIfEmpty: true,
              targetCount: { base: 999, formula: [] }
            },
            statusPayload: {
              statusIds: ["blessing"],
              durationBaseTurns: 0,
              durationPerSkillLevelTurns: 0
            }
          }
        ],
        null,
        2
      )
    })
  }
}

const DEFAULT_CATEGORY_OPTIONS = ["blade", "spear", "bow", "arcane", "holy", "tactical"]

const POSITION_OPTIONS = [
  { value: "vanguard", label: "前排" },
  { value: "midguard", label: "中排" },
  { value: "rearguard", label: "后排" }
]

const TARGET_SCOPE_OPTIONS = [
  { value: "ally", label: "友军" },
  { value: "enemy", label: "敌军" },
  { value: "all", label: "全部" },
  { value: "self", label: "自身" }
]

const SECONDARY_SEEK_OPTIONS = [
  { value: "front_to_rear", label: "前排 -> 后排" },
  { value: "rear_to_front", label: "后排 -> 前排" },
  { value: "random_alive", label: "随机" },
  { value: "none", label: "无" }
]

const SELECT_RULE_OPTIONS = [
  { value: "hp_max", label: "生命最高" },
  { value: "hp_min", label: "生命最低" },
  { value: "hp_percent_max", label: "生命百分比最高" },
  { value: "hp_percent_min", label: "生命百分比最低" },
  { value: "attr_max", label: "属性最高" },
  { value: "attr_min", label: "属性最低" },
  { value: "boss_first", label: "BOSS优先" },
  { value: "first", label: "第一个" },
  { value: "random", label: "随机" },
  { value: "self", label: "自身" }
]

const EFFECT_KIND_OPTIONS = [
  { value: "damage", label: "伤害" },
  { value: "heal", label: "治疗" },
  { value: "status", label: "状态" }
]

const DAMAGE_TYPE_OPTIONS = [
  { value: "physical", label: "物理" },
  { value: "magical", label: "魔法" }
]

const ATTR_OPTIONS = [
  { value: "battle", label: "战斗 (battle)" },
  { value: "strength", label: "力量 (strength)" },
  { value: "dexterity", label: "敏捷 (dexterity)" },
  { value: "intelligence", label: "智力 (intelligence)" },
  { value: "stamina", label: "体力 (stamina)" },
  { value: "luck", label: "幸运 (luck)" },
  { value: "hp", label: "生命值 (hp)" },
  { value: "hp_regen", label: "生命恢复 (hp_regen)" },
  { value: "mp", label: "魔法值 (mp)" },
  { value: "mp_regen", label: "魔法恢复 (mp_regen)" },
  { value: "speed", label: "速度 (speed)" },
  { value: "physics_damage_amplification", label: "物理伤害增幅 (physics_damage_amplification)" },
  { value: "physics_damage_resist", label: "物理伤害减免 (physics_damage_resist)" },
  { value: "physics_flat_defense_penetration", label: "物理伤害穿透（固定） (physics_flat_defense_penetration)" },
  { value: "physics_percent_defense_penetration", label: "物理伤害穿透（百分比） (physics_percent_defense_penetration)" },
  { value: "magical_damage_amplification", label: "魔法伤害增幅 (magical_damage_amplification)" },
  { value: "magical_damage_resist", label: "魔法伤害减免 (magical_damage_resist)" },
  { value: "magicial_flat_defense_penetration", label: "魔法伤害穿透（固定） (magicial_flat_defense_penetration)" },
  { value: "magicial_percent_defense_penetration", label: "魔法伤害穿透（百分比） (magicial_percent_defense_penetration)" },
  { value: "physics_damage", label: "物理攻击 (physics_damage)" },
  { value: "physics_defence", label: "物理防御 (physics_defence)" },
  { value: "magical_damage", label: "魔法攻击 (magical_damage)" },
  { value: "magical_defence", label: "魔法防御 (magical_defence)" },
  { value: "crit_chance", label: "暴击率 (crit_chance)" },
  { value: "crit_damage", label: "暴击伤害 (crit_damage)" },
  { value: "mining", label: "采矿 (mining)" },
  { value: "collecting", label: "采集 (collecting)" },
  { value: "farming_animal", label: "养殖 (farming_animal)" },
  { value: "cooking", label: "烹饪 (cooking)" },
  { value: "crafting", label: "制作 (crafting)" },
  { value: "sewing", label: "缝纫 (sewing)" },
  { value: "forging", label: "锻造 (forging)" },
  { value: "enhance", label: "强化 (enhance)" }
]

const DEFAULT_STATUS_OPTIONS = [
  { value: "blessing", label: "祝福 (blessing)" },
  { value: "bleed", label: "流血 (bleed)" }
]

const DEFAULT_WEAPON_OPTIONS = [
  { value: "sword", label: "剑 (sword)" },
  { value: "spear", label: "长枪 (spear)" },
  { value: "bow", label: "弓 (bow)" },
  { value: "crossbow", label: "弩 (crossbow)" },
  { value: "staff", label: "法杖 (staff)" },
  { value: "dagger", label: "匕首 (dagger)" },
  { value: "scythe", label: "镰刀 (scythe)" },
  { value: "single_edged_sword", label: "单刃剑 (single_edged_sword)" }
]

const POSITION_LABELS = { vanguard: "前排", midguard: "中排", rearguard: "后排" }
const SCOPE_LABELS = { ally: "友军", enemy: "敌军", all: "全部" }
const SEEK_LABELS = {
  front_to_rear: "前排 -> 后排",
  rear_to_front: "后排 -> 前排",
  random_alive: "随机",
  none: "无"
}
const RULE_LABELS = {
  hp_max: "生命最高",
  hp_min: "生命最低",
  hp_percent_max: "生命百分比最高",
  hp_percent_min: "生命百分比最低",
  attr_max: "属性最高",
  attr_min: "属性最低",
  boss_first: "BOSS优先",
  first: "第一个",
  random: "随机"
}

const $ = (selector) => document.querySelector(selector)
const tableHead = $("#table-head")
const tableBody = $("#table-body")
const templateSelect = $("#template-select")
const statusEl = $("#status")
const copyBtn = $("#copy-btn")
const addRowBtn = $("#add-row-btn")
const importBtn = $("#import-btn")
const importJsonBtn = $("#import-json-btn")
const formatJsonBtn = $("#format-json-btn")
const resetBtn = $("#reset-btn")
const editorModal = $("#editor-modal")
const editorTitle = $("#editor-title")
const editorContent = $("#editor-content")
const editorCloseBtn = $("#editor-close-btn")
const editorSaveBtn = $("#editor-save-btn")

let rows = []

function collectCategoryOptions() {
  const values = new Set(DEFAULT_CATEGORY_OPTIONS)
  for (const row of rows) {
    const category = asString(row?.category).trim()
    if (category) values.add(category)
  }
  return Array.from(values)
}

function collectWeaponOptions() {
  const values = new Map(DEFAULT_WEAPON_OPTIONS.map((item) => [item.value, item.label]))
  for (const row of rows) {
    for (const value of safeParseJson(row?.weaponRequirement, [])) {
      const normalized = asString(value).trim()
      if (!normalized) continue
      if (!values.has(normalized)) values.set(normalized, `${normalized} (${normalized})`)
    }
  }
  return Array.from(values.entries()).map(([value, label]) => ({ value, label }))
}

function collectStatusOptions() {
  const values = new Map(DEFAULT_STATUS_OPTIONS.map((item) => [item.value, item.label]))
  for (const row of rows) {
    const effects = safeParseJson(row?.effects, [])
    if (!Array.isArray(effects)) continue
    for (const effect of effects) {
      const payload = effect && typeof effect === "object" ? effect.statusPayload : null
      const statusIDs = Array.isArray(payload?.statusIds) ? payload.statusIds : []
      for (const statusID of statusIDs) {
        const normalized = asString(statusID).trim()
        if (!normalized) continue
        if (!values.has(normalized)) values.set(normalized, `${normalized} (${normalized})`)
      }
    }
  }
  return Array.from(values.entries()).map(([value, label]) => ({ value, label }))
}

function uid() {
  if (typeof crypto !== "undefined" && typeof crypto.randomUUID === "function") {
    return crypto.randomUUID()
  }
  return `row-${Date.now()}-${Math.random().toString(16).slice(2)}`
}

function clone(value) {
  return JSON.parse(JSON.stringify(value))
}

function buildTemplates() {
  return Object.entries(templates).map(([key, value]) => ({ key, label: value.name }))
}

function createRowFromTemplate(templateKey) {
  const factory = templates[templateKey] || templates.damage
  return { __id: uid(), ...factory.row() }
}

function escapeHtml(value) {
  return String(value ?? "")
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
}

function asString(value) {
  if (value === null || value === undefined) return ""
  return String(value)
}

function asNumber(value, fallback = 0) {
  const parsed = Number(value)
  return Number.isFinite(parsed) ? parsed : fallback
}

function safeParseJson(value, fallback) {
  const text = asString(value).trim()
  if (!text) return clone(fallback)
  try {
    return JSON.parse(text)
  } catch {
    return clone(fallback)
  }
}

function compactJson(value, fallback) {
  const text = asString(value).trim()
  if (!text) return JSON.stringify(fallback)
  try {
    return JSON.stringify(JSON.parse(text))
  } catch {
    return text.replaceAll(/\t/g, " ").replaceAll(/\r?\n/g, " ")
  }
}

function makeDefaultTerm() {
  return { attrKey: "", coefficient: 0, perSkillLevel: 0 }
}

function makeDefaultTargeting() {
  return {
    targetPositions: ["vanguard"],
    secondarySeek: "front_to_rear",
    targetScope: "enemy",
    selectRule: "first",
    selectAttrKey: "",
    fallbackSelfIfEmpty: false,
    targetCount: { base: 1, formula: [] }
  }
}

function makeDefaultEffect(kind = "damage") {
  if (kind === "status") {
    return {
      kind: "status",
      targeting: makeDefaultTargeting(),
      ignoreUntargetable: false,
      onlyWhenNoTargetFromEffects: [],
      reuseTargetsFromEffect: 0,
      statusPayload: {
        statusIds: ["blessing"],
        durationBaseTurns: 0,
        durationPerSkillLevelTurns: 0
      }
    }
  }
  return {
    kind,
    targeting: makeDefaultTargeting(),
    ignoreUntargetable: false,
    onlyWhenNoTargetFromEffects: [],
    reuseTargetsFromEffect: 0,
    damageType: kind === "heal" ? "magical" : "physical",
    amountFormula: { baseAdd: 0, terms: [makeDefaultTerm()] }
  }
}

function normalizeEffectReferenceIndexes(input) {
  const refs = []
  const appendRef = (value) => {
    const parsed = Math.trunc(asNumber(value, Number.NaN))
    if (!Number.isFinite(parsed) || parsed <= 0) return
    if (refs.includes(parsed)) return
    refs.push(parsed)
  }
  if (Array.isArray(input)) {
    input.forEach((value) => appendRef(value))
  } else if (typeof input === "string") {
    input.split(",").forEach((value) => appendRef(value.trim()))
  } else if (input !== undefined && input !== null) {
    appendRef(input)
  }
  return refs
}

function formatEffectReferenceIndexes(input) {
  return normalizeEffectReferenceIndexes(input).join(",")
}

function normalizeTerm(term) {
  return {
    attrKey: asString(term?.attrKey ?? "").trim(),
    coefficient: asNumber(term?.coefficient ?? 0, 0),
    perSkillLevel: asNumber(term?.perSkillLevel ?? 0, 0)
  }
}

function normalizeTargeting(input) {
  const raw = input && typeof input === "object" ? input : {}
  const targetCount = raw.targetCount && typeof raw.targetCount === "object" ? raw.targetCount : {}
  const positions = Array.isArray(raw.targetPositions)
    ? raw.targetPositions.filter((item) => POSITION_LABELS[item])
    : []
  const safePositions = positions.length ? positions : ["vanguard"]
  return {
    targetPositions: safePositions,
    secondarySeek: SECONDARY_SEEK_OPTIONS.some((item) => item.value === raw.secondarySeek) ? raw.secondarySeek : "front_to_rear",
    targetScope: TARGET_SCOPE_OPTIONS.some((item) => item.value === raw.targetScope) ? raw.targetScope : "enemy",
    selectRule: SELECT_RULE_OPTIONS.some((item) => item.value === raw.selectRule) ? raw.selectRule : "first",
    selectAttrKey: asString(raw.selectAttrKey ?? "").trim(),
    fallbackSelfIfEmpty: Boolean(raw.fallbackSelfIfEmpty),
    targetCount: {
      base: asNumber(targetCount.base ?? 1, 1),
      formula: Array.isArray(targetCount.formula) ? targetCount.formula.map(normalizeTerm).filter((term) => term.attrKey || term.coefficient || term.perSkillLevel) : []
    }
  }
}

function normalizeEffect(effect) {
  const raw = effect && typeof effect === "object" ? effect : {}
  const kind = EFFECT_KIND_OPTIONS.some((item) => item.value === raw.kind) ? raw.kind : "damage"
  const targeting = normalizeTargeting(raw.targeting)
  const onlyWhenNoTargetFromEffects = normalizeEffectReferenceIndexes(raw.onlyWhenNoTargetFromEffects)
  const reuseTargetsFromEffect = Math.max(0, Math.trunc(asNumber(raw.reuseTargetsFromEffect, 0)))
  if (kind === "status") {
    const payload = raw.statusPayload && typeof raw.statusPayload === "object" ? raw.statusPayload : {}
    const statusIds = Array.isArray(payload.statusIds)
      ? payload.statusIds.map((item) => asString(item).trim()).filter(Boolean)
      : []
    return {
      kind: "status",
      targeting,
      ignoreUntargetable: Boolean(raw.ignoreUntargetable),
      onlyWhenNoTargetFromEffects,
      reuseTargetsFromEffect,
      statusPayload: {
        statusIds: statusIds.length ? statusIds : ["blessing"],
        durationBaseTurns: asNumber(payload.durationBaseTurns ?? 0, 0),
        durationPerSkillLevelTurns: asNumber(payload.durationPerSkillLevelTurns ?? 0, 0)
      }
    }
  }
  const amountFormula = raw.amountFormula && typeof raw.amountFormula === "object" ? raw.amountFormula : {}
  return {
    kind,
    targeting,
    ignoreUntargetable: Boolean(raw.ignoreUntargetable),
    onlyWhenNoTargetFromEffects,
    reuseTargetsFromEffect,
    damageType: raw.damageType === "magical" ? "magical" : "physical",
    amountFormula: {
      baseAdd: asNumber(amountFormula.baseAdd ?? 0, 0),
      terms: Array.isArray(amountFormula.terms)
        ? amountFormula.terms.map(normalizeTerm).filter((term) => term.attrKey || term.coefficient || term.perSkillLevel)
        : [makeDefaultTerm()]
    }
  }
}

function normalizeEffects(input) {
  const list = Array.isArray(input) ? input.map(normalizeEffect) : []
  return list.length ? list : [makeDefaultEffect("damage")]
}

function pickPrimaryEffect(effects) {
  if (!Array.isArray(effects) || effects.length === 0) return null
  return effects.find((effect) => effect && (effect.kind === "damage" || effect.kind === "heal")) || effects[0] || null
}

function firstTargetPosition(targetPositions) {
  if (!Array.isArray(targetPositions) || targetPositions.length === 0) return "vanguard"
  const value = asString(targetPositions[0]).trim()
  return value || "vanguard"
}

function deriveTargetModeFromEffect(effect, fallbackMode = "single") {
  const base = Math.max(1, Math.trunc(asNumber(effect?.targeting?.targetCount?.base, 1)))
  if (base > 1) return "multi"
  const normalized = asString(fallbackMode).trim()
  return ["single", "multi", "all"].includes(normalized) ? normalized : "single"
}

function deriveLegacySkillFields(effects, fallback = {}) {
  const primaryEffect = pickPrimaryEffect(effects)
  const fallbackTargeting = normalizeTargeting(parseMaybeJsonValue(fallback.targeting, makeDefaultTargeting()))
  const targeting = primaryEffect?.targeting ? normalizeTargeting(primaryEffect.targeting) : fallbackTargeting
  const targetFormulaFallback = parseMaybeJsonValue(fallback.targetFormula, [])
  const damageFormulaFallback = parseMaybeJsonValue(fallback.damageFormula, [])
  const amountFormula = primaryEffect?.amountFormula && typeof primaryEffect.amountFormula === "object" ? primaryEffect.amountFormula : null
  return {
    damageType: primaryEffect?.damageType === "magical" ? "magical" : asString(fallback.damageType || "physical") === "magical" ? "magical" : "physical",
    targetDirection: firstTargetPosition(targeting.targetPositions),
    targetMode: deriveTargetModeFromEffect(primaryEffect, fallback.targetMode),
    targetBaseCount: Math.max(1, Math.trunc(asNumber(targeting.targetCount?.base, asNumber(fallback.targetBaseCount, 1)))),
    targetSeekStrategy: asString(targeting.secondarySeek || fallback.targetSeekStrategy || "front_to_rear"),
    damageFormula: Array.isArray(amountFormula?.terms) ? amountFormula.terms : Array.isArray(damageFormulaFallback) ? damageFormulaFallback : [],
    targetFormula: Array.isArray(targeting.targetCount?.formula) ? targeting.targetCount.formula : Array.isArray(targetFormulaFallback) ? targetFormulaFallback : [],
    targeting
  }
}

function normalizeStringList(input, fallback = []) {
  const list = Array.isArray(input)
    ? input.map((item) => asString(item).trim()).filter(Boolean)
    : typeof input === "string"
      ? input.split(",").map((item) => item.trim()).filter(Boolean)
      : fallback
  return Array.from(new Set(list))
}

function parseMaybeJsonValue(value, fallback) {
  if (value === undefined || value === null || value === "") return fallback
  if (typeof value === "string") {
    try {
      return JSON.parse(value)
    } catch {
      return fallback
    }
  }
  return value
}

function normalizeStatusRequirementMap(input) {
  if (!input || typeof input !== "object" || Array.isArray(input)) return {}
  const out = {}
  for (const [attrKey, required] of Object.entries(input)) {
    const key = asString(attrKey).trim()
    const value = asNumber(required, Number.NaN)
    if (!key || !Number.isFinite(value) || value < 0) continue
    out[key] = value
  }
  return out
}

function normalizeSkillRowFromJson(raw, fallbackRow = null) {
  const source = raw && typeof raw === "object" && !Array.isArray(raw) ? raw : {}
  const fallback = fallbackRow && typeof fallbackRow === "object" ? fallbackRow : {}
  const effects = normalizeEffects(parseMaybeJsonValue(source.effects, parseMaybeJsonValue(fallback.effects, [makeDefaultEffect("damage")])))
  const legacyFields = deriveLegacySkillFields(effects, {
    ...fallback,
    damageType: source.damageType ?? fallback.damageType,
    targetDirection: source.targetDirection ?? fallback.targetDirection,
    targetMode: source.targetMode ?? fallback.targetMode,
    targetBaseCount: source.targetBaseCount ?? fallback.targetBaseCount,
    targetSeekStrategy: source.targetSeekStrategy ?? fallback.targetSeekStrategy,
    damageFormula: source.damageFormula ?? fallback.damageFormula,
    targetFormula: source.targetFormula ?? fallback.targetFormula,
    targeting: source.targeting ?? fallback.targeting
  })
  const damageFormulaRaw = parseMaybeJsonValue(source.damageFormula, parseMaybeJsonValue(fallback.damageFormula, []))
  const targetFormulaRaw = parseMaybeJsonValue(source.targetFormula, parseMaybeJsonValue(fallback.targetFormula, []))
  const damageFormula = Array.isArray(damageFormulaRaw)
    ? damageFormulaRaw.map(normalizeTerm).filter((term) => term.attrKey || term.coefficient || term.perSkillLevel)
    : []
  const targetFormula = Array.isArray(targetFormulaRaw)
    ? targetFormulaRaw.map(normalizeTerm).filter((term) => term.attrKey || term.coefficient || term.perSkillLevel)
    : []
  return {
    __id: fallback.__id || uid(),
    id: asString(source.id ?? fallback.id).trim(),
    name: asString(source.name ?? fallback.name).trim(),
    category: asString(source.category ?? fallback.category).trim(),
    hpCost: Math.max(0, asNumber(source.hpCost ?? fallback.hpCost, 0)),
    mpCost: Math.max(0, asNumber(source.mpCost ?? fallback.mpCost, 0)),
    cooldown: Math.max(0, asNumber(source.cooldown ?? fallback.cooldown, 0)),
    damageType: legacyFields.damageType,
    targetDirection: POSITION_OPTIONS.some((option) => option.value === legacyFields.targetDirection) ? legacyFields.targetDirection : "vanguard",
    targetMode: ["single", "multi", "all"].includes(legacyFields.targetMode) ? legacyFields.targetMode : "single",
    targetBaseCount: Math.max(1, Math.trunc(asNumber(legacyFields.targetBaseCount, 1))),
    targetSeekStrategy: SECONDARY_SEEK_OPTIONS.some((option) => option.value === legacyFields.targetSeekStrategy) ? legacyFields.targetSeekStrategy : "front_to_rear",
    statusRequirement: JSON.stringify(
      normalizeStatusRequirementMap(source.statusRequirement ?? safeParseJson(fallback.statusRequirement, {})),
      null,
      2
    ),
    weaponRequirement: JSON.stringify(
      normalizeStringList(source.weaponRequirement ?? safeParseJson(fallback.weaponRequirement, [])),
      null,
      2
    ),
    damageFormula: JSON.stringify(damageFormula.length ? damageFormula : legacyFields.damageFormula, null, 2),
    targetFormula: JSON.stringify(targetFormula.length ? targetFormula : legacyFields.targetFormula, null, 2),
    targeting: JSON.stringify(legacyFields.targeting, null, 2),
    effects: JSON.stringify(effects, null, 2)
  }
}

function skillRowToJsonObject(row) {
  return {
    id: asString(row.id).trim(),
    name: asString(row.name).trim(),
    category: asString(row.category).trim(),
    hpCost: Math.max(0, asNumber(row.hpCost, 0)),
    mpCost: Math.max(0, asNumber(row.mpCost, 0)),
    cooldown: Math.max(0, asNumber(row.cooldown, 0)),
    statusRequirement: normalizeStatusRequirementMap(safeParseJson(row.statusRequirement, {})),
    weaponRequirement: normalizeStringList(safeParseJson(row.weaponRequirement, [])),
    effects: normalizeEffects(safeParseJson(row.effects, [makeDefaultEffect("damage")]))
  }
}

function skillDraftToJsonObject(draft) {
  return {
    id: asString(draft.id).trim(),
    name: asString(draft.name).trim(),
    category: asString(draft.category).trim(),
    hpCost: Math.max(0, asNumber(draft.hpCost, 0)),
    mpCost: Math.max(0, asNumber(draft.mpCost, 0)),
    cooldown: Math.max(0, asNumber(draft.cooldown, 0)),
    statusRequirement: normalizeStatusRequirementMap(draft.statusRequirement),
    weaponRequirement: normalizeStringList(draft.weaponRequirement),
    effects: normalizeEffects(draft.effects)
  }
}

function skillJsonTextFromRow(row) {
  return JSON.stringify(skillRowToJsonObject(row), null, 2)
}

function skillJsonTextFromDraft(draft) {
  return JSON.stringify(skillDraftToJsonObject(draft), null, 2)
}

function renderJsonEditorSection(row) {
  return `
    <section class="editor-section json-editor">
      <div class="section-head">
        <h4>JSON 导入 / 导出</h4>
        <div class="json-editor-actions">
          <button type="button" data-action="load-json">导入到表单</button>
          <button type="button" class="primary" data-action="copy-json">复制 JSON</button>
        </div>
      </div>
      <textarea
        class="skill-json-textarea"
        data-skill-json
        spellcheck="false"
        placeholder="把单个技能对象 JSON 粘贴到这里，再点击“导入到表单”"
      >${escapeHtml(skillJsonTextFromRow(row))}</textarea>
      <div class="muted">支持单个技能对象；导入后会同步到下方表单，导出 JSON 仅保留 effects 驱动字段。</div>
    </section>
  `
}

function toOptionHtml(options, selected) {
  return options
    .map((option) => `<option value="${option.value}" ${option.value === selected ? "selected" : ""}>${escapeHtml(option.label)}</option>`)
    .join("")
}

function renderPillCheckboxes(name, valueList) {
  const selected = new Set(valueList)
  return POSITION_OPTIONS.map(
    (option) => `
      <label class="pill">
        <input type="checkbox" name="${name}" value="${option.value}" ${selected.has(option.value) ? "checked" : ""} />
        <span>${option.label}</span>
      </label>
    `
  ).join("")
}

function renderCheckboxPills(name, options, valueList) {
  const selected = new Set(valueList)
  return options
    .map(
      (option) => `
        <label class="pill">
          <input type="checkbox" name="${name}" value="${option.value}" ${selected.has(option.value) ? "checked" : ""} />
          <span>${escapeHtml(option.label)}</span>
        </label>
      `
    )
    .join("")
}

function renderOptionList(options, selected) {
  return options
    .map((option) => `<option value="${option.value}" ${option.value === selected ? "selected" : ""}>${escapeHtml(option.label)}</option>`)
    .join("")
}

function renderAttrOptionList(selected) {
  const normalized = asString(selected).trim()
  if (normalized && !ATTR_OPTIONS.some((option) => option.value === normalized)) {
    return renderOptionList([{ value: normalized, label: `${normalized} (custom)` }, ...ATTR_OPTIONS], normalized)
  }
  return renderOptionList(ATTR_OPTIONS, normalized)
}

function renderFormulaRow(term, scope) {
  return `
    <div class="repeat-row" data-term-row="${scope}">
      <label class="field-label">
        <span>属性键</span>
        <select data-term-field="attrKey">
          <option value="">未选择</option>
          ${renderAttrOptionList(term?.attrKey || "")}
        </select>
      </label>
      <label class="field-label">
        <span>系数</span>
        <input type="number" step="0.01" data-term-field="coefficient" value="${escapeHtml(term?.coefficient ?? 0)}" />
      </label>
      <label class="field-label">
        <span>每级增量</span>
        <input type="number" step="0.01" data-term-field="perSkillLevel" value="${escapeHtml(term?.perSkillLevel ?? 0)}" />
      </label>
      <button type="button" data-action="remove-term">删</button>
    </div>
  `
}

function renderStatusRequirementRow(item) {
  const selectedAttrKey = asString(item?.attrKey || ATTR_OPTIONS[0].value).trim() || ATTR_OPTIONS[0].value
  return `
    <div class="repeat-row" data-status-requirement-row>
      <label class="field-label">
        <span>属性键</span>
        <select data-status-requirement-field="attrKey">
          ${renderAttrOptionList(selectedAttrKey)}
        </select>
      </label>
      <label class="field-label">
        <span>门槛值</span>
        <input type="number" step="1" data-status-requirement-field="required" value="${escapeHtml(item?.required ?? 0)}" />
      </label>
      <button type="button" data-action="remove-status-requirement-row">删</button>
    </div>
  `
}

function normalizeRequirementRows(list) {
  return list
    .map((item) => ({
      attrKey: asString(item?.attrKey ?? "").trim() || ATTR_OPTIONS[0].value,
      required: Math.max(0, asNumber(item?.required ?? 0, 0))
    }))
    .filter((item) => item.attrKey && item.required > 0)
}

function renderTermRow(term, scope) {
  return renderFormulaRow(term, scope)
}

function renderStatusIdRow(statusId) {
  return `
    <div class="repeat-row" data-status-row>
      <label class="field-label">
        <span>状态 ID</span>
        <input type="text" data-status-field="statusId" list="status-id-options" value="${escapeHtml(statusId ?? "")}" />
      </label>
      <button type="button" data-action="remove-status-id">删</button>
    </div>
  `
}

function renderEffectTargetingEditor(targeting) {
  return `
    <div class="editor-section">
      <h4>效果目标规则</h4>
      <div class="pill-grid">
        ${POSITION_OPTIONS.map((option) => `
          <label class="pill">
            <input
              type="checkbox"
              data-effect-target-position
              value="${option.value}"
              ${targeting.targetPositions.includes(option.value) ? "checked" : ""}
            />
            <span>${option.label}</span>
          </label>
        `).join("")}
      </div>
      <div class="form-grid compact mt-2">
        <label>次要寻敌
          <select data-effect-targeting-field="secondarySeek">
            ${toOptionHtml(SECONDARY_SEEK_OPTIONS, targeting.secondarySeek)}
          </select>
        </label>
        <label>目标范围
          <select data-effect-targeting-field="targetScope">
            ${toOptionHtml(TARGET_SCOPE_OPTIONS, targeting.targetScope)}
          </select>
        </label>
        <label>选择逻辑
          <select data-effect-targeting-field="selectRule">
            ${toOptionHtml(SELECT_RULE_OPTIONS, targeting.selectRule)}
          </select>
        </label>
        <label>属性键
          <select data-effect-targeting-field="selectAttrKey">
            <option value="">不使用</option>
            ${renderAttrOptionList(targeting.selectAttrKey)}
          </select>
        </label>
        <label>基础目标数量
          <input type="number" step="1" data-effect-targeting-field="targetCountBase" value="${escapeHtml(targeting.targetCount.base)}" min="1" />
        </label>
        <label class="checkbox-inline">
          <input type="checkbox" data-effect-targeting-field="fallbackSelfIfEmpty" ${targeting.fallbackSelfIfEmpty ? "checked" : ""} />
          <span>无目标时回退自己</span>
        </label>
      </div>
      <div class="repeat-block">
        <div class="repeat-header">
          <strong>目标数量系数</strong>
          <button type="button" data-action="add-term" data-term-scope="effectTargetCount">新增系数</button>
        </div>
        <div class="repeat-list" data-term-list="effectTargetCount">
          ${(targeting.targetCount.formula.length ? targeting.targetCount.formula : [makeDefaultTerm()]).map((term) => renderTermRow(term, "effectTargetCount")).join("")}
        </div>
      </div>
    </div>
  `
}

function renderEffectCard(effect, index) {
  const normalized = normalizeEffect(effect)
  const effectTargeting =
    normalized.targeting && typeof normalized.targeting === "object"
      ? normalized.targeting
      : makeDefaultTargeting()
  const amountFormula =
    normalized.kind === "status"
      ? { baseAdd: 0, terms: [makeDefaultTerm()] }
      : normalized.amountFormula && typeof normalized.amountFormula === "object"
        ? normalized.amountFormula
        : { baseAdd: 0, terms: [makeDefaultTerm()] }
  const statusPayload =
    normalized.kind === "status" && normalized.statusPayload && typeof normalized.statusPayload === "object"
      ? normalized.statusPayload
      : { statusIds: ["blessing"], durationBaseTurns: 0, durationPerSkillLevelTurns: 0 }
  const terms = amountFormula.terms.length ? amountFormula.terms : [makeDefaultTerm()]
  const statusIds = statusPayload.statusIds
  const kindLabel = EFFECT_KIND_OPTIONS.find((item) => item.value === normalized.kind)?.label || normalized.kind
  return `
    <article class="effect-card" data-effect-card data-effect-kind="${normalized.kind}">
      <header class="effect-head">
        <div class="effect-title">
          <strong>效果 #${index + 1}</strong>
          <span class="muted">${kindLabel}</span>
        </div>
        <div class="effect-actions">
          <button type="button" data-action="move-effect-up">↑</button>
          <button type="button" data-action="move-effect-down">↓</button>
          <button type="button" class="danger" data-action="remove-effect">删</button>
        </div>
      </header>

      <div class="form-grid compact">
        <label>类型
          <select name="effectKind">
            ${toOptionHtml(EFFECT_KIND_OPTIONS, normalized.kind)}
          </select>
        </label>
        <label>仅当前序效果无目标（逗号分隔索引）
          <input type="text" name="onlyWhenNoTargetFromEffects" value="${escapeHtml(formatEffectReferenceIndexes(normalized.onlyWhenNoTargetFromEffects))}" placeholder="例如 1,2" />
        </label>
        <label>复用目标来源效果索引
          <input type="number" step="1" min="0" name="reuseTargetsFromEffect" value="${escapeHtml(Math.max(0, Math.trunc(asNumber(normalized.reuseTargetsFromEffect, 0))))}" />
        </label>
        <label class="checkbox-inline">
          <input type="checkbox" name="ignoreUntargetable" ${normalized.ignoreUntargetable ? "checked" : ""} />
          <span>忽略潜行类“不可选中”</span>
        </label>
      </div>

      ${renderEffectTargetingEditor(effectTargeting)}

      <div class="effect-group effect-group-damage">
        <div class="form-grid compact">
          <label>伤害类型
            <select name="damageType">
              ${toOptionHtml(DAMAGE_TYPE_OPTIONS, normalized.damageType)}
            </select>
          </label>
          <label>基础加数
            <input type="number" step="0.01" name="baseAdd" value="${escapeHtml(amountFormula.baseAdd)}" />
          </label>
        </div>

        <div class="repeat-block">
          <div class="repeat-header">
            <strong>公式项</strong>
            <button type="button" data-action="add-term" data-term-scope="effect">新增系数</button>
          </div>
          <div class="repeat-list" data-term-list="effect">
            ${terms.map((term) => renderTermRow(term, "effect")).join("")}
          </div>
        </div>
      </div>

      <div class="effect-group effect-group-status">
        <div class="repeat-block">
          <div class="repeat-header">
            <strong>状态 id</strong>
            <button type="button" data-action="add-status-id">新增状态</button>
          </div>
          <div class="repeat-list" data-status-list>
            ${(statusIds.length ? statusIds : ["blessing"]).map((statusId) => renderStatusIdRow(statusId)).join("")}
          </div>
        </div>

        <div class="form-grid compact">
          <label>持续回合基础值
            <input type="number" step="1" name="durationBaseTurns" value="${escapeHtml(statusPayload.durationBaseTurns)}" />
          </label>
          <label>持续回合系数
            <input type="number" step="1" name="durationPerSkillLevelTurns" value="${escapeHtml(statusPayload.durationPerSkillLevelTurns)}" />
          </label>
        </div>
      </div>
    </article>
  `
}

function renderEffectsEditor(effects) {
  const list = effects.length ? effects : [makeDefaultEffect("damage")]
  return `
    <div class="editor-section">
      <div class="repeat-header">
        <strong>效果列表</strong>
        <div class="inline-actions">
          <button type="button" data-action="add-effect" data-effect-kind="damage">新增伤害</button>
          <button type="button" data-action="add-effect" data-effect-kind="heal">新增治疗</button>
          <button type="button" data-action="add-effect" data-effect-kind="status">新增状态</button>
        </div>
      </div>
      <div class="effect-list">
        ${list.map((effect, index) => renderEffectCard(effect, index)).join("")}
      </div>
    </div>
  `
}

function renderRequirementEditor(requirements) {
  const rows = Object.entries(requirements || {}).map(([attrKey, required]) => ({ attrKey, required }))
  return `
    <div class="repeat-block">
      <div class="repeat-header">
        <strong>属性门槛</strong>
        <button type="button" data-action="add-status-requirement-row">新增门槛</button>
      </div>
      <div class="repeat-list" data-status-requirement-list>
        ${(rows.length ? rows : [{ attrKey: ATTR_OPTIONS[0].value, required: 1 }]).map((item) => renderStatusRequirementRow(item)).join("")}
      </div>
    </div>
  `
}

function renderWeaponRequirementEditor(selectedWeapons) {
  const weaponOptions = collectWeaponOptions()
  return `
    <div class="repeat-block">
      <div class="repeat-header">
        <strong>武器门槛</strong>
      </div>
      <div class="pill-grid">
        ${renderCheckboxPills("weaponRequirement", weaponOptions, selectedWeapons)}
      </div>
    </div>
  `
}

function renderOptionalSection({ title, toggleName, enabled, content }) {
  return `
    <section class="editor-section optional-section" data-optional-section>
      <div class="section-head">
        <h4>${escapeHtml(title)}</h4>
        <label class="checkbox-inline optional-toggle">
          <input type="checkbox" data-optional-toggle name="${escapeHtml(toggleName)}" ${enabled ? "checked" : ""} />
          <span>启用</span>
        </label>
      </div>
      <div class="optional-body">
        ${content}
      </div>
    </section>
  `
}

function renderBasicEditor(row) {
  return `
    <div class="editor-grid">
      <section class="editor-section">
        <h4>基本信息</h4>
        <div class="form-grid">
          <label>技能 ID
            <input type="text" name="id" value="${escapeHtml(row.id)}" />
          </label>
          <label>技能名称
            <input type="text" name="name" value="${escapeHtml(row.name)}" />
          </label>
          <label>分类
            <input type="text" name="category" list="category-options" value="${escapeHtml(row.category)}" placeholder="例如 blade / arcane" />
          </label>
          <label>HP 消耗
            <input type="number" step="1" name="hpCost" value="${escapeHtml(row.hpCost)}" min="0" />
          </label>
          <label>MP 消耗
            <input type="number" step="1" name="mpCost" value="${escapeHtml(row.mpCost)}" min="0" />
          </label>
          <label>冷却
            <input type="number" step="1" name="cooldown" value="${escapeHtml(row.cooldown)}" min="0" />
          </label>
        </div>
      </section>
    </div>
  `
}

function renderEffectsJsonSection(row) {
  const effectsText = JSON.stringify(normalizeEffects(safeParseJson(row.effects, [makeDefaultEffect("damage")])), null, 2)
  return `
    <section class="editor-section">
      <div class="section-head">
        <h4>effects JSON</h4>
        <div class="json-editor-actions">
          <button type="button" data-action="generate-effects-json">生成 effects JSON</button>
          <button type="button" class="primary" data-action="copy-effects-json">复制 effects JSON</button>
        </div>
      </div>
      <textarea
        class="skill-json-textarea"
        data-effects-json
        spellcheck="false"
        readonly
        placeholder="点击生成后显示 effects 数组 JSON"
      >${escapeHtml(effectsText)}</textarea>
      <div class="muted">生成后可直接复制当前技能的 effects 数组 JSON。</div>
    </section>
  `
}

function renderHead() {
  tableHead.innerHTML =
    columns
      .map((column) => `<th>${escapeHtml(column.label)}</th>`)
      .join("") +
    "<th>actions</th>"
}

function renderCellValue(row, column) {
  const value = row[column.key]
  let preview = asString(value)
  if (column.key === "statusRequirement") {
    const requirement = safeParseJson(value, {})
    preview = Object.entries(requirement)
      .map(([attrKey, required]) => `${attrKey} ≥ ${required}`)
      .join(" / ") || "无"
  } else if (column.key === "weaponRequirement") {
    const weapons = safeParseJson(value, [])
    preview = Array.isArray(weapons) && weapons.length ? weapons.join(" / ") : "无"
  } else if (column.key === "targetFormula") {
    const terms = safeParseJson(value, [])
    preview = Array.isArray(terms) && terms.length ? `${terms.length} 项系数` : "无"
  } else if (column.key === "damageFormula") {
    const terms = safeParseJson(value, [])
    preview = Array.isArray(terms) && terms.length ? `${terms.length} 项系数` : "无"
  } else if (column.key === "targeting") {
    preview = summarizeTargeting(value)
  } else if (column.key === "effects") {
    preview = summarizeEffects(value)
  }
  return `
    <div class="visual-cell">
      <div class="visual-preview">${escapeHtml(preview)}</div>
    </div>
  `
}

function renderBody() {
  tableBody.innerHTML = rows
    .map((row, index) => {
      const cells = columns.map((column) => `<td>${renderCellValue(row, column)}</td>`).join("")
      return `
        <tr data-row-id="${row.__id}">
          ${cells}
          <td class="actions">
            <div class="row-action-group">
              <button type="button" class="primary" data-action="edit-row" data-row-id="${row.__id}">编辑</button>
              <button type="button" data-action="dup" data-row-id="${row.__id}">复制</button>
              <button type="button" data-action="up" data-row-id="${row.__id}">↑</button>
              <button type="button" data-action="down" data-row-id="${row.__id}">↓</button>
              <button type="button" class="danger" data-action="del" data-row-id="${row.__id}">删</button>
              <span class="muted">#${index + 1}</span>
            </div>
          </td>
        </tr>
      `
    })
    .join("")

  for (const textarea of tableBody.querySelectorAll("textarea.json")) {
    validateJsonField(textarea)
  }
}

function render() {
  renderTemplateSelect()
  renderHead()
  renderBody()
}

function renderTemplateSelect() {
  templateSelect.innerHTML = buildTemplates()
    .map((template) => `<option value="${template.key}">${escapeHtml(template.label)}</option>`)
    .join("")
}

function setStatus(message, isError = false) {
  statusEl.textContent = message
  statusEl.style.color = isError ? "var(--danger)" : ""
}

function findRow(rowId) {
  return rows.find((row) => row.__id === rowId)
}

function getEditorJsonTextarea() {
  const field = editorContent.querySelector("[data-skill-json]")
  return field instanceof HTMLTextAreaElement ? field : null
}

function openEditorWithRow(row, titleOverride = "") {
  editorModal.dataset.rowId = row.__id
  editorTitle.textContent = titleOverride || `${row.name || row.id || "未命名技能"} · 技能编辑`
  editorContent.innerHTML = renderSkillEditor(row)
  syncEffectCardKinds()
  syncOptionalSections()
  refreshEditorValidation(row)
  editorModal.classList.remove("hidden")
}

function moveRow(rowId, direction) {
  const index = rows.findIndex((row) => row.__id === rowId)
  if (index < 0) return
  const target = index + direction
  if (target < 0 || target >= rows.length) return
  const copy = rows[index]
  rows.splice(index, 1)
  rows.splice(target, 0, copy)
  renderBody()
}

function deleteRow(rowId) {
  rows = rows.filter((row) => row.__id !== rowId)
  renderBody()
}

function duplicateRow(rowId) {
  const index = rows.findIndex((row) => row.__id === rowId)
  if (index < 0) return
  const source = rows[index]
  const cloneRow = { __id: uid(), ...clone(source) }
  cloneRow.__id = uid()
  rows.splice(index + 1, 0, cloneRow)
  renderBody()
}

function updateRowField(rowId, key, value) {
  const row = findRow(rowId)
  if (!row) return
  row[key] = value
}

function openEditor(rowId) {
  const row = findRow(rowId)
  if (!row) return
  openEditorWithRow(row)
}

function openJsonImportEditor() {
  const row = createRowFromTemplate(templateSelect.value || "damage")
  row.__id = `json-import-${uid()}`
  openEditorWithRow(row, "JSON 导入 · 技能编辑")
  const textarea = getEditorJsonTextarea()
  if (textarea) textarea.value = ""
}

function importJsonIntoEditor() {
  const textarea = getEditorJsonTextarea()
  if (!textarea) return
  const raw = textarea.value.trim()
  if (!raw) {
    setStatus("请输入技能 JSON", true)
    return
  }
  let parsed
  try {
    parsed = JSON.parse(raw)
  } catch {
    setStatus("JSON 解析失败：格式不正确", true)
    return
  }
  const currentRowId = editorModal.dataset.rowId || ""
  const currentRow = currentRowId ? findRow(currentRowId) : null
  const normalized = normalizeSkillRowFromJson(parsed, currentRow)
  editorTitle.textContent = `${normalized.name || normalized.id || "未命名技能"} · 技能编辑`
  editorContent.innerHTML = renderSkillEditor(normalized)
  syncEffectCardKinds()
  syncOptionalSections()
  refreshEditorValidation(normalized)
  setStatus("JSON 导入成功")
}

async function copyCurrentSkillJson() {
  const rowId = editorModal.dataset.rowId || ""
  const currentRow = rowId ? findRow(rowId) : null
  const draft = buildEditorDraft(currentRow)
  const errors = validateSkillDraft(draft)
  renderEditorValidation(errors)
  if (errors.length > 0) {
    setStatus(`复制失败：${errors[0]}`, true)
    return
  }
  const jsonText = skillDraftToJsonObject(draft)
  const text = JSON.stringify(jsonText, null, 2)
  const textarea = getEditorJsonTextarea()
  if (textarea) textarea.value = text
  await copyToClipboard(text)
  setStatus("JSON 已复制")
}

function validateJsonField(field) {
  const value = field.value.trim()
  if (!value) {
    field.classList.remove("invalid")
    return true
  }
  try {
    JSON.parse(value)
    field.classList.remove("invalid")
    return true
  } catch {
    field.classList.add("invalid")
    return false
  }
}

function formatJsonRows() {
  for (const row of rows) {
    for (const column of columns.filter((item) => item.type === "json")) {
      row[column.key] = compactJson(row[column.key], column.key === "targeting" ? makeDefaultTargeting() : [])
    }
  }
  renderBody()
  setStatus("JSON 已格式化")
}

async function copyToClipboard(text) {
  if (navigator.clipboard && navigator.clipboard.writeText) {
    await navigator.clipboard.writeText(text)
    return
  }
  const helper = document.createElement("textarea")
  helper.value = text
  helper.style.position = "fixed"
  helper.style.left = "-9999px"
  document.body.appendChild(helper)
  helper.select()
  document.execCommand("copy")
  document.body.removeChild(helper)
}

async function copyRichToClipboard(html, text) {
  if (navigator.clipboard && typeof ClipboardItem !== "undefined" && navigator.clipboard.write) {
    const item = new ClipboardItem({
      "text/html": new Blob([html], { type: "text/html" }),
      "text/plain": new Blob([text], { type: "text/plain" })
    })
    await navigator.clipboard.write([item])
    return
  }
  await copyToClipboard(text)
}

function normalizeRowForExport(row) {
  const out = {}
  for (const column of columns) {
    const raw = row[column.key]
    if (column.type === "number") {
      const num = Number(raw)
      out[column.key] = Number.isFinite(num) ? String(num) : "0"
    } else if (column.type === "json") {
      out[column.key] = compactJson(raw, column.key === "targeting" ? makeDefaultTargeting() : [])
    } else if (column.type === "visual-targeting") {
      out[column.key] = JSON.stringify(normalizeTargeting(safeParseJson(raw, makeDefaultTargeting())))
    } else if (column.type === "visual-effects") {
      out[column.key] = JSON.stringify(normalizeEffects(safeParseJson(raw, [makeDefaultEffect("damage")])))
    } else {
      out[column.key] = asString(raw).replaceAll(/\t/g, " ").replaceAll(/\r?\n/g, " ")
    }
  }
  return out
}

function buildTableExportRows(sourceRows) {
  return sourceRows.map((row) => normalizeRowForExport(row))
}

function buildTableExportMarkup(sourceRows) {
  const exportRows = buildTableExportRows(sourceRows)
  const headerCells = columns.map((column) => `<th>${escapeHtml(column.label)}</th>`).join("")
  const body = exportRows
    .map((row) => {
      const cells = columns.map((column) => `<td>${escapeHtml(row[column.key])}</td>`).join("")
      return `<tr>${cells}</tr>`
    })
    .join("")
  const html = `
    <table>
      <thead><tr>${headerCells}</tr></thead>
      <tbody>${body}</tbody>
    </table>
  `
  const text = [columns.map((column) => column.label).join("\t"), ...exportRows.map((row) => columns.map((column) => row[column.key]).join("\t"))].join("\n")
  return { html, text }
}

function renderTableExportPreview(container, sourceRows) {
  if (!(container instanceof HTMLElement)) return null
  const payload = buildTableExportMarkup(sourceRows)
  container.innerHTML = payload.html
  container.dataset.exportText = payload.text
  container.dataset.exportHtml = payload.html
  return payload
}

async function copyTableRowsToClipboard(sourceRows) {
  const payload = buildTableExportMarkup(sourceRows)
  await copyRichToClipboard(payload.html, payload.text)
}

function parseTsvRows(tsvText) {
  const lines = asString(tsvText)
    .replaceAll(/\r/g, "")
    .split("\n")
    .filter((line) => line.trim() !== "")
  if (lines.length < 2) {
    throw new Error("表格至少需要表头和一行数据")
  }
  const header = lines[0].split("\t").map((item) => item.trim())
  const colIndexMap = new Map()
  for (const column of columns) {
    const idx = header.findIndex((item) => item === column.label)
    if (idx < 0) {
      throw new Error(`表格缺少列: ${column.label}`)
    }
    colIndexMap.set(column.key, idx)
  }
  return lines.slice(1).map((line) => {
    const parts = line.split("\t")
    const row = { __id: uid() }
    for (const column of columns) {
      const index = colIndexMap.get(column.key)
      const raw = index < parts.length ? parts[index] : ""
      if (column.type === "number") {
        row[column.key] = asNumber(raw, 0)
      } else {
        row[column.key] = asString(raw)
      }
    }
    return row
  })
}

function importTsv() {
  const raw = window.prompt("请粘贴 skills sheet 的表格内容（包含表头）")
  if (!raw || !raw.trim()) return
  rows = parseTsvRows(raw)
  renderBody()
  setStatus(`表格已导入 ${rows.length} 行`)
}

function loadTemplate(templateKey) {
  rows = [createRowFromTemplate(templateKey)]
  renderBody()
  setStatus(`已载入 ${templates[templateKey]?.name || templateKey}`)
}

function resetSampleRows() {
  rows = [createRowFromTemplate("damage"), createRowFromTemplate("heal"), createRowFromTemplate("status")]
  renderBody()
  setStatus("示例已重置")
}

function summarizeTargeting(value) {
  const targeting = normalizeTargeting(safeParseJson(value, makeDefaultTargeting()))
  const positions = targeting.targetPositions.map((item) => POSITION_LABELS[item] || item).join("+")
  const scope = SCOPE_LABELS[targeting.targetScope] || targeting.targetScope
  const seek = SEEK_LABELS[targeting.secondarySeek] || targeting.secondarySeek
  const rule = RULE_LABELS[targeting.selectRule] || targeting.selectRule
  const count = targeting.targetCount.base
  return `${positions} · ${scope} · ${rule} · ${seek} · 数量 ${count}`
}

function summarizeEffects(value) {
  const effects = normalizeEffects(safeParseJson(value, [makeDefaultEffect("damage")]))
  return effects
    .map((effect) => {
      if (effect.kind === "status") {
        return `状态 x${effect.statusPayload.statusIds.length}`
      }
      const termCount = effect.amountFormula.terms.length
      return `${effect.kind === "heal" ? "治疗" : "伤害"} ${effect.damageType} · ${termCount} 项`
    })
    .join(" / ")
}

function renderSkillEditor(row) {
  const effects = normalizeEffects(safeParseJson(row.effects, [makeDefaultEffect("damage")]))
  const statusRequirement = safeParseJson(row.statusRequirement, {})
  const weaponRequirement = safeParseJson(row.weaponRequirement, [])
  const categoryOptions = collectCategoryOptions()
  const statusOptions = collectStatusOptions()
  const statusRequirementEnabled = Object.keys(statusRequirement).length > 0
  const weaponRequirementEnabled = Array.isArray(weaponRequirement) && weaponRequirement.length > 0
  return `
    <div class="editor-grid">
      ${renderJsonEditorSection(row)}

      <datalist id="category-options">
        ${categoryOptions.map((value) => `<option value="${escapeHtml(value)}"></option>`).join("")}
      </datalist>
      <datalist id="status-id-options">
        ${statusOptions.map((option) => `<option value="${escapeHtml(option.value)}">${escapeHtml(option.label)}</option>`).join("")}
      </datalist>

      <section class="editor-section">
        <h4>实时校验</h4>
        <ul id="editor-validation-list" class="validation-list"></ul>
      </section>

      ${renderBasicEditor(row)}

      ${renderOptionalSection({
        title: "属性前置条件",
        toggleName: "enableStatusRequirement",
        enabled: statusRequirementEnabled,
        content: renderRequirementEditor(statusRequirement)
      })}

      ${renderOptionalSection({
        title: "武器前置条件",
        toggleName: "enableWeaponRequirement",
        enabled: weaponRequirementEnabled,
        content: renderWeaponRequirementEditor(weaponRequirement)
      })}

      ${renderEffectsEditor(effects)}
      ${renderEffectsJsonSection(row)}
    </div>
  `
}

function getEditorEffectsJsonTextarea() {
  const container = editorContent.querySelector("[data-effects-json]")
  return container instanceof HTMLTextAreaElement ? container : null
}

function buildCurrentEffectsJson(existingRow = null) {
  const draft = buildEditorDraft(existingRow)
  const errors = validateSkillDraft(draft)
  renderEditorValidation(errors)
  if (errors.length > 0) {
    throw new Error(errors[0])
  }
  return JSON.stringify(normalizeEffects(draft.effects), null, 2)
}

function generateCurrentEffectsJson(existingRow = null) {
  const textarea = getEditorEffectsJsonTextarea()
  if (!textarea) return null
  const text = buildCurrentEffectsJson(existingRow)
  textarea.value = text
  return text
}

async function copyCurrentEffectsJson(existingRow = null) {
  const text = generateCurrentEffectsJson(existingRow) || buildCurrentEffectsJson(existingRow)
  await copyToClipboard(text)
  setStatus("effects JSON 已复制")
}

function closeEditor() {
  editorModal.classList.add("hidden")
  editorModal.removeAttribute("data-row-id")
  editorContent.innerHTML = ""
}

function syncEffectCardKinds() {
  for (const card of editorContent.querySelectorAll("[data-effect-card]")) {
    const kind = card.querySelector('select[name="effectKind"]')?.value || "damage"
    card.dataset.effectKind = kind
  }
}

function syncOptionalSections() {
  for (const section of editorContent.querySelectorAll("[data-optional-section]")) {
    const toggle = section.querySelector('[data-optional-toggle]')
    const body = section.querySelector(".optional-body")
    if (!(toggle instanceof HTMLInputElement) || !(body instanceof HTMLElement)) continue
    section.classList.toggle("is-disabled", !toggle.checked)
    body.querySelectorAll("input, select, textarea, button").forEach((node) => {
      if (!(node instanceof HTMLElement)) return
      if (node === toggle) return
      node.toggleAttribute("disabled", !toggle.checked)
    })
  }
}

function addEffectCard(kind = "damage") {
  const list = editorContent.querySelector(".effect-list")
  if (!list) return
  const index = list.querySelectorAll("[data-effect-card]").length
  const html = renderEffectCard(makeDefaultEffect(kind), index)
  list.insertAdjacentHTML("beforeend", html)
  syncEffectCardKinds()
}

function addTermRow(scope, term = makeDefaultTerm(), root = editorContent) {
  const list = root.querySelector(`[data-term-list="${scope}"]`)
  if (!list) return
  list.insertAdjacentHTML("beforeend", renderTermRow(term, scope))
}

function addStatusRow(statusId = "") {
  const list = editorContent.querySelector("[data-status-list]")
  if (!list) return
  list.insertAdjacentHTML("beforeend", renderStatusIdRow(statusId))
}

function moveEffectCard(card, direction) {
  const sibling = direction < 0 ? card.previousElementSibling : card.nextElementSibling
  if (!sibling || !sibling.matches("[data-effect-card]")) return
  if (direction < 0) {
    card.parentElement.insertBefore(card, sibling)
  } else {
    card.parentElement.insertBefore(sibling, card)
  }
  syncEffectCardKinds()
}

function collectTerms(container) {
  return Array.from(container.querySelectorAll("[data-term-row]"))
    .map((row) => {
      const attrKey = row.querySelector('[data-term-field="attrKey"]')?.value.trim() || ""
      const coefficient = asNumber(row.querySelector('[data-term-field="coefficient"]')?.value, 0)
      const perSkillLevel = asNumber(row.querySelector('[data-term-field="perSkillLevel"]')?.value, 0)
      return { attrKey, coefficient, perSkillLevel }
    })
    .filter((term) => term.attrKey || term.coefficient || term.perSkillLevel)
}

function collectStatusIds(container) {
  return Array.from(container.querySelectorAll("[data-status-row]"))
    .map((row) => row.querySelector('[data-status-field="statusId"]')?.value.trim() || "")
    .filter(Boolean)
}

function collectStatusRequirements(container) {
  return normalizeRequirementRows(
    Array.from(container.querySelectorAll("[data-status-requirement-row]")).map((row) => ({
      attrKey: row.querySelector('[data-status-requirement-field="attrKey"]')?.value || ATTR_OPTIONS[0].value,
      required: row.querySelector('[data-status-requirement-field="required"]')?.value
    }))
  ).reduce((acc, item) => {
    acc[item.attrKey] = item.required
    return acc
  }, {})
}

function collectEffectTargeting(card) {
  const positions = Array.from(card.querySelectorAll('[data-effect-target-position]:checked')).map((item) => item.value)
  const secondarySeek = card.querySelector('[data-effect-targeting-field="secondarySeek"]')?.value || "front_to_rear"
  const targetScope = card.querySelector('[data-effect-targeting-field="targetScope"]')?.value || "enemy"
  const selectRule = card.querySelector('[data-effect-targeting-field="selectRule"]')?.value || "first"
  const selectAttrKey = card.querySelector('[data-effect-targeting-field="selectAttrKey"]')?.value.trim() || ""
  const fallbackSelfIfEmpty = Boolean(card.querySelector('[data-effect-targeting-field="fallbackSelfIfEmpty"]')?.checked)
  const targetCountBase = asNumber(card.querySelector('[data-effect-targeting-field="targetCountBase"]')?.value, 1)
  const targetCountFormula = collectTerms(card.querySelector('[data-term-list="effectTargetCount"]') || card)
  return normalizeTargeting({
    targetPositions: positions,
    secondarySeek,
    targetScope,
    selectRule,
    selectAttrKey,
    fallbackSelfIfEmpty,
    targetCount: { base: targetCountBase, formula: targetCountFormula }
  })
}

function collectEffects() {
  const cards = Array.from(editorContent.querySelectorAll("[data-effect-card]"))
  const effects = cards.map((card) => {
    const kind = card.querySelector('select[name="effectKind"]')?.value || "damage"
    const targeting = collectEffectTargeting(card)
    const ignoreUntargetable = Boolean(card.querySelector('input[name="ignoreUntargetable"]')?.checked)
    const onlyWhenNoTargetFromEffects = normalizeEffectReferenceIndexes(card.querySelector('input[name="onlyWhenNoTargetFromEffects"]')?.value || "")
    const reuseTargetsFromEffect = Math.max(0, Math.trunc(asNumber(card.querySelector('input[name="reuseTargetsFromEffect"]')?.value, 0)))
    if (kind === "status") {
      return normalizeEffect({
        kind,
        targeting,
        ignoreUntargetable,
        onlyWhenNoTargetFromEffects,
        reuseTargetsFromEffect,
        statusPayload: {
          statusIds: collectStatusIds(card.querySelector("[data-status-list]") || card),
          durationBaseTurns: asNumber(card.querySelector('input[name="durationBaseTurns"]')?.value, 0),
          durationPerSkillLevelTurns: asNumber(card.querySelector('input[name="durationPerSkillLevelTurns"]')?.value, 0)
        }
      })
    }
    return normalizeEffect({
      kind,
      targeting,
      ignoreUntargetable,
      onlyWhenNoTargetFromEffects,
      reuseTargetsFromEffect,
      damageType: card.querySelector('select[name="damageType"]')?.value || "physical",
      amountFormula: {
        baseAdd: asNumber(card.querySelector('input[name="baseAdd"]')?.value, 0),
        terms: collectTerms(card.querySelector('[data-term-list="effect"]') || card)
      }
    })
  })
  return normalizeEffects(effects)
}

function validateTargeting(targeting, path) {
  const errors = []
  if (!targeting || typeof targeting !== "object") {
    errors.push(`${path} 必须是对象`)
    return errors
  }
  if (!Array.isArray(targeting.targetPositions) || targeting.targetPositions.length === 0) {
    errors.push(`${path}.targetPositions 至少选择一个站位`)
  }
  if (!Number.isFinite(targeting.targetCount?.base) || targeting.targetCount.base < 1) {
    errors.push(`${path}.targetCount.base 必须 >= 1`)
  }
  if (!Number.isInteger(targeting.targetCount?.base)) {
    errors.push(`${path}.targetCount.base 必须是整数`)
  }
  if ((targeting.selectRule === "attr_max" || targeting.selectRule === "attr_min") && !asString(targeting.selectAttrKey).trim()) {
    errors.push(`${path}.selectAttrKey 在 attr_* 规则下必填`)
  }
  return errors
}

function buildEditorDraft(existingRow = null) {
  const idRaw = editorContent.querySelector('input[name="id"]')?.value.trim() || ""
  const nameRaw = editorContent.querySelector('input[name="name"]')?.value.trim() || ""
  const categoryRaw = editorContent.querySelector('input[name="category"]')?.value.trim() || ""
  const hpCostRaw = asNumber(editorContent.querySelector('input[name="hpCost"]')?.value, 0)
  const mpCostRaw = asNumber(editorContent.querySelector('input[name="mpCost"]')?.value, 0)
  const cooldownRaw = asNumber(editorContent.querySelector('input[name="cooldown"]')?.value, 0)

  const statusRequirementEnabled = Boolean(editorContent.querySelector('input[name="enableStatusRequirement"]')?.checked)
  const weaponRequirementEnabled = Boolean(editorContent.querySelector('input[name="enableWeaponRequirement"]')?.checked)
  const statusRequirement = statusRequirementEnabled
    ? collectStatusRequirements(editorContent.querySelector("[data-status-requirement-list]") || editorContent)
    : {}
  const weaponRequirement = weaponRequirementEnabled
    ? Array.from(editorContent.querySelectorAll('input[name="weaponRequirement"]:checked'))
        .map((item) => item.value)
        .filter(Boolean)
    : []
  const effects = collectEffects()

  return {
    id: idRaw || asString(existingRow?.id).trim(),
    name: nameRaw || asString(existingRow?.name).trim(),
    category: categoryRaw,
    hpCost: Math.max(0, hpCostRaw),
    mpCost: Math.max(0, mpCostRaw),
    cooldown: Math.max(0, cooldownRaw),
    statusRequirement,
    weaponRequirement,
    effects
  }
}

function validateSkillDraft(draft) {
  const errors = []
  if (!asString(draft.id).trim()) errors.push("技能 ID 必填")
  if (!Number.isFinite(draft.hpCost) || draft.hpCost < 0) errors.push("hpCost 必须是 >= 0 的数字")
  if (!Number.isFinite(draft.mpCost) || draft.mpCost < 0) errors.push("mpCost 必须是 >= 0 的数字")
  if (!Number.isFinite(draft.cooldown) || draft.cooldown < 0) errors.push("cooldown 必须是 >= 0 的数字")

  for (const [attrKey, required] of Object.entries(draft.statusRequirement || {})) {
    if (!attrKey) errors.push("statusRequirement.attrKey 不能为空")
    if (!Number.isFinite(required) || required < 0) errors.push(`statusRequirement.${attrKey || "unknown"} 必须 >= 0`)
  }

  if (!Array.isArray(draft.effects) || draft.effects.length === 0) {
    errors.push("effects 至少保留一个效果")
    return errors
  }
  draft.effects.forEach((effect, index) => {
    const effectIndex = index + 1
    const label = `effects[${index}]`
    if (!effect || typeof effect !== "object") {
      errors.push(`${label} 必须是对象`)
      return
    }
    if (!["damage", "heal", "status"].includes(effect.kind)) {
      errors.push(`${label}.kind 无效`)
    }
    if (effect.reuseTargetsFromEffect !== undefined) {
      if (!Number.isFinite(effect.reuseTargetsFromEffect) || !Number.isInteger(effect.reuseTargetsFromEffect) || effect.reuseTargetsFromEffect < 0) {
        errors.push(`${label}.reuseTargetsFromEffect 必须是 >= 0 的整数`)
      } else if (effect.reuseTargetsFromEffect > 0 && effect.reuseTargetsFromEffect >= effectIndex) {
        errors.push(`${label}.reuseTargetsFromEffect 必须引用前序效果（1-based）`)
      }
    }
    if (effect.onlyWhenNoTargetFromEffects !== undefined) {
      if (!Array.isArray(effect.onlyWhenNoTargetFromEffects)) {
        errors.push(`${label}.onlyWhenNoTargetFromEffects 必须是数组`)
      } else {
        const dedupe = new Set()
        effect.onlyWhenNoTargetFromEffects.forEach((ref, refIndex) => {
          if (!Number.isFinite(ref) || !Number.isInteger(ref) || ref <= 0) {
            errors.push(`${label}.onlyWhenNoTargetFromEffects[${refIndex}] 必须是正整数`)
            return
          }
          if (ref >= effectIndex) errors.push(`${label}.onlyWhenNoTargetFromEffects[${refIndex}] 必须引用前序效果（1-based）`)
          if (dedupe.has(ref)) errors.push(`${label}.onlyWhenNoTargetFromEffects[${refIndex}] 重复`)
          dedupe.add(ref)
        })
      }
    }
    errors.push(...validateTargeting(effect.targeting, `${label}.targeting`))
    if (effect.kind === "status") {
      if (!Array.isArray(effect.statusPayload?.statusIds) || effect.statusPayload.statusIds.length === 0) {
        errors.push(`${label}.statusPayload.statusIds 至少填一个`)
      }
      return
    }
    if (!effect.amountFormula || typeof effect.amountFormula !== "object") {
      errors.push(`${label}.amountFormula 缺失`)
      return
    }
    if (!Number.isFinite(effect.amountFormula.baseAdd)) {
      errors.push(`${label}.amountFormula.baseAdd 必须是数字`)
    }
    if (!Array.isArray(effect.amountFormula.terms) || effect.amountFormula.terms.length === 0) {
      errors.push(`${label}.amountFormula.terms 至少一项`)
      return
    }
    effect.amountFormula.terms.forEach((term, termIndex) => {
      if (!term || typeof term !== "object") {
        errors.push(`${label}.amountFormula.terms[${termIndex}] 必须是对象`)
        return
      }
      if (!asString(term.attrKey).trim()) {
        errors.push(`${label}.amountFormula.terms[${termIndex}].attrKey 必填`)
      }
      if (!Number.isFinite(term.coefficient)) {
        errors.push(`${label}.amountFormula.terms[${termIndex}].coefficient 必须是数字`)
      }
      if (!Number.isFinite(term.perSkillLevel)) {
        errors.push(`${label}.amountFormula.terms[${termIndex}].perSkillLevel 必须是数字`)
      }
    })
  })
  return errors
}

function renderEditorValidation(errors) {
  const list = editorContent.querySelector("#editor-validation-list")
  if (!list) return
  if (!errors.length) {
    list.innerHTML = '<li class="ok">✓ 当前配置可保存</li>'
    return
  }
  list.innerHTML = errors.map((item) => `<li>${escapeHtml(item)}</li>`).join("")
}

function refreshEditorValidation(existingRow = null) {
  if (!editorContent.innerHTML) return true
  const draft = buildEditorDraft(existingRow)
  const errors = validateSkillDraft(draft)
  renderEditorValidation(errors)
  return errors.length === 0
}

function refreshCurrentEditorValidation() {
  const rowId = editorModal.dataset.rowId || ""
  const row = rowId ? findRow(rowId) : null
  return refreshEditorValidation(row)
}

function draftToRowRecord(draft, rowId) {
  const legacyFields = deriveLegacySkillFields(draft.effects, draft)
  return {
    __id: rowId,
    id: draft.id,
    name: draft.name,
    category: draft.category,
    hpCost: draft.hpCost,
    mpCost: draft.mpCost,
    cooldown: draft.cooldown,
    damageType: legacyFields.damageType,
    targetDirection: legacyFields.targetDirection,
    targetMode: legacyFields.targetMode,
    targetBaseCount: legacyFields.targetBaseCount,
    targetSeekStrategy: legacyFields.targetSeekStrategy,
    statusRequirement: JSON.stringify(draft.statusRequirement, null, 2),
    weaponRequirement: JSON.stringify(draft.weaponRequirement, null, 2),
    damageFormula: JSON.stringify(legacyFields.damageFormula, null, 2),
    targetFormula: JSON.stringify(legacyFields.targetFormula, null, 2),
    targeting: JSON.stringify(legacyFields.targeting, null, 2),
    effects: JSON.stringify(draft.effects, null, 2)
  }
}

function saveEditor() {
  const rowId = editorModal.dataset.rowId
  if (!rowId) return
  const row = findRow(rowId)
  const draft = buildEditorDraft(row)
  const errors = validateSkillDraft(draft)
  renderEditorValidation(errors)
  if (errors.length > 0) {
    setStatus(`保存失败：${errors[0]}`, true)
    return
  }
  const nextRow = draftToRowRecord(draft, rowId)
  if (row) {
    Object.assign(row, nextRow)
  } else {
    rows.push(nextRow)
  }
  renderBody()
  closeEditor()
  setStatus("技能已更新")
}

tableBody.addEventListener("input", (event) => {
  const target = event.target
  if (!(target instanceof HTMLInputElement || target instanceof HTMLTextAreaElement || target instanceof HTMLSelectElement)) {
    return
  }
  const rowId = target.dataset.rowId
  const key = target.dataset.key
  if (!rowId || !key) return
  updateRowField(rowId, key, target.value)
  if (target.classList.contains("json")) {
    validateJsonField(target)
  }
})

tableBody.addEventListener("click", (event) => {
  const target = event.target
  if (!(target instanceof HTMLElement)) return
  const action = target.dataset.action
  const rowId = target.dataset.rowId
  if (!action || !rowId) return
  if (action === "dup") duplicateRow(rowId)
  if (action === "up") moveRow(rowId, -1)
  if (action === "down") moveRow(rowId, 1)
  if (action === "del") deleteRow(rowId)
  if (action === "edit-row") openEditor(rowId)
})

editorModal.addEventListener("click", (event) => {
  const target = event.target
  if (!(target instanceof HTMLElement)) return
  const action = target.dataset.action
  if (!action) return

  if (action === "close-editor") {
    closeEditor()
    return
  }
  if (action === "save-editor") {
    saveEditor()
    return
  }
  if (action === "load-json") {
    importJsonIntoEditor()
    return
  }
  if (action === "copy-json") {
    copyCurrentSkillJson().catch((err) => setStatus(err instanceof Error ? err.message : "复制失败", true))
    return
  }
  if (action === "add-term") {
    const scope = target.dataset.termScope || "effect"
    const root = target.closest("[data-effect-card], .editor-section, .repeat-block") || editorContent
    addTermRow(scope, makeDefaultTerm(), root)
    refreshCurrentEditorValidation()
    return
  }
  if (action === "remove-term") {
    target.closest("[data-term-row]")?.remove()
    refreshCurrentEditorValidation()
    return
  }
  if (action === "add-status-requirement-row") {
    const list = editorContent.querySelector("[data-status-requirement-list]")
    if (list) {
      list.insertAdjacentHTML("beforeend", renderStatusRequirementRow({ attrKey: ATTR_OPTIONS[0].value, required: 1 }))
    }
    refreshCurrentEditorValidation()
    return
  }
  if (action === "remove-status-requirement-row") {
    target.closest("[data-status-requirement-row]")?.remove()
    refreshCurrentEditorValidation()
    return
  }
  if (action === "add-formula-row") {
    const scope = target.dataset.formulaScope || "effect"
    const root = target.closest(".repeat-block") || editorContent
    addTermRow(scope, makeDefaultTerm(), root)
    refreshCurrentEditorValidation()
    return
  }
  if (action === "add-effect") {
    addEffectCard(target.dataset.effectKind || "damage")
    refreshCurrentEditorValidation()
    return
  }
  if (action === "remove-effect") {
    target.closest("[data-effect-card]")?.remove()
    syncEffectCardKinds()
    refreshCurrentEditorValidation()
    return
  }
  if (action === "move-effect-up") {
    const card = target.closest("[data-effect-card]")
    if (card) moveEffectCard(card, -1)
    refreshCurrentEditorValidation()
    return
  }
  if (action === "move-effect-down") {
    const card = target.closest("[data-effect-card]")
    if (card) moveEffectCard(card, 1)
    refreshCurrentEditorValidation()
    return
  }
  if (action === "add-status-id") {
    addStatusRow()
    refreshCurrentEditorValidation()
    return
  }
  if (action === "remove-status-id") {
    target.closest("[data-status-row]")?.remove()
    refreshCurrentEditorValidation()
    return
  }
  if (action === "generate-effects-json") {
    try {
      generateCurrentEffectsJson(findRow(editorModal.dataset.rowId || ""))
      setStatus("effects JSON 已生成")
    } catch (err) {
      setStatus(err instanceof Error ? err.message : "生成失败", true)
    }
    return
  }
  if (action === "copy-effects-json") {
    copyCurrentEffectsJson(findRow(editorModal.dataset.rowId || "")).catch((err) => setStatus(err instanceof Error ? err.message : "复制失败", true))
    return
  }
})

editorModal.addEventListener("change", (event) => {
  const target = event.target
  if (target instanceof HTMLInputElement && target.matches("[data-optional-toggle]")) {
    syncOptionalSections()
    refreshCurrentEditorValidation()
    return
  }
  if (!(target instanceof HTMLSelectElement)) {
    refreshCurrentEditorValidation()
    return
  }
  if (target.name === "effectKind") {
    const card = target.closest("[data-effect-card]")
    if (card) {
      card.dataset.effectKind = target.value
    }
  }
  refreshCurrentEditorValidation()
})

editorModal.addEventListener("input", () => {
  refreshCurrentEditorValidation()
})

copyBtn.addEventListener("click", () => {
  copyTableRowsToClipboard(rows)
    .then(() => setStatus("表格已复制"))
    .catch((err) => setStatus(err instanceof Error ? err.message : "复制失败", true))
})

templateSelect.addEventListener("change", () => {
  const key = templateSelect.value
  if (!key) return
  loadTemplate(key)
})

addRowBtn.addEventListener("click", () => {
  const key = templateSelect.value || "damage"
  rows.push(createRowFromTemplate(key))
  renderBody()
  setStatus(`新增 ${templates[key]?.name || key}`)
})

importBtn.addEventListener("click", () => {
  try {
    importTsv()
  } catch (err) {
    setStatus(err instanceof Error ? err.message : "导入失败", true)
  }
})

importJsonBtn.addEventListener("click", openJsonImportEditor)

formatJsonBtn.addEventListener("click", formatJsonRows)
resetBtn.addEventListener("click", resetSampleRows)
editorCloseBtn.addEventListener("click", closeEditor)
editorSaveBtn.addEventListener("click", saveEditor)

render()
resetSampleRows()
