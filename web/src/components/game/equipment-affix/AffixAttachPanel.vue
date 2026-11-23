<template>
  <div class="space-y-4">
    <div class="text-xs app-muted">{{ t("game.affixAttachOperationHint") }}</div>

    <div class="space-y-2">
      <div class="text-sm">{{ t("game.itemDetailAffixSlot") }}</div>
      <template v-if="emptySlots.length">
        <el-radio-group v-model="selectedSlotIndex" class="affix-slot-radio-group">
          <el-radio
            v-for="slot in emptySlots"
            :key="`embed-slot-${slot}`"
            :label="slot"
            class="affix-slot-radio-option"
          >
            <el-tag type="primary" effect="plain" :disable-transitions="true">
              {{ t("game.itemDetailAffixSlot") }} #{{ slot + 1 }} · {{ t("game.itemDetailAffixEmpty") }}
            </el-tag>
          </el-radio>
        </el-radio-group>
      </template>
      <template v-else>
        <div class="text-sm app-muted">{{ t("game.itemDetailAffixSlotNoAvailableSlot") }}</div>
      </template>
    </div>

    <div class="space-y-2">
      <div class="text-sm">{{ t("game.itemDetailAttachCarrier") }}</div>
      <div class="flex items-center gap-2">
        <Card
          v-if="selectedCarrierEntry"
          :item-id="selectedCarrierEntry.templateItemId"
          :instance-id="selectedCarrierEntry.instanceId"
          :quantity="selectedCarrierEntry.quantity"
          @click="carrierSelectorVisible = true"
        />
        <EmptyCard v-else :label="t('game.inventorySelectorChoose')" @click="carrierSelectorVisible = true" />
        <div class="text-xs app-muted" v-if="carrierCandidateInstanceIds.length > 0">
          {{ t("game.itemDetailOwnedCount", { count: carrierCandidateInstanceIds.length }) }}
        </div>
      </div>
    </div>

    <div class="flex">
      <el-button type="primary" :disabled="!canSubmit" :loading="submitting" @click="submitAttach">{{ t("game.confirm") }}</el-button>
    </div>

    <InventoryInstanceSelectorDialog
      v-model="carrierSelectorVisible"
      :title="t('game.itemDetailAttachCarrier')"
      :instance-ids="carrierCandidateInstanceIds"
      :selected-instance-id="selectedCarrierInstanceId"
      :allow-clear="true"
      :template-item-ids="attachCarrierTemplateItemIds"
      :template-items-title="t('game.inventorySelectorAvailableItemsTitle', { title: t('game.itemDetailAttachCarrier') })"
      :empty-text="t('game.inventorySelectorNoItems')"
      @confirm="selectedCarrierInstanceId = $event"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from "vue"
import { ElMessage } from "element-plus"
import { useI18n } from "vue-i18n"
import { apiPost } from "@services/api"
import { resolveErrorMessage } from "@services/error-messages"
import { getEmptyAffixSlotIndexes, hasAffixCapacity, isAffixFull } from "@services/equipment-affix-slots"
import { getAffixDefinition, getEquipmentEquipType } from "@services/item-definition"
import { useGameStore } from "@stores/game"
import { useInventoryStore } from "@stores/inventory"
import { useInventorySummary } from "@/composables/game/useInventorySummary"
import Card from "@components/game/item/Card.vue"
import EmptyCard from "@components/game/item/EmptyCard.vue"
import InventoryInstanceSelectorDialog from "@components/game/inventory/InventoryInstanceSelectorDialog.vue"
import type { ItemDetailResponse } from "@/types/equipment"

const props = defineProps<{
  detail: ItemDetailResponse | null
  targetInstanceId: number | null
}>()

const emit = defineEmits<{
  (e: "submitted"): void
}>()

const { t } = useI18n()
const gameStore = useGameStore()
const inventoryStore = useInventoryStore()
const { inventoryInstances } = useInventorySummary()

const selectedSlotIndex = ref<number | null>(null)
const selectedCarrierInstanceId = ref<number | null>(null)
const carrierSelectorVisible = ref(false)
const submitting = ref(false)

const affixSlotCount = computed(() => props.detail?.operations?.affixSlotCount || 0)
const affixSlots = computed(() => props.detail?.operations?.affixSlots || [])
const hasAffixSlots = computed(() => hasAffixCapacity(affixSlotCount.value))
const affixSlotsFull = computed(() => isAffixFull(affixSlots.value, affixSlotCount.value))
const targetEquipType = computed(() => getEquipmentEquipType(props.detail?.item.id || ""))
const carrierRules = computed(() => props.detail?.operations?.carrierRules || [])
const carrierRuleItemIdSet = computed(() => new Set(carrierRules.value.map((rule) => rule.carrierItemId)))

const emptySlots = computed(() => getEmptyAffixSlotIndexes(affixSlots.value, affixSlotCount.value))
const attachCarrierTemplateItemIds = computed(() => {
  const out: string[] = []
  const seen = new Set<string>()
  for (const rule of carrierRules.value) {
    const id = String(rule.carrierItemId || "").trim()
    if (!id || seen.has(id)) continue
    seen.add(id)
    out.push(id)
  }
  return out
})
const carrierCandidateInstanceIds = computed(() =>
  inventoryInstances.value
    .filter((item) => {
      if (!carrierRuleItemIdSet.value.has(item.templateItemId)) return false
      if (item.quantity !== 1) return false
      const firstAffix = (item.affixes || [])[0]
      if (!firstAffix?.affixId) return false
      // Custom-crafted affixes store their alchemy kind in effectParams.
      // Legacy custom affixes without alchemyKind remain selectable.
      const isCustomCraft =
        firstAffix.sourceKind === "craft" ||
        String(firstAffix.affixId || "").startsWith("custom:")
      if (isCustomCraft) return customAlchemyAffixMatchesTarget(firstAffix.effectParams)
      return getAffixDefinition(firstAffix.affixId)?.type === targetEquipType.value
    })
    .map((item) => item.instanceId)
)

function customAlchemyAffixMatchesTarget(effectParams: Record<string, unknown> | undefined): boolean {
  const rawKind = String(effectParams?.alchemyKind || "").trim().toLowerCase()
  if (!rawKind) return true
  const expected = rawKind === "life" || rawKind === "work" ? "work" : rawKind === "combat" || rawKind === "battle" ? "battle" : ""
  if (!expected) return false
  return targetEquipType.value === expected
}
const selectedCarrierEntry = computed(() =>
  inventoryStore.instances.find((item) => item.instanceId === selectedCarrierInstanceId.value) || null
)

const canSubmit = computed(() => {
  return (
    !submitting.value &&
    hasAffixSlots.value &&
    !affixSlotsFull.value &&
    !!gameStore.token &&
    !!props.targetInstanceId &&
    selectedSlotIndex.value !== null &&
    selectedCarrierInstanceId.value !== null
  )
})

watch(
  () => props.detail,
  () => {
    selectedSlotIndex.value = emptySlots.value[0] ?? null
    selectedCarrierInstanceId.value = carrierCandidateInstanceIds.value[0] ?? null
  },
  { immediate: true }
)

watch(
  () => carrierCandidateInstanceIds.value.join(","),
  () => {
    if (!selectedCarrierInstanceId.value || !carrierCandidateInstanceIds.value.includes(selectedCarrierInstanceId.value)) {
      selectedCarrierInstanceId.value = carrierCandidateInstanceIds.value[0] ?? null
    }
  }
)

async function submitAttach() {
  if (!gameStore.token || !props.targetInstanceId || selectedSlotIndex.value === null || selectedCarrierInstanceId.value === null) return
  submitting.value = true
  try {
    await apiPost("/api/equipment/affix/attach", gameStore.token, {
      equipmentInstanceId: props.targetInstanceId,
      affixInstanceId: selectedCarrierInstanceId.value,
      slotIndex: selectedSlotIndex.value
    })
    ElMessage.success(t("game.itemDetailSuccess"))
    emit("submitted")
  } catch (err) {
    ElMessage.error(resolveErrorMessage(t, err, "game.itemOperationFailed"))
  } finally {
    submitting.value = false
  }
}

</script>

<style scoped>
.affix-slot-radio-group {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 8px;
}

.affix-slot-radio-option {
  margin-right: 0;
}
</style>
