const columns = [
  { key: "id", label: "id", type: "text" },
  { key: "equipType", label: "装备类型", type: "select", options: ["battle", "work"] },
  { key: "slot", label: "slot", type: "text" },
  { key: "occupySlots", label: "占用槽位", type: "json" },
  { key: "weaponClass", label: "武器类别", type: "text" },
  { key: "statusRequirement", label: "属性门槛", type: "json" },
  { key: "accessoryGroup", label: "饰品互斥组", type: "text" },
  { key: "affixSlotCount", label: "词条槽位数", type: "number" },
  { key: "affixAllowedIds", label: "词条白名单", type: "json" },
  { key: "baseAttrs", label: "基础属性", type: "json" },
  { key: "maxDurability", label: "最大耐久", type: "number" },
  { key: "decayRate", label: "耐久衰减", type: "number" },
  { key: "enhanceMaxLevel", label: "最大强化等级", type: "number" },
  { key: "enhanceDurationS", label: "强化耗时(秒)", type: "number" },
  { key: "enhanceExp", label: "强化经验", type: "number" },
  { key: "enhanceProtectItemId", label: "保护道具", type: "text" },
  { key: "enhanceCostCurve", label: "强化消耗曲线", type: "json" },
  { key: "enhanceAttrCurve", label: "强化属性曲线", type: "json" },
  { key: "battleHooks", label: "战斗钩子", type: "json" },
  { key: "basicAttackEffects", label: "普攻效果", type: "json" }
]

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

const SEEK_OPTIONS = [
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
  { value: "boss_first", label: "首领优先" },
  { value: "attr_max", label: "属性最高" },
  { value: "attr_min", label: "属性最低" },
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

const EFFECT_KIND_OPTIONS_PREDICATE_KINDS = new Set(["compare_attr", "exclude_enemy_type", "require_target_has_status", "target_hp_not_full"])

const BATTLE_HOOK_TRIGGER_OPTIONS = [
  { value: "on_after_take_damage", label: "受到伤害后" }
]

const BATTLE_HOOK_CONDITION_OPTIONS = [
  { value: "incoming_damage_gte", label: "受到伤害 >= 数值" },
  { value: "incoming_damage_lte", label: "受到伤害 <= 数值" },
  { value: "self_hp_percent_lte", label: "自身生命百分比 <= 数值" },
  { value: "self_hp_percent_gte", label: "自身生命百分比 >= 数值" },
  { value: "has_buff", label: "自身拥有指定 buff" },
  { value: "source_has_buff", label: "来源拥有指定 buff" }
]

const BATTLE_HOOK_TARGET_MODE_OPTIONS = [
  { value: "self", label: "自己（self）" },
  { value: "source", label: "伤害来源（source）" },
  { value: "targeting", label: "按目标规则（targeting）" }
]

const DEFAULT_ATTR_OPTIONS = [
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
  "physics_damage_amplification",
  "physics_damage_resist",
  "physics_flat_defense_penetration",
  "physics_percent_defense_penetration",
  "magical_damage_amplification",
  "magical_damage_resist",
  "magicial_flat_defense_penetration",
  "magicial_percent_defense_penetration",
  "mining",
  "collecting",
  "farming_animal",
  "cooking",
  "crafting",
  "sewing",
  "forging",
  "enhance",
  "enhancement_success_rate",
  "collecting_speed",
  "mining_speed",
  "farming_animal_speed",
  "crafting_speed",
  "sewing_speed",
  "forging_speed"
]

const ATTR_LABELS = {
  battle: "战斗等级",
  strength: "力量",
  dexterity: "敏捷",
  intelligence: "智力",
  stamina: "体质",
  luck: "幸运",
  hp: "生命",
  hp_regen: "生命回复",
  mp: "法力",
  mp_regen: "法力回复",
  speed: "速度",
  physics_damage: "物理攻击",
  physics_defence: "物理防御",
  magical_damage: "魔法攻击",
  magical_defence: "魔法防御",
  crit_chance: "暴击率",
  crit_damage: "暴击伤害",
  physics_damage_amplification: "物理伤害加成",
  physics_damage_resist: "物理伤害减免",
  physics_flat_defense_penetration: "物理固定穿透",
  physics_percent_defense_penetration: "物理百分比穿透",
  magical_damage_amplification: "魔法伤害加成",
  magical_damage_resist: "魔法伤害减免",
  magicial_flat_defense_penetration: "魔法固定穿透",
  magicial_percent_defense_penetration: "魔法百分比穿透",
  mining: "采矿",
  collecting: "采集",
  farming_animal: "畜牧",
  cooking: "烹饪",
  crafting: "制作",
  sewing: "缝纫",
  forging: "锻造",
  enhance: "强化",
  enhancement_success_rate: "强化成功率",
  collecting_speed: "采集速度",
  mining_speed: "采矿速度",
  farming_animal_speed: "畜牧速度",
  crafting_speed: "制作速度",
  sewing_speed: "缝纫速度",
  forging_speed: "锻造速度"
}

const ATTR_SELECT_OPTIONS = DEFAULT_ATTR_OPTIONS.map((value) => ({
  value,
  label: ATTR_LABELS[value] ? `${ATTR_LABELS[value]} (${value})` : value
}))
const ATTR_KEY_SET = new Set(DEFAULT_ATTR_OPTIONS)

const DEFAULT_WEAPON_CLASS_OPTIONS = ["sword", "spear", "bow", "crossbow", "staff"]
const DEFAULT_SLOT_OPTIONS = ["weapon", "head", "chest", "legs", "ring", "necklace"]

const $ = (selector) => document.querySelector(selector)
const tableHead = $("#table-head")
const tableBody = $("#table-body")
const templateSelect = $("#template-select")
const statusEl = $("#status")
const copyBtn = $("#copy-btn")
const downloadBtn = $("#download-btn")
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

function uid() {
  if (typeof crypto !== "undefined" && typeof crypto.randomUUID === "function") return crypto.randomUUID()
  return `row-${Date.now()}-${Math.random().toString(16).slice(2)}`
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

function parseListLike(value, fallback = []) {
  if (Array.isArray(value)) {
    return Array.from(new Set(value.map((item) => asString(item).trim()).filter(Boolean)))
  }
  if (typeof value === "string") {
    const text = value.trim()
    if (!text) return clone(fallback)
    if (text.startsWith("[")) {
      try {
        const parsed = JSON.parse(text)
        return parseListLike(parsed, fallback)
      } catch {
        return text.split(",").map((item) => item.trim()).filter(Boolean)
      }
    }
    return text.split(",").map((item) => item.trim()).filter(Boolean)
  }
  return clone(fallback)
}

function parseObjectLike(value, fallback = {}) {
  if (value && typeof value === "object" && !Array.isArray(value)) return value
  if (typeof value === "string") {
    const text = value.trim()
    if (!text) return clone(fallback)
    try {
      const parsed = JSON.parse(text)
      return parsed && typeof parsed === "object" && !Array.isArray(parsed) ? parsed : clone(fallback)
    } catch {
      return clone(fallback)
    }
  }
  return clone(fallback)
}

function parseArrayLike(value, fallback = []) {
  if (Array.isArray(value)) return value
  if (typeof value === "string") {
    const text = value.trim()
    if (!text) return clone(fallback)
    try {
      const parsed = JSON.parse(text)
      return Array.isArray(parsed) ? parsed : clone(fallback)
    } catch {
      return clone(fallback)
    }
  }
  return clone(fallback)
}

function normalizeTerm(term) {
  return {
    attrKey: asString(term?.attrKey ?? "").trim(),
    coefficient: asNumber(term?.coefficient ?? 0, 0),
    perSkillLevel: asNumber(term?.perSkillLevel ?? 0, 0)
  }
}

function makeDefaultTerm() {
  return { attrKey: "strength", coefficient: 1, perSkillLevel: 0 }
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

function makeDefaultDamageSpec() {
  return {
    damageType: "physical",
    amountFormula: { baseAdd: 0, terms: [makeDefaultTerm()] },
    flatDefensePenetrationFormula: { baseAdd: 0, terms: [] },
    percentDefensePenetrationFormula: { baseAdd: 0, terms: [] },
    physicalFlatDefensePenetrationFormula: { baseAdd: 0, terms: [] },
    physicalPercentDefensePenetrationFormula: { baseAdd: 0, terms: [] },
    magicalFlatDefensePenetrationFormula: { baseAdd: 0, terms: [] },
    magicalPercentDefensePenetrationFormula: { baseAdd: 0, terms: [] },
    extraDamageAmplificationFormula: { baseAdd: 0, terms: [] },
    bonusAgainstTargetTypes: {},
    hitCount: { base: 0, formula: [] }
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
        statusIds: ["bleed"],
        durationBaseTurns: 0,
        durationPerSkillLevelTurns: 0
      }
    }
  }
  const defaultDamageSpec = kind === "damage" ? makeDefaultDamageSpec() : undefined
  return {
    kind,
    targeting: makeDefaultTargeting(),
    ignoreUntargetable: false,
    onlyWhenNoTargetFromEffects: [],
    reuseTargetsFromEffect: 0,
    damageType: kind === "heal" ? "magical" : "physical",
    amountFormula: kind === "damage" ? defaultDamageSpec.amountFormula : { baseAdd: 0, terms: [makeDefaultTerm()] },
    damageSpec: defaultDamageSpec
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

function normalizeTargeting(input) {
  const raw = input && typeof input === "object" ? input : {}
  const targetCount = raw.targetCount && typeof raw.targetCount === "object" ? raw.targetCount : {}
  const targetPositions = Array.isArray(raw.targetPositions)
    ? raw.targetPositions.map((item) => asString(item).trim()).filter((item) => POSITION_OPTIONS.some((option) => option.value === item))
    : []
  return {
    targetPositions: targetPositions.length ? targetPositions : ["vanguard"],
    secondarySeek: SEEK_OPTIONS.some((item) => item.value === raw.secondarySeek) ? raw.secondarySeek : "front_to_rear",
    targetScope: TARGET_SCOPE_OPTIONS.some((item) => item.value === raw.targetScope) ? raw.targetScope : "enemy",
    selectRule: SELECT_RULE_OPTIONS.some((item) => item.value === raw.selectRule) ? raw.selectRule : "first",
    selectAttrKey: asString(raw.selectAttrKey ?? "").trim(),
    fallbackSelfIfEmpty: Boolean(raw.fallbackSelfIfEmpty),
    targetCount: {
      base: asNumber(targetCount.base ?? 1, 1),
      formula: Array.isArray(targetCount.formula)
        ? targetCount.formula.map(normalizeTerm).filter((term) => term.attrKey || term.coefficient || term.perSkillLevel)
        : []
    },
    targetPredicates: Array.isArray(raw.targetPredicates)
      ? raw.targetPredicates.filter((p) => p && typeof p === "object" && EFFECT_KIND_OPTIONS_PREDICATE_KINDS.has(p.kind))
      : []
  }
}

function normalizeFormula(input, fallback = [makeDefaultTerm()]) {
  const terms = parseArrayLike(input, fallback)
  return terms.map(normalizeTerm).filter((term) => term.attrKey || term.coefficient || term.perSkillLevel)
}

function normalizeDamageSpec(input) {
  const raw = input && typeof input === "object" ? input : {}
  const hitCount = raw.hitCount && typeof raw.hitCount === "object" ? raw.hitCount : {}
  const bonusAgainstTargetTypes = raw.bonusAgainstTargetTypes && typeof raw.bonusAgainstTargetTypes === "object" && !Array.isArray(raw.bonusAgainstTargetTypes)
    ? raw.bonusAgainstTargetTypes
    : {}
  return {
    damageType: raw.damageType === "magical" ? "magical" : "physical",
    amountFormula: {
      baseAdd: asNumber(raw.amountFormula?.baseAdd ?? 0, 0),
      terms: normalizeFormula(raw.amountFormula?.terms, [makeDefaultTerm()])
    },
    flatDefensePenetrationFormula: {
      baseAdd: asNumber(raw.flatDefensePenetrationFormula?.baseAdd ?? 0, 0),
      terms: normalizeFormula(raw.flatDefensePenetrationFormula?.terms, [])
    },
    percentDefensePenetrationFormula: {
      baseAdd: asNumber(raw.percentDefensePenetrationFormula?.baseAdd ?? 0, 0),
      terms: normalizeFormula(raw.percentDefensePenetrationFormula?.terms, [])
    },
    physicalFlatDefensePenetrationFormula: {
      baseAdd: asNumber(raw.physicalFlatDefensePenetrationFormula?.baseAdd ?? 0, 0),
      terms: normalizeFormula(raw.physicalFlatDefensePenetrationFormula?.terms, [])
    },
    physicalPercentDefensePenetrationFormula: {
      baseAdd: asNumber(raw.physicalPercentDefensePenetrationFormula?.baseAdd ?? 0, 0),
      terms: normalizeFormula(raw.physicalPercentDefensePenetrationFormula?.terms, [])
    },
    magicalFlatDefensePenetrationFormula: {
      baseAdd: asNumber(raw.magicalFlatDefensePenetrationFormula?.baseAdd ?? 0, 0),
      terms: normalizeFormula(raw.magicalFlatDefensePenetrationFormula?.terms, [])
    },
    magicalPercentDefensePenetrationFormula: {
      baseAdd: asNumber(raw.magicalPercentDefensePenetrationFormula?.baseAdd ?? 0, 0),
      terms: normalizeFormula(raw.magicalPercentDefensePenetrationFormula?.terms, [])
    },
    extraDamageAmplificationFormula: {
      baseAdd: asNumber(raw.extraDamageAmplificationFormula?.baseAdd ?? 0, 0),
      terms: normalizeFormula(raw.extraDamageAmplificationFormula?.terms, [])
    },
    bonusAgainstTargetTypes: Object.fromEntries(
      Object.entries(bonusAgainstTargetTypes)
        .map(([key, value]) => [asString(key).trim(), asNumber(value, Number.NaN)])
        .filter(([key, value]) => key && Number.isFinite(value))
    ),
    hitCount: {
      base: Math.max(0, Math.trunc(asNumber(hitCount.base ?? 0, 0))),
      formula: Array.isArray(hitCount.formula) ? hitCount.formula.map(normalizeTerm).filter((term) => term.attrKey || term.coefficient || term.perSkillLevel) : []
    }
  }
}

function isFormulaEmpty(formula) {
  if (!formula || typeof formula !== "object") return true
  const baseAdd = asNumber(formula.baseAdd ?? 0, 0)
  const terms = Array.isArray(formula.terms) ? formula.terms : []
  return baseAdd === 0 && terms.length === 0
}

function compactDamageSpec(spec) {
  const normalized = normalizeDamageSpec(spec)
  const out = {
    damageType: normalized.damageType,
    amountFormula: normalized.amountFormula
  }
  const optionalFormulaFields = [
    "flatDefensePenetrationFormula",
    "percentDefensePenetrationFormula",
    "physicalFlatDefensePenetrationFormula",
    "physicalPercentDefensePenetrationFormula",
    "magicalFlatDefensePenetrationFormula",
    "magicalPercentDefensePenetrationFormula",
    "extraDamageAmplificationFormula"
  ]
  for (const key of optionalFormulaFields) {
    if (!isFormulaEmpty(normalized[key])) out[key] = normalized[key]
  }
  if (normalized.bonusAgainstTargetTypes && Object.keys(normalized.bonusAgainstTargetTypes).length) {
    out.bonusAgainstTargetTypes = normalized.bonusAgainstTargetTypes
  }
  if (normalized.hitCount && (normalized.hitCount.base > 0 || normalized.hitCount.formula.length > 0)) {
    out.hitCount = normalized.hitCount
  }
  return out
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
      kind,
      targeting,
      ignoreUntargetable: Boolean(raw.ignoreUntargetable),
      onlyWhenNoTargetFromEffects,
      reuseTargetsFromEffect,
      statusPayload: {
        statusIds: statusIds.length ? statusIds : ["bleed"],
        durationBaseTurns: Math.max(0, Math.trunc(asNumber(payload.durationBaseTurns ?? 0, 0))),
        durationPerSkillLevelTurns: asNumber(payload.durationPerSkillLevelTurns ?? 0, 0)
      }
    }
  }

  const damageSpec = kind === "damage" ? normalizeDamageSpec(raw.damageSpec) : null
  const fallbackAmountFormula = kind === "damage" ? damageSpec.amountFormula : { baseAdd: 0, terms: [makeDefaultTerm()] }
  const amountFormula = raw.amountFormula && typeof raw.amountFormula === "object" ? raw.amountFormula : fallbackAmountFormula
  const out = {
    kind,
    targeting,
    ignoreUntargetable: Boolean(raw.ignoreUntargetable),
    onlyWhenNoTargetFromEffects,
    reuseTargetsFromEffect,
    amountFormula: {
      baseAdd: asNumber(amountFormula.baseAdd ?? 0, 0),
      terms: normalizeFormula(amountFormula.terms, [makeDefaultTerm()])
    }
  }

  if (kind === "damage") {
    out.damageSpec = damageSpec
    out.damageType = raw.damageType === "magical" ? "magical" : out.damageSpec.damageType
    out.amountFormula = {
      baseAdd: asNumber(out.damageSpec.amountFormula?.baseAdd ?? out.amountFormula.baseAdd, 0),
      terms: normalizeFormula(out.damageSpec.amountFormula?.terms ?? out.amountFormula.terms, [makeDefaultTerm()])
    }
  } else {
    out.damageType = raw.damageType === "magical" ? "magical" : "physical"
  }
  if (kind === "heal") {
    const mpCostBase = asNumber(raw.mpCost?.baseAdd ?? 0, 0)
    const mpCostTerms = normalizeFormula(raw.mpCost?.terms, [])
    if (mpCostBase !== 0 || mpCostTerms.length > 0) {
      out.mpCost = { baseAdd: mpCostBase, terms: mpCostTerms }
    }
  }
  return out
}

function normalizeEffects(input) {
  const list = Array.isArray(input) ? input.map(normalizeEffect) : []
  return list.length ? list : [makeDefaultEffect("damage")]
}

function normalizeBattleHooks(input) {
  const raw = Array.isArray(input) ? input : []
  const out = []
  for (const row of raw) {
    if (!row || typeof row !== "object" || Array.isArray(row)) continue
    const trigger = asString(row.trigger || row.when).trim().toLowerCase()
    if (!trigger) continue
    const effectsRaw = Array.isArray(row.effects) ? row.effects : []
    const effects = []
    for (const effectNode of effectsRaw) {
      if (!effectNode || typeof effectNode !== "object" || Array.isArray(effectNode)) continue
      const effect = effectNode
      const kindRaw = asString(effect.kind || effect.type).trim().toLowerCase()
      if (!kindRaw) continue
      if (kindRaw === "add_buff" || kindRaw === "addbuff") {
        const buffId = asString(effect.buffId).trim()
        if (!buffId) continue
        const durationTurns = Math.max(0, Math.trunc(asNumber(effect.durationTurns, 0)))
        const targetModeRaw = asString(effect.targetMode || effect.target).trim().toLowerCase()
        const targetMode = targetModeRaw === "source" || targetModeRaw === "targeting" ? targetModeRaw : "self"
        effects.push({
          kind: "add_buff",
          buffId,
          ...(durationTurns > 0 ? { durationTurns } : {}),
          targetMode,
          ...(targetMode === "targeting" ? { targeting: normalizeTargeting(effect.targeting) } : {})
        })
        continue
      }
      effects.push({ ...effect, kind: kindRaw })
    }
    if (!effects.length) continue
    const conditionsRaw = Array.isArray(row.conditions) ? row.conditions : []
    const conditions = conditionsRaw.map((condition) => normalizeBattleHookCondition(condition)).filter(Boolean)
    const id = asString(row.id).trim()
    out.push({
      ...(id ? { id } : {}),
      trigger,
      priority: Math.trunc(asNumber(row.priority, 0)),
      conditions,
      effects
    })
  }
  return out
}

function normalizeMapObject(input, valueKind = "number") {
  const raw = parseObjectLike(input, {})
  return Object.fromEntries(
    Object.entries(raw)
      .map(([key, value]) => [asString(key).trim(), valueKind === "number" ? asNumber(value, Number.NaN) : asString(value).trim()])
      .filter(([key, value]) => key && (valueKind === "number" ? Number.isFinite(value) : Boolean(value)))
  )
}

function normalizeListText(value) {
  return parseListLike(value, [])
}

function makeDefaultEnhanceCurve() {
  return {
    base: {},
    step: {},
    overrides: {}
  }
}

function normalizeEnhanceCurve(input, valueKind = "int") {
  const raw = input && typeof input === "object" && !Array.isArray(input) ? input : {}
  const base = normalizeMapObject(raw.base, valueKind === "int" ? "number" : "number")
  const step = normalizeMapObject(raw.step, valueKind === "int" ? "number" : "number")
  const overrides = raw.overrides && typeof raw.overrides === "object" && !Array.isArray(raw.overrides) ? raw.overrides : {}
  const normalizedOverrides = {}
  for (const [level, mapValue] of Object.entries(overrides)) {
    const levelText = asString(level).trim()
    const parsedLevel = Math.trunc(asNumber(levelText, Number.NaN))
    if (!Number.isFinite(parsedLevel) || parsedLevel <= 0) continue
    normalizedOverrides[String(parsedLevel)] = normalizeMapObject(mapValue, valueKind === "int" ? "number" : "number")
  }
  return {
    base,
    step,
    overrides: normalizedOverrides
  }
}

function formatList(value) {
  const list = normalizeListText(value)
  return list.join(", ")
}

function makeDefaultWeaponTemplate(type) {
  const configs = {
    sword: {
      id: "long_sword",
      weaponClass: ["sword"],
      baseAttrs: { physics_damage: 12, strength: 6 },
      effects: [makeDefaultEffect("damage")]
    },
    spear: {
      id: "iron_spear",
      weaponClass: ["spear"],
      baseAttrs: { physics_damage: 10, strength: 5, dexterity: 2 },
      effects: [makeDefaultEffect("damage")]
    },
    bow: {
      id: "hunter_bow",
      weaponClass: ["bow"],
      baseAttrs: { physics_damage: 9, dexterity: 7 },
      effects: [makeDefaultEffect("damage")]
    },
    staff: {
      id: "apprentice_staff",
      weaponClass: ["staff"],
      baseAttrs: { magical_damage: 11, intelligence: 8 },
      effects: [makeDefaultEffect("damage")]
    }
  }
  const cfg = configs[type] || configs.sword
  return {
    id: cfg.id,
    equipType: "battle",
    slot: "weapon",
    occupySlots: JSON.stringify(["weapon"], null, 2),
    weaponClass: cfg.weaponClass.join(", "),
    statusRequirement: JSON.stringify({}, null, 2),
    accessoryGroup: "",
    affixSlotCount: 2,
    affixAllowedIds: JSON.stringify([], null, 2),
    baseAttrs: JSON.stringify(cfg.baseAttrs, null, 2),
    maxDurability: 100,
    decayRate: 0,
    enhanceMaxLevel: 10,
    enhanceDurationS: 8,
    enhanceExp: 1,
    enhanceProtectItemId: "",
    enhanceCostCurve: JSON.stringify({ base: { gold_coin: 1000 }, step: { gold_coin: 200 }, overrides: {} }, null, 2),
    enhanceAttrCurve: JSON.stringify({ base: { physics_damage: 2 }, step: { physics_damage: 1 }, overrides: {} }, null, 2),
    battleHooks: JSON.stringify([], null, 2),
    basicAttackEffects: JSON.stringify(
      cfg.effects.map((effect) => ({
        ...effect,
        damageSpec: effect.kind === "damage" ? makeDefaultDamageSpec() : undefined
      })),
      null,
      2
    )
  }
}

const templates = {
  sword: {
    name: "长剑模板",
    row: () => makeDefaultWeaponTemplate("sword")
  },
  spear: {
    name: "长枪模板",
    row: () => makeDefaultWeaponTemplate("spear")
  },
  bow: {
    name: "弓模板",
    row: () => makeDefaultWeaponTemplate("bow")
  },
  staff: {
    name: "法杖模板",
    row: () => makeDefaultWeaponTemplate("staff")
  }
}

function buildTemplates() {
  return Object.entries(templates).map(([key, value]) => ({ key, label: value.name }))
}

function createRowFromTemplate(templateKey) {
  const factory = templates[templateKey] || templates.sword
  return { __id: uid(), ...factory.row() }
}

function renderOptionList(options, selected) {
  return options
    .map((option) => `<option value="${escapeHtml(option.value)}" ${option.value === selected ? "selected" : ""}>${escapeHtml(option.label)}</option>`)
    .join("")
}

function toOptionHtml(options, selected) {
  return options
    .map((option) => `<option value="${escapeHtml(option)}" ${option === selected ? "selected" : ""}>${escapeHtml(option)}</option>`)
    .join("")
}

function normalizeSelectOptions(options) {
  if (!Array.isArray(options)) return []
  return options
    .map((option) => {
      if (option && typeof option === "object" && !Array.isArray(option)) {
        const value = asString(option.value).trim()
        if (!value) return null
        const label = asString(option.label || value).trim() || value
        return { value, label }
      }

      function findOptionLabel(options, value) {
        const normalized = normalizeSelectOptions(options)
        const matched = normalized.find((option) => option.value === value)
        return matched ? matched.label : value
      }
      const value = asString(option).trim()
      if (!value) return null
      return { value, label: value }
    })
    .filter(Boolean)
}

function renderHead() {
  tableHead.innerHTML = columns.map((column) => `<th>${escapeHtml(column.label)}</th>`).join("") + "<th>actions</th>"
}

function summarizeObject(value) {
  const obj = parseObjectLike(value, {})
  const entries = Object.entries(obj)
  if (!entries.length) return "无"
  return entries.slice(0, 3).map(([key, val]) => `${key}:${val}`).join(" / ") + (entries.length > 3 ? " ..." : "")
}

function summarizeList(value) {
  const list = parseListLike(value, [])
  return list.length ? list.slice(0, 4).join(" / ") + (list.length > 4 ? " ..." : "") : "无"
}

function summarizeTargeting(value) {
  const targeting = normalizeTargeting(safeParseJson(value, makeDefaultTargeting()))
  const positions = targeting.targetPositions.map((item) => findOptionLabel(POSITION_OPTIONS, item)).join("+")
  const scope = findOptionLabel(TARGET_SCOPE_OPTIONS, targeting.targetScope)
  const seek = findOptionLabel(SEEK_OPTIONS, targeting.secondarySeek)
  const rule = findOptionLabel(SELECT_RULE_OPTIONS, targeting.selectRule)
  return `${positions} · ${scope} · ${rule} · ${seek} · 数量 ${targeting.targetCount.base}`
}

function summarizeDamageSpec(value) {
  const spec = normalizeDamageSpec(safeParseJson(value, makeDefaultDamageSpec()))
  const damageTypeLabel = findOptionLabel(DAMAGE_TYPE_OPTIONS, spec.damageType)
  return `${damageTypeLabel} · ${spec.amountFormula.terms.length} 项 · hit ${spec.hitCount.base}`
}

function summarizeEffects(value) {
  const effects = normalizeEffects(safeParseJson(value, [makeDefaultEffect("damage")]))
  return effects
    .map((effect) => {
      if (effect.kind === "status") return `状态 x${effect.statusPayload.statusIds.length}`
      if (effect.kind === "heal") return `治疗 ${effect.amountFormula.terms.length} 项`
      const spec = effect.damageSpec ? ` / spec ${effect.damageSpec.hitCount.base}` : ""
      return `伤害 ${effect.amountFormula.terms.length} 项${spec}`
    })
    .join(" | ")
}

function summarizeBattleHooks(value) {
  const hooks = normalizeBattleHooks(safeParseJson(value, []))
  if (!hooks.length) return ""
  return hooks.map((hook) => {
    const triggerLabel = findOptionLabel(BATTLE_HOOK_TRIGGER_OPTIONS, hook.trigger)
    const effects = (Array.isArray(hook.effects) ? hook.effects : [])
      .map((effect) => {
        const kind = asString(effect.kind).trim().toLowerCase()
        if (kind === "add_buff") {
          const buffId = asString(effect.buffId).trim() || "?"
          const turns = Math.max(0, Math.trunc(asNumber(effect.durationTurns, 0)))
          return turns > 0 ? `add_buff:${buffId}(${turns}t)` : `add_buff:${buffId}`
        }
        return kind || "unknown"
      })
      .join(", ")
    return `${triggerLabel} -> ${effects}`
  }).join(" | ")
}

function renderCellValue(row, column) {
  const value = row[column.key]
  let preview = asString(value)
  if (column.key === "occupySlots" || column.key === "affixAllowedIds") {
    preview = summarizeList(value)
  } else if (column.key === "statusRequirement" || column.key === "baseAttrs" || column.key === "enhanceCostCurve" || column.key === "enhanceAttrCurve") {
    preview = summarizeObject(value)
  } else if (column.key === "battleHooks") {
    preview = summarizeBattleHooks(value)
  } else if (column.key === "basicAttackEffects") {
    preview = summarizeEffects(value)
  }
  return `<div class="visual-cell"><div class="visual-preview">${escapeHtml(preview)}</div></div>`
}

function renderBody() {
  tableBody.innerHTML = rows
    .map((row, index) => {
      const cells = columns.map((column) => `<td>${renderCellValue(row, column)}</td>`).join("")
      return `
        <tr data-row-id="${escapeHtml(row.__id)}">
          ${cells}
          <td class="actions">
            <div class="row-action-group">
              <button type="button" class="primary" data-action="edit-row" data-row-id="${escapeHtml(row.__id)}">编辑</button>
              <button type="button" data-action="dup" data-row-id="${escapeHtml(row.__id)}">复制</button>
              <button type="button" data-action="up" data-row-id="${escapeHtml(row.__id)}">↑</button>
              <button type="button" data-action="down" data-row-id="${escapeHtml(row.__id)}">↓</button>
              <button type="button" class="danger" data-action="del" data-row-id="${escapeHtml(row.__id)}">删</button>
              <span class="muted">#${index + 1}</span>
            </div>
          </td>
        </tr>
      `
    })
    .join("")
}

function render() {
  renderHead()
  templateSelect.innerHTML = buildTemplates().map((template) => `<option value="${escapeHtml(template.key)}">${escapeHtml(template.label)}</option>`).join("")
  renderBody()
}

function setStatus(message, isError = false) {
  statusEl.textContent = message
  statusEl.style.color = isError ? "var(--danger)" : ""
}

function findRow(rowId) {
  return rows.find((row) => row.__id === rowId)
}

function normalizeWeaponRowFromJson(raw, fallbackRow = null) {
  const source = raw && typeof raw === "object" && !Array.isArray(raw) ? raw : {}
  const fallback = fallbackRow && typeof fallbackRow === "object" ? fallbackRow : {}
  const occupySlots = parseArrayLike(source.occupySlots ?? fallback.occupySlots ?? ["weapon"], ["weapon"])
  const weaponClass = parseListLike(source.weaponClass ?? fallback.weaponClass ?? "sword", ["sword"])
  const statusRequirement = parseObjectLike(source.statusRequirement ?? fallback.statusRequirement ?? {}, {})
  const affixAllowedIds = parseArrayLike(source.affixAllowedIds ?? fallback.affixAllowedIds ?? [], [])
  const baseAttrs = parseObjectLike(source.baseAttrs ?? fallback.baseAttrs ?? {}, {})
  const enhanceCostCurve = normalizeEnhanceCurve(source.enhanceCostCurve ?? fallback.enhanceCostCurve ?? makeDefaultEnhanceCurve(), "int")
  const enhanceAttrCurve = normalizeEnhanceCurve(source.enhanceAttrCurve ?? fallback.enhanceAttrCurve ?? makeDefaultEnhanceCurve(), "float")
  const battleHooks = normalizeBattleHooks(source.battleHooks ?? fallback.battleHooks ?? [])
  const basicAttackEffects = normalizeEffects(source.basicAttackEffects ?? fallback.basicAttackEffects ?? [makeDefaultEffect("damage")])

  return {
    __id: fallback.__id || uid(),
    id: asString(source.id ?? fallback.id).trim(),
    equipType: source.equipType === "work" ? "work" : "battle",
    slot: asString(source.slot ?? fallback.slot ?? "weapon").trim(),
    occupySlots: JSON.stringify(occupySlots, null, 2),
    weaponClass: weaponClass.join(", "),
    statusRequirement: JSON.stringify(statusRequirement, null, 2),
    accessoryGroup: asString(source.accessoryGroup ?? fallback.accessoryGroup ?? "").trim(),
    affixSlotCount: Math.max(0, Math.trunc(asNumber(source.affixSlotCount ?? fallback.affixSlotCount ?? 0, 0))),
    affixAllowedIds: JSON.stringify(affixAllowedIds, null, 2),
    baseAttrs: JSON.stringify(baseAttrs, null, 2),
    maxDurability: Math.max(1, asNumber(source.maxDurability ?? fallback.maxDurability ?? 100, 100)),
    decayRate: Math.max(0, asNumber(source.decayRate ?? fallback.decayRate ?? 0, 0)),
    enhanceMaxLevel: Math.max(0, Math.trunc(asNumber(source.enhanceMaxLevel ?? fallback.enhanceMaxLevel ?? 10, 10))),
    enhanceDurationS: Math.max(1, Math.trunc(asNumber(source.enhanceDurationS ?? fallback.enhanceDurationS ?? 8, 8))),
    enhanceExp: Math.max(0, asNumber(source.enhanceExp ?? fallback.enhanceExp ?? 1, 1)),
    enhanceProtectItemId: formatList(source.enhanceProtectItemId ?? fallback.enhanceProtectItemId ?? ""),
    enhanceCostCurve: JSON.stringify(enhanceCostCurve, null, 2),
    enhanceAttrCurve: JSON.stringify(enhanceAttrCurve, null, 2),
    battleHooks: JSON.stringify(battleHooks, null, 2),
    basicAttackEffects: JSON.stringify(basicAttackEffects, null, 2)
  }
}

function renderJsonEditorSection(row) {
  return `
    <section class="editor-section json-editor">
      <details>
        <summary>高级：JSON 导入 / 导出（可选）</summary>
        <div class="section-head">
          <h4>JSON 导入 / 导出</h4>
          <div class="json-editor-actions">
            <button type="button" data-action="load-json">导入到表单</button>
            <button type="button" class="primary" data-action="copy-json">复制 JSON</button>
          </div>
        </div>
        <label class="cell-stack">装备模板 JSON
          <textarea class="weapon-json-textarea" data-weapon-json spellcheck="false" placeholder="粘贴单个 equipment_templates 对象 JSON">${escapeHtml(weaponJsonTextFromRow(row))}</textarea>
        </label>
        <div class="muted">主流程请使用可视化控件；此区域仅用于高级导入导出。</div>
      </details>
    </section>
  `
}

function renderBattleHooksEditor(row) {
  const hooks = normalizeBattleHooks(safeParseJson(row.battleHooks, []))
  return `
    <section class="editor-section">
      <div class="repeat-header">
        <strong>战斗钩子（battleHooks）</strong>
        <button type="button" data-action="add-battle-hook">新增钩子</button>
      </div>
      <div class="effect-list" data-battle-hook-list>
        ${(hooks.length ? hooks : [makeDefaultBattleHook()]).map((hook, index) => renderBattleHookCard(hook, index)).join("")}
      </div>
      <div class="muted">可视化编辑：触发时机 + 效果；当前支持“受到伤害后 -> add_buff”。</div>
    </section>
  `
}

function makeDefaultBattleHook() {
  return {
    trigger: "on_after_take_damage",
    priority: 0,
    conditions: [makeDefaultBattleHookCondition()],
    effects: [makeDefaultBattleHookEffect()]
  }
}

function makeDefaultBattleHookEffect() {
  return {
    kind: "add_buff",
    buffId: "",
    durationTurns: 1
  }
}

function makeDefaultBattleHookCondition() {
  return {
    kind: "incoming_damage_gte",
    value: 1,
    buffId: ""
  }
}

function normalizeBattleHookCondition(input) {
  if (!input || typeof input !== "object" || Array.isArray(input)) return null
  const kindRaw = asString(input.kind || input.type).trim().toLowerCase()
  if (!kindRaw) return null
  const kind = kindRaw.replaceAll(" ", "_")
  const allowed = new Set(BATTLE_HOOK_CONDITION_OPTIONS.map((item) => item.value))
  if (!allowed.has(kind)) return null
  if (kind === "has_buff" || kind === "source_has_buff") {
    const buffId = asString(input.buffId).trim()
    if (!buffId) return null
    return { kind, buffId }
  }
  return { kind, value: asNumber(input.value, 0) }
}

function normalizeBattleHookEffect(input) {
  const source = input && typeof input === "object" && !Array.isArray(input) ? input : {}
  const kindRaw = asString(source.kind || source.type).trim().toLowerCase()
  const kind = kindRaw === "addbuff" ? "add_buff" : (kindRaw || "add_buff")
  const targetModeRaw = asString(source.targetMode || source.target).trim().toLowerCase()
  const targetMode = targetModeRaw === "source" || targetModeRaw === "targeting" ? targetModeRaw : "self"
  return {
    kind: "add_buff",
    buffId: asString(source.buffId).trim(),
    durationTurns: Math.max(0, Math.trunc(asNumber(source.durationTurns, 1))),
    targetMode,
    targeting: normalizeTargeting(source.targeting)
  }
}

function renderBattleHookEffectRow(effect = null) {
  const normalized = normalizeBattleHookEffect(effect || makeDefaultBattleHookEffect())
  return `
    <div class="repeat-block" data-battle-hook-effect-entry>
      <div class="repeat-row" data-battle-hook-effect-row>
        <label class="cell-stack">效果类型（kind）
          <select data-battle-hook-effect-field="kind">
            <option value="add_buff" selected>添加 buff（add_buff）</option>
          </select>
        </label>
        <label class="cell-stack">状态 ID（buffId）
          <input type="text" data-battle-hook-effect-field="buffId" value="${escapeHtml(normalized.buffId)}" placeholder="例如 camouflage" />
        </label>
        <label class="cell-stack">持续回合（durationTurns）
          <input type="number" step="1" min="0" data-battle-hook-effect-field="durationTurns" value="${escapeHtml(normalized.durationTurns)}" />
        </label>
        <label class="cell-stack">目标模式（targetMode）
          <select data-battle-hook-effect-field="targetMode">
            ${renderOptionList(BATTLE_HOOK_TARGET_MODE_OPTIONS, normalized.targetMode)}
          </select>
        </label>
        <button type="button" data-action="remove-battle-hook-effect">删</button>
      </div>
      <div class="repeat-block">
        <div class="repeat-header">
          <strong>目标规则（targeting）</strong>
        </div>
        <div class="pill-grid">
          ${POSITION_OPTIONS.map((option) => `
            <label class="pill">
              <input type="checkbox" data-battle-hook-target-position value="${escapeHtml(option.value)}" ${normalized.targeting.targetPositions.includes(option.value) ? "checked" : ""} />
              <span>${escapeHtml(option.label)}</span>
            </label>
          `).join("")}
        </div>
        <div class="form-grid compact">
          <label>次要寻敌（secondarySeek）
            <select data-battle-hook-targeting-field="secondarySeek">${renderOptionList(SEEK_OPTIONS, normalized.targeting.secondarySeek)}</select>
          </label>
          <label>目标范围（targetScope）
            <select data-battle-hook-targeting-field="targetScope">${renderOptionList(TARGET_SCOPE_OPTIONS, normalized.targeting.targetScope)}</select>
          </label>
          <label>选择逻辑（selectRule）
            <select data-battle-hook-targeting-field="selectRule">${renderOptionList(SELECT_RULE_OPTIONS, normalized.targeting.selectRule)}</select>
          </label>
          <label>属性键（selectAttrKey）
            <select data-battle-hook-targeting-field="selectAttrKey">
              <option value="">不使用</option>
              ${ATTR_SELECT_OPTIONS.map((option) => `<option value="${escapeHtml(option.value)}" ${option.value === normalized.targeting.selectAttrKey ? "selected" : ""}>${escapeHtml(option.label)}</option>`).join("")}
            </select>
          </label>
          <label>基础数量（targetCount.base）
            <input type="number" step="1" min="1" data-battle-hook-targeting-field="targetCountBase" value="${escapeHtml(normalized.targeting.targetCount.base)}" />
          </label>
          <label class="checkbox-inline">
            <input type="checkbox" data-battle-hook-targeting-field="fallbackSelfIfEmpty" ${normalized.targeting.fallbackSelfIfEmpty ? "checked" : ""} />
            <span>无目标回退自己（fallbackSelfIfEmpty）</span>
          </label>
        </div>
        <div class="repeat-block">
          <div class="repeat-header">
            <strong>目标数量系数（targetCount.formula）</strong>
            <button type="button" data-action="add-battle-hook-target-count-term">新增系数</button>
          </div>
          <div class="repeat-list" data-battle-hook-target-count-formula-list>
            ${(Array.isArray(normalized.targeting.targetCount.formula) && normalized.targeting.targetCount.formula.length
              ? normalized.targeting.targetCount.formula
              : []).map((term) => renderBattleHookTargetCountFormulaRow(term)).join("")}
          </div>
        </div>
      </div>
    </div>
  `
}

function renderBattleHookConditionRow(condition = null) {
  const normalized = normalizeBattleHookCondition(condition) || makeDefaultBattleHookCondition()
  return `
    <div class="repeat-row" data-battle-hook-condition-row>
      <label class="cell-stack">条件类型（kind）
        <select data-battle-hook-condition-field="kind">
          ${renderOptionList(BATTLE_HOOK_CONDITION_OPTIONS, normalized.kind)}
        </select>
      </label>
      <label class="cell-stack">条件数值（value）
        <input type="number" step="0.01" data-battle-hook-condition-field="value" value="${escapeHtml(asNumber(normalized.value, 0))}" />
      </label>
      <label class="cell-stack">条件状态 ID（buffId）
        <input type="text" data-battle-hook-condition-field="buffId" value="${escapeHtml(normalized.buffId || "")}" placeholder="has_buff/source_has_buff 时填写" />
      </label>
      <button type="button" data-action="remove-battle-hook-condition">删</button>
    </div>
  `
}

function renderBattleHookTargetCountFormulaRow(term = null) {
  const normalized = normalizeTerm(term || makeDefaultTerm())
  return `
    <div class="repeat-row" data-battle-hook-target-count-formula-row>
      <label class="cell-stack">属性键（attrKey）
        <select data-battle-hook-target-count-formula-field="attrKey">
          <option value="">未选择</option>
          ${ATTR_SELECT_OPTIONS.map((option) => `<option value="${escapeHtml(option.value)}" ${option.value === (normalized.attrKey || "") ? "selected" : ""}>${escapeHtml(option.label)}</option>`).join("")}
        </select>
      </label>
      <label class="cell-stack">系数（coefficient）
        <input type="number" step="0.01" data-battle-hook-target-count-formula-field="coefficient" value="${escapeHtml(normalized.coefficient)}" />
      </label>
      <label class="cell-stack">每级增量（perSkillLevel）
        <input type="number" step="0.01" data-battle-hook-target-count-formula-field="perSkillLevel" value="${escapeHtml(normalized.perSkillLevel)}" />
      </label>
      <button type="button" data-action="remove-battle-hook-target-count-term">删</button>
    </div>
  `
}

function renderBattleHookCard(hook, index) {
  const normalized = normalizeBattleHooks([hook])[0] || makeDefaultBattleHook()
  return `
    <article class="effect-card" data-battle-hook-card>
      <header class="effect-head">
        <div class="effect-title">
          <strong>钩子 #${index + 1}</strong>
        </div>
        <div class="effect-actions">
          <button type="button" data-action="move-battle-hook-up">↑</button>
          <button type="button" data-action="move-battle-hook-down">↓</button>
          <button type="button" class="danger" data-action="remove-battle-hook">删</button>
        </div>
      </header>
      <div class="form-grid compact">
        <label>触发时机（trigger）
          <select data-battle-hook-field="trigger">${renderOptionList(BATTLE_HOOK_TRIGGER_OPTIONS, normalized.trigger)}</select>
        </label>
        <label>优先级（priority）
          <input type="number" step="1" data-battle-hook-field="priority" value="${escapeHtml(normalized.priority)}" />
        </label>
      </div>
      <div class="repeat-block">
        <div class="repeat-header">
          <strong>conditions（触发条件）</strong>
          <button type="button" data-action="add-battle-hook-condition">新增条件</button>
        </div>
        <div class="repeat-list" data-battle-hook-condition-list>
          ${(Array.isArray(normalized.conditions) && normalized.conditions.length ? normalized.conditions : [makeDefaultBattleHookCondition()]).map((condition) => renderBattleHookConditionRow(condition)).join("")}
        </div>
      </div>
      <div class="repeat-block">
        <div class="repeat-header">
          <strong>effects（触发效果）</strong>
          <button type="button" data-action="add-battle-hook-effect">新增效果</button>
        </div>
        <div class="repeat-list" data-battle-hook-effect-list>
          ${(Array.isArray(normalized.effects) && normalized.effects.length ? normalized.effects : [makeDefaultBattleHookEffect()]).map((effect) => renderBattleHookEffectRow(effect)).join("")}
        </div>
      </div>
    </article>
  `
}

function renderMapRow(row = { key: "", value: 0 }, scope, valueKind = "number", keyOptions = null) {
  const normalizedOptions = normalizeSelectOptions(keyOptions)
  const keyControl = Array.isArray(keyOptions) && keyOptions.length
    ? `
      <select data-map-field="key">
        <option value="">未选择</option>
        ${normalizedOptions.map((option) => `<option value="${escapeHtml(option.value)}" ${option.value === (row.key ?? "") ? "selected" : ""}>${escapeHtml(option.label)}</option>`).join("")}
      </select>
    `
    : `<input type="text" data-map-field="key" placeholder="key" value="${escapeHtml(row.key ?? "")}" />`
  return `
    <div class="repeat-row" data-map-row="${escapeHtml(scope)}">
      <label class="cell-stack">键
        ${keyControl}
      </label>
      <label class="cell-stack">值
        <input type="${valueKind === "number" ? "number" : "text"}" step="${valueKind === "number" ? "0.01" : "1"}" data-map-field="value" placeholder="value" value="${escapeHtml(row.value ?? "")}" />
      </label>
      <button type="button" data-action="remove-map-row">删</button>
    </div>
  `
}

function renderMapEditor(title, scope, map, valueKind = "number", keyOptions = null, allowEmpty = false) {
  const entries = Object.entries(map || {}).map(([key, value]) => ({ key, value }))
  const initial = entries.length ? entries : (allowEmpty ? [] : [{ key: "", value: valueKind === "number" ? 0 : "" }])
  const mergedKeyOptions = Array.isArray(keyOptions) && keyOptions.length
    ? Array.from(new Set([...keyOptions, ...entries.map((item) => item.key)])).filter(Boolean)
    : null
  return `
    <div class="repeat-block">
      <div class="repeat-header">
        <strong>${escapeHtml(title)}</strong>
        <button type="button" data-action="add-map-row" data-map-scope="${escapeHtml(scope)}">新增键值</button>
      </div>
      <div
        class="repeat-list"
        data-map-list="${escapeHtml(scope)}"
        data-map-value-kind="${escapeHtml(valueKind)}"
        data-map-key-options="${escapeHtml(JSON.stringify(mergedKeyOptions || []))}"
      >
        ${initial.map((item) => renderMapRow(item, scope, valueKind, mergedKeyOptions)).join("")}
      </div>
    </div>
  `
}

function renderListRow(value = "", scope, options = null) {
  const normalizedOptions = normalizeSelectOptions(options)
  const control = Array.isArray(options) && options.length
    ? `
      <select data-list-field="value">
        <option value="">未选择</option>
        ${normalizedOptions.map((option) => `<option value="${escapeHtml(option.value)}" ${option.value === (value ?? "") ? "selected" : ""}>${escapeHtml(option.label)}</option>`).join("")}
      </select>
    `
    : `<input type="text" data-list-field="value" placeholder="值" value="${escapeHtml(value)}" />`
  return `
    <div class="repeat-row" data-list-row="${escapeHtml(scope)}">
      <label class="cell-stack">值
        ${control}
      </label>
      <button type="button" data-action="remove-list-row">删</button>
    </div>
  `
}

function renderListEditor(title, scope, list, options = null) {
  const values = Array.isArray(list) ? list : []
  const initial = values.length ? values : [""]
  const mergedOptions = Array.isArray(options) && options.length
    ? Array.from(new Set([...options, ...values])).filter(Boolean)
    : null
  return `
    <div class="repeat-block">
      <div class="repeat-header">
        <strong>${escapeHtml(title)}</strong>
        <button type="button" data-action="add-list-row" data-list-scope="${escapeHtml(scope)}">新增项</button>
      </div>
      <div class="repeat-list" data-list="${escapeHtml(scope)}" data-list-options="${escapeHtml(JSON.stringify(mergedOptions || []))}">
        ${initial.map((value) => renderListRow(value, scope, mergedOptions)).join("")}
      </div>
    </div>
  `
}

function renderFormulaRow(term, scope) {
  return `
    <div class="repeat-row" data-term-row="${escapeHtml(scope)}">
      <label class="cell-stack">属性键
        <select data-term-field="attrKey">
          <option value="">未选择</option>
          ${ATTR_SELECT_OPTIONS.map((option) => `<option value="${escapeHtml(option.value)}" ${option.value === (term?.attrKey || "") ? "selected" : ""}>${escapeHtml(option.label)}</option>`).join("")}
        </select>
      </label>
      <label class="cell-stack">系数
        <input type="number" step="0.01" data-term-field="coefficient" placeholder="系数" value="${escapeHtml(term?.coefficient ?? 0)}" />
      </label>
      <label class="cell-stack">每级增量
        <input type="number" step="0.01" data-term-field="perSkillLevel" placeholder="每级" value="${escapeHtml(term?.perSkillLevel ?? 0)}" />
      </label>
      <button type="button" data-action="remove-term">删</button>
    </div>
  `
}

function renderFormulaEditor(title, scope, terms, allowEmpty = false) {
  const list = Array.isArray(terms) && terms.length ? terms : (allowEmpty ? [] : [makeDefaultTerm()])
  return `
    <div class="repeat-block">
      <div class="repeat-header">
        <strong>${escapeHtml(title)}</strong>
        <button type="button" data-action="add-term" data-term-scope="${escapeHtml(scope)}">新增系数</button>
      </div>
      <div class="repeat-list" data-term-list="${escapeHtml(scope)}">
        ${list.map((term) => renderFormulaRow(term, scope)).join("")}
      </div>
    </div>
  `
}

function renderTargetingEditor(targeting) {
  return `
    <div class="editor-grid">
      <section class="editor-section">
        <h4>目标位置</h4>
        <div class="pill-grid">
          ${POSITION_OPTIONS.map((option) => `
            <label class="pill">
              <input type="checkbox" name="targetPositions" value="${escapeHtml(option.value)}" ${targeting.targetPositions.includes(option.value) ? "checked" : ""} />
              <span>${escapeHtml(option.label)}</span>
            </label>
          `).join("")}
        </div>
        <div class="inline-actions">
          <button type="button" data-action="target-positions-all">全选</button>
          <button type="button" data-action="target-positions-clear">清空</button>
        </div>
      </section>

      <section class="editor-section">
        <h4>寻敌与选择</h4>
        <div class="form-grid">
          <label>次要寻敌
            <select name="secondarySeek">${renderOptionList(SEEK_OPTIONS, targeting.secondarySeek)}</select>
          </label>
          <label>目标范围
            <select name="targetScope">${renderOptionList(TARGET_SCOPE_OPTIONS, targeting.targetScope)}</select>
          </label>
          <label>选择逻辑
            <select name="selectRule">${renderOptionList(SELECT_RULE_OPTIONS, targeting.selectRule)}</select>
          </label>
          <label>属性键
            <select name="selectAttrKey">
              <option value="">不使用</option>
              ${ATTR_SELECT_OPTIONS.map((option) => `<option value="${escapeHtml(option.value)}" ${option.value === targeting.selectAttrKey ? "selected" : ""}>${escapeHtml(option.label)}</option>`).join("")}
            </select>
          </label>
          <label class="checkbox-inline">
            <input type="checkbox" name="fallbackSelfIfEmpty" ${targeting.fallbackSelfIfEmpty ? "checked" : ""} />
            <span>无目标时回退自己</span>
          </label>
        </div>
      </section>

      <section class="editor-section">
        <h4>目标数量</h4>
        <div class="form-grid compact">
          <label>基础数量
            <input type="number" step="1" name="targetCountBase" value="${escapeHtml(targeting.targetCount.base)}" min="1" />
          </label>
        </div>
        ${renderFormulaEditor("目标数量系数", "targetCount", targeting.targetCount.formula)}
      </section>
    </div>
  `
}

function renderDamageSpecEditor(spec) {
  const normalized = normalizeDamageSpec(spec)
  return `
    <section class="editor-section">
      <h4>伤害细节</h4>
      <div class="form-grid">
        <label>伤害类型
          <select name="specDamageType">${renderOptionList(DAMAGE_TYPE_OPTIONS, normalized.damageType)}</select>
        </label>
        <label>命中次数基础值
          <input type="number" step="1" name="specHitCountBase" value="${escapeHtml(normalized.hitCount.base || "")}" min="0" />
        </label>
      </div>
      ${renderFormulaEditor("伤害值公式", "specAmountFormula", normalized.amountFormula.terms)}
      <label class="field-note">伤害值基础加成
        <input type="number" step="0.01" name="specAmountBaseAdd" value="${escapeHtml(normalized.amountFormula.baseAdd)}" />
      </label>
      ${renderFormulaEditor("穿透固定公式", "specFlatPenetration", normalized.flatDefensePenetrationFormula.terms, true)}
      <label class="field-note">穿透固定基础加成
        <input type="number" step="0.01" name="specFlatBaseAdd" value="${escapeHtml(normalized.flatDefensePenetrationFormula.baseAdd)}" />
      </label>
      ${renderFormulaEditor("穿透百分比公式", "specPercentPenetration", normalized.percentDefensePenetrationFormula.terms, true)}
      <label class="field-note">穿透百分比基础加成
        <input type="number" step="0.01" name="specPercentBaseAdd" value="${escapeHtml(normalized.percentDefensePenetrationFormula.baseAdd)}" />
      </label>
      ${renderFormulaEditor("物理固定穿透公式", "specPhysicalFlatPenetration", normalized.physicalFlatDefensePenetrationFormula.terms, true)}
      <label class="field-note">物理固定穿透基础加成
        <input type="number" step="0.01" name="specPhysicalFlatBaseAdd" value="${escapeHtml(normalized.physicalFlatDefensePenetrationFormula.baseAdd)}" />
      </label>
      ${renderFormulaEditor("物理百分比穿透公式", "specPhysicalPercentPenetration", normalized.physicalPercentDefensePenetrationFormula.terms, true)}
      <label class="field-note">物理百分比穿透基础加成
        <input type="number" step="0.01" name="specPhysicalPercentBaseAdd" value="${escapeHtml(normalized.physicalPercentDefensePenetrationFormula.baseAdd)}" />
      </label>
      ${renderFormulaEditor("魔法固定穿透公式", "specMagicalFlatPenetration", normalized.magicalFlatDefensePenetrationFormula.terms, true)}
      <label class="field-note">魔法固定穿透基础加成
        <input type="number" step="0.01" name="specMagicalFlatBaseAdd" value="${escapeHtml(normalized.magicalFlatDefensePenetrationFormula.baseAdd)}" />
      </label>
      ${renderFormulaEditor("魔法百分比穿透公式", "specMagicalPercentPenetration", normalized.magicalPercentDefensePenetrationFormula.terms, true)}
      <label class="field-note">魔法百分比穿透基础加成
        <input type="number" step="0.01" name="specMagicalPercentBaseAdd" value="${escapeHtml(normalized.magicalPercentDefensePenetrationFormula.baseAdd)}" />
      </label>
      ${renderFormulaEditor("额外增伤公式", "specExtraAmplification", normalized.extraDamageAmplificationFormula.terms, true)}
      <label class="field-note">额外增伤基础加成
        <input type="number" step="0.01" name="specExtraBaseAdd" value="${escapeHtml(normalized.extraDamageAmplificationFormula.baseAdd)}" />
      </label>
      ${renderMapEditor("目标类型加成", "bonusAgainstTargetTypes", normalized.bonusAgainstTargetTypes, "number", null, true)}
      ${renderFormulaEditor("命中次数系数", "specHitCountFormula", normalized.hitCount.formula, true)}
    </section>
  `
}

function renderEffectTargetingEditor(targeting) {
  return `
    <section class="editor-section">
      <h4>效果目标规则</h4>
      <div class="pill-grid">
        ${POSITION_OPTIONS.map((option) => `
          <label class="pill">
            <input type="checkbox" data-effect-target-position value="${escapeHtml(option.value)}" ${targeting.targetPositions.includes(option.value) ? "checked" : ""} />
            <span>${escapeHtml(option.label)}</span>
          </label>
        `).join("")}
      </div>
      <div class="form-grid compact">
        <label>次要寻敌
          <select data-effect-targeting-field="secondarySeek">${renderOptionList(SEEK_OPTIONS, targeting.secondarySeek)}</select>
        </label>
        <label>目标范围
          <select data-effect-targeting-field="targetScope">${renderOptionList(TARGET_SCOPE_OPTIONS, targeting.targetScope)}</select>
        </label>
        <label>选择逻辑
          <select data-effect-targeting-field="selectRule">${renderOptionList(SELECT_RULE_OPTIONS, targeting.selectRule)}</select>
        </label>
        <label>属性键
          <select data-effect-targeting-field="selectAttrKey">
            <option value="">不使用</option>
            ${ATTR_SELECT_OPTIONS.map((option) => `<option value="${escapeHtml(option.value)}" ${option.value === targeting.selectAttrKey ? "selected" : ""}>${escapeHtml(option.label)}</option>`).join("")}
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
      ${renderFormulaEditor("目标数量系数", "effectTargetCount", targeting.targetCount.formula)}
      <div class="form-grid compact">
        <label class="checkbox-inline">
          <input type="checkbox" data-effect-predicate="target_hp_not_full" ${(targeting.targetPredicates || []).some((p) => p.kind === "target_hp_not_full") ? "checked" : ""} />
          <span>仅生命值不满的目标</span>
        </label>
      </div>
    </section>
  `
}

function renderEffectCard(effect, index) {
  const normalized = normalizeEffect(effect)
  const kindLabel = EFFECT_KIND_OPTIONS.find((item) => item.value === normalized.kind)?.label || normalized.kind
  return `
    <article class="effect-card" data-effect-card data-effect-kind="${escapeHtml(normalized.kind)}">
      <header class="effect-head">
        <div class="effect-title">
          <strong>效果 #${index + 1}</strong>
          <span class="muted">${escapeHtml(kindLabel)}</span>
        </div>
        <div class="effect-actions">
          <button type="button" data-action="move-effect-up">↑</button>
          <button type="button" data-action="move-effect-down">↓</button>
          <button type="button" class="danger" data-action="remove-effect">删</button>
        </div>
      </header>

      <div class="form-grid compact">
        <label>类型
          <select name="effectKind">${renderOptionList(EFFECT_KIND_OPTIONS, normalized.kind)}</select>
        </label>
        ${normalized.kind !== "status" ? `
          <label>damageType
            <select name="damageType">${renderOptionList(DAMAGE_TYPE_OPTIONS, normalized.damageType || normalized.damageSpec?.damageType || "physical")}</select>
          </label>
        ` : ""}
        <label>仅当前序效果无目标（逗号分隔索引）
          <input type="text" name="onlyWhenNoTargetFromEffects" value="${escapeHtml(formatEffectReferenceIndexes(normalized.onlyWhenNoTargetFromEffects))}" placeholder="例如 1,2" />
          <small class="muted">注：被引用效果若有魔法消耗（mpCost），魔法不足时同样视为「无目标」并触发此效果</small>
        </label>
        <label>复用目标来源效果索引
          <input type="number" step="1" min="0" name="reuseTargetsFromEffect" value="${escapeHtml(Math.max(0, Math.trunc(asNumber(normalized.reuseTargetsFromEffect, 0))))}" />
        </label>
        <label class="checkbox-inline">
          <input type="checkbox" name="ignoreUntargetable" ${normalized.ignoreUntargetable ? "checked" : ""} />
          <span>忽略不可选中</span>
        </label>
      </div>

      ${renderEffectTargetingEditor(normalized.targeting)}

      <div class="effect-group effect-group-damage">
        ${normalized.kind === "damage"
          ? `
            <div class="muted">伤害效果仅配置 damageSpec.amountFormula；effect.amountFormula 会自动同步生成。</div>
            ${renderDamageSpecEditor(normalized.damageSpec)}
          `
          : `
            <div class="form-grid compact">
              <label>基础加数
                <input type="number" step="0.01" name="baseAdd" value="${escapeHtml(normalized.amountFormula.baseAdd)}" />
              </label>
            </div>
            ${renderFormulaEditor("公式项", "effect", normalized.amountFormula.terms)}
          `
        }
      </div>

      ${normalized.kind === "heal" ? `
        <div class="effect-group">
          <h4>魔法消耗（mpCost）</h4>
          <p class="muted">每次触发此效果前消耗魔法值。魔法不足时，此效果视为「无目标」跳过，可配合 onlyWhenNoTargetFromEffects 触发备选伤害效果。</p>
          <div class="form-grid compact">
            <label>基础消耗
              <input type="number" step="0.01" name="mpCostBaseAdd" value="${escapeHtml(normalized.mpCost?.baseAdd ?? 0)}" />
            </label>
          </div>
          ${renderFormulaEditor("消耗公式项", "mpCost", normalized.mpCost?.terms ?? [])}
        </div>
      ` : ""}

      <div class="effect-group effect-group-status">
        <div class="repeat-block">
          <div class="repeat-header">
            <strong>状态 id</strong>
            <button type="button" data-action="add-status-id">新增状态</button>
          </div>
          <div class="repeat-list" data-status-list>
            ${(normalized.statusPayload?.statusIds?.length ? normalized.statusPayload.statusIds : ["bleed"]).map((statusId) => `
              <div class="repeat-row" data-status-row>
                <label class="cell-stack">状态 ID
                  <input type="text" data-status-field="statusId" placeholder="状态 id" value="${escapeHtml(statusId)}" />
                </label>
                <button type="button" data-action="remove-status-id">删</button>
              </div>
            `).join("")}
          </div>
        </div>
        <div class="form-grid compact">
          <label>持续回合基础值
            <input type="number" step="1" name="durationBaseTurns" value="${escapeHtml(normalized.statusPayload?.durationBaseTurns ?? 0)}" />
          </label>
          <label>持续回合系数
            <input type="number" step="0.01" name="durationPerSkillLevelTurns" value="${escapeHtml(normalized.statusPayload?.durationPerSkillLevelTurns ?? 0)}" />
          </label>
        </div>
      </div>
    </article>
  `
}

function renderEffectsEditor(effects) {
  const list = Array.isArray(effects) && effects.length ? effects : [makeDefaultEffect("damage")]
  return `
    <section class="editor-section">
      <div class="repeat-header">
        <strong>普攻效果</strong>
        <div class="inline-actions">
          <button type="button" data-action="add-effect" data-effect-kind="damage">新增伤害</button>
          <button type="button" data-action="add-effect" data-effect-kind="heal">新增治疗</button>
          <button type="button" data-action="add-effect" data-effect-kind="status">新增状态</button>
        </div>
      </div>
      <div class="effect-list">
        ${list.map((effect, index) => renderEffectCard(effect, index)).join("")}
      </div>
    </section>
  `
}

function renderBasicFields(row) {
  return `
    <section class="editor-section">
      <h4>基础信息</h4>
      <div class="form-grid">
        <label>id
          <input type="text" name="id" value="${escapeHtml(row.id)}" />
        </label>
        <label>equipType
          <select name="equipType">${renderOptionList([{ value: "battle", label: "战斗" }, { value: "work", label: "生活" }], row.equipType)}</select>
        </label>
        <label>slot
          <select name="slot">${toOptionHtml(Array.from(new Set([...DEFAULT_SLOT_OPTIONS, row.slot].filter(Boolean))), row.slot)}</select>
        </label>
        <label>accessoryGroup
          <input type="text" name="accessoryGroup" value="${escapeHtml(row.accessoryGroup)}" />
        </label>
        <label>maxDurability
          <input type="number" step="1" name="maxDurability" value="${escapeHtml(row.maxDurability)}" min="1" />
        </label>
        <label>decayRate
          <input type="number" step="0.01" name="decayRate" value="${escapeHtml(row.decayRate)}" min="0" />
        </label>
      </div>
    </section>
  `
}

function renderWeaponClassEditor(row) {
  return `
    <section class="editor-section">
      <h4>武器槽位 / 类别</h4>
      ${renderListEditor("占用槽位", "occupySlots", parseArrayLike(row.occupySlots, []), DEFAULT_SLOT_OPTIONS)}
      ${renderListEditor("武器类别", "weaponClass", parseListLike(row.weaponClass, []), DEFAULT_WEAPON_CLASS_OPTIONS)}
      <div class="muted">占用槽位和武器类别都可在这里直接增删，不再手填 JSON。</div>
    </section>
  `
}

function renderRequirementEditor(row) {
  return `
    <section class="editor-section">
      <h4>属性 / 词条</h4>
      ${renderMapEditor("属性门槛", "statusRequirement", parseObjectLike(row.statusRequirement, {}), "number", ATTR_SELECT_OPTIONS)}
      ${renderMapEditor("基础属性", "baseAttrs", parseObjectLike(row.baseAttrs, {}), "number", ATTR_SELECT_OPTIONS)}
      <div class="form-grid">
        <label>affixSlotCount
          <input type="number" step="1" name="affixSlotCount" value="${escapeHtml(row.affixSlotCount)}" min="0" />
        </label>
      </div>
      ${renderListEditor("词条白名单", "affixAllowedIds", parseArrayLike(row.affixAllowedIds, []))}
      <div class="muted">词条白名单也改成了可交互列表。</div>
    </section>
  `
}

function renderEnhanceEditor(row) {
  const costCurve = safeParseJson(row.enhanceCostCurve, makeDefaultEnhanceCurve())
  const attrCurve = safeParseJson(row.enhanceAttrCurve, makeDefaultEnhanceCurve())
  return `
    <section class="editor-section">
      <h4>强化配置</h4>
      <div class="form-grid">
        <label>enhanceMaxLevel
          <input type="number" step="1" name="enhanceMaxLevel" value="${escapeHtml(row.enhanceMaxLevel)}" min="0" />
        </label>
        <label>enhanceDurationS
          <input type="number" step="1" name="enhanceDurationS" value="${escapeHtml(row.enhanceDurationS)}" min="1" />
        </label>
        <label>enhanceExp
          <input type="number" step="0.01" name="enhanceExp" value="${escapeHtml(row.enhanceExp)}" min="0" />
        </label>
        <label>enhanceProtectItemId（逗号分隔）
          <input type="text" name="enhanceProtectItemId" value="${escapeHtml(row.enhanceProtectItemId)}" />
        </label>
      </div>
      ${renderCurveEditor("enhanceCostCurve", "enhanceCostCurve", costCurve, "number")}
      ${renderCurveEditor("enhanceAttrCurve", "enhanceAttrCurve", attrCurve, "number")}
    </section>
  `
}

function renderCurveEditor(title, scope, curve, valueKind = "number") {
  const base = curve?.base && typeof curve.base === "object" ? curve.base : {}
  const step = curve?.step && typeof curve.step === "object" ? curve.step : {}
  const overrides = curve?.overrides && typeof curve.overrides === "object" ? curve.overrides : {}
  const overrideEntries = Object.entries(overrides)
  const initialOverrides = overrideEntries.length ? overrideEntries : [["1", {}]]
  return `
    <section class="editor-section">
      <div class="repeat-header">
        <strong>${escapeHtml(title)}</strong>
        <button type="button" data-action="add-curve-override" data-curve-scope="${escapeHtml(scope)}">新增覆盖</button>
      </div>
      ${renderMapEditor(`${title}.base`, `${scope}-base`, base, valueKind, scope === "enhanceAttrCurve" ? ATTR_SELECT_OPTIONS : null)}
      ${renderMapEditor(`${title}.step`, `${scope}-step`, step, valueKind, scope === "enhanceAttrCurve" ? ATTR_SELECT_OPTIONS : null)}
      <div class="repeat-block">
        <div class="repeat-header"><strong>${escapeHtml(title)}.overrides</strong></div>
        <div class="repeat-list" data-override-list="${escapeHtml(scope)}">
          ${initialOverrides.map(([level, map]) => renderCurveOverrideRow(level, map, scope, valueKind)).join("")}
        </div>
      </div>
    </section>
  `
}

function renderCurveOverrideRow(level, map, scope, valueKind = "number") {
  return `
    <article class="editor-section" data-curve-override-row data-curve-scope="${escapeHtml(scope)}">
      <div class="form-grid">
        <label>等级
          <input type="number" step="1" min="1" data-curve-field="level" value="${escapeHtml(level)}" />
        </label>
        <div class="inline-actions">
          <button type="button" data-action="remove-curve-override">删</button>
        </div>
      </div>
      ${renderMapEditor("覆盖值", `${scope}-override`, map, valueKind)}
    </article>
  `
}

function renderWeaponEditor(row) {
  const basicAttackEffects = normalizeEffects(safeParseJson(row.basicAttackEffects, [makeDefaultEffect("damage")]))
  return `
    <div class="editor-grid">
      ${renderJsonEditorSection(row)}
      <section class="editor-section">
        <h4>实时校验</h4>
        <ul id="editor-validation-list" class="validation-list"></ul>
      </section>
      ${renderBasicFields(row)}
      ${renderWeaponClassEditor(row)}
      ${renderRequirementEditor(row)}
      ${renderEnhanceEditor(row)}
      ${renderBattleHooksEditor(row)}
      ${renderEffectsEditor(basicAttackEffects)}
    </div>
  `
}

function weaponJsonTextFromRow(row) {
  return JSON.stringify(weaponRowToJsonObject(row), null, 2)
}

function weaponRowToJsonObject(row) {
  return {
    id: asString(row.id).trim(),
    equipType: asString(row.equipType || "battle"),
    slot: asString(row.slot).trim(),
    occupySlots: parseArrayLike(row.occupySlots, ["weapon"]),
    weaponClass: parseListLike(row.weaponClass, []),
    statusRequirement: parseObjectLike(row.statusRequirement, {}),
    accessoryGroup: asString(row.accessoryGroup).trim(),
    affixSlotCount: Math.max(0, Math.trunc(asNumber(row.affixSlotCount, 0))),
    affixAllowedIds: parseArrayLike(row.affixAllowedIds, []),
    baseAttrs: parseObjectLike(row.baseAttrs, {}),
    maxDurability: Math.max(1, asNumber(row.maxDurability, 100)),
    decayRate: Math.max(0, asNumber(row.decayRate, 0)),
    enhanceMaxLevel: Math.max(0, Math.trunc(asNumber(row.enhanceMaxLevel, 0))),
    enhanceDurationS: Math.max(1, Math.trunc(asNumber(row.enhanceDurationS, 1))),
    enhanceExp: Math.max(0, asNumber(row.enhanceExp, 0)),
    enhanceProtectItemId: parseListLike(row.enhanceProtectItemId, []),
    enhanceCostCurve: safeParseJson(row.enhanceCostCurve, makeDefaultEnhanceCurve()),
    enhanceAttrCurve: safeParseJson(row.enhanceAttrCurve, makeDefaultEnhanceCurve()),
    battleHooks: normalizeBattleHooks(safeParseJson(row.battleHooks, [])),
    basicAttackEffects: normalizeEffects(safeParseJson(row.basicAttackEffects, [makeDefaultEffect("damage")]))
  }
}

function renderJsonImportText(text) {
  const textarea = editorContent.querySelector("[data-weapon-json]")
  if (textarea instanceof HTMLTextAreaElement) textarea.value = text
}

function openEditorWithRow(row, titleOverride = "") {
  editorModal.dataset.rowId = row.__id
  editorTitle.textContent = titleOverride || `${row.id || "未命名武器"} · 武器编辑`
  editorContent.innerHTML = renderWeaponEditor(row)
  refreshEditorValidation(row)
  editorModal.classList.remove("hidden")
}

function openEditor(rowId) {
  const row = findRow(rowId)
  if (row) openEditorWithRow(row)
}

function closeEditor() {
  editorModal.classList.add("hidden")
  editorModal.removeAttribute("data-row-id")
  editorContent.innerHTML = ""
}

function getEditorJsonTextarea() {
  const field = editorContent.querySelector("[data-weapon-json]")
  return field instanceof HTMLTextAreaElement ? field : null
}

function importJsonIntoEditor() {
  const textarea = getEditorJsonTextarea()
  if (!textarea) return
  const raw = textarea.value.trim()
  if (!raw) {
    setStatus("请输入武器 JSON", true)
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
  const normalized = normalizeWeaponRowFromJson(parsed, currentRow)
  editorTitle.textContent = `${normalized.id || "未命名武器"} · 武器编辑`
  editorContent.innerHTML = renderWeaponEditor(normalized)
  refreshEditorValidation(normalized)
  setStatus("JSON 已导入")
}

async function copyCurrentWeaponJson() {
  const rowId = editorModal.dataset.rowId || ""
  const currentRow = rowId ? findRow(rowId) : null
  const draft = buildEditorDraft(currentRow)
  const errors = validateWeaponDraft(draft)
  renderEditorValidation(errors)
  if (errors.length > 0) {
    setStatus(`复制失败：${errors[0]}`, true)
    return
  }
  const text = JSON.stringify(weaponDraftToJsonObject(draft), null, 2)
  const textarea = getEditorJsonTextarea()
  if (textarea) textarea.value = text
  await copyToClipboard(text)
  setStatus("JSON 已复制")
}

function copyCurrentWeaponJsonText() {
  return copyCurrentWeaponJson()
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
    row.occupySlots = compactJson(row.occupySlots, ["weapon"])
    row.statusRequirement = compactJson(row.statusRequirement, {})
    row.affixAllowedIds = compactJson(row.affixAllowedIds, [])
    row.baseAttrs = compactJson(row.baseAttrs, {})
    row.enhanceCostCurve = compactJson(row.enhanceCostCurve, makeDefaultEnhanceCurve())
    row.enhanceAttrCurve = compactJson(row.enhanceAttrCurve, makeDefaultEnhanceCurve())
    row.basicAttackEffects = compactJson(row.basicAttackEffects, [makeDefaultEffect("damage")])
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

function normalizeRowForExport(row) {
  const out = {}
  for (const column of columns) {
    const raw = row[column.key]
    if (column.type === "number") {
      const num = Number(raw)
      out[column.key] = Number.isFinite(num) ? String(num) : "0"
    } else if (column.type === "json") {
      out[column.key] = compactJson(raw, column.key === "occupySlots" || column.key === "affixAllowedIds" ? [] : {})
    } else {
      out[column.key] = asString(raw).replaceAll(/\t/g, " ").replaceAll(/\r?\n/g, " ")
    }
  }
  return out
}

function toTsv() {
  const header = columns.map((column) => column.label).join("\t")
  const body = rows.map((row) => {
    const exportRow = normalizeRowForExport(row)
    return columns.map((column) => exportRow[column.key]).join("\t")
  })
  return [header, ...body].join("\n")
}

async function copyTsv() {
  await copyToClipboard(toTsv())
  setStatus("TSV 已复制")
}

function downloadTsv() {
  const blob = new Blob([toTsv()], { type: "text/tab-separated-values;charset=utf-8" })
  const url = URL.createObjectURL(blob)
  const link = document.createElement("a")
  link.href = url
  link.download = "weapons.tsv"
  document.body.appendChild(link)
  link.click()
  link.remove()
  URL.revokeObjectURL(url)
  setStatus("TSV 已下载")
}

function parseTsvRows(tsvText) {
  const lines = asString(tsvText)
    .replaceAll(/\r/g, "")
    .split("\n")
    .filter((line) => line.trim() !== "")
  if (lines.length < 2) throw new Error("TSV 至少需要表头和一行数据")
  const header = lines[0].split("\t").map((item) => item.trim())
  const colIndexMap = new Map()
  for (const column of columns) {
    const idx = header.findIndex((item) => item === column.label)
    if (idx < 0) throw new Error(`TSV 缺少列: ${column.label}`)
    colIndexMap.set(column.key, idx)
  }
  return lines.slice(1).map((line) => {
    const parts = line.split("\t")
    const row = { __id: uid() }
    for (const column of columns) {
      const index = colIndexMap.get(column.key)
      const raw = index < parts.length ? parts[index] : ""
      row[column.key] = column.type === "number" ? asNumber(raw, 0) : asString(raw)
    }
    return row
  })
}

function importTsv() {
  const raw = window.prompt("请粘贴 equipment_templates 的 TSV（包含表头）")
  if (!raw || !raw.trim()) return
  rows = parseTsvRows(raw)
  renderBody()
  setStatus(`已导入 ${rows.length} 行`)
}

function loadTemplate(templateKey) {
  rows = [createRowFromTemplate(templateKey)]
  renderBody()
  setStatus(`已载入 ${templates[templateKey]?.name || templateKey}`)
}

function resetSampleRows() {
  rows = [createRowFromTemplate("sword"), createRowFromTemplate("bow"), createRowFromTemplate("staff")]
  renderBody()
  setStatus("示例已重置")
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
  const cloneRow = { __id: uid(), ...clone(rows[index]) }
  cloneRow.__id = uid()
  rows.splice(index + 1, 0, cloneRow)
  renderBody()
}

function updateRowField(rowId, key, value) {
  const row = findRow(rowId)
  if (!row) return
  row[key] = value
}

function buildEditorDraft(existingRow = null) {
  const query = (selector) => editorContent.querySelector(selector)
  const id = asString(query('input[name="id"]')?.value).trim()
  const equipType = query('select[name="equipType"]')?.value === "work" ? "work" : "battle"
  const slot = asString(query('select[name="slot"]')?.value).trim()
  const accessoryGroup = asString(query('input[name="accessoryGroup"]')?.value).trim()
  const maxDurability = asNumber(query('input[name="maxDurability"]')?.value, 100)
  const decayRate = asNumber(query('input[name="decayRate"]')?.value, 0)
  const affixSlotCount = Math.max(0, Math.trunc(asNumber(query('input[name="affixSlotCount"]')?.value, 0)))
  const enhanceMaxLevel = Math.max(0, Math.trunc(asNumber(query('input[name="enhanceMaxLevel"]')?.value, 0)))
  const enhanceDurationS = Math.max(1, Math.trunc(asNumber(query('input[name="enhanceDurationS"]')?.value, 1)))
  const enhanceExp = Math.max(0, asNumber(query('input[name="enhanceExp"]')?.value, 0))
  const occupySlots = collectList(editorContent.querySelector('[data-list="occupySlots"]') || editorContent)
  const weaponClass = collectList(editorContent.querySelector('[data-list="weaponClass"]') || editorContent)
  const statusRequirement = collectMapObject(editorContent.querySelector('[data-map-list="statusRequirement"]') || editorContent, "number")
  const affixAllowedIds = collectList(editorContent.querySelector('[data-list="affixAllowedIds"]') || editorContent)
  const baseAttrs = collectMapObject(editorContent.querySelector('[data-map-list="baseAttrs"]') || editorContent, "number")
  const enhanceProtectItemId = parseListLike(query('input[name="enhanceProtectItemId"]')?.value ?? "", [])
  const enhanceCostCurve = collectCurve("enhanceCostCurve")
  const enhanceAttrCurve = collectCurve("enhanceAttrCurve")
  const battleHooks = collectBattleHooks()
  const basicAttackEffects = collectEffects()

  return {
    id: id || asString(existingRow?.id).trim(),
    equipType,
    slot,
    occupySlots,
    weaponClass,
    statusRequirement,
    accessoryGroup,
    affixSlotCount,
    affixAllowedIds,
    baseAttrs,
    maxDurability,
    decayRate,
    enhanceMaxLevel,
    enhanceDurationS,
    enhanceExp,
    enhanceProtectItemId,
    enhanceCostCurve,
    enhanceAttrCurve,
    battleHooks,
    basicAttackEffects
  }
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

function collectMap(root) {
  return Array.from(root.querySelectorAll("[data-map-row]"))
    .map((row) => ({
      key: row.querySelector('[data-map-field="key"]')?.value.trim() || "",
      value: row.querySelector('[data-map-field="value"]')?.value
    }))
    .filter((item) => item.key !== "")
}

function collectMapObject(root, valueKind = "number") {
  return collectMap(root).reduce((acc, item) => {
    acc[item.key] = valueKind === "number" ? asNumber(item.value, 0) : asString(item.value).trim()
    return acc
  }, {})
}

function collectList(root) {
  return Array.from(root.querySelectorAll("[data-list-row]"))
    .map((row) => row.querySelector('[data-list-field="value"]')?.value.trim() || "")
    .filter(Boolean)
}

function collectCurve(scope) {
  const base = collectMapObject(editorContent.querySelector(`[data-map-list="${scope}-base"]`) || editorContent, "number")
  const step = collectMapObject(editorContent.querySelector(`[data-map-list="${scope}-step"]`) || editorContent, "number")
  const overrides = {}
  for (const row of editorContent.querySelectorAll(`[data-curve-override-row][data-curve-scope="${scope}"]`)) {
    const level = Math.trunc(asNumber(row.querySelector('[data-curve-field="level"]')?.value, 0))
    if (level <= 0) continue
    const mapRoot = row.querySelector('[data-map-list$="-override"]') || row
    overrides[String(level)] = collectMapObject(mapRoot, "number")
  }
  return { base, step, overrides }
}

function collectTargetingFromContainer(container) {
  const positions = Array.from(container.querySelectorAll('input[name="targetPositions"]:checked')).map((item) => item.value)
  const secondarySeek = container.querySelector('select[name="secondarySeek"]')?.value || "front_to_rear"
  const targetScope = container.querySelector('select[name="targetScope"]')?.value || "enemy"
  const selectRule = container.querySelector('select[name="selectRule"]')?.value || "first"
  const selectAttrKey = container.querySelector('select[name="selectAttrKey"]')?.value.trim() || ""
  const fallbackSelfIfEmpty = Boolean(container.querySelector('input[name="fallbackSelfIfEmpty"]')?.checked)
  const targetCountBase = asNumber(container.querySelector('input[name="targetCountBase"]')?.value, 1)
  const targetCountFormula = collectTerms(container.querySelector('[data-term-list="targetCount"]') || container)
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

function collectEffectTargeting(card) {
  const positions = Array.from(card.querySelectorAll('[data-effect-target-position]:checked')).map((item) => item.value)
  const secondarySeek = card.querySelector('[data-effect-targeting-field="secondarySeek"]')?.value || "front_to_rear"
  const targetScope = card.querySelector('[data-effect-targeting-field="targetScope"]')?.value || "enemy"
  const selectRule = card.querySelector('[data-effect-targeting-field="selectRule"]')?.value || "first"
  const selectAttrKey = card.querySelector('[data-effect-targeting-field="selectAttrKey"]')?.value.trim() || ""
  const fallbackSelfIfEmpty = Boolean(card.querySelector('[data-effect-targeting-field="fallbackSelfIfEmpty"]')?.checked)
  const targetCountBase = asNumber(card.querySelector('[data-effect-targeting-field="targetCountBase"]')?.value, 1)
  const targetCountFormula = collectTerms(card.querySelector('[data-term-list="effectTargetCount"]') || card)
  const targetPredicates = []
  if (card.querySelector('[data-effect-predicate="target_hp_not_full"]')?.checked) {
    targetPredicates.push({ kind: "target_hp_not_full" })
  }
  return normalizeTargeting({
    targetPositions: positions,
    secondarySeek,
    targetScope,
    selectRule,
    selectAttrKey,
    fallbackSelfIfEmpty,
    targetCount: { base: targetCountBase, formula: targetCountFormula },
    targetPredicates
  })
}

function collectDamageSpec(root) {
  const damageType = root.querySelector('select[name="specDamageType"]')?.value || "physical"
  const amountBaseAdd = asNumber(root.querySelector('input[name="specAmountBaseAdd"]')?.value, 0)
  const flatBaseAdd = asNumber(root.querySelector('input[name="specFlatBaseAdd"]')?.value, 0)
  const percentBaseAdd = asNumber(root.querySelector('input[name="specPercentBaseAdd"]')?.value, 0)
  const physicalFlatBaseAdd = asNumber(root.querySelector('input[name="specPhysicalFlatBaseAdd"]')?.value, 0)
  const physicalPercentBaseAdd = asNumber(root.querySelector('input[name="specPhysicalPercentBaseAdd"]')?.value, 0)
  const magicalFlatBaseAdd = asNumber(root.querySelector('input[name="specMagicalFlatBaseAdd"]')?.value, 0)
  const magicalPercentBaseAdd = asNumber(root.querySelector('input[name="specMagicalPercentBaseAdd"]')?.value, 0)
  const extraBaseAdd = asNumber(root.querySelector('input[name="specExtraBaseAdd"]')?.value, 0)
  const hitCountBase = Math.max(0, Math.trunc(asNumber(root.querySelector('input[name="specHitCountBase"]')?.value, 0)))
  return compactDamageSpec({
    damageType,
    amountFormula: { baseAdd: amountBaseAdd, terms: collectTerms(root.querySelector('[data-term-list="specAmountFormula"]') || root) },
    flatDefensePenetrationFormula: { baseAdd: flatBaseAdd, terms: collectTerms(root.querySelector('[data-term-list="specFlatPenetration"]') || root) },
    percentDefensePenetrationFormula: { baseAdd: percentBaseAdd, terms: collectTerms(root.querySelector('[data-term-list="specPercentPenetration"]') || root) },
    physicalFlatDefensePenetrationFormula: { baseAdd: physicalFlatBaseAdd, terms: collectTerms(root.querySelector('[data-term-list="specPhysicalFlatPenetration"]') || root) },
    physicalPercentDefensePenetrationFormula: { baseAdd: physicalPercentBaseAdd, terms: collectTerms(root.querySelector('[data-term-list="specPhysicalPercentPenetration"]') || root) },
    magicalFlatDefensePenetrationFormula: { baseAdd: magicalFlatBaseAdd, terms: collectTerms(root.querySelector('[data-term-list="specMagicalFlatPenetration"]') || root) },
    magicalPercentDefensePenetrationFormula: { baseAdd: magicalPercentBaseAdd, terms: collectTerms(root.querySelector('[data-term-list="specMagicalPercentPenetration"]') || root) },
    extraDamageAmplificationFormula: { baseAdd: extraBaseAdd, terms: collectTerms(root.querySelector('[data-term-list="specExtraAmplification"]') || root) },
    bonusAgainstTargetTypes: collectMapObject(root.querySelector('[data-map-list="bonusAgainstTargetTypes"]') || root, "number"),
    hitCount: { base: hitCountBase, formula: collectTerms(root.querySelector('[data-term-list="specHitCountFormula"]') || root) }
  })
}

function collectEffects() {
  return Array.from(editorContent.querySelectorAll("[data-effect-card]")).map((card) => {
    const kind = card.querySelector('select[name="effectKind"]')?.value || "damage"
    const targeting = collectEffectTargeting(card)
    const ignoreUntargetable = Boolean(card.querySelector('input[name="ignoreUntargetable"]')?.checked)
    const onlyWhenNoTargetFromEffects = normalizeEffectReferenceIndexes(card.querySelector('input[name="onlyWhenNoTargetFromEffects"]')?.value || "")
    const reuseTargetsFromEffect = Math.max(0, Math.trunc(asNumber(card.querySelector('input[name="reuseTargetsFromEffect"]')?.value, 0)))
    const selectedDamageType = card.querySelector('select[name="damageType"]')?.value === "magical" ? "magical" : "physical"
    if (kind === "status") {
      return normalizeEffect({
        kind,
        targeting,
        ignoreUntargetable,
        onlyWhenNoTargetFromEffects,
        reuseTargetsFromEffect,
        statusPayload: {
          statusIds: Array.from(card.querySelectorAll("[data-status-row]")).map((row) => row.querySelector('[data-status-field="statusId"]')?.value.trim() || "").filter(Boolean),
          durationBaseTurns: Math.max(0, Math.trunc(asNumber(card.querySelector('input[name="durationBaseTurns"]')?.value, 0))),
          durationPerSkillLevelTurns: asNumber(card.querySelector('input[name="durationPerSkillLevelTurns"]')?.value, 0)
        }
      })
    }
    if (kind === "damage") {
      const damageSpec = collectDamageSpec(card)
      damageSpec.damageType = selectedDamageType
      const effect = normalizeEffect({
        kind,
        targeting,
        ignoreUntargetable,
        onlyWhenNoTargetFromEffects,
        reuseTargetsFromEffect,
        damageSpec,
        amountFormula: damageSpec.amountFormula,
        damageType: selectedDamageType
      })
      effect.damageSpec = damageSpec
      effect.damageType = selectedDamageType
      effect.amountFormula = damageSpec.amountFormula
      return effect
    }
    const amountBaseAdd = asNumber(card.querySelector('input[name="baseAdd"]')?.value, 0)
    const effect = normalizeEffect({
      kind,
      targeting,
      ignoreUntargetable,
      onlyWhenNoTargetFromEffects,
      reuseTargetsFromEffect,
      damageType: selectedDamageType,
      amountFormula: { baseAdd: amountBaseAdd, terms: collectTerms(card.querySelector('[data-term-list="effect"]') || card) },
      ...(kind === "heal" ? {
        mpCost: {
          baseAdd: asNumber(card.querySelector('input[name="mpCostBaseAdd"]')?.value, 0),
          terms: collectTerms(card.querySelector('[data-term-list="mpCost"]') || card)
        }
      } : {})
    })
    effect.damageType = selectedDamageType
    return effect
  })
}

function collectBattleHooks() {
  const hookCards = Array.from(editorContent.querySelectorAll("[data-battle-hook-card]"))
  return normalizeBattleHooks(hookCards.map((card) => {
    const trigger = card.querySelector('[data-battle-hook-field="trigger"]')?.value || "on_after_take_damage"
    const priority = Math.trunc(asNumber(card.querySelector('[data-battle-hook-field="priority"]')?.value, 0))
    const conditions = Array.from(card.querySelectorAll("[data-battle-hook-condition-row]")).map((row) => {
      const kind = row.querySelector('[data-battle-hook-condition-field="kind"]')?.value || "incoming_damage_gte"
      const value = asNumber(row.querySelector('[data-battle-hook-condition-field="value"]')?.value, 0)
      const buffId = row.querySelector('[data-battle-hook-condition-field="buffId"]')?.value.trim() || ""
      return { kind, value, buffId }
    })
    const effects = Array.from(card.querySelectorAll("[data-battle-hook-effect-entry]")).map((entry) => {
      const buffId = entry.querySelector('[data-battle-hook-effect-field="buffId"]')?.value.trim() || ""
      const durationTurns = Math.max(0, Math.trunc(asNumber(entry.querySelector('[data-battle-hook-effect-field="durationTurns"]')?.value, 1)))
      const targetModeRaw = entry.querySelector('[data-battle-hook-effect-field="targetMode"]')?.value || "self"
      const targetMode = targetModeRaw === "source" || targetModeRaw === "targeting" ? targetModeRaw : "self"
      const targetPositions = Array.from(entry.querySelectorAll('[data-battle-hook-target-position]:checked')).map((item) => item.value)
      const secondarySeek = entry.querySelector('[data-battle-hook-targeting-field="secondarySeek"]')?.value || "front_to_rear"
      const targetScope = entry.querySelector('[data-battle-hook-targeting-field="targetScope"]')?.value || "enemy"
      const selectRule = entry.querySelector('[data-battle-hook-targeting-field="selectRule"]')?.value || "first"
      const selectAttrKey = entry.querySelector('[data-battle-hook-targeting-field="selectAttrKey"]')?.value.trim() || ""
      const targetCountBase = Math.max(1, Math.trunc(asNumber(entry.querySelector('[data-battle-hook-targeting-field="targetCountBase"]')?.value, 1)))
      const fallbackSelfIfEmpty = Boolean(entry.querySelector('[data-battle-hook-targeting-field="fallbackSelfIfEmpty"]')?.checked)
      const targetCountFormula = Array.from(entry.querySelectorAll("[data-battle-hook-target-count-formula-row]")).map((termRow) => ({
        attrKey: termRow.querySelector('[data-battle-hook-target-count-formula-field="attrKey"]')?.value.trim() || "",
        coefficient: asNumber(termRow.querySelector('[data-battle-hook-target-count-formula-field="coefficient"]')?.value, 0),
        perSkillLevel: asNumber(termRow.querySelector('[data-battle-hook-target-count-formula-field="perSkillLevel"]')?.value, 0)
      })).filter((term) => term.attrKey)
      return {
        kind: "add_buff",
        buffId,
        durationTurns,
        targetMode,
        targeting: normalizeTargeting({
          targetPositions,
          secondarySeek,
          targetScope,
          selectRule,
          selectAttrKey,
          fallbackSelfIfEmpty,
          targetCount: { base: targetCountBase, formula: targetCountFormula }
        })
      }
    })
    return {
      trigger,
      priority,
      effects,
      conditions
    }
  }))
}

function validateTargeting(targeting, path) {
  const errors = []
  if (!targeting || typeof targeting !== "object") {
    errors.push(`${path} 必须是对象`)
    return errors
  }
  if (!Array.isArray(targeting.targetPositions) || targeting.targetPositions.length === 0) errors.push(`${path}.targetPositions 至少选择一个站位`)
  if (!Number.isFinite(targeting.targetCount?.base) || targeting.targetCount.base < 1) errors.push(`${path}.targetCount.base 必须 >= 1`)
  if (!Number.isInteger(targeting.targetCount?.base)) errors.push(`${path}.targetCount.base 必须是整数`)
  if ((targeting.selectRule === "attr_max" || targeting.selectRule === "attr_min") && !asString(targeting.selectAttrKey).trim()) {
    errors.push(`${path}.selectAttrKey 在 attr_* 规则下必填`)
  } else if ((targeting.selectRule === "attr_max" || targeting.selectRule === "attr_min") && !ATTR_KEY_SET.has(asString(targeting.selectAttrKey).trim())) {
    errors.push(`${path}.selectAttrKey 必须是已定义属性`)
  }
  return errors
}

function validateFormula(formula, path, errors) {
  if (!formula || typeof formula !== "object") {
    errors.push(`${path} 必须是对象`)
    return
  }
  if (!Number.isFinite(formula.baseAdd)) errors.push(`${path}.baseAdd 必须是数字`)
  if (!Array.isArray(formula.terms) || formula.terms.length === 0) {
    errors.push(`${path}.terms 至少一项`)
    return
  }
  formula.terms.forEach((term, index) => {
    if (!term || typeof term !== "object") {
      errors.push(`${path}.terms[${index}] 必须是对象`)
      return
    }
    if (!asString(term.attrKey).trim()) errors.push(`${path}.terms[${index}].attrKey 必填`)
    else if (!ATTR_KEY_SET.has(asString(term.attrKey).trim())) errors.push(`${path}.terms[${index}].attrKey 必须是已定义属性`)
    if (!Number.isFinite(term.coefficient)) errors.push(`${path}.terms[${index}].coefficient 必须是数字`)
    if (!Number.isFinite(term.perSkillLevel)) errors.push(`${path}.terms[${index}].perSkillLevel 必须是数字`)
  })
}

function validateOptionalFormula(formula, path, errors) {
  if (formula == null) return
  if (!formula || typeof formula !== "object" || Array.isArray(formula)) {
    errors.push(`${path} 必须是对象`)
    return
  }
  if (formula.baseAdd != null && !Number.isFinite(formula.baseAdd)) errors.push(`${path}.baseAdd 必须是数字`)
  if (formula.terms == null) return
  if (!Array.isArray(formula.terms)) {
    errors.push(`${path}.terms 必须是数组`)
    return
  }
  formula.terms.forEach((term, index) => {
    if (!term || typeof term !== "object") {
      errors.push(`${path}.terms[${index}] 必须是对象`)
      return
    }
    if (!asString(term.attrKey).trim()) errors.push(`${path}.terms[${index}].attrKey 必填`)
    else if (!ATTR_KEY_SET.has(asString(term.attrKey).trim())) errors.push(`${path}.terms[${index}].attrKey 必须是已定义属性`)
    if (!Number.isFinite(term.coefficient)) errors.push(`${path}.terms[${index}].coefficient 必须是数字`)
    if (!Number.isFinite(term.perSkillLevel)) errors.push(`${path}.terms[${index}].perSkillLevel 必须是数字`)
  })
}

function validateDamageSpec(spec, path, errors) {
  if (!spec || typeof spec !== "object") {
    errors.push(`${path} 必须是对象`)
    return
  }
  if (spec.damageType !== "physical" && spec.damageType !== "magical") errors.push(`${path}.damageType 无效`)
  validateFormula(spec.amountFormula, `${path}.amountFormula`, errors)
  validateOptionalFormula(spec.flatDefensePenetrationFormula, `${path}.flatDefensePenetrationFormula`, errors)
  validateOptionalFormula(spec.percentDefensePenetrationFormula, `${path}.percentDefensePenetrationFormula`, errors)
  validateOptionalFormula(spec.physicalFlatDefensePenetrationFormula, `${path}.physicalFlatDefensePenetrationFormula`, errors)
  validateOptionalFormula(spec.physicalPercentDefensePenetrationFormula, `${path}.physicalPercentDefensePenetrationFormula`, errors)
  validateOptionalFormula(spec.magicalFlatDefensePenetrationFormula, `${path}.magicalFlatDefensePenetrationFormula`, errors)
  validateOptionalFormula(spec.magicalPercentDefensePenetrationFormula, `${path}.magicalPercentDefensePenetrationFormula`, errors)
  validateOptionalFormula(spec.extraDamageAmplificationFormula, `${path}.extraDamageAmplificationFormula`, errors)
  if (spec.bonusAgainstTargetTypes != null) {
    if (!spec.bonusAgainstTargetTypes || typeof spec.bonusAgainstTargetTypes !== "object" || Array.isArray(spec.bonusAgainstTargetTypes)) {
      errors.push(`${path}.bonusAgainstTargetTypes 必须是对象`)
    } else {
      for (const [key, value] of Object.entries(spec.bonusAgainstTargetTypes)) {
        if (!asString(key).trim() || !Number.isFinite(value)) errors.push(`${path}.bonusAgainstTargetTypes.${key} 必须是数字`)
      }
    }
  }
  if (spec.hitCount != null) {
    if (!spec.hitCount || typeof spec.hitCount !== "object" || Array.isArray(spec.hitCount)) {
      errors.push(`${path}.hitCount 必须是对象`)
    } else {
      if (spec.hitCount.base != null && (!Number.isFinite(spec.hitCount.base) || !Number.isInteger(spec.hitCount.base) || spec.hitCount.base < 0)) {
        errors.push(`${path}.hitCount.base 必须是 >= 0 的整数`)
      }
      validateOptionalFormula({ baseAdd: 0, terms: spec.hitCount.formula || [] }, `${path}.hitCount.formula`, errors)
    }
  }
}

function validateWeaponDraft(draft) {
  const errors = []
  if (!asString(draft.id).trim()) errors.push("id 必填")
  if (draft.equipType !== "battle" && draft.equipType !== "work") errors.push("equipType 无效")
  if (!asString(draft.slot).trim()) errors.push("slot 必填")
  if (!Array.isArray(draft.occupySlots) || draft.occupySlots.some((item) => typeof item !== "string" || item.trim() === "")) errors.push("occupySlots 必须是非空字符串数组")
  if (!Array.isArray(draft.weaponClass) || draft.weaponClass.some((item) => typeof item !== "string" || item.trim() === "")) errors.push("weaponClass 必须是非空字符串数组")
  if (!draft.statusRequirement || typeof draft.statusRequirement !== "object" || Array.isArray(draft.statusRequirement)) errors.push("statusRequirement 必须是对象")
  else {
    for (const [key, value] of Object.entries(draft.statusRequirement)) {
      if (!asString(key).trim() || !Number.isFinite(value) || value < 0) errors.push(`statusRequirement.${key} 必须是 >= 0 的数字`)
    }
  }
  if (!asString(draft.accessoryGroup) && draft.accessoryGroup !== "") errors.push("accessoryGroup 必须是字符串")
  if (!Number.isFinite(draft.affixSlotCount) || draft.affixSlotCount < 0 || !Number.isInteger(draft.affixSlotCount)) errors.push("affixSlotCount 必须是 >= 0 的整数")
  if (!Array.isArray(draft.affixAllowedIds) || draft.affixAllowedIds.some((item) => typeof item !== "string" || item.trim() === "")) errors.push("affixAllowedIds 必须是字符串数组")
  if (!draft.baseAttrs || typeof draft.baseAttrs !== "object" || Array.isArray(draft.baseAttrs)) errors.push("baseAttrs 必须是对象")
  else {
    for (const [key, value] of Object.entries(draft.baseAttrs)) {
      if (!asString(key).trim() || !Number.isFinite(value)) errors.push(`baseAttrs.${key} 必须是数字`)
    }
  }
  if (!Number.isFinite(draft.maxDurability) || draft.maxDurability <= 0) errors.push("maxDurability 必须 > 0")
  if (!Number.isFinite(draft.decayRate) || draft.decayRate < 0) errors.push("decayRate 必须 >= 0")
  if (!Number.isFinite(draft.enhanceMaxLevel) || !Number.isInteger(draft.enhanceMaxLevel) || draft.enhanceMaxLevel < 0 || draft.enhanceMaxLevel > 1000) errors.push("enhanceMaxLevel 必须是 0~1000 的整数")
  if (!Number.isFinite(draft.enhanceDurationS) || !Number.isInteger(draft.enhanceDurationS) || draft.enhanceDurationS <= 0) errors.push("enhanceDurationS 必须是正整数")
  if (!Number.isFinite(draft.enhanceExp) || draft.enhanceExp < 0) errors.push("enhanceExp 必须 >= 0")
  if (!Array.isArray(draft.enhanceProtectItemId) || draft.enhanceProtectItemId.some((item) => typeof item !== "string" || item.trim() === "")) errors.push("enhanceProtectItemId 必须是字符串数组")
  if (!draft.enhanceCostCurve || typeof draft.enhanceCostCurve !== "object" || Array.isArray(draft.enhanceCostCurve)) errors.push("enhanceCostCurve 必须是对象")
  else {
    if (!draft.enhanceCostCurve.base || typeof draft.enhanceCostCurve.base !== "object" || Array.isArray(draft.enhanceCostCurve.base)) errors.push("enhanceCostCurve.base 必须是对象")
    if (!draft.enhanceCostCurve.step || typeof draft.enhanceCostCurve.step !== "object" || Array.isArray(draft.enhanceCostCurve.step)) errors.push("enhanceCostCurve.step 必须是对象")
    if (!draft.enhanceCostCurve.overrides || typeof draft.enhanceCostCurve.overrides !== "object" || Array.isArray(draft.enhanceCostCurve.overrides)) errors.push("enhanceCostCurve.overrides 必须是对象")
  }
  if (!draft.enhanceAttrCurve || typeof draft.enhanceAttrCurve !== "object" || Array.isArray(draft.enhanceAttrCurve)) errors.push("enhanceAttrCurve 必须是对象")
  else {
    if (!draft.enhanceAttrCurve.base || typeof draft.enhanceAttrCurve.base !== "object" || Array.isArray(draft.enhanceAttrCurve.base)) errors.push("enhanceAttrCurve.base 必须是对象")
    if (!draft.enhanceAttrCurve.step || typeof draft.enhanceAttrCurve.step !== "object" || Array.isArray(draft.enhanceAttrCurve.step)) errors.push("enhanceAttrCurve.step 必须是对象")
    if (!draft.enhanceAttrCurve.overrides || typeof draft.enhanceAttrCurve.overrides !== "object" || Array.isArray(draft.enhanceAttrCurve.overrides)) errors.push("enhanceAttrCurve.overrides 必须是对象")
  }
  if (!Array.isArray(draft.battleHooks)) {
    errors.push("battleHooks 必须是数组")
  } else {
    draft.battleHooks.forEach((hook, index) => {
      const label = `battleHooks[${index}]`
      if (!hook || typeof hook !== "object" || Array.isArray(hook)) {
        errors.push(`${label} 必须是对象`)
        return
      }
      if (asString(hook.trigger).trim() !== "on_after_take_damage") {
        errors.push(`${label}.trigger 仅支持 on_after_take_damage`)
      }
      if (!Array.isArray(hook.effects) || hook.effects.length === 0) {
        errors.push(`${label}.effects 至少 1 项`)
        return
      }
      hook.effects.forEach((effect, effectIndex) => {
        const effectLabel = `${label}.effects[${effectIndex}]`
        if (!effect || typeof effect !== "object" || Array.isArray(effect)) {
          errors.push(`${effectLabel} 必须是对象`)
          return
        }
        const kind = asString(effect.kind || effect.type).trim().toLowerCase()
        if (kind !== "add_buff" && kind !== "addbuff") {
          errors.push(`${effectLabel}.kind 仅支持 add_buff`)
          return
        }
        if (!asString(effect.buffId).trim()) {
          errors.push(`${effectLabel}.buffId 必填`)
        }
        const targetMode = asString(effect.targetMode || effect.target).trim().toLowerCase() || "self"
        if (!["self", "source", "targeting"].includes(targetMode)) {
          errors.push(`${effectLabel}.targetMode 无效`)
        }
        if (targetMode === "targeting") {
          errors.push(...validateTargeting(effect.targeting, `${effectLabel}.targeting`))
        }
      })
    })
  }
  if (!Array.isArray(draft.basicAttackEffects) || draft.basicAttackEffects.length === 0) {
    errors.push("basicAttackEffects 至少保留一个效果")
  } else {
    draft.basicAttackEffects.forEach((effect, index) => {
      const effectIndex = index + 1
      const label = `basicAttackEffects[${index}]`
      if (!effect || typeof effect !== "object" || Array.isArray(effect)) {
        errors.push(`${label} 必须是对象`)
        return
      }
      if (!["damage", "heal", "status"].includes(effect.kind)) errors.push(`${label}.kind 无效`)
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
        if (!effect.statusPayload || typeof effect.statusPayload !== "object" || Array.isArray(effect.statusPayload)) {
          errors.push(`${label}.statusPayload 必须是对象`)
        } else {
          if (!Array.isArray(effect.statusPayload.statusIds) || effect.statusPayload.statusIds.some((item) => typeof item !== "string" || item.trim() === "")) errors.push(`${label}.statusPayload.statusIds 无效`)
          if (!Number.isFinite(effect.statusPayload.durationBaseTurns) || effect.statusPayload.durationBaseTurns < 0 || !Number.isInteger(effect.statusPayload.durationBaseTurns)) errors.push(`${label}.statusPayload.durationBaseTurns 无效`)
          if (!Number.isFinite(effect.statusPayload.durationPerSkillLevelTurns)) errors.push(`${label}.statusPayload.durationPerSkillLevelTurns 无效`)
        }
        return
      }
      if (!Number.isFinite(effect.amountFormula?.baseAdd)) errors.push(`${label}.amountFormula.baseAdd 必须是数字`)
      validateFormula(effect.amountFormula, `${label}.amountFormula`, errors)
      if (!["physical", "magical"].includes(effect.damageType)) {
        errors.push(`${label}.damageType 无效`)
      }
      if (effect.kind === "damage") {
        validateDamageSpec(effect.damageSpec, `${label}.damageSpec`, errors)
      }
    })
  }
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
  const errors = validateWeaponDraft(draft)
  renderEditorValidation(errors)
  return errors.length === 0
}

function refreshCurrentEditorValidation() {
  const rowId = editorModal.dataset.rowId || ""
  const row = rowId ? findRow(rowId) : null
  return refreshEditorValidation(row)
}

function draftToRowRecord(draft, rowId) {
  return {
    __id: rowId,
    id: draft.id,
    equipType: draft.equipType,
    slot: draft.slot,
    occupySlots: JSON.stringify(draft.occupySlots, null, 2),
    weaponClass: draft.weaponClass.join(", "),
    statusRequirement: JSON.stringify(draft.statusRequirement, null, 2),
    accessoryGroup: draft.accessoryGroup,
    affixSlotCount: draft.affixSlotCount,
    affixAllowedIds: JSON.stringify(draft.affixAllowedIds, null, 2),
    baseAttrs: JSON.stringify(draft.baseAttrs, null, 2),
    maxDurability: draft.maxDurability,
    decayRate: draft.decayRate,
    enhanceMaxLevel: draft.enhanceMaxLevel,
    enhanceDurationS: draft.enhanceDurationS,
    enhanceExp: draft.enhanceExp,
    enhanceProtectItemId: draft.enhanceProtectItemId.join(", "),
    enhanceCostCurve: JSON.stringify(draft.enhanceCostCurve, null, 2),
    enhanceAttrCurve: JSON.stringify(draft.enhanceAttrCurve, null, 2),
    battleHooks: JSON.stringify(draft.battleHooks, null, 2),
    basicAttackEffects: JSON.stringify(draft.basicAttackEffects, null, 2)
  }
}

function weaponDraftToJsonObject(draft) {
  return {
    id: draft.id,
    equipType: draft.equipType,
    slot: draft.slot,
    occupySlots: draft.occupySlots,
    weaponClass: draft.weaponClass,
    statusRequirement: draft.statusRequirement,
    accessoryGroup: draft.accessoryGroup,
    affixSlotCount: draft.affixSlotCount,
    affixAllowedIds: draft.affixAllowedIds,
    baseAttrs: draft.baseAttrs,
    maxDurability: draft.maxDurability,
    decayRate: draft.decayRate,
    enhanceMaxLevel: draft.enhanceMaxLevel,
    enhanceDurationS: draft.enhanceDurationS,
    enhanceExp: draft.enhanceExp,
    enhanceProtectItemId: draft.enhanceProtectItemId,
    enhanceCostCurve: draft.enhanceCostCurve,
    enhanceAttrCurve: draft.enhanceAttrCurve,
    battleHooks: draft.battleHooks,
    basicAttackEffects: draft.basicAttackEffects
  }
}

function saveEditor() {
  const rowId = editorModal.dataset.rowId
  if (!rowId) return
  const row = findRow(rowId)
  const draft = buildEditorDraft(row)
  const errors = validateWeaponDraft(draft)
  renderEditorValidation(errors)
  if (errors.length > 0) {
    setStatus(`保存失败：${errors[0]}`, true)
    return
  }
  const nextRow = draftToRowRecord(draft, rowId)
  if (row) Object.assign(row, nextRow)
  else rows.push(nextRow)
  renderBody()
  closeEditor()
  setStatus("武器已更新")
}

function addTermRow(scope, term = makeDefaultTerm(), root = editorContent) {
  const list = root.querySelector(`[data-term-list="${scope}"]`)
  if (!list) return
  list.insertAdjacentHTML("beforeend", renderFormulaRow(term, scope))
}

function parseDatasetOptions(raw) {
  if (!raw) return null
  const parsed = safeParseJson(raw, [])
  if (!Array.isArray(parsed)) return null
  const normalized = parsed
    .map((item) => {
      if (item && typeof item === "object" && !Array.isArray(item)) {
        const value = asString(item.value).trim()
        if (!value) return null
        return { value, label: asString(item.label || value).trim() || value }
      }
      const value = asString(item).trim()
      return value ? value : null
    })
    .filter(Boolean)
  return normalized.length ? normalized : null
}

function addMapRow(scope, row = { key: "", value: 0 }, root = editorContent) {
  const list = root.querySelector(`[data-map-list="${scope}"]`)
  if (!list) return
  const valueKind = list.dataset.mapValueKind === "text" ? "text" : "number"
  const keyOptions = parseDatasetOptions(list.dataset.mapKeyOptions)
  list.insertAdjacentHTML("beforeend", renderMapRow(row, scope, valueKind, keyOptions))
}

function addListRow(scope, value = "", root = editorContent) {
  const list = root.querySelector(`[data-list="${scope}"]`)
  if (!list) return
  const options = parseDatasetOptions(list.dataset.listOptions)
  list.insertAdjacentHTML("beforeend", renderListRow(value, scope, options))
}

function addCurveOverride(scope) {
  const list = editorContent.querySelector(`[data-override-list="${scope}"]`)
  if (!list) return
  list.insertAdjacentHTML("beforeend", renderCurveOverrideRow("1", {}, scope, "number"))
}

function addEffectCard(kind = "damage") {
  const list = editorContent.querySelector(".effect-list")
  if (!list) return
  const index = list.querySelectorAll("[data-effect-card]").length
  list.insertAdjacentHTML("beforeend", renderEffectCard(makeDefaultEffect(kind), index))
}

function moveEffectCard(card, direction) {
  const sibling = direction < 0 ? card.previousElementSibling : card.nextElementSibling
  if (!sibling || !sibling.matches("[data-effect-card]")) return
  if (direction < 0) card.parentElement.insertBefore(card, sibling)
  else card.parentElement.insertBefore(sibling, card)
}

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
    copyCurrentWeaponJsonText().catch((err) => setStatus(err instanceof Error ? err.message : "复制失败", true))
    return
  }
  if (action === "target-positions-all") {
    editorContent.querySelectorAll('input[name="targetPositions"]').forEach((item) => {
      item.checked = true
    })
    refreshCurrentEditorValidation()
    return
  }
  if (action === "target-positions-clear") {
    editorContent.querySelectorAll('input[name="targetPositions"]').forEach((item) => {
      item.checked = false
    })
    refreshCurrentEditorValidation()
    return
  }
  if (action === "add-term") {
    addTermRow(target.dataset.termScope || "basicFormula")
    refreshCurrentEditorValidation()
    return
  }
  if (action === "remove-term") {
    target.closest("[data-term-row]")?.remove()
    refreshCurrentEditorValidation()
    return
  }
  if (action === "add-map-row") {
    addMapRow(target.dataset.mapScope || "enhanceCostCurve-base")
    refreshCurrentEditorValidation()
    return
  }
  if (action === "remove-map-row") {
    target.closest("[data-map-row]")?.remove()
    refreshCurrentEditorValidation()
    return
  }
  if (action === "add-list-row") {
    addListRow(target.dataset.listScope || "basicList")
    refreshCurrentEditorValidation()
    return
  }
  if (action === "remove-list-row") {
    target.closest("[data-list-row]")?.remove()
    refreshCurrentEditorValidation()
    return
  }
  if (action === "add-curve-override") {
    addCurveOverride(target.dataset.curveScope || "enhanceCostCurve")
    refreshCurrentEditorValidation()
    return
  }
  if (action === "remove-curve-override") {
    target.closest("[data-curve-override-row]")?.remove()
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
    const list = target.closest("[data-effect-card]")?.querySelector("[data-status-list]")
    if (list) list.insertAdjacentHTML("beforeend", `
      <div class="repeat-row" data-status-row>
        <label class="cell-stack">状态 ID
          <input type="text" data-status-field="statusId" placeholder="status id" value="" />
        </label>
        <button type="button" data-action="remove-status-id">删</button>
      </div>
    `)
    refreshCurrentEditorValidation()
    return
  }
  if (action === "remove-status-id") {
    target.closest("[data-status-row]")?.remove()
    refreshCurrentEditorValidation()
    return
  }
  if (action === "add-battle-hook") {
    const list = editorContent.querySelector("[data-battle-hook-list]")
    if (list) list.insertAdjacentHTML("beforeend", renderBattleHookCard(makeDefaultBattleHook(), list.querySelectorAll("[data-battle-hook-card]").length))
    refreshCurrentEditorValidation()
    return
  }
  if (action === "remove-battle-hook") {
    target.closest("[data-battle-hook-card]")?.remove()
    refreshCurrentEditorValidation()
    return
  }
  if (action === "move-battle-hook-up") {
    const card = target.closest("[data-battle-hook-card]")
    if (card) moveEffectCard(card, -1)
    refreshCurrentEditorValidation()
    return
  }
  if (action === "move-battle-hook-down") {
    const card = target.closest("[data-battle-hook-card]")
    if (card) moveEffectCard(card, 1)
    refreshCurrentEditorValidation()
    return
  }
  if (action === "add-battle-hook-effect") {
    const list = target.closest("[data-battle-hook-card]")?.querySelector("[data-battle-hook-effect-list]")
    if (list) list.insertAdjacentHTML("beforeend", renderBattleHookEffectRow(makeDefaultBattleHookEffect()))
    refreshCurrentEditorValidation()
    return
  }
  if (action === "remove-battle-hook-effect") {
    target.closest("[data-battle-hook-effect-entry]")?.remove()
    refreshCurrentEditorValidation()
    return
  }
  if (action === "add-battle-hook-condition") {
    const list = target.closest("[data-battle-hook-card]")?.querySelector("[data-battle-hook-condition-list]")
    if (list) list.insertAdjacentHTML("beforeend", renderBattleHookConditionRow(makeDefaultBattleHookCondition()))
    refreshCurrentEditorValidation()
    return
  }
  if (action === "remove-battle-hook-condition") {
    target.closest("[data-battle-hook-condition-row]")?.remove()
    refreshCurrentEditorValidation()
    return
  }
  if (action === "add-battle-hook-target-count-term") {
    const list = target.closest("[data-battle-hook-effect-entry]")?.querySelector("[data-battle-hook-target-count-formula-list]")
    if (list) list.insertAdjacentHTML("beforeend", renderBattleHookTargetCountFormulaRow(makeDefaultTerm()))
    refreshCurrentEditorValidation()
    return
  }
  if (action === "remove-battle-hook-target-count-term") {
    target.closest("[data-battle-hook-target-count-formula-row]")?.remove()
    refreshCurrentEditorValidation()
  }
})

editorModal.addEventListener("change", (event) => {
  const target = event.target
  if (target instanceof HTMLSelectElement && target.name === "effectKind") {
    const card = target.closest("[data-effect-card]")
    if (card) card.dataset.effectKind = target.value
  }
  refreshCurrentEditorValidation()
})

editorModal.addEventListener("input", () => {
  refreshCurrentEditorValidation()
})

copyBtn.addEventListener("click", () => {
  copyTsv().catch((err) => setStatus(err instanceof Error ? err.message : "复制失败", true))
})

downloadBtn.addEventListener("click", downloadTsv)

templateSelect.addEventListener("change", () => {
  const key = templateSelect.value
  if (!key) return
  loadTemplate(key)
})

addRowBtn.addEventListener("click", () => {
  const key = templateSelect.value || "sword"
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

importJsonBtn.addEventListener("click", () => {
  const row = createRowFromTemplate(templateSelect.value || "sword")
  row.__id = `json-import-${uid()}`
  openEditorWithRow(row, "JSON 导入 · 武器编辑")
  const textarea = getEditorJsonTextarea()
  if (textarea) textarea.value = ""
})

formatJsonBtn.addEventListener("click", formatJsonRows)
resetBtn.addEventListener("click", resetSampleRows)
editorCloseBtn.addEventListener("click", closeEditor)
editorSaveBtn.addEventListener("click", saveEditor)

render()
resetSampleRows()
