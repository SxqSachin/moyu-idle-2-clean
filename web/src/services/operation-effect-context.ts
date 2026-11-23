import { isBulkInfusionMaterial, isPreciousInfusionMaterial, infusionMaterialTier } from "./alchemy-operation-effects"
import type { OperationEffectDomain, OperationEffectView } from "./operation-effects"
import { isPrimaryWork } from "./work-meal"

export type OperationEffectQuery = {
  domain: OperationEffectDomain
  actionId?: string
  mainType?: string
  subType?: string
  actorKind?: "player" | "kitty"
  actorId?: number
}

export type OperationEffectContext = OperationEffectQuery & {
  mainOutput?: string
  conversionInput?: boolean
  transmuteGoldEligible?: boolean
  enchantWeights?: Record<string, number>
  infusionMode?: "materials" | "fish"
  materialRarities?: readonly string[]
}

export type EffectEligibility = { applicable: true } | { applicable: false; reason: "conversionInput" | "transmuteInput" | "enchantCandidates" | "materialsRequired" | "materialRarity" | "basicMaterialRarity" | "refinedMaterialRarity" | "preciousMaterialRarity" | "fishInfusion" | "unsupported" }

export function operationEffectMatchesQuery(effect: OperationEffectView, query: OperationEffectQuery): boolean {
  if (effect.domain !== query.domain) return false
  const player = (query.actorKind || "player") === "player"
  const targetMatches = player
    ? ["player", "player_and_kitties"].includes(effect.targetKind)
    : ["all_kitties", "player_and_kitties"].includes(effect.targetKind) ||
      (effect.targetKind === "kitty" && Number(effect.targetActorId || 0) === Number(query.actorId || 0))
  if (!targetMatches) return false
  const selector = effect.selector || {}
  const allows = (values: string[] | undefined, value: string | undefined) => !values?.length || (!!value && values.includes(value))
  return allows(selector.actionIds, query.actionId) && allows(selector.mainTypes, query.mainType) && allows(selector.subTypes, query.subType)
}

type EligibilityRule = (context: OperationEffectContext, effect: OperationEffectView) => EffectEligibility
const applicable: EffectEligibility = { applicable: true }
const infusion = (context: OperationEffectContext): EffectEligibility => {
  if (context.infusionMode === "fish") return { applicable: false, reason: "fishInfusion" }
  if (!context.materialRarities?.length) return { applicable: false, reason: "materialsRequired" }
  return applicable
}

// UI-only eligibility mirrors registered domain handlers. Unknown rules are
// never advertised as active; settlement remains entirely server-authoritative.
const eligibilityRules: Readonly<Record<string, EligibilityRule>> = {
  "alchemy.conversion_loss_reduction": context => context.conversionInput ? applicable : { applicable: false, reason: "conversionInput" },
  "life_action.harvest_bonus": context => isPrimaryWork(context.mainType) && !!context.mainOutput ? applicable : { applicable: false, reason: "unsupported" },
  "life_action.focused_harvest": context => isPrimaryWork(context.mainType) && !!context.mainOutput ? applicable : { applicable: false, reason: "unsupported" },
  "life_action.patient_harvest": context => isPrimaryWork(context.mainType) && !!context.mainOutput ? applicable : { applicable: false, reason: "unsupported" },
  "life_action.work_experience": context => isPrimaryWork(context.mainType) && !!context.mainOutput ? applicable : { applicable: false, reason: "unsupported" },
  "alchemy.transmute_duration": () => applicable,
  "alchemy.transmute_gold_bonus": context => context.transmuteGoldEligible ? applicable : { applicable: false, reason: "transmuteInput" },
  "enchantment.high_rarity_weight": context => {
    const entries = Object.entries(context.enchantWeights || {}).filter(([, value]) => Number.isFinite(value) && value > 0)
    const isHighRarity = (rarity: string) => rarity === "epic" || rarity === "legendary"
    return entries.some(([rarity]) => isHighRarity(rarity)) && entries.some(([rarity]) => !isHighRarity(rarity))
      ? applicable : { applicable: false, reason: "enchantCandidates" }
  },
  "alchemy.infusion_capacity_floor_tradeoff": infusion,
  "alchemy.infusion_precision_ceiling_bonus": infusion,
  "alchemy.infusion_success_bonus": infusion,
  "alchemy.infusion_precious_capacity_bonus": context => {
    const eligibility = infusion(context)
    if (!eligibility.applicable) return eligibility
    return context.materialRarities!.some(isPreciousInfusionMaterial) ? applicable : { applicable: false, reason: "preciousMaterialRarity" }
  },
  "alchemy.infusion_bulk_capacity_floor_tradeoff": (context, effect) => {
    const eligibility = infusion(context)
    if (!eligibility.applicable) return eligibility
    const tier = Number(effect.params.materialTier || 0)
    if (tier === 1 || tier === 2) return context.materialRarities!.some(rarity => infusionMaterialTier(rarity) === tier)
      ? applicable : { applicable: false, reason: tier === 1 ? "basicMaterialRarity" : "refinedMaterialRarity" }
    return context.materialRarities!.some(isBulkInfusionMaterial) ? applicable : { applicable: false, reason: "materialRarity" }
  },
  "enhancement.success_bonus": () => applicable,
  "fishing_cast.bite_bonus": () => applicable,
  "fishing_cast.line_load_reduction": () => applicable,
  "fishing_cast.landing_bonus": () => applicable
}

export function operationEffectEligibility(effect: OperationEffectView, context: OperationEffectContext): EffectEligibility {
  return eligibilityRules[effect.kind]?.(context, effect) || { applicable: false, reason: "unsupported" }
}
