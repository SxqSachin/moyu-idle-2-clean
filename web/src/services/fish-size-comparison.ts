export type FishSizeMetric = "length" | "weight"

export type FishSizeReferenceSpecies = {
  minLengthCm: number
  maxLengthCm: number
  shapeFactor: number
}

export type FishSizeComparison = {
  arrowCount: 1 | 2 | 3
  excessPercent: number
  standardValue: number
}

// Keep this aligned with fishingSaleReferenceLengthRatio in the authoritative sale formula.
export const FISH_STANDARD_LENGTH_RANGE_RATIO = 0.30

const SECOND_ARROW_EXCESS_RATIO = 0.20
const THIRD_ARROW_EXCESS_RATIO = 0.50
const COMPARISON_EPSILON = 1e-9

export function fishStandardMeasurements(species: FishSizeReferenceSpecies | null | undefined) {
  if (!species) return null
  const minLengthCm = Number(species.minLengthCm)
  const maxLengthCm = Number(species.maxLengthCm)
  const shapeFactor = Number(species.shapeFactor)
  if (!Number.isFinite(minLengthCm) || minLengthCm <= 0) return null
  if (!Number.isFinite(maxLengthCm) || maxLengthCm <= minLengthCm) return null
  if (!Number.isFinite(shapeFactor) || shapeFactor <= 0) return null

  const lengthCm = minLengthCm + (maxLengthCm - minLengthCm) * FISH_STANDARD_LENGTH_RANGE_RATIO
  const weightKg = shapeFactor * Math.pow(lengthCm, 3) / 100_000
  if (!Number.isFinite(weightKg) || weightKg <= 0) return null
  return { lengthCm, weightKg }
}

export function compareFishSize(
  metric: FishSizeMetric,
  rawValue: number,
  species: FishSizeReferenceSpecies | null | undefined
): FishSizeComparison | null {
  const value = Number(rawValue)
  if (!Number.isFinite(value) || value <= 0) return null
  const standard = fishStandardMeasurements(species)
  if (!standard) return null

  const standardValue = metric === "length" ? standard.lengthCm : standard.weightKg
  const excessRatio = value / standardValue - 1
  if (!Number.isFinite(excessRatio) || excessRatio <= 0) return null

  let arrowCount: FishSizeComparison["arrowCount"] = 1
  if (excessRatio + COMPARISON_EPSILON >= THIRD_ARROW_EXCESS_RATIO) arrowCount = 3
  else if (excessRatio + COMPARISON_EPSILON >= SECOND_ARROW_EXCESS_RATIO) arrowCount = 2

  return {
    arrowCount,
    excessPercent: Math.max(1, Math.round(excessRatio * 100)),
    standardValue
  }
}
