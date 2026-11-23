import { getGameDataChunk } from "@services/game-data-runtime"

type AttributeMeta = {
  id: string
  relatedActionMainType: string
  relatedActionEffect: string
}

export function buildActionEfficiencyIndex(rows: Array<Record<string, unknown>>): Map<string, string[]> {
  const index = new Map<string, string[]>()
  for (const row of rows) {
    const meta = normalizeAttributeMeta(row)
    if (!meta) continue
    if (meta.relatedActionEffect !== "actionefficiency") continue
    if (!index.has(meta.relatedActionMainType)) {
      index.set(meta.relatedActionMainType, [])
    }
    index.get(meta.relatedActionMainType)!.push(meta.id)
  }
  return index
}

let actionEfficiencyByMainType: Map<string, string[]> | null = null
let actionEfficiencyAttrKeySet: Set<string> | null = null

function ensureActionEfficiencyIndex() {
  if (actionEfficiencyByMainType && actionEfficiencyAttrKeySet) return
  const attributesChunk = getGameDataChunk("attributes")
  actionEfficiencyByMainType = buildActionEfficiencyIndex(
    Array.isArray((attributesChunk as { attributes?: unknown }).attributes)
      ? ((attributesChunk as { attributes: Array<Record<string, unknown>> }).attributes || [])
      : []
  )
  actionEfficiencyAttrKeySet = new Set(
    Array.from(actionEfficiencyByMainType.values()).flat()
  )
}

function normalizeAttributeMeta(row: Record<string, unknown>): AttributeMeta | null {
  const id = String(row.id || "").trim()
  const relatedActionMainType = String(row.relatedActionMainType || "").trim().toLowerCase()
  const rawEffect = String(row.relatedActionEffect || "").trim().toLowerCase()
  const relatedActionEffect = rawEffect || "actionefficiency"
  if (!id || !relatedActionMainType) return null
  return { id, relatedActionMainType, relatedActionEffect }
}

export function resolveActionDurationSeconds(
  baseDurationSeconds: number,
  actionMainType: string | undefined,
  _actionSubType: string | undefined,
  _actionID: string | undefined,
  actorType: "player" | "kitty",
  attributes: Record<string, number>,
  additionalEfficiency = 0
): number {
  const base = Number.isFinite(baseDurationSeconds) && baseDurationSeconds > 0 ? baseDurationSeconds : 1
  const efficiency = resolveActionEfficiency(actionMainType, attributes) + (actorType === 'kitty' && actionMainType !== 'fishing' && Number.isFinite(additionalEfficiency) ? additionalEfficiency : 0)
  const minMultiplier = actorType === "kitty" ? 0.5 : 0.05
  const normalizedEfficiency = Number.isFinite(efficiency) ? efficiency : 0
  const duration = normalizedEfficiency >= 0
    ? Math.max(base * minMultiplier, base / Math.max(0.01, 1 + normalizedEfficiency))
    : base * (1 + Math.abs(normalizedEfficiency))
  return Math.max(2, Math.floor(duration * 1000) / 1000)
}

export function resolveActionEfficiency(
  actionMainType: string | undefined,
  attributes: Record<string, number>
): number {
  const normalizedMainType = String(actionMainType || "").trim().toLowerCase()
  if (!normalizedMainType) return 0
  ensureActionEfficiencyIndex()
  const keys = actionEfficiencyByMainType!.get(normalizedMainType) || []
  let bonus = 0
  for (const attrKey of keys) {
    const value = Number(attributes[attrKey] || 0)
    if (!Number.isFinite(value)) continue
    bonus += value
  }
  return bonus
}

export function isActionEfficiencyAttributeKey(attrKey: string | undefined): boolean {
  const normalized = String(attrKey || "").trim()
  if (!normalized) return false
  ensureActionEfficiencyIndex()
  return actionEfficiencyAttrKeySet!.has(normalized)
}
