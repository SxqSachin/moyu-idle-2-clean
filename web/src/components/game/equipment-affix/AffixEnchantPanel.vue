<template>
  <div class="space-y-4">
    <OperationEffectNotice :context="foodContext" />
    <div class="text-xs app-muted">{{ t("game.affixEnchantOperationHint") }}</div>

    <div class="space-y-2">
      <div class="text-sm">{{ t("game.itemDetailAffixSlot") }}</div>
      <template v-if="emptySlots.length">
        <el-radio-group v-model="selectedSlotIndex" class="affix-slot-radio-group">
          <el-radio
            v-for="slot in emptySlots"
            :key="`enchant-slot-${slot}`"
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
      <div class="text-sm">{{ t("game.enchantTool") }}</div>
      <div class="flex items-center gap-2">
        <Card
          v-if="selectedToolEntry"
          :item-id="selectedToolEntry.templateItemId"
          :instance-id="selectedToolEntry.instanceId"
          :quantity="selectedToolEntry.quantity"
          @click="toolSelectorVisible = true"
        />
        <EmptyCard v-else :label="t('game.inventorySelectorChoose')" @click="toolSelectorVisible = true" />
        <!-- <div class="text-xs app-muted" v-if="toolCandidateInstanceIds.length > 0">
          {{ t("game.itemDetailOwnedCount", { count: toolCandidateInstanceIds.length }) }}
        </div> -->
      </div>
      <div v-if="selectedRule" class="space-y-1 text-xs app-muted">
        <div>{{ t("game.itemDetailMaterialNeedOwned", { need: selectedRule.consumeCount, owned: toolOwnedCount }) }}</div>
        <div>{{ failLabel(selectedRule.failChance) }}</div>
        <div v-if="costEntries.length" class="space-y-1">
          <div>{{ t("game.actionRequirements") }}:</div>
          <div class="flex flex-wrap gap-2">
            <QuickRefWithRequire
              v-for="entry in costEntries"
              :key="`enchant-cost-${selectedRule.toolItemId}-${entry.itemId}`"
              :item-id="entry.itemId"
              :required-count="entry.quantity"
            />
          </div>
        </div>
        <div class="flex flex-wrap items-center gap-1">
          <span>{{ t("game.enchantRarityWeights") }}:</span>
          <template v-if="rarityWeightEntries.length">
            <span v-for="(entry, idx) in rarityWeightEntries" :key="`rarity-weight-${entry.rarityId}`" class="inline-flex items-center gap-1">
              <RarityText :rarity-id="entry.rarityId" />
              <span>{{ entry.weight }}</span>
              <span v-if="idx < rarityWeightEntries.length - 1">/</span>
            </span>
          </template>
          <template v-else>{{ t("game.itemDetailUnknown") }}</template>
        </div>
        <div v-if="foodEffects.length" class="flex flex-wrap gap-2">
          <FoodValueHint v-for="row in foodProbabilities" :key="row.rarityId">
            <RarityText :rarity-id="row.rarityId" /> {{ (row.before * 100).toFixed(2) }}% → {{ (row.after * 100).toFixed(2) }}%
          </FoodValueHint>
        </div>
        <div>
          {{ t("game.enchantAllowedAffixes") }}:
          <div v-if="selectedRuleAllowedAffixIds.length" class="flex flex-wrap gap-1 mt-1">
            <AffixQuickRef
              v-for="affixID in selectedRuleAllowedAffixIds"
              :key="`enchant-rule-affix-${affixID}`"
              :affix-id="affixID"
            />
          </div>
          <template v-else>{{ t("game.enchantAllowedAffixesAll") }}</template>
        </div>
      </div>
    </div>

    <div class="flex">
      <el-button type="primary" :disabled="!canSubmit" :loading="submitting" @click="submitEnchant">{{ t("game.confirm") }}</el-button>
    </div>

    <InventoryInstanceSelectorDialog
      v-model="toolSelectorVisible"
      :title="t('game.enchantTool')"
      :instance-ids="toolCandidateInstanceIds"
      :selected-instance-id="selectedToolInstanceId"
      :allow-clear="true"
      :template-item-ids="enchantToolTemplateItemIds"
      :template-items-title="t('game.inventorySelectorAvailableItemsTitle', { title: t('game.enchantTool') })"
      :empty-text="t('game.inventorySelectorNoItems')"
      @confirm="handleSelectTool"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from "vue"
import OperationEffectNotice from "@components/game/operation-effect/OperationEffectNotice.vue"
import FoodValueHint from "@components/game/operation-effect/FoodValueHint.vue"
import { useOperationEffectContext } from "@/composables/game/useOperationEffectContext"
import { useOperationEffects } from "@/composables/game/useOperationEffects"
import { enchantFoodProbabilities } from "@services/cuisine-operation-modifiers"
import { ElMessage } from "element-plus"
import { useI18n } from "vue-i18n"
import { apiPost } from "@services/api"
import { resolveErrorMessage } from "@services/error-messages"
import { getAffixDefinition, getEquipmentEquipType, listAffixDefinitions, listRarityDefinitions } from "@services/item-definition"
import { getEmptyAffixSlotIndexes, hasAffixCapacity, isAffixFull } from "@services/equipment-affix-slots"
import { useGameStore } from "@stores/game"
import { useInventoryStore } from "@stores/inventory"
import { useInventorySummary } from "@/composables/game/useInventorySummary"
import RarityText from "@components/game/common/RarityText.vue"
import AffixQuickRef from "@components/game/affix/AffixQuickRef.vue"
import Card from "@components/game/item/Card.vue"
import EmptyCard from "@components/game/item/EmptyCard.vue"
import QuickRefWithRequire from "@components/game/item/QuickRefWithRequire.vue"
import InventoryInstanceSelectorDialog from "@components/game/inventory/InventoryInstanceSelectorDialog.vue"
import type { ItemDetailResponse } from "@/types/equipment"

type EnchantResponse = {
  failed: boolean
  affix?: { affixId?: string }
}

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
const { inventoryInstances, getInventoryQuantity } = useInventorySummary()

const selectedSlotIndex = ref<number | null>(null)
const selectedToolInstanceId = ref<number | null>(null)
const toolSelectorVisible = ref(false)
const submitting = ref(false)

const affixSlotCount = computed(() => props.detail?.operations?.affixSlotCount || 0)
const affixSlots = computed(() => props.detail?.operations?.affixSlots || [])
const targetEquipType = computed(() => getEquipmentEquipType(props.detail?.item.id || ""))
const hasAffixSlots = computed(() => hasAffixCapacity(affixSlotCount.value))
const enchantRules = computed(() => props.detail?.operations?.enchantRules || [])
const hasCompatibleAffixForRule = (rule: NonNullable<typeof enchantRules.value[number]>): boolean => {
  const hasCompatibleRarity = (allowedRarities: string[] | undefined): boolean => {
    const rarityWeights = rule.rarityWeights || {}
    const weightedRarities = Object.entries(rarityWeights)
      .filter(([, weight]) => Number.isFinite(weight) && weight > 0)
      .map(([rarityId]) => String(rarityId || "").trim())
      .filter(Boolean)
    if (weightedRarities.length === 0) return false
    const allowedSet = new Set((allowedRarities || []).map((entry) => String(entry || "").trim()).filter(Boolean))
    if (allowedSet.size === 0) return true
    return weightedRarities.some((rarityId) => allowedSet.has(rarityId))
  }

  const allowedAffixIDs = (rule.allowedAffixIds || []).map((entry) => String(entry || "").trim()).filter(Boolean)
  if (allowedAffixIDs.length > 0) {
    return allowedAffixIDs.some((affixID) => {
      const def = getAffixDefinition(affixID)
      if (!def || !def.active || def.type !== targetEquipType.value) return false
      return hasCompatibleRarity(def.allowedRarities)
    })
  }
  return listAffixDefinitions().some((def) => def.active && def.type === targetEquipType.value && hasCompatibleRarity(def.allowedRarities))
}
const compatibleEnchantRules = computed(() => enchantRules.value.filter((rule) => hasCompatibleAffixForRule(rule)))
const emptySlots = computed(() => getEmptyAffixSlotIndexes(affixSlots.value, affixSlotCount.value))
const affixSlotsFull = computed(() => isAffixFull(affixSlots.value, affixSlotCount.value))
const enchantRuleByToolItemId = computed(() => new Map(compatibleEnchantRules.value.map((rule) => [rule.toolItemId, rule])))
const allowedToolItemIdSet = computed(() => new Set(compatibleEnchantRules.value.map((rule) => rule.toolItemId)))
const costEntries = computed(() => {
  if (!selectedRule.value) return []
  return Object.entries(selectedRule.value.costs || {})
    .filter(([, qty]) => Number.isFinite(qty) && qty > 0)
    .map(([itemId, qty]) => ({ itemId, quantity: qty }))
})
const requiredCountsByItemId = computed(() => {
  const required = new Map<string, number>()
  const add = (itemId: string, qty: number) => {
    const normalizedItemId = String(itemId || "").trim()
    const normalizedQty = Number(qty)
    if (!normalizedItemId || !Number.isFinite(normalizedQty) || normalizedQty <= 0) return
    required.set(normalizedItemId, (required.get(normalizedItemId) || 0) + normalizedQty)
  }
  if (selectedRule.value && selectedToolItemId.value) {
    add(selectedToolItemId.value, selectedRule.value.consumeCount)
  }
  for (const entry of costEntries.value) {
    add(entry.itemId, entry.quantity)
  }
  return required
})
const toolCandidateInstanceIds = computed(() =>
  inventoryInstances.value
    .filter((item) => allowedToolItemIdSet.value.has(item.templateItemId))
    .map((item) => item.instanceId)
)
const enchantToolTemplateItemIds = computed(() => {
  const out: string[] = []
  const seen = new Set<string>()
  for (const rule of compatibleEnchantRules.value) {
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
const selectedRule = computed(() => enchantRuleByToolItemId.value.get(selectedToolItemId.value))
const selectedRuleAllowedAffixIds = computed(() =>
  (selectedRule.value?.allowedAffixIds || [])
    .map((entry) => String(entry || "").trim())
    .filter((entry) => !!entry && getAffixDefinition(entry)?.type === targetEquipType.value)
)
const toolOwnedCount = computed(() => (selectedToolItemId.value ? getInventoryQuantity(selectedToolItemId.value) : 0))
const effectCache = useOperationEffects()
const foodContext = computed(() => ({ domain: "enchantment" as const, actionId: "equipment.enchant", enchantWeights: selectedRule.value?.rarityWeights || {} }))
const foodState = useOperationEffectContext(() => foodContext.value)
const foodEffects = computed(() => foodState.initialized.value && !foodState.stale.value ? foodState.applicable.value : [])
const foodProbabilities = computed(() => enchantFoodProbabilities(foodContext.value.enchantWeights, foodEffects.value))
const rarityWeightEntries = computed(() => buildRarityWeightEntries(selectedRule.value?.rarityWeights || {}))
const canSubmit = computed(() => {
  if (!selectedRule.value || !selectedToolItemId.value) return false
  if (
    !submitting.value &&
    hasAffixSlots.value &&
    !affixSlotsFull.value &&
    !!gameStore.token &&
    !!props.targetInstanceId &&
    selectedSlotIndex.value !== null
  ) {
    for (const [itemId, requiredCount] of requiredCountsByItemId.value.entries()) {
      if (getInventoryQuantity(itemId) < requiredCount) {
        return false
      }
    }
    return true
  }
  return false
})

watch(
  () => props.detail,
  () => {
    selectedSlotIndex.value = emptySlots.value[0] ?? null
    selectedToolInstanceId.value = toolCandidateInstanceIds.value[0] ?? null
  },
  { immediate: true }
)

watch(
  () => toolCandidateInstanceIds.value.join(","),
  () => {
    if (!selectedToolInstanceId.value || !toolCandidateInstanceIds.value.includes(selectedToolInstanceId.value)) {
      selectedToolInstanceId.value = toolCandidateInstanceIds.value[0] ?? null
    }
  }
)

function handleSelectTool(instanceId: number | null) {
  selectedToolInstanceId.value = instanceId
}

async function submitEnchant() {
  if (!gameStore.token || !props.targetInstanceId || selectedSlotIndex.value === null || !selectedToolItemId.value) return
  submitting.value = true
  try {
    const result = await apiPost<EnchantResponse>("/api/equipment/affix/enchant", gameStore.token, {
      equipmentInstanceId: props.targetInstanceId,
      slotIndex: selectedSlotIndex.value,
      toolItemId: selectedToolItemId.value
    })
    if (result.failed) {
      ElMessage.warning(t("game.enchantFailed"))
    } else {
      ElMessage.success(t("game.enchantSuccess"))
    }
    emit("submitted")
  } catch (err) {
    ElMessage.error(resolveErrorMessage(t, err, "game.enchantFailed"))
  } finally {
    // A lost response may still have committed the enchantment and food use.
    effectCache.invalidate(gameStore.token)
    submitting.value = false
  }
}

function failLabel(failChance: number) {
  const ratio = Math.max(0, Math.min(100, Math.round(failChance * 100)))
  return t("game.enchantFailChance", { value: ratio })
}

function buildRarityWeightEntries(weights: Record<string, number>): Array<{ rarityId: string; weight: number }> {
  const rarityOrder = new Map(listRarityDefinitions().map((row) => [row.id, row.order]))
  const entries = Object.entries(weights || {})
    .filter(([, weight]) => Number.isFinite(weight) && weight > 0)
    .sort((a, b) => {
      const orderA = rarityOrder.get(a[0]) || Number.MAX_SAFE_INTEGER
      const orderB = rarityOrder.get(b[0]) || Number.MAX_SAFE_INTEGER
      if (orderA !== orderB) return orderA - orderB
      return a[0].localeCompare(b[0])
    })
  return entries.map(([rarityId, weight]) => ({ rarityId, weight }))
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
