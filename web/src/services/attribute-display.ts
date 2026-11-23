import { getGameDataChunk } from "@services/game-data-runtime"
import { formatDecimal } from "@utils/number-format"

export type AttributeUnit = "number" | "percent"

export type AttributeDefinition = {
  id?: string
  icon?: string
  iconId?: string
  emojiIcon?: string
  nameKey?: string
  descKey?: string
  unit?: string
  category?: string
  growthType?: string
  order?: number
}

let attributeUnitMap: Map<string, AttributeUnit> | null = null
let attributeDefinitionMap: Map<string, AttributeDefinition> | null = null
let attributeDefinitions: AttributeDefinition[] | null = null

function ensureAttributeDefinitions() {
  if (attributeUnitMap && attributeDefinitionMap && attributeDefinitions) return
  attributeUnitMap = new Map<string, AttributeUnit>()
  attributeDefinitionMap = new Map<string, AttributeDefinition>()
  attributeDefinitions = []
  const attributesChunk = getGameDataChunk("attributes")
  const rows = Array.isArray((attributesChunk as { attributes?: unknown }).attributes)
    ? ((attributesChunk as { attributes: Array<Record<string, unknown>> }).attributes || [])
    : []
  for (const row of rows) {
    const key = String(row.id || "").trim()
    const unit = String(row.unit || "").trim().toLowerCase()
    if (!key) continue
    const definition = row as AttributeDefinition
    attributeDefinitions.push(definition)
    attributeDefinitionMap.set(key, definition)
    attributeUnitMap.set(key, unit === "percent" ? "percent" : "number")
  }
}

export function getAttributeUnit(attrKey: string, fallback: AttributeUnit = "number"): AttributeUnit {
  const normalized = String(attrKey || "").trim()
  if (!normalized) return fallback
  ensureAttributeDefinitions()
  return attributeUnitMap!.get(normalized) || fallback
}

export function getAttributeDefinition(attrKey: string): AttributeDefinition | null {
  const normalized = String(attrKey || "").trim()
  if (!normalized) return null
  ensureAttributeDefinitions()
  return attributeDefinitionMap!.get(normalized) || null
}

export function listAttributeDefinitions(): AttributeDefinition[] {
  ensureAttributeDefinitions()
  return [...attributeDefinitions!]
}

export function getAttributeDomain(attrKey: string): "battle" | "work" | "other" {
  const definition = getAttributeDefinition(attrKey)
  const category = String(definition?.category || "").trim().toLowerCase()
  if (category === "battle" || category === "work") return category
  return "other"
}

export function formatAttributeValue(value: number, unit: AttributeUnit, signed = false, precision = 2): string {
  const numeric = Number.isFinite(value) ? value : 0
  const converted = unit === "percent" ? numeric * 100 : numeric
  const signPrefix = signed ? (converted >= 0 ? "+" : "-") : ""
  const baseValue = signed ? Math.abs(converted) : converted
  const text = formatDecimal(baseValue, precision)

  return unit === "percent" ? `${signPrefix}${text}%` : `${signPrefix}${text}`
}

export function formatAttributeValueByKey(
  attrKey: string,
  value: number,
  signed = false,
  precision = 2,
  fallbackUnit: AttributeUnit = "number"
): string {
  return formatAttributeValue(value, getAttributeUnit(attrKey, fallbackUnit), signed, precision)
}
