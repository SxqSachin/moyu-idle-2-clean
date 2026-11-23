<template>
  <el-tag
    class="building-quick-ref-tag"
    :type="hasEnoughLevel ? 'primary' : 'danger'"
    effect="plain"
    :disable-transitions="true"
  >
    <span class="building-quick-ref-content">
      <TypeIcon :type="iconType" :id="iconID" :alt="buildingName" :size="20" fallback="🏗️" />
      <span>{{ buildingName }}</span>
      <span v-if="requiredLevel > 0">: {{ requiredLevel }}({{ effectiveCurrentLevel }})</span>
    </span>
  </el-tag>
</template>

<script setup lang="ts">
import { computed } from "vue"
import { useI18n } from "vue-i18n"
import TypeIcon from "@components/game/icon/TypeIcon.vue"
import { parseIconId } from "@services/item-icons"
import { getBuildingDefinition } from "@services/building-definition"
import { usePlayerStore } from "@stores/player"

const props = withDefaults(
  defineProps<{
    buildingId: string
    requiredLevel?: number
    currentLevel?: number
  }>(),
  {
    requiredLevel: 0,
    currentLevel: undefined
  }
)

const { t } = useI18n()
const playerStore = usePlayerStore()

const normalizedBuildingID = computed(() => String(props.buildingId || "").trim())
const buildingDefinition = computed(() => getBuildingDefinition(normalizedBuildingID.value))
const buildingName = computed(() => {
  const key = buildingDefinition.value?.nameKey || `building.${normalizedBuildingID.value}.name`
  const localized = t(key)
  if (localized !== key && localized.trim() !== "") return localized
  return normalizedBuildingID.value || key
})
const iconInfo = computed(() => {
  const iconKey = buildingDefinition.value?.iconId || `building.${normalizedBuildingID.value}`
  return parseIconId(iconKey)
})
const iconType = computed(() => iconInfo.value?.type || "building")
const iconID = computed(() => iconInfo.value?.id || normalizedBuildingID.value)
const effectiveCurrentLevel = computed(() => {
  if (typeof props.currentLevel === "number" && Number.isFinite(props.currentLevel)) {
    return Math.max(0, Math.floor(props.currentLevel))
  }
  return Math.max(0, Math.floor(Number(playerStore.buildings?.[normalizedBuildingID.value] || 0)))
})
const requiredLevel = computed(() => Math.max(0, Math.floor(Number(props.requiredLevel || 0))))
const hasEnoughLevel = computed(() => effectiveCurrentLevel.value >= requiredLevel.value)
</script>

<style scoped>
.building-quick-ref-content {
  display: inline-flex;
  align-items: center;
  gap: 4px;
}
</style>
