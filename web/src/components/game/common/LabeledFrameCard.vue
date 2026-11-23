<template>
  <div
    class="labeled-frame-card flex flex-col items-center justify-center"
    :class="[hideBorder ? '' : 'item-card-wrap', { 'labeled-frame-card--two-lines': labelLines === 2 }]"
    :style="frameStyle"
  >
    <div class="relative item-card" :class="hideBorder ? '' : 'item-card--border'" :style="cardStyle" v-bind="$attrs">
      <slot />
    </div>
    <div v-if="label" :class="['item-card-name text-gray-600 dark:text-gray-400', { 'item-card-name--two-lines': labelLines === 2 }, labelClass]">{{ label }}</div>
  </div>
</template>

<script setup lang="ts">
import { computed } from "vue"

defineOptions({
  inheritAttrs: false,
})

const props = withDefaults(defineProps<{
  hideBorder?: boolean
  label?: string
  rarityColor?: string
  width?: number | string
  height?: number | string
  labelClass?: string
  labelLines?: 1 | 2
}>(), {
  rarityColor: "",
  labelClass: "",
  labelLines: 1
})

const cardDimensions = computed(() => {
  const width = props.width ?? (props.hideBorder ? 48 : 56)
  const height = props.height ?? (props.hideBorder ? 48 : 56)
  return {
    width: typeof width === "number" ? `${width}px` : width,
    height: typeof height === "number" ? `${height}px` : height
  }
})

const frameStyle = computed<Record<string, string>>(() => ({
  width: cardDimensions.value.width
}))

const cardStyle = computed<Record<string, string>>(() => {
  const style: Record<string, string> = {
    width: cardDimensions.value.width,
    height: cardDimensions.value.height
  }
  if (props.rarityColor) {
    style["--item-rarity-color"] = props.rarityColor
  }
  return style
})
</script>

<style scoped>
.labeled-frame-card {
  min-width: 0;
  max-width: 100%;
}

.labeled-frame-card--two-lines {
  justify-content: flex-start;
}

.item-card {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  cursor: pointer;

  width: 48px;
  height: 48px;
  overflow: visible;

  position: relative;
  border: 1px solid transparent;
  border-radius: 12px;
  background:
    linear-gradient(
      to top,
      color-mix(in srgb, var(--item-rarity-color, transparent) 20%, transparent) 0%,
      color-mix(in srgb, var(--item-rarity-color, transparent) 10%, transparent) 34%,
      color-mix(in srgb, var(--item-rarity-color, transparent) 4%, transparent) 56%,
      transparent 70%
    ),
    color-mix(in srgb, var(--el-fill-color-light) 84%, white 16%);
  box-shadow:
    0 1px 2px color-mix(in srgb, var(--el-text-color-primary) 10%, transparent);
  transition:
    transform 0.22s ease,
    box-shadow 0.22s ease,
    border-color 0.18s ease,
    background-color 0.18s ease;
}

:global(.dark) .item-card {
  background:
    linear-gradient(
      to top,
      color-mix(in srgb, var(--item-rarity-color, transparent) 22%, transparent) 0%,
      color-mix(in srgb, var(--item-rarity-color, transparent) 11%, transparent) 34%,
      color-mix(in srgb, var(--item-rarity-color, transparent) 5%, transparent) 56%,
      transparent 70%
    ),
    linear-gradient(145deg, color-mix(in srgb, white 6%, transparent) 0%, transparent 48%, color-mix(in srgb, black 16%, transparent) 100%),
    color-mix(in srgb, var(--el-fill-color-light) 90%, black 10%);
  box-shadow:
    inset 0 0 0 1px color-mix(in srgb, white 7%, transparent),
    0 1px 2px color-mix(in srgb, black 42%, transparent);
}

.item-card.item-card--border {
  width: 56px;
  height: 56px;
  overflow: visible;
  border: 1px solid var(--el-border-color);
  border-radius: 12px;
}

.item-card-wrap:hover .item-card,
.item-card-wrap:focus-within .item-card {
  transform: translateY(-2px);
  border-color: var(--el-color-primary-light-5);
  box-shadow:
    0 8px 18px color-mix(in srgb, var(--el-color-primary) 16%, transparent);
}

:global(.dark) .item-card-wrap:hover .item-card,
:global(.dark) .item-card-wrap:focus-within .item-card {
  box-shadow:
    inset 0 0 0 1px color-mix(in srgb, white 9%, transparent),
    0 8px 18px color-mix(in srgb, var(--el-color-primary) 20%, transparent);
}

:global(.theme-switching) .item-card,
:global(.theme-switching) .item-card-name {
  transition: none !important;
}

.item-card-name {
  font-size: 11px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  margin-top: 4px;
  width: 100%;
  min-width: 0;
  max-width: 100%;
  text-align: center;
  transition: color 0.38s ease, transform 0.18s ease;
}

.item-card-name--two-lines {
  display: -webkit-box;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
  white-space: normal;
  overflow-wrap: anywhere;
  line-height: 1.25;
  max-height: 2.5em;
  flex-shrink: 0;
}

.item-card-wrap:hover .item-card-name,
.item-card-wrap:focus-within .item-card-name {
  color: var(--el-color-primary);
  transform: translateY(-1px);
}
</style>
