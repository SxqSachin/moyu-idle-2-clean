<template>
  <button type="button" class="equipment-slot-cell" @click="$emit('open', slot)">
    <span class="equipment-slot-empty">+</span>
    <span class="equipment-slot-label">{{ localizeEquipSlot(slot) }}</span>
  </button>
</template>

<script setup lang="ts">
import { useI18n } from "vue-i18n"

defineProps<{
  slot: string
}>()

defineEmits<{
  (e: "open", slot: string): void
}>()

const { t } = useI18n()

function localizeEquipSlot(slot: string) {
  const key = `game.equipSlot.${slot}`
  const localized = t(key)
  return localized === key ? slot : localized
}
</script>

<style scoped>
.equipment-slot-cell {
  width: 64px;
  height: 64px;
  border: 1px solid var(--app-border);
  border-radius: 12px;
  display: inline-flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 4px;
  background: var(--app-surface);
  cursor: pointer;

  position: relative;
}

.equipment-slot-label {
  font-size: 10px;
  line-height: 1.1;
  color: var(--app-muted);
  text-align: center;

  position: absolute;
  bottom: -1.5em;
}

.equipment-slot-empty {
  font-size: 12px;
  line-height: 1;
}
</style>
