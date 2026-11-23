<template>
  <Icon :type="type" :id="id" :alt="alt || `${type}.${id}`" :size="size" :fallback="fallback" />
</template>

<script setup lang="ts">
import Icon from "@components/game/icon/Icon.vue"

withDefaults(
  defineProps<{
    type: string
    id: string
    alt?: string
    size?: number
    fallback?: string
  }>(),
  {
    alt: "",
    size: 14,
    fallback: "🖼️"
  }
)
</script>
