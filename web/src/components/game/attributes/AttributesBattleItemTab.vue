<template>
  <div class="space-y-3 text-sm" data-tutorial="battle-items-panel">
    <div class="text-xs app-muted" data-tutorial="battle-items-capacity">
      {{ t("game.battleItemBagTitle") }} · {{ bagKinds }} / {{ limits.maxKinds }} · {{ bagTotalQuantity }} / {{ limits.maxTotalQuantity }}
    </div>

    <div class="grid gap-2 md:grid-cols-2 xl:grid-cols-2" v-loading="loading" data-tutorial="battle-items-slots">
      <AttributesBattleItemSlotCard
        v-for="slot in slotItems"
        :key="`battle-bag-slot-${slot.index}`"
        :slot-index="slot.index"
        :item="slot.item"
        :quantity="slotActionQuantity(slot)"
        :rule-count="slot.item ? getItemRuleCount(slot.item.itemId) : 0"
        :disabled="isDraftMode ? false : isBattleRoomPlaying"
        :show-rule-button="!isDraftMode"
        @update:quantity="setSlotActionQuantity(slot, $event)"
        @select="openBagDialog(slot.index, slot.item?.itemId || null)"
        @add="handleAddFromSlot(slot)"
        @take-back="handleTakeBackFromSlot(slot)"
        @rule="slot.item && openRuleDialog(slot.item.itemId)"
      />
    </div>

    <InventoryInstanceSelectorDialog
      v-model="bagDialogVisible"
      :title="t('game.battleItemBagTitle')"
      :instance-ids="selectorInstanceIds"
      :selected-instance-id="selectedInstanceId"
      :allow-clear="false"
      :template-item-ids="allCarryableTemplateIDs"
      :template-items-title="t('game.inventorySelectorAvailableItemsTitle', { title: t('game.battleItemBagTitle') })"
      :empty-text="t('game.battleItemBagNoAvailableInstances')"
      width="760px"
      @confirm="handleSelectInstance"
    />

    <GameDialog v-model="ruleDialogVisible" :title="ruleDialogTitle" width="560px">
      <div class="space-y-3">
        <div v-if="selectedRuleSlot" class="rounded-lg border border-[var(--app-border)] p-3">
          <ItemQuickRef :item-id="selectedRuleSlot" />
        </div>
        <RuleLogicEditor
          v-if="selectedRuleSlot"
          v-model="selectedRuleGroups"
          :max-groups="ruleLimits.maxGroups"
          :max-conditions="ruleLimits.maxConditionsPerGroup"
          :skill-options="[]"
          :item-options="ruleItemOptions"
          @validation="ruleValidation = $event"
        />
      </div>
      <template #footer>
        <el-button @click="ruleDialogVisible = false">{{ t("game.cancel") }}</el-button>
        <el-button type="primary" :disabled="!selectedRuleSlot || !ruleValidation.valid" @click="saveRule">
          {{ t("game.saveConfig") }}
        </el-button>
      </template>
    </GameDialog>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from "vue"
import { ElMessage } from "element-plus"
import { useI18n } from "vue-i18n"
import { apiGet, apiPost } from "@services/api"
import { resolveErrorMessage } from "@services/error-messages"
import RuleLogicEditor from "@components/game/rules/RuleLogicEditor.vue"
import InventoryInstanceSelectorDialog from "@components/game/inventory/InventoryInstanceSelectorDialog.vue"
import AttributesBattleItemSlotCard from "@components/game/attributes/AttributesBattleItemSlotCard.vue"
import ItemQuickRef from "@components/game/item/QuickRef.vue"
import { normalizeRuleGroups, type RuleGroup } from "@services/rule-logic"
import { getItemDefinition, listItemDefinitions } from "@services/item-definition"
import { getLocalizedItemName } from "@services/item-label"
import { useBattleItemBag } from "@/composables/game/attributes/useBattleItemBag"
import { useRealtimeStore } from "@stores/realtime"
import { useInventorySummary } from "@/composables/game/useInventorySummary"
import GameDialog from "@components/game/common/GameDialog.vue"

type RuleSet = {
  slotKey: string
  enabled: boolean
  groups: unknown
}

type SlotItem = {
  itemId: string
  label: string
  bagQuantity: number
  inventoryQuantity: number
}

type DraftBattleBagEntry = {
  slotNo: number
  itemId: string
  quantity: number
}

const props = withDefaults(defineProps<{
  token: string
  mode?: "live" | "draft"
  draftItems?: DraftBattleBagEntry[]
}>(), {
  mode: "live",
  draftItems: () => []
})

const emit = defineEmits<{
  (e: "update:draftItems", value: DraftBattleBagEntry[]): void
}>()

const { t } = useI18n()
const realtimeStore = useRealtimeStore()
const { inventoryInstances } = useInventorySummary()
const { loading, limits, bagKinds, bagTotalQuantity, battleBagEntries, inventoryCountByItem, loadAll, addQuantity, takeBackSlot } = useBattleItemBag(
  () => props.token
)
const isDraftMode = computed(() => props.mode === "draft")

const ruleLimits = ref({
  maxGroups: 6,
  maxConditionsPerGroup: 6
})
const itemRulesBySlot = ref<Record<string, { enabled: boolean; groups: RuleGroup[] }>>({})
const selectedRuleSlot = ref("")
const ruleDialogVisible = ref(false)
const ruleValidation = ref<{ valid: boolean; errors: string[] }>({ valid: true, errors: [] })
const bagDialogVisible = ref(false)
const selectedInstanceId = ref<number | null>(null)
const selectingSlotIndex = ref<number | null>(null)
const preferredTemplateItemId = ref("")
const slotDraftItemIdMap = ref<Record<number, string>>({})
const slotActionQuantityMap = ref<Record<number, number>>({})

const isBattleRoomPlaying = computed(() => realtimeStore.battleRoomStatus === "playing")

const allCarryableTemplateIDs = computed(() => {
  const out = new Set<string>()
  for (const item of listItemDefinitions()) {
    if (!item.carryableInCombat) continue
    if (!String(item.battleUsableItemId || "").trim()) continue
    out.add(item.id)
  }
  return Array.from(out)
})

const carryableInventoryInstances = computed(() =>
  inventoryInstances.value.filter((entry) => {
    if (entry.state !== "inventory") return false
    if (Math.max(0, Math.trunc(Number(entry.quantity || 0))) <= 0) return false
    return allCarryableTemplateIDs.value.includes(String(entry.templateItemId || "").trim())
  })
)

const battleBagBySlot = computed(() => {
  const map = new Map<number, { itemId: string; quantity: number }>()
  const sourceRows = isDraftMode.value ? (Array.isArray(props.draftItems) ? props.draftItems : []) : battleBagEntries.value
  for (const row of sourceRows) {
    const slotNo = Math.max(1, Math.trunc(Number(row.slotNo || 1)))
    const itemId = String(row.itemId || "").trim()
    if (!itemId) continue
    map.set(slotNo, {
      itemId,
      quantity: Math.max(0, Math.trunc(Number(row.quantity || 0)))
    })
  }
  return map
})

const slotItems = computed(() => {
  const maxSlots = Math.max(1, Math.trunc(Number(limits.value.maxKinds || 4)))
  const slots: Array<{ index: number; item: SlotItem | null }> = Array.from({ length: maxSlots }, (_, index) => ({
    index,
    item: null
  }))

  for (let index = 0; index < maxSlots; index += 1) {
    const slotNo = index + 1
    const persisted = battleBagBySlot.value.get(slotNo)
    const itemId = String(slotDraftItemIdMap.value[index] || "").trim()
    const effectiveItemId = itemId || String(persisted?.itemId || "").trim()
    if (!effectiveItemId) continue
    const persistedQuantity = itemId && persisted?.itemId === itemId
      ? persisted.quantity
      : !itemId
        ? Math.max(0, Math.trunc(Number(persisted?.quantity || 0)))
        : 0
    slots[index].item = {
      itemId: effectiveItemId,
      label: getLocalizedItemName(t, effectiveItemId, { fallbackText: effectiveItemId }),
      bagQuantity: persistedQuantity,
      inventoryQuantity: Math.max(0, Math.trunc(Number(inventoryCountByItem.value[effectiveItemId] || 0)))
    }
  }

  return slots
})

const occupiedTemplateIDsForSelector = computed(() => {
  const set = new Set<string>()
  for (const slot of slotItems.value) {
    if (!slot.item) continue
    if (selectingSlotIndex.value !== null && slot.index === selectingSlotIndex.value) continue
    set.add(slot.item.itemId)
  }
  return set
})

const selectorTemplateItemIds = computed(() => {
  const seen = new Set<string>()
  const out: string[] = []
  for (const entry of carryableInventoryInstances.value) {
    const itemId = String(entry.templateItemId || "").trim()
    if (!itemId || seen.has(itemId)) continue
    if (occupiedTemplateIDsForSelector.value.has(itemId)) continue
    seen.add(itemId)
    out.push(itemId)
  }
  return out
})

const selectorInstanceIds = computed(() => {
  const preferred = String(preferredTemplateItemId.value || "").trim()
  const filtered = carryableInventoryInstances.value.filter((entry) => {
    const itemId = String(entry.templateItemId || "").trim()
    if (!selectorTemplateItemIds.value.includes(itemId)) return false
    if (!preferred) return true
    return itemId === preferred
  })
  return filtered.map((entry) => entry.instanceId)
})

const ruleItemOptions = computed(() =>
  selectorTemplateItemIds.value.map((itemId) => ({
    value: itemId,
    label: getLocalizedItemName(t, itemId, { fallbackText: itemId })
  }))
)

const selectedRuleGroups = computed({
  get: () => {
    const key = selectedRuleSlot.value
    if (!key) return []
    if (!itemRulesBySlot.value[key]) {
      itemRulesBySlot.value[key] = { enabled: false, groups: [] }
    }
    return itemRulesBySlot.value[key].groups
  },
  set: (value: RuleGroup[]) => {
    const key = selectedRuleSlot.value
    if (!key) return
    if (!itemRulesBySlot.value[key]) {
      itemRulesBySlot.value[key] = { enabled: false, groups: [] }
    }
    itemRulesBySlot.value[key].groups = normalizeRuleGroups(value, { allowEmpty: true })
  }
})

const ruleDialogTitle = computed(() => {
  if (!selectedRuleSlot.value) return t("game.battleItemBagRuleConfigTitle")
  return `${t("game.battleItemBagRuleConfigTitle")} · ${getLocalizedItemName(t, selectedRuleSlot.value, { fallbackText: selectedRuleSlot.value })}`
})

watch(
  () => [props.token, props.mode] as const,
  async (token) => {
    if (!token[0]) return
    if (isDraftMode.value) return
    await Promise.all([loadAll(), loadRules()])
  },
  { immediate: true }
)

function openBagDialog(slotIndex: number, currentItemId: string | null) {
  selectingSlotIndex.value = slotIndex
  preferredTemplateItemId.value = String(currentItemId || "").trim()
  const preferredMatch = preferredTemplateItemId.value
    ? carryableInventoryInstances.value.find((entry) => String(entry.templateItemId || "").trim() === preferredTemplateItemId.value)?.instanceId ?? null
    : null
  selectedInstanceId.value = preferredMatch ?? selectorInstanceIds.value[0] ?? null
  bagDialogVisible.value = true
}

function handleSelectInstance(instanceId: number | null) {
  if (!instanceId) return
  const entry = carryableInventoryInstances.value.find((row) => row.instanceId === instanceId)
  if (!entry) return
  const itemId = String(entry.templateItemId || "").trim()
  if (!itemId) return
  if (selectingSlotIndex.value === null) return
  slotDraftItemIdMap.value[selectingSlotIndex.value] = itemId
  slotActionQuantityMap.value[selectingSlotIndex.value] = 1
}

function slotActionQuantity(slot: { index: number; item: SlotItem | null }) {
  const current = Math.max(1, Math.trunc(Number(slotActionQuantityMap.value[slot.index] || 1)))
  const max = slot.item ? Math.max(1, Math.trunc(Number(slot.item.inventoryQuantity || 1))) : 1
  return Math.min(current, max)
}

function setSlotActionQuantity(slot: { index: number; item: SlotItem | null }, raw: unknown) {
  const normalized = Math.max(1, Math.trunc(Number(raw || 1)))
  const max = slot.item ? Math.max(1, Math.trunc(Number(slot.item.inventoryQuantity || 1))) : 1
  slotActionQuantityMap.value[slot.index] = Math.min(normalized, max)
}

async function handleAddFromSlot(slot: { index: number; item: SlotItem | null }) {
  if (!slot.item || isBattleRoomPlaying.value) return
  const qty = Math.max(1, Math.trunc(Number(slotActionQuantity(slot) || 1)))
  if (qty > slot.item.inventoryQuantity) return
  if (isDraftMode.value) {
    const next = Array.isArray(props.draftItems) ? props.draftItems.slice() : []
    const slotNo = slot.index + 1
    const idx = next.findIndex((row) => Math.trunc(Number(row.slotNo || 0)) === slotNo)
    if (idx >= 0) {
      next[idx] = {
        slotNo,
        itemId: slot.item.itemId,
        quantity: Math.max(0, Math.trunc(Number(next[idx].quantity || 0))) + qty
      }
    } else {
      next.push({ slotNo, itemId: slot.item.itemId, quantity: qty })
    }
    emit("update:draftItems", next.sort((a, b) => a.slotNo - b.slotNo))
    return
  }
  try {
    await addQuantity(slot.index + 1, slot.item.itemId, qty)
  } catch (err) {
    ElMessage.error(resolveErrorMessage(t, err, "game.saveFailed"))
  }
}

async function handleTakeBackFromSlot(slot: { index: number; item: SlotItem | null }) {
  if (!slot.item || isBattleRoomPlaying.value) return
  if (isDraftMode.value) {
    const slotNo = slot.index + 1
    const next = (Array.isArray(props.draftItems) ? props.draftItems : [])
      .filter((row) => Math.trunc(Number(row.slotNo || 0)) !== slotNo)
    delete slotDraftItemIdMap.value[slot.index]
    slotActionQuantityMap.value[slot.index] = 1
    emit("update:draftItems", next)
    return
  }
  try {
    await takeBackSlot(slot.index + 1)
    delete slotDraftItemIdMap.value[slot.index]
    slotActionQuantityMap.value[slot.index] = 1
  } catch (err) {
    ElMessage.error(resolveErrorMessage(t, err, "game.saveFailed"))
  }
}

async function loadRules() {
  if (!props.token) return
  const payload = await apiGet<{ items: RuleSet[] }>("/api/rules/config?kind=item", props.token)
  itemRulesBySlot.value = {}
  const rows = Array.isArray(payload.items) ? payload.items : []
  for (const row of rows) {
    const key = String(row.slotKey || "").trim()
    if (!key) continue
    itemRulesBySlot.value[key] = {
      enabled: Boolean(row.enabled),
      groups: normalizeRuleGroups(row.groups, { allowEmpty: true })
    }
  }
}

function openRuleDialog(itemId: string) {
  const key = String(itemId || "").trim()
  if (!key) return
  selectedRuleSlot.value = key
  ruleDialogVisible.value = true
}

function getItemRuleCount(itemId: string): number {
  const key = String(itemId || "").trim()
  if (!key) return 0
  const state = itemRulesBySlot.value[key]
  if (!state) return 0
  return normalizeRuleGroups(state.groups, { allowEmpty: true }).filter((group) => group.conditions.length > 0).length
}

async function saveRule() {
  if (!props.token || !selectedRuleSlot.value || isDraftMode.value) return
  if (!ruleValidation.value.valid) return
  try {
    const key = selectedRuleSlot.value
    const state = itemRulesBySlot.value[key] || { enabled: false, groups: [] }
    const normalizedGroups = normalizeRuleGroups(state.groups, { allowEmpty: true })
    await apiPost("/api/rules/config", props.token, {
      ruleKind: "item",
      slotKey: key,
      enabled: normalizedGroups.some((group) => group.conditions.length > 0),
      groups: normalizedGroups
    })
    ElMessage.success(t("game.saveSuccess"))
    ruleDialogVisible.value = false
  } catch (err) {
    ElMessage.error(resolveErrorMessage(t, err, "game.saveFailed"))
  }
}
</script>
