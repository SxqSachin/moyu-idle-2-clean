<template>
  <section v-if="effectDefinition || foodSnapshot" class="fish-food-instance-info">
    <strong>{{ t('game.itemDetailEffect') }}</strong>
    <div v-if="lifetimeText" class="fish-food-instance-info__lifetime">{{ lifetimeText }}</div>
    <OperationEffectAudience v-if="effectDefinition" :targets="effectDefinition.allowedTargets" :lifetime-kind="effectDefinition.lifetime.kind" />
    <div v-if="effectLines.length" class="fish-food-instance-info__effects">
      <div v-for="line in effectLines" :key="line"><Icon icon="lucide:sparkles" /><span>{{ line }}</span></div>
    </div>
    <details v-if="foodSnapshot" :key="detailKey" class="fish-food-instance-info__details">
      <summary class="fish-food-instance-info__heading">
        <span>{{ t("game.fishCuisine.qualityDetails") }}</span>
        <strong>{{ t(foodSnapshot.quality.parameterMultipliers ? "game.fishCuisine.specialMultiplier" : "game.fishFood.qualityMultiplier", { value: foodSnapshot.quality.multiplier.toFixed(4) }) }}</strong>
        <Icon icon="lucide:chevron-down" />
      </summary>
    <FishFoodQualityBreakdown v-if="foodSnapshot.quality.parameterMultipliers" :quality="foodSnapshot.quality" />
    <div v-else class="fish-food-instance-info__quality">
      <span>{{ t("game.fishFood.qualityLength", { value: formatScore(foodSnapshot.quality.lengthScore) }) }}</span>
      <span>{{ t("game.fishFood.qualityWeight", { value: formatScore(foodSnapshot.quality.weightScore) }) }}</span>
      <span>{{ t("game.fishFood.qualityTraits", { value: formatSigned(foodSnapshot.quality.traitAverage) }) }}</span>
      <span>{{ t("game.fishFood.qualityMutations", { count: foodSnapshot.quality.mutationCount }) }}</span>
    </div>

    <div class="fish-food-instance-info__ingredients">
      <div v-for="(ingredient, index) in foodSnapshot.ingredients" :key="`${ingredient.speciesId}-${index}`">
        <FishMutationArt
          v-if="fishSpeciesIcon(ingredient.speciesId)"
          :src="fishSpeciesIcon(ingredient.speciesId)"
          :alt="fishSpeciesLabel(ingredient.speciesId)"
          :fish="{ mutations: ingredient.mutations }"
          width="42px"
        />
        <div class="min-w-0">
          <strong>{{ fishSpeciesLabel(ingredient.speciesId) }}</strong>
          <span>{{ t("game.fishFood.selectedMetrics", { length: ingredient.lengthCm.toFixed(1), weight: formatWeight(ingredient.weightKg) }) }}</span>
        </div>
      </div>
    </div>
    </details>
  </section>
</template>

<script setup lang="ts">
import { operationEffectLifetimeText, operationEffectModifierLines } from "@services/operation-effect-display"
import { projectReplacedFishFoodSnapshot, resolveFishFoodItemEffect } from "@services/fish-food"
import type { OperationEffectDefinitionView } from "@services/operation-effects"
import FishFoodQualityBreakdown from "./FishFoodQualityBreakdown.vue"
import OperationEffectAudience from "@components/game/operation-effect/OperationEffectAudience.vue"
import { computed } from "vue"
import { Icon } from "@iconify/vue"
import { useI18n } from "vue-i18n"
import FishMutationArt from "@components/game/fishing/FishMutationArt.vue"
import { getFishSpeciesDisplay, localizedFishText } from "@services/fish-instance-display"

type FishFoodIngredientSnapshot = {
  speciesId: string
  lengthCm: number
  weightKg: number
  mutations: string[]
}

type FishFoodSnapshot = {
  schemaVersion: number
  quality: {
    multiplier: number
    lengthScore: number
    weightScore: number
    traitAverage: number
    mutationCount: number
    parameterMultipliers?: Record<string, number>
  }
  ingredients: FishFoodIngredientSnapshot[]
}

const props = defineProps<{
  itemId?: string
  authoritativeEffect?: OperationEffectDefinitionView | null
  baseSnapshot?: unknown
}>()

const { t, locale } = useI18n()

const rootSnapshot = computed<Record<string, unknown> | null>(() => {
  const value = props.baseSnapshot
  if (!value || typeof value !== "object" || Array.isArray(value)) return null
  return projectReplacedFishFoodSnapshot(value as Record<string, unknown>)
})

const foodSnapshot = computed<FishFoodSnapshot | null>(() => {
  const raw = rootSnapshot.value?.fishFood
  if (!raw || typeof raw !== "object" || Array.isArray(raw)) return null
  const value = raw as Record<string, unknown>
  const qualityRaw = value.quality
  const ingredientsRaw = value.ingredients
  if (Number(value.schemaVersion) !== 1 || !qualityRaw || typeof qualityRaw !== "object" || Array.isArray(qualityRaw) || !Array.isArray(ingredientsRaw)) {
    return null
  }
  const quality = qualityRaw as Record<string, unknown>
  const ingredients = ingredientsRaw
    .map((row): FishFoodIngredientSnapshot | null => {
      if (!row || typeof row !== "object" || Array.isArray(row)) return null
      const entry = row as Record<string, unknown>
      const speciesId = String(entry.speciesId || "").trim()
      const lengthCm = Number(entry.lengthCm)
      const weightKg = Number(entry.weightKg)
      if (!speciesId || !Number.isFinite(lengthCm) || !Number.isFinite(weightKg)) return null
      return {
        speciesId,
        lengthCm,
        weightKg,
        mutations: Array.isArray(entry.mutations)
          ? entry.mutations.map((id) => String(id || "").trim()).filter(Boolean)
          : []
      }
    })
    .filter((row): row is FishFoodIngredientSnapshot => !!row)
  const parsed: FishFoodSnapshot = {
    schemaVersion: 1,
    quality: {
      multiplier: Number(quality.multiplier),
      lengthScore: Number(quality.lengthScore),
      weightScore: Number(quality.weightScore),
      traitAverage: Number(quality.traitAverage),
      mutationCount: Math.max(0, Math.trunc(Number(quality.mutationCount || 0))),
      parameterMultipliers: quality.parameterMultipliers && typeof quality.parameterMultipliers === "object" && !Array.isArray(quality.parameterMultipliers)
        ? Object.fromEntries(Object.entries(quality.parameterMultipliers).map(([key, value]) => [key, Number(value)]).filter(([, value]) => typeof value === "number" && Number.isFinite(value))) as Record<string, number>
        : undefined
    },
    ingredients
  }
  if (!Number.isFinite(parsed.quality.multiplier) ||
      !Number.isFinite(parsed.quality.lengthScore) ||
      !Number.isFinite(parsed.quality.weightScore) ||
      !Number.isFinite(parsed.quality.traitAverage) ||
      parsed.ingredients.length === 0) {
    return null
  }
  return parsed
})

const detailKey = computed(() => `${props.itemId}:${JSON.stringify(props.baseSnapshot || {})}`)
const effectDefinition = computed(() => props.authoritativeEffect || resolveFishFoodItemEffect(props.itemId || "", rootSnapshot.value))
const effectLines = computed(() => effectDefinition.value ? operationEffectModifierLines(effectDefinition.value.kind, effectDefinition.value.params, t) : [])
const lifetimeText = computed(() => {
  const def = effectDefinition.value
  if (!def) return ""
  return operationEffectLifetimeText(def, t)
})

function fishSpeciesLabel(speciesID: string): string {
  const species = getFishSpeciesDisplay(speciesID)
  return localizedFishText(species?.names, locale.value, speciesID)
}

function fishSpeciesIcon(speciesID: string): string {
  return getFishSpeciesDisplay(speciesID)?.iconPath || ""
}

function formatScore(value: number): string {
  return `${Math.round(Math.max(0, Math.min(1, value)) * 100)}%`
}

function formatSigned(value: number): string {
  const normalized = Number.isFinite(value) ? value : 0
  return `${normalized >= 0 ? "+" : ""}${normalized.toFixed(2)}`
}

function formatWeight(value: number): string {
  return value.toFixed(value >= 10 ? 1 : 2)
}
</script>

<style scoped>
.fish-food-instance-info {
  display: grid;
  gap: 9px;
  padding: 11px 0;
  border-block: 1px solid var(--app-border);
}

.fish-food-instance-info__heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  color: var(--app-text-secondary);
  font-size: 12px;
}

.fish-food-instance-info__details summary { cursor: pointer; list-style: none; padding: 8px 0; }
.fish-food-instance-info__details summary::-webkit-details-marker { display: none; }
.fish-food-instance-info__details summary > span { flex: 1; }
.fish-food-instance-info__details summary > svg { flex: 0 0 auto; transition: transform 150ms; }
.fish-food-instance-info__details[open] summary > svg { transform: rotate(180deg); }
.fish-food-instance-info__details summary:focus-visible { outline: 2px solid var(--app-primary); outline-offset: 2px; }
.fish-food-instance-info__lifetime { font-size: 13px; color: var(--app-text-secondary); }
.fish-food-instance-info__details .fish-food-instance-info__ingredients { margin-top: 10px; }

.fish-food-instance-info__heading strong {
  color: var(--app-primary);
  font-size: 13px;
  font-variant-numeric: tabular-nums;
}

.fish-food-instance-info__quality {
  display: flex;
  flex-wrap: wrap;
  gap: 5px 10px;
  color: var(--app-muted);
  font-size: 11px;
}

.fish-food-instance-info__effects {
  display: grid;
  gap: 5px;
  color: var(--app-text-secondary);
  font-size: 12px;
}

.fish-food-instance-info__effects > div {
  display: flex;
  align-items: flex-start;
  gap: 6px;
}

.fish-food-instance-info__effects svg {
  flex: 0 0 auto;
  margin-top: 2px;
  color: var(--app-success);
}

.fish-food-instance-info__ingredients {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px;
}

.fish-food-instance-info__ingredients > div {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 6px;
}

.fish-food-instance-info__ingredients strong,
.fish-food-instance-info__ingredients span {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.fish-food-instance-info__ingredients strong {
  color: var(--app-text);
  font-size: 11px;
}

.fish-food-instance-info__ingredients span {
  margin-top: 2px;
  color: var(--app-muted);
  font-size: 10px;
  font-variant-numeric: tabular-nums;
}

@media (max-width: 420px) {
  .fish-food-instance-info__ingredients {
    grid-template-columns: 1fr;
  }
}
</style>
