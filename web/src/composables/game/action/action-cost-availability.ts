export type ActionCostAvailabilityRow = {
  itemId: string
  required: number
  owned: number
}

export function buildInsufficientActionCosts(
  costs: Record<string, number> | undefined,
  count: number,
  getOwnedQuantity: (itemId: string) => number
): ActionCostAvailabilityRow[] {
  if (!costs) return []
  const requiredCount = Math.max(1, Math.floor(Number(count || 1)))
  const rows: ActionCostAvailabilityRow[] = []
  for (const [itemId, rawRequired] of Object.entries(costs)) {
    const required = Math.max(0, Math.floor(Number(rawRequired) * requiredCount))
    if (required <= 0) continue
    const owned = Math.max(0, Math.floor(Number(getOwnedQuantity(itemId) || 0)))
    if (owned >= required) continue
    rows.push({ itemId, required, owned })
  }
  return rows
}
