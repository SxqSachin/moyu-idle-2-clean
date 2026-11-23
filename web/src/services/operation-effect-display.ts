import type { OperationEffectDefinitionView } from "@services/operation-effects"
import { formatDurationI18n } from "@/utils/duration-format"

type Translate = (key: string, params?: Record<string, string | number>) => string

const operationNames: Record<string, string> = {
  "life_action.harvest_bonus": "primaryWork",
  "life_action.focused_harvest": "primaryWork",
  "life_action.patient_harvest": "primaryWork",
  "life_action.work_experience": "primaryWork",
  "alchemy.transmute_duration": "transmute",
  "alchemy.conversion_loss_reduction": "conversion",
  "alchemy.transmute_gold_bonus": "transmute",
  "alchemy.infusion_capacity_floor_tradeoff": "infusion",
  "alchemy.infusion_precision_ceiling_bonus": "infusion",
  "alchemy.infusion_success_bonus": "infusion",
  "alchemy.infusion_precious_capacity_bonus": "preciousInfusion",
  "enchantment.high_rarity_weight": "enchantment",
  "enhancement.success_bonus": "enhancement",
  "fishing_cast.bite_bonus": "cast",
  "fishing_cast.line_load_reduction": "cast",
  "fishing_cast.landing_bonus": "cast",
  "life_action.material_refund_chance": "lifeAction",
  "life_action.extra_main_output_chance": "lifeAction"
}

export function operationEffectLifetimeText(
  definition: Pick<OperationEffectDefinitionView, "kind" | "params" | "lifetime">,
  t: Translate
): string {
  let name = operationNames[definition.kind]
  if (definition.kind === "alchemy.infusion_bulk_capacity_floor_tradeoff") {
    const tier = Number(definition.params.materialTier || 0)
    name = tier === 1 ? "basicInfusion" : tier === 2 ? "refinedInfusion" : "bulkInfusion"
  }
  // Unknown business effects must supply their own wording instead of promising a scope.
  if (!name) return ""
  const operation = t(`game.fishCuisine.operations.${name}`)
  const lifetime = definition.lifetime
  return lifetime.kind === "charges"
    ? t("game.fishCuisine.effectCharges", { count: lifetime.charges || 0, operation })
    : t("game.fishCuisine.effectDuration", {
      time: formatDurationI18n(lifetime.durationSeconds || 0, t), operation
    })
}

export function operationEffectModifierLines(kind: string, params: Record<string, unknown>, t: Translate, includeRules = true, fractionDigits = 2): string[] {
  const number = (key: string) => Math.max(0, Number(params[key] || 0))
  const percent = (key: string) => (number(key) * 100).toFixed(Math.min(12, Math.max(0, fractionDigits))).replace(/(\.\d*?[1-9])0+$|\.0+$/, "$1")
  const capacity = (key: string) => number("quantityLimitBonusRate") > 0
    ? t(`${key}Breakthrough`, { count: Math.trunc(number("extraMaterialQuantity")), percent: percent("quantityLimitBonusRate") })
    : t(key, { count: Math.trunc(number("extraMaterialQuantity")) })
  const penalty = () => number("successRatePenalty") > 0 ? [t("game.fishCuisine.successPenalty", { percent: percent("successRatePenalty") })] : []
  const tierPrefix = number("materialTier") === 1 ? "basic" : number("materialTier") === 2 ? "refined" : "bulk"
  switch (kind) {
    case "alchemy.conversion_loss_reduction": return [t("game.fishCuisine.conversionLoss", { percent: percent("lossReductionRate") })]
    case "life_action.harvest_bonus":
    case "life_action.focused_harvest": return [t("game.fishCuisine.workHarvest", { percent: percent("chance") }), ...(number("durationReductionRate") > 0 ? [t("game.fishCuisine.workSpeed", { percent: percent("durationReductionRate") })] : [])]
    case "life_action.patient_harvest": return [t("game.fishCuisine.workHarvest", { percent: percent("chance") }), t("game.fishCuisine.workDuration", { percent: percent("durationPenaltyRate") })]
    case "life_action.work_experience": return [t("game.fishCuisine.workExperience", { percent: percent("experienceBonusRate") })]
case "alchemy.transmute_duration": return [t("game.fishCuisine.transmuteSpeed")]
case "alchemy.transmute_gold_bonus": return [t("game.fishCuisine.transmuteGold", { percent: percent("bonusRate") })]
case "enchantment.high_rarity_weight": return [t("game.fishCuisine.enchantWeight", { percent: percent("weightBonusRate") })]
    case "alchemy.infusion_precious_capacity_bonus": return [
      capacity("game.fishCuisine.preciousCapacity"),
      ...penalty(),
      ...(includeRules ? [t("game.fishCuisine.preciousLimit")] : [])
    ]
    case "alchemy.infusion_bulk_capacity_floor_tradeoff": return [
      capacity(`game.fishCuisine.${tierPrefix}Capacity`),
      t("game.fishFood.positiveFloorReduction", { percent: percent("positiveFloorReductionRate") }),
      ...penalty(),
      ...(includeRules ? [t(`game.fishCuisine.${tierPrefix}Limit`)] : [])
    ]
    case "alchemy.infusion_capacity_floor_tradeoff": return [
      t("game.fishFood.extraMaterialQuantity", { count: Math.trunc(number("extraMaterialQuantity")) }),
      t("game.fishFood.positiveFloorReduction", { percent: percent("positiveFloorReductionRate") })
    ]
    case "alchemy.infusion_precision_ceiling_bonus": return [t("game.fishFood.positiveCeilingBonus", { percent: percent("positiveCeilingBonusRate") })]
    case "alchemy.infusion_success_bonus": return [t("game.fishCuisine.infusionSuccess", { percent: percent("bonusRate") })]
    case "enhancement.success_bonus": return [t("game.fishCuisine.enhance", { percent: percent("bonusRate") })]
    case "fishing_cast.bite_bonus": return [t("game.fishCuisine.bite", { percent: percent("bonusRate") })]
    case "fishing_cast.line_load_reduction": return [t("game.fishCuisine.lineLoad", { percent: percent("rate") })]
    case "fishing_cast.landing_bonus": return [t("game.fishCuisine.landing", { percent: percent("rate") })]
    case "life_action.material_refund_chance": return [t("game.fishCuisine.refund", { percent: percent("chance"), rate: percent("refundRate") })]
    case "life_action.extra_main_output_chance": return [t("game.fishCuisine.extra", { percent: percent("chance"), count: number("quantity") })]
    default: return []
  }
}
