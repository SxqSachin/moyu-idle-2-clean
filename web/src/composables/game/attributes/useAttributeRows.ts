import { computed, type Ref } from "vue"
import { buildAttributeBreakdownLines } from "@services/attribute-breakdown"
import { buildBaseAttrImpactMap } from "@services/attribute-derived-impact"
import { formatAttributeValue } from "@services/attribute-display"
import { formatDecimal } from "@utils/number-format"
import { DEFENSE_K } from "@/config/shared-global-config"

type AttributeProgress = {
  value?: number
  level?: number
  exp?: number
  nextExp?: number
}

type AttributeMeta = {
  value?: number
  baseValue?: number
  skillPointLevel?: number
  allowBattlePointLevelUp?: boolean
  unit?: string
  category?: string
  growthType?: string
  order?: number
  nameKey?: string
  descKey?: string
  sources?: unknown
}

type FormulaSnapshot = {
  derived?: Record<string, string>
}

export type AttributeRow = {
  key: string
  name: string
  desc: string
  value: string
  valueSuffix?: string
  rawValue: number
  skillPointLevel: number
  allowBattlePointLevelUp: boolean
  compositionLines: string[]
  impactIntro?: string
  impactLines: string[]
  breakdownLines: string[]
  growthType: string
  category: string
  order: number
  level: number
  exp: number
  nextExp: number
  progress: number
}

const categoryWeight: Record<string, number> = { battle: 0, work: 1, other: 2 }
const DEFENSE_REDUCTION_KEYS = new Set(["physics_defence", "magical_defence"])
const ALCHEMY_IMPACT_KEYS: Record<string, string[]> = {
  alchemy_precision: [
    "game.attrAlchemyImpactPrecisionCeiling",
    "game.attrAlchemyImpactPrecisionNegative"
  ],
  alchemy_craft_success: [
    "game.attrAlchemyImpactStabilityFailure",
    "game.attrAlchemyImpactStabilityFloor",
    "game.attrAlchemyImpactStabilityCap"
  ],
  alchemy_lucky_chance: [
    "game.attrAlchemyImpactLuckyInfusion",
    "game.attrAlchemyImpactLuckyReward"
  ],
  alchemy_efficiency: [
    "game.attrAlchemyImpactEfficiencyQueue"
  ]
}

function localizedAttrDescription(
  key: string,
  meta: AttributeMeta | undefined,
  t: (key: string, params?: Record<string, string | number>) => string
) {
  const descKey = typeof meta?.descKey === "string" && meta.descKey ? meta.descKey : `attr.${key}.desc`
  return t(descKey)
}

function buildAlchemyImpactLines(
  key: string,
  t: (key: string, params?: Record<string, string | number>) => string
) {
  return (ALCHEMY_IMPACT_KEYS[key] || []).map(lineKey => t(lineKey))
}

export function useAttributeRows(
  attributes: Ref<Record<string, AttributeMeta>>,
  attributeProgress: Ref<Record<string, AttributeProgress>>,
  formula: Ref<FormulaSnapshot>,
  t: (key: string, params?: Record<string, string | number>) => string
) {
  const attrRows = computed<AttributeRow[]>(() => {
    const impactMapByBase = buildBaseAttrImpactMap(formula.value?.derived)
    const rows = Object.entries(attributes.value || {}).map(([key, meta]) => {
      const growthType = meta?.growthType || "levelable"
      const progressNode = attributeProgress.value[key]
      const current = progressNode || {
        value: Number(meta?.value || 0),
        level: 1,
        exp: 0,
        nextExp: 0
      }
      const nextExp = Number(current.nextExp || 0)
      const currentExp = Number(current.exp || 0)
      const rawValue = growthType === "static"
        ? Number(meta?.value || 0)
        : Number(current.value ?? meta?.value ?? 0)
      const unit = meta?.unit || "number"
      const value = formatAttributeValue(rawValue, unit === "percent" ? "percent" : "number")
      const displayName = typeof meta?.nameKey === "string" && meta.nameKey ? t(meta.nameKey) : t(`attr.${key}`)
      const compositionLines = buildAttributeBreakdownLines(key, meta as Record<string, unknown>, t)
      const alchemyImpactLines = buildAlchemyImpactLines(key, t)
      const impactLines: string[] = [...alchemyImpactLines]
      const impacts = [...(impactMapByBase[key] || [])].sort((a, b) => {
        const targetA = attributes.value[a.targetAttrKey]
        const targetB = attributes.value[b.targetAttrKey]
        const aw = categoryWeight[targetA?.category || "other"] ?? 99
        const bw = categoryWeight[targetB?.category || "other"] ?? 99
        if (aw !== bw) return aw - bw
        const ao = Number(targetA?.order || 0)
        const bo = Number(targetB?.order || 0)
        if (ao !== bo) return ao - bo
        return a.targetAttrKey.localeCompare(b.targetAttrKey)
      })
      if (impacts.length > 0) {
        for (const impact of impacts) {
          const targetMeta = attributes.value[impact.targetAttrKey]
          const targetLabel =
            typeof targetMeta?.nameKey === "string" && targetMeta.nameKey
              ? t(targetMeta.nameKey)
              : t(`game.attr.${impact.targetAttrKey}`)
          const impactValue = formatAttributeValue(
            impact.deltaPerPoint,
            targetMeta?.unit === "percent" ? "percent" : "number",
            true
          )
          impactLines.push(t("game.attrPerPointImpactLine", { target: targetLabel, value: impactValue }))
        }
      }
      const breakdownLines = [...compositionLines, ...impactLines]
      const impactIntro = alchemyImpactLines.length > 0
        ? t("game.attrAlchemyImpactTitle")
        : t("game.attrPerPointImpactTitle")
      let valueSuffix: string | undefined
      if (DEFENSE_REDUCTION_KEYS.has(key) && rawValue > 0) {
        const pct = (rawValue / (DEFENSE_K + rawValue)) * 100
        valueSuffix = ` (${formatDecimal(pct, 1)}%)`
      }
      return {
        key,
        name: displayName,
        desc: localizedAttrDescription(key, meta, t),
        value,
        valueSuffix,
        rawValue,
        skillPointLevel: Number(meta?.skillPointLevel || 0),
        allowBattlePointLevelUp: Boolean(meta?.allowBattlePointLevelUp),
        compositionLines,
        impactIntro,
        impactLines,
        breakdownLines,
        growthType,
        category: meta?.category || "other",
        order: Number(meta?.order || 0),
        level: current.level || 1,
        exp: currentExp,
        nextExp,
        progress: nextExp > 0
          ? Math.max(0, Math.min(100, Math.round((currentExp / nextExp) * 100)))
          : 100
      }
    })
    return rows.sort((a, b) => {
      const aw = categoryWeight[a.category] ?? 99
      const bw = categoryWeight[b.category] ?? 99
      if (aw !== bw) return aw - bw
      if (a.order !== b.order) return a.order - b.order
      return a.key.localeCompare(b.key)
    })
  })

  return { attrRows }
}
