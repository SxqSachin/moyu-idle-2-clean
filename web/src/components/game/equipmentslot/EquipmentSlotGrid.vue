<template>
  <el-tabs v-model="tabValue" class="equipment-tabs" data-tutorial="equipment-type-tabs">
    <el-tab-pane
      v-for="tab in visibleEquipmentTabs"
      :key="tab.name"
      :label="t(tab.labelKey)"
      :lazy="true"
      :name="tab.name"
    >
      <div v-if="hasVisibleMainSlots" class="equipment-main-grid gap-6 md:gap-6 place-items-center" data-tutorial="equipment-main-slots">
        <template v-for="slot in mainSlots" :key="`${tab.name}-${slot || 'empty'}`">
          <EquipmentSlotOccupiedCell
            v-if="slot && isSlotVisible(slot) && slotCardByKey(slot)?.instance"
            :slot="slot"
            :card="slotCardByKey(slot)"
            @open="openSlotDialog"
          />
          <EquipmentSlotEmptyCell
            v-else-if="slot && isSlotVisible(slot)"
            :slot="slot"
            @open="openSlotDialog"
          />
          <div v-else class="equipment-slot-empty-cell" />
        </template>
      </div>

      <template v-if="visibleToolSlots.length">
        <el-divider v-if="tab.name !== 'fishing'" class="my-4" content-position="center">{{ t("game.equipmentToolDivider") }}</el-divider>
        <div class="equipment-tool-slots" data-tutorial="equipment-tool-slots">
          <template v-for="slot in visibleToolSlots" :key="`${tab.name}-tool-${slot}`">
            <EquipmentSlotOccupiedCell
              v-if="slotCardByKey(slot)?.instance"
              :slot="slot"
              :card="slotCardByKey(slot)"
              @open="openSlotDialog"
            />
            <EquipmentSlotEmptyCell v-else :slot="slot" @open="openSlotDialog" />
          </template>
        </div>
      </template>

      <div v-if="visibleSpecializationSlots.length" class="equipment-specialization-slots" data-tutorial="equipment-specialization-slots">
        <template v-for="slot in visibleSpecializationSlots" :key="`${tab.name}-specialization-${slot}`">
          <EquipmentSlotOccupiedCell
            v-if="slotCardByKey(slot)?.instance"
            :slot="slot"
            :card="slotCardByKey(slot)"
            @open="openSlotDialog"
          />
          <EquipmentSlotEmptyCell v-else :slot="slot" @open="openSlotDialog" />
        </template>
      </div>

      <el-divider v-if="visibleAccessorySlots.length" class="my-4" content-position="center">{{ t("game.equipmentAccessoryDivider") }}</el-divider>

      <div v-if="visibleAccessorySlots.length" class="grid grid-cols-3 justify-center gap-6 md:grid-cols-4 md:gap-4 place-items-center" data-tutorial="equipment-accessory-slots">
        <template v-for="slot in visibleAccessorySlots" :key="`${tab.name}-acc-${slot}`">
          <EquipmentSlotOccupiedCell
            v-if="slotCardByKey(slot)?.instance"
            :slot="slot"
            :card="slotCardByKey(slot)"
            @open="openSlotDialog"
          />
          <EquipmentSlotEmptyCell
            v-else
            :slot="slot"
            @open="openSlotDialog"
          />
        </template>
      </div>
    </el-tab-pane>
  </el-tabs>

  <GameDialog
    v-model="emptyDialogVisible"
    :title="slotPickerTitle"
    width="680px"
  >
    <EquipmentSlotCandidatePanel
      :eligible-candidates="eligibleCandidates"
      :ineligible-candidates="ineligibleCandidates"
      :equipping-instance-id="equippingInstanceId"
      @equip="handleEquip"
    />
  </GameDialog>

  <GameDialog
    v-model="occupiedDialogVisible"
    :title="slotPickerTitle"
    :width="combinedDialogWidth"
    class="equipment-slot-combined-dialog"
  >
    <div v-if="selectedSlotCard?.instance" class="equipment-slot-combined">
      <section class="equipment-slot-combined-detail">
        <slot
          name="occupied-detail"
          :instance="selectedSlotCard.instance"
          :close="closeDialog"
          :updated="handleDetailUpdated"
        >
          <ItemDetailPanel
            :item-id="selectedSlotCard.instance.templateItemId"
            :instance-id="selectedSlotCard.instance.instanceId"
            :requirement-attributes="selectedSlotCard.requirementAttributes"
            :active="occupiedDialogVisible"
            @updated="handleDetailUpdated"
            @request-close="closeDialog"
          />
        </slot>
      </section>

      <section class="equipment-slot-combined-picker">
        <EquipmentSlotCandidatePanel
          :eligible-candidates="eligibleCandidates"
          :ineligible-candidates="ineligibleCandidates"
          :equipping-instance-id="equippingInstanceId"
          @equip="handleEquip"
        />
      </section>
    </div>
  </GameDialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from "vue"
import { useI18n } from "vue-i18n"
import { useDevice } from "@composables/game/useDevice"
import type { EquipmentSlotCard } from "@/composables/game/attributes/useEquipmentSlots"
import EquipmentSlotEmptyCell from "@components/game/equipmentslot/EquipmentSlotEmptyCell.vue"
import EquipmentSlotOccupiedCell from "@components/game/equipmentslot/EquipmentSlotOccupiedCell.vue"
import EquipmentSlotCandidatePanel from "@components/game/equipmentslot/EquipmentSlotCandidatePanel.vue"
import ItemDetailPanel from "@components/game/item/ItemDetailPanel.vue"
import GameDialog from "@components/game/common/GameDialog.vue"
import type { EquipmentPanelTab, EquipmentSlotType } from "@/types/equipment"

const props = withDefaults(defineProps<{
  slotCards: EquipmentSlotCard[]
  equipmentTypeActiveTab: EquipmentPanelTab
  equippingInstanceId: number | null
  equipSuccessVersion: number
  allowedEquipTypes?: EquipmentPanelTab[]
}>(), {
  allowedEquipTypes: () => ["work", "fishing", "battle"]
})

const emit = defineEmits<{
  (e: "equip", payload: { slot: string; instanceId: number; equipType: EquipmentSlotType }): void
  (e: "updated"): void
  (e: "update:equipmentTypeActiveTab", value: EquipmentPanelTab): void
}>()

const { t } = useI18n()
const { isMobile } = useDevice()

const activeSlot = ref("")
const dialogMode = ref<"" | "empty" | "occupied">("")
const dialogVisible = ref(false)

const tabValue = computed({
  get: () => props.equipmentTypeActiveTab,
  set: (value: EquipmentPanelTab) => emit("update:equipmentTypeActiveTab", value)
})

const equipmentTabs = [
  { name: "work" as const, labelKey: "game.equipmentTypeWork" },
  { name: "fishing" as const, labelKey: "game.equipmentTypeFishing" },
  { name: "battle" as const, labelKey: "game.equipmentTypeBattle" }
]
const visibleEquipmentTabs = computed(() => {
  const tabsByName = new Map(equipmentTabs.map((tab) => [tab.name, tab]))
  const requested: EquipmentPanelTab[] = props.allowedEquipTypes.length > 0
    ? props.allowedEquipTypes
    : ["work", "fishing", "battle"]
  return requested.flatMap((name) => {
    const tab = tabsByName.get(name)
    return tab ? [tab] : []
  })
})

const mainSlots: Array<string | null> = [
  "cloak", "head", null,
  "left_hand", "body", "right_hand",
  "arm", "legs", "feet"
]
const mainSlotSet = new Set(mainSlots.filter((slot): slot is string => !!slot))
const specializationSlotSet = new Set(["head_2", "body_2", "right_hand_2", "arm_2", "feet_2"])
const slotCardMap = computed(() => new Map(props.slotCards.map((card) => [card.slot, card])))
const visibleSlotSet = computed(() => new Set(props.slotCards.map((card) => card.slot)))
const visibleAccessorySlots = computed(() => props.slotCards
  .map((card) => card.slot)
  .filter((slot) => /^accessory_[1-8]$/.test(slot))
  .sort((left, right) => Number(left.split("_")[1]) - Number(right.split("_")[1])))
const visibleSpecializationSlots = computed(() => ["head_2", "body_2", "right_hand_2", "arm_2", "feet_2"]
  .filter((slot) => visibleSlotSet.value.has(slot)))
const hasVisibleMainSlots = computed(() => [...mainSlotSet].some((slot) => visibleSlotSet.value.has(slot)))
const visibleToolSlots = computed(() => props.slotCards
  .map((card) => card.slot)
  .filter((slot) => !mainSlotSet.has(slot) && !specializationSlotSet.has(slot) && !/^accessory_[1-8]$/.test(slot)))
const selectedSlotCard = computed(() => (activeSlot.value ? slotCardByKey(activeSlot.value) : undefined))
const eligibleCandidates = computed(() => (selectedSlotCard.value?.candidates || []).filter((candidate) => candidate.canEquip))
const ineligibleCandidates = computed(() => (selectedSlotCard.value?.candidates || []).filter((candidate) => !candidate.canEquip))

const emptyDialogVisible = computed({
  get: () => dialogVisible.value && dialogMode.value === "empty",
  set: (value: boolean) => {
    if (!value) closeDialog()
  }
})

const occupiedDialogVisible = computed({
  get: () => dialogVisible.value && dialogMode.value === "occupied",
  set: (value: boolean) => {
    if (!value) closeDialog()
  }
})

const slotPickerTitle = computed(() => {
  if (!activeSlot.value) return t("game.attributesTabEquipment")
  return t("game.equipmentSlotPickerTitle", { slot: localizeEquipSlot(activeSlot.value) })
})

const combinedDialogWidth = computed(() => {
  if (isMobile.value) return "96%"
  return "1100px"
})

watch(
  () => props.equipSuccessVersion,
  (next, prev) => {
    if (prev === undefined || next === prev) return
    closeDialog()
  }
)

watch(
  visibleEquipmentTabs,
  (tabs) => {
    if (tabs.length === 0) return
    if (!tabs.some((tab) => tab.name === tabValue.value)) {
      emit("update:equipmentTypeActiveTab", tabs[0].name)
    }
  },
  { immediate: true }
)

watch(
  () => selectedSlotCard.value?.instance,
  (instance) => {
    if (!dialogVisible.value) return
    if (dialogMode.value !== "occupied") return
    if (!instance) closeDialog()
  }
)

function slotCardByKey(slot: string) {
  return slotCardMap.value.get(slot)
}

function isSlotVisible(slot: string) {
  return visibleSlotSet.value.has(slot)
}

function openSlotDialog(slot: string) {
  if (!isSlotVisible(slot)) return
  const hasInstance = !!slotCardByKey(slot)?.instance
  if (dialogVisible.value && activeSlot.value === slot) {
    closeDialog()
    return
  }
  activeSlot.value = slot
  dialogMode.value = hasInstance ? "occupied" : "empty"
  dialogVisible.value = true
}

function closeDialog() {
  dialogVisible.value = false
  dialogMode.value = ""
  activeSlot.value = ""
}

function handleEquip(instanceId: number) {
  if (!activeSlot.value) return
  emit("equip", {
    slot: activeSlot.value,
    instanceId,
    equipType: tabValue.value === "fishing" ? "work" : tabValue.value
  })
}

function handleDetailUpdated() {
  closeDialog()
  emit("updated")
}

function localizeEquipSlot(slot: string) {
  const key = `game.equipSlot.${slot}`
  const localized = t(key)
  return localized === key ? slot : localized
}
</script>

<style scoped>
.equipment-tabs {
  --el-tabs-header-height: 48px;
}

.equipment-main-grid {
  display: grid;
  grid-template-columns: repeat(3, 72px);
  justify-content: center;
  margin-bottom: 3em;
  justify-items: center;
}

.equipment-slot-empty-cell {
  width: 72px;
  height: 72px;
  border: 1px dashed var(--app-border);
  border-radius: 10px;
  opacity: 0.45;
}

.equipment-specialization-slots {
  display: flex;
  min-height: 72px;
  justify-content: center;
  flex-wrap: wrap;
  gap: 24px;
  margin: -1.5em 0 1.5em;
}

.equipment-tool-slots {
  display: flex;
  min-height: 86px;
  align-items: flex-start;
  justify-content: center;
  flex-wrap: wrap;
  gap: 24px;
}

.equipment-slot-combined {
  display: grid;
  grid-template-columns: minmax(320px, 420px) minmax(0, 1fr);
  gap: 16px;
  align-items: start;
}

.equipment-slot-combined-detail {
  min-width: 0;
  border-right: 1px solid var(--app-border);
  padding-right: 14px;
}

.equipment-slot-combined-picker {
  min-width: 0;
}

@media (max-width: 900px) {
  .equipment-slot-combined {
    grid-template-columns: 1fr;
  }

  .equipment-slot-combined-detail {
    border-right: none;
    border-bottom: 1px solid var(--app-border);
    padding-right: 0;
    padding-bottom: 16px;
  }
}
</style>
