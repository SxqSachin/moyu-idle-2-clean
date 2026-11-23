<template>
  <GameDialog
    :model-value="modelValue"
    class="fish-selector-dialog"
    :title="title"
    width="min(980px, 96vw)"
    @update:model-value="emit('update:modelValue', $event)"
  >
    <div class="fish-selector">
      <p v-if="hint" class="fish-selector__hint"><Icon icon="lucide:info" />{{ hint }}</p>

      <FishFilterControls v-model="filters" :species="species" :mutations="mutations" />
      <div class="fish-selector__summary">
        <span>{{ t("game.fishing.fishSelectorResults", { count: filteredFish.length }) }}</span>
        <strong>{{ t("game.fishing.fishSelectorSelected", { selected: selectedIds.length, max: selectionLimit }) }}</strong>
      </div>

      <div v-if="filteredFish.length" ref="gridRef" class="fish-selector__grid">
        <button
          v-for="row in pageFish"
          :key="row.instanceId"
          type="button"
          class="fish-selector-card"
          :class="{ 'is-selected': isSelected(row.instanceId), 'is-disabled': selectionBlocked(row.instanceId) }"
          :style="{ '--fish-rarity': rarityColor(row) }"
          :aria-pressed="isSelected(row.instanceId)"
          :disabled="selectionBlocked(row.instanceId)"
          @click="toggle(row.instanceId)"
        >
          <span class="fish-selector-card__check"><Icon :icon="isSelected(row.instanceId) ? 'lucide:check' : 'lucide:plus'" /></span>
          <FishMutationArt :src="speciesIcon(row.fish.speciesId)" :fish="row.fish" :mutations="mutations" :animated="false" width="62px" />
          <span class="fish-selector-card__copy">
            <strong :title="fishName(row)">{{ fishName(row) }}</strong>
            <small v-if="customName(row)">{{ speciesName(row.fish.speciesId) }}</small>
            <RarityText :rarity-id="fishRarity(row)" />
          </span>
          <dl>
            <div><dt><Icon icon="lucide:ruler" />{{ t("game.fishing.length") }}</dt><dd>{{ formatLength(row.fish.lengthCm) }}<FishSizeIndicator metric="length" :value="row.fish.lengthCm" :species="speciesById.get(row.fish.speciesId)" /></dd></div>
            <div><dt><Icon icon="lucide:weight" />{{ t("game.fishing.weight") }}</dt><dd>{{ formatWeight(row.fish.weightKg) }}<FishSizeIndicator metric="weight" :value="row.fish.weightKg" :species="speciesById.get(row.fish.speciesId)" /></dd></div>
            <div><dt><Icon icon="lucide:heart-pulse" />{{ t("game.fishing.health") }}</dt><dd>{{ healthPercent(row) }}%</dd></div>
          </dl>
          <span class="fish-selector-card__meta">{{ sexLabel(row.fish.sex) }} · {{ formatAge(row.fish.ageDays) }}</span>
          <span v-if="ecologyWarning(row)" class="fish-selector-card__ecology" :class="{ 'is-predator': ecologyWarning(row)?.predator }">
            <Icon :icon="ecologyWarning(row)?.predator ? 'lucide:utensils' : 'lucide:shield-alert'" />
            {{ ecologyWarningLabel(row) }}
          </span>
        </button>
      </div>
      <div v-else class="fish-selector__empty">
        <Icon icon="lucide:search-x" />
        <span>{{ t("game.fishing.fishSelectorEmpty") }}</span>
      </div>
      <el-pagination v-if="filteredFish.length > pageSize" v-model:current-page="page" class="fish-selector__pagination" small layout="prev, pager, next" :pager-count="5" :page-size="pageSize" :total="filteredFish.length" />
    </div>

    <template #footer>
      <div class="fish-selector__footer">
        <span>{{ t("game.fishing.fishSelectorSelectedCount", { count: selectedIds.length }) }}</span>
        <div>
          <el-button @click="emit('update:modelValue', false)">{{ t("game.fishing.cancel") }}</el-button>
          <el-button type="primary" :loading="loading" :disabled="selectedIds.length === 0" @click="confirmSelection">{{ confirmLabel }}</el-button>
        </div>
      </div>
    </template>
  </GameDialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from "vue"
import { Icon } from "@iconify/vue"
import { useI18n } from "vue-i18n"
import GameDialog from "@components/game/common/GameDialog.vue"
import RarityText from "@components/game/common/RarityText.vue"
import FishMutationArt from "./FishMutationArt.vue"
import FishSizeIndicator from "./FishSizeIndicator.vue"
import { getRarityDefinition } from "@services/item-definition"
import { defaultFishFilters, filterAndSortFish, fishHealthPercent } from "@services/fish-filters"
import FishFilterControls from "./FishFilterControls.vue"
import { fishingPondPredationRisks, mergeFishingPondEcosystem } from "@services/fishing-pond-ecology"
import { localizedFishingText, type FishingMutationDefinition, type FishingPondFish, type FishingSpecies } from "@services/fishing"

const props = withDefaults(defineProps<{
  modelValue: boolean
  fish: FishingPondFish[]
  species: FishingSpecies[]
  title: string
  confirmLabel: string
  hint?: string
  multiple?: boolean
  maxSelection?: number
  initialSelectedIds?: number[]
  loading?: boolean
  ecologyFish?: FishingPondFish[]
  mutations?: FishingMutationDefinition[]
}>(), {
  hint: "",
  multiple: false,
  maxSelection: 1,
  initialSelectedIds: () => [],
  loading: false,
  ecologyFish: () => [],
  mutations: () => []
})

const emit = defineEmits<{
  (event: "update:modelValue", value: boolean): void
  (event: "confirm", instanceIds: number[]): void
}>()

const { t, locale } = useI18n()
const filters = ref(defaultFishFilters())
const selectedIds = ref<number[]>([])
const page = ref(1)
const gridRef = ref<HTMLElement | null>(null)
const pageSize = 36
const speciesById = computed(() => new Map(props.species.map((row) => [row.id, row])))
const selectionLimit = computed(() => Math.max(1, props.multiple ? props.maxSelection : 1))
const filteredFish = computed(() => filterAndSortFish(props.fish, filters.value, {
  fish: (row) => row.fish, name: fishName, speciesName,
  speciesById: speciesById.value, instanceId: (row) => row.instanceId, locale: locale.value
}))
const pageFish = computed(() => filteredFish.value.slice((page.value - 1) * pageSize, page.value * pageSize))
watch(filters, () => { page.value = 1 })
watch(page, () => { gridRef.value?.scrollTo({ top: 0 }) }, { flush: "post" })
watch(() => filteredFish.value.length, (count) => { page.value = Math.min(page.value, Math.max(1, Math.ceil(count / pageSize))) })
const ecologyWarnings = computed(() => {
  const selected = props.fish.filter((row) => selectedIds.value.includes(row.instanceId))
  const base = mergeFishingPondEcosystem(props.ecologyFish, selected)
  const warnings = new Map<number, { predator: boolean; predatorTargets: number; preyThreats: number }>()
  for (const row of pageFish.value) {
    const risks = fishingPondPredationRisks(mergeFishingPondEcosystem(base, [row]))
      .filter((risk) => risk.predator.instanceId === row.instanceId || risk.prey.instanceId === row.instanceId)
    const predatorTargets = new Set(risks.filter((risk) => risk.predator.instanceId === row.instanceId).map((risk) => risk.prey.instanceId)).size
    const preyThreats = new Set(risks.filter((risk) => risk.prey.instanceId === row.instanceId).map((risk) => risk.predator.instanceId)).size
    if (predatorTargets || preyThreats) warnings.set(row.instanceId, { predator: predatorTargets > 0, predatorTargets, preyThreats })
  }
  return warnings
})

watch(() => props.modelValue, (open) => {
  if (!open) return
  filters.value = defaultFishFilters()
  page.value = 1
  selectedIds.value = props.initialSelectedIds.filter((id) => props.fish.some((row) => row.instanceId === id)).slice(0, selectionLimit.value)
})

function toggle(instanceId: number) {
  if (isSelected(instanceId)) {
    selectedIds.value = selectedIds.value.filter((id) => id !== instanceId)
    return
  }
  if (!props.multiple) {
    selectedIds.value = [instanceId]
    return
  }
  if (selectedIds.value.length < selectionLimit.value) selectedIds.value = [...selectedIds.value, instanceId]
}
function isSelected(instanceId: number) { return selectedIds.value.includes(instanceId) }
function selectionBlocked(instanceId: number) { return !isSelected(instanceId) && selectedIds.value.length >= selectionLimit.value }
function confirmSelection() { emit("confirm", [...selectedIds.value]) }
function speciesName(speciesId: string) { const row = speciesById.value.get(speciesId); return localizedFishingText(row?.names, locale.value) || speciesId }
function speciesIcon(speciesId: string) { return speciesById.value.get(speciesId)?.iconPath || "/assets/icons/item/fish_catch.png" }
function customName(row: FishingPondFish) { return String(row.customData?.name || "").trim() }
function fishName(row: FishingPondFish) { return customName(row) || speciesName(row.fish.speciesId) }
function fishRarity(row: FishingPondFish) { return speciesById.value.get(row.fish.speciesId)?.rarity || "common" }
function rarityColor(row: FishingPondFish) { return getRarityDefinition(fishRarity(row))?.color || "#9ca3af" }
function healthPercent(row: FishingPondFish) { return fishHealthPercent(row.fish) }
function sexLabel(sex: string) { return t(`game.fishing.sex_${sex === "female" ? "female" : "male"}`) }
function formatLength(value: number) { return `${Number(value || 0).toFixed(1)} cm` }
function formatWeight(value: number) { return `${Number(value || 0).toFixed(value < 1 ? 3 : 1)} kg` }
function formatAge(days: number) { if (days >= 365) return t("game.fishing.ageYearsMonths", { years: Math.floor(days / 365), months: Math.floor(days % 365 / 30) }); if (days >= 30) return t("game.fishing.ageMonths", { months: Math.floor(days / 30) }); return t("game.fishing.ageDays", { days }) }
function ecologyWarning(row: FishingPondFish) { return ecologyWarnings.value.get(row.instanceId) }
function ecologyWarningLabel(row: FishingPondFish) { const warning = ecologyWarning(row); if (!warning) return ""; if (warning.predatorTargets && warning.preyThreats) return t("game.fishing.pondPredationConflict"); if (warning.predatorTargets) return t("game.fishing.pondPredationTargets", { count: warning.predatorTargets }); return t("game.fishing.pondPredationAtRisk") }
</script>

<style scoped>
.fish-selector { min-height: 360px; }
.fish-selector__pagination { justify-content: center; margin-top: 12px; }
.fish-selector__hint { display: flex; gap: 8px; align-items: flex-start; margin: 0 0 12px; color: var(--app-text-muted); font-size: 12px; line-height: 1.5; }
.fish-selector__hint svg { flex: 0 0 auto; margin-top: 2px; color: var(--el-color-primary); }
.fish-selector__summary { display: flex; justify-content: space-between; gap: 12px; margin: 14px 0 9px; color: var(--app-text-muted); font-size: 12px; }
.fish-selector__summary strong { color: var(--app-text); }
.fish-selector__grid { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 8px; max-height: 57vh; overflow: auto; padding: 1px 4px 4px 1px; }
.fish-selector-card { position: relative; display: grid; grid-template-columns: 62px minmax(0, 1fr); gap: 8px 10px; min-width: 0; padding: 10px; border: 1px solid var(--el-border-color); border-left: 3px solid var(--fish-rarity); border-radius: 6px; background: var(--el-fill-color-blank); color: inherit; text-align: left; cursor: pointer; }
.fish-selector-card:hover { border-color: color-mix(in srgb, var(--fish-rarity) 68%, var(--el-border-color)); }
.fish-selector-card.is-selected { background: color-mix(in srgb, var(--fish-rarity) 8%, var(--el-fill-color-blank)); box-shadow: 0 0 0 1px var(--fish-rarity); }
.fish-selector-card.is-disabled { opacity: .48; cursor: not-allowed; }
.fish-selector-card > .fish-mutation-art { width: 62px; height: 52px; align-self: center; filter: drop-shadow(0 4px 3px rgb(0 0 0 / 18%)); }
.fish-selector-card__check { position: absolute; top: 7px; right: 7px; display: grid; place-items: center; width: 22px; height: 22px; border: 1px solid var(--el-border-color); border-radius: 50%; background: var(--el-fill-color-blank); color: var(--app-text-muted); }
.is-selected .fish-selector-card__check { border-color: var(--fish-rarity); background: var(--fish-rarity); color: #fff; }
.fish-selector-card__copy { min-width: 0; display: grid; align-content: center; padding-right: 24px; }
.fish-selector-card__copy strong, .fish-selector-card__copy small { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.fish-selector-card__copy strong { font-size: 14px; }
.fish-selector-card__copy small { color: var(--app-text-muted); font-size: 11px; }
.fish-selector-card__copy :deep(.rarity-text) { margin-top: 2px; font-size: 11px; }
.fish-selector-card dl { grid-column: 1 / -1; display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 5px; margin: 0; padding-top: 8px; border-top: 1px solid var(--el-border-color-lighter); }
.fish-selector-card dl div { min-width: 0; display: grid; gap: 2px; }
.fish-selector-card dt { display: flex; align-items: center; gap: 3px; color: var(--app-text-muted); font-size: 9px; }
.fish-selector-card dd { overflow: hidden; margin: 0; font-size: 11px; font-weight: 700; text-overflow: ellipsis; white-space: nowrap; }
.fish-selector-card__meta { grid-column: 1 / -1; color: var(--app-text-muted); font-size: 10px; }
.fish-selector-card__ecology { grid-column: 1 / -1; display: inline-flex; align-items: center; gap: 4px; width: fit-content; color: var(--el-color-warning-dark-2); font-size: 10px; font-weight: 700; }.fish-selector-card__ecology.is-predator { color: var(--el-color-danger); }.fish-selector-card__ecology svg { flex: none; font-size: 12px; }
.fish-selector__empty { min-height: 260px; display: grid; place-content: center; justify-items: center; gap: 8px; color: var(--app-text-muted); }
.fish-selector__empty svg { font-size: 30px; }
.fish-selector__footer { display: flex; align-items: center; justify-content: space-between; gap: 16px; width: 100%; }
.fish-selector__footer > span { color: var(--app-text-muted); font-size: 12px; }
.fish-selector__footer > div { display: flex; gap: 8px; }
@media (max-width: 900px) { .fish-selector__grid { grid-template-columns: repeat(2, minmax(0, 1fr)); } }
@media (max-width: 600px) { .fish-selector__grid { grid-template-columns: 1fr; max-height: 52vh; }.fish-selector__footer { align-items: flex-end; }.fish-selector-card dl { grid-template-columns: repeat(3, minmax(0, 1fr)); } }
</style>
