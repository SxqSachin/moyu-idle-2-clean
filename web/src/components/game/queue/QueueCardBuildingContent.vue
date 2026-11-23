<template>
  <div class="text-xs text-[var(--app-muted)] space-y-1">
    <div>
      <span>{{ t("game.buildingLabel") }}:</span>
      <span class="ml-1">{{ buildingName }}</span>
    </div>
    <div>
      <span>{{ t("game.buildingLevel") }}:</span>
      <span class="ml-1">{{ building.fromLevel }} → {{ building.toLevel }}</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from "vue"
import { useI18n } from "vue-i18n"
import type { QueueCardDisplayBuilding } from "@components/game/queue/types"

const props = defineProps<{
  building: QueueCardDisplayBuilding
}>()

const { t } = useI18n()

const buildingName = computed(() => {
  const key = `building.${props.building.buildingId}.name`
  const localized = t(key)
  return localized === key ? props.building.buildingId : localized
})
</script>

