<template>
  <div class="smelt-panel">
    <div v-if="allCarriers.length === 0" class="text-sm app-muted py-4">
      {{ t("game.smeltNoCarriers") }}
    </div>

    <template v-else>
      <!-- filter + sort toolbar -->
      <div class="smelt-toolbar">
        <div class="smelt-rarity-filters">
          <button
            v-for="rarity in rarityOptions"
            :key="rarity.id"
            class="smelt-rarity-chip"
            :class="{ 'smelt-rarity-chip--active': activeRarityFilter.has(rarity.id) }"
            :style="activeRarityFilter.has(rarity.id)
              ? { borderColor: rarity.color, backgroundColor: rarity.color + '28', color: rarity.color }
              : { borderColor: rarity.color + '80', color: rarity.color + 'cc' }"
            @click="toggleRarityFilter(rarity.id)"
          >
            {{ t(getRarityNameKey(rarity.id)) }}
          </button>
        </div>

        <div class="smelt-toolbar-right">
          <el-button size="small" @click="selectAll">{{ t("game.smeltSelectAll") }}</el-button>
          <el-button size="small" @click="clearAll">{{ t("game.smeltClearAll") }}</el-button>
          <el-button
            size="small"
            plain
            :type="sortOrder !== 'default' ? 'primary' : ''"
            @click="cycleSort"
          >
            <span v-if="sortOrder === 'default'">{{ t("game.smeltSortDefault") }}</span>
            <span v-else-if="sortOrder === 'desc'">{{ t("game.smeltSortRarityDesc") }}</span>
            <span v-else>{{ t("game.smeltSortRarityAsc") }}</span>
          </el-button>
        </div>
      </div>

      <div class="text-xs app-muted mb-2">
        {{ t("game.smeltCarrierCount", { shown: filteredCarriers.length, total: allCarriers.length }) }}
      </div>

      <!-- scrollable grid -->
      <div class="smelt-grid-scroll">
        <div v-if="filteredCarriers.length === 0" class="text-sm app-muted py-3 text-center">
          {{ t("game.smeltNoMatchingCarriers") }}
        </div>

        <div v-else class="smelt-grid">
          <div
            v-for="carrier in filteredCarriers"
            :key="carrier.instanceId"
            class="smelt-card"
            :class="{ 'smelt-card--selected': selectedIds.has(carrier.instanceId) }"
            @click="toggleCarrier(carrier.instanceId)"
          >
            <!-- selected badge -->
            <div v-if="selectedIds.has(carrier.instanceId)" class="smelt-card__badge">✓</div>

            <!-- header: icon + item name -->
            <div class="smelt-card__header">
              <Card
                :item-id="carrier.templateItemId"
                :instance-id="carrier.instanceId"
                :width="48"
                :height="48"
                hide-label
              />
              <!-- <span class="smelt-card__name">{{ getCarrierName(carrier.templateItemId) }}</span> -->
              <div class="smelt-card__affixes">
                <AffixQuickRef
                  v-for="slot in carrier.affixes"
                  :key="`sa-${carrier.instanceId}-${slot.slotIndex}`"
                  :affix-id="slot.affixId"
                  :slot="slot"
                  :enable-popover="true"
                />
              </div>
            </div>

            <!-- affixes -->


            <!-- rewards -->
            <!-- <div class="smelt-card__rewards">
              <ItemQuickRef
                v-for="(r, idx) in previewRewards(carrier)"
                :key="idx"
                :item-id="r.itemId"
                :count="r.minCount"
              ><span v-if="r.maxCount > r.minCount" class="smelt-count-sfx">~{{ r.maxCount }}</span></ItemQuickRef>
            </div> -->
          </div>
        </div>
      </div>

      <!-- sticky footer -->
      <div class="smelt-footer">
        <div class="smelt-footer__summary">
          <template v-if="selectedIds.size > 0 && totalPreviewRewards.length > 0">
            <span class="text-xs app-muted mr-1">{{ t("game.smeltPreviewRewards") }}:</span>
            <ItemQuickRef
              v-for="r in totalPreviewRewards"
              :key="r.itemId"
              :item-id="r.itemId"
              :count="r.count"
            ><span v-if="r.maxCount > r.count" class="smelt-count-sfx">~{{ r.maxCount }}</span></ItemQuickRef>
          </template>
          <span v-else-if="selectedIds.size === 0" class="text-xs app-muted">{{ t("game.smeltSelectCarriers") }}</span>
          <span v-else class="text-xs app-muted">{{ t("game.smeltNoAffix") }}</span>
        </div>

        <el-button
          type="primary"
          :disabled="selectedIds.size === 0 || submitting"
          :loading="submitting"
          @click="confirmSmelt"
        >
          {{ t("game.smeltConfirm") }}<span v-if="selectedIds.size > 0"> ({{ selectedIds.size }})</span>
        </el-button>
      </div>

      <!-- result -->
      <div v-if="lastResult" class="smelt-result">
        <span class="smelt-result__title">{{ t("game.smeltSuccess") }}</span>
        <span class="app-muted mx-1">·</span>
        <span class="text-xs app-muted mr-1">{{ t("game.smeltActualRewards") }}:</span>
        <ItemQuickRef
          v-for="r in lastResult.rewards"
          :key="r.itemId"
          :item-id="r.itemId"
          :count="r.quantity"
        />
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from "vue"
import { ElMessage, ElMessageBox } from "element-plus"
import { useI18n } from "vue-i18n"
import { apiPost } from "@services/api"
import { resolveErrorMessage } from "@services/error-messages"
import {
  getAffixDefinition,
  getDefaultAffixSmeltRule,
  getItemDefinition,
  getRarityDefinition,
  getRarityNameKey,
  isCarrierItemId,
  listRarityDefinitions
} from "@services/item-definition"
import { useGameStore } from "@stores/game"
import { useInventorySummary } from "@/composables/game/useInventorySummary"
import AffixQuickRef from "@components/game/affix/AffixQuickRef.vue"
import Card from "@components/game/item/Card.vue"
import ItemQuickRef from "@components/game/item/QuickRef.vue"
import type { AffixSmeltRewardEntry, EquipmentAffixSlot } from "@/types/equipment"

interface SmeltResult {
  consumedCarrierInstanceIds: number[]
  rewards: { itemId: string; quantity: number }[]
}

type SortOrder = "default" | "desc" | "asc"

const emit = defineEmits<{
  (e: "submitted"): void
}>()

const { t } = useI18n()
const gameStore = useGameStore()
const { inventoryInstances } = useInventorySummary()

const selectedIds = ref(new Set<number>())
const submitting = ref(false)
const lastResult = ref<SmeltResult | null>(null)
const activeRarityFilter = ref(new Set<string>())
const sortOrder = ref<SortOrder>("default")

const smeltRule = getDefaultAffixSmeltRule()

const rarityOptions = computed(() =>
  listRarityDefinitions().sort((a, b) => a.order - b.order)
)

// only carriers WITH affixes
const allCarriers = computed(() =>
  inventoryInstances.value.filter(
    (item) =>
      item.state === "inventory" &&
      item.quantity > 0 &&
      isCarrierItemId(item.templateItemId) &&
      item.affixes &&
      item.affixes.length > 0
  )
)

function carrierMaxRarityOrder(carrier: (typeof allCarriers.value)[number]): number {
  return (carrier.affixes ?? []).reduce((max, slot) => {
    const ord = getRarityDefinition(slot.rarityId)?.order ?? 0
    return ord > max ? ord : max
  }, 0)
}

const filteredCarriers = computed(() => {
  let list = allCarriers.value

  if (activeRarityFilter.value.size > 0) {
    list = list.filter((c) =>
      (c.affixes ?? []).some((slot) => activeRarityFilter.value.has(slot.rarityId))
    )
  }

  if (sortOrder.value === "desc") {
    list = [...list].sort((a, b) => carrierMaxRarityOrder(b) - carrierMaxRarityOrder(a))
  } else if (sortOrder.value === "asc") {
    list = [...list].sort((a, b) => carrierMaxRarityOrder(a) - carrierMaxRarityOrder(b))
  }

  return list
})

function toggleRarityFilter(rarityId: string) {
  const next = new Set(activeRarityFilter.value)
  if (next.has(rarityId)) next.delete(rarityId)
  else next.add(rarityId)
  activeRarityFilter.value = next
}

function cycleSort() {
  sortOrder.value =
    sortOrder.value === "default" ? "desc" : sortOrder.value === "desc" ? "asc" : "default"
}

function toggleCarrier(id: number) {
  const next = new Set(selectedIds.value)
  if (next.has(id)) next.delete(id)
  else next.add(id)
  selectedIds.value = next
}

function selectAll() {
  selectedIds.value = new Set(filteredCarriers.value.map((c) => c.instanceId))
}

function clearAll() {
  selectedIds.value = new Set()
}

function getCarrierName(itemId: string): string {
  const def = getItemDefinition(itemId)
  if (!def) return itemId
  return t(def.nameKey as never) || itemId
}

function previewRewards(
  carrier: (typeof allCarriers.value)[number]
): { itemId: string; minCount: number; maxCount: number }[] {
  if (!smeltRule || !carrier.affixes) return []
  const accum: Record<string, { itemId: string; minCount: number; maxCount: number }> = {}
  for (const slot of carrier.affixes) {
    for (const e of resolveSmeltEntries(slot)) {
      if (!accum[e.itemId]) accum[e.itemId] = { itemId: e.itemId, minCount: 0, maxCount: 0 }
      accum[e.itemId].minCount += e.minCount
      accum[e.itemId].maxCount += e.maxCount
    }
  }
  return Object.values(accum)
}

function resolveSmeltEntries(slot: EquipmentAffixSlot): AffixSmeltRewardEntry[] {
  if (!smeltRule) return []
  const affixDef = getAffixDefinition(slot.affixId)
  if (affixDef?.smeltOverride) {
    const override = affixDef.smeltOverride[slot.rarityId]
    if (override?.length) return override
  }
  return smeltRule.rarityRewards[slot.rarityId] || []
}

const totalPreviewRewards = computed(() => {
  const accum: Record<string, { itemId: string; count: number; maxCount: number }> = {}
  for (const carrier of allCarriers.value) {
    if (!selectedIds.value.has(carrier.instanceId)) continue
    for (const r of previewRewards(carrier)) {
      if (!accum[r.itemId]) accum[r.itemId] = { itemId: r.itemId, count: 0, maxCount: 0 }
      accum[r.itemId].count += r.minCount
      accum[r.itemId].maxCount += r.maxCount
    }
  }
  return Object.values(accum)
})

async function confirmSmelt() {
  if (selectedIds.value.size === 0 || !gameStore.token) return
  try {
    await ElMessageBox.confirm(
      t("game.smeltConfirmMessage", { count: selectedIds.value.size }),
      t("game.smeltConfirm"),
      { type: "warning", confirmButtonText: t("game.confirm"), cancelButtonText: t("game.cancel") }
    )
  } catch {
    return
  }
  submitting.value = true
  lastResult.value = null
  try {
    const result = await apiPost<SmeltResult>("/api/equipment/affix/smelt", gameStore.token, {
      carrierInstanceIds: Array.from(selectedIds.value)
    })
    lastResult.value = result
    selectedIds.value = new Set()
    ElMessage.success(t("game.smeltSuccess"))
    emit("submitted")
  } catch (err) {
    ElMessage.error(resolveErrorMessage(t, err, "game.itemOperationFailed"))
  } finally {
    submitting.value = false
  }
}
</script>

<style scoped>
.smelt-panel {
  display: flex;
  flex-direction: column;
}

/* ─── toolbar ─── */
.smelt-toolbar {
  display: flex;
  flex-wrap: wrap;
  align-items: flex-start;
  gap: 8px;
  margin-bottom: 8px;
}

.smelt-rarity-filters {
  display: flex;
  flex-wrap: wrap;
  gap: 5px;
  flex: 1;
  min-width: 0;
}

.smelt-rarity-chip {
  padding: 3px 9px;
  border-radius: 99px;
  border: 1px solid;
  font-size: 12px;
  line-height: 1.5;
  cursor: pointer;
  background: transparent;
  transition: background-color 0.15s, opacity 0.15s;
  white-space: nowrap;
  touch-action: manipulation;
  -webkit-tap-highlight-color: transparent;
}

.smelt-rarity-chip--active {
  font-weight: 600;
}

.smelt-toolbar-right {
  display: flex;
  gap: 5px;
  flex-shrink: 0;
  flex-wrap: wrap;
}

/* ─── scroll container ─── */
.smelt-grid-scroll {
  max-height: 420px;
  overflow-y: auto;
  overflow-x: hidden;
  padding-right: 2px;
}

/* ─── grid ─── */
.smelt-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(210px, 1fr));
  gap: 8px;
  padding-bottom: 4px;
}

/* ─── card ─── */
.smelt-card {
  position: relative;
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 10px;
  border: 1.5px solid var(--el-border-color);
  border-radius: 8px;
  cursor: pointer;
  transition: border-color 0.15s, background-color 0.15s, box-shadow 0.15s;
  touch-action: manipulation;
  -webkit-tap-highlight-color: transparent;
  user-select: none;
}

.smelt-card:hover {
  border-color: var(--el-color-primary-light-3);
  box-shadow: 0 1px 6px rgba(0, 0, 0, 0.06);
}

.smelt-card--selected {
  border-color: var(--el-color-primary);
  background-color: var(--el-color-primary-light-9);
  box-shadow: 0 0 0 1px var(--el-color-primary-light-5);
}

/* selected checkmark badge */
.smelt-card__badge {
  position: absolute;
  top: -1px;
  right: -1px;
  width: 20px;
  height: 20px;
  background: var(--el-color-primary);
  color: #fff;
  font-size: 11px;
  font-weight: 700;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 0 8px 0 8px;
  line-height: 1;
}

/* header row: icon + name */
.smelt-card__header {
  display: flex;
  align-items: center;
  gap: 7px;
  min-width: 0;
}

.smelt-card__name {
  font-size: 12px;
  font-weight: 500;
  line-height: 1.3;
  overflow: hidden;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  word-break: break-all;
  flex: 1;
  min-width: 0;
}

/* affixes */
.smelt-card__affixes {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}

/* rewards */
.smelt-card__rewards {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  padding-top: 2px;
}

.smelt-count-sfx {
  font-size: 11px;
  opacity: 0.75;
}

/* ─── sticky footer ─── */
.smelt-footer {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 0 4px;
  border-top: 1px solid var(--el-border-color-lighter);
  margin-top: 8px;
  position: sticky;
  bottom: 0;
  background: var(--el-bg-color);
  z-index: 1;
}

.smelt-footer__summary {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 5px;
}

/* ─── result ─── */
.smelt-result {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 5px;
  padding: 8px 0 0;
  font-size: 12px;
}

.smelt-result__title {
  font-weight: 600;
  color: var(--el-color-success);
}

/* ─── mobile ─── */
@media (max-width: 520px) {
  .smelt-grid {
    grid-template-columns: repeat(auto-fill, minmax(130px, 1fr));
    gap: 6px;
  }

  .smelt-card {
    padding: 8px;
    gap: 5px;
  }

  .smelt-rarity-chip {
    font-size: 11px;
    padding: 2px 7px;
  }

  .smelt-toolbar-right {
    width: 100%;
    justify-content: flex-end;
  }

  .smelt-grid-scroll {
    max-height: 360px;
  }

  .smelt-footer {
    flex-wrap: wrap;
  }

  .smelt-footer__summary {
    width: 100%;
    order: 2;
  }

  .smelt-footer :deep(.el-button) {
    width: 100%;
    order: 1;
  }
}
</style>
