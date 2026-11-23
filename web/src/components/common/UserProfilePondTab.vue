<template>
  <div class="profile-pond">
    <div v-if="loading" class="profile-pond__state" aria-live="polite">
      <Icon icon="lucide:loader-circle" class="is-spinning" />
      <span>{{ t("game.fishing.pondLoading") }}</span>
    </div>

    <div v-else-if="loadFailed" class="profile-pond__state">
      <Icon icon="lucide:waves" />
      <span>{{ t("game.userProfilePondLoadFailed") }}</span>
      <el-button circle :aria-label="t('game.fishing.refresh')" :title="t('game.fishing.refresh')" @click="loadPond">
        <Icon icon="lucide:refresh-cw" />
      </el-button>
    </div>

    <div v-else-if="pond && !pond.built" class="profile-pond__state">
      <Icon icon="lucide:construction" />
      <span>{{ t("game.userProfilePondNotBuilt") }}</span>
    </div>

    <template v-else-if="pond">
      <section class="profile-pond__status" aria-live="polite">
        <div>
          <Icon icon="lucide:fish-symbol" />
          <span>{{ t("game.fishing.pondLevel") }}</span>
          <strong>{{ pond.level }}</strong>
        </div>
        <div>
          <Icon icon="lucide:fish" />
          <span>{{ t("game.fishing.pondMainZone") }}</span>
          <strong>{{ mainPondFish.length }} / {{ pond.limits.fishCapacity }}</strong>
        </div>
        <div>
          <Icon icon="game-icons:fish-eggs" />
          <span>{{ t("game.fishing.pondNurseryZone") }}</span>
          <strong>{{ nurseryPondFish.length }} / {{ pond.limits.nurseryCapacity }}</strong>
        </div>
        <div>
          <Icon icon="lucide:weight" />
          <span>{{ t("game.fishing.pondBiomass") }}</span>
          <strong>{{ formatWeight(pond.biomassKg) }} / {{ formatWeight(pond.limits.biomassKg) }}</strong>
        </div>
      </section>

      <div class="profile-pond__zone-switch">
        <el-segmented v-model="activePondZone" :options="pondZoneOptions" />
        <span v-if="activePondZone === 'nursery'"><Icon icon="lucide:shield-check" />{{ t("game.fishing.pondNurseryProtected") }}</span>
      </div>

      <PondWaterScene
        data-testid="public-pond-water"
        :fish="visiblePondFish"
        :species="species"
        :mutations="pond.mutations || []"
        :capabilities="visitorCapabilities"
      />

      <header class="profile-pond__heading">
        <span>{{ t(activePondZone === 'main' ? "game.fishing.pondMainZone" : "game.fishing.pondNurseryZone") }}</span>
        <strong>{{ t("game.fishing.pondFishCount", { count: visiblePondFish.length }) }}</strong>
      </header>

      <div v-if="visiblePondFish.length" class="profile-pond__fish-list">
        <article v-for="row in visiblePondFish" :key="row.instanceId" :class="[mutationClasses(row), { 'is-dead': row.fish.lifeState === 'dead' }]" :data-fish-mutations="mutationIds(row).join(' ') || undefined">
          <FishMutationArt :src="speciesIcon(row.fish.speciesId)" :fish="row.fish" :mutations="pond.mutations || []" :dead="row.fish.lifeState === 'dead'" :animated="false" width="50px" />
          <div class="profile-pond__fish-copy">
            <div class="profile-pond__fish-title">
              <strong :class="itemCustomLabelClass(row.customData)">{{ fishName(row) }}</strong>
              <RarityText :rarity-id="fishRarity(row)" />
            </div>
            <small v-if="customName(row)">{{ speciesName(row.fish.speciesId) }}</small>
            <small>{{ sexLabel(row.fish.sex) }} · {{ t("game.fishing.pondGeneration", { value: row.fish.generation || 0 }) }} · {{ formatAge(row.fish.ageDays) }}</small>
            <small>{{ formatLength(row.fish.lengthCm) }}<FishSizeIndicator metric="length" :value="row.fish.lengthCm" :species="speciesById.get(row.fish.speciesId)" /> · {{ formatWeight(row.fish.weightKg) }}<FishSizeIndicator metric="weight" :value="row.fish.weightKg" :species="speciesById.get(row.fish.speciesId)" /></small>
            <div class="profile-pond__tags">
              <el-tag size="small" :type="row.mature ? 'success' : 'info'" effect="plain">
                {{ t(row.mature ? "game.fishing.pondMature" : "game.fishing.pondImmature") }}
              </el-tag>
              <el-tag v-if="row.fish.lifeState === 'dead'" size="small" type="danger" effect="dark">
                {{ t("game.fishing.pondDead") }}
              </el-tag>
              <el-tag v-for="trait in row.phenotypes || []" :key="trait" size="small" type="warning" effect="plain">
                {{ phenotypeLabel(trait) }}
              </el-tag>
              <el-tooltip
                v-for="mutation in mutationRows(row)"
                :key="mutation.id"
                :content="mutationTooltip(mutation)"
                placement="top"
                popper-class="profile-pond-mutation-popper"
              >
                <el-tag tabindex="0" size="small" effect="plain" class="profile-pond__mutation-tag">
                  <div class="flex">
                    <Icon icon="lucide:sparkles" />{{ t(mutation.nameKey) }}
                  </div>
                </el-tag>
              </el-tooltip>
            </div>
          </div>
        </article>
      </div>

      <div v-else class="profile-pond__state profile-pond__state--empty">
        <Icon icon="lucide:fish-off" />
        <span>{{ t(activePondZone === 'main' ? "game.fishing.pondEmpty" : "game.fishing.pondNurseryEmpty") }}</span>
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from "vue"
import { Icon } from "@iconify/vue"
import { useI18n } from "vue-i18n"
import { useGameStore } from "@stores/game"
import { getGameDataTable } from "@services/game-data-runtime"
import {
  fetchPlayerFishingPond,
  fishingPondZone,
  localizedFishingText,
  type FishingPondPublicOverview,
  type FishingPondVisibleFish,
  type FishingPondZone,
  type FishingSpecies
} from "@services/fishing"
import { restrictFishingPondCapabilitiesToReadOnly } from "@services/fishing-pond-access"
import { itemCustomLabelClass } from "@services/item-customization"
import { fishMutationClassNames, fishMutationIds, resolveFishMutations, type ResolvedFishMutation } from "@services/fish-mutations"
import PondWaterScene from "@components/game/fishing/PondWaterScene.vue"
import FishMutationArt from "@components/game/fishing/FishMutationArt.vue"
import FishSizeIndicator from "@components/game/fishing/FishSizeIndicator.vue"
import RarityText from "@components/game/common/RarityText.vue"

const props = defineProps<{ userId: number }>()
const { t, locale } = useI18n()
const gameStore = useGameStore()
const pond = ref<FishingPondPublicOverview | null>(null)
const activePondZone = ref<FishingPondZone>("main")
const loading = ref(false)
const loadFailed = ref(false)
let requestSequence = 0

const species = computed(() => getGameDataTable<FishingSpecies>("fishing_species"))
const speciesById = computed(() => new Map(species.value.map((row) => [row.id, row])))
const visitorCapabilities = computed(() => restrictFishingPondCapabilitiesToReadOnly(pond.value?.capabilities || []))
const mainPondFish = computed(() => (pond.value?.fish || []).filter((row) => fishingPondZone(row.fish) === "main"))
const nurseryPondFish = computed(() => (pond.value?.fish || []).filter((row) => fishingPondZone(row.fish) === "nursery"))
const visiblePondFish = computed(() => activePondZone.value === "main" ? mainPondFish.value : nurseryPondFish.value)
const pondZoneOptions = computed(() => [
  { value: "main", label: `${t("game.fishing.pondMainZone")} ${mainPondFish.value.length}/${pond.value?.limits.fishCapacity || 0}` },
  { value: "nursery", label: `${t("game.fishing.pondNurseryZone")} ${nurseryPondFish.value.length}/${pond.value?.limits.nurseryCapacity || 0}` }
])

watch(
  () => [props.userId, gameStore.token] as const,
  () => loadPond(),
  { immediate: true }
)

onBeforeUnmount(() => {
  requestSequence++
})

async function loadPond() {
  const token = gameStore.token
  const userId = props.userId
  const requestId = ++requestSequence
  pond.value = null
  loadFailed.value = false
  loading.value = false
  if (!token || userId <= 0) return
  loading.value = true
  try {
    const result = await fetchPlayerFishingPond(token, userId)
    if (result.ownerUserId !== userId) throw new Error("pond owner mismatch")
    if (requestId === requestSequence) pond.value = result
  } catch {
    if (requestId === requestSequence) loadFailed.value = true
  } finally {
    if (requestId === requestSequence) loading.value = false
  }
}

function customName(row: FishingPondVisibleFish) {
  return String(row.customData?.name || "").trim()
}

function fishName(row: FishingPondVisibleFish) {
  return customName(row) || speciesName(row.fish.speciesId)
}

function speciesName(speciesId: string) {
  const row = speciesById.value.get(speciesId)
  return localizedFishingText(row?.names, locale.value) || speciesId
}

function speciesIcon(speciesId: string) {
  return speciesById.value.get(speciesId)?.iconPath || "/assets/icons/item/fish_catch.png"
}

function fishRarity(row: FishingPondVisibleFish) {
  return speciesById.value.get(row.fish.speciesId)?.rarity || "common"
}

function sexLabel(sex: string) {
  return t(sex === "female" ? "game.fishing.sex_female" : "game.fishing.sex_male")
}

function phenotypeLabel(trait: string) {
  const key = `game.fishing.pondPhenotype_${trait}`
  const translated = t(key)
  return translated === key ? trait : translated
}

function mutationRows(row: FishingPondVisibleFish) {
  return resolveFishMutations(row.fish, pond.value?.mutations)
}

function mutationIds(row: FishingPondVisibleFish) {
  return fishMutationIds(row.fish)
}

function mutationClasses(row: FishingPondVisibleFish) {
  return fishMutationClassNames(row.fish, pond.value?.mutations)
}

function mutationTooltip(mutation: ResolvedFishMutation) {
  const details = mutation.definition
  if (!details) return t(mutation.descriptionKey)
  return `${t(mutation.descriptionKey)} · ${t("game.fishing.pondMutationEffects", {
    lifespan: Math.round((details.lifespanMultiplier - 1) * 100),
    alchemy: Math.round((details.alchemy.primaryValueMultiplier - 1) * 100)
  })}`
}

function formatAge(value: number) {
  return t("game.fishing.ageDays", { days: Math.max(0, Math.floor(Number(value) || 0)) })
}

function formatLength(value: number) {
  return `${Number(value || 0).toFixed(1)} cm`
}

function formatWeight(value: number) {
  const normalized = Number(value || 0)
  return `${normalized.toFixed(normalized < 1 ? 3 : 1)} kg`
}
</script>

<style scoped>
.profile-pond { min-width: 0; display: grid; gap: 14px; padding-top: 8px; container-type: inline-size; }
.profile-pond > * { min-width: 0; }
.profile-pond__status { display: grid; grid-template-columns: .7fr 1fr 1fr 1.4fr; border-block: 1px solid var(--app-border); }
.profile-pond__status > div { min-width: 0; display: grid; grid-template-columns: auto minmax(0, 1fr); align-items: center; gap: 2px 8px; padding: 10px 12px; }
.profile-pond__status > div + div { border-left: 1px solid var(--app-border); }
.profile-pond__status svg { grid-row: 1 / 3; color: #287d68; font-size: 21px; }
.profile-pond__status span { overflow: hidden; color: var(--app-muted); font-size: 11px; text-overflow: ellipsis; white-space: nowrap; }
.profile-pond__status strong { min-width: 0; overflow: hidden; font-size: 14px; text-overflow: ellipsis; white-space: nowrap; }
.profile-pond__zone-switch { display: flex; align-items: center; justify-content: space-between; gap: 10px; }.profile-pond__zone-switch :deep(.el-segmented) { width: min(100%, 420px); }.profile-pond__zone-switch :deep(.el-segmented__group) { width: 100%; }.profile-pond__zone-switch :deep(.el-segmented__item) { min-width: 0; flex: 1; }.profile-pond__zone-switch > span { display: flex; align-items: center; gap: 5px; color: var(--app-muted); font-size: 11px; }.profile-pond__zone-switch > span svg { color: #287d68; }
.profile-pond :deep(.pond-water) { min-height: 240px; height: min(320px, 38vh); }
.profile-pond__heading { display: flex; align-items: end; justify-content: space-between; gap: 12px; padding-top: 2px; }
.profile-pond__heading span { color: var(--app-muted); font-size: 11px; }
.profile-pond__heading strong { font-size: 14px; }
.profile-pond__fish-list { min-width: 0; width: 100%; display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); align-items: stretch; gap: 8px; }
.profile-pond__fish-list article { position: relative; min-width: 0; display: grid; grid-template-columns: 50px minmax(0, 1fr); align-items: start; gap: 8px; overflow: hidden; padding: 9px; border: 1px solid var(--app-border); border-radius: 6px; background: var(--app-surface); isolation: isolate; }
.profile-pond__fish-list article.is-dead { opacity: .68; }
.profile-pond__fish-list article > .fish-mutation-art { align-self: start; height: 38px; }
.profile-pond__fish-list article.has-fish-mutation { border-color: color-mix(in srgb, #68bed2 52%, var(--app-border)); }
.profile-pond__fish-list article.fish-mutation--prismatic { box-shadow: inset 3px 0 0 rgb(94 209 226 / 72%); }
.profile-pond__fish-list article.fish-mutation--rainbow::before { content: ""; position: absolute; z-index: -1; inset: 0 0 auto; height: 3px; background: linear-gradient(90deg, #f16b7d, #f1c453, #55bd8a, #559bd8, #ad75d4); opacity: .76; }
.profile-pond__fish-list article.fish-mutation--shimmering { box-shadow: inset 0 0 0 1px rgb(246 203 94 / 44%); }
.profile-pond__fish-list article.has-stacked-fish-mutations { background: color-mix(in srgb, var(--app-surface) 95%, #d8f5f4); }
.profile-pond__fish-copy { min-width: 0; display: grid; align-content: start; gap: 3px; }
.profile-pond__fish-title { min-width: 0; display: flex; align-items: baseline; gap: 7px; }
.profile-pond__fish-title strong { min-width: 0; overflow: hidden; font-size: 13px; text-overflow: ellipsis; white-space: nowrap; }
.profile-pond__fish-title :deep(.rarity-text) { flex: none; font-size: 10px; }
.profile-pond__fish-copy > small { overflow: hidden; color: var(--app-muted); font-size: 10px; text-overflow: ellipsis; white-space: nowrap; }
.profile-pond__tags { display: flex; flex-wrap: wrap; gap: 4px; margin-top: 3px; }
.profile-pond__tags :deep(.el-tag) { max-width: 100%; }
.profile-pond__tags :deep(.profile-pond__mutation-tag) { border-color: color-mix(in srgb, #4daab6 46%, var(--app-border)); color: #267884; }
.profile-pond__mutation-tag :deep(svg) { margin-right: 3px; }
:global(.profile-pond-mutation-popper.el-popper) { max-width: min(320px, calc(100vw - 24px)); line-height: 1.5; overflow-wrap: anywhere; text-align: left; white-space: normal; }
.profile-pond__state { min-height: 190px; display: flex; align-items: center; justify-content: center; gap: 9px; color: var(--app-muted); font-size: 13px; text-align: center; }
.profile-pond__state > svg { flex: none; color: #287d68; font-size: 24px; }
.profile-pond__state .is-spinning { animation: profile-pond-spin 1s linear infinite; }
.profile-pond__state--empty { min-height: 90px; border-top: 1px solid var(--app-border); }
@keyframes profile-pond-spin { to { transform: rotate(360deg); } }
@container (max-width: 520px) {
  .profile-pond__fish-list { grid-template-columns: minmax(0, 1fr); }
}
@media (max-width: 640px) {
  .profile-pond__status { grid-template-columns: 1fr 1fr; }
  .profile-pond__status > div { padding: 9px 8px; }
  .profile-pond__status > div:nth-child(3) { border-top: 1px solid var(--app-border); border-left: 0; }
  .profile-pond__status > div:nth-child(4) { border-top: 1px solid var(--app-border); }
  .profile-pond__zone-switch { align-items: stretch; flex-direction: column; }
  .profile-pond__zone-switch :deep(.el-segmented) { width: 100%; }
  .profile-pond :deep(.pond-water) { min-height: 230px; height: 34vh; }
}
@media (prefers-reduced-motion: reduce) { .profile-pond__state .is-spinning { animation: none; } }
</style>
