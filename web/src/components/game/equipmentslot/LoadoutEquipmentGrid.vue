<template>
  <div>
    <div class="loadout-main-grid place-items-center">
      <template v-for="slot in mainSlots" :key="slot || 'empty'">
        <EquipmentSlotOccupiedCell
          v-if="slot && isSlotVisible(slot) && slotCardByKey(slot)?.instance"
          :slot="slot"
          :card="slotCardByKey(slot)"
          @open="openSlotSelector"
        />
        <EquipmentSlotEmptyCell
          v-else-if="slot && isSlotVisible(slot)"
          :slot="slot"
          @open="openSlotSelector"
        />
        <div v-else class="loadout-slot-placeholder" />
      </template>
    </div>

    <div v-if="visibleSpecializationSlots.length" class="loadout-specialization-slots">
      <template v-for="slot in visibleSpecializationSlots" :key="`specialization-${slot}`">
        <EquipmentSlotOccupiedCell
          v-if="slotCardByKey(slot)?.instance"
          :slot="slot"
          :card="slotCardByKey(slot)"
          @open="openSlotSelector"
        />
        <EquipmentSlotEmptyCell v-else :slot="slot" @open="openSlotSelector" />
      </template>
    </div>

    <el-divider class="my-4" content-position="center">{{ t("game.equipmentAccessoryDivider") }}</el-divider>

    <div class="grid grid-cols-3 justify-center gap-6 md:grid-cols-4 md:gap-4 place-items-center">
      <template v-for="slot in visibleAccessorySlots" :key="`acc-${slot}`">
        <EquipmentSlotOccupiedCell
          v-if="slotCardByKey(slot)?.instance"
          :slot="slot"
          :card="slotCardByKey(slot)"
          @open="openSlotSelector"
        />
        <EquipmentSlotEmptyCell
          v-else
          :slot="slot"
          @open="openSlotSelector"
        />
      </template>
    </div>

    <InventoryInstanceSelectorDialog
      v-model="selectorVisible"
      :title="selectorTitle"
      :instance-ids="candidateInstanceIds"
      :ineligible-instance-ids="ineligibleCandidateInstanceIds"
      :selected-instance-id="activeSlotInstanceId"
      :allow-clear="!!activeSlotCard?.instance"
      @confirm="handleConfirm"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from "vue"
import { useI18n } from "vue-i18n"
import type { EquipmentSlotCard } from "@/composables/game/attributes/useEquipmentSlots"
import EquipmentSlotOccupiedCell from "@components/game/equipmentslot/EquipmentSlotOccupiedCell.vue"
import EquipmentSlotEmptyCell from "@components/game/equipmentslot/EquipmentSlotEmptyCell.vue"
import InventoryInstanceSelectorDialog from "@components/game/inventory/InventoryInstanceSelectorDialog.vue"

const props = defineProps<{
  slotCards: EquipmentSlotCard[]
}>()

const emit = defineEmits<{
  (e: "equip", payload: { slot: string; instanceId: number }): void
  (e: "unequip", payload: { slot: string; instanceId: number }): void
}>()

const { t } = useI18n()

const mainSlots: Array<string | null> = [
  "cloak", "head", null,
  "left_hand", "body", "right_hand",
  "arm", "legs", "feet"
]
const activeSlot = ref("")
const selectorVisible = ref(false)

const slotCardMap = computed(() => new Map(props.slotCards.map((card) => [card.slot, card])))
const visibleSlotSet = computed(() => new Set(props.slotCards.map((card) => card.slot)))
const visibleAccessorySlots = computed(() => props.slotCards
  .map((card) => card.slot)
  .filter((slot) => /^accessory_[1-8]$/.test(slot))
  .sort((left, right) => Number(left.split("_")[1]) - Number(right.split("_")[1])))
const visibleSpecializationSlots = computed(() => ["head_2", "body_2", "right_hand_2", "arm_2", "feet_2"]
  .filter((slot) => visibleSlotSet.value.has(slot)))

const activeSlotCard = computed(() => activeSlot.value ? slotCardMap.value.get(activeSlot.value) : undefined)
const activeSlotInstanceId = computed(() => activeSlotCard.value?.instance?.instanceId ?? null)
const candidateInstanceIds = computed(() => (activeSlotCard.value?.candidates || []).filter((c) => c.canEquip).map((c) => c.instanceId))
const ineligibleCandidateInstanceIds = computed(() => (activeSlotCard.value?.candidates || []).filter((c) => !c.canEquip).map((c) => c.instanceId))

const selectorTitle = computed(() => {
  if (!activeSlot.value) return t("game.attributesTabEquipment")
  const key = `game.equipSlot.${activeSlot.value}`
  const localized = t(key)
  const slotLabel = localized === key ? activeSlot.value : localized
  return t("game.equipmentSlotPickerTitle", { slot: slotLabel })
})

function isSlotVisible(slot: string) {
  return visibleSlotSet.value.has(slot)
}

function slotCardByKey(slot: string) {
  return slotCardMap.value.get(slot)
}

function openSlotSelector(slot: string) {
  activeSlot.value = slot
  selectorVisible.value = true
}

function handleConfirm(instanceId: number | null) {
  const slot = activeSlot.value
  const currentInstance = activeSlotCard.value?.instance
  activeSlot.value = ""
  if (instanceId) {
    emit("equip", { slot, instanceId })
  } else if (currentInstance) {
    emit("unequip", { slot, instanceId: currentInstance.instanceId })
  }
}
</script>

<style scoped>
.loadout-main-grid {
  display: grid;
  grid-template-columns: repeat(3, 72px);
  justify-content: center;
  margin-bottom: 3em;
  gap: 24px;
  justify-items: center;
}

.loadout-slot-placeholder {
  width: 72px;
  height: 72px;
  border: 1px dashed var(--app-border);
  border-radius: 10px;
  opacity: 0.45;
}


.loadout-specialization-slots {
  display: flex;
  min-height: 72px;
  justify-content: center;
  flex-wrap: wrap;
  gap: 24px;
  margin: -1.5em 0 1.5em;
}
</style>
