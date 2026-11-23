<template>
  <div class="space-y-4">
    <div class="space-y-2">
      <div class="text-sm">{{ t("game.itemDetailAffixSlot") }}</div>
      <template v-if="filledSlots.length > 0">
        <el-radio-group v-model="selectedSlotIndex" class="affix-slot-radio-group">
          <el-radio
            v-for="slot in filledSlots"
            :key="`extract-slot-${slot.slotIndex}`"
            :label="slot.slotIndex"
            class="affix-slot-radio-option"
          >
            <AffixQuickRef
              :affix-id="slot.affixId"
              :slot="slot"
              :prefix-label="`${t('game.itemDetailAffixSlot')} #${slot.slotIndex + 1}`"
            />
          </el-radio>
        </el-radio-group>
      </template>
      <template v-else>
        <div class="text-sm app-muted">{{ t("game.itemDetailAffixSlotNoAvailableSlot") }}</div>
      </template>
    </div>

    <div class="space-y-2">
      <div class="text-sm">{{ t("game.itemDetailDetachTool") }}</div>
      <div class="flex items-center gap-2">
        <Card
          v-if="selectedToolEntry"
          :item-id="selectedToolEntry.templateItemId"
          :instance-id="selectedToolEntry.instanceId"
          :quantity="selectedToolEntry.quantity"
          @click="toolSelectorVisible = true"
        />
        <EmptyCard v-else :label="t('game.inventorySelectorChoose')" @click="toolSelectorVisible = true" />
        <div class="text-xs app-muted" v-if="toolCandidateInstanceIds.length > 0">
          <!-- {{ t("game.itemDetailOwnedCount", { count: toolCandidateInstanceIds.length }) }} -->
        </div>
      </div>
      <div v-if="selectedToolItemId" class="text-xs app-muted">
        {{ t("game.itemDetailMaterialNeedOwned", { need: 1, owned: toolOwnedCount }) }}
      </div>
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
          <!-- {{ t("game.itemDetailOwnedCount", { count: carrierCandidateInstanceIds.length }) }} -->
        </div>
      </div>
      <div v-if="carrierCandidateInstanceIds.length" class="text-xs app-muted">
        {{ t("game.itemDetailMaterialNeedOwned", { need: 1, owned: carrierCandidateInstanceIds.length }) }}
      </div>
    </div>

    <div class="flex">
      <el-button type="primary" :disabled="!canSubmit" :loading="submitting" @click="submitExtract">{{ t("game.confirm") }}</el-button>
    </div>

    <InventoryInstanceSelectorDialog
      v-model="toolSelectorVisible"
      :title="t('game.itemDetailDetachTool')"
      :instance-ids="toolCandidateInstanceIds"
      :selected-instance-id="selectedToolInstanceId"
      :allow-clear="true"
      :template-item-ids="detachToolTemplateItemIds"
      :template-items-title="t('game.inventorySelectorAvailableItemsTitle', { title: t('game.itemDetailDetachTool') })"
      :empty-text="t('game.inventorySelectorNoItems')"
      @confirm="selectedToolInstanceId = $event"
    />

    <InventoryInstanceSelectorDialog
      v-model="carrierSelectorVisible"
      :title="t('game.itemDetailAttachCarrier')"
      :instance-ids="carrierCandidateInstanceIds"
      :selected-instance-id="selectedCarrierInstanceId"
      :allow-clear="true"
      :template-item-ids="detachCarrierTemplateItemIds"
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
import { getFilledAffixSlots, hasAffixCapacity, hasAnyAffix } from "@services/equipment-affix-slots"
import { useGameStore } from "@stores/game"
import { useInventoryStore } from "@stores/inventory"
import { useInventorySummary } from "@/composables/game/useInventorySummary"
import AffixQuickRef from "@components/game/affix/AffixQuickRef.vue"
import Card from "@components/game/item/Card.vue"
import EmptyCard from "@components/game/item/EmptyCard.vue"
import InventoryInstanceSelectorDialog from "@components/game/inventory/InventoryInstanceSelectorDialog.vue"
import type { ItemDetailResponse } from "@/types/equipment"

const props = defineProps<{
  detail: ItemDetailResponse | null
  targetInstanceId: number | null
}>()

type AffixDetachResult = {
  equipmentInstanceId: number
  affixItemInstanceId: number
  slotIndex: number
  toolItemId: string
  destroyChance: number
  destroyed: boolean
}

const emit = defineEmits<{
  (e: "submitted", result: AffixDetachResult): void
}>()

const { t } = useI18n()
const gameStore = useGameStore()
const inventoryStore = useInventoryStore()
const { inventoryInstances, getInventoryQuantity } = useInventorySummary()

const selectedSlotIndex = ref<number | null>(null)
const selectedToolInstanceId = ref<number | null>(null)
const selectedCarrierInstanceId = ref<number | null>(null)
const toolSelectorVisible = ref(false)
const carrierSelectorVisible = ref(false)
const submitting = ref(false)

const affixSlotCount = computed(() => props.detail?.operations?.affixSlotCount || 0)
const hasAffixSlots = computed(() => hasAffixCapacity(affixSlotCount.value))
const detachRules = computed(() => props.detail?.operations?.detachRules || [])
const carrierRules = computed(() => props.detail?.operations?.carrierRules || [])
const filledSlots = computed(() => getFilledAffixSlots(props.detail?.operations?.affixSlots || [], affixSlotCount.value))
const hasAnyFilledAffix = computed(() => hasAnyAffix(props.detail?.operations?.affixSlots || [], affixSlotCount.value))
const selectedSlot = computed(() => filledSlots.value.find((slot) => slot.slotIndex === selectedSlotIndex.value) || null)

const allowedToolItemIdSet = computed(() => new Set(detachRules.value.map((rule) => rule.toolItemId)))
const toolCandidateInstanceIds = computed(() =>
  inventoryInstances.value
    .filter((item) => allowedToolItemIdSet.value.has(item.templateItemId))
    .map((item) => item.instanceId)
)
const detachToolTemplateItemIds = computed(() => {
  const out: string[] = []
  const seen = new Set<string>()
  for (const rule of detachRules.value) {
    const id = String(rule.toolItemId || "").trim()
    if (!id || seen.has(id)) continue
    seen.add(id)
    out.push(id)
  }
  return out
})
const selectedToolEntry = computed(() =>
  inventoryStore.instances.find((item) => item.instanceId === selectedToolInstanceId.value) || null
)
const selectedToolItemId = computed(() => {
  return selectedToolEntry.value?.templateItemId || ""
})
const toolOwnedCount = computed(() => (selectedToolItemId.value ? getInventoryQuantity(selectedToolItemId.value) : 0))

const carrierRuleByItemId = computed(() => new Map(carrierRules.value.map((rule) => [rule.carrierItemId, rule.allowedRarityIds || []])))
const detachCarrierTemplateItemIds = computed(() => {
  const out: string[] = []
  const seen = new Set<string>()
  const targetRarityId = selectedSlot.value?.rarityId || ""
  for (const rule of carrierRules.value) {
    const id = String(rule.carrierItemId || "").trim()
    if (!id || seen.has(id)) continue
    const allowed = rule.allowedRarityIds || []
    if (targetRarityId && allowed.length > 0 && !allowed.includes(targetRarityId)) continue
    seen.add(id)
    out.push(id)
  }
  return out
})
const carrierCandidateInstanceIds = computed(() => {
  const targetRarityId = selectedSlot.value?.rarityId || ""
  return inventoryInstances.value
    .filter((item) => {
      const allowedRarities = carrierRuleByItemId.value.get(item.templateItemId)
      if (!allowedRarities) return false
      // if (item.quantity !== 1) return false
      if ((item.affixes || []).length > 0) return false
      if (!targetRarityId) return true
      if (allowedRarities.length === 0) return true
      return allowedRarities.includes(targetRarityId)
    })
    .map((item) => item.instanceId)
})
const selectedCarrierEntry = computed(() =>
  inventoryStore.instances.find((item) => item.instanceId === selectedCarrierInstanceId.value) || null
)

const canSubmit = computed(() => {
  return (
    !submitting.value &&
    hasAffixSlots.value &&
    hasAnyFilledAffix.value &&
    !!gameStore.token &&
    !!props.targetInstanceId &&
    selectedSlotIndex.value !== null &&
    selectedToolItemId.value.trim() !== "" &&
    toolOwnedCount.value >= 1 &&
    selectedCarrierInstanceId.value !== null
  )
})

watch(
  () => props.detail,
  () => {
    selectedSlotIndex.value = filledSlots.value[0]?.slotIndex ?? null
    selectedToolInstanceId.value = toolCandidateInstanceIds.value[0] ?? null
    selectedCarrierInstanceId.value = carrierCandidateInstanceIds.value[0] ?? null
  },
  { immediate: true }
)

watch(
  () => [selectedSlotIndex.value, carrierCandidateInstanceIds.value.join(",")] as const,
  () => {
    if (!selectedCarrierInstanceId.value || !carrierCandidateInstanceIds.value.includes(selectedCarrierInstanceId.value)) {
      selectedCarrierInstanceId.value = carrierCandidateInstanceIds.value[0] ?? null
    }
  }
)

async function submitExtract() {
  if (!gameStore.token || !props.targetInstanceId || selectedSlotIndex.value === null || !selectedCarrierInstanceId.value) return
  submitting.value = true
  try {
    const result = await apiPost<AffixDetachResult>("/api/equipment/affix/detach", gameStore.token, {
      instanceId: props.targetInstanceId,
      slotIndex: selectedSlotIndex.value,
      toolItemId: selectedToolItemId.value,
      carrierInstanceId: selectedCarrierInstanceId.value
    })
    if (!result.destroyed) {
      ElMessage.success(t("game.itemDetailSuccess"))
    }
    emit("submitted", result)
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
  flex-wrap: wrap;
  flex-direction: column;
  align-items: flex-start;
  gap: 8px;
}

.affix-slot-radio-option {
  margin-right: 0;
}
</style>
