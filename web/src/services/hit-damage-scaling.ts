export const POWERED_EXPONENTIAL_DECAY_KIND = "powered_exponential_decay" as const
export const MAX_RUNTIME_DAMAGE_HITS = 64

export const DEFAULT_POWERED_EXPONENTIAL_DECAY = Object.freeze({
  kind: POWERED_EXPONENTIAL_DECAY_KIND,
  secondHitMultiplier: 0.998,
  curvePower: 2.1708
})

export type PoweredExponentialDecayDefinition = {
  kind: typeof POWERED_EXPONENTIAL_DECAY_KIND
  secondHitMultiplier: number
  curvePower: number
}

export type HitDamageScalingDefinition = PoweredExponentialDecayDefinition

export type HitDamageEquivalentCap = {
  equivalentHits: number
  converged: boolean
  evaluatedHits: number
}

export function normalizeHitDamageScaling(raw: unknown): HitDamageScalingDefinition | null {
  if (!raw || typeof raw !== "object" || Array.isArray(raw)) return null
  const row = raw as Record<string, unknown>
  const allowedKeys = new Set(["kind", "secondHitMultiplier", "curvePower"])
  if (Object.keys(row).some((key) => !allowedKeys.has(key))) return null
  if (row.kind !== POWERED_EXPONENTIAL_DECAY_KIND) return null
  if (typeof row.secondHitMultiplier !== "number" || typeof row.curvePower !== "number") return null
  const secondHitMultiplier = row.secondHitMultiplier
  const curvePower = row.curvePower
  if (!Number.isFinite(secondHitMultiplier) || secondHitMultiplier <= 0 || secondHitMultiplier >= 1) return null
  if (!Number.isFinite(curvePower) || curvePower < 1 || curvePower > 3) return null
  return { kind: POWERED_EXPONENTIAL_DECAY_KIND, secondHitMultiplier, curvePower }
}

export function hitDamageMultiplier(
  scaling: HitDamageScalingDefinition | null | undefined,
  hitIndex: number
): number {
  const normalized = normalizeHitDamageScaling(scaling)
  if (!normalized) return 1
  const index = Math.max(1, Math.trunc(Number(hitIndex) || 1))
  return poweredExponentialDecayMultiplier(normalized, index)
}

export function hitDamageEquivalentSum(
  scaling: HitDamageScalingDefinition | null | undefined,
  hitCount: number
): number {
  const normalized = normalizeHitDamageScaling(scaling)
  const count = Math.max(1, Math.min(MAX_RUNTIME_DAMAGE_HITS, Math.trunc(Number(hitCount) || 1)))
  if (!normalized) return count
  let sum = 0
  for (let hitIndex = 1; hitIndex <= count; hitIndex += 1) {
    const multiplier = poweredExponentialDecayMultiplier(normalized, hitIndex)
    sum += multiplier
    if (multiplier <= Number.EPSILON) break
  }
  return sum
}

export function approximateHitDamageEquivalentCap(
  scaling: HitDamageScalingDefinition | null | undefined,
  options: { epsilon?: number; maxHits?: number } = {}
): HitDamageEquivalentCap | null {
  const normalized = normalizeHitDamageScaling(scaling)
  if (!normalized) return null
  const epsilonRaw = Number(options.epsilon ?? 1e-10)
  const epsilon = Number.isFinite(epsilonRaw) && epsilonRaw > 0 ? epsilonRaw : 1e-10
  const maxHitsRaw = Math.trunc(Number(options.maxHits ?? 4096))
  const maxHits = Number.isFinite(maxHitsRaw) ? Math.max(2, Math.min(100_000, maxHitsRaw)) : 4096
  let equivalentHits = 0
  for (let hitIndex = 1; hitIndex <= maxHits; hitIndex += 1) {
    const multiplier = poweredExponentialDecayMultiplier(normalized, hitIndex)
    equivalentHits += multiplier
    if (hitIndex > 1 && multiplier <= epsilon) {
      return { equivalentHits, converged: true, evaluatedHits: hitIndex }
    }
  }
  return { equivalentHits, converged: false, evaluatedHits: maxHits }
}

function poweredExponentialDecayMultiplier(scaling: PoweredExponentialDecayDefinition, hitIndex: number): number {
  if (hitIndex <= 1) return 1
  const exponent = Math.pow(hitIndex - 1, scaling.curvePower)
  const multiplier = Math.pow(scaling.secondHitMultiplier, exponent)
  if (!Number.isFinite(multiplier)) return 0
  return Math.max(0, Math.min(1, multiplier))
}
