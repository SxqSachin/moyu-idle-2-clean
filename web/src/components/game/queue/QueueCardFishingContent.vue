<template>
  <div class="fishing-queue-summary">
    <div class="fishing-queue-summary__context">
      <Icon icon="lucide:map-pin" aria-hidden="true" />
      <span class="fishing-queue-summary__water">{{ waterName }}</span>
      <span aria-hidden="true">·</span>
      <span>{{ fishing.loadoutName || t("game.fishing.loadout") }}</span>
      <span aria-hidden="true">·</span>
      <span>{{ methodLabel }}</span>
    </div>

    <div class="fishing-queue-summary__metrics">
      <template v-if="card.status === 'running'">
        <div class="fishing-queue-metric">
          <Icon icon="lucide:fish-symbol" aria-hidden="true" />
          <span>{{ t("game.fishing.queueCatch") }}</span>
          <strong>{{ t("game.fishing.queueFishCount", { count: fishing.landed || 0 }) }}</strong>
        </div>
        <div class="fishing-queue-metric">
          <Icon icon="lucide:scale" aria-hidden="true" />
          <span>{{ t("game.fishing.totalWeight") }}</span>
          <strong>{{ totalWeightLabel }}</strong>
        </div>
        <div class="fishing-queue-metric">
          <Icon icon="lucide:gauge" aria-hidden="true" />
          <span>{{ t("game.fishing.lineState") }}</span>
          <strong>{{ lineStateLabel }}</strong>
        </div>
      </template>
      <template v-else>
        <div class="fishing-queue-metric">
          <Icon icon="lucide:clock-3" aria-hidden="true" />
          <span>{{ t("game.fishing.duration") }}</span>
          <strong>{{ durationLabel }}</strong>
        </div>
        <div class="fishing-queue-metric">
          <Icon icon="lucide:waves" aria-hidden="true" />
          <span>{{ t("game.fishing.method") }}</span>
          <strong>{{ methodLabel }}</strong>
        </div>
        <div class="fishing-queue-metric">
          <Icon icon="lucide:hand" aria-hidden="true" />
          <span>{{ t("game.fishing.strategy") }}</span>
          <strong>{{ strategyLabel }}</strong>
        </div>
      </template>

      <div class="fishing-queue-metric">
        <Icon :icon="supplyIcon" aria-hidden="true" />
        <span>{{ supplyLabel }}</span>
        <strong>{{ supplyCount }}</strong>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from "vue"
import { Icon } from "@iconify/vue"
import { useI18n } from "vue-i18n"
import { getGameDataTable } from "@services/game-data-runtime"
import { localizedFishingText, type FishingWater } from "@services/fishing"
import { formatDurationI18n } from "@/utils/duration-format"
import type { QueueCard, QueueCardDisplayFishing } from "@components/game/queue/types"

const props = defineProps<{
  fishing: QueueCardDisplayFishing
  card: QueueCard
}>()

const { t, locale } = useI18n()
const watersByID = new Map(
  getGameDataTable<FishingWater>("fishing_waters").map((water) => [water.id, water])
)

const waterName = computed(() => {
  const water = watersByID.get(props.fishing.waterId)
  return localizedFishingText(water?.names, locale.value) || t("game.fishing.waters")
})
const methodLabel = computed(() => t(`game.fishing.method_${props.fishing.method || "float"}`))
const strategyLabel = computed(() => t(`game.fishing.strategy_${props.fishing.strategy || "balanced"}`))
const durationLabel = computed(() => {
  if (props.fishing.plannedDurationSeconds <= 0) return t("game.fishing.queueContinuous")
  return formatDurationI18n(props.fishing.plannedDurationSeconds, t)
})
const lineStateLabel = computed(() => {
  const key = props.fishing.lineState === "critical"
    ? "lineCritical"
    : props.fishing.lineState === "tight"
      ? "lineTight"
      : "lineSteady"
  return t(`game.fishing.${key}`)
})
const totalWeightLabel = computed(() => {
  const value = new Intl.NumberFormat(locale.value, { maximumFractionDigits: 2 }).format(
    Math.max(0, Number(props.fishing.totalWeightKg || 0))
  )
  return `${value} kg`
})
const usesLure = computed(() => props.fishing.method === "lure")
const supplyLabel = computed(() => t(usesLure.value ? "game.fishing.lure" : "game.fishing.bait"))
const supplyCount = computed(() => Math.max(
  0,
  Number(usesLure.value
    ? props.fishing.remainingSupplies?.lure
    : props.fishing.remainingSupplies?.bait) || 0
))
const supplyIcon = computed(() => usesLure.value ? "game-icons:fishing-lure" : "game-icons:worm-mouth")
</script>

<style scoped>
.fishing-queue-summary {
  display: grid;
  gap: 10px;
  margin-top: 12px;
}

.fishing-queue-summary__context {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 6px;
  color: var(--app-muted);
  font-size: 12px;
  line-height: 18px;
}

.fishing-queue-summary__context svg {
  width: 15px;
  height: 15px;
  flex: none;
  color: #157b7b;
}

.fishing-queue-summary__water {
  color: var(--app-text);
  font-weight: 600;
}

.fishing-queue-summary__metrics {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 8px;
}

.fishing-queue-metric {
  display: grid;
  grid-template-columns: 18px minmax(0, 1fr) auto;
  align-items: center;
  gap: 7px;
  min-height: 38px;
  border: 1px solid color-mix(in srgb, var(--app-border) 74%, transparent);
  border-radius: 6px;
  padding: 7px 9px;
  background: color-mix(in srgb, var(--app-surface) 86%, transparent);
  font-size: 12px;
  color: var(--app-muted);
}

.fishing-queue-metric svg {
  width: 16px;
  height: 16px;
  color: #157b7b;
}

.fishing-queue-metric strong {
  overflow: hidden;
  color: var(--app-text);
  font-weight: 650;
  text-overflow: ellipsis;
  white-space: nowrap;
}

@media (max-width: 840px) {
  .fishing-queue-summary__metrics {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 420px) {
  .fishing-queue-summary__context {
    flex-wrap: wrap;
  }

  .fishing-queue-metric {
    grid-template-columns: 17px minmax(0, 1fr);
  }

  .fishing-queue-metric strong {
    grid-column: 2;
  }
}
</style>
