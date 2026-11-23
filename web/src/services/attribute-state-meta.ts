export type AttributeStateMeta = {
  value: number
  baseValue: number
  skillPointLevel: number
  unit: string
  category: string
  growthType: string
  order: number
  nameKey: string
  descKey: string
  iconId: string
  emojiIcon: string
  allowBattlePointLevelUp: boolean
  sources: Record<string, unknown>
}

export function mapAttributeStateMeta(row: Record<string, unknown>): AttributeStateMeta {
  const sourceMods = row.sourceMods && typeof row.sourceMods === "object"
    ? row.sourceMods as Record<string, unknown>
    : {}
  return {
    value: Number(row.finalValue || 0),
    baseValue: Number(row.baseDefault || 0),
    skillPointLevel: Number(row.skillPointLevel || 0),
    unit: String(row.unit || "number"),
    category: String(row.category || "other"),
    growthType: String(row.growthType || "static"),
    order: Number(row.order || 0),
    nameKey: String(row.nameKey || ""),
    descKey: String(row.descKey || ""),
    iconId: String(row.iconId || ""),
    emojiIcon: String(row.emojiIcon || ""),
    allowBattlePointLevelUp: Boolean(row.allowBattlePointLevelUp),
    sources: sourceMods
  }
}
