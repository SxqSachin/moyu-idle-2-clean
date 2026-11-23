import fs from "node:fs"
import path from "node:path"
import { execFileSync } from "node:child_process"
import { fileURLToPath } from "node:url"

const scriptDir = path.dirname(fileURLToPath(import.meta.url))
const projectRoot = path.resolve(scriptDir, "../../..")
const gameDataPath = path.join(projectRoot, "packages/game-data/generated/game-data.json")
const updateLogPath = path.join(projectRoot, "web/src/data/update-log.ts")
const sourceEntriesMarker = "const sourceEntries: UpdateLogEntry[] = ["
const defaultTodoLine = "TODO: translate zh-CN update log generated from game-data diff"
const datePattern = /^\d{4}-\d{2}-\d{2}$/

const quickRefTables = new Map([
  ["items", "item"],
  ["equipment_templates", "item"],
  ["skills", "skill"],
  ["affix_definitions", "affix"],
  ["enemies", "enemy"],
  ["attr", "attr"]
])

const tableOrder = [
  "items",
  "equipment_templates",
  "skills",
  "affix_definitions",
  "affix_enchant_rules",
  "affix_carrier_rules",
  "buff",
  "buff_effects",
  "enemies",
  "world"
]

const genericFieldLabels = new Map([
  ["id", "ID"],
  ["nameKey", "名称"],
  ["descKey", "描述"],
  ["effectDescKey", "效果描述"],
  ["iconId", "图标"],
  ["emojiIcon", "图标"],
  ["rarity", "稀有度"],
  ["type", "类型"],
  ["category", "分类"],
  ["active", "启用状态"],
  ["order", "排序"],
  ["color", "颜色"],
  ["level", "等级"],
  ["position", "站位"],
  ["drops", "掉落"],
  ["skillIds", "技能"],
  ["enemyIds", "普通敌人池"],
  ["specialEnemyIds", "特殊敌人池"],
  ["customLineupConf", "自定义阵容"],
  ["specialWaves", "特殊波次"],
  ["normalEnemyCount", "普通敌人数"],
  ["entryCosts", "进入消耗"]
])

const tableFieldLabels = {
  items: [
    [["rarity"], "稀有度"],
    [["type"], "类型"],
    [["economyBaseValue"], "价值"],
    [["stackable"], "堆叠规则"],
    [["carryableInCombat"], "战斗携带"],
    [["marketListable"], "市场交易"],
    [["useGrantSkillId"], "使用授予技能"],
    [["useSkillExpGain"], "使用技能经验"],
    [["nameKey"], "名称"],
    [["descKey"], "描述"],
    [["effectDescKey"], "效果描述"]
  ],
  equipment_templates: [
    [["baseAttrs"], "基础属性"],
    [["enhanceAttrCurve"], "强化属性曲线"],
    [["enhanceCostCurve"], "强化消耗"],
    [["basicAttackEffects"], "普攻效果"],
    [["defaultAffixSlots"], "默认词条"],
    [["affixSlotCount"], "词条槽"],
    [["affixAllowedIds"], "词条白名单"],
    [["equipType"], "装备类型"],
    [["slot"], "装备部位"],
    [["occupySlots"], "占用槽位"],
    [["weaponClass"], "武器类型"],
    [["statusRequirement"], "装备需求"],
    [["battleHooks"], "战斗钩子"],
    [["maxDurability"], "耐久"],
    [["decayRate"], "耐久衰减"],
    [["enhanceMaxLevel"], "强化上限"]
  ],
  skills: [
    [["targeting", "targetCount", "max"], "目标数上限"],
    [["targeting", "targetCount"], "目标数量"],
    [["targeting"], "目标配置"],
    [["effects"], "效果列表"],
    [["effects", "*", "amountFormula"], "数值公式"],
    [["effects", "*", "damageSpec"], "伤害配置"],
    [["effects", "*", "targeting", "targetCount", "max"], "目标数上限"],
    [["effects", "*", "targeting"], "效果目标配置"],
    [["hpCost"], "HP消耗"],
    [["mpCost"], "MP消耗"],
    [["cooldown"], "冷却"],
    [["statusRequirement"], "属性需求"],
    [["weaponRequirement"], "武器需求"],
    [["category"], "技能类型"]
  ],
  affix_definitions: [
    [["allowedRarities"], "可用稀有度"],
    [["effectParams", "attrMods"], "属性范围"],
    [["effectParams", "effectMutations"], "效果变更"],
    [["effectParams", "battleHooks"], "战斗钩子"],
    [["manualScore"], "手动评分"],
    [["maxEffectivePerEquipment"], "单装备上限"],
    [["maxEffectivePerBody"], "最多同时生效"],
    [["effectType"], "效果类型"],
    [["type"], "装备类型约束"],
    [["valuePrecision"], "数值精度"]
  ],
  enemies: [
    [["skillIds"], "技能"],
    [["equipment"], "装备"],
    [["drops"], "掉落"],
    [["type"], "敌人类型"],
    [["level"], "等级"],
    [["position"], "站位"],
    [["exp"], "经验"]
  ],
  world: [
    [["enemyIds"], "普通敌人池"],
    [["specialEnemyIds"], "特殊敌人池"],
    [["customLineupConf"], "自定义阵容"],
    [["specialWaves"], "特殊波次"],
    [["normalEnemyCount"], "普通敌人数"],
    [["entryCosts"], "进入消耗"]
  ]
}

const tableDisplayNames = new Map([
  ["items", "物品"],
  ["equipment_templates", "装备"],
  ["skills", "技能"],
  ["affix_definitions", "词条"],
  ["affix_enchant_rules", "附魔规则"],
  ["affix_carrier_rules", "词条载体规则"],
  ["buff", "Buff"],
  ["buff_effects", "Buff效果"],
  ["enemies", "敌人"],
  ["world", "区域"]
])

const targetSeekLabels = new Map([
  ["none", "不继续寻敌"],
  ["front_to_rear", "从前排向后排继续寻找目标"],
  ["rear_to_front", "从后排向前排继续寻找目标"],
  ["random_alive", "随机寻找存活目标"]
])

const targetPositionLabels = new Map([
  ["vanguard", "前排"],
  ["midguard", "中排"],
  ["rearguard", "后排"]
])

const attrLabels = new Map([
  ["hp", "生命值"],
  ["mp", "魔法值"],
  ["hp_regen", "生命恢复"],
  ["mp_regen", "魔法恢复"],
  ["strength", "力量"],
  ["dexterity", "敏捷"],
  ["intelligence", "智力"],
  ["stamina", "耐力"],
  ["luck", "幸运"],
  ["physics_damage", "物理伤害"],
  ["magical_damage", "魔法伤害"],
  ["physics_defence", "物理防御"],
  ["magical_defence", "魔法防御"],
  ["crit_chance", "暴击率"],
  ["crit_damage", "暴击伤害"],
  ["speed", "速度"],
  ["physics_damage_amplification", "物理增伤"],
  ["magical_damage_amplification", "魔法增伤"],
  ["physics_damage_resist", "物理减伤"],
  ["magical_damage_resist", "魔法减伤"],
  ["physics_flat_defense_penetration", "物理固定穿透"],
  ["magicial_flat_defense_penetration", "魔法固定穿透"],
  ["physics_percent_defense_penetration", "物理百分比穿透"],
  ["magicial_percent_defense_penetration", "魔法百分比穿透"],
  ["enhancement_success_rate", "强化成功率"]
])

function parseArgs(argv) {
  const args = {
    base: "HEAD",
    date: todayString(),
    id: "",
    dryRun: false,
    help: false
  }
  for (let index = 0; index < argv.length; index += 1) {
    const arg = argv[index]
    if (arg === "--base") {
      args.base = requireValue(argv, ++index, arg)
    } else if (arg === "--date") {
      args.date = requireValue(argv, ++index, arg)
    } else if (arg === "--id") {
      args.id = requireValue(argv, ++index, arg)
    } else if (arg === "--dry-run") {
      args.dryRun = true
    } else if (arg === "--help" || arg === "-h") {
      args.help = true
    } else {
      throw new Error(`unknown argument: ${arg}`)
    }
  }
  if (!args.id) args.id = args.date
  if (!datePattern.test(args.date)) {
    throw new Error(`--date must be YYYY-MM-DD, got: ${args.date}`)
  }
  if (!args.id.trim()) {
    throw new Error("--id must be non-empty")
  }
  return args
}

function requireValue(argv, index, flag) {
  const value = argv[index]
  if (!value || value.startsWith("--")) {
    throw new Error(`${flag} requires a value`)
  }
  return value
}

function todayString() {
  const now = new Date()
  const year = now.getFullYear()
  const month = String(now.getMonth() + 1).padStart(2, "0")
  const day = String(now.getDate()).padStart(2, "0")
  return `${year}-${month}-${day}`
}

function readJson(filePath) {
  return JSON.parse(fs.readFileSync(filePath, "utf8"))
}

function readBaseGameData(base) {
  const relativePath = path.relative(projectRoot, gameDataPath).replaceAll(path.sep, "/")
  const content = execFileSync("git", ["show", `${base}:${relativePath}`], {
    cwd: projectRoot,
    encoding: "utf8",
    maxBuffer: 1024 * 1024 * 200
  })
  return JSON.parse(content)
}

function rowId(row) {
  if (!row || typeof row !== "object" || Array.isArray(row)) return ""
  return String(row.id ?? "").trim()
}

function indexRows(rows) {
  const indexed = new Map()
  for (const row of Array.isArray(rows) ? rows : []) {
    const id = rowId(row)
    if (id) indexed.set(id, row)
  }
  return indexed
}

function isRecord(value) {
  return Boolean(value) && typeof value === "object" && !Array.isArray(value)
}

function normalizeScalar(value) {
  if (value === undefined) return null
  return value
}

function valuesEqual(left, right) {
  return JSON.stringify(normalizeScalar(left)) === JSON.stringify(normalizeScalar(right))
}

function diffValues(before, after, pathParts = []) {
  if (valuesEqual(before, after)) return []
  if (Array.isArray(before) || Array.isArray(after)) {
    const left = Array.isArray(before) ? before : []
    const right = Array.isArray(after) ? after : []
    const max = Math.max(left.length, right.length)
    const paths = []
    for (let index = 0; index < max; index += 1) {
      paths.push(...diffValues(left[index], right[index], [...pathParts, String(index)]))
    }
    return paths.length > 0 ? paths : [pathParts]
  }
  if (isRecord(before) || isRecord(after)) {
    const left = isRecord(before) ? before : {}
    const right = isRecord(after) ? after : {}
    const keys = Array.from(new Set([...Object.keys(left), ...Object.keys(right)])).sort()
    return keys.flatMap((key) => diffValues(left[key], right[key], [...pathParts, key]))
  }
  return [pathParts]
}

function comparePath(pattern, actual) {
  if (pattern.length > actual.length) return false
  for (let index = 0; index < pattern.length; index += 1) {
    if (pattern[index] === "*") continue
    if (pattern[index] !== actual[index]) return false
  }
  return true
}

function labelForPath(table, pathParts) {
  const labels = tableFieldLabels[table] || []
  const matched = labels
    .filter(([pattern]) => comparePath(pattern, pathParts))
    .sort((a, b) => b[0].length - a[0].length)[0]
  if (matched) return matched[1]
  const first = pathParts[0] || ""
  return genericFieldLabels.get(first) || first || "配置"
}

function summarizeChangedPaths(table, paths) {
  const labels = []
  const seen = new Set()
  for (const pathParts of paths) {
    const label = labelForPath(table, pathParts)
    if (!label || seen.has(label)) continue
    seen.add(label)
    labels.push(label)
  }
  return labels.slice(0, 8)
}

function pathStartsWith(pathParts, prefix) {
  if (prefix.length > pathParts.length) return false
  return prefix.every((part, index) => part === pathParts[index])
}

function pathHas(pathParts, key) {
  return pathParts.includes(key)
}

function getAtPath(value, pathParts) {
  let current = value
  for (const part of pathParts) {
    if (current === undefined || current === null) return undefined
    current = current[part]
  }
  return current
}

const percentPathWords = ["percent", "chance", "amplification", "resist"]
const percentAttrIDs = new Set([
  "crit_chance",
  "crit_damage",
  "physics_damage_amplification",
  "magical_damage_amplification",
  "physics_damage_resist",
  "magical_damage_resist",
  "physics_percent_defense_penetration",
  "magicial_percent_defense_penetration",
  "enhancement_success_rate"
])

function isPercentPath(pathParts = []) {
  return pathParts.some((part) => {
    const normalized = String(part || "").toLowerCase()
    return percentAttrIDs.has(normalized) || percentPathWords.some((word) => normalized.includes(word))
  })
}

function isTargetCountPath(pathParts = []) {
  return pathParts.includes("targetCount") && (pathParts.at(-1) === "base" || pathParts.at(-1) === "max")
}

function formatTargetCountValue(value) {
  const numeric = Number(value)
  if (Number.isFinite(numeric) && numeric >= 999) return "全部目标"
  return formatValue(value)
}

function formatPercentValue(value) {
  const numeric = Number(value)
  if (!Number.isFinite(numeric)) return formatValue(value)
  const percent = Math.round(numeric * 1000000) / 10000
  return `${String(percent).replace(/\.?0+$/, "")}%`
}

function getEffect(row, rootKey, pathParts) {
  const index = Number(pathParts[1])
  if (!Number.isInteger(index)) return null
  const effects = Array.isArray(row?.[rootKey]) ? row[rootKey] : []
  return effects[index] && typeof effects[index] === "object" ? effects[index] : null
}

function formatValue(value, pathParts = []) {
  if (typeof value === "boolean") return value ? "开启" : "关闭"
  if (typeof value === "number" && Number.isFinite(value)) {
    if (isTargetCountPath(pathParts)) return formatTargetCountValue(value)
    if (isPercentPath(pathParts)) return formatPercentValue(value)
    const rounded = Math.round(value * 10000) / 10000
    if (Object.is(rounded, 0)) return "0"
    return String(rounded).replace(/\.?0+$/, "")
  }
  if (value === undefined || value === null || value === "") return "未设置"
  if (Array.isArray(value)) return value.map((entry) => formatValue(entry, pathParts)).join("、") || "未设置"
  if (typeof value === "object") return "配置项"
  return String(value)
}

function isUnset(value) {
  return value === undefined || value === null || value === ""
}

function describeScalarChange(label, before, after, pathParts = []) {
  if (isUnset(before) && !isUnset(after)) return `${label}新增为 ${formatValue(after, pathParts)}`
  if (!isUnset(before) && isUnset(after)) return `${label}移除`
  return `${label}从 ${formatValue(before, pathParts)} 调整为 ${formatValue(after, pathParts)}`
}

function describeSeekChange(before, after, label = "寻敌方式") {
  if (String(before ?? "") === "none" && String(after ?? "") === "front_to_rear") {
    return `${label}改为前排没有目标时，会继续向后排寻找`
  }
  if (String(before ?? "") === "none" && String(after ?? "") === "rear_to_front") {
    return `${label}改为后排没有目标时，会继续向前排寻找`
  }
  if (String(after ?? "") === "none") {
    return `${label}改为只检查当前目标范围`
  }
  const left = targetSeekLabels.get(String(before ?? "")) || formatValue(before)
  const right = targetSeekLabels.get(String(after ?? "")) || formatValue(after)
  return `${label}从“${left}”调整为“${right}”`
}

function describePositionChange(beforePositions, afterPositions, label = "可攻击位置") {
  const beforeSet = new Set((Array.isArray(beforePositions) ? beforePositions : []).map(String))
  const afterSet = new Set((Array.isArray(afterPositions) ? afterPositions : []).map(String))
  const added = [...afterSet].filter((value) => !beforeSet.has(value))
  const removed = [...beforeSet].filter((value) => !afterSet.has(value))
  const parts = []
  if (added.length > 0) parts.push(`新增${added.map((value) => targetPositionLabels.get(value) || value).join("、")}目标`)
  if (removed.length > 0) parts.push(`移除${removed.map((value) => targetPositionLabels.get(value) || value).join("、")}目标`)
  if (parts.length > 0) return `${label}${parts.join("，")}`
  return `${label}调整为${[...afterSet].map((value) => targetPositionLabels.get(value) || value).join("、") || "未设置"}`
}

function describeFormulaTerm(pathParts, before, after, labelPrefix = "伤害系数") {
  const termPath = pathParts.slice(0, pathParts.indexOf("terms") + 2)
  const beforeTerm = getAtPath(before, termPath)
  const afterTerm = getAtPath(after, termPath)
  const attrKey = String(afterTerm?.attrKey || beforeTerm?.attrKey || "")
  const attrLabel = attrLabels.get(attrKey) || attrKey || "属性"
  if (pathParts.at(-1) === "coefficient") {
    if (isUnset(getAtPath(before, pathParts))) return `${labelPrefix}中新增${attrLabel}系数 ${formatValue(getAtPath(after, pathParts))}`
    return `${labelPrefix}中${attrLabel}系数从 ${formatValue(getAtPath(before, pathParts))} 调整为 ${formatValue(getAtPath(after, pathParts))}`
  }
  if (pathParts.at(-1) === "perSkillLevel") {
    if (isUnset(getAtPath(before, pathParts)) && Number(getAtPath(after, pathParts)) === 0) return ""
    if (isUnset(getAtPath(before, pathParts))) return `${labelPrefix}中新增${attrLabel}的每级成长 ${formatValue(getAtPath(after, pathParts))}`
    return `${labelPrefix}中${attrLabel}的每级成长从 ${formatValue(getAtPath(before, pathParts))} 调整为 ${formatValue(getAtPath(after, pathParts))}`
  }
  if (pathParts.at(-1) === "attrKey") return ""
  return `${labelPrefix}调整`
}

function describeFormulaArrayTerm(pathParts, before, after, formulaKey, labelPrefix) {
  const formulaIndex = pathParts.indexOf(formulaKey)
  const termPath = pathParts.slice(0, formulaIndex + 2)
  const beforeTerm = getAtPath(before, termPath)
  const afterTerm = getAtPath(after, termPath)
  const attrKey = String(afterTerm?.attrKey || beforeTerm?.attrKey || "")
  const attrLabel = attrLabels.get(attrKey) || attrKey || "属性"
  if (pathParts.at(-1) === "coefficient") {
    if (isUnset(getAtPath(before, pathParts))) return `${labelPrefix}中新增${attrLabel}系数 ${formatValue(getAtPath(after, pathParts))}`
    return `${labelPrefix}中${attrLabel}系数从 ${formatValue(getAtPath(before, pathParts))} 调整为 ${formatValue(getAtPath(after, pathParts))}`
  }
  if (pathParts.at(-1) === "perSkillLevel") {
    if (isUnset(getAtPath(before, pathParts)) && Number(getAtPath(after, pathParts)) === 0) return ""
    if (isUnset(getAtPath(before, pathParts))) return `${labelPrefix}中新增${attrLabel}的每级成长 ${formatValue(getAtPath(after, pathParts))}`
    return `${labelPrefix}中${attrLabel}的每级成长从 ${formatValue(getAtPath(before, pathParts))} 调整为 ${formatValue(getAtPath(after, pathParts))}`
  }
  if (pathParts.at(-1) === "attrKey") return ""
  return `${labelPrefix}调整`
}

function describeTargetCountChange(pathParts, beforeRoot, afterRoot, labelPrefix = "目标数量") {
  if (pathParts.at(-1) === "base") {
    return `${labelPrefix}从 ${formatValue(getAtPath(beforeRoot, pathParts), pathParts)} 调整为 ${formatValue(getAtPath(afterRoot, pathParts), pathParts)}`
  }
  if (pathParts.at(-1) === "max") {
    if (isUnset(getAtPath(beforeRoot, pathParts))) {
      return `${labelPrefix}上限新增为 ${formatValue(getAtPath(afterRoot, pathParts), pathParts)}`
    }
    return `${labelPrefix}上限从 ${formatValue(getAtPath(beforeRoot, pathParts), pathParts)} 调整为 ${formatValue(getAtPath(afterRoot, pathParts), pathParts)}`
  }
  if (pathParts.includes("terms")) {
    return describeFormulaTerm(pathParts, beforeRoot, afterRoot, `${labelPrefix}公式`)
  }
  if (pathParts.includes("formula")) {
    return describeFormulaArrayTerm(pathParts, beforeRoot, afterRoot, "formula", `${labelPrefix}公式`)
  }
  return `${labelPrefix}的公式调整`
}

function describeEffectPath(table, rootKey, pathParts, before, after) {
  const beforeEffect = getEffect(before, rootKey, pathParts)
  const afterEffect = getEffect(after, rootKey, pathParts)
  const effectNo = Number(pathParts[1]) + 1
  const prefix = rootKey === "basicAttackEffects" ? "普通攻击" : `第 ${effectNo} 个效果`
  const relativePath = pathParts.slice(2)
  if (!beforeEffect && afterEffect) return `新增${prefix}`
  if (beforeEffect && !afterEffect) return `移除${prefix}`
  if (pathHas(relativePath, "secondarySeek")) {
    return describeSeekChange(getAtPath(beforeEffect, relativePath), getAtPath(afterEffect, relativePath), `${prefix}的寻敌方式`)
  }
  if (pathHas(relativePath, "targetPositions")) {
    return describePositionChange(beforeEffect?.targeting?.targetPositions, afterEffect?.targeting?.targetPositions, `${prefix}的目标范围`)
  }
  if (pathStartsWith(relativePath, ["targeting", "targetCount"])) {
    return describeTargetCountChange(relativePath, beforeEffect, afterEffect, `${prefix}的目标数量`)
  }
  if (pathStartsWith(relativePath, ["amountFormula"])) {
    return describeFormulaTerm(relativePath, beforeEffect, afterEffect, `${prefix}的数值公式`)
  }
  if (pathStartsWith(relativePath, ["damageSpec", "amountFormula"])) {
    return describeFormulaTerm(relativePath.slice(1), beforeEffect?.damageSpec, afterEffect?.damageSpec, `${prefix}的伤害公式`)
  }
  if (pathStartsWith(relativePath, ["damageSpec"])) {
    return `${prefix}的伤害结算细节调整`
  }
  if (relativePath[0] === "damageType") {
    return describeScalarChange(`${prefix}的伤害类型`, getAtPath(beforeEffect, relativePath), getAtPath(afterEffect, relativePath))
  }
  return `${prefix}调整`
}

function describeEnhanceCurvePath(pathParts, before, after, labelPrefix) {
  const tail = pathParts[pathParts.length - 1] || ""
  const attrKey = pathParts.find((part) => attrLabels.has(part))
  const attrLabel = attrKey ? attrLabels.get(attrKey) : ""
  const overridesIndex = pathParts.indexOf("overrides")
  const levelLabel = overridesIndex >= 0 && pathParts[overridesIndex + 1] ? `${pathParts[overridesIndex + 1]}级` : ""
  if (typeof getAtPath(before, pathParts) === "number" || typeof getAtPath(after, pathParts) === "number") {
    const label = attrLabel ? `${levelLabel}${labelPrefix}的${attrLabel}` : `${levelLabel}${labelPrefix}`
    return describeScalarChange(label, getAtPath(before, pathParts), getAtPath(after, pathParts), pathParts)
  }
  if (tail === "base") return `${labelPrefix}的初始值调整`
  if (tail === "step") return `${labelPrefix}的每级成长调整`
  return `${labelPrefix}调整`
}

function describeAffixAttrMods(pathParts, before, after) {
  const attrKey = pathParts.find((part) => attrLabels.has(part))
  const attrLabel = attrKey ? attrLabels.get(attrKey) : "属性"
  const bound = pathParts.at(-1)
  const boundLabel = bound === "min" ? "最低值" : bound === "max" ? "最高值" : "数值"
  return describeScalarChange(`${attrLabel}${boundLabel}`, getAtPath(before, pathParts), getAtPath(after, pathParts), pathParts)
}

function describeLimitChange(label, before, after) {
  const formatLimit = (value) => {
    const numeric = Number(value)
    if (!Number.isFinite(numeric) || numeric <= 0) return "未限制"
    return formatValue(numeric)
  }
  return `${label}从 ${formatLimit(before)} 调整为 ${formatLimit(after)}`
}

function describeEnemyDropPath(pathParts, before, after) {
  const itemID = pathParts[1] || ""
  const itemRef = itemID ? `item:${itemID}` : "掉落物"
  const beforeDrop = isRecord(before?.drops) ? before.drops[itemID] : undefined
  const afterDrop = isRecord(after?.drops) ? after.drops[itemID] : undefined
  if (!beforeDrop && afterDrop) {
    const percent = formatValue(afterDrop.percent, ["drops", itemID, "percent"])
    const min = formatValue(afterDrop.range?.min)
    const max = formatValue(afterDrop.range?.max)
    return `新增掉落 ${itemRef}，掉落率 ${percent}，数量 ${min}~${max}`
  }
  if (beforeDrop && !afterDrop) {
    return `移除掉落 ${itemRef}`
  }
  if (pathParts.at(-1) === "percent") {
    return `${itemRef} 掉落率从 ${formatValue(getAtPath(before, pathParts), pathParts)} 调整为 ${formatValue(getAtPath(after, pathParts), pathParts)}`
  }
  if (pathParts.includes("range")) {
    const beforeMin = formatValue(beforeDrop?.range?.min)
    const beforeMax = formatValue(beforeDrop?.range?.max)
    const afterMin = formatValue(afterDrop?.range?.min)
    const afterMax = formatValue(afterDrop?.range?.max)
    return `${itemRef} 掉落数量从 ${beforeMin}~${beforeMax} 调整为 ${afterMin}~${afterMax}`
  }
  return `${itemRef} 掉落配置调整`
}

function describeEnemyEquipmentPath(pathParts, before, after) {
  const index = Number(pathParts[1])
  const beforeEquipment = Array.isArray(before?.equipment) ? before.equipment[index] : undefined
  const afterEquipment = Array.isArray(after?.equipment) ? after.equipment[index] : undefined
  const templateID = String(afterEquipment?.templateItemId || beforeEquipment?.templateItemId || "")
  const itemRef = templateID ? `item:${templateID}` : "装备"
  const tail = pathParts.at(-1)

  if (!beforeEquipment && afterEquipment) return `在战斗中装备了物品 ${itemRef}`
  if (beforeEquipment && !afterEquipment) return `不再在战斗中装备物品 ${itemRef}`

  if (tail === "templateItemId") {
    const beforeRef = beforeEquipment?.templateItemId ? `item:${beforeEquipment.templateItemId}` : "未设置"
    const afterRef = afterEquipment?.templateItemId ? `item:${afterEquipment.templateItemId}` : "未设置"
    return `战斗装备从 ${beforeRef} 调整为 ${afterRef}`
  }
  if (tail === "enhanceLevel") {
    return `${itemRef} 强化等级从 +${formatValue(getAtPath(before, pathParts))} 调整为 +${formatValue(getAtPath(after, pathParts))}`
  }
  if (tail === "equippedSlot" || tail === "slot") {
    return `${itemRef} 装备部位从 ${formatValue(getAtPath(before, pathParts))} 调整为 ${formatValue(getAtPath(after, pathParts))}`
  }
  if (pathParts.includes("baseSnapshot")) {
    const attrKey = pathParts.find((part) => attrLabels.has(part))
    const attrLabel = attrKey ? attrLabels.get(attrKey) : "快照属性"
    return `${itemRef} 的${attrLabel}从 ${formatValue(getAtPath(before, pathParts), pathParts)} 调整为 ${formatValue(getAtPath(after, pathParts), pathParts)}`
  }
  if (pathParts.includes("affixes")) return `${itemRef} 的词条配置调整`
  if (pathParts.includes("durability")) return `${itemRef} 的耐久配置调整`
  return `${itemRef} 配置调整`
}

function describeChangeDetail(table, pathParts, before, after) {
  if (table === "equipment_templates") {
    if (pathStartsWith(pathParts, ["basicAttackEffects"])) {
      return describeEffectPath(table, "basicAttackEffects", pathParts, before, after)
    }
    if (pathStartsWith(pathParts, ["enhanceAttrCurve"])) {
      return describeEnhanceCurvePath(pathParts, before, after, "强化加成")
    }
    if (pathStartsWith(pathParts, ["enhanceCostCurve"])) {
      return describeEnhanceCurvePath(pathParts, before, after, "强化消耗")
    }
    if (pathStartsWith(pathParts, ["baseAttrs"])) {
      const attrKey = pathParts[1]
      return describeScalarChange(`${attrLabels.get(attrKey) || attrKey || "基础属性"}`, getAtPath(before, pathParts), getAtPath(after, pathParts), pathParts)
    }
    if (["affixSlotCount", "enhanceMaxLevel", "maxDurability"].includes(pathParts[0])) {
      return describeScalarChange(labelForPath(table, pathParts), getAtPath(before, pathParts), getAtPath(after, pathParts), pathParts)
    }
  }
  if (table === "skills") {
    if (pathStartsWith(pathParts, ["effects"])) {
      return describeEffectPath(table, "effects", pathParts, before, after)
    }
    if (pathStartsWith(pathParts, ["targeting", "targetCount"])) {
      return describeTargetCountChange(pathParts, before, after, "默认目标数量")
    }
    if (pathHas(pathParts, "secondarySeek")) {
      return describeSeekChange(getAtPath(before, pathParts), getAtPath(after, pathParts), "默认寻敌方式")
    }
    if (pathHas(pathParts, "targetPositions")) {
      return describePositionChange(before?.targeting?.targetPositions, after?.targeting?.targetPositions, "默认目标范围")
    }
    if (["hpCost", "mpCost", "cooldown"].includes(pathParts[0])) {
      return describeScalarChange(labelForPath(table, pathParts), getAtPath(before, pathParts), getAtPath(after, pathParts), pathParts)
    }
  }
  if (table === "affix_definitions") {
    if (pathStartsWith(pathParts, ["effectParams", "attrMods"])) {
      return describeAffixAttrMods(pathParts, before, after)
    }
    if (pathParts[0] === "maxEffectivePerBody") {
      return describeLimitChange("最多同时生效数量", getAtPath(before, pathParts), getAtPath(after, pathParts))
    }
    if (pathParts[0] === "maxEffectivePerEquipment") {
      return describeLimitChange("单件装备最多生效数量", getAtPath(before, pathParts), getAtPath(after, pathParts))
    }
    if (pathParts[0] === "manualScore") {
      return describeScalarChange("特殊效果评分", getAtPath(before, pathParts), getAtPath(after, pathParts), pathParts)
    }
  }
  if (table === "enemies") {
    if (pathStartsWith(pathParts, ["equipment"])) {
      return describeEnemyEquipmentPath(pathParts, before, after)
    }
    if (pathStartsWith(pathParts, ["drops"])) {
      return describeEnemyDropPath(pathParts, before, after)
    }
  }
  if (typeof getAtPath(before, pathParts) !== "object" && typeof getAtPath(after, pathParts) !== "object") {
    return describeScalarChange(labelForPath(table, pathParts), getAtPath(before, pathParts), getAtPath(after, pathParts), pathParts)
  }
  return labelForPath(table, pathParts)
}

function summarizeChangeDetails(table, before, after, paths) {
  const details = []
  const seen = new Set()
  for (const pathParts of paths) {
    const detail = describeChangeDetail(table, pathParts, before, after)
    if (!detail || seen.has(detail)) continue
    seen.add(detail)
    details.push(detail)
  }
  return details.slice(0, 3)
}

function entityRef(table, id) {
  const quickRef = quickRefTables.get(table)
  if (quickRef) return `${quickRef}:${id}`
  if (table === "world") return `world:${id}`
  if (table === "enemies") return id
  return `${table}:${id}`
}

function tableSortIndex(table) {
  const index = tableOrder.indexOf(table)
  return index >= 0 ? index : 1000
}

function buildChangeLines(beforeCatalog, afterCatalog) {
  const allTables = Array.from(new Set([...Object.keys(beforeCatalog), ...Object.keys(afterCatalog)]))
    .filter((table) => table !== "version")
    .filter((table) => Array.isArray(beforeCatalog[table]) || Array.isArray(afterCatalog[table]))
    .sort((a, b) => tableSortIndex(a) - tableSortIndex(b) || a.localeCompare(b))
  const lines = []
  for (const table of allTables) {
    const beforeRows = indexRows(beforeCatalog[table])
    const afterRows = indexRows(afterCatalog[table])
    const ids = Array.from(new Set([...beforeRows.keys(), ...afterRows.keys()])).sort()
    for (const id of ids) {
      const before = beforeRows.get(id)
      const after = afterRows.get(id)
      const ref = entityRef(table, id)
      if (!before && after) {
        lines.push(`新增：${ref}`)
        continue
      }
      if (before && !after) {
        lines.push(`移除：${ref}`)
        continue
      }
      const changedPaths = diffValues(before, after)
      if (changedPaths.length <= 0) continue
      const details = summarizeChangeDetails(table, before, after, changedPaths)
      const suffix = details.length > 0 ? details.join("；") : `${tableDisplayNames.get(table) || table}配置调整`
      lines.push(`调整：${ref} ${suffix}`)
    }
  }
  return lines
}

function formatStringArray(lines, indent) {
  return lines.map((line) => `${indent}${JSON.stringify(line)},`).join("\n")
}

function formatEntry({ id, date, zhLines }) {
  return [
    "  {",
    `    id: ${JSON.stringify(id)},`,
    `    date: ${JSON.stringify(date)},`,
    "    items: {",
    "      \"zh-CN\": [",
    formatStringArray(zhLines, "        "),
    "      ],",
    "      \"en-US\": [",
    `        ${JSON.stringify(defaultTodoLine)},`,
    "      ],",
    "    }",
    "  }"
  ].join("\n")
}

function hasEntryId(updateLogSource, id) {
  const pattern = new RegExp(`\\bid\\s*:\\s*${escapeRegExp(JSON.stringify(id))}\\s*,`)
  return pattern.test(updateLogSource)
}

function escapeRegExp(value) {
  return value.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")
}

function insertEntry(updateLogSource, entryText) {
  const markerIndex = updateLogSource.indexOf(sourceEntriesMarker)
  if (markerIndex < 0) {
    throw new Error(`cannot find marker in update-log.ts: ${sourceEntriesMarker}`)
  }
  const insertIndex = markerIndex + sourceEntriesMarker.length
  return `${updateLogSource.slice(0, insertIndex)}\n${entryText},${updateLogSource.slice(insertIndex)}`
}

function printHelp() {
  console.log(`Usage: node packages/game-data/scripts/generate_update_log_from_diff.mjs [options]

Options:
  --base <git-ref>      Git ref used as the old game-data.json. Default: HEAD
  --date <YYYY-MM-DD>   Update log date. Default: today
  --id <entry-id>       Update log entry id. Default: same as --date
  --dry-run             Print the generated entry instead of editing update-log.ts
  -h, --help            Show this help
`)
}

function main() {
  const args = parseArgs(process.argv.slice(2))
  if (args.help) {
    printHelp()
    return
  }
  const before = readBaseGameData(args.base)
  const after = readJson(gameDataPath)
  const zhLines = buildChangeLines(before, after)
  if (zhLines.length <= 0) {
    console.log("No game-data diff detected.")
    return
  }
  const entryText = formatEntry({ id: args.id, date: args.date, zhLines })
  if (args.dryRun) {
    console.log(entryText)
    return
  }
  const updateLogSource = fs.readFileSync(updateLogPath, "utf8")
  if (hasEntryId(updateLogSource, args.id)) {
    throw new Error(`update-log entry id already exists: ${args.id}. Use --id to choose another id.`)
  }
  fs.writeFileSync(updateLogPath, insertEntry(updateLogSource, entryText), "utf8")
  console.log(`Inserted update-log entry ${args.id} with ${zhLines.length} generated zh-CN lines.`)
}

try {
  main()
} catch (error) {
  console.error(error instanceof Error ? error.message : String(error))
  process.exitCode = 1
}
