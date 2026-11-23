<template>
  <button type="button" class="relative equipment-slot-cell relative" @click="$emit('open', slot)">
    <template v-if="card?.instance">
      <!-- <ItemIcon :item-id="card.instance.templateItemId" :size="30" />
      <span v-if="card.instance.enhanceLevel > 0" class="equipment-slot-level absolute top-0 right-0">+{{ card.instance.enhanceLevel }}</span> -->
      <Card 
        :item-id="card.instance.templateItemId"
        :instance-id="card.instance.instanceId"
        :width="64"
        :height="64"
        :icon-size="52"
        :requirement-attributes="card.requirementAttributes"
        hide-border
        hide-label
      ></Card>
    </template>
    <span class="equipment-slot-label">{{ localizeEquipSlot(slot) }}</span>
  </button>
</template>

<script setup lang="ts">
import { useI18n } from "vue-i18n"
import type { EquipmentSlotCard } from "@/composables/game/attributes/useEquipmentSlots"
import ItemIcon from "@components/game/icon/ItemIcon.vue"
import Card from "../item/Card.vue";

defineProps<{
  slot: string
  card: EquipmentSlotCard | undefined
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
  width: 65px;
  height: 65px;
  border: 1px solid var(--app-border);
  border-radius: 12px;
  display: inline-flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 4px;
  background: var(--app-surface);
  cursor: pointer;
}

.equipment-slot-label {
  position: absolute;
  bottom: -1.5em;

  font-size: 10px;
  line-height: 1.1;
  color: var(--app-muted);
  text-align: center;
}

.equipment-slot-level {
  font-size: 12px;
  line-height: 1;
}
</style>
