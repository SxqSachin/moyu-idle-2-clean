<template>
  <el-card class="equipment-card py-2" shadow="hover" body-class="pt-0!" data-tutorial="equipment-panel">
    <EquipmentSlotGrid
      :slot-cards="slotCards"
      :equipment-type-active-tab="equipmentTypeActiveTab"
      :equipping-instance-id="equippingInstanceId"
      :equip-success-version="equipSuccessVersion"
      :allowed-equip-types="allowedEquipTypes"
      @equip="$emit('equip', $event)"
      @updated="$emit('updated')"
      @update:equipment-type-active-tab="$emit('update:equipmentTypeActiveTab', $event)"
    >
      <template #occupied-detail="slotProps">
        <slot name="occupied-detail" v-bind="slotProps">
          <ItemDetailPanel
            :item-id="slotProps.instance.templateItemId"
            :instance-id="slotProps.instance.instanceId"
            :active="true"
            @updated="slotProps.updated"
            @request-close="slotProps.close"
          />
        </slot>
      </template>
    </EquipmentSlotGrid>
  </el-card>
</template>

<script setup lang="ts">
import type { EquipmentSlotCard } from "@/composables/game/attributes/useEquipmentSlots"
import EquipmentSlotGrid from "@components/game/equipmentslot/EquipmentSlotGrid.vue"
import ItemDetailPanel from "@components/game/item/ItemDetailPanel.vue"
import type { EquipmentPanelTab, EquipmentSlotType } from "@/types/equipment"

withDefaults(defineProps<{
  slotCards: EquipmentSlotCard[]
  equipmentTypeActiveTab: EquipmentPanelTab
  equippingInstanceId: number | null
  equipSuccessVersion: number
  allowedEquipTypes?: EquipmentPanelTab[]
}>(), {
  allowedEquipTypes: () => ["work", "fishing", "battle"]
})

defineEmits<{
  (e: "equip", payload: { slot: string; instanceId: number; equipType: EquipmentSlotType }): void
  (e: "updated"): void
  (e: "update:equipmentTypeActiveTab", value: EquipmentPanelTab): void
}>()
</script>

<style scoped>
.equipment-card {
  max-width: 540px;
  margin: 0 auto;
}
</style>
