export type DerivedImpact = {
  targetAttrKey: string
  deltaPerPoint: number
}

export type DerivedImpactMap = Record<string, DerivedImpact[]>
type NumberMap = Record<string, number>

const LINEAR_TERM_PATTERN = /^([a-zA-Z_][a-zA-Z0-9_]*)\*([+-]?(?:\d+(?:\.\d+)?|\.\d+))$/

function parseLinearTerms(expression: string): Array<{ attrKey: string; coefficient: number }> {
  const compact = expression.replace(/\s+/g, "")
  if (!compact) return []
  const out: Array<{ attrKey: string; coefficient: number }> = []
  for (const rawTerm of compact.split("+")) {
    const term = rawTerm.trim()
    if (!term) continue
    const match = term.match(LINEAR_TERM_PATTERN)
    if (!match) continue
    const [, attrKey, coefficientRaw] = match
    const coefficient = Number(coefficientRaw)
    if (!Number.isFinite(coefficient)) continue
    out.push({ attrKey, coefficient })
  }
  return out
}

export function buildBaseAttrImpactMap(derived: Record<string, string> | null | undefined): DerivedImpactMap {
  if (!derived) return {}
  const out: DerivedImpactMap = {}
  for (const [targetAttrKey, expression] of Object.entries(derived)) {
    if (typeof expression !== "string") continue
    const terms = parseLinearTerms(expression)
    for (const term of terms) {
      if (!out[term.attrKey]) {
        out[term.attrKey] = []
      }
      out[term.attrKey].push({
        targetAttrKey,
        deltaPerPoint: term.coefficient
      })
    }
  }
  return out
}

function roundTo4(value: number): number {
  return Math.round(value * 10000) / 10000
}

function normalizeNumberMap(input: Record<string, unknown> | null | undefined): NumberMap {
  const out: NumberMap = {}
  if (!input || typeof input !== "object") return out
  for (const [rawKey, rawValue] of Object.entries(input)) {
    const key = String(rawKey || "").trim()
    if (!key) continue
    const value = Number(rawValue)
    if (!Number.isFinite(value)) continue
    out[key] = roundTo4(value)
  }
  return out
}

export function projectDerivedFinalStats(
  baseStats: Record<string, unknown> | null | undefined,
  derived: Record<string, string> | null | undefined
): NumberMap {
  const finalStats = normalizeNumberMap(baseStats)
  if (!derived || typeof derived !== "object") return finalStats
  for (const [rawTargetAttrKey, expression] of Object.entries(derived)) {
    if (typeof expression !== "string") continue
    const targetAttrKey = String(rawTargetAttrKey || "").trim().toLowerCase()
    if (!targetAttrKey) continue
    const terms = parseLinearTerms(expression)
    if (terms.length === 0) continue
    let total = 0
    for (const term of terms) {
      const sourceKey = String(term.attrKey || "").trim().toLowerCase()
      if (!sourceKey) continue
      const sourceValue = Number(finalStats[sourceKey] ?? 0)
      if (!Number.isFinite(sourceValue)) continue
      total += sourceValue * term.coefficient
    }
    const targetBase = Number(finalStats[targetAttrKey] ?? 0)
    finalStats[targetAttrKey] = roundTo4((Number.isFinite(targetBase) ? targetBase : 0) + total)
  }
  return finalStats
}
