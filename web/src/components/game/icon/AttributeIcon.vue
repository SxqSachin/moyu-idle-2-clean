<template>
  <span class="game-attribute-icon" :style="iconStyle">
    <Icon v-if="iconifyIcon" :icon="iconifyIcon" :width="size" :height="size" />
    <TypeIcon
      v-else-if="legacyType && legacyId"
      :type="legacyType"
      :id="legacyId"
      :alt="resolvedAlt"
      :size="size"
      :fallback="resolvedFallback"
    />
    <span v-else class="game-attribute-icon__fallback" :style="fallbackStyle">{{ resolvedFallback }}</span>
  </span>
</template>

<script setup lang="ts">
import { computed } from "vue"
import { Icon } from "@iconify/vue"
import TypeIcon from "@components/game/icon/TypeIcon.vue"
import { getAttributeDefinition } from "@services/attribute-display"
import { parseIconId } from "@services/item-icons"

const props = withDefaults(
  defineProps<{
    attrKey: string
    alt?: string
    size?: number
    fallback?: string
  }>(),
  {
    alt: "",
    size: 14,
    fallback: "📈"
  }
)

const definition = computed(() => getAttributeDefinition(props.attrKey))
const iconifyIcon = computed(() => {
  const icon = definition.value?.icon
  return typeof icon === "string" && icon.trim() !== "" ? icon.trim() : ""
})
const legacyIcon = computed(() => {
  const iconId = definition.value?.iconId
  if (typeof iconId !== "string" || iconId.trim() === "") return null
  return parseIconId(iconId.trim())
})
const legacyType = computed(() => legacyIcon.value?.type || "")
const legacyId = computed(() => legacyIcon.value?.id || "")
const resolvedFallback = computed(() => {
  const emoji = definition.value?.emojiIcon
  if (typeof emoji === "string" && emoji.trim() !== "") return emoji.trim()
  return props.fallback
})
const resolvedAlt = computed(() => props.alt || definition.value?.id || props.attrKey)
const iconStyle = computed(() => ({
  width: `${props.size}px`,
  height: `${props.size}px`
}))
const fallbackStyle = computed(() => ({
  fontSize: `${Math.max(10, props.size - 4)}px`
}))
</script>

<style scoped>
.game-attribute-icon,
.game-attribute-icon__fallback {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  vertical-align: middle;
}
</style>
