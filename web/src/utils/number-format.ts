export type NumberFormatShortUnit = "k" | "m" | "b"
export type NumberFormatRoundingMode = "round" | "floor"

export function formatNumberWithThousands(value: number): string {
  const integer = Number.isFinite(value) ? Math.trunc(value) : 0
  return new Intl.NumberFormat("en-US", {
    maximumFractionDigits: 0
  }).format(integer)
}

export function formatNumberShort(
  value: number,
  options: {
    base?: 1000 | 1024
    maxFractionDigits?: number
    trimTrailingZeros?: boolean
    suffixes?: NumberFormatShortUnit[]
    roundingMode?: NumberFormatRoundingMode
    minIntegerDigitsForSuffix?: number
  } = {}
): string {
  const integer = Number.isFinite(value) ? Math.trunc(value) : 0
  const sign = integer < 0 ? "-" : ""
  const absolute = Math.abs(integer)
  const base = options.base || 1000
  const maxFractionDigits = options.maxFractionDigits ?? 1
  const trimTrailingZeros = options.trimTrailingZeros ?? true
  const suffixes = options.suffixes || ["k", "m", "b"]
  const roundingMode = options.roundingMode || "round"
  const minIntegerDigitsForSuffix = Math.max(1, Math.trunc(options.minIntegerDigitsForSuffix ?? 1))
  const minScaledValueForSuffix = 10 ** (minIntegerDigitsForSuffix - 1)

  if (absolute < base || suffixes.length === 0) return `${integer}`

  const candidates: Array<{ scaled: number; suffix: NumberFormatShortUnit }> = []
  let scaled = absolute
  for (const currentSuffix of suffixes) {
    scaled = scaled / base
    candidates.push({ scaled, suffix: currentSuffix })
    if (scaled < base) break
  }

  let selected: { scaled: number; suffix: NumberFormatShortUnit } | null = null
  for (const candidate of candidates) {
    if (candidate.scaled >= minScaledValueForSuffix) selected = candidate
  }
  if (!selected) return `${integer}`

  const safeFractionDigits = Math.max(0, Math.trunc(maxFractionDigits))
  const scalePow = 10 ** safeFractionDigits
  const roundedScaled = roundingMode === "floor"
    ? Math.floor(selected.scaled * scalePow) / scalePow
    : Math.round(selected.scaled * scalePow) / scalePow

  if (safeFractionDigits <= 0) return `${sign}${Math.trunc(roundedScaled)}${selected.suffix}`

  let formatted = roundedScaled.toFixed(safeFractionDigits)
  if (trimTrailingZeros) {
    formatted = formatted.replace(/\.0+$/, "").replace(/(\.\d*[1-9])0+$/, "$1")
  }
  return `${sign}${formatted}${selected.suffix}`
}

export function formatQuantityDisplay(value: number): string {
  return formatNumberShort(value, {
    base: 1000,
    maxFractionDigits: 1,
    trimTrailingZeros: true,
    roundingMode: "floor",
    minIntegerDigitsForSuffix: 2
  })
}

export function formatDecimal(value: number, fractionDigits = 2): string {
  if (!Number.isFinite(value)) return "0"
  const safeDigits = Number.isInteger(fractionDigits) && fractionDigits >= 0 ? fractionDigits : 2
  const pow = 10 ** safeDigits
  const rounded = Math.round(value * pow) / pow
  if (safeDigits <= 0) return String(Math.trunc(rounded))
  return rounded.toFixed(safeDigits).replace(/\.0+$/, "").replace(/(\.\d*[1-9])0+$/, "$1")
}
