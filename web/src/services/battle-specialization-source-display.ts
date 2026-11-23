export type BattleSourceTranslate = (key: string, params?: Record<string, unknown>) => string

export function buildBattleSpecializationSourceLabel(
  meta: Record<string, unknown>,
  t: BattleSourceTranslate
): string {
  const candidates: Record<string, unknown>[] = [meta]
  for (const key of ["runtimeDamageAmpSources", "mutationSources"] as const) {
    const sources = meta[key]
    if (!Array.isArray(sources)) continue
    for (const source of sources) {
      if (source && typeof source === "object" && !Array.isArray(source)) {
        candidates.push(source as Record<string, unknown>)
      }
    }
  }
  const names: string[] = []
  for (const source of candidates) {
    const sourceKind = String(source.sourceKind || source.kind || "").trim().toLowerCase()
    if (sourceKind !== "specialization") continue
    const nameKey = String(source.sourceNameKey || "").trim()
    const translated = nameKey ? t(nameKey) : ""
    const fallback = String(source.nodeId || source.id || source.sourceId || "").trim()
    const name = translated && translated !== nameKey ? translated : fallback
    if (name && !names.includes(name)) names.push(name)
  }
  return names.length > 0 ? t("game.battleSpecializationSource", { name: names.join(" / ") }) : ""
}
