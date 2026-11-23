<template>
  <GamePageFrame
    title-key="building.fish_pond.name"
    icon="game-icons:fish-eggs"
    tutorial-id="pond"
    :visual-src="fishingVisual.src"
    :visual-tone="fishingVisual.tone"
    body-padding="0 18px 18px"
    data-tutorial="pond-page"
  >
    <template #actions>
      <el-tooltip :content="t('game.fishing.refresh')">
        <el-button circle size="small" :loading="loading" :aria-label="t('game.fishing.refresh')" @click="refresh"><Icon icon="lucide:refresh-cw" /></el-button>
      </el-tooltip>
    </template>
    <FishingPondPanel
      ref="pondPanel"
      data-tutorial="fishing-pond"
      :species="species"
      :tournament-target-species-id="tournamentTargetSpeciesId"
    />
  </GamePageFrame>
</template>

<script setup lang="ts">
import { computed, ref, watch } from "vue"
import { Icon } from "@iconify/vue"
import { useI18n } from "vue-i18n"
import GamePageFrame from "@components/game/common/GamePageFrame.vue"
import FishingPondPanel from "@components/game/fishing/FishingPondPanel.vue"
import { useGameStore } from "@stores/game"
import { gamePageVisuals } from "@services/game-page-visuals"
import { fetchFishingOverview, fetchFishingTournaments, type FishingSpecies, type FishingTournamentOverview } from "@services/fishing"

const { t } = useI18n()
const gameStore = useGameStore()
const fishingVisual = gamePageVisuals.fishing
const pondPanel = ref<InstanceType<typeof FishingPondPanel> | null>(null)
const species = ref<FishingSpecies[]>([])
const tournament = ref<FishingTournamentOverview | null>(null)
const loading = ref(false)
const tournamentTargetSpeciesId = computed(() => tournament.value?.current.targetSpeciesId || "")

watch(() => gameStore.token, (token) => {
  if (token && species.value.length === 0 && !loading.value) void loadContext()
}, { immediate: true })

async function loadContext() {
  if (!gameStore.token) return
  loading.value = true
  const [overviewResult, tournamentResult] = await Promise.allSettled([
    fetchFishingOverview(gameStore.token),
    fetchFishingTournaments(gameStore.token)
  ])
  if (overviewResult.status === "fulfilled") species.value = overviewResult.value.species || []
  if (tournamentResult.status === "fulfilled") tournament.value = tournamentResult.value
  loading.value = false
}

async function refresh() {
  loading.value = true
  await Promise.allSettled([loadContext(), pondPanel.value?.loadPond()])
  loading.value = false
}
</script>
