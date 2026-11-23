export type StatusRequireMap = Record<string, number>

export type StatusRequireEntry = {
  attrKey: string
  required: number
}

export type StatusRequireUnmetEntry = StatusRequireEntry & {
  current: number
}

export function toStatusRequireEntries(map?: StatusRequireMap | null): StatusRequireEntry[] {
  return Object.entries(map || {})
    .map(([attrKey, rawRequired]) => ({
      attrKey,
      required: Math.max(0, Math.floor(Number(rawRequired)))
    }))
    .filter((entry) => Number.isFinite(entry.required) && entry.required > 0)
}

export function evaluateStatusRequire(
  map: StatusRequireMap | null | undefined,
  attributes: Record<string, number> | null | undefined
): StatusRequireUnmetEntry[] {
  const currentAttributes = attributes || {}
  return toStatusRequireEntries(map).flatMap((entry) => {
    const current = Number(currentAttributes[entry.attrKey] || 0)
    if (current >= entry.required) return []
    return [{ ...entry, current }]
  })
}
