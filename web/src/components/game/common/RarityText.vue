<template>
  <span class="rarity-text" :style="textStyle">{{ rarityLabel }}</span>
</template>

<script setup lang="ts">
import { computed } from "vue"
import { useI18n } from "vue-i18n"
import { getRarityDefinition, getRarityNameKey } from "@services/item-definition"

const props = withDefaults(
  defineProps<{
    rarityId: string
    fallbackText?: string
  }>(),
  {
    fallbackText: ""
  }
)

const { t } = useI18n()

const normalizedRarityId = computed(() => String(props.rarityId || "").trim())
const rarityColor = computed(() => {
  const rarity = getRarityDefinition(normalizedRarityId.value)
  return rarity?.color || "var(--el-text-color-regular)"
})

const rarityLabel = computed(() => {
  const rarityId = normalizedRarityId.value
  if (!rarityId) return props.fallbackText || t("game.itemDetailUnknown")
  const fallbackKeys = [getRarityNameKey(rarityId), `rarity.${rarityId}.name`, `rarity.${rarityId}`].filter(Boolean)
  for (const key of fallbackKeys) {
    const localized = t(key)
    if (localized !== key && localized.trim() !== "") return localized
  }
  return props.fallbackText || rarityId
})

const textStyle = computed(() => ({
  color: rarityColor.value
}))
</script>

<style scoped>
.rarity-text {
  font-weight: 600;
}
</style>
