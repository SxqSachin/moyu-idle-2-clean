<template>
  <div
    class="candidate-item"
    :class="{ 'is-disabled': !candidate.canEquip, 'is-loading': equippingInstanceId === candidate.instanceId }"
    @click="onEquip"
  >
    <div class="candidate-item-body">
      <ItemCard
        :item-id="candidate.templateItemId"
        :instance-id="candidate.instanceId"
        :requirement-attributes="candidate.requirementAttributes"
      />
      <div v-if="reasonText" class="candidate-item-reason">
        {{ reasonText }}
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from "vue"
import { useI18n } from "vue-i18n"
import type { SlotCandidate } from "@/composables/game/attributes/useEquipmentSlots"
import ItemCard from "@components/game/item/Card.vue"

const props = defineProps<{
  candidate: SlotCandidate
  equippingInstanceId: number | null
}>()

const emit = defineEmits<{
  (e: "equip", instanceID: number): void
}>()

const { t } = useI18n()

const reasonText = computed(() => {
  if (props.candidate.canEquip) return ""
  if (!props.candidate.reasonKey) return ""
  return t(props.candidate.reasonKey)
})

function onEquip() {
  if (!props.candidate.canEquip) return
  if (props.equippingInstanceId !== null) return
  emit("equip", props.candidate.instanceId)
}
</script>

<style scoped>
.candidate-item {
  display: block;
  min-width: 0;
  border: 1px solid transparent;
  border-radius: 10px;
  padding: 2px;
  cursor: pointer;
  transition: border-color 120ms ease, transform 80ms ease;
}

.candidate-item:not(.is-disabled):hover {
  border-color: var(--el-color-primary-light-5);
}

.candidate-item:not(.is-disabled):active {
  transform: scale(0.98);
}

.candidate-item.is-disabled {
  cursor: not-allowed;
  opacity: 0.5;
}

.candidate-item.is-loading {
  opacity: 0.7;
}

.candidate-item-body {
  display: grid;
  gap: 4px;
  justify-items: center;
  min-width: 0;
}

.candidate-item-reason {
  max-width: 100%;
  font-size: 11px;
  line-height: 1.2;
  color: var(--app-muted);
  text-align: center;
  word-break: break-word;
}
</style>
