import { formatAttributeValue, getAttributeUnit } from "@services/attribute-display"

type TranslateFn = (key: string, params?: Record<string, unknown>) => string

function asNumber(value: unknown): number | null {
  return typeof value === "number" && Number.isFinite(value) ? value : null
}

function localizeWithFallback(t: TranslateFn, key: string, fallback: string): string {
  const localized = t(key)
  if (localized !== key && localized.trim() !== "") return localized
  return fallback
}

// Affixes have two flavors:
//   - Static (id like "strength", "crit_chance") → i18n key `affix.<id>.name`.
//   - Custom craft (id like "custom:c452..."), produced by alchemy bench.
//     These ship with displayNameKey + displayNameArgs (e.g.,
//     "alchemy.affix.template.multi" + { prefix, suffix }) so we render the
//     player-facing name without depending on a static catalog row.
function resolveAffixLabel(t: TranslateFn, affixId: string, affix: Record<string, unknown>): string {
  const isCustom = affixId.startsWith("custom:")
  const displayNameKey = typeof affix.displayNameKey === "string" ? affix.displayNameKey.trim() : ""
  const displayNameArgs = (affix.displayNameArgs && typeof affix.displayNameArgs === "object" && !Array.isArray(affix.displayNameArgs))
    ? affix.displayNameArgs as Record<string, unknown>
    : undefined
  if (isCustom && displayNameKey) {
    const args = expandAffixNameArgs(t, displayNameArgs)
    const localized = t(displayNameKey, args || {})
    if (localized !== displayNameKey && localized.trim() !== "") return localized
  }
  return localizeWithFallback(t, `affix.${affixId}.name`, affixId)
}

// Display name args may carry nested i18n keys (e.g., prefixKey, suffixKey,
// prefixKeys[]). Resolve them to localized strings + a joined prefix string
// before passing to the template translator.
function expandAffixNameArgs(t: TranslateFn, raw: Record<string, unknown> | undefined): Record<string, unknown> | undefined {
  if (!raw) return undefined
  const out: Record<string, unknown> = { ...raw }
  const localizeOne = (key: unknown): string => {
    const k = typeof key === "string" ? key.trim() : ""
    if (!k) return ""
    const localized = t(k)
    return localized === k ? k : localized
  }
  if (typeof raw.prefixKey === "string") out.prefix = localizeOne(raw.prefixKey)
  if (typeof raw.suffixKey === "string") out.suffix = localizeOne(raw.suffixKey)
  if (Array.isArray(raw.prefixKeys)) {
    const parts = (raw.prefixKeys as unknown[]).map(localizeOne).filter(Boolean)
    const separator = t("alchemy.affix.prefixSeparator")
    const joined = parts.join(separator === "alchemy.affix.prefixSeparator" ? "" : separator)
    out.prefix = joined
    out.prefixes = joined
  }
  if (Array.isArray(raw.affixNameKeys)) {
    const parts = (raw.affixNameKeys as unknown[]).map(localizeOne).filter(Boolean)
    const separator = t("alchemy.affix.prefixSeparator")
    out.affixName = parts.join(separator === "alchemy.affix.prefixSeparator" ? "" : separator)
  }
  return out
}

function formatContributionValue(value: number, attrKey: string, fallbackUnit: unknown): string {
  if (!Number.isFinite(value)) return "0"
  const normalizedKey = String(attrKey || "").trim()
  const normalizedFallback = String(fallbackUnit || "").trim().toLowerCase()
  const unit =
    normalizedKey
      ? getAttributeUnit(normalizedKey, normalizedFallback === "percent" ? "percent" : "number")
      : normalizedFallback === "percent" ? "percent" : "number"
  return formatAttributeValue(value, unit, false, 2)
}

function appendEquipmentLines(
  lines: string[],
  rawEntries: unknown,
  t: TranslateFn,
  itemPrefix: string,
  detailPrefix: string,
  attrKey: string,
  fallbackUnit: unknown
) {
  const rawEquipments = Array.isArray(rawEntries) ? rawEntries : []
  for (const entry of rawEquipments) {
    if (!entry || typeof entry !== "object") continue
    const row = entry as Record<string, unknown>
    const templateItemId = typeof row.templateItemId === "string" ? row.templateItemId : ""
    const label = templateItemId ? localizeWithFallback(t, `item.${templateItemId}.name`, templateItemId) : "-"
    const total = asNumber(row.total)
    if (total === null) continue
    lines.push(`${itemPrefix}${label}: ${formatContributionValue(total, attrKey, fallbackUnit)}`)

    const affixes = Array.isArray(row.affixes) ? row.affixes : []
    const visibleAffixes: Array<{ label: string; value: number }> = []
    for (const affixEntry of affixes) {
      if (!affixEntry || typeof affixEntry !== "object") continue
      const affix = affixEntry as Record<string, unknown>
      const affixId = typeof affix.affixId === "string" ? affix.affixId : ""
      const affixValue = asNumber(affix.value)
      if (!affixId || affixValue === null || affixValue === 0) continue
      // Custom-crafted affixes don't live in static affix_definitions; the
      // backend ships a displayNameKey + displayNameArgs so we can render
      // their human label here. Fall back to the legacy lookup otherwise.
      const affixLabel = resolveAffixLabel(t, affixId, affix)
      visibleAffixes.push({ label: affixLabel, value: affixValue })
    }

    const base = asNumber(row.base)
    const enhance = asNumber(row.enhance)
    if ((visibleAffixes.length > 0 || (enhance !== null && enhance !== 0)) && base !== null && base !== 0) {
      lines.push(`${detailPrefix}${t("game.attrSourceEquipmentBase")}: ${formatContributionValue(base, attrKey, fallbackUnit)}`)
    }
    if (enhance !== null && enhance !== 0) {
      lines.push(
        `${detailPrefix}${t("game.enhance")}: ${formatContributionValue(enhance, attrKey, fallbackUnit)} (${t("game.attrSourceFromEnhance")})`
      )
    }
    for (const affix of visibleAffixes) {
      lines.push(
        `${detailPrefix}${affix.label}: ${formatContributionValue(affix.value, attrKey, fallbackUnit)} (${t("game.attrSourceFromAffix")})`
      )
    }
  }
}

function appendPersonalityLines(
  lines: string[],
  rawEntries: unknown,
  t: TranslateFn,
  attrKey: string,
  fallbackUnit: unknown
) {
  const entries = Array.isArray(rawEntries) ? rawEntries : []
  for (const entry of entries) {
    if (!entry || typeof entry !== "object") continue
    const row = entry as Record<string, unknown>
    const personalityId = typeof row.personalityId === "string" ? row.personalityId : ""
    const nameKey = typeof row.nameKey === "string" ? row.nameKey : ""
    const label = nameKey
      ? localizeWithFallback(t, nameKey, personalityId || nameKey)
      : localizeWithFallback(t, `kitty.personality.${personalityId}.name`, personalityId || "-")
    const total = asNumber(row.total)
    if (total === null) continue
    lines.push(`- ${t("game.attrSourcePersonality")} ${label}: ${formatContributionValue(total, attrKey, fallbackUnit)}`)
  }
}

function appendCollectibleLines(
  lines: string[],
  rawEntries: unknown,
  t: TranslateFn,
  attrKey: string,
  fallbackUnit: unknown
) {
  const entries = Array.isArray(rawEntries) ? rawEntries : []
  for (const entry of entries) {
    if (!entry || typeof entry !== "object") continue
    const row = entry as Record<string, unknown>
    const collectibleId = typeof row.collectibleId === "string" ? row.collectibleId : ""
    const nameKey = typeof row.nameKey === "string" ? row.nameKey : ""
    const label = nameKey
      ? localizeWithFallback(t, nameKey, collectibleId || nameKey)
      : localizeWithFallback(t, `collectible.${collectibleId}.name`, collectibleId || "-")
    const total = asNumber(row.total)
    if (total === null) continue
    lines.push(`- ${t("game.attrSourceCollectible")} ${label}: ${formatContributionValue(total, attrKey, fallbackUnit)}`)
  }
}

function appendConversionLines(
  lines: string[],
  rawConversion: unknown,
  t: TranslateFn,
  targetAttrKey: string,
  fallbackUnit: unknown,
  localizeAttrFn: (key: string) => string
) {
  if (!rawConversion || typeof rawConversion !== "object" || Array.isArray(rawConversion)) return
  const conv = rawConversion as Record<string, unknown>
  const totalValue = asNumber(conv.value)
  if (totalValue === null || totalValue === 0) return

  const isDeduction = totalValue < 0
  const sourceLabel = isDeduction
    ? t("game.attrSourceConversionDeduction")
    : t("game.attrSourceConversionBonus")

  const sources = Array.isArray(conv.sources) ? conv.sources : []
  for (const rawSrc of sources) {
    if (!rawSrc || typeof rawSrc !== "object") continue
    const src = rawSrc as Record<string, unknown>
    const templateItemId = typeof src.templateItemId === "string" ? src.templateItemId : ""
    const fromAttr = typeof src.fromAttr === "string" ? src.fromAttr : ""
    const toAttr = typeof src.toAttr === "string" ? src.toAttr : ""
    const excess = asNumber(src.excess) ?? 0
    const ratio = asNumber(src.ratio) ?? 0
    const converted = asNumber(src.converted) ?? 0

    const itemName = templateItemId
      ? localizeWithFallback(t, `item.${templateItemId}.name`, templateItemId)
      : "-"

    const valueStr = formatContributionValue(isDeduction ? -excess : converted, targetAttrKey, fallbackUnit)
    lines.push(`- ${itemName} (${sourceLabel}): ${valueStr}`)

    if (isDeduction) {
      const toAttrName = localizeAttrFn(toAttr)
      const ratioStr = Math.round(ratio * 100)
      lines.push(
        `  - ${t("game.attrSourceConversionTo", { toAttr: toAttrName, ratio: ratioStr })}`
      )
    } else {
      const fromAttrName = localizeAttrFn(fromAttr)
      const excessStr = formatContributionValue(excess, fromAttr, fallbackUnit)
      const ratioStr = Math.round(ratio * 100)
      lines.push(
        `  - ${t("game.attrSourceConversionFrom", { fromAttr: fromAttrName, excess: excessStr, ratio: ratioStr })}`
      )
    }
  }
}

function buildV2BreakdownLines(
  targetAttrKey: string,
  fallbackUnit: unknown,
  rawSources: Record<string, unknown>,
  t: TranslateFn,
  localizeAttrFn: (key: string) => string
): string[] {
  const lines: string[] = []
  const base = rawSources.base
  if (base && typeof base === "object") {
    const baseRow = base as Record<string, unknown>
    const baseDefault = asNumber(baseRow.baseDefault)
    if (baseDefault !== null && baseDefault !== 0) {
      lines.push(`- ${t("game.attrSourceBase")}: ${formatContributionValue(baseDefault, targetAttrKey, fallbackUnit)}`)
    }
    const baseLevelBonus = asNumber(baseRow.baseLevelBonus)
    if (baseLevelBonus !== null && baseLevelBonus !== 0) {
      lines.push(`- ${t("game.attrSourceBaseLevelBonus")}: ${formatContributionValue(baseLevelBonus, targetAttrKey, fallbackUnit)}`)
    }
    const skillPointBonus = asNumber(baseRow.skillPointBonus)
    if (skillPointBonus !== null && skillPointBonus !== 0) {
      lines.push(`- ${t("game.attrSourceAttributePoints")}: ${formatContributionValue(skillPointBonus, targetAttrKey, fallbackUnit)}`)
    }
  }
  appendEquipmentLines(lines, rawSources.equipment, t, "- ", "  - ", targetAttrKey, fallbackUnit)
  appendPersonalityLines(lines, rawSources.personality, t, targetAttrKey, fallbackUnit)
  appendCollectibleLines(lines, rawSources.collectible, t, targetAttrKey, fallbackUnit)

  const formula = rawSources.formula
  if (formula && typeof formula === "object") {
    const formulaRow = formula as Record<string, unknown>
    const sources = Array.isArray(formulaRow.sources) ? formulaRow.sources : []
    for (const entry of sources) {
      if (!entry || typeof entry !== "object") continue
      const row = entry as Record<string, unknown>
      const sourceAttrKey = typeof row.attrKey === "string" ? row.attrKey : ""
      if (!sourceAttrKey) continue
      const lineValue =
        asNumber(row.value) ??
        (() => {
          const attrValue = asNumber(row.attributeValue)
          const coefficient = asNumber(row.coefficient)
          if (attrValue === null || coefficient === null) return null
          return attrValue * coefficient
        })()
      if (lineValue === null) continue
      const attrLabel = localizeWithFallback(t, `attr.${sourceAttrKey}.name`, sourceAttrKey)
      lines.push(`- ${attrLabel}: ${formatContributionValue(lineValue, targetAttrKey, fallbackUnit)}`)
    }
  }

  appendConversionLines(lines, rawSources.conversion, t, targetAttrKey, fallbackUnit, localizeAttrFn)

  return lines
}

export function buildAttributeBreakdownLines(
  attrKey: string,
  meta: Record<string, unknown>,
  t: TranslateFn,
  localizeAttrFn?: (key: string) => string
): string[] {
  const rawSources = meta.sources
  if (!rawSources || typeof rawSources !== "object") return []
  const sourceObj = rawSources as Record<string, unknown>
  if (sourceObj.version !== "v2") return []
  const attrLabelFn = localizeAttrFn ?? ((key: string) => localizeWithFallback(t, `attr.${key}.name`, key))
  return buildV2BreakdownLines(attrKey, meta.unit, sourceObj, t, attrLabelFn)
}
