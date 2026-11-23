const object = (v) => v !== null && typeof v === "object" && !Array.isArray(v)
function shape(v, keys, label) {
  if (!object(v) || Object.keys(v).some(k => !keys.includes(k))) throw new Error(`invalid ${label}`)
}
export function normalizeMarketPolicy(raw) {
  if (raw === undefined) return undefined
  shape(raw, ["fixedPrice", "buyOrders", "auction"], "marketPolicy")
  shape(raw.fixedPrice, ["mode", "profileId"], "marketPolicy.fixedPrice")
  if (raw.fixedPrice.mode !== "instance_variants" || typeof raw.fixedPrice.profileId !== "string" || !raw.fixedPrice.profileId.trim() || raw.buyOrders !== false || typeof raw.auction !== "boolean") throw new Error("invalid marketPolicy capabilities")
  return raw
}
function key(v, extra = []) {
  shape(v, ["source", "path", "compare", ...extra], "market profile key")
  if (!["instance", "resolved.operationEffect"].includes(v.source) || !["exact", "set"].includes(v.compare) || typeof v.path !== "string" || v.path.length > 256 || (v.path !== "" && !v.path.startsWith("/")) || /[\[\]*]/.test(v.path) || /~(?![01])/.test(v.path) || v.path.split("/").some(k => ["__proto__", "constructor", "prototype"].includes(k))) throw new Error("invalid market profile selector")
  const roots = v.source === "instance" ? ["baseSnapshot", "customData", "enhanceLevel", "affixes", "durability", "maxDurability", "fishData", "specimenNames"] : ["id", "rulesetVersion", "domain", "kind", "priority", "params", "lifetime", "allowedTargets", "selector", "stacking"]
  if (v.path && !roots.includes(v.path.split("/")[1])) throw new Error("unknown market source field")
}
const sourcePath = p => typeof p === 'string' && p.length <= 256 && /^\/(?!$)/.test(p) && !/[\[\]*]/.test(p) && !/~(?![01])/.test(p) && !p.split('/').some(k => ['__proto__', 'constructor', 'prototype'].includes(k))
const readPath = (row, path) => path.slice(1).split('/').reduce((v, k) => v?.[k.replace(/~1/g, '/').replace(/~0/g, '~')], row)
export function validateMarketPolicies(items, profiles, tables) {
  if (!Array.isArray(profiles)) throw new Error("market_profiles must be an array")
  const ids = new Set()
  for (const p of profiles) {
    shape(p, ["id", "disabled", "schemaVersion", "groupingVersion", "catalogKeys", "catalogSource", "groupKeys", "uniqueByInstance", "presentation"], "market profile")
    if (p.disabled !== undefined && typeof p.disabled !== "boolean") throw new Error("invalid market profile disabled flag")
    if (!p.id || ids.has(p.id) || p.schemaVersion !== 1 || !Number.isSafeInteger(p.groupingVersion) || p.groupingVersion < 1) throw new Error("invalid market profile identity")
    ids.add(p.id)
    if (!Array.isArray(p.catalogKeys) || p.catalogKeys.length > 4 || !Array.isArray(p.groupKeys) || p.groupKeys.length < 1 || p.groupKeys.length > 32 || !Array.isArray(p.presentation) || p.presentation.length > 12 || (p.uniqueByInstance !== undefined && typeof p.uniqueByInstance !== "boolean")) throw new Error("invalid market profile fields")
    for (const k of [...p.catalogKeys, ...p.groupKeys]) key(k)
    if (p.catalogSource !== undefined) {
      const s = p.catalogSource
      shape(s, ['table', 'namesPath', 'valuePaths'], 'market catalog source')
      if (typeof s.table !== 'string' || !/^[a-z][a-z0-9_]*$/.test(s.table) || !sourcePath(s.namesPath) || !Array.isArray(s.valuePaths) || !s.valuePaths.length || s.valuePaths.length !== p.catalogKeys.length || !s.valuePaths.every(sourcePath) || p.catalogKeys.some(k => k.source !== 'instance' || k.compare !== 'exact' || !k.path)) throw new Error('invalid market catalog source')
      if (tables) {
        if (!Array.isArray(tables[s.table])) throw new Error('unknown market catalog table')
        const identities = new Set()
        for (const row of tables[s.table].filter(row => !row.disabled)) {
          const values = s.valuePaths.map(path => readPath(row, path))
          const names = readPath(row, s.namesPath)
          if (values.some(v => typeof v !== 'string' || !v || v.length > 256) || !object(names) || !Object.values(names).length || !Object.values(names).every(v => typeof v === 'string' && v.trim())) throw new Error('invalid market catalog row')
          const identity = JSON.stringify(values)
          if (identities.has(identity)) throw new Error('duplicate market catalog values')
          identities.add(identity)
        }
      }
    }
    for (const f of p.presentation) {
      key(f, ["labelKey", "format"])
      if (!f.labelKey || !["text", "number", "percent", "seconds", "operation_effect"].includes(f.format)) throw new Error("invalid market profile presentation")
    }
  }
  for (const i of items) {
    if (i.marketPolicy === undefined) continue
    normalizeMarketPolicy(i.marketPolicy)
    if (i.stackable || i.type.includes("currency") || !ids.has(i.marketPolicy.fixedPrice.profileId)) throw new Error(`invalid market profile reference: ${i.id}`)
  }
}
