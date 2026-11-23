import fs from "node:fs"
import path from "node:path"
import { execFileSync } from "node:child_process"
import vm from "node:vm"
import crypto from "node:crypto"
import { fileURLToPath } from "node:url"
import { normalizeMarketPolicy, validateMarketPolicies } from "./market-policy.mjs"
import { validateCatteryFurniture } from "./cattery-furniture.mjs"
import { validateEquipmentSkills } from "./equipment-skills.mjs"

const projectRoot = process.cwd()
const dataRoot = path.join(projectRoot, "packages/game-data")
const sourceDir = path.join(dataRoot, "source")
const generatedDir = path.join(dataRoot, "generated")
const webTypePath = path.join(projectRoot, "web/src/types/generated/game-data.ts")
const webGameDataDir = path.join(projectRoot, "web/public/game-data")
const webGameDataAssetsDir = path.join(webGameDataDir, "assets")
const zhLocalePath = path.join(projectRoot, "web/src/locales/game-data/zh-CN.ts")
const enLocalePath = path.join(projectRoot, "web/src/locales/game-data/en-US.ts")
const gameDataLocalePaths = {
  "zh-CN": zhLocalePath,
  "zh-TW": path.join(projectRoot, "web/src/locales/game-data/zh-TW.ts"),
  "en-US": enLocalePath,
  ja: path.join(projectRoot, "web/src/locales/game-data/ja.ts"),
  ko: path.join(projectRoot, "web/src/locales/game-data/ko.ts"),
  ru: path.join(projectRoot, "web/src/locales/game-data/ru.ts")
}
const webIconsDir = path.join(projectRoot, "web/public/assets/icons")
const combatModelEquipmentSecondaryClassesPath = path.join(projectRoot, "combat-model-vue3/src/model/equipment-secondary-classes.json")

const jsonSource = path.join(sourceDir, "items.json")
const csvSource = path.join(sourceDir, "items.csv")
const iconsJsonSource = path.join(sourceDir, "icons.json")
const iconsCsvSource = path.join(sourceDir, "icons.csv")
const versionSource = path.join(sourceDir, "version.json")
const excelSource = path.join(sourceDir, "game-data.xlsx")
const excelWorkbookDir = path.join(sourceDir, "workbooks")
const sourceJsonDir = path.join(sourceDir, "json")
const excelReaderScript = path.join(dataRoot, "scripts/read_xlsx.py")
const iconAtlasScript = path.join(dataRoot, "scripts/build_icon_atlas.py")
const AGGREGATED_WORKBOOK = "actions_items_export_order_v4_fixed.xlsx"
const AGGREGATED_EXPORT_ITEMS_SHEET = "Export_Items"
const AGGREGATED_EXPORT_ACTIONS_SHEET = "Export_Actions"
const WORKBOOK_READ_ORDER = ["items.xlsx", "equipment.xlsx"]
const SPECIAL_TYPED_TABLES = new Set(["actions", "action_main_types", "action_sub_types", "buildings"])

// Built-in server-side action main types that have no game-data source entry.
// These are merged into action_main_types after game-data processing so that
// locale sync and chunk generation include them without manual Excel edits.
const BUILTIN_ACTION_MAIN_TYPES = [
  { id: "equipment", nameKey: "action_main_type.equipment.name", descKey: "action_main_type.equipment.desc", icon: "", color: "#333", order: 99, actions: [] }
]
const EXTRA_TABLES = [
  "item_types",
  "market_fee_tiers",
  "equipment_templates",
  "rarity_definitions",
  "affix_definitions",
  "affix_detach_rules",
  "affix_enchant_rules",
  "affix_carrier_rules",
  "enemies",
  "world",
  "actions",
  "action_main_types",
  "action_sub_types",
  "buildings",
  "market_npc_offers",
  "drop_tables",
  "events_world",
  "skills",
  "specialization_trees",
  "arena_formats",
  "combat_trial_metrics",
  "combat_trial_formats",
  "skill_type",
  "weapon_classes",
  "buff",
  "buff_type",
  "buff_effects",
  "combat_config_rules",
  "skill_rule_sets",
  "item_rule_sets",
  "battle_usable_items",
  "operation_effects",
  "cattery_furniture",
  "market_profiles",
  "fish_food_recipes",
  "activity_mutex_rules",
  "effects",
  "cat_village_commissions",
  "collectibles",
  "attributes",
  "attribute_exp_profiles",
  "warehouse_rules",
  "kitty_personalities",
  "kitty_journal_personality_grants",
  "affix_smelt_rules",
  "title_categories",
  "title_prefixes",
  "title_suffixes",
  "achievement_categories",
  "achievements",
  "collectible_categories",
  "alchemy_groups",
  "item_traits",
  "alchemy_combo_recipes",
  "alchemy_transmute_rules",
  "alchemy_inherit_mediums",
  "alchemy_mediums",
  "skill_alchemy_modules",
  "fishing_waters",
  "fishing_spots",
  "fishing_species",
  "fishing_pond_species",
  "fishing_tackle",
  "fishing_infusion_profiles"
]
const GENERIC_EXTRA_TABLES = EXTRA_TABLES.filter((name) => !SPECIAL_TYPED_TABLES.has(name))
const RESERVED_SHEETS = new Set(["items", "icons", ...EXTRA_TABLES])
const LOCALE_SYNC_MARKER_START = "// AUTO-GENERATED START: game-data-i18n"
const LOCALE_SYNC_MARKER_END = "// AUTO-GENERATED END: game-data-i18n"
const ROW_SOURCE_META_KEY = "__rowSourceMeta"
const SPLIT_EXCLUDED_KEYS = new Set(["version"])
const FRONTEND_GAME_DATA_BUNDLE_ORDER = [
  "core",
  "equipment",
  "actions",
  "combat",
  "misc"
]

function ensureDir(dir) {
  if (!fs.existsSync(dir)) fs.mkdirSync(dir, { recursive: true })
}

function cleanDir(dir) {
  if (!fs.existsSync(dir)) return
  for (const entry of fs.readdirSync(dir)) {
    fs.rmSync(path.join(dir, entry), { recursive: true, force: true })
  }
}

function hashContent(content) {
  return crypto.createHash("sha256").update(content).digest("hex").slice(0, 12)
}

function writeFrontendGameDataAsset(fileID, payload) {
  ensureDir(webGameDataAssetsDir)
  const content = JSON.stringify(payload)
  const hash = hashContent(content)
  const safeName = fileID.replace(/[^a-z0-9_]/gi, "_").toLowerCase()
  const fileName = `${safeName}.${hash}.json`
  fs.writeFileSync(path.join(webGameDataAssetsDir, fileName), content)
  return {
    url: `/game-data/assets/${fileName}`,
    hash,
    bytes: Buffer.byteLength(content),
    label: safeName
  }
}

function resolveFrontendGameDataBundle(key) {
  if (key === "equipment_templates" || key === "item_traits" || key === "item_types") return "equipment"
  if (key === "actions" || key === "action_main_types" || key === "action_sub_types" || key === "activity_mutex_rules") return "actions"
  if (
    key === "skills" ||
    key === "specialization_trees" ||
    key === "arena_formats" ||
    key === "combat_trial_metrics" ||
    key === "combat_trial_formats" ||
    key === "skill_type" ||
    key === "skill_rule_sets" ||
    key === "enemies" ||
    key === "battle_usable_items" ||
    key === "combat_trigger_items" ||
    key === "combat_config_rules" ||
    key === "buff" ||
    key === "buff_effects" ||
    key === "buff_type" ||
    key === "effects" ||
    key === "skill_alchemy_modules" ||
    key === "drop_tables"
  ) {
    return "combat"
  }
  return "misc"
}

function buildFrontendGameDataBundles(catalog, chunkDir) {
  const bundles = new Map()
  const addToBundle = (bundleId, key, payload) => {
    const bundle = bundles.get(bundleId) || {
      version: catalog.version,
      contains: [],
      payload: { version: catalog.version }
    }
    bundle.contains.push(key)
    Object.assign(bundle.payload, payload)
    bundles.set(bundleId, bundle)
  }

  const chunkFiles = fs.readdirSync(chunkDir)
    .filter((fileName) => fileName.endsWith(".json"))
    .sort((a, b) => a.localeCompare(b))

  for (const fileName of chunkFiles) {
    const key = fileName.replace(/\.json$/, "")
    const payload = JSON.parse(fs.readFileSync(path.join(chunkDir, fileName), "utf8"))
    if (key === "core") {
      addToBundle("core", "core", payload)
      continue
    }
    addToBundle(resolveFrontendGameDataBundle(key), key, payload)
  }

  return bundles
}

function writeSplitCatalogChunks(catalog) {
  const chunkDir = path.join(generatedDir, "chunks")
  ensureDir(chunkDir)
  ensureDir(webGameDataDir)
  cleanDir(webGameDataAssetsDir)

  const corePayload = {
    version: catalog.version,
    items: Array.isArray(catalog.items) ? catalog.items : [],
    icons: Array.isArray(catalog.icons) ? catalog.icons : []
  }
  fs.writeFileSync(path.join(chunkDir, "core.json"), JSON.stringify(corePayload, null, 2))
  const tables = {}
  for (const [key, value] of Object.entries(catalog)) {
    if (SPLIT_EXCLUDED_KEYS.has(key) || key === "items" || key === "icons") continue
    const safeName = key.replace(/[^a-z0-9_]/gi, "_").toLowerCase()
    const fileName = `${safeName}.json`
    const payload = {
      version: catalog.version,
      [key]: value
    }
    fs.writeFileSync(path.join(chunkDir, fileName), JSON.stringify(payload, null, 2))
    tables[key] = `chunks/${fileName}`
  }

  fs.writeFileSync(path.join(generatedDir, "game-data-manifest.json"), JSON.stringify({
    version: catalog.version,
    core: "chunks/core.json",
    tables
  }, null, 2))

  const frontendBundles = buildFrontendGameDataBundles(catalog, chunkDir)
  const frontendFiles = {}
  for (const bundleId of FRONTEND_GAME_DATA_BUNDLE_ORDER) {
    const bundle = frontendBundles.get(bundleId)
    if (!bundle) continue
    frontendFiles[bundleId] = {
      ...writeFrontendGameDataAsset(bundleId, bundle.payload),
      contains: bundle.contains
    }
  }

  const loadOrder = [
    ...FRONTEND_GAME_DATA_BUNDLE_ORDER
  ].filter((key) => frontendFiles[key])
  fs.writeFileSync(path.join(webGameDataDir, "manifest.json"), JSON.stringify({
    schemaVersion: 1,
    gameDataVersion: catalog.version,
    buildId: `${catalog.version}-${hashContent(JSON.stringify(loadOrder.map((key) => frontendFiles[key].hash)))}`,
    generatedAt: new Date().toISOString(),
    loadOrder,
    files: frontendFiles
  }, null, 2))
}

function buildIconAtlases() {
  if (!fs.existsSync(iconAtlasScript)) return
  try {
    execFileSync("python3", [iconAtlasScript], {
      cwd: projectRoot,
      stdio: "inherit"
    })
  } catch (error) {
    console.warn("[icon-atlas] skipped:", error instanceof Error ? error.message : String(error))
  }
}

function readVersion() {
  if (!fs.existsSync(versionSource)) return 1
  const parsed = JSON.parse(fs.readFileSync(versionSource, "utf8"))
  if (typeof parsed.version !== "number" || parsed.version <= 0) {
    throw new Error("source/version.json version must be a positive number")
  }
  return parsed.version
}

function parseCsv(content) {
  const lines = content
    .split("\n")
    .map((s) => s.trim())
    .filter(Boolean)
  if (lines.length === 0) return []
  const header = lines[0].split(",").map((s) => s.trim())
  return lines.slice(1).map((line) => {
    const cols = line.split(",").map((s) => s.trim())
    const obj = {}
    for (let i = 0; i < header.length; i++) {
      const key = header[i]
      if (!key || key.startsWith("_")) continue
      obj[key] = cols[i]
    }
    return obj
  })
}

function toPascalCase(raw) {
  return raw
    .split("_")
    .filter(Boolean)
    .map((part) => part[0].toUpperCase() + part.slice(1))
    .join("")
}

function parseBoolean(raw, fallback = false) {
  if (typeof raw === "boolean") return raw
  if (typeof raw === "number") return raw !== 0
  if (typeof raw === "string") {
    const normalized = raw.trim().toLowerCase()
    if (normalized === "true" || normalized === "1") return true
    if (normalized === "false" || normalized === "0") return false
  }
  return fallback
}

function parseNumber(raw, fallback = 0) {
  if (typeof raw === "number" && Number.isFinite(raw)) return raw
  if (typeof raw === "string" && raw.trim() !== "") {
    const parsed = Number(raw)
    if (Number.isFinite(parsed)) return parsed
  }
  return fallback
}

function clampNumber(value, min, max) {
  if (!Number.isFinite(value)) return min
  if (value < min) return min
  if (value > max) return max
  return value
}

function parseOptionalString(raw) {
  if (typeof raw === "string" && raw.trim() !== "") return raw.trim()
  return undefined
}

function parseNumberStrict(raw) {
  if (typeof raw === "number") return raw
  if (typeof raw === "string" && raw.trim() !== "") return Number(raw)
  return Number.NaN
}

function attachRowSourceMeta(row, source) {
  if (!row || typeof row !== "object" || !source || typeof source !== "object") return row
  Object.defineProperty(row, ROW_SOURCE_META_KEY, {
    value: source,
    enumerable: false,
    configurable: true,
    writable: true
  })
  return row
}

function copyRowSourceMeta(target, source) {
  const meta = source?.[ROW_SOURCE_META_KEY]
  if (!meta || typeof meta !== "object") return target
  return attachRowSourceMeta(target, { ...meta })
}

function formatRowSource(row, fallbackItemID = "") {
  const source = row?.[ROW_SOURCE_META_KEY]
  if (source && source.file && source.sheet && Number.isInteger(source.row) && source.row > 0) {
    return `file=${source.file}, sheet=${source.sheet}, row=${source.row}`
  }
  if (fallbackItemID) return `item.id=${fallbackItemID}`
  return "item.id=<unknown>"
}

function normalizeEconomyBaseValue(rawValue, row, itemID) {
  if (rawValue === undefined || rawValue === null) return 0
  if (typeof rawValue === "string" && rawValue.trim() === "") return 0

  const parsed = parseNumberStrict(rawValue)
  if (!Number.isFinite(parsed) || parsed < 0 || !Number.isInteger(parsed)) {
    const source = formatRowSource(row, itemID)
    throw new Error(`item.economyBaseValue invalid: ${itemID || "<unknown>"} value=${JSON.stringify(rawValue)} ${source}`)
  }
  return parsed
}

function parseStringArray(raw) {
  if (Array.isArray(raw)) {
    return raw.map((item) => String(item).trim()).filter(Boolean)
  }
  if (typeof raw === "string" && raw.trim() !== "") {
    return raw
      .split("|")
      .map((part) => part.trim())
      .filter(Boolean)
  }
  return undefined
}

function validateOptionalStringArray(raw, message) {
  if (raw === undefined) return
  if (!Array.isArray(raw) || raw.some((item) => typeof item !== "string")) {
    throw new Error(message)
  }
}

function validateOptionalBoolean(raw, message) {
  if (raw === undefined) return
  if (typeof raw !== "boolean") {
    throw new Error(message)
  }
}

function parseCommaSeparatedArray(raw) {
  if (Array.isArray(raw)) {
    return raw.map((item) => String(item).trim()).filter(Boolean)
  }
  if (typeof raw === "string" && raw.trim() !== "") {
    return raw
      .split(",")
      .map((part) => part.trim())
      .filter(Boolean)
  }
  return []
}

function normalizeKittyPersonalityEffects(raw) {
  if (!Array.isArray(raw)) return []
  const out = []
  for (const row of raw) {
    if (!row || typeof row !== "object" || Array.isArray(row)) continue
    const kind = String(row.kind || "").trim().toLowerCase()
    if (!kind) continue
    const normalized = { kind }
    if (kind === "attr_modifier") {
      const attrs = {}
      if (row.attrs && typeof row.attrs === "object" && !Array.isArray(row.attrs)) {
        for (const [attrKey, rawValue] of Object.entries(row.attrs)) {
          const normalizedAttrKey = String(attrKey || "").trim()
          if (!normalizedAttrKey) continue
          const value = parseNumber(rawValue, 0)
          if (!Number.isFinite(value) || value === 0) continue
          attrs[normalizedAttrKey] = value
        }
      }
      normalized.attrs = attrs
      out.push(normalized)
      continue
    }
    if (kind === "battle_position_lock") {
      normalized.position = String(row.position || "").trim().toLowerCase()
      out.push(normalized)
      continue
    }
    if (kind === "battle_hook") {
      normalized.battleHooks = normalizeBattleHookParams(row.battleHooks || row.battle_hooks || row.hooks)
      out.push(normalized)
      continue
    }
    if (kind === "effect_mutation") {
      const mutations = Array.isArray(row.effectMutations)
        ? row.effectMutations
        : Array.isArray(row.effect_mutations)
          ? row.effect_mutations
          : []
      normalized.effectMutations = mutations
      out.push(normalized)
      continue
    }
  }
  return out
}

function validateBattleHooksPayload(hooks, rowID, prefix) {
  if (!Array.isArray(hooks) || hooks.length === 0) {
    throw new Error(`${prefix} invalid: ${rowID}`)
  }
  for (const hook of hooks) {
    if (!hook || typeof hook !== "object" || Array.isArray(hook)) {
      throw new Error(`${prefix} entry invalid: ${rowID}`)
    }
    const triggerKey = normalizeBuffDSLToken(hook.trigger)
    if (
      triggerKey !== "onaftertakedamage" &&
      triggerKey !== "onafterdealdamage" &&
      triggerKey !== "onafterdealcrit" &&
      triggerKey !== "onafterdealnocrit" &&
      triggerKey !== "onbeforedealdamage" &&
      triggerKey !== "onbeforetakedamage" &&
      triggerKey !== "onbattlestart" &&
      triggerKey !== "onbattleend" &&
      triggerKey !== "onallyaftertakedamage" &&
      triggerKey !== "onafterdealheal" &&
      triggerKey !== "onafterbasicattackdealdamage" &&
      triggerKey !== "onunitdeath" &&
      triggerKey !== "onkill"
    ) {
      throw new Error(`${prefix}.trigger invalid: ${rowID}.${hook.trigger}`)
    }
    if (!Array.isArray(hook.conditions)) {
      throw new Error(`${prefix}.conditions invalid: ${rowID}`)
    }
    if (!Array.isArray(hook.effects) || hook.effects.length === 0) {
      throw new Error(`${prefix}.effects invalid: ${rowID}`)
    }
    for (const effect of hook.effects) {
      if (!effect || typeof effect !== "object" || Array.isArray(effect)) {
        throw new Error(`${prefix}.effects entry invalid: ${rowID}`)
      }
      const effectKey = normalizeBuffDSLToken(effect.kind || effect.type)
      if (
        effectKey !== "addbuff" &&
        effectKey !== "removebuff" &&
        effectKey !== "runtimedamageamp" &&
        effectKey !== "addruntimedamageamp" &&
        effectKey !== "adddamageamp" &&
        effectKey !== "extraturn" &&
        effectKey !== "consumemp" &&
        effectKey !== "summon"
      ) {
        throw new Error(`${prefix}.effects.kind invalid: ${rowID}.${effect.kind || effect.type}`)
      }
      if ((effectKey === "addbuff" || effectKey === "removebuff") && (typeof effect.buffId !== "string" || effect.buffId.trim() === "")) {
        throw new Error(`${prefix}.effects.buffId invalid: ${rowID}`)
      }
      if (
        (effectKey === "runtimedamageamp" || effectKey === "addruntimedamageamp" || effectKey === "adddamageamp") &&
        (typeof effect.value !== "number" || !Number.isFinite(effect.value))
      ) {
        throw new Error(`${prefix}.effects.value invalid: ${rowID}.${effect.kind || effect.type}`)
      }
      if (effectKey === "summon") {
        if (!effect.summonPayload || typeof effect.summonPayload !== "object" || Array.isArray(effect.summonPayload)) {
          throw new Error(`${prefix}.effects.summonPayload invalid: ${rowID}`)
        }
        if (typeof effect.summonPayload.unitId !== "string" || effect.summonPayload.unitId.trim() === "") {
          throw new Error(`${prefix}.effects.summonPayload.unitId invalid: ${rowID}`)
        }
        if (effect.summonPayload.maxCountFormula !== undefined) {
          validateSummonMaxCountFormula(effect.summonPayload.maxCountFormula, `${prefix}.effects.summonPayload.maxCountFormula: ${rowID}`)
        }
      }
      if (effectKey === "extraturn") {
        validateBattleHookExtraTurnEffect(effect, rowID, `${prefix}.effects`)
      }
    }
    validateBattleHookSubjectFilter(hook.subjectFilter, rowID, `${prefix}.subjectFilter`)
  }
}

const ALLOWED_BATTLE_HOOK_UNIT_FILTER_TYPES = new Set(["summon", "player", "kitty", "enemy"])

function validateBattleHookSubjectFilter(filter, rowID, prefix) {
  if (filter === undefined || filter === null) return
  if (!filter || typeof filter !== "object" || Array.isArray(filter)) {
    throw new Error(`${prefix} invalid: ${rowID}`)
  }
  if (filter.requireAlive !== undefined && typeof filter.requireAlive !== "boolean") {
    throw new Error(`${prefix}.requireAlive invalid: ${rowID}`)
  }
  for (const key of ["includeUnitTypes", "excludeUnitTypes"]) {
    if (filter[key] === undefined) continue
    if (!Array.isArray(filter[key])) throw new Error(`${prefix}.${key} invalid: ${rowID}`)
    for (const unitType of filter[key]) {
      if (typeof unitType !== "string" || !ALLOWED_BATTLE_HOOK_UNIT_FILTER_TYPES.has(unitType)) {
        throw new Error(`${prefix}.${key} value invalid: ${rowID}.${unitType}`)
      }
    }
  }
  for (const key of ["includeSummonTemplateIds", "excludeSummonTemplateIds"]) {
    if (filter[key] === undefined) continue
    if (!Array.isArray(filter[key]) || filter[key].some((value) => typeof value !== "string" || value.trim() === "")) {
      throw new Error(`${prefix}.${key} invalid: ${rowID}`)
    }
  }
}

function validateBattleHookExtraTurnEffect(effect, rowID, prefix) {
  const targetMode = effect.targetMode ?? effect.target
  if (targetMode !== undefined && targetMode !== "self") {
    throw new Error(`${prefix}.extra_turn.targetMode invalid: ${rowID}`)
  }
  const turnMode = effect.turnMode ?? effect.turn_mode
  if (turnMode !== undefined && turnMode !== "full") {
    throw new Error(`${prefix}.extra_turn.turnMode invalid: ${rowID}`)
  }
}

function validateKittyPersonalityEffectMutations(mutations, rowID) {
  if (!Array.isArray(mutations) || mutations.length === 0) {
    throw new Error(`kitty_personalities.effect_mutation.effectMutations invalid: ${rowID}`)
  }
  for (const mutation of mutations) {
    if (!mutation || typeof mutation !== "object" || Array.isArray(mutation)) {
      throw new Error(`kitty_personalities.effect_mutation.effectMutations entry invalid: ${rowID}`)
    }
    const scope = normalizeBuffDSLToken(mutation.scope)
    if (scope && scope !== "skill" && scope !== "basicattack" && scope !== "both") {
      throw new Error(`kitty_personalities.effect_mutation.scope invalid: ${rowID}.${mutation.scope}`)
    }
    const selector = mutation.effectSelector && typeof mutation.effectSelector === "object" && !Array.isArray(mutation.effectSelector)
      ? mutation.effectSelector
      : mutation.effect_selector && typeof mutation.effect_selector === "object" && !Array.isArray(mutation.effect_selector)
        ? mutation.effect_selector
        : {}
    const mode = normalizeBuffDSLToken(selector.mode)
    if (mode && mode !== "all" && mode !== "index") {
      throw new Error(`kitty_personalities.effect_mutation.effectSelector.mode invalid: ${rowID}.${selector.mode}`)
    }
    const ops = mutation.ops && typeof mutation.ops === "object" && !Array.isArray(mutation.ops)
      ? mutation.ops
      : mutation.operations && typeof mutation.operations === "object" && !Array.isArray(mutation.operations)
        ? mutation.operations
        : {}
    const hasWork =
      ops.targetCountDelta !== undefined ||
      ops.amountBaseAddDelta !== undefined ||
      Array.isArray(ops.amountTermsAppend) ||
      Array.isArray(ops.statusIdsAppend) ||
      Array.isArray(ops.statusIdsRemove) ||
      Array.isArray(ops.appendEffects)
    if (!hasWork) {
      throw new Error(`kitty_personalities.effect_mutation.ops invalid: ${rowID}`)
    }
  }
}

function normalizeKittyPersonalities(rows) {
  if (!Array.isArray(rows)) return []
  const out = []
  for (const row of rows) {
    if (!row || typeof row !== "object" || Array.isArray(row)) continue
    const id = String(row.id || "").trim().toLowerCase()
    if (!id) continue
    out.push({
      id,
      nameKey: String(row.nameKey || `kitty.personality.${id}.name`).trim(),
      descKey: String(row.descKey || `kitty.personality.${id}.desc`).trim(),
      weight: Math.max(0, parseNumber(row.weight, 0)),
      effects: normalizeKittyPersonalityEffects(row.effects)
    })
  }
  return out
}

function validateKittyPersonalities(rows) {
  if (!Array.isArray(rows) || rows.length === 0) {
    throw new Error("kitty_personalities must be a non-empty array")
  }
  const idSet = new Set()
  const allowedKinds = new Set(["attr_modifier", "battle_position_lock", "battle_hook", "effect_mutation"])
  const allowedPositions = new Set(["vanguard", "midguard", "rearguard"])
  for (const row of rows) {
    if (!row || typeof row !== "object") throw new Error("kitty_personalities row must be object")
    if (!row.id || typeof row.id !== "string") throw new Error("kitty_personalities.id is required string")
    if (idSet.has(row.id)) throw new Error(`duplicate kitty_personalities.id: ${row.id}`)
    idSet.add(row.id)
    if (!row.nameKey || typeof row.nameKey !== "string") throw new Error(`kitty_personalities.nameKey invalid: ${row.id}`)
    if (!row.descKey || typeof row.descKey !== "string") throw new Error(`kitty_personalities.descKey invalid: ${row.id}`)
    if (!Number.isFinite(row.weight) || row.weight < 0) throw new Error(`kitty_personalities.weight invalid: ${row.id}`)
    if (!Array.isArray(row.effects)) throw new Error(`kitty_personalities.effects invalid: ${row.id}`)
    for (const effect of row.effects) {
      if (!effect || typeof effect !== "object") throw new Error(`kitty_personalities.effects row invalid: ${row.id}`)
      if (!allowedKinds.has(effect.kind)) throw new Error(`kitty_personalities.effect kind invalid: ${row.id}.${effect.kind}`)
      if (effect.kind === "attr_modifier") {
        if (!effect.attrs || typeof effect.attrs !== "object" || Array.isArray(effect.attrs)) {
          throw new Error(`kitty_personalities.attr_modifier.attrs invalid: ${row.id}`)
        }
        const entries = Object.entries(effect.attrs)
        if (entries.length === 0) {
          throw new Error(`kitty_personalities.attr_modifier.attrs empty: ${row.id}`)
        }
        for (const [attrKey, value] of entries) {
          if (!attrKey || typeof attrKey !== "string") {
            throw new Error(`kitty_personalities.attr_modifier.attrKey invalid: ${row.id}`)
          }
          if (!Number.isFinite(value)) {
            throw new Error(`kitty_personalities.attr_modifier.attrValue invalid: ${row.id}.${attrKey}`)
          }
        }
      } else if (effect.kind === "battle_position_lock") {
        if (!allowedPositions.has(effect.position)) {
          throw new Error(`kitty_personalities.battle_position_lock.position invalid: ${row.id}`)
        }
      } else if (effect.kind === "battle_hook") {
        validateBattleHooksPayload(effect.battleHooks, row.id, "kitty_personalities.battle_hook.battleHooks")
      } else if (effect.kind === "effect_mutation") {
        validateKittyPersonalityEffectMutations(effect.effectMutations, row.id)
      }
    }
  }
}


function getWorkbookPriority(fileName) {
  const normalized = fileName.toLowerCase()
  const index = WORKBOOK_READ_ORDER.findIndex((item) => item === normalized)
  return index >= 0 ? index : Number.MAX_SAFE_INTEGER
}

function parseBaseAttrs(raw) {
  if (raw && typeof raw === "object" && !Array.isArray(raw)) return raw
  if (typeof raw === "string" && raw.trim() !== "") return JSON.parse(raw)
  return {}
}

function parseNumberMap(raw) {
  if (raw && typeof raw === "object" && !Array.isArray(raw)) {
    const out = {}
    for (const [key, value] of Object.entries(raw)) {
      const normalizedKey = String(key || "").trim()
      const num = parseNumber(value, Number.NaN)
      if (!normalizedKey || !Number.isFinite(num)) continue
      out[normalizedKey] = num
    }
    return out
  }
  if (typeof raw === "string" && raw.trim() !== "") {
    try {
      const parsed = JSON.parse(raw)
      return parseNumberMap(parsed)
    } catch {
      return {}
    }
  }
  return {}
}

const BATTLE_HOOK_PROC_CHANCE_FIELD_ALIASES = Object.freeze([
  "procChance",
  "proc_chance",
  "procChanceBase",
  "proc_chance_base",
  "procChanceFormula",
  "proc_chance_formula",
  "procChanceMin",
  "proc_chance_min",
  "procChanceMax",
  "proc_chance_max"
])

export function probabilityLifecycleProcChanceFields(effect) {
  if (!effect || typeof effect !== "object" || Array.isArray(effect)) return []
  return BATTLE_HOOK_PROC_CHANCE_FIELD_ALIASES.filter((field) => Object.hasOwn(effect, field))
}

function parseEquipmentBattleHooks(raw) {
  if (!Array.isArray(raw)) {
    if (!raw || typeof raw !== "object" || Array.isArray(raw)) return []
    raw = [raw]
  }
  const out = []
  for (const row of raw) {
    if (!row || typeof row !== "object" || Array.isArray(row)) continue
    const id = parseOptionalString(row.id) || ""
    const trigger = parseOptionalString(row.trigger) || parseOptionalString(row.when) || ""
    let conditions = row.conditions
    if (!Array.isArray(conditions)) {
      if (conditions && typeof conditions === "object") conditions = [conditions]
      else conditions = []
    }
    let effects = row.effects
    if (!Array.isArray(effects)) {
      if (effects && typeof effects === "object") effects = [effects]
      else effects = []
    }
    const normalizedEffects = []
    for (const effect of effects) {
      if (!effect || typeof effect !== "object" || Array.isArray(effect)) continue
      const kindRaw = String(effect.kind || effect.type || "").trim().toLowerCase()
      if (!kindRaw) continue
      if (kindRaw === "add_buff" || kindRaw === "addbuff") {
        const buffId = parseOptionalString(effect.buffId) || ""
        if (!buffId) continue
        const durationTurns = Math.max(0, Math.round(parseNumber(effect.durationTurns, 0)))
        const targetModeRaw = (parseOptionalString(effect.targetMode) || parseOptionalString(effect.target) || "self").toLowerCase()
        const targetMode = targetModeRaw === "source" || targetModeRaw === "targeting" ? targetModeRaw : "self"
        const targeting = parseSkillTargeting(effect.targeting, {
          targetDirection: "vanguard",
          targetSeekStrategy: "front_to_rear",
          targetBaseCount: 1,
          targetFormula: []
        })
        normalizedEffects.push({
          kind: "add_buff",
          buffId,
          ...(durationTurns > 0 ? { durationTurns } : {}),
          targetMode,
          ...(targetMode === "targeting" ? { targeting } : {}),
          ...(effect.payload && typeof effect.payload === "object" && !Array.isArray(effect.payload) ? { payload: effect.payload } : {}),
          ...(effect.procChanceBase !== undefined ? { procChanceBase: parseNumber(effect.procChanceBase, 0) } : {}),
          ...(effect.proc_chance_base !== undefined ? { procChanceBase: parseNumber(effect.proc_chance_base, 0) } : {}),
          ...(effect.procChance !== undefined ? { procChance: parseNumber(effect.procChance, 0) } : {}),
          ...(effect.proc_chance !== undefined ? { procChance: parseNumber(effect.proc_chance, 0) } : {}),
          ...(effect.procChanceMin !== undefined ? { procChanceMin: parseNumber(effect.procChanceMin, 0) } : {}),
          ...(effect.proc_chance_min !== undefined ? { procChanceMin: parseNumber(effect.proc_chance_min, 0) } : {}),
          ...(effect.procChanceMax !== undefined ? { procChanceMax: parseNumber(effect.procChanceMax, 1) } : {}),
          ...(effect.proc_chance_max !== undefined ? { procChanceMax: parseNumber(effect.proc_chance_max, 1) } : {}),
          ...(effect.procChanceFormula !== undefined ? { procChanceFormula: effect.procChanceFormula } : {}),
          ...(effect.proc_chance_formula !== undefined ? { procChanceFormula: effect.proc_chance_formula } : {})
        })
        continue
      }
      normalizedEffects.push(effect)
    }
    if (!trigger || normalizedEffects.length === 0) continue
    out.push({
      ...(id ? { id } : {}),
      trigger,
      priority: Math.round(parseNumber(row.priority, 0)),
      ...(row.subjectFilter && typeof row.subjectFilter === "object" && !Array.isArray(row.subjectFilter) ? { subjectFilter: row.subjectFilter } : {}),
      ...(row.subject_filter && typeof row.subject_filter === "object" && !Array.isArray(row.subject_filter) ? { subjectFilter: row.subject_filter } : {}),
      ...(row.eventFilter && typeof row.eventFilter === "object" && !Array.isArray(row.eventFilter) ? { eventFilter: row.eventFilter } : {}),
      ...(row.event_filter && typeof row.event_filter === "object" && !Array.isArray(row.event_filter) ? { eventFilter: row.event_filter } : {}),
      conditions,
      effects: normalizedEffects,
      ...(row.hideDescription === true || row.hide_description === true ? { hideDescription: true } : {})
    })
  }
  return out
}

function normalizeBattleHookParams(raw) {
  if (Array.isArray(raw)) return parseEquipmentBattleHooks(raw)
  if (!raw || typeof raw !== "object") return []
  const direct = parseEquipmentBattleHooks(raw)
  if (direct.length > 0) return direct
  const out = {}
  for (const [key, value] of Object.entries(raw)) {
    const hooks = parseEquipmentBattleHooks(value)
    if (hooks.length > 0) out[key] = hooks
  }
  return Object.keys(out).length > 0 ? out : []
}

function parseDefaultAffixSlots(raw) {
  if (!Array.isArray(raw)) {
    if (!raw || typeof raw !== "object" || Array.isArray(raw)) return []
    raw = [raw]
  }
  const out = []
  for (const row of raw) {
    if (!row || typeof row !== "object" || Array.isArray(row)) continue
    const slotIndexRaw = parseNumber(row.slotIndex, Number.NaN)
    const slotIndex = Number.isFinite(slotIndexRaw) ? Math.round(slotIndexRaw) : Number.NaN
    const affixId = parseOptionalString(row.affixId) || ""
    const rarityId = parseOptionalString(row.rarityId) || ""
    const rollValue = clampNumber(parseNumber(row.rollValue, 0), 0, 1)
    out.push({
      slotIndex,
      affixId,
      rarityId,
      rollValue
    })
  }
  return out
}

function parseEnhanceAffixMilestonePools(raw) {
  const node = raw && typeof raw === "object" && !Array.isArray(raw)
    ? raw
    : typeof raw === "string" && raw.trim() !== ""
      ? JSON.parse(raw)
      : {}
  if (!node || typeof node !== "object" || Array.isArray(node)) {
    return {}
  }
  const out = {}
  for (const [level, payload] of Object.entries(node)) {
    const normalizedLevel = String(level || "").trim()
    if (!normalizedLevel) continue
    const rows = Array.isArray(payload) ? payload : payload && typeof payload === "object" ? [payload] : []
    const pool = []
    for (const entry of rows) {
      if (!entry || typeof entry !== "object" || Array.isArray(entry)) continue
      const affixId = parseOptionalString(entry.affixId) || ""
      const rarityId = parseOptionalString(entry.rarityId) || ""
      if (!affixId || !rarityId) continue
      const rollValueRaw = entry.rollValue
      if (rollValueRaw === undefined || rollValueRaw === null || rollValueRaw === "") {
        pool.push({ affixId, rarityId })
        continue
      }
      const rollValue = parseNumber(rollValueRaw, Number.NaN)
      if (!Number.isFinite(rollValue)) continue
      pool.push({ affixId, rarityId, rollValue })
    }
    if (pool.length > 0) out[normalizedLevel] = pool
  }
  return out
}

function parseFiniteNumberMap(raw) {
  if (raw && typeof raw === "object" && !Array.isArray(raw)) {
    const out = {}
    for (const [key, value] of Object.entries(raw)) {
      const normalizedKey = String(key || "").trim()
      const num = parseNumberStrict(value)
      if (!normalizedKey) {
        throw new Error("enhance curve key must be non-empty string")
      }
      if (!Number.isFinite(num)) {
        throw new Error(`enhance curve value must be finite number: ${normalizedKey}`)
      }
      out[normalizedKey] = num
    }
    return out
  }
  if (typeof raw === "string" && raw.trim() !== "") {
    return parseFiniteNumberMap(JSON.parse(raw))
  }
  return {}
}

function parseEnhanceMapCurve(raw) {
  const node = raw && typeof raw === "object" && !Array.isArray(raw)
    ? raw
    : typeof raw === "string" && raw.trim() !== ""
      ? JSON.parse(raw)
      : {}
  if (!node || typeof node !== "object" || Array.isArray(node)) {
    return { base: {}, step: {}, overrides: {} }
  }
  const base = parseFiniteNumberMap(node.base)
  const step = parseFiniteNumberMap(node.step)
  const overrides = {}
  if (node.overrides && typeof node.overrides === "object" && !Array.isArray(node.overrides)) {
    for (const [level, payload] of Object.entries(node.overrides)) {
      overrides[String(level).trim()] = parseFiniteNumberMap(payload)
    }
  } else if (typeof node.overrides === "string" && node.overrides.trim() !== "") {
    const parsedOverrides = JSON.parse(node.overrides)
    if (parsedOverrides && typeof parsedOverrides === "object" && !Array.isArray(parsedOverrides)) {
      for (const [level, payload] of Object.entries(parsedOverrides)) {
        overrides[String(level).trim()] = parseFiniteNumberMap(payload)
      }
    }
  }
  return { base, step, overrides }
}

function parseEnhanceAttrMilestoneOverrides(raw) {
  const node = raw && typeof raw === "object" && !Array.isArray(raw)
    ? raw
    : typeof raw === "string" && raw.trim() !== ""
      ? JSON.parse(raw)
      : {}
  if (!node || typeof node !== "object" || Array.isArray(node)) {
    return {}
  }
  const overrides = {}
  for (const [level, payload] of Object.entries(node)) {
    overrides[String(level).trim()] = parseFiniteNumberMap(payload)
  }
  return overrides
}

function normalizeItem(raw) {
  const id = typeof raw?.id === "string" ? raw.id.trim() : ""
  const type = parseCommaSeparatedArray(raw?.type)
  const stackableFlag = parseBoolean(raw?.stackable, false)
  const hasCurrencyType = type.includes("currency")
  const iconIdFallback = type.includes("skillbook")
    ? "item.skillbook"
    : (parseOptionalString(raw?.iconId) || (id ? `item.${id}` : undefined))
  const normalized = {
    id,
    type,
    rarity: typeof raw?.rarity === "string" ? raw.rarity.trim() : "",
    skillForgeEligible: parseBoolean(raw?.skillForgeEligible, false),
    category: parseOptionalString(raw?.category) || inferItemCategory(type, id),
    isEquipment: parseBoolean(raw?.isEquipment, false),
    disabled: parseBoolean(raw?.disabled, false),
    baseAttrs: parseBaseAttrs(raw?.baseAttrs),
    stackable: stackableFlag,
    carryableInCombat: parseBoolean(raw?.carryableInCombat, false),
    marketListable: parseBoolean(raw?.marketListable, !hasCurrencyType),
    marketPolicy: normalizeMarketPolicy(raw?.marketPolicy),
    iconId: iconIdFallback,
    emojiIcon: parseOptionalString(raw?.emojiIcon),
    nameKey: parseOptionalString(raw?.nameKey) || (id ? `item.${id}.name` : undefined),
    descKey: parseOptionalString(raw?.descKey) || (id ? `item.${id}.desc` : undefined),
    effectDescKey: parseOptionalString(raw?.effectDescKey) || (id ? `item.${id}.effect` : undefined),
    economyBaseValue: normalizeEconomyBaseValue(raw?.economyBaseValue, raw, id),
    useGrantSkillId: parseOptionalString(raw?.useGrantSkillId),
    useGrantCollectibleId: parseOptionalString(raw?.useGrantCollectibleId),
    useOperationEffectId: parseOptionalString(raw?.useOperationEffectId),
    battleUsableItemId: parseOptionalString(raw?.battleUsableItemId),
    useSkillExpGain: parseNumber(raw?.useSkillExpGain, 0),
    kittyFeed: parseKittyFeed(raw?.kittyFeed),
    pondFeed: parsePondFeed(raw?.pondFeed, id),
    alchemy: parseItemAlchemyMeta(raw?.alchemy)
  }
  return copyRowSourceMeta(normalized, raw)
}

function parseItemAlchemyMeta(raw) {
  let node = raw
  if (typeof node === "string" && node.trim() !== "") {
    node = JSON.parse(node)
  }
  if (!node || typeof node !== "object" || Array.isArray(node)) {
    return undefined
  }
  const convertExcluded = parseBoolean(node.convertExcluded, false)
  const convertWeight = node.convertWeight === undefined
    ? undefined
    : parseNumber(node.convertWeight, Number.NaN)
  const convertWeightMultiplier = node.convertWeightMultiplier === undefined
    ? undefined
    : parseNumber(node.convertWeightMultiplier, Number.NaN)
  const nameAffixRaw = node.nameAffix && typeof node.nameAffix === "object" && !Array.isArray(node.nameAffix)
    ? node.nameAffix
    : null
  const out = {}
  if (nameAffixRaw) {
    const prefixKey = parseOptionalString(nameAffixRaw.prefixKey) || ""
    const suffixKey = parseOptionalString(nameAffixRaw.suffixKey) || ""
    if (prefixKey || suffixKey) out.nameAffix = { prefixKey, suffixKey }
  }
  if (convertExcluded) out.convertExcluded = true
  if (convertWeight !== undefined) out.convertWeight = convertWeight
  if (convertWeightMultiplier !== undefined) out.convertWeightMultiplier = convertWeightMultiplier
  return Object.keys(out).length > 0 ? out : undefined
}

function inferItemCategory(type, _id) {
  // category is the item's primary classification. Trait-bearing materials
  // (for alchemy bench) carry "alchemy_material"; that's set explicitly in
  // items.json — DO NOT derive it from a hardcoded id list here.
  if (type.includes("equipment")) return "equipment"
  if (type.includes("currency")) return "currency"
  if (type.includes("material")) return "material"
  if (type.includes("loot")) return "loot"
  if (type.includes("special")) return "special"
  if (type.includes("skillbook")) return "skillbook"
  if (type.includes("food")) return "food"
  if (type.includes("collectible")) return "collectible"
  if (type.includes("trigger_item")) return "trigger_item"
  return type[0] || "other"
}

function parseKittyFeed(raw) {
  let node = raw
  if (typeof node === "string" && node.trim() !== "") {
    node = JSON.parse(node)
  }
  if (!node || typeof node !== "object" || Array.isArray(node)) {
    return undefined
  }
  const battleLevelCapGain = Math.round(parseNumber(node?.battleLevelCapGain, 0))
  if (!Number.isFinite(battleLevelCapGain) || battleLevelCapGain <= 0) {
    return undefined
  }
  return { battleLevelCapGain }
}

function parsePondFeed(raw, itemID = "") {
  let node = raw
  if (typeof node === "string" && node.trim() !== "") {
    node = JSON.parse(node)
  }
  if (node === undefined || node === null || node === "") return undefined
  if (!node || typeof node !== "object" || Array.isArray(node)) {
    throw new Error(`item.pondFeed must be an object: ${itemID || "(unknown)"}`)
  }
  if (Object.keys(node).length === 0) return undefined
  const allowedKeys = new Set(["nutrition", "feedClass", "rulesetVersion", "consumePriority", "conditions", "effects", "role", "maxPortionsPerFishHour", "delivery"])
  for (const key of Object.keys(node)) {
    if (!allowedKeys.has(key)) throw new Error(`item.pondFeed.${key} is unsupported: ${itemID || "(unknown)"}`)
  }
  const nutrition = parseNumber(node.nutrition, Number.NaN)
  const feedClass = String(node.feedClass || "").trim().toLowerCase()
  const out = { nutrition, feedClass }
  if (node.delivery !== undefined) out.delivery = String(node.delivery)
  if (node.rulesetVersion !== undefined) out.rulesetVersion = String(node.rulesetVersion || "").trim()
  if (node.consumePriority !== undefined) out.consumePriority = parseNumber(node.consumePriority, Number.NaN)
  if (node.role !== undefined) out.role = String(node.role)
  if (node.maxPortionsPerFishHour !== undefined) out.maxPortionsPerFishHour = parseNumber(node.maxPortionsPerFishHour, Number.NaN)
  if (node.conditions !== undefined) {
    out.conditions = Array.isArray(node.conditions)
      ? node.conditions.map((condition) => {
        if (!condition || typeof condition !== "object" || Array.isArray(condition)) return condition
        const allowed = new Set(["field", "op", "value"])
        for (const key of Object.keys(condition)) {
          if (!allowed.has(key)) throw new Error(`item.pondFeed.conditions.${key} is unsupported: ${itemID || "(unknown)"}`)
        }
        return {
          field: String(condition.field || "").trim(),
          op: String(condition.op || "").trim(),
          value: condition.value
        }
      })
      : node.conditions
  }
  if (node.effects !== undefined) {
    out.effects = Array.isArray(node.effects)
      ? node.effects.map((effect) => {
        if (!effect || typeof effect !== "object" || Array.isArray(effect)) return effect
        const allowed = new Set(["kind", "stat", "mode", "valuePerPortion", "maxTargetMultiplier", "floor", "ceiling"])
        for (const key of Object.keys(effect)) {
          if (!allowed.has(key)) throw new Error(`item.pondFeed.effects.${key} is unsupported: ${itemID || "(unknown)"}`)
        }
        const normalized = {
          kind: String(effect.kind || "").trim(),
          stat: String(effect.stat || "").trim(),
          mode: String(effect.mode || "").trim(),
          valuePerPortion: parseNumber(effect.valuePerPortion, Number.NaN)
        }
        if (effect.maxTargetMultiplier !== undefined) normalized.maxTargetMultiplier = parseNumber(effect.maxTargetMultiplier, Number.NaN)
        if (effect.floor !== undefined) normalized.floor = parseNumber(effect.floor, Number.NaN)
        if (effect.ceiling !== undefined) normalized.ceiling = parseNumber(effect.ceiling, Number.NaN)
        return normalized
      })
      : node.effects
  }
  return out
}

function parseItemsCsv(content) {
  return parseCsv(content).map((obj) => normalizeItem(obj))
}

function parseIconsCsv(content) {
  return parseCsv(content).map((obj) => {
    return {
      id: obj.id,
      path: obj.path
    }
  })
}

function loadItems() {
  const fromExcel = loadItemsFromExcel()
  const fromSourceJson = loadFromSourceJsonTable("items")
  if (fromExcel && fromSourceJson) {
    return mergeExcelAndSourceJsonRows("items", fromExcel, fromSourceJson)
  }
  if (fromExcel) return fromExcel
  if (fromSourceJson) return fromSourceJson
  const hasExcelWorkbooks = listExcelSources().length > 0
  if (fs.existsSync(jsonSource)) {
    return JSON.parse(fs.readFileSync(jsonSource, "utf8"))
  }
  if (fs.existsSync(csvSource)) {
    return parseItemsCsv(fs.readFileSync(csvSource, "utf8"))
  }
  if (hasExcelWorkbooks) {
    throw new Error("missing source data: no rows found in 'items'/'items_*' sheets and no items.json/items.csv fallback")
  }
  throw new Error("missing source file: items.json or items.csv")
}

function loadItemsFromExcel() {
  const fromAggregated = loadRowsFromWorkbookSheet(AGGREGATED_WORKBOOK, AGGREGATED_EXPORT_ITEMS_SHEET)
  if (fromAggregated && fromAggregated.length > 0) return fromAggregated
  const tables = loadExcelTables()
  const out = []
  const itemSheetNames = Object.keys(tables)
    .filter((sheetName) => sheetName === "items" || sheetName.startsWith("items_"))
    .sort((a, b) => a.localeCompare(b))
  for (const sheetName of itemSheetNames) {
    const rows = tables[sheetName]
    if (!Array.isArray(rows) || rows.length === 0) continue
    out.push(...rows)
  }
  return out.length > 0 ? out : null
}

function loadPrefixedTableFromExcel(prefix) {
  const tables = loadExcelTables()
  const out = []
  const matchedSheetNames = Object.keys(tables)
    .filter((sheetName) => sheetName.startsWith(prefix))
    .sort((a, b) => a.localeCompare(b))
  for (const sheetName of matchedSheetNames) {
    const rows = tables[sheetName]
    if (!Array.isArray(rows) || rows.length === 0) continue
    out.push(...rows)
  }
  return out.length > 0 ? out : null
}

function loadExactAndSplitTablesFromExcel(name) {
  const tables = loadExcelTables()
  const out = []
  const matchedSheetNames = Object.keys(tables)
    .filter((sheetName) => {
      if (sheetName !== name && !sheetName.startsWith(`${name}_`)) return false
      if (name === "buff" && (sheetName === "buff_type" || sheetName.startsWith("buff_effects"))) return false
      return true
    })
    .sort((a, b) => a.localeCompare(b))
  for (const sheetName of matchedSheetNames) {
    const rows = tables[sheetName]
    if (!Array.isArray(rows) || rows.length === 0) continue
    out.push(...rows)
  }
  return out.length > 0 ? out : null
}

function loadIcons() {
  const fromExcel = loadFromExcel("icons")
  if (fromExcel) return fromExcel
  if (fs.existsSync(iconsJsonSource)) {
    return JSON.parse(fs.readFileSync(iconsJsonSource, "utf8"))
  }
  if (fs.existsSync(iconsCsvSource)) {
    return parseIconsCsv(fs.readFileSync(iconsCsvSource, "utf8"))
  }
  return []
}

function loadOptionalTable(name) {
  let fromExcel = null
  if (name === "action_main_types" || name === "action_sub_types" || name === "item_types") {
    const fromAggregated = loadRowsFromWorkbookSheet(AGGREGATED_WORKBOOK, name)
    if (fromAggregated && fromAggregated.length > 0) fromExcel = fromAggregated
  }
  if (name === "enemies") {
    const fromEnemyWorkbook = loadEnemiesFromEnemyWorkbook()
    if (fromEnemyWorkbook.length > 0) fromExcel = fromEnemyWorkbook
  }
  if (name === "equipment_templates") {
    const fromEquipmentTemplateSheets = loadPrefixedTableFromExcel("equipment_templates")
    if (fromEquipmentTemplateSheets) fromExcel = fromEquipmentTemplateSheets
  }
  if (
    name === "affix_definitions" ||
    name === "affix_enchant_rules" ||
    name === "skills" ||
    name === "weapon_classes" ||
    name === "buff" ||
    name === "buff_effects"
  ) {
    const fromSplitSheets = loadExactAndSplitTablesFromExcel(name)
    if (fromSplitSheets) fromExcel = fromSplitSheets
  }
  if (name === "world") {
    const fromEnemyWorkbook = loadWorldFromEnemyWorkbook()
    if (fromEnemyWorkbook.length > 0) fromExcel = fromEnemyWorkbook
  }
  if (!fromExcel) fromExcel = loadFromExcel(name)
  const fromSourceJson = loadFromSourceJsonTable(name)
  if (fromExcel && fromSourceJson) {
    return mergeExcelAndSourceJsonRows(name, fromExcel, fromSourceJson)
  }
  if (fromExcel) return fromExcel
  if (fromSourceJson) return fromSourceJson
  const jsonPath = path.join(sourceDir, `${name}.json`)
  const csvPath = path.join(sourceDir, `${name}.csv`)
  if (fs.existsSync(jsonPath)) {
    return JSON.parse(fs.readFileSync(jsonPath, "utf8"))
  }
  if (fs.existsSync(csvPath)) {
    return parseCsv(fs.readFileSync(csvPath, "utf8"))
  }
  return []
}

function loadFromSourceJsonTable(name) {
  const jsonPath = path.join(sourceJsonDir, `${name}.json`)
  if (!fs.existsSync(jsonPath)) return null
  const parsed = JSON.parse(fs.readFileSync(jsonPath, "utf8"))
  if (Array.isArray(parsed)) return parsed
  if (parsed && typeof parsed === "object" && Array.isArray(parsed[name])) return parsed[name]
  throw new Error(`source/json/${name}.json must be an array or object with ${name} array`)
}

function loadEquipmentSecondaryClassesFromCombatModel() {
  if (!fs.existsSync(combatModelEquipmentSecondaryClassesPath)) return []
  const parsed = JSON.parse(fs.readFileSync(combatModelEquipmentSecondaryClassesPath, "utf8"))
  if (!Array.isArray(parsed)) {
    throw new Error("combat-model equipment-secondary-classes.json must be an array")
  }
  return parsed
}

function mergeExcelAndSourceJsonRows(name, excelRows, jsonRows) {
  if (!Array.isArray(excelRows) || excelRows.length === 0) return Array.isArray(jsonRows) ? jsonRows : []
  if (!Array.isArray(jsonRows) || jsonRows.length === 0) return excelRows
  const excelByID = new Map()
  const out = []
  const rowMergeKey = (row) => {
    const id = typeof row?.id === "string" ? row.id.trim() : ""
    if (!id) return ""
    if (name === "attribute_exp_profiles") {
      const level = parseNumber(row?.level, Number.NaN)
      if (!Number.isFinite(level)) return ""
      return `${id}::${Math.round(level)}`
    }
    return id
  }
  for (const row of excelRows) {
    const key = rowMergeKey(row)
    if (key && !excelByID.has(key)) excelByID.set(key, row)
    out.push(row)
  }
  const outIndexByID = new Map()
  for (let index = 0; index < out.length; index += 1) {
    const key = rowMergeKey(out[index])
    if (key && !outIndexByID.has(key)) outIndexByID.set(key, index)
  }
  const overridden = []
  for (const row of jsonRows) {
    const key = rowMergeKey(row)
    if (!key) {
      out.push(row)
      continue
    }
    const existingIndex = outIndexByID.get(key)
    if (existingIndex === undefined) {
      outIndexByID.set(key, out.length)
      out.push(row)
      continue
    }
    const excelRow = excelByID.get(key)
    if (excelRow && process.env.GAME_DATA_STRICT_JSON_CONFLICT === "1") {
      const excelSource = formatRowSource(excelRow)
      throw new Error(`source/json conflict: table=${name}, id=${key}, excel=${excelSource}, json=source/json/${name}.json`)
    }
    out[existingIndex] = row
    overridden.push(key)
  }
  if (overridden.length > 0) {
    const preview = overridden.slice(0, 8).join(", ")
    const suffix = overridden.length > 8 ? `, ... +${overridden.length - 8}` : ""
    console.warn(`source/json overrides ${name}: ${preview}${suffix}`)
  }
  return out
}

function rowSourceFile(row) {
  const source = row?.[ROW_SOURCE_META_KEY]
  if (!source || typeof source !== "object") return ""
  if (typeof source.file !== "string") return ""
  return source.file.trim().toLowerCase()
}

function matchesWorkbookFileName(row, workbookFileName) {
  const sourceFile = rowSourceFile(row)
  return sourceFile === String(workbookFileName || "").trim().toLowerCase()
}

function loadRowsFromWorkbookSheet(workbookFileName, sheetName) {
  const tables = loadExcelTables()
  const rows = tables[sheetName]
  if (!Array.isArray(rows) || rows.length === 0) return null
  const filtered = rows.filter((row) => row && typeof row === "object" && matchesWorkbookFileName(row, workbookFileName))
  return filtered.length > 0 ? filtered : null
}

function loadEnemiesFromEnemyWorkbook() {
  const tables = loadExcelTables()
  const out = []
  for (const [sheetName, rows] of Object.entries(tables)) {
    if (sheetName === "world") continue
    if (!Array.isArray(rows) || rows.length === 0) continue
    for (const row of rows) {
      if (!row || typeof row !== "object") continue
      if (rowSourceFile(row) !== "enemy.xlsx") continue
      if (sheetName.startsWith("enemy_drop_overrides_")) continue
      const id = typeof row.id === "string" ? row.id.trim() : ""
      if (!id) continue
      out.push(row)
    }
  }
  return out
}

function loadEnemyDropOverridesFromExcel() {
  const tables = loadExcelTables()
  const out = []
  const matchedSheetNames = Object.keys(tables)
    .filter((sheetName) => sheetName.startsWith("enemy_drop_overrides_"))
    .sort((a, b) => a.localeCompare(b))
  for (const sheetName of matchedSheetNames) {
    const rows = tables[sheetName]
    if (!Array.isArray(rows) || rows.length === 0) continue
    out.push(...rows)
  }
  return out
}

function loadEnemyDropsFromExcel() {
  const tables = loadExcelTables()
  const out = []
  const matchedSheetNames = Object.keys(tables)
    .filter((sheetName) => sheetName.startsWith("enemy_drops_"))
    .sort((a, b) => a.localeCompare(b))
  for (const sheetName of matchedSheetNames) {
    const rows = tables[sheetName]
    if (!Array.isArray(rows) || rows.length === 0) continue
    out.push(...rows)
  }
  return out
}

function applyEnemyDropOverrides(enemies, overrides) {
  if (!Array.isArray(overrides) || overrides.length === 0) return enemies
  const byID = new Map()
  for (const row of enemies || []) {
    const id = typeof row?.id === "string" ? row.id.trim() : ""
    if (id) byID.set(id, row)
  }
  for (const raw of overrides) {
    const id = typeof raw?.id === "string" ? raw.id.trim() : ""
    if (!id) continue
    const enemy = byID.get(id)
    if (!enemy) {
      throw new Error(`enemy_drop_overrides enemy not found: ${id}`)
    }
    const drops = raw?.drops && typeof raw.drops === "object" && !Array.isArray(raw.drops) ? raw.drops : {}
    enemy.drops = {
      ...(enemy.drops && typeof enemy.drops === "object" && !Array.isArray(enemy.drops) ? enemy.drops : {}),
      ...drops
    }
  }
  return enemies
}

function applyEnemyDrops(enemies, dropRows, items) {
  if (!Array.isArray(dropRows) || dropRows.length === 0) return enemies
  const byID = new Map()
  for (const row of enemies || []) {
    const id = typeof row?.id === "string" ? row.id.trim() : ""
    if (id) byID.set(id, row)
  }
  const itemIDs = new Set(
    (items || [])
      .map((item) => (typeof item?.id === "string" ? item.id.trim() : ""))
      .filter(Boolean)
  )
  const seen = new Set()
  for (const raw of dropRows) {
    const enemyID = typeof raw?.enemyId === "string" ? raw.enemyId.trim() : ""
    const itemID = typeof raw?.itemId === "string" ? raw.itemId.trim() : ""
    const source = formatRowSource(raw)
    if (!enemyID) throw new Error(`enemy_drops.enemyId missing: ${source}`)
    if (!itemID) throw new Error(`enemy_drops.itemId missing: ${source}`)
    const enemy = byID.get(enemyID)
    if (!enemy) throw new Error(`enemy_drops enemy not found: ${enemyID} ${source}`)
    if (!itemIDs.has(itemID)) throw new Error(`enemy_drops item not found: ${enemyID}.${itemID} ${source}`)
    const duplicateKey = `${enemyID}.${itemID}`
    if (seen.has(duplicateKey)) throw new Error(`duplicate enemy_drops row: ${duplicateKey} ${source}`)
    seen.add(duplicateKey)
    const existingDrops = enemy.drops && typeof enemy.drops === "object" && !Array.isArray(enemy.drops) ? enemy.drops : {}
    const percent = parseNumberStrict(raw?.percent)
    if (!Number.isFinite(percent) || percent < 0 || percent > 1) {
      throw new Error(`enemy_drops.percent invalid: ${duplicateKey} ${source}`)
    }
    const min = parseNumberStrict(raw?.min)
    const max = parseNumberStrict(raw?.max)
    if (
      !Number.isFinite(min) ||
      !Number.isFinite(max) ||
      min < 1 ||
      max < min ||
      !Number.isInteger(min) ||
      !Number.isInteger(max)
    ) {
      throw new Error(`enemy_drops.range invalid: ${duplicateKey} ${source}`)
    }
    if (existingDrops[itemID] !== undefined) {
      const existing = existingDrops[itemID]
      const existingPercent = parseNumberStrict(existing?.percent)
      const existingMin = parseNumberStrict(existing?.range?.min)
      const existingMax = parseNumberStrict(existing?.range?.max)
      if (existingPercent === percent && existingMin === min && existingMax === max) {
        continue
      }
      throw new Error(`enemy_drops duplicate existing drop: ${duplicateKey} ${source}`)
    }
    enemy.drops = {
      ...existingDrops,
      [itemID]: {
        percent,
        range: { min, max }
      }
    }
  }
  return enemies
}

function loadWorldFromEnemyWorkbook() {
  const tables = loadExcelTables()
  const rows = tables.world
  if (!Array.isArray(rows) || rows.length === 0) return []
  return rows.filter((row) => row && typeof row === "object" && rowSourceFile(row) === "enemy.xlsx")
}

let excelTablesCache = null

function listExcelSources() {
  const paths = []
  if (fs.existsSync(excelWorkbookDir) && fs.statSync(excelWorkbookDir).isDirectory()) {
    const workbooks = fs
      .readdirSync(excelWorkbookDir)
      .filter((file) => {
        const lower = file.toLowerCase()
        if (!lower.endsWith(".xlsx")) return false
        // Skip temporary/lock files created by spreadsheet editors.
        if (file.startsWith(".~") || file.startsWith("~$") || file.startsWith(".")) return false
        const fullPath = path.join(excelWorkbookDir, file)
        return fs.statSync(fullPath).isFile()
      })
      .sort((a, b) => {
        const priorityDiff = getWorkbookPriority(a) - getWorkbookPriority(b)
        if (priorityDiff !== 0) return priorityDiff
        return a.localeCompare(b)
      })
      .map((file) => path.join(excelWorkbookDir, file))
    paths.push(...workbooks)
  }
  if (paths.length === 0 && fs.existsSync(excelSource)) {
    paths.push(excelSource)
  }
  return paths
}

function loadExcelTables() {
  if (excelTablesCache !== null) return excelTablesCache
  const excelSources = listExcelSources()
  if (excelSources.length === 0) {
    excelTablesCache = {}
    return excelTablesCache
  }
  const mergedTables = {}
  for (const workbookPath of excelSources) {
    const raw = execFileSync("python3", [excelReaderScript, workbookPath], {
      encoding: "utf8",
      maxBuffer: 64 * 1024 * 1024
    })
    const sourceFile = path.basename(workbookPath)
    const workbookTables = JSON.parse(raw)
    for (const [sheetName, rows] of Object.entries(workbookTables)) {
      if (!Array.isArray(rows) || rows.length === 0) continue
      if (!Array.isArray(mergedTables[sheetName])) {
        mergedTables[sheetName] = []
      }
      for (const row of rows) {
        if (!row || typeof row !== "object" || Array.isArray(row)) continue
        const normalizedRow = { ...row }
        const rowNumberRaw = normalizedRow.__rowNum
        delete normalizedRow.__rowNum
        const rowNumber = Number(rowNumberRaw)
        if (Number.isInteger(rowNumber) && rowNumber > 0) {
          attachRowSourceMeta(normalizedRow, {
            file: sourceFile,
            sheet: sheetName,
            row: rowNumber
          })
        } else {
          attachRowSourceMeta(normalizedRow, {
            file: sourceFile,
            sheet: sheetName
          })
        }
        mergedTables[sheetName].push(normalizedRow)
      }
    }
  }
  excelTablesCache = mergedTables
  return excelTablesCache
}

function loadFromExcel(sheetName) {
  const tables = loadExcelTables()
  const table = tables[sheetName]
  if (!Array.isArray(table) || table.length === 0) return null
  return table
}

function validateIcons(icons) {
  if (!Array.isArray(icons)) throw new Error("icons must be an array")
  const ids = new Set()
  for (const icon of icons) {
    if (!icon || typeof icon !== "object") throw new Error("icon must be object")
    if (!icon.id || typeof icon.id !== "string") throw new Error("icon.id is required string")
    if (ids.has(icon.id)) throw new Error(`duplicate icon id: ${icon.id}`)
    ids.add(icon.id)
    if (typeof icon.path !== "string") throw new Error(`icon.path invalid: ${icon.id}`)
  }
}

function validatePondFeed(item) {
  const feed = item.pondFeed
  if (!feed || typeof feed !== "object" || Array.isArray(feed)) {
    throw new Error(`item.pondFeed invalid: ${item.id}`)
  }
  const targeted = feed.delivery === "targeted"
  if (feed.delivery !== undefined && !["feeder", "targeted"].includes(feed.delivery)) throw new Error(`item.pondFeed.delivery invalid: ${item.id}`)
  if (targeted && (feed.rulesetVersion !== "pond-feed-v2" || feed.role !== "functional" || feed.nutrition !== 0 || feed.maxPortionsPerFishHour !== undefined || feed.consumePriority !== undefined)) throw new Error(`targeted feed requires functional DSL, zero nutrition and no feeder dose/priority: ${item.id}`)
  if (typeof feed.nutrition !== "number" || !Number.isFinite(feed.nutrition) || feed.nutrition < 0 || !targeted && feed.nutrition === 0) {
    throw new Error(`item.pondFeed.nutrition invalid: ${item.id}`)
  }
  if (!["herbal", "insect", "protein", "arcane", "recycled"].includes(feed.feedClass)) {
    throw new Error(`item.pondFeed.feedClass invalid: ${item.id}`)
  }
  const hasDSL = feed.rulesetVersion !== undefined || feed.consumePriority !== undefined || feed.conditions !== undefined || feed.effects !== undefined
  if (hasDSL && feed.rulesetVersion !== "pond-feed-v2") {
    throw new Error(`item.pondFeed.rulesetVersion invalid: ${item.id}`)
  }
  const functional = feed.role === "functional" || feed.role === undefined && (feed.effects || []).length > 0
  if (feed.role !== undefined && !["staple", "functional"].includes(feed.role)) throw new Error(`item.pondFeed.role invalid: ${item.id}`)
  if (feed.role === "staple" && (feed.effects || []).length > 0) throw new Error(`staple feed cannot contain effects: ${item.id}`)
  if (feed.maxPortionsPerFishHour !== undefined && (!functional || !Number.isFinite(feed.maxPortionsPerFishHour) || feed.maxPortionsPerFishHour <= 0 || feed.maxPortionsPerFishHour > 0.5)) throw new Error(`item.pondFeed.maxPortionsPerFishHour invalid: ${item.id}`)
  if (functional && !(feed.effects || []).length) throw new Error(`functional feed requires effects: ${item.id}`)
  if (functional && Array.isArray(feed.effects) && !feed.effects.some(effect => effect?.valuePerPortion > 0)) throw new Error(`functional feed requires a positive effect: ${item.id}`)
  if (feed.consumePriority !== undefined && (!Number.isInteger(feed.consumePriority) || feed.consumePriority < -100 || feed.consumePriority > 100)) {
    throw new Error(`item.pondFeed.consumePriority invalid: ${item.id}`)
  }
  if (feed.conditions !== undefined) {
    if (!Array.isArray(feed.conditions)) throw new Error(`item.pondFeed.conditions invalid: ${item.id}`)
    const numericFields = new Map([
      ["health", [0, 1]],
      ["satiety", [0, 100]],
      ["condition", [0, 1.12]]
    ])
    const stringFields = new Map([
      ["dietClass", new Set(["herbal", "insect", "protein", "arcane"])],
      ["pondZone", new Set(["main", "nursery"])]
    ])
    for (const [index, condition] of feed.conditions.entries()) {
      if (!condition || typeof condition !== "object" || Array.isArray(condition)) throw new Error(`item.pondFeed.conditions invalid: ${item.id}.${index}`)
      const field = condition.field
      const op = condition.op
      if (numericFields.has(field)) {
        if (!["eq", "neq", "lt", "lte", "gt", "gte"].includes(op)) throw new Error(`item.pondFeed.conditions.op invalid: ${item.id}.${index}`)
        const value = condition.value
        const [minimum, maximum] = numericFields.get(field)
        if (typeof value !== "number" || !Number.isFinite(value) || value < minimum || value > maximum) {
          throw new Error(`item.pondFeed.conditions.value invalid: ${item.id}.${index}`)
        }
      } else if (stringFields.has(field)) {
        if (!["eq", "neq"].includes(op) || typeof condition.value !== "string" || !stringFields.get(field).has(condition.value)) {
          throw new Error(`item.pondFeed.conditions invalid: ${item.id}.${index}`)
        }
      } else {
        throw new Error(`item.pondFeed.conditions.field invalid: ${item.id}.${index}`)
      }
    }
  }
  if (feed.effects !== undefined) {
    if (!Array.isArray(feed.effects)) throw new Error(`item.pondFeed.effects invalid: ${item.id}`)
    if (feed.effects.length > 4 || new Set(feed.effects.map(effect => effect?.stat)).size !== feed.effects.length) throw new Error(`pond feed effects require unique stats, at most four: ${item.id}`)
    for (const [index, effect] of feed.effects.entries()) {
      if (!effect || typeof effect !== "object" || Array.isArray(effect) || effect.kind !== "stat_delta") {
        throw new Error(`item.pondFeed.effects.kind invalid: ${item.id}.${index}`)
      }
      if (!["lengthCm", "weightKg", "health", "satiety", "longevityDays", "naturalProtectionDays"].includes(effect.stat)) {
        throw new Error(`item.pondFeed.effects.stat invalid: ${item.id}.${index}`)
      }
      if (typeof effect.valuePerPortion !== "number" || !Number.isFinite(effect.valuePerPortion) || effect.valuePerPortion === 0) {
        throw new Error(`item.pondFeed.effects.valuePerPortion invalid: ${item.id}.${index}`)
      }
      if (effect.mode === "bonus_band") {
        if (!["lengthCm", "weightKg"].includes(effect.stat) || effect.valuePerPortion <= 0 || effect.valuePerPortion > 0.2) {
          throw new Error(`item.pondFeed.effects bonus_band invalid: ${item.id}.${index}`)
        }
        if (typeof effect.maxTargetMultiplier !== "number" || !Number.isFinite(effect.maxTargetMultiplier) || effect.maxTargetMultiplier <= 1 || effect.maxTargetMultiplier > 1.15) {
          throw new Error(`item.pondFeed.effects.maxTargetMultiplier invalid: ${item.id}.${index}`)
        }
        if (effect.floor !== undefined || effect.ceiling !== undefined) throw new Error(`item.pondFeed.effects bonus_band bounds unsupported: ${item.id}.${index}`)
      } else if (effect.mode === "flat" && ["longevityDays", "naturalProtectionDays"].includes(effect.stat)) {
        if (!targeted || effect.valuePerPortion <= 0 || effect.valuePerPortion > 36500 || effect.floor !== undefined || effect.ceiling !== undefined || effect.maxTargetMultiplier !== undefined) throw new Error(`life effects require targeted positive days <= 36500 without bounds: ${item.id}`)
      } else if (effect.mode === "flat") {
        if (!["health", "satiety"].includes(effect.stat) || effect.maxTargetMultiplier !== undefined) {
          throw new Error(`item.pondFeed.effects flat invalid: ${item.id}.${index}`)
        }
        const [minimum, maximum, maxMagnitude] = effect.stat === "health" ? [0, 1, 0.1] : [0, 100, 25]
        if (effect.valuePerPortion < 0 && effect.floor === undefined) throw new Error(`negative pond feed effect requires floor: ${item.id}`)
        if (Math.abs(effect.valuePerPortion) > maxMagnitude) throw new Error(`item.pondFeed.effects flat magnitude invalid: ${item.id}.${index}`)
        for (const key of ["floor", "ceiling"]) {
          if (effect[key] !== undefined && (typeof effect[key] !== "number" || !Number.isFinite(effect[key]) || effect[key] < minimum || effect[key] > maximum)) {
            throw new Error(`item.pondFeed.effects.${key} invalid: ${item.id}.${index}`)
          }
        }
        if (effect.floor !== undefined && effect.ceiling !== undefined && effect.floor > effect.ceiling) {
          throw new Error(`item.pondFeed.effects bounds inverted: ${item.id}.${index}`)
        }
      } else {
        throw new Error(`item.pondFeed.effects.mode invalid: ${item.id}.${index}`)
      }
    }
  }
}

function validateItems(items, iconIdSet) {
  if (!Array.isArray(items)) throw new Error("items must be an array")
  const ids = new Set()
  for (const item of items) {
    if (!item || typeof item !== "object") throw new Error("item must be object")
    if (!item.id || typeof item.id !== "string") throw new Error("item.id is required string")
    if (ids.has(item.id)) throw new Error(`duplicate item id: ${item.id}`)
    ids.add(item.id)
    if (!Array.isArray(item.type) || item.type.length === 0) throw new Error(`item.type invalid: ${item.id}`)
    for (const itemType of item.type) {
      if (typeof itemType !== "string" || itemType.trim() === "") throw new Error(`item.type invalid: ${item.id}`)
    }
    if (!item.rarity || typeof item.rarity !== "string") throw new Error(`item.rarity invalid: ${item.id}`)
    if (typeof item.skillForgeEligible !== "boolean") throw new Error(`item.skillForgeEligible invalid: ${item.id}`)
    if (typeof item.category !== "string" || item.category.trim() === "") throw new Error(`item.category invalid: ${item.id}`)
    if (typeof item.isEquipment !== "boolean") throw new Error(`item.isEquipment invalid: ${item.id}`)
    if (item.disabled !== undefined && typeof item.disabled !== "boolean") {
      throw new Error(`item.disabled invalid: ${item.id}`)
    }
    if (typeof item.stackable !== "boolean") {
      throw new Error(`item.stackable invalid: ${item.id}`)
    }
    if (typeof item.carryableInCombat !== "boolean") {
      throw new Error(`item.carryableInCombat invalid: ${item.id}`)
    }
    if (typeof item.marketListable !== "boolean") {
      throw new Error(`item.marketListable invalid: ${item.id}`)
    }
    if (!item.baseAttrs || typeof item.baseAttrs !== "object") {
      throw new Error(`item.baseAttrs invalid: ${item.id}`)
    }
    for (const [k, v] of Object.entries(item.baseAttrs)) {
      if (!k || typeof k !== "string") throw new Error(`item.baseAttrs key invalid: ${item.id}`)
      if (typeof v !== "number") throw new Error(`item.baseAttrs value invalid: ${item.id}.${k}`)
    }
    if (item.economyBaseValue !== undefined) {
      if (
        typeof item.economyBaseValue !== "number" ||
        !Number.isFinite(item.economyBaseValue) ||
        item.economyBaseValue < 0 ||
        !Number.isInteger(item.economyBaseValue)
      ) {
        const source = formatRowSource(item, item.id)
        throw new Error(`item.economyBaseValue invalid: ${item.id} value=${JSON.stringify(item.economyBaseValue)} ${source}`)
      }
    }
    if (item.useGrantSkillId !== undefined && (typeof item.useGrantSkillId !== "string" || item.useGrantSkillId === "")) {
      throw new Error(`item.useGrantSkillId invalid: ${item.id}`)
    }
    if (item.useGrantCollectibleId !== undefined && (typeof item.useGrantCollectibleId !== "string" || item.useGrantCollectibleId === "")) {
      throw new Error(`item.useGrantCollectibleId invalid: ${item.id}`)
    }
    if (item.useOperationEffectId !== undefined && (typeof item.useOperationEffectId !== "string" || item.useOperationEffectId === "")) {
      throw new Error(`item.useOperationEffectId invalid: ${item.id}`)
    }
    if (item.battleUsableItemId !== undefined && (typeof item.battleUsableItemId !== "string" || item.battleUsableItemId === "")) {
      throw new Error(`item.battleUsableItemId invalid: ${item.id}`)
    }
    if (item.useSkillExpGain !== undefined && typeof item.useSkillExpGain !== "number") {
      throw new Error(`item.useSkillExpGain invalid: ${item.id}`)
    }
    if (item.kittyFeed !== undefined) {
      if (!item.kittyFeed || typeof item.kittyFeed !== "object" || Array.isArray(item.kittyFeed)) {
        throw new Error(`item.kittyFeed invalid: ${item.id}`)
      }
      if (!Number.isInteger(item.kittyFeed.battleLevelCapGain) || item.kittyFeed.battleLevelCapGain <= 0) {
        throw new Error(`item.kittyFeed.battleLevelCapGain invalid: ${item.id}`)
      }
      if (!item.type.includes("food")) {
        throw new Error(`item.kittyFeed requires food type: ${item.id}`)
      }
    }
    if (item.pondFeed !== undefined) {
      validatePondFeed(item)
    }
    if (item.alchemy?.convertWeight !== undefined && (
      typeof item.alchemy.convertWeight !== "number" ||
      !Number.isFinite(item.alchemy.convertWeight) ||
      item.alchemy.convertWeight <= 0
    )) {
      throw new Error(`item.alchemy.convertWeight must be finite and > 0: ${item.id}`)
    }
    if (item.alchemy?.convertWeightMultiplier !== undefined && (
      typeof item.alchemy.convertWeightMultiplier !== "number" ||
      !Number.isFinite(item.alchemy.convertWeightMultiplier) ||
      item.alchemy.convertWeightMultiplier <= 0
    )) {
      throw new Error(`item.alchemy.convertWeightMultiplier must be finite and > 0: ${item.id}`)
    }
    if (item.iconId !== undefined) {
      if (typeof item.iconId !== "string" || item.iconId === "") throw new Error(`item.iconId invalid: ${item.id}`)
      if (!iconIdSet.has(item.iconId)) throw new Error(`item.iconId not found: ${item.id}.${item.iconId}`)
    }

    if (item.emojiIcon !== undefined && (typeof item.emojiIcon !== "string" || item.emojiIcon === "")) {
      throw new Error(`item.emojiIcon invalid: ${item.id}`)
    }
    for (const key of ["nameKey", "descKey", "effectDescKey"]) {
      const value = item[key]
      if (value !== undefined && (typeof value !== "string" || value === "")) {
        throw new Error(`item.${key} invalid: ${item.id}`)
      }
    }

  }
}

function normalizeAlchemyGroups(rows) {
  return (rows || [])
    .map((raw) => {
      const id = parseOptionalString(raw?.id)
      if (!id) return null
      return {
        ...raw,
        id,
        nameKey: parseOptionalString(raw?.nameKey) || `alchemy.group.${id}.name`,
        descKey: parseOptionalString(raw?.descKey) || `alchemy.group.${id}.desc`,
        minAlchemyLevel: Math.max(1, Math.round(parseNumber(raw?.minAlchemyLevel, 1))),
        goldCost: Math.max(0, Math.round(parseNumber(raw?.goldCost, 0))),
        durationSec: Math.max(0, parseNumber(raw?.durationSec, 0)),
        alchemyExp: Math.max(0, parseNumber(raw?.alchemyExp, 0)),
        failChanceBase: Math.max(0, Math.min(1, parseNumber(raw?.failChanceBase, 0))),
        minFailChance: Math.max(0, Math.min(1, parseNumber(raw?.minFailChance, 0))),
        upgradeChanceBase: Math.max(0, Math.min(0.25, parseNumber(raw?.upgradeChanceBase, 0))),
        upgradeEntries: Array.isArray(raw?.upgradeEntries)
          ? raw.upgradeEntries.map((entry) => ({
              itemId: parseOptionalString(entry?.itemId) || "",
              weight: Math.max(0, parseNumber(entry?.weight, 0))
            })).filter((entry) => entry.itemId && entry.weight > 0)
          : [],
        entries: Array.isArray(raw?.entries)
          ? raw.entries.map((entry) => ({
              itemId: parseOptionalString(entry?.itemId) || "",
              weight: Math.max(0, parseNumber(entry?.weight, 0))
            })).filter((entry) => entry.itemId && entry.weight > 0)
          : [],
        disabled: parseBoolean(raw?.disabled, false)
      }
    })
    .filter(Boolean)
}

function normalizeItemTraits(rows) {
  return (rows || [])
    .map((raw) => {
      const itemId = parseOptionalString(raw?.itemId)
      if (!itemId) return null
      // NOTE: nameAffix is intentionally not parsed here — it lives on
      // items.json `alchemy.nameAffix` (item-of-noun metadata) and is
      // accessed via Data.ItemAlchemyNameAffix(itemId) at runtime.
      return {
        ...raw,
        itemId,
        traits: Array.isArray(raw?.traits)
          ? raw.traits.map((trait) => {
              if (!trait || typeof trait !== "object" || Array.isArray(trait)) return null
              const effectsRaw = Array.isArray(trait?.effects) ? trait.effects : []
              const effects = effectsRaw
                .map((eff) => {
                  if (!eff || typeof eff !== "object" || Array.isArray(eff)) return null
                  const attr = parseOptionalString(eff.attr)
                  if (!attr) return null
                  return {
                    attr,
                    valuePerUnit: {
                      min: parseNumber(eff?.valuePerUnit?.min, 0),
                      max: parseNumber(eff?.valuePerUnit?.max, 0)
                    }
                  }
                })
                .filter(Boolean)
              return {
                traitId: parseOptionalString(trait?.traitId) || "",
                kind: parseOptionalString(trait?.kind) || "",
                family: parseOptionalString(trait?.family) || "",
                effectType: parseOptionalString(trait?.effectType) || "add_stat",
                effects,
                unitBasis: Math.max(1, Math.round(parseNumber(trait?.unitBasis, 1))),
                maxBasisMultiplier: Math.max(1, Math.round(parseNumber(trait?.maxBasisMultiplier, 1))),
                minAlchemyLevel: Math.max(1, Math.round(parseNumber(trait?.minAlchemyLevel, 1))),
                failBaseChance: Math.max(0, Math.min(1, parseNumber(trait?.failBaseChance, 0))),
                failChancePerLevelAbove: Math.max(0, parseNumber(trait?.failChancePerLevelAbove, 0)),
                failChancePerExtraBasis: Math.max(0, parseNumber(trait?.failChancePerExtraBasis, 0)),
                failChanceQuantityCap: Math.max(0, Math.min(1, parseNumber(trait?.failChanceQuantityCap, 0))),
                goldCostPerUnit: Math.max(0, Math.round(parseNumber(trait?.goldCostPerUnit, 0))),
                durationSecPerUnit: Math.max(0, parseNumber(trait?.durationSecPerUnit, 0)),
                alchemyExpPerUnit: Math.max(0, parseNumber(trait?.alchemyExpPerUnit, 0)),
                maxEffectivePerEquipment: Math.max(1, Math.round(parseNumber(trait?.maxEffectivePerEquipment, 2))),
                maxEffectivePerBody: Math.max(1, Math.round(parseNumber(trait?.maxEffectivePerBody, 8)))
              }
            }).filter((trait) => trait && trait.traitId)
          : []
      }
    })
    .filter(Boolean)
}

function normalizeSkillAlchemyModules(rows) {
  return (rows || [])
    .map((raw) => {
      const itemId = parseOptionalString(raw?.itemId)
      if (!itemId) return null
      const normalizePorts = (ports) => (Array.isArray(ports) ? ports : [])
        .map((port) => ({
          id: parseOptionalString(port?.id) || "",
          type: parseOptionalString(port?.type) || ""
        }))
        .filter((port) => port.id && port.type)
      return {
        id: parseOptionalString(raw?.id) || `skill_forge.${itemId}`,
        itemId,
        nodeKind: parseOptionalString(raw?.nodeKind) || "material",
        typedPorts: {
          inputs: normalizePorts(raw?.typedPorts?.inputs),
          outputs: normalizePorts(raw?.typedPorts?.outputs)
        },
        compileOp: raw?.compileOp && typeof raw.compileOp === "object" && !Array.isArray(raw.compileOp)
          ? { ...raw.compileOp, kind: parseOptionalString(raw.compileOp.kind) || "" }
          : { kind: "" },
        valuePerUnit: {
          min: parseNumber(raw?.valuePerUnit?.min, 0),
          max: parseNumber(raw?.valuePerUnit?.max, 0)
        },
        unitBasis: Math.max(1, Math.round(parseNumber(raw?.unitBasis, 1))),
        maxBasisMultiplier: Math.max(1, Math.round(parseNumber(raw?.maxBasisMultiplier, 1))),
        minAlchemyLevel: Math.max(1, Math.round(parseNumber(raw?.minAlchemyLevel, 1))),
        failBaseChance: Math.max(0, Math.min(1, parseNumber(raw?.failBaseChance, 0))),
        failChancePerLevelAbove: Math.max(0, parseNumber(raw?.failChancePerLevelAbove, 0)),
        successChanceModifier: parseNumber(raw?.successChanceModifier, 0),
        budgetCreditPerUnit: Math.max(0, parseNumber(raw?.budgetCreditPerUnit, 0)),
        nameParts: raw?.nameParts && typeof raw.nameParts === "object" && !Array.isArray(raw.nameParts) ? { ...raw.nameParts } : {},
        icon: parseOptionalString(raw?.icon) || "",
        disabled: parseBoolean(raw?.disabled, false),
        lowRarityWhitelist: parseBoolean(raw?.lowRarityWhitelist, false)
      }
    })
    .filter(Boolean)
}

// Success chance is balanced per material batch, not per individual item. Keep
// the policy here so every newly added loot module must make the same explicit
// power-versus-reliability tradeoff as the current catalog.
function expectedSkillForgeSuccessModifier(row, rarity) {
  const kind = row.compileOp.kind
  const direction = row.compileOp.direction
  const numericPenalty = {
    common: -0.01,
    uncommon: -0.015,
    rare: -0.025,
    epic: -0.04,
    legendary: -0.06,
    mythic: -0.09
  }[rarity]
  const tradeoffCredit = {
    common: 0.04,
    uncommon: 0.04,
    rare: 0.05,
    epic: 0.06,
    legendary: 0.07,
    mythic: 0.08
  }[rarity]

  if (["formula_term", "flat_amount"].includes(kind)) return numericPenalty
  if (["hp_cost", "mp_cost", "cooldown"].includes(kind) && direction === "increase") return tradeoffCredit
  if (kind === "self_current_hp_loss") return tradeoffCredit
  if (kind === "effect_slot") return -0.20
  if (kind === "hit_count") return -0.15
  if (["target_count", "target_count_max"].includes(kind)) return -0.12
  if (kind === "cooldown" && direction === "reduce") return -0.12
  if (kind === "skill_level_term" || kind === "requirement_reduction") return -0.10
  if (["target_scope", "target_group", "effect_chain"].includes(kind)) return -0.10
  if (["target_position", "target_direction", "select_rule"].includes(kind)) return -0.08
  if (["caster_status_condition", "target_status_condition", "consume_caster_status", "consume_target_status"].includes(kind)) return -0.08
  if (["status_apply", "status_duration"].includes(kind)) return -0.06
  if (["hp_cost", "mp_cost"].includes(kind) && direction === "reduce") return -0.07
  throw new Error(`skill forge success policy missing compile operation: ${row.id}.${kind}`)
}

function validateSkillAlchemyModules(rows, items, rarityDefinitions, attributes, buffs, explicitSkillForgeOptOutItemIDs = new Set()) {
  const itemById = new Map(items.map((item) => [item.id, item]))
  const attributeIDs = new Set((attributes || []).map((row) => row.id))
  const workAttributeIDs = new Set((attributes || []).filter((row) => row.category === "work" && row.growthType === "levelable").map((row) => row.id))
  const buffByID = new Map((buffs || []).map((row) => [row.id, row]))
  const buffIDs = new Set(buffByID.keys())
  const rarityOrder = new Map(rarityDefinitions.map((row) => [row.id, Number(row.order || 0)]))
  const rareOrder = rarityOrder.get("rare")
  if (!Number.isFinite(rareOrder)) throw new Error("skill_alchemy_modules requires rare rarity definition")
  const activeModuleItems = new Set()
  const ids = new Set()
  const allowedOps = new Set([
    "requirement_reduction", "formula_term", "skill_level_term", "self_current_hp_loss", "flat_amount", "target_count", "target_count_max", "target_direction", "hit_count", "target_scope", "target_position", "target_group",
    "select_rule", "status_apply", "status_duration", "effect_chain", "caster_status_condition",
    "target_status_condition", "consume_caster_status", "consume_target_status", "effect_slot", "hp_cost", "mp_cost", "cooldown"
  ])
  const allowedPortTypes = new Set(["flow", "effect", "skill", "condition"])
  const opKindsByNodeKind = new Map([
    ["formula", new Set(["formula_term", "skill_level_term", "self_current_hp_loss", "flat_amount"])],
    ["target", new Set(["target_count", "target_count_max", "target_direction", "target_scope", "target_position", "select_rule", "target_group"])],
    ["flow", new Set(["hit_count", "effect_chain"])],
    ["status", new Set(["status_apply", "status_duration"])],
    ["condition", new Set(["caster_status_condition", "target_status_condition", "consume_caster_status", "consume_target_status"])],
    ["cost", new Set(["requirement_reduction", "hp_cost", "mp_cost", "cooldown"])],
    ["structure", new Set(["effect_slot"])]
  ])
  const portTypesByNodeKind = new Map([
    ["formula", ["effect", "effect"]], ["target", ["effect", "effect"]], ["flow", ["effect", "effect"]],
    ["status", ["effect", "effect"]], ["condition", ["condition", "effect"]], ["cost", ["skill", "skill"]],
    ["structure", ["skill", "skill"]]
  ])
  const allowedNameParts = new Set([
    "skillForge.name.damage", "skillForge.name.arcane", "skillForge.name.multicast", "skillForge.name.reach",
    "skillForge.name.echo", "skillForge.name.lingering", "skillForge.name.chain", "skillForge.name.heal",
    "skillForge.name.swift", "skillForge.name.hunt", "skillForge.name.guard", "skillForge.name.precise",
    "skillForge.name.costly", "skillForge.name.efficient", "skillForge.name.conditional", "skillForge.name.blessing",
    "skillForge.name.shadow", "skillForge.name.break", "skillForge.name.consume", "skillForge.name.sacrifice"
  ])
  for (const row of rows) {
    if (ids.has(row.id)) throw new Error(`duplicate skill_alchemy_modules.id: ${row.id}`)
    ids.add(row.id)
    const item = itemById.get(row.itemId)
    const itemTypes = new Set(item?.type || [])
    if (!item || item.disabled || (!itemTypes.has("material") && !itemTypes.has("loot"))) {
      throw new Error(`skill_alchemy_modules.itemId must reference enabled loot or material: ${row.itemId}`)
    }
    if (!row.disabled) {
      if (activeModuleItems.has(row.itemId)) {
        throw new Error(`skill_alchemy_modules allows only one active effect per item: ${row.itemId}`)
      }
      activeModuleItems.add(row.itemId)
    }
    const order = rarityOrder.get(item.rarity)
    if (Number.isFinite(order) && order < rareOrder && !row.lowRarityWhitelist) {
      throw new Error(`low rarity skill alchemy module must be explicitly whitelisted: ${row.itemId}`)
    }
    if (!allowedOps.has(row.compileOp.kind)) throw new Error(`skill_alchemy_modules.compileOp.kind invalid: ${row.id}`)
    if (!opKindsByNodeKind.get(row.nodeKind)?.has(row.compileOp.kind)) {
      throw new Error(`skill_alchemy_modules nodeKind/compileOp mismatch: ${row.id}`)
    }
    if (row.valuePerUnit.min > row.valuePerUnit.max) throw new Error(`skill_alchemy_modules.valuePerUnit invalid: ${row.id}`)
    if (row.unitBasis < 1) throw new Error(`skill_alchemy_modules.unitBasis invalid: ${row.id}`)
    if (!row.icon) throw new Error(`skill_alchemy_modules.icon missing: ${row.id}`)
    for (const side of ["inputs", "outputs"]) {
      const ports = row.typedPorts[side]
      if (ports.length === 0) throw new Error(`skill_alchemy_modules.typedPorts.${side} empty: ${row.id}`)
      const seen = new Set()
      for (const port of ports) {
        if (seen.has(port.id)) throw new Error(`skill_alchemy_modules duplicate port: ${row.id}.${side}.${port.id}`)
        seen.add(port.id)
        if (!allowedPortTypes.has(port.type)) throw new Error(`skill_alchemy_modules port type invalid: ${row.id}.${port.id}`)
      }
    }
    const expectedPorts = portTypesByNodeKind.get(row.nodeKind)
    const isBranchCondition = ["caster_status_condition", "target_status_condition"].includes(row.compileOp.kind)
    const expectedOutputCount = isBranchCondition ? 2 : 1
    if (!expectedPorts || row.typedPorts.inputs.length !== 1 || row.typedPorts.outputs.length !== expectedOutputCount || row.typedPorts.inputs[0].type !== expectedPorts[0] || row.typedPorts.outputs.some((port) => port.type !== expectedPorts[1])) {
      throw new Error(`skill_alchemy_modules ports do not match nodeKind: ${row.id}`)
    }
    if (isBranchCondition && (!row.typedPorts.outputs.some((port) => port.id === "true") || !row.typedPorts.outputs.some((port) => port.id === "false"))) {
      throw new Error(`skill_alchemy_modules condition ports must expose true and false: ${row.id}`)
    }
    const expectedCoreKey = item.nameKey || `item.${item.id}.name`
    if (row.nameParts.coreKey !== expectedCoreKey || !allowedNameParts.has(row.nameParts.suffixKey)) {
      throw new Error(`skill_alchemy_modules.nameParts i18n invalid: ${row.id}`)
    }
    if (row.compileOp.kind === "formula_term" && !attributeIDs.has(row.compileOp.attrKey)) {
      throw new Error(`skill_alchemy_modules.compileOp.attrKey not found: ${row.id}.${row.compileOp.attrKey}`)
    }
    if (row.compileOp.kind === "formula_term" && row.compileOp.effectKind === "heal" && !["intelligence", "perception"].includes(row.compileOp.attrKey) && !workAttributeIDs.has(row.compileOp.attrKey)) {
      throw new Error(`skill_alchemy_modules heal formula only supports intelligence/perception/work attributes: ${row.id}.${row.compileOp.attrKey}`)
    }
    if (["status_apply", "caster_status_condition", "target_status_condition", "consume_caster_status", "consume_target_status"].includes(row.compileOp.kind) && !buffIDs.has(row.compileOp.buffId)) {
      throw new Error(`skill_alchemy_modules.compileOp.buffId not found: ${row.id}.${row.compileOp.buffId}`)
    }
    if (row.compileOp.kind === "status_apply" && (!Number.isInteger(row.compileOp.durationBaseTurns) || row.compileOp.durationBaseTurns < 1)) {
      throw new Error(`skill_alchemy_modules.compileOp.durationBaseTurns invalid: ${row.id}`)
    }
    if (["hp_cost", "mp_cost", "cooldown"].includes(row.compileOp.kind) && !["increase", "reduce"].includes(row.compileOp.direction)) {
      throw new Error(`skill_alchemy_modules.compileOp.direction invalid: ${row.id}`)
    }
    if (["target_scope", "target_group"].includes(row.compileOp.kind) && !["enemy", "ally", "all", "self"].includes(row.compileOp.scope)) {
      throw new Error(`skill_alchemy_modules.compileOp.scope invalid: ${row.id}`)
    }
    if (
      ["target_scope", "target_group"].includes(row.compileOp.kind)
      && ((!row.disabled && typeof row.compileOp.excludeSelf !== "boolean")
        || (row.compileOp.excludeSelf !== undefined && typeof row.compileOp.excludeSelf !== "boolean"))
    ) {
      throw new Error(`skill_alchemy_modules.compileOp.excludeSelf invalid: ${row.id}`)
    }
    if (row.compileOp.kind === "target_direction" && !["front_to_rear", "rear_to_front", "random_alive", "none"].includes(row.compileOp.seek)) {
      throw new Error(`skill_alchemy_modules.compileOp.seek invalid: ${row.id}`)
    }
    if (row.compileOp.kind === "select_rule" && !["random", "first", "boss_first", "hp_max", "hp_min", "hp_percent_max", "hp_percent_min", "attr_max", "attr_min"].includes(row.compileOp.rule)) {
      throw new Error(`skill_alchemy_modules.compileOp.rule invalid: ${row.id}`)
    }
    if (row.compileOp.kind === "select_rule" && ["attr_max", "attr_min"].includes(row.compileOp.rule) && !attributeIDs.has(row.compileOp.attrKey)) {
      throw new Error(`skill_alchemy_modules.compileOp.attrKey not found: ${row.id}.${row.compileOp.attrKey}`)
    }
    if (!row.disabled) {
      const isLoot = (item.type || []).includes("loot") || item.category === "loot"
      const isExplicitLootOptOut = isLoot && explicitSkillForgeOptOutItemIDs.has(item.id)
      const isExplicitLifeOutput = item.skillForgeEligible === true && (item.type || []).includes("material")
      if (isExplicitLootOptOut) {
        throw new Error(`active skill alchemy module conflicts with skillForgeEligible=false: ${row.id}.${row.itemId}`)
      }
      if (!isLoot && !isExplicitLifeOutput) {
        throw new Error(`active skill alchemy module must use loot or an explicitly eligible life output: ${row.id}.${row.itemId}`)
      }
      if (isExplicitLifeOutput && Number(order || 0) < Number(rarityOrder.get("epic") || 0)) {
        throw new Error(`skill forge life output must be epic or higher: ${row.id}.${row.itemId}`)
      }
      if (row.compileOp.kind === "status_apply") {
        if (row.valuePerUnit.min !== 0 || row.valuePerUnit.max !== 0) {
          throw new Error(`skill forge status chance is owned by global config, valuePerUnit must be zero: ${row.id}`)
        }
        const buff = buffByID.get(row.compileOp.buffId)
        const durationTurns = Number(buff?.durationTurns || 0)
        const kind = String(buff?.kind || "")
        if (!buff || buff.disabled || buff.permanent === true || !Number.isInteger(durationTurns) || durationTurns < 1 || durationTurns > 5 || buff.dispellable !== true || !["buff", "debuff"].includes(kind)) {
          throw new Error(`skill forge status material must reference a temporary dispellable combat status: ${row.id}.${row.compileOp.buffId}`)
        }
        const scope = String(row.compileOp.targetScope || "")
        if ((kind === "buff" && !["ally", "self"].includes(scope)) || (kind === "debuff" && scope !== "enemy")) {
          throw new Error(`skill forge status material target scope does not match buff kind: ${row.id}.${row.compileOp.buffId}`)
        }
      }
      const preciousMinimums = new Map([
        ["target_count", "mythic"], ["target_count_max", "mythic"], ["hit_count", "mythic"], ["effect_slot", "mythic"],
        ["target_position", "legendary"], ["target_direction", "rare"], ["select_rule", "rare"], ["requirement_reduction", "legendary"], ["skill_level_term", "mythic"],
        ["target_scope", "epic"], ["target_group", "epic"], ["effect_chain", "epic"], ["status_apply", "epic"]
      ])
      let minimumRarity = preciousMinimums.get(row.compileOp.kind)
      if (["hp_cost", "mp_cost", "cooldown"].includes(row.compileOp.kind) && row.compileOp.direction === "reduce") {
        minimumRarity = row.compileOp.kind === "cooldown" ? "mythic" : "epic"
      }
      if (minimumRarity && Number(order || 0) < Number(rarityOrder.get(minimumRarity) || 0)) {
        throw new Error(`precious skill alchemy parameter requires ${minimumRarity}+ material: ${row.id}.${row.itemId}`)
      }
      const fixedOps = new Set(["target_scope", "target_group", "target_position", "target_direction", "select_rule", "caster_status_condition", "target_status_condition", "consume_caster_status", "consume_target_status", "effect_chain"])
      const rarityUnitBasis = { common: 1000, uncommon: 1000, rare: 1000, epic: 400, legendary: 200, mythic: 15 }[item.rarity]
      const expectedUnitBasis = rarityUnitBasis * (fixedOps.has(row.compileOp.kind) ? 5 : 1)
      if (!expectedUnitBasis || row.unitBasis !== expectedUnitBasis) {
        throw new Error(`skill alchemy material step must match rarity and effect kind (${item.rarity}=${expectedUnitBasis}): ${row.id}.${row.itemId}`)
      }
      if (row.maxBasisMultiplier > 5) {
        throw new Error(`skill alchemy material maxBasisMultiplier must be 1..5: ${row.id}.${row.itemId}`)
      }
      if (row.minAlchemyLevel !== 1 || row.failChancePerLevelAbove !== 0) {
        throw new Error(`skill forge modules cannot use alchemy-level corrections: ${row.id}.${row.itemId}`)
      }
      const successTradeoff = (
        (["hp_cost", "mp_cost", "cooldown"].includes(row.compileOp.kind) && row.compileOp.direction === "increase")
        || row.compileOp.kind === "self_current_hp_loss"
      )
      if (!Number.isFinite(row.successChanceModifier) || Math.abs(row.successChanceModifier) > 0.2 || Math.abs(row.successChanceModifier) < 0.000001) {
        throw new Error(`skill forge successChanceModifier must be non-zero and within -0.2..0.2: ${row.id}.${row.itemId}`)
      }
      if ((successTradeoff && row.successChanceModifier <= 0) || (!successTradeoff && row.successChanceModifier >= 0)) {
        throw new Error(`skill forge successChanceModifier sign does not match its risk or tradeoff: ${row.id}.${row.itemId}`)
      }
      const expectedSuccessModifier = expectedSkillForgeSuccessModifier(row, item.rarity)
      if (Math.abs(row.successChanceModifier - expectedSuccessModifier) > 0.000001) {
        throw new Error(`skill forge successChanceModifier must follow the per-batch balance policy (${expectedSuccessModifier}): ${row.id}.${row.itemId}`)
      }
      if (["common", "uncommon"].includes(item.rarity)) {
        const lowRarityNumeric = ["formula_term", "flat_amount"].includes(row.compileOp.kind)
        const lowRarityTradeoff = ["hp_cost", "mp_cost", "cooldown"].includes(row.compileOp.kind) && row.compileOp.direction === "increase"
        if (!lowRarityNumeric && !lowRarityTradeoff) {
          throw new Error(`low rarity skill forge material must provide a small numeric effect or cost tradeoff: ${row.id}.${row.itemId}`)
        }
      }
      if (fixedOps.has(row.compileOp.kind) && (row.maxBasisMultiplier !== 1 || row.valuePerUnit.min !== 0 || row.valuePerUnit.max !== 0)) {
        throw new Error(`fixed skill forge option must consume exactly one batch and provide no numeric range: ${row.id}.${row.itemId}`)
      }
      const integerOps = new Set(["target_count", "target_count_max", "hit_count", "status_duration", "hp_cost", "mp_cost", "cooldown"])
      if (integerOps.has(row.compileOp.kind)) {
        const stepMin = row.valuePerUnit.min * row.unitBasis
        const stepMax = row.valuePerUnit.max * row.unitBasis
        if (Math.abs(stepMin - 1) > 0.000001 || Math.abs(stepMax - 1) > 0.000001) {
          throw new Error(`integer skill forge parameter must change by exactly one per batch: ${row.id}.${row.itemId}`)
        }
      }
      const formulaCaps = { common: 0.004, uncommon: 0.006, rare: 0.01, epic: 0.02, legendary: 0.04, mythic: 0.08 }
      if (["formula_term", "skill_level_term", "self_current_hp_loss"].includes(row.compileOp.kind)) {
        const batchMax = Math.abs(row.valuePerUnit.max * row.unitBasis)
        if (batchMax > formulaCaps[item.rarity] + 0.000001) {
          throw new Error(`skill forge formula batch exceeds ${item.rarity} cap ${formulaCaps[item.rarity]}: ${row.id}.${row.itemId}`)
        }
      }
    }
  }
  for (const item of items) {
    if (item.disabled) continue
    const isLoot = (item.type || []).includes("loot") || item.category === "loot"
    const isEligibleLoot = isLoot && !explicitSkillForgeOptOutItemIDs.has(item.id)
    const isExplicitLifeOutput = item.skillForgeEligible === true && (item.type || []).includes("material")
    if ((isEligibleLoot || isExplicitLifeOutput) && !activeModuleItems.has(item.id)) {
      throw new Error(`eligible skill forge material missing active module: ${item.id}`)
    }
  }
  const coverageChecks = [
    ["requirements", (row) => row.compileOp.kind === "requirement_reduction"],
    ["hp_cost", (row) => row.compileOp.kind === "hp_cost"],
    ["mp_cost", (row) => row.compileOp.kind === "mp_cost"],
    ["cooldown", (row) => row.compileOp.kind === "cooldown"],
    ["conditions", (row) => ["caster_status_condition", "target_status_condition", "consume_caster_status", "consume_target_status"].includes(row.compileOp.kind)],
    ["target_scope", (row) => ["target_scope", "target_group"].includes(row.compileOp.kind)],
    ["target_count", (row) => row.compileOp.kind === "target_count"],
    ["target_count_max", (row) => row.compileOp.kind === "target_count_max"],
    ["target_direction", (row) => row.compileOp.kind === "target_direction"],
    ["select_rule", (row) => row.compileOp.kind === "select_rule"],
    ["target_positions", (row) => row.compileOp.kind === "target_position"],
    ["damage_formula", (row) => ["formula_term", "skill_level_term", "flat_amount"].includes(row.compileOp.kind) && row.compileOp.effectKind === "damage"],
    ["heal_formula", (row) => ["formula_term", "skill_level_term", "flat_amount"].includes(row.compileOp.kind) && row.compileOp.effectKind === "heal"],
    ["resource_formula", (row) => row.compileOp.kind === "self_current_hp_loss"],
    ["hit_count", (row) => row.compileOp.kind === "hit_count"],
    ["status_apply", (row) => row.compileOp.kind === "status_apply"],
    ["status_duration", (row) => row.compileOp.kind === "status_duration"],
    ["effect_chain", (row) => row.compileOp.kind === "effect_chain"]
  ]
  for (const [parameter, matches] of coverageChecks) {
    const count = rows.filter((row) => !row.disabled && matches(row)).length
    if (count < 1) throw new Error(`skill_alchemy_modules must provide a material for ${parameter}; got ${count}`)
  }
}

function validateSkillForgeSkillbooks(items, skills) {
  const skillIDs = new Set((skills || []).filter((row) => !row.disabled).map((row) => row.id))
  const output = items.find((item) => item.id === "forged_skillbook")
  if (!output || output.disabled || output.stackable !== false || output.marketListable !== false || Number(output.economyBaseValue || 0) !== 0 || !(output.type || []).includes("skillbook") || output.useGrantSkillId) {
    throw new Error("forged_skillbook must be enabled, non-stackable, non-listable, valueless, and have no static granted skill")
  }
  for (const item of items) {
    if (item.disabled || !(item.type || []).includes("skillbook") || item.id === output.id || !item.useGrantSkillId) continue
    if (!skillIDs.has(item.useGrantSkillId)) {
      throw new Error(`skillbook useGrantSkillId not found: ${item.id}.${item.useGrantSkillId}`)
    }
  }
}

function normalizeAlchemyComboRecipes(rows) {
  return (rows || [])
    .map((raw) => {
      const id = parseOptionalString(raw?.id)
      if (!id) return null
      const outputAffixRaw = raw?.outputAffix && typeof raw.outputAffix === "object" && !Array.isArray(raw.outputAffix) ? raw.outputAffix : {}
      const effectType = parseOptionalString(outputAffixRaw.effectType) || ""
      const effectTypeKey = effectType.trim().toLowerCase().replaceAll("-", "_")
      const rawParams = outputAffixRaw.params && typeof outputAffixRaw.params === "object" && !Array.isArray(outputAffixRaw.params) ? outputAffixRaw.params : {}
      const params = { ...rawParams }
      if (effectTypeKey === "battle_trigger" || effectTypeKey === "battle_hook") {
        const hooks = normalizeBattleHookParams(rawParams?.battleHooks || rawParams?.battle_hooks || rawParams?.hooks)
        if (Array.isArray(hooks) && hooks.length > 0) params.battleHooks = hooks
      }
      return {
        ...raw,
        id,
        nameKey: parseOptionalString(raw?.nameKey) || `alchemy.recipe.${id}.name`,
        descKey: parseOptionalString(raw?.descKey) || `alchemy.recipe.${id}.desc`,
        summaryKey: parseOptionalString(raw?.summaryKey) || "",
        minAlchemyLevel: Math.max(1, Math.round(parseNumber(raw?.minAlchemyLevel, 1))),
        kind: parseOptionalString(raw?.kind) || "",
        inputs: parseNumberMap(raw?.inputs),
        outputAffix: {
          effectType,
          params,
          valueRange: {
            min: parseNumber(outputAffixRaw.valueRange?.min, 0),
            max: parseNumber(outputAffixRaw.valueRange?.max, 0)
          }
        },
        outputRarity: parseOptionalString(raw?.outputRarity) || null,
        extraGoldCost: Math.max(0, Math.round(parseNumber(raw?.extraGoldCost, 0))),
        extraDurationSec: Math.max(0, parseNumber(raw?.extraDurationSec, 0)),
        failBaseChance: Math.max(0, Math.min(1, parseNumber(raw?.failBaseChance, 0))),
        failChancePerLevelAbove: Math.max(0, parseNumber(raw?.failChancePerLevelAbove, 0)),
        priority: Math.round(parseNumber(raw?.priority, 0)),
        disabled: parseBoolean(raw?.disabled, false)
      }
    })
    .filter(Boolean)
}

function normalizeAlchemyMediums(rows) {
  return (rows || [])
    .map((raw) => {
      const id = parseOptionalString(raw?.id)
      if (!id) return null
      const itemId = parseOptionalString(raw?.itemId)
      const modsRaw = raw?.modifiers && typeof raw.modifiers === "object" && !Array.isArray(raw.modifiers) ? raw.modifiers : {}
      const familyAffinityRaw = modsRaw.familyAffinity && typeof modsRaw.familyAffinity === "object" && !Array.isArray(modsRaw.familyAffinity)
        ? modsRaw.familyAffinity : {}
      const familyAffinity = {}
      for (const [k, v] of Object.entries(familyAffinityRaw)) {
        const numeric = parseNumber(v, Number.NaN)
        if (Number.isFinite(numeric)) familyAffinity[k] = numeric
      }
      return {
        ...raw,
        id,
        itemId,
        nameKey: parseOptionalString(raw?.nameKey) || `alchemy.medium.${id}.name`,
        descKey: parseOptionalString(raw?.descKey) || `alchemy.medium.${id}.desc`,
        consumePerCraft: Math.max(1, Math.round(parseNumber(raw?.consumePerCraft, 1))),
        minAlchemyLevel: Math.max(1, Math.round(parseNumber(raw?.minAlchemyLevel, 1))),
        modifiers: {
          valueMultiplier: parseNumber(modsRaw.valueMultiplier, 1),
          negativeMultiplier: parseNumber(modsRaw.negativeMultiplier, 1),
          failChanceDelta: parseNumber(modsRaw.failChanceDelta, 0),
          successFloor: Math.max(0, Math.min(1, parseNumber(modsRaw.successFloor, 0))),
          maxTraitSlots: Math.max(0, Math.round(parseNumber(modsRaw.maxTraitSlots, 0))),
          familyAffinity
        },
        active: raw?.active !== false,
        disabled: parseBoolean(raw?.disabled, false)
      }
    })
    .filter(Boolean)
}

function validateAlchemyMediums(mediums, items) {
  const itemByID = new Map((items || []).map((item) => [item.id, item]))
  const seen = new Set()
  const allowedFamilies = new Set(["gathering", "mining", "livestock", "crafting", "sewing", "alchemy", "physical", "crit", "magic_crit", "arcane", "shadow", "brute", "defense", "apex"])
  for (const row of mediums || []) {
    if (seen.has(row.id)) throw new Error(`duplicate alchemy_mediums.id: ${row.id}`)
    seen.add(row.id)
    if (!row.itemId) throw new Error(`alchemy_mediums.itemId required: ${row.id}`)
    if (!itemByID.has(row.itemId)) throw new Error(`alchemy_mediums.itemId not found in items: ${row.id}.${row.itemId}`)
    if (row.modifiers.valueMultiplier <= 0) throw new Error(`alchemy_mediums.modifiers.valueMultiplier must be > 0: ${row.id}`)
    if (row.modifiers.negativeMultiplier < 0) throw new Error(`alchemy_mediums.modifiers.negativeMultiplier must be ≥ 0: ${row.id}`)
    const families = Object.keys(row.modifiers.familyAffinity || {})
    if (families.length > 3) throw new Error(`alchemy_mediums.familyAffinity too broad (max 3): ${row.id}`)
    for (const family of families) {
      if (!allowedFamilies.has(family)) throw new Error(`alchemy_mediums.familyAffinity unknown family: ${row.id}.${family}`)
    }
  }
}

function normalizeAlchemyTransmuteRules(rows) {
  return (rows || [])
    .map((raw) => {
      const id = parseOptionalString(raw?.id)
      if (!id) return null
      return {
        ...raw,
        id,
        itemIds: Array.isArray(raw?.itemIds)
          ? raw.itemIds.map((value) => parseOptionalString(value) || "").filter(Boolean)
          : [],
        priority: Math.round(parseNumber(raw?.priority, 0)),
        default: parseBoolean(raw?.default, false),
        returnRatio: {
          minRatio: parseNumber(raw?.returnRatio?.minRatio, 0.8),
          maxRatio: parseNumber(raw?.returnRatio?.maxRatio, 0.95)
        },
        failChanceBase: Math.max(0, Math.min(1, parseNumber(raw?.failChanceBase, 0))),
        minFailChance: Math.max(0, Math.min(1, parseNumber(raw?.minFailChance, 0))),
        luckyChanceBase: Math.max(0, Math.min(1, parseNumber(raw?.luckyChanceBase, 0))),
        durationSec: Math.max(0, parseNumber(raw?.durationSec, 0)),
        rarityDurationAddSecByRarityIndex: Array.isArray(raw?.rarityDurationAddSecByRarityIndex)
          ? raw.rarityDurationAddSecByRarityIndex.map((value) => Math.max(0, parseNumber(value, 0)))
          : [],
        alchemyExp: Math.max(0, parseNumber(raw?.alchemyExp, 0)),
        luckyTiers: Array.isArray(raw?.luckyTiers)
          ? raw.luckyTiers.map((tier) => ({
              ...tier,
              id: parseOptionalString(tier?.id) || "",
              weight: Math.max(0, parseNumber(tier?.weight, 0)),
              kind: parseOptionalString(tier?.kind) || "",
              value: parseNumber(tier?.value, 0),
              itemId: parseOptionalString(tier?.itemId) || "",
              min: Math.max(0, Math.round(parseNumber(tier?.min, 0))),
              max: Math.max(0, Math.round(parseNumber(tier?.max, 0)))
            })).filter((tier) => tier.id && tier.weight > 0)
          : [],
        minAlchemyLevel: Math.max(1, Math.round(parseNumber(raw?.minAlchemyLevel, 1))),
        disabled: parseBoolean(raw?.disabled, false)
      }
    })
    .filter(Boolean)
}

export function validateAlchemyTables(groups, traits, combos, transmuteRules, items, attributes, actions, locales) {
  const itemByID = new Map((items || []).map((item) => [item.id, item]))
  const attrCategoryByID = new Map((attributes || []).map((attr) => [attr.id, attr.category]))
  const traitItemIDs = new Set((traits || []).map((row) => row.itemId))
  const rarityOrder = new Map(["common", "uncommon", "rare", "epic", "legendary", "mythic"].map((id, index) => [id, index]))
  const lowTierLifeConvertMainTypes = new Set(["collecting", "mining", "farming_animal"])
  const lowTierLifeConvertRarities = new Set(["common", "uncommon"])
  const lowTierLifeConvertItemIDs = new Set()
  for (const action of actions || []) {
    const mainType = String(action?.mainType || action?.main_type || action?.menu || "")
    if (!lowTierLifeConvertMainTypes.has(mainType)) continue
    const rewards = action?.itemReward && typeof action.itemReward === "object" && !Array.isArray(action.itemReward)
      ? action.itemReward
      : {}
    for (const itemID of Object.keys(rewards)) {
      const item = itemByID.get(itemID)
      if (item && lowTierLifeConvertRarities.has(String(item.rarity || "common"))) {
        lowTierLifeConvertItemIDs.add(itemID)
      }
    }
  }
  const alchemyItemAllowed = (item, groupID = "") => {
    if (!item || item.disabled || item.isEquipment) return false
    const id = String(item.id || "").toLowerCase()
    const category = String(item.category || "").toLowerCase()
    const types = Array.isArray(item.type) ? item.type.map((entry) => String(entry || "").toLowerCase()) : []
    if (item.alchemy?.convertExcluded) return false
    if (String(groupID || "").startsWith("group.skillbook_")) {
      if (category && category !== "skillbook") return false
      if (!types.includes("skillbook")) return false
      if (types.some((entry) => ["trigger_item", "collectible", "currency", "map", "key", "quest"].includes(entry))) return false
      return !id.startsWith("map_") && !id.startsWith("key_") && !id.startsWith("boss_") && !id.includes("rumor")
    }
    if (["equipment", "alchemy_medium", "currency", "collectible", "quest", "map", "key"].includes(category)) return false
    if (types.some((entry) => ["trigger_item", "collectible", "skillbook", "currency", "map", "key", "quest"].includes(entry))) return false
    if (id.startsWith("map_") || id.startsWith("skillbook_") || id.startsWith("key_") || id.startsWith("world_boss") || id.startsWith("boss_") || id.includes("rumor")) return false
    return true
  }
  const alchemyGroupAllowsTraitlessItem = (group, itemID) => {
    return String(group?.id || "").startsWith("group.life_primary_") && lowTierLifeConvertItemIDs.has(itemID)
  }
  const groupItemSeen = new Set()
  for (const group of groups || []) {
    if (!group.id) throw new Error("alchemy_groups.id required")
    if ("ultraRareChanceBase" in group || "ultraRareEntries" in group) throw new Error(`alchemy_groups conversion does not support ultra-rare reward branches: ${group.id}`)
    if (!Array.isArray(group.entries) || group.entries.length === 0) throw new Error(`alchemy_groups.entries empty: ${group.id}`)
    if (group.goldCost < 0 || group.durationSec < 0 || group.alchemyExp < 0) throw new Error(`alchemy_groups economics invalid: ${group.id}`)
    if (group.failChanceBase < 0 || group.failChanceBase > 1) throw new Error(`alchemy_groups.failChanceBase invalid: ${group.id}`)
    if (group.minFailChance < 0 || group.minFailChance > 1) throw new Error(`alchemy_groups.minFailChance invalid: ${group.id}`)
    if (group.upgradeChanceBase < 0 || group.upgradeChanceBase > 0.25) throw new Error(`alchemy_groups.upgradeChanceBase invalid: ${group.id}`)
    const entryRarityIndexes = new Set()
    for (const entry of group.entries) {
      const item = itemByID.get(entry.itemId)
      if (!item) throw new Error(`alchemy_groups.entries.itemId not found: ${group.id}.${entry.itemId}`)
      if (!alchemyItemAllowed(item, group.id)) throw new Error(`alchemy_groups.entries item is not safe for alchemy: ${group.id}.${entry.itemId}`)
      // Conversion can include low-tier life action outputs that are intentionally
      // not infusion materials; other non-skill items still require traits.
      if (!String(group.id || "").startsWith("group.skillbook_") && !traitItemIDs.has(entry.itemId) && !alchemyGroupAllowsTraitlessItem(group, entry.itemId)) throw new Error(`alchemy_groups.entries.itemId must have an item_traits row or be a low-tier life output: ${group.id}.${entry.itemId}`)
      if (groupItemSeen.has(entry.itemId)) throw new Error(`alchemy_groups item appears in multiple groups: ${entry.itemId}`)
      groupItemSeen.add(entry.itemId)
      if (typeof entry.weight !== "number" || !Number.isFinite(entry.weight) || entry.weight <= 0) throw new Error(`alchemy_groups.entries.weight invalid: ${group.id}.${entry.itemId}`)
      entryRarityIndexes.add(rarityOrder.get(item.rarity) ?? 0)
    }
    const baseRarity = Math.min(...entryRarityIndexes)
    for (const entry of group.upgradeEntries || []) {
      const item = itemByID.get(entry.itemId)
      if (!item) throw new Error(`alchemy_groups.upgradeEntries.itemId not found: ${group.id}.${entry.itemId}`)
      if (!alchemyItemAllowed(item, group.id)) throw new Error(`alchemy_groups.upgradeEntries item is not safe for alchemy: ${group.id}.${entry.itemId}`)
      if (String(item.rarity || "").toLowerCase() === "mythic") throw new Error(`alchemy_groups.upgradeEntries cannot produce mythic items: ${group.id}.${entry.itemId}`)
      if (!String(group.id || "").startsWith("group.skillbook_") && !traitItemIDs.has(entry.itemId) && !alchemyGroupAllowsTraitlessItem(group, entry.itemId)) throw new Error(`alchemy_groups.upgradeEntries.itemId must have an item_traits row or be a low-tier life output: ${group.id}.${entry.itemId}`)
      const upgradeRarity = rarityOrder.get(item.rarity) ?? 0
      if (!String(group.id || "").startsWith("group.skillbook_") && upgradeRarity !== baseRarity + 1) {
        throw new Error(`alchemy_groups.upgradeEntries must be exactly one rarity higher: ${group.id}.${entry.itemId}`)
      }
      if (typeof entry.weight !== "number" || !Number.isFinite(entry.weight) || entry.weight <= 0) throw new Error(`alchemy_groups.upgradeEntries.weight invalid: ${group.id}.${entry.itemId}`)
    }
  }
  const traitIDs = new Set()
  const luckTraitWhitelist = new Set(["holy_cat_royal_jewel"])
  for (const row of traits || []) {
    const item = itemByID.get(row.itemId)
    if (!item) throw new Error(`item_traits.itemId not found: ${row.itemId}`)
    // Single source of truth: nameAffix lives on items.json `alchemy.nameAffix`.
    // Validate the cross-file invariant: trait-bearing items MUST declare nameAffix.
    const nameAffix = item.alchemy?.nameAffix
    if (!nameAffix?.prefixKey || !nameAffix?.suffixKey) {
      throw new Error(`items[${row.itemId}].alchemy.nameAffix missing — required because item_traits references this item`)
    }
    for (const key of [nameAffix.prefixKey, nameAffix.suffixKey]) {
      for (const locale of locales || []) {
        if (!localeHasPath(locale, key)) throw new Error(`items[${row.itemId}].alchemy.nameAffix i18n key missing: ${key}`)
      }
    }
    const kinds = new Set()
    let firstTraitParams = null
    for (const trait of row.traits || []) {
      if (traitIDs.has(trait.traitId)) throw new Error(`duplicate item_traits.traitId: ${trait.traitId}`)
      traitIDs.add(trait.traitId)
      if (trait.kind !== "life" && trait.kind !== "combat") throw new Error(`item_traits.kind invalid: ${trait.traitId}`)
      kinds.add(trait.kind)
      if (trait.effectType !== "add_stat") throw new Error(`item_traits.effectType unsupported: ${trait.traitId}`)
      if (!Number.isInteger(trait.unitBasis) || trait.unitBasis <= 0) throw new Error(`item_traits.unitBasis invalid: ${trait.traitId}`)
      if (!Number.isInteger(trait.maxBasisMultiplier) || trait.maxBasisMultiplier <= 0) throw new Error(`item_traits.maxBasisMultiplier invalid: ${trait.traitId}`)
      if (trait.goldCostPerUnit < 0 || trait.durationSecPerUnit < 0 || trait.alchemyExpPerUnit < 0) {
        throw new Error(`item_traits economics invalid: ${trait.traitId}`)
      }
      if (trait.failChancePerExtraBasis < 0 || trait.failChanceQuantityCap < 0 || trait.failChanceQuantityCap > 1) {
        throw new Error(`item_traits quantity fail params invalid: ${trait.traitId}`)
      }
      if (!Number.isInteger(trait.maxEffectivePerEquipment) || trait.maxEffectivePerEquipment <= 0) {
        throw new Error(`item_traits.maxEffectivePerEquipment invalid: ${trait.traitId}`)
      }
      if (!Number.isInteger(trait.maxEffectivePerBody) || trait.maxEffectivePerBody <= 0) {
        throw new Error(`item_traits.maxEffectivePerBody invalid: ${trait.traitId}`)
      }
      if (trait.maxEffectivePerEquipment > trait.maxEffectivePerBody) {
        throw new Error(`item_traits maxEffectivePerEquipment cannot exceed maxEffectivePerBody: ${trait.traitId}`)
      }
      if (!Array.isArray(trait.effects) || trait.effects.length === 0) throw new Error(`item_traits.effects empty: ${trait.traitId}`)
      const seenAttrs = new Set()
      let positiveEffectCount = 0
      let negativeEffectCount = 0
      for (const eff of trait.effects) {
        if (!eff.attr) throw new Error(`item_traits.effects.attr missing: ${trait.traitId}`)
        if (seenAttrs.has(eff.attr)) throw new Error(`item_traits.effects duplicate attr ${eff.attr} in trait: ${trait.traitId}`)
        seenAttrs.add(eff.attr)
        const attrCategory = attrCategoryByID.get(eff.attr)
        if (!attrCategory) throw new Error(`item_traits.effects.attr not found in attributes: ${trait.traitId}.${eff.attr}`)
        if (trait.kind === "life" && attrCategory !== "work") {
          throw new Error(`item_traits life trait must use work attr: ${trait.traitId}.${eff.attr} (${attrCategory})`)
        }
        if (trait.kind === "combat" && attrCategory !== "battle") {
          throw new Error(`item_traits combat trait must use battle attr: ${trait.traitId}.${eff.attr} (${attrCategory})`)
        }
        if (eff.valuePerUnit.min > eff.valuePerUnit.max) throw new Error(`item_traits.effects.valuePerUnit invalid (min>max): ${trait.traitId}.${eff.attr}`)
        if (eff.valuePerUnit.max > 0) positiveEffectCount++
        if (eff.valuePerUnit.max <= 0) negativeEffectCount++
      }
      if (positiveEffectCount > 2) throw new Error(`item_traits has too many positive effects (max 2): ${trait.traitId}`)
      if (seenAttrs.has("luck")) {
        if (!luckTraitWhitelist.has(row.itemId)) throw new Error(`item_traits luck is whitelist-only: ${trait.traitId}`)
        if (positiveEffectCount !== 1 || negativeEffectCount < 2) {
          throw new Error(`item_traits luck material must be 1 positive luck + multiple negatives: ${trait.traitId}`)
        }
      }
      // All traits of one material must share basis/maxMul/level/failChance shape so the same qty applies uniformly.
      const shape = { unitBasis: trait.unitBasis, maxBasisMultiplier: trait.maxBasisMultiplier, minAlchemyLevel: trait.minAlchemyLevel }
      if (!firstTraitParams) {
        firstTraitParams = shape
      } else if (firstTraitParams.unitBasis !== shape.unitBasis || firstTraitParams.maxBasisMultiplier !== shape.maxBasisMultiplier) {
        throw new Error(`item_traits[${row.itemId}] traits share same qty domain but unitBasis/maxBasisMultiplier diverge — got ${JSON.stringify(firstTraitParams)} vs ${JSON.stringify(shape)}`)
      }
    }
    if (kinds.size > 1) throw new Error(`item_traits mixed kind for item: ${row.itemId}`)
  }
  const comboIDs = new Set()
  for (const combo of combos || []) {
    if (comboIDs.has(combo.id)) throw new Error(`duplicate alchemy_combo_recipes.id: ${combo.id}`)
    comboIDs.add(combo.id)
    if (combo.kind !== "life" && combo.kind !== "combat") throw new Error(`alchemy_combo_recipes.kind invalid: ${combo.id}`)
    if (!Number.isInteger(combo.inputs?.glow_core) || combo.inputs.glow_core <= 0) throw new Error(`alchemy_combo_recipes.inputs missing glow_core: ${combo.id}`)
    for (const [itemID, qty] of Object.entries(combo.inputs || {})) {
      if (!itemByID.has(itemID)) throw new Error(`alchemy_combo_recipes.inputs item not found: ${combo.id}.${itemID}`)
      if (!Number.isInteger(qty) || qty <= 0) throw new Error(`alchemy_combo_recipes.inputs qty invalid: ${combo.id}.${itemID}`)
    }
    if (combo.extraGoldCost < 0 || combo.extraDurationSec < 0) throw new Error(`alchemy_combo_recipes economics invalid: ${combo.id}`)
    if (combo.outputAffix.valueRange.min > combo.outputAffix.valueRange.max) throw new Error(`alchemy_combo_recipes.outputAffix.valueRange invalid: ${combo.id}`)
  }
  const transmuteRuleIDs = new Set()
  const transmuteItemMatches = new Map()
  let activeDefaultRuleCount = 0
  for (const rule of transmuteRules || []) {
    if (transmuteRuleIDs.has(rule.id)) throw new Error(`duplicate alchemy_transmute_rules.id: ${rule.id}`)
    transmuteRuleIDs.add(rule.id)
    if (rule.returnRatio.minRatio > rule.returnRatio.maxRatio || rule.returnRatio.minRatio < 0) throw new Error(`alchemy_transmute_rules.returnRatio invalid: ${rule.id}`)
    if (rule.failChanceBase < 0 || rule.failChanceBase > 1) throw new Error(`alchemy_transmute_rules.failChanceBase invalid: ${rule.id}`)
    if (rule.minFailChance < 0 || rule.minFailChance > 1) throw new Error(`alchemy_transmute_rules.minFailChance invalid: ${rule.id}`)
    if (rule.durationSec < 0 || rule.alchemyExp < 0) throw new Error(`alchemy_transmute_rules economics invalid: ${rule.id}`)
    if ((rule.rarityDurationAddSecByRarityIndex || []).some((value) => value < 0)) throw new Error(`alchemy_transmute_rules.rarityDurationAddSecByRarityIndex invalid: ${rule.id}`)
    if (rule.default && (rule.itemIds || []).length > 0) throw new Error(`alchemy_transmute_rules default rule cannot declare itemIds: ${rule.id}`)
    if (!rule.disabled && rule.default) activeDefaultRuleCount++
    const localItemIDs = new Set()
    for (const itemID of rule.itemIds || []) {
      if (localItemIDs.has(itemID)) throw new Error(`alchemy_transmute_rules duplicate itemIds entry: ${rule.id}.${itemID}`)
      localItemIDs.add(itemID)
      const item = itemByID.get(itemID)
      if (!item) throw new Error(`alchemy_transmute_rules.itemIds item not found: ${rule.id}.${itemID}`)
      if (!alchemyItemAllowed(item)) throw new Error(`alchemy_transmute_rules.itemIds item is not transmutable: ${rule.id}.${itemID}`)
      if (!rule.disabled) {
        const previous = transmuteItemMatches.get(itemID)
        if (previous) throw new Error(`alchemy_transmute_rules active item match is ambiguous: ${itemID} (${previous}, ${rule.id})`)
        transmuteItemMatches.set(itemID, rule.id)
      }
    }
    for (const tier of rule.luckyTiers || []) {
      if (tier.kind !== "gold_multiplier") throw new Error(`alchemy_transmute_rules.luckyTiers.kind must be gold_multiplier: ${rule.id}.${tier.id}`)
      if (tier.value <= 1) throw new Error(`alchemy_transmute_rules.luckyTiers.value invalid: ${rule.id}.${tier.id}`)
    }
  }
  if (activeDefaultRuleCount !== 1) throw new Error(`alchemy_transmute_rules must have exactly one active default rule; got ${activeDefaultRuleCount}`)
}

function localeHasPath(locale, path) {
  if (!locale || typeof locale !== "object") return false
  const parts = String(path || "").split(".").filter(Boolean)
  let node = locale
  for (const part of parts) {
    if (!node || typeof node !== "object" || !(part in node)) return false
    node = node[part]
  }
  return typeof node === "string"
}

function validateItemCollectibleReferences(items, collectibles) {
  const collectibleByID = new Map()
  for (const row of collectibles || []) {
    const id = String(row?.id || "").trim()
    if (!id || row?.disabled) continue
    collectibleByID.set(id, row)
  }
  for (const item of items || []) {
    const collectibleID = String(item?.useGrantCollectibleId || "").trim()
    if (!collectibleID) continue
    const collectible = collectibleByID.get(collectibleID)
    if (!collectible) throw new Error(`item.useGrantCollectibleId not found: ${item.id}.${collectibleID}`)
    if (!Array.isArray(item.type) || !item.type.includes("collectible")) {
      throw new Error(`item.useGrantCollectibleId requires collectible type: ${item.id}`)
    }
    const collectibleItemID = String(collectible?.itemId || "").trim()
    // Upgradeable collectibles may be upgraded by a different item than their itemId (e.g. lucky_clover upgrades lucky_clover_statue).
    // Only enforce itemId symmetry for non-upgradeable collectibles.
    if (!collectible.upgradeable && collectibleItemID !== item.id) {
      throw new Error(`item.useGrantCollectibleId item mismatch: ${item.id}.${collectibleID}.${collectibleItemID}`)
    }
  }
}

function validateBattleUsableItems(rows, items = [], enemies = [], buffs = [], worlds = []) {
  if (!Array.isArray(rows)) throw new Error("battle_usable_items must be an array")
  const ids = new Set()
  const itemIDs = new Set()
  const knownItemIDs = new Set((items || []).map((row) => String(row?.id || "").trim()).filter(Boolean))
  const knownEnemyIDs = new Set((enemies || []).map((row) => String(row?.id || "").trim()).filter(Boolean))
  const knownBuffIDs = new Set((buffs || []).map((row) => String(row?.id || "").trim()).filter(Boolean))
  const knownWorldIDs = new Set((worlds || []).map((row) => String(row?.id || "").trim()).filter(Boolean))
  const validateApplyChance = (payload, label, required = false) => {
    const configured = payload?.applyChanceBase !== undefined || payload?.applyChance !== undefined
    if (required && !configured) throw new Error(`${label}.applyChanceBase required`)
    if (!configured) return
    const base = parseNumberStrict(payload.applyChanceBase ?? payload.applyChance)
    const min = payload.applyChanceMin === undefined ? 0 : parseNumberStrict(payload.applyChanceMin)
    const max = payload.applyChanceMax === undefined ? 1 : parseNumberStrict(payload.applyChanceMax)
    if (!Number.isFinite(base) || base < 0 || base > 1 || !Number.isFinite(min) || !Number.isFinite(max) || min < 0 || max > 1 || min > max) {
      throw new Error(`${label}.applyChance invalid`)
    }
  }
  for (const row of rows) {
    if (!row || typeof row !== "object" || Array.isArray(row)) throw new Error("battle_usable_items row must be object")
    if (typeof row.id !== "string" || row.id.trim() === "") {
      throw new Error("battle_usable_items.id is required string")
    }
    if (ids.has(row.id)) throw new Error(`duplicate battle_usable_items.id: ${row.id}`)
    ids.add(row.id)
    if (typeof row.itemId !== "string" || row.itemId.trim() === "") {
      throw new Error(`battle_usable_items.itemId is required string: ${row.id}`)
    }
    if (itemIDs.has(row.itemId)) throw new Error(`duplicate battle_usable_items.itemId: ${row.itemId}`)
    itemIDs.add(row.itemId)
    if (!knownItemIDs.has(row.itemId.trim())) throw new Error(`battle_usable_items.itemId not found: ${row.id}.${row.itemId}`)
    rejectEnemyOnlyTargetPriority(row.targeting, `battle_usable_items.targeting: ${row.id}`)
    if (row.cooldownTurns !== undefined) {
      const cooldownTurns = Number(row.cooldownTurns)
      if (!Number.isInteger(cooldownTurns) || cooldownTurns < 0) {
        throw new Error(`battle_usable_items.cooldownTurns invalid: ${row.id}`)
      }
    }
    if (row.isTrigger === true) {
      const dungeonIDs = parseIDList(row.dungeonIds)
      const spawnEnemyIDs = parseIDList(row.spawnEnemyIds)
      if (dungeonIDs.length === 0 || dungeonIDs.some((id) => !knownWorldIDs.has(id))) throw new Error(`battle_usable_items.dungeonIds invalid: ${row.id}`)
      if (spawnEnemyIDs.length === 0 || spawnEnemyIDs.some((id) => !knownEnemyIDs.has(id))) throw new Error(`battle_usable_items.spawnEnemyIds invalid: ${row.id}`)
      if (!Number.isInteger(row.minWave) || row.minWave < 1) throw new Error(`battle_usable_items.minWave invalid: ${row.id}`)
      if (!Number.isInteger(row.intervalWaves) || row.intervalWaves < 0) throw new Error(`battle_usable_items.intervalWaves invalid: ${row.id}`)
      if (!Number.isFinite(row.perWaveChance) || row.perWaveChance < 0 || row.perWaveChance > 1) throw new Error(`battle_usable_items.perWaveChance invalid: ${row.id}`)
      if (row.partyTriggerGroup !== undefined && (typeof row.partyTriggerGroup !== "string" || !/^[a-z0-9_]+$/.test(row.partyTriggerGroup))) {
        throw new Error(`battle_usable_items.partyTriggerGroup invalid: ${row.id}`)
      }
      continue
    }
    if (row.isTrigger !== undefined && typeof row.isTrigger !== "boolean") throw new Error(`battle_usable_items.isTrigger invalid: ${row.id}`)
    if (!Array.isArray(row.effects) || row.effects.length === 0) throw new Error(`battle_usable_items.effects required: ${row.id}`)
    for (const effect of row.effects) {
      if (!effect || typeof effect !== "object" || Array.isArray(effect)) throw new Error(`battle_usable_items.effects entry invalid: ${row.id}`)
      const kind = String(effect.kind || "").trim().toLowerCase()
      if (!["damage", "heal", "resource", "status", "summon", "adjust_skill_cooldown", "weighted_random_per_target", "remove_buff", "remove_status"].includes(kind)) {
        throw new Error(`battle_usable_items.effects.kind invalid: ${row.id}.${kind}`)
      }
      if (!effect.targeting || typeof effect.targeting !== "object" || Array.isArray(effect.targeting)) throw new Error(`battle_usable_items.effects.targeting invalid: ${row.id}.${kind}`)
      rejectEnemyOnlyTargetPriority(effect.targeting, `battle_usable_items.effects.targeting: ${row.id}.${kind}`)
      if (kind === "status") {
        const payload = effect.statusPayload
        if (!payload || !Array.isArray(payload.statusIds) || payload.statusIds.length === 0) throw new Error(`battle_usable_items.effects.statusPayload invalid: ${row.id}`)
        for (const buffID of payload.statusIds) if (!knownBuffIDs.has(buffID)) throw new Error(`battle_usable_items.effects.status buff not found: ${row.id}.${buffID}`)
        validateApplyChance(payload, `battle_usable_items.effects.statusPayload: ${row.id}`)
      }
      if (kind === "weighted_random_per_target") {
        validateApplyChance(effect.statusPayload || effect, `battle_usable_items.effects.weighted_random_per_target: ${row.id}`, true)
        if (!Array.isArray(effect.choices) || effect.choices.length < 2) throw new Error(`battle_usable_items.effects.weighted choices invalid: ${row.id}`)
        const choiceIDs = new Set()
        for (const choice of effect.choices) {
          const choiceID = parseOptionalString(choice?.id) || ""
          const buffID = parseOptionalString(choice?.statusId ?? choice?.buffId) || ""
          if (!choiceID || choiceIDs.has(choiceID) || !knownBuffIDs.has(buffID) || !Number.isFinite(choice?.weight) || choice.weight <= 0 || !Number.isInteger(choice?.durationTurns) || choice.durationTurns <= 0) {
            throw new Error(`battle_usable_items.effects.weighted choice invalid: ${row.id}.${choiceID}`)
          }
          choiceIDs.add(choiceID)
        }
      }
      if (kind === "summon") {
        validateSummonUnitSelector(effect.summonPayload || effect, `battle_usable_items.effects.summonPayload: ${row.id}`, knownEnemyIDs)
      }
      if (kind === "remove_buff" || kind === "remove_status") {
        const buffIDs = parseIDList(effect.buffIds ?? effect.statusIds ?? effect.buffId ?? effect.statusId)
        if (buffIDs.length === 0 || buffIDs.some((id) => !knownBuffIDs.has(id))) throw new Error(`battle_usable_items.effects.remove_buff invalid: ${row.id}`)
      }
    }
  }
}

function validateBattleUsableItemReferences(items, battleUsableItems) {
  const itemIDSet = new Set((items || []).map((row) => String(row?.id || "").trim()).filter(Boolean))
  const battleByID = new Map()
  for (const row of battleUsableItems || []) {
    if (!row || typeof row !== "object") continue
    const battleID = String(row.id || "").trim()
    const itemID = String(row.itemId || "").trim()
    if (!battleID) continue
    battleByID.set(battleID, itemID)
    if (itemID && !itemIDSet.has(itemID)) {
      throw new Error(`battle_usable_items.itemId not found in items: ${battleID}.${itemID}`)
    }
  }
  for (const item of items || []) {
    const battleUsableItemID = String(item?.battleUsableItemId || "").trim()
    if (!battleUsableItemID) continue
    const linkedItemID = battleByID.get(battleUsableItemID)
    if (!linkedItemID) {
      throw new Error(`item.battleUsableItemId not found in battle_usable_items: ${item.id}.${battleUsableItemID}`)
    }
    if (linkedItemID !== item.id) {
      throw new Error(`item.battleUsableItemId mismatch itemId: ${item.id}.${battleUsableItemID}.${linkedItemID}`)
    }
  }
}

/**
 * Validates that every commission's gold_coin reward is proportional to the
 * sum of (qty_min × economyBaseValue) across all required items.
 *
 * Rules:
 *   Life commissions  — reward.min must be 0.80×–2.50× expected cost value.
 *   Battle commissions — reward.min must be 0.85×–1.15× expected cost value,
 *                        and reward.max must be 1.25×–1.75× reward.min (≈ 1.5×).
 *   Rumor commissions  — skipped entirely (non-standard rewards).
 *   costOptions        — each option's min total is computed; the lowest value
 *                        is used as the expected baseline.
 *   rewardOptions      — skipped (paired rewards are too complex to validate
 *                        without pairing logic here).
 */
function validateCommissionRewards(commissions, items) {
  const itemByID = new Map(items.map((i) => [i.id, i]))
  const errors = []

  for (const c of commissions) {
    if (c.disabled || c.isRumor) continue
    // Skip commissions that use paired rewardOptions (non-uniform reward pools)
    if (Array.isArray(c.rewardOptions) && c.rewardOptions.length > 0) continue

    const reward = c.rewardPool?.gold_coin
    if (!reward || typeof reward.min !== "number" || typeof reward.max !== "number") continue

    const rewardMin = reward.min
    const rewardMax = reward.max

    if (rewardMin <= 0) {
      errors.push(`${c.id}: reward.min must be positive, got ${rewardMin}`)
      continue
    }
    if (rewardMax < rewardMin) {
      errors.push(`${c.id}: reward.max (${rewardMax}) must be >= reward.min (${rewardMin})`)
      continue
    }

    // Collect cost groups: either one fixed group or multiple option groups
    const groups =
      c.costs && Object.keys(c.costs).length > 0
        ? [c.costs]
        : Array.isArray(c.costOptions) && c.costOptions.length > 0
          ? c.costOptions
          : []

    if (groups.length === 0) continue

    // Compute each group's total: qty_min × economyBaseValue
    const optionValues = []
    let hasUnknownItem = false
    for (const group of groups) {
      let total = 0
      for (const [itemId, range] of Object.entries(group)) {
        const item = itemByID.get(itemId)
        if (!item) {
          errors.push(`${c.id}: cost references unknown item "${itemId}"`)
          hasUnknownItem = true
          break
        }
        const base = item.economyBaseValue ?? 0
        if (base <= 0) continue // items with no value (e.g. currency) are excluded
        total += (range.min ?? 0) * base
      }
      if (total > 0) optionValues.push(total)
    }
    if (hasUnknownItem || optionValues.length === 0) continue

    // Use the cheapest option as the expected baseline
    const expectedMin = Math.min(...optionValues)
    const ratio = rewardMin / expectedMin

    if (c.isBattle) {
      // Battle: tight ratio (≈ 1.0), max ≈ min × 1.5
      if (ratio < 0.85 || ratio > 1.15) {
        errors.push(
          `${c.id}: [battle] reward.min=${rewardMin} but expected≈${Math.round(expectedMin)} ` +
            `(ratio=${ratio.toFixed(3)}, must be 0.85–1.15)`
        )
      }
      const maxMinRatio = rewardMax / rewardMin
      if (maxMinRatio < 1.25 || maxMinRatio > 1.75) {
        errors.push(
          `${c.id}: [battle] reward.max/reward.min=${maxMinRatio.toFixed(3)}, must be 1.25–1.75 (≈ 1.5×)`
        )
      }
    } else {
      // Life: looser ratio (organically designed rewards)
      if (ratio < 0.80 || ratio > 2.50) {
        errors.push(
          `${c.id}: [life] reward.min=${rewardMin} but expected≈${Math.round(expectedMin)} ` +
            `(ratio=${ratio.toFixed(3)}, must be 0.80–2.50)`
        )
      }
    }
  }

  if (errors.length > 0) {
    throw new Error(
      `validateCommissionRewards: ${errors.length} error(s) found:\n` +
        errors.map((e) => `  ✗ ${e}`).join("\n")
    )
  }
}

function validateRumorCommissionDeletionRules(commissions) {
  const errors = []
  for (const c of commissions) {
    if (c.disabled || !c.isRumor) continue
    const prereqRumorIds = Array.isArray(c.prereqRumorIds)
      ? c.prereqRumorIds.map((id) => String(id || "").trim()).filter(Boolean)
      : []
    const isFirstStage = prereqRumorIds.length === 0
    if (isFirstStage && c.notDeletable === true) {
      errors.push(`${c.id}: first-stage rumor commissions must be deletable; omit notDeletable or set it to false`)
    }
    if (!isFirstStage && c.notDeletable !== true) {
      errors.push(`${c.id}: follow-up rumor commissions must set notDeletable: true`)
    }
  }
  if (errors.length > 0) {
    throw new Error(
      `validateRumorCommissionDeletionRules: ${errors.length} error(s) found:\n` +
        errors.map((e) => `  ✗ ${e}`).join("\n")
    )
  }
}

const SPECIALIZATION_SCHOOL_IDS = ["strength", "dexterity", "intelligence", "perception", "stamina"]
const SPECIALIZATION_NODE_COSTS = {
  root: 1,
  technique: 2,
  confluence: 6,
  terminal: 4
}
const SPECIALIZATION_BATTLE_LEVEL_CAP = 500
const SPECIALIZATION_ATTRIBUTE_ENVELOPE_MAX = 500
const SPECIALIZATION_PAYLOAD_ID_PATTERN = /^[a-z][a-z0-9_]*$/
const SPECIALIZATION_TOPOLOGY_COSTS = {
  strength_terminal_three_armed: [22],
  confluence_intelligence_perception_crownless_astrolabe: [10],
  dexterity_terminal_shadow_dance: [4, 2, 2, 20],
  intelligence_terminal_archmage: [4, 2, 2, 20],
  stamina_terminal_absolute_defense: [4, 2, 2, 20]
}
const SPECIALIZATION_KEYWORD_MECHANIC_KINDS = new Set(["counter", "repeat_cast", "repeat_effect", "damage_intercept", "extra_turn", "debuff_family", "debuff", "buff"])
const SPECIALIZATION_SLOT_FAMILIES = {
  cloak: { base: 1, min: 0, max: 1 },
  head: { base: 1, min: 0, max: 2 },
  left_hand: { base: 1, min: 0, max: 1 },
  body: { base: 1, min: 0, max: 2 },
  right_hand: { base: 1, min: 0, max: 2 },
  arm: { base: 1, min: 0, max: 2 },
  legs: { base: 1, min: 0, max: 1 },
  feet: { base: 1, min: 0, max: 2 },
  accessory: { base: 4, min: 0, max: 8 }
}

function specializationIsObject(value) {
  return Boolean(value) && typeof value === "object" && !Array.isArray(value)
}

function specializationRuntimeTokens(value, tokens = new Set(), parentKind = "", key = "") {
  if (Array.isArray(value)) {
    if (parentKind === "target_has_any_buff" && key === "buffIds") return tokens
    for (const entry of value) specializationRuntimeTokens(entry, tokens, parentKind, key)
    return tokens
  }
  if (specializationIsObject(value)) {
    const kind = typeof value.kind === "string" ? value.kind : parentKind
    for (const [childKey, childValue] of Object.entries(value)) specializationRuntimeTokens(childValue, tokens, kind, childKey)
    return tokens
  }
  if (typeof value === "string") tokens.add(value)
  return tokens
}

function specializationRequiredString(value, label) {
  if (typeof value !== "string" || value.trim() === "") throw new Error(`${label} must be a non-empty string`)
  return value.trim()
}

const SPECIALIZATION_BATTLE_TRIGGERS = new Set([
  "onAfterDealCrit",
  "onAfterDealDamage",
  "onAfterSkillCast",
  "onAfterTakeDamage",
  "onAllyBeforeTakeDamage",
  "onBattleStart",
  "onBeforeDealDamage",
  "onBeforeSkillCast",
  "onBeforeTakeDamage",
  "onKill",
  "onTurnEnd",
  "onTurnStart",
  "onWaveStart"
])
const SPECIALIZATION_DAMAGE_TYPES = new Set(["physical", "magical", "true"])
const SPECIALIZATION_TARGET_MODES = new Set(["self", "source", "target", "enemy", "marked_enemy", "targeting", "lowest_hp_ally"])
const SPECIALIZATION_MUTATION_EFFECT_KINDS = new Set(["adjust_skill_cooldown", "corpse_explosion", "damage", "heal", "resource", "status"])
const SPECIALIZATION_DAMAGE_CONTEXT_TRIGGERS = new Set([
  "onBeforeDealDamage",
  "onAfterDealDamage",
  "onAfterDealCrit",
  "onBeforeTakeDamage",
  "onAfterTakeDamage",
  "onAllyBeforeTakeDamage",
  "onKill"
])
const SPECIALIZATION_SKILL_CONTEXT_TRIGGERS = new Set([
  "onBeforeSkillCast",
  "onAfterSkillCast",
  ...SPECIALIZATION_DAMAGE_CONTEXT_TRIGGERS
])

function specializationTriggerHasDamageContext(trigger) {
  return SPECIALIZATION_DAMAGE_CONTEXT_TRIGGERS.has(trigger)
}

function specializationTriggerHasSkillContext(trigger) {
  return SPECIALIZATION_SKILL_CONTEXT_TRIGGERS.has(trigger)
}

function specializationTriggerHasEventSubject(trigger) {
  return specializationTriggerHasDamageContext(trigger)
}

function specializationRejectUnknownKeys(value, allowedKeys, label) {
  const allowed = allowedKeys instanceof Set ? allowedKeys : new Set(allowedKeys)
  for (const key of Object.keys(value)) {
    if (!allowed.has(key)) throw new Error(`${label}.${key} is an unknown key`)
  }
}

function specializationValidateFinite(value, label, { integer = false, min = -Infinity, max = Infinity, nonZero = false } = {}) {
  if (!Number.isFinite(value) || (integer && !Number.isInteger(value)) || value < min || value > max || (nonZero && value === 0)) {
    throw new Error(`${label} invalid`)
  }
}

function specializationValidateStringArray(value, label, { allowedValues, knownValues } = {}) {
  if (!Array.isArray(value) || value.length === 0) throw new Error(`${label} must be a non-empty array`)
  const seen = new Set()
  for (const raw of value) {
    const entry = specializationRequiredString(raw, `${label} entry`)
    if (seen.has(entry)) throw new Error(`${label} duplicate: ${entry}`)
    seen.add(entry)
    if (allowedValues && !allowedValues.has(entry)) throw new Error(`${label} value invalid: ${entry}`)
    if (knownValues && !knownValues.has(entry)) throw new Error(`${label} reference not found: ${entry}`)
  }
  return [...seen]
}

function validateSpecializationPlayerCopy(line, label, keywordIDs, declaredKeywordIDs = keywordIDs) {
  const tokens = [...line.matchAll(/\[\[keyword:([a-z0-9_]+)\]\]/g)]
  for (const token of tokens) {
    if (!keywordIDs.has(token[1])) throw new Error(`${label} references unknown keyword token: ${token[1]}`)
    if (!declaredKeywordIDs.has(token[1])) throw new Error(`${label} references a keyword not declared by the node: ${token[1]}`)
  }
  const plainText = line.replace(/\[\[keyword:[a-z0-9_]+\]\]/g, "")
  if (/[{}\[\]]|\b(?:attributeKey|valueFormula|battleHooks|effectMutations|targetMode|selectionMode|dispatchHooks|baseAdd|coefficient)\b/i.test(plainText)) {
    throw new Error(`${label} must be player-facing copy, not DSL or JSON`)
  }
}

function validateSpecializationFormula(formula, label, refs) {
  if (!specializationIsObject(formula)) throw new Error(`${label} must be an object`)
  specializationRejectUnknownKeys(formula, ["baseAdd", "terms"], label)
  specializationValidateFinite(formula.baseAdd, `${label}.baseAdd`)
  if (!Array.isArray(formula.terms) || formula.terms.length === 0) throw new Error(`${label}.terms must be a non-empty array`)
  for (let index = 0; index < formula.terms.length; index += 1) {
    const term = formula.terms[index]
    const termLabel = `${label}.terms[${index}]`
    if (!specializationIsObject(term)) throw new Error(`${termLabel} must be an object`)
    specializationRejectUnknownKeys(term, ["attributeKey", "coefficient"], termLabel)
    const attributeKey = specializationRequiredString(term.attributeKey, `${termLabel}.attributeKey`)
    if (!refs.attributeIDs.has(attributeKey)) throw new Error(`${termLabel}.attributeKey not found: ${attributeKey}`)
    specializationValidateFinite(term.coefficient, `${termLabel}.coefficient`, { nonZero: true })
  }
}

function specializationFormulaEnvelope(formula, attributeMaximum = 500) {
  let minimum = formula.baseAdd
  let maximum = formula.baseAdd
  for (const term of formula.terms || []) {
    const projected = term.coefficient * attributeMaximum
    minimum += Math.min(0, projected)
    maximum += Math.max(0, projected)
  }
  return { minimum, maximum }
}

function specializationFormulaSchoolAttributeIDs(value, out = new Set()) {
  if (Array.isArray(value)) {
    for (const entry of value) specializationFormulaSchoolAttributeIDs(entry, out)
    return out
  }
  if (!specializationIsObject(value)) return out
  if (typeof value.attributeKey === "string" && Number.isFinite(value.coefficient) && value.coefficient !== 0) {
    out.add(value.attributeKey.trim())
  }
  if (value.kind === "retaliate") {
    if (Number.isFinite(value.strengthRatio) && value.strengthRatio !== 0) out.add("strength")
    if (Number.isFinite(value.staminaRatio) && value.staminaRatio !== 0) out.add("stamina")
  }
  for (const child of Object.values(value)) specializationFormulaSchoolAttributeIDs(child, out)
  return out
}

function validateSpecializationValueFormula(value, label, refs, envelope = undefined) {
  if (value === undefined) return
  validateSpecializationFormula(value, `${label}.valueFormula`, refs)
  if (!envelope) return
  const { minimum, maximum } = specializationFormulaEnvelope(value, SPECIALIZATION_ATTRIBUTE_ENVELOPE_MAX)
  if (minimum < envelope.min || (envelope.minExclusive && minimum <= envelope.min)) {
    throw new Error(`${label}.valueFormula attribute-500 envelope minimum invalid: ${minimum}`)
  }
  if (maximum > envelope.max) throw new Error(`${label}.valueFormula attribute-500 envelope maximum invalid: ${maximum}`)
}

function validateSpecializationTriggerLimits(limits, label) {
  if (!specializationIsObject(limits)) throw new Error(`${label} must be an object`)
  const keys = ["perTurn", "perWave", "perBattle"]
  specializationRejectUnknownKeys(limits, keys, label)
  if (Object.keys(limits).length === 0) throw new Error(`${label} must not be empty`)
  for (const key of keys) {
    if (limits[key] !== undefined) specializationValidateFinite(limits[key], `${label}.${key}`, { integer: true, min: 1 })
  }
}

function validateSpecializationEventFilter(filter, label, refs, trigger) {
  if (!specializationIsObject(filter)) throw new Error(`${label} must be an object`)
  specializationRejectUnknownKeys(filter, [
    "skillIds",
    "excludeSkillIds",
    "skillCategories",
    "damageTypes",
    "turnKinds",
    "includeBasicAttack",
    "includePreparation",
    "includeSuppressedTurns",
    "excludeSelf",
    "buffIds",
    "excludeBuffIds",
    "buffKinds",
    "firstBuffInstanceOnly",
    "applicationKinds",
    "probabilityKinds",
    "sourceIds",
    "excludeSourceIds",
    "unitTypes"
  ], label)
  if (Object.keys(filter).length === 0) throw new Error(`${label} must not be empty`)
  for (const key of ["buffIds", "excludeBuffIds", "buffKinds", "firstBuffInstanceOnly", "applicationKinds"]) {
    if (filter[key] !== undefined) throw new Error(`${label}.${key} requires a buff trigger, which specializations do not expose`)
  }
  for (const key of ["probabilityKinds", "sourceIds", "excludeSourceIds"]) {
    if (filter[key] !== undefined) throw new Error(`${label}.${key} requires a probability trigger, which specializations do not expose`)
  }
  for (const key of ["skillIds", "excludeSkillIds", "skillCategories", "includeBasicAttack"]) {
    if (filter[key] !== undefined && !specializationTriggerHasSkillContext(trigger)) {
      throw new Error(`${label}.${key} requires a skill or damage trigger`)
    }
  }
  if (filter.damageTypes !== undefined && !specializationTriggerHasDamageContext(trigger)) {
    throw new Error(`${label}.damageTypes requires a damage trigger`)
  }
  for (const key of ["unitTypes", "excludeSelf"]) {
    if (filter[key] !== undefined && !specializationTriggerHasEventSubject(trigger)) {
      throw new Error(`${label}.${key} requires an event subject`)
    }
  }
  for (const key of ["skillIds", "excludeSkillIds"]) {
    if (filter[key] !== undefined) specializationValidateStringArray(filter[key], `${label}.${key}`, { knownValues: refs.skillIDs })
  }
  if (filter.skillCategories !== undefined) specializationValidateStringArray(filter.skillCategories, `${label}.skillCategories`, { knownValues: refs.skillCategories })
  if (filter.damageTypes !== undefined) specializationValidateStringArray(filter.damageTypes, `${label}.damageTypes`, { allowedValues: SPECIALIZATION_DAMAGE_TYPES })
  if (filter.turnKinds !== undefined) specializationValidateStringArray(filter.turnKinds, `${label}.turnKinds`, { allowedValues: new Set(["ordered", "extra"]) })
  for (const key of ["includeBasicAttack", "includePreparation", "includeSuppressedTurns", "excludeSelf", "firstBuffInstanceOnly"]) {
    if (filter[key] !== undefined && typeof filter[key] !== "boolean") throw new Error(`${label}.${key} must be boolean`)
  }
  for (const key of ["buffIds", "excludeBuffIds"]) {
    if (filter[key] !== undefined) specializationValidateStringArray(filter[key], `${label}.${key}`, { knownValues: refs.buffIDs })
  }
  if (filter.buffKinds !== undefined) specializationValidateStringArray(filter.buffKinds, `${label}.buffKinds`, { knownValues: refs.buffKinds })
  if (filter.applicationKinds !== undefined) specializationValidateStringArray(filter.applicationKinds, `${label}.applicationKinds`, { allowedValues: new Set(["attach", "refresh"]) })
  if (filter.probabilityKinds !== undefined) specializationValidateStringArray(filter.probabilityKinds, `${label}.probabilityKinds`, { allowedValues: new Set(["equipment_proc", "status_apply"]) })
  for (const key of ["sourceIds", "excludeSourceIds"]) {
    if (filter[key] !== undefined) specializationValidateStringArray(filter[key], `${label}.${key}`)
  }
  if (filter.unitTypes !== undefined) specializationValidateStringArray(filter.unitTypes, `${label}.unitTypes`, { allowedValues: new Set(["player", "kitty", "enemy", "summon"]) })
  const isTurnTrigger = trigger === "onTurnStart" || trigger === "onTurnEnd"
  if ((filter.turnKinds !== undefined || filter.includePreparation !== undefined) && !isTurnTrigger) throw new Error(`${label} turn filters require onTurnStart or onTurnEnd`)
  if (filter.includeSuppressedTurns !== undefined && trigger !== "onTurnEnd") throw new Error(`${label}.includeSuppressedTurns requires onTurnEnd`)
}

function validateSpecializationConditionTrigger(condition, label, trigger) {
  const kind = String(condition?.kind || "").trim()
  if ((kind === "context_damage_type" || kind === "context_hit_count_gte") && !specializationTriggerHasDamageContext(trigger)) {
    throw new Error(`${label}.${kind} requires a damage trigger`)
  }
  if (kind === "context_mp_insufficient" && trigger !== "onBeforeSkillCast") {
    throw new Error(`${label}.${kind} requires onBeforeSkillCast`)
  }
  if (kind === "context_skill_non_basic" && !specializationTriggerHasSkillContext(trigger)) {
    throw new Error(`${label}.${kind} requires a skill or damage trigger`)
  }
  if (kind === "incoming_damage_would_be_lethal" && trigger !== "onBeforeTakeDamage" && trigger !== "onAllyBeforeTakeDamage") {
    throw new Error(`${label}.${kind} requires a before-take-damage trigger`)
  }
  if (["source_is_enemy", "target_has_any_buff", "target_has_buff", "target_is_lowest_hp_ally"].includes(kind) && !specializationTriggerHasEventSubject(trigger)) {
    throw new Error(`${label}.${kind} requires an event subject`)
  }
}

function validateSpecializationHookEffectTrigger(effect, label, trigger) {
  const kind = String(effect?.kind || "").trim()
  const requiresOneOf = (allowed, requirement) => {
    if (!allowed.has(trigger)) throw new Error(`${label}.${kind} requires ${requirement}`)
  }
  if (["consume_counter_for_damage_amp", "flat_defense_penetration", "runtime_damage_amp"].includes(kind)) {
    requiresOneOf(new Set(["onBeforeDealDamage", "onBeforeTakeDamage"]), "a before-damage trigger")
  } else if (kind === "redirect_damage") {
    requiresOneOf(new Set(["onAllyBeforeTakeDamage"]), "onAllyBeforeTakeDamage")
  } else if (kind === "prevent_lethal" || kind === "resource_shield") {
    requiresOneOf(new Set(["onBeforeTakeDamage"]), "onBeforeTakeDamage")
  } else if (kind === "resource_debt_cast") {
    requiresOneOf(new Set(["onBeforeSkillCast"]), "onBeforeSkillCast")
  } else if (kind === "mirror_cast_skill") {
    requiresOneOf(new Set(["onAfterSkillCast"]), "onAfterSkillCast")
  } else if (kind === "buff_cycle") {
    requiresOneOf(new Set(["onAfterDealDamage"]), "onAfterDealDamage")
  } else if (kind === "repeat_context_effect") {
    requiresOneOf(new Set(["onBeforeDealDamage", "onBeforeTakeDamage", "onAfterDealDamage", "onAfterTakeDamage", "onAfterDealCrit"]), "a damage trigger")
  } else if (kind === "heal" && trigger === "onBeforeTakeDamage" && effect.targetMode !== "self") {
    throw new Error(`${label}.${kind} onBeforeTakeDamage targetMode must be self`)
  }
  if (kind === "adjust_skill_cooldown" && effect.selectionMode === "context_skill" && !specializationTriggerHasSkillContext(trigger)) {
    throw new Error(`${label}.${kind} context_skill requires a skill or damage trigger`)
  }
  if ((effect.targetMode === "source" || effect.targetMode === "targeting") && !specializationTriggerHasEventSubject(trigger)) {
    throw new Error(`${label}.${kind} targetMode ${effect.targetMode} requires an event subject`)
  }
}

function validateSpecializationCondition(condition, label, refs) {
  if (!specializationIsObject(condition)) throw new Error(`${label} must be an object`)
  const kind = specializationRequiredString(condition.kind, `${label}.kind`)
  const noPayloadKinds = new Set([
    "adjacent_ally_exists",
    "context_mp_insufficient",
    "context_skill_non_basic",
    "incoming_damage_would_be_lethal",
    "source_is_enemy",
    "target_is_lowest_hp_ally"
  ])
  if (noPayloadKinds.has(kind)) {
    specializationRejectUnknownKeys(condition, ["kind"], label)
    return
  }
  if (kind === "context_damage_type") {
    specializationRejectUnknownKeys(condition, ["kind", "value"], label)
    if (!SPECIALIZATION_DAMAGE_TYPES.has(condition.value)) throw new Error(`${label}.value invalid: ${condition.value}`)
    return
  }
  if (kind === "context_hit_count_gte") {
    specializationRejectUnknownKeys(condition, ["kind", "value"], label)
    specializationValidateFinite(condition.value, `${label}.value`, { integer: true, min: 1 })
    return
  }
  if (kind === "counter_gte") {
    specializationRejectUnknownKeys(condition, ["kind", "counterId", "value"], label)
    specializationRequiredString(condition.counterId, `${label}.counterId`)
    specializationValidateFinite(condition.value, `${label}.value`, { integer: true, min: 1 })
    return
  }
  if (kind === "self_hp_ratio_lte" || kind === "self_mp_ratio_gte") {
    specializationRejectUnknownKeys(condition, ["kind", "value"], label)
    specializationValidateFinite(condition.value, `${label}.value`, { min: 0, max: 1 })
    return
  }
  if (kind === "self_weapon_class") {
    specializationRejectUnknownKeys(condition, ["kind", "weaponClasses"], label)
    specializationValidateStringArray(condition.weaponClasses, `${label}.weaponClasses`, { knownValues: refs.weaponClasses })
    return
  }
  if (kind === "target_has_buff") {
    specializationRejectUnknownKeys(condition, ["kind", "buffId"], label)
    const buffID = specializationRequiredString(condition.buffId, `${label}.buffId`)
    if (!refs.buffIDs.has(buffID)) throw new Error(`${label}.buffId not found: ${buffID}`)
    return
  }
  if (kind === "target_has_any_buff") {
    specializationRejectUnknownKeys(condition, ["kind", "buffIds"], label)
    specializationValidateStringArray(condition.buffIds, `${label}.buffIds`, { knownValues: refs.buffIDs })
    return
  }
  if (kind === "trigger_limit") {
    specializationRejectUnknownKeys(condition, ["kind", "perTurn", "perWave", "perBattle"], label)
    const limits = Object.fromEntries(Object.entries(condition).filter(([key]) => key !== "kind"))
    validateSpecializationTriggerLimits(limits, label)
    return
  }
  throw new Error(`${label}.kind invalid: ${kind}`)
}

function validateSpecializationCounterPayoff(payoff, label, refs) {
  if (!specializationIsObject(payoff)) throw new Error(`${label} must be an object`)
  const kind = specializationRequiredString(payoff.kind, `${label}.kind`)
  if (kind === "empower_next_hit") {
    specializationRejectUnknownKeys(payoff, ["kind", "damageAmp", "consumeStacks", "valueFormula"], label)
    specializationValidateFinite(payoff.damageAmp, `${label}.damageAmp`, { min: 0, max: 0.45, nonZero: true })
  } else if (kind === "action_meter_gain") {
    throw new Error(`${label}.kind action_meter_gain is retired in specialization schema v2; use an integer trigger or explicit extra_turn`)
  } else if (kind === "restore_mp") {
    specializationRejectUnknownKeys(payoff, ["kind", "mpRatio", "consumeStacks", "valueFormula"], label)
    specializationValidateFinite(payoff.mpRatio, `${label}.mpRatio`, { min: 0, max: 0.5, nonZero: true })
  } else if (kind === "restore_mp_and_tick_cooldown") {
    specializationRejectUnknownKeys(payoff, ["kind", "mpRatio", "cooldownAmount", "consumeStacks", "valueFormula"], label)
    specializationValidateFinite(payoff.mpRatio, `${label}.mpRatio`, { min: 0, max: 0.5, nonZero: true })
    specializationValidateFinite(payoff.cooldownAmount, `${label}.cooldownAmount`, { integer: true, nonZero: true })
  } else if (kind === "guard_lowest_hp_ally") {
    specializationRejectUnknownKeys(payoff, ["kind", "damageReduction", "durationTurns", "consumeStacks", "valueFormula"], label)
    specializationValidateFinite(payoff.damageReduction, `${label}.damageReduction`, { min: 0, max: 0.5, nonZero: true })
    specializationValidateFinite(payoff.durationTurns, `${label}.durationTurns`, { integer: true, min: 1 })
  } else {
    throw new Error(`${label}.kind invalid: ${kind}`)
  }
  const payoffEnvelope = kind === "empower_next_hit" ? { min: 0, minExclusive: true, max: 0.45 }
    : kind === "restore_mp" || kind === "restore_mp_and_tick_cooldown" || kind === "guard_lowest_hp_ally" ? { min: 0, minExclusive: true, max: 0.5 }
      : undefined
  validateSpecializationValueFormula(payoff.valueFormula, label, refs, payoffEnvelope)
  specializationValidateFinite(payoff.consumeStacks, `${label}.consumeStacks`, { integer: true, min: 1 })
}

function validateSpecializationHookEffect(effect, label, refs) {
  if (!specializationIsObject(effect)) throw new Error(`${label} must be an object`)
  const kind = specializationRequiredString(effect.kind, `${label}.kind`)
  if (kind === "action_meter_gain") {
    throw new Error(`${label}.kind action_meter_gain is retired in specialization schema v2; use an integer trigger or explicit extra_turn`)
  }
  if (kind === "adjust_skill_cooldown") {
    specializationRejectUnknownKeys(effect, ["kind", "targetMode", "amount", "selectionMode", "maxSkills", "skillIds"], label)
    if (effect.targetMode !== undefined && !SPECIALIZATION_TARGET_MODES.has(effect.targetMode)) throw new Error(`${label}.targetMode invalid: ${effect.targetMode}`)
    specializationValidateFinite(effect.amount, `${label}.amount`, { integer: true, nonZero: true })
    if (effect.selectionMode !== undefined && !new Set(["max_remaining", "context_skill", "all"]).has(effect.selectionMode)) throw new Error(`${label}.selectionMode invalid: ${effect.selectionMode}`)
    if (effect.maxSkills !== undefined) specializationValidateFinite(effect.maxSkills, `${label}.maxSkills`, { integer: true, min: 1 })
    if (effect.selectionMode === "context_skill" && effect.maxSkills !== undefined && effect.maxSkills !== 1) {
      throw new Error(`${label}.maxSkills must be 1 when selectionMode is context_skill`)
    }
    if (effect.skillIds !== undefined) specializationValidateStringArray(effect.skillIds, `${label}.skillIds`, { knownValues: refs.skillIDs })
    if (effect.selectionMode === undefined && effect.skillIds === undefined) throw new Error(`${label} requires selectionMode or skillIds`)
    return
  }
  if (kind === "consume_counter_for_damage_amp") {
    specializationRejectUnknownKeys(effect, ["kind", "counterId", "consume", "value", "percentDefensePenetration", "valueFormula"], label)
    specializationRequiredString(effect.counterId, `${label}.counterId`)
    specializationValidateFinite(effect.consume, `${label}.consume`, { integer: true, min: 1 })
    specializationValidateFinite(effect.value, `${label}.value`, { min: 0, max: 0.8, nonZero: true })
    if (effect.percentDefensePenetration !== undefined) specializationValidateFinite(effect.percentDefensePenetration, `${label}.percentDefensePenetration`, { min: 0, max: 1, nonZero: true })
    validateSpecializationValueFormula(effect.valueFormula, label, refs, { min: 0, minExclusive: true, max: 0.8 })
    return
  }
  if (kind === "cycle_counter") {
    specializationRejectUnknownKeys(effect, ["kind", "counterId", "gain", "maxStacks", "atMax"], label)
    specializationRequiredString(effect.counterId, `${label}.counterId`)
    specializationValidateFinite(effect.gain, `${label}.gain`, { integer: true, min: 1 })
    specializationValidateFinite(effect.maxStacks, `${label}.maxStacks`, { integer: true, min: effect.gain })
    if (effect.atMax !== undefined) {
      validateSpecializationCounterPayoff(effect.atMax, `${label}.atMax`, refs)
      if (effect.atMax.consumeStacks > effect.maxStacks) throw new Error(`${label}.atMax.consumeStacks exceeds maxStacks`)
    }
    return
  }
  if (kind === "damage") {
    specializationRejectUnknownKeys(effect, ["kind", "targetMode", "damageType", "amountFormula", "dispatchHooks"], label)
    if (!SPECIALIZATION_TARGET_MODES.has(effect.targetMode)) throw new Error(`${label}.targetMode invalid: ${effect.targetMode}`)
    if (!SPECIALIZATION_DAMAGE_TYPES.has(effect.damageType)) throw new Error(`${label}.damageType invalid: ${effect.damageType}`)
    validateSpecializationFormula(effect.amountFormula, `${label}.amountFormula`, refs)
    if (typeof effect.dispatchHooks !== "boolean") throw new Error(`${label}.dispatchHooks must be boolean`)
    return
  }
  if (kind === "extra_turn") {
    specializationRejectUnknownKeys(effect, ["kind", "targetMode", "turnMode", "procChanceBase", "procChanceMax", "abortHookOnFail", "dispatchHooks"], label)
    if (effect.targetMode !== "self" || effect.turnMode !== "full") throw new Error(`${label} only supports self/full`)
    const chanceConfigured = effect.procChanceBase !== undefined || effect.procChanceMax !== undefined
    if (chanceConfigured) {
      if (effect.procChanceBase === undefined) throw new Error(`${label}.procChanceBase is required when probability is configured`)
      specializationValidateFinite(effect.procChanceBase, `${label}.procChanceBase`, { min: 0, max: 1 })
      if (effect.procChanceMax !== undefined) {
        specializationValidateFinite(effect.procChanceMax, `${label}.procChanceMax`, { min: 0, max: 1 })
        if (effect.procChanceMax < effect.procChanceBase) throw new Error(`${label}.procChanceMax must be at least procChanceBase`)
      }
    }
    if (effect.abortHookOnFail !== undefined && typeof effect.abortHookOnFail !== "boolean") throw new Error(`${label}.abortHookOnFail must be boolean`)
    if (effect.abortHookOnFail === true && !chanceConfigured) throw new Error(`${label}.abortHookOnFail requires procChanceBase`)
    if (typeof effect.dispatchHooks !== "boolean") throw new Error(`${label}.dispatchHooks must be boolean`)
    return
  }
  if (kind === "flat_defense_penetration") {
    specializationRejectUnknownKeys(effect, ["kind", "valueFormula"], label)
    validateSpecializationFormula(effect.valueFormula, `${label}.valueFormula`, refs)
    return
  }
  if (kind === "heal") {
    specializationRejectUnknownKeys(effect, ["kind", "targetMode", "maxHpRatio", "missingHpRatio", "valueFormula"], label)
    if (!SPECIALIZATION_TARGET_MODES.has(effect.targetMode)) throw new Error(`${label}.targetMode invalid: ${effect.targetMode}`)
    const ratioKeys = ["maxHpRatio", "missingHpRatio"].filter((key) => effect[key] !== undefined)
    if (ratioKeys.length !== 1) throw new Error(`${label} requires exactly one healing ratio`)
    specializationValidateFinite(effect[ratioKeys[0]], `${label}.${ratioKeys[0]}`, { min: 0, max: 0.5, nonZero: true })
    validateSpecializationValueFormula(effect.valueFormula, label, refs, { min: 0, minExclusive: true, max: 0.5 })
    return
  }
  if (kind === "buff_cycle") {
    specializationRejectUnknownKeys(effect, ["kind", "buffId", "firstHit", "buffedHit"], label)
    const buffID = specializationRequiredString(effect.buffId, `${label}.buffId`)
    if (!refs.buffIDs.has(buffID)) throw new Error(`${label}.buffId not found: ${buffID}`)
    if (effect.firstHit !== "apply_buff") throw new Error(`${label}.firstHit invalid: ${effect.firstHit}`)
    if (!specializationIsObject(effect.buffedHit)) throw new Error(`${label}.buffedHit must be an object`)
    specializationRejectUnknownKeys(effect.buffedHit, ["kind", "value", "consumeBuff", "valueFormula"], `${label}.buffedHit`)
    if (effect.buffedHit.kind !== "runtime_damage_amp") throw new Error(`${label}.buffedHit.kind invalid: ${effect.buffedHit.kind}`)
    specializationValidateFinite(effect.buffedHit.value, `${label}.buffedHit.value`, { min: 0, max: 0.8, nonZero: true })
    validateSpecializationValueFormula(effect.buffedHit.valueFormula, `${label}.buffedHit`, refs, { min: 0, minExclusive: true, max: 0.8 })
    if (typeof effect.buffedHit.consumeBuff !== "boolean") throw new Error(`${label}.buffedHit.consumeBuff must be boolean`)
    return
  }
  if (kind === "mirror_cast_skill") {
    specializationRejectUnknownKeys(effect, ["kind", "amountRatio", "applyCooldown", "allowMirroredCastTrigger", "skillIds", "dispatchHooks", "valueFormula"], label)
    specializationValidateFinite(effect.amountRatio, `${label}.amountRatio`, { min: 0, max: 0.75, nonZero: true })
    for (const key of ["applyCooldown", "allowMirroredCastTrigger"]) if (typeof effect[key] !== "boolean") throw new Error(`${label}.${key} must be boolean`)
    if (effect.skillIds !== undefined) specializationValidateStringArray(effect.skillIds, `${label}.skillIds`, { knownValues: refs.skillIDs })
    validateSpecializationValueFormula(effect.valueFormula, label, refs, { min: 0, minExclusive: true, max: 0.75 })
    if (typeof effect.dispatchHooks !== "boolean") throw new Error(`${label}.dispatchHooks must be boolean`)
    return
  }
  if (kind === "prevent_lethal") {
    specializationRejectUnknownKeys(effect, ["kind", "remainingHpRatio", "valueFormula"], label)
    specializationValidateFinite(effect.remainingHpRatio, `${label}.remainingHpRatio`, { min: 0, max: 0.45, nonZero: true })
    validateSpecializationValueFormula(effect.valueFormula, label, refs, { min: 0, minExclusive: true, max: 0.45 })
    return
  }
  if (kind === "redirect_damage") {
    specializationRejectUnknownKeys(effect, ["kind", "targetMode", "ratio", "valueFormula"], label)
    if (effect.targetMode !== "self") throw new Error(`${label}.targetMode must be self`)
    specializationValidateFinite(effect.ratio, `${label}.ratio`, { min: 0, max: 0.65, nonZero: true })
    validateSpecializationValueFormula(effect.valueFormula, label, refs, { min: 0, minExclusive: true, max: 0.65 })
    return
  }
  if (kind === "remove_buff_by_kind") {
    specializationRejectUnknownKeys(effect, ["kind", "buffKind", "selectionMode", "maxBuffs"], label)
    const buffKind = specializationRequiredString(effect.buffKind, `${label}.buffKind`)
    if (!refs.buffKinds.has(buffKind)) throw new Error(`${label}.buffKind not found: ${buffKind}`)
    if (effect.selectionMode !== "max_remaining") throw new Error(`${label}.selectionMode invalid: ${effect.selectionMode}`)
    specializationValidateFinite(effect.maxBuffs, `${label}.maxBuffs`, { integer: true, min: 1 })
    return
  }
  if (kind === "repeat_context_effect") {
    specializationRejectUnknownKeys(effect, ["kind", "amountRatio", "targetMode", "dispatchHooks", "valueFormula"], label)
    specializationValidateFinite(effect.amountRatio, `${label}.amountRatio`, { min: 0, max: 0.75, nonZero: true })
    if (!SPECIALIZATION_TARGET_MODES.has(effect.targetMode)) throw new Error(`${label}.targetMode invalid: ${effect.targetMode}`)
    validateSpecializationValueFormula(effect.valueFormula, label, refs, { min: 0, minExclusive: true, max: 0.75 })
    if (typeof effect.dispatchHooks !== "boolean") throw new Error(`${label}.dispatchHooks must be boolean`)
    return
  }
  if (kind === "resource_debt_cast") {
    specializationRejectUnknownKeys(effect, ["kind", "hpCostRatio", "cooldownDelta", "minimumHp", "valueFormula"], label)
    specializationValidateFinite(effect.hpCostRatio, `${label}.hpCostRatio`, { min: 0.05, max: 0.2, nonZero: true })
    specializationValidateFinite(effect.cooldownDelta, `${label}.cooldownDelta`, { integer: true, nonZero: true })
    specializationValidateFinite(effect.minimumHp, `${label}.minimumHp`, { integer: true, min: 1 })
    validateSpecializationValueFormula(effect.valueFormula, label, refs, { min: 0.05, max: 0.2 })
    return
  }
  if (kind === "resource_shield") {
    specializationRejectUnknownKeys(effect, ["kind", "resource", "absorbRatio", "resourcePerDamage", "valueFormula"], label)
    if (effect.resource !== "mp") throw new Error(`${label}.resource invalid: ${effect.resource}`)
    specializationValidateFinite(effect.absorbRatio, `${label}.absorbRatio`, { min: 0, max: 0.65, nonZero: true })
    specializationValidateFinite(effect.resourcePerDamage, `${label}.resourcePerDamage`, { min: 0, nonZero: true })
    validateSpecializationValueFormula(effect.valueFormula, label, refs, { min: 0, minExclusive: true, max: 0.65 })
    return
  }
  if (kind === "restore_mp") {
    specializationRejectUnknownKeys(effect, ["kind", "maxMpRatio", "valueFormula"], label)
    specializationValidateFinite(effect.maxMpRatio, `${label}.maxMpRatio`, { min: 0, max: 0.5, nonZero: true })
    validateSpecializationValueFormula(effect.valueFormula, label, refs, { min: 0, minExclusive: true, max: 0.5 })
    return
  }
  if (kind === "retaliate") {
    specializationRejectUnknownKeys(effect, ["kind", "targetMode", "strengthRatio", "staminaRatio", "dispatchHooks"], label)
    if (effect.targetMode !== "source") throw new Error(`${label}.targetMode must be source`)
    specializationValidateFinite(effect.strengthRatio, `${label}.strengthRatio`, { min: 0, nonZero: true })
    specializationValidateFinite(effect.staminaRatio, `${label}.staminaRatio`, { min: 0, nonZero: true })
    if (typeof effect.dispatchHooks !== "boolean") throw new Error(`${label}.dispatchHooks must be boolean`)
    return
  }
  if (kind === "runtime_damage_amp") {
    specializationRejectUnknownKeys(effect, ["kind", "value", "valueFormula"], label)
    specializationValidateFinite(effect.value, `${label}.value`, { min: 0, max: 0.8, nonZero: true })
    validateSpecializationValueFormula(effect.valueFormula, label, refs, { min: 0, minExclusive: true, max: 0.8 })
    return
  }
  if (kind === "shared_damage_reduction") {
    specializationRejectUnknownKeys(effect, ["kind", "value", "durationTurns", "valueFormula"], label)
    specializationValidateFinite(effect.value, `${label}.value`, { min: 0, max: 0.45, nonZero: true })
    specializationValidateFinite(effect.durationTurns, `${label}.durationTurns`, { integer: true, min: 1 })
    validateSpecializationValueFormula(effect.valueFormula, label, refs, { min: 0, minExclusive: true, max: 0.45 })
    return
  }
  if (kind === "shift_position") {
    specializationRejectUnknownKeys(effect, ["kind", "policy", "maxSteps"], label)
    if (effect.policy !== "toward_lowest_hp_ally") throw new Error(`${label}.policy invalid: ${effect.policy}`)
    specializationValidateFinite(effect.maxSteps, `${label}.maxSteps`, { integer: true, min: 1 })
    return
  }
  if (kind === "add_buff" || kind === "remove_buff") {
    const allowedKeys = kind === "add_buff"
      ? ["kind", "buffId", "durationTurns", "targetMode", "selectionMode", "dispatchHooks"]
      : ["kind", "buffId", "targetMode", "dispatchHooks"]
    specializationRejectUnknownKeys(effect, allowedKeys, label)
    const buffID = specializationRequiredString(effect.buffId, `${label}.buffId`)
    if (!refs.buffIDs.has(buffID)) throw new Error(`${label}.buffId not found: ${buffID}`)
    if (effect.durationTurns !== undefined) specializationValidateFinite(effect.durationTurns, `${label}.durationTurns`, { integer: true, min: 1 })
    if (effect.targetMode !== undefined && !SPECIALIZATION_TARGET_MODES.has(effect.targetMode)) throw new Error(`${label}.targetMode invalid: ${effect.targetMode}`)
    if (effect.selectionMode !== undefined && !new Set(["highest_attack", "highest_threat", "lowest_hp", "random"]).has(effect.selectionMode)) throw new Error(`${label}.selectionMode invalid: ${effect.selectionMode}`)
    if (effect.selectionMode !== undefined && effect.targetMode !== "enemy") throw new Error(`${label}.selectionMode requires enemy targetMode`)
    if (typeof effect.dispatchHooks !== "boolean") throw new Error(`${label}.dispatchHooks must be boolean`)
    return
  }
  if (kind === "cast_skill") {
    specializationRejectUnknownKeys(effect, ["kind", "skillId", "amountRatio", "dispatchHooks"], label)
    const skillID = specializationRequiredString(effect.skillId, `${label}.skillId`)
    if (!refs.skillIDs.has(skillID)) throw new Error(`${label}.skillId not found: ${skillID}`)
    if (effect.amountRatio !== undefined) specializationValidateFinite(effect.amountRatio, `${label}.amountRatio`, { min: 0, nonZero: true })
    if (typeof effect.dispatchHooks !== "boolean") throw new Error(`${label}.dispatchHooks must be boolean`)
    return
  }
  throw new Error(`${label}.kind invalid: ${kind}`)
}

function validateSpecializationRequirement(requirement, label, nodeMaxRanks, attributeIDs) {
  if (!specializationIsObject(requirement)) throw new Error(`${label} must be an object`)
  const kind = specializationRequiredString(requirement.kind, `${label}.kind`)
  if (kind === "node_rank") {
    specializationRejectUnknownKeys(requirement, ["kind", "nodeId", "minRank"], label)
    const nodeID = specializationRequiredString(requirement.nodeId, `${label}.nodeId`)
    if (!nodeMaxRanks.has(nodeID)) throw new Error(`${label}.nodeId not found: ${nodeID}`)
    if (!Number.isInteger(requirement.minRank) || requirement.minRank < 1 || requirement.minRank > nodeMaxRanks.get(nodeID)) {
      throw new Error(`${label}.minRank must be within the prerequisite node rank range`)
    }
    return nodeID
  }
  if (kind === "attribute_gte") {
    specializationRejectUnknownKeys(requirement, ["kind", "attributeKey", "value"], label)
    const attributeKey = specializationRequiredString(requirement.attributeKey, `${label}.attributeKey`)
    if (!attributeIDs.has(attributeKey)) throw new Error(`${label}.attributeKey not found: ${attributeKey}`)
    if (!Number.isFinite(requirement.value) || requirement.value < 0) throw new Error(`${label}.value must be a finite non-negative number`)
    return ""
  }
  if (kind === "pure_school_points_gte") {
    specializationRejectUnknownKeys(requirement, ["kind", "schoolId", "points"], label)
    const schoolID = specializationRequiredString(requirement.schoolId, `${label}.schoolId`)
    if (!SPECIALIZATION_SCHOOL_IDS.includes(schoolID)) throw new Error(`${label}.schoolId not found: ${schoolID}`)
    if (!Number.isInteger(requirement.points) || requirement.points < 1) throw new Error(`${label}.points must be a positive integer`)
    return ""
  }
  throw new Error(`${label}.kind invalid: ${kind}`)
}

function validateSpecializationBattleHooks(hooks, label, refs) {
  if (!Array.isArray(hooks) || hooks.length === 0) throw new Error(`${label} must be a non-empty array`)
  const hookIDs = new Set()
  for (let index = 0; index < hooks.length; index += 1) {
    const hook = hooks[index]
    const hookLabel = `${label}[${index}]`
    if (!specializationIsObject(hook)) throw new Error(`${hookLabel} must be an object`)
    specializationRejectUnknownKeys(hook, ["id", "trigger", "priority", "conditions", "effects", "eventFilter", "limits", "dispatchHooks"], hookLabel)
    const hookID = specializationRequiredString(hook.id, `${hookLabel}.id`)
    if (!SPECIALIZATION_PAYLOAD_ID_PATTERN.test(hookID)) throw new Error(`${hookLabel}.id must match ${SPECIALIZATION_PAYLOAD_ID_PATTERN}`)
    if (hookIDs.has(hookID)) throw new Error(`${hookLabel}.id duplicate: ${hookID}`)
    hookIDs.add(hookID)
    const trigger = specializationRequiredString(hook.trigger, `${hookLabel}.trigger`)
    if (!SPECIALIZATION_BATTLE_TRIGGERS.has(trigger)) throw new Error(`${hookLabel}.trigger invalid: ${trigger}`)
    specializationValidateFinite(hook.priority, `${hookLabel}.priority`, { integer: true })
    if (hook.eventFilter !== undefined) validateSpecializationEventFilter(hook.eventFilter, `${hookLabel}.eventFilter`, refs, trigger)
    if (hook.limits !== undefined) validateSpecializationTriggerLimits(hook.limits, `${hookLabel}.limits`)
    if (typeof hook.dispatchHooks !== "boolean") throw new Error(`${hookLabel}.dispatchHooks must be boolean`)
    if (!Array.isArray(hook.conditions)) throw new Error(`${hookLabel}.conditions must be an array`)
    for (let conditionIndex = 0; conditionIndex < hook.conditions.length; conditionIndex += 1) {
      const conditionLabel = `${hookLabel}.conditions[${conditionIndex}]`
      validateSpecializationCondition(hook.conditions[conditionIndex], conditionLabel, refs)
      validateSpecializationConditionTrigger(hook.conditions[conditionIndex], conditionLabel, trigger)
    }
    if (!Array.isArray(hook.effects) || hook.effects.length === 0) throw new Error(`${hookLabel}.effects must be a non-empty array`)
    for (let effectIndex = 0; effectIndex < hook.effects.length; effectIndex += 1) {
      const effectLabel = `${hookLabel}.effects[${effectIndex}]`
      validateSpecializationHookEffect(hook.effects[effectIndex], effectLabel, refs)
      validateSpecializationHookEffectTrigger(hook.effects[effectIndex], effectLabel, trigger)
    }
    const grantsDirectExtraTurn = hook.effects.some((effect) => effect.kind === "extra_turn")
    const directExtraTurnLimits = [hook.limits, ...hook.conditions.filter((condition) => condition.kind === "trigger_limit")]
      .filter(specializationIsObject)
    const hasOneShotOuterLimit = directExtraTurnLimits.some((limit) => limit.perWave === 1 || limit.perBattle === 1)
    if (grantsDirectExtraTurn && !hasOneShotOuterLimit) {
      throw new Error(`${hookLabel} direct extra_turn requires an explicit perWave=1 or perBattle=1 outer limit`)
    }
  }
}

function validateSpecializationRiskPolicy(node, label) {
  const schoolIDs = new Set(node.schoolIds || [])
  for (let levelIndex = 0; levelIndex < node.levels.length; levelIndex += 1) {
    for (const grant of node.levels[levelIndex].grants || []) {
      for (const mutation of grant.effectMutations || []) {
        if (Number(mutation.ops?.hitCountDelta) > 0 && (!Array.isArray(mutation.weaponClasses) || mutation.weaponClasses.length === 0)) {
          throw new Error(`${label}.levels[${levelIndex}] extra-hit mutation requires an explicit weaponClasses restriction`)
        }
        if ((mutation.ops?.cooldownDelta || 0) >= 0) continue
        if (!schoolIDs.has("intelligence")) throw new Error(`${label}.levels[${levelIndex}] cooldown reduction is restricted to Intelligence routes`)
        if ((mutation.ops?.cooldownFloor || 0) < 1) throw new Error(`${label}.levels[${levelIndex}] cooldown reduction requires cooldownFloor >= 1`)
        if ((mutation.ops?.mpCostMultiplier || 0) < 3) throw new Error(`${label}.levels[${levelIndex}] cooldown reduction requires mpCostMultiplier >= 3`)
        if (!Array.isArray(mutation.weaponClasses) || mutation.weaponClasses.length === 0) {
          throw new Error(`${label}.levels[${levelIndex}] cooldown reduction requires an explicit weaponClasses restriction`)
        }
      }
      for (const hook of grant.battleHooks || []) {
        const conditions = Array.isArray(hook.conditions) ? hook.conditions : []
        const hasWeaponGate = conditions.some((condition) => condition.kind === "self_weapon_class" && Array.isArray(condition.weaponClasses) && condition.weaponClasses.length > 0)
        const hasOneShotLimit = [hook.limits, ...conditions.filter((condition) => condition.kind === "trigger_limit")]
          .filter(specializationIsObject)
          .some((limit) => limit.perWave === 1 || limit.perBattle === 1)
        const mpGate = conditions.find((condition) => condition.kind === "self_mp_ratio_gte")
        for (const effect of hook.effects || []) {
          if (effect.kind === "adjust_skill_cooldown" && Number(effect.amount) < 0) {
            throw new Error(`${label}.levels[${levelIndex}] runtime cooldown reduction is forbidden; use the gated Intelligence mutation contract`)
          }
          if (effect.kind === "cycle_counter") {
            const payoff = effect.atMax || {}
            if (payoff.kind === "restore_mp_and_tick_cooldown" && Number(payoff.cooldownAmount) < 0) {
              throw new Error(`${label}.levels[${levelIndex}] counter payoff cooldown reduction is forbidden`)
            }
            if (payoff.kind === "extra_turn" || payoff.kind === "immediate_action") {
              throw new Error(`${label}.levels[${levelIndex}] nested counter extra turns are forbidden`)
            }
          }
          if (effect.kind === "extra_turn") {
            if (!hasWeaponGate || !hasOneShotLimit) throw new Error(`${label}.levels[${levelIndex}] extra_turn requires a weapon gate and perWave/perBattle limit`)
            if (!(Number(effect.procChanceBase) > 0) || !(Number(effect.procChanceMax) > 0) || Number(effect.procChanceMax) > 0.4 || Number(effect.procChanceBase) > Number(effect.procChanceMax)) {
              throw new Error(`${label}.levels[${levelIndex}] extra_turn chance must be explicit and capped at 40%`)
            }
            if (effect.abortHookOnFail !== true || effect.dispatchHooks !== false) throw new Error(`${label}.levels[${levelIndex}] extra_turn must abort on failure and must not dispatch derived hooks`)
          }
          if (effect.kind === "mirror_cast_skill") {
            if (!hasWeaponGate || !hasOneShotLimit || !conditions.some((condition) => condition.kind === "context_skill_non_basic") || Number(mpGate?.value) < 0.75) {
              throw new Error(`${label}.levels[${levelIndex}] mirror_cast_skill requires a weapon gate, non-basic skill, 75% MP gate, and per-wave/per-battle limit`)
            }
            if (effect.allowMirroredCastTrigger !== false || effect.dispatchHooks !== false || effect.valueFormula !== undefined || Number(effect.amountRatio) > 0.55) {
              throw new Error(`${label}.levels[${levelIndex}] mirror_cast_skill must be non-recursive and capped at 55%`)
            }
          }
          if (effect.kind === "repeat_context_effect") {
            const hasAnyLimit = [hook.limits, ...conditions.filter((condition) => condition.kind === "trigger_limit")]
              .filter(specializationIsObject)
              .some((limit) => Number(limit.perTurn || limit.perWave || limit.perBattle) > 0)
            if (!hasWeaponGate || !hasAnyLimit) throw new Error(`${label}.levels[${levelIndex}] repeat_context_effect requires a weapon gate and explicit trigger limit`)
            if (effect.dispatchHooks !== false || effect.valueFormula !== undefined || !(Number(effect.amountRatio) > 0) || Number(effect.amountRatio) > 0.45) {
              throw new Error(`${label}.levels[${levelIndex}] repeat_context_effect must be non-recursive and capped at 45%`)
            }
          }
        }
      }
    }
  }
}

function validateSpecializationTargetingPatch(patch, label, refs) {
  if (!specializationIsObject(patch)) throw new Error(`${label} must be an object`)
  specializationRejectUnknownKeys(patch, ["targetScope", "targetPositions", "secondarySeek", "selectRule", "selectAttrKey", "fallbackSelfIfEmpty", "targetCount"], label)
  if (Object.keys(patch).length === 0) throw new Error(`${label} must not be empty`)
  if (patch.targetScope !== undefined && !new Set(["self", "ally", "enemy", "all"]).has(patch.targetScope)) throw new Error(`${label}.targetScope invalid: ${patch.targetScope}`)
  if (patch.targetPositions !== undefined) specializationValidateStringArray(patch.targetPositions, `${label}.targetPositions`, { allowedValues: new Set(["vanguard", "midguard", "rearguard"]) })
  if (patch.secondarySeek !== undefined && !new Set(["front_to_rear", "rear_to_front", "random_alive", "none"]).has(patch.secondarySeek)) throw new Error(`${label}.secondarySeek invalid: ${patch.secondarySeek}`)
  if (patch.selectRule !== undefined && !new Set(["hp_max", "hp_min", "hp_percent_max", "hp_percent_min", "first", "boss_first", "attr_max", "attr_min", "random", "self"]).has(patch.selectRule)) throw new Error(`${label}.selectRule invalid: ${patch.selectRule}`)
  if (patch.selectAttrKey !== undefined) {
    const attributeKey = specializationRequiredString(patch.selectAttrKey, `${label}.selectAttrKey`)
    if (!refs.attributeIDs.has(attributeKey)) throw new Error(`${label}.selectAttrKey not found: ${attributeKey}`)
  }
  const usesAttributeRule = patch.selectRule === "attr_max" || patch.selectRule === "attr_min"
  const hasSelectAttribute = patch.selectAttrKey !== undefined
  if (usesAttributeRule !== hasSelectAttribute) {
    throw new Error(`${label} selectRule attr_max/attr_min and selectAttrKey must be authored together`)
  }
  if (patch.fallbackSelfIfEmpty !== undefined && typeof patch.fallbackSelfIfEmpty !== "boolean") throw new Error(`${label}.fallbackSelfIfEmpty must be boolean`)
  if (patch.targetCount !== undefined) specializationValidateFinite(patch.targetCount, `${label}.targetCount`, { integer: true, min: 1 })
}

function validateSpecializationMutationAppendEffect(effect, label, refs) {
  if (!specializationIsObject(effect)) throw new Error(`${label} must be an object`)
  specializationRequiredString(effect.id, `${label}.id`)
  const kind = specializationRequiredString(effect.kind, `${label}.kind`)
  if (kind === "consume_hp") {
    specializationRejectUnknownKeys(effect, ["id", "kind", "maxHpRatio", "minimumHp"], label)
    specializationValidateFinite(effect.maxHpRatio, `${label}.maxHpRatio`, { min: 0, max: 1, nonZero: true })
    specializationValidateFinite(effect.minimumHp, `${label}.minimumHp`, { integer: true, min: 1 })
    return
  }
  if (kind === "runtime_damage_amp") {
    specializationRejectUnknownKeys(effect, ["id", "kind", "value"], label)
    specializationValidateFinite(effect.value, `${label}.value`, { min: 0, nonZero: true })
    return
  }
  if (kind === "splash_damage") {
    specializationRejectUnknownKeys(effect, ["id", "kind", "targetCount", "amountRatio"], label)
    specializationValidateFinite(effect.targetCount, `${label}.targetCount`, { integer: true, min: 1 })
    specializationValidateFinite(effect.amountRatio, `${label}.amountRatio`, { min: 0, nonZero: true })
    return
  }
  if (kind === "secondary_damage") {
    specializationRejectUnknownKeys(effect, ["id", "kind", "targetPolicy", "targetPosition", "amountRatio"], label)
    if (!new Set(["same_row", "fixed_row", "all_rows"]).has(effect.targetPolicy)) throw new Error(`${label}.targetPolicy invalid: ${effect.targetPolicy}`)
    specializationValidateFinite(effect.amountRatio, `${label}.amountRatio`, { min: 0, max: 0.5, nonZero: true })
    if (effect.targetPolicy === "fixed_row") {
      if (!new Set(["vanguard", "midguard", "rearguard"]).has(effect.targetPosition)) throw new Error(`${label}.targetPosition must be vanguard, midguard, or rearguard for fixed_row`)
    } else if (effect.targetPosition !== undefined) {
      throw new Error(`${label}.targetPosition is only valid for fixed_row`)
    }
    return
  }
  if (kind === "status") {
    specializationRejectUnknownKeys(effect, ["id", "kind", "statusPayload"], label)
    if (!specializationIsObject(effect.statusPayload)) throw new Error(`${label}.statusPayload must be an object`)
    specializationRejectUnknownKeys(effect.statusPayload, ["statusIds", "durationBaseTurns", "durationPerSkillLevelTurns"], `${label}.statusPayload`)
    specializationValidateStringArray(effect.statusPayload.statusIds, `${label}.statusPayload.statusIds`, { knownValues: refs.buffIDs })
    specializationValidateFinite(effect.statusPayload.durationBaseTurns, `${label}.statusPayload.durationBaseTurns`, { integer: true, min: 1 })
    specializationValidateFinite(effect.statusPayload.durationPerSkillLevelTurns, `${label}.statusPayload.durationPerSkillLevelTurns`, { integer: true, min: 0 })
    return
  }
  throw new Error(`${label}.kind invalid: ${kind}`)
}

function validateSpecializationEffectMutations(mutations, label, refs) {
  if (!Array.isArray(mutations) || mutations.length === 0) throw new Error(`${label} must be a non-empty array`)
  const mutationIDs = new Set()
  for (let index = 0; index < mutations.length; index += 1) {
    const mutation = mutations[index]
    const mutationLabel = `${label}[${index}]`
    if (!specializationIsObject(mutation)) throw new Error(`${mutationLabel} must be an object`)
    specializationRejectUnknownKeys(mutation, ["id", "priority", "scope", "skillIds", "skillCategories", "weaponClasses", "damageTypes", "effectSelector", "ops"], mutationLabel)
    const mutationID = specializationRequiredString(mutation.id, `${mutationLabel}.id`)
    if (!SPECIALIZATION_PAYLOAD_ID_PATTERN.test(mutationID)) throw new Error(`${mutationLabel}.id must match ${SPECIALIZATION_PAYLOAD_ID_PATTERN}`)
    if (mutationIDs.has(mutationID)) throw new Error(`${mutationLabel}.id duplicate: ${mutationID}`)
    mutationIDs.add(mutationID)
    if (mutation.priority !== undefined) specializationValidateFinite(mutation.priority, `${mutationLabel}.priority`, { integer: true })
    if (mutation.scope !== "skill" && mutation.scope !== "basic_attack" && mutation.scope !== "both") {
      throw new Error(`${mutationLabel}.scope invalid: ${mutation.scope}`)
    }
    if (mutation.skillIds !== undefined) specializationValidateStringArray(mutation.skillIds, `${mutationLabel}.skillIds`, { knownValues: refs.skillIDs })
    if (mutation.skillCategories !== undefined) specializationValidateStringArray(mutation.skillCategories, `${mutationLabel}.skillCategories`, { knownValues: refs.skillCategories })
    if (mutation.weaponClasses !== undefined) specializationValidateStringArray(mutation.weaponClasses, `${mutationLabel}.weaponClasses`, { knownValues: refs.weaponClasses })
    if (mutation.damageTypes !== undefined) specializationValidateStringArray(mutation.damageTypes, `${mutationLabel}.damageTypes`, { allowedValues: SPECIALIZATION_DAMAGE_TYPES })
    if (!specializationIsObject(mutation.effectSelector)) throw new Error(`${mutationLabel}.effectSelector must be an object`)
    const mode = mutation.effectSelector.mode
    if (mode === "index" || mutation.effectSelector.indexes !== undefined) {
      throw new Error(`${mutationLabel}.effectSelector must not use unstable array indexes`)
    }
    if (mode !== "all" && mode !== "kind" && mode !== "effect_id") throw new Error(`${mutationLabel}.effectSelector.mode invalid: ${mode}`)
    const selectorKeys = mode === "all" ? ["mode", "includeGenerated"] : mode === "kind" ? ["mode", "kinds", "includeGenerated"] : ["mode", "effectIds", "includeGenerated"]
    specializationRejectUnknownKeys(mutation.effectSelector, selectorKeys, `${mutationLabel}.effectSelector`)
    if (mutation.effectSelector.includeGenerated !== undefined && typeof mutation.effectSelector.includeGenerated !== "boolean") {
      throw new Error(`${mutationLabel}.effectSelector.includeGenerated must be boolean`)
    }
    if (mode === "kind") specializationValidateStringArray(mutation.effectSelector.kinds, `${mutationLabel}.effectSelector.kinds`, { allowedValues: SPECIALIZATION_MUTATION_EFFECT_KINDS, knownValues: refs.skillEffectKinds })
    if (mode === "effect_id") specializationValidateStringArray(mutation.effectSelector.effectIds, `${mutationLabel}.effectSelector.effectIds`, { knownValues: refs.effectIDs })
    if (!specializationIsObject(mutation.ops) || Object.keys(mutation.ops).length === 0) throw new Error(`${mutationLabel}.ops must be a non-empty object`)
    specializationRejectUnknownKeys(mutation.ops, [
      "targetCountDelta",
      "amountBaseAddDelta",
      "cooldownDelta",
      "cooldownFloor",
      "mpCostDelta",
      "mpCostMultiplier",
      "hitCountDelta",
      "amountMultiplier",
      "amountTermsAppend",
      "statusIdsAppend",
      "statusIdsRemove",
      "appendEffects",
      "attackStatPolicy",
      "damageTypePolicy",
      "targetingPatch"
    ], `${mutationLabel}.ops`)
    for (const key of ["targetCountDelta", "cooldownDelta", "mpCostDelta", "hitCountDelta"]) {
      if (mutation.ops[key] !== undefined) specializationValidateFinite(mutation.ops[key], `${mutationLabel}.ops.${key}`, { integer: true, nonZero: true })
    }
    if (mutation.ops.cooldownFloor !== undefined) specializationValidateFinite(mutation.ops.cooldownFloor, `${mutationLabel}.ops.cooldownFloor`, { integer: true, min: 1 })
    if (mutation.ops.mpCostMultiplier !== undefined && (!Number.isFinite(mutation.ops.mpCostMultiplier) || mutation.ops.mpCostMultiplier <= 0)) {
      throw new Error(`${mutationLabel}.ops.mpCostMultiplier must be a positive finite number`)
    }
    if (Number(mutation.ops.cooldownDelta) < 0) {
      if (!Number.isInteger(mutation.ops.cooldownFloor) || mutation.ops.cooldownFloor < 1) {
        throw new Error(`${mutationLabel}.ops cooldown reduction requires cooldownFloor >= 1`)
      }
      if (!Number.isFinite(mutation.ops.mpCostMultiplier) || mutation.ops.mpCostMultiplier < 3) {
        throw new Error(`${mutationLabel}.ops cooldown reduction requires mpCostMultiplier >= 3`)
      }
      if (Number(mutation.ops.mpCostDelta || 0) < 0) {
        throw new Error(`${mutationLabel}.ops cooldown reduction requires mpCostDelta >= 0`)
      }
    }
    if (Number(mutation.ops.hitCountDelta) > 0 && (!Array.isArray(mutation.weaponClasses) || mutation.weaponClasses.length === 0)) {
      throw new Error(`${mutationLabel}.ops extra-hit mutation requires an explicit weaponClasses restriction`)
    }
    if (mutation.ops.amountBaseAddDelta !== undefined) specializationValidateFinite(mutation.ops.amountBaseAddDelta, `${mutationLabel}.ops.amountBaseAddDelta`, { nonZero: true })
    if (mutation.ops.amountMultiplier !== undefined && (!Number.isFinite(mutation.ops.amountMultiplier) || mutation.ops.amountMultiplier <= 0)) {
      throw new Error(`${mutationLabel}.ops.amountMultiplier must be a positive finite number`)
    }
    if (mutation.ops.amountTermsAppend !== undefined) {
      if (!Array.isArray(mutation.ops.amountTermsAppend) || mutation.ops.amountTermsAppend.length === 0) throw new Error(`${mutationLabel}.ops.amountTermsAppend must be a non-empty array`)
      if (mutation.ops.amountTermsAppend.length > 2) throw new Error(`${mutationLabel}.ops.amountTermsAppend supports at most two bounded attribute terms`)
      const termAttributes = new Set()
      for (let termIndex = 0; termIndex < mutation.ops.amountTermsAppend.length; termIndex += 1) {
        const term = mutation.ops.amountTermsAppend[termIndex]
        const termLabel = `${mutationLabel}.ops.amountTermsAppend[${termIndex}]`
        if (!specializationIsObject(term)) throw new Error(`${termLabel} must be an object`)
        specializationRejectUnknownKeys(term, ["attributeKey", "coefficient"], termLabel)
        const attributeKey = specializationRequiredString(term.attributeKey, `${termLabel}.attributeKey`)
        if (!refs.attributeIDs.has(attributeKey)) throw new Error(`${termLabel}.attributeKey not found: ${attributeKey}`)
        if (termAttributes.has(attributeKey)) throw new Error(`${mutationLabel}.ops.amountTermsAppend duplicate attribute: ${attributeKey}`)
        termAttributes.add(attributeKey)
        specializationValidateFinite(term.coefficient, `${termLabel}.coefficient`, { min: 0, nonZero: true })
      }
      const positiveHitDelta = Math.max(0, mutation.ops.hitCountDelta || 0)
      const positiveTargetDelta = Math.max(0, mutation.ops.targetCountDelta || 0)
      if (positiveHitDelta > 1 || positiveTargetDelta > 1 || (positiveHitDelta > 0 && positiveTargetDelta > 0)) {
        throw new Error(`${mutationLabel}.ops expansion envelope allows at most one extra hit or one extra target, never both`)
      }
      const amountMultiplier = mutation.ops.amountMultiplier ?? 1
      if ((positiveHitDelta > 0 || positiveTargetDelta > 0) && (amountMultiplier < 0.5 || amountMultiplier > 0.8)) {
        throw new Error(`${mutationLabel}.ops.amountMultiplier must be within 0.5-0.8 when adding a hit or target`)
      }
      const hasNarrowingFilter = mutation.skillIds !== undefined || mutation.skillCategories !== undefined || mutation.weaponClasses !== undefined || mutation.damageTypes !== undefined
      const broadSkillSelector = mutation.scope === "skill" && !hasNarrowingFilter
        && (mutation.effectSelector.mode === "all" || mutation.effectSelector.mode === "kind")
      const isWide = mutation.scope === "both" || (mutation.effectSelector.mode === "all" && !hasNarrowingFilter) || broadSkillSelector
      const perTermLimit = isWide ? 0.04 : 0.05
      const sumLimit = isWide ? 0.05 : 0.08
      const projectionLimit = isWide ? 25 : 40
      const relativeLimit = isWide ? 0.12 : 0.22
      const coefficientSum = mutation.ops.amountTermsAppend.reduce((sum, term) => sum + term.coefficient, 0)
      if (mutation.ops.amountTermsAppend.some((term) => term.coefficient > perTermLimit) || coefficientSum > sumLimit) {
        throw new Error(`${mutationLabel}.ops.amountTermsAppend exceeds the ${isWide ? "wide" : "restricted"} per-term/total coefficient envelope`)
      }
      if (coefficientSum * 500 > projectionLimit) throw new Error(`${mutationLabel}.ops.amountTermsAppend exceeds the attribute-500 projection envelope`)
      const coverageAdjusted = coefficientSum / 0.55 * (1 + positiveHitDelta) * (1 + positiveTargetDelta) * amountMultiplier
      if (coverageAdjusted > relativeLimit) throw new Error(`${mutationLabel}.ops.amountTermsAppend exceeds the hit/target coverage envelope`)
    }
    for (const key of ["statusIdsAppend", "statusIdsRemove"]) {
      if (mutation.ops[key] !== undefined) specializationValidateStringArray(mutation.ops[key], `${mutationLabel}.ops.${key}`, { knownValues: refs.buffIDs })
    }
    if (mutation.ops.appendEffects !== undefined) {
      if (!Array.isArray(mutation.ops.appendEffects) || mutation.ops.appendEffects.length === 0) throw new Error(`${mutationLabel}.ops.appendEffects must be a non-empty array`)
      const appendedEffectIDs = new Set()
      for (let effectIndex = 0; effectIndex < mutation.ops.appendEffects.length; effectIndex += 1) {
        const effect = mutation.ops.appendEffects[effectIndex]
        validateSpecializationMutationAppendEffect(effect, `${mutationLabel}.ops.appendEffects[${effectIndex}]`, refs)
        if (appendedEffectIDs.has(effect.id)) throw new Error(`${mutationLabel}.ops.appendEffects id duplicate: ${effect.id}`)
        appendedEffectIDs.add(effect.id)
      }
      const secondaryDamageCount = mutation.ops.appendEffects.filter((effect) => effect?.kind === "secondary_damage").length
      if (secondaryDamageCount > 1) {
        throw new Error(`${mutationLabel}.ops.appendEffects supports at most one secondary_damage`)
      }
      if (secondaryDamageCount === 1 && (
        mutation.ops.appendEffects.some((effect) => effect?.kind === "splash_damage")
        || Number(mutation.ops.targetCountDelta) > 0
        || Number(mutation.ops.hitCountDelta) > 0
      )) {
        throw new Error(`${mutationLabel}.ops secondary_damage cannot be combined with another hit or target expansion`)
      }
    }
    if (mutation.ops.attackStatPolicy !== undefined) {
      if (mutation.ops.attackStatPolicy !== "higher") throw new Error(`${mutationLabel}.ops.attackStatPolicy invalid: ${mutation.ops.attackStatPolicy}`)
      if (mode === "all") throw new Error(`${mutationLabel}.ops.attackStatPolicy requires a damage-only selector`)
    }
    if (mutation.ops.damageTypePolicy !== undefined && !new Set(["preserve", "physical", "magical", "higher_final_damage"]).has(mutation.ops.damageTypePolicy)) throw new Error(`${mutationLabel}.ops.damageTypePolicy invalid: ${mutation.ops.damageTypePolicy}`)
    if (mutation.ops.targetingPatch !== undefined) validateSpecializationTargetingPatch(mutation.ops.targetingPatch, `${mutationLabel}.ops.targetingPatch`, refs)
  }
}

function validateSpecializationGrant(grant, label, refs) {
  if (!specializationIsObject(grant)) throw new Error(`${label} must be an object`)
  const grantID = specializationRequiredString(grant.id, `${label}.id`)
  if (!SPECIALIZATION_PAYLOAD_ID_PATTERN.test(grantID)) throw new Error(`${label}.id must match ${SPECIALIZATION_PAYLOAD_ID_PATTERN}`)
  const kind = specializationRequiredString(grant.kind, `${label}.kind`)
  if (kind === "stat_modifier") {
    specializationRejectUnknownKeys(grant, ["id", "kind", "scope", "modifiers"], label)
    if (grant.scope !== "character" && grant.scope !== "battle") throw new Error(`${label}.scope invalid: ${grant.scope}`)
    if (!Array.isArray(grant.modifiers) || grant.modifiers.length === 0) throw new Error(`${label}.modifiers must be a non-empty array`)
    for (let index = 0; index < grant.modifiers.length; index += 1) {
      const modifier = grant.modifiers[index]
      const modifierLabel = `${label}.modifiers[${index}]`
      if (!specializationIsObject(modifier)) throw new Error(`${modifierLabel} must be an object`)
      specializationRejectUnknownKeys(modifier, ["attributeKey", "operation", "value"], modifierLabel)
      const attributeKey = specializationRequiredString(modifier.attributeKey, `${modifierLabel}.attributeKey`)
      if (!refs.attributeIDs.has(attributeKey)) throw new Error(`${modifierLabel}.attributeKey not found: ${attributeKey}`)
      if (modifier.operation !== "add" && modifier.operation !== "multiply") throw new Error(`${modifierLabel}.operation invalid: ${modifier.operation}`)
      if (!Number.isFinite(modifier.value) || modifier.value === 0) throw new Error(`${modifierLabel}.value must be finite and non-zero`)
    }
    return
  }
  if (kind === "battle_hook") {
    specializationRejectUnknownKeys(grant, ["id", "kind", "battleHooks"], label)
    validateSpecializationBattleHooks(grant.battleHooks, `${label}.battleHooks`, refs)
    validateSpecializationBattleHookCounterCapacity(grant.battleHooks, `${label}.battleHooks`)
    return
  }
  if (kind === "effect_mutation") {
    specializationRejectUnknownKeys(grant, ["id", "kind", "effectMutations"], label)
    validateSpecializationEffectMutations(grant.effectMutations, `${label}.effectMutations`, refs)
    return
  }
  if (kind === "equipment_slot_capacity") {
    specializationRejectUnknownKeys(grant, ["id", "kind", "equipType", "deltas"], label)
    if (grant.equipType !== "battle") throw new Error(`${label}.equipType must be battle`)
    if (!specializationIsObject(grant.deltas) || Object.keys(grant.deltas).length === 0) throw new Error(`${label}.deltas must be a non-empty object`)
    let hasPositive = false
    let hasNegative = false
    for (const [family, delta] of Object.entries(grant.deltas)) {
      if (!SPECIALIZATION_SLOT_FAMILIES[family]) throw new Error(`${label}.deltas family invalid: ${family}`)
      if (!Number.isInteger(delta) || delta === 0) throw new Error(`${label}.deltas.${family} must be a non-zero integer`)
      hasPositive ||= delta > 0
      hasNegative ||= delta < 0
    }
    if (!hasPositive || !hasNegative) throw new Error(`${label}.deltas must exchange capacity instead of granting a free slot`)
    return
  }
  throw new Error(`${label}.kind invalid: ${kind}`)
}

const SPECIALIZATION_UNORDERED_CORE_ARRAY_KEYS = new Set([
  "grants", "battleHooks", "effectMutations", "appendEffects", "conditions",
  "skillIds", "skillCategories", "weaponClasses", "damageTypes", "kinds", "effectIds"
])

function specializationCanonicalCoreSnapshot(value, containerKey = "") {
  if (Array.isArray(value)) {
    const entries = value.map((entry) => specializationCanonicalCoreSnapshot(entry))
    if (SPECIALIZATION_UNORDERED_CORE_ARRAY_KEYS.has(containerKey)) {
      entries.sort((left, right) => {
        const idOrder = String(left?.id || "").localeCompare(String(right?.id || ""))
        return idOrder || JSON.stringify(left).localeCompare(JSON.stringify(right))
      })
    }
    return entries
  }
  if (!specializationIsObject(value)) return value
  return Object.fromEntries(Object.keys(value).sort().map((key) => [key, specializationCanonicalCoreSnapshot(value[key], key)]))
}

function specializationHookEffectStateBindings(effects) {
  const bindings = new Map()
  const occurrenceByKind = new Map()
  for (const effect of Array.isArray(effects) ? effects : []) {
    if (!specializationIsObject(effect)) continue
    const kind = String(effect.kind || "").trim()
    const occurrence = occurrenceByKind.get(kind) || 0
    occurrenceByKind.set(kind, occurrence + 1)
    for (const field of ["counterId", "markId"]) {
      const value = typeof effect[field] === "string" ? effect[field].trim() : ""
      if (value) bindings.set(`${kind}#${occurrence}.${field}`, value)
    }
    for (const field of ["counterIds", "markIds"]) {
      if (!Array.isArray(effect[field])) continue
      effect[field].forEach((rawValue, index) => {
        const value = typeof rawValue === "string" ? rawValue.trim() : ""
        if (value) bindings.set(`${kind}#${occurrence}.${field}[${index}]`, value)
      })
    }
  }
  return bindings
}

function specializationRequireRetainedHookEffectState(previousEffects, currentEffects, label) {
  const current = specializationHookEffectStateBindings(currentEffects)
  for (const [binding, value] of specializationHookEffectStateBindings(previousEffects)) {
    if (current.get(binding) !== value) throw new Error(`${label} must retain battleHook effect resource binding: ${binding}=${value}`)
  }
  const previousCycles = (Array.isArray(previousEffects) ? previousEffects : []).filter((effect) => effect?.kind === "cycle_counter")
  const currentCycles = (Array.isArray(currentEffects) ? currentEffects : []).filter((effect) => effect?.kind === "cycle_counter")
  previousCycles.forEach((previousEffect, index) => {
    if (currentCycles[index] && currentCycles[index].maxStacks < previousEffect.maxStacks) {
      throw new Error(`${label} must not reduce cycle_counter maxStacks: cycle_counter#${index}`)
    }
  })
  const previousExtraTurns = (Array.isArray(previousEffects) ? previousEffects : []).filter((effect) => effect?.kind === "extra_turn")
  const currentExtraTurns = (Array.isArray(currentEffects) ? currentEffects : []).filter((effect) => effect?.kind === "extra_turn")
  previousExtraTurns.forEach((previousEffect, index) => {
    const currentEffect = currentExtraTurns[index]
    if (!currentEffect) return
    if (Number.isFinite(previousEffect.procChanceBase) && currentEffect.procChanceBase < previousEffect.procChanceBase) {
      throw new Error(`${label} must not reduce extra_turn procChanceBase: extra_turn#${index}`)
    }
    if (Number.isFinite(previousEffect.procChanceMax) && currentEffect.procChanceMax < previousEffect.procChanceMax) {
      throw new Error(`${label} must not reduce extra_turn procChanceMax: extra_turn#${index}`)
    }
  })
}

function specializationConditionRelaxes(previousCondition, currentCondition) {
  if (!specializationIsObject(previousCondition) || !specializationIsObject(currentCondition)) return false
  const kind = String(previousCondition.kind || "").trim()
  if (kind !== String(currentCondition.kind || "").trim()) return false
  if (kind === "trigger_limit") {
    const order = ["perBattle", "perWave", "perTurn"]
    const previousField = order.find((field) => Number.isInteger(previousCondition[field]))
    const currentField = order.find((field) => Number.isInteger(currentCondition[field]))
    if (!previousField || !currentField || order.indexOf(currentField) < order.indexOf(previousField)) return false
    return currentCondition[currentField] >= previousCondition[previousField]
  }
  const thresholdDirection = new Map([
    ["self_mp_ratio_gte", "down"],
    ["counter_gte", "down"],
    ["context_hit_count_gte", "down"],
    ["self_hp_ratio_lte", "up"]
  ])
  const direction = thresholdDirection.get(kind)
  if (direction) {
    const previousIdentity = { ...previousCondition }
    const currentIdentity = { ...currentCondition }
    delete previousIdentity.value
    delete currentIdentity.value
    if (JSON.stringify(specializationCanonicalCoreSnapshot(previousIdentity)) !== JSON.stringify(specializationCanonicalCoreSnapshot(currentIdentity))) return false
    return direction === "down" ? currentCondition.value <= previousCondition.value : currentCondition.value >= previousCondition.value
  }
  return JSON.stringify(specializationCanonicalCoreSnapshot(previousCondition)) === JSON.stringify(specializationCanonicalCoreSnapshot(currentCondition))
}

function specializationRequireNonNarrowingConditions(previousConditions, currentConditions, label) {
  const unmatched = [...(Array.isArray(previousConditions) ? previousConditions : [])]
  for (const currentCondition of Array.isArray(currentConditions) ? currentConditions : []) {
    const matchIndex = unmatched.findIndex((previousCondition) => specializationConditionRelaxes(previousCondition, currentCondition))
    if (matchIndex < 0) throw new Error(`${label} must not add or narrow battleHook conditions`)
    unmatched.splice(matchIndex, 1)
  }
}

const SPECIALIZATION_LIMIT_SCOPE_ORDER = ["perBattle", "perWave", "perTurn"]

function specializationRequireNonNarrowingLimits(previousLimits, currentLimits, label) {
  const previous = specializationIsObject(previousLimits) ? previousLimits : undefined
  const current = specializationIsObject(currentLimits) ? currentLimits : undefined
  if (!current) return
  if (!previous) throw new Error(`${label} must not add battleHook limits after an unlimited rank`)
  for (const [currentScope, currentCount] of Object.entries(current)) {
    const currentScopeIndex = SPECIALIZATION_LIMIT_SCOPE_ORDER.indexOf(currentScope)
    const impliedByPrevious = Object.entries(previous).some(([previousScope, previousCount]) => {
      const previousScopeIndex = SPECIALIZATION_LIMIT_SCOPE_ORDER.indexOf(previousScope)
      return previousScopeIndex >= 0 && previousScopeIndex <= currentScopeIndex && previousCount <= currentCount
    })
    if (!impliedByPrevious) throw new Error(`${label} must not narrow battleHook limits: ${currentScope}=${currentCount}`)
  }
}

const SPECIALIZATION_EVENT_FILTER_INCLUDE_ARRAYS = new Set([
  "skillIds", "skillCategories", "damageTypes", "turnKinds", "buffIds", "buffKinds",
  "applicationKinds", "probabilityKinds", "sourceIds", "unitTypes"
])
const SPECIALIZATION_EVENT_FILTER_EXCLUDE_ARRAYS = new Set(["excludeSkillIds", "excludeBuffIds", "excludeSourceIds"])
const SPECIALIZATION_EVENT_FILTER_INCLUDE_BOOLEANS = new Set(["includeBasicAttack", "includePreparation", "includeSuppressedTurns"])
const SPECIALIZATION_EVENT_FILTER_EXCLUDE_BOOLEANS = new Set(["excludeSelf", "firstBuffInstanceOnly"])

function specializationRequireNonNarrowingEventFilter(previousFilter, currentFilter, label) {
  const previous = specializationIsObject(previousFilter) ? previousFilter : {}
  const current = specializationIsObject(currentFilter) ? currentFilter : {}
  for (const key of SPECIALIZATION_EVENT_FILTER_INCLUDE_ARRAYS) {
    if (current[key] === undefined) continue
    if (previous[key] === undefined) throw new Error(`${label}.${key} must not add a narrowing eventFilter after an unfiltered rank`)
    specializationRequireRetainedStrings(previous[key], current[key], `${label}.${key}`)
  }
  for (const key of SPECIALIZATION_EVENT_FILTER_EXCLUDE_ARRAYS) {
    if (current[key] === undefined) continue
    if (previous[key] === undefined) throw new Error(`${label}.${key} must not add exclusions after an unfiltered rank`)
    specializationRequireRetainedStrings(current[key], previous[key], `${label}.${key}`)
  }
  for (const key of SPECIALIZATION_EVENT_FILTER_INCLUDE_BOOLEANS) {
    const previousRestricts = previous[key] === false
    const currentRestricts = current[key] === false
    if (currentRestricts && !previousRestricts) throw new Error(`${label}.${key} must not exclude events accepted by the previous rank`)
  }
  for (const key of SPECIALIZATION_EVENT_FILTER_EXCLUDE_BOOLEANS) {
    const previousRestricts = previous[key] === true
    const currentRestricts = current[key] === true
    if (currentRestricts && !previousRestricts) throw new Error(`${label}.${key} must not exclude events accepted by the previous rank`)
  }
}

function validateSpecializationBattleHookCounterCapacity(hooks, label) {
  const capacityByCounter = new Map()
  const demandByCounter = new Map()
  const rememberMax = (map, rawCounterID, value) => {
    const counterID = typeof rawCounterID === "string" ? rawCounterID.trim() : ""
    if (!counterID || !Number.isFinite(value)) return
    map.set(counterID, Math.max(map.get(counterID) || 0, value))
  }
  for (const hook of Array.isArray(hooks) ? hooks : []) {
    if (!specializationIsObject(hook)) continue
    for (const condition of Array.isArray(hook.conditions) ? hook.conditions : []) {
      if (specializationIsObject(condition) && condition.kind === "counter_gte") rememberMax(demandByCounter, condition.counterId, condition.value)
    }
    for (const effect of Array.isArray(hook.effects) ? hook.effects : []) {
      if (!specializationIsObject(effect)) continue
      if (effect.kind === "cycle_counter") {
        rememberMax(capacityByCounter, effect.counterId, effect.maxStacks)
        if (specializationIsObject(effect.atMax)) rememberMax(demandByCounter, effect.counterId, effect.atMax.consumeStacks)
      }
      if (effect.kind === "consume_counter_for_damage_amp") rememberMax(demandByCounter, effect.counterId, effect.consume)
    }
  }
  for (const [counterID, capacity] of capacityByCounter) {
    const demand = demandByCounter.get(counterID) || 0
    if (capacity < demand) throw new Error(`${label} cycle_counter ${counterID} maxStacks ${capacity} is below required ${demand}`)
  }
}

function specializationRequireRetainedStrings(previousValues, currentValues, label) {
  const current = new Set((Array.isArray(currentValues) ? currentValues : []).map((value) => String(value || "").trim()).filter(Boolean))
  for (const rawValue of Array.isArray(previousValues) ? previousValues : []) {
    const value = String(rawValue || "").trim()
    if (value && !current.has(value)) throw new Error(`${label} must retain prior selector value: ${value}`)
  }
}

function specializationRequireNonNarrowingFilter(previousValues, currentValues, label) {
  const previousIsBounded = Array.isArray(previousValues)
  const currentIsBounded = Array.isArray(currentValues)
  if (!previousIsBounded && currentIsBounded) throw new Error(`${label} must not add a narrowing filter after an unfiltered rank`)
  if (!previousIsBounded || !currentIsBounded) return
  specializationRequireRetainedStrings(previousValues, currentValues, label)
}

function specializationRequireRetainedEffectKinds(previousEffects, currentEffects, label) {
  const currentKinds = new Map()
  for (const effect of Array.isArray(currentEffects) ? currentEffects : []) {
    const kind = String(effect?.kind || "").trim()
    currentKinds.set(kind, (currentKinds.get(kind) || 0) + 1)
  }
  for (const effect of Array.isArray(previousEffects) ? previousEffects : []) {
    const kind = String(effect?.kind || "").trim()
    if ((currentKinds.get(kind) || 0) <= 0) throw new Error(`${label} must retain prior hook effect kind: ${kind || "<missing>"}`)
    currentKinds.set(kind, currentKinds.get(kind) - 1)
  }
}

function specializationValidateAppendEffectUpgrade(previousEffect, currentEffect, label) {
  const kind = String(previousEffect?.kind || "").trim()
  if (kind === "splash_damage") {
    if (currentEffect.targetCount < previousEffect.targetCount || currentEffect.amountRatio < previousEffect.amountRatio) {
      throw new Error(`${label} must not reduce splash targetCount or amountRatio`)
    }
  } else if (kind === "secondary_damage") {
    if (currentEffect.targetPolicy !== previousEffect.targetPolicy || currentEffect.targetPosition !== previousEffect.targetPosition) {
      throw new Error(`${label} must retain secondary_damage target policy`)
    }
    if (currentEffect.amountRatio < previousEffect.amountRatio) throw new Error(`${label} must not reduce secondary_damage amountRatio`)
  } else if (kind === "status") {
    specializationRequireRetainedStrings(previousEffect.statusPayload?.statusIds, currentEffect.statusPayload?.statusIds, `${label}.statusPayload.statusIds`)
    if (Number(currentEffect.statusPayload?.durationBaseTurns) < Number(previousEffect.statusPayload?.durationBaseTurns)
      || Number(currentEffect.statusPayload?.durationPerSkillLevelTurns) < Number(previousEffect.statusPayload?.durationPerSkillLevelTurns)) {
      throw new Error(`${label} must not reduce status duration`)
    }
  } else if (kind === "runtime_damage_amp") {
    if (currentEffect.value < previousEffect.value) throw new Error(`${label} must not reduce runtime_damage_amp value`)
  } else if (kind === "consume_hp") {
    if (currentEffect.maxHpRatio > previousEffect.maxHpRatio || currentEffect.minimumHp < previousEffect.minimumHp) {
      throw new Error(`${label} must not increase consume_hp cost`)
    }
  }
}

export function validateSpecializationGrantSnapshotRetention(previous, current, label = "specialization grant snapshot") {
  const grantID = String(previous?.id || "").trim()
  const previousKind = String(previous?.kind || "").trim().toLowerCase()
  const currentKind = String(current?.kind || "").trim().toLowerCase()
  if (!specializationIsObject(current) || !grantID || !previousKind || currentKind !== previousKind) {
    throw new Error(`${label} must retain prior rank grant id/kind: ${grantID || "<missing>"}/${previousKind || "<missing>"}`)
  }
  if (previousKind === "stat_modifier") {
    if (String(previous.scope || "").trim().toLowerCase() !== String(current.scope || "").trim().toLowerCase()) {
      throw new Error(`${label} must retain stat_modifier scope: ${grantID}`)
    }
    const currentSignatures = new Map()
    for (const modifier of Array.isArray(current.modifiers) ? current.modifiers : []) {
      const signature = `${String(modifier?.attributeKey || "").trim()}\u0000${String(modifier?.operation || "").trim().toLowerCase()}`
      currentSignatures.set(signature, (currentSignatures.get(signature) || 0) + 1)
    }
    for (const modifier of Array.isArray(previous.modifiers) ? previous.modifiers : []) {
      const signature = `${String(modifier?.attributeKey || "").trim()}\u0000${String(modifier?.operation || "").trim().toLowerCase()}`
      if ((currentSignatures.get(signature) || 0) <= 0) throw new Error(`${label} must retain stat_modifier attributeKey/operation signature: ${grantID}`)
      currentSignatures.set(signature, currentSignatures.get(signature) - 1)
    }
    return
  }
  if (previousKind === "battle_hook" || previousKind === "effect_mutation") {
    const field = previousKind === "battle_hook" ? "battleHooks" : "effectMutations"
    const payloadLabel = previousKind === "battle_hook" ? "battleHook" : "effectMutation"
    const currentByID = new Map((Array.isArray(current[field]) ? current[field] : []).map((payload) => [String(payload?.id || "").trim(), payload]))
    for (const payload of Array.isArray(previous[field]) ? previous[field] : []) {
      const payloadID = String(payload?.id || "").trim()
      const currentPayload = currentByID.get(payloadID)
      if (!currentPayload) throw new Error(`${label} must retain ${payloadLabel} stable payload id: ${grantID}/${payloadID || "<missing>"}`)
      if (previousKind === "battle_hook") {
        const previousTrigger = String(payload.trigger || "").trim()
        const currentTrigger = String(currentPayload.trigger || "").trim()
        if (previousTrigger !== currentTrigger && !(previousTrigger === "onBattleStart" && currentTrigger === "onWaveStart")) {
          throw new Error(`${label} must retain battleHook trigger (only onBattleStart->onWaveStart may widen): ${grantID}/${payloadID}/${previousTrigger}->${currentTrigger}`)
        }
        specializationRequireRetainedEffectKinds(payload.effects, currentPayload.effects, `${label} ${grantID}/${payloadID}`)
        specializationRequireRetainedHookEffectState(payload.effects, currentPayload.effects, `${label} ${grantID}/${payloadID}`)
        specializationRequireNonNarrowingConditions(payload.conditions, currentPayload.conditions, `${label} ${grantID}/${payloadID}`)
        specializationRequireNonNarrowingLimits(payload.limits, currentPayload.limits, `${label} ${grantID}/${payloadID}`)
        specializationRequireNonNarrowingEventFilter(payload.eventFilter, currentPayload.eventFilter, `${label} ${grantID}/${payloadID}.eventFilter`)
        continue
      }
      const previousScope = String(payload.scope || "").trim()
      const currentScope = String(currentPayload.scope || "").trim()
      const allowedScopes = previousScope === "both" ? new Set(["both"])
        : previousScope === "skill" ? new Set(["skill", "both"])
          : new Set(["basic_attack", "both"])
      if (!allowedScopes.has(currentScope)) throw new Error(`${label} must retain or widen effectMutation scope: ${grantID}/${payloadID}/${previousScope}->${currentScope}`)
      const previousSelector = specializationIsObject(payload.effectSelector) ? payload.effectSelector : {}
      const currentSelector = specializationIsObject(currentPayload.effectSelector) ? currentPayload.effectSelector : {}
      if (String(previousSelector.mode || "").trim() !== String(currentSelector.mode || "").trim()) {
        throw new Error(`${label} must retain effectMutation selector mode: ${grantID}/${payloadID}`)
      }
      for (const selectorField of ["skillIds", "skillCategories", "weaponClasses", "damageTypes"]) {
        specializationRequireNonNarrowingFilter(payload[selectorField], currentPayload[selectorField], `${label} ${grantID}/${payloadID}.${selectorField}`)
      }
      for (const selectorField of ["effectIds", "kinds"]) {
        specializationRequireRetainedStrings(previousSelector[selectorField], currentSelector[selectorField], `${label} ${grantID}/${payloadID}.effectSelector.${selectorField}`)
      }
      const currentAppendEffects = new Map((Array.isArray(currentPayload?.ops?.appendEffects) ? currentPayload.ops.appendEffects : [])
        .map((effect) => [String(effect?.id || "").trim(), effect]))
      for (const effect of Array.isArray(payload?.ops?.appendEffects) ? payload.ops.appendEffects : []) {
        const effectID = String(effect?.id || "").trim()
        const currentEffect = currentAppendEffects.get(effectID)
        if (!currentEffect) throw new Error(`${label} must retain effectMutation appended stable effect id: ${grantID}/${payloadID}/${effectID || "<missing>"}`)
        if (String(effect.kind || "").trim() !== String(currentEffect.kind || "").trim()) {
          throw new Error(`${label} must retain effectMutation appended effect kind: ${grantID}/${payloadID}/${effectID}`)
        }
        specializationValidateAppendEffectUpgrade(effect, currentEffect, `${label} ${grantID}/${payloadID}/${effectID}`)
      }
    }
    return
  }
  if (previousKind === "equipment_slot_capacity") {
    const currentFamilies = new Set(Object.keys(specializationIsObject(current.deltas) ? current.deltas : {}).map((family) => family.trim()))
    for (const family of Object.keys(specializationIsObject(previous.deltas) ? previous.deltas : {})) {
      if (!currentFamilies.has(family.trim())) throw new Error(`${label} must retain equipment slot family: ${grantID}/${family}`)
    }
    return
  }
  throw new Error(`${label} cannot retain unknown grant kind: ${previousKind}`)
}

function validateSpecializationCapacityEnvelope(nodes, treeID) {
  const independent = []
  const exclusiveGroups = new Map()
  for (const node of nodes) {
    const deltas = {}
    for (const grant of node.levels.flatMap((level) => level.grants)) {
      if (grant.kind !== "equipment_slot_capacity") continue
      for (const [family, delta] of Object.entries(grant.deltas)) deltas[family] = (deltas[family] || 0) + delta
    }
    if (Object.keys(deltas).length === 0) continue
    if (node.exclusiveGroupId) {
      const group = exclusiveGroups.get(node.exclusiveGroupId) || []
      group.push(deltas)
      exclusiveGroups.set(node.exclusiveGroupId, group)
    } else {
      independent.push(deltas)
    }
  }
  for (const [family, rule] of Object.entries(SPECIALIZATION_SLOT_FAMILIES)) {
    let minDelta = 0
    let maxDelta = 0
    for (const deltas of independent) {
      minDelta += Math.min(0, deltas[family] || 0)
      maxDelta += Math.max(0, deltas[family] || 0)
    }
    for (const group of exclusiveGroups.values()) {
      const values = group.map((deltas) => deltas[family] || 0)
      minDelta += Math.min(0, ...values)
      maxDelta += Math.max(0, ...values)
    }
    const minimum = rule.base + minDelta
    const maximum = rule.base + maxDelta
    if (minimum < rule.min || maximum > rule.max) {
      throw new Error(`specialization_trees capacity envelope outside bounds: ${treeID}.${family}=[${minimum},${maximum}] allowed=[${rule.min},${rule.max}]`)
    }
  }
}

function specializationFormulaAttributeIDs(value, out = new Set()) {
  if (Array.isArray(value)) {
    for (const entry of value) specializationFormulaAttributeIDs(entry, out)
    return out
  }
  if (!specializationIsObject(value)) return out
  if (typeof value.attributeKey === "string" && Number.isFinite(value.coefficient) && value.coefficient !== 0) {
    out.add(value.attributeKey.trim())
  }
  if (value.kind === "retaliate") {
    if (Number.isFinite(value.strengthRatio) && value.strengthRatio !== 0) out.add("strength")
    if (Number.isFinite(value.staminaRatio) && value.staminaRatio !== 0) out.add("stamina")
  }
  for (const entry of Object.values(value)) specializationFormulaAttributeIDs(entry, out)
  return out
}

function specializationStableEffectIDs(trees, selectableEffects) {
  const effectIDs = new Set()
  const ownerByIdentity = new Map()
  const register = (rawID, label, owner = label) => {
    const effectID = String(rawID || "").trim()
    if (!effectID) return
    if (!STABLE_BATTLE_EFFECT_ID_PATTERN.test(effectID)) {
      throw new Error(`${label}.id invalid: stable effect ids must use lowercase letters, digits, dot, underscore, colon, or hyphen`)
    }
    const identity = effectID.toLowerCase()
    const previousOwner = ownerByIdentity.get(identity)
    if (previousOwner && previousOwner !== owner) throw new Error(`battle effect stable id duplicate: ${effectID} (${previousOwner} / ${label})`)
    ownerByIdentity.set(identity, owner)
    effectIDs.add(effectID)
  }

  for (let effectIndex = 0; effectIndex < selectableEffects.length; effectIndex += 1) {
    const effect = selectableEffects[effectIndex]
    register(effect?.id ?? effect?.effectId, `catalog effect[${effectIndex}]`)
  }
  for (let treeIndex = 0; treeIndex < trees.length; treeIndex += 1) {
    const tree = trees[treeIndex]
    const treeID = String(tree?.id || `#${treeIndex + 1}`).trim()
    for (let nodeIndex = 0; nodeIndex < (Array.isArray(tree?.nodes) ? tree.nodes.length : 0); nodeIndex += 1) {
      const node = tree.nodes[nodeIndex]
      const nodeID = String(node?.id || `#${nodeIndex + 1}`).trim()
      for (let levelIndex = 0; levelIndex < (Array.isArray(node?.levels) ? node.levels.length : 0); levelIndex += 1) {
        const level = node.levels[levelIndex]
        for (let grantIndex = 0; grantIndex < (Array.isArray(level?.grants) ? level.grants.length : 0); grantIndex += 1) {
          const grant = level.grants[grantIndex]
          for (let mutationIndex = 0; mutationIndex < (Array.isArray(grant?.effectMutations) ? grant.effectMutations.length : 0); mutationIndex += 1) {
            const mutation = grant.effectMutations[mutationIndex]
            const appendEffects = Array.isArray(mutation?.ops?.appendEffects) ? mutation.ops.appendEffects : []
            for (let effectIndex = 0; effectIndex < appendEffects.length; effectIndex += 1) {
              const effectID = String(appendEffects[effectIndex]?.id || "").trim()
              const owner = `specialization_trees.${treeID}.${nodeID}.${String(grant?.id || "").trim()}.${String(mutation?.id || "").trim()}.${effectID.toLowerCase()}`
              register(effectID, `specialization_trees.${treeID}.${nodeID}.levels[${levelIndex}].grants[${grantIndex}].effectMutations[${mutationIndex}].ops.appendEffects[${effectIndex}]`, owner)
            }
          }
        }
      }
    }
  }
  return effectIDs
}

function validateSpecializationMutationKindCompatibility(descriptor, selectedKinds) {
  const kinds = [...new Set(selectedKinds.map((kind) => String(kind || "").trim().toLowerCase()).filter(Boolean))]
  if (kinds.length === 0) return
  const { mutation, label } = descriptor
  const ops = mutation.ops || {}
  const requireKinds = (operation, allowedKinds) => {
    for (const kind of kinds) {
      if (!allowedKinds.has(kind)) throw new Error(`${label}.ops ${operation} cannot target effect kind: ${kind}`)
    }
  }
  if (["amountBaseAddDelta", "amountTermsAppend", "amountMultiplier"].some((key) => ops[key] !== undefined)) {
    requireKinds("amount", new Set(["damage", "heal"]))
  }
  if (["hitCountDelta", "attackStatPolicy", "damageTypePolicy"].some((key) => ops[key] !== undefined)) {
    requireKinds("damage", new Set(["damage"]))
  }
  if (["statusIdsAppend", "statusIdsRemove"].some((key) => ops[key] !== undefined)) {
    requireKinds("status", new Set(["status"]))
  }
  for (const effect of Array.isArray(ops.appendEffects) ? ops.appendEffects : []) {
    if (effect?.kind === "runtime_damage_amp" || effect?.kind === "splash_damage" || effect?.kind === "secondary_damage") {
      requireKinds("appendEffects", new Set(["damage"]))
    }
  }
}

function specializationGuaranteedNodeDependencies(tree) {
  const nodesByID = new Map((tree.nodes || []).map((node) => [node.id, node]))
  const memo = new Map()
  const visiting = new Set()
  const mergeMaximum = (target, source) => {
    for (const [nodeID, minRank] of source) target.set(nodeID, Math.max(target.get(nodeID) || 0, minRank))
  }
  const intersectMinimum = (maps) => {
    if (maps.length === 0) return new Map()
    const out = new Map()
    for (const nodeID of maps[0].keys()) {
      if (maps.slice(1).every((map) => map.has(nodeID))) out.set(nodeID, Math.min(...maps.map((map) => map.get(nodeID))))
    }
    return out
  }
  const visit = (nodeID) => {
    if (memo.has(nodeID)) return new Map(memo.get(nodeID))
    if (visiting.has(nodeID)) return new Map()
    visiting.add(nodeID)
    const out = new Map()
    const node = nodesByID.get(nodeID)
    const dependencyMap = (requirement) => {
      if (requirement?.kind !== "node_rank") return new Map()
      const dependencies = visit(requirement.nodeId)
      dependencies.set(requirement.nodeId, Math.max(dependencies.get(requirement.nodeId) || 0, requirement.minRank || 1))
      return dependencies
    }
    for (const requirement of node?.requirements?.all || []) {
      mergeMaximum(out, dependencyMap(requirement))
    }
    for (const group of node?.requirements?.anyOf || []) {
      mergeMaximum(out, intersectMinimum((group || []).map(dependencyMap)))
    }
    visiting.delete(nodeID)
    memo.set(nodeID, out)
    return new Map(out)
  }
  for (const nodeID of nodesByID.keys()) visit(nodeID)
  return memo
}

export function validateSpecializationMutationSemantics(trees, refs) {
  const descriptors = []
  const dependenciesByTree = new Map()
  for (const tree of trees) {
    dependenciesByTree.set(tree.id, specializationGuaranteedNodeDependencies(tree))
    for (const node of tree.nodes || []) {
      for (let levelIndex = 0; levelIndex < (node?.levels || []).length; levelIndex += 1) {
        const level = node.levels[levelIndex]
        for (const grant of level.grants || []) {
          if (grant?.kind !== "effect_mutation") continue
          for (let mutationIndex = 0; mutationIndex < (grant.effectMutations || []).length; mutationIndex += 1) {
            const mutation = grant.effectMutations[mutationIndex]
            descriptors.push({
              tree,
              node,
              rank: level.level ?? levelIndex + 1,
              grant,
              mutation,
              mutationIndex,
              priority: mutation.priority ?? 0,
              runtimeID: `${tree.id}:${node.id}:${grant.id}:${mutation.id}`,
              label: `specialization_trees.${tree.id}.${node.id}.levels[${levelIndex}].${grant.id}.effectMutations[${mutationIndex}]`
            })
          }
        }
      }
    }
  }

  const producerByEffectID = new Map()
  for (const descriptor of descriptors) {
    for (const effect of descriptor.mutation?.ops?.appendEffects || []) {
      const effectID = String(effect.id || "").trim().toLowerCase()
      const producers = producerByEffectID.get(effectID) || []
      producers.push({ descriptor, kind: String(effect.kind || "").trim() })
      producerByEffectID.set(effectID, producers)
    }
  }
  const executesBefore = (producer, consumer) => producer.priority > consumer.priority
    || (producer.priority === consumer.priority && producer.runtimeID < consumer.runtimeID)

  for (const descriptor of descriptors) {
    const { mutation, label } = descriptor
    const selector = mutation.effectSelector || {}
    let selectedKinds = selector.mode === "kind" ? [...(selector.kinds || [])] : []
    if (selector.mode === "effect_id") {
      selectedKinds = []
      const hasSkillScalarOps = mutation.ops?.cooldownDelta !== undefined
        || mutation.ops?.cooldownFloor !== undefined
        || mutation.ops?.mpCostDelta !== undefined
        || mutation.ops?.mpCostMultiplier !== undefined
      for (const rawEffectID of selector.effectIds || []) {
        const effectID = String(rawEffectID || "").trim()
        const normalizedEffectID = effectID.toLowerCase()
        const generatedSnapshots = producerByEffectID.get(normalizedEffectID) || []
        if (generatedSnapshots.length > 0) {
          if (hasSkillScalarOps) throw new Error(`${label} cannot use generated effect ${effectID} with skill scalar operations`)
          if (selector.includeGenerated !== true) throw new Error(`${label} selects generated effect ${effectID} without includeGenerated`)
          const sameTreeSnapshots = generatedSnapshots.filter((entry) => entry.descriptor.tree.id === descriptor.tree.id)
          if (sameTreeSnapshots.length === 0) throw new Error(`${label} selects inactive generated effect from another tree: ${effectID}`)
          const producerNodeIDs = new Set(sameTreeSnapshots.map((entry) => entry.descriptor.node.id))
          if (producerNodeIDs.size !== 1) throw new Error(`${label} generated effect ${effectID} has ambiguous producer identities`)
          const producerNodeID = sameTreeSnapshots[0].descriptor.node.id
          let reachableSnapshots
          if (producerNodeID === descriptor.node.id) {
            reachableSnapshots = sameTreeSnapshots.filter((entry) => entry.descriptor.rank === descriptor.rank)
            if (reachableSnapshots.length === 0) throw new Error(`${label} selects generated effect ${effectID} that is inactive at node rank ${descriptor.rank}`)
          } else {
            const guaranteedRank = dependenciesByTree.get(descriptor.tree.id)?.get(descriptor.node.id)?.get(producerNodeID) || 0
            const firstProducerRank = Math.min(...sameTreeSnapshots.map((entry) => entry.descriptor.rank))
            if (guaranteedRank < firstProducerRank) {
              throw new Error(`${label} selects generated effect ${effectID} without requiring producer node ${producerNodeID} at rank ${firstProducerRank}`)
            }
            const producerMaxRank = sameTreeSnapshots[0].descriptor.node.levels.length
            reachableSnapshots = sameTreeSnapshots.filter((entry) => entry.descriptor.rank >= guaranteedRank)
            for (let rank = guaranteedRank; rank <= producerMaxRank; rank += 1) {
              if (!reachableSnapshots.some((entry) => entry.descriptor.rank === rank)) {
                throw new Error(`${label} selects generated effect ${effectID} that is inactive at guaranteed producer rank ${rank}`)
              }
            }
          }
          for (const generated of reachableSnapshots) {
            const producer = generated.descriptor
            if (producer === descriptor) throw new Error(`${label} cannot select its own generated effect: ${effectID}`)
            if (!executesBefore(producer, descriptor)) throw new Error(`${label} selects generated effect ${effectID} before its producer executes`)
            if (generated.kind === "splash_damage" || generated.kind === "secondary_damage") selectedKinds.push("damage")
            else if (generated.kind === "status") selectedKinds.push("status")
            else throw new Error(`${label} selects generated effect ${effectID} whose kind ${generated.kind} is not mutable`)
          }
          continue
        }
        const staticKind = refs.effectKindsByID.get(normalizedEffectID)
        if (!staticKind) throw new Error(`${label} selects unknown or inactive effect id: ${effectID}`)
        if (hasSkillScalarOps && refs.equipmentEffectIDs.has(normalizedEffectID)) {
          throw new Error(`${label} cannot use equipment basic-attack effect ${effectID} with skill scalar operations`)
        }
        selectedKinds.push(staticKind)
      }
    }
    validateSpecializationMutationKindCompatibility(descriptor, selectedKinds)
  }
}

export function validateSpecializationTrees(trees, attributes, items, skills = [], buffs = [], equipmentTemplates = []) {
  if (!Array.isArray(trees) || trees.length === 0) throw new Error("specialization_trees must be a non-empty array")
  const attributeIDs = new Set((attributes || []).map((row) => String(row?.id || "").trim()).filter(Boolean))
  const itemIDs = new Set((items || []).map((row) => String(row?.id || "").trim()).filter(Boolean))
  const skillIDs = new Set((skills || []).map((row) => String(row?.id || "").trim()).filter(Boolean))
  const skillCategories = new Set((skills || []).map((row) => String(row?.category || "").trim()).filter(Boolean))
  const buffIDs = new Set((buffs || []).map((row) => String(row?.id || "").trim()).filter(Boolean))
  const buffKinds = new Set((buffs || []).map((row) => String(row?.kind || "").trim()).filter(Boolean))
  const skillEffects = (skills || []).flatMap((row) => Array.isArray(row?.effects) ? row.effects : [])
  const basicAttackEffects = (equipmentTemplates || []).flatMap((row) => Array.isArray(row?.basicAttackEffects) ? row.basicAttackEffects : [])
  const selectableEffects = [...skillEffects, ...basicAttackEffects]
  const skillEffectKinds = new Set(selectableEffects.map((effect) => String(effect?.kind || "").trim()).filter(Boolean))
  const effectIDs = specializationStableEffectIDs(trees, selectableEffects)
  const effectKindsByID = new Map(selectableEffects.map((effect) => [String(effect?.id ?? effect?.effectId ?? "").trim().toLowerCase(), String(effect?.kind || "").trim()]).filter(([id, kind]) => id && kind))
  const equipmentEffectIDs = new Set(basicAttackEffects.map((effect) => String(effect?.id ?? effect?.effectId ?? "").trim().toLowerCase()).filter(Boolean))
  const weaponClasses = new Set((equipmentTemplates || []).flatMap((row) => Array.isArray(row?.weaponClass) ? row.weaponClass : []).map((id) => String(id || "").trim()).filter(Boolean))
  const refs = { attributeIDs, itemIDs, skillIDs, skillCategories, weaponClasses, buffIDs, buffKinds, skillEffectKinds, effectIDs, effectKindsByID, equipmentEffectIDs }
  const treeIDs = new Set()
  for (const tree of trees) {
    if (!specializationIsObject(tree)) throw new Error("specialization_trees row must be an object")
    const treeID = specializationRequiredString(tree.id, "specialization_trees.id")
    if (treeIDs.has(treeID)) throw new Error(`duplicate specialization_trees.id: ${treeID}`)
    treeIDs.add(treeID)
    specializationRejectUnknownKeys(tree, ["id", "schemaVersion", "rulesetVersion", "balanceRevision", "ownerType", "nameKey", "descKey", "initialPoints", "maxPoints", "maxActiveRootNodes", "unlock", "pointBands", "reset", "schools", "nodes", "keywords", "ui", "i18n"], `specialization_trees.${treeID}`)
    if (tree.schemaVersion !== 2) throw new Error(`specialization_trees.schemaVersion unsupported: ${treeID}; expected 2`)
    if (tree.rulesetVersion !== "specialization-v6") throw new Error(`specialization_trees.rulesetVersion unsupported: ${treeID}; expected specialization-v6`)
    if (!Number.isInteger(tree.balanceRevision) || tree.balanceRevision <= 0) throw new Error(`specialization_trees.balanceRevision invalid: ${treeID}`)
    if (tree.ownerType !== "player") throw new Error(`specialization_trees.ownerType invalid: ${treeID}`)
    if (tree.nameKey !== `specialization.tree.${treeID}.name`) throw new Error(`specialization_trees.nameKey invalid: ${treeID}`)
    if (tree.descKey !== `specialization.tree.${treeID}.desc`) throw new Error(`specialization_trees.descKey invalid: ${treeID}`)
    if (tree.initialPoints !== 2) throw new Error(`specialization_trees.initialPoints invalid: ${treeID}; level 10 must grant exactly 2 points`)
    if (tree.maxPoints !== 65 || tree.maxPoints <= tree.initialPoints) throw new Error(`specialization_trees.maxPoints invalid: ${treeID}; battle level 500 must award exactly 65 points`)
    if (tree.maxActiveRootNodes !== 3) throw new Error(`specialization_trees.maxActiveRootNodes invalid: ${treeID}; exactly 3 root routes may be active`)
    if (!specializationIsObject(tree.unlock) || tree.unlock.battleLevel !== 10) {
      throw new Error(`specialization_trees.unlock.battleLevel invalid: ${treeID}; specializations must unlock at battle level 10`)
    }
    if (!Array.isArray(tree.pointBands) || tree.pointBands.length === 0) throw new Error(`specialization_trees.pointBands invalid: ${treeID}`)
    let previousToLevel = tree.unlock.battleLevel
    let awardedPoints = tree.initialPoints
    const pointBandIDs = new Set()
    for (const band of tree.pointBands) {
      if (!specializationIsObject(band)) throw new Error(`specialization_trees.pointBands entry invalid: ${treeID}`)
      const bandID = specializationRequiredString(band.id, `specialization_trees.pointBands.id: ${treeID}`)
      if (pointBandIDs.has(bandID)) throw new Error(`specialization_trees.pointBands.id duplicate: ${treeID}.${bandID}`)
      pointBandIDs.add(bandID)
      if (!Number.isInteger(band.fromLevel) || !Number.isInteger(band.toLevel) || band.fromLevel <= previousToLevel || band.toLevel < band.fromLevel || band.toLevel > SPECIALIZATION_BATTLE_LEVEL_CAP) {
        throw new Error(`specialization_trees.pointBands level range invalid: ${treeID}.${bandID}`)
      }
      if (!Number.isInteger(band.everyLevels) || band.everyLevels <= 0 || (band.toLevel - band.fromLevel) % band.everyLevels !== 0) {
        throw new Error(`specialization_trees.pointBands.everyLevels invalid: ${treeID}.${bandID}`)
      }
      if (!Number.isInteger(band.pointsPerGrant) || band.pointsPerGrant <= 0) throw new Error(`specialization_trees.pointBands.pointsPerGrant invalid: ${treeID}.${bandID}`)
      awardedPoints += (Math.floor((band.toLevel - band.fromLevel) / band.everyLevels) + 1) * band.pointsPerGrant
      previousToLevel = band.toLevel
    }
    if (awardedPoints !== tree.maxPoints) throw new Error(`specialization_trees.maxPoints does not match pointBands: ${treeID} got=${awardedPoints} want=${tree.maxPoints}`)
    const pointsAtLevel = (battleLevel) => tree.initialPoints + tree.pointBands.reduce((total, band) => {
      if (battleLevel < band.fromLevel) return total
      const upper = Math.min(battleLevel, band.toLevel)
      return total + (Math.floor((upper - band.fromLevel) / band.everyLevels) + 1) * band.pointsPerGrant
    }, 0)
    for (const [battleLevel, expectedPoints] of [[200, 40], [300, 60], [500, 65]]) {
      const actualPoints = pointsAtLevel(battleLevel)
      if (actualPoints !== expectedPoints) throw new Error(`specialization_trees point checkpoint invalid: ${treeID} level=${battleLevel} got=${actualPoints} want=${expectedPoints}`)
    }
    if (!specializationIsObject(tree.reset)) throw new Error(`specialization_trees.reset invalid: ${treeID}`)
    const resetItemID = specializationRequiredString(tree.reset.itemId, `specialization_trees.reset.itemId: ${treeID}`)
    if (!itemIDs.has(resetItemID)) throw new Error(`specialization_trees.reset.itemId not found: ${treeID}.${resetItemID}`)
    if (!Number.isInteger(tree.reset.count) || tree.reset.count <= 0) throw new Error(`specialization_trees.reset.count invalid: ${treeID}`)

    if (!Array.isArray(tree.schools) || tree.schools.length !== SPECIALIZATION_SCHOOL_IDS.length) throw new Error(`specialization_trees.schools must contain five schools: ${treeID}`)
    const schoolIDs = new Set()
    const schoolColumns = new Set()
    for (const school of tree.schools) {
      if (!specializationIsObject(school)) throw new Error(`specialization_trees.schools entry invalid: ${treeID}`)
      const schoolID = specializationRequiredString(school.id, `specialization_trees.schools.id: ${treeID}`)
      if (!SPECIALIZATION_SCHOOL_IDS.includes(schoolID) || schoolIDs.has(schoolID)) throw new Error(`specialization_trees.schools.id invalid or duplicate: ${treeID}.${schoolID}`)
      schoolIDs.add(schoolID)
      if (school.nameKey !== `specialization.school.${schoolID}.name`) throw new Error(`specialization_trees.schools.nameKey invalid: ${treeID}.${schoolID}`)
      if (school.descKey !== `specialization.school.${schoolID}.desc`) throw new Error(`specialization_trees.schools.descKey invalid: ${treeID}.${schoolID}`)
      if (school.attributeKey !== schoolID || !attributeIDs.has(school.attributeKey)) throw new Error(`specialization_trees.schools.attributeKey invalid: ${treeID}.${schoolID}`)
      if (!specializationIsObject(school.ui) || !Number.isInteger(school.ui.column) || school.ui.column < 0) throw new Error(`specialization_trees.schools.ui.column invalid: ${treeID}.${schoolID}`)
      if (schoolColumns.has(school.ui.column)) throw new Error(`specialization_trees.schools.ui.column duplicate: ${treeID}.${school.ui.column}`)
      schoolColumns.add(school.ui.column)
      if (typeof school.ui.color !== "string" || !/^#[0-9a-f]{6}$/i.test(school.ui.color)) throw new Error(`specialization_trees.schools.ui.color invalid: ${treeID}.${schoolID}`)
    }
    if (SPECIALIZATION_SCHOOL_IDS.some((schoolID) => !schoolIDs.has(schoolID))) throw new Error(`specialization_trees.schools missing required school: ${treeID}`)
    if (!specializationIsObject(tree.ui) || tree.ui.layout !== "five_spoke" || !Array.isArray(tree.ui.schoolOrder) || tree.ui.schoolOrder.join("|") !== SPECIALIZATION_SCHOOL_IDS.join("|")) {
      throw new Error(`specialization_trees.ui invalid: ${treeID}`)
    }

    if (!Array.isArray(tree.nodes) || tree.nodes.length !== 70) throw new Error(`specialization_trees.nodes must contain 70 nodes: ${treeID}`)
    const nodeIDs = new Set()
    const nodeMaxRanks = new Map()
    const uiPositions = new Set()
    const radialPositions = new Set()
    for (const node of tree.nodes) {
      if (!specializationIsObject(node)) throw new Error(`specialization_trees.nodes entry invalid: ${treeID}`)
      const nodeID = specializationRequiredString(node.id, `specialization_trees.nodes.id: ${treeID}`)
      if (nodeIDs.has(nodeID)) throw new Error(`specialization_trees.nodes.id duplicate: ${treeID}.${nodeID}`)
      nodeIDs.add(nodeID)
      specializationRejectUnknownKeys(node, ["id", "kind", "schoolIds", "nameKey", "descKey", "keywordIds", "requirements", "exclusiveGroupId", "levels", "ui"], `specialization_trees.nodes.${treeID}.${nodeID}`)
      if (!SPECIALIZATION_NODE_COSTS[node.kind]) throw new Error(`specialization_trees.nodes.kind invalid: ${treeID}.${nodeID}`)
      if (!Array.isArray(node.schoolIds) || node.schoolIds.length !== (node.kind === "confluence" ? 2 : 1)) throw new Error(`specialization_trees.nodes.schoolIds invalid: ${treeID}.${nodeID}`)
      if (new Set(node.schoolIds).size !== node.schoolIds.length || node.schoolIds.some((schoolID) => !schoolIDs.has(schoolID))) throw new Error(`specialization_trees.nodes.schoolIds unknown or duplicate: ${treeID}.${nodeID}`)
      if (node.nameKey !== `specialization.node.${nodeID}.name`) throw new Error(`specialization_trees.nodes.nameKey invalid: ${treeID}.${nodeID}`)
      if (node.descKey !== `specialization.node.${nodeID}.desc`) throw new Error(`specialization_trees.nodes.descKey invalid: ${treeID}.${nodeID}`)
      if (!Array.isArray(node.keywordIds)) throw new Error(`specialization_trees.nodes.keywordIds must be an array: ${treeID}.${nodeID}`)
      if (new Set(node.keywordIds).size !== node.keywordIds.length || node.keywordIds.some((keywordID) => typeof keywordID !== "string" || keywordID.trim() === "")) {
        throw new Error(`specialization_trees.nodes.keywordIds invalid or duplicate: ${treeID}.${nodeID}`)
      }
      if (!specializationIsObject(node.requirements) || !Array.isArray(node.requirements.all) || !Array.isArray(node.requirements.anyOf)) throw new Error(`specialization_trees.nodes.requirements invalid: ${treeID}.${nodeID}`)
      const topologyCosts = SPECIALIZATION_TOPOLOGY_COSTS[nodeID]
      const expectedRankCount = topologyCosts?.length ?? (node.kind === "root" ? 6 : node.kind === "technique" ? 4 : 3)
      if (!Array.isArray(node.levels) || node.levels.length !== expectedRankCount) throw new Error(`specialization_trees.nodes.levels rank count invalid: ${treeID}.${nodeID}; expected ${expectedRankCount}`)
      nodeMaxRanks.set(nodeID, node.levels.length)
      const expectedCosts = topologyCosts ?? (node.kind === "root" ? [2, 1, 1, 1, 1, 1]
          : node.kind === "technique" ? [2, 1, 1, 1]
            : node.kind === "terminal" ? [4, 2, 2]
              : [6, 2, 2])
      for (let levelIndex = 0; levelIndex < node.levels.length; levelIndex += 1) {
        const level = node.levels[levelIndex]
        if (!specializationIsObject(level) || level.level !== levelIndex + 1) throw new Error(`specialization_trees.nodes.levels must be contiguous from level 1: ${treeID}.${nodeID}`)
        if (!Number.isInteger(level.cost) || level.cost < 1 || level.cost > 24) throw new Error(`specialization_trees.nodes.levels.cost must be 1-24: ${treeID}.${nodeID}.levels[${levelIndex}]`)
        if (level.cost !== expectedCosts[levelIndex]) throw new Error(`specialization_trees.nodes.levels.cost invalid: ${treeID}.${nodeID}.levels[${levelIndex}] expected=${expectedCosts[levelIndex]}`)
        if (!Array.isArray(level.grants) || level.grants.length === 0) throw new Error(`specialization_trees.nodes.levels.grants invalid: ${treeID}.${nodeID}.levels[${levelIndex}]`)
      }
      for (let levelIndex = 1; levelIndex < node.levels.length; levelIndex += 1) {
        const previousCore = node.levels[levelIndex - 1].grants.filter((grant) => grant.kind !== "stat_modifier")
        const currentCore = node.levels[levelIndex].grants.filter((grant) => grant.kind !== "stat_modifier")
        if (JSON.stringify(specializationCanonicalCoreSnapshot(previousCore, "grants")) === JSON.stringify(specializationCanonicalCoreSnapshot(currentCore, "grants"))) {
          throw new Error(`specialization_trees.nodes.levels must strictly enhance a non-stat core mechanic: ${treeID}.${nodeID}.levels[${levelIndex}]`)
        }
      }
      if (!specializationIsObject(node.ui) || !Number.isInteger(node.ui.column) || !Number.isInteger(node.ui.row) || node.ui.column < 0 || node.ui.row < 0 || !Number.isFinite(node.ui.x) || !Number.isFinite(node.ui.y) || Math.abs(node.ui.x) > 1 || Math.abs(node.ui.y) > 1) throw new Error(`specialization_trees.nodes.ui invalid: ${treeID}.${nodeID}`)
      const positionKey = `${node.ui.column}:${node.ui.row}`
      if (uiPositions.has(positionKey)) throw new Error(`specialization_trees.nodes.ui position duplicate: ${treeID}.${positionKey}`)
      uiPositions.add(positionKey)
      const radialPositionKey = `${node.ui.x}:${node.ui.y}`
      if (radialPositions.has(radialPositionKey)) throw new Error(`specialization_trees.nodes.ui radial position duplicate: ${treeID}.${radialPositionKey}`)
      radialPositions.add(radialPositionKey)
      if (typeof node.ui.accent !== "string" || !/^#[0-9a-f]{6}$/i.test(node.ui.accent)) throw new Error(`specialization_trees.nodes.ui.accent invalid: ${treeID}.${nodeID}`)
    }

    if (!Array.isArray(tree.keywords) || tree.keywords.length === 0) throw new Error(`specialization_trees.keywords must be a non-empty array: ${treeID}`)
    const keywordIDs = new Set()
    const keywordsByID = new Map()
    const nodeByID = new Map(tree.nodes.map((node) => [node.id, node]))
    const runtimeTokensByNode = new Map(tree.nodes.map((node) => [node.id, specializationRuntimeTokens(node.levels)]))
    for (let keywordIndex = 0; keywordIndex < tree.keywords.length; keywordIndex += 1) {
      const keyword = tree.keywords[keywordIndex]
      const keywordLabel = `specialization_trees.keywords.${treeID}[${keywordIndex}]`
      if (!specializationIsObject(keyword)) throw new Error(`${keywordLabel} must be an object`)
      specializationRejectUnknownKeys(keyword, ["id", "mechanicKind", "runtimeIds", "sourceNodeIds", "nameKey", "summaryKey", "mechanicDetailsKey", "sourceHintKey"], keywordLabel)
      const keywordID = specializationRequiredString(keyword.id, `${keywordLabel}.id`)
      if (keywordIDs.has(keywordID)) throw new Error(`${keywordLabel}.id duplicate: ${keywordID}`)
      keywordIDs.add(keywordID)
      keywordsByID.set(keywordID, keyword)
      const mechanicKind = specializationRequiredString(keyword.mechanicKind, `${keywordLabel}.mechanicKind`)
      if (!SPECIALIZATION_KEYWORD_MECHANIC_KINDS.has(mechanicKind)) throw new Error(`${keywordLabel}.mechanicKind invalid: ${mechanicKind}`)
      const runtimeIDs = specializationValidateStringArray(keyword.runtimeIds, `${keywordLabel}.runtimeIds`)
      const sourceNodeIDs = specializationValidateStringArray(keyword.sourceNodeIds, `${keywordLabel}.sourceNodeIds`, { knownValues: nodeIDs })
      if (keyword.nameKey !== `specialization.keyword.${keywordID}.name`
        || keyword.summaryKey !== `specialization.keyword.${keywordID}.summary`
        || keyword.mechanicDetailsKey !== `specialization.keyword.${keywordID}.mechanic_details`
        || keyword.sourceHintKey !== `specialization.keyword.${keywordID}.source_hint`) {
        throw new Error(`${keywordLabel} localization keys must be derived from the stable keyword id`)
      }
      if (sourceNodeIDs.some((nodeID) => !runtimeIDs.some((runtimeID) => runtimeTokensByNode.get(nodeID).has(runtimeID)))) {
        throw new Error(`${keywordLabel}.sourceNodeIds must contain the declared producer or consumer runtime ids`)
      }
      if (runtimeIDs.some((runtimeID) => !sourceNodeIDs.some((nodeID) => runtimeTokensByNode.get(nodeID).has(runtimeID)))) {
        throw new Error(`${keywordLabel}.runtimeIds must be backed by at least one declared source node`)
      }
    }
    for (const node of tree.nodes) {
      const runtimeTokens = runtimeTokensByNode.get(node.id)
      const expectedKeywordIDs = tree.keywords
        .filter((keyword) => keyword.runtimeIds.some((runtimeID) => runtimeTokens.has(runtimeID)))
        .map((keyword) => keyword.id)
        .sort()
      const authoredKeywordIDs = [...node.keywordIds].sort()
      if (authoredKeywordIDs.some((keywordID) => !keywordsByID.has(keywordID))) throw new Error(`specialization_trees.nodes.keywordIds reference not found: ${treeID}.${node.id}`)
      if (authoredKeywordIDs.join("|") !== expectedKeywordIDs.join("|")) {
        throw new Error(`specialization_trees.nodes.keywordIds must exactly match authored keyword mechanics: ${treeID}.${node.id}`)
      }
    }

    if (!specializationIsObject(tree.i18n)) throw new Error(`specialization_trees.i18n invalid: ${treeID}`)
    for (const localeTag of ["zh-CN", "en-US"]) {
      const locale = tree.i18n[localeTag]
      if (!specializationIsObject(locale)) throw new Error(`specialization_trees.i18n locale missing: ${treeID}.${localeTag}`)
      specializationRequiredString(locale.name, `specialization_trees.i18n.name: ${treeID}.${localeTag}`)
      specializationRequiredString(locale.desc, `specialization_trees.i18n.desc: ${treeID}.${localeTag}`)
      if (!specializationIsObject(locale.schools) || Object.keys(locale.schools).length !== schoolIDs.size) throw new Error(`specialization_trees.i18n.schools invalid: ${treeID}.${localeTag}`)
      for (const schoolID of schoolIDs) {
        const entry = locale.schools[schoolID]
        if (!specializationIsObject(entry)) throw new Error(`specialization_trees.i18n.schools entry missing: ${treeID}.${localeTag}.${schoolID}`)
        specializationRequiredString(entry.name, `specialization_trees.i18n.schools.name: ${treeID}.${localeTag}.${schoolID}`)
        specializationRequiredString(entry.desc, `specialization_trees.i18n.schools.desc: ${treeID}.${localeTag}.${schoolID}`)
      }
      if (!specializationIsObject(locale.keywords) || Object.keys(locale.keywords).length !== keywordIDs.size) throw new Error(`specialization_trees.i18n.keywords invalid: ${treeID}.${localeTag}`)
      for (const keywordID of keywordIDs) {
        const entry = locale.keywords[keywordID]
        const entryLabel = `specialization_trees.i18n.keywords.${treeID}.${localeTag}.${keywordID}`
        if (!specializationIsObject(entry)) throw new Error(`${entryLabel} missing`)
        specializationRejectUnknownKeys(entry, ["name", "playerSummary", "mechanicDetails", "sourceHint"], entryLabel)
        specializationRequiredString(entry.name, `${entryLabel}.name`)
        specializationRequiredString(entry.playerSummary, `${entryLabel}.playerSummary`)
        specializationRequiredString(entry.mechanicDetails, `${entryLabel}.mechanicDetails`)
        specializationRequiredString(entry.sourceHint, `${entryLabel}.sourceHint`)
      }
      if (!specializationIsObject(locale.nodes) || Object.keys(locale.nodes).length !== nodeIDs.size) throw new Error(`specialization_trees.i18n.nodes invalid: ${treeID}.${localeTag}`)
      for (const nodeID of nodeIDs) {
        const entry = locale.nodes[nodeID]
        const nodeKeywordIDs = new Set(nodeByID.get(nodeID).keywordIds)
        if (!specializationIsObject(entry)) throw new Error(`specialization_trees.i18n.nodes entry missing: ${treeID}.${localeTag}.${nodeID}`)
        specializationRejectUnknownKeys(entry, ["name", "desc", "effectSummary", "levelEffectSummary"], `specialization_trees.i18n.nodes.${treeID}.${localeTag}.${nodeID}`)
        specializationRequiredString(entry.name, `specialization_trees.i18n.nodes.name: ${treeID}.${localeTag}.${nodeID}`)
        specializationRequiredString(entry.desc, `specialization_trees.i18n.nodes.desc: ${treeID}.${localeTag}.${nodeID}`)
        if (!Array.isArray(entry.effectSummary) || entry.effectSummary.length < 1 || entry.effectSummary.length > 7) {
          throw new Error(`specialization_trees.i18n.nodes.effectSummary must contain 1-7 lines: ${treeID}.${localeTag}.${nodeID}`)
        }
        for (let lineIndex = 0; lineIndex < entry.effectSummary.length; lineIndex += 1) {
          const lineLabel = `specialization_trees.i18n.nodes.effectSummary: ${treeID}.${localeTag}.${nodeID}[${lineIndex}]`
          const line = specializationRequiredString(entry.effectSummary[lineIndex], lineLabel)
          if (line.length > 180) throw new Error(`${lineLabel} must not exceed 180 characters`)
          validateSpecializationPlayerCopy(line, lineLabel, keywordIDs, nodeKeywordIDs)
        }
        if (!Array.isArray(entry.levelEffectSummary) || entry.levelEffectSummary.length !== nodeMaxRanks.get(nodeID)) {
          throw new Error(`specialization_trees.i18n.nodes.levelEffectSummary must match node ranks: ${treeID}.${localeTag}.${nodeID}`)
        }
        for (let levelIndex = 0; levelIndex < entry.levelEffectSummary.length; levelIndex += 1) {
          const lines = entry.levelEffectSummary[levelIndex]
          if (!Array.isArray(lines) || lines.length < 1 || lines.length > 4) throw new Error(`specialization_trees.i18n.nodes.levelEffectSummary must contain 1-4 lines per rank: ${treeID}.${localeTag}.${nodeID}[${levelIndex}]`)
          for (let lineIndex = 0; lineIndex < lines.length; lineIndex += 1) {
            const line = specializationRequiredString(lines[lineIndex], `specialization_trees.i18n.nodes.levelEffectSummary: ${treeID}.${localeTag}.${nodeID}[${levelIndex}][${lineIndex}]`)
            if (line.length > 180) throw new Error(`specialization_trees.i18n.nodes.levelEffectSummary must remain short player-facing copy: ${treeID}.${localeTag}.${nodeID}[${levelIndex}][${lineIndex}]`)
            validateSpecializationPlayerCopy(line, `specialization_trees.i18n.nodes.levelEffectSummary: ${treeID}.${localeTag}.${nodeID}[${levelIndex}][${lineIndex}]`, keywordIDs, nodeKeywordIDs)
          }
          const rankCopy = lines.join("\n")
          for (const keywordID of nodeKeywordIDs) {
            if (!rankCopy.includes(`[[keyword:${keywordID}]]`)) throw new Error(`specialization_trees.i18n.nodes.levelEffectSummary must tag keyword ${keywordID}: ${treeID}.${localeTag}.${nodeID}[${levelIndex}]`)
          }
        }
      }
    }

    const prerequisitesByNode = new Map()
    const grantOwners = new Map()
    const nodesBySchool = new Map(SPECIALIZATION_SCHOOL_IDS.map((schoolID) => [schoolID, { root: [], technique: [], terminal: [] }]))
    const confluencePairs = new Map()
    const exclusiveGroups = new Map()
    let totalCost = 0
    for (const node of tree.nodes) {
      const nodeLabel = `specialization_trees.nodes: ${treeID}.${node.id}`
      const prerequisites = []
      for (let index = 0; index < node.requirements.all.length; index += 1) {
        const prerequisite = validateSpecializationRequirement(node.requirements.all[index], `${nodeLabel}.requirements.all[${index}]`, nodeMaxRanks, attributeIDs)
        if (prerequisite) prerequisites.push(prerequisite)
      }
      for (let groupIndex = 0; groupIndex < node.requirements.anyOf.length; groupIndex += 1) {
        const group = node.requirements.anyOf[groupIndex]
        if (!Array.isArray(group) || group.length === 0) throw new Error(`${nodeLabel}.requirements.anyOf[${groupIndex}] must be a non-empty array`)
        for (let index = 0; index < group.length; index += 1) {
          const prerequisite = validateSpecializationRequirement(group[index], `${nodeLabel}.requirements.anyOf[${groupIndex}][${index}]`, nodeMaxRanks, attributeIDs)
          if (prerequisite) prerequisites.push(prerequisite)
        }
      }
      const pureSchoolGates = [
        ...node.requirements.all,
        ...node.requirements.anyOf.flat()
      ].filter((requirement) => requirement.kind === "pure_school_points_gte")
      if (node.kind === "terminal") {
        if (pureSchoolGates.length !== 1 || !node.requirements.all.includes(pureSchoolGates[0]) || pureSchoolGates[0].schoolId !== node.schoolIds[0] || pureSchoolGates[0].points !== 11) {
          throw new Error(`${nodeLabel} terminal must require 11 points already invested in its own pure school`)
        }
      } else if (node.kind === "confluence") {
        const expectedPoints = node.id === "confluence_intelligence_perception_crownless_astrolabe" ? 11 : 9
        if (pureSchoolGates.length !== 2 || pureSchoolGates.some((gate) => !node.requirements.all.includes(gate) || gate.points !== expectedPoints)
          || new Set(pureSchoolGates.map((gate) => gate.schoolId)).size !== 2
          || node.schoolIds.some((schoolID) => !pureSchoolGates.some((gate) => gate.schoolId === schoolID))) {
          throw new Error(`${nodeLabel} confluence must require ${expectedPoints} pure points in each participating school`)
        }
      } else if (pureSchoolGates.length !== 0) {
        throw new Error(`${nodeLabel} root and technique nodes cannot declare pure-school investment gates`)
      }
      if (prerequisites.includes(node.id)) throw new Error(`${nodeLabel} cannot require itself`)
      prerequisitesByNode.set(node.id, [...new Set(prerequisites)])
      for (let levelIndex = 0; levelIndex < node.levels.length; levelIndex += 1) {
        const levelGrantIDs = new Set()
        for (let index = 0; index < node.levels[levelIndex].grants.length; index += 1) {
          const grant = node.levels[levelIndex].grants[index]
          validateSpecializationGrant(grant, `${nodeLabel}.levels[${levelIndex}].grants[${index}]`, refs)
          if (levelGrantIDs.has(grant.id)) throw new Error(`${nodeLabel}.grant id duplicate within rank ${levelIndex + 1}: ${grant.id}`)
          levelGrantIDs.add(grant.id)
          const previousOwner = grantOwners.get(grant.id)
          if (previousOwner && previousOwner !== node.id) throw new Error(`${nodeLabel}.grant id belongs to another node: ${grant.id}`)
          grantOwners.set(grant.id, node.id)
          if (grant.kind === "equipment_slot_capacity" && levelIndex !== node.levels.length - 1) {
            throw new Error(`${nodeLabel} equipment topology grant must be introduced only at the final rank`)
          }
        }
      }
      for (let levelIndex = 1; levelIndex < node.levels.length; levelIndex += 1) {
        const previousGrants = new Map(node.levels[levelIndex - 1].grants.map((grant) => [grant.id, grant]))
        const currentGrants = new Map(node.levels[levelIndex].grants.map((grant) => [grant.id, grant]))
        for (const [grantID, previousGrant] of previousGrants) {
          validateSpecializationGrantSnapshotRetention(previousGrant, currentGrants.get(grantID), `${nodeLabel}.levels[${levelIndex}]`)
        }
      }
      validateSpecializationRiskPolicy(node, nodeLabel)
      if (!node.levels[0].grants.some((grant) => grant.kind !== "stat_modifier")) {
        throw new Error(`${nodeLabel} must define a gameplay grant instead of pure stat filler`)
      }
      const formulaSchoolIDs = specializationFormulaSchoolAttributeIDs(node.levels)
      const declaredSchoolIDs = new Set(node.schoolIds)
      const hasForeignFormulaSchool = [...formulaSchoolIDs].some((schoolID) => !declaredSchoolIDs.has(schoolID))
      const missesDeclaredFormulaSchool = formulaSchoolIDs.size > 0 && node.schoolIds.some((schoolID) => !formulaSchoolIDs.has(schoolID))
      if (hasForeignFormulaSchool || missesDeclaredFormulaSchool) {
        throw new Error(`${nodeLabel} formula school attributes must exactly match schoolIds when school scaling is present`)
      }
      const allGrants = node.levels.flatMap((level) => level.grants)
      const topologyNode = allGrants.some((grant) => grant.kind === "equipment_slot_capacity")
      const expectedTopologyNode = Object.prototype.hasOwnProperty.call(SPECIALIZATION_TOPOLOGY_COSTS, node.id)
      if (topologyNode !== expectedTopologyNode) throw new Error(`${nodeLabel} equipment topology grant must stay on the approved high-cost node ids`)
      totalCost += node.levels.reduce((sum, level) => sum + level.cost, 0)
      if (node.kind === "confluence") {
        const pair = [...node.schoolIds].sort().join("+")
        const pairNodes = confluencePairs.get(pair) || []
        pairNodes.push(node)
        confluencePairs.set(pair, pairNodes)
      } else {
        nodesBySchool.get(node.schoolIds[0])[node.kind].push(node)
      }
      if (node.exclusiveGroupId) {
        const groupID = specializationRequiredString(node.exclusiveGroupId, `${nodeLabel}.exclusiveGroupId`)
        const group = exclusiveGroups.get(groupID) || []
        group.push(node)
        exclusiveGroups.set(groupID, group)
      }
    }
    if (confluencePairs.size !== 10 || [...confluencePairs.values()].some((nodes) => nodes.length !== 2)) {
      throw new Error(`specialization_trees must define two horizontal confluences for all ten school pairs: ${treeID}`)
    }
    for (const [schoolID, schoolNodes] of nodesBySchool) {
      if (schoolNodes.root.length !== 2 || schoolNodes.technique.length !== 4 || schoolNodes.terminal.length !== 4) {
        throw new Error(`specialization_trees school branch shape invalid: ${treeID}.${schoolID}`)
      }
      const rootIDs = new Set(schoolNodes.root.map((root) => root.id))
      const techniquesByRoot = new Map(schoolNodes.root.map((root) => [root.id, []]))
      for (const root of schoolNodes.root) {
        if (prerequisitesByNode.get(root.id).length !== 0) throw new Error(`specialization_trees root cannot have node prerequisites: ${treeID}.${root.id}`)
        if (!root.levels[0].grants.some((grant) => grant.kind === "battle_hook" || grant.kind === "effect_mutation")) {
          throw new Error(`specialization_trees root must define a combat loop with battle_hook or effect_mutation: ${treeID}.${root.id}`)
        }
      }
      for (const technique of schoolNodes.technique) {
        const roots = prerequisitesByNode.get(technique.id).filter((id) => rootIDs.has(id))
        if (roots.length !== 1) throw new Error(`specialization_trees technique must require exactly one horizontal root: ${treeID}.${technique.id}`)
        const rootRequirement = technique.requirements.all.find((requirement) => requirement.kind === "node_rank" && requirement.nodeId === roots[0])
        if (!rootRequirement || rootRequirement.minRank !== 6 || technique.requirements.anyOf.length !== 0) {
          throw new Error(`specialization_trees technique must require its root at rank 6: ${treeID}.${technique.id}`)
        }
        techniquesByRoot.get(roots[0]).push(technique)
      }
      if ([...techniquesByRoot.values()].some((techniques) => techniques.length !== 2)) throw new Error(`specialization_trees each root must own two techniques: ${treeID}.${schoolID}`)
      const terminalsByRoot = new Map(schoolNodes.root.map((root) => [root.id, []]))
      for (const terminal of schoolNodes.terminal) {
        const prerequisites = prerequisitesByNode.get(terminal.id)
        const directTechniques = schoolNodes.technique.filter((technique) => prerequisites.includes(technique.id))
        const allNodeRequirements = terminal.requirements.all.filter((requirement) => requirement.kind === "node_rank")
        const nodeChoiceGroups = terminal.requirements.anyOf.filter((group) => group.some((requirement) => requirement.kind === "node_rank"))
        const singleRoute = allNodeRequirements.length === 1 && terminal.requirements.anyOf.length === 0
        const choiceRoute = allNodeRequirements.length === 0
          && terminal.requirements.anyOf.length === 1
          && nodeChoiceGroups.length === 1
          && nodeChoiceGroups[0].length === 2
          && nodeChoiceGroups[0].every((requirement) => requirement.kind === "node_rank")
        if (!singleRoute && !choiceRoute) throw new Error(`specialization_trees terminal must require one direct technique or one two-technique choice: ${treeID}.${terminal.id}`)
        const terminalRankRequirements = [...allNodeRequirements, ...nodeChoiceGroups.flat()]
        if (terminalRankRequirements.some((requirement) => requirement.kind !== "node_rank" || requirement.minRank !== 4)) {
          throw new Error(`specialization_trees terminal technique prerequisites must be rank 4: ${treeID}.${terminal.id}`)
        }
        if (directTechniques.length !== prerequisites.length || directTechniques.length < 1 || directTechniques.length > 2) {
          throw new Error(`specialization_trees terminal prerequisites must be direct techniques from its own school: ${treeID}.${terminal.id}`)
        }
        const parentRoots = new Set(directTechniques.flatMap((technique) => schoolNodes.root
          .filter((root) => prerequisitesByNode.get(technique.id).includes(root.id))
          .map((root) => root.id)))
        if (parentRoots.size !== 1 || prerequisites.some((id) => rootIDs.has(id))) {
          throw new Error(`specialization_trees terminal choices must stay under one root and must not repeat that root: ${treeID}.${terminal.id}`)
        }
        terminalsByRoot.get([...parentRoots][0]).push(terminal)
      }
      if ([...terminalsByRoot.values()].some((terminals) => terminals.length !== 2)) throw new Error(`specialization_trees each root must lead to two terminals: ${treeID}.${schoolID}`)
    }
    for (const node of tree.nodes.filter((row) => row.kind === "confluence")) {
      if (node.requirements.all.some((requirement) => requirement.kind === "node_rank")) throw new Error(`specialization_trees confluence must declare only direct technique choices: ${treeID}.${node.id}`)
      const expectedTechniqueRank = node.id === "confluence_intelligence_perception_crownless_astrolabe" ? 4 : 2
      for (const schoolID of node.schoolIds) {
        const schoolNodes = nodesBySchool.get(schoolID)
        const hasTechniqueChoice = node.requirements.anyOf.some((group) => {
          const groupIDs = group.filter((req) => req.kind === "node_rank").map((req) => req.nodeId)
          if (groupIDs.length !== 2 || group.some((requirement) => requirement.kind !== "node_rank" || requirement.minRank !== expectedTechniqueRank)
            || groupIDs.some((id) => !schoolNodes.technique.some((technique) => technique.id === id))) return false
          const parentRoots = new Set(groupIDs.flatMap((id) => schoolNodes.root.filter((root) => prerequisitesByNode.get(id).includes(root.id)).map((root) => root.id)))
          return parentRoots.size === 1
        })
        if (!hasTechniqueChoice) throw new Error(`specialization_trees confluence must accept either direct technique from one branch: ${treeID}.${node.id}.${schoolID}`)
      }
    }
    for (const [groupID, group] of exclusiveGroups) {
      if (group.length !== 2 || group.some((node) => node.kind !== "terminal") || new Set(group.map((node) => node.schoolIds[0])).size !== 1) {
        throw new Error(`specialization_trees exclusive terminal group invalid: ${treeID}.${groupID}`)
      }
    }
    if (exclusiveGroups.size !== SPECIALIZATION_SCHOOL_IDS.length * 2 || tree.nodes.some((node) => node.kind === "terminal" && !node.exclusiveGroupId)) {
      throw new Error(`specialization_trees must have two two-terminal exclusive groups per school: ${treeID}`)
    }

    const visiting = new Set()
    const visited = new Set()
    const reaches = (fromID, targetID, seen = new Set()) => {
      if (fromID === targetID) return true
      if (seen.has(fromID)) return false
      seen.add(fromID)
      return (prerequisitesByNode.get(fromID) || []).some((dependencyID) => reaches(dependencyID, targetID, seen))
    }
    const visit = (nodeID) => {
      if (visiting.has(nodeID)) throw new Error(`specialization_trees dependency cycle detected: ${treeID}.${nodeID}`)
      if (visited.has(nodeID)) return
      visiting.add(nodeID)
      for (const prerequisiteID of prerequisitesByNode.get(nodeID) || []) visit(prerequisiteID)
      visiting.delete(nodeID)
      visited.add(nodeID)
    }
    for (const nodeID of nodeIDs) visit(nodeID)
    for (const node of tree.nodes) {
      const direct = prerequisitesByNode.get(node.id) || []
      for (const dependencyID of direct) {
        if (direct.some((otherID) => otherID !== dependencyID && reaches(otherID, dependencyID))) {
          throw new Error(`specialization_trees transitively redundant prerequisite: ${treeID}.${node.id}->${dependencyID}`)
        }
      }
    }
    if (tree.maxPoints >= totalCost) throw new Error(`specialization_trees.maxPoints must preserve build choice: ${treeID}`)
    validateSpecializationCapacityEnvelope(tree.nodes, treeID)
  }
  validateSpecializationMutationSemantics(trees, refs)
}

function validateGenericTable(name, rows) {
  if (!Array.isArray(rows)) {
    throw new Error(`${name} must be an array`)
  }
  const ids = new Set()
  const profileKeys = new Set()
  for (const row of rows) {
    if (!row || typeof row !== "object") throw new Error(`${name} row must be object`)
    if (Object.prototype.hasOwnProperty.call(row, "id")) {
      if (typeof row.id !== "string" || row.id.trim() === "") {
        continue
      }
      if (name === "attribute_exp_profiles") {
        const level = parseNumber(row.level, Number.NaN)
        if (!Number.isFinite(level)) throw new Error("attribute_exp_profiles.level must be number")
        const key = `${row.id}::${level}`
        if (profileKeys.has(key)) throw new Error(`duplicate attribute_exp_profiles row: ${key}`)
        profileKeys.add(key)
      } else if (ids.has(row.id)) {
        throw new Error(`duplicate ${name}.id: ${row.id}`)
      }
      ids.add(row.id)
    }
  }
}

function validateOperationEffects(rows, items, actions, actionMainTypes, actionSubTypes, iconIdSet) {
  if (!Array.isArray(rows)) throw new Error("operation_effects must be an array")
  const knownActions = new Set((actions || []).map((row) => String(row?.id || "").trim()).filter(Boolean))
  knownActions.add("equipment.enchant")
  knownActions.add("equipment_enhance") // Built-in spec, not an authored ordinary action.
  const knownMainTypes = new Set((actionMainTypes || []).map((row) => String(row?.id || "").trim()).filter(Boolean))
  const knownSubTypes = new Set((actionSubTypes || []).map((row) => String(row?.id || "").trim()).filter(Boolean))
  const knownTargets = new Set(["player", "kitty", "all_kitties", "player_and_kitties"])
  const knownKinds = new Map([
    ["life_action.material_refund_chance", "life_action"],
    ["life_action.extra_main_output_chance", "life_action"],
    ["life_action.harvest_bonus", "life_action"],
    ["life_action.focused_harvest", "life_action"],
    ["alchemy.conversion_loss_reduction", "alchemy"],
    ["life_action.patient_harvest", "life_action"],
    ["life_action.work_experience", "life_action"],
    ["alchemy.infusion_capacity_floor_tradeoff", "alchemy"],
    ["alchemy.infusion_bulk_capacity_floor_tradeoff", "alchemy"],
    ["alchemy.infusion_precious_capacity_bonus", "alchemy"],
    ["alchemy.infusion_precision_ceiling_bonus", "alchemy"],
    ["alchemy.infusion_success_bonus", "alchemy"],
    ["alchemy.transmute_duration", "alchemy"],
    ["alchemy.transmute_gold_bonus", "alchemy"],
    ["enchantment.high_rarity_weight", "enchantment"],
    ["enhancement.success_bonus", "enhancement"],
    ["fishing_cast.bite_bonus", "fishing_cast"],
    ["fishing_cast.landing_bonus", "fishing_cast"],
    ["fishing_cast.line_load_reduction", "fishing_cast"]
  ])
  const infusionKinds = new Set([
    "alchemy.infusion_capacity_floor_tradeoff",
    "alchemy.infusion_bulk_capacity_floor_tradeoff",
    "alchemy.infusion_precious_capacity_bonus",
    "alchemy.infusion_precision_ceiling_bonus",
    "alchemy.infusion_success_bonus"
  ])
  const ids = new Set()
  for (const row of rows) {
    if (!row || typeof row !== "object" || Array.isArray(row)) throw new Error("operation_effects row must be object")
    const id = String(row.id || "").trim()
    if (!id) throw new Error("operation_effects.id is required")
    if (ids.has(id)) throw new Error(`duplicate operation_effects.id: ${id}`)
    ids.add(id)
    const kind = String(row.kind || "").trim()
    const domain = String(row.domain || "").trim()
    if (!knownKinds.has(kind)) throw new Error(`operation_effects.kind is not registered: ${id}.${kind}`)
    if (knownKinds.get(kind) !== domain) throw new Error(`operation_effects.domain mismatch: ${id}.${domain}`)
    for (const key of ["rulesetVersion", "nameKey", "descKey"]) {
      if (!String(row[key] || "").trim()) throw new Error(`operation_effects.${key} is required: ${id}`)
    }
    const iconID = String(row.iconId || "").trim()
    if (iconID && !iconIdSet.has(iconID)) throw new Error(`operation_effects.iconId not found: ${id}.${iconID}`)
    if (row.priority !== undefined && (!Number.isSafeInteger(row.priority) || row.priority < -2147483648 || row.priority > 2147483647)) {
      throw new Error(`operation_effects.priority is invalid: ${id}`)
    }
    if (!Array.isArray(row.allowedTargets) || row.allowedTargets.length === 0) {
      throw new Error(`operation_effects.allowedTargets is required: ${id}`)
    }
    const targets = new Set()
    for (const target of row.allowedTargets) {
      if (!knownTargets.has(target)) throw new Error(`operation_effects.allowedTargets is invalid: ${id}.${target}`)
      if (targets.has(target)) throw new Error(`operation_effects.allowedTargets is duplicated: ${id}.${target}`)
      targets.add(target)
    }
    const lifetime = row.lifetime
    if (!lifetime || typeof lifetime !== "object" || Array.isArray(lifetime)) {
      throw new Error(`operation_effects.lifetime is required: ${id}`)
    }
    const stacking = row.stacking
    if (!stacking || typeof stacking !== "object" || Array.isArray(stacking) || !String(stacking.key || "").trim()) {
      throw new Error(`operation_effects.stacking is invalid: ${id}`)
    }
    if (stacking.exclusive !== undefined && (typeof stacking.exclusive !== "boolean" || (stacking.exclusive && stacking.mode === "independent"))) {
      throw new Error(`operation_effects.stacking.exclusive is invalid: ${id}`)
    }
    if (lifetime.kind === "duration") {
      if (!Number.isSafeInteger(lifetime.durationSeconds) || lifetime.durationSeconds <= 0 || lifetime.durationSeconds > 9223372036 || lifetime.charges !== undefined) {
        throw new Error(`operation_effects duration lifetime is invalid: ${id}`)
      }
      if (!["extend_duration", "independent"].includes(stacking.mode)) {
        throw new Error(`operation_effects duration stacking mode is invalid: ${id}`)
      }
      if (stacking.maxDurationSeconds !== undefined && (!Number.isSafeInteger(stacking.maxDurationSeconds) || stacking.maxDurationSeconds < lifetime.durationSeconds || stacking.maxDurationSeconds > 9223372036)) {
        throw new Error(`operation_effects.maxDurationSeconds is invalid: ${id}`)
      }
      if (stacking.maxCharges !== undefined) {
        throw new Error(`operation_effects duration cannot define maxCharges: ${id}`)
      }
      if (stacking.mode === "independent" && stacking.maxDurationSeconds !== undefined) {
        throw new Error(`operation_effects independent duration cannot define maxDurationSeconds: ${id}`)
      }
    } else if (lifetime.kind === "charges") {
      if (!Number.isSafeInteger(lifetime.charges) || lifetime.charges <= 0 || lifetime.durationSeconds !== undefined) {
        throw new Error(`operation_effects charge lifetime is invalid: ${id}`)
      }
      if (!["add_charges", "independent"].includes(stacking.mode)) {
        throw new Error(`operation_effects charge stacking mode is invalid: ${id}`)
      }
      if (stacking.maxCharges !== undefined && (!Number.isSafeInteger(stacking.maxCharges) || stacking.maxCharges < lifetime.charges)) {
        throw new Error(`operation_effects.maxCharges is invalid: ${id}`)
      }
      if (stacking.maxDurationSeconds !== undefined) {
        throw new Error(`operation_effects charges cannot define maxDurationSeconds: ${id}`)
      }
      if (stacking.mode === "independent" && stacking.maxCharges !== undefined) {
        throw new Error(`operation_effects independent charges cannot define maxCharges: ${id}`)
      }
    } else {
      throw new Error(`operation_effects.lifetime.kind is invalid: ${id}`)
    }
    const selector = row.selector || {}
    if (!selector || typeof selector !== "object" || Array.isArray(selector)) {
      throw new Error(`operation_effects.selector is invalid: ${id}`)
    }
    for (const [key, known] of [["actionIds", knownActions], ["mainTypes", knownMainTypes], ["subTypes", knownSubTypes]]) {
      const values = selector[key]
      if (values === undefined) continue
      if (!Array.isArray(values) || values.some((value) => typeof value !== "string" || !value.trim())) {
        throw new Error(`operation_effects.selector.${key} is invalid: ${id}`)
      }
      if (new Set(values).size !== values.length) throw new Error(`operation_effects.selector.${key} has duplicates: ${id}`)
      for (const value of values) {
        if (!known.has(value)) throw new Error(`operation_effects.selector.${key} reference not found: ${id}.${value}`)
      }
    }
    if (infusionKinds.has(kind) && Array.isArray(selector.actionIds) && selector.actionIds.some((actionID) => actionID !== "alchemy.bench")) {
      throw new Error(`alchemy infusion operation_effects.selector.actionIds must contain only alchemy.bench: ${id}`)
    }
    const params = row.params
    if (kind === "alchemy.conversion_loss_reduction" && (Object.keys(params || {}).join() !== "lossReductionRate" || !Number.isFinite(params.lossReductionRate) || params.lossReductionRate <= 0 || params.lossReductionRate > .10 || selector.actionIds?.join() !== "alchemy.convert" || !stacking.exclusive || stacking.key !== "food.conversion")) throw new Error(`invalid conversion food: ${id}`)
    if (!params || typeof params !== "object" || Array.isArray(params)) throw new Error(`operation_effects.params is invalid: ${id}`)
    if (["alchemy.transmute_duration", "alchemy.transmute_gold_bonus", "enchantment.high_rarity_weight"].includes(kind)) {
const enchant = kind === "enchantment.high_rarity_weight", speed = kind === "alchemy.transmute_duration";
const field = enchant ? "weightBonusRate" : speed ? "durationSeconds" : "bonusRate";
if (Object.keys(params).length !== 1 || !Number.isFinite(params[field]) || params[field] <= 0 || params[field] > (enchant ? .25 : speed ? 1 : .10) || (speed && params[field] !== 1)) throw new Error("invalid point gold / enchant effect: " + id);
if ((selector.actionIds || []).some(id => id !== (enchant ? "equipment.enchant" : "alchemy.transmute"))) throw new Error("invalid point gold / enchant selector: " + id);
if ((speed || enchant) && lifetime.kind !== "charges" || enchant && lifetime.charges !== 1) throw new Error("invalid point gold / enchant lifetime: " + id);
}
    if (["fishing_cast.landing_bonus", "fishing_cast.line_load_reduction"].includes(kind)) {
      if ((selector.actionIds || []).some((value) => value !== "fishing.trip")) throw new Error(`fishing effect selector mismatch: ${id}`)
      if (!Number.isFinite(params.rate) || params.rate <= 0 || params.rate > 0.15 || Object.keys(params).some((key) => key !== "rate")) throw new Error(`invalid fishing effect params: ${id}`)
    }
    if (["enhancement.success_bonus", "fishing_cast.bite_bonus"].includes(kind)) {
      const actionID = kind === "enhancement.success_bonus" ? "equipment_enhance" : "fishing.trip"
      const maximum = kind === "enhancement.success_bonus" ? 0.10 : 0.25
      if ((selector.actionIds || []).some((value) => value !== actionID)) throw new Error(`rate effect selector mismatch: ${id}`)
      if (!Number.isFinite(params.bonusRate) || params.bonusRate <= 0 || params.bonusRate > maximum || Object.keys(params).some((key) => key !== "bonusRate")) throw new Error(`invalid rate effect params: ${id}`)
    }
    if (["life_action.harvest_bonus", "life_action.focused_harvest", "life_action.patient_harvest", "life_action.work_experience"].includes(kind)) {
      const allowed = kind === "life_action.work_experience" ? ["experienceBonusRate"] : kind === "life_action.patient_harvest" ? ["chance", "durationPenaltyRate"] : ["chance"]
      if (kind === "life_action.focused_harvest" || kind === "life_action.harvest_bonus" && params.durationReductionRate !== undefined) allowed.push("durationReductionRate")
      if (Object.keys(params).sort().join() !== allowed.sort().join() || Object.values(params).some(v => !Number.isFinite(v) || v <= 0)) throw new Error(`invalid work meal fields: ${id}`)
      if (kind === "life_action.harvest_bonus" && params.chance > .05 || kind === "life_action.patient_harvest" && (params.chance < .08 || params.chance > .11 || params.durationPenaltyRate !== .04) || kind === "life_action.work_experience" && params.experienceBonusRate > .15) throw new Error(`invalid work meal bounds: ${id}`)
      if (kind === "life_action.focused_harvest" && params.chance > .03 || params.durationReductionRate > (kind === "life_action.focused_harvest" ? .08 : .05)) throw new Error(`invalid work meal haste: ${id}`)
      if (!selector.mainTypes?.length || selector.mainTypes.some(v => !["collecting", "mining", "farming_animal"].includes(v)) || !stacking.exclusive || stacking.key !== "food.primary_work") throw new Error(`invalid work meal scope or stacking: ${id}`)
    }
    if (kind === "life_action.material_refund_chance") {
      const chance = params.chance
      if (typeof chance !== "number" || !Number.isFinite(chance) || chance < 0 || chance > 1) {
        throw new Error(`operation_effects.params.chance is invalid: ${id}`)
      }
      if (typeof params.refundRate !== "number" || !Number.isFinite(params.refundRate) || params.refundRate <= 0 || params.refundRate > 1) {
        throw new Error(`operation_effects.params.refundRate is invalid: ${id}`)
      }
      const unknown = Object.keys(params).find((key) => !["chance", "refundRate"].includes(key))
      if (unknown) throw new Error(`operation_effects.params.${unknown} is unsupported: ${id}`)
    }
    if (kind === "life_action.extra_main_output_chance") {
      const chance = params.chance
      if (typeof chance !== "number" || !Number.isFinite(chance) || chance < 0 || chance > 1) {
        throw new Error(`operation_effects.params.chance is invalid: ${id}`)
      }
      if (!Number.isInteger(params.quantity) || params.quantity <= 0 || params.quantity > 1000) {
        throw new Error(`operation_effects.params.quantity is invalid: ${id}`)
      }
      const unknown = Object.keys(params).find((key) => !["chance", "quantity"].includes(key))
      if (unknown) throw new Error(`operation_effects.params.${unknown} is unsupported: ${id}`)
    }
    if (["alchemy.infusion_capacity_floor_tradeoff", "alchemy.infusion_bulk_capacity_floor_tradeoff"].includes(kind)) {
      if (!Number.isSafeInteger(params.extraMaterialQuantity) || params.extraMaterialQuantity <= 0 || params.extraMaterialQuantity > 1000000) {
        throw new Error(`operation_effects.params.extraMaterialQuantity is invalid: ${id}`)
      }
      if (typeof params.positiveFloorReductionRate !== "number" || !Number.isFinite(params.positiveFloorReductionRate) || params.positiveFloorReductionRate <= 0 || params.positiveFloorReductionRate > 0.95) {
        throw new Error(`operation_effects.params.positiveFloorReductionRate is invalid: ${id}`)
      }
      const allowed = kind === "alchemy.infusion_bulk_capacity_floor_tradeoff" ? ["extraMaterialQuantity", "positiveFloorReductionRate", "quantityLimitBonusRate", "successRatePenalty", "materialTier"] : ["extraMaterialQuantity", "positiveFloorReductionRate"]
      if (params.materialTier !== undefined && ![1, 2].includes(params.materialTier)) throw new Error(`invalid capacity material tier: ${id}`)
      const unknown = Object.keys(params).find((key) => !allowed.includes(key))
      if (unknown) throw new Error(`operation_effects.params.${unknown} is unsupported: ${id}`)
    }
    if (kind === "alchemy.infusion_precious_capacity_bonus") {
      if (!Number.isSafeInteger(params.extraMaterialQuantity) || params.extraMaterialQuantity <= 0 || params.extraMaterialQuantity > 1000000 || Object.keys(params).some(key => !["extraMaterialQuantity","quantityLimitBonusRate","successRatePenalty"].includes(key))) {
        throw new Error(`operation_effects precious capacity params are invalid: ${id}`)
      }
    }
    const capacityRateLimit = params.materialTier === 1 ? .20 : params.materialTier === 2 ? .15 : .17
    if (params.quantityLimitBonusRate !== undefined && (typeof params.quantityLimitBonusRate !== "number" || !Number.isFinite(params.quantityLimitBonusRate) || params.quantityLimitBonusRate < 0 || params.quantityLimitBonusRate > capacityRateLimit)) throw new Error(`invalid capacity limit rate: ${id}`)
    if (params.successRatePenalty !== undefined && (typeof params.successRatePenalty !== "number" || !Number.isFinite(params.successRatePenalty) || params.successRatePenalty < 0 || params.successRatePenalty > .20)) throw new Error(`invalid capacity success penalty: ${id}`)
    if (kind === "alchemy.infusion_success_bonus") {
      if (typeof params.bonusRate !== "number" || !Number.isFinite(params.bonusRate) || params.bonusRate <= 0 || params.bonusRate > .10 || Object.keys(params).some(key => key !== "bonusRate")) {
        throw new Error(`operation_effects infusion success bonus is invalid: ${id}`)
      }
    }
    if (kind === "alchemy.infusion_precision_ceiling_bonus") {
      if (typeof params.positiveCeilingBonusRate !== "number" || !Number.isFinite(params.positiveCeilingBonusRate) || params.positiveCeilingBonusRate <= 0 || params.positiveCeilingBonusRate > 0.25) {
        throw new Error(`operation_effects.params.positiveCeilingBonusRate is invalid: ${id}`)
      }
      const unknown = Object.keys(params).find((key) => key !== "positiveCeilingBonusRate")
      if (unknown) throw new Error(`operation_effects.params.${unknown} is unsupported: ${id}`)
    }
  }
  for (const item of items || []) {
    const effectID = String(item?.useOperationEffectId || "").trim()
    if (!effectID) continue
    if (!ids.has(effectID)) throw new Error(`item.useOperationEffectId not found: ${item.id}.${effectID}`)
    if (!Array.isArray(item.type) || !item.type.includes("food")) {
      throw new Error(`item.useOperationEffectId requires food type: ${item.id}`)
    }
  }
}

function validateFishFoodRecipes(rows, items, actions, operationEffects, fishingSpecies) {
  if (!Array.isArray(rows)) throw new Error("fish_food_recipes must be an array")
  const itemByID = new Map((items || []).map((row) => [String(row?.id || "").trim(), row]))
  const actionByID = new Map((actions || []).map((row) => [String(row?.id || "").trim(), row]))
  const effectByID = new Map((operationEffects || []).map((row) => [String(row?.id || "").trim(), row]))
  const speciesIDs = new Set((fishingSpecies || []).map((row) => String(row?.id || "").trim()).filter(Boolean))
  const ids = new Set()
  const actionIDs = new Set()
  const outputIDs = new Set()
  const expectedLifetimeByKind = new Map([
    ["grilled_fish", "charges"],
    ["fish_soup", "duration"]
  ])
  for (const row of rows) {
    if (!row || typeof row !== "object" || Array.isArray(row)) throw new Error("fish_food_recipes row must be object")
    const id = String(row.id || "").trim()
    const rulesetVersion = String(row.rulesetVersion || "").trim()
    const actionID = String(row.actionId || "").trim()
    const outputItemID = String(row.outputItemId || "").trim()
    const kind = String(row.kind || "").trim()
    const qualityRulesetVersion = String(row.qualityRulesetVersion || "").trim()
    if (!id || ids.has(id)) throw new Error(`fish_food_recipes.id invalid or duplicate: ${id}`)
    if (!rulesetVersion) throw new Error(`fish_food_recipes.rulesetVersion is required: ${id}`)
    if (!actionID || actionIDs.has(actionID)) throw new Error(`fish_food_recipes.actionId invalid or duplicate: ${id}.${actionID}`)
    if (!outputItemID || outputIDs.has(outputItemID)) throw new Error(`fish_food_recipes.outputItemId invalid or duplicate: ${id}.${outputItemID}`)
    if (!expectedLifetimeByKind.has(kind)) throw new Error(`fish_food_recipes.kind is invalid: ${id}.${kind}`)
    if (!["fish-food-quality-v1", "fish-food-quality-v2"].includes(qualityRulesetVersion)) throw new Error(`fish_food_recipes.qualityRulesetVersion is unsupported: ${id}.${qualityRulesetVersion}`)
    if (row.allowSpecificIngredients !== true) throw new Error(`fish_food_recipes.allowSpecificIngredients must be true: ${id}`)
    ids.add(id)
    actionIDs.add(actionID)
    outputIDs.add(outputItemID)

    const action = actionByID.get(actionID)
    if (!action || action.mainType !== "cooking" || action.subType !== (qualityRulesetVersion === "fish-food-quality-v2" ? `cooking_${kind}` : "cooking_fish_food") || action.mainOutput !== outputItemID) {
      throw new Error(`fish_food_recipes action contract mismatch: ${id}.${actionID}`)
    }
    const reward = action.itemReward?.[outputItemID]
    const rewardCount = typeof reward === "number" ? reward : reward?.count
    if (rewardCount !== 1 || Object.keys(action.itemReward || {}).length !== 1) {
      throw new Error(`fish_food_recipes action must grant exactly one configured output: ${id}`)
    }
    if (!action.costs || typeof action.costs !== "object" || Object.keys(action.costs).length === 0 || Object.prototype.hasOwnProperty.call(action.costs, "fish_catch")) {
      throw new Error(`fish_food_recipes action costs must contain only explicit non-fish costs: ${id}`)
    }
    let hasLootCost = false
    let hasMaterialCost = false
    for (const [costItemID, quantity] of Object.entries(action.costs)) {
      const costItem = itemByID.get(costItemID)
      if (!costItem || !Number.isSafeInteger(quantity) || quantity <= 0) throw new Error(`fish_food_recipes action cost is invalid: ${id}.${costItemID}`)
      hasLootCost ||= Array.isArray(costItem.type) && costItem.type.includes("loot")
      hasMaterialCost ||= Array.isArray(costItem.type) && costItem.type.includes("material")
    }
    const workMeal = ["life_action.harvest_bonus", "life_action.focused_harvest", "life_action.patient_harvest", "life_action.work_experience", "alchemy.conversion_loss_reduction"].includes(effectByID.get(itemByID.get(outputItemID)?.useOperationEffectId)?.kind)
    if ((!hasLootCost && !workMeal) || !hasMaterialCost) throw new Error(`fish_food_recipes action requires both loot and food-material costs: ${id}`)

    const outputItem = itemByID.get(outputItemID)
    if (!outputItem || !Array.isArray(outputItem.type) || !outputItem.type.includes("food") || outputItem.stackable !== false || outputItem.marketListable !== true) {
      throw new Error(`fish_food_recipes output item contract mismatch: ${id}.${outputItemID}`)
    }
    const effect = effectByID.get(String(outputItem.useOperationEffectId || "").trim())
    if (!effect || effect.lifetime?.kind !== expectedLifetimeByKind.get(kind)) {
      throw new Error(`fish_food_recipes output effect lifetime mismatch: ${id}`)
    }
    if (effect.lifetime.kind === "duration" && effect.allowedTargets.some(target => target !== "player" && target !== "kitty")) {
      throw new Error(`fish_food_recipes timed food requires an individual target: ${id}`)
    }
    if (qualityRulesetVersion === "fish-food-quality-v2") {
      const cookingMaterial = kind === "grilled_fish" ? "coal" : "morning_dew"
      const minimumFuel = kind === "grilled_fish" ? 2 : 3
      if (workMeal ? action.costs[cookingMaterial] < minimumFuel : action.costs[cookingMaterial] !== minimumFuel) throw new Error(`fish food cooking material mismatch: ${id}.${cookingMaterial}`)
      const fields = {
        "life_action.harvest_bonus": ["chance"],
        "life_action.focused_harvest": ["chance", "durationReductionRate"],
        "alchemy.conversion_loss_reduction": ["lossReductionRate"],
        "life_action.patient_harvest": ["chance", "durationPenaltyRate"],
        "life_action.work_experience": ["experienceBonusRate"],
        "alchemy.infusion_capacity_floor_tradeoff": ["extraMaterialQuantity", "positiveFloorReductionRate"],
        "alchemy.infusion_bulk_capacity_floor_tradeoff": ["extraMaterialQuantity", "positiveFloorReductionRate"],
        "alchemy.infusion_precious_capacity_bonus": ["extraMaterialQuantity"],
        "alchemy.infusion_precision_ceiling_bonus": ["positiveCeilingBonusRate"],
        "alchemy.infusion_success_bonus": ["bonusRate"],
"alchemy.transmute_duration": [],
"alchemy.transmute_gold_bonus": ["bonusRate"],
"enchantment.high_rarity_weight": ["weightBonusRate"],
        "enhancement.success_bonus": ["bonusRate"], "fishing_cast.bite_bonus": ["bonusRate"],
        "fishing_cast.landing_bonus": ["rate"], "fishing_cast.line_load_reduction": ["rate"]
      }[effect.kind]
      if (fields && effect.params.quantityLimitBonusRate !== undefined) fields.push("quantityLimitBonusRate")
      if (fields && effect.params.successRatePenalty !== undefined) fields.push("successRatePenalty")
      if (fields && effect.kind === "life_action.harvest_bonus" && effect.params.durationReductionRate !== undefined) fields.push("durationReductionRate")
      if (!fields || !row.parameterScaling || Object.keys(row.parameterScaling).sort().join() !== fields.sort().join()) throw new Error(`fish food scaling fields mismatch: ${id}`)
      for (const [name, scale] of Object.entries(row.parameterScaling)) {
        const growth = ["size_growth", "bounded_growth"].includes(scale?.curve)
        if (name === "lossReductionRate" && scale?.max > .10) throw new Error(`fish food conversion scaling exceeds handler bounds: ${id}.${name}`)
        if (!scale || [scale.length, scale.weight, scale.traits, scale.min, scale.max].some((value) => !Number.isFinite(value) || value < 0) || (!growth && scale.length+scale.weight+scale.traits > 0.8) || scale.max < scale.min || scale.min > effect.params[name] || scale.max < effect.params[name]) throw new Error(`invalid fish food scaling: ${id}.${name}`)
        if (Object.keys(scale).some((key) => !["length", "weight", "traits", "min", "max", "inverse", "integer", "curve", "maxBeforeMutation"].includes(key)) || [scale.inverse, scale.integer].some((value) => value !== undefined && typeof value !== "boolean")) throw new Error(`invalid fish food scaling fields: ${id}.${name}`)
        if (growth ? (!(scale.curve === "bounded_growth" ? ["bonusRate", "weightBonusRate", "chance", "experienceBonusRate", "durationReductionRate", "lossReductionRate"] : ["extraMaterialQuantity", "quantityLimitBonusRate"]).includes(name) || Math.abs(scale.length+scale.weight-1) > 1e-9 || scale.traits > 0.2 || !Number.isFinite(scale.maxBeforeMutation) || (name === "extraMaterialQuantity" && !Number.isSafeInteger(scale.maxBeforeMutation)) || scale.maxBeforeMutation < effect.params[name] || scale.maxBeforeMutation > scale.max) : (scale.curve !== undefined || scale.maxBeforeMutation !== undefined)) throw new Error(`invalid fish food growth curve: ${id}.${name}`)
        if (Boolean(scale.inverse) !== (["positiveFloorReductionRate", "successRatePenalty"].includes(name)) || Boolean(scale.integer) !== (name === "extraMaterialQuantity")) throw new Error(`invalid fish food scaling mode: ${id}.${name}`)
        const capacityRateLimit = effect.params.materialTier === 1 ? .20 : effect.params.materialTier === 2 ? .15 : .17
        const maximum = name === "successRatePenalty" ? .20 : name === "quantityLimitBonusRate" ? capacityRateLimit : name === "extraMaterialQuantity" ? 1000000 : name === "positiveFloorReductionRate" ? 0.95 : name === "positiveCeilingBonusRate" ? 0.25 : name === "rate" ? 0.15 : ["alchemy.infusion_success_bonus", "enhancement.success_bonus", "alchemy.transmute_gold_bonus"].includes(effect.kind) ? 0.10 : 0.25
        if (scale.min <= 0 || scale.max > maximum || (scale.integer && (!Number.isSafeInteger(scale.min) || !Number.isSafeInteger(scale.max)))) throw new Error(`fish food scaling exceeds handler bounds: ${id}.${name}`)
        const workMaximum = name === "durationReductionRate" ? (effect.kind === "life_action.focused_harvest" ? .08 : .05) : effect.kind === "life_action.focused_harvest" ? .03 : effect.kind === "life_action.harvest_bonus" ? .05 : effect.kind === "life_action.patient_harvest" ? .11 : effect.kind === "life_action.work_experience" ? .15 : undefined
        if (workMaximum !== undefined && (scale.max > workMaximum || (name === "durationPenaltyRate" && (scale.min !== .04 || scale.max !== .04)) || (effect.kind === "life_action.patient_harvest" && name === "chance" && scale.min < .08))) throw new Error(`fish food work scaling exceeds handler bounds: ${id}.${name}`)
      }
      if (row.legacyEffectIds !== undefined && (!Array.isArray(row.legacyEffectIds) || new Set(row.legacyEffectIds).size !== row.legacyEffectIds.length)) throw new Error(`invalid legacy fish effect list: ${id}`)
      for (const legacyID of row.legacyEffectIds || []) {
        const legacy = effectByID.get(legacyID)
        const compatibleCapacity = legacy?.kind === "alchemy.infusion_capacity_floor_tradeoff" && effect.kind === "alchemy.infusion_bulk_capacity_floor_tradeoff"
        if (!legacy || (!compatibleCapacity && legacy.kind !== effect.kind) || legacy.domain !== effect.domain || legacy.lifetime.kind !== effect.lifetime.kind) throw new Error(`invalid legacy fish effect: ${id}.${legacyID}`)
      }
      if (row.replacedEffectIds !== undefined && (!Array.isArray(row.replacedEffectIds) || new Set(row.replacedEffectIds).size !== row.replacedEffectIds.length)) throw new Error(`invalid replaced fish effect list: ${id}`)
      for (const replacedID of row.replacedEffectIds || []) {
        const replaced = effectByID.get(replacedID)
        if (!replaced || replacedID === effect.id || replaced.lifetime.kind !== effect.lifetime.kind || (row.legacyEffectIds || []).includes(replacedID)) throw new Error(`invalid replaced fish effect: ${id}.${replacedID}`)
      }
      if (row.lifetimeScaling !== undefined) {
const field = kind === "grilled_fish" ? "charges" : "durationSeconds", scale = row.lifetimeScaling?.[field], base = effect.lifetime[field];
const limit = field === "charges" ? effect.stacking.maxCharges : effect.stacking.maxDurationSeconds;
if (Object.keys(row.lifetimeScaling).join() !== field || !scale || scale.curve !== "bounded_growth" || scale.inverse || scale.integer || [scale.min, scale.max, scale.maxBeforeMutation].some(x => !Number.isSafeInteger(x) || x <= 0) || scale.min !== base || scale.maxBeforeMutation < base || scale.maxBeforeMutation > scale.max || scale.max > limit || scale.max > (field === "charges" ? 1000 : 4000) || [scale.length,scale.weight,scale.traits].some(x => !Number.isFinite(x) || x < 0) || Math.abs(scale.length+scale.weight-1)>1e-9 || scale.traits>.2 || Object.keys(scale).some(key => !["curve","length","weight","traits","min","max","maxBeforeMutation"].includes(key))) throw new Error("invalid fish food lifetime scaling: "+id);
}
if (effect.kind === "enchantment.high_rarity_weight" && (row.lifetimeScaling || Object.keys(row.mutationLifetimeBonuses || {}).length)) throw new Error("enchantment food must stay one use");
      const bonuses = row.mutationLifetimeBonuses ?? (kind === "grilled_fish" ? { prismatic: { charges: 1 } } : { rainbow: { durationSeconds: 900 } })
      const field = kind === "grilled_fish" ? "charges" : "durationSeconds"
      const limit = kind === "grilled_fish" ? (workMeal ? 10000 : 1000) : 86400
      if (!bonuses || typeof bonuses !== "object" || Array.isArray(bonuses)) throw new Error(`invalid fish food lifetime bonuses: ${id}`)
      let totalBonus = 0
      for (const [mutation, bonus] of Object.entries(bonuses)) {
        if (!["shimmering", "prismatic", "rainbow"].includes(mutation) || !bonus || Object.keys(bonus).length !== 1 || !Number.isSafeInteger(bonus[field]) || bonus[field] < 0 || bonus[field] > limit) throw new Error(`invalid fish food mutation bonus: ${id}.${mutation}`)
        totalBonus += bonus[field]
      }
      const maximum = kind === "grilled_fish" ? effect.stacking.maxCharges : effect.stacking.maxDurationSeconds
      if (!effect.stacking.exclusive || effect.lifetime[field] + totalBonus > Math.min(maximum, limit)) throw new Error(`invalid fish food serving: ${id}`)
    }

    if (!Array.isArray(row.fishIngredients) || row.fishIngredients.length < 1 || row.fishIngredients.length > 8) {
      throw new Error(`fish_food_recipes must contain at least one fish species: ${id}`)
    }
    const recipeSpecies = new Set()
    let totalFish = 0
    for (const ingredient of row.fishIngredients) {
      const speciesID = String(ingredient?.speciesId || "").trim()
      const quantity = Number(ingredient?.quantity)
      if (!speciesIDs.has(speciesID) || recipeSpecies.has(speciesID) || !Number.isSafeInteger(quantity) || quantity <= 0 || quantity > 8) {
        throw new Error(`fish_food_recipes fish ingredient is invalid: ${id}.${speciesID}`)
      }
      recipeSpecies.add(speciesID)
      totalFish += quantity
    }
    if (totalFish > 16) throw new Error(`fish_food_recipes total fish quantity is too large: ${id}`)
  }
  const pairedRecipes = new Map()
  for (const row of rows.filter((recipe) => recipe.qualityRulesetVersion === "fish-food-quality-v2")) {
    const effect = effectByID.get(itemByID.get(row.outputItemId).useOperationEffectId)
    const pairKey = effect.stacking.key + ":" + effect.kind
    const pair = pairedRecipes.get(pairKey) || []
    pair.push({ recipe: row, effect })
    pairedRecipes.set(pairKey, pair)
  }
  const canonicalIngredients = (recipe) => recipe.fishIngredients.map((row) => `${row.speciesId}:${row.quantity}`).sort().join("|")
  for (const [key, pair] of pairedRecipes) {
    if (["alchemy.transmute_duration","enchantment.high_rarity_weight"].includes(pair[0].effect.kind)) {
      if (pair.length !== 1 || pair[0].recipe.kind !== "grilled_fish") throw new Error("single-serving recipe mismatch: "+key)
      continue
    }
    if (pair.length !== 2 || new Set(pair.map(({ recipe }) => recipe.kind)).size !== 2 || pair[0].effect.kind !== pair[1].effect.kind || canonicalIngredients(pair[0].recipe) !== canonicalIngredients(pair[1].recipe)) {
      throw new Error(`fish food must pair matching grill and soup recipes: ${key}`)
    }
  }
}

function validateFishingTables(waters, spots, species, tackle, infusionProfiles, items, equipmentTemplates, actions, attributes) {
  const locales = ["zh-CN", "zh-TW", "en-US", "ja", "ko", "ru"]
  const fishingMethods = new Set(["float", "bottom", "lure"])
  const fishingLayers = new Set(["surface", "midwater", "near_bottom"])
  if (!Array.isArray(waters) || waters.length !== 3) throw new Error("fishing_waters must contain exactly 3 waters")
  if (!Array.isArray(spots) || spots.length !== 12) throw new Error("fishing_spots must contain exactly 12 spots")
  if (!Array.isArray(species) || species.length !== 24) throw new Error("fishing_species must contain exactly 24 species")
  const itemIDs = new Set((items || []).map((row) => String(row?.id || "").trim()).filter(Boolean))
  const waterIDs = new Set()
  for (const water of waters) {
    const id = String(water?.id || "").trim()
    if (!id || waterIDs.has(id)) throw new Error(`fishing_waters.id invalid or duplicate: ${id}`)
    if (Object.prototype.hasOwnProperty.call(water, "sceneWide") || Object.prototype.hasOwnProperty.call(water, "sceneTall")) {
      throw new Error(`fishing_waters must not contain scene image fields: ${id}`)
    }
    waterIDs.add(id)
    for (const locale of locales) {
      if (!String(water?.names?.[locale] || "").trim() || !String(water?.descriptions?.[locale] || "").trim()) {
        throw new Error(`fishing_waters localization missing: ${id}.${locale}`)
      }
    }
  }
  const speciesIDs = new Set()
  const speciesCountByWater = new Map()
  const fantasyCountByWater = new Map()
  for (const fish of species) {
    const id = String(fish?.id || "").trim()
    const waterID = String(fish?.waterId || "").trim()
    if (!id || speciesIDs.has(id)) throw new Error(`fishing_species.id invalid or duplicate: ${id}`)
    if (!waterIDs.has(waterID)) throw new Error(`fishing_species.waterId not found: ${id}.${waterID}`)
    speciesIDs.add(id)
    speciesCountByWater.set(waterID, (speciesCountByWater.get(waterID) || 0) + 1)
    if (fish.fantasy) fantasyCountByWater.set(waterID, (fantasyCountByWater.get(waterID) || 0) + 1)
    if (!(Number(fish.minLengthCm) > 0) || !(Number(fish.maxLengthCm) > Number(fish.minLengthCm))) throw new Error(`fishing_species length range invalid: ${id}`)
    if (!(Number(fish.maturityLengthCm) > Number(fish.minLengthCm)) || !(Number(fish.maturityLengthCm) < Number(fish.maxLengthCm))) {
      throw new Error(`fishing_species maturity length invalid: ${id}`)
    }
    if (!Number.isSafeInteger(Number(fish.maturityAgeDays)) || !(Number(fish.maturityAgeDays) > 0) || !Number.isSafeInteger(Number(fish.lifespanDays)) || !(Number(fish.lifespanDays) > Number(fish.maturityAgeDays))) {
      throw new Error(`fishing_species life history invalid: ${id}`)
    }
    if (Object.prototype.hasOwnProperty.call(fish, "baseValuePerKg")) throw new Error(`fishing_species legacy per-kg value is not supported: ${id}`)
    if (!Number.isSafeInteger(Number(fish.baseValue)) || !(Number(fish.baseValue) > 0) || !(Number(fish.fightPower) > 0)) {
      throw new Error(`fishing_species economy or fight power invalid: ${id}`)
    }
    if (!(Number(fish.castingChallenge) >= 0) || !(Number(fish.hookingChallenge) >= 0)) throw new Error(`fishing_species challenge invalid: ${id}`)
    for (const field of ["mainLineWearMultiplier", "leaderWearMultiplier"]) {
      const value = Number(fish[field])
      if (!(value > 0) || value > 2) throw new Error(`fishing_species ${field} invalid: ${id}`)
    }
    if (!Array.isArray(fish.preferredMethods) || fish.preferredMethods.length === 0 || fish.preferredMethods.some((method) => !fishingMethods.has(method))) {
      throw new Error(`fishing_species preferredMethods invalid: ${id}`)
    }
    if (!Array.isArray(fish.preferredLayers) || fish.preferredLayers.length === 0 || fish.preferredLayers.some((layer) => !fishingLayers.has(layer)) || new Set(fish.preferredLayers).size !== fish.preferredLayers.length) {
      throw new Error(`fishing_species preferredLayers invalid: ${id}`)
    }
    if (!Array.isArray(fish.activeHours) || fish.activeHours.length === 0 || fish.activeHours.some((hour) => !Number.isInteger(hour) || hour < 0 || hour > 23)) {
      throw new Error(`fishing_species activeHours invalid: ${id}`)
    }
    if (fish.fantasy && !String(fish.windowWeather || "").trim()) throw new Error(`fishing fantasy species windowWeather missing: ${id}`)
    if (!String(fish.iconPath || "").startsWith("/assets/fishing/fish/")) throw new Error(`fishing_species iconPath invalid: ${id}`)
    if (!fs.existsSync(path.join(projectRoot, "web/public", String(fish.iconPath).replace(/^\/+/, "")))) throw new Error(`fishing_species icon asset missing: ${id}`)
    for (const locale of locales) {
      if (!String(fish?.names?.[locale] || "").trim()) throw new Error(`fishing_species localization missing: ${id}.${locale}`)
    }
  }
  for (const waterID of waterIDs) {
    if (speciesCountByWater.get(waterID) !== 8 || fantasyCountByWater.get(waterID) !== 2) {
      throw new Error(`fishing water must contain 8 species including 2 fantasy species: ${waterID}`)
    }
  }
  const spotIDs = new Set()
  const spotCountByWater = new Map()
  for (const spot of spots) {
    const id = String(spot?.id || "").trim()
    const waterID = String(spot?.waterId || "").trim()
    if (!id || spotIDs.has(id)) throw new Error(`fishing_spots.id invalid or duplicate: ${id}`)
    if (!waterIDs.has(waterID)) throw new Error(`fishing_spots.waterId not found: ${id}.${waterID}`)
    spotIDs.add(id)
    spotCountByWater.set(waterID, (spotCountByWater.get(waterID) || 0) + 1)
    if (!Array.isArray(spot.methods) || spot.methods.length === 0 || spot.methods.some((method) => !fishingMethods.has(method))) throw new Error(`fishing_spots.methods invalid: ${id}`)
    if (Object.keys(spot.speciesWeights || {}).length === 0) throw new Error(`fishing_spots speciesWeights missing: ${id}`)
    for (const [speciesID, weight] of Object.entries(spot.speciesWeights || {})) {
      if (!speciesIDs.has(speciesID) || !(Number(weight) > 0)) throw new Error(`fishing_spots species weight invalid: ${id}.${speciesID}`)
      const fish = species.find((entry) => entry.id === speciesID)
      if (fish?.waterId !== waterID) throw new Error(`fishing_spots species belongs to another water: ${id}.${speciesID}`)
    }
    for (const locale of locales) {
      if (!String(spot?.names?.[locale] || "").trim()) throw new Error(`fishing_spots localization missing: ${id}.${locale}`)
    }
  }
  for (const waterID of waterIDs) {
    if (spotCountByWater.get(waterID) !== 4) throw new Error(`fishing water must contain 4 spots: ${waterID}`)
  }
  const tackleItems = new Set()
  const tackleSlots = new Set(["rod", "reel", "main_line", "leader", "hook", "float_or_sinker", "lure", "bait"])
  const itemByID = new Map((items || []).map((row) => [String(row?.id || "").trim(), row]))
  const equipmentByID = new Map((equipmentTemplates || []).map((row) => [String(row?.id || "").trim(), row]))
  for (const row of tackle || []) {
    const itemID = String(row?.itemId || "").trim()
    if (!itemID || tackleItems.has(itemID) || !itemIDs.has(itemID)) throw new Error(`fishing_tackle.itemId invalid: ${itemID}`)
    const item = itemByID.get(itemID)
    if (!Array.isArray(item?.type) || !item.type.includes("fishing_gear")) {
      throw new Error(`fishing_tackle item must use fishing_gear type: ${itemID}`)
    }
    const iconID = String(item?.iconId || "").trim()
    if (iconID !== `item.${itemID}` || !fs.existsSync(path.join(webIconsDir, "item", `${itemID}.png`))) {
      throw new Error(`fishing_tackle icon missing or mismatched: ${itemID}`)
    }
    const slot = String(row?.slot || "").trim()
    if (!tackleSlots.has(slot)) throw new Error(`fishing_tackle slot invalid: ${itemID}`)
    if (!Array.isArray(row.methods) || row.methods.length === 0 || row.methods.some((method) => !fishingMethods.has(method))) {
      throw new Error(`fishing_tackle methods invalid: ${itemID}`)
    }
    for (const field of ["control", "sensitivity", "casting", "hooking", "efficiency", "maxLoadKg", "wearPerFight", "biteMultiplier", "mutationChanceMultiplier"]) {
      if (row[field] !== undefined && !Number.isFinite(Number(row[field]))) throw new Error(`fishing_tackle ${field} invalid: ${itemID}`)
    }
    if (row.mutationChanceMultiplier !== undefined) {
      const multiplier = Number(row.mutationChanceMultiplier)
      if (slot !== "bait" || !(multiplier > 0) || multiplier > 100) {
        throw new Error(`fishing_tackle mutationChanceMultiplier invalid: ${itemID}`)
      }
    }
    const wearPerFight = Number(row.wearPerFight)
    if (slot === "main_line" || slot === "leader") {
      if (!(wearPerFight > 0) || wearPerFight > 0.25) throw new Error(`fishing_tackle wearPerFight invalid: ${itemID}`)
    } else if (row.wearPerFight !== undefined) {
      throw new Error(`fishing_tackle wearPerFight unsupported for slot: ${itemID}.${slot}`)
    }
    for (const waterID of Object.keys(row.waterBiteModifiers || {})) {
      if (!waterIDs.has(waterID) || !Number.isFinite(Number(row.waterBiteModifiers[waterID]))) throw new Error(`fishing_tackle water modifier invalid: ${itemID}.${waterID}`)
    }
    for (const speciesID of Object.keys(row.speciesWeightModifiers || {})) {
      if (!speciesIDs.has(speciesID) || !Number.isFinite(Number(row.speciesWeightModifiers[speciesID]))) throw new Error(`fishing_tackle species modifier invalid: ${itemID}.${speciesID}`)
    }
    if (slot === "rod" || slot === "reel") {
      const equipment = equipmentByID.get(itemID)
      const expectedSlot = slot === "rod" ? "fishing_rod" : "fishing_reel"
      if (!item?.isEquipment || equipment?.equipType !== "work" || equipment?.slot !== expectedSlot) {
        throw new Error(`fishing equipment template missing or mismatched: ${itemID}`)
      }
    } else if (item?.isEquipment) {
      throw new Error(`fishing terminal tackle must not be equipment: ${itemID}`)
    }
    tackleItems.add(itemID)
  }
  for (const item of items || []) {
    if ((item?.type || []).includes("fishing_gear") && !tackleItems.has(String(item?.id || "").trim())) {
      throw new Error(`fishing_gear item missing tackle definition: ${item?.id || "<unknown>"}`)
    }
  }
  const requiredItems = [
    "basic_fishing_rod", "silverleaf_float_rod", "silkwood_search_rod", "rimebone_deep_rod", "moonwood_precision_rod",
    "basic_spinning_reel", "light_copper_reel", "sealed_drag_reel",
    "braided_main_line", "stretch_mono_line", "cushion_braid_line", "trophy_core_line", "clear_leader", "wire_leader", "shock_leader",
    "fine_fishing_hook", "barbless_hook", "circle_hook", "reed_float", "slip_float", "quill_float", "river_sinker", "distance_sinker",
    "silver_shad_lure", "frost_jig", "earthworm_bait", "sweet_dough_bait", "starripple_bait",
    "champion_current_rod", "tournament_current_rod", "fish_catch", "scaleglow_medium",
  ]
  for (const required of requiredItems) {
    if (!itemIDs.has(required)) throw new Error(`fishing required item missing: ${required}`)
    const item = itemByID.get(required)
    if (String(item?.iconId || "").trim() !== `item.${required}` || !fs.existsSync(path.join(webIconsDir, "item", `${required}.png`))) {
      throw new Error(`fishing required item icon missing or mismatched: ${required}`)
    }
  }
  if (itemByID.get("fish_catch")?.marketListable !== false) {
    throw new Error("caught fish must not be market listable")
  }
  for (const required of requiredItems.filter((id) => !["fish_catch", "scaleglow_medium"].includes(id))) {
    if (!tackleItems.has(required)) throw new Error(`fishing required tackle definition missing: ${required}`)
  }
  const actionByID = new Map((actions || []).map((row) => [String(row?.id || "").trim(), row]))
  const craftedItems = requiredItems.filter((id) => !["champion_current_rod", "tournament_current_rod", "fish_catch", "scaleglow_medium"].includes(id))
  const forgedRodItems = new Set(["basic_fishing_rod", "silverleaf_float_rod", "silkwood_search_rod", "rimebone_deep_rod", "moonwood_precision_rod"])
  for (const itemID of craftedItems) {
    if (itemByID.get(itemID)?.marketListable !== true) {
      throw new Error(`crafted fishing tackle must be market listable: ${itemID}`)
    }
    const actionID = `craft_${itemID}`
    const action = actionByID.get(actionID)
    const expectedMainType = forgedRodItems.has(itemID) ? "forging" : "crafting"
    const expectedSubType = forgedRodItems.has(itemID) ? "forge_fishing_rod" : "craft_fishing_tackle"
    if (!action || String(action.mainType || "").trim() !== expectedMainType || String(action.subType || "").trim() !== expectedSubType || Number(action?.itemReward?.[itemID] || 0) <= 0 || String(action.iconId || "").trim() !== `item.${itemID}`) {
      throw new Error(`fishing crafting recipe missing or invalid: ${actionID}`)
    }
  }
  if (!actionByID.has("fishing.trip")) {
    throw new Error("fishing trip action missing: fishing.trip")
  }
  const attrIDs = new Set((attributes || []).map((row) => String(row?.id || "").trim()).filter(Boolean))
  const profileSpecies = new Set()
  if (!Array.isArray(infusionProfiles) || infusionProfiles.length !== 24) throw new Error("fishing_infusion_profiles must contain exactly 24 profiles")
  for (const profile of infusionProfiles) {
    const speciesID = String(profile?.speciesId || "").trim()
    if (!speciesIDs.has(speciesID) || profileSpecies.has(speciesID)) throw new Error(`fishing infusion species invalid or duplicate: ${speciesID}`)
    if (!["combat", "life"].includes(String(profile?.traitKind || "").trim())) throw new Error(`fishing infusion trait kind invalid: ${speciesID}`)
    if (!attrIDs.has(String(profile?.attrKey || "").trim())) throw new Error(`fishing infusion attribute missing: ${speciesID}.${profile?.attrKey || ""}`)
    if (!(Number(profile?.unitScale) > 0)) throw new Error(`fishing infusion unit scale invalid: ${speciesID}`)
    profileSpecies.add(speciesID)
  }
}

function validateFishingPondSpecies(rows, species) {
  if (!Array.isArray(rows) || rows.length !== species.length) throw new Error("fishing_pond_species must cover every species")
  const speciesIDs = new Set(species.map((row) => String(row?.id || "").trim()).filter(Boolean))
  const speciesByID = new Map(species.map((row) => [String(row?.id || "").trim(), row]))
  const seen = new Set()
  const feedClasses = new Set(["herbal", "insect", "protein", "arcane"])
  for (const row of rows) {
    const speciesID = String(row?.speciesId || "").trim()
    if (!speciesIDs.has(speciesID) || seen.has(speciesID)) throw new Error(`fishing_pond_species species invalid: ${speciesID}`)
    seen.add(speciesID)
    if (!feedClasses.has(String(row?.dietClass || ""))) throw new Error(`fishing_pond_species diet invalid: ${speciesID}`)
    if (!Array.isArray(row.secondaryFeedClasses) || row.secondaryFeedClasses.some((entry) => !feedClasses.has(String(entry)))) {
      throw new Error(`fishing_pond_species secondary diet invalid: ${speciesID}`)
    }
    if (!(Number(row.maturityAgeDays) > 0) || !(Number(row.lifespanDays) > Number(row.maturityAgeDays))) {
      throw new Error(`fishing_pond_species ages invalid: ${speciesID}`)
    }
    const fish = speciesByID.get(speciesID)
    if (Number(row.maturityAgeDays) !== Number(fish?.maturityAgeDays) || Number(row.lifespanDays) !== Number(fish?.lifespanDays)) {
      throw new Error(`fishing_pond_species life history must match fishing_species: ${speciesID}`)
    }
    if (row.predation != null) {
      const maxPreyLengthRatio = Number(row.predation.maxPreyLengthRatio)
      const huntBelowSatiety = Number(row.predation.huntBelowSatiety)
      const dailyHuntChance = Number(row.predation.dailyHuntChance)
      if (!(maxPreyLengthRatio >= 0.1 && maxPreyLengthRatio <= 0.6)) {
        throw new Error(`fishing_pond_species predation size ratio invalid: ${speciesID}`)
      }
      if (!(huntBelowSatiety >= 20 && huntBelowSatiety <= 90)) {
        throw new Error(`fishing_pond_species predation satiety invalid: ${speciesID}`)
      }
      if (!(dailyHuntChance > 0 && dailyHuntChance <= 1)) {
        throw new Error(`fishing_pond_species predation chance invalid: ${speciesID}`)
      }
    }
  }
}

function extractGameDataLocaleObject(filePath) {
  if (!fs.existsSync(filePath)) throw new Error(`fishing locale file missing: ${filePath}`)
  const content = fs.readFileSync(filePath, "utf8")
  const prefix = "export default"
  const index = content.indexOf(prefix)
  if (index < 0) throw new Error(`fishing locale export missing: ${filePath}`)
  let source = content.slice(index + prefix.length).trim()
  if (source.endsWith(";")) source = source.slice(0, -1)
  const parsed = vm.runInNewContext(`(${source})`)
  if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) throw new Error(`fishing locale object invalid: ${filePath}`)
  return parsed
}

function validateFishingLocaleCoverage() {
  const itemIDs = [
    "basic_fishing_rod", "silverleaf_float_rod", "silkwood_search_rod", "rimebone_deep_rod", "moonwood_precision_rod",
    "basic_spinning_reel", "light_copper_reel", "sealed_drag_reel",
    "braided_main_line", "stretch_mono_line", "cushion_braid_line", "trophy_core_line", "clear_leader", "wire_leader", "shock_leader",
    "fine_fishing_hook", "barbless_hook", "circle_hook", "reed_float", "slip_float", "quill_float", "river_sinker", "distance_sinker",
    "silver_shad_lure", "frost_jig", "earthworm_bait", "sweet_dough_bait", "starripple_bait",
    "champion_current_rod", "tournament_current_rod", "fish_catch", "scaleglow_medium",
  ]
  const actionIDs = ["fishing.trip", ...itemIDs.filter((id) => !["champion_current_rod", "tournament_current_rod", "fish_catch", "scaleglow_medium"].includes(id)).map((id) => `craft_${id}`)]
  const attrIDs = ["fishing", "fishing_control", "fishing_sensitivity", "fishing_efficiency", "fishing_power", "fishing_casting", "fishing_hooking"]
  const affixIDs = ["fishing_control_minor", "fishing_sensitivity_minor", "fishing_efficiency_minor", "fishing_casting_minor", "fishing_hooking_minor"]
  for (const [locale, filePath] of Object.entries(gameDataLocalePaths)) {
    const data = extractGameDataLocaleObject(filePath)
    for (const itemID of itemIDs) {
      const entry = data?.item?.[itemID]
      if (!String(entry?.name || "").trim() || !String(entry?.desc || "").trim() || !String(entry?.effect || "").trim()) {
        throw new Error(`fishing item locale missing: ${locale}.item.${itemID}`)
      }
    }
    for (const actionID of actionIDs) {
      const entry = data?.action?.[actionID]
      if (!String(entry?.name || "").trim() || !String(entry?.desc || "").trim()) {
        throw new Error(`fishing action locale missing: ${locale}.action.${actionID}`)
      }
    }
    for (const attrID of attrIDs) {
      const entry = data?.attr?.[attrID]
      if (!String(entry?.name || "").trim() || !String(entry?.desc || "").trim()) {
        throw new Error(`fishing attribute locale missing: ${locale}.attr.${attrID}`)
      }
    }
    for (const affixID of affixIDs) {
      const entry = data?.affix?.[affixID]
      if (!String(entry?.name || "").trim() || !String(entry?.desc || "").trim()) {
        throw new Error(`fishing affix locale missing: ${locale}.affix.${affixID}`)
      }
    }
    if (!String(data?.action_main_type?.fishing?.name || "").trim() || !String(data?.action_main_type?.fishing?.desc || "").trim()) {
      throw new Error(`fishing action main type locale missing: ${locale}`)
    }
    if (!String(data?.action_sub_type?.fishing_trip?.name || "").trim() || !String(data?.action_sub_type?.fishing_trip?.desc || "").trim()) {
      throw new Error(`fishing action sub type locale missing: ${locale}`)
    }
    for (const subTypeID of ["craft_fishing_tackle", "forge_fishing_rod"]) {
      if (!String(data?.action_sub_type?.[subTypeID]?.name || "").trim() || !String(data?.action_sub_type?.[subTypeID]?.desc || "").trim()) {
        throw new Error(`fishing action sub type locale missing: ${locale}.${subTypeID}`)
      }
    }
    if (!String(data?.title_prefix?.fishing_champion?.name || "").trim()) throw new Error(`fishing champion title locale missing: ${locale}`)
  }
}

function validateAchievements(categories, achievements, items, actions, enemies, commissions, attributes, rarityDefinitions, titlePrefixes, titleSuffixes, fishingSpecies) {
  const categoryIDs = new Set((categories || []).map((row) => String(row?.id || "").trim()).filter(Boolean))
  const achievementIDs = new Set()
  const itemIDs = new Set((items || []).map((row) => String(row?.id || "").trim()).filter(Boolean))
  const actionIDs = new Set((actions || []).map((row) => String(row?.id || "").trim()).filter(Boolean))
  const enemyIDs = new Set((enemies || []).map((row) => String(row?.id || "").trim()).filter(Boolean))
  const commissionIDs = new Set((commissions || []).map((row) => String(row?.id || "").trim()).filter(Boolean))
  const attrIDs = new Set((attributes || []).map((row) => String(row?.id || "").trim()).filter(Boolean))
  const rarityIDs = new Set((rarityDefinitions || []).map((row) => String(row?.id || "").trim()).filter(Boolean))
  const titlePrefixIDs = new Set((titlePrefixes || []).map((row) => String(row?.id || "").trim()).filter(Boolean))
  const titleSuffixIDs = new Set((titleSuffixes || []).map((row) => String(row?.id || "").trim()).filter(Boolean))
  const fishingSpeciesIDs = new Set((fishingSpecies || []).map((row) => String(row?.id || "").trim()).filter(Boolean))

  for (const row of categories || []) {
    const id = String(row?.id || "").trim()
    if (!id) throw new Error("achievement_categories.id invalid")
    const expectedNameKey = `achievement_category.${id}.name`
    if (String(row?.nameKey || "").trim() !== expectedNameKey) {
      throw new Error(`achievement_categories.nameKey invalid for ${id}; expected ${expectedNameKey}`)
    }
  }

  const expandedAchievements = []
  for (const row of achievements || []) {
    const id = String(row?.id || "").trim()
    if (!id) throw new Error("achievements.id invalid")
    if (!categoryIDs.has(String(row?.categoryId || "").trim())) {
      throw new Error(`achievements.categoryId not found: ${id}.${row?.categoryId || ""}`)
    }
    const rarityID = String(row?.rarity || "").trim()
    if (!rarityID) throw new Error(`achievements.rarity invalid: ${id}`)
    if (rarityIDs.size > 0 && !rarityIDs.has(rarityID)) throw new Error(`achievements.rarity not found: ${id}.${rarityID}`)
    const stages = Array.isArray(row?.stages) ? row.stages : []
    if (stages.length > 0) {
      const stageProgressMode = String(row?.stageProgressMode || "").trim()
      if (stageProgressMode && !["sequential", "cumulative"].includes(stageProgressMode)) {
        throw new Error(`achievements.stageProgressMode invalid: ${id}.${stageProgressMode}`)
      }
      stages.forEach((stage, index) => {
        const stageID = `${id}__stage_${index + 1}`
        if (achievementIDs.has(stageID)) throw new Error(`achievements.id duplicate: ${stageID}`)
        achievementIDs.add(stageID)
        expandedAchievements.push(expandAchievementStageForValidation(row, stage, stageID, index))
      })
    } else {
      if (achievementIDs.has(id)) throw new Error(`achievements.id duplicate: ${id}`)
      achievementIDs.add(id)
      expandedAchievements.push(row)
    }
  }

  const prereqMap = new Map()
  for (const row of expandedAchievements) {
    const id = String(row?.id || "").trim()
    const prereqs = Array.isArray(row?.prereqAchievementIds)
      ? row.prereqAchievementIds.map((entry) => String(entry || "").trim()).filter(Boolean)
      : []
    for (const prereqID of prereqs) {
      if (!achievementIDs.has(prereqID)) throw new Error(`achievements.prereqAchievementIds not found: ${id}.${prereqID}`)
    }
    prereqMap.set(id, prereqs)
  }
  const visiting = new Set()
  const visited = new Set()
  const visit = (id) => {
    if (visited.has(id)) return
    if (visiting.has(id)) throw new Error(`achievements.prereqAchievementIds cycle: ${id}`)
    visiting.add(id)
    for (const prereqID of prereqMap.get(id) || []) visit(prereqID)
    visiting.delete(id)
    visited.add(id)
  }
  for (const id of achievementIDs) visit(id)

  for (const row of expandedAchievements) {
    const id = String(row?.id || "").trim()
    const condition = row?.condition
    if (!condition || typeof condition !== "object" || Array.isArray(condition)) throw new Error(`achievements.condition invalid: ${id}`)
    const kind = String(condition.kind || "").trim()
    const targetRaw = kind === "player_attr_reached"
      ? (condition.targetValue ?? condition.value ?? condition.count)
      : (kind === "own_item" || kind === "obtain_item" ? condition.quantity : condition.count)
    const count = Number(targetRaw || 0)
    if (!Number.isFinite(count) || count <= 0 || !Number.isInteger(count)) throw new Error(`achievements.condition.count invalid: ${id}`)
    switch (kind) {
      case "kill_enemy":
        if (!enemyIDs.has(String(condition.enemyId || "").trim())) throw new Error(`achievements.condition.enemyId not found: ${id}`)
        break
      case "complete_action":
        if (!actionIDs.has(String(condition.actionId || "").trim())) throw new Error(`achievements.condition.actionId not found: ${id}`)
        if (condition.actorType !== undefined && !["", "any", "player", "kitty"].includes(String(condition.actorType || "").trim())) {
          throw new Error(`achievements.condition.actorType invalid: ${id}.${condition.actorType}`)
        }
        break
      case "own_kitty":
      case "complete_commission":
      case "complete_rumor_commission":
      case "enchant_count":
        break
      case "own_item":
      case "obtain_item":
        if (!itemIDs.has(String(condition.itemId || "").trim())) throw new Error(`achievements.condition.itemId not found: ${id}`)
        break
      case "enhance_count":
        if (condition.itemId !== undefined && String(condition.itemId || "").trim() && !itemIDs.has(String(condition.itemId || "").trim())) {
          throw new Error(`achievements.condition.itemId not found: ${id}`)
        }
        break
      case "complete_specific_commission":
        if (!commissionIDs.has(String(condition.commissionId || "").trim())) throw new Error(`achievements.condition.commissionId not found: ${id}`)
        break
      case "player_attr_reached":
        if (!attrIDs.has(String(condition.attrKey || "").trim())) throw new Error(`achievements.condition.attrKey not found: ${id}`)
        if (condition.valueSource !== undefined && !["", "base_level", "final_value", "skill_point_level"].includes(String(condition.valueSource || "").trim())) {
          throw new Error(`achievements.condition.valueSource invalid: ${id}.${condition.valueSource}`)
        }
        break
      case "catch_fish":
        if (!fishingSpeciesIDs.has(String(condition.speciesId || "").trim())) throw new Error(`achievements.condition.speciesId not found: ${id}`)
        break
      default:
        throw new Error(`achievements.condition.kind invalid: ${id}.${kind}`)
    }

    const rewards = Array.isArray(row?.rewards) ? row.rewards : []
    for (const reward of rewards) {
      const rewardKind = String(reward?.kind || "").trim()
      switch (rewardKind) {
        case "item": {
          const itemID = String(reward?.itemId || "").trim()
          const quantity = Number(reward?.quantity || 0)
          if (!itemIDs.has(itemID)) throw new Error(`achievements.rewards.itemId not found: ${id}.${itemID}`)
          if (!Number.isFinite(quantity) || quantity <= 0 || !Number.isInteger(quantity)) throw new Error(`achievements.rewards.quantity invalid: ${id}.${itemID}`)
          break
        }
        case "wardrobe": {
          const category = String(reward?.category || "").trim()
          const itemID = String(reward?.itemId || "").trim()
          if (category === "title_prefix") {
            if (!titlePrefixIDs.has(itemID)) throw new Error(`achievements.rewards.title_prefix not found: ${id}.${itemID}`)
          } else if (category === "title_suffix") {
            if (!titleSuffixIDs.has(itemID)) throw new Error(`achievements.rewards.title_suffix not found: ${id}.${itemID}`)
          } else {
            throw new Error(`achievements.rewards.wardrobe.category invalid: ${id}.${category}`)
          }
          break
        }
        default:
          throw new Error(`achievements.rewards.kind invalid: ${id}.${rewardKind}`)
      }
    }
  }
}

function expandAchievementStageForValidation(row, stage, stageID, index) {
  const baseID = String(row?.id || "").trim()
  const prereqs = Array.isArray(row?.prereqAchievementIds)
    ? row.prereqAchievementIds.map((entry) => String(entry || "").trim()).filter(Boolean)
    : []
  if (index > 0) prereqs.push(`${baseID}__stage_${index}`)
  return {
    ...row,
    ...stage,
    id: stageID,
    categoryId: row.categoryId,
    prereqAchievementIds: prereqs,
    condition: stage?.condition || row?.condition,
    rewards: Array.isArray(stage?.rewards) ? stage.rewards : row?.rewards,
    stages: undefined
  }
}

function isHexColor(raw) {
  return typeof raw === "string" && /^#(?:[0-9a-fA-F]{3}|[0-9a-fA-F]{6})$/.test(raw.trim())
}

function normalizeTargetSeekStrategy(raw, fallback = "front_to_rear") {
  const value = String(raw || "").trim().toLowerCase()
  if (value === "rear_to_front" || value === "front_to_rear" || value === "random_alive" || value === "none") {
    return value
  }
  return fallback
}

function normalizeTargetScope(raw, fallback = "enemy") {
  const value = String(raw || "").trim().toLowerCase()
  if (value === "ally" || value === "enemy" || value === "all" || value === "self") {
    return value
  }
  return fallback
}

function normalizeSelectRule(raw, fallback = "random") {
  const value = String(raw || "").trim().toLowerCase()
  if (
    value === "hp_max" ||
    value === "hp_min" ||
    value === "hp_percent_max" ||
    value === "hp_percent_min" ||
    value === "first" ||
    value === "boss_first" ||
    value === "attr_max" ||
    value === "attr_min" ||
    value === "random" ||
    value === "self"
  ) {
    return value
  }
  if (value === "self_first") return "self"
  return fallback
}

function parseTargetPositions(raw, fallbackDirection = "vanguard") {
  const normalizedFallback = String(fallbackDirection || "").trim().toLowerCase() || "vanguard"
  const rawRows = Array.isArray(raw)
    ? raw
    : typeof raw === "string" && raw.trim() !== ""
      ? raw.split(",")
      : []
  const valid = new Set(["vanguard", "midguard", "rearguard"])
  const out = []
  for (const row of rawRows) {
    const value = String(row || "").trim().toLowerCase()
    if (!valid.has(value)) continue
    if (out.includes(value)) continue
    out.push(value)
  }
  if (out.length > 0) return out
  return [valid.has(normalizedFallback) ? normalizedFallback : "vanguard"]
}

function parseBasicAttackFormulaTerms(raw) {
  if (Array.isArray(raw)) {
    return raw
      .map((term) => {
        if (!term || typeof term !== "object" || Array.isArray(term)) return null
        const attrKey = parseOptionalString(term.attrKey) || ""
        const coefficient = parseNumber(term.coefficient, Number.NaN)
        if (!attrKey || !Number.isFinite(coefficient)) return null
        return { attrKey, coefficient }
      })
      .filter(Boolean)
  }
  if (typeof raw === "string" && raw.trim() !== "") {
    try {
      return parseBasicAttackFormulaTerms(JSON.parse(raw))
    } catch {
      return []
    }
  }
  return []
}

function parseSkillFormulaTerms(raw) {
  if (Array.isArray(raw)) {
    return raw
      .map((term) => {
        if (!term || typeof term !== "object" || Array.isArray(term)) return null
        const attrKey = parseOptionalString(term.attrKey) || ""
        const coefficient = parseNumber(term.coefficient, Number.NaN)
        const perSkillLevel = parseNumber(term.perSkillLevel, 0)
        if (!attrKey || !Number.isFinite(coefficient) || !Number.isFinite(perSkillLevel)) return null
        return { attrKey, coefficient, perSkillLevel }
      })
      .filter(Boolean)
  }
  if (typeof raw === "string" && raw.trim() !== "") {
    try {
      return parseSkillFormulaTerms(JSON.parse(raw))
    } catch {
      return []
    }
  }
  return []
}

export function parseSkillTargeting(raw, fallback = {}) {
  const fallbackDirection = String(fallback.targetDirection || "vanguard").trim().toLowerCase()
  const fallbackSeek = normalizeTargetSeekStrategy(fallback.targetSeekStrategy, "front_to_rear")
  const fallbackBaseCount = Math.max(1, Math.round(parseNumber(fallback.targetBaseCount, 1)))
  const fallbackMaxRaw = parseNumber(fallback.targetMax, Number.NaN)
  const fallbackMax = Number.isFinite(fallbackMaxRaw) && Number.isInteger(fallbackMaxRaw) && fallbackMaxRaw > 0
    ? fallbackMaxRaw
    : undefined
  const fallbackFormula = parseSkillFormulaTerms(fallback.targetFormula)
  const source = raw && typeof raw === "object" && !Array.isArray(raw) ? raw : {}
  const selectRule = normalizeSelectRule(source.selectRule, "random")
  const selectAttrKey = parseOptionalString(source.selectAttrKey) || parseOptionalString(source.attrKey) || ""
  const sourceMaxRaw = parseNumber(source?.targetCount?.max, Number.NaN)
  const sourceMax = Number.isFinite(sourceMaxRaw) && Number.isInteger(sourceMaxRaw) && sourceMaxRaw > 0
    ? sourceMaxRaw
    : undefined
  const parseTargetPredicates = (value) => {
    if (!Array.isArray(value)) return []
    const predicates = []
    for (const item of value) {
      if (!item || typeof item !== "object" || Array.isArray(item)) continue
      const kind = String(item.kind || "").trim().toLowerCase()
      if (kind === "compare_attr") {
        const actorAttr = parseOptionalString(item.actorAttr) || parseOptionalString(item.actor_attr) || ""
        const targetAttr = parseOptionalString(item.targetAttr) || parseOptionalString(item.target_attr) || ""
        const operator = String(item.operator || item.op || ">").trim().toLowerCase()
        if (!actorAttr || !targetAttr) continue
        predicates.push({ kind, actorAttr, targetAttr, operator })
        continue
      }
      if (kind === "exclude_enemy_type") {
        const enemyType = String(item.enemyType || item.enemy_type || "").trim().toLowerCase()
        if (!enemyType) continue
        predicates.push({ kind, enemyType })
        continue
      }
      if (kind === "require_target_has_status" || kind === "exclude_target_has_status") {
        const statusId = parseOptionalString(item.statusId) || parseOptionalString(item.status_id) || parseOptionalString(item.buffId) || parseOptionalString(item.buff_id) || ""
        if (!statusId) continue
        predicates.push({ kind, statusId })
        continue
      }
      if (kind === "target_hp_not_full") {
        predicates.push({ kind })
        continue
      }
    }
    return predicates
  }
  return {
    targetPositions: parseTargetPositions(source.targetPositions, fallbackDirection),
    secondarySeek: normalizeTargetSeekStrategy(source.secondarySeek, fallbackSeek),
    targetScope: normalizeTargetScope(source.targetScope, "enemy"),
    selectRule,
    selectAttrKey: selectRule === "attr_max" || selectRule === "attr_min" ? selectAttrKey : "",
    fallbackSelfIfEmpty: parseBoolean(source.fallbackSelfIfEmpty, false),
    excludeSelf: parseBoolean(source.excludeSelf ?? source.exclude_self, false),
    ...(Object.hasOwn(source, "ignoreTargetPriority")
      ? { ignoreTargetPriority: source.ignoreTargetPriority }
      : {}),
    targetCount: {
      base: Math.max(1, Math.round(parseNumber(source?.targetCount?.base, fallbackBaseCount))),
      formula: parseSkillFormulaTerms(source?.targetCount?.formula ?? fallbackFormula),
      ...(sourceMax !== undefined ? { max: sourceMax } : (fallbackMax !== undefined ? { max: fallbackMax } : {}))
    },
    targetPredicates: parseTargetPredicates(source.targetPredicates)
  }
}

function parseSkillAmountFormula(raw, fallbackTerms = []) {
  const source = raw && typeof raw === "object" && !Array.isArray(raw) ? raw : {}
  return {
    baseAdd: parseNumber(source.baseAdd, 0),
    terms: parseSkillFormulaTerms(source.terms ?? fallbackTerms)
  }
}

const ENEMY_ONLY_DAMAGE_REDUCTION_PIERCE_FIELDS = Object.freeze([
  "enemyOnlyDamageReductionPierce",
  "enemyOnlyDamageReductionPierceTargetStatuses"
])

export function enemyOnlyDamageReductionPierceFields(raw) {
  if (!raw || typeof raw !== "object" || Array.isArray(raw)) return []
  return ENEMY_ONLY_DAMAGE_REDUCTION_PIERCE_FIELDS.filter((field) => Object.hasOwn(raw, field))
}

const HIT_DAMAGE_SCALING_KIND = "powered_exponential_decay"
const HIT_DAMAGE_SCALING_FIELDS = Object.freeze([
  "kind",
  "secondHitMultiplier",
  "curvePower"
])

export function validateHitDamageScaling(damageSpec, path = "damageSpec", ownerID = "") {
  if (!damageSpec || typeof damageSpec !== "object" || Array.isArray(damageSpec) || !Object.hasOwn(damageSpec, "hitDamageScaling")) {
    return
  }
  const scaling = damageSpec.hitDamageScaling
  const label = `${path}.hitDamageScaling`
  const suffix = ownerID ? `: ${ownerID}` : ""
  if (!scaling || typeof scaling !== "object" || Array.isArray(scaling)) {
    throw new Error(`${label} invalid${suffix}`)
  }
  const unsupportedField = Object.keys(scaling).find((field) => !HIT_DAMAGE_SCALING_FIELDS.includes(field))
  if (unsupportedField) {
    throw new Error(`${label}.${unsupportedField} unsupported${suffix}`)
  }
  if (scaling.kind !== HIT_DAMAGE_SCALING_KIND) {
    throw new Error(`${label}.kind invalid${suffix}`)
  }
  if (
    typeof scaling.secondHitMultiplier !== "number" ||
    !Number.isFinite(scaling.secondHitMultiplier) ||
    scaling.secondHitMultiplier <= 0 ||
    scaling.secondHitMultiplier >= 1
  ) {
    throw new Error(`${label}.secondHitMultiplier invalid${suffix}`)
  }
  if (
    typeof scaling.curvePower !== "number" ||
    !Number.isFinite(scaling.curvePower) ||
    scaling.curvePower < 1 ||
    scaling.curvePower > 3
  ) {
    throw new Error(`${label}.curvePower invalid${suffix}`)
  }
}

export function parseDamageSpec(raw, fallback = {}) {
  const source = raw && typeof raw === "object" && !Array.isArray(raw) ? raw : {}
  const fallbackDamageTypeRaw = String(fallback.damageType || "").trim().toLowerCase()
  const fallbackDamageType = fallbackDamageTypeRaw === "magical" ? "magical" : "physical"
  const fallbackAmountFormulaRaw = fallback.amountFormula && typeof fallback.amountFormula === "object" && !Array.isArray(fallback.amountFormula)
    ? fallback.amountFormula
    : { baseAdd: 0, terms: [] }
  const out = {
    damageType: fallbackDamageType,
    amountFormula: {
      baseAdd: parseNumber(fallbackAmountFormulaRaw.baseAdd, 0),
      terms: parseSkillFormulaTerms(fallbackAmountFormulaRaw.terms)
    },
    hitCount: {
      base: 1,
      formula: []
    },
    flatDefensePenetrationFormula: {
      baseAdd: 0,
      terms: []
    },
    percentDefensePenetrationFormula: {
      baseAdd: 0,
      terms: []
    },
    physicalFlatDefensePenetrationFormula: {
      baseAdd: 0,
      terms: []
    },
    physicalPercentDefensePenetrationFormula: {
      baseAdd: 0,
      terms: []
    },
    magicalFlatDefensePenetrationFormula: {
      baseAdd: 0,
      terms: []
    },
    magicalPercentDefensePenetrationFormula: {
      baseAdd: 0,
      terms: []
    },
    extraDamageAmplificationFormula: {
      baseAdd: 0,
      terms: []
    },
    bonusAgainstTargetTypes: {}
  }
  const damageTypeRaw = String(source.damageType || "").trim().toLowerCase()
  if (damageTypeRaw === "physical" || damageTypeRaw === "magical") {
    out.damageType = damageTypeRaw
  }
  if (source.amountFormula && typeof source.amountFormula === "object" && !Array.isArray(source.amountFormula)) {
    out.amountFormula = parseSkillAmountFormula(source.amountFormula, out.amountFormula.terms)
  }
  if (source.hitCount && typeof source.hitCount === "object" && !Array.isArray(source.hitCount)) {
    const base = Math.max(1, Math.round(parseNumber(source.hitCount.base, out.hitCount.base)))
    out.hitCount.base = base
    out.hitCount.formula = parseSkillFormulaTerms(source.hitCount.formula)
  }
  if (Object.hasOwn(source, "hitDamageScaling")) {
    validateHitDamageScaling(source)
    out.hitDamageScaling = {
      kind: source.hitDamageScaling.kind,
      secondHitMultiplier: source.hitDamageScaling.secondHitMultiplier,
      curvePower: source.hitDamageScaling.curvePower
    }
  }
  out.flatDefensePenetrationFormula = parseSkillAmountFormula(source.flatDefensePenetrationFormula, [])
  out.percentDefensePenetrationFormula = parseSkillAmountFormula(source.percentDefensePenetrationFormula, [])
  out.percentDefensePenetrationFormula.baseAdd = clampNumber(out.percentDefensePenetrationFormula.baseAdd, 0, 1)
  out.physicalFlatDefensePenetrationFormula = parseSkillAmountFormula(source.physicalFlatDefensePenetrationFormula, [])
  out.physicalPercentDefensePenetrationFormula = parseSkillAmountFormula(source.physicalPercentDefensePenetrationFormula, [])
  out.physicalPercentDefensePenetrationFormula.baseAdd = clampNumber(out.physicalPercentDefensePenetrationFormula.baseAdd, 0, 1)
  out.magicalFlatDefensePenetrationFormula = parseSkillAmountFormula(source.magicalFlatDefensePenetrationFormula, [])
  out.magicalPercentDefensePenetrationFormula = parseSkillAmountFormula(source.magicalPercentDefensePenetrationFormula, [])
  out.magicalPercentDefensePenetrationFormula.baseAdd = clampNumber(out.magicalPercentDefensePenetrationFormula.baseAdd, 0, 1)
  out.extraDamageAmplificationFormula = parseSkillAmountFormula(source.extraDamageAmplificationFormula, [])
  if (source.bonusAgainstTargetTypes && typeof source.bonusAgainstTargetTypes === "object" && !Array.isArray(source.bonusAgainstTargetTypes)) {
    const bonuses = {}
    for (const [key, value] of Object.entries(source.bonusAgainstTargetTypes)) {
      const normalizedKey = String(key || "").trim().toLowerCase()
      const amount = parseNumber(value, Number.NaN)
      if (!normalizedKey || !Number.isFinite(amount)) continue
      bonuses[normalizedKey] = amount
    }
    out.bonusAgainstTargetTypes = bonuses
  }
  if (Object.hasOwn(source, "enemyOnlyDamageReductionPierce")) {
    out.enemyOnlyDamageReductionPierce = typeof source.enemyOnlyDamageReductionPierce === "number"
      ? source.enemyOnlyDamageReductionPierce
      : Number.NaN
  }
  if (Object.hasOwn(source, "enemyOnlyDamageReductionPierceTargetStatuses")) {
    out.enemyOnlyDamageReductionPierceTargetStatuses = Array.isArray(source.enemyOnlyDamageReductionPierceTargetStatuses)
      ? source.enemyOnlyDamageReductionPierceTargetStatuses.map((statusID) => typeof statusID === "string" ? statusID.trim() : statusID)
      : source.enemyOnlyDamageReductionPierceTargetStatuses
  }
  return out
}

function parseSkillStatusPayload(raw) {
  const source = raw && typeof raw === "object" && !Array.isArray(raw) ? raw : {}
  const ids = Array.isArray(source.statusIds)
    ? source.statusIds.map((item) => String(item || "").trim()).filter(Boolean)
    : typeof source.statusIds === "string"
      ? source.statusIds.split(",").map((part) => part.trim()).filter(Boolean)
      : []
  const uniqueIDs = []
  for (const id of ids) {
    if (uniqueIDs.includes(id)) continue
    uniqueIDs.push(id)
  }
  const payload = source.payload && typeof source.payload === "object" && !Array.isArray(source.payload)
    ? Object.fromEntries(Object.entries(source.payload).filter(([key]) => String(key || "").trim() !== ""))
    : undefined
  const payloadFormulaSource = source.payloadFormula && typeof source.payloadFormula === "object" && !Array.isArray(source.payloadFormula)
    ? source.payloadFormula
    : source.payloadFormulas && typeof source.payloadFormulas === "object" && !Array.isArray(source.payloadFormulas)
      ? source.payloadFormulas
      : undefined
  const payloadFormula = payloadFormulaSource
    ? Object.fromEntries(Object.entries(payloadFormulaSource)
      .map(([key, value]) => [String(key || "").trim(), parseSkillAmountFormula(value, [])])
      .filter(([key]) => key !== ""))
    : undefined
  const applyChanceConfigured = [
    source.applyChanceBase,
    source.apply_chance_base,
    source.applyChance,
    source.apply_chance,
    source.applyChancePerSkillLevel,
    source.apply_chance_per_skill_level,
    source.applyChanceFormula,
    source.apply_chance_formula,
    source.applyChanceMin,
    source.apply_chance_min,
    source.applyChanceMax,
    source.apply_chance_max
  ].some((value) => value !== undefined && value !== null)
  const applyChanceMin = Math.max(0, Math.min(1, parseNumber(source.applyChanceMin ?? source.apply_chance_min, 0)))
  const applyChanceMax = Math.max(applyChanceMin, Math.min(1, parseNumber(source.applyChanceMax ?? source.apply_chance_max, 1)))
  return {
    statusIds: uniqueIDs,
    durationBaseTurns: Math.max(0, Math.round(parseNumber(source.durationBaseTurns, 0))),
    durationPerSkillLevelTurns: parseNumber(source.durationPerSkillLevelTurns, 0),
    durationFormula: parseSkillAmountFormula(source.durationFormula, []),
    ...(applyChanceConfigured ? {
      applyChanceBase: parseNumber(source.applyChanceBase ?? source.apply_chance_base ?? source.applyChance ?? source.apply_chance, 0),
      applyChancePerSkillLevel: parseNumber(source.applyChancePerSkillLevel ?? source.apply_chance_per_skill_level, 0),
      applyChanceFormula: parseSkillAmountFormula(source.applyChanceFormula ?? source.apply_chance_formula, []),
      applyChanceMin,
      applyChanceMax
    } : {}),
    ...(payload && Object.keys(payload).length > 0 ? { payload } : {}),
    ...(payloadFormula && Object.keys(payloadFormula).length > 0 ? { payloadFormula } : {})
  }
}

function normalizeSummonOnCasterDeath(raw) {
  return String(raw || "").trim().toLowerCase() === "despawn_immediately"
    ? "despawn_immediately"
    : "keep_until_duration_end"
}

function normalizeSummonOnCapOverflow(raw) {
  return String(raw || "").trim().toLowerCase() === "fail_new_summon"
    ? "fail_new_summon"
    : "replace_oldest"
}

function normalizeSummonOnDispel(raw) {
  return String(raw || "").trim().toLowerCase() === "ignore_dispel"
    ? "ignore_dispel"
    : "despawn_immediately"
}

function normalizeSummonOnDurationEnd(raw) {
  return String(raw || "").trim().toLowerCase() === "despawn_immediately"
    ? "despawn_immediately"
    : "despawn_immediately"
}

function validateSummonUnitSelector(payload, fieldPath, knownEnemyIDs = null) {
  if (!payload || typeof payload !== "object" || Array.isArray(payload)) {
    throw new Error(`${fieldPath} invalid`)
  }
  const unitID = parseOptionalString(payload.unitId ?? payload.enemyId) || ""
  if (!unitID) throw new Error(`${fieldPath}.unitId invalid`)
  if (knownEnemyIDs && !knownEnemyIDs.has(unitID)) {
    throw new Error(`${fieldPath}.unitId not found: ${unitID}`)
  }
}

function validateSummonMaxCountFormula(formula, fieldPath, allowedAttrIDs = null) {
  if (!formula || typeof formula !== "object" || Array.isArray(formula)) {
    throw new Error(`${fieldPath} invalid`)
  }
  if (!Number.isFinite(formula.baseAdd) || !Array.isArray(formula.terms)) {
    throw new Error(`${fieldPath} invalid`)
  }
  if (formula.baseAdd === 0 && formula.terms.length === 0) {
    throw new Error(`${fieldPath} must not be empty`)
  }
  for (const term of formula.terms) {
    if (!term || typeof term !== "object" || Array.isArray(term)) {
      throw new Error(`${fieldPath}.terms invalid`)
    }
    const attrKey = typeof term.attrKey === "string" ? term.attrKey.trim() : ""
    if (!attrKey || (allowedAttrIDs && !allowedAttrIDs.has(attrKey))) {
      throw new Error(`${fieldPath}.terms.attrKey invalid: ${attrKey}`)
    }
    if (
      !Number.isFinite(term.coefficient) ||
      (term.perSkillLevel !== undefined && !Number.isFinite(term.perSkillLevel)) ||
      (term.perEnhanceLevel !== undefined && !Number.isFinite(term.perEnhanceLevel))
    ) {
      throw new Error(`${fieldPath}.terms coefficient invalid: ${attrKey}`)
    }
  }
}

export function validateEquipmentExtraEnemyRefreshEffect(effect, hook, template) {
  const templateID = String(template?.id || "").trim() || "(unknown)"
  const triggerKey = normalizeBuffDSLToken(hook?.trigger)
  if (triggerKey !== "onenemyrefresh") {
    throw new Error(`equipment_templates.battleHooks.effects.extra_enemy_refresh trigger invalid: ${templateID}`)
  }
  if (!Array.isArray(hook?.effects) || hook.effects.length !== 1 || !Array.isArray(hook?.conditions) || hook.conditions.length !== 0) {
    throw new Error(`equipment_templates.battleHooks.effects.extra_enemy_refresh requires one dedicated unconditional hook: ${templateID}`)
  }
  for (const field of ["maxTriggersPerTurn", "maxTriggersPerWave", "maxTriggersPerBattle", "cooldownCompletedTurns", "activationCost", "eventFilter", "subjectFilter"]) {
    if (hook[field] !== undefined) {
      throw new Error(`equipment_templates.battleHooks.effects.extra_enemy_refresh does not support ${field}: ${templateID}`)
    }
  }
  if (effect?.enemyPool !== "current_wave_boss") {
    throw new Error(`equipment_templates.battleHooks.effects.extra_enemy_refresh.enemyPool invalid: ${templateID}`)
  }
  if (effect?.partyStackMode !== "highest") {
    throw new Error(`equipment_templates.battleHooks.effects.extra_enemy_refresh.partyStackMode invalid: ${templateID}`)
  }
  if (effect?.settlementMode !== "normal") {
    throw new Error(`equipment_templates.battleHooks.effects.extra_enemy_refresh.settlementMode invalid: ${templateID}`)
  }
  for (const field of probabilityLifecycleProcChanceFields(effect || {})) {
    throw new Error(`equipment_templates.battleHooks.effects.extra_enemy_refresh.${field} unsupported: ${templateID}`)
  }
  const formula = effect?.countFormula
  validateSummonMaxCountFormula(
    formula,
    `equipment_templates.battleHooks.effects.extra_enemy_refresh.countFormula: ${templateID}`,
    new Set(["equipment_enhance_level"])
  )
  if (!Number.isInteger(formula.baseAdd) || formula.baseAdd < 1 || formula.terms.length !== 1) {
    throw new Error(`equipment_templates.battleHooks.effects.extra_enemy_refresh.countFormula invalid: ${templateID}`)
  }
  const term = formula.terms[0]
  if (term.attrKey !== "equipment_enhance_level" || term.coefficient <= 0 || (term.perSkillLevel ?? 0) !== 0 || term.perEnhanceLevel !== undefined) {
    throw new Error(`equipment_templates.battleHooks.effects.extra_enemy_refresh.countFormula enhancement term invalid: ${templateID}`)
  }
  const maxCount = effect?.maxCount
  if (!Number.isInteger(maxCount) || maxCount < 1 || maxCount > 8) {
    throw new Error(`equipment_templates.battleHooks.effects.extra_enemy_refresh.maxCount invalid: ${templateID}`)
  }
  const enhanceMaxLevel = Number(template?.enhanceMaxLevel || 0)
  const formulaAtMax = Math.floor(formula.baseAdd + term.coefficient * enhanceMaxLevel)
  if (maxCount < formulaAtMax) {
    throw new Error(`equipment_templates.battleHooks.effects.extra_enemy_refresh.maxCount truncates authored enhancement curve: ${templateID}`)
  }
}

function parseSkillSummonPayload(raw) {
  const source = raw && typeof raw === "object" && !Array.isArray(raw) ? raw : {}
  const unitID = String(source.unitId || source.enemyId || "").trim()
  const statsFormula = {}
  const statsSource = source.statsFormula && typeof source.statsFormula === "object" && !Array.isArray(source.statsFormula) ? source.statsFormula : {}
  for (const [key, value] of Object.entries(statsSource)) {
    const trimmed = String(key || "").trim()
    if (!trimmed) continue
    statsFormula[trimmed] = parseSkillAmountFormula(value, [])
  }
  return {
    unitId: unitID,
    side: String(source.side || "").trim() || "source",
    countFormula: parseSkillAmountFormula(source.countFormula, [{ coefficient: 1 }]),
    durationFormula: parseSkillAmountFormula(source.durationFormula, []),
    durationTurns: Math.max(0, Math.round(parseNumber(source.durationTurns, 0))),
    maxCountPerUnit: Math.max(0, Math.round(parseNumber(source.maxCountPerUnit, 0))),
    ...(source.maxCountFormula !== undefined ? { maxCountFormula: parseSkillAmountFormula(source.maxCountFormula, []) } : {}),
    persistAcrossBattles: parseBoolean(source.persistAcrossBattles, false),
    onCasterDeath: normalizeSummonOnCasterDeath(source.onCasterDeath),
    onCapOverflow: normalizeSummonOnCapOverflow(source.onCapOverflow),
    onDispel: normalizeSummonOnDispel(source.onDispel),
    onDurationEnd: normalizeSummonOnDurationEnd(source.onDurationEnd),
    statsFormula
  }
}

function parseEffectReferenceIndexes(raw) {
  const refs = []
  const appendRef = (value) => {
    const parsed = Math.round(parseNumber(value, Number.NaN))
    if (!Number.isFinite(parsed) || parsed <= 0) return
    if (refs.includes(parsed)) return
    refs.push(parsed)
  }
  if (Array.isArray(raw)) {
    for (const value of raw) appendRef(value)
  } else if (typeof raw === "string" && raw.trim() !== "") {
    for (const value of raw.split(",")) appendRef(value)
  } else if (raw !== undefined && raw !== null) {
    appendRef(raw)
  }
  return refs
}

function parseEffectReferenceIndex(raw) {
  const refs = parseEffectReferenceIndexes(raw)
  return refs.length > 0 ? refs[0] : 0
}

function parseWeightedStatusChoices(raw) {
  if (!Array.isArray(raw)) return []
  return raw.map((choice) => ({
    id: parseOptionalString(choice?.id) || "",
    weight: parseNumber(choice?.weight, 0),
    statusId: parseOptionalString(choice?.statusId ?? choice?.buffId) || "",
    durationTurns: Math.max(0, Math.round(parseNumber(choice?.durationTurns, 0))),
    ...(choice?.payload && typeof choice.payload === "object" && !Array.isArray(choice.payload) ? { payload: choice.payload } : {})
  }))
}

const STABLE_BATTLE_EFFECT_ID_PATTERN = /^[a-z0-9][a-z0-9._:-]*$/

function sourceStableBattleEffectID(effect, label) {
  if (!effect || typeof effect !== "object" || Array.isArray(effect)) return ""
  const hasCanonical = Object.prototype.hasOwnProperty.call(effect, "id")
  const hasLegacy = Object.prototype.hasOwnProperty.call(effect, "effectId")
  const canonical = hasCanonical && typeof effect.id === "string" ? effect.id.trim() : ""
  const legacy = hasLegacy && typeof effect.effectId === "string" ? effect.effectId.trim() : ""
  if (hasCanonical && !canonical) throw new Error(`${label}.id invalid`)
  if (hasLegacy && !legacy) throw new Error(`${label}.effectId invalid`)
  if (canonical && legacy && canonical !== legacy) throw new Error(`${label} stable id mismatch: ${canonical} != ${legacy}`)
  const stableID = canonical || legacy
  if (!stableID) throw new Error(`${label}.id required`)
  if (!STABLE_BATTLE_EFFECT_ID_PATTERN.test(stableID)) {
    throw new Error(`${label}.id invalid: stable effect ids must use lowercase letters, digits, dot, underscore, colon, or hyphen`)
  }
  return stableID
}

export function validateStableBattleEffectSourceIDs(skills = [], equipmentTemplates = []) {
  const ownerByID = new Map()
  const validateRows = (rows, effectKey, domain) => {
    if (!Array.isArray(rows)) return
    for (const row of rows) {
      if (!row || typeof row !== "object" || !Array.isArray(row[effectKey])) continue
      const ownerID = String(row.id || "").trim() || "(missing-owner)"
      for (let effectIndex = 0; effectIndex < row[effectKey].length; effectIndex += 1) {
        const label = `${domain}.${ownerID}.${effectKey}[${effectIndex}]`
        const stableID = sourceStableBattleEffectID(row[effectKey][effectIndex], label)
        if (!stableID) continue
        const identity = stableID.toLowerCase()
        const previousOwner = ownerByID.get(identity)
        if (previousOwner) throw new Error(`battle effect stable id duplicate: ${stableID} (${previousOwner} / ${label})`)
        ownerByID.set(identity, label)
      }
    }
  }
  validateRows(skills, "effects", "skills")
  validateRows(equipmentTemplates, "basicAttackEffects", "equipment_templates")
}

function parseSkillEffects(raw, fallback = {}) {
  if (!Array.isArray(raw)) return []
  const out = []
  for (const row of raw) {
    if (!row || typeof row !== "object" || Array.isArray(row)) continue
    let kind = String(row.kind || "").trim().toLowerCase()
    if (kind === "remove_status") kind = "remove_buff"
    if (kind !== "damage" && kind !== "heal" && kind !== "resource" && kind !== "status" && kind !== "summon" && kind !== "corpse_explosion" && kind !== "adjust_skill_cooldown" && kind !== "weighted_random_per_target" && kind !== "remove_buff") continue
    const targeting = parseSkillTargeting(row.targeting, fallback)
    const effect = {
      id: parseOptionalString(row.id ?? row.effectId) || "",
      kind,
      targeting
    }
    if (kind === "corpse_explosion") {
      effect.corpseTargeting = parseSkillTargeting(row.corpseTargeting ?? row.corpse_targeting, {
        targetDirection: targeting.targetPositions?.[0] || "vanguard",
        targetSeekStrategy: targeting.secondarySeek || "front_to_rear",
        targetBaseCount: 1,
        targetMax: 1,
        targetFormula: []
      })
      effect.consumeCorpse = parseBoolean(row.consumeCorpse, true)
    }
    const onlyWhenNoTargetFromEffects = parseEffectReferenceIndexes(row.onlyWhenNoTargetFromEffects)
    if (onlyWhenNoTargetFromEffects.length > 0) {
      effect.onlyWhenNoTargetFromEffects = onlyWhenNoTargetFromEffects
    }
    const reuseTargetsFromEffect = parseEffectReferenceIndex(row.reuseTargetsFromEffect)
    if (reuseTargetsFromEffect > 0) {
      effect.reuseTargetsFromEffect = reuseTargetsFromEffect
    }
    const ignoreUntargetable = parseBoolean(row.ignoreUntargetable, false)
    if (ignoreUntargetable) {
      effect.ignoreUntargetable = true
    }
    const consumeStatusesOnHit = Array.isArray(row.consumeStatusesOnHit)
      ? row.consumeStatusesOnHit.map((item) => String(item || "").trim()).filter(Boolean)
      : []
    if (consumeStatusesOnHit.length > 0) {
      effect.consumeStatusesOnHit = [...new Set(consumeStatusesOnHit)]
    }
    const consumeCasterStatusesOnHit = Array.isArray(row.consumeCasterStatusesOnHit)
      ? row.consumeCasterStatusesOnHit.map((item) => String(item || "").trim()).filter(Boolean)
      : []
    if (consumeCasterStatusesOnHit.length > 0) {
      effect.consumeCasterStatusesOnHit = [...new Set(consumeCasterStatusesOnHit)]
    }
    const requiredCasterStatuses = Array.isArray(row.requiredCasterStatuses)
      ? row.requiredCasterStatuses.map((item) => String(item || "").trim()).filter(Boolean)
      : []
    if (requiredCasterStatuses.length > 0) {
      effect.requiredCasterStatuses = [...new Set(requiredCasterStatuses)]
    }
    const requiredTargetStatuses = Array.isArray(row.requiredTargetStatuses)
      ? row.requiredTargetStatuses.map((item) => String(item || "").trim()).filter(Boolean)
      : []
    if (requiredTargetStatuses.length > 0) {
      effect.requiredTargetStatuses = [...new Set(requiredTargetStatuses)]
    }
    const forbiddenCasterStatuses = Array.isArray(row.forbiddenCasterStatuses)
      ? row.forbiddenCasterStatuses.map((item) => String(item || "").trim()).filter(Boolean)
      : []
    if (forbiddenCasterStatuses.length > 0) {
      effect.forbiddenCasterStatuses = [...new Set(forbiddenCasterStatuses)]
    }
    const forbiddenTargetStatuses = Array.isArray(row.forbiddenTargetStatuses)
      ? row.forbiddenTargetStatuses.map((item) => String(item || "").trim()).filter(Boolean)
      : []
    if (forbiddenTargetStatuses.length > 0) {
      effect.forbiddenTargetStatuses = [...new Set(forbiddenTargetStatuses)]
    }
    if (kind === "damage" || kind === "heal" || kind === "resource" || kind === "corpse_explosion") {
      const damageTypeRaw = String(row.damageType || fallback.damageType || "").trim().toLowerCase()
      const amountFormula = parseSkillAmountFormula(row.amountFormula, [])
      effect.amountFormula = amountFormula
      if (kind === "resource") {
        const resource = String(row.resource || "mp").trim().toLowerCase()
        effect.resource = resource === "hp" ? "hp" : "mp"
      } else {
        effect.damageType = damageTypeRaw === "magical" ? "magical" : "physical"
      }
      if (kind === "damage") {
        effect.damageSpec = parseDamageSpec(row.damageSpec, {
          damageType: effect.damageType,
          amountFormula
        })
      }
      if (kind === "heal" && row.mpCost && typeof row.mpCost === "object" && !Array.isArray(row.mpCost)) {
        const mpCostFormula = parseSkillAmountFormula(row.mpCost, [])
        if (mpCostFormula.baseAdd !== 0 || mpCostFormula.terms.length > 0) {
          effect.mpCost = mpCostFormula
        }
      }
    } else if (kind === "status") {
      effect.statusPayload = parseSkillStatusPayload(row.statusPayload)
    } else if (kind === "weighted_random_per_target") {
      const statusPayload = row.statusPayload && typeof row.statusPayload === "object" && !Array.isArray(row.statusPayload)
        ? { ...row.statusPayload }
        : {}
      for (const key of ["applyChanceBase", "applyChancePerSkillLevel", "applyChanceFormula", "applyChanceMin", "applyChanceMax"]) {
        if (row[key] !== undefined) statusPayload[key] = row[key]
      }
      effect.statusPayload = parseSkillStatusPayload(statusPayload)
      effect.choices = parseWeightedStatusChoices(row.choices)
    } else if (kind === "remove_buff") {
      effect.buffIds = [...new Set(parseIDList(row.buffIds ?? row.statusIds ?? row.buffId ?? row.statusId))]
      effect.maxRemoved = Math.max(0, Math.round(parseNumber(row.maxRemoved, effect.buffIds.length)))
    } else if (kind === "summon") {
      effect.summonPayload = parseSkillSummonPayload(row.summonPayload || row.summon || {})
    } else if (kind === "adjust_skill_cooldown") {
      effect.amount = Math.round(parseNumber(row.amount ?? row.turns ?? row.value, 0))
      effect.includeBasic = parseBoolean(row.includeBasic ?? row.include_basic, false)
      effect.skillIds = parseIDList(row.skillIds ?? row.skills)
      effect.excludeSkillIds = parseIDList(row.excludeSkillIds ?? row.excludeSkills)
    }
    out.push(effect)
  }
  return out
}

function cloneSkillTargeting(targeting, overrides = {}) {
  const next = {
    ...targeting,
    ...overrides
  }
  next.targetCount = {
    ...targeting.targetCount,
    ...(overrides.targetCount || {})
  }
  return next
}

function buildTargetingEffect(targeting, overrides = {}) {
  return cloneSkillTargeting(targeting, overrides)
}

function normalizeBasicAttackFormula(raw) {
  if (!raw || typeof raw !== "object" || Array.isArray(raw)) {
    return undefined
  }
  const damageTypeRaw = String(raw.damageType || "").trim().toLowerCase()
  if (damageTypeRaw !== "physical" && damageTypeRaw !== "magical") {
    return undefined
  }
  const formula = parseBasicAttackFormulaTerms(raw.formula)
  if (formula.length === 0) {
    return undefined
  }
  const fallbackAmountFormula = {
    baseAdd: 0,
    terms: formula.map((term) => ({
      attrKey: term.attrKey,
      coefficient: term.coefficient,
      perSkillLevel: 0
    }))
  }
  const damageSpec = parseDamageSpec(raw.damageSpec, {
    damageType: damageTypeRaw,
    amountFormula: fallbackAmountFormula
  })
  return {
    damageType: damageTypeRaw,
    formula,
    damageSpec
  }
}

function buildLegacyBasicAttackEffectsFromFormula({
  basicAttackFormula,
  basicAttackTargetDirection,
  basicAttackSeekStrategy
}) {
  if (!basicAttackFormula) return []
  const amountTerms = basicAttackFormula.formula.map((term) => ({
    attrKey: term.attrKey,
    coefficient: term.coefficient,
    perSkillLevel: 0
  }))
  const fallbackTargeting = parseSkillTargeting(undefined, {
    targetDirection: basicAttackTargetDirection || "vanguard",
    targetSeekStrategy: basicAttackSeekStrategy || "front_to_rear",
    targetBaseCount: 1,
    targetFormula: []
  })
  const damageSpec = parseDamageSpec(basicAttackFormula.damageSpec, {
    damageType: basicAttackFormula.damageType,
    amountFormula: {
      baseAdd: 0,
      terms: amountTerms
    }
  })
  return [
    {
      kind: "damage",
      damageType: basicAttackFormula.damageType,
      targeting: fallbackTargeting,
      amountFormula: {
        baseAdd: 0,
        terms: amountTerms
      },
      damageSpec
    }
  ]
}

function validateEffectLinkage(effect, effectIndex, ownerID, prefix) {
  if (effect.onlyWhenNoTargetFromEffects !== undefined) {
    if (!Array.isArray(effect.onlyWhenNoTargetFromEffects) || effect.onlyWhenNoTargetFromEffects.length === 0) {
      throw new Error(`${prefix}.onlyWhenNoTargetFromEffects invalid: ${ownerID}`)
    }
    const dedupe = new Set()
    for (const ref of effect.onlyWhenNoTargetFromEffects) {
      if (!Number.isFinite(ref) || !Number.isInteger(ref) || ref <= 0 || ref >= effectIndex) {
        throw new Error(`${prefix}.onlyWhenNoTargetFromEffects reference invalid: ${ownerID}`)
      }
      if (dedupe.has(ref)) {
        throw new Error(`${prefix}.onlyWhenNoTargetFromEffects duplicated reference: ${ownerID}`)
      }
      dedupe.add(ref)
    }
  }
  if (effect.reuseTargetsFromEffect !== undefined) {
    if (!Number.isFinite(effect.reuseTargetsFromEffect) || !Number.isInteger(effect.reuseTargetsFromEffect) || effect.reuseTargetsFromEffect <= 0 || effect.reuseTargetsFromEffect >= effectIndex) {
      throw new Error(`${prefix}.reuseTargetsFromEffect invalid: ${ownerID}`)
    }
  }
}

function normalizeSkills(rows) {
  return (rows || [])
    .map((raw) => {
      const id = typeof raw?.id === "string" ? raw.id.trim() : ""
      if (!id) return null
      const parsedEffects = parseSkillEffects(raw?.effects, {
        damageType: "physical",
        targetDirection: "vanguard",
        targetSeekStrategy: "front_to_rear",
        targetBaseCount: 1,
        targetFormula: []
      })
      return {
        ...raw,
        id,
        name: parseOptionalString(raw?.name) || "",
        icon: parseOptionalString(raw?.icon) || "",
        nameKey: parseOptionalString(raw?.nameKey) || `skill.${id}.name`,
        descKey: parseOptionalString(raw?.descKey) || `skill.${id}.desc`,
        category: parseOptionalString(raw?.category) || "",
        hpCost: Math.max(0, parseNumber(raw?.hpCost, 0)),
        mpCost: Math.max(0, parseNumber(raw?.mpCost, 0)),
        cooldown: Math.max(0, Math.round(parseNumber(raw?.cooldown, 0))),
        maxLevel: Math.max(1, Math.round(parseNumber(raw?.maxLevel, 200))),
        enemyOnly: raw?.enemyOnly === true,
        statusRequirement: parseNumberMap(raw?.statusRequirement),
        weaponRequirement: parseCommaSeparatedArray(raw?.weaponRequirement),
        ...(raw?.targeting !== undefined
          ? {
              targeting: parseSkillTargeting(raw?.targeting, {
                targetDirection: "vanguard",
                targetSeekStrategy: "front_to_rear",
                targetBaseCount: 1,
                targetFormula: []
              })
            }
          : {}),
        effects: parsedEffects,
        castGuards: Array.isArray(raw?.castGuards)
          ? raw.castGuards
            .map((item) => {
              if (!item || typeof item !== "object" || Array.isArray(item)) return null
              const kind = String(item.kind || "").trim().toLowerCase()
              if (kind === "ally_side_has_non_full_hp") return { kind }
              if (kind !== "enemy_side_not_has_effect") return null
              const effectKind = String(item.effectKind || item.effect || "").trim().toLowerCase()
              if (!effectKind) return null
              return { kind, effectKind }
            })
            .filter(Boolean)
          : []
      }
    })
    .filter(Boolean)
}

function normalizeSkillTypes(rows) {
  return (rows || [])
    .map((raw) => {
      const id = typeof raw?.id === "string" ? raw.id.trim() : ""
      if (!id) return null
      return {
        ...raw,
        id,
        nameKey: parseOptionalString(raw?.nameKey) || `skill_type.${id}.name`,
        descKey: parseOptionalString(raw?.descKey) || `skill_type.${id}.desc`,
        color: parseOptionalString(raw?.color) || ""
      }
    })
    .filter(Boolean)
    .sort((a, b) => a.id.localeCompare(b.id))
}

function normalizeWeaponClasses(rows) {
  return (rows || [])
    .map((raw, index) => {
      const id = typeof raw?.id === "string" ? raw.id.trim() : ""
      if (!id) return null
      const order = Number.isFinite(Number(raw?.order)) ? Number(raw.order) : index
      return {
        ...raw,
        id,
        nameKey: parseOptionalString(raw?.nameKey) || `weapon_class.${id}.name`,
        descKey: parseOptionalString(raw?.descKey) || `weapon_class.${id}.desc`,
        icon: parseOptionalString(raw?.icon) || "",
        color: parseOptionalString(raw?.color) || "#909399",
        order
      }
    })
    .filter(Boolean)
    .sort((a, b) => {
      if (a.order !== b.order) return a.order - b.order
      return a.id.localeCompare(b.id)
    })
}

function normalizeEquipmentSecondaryClasses(rows) {
  return (rows || [])
    .map((raw, index) => {
      const id = typeof raw?.id === "string" ? raw.id.trim() : ""
      if (!id) return null
      const module = raw?.module === "work" || raw?.module === "battle" || raw?.module === "all"
        ? raw.module
        : "all"
      const order = Number.isFinite(Number(raw?.order)) ? Number(raw.order) : index
      const zhCN = raw?.zhCN && typeof raw.zhCN === "object" && !Array.isArray(raw.zhCN) ? raw.zhCN : {}
      const enUS = raw?.enUS && typeof raw.enUS === "object" && !Array.isArray(raw.enUS) ? raw.enUS : {}
      return {
        id,
        module,
        order,
        nameKey: parseOptionalString(raw?.nameKey) || `equipment_secondary_class.${id}.name`,
        descKey: parseOptionalString(raw?.descKey) || `equipment_secondary_class.${id}.desc`,
        zhCN: {
          name: parseOptionalString(zhCN?.name) || "",
          desc: parseOptionalString(zhCN?.desc) || ""
        },
        enUS: {
          name: parseOptionalString(enUS?.name) || "",
          desc: parseOptionalString(enUS?.desc) || ""
        }
      }
    })
    .filter(Boolean)
    .sort((a, b) => {
      if (a.order !== b.order) return a.order - b.order
      return a.id.localeCompare(b.id)
    })
}

function normalizeBuffTypes(rows) {
  return (rows || [])
    .map((raw) => {
      const id = typeof raw?.id === "string" ? raw.id.trim() : ""
      if (!id) return null
      if (parseBoolean(raw?.disabled, false)) return null
      return {
        ...raw,
        id,
        nameKey: parseOptionalString(raw?.nameKey) || `buff_type.${id}.name`,
        descKey: parseOptionalString(raw?.descKey) || `buff_type.${id}.desc`,
        color: parseOptionalString(raw?.color) || "#909399"
      }
    })
    .filter(Boolean)
    .sort((a, b) => a.id.localeCompare(b.id))
}

function normalizeBuffs(rows) {
  return (rows || [])
    .map((raw) => {
      const id = typeof raw?.id === "string" ? raw.id.trim() : ""
      if (!id) return null
      const kindRaw = parseOptionalString(raw?.kind) || ""
      const isBuffFlag = parseBoolean(raw?.isBuff, false)
      const isDebuffFlag = parseBoolean(raw?.isDebuff, false)
      let kind = kindRaw.toLowerCase()
      if (kind !== "buff" && kind !== "debuff") {
        if (isDebuffFlag && !isBuffFlag) kind = "debuff"
        else kind = "buff"
      }
      const dispellableRaw = raw?.dispellable
      const removableRaw = raw?.removable
      const unremovableRaw = raw?.unremovable
      const permanent = parseBoolean(raw?.permanent, false)
      const durationTurns = Math.max(0, Math.round(parseNumber(raw?.durationTurns, parseNumber(raw?.duration, 0))))
      const stackPolicyRaw = String(raw?.stackPolicy || "").trim().toLowerCase()
      const stackPolicy = stackPolicyRaw === "stack" ? "stack" : "refresh_duration"
      const maxStacks = Math.max(0, Math.round(parseNumber(raw?.maxStacks ?? raw?.stackLimit, 0)))
      const dispellable = dispellableRaw !== undefined
        ? parseBoolean(dispellableRaw, true)
        : removableRaw !== undefined
          ? parseBoolean(removableRaw, true)
          : unremovableRaw !== undefined
            ? !parseBoolean(unremovableRaw, false)
            : true
      return {
        ...raw,
        id,
        nameKey: parseOptionalString(raw?.nameKey) || `buff.${id}.name`,
        descKey: parseOptionalString(raw?.descKey) || `buff.${id}.desc`,
        durationTurns: permanent ? 0 : durationTurns,
        permanent,
        kind,
        dispellable,
        type: parseOptionalString(raw?.type) || parseOptionalString(raw?.category) || "",
        stackPolicy,
        ...(maxStacks > 0 ? { maxStacks } : {})
      }
    })
    .filter(Boolean)
}

function normalizeBuffEffects(rows) {
  return (rows || [])
    .map((raw) => {
      if (!raw || typeof raw !== "object") return null
      const id = typeof raw?.id === "string" ? raw.id.trim() : ""
      const buffId = parseOptionalString(raw?.buffId) || ""
      if (!id && !buffId) return null
      const trigger = parseOptionalString(raw?.trigger) || parseOptionalString(raw?.when) || ""
      let conditions = raw?.conditions
      if (!Array.isArray(conditions)) {
        if (conditions && typeof conditions === "object") conditions = [conditions]
        else conditions = []
      }
      let effects = raw?.effects
      if (!Array.isArray(effects)) {
        if (effects && typeof effects === "object") effects = [effects]
        else effects = []
      }
      return {
        ...raw,
        id: id || `${buffId}:${trigger || "effect"}`,
        buffId,
        trigger,
        priority: Math.round(parseNumber(raw?.priority, 0)),
        conditions,
        effects
      }
    })
    .filter(Boolean)
}

export function validateEnemyOnlyDamageReductionPierceSkills(skills, buffs = []) {
  const knownBuffIDs = new Set((buffs || []).map((row) => String(row?.id || "").trim()).filter(Boolean))
  for (const skill of skills || []) {
    if (!skill || typeof skill !== "object" || Array.isArray(skill)) continue
    const skillID = String(skill.id || "").trim() || "(unknown)"
    for (const effect of Array.isArray(skill.effects) ? skill.effects : []) {
      if (!effect || typeof effect !== "object" || Array.isArray(effect) || effect.kind !== "damage") continue
      const spec = effect.damageSpec
      if (!spec || typeof spec !== "object" || Array.isArray(spec)) continue
      const configured = enemyOnlyDamageReductionPierceFields(spec)
      if (configured.length === 0) continue

      const pierce = spec.enemyOnlyDamageReductionPierce
      if (!Number.isFinite(pierce) || pierce < 0 || pierce > 1) {
        throw new Error(`skills.effects.damageSpec.enemyOnlyDamageReductionPierce invalid: ${skillID}`)
      }
      if (pierce > 0 && skill.enemyOnly !== true) {
        throw new Error(`skills.enemyOnly required for damage-reduction pierce: ${skillID}`)
      }

      if (Object.hasOwn(spec, "enemyOnlyDamageReductionPierceTargetStatuses")) {
        const statuses = spec.enemyOnlyDamageReductionPierceTargetStatuses
        if (!Array.isArray(statuses) || statuses.some((statusID) => typeof statusID !== "string" || statusID.trim() === "")) {
          throw new Error(`skills.effects.damageSpec.enemyOnlyDamageReductionPierceTargetStatuses invalid: ${skillID}`)
        }
        const seen = new Set()
        for (const rawStatusID of statuses) {
          const statusID = rawStatusID.trim()
          if (seen.has(statusID)) {
            throw new Error(`skills.effects.damageSpec.enemyOnlyDamageReductionPierceTargetStatuses duplicate: ${skillID}.${statusID}`)
          }
          seen.add(statusID)
          if (!knownBuffIDs.has(statusID)) {
            throw new Error(`skills.effects.damageSpec.enemyOnlyDamageReductionPierceTargetStatuses buff not found: ${skillID}.${statusID}`)
          }
        }
      }
    }
  }
}

export function validateEnemyOnlyTargetPrioritySkills(skills) {
  for (const skill of skills || []) {
    if (!skill || typeof skill !== "object" || Array.isArray(skill)) continue
    const skillID = String(skill.id || "").trim() || "(unknown)"
    const targetings = [
      skill.targeting,
      ...(Array.isArray(skill.effects)
        ? skill.effects.flatMap((effect) => effect && typeof effect === "object" && !Array.isArray(effect)
          ? [effect.targeting, effect.corpseTargeting]
          : [])
        : [])
    ]
    for (const targeting of targetings) {
      if (!targeting || typeof targeting !== "object" || Array.isArray(targeting) || !Object.hasOwn(targeting, "ignoreTargetPriority")) continue
      if (typeof targeting.ignoreTargetPriority !== "boolean") {
        throw new Error(`skills.targeting.ignoreTargetPriority invalid: ${skillID}`)
      }
      if (skill.enemyOnly !== true) {
        throw new Error(`skills.enemyOnly required for targeting.ignoreTargetPriority: ${skillID}`)
      }
    }
  }
}

export function rejectEnemyOnlyTargetPriority(targeting, label) {
  if (targeting && typeof targeting === "object" && !Array.isArray(targeting) && Object.hasOwn(targeting, "ignoreTargetPriority")) {
    throw new Error(`${label}.ignoreTargetPriority is enemy-skill-only`)
  }
}

export function rejectEnemyOnlyDamageReductionPierce(spec, label) {
  const configured = enemyOnlyDamageReductionPierceFields(spec)
  if (configured.length > 0) {
    throw new Error(`${label}.${configured[0]} is enemy-skill-only`)
  }
}

function validateSkillTables(skills, skillTypes, buffs = [], enemies = []) {
  if (!Array.isArray(skills)) throw new Error("skills must be an array")
  if (!Array.isArray(skillTypes)) throw new Error("skill_type must be an array")

  const knownBuffIDs = new Set((buffs || []).map((row) => String(row?.id || "").trim()).filter(Boolean))
  const knownEnemyIDs = new Set((enemies || []).map((row) => String(row?.id || "").trim()).filter(Boolean))

  validateEnemyOnlyDamageReductionPierceSkills(skills, buffs)
  validateEnemyOnlyTargetPrioritySkills(skills)

  const typeIDs = new Set()
  for (const row of skillTypes) {
    if (!row || typeof row !== "object") throw new Error("skill_type row must be object")
    if (typeof row.id !== "string" || row.id === "") throw new Error("skill_type.id must be non-empty string")
    if (typeIDs.has(row.id)) throw new Error(`duplicate skill_type.id: ${row.id}`)
    typeIDs.add(row.id)
    if (typeof row.nameKey !== "string" || row.nameKey === "") throw new Error(`skill_type.nameKey invalid: ${row.id}`)
    if (typeof row.descKey !== "string" || row.descKey === "") throw new Error(`skill_type.descKey invalid: ${row.id}`)
    if (!isHexColor(row.color)) throw new Error(`skill_type.color invalid: ${row.id}`)
  }

  const skillIDs = new Set()
  const stableEffectIDs = new Set()
  for (const row of skills) {
    if (!row || typeof row !== "object") throw new Error("skills row must be object")
    if (typeof row.id !== "string" || row.id === "") throw new Error("skills.id must be non-empty string")
    if (skillIDs.has(row.id)) throw new Error(`duplicate skills.id: ${row.id}`)
    skillIDs.add(row.id)
    if (typeof row.nameKey !== "string" || row.nameKey === "") throw new Error(`skills.nameKey invalid: ${row.id}`)
    if (typeof row.descKey !== "string" || row.descKey === "") throw new Error(`skills.descKey invalid: ${row.id}`)
    if (typeof row.icon !== "string") throw new Error(`skills.icon invalid: ${row.id}`)
    if (typeof row.enemyOnly !== "boolean") throw new Error(`skills.enemyOnly invalid: ${row.id}`)
    if (!Number.isFinite(row.hpCost) || row.hpCost < 0) throw new Error(`skills.hpCost invalid: ${row.id}`)
    if (!Number.isFinite(row.mpCost) || row.mpCost < 0) throw new Error(`skills.mpCost invalid: ${row.id}`)
    if (!Number.isFinite(row.cooldown) || row.cooldown < 0) throw new Error(`skills.cooldown invalid: ${row.id}`)
    if (!row.statusRequirement || typeof row.statusRequirement !== "object" || Array.isArray(row.statusRequirement)) {
      throw new Error(`skills.statusRequirement invalid: ${row.id}`)
    }
    for (const [attrID, required] of Object.entries(row.statusRequirement)) {
      if (!attrID || typeof required !== "number" || !Number.isFinite(required) || required < 0) {
        throw new Error(`skills.statusRequirement value invalid: ${row.id}.${attrID}`)
      }
    }
    if (!Array.isArray(row.weaponRequirement) || row.weaponRequirement.some((value) => typeof value !== "string" || value.trim() === "")) {
      throw new Error(`skills.weaponRequirement invalid: ${row.id}`)
    }
    if (row.targeting !== undefined) {
      if (!row.targeting || typeof row.targeting !== "object" || Array.isArray(row.targeting)) {
        throw new Error(`skills.targeting invalid: ${row.id}`)
      }
      if (!Array.isArray(row.targeting.targetPositions) || row.targeting.targetPositions.length === 0) {
        throw new Error(`skills.targeting.targetPositions invalid: ${row.id}`)
      }
      for (const value of row.targeting.targetPositions) {
        if (value !== "vanguard" && value !== "midguard" && value !== "rearguard") {
          throw new Error(`skills.targeting.targetPositions value invalid: ${row.id}`)
        }
      }
      if (row.targeting.secondarySeek !== "rear_to_front" && row.targeting.secondarySeek !== "front_to_rear" && row.targeting.secondarySeek !== "random_alive" && row.targeting.secondarySeek !== "none") {
        throw new Error(`skills.targeting.secondarySeek invalid: ${row.id}`)
      }
      if (row.targeting.targetScope !== "ally" && row.targeting.targetScope !== "enemy" && row.targeting.targetScope !== "all" && row.targeting.targetScope !== "self") {
        throw new Error(`skills.targeting.targetScope invalid: ${row.id}`)
      }
      if (
        row.targeting.selectRule !== "hp_max" &&
        row.targeting.selectRule !== "hp_min" &&
        row.targeting.selectRule !== "hp_percent_max" &&
        row.targeting.selectRule !== "hp_percent_min" &&
        row.targeting.selectRule !== "first" &&
        row.targeting.selectRule !== "boss_first" &&
        row.targeting.selectRule !== "attr_max" &&
        row.targeting.selectRule !== "attr_min" &&
        row.targeting.selectRule !== "random" &&
        row.targeting.selectRule !== "self"
      ) {
        throw new Error(`skills.targeting.selectRule invalid: ${row.id}`)
      }
      if ((row.targeting.selectRule === "attr_max" || row.targeting.selectRule === "attr_min") && (!row.targeting.selectAttrKey || typeof row.targeting.selectAttrKey !== "string")) {
        throw new Error(`skills.targeting.selectAttrKey invalid: ${row.id}`)
      }
      if (typeof row.targeting.fallbackSelfIfEmpty !== "boolean") {
        throw new Error(`skills.targeting.fallbackSelfIfEmpty invalid: ${row.id}`)
      }
      if (typeof row.targeting.excludeSelf !== "boolean") {
        throw new Error(`skills.targeting.excludeSelf invalid: ${row.id}`)
      }
      if (!row.targeting.targetCount || typeof row.targeting.targetCount !== "object" || Array.isArray(row.targeting.targetCount)) {
        throw new Error(`skills.targeting.targetCount invalid: ${row.id}`)
      }
      if (!Number.isFinite(row.targeting.targetCount.base) || row.targeting.targetCount.base <= 0 || !Number.isInteger(row.targeting.targetCount.base)) {
        throw new Error(`skills.targeting.targetCount.base invalid: ${row.id}`)
      }
      if (!Array.isArray(row.targeting.targetCount.formula)) {
        throw new Error(`skills.targeting.targetCount.formula invalid: ${row.id}`)
      }
      if (row.targeting.targetCount.max !== undefined) {
        if (!Number.isFinite(row.targeting.targetCount.max) || row.targeting.targetCount.max <= 0 || !Number.isInteger(row.targeting.targetCount.max)) {
          throw new Error(`skills.targeting.targetCount.max invalid: ${row.id}`)
        }
      }
      for (const term of row.targeting.targetCount.formula) {
        if (!term || typeof term !== "object" || Array.isArray(term)) throw new Error(`skills.targeting.targetCount.formula term invalid: ${row.id}`)
        if (typeof term.attrKey !== "string" || term.attrKey.trim() === "") throw new Error(`skills.targeting.targetCount.formula.attrKey invalid: ${row.id}`)
        if (!Number.isFinite(term.coefficient)) throw new Error(`skills.targeting.targetCount.formula.coefficient invalid: ${row.id}`)
        if (!Number.isFinite(term.perSkillLevel)) throw new Error(`skills.targeting.targetCount.formula.perSkillLevel invalid: ${row.id}`)
      }
      if (!Array.isArray(row.targeting.targetPredicates)) {
        throw new Error(`skills.targeting.targetPredicates invalid: ${row.id}`)
      }
    }
    if (row.castGuards !== undefined) {
      if (!Array.isArray(row.castGuards)) {
        throw new Error(`skills.castGuards invalid: ${row.id}`)
      }
      for (const guard of row.castGuards) {
        if (!guard || typeof guard !== "object" || Array.isArray(guard)) throw new Error(`skills.castGuards entry invalid: ${row.id}`)
        if (guard.kind === "ally_side_has_non_full_hp") continue
        if (guard.kind !== "enemy_side_not_has_effect") throw new Error(`skills.castGuards.kind invalid: ${row.id}`)
        if (typeof guard.effectKind !== "string" || guard.effectKind.trim() === "") throw new Error(`skills.castGuards.effectKind invalid: ${row.id}`)
      }
    }
    if (!Array.isArray(row.effects)) {
      throw new Error(`skills.effects invalid: ${row.id}`)
    }
    if (row.effects.length === 0) {
      if (!(row.equipmentOnly === true && ["equipment_domination", "equipment_martial_combo"].includes(row.id))) throw new Error(`skills.effects empty: ${row.id}`)
    }
    for (let effectIndex = 0; effectIndex < row.effects.length; effectIndex++) {
      const effect = row.effects[effectIndex]
      if (!effect || typeof effect !== "object" || Array.isArray(effect)) throw new Error(`skills.effects entry invalid: ${row.id}`)
      const canonicalEffectID = typeof effect.id === "string" ? effect.id.trim() : ""
      const legacyEffectID = typeof effect.effectId === "string" ? effect.effectId.trim() : ""
      if (effect.id !== undefined && !canonicalEffectID) throw new Error(`skills.effects.id invalid: ${row.id}.${effectIndex + 1}`)
      if (effect.effectId !== undefined && !legacyEffectID) throw new Error(`skills.effects.effectId invalid: ${row.id}.${effectIndex + 1}`)
      if (canonicalEffectID && legacyEffectID && canonicalEffectID !== legacyEffectID) throw new Error(`skills.effects stable id mismatch: ${row.id}.${effectIndex + 1}`)
      const stableEffectID = canonicalEffectID || legacyEffectID
      if (!stableEffectID) throw new Error(`skills.effects.id required: ${row.id}.${effectIndex + 1}`)
      if (!STABLE_BATTLE_EFFECT_ID_PATTERN.test(stableEffectID)) throw new Error(`skills.effects.id invalid: ${row.id}.${stableEffectID}`)
      const stableEffectIdentity = stableEffectID.toLowerCase()
      if (stableEffectIDs.has(stableEffectIdentity)) throw new Error(`skills.effects.id duplicate: ${stableEffectID}`)
      stableEffectIDs.add(stableEffectIdentity)
      if (effect.kind !== "damage" && effect.kind !== "heal" && effect.kind !== "resource" && effect.kind !== "status" && effect.kind !== "summon" && effect.kind !== "corpse_explosion" && effect.kind !== "adjust_skill_cooldown" && effect.kind !== "weighted_random_per_target" && effect.kind !== "remove_buff") throw new Error(`skills.effects.kind invalid: ${row.id}`)
      if (!effect.targeting || typeof effect.targeting !== "object" || Array.isArray(effect.targeting)) throw new Error(`skills.effects.targeting invalid: ${row.id}`)
      validateEffectLinkage(effect, effectIndex + 1, row.id, "skills.effects")
      if (effect.ignoreUntargetable !== undefined && typeof effect.ignoreUntargetable !== "boolean") throw new Error(`skills.effects.ignoreUntargetable invalid: ${row.id}`)
      if (!Array.isArray(effect.targeting.targetPositions) || effect.targeting.targetPositions.length === 0) throw new Error(`skills.effects.targeting.targetPositions invalid: ${row.id}`)
      if (effect.targeting.secondarySeek !== "rear_to_front" && effect.targeting.secondarySeek !== "front_to_rear" && effect.targeting.secondarySeek !== "random_alive" && effect.targeting.secondarySeek !== "none") {
        throw new Error(`skills.effects.targeting.secondarySeek invalid: ${row.id}`)
      }
      if (effect.targeting.targetScope !== "ally" && effect.targeting.targetScope !== "enemy" && effect.targeting.targetScope !== "all" && effect.targeting.targetScope !== "self") {
        throw new Error(`skills.effects.targeting.targetScope invalid: ${row.id}`)
      }
      if (
        effect.targeting.selectRule !== "hp_max" &&
        effect.targeting.selectRule !== "hp_min" &&
        effect.targeting.selectRule !== "hp_percent_max" &&
        effect.targeting.selectRule !== "hp_percent_min" &&
        effect.targeting.selectRule !== "first" &&
        effect.targeting.selectRule !== "boss_first" &&
        effect.targeting.selectRule !== "attr_max" &&
        effect.targeting.selectRule !== "attr_min" &&
        effect.targeting.selectRule !== "random" &&
        effect.targeting.selectRule !== "self"
      ) {
        throw new Error(`skills.effects.targeting.selectRule invalid: ${row.id}`)
      }
      if ((effect.targeting.selectRule === "attr_max" || effect.targeting.selectRule === "attr_min") && (!effect.targeting.selectAttrKey || typeof effect.targeting.selectAttrKey !== "string")) {
        throw new Error(`skills.effects.targeting.selectAttrKey invalid: ${row.id}`)
      }
      if (typeof effect.targeting.fallbackSelfIfEmpty !== "boolean") {
        throw new Error(`skills.effects.targeting.fallbackSelfIfEmpty invalid: ${row.id}`)
      }
      if (typeof effect.targeting.excludeSelf !== "boolean") {
        throw new Error(`skills.effects.targeting.excludeSelf invalid: ${row.id}`)
      }
      if (!effect.targeting.targetCount || typeof effect.targeting.targetCount !== "object" || Array.isArray(effect.targeting.targetCount)) {
        throw new Error(`skills.effects.targeting.targetCount invalid: ${row.id}`)
      }
      if (!Number.isFinite(effect.targeting.targetCount.base) || effect.targeting.targetCount.base <= 0 || !Number.isInteger(effect.targeting.targetCount.base)) {
        throw new Error(`skills.effects.targeting.targetCount.base invalid: ${row.id}`)
      }
      if (!Array.isArray(effect.targeting.targetCount.formula)) {
        throw new Error(`skills.effects.targeting.targetCount.formula invalid: ${row.id}`)
      }
      if (effect.targeting.targetCount.max !== undefined) {
        if (!Number.isFinite(effect.targeting.targetCount.max) || effect.targeting.targetCount.max <= 0 || !Number.isInteger(effect.targeting.targetCount.max)) {
          throw new Error(`skills.effects.targeting.targetCount.max invalid: ${row.id}`)
        }
      }
      if (!Array.isArray(effect.targeting.targetPredicates)) {
        throw new Error(`skills.effects.targeting.targetPredicates invalid: ${row.id}`)
      }
      for (const predicate of effect.targeting.targetPredicates) {
        if (!predicate || typeof predicate !== "object" || Array.isArray(predicate)) throw new Error(`skills.effects.targeting.targetPredicates entry invalid: ${row.id}`)
        if (predicate.kind !== "compare_attr" && predicate.kind !== "exclude_enemy_type" && predicate.kind !== "require_target_has_status" && predicate.kind !== "exclude_target_has_status" && predicate.kind !== "target_hp_not_full") {
          throw new Error(`skills.effects.targeting.targetPredicates.kind invalid: ${row.id}`)
        }
      }
      if (effect.kind === "corpse_explosion") {
        if (!effect.corpseTargeting || typeof effect.corpseTargeting !== "object" || Array.isArray(effect.corpseTargeting)) throw new Error(`skills.effects.corpseTargeting invalid: ${row.id}`)
        if (typeof effect.consumeCorpse !== "boolean") throw new Error(`skills.effects.consumeCorpse invalid: ${row.id}`)
        if (!Array.isArray(effect.corpseTargeting.targetPositions) || effect.corpseTargeting.targetPositions.length === 0) throw new Error(`skills.effects.corpseTargeting.targetPositions invalid: ${row.id}`)
        for (const value of effect.corpseTargeting.targetPositions) {
          if (value !== "vanguard" && value !== "midguard" && value !== "rearguard") {
            throw new Error(`skills.effects.corpseTargeting.targetPositions value invalid: ${row.id}`)
          }
        }
        if (effect.corpseTargeting.secondarySeek !== "rear_to_front" && effect.corpseTargeting.secondarySeek !== "front_to_rear" && effect.corpseTargeting.secondarySeek !== "random_alive" && effect.corpseTargeting.secondarySeek !== "none") {
          throw new Error(`skills.effects.corpseTargeting.secondarySeek invalid: ${row.id}`)
        }
        if (effect.corpseTargeting.targetScope !== "ally" && effect.corpseTargeting.targetScope !== "enemy" && effect.corpseTargeting.targetScope !== "all" && effect.corpseTargeting.targetScope !== "self") {
          throw new Error(`skills.effects.corpseTargeting.targetScope invalid: ${row.id}`)
        }
        if (
          effect.corpseTargeting.selectRule !== "hp_max" &&
          effect.corpseTargeting.selectRule !== "hp_min" &&
          effect.corpseTargeting.selectRule !== "hp_percent_max" &&
          effect.corpseTargeting.selectRule !== "hp_percent_min" &&
          effect.corpseTargeting.selectRule !== "first" &&
          effect.corpseTargeting.selectRule !== "boss_first" &&
          effect.corpseTargeting.selectRule !== "attr_max" &&
          effect.corpseTargeting.selectRule !== "attr_min" &&
          effect.corpseTargeting.selectRule !== "random" &&
          effect.corpseTargeting.selectRule !== "self"
        ) {
          throw new Error(`skills.effects.corpseTargeting.selectRule invalid: ${row.id}`)
        }
        if ((effect.corpseTargeting.selectRule === "attr_max" || effect.corpseTargeting.selectRule === "attr_min") && (!effect.corpseTargeting.selectAttrKey || typeof effect.corpseTargeting.selectAttrKey !== "string")) {
          throw new Error(`skills.effects.corpseTargeting.selectAttrKey invalid: ${row.id}`)
        }
        if (typeof effect.corpseTargeting.fallbackSelfIfEmpty !== "boolean") {
          throw new Error(`skills.effects.corpseTargeting.fallbackSelfIfEmpty invalid: ${row.id}`)
        }
        if (!effect.corpseTargeting.targetCount || typeof effect.corpseTargeting.targetCount !== "object" || Array.isArray(effect.corpseTargeting.targetCount)) {
          throw new Error(`skills.effects.corpseTargeting.targetCount invalid: ${row.id}`)
        }
        if (!Number.isFinite(effect.corpseTargeting.targetCount.base) || effect.corpseTargeting.targetCount.base <= 0 || !Number.isInteger(effect.corpseTargeting.targetCount.base)) {
          throw new Error(`skills.effects.corpseTargeting.targetCount.base invalid: ${row.id}`)
        }
        if (!Array.isArray(effect.corpseTargeting.targetCount.formula)) {
          throw new Error(`skills.effects.corpseTargeting.targetCount.formula invalid: ${row.id}`)
        }
        if (effect.corpseTargeting.targetCount.max !== undefined) {
          if (!Number.isFinite(effect.corpseTargeting.targetCount.max) || effect.corpseTargeting.targetCount.max <= 0 || !Number.isInteger(effect.corpseTargeting.targetCount.max)) {
            throw new Error(`skills.effects.corpseTargeting.targetCount.max invalid: ${row.id}`)
          }
        }
        if (!Array.isArray(effect.corpseTargeting.targetPredicates)) {
          throw new Error(`skills.effects.corpseTargeting.targetPredicates invalid: ${row.id}`)
        }
      }
      for (const key of ["consumeStatusesOnHit", "consumeCasterStatusesOnHit"]) {
        if (effect[key] !== undefined) {
          if (!Array.isArray(effect[key]) || effect[key].some((value) => typeof value !== "string" || value.trim() === "")) {
            throw new Error(`skills.effects.${key} invalid: ${row.id}`)
          }
        }
      }
      for (const key of ["requiredCasterStatuses", "requiredTargetStatuses", "forbiddenCasterStatuses", "forbiddenTargetStatuses"]) {
        if (effect[key] !== undefined) {
          if (!Array.isArray(effect[key]) || effect[key].some((value) => typeof value !== "string" || value.trim() === "")) {
            throw new Error(`skills.effects.${key} invalid: ${row.id}`)
          }
        }
      }
      if (effect.kind === "damage" || effect.kind === "heal" || effect.kind === "resource" || effect.kind === "corpse_explosion") {
        if (effect.kind !== "resource" && effect.damageType !== "physical" && effect.damageType !== "magical") {
          throw new Error(`skills.effects.damageType invalid: ${row.id}`)
        }
        if (!effect.amountFormula || typeof effect.amountFormula !== "object" || Array.isArray(effect.amountFormula)) {
          throw new Error(`skills.effects.amountFormula invalid: ${row.id}`)
        }
        if (!Number.isFinite(effect.amountFormula.baseAdd)) {
          throw new Error(`skills.effects.amountFormula.baseAdd invalid: ${row.id}`)
        }
        if (!Array.isArray(effect.amountFormula.terms)) {
          throw new Error(`skills.effects.amountFormula.terms invalid: ${row.id}`)
        }
        for (const term of effect.amountFormula.terms) {
          if (!term || typeof term !== "object" || Array.isArray(term)) throw new Error(`skills.effects.amountFormula term invalid: ${row.id}`)
          if (typeof term.attrKey !== "string" || term.attrKey.trim() === "") throw new Error(`skills.effects.amountFormula.attrKey invalid: ${row.id}`)
          if (!Number.isFinite(term.coefficient)) throw new Error(`skills.effects.amountFormula.coefficient invalid: ${row.id}`)
          if (!Number.isFinite(term.perSkillLevel)) throw new Error(`skills.effects.amountFormula.perSkillLevel invalid: ${row.id}`)
        }
        if (effect.kind === "resource" && effect.resource !== "hp" && effect.resource !== "mp") {
          throw new Error(`skills.effects.resource invalid: ${row.id}`)
        }
        if (effect.kind === "damage") {
          if (!effect.damageSpec || typeof effect.damageSpec !== "object" || Array.isArray(effect.damageSpec)) {
            throw new Error(`skills.effects.damageSpec invalid: ${row.id}`)
          }
          validateHitDamageScaling(effect.damageSpec, "skills.effects.damageSpec", row.id)
          if (effect.damageSpec.damageType !== "physical" && effect.damageSpec.damageType !== "magical") {
            throw new Error(`skills.effects.damageSpec.damageType invalid: ${row.id}`)
          }
          const formulaFields = [
            ["amountFormula", effect.damageSpec.amountFormula],
            ["flatDefensePenetrationFormula", effect.damageSpec.flatDefensePenetrationFormula],
            ["percentDefensePenetrationFormula", effect.damageSpec.percentDefensePenetrationFormula],
            ["extraDamageAmplificationFormula", effect.damageSpec.extraDamageAmplificationFormula]
          ]
          for (const [label, formula] of formulaFields) {
            if (!formula || typeof formula !== "object" || Array.isArray(formula)) {
              throw new Error(`skills.effects.damageSpec.${label} invalid: ${row.id}`)
            }
            if (!Number.isFinite(formula.baseAdd)) {
              throw new Error(`skills.effects.damageSpec.${label}.baseAdd invalid: ${row.id}`)
            }
            if (!Array.isArray(formula.terms)) {
              throw new Error(`skills.effects.damageSpec.${label}.terms invalid: ${row.id}`)
            }
            for (const term of formula.terms) {
              if (!term || typeof term !== "object" || Array.isArray(term)) throw new Error(`skills.effects.damageSpec.${label} term invalid: ${row.id}`)
              if (typeof term.attrKey !== "string" || term.attrKey.trim() === "") throw new Error(`skills.effects.damageSpec.${label}.attrKey invalid: ${row.id}`)
              if (!Number.isFinite(term.coefficient)) throw new Error(`skills.effects.damageSpec.${label}.coefficient invalid: ${row.id}`)
              if (!Number.isFinite(term.perSkillLevel)) throw new Error(`skills.effects.damageSpec.${label}.perSkillLevel invalid: ${row.id}`)
            }
          }
          if (effect.damageSpec.bonusAgainstTargetTypes !== undefined) {
            if (!effect.damageSpec.bonusAgainstTargetTypes || typeof effect.damageSpec.bonusAgainstTargetTypes !== "object" || Array.isArray(effect.damageSpec.bonusAgainstTargetTypes)) {
              throw new Error(`skills.effects.damageSpec.bonusAgainstTargetTypes invalid: ${row.id}`)
            }
            for (const [targetType, bonus] of Object.entries(effect.damageSpec.bonusAgainstTargetTypes)) {
              if (!targetType || typeof targetType !== "string" || !Number.isFinite(bonus)) {
                throw new Error(`skills.effects.damageSpec.bonusAgainstTargetTypes entry invalid: ${row.id}.${targetType}`)
              }
            }
          }
          if (!effect.damageSpec.hitCount || typeof effect.damageSpec.hitCount !== "object" || Array.isArray(effect.damageSpec.hitCount)) {
            throw new Error(`skills.effects.damageSpec.hitCount invalid: ${row.id}`)
          }
          if (!Number.isFinite(effect.damageSpec.hitCount.base) || !Number.isInteger(effect.damageSpec.hitCount.base) || effect.damageSpec.hitCount.base <= 0) {
            throw new Error(`skills.effects.damageSpec.hitCount.base invalid: ${row.id}`)
          }
          if (!Array.isArray(effect.damageSpec.hitCount.formula)) {
            throw new Error(`skills.effects.damageSpec.hitCount.formula invalid: ${row.id}`)
          }
          for (const term of effect.damageSpec.hitCount.formula) {
            if (!term || typeof term !== "object" || Array.isArray(term)) throw new Error(`skills.effects.damageSpec.hitCount.formula term invalid: ${row.id}`)
            if (typeof term.attrKey !== "string" || term.attrKey.trim() === "") throw new Error(`skills.effects.damageSpec.hitCount.formula.attrKey invalid: ${row.id}`)
            if (!Number.isFinite(term.coefficient)) throw new Error(`skills.effects.damageSpec.hitCount.formula.coefficient invalid: ${row.id}`)
            if (!Number.isFinite(term.perSkillLevel)) throw new Error(`skills.effects.damageSpec.hitCount.formula.perSkillLevel invalid: ${row.id}`)
          }
        }
      }
      if (effect.kind === "status") {
        if (!effect.statusPayload || typeof effect.statusPayload !== "object" || Array.isArray(effect.statusPayload)) {
          throw new Error(`skills.effects.statusPayload invalid: ${row.id}`)
        }
        if (!Array.isArray(effect.statusPayload.statusIds) || effect.statusPayload.statusIds.some((value) => typeof value !== "string" || value.trim() === "")) {
          throw new Error(`skills.effects.statusPayload.statusIds invalid: ${row.id}`)
        }
        for (const buffID of effect.statusPayload.statusIds) {
          if (!knownBuffIDs.has(buffID)) throw new Error(`skills.effects.statusPayload.statusIds buff not found: ${row.id}.${buffID}`)
        }
        if (!Number.isFinite(effect.statusPayload.durationBaseTurns) || effect.statusPayload.durationBaseTurns < 0 || !Number.isInteger(effect.statusPayload.durationBaseTurns)) {
          throw new Error(`skills.effects.statusPayload.durationBaseTurns invalid: ${row.id}`)
        }
        if (!Number.isFinite(effect.statusPayload.durationPerSkillLevelTurns)) {
          throw new Error(`skills.effects.statusPayload.durationPerSkillLevelTurns invalid: ${row.id}`)
        }
        if (!effect.statusPayload.durationFormula || typeof effect.statusPayload.durationFormula !== "object" || Array.isArray(effect.statusPayload.durationFormula)) {
          throw new Error(`skills.effects.statusPayload.durationFormula invalid: ${row.id}`)
        }
        if (!Number.isFinite(effect.statusPayload.durationFormula.baseAdd)) {
          throw new Error(`skills.effects.statusPayload.durationFormula.baseAdd invalid: ${row.id}`)
        }
        if (!Array.isArray(effect.statusPayload.durationFormula.terms)) {
          throw new Error(`skills.effects.statusPayload.durationFormula.terms invalid: ${row.id}`)
        }
        if (effect.statusPayload.applyChanceBase !== undefined) {
          if (!Number.isFinite(effect.statusPayload.applyChanceBase) || !Number.isFinite(effect.statusPayload.applyChancePerSkillLevel)) {
            throw new Error(`skills.effects.statusPayload.applyChance invalid: ${row.id}`)
          }
          if (!Number.isFinite(effect.statusPayload.applyChanceMin) || !Number.isFinite(effect.statusPayload.applyChanceMax) || effect.statusPayload.applyChanceMin < 0 || effect.statusPayload.applyChanceMax > 1 || effect.statusPayload.applyChanceMin > effect.statusPayload.applyChanceMax) {
            throw new Error(`skills.effects.statusPayload.applyChance bounds invalid: ${row.id}`)
          }
          if (!effect.statusPayload.applyChanceFormula || typeof effect.statusPayload.applyChanceFormula !== "object" || !Number.isFinite(effect.statusPayload.applyChanceFormula.baseAdd) || !Array.isArray(effect.statusPayload.applyChanceFormula.terms)) {
            throw new Error(`skills.effects.statusPayload.applyChanceFormula invalid: ${row.id}`)
          }
        }
      }
      if (effect.kind === "weighted_random_per_target") {
        if (!effect.statusPayload || typeof effect.statusPayload !== "object" || !Number.isFinite(effect.statusPayload.applyChanceBase) || effect.statusPayload.applyChanceBase < 0 || effect.statusPayload.applyChanceBase > 1) {
          throw new Error(`skills.effects.weighted_random_per_target.applyChanceBase invalid: ${row.id}`)
        }
        if (!Array.isArray(effect.choices) || effect.choices.length < 2) throw new Error(`skills.effects.weighted_random_per_target.choices invalid: ${row.id}`)
        const choiceIDs = new Set()
        for (const choice of effect.choices) {
          if (!choice?.id || choiceIDs.has(choice.id) || !Number.isFinite(choice.weight) || choice.weight <= 0 || !knownBuffIDs.has(choice.statusId) || !Number.isInteger(choice.durationTurns) || choice.durationTurns <= 0) {
            throw new Error(`skills.effects.weighted_random_per_target.choice invalid: ${row.id}.${choice?.id || ""}`)
          }
          choiceIDs.add(choice.id)
        }
      }
      if (effect.kind === "remove_buff") {
        if (!Array.isArray(effect.buffIds) || effect.buffIds.length === 0 || !Number.isInteger(effect.maxRemoved) || effect.maxRemoved <= 0) {
          throw new Error(`skills.effects.remove_buff invalid: ${row.id}`)
        }
        for (const buffID of effect.buffIds) {
          if (!knownBuffIDs.has(buffID)) throw new Error(`skills.effects.remove_buff buff not found: ${row.id}.${buffID}`)
        }
      }
      if (effect.kind === "adjust_skill_cooldown") {
        if (!Number.isFinite(effect.amount) || !Number.isInteger(effect.amount) || effect.amount === 0 || effect.amount < -20 || effect.amount > 20) {
          throw new Error(`skills.effects.adjust_skill_cooldown.amount invalid: ${row.id}`)
        }
        if (typeof effect.includeBasic !== "boolean") {
          throw new Error(`skills.effects.adjust_skill_cooldown.includeBasic invalid: ${row.id}`)
        }
        for (const key of ["skillIds", "excludeSkillIds"]) {
          if (!Array.isArray(effect[key]) || effect[key].some((value) => typeof value !== "string" || value.trim() === "")) {
            throw new Error(`skills.effects.adjust_skill_cooldown.${key} invalid: ${row.id}`)
          }
          for (const skillID of effect[key]) {
            if (!skillIDs.has(skillID.trim())) {
              throw new Error(`skills.effects.adjust_skill_cooldown.${key} skill not found: ${row.id}.${skillID}`)
            }
          }
        }
      }
      if (effect.kind === "summon") {
        validateSummonUnitSelector(effect.summonPayload, `skills.effects.summonPayload: ${row.id}`, knownEnemyIDs)
        if (effect.summonPayload.side !== undefined && effect.summonPayload.side !== "player" && effect.summonPayload.side !== "enemy" && effect.summonPayload.side !== "source") {
          throw new Error(`skills.effects.summonPayload.side invalid: ${row.id}`)
        }
        if (effect.summonPayload.maxCountPerUnit !== undefined && (!Number.isFinite(effect.summonPayload.maxCountPerUnit) || effect.summonPayload.maxCountPerUnit < 0 || !Number.isInteger(effect.summonPayload.maxCountPerUnit))) {
          throw new Error(`skills.effects.summonPayload.maxCountPerUnit invalid: ${row.id}`)
        }
        if (effect.summonPayload.maxCountFormula !== undefined) {
          validateSummonMaxCountFormula(effect.summonPayload.maxCountFormula, `skills.effects.summonPayload.maxCountFormula: ${row.id}`)
        }
        if (effect.summonPayload.durationTurns !== undefined && (!Number.isFinite(effect.summonPayload.durationTurns) || effect.summonPayload.durationTurns < 0 || !Number.isInteger(effect.summonPayload.durationTurns))) {
          throw new Error(`skills.effects.summonPayload.durationTurns invalid: ${row.id}`)
        }
        if (effect.summonPayload.durationFormula !== undefined) {
          if (!effect.summonPayload.durationFormula || typeof effect.summonPayload.durationFormula !== "object" || Array.isArray(effect.summonPayload.durationFormula)) {
            throw new Error(`skills.effects.summonPayload.durationFormula invalid: ${row.id}`)
          }
          if (!Number.isFinite(effect.summonPayload.durationFormula.baseAdd) || !Array.isArray(effect.summonPayload.durationFormula.terms)) {
            throw new Error(`skills.effects.summonPayload.durationFormula invalid: ${row.id}`)
          }
        }
        if (effect.summonPayload.statsFormula !== undefined && (typeof effect.summonPayload.statsFormula !== "object" || Array.isArray(effect.summonPayload.statsFormula))) {
          throw new Error(`skills.effects.summonPayload.statsFormula invalid: ${row.id}`)
        }
        if (effect.summonPayload.onCasterDeath !== undefined && effect.summonPayload.onCasterDeath !== "keep_until_duration_end" && effect.summonPayload.onCasterDeath !== "despawn_immediately") {
          throw new Error(`skills.effects.summonPayload.onCasterDeath invalid: ${row.id}`)
        }
        if (effect.summonPayload.onCapOverflow !== undefined && effect.summonPayload.onCapOverflow !== "replace_oldest" && effect.summonPayload.onCapOverflow !== "fail_new_summon") {
          throw new Error(`skills.effects.summonPayload.onCapOverflow invalid: ${row.id}`)
        }
        if (effect.summonPayload.onDispel !== undefined && effect.summonPayload.onDispel !== "despawn_immediately" && effect.summonPayload.onDispel !== "ignore_dispel") {
          throw new Error(`skills.effects.summonPayload.onDispel invalid: ${row.id}`)
        }
        if (effect.summonPayload.onDurationEnd !== undefined && effect.summonPayload.onDurationEnd !== "despawn_immediately") {
          throw new Error(`skills.effects.summonPayload.onDurationEnd invalid: ${row.id}`)
        }
      }
    }
    if (row.category && !typeIDs.has(row.category)) {
      throw new Error(`skills.category not found in skill_type: ${row.id}.${row.category}`)
    }
  }
}

function validateWeaponClasses(weaponClasses) {
  if (!Array.isArray(weaponClasses)) throw new Error("weapon_classes must be an array")
  const ids = new Set()
  for (const row of weaponClasses) {
    if (!row || typeof row !== "object") throw new Error("weapon_classes row must be object")
    if (typeof row.id !== "string" || row.id === "") throw new Error("weapon_classes.id must be non-empty string")
    if (ids.has(row.id)) throw new Error(`duplicate weapon_classes.id: ${row.id}`)
    ids.add(row.id)
    if (typeof row.nameKey !== "string" || row.nameKey === "") throw new Error(`weapon_classes.nameKey invalid: ${row.id}`)
    if (typeof row.descKey !== "string" || row.descKey === "") throw new Error(`weapon_classes.descKey invalid: ${row.id}`)
    if (!isHexColor(row.color)) throw new Error(`weapon_classes.color invalid: ${row.id}`)
  }
}

function validateEquipmentSecondaryClasses(rows) {
  if (!Array.isArray(rows)) throw new Error("equipment secondary classes must be an array")
  const ids = new Set()
  for (const row of rows) {
    if (!row || typeof row !== "object") throw new Error("equipment secondary class row must be object")
    if (typeof row.id !== "string" || row.id === "") throw new Error("equipment secondary class id must be non-empty string")
    if (ids.has(row.id)) throw new Error(`duplicate equipment secondary class id: ${row.id}`)
    ids.add(row.id)
    if (row.module !== "work" && row.module !== "battle" && row.module !== "all") {
      throw new Error(`equipment secondary class module invalid: ${row.id}`)
    }
    if (typeof row.nameKey !== "string" || row.nameKey === "") throw new Error(`equipment secondary class nameKey invalid: ${row.id}`)
    if (typeof row.descKey !== "string" || row.descKey === "") throw new Error(`equipment secondary class descKey invalid: ${row.id}`)
  }
}

function normalizeBuffDSLToken(raw) {
  return String(raw || "").trim().toLowerCase().replace(/[^a-z0-9]/g, "")
}

export function validateBuffEventFilter(filter, label, trigger) {
  if (!filter || typeof filter !== "object" || Array.isArray(filter)) throw new Error(`${label} invalid`)
  const allowedKeys = new Set([
    "probabilityKinds",
    "sourceIds",
    "turnKinds",
    "includePreparation",
    "includeSuppressedTurns"
  ])
  for (const key of Object.keys(filter)) {
    if (!allowedKeys.has(key)) throw new Error(`${label}.${key} is an unknown key`)
  }
  if (Object.keys(filter).length === 0) throw new Error(`${label} must not be empty`)

  const triggerKey = normalizeBuffDSLToken(trigger)
  const probabilityTriggers = new Set(["onbeforeprobabilitycheck", "onprobabilitysuccess", "onprobabilityfailure"])
  const turnTriggers = new Set(["onturnstart", "onturnend", "onturnsettled"])

  if (filter.probabilityKinds !== undefined) {
    if (!Array.isArray(filter.probabilityKinds) || filter.probabilityKinds.length === 0 || filter.probabilityKinds.some((kind) => kind !== "equipment_proc" && kind !== "status_apply")) {
      throw new Error(`${label}.probabilityKinds invalid`)
    }
  }
  if (filter.sourceIds !== undefined && (!Array.isArray(filter.sourceIds) || filter.sourceIds.length === 0 || filter.sourceIds.some((id) => typeof id !== "string" || !id.trim()))) {
    throw new Error(`${label}.sourceIds invalid`)
  }
  if ((filter.probabilityKinds !== undefined || filter.sourceIds !== undefined) && !probabilityTriggers.has(triggerKey)) {
    throw new Error(`${label} probability filters require a probability lifecycle trigger`)
  }

  if (filter.turnKinds !== undefined) {
    if (!Array.isArray(filter.turnKinds) || filter.turnKinds.length === 0 || filter.turnKinds.some((kind) => kind !== "ordered" && kind !== "extra")) {
      throw new Error(`${label}.turnKinds invalid`)
    }
  }
  for (const key of ["includePreparation", "includeSuppressedTurns"]) {
    if (filter[key] !== undefined && typeof filter[key] !== "boolean") throw new Error(`${label}.${key} invalid`)
  }
  if ((filter.turnKinds !== undefined || filter.includePreparation !== undefined) && !turnTriggers.has(triggerKey)) {
    throw new Error(`${label} turn filters require on_turn_start, on_turn_end, or on_turn_settled`)
  }
  if (filter.includeSuppressedTurns !== undefined && triggerKey !== "onturnend") {
    throw new Error(`${label}.includeSuppressedTurns requires on_turn_end`)
  }
}

export function validateBuffConditionTrigger(condition, trigger, label = "buff condition") {
  const conditionKey = normalizeBuffDSLToken(condition?.type || condition?.kind)
  const triggerKey = normalizeBuffDSLToken(trigger)
  if (conditionKey === "incomingdamagewouldbelethal" && triggerKey !== "onbeforetakedamage") {
    throw new Error(`${label}.incoming_damage_would_be_lethal requires on_before_take_damage`)
  }
}

function validateBuffTables(buffs, buffTypes, buffEffects, skillTypes = [], enemies = []) {
  if (!Array.isArray(buffs)) throw new Error("buff must be an array")
  if (!Array.isArray(buffTypes)) throw new Error("buff_type must be an array")
  if (!Array.isArray(buffEffects)) throw new Error("buff_effects must be an array")

  const typeIDs = new Set()
  for (const row of buffTypes) {
    if (!row || typeof row !== "object") throw new Error("buff_type row must be object")
    if (typeof row.id !== "string" || row.id === "") throw new Error("buff_type.id must be non-empty string")
    if (typeIDs.has(row.id)) throw new Error(`duplicate buff_type.id: ${row.id}`)
    typeIDs.add(row.id)
    if (typeof row.nameKey !== "string" || row.nameKey === "") throw new Error(`buff_type.nameKey invalid: ${row.id}`)
    if (typeof row.descKey !== "string" || row.descKey === "") throw new Error(`buff_type.descKey invalid: ${row.id}`)
    if (!isHexColor(row.color)) throw new Error(`buff_type.color invalid: ${row.id}`)
  }

  const buffIDs = new Set()
  for (const row of buffs) {
    if (!row || typeof row !== "object") throw new Error("buff row must be object")
    if (typeof row.id !== "string" || row.id === "") throw new Error("buff.id must be non-empty string")
    if (buffIDs.has(row.id)) throw new Error(`duplicate buff.id: ${row.id}`)
    buffIDs.add(row.id)
    if (typeof row.nameKey !== "string" || row.nameKey === "") throw new Error(`buff.nameKey invalid: ${row.id}`)
    if (typeof row.descKey !== "string" || row.descKey === "") throw new Error(`buff.descKey invalid: ${row.id}`)
    if (typeof row.durationTurns !== "number" || !Number.isFinite(row.durationTurns) || row.durationTurns < 0) {
      throw new Error(`buff.durationTurns invalid: ${row.id}`)
    }
    if (typeof row.permanent !== "boolean") throw new Error(`buff.permanent invalid: ${row.id}`)
    if (typeof row.dispellable !== "boolean") throw new Error(`buff.dispellable invalid: ${row.id}`)
    if (row.kind !== "buff" && row.kind !== "debuff") throw new Error(`buff.kind invalid: ${row.id}`)
    if (row.stackPolicy !== "refresh_duration" && row.stackPolicy !== "stack") {
      throw new Error(`buff.stackPolicy invalid: ${row.id}`)
    }
    if (row.maxStacks !== undefined) {
      if (typeof row.maxStacks !== "number" || !Number.isFinite(row.maxStacks) || row.maxStacks <= 0 || Math.floor(row.maxStacks) !== row.maxStacks) {
        throw new Error(`buff.maxStacks invalid: ${row.id}`)
      }
      if (row.stackPolicy !== "stack") {
        throw new Error(`buff.maxStacks requires stack policy: ${row.id}`)
      }
    }
    if (row.type && !typeIDs.has(row.type)) {
      throw new Error(`buff.type not found in buff_type: ${row.id}.${row.type}`)
    }
  }

  const effectIDs = new Set()
  const skillTypeIDs = new Set((Array.isArray(skillTypes) ? skillTypes : []).map((row) => String(row?.id || "").trim()).filter(Boolean))
  const enemyIDs = new Set((Array.isArray(enemies) ? enemies : []).map((row) => String(row?.id || "").trim()).filter(Boolean))
  const allowedTriggers = new Set([
    "whileactive",
    "onattach",
    "ondetach",
    "onenemyrefresh",
    "onturnstart",
    "onturnend",
    "onturnsettled",
    "onbeforetakedamage",
    "onaftertakedamage",
    "onbeforedealdamage",
    "onafterdealdamage",
    "onbeforeprobabilitycheck",
    "onprobabilitysuccess",
    "onprobabilityfailure"
  ])
  const allowedConditions = new Set([
    "selfhppercentlte",
    "selfhppercentgte",
    "sourcehppercentlte",
    "sourcehppercentgte",
    "targethppercentlte",
    "targethppercentgte",
    "selfmplt",
    "selfmpgte",
    "incomingdamagegte",
    "incomingdamagelte",
    "incomingdamagewouldbelethal",
    "incominghealgte",
    "incomingheallte",
    "hasbuff",
    "sourcehasbuff",
    "targethasbuff",
    "selfweaponclass",
    "sourceweaponclass",
    "selfposition",
    "allysidehaspositions",
    "selfchargesremaininggt",
    "selfchargesremaininggte",
    "selfchargesremaininglt",
    "selfchargesremaininglte",
    "selfbuffcountgte",
    "selfbuffcountgt",
    "selfbuffcountlte",
    "selfbuffcountlt",
    "selfbuffcounteq",
    "selfskillcooldowncountgte",
    "selfcompletedturncountmodeq",
    "waveintervaleq"
  ])
  const allowedEffects = new Set([
    "addstat",
    "addstatpercent",
    "heal",
    "damage",
    "resource",
    "addskillcooldown",
    "adjustskillcooldown",
    "addbuff",
    "removebuff",
    "consumecharge",
    "reducedamagepercent",
    "setdamagevalue",
    "setdamagemultiplier",
    "adddamagepercent",
    "adddamageflat",
    "adddamage",
    "untargetablebyenemy",
    "untargetablebyally",
    "treatallasenemy",
    "camouflagetargetingremap",
    "preferasenemytarget",
    "avoidasenemytarget",
    "preferasallytarget",
    "avoidasallytarget",
    "turnactionsuppressed",
    "skillactionsuppressed",
    "basicattacksuppressed",
    "frontlinefixation",
    "positionoverride",
    "advanceposition",
    "positionmap",
    "sideoverride",
    "invertrelationperspective",
    "overridetargeting",
    "damageredirecttosource",
    "markedtarget",
    "skipturnorder",
    "turnineligible",
    "triggersyncedsummonturn",
    "suppressmirrorcasttrigger",
    "extraenemyrefresh",
    "addprobability",
    "setprobability",
    "forceprobabilityfailure",
    "linearoutputmultiplier"
  ])
  for (const row of buffEffects) {
    if (!row || typeof row !== "object") throw new Error("buff_effects row must be object")
    if (typeof row.id !== "string" || row.id === "") throw new Error("buff_effects.id must be non-empty string")
    if (effectIDs.has(row.id)) throw new Error(`duplicate buff_effects.id: ${row.id}`)
    effectIDs.add(row.id)
    if (typeof row.buffId !== "string" || row.buffId === "") throw new Error(`buff_effects.buffId invalid: ${row.id}`)
    if (!buffIDs.has(row.buffId)) throw new Error(`buff_effects.buffId not found: ${row.id}.${row.buffId}`)
    if (typeof row.trigger !== "string" || row.trigger === "") throw new Error(`buff_effects.trigger invalid: ${row.id}`)
    const triggerKey = normalizeBuffDSLToken(row.trigger)
    if (!allowedTriggers.has(triggerKey)) {
      throw new Error(`buff_effects.trigger unsupported: ${row.id}.${row.trigger}`)
    }
    if (!Array.isArray(row.conditions)) throw new Error(`buff_effects.conditions invalid: ${row.id}`)
    if (!Array.isArray(row.effects) || row.effects.length === 0) throw new Error(`buff_effects.effects invalid: ${row.id}`)
    if (typeof row.priority !== "number" || !Number.isFinite(row.priority)) throw new Error(`buff_effects.priority invalid: ${row.id}`)
    if (row.eventFilter !== undefined) {
      validateBuffEventFilter(row.eventFilter, `buff_effects.eventFilter: ${row.id}`, triggerKey)
    }
    for (const condition of row.conditions) {
      if (!condition || typeof condition !== "object") throw new Error(`buff_effects.conditions row invalid: ${row.id}`)
      const conditionKey = normalizeBuffDSLToken(condition.type || condition.kind)
      if (conditionKey && !allowedConditions.has(conditionKey)) {
        throw new Error(`buff_effects.conditions.kind unsupported: ${row.id}.${condition.type || condition.kind}`)
      }
      validateBuffConditionTrigger(condition, triggerKey, `buff_effects.conditions: ${row.id}`)
      if (conditionKey === "selfskillcooldowncountgte") {
        const value = parseNumberStrict(condition.value)
        if (!Number.isInteger(value) || value < 1) throw new Error(`buff_effects.conditions.self_skill_cooldown_count_gte.value invalid: ${row.id}`)
        validateOptionalBoolean(condition.includeBasicAttack ?? condition.includeBasic, `buff_effects.conditions.self_skill_cooldown_count_gte.includeBasicAttack invalid: ${row.id}`)
        validateOptionalStringArray(condition.skillCategories, `buff_effects.conditions.self_skill_cooldown_count_gte.skillCategories invalid: ${row.id}`)
        for (const category of condition.skillCategories || []) {
          if (!skillTypeIDs.has(category)) throw new Error(`buff_effects.conditions.self_skill_cooldown_count_gte.skillCategories not found: ${row.id}.${category}`)
        }
      }
    }
    for (const effect of row.effects) {
      if (!effect || typeof effect !== "object") throw new Error(`buff_effects.effects row invalid: ${row.id}`)
      const effectKey = normalizeBuffDSLToken(effect.kind || effect.type)
      if (!effectKey) throw new Error(`buff_effects.effects.kind invalid: ${row.id}`)
      if (!allowedEffects.has(effectKey)) {
        throw new Error(`buff_effects.effects.kind unsupported: ${row.id}.${effect.kind || effect.type}`)
      }
      if (effectKey === "linearoutputmultiplier") {
        if (triggerKey !== "whileactive") throw new Error(`buff_effects.effects.linear_output_multiplier trigger invalid: ${row.id}`)
        if (!Array.isArray(effect.outputs) || effect.outputs.length !== 2 || new Set(effect.outputs).size !== 2 || effect.outputs.some((output) => output !== "damage" && output !== "heal")) {
          throw new Error(`buff_effects.effects.linear_output_multiplier.outputs invalid: ${row.id}`)
        }
        for (const field of ["startMultiplier", "recoveryPerChargeConsumed", "maximumMultiplier"]) {
          const value = parseNumberStrict(effect[field])
          if (!Number.isFinite(value) || value < 0 || value > 10) throw new Error(`buff_effects.effects.linear_output_multiplier.${field} invalid: ${row.id}`)
        }
        if (effect.recoveryPerChargeConsumed <= 0 || effect.maximumMultiplier < effect.startMultiplier) {
          throw new Error(`buff_effects.effects.linear_output_multiplier curve invalid: ${row.id}`)
        }
        for (const field of ["startMultiplierPayloadKey", "recoveryPerChargeConsumedPayloadKey", "maximumMultiplierPayloadKey"]) {
          if (typeof effect[field] !== "string" || !effect[field].trim()) throw new Error(`buff_effects.effects.linear_output_multiplier.${field} invalid: ${row.id}`)
        }
      }
      if (effectKey === "resource") {
        if (effect.resource !== "hp" && effect.resource !== "mp") {
          throw new Error(`buff_effects.effects.resource.resource invalid: ${row.id}`)
        }
        if (effect.valueScale !== undefined && effect.valueScale !== "max_resource") {
          throw new Error(`buff_effects.effects.resource.valueScale invalid: ${row.id}`)
        }
        if (effect.minimumHp !== undefined) {
          const minimumHp = parseNumberStrict(effect.minimumHp)
          if (effect.resource !== "hp" || !Number.isFinite(minimumHp) || minimumHp < 0 || minimumHp > 1e12) {
            throw new Error(`buff_effects.effects.resource.minimumHp invalid: ${row.id}`)
          }
        }
      }
      if ((triggerKey === "onbeforeprobabilitycheck" || triggerKey === "onprobabilitysuccess" || triggerKey === "onprobabilityfailure") && probabilityLifecycleProcChanceFields(effect).length > 0) {
        throw new Error(`buff_effects probability lifecycle cannot contain procChance: ${row.id}`)
      }
      if (effectKey === "positionmap") {
        if (triggerKey !== "whileactive" || !effect.mapping || typeof effect.mapping !== "object" || Array.isArray(effect.mapping)) throw new Error(`buff_effects.effects.position_map invalid: ${row.id}`)
        const positions = ["vanguard", "midguard", "rearguard"]
        if (positions.some((position) => !positions.includes(effect.mapping[position]))) throw new Error(`buff_effects.effects.position_map.mapping invalid: ${row.id}`)
        if (effect.priority !== undefined && (!Number.isFinite(effect.priority) || !Number.isInteger(effect.priority))) throw new Error(`buff_effects.effects.position_map.priority invalid: ${row.id}`)
      }
      if (effectKey === "addprobability" || effectKey === "setprobability") {
        if (triggerKey !== "onbeforeprobabilitycheck" || !Number.isFinite(effect.value)) throw new Error(`buff_effects.effects.${effect.kind || effect.type}.value invalid: ${row.id}`)
        if (effectKey === "setprobability" && (effect.value < 0 || effect.value > 1)) throw new Error(`buff_effects.effects.set_probability.value invalid: ${row.id}`)
      }
      if (effectKey === "forceprobabilityfailure" && triggerKey !== "onbeforeprobabilitycheck") throw new Error(`buff_effects.effects.force_probability_failure trigger invalid: ${row.id}`)
      if (effectKey === "addskillcooldown" || effectKey === "adjustskillcooldown") {
        const amount = parseNumberStrict(effect.amount ?? effect.turns ?? effect.value)
        if (!Number.isInteger(amount) || amount === 0 || amount > 20 || (effectKey === "addskillcooldown" && amount < 0) || amount < -20) {
          throw new Error(`buff_effects.effects.${effect.kind || effect.type}.amount invalid: ${row.id}`)
        }
        if (effect.includeBasic !== undefined && typeof effect.includeBasic !== "boolean") {
          throw new Error(`buff_effects.effects.${effect.kind || effect.type}.includeBasic invalid: ${row.id}`)
        }
        validateOptionalStringArray(effect.skillCategories, `buff_effects.effects.${effect.kind || effect.type}.skillCategories invalid: ${row.id}`)
        for (const category of effect.skillCategories || []) {
          if (!skillTypeIDs.has(category)) throw new Error(`buff_effects.effects.${effect.kind || effect.type}.skillCategories not found: ${row.id}.${category}`)
        }
        if (effect.selectionMode !== undefined && effect.selectionMode !== "all" && effect.selectionMode !== "max_remaining") {
          throw new Error(`buff_effects.effects.${effect.kind || effect.type}.selectionMode invalid: ${row.id}`)
        }
        if (effect.maxSkills !== undefined && (!Number.isInteger(effect.maxSkills) || effect.maxSkills < 0)) {
          throw new Error(`buff_effects.effects.${effect.kind || effect.type}.maxSkills invalid: ${row.id}`)
        }
      }
      if (effectKey === "triggersyncedsummonturn") {
        validateOptionalStringArray(effect.includeSummonTemplateIds, `buff_effects.effects.trigger_synced_summon_turn.includeSummonTemplateIds invalid: ${row.id}`)
        for (const templateID of effect.includeSummonTemplateIds || []) {
          if (!enemyIDs.has(templateID)) throw new Error(`buff_effects.effects.trigger_synced_summon_turn summon template not found: ${row.id}.${templateID}`)
        }
      }
      if (effectKey === "extraenemyrefresh") {
        if (triggerKey !== "onenemyrefresh") throw new Error(`buff_effects.effects.extra_enemy_refresh trigger invalid: ${row.id}`)
        if (effect.enemyPool !== "current_world") throw new Error(`buff_effects.effects.extra_enemy_refresh.enemyPool invalid: ${row.id}`)
        if (!Number.isInteger(effect.count) || effect.count < 1 || effect.count > 20) {
          throw new Error(`buff_effects.effects.extra_enemy_refresh.count invalid: ${row.id}`)
        }
      }
      if (effectKey === "damage") {
        if (effect.value !== undefined && !Number.isFinite(effect.value)) {
          throw new Error(`buff_effects.effects.damage.value invalid: ${row.id}`)
        }
        if (effect.amount !== undefined && !Number.isFinite(effect.amount)) {
          throw new Error(`buff_effects.effects.damage.amount invalid: ${row.id}`)
        }
        if (effect.amountFormula !== undefined) {
          if (!effect.amountFormula || typeof effect.amountFormula !== "object" || Array.isArray(effect.amountFormula)) {
            throw new Error(`buff_effects.effects.damage.amountFormula invalid: ${row.id}`)
          }
          if (effect.amountFormula.baseAdd !== undefined && !Number.isFinite(effect.amountFormula.baseAdd)) {
            throw new Error(`buff_effects.effects.damage.amountFormula.baseAdd invalid: ${row.id}`)
          }
          if (effect.amountFormula.terms !== undefined) {
            if (!Array.isArray(effect.amountFormula.terms)) {
              throw new Error(`buff_effects.effects.damage.amountFormula.terms invalid: ${row.id}`)
            }
            for (const term of effect.amountFormula.terms) {
              if (!term || typeof term !== "object" || Array.isArray(term)) {
                throw new Error(`buff_effects.effects.damage.amountFormula term invalid: ${row.id}`)
              }
              if (typeof term.attrKey !== "string" || term.attrKey.trim() === "") {
                throw new Error(`buff_effects.effects.damage.amountFormula.attrKey invalid: ${row.id}`)
              }
              if (!Number.isFinite(term.coefficient)) {
                throw new Error(`buff_effects.effects.damage.amountFormula.coefficient invalid: ${row.id}`)
              }
              if (term.perSkillLevel !== undefined && !Number.isFinite(term.perSkillLevel)) {
                throw new Error(`buff_effects.effects.damage.amountFormula.perSkillLevel invalid: ${row.id}`)
              }
            }
          }
        }
        const hasFlatAmount = Number.isFinite(effect.value) || Number.isFinite(effect.amount)
        const hasFormula = !!effect.amountFormula
        if (!hasFlatAmount && !hasFormula) {
          throw new Error(`buff_effects.effects.damage requires value/amount or amountFormula: ${row.id}`)
        }
      }
    }
  }
}

function validateItemTypes(items, itemTypes) {
  if (!Array.isArray(itemTypes)) throw new Error("item_types must be an array")
  const typeSet = new Set()
  for (const row of itemTypes) {
    if (!row || typeof row !== "object") throw new Error("item_types row must be object")
    if (!row.id || typeof row.id !== "string") throw new Error("item_types.id must be non-empty string")
    if (typeSet.has(row.id)) throw new Error(`duplicate item_types.id: ${row.id}`)
    typeSet.add(row.id)
    for (const keyName of ["nameKey", "descKey"]) {
      const keyValue = row[keyName]
      if (keyValue === undefined || keyValue === null || keyValue === "") continue
      if (typeof keyValue !== "string") throw new Error(`item_types.${keyName} must be string: ${row.id}`)
    }
  }
  if (typeSet.size === 0) return
  for (const item of items) {
    for (const type of item.type || []) {
      if (!typeSet.has(type)) {
        throw new Error(`item.type not found in item_types: ${item.id}.${type}`)
      }
    }
  }
}

function validateAttributes(attributes, attributeExpProfiles) {
  if (!Array.isArray(attributes)) throw new Error("attributes must be an array")
  const idRegex = /^[a-z][a-z0-9_]*$/
  const allowedCategories = new Set(["work", "battle", "other"])
  const allowedUnits = new Set(["number", "percent"])
  const allowedGrowthTypes = new Set(["levelable", "static"])
  const relatedActionEffectRegex = /^[a-z][a-zA-Z0-9_]*$/
  const ids = new Set()
  const orders = new Set()
  const expProfileIDs = new Set(
    (attributeExpProfiles || [])
      .map((row) => (typeof row?.id === "string" ? row.id.trim() : ""))
      .filter(Boolean)
  )

  for (const row of attributes) {
    if (!row || typeof row !== "object") throw new Error("attributes row must be object")
    const id = typeof row.id === "string" ? row.id.trim() : ""
    if (!id) throw new Error("attributes.id is required")
    if (!idRegex.test(id)) throw new Error(`attributes.id must be snake_case: ${id}`)
    if (ids.has(id)) throw new Error(`duplicate attributes.id: ${id}`)
    ids.add(id)

    const nameKey = typeof row.nameKey === "string" ? row.nameKey.trim() : ""
    const descKey = typeof row.descKey === "string" ? row.descKey.trim() : ""
    if (nameKey !== `attr.${id}.name`) throw new Error(`attributes.nameKey invalid for ${id}; expected attr.${id}.name`)
    if (descKey !== `attr.${id}.desc`) throw new Error(`attributes.descKey invalid for ${id}; expected attr.${id}.desc`)

    const icon = typeof row.icon === "string" ? row.icon.trim() : ""
    if (row.icon !== undefined) {
      if (typeof row.icon !== "string" || icon === "") throw new Error(`attributes.icon invalid for ${id}`)
      if (!icon.startsWith("game-icons:")) throw new Error(`attributes.icon must use game-icons prefix for ${id}`)
    }

    const category = typeof row.category === "string" ? row.category.trim() : ""
    if (!allowedCategories.has(category)) throw new Error(`attributes.category invalid for ${id}: ${category}`)

    let unit = typeof row.unit === "string" ? row.unit.trim() : ""
    if (!unit) unit = "number"
    if (!allowedUnits.has(unit)) throw new Error(`attributes.unit invalid for ${id}: ${unit}`)

    const growthType = typeof row.growthType === "string" ? row.growthType.trim() : ""
    if (!allowedGrowthTypes.has(growthType)) throw new Error(`attributes.growthType invalid for ${id}: ${growthType}`)

    const order = parseNumber(row.order, Number.NaN)
    if (!Number.isFinite(order)) throw new Error(`attributes.order invalid for ${id}`)
    if (orders.has(order)) throw new Error(`duplicate attributes.order: ${order}`)
    orders.add(order)

    const baseDefault = parseNumber(row.baseDefault, Number.NaN)
    if (!Number.isFinite(baseDefault)) throw new Error(`attributes.baseDefault invalid for ${id}`)

    const expProfileID = typeof row.expProfileId === "string" ? row.expProfileId.trim() : ""
    if (growthType === "levelable") {
      if (!expProfileID) throw new Error(`attributes.expProfileId required for levelable attr: ${id}`)
      if (expProfileIDs.size > 0 && !expProfileIDs.has(expProfileID)) {
        // 允许在迁移期先配置 attributes，再补全 profile 表；仅在 profile 表已存在并声明 id 时做强校验。
      }
    } else if (expProfileID) {
      throw new Error(`attributes.expProfileId must be empty for static attr: ${id}`)
    }

    const relatedActionMainType = parseOptionalString(row.relatedActionMainType) || ""
    const relatedActionEffect = parseOptionalString(row.relatedActionEffect) || ""
    if (relatedActionMainType && !idRegex.test(relatedActionMainType)) {
      throw new Error(`attributes.relatedActionMainType invalid for ${id}: ${relatedActionMainType}`)
    }
    if (relatedActionEffect && !relatedActionEffectRegex.test(relatedActionEffect)) {
      throw new Error(`attributes.relatedActionEffect invalid for ${id}: ${relatedActionEffect}`)
    }
  }
}

function parseIDList(raw) {
  if (Array.isArray(raw)) {
    return raw.map((item) => String(item).trim()).filter(Boolean)
  }
  if (typeof raw === "string" && raw.trim() !== "") {
    if (raw.trim().startsWith("[")) {
      try {
        const parsed = JSON.parse(raw)
        if (Array.isArray(parsed)) {
          return parsed.map((item) => String(item).trim()).filter(Boolean)
        }
      } catch {
        // fallback to comma-separated parser
      }
    }
    return raw
      .split(",")
      .map((part) => part.trim())
      .filter(Boolean)
  }
  return []
}

function parseWorldSpecialWaves(raw) {
  const fallback = { fixedWaves: [], periodic: null, normalized: [] }
  if (Array.isArray(raw)) {
    const fixed = Array.from(
      new Set(
        raw
          .map((item) => parseNumberStrict(item))
          .filter(Number.isFinite)
          .map((item) => Math.trunc(item))
          .filter((item) => item > 0)
      )
    ).sort((a, b) => a - b)
    return { fixedWaves: fixed, periodic: null, normalized: fixed }
  }
  if (typeof raw === "string" && raw.trim() !== "") {
    const trimmed = raw.trim()
    if (trimmed.startsWith("{") || trimmed.startsWith("[")) {
      try {
        return parseWorldSpecialWaves(JSON.parse(trimmed))
      } catch {
        return fallback
      }
    }
    const fixed = Array.from(
      new Set(
        trimmed
          .split(",")
          .map((item) => parseNumberStrict(item.trim()))
          .filter(Number.isFinite)
          .map((item) => Math.trunc(item))
          .filter((item) => item > 0)
      )
    ).sort((a, b) => a - b)
    return { fixedWaves: fixed, periodic: null, normalized: fixed }
  }
  if (!raw || typeof raw !== "object" || Array.isArray(raw)) {
    return fallback
  }
  const node = raw
  const every = parseNumberStrict(node.every)
  const minRaw = parseNumberStrict(node.minCount)
  const maxRaw = parseNumberStrict(node.maxCount)
  if (!Number.isFinite(every) || Math.trunc(every) <= 0) {
    return fallback
  }
  const minCount = Number.isFinite(minRaw) ? Math.trunc(minRaw) : 1
  const maxCount = Number.isFinite(maxRaw) ? Math.trunc(maxRaw) : minCount
  const periodic = {
    every: Math.trunc(every),
    minCount: Math.max(1, minCount),
    maxCount: Math.max(Math.max(1, minCount), maxCount)
  }
  return { fixedWaves: [], periodic, normalized: periodic }
}

function normalizeWorldWaveSchedules(raw, worldID) {
  if (!Array.isArray(raw)) return []
  return raw.map((entry) => {
    const id = parseOptionalString(entry?.id) || ""
    const enemyChoices = Array.isArray(entry?.enemyChoices)
      ? entry.enemyChoices.map((choice) => ({
          enemyId: parseOptionalString(choice?.enemyId) || "",
          weight: parseNumber(choice?.weight, 0)
        }))
      : []
    return {
      id,
      nameKey: parseOptionalString(entry?.nameKey) || (id ? `world.${worldID}.waveSchedule.${id}.name` : ""),
      descKey: parseOptionalString(entry?.descKey) || (id ? `world.${worldID}.waveSchedule.${id}.desc` : ""),
      every: Math.trunc(parseNumber(entry?.every, 0)),
      priority: Math.trunc(parseNumber(entry?.priority, 0)),
      enemyChoices,
      minCount: Math.trunc(parseNumber(entry?.minCount, 0)),
      maxCount: Math.trunc(parseNumber(entry?.maxCount, 0))
    }
  })
}

function parseWorldEnemyCountRange(raw, fallbackMin = 0, fallbackMax = fallbackMin) {
  if (raw === undefined || raw === null || raw === "") {
    return { min: fallbackMin, max: fallbackMax }
  }
  const numeric = parseNumberStrict(raw)
  if (Number.isFinite(numeric)) {
    const value = Math.trunc(numeric)
    return { min: value, max: value }
  }
  let node = raw
  if (typeof raw === "string" && raw.trim() !== "") {
    try {
      node = JSON.parse(raw)
    } catch {
      return { min: Number.NaN, max: Number.NaN }
    }
  }
  if (!node || typeof node !== "object" || Array.isArray(node)) {
    return { min: Number.NaN, max: Number.NaN }
  }
  const min = parseNumberStrict(node.min)
  const max = parseNumberStrict(node.max)
  if (!Number.isFinite(min) || !Number.isFinite(max)) {
    return { min: Number.NaN, max: Number.NaN }
  }
  return { min: Math.trunc(min), max: Math.trunc(max) }
}

function normalizeBattleDropItems(raw) {
  const result = {}
  if (!raw || typeof raw !== "object" || Array.isArray(raw)) {
    return result
  }
  for (const [itemID, payload] of Object.entries(raw)) {
    const normalizedItemID = String(itemID || "").trim()
    if (!normalizedItemID || !payload || typeof payload !== "object" || Array.isArray(payload)) continue
    const percentRaw = parseNumberStrict(payload.percent)
    const rangeNode = payload.range && typeof payload.range === "object" && !Array.isArray(payload.range)
      ? payload.range
      : payload
    const min = parseNumberStrict(rangeNode?.min)
    const max = parseNumberStrict(rangeNode?.max)
    if (!Number.isFinite(percentRaw) || !Number.isFinite(min) || !Number.isFinite(max)) continue
    result[normalizedItemID] = {
      min: Math.trunc(min),
      max: Math.trunc(max),
      percent: percentRaw
    }
  }
  return result
}

function normalizeWorldCustomLineupConf(raw, worldID) {
  const input = (() => {
    if (Array.isArray(raw)) return raw
    if (typeof raw === "string" && raw.trim() !== "") {
      try {
        const parsed = JSON.parse(raw)
        return Array.isArray(parsed) ? parsed : []
      } catch {
        return []
      }
    }
    return []
  })()
  return input.map((entry) => {
    const id = typeof entry?.id === "string" ? entry.id.trim() : ""
    const fallbackBase = id && worldID ? `world.${worldID}.customLineup.${id}` : ""
    return {
      id,
      nameKey: parseOptionalString(entry?.nameKey) || (fallbackBase ? `${fallbackBase}.name` : ""),
      descKey: parseOptionalString(entry?.descKey) || (fallbackBase ? `${fallbackBase}.desc` : ""),
      percent: parseNumber(entry?.percent, 0),
      isSpecialWaveLineup: parseBoolean(entry?.isSpecialWaveLineup, false),
      lineup: parseIDList(entry?.lineup),
      drops: {
        items: normalizeBattleDropItems(entry?.drops?.items)
      }
    }
  })
}

function validateBattleDropItems(items, source) {
  if (!items || typeof items !== "object" || Array.isArray(items)) {
    throw new Error(`${source} invalid`)
  }
  for (const [itemID, payload] of Object.entries(items)) {
    if (!itemID || typeof itemID !== "string") {
      throw new Error(`${source} item key invalid`)
    }
    if (!payload || typeof payload !== "object" || Array.isArray(payload)) {
      throw new Error(`${source} payload invalid: ${itemID}`)
    }
    const percent = parseNumberStrict(payload.percent)
    if (!Number.isFinite(percent) || percent < 0 || percent > 1) {
      throw new Error(`${source}.percent invalid: ${itemID}`)
    }
    const min = parseNumberStrict(payload.min)
    const max = parseNumberStrict(payload.max)
    if (!Number.isFinite(min) || !Number.isFinite(max) || min < 0 || max < min) {
      throw new Error(`${source}.range invalid: ${itemID}`)
    }
  }
}

function validateEnemyDropItems(items, enemyID) {
  if (!items || typeof items !== "object" || Array.isArray(items)) {
    throw new Error(`enemies.drops invalid for ${enemyID}`)
  }
  for (const [itemID, payload] of Object.entries(items)) {
    if (!itemID || typeof itemID !== "string") throw new Error(`enemies.drops item key invalid: ${enemyID}`)
    if (!payload || typeof payload !== "object" || Array.isArray(payload)) {
      throw new Error(`enemies.drops payload invalid: ${enemyID}.${itemID}`)
    }
    const percent = parseNumberStrict(payload.percent)
    if (!Number.isFinite(percent) || percent < 0 || percent > 1) {
      throw new Error(`enemies.drops.percent invalid: ${enemyID}.${itemID}`)
    }
    const range = payload.range
    if (!range || typeof range !== "object" || Array.isArray(range)) {
      throw new Error(`enemies.drops.range invalid: ${enemyID}.${itemID}`)
    }
    const min = parseNumberStrict(range.min)
    const max = parseNumberStrict(range.max)
    if (!Number.isFinite(min) || !Number.isFinite(max) || min < 0 || max < min) {
      throw new Error(`enemies.drops.range invalid: ${enemyID}.${itemID}`)
    }
  }
}

function validateEnemyMaterialDropTypes(items, enemies) {
  const itemByID = new Map((items || []).map((item) => [item.id, item]))
  for (const enemy of enemies || []) {
    for (const itemID of Object.keys(enemy?.drops || {})) {
      const item = itemByID.get(itemID)
      if (!item?.type?.includes("material")) continue
      if (item.type[0] !== "loot") {
        throw new Error(`enemy material drop must list loot as primary item type: ${enemy.id}.${itemID}`)
      }
    }
  }
}

function validateEnemyEquipment(rows, equipmentTemplates = [], affixDefinitions = [], rarityDefinitions = []) {
  const templateByID = new Map(
    (equipmentTemplates || [])
      .filter((row) => row && typeof row === "object" && typeof row.id === "string" && row.id.trim())
      .map((row) => [row.id.trim(), row])
  )
  const knownAffixIDs = new Set(
    (affixDefinitions || [])
      .filter((row) => row && typeof row === "object" && row.active !== false)
      .map((row) => (typeof row.id === "string" ? row.id.trim() : ""))
      .filter(Boolean)
  )
  const knownRarityIDs = new Set(
    (rarityDefinitions || [])
      .filter((row) => row && typeof row === "object")
      .map((row) => (typeof row.id === "string" ? row.id.trim() : ""))
      .filter(Boolean)
  )
  for (const row of rows) {
    const enemyID = typeof row?.id === "string" ? row.id.trim() : ""
    if (row.equipment === undefined || row.equipment === null) continue
    if (!Array.isArray(row.equipment)) {
      throw new Error(`enemies.equipment invalid for ${enemyID}: must be array`)
    }
    const occupiedSlots = new Map()
    for (let index = 0; index < row.equipment.length; index++) {
      const entry = row.equipment[index]
      if (!entry || typeof entry !== "object" || Array.isArray(entry)) {
        throw new Error(`enemies.equipment entry invalid for ${enemyID}[${index}]`)
      }
      const templateItemID = typeof entry.templateItemId === "string" ? entry.templateItemId.trim() : ""
      if (!templateItemID) {
        throw new Error(`enemies.equipment.templateItemId required for ${enemyID}[${index}]`)
      }
      const template = templateByID.get(templateItemID)
      if (!template) {
        throw new Error(`enemies.equipment.templateItemId not found: ${enemyID}.${templateItemID}`)
      }
      const equipType = typeof entry.equipType === "string" && entry.equipType.trim() ? entry.equipType.trim() : template.equipType
      if (equipType !== "battle" && equipType !== "work") {
        throw new Error(`enemies.equipment.equipType invalid: ${enemyID}.${templateItemID}`)
      }
      const enhanceLevel = parseNumberStrict(entry.enhanceLevel ?? 0)
      if (!Number.isFinite(enhanceLevel) || !Number.isInteger(enhanceLevel) || enhanceLevel < 0) {
        throw new Error(`enemies.equipment.enhanceLevel invalid: ${enemyID}.${templateItemID}`)
      }
      if (Number.isFinite(template.enhanceMaxLevel) && enhanceLevel > template.enhanceMaxLevel) {
        throw new Error(`enemies.equipment.enhanceLevel exceeds max: ${enemyID}.${templateItemID}`)
      }
      const slotText = typeof entry.equippedSlot === "string" && entry.equippedSlot.trim()
        ? entry.equippedSlot.trim()
        : template.slot
      const slots = parseIDList(slotText)
      if (slots.length === 0) {
        throw new Error(`enemies.equipment.equippedSlot invalid: ${enemyID}.${templateItemID}`)
      }
      for (const slot of slots) {
        if (occupiedSlots.has(slot)) {
          throw new Error(`enemies.equipment duplicate slot: ${enemyID}.${slot}.${occupiedSlots.get(slot)}.${templateItemID}`)
        }
        occupiedSlots.set(slot, templateItemID)
      }
      if (entry.baseSnapshot !== undefined) {
        if (!entry.baseSnapshot || typeof entry.baseSnapshot !== "object" || Array.isArray(entry.baseSnapshot)) {
          throw new Error(`enemies.equipment.baseSnapshot invalid: ${enemyID}.${templateItemID}`)
        }
        for (const [attrID, value] of Object.entries(entry.baseSnapshot)) {
          if (!attrID || !Number.isFinite(parseNumberStrict(value))) {
            throw new Error(`enemies.equipment.baseSnapshot value invalid: ${enemyID}.${templateItemID}.${attrID}`)
          }
        }
      }
      const affixes = Array.isArray(entry.affixes) ? entry.affixes : []
      for (const affix of affixes) {
        if (!affix || typeof affix !== "object" || Array.isArray(affix)) {
          throw new Error(`enemies.equipment.affixes entry invalid: ${enemyID}.${templateItemID}`)
        }
        if (!Number.isFinite(parseNumberStrict(affix.slotIndex)) || parseNumberStrict(affix.slotIndex) < 0) {
          throw new Error(`enemies.equipment.affixes.slotIndex invalid: ${enemyID}.${templateItemID}`)
        }
        const affixID = typeof affix.affixId === "string" ? affix.affixId.trim() : ""
        if (!affixID || !knownAffixIDs.has(affixID)) {
          throw new Error(`enemies.equipment.affixes.affixId invalid: ${enemyID}.${templateItemID}.${affixID}`)
        }
        const rarityID = typeof affix.rarityId === "string" ? affix.rarityId.trim() : ""
        if (rarityID && !knownRarityIDs.has(rarityID)) {
          throw new Error(`enemies.equipment.affixes.rarityId invalid: ${enemyID}.${templateItemID}.${rarityID}`)
        }
      }
    }
  }
}

function validateEnemies(rows, skills = [], equipmentTemplates = [], affixDefinitions = [], rarityDefinitions = []) {
  if (!Array.isArray(rows)) throw new Error("enemies must be an array")
  const validSkillIDs = new Set(
    (skills || [])
      .map((row) => (typeof row?.id === "string" ? row.id.trim() : ""))
      .filter(Boolean)
  )
  const ids = new Set()
  const allowedPositions = new Set(["vanguard", "midguard", "rearguard"])
  for (const row of rows) {
    if (!row || typeof row !== "object") throw new Error("enemies row must be object")
    if (row.disabled !== undefined && typeof row.disabled !== "boolean") throw new Error("enemies.disabled invalid")
    const id = typeof row.id === "string" ? row.id.trim() : ""
    if (!id) throw new Error("enemies.id is required")
    if (ids.has(id)) throw new Error(`duplicate enemies.id: ${id}`)
    ids.add(id)

    const nameKey = typeof row.nameKey === "string" ? row.nameKey.trim() : ""
    const descKey = typeof row.descKey === "string" ? row.descKey.trim() : ""
    if (nameKey !== `enemy.${id}.name`) throw new Error(`enemies.nameKey invalid for ${id}; expected enemy.${id}.name`)
    if (descKey !== `enemy.${id}.desc`) throw new Error(`enemies.descKey invalid for ${id}; expected enemy.${id}.desc`)

    const position = typeof row.position === "string" ? row.position.trim().toLowerCase() : ""
    if (!allowedPositions.has(position)) {
      throw new Error(`enemies.position invalid for ${id}: ${position}`)
    }
    if (row.level !== undefined && row.level !== null && row.level !== "") {
      const level = parseNumberStrict(row.level)
      if (!Number.isFinite(level) || !Number.isInteger(level) || level < 1) {
        throw new Error(`enemies.level invalid for ${id}`)
      }
    }
    if (row.type !== undefined && row.type !== null && row.type !== "") {
      const type = typeof row.type === "string" ? row.type.trim().toLowerCase() : ""
      if (type !== "normal" && type !== "elite" && type !== "boss") {
        throw new Error(`enemies.type invalid for ${id}: ${type}`)
      }
    }
    const exp = parseNumberStrict(row.exp)
    if (!Number.isFinite(exp) || exp < 0) throw new Error(`enemies.exp invalid for ${id}`)

    validateEnemyDropItems(row.drops, id)

    if (!Array.isArray(row.skillIds)) {
      throw new Error(`enemies.skillIds invalid for ${id}: must be array`)
    }
    for (const skillIDRaw of row.skillIds) {
      if (typeof skillIDRaw !== "string" || !skillIDRaw.trim()) {
        throw new Error(`enemies.skillIds entry invalid for ${id}`)
      }
      const skillID = skillIDRaw.trim()
      if (validSkillIDs.size > 0 && !validSkillIDs.has(skillID)) {
        throw new Error(`enemies.skillIds skill not found: ${id}.${skillID}`)
      }
    }
  }
  validateEnemyEquipment(rows, equipmentTemplates, affixDefinitions, rarityDefinitions)
}

function validateWorldBattlefieldEffects(raw, worldID, validBuffIDs = new Set(), validSkillIDs = new Set()) {
  if (raw === undefined || raw === null) return
  if (!Array.isArray(raw)) throw new Error(`world.battlefieldEffects invalid: ${worldID}`)
  const effectIDs = new Set()
  for (const effect of raw) {
    if (!effect || typeof effect !== "object" || Array.isArray(effect)) {
      throw new Error(`world.battlefieldEffects entry invalid: ${worldID}`)
    }
    const effectID = parseOptionalString(effect.id) || ""
    if (!effectID) throw new Error(`world.battlefieldEffects.id required: ${worldID}`)
    if (effectIDs.has(effectID)) throw new Error(`duplicate world.battlefieldEffects.id: ${worldID}.${effectID}`)
    effectIDs.add(effectID)
    if (effect.nameKey !== `battlefield_effect.${effectID}.name`) {
      throw new Error(`world.battlefieldEffects.nameKey invalid: ${worldID}.${effectID}`)
    }
    if (effect.descKey !== `battlefield_effect.${effectID}.desc`) {
      throw new Error(`world.battlefieldEffects.descKey invalid: ${worldID}.${effectID}`)
    }
    if (effect.displayMode !== undefined && effect.displayMode !== "countdown" && effect.displayMode !== "progress") {
      throw new Error(`world.battlefieldEffects.displayMode invalid: ${worldID}.${effectID}`)
    }

    const counter = effect.counter
    if (!counter || typeof counter !== "object" || Array.isArray(counter)) {
      throw new Error(`world.battlefieldEffects.counter invalid: ${worldID}.${effectID}`)
    }
    if (counter.source !== "turn_consumed") {
      throw new Error(`world.battlefieldEffects.counter.source invalid: ${worldID}.${effectID}`)
    }
    if (counter.aggregation !== "per_event" && counter.aggregation !== "alive_roster_cycle") {
      throw new Error(`world.battlefieldEffects.counter.aggregation invalid: ${worldID}.${effectID}`)
    }
    for (const [field, value] of [["initial", counter.initial], ["resetTo", counter.resetTo]]) {
      const parsed = parseNumberStrict(value)
      if (!Number.isFinite(parsed) || !Number.isInteger(parsed) || parsed <= 0) {
        throw new Error(`world.battlefieldEffects.counter.${field} invalid: ${worldID}.${effectID}`)
      }
    }
    if (typeof counter.countSuppressedTurns !== "boolean" || typeof counter.carryProgressAcrossWaves !== "boolean") {
      throw new Error(`world.battlefieldEffects.counter flags invalid: ${worldID}.${effectID}`)
    }
    const actorFilter = counter.actorFilter
    if (!actorFilter || typeof actorFilter !== "object" || Array.isArray(actorFilter)) {
      throw new Error(`world.battlefieldEffects.counter.actorFilter invalid: ${worldID}.${effectID}`)
    }
    const sides = parseIDList(actorFilter.sides)
    const unitTypes = parseIDList(actorFilter.unitTypes)
    const turnKinds = parseIDList(actorFilter.turnKinds)
    if (sides.length === 0 || sides.some((side) => side !== "player" && side !== "enemy")) {
      throw new Error(`world.battlefieldEffects.counter.actorFilter.sides invalid: ${worldID}.${effectID}`)
    }
    if (unitTypes.length === 0 || unitTypes.some((unitType) => !["player", "kitty", "enemy", "summon"].includes(unitType))) {
      throw new Error(`world.battlefieldEffects.counter.actorFilter.unitTypes invalid: ${worldID}.${effectID}`)
    }
    if (turnKinds.length === 0 || turnKinds.some((turnKind) => turnKind !== "ordered" && turnKind !== "extra")) {
      throw new Error(`world.battlefieldEffects.counter.actorFilter.turnKinds invalid: ${worldID}.${effectID}`)
    }
    if (typeof actorFilter.includeSummons !== "boolean" || typeof actorFilter.includePreparation !== "boolean") {
      throw new Error(`world.battlefieldEffects.counter.actorFilter flags invalid: ${worldID}.${effectID}`)
    }

    if (!Array.isArray(effect.triggerEffects) || effect.triggerEffects.length === 0) {
      throw new Error(`world.battlefieldEffects.triggerEffects required: ${worldID}.${effectID}`)
    }
    for (const triggerEffect of effect.triggerEffects) {
      if (triggerEffect?.kind !== "damage" && triggerEffect?.kind !== "weighted_random_per_target") {
        throw new Error(`world.battlefieldEffects.triggerEffects type invalid: ${worldID}.${effectID}`)
      }
      if (triggerEffect.kind === "damage") {
        if (triggerEffect.targetScope !== "all_alive_players") throw new Error(`world.battlefieldEffects.triggerEffects.targetScope invalid: ${worldID}.${effectID}`)
        if (triggerEffect.damageType !== "physical" && triggerEffect.damageType !== "magical") throw new Error(`world.battlefieldEffects.triggerEffects.damageType invalid: ${worldID}.${effectID}`)
        const curve = triggerEffect.amountByDifficulty
        const base = parseNumberStrict(curve?.base)
        const perLevelMult = parseNumberStrict(curve?.perLevelMult)
        if (!Number.isFinite(base) || base <= 0 || !Number.isFinite(perLevelMult) || perLevelMult < 0) throw new Error(`world.battlefieldEffects.triggerEffects.amountByDifficulty invalid: ${worldID}.${effectID}`)
        if (typeof triggerEffect.includeSummons !== "boolean") throw new Error(`world.battlefieldEffects.triggerEffects.includeSummons invalid: ${worldID}.${effectID}`)
        continue
      }
      if (triggerEffect.targetScope !== "alive_players" || triggerEffect.selection !== "uniform_without_replacement") {
        throw new Error(`world.battlefieldEffects.triggerEffects.random targeting invalid: ${worldID}.${effectID}`)
      }
      const targetFilter = triggerEffect.targetFilter
      const targetUnitTypes = parseIDList(targetFilter?.unitTypes)
      if (!targetFilter || typeof targetFilter !== "object" || Array.isArray(targetFilter) || typeof targetFilter.includeSummons !== "boolean" || targetUnitTypes.length === 0 || targetUnitTypes.some((unitType) => !["player", "kitty", "summon"].includes(unitType))) {
        throw new Error(`world.battlefieldEffects.triggerEffects.targetFilter invalid: ${worldID}.${effectID}`)
      }
      const targetCount = triggerEffect.targetCount
      if (!targetCount || targetCount.source !== "eligible_count" || !["ceil", "floor", "round"].includes(targetCount.rounding)) {
        throw new Error(`world.battlefieldEffects.triggerEffects.targetCount invalid: ${worldID}.${effectID}`)
      }
      for (const field of ["divisor", "min", "max"]) {
        const value = parseNumberStrict(targetCount[field])
        if (!Number.isInteger(value) || value <= 0) throw new Error(`world.battlefieldEffects.triggerEffects.targetCount.${field} invalid: ${worldID}.${effectID}`)
      }
      if (targetCount.min > targetCount.max) throw new Error(`world.battlefieldEffects.triggerEffects.targetCount bounds invalid: ${worldID}.${effectID}`)
      if (!Array.isArray(triggerEffect.choices) || triggerEffect.choices.length < 2) throw new Error(`world.battlefieldEffects.triggerEffects.choices invalid: ${worldID}.${effectID}`)
      const choiceIDs = new Set()
      let totalWeight = 0
      for (const choice of triggerEffect.choices) {
        const choiceID = parseOptionalString(choice?.id) || ""
        const weight = parseNumberStrict(choice?.weight)
        if (!choiceID || choiceIDs.has(choiceID) || !Number.isFinite(weight) || weight <= 0) throw new Error(`world.battlefieldEffects.triggerEffects.choice invalid: ${worldID}.${effectID}`)
        choiceIDs.add(choiceID)
        totalWeight += weight
        const choiceEffect = choice?.effect
        if (!choiceEffect || (choiceEffect.kind !== "status" && choiceEffect.kind !== "adjust_skill_cooldown")) throw new Error(`world.battlefieldEffects.triggerEffects.choice.effect invalid: ${worldID}.${effectID}.${choiceID}`)
        if (choiceEffect.kind === "status") {
          const statusID = parseOptionalString(choiceEffect.statusId) || ""
          const duration = parseNumberStrict(choiceEffect.durationTurns)
          if (!validBuffIDs.has(statusID) || !Number.isInteger(duration) || duration <= 0) throw new Error(`world.battlefieldEffects.triggerEffects.choice.status invalid: ${worldID}.${effectID}.${choiceID}`)
        } else {
          const amount = parseNumberStrict(choiceEffect.amount)
          if (!Number.isInteger(amount) || amount === 0 || amount < -20 || amount > 20 || typeof choiceEffect.includeBasic !== "boolean") throw new Error(`world.battlefieldEffects.triggerEffects.choice.cooldown invalid: ${worldID}.${effectID}.${choiceID}`)
          for (const skillID of parseIDList(choiceEffect.excludeSkillIds)) {
            if (!validSkillIDs.has(skillID)) throw new Error(`world.battlefieldEffects.triggerEffects.choice.skill not found: ${worldID}.${effectID}.${skillID}`)
          }
        }
      }
      if (Math.abs(totalWeight - 1) > 0.000001) throw new Error(`world.battlefieldEffects.triggerEffects.choice weights must sum to 1: ${worldID}.${effectID}`)
    }

    const windowIDs = new Set()
    for (const window of Array.isArray(effect.postTriggerWindows) ? effect.postTriggerWindows : []) {
      const windowID = parseOptionalString(window?.id) || ""
      const duration = parseNumberStrict(window?.duration)
      if (!windowID || windowIDs.has(windowID) || !Number.isFinite(duration) || !Number.isInteger(duration) || duration <= 0 || window?.unit !== "player_round") {
        throw new Error(`world.battlefieldEffects.postTriggerWindows invalid: ${worldID}.${effectID}`)
      }
      windowIDs.add(windowID)
    }
    for (const modifier of Array.isArray(effect.conditionalModifiers) ? effect.conditionalModifiers : []) {
      const value = parseNumberStrict(modifier?.value)
      if (modifier?.kind !== "incoming_damage_reduction" || modifier?.targetScope !== "special_enemies" || !Number.isFinite(value) || value < 0 || value > 1) {
        throw new Error(`world.battlefieldEffects.conditionalModifiers invalid: ${worldID}.${effectID}`)
      }
      if (!windowIDs.has(parseOptionalString(modifier?.inactiveDuringWindow) || "")) {
        throw new Error(`world.battlefieldEffects.conditionalModifiers window not found: ${worldID}.${effectID}`)
      }
    }
  }
}

function validateArenaFormats(rows, buffs) {
  if (!Array.isArray(rows) || rows.length === 0) throw new Error("arena_formats must be a non-empty array")
  const buffIDs = new Set((buffs || []).map((row) => String(row?.id || "").trim()).filter(Boolean))
  const buffByID = new Map((buffs || []).map((row) => [String(row?.id || "").trim(), row]))
  const formatIDs = new Set()
  const stableIDPattern = /^(?!all$)[a-z0-9][a-z0-9_:-]{0,63}$/
  const authoredFormatIDs = new Set(rows.map((row) => String(row?.id || "").trim()).filter((id) => stableIDPattern.test(id)))
  const baseByFormatID = new Map()
  const triggerIDsByFormatID = new Map()
  if (!authoredFormatIDs.has("standard")) throw new Error("arena_formats.standard is required")
  for (const row of rows) {
    if (!specializationIsObject(row)) throw new Error("arena_formats row must be an object")
    specializationRejectUnknownKeys(row, [
      "id", "baseFormatId", "schemaVersion", "rulesetVersion", "balanceRevision", "nameKey", "shortDescKey",
      "descKey", "icon", "sortOrder", "enabled", "triggers"
    ], `arena_formats.${String(row?.id || "unknown")}`)
    const formatID = specializationRequiredString(row.id, "arena_formats.id")
    if (!stableIDPattern.test(formatID) || formatID === "all") {
      throw new Error(`arena_formats.id invalid: ${formatID}`)
    }
    if (formatIDs.has(formatID)) throw new Error(`duplicate arena_formats.id: ${formatID}`)
    formatIDs.add(formatID)
    if (row.schemaVersion !== 1) throw new Error(`arena_formats.schemaVersion unsupported: ${formatID}`)
    const baseFormatID = parseOptionalString(row.baseFormatId) || ""
    if (formatID === "standard" && baseFormatID) throw new Error("arena_formats.standard cannot inherit another format")
    if (formatID !== "standard" && !baseFormatID) throw new Error(`arena_formats must inherit standard: ${formatID}`)
    if (baseFormatID && (baseFormatID === formatID || !authoredFormatIDs.has(baseFormatID))) throw new Error(`arena_formats.baseFormatId invalid: ${formatID}.${baseFormatID}`)
    baseByFormatID.set(formatID, baseFormatID)
    if (row.rulesetVersion !== "arena-format-v1") throw new Error(`arena_formats.rulesetVersion unsupported: ${formatID}`)
    specializationValidateFinite(row.balanceRevision, `arena_formats.balanceRevision: ${formatID}`, { integer: true, min: 1 })
    specializationValidateFinite(row.sortOrder, `arena_formats.sortOrder: ${formatID}`, { integer: true, min: 0, max: 10000 })
    if (typeof row.enabled !== "boolean") throw new Error(`arena_formats.enabled invalid: ${formatID}`)
    if (formatID === "standard" && !row.enabled) throw new Error("arena_formats.standard must be enabled")
    if (row.nameKey !== `game.arena.formats.${formatID}.name` ||
        row.shortDescKey !== `game.arena.formats.${formatID}.shortDesc` ||
        row.descKey !== `game.arena.formats.${formatID}.desc`) {
      throw new Error(`arena_formats locale keys invalid: ${formatID}`)
    }
    specializationRequiredString(row.icon, `arena_formats.icon: ${formatID}`)
    if (!Array.isArray(row.triggers) || row.triggers.length === 0) throw new Error(`arena_formats.triggers required: ${formatID}`)
    const triggerIDs = new Set()
    for (const trigger of row.triggers) {
      if (!specializationIsObject(trigger)) throw new Error(`arena_formats.triggers entry invalid: ${formatID}`)
      specializationRejectUnknownKeys(trigger, ["id", "event", "effects"], `arena_formats.${formatID}.triggers`)
      const triggerID = specializationRequiredString(trigger.id, `arena_formats.triggers.id: ${formatID}`)
      if (!stableIDPattern.test(triggerID) || triggerIDs.has(triggerID)) throw new Error(`arena_formats.triggers.id invalid or duplicate: ${formatID}.${triggerID}`)
      triggerIDs.add(triggerID)
      if (trigger.event !== "lineup_snapshot" && trigger.event !== "battle_start" && trigger.event !== "after_summon") {
        throw new Error(`arena_formats.triggers.event invalid: ${formatID}.${triggerID}`)
      }
      if (!Array.isArray(trigger.effects) || trigger.effects.length === 0) throw new Error(`arena_formats.triggers.effects required: ${formatID}.${triggerID}`)
      for (const [effectIndex, effect] of trigger.effects.entries()) {
        const label = `arena_formats.${formatID}.${triggerID}.effects[${effectIndex}]`
        if (!specializationIsObject(effect)) throw new Error(`${label} must be an object`)
        if (effect.kind === "warmup_buff") {
          specializationRejectUnknownKeys(effect, ["kind", "targetScope", "includeSummons", "buffId", "warmup"], label)
          if (trigger.event !== "battle_start" || effect.targetScope !== "eligible_side_units" || effect.includeSummons !== true) {
            throw new Error(`${label} warmup_buff contract invalid`)
          }
          const buffID = specializationRequiredString(effect.buffId, `${label}.buffId`)
          if (buffID !== "arena_output_warmup" || !buffIDs.has(buffID)) throw new Error(`${label}.buffId invalid: ${buffID}`)
          if (!specializationIsObject(effect.warmup)) throw new Error(`${label}.warmup invalid`)
          specializationRejectUnknownKeys(effect.warmup, [
            "levelSource", "higherSideGapExceeds", "alwaysAboveLevel", "normalActions", "startMultiplier",
            "recoveryPerNormalAction", "maximumMultiplier", "outputs"
          ], `${label}.warmup`)
          if (effect.warmup.levelSource !== "player_battle_level") throw new Error(`${label}.warmup.levelSource invalid`)
          specializationValidateFinite(effect.warmup.higherSideGapExceeds, `${label}.warmup.higherSideGapExceeds`, { integer: true, min: 1 })
          specializationValidateFinite(effect.warmup.alwaysAboveLevel, `${label}.warmup.alwaysAboveLevel`, { integer: true, min: 1 })
          specializationValidateFinite(effect.warmup.normalActions, `${label}.warmup.normalActions`, { integer: true, min: 1, max: 10 })
          specializationValidateFinite(effect.warmup.startMultiplier, `${label}.warmup.startMultiplier`, { min: 0, max: 1 })
          specializationValidateFinite(effect.warmup.recoveryPerNormalAction, `${label}.warmup.recoveryPerNormalAction`, { min: Number.MIN_VALUE, max: 1 })
          specializationValidateFinite(effect.warmup.maximumMultiplier, `${label}.warmup.maximumMultiplier`, { min: Number.MIN_VALUE, max: 1 })
          if (Math.abs(effect.warmup.startMultiplier + effect.warmup.normalActions * effect.warmup.recoveryPerNormalAction - effect.warmup.maximumMultiplier) > 0.000001) {
            throw new Error(`${label}.warmup curve must recover exactly to maximumMultiplier`)
          }
          if (!Array.isArray(effect.warmup.outputs) || effect.warmup.outputs.length !== 2 || new Set(effect.warmup.outputs).size !== 2 || effect.warmup.outputs.some((output) => output !== "damage" && output !== "heal")) {
            throw new Error(`${label}.warmup.outputs invalid`)
          }
          if (buffByID.get(buffID)?.maxCharges !== effect.warmup.normalActions || buffByID.get(buffID)?.dispellable !== false) {
            throw new Error(`${label} warmup buff must be undispellable and match normalActions with maxCharges`)
          }
          continue
        }
        if (effect.kind === "matchup_hp_buff") {
          specializationRejectUnknownKeys(effect, ["kind", "targetScope", "includeSummons", "buffId", "payloadKey", "matchupHp"], label)
          if (trigger.event !== "battle_start" || effect.targetScope !== "formal_units_by_side" || effect.includeSummons !== false || effect.payloadKey !== "hpBonus") {
            throw new Error(`${label} matchup_hp_buff contract invalid`)
          }
          const buffID = specializationRequiredString(effect.buffId, `${label}.buffId`)
          if (buffID !== "arena_matchup_hp_adjustment" || !buffIDs.has(buffID) || buffByID.get(buffID)?.dispellable !== false) {
            throw new Error(`${label}.buffId invalid: ${buffID}`)
          }
          if (!specializationIsObject(effect.matchupHp)) throw new Error(`${label}.matchupHp invalid`)
          specializationRejectUnknownKeys(effect.matchupHp, ["levelSource", "bothAtLeastLevel", "bothSidesFlatBonus", "lowerSideBonusPerLevel"], `${label}.matchupHp`)
          if (effect.matchupHp.levelSource !== "player_battle_level") throw new Error(`${label}.matchupHp.levelSource invalid`)
          specializationValidateFinite(effect.matchupHp.bothAtLeastLevel, `${label}.matchupHp.bothAtLeastLevel`, { integer: true, min: 1 })
          specializationValidateFinite(effect.matchupHp.bothSidesFlatBonus, `${label}.matchupHp.bothSidesFlatBonus`, { min: Number.MIN_VALUE })
          specializationValidateFinite(effect.matchupHp.lowerSideBonusPerLevel, `${label}.matchupHp.lowerSideBonusPerLevel`, { min: Number.MIN_VALUE })
          continue
        }
        if (effect.kind === "specialization_policy") {
          specializationRejectUnknownKeys(effect, ["kind", "targetScope", "includeSummons", "specializationEnabled"], label)
          if (trigger.event !== "lineup_snapshot" || effect.targetScope !== "player_units" || effect.includeSummons !== false || typeof effect.specializationEnabled !== "boolean") {
            throw new Error(`${label} specialization_policy contract invalid`)
          }
          continue
        }
        if (effect.kind === "turn_barrier") {
          specializationRejectUnknownKeys(effect, ["kind", "targetScope", "includeSummons", "buffId", "charges"], label)
          if (trigger.event !== "battle_start" || effect.targetScope !== "all_units" || typeof effect.includeSummons !== "boolean") {
            throw new Error(`${label} turn_barrier contract invalid`)
          }
          const buffID = specializationRequiredString(effect.buffId, `${label}.buffId`)
          if (!buffIDs.has(buffID)) throw new Error(`${label}.buffId not found: ${buffID}`)
          specializationValidateFinite(effect.charges, `${label}.charges`, { integer: true, min: 1, max: 12 })
          continue
        }
        if (effect.kind === "damage") {
          specializationRejectUnknownKeys(effect, [
            "kind", "targetScope", "includeSummons", "damageType", "skipFinalDamageModifiers",
            "dispatchHooks", "simultaneous", "amount"
          ], label)
          if (trigger.event !== "after_summon" || effect.targetScope !== "all_alive_units" ||
              typeof effect.includeSummons !== "boolean" || !["physical", "magical"].includes(effect.damageType) ||
              typeof effect.skipFinalDamageModifiers !== "boolean" || typeof effect.dispatchHooks !== "boolean" ||
              effect.simultaneous !== true) {
            throw new Error(`${label} damage contract invalid`)
          }
          if (!specializationIsObject(effect.amount)) throw new Error(`${label}.amount invalid`)
          specializationRejectUnknownKeys(effect.amount, ["basis", "multiplier", "flat", "minimum", "maximum"], `${label}.amount`)
          if (effect.amount.basis !== "target_max_hp") throw new Error(`${label}.amount.basis invalid`)
          specializationValidateFinite(effect.amount.multiplier, `${label}.amount.multiplier`, { min: 0, max: 1 })
          specializationValidateFinite(effect.amount.flat, `${label}.amount.flat`, { min: 0 })
          specializationValidateFinite(effect.amount.minimum, `${label}.amount.minimum`, { min: 0 })
          if (effect.amount.maximum !== undefined) {
            specializationValidateFinite(effect.amount.maximum, `${label}.amount.maximum`, { min: effect.amount.minimum })
          }
          if (effect.amount.multiplier === 0 && effect.amount.flat === 0) throw new Error(`${label}.amount must deal positive damage`)
          continue
        }
        throw new Error(`${label}.kind invalid: ${String(effect.kind || "")}`)
      }
    }
    triggerIDsByFormatID.set(formatID, triggerIDs)
  }

  const resolvedTriggerIDs = new Map()
  const visiting = new Set()
  const resolveTriggerIDs = (formatID) => {
    if (resolvedTriggerIDs.has(formatID)) return resolvedTriggerIDs.get(formatID)
    if (visiting.has(formatID)) throw new Error(`arena_formats inheritance cycle: ${formatID}`)
    visiting.add(formatID)
    const out = new Set()
    const baseFormatID = baseByFormatID.get(formatID) || ""
    if (baseFormatID) {
      for (const triggerID of resolveTriggerIDs(baseFormatID)) out.add(triggerID)
    }
    for (const triggerID of triggerIDsByFormatID.get(formatID) || []) {
      if (out.has(triggerID)) throw new Error(`arena_formats inherited trigger duplicate: ${formatID}.${triggerID}`)
      out.add(triggerID)
    }
    visiting.delete(formatID)
    resolvedTriggerIDs.set(formatID, out)
    return out
  }
  for (const formatID of formatIDs) resolveTriggerIDs(formatID)
}

const COMBAT_TRIAL_ENEMY_PROFILE_STATS = new Set([
  "hp", "mp", "hp_regen", "mp_regen",
  "strength", "dexterity", "intelligence", "stamina", "luck", "perception",
  "physics_damage", "magical_damage", "physics_defence", "magical_defence", "speed",
  "crit_chance", "crit_damage", "magic_crit_chance", "magic_crit_damage", "crit_damage_resist",
  "physics_damage_amplification", "magical_damage_amplification",
  "physics_damage_resist", "magical_damage_resist",
  "physics_flat_defense_penetration", "magicial_flat_defense_penetration",
  "physics_percent_defense_penetration", "magicial_percent_defense_penetration"
])

function validateCombatTrialEnemyStatProfile(raw, label) {
  if (raw == null) return
  if (!specializationIsObject(raw)) throw new Error(`${label} must be an object`)
  specializationRejectUnknownKeys(raw, ["difficultyLevel", "multiplier", "flatBonuses"], label)
  specializationValidateFinite(raw.difficultyLevel, `${label}.difficultyLevel`, { integer: true, min: 1, max: 1000 })
  specializationValidateFinite(raw.multiplier, `${label}.multiplier`, { min: Number.EPSILON, max: 1000 })
  if (raw.flatBonuses == null) return
  if (!specializationIsObject(raw.flatBonuses)) throw new Error(`${label}.flatBonuses must be an object`)
  for (const [rawKey, value] of Object.entries(raw.flatBonuses)) {
    const key = String(rawKey || "").trim().toLowerCase()
    if (key !== rawKey || !COMBAT_TRIAL_ENEMY_PROFILE_STATS.has(key)) {
      throw new Error(`${label}.flatBonuses contains an unsupported combat stat: ${rawKey}`)
    }
    specializationValidateFinite(value, `${label}.flatBonuses.${key}`, { min: -1e12, max: 1e12 })
  }
}

function validateCombatTrialEnemyStatOverrides(raw, label) {
  if (raw == null) return
  if (!specializationIsObject(raw)) throw new Error(`${label} must be an object`)
  for (const [rawKey, value] of Object.entries(raw)) {
    const key = String(rawKey || "").trim().toLowerCase()
    if (key !== rawKey || !COMBAT_TRIAL_ENEMY_PROFILE_STATS.has(key)) {
      throw new Error(`${label} contains an unsupported combat stat: ${rawKey}`)
    }
    specializationValidateFinite(value, `${label}.${key}`, { min: -1e12, max: 1e12 })
    if (key === "hp" && value <= 0) throw new Error(`${label}.hp must be positive`)
    if (["mp", "hp_regen", "mp_regen"].includes(key) && value < 0) throw new Error(`${label}.${key} must not be negative`)
    if (["crit_damage_resist", "physics_damage_resist", "magical_damage_resist"].includes(key) && (value < 0 || value > 1)) {
      throw new Error(`${label}.${key} must be between 0 and 1`)
    }
  }
}

function validateCombatTrialDefinitions(metrics, formats, enemies = [], buffs = []) {
  if (!Array.isArray(metrics) || metrics.length === 0) throw new Error("combat_trial_metrics must be a non-empty array")
  if (!Array.isArray(formats) || formats.length === 0) throw new Error("combat_trial_formats must be a non-empty array")
  const stableIDPattern = /^(?!all$)[a-z0-9][a-z0-9_:-]{0,63}$/
  const stableIDArray = (raw, label, { optional = false } = {}) => {
    if (raw == null && optional) return []
    if (!Array.isArray(raw)) throw new Error(`${label} must be an array`)
    const values = raw.map((value, index) => specializationRequiredString(value, `${label}[${index}]`))
    if (values.length !== new Set(values).size || values.some((id) => !stableIDPattern.test(id))) {
      throw new Error(`${label} contains an invalid or duplicate id`)
    }
    return values
  }
  const metricIDs = new Set()
  const enabledMetricIDs = new Set()
  const metricsByID = new Map()
  for (const row of metrics) {
    if (!specializationIsObject(row)) throw new Error("combat_trial_metrics row must be an object")
    specializationRejectUnknownKeys(row, [
      "id", "schemaVersion", "nameKey", "shortNameKey", "descKey", "eventType", "aggregation",
      "valueSource", "sourceScope", "targetScope", "ranked", "rankDirection", "eligibleEndReasons",
      "valueFormat", "sortOrder", "enabled"
    ], `combat_trial_metrics.${String(row?.id || "unknown")}`)
    const metricID = specializationRequiredString(row.id, "combat_trial_metrics.id")
    if (!stableIDPattern.test(metricID) || metricIDs.has(metricID)) throw new Error(`combat_trial_metrics.id invalid or duplicate: ${metricID}`)
    metricIDs.add(metricID)
    if (row.schemaVersion !== 2) throw new Error(`combat_trial_metrics.schemaVersion unsupported: ${metricID}`)
    for (const key of ["nameKey", "shortNameKey", "descKey"]) specializationRequiredString(row[key], `combat_trial_metrics.${metricID}.${key}`)
    if (!["damage", "heal", "battle_end"].includes(row.eventType)) throw new Error(`combat_trial_metrics.eventType unsupported: ${metricID}`)
    if (row.eventType === "battle_end") {
      if (row.aggregation !== "final" || !["player_main_actions", "waves_cleared", "objective_units_defeated", "objective_units_remaining_hp"].includes(row.valueSource)) {
        throw new Error(`combat_trial_metrics result source invalid: ${metricID}`)
      }
    } else if (!["sum", "max"].includes(row.aggregation)) {
      throw new Error(`combat_trial_metrics.aggregation invalid: ${metricID}`)
    }
    if (!["player_side", "enemy_side", "any"].includes(row.sourceScope) || !["objective_units", "trial_dummies", "player_side", "enemy_side", "any"].includes(row.targetScope)) {
      throw new Error(`combat_trial_metrics scope invalid: ${metricID}`)
    }
    if (typeof row.ranked !== "boolean" || !["asc", "desc"].includes(row.rankDirection)) throw new Error(`combat_trial_metrics rank invalid: ${metricID}`)
    stableIDArray(row.eligibleEndReasons, `combat_trial_metrics.${metricID}.eligibleEndReasons`, { optional: true })
    if (!["number", "integer"].includes(row.valueFormat)) throw new Error(`combat_trial_metrics valueFormat invalid: ${metricID}`)
    specializationValidateFinite(row.sortOrder, `combat_trial_metrics.sortOrder: ${metricID}`, { integer: true, min: 0, max: 10000 })
    if (typeof row.enabled !== "boolean") throw new Error(`combat_trial_metrics.enabled invalid: ${metricID}`)
    if (row.enabled) enabledMetricIDs.add(metricID)
    metricsByID.set(metricID, row)
  }

  const knownEnemyIDs = new Set((enemies || []).map((row) => String(row?.id || "").trim()).filter(Boolean))
  const buffsByID = new Map((buffs || []).map((row) => [String(row?.id || "").trim(), row]).filter(([id]) => id))
  const formatIDs = new Set()
  const formatsByID = new Map()
  for (const row of formats) {
    if (!specializationIsObject(row)) throw new Error("combat_trial_formats row must be an object")
    specializationRejectUnknownKeys(row, [
      "id", "baseFormatId", "schemaVersion", "rulesetVersion", "balanceRevision", "nameKey", "shortDescKey", "descKey",
      "icon", "sortOrder", "enabled", "lineupRules", "metricIds", "limits", "encounter", "endConditions", "triggers"
    ], `combat_trial_formats.${String(row?.id || "unknown")}`)
    const formatID = specializationRequiredString(row.id, "combat_trial_formats.id")
    if (!stableIDPattern.test(formatID) || formatIDs.has(formatID)) throw new Error(`combat_trial_formats.id invalid or duplicate: ${formatID}`)
    formatIDs.add(formatID)
    formatsByID.set(formatID, row)
    if (row.schemaVersion !== 2 || row.rulesetVersion !== "combat-trial-format-v2") throw new Error(`combat_trial_formats version unsupported: ${formatID}`)
    specializationValidateFinite(row.balanceRevision, `combat_trial_formats.balanceRevision: ${formatID}`, { integer: true, min: 1 })
    specializationValidateFinite(row.sortOrder, `combat_trial_formats.sortOrder: ${formatID}`, { integer: true, min: 0, max: 10000 })
    for (const key of ["nameKey", "shortDescKey", "descKey", "icon"]) specializationRequiredString(row[key], `combat_trial_formats.${formatID}.${key}`)
    if (typeof row.enabled !== "boolean") throw new Error(`combat_trial_formats.enabled invalid: ${formatID}`)
    if (row.baseFormatId != null && row.baseFormatId !== "") {
      const baseID = specializationRequiredString(row.baseFormatId, `combat_trial_formats.${formatID}.baseFormatId`)
      if (!stableIDPattern.test(baseID) || baseID === formatID) throw new Error(`combat_trial_formats.baseFormatId invalid: ${formatID}`)
    }
    if (row.lineupRules != null) {
      if (!specializationIsObject(row.lineupRules)) throw new Error(`combat_trial_formats.lineupRules invalid: ${formatID}`)
      specializationRejectUnknownKeys(row.lineupRules, ["positionPolicy"], `combat_trial_formats.${formatID}.lineupRules`)
      if (!["shared", "distinct"].includes(row.lineupRules.positionPolicy)) {
        throw new Error(`combat_trial_formats.positionPolicy invalid: ${formatID}`)
      }
    }
    if (row.metricIds != null) {
      const referencedMetrics = stableIDArray(row.metricIds, `combat_trial_formats.${formatID}.metricIds`)
      if (referencedMetrics.length === 0 || referencedMetrics.some((id) => !enabledMetricIDs.has(id))) {
        throw new Error(`combat_trial_formats.metricIds invalid: ${formatID}`)
      }
    }
    if (row.limits != null) {
      if (!Array.isArray(row.limits)) throw new Error(`combat_trial_formats.limits invalid: ${formatID}`)
      const limitKinds = new Set()
      for (const limit of row.limits) {
        if (!specializationIsObject(limit)) throw new Error(`combat_trial_formats.limit invalid: ${formatID}`)
        specializationRejectUnknownKeys(limit, ["kind", "value"], `combat_trial_formats.${formatID}.limits`)
        if (limit.kind !== "player_main_actions" || limitKinds.has(limit.kind)) throw new Error(`combat_trial_formats.limit kind invalid: ${formatID}`)
        limitKinds.add(limit.kind)
        specializationValidateFinite(limit.value, `combat_trial_formats.limit.value: ${formatID}`, { integer: true, min: 1, max: 1000 })
      }
    }
    if (row.encounter != null) {
      const encounter = row.encounter
      if (!specializationIsObject(encounter)) throw new Error(`combat_trial_formats.encounter invalid: ${formatID}`)
      specializationRejectUnknownKeys(encounter, ["kind", "units", "waves", "enemyStatProfile"], `combat_trial_formats.${formatID}.encounter`)
      validateCombatTrialEnemyStatProfile(encounter.enemyStatProfile, `combat_trial_formats.${formatID}.encounter.enemyStatProfile`)
      const unitIDs = new Set()
      let objectiveCount = 0
      let totalUnits = 0
      const validateRoster = (units, label, enemyStatProfile) => {
        if (!Array.isArray(units) || units.length < 1 || units.length > 12) throw new Error(`combat_trial_formats.encounter roster invalid: ${formatID}.${label}`)
        let rosterObjectives = 0
        let enemyCount = 0
        let authoredEnemyCount = 0
        for (const unit of units) {
          if (!specializationIsObject(unit)) throw new Error(`combat_trial_formats.encounter unit invalid: ${formatID}`)
          specializationRejectUnknownKeys(unit, [
            "id", "kind", "enemyId", "nameKey", "side", "position", "behavior", "healthMode", "objectiveTarget",
            "initialHpRatio", "skipTurnOrder", "initialBuffs", "defenseSource", "maxHp", "damageResist", "criticalDamageResist", "statOverrides"
          ], `combat_trial_formats.${formatID}.encounter.${label}`)
          const unitID = specializationRequiredString(unit.id, `combat_trial_formats.${formatID}.encounter.unit.id`)
          if (!stableIDPattern.test(unitID) || unitIDs.has(unitID)) throw new Error(`combat_trial_formats encounter unit id invalid: ${formatID}.${unitID}`)
          unitIDs.add(unitID)
          const side = String(unit.side || "enemy").trim()
          if (!["player", "enemy"].includes(side) || !["vanguard", "midguard", "rearguard"].includes(unit.position) || !["passive", "active"].includes(unit.behavior) || typeof unit.objectiveTarget !== "boolean") {
            throw new Error(`combat_trial_formats encounter unit behavior invalid: ${formatID}.${unitID}`)
          }
          if (unit.objectiveTarget) {
            objectiveCount++
            rosterObjectives++
          }
          if (side === "enemy") enemyCount++
          if (unit.damageResist != null) specializationValidateFinite(unit.damageResist, `combat_trial_formats damageResist: ${formatID}.${unitID}`, { min: 0, max: 1 })
          if (unit.criticalDamageResist != null) specializationValidateFinite(unit.criticalDamageResist, `combat_trial_formats criticalDamageResist: ${formatID}.${unitID}`, { min: 0, max: 1 })
          if (unit.initialBuffs != null) {
            if (!Array.isArray(unit.initialBuffs)) throw new Error(`combat_trial_formats initialBuffs invalid: ${formatID}.${unitID}`)
            const initialBuffIDs = new Set()
            for (const initialBuff of unit.initialBuffs) {
              if (!specializationIsObject(initialBuff)) throw new Error(`combat_trial_formats initialBuff invalid: ${formatID}.${unitID}`)
              specializationRejectUnknownKeys(initialBuff, ["buffId", "payload"], `combat_trial_formats.${formatID}.encounter.${label}.${unitID}.initialBuffs`)
              const buffID = specializationRequiredString(initialBuff.buffId, `combat_trial_formats.${formatID}.${unitID}.initialBuff.buffId`)
              const buff = buffsByID.get(buffID)
              if (!stableIDPattern.test(buffID) || initialBuffIDs.has(buffID) || !buff || buff.disabled === true) {
                throw new Error(`combat_trial_formats initialBuff reference invalid: ${formatID}.${unitID}.${buffID}`)
              }
              initialBuffIDs.add(buffID)
              if (unit.kind === "scripted_ally" && buff.permanent !== true) {
                throw new Error(`combat_trial_formats scripted ally initialBuff must be permanent: ${formatID}.${unitID}.${buffID}`)
              }
              if (initialBuff.payload != null) {
                if (!specializationIsObject(initialBuff.payload)) throw new Error(`combat_trial_formats initialBuff payload invalid: ${formatID}.${unitID}.${buffID}`)
                for (const [payloadKey, payloadValue] of Object.entries(initialBuff.payload)) {
                  if (!stableIDPattern.test(payloadKey) || !Number.isFinite(payloadValue) || Math.abs(payloadValue) > 1e12) {
                    throw new Error(`combat_trial_formats initialBuff payload invalid: ${formatID}.${unitID}.${buffID}.${payloadKey}`)
                  }
                }
              }
            }
          }
          validateCombatTrialEnemyStatOverrides(unit.statOverrides, `combat_trial_formats.${formatID}.encounter.${label}.${unitID}.statOverrides`)
          if (unit.kind === "training_target") {
            if (side !== "enemy" || unit.behavior !== "passive" || unit.healthMode !== "immutable_full" || unit.defenseSource !== "combat_balance_defense_k" || unit.initialHpRatio != null || unit.skipTurnOrder === true || unit.statOverrides != null) {
              throw new Error(`combat_trial_formats training target invalid: ${formatID}.${unitID}`)
            }
            specializationValidateFinite(unit.maxHp, `combat_trial_formats training target maxHp: ${formatID}.${unitID}`, { min: 1 })
          } else if (unit.kind === "scripted_ally") {
            if (side !== "player" || unit.behavior !== "passive" || unit.healthMode !== "normal" || unit.skipTurnOrder !== true || !String(unit.nameKey || "").trim() || unit.statOverrides != null) {
              throw new Error(`combat_trial_formats scripted ally invalid: ${formatID}.${unitID}`)
            }
            specializationValidateFinite(unit.maxHp, `combat_trial_formats scripted ally maxHp: ${formatID}.${unitID}`, { min: 1 })
            specializationValidateFinite(unit.initialHpRatio, `combat_trial_formats scripted ally initialHpRatio: ${formatID}.${unitID}`, { min: Number.EPSILON, max: 1 })
            if (unit.defenseSource != null && unit.defenseSource !== "combat_balance_defense_k") throw new Error(`combat_trial_formats scripted ally defense invalid: ${formatID}.${unitID}`)
          } else if (unit.kind === "authored_enemy") {
            if (side !== "enemy" || unit.healthMode !== "normal" || unit.initialHpRatio != null || unit.skipTurnOrder === true || !knownEnemyIDs.has(String(unit.enemyId || "").trim())) {
              throw new Error(`combat_trial_formats authored enemy invalid: ${formatID}.${unitID}`)
            }
            authoredEnemyCount++
          } else {
            throw new Error(`combat_trial_formats encounter unit kind unsupported: ${formatID}.${unitID}`)
          }
        }
        if (rosterObjectives === 0 || enemyCount === 0) throw new Error(`combat_trial_formats encounter roster objective or enemy missing: ${formatID}.${label}`)
        if (enemyStatProfile != null && authoredEnemyCount === 0) throw new Error(`combat_trial_formats enemy stat profile has no authored enemy: ${formatID}.${label}`)
        totalUnits += units.length
      }
      if (encounter.kind === "unit_roster") {
        if (encounter.waves != null) throw new Error(`combat_trial_formats unit roster cannot define waves: ${formatID}`)
        validateRoster(encounter.units, "units", encounter.enemyStatProfile)
      } else if (encounter.kind === "wave_roster") {
        if (encounter.units != null || !Array.isArray(encounter.waves) || encounter.waves.length < 1 || encounter.waves.length > 20) {
          throw new Error(`combat_trial_formats wave roster invalid: ${formatID}`)
        }
        const waveIDs = new Set()
        for (const wave of encounter.waves) {
          if (!specializationIsObject(wave)) throw new Error(`combat_trial_formats encounter wave invalid: ${formatID}`)
          specializationRejectUnknownKeys(wave, ["id", "nameKey", "units", "enemyStatProfile"], `combat_trial_formats.${formatID}.encounter.waves`)
          const waveID = specializationRequiredString(wave.id, `combat_trial_formats.${formatID}.encounter.wave.id`)
          if (!stableIDPattern.test(waveID) || waveIDs.has(waveID)) throw new Error(`combat_trial_formats encounter wave id invalid: ${formatID}.${waveID}`)
          waveIDs.add(waveID)
          if (wave.nameKey != null) specializationRequiredString(wave.nameKey, `combat_trial_formats.${formatID}.encounter.wave.nameKey`)
          validateCombatTrialEnemyStatProfile(wave.enemyStatProfile, `combat_trial_formats.${formatID}.encounter.waves.${waveID}.enemyStatProfile`)
          validateRoster(wave.units, `waves.${waveID}.units`, wave.enemyStatProfile ?? encounter.enemyStatProfile)
        }
      } else {
        throw new Error(`combat_trial_formats.encounter kind invalid: ${formatID}`)
      }
      if (objectiveCount === 0 || totalUnits > 60) throw new Error(`combat_trial_formats encounter objective missing or too large: ${formatID}`)
    }
    if (row.endConditions != null) {
      if (!Array.isArray(row.endConditions)) throw new Error(`combat_trial_formats.endConditions invalid: ${formatID}`)
      const conditionKinds = new Set()
      const conditionReasons = new Set()
      for (const condition of row.endConditions) {
        if (!specializationIsObject(condition)) throw new Error(`combat_trial_formats.endCondition invalid: ${formatID}`)
        specializationRejectUnknownKeys(condition, ["kind", "reason", "labelKey"], `combat_trial_formats.${formatID}.endConditions`)
        specializationRequiredString(condition.labelKey, `combat_trial_formats.${formatID}.endCondition.labelKey`)
        const conditionReason = specializationRequiredString(condition.reason, `combat_trial_formats.${formatID}.endCondition.reason`)
        if (!["player_team_defeated", "enemy_team_defeated", "player_main_actions_reached", "encounter_completed"].includes(condition.kind) || conditionKinds.has(condition.kind) || conditionReasons.has(conditionReason) || !stableIDPattern.test(conditionReason)) {
          throw new Error(`combat_trial_formats.endCondition unsupported: ${formatID}`)
        }
        conditionKinds.add(condition.kind)
        conditionReasons.add(conditionReason)
      }
    }
    if (row.triggers != null && !Array.isArray(row.triggers)) throw new Error(`combat_trial_formats.triggers invalid: ${formatID}`)
    const ruleIDs = new Set()
    for (const rule of row.triggers || []) {
      if (!specializationIsObject(rule)) throw new Error(`combat_trial_formats.rule invalid: ${formatID}`)
      specializationRejectUnknownKeys(rule, ["id", "event", "effects"], `combat_trial_formats.${formatID}.triggers`)
      const ruleID = specializationRequiredString(rule.id, `combat_trial_formats.${formatID}.triggers.id`)
      if (!stableIDPattern.test(ruleID) || ruleIDs.has(ruleID) || !["damage_calculation", "player_main_action_completed"].includes(rule.event) || !Array.isArray(rule.effects) || rule.effects.length === 0) {
        throw new Error(`combat_trial_formats.rule invalid: ${formatID}.${ruleID}`)
      }
      ruleIDs.add(ruleID)
      for (const effect of rule.effects) {
        if (!specializationIsObject(effect)) throw new Error(`combat_trial_formats.rule effect invalid: ${formatID}.${ruleID}`)
        specializationRejectUnknownKeys(effect, [
          "kind", "sourceScope", "includeSummons", "damageType", "multiplier",
          "targetScope", "targetUnitIds", "maxHpRatio", "nonLethal"
        ], `combat_trial_formats.${formatID}.triggers.${ruleID}.effects`)
        if (rule.event === "damage_calculation") {
          if (effect.kind !== "outgoing_damage_multiplier" || effect.sourceScope !== "player_side" ||
            typeof effect.includeSummons !== "boolean" || !["physical", "magical", "all"].includes(effect.damageType)) {
            throw new Error(`combat_trial_formats.rule effect invalid: ${formatID}.${ruleID}`)
          }
          specializationValidateFinite(effect.multiplier, `combat_trial_formats.rule multiplier: ${formatID}.${ruleID}`, { min: Number.EPSILON, max: 10 })
          continue
        }
        if (effect.kind !== "max_hp_percent_damage" || effect.targetScope !== "objective_units" || typeof effect.nonLethal !== "boolean") {
          throw new Error(`combat_trial_formats.rule effect invalid: ${formatID}.${ruleID}`)
        }
        stableIDArray(effect.targetUnitIds, `combat_trial_formats.${formatID}.triggers.${ruleID}.targetUnitIds`, { optional: true })
        specializationValidateFinite(effect.maxHpRatio, `combat_trial_formats.rule maxHpRatio: ${formatID}.${ruleID}`, { min: Number.EPSILON, max: 1 })
      }
    }
  }
  if (!formatIDs.has("standard")) throw new Error("combat_trial_formats.standard is required")
  const resolving = new Set()
  const resolved = new Map()
  const resolve = (formatID) => {
    if (resolved.has(formatID)) return resolved.get(formatID)
    const authored = formatsByID.get(formatID)
    if (!authored) throw new Error(`combat_trial_formats base missing: ${formatID}`)
    if (resolving.has(formatID)) throw new Error(`combat_trial_formats inheritance cycle: ${formatID}`)
    resolving.add(formatID)
    let row = { ...authored }
    if (authored.baseFormatId) {
      const base = resolve(authored.baseFormatId)
      row = {
        ...authored,
        lineupRules: authored.lineupRules ?? base.lineupRules,
        metricIds: authored.metricIds ?? base.metricIds,
        limits: authored.limits ?? base.limits,
        encounter: authored.encounter ?? base.encounter,
        endConditions: authored.endConditions ?? base.endConditions,
        triggers: [...(base.triggers || []), ...(authored.triggers || [])]
      }
      if (authored.enabled && !base.enabled) throw new Error(`combat_trial_formats enabled format inherits disabled base: ${formatID}`)
    }
    resolving.delete(formatID)
    const metricRefs = stableIDArray(row.metricIds, `combat_trial_formats.${formatID}.metricIds`)
    if (metricRefs.length === 0 || metricRefs.some((id) => !enabledMetricIDs.has(id))) throw new Error(`combat_trial_formats resolved metrics invalid: ${formatID}`)
    if (!specializationIsObject(row.lineupRules) || !["shared", "distinct"].includes(row.lineupRules.positionPolicy)) {
      throw new Error(`combat_trial_formats resolved lineup rules missing: ${formatID}`)
    }
    if (!Array.isArray(row.limits) || !row.limits.some((limit) => limit.kind === "player_main_actions")) throw new Error(`combat_trial_formats resolved action limit missing: ${formatID}`)
    if (!specializationIsObject(row.encounter)) throw new Error(`combat_trial_formats resolved encounter missing: ${formatID}`)
    const encounterUnits = row.encounter.kind === "wave_roster"
      ? (Array.isArray(row.encounter.waves) ? row.encounter.waves.flatMap((wave) => Array.isArray(wave?.units) ? wave.units : []) : [])
      : (Array.isArray(row.encounter.units) ? row.encounter.units : [])
    if (encounterUnits.length === 0) throw new Error(`combat_trial_formats resolved encounter missing: ${formatID}`)
    if (!Array.isArray(row.endConditions) || !row.endConditions.some((condition) => condition.kind === "player_main_actions_reached")) throw new Error(`combat_trial_formats resolved action end condition missing: ${formatID}`)
    if (!row.endConditions.some((condition) => condition.kind === "player_team_defeated")) throw new Error(`combat_trial_formats resolved player defeat end condition missing: ${formatID}`)
    const enemyEncounterUnits = encounterUnits.filter((unit) => String(unit?.side || "enemy") === "enemy")
    if (row.encounter.kind === "wave_roster" && !row.endConditions.some((condition) => condition.kind === "encounter_completed")) {
      throw new Error(`combat_trial_formats resolved encounter completion condition missing: ${formatID}`)
    }
    if (row.encounter.kind !== "wave_roster" && enemyEncounterUnits.every((unit) => unit.healthMode === "normal") && !row.endConditions.some((condition) => condition.kind === "enemy_team_defeated")) {
      throw new Error(`combat_trial_formats resolved enemy defeat end condition missing: ${formatID}`)
    }
    const endReasons = new Set(row.endConditions.map((condition) => condition.reason))
    const resolvedMetrics = metricRefs.map((metricID) => metricsByID.get(metricID))
    if (!resolvedMetrics.some((metric) => metric?.ranked === true)) {
      throw new Error(`combat_trial_formats resolved ranked metric missing: ${formatID}`)
    }
    for (const metric of resolvedMetrics) {
      const eligibleEndReasons = stableIDArray(metric?.eligibleEndReasons, `combat_trial_metrics.${metric?.id}.eligibleEndReasons`, { optional: true })
      if (eligibleEndReasons.length > 0 && !eligibleEndReasons.some((reason) => endReasons.has(reason))) {
        throw new Error(`combat_trial_formats metric has no eligible end reason: ${formatID}.${metric.id}`)
      }
    }
    const resolvedRuleIDs = (row.triggers || []).map((rule) => rule.id)
    if (new Set(resolvedRuleIDs).size !== resolvedRuleIDs.length) throw new Error(`combat_trial_formats resolved rule duplicate: ${formatID}`)
    const objectiveUnitIDs = new Set(encounterUnits.filter((unit) => unit?.objectiveTarget === true).map((unit) => String(unit.id || "").trim()).filter(Boolean))
    for (const rule of row.triggers || []) {
      for (const effect of rule.effects || []) {
        for (const targetUnitID of stableIDArray(effect.targetUnitIds, `combat_trial_formats.${formatID}.triggers.${rule.id}.targetUnitIds`, { optional: true })) {
          if (!objectiveUnitIDs.has(targetUnitID)) throw new Error(`combat_trial_formats rule target is not an objective unit: ${formatID}.${rule.id}.${targetUnitID}`)
        }
      }
    }
    resolved.set(formatID, row)
    return row
  }
  for (const formatID of formatIDs) resolve(formatID)
  const standard = resolve("standard")
  const standardBudget = standard.limits.find((limit) => limit.kind === "player_main_actions")?.value
  if (!standard?.enabled || standard.baseFormatId || standardBudget !== 20) throw new Error("combat_trial_formats.standard must be enabled with a 20 action budget")
  const standardMetricIDs = new Set(parseIDList(standard.metricIds))
  if (!["damage_total", "damage_max_segment"].every((metricID) => standardMetricIDs.has(metricID))) {
    throw new Error("combat_trial_formats.standard must include both V1 damage metrics")
  }
  if (standard.encounter.units.length !== 3 || standard.encounter.units.map((unit) => unit.position).join(",") !== "vanguard,midguard,rearguard" ||
    standard.encounter.units.some((unit) => unit.kind !== "training_target" || unit.objectiveTarget !== true)) {
    throw new Error("combat_trial_formats.standard must include three objective training targets")
  }
}

function validateWorldRows(rows, enemies = [], buffs = [], skills = []) {
  if (!Array.isArray(rows)) throw new Error("world must be an array")
  const knownEnemyIDs = new Set(
    (enemies || [])
      .map((row) => (typeof row?.id === "string" ? row.id.trim() : ""))
      .filter(Boolean)
  )
  const validBuffIDs = new Set((buffs || []).map((row) => String(row?.id || "").trim()).filter(Boolean))
  const validSkillIDs = new Set((skills || []).map((row) => String(row?.id || "").trim()).filter(Boolean))
  const ids = new Set()
  for (const row of rows) {
    if (!row || typeof row !== "object") throw new Error("world row must be object")
    if (row.disabled !== undefined && typeof row.disabled !== "boolean") throw new Error("world.disabled invalid")
    const id = typeof row.id === "string" ? row.id.trim() : ""
    if (!id) throw new Error("world.id is required")
    if (ids.has(id)) throw new Error(`duplicate world.id: ${id}`)
    ids.add(id)

    const nameKey = typeof row.nameKey === "string" ? row.nameKey.trim() : ""
    const descKey = typeof row.descKey === "string" ? row.descKey.trim() : ""
    if (nameKey !== `world.${id}.name`) throw new Error(`world.nameKey invalid for ${id}; expected world.${id}.name`)
    if (descKey !== `world.${id}.desc`) throw new Error(`world.descKey invalid for ${id}; expected world.${id}.desc`)
    validateWorldBattlefieldEffects(row.battlefieldEffects, id, validBuffIDs, validSkillIDs)

    const rowEnemyIDs = parseIDList(row.enemyIds)
    if (rowEnemyIDs.length === 0) throw new Error(`world.enemyIds required: ${id}`)
    for (const enemyID of rowEnemyIDs) {
      if (!knownEnemyIDs.has(enemyID)) throw new Error(`world.enemyIds enemy not found: ${id}.${enemyID}`)
    }
    const specialEnemyIDs = parseIDList(row.specialEnemyIds)
    for (const enemyID of specialEnemyIDs) {
      if (!knownEnemyIDs.has(enemyID)) throw new Error(`world.specialEnemyIds enemy not found: ${id}.${enemyID}`)
    }
    const specialWaveConfig = parseWorldSpecialWaves(row.specialWaves)
    const hasFixedWaves = specialWaveConfig.fixedWaves.length > 0
    const hasPeriodic = Boolean(specialWaveConfig.periodic)
    const hasWaveSchedules = Array.isArray(row.waveSchedules) && row.waveSchedules.length > 0
    if (specialEnemyIDs.length > 0 && !hasFixedWaves && !hasPeriodic && !hasWaveSchedules) {
      throw new Error(`world.specialWaves required when specialEnemyIds present: ${id}`)
    }
    if (hasPeriodic) {
      const periodic = specialWaveConfig.periodic
      if (!periodic || !Number.isFinite(periodic.every) || periodic.every <= 0 || !Number.isInteger(periodic.every)) {
        throw new Error(`world.specialWaves.every invalid: ${id}`)
      }
      if (!Number.isFinite(periodic.minCount) || periodic.minCount <= 0 || !Number.isInteger(periodic.minCount)) {
        throw new Error(`world.specialWaves.minCount invalid: ${id}`)
      }
      if (!Number.isFinite(periodic.maxCount) || periodic.maxCount <= 0 || !Number.isInteger(periodic.maxCount)) {
        throw new Error(`world.specialWaves.maxCount invalid: ${id}`)
      }
      if (periodic.maxCount < periodic.minCount) {
        throw new Error(`world.specialWaves maxCount < minCount: ${id}`)
      }
    }

    const scheduleIDs = new Set()
    for (const schedule of Array.isArray(row.waveSchedules) ? row.waveSchedules : []) {
      const scheduleID = parseOptionalString(schedule?.id) || ""
      if (!scheduleID || scheduleIDs.has(scheduleID)) throw new Error(`world.waveSchedules.id invalid: ${id}.${scheduleID}`)
      scheduleIDs.add(scheduleID)
      if (schedule.nameKey !== `world.${id}.waveSchedule.${scheduleID}.name`) throw new Error(`world.waveSchedules.nameKey invalid: ${id}.${scheduleID}`)
      if (schedule.descKey !== `world.${id}.waveSchedule.${scheduleID}.desc`) throw new Error(`world.waveSchedules.descKey invalid: ${id}.${scheduleID}`)
      if (!Number.isInteger(schedule.every) || schedule.every <= 0) throw new Error(`world.waveSchedules.every invalid: ${id}.${scheduleID}`)
      if (!Number.isInteger(schedule.priority)) throw new Error(`world.waveSchedules.priority invalid: ${id}.${scheduleID}`)
      if (!Number.isInteger(schedule.minCount) || schedule.minCount <= 0) throw new Error(`world.waveSchedules.minCount invalid: ${id}.${scheduleID}`)
      if (!Number.isInteger(schedule.maxCount) || schedule.maxCount < schedule.minCount) throw new Error(`world.waveSchedules.maxCount invalid: ${id}.${scheduleID}`)
      if (!Array.isArray(schedule.enemyChoices) || schedule.enemyChoices.length === 0) throw new Error(`world.waveSchedules.enemyChoices required: ${id}.${scheduleID}`)
      const scheduledEnemyIDs = new Set()
      for (const choice of schedule.enemyChoices) {
        const enemyID = parseOptionalString(choice?.enemyId) || ""
        const weight = parseNumberStrict(choice?.weight)
        if (!enemyID || scheduledEnemyIDs.has(enemyID) || !knownEnemyIDs.has(enemyID) || !Number.isFinite(weight) || weight <= 0) {
          throw new Error(`world.waveSchedules.enemyChoices invalid: ${id}.${scheduleID}.${enemyID}`)
        }
        scheduledEnemyIDs.add(enemyID)
      }
    }

    const entryDuration = parseNumberStrict(row.entryDurationSeconds)
    if (!Number.isFinite(entryDuration) || entryDuration <= 0) {
      throw new Error(`world.entryDurationSeconds invalid: ${id}`)
    }
    const entryCosts = row.entryCosts
    if (!entryCosts || typeof entryCosts !== "object" || Array.isArray(entryCosts)) {
      throw new Error(`world.entryCosts invalid: ${id}`)
    }
    for (const [itemID, qty] of Object.entries(entryCosts)) {
      const n = parseNumberStrict(qty)
      if (!itemID || !Number.isFinite(n) || n < 0) {
        throw new Error(`world.entryCosts invalid: ${id}.${itemID}`)
      }
    }
    const normalEnemyCount = parseWorldEnemyCountRange(row.normalEnemyCount, 0, 0)
    if (!Number.isFinite(normalEnemyCount.min) || !Number.isFinite(normalEnemyCount.max) || normalEnemyCount.min < 0 || normalEnemyCount.max < normalEnemyCount.min) {
      throw new Error(`world.normalEnemyCount invalid: ${id}`)
    }
    const extraEnemyPerPlayer = parseWorldEnemyCountRange(row.extraEnemyPerPlayer, 0, 0)
    if (!Number.isFinite(extraEnemyPerPlayer.min) || !Number.isFinite(extraEnemyPerPlayer.max) || extraEnemyPerPlayer.min < 0 || extraEnemyPerPlayer.max < extraEnemyPerPlayer.min) {
      throw new Error(`world.extraEnemyPerPlayer invalid: ${id}`)
    }
    const customLineupConf = Array.isArray(row.customLineupConf) ? row.customLineupConf : []
    const lineupIDs = new Set()
    for (const conf of customLineupConf) {
      const confID = typeof conf?.id === "string" ? conf.id.trim() : ""
      if (!confID) throw new Error(`world.customLineupConf.id required: ${id}`)
      if (lineupIDs.has(confID)) throw new Error(`duplicate world.customLineupConf.id: ${id}.${confID}`)
      lineupIDs.add(confID)
      const expectedNameKey = `world.${id}.customLineup.${confID}.name`
      const expectedDescKey = `world.${id}.customLineup.${confID}.desc`
      if (conf.nameKey !== expectedNameKey) {
        throw new Error(`world.customLineupConf.nameKey invalid for ${id}.${confID}; expected ${expectedNameKey}`)
      }
      if (conf.descKey !== expectedDescKey) {
        throw new Error(`world.customLineupConf.descKey invalid for ${id}.${confID}; expected ${expectedDescKey}`)
      }
      const percent = parseNumberStrict(conf.percent)
      if (!Number.isFinite(percent) || percent < 0 || percent > 1) {
        throw new Error(`world.customLineupConf.percent invalid: ${id}.${confID}`)
      }
      if (typeof conf.isSpecialWaveLineup !== "boolean") {
        throw new Error(`world.customLineupConf.isSpecialWaveLineup invalid: ${id}.${confID}`)
      }
      if (!Array.isArray(conf.lineup) || conf.lineup.length === 0) {
        throw new Error(`world.customLineupConf.lineup required: ${id}.${confID}`)
      }
      for (const enemyID of conf.lineup) {
        if (typeof enemyID !== "string" || !enemyID.trim()) {
          throw new Error(`world.customLineupConf.lineup invalid: ${id}.${confID}`)
        }
        if (knownEnemyIDs.size > 0 && !knownEnemyIDs.has(enemyID.trim())) {
          throw new Error(`world.customLineupConf.lineup enemy not found: ${id}.${confID}.${enemyID}`)
        }
      }
      if (!conf.drops || typeof conf.drops !== "object" || Array.isArray(conf.drops)) {
        throw new Error(`world.customLineupConf.drops invalid: ${id}.${confID}`)
      }
      validateBattleDropItems(conf.drops.items, `world.customLineupConf.drops.items invalid: ${id}.${confID}`)
    }
  }
}

function normalizeEnemies(rows) {
  return (rows || []).map((raw) => {
    const id = typeof raw?.id === "string" ? raw.id.trim() : ""
    const positionRaw = (parseOptionalString(raw?.position) || "").toLowerCase()
    const position = positionRaw === "front"
      ? "vanguard"
      : positionRaw === "middle"
        ? "midguard"
        : positionRaw === "back" || positionRaw === "rear"
          ? "rearguard"
          : (positionRaw || "vanguard")
    const uniqueSkillIDs = Array.from(
      new Set(
        parseIDList(raw?.skillIds)
          .map((item) => item.trim())
          .filter(Boolean)
      )
    )
    const nonBasicSkillIDs = uniqueSkillIDs.filter((item) => item !== "basic_attack")
    const normalizedSkillIDs = uniqueSkillIDs.includes("basic_attack")
      ? [...nonBasicSkillIDs, "basic_attack"]
      : nonBasicSkillIDs
    const level = Math.max(1, Math.floor(parseNumber(raw?.level, 1)))
    const typeRaw = (parseOptionalString(raw?.type) || parseOptionalString(raw?.enemyType) || "").toLowerCase()
    const inferredType = id.includes("boss") ? "boss" : id.includes("elite") ? "elite" : "normal"
    const type = typeRaw === "elite" || typeRaw === "boss" || typeRaw === "normal" ? typeRaw : inferredType
    const equipment = Array.isArray(raw?.equipment)
      ? raw.equipment
          .map((entry) => {
            if (!entry || typeof entry !== "object" || Array.isArray(entry)) return null
            const templateItemId = parseOptionalString(entry.templateItemId || entry.itemId || entry.id) || ""
            if (!templateItemId) return null
            const baseSnapshot = parseBaseAttrs(entry.baseSnapshot)
            const affixes = parseDefaultAffixSlots(entry.affixes)
            return {
              templateItemId,
              equippedSlot: parseOptionalString(entry.equippedSlot) || "",
              equipType: parseOptionalString(entry.equipType) || "",
              enhanceLevel: Math.max(0, Math.floor(parseNumber(entry.enhanceLevel, 0))),
              baseSnapshot: Object.keys(baseSnapshot).length > 0 ? baseSnapshot : undefined,
              affixes: affixes.length > 0 ? affixes : undefined
            }
          })
          .filter(Boolean)
      : []
    return {
      ...raw,
      id,
      disabled: parseBoolean(raw?.disabled, false),
      nameKey: parseOptionalString(raw?.nameKey) || (id ? `enemy.${id}.name` : ""),
      descKey: parseOptionalString(raw?.descKey) || (id ? `enemy.${id}.desc` : ""),
      position,
      level,
      type,
      skillIds: normalizedSkillIDs,
      equipment: equipment.length > 0 ? equipment : undefined
    }
  })
}

function normalizeWorld(rows) {
  return (rows || []).map((raw) => {
    const id = typeof raw?.id === "string" ? raw.id.trim() : ""
    const specialWaves = parseWorldSpecialWaves(raw?.specialWaves).normalized
    const normalEnemyCount = parseWorldEnemyCountRange(raw?.normalEnemyCount, 0, 0)
    const extraEnemyPerPlayer = parseWorldEnemyCountRange(raw?.extraEnemyPerPlayer, 0, 0)
    const waveSchedules = normalizeWorldWaveSchedules(raw?.waveSchedules, id)
    return {
      ...raw,
      id,
      disabled: parseBoolean(raw?.disabled, false),
      specialWaves,
      waveSchedules,
      normalEnemyCount,
      extraEnemyPerPlayer,
      customLineupConf: normalizeWorldCustomLineupConf(raw?.customLineupConf, id),
      nameKey: parseOptionalString(raw?.nameKey) || (id ? `world.${id}.name` : ""),
      descKey: parseOptionalString(raw?.descKey) || (id ? `world.${id}.desc` : "")
    }
  })
}

function normalizeAttributes(rows) {
  return (rows || []).map((raw, index) => {
    const id = typeof raw?.id === "string" ? raw.id.trim() : ""
    const nameKey = parseOptionalString(raw?.nameKey) || (id ? `attr.${id}.name` : "")
    const descKey = parseOptionalString(raw?.descKey) || (id ? `attr.${id}.desc` : "")
    const relatedActionMainType = parseOptionalString(raw?.relatedActionMainType) || ""
    const relatedActionEffect = parseOptionalString(raw?.relatedActionEffect) || (relatedActionMainType ? "actionEfficiency" : "")
    return {
      ...raw,
      id,
      icon: parseOptionalString(raw?.icon) || "",
      iconId: parseOptionalString(raw?.iconId) || "",
      emojiIcon: parseOptionalString(raw?.emojiIcon) || "",
      nameKey,
      descKey,
      baseDefault: parseNumber(raw?.baseDefault, 0),
      growthType: parseOptionalString(raw?.growthType) || "",
      expProfileId: parseOptionalString(raw?.expProfileId) || "",
      unit: parseOptionalString(raw?.unit) || "number",
      order: parseNumber(raw?.order, index + 1),
      category: parseOptionalString(raw?.category) || "",
      allowBattlePointLevelUp: parseBoolean(raw?.allowBattlePointLevelUp, false),
      relatedActionMainType,
      relatedActionEffect
    }
  })
}

function normalizeRarityDefinitions(rows) {
  return (rows || [])
    .map((raw, index) => {
      const id = typeof raw?.id === "string" ? raw.id.trim() : ""
      if (!id) return null
      return {
        ...raw,
        id,
        nameKey: parseOptionalString(raw?.nameKey) || `rarity.${id}`,
        order: Math.max(1, Math.round(parseNumber(raw?.order, index + 1))),
        color: parseOptionalString(raw?.color) || "",
        economyMultiplier: parseNumber(raw?.economyMultiplier, 1)
      }
    })
    .filter(Boolean)
}

function normalizeItemTypes(rows) {
  return (rows || [])
    .map((raw) => {
      const id = typeof raw?.id === "string" ? raw.id.trim() : ""
      if (!id) return null
      return {
        ...raw,
        id,
        nameKey: parseOptionalString(raw?.nameKey) || `item_type.${id}.name`,
        descKey: parseOptionalString(raw?.descKey) || `item_type.${id}.desc`
      }
    })
    .filter(Boolean)
}

function stripLegacyDurationModifierFields(input) {
  if (Array.isArray(input)) {
    return input.map((entry) => stripLegacyDurationModifierFields(entry))
  }
  if (!input || typeof input !== "object") {
    return input
  }
  const out = {}
  for (const [key, value] of Object.entries(input)) {
    if (key === "actionDurationModifiers" || key === "durationModifiers") {
      continue
    }
    out[key] = stripLegacyDurationModifierFields(value)
  }
  return out
}

function normalizeAffixDefinitions(rows) {
  const normalizeAffixType = (rawType) => {
    const normalized = parseOptionalString(rawType)?.toLowerCase()
    if (normalized === "work") return "work"
    if (normalized === "battle") return "battle"
    return ""
  }
  return (rows || [])
    .map((raw) => {
      const { actionDurationModifiers: _legacyActionDurationModifiers, ...rawWithoutLegacyDuration } = raw || {}
      const id = typeof raw?.id === "string" ? raw.id.trim() : ""
      if (!id) return null
      const defaultNameKey = id.startsWith("affix.") ? `${id}.name` : `affix.${id}.name`
      const defaultDescKey = id.startsWith("affix.") ? `${id}.desc` : `affix.${id}.desc`
      const effectParams =
        raw?.effectParams && typeof raw.effectParams === "object" && !Array.isArray(raw.effectParams)
          ? stripLegacyDurationModifierFields(raw.effectParams)
          : {}
      const effectType = parseOptionalString(raw?.effectType) || ""
      const effectTypeKey = effectType.trim().toLowerCase()
      const normalizedEffectParams = { ...effectParams }
      if (!Array.isArray(normalizedEffectParams.effectMutations) && Array.isArray(normalizedEffectParams.effect_mutations)) {
        normalizedEffectParams.effectMutations = normalizedEffectParams.effect_mutations
      }
      delete normalizedEffectParams.effect_mutations
      if (effectTypeKey === "battle_trigger" || effectTypeKey === "battle_hook") {
        const hooks = normalizeBattleHookParams(effectParams?.battleHooks || effectParams?.hooks)
        normalizedEffectParams.battleHooks = hooks
      }
      const normalizedEffectType = effectTypeKey === "effectmutation" || effectTypeKey === "effect_mutation" ? "effectMutation" : effectType
      const manualScore = raw?.manualScore === undefined || raw?.manualScore === null || raw?.manualScore === ""
        ? undefined
        : Math.max(0, parseNumber(raw?.manualScore, 0))
      return {
        ...rawWithoutLegacyDuration,
        id,
        // Affix i18n keys are generated from id; manual sheet values are ignored.
        nameKey: defaultNameKey,
        descKey: defaultDescKey,
        effectType: normalizedEffectType,
        effectParams: normalizedEffectParams,
        type: normalizeAffixType(raw?.type),
        valuePrecision: Math.max(0, Math.round(parseNumber(raw?.valuePrecision, 0))),
        ...(manualScore === undefined ? {} : { manualScore }),
        maxEffectivePerEquipment: Math.max(0, Math.round(parseNumber(raw?.maxEffectivePerEquipment, 0))),
        maxEffectivePerBody: Math.max(0, Math.round(parseNumber(raw?.maxEffectivePerBody, 0))),
        allowedRarities: parseStringArray(raw?.allowedRarities) || [],
        active: parseBoolean(raw?.active, true)
      }
    })
    .filter(Boolean)
}

function validateAffixDefinitions(rows) {
  if (!Array.isArray(rows)) throw new Error("affix_definitions must be an array")
  const ids = new Set()
  for (const row of rows) {
    if (!row || typeof row !== "object") throw new Error("affix_definitions row must be object")
    if (typeof row.id !== "string" || row.id === "") throw new Error("affix_definitions.id must be non-empty string")
    if (ids.has(row.id)) throw new Error(`duplicate affix_definitions.id: ${row.id}`)
    ids.add(row.id)
    if (typeof row.nameKey !== "string" || row.nameKey === "") throw new Error(`affix_definitions.nameKey invalid: ${row.id}`)
    if (typeof row.descKey !== "string" || row.descKey === "") throw new Error(`affix_definitions.descKey invalid: ${row.id}`)
    if (typeof row.effectType !== "string" || row.effectType === "") throw new Error(`affix_definitions.effectType invalid: ${row.id}`)
    if (!row.effectParams || typeof row.effectParams !== "object" || Array.isArray(row.effectParams)) {
      throw new Error(`affix_definitions.effectParams invalid: ${row.id}`)
    }
    const effectTypeKey = row.effectType.trim().toLowerCase()
    const effectMutationType = effectTypeKey === "effectmutation" || effectTypeKey === "effect_mutation"
    const effectMutations = row.effectParams.effectMutations
    const hasEffectMutations = Array.isArray(effectMutations)
    if (hasEffectMutations) {
      if (!effectMutationType) {
        throw new Error(`affix_definitions.effectType must be effectMutation when effectMutations exists: ${row.id}`)
      }
      const illegalKeys = Object.keys(row.effectParams).filter((key) => key !== "effectMutations")
      if (illegalKeys.length > 0) {
        throw new Error(`affix_definitions.effectParams only allows effectMutations when effectType=effectMutation: ${row.id}`)
      }
    }
    if (effectMutationType && !hasEffectMutations) {
      throw new Error(`affix_definitions.effectParams.effectMutations required when effectType=effectMutation: ${row.id}`)
    }
    if (effectTypeKey === "battle_trigger" || effectTypeKey === "battle_hook") {
      const battleHookGroups = Array.isArray(row.effectParams.battleHooks)
        ? { default: row.effectParams.battleHooks }
        : row.effectParams.battleHooks && typeof row.effectParams.battleHooks === "object"
          ? row.effectParams.battleHooks
          : {}
      if (Object.keys(battleHookGroups).length === 0) {
        throw new Error(`affix_definitions.effectParams.battleHooks required: ${row.id}`)
      }
      for (const hooks of Object.values(battleHookGroups)) {
        if (!Array.isArray(hooks) || hooks.length === 0) {
          throw new Error(`affix_definitions.effectParams.battleHooks required: ${row.id}`)
        }
        for (const hook of hooks) {
          if (!hook || typeof hook !== "object" || Array.isArray(hook)) {
            throw new Error(`affix_definitions.effectParams.battleHooks entry invalid: ${row.id}`)
          }
          const triggerKey = normalizeBuffDSLToken(hook.trigger)
          if (
            triggerKey !== "onaftertakedamage" &&
            triggerKey !== "onafterdealdamage" &&
            triggerKey !== "onafterdealcrit" &&
            triggerKey !== "onafterdealnocrit" &&
            triggerKey !== "onbeforedealdamage" &&
            triggerKey !== "onbeforetakedamage" &&
            triggerKey !== "onbattlestart" &&
            triggerKey !== "onbattleend" &&
            triggerKey !== "onallyaftertakedamage" &&
            triggerKey !== "onafterdealheal" &&
            triggerKey !== "onafterbasicattackdealdamage" &&
            triggerKey !== "onaftersummon" &&
            triggerKey !== "onotherunitcastskill" &&
            triggerKey !== "onotherunitturnend" &&
            triggerKey !== "onunitdeath" &&
            triggerKey !== "onkill"
          ) {
            throw new Error(`affix_definitions.effectParams.battleHooks.trigger invalid: ${row.id}.${hook.trigger}`)
          }
          validateBattleHookSubjectFilter(hook.subjectFilter, row.id, "affix_definitions.effectParams.battleHooks.subjectFilter")
          if (!Array.isArray(hook.conditions)) {
            throw new Error(`affix_definitions.effectParams.battleHooks.conditions invalid: ${row.id}`)
          }
          if (!Array.isArray(hook.effects) || hook.effects.length === 0) {
            throw new Error(`affix_definitions.effectParams.battleHooks.effects invalid: ${row.id}`)
          }
          for (const effect of hook.effects) {
            if (!effect || typeof effect !== "object" || Array.isArray(effect)) {
              throw new Error(`affix_definitions.effectParams.battleHooks.effects entry invalid: ${row.id}`)
            }
            const effectKey = normalizeBuffDSLToken(effect.kind || effect.type)
            if (
              effectKey !== "addbuff" &&
              effectKey !== "removebuff" &&
              effectKey !== "runtimedamageamp" &&
              effectKey !== "addruntimedamageamp" &&
              effectKey !== "adddamageamp" &&
              effectKey !== "execute" &&
              effectKey !== "consumemp" &&
              effectKey !== "extraturn" &&
              effectKey !== "mirrorcastskill" &&
              effectKey !== "basicattackmarkedtarget" &&
              effectKey !== "summon"
            ) {
              throw new Error(`affix_definitions.effectParams.battleHooks.effects.kind invalid: ${row.id}.${effect.kind || effect.type}`)
            }
            if ((effectKey === "addbuff" || effectKey === "removebuff") && (typeof effect.buffId !== "string" || effect.buffId.trim() === "")) {
              throw new Error(`affix_definitions.effectParams.battleHooks.effects.buffId invalid: ${row.id}`)
            }
            if (effectKey === "mirrorcastskill") {
              validateOptionalStringArray(effect.skillIds, `affix_definitions.effectParams.battleHooks.effects.skillIds invalid: ${row.id}`)
              validateOptionalStringArray(effect.skillCategories, `affix_definitions.effectParams.battleHooks.effects.skillCategories invalid: ${row.id}`)
              validateOptionalBoolean(effect.includeBasicAttack, `affix_definitions.effectParams.battleHooks.effects.includeBasicAttack invalid: ${row.id}`)
              validateOptionalBoolean(effect.allowMirroredCastTrigger, `affix_definitions.effectParams.battleHooks.effects.allowMirroredCastTrigger invalid: ${row.id}`)
              validateOptionalBoolean(effect.applyCooldown, `affix_definitions.effectParams.battleHooks.effects.applyCooldown invalid: ${row.id}`)
            }
            if (effectKey === "extraturn") {
              validateBattleHookExtraTurnEffect(effect, row.id, "affix_definitions.effectParams.battleHooks.effects")
            }
            if (
              (effectKey === "runtimedamageamp" || effectKey === "addruntimedamageamp" || effectKey === "adddamageamp") &&
              (typeof effect.value !== "number" || !Number.isFinite(effect.value))
            ) {
              throw new Error(`affix_definitions.effectParams.battleHooks.effects.value invalid: ${row.id}.${effect.kind || effect.type}`)
            }
            if ((effectKey === "runtimedamageamp" || effectKey === "addruntimedamageamp" || effectKey === "adddamageamp") && effect.valueFormula !== undefined) {
              if (!effect.valueFormula || typeof effect.valueFormula !== "object" || Array.isArray(effect.valueFormula)) {
                throw new Error(`affix_definitions.effectParams.battleHooks.effects.valueFormula invalid: ${row.id}`)
              }
              if (typeof effect.valueFormula.baseAdd !== "number" || !Number.isFinite(effect.valueFormula.baseAdd)) {
                throw new Error(`affix_definitions.effectParams.battleHooks.effects.valueFormula.baseAdd invalid: ${row.id}`)
              }
              if (!Array.isArray(effect.valueFormula.terms)) {
                throw new Error(`affix_definitions.effectParams.battleHooks.effects.valueFormula.terms invalid: ${row.id}`)
              }
            }
            if (effectKey === "summon") {
              if (!effect.summonPayload || typeof effect.summonPayload !== "object" || Array.isArray(effect.summonPayload)) {
                throw new Error(`affix_definitions.effectParams.battleHooks.effects.summonPayload invalid: ${row.id}`)
              }
              if (typeof effect.summonPayload.unitId !== "string" || effect.summonPayload.unitId.trim() === "") {
                throw new Error(`affix_definitions.effectParams.battleHooks.effects.summonPayload.unitId invalid: ${row.id}`)
              }
              if (effect.summonPayload.maxCountPerUnit !== undefined && (!Number.isInteger(effect.summonPayload.maxCountPerUnit) || effect.summonPayload.maxCountPerUnit < 0)) {
                throw new Error(`affix_definitions.effectParams.battleHooks.effects.summonPayload.maxCountPerUnit invalid: ${row.id}`)
              }
              if (effect.summonPayload.maxCountFormula !== undefined) {
                validateSummonMaxCountFormula(effect.summonPayload.maxCountFormula, `affix_definitions.effectParams.battleHooks.effects.summonPayload.maxCountFormula: ${row.id}`)
              }
              if (effect.summonPayload.durationTurns !== undefined && (!Number.isFinite(effect.summonPayload.durationTurns) || effect.summonPayload.durationTurns < 0 || !Number.isInteger(effect.summonPayload.durationTurns))) {
                throw new Error(`affix_definitions.effectParams.battleHooks.effects.summonPayload.durationTurns invalid: ${row.id}`)
              }
              if (effect.summonPayload.durationFormula !== undefined) {
                if (!effect.summonPayload.durationFormula || typeof effect.summonPayload.durationFormula !== "object" || Array.isArray(effect.summonPayload.durationFormula)) {
                  throw new Error(`affix_definitions.effectParams.battleHooks.effects.summonPayload.durationFormula invalid: ${row.id}`)
                }
                if (!Number.isFinite(effect.summonPayload.durationFormula.baseAdd) || !Array.isArray(effect.summonPayload.durationFormula.terms)) {
                  throw new Error(`affix_definitions.effectParams.battleHooks.effects.summonPayload.durationFormula invalid: ${row.id}`)
                }
              }
            }
          }
        }
      }
    }
    if (typeof row.valuePrecision !== "number" || !Number.isFinite(row.valuePrecision) || row.valuePrecision < 0) {
      throw new Error(`affix_definitions.valuePrecision invalid: ${row.id}`)
    }
    if (row.manualScore !== undefined && (typeof row.manualScore !== "number" || !Number.isFinite(row.manualScore) || row.manualScore < 0)) {
      throw new Error(`affix_definitions.manualScore invalid: ${row.id}`)
    }
    if (
      typeof row.maxEffectivePerEquipment !== "number" ||
      !Number.isFinite(row.maxEffectivePerEquipment) ||
      row.maxEffectivePerEquipment < 0 ||
      !Number.isInteger(row.maxEffectivePerEquipment)
    ) {
      throw new Error(`affix_definitions.maxEffectivePerEquipment invalid: ${row.id}`)
    }
    if (
      typeof row.maxEffectivePerBody !== "number" ||
      !Number.isFinite(row.maxEffectivePerBody) ||
      row.maxEffectivePerBody < 0 ||
      !Number.isInteger(row.maxEffectivePerBody)
    ) {
      throw new Error(`affix_definitions.maxEffectivePerBody invalid: ${row.id}`)
    }
    if (typeof row.type !== "string" || (row.type !== "work" && row.type !== "battle")) {
      throw new Error(`affix_definitions.type invalid: ${row.id}`)
    }
    if (!Array.isArray(row.allowedRarities) || row.allowedRarities.some((rarityID) => typeof rarityID !== "string" || rarityID.trim() === "")) {
      throw new Error(`affix_definitions.allowedRarities invalid: ${row.id}`)
    }
    if (typeof row.active !== "boolean") throw new Error(`affix_definitions.active invalid: ${row.id}`)
  }
}

function normalizeAffixEnchantRules(rows) {
  const normalizeScope = (rawScope) => {
    if (!rawScope || typeof rawScope !== "object" || Array.isArray(rawScope)) return null
    const equipTypeRaw = parseOptionalString(rawScope?.equipType)
    const slots = parseCommaSeparatedArray(rawScope?.slots)
    const weaponClasses = parseCommaSeparatedArray(rawScope?.weaponClasses)
    return {
      equipType: equipTypeRaw ? equipTypeRaw.toLowerCase() : "",
      slots,
      weaponClasses,
      allowedAffixIds: parseCommaSeparatedArray(rawScope?.allowedAffixIds)
    }
  }
  const parseScopes = (rawScopes) => {
    if (!rawScopes) return []
    if (Array.isArray(rawScopes)) {
      return rawScopes.map((item) => normalizeScope(item)).filter(Boolean)
    }
    if (typeof rawScopes === "object") {
      const normalized = normalizeScope(rawScopes)
      return normalized ? [normalized] : []
    }
    if (typeof rawScopes === "string" && rawScopes.trim() !== "") {
      try {
        return parseScopes(JSON.parse(rawScopes))
      } catch {
        return []
      }
    }
    return []
  }
  return (rows || [])
    .map((raw) => {
      const id = typeof raw?.id === "string" ? raw.id.trim() : ""
      if (!id) return null
      return {
        ...raw,
        id,
        toolItemId: parseOptionalString(raw?.toolItemId) || "",
        consumeCount: Math.max(1, Math.round(parseNumber(raw?.consumeCount, 1))),
        costs: parseNumberMap(raw?.costs),
        failChance: Math.max(0, Math.min(1, parseNumber(raw?.failChance, 0))),
        rarityWeights: parseNumberMap(raw?.rarityWeights),
        allowedAffixIds: parseCommaSeparatedArray(raw?.allowedAffixIds),
        allowedAffixScopes: parseScopes(raw?.allowedAffixScopes),
        active: parseBoolean(raw?.active, true)
      }
    })
    .filter(Boolean)
}

function validateAffixEnchantRules(rows, equipmentTemplates, weaponClasses) {
  if (!Array.isArray(rows)) throw new Error("affix_enchant_rules must be an array")
  if (!Array.isArray(equipmentTemplates)) throw new Error("equipment_templates must be an array")
  if (!Array.isArray(weaponClasses)) throw new Error("weapon_classes must be an array")

  const slotsByEquipType = {
    battle: new Set(),
    work: new Set()
  }
  const validWeaponClasses = new Set()
  for (const row of weaponClasses) {
    if (!row || typeof row !== "object") continue
    const id = typeof row.id === "string" ? row.id.trim() : ""
    if (id) validWeaponClasses.add(id)
  }
  for (const template of equipmentTemplates) {
    if (!template || typeof template !== "object") continue
    if (template.equipType !== "battle" && template.equipType !== "work") continue
    const slot = typeof template.slot === "string" ? template.slot.trim() : ""
    if (!slot) continue
    slotsByEquipType[template.equipType].add(slot)
  }

  const ids = new Set()
  for (const row of rows) {
    if (!row || typeof row !== "object") throw new Error("affix_enchant_rules row must be object")
    if (typeof row.id !== "string" || row.id.trim() === "") throw new Error("affix_enchant_rules.id must be non-empty string")
    if (ids.has(row.id)) throw new Error(`duplicate affix_enchant_rules.id: ${row.id}`)
    ids.add(row.id)
    if (typeof row.toolItemId !== "string" || row.toolItemId.trim() === "") throw new Error(`affix_enchant_rules.toolItemId invalid: ${row.id}`)
    if (typeof row.consumeCount !== "number" || !Number.isFinite(row.consumeCount) || !Number.isInteger(row.consumeCount) || row.consumeCount <= 0) {
      throw new Error(`affix_enchant_rules.consumeCount invalid: ${row.id}`)
    }
    if (!row.costs || typeof row.costs !== "object" || Array.isArray(row.costs)) {
      throw new Error(`affix_enchant_rules.costs invalid: ${row.id}`)
    }
    for (const [itemID, qty] of Object.entries(row.costs)) {
      if (typeof itemID !== "string" || itemID.trim() === "") {
        throw new Error(`affix_enchant_rules.costs item id invalid: ${row.id}`)
      }
      if (typeof qty !== "number" || !Number.isFinite(qty) || !Number.isInteger(qty) || qty < 0) {
        throw new Error(`affix_enchant_rules.costs invalid: ${row.id}.${itemID}`)
      }
    }
    if (typeof row.failChance !== "number" || !Number.isFinite(row.failChance) || row.failChance < 0 || row.failChance > 1) {
      throw new Error(`affix_enchant_rules.failChance invalid: ${row.id}`)
    }
    if (!row.rarityWeights || typeof row.rarityWeights !== "object" || Array.isArray(row.rarityWeights)) {
      throw new Error(`affix_enchant_rules.rarityWeights invalid: ${row.id}`)
    }
    if (!Array.isArray(row.allowedAffixIds) || row.allowedAffixIds.some((item) => typeof item !== "string" || item.trim() === "")) {
      throw new Error(`affix_enchant_rules.allowedAffixIds invalid: ${row.id}`)
    }
    if (typeof row.active !== "boolean") throw new Error(`affix_enchant_rules.active invalid: ${row.id}`)
    if (!Array.isArray(row.allowedAffixScopes)) {
      throw new Error(`affix_enchant_rules.allowedAffixScopes invalid: ${row.id}`)
    }
    const scopeKeys = new Set()
    for (let index = 0; index < row.allowedAffixScopes.length; index += 1) {
      const scope = row.allowedAffixScopes[index]
      if (!scope || typeof scope !== "object" || Array.isArray(scope)) {
        throw new Error(`affix_enchant_rules.allowedAffixScopes entry invalid: ${row.id}[${index}]`)
      }
      if (scope.equipType !== "battle" && scope.equipType !== "work") {
        throw new Error(`affix_enchant_rules.allowedAffixScopes.equipType invalid: ${row.id}[${index}]`)
      }
      if (!Array.isArray(scope.slots) || scope.slots.some((slot) => typeof slot !== "string" || slot.trim() === "")) {
        throw new Error(`affix_enchant_rules.allowedAffixScopes.slots invalid: ${row.id}[${index}]`)
      }
      if (!Array.isArray(scope.weaponClasses) || scope.weaponClasses.some((weaponClass) => typeof weaponClass !== "string" || weaponClass.trim() === "")) {
        throw new Error(`affix_enchant_rules.allowedAffixScopes.weaponClasses invalid: ${row.id}[${index}]`)
      }
      if (!Array.isArray(scope.allowedAffixIds) || scope.allowedAffixIds.some((item) => typeof item !== "string" || item.trim() === "")) {
        throw new Error(`affix_enchant_rules.allowedAffixScopes.allowedAffixIds invalid: ${row.id}[${index}]`)
      }
      const normalizedSlots = scope.slots.map((slot) => slot.trim())
      const normalizedWeaponClasses = scope.weaponClasses.map((weaponClass) => weaponClass.trim())
      const slotsInEquipType = slotsByEquipType[scope.equipType]
      const localWeaponClasses = new Set()
      for (const weaponClass of normalizedWeaponClasses) {
        if (localWeaponClasses.has(weaponClass)) {
          throw new Error(`affix_enchant_rules.allowedAffixScopes duplicate weaponClass in one scope: ${row.id}.${scope.equipType}.${weaponClass}`)
        }
        localWeaponClasses.add(weaponClass)
        if (!validWeaponClasses.has(weaponClass)) {
          throw new Error(`affix_enchant_rules.allowedAffixScopes weaponClass not found: ${row.id}.${scope.equipType}.${weaponClass}`)
        }
      }
      const slotKeys = normalizedSlots.length === 0 ? ["*"] : normalizedSlots
      const weaponClassKeys = normalizedWeaponClasses.length === 0 ? ["*"] : normalizedWeaponClasses
      if (normalizedSlots.length > 0) {
        const localSlots = new Set()
        for (const slot of normalizedSlots) {
          if (localSlots.has(slot)) {
            throw new Error(`affix_enchant_rules.allowedAffixScopes duplicate slot in one scope: ${row.id}.${scope.equipType}.${slot}`)
          }
          localSlots.add(slot)
          if (!slotsInEquipType.has(slot)) {
            throw new Error(`affix_enchant_rules.allowedAffixScopes slot not found: ${row.id}.${scope.equipType}.${slot}`)
          }
        }
      }
      for (const slotKey of slotKeys) {
        for (const weaponClassKey of weaponClassKeys) {
          const key = `${scope.equipType}::${slotKey}::${weaponClassKey}`
          if (scopeKeys.has(key)) {
            throw new Error(`affix_enchant_rules.allowedAffixScopes duplicate scope: ${row.id}.${scope.equipType}.${slotKey}.${weaponClassKey}`)
          }
          scopeKeys.add(key)
        }
      }
    }
  }
}

function normalizeAffixCarrierRules(rows) {
  return (rows || [])
    .map((raw) => {
      const id = typeof raw?.id === "string" ? raw.id.trim() : ""
      if (!id) return null
      return {
        ...raw,
        id,
        carrierItemId: parseOptionalString(raw?.carrierItemId) || "",
        allowedRarityIds: parseStringArray(raw?.allowedRarityIds) || [],
        maxCustomAffixes: Math.max(0, Math.round(parseNumber(raw?.maxCustomAffixes, 0))),
        allowedTraitKind: ["life", "combat", "both"].includes(parseOptionalString(raw?.allowedTraitKind))
          ? parseOptionalString(raw?.allowedTraitKind)
          : "both",
        active: parseBoolean(raw?.active, true)
      }
    })
    .filter(Boolean)
}

function validateRarityDefinitions(rarityDefinitions) {
  if (!Array.isArray(rarityDefinitions)) throw new Error("rarity_definitions must be an array")
  const ids = new Set()
  const orders = new Set()
  for (const row of rarityDefinitions) {
    if (!row || typeof row !== "object") throw new Error("rarity_definitions row must be object")
    if (typeof row.id !== "string" || row.id === "") throw new Error("rarity_definitions.id must be non-empty string")
    if (ids.has(row.id)) throw new Error(`duplicate rarity_definitions.id: ${row.id}`)
    ids.add(row.id)
    if (typeof row.nameKey !== "string" || row.nameKey === "") throw new Error(`rarity_definitions.nameKey invalid: ${row.id}`)
    if (typeof row.order !== "number" || !Number.isFinite(row.order)) throw new Error(`rarity_definitions.order invalid: ${row.id}`)
    if (orders.has(row.order)) throw new Error(`duplicate rarity_definitions.order: ${row.order}`)
    orders.add(row.order)
    if (typeof row.color !== "string") throw new Error(`rarity_definitions.color invalid: ${row.id}`)
    if (typeof row.economyMultiplier !== "number" || !Number.isFinite(row.economyMultiplier) || row.economyMultiplier <= 0) {
      throw new Error(`rarity_definitions.economyMultiplier invalid: ${row.id}`)
    }
  }
}

function validateActionRewardAttributes(actions, attributes) {
  if (!Array.isArray(actions)) return
  if (!Array.isArray(attributes) || attributes.length === 0) return
  const attrMap = new Map()
  for (const row of attributes || []) {
    if (!row || typeof row !== "object") continue
    const id = typeof row.id === "string" ? row.id.trim() : ""
    if (!id) continue
    const growthType = typeof row.growthType === "string" ? row.growthType.trim() : ""
    attrMap.set(id, growthType)
  }
  for (const row of actions) {
    if (!row || typeof row !== "object") continue
    const rewards = row.statusExpReward
    if (!rewards || typeof rewards !== "object") continue
    for (const attrID of Object.keys(rewards)) {
      if (!attrMap.has(attrID)) {
        throw new Error(`action.statusExpReward attr not found: ${row.id}.${attrID}`)
      }
      if (attrMap.get(attrID) !== "levelable") {
        throw new Error(`action.statusExpReward attr is not levelable: ${row.id}.${attrID}`)
      }
    }
  }
}

function validateStatusRequirementAttributes(equipmentTemplates, actions, attributes) {
  if (!Array.isArray(attributes) || attributes.length === 0) return
  const attrSet = new Set(
    attributes
      .map((row) => (typeof row?.id === "string" ? row.id.trim() : ""))
      .filter(Boolean)
  )
  const validateRequire = (kind, id, value) => {
    if (!value || typeof value !== "object" || Array.isArray(value)) return
    for (const [attrID, required] of Object.entries(value)) {
      if (!attrSet.has(attrID)) {
        throw new Error(`${kind} attr not found: ${id}.${attrID}`)
      }
      if (typeof required !== "number" || !Number.isFinite(required) || required < 0) {
        throw new Error(`${kind} value invalid: ${id}.${attrID}`)
      }
    }
  }
  for (const template of equipmentTemplates || []) {
    if (!template || typeof template !== "object") continue
    validateRequire("equipment_templates.statusRequirement", template.id, template.statusRequirement)
  }
  for (const action of actions || []) {
    if (!action || typeof action !== "object") continue
    validateRequire("action.statusRequirement", action.id, action.statusRequirement)
  }
}

function validateActionBuildingRequirements(actions, buildings) {
  if (!Array.isArray(actions)) return
  if (!Array.isArray(buildings) || buildings.length === 0) return
  const buildingSet = new Set(
    buildings
      .map((row) => (typeof row?.id === "string" ? row.id.trim() : ""))
      .filter(Boolean)
  )
  for (const action of actions || []) {
    if (!action || typeof action !== "object") continue
    const requirement = action.buildingRequirement
    if (!requirement || typeof requirement !== "object" || Array.isArray(requirement)) continue
    for (const [buildingID, required] of Object.entries(requirement)) {
      if (typeof required !== "number" || !Number.isFinite(required) || required < 0) {
        throw new Error(`action.buildingRequirement value invalid: ${action.id}.${buildingID}`)
      }
      if (required <= 0) continue
      if (!buildingSet.has(buildingID)) {
        throw new Error(`action.buildingRequirement building not found: ${action.id}.${buildingID}`)
      }
    }
  }
}

function validateSkillStatusRequirementAttributes(skills, attributes) {
  if (!Array.isArray(attributes) || attributes.length === 0) return
  const attrSet = new Set(
    attributes
      .map((row) => (typeof row?.id === "string" ? row.id.trim() : ""))
      .filter(Boolean)
  )
  for (const skill of skills || []) {
    if (!skill || typeof skill !== "object") continue
    if (!skill.statusRequirement || typeof skill.statusRequirement !== "object" || Array.isArray(skill.statusRequirement)) continue
    for (const [attrID, required] of Object.entries(skill.statusRequirement)) {
      if (!attrSet.has(attrID)) {
        throw new Error(`skills.statusRequirement attr not found: ${skill.id}.${attrID}`)
      }
      if (typeof required !== "number" || !Number.isFinite(required) || required < 0) {
        throw new Error(`skills.statusRequirement value invalid: ${skill.id}.${attrID}`)
      }
    }
  }
}

function validateSkillDamageFormulaAttributes(skills, attributes) {
  if (!Array.isArray(attributes) || attributes.length === 0) return
  const attrSet = new Set(
    attributes
      .map((row) => (typeof row?.id === "string" ? row.id.trim() : ""))
      .filter(Boolean)
  )
  const builtinAttrSet = new Set(["physics_damage", "magical_damage"])
  const corpseExplosionBuiltinAttrSet = new Set(["corpse_max_hp"])
  const summonBuiltinAttrSet = new Set(["equipment_enhance_level"])
  const validateAttrKey = (skillID, attrKey, fieldPath) => {
    if (!attrKey) return
    if (!attrSet.has(attrKey) && !builtinAttrSet.has(attrKey)) {
      throw new Error(`${fieldPath} attr not found: ${skillID}.${attrKey}`)
    }
  }
  for (const skill of skills || []) {
    if (!skill || typeof skill !== "object") continue
    if (skill.targeting && typeof skill.targeting === "object" && !Array.isArray(skill.targeting)) {
      const selectRule = String(skill.targeting.selectRule || "").trim()
      if ((selectRule === "attr_max" || selectRule === "attr_min") && typeof skill.targeting.selectAttrKey === "string") {
        validateAttrKey(skill.id, String(skill.targeting.selectAttrKey).trim(), "skills.targeting.selectAttrKey")
      }
    }
    if (!Array.isArray(skill.effects)) continue
    for (const effect of skill.effects) {
      if (!effect || typeof effect !== "object" || Array.isArray(effect)) continue
      if (effect.targeting && typeof effect.targeting === "object" && !Array.isArray(effect.targeting)) {
        const selectRule = String(effect.targeting.selectRule || "").trim()
        if ((selectRule === "attr_max" || selectRule === "attr_min") && typeof effect.targeting.selectAttrKey === "string") {
          validateAttrKey(skill.id, String(effect.targeting.selectAttrKey).trim(), "skills.effects.targeting.selectAttrKey")
        }
        if (Array.isArray(effect.targeting.targetPredicates)) {
          for (const predicate of effect.targeting.targetPredicates) {
            if (!predicate || typeof predicate !== "object" || Array.isArray(predicate)) continue
            if (predicate.kind !== "compare_attr") continue
            validateAttrKey(skill.id, String(predicate.actorAttr || "").trim(), "skills.effects.targeting.targetPredicates.actorAttr")
            validateAttrKey(skill.id, String(predicate.targetAttr || "").trim(), "skills.effects.targeting.targetPredicates.targetAttr")
          }
        }
      }
      if (effect.corpseTargeting && typeof effect.corpseTargeting === "object" && !Array.isArray(effect.corpseTargeting)) {
        const selectRule = String(effect.corpseTargeting.selectRule || "").trim()
        if ((selectRule === "attr_max" || selectRule === "attr_min") && typeof effect.corpseTargeting.selectAttrKey === "string") {
          validateAttrKey(skill.id, String(effect.corpseTargeting.selectAttrKey).trim(), "skills.effects.corpseTargeting.selectAttrKey")
        }
      }
      if ((effect.kind === "damage" || effect.kind === "heal" || effect.kind === "corpse_explosion") && effect.amountFormula && typeof effect.amountFormula === "object" && Array.isArray(effect.amountFormula.terms)) {
        for (const term of effect.amountFormula.terms) {
          const attrKey = String(term?.attrKey || "").trim()
          if (!attrKey) continue
          if (effect.kind === "corpse_explosion" && corpseExplosionBuiltinAttrSet.has(attrKey)) continue
          validateAttrKey(skill.id, attrKey, "skills.effects.amountFormula")
        }
      }
      if (effect.kind === "status" && effect.statusPayload && typeof effect.statusPayload === "object" && effect.statusPayload.durationFormula && Array.isArray(effect.statusPayload.durationFormula.terms)) {
        for (const term of effect.statusPayload.durationFormula.terms) {
          const attrKey = String(term?.attrKey || "").trim()
          if (!attrKey) continue
          validateAttrKey(skill.id, attrKey, "skills.effects.statusPayload.durationFormula")
        }
      }
      if (effect.kind === "summon" && effect.summonPayload && typeof effect.summonPayload === "object" && !Array.isArray(effect.summonPayload)) {
        const validateSummonAttr = (attrKey, fieldPath) => {
          if (!attrKey) return
          if (!attrSet.has(attrKey) && !summonBuiltinAttrSet.has(attrKey)) {
            throw new Error(`${fieldPath} attr not found: ${skill.id}.${attrKey}`)
          }
        }
        if (effect.summonPayload.countFormula && Array.isArray(effect.summonPayload.countFormula.terms)) {
          for (const term of effect.summonPayload.countFormula.terms) {
            validateSummonAttr(String(term?.attrKey || "").trim(), "skills.effects.summonPayload.countFormula")
          }
        }
        if (effect.summonPayload.maxCountFormula && Array.isArray(effect.summonPayload.maxCountFormula.terms)) {
          for (const term of effect.summonPayload.maxCountFormula.terms) {
            validateSummonAttr(String(term?.attrKey || "").trim(), "skills.effects.summonPayload.maxCountFormula")
          }
        }
        if (effect.summonPayload.durationFormula && Array.isArray(effect.summonPayload.durationFormula.terms)) {
          for (const term of effect.summonPayload.durationFormula.terms) {
            validateSummonAttr(String(term?.attrKey || "").trim(), "skills.effects.summonPayload.durationFormula")
          }
        }
        if (effect.summonPayload.statsFormula && typeof effect.summonPayload.statsFormula === "object" && !Array.isArray(effect.summonPayload.statsFormula)) {
          for (const [statAttrKey, statFormula] of Object.entries(effect.summonPayload.statsFormula)) {
            if (!statAttrKey || !attrSet.has(statAttrKey)) {
              throw new Error(`skills.effects.summonPayload.statsFormula attr not found: ${skill.id}.${statAttrKey}`)
            }
            if (statFormula && typeof statFormula === "object" && Array.isArray(statFormula.terms)) {
              for (const term of statFormula.terms) {
                validateSummonAttr(String(term?.attrKey || "").trim(), `skills.effects.summonPayload.statsFormula.${statAttrKey}`)
              }
            }
          }
        }
      }
    }
  }
}

function validateSkillTargetFormulaAttributes(skills, attributes) {
  if (!Array.isArray(attributes) || attributes.length === 0) return
  const attrSet = new Set(
    attributes
      .map((row) => (typeof row?.id === "string" ? row.id.trim() : ""))
      .filter(Boolean)
  )
  const validateAttrKey = (skillID, attrKey, fieldPath) => {
    if (!attrKey) return
    if (!attrSet.has(attrKey)) {
      throw new Error(`${fieldPath} attr not found: ${skillID}.${attrKey}`)
    }
  }
  for (const skill of skills || []) {
    if (!skill || typeof skill !== "object") continue
    if (skill.targeting && typeof skill.targeting === "object" && !Array.isArray(skill.targeting) && skill.targeting.targetCount && typeof skill.targeting.targetCount === "object" && Array.isArray(skill.targeting.targetCount.formula)) {
      for (const term of skill.targeting.targetCount.formula) {
        const attrKey = String(term?.attrKey || "").trim()
        if (!attrKey) continue
        validateAttrKey(skill.id, attrKey, "skills.targeting.targetCount")
      }
    }
    if (!Array.isArray(skill.effects)) continue
    for (const effect of skill.effects) {
      if (!effect || typeof effect !== "object" || Array.isArray(effect)) continue
      if (!effect.targeting || typeof effect.targeting !== "object" || Array.isArray(effect.targeting)) continue
      if (!effect.targeting.targetCount || typeof effect.targeting.targetCount !== "object" || !Array.isArray(effect.targeting.targetCount.formula)) continue
      for (const term of effect.targeting.targetCount.formula) {
        const attrKey = String(term?.attrKey || "").trim()
        if (!attrKey) continue
        validateAttrKey(skill.id, attrKey, "skills.effects.targeting.targetCount")
      }
      if (effect.corpseTargeting && typeof effect.corpseTargeting === "object" && !Array.isArray(effect.corpseTargeting) && effect.corpseTargeting.targetCount && typeof effect.corpseTargeting.targetCount === "object" && Array.isArray(effect.corpseTargeting.targetCount.formula)) {
        for (const term of effect.corpseTargeting.targetCount.formula) {
          const attrKey = String(term?.attrKey || "").trim()
          if (!attrKey) continue
          validateAttrKey(skill.id, attrKey, "skills.effects.corpseTargeting.targetCount")
        }
      }
    }
  }
}

function validateEquipmentBasicAttackFormulaAttributes(equipmentTemplates, attributes) {
  if (!Array.isArray(attributes) || attributes.length === 0) return
  const attrSet = new Set(
    attributes
      .map((row) => (typeof row?.id === "string" ? row.id.trim() : ""))
      .filter(Boolean)
  )
  const builtinAttrSet = new Set(["physics_damage", "magical_damage"])
  const validateAttrKey = (templateID, attrKey, fieldPath, allowBuiltin = false) => {
    if (!attrKey) return
    const known = attrSet.has(attrKey) || (allowBuiltin && builtinAttrSet.has(attrKey))
    if (!known) {
      throw new Error(`${fieldPath} attr not found: ${templateID}.${attrKey}`)
    }
  }
  const validateFormulaTerms = (templateID, terms, fieldPath, allowBuiltin = false) => {
    if (!Array.isArray(terms)) return
    for (const term of terms) {
      const attrKey = String(term?.attrKey || "").trim()
      if (!attrKey) continue
      validateAttrKey(templateID, attrKey, fieldPath, allowBuiltin)
    }
  }

  for (const template of equipmentTemplates || []) {
    if (!template || typeof template !== "object") continue
    if (!Array.isArray(template.basicAttackEffects)) continue
    const templateID = String(template.id || "")
    for (const effect of template.basicAttackEffects) {
      if (!effect || typeof effect !== "object" || Array.isArray(effect)) continue
      if (effect.targeting && typeof effect.targeting === "object" && !Array.isArray(effect.targeting)) {
        const selectRule = String(effect.targeting.selectRule || "").trim()
        if ((selectRule === "attr_max" || selectRule === "attr_min") && typeof effect.targeting.selectAttrKey === "string") {
          validateAttrKey(templateID, String(effect.targeting.selectAttrKey).trim(), "equipment_templates.basicAttackEffects.targeting.selectAttrKey")
        }
        if (
          effect.targeting.targetCount &&
          typeof effect.targeting.targetCount === "object" &&
          Array.isArray(effect.targeting.targetCount.formula)
        ) {
          validateFormulaTerms(templateID, effect.targeting.targetCount.formula, "equipment_templates.basicAttackEffects.targeting.targetCount")
        }
      }

      if (effect.kind !== "damage" && effect.kind !== "heal") continue
      if (effect.amountFormula && typeof effect.amountFormula === "object" && Array.isArray(effect.amountFormula.terms)) {
        validateFormulaTerms(templateID, effect.amountFormula.terms, "equipment_templates.basicAttackEffects.amountFormula", true)
      }
      if (effect.kind !== "damage") continue
      if (!effect.damageSpec || typeof effect.damageSpec !== "object" || Array.isArray(effect.damageSpec)) continue
      const damageSpecFormulaKeys = [
        "amountFormula",
        "flatDefensePenetrationFormula",
        "percentDefensePenetrationFormula",
        "physicalFlatDefensePenetrationFormula",
        "physicalPercentDefensePenetrationFormula",
        "magicalFlatDefensePenetrationFormula",
        "magicalPercentDefensePenetrationFormula",
        "extraDamageAmplificationFormula"
      ]
      for (const key of damageSpecFormulaKeys) {
        const formula = effect.damageSpec[key]
        if (!formula || typeof formula !== "object" || Array.isArray(formula) || !Array.isArray(formula.terms)) continue
        validateFormulaTerms(templateID, formula.terms, `equipment_templates.basicAttackEffects.damageSpec.${key}`, true)
      }
      if (
        effect.damageSpec.hitCount &&
        typeof effect.damageSpec.hitCount === "object" &&
        Array.isArray(effect.damageSpec.hitCount.formula)
      ) {
        validateFormulaTerms(templateID, effect.damageSpec.hitCount.formula, "equipment_templates.basicAttackEffects.damageSpec.hitCount.formula")
      }
    }

  }
}

function validateBuffDamageFormulaAttributes(buffEffects, attributes) {
  if (!Array.isArray(attributes) || attributes.length === 0 || !Array.isArray(buffEffects)) return
  const attrSet = new Set(
    attributes
      .map((row) => (typeof row?.id === "string" ? row.id.trim() : ""))
      .filter(Boolean)
  )
  const validateAttrKey = (effectID, attrKey) => {
    if (!attrKey) return
    if (!attrSet.has(attrKey)) {
      throw new Error(`buff_effects.effects.damage.amountFormula attr not found: ${effectID}.${attrKey}`)
    }
  }
  for (const row of buffEffects) {
    if (!row || typeof row !== "object" || !Array.isArray(row.effects)) continue
    for (const effect of row.effects) {
      if (!effect || typeof effect !== "object" || Array.isArray(effect)) continue
      const effectKey = normalizeBuffDSLToken(effect.kind || effect.type)
      if (effectKey !== "damage") continue
      if (!effect.amountFormula || typeof effect.amountFormula !== "object" || !Array.isArray(effect.amountFormula.terms)) continue
      for (const term of effect.amountFormula.terms) {
        const attrKey = String(term?.attrKey || "").trim()
        if (!attrKey) continue
        validateAttrKey(row.id, attrKey)
      }
    }
  }
}

function normalizeItems(items) {
  return (items || [])
    .map((raw) => normalizeItem(raw))
    .filter((row) => row.id)
}

function ensureSkillbookItemType(itemTypes, needsSkillbook) {
  const rows = Array.isArray(itemTypes) ? [...itemTypes] : []
  if (!needsSkillbook) return rows
  const exists = rows.some((row) => String(row?.id || "").trim() === "skillbook")
  if (exists) return rows
  rows.push({
    id: "skillbook",
    nameKey: "item_type.skillbook.name",
    descKey: "item_type.skillbook.desc"
  })
  return rows
}

function skillbookItemID(skillID) {
  return `skillbook_${String(skillID || "").trim()}`
}

function defaultSkillbookRarity(rarityDefinitions) {
  const rows = Array.isArray(rarityDefinitions) ? rarityDefinitions : []
  if (rows.some((row) => String(row?.id || "").trim() === "common")) return "common"
  const first = String(rows[0]?.id || "").trim()
  return first || "common"
}

export function shouldGenerateAutoSkillbook(skill) {
  const skillID = String(skill?.id || "").trim()
  return Boolean(skillID && skillID.toLowerCase() !== "basic_attack" && skill?.enemyOnly !== true && skill?.equipmentOnly !== true)
}

function mergeAutoSkillbooks(items, skills, rarityDefinitions) {
  const sourceItems = Array.isArray(items) ? items : []
  const sourceSkills = Array.isArray(skills) ? skills : []
  const out = [...sourceItems]
  const indexByID = new Map(out.map((item, idx) => [String(item?.id || "").trim(), idx]))
  const bookRarity = defaultSkillbookRarity(rarityDefinitions)

  for (const skill of sourceSkills) {
    if (!shouldGenerateAutoSkillbook(skill)) continue
    const skillID = String(skill?.id || "").trim()
    const itemID = skillbookItemID(skillID)
    const autoItem = normalizeItem({
      id: itemID,
      type: "skillbook",
      rarity: bookRarity,
      stackable: true,
      carryableInCombat: false,
      marketListable: true,
      useGrantSkillId: skillID,
      useSkillExpGain: 100
    })
    const hit = indexByID.get(itemID)
    if (hit === undefined) {
      out.push(autoItem)
      indexByID.set(itemID, out.length - 1)
      continue
    }
    const existing = out[hit]
    const merged = { ...existing }
    const typeSet = new Set(Array.isArray(existing.type) ? existing.type : [])
    typeSet.add("skillbook")
    merged.type = [...typeSet]
    if (!parseOptionalString(existing.useGrantSkillId)) {
      merged.useGrantSkillId = skillID
    }
    if (!Number.isFinite(existing.useSkillExpGain) || Number(existing.useSkillExpGain) <= 0) {
      merged.useSkillExpGain = 100
    }
    out[hit] = merged
  }

  return out
}

function normalizeEquipmentTemplate(raw) {
  const {
    actionDurationModifiers: _legacyActionDurationModifiers,
    enhanceSuccessCurve: _legacyEnhanceSuccessCurve,
    enhanceAttrMilestoneOverrides: _legacyEnhanceAttrMilestoneOverrides,
    enhanceAffixMilestonePools: _rawEnhanceAffixMilestonePools,
    ...rawWithoutLegacyDuration
  } = raw || {}
  const id = typeof raw?.id === "string" ? raw.id.trim() : ""
  const equipTypeRaw = (parseOptionalString(raw?.equipType) || "").toLowerCase()
  const equipType = equipTypeRaw === "work" ? "work" : "battle"
  const slot = parseOptionalString(raw?.slot) || ""
  const occupySlots = parseStringArray(raw?.occupySlots) || []
  const equipTargets = parseCommaSeparatedArray(raw?.equipTargets)
  const weaponClass = parseCommaSeparatedArray(raw?.weaponClass)
  const equipmentSecondaryClass = parseOptionalString(raw?.equipmentSecondaryClass) || ""
  const statusRequirement = parseNumberMap(raw?.statusRequirement)
  const accessoryGroup = parseOptionalString(raw?.accessoryGroup) || ""
  const affixSlotCount = Math.max(0, Math.round(parseNumber(raw?.affixSlotCount, 0)))
  const affixAllowedIds = parseStringArray(raw?.affixAllowedIds) || []
  const baseAttrs = parseBaseAttrs(raw?.baseAttrs)
  const maxDurability = Math.max(1, Math.round(parseNumber(raw?.maxDurability, 100)))
  const decayRate = parseNumber(raw?.decayRate, 0)
  const enhanceMaxLevel = Math.max(0, Math.round(parseNumber(raw?.enhanceMaxLevel, 0)))
  const enhanceDurationS = Math.max(1, Math.round(parseNumber(raw?.enhanceDurationS, 1)))
  const enhanceExp = Math.max(0, parseNumber(raw?.enhanceExp, 0))
  const enhanceSuccessRateOverrideRaw = raw?.enhanceSuccessRateOverride
  const enhanceSuccessRateOverride =
    enhanceSuccessRateOverrideRaw === undefined || enhanceSuccessRateOverrideRaw === null || enhanceSuccessRateOverrideRaw === ""
      ? undefined
      : parseNumber(enhanceSuccessRateOverrideRaw, Number.NaN)
  const enhanceProtectItemId = parseCommaSeparatedArray(raw?.enhanceProtectItemId)
  const enhanceCostCurve = parseEnhanceMapCurve(raw?.enhanceCostCurve)
  const enhanceAttrCurveParsed = parseEnhanceMapCurve(raw?.enhanceAttrCurve)
  const enhanceAttrMilestoneOverrides = parseEnhanceAttrMilestoneOverrides(raw?.enhanceAttrMilestoneOverrides)
  const enhanceAttrCurve = {
    ...enhanceAttrCurveParsed,
    overrides: {
      ...enhanceAttrCurveParsed.overrides,
      ...enhanceAttrMilestoneOverrides
    }
  }
  const enhanceAffixMilestonePools = parseEnhanceAffixMilestonePools(raw?.enhanceAffixMilestonePools)
  const basicAttackTargetDirection = parseOptionalString(raw?.basicAttackTargetDirection)
  const basicAttackSeekStrategy = parseOptionalString(raw?.targetSeekStrategy)
    ? normalizeTargetSeekStrategy(raw?.targetSeekStrategy, "")
    : undefined
  const basicAttackFormula = normalizeBasicAttackFormula(raw?.basicAttackFormula)
  const rawBasicAttackEffects = Array.isArray(raw?.basicAttackEffects)
    ? raw.basicAttackEffects
    : buildLegacyBasicAttackEffectsFromFormula({
        basicAttackFormula,
        basicAttackTargetDirection,
        basicAttackSeekStrategy
      })
  const basicAttackEffects = parseSkillEffects(rawBasicAttackEffects, {
    damageType: basicAttackFormula?.damageType || "physical",
    targetDirection: basicAttackTargetDirection || "vanguard",
    targetSeekStrategy: basicAttackSeekStrategy || "front_to_rear",
    targetBaseCount: 1,
    targetFormula: []
  })
  const battleHooks = parseEquipmentBattleHooks(raw?.battleHooks)
  const defaultAffixSlots = parseDefaultAffixSlots(raw?.defaultAffixSlots)
  // const effectiveBattleHooks = applyDefaultShieldBattleHooks(weaponClass, battleHooks)
  return {
    ...rawWithoutLegacyDuration,
    id,
    equipType,
    slot,
    occupySlots,
    equipTargets,
    weaponClass,
    equipmentSecondaryClass,
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
    enhanceSuccessRateOverride,
    enhanceProtectItemId,
    enhanceCostCurve,
    enhanceAttrCurve,
    enhanceAffixMilestonePools: Object.keys(enhanceAffixMilestonePools).length > 0 ? enhanceAffixMilestonePools : undefined,
    basicAttackTargetDirection,
    targetSeekStrategy: basicAttackSeekStrategy,
    basicAttackFormula,
    basicAttackEffects: basicAttackEffects.length > 0 ? basicAttackEffects : undefined,
    defaultAffixSlots: defaultAffixSlots.length > 0 ? defaultAffixSlots : undefined,
    // battleHooks: effectiveBattleHooks.length > 0 ? effectiveBattleHooks : undefined
  }
}

function normalizeEquipmentTemplates(rows) {
  return (rows || []).map((raw) => normalizeEquipmentTemplate(raw)).filter((row) => row.id)
}

function validateEquipmentTemplates(rows, affixDefinitions, rarityDefinitions, equipmentSecondaryClasses, weaponClasses = [], skillTypes = [], buffs = [], skills = [], attributes = [], enemies = []) {
  if (!Array.isArray(rows)) throw new Error("equipment_templates must be an array")
  if (!Array.isArray(affixDefinitions)) throw new Error("affix_definitions must be an array")
  if (!Array.isArray(rarityDefinitions)) throw new Error("rarity_definitions must be an array")
  const knownAffixIDs = new Set()
  const allowedRarityByAffixID = new Map()
  const affixTypeByAffixID = new Map()
  for (const row of affixDefinitions) {
    if (!row || typeof row !== "object" || row.active === false) continue
    const affixID = typeof row.id === "string" ? row.id.trim() : ""
    if (!affixID) continue
    knownAffixIDs.add(affixID)
    affixTypeByAffixID.set(affixID, row.type === "work" ? "work" : "battle")
    const raritySet = new Set(
      Array.isArray(row.allowedRarities)
        ? row.allowedRarities.map((entry) => String(entry || "").trim()).filter(Boolean)
        : []
    )
    allowedRarityByAffixID.set(affixID, raritySet)
  }
  const knownRarityIDs = new Set(
    rarityDefinitions
      .filter((row) => row && typeof row === "object")
      .map((row) => (typeof row.id === "string" ? row.id.trim() : ""))
      .filter(Boolean)
  )
  const knownEquipmentSecondaryClassIDs = new Set(
    (Array.isArray(equipmentSecondaryClasses) ? equipmentSecondaryClasses : [])
      .map((row) => (typeof row?.id === "string" ? row.id.trim() : ""))
      .filter(Boolean)
  )
  const knownWeaponClassIDs = new Set(
    (Array.isArray(weaponClasses) ? weaponClasses : [])
      .map((row) => (typeof row?.id === "string" ? row.id.trim() : ""))
      .filter(Boolean)
  )
  const knownSkillTypeIDs = new Set(
    (Array.isArray(skillTypes) ? skillTypes : [])
      .map((row) => (typeof row?.id === "string" ? row.id.trim() : ""))
      .filter(Boolean)
  )
  const knownBuffIDs = new Set(
    (Array.isArray(buffs) ? buffs : [])
      .map((row) => (typeof row?.id === "string" ? row.id.trim() : ""))
      .filter(Boolean)
  )
  const knownBuffKinds = new Set(
    (Array.isArray(buffs) ? buffs : [])
      .map((row) => (typeof row?.kind === "string" ? row.kind.trim().toLowerCase() : ""))
      .filter(Boolean)
  )
  const knownSkillIDs = new Set(
    (Array.isArray(skills) ? skills : [])
      .map((row) => (typeof row?.id === "string" ? row.id.trim() : ""))
      .filter(Boolean)
  )
  const knownEnemyIDs = new Set(
    (Array.isArray(enemies) ? enemies : [])
      .map((row) => (typeof row?.id === "string" ? row.id.trim() : ""))
      .filter(Boolean)
  )
  const knownEquipmentTemplateIDs = new Set(
    rows
      .map((row) => (typeof row?.id === "string" ? row.id.trim() : ""))
      .filter(Boolean)
  )
  const stableEffectIDs = new Set(
    (Array.isArray(skills) ? skills : [])
      .flatMap((row) => Array.isArray(row?.effects) ? row.effects : [])
      .flatMap((effect) => [effect?.id, effect?.effectId])
      .map((id) => String(id || "").trim())
      .filter(Boolean)
      .map((id) => id.toLowerCase())
  )
  const knownBattleHookFormulaAttrIDs = new Set([
    "equipment_enhance_level",
    ...(Array.isArray(attributes) ? attributes : [])
      .map((row) => (typeof row?.id === "string" ? row.id.trim() : ""))
      .filter(Boolean)
  ])
  const allowedBattleHookConditions = new Set([
    "selfhppercentlte",
    "selfhppercentgte",
    "sourcehppercentlte",
    "sourcehppercentgte",
    "targethppercentlte",
    "targethppercentgte",
    "selfmplt",
    "selfmpgte",
    "incomingdamagegte",
    "incomingdamagelte",
    "incominghealgte",
    "incomingheallte",
    "hasbuff",
    "sourcehasbuff",
    "targethasbuff",
    "selfweaponclass",
    "sourceweaponclass",
    "selfposition",
    "allysidehaspositions",
    "selfchargesremaininggt",
    "selfchargesremaininggte",
    "selfchargesremaininglt",
    "selfchargesremaininglte",
    "selfbuffcountgte",
    "selfbuffcountgt",
    "selfbuffcountlte",
    "selfbuffcountlt",
    "selfbuffcounteq",
    "targetbuffcountgte",
    "targetdistinctsourcebuffcountgte",
    "selfskillcooldowncountgte",
    "equipmentenhancelevelgte",
    "playerpartyequippedunitcounteq",
    "selfcompletedturncountmodeq",
    "waveintervaleq"
  ])
  const maxEnhanceLevelCap = 1000
  const validateEnhanceLevelKey = (templateID, key, maxLevel) => {
    if (!/^\d+$/.test(key)) {
      throw new Error(`equipment_templates enhance overrides key invalid: ${templateID}.${key}`)
    }
    const level = Number(key)
    if (!Number.isSafeInteger(level) || level <= 0) {
      throw new Error(`equipment_templates enhance overrides level invalid: ${templateID}.${key}`)
    }
    if (maxLevel > 0 && level > maxLevel) {
      throw new Error(`equipment_templates enhance overrides level exceeds max: ${templateID}.${key}`)
    }
  }
  const validateFiniteMap = (label, templateID, value) => {
    if (!value || typeof value !== "object" || Array.isArray(value)) {
      throw new Error(`${label} invalid: ${templateID}`)
    }
    for (const [k, v] of Object.entries(value)) {
      if (!k || typeof k !== "string") {
        throw new Error(`${label} key invalid: ${templateID}`)
      }
      if (typeof v !== "number" || !Number.isFinite(v)) {
        throw new Error(`${label} value invalid: ${templateID}.${k}`)
      }
    }
  }
  const validateIntegerMap = (label, templateID, value) => {
    validateFiniteMap(label, templateID, value)
    for (const [k, v] of Object.entries(value)) {
      if (!Number.isInteger(v) || v < 0) {
        throw new Error(`${label} must be non-negative integer: ${templateID}.${k}`)
      }
    }
  }
  const ids = new Set()
  for (const row of rows) {
    if (!row || typeof row !== "object") throw new Error("equipment_templates row must be object")
    if (typeof row.id !== "string" || row.id === "") throw new Error("equipment_templates.id must be non-empty string")
    if (ids.has(row.id)) throw new Error(`duplicate equipment_templates.id: ${row.id}`)
    ids.add(row.id)
    if (typeof row.slot !== "string" || row.slot.trim() === "") throw new Error(`equipment_templates.slot invalid: ${row.id}`)
    if (row.equipType !== "work" && row.equipType !== "battle") throw new Error(`equipment_templates.equipType invalid: ${row.id}`)
    if (!Array.isArray(row.occupySlots) || row.occupySlots.some((slot) => typeof slot !== "string" || slot.trim() === "")) {
      throw new Error(`equipment_templates.occupySlots invalid: ${row.id}`)
    }
    if (!Array.isArray(row.equipTargets) || row.equipTargets.some((target) => typeof target !== "string" || target.trim() === "")) {
      throw new Error(`equipment_templates.equipTargets invalid: ${row.id}`)
    }
    const seenEquipTargets = new Set()
    for (const rawTarget of row.equipTargets) {
      const target = rawTarget.trim().toLowerCase()
      if (target !== "player" && target !== "kitty") {
        throw new Error(`equipment_templates.equipTargets value invalid: ${row.id}.${rawTarget}`)
      }
      if (seenEquipTargets.has(target)) {
        throw new Error(`equipment_templates.equipTargets duplicate: ${row.id}.${target}`)
      }
      seenEquipTargets.add(target)
    }
    if (!Array.isArray(row.weaponClass) || row.weaponClass.some((weaponClass) => typeof weaponClass !== "string" || weaponClass.trim() === "")) {
      throw new Error(`equipment_templates.weaponClass invalid: ${row.id}`)
    }
    if (typeof row.equipmentSecondaryClass !== "string") {
      throw new Error(`equipment_templates.equipmentSecondaryClass invalid: ${row.id}`)
    }
    if (
      row.equipmentSecondaryClass &&
      !knownEquipmentSecondaryClassIDs.has(row.equipmentSecondaryClass)
    ) {
      throw new Error(`equipment_templates.equipmentSecondaryClass not found: ${row.id}.${row.equipmentSecondaryClass}`)
    }
    if (!row.weaponClass.length && !row.equipmentSecondaryClass) {
      throw new Error(`equipment_templates.equipmentSecondaryClass required for non-weapon template: ${row.id}`)
    }
    if (!row.statusRequirement || typeof row.statusRequirement !== "object" || Array.isArray(row.statusRequirement)) {
      throw new Error(`equipment_templates.statusRequirement invalid: ${row.id}`)
    }
    for (const [attrID, required] of Object.entries(row.statusRequirement)) {
      if (!attrID || typeof required !== "number" || !Number.isFinite(required) || required < 0) {
        throw new Error(`equipment_templates.statusRequirement value invalid: ${row.id}.${attrID}`)
      }
    }
    if (typeof row.accessoryGroup !== "string") throw new Error(`equipment_templates.accessoryGroup invalid: ${row.id}`)
    if (typeof row.affixSlotCount !== "number" || !Number.isFinite(row.affixSlotCount) || row.affixSlotCount < 0) {
      throw new Error(`equipment_templates.affixSlotCount invalid: ${row.id}`)
    }
    if (!Array.isArray(row.affixAllowedIds) || row.affixAllowedIds.some((item) => typeof item !== "string" || item.trim() === "")) {
      throw new Error(`equipment_templates.affixAllowedIds invalid: ${row.id}`)
    }
    if (!row.baseAttrs || typeof row.baseAttrs !== "object" || Array.isArray(row.baseAttrs)) {
      throw new Error(`equipment_templates.baseAttrs invalid: ${row.id}`)
    }
    for (const [attrID, value] of Object.entries(row.baseAttrs)) {
      if (!attrID || typeof value !== "number" || !Number.isFinite(value)) {
        throw new Error(`equipment_templates.baseAttrs value invalid: ${row.id}.${attrID}`)
      }
    }
    if (typeof row.maxDurability !== "number" || !Number.isFinite(row.maxDurability) || row.maxDurability <= 0) {
      throw new Error(`equipment_templates.maxDurability invalid: ${row.id}`)
    }
    if (typeof row.decayRate !== "number" || !Number.isFinite(row.decayRate) || row.decayRate < 0) {
      throw new Error(`equipment_templates.decayRate invalid: ${row.id}`)
    }
    if (typeof row.enhanceMaxLevel !== "number" || !Number.isFinite(row.enhanceMaxLevel) || !Number.isInteger(row.enhanceMaxLevel) || row.enhanceMaxLevel < 0 || row.enhanceMaxLevel > maxEnhanceLevelCap) {
      throw new Error(`equipment_templates.enhanceMaxLevel invalid: ${row.id}`)
    }
    if (typeof row.enhanceDurationS !== "number" || !Number.isFinite(row.enhanceDurationS) || !Number.isInteger(row.enhanceDurationS) || row.enhanceDurationS <= 0) {
      throw new Error(`equipment_templates.enhanceDurationS invalid: ${row.id}`)
    }
    if (
      row.enhanceSuccessRateOverride !== undefined &&
      (typeof row.enhanceSuccessRateOverride !== "number" ||
        !Number.isFinite(row.enhanceSuccessRateOverride) ||
        row.enhanceSuccessRateOverride < 0 ||
        row.enhanceSuccessRateOverride > 1)
    ) {
      throw new Error(`equipment_templates.enhanceSuccessRateOverride invalid: ${row.id}`)
    }
    if (
      row.basicAttackTargetDirection !== undefined &&
      row.basicAttackTargetDirection !== "vanguard" &&
      row.basicAttackTargetDirection !== "midguard" &&
      row.basicAttackTargetDirection !== "rearguard"
    ) {
      throw new Error(`equipment_templates.basicAttackTargetDirection invalid: ${row.id}`)
    }
    if (
      row.targetSeekStrategy !== undefined &&
      row.targetSeekStrategy !== "rear_to_front" &&
      row.targetSeekStrategy !== "front_to_rear" &&
      row.targetSeekStrategy !== "random_alive"
    ) {
      throw new Error(`equipment_templates.targetSeekStrategy invalid: ${row.id}`)
    }
    if (row.basicAttackFormula !== undefined) {
      if (!row.basicAttackFormula || typeof row.basicAttackFormula !== "object" || Array.isArray(row.basicAttackFormula)) {
        throw new Error(`equipment_templates.basicAttackFormula invalid: ${row.id}`)
      }
      if (row.basicAttackFormula.damageType !== "physical" && row.basicAttackFormula.damageType !== "magical") {
        throw new Error(`equipment_templates.basicAttackFormula.damageType invalid: ${row.id}`)
      }
      if (!Array.isArray(row.basicAttackFormula.formula) || row.basicAttackFormula.formula.length === 0) {
        throw new Error(`equipment_templates.basicAttackFormula.formula invalid: ${row.id}`)
      }
      for (const term of row.basicAttackFormula.formula) {
        if (!term || typeof term !== "object" || Array.isArray(term)) {
          throw new Error(`equipment_templates.basicAttackFormula.formula term invalid: ${row.id}`)
        }
        if (typeof term.attrKey !== "string" || term.attrKey.trim() === "") {
          throw new Error(`equipment_templates.basicAttackFormula.formula.attrKey invalid: ${row.id}`)
        }
        if (typeof term.coefficient !== "number" || !Number.isFinite(term.coefficient)) {
          throw new Error(`equipment_templates.basicAttackFormula.formula.coefficient invalid: ${row.id}.${term.attrKey}`)
        }
      }
      if (!row.basicAttackFormula.damageSpec || typeof row.basicAttackFormula.damageSpec !== "object" || Array.isArray(row.basicAttackFormula.damageSpec)) {
        throw new Error(`equipment_templates.basicAttackFormula.damageSpec invalid: ${row.id}`)
      }
      validateHitDamageScaling(row.basicAttackFormula.damageSpec, "equipment_templates.basicAttackFormula.damageSpec", row.id)
      rejectEnemyOnlyDamageReductionPierce(row.basicAttackFormula.damageSpec, `equipment_templates.basicAttackFormula.damageSpec: ${row.id}`)
      if (row.basicAttackFormula.damageSpec.damageType !== "physical" && row.basicAttackFormula.damageSpec.damageType !== "magical") {
        throw new Error(`equipment_templates.basicAttackFormula.damageSpec.damageType invalid: ${row.id}`)
      }
      const specFormulaFields = [
        ["amountFormula", row.basicAttackFormula.damageSpec.amountFormula],
        ["flatDefensePenetrationFormula", row.basicAttackFormula.damageSpec.flatDefensePenetrationFormula],
        ["percentDefensePenetrationFormula", row.basicAttackFormula.damageSpec.percentDefensePenetrationFormula],
        ["extraDamageAmplificationFormula", row.basicAttackFormula.damageSpec.extraDamageAmplificationFormula]
      ]
      for (const [label, formula] of specFormulaFields) {
        if (!formula || typeof formula !== "object" || Array.isArray(formula)) {
          throw new Error(`equipment_templates.basicAttackFormula.damageSpec.${label} invalid: ${row.id}`)
        }
        if (!Number.isFinite(formula.baseAdd)) {
          throw new Error(`equipment_templates.basicAttackFormula.damageSpec.${label}.baseAdd invalid: ${row.id}`)
        }
        if (!Array.isArray(formula.terms)) {
          throw new Error(`equipment_templates.basicAttackFormula.damageSpec.${label}.terms invalid: ${row.id}`)
        }
        for (const term of formula.terms) {
          if (!term || typeof term !== "object" || Array.isArray(term)) throw new Error(`equipment_templates.basicAttackFormula.damageSpec.${label} term invalid: ${row.id}`)
          if (typeof term.attrKey !== "string" || term.attrKey.trim() === "") throw new Error(`equipment_templates.basicAttackFormula.damageSpec.${label}.attrKey invalid: ${row.id}`)
          if (!Number.isFinite(term.coefficient)) throw new Error(`equipment_templates.basicAttackFormula.damageSpec.${label}.coefficient invalid: ${row.id}`)
          if (!Number.isFinite(term.perSkillLevel)) throw new Error(`equipment_templates.basicAttackFormula.damageSpec.${label}.perSkillLevel invalid: ${row.id}`)
        }
      }
      if (row.basicAttackFormula.damageSpec.bonusAgainstTargetTypes !== undefined) {
        if (!row.basicAttackFormula.damageSpec.bonusAgainstTargetTypes || typeof row.basicAttackFormula.damageSpec.bonusAgainstTargetTypes !== "object" || Array.isArray(row.basicAttackFormula.damageSpec.bonusAgainstTargetTypes)) {
          throw new Error(`equipment_templates.basicAttackFormula.damageSpec.bonusAgainstTargetTypes invalid: ${row.id}`)
        }
        for (const [targetType, bonus] of Object.entries(row.basicAttackFormula.damageSpec.bonusAgainstTargetTypes)) {
          if (!targetType || typeof targetType !== "string" || !Number.isFinite(bonus)) {
            throw new Error(`equipment_templates.basicAttackFormula.damageSpec.bonusAgainstTargetTypes entry invalid: ${row.id}.${targetType}`)
          }
        }
      }
      if (!row.basicAttackFormula.damageSpec.hitCount || typeof row.basicAttackFormula.damageSpec.hitCount !== "object" || Array.isArray(row.basicAttackFormula.damageSpec.hitCount)) {
        throw new Error(`equipment_templates.basicAttackFormula.damageSpec.hitCount invalid: ${row.id}`)
      }
      if (!Number.isFinite(row.basicAttackFormula.damageSpec.hitCount.base) || !Number.isInteger(row.basicAttackFormula.damageSpec.hitCount.base) || row.basicAttackFormula.damageSpec.hitCount.base <= 0) {
        throw new Error(`equipment_templates.basicAttackFormula.damageSpec.hitCount.base invalid: ${row.id}`)
      }
      if (!Array.isArray(row.basicAttackFormula.damageSpec.hitCount.formula)) {
        throw new Error(`equipment_templates.basicAttackFormula.damageSpec.hitCount.formula invalid: ${row.id}`)
      }
      for (const term of row.basicAttackFormula.damageSpec.hitCount.formula) {
        if (!term || typeof term !== "object" || Array.isArray(term)) throw new Error(`equipment_templates.basicAttackFormula.damageSpec.hitCount.formula term invalid: ${row.id}`)
        if (typeof term.attrKey !== "string" || term.attrKey.trim() === "") throw new Error(`equipment_templates.basicAttackFormula.damageSpec.hitCount.formula.attrKey invalid: ${row.id}`)
        if (!Number.isFinite(term.coefficient)) throw new Error(`equipment_templates.basicAttackFormula.damageSpec.hitCount.formula.coefficient invalid: ${row.id}`)
        if (!Number.isFinite(term.perSkillLevel)) throw new Error(`equipment_templates.basicAttackFormula.damageSpec.hitCount.formula.perSkillLevel invalid: ${row.id}`)
      }
    }
    if (row.basicAttackEffects !== undefined) {
      if (!Array.isArray(row.basicAttackEffects) || row.basicAttackEffects.length === 0) {
        throw new Error(`equipment_templates.basicAttackEffects invalid: ${row.id}`)
      }
      for (let effectIndex = 0; effectIndex < row.basicAttackEffects.length; effectIndex++) {
        const effect = row.basicAttackEffects[effectIndex]
        if (!effect || typeof effect !== "object" || Array.isArray(effect)) throw new Error(`equipment_templates.basicAttackEffects entry invalid: ${row.id}`)
        const canonicalEffectID = typeof effect.id === "string" ? effect.id.trim() : ""
        const legacyEffectID = typeof effect.effectId === "string" ? effect.effectId.trim() : ""
        if (effect.id !== undefined && !canonicalEffectID) throw new Error(`equipment_templates.basicAttackEffects.id invalid: ${row.id}.${effectIndex + 1}`)
        if (effect.effectId !== undefined && !legacyEffectID) throw new Error(`equipment_templates.basicAttackEffects.effectId invalid: ${row.id}.${effectIndex + 1}`)
        if (canonicalEffectID && legacyEffectID && canonicalEffectID !== legacyEffectID) throw new Error(`equipment_templates.basicAttackEffects stable id mismatch: ${row.id}.${effectIndex + 1}`)
        const stableEffectID = canonicalEffectID || legacyEffectID
        if (!stableEffectID) throw new Error(`equipment_templates.basicAttackEffects.id required: ${row.id}.${effectIndex + 1}`)
        if (!STABLE_BATTLE_EFFECT_ID_PATTERN.test(stableEffectID)) throw new Error(`equipment_templates.basicAttackEffects.id invalid: ${row.id}.${stableEffectID}`)
        const stableEffectIdentity = stableEffectID.toLowerCase()
        if (stableEffectIDs.has(stableEffectIdentity)) throw new Error(`battle effect stable id duplicate: ${stableEffectID}`)
        stableEffectIDs.add(stableEffectIdentity)
        if (effect.kind !== "damage" && effect.kind !== "heal" && effect.kind !== "status") throw new Error(`equipment_templates.basicAttackEffects.kind invalid: ${row.id}`)
        if (!effect.targeting || typeof effect.targeting !== "object" || Array.isArray(effect.targeting)) throw new Error(`equipment_templates.basicAttackEffects.targeting invalid: ${row.id}`)
        rejectEnemyOnlyTargetPriority(effect.targeting, `equipment_templates.basicAttackEffects.targeting: ${row.id}`)
        validateEffectLinkage(effect, effectIndex + 1, row.id, "equipment_templates.basicAttackEffects")
        if (effect.ignoreUntargetable !== undefined && typeof effect.ignoreUntargetable !== "boolean") throw new Error(`equipment_templates.basicAttackEffects.ignoreUntargetable invalid: ${row.id}`)
        if (!Array.isArray(effect.targeting.targetPositions) || effect.targeting.targetPositions.length === 0) throw new Error(`equipment_templates.basicAttackEffects.targeting.targetPositions invalid: ${row.id}`)
        for (const value of effect.targeting.targetPositions) {
          if (value !== "vanguard" && value !== "midguard" && value !== "rearguard") {
            throw new Error(`equipment_templates.basicAttackEffects.targeting.targetPositions value invalid: ${row.id}`)
          }
        }
        if (effect.targeting.secondarySeek !== "rear_to_front" && effect.targeting.secondarySeek !== "front_to_rear" && effect.targeting.secondarySeek !== "random_alive" && effect.targeting.secondarySeek !== "none") {
          throw new Error(`equipment_templates.basicAttackEffects.targeting.secondarySeek invalid: ${row.id}`)
        }
        if (effect.targeting.targetScope !== "ally" && effect.targeting.targetScope !== "enemy" && effect.targeting.targetScope !== "all" && effect.targeting.targetScope !== "self") {
          throw new Error(`equipment_templates.basicAttackEffects.targeting.targetScope invalid: ${row.id}`)
        }
        if (
          effect.targeting.selectRule !== "hp_max" &&
          effect.targeting.selectRule !== "hp_min" &&
          effect.targeting.selectRule !== "hp_percent_max" &&
          effect.targeting.selectRule !== "hp_percent_min" &&
          effect.targeting.selectRule !== "first" &&
          effect.targeting.selectRule !== "boss_first" &&
          effect.targeting.selectRule !== "attr_max" &&
          effect.targeting.selectRule !== "attr_min" &&
          effect.targeting.selectRule !== "random" &&
          effect.targeting.selectRule !== "self"
        ) {
          throw new Error(`equipment_templates.basicAttackEffects.targeting.selectRule invalid: ${row.id}`)
        }
        if ((effect.targeting.selectRule === "attr_max" || effect.targeting.selectRule === "attr_min") && (!effect.targeting.selectAttrKey || typeof effect.targeting.selectAttrKey !== "string")) {
          throw new Error(`equipment_templates.basicAttackEffects.targeting.selectAttrKey invalid: ${row.id}`)
        }
        if (typeof effect.targeting.fallbackSelfIfEmpty !== "boolean") {
          throw new Error(`equipment_templates.basicAttackEffects.targeting.fallbackSelfIfEmpty invalid: ${row.id}`)
        }
        if (typeof effect.targeting.excludeSelf !== "boolean") {
          throw new Error(`equipment_templates.basicAttackEffects.targeting.excludeSelf invalid: ${row.id}`)
        }
        if (!effect.targeting.targetCount || typeof effect.targeting.targetCount !== "object" || Array.isArray(effect.targeting.targetCount)) {
          throw new Error(`equipment_templates.basicAttackEffects.targeting.targetCount invalid: ${row.id}`)
        }
        if (!Number.isFinite(effect.targeting.targetCount.base) || effect.targeting.targetCount.base <= 0 || !Number.isInteger(effect.targeting.targetCount.base)) {
          throw new Error(`equipment_templates.basicAttackEffects.targeting.targetCount.base invalid: ${row.id}`)
        }
        if (!Array.isArray(effect.targeting.targetCount.formula)) {
          throw new Error(`equipment_templates.basicAttackEffects.targeting.targetCount.formula invalid: ${row.id}`)
        }
        if (effect.targeting.targetCount.max !== undefined) {
          if (!Number.isFinite(effect.targeting.targetCount.max) || effect.targeting.targetCount.max <= 0 || !Number.isInteger(effect.targeting.targetCount.max)) {
            throw new Error(`equipment_templates.basicAttackEffects.targeting.targetCount.max invalid: ${row.id}`)
          }
        }
        if (effect.kind === "damage" || effect.kind === "heal") {
          if (effect.damageType !== "physical" && effect.damageType !== "magical") {
            throw new Error(`equipment_templates.basicAttackEffects.damageType invalid: ${row.id}`)
          }
          if (!effect.amountFormula || typeof effect.amountFormula !== "object" || Array.isArray(effect.amountFormula)) {
            throw new Error(`equipment_templates.basicAttackEffects.amountFormula invalid: ${row.id}`)
          }
          if (!Number.isFinite(effect.amountFormula.baseAdd)) {
            throw new Error(`equipment_templates.basicAttackEffects.amountFormula.baseAdd invalid: ${row.id}`)
          }
          if (!Array.isArray(effect.amountFormula.terms)) {
            throw new Error(`equipment_templates.basicAttackEffects.amountFormula.terms invalid: ${row.id}`)
          }
          for (const term of effect.amountFormula.terms) {
            if (!term || typeof term !== "object" || Array.isArray(term)) throw new Error(`equipment_templates.basicAttackEffects.amountFormula term invalid: ${row.id}`)
            if (typeof term.attrKey !== "string" || term.attrKey.trim() === "") throw new Error(`equipment_templates.basicAttackEffects.amountFormula.attrKey invalid: ${row.id}`)
            if (!Number.isFinite(term.coefficient)) throw new Error(`equipment_templates.basicAttackEffects.amountFormula.coefficient invalid: ${row.id}`)
            if (!Number.isFinite(term.perSkillLevel)) throw new Error(`equipment_templates.basicAttackEffects.amountFormula.perSkillLevel invalid: ${row.id}`)
          }
          if (effect.kind === "damage") {
            if (!effect.damageSpec || typeof effect.damageSpec !== "object" || Array.isArray(effect.damageSpec)) {
              throw new Error(`equipment_templates.basicAttackEffects.damageSpec invalid: ${row.id}`)
            }
            validateHitDamageScaling(effect.damageSpec, "equipment_templates.basicAttackEffects.damageSpec", row.id)
            rejectEnemyOnlyDamageReductionPierce(effect.damageSpec, `equipment_templates.basicAttackEffects.damageSpec: ${row.id}`)
            if (effect.damageSpec.damageType !== "physical" && effect.damageSpec.damageType !== "magical") {
              throw new Error(`equipment_templates.basicAttackEffects.damageSpec.damageType invalid: ${row.id}`)
            }
            const formulaFields = [
              ["amountFormula", effect.damageSpec.amountFormula],
              ["flatDefensePenetrationFormula", effect.damageSpec.flatDefensePenetrationFormula],
              ["percentDefensePenetrationFormula", effect.damageSpec.percentDefensePenetrationFormula],
              ["extraDamageAmplificationFormula", effect.damageSpec.extraDamageAmplificationFormula]
            ]
            for (const [label, formula] of formulaFields) {
              if (!formula || typeof formula !== "object" || Array.isArray(formula)) {
                throw new Error(`equipment_templates.basicAttackEffects.damageSpec.${label} invalid: ${row.id}`)
              }
              if (!Number.isFinite(formula.baseAdd)) {
                throw new Error(`equipment_templates.basicAttackEffects.damageSpec.${label}.baseAdd invalid: ${row.id}`)
              }
              if (!Array.isArray(formula.terms)) {
                throw new Error(`equipment_templates.basicAttackEffects.damageSpec.${label}.terms invalid: ${row.id}`)
              }
              for (const term of formula.terms) {
                if (!term || typeof term !== "object" || Array.isArray(term)) throw new Error(`equipment_templates.basicAttackEffects.damageSpec.${label} term invalid: ${row.id}`)
                if (typeof term.attrKey !== "string" || term.attrKey.trim() === "") throw new Error(`equipment_templates.basicAttackEffects.damageSpec.${label}.attrKey invalid: ${row.id}`)
                if (!Number.isFinite(term.coefficient)) throw new Error(`equipment_templates.basicAttackEffects.damageSpec.${label}.coefficient invalid: ${row.id}`)
                if (!Number.isFinite(term.perSkillLevel)) throw new Error(`equipment_templates.basicAttackEffects.damageSpec.${label}.perSkillLevel invalid: ${row.id}`)
              }
            }
            if (effect.damageSpec.bonusAgainstTargetTypes !== undefined) {
              if (!effect.damageSpec.bonusAgainstTargetTypes || typeof effect.damageSpec.bonusAgainstTargetTypes !== "object" || Array.isArray(effect.damageSpec.bonusAgainstTargetTypes)) {
                throw new Error(`equipment_templates.basicAttackEffects.damageSpec.bonusAgainstTargetTypes invalid: ${row.id}`)
              }
              for (const [targetType, bonus] of Object.entries(effect.damageSpec.bonusAgainstTargetTypes)) {
                if (!targetType || typeof targetType !== "string" || !Number.isFinite(bonus)) {
                  throw new Error(`equipment_templates.basicAttackEffects.damageSpec.bonusAgainstTargetTypes entry invalid: ${row.id}.${targetType}`)
                }
              }
            }
            if (!effect.damageSpec.hitCount || typeof effect.damageSpec.hitCount !== "object" || Array.isArray(effect.damageSpec.hitCount)) {
              throw new Error(`equipment_templates.basicAttackEffects.damageSpec.hitCount invalid: ${row.id}`)
            }
            if (!Number.isFinite(effect.damageSpec.hitCount.base) || !Number.isInteger(effect.damageSpec.hitCount.base) || effect.damageSpec.hitCount.base <= 0) {
              throw new Error(`equipment_templates.basicAttackEffects.damageSpec.hitCount.base invalid: ${row.id}`)
            }
            if (!Array.isArray(effect.damageSpec.hitCount.formula)) {
              throw new Error(`equipment_templates.basicAttackEffects.damageSpec.hitCount.formula invalid: ${row.id}`)
            }
            for (const term of effect.damageSpec.hitCount.formula) {
              if (!term || typeof term !== "object" || Array.isArray(term)) throw new Error(`equipment_templates.basicAttackEffects.damageSpec.hitCount.formula term invalid: ${row.id}`)
              if (typeof term.attrKey !== "string" || term.attrKey.trim() === "") throw new Error(`equipment_templates.basicAttackEffects.damageSpec.hitCount.formula.attrKey invalid: ${row.id}`)
              if (!Number.isFinite(term.coefficient)) throw new Error(`equipment_templates.basicAttackEffects.damageSpec.hitCount.formula.coefficient invalid: ${row.id}`)
              if (!Number.isFinite(term.perSkillLevel)) throw new Error(`equipment_templates.basicAttackEffects.damageSpec.hitCount.formula.perSkillLevel invalid: ${row.id}`)
            }
          }
        }
        if (effect.kind === "status") {
          if (!effect.statusPayload || typeof effect.statusPayload !== "object" || Array.isArray(effect.statusPayload)) {
            throw new Error(`equipment_templates.basicAttackEffects.statusPayload invalid: ${row.id}`)
          }
          if (!Array.isArray(effect.statusPayload.statusIds) || effect.statusPayload.statusIds.some((value) => typeof value !== "string" || value.trim() === "")) {
            throw new Error(`equipment_templates.basicAttackEffects.statusPayload.statusIds invalid: ${row.id}`)
          }
          if (!Number.isFinite(effect.statusPayload.durationBaseTurns) || effect.statusPayload.durationBaseTurns < 0 || !Number.isInteger(effect.statusPayload.durationBaseTurns)) {
            throw new Error(`equipment_templates.basicAttackEffects.statusPayload.durationBaseTurns invalid: ${row.id}`)
          }
          if (!Number.isFinite(effect.statusPayload.durationPerSkillLevelTurns)) {
            throw new Error(`equipment_templates.basicAttackEffects.statusPayload.durationPerSkillLevelTurns invalid: ${row.id}`)
          }
          if (effect.statusPayload.applyChanceBase !== undefined) {
            if (!Number.isFinite(effect.statusPayload.applyChanceBase) || !Number.isFinite(effect.statusPayload.applyChancePerSkillLevel)) {
              throw new Error(`equipment_templates.basicAttackEffects.statusPayload.applyChance invalid: ${row.id}`)
            }
            if (!Number.isFinite(effect.statusPayload.applyChanceMin) || !Number.isFinite(effect.statusPayload.applyChanceMax) || effect.statusPayload.applyChanceMin < 0 || effect.statusPayload.applyChanceMax > 1 || effect.statusPayload.applyChanceMin > effect.statusPayload.applyChanceMax) {
              throw new Error(`equipment_templates.basicAttackEffects.statusPayload.applyChance bounds invalid: ${row.id}`)
            }
          }
        }
      }
    }
    if (row.battleHooks !== undefined) {
      if (!row.battleHooks) {
        row.battleHooks = []
      }
      if (!Array.isArray(row.battleHooks)) {
        throw new Error(`equipment_templates.battleHooks invalid: ${row.id}`)
      }
      for (const hook of row.battleHooks) {
        if (!hook || typeof hook !== "object" || Array.isArray(hook)) {
          throw new Error(`equipment_templates.battleHooks entry invalid: ${row.id}`)
        }
        if (hook.id !== undefined && (typeof hook.id !== "string" || hook.id.trim() === "")) {
          throw new Error(`equipment_templates.battleHooks.id invalid: ${row.id}`)
        }
        const triggerKey = normalizeBuffDSLToken(hook.trigger)
        if (
          triggerKey !== "onaftertakedamage" &&
          triggerKey !== "onafterdealdamage" &&
          triggerKey !== "onafterapplybuff" &&
          triggerKey !== "onafterdealcrit" &&
          triggerKey !== "onafterdealnocrit" &&
          triggerKey !== "onbeforedealdamage" &&
          triggerKey !== "onbeforetakedamage" &&
          triggerKey !== "onbattlestart" &&
          triggerKey !== "onbattleend" &&
          triggerKey !== "onturnstart" &&
          triggerKey !== "onturnend" &&
          triggerKey !== "onallyaftertakedamage" &&
          triggerKey !== "onotherunitturnend" &&
          triggerKey !== "onwavestart" &&
          triggerKey !== "onenemyrefresh" &&
          triggerKey !== "onafterdealheal" &&
          triggerKey !== "onafterbasicattackdealdamage" &&
          triggerKey !== "onaftersummon" &&
          triggerKey !== "onotherunitcastskill" &&
          triggerKey !== "onafterskillcooldowntick" &&
          triggerKey !== "onunitdeath" &&
          triggerKey !== "onkill" &&
          triggerKey !== "onbeforeprobabilitycheck" &&
          triggerKey !== "onprobabilitysuccess" &&
          triggerKey !== "onprobabilityfailure"
        ) {
          throw new Error(`equipment_templates.battleHooks.trigger invalid: ${row.id}.${hook.trigger}`)
        }
        validateBattleHookSubjectFilter(hook.subjectFilter, row.id, "equipment_templates.battleHooks.subjectFilter")
        for (const templateID of [
          ...(hook.subjectFilter?.includeSummonTemplateIds || []),
          ...(hook.subjectFilter?.excludeSummonTemplateIds || [])
        ]) {
          if (!knownEnemyIDs.has(templateID)) {
            throw new Error(`equipment_templates.battleHooks.subjectFilter summon template not found: ${row.id}.${templateID}`)
          }
        }
        if (hook.eventFilter !== undefined) {
          if (!hook.eventFilter || typeof hook.eventFilter !== "object" || Array.isArray(hook.eventFilter)) {
            throw new Error(`equipment_templates.battleHooks.eventFilter invalid: ${row.id}`)
          }
          if (hook.eventFilter.excludeSelf !== undefined && typeof hook.eventFilter.excludeSelf !== "boolean") {
            throw new Error(`equipment_templates.battleHooks.eventFilter.excludeSelf invalid: ${row.id}`)
          }
          if (hook.eventFilter.buffIds !== undefined) {
            if (!Array.isArray(hook.eventFilter.buffIds) || hook.eventFilter.buffIds.length === 0) {
              throw new Error(`equipment_templates.battleHooks.eventFilter.buffIds invalid: ${row.id}`)
            }
            for (const buffID of hook.eventFilter.buffIds) {
              if (typeof buffID !== "string" || !knownBuffIDs.has(buffID)) {
                throw new Error(`equipment_templates.battleHooks.eventFilter.buffIds not found: ${row.id}.${buffID}`)
              }
            }
          }
          if (hook.eventFilter.excludeBuffIds !== undefined) {
            if (!Array.isArray(hook.eventFilter.excludeBuffIds) || hook.eventFilter.excludeBuffIds.length === 0) {
              throw new Error(`equipment_templates.battleHooks.eventFilter.excludeBuffIds invalid: ${row.id}`)
            }
            for (const buffID of hook.eventFilter.excludeBuffIds) {
              if (typeof buffID !== "string" || !knownBuffIDs.has(buffID)) {
                throw new Error(`equipment_templates.battleHooks.eventFilter.excludeBuffIds not found: ${row.id}.${buffID}`)
              }
            }
          }
          if (hook.eventFilter.buffKinds !== undefined) {
            if (!Array.isArray(hook.eventFilter.buffKinds) || hook.eventFilter.buffKinds.length === 0) {
              throw new Error(`equipment_templates.battleHooks.eventFilter.buffKinds invalid: ${row.id}`)
            }
            for (const buffKind of hook.eventFilter.buffKinds) {
              if (typeof buffKind !== "string" || !knownBuffKinds.has(buffKind.toLowerCase())) {
                throw new Error(`equipment_templates.battleHooks.eventFilter.buffKinds not found: ${row.id}.${buffKind}`)
              }
            }
          }
          if (hook.eventFilter.firstBuffInstanceOnly !== undefined && typeof hook.eventFilter.firstBuffInstanceOnly !== "boolean") {
            throw new Error(`equipment_templates.battleHooks.eventFilter.firstBuffInstanceOnly invalid: ${row.id}`)
          }
          if (hook.eventFilter.applicationKinds !== undefined) {
            if (!Array.isArray(hook.eventFilter.applicationKinds) || hook.eventFilter.applicationKinds.length === 0) {
              throw new Error(`equipment_templates.battleHooks.eventFilter.applicationKinds invalid: ${row.id}`)
            }
            for (const applicationKind of hook.eventFilter.applicationKinds) {
              if (applicationKind !== "attach" && applicationKind !== "refresh") {
                throw new Error(`equipment_templates.battleHooks.eventFilter.applicationKinds invalid: ${row.id}.${applicationKind}`)
              }
            }
          }
          if (hook.eventFilter.turnKinds !== undefined) {
            if (!Array.isArray(hook.eventFilter.turnKinds) || hook.eventFilter.turnKinds.length === 0) {
              throw new Error(`equipment_templates.battleHooks.eventFilter.turnKinds invalid: ${row.id}`)
            }
            if (triggerKey !== "onturnstart" && triggerKey !== "onturnend") {
              throw new Error(`equipment_templates.battleHooks.eventFilter.turnKinds requires turn trigger: ${row.id}`)
            }
            for (const turnKind of hook.eventFilter.turnKinds) {
              if (turnKind !== "ordered" && turnKind !== "extra") {
                throw new Error(`equipment_templates.battleHooks.eventFilter.turnKinds invalid: ${row.id}.${turnKind}`)
              }
            }
          }
          if (hook.eventFilter.includePreparation !== undefined) {
            if (typeof hook.eventFilter.includePreparation !== "boolean" || (triggerKey !== "onturnstart" && triggerKey !== "onturnend")) {
              throw new Error(`equipment_templates.battleHooks.eventFilter.includePreparation invalid: ${row.id}`)
            }
          }
          if (hook.eventFilter.includeSuppressedTurns !== undefined) {
            if (typeof hook.eventFilter.includeSuppressedTurns !== "boolean" || triggerKey !== "onturnend") {
              throw new Error(`equipment_templates.battleHooks.eventFilter.includeSuppressedTurns invalid: ${row.id}`)
            }
          }
          if (hook.eventFilter.probabilityKinds !== undefined) {
            if (!Array.isArray(hook.eventFilter.probabilityKinds) || hook.eventFilter.probabilityKinds.length === 0 || hook.eventFilter.probabilityKinds.some((kind) => kind !== "equipment_proc" && kind !== "status_apply")) {
              throw new Error(`equipment_templates.battleHooks.eventFilter.probabilityKinds invalid: ${row.id}`)
            }
          }
          for (const key of ["sourceIds", "excludeSourceIds"]) {
            if (hook.eventFilter[key] !== undefined && (!Array.isArray(hook.eventFilter[key]) || hook.eventFilter[key].length === 0 || hook.eventFilter[key].some((id) => typeof id !== "string" || !id.trim()))) {
              throw new Error(`equipment_templates.battleHooks.eventFilter.${key} invalid: ${row.id}`)
            }
          }
        }
        for (const [field, value] of [["maxTriggersPerTurn", hook.maxTriggersPerTurn], ["maxTriggersPerWave", hook.maxTriggersPerWave], ["maxTriggersPerBattle", hook.maxTriggersPerBattle]]) {
          if (value !== undefined && (!Number.isInteger(value) || value <= 0)) throw new Error(`equipment_templates.battleHooks.${field} invalid: ${row.id}`)
        }
        if (hook.cooldownCompletedTurns !== undefined && (!Number.isInteger(hook.cooldownCompletedTurns) || hook.cooldownCompletedTurns <= 0 || hook.cooldownCompletedTurns > 99)) {
          throw new Error(`equipment_templates.battleHooks.cooldownCompletedTurns invalid: ${row.id}`)
        }
        if (hook.activationCost !== undefined) {
          if (!hook.activationCost || typeof hook.activationCost !== "object" || Array.isArray(hook.activationCost)) {
            throw new Error(`equipment_templates.battleHooks.activationCost invalid: ${row.id}`)
          }
          const allowedActivationCostKeys = new Set(["maxHpRatio", "maxMpRatio"])
          for (const key of Object.keys(hook.activationCost)) {
            if (!allowedActivationCostKeys.has(key)) throw new Error(`equipment_templates.battleHooks.activationCost.${key} unsupported: ${row.id}`)
          }
          const hpRatio = parseNumberStrict(hook.activationCost.maxHpRatio ?? 0)
          const mpRatio = parseNumberStrict(hook.activationCost.maxMpRatio ?? 0)
          if (!Number.isFinite(hpRatio) || hpRatio < 0 || hpRatio > 1 || !Number.isFinite(mpRatio) || mpRatio < 0 || mpRatio > 1 || (hpRatio === 0 && mpRatio === 0)) {
            throw new Error(`equipment_templates.battleHooks.activationCost ratio invalid: ${row.id}`)
          }
        }
        if (hook.sharedTriggerGroup !== undefined && (typeof hook.sharedTriggerGroup !== "string" || !/^[a-z0-9_]+$/.test(hook.sharedTriggerGroup))) {
          throw new Error(`equipment_templates.battleHooks.sharedTriggerGroup invalid: ${row.id}`)
        }
        if (!Array.isArray(hook.conditions)) {
          throw new Error(`equipment_templates.battleHooks.conditions invalid: ${row.id}`)
        }
        for (const condition of hook.conditions) {
          if (!condition || typeof condition !== "object" || Array.isArray(condition)) {
            throw new Error(`equipment_templates.battleHooks.conditions entry invalid: ${row.id}`)
          }
          const conditionKey = normalizeBuffDSLToken(condition.type || condition.kind)
          if (conditionKey && !allowedBattleHookConditions.has(conditionKey)) {
            throw new Error(`equipment_templates.battleHooks.conditions.kind invalid: ${row.id}.${condition.type || condition.kind}`)
          }
          if (conditionKey === "selfweaponclass" || conditionKey === "sourceweaponclass") {
            const weaponClassList = parseStringArray(condition.weaponClasses ?? condition.weaponClass ?? condition.weapon_classes ?? condition.weapon_class ?? condition.value) || []
            if (weaponClassList.length === 0) {
              throw new Error(`equipment_templates.battleHooks.conditions.weaponClasses invalid: ${row.id}`)
            }
            for (const weaponClass of weaponClassList) {
              if (!knownWeaponClassIDs.has(weaponClass)) {
                throw new Error(`equipment_templates.battleHooks.conditions.weaponClasses not found: ${row.id}.${weaponClass}`)
              }
            }
          }
          if (conditionKey === "selfposition" || conditionKey === "allysidehaspositions") {
            const positionList = parseStringArray(condition.positions ?? condition.position ?? condition.value) || []
            if (positionList.length === 0) {
              throw new Error(`equipment_templates.battleHooks.conditions.positions invalid: ${row.id}`)
            }
            for (const position of positionList) {
              if (position !== "vanguard" && position !== "midguard" && position !== "rearguard") {
                throw new Error(`equipment_templates.battleHooks.conditions.positions invalid: ${row.id}.${position}`)
              }
            }
          }
          if (conditionKey === "selfskillcooldowncountgte") {
            const value = parseNumberStrict(condition.value)
            if (!Number.isInteger(value) || value < 1) throw new Error(`equipment_templates.battleHooks.conditions.self_skill_cooldown_count_gte.value invalid: ${row.id}`)
            validateOptionalBoolean(condition.includeBasicAttack ?? condition.includeBasic, `equipment_templates.battleHooks.conditions.self_skill_cooldown_count_gte.includeBasicAttack invalid: ${row.id}`)
            validateOptionalStringArray(condition.skillCategories, `equipment_templates.battleHooks.conditions.self_skill_cooldown_count_gte.skillCategories invalid: ${row.id}`)
            for (const category of condition.skillCategories || []) {
              if (!knownSkillTypeIDs.has(category)) throw new Error(`equipment_templates.battleHooks.conditions.self_skill_cooldown_count_gte.skillCategories not found: ${row.id}.${category}`)
            }
          }
          if (conditionKey === "equipmentenhancelevelgte") {
            const value = parseNumberStrict(condition.value)
            if (!Number.isInteger(value) || value < 0 || value > row.enhanceMaxLevel) throw new Error(`equipment_templates.battleHooks.conditions.equipment_enhance_level_gte.value invalid: ${row.id}`)
          }
          if (conditionKey === "playerpartyequippedunitcounteq") {
            const value = parseNumberStrict(condition.value)
            if (typeof condition.itemId !== "string" || !knownEquipmentTemplateIDs.has(condition.itemId)) throw new Error(`equipment_templates.battleHooks.conditions.player_party_equipped_unit_count_eq.itemId not found: ${row.id}.${condition.itemId}`)
            if (!Number.isInteger(value) || value < 0) throw new Error(`equipment_templates.battleHooks.conditions.player_party_equipped_unit_count_eq.value invalid: ${row.id}`)
          }
          if (conditionKey === "targetbuffcountgte") {
            if (typeof condition.buffId !== "string" || !knownBuffIDs.has(condition.buffId)) throw new Error(`equipment_templates.battleHooks.conditions.target_buff_count_gte.buffId not found: ${row.id}.${condition.buffId}`)
            const value = parseNumberStrict(condition.value)
            if (!Number.isInteger(value) || value < 1) throw new Error(`equipment_templates.battleHooks.conditions.target_buff_count_gte.value invalid: ${row.id}`)
          }
          if (conditionKey === "targetdistinctsourcebuffcountgte") {
            const value = parseNumberStrict(condition.value)
            if (!Number.isInteger(value) || value < 1) throw new Error(`equipment_templates.battleHooks.conditions.target_distinct_source_buff_count_gte.value invalid: ${row.id}`)
            if (typeof condition.buffKind !== "string" || !knownBuffKinds.has(condition.buffKind.toLowerCase())) throw new Error(`equipment_templates.battleHooks.conditions.target_distinct_source_buff_count_gte.buffKind invalid: ${row.id}.${condition.buffKind}`)
          }
          if (conditionKey === "selfcompletedturncountmodeq") {
            const divisor = parseNumberStrict(condition.divisor)
            const remainder = parseNumberStrict(condition.remainder ?? 0)
            if (!Number.isInteger(divisor) || divisor <= 0 || !Number.isInteger(remainder) || remainder < 0 || remainder >= divisor) {
              throw new Error(`equipment_templates.battleHooks.conditions.self_completed_turn_count_mod_eq invalid: ${row.id}`)
            }
          }
        }
        if (!Array.isArray(hook.effects) || hook.effects.length === 0) {
          throw new Error(`equipment_templates.battleHooks.effects invalid: ${row.id}`)
        }
        if (typeof hook.priority !== "number" || !Number.isFinite(hook.priority)) {
          throw new Error(`equipment_templates.battleHooks.priority invalid: ${row.id}`)
        }
        for (const effect of hook.effects) {
          if (!effect || typeof effect !== "object" || Array.isArray(effect)) {
            throw new Error(`equipment_templates.battleHooks.effects entry invalid: ${row.id}`)
          }
          const effectKey = normalizeBuffDSLToken(effect.kind || effect.type)
          if (
            effectKey !== "addbuff" &&
            effectKey !== "removebuff" &&
            effectKey !== "removebuffbykind" &&
            effectKey !== "damage" &&
            effectKey !== "runtimedamageamp" &&
            effectKey !== "addruntimedamageamp" &&
            effectKey !== "adddamageamp" &&
            effectKey !== "basicattackmarkedtarget" &&
            effectKey !== "extraenemyrefresh" &&
            effectKey !== "summon" &&
            effectKey !== "heal" &&
            effectKey !== "consumemp" &&
            effectKey !== "extraturn" &&
            effectKey !== "mirrorcastskill" &&
            effectKey !== "adjustskillcooldown" &&
            effectKey !== "transferskillcooldown" &&
            effectKey !== "execute" &&
            effectKey !== "playerclone" &&
            effectKey !== "addprobability" &&
            effectKey !== "setprobability" &&
            effectKey !== "forceprobabilityfailure"
          ) {
            throw new Error(`equipment_templates.battleHooks.effects.kind invalid: ${row.id}.${effect.kind || effect.type}`)
          }
          const lifecycleTrigger = triggerKey === "onbeforeprobabilitycheck" || triggerKey === "onprobabilitysuccess" || triggerKey === "onprobabilityfailure"
          if (lifecycleTrigger && probabilityLifecycleProcChanceFields(effect).length > 0) {
            throw new Error(`equipment_templates.battleHooks probability lifecycle cannot contain procChance: ${row.id}.${hook.id || hook.trigger}`)
          }
          const chanceConfigured = effect.procChance !== undefined || effect.procChanceBase !== undefined || effect.procChanceFormula !== undefined || effect.procChanceMin !== undefined || effect.procChanceMax !== undefined
          if (chanceConfigured) {
            const base = parseNumberStrict(effect.procChanceBase ?? effect.procChance ?? 0)
            const min = effect.procChanceMin === undefined ? 0 : parseNumberStrict(effect.procChanceMin)
            const max = effect.procChanceMax === undefined ? 1 : parseNumberStrict(effect.procChanceMax)
            if (!Number.isFinite(base) || !Number.isFinite(min) || !Number.isFinite(max) || min < 0 || max > 1 || min > max) throw new Error(`equipment_templates.battleHooks.effects.procChance invalid: ${row.id}.${hook.id || hook.trigger}`)
          }
          if (effect.probabilityGroup !== undefined && (typeof effect.probabilityGroup !== "string" || !/^[a-z0-9_]+$/.test(effect.probabilityGroup))) throw new Error(`equipment_templates.battleHooks.effects.probabilityGroup invalid: ${row.id}`)
          if (effect.maxSuccessesPerTurn !== undefined && (!Number.isInteger(effect.maxSuccessesPerTurn) || effect.maxSuccessesPerTurn <= 0 || typeof effect.probabilityGroup !== "string" || !effect.probabilityGroup)) throw new Error(`equipment_templates.battleHooks.effects.maxSuccessesPerTurn invalid: ${row.id}`)
          if (effectKey === "addprobability" || effectKey === "setprobability") {
            if (triggerKey !== "onbeforeprobabilitycheck" || !Number.isFinite(effect.value)) throw new Error(`equipment_templates.battleHooks.effects.${effect.kind || effect.type}.value invalid: ${row.id}`)
            if (effectKey === "setprobability" && (effect.value < 0 || effect.value > 1)) throw new Error(`equipment_templates.battleHooks.effects.set_probability.value invalid: ${row.id}`)
          }
          if (effectKey === "forceprobabilityfailure" && triggerKey !== "onbeforeprobabilitycheck") throw new Error(`equipment_templates.battleHooks.effects.force_probability_failure trigger invalid: ${row.id}`)
          if (triggerKey === "onenemyrefresh" && effectKey !== "extraenemyrefresh") {
            throw new Error(`equipment_templates.battleHooks on_enemy_refresh only supports extra_enemy_refresh: ${row.id}`)
          }
          if (effectKey === "extraenemyrefresh") {
            validateEquipmentExtraEnemyRefreshEffect(effect, hook, row)
          }
          if ((effectKey === "addbuff" || effectKey === "removebuff") && (typeof effect.buffId !== "string" || effect.buffId.trim() === "")) {
            throw new Error(`equipment_templates.battleHooks.effects.buffId invalid: ${row.id}`)
          }
          if (effectKey === "removebuffbykind") {
            if (typeof effect.buffKind !== "string" || !knownBuffKinds.has(effect.buffKind.toLowerCase())) throw new Error(`equipment_templates.battleHooks.effects.remove_buff_by_kind.buffKind invalid: ${row.id}.${effect.buffKind}`)
            if (effect.selectionMode !== undefined && effect.selectionMode !== "max_remaining" && effect.selectionMode !== "id") throw new Error(`equipment_templates.battleHooks.effects.remove_buff_by_kind.selectionMode invalid: ${row.id}`)
            if (effect.maxBuffs !== undefined && (!Number.isInteger(effect.maxBuffs) || effect.maxBuffs < 1 || effect.maxBuffs > 16)) throw new Error(`equipment_templates.battleHooks.effects.remove_buff_by_kind.maxBuffs invalid: ${row.id}`)
            if (effect.dispellableOnly !== undefined && typeof effect.dispellableOnly !== "boolean") throw new Error(`equipment_templates.battleHooks.effects.remove_buff_by_kind.dispellableOnly invalid: ${row.id}`)
          }
          if (effectKey === "damage") {
            if (effect.damageType !== "physical" && effect.damageType !== "magical") {
              throw new Error(`equipment_templates.battleHooks.effects.damage.damageType invalid: ${row.id}`)
            }
            const validateDamageFormula = (formula, path) => {
              if (!formula || typeof formula !== "object" || Array.isArray(formula) || !Number.isFinite(formula.baseAdd) || !Array.isArray(formula.terms)) {
                throw new Error(`${path} invalid: ${row.id}`)
              }
              for (const term of formula.terms) {
                if (!term || typeof term !== "object" || Array.isArray(term)) {
                  throw new Error(`${path} term invalid: ${row.id}`)
                }
                const attrKey = typeof term.attrKey === "string" ? term.attrKey.trim() : ""
                if (!attrKey || !knownBattleHookFormulaAttrIDs.has(attrKey)) {
                  throw new Error(`${path}.attrKey invalid: ${row.id}.${attrKey}`)
                }
                if (!Number.isFinite(term.coefficient) || !Number.isFinite(term.perSkillLevel) || (term.perEnhanceLevel !== undefined && !Number.isFinite(term.perEnhanceLevel))) {
                  throw new Error(`${path} coefficient invalid: ${row.id}.${attrKey}`)
                }
              }
            }
            const relationFormulas = effect.amountFormulaByRelation
            if (relationFormulas !== undefined) {
              if (!relationFormulas || typeof relationFormulas !== "object" || Array.isArray(relationFormulas) || effect.relationMode !== "team_side") {
                throw new Error(`equipment_templates.battleHooks.effects.damage.amountFormulaByRelation invalid: ${row.id}`)
              }
              validateDamageFormula(relationFormulas.ally, "equipment_templates.battleHooks.effects.damage.amountFormulaByRelation.ally")
              validateDamageFormula(relationFormulas.enemy, "equipment_templates.battleHooks.effects.damage.amountFormulaByRelation.enemy")
            } else {
              validateDamageFormula(effect.amountFormula, "equipment_templates.battleHooks.effects.damage.amountFormula")
            }
            if (effect.relationMode !== undefined && effect.relationMode !== "team_side") {
              throw new Error(`equipment_templates.battleHooks.effects.damage.relationMode invalid: ${row.id}`)
            }
            if (effect.simultaneous !== undefined && typeof effect.simultaneous !== "boolean") {
              throw new Error(`equipment_templates.battleHooks.effects.damage.simultaneous invalid: ${row.id}`)
            }
            if (effect.dispatchEquipmentHooks !== undefined && typeof effect.dispatchEquipmentHooks !== "boolean") {
              throw new Error(`equipment_templates.battleHooks.effects.damage.dispatchEquipmentHooks invalid: ${row.id}`)
            }
          }
          if (effectKey === "mirrorcastskill") {
            validateOptionalStringArray(effect.skillIds, `equipment_templates.battleHooks.effects.skillIds invalid: ${row.id}`)
            validateOptionalStringArray(effect.skillCategories, `equipment_templates.battleHooks.effects.skillCategories invalid: ${row.id}`)
            validateOptionalBoolean(effect.includeBasicAttack, `equipment_templates.battleHooks.effects.includeBasicAttack invalid: ${row.id}`)
            validateOptionalBoolean(effect.allowMirroredCastTrigger, `equipment_templates.battleHooks.effects.allowMirroredCastTrigger invalid: ${row.id}`)
            validateOptionalBoolean(effect.applyCooldown, `equipment_templates.battleHooks.effects.applyCooldown invalid: ${row.id}`)
            for (const key of ["amountRatio", "amountRatioBase", "amountRatioPerEnhanceLevel", "mpCostMultiplier", "hpCostMultiplier"]) {
              if (effect[key] !== undefined && (!Number.isFinite(effect[key]) || effect[key] < 0)) {
                throw new Error(`equipment_templates.battleHooks.effects.mirror_cast_skill.${key} invalid: ${row.id}`)
              }
            }
          }
          if (effectKey === "adjustskillcooldown") {
            const amount = parseNumberStrict(effect.amount ?? effect.turns ?? effect.value)
            if (!Number.isInteger(amount) || amount === 0 || amount < -20 || amount > 20) throw new Error(`equipment_templates.battleHooks.effects.adjust_skill_cooldown.amount invalid: ${row.id}`)
            validateOptionalBoolean(effect.includeBasicAttack ?? effect.includeBasic, `equipment_templates.battleHooks.effects.adjust_skill_cooldown.includeBasicAttack invalid: ${row.id}`)
            validateOptionalStringArray(effect.skillIds, `equipment_templates.battleHooks.effects.adjust_skill_cooldown.skillIds invalid: ${row.id}`)
            validateOptionalStringArray(effect.excludeSkillIds, `equipment_templates.battleHooks.effects.adjust_skill_cooldown.excludeSkillIds invalid: ${row.id}`)
            validateOptionalStringArray(effect.skillCategories, `equipment_templates.battleHooks.effects.adjust_skill_cooldown.skillCategories invalid: ${row.id}`)
            for (const category of effect.skillCategories || []) {
              if (!knownSkillTypeIDs.has(category)) throw new Error(`equipment_templates.battleHooks.effects.adjust_skill_cooldown.skillCategories not found: ${row.id}.${category}`)
            }
            if (effect.selectionMode !== undefined && effect.selectionMode !== "all" && effect.selectionMode !== "max_remaining") throw new Error(`equipment_templates.battleHooks.effects.adjust_skill_cooldown.selectionMode invalid: ${row.id}`)
            if (effect.maxSkills !== undefined && (!Number.isInteger(effect.maxSkills) || effect.maxSkills < 0)) throw new Error(`equipment_templates.battleHooks.effects.adjust_skill_cooldown.maxSkills invalid: ${row.id}`)
          }
          if (effectKey === "transferskillcooldown") {
            const amount = parseNumberStrict(effect.amount ?? effect.turns ?? effect.value)
            if (!Number.isInteger(amount) || amount < 1 || amount > 20) throw new Error(`equipment_templates.battleHooks.effects.transfer_skill_cooldown.amount invalid: ${row.id}`)
            if (effect.targetMode !== undefined && effect.targetMode !== "self") throw new Error(`equipment_templates.battleHooks.effects.transfer_skill_cooldown.targetMode invalid: ${row.id}`)
            if (effect.requireBoth !== true) throw new Error(`equipment_templates.battleHooks.effects.transfer_skill_cooldown.requireBoth must be true: ${row.id}`)
            if (!effect.from || typeof effect.from !== "object" || Array.isArray(effect.from)) throw new Error(`equipment_templates.battleHooks.effects.transfer_skill_cooldown.from invalid: ${row.id}`)
            if (!effect.to || typeof effect.to !== "object" || Array.isArray(effect.to)) throw new Error(`equipment_templates.battleHooks.effects.transfer_skill_cooldown.to invalid: ${row.id}`)
            validateOptionalBoolean(effect.from.includeBasicAttack ?? effect.from.includeBasic, `equipment_templates.battleHooks.effects.transfer_skill_cooldown.from.includeBasic invalid: ${row.id}`)
            validateOptionalBoolean(effect.to.includeBasicAttack ?? effect.to.includeBasic, `equipment_templates.battleHooks.effects.transfer_skill_cooldown.to.includeBasic invalid: ${row.id}`)
            validateOptionalStringArray(effect.from.skillIds, `equipment_templates.battleHooks.effects.transfer_skill_cooldown.from.skillIds invalid: ${row.id}`)
            validateOptionalStringArray(effect.from.excludeSkillIds, `equipment_templates.battleHooks.effects.transfer_skill_cooldown.from.excludeSkillIds invalid: ${row.id}`)
            validateOptionalStringArray(effect.to.skillIds, `equipment_templates.battleHooks.effects.transfer_skill_cooldown.to.skillIds invalid: ${row.id}`)
            validateOptionalStringArray(effect.to.excludeSkillIds, `equipment_templates.battleHooks.effects.transfer_skill_cooldown.to.excludeSkillIds invalid: ${row.id}`)
            validateOptionalStringArray(effect.from.skillCategories, `equipment_templates.battleHooks.effects.transfer_skill_cooldown.from.skillCategories invalid: ${row.id}`)
            validateOptionalStringArray(effect.to.skillCategories, `equipment_templates.battleHooks.effects.transfer_skill_cooldown.to.skillCategories invalid: ${row.id}`)
            for (const skillID of [
              ...(effect.from.skillIds || []),
              ...(effect.from.excludeSkillIds || []),
              ...(effect.to.skillIds || []),
              ...(effect.to.excludeSkillIds || [])
            ]) {
              if (!knownSkillIDs.has(skillID)) throw new Error(`equipment_templates.battleHooks.effects.transfer_skill_cooldown.skill not found: ${row.id}.${skillID}`)
            }
            for (const category of [...(effect.from.skillCategories || []), ...(effect.to.skillCategories || [])]) {
              if (!knownSkillTypeIDs.has(category)) throw new Error(`equipment_templates.battleHooks.effects.transfer_skill_cooldown.skillCategories not found: ${row.id}.${category}`)
            }
            if (effect.from.excludeContextSkill !== true) throw new Error(`equipment_templates.battleHooks.effects.transfer_skill_cooldown.from.excludeContextSkill must be true: ${row.id}`)
            if (effect.from.requireCooling !== true) throw new Error(`equipment_templates.battleHooks.effects.transfer_skill_cooldown.from.requireCooling must be true: ${row.id}`)
            if (effect.from.selectionMode !== "max_remaining") throw new Error(`equipment_templates.battleHooks.effects.transfer_skill_cooldown.from.selectionMode invalid: ${row.id}`)
            if (effect.from.maxSkills !== 1) throw new Error(`equipment_templates.battleHooks.effects.transfer_skill_cooldown.from.maxSkills must be 1: ${row.id}`)
            if (effect.to.selectionMode !== "context_skill") throw new Error(`equipment_templates.battleHooks.effects.transfer_skill_cooldown.to.selectionMode invalid: ${row.id}`)
            if (effect.waiveIncreaseBuffId !== undefined) {
              if (typeof effect.waiveIncreaseBuffId !== "string" || !knownBuffIDs.has(effect.waiveIncreaseBuffId)) throw new Error(`equipment_templates.battleHooks.effects.transfer_skill_cooldown.waiveIncreaseBuffId not found: ${row.id}.${effect.waiveIncreaseBuffId}`)
              if (effect.consumeWaiverBuff !== true) throw new Error(`equipment_templates.battleHooks.effects.transfer_skill_cooldown.consumeWaiverBuff must be true: ${row.id}`)
            }
          }
          if (effectKey === "extraturn") {
            validateBattleHookExtraTurnEffect(effect, row.id, "equipment_templates.battleHooks.effects")
          }
          if (
            (effectKey === "runtimedamageamp" || effectKey === "addruntimedamageamp" || effectKey === "adddamageamp") &&
            (typeof effect.value !== "number" || !Number.isFinite(effect.value))
          ) {
            throw new Error(`equipment_templates.battleHooks.effects.value invalid: ${row.id}.${effect.kind || effect.type}`)
          }
          if ((effectKey === "runtimedamageamp" || effectKey === "addruntimedamageamp" || effectKey === "adddamageamp") && effect.valueFormula !== undefined) {
            if (!effect.valueFormula || typeof effect.valueFormula !== "object" || Array.isArray(effect.valueFormula)) {
              throw new Error(`equipment_templates.battleHooks.effects.valueFormula invalid: ${row.id}`)
            }
            if (typeof effect.valueFormula.baseAdd !== "number" || !Number.isFinite(effect.valueFormula.baseAdd)) {
              throw new Error(`equipment_templates.battleHooks.effects.valueFormula.baseAdd invalid: ${row.id}`)
            }
            if (!Array.isArray(effect.valueFormula.terms)) {
              throw new Error(`equipment_templates.battleHooks.effects.valueFormula.terms invalid: ${row.id}`)
            }
          }
          if (effectKey === "summon") {
            if (!effect.summonPayload || typeof effect.summonPayload !== "object" || Array.isArray(effect.summonPayload)) {
              throw new Error(`equipment_templates.battleHooks.effects.summonPayload invalid: ${row.id}`)
            }
            if (typeof effect.summonPayload.unitId !== "string" || effect.summonPayload.unitId.trim() === "") {
              throw new Error(`equipment_templates.battleHooks.effects.summonPayload.unitId invalid: ${row.id}`)
            }
            if (!knownEnemyIDs.has(effect.summonPayload.unitId.trim())) {
              throw new Error(`equipment_templates.battleHooks.effects.summonPayload.unitId not found: ${row.id}.${effect.summonPayload.unitId}`)
            }
            if (effect.summonPayload.side !== undefined && effect.summonPayload.side !== "player" && effect.summonPayload.side !== "enemy" && effect.summonPayload.side !== "source") {
              throw new Error(`equipment_templates.battleHooks.effects.summonPayload.side invalid: ${row.id}`)
            }
            if (effect.summonPayload.maxCountPerUnit !== undefined && (!Number.isInteger(effect.summonPayload.maxCountPerUnit) || effect.summonPayload.maxCountPerUnit < 0)) {
              throw new Error(`equipment_templates.battleHooks.effects.summonPayload.maxCountPerUnit invalid: ${row.id}`)
            }
            if (effect.summonPayload.maxCountFormula !== undefined) {
              validateSummonMaxCountFormula(
                effect.summonPayload.maxCountFormula,
                `equipment_templates.battleHooks.effects.summonPayload.maxCountFormula: ${row.id}`,
                knownBattleHookFormulaAttrIDs
              )
            }
            if (effect.summonPayload.durationTurns !== undefined && (!Number.isFinite(effect.summonPayload.durationTurns) || effect.summonPayload.durationTurns < 0 || !Number.isInteger(effect.summonPayload.durationTurns))) {
              throw new Error(`equipment_templates.battleHooks.effects.summonPayload.durationTurns invalid: ${row.id}`)
            }
            if (effect.summonPayload.durationFormula !== undefined) {
              if (!effect.summonPayload.durationFormula || typeof effect.summonPayload.durationFormula !== "object" || Array.isArray(effect.summonPayload.durationFormula)) {
                throw new Error(`equipment_templates.battleHooks.effects.summonPayload.durationFormula invalid: ${row.id}`)
              }
              if (!Number.isFinite(effect.summonPayload.durationFormula.baseAdd) || !Array.isArray(effect.summonPayload.durationFormula.terms)) {
                throw new Error(`equipment_templates.battleHooks.effects.summonPayload.durationFormula invalid: ${row.id}`)
              }
            }
          }
          if (effectKey === "playerclone") {
            if (effect.durationTurns !== undefined && (!Number.isFinite(effect.durationTurns) || effect.durationTurns < 1 || !Number.isInteger(effect.durationTurns))) {
              throw new Error(`equipment_templates.battleHooks.effects.player_clone.durationTurns invalid: ${row.id}`)
            }
            if (effect.durationFormula !== undefined) {
              if (!effect.durationFormula || typeof effect.durationFormula !== "object" || Array.isArray(effect.durationFormula)) {
                throw new Error(`equipment_templates.battleHooks.effects.player_clone.durationFormula invalid: ${row.id}`)
              }
              if (!Number.isFinite(effect.durationFormula.baseAdd) || !Array.isArray(effect.durationFormula.terms)) {
                throw new Error(`equipment_templates.battleHooks.effects.player_clone.durationFormula invalid: ${row.id}`)
              }
            }
            if (effect.statsFormula !== undefined && (typeof effect.statsFormula !== "object" || Array.isArray(effect.statsFormula))) {
              throw new Error(`equipment_templates.battleHooks.effects.player_clone.statsFormula invalid: ${row.id}`)
            }
          }
          if (
            effectKey === "addbuff" &&
            effect.durationTurns !== undefined &&
            (!Number.isFinite(effect.durationTurns) || !Number.isInteger(effect.durationTurns) || effect.durationTurns < 0)
          ) {
            throw new Error(`equipment_templates.battleHooks.effects.durationTurns invalid: ${row.id}`)
          }
          if (effect.targetMode !== undefined && effect.targetMode !== "self" && effect.targetMode !== "source" && effect.targetMode !== "targeting") {
            throw new Error(`equipment_templates.battleHooks.effects.targetMode invalid: ${row.id}`)
          }
          if (effect.targetMode === "targeting") {
            if (!effect.targeting || typeof effect.targeting !== "object" || Array.isArray(effect.targeting)) {
              throw new Error(`equipment_templates.battleHooks.effects.targeting invalid: ${row.id}`)
            }
            rejectEnemyOnlyTargetPriority(effect.targeting, `equipment_templates.battleHooks.effects.targeting: ${row.id}`)
            if (!Array.isArray(effect.targeting.targetPositions) || effect.targeting.targetPositions.length === 0) {
              throw new Error(`equipment_templates.battleHooks.effects.targeting.targetPositions invalid: ${row.id}`)
            }
            if (!Number.isFinite(effect.targeting?.targetCount?.base) || !Number.isInteger(effect.targeting.targetCount.base) || effect.targeting.targetCount.base < 1) {
              throw new Error(`equipment_templates.battleHooks.effects.targeting.targetCount.base invalid: ${row.id}`)
            }
          }
        }
      }
    }
    if (row.defaultAffixSlots !== undefined) {
      if (!Array.isArray(row.defaultAffixSlots)) {
        throw new Error(`equipment_templates.defaultAffixSlots invalid: ${row.id}`)
      }
      if (row.defaultAffixSlots.length > 0 && row.affixSlotCount <= 0) {
        throw new Error(`equipment_templates.defaultAffixSlots requires affixSlotCount > 0: ${row.id}`)
      }
      const seenSlotIndex = new Set()
      for (const slot of row.defaultAffixSlots) {
        if (!slot || typeof slot !== "object" || Array.isArray(slot)) {
          throw new Error(`equipment_templates.defaultAffixSlots entry invalid: ${row.id}`)
        }
        if (!Number.isFinite(slot.slotIndex) || !Number.isInteger(slot.slotIndex) || slot.slotIndex < 0) {
          throw new Error(`equipment_templates.defaultAffixSlots.slotIndex invalid: ${row.id}`)
        }
        if (slot.slotIndex >= row.affixSlotCount) {
          throw new Error(`equipment_templates.defaultAffixSlots.slotIndex out of range: ${row.id}.${slot.slotIndex}`)
        }
        if (seenSlotIndex.has(slot.slotIndex)) {
          throw new Error(`equipment_templates.defaultAffixSlots duplicate slotIndex: ${row.id}.${slot.slotIndex}`)
        }
        seenSlotIndex.add(slot.slotIndex)
        if (typeof slot.affixId !== "string" || slot.affixId.trim() === "") {
          throw new Error(`equipment_templates.defaultAffixSlots.affixId invalid: ${row.id}`)
        }
        if (!knownAffixIDs.has(slot.affixId.trim())) {
          throw new Error(`equipment_templates.defaultAffixSlots.affixId not found or inactive: ${row.id}.${slot.affixId}`)
        }
        if (typeof slot.rarityId !== "string" || slot.rarityId.trim() === "") {
          throw new Error(`equipment_templates.defaultAffixSlots.rarityId required: ${row.id}.${slot.affixId}`)
        }
        if (!knownRarityIDs.has(slot.rarityId.trim())) {
          throw new Error(`equipment_templates.defaultAffixSlots.rarityId invalid: ${row.id}.${slot.affixId}.${slot.rarityId}`)
        }
        const allowedRarities = allowedRarityByAffixID.get(slot.affixId.trim())
        if (allowedRarities instanceof Set && allowedRarities.size > 0 && !allowedRarities.has(slot.rarityId.trim())) {
          throw new Error(`equipment_templates.defaultAffixSlots.rarityId not supported by affix: ${row.id}.${slot.affixId}.${slot.rarityId}`)
        }
        const affixType = affixTypeByAffixID.get(slot.affixId.trim())
        if (affixType && affixType !== row.equipType) {
          throw new Error(`equipment_templates.defaultAffixSlots.affixType mismatch equipType: ${row.id}.${slot.affixId}.${affixType}.${row.equipType}`)
        }
        if (typeof slot.rollValue !== "number" || !Number.isFinite(slot.rollValue) || slot.rollValue < 0 || slot.rollValue > 1) {
          throw new Error(`equipment_templates.defaultAffixSlots.rollValue invalid: ${row.id}.${slot.affixId}`)
        }
      }
    }
    if (row.enhanceAffixMilestonePools !== undefined) {
      if (!row.enhanceAffixMilestonePools || typeof row.enhanceAffixMilestonePools !== "object" || Array.isArray(row.enhanceAffixMilestonePools)) {
        throw new Error(`equipment_templates.enhanceAffixMilestonePools invalid: ${row.id}`)
      }
      if (row.affixSlotCount <= 0 && Object.keys(row.enhanceAffixMilestonePools).length > 0) {
        throw new Error(`equipment_templates.enhanceAffixMilestonePools requires affixSlotCount > 0: ${row.id}`)
      }
      for (const [level, pool] of Object.entries(row.enhanceAffixMilestonePools)) {
        validateEnhanceLevelKey(row.id, level, row.enhanceMaxLevel)
        if (!Array.isArray(pool) || pool.length === 0) {
          throw new Error(`equipment_templates.enhanceAffixMilestonePools entry invalid: ${row.id}.${level}`)
        }
        for (const entry of pool) {
          if (!entry || typeof entry !== "object" || Array.isArray(entry)) {
            throw new Error(`equipment_templates.enhanceAffixMilestonePools item invalid: ${row.id}.${level}`)
          }
          if (typeof entry.affixId !== "string" || entry.affixId.trim() === "") {
            throw new Error(`equipment_templates.enhanceAffixMilestonePools.affixId invalid: ${row.id}.${level}`)
          }
          const affixID = entry.affixId.trim()
          if (!knownAffixIDs.has(affixID)) {
            throw new Error(`equipment_templates.enhanceAffixMilestonePools.affixId not found or inactive: ${row.id}.${level}.${affixID}`)
          }
          const affixType = affixTypeByAffixID.get(affixID)
          if (affixType !== row.equipType) {
            throw new Error(`equipment_templates.enhanceAffixMilestonePools.affixType mismatch equipType: ${row.id}.${level}.${affixID}.${affixType}.${row.equipType}`)
          }
          if (typeof entry.rarityId !== "string" || entry.rarityId.trim() === "") {
            throw new Error(`equipment_templates.enhanceAffixMilestonePools.rarityId required: ${row.id}.${level}.${affixID}`)
          }
          const rarityID = entry.rarityId.trim()
          if (!knownRarityIDs.has(rarityID)) {
            throw new Error(`equipment_templates.enhanceAffixMilestonePools.rarityId invalid: ${row.id}.${level}.${affixID}.${rarityID}`)
          }
          const allowedRarities = allowedRarityByAffixID.get(affixID)
          if (allowedRarities instanceof Set && allowedRarities.size > 0 && !allowedRarities.has(rarityID)) {
            throw new Error(`equipment_templates.enhanceAffixMilestonePools.rarityId not supported by affix: ${row.id}.${level}.${affixID}.${rarityID}`)
          }
          if (entry.rollValue !== undefined && (typeof entry.rollValue !== "number" || !Number.isFinite(entry.rollValue) || entry.rollValue < 0 || entry.rollValue > 1)) {
            throw new Error(`equipment_templates.enhanceAffixMilestonePools.rollValue invalid: ${row.id}.${level}.${affixID}`)
          }
        }
      }
    }
    if (!Array.isArray(row.enhanceProtectItemId) || row.enhanceProtectItemId.some((item) => typeof item !== "string" || item.trim() === "")) {
      throw new Error(`equipment_templates.enhanceProtectItemId invalid: ${row.id}`)
    }
    if (typeof row.enhanceExp !== "number" || !Number.isFinite(row.enhanceExp) || row.enhanceExp < 0) {
      throw new Error(`equipment_templates.enhanceExp invalid: ${row.id}`)
    }
    if (!row.enhanceCostCurve || typeof row.enhanceCostCurve !== "object" || Array.isArray(row.enhanceCostCurve)) {
      throw new Error(`equipment_templates.enhanceCostCurve invalid: ${row.id}`)
    }
    validateIntegerMap("equipment_templates.enhanceCostCurve.base", row.id, row.enhanceCostCurve.base)
    validateIntegerMap("equipment_templates.enhanceCostCurve.step", row.id, row.enhanceCostCurve.step)
    if (!row.enhanceCostCurve.overrides || typeof row.enhanceCostCurve.overrides !== "object" || Array.isArray(row.enhanceCostCurve.overrides)) {
      throw new Error(`equipment_templates.enhanceCostCurve.overrides invalid: ${row.id}`)
    }
    for (const [level, costMap] of Object.entries(row.enhanceCostCurve.overrides)) {
      validateEnhanceLevelKey(row.id, level, row.enhanceMaxLevel)
      validateIntegerMap("equipment_templates.enhanceCostCurve.overrides", `${row.id}.${level}`, costMap)
    }
    if (!row.enhanceAttrCurve || typeof row.enhanceAttrCurve !== "object" || Array.isArray(row.enhanceAttrCurve)) {
      throw new Error(`equipment_templates.enhanceAttrCurve invalid: ${row.id}`)
    }
    validateFiniteMap("equipment_templates.enhanceAttrCurve.base", row.id, row.enhanceAttrCurve.base)
    validateFiniteMap("equipment_templates.enhanceAttrCurve.step", row.id, row.enhanceAttrCurve.step)
    if (!row.enhanceAttrCurve.overrides || typeof row.enhanceAttrCurve.overrides !== "object" || Array.isArray(row.enhanceAttrCurve.overrides)) {
      throw new Error(`equipment_templates.enhanceAttrCurve.overrides invalid: ${row.id}`)
    }
    for (const [level, attrMap] of Object.entries(row.enhanceAttrCurve.overrides)) {
      validateEnhanceLevelKey(row.id, level, row.enhanceMaxLevel)
      validateFiniteMap("equipment_templates.enhanceAttrCurve.overrides", `${row.id}.${level}`, attrMap)
    }
  }
}

function validateEquipmentItemFlags(items, equipmentTemplates) {
  const templateIDSet = new Set(
    (equipmentTemplates || [])
      .map((row) => (typeof row?.id === "string" ? row.id.trim() : ""))
      .filter(Boolean)
  )
  const itemMap = new Map(
    (items || [])
      .filter((row) => row && typeof row === "object" && typeof row.id === "string")
      .map((row) => [row.id, row])
  )

  for (const templateID of templateIDSet) {
    if (!itemMap.has(templateID)) {
      throw new Error(`equipment_templates.id not found in items: ${templateID}`)
    }
  }
  for (const item of items || []) {
    if (!item || typeof item !== "object" || typeof item.id !== "string") continue
    const shouldBeEquipment = templateIDSet.has(item.id)
    if (item.isEquipment !== shouldBeEquipment) {
      throw new Error(`item.isEquipment inconsistent: ${item.id}`)
    }
  }
}

function applyEquipmentMetadataToItems(items, equipmentTemplates) {
  const byTemplateID = new Map()
  for (const row of equipmentTemplates || []) {
    if (!row || typeof row !== "object") continue
    const id = typeof row.id === "string" ? row.id.trim() : ""
    if (!id) continue
    byTemplateID.set(id, parseBaseAttrs(row.baseAttrs))
  }
  return (items || []).map((item) => {
    if (!item || typeof item !== "object") return item
    const fromTemplate = byTemplateID.get(item.id)
    if (fromTemplate && typeof fromTemplate === "object") {
      return copyRowSourceMeta({
        ...item,
        isEquipment: true,
        baseAttrs: fromTemplate
      }, item)
    }
    return copyRowSourceMeta({
      ...item,
      isEquipment: false,
      baseAttrs: parseBaseAttrs(item.baseAttrs)
    }, item)
  })
}

function parseIconIdentifier(iconID) {
  if (typeof iconID !== "string" || iconID.trim() === "") return null
  const [rawType, ...rest] = iconID.trim().split(".")
  const type = rawType.trim()
  const id = rest.join(".").trim()
  if (!type || !id) return null
  return { type, id }
}

function defaultIconPath(iconID) {
  const parsed = parseIconIdentifier(iconID)
  if (!parsed) return `/assets/icons/common/${iconID}.png`
  const folder = parsed.type.endsWith("s") ? parsed.type : `${parsed.type}s`
  return `/assets/icons/${folder}/${parsed.id}.png`
}

let existingIconPathsCache = null

function loadExistingIconPaths() {
  if (existingIconPathsCache) return existingIconPathsCache
  const out = new Set()
  if (!fs.existsSync(webIconsDir)) {
    existingIconPathsCache = out
    return out
  }
  const walk = (dir, relDir = "") => {
    const entries = fs.readdirSync(dir, { withFileTypes: true })
    for (const entry of entries) {
      const nextAbs = path.join(dir, entry.name)
      const nextRel = relDir ? path.posix.join(relDir, entry.name) : entry.name
      if (entry.isDirectory()) {
        walk(nextAbs, nextRel)
        continue
      }
      if (!entry.isFile() || !entry.name.toLowerCase().endsWith(".png")) continue
      out.add(`/assets/icons/${nextRel}`)
    }
  }
  walk(webIconsDir, "")
  existingIconPathsCache = out
  return out
}

function resolveIconPathCandidates(iconID, rawPath) {
  const parsed = parseIconIdentifier(iconID)
  const out = []
  if (typeof rawPath === "string" && rawPath.trim() !== "") {
    out.push(normalizeIconPath(iconID, rawPath))
  }
  if (!parsed) {
    out.push(`/assets/icons/common/${iconID}.png`)
    return [...new Set(out)]
  }
  const typeCandidates = new Set([parsed.type])
  if (parsed.type.endsWith("s")) {
    const singular = parsed.type.slice(0, -1).trim()
    if (singular) typeCandidates.add(singular)
  } else {
    typeCandidates.add(`${parsed.type}s`)
  }
  for (const folder of typeCandidates) {
    out.push(`/assets/icons/${folder}/${parsed.id}.png`)
  }
  out.push(`/assets/icons/common/${parsed.id}.png`)
  return [...new Set(out)]
}

function resolveExistingIconPath(iconID, rawPath) {
  const known = loadExistingIconPaths()
  const candidates = resolveIconPathCandidates(iconID, rawPath)
  for (const candidate of candidates) {
    if (known.has(candidate)) return candidate
  }
  return ""
}

function normalizeIconPath(iconID, rawPath) {
  if (typeof rawPath !== "string" || rawPath.trim() === "") return defaultIconPath(iconID)
  const parsed = parseIconIdentifier(iconID)
  if (!parsed) return rawPath
  const normalized = rawPath.startsWith("/") ? rawPath : `/${rawPath}`
  const file = path.posix.basename(normalized)
  const dir = path.posix.dirname(normalized)
  const legacyCandidates = new Set([
    `${parsed.type}.${parsed.id}.png`,
    `${parsed.type}:${parsed.id}.png`
  ])
  if (legacyCandidates.has(file)) {
    return `${dir}/${parsed.id}.png`
  }
  return normalized
}

function normalizeIcons(items, actions, icons) {
  const out = Array.isArray(icons) ? [...icons] : []
  const known = new Set(out.map((icon) => icon?.id).filter((id) => typeof id === "string"))
  const sources = [...(Array.isArray(items) ? items : []), ...(Array.isArray(actions) ? actions : [])]
  for (const row of sources) {
    if (typeof row?.iconId !== "string" || row.iconId === "") continue
    if (known.has(row.iconId)) continue
    out.push({ id: row.iconId, path: defaultIconPath(row.iconId) })
    known.add(row.iconId)
  }
  return out.map((icon) => ({
    ...icon,
    path: resolveExistingIconPath(icon.id, icon.path)
  }))
}

function normalizeActionRows(rows, menu) {
  const normalizeRewardKeys = (raw) => {
    if (!raw || typeof raw !== "object") return {}
    const alias = { }
    const out = {}
    for (const [key, value] of Object.entries(raw)) {
      const normalized = alias[key] || key
      out[normalized] = value
    }
    return out
  }
  const normalizeItemReward = (raw) => {
    if (!raw || typeof raw !== "object" || Array.isArray(raw)) return {}
    const out = {}
    for (const [itemID, reward] of Object.entries(raw)) {
      const normalizedItemID = String(itemID || "").trim()
      if (!normalizedItemID) continue
      if (typeof reward === "number") {
        out[normalizedItemID] = reward
        continue
      }
      if (!reward || typeof reward !== "object" || Array.isArray(reward)) {
        out[normalizedItemID] = reward
        continue
      }
      const normalized = { ...reward }
      if (typeof normalized.grantPolicy === "string") {
        normalized.grantPolicy = normalized.grantPolicy.trim()
      }
      if (normalized.defaultAffixRarityWeights && typeof normalized.defaultAffixRarityWeights === "object" && !Array.isArray(normalized.defaultAffixRarityWeights)) {
        const weights = {}
        for (const [rarityID, weight] of Object.entries(normalized.defaultAffixRarityWeights)) {
          const normalizedRarityID = String(rarityID || "").trim()
          const normalizedWeight = Number(weight)
          if (normalizedRarityID) weights[normalizedRarityID] = normalizedWeight
        }
        normalized.defaultAffixRarityWeights = weights
      }
      out[normalizedItemID] = normalized
    }
    return out
  }
  const normalizeMaterialInstanceRequirements = (raw) => {
    if (!raw || typeof raw !== "object" || Array.isArray(raw)) return {}
    const out = {}
    for (const [itemID, requirement] of Object.entries(raw)) {
      const normalizedItemID = String(itemID || "").trim()
      if (!normalizedItemID || !requirement || typeof requirement !== "object" || Array.isArray(requirement)) continue
      out[normalizedItemID] = {
        enhanceLevel: Number(requirement.enhanceLevel),
        requireTemplateAffixes: requirement.requireTemplateAffixes === true,
        durability: String(requirement.durability || "").trim()
      }
    }
    return out
  }
  return (rows || []).map((row) => {
    const { durationModifiers: _legacyDurationModifiers, ...rowWithoutLegacyDuration } = row || {}
    const id = typeof row.id === "string" ? row.id : ""
    const mainType = typeof row.mainType === "string" && row.mainType ? row.mainType : typeof row.main_type === "string" && row.main_type ? row.main_type : menu
    const normalizedMenu = typeof row.menu === "string" && row.menu ? row.menu : mainType
    const subType = typeof row.subType === "string" && row.subType ? row.subType : typeof row.sub_type === "string" && row.sub_type ? row.sub_type : "common"
    return {
      ...rowWithoutLegacyDuration,
      mainType,
      menu: normalizedMenu,
      subType,
      disabled: parseBoolean(row?.disabled, false),
      rarity: typeof row.rarity === "string" && row.rarity.trim() ? row.rarity.trim() : "common",
      iconId: parseOptionalString(row?.iconId) || (id ? `action.${id}` : undefined),
      nameKey: typeof row.nameKey === "string" && row.nameKey ? row.nameKey : `action.${id}.name`,
      descKey: typeof row.descKey === "string" && row.descKey ? row.descKey : `action.${id}.desc`,
      queueable: typeof row.queueable === "boolean" ? row.queueable : true,
      durationS: Math.max(1, Math.round(typeof row.durationS === "number" ? row.durationS : Number(row.durationS || 1))),
      costs: typeof row.costs === "object" && row.costs ? row.costs : {},
      materialInstanceRequirements: normalizeMaterialInstanceRequirements(row?.materialInstanceRequirements),
      statusRequirement: parseNumberMap(row.statusRequirement),
      buildingRequirement: parseNumberMap(row.buildingRequirement),
      itemReward: normalizeItemReward(row.itemReward),
      statusExpReward: normalizeRewardKeys(row.statusExpReward),
      economyIndustryTier: parseNumber(row?.economyIndustryTier, 1),
      economyDifficultyMultiplier: parseNumber(row?.economyDifficultyMultiplier, 1),
    }
  })
}

function normalizeActionSubTypes(rows) {
  return (rows || []).map((row, index) => {
    const id =
      typeof row.id === "string" && row.id
        ? row.id
        : typeof row.subType === "string" && row.subType
          ? row.subType
          : typeof row.sub_type === "string" && row.sub_type
            ? row.sub_type
            : ""
    const normalizedOrder =
      typeof row.order === "number"
        ? row.order
        : Number.isFinite(Number(row.order))
          ? Number(row.order)
          : Number.isFinite(Number(row.sortOrder))
            ? Number(row.sortOrder)
            : index
    const actions = Array.isArray(row.actions)
      ? row.actions.map((actionID) => String(actionID).trim()).filter(Boolean)
      : []
    return {
      id,
      nameKey:
        typeof row.nameKey === "string" && row.nameKey
          ? row.nameKey
          : `action_sub_type.${id}.name`,
      descKey:
        typeof row.descKey === "string" && row.descKey
          ? row.descKey
          : `action_sub_type.${id}.desc`,
      icon: typeof row.icon === "string" ? row.icon : "",
      color: typeof row.color === "string" && row.color ? row.color : "#333",
      order: normalizedOrder,
      actions
    }
  })
}

function normalizeActionMainTypes(rows) {
  return (rows || []).map((row, index) => {
    const id =
      typeof row.id === "string" && row.id
        ? row.id
        : typeof row.mainType === "string" && row.mainType
          ? row.mainType
          : typeof row.main_type === "string" && row.main_type
            ? row.main_type
            : ""
    const normalizedOrder =
      typeof row.order === "number"
        ? row.order
        : Number.isFinite(Number(row.order))
          ? Number(row.order)
          : Number.isFinite(Number(row.sortOrder))
            ? Number(row.sortOrder)
            : index
    const actions = Array.isArray(row.actions)
      ? row.actions.map((actionID) => String(actionID).trim()).filter(Boolean)
      : []
    return {
      id,
      nameKey:
        typeof row.nameKey === "string" && row.nameKey
          ? row.nameKey
          : `action_main_type.${id}.name`,
      descKey:
        typeof row.descKey === "string" && row.descKey
          ? row.descKey
          : `action_main_type.${id}.desc`,
      icon: typeof row.icon === "string" ? row.icon : "",
      color: typeof row.color === "string" && row.color ? row.color : "#333",
      order: normalizedOrder,
      actions
    }
  })
}

function collectActionRows(extraTables) {
  const out = []
  const fromAggregated = loadRowsFromWorkbookSheet(AGGREGATED_WORKBOOK, AGGREGATED_EXPORT_ACTIONS_SHEET)
  if (fromAggregated && fromAggregated.length > 0) {
    out.push(...normalizeActionRows(fromAggregated, ""))
  } else {
    // out.push(...normalizeActionRows(extraTables.actions || [], ""))

    // const tables = loadExcelTables()
    // for (const [sheetName, rows] of Object.entries(tables)) {
    //   if (!sheetName.startsWith("actions_")) continue
    //   if (!Array.isArray(rows) || rows.length === 0) continue
    //   const fallbackMainType = sheetName.slice("actions_".length)
    //   out.push(...normalizeActionRows(rows, fallbackMainType))
    // }
  }
  out.push(...normalizeActionRows(extraTables.actions || [], ""))

  const seen = new Set()
  const deduped = []
  for (const row of out) {
    if (typeof row.id !== "string" || row.id === "") continue
    if (typeof row.mainType !== "string" || row.mainType === "") {
      throw new Error(`action.mainType missing: ${row.id}`)
    }
    const existingIndex = deduped.findIndex((entry) => entry.id === row.id)
    if (existingIndex >= 0) {
      deduped[existingIndex] = row
      continue
    }
    if (seen.has(row.id)) {
      throw new Error(`duplicate action.id: ${row.id}`)
    }
    seen.add(row.id)
    deduped.push(row)
  }
  return deduped
}

function collectActionMainTypes(rawMainTypes, actions) {
  const normalized = normalizeActionMainTypes(rawMainTypes)
  const mainTypeMap = new Map()
  for (const row of normalized) {
    if (!row.id) continue
    if (mainTypeMap.has(row.id)) {
      throw new Error(`duplicate action_main_types.id: ${row.id}`)
    }
    mainTypeMap.set(row.id, row)
  }

  for (const action of actions) {
    if (typeof action.mainType !== "string" || action.mainType === "") continue
    if (!mainTypeMap.has(action.mainType)) {
      mainTypeMap.set(action.mainType, {
        id: action.mainType,
        nameKey: `action_main_type.${action.mainType}.name`,
        descKey: `action_main_type.${action.mainType}.desc`,
        icon: "",
        color: "#333",
        order: Number.MAX_SAFE_INTEGER,
        actions: []
      })
    }
  }

  const rows = [...mainTypeMap.values()].sort((a, b) => {
    const ao = typeof a.order === "number" ? a.order : Number.MAX_SAFE_INTEGER
    const bo = typeof b.order === "number" ? b.order : Number.MAX_SAFE_INTEGER
    if (ao !== bo) return ao - bo
    return a.id.localeCompare(b.id)
  })

  const actionGroups = new Map()
  for (const action of actions) {
    if (!actionGroups.has(action.mainType)) actionGroups.set(action.mainType, [])
    actionGroups.get(action.mainType).push(action.id)
  }
  return rows.map((row) => ({
    ...row,
    actions: actionGroups.get(row.id) || row.actions || []
  }))
}

function collectActionSubTypes(rawSubTypes, actions) {
  const normalized = normalizeActionSubTypes(rawSubTypes)
  const subTypeMap = new Map()
  for (const row of normalized) {
    if (!row.id) continue
    if (subTypeMap.has(row.id)) {
      throw new Error(`duplicate action_sub_types.id: ${row.id}`)
    }
    subTypeMap.set(row.id, row)
  }

  for (const action of actions) {
    const subType = typeof action.subType === "string" && action.subType ? action.subType : "common"
    if (!subTypeMap.has(subType)) {
      subTypeMap.set(subType, {
        id: subType,
        nameKey: `action_sub_type.${subType}.name`,
        descKey: `action_sub_type.${subType}.desc`,
        icon: "",
        color: "#333",
        order: Number.MAX_SAFE_INTEGER,
        actions: []
      })
    }
  }

  const rows = [...subTypeMap.values()].sort((a, b) => {
    const ao = typeof a.order === "number" ? a.order : Number.MAX_SAFE_INTEGER
    const bo = typeof b.order === "number" ? b.order : Number.MAX_SAFE_INTEGER
    if (ao !== bo) return ao - bo
    return a.id.localeCompare(b.id)
  })

  const actionGroups = new Map()
  for (const action of actions) {
    const subType = typeof action.subType === "string" && action.subType ? action.subType : "common"
    if (!actionGroups.has(subType)) actionGroups.set(subType, [])
    actionGroups.get(subType).push(action.id)
  }
  return rows.map((row) => ({
    ...row,
    actions: actionGroups.get(row.id) || row.actions || []
  }))
}

function toNonNegativeIntegerMap(raw) {
  const parsed = parseNumberMap(raw)
  const out = {}
  for (const [key, value] of Object.entries(parsed)) {
    if (!Number.isFinite(value)) continue
    const normalized = Math.max(0, Math.floor(value))
    out[key] = normalized
  }
  return out
}

function normalizeBuildings(rows) {
  return (rows || [])
    .map((raw, index) => {
      const id = typeof raw?.id === "string" ? raw.id.trim() : ""
      if (!id) return null
      const maxLevel = Math.max(1, Math.floor(parseNumber(raw?.maxLevel, 1)))
      const levels = (Array.isArray(raw?.levels) ? raw.levels : [])
        .map((entry) => ({
          level: Math.max(1, Math.floor(parseNumber(entry?.level, 1))),
          durationS: Math.max(1, Math.floor(parseNumber(entry?.durationS, 1))),
          costs: toNonNegativeIntegerMap(entry?.costs),
          statusRequirement: toNonNegativeIntegerMap(entry?.statusRequirement),
          bonuses: toNonNegativeIntegerMap(entry?.bonuses)
        }))
        .sort((a, b) => a.level - b.level)
      return {
        ...raw,
        id,
        iconId: parseOptionalString(raw?.iconId) || `building.${id}`,
        // Building i18n keys are generated from id; manual sheet values are ignored.
        nameKey: `building.${id}.name`,
        descKey: `building.${id}.desc`,
        order: Number.isFinite(Number(raw?.order)) ? Math.floor(Number(raw.order)) : index,
        maxLevel,
        baseDurationS: Math.max(1, Math.floor(parseNumber(raw?.baseDurationS, 1))),
        stepDurationS: Math.max(0, Math.floor(parseNumber(raw?.stepDurationS, 0))),
        baseCosts: toNonNegativeIntegerMap(raw?.baseCosts),
        stepCosts: toNonNegativeIntegerMap(raw?.stepCosts),
        baseStatusRequirement: toNonNegativeIntegerMap(raw?.baseStatusRequirement),
        stepStatusRequirement: toNonNegativeIntegerMap(raw?.stepStatusRequirement),
        levels,
      }
    })
    .filter(Boolean)
    .sort((a, b) => {
      if (a.order !== b.order) return a.order - b.order
      return a.id.localeCompare(b.id)
    })
}

function validateBuildings(buildings, iconIdSet, attributes) {
  if (!Array.isArray(buildings)) throw new Error("buildings must be an array")
  const ids = new Set()
  const attrSet = new Set(
    (Array.isArray(attributes) ? attributes : [])
      .map((row) => (typeof row?.id === "string" ? row.id.trim() : ""))
      .filter(Boolean)
  )
  for (const row of buildings) {
    if (!row || typeof row !== "object") throw new Error("buildings row must be object")
    if (!row.id || typeof row.id !== "string") throw new Error("buildings.id is required string")
    if (ids.has(row.id)) throw new Error(`duplicate buildings.id: ${row.id}`)
    ids.add(row.id)
    if (!row.iconId || typeof row.iconId !== "string") throw new Error(`buildings.iconId invalid: ${row.id}`)
    if (!iconIdSet.has(row.iconId)) throw new Error(`buildings.iconId not found: ${row.id}.${row.iconId}`)
    if (!row.nameKey || typeof row.nameKey !== "string") throw new Error(`buildings.nameKey invalid: ${row.id}`)
    if (row.descKey !== undefined && (typeof row.descKey !== "string" || row.descKey === "")) {
      throw new Error(`buildings.descKey invalid: ${row.id}`)
    }
    if (!Number.isFinite(row.order)) throw new Error(`buildings.order invalid: ${row.id}`)
    if (!Number.isFinite(row.maxLevel) || row.maxLevel < 1) throw new Error(`buildings.maxLevel invalid: ${row.id}`)
    if (!Number.isFinite(row.baseDurationS) || row.baseDurationS < 1) throw new Error(`buildings.baseDurationS invalid: ${row.id}`)
    if (!Number.isFinite(row.stepDurationS) || row.stepDurationS < 0) throw new Error(`buildings.stepDurationS invalid: ${row.id}`)
    const mapFields = ["baseCosts", "stepCosts", "baseStatusRequirement", "stepStatusRequirement"]
    for (const fieldName of mapFields) {
      const rawMap = row[fieldName]
      if (!rawMap || typeof rawMap !== "object" || Array.isArray(rawMap)) {
        throw new Error(`buildings.${fieldName} invalid: ${row.id}`)
      }
      for (const [key, value] of Object.entries(rawMap)) {
        if (!key) throw new Error(`buildings.${fieldName} key invalid: ${row.id}`)
        if (!Number.isFinite(value) || value < 0) throw new Error(`buildings.${fieldName} value invalid: ${row.id}.${key}`)
        if (fieldName.endsWith("StatusRequirement") && attrSet.size > 0 && !attrSet.has(key)) {
          throw new Error(`buildings.${fieldName} attr not found: ${row.id}.${key}`)
        }
      }
    }
    if (!Array.isArray(row.levels)) throw new Error(`buildings.levels invalid: ${row.id}`)
    if (row.levels.length > 0) {
      if (row.levels.length !== row.maxLevel) throw new Error(`buildings.levels must cover maxLevel: ${row.id}`)
      for (let index = 0; index < row.levels.length; index += 1) {
        const level = row.levels[index]
        if (level.level !== index + 1) throw new Error(`buildings.levels sequence invalid: ${row.id}.${level.level}`)
        if (!Number.isFinite(level.durationS) || level.durationS < 1) throw new Error(`buildings.levels duration invalid: ${row.id}.${level.level}`)
        for (const fieldName of ["costs", "statusRequirement", "bonuses"]) {
          const rawMap = level[fieldName]
          if (!rawMap || typeof rawMap !== "object" || Array.isArray(rawMap)) throw new Error(`buildings.levels.${fieldName} invalid: ${row.id}.${level.level}`)
          for (const [key, value] of Object.entries(rawMap)) {
            if (!key || !Number.isFinite(value) || value < 0) throw new Error(`buildings.levels.${fieldName} value invalid: ${row.id}.${level.level}.${key}`)
            if (fieldName === "statusRequirement" && attrSet.size > 0 && !attrSet.has(key)) {
              throw new Error(`buildings.levels status attr not found: ${row.id}.${level.level}.${key}`)
            }
          }
        }
      }
    }
  }
}

function validateActions(actions, actionMainTypes, actionSubTypes, iconIdSet, rarityDefinitions, equipmentTemplates, affixDefinitions, items) {
  if (!Array.isArray(actions)) throw new Error("actions must be an array")
  const playerOutputs = new Set(actions.filter(action => !action.disabled).flatMap(action => Object.keys(action.itemReward || {})))
  if (!Array.isArray(actionMainTypes)) throw new Error("action_main_types must be an array")
  if (!Array.isArray(actionSubTypes)) throw new Error("action_sub_types must be an array")
  const knownMainTypes = new Set(actionMainTypes.map((row) => row.id))
  const knownSubTypes = new Set(actionSubTypes.map((row) => row.id))
  const knownRarityIDs = new Set(
    (Array.isArray(rarityDefinitions) ? rarityDefinitions : [])
      .map((row) => (typeof row?.id === "string" ? row.id.trim() : ""))
      .filter(Boolean)
  )
  const equipmentByID = new Map(
    (Array.isArray(equipmentTemplates) ? equipmentTemplates : [])
      .filter((row) => row && typeof row === "object" && typeof row.id === "string" && row.id.trim())
      .map((row) => [row.id.trim(), row])
  )
  const affixByID = new Map(
    (Array.isArray(affixDefinitions) ? affixDefinitions : [])
      .filter((row) => row && typeof row === "object" && typeof row.id === "string" && row.id.trim() && row.active !== false)
      .map((row) => [row.id.trim(), row])
  )
  const itemByID = new Map(
    (Array.isArray(items) ? items : [])
      .filter((row) => row && typeof row === "object" && typeof row.id === "string" && row.id.trim())
      .map((row) => [row.id.trim(), row])
  )
  const ids = new Set()
  for (const row of actions) {
    if (!row || typeof row !== "object") throw new Error("action row must be object")
    if (!row.id || typeof row.id !== "string") throw new Error("action.id is required string")
    if (ids.has(row.id)) throw new Error(`duplicate action id: ${row.id}`)
    ids.add(row.id)
    if (!row.mainType || typeof row.mainType !== "string") throw new Error(`action.mainType invalid: ${row.id}`)
    if (!knownMainTypes.has(row.mainType)) throw new Error(`action.mainType not found: ${row.id}.${row.mainType}`)
    if (!row.subType || typeof row.subType !== "string") throw new Error(`action.subType invalid: ${row.id}`)
    if (!knownSubTypes.has(row.subType)) throw new Error(`action.subType not found: ${row.id}.${row.subType}`)
    if (!row.menu || typeof row.menu !== "string") throw new Error(`action.menu invalid: ${row.id}`)
    if (row.disabled !== undefined && typeof row.disabled !== "boolean") throw new Error(`action.disabled invalid: ${row.id}`)
    if (!row.rarity || typeof row.rarity !== "string") throw new Error(`action.rarity invalid: ${row.id}`)
    if (knownRarityIDs.size > 0 && !knownRarityIDs.has(row.rarity)) throw new Error(`action.rarity not found: ${row.id}.${row.rarity}`)
    if (row.iconId !== undefined && (typeof row.iconId !== "string" || row.iconId === "")) {
      throw new Error(`action.iconId invalid: ${row.id}`)
    }
    if (typeof row.iconId === "string" && row.iconId !== "" && iconIdSet instanceof Set && !iconIdSet.has(row.iconId)) {
      throw new Error(`action.iconId not found: ${row.id}.${row.iconId}`)
    }
    if (typeof row.economyIndustryTier !== "number" || !Number.isFinite(row.economyIndustryTier) || row.economyIndustryTier <= 0) {
      throw new Error(`action.economyIndustryTier invalid: ${row.id}`)
    }
    if (typeof row.economyDifficultyMultiplier !== "number" || !Number.isFinite(row.economyDifficultyMultiplier) || row.economyDifficultyMultiplier <= 0) {
      throw new Error(`action.economyDifficultyMultiplier invalid: ${row.id}`)
    }
    if (row.buildingRequirement !== undefined) {
      if (!row.buildingRequirement || typeof row.buildingRequirement !== "object" || Array.isArray(row.buildingRequirement)) {
        throw new Error(`action.buildingRequirement invalid: ${row.id}`)
      }
      for (const [buildingID, required] of Object.entries(row.buildingRequirement)) {
        if (!buildingID || typeof required !== "number" || !Number.isFinite(required) || required < 0) {
          throw new Error(`action.buildingRequirement value invalid: ${row.id}.${buildingID}`)
        }
      }
    }
    if (row.materialInstanceRequirements !== undefined) {
      if (!row.materialInstanceRequirements || typeof row.materialInstanceRequirements !== "object" || Array.isArray(row.materialInstanceRequirements)) {
        throw new Error(`action.materialInstanceRequirements invalid: ${row.id}`)
      }
      for (const [itemID, requirement] of Object.entries(row.materialInstanceRequirements)) {
        if (!Object.prototype.hasOwnProperty.call(row.costs || {}, itemID)) {
          throw new Error(`action.materialInstanceRequirements cost not found: ${row.id}.${itemID}`)
        }
        if (!equipmentByID.has(itemID)) {
          throw new Error(`action.materialInstanceRequirements equipment not found: ${row.id}.${itemID}`)
        }
        if (!requirement || typeof requirement !== "object" || Array.isArray(requirement)) {
          throw new Error(`action.materialInstanceRequirements value invalid: ${row.id}.${itemID}`)
        }
        if (!Number.isInteger(requirement.enhanceLevel) || requirement.enhanceLevel < 0) {
          throw new Error(`action.materialInstanceRequirements.enhanceLevel invalid: ${row.id}.${itemID}`)
        }
        if (typeof requirement.requireTemplateAffixes !== "boolean") {
          throw new Error(`action.materialInstanceRequirements.requireTemplateAffixes invalid: ${row.id}.${itemID}`)
        }
        if (requirement.durability !== "full") {
          throw new Error(`action.materialInstanceRequirements.durability invalid: ${row.id}.${itemID}`)
        }
      }
    }
    if (row.itemReward !== undefined) {
      if (!row.itemReward || typeof row.itemReward !== "object" || Array.isArray(row.itemReward)) {
        throw new Error(`action.itemReward invalid: ${row.id}`)
      }
      for (const [itemID, reward] of Object.entries(row.itemReward)) {
        if (!itemID) throw new Error(`action.itemReward item id invalid: ${row.id}`)
        if (typeof reward === "number") continue
        if (!reward || typeof reward !== "object" || Array.isArray(reward)) {
          throw new Error(`action.itemReward value invalid: ${row.id}.${itemID}`)
        }
        const grantPolicy = typeof reward.grantPolicy === "string" ? reward.grantPolicy.trim() : ""
        if (!grantPolicy) continue
        if (grantPolicy !== "rollDefaultAffixes") {
          throw new Error(`action.itemReward.grantPolicy invalid: ${row.id}.${itemID}.${grantPolicy}`)
        }
        const template = equipmentByID.get(itemID)
        if (!template) {
          throw new Error(`action.itemReward.grantPolicy requires equipment reward: ${row.id}.${itemID}`)
        }
        if (!Array.isArray(template.defaultAffixSlots) || template.defaultAffixSlots.length === 0) {
          throw new Error(`action.itemReward.grantPolicy requires defaultAffixSlots: ${row.id}.${itemID}`)
        }
        const weights = reward.defaultAffixRarityWeights
        if (!weights || typeof weights !== "object" || Array.isArray(weights)) {
          throw new Error(`action.itemReward.defaultAffixRarityWeights invalid: ${row.id}.${itemID}`)
        }
        const normalizedWeights = Object.entries(weights).filter(([rarityID, weight]) => rarityID && typeof weight === "number" && Number.isFinite(weight) && weight > 0)
        if (normalizedWeights.length === 0) {
          throw new Error(`action.itemReward.defaultAffixRarityWeights empty: ${row.id}.${itemID}`)
        }
        for (const [rarityID] of normalizedWeights) {
          if (knownRarityIDs.size > 0 && !knownRarityIDs.has(rarityID)) {
            throw new Error(`action.itemReward.defaultAffixRarityWeights rarity not found: ${row.id}.${itemID}.${rarityID}`)
          }
        }
        for (const slot of template.defaultAffixSlots) {
          const affixID = typeof slot?.affixId === "string" ? slot.affixId.trim() : ""
          const affix = affixByID.get(affixID)
          if (!affix) throw new Error(`action.itemReward.grantPolicy affix not found: ${row.id}.${itemID}.${affixID}`)
          const allowed = Array.isArray(affix.allowedRarities)
            ? affix.allowedRarities.map((entry) => String(entry || "").trim()).filter(Boolean)
            : []
          const unsupportedRarity = allowed.length > 0
            ? normalizedWeights.find(([rarityID]) => !allowed.includes(rarityID))?.[0]
            : ""
          if (unsupportedRarity) {
            throw new Error(`action.itemReward.grantPolicy rarity not supported by affix: ${row.id}.${itemID}.${affixID}.${unsupportedRarity}`)
          }
          const hasCompatibleRarity = normalizedWeights.some(([rarityID]) => allowed.length === 0 || allowed.includes(rarityID))
          if (!hasCompatibleRarity) {
            throw new Error(`action.itemReward.grantPolicy no compatible rarity: ${row.id}.${itemID}.${affixID}`)
          }
        }
      }
    }
    if (row.mainOutput !== undefined) {
      if (typeof row.mainOutput !== "string" || row.mainOutput !== row.mainOutput.trim()) {
        throw new Error(`action.mainOutput invalid: ${row.id}`)
      }
      if (row.mainOutput && !Object.prototype.hasOwnProperty.call(row.itemReward || {}, row.mainOutput)) {
        throw new Error(`action.mainOutput must reference itemReward: ${row.id}.${row.mainOutput}`)
      }
    }
    const feedOutputs = Object.keys(row.itemReward || {}).filter(id => itemByID.get(id)?.pondFeed)
    if (feedOutputs.length && (feedOutputs.length !== 1 || feedOutputs[0] !== row.mainOutput)) throw new Error(`pond feed must be the sole feed output and mainOutput: ${row.id}`)
    const mainOutputItem = itemByID.get(String(row.mainOutput || "").trim())
    if (mainOutputItem?.pondFeed) {
      if (Object.prototype.hasOwnProperty.call(row.buildingRequirement || {}, "fish_pond")) {
        throw new Error(`pond feed action must not require fish_pond: ${row.id}`)
      }
      const reward = row.itemReward?.[row.mainOutput]
      const outputCount = typeof reward === "number" ? reward : Number(reward?.count)
      if (!Number.isInteger(outputCount) || outputCount <= 0) throw new Error(`pond feed output count invalid: ${row.id}`)
      const ingredients = Object.entries(row.costs || {})
      if (ingredients.length === 0) throw new Error(`pond feed recipe must consume ingredients: ${row.id}`)
      const pondFeedInputs = []
      let inputValue = 0
      let hasLootInput = false
      let hasPlayerOutput = false
      for (const [itemID, quantity] of ingredients) {
        if (!Number.isInteger(quantity) || quantity < outputCount) {
          throw new Error(`pond feed output exceeds ingredient quantity: ${row.id}.${itemID}`)
        }
        const ingredient = itemByID.get(itemID)
        if (ingredient?.pondFeed) pondFeedInputs.push(itemID)
        const isLoot = (ingredient?.type || []).includes("loot") || ingredient?.category === "loot"
        if (!ingredient?.pondFeed && !isLoot && playerOutputs.has(itemID)) hasPlayerOutput = true
        if (isLoot) hasLootInput = true
        inputValue += quantity * Number(ingredient?.economyBaseValue || 0)
      }
      if (pondFeedInputs.length > 1) throw new Error(`pond feed recipe uses multiple finished feeds: ${row.id}.${pondFeedInputs.join(",")}`)
      const outputValue = outputCount * Number(mainOutputItem.economyBaseValue || 0)
      if (outputValue <= 0 || inputValue < outputValue * 1.2) throw new Error(`pond feed recipe input value is too low: ${row.id}`)
      if (mainOutputItem.pondFeed.rulesetVersion === "pond-feed-v2" && (!hasLootInput || !hasPlayerOutput)) throw new Error(`pond feed v2 recipe requires loot and a player-produced ingredient: ${row.id}`)
    }
  }
}

function renderGo(genericTables) {
  return `package generated

type Item struct {
	ID            string         \`json:"id"\`
	Type          []string       \`json:"type"\`
	Rarity        string         \`json:"rarity"\`
	SkillForgeEligible bool      \`json:"skillForgeEligible"\`
	Category      string         \`json:"category"\`
	IsEquipment   bool           \`json:"isEquipment"\`
	Disabled      bool           \`json:"disabled"\`
	BaseAttrs     map[string]float64 \`json:"baseAttrs"\`
	Stackable     bool           \`json:"stackable"\`
	CarryableInCombat bool       \`json:"carryableInCombat"\`
	MarketListable bool          \`json:"marketListable"\`
	MarketPolicy map[string]interface{} \`json:"marketPolicy,omitempty"\`
	IconID        string         \`json:"iconId,omitempty"\`
	EmojiIcon     string         \`json:"emojiIcon,omitempty"\`
	NameKey       string         \`json:"nameKey,omitempty"\`
	DescKey       string         \`json:"descKey,omitempty"\`
	EffectDescKey string         \`json:"effectDescKey,omitempty"\`
	EconomyBaseValue float64     \`json:"economyBaseValue,omitempty"\`
	UseGrantSkillID string       \`json:"useGrantSkillId,omitempty"\`
	UseGrantCollectibleID string \`json:"useGrantCollectibleId,omitempty"\`
	UseOperationEffectID string  \`json:"useOperationEffectId,omitempty"\`
	BattleUsableItemID string    \`json:"battleUsableItemId,omitempty"\`
	UseSkillExpGain int64        \`json:"useSkillExpGain,omitempty"\`
	KittyFeed map[string]int64   \`json:"kittyFeed,omitempty"\`
	PondFeed *PondFeedConfig    \`json:"pondFeed,omitempty"\`
	Alchemy *ItemAlchemyMeta    \`json:"alchemy,omitempty"\`
}

type PondFeedConfig struct {
	Delivery string \`json:"delivery,omitempty"\`
	Role string \`json:"role,omitempty"\`
	MaxPortionsPerFishHour float64 \`json:"maxPortionsPerFishHour,omitempty"\`
		Nutrition float64 \`json:"nutrition"\`
		FeedClass string \`json:"feedClass"\`
		RulesetVersion string \`json:"rulesetVersion,omitempty"\`
		ConsumePriority int \`json:"consumePriority,omitempty"\`
		Conditions []PondFeedCondition \`json:"conditions,omitempty"\`
		Effects []PondFeedEffect \`json:"effects,omitempty"\`
	}

	type PondFeedCondition struct {
		Field string \`json:"field"\`
		Op string \`json:"op"\`
		Value any \`json:"value"\`
	}

	type PondFeedEffect struct {
		Kind string \`json:"kind"\`
		Stat string \`json:"stat"\`
		Mode string \`json:"mode"\`
		ValuePerPortion float64 \`json:"valuePerPortion"\`
		MaxTargetMultiplier float64 \`json:"maxTargetMultiplier,omitempty"\`
		Floor *float64 \`json:"floor,omitempty"\`
		Ceiling *float64 \`json:"ceiling,omitempty"\`
	}

type ItemAlchemyMeta struct {
	NameAffix AlchemyNameAffix \`json:"nameAffix"\`
	ConvertExcluded bool \`json:"convertExcluded,omitempty"\`
	ConvertWeight float64 \`json:"convertWeight,omitempty"\`
	ConvertWeightMultiplier float64 \`json:"convertWeightMultiplier,omitempty"\`
}

type AlchemyNameAffix struct {
	PrefixKey string \`json:"prefixKey"\`
	SuffixKey string \`json:"suffixKey"\`
}

type Icon struct {
	ID   string \`json:"id"\`
	Path string \`json:"path"\`
}

type ActionDefinition struct {
	ID string \`json:"id"\`
	MainType string \`json:"mainType"\`
	SubType string \`json:"subType"\`
	Menu string \`json:"menu"\`
	Disabled bool \`json:"disabled"\`
	Rarity string \`json:"rarity"\`
	IconID string \`json:"iconId,omitempty"\`
	NameKey string \`json:"nameKey"\`
	DescKey string \`json:"descKey,omitempty"\`
	Queueable bool \`json:"queueable"\`
	DurationS int64 \`json:"durationS"\`
	Costs map[string]int64 \`json:"costs"\`
	MaterialInstanceRequirements map[string]ActionMaterialInstanceRequirement \`json:"materialInstanceRequirements,omitempty"\`
	StatusRequirement map[string]int64 \`json:"statusRequirement,omitempty"\`
	BuildingRequirement map[string]int64 \`json:"buildingRequirement,omitempty"\`
	ItemReward map[string]any \`json:"itemReward"\`
  MainOutput string \`json:"mainOutput"\`
	StatusExpReward map[string]float64 \`json:"statusExpReward"\`
	EconomyIndustryTier float64 \`json:"economyIndustryTier,omitempty"\`
	EconomyDifficultyMultiplier float64 \`json:"economyDifficultyMultiplier,omitempty"\`
}

type ActionMaterialInstanceRequirement struct {
	EnhanceLevel int64 \`json:"enhanceLevel"\`
	RequireTemplateAffixes bool \`json:"requireTemplateAffixes"\`
	Durability string \`json:"durability"\`
}

type BuildingDefinition struct {
	ID string \`json:"id"\`
	IconID string \`json:"iconId"\`
	NameKey string \`json:"nameKey"\`
	DescKey string \`json:"descKey,omitempty"\`
	Order int64 \`json:"order"\`
	MaxLevel int64 \`json:"maxLevel"\`
	BaseDurationS int64 \`json:"baseDurationS"\`
	StepDurationS int64 \`json:"stepDurationS"\`
	BaseCosts map[string]int64 \`json:"baseCosts"\`
	StepCosts map[string]int64 \`json:"stepCosts"\`
	BaseStatusRequirement map[string]int64 \`json:"baseStatusRequirement"\`
	StepStatusRequirement map[string]int64 \`json:"stepStatusRequirement"\`
	Levels []BuildingLevelDefinition \`json:"levels,omitempty"\`
}

type BuildingLevelDefinition struct {
	Level int64 \`json:"level"\`
	DurationS int64 \`json:"durationS"\`
	Costs map[string]int64 \`json:"costs"\`
	StatusRequirement map[string]int64 \`json:"statusRequirement"\`
	Bonuses map[string]int64 \`json:"bonuses"\`
}

type ActionMainTypeDefinition struct {
	ID string \`json:"id"\`
	NameKey string \`json:"nameKey"\`
	DescKey string \`json:"descKey,omitempty"\`
	Icon string \`json:"icon"\`
	Color string \`json:"color"\`
	Order int64 \`json:"order"\`
	Actions []string \`json:"actions"\`
}

type ActionSubTypeDefinition struct {
	ID string \`json:"id"\`
	NameKey string \`json:"nameKey"\`
	DescKey string \`json:"descKey,omitempty"\`
	Icon string \`json:"icon"\`
	Color string \`json:"color"\`
	Order int64 \`json:"order"\`
	Actions []string \`json:"actions"\`
}

type Catalog struct {
	Version int    \`json:"version"\`
	Items   []Item \`json:"items"\`
	Icons   []Icon \`json:"icons"\`
	Actions []ActionDefinition \`json:"actions"\`
	ActionMainTypes []ActionMainTypeDefinition \`json:"action_main_types"\`
	ActionSubTypes []ActionSubTypeDefinition \`json:"action_sub_types"\`
	Buildings []BuildingDefinition \`json:"buildings"\`
${genericTables.map((t) => `\t${toPascalCase(t)} []map[string]any \`json:"${t}"\``).join("\n")}
}
`
}

function renderTs(genericTables) {
  return `export interface Item {
  id: string
  type: string[]
  rarity: string
  skillForgeEligible: boolean
  category: string
  isEquipment: boolean
    disabled: boolean
    baseAttrs: Record<string, number>
  stackable: boolean
  carryableInCombat: boolean
  marketListable: boolean
  marketPolicy?: { fixedPrice: { mode: "instance_variants"; profileId: string }; buyOrders: false; auction: boolean }
  iconId?: string
  emojiIcon?: string
  nameKey?: string
  descKey?: string
  effectDescKey?: string
  economyBaseValue?: number
  useGrantSkillId?: string
  useGrantCollectibleId?: string
  useOperationEffectId?: string
  battleUsableItemId?: string
  useSkillExpGain?: number
  kittyFeed?: {
    battleLevelCapGain: number
  }
	  pondFeed?: {
	    nutrition: number
	    feedClass: string
	    rulesetVersion?: string
	    consumePriority?: number
	    role?: "staple" | "functional"
	    maxPortionsPerFishHour?: number
	    delivery?: "feeder" | "targeted"
	    conditions?: Array<{
	      field: string
	      op: string
	      value: string | number
	    }>
	    effects?: Array<{
	      kind: string
	      stat: string
	      mode: string
	      valuePerPortion: number
	      maxTargetMultiplier?: number
	      floor?: number
	      ceiling?: number
	    }>
	  }
  alchemy?: {
    convertExcluded?: boolean
    convertWeight?: number
    convertWeightMultiplier?: number
    nameAffix?: {
      prefixKey: string
      suffixKey: string
    }
  }
}

export interface Icon {
  id: string
  path: string
}

export interface ActionDefinition {
  id: string
  mainType: string
  subType: string
  menu: string
  disabled: boolean
  rarity: string
  iconId?: string
  nameKey: string
  descKey?: string
  queueable: boolean
  durationS: number
  costs: Record<string, number>
  materialInstanceRequirements?: Record<string, ActionMaterialInstanceRequirement>
  statusRequirement?: Record<string, number>
  buildingRequirement?: Record<string, number>
  itemReward: Record<string, number | ActionItemRewardRule>
  mainOutput?: string
  statusExpReward: Record<string, number>
  economyIndustryTier?: number
  economyDifficultyMultiplier?: number
}

export interface ActionMaterialInstanceRequirement {
  enhanceLevel: number
  requireTemplateAffixes: boolean
  durability: "full"
}

export interface BuildingDefinition {
  id: string
  iconId: string
  nameKey: string
  descKey?: string
  order: number
  maxLevel: number
  baseDurationS: number
  stepDurationS: number
  baseCosts: Record<string, number>
  stepCosts: Record<string, number>
  baseStatusRequirement: Record<string, number>
  stepStatusRequirement: Record<string, number>
  levels?: BuildingLevelDefinition[]
}

export interface BuildingLevelDefinition {
  level: number
  durationS: number
  costs: Record<string, number>
  statusRequirement: Record<string, number>
  bonuses: Record<string, number>
}

export interface ActionItemRewardRule {
  count?: number
  range?: {
    min: number
    max: number
  }
  percent?: number
  grantPolicy?: "rollDefaultAffixes"
  defaultAffixRarityWeights?: Record<string, number>
}

export interface ActionMainTypeDefinition {
  id: string
  nameKey: string
  descKey?: string
  icon: string
  color: string
  order: number
  actions: string[]
}

export interface ActionSubTypeDefinition {
  id: string
  nameKey: string
  descKey?: string
  icon: string
  color: string
  order: number
  actions: string[]
}

export interface BattlefieldEffectStateDefinition {
  id: string
  nameKey: string
  descKey: string
  icon?: string
  displayMode?: "countdown" | "progress"
  counter: {
    source: "turn_consumed"
    actorFilter: {
      sides: Array<"player" | "enemy">
      unitTypes: Array<"player" | "kitty" | "enemy" | "summon">
      includeSummons: boolean
      turnKinds: Array<"ordered" | "extra">
      includePreparation: boolean
    }
    aggregation: "per_event" | "alive_roster_cycle"
    countSuppressedTurns: boolean
    initial: number
    resetTo: number
    carryProgressAcrossWaves: boolean
  }
  triggerEffects: Array<
    | {
        kind: "damage"
        targetScope: "all_alive_players"
        includeSummons: boolean
        damageType: "physical" | "magical"
        amountByDifficulty: { base: number; perLevelMult: number }
      }
    | {
        kind: "weighted_random_per_target"
        targetScope: "alive_players"
        targetFilter: { unitTypes: Array<"player" | "kitty" | "summon">; includeSummons: boolean }
        targetCount: { source: "eligible_count"; divisor: number; rounding: "ceil" | "floor" | "round"; min: number; max: number }
        selection: "uniform_without_replacement"
        choices: Array<{
          id: string
          weight: number
          effect:
            | { kind: "status"; statusId: string; durationTurns: number }
            | { kind: "adjust_skill_cooldown"; amount: number; includeBasic: boolean; excludeSkillIds?: string[] }
        }>
      }
  >
  postTriggerWindows?: Array<{ id: string; duration: number; unit: "player_round" }>
  conditionalModifiers?: Array<{
    kind: "incoming_damage_reduction"
    targetScope: "special_enemies"
    value: number
    inactiveDuringWindow: string
  }>
}

export interface WorldDefinition extends Record<string, unknown> {
  id: string
  nameKey: string
  descKey: string
  battlefieldEffects?: BattlefieldEffectStateDefinition[]
}

export interface Catalog {
  version: number
  items: Item[]
  icons: Icon[]
  actions: ActionDefinition[]
  action_main_types: ActionMainTypeDefinition[]
  action_sub_types: ActionSubTypeDefinition[]
  buildings: BuildingDefinition[]
${genericTables.map((t) => t === "world" ? "  world: WorldDefinition[]" : `  ${t}: Record<string, unknown>[]`).join("\n")}
  [key: string]: unknown
}
`
}

function extractLocaleManagedObject(content, filePath) {
  const startIdx = content.indexOf(LOCALE_SYNC_MARKER_START)
  const endIdx = content.indexOf(LOCALE_SYNC_MARKER_END)
  if (startIdx < 0 || endIdx < 0 || endIdx <= startIdx) {
    throw new Error(`locale marker block missing in ${filePath}`)
  }
  const raw = content
    .slice(startIdx + LOCALE_SYNC_MARKER_START.length, endIdx)
    .trim()
  if (!raw) return {}
  const wrapped = `({\n${raw}\n})`
  const parsed = vm.runInNewContext(wrapped)
  if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) {
    throw new Error(`locale marker block invalid in ${filePath}`)
  }
  return parsed
}

function localeSectionID(row, section) {
  const nameKey = String(row?.nameKey || "").trim()
  const prefix = `${section}.`
  const suffix = ".name"
  if (nameKey.startsWith(prefix) && nameKey.endsWith(suffix)) {
    return nameKey.slice(prefix.length, -suffix.length)
  }
  return String(row?.id || "").trim()
}

function buildLocaleSectionObjects(items, actions, actionMainTypes, actionSubTypes, buildings, itemTypes, attributes, rarityDefinitions, affixDefinitions, enemies, world, skills, skillTypes, weaponClasses, equipmentSecondaryClasses, buffs, buffTypes, achievementCategories, achievements, collectibles, collectibleCategories, specializationTrees, localeTag, existing) {
  const existingSkillNameByID = {}
  for (const [skillID, row] of Object.entries(existing?.skill || {})) {
    if (!row || typeof row !== "object") continue
    if (typeof row.name !== "string") continue
    const name = row.name.trim()
    if (!name) continue
    existingSkillNameByID[skillID] = name
  }

  const inferSkillbookSkillID = (itemRow) => {
    const explicit = parseOptionalString(itemRow?.useGrantSkillId)
    if (explicit) return explicit
    const itemID = parseOptionalString(itemRow?.id)
    if (!itemID || !itemID.startsWith("skillbook_")) return ""
    return itemID.slice("skillbook_".length)
  }

  const defaultSkillbookText = (itemRow) => {
    if (localeTag !== "zh-CN") return null
    const skillID = inferSkillbookSkillID(itemRow)
    if (!skillID) return null
    const skillName = existingSkillNameByID[skillID]
    if (!skillName) return null
    return {
      name: `${skillName}技能书`,
      desc: `使用后获得技能${skillName}，或者获得对应技能100点经验`
    }
  }

  const item = {}
  for (const row of items) {
    if (!row?.id) continue
    const previous = existing?.item?.[row.id] || {}
    const skillbookText = defaultSkillbookText(row)
    const previousName = typeof previous.name === "string" ? previous.name : ""
    const previousDesc = typeof previous.desc === "string" ? previous.desc : ""
    item[row.id] = {
      name: previousName || skillbookText?.name || "",
      desc: previousDesc || skillbookText?.desc || "",
      effect: typeof previous.effect === "string" ? previous.effect : ""
    }
  }

  const action = {}
  for (const row of actions) {
    if (!row?.id) continue
    const previous = existing?.action?.[row.id] || {}
    action[row.id] = {
      name: typeof previous.name === "string" ? previous.name : "",
      desc: typeof previous.desc === "string" ? previous.desc : ""
    }
  }

  const action_main_type = {}
  for (const row of actionMainTypes) {
    if (!row?.id) continue
    const previous = existing?.action_main_type?.[row.id] || {}
    action_main_type[row.id] = {
      name: typeof previous.name === "string" ? previous.name : "",
      desc: typeof previous.desc === "string" ? previous.desc : ""
    }
  }
  for (const [id, prev] of Object.entries(existing?.action_main_type || {})) {
    if (action_main_type[id] || !prev || typeof prev !== "object") continue
    action_main_type[id] = {
      name: typeof prev.name === "string" ? prev.name : "",
      desc: typeof prev.desc === "string" ? prev.desc : ""
    }
  }

  const action_sub_type = {}
  for (const row of actionSubTypes) {
    if (!row?.id) continue
    const previous = existing?.action_sub_type?.[row.id] || {}
    action_sub_type[row.id] = {
      name: typeof previous.name === "string" ? previous.name : "",
      desc: typeof previous.desc === "string" ? previous.desc : ""
    }
  }

  const building = {}
  for (const row of buildings || []) {
    if (!row?.id) continue
    const previous = existing?.building?.[row.id] || {}
    building[row.id] = {
      name: typeof previous.name === "string" ? previous.name : "",
      desc: typeof previous.desc === "string" ? previous.desc : ""
    }
  }

  const item_type = {}
  for (const row of itemTypes || []) {
    if (!row?.id) continue
    const previous = existing?.item_type?.[row.id] || {}
    item_type[row.id] = {
      name: typeof previous.name === "string" && previous.name ? previous.name : row.id,
      desc: typeof previous.desc === "string" ? previous.desc : ""
    }
  }

  const attr = {}
  for (const row of attributes || []) {
    if (!row?.id) continue
    const previous = existing?.attr?.[row.id] || {}
    attr[row.id] = {
      name: typeof previous.name === "string" && previous.name ? previous.name : row.id,
      desc: typeof previous.desc === "string" ? previous.desc : ""
    }
  }

  const rarity = {}
  for (const row of rarityDefinitions || []) {
    if (!row?.id) continue
    const previous = existing?.rarity?.[row.id] || {}
    rarity[row.id] = {
      name: typeof previous.name === "string" && previous.name ? previous.name : row.id
    }
  }

  const affix = {}
  for (const row of affixDefinitions || []) {
    if (!row?.id) continue
    const previous = existing?.affix?.[row.id] || {}
    affix[row.id] = {
      name: typeof previous.name === "string" ? previous.name : "",
      desc: typeof previous.desc === "string" ? previous.desc : ""
    }
  }

  const enemy = {}
  for (const row of enemies || []) {
    if (!row?.id) continue
    const previous = existing?.enemy?.[row.id] || {}
    enemy[row.id] = {
      name: typeof previous.name === "string" ? previous.name : "",
      desc: typeof previous.desc === "string" ? previous.desc : ""
    }
  }

  const worldSection = {}
  for (const row of world || []) {
    if (!row?.id) continue
    const previous = existing?.world?.[row.id] || {}
    const waveSchedule = {}
    for (const schedule of row.waveSchedules || []) {
      if (!schedule?.id) continue
      const previousSchedule = previous?.waveSchedule?.[schedule.id] || {}
      waveSchedule[schedule.id] = {
        name: typeof previousSchedule.name === "string" ? previousSchedule.name : "",
        desc: typeof previousSchedule.desc === "string" ? previousSchedule.desc : ""
      }
    }
    const customLineup = {}
    for (const conf of row.customLineupConf || []) {
      if (!conf?.id) continue
      const prevLineup = previous?.customLineup?.[conf.id] || {}
      customLineup[conf.id] = {
        name: typeof prevLineup.name === "string" ? prevLineup.name : "",
        desc: typeof prevLineup.desc === "string" ? prevLineup.desc : ""
      }
    }
    worldSection[row.id] = {
      name: typeof previous.name === "string" ? previous.name : "",
      desc: typeof previous.desc === "string" ? previous.desc : "",
      waveSchedule,
      customLineup
    }
  }

  const battlefield_effect = {}
  for (const row of world || []) {
    for (const effect of Array.isArray(row?.battlefieldEffects) ? row.battlefieldEffects : []) {
      const id = String(effect?.id || "").trim()
      if (!id) continue
      const previous = existing?.battlefield_effect?.[id] || {}
      const outcomes = {}
      for (const triggerEffect of Array.isArray(effect?.triggerEffects) ? effect.triggerEffects : []) {
        for (const choice of Array.isArray(triggerEffect?.choices) ? triggerEffect.choices : []) {
          const choiceID = String(choice?.id || "").trim()
          if (choiceID) outcomes[choiceID] = typeof previous?.outcomes?.[choiceID] === "string" ? previous.outcomes[choiceID] : ""
        }
      }
      battlefield_effect[id] = {
        name: typeof previous.name === "string" ? previous.name : "",
        desc: typeof previous.desc === "string" ? previous.desc : "",
        outcomes
      }
    }
  }

  const skill = {}
  for (const row of skills || []) {
    if (!row?.id) continue
    const previous = existing?.skill?.[row.id] || {}
    skill[row.id] = {
      name: typeof previous.name === "string" ? previous.name : "",
      desc: typeof previous.desc === "string" ? previous.desc : ""
    }
  }

  const skill_type = {}
  for (const row of skillTypes || []) {
    if (!row?.id) continue
    const previous = existing?.skill_type?.[row.id] || {}
    skill_type[row.id] = {
      name: typeof previous.name === "string" ? previous.name : "",
      desc: typeof previous.desc === "string" ? previous.desc : ""
    }
  }

  const weapon_class = {}
  for (const row of weaponClasses || []) {
    if (!row?.id) continue
    const previous = existing?.weapon_class?.[row.id] || {}
    weapon_class[row.id] = {
      name: typeof previous.name === "string" ? previous.name : "",
      desc: typeof previous.desc === "string" ? previous.desc : ""
    }
  }

  const equipment_secondary_class = {}
  for (const row of equipmentSecondaryClasses || []) {
    if (!row?.id) continue
    const previous = existing?.equipment_secondary_class?.[row.id] || {}
    const fallback = localeTag === "zh-CN" ? row?.zhCN : row?.enUS
    equipment_secondary_class[row.id] = {
      name: typeof previous.name === "string" && previous.name
        ? previous.name
        : (typeof fallback?.name === "string" ? fallback.name : ""),
      desc: typeof previous.desc === "string" && previous.desc
        ? previous.desc
        : (typeof fallback?.desc === "string" ? fallback.desc : "")
    }
  }

  const buff = {}
  for (const row of buffs || []) {
    if (!row?.id) continue
    const previous = existing?.buff?.[row.id] || {}
    buff[row.id] = {
      name: typeof previous.name === "string" ? previous.name : "",
      desc: typeof previous.desc === "string" ? previous.desc : ""
    }
  }

  const buff_type = {}
  for (const row of buffTypes || []) {
    if (!row?.id) continue
    const previous = existing?.buff_type?.[row.id] || {}
    buff_type[row.id] = {
      name: typeof previous.name === "string" ? previous.name : "",
      desc: typeof previous.desc === "string" ? previous.desc : ""
    }
  }

  const achievement_category = {}
  for (const row of achievementCategories || []) {
    if (!row?.id) continue
    const previous = existing?.achievement_category?.[row.id] || {}
    const localizedName = row?.names?.[localeTag] || row?.names?.["en-US"] || ""
    achievement_category[row.id] = {
      name: typeof previous.name === "string" && previous.name ? previous.name : localizedName
    }
  }

  const achievement = {}
  for (const row of achievements || []) {
    if (!row?.id) continue
    const stageRows = Array.isArray(row.stages) && row.stages.length > 0
      ? row.stages.map((stage, index) => ({ ...stage, id: `${row.id}__stage_${index + 1}` }))
      : [row]
    for (const stageRow of stageRows) {
      const hasNameKey = String(stageRow.nameKey || "").trim() !== ""
      const hasDescKey = String(stageRow.descKey || "").trim() !== ""
      if (!hasNameKey && !hasDescKey) continue
      const previous = existing?.achievement?.[stageRow.id] || {}
      const entry = {}
      if (hasNameKey) {
        entry.name = typeof previous.name === "string" ? previous.name : ""
      }
      if (hasDescKey) {
        entry.desc = typeof previous.desc === "string" ? previous.desc : ""
      }
      achievement[stageRow.id] = entry
    }
  }

  const collectible = {}
  for (const row of collectibles || []) {
    const id = localeSectionID(row, "collectible")
    if (!id) continue
    const previous = existing?.collectible?.[id] || {}
    const itemLocale = row.itemId ? item[row.itemId] || {} : {}
    const localizedName = row?.names?.[localeTag] || row?.names?.["en-US"] || ""
    const localizedDesc = row?.descs?.[localeTag] || row?.descs?.["en-US"] || ""
    const localizedTitle = row?.titleNames?.[localeTag] || row?.titleNames?.["en-US"] || ""
    collectible[id] = {
      name: typeof previous.name === "string" && previous.name
        ? previous.name
        : (localizedName || (typeof itemLocale.name === "string" ? itemLocale.name : "")),
      desc: typeof previous.desc === "string" && previous.desc
        ? previous.desc
        : (localizedDesc || (typeof itemLocale.desc === "string" ? itemLocale.desc : "")),
      ...(typeof previous.title === "string" && previous.title
        ? { title: previous.title }
        : (localizedTitle ? { title: localizedTitle } : {}))
    }
  }

  const collectibleCategoryFallbacks = {
    "zh-CN": {
      plains: "平原",
      forest: "森林",
      cave: "洞窟",
      ruins: "遗迹",
      dragon: "巨龙",
      snowfield: "雪原",
      shadow: "暗影",
      holy_temple: "圣殿",
      shadow_paw: "影爪",
      special: "特殊"
    },
    "en-US": {
      plains: "Plains",
      forest: "Forest",
      cave: "Cave",
      ruins: "Ruins",
      dragon: "Dragon",
      snowfield: "Snowfield",
      shadow: "Shadow",
      holy_temple: "Holy Temple",
      shadow_paw: "Shadow Paw",
      special: "Special"
    }
  }

  const collectible_category = {}
  for (const row of collectibleCategories || []) {
    const id = localeSectionID(row, "collectible_category")
    if (!id) continue
    const previous = existing?.collectible_category?.[id] || {}
    const fallback = collectibleCategoryFallbacks[localeTag]?.[id]
    const localizedName = row?.names?.[localeTag] || row?.names?.["en-US"] || ""
    collectible_category[id] = {
      name: typeof previous.name === "string" && previous.name ? previous.name : (localizedName || fallback || row.id)
    }
  }

  const specialization = { tree: {}, school: {}, node: {} }
  for (const tree of specializationTrees || []) {
    if (!tree?.id) continue
    const source = tree.i18n?.[localeTag]
    if (!source || typeof source !== "object") continue
    specialization.tree[tree.id] = { name: source.name, desc: source.desc }
    for (const school of tree.schools || []) {
      const row = source.schools?.[school.id]
      if (row) specialization.school[school.id] = { name: row.name, desc: row.desc }
    }
    for (const node of tree.nodes || []) {
      const row = source.nodes?.[node.id]
      if (row) specialization.node[node.id] = { name: row.name, desc: row.desc }
    }
  }

  return { item, action, action_main_type, action_sub_type, building, item_type, attr, rarity, affix, enemy, world: worldSection, battlefield_effect, skill, skill_type, weapon_class, equipment_secondary_class, buff, buff_type, achievement_category, achievement, collectible, collectible_category, specialization }
}

function renderLocaleManagedBlock(sections) {
  const entries = Object.entries(sections).map(([key, value]) => {
    const serialized = JSON.stringify(value, null, 2).replace(/\n/g, "\n  ")
    return `  ${key}: ${serialized},`
  })
  return [`  ${LOCALE_SYNC_MARKER_START}`, ...entries, `  ${LOCALE_SYNC_MARKER_END}`].join("\n")
}

function syncLocaleBaseline(filePath, localeTag, items, actions, actionMainTypes, actionSubTypes, buildings, itemTypes, attributes, rarityDefinitions, affixDefinitions, enemies, world, skills, skillTypes, weaponClasses, equipmentSecondaryClasses, buffs, buffTypes, achievementCategories, achievements, collectibles, collectibleCategories, specializationTrees) {
  if (!fs.existsSync(filePath)) return
  const content = fs.readFileSync(filePath, "utf8")
  const existing = extractLocaleManagedObject(content, filePath)
  const sections = buildLocaleSectionObjects(items, actions, actionMainTypes, actionSubTypes, buildings, itemTypes, attributes, rarityDefinitions, affixDefinitions, enemies, world, skills, skillTypes, weaponClasses, equipmentSecondaryClasses, buffs, buffTypes, achievementCategories, achievements, collectibles, collectibleCategories, specializationTrees, localeTag, existing)
  const block = renderLocaleManagedBlock(sections)

  const startIdx = content.indexOf(LOCALE_SYNC_MARKER_START)
  const endIdx = content.indexOf(LOCALE_SYNC_MARKER_END)
  const endLineIdx = content.indexOf("\n", endIdx)
  const head = content.slice(0, startIdx).trimEnd()
  const tail = endLineIdx >= 0 ? content.slice(endLineIdx + 1).trimStart() : ""
  const merged = `${head}\n${block}\n${tail}`
  fs.writeFileSync(filePath, merged)
}

function validateBattlefieldEffectLocaleCoverage(filePath, world) {
  if (!fs.existsSync(filePath)) throw new Error(`battlefield effect locale missing: ${filePath}`)
  const content = fs.readFileSync(filePath, "utf8")
  const locale = extractLocaleManagedObject(content, filePath)
  for (const row of world || []) {
    for (const effect of Array.isArray(row?.battlefieldEffects) ? row.battlefieldEffects : []) {
      const id = String(effect?.id || "").trim()
      if (!id) continue
      const entry = locale?.battlefield_effect?.[id]
      if (!String(entry?.name || "").trim() || !String(entry?.desc || "").trim()) {
        throw new Error(`battlefield effect i18n missing: ${path.basename(filePath)}.${id}`)
      }
      for (const triggerEffect of Array.isArray(effect?.triggerEffects) ? effect.triggerEffects : []) {
        for (const choice of Array.isArray(triggerEffect?.choices) ? triggerEffect.choices : []) {
          const choiceID = String(choice?.id || "").trim()
          if (choiceID && !String(entry?.outcomes?.[choiceID] || "").trim()) throw new Error(`battlefield effect outcome i18n missing: ${path.basename(filePath)}.${id}.${choiceID}`)
        }
      }
    }
  }
}

function validateWorldScheduleLocaleCoverage(filePath, world) {
  if (!fs.existsSync(filePath)) throw new Error(`world schedule locale missing: ${filePath}`)
  const locale = extractLocaleManagedObject(fs.readFileSync(filePath, "utf8"), filePath)
  for (const row of world || []) {
    for (const schedule of Array.isArray(row?.waveSchedules) ? row.waveSchedules : []) {
      const worldID = String(row?.id || "").trim()
      const scheduleID = String(schedule?.id || "").trim()
      if (!worldID || !scheduleID) continue
      const entry = locale?.world?.[worldID]?.waveSchedule?.[scheduleID]
      if (!String(entry?.name || "").trim() || !String(entry?.desc || "").trim()) {
        throw new Error(`world schedule i18n missing: ${path.basename(filePath)}.${worldID}.${scheduleID}`)
      }
    }
  }
}

function validateSpecializationLocaleCoverage(filePath, specializationTrees) {
  if (!fs.existsSync(filePath)) throw new Error(`specialization locale missing: ${filePath}`)
  const locale = extractLocaleManagedObject(fs.readFileSync(filePath, "utf8"), filePath)?.specialization
  for (const tree of specializationTrees || []) {
    const treeEntry = locale?.tree?.[tree.id]
    if (!String(treeEntry?.name || "").trim() || !String(treeEntry?.desc || "").trim()) {
      throw new Error(`specialization tree i18n missing: ${path.basename(filePath)}.${tree.id}`)
    }
    for (const school of tree.schools || []) {
      const entry = locale?.school?.[school.id]
      if (!String(entry?.name || "").trim() || !String(entry?.desc || "").trim()) {
        throw new Error(`specialization school i18n missing: ${path.basename(filePath)}.${school.id}`)
      }
    }
    for (const node of tree.nodes || []) {
      const entry = locale?.node?.[node.id]
      if (!String(entry?.name || "").trim() || !String(entry?.desc || "").trim()) {
        throw new Error(`specialization node i18n missing: ${path.basename(filePath)}.${node.id}`)
      }
    }
  }
}

function main() {
  ensureDir(generatedDir)
  const version = readVersion()
  const equipmentSecondaryClasses = normalizeEquipmentSecondaryClasses(loadEquipmentSecondaryClassesFromCombatModel())
  const loadedItems = loadItems()
  // Loot remains forge-eligible by legacy convention unless the source row explicitly opts out.
  const explicitSkillForgeOptOutItemIDs = new Set(
    loadedItems
      .filter((item) => {
        const raw = item?.skillForgeEligible
        return raw !== undefined && raw !== null && raw !== "" && parseBoolean(raw, true) === false
      })
      .map((item) => String(item?.id || "").trim())
      .filter(Boolean)
  )
  const rawItems = normalizeItems(loadedItems)
  const extraTables = {}
  for (const tableName of EXTRA_TABLES) {
    extraTables[tableName] = loadOptionalTable(tableName)
  }
  validateStableBattleEffectSourceIDs(extraTables.skills, extraTables.equipment_templates)
  extraTables.equipment_templates = normalizeEquipmentTemplates(extraTables.equipment_templates)
  let items = applyEquipmentMetadataToItems(rawItems, extraTables.equipment_templates)
  const excelTables = loadExcelTables()
  const dynamicItemTables = []
  for (const [sheetName, rows] of Object.entries(excelTables)) {
    if (!Array.isArray(rows) || rows.length === 0) continue
    if (RESERVED_SHEETS.has(sheetName)) continue
    if (!sheetName.startsWith("item_")) continue
    extraTables[sheetName] = rows
    dynamicItemTables.push(sheetName)
  }
  extraTables.actions = collectActionRows(extraTables)
  extraTables.action_main_types = collectActionMainTypes(extraTables.action_main_types, extraTables.actions)
  for (const builtinType of BUILTIN_ACTION_MAIN_TYPES) {
    if (!extraTables.action_main_types.some((t) => t.id === builtinType.id)) {
      extraTables.action_main_types.push(builtinType)
    }
  }
  extraTables.action_sub_types = collectActionSubTypes(extraTables.action_sub_types, extraTables.actions)
  extraTables.buildings = normalizeBuildings(extraTables.buildings)
  extraTables.item_types = normalizeItemTypes(extraTables.item_types)
  extraTables.skills = normalizeSkills(extraTables.skills)
  extraTables.skill_type = normalizeSkillTypes(extraTables.skill_type)
  extraTables.weapon_classes = normalizeWeaponClasses(extraTables.weapon_classes)
  extraTables.buff = normalizeBuffs(extraTables.buff)
  extraTables.buff_type = normalizeBuffTypes(extraTables.buff_type)
  extraTables.buff_effects = normalizeBuffEffects(extraTables.buff_effects)
  extraTables.enemies = applyEnemyDrops(
    applyEnemyDropOverrides(normalizeEnemies(extraTables.enemies), loadEnemyDropOverridesFromExcel()),
    loadEnemyDropsFromExcel(),
    items
  )
  extraTables.world = normalizeWorld(extraTables.world)
  extraTables.affix_definitions = normalizeAffixDefinitions(extraTables.affix_definitions)
  extraTables.affix_enchant_rules = normalizeAffixEnchantRules(extraTables.affix_enchant_rules)
  extraTables.affix_carrier_rules = normalizeAffixCarrierRules(extraTables.affix_carrier_rules)
  extraTables.alchemy_groups = normalizeAlchemyGroups(extraTables.alchemy_groups)
  extraTables.item_traits = normalizeItemTraits(extraTables.item_traits)
  extraTables.skill_alchemy_modules = normalizeSkillAlchemyModules(extraTables.skill_alchemy_modules)
  extraTables.alchemy_combo_recipes = normalizeAlchemyComboRecipes(extraTables.alchemy_combo_recipes)
  extraTables.alchemy_transmute_rules = normalizeAlchemyTransmuteRules(extraTables.alchemy_transmute_rules)
  extraTables.alchemy_mediums = normalizeAlchemyMediums(extraTables.alchemy_mediums)
  extraTables.attributes = normalizeAttributes(extraTables.attributes)
  extraTables.attribute_exp_profiles = (extraTables.attribute_exp_profiles || []).map((row) => ({
    id: row.id,
    attr: row.attr,
    level: row.level,
    expRequired: row.expRequired
  }))
  extraTables.kitty_personalities = normalizeKittyPersonalities(extraTables.kitty_personalities)
  extraTables.rarity_definitions = normalizeRarityDefinitions(extraTables.rarity_definitions)
  items = mergeAutoSkillbooks(items, extraTables.skills, extraTables.rarity_definitions)
  extraTables.item_types = ensureSkillbookItemType(extraTables.item_types, items.some((item) => (item.type || []).includes("skillbook")))
  const icons = normalizeIcons(items, extraTables.actions, loadIcons())
  const iconIdSet = new Set(icons.map((icon) => icon.id))
  validateEquipmentSecondaryClasses(equipmentSecondaryClasses)
  validateEquipmentTemplates(extraTables.equipment_templates, extraTables.affix_definitions, extraTables.rarity_definitions, equipmentSecondaryClasses, extraTables.weapon_classes, extraTables.skill_type, extraTables.buff, extraTables.skills, extraTables.attributes, extraTables.enemies)
  validateEquipmentSkills(extraTables.equipment_templates, extraTables.skills)
  validateIcons(icons)
  validateItems(items, iconIdSet)
validateMarketPolicies(items, extraTables.market_profiles, extraTables)
  validateItemCollectibleReferences(items, extraTables.collectibles)
  validateEquipmentItemFlags(items, extraTables.equipment_templates)
  validateItemTypes(items, extraTables.item_types)
  validateActions(
    extraTables.actions,
    extraTables.action_main_types,
    extraTables.action_sub_types,
    iconIdSet,
    extraTables.rarity_definitions,
    extraTables.equipment_templates,
    extraTables.affix_definitions,
    items
  )
  validateCatteryFurniture(extraTables.cattery_furniture, items, extraTables.actions, extraTables.action_main_types, extraTables.action_sub_types)
  validateOperationEffects(
    extraTables.operation_effects,
    items,
    extraTables.actions,
    extraTables.action_main_types,
    extraTables.action_sub_types,
    iconIdSet
  )
  validateFishFoodRecipes(
    extraTables.fish_food_recipes,
    items,
    extraTables.actions,
    extraTables.operation_effects,
    extraTables.fishing_species
  )
  validateBattleUsableItems(extraTables.battle_usable_items, items, extraTables.enemies, extraTables.buff, extraTables.world)
  validateBattleUsableItemReferences(items, extraTables.battle_usable_items)
  validateBuildings(extraTables.buildings, iconIdSet, extraTables.attributes)
  validateSkillTables(extraTables.skills, extraTables.skill_type, extraTables.buff, extraTables.enemies)
  validateWeaponClasses(extraTables.weapon_classes)
  validateBuffTables(extraTables.buff, extraTables.buff_type, extraTables.buff_effects, extraTables.skill_type, extraTables.enemies)
  validateArenaFormats(extraTables.arena_formats, extraTables.buff)
	validateCombatTrialDefinitions(extraTables.combat_trial_metrics, extraTables.combat_trial_formats, extraTables.enemies, extraTables.buff)
  validateRarityDefinitions(extraTables.rarity_definitions)
  validateAffixDefinitions(extraTables.affix_definitions)
  validateAffixEnchantRules(extraTables.affix_enchant_rules, extraTables.equipment_templates, extraTables.weapon_classes)
  validateAlchemyTables(
    extraTables.alchemy_groups,
    extraTables.item_traits,
    extraTables.alchemy_combo_recipes,
    extraTables.alchemy_transmute_rules,
    items,
    extraTables.attributes,
    extraTables.actions,
    []
  )
  validateAlchemyMediums(extraTables.alchemy_mediums, items)
  validateSkillAlchemyModules(
    extraTables.skill_alchemy_modules,
    items,
    extraTables.rarity_definitions,
    extraTables.attributes,
    extraTables.buff,
    explicitSkillForgeOptOutItemIDs
  )
  validateSkillForgeSkillbooks(items, extraTables.skills)
  validateAttributes(extraTables.attributes, extraTables.attribute_exp_profiles)
  validateSpecializationTrees(extraTables.specialization_trees, extraTables.attributes, items, extraTables.skills, extraTables.buff, extraTables.equipment_templates)
  validateKittyPersonalities(extraTables.kitty_personalities)
  validateEnemies(
    extraTables.enemies,
    extraTables.skills,
    extraTables.equipment_templates,
    extraTables.affix_definitions,
    extraTables.rarity_definitions
  )
  validateEnemyMaterialDropTypes(items, extraTables.enemies)
  validateWorldRows(extraTables.world, extraTables.enemies, extraTables.buff, extraTables.skills)
  validateActionRewardAttributes(extraTables.actions, extraTables.attributes)
  validateStatusRequirementAttributes(extraTables.equipment_templates, extraTables.actions, extraTables.attributes)
  validateActionBuildingRequirements(extraTables.actions, extraTables.buildings)
  validateSkillStatusRequirementAttributes(extraTables.skills, extraTables.attributes)
  validateSkillDamageFormulaAttributes(extraTables.skills, extraTables.attributes)
  validateSkillTargetFormulaAttributes(extraTables.skills, extraTables.attributes)
  validateEquipmentBasicAttackFormulaAttributes(extraTables.equipment_templates, extraTables.attributes)
  validateBuffDamageFormulaAttributes(extraTables.buff_effects, extraTables.attributes)
  validateAchievements(
    extraTables.achievement_categories,
    extraTables.achievements,
    items,
    extraTables.actions,
    extraTables.enemies,
    extraTables.cat_village_commissions,
    extraTables.attributes,
    extraTables.rarity_definitions,
    extraTables.title_prefixes,
    extraTables.title_suffixes,
    extraTables.fishing_species
  )
  validateCommissionRewards(extraTables.cat_village_commissions, items)
  validateRumorCommissionDeletionRules(extraTables.cat_village_commissions)
  validateFishingTables(extraTables.fishing_waters, extraTables.fishing_spots, extraTables.fishing_species, extraTables.fishing_tackle, extraTables.fishing_infusion_profiles, items, extraTables.equipment_templates, extraTables.actions, extraTables.attributes)
  validateFishingPondSpecies(extraTables.fishing_pond_species, extraTables.fishing_species)

  const allGenericTables = [...new Set([...GENERIC_EXTRA_TABLES, ...dynamicItemTables])].sort((a, b) =>
    a.localeCompare(b)
  )
  for (const tableName of allGenericTables) {
    validateGenericTable(tableName, extraTables[tableName])
  }

  const catalog = { version, items, icons, ...extraTables }
  fs.writeFileSync(path.join(generatedDir, "game-data.json"), JSON.stringify(catalog, null, 2))
  writeSplitCatalogChunks(catalog)
  fs.writeFileSync(path.join(generatedDir, "game_data.go"), renderGo(allGenericTables))
  fs.writeFileSync(path.join(generatedDir, "game-data.ts"), renderTs(allGenericTables))
  buildIconAtlases()

  ensureDir(path.dirname(webTypePath))
  fs.writeFileSync(webTypePath, renderTs(allGenericTables))
  syncLocaleBaseline(
    zhLocalePath,
    "zh-CN",
    items,
    extraTables.actions,
    extraTables.action_main_types,
    extraTables.action_sub_types,
    extraTables.buildings,
    extraTables.item_types,
    extraTables.attributes,
    extraTables.rarity_definitions,
    extraTables.affix_definitions,
    extraTables.enemies,
    extraTables.world,
    extraTables.skills,
    extraTables.skill_type,
    extraTables.weapon_classes,
    equipmentSecondaryClasses,
    extraTables.buff,
    extraTables.buff_type,
    extraTables.achievement_categories,
    extraTables.achievements,
    extraTables.collectibles,
    extraTables.collectible_categories,
    extraTables.specialization_trees
  )
  syncLocaleBaseline(
    enLocalePath,
    "en-US",
    items,
    extraTables.actions,
    extraTables.action_main_types,
    extraTables.action_sub_types,
    extraTables.buildings,
    extraTables.item_types,
    extraTables.attributes,
    extraTables.rarity_definitions,
    extraTables.affix_definitions,
    extraTables.enemies,
    extraTables.world,
    extraTables.skills,
    extraTables.skill_type,
    extraTables.weapon_classes,
    equipmentSecondaryClasses,
    extraTables.buff,
    extraTables.buff_type,
    extraTables.achievement_categories,
    extraTables.achievements,
    extraTables.collectibles,
    extraTables.collectible_categories,
    extraTables.specialization_trees
  )
  validateFishingLocaleCoverage()
  validateBattlefieldEffectLocaleCoverage(zhLocalePath, extraTables.world)
  validateBattlefieldEffectLocaleCoverage(enLocalePath, extraTables.world)
  validateWorldScheduleLocaleCoverage(zhLocalePath, extraTables.world)
  validateWorldScheduleLocaleCoverage(enLocalePath, extraTables.world)
  validateSpecializationLocaleCoverage(zhLocalePath, extraTables.specialization_trees)
  validateSpecializationLocaleCoverage(enLocalePath, extraTables.specialization_trees)
  console.log("game-data generated with version:", version)
}

if (process.argv[1] && path.resolve(process.argv[1]) === path.resolve(fileURLToPath(import.meta.url))) {
  main()
}
