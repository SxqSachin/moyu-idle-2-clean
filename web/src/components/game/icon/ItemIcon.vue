<template>
  <TypeIcon type="item" :id="iconId" :size="size" :fallback="fallbackIcon" />
</template>

<script setup lang="ts">
import { computed } from "vue"
import { parseIconId, resolveIconIdByItemId, resolveItemFallbackIcon } from "@services/item-icons"
import TypeIcon from "./TypeIcon.vue";

const props = withDefaults(
  defineProps<{
    itemId: string
    alt?: string
    size?: number
    fallback?: string
  }>(),
  {
    alt: "",
    size: 14,
    fallback: ""
  }
)

const iconInfo = computed(() => parseIconId(resolveIconIdByItemId(props.itemId)))
const iconType = computed(() => iconInfo.value?.type || "item")
const iconId = computed(() => iconInfo.value?.id || props.itemId)

const fallbackIcon = computed(() => props.fallback || resolveItemFallbackIcon(props.itemId))
</script>
