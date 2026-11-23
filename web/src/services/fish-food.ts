import { getGameDataTable } from "@services/game-data-runtime"
import { getFishSpeciesDisplay } from "@services/fish-instance-display"
import { getItemDefinition } from "@services/item-definition"
import type { FishInstanceData, InventoryInstance } from "@/types/equipment"
import type { OperationEffectDefinitionView } from "@services/operation-effects"

export const FISH_FOOD_QUALITY_RULESET_V1 = "fish-food-quality-v1"
export const FISH_FOOD_QUALITY_RULESET_V2 = "fish-food-quality-v2"

export type FishFoodParameterScale = { length: number; weight: number; traits: number; min: number; max: number; inverse?: boolean; integer?: boolean; curve?: "size_growth" | "bounded_growth"; maxBeforeMutation?: number }

export type FishFoodKind = "grilled_fish" | "fish_soup"

export type FishFoodIngredientDefinition = {
  speciesId: string
  quantity: number
}

export type FishFoodRecipeDefinition = {
  id: string
  rulesetVersion: string
  actionId: string
  outputItemId: string
  kind: FishFoodKind
  qualityRulesetVersion: string
  allowSpecificIngredients: boolean
  fishIngredients: FishFoodIngredientDefinition[]
  parameterScaling?: Record<string, FishFoodParameterScale>
  lifetimeScaling?: { charges?: FishFoodParameterScale; durationSeconds?: FishFoodParameterScale }
  replacedEffectIds?: string[]
  legacyEffectIds?: string[]
  mutationLifetimeBonuses?: Record<string, { charges?: number; durationSeconds?: number }>
}

export type FishFoodIngredientQuality = {
  speciesId: string
  lengthScore: number
  weightScore: number
  traitAverage: number
  mutationCount: number
  multiplier: number
}

export type FishFoodQualityPreview = {
  multiplier: number
  lengthScore: number
  weightScore: number
  traitAverage: number
  mutationCount: number
  ingredients: FishFoodIngredientQuality[]
  specialMultiplier?: number
  specialMutations?: string[]
  extraCharges?: number
  extraDurationSeconds?: number
  parameterMultipliers?: Record<string, number>
}

export type FishFoodSpecificArgs = {
  ingredientMode: "specific"
  fishIngredientInstances: Record<string, number[]>
}

export function listFishFoodRecipes(): FishFoodRecipeDefinition[] {
  return getGameDataTable<FishFoodRecipeDefinition>("fish_food_recipes")
}

export function getFishFoodRecipeByActionId(actionId: string): FishFoodRecipeDefinition | null {
  const normalized = String(actionId || "").trim()
  return listFishFoodRecipes().find((row) => row.actionId === normalized) || null
}

export function getFishFoodEffectDefinition(recipe: FishFoodRecipeDefinition | null): OperationEffectDefinitionView | null {
  if (!recipe) return null
  const effectId = String(getItemDefinition(recipe.outputItemId)?.useOperationEffectId || "").trim()
  if (!effectId) return null
  return getGameDataTable<OperationEffectDefinitionView>("operation_effects").find((row) => row.id === effectId) || null
}

export function resolveFishFoodItemEffect(itemId: string, root?: Record<string, unknown> | null): OperationEffectDefinitionView | null {
  const recipe = listFishFoodRecipes().find((row) => row.outputItemId === itemId)
  if (!recipe) return null
  const current = getFishFoodEffectDefinition(recipe)
  if (!current) return null
  const projected = root ? projectReplacedFishFoodSnapshot(root) : null
  if (!root || !("operationEffect" in root)) return current
  const override = projected?.operationEffect as { definitionId?: string; rulesetVersion?: string; params?: Record<string, unknown>; lifetime?: OperationEffectDefinitionView["lifetime"] } | undefined
  if (!override || ![current.id, ...(recipe.legacyEffectIds || [])].includes(override.definitionId || "")) return null
  const definition = getGameDataTable<OperationEffectDefinitionView>("operation_effects").find(row => row.id === override.definitionId)
  if (!definition || definition.rulesetVersion !== override.rulesetVersion || !override.params) return null
  const legacyBulk = current.kind === "alchemy.infusion_bulk_capacity_floor_tradeoff" && definition.kind === "alchemy.infusion_capacity_floor_tradeoff"
  return { ...definition, stacking: current.stacking, allowedTargets: current.allowedTargets, kind: legacyBulk ? current.kind : definition.kind, params: override.params, lifetime: override.lifetime || definition.lifetime }
}

export function buildFishFoodSpecificArgs(
  recipe: FishFoodRecipeDefinition,
  bindings: Record<string, number[]>
): FishFoodSpecificArgs | undefined {
  const normalized: Record<string, number[]> = {}
  const seen = new Set<number>()
  for (const ingredient of recipe.fishIngredients || []) {
    const speciesId = String(ingredient.speciesId || "").trim()
    const required = Math.max(0, Math.trunc(Number(ingredient.quantity || 0)))
    const ids = (bindings[speciesId] || []).map((id) => Math.trunc(Number(id || 0))).filter((id) => id > 0)
    if (!speciesId || required <= 0 || ids.length !== required) return undefined
    for (const id of ids) {
      if (seen.has(id)) return undefined
      seen.add(id)
    }
    normalized[speciesId] = ids.slice().sort((a, b) => a - b)
  }
  if (Object.keys(normalized).length !== recipe.fishIngredients.length) return undefined
  return {
    ingredientMode: "specific",
    fishIngredientInstances: normalized
  }
}

export function resolveFishFoodQuality(
  recipe: FishFoodRecipeDefinition,
  selectedInstances: InventoryInstance[]
): FishFoodQualityPreview | null {
  const v2 = recipe.qualityRulesetVersion === FISH_FOOD_QUALITY_RULESET_V2
  if (!v2 && recipe.qualityRulesetVersion !== FISH_FOOD_QUALITY_RULESET_V1) return null
  const requiredCount = (recipe.fishIngredients || []).reduce(
    (total, ingredient) => total + Math.max(0, Math.trunc(Number(ingredient.quantity || 0))),
    0
  )
  if (requiredCount <= 0 || selectedInstances.length !== requiredCount) return null

  const remainingBySpecies = new Map<string, number>()
  for (const ingredient of recipe.fishIngredients || []) {
    remainingBySpecies.set(ingredient.speciesId, Math.max(0, Math.trunc(Number(ingredient.quantity || 0))))
  }
  const ingredients: FishFoodIngredientQuality[] = []
  const mutationIds = new Set<string>()
  for (const instance of selectedInstances) {
    const fish = instance.fishData
    const speciesId = String(fish?.speciesId || "").trim()
    const remaining = remainingBySpecies.get(speciesId) || 0
    if (!fish || remaining <= 0) return null
    remainingBySpecies.set(speciesId, remaining - 1)
    const quality = resolveFishIngredientQualityV1(fish)
    if (!quality) return null
    if (v2) {
      const species = getFishSpeciesDisplay(fish.speciesId)
      if (!species || !(fish.lengthCm > 0) || !(fish.weightKg > 0)) return null
      const condition = fish.weightKg / (species.shapeFactor * fish.lengthCm ** 3 / 100_000)
      quality.weightScore = round(clamp((condition - 0.75) / 0.5, 0, 1), 4)
      quality.multiplier = 1
    }
    ingredients.push(quality)
    for (const mutationId of fish.genes?.mutations || []) {
      const normalized = String(mutationId || "").trim()
      if (normalized) mutationIds.add(normalized)
    }
  }
  if ([...remainingBySpecies.values()].some((count) => count !== 0)) return null
  const count = ingredients.length
  const result: FishFoodQualityPreview = {
    multiplier: round(clamp(ingredients.reduce((sum, row) => sum + row.multiplier, 0) / count, 0.75, 1.35), 4),
    lengthScore: round(ingredients.reduce((sum, row) => sum + row.lengthScore, 0) / count, 4),
    weightScore: round(ingredients.reduce((sum, row) => sum + row.weightScore, 0) / count, 4),
    traitAverage: round(ingredients.reduce((sum, row) => sum + row.traitAverage, 0) / count, 4),
    mutationCount: mutationIds.size,
    ingredients
  }
  if (v2) {
    const specials = [["shimmering", 1.50], ["prismatic", 1.10], ["rainbow", 1.10]] as const
    result.specialMutations = specials.filter(([id]) => mutationIds.has(id)).map(([id]) => id)
    result.specialMultiplier = round(specials.reduce((total, [id, multiplier]) => total * (mutationIds.has(id) ? multiplier : 1), 1), 4)
    result.multiplier = result.specialMultiplier
    Object.assign(result, resolveFishFoodMutationLifetime(recipe, mutationIds))
    const definition = getFishFoodEffectDefinition(recipe)
    if (definition) applyFishFoodLifetimeScaling(recipe, definition, result)
    result.parameterMultipliers = Object.fromEntries(Object.entries(recipe.parameterScaling || {}).map(([name, scale]) => [name, round(parameterMultiplier(result, scale, Number(definition?.params[name] || 0)), 4)]))
  }
  return result
}

function parameterMultiplier(quality: FishFoodQualityPreview, scale: FishFoodParameterScale, base = 0): number {
  if (scale.curve === "bounded_growth" && base > 0) {
    const score = clamp(quality.lengthScore * scale.length + quality.weightScore * scale.weight + quality.traitAverage * scale.traits, 0, 1)
    return (base + ((scale.maxBeforeMutation || base) - base) * score ** 2) / base * (quality.specialMultiplier || 1)
  }
  if (scale.curve === "size_growth" && base > 0) {
    const score = clamp(quality.lengthScore * scale.length + quality.weightScore * scale.weight, 0, 1)
    const maximum = scale.maxBeforeMutation || base
    const grown = clamp(base * (1 + (maximum / base - 1) * score ** 2) * (1 + quality.traitAverage * scale.traits), scale.min, maximum)
    return grown / base * (quality.specialMultiplier || 1)
  }
  return round((1 + (quality.lengthScore - 0.5) * scale.length + (quality.weightScore - 0.5) * scale.weight + quality.traitAverage * scale.traits) * (quality.specialMultiplier || 1), 4)
}

export function resolveFishFoodMutationLifetime(recipe: FishFoodRecipeDefinition, mutations: Set<string>) {
  const bonuses = recipe.mutationLifetimeBonuses ?? { prismatic: { charges: 1 }, rainbow: { durationSeconds: 900 } }
  let extraCharges = 0, extraDurationSeconds = 0
  for (const [id, bonus] of Object.entries(bonuses)) {
    if (!mutations.has(id)) continue
    if (recipe.kind === "grilled_fish") extraCharges += bonus.charges || 0
    else extraDurationSeconds += bonus.durationSeconds || 0
  }
  return { extraCharges, extraDurationSeconds }
}

export function applyFishFoodLifetimeScaling(recipe: FishFoodRecipeDefinition, definition: OperationEffectDefinitionView, quality: FishFoodQualityPreview) {
  const charges = definition.lifetime.kind === "charges"
  const scale = charges ? recipe.lifetimeScaling?.charges : recipe.lifetimeScaling?.durationSeconds
  if (!scale) return
  const base = Number(charges ? definition.lifetime.charges : definition.lifetime.durationSeconds)
  const score = clamp(quality.lengthScore * scale.length + quality.weightScore * scale.weight + quality.traitAverage * scale.traits, 0, 1)
  const grown = base + ((scale.maxBeforeMutation || base) - base) * score ** 2
  const value = clamp(Math.floor(grown * (quality.specialMultiplier || 1)) + (charges ? quality.extraCharges || 0 : quality.extraDurationSeconds || 0), scale.min, scale.max) - base
  if (charges) quality.extraCharges = value
  else quality.extraDurationSeconds = value
}
export function resolveFishFoodLifetime(definition: OperationEffectDefinitionView, quality?: FishFoodQualityPreview | null) {
  return definition.lifetime.kind === "charges"
    ? { kind: "charges" as const, charges: Number(definition.lifetime.charges || 0) + (quality?.extraCharges || 0) }
    : { kind: "duration" as const, durationSeconds: Number(definition.lifetime.durationSeconds || 0) + (quality?.extraDurationSeconds || 0) }
}

// Display-only projection of explicitly replaced dishes. Consumption always
// resolves and validates the persisted snapshot again on the server.
export function projectReplacedFishFoodSnapshot(root: Record<string, unknown>): Record<string, unknown> {
  const food = root.fishFood as { schemaVersion?: number; recipeId?: string; qualityRulesetVersion?: string; quality?: FishFoodQualityPreview; ingredients?: { mutations?: string[] }[] } | undefined
  const effect = root.operationEffect as { definitionId?: string } | undefined
  const recipe = listFishFoodRecipes().find((row) => row.id === food?.recipeId && row.replacedEffectIds?.includes(effect?.definitionId || ""))
  if (!recipe) return root
  const definition = getFishFoodEffectDefinition(recipe)
  const q = food?.quality
  if (!definition || food?.schemaVersion !== 1 || food.qualityRulesetVersion !== FISH_FOOD_QUALITY_RULESET_V2 || !q || !Array.isArray(food.ingredients) || !food.ingredients.length ||
      ![q.lengthScore, q.weightScore, q.traitAverage, q.specialMultiplier].every(Number.isFinite) ||
      q.lengthScore < 0 || q.lengthScore > 1 || q.weightScore < 0 || q.weightScore > 1 || Math.abs(q.traitAverage) > 1 || (q.specialMultiplier || 0) < 1 || (q.specialMultiplier || 0) > 1.815) {
    return { ...root, operationEffect: undefined }
  }
  const mutations = new Set(food.ingredients.flatMap((row) => row.mutations || []))
  const quality = { ...q, ...resolveFishFoodMutationLifetime(recipe, mutations) }
  applyFishFoodLifetimeScaling(recipe, definition, quality)
  quality.parameterMultipliers = Object.fromEntries(Object.entries(recipe.parameterScaling || {}).map(([name, scale]) => [name, round(parameterMultiplier(quality, scale, Number(definition.params[name])), 4)]))
  return {
    ...root,
    fishFood: { ...food, quality },
    operationEffect: { schemaVersion: 2, definitionId: definition.id, rulesetVersion: definition.rulesetVersion, params: resolveFishFoodEffectParams(definition, quality, recipe), lifetime: resolveFishFoodLifetime(definition, quality) }
  }
}

export function resolveFishFoodEffectParams(
  definition: OperationEffectDefinitionView,
  multiplier: number | FishFoodQualityPreview = 1,
  recipe?: FishFoodRecipeDefinition
): Record<string, number> | null {
  if (typeof multiplier !== "number") {
    const params = Object.fromEntries(Object.entries(definition.params).map(([key, value]) => [key, Number(value)]))
    for (const [name, scale] of Object.entries(recipe?.parameterScaling || {})) {
      const factor = parameterMultiplier(multiplier, scale, params[name])
      const value = clamp(scale.inverse ? params[name] / factor : params[name] * factor, scale.min, scale.max)
      params[name] = scale.integer ? Math.round(value) : round(value, 4)
    }
    return params
  }
  const normalizedMultiplier = clamp(Number(multiplier || 1), 0.75, 1.35)
  if (definition.kind === "alchemy.infusion_capacity_floor_tradeoff") {
    const extraMaterialQuantity = Math.round(Number(definition.params?.extraMaterialQuantity || 0) * normalizedMultiplier)
    const positiveFloorReductionRate = round(
      Number(definition.params?.positiveFloorReductionRate || 0) / normalizedMultiplier,
      4
    )
    if (extraMaterialQuantity <= 0 || positiveFloorReductionRate <= 0) return null
    return { extraMaterialQuantity, positiveFloorReductionRate }
  }
  if (definition.kind === "alchemy.infusion_precision_ceiling_bonus") {
    const positiveCeilingBonusRate = round(
      Number(definition.params?.positiveCeilingBonusRate || 0) * normalizedMultiplier,
      4
    )
    if (positiveCeilingBonusRate <= 0) return null
    return { positiveCeilingBonusRate }
  }
  return Object.fromEntries(Object.entries(definition.params).map(([key, value]) => [key, Number(value)]))
}

function resolveFishIngredientQualityV1(fish: FishInstanceData): FishFoodIngredientQuality | null {
  const species = getFishSpeciesDisplay(fish.speciesId)
  if (!species || !(species.maxLengthCm > species.minLengthCm) || !(species.shapeFactor > 0)) return null
  const range = species.maxLengthCm - species.minLengthCm
  const lengthScore = clamp((Number(fish.lengthCm || 0) - species.minLengthCm) / range, 0, 1)
  const weight = Number(fish.weightKg || 0)
  const equivalentLength = weight > 0
    ? Math.cbrt(weight * 100_000 / species.shapeFactor)
    : species.minLengthCm
  const weightScore = clamp((equivalentLength - species.minLengthCm) / range, 0, 1)
  const genes = fish.genes || {}
  const traitValues = [
    genes.sizePotential,
    genes.growthRate,
    genes.feedEfficiency,
    genes.vitality,
    genes.lifespan,
    genes.fertility
  ].map((value) => clamp(Number(value || 0), -1, 1))
  const traitAverage = traitValues.reduce((sum, value) => sum + value, 0) / traitValues.length
  const mutations = new Set((genes.mutations || []).map((id) => String(id || "").trim()).filter(Boolean))
  const mutationCount = Math.min(3, mutations.size)
  const multiplier = clamp(
    1 + (lengthScore - 0.5) * 0.2 + (weightScore - 0.5) * 0.2 + traitAverage * 0.1 + mutationCount * 0.04,
    0.75,
    1.35
  )
  return {
    speciesId: fish.speciesId,
    lengthScore: round(lengthScore, 4),
    weightScore: round(weightScore, 4),
    traitAverage: round(traitAverage, 4),
    mutationCount: mutations.size,
    multiplier: round(multiplier, 4)
  }
}

function clamp(value: number, min: number, max: number): number {
  if (!Number.isFinite(value)) return min
  return Math.min(max, Math.max(min, value))
}

function round(value: number, precision: number): number {
  const scale = 10 ** precision
  return Math.round((value + Number.EPSILON) * scale) / scale
}
