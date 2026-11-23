<template>
  <div class="page-shell space-y-4">
    <GamePageFrame
      title-key="game.menu.warehouse"
      tutorial-id="warehouse"
      :visual-src="warehouseVisual.src"
      :visual-tone="warehouseVisual.tone"
      visual-key="inventory"
      data-tutorial="warehouse-page"
    >
      <template #actions>
        <el-button size="small" @click="refreshCurrentTab">{{ t("game.refresh") }}</el-button>
      </template>

      <el-tabs v-model="activeTab" data-tutorial="warehouse-tabs">
        <el-tab-pane :label="t('game.inventoryTab')" name="inventory" :lazy="true">
          <div data-tutorial="warehouse-items">
          <ItemBrowserPanel
            :items="inventoryBrowserItems"
            :empty-text="t('game.warehouseEmpty')"
            :search-placeholder="t('game.inventorySearchPlaceholder')"
            search-width-class="md:w-32!"
            :grid-gap-class="warehouseGridGapClass"
            :grid-min-width="warehouseGridMinWidth"
            :virtualized="true"
            tutorial-scope="warehouse-inventory"
            @item-click="handleInventoryItemClick"
          >
            <template #sort-icon>
              <el-icon><Icon icon="material-symbols:settings-outline" /></el-icon>
            </template>
            <template #toolbar-extra="{ close }">
              <el-divider class="my-2!" />
              <template v-if="!batchMode">
                <div class="px-3 py-1" @click.stop>
                  <el-button
                    size="small"
                    type="primary"
                    plain
                    style="width: 100%"
                    data-tutorial="warehouse-batch-archive"
                    @click="enterBatchArchiveMode(); close()"
                  >
                    {{ t("game.batchArchive") }}
                  </el-button>
                </div>
                <div class="px-3 py-1" @click.stop>
                  <el-button
                    size="small"
                    type="warning"
                    plain
                    style="width: 100%"
                    data-tutorial="warehouse-batch-recycle"
                    @click="enterBatchRecycleMode(); close()"
                  >
                    {{ t("game.batchRecycle") }}
                  </el-button>
                </div>
                <div class="px-3 py-1" @click.stop>
                  <el-button size="small" type="danger" plain style="width: 100%" @click="enterDestructiveRecycleMode(); close()">
                    <Icon icon="lucide:trash-2" />{{ t("game.destructiveRecycle.title") }}
                  </el-button>
                </div>
              </template>
              <template v-else>
                <div class="px-3 py-1 space-y-1" @click.stop>
                  <el-button
                    size="small"
                    :type="batchActionType"
                    :loading="batchActionSubmitting"
                    :disabled="batchSelectedIds.size === 0"
                    style="width: 100%"
                    @click="openBatchConfirm(); close()"
                  >
                    {{ batchActionLabel }}
                    <template v-if="batchSelectedIds.size > 0"> ({{ batchSelectedIds.size }})</template>
                  </el-button>
                  <el-button size="small" style="width: 100%; margin-left: 0" @click="exitBatchMode(); close()">
                    {{ batchExitLabel }}
                  </el-button>
                </div>
              </template>
            </template>
            <template #item="{ item, onClick }">
              <div
                class="batch-card-wrapper"
                :class="batchMode ? {
                  'batch-card-wrapper--selected': item.instanceId != null && batchSelectedIds.has(item.instanceId),
                  'batch-card-wrapper--unselectable': batchRecycleMode && item.instanceId != null && !isRecyclableInstance(item.instanceId),
                } : {}"
                @click="batchMode ? onBatchCardClick(item) : onClick()"
              >
                <div v-if="batchMode && item.instanceId != null && batchSelectedIds.has(item.instanceId)" class="batch-card-badge">✓</div>
                <Card
                  :quantity="item.quantity || 0"
                  :item-id="item.itemId"
                  :instance-id="item.instanceId"
                  :instance-preview="item.payload || null"
                  :width="warehouseCardSize"
                  :height="warehouseCardSize"
                  :icon-size="warehouseCardIconSize"
                />
              </div>
            </template>
          </ItemBrowserPanel>
          </div>

        </el-tab-pane>

        <el-tab-pane :label="t('game.inventoryArchivedTab')" name="archived" :lazy="true">
          <div data-tutorial="warehouse-archive">
          <ItemBrowserPanel
            :items="archivedBrowserItems"
            :empty-text="t('game.warehouseArchivedEmpty')"
            :search-placeholder="t('game.inventorySearchPlaceholder')"
            search-width-class="md:w-32!"
            :grid-gap-class="warehouseGridGapClass"
            :grid-min-width="warehouseGridMinWidth"
            :virtualized="true"
            tutorial-scope="warehouse-archive"
            @item-click="handleInventoryItemClick"
          >
            <template #sort-icon>
              <el-icon><Icon icon="material-symbols:settings-outline" /></el-icon>
            </template>
            <template #toolbar-extra="{ close }">
              <el-divider class="my-2!" />
              <template v-if="!batchMode">
                <div class="px-3 py-1" @click.stop>
                  <el-button
                    size="small"
                    type="primary"
                    plain
                    style="width: 100%"
                    data-tutorial="warehouse-batch-unarchive"
                    @click="enterBatchUnarchiveMode(); close()"
                  >
                    {{ t("game.batchUnarchive") }}
                  </el-button>
                </div>
                <div class="px-3 py-1" @click.stop>
                  <el-button
                    size="small"
                    type="warning"
                    plain
                    style="width: 100%"
                    data-tutorial="warehouse-batch-recycle"
                    @click="enterBatchRecycleMode(); close()"
                  >
                    {{ t("game.batchRecycle") }}
                  </el-button>
                </div>
                <div class="px-3 py-1" @click.stop>
                  <el-button size="small" type="danger" plain style="width: 100%" @click="enterDestructiveRecycleMode(); close()">
                    <Icon icon="lucide:trash-2" />{{ t("game.destructiveRecycle.title") }}
                  </el-button>
                </div>
              </template>
              <template v-else>
                <div class="px-3 py-1 space-y-1" @click.stop>
                  <el-button
                    size="small"
                    :type="batchActionType"
                    :loading="batchActionSubmitting"
                    :disabled="batchSelectedIds.size === 0"
                    style="width: 100%"
                    @click="openBatchConfirm(); close()"
                  >
                    {{ batchActionLabel }}
                    <template v-if="batchSelectedIds.size > 0"> ({{ batchSelectedIds.size }})</template>
                  </el-button>
                  <el-button size="small" style="width: 100%; margin-left: 0" @click="exitBatchMode(); close()">
                    {{ batchExitLabel }}
                  </el-button>
                </div>
              </template>
            </template>
            <template #item="{ item, onClick }">
              <div
                class="batch-card-wrapper"
                :class="batchMode ? {
                  'batch-card-wrapper--selected': item.instanceId != null && batchSelectedIds.has(item.instanceId),
                  'batch-card-wrapper--unselectable': batchMode === 'destructive' && item.instanceId != null && !isRecyclableInstance(item.instanceId),
                } : {}"
                @click="batchMode ? onBatchCardClick(item) : onClick()"
              >
                <div v-if="batchMode && item.instanceId != null && batchSelectedIds.has(item.instanceId)" class="batch-card-badge">✓</div>
                <Card
                  :quantity="item.quantity || 0"
                  :item-id="item.itemId"
                  :instance-id="item.instanceId"
                  :instance-preview="item.payload || null"
                  :width="warehouseCardSize"
                  :height="warehouseCardSize"
                  :icon-size="warehouseCardIconSize"
                />
              </div>
            </template>
          </ItemBrowserPanel>
          </div>
        </el-tab-pane>

        <el-tab-pane v-if="hasCollectibles" :label="t('game.collectiblesTab')" name="collectibles" :lazy="true">
          <div v-loading="collectiblesLoading" class="collectibles-grid" data-tutorial="warehouse-collectibles">
            <template
              v-for="c in collectibles"
              :key="c.collectibleId"
              class="app-card collectible-card"
              shadow="hover"
            >
              <CollectibleItem
                :item-id="c.itemId"
                :name-key="c.nameKey"
                :desc-key="c.descKey"
                :fallback-name="c.collectibleId"
                :icon-size="64"
                :show-date="false"
                :level="c.upgradeable ? c.level : 0"
              >
                <div v-if="hasMods(c.playerMods)" class="mt-1.5">
                  <div class="text-[10px] text-[var(--app-muted)] mb-0.5">{{ t("collectible.playerBonus") }}</div>
                  <div class="flex flex-wrap gap-1">
                    <StatusTag
                      v-for="entry in modEntries(c.playerMods)"
                      :key="`player-${c.collectibleId}-${entry.attrKey}`"
                      :attr-key="entry.attrKey"
                      :value="entry.value"
                      :signed="true"
                    />
                  </div>
                </div>
                <div v-if="hasMods(c.kittyMods)" class="mt-1.5">
                  <div class="text-[10px] text-[var(--app-muted)] mb-0.5">{{ t("collectible.kittyBonus") }}</div>
                  <div class="flex flex-wrap gap-1">
                    <StatusTag
                      v-for="entry in modEntries(c.kittyMods)"
                      :key="`kitty-${c.collectibleId}-${entry.attrKey}`"
                      :attr-key="entry.attrKey"
                      :value="entry.value"
                      :signed="true"
                    />
                  </div>
                </div>
                <div class="mt-1.5 text-[10px] text-[var(--app-muted)]">
                  {{ t("collectible.obtainedAt") }}: {{ new Date(c.obtainedAt).toLocaleDateString() }}
                </div>
              </CollectibleItem>
            </template>
          </div>
        </el-tab-pane>

        <el-tab-pane :label="t('game.warehouseLogTitle')" name="log" :lazy="true">
          <div data-tutorial="warehouse-log">
          <div v-loading="logLoading">
            <el-tabs v-model="activeLogCategory" class="warehouse-log-category-tabs">
              <el-tab-pane
                v-for="category in logCategoryTabs"
                :key="category"
                :label="auditCategoryLabel(category)"
                :name="category"
                :lazy="true"
              />
            </el-tabs>

            <div v-if="displayLogRows.length === 0" class="text-sm app-muted py-4">
              {{ t("game.warehouseLogEmpty") }}
            </div>
            <div v-else class="space-y-2">
              <el-card v-for="row in displayLogRows" :key="row.key" class="app-card">
                <div v-if="row.kind === 'alchemy'" class="flex items-start justify-between gap-3 text-sm">
                  <div class="min-w-0 flex-1">
                    <div class="font-semibold">{{ alchemyLogTitle(row.group) }}</div>
                    <div class="alchemy-log-flow">
                      <div class="alchemy-log-part">
                        <span class="alchemy-log-part__label">{{ alchemyCostLabel(row.group) }}</span>
                        <ItemQuickRef
                          v-for="entry in row.group.costs"
                          :key="`cost-${row.key}-${entry.itemId}`"
                          :item-id="entry.itemId"
                          :count="entry.quantity"
                          normal-number-format
                        />
                        <span v-if="row.group.costs.length === 0" class="app-muted">-</span>
                      </div>
                      <span class="alchemy-log-flow__arrow">→</span>
                      <div class="alchemy-log-part">
                        <span class="alchemy-log-part__label">{{ alchemyRewardLabel(row.group) }}</span>
                        <template v-if="row.group.rewards.length > 0">
                          <template v-for="entry in row.group.rewards" :key="`reward-${row.key}-${entry.itemId}-${entry.instanceId || 0}`">
                            <Card
                              v-if="row.group.kind === 'bench' && entry.instanceId"
                              :item-id="entry.itemId"
                              :instance-id="entry.instanceId"
                              :quantity="entry.quantity"
                              :width="56"
                              :height="56"
                              @click="openDetail(entry.itemId, entry.instanceId)"
                            />
                            <ItemQuickRef
                              v-else
                              :item-id="entry.itemId"
                              :count="entry.quantity"
                              normal-number-format
                            />
                          </template>
                        </template>
                        <span v-else>{{ alchemyEmptyRewardLabel(row.group) }}</span>
                      </div>
                    </div>
                    <div v-if="row.group.kind === 'bench' && alchemyBenchInstanceSummary(row.group)" class="text-xs app-muted">
                      {{ alchemyBenchInstanceSummary(row.group) }}
                    </div>
                    <div class="text-xs app-muted">
                      {{ t("game.warehouseLogSurfaceFlow") }}: {{ alchemySurfaceSummary(row.group) }}
                    </div>
                  </div>
                  <div class="text-xs app-muted">{{ formatTime(row.group.createdAt) }}</div>
                </div>
                <div v-else class="flex items-start justify-between gap-3 text-sm">
                  <div>
                    <div class="font-semibold">{{ auditLabel(row.item) }}</div>
                    <div class="app-muted flex flex-wrap items-center gap-2">
                      <ItemQuickRef
                        v-for="entry in logItemEntries(row.item)"
                        :key="entry.itemId"
                        :item-id="entry.itemId"
                        :count="entry.quantity"
                        normal-number-format
                      />
                    </div>
                    <div class="text-xs app-muted">
                      {{ t("game.warehouseLogSurfaceFlow") }}: {{ surfaceLabel(row.item.sourceSurface) }} →
                      {{ surfaceLabel(row.item.targetSurface) }}
                    </div>
                  </div>
                  <div class="text-xs app-muted">{{ formatTime(row.item.createdAt) }}</div>
                </div>
              </el-card>
            </div>
          </div>
          </div>
        </el-tab-pane>
      </el-tabs>
    </GamePageFrame>

    <ItemDetailDialog
      v-model="itemDetailVisible"
      :item-id="detailItemId"
      :instance-id="detailInstanceId"
      @updated="handleItemDetailUpdated"
    />

    <ItemDestructiveRecycleDialog
      v-model="destructiveConfirmVisible"
      :items="batchSelectedInstances"
      @done="onBatchRecycleDone"
    />
    <ItemBatchRecycleDialog
      v-model="batchConfirmVisible"
      :items="batchSelectedInstances"
      @done="onBatchRecycleDone"
    />

    <BatchActionBar :visible="Boolean(batchMode)" tutorial-id="warehouse-batch-action-bar">
      <template #hint>{{ t("game.batchRecycleSelected", { n: batchSelectedIds.size }) }}</template>
      <el-button
        v-if="batchSameItemSelectVisible"
        size="small"
        plain
        :type="batchActionType"
        :disabled="batchActionSubmitting"
        @click="selectAllFocusedBatchItem"
      >
        {{ batchSameItemSelectLabel }}
      </el-button>
      <el-button size="small" :disabled="batchActionSubmitting" @click="exitBatchMode">
        {{ batchExitLabel }}
      </el-button>
      <el-button
        size="small"
        :type="batchActionType"
        :loading="batchActionSubmitting"
        :disabled="batchSelectedIds.size === 0"
        @click="openBatchConfirm"
      >
        {{ batchActionLabel }}
        <template v-if="batchSelectedIds.size > 0"> ({{ batchSelectedIds.size }})</template>
      </el-button>
    </BatchActionBar>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue"
import { ElMessage } from "element-plus"
import { Icon } from "@iconify/vue"
import { useI18n } from "vue-i18n"
import { apiPost } from "@services/api"
import { resolveErrorMessage } from "@services/error-messages"
import {
  INVENTORY_AUDIT_CATEGORIES,
  type InventoryAuditCategoryFilter
} from "@services/inventory-audit"
import { getItemDefinition } from "@services/item-definition"
import { getLocalizedItemName } from "@services/item-label"
import { getRecycleTemplateMatchKind } from "@composables/game/useInventorySummary"
import { useDevice } from "@composables/game/useDevice"
import { useGameStore } from "@stores/game"
import { useInventoryStore } from "@stores/inventory"
import { useWarehouseStore, type WarehouseLogItem } from "@stores/warehouse"
import { useCollectiblesStore } from "@stores/collectibles"
import StatusTag from "@components/game/attribute/StatusTag.vue"
import CollectibleItem from "@components/common/CollectibleItem.vue"
import ItemDetailDialog from "@components/game/item/ItemDetailDialog.vue"
import ItemBatchRecycleDialog from "@components/game/item/ItemBatchRecycleDialog.vue"
import ItemDestructiveRecycleDialog from "@components/game/item/ItemDestructiveRecycleDialog.vue"
import { isDestructiveRecycleCandidate } from "@services/destructive-recycle"
import ItemBrowserPanel, { type ItemBrowserItem } from "@components/game/item/ItemBrowserPanel.vue"
import ItemQuickRef from "@components/game/item/QuickRef.vue"
import type { InventoryGridItem } from "@components/game/inventory/types"
import Card from "@/components/game/item/Card.vue"
import BatchActionBar from "@components/game/common/BatchActionBar.vue"
import GamePageFrame from "@components/game/common/GamePageFrame.vue"
import { gamePageVisuals } from "@services/game-page-visuals"
import type { InventoryInstance } from "@/types/equipment"

type ModEntry = {
  attrKey: string
  value: number
}

type LogItemEntry = {
  itemId: string
  quantity: number
  instanceId?: number | null
}

type AlchemyLogKind = "convert" | "bench" | "transmute" | "other"

type AlchemyLogDisplay = {
  key: string
  actionId: string
  kind: AlchemyLogKind
  runNo: number | null
  createdAt: string
  costs: LogItemEntry[]
  rewards: LogItemEntry[]
  sourceSurfaces: string[]
  targetSurfaces: string[]
  carrierInstanceId: number | null
  targetCarrierInstanceId: number | null
  failed: boolean | null
}

type AlchemyLogDraft = Omit<AlchemyLogDisplay, "costs" | "rewards" | "sourceSurfaces" | "targetSurfaces"> & {
  costMap: Map<string, number>
  rewardMap: Map<string, number>
  rewardInstances: LogItemEntry[]
  sourceSurfaceSet: Set<string>
  targetSurfaceSet: Set<string>
}

type WarehouseLogDisplayRow =
  | { kind: "alchemy"; key: string; group: AlchemyLogDisplay }
  | { kind: "log"; key: string; item: WarehouseLogItem }

type WarehouseTab = "inventory" | "archived" | "log" | "collectibles"
type BatchMode = "recycle" | "destructive" | "archive" | "unarchive" | null
const BATCH_SELECTION_LIMIT = 1000

const WAREHOUSE_TUTORIAL_SWITCH_TAB_EVENT = "warehouse:tutorial-switch-tab"
const WAREHOUSE_TUTORIAL_OPEN_DETAIL_EVENT = "warehouse:tutorial-open-detail"
const WAREHOUSE_TUTORIAL_CLOSE_DETAIL_EVENT = "warehouse:tutorial-close-detail"

const { t, te } = useI18n()
const { isMobile } = useDevice()
const warehouseVisual = gamePageVisuals.warehouse
const store = useGameStore()
const inventoryStore = useInventoryStore()
const warehouseStore = useWarehouseStore()

const collectiblesStore = useCollectiblesStore()
const collectibles = computed(() => collectiblesStore.collectibles)
const hasCollectibles = computed(() => collectibles.value.length > 0)
const collectiblesLoading = ref(false)

const activeTab = ref<WarehouseTab>("inventory")
const activeLogCategory = ref<InventoryAuditCategoryFilter>("all")
const itemDetailVisible = ref(false)
const detailItemId = ref("")
const detailInstanceId = ref<number | null>(null)
const logLoading = ref(false)

// batch inventory operation state
const batchMode = ref<BatchMode>(null)
const batchSelectedIds = ref(new Set<number>())
const batchFocusedItemId = ref("")
const batchConfirmVisible = ref(false)
const destructiveConfirmVisible = ref(false)
const batchActionSubmitting = ref(false)

const logItems = computed<WarehouseLogItem[]>(() => warehouseStore.logItems)
const displayLogRows = computed<WarehouseLogDisplayRow[]>(() => buildWarehouseLogRows(logItems.value))
const logCategoryTabs: InventoryAuditCategoryFilter[] = ["all", ...INVENTORY_AUDIT_CATEGORIES]
const flattenedItems = computed<InventoryGridItem[]>(() =>
  inventoryStore.instances
    .filter((item) => item.state === "inventory" && !item.archivedAt)
    .map(toInventoryGridItem)
)
const archivedFlattenedItems = computed<InventoryGridItem[]>(() =>
  inventoryStore.instances
    .filter((item) => item.state === "inventory" && !!item.archivedAt)
    .map(toInventoryGridItem)
)
const inventoryBrowserItems = computed<ItemBrowserItem[]>(() =>
  flattenedItems.value.map(toBrowserItem)
)
const archivedBrowserItems = computed<ItemBrowserItem[]>(() =>
  archivedFlattenedItems.value.map(toBrowserItem)
)
const batchRecycleMode = computed(() => batchMode.value === "recycle" || batchMode.value === "destructive")
const batchActionType = computed(() => batchMode.value === "destructive" ? "danger" : batchRecycleMode.value ? "warning" : "primary")
const batchActionLabel = computed(() => {
  if (batchMode.value === "destructive") return t("game.destructiveRecycle.title")
  if (batchMode.value === "archive") return t("game.batchArchiveConfirm")
  if (batchMode.value === "unarchive") return t("game.batchUnarchiveConfirm")
  return t("game.batchRecycleConfirm")
})
const batchExitLabel = computed(() => {
  if (batchMode.value === "archive") return t("game.batchArchiveExit")
  if (batchMode.value === "unarchive") return t("game.batchUnarchiveExit")
  return t("game.batchRecycleExit")
})
const warehouseGridGapClass = computed(() => (isMobile.value ? "gap-2" : "gap-4"))
const warehouseGridMinWidth = computed(() => (isMobile.value ? 56 : 64))
const warehouseCardSize = computed(() => (isMobile.value ? 48 : 56))
const warehouseCardIconSize = computed(() => (isMobile.value ? 28 : 32))
const batchFocusedTemplateItemId = computed(() => {
  if (batchFocusedItemId.value && batchSelectedItemIds.value.has(batchFocusedItemId.value)) {
    return batchFocusedItemId.value
  }
  return batchSelectedInstances.value[0]?.templateItemId || ""
})
const batchSelectedItemIds = computed(() => {
  const out = new Set<string>()
  for (const inst of batchSelectedInstances.value) {
    out.add(inst.templateItemId)
  }
  return out
})
const batchFocusedSameItemInstances = computed(() => {
  const itemId = batchFocusedTemplateItemId.value
  if (!itemId) return []
  return batchSelectableInstances.value.filter((inst) => inst.templateItemId === itemId)
})
const batchFocusedSameItemUnselectedCount = computed(() =>
  batchFocusedSameItemInstances.value.filter((inst) => !batchSelectedIds.value.has(inst.instanceId)).length
)
const batchSameItemSelectVisible = computed(() =>
  batchFocusedSameItemInstances.value.length > 5 && batchFocusedSameItemUnselectedCount.value > 0
)
const batchSameItemSelectLabel = computed(() => {
  const itemId = batchFocusedTemplateItemId.value
  const name = itemId ? itemLabel(itemId) : ""
  return t("game.batchSelectAllSameItem", { name, n: batchFocusedSameItemInstances.value.length })
})
const batchSelectableInstances = computed(() =>
  inventoryStore.instances.filter((inst) => isBatchSelectableInstance(inst))
)

function toInventoryGridItem(item: InventoryInstance): InventoryGridItem {
  return {
    instanceId: item.instanceId,
    templateItemId: item.templateItemId,
    enhanceLevel: item.enhanceLevel || 0,
    quantity: item.quantity || 0,
    fishData: item.fishData
  }
}

function toBrowserItem(item: InventoryGridItem): ItemBrowserItem {
  return {
    key: item.instanceId,
    itemId: item.templateItemId,
    itemTypes: item.fishData ? ["fish"] : getItemDefinition(item.templateItemId)?.type || [],
    instanceId: item.instanceId,
    quantity: item.quantity,
    payload: item.fishData ? { fishData: item.fishData } : undefined
  }
}

function getInstanceById(instanceId: number) {
  return inventoryStore.instancesById.get(instanceId)
}

function isRecyclableInstance(instanceId: number): boolean {
  const inst = getInstanceById(instanceId)
  if (!inst) return false
  if (batchMode.value === "destructive") return isDestructiveRecycleCandidate(inst)
  if (getRecycleTemplateMatchKind(inst) === "not_recyclable") return false
  return (getItemDefinition(inst.templateItemId)?.economyBaseValue ?? 0) > 0
}

function isBatchSelectableInstance(inst: InventoryInstance): boolean {
  if (inst.state !== "inventory") return false
  if (batchMode.value === "archive" && inst.archivedAt) return false
  if (batchMode.value === "unarchive" && !inst.archivedAt) return false
  if (batchRecycleMode.value) {
    if (activeTab.value === "archived" && !inst.archivedAt) return false
    if (activeTab.value !== "archived" && inst.archivedAt) return false
    return isRecyclableInstance(inst.instanceId)
  }
  return batchMode.value === "archive" || batchMode.value === "unarchive"
}

const batchSelectedInstances = computed(() => {
  const result = []
  for (const id of batchSelectedIds.value) {
    const inst = getInstanceById(id)
    if (inst) result.push(inst)
  }
  return result
})

function isWarehouseTab(value: unknown): value is WarehouseTab {
  return value === "inventory" || value === "archived" || value === "collectibles" || value === "log"
}

function handleWarehouseTutorialSwitchTab(event: Event) {
  const detail = (event as CustomEvent<{ tab?: unknown } | WarehouseTab>).detail
  const tab = typeof detail === "string" ? detail : detail?.tab
  if (!isWarehouseTab(tab)) return
  if (tab === "collectibles" && !hasCollectibles.value) return
  activeTab.value = tab
}

async function handleWarehouseTutorialOpenDetail() {
  activeTab.value = "inventory"
  if (batchMode.value) exitBatchMode()
  await refreshInventory(false)
  const targetItem = inventoryBrowserItems.value.find((item) => item.instanceId != null)
  if (!targetItem?.instanceId) return
  openDetail(targetItem.itemId, targetItem.instanceId)
}

function handleWarehouseTutorialCloseDetail() {
  itemDetailVisible.value = false
}

onMounted(() => {
  window.addEventListener(WAREHOUSE_TUTORIAL_SWITCH_TAB_EVENT, handleWarehouseTutorialSwitchTab)
  window.addEventListener(WAREHOUSE_TUTORIAL_OPEN_DETAIL_EVENT, handleWarehouseTutorialOpenDetail)
  window.addEventListener(WAREHOUSE_TUTORIAL_CLOSE_DETAIL_EVENT, handleWarehouseTutorialCloseDetail)
  store.restoreToken()
  void refreshInventory(false)
  if (store.token) void refreshCollectibles()
})

onBeforeUnmount(() => {
  window.removeEventListener(WAREHOUSE_TUTORIAL_SWITCH_TAB_EVENT, handleWarehouseTutorialSwitchTab)
  window.removeEventListener(WAREHOUSE_TUTORIAL_OPEN_DETAIL_EVENT, handleWarehouseTutorialOpenDetail)
  window.removeEventListener(WAREHOUSE_TUTORIAL_CLOSE_DETAIL_EVENT, handleWarehouseTutorialCloseDetail)
})

watch(
  () => activeTab.value,
  (tab) => {
    if (batchMode.value) exitBatchMode()
    if (tab !== "log") return
    void refreshLogs(false)
  }
)

watch(
  () => activeLogCategory.value,
  () => {
    if (activeTab.value !== "log") return
    void refreshLogs(false)
  }
)

async function refreshInventory(force = true) {
  if (!store.token) return
  try {
    await inventoryStore.initialize(store.token, force)
  } catch (err) {
    ElMessage.error(resolveErrorMessage(t, err, "game.warehouseLoadFailed"))
  }
}

async function refreshLogs(force = true) {
  if (!store.token || logLoading.value) return
  logLoading.value = true
  try {
    await warehouseStore.initialize(store.token, force, activeLogCategory.value)
  } catch (err) {
    ElMessage.error(resolveErrorMessage(t, err, "game.warehouseLoadFailed"))
  } finally {
    logLoading.value = false
  }
}

async function refreshCollectibles() {
  if (!store.token || collectiblesLoading.value) return
  collectiblesLoading.value = true
  try {
    await collectiblesStore.load(store.token)
  } catch (err) {
    ElMessage.error(resolveErrorMessage(t, err, "game.warehouseLoadFailed"))
  } finally {
    collectiblesLoading.value = false
  }
}

async function refreshCurrentTab() {
  if (activeTab.value === "inventory" || activeTab.value === "archived") { await refreshInventory(true); return }
  if (activeTab.value === "collectibles") { await refreshCollectibles(); return }
  await refreshLogs(true)
}

async function handleItemDetailUpdated() {
  await refreshInventory(true)
  await refreshCollectibles()
}

function hasMods(mods: Record<string, number> | undefined): boolean {
  return modEntries(mods).length > 0
}

function modEntries(mods: Record<string, number> | undefined): ModEntry[] {
  if (!mods) return []
  return Object.entries(mods)
    .filter(([, value]) => Number.isFinite(Number(value)) && Number(value) !== 0)
    .map(([attrKey, value]) => ({ attrKey, value: Number(value) }))
    .sort((a, b) => a.attrKey.localeCompare(b.attrKey))
}

function openDetail(itemId: string, instanceId: number) {
  detailItemId.value = itemId
  detailInstanceId.value = instanceId
  itemDetailVisible.value = true
}

function itemLabel(itemID: string) {
  return getLocalizedItemName(t, itemID, { fallbackText: itemID })
}

function handleInventoryItemClick(item: ItemBrowserItem | null) {
  if (batchMode.value) return
  if (!item?.instanceId) return
  openDetail(item.itemId, item.instanceId)
}

function onBatchCardClick(item: ItemBrowserItem) {
  const id = item.instanceId
  if (id == null) return
  if (batchRecycleMode.value && !isRecyclableInstance(id)) return
  batchFocusedItemId.value = item.itemId
  const next = new Set(batchSelectedIds.value)
  if (next.has(id)) {
    next.delete(id)
  } else {
    if (batchMode.value === "destructive" && next.size >= BATCH_SELECTION_LIMIT) {
      ElMessage.warning(t("game.batchSelectionLimitReached", { n: BATCH_SELECTION_LIMIT }))
      return
    }
    next.add(id)
  }
  batchSelectedIds.value = next
}

function enterBatchRecycleMode() {
  batchMode.value = "recycle"
  batchSelectedIds.value = new Set()
  batchFocusedItemId.value = ""
}

function enterDestructiveRecycleMode() {
  batchMode.value = "destructive"
  batchSelectedIds.value = new Set()
  batchFocusedItemId.value = ""
}

function enterBatchArchiveMode() {
  batchMode.value = "archive"
  batchSelectedIds.value = new Set()
  batchFocusedItemId.value = ""
}

function enterBatchUnarchiveMode() {
  batchMode.value = "unarchive"
  batchSelectedIds.value = new Set()
  batchFocusedItemId.value = ""
}

function exitBatchMode() {
  destructiveConfirmVisible.value = false
  batchMode.value = null
  batchSelectedIds.value = new Set()
  batchFocusedItemId.value = ""
}

function selectAllFocusedBatchItem() {
  const itemId = batchFocusedTemplateItemId.value
  if (!itemId) return
  const next = new Set(batchSelectedIds.value)
  let skipped = 0
  for (const inst of batchFocusedSameItemInstances.value) {
    if (!next.has(inst.instanceId) && next.size >= BATCH_SELECTION_LIMIT) {
      skipped++
      continue
    }
    next.add(inst.instanceId)
  }
  batchFocusedItemId.value = itemId
  batchSelectedIds.value = next
  if (skipped > 0) {
    ElMessage.warning(t("game.batchSelectionLimitReached", { n: BATCH_SELECTION_LIMIT }))
  }
}

function openBatchConfirm() {
  if (batchSelectedIds.value.size === 0) return
  if (batchMode.value === "destructive") {
    destructiveConfirmVisible.value = true
    return
  }
  if (batchMode.value === "archive" || batchMode.value === "unarchive") {
    void submitBatchArchive(batchMode.value === "archive")
    return
  }
  batchConfirmVisible.value = true
}

function onBatchRecycleDone() {
  exitBatchMode()
  void refreshInventory(true)
}

async function submitBatchArchive(archived: boolean) {
  if (!store.token || batchActionSubmitting.value || batchSelectedIds.value.size === 0) return
  batchActionSubmitting.value = true
  try {
    const instanceIds = [...batchSelectedIds.value].sort((a, b) => a - b)
    await apiPost<{ archived: boolean; updatedCount: number }>(
      "/api/inventory/instances/archive",
      store.token,
      { instanceIds, archived }
    )
    ElMessage.success(t(archived ? "game.batchArchiveSuccess" : "game.batchUnarchiveSuccess", { n: instanceIds.length }))
    exitBatchMode()
    await refreshInventory(true)
  } catch (err) {
    ElMessage.error(resolveErrorMessage(t, err, "game.inventoryArchiveFailed"))
  } finally {
    batchActionSubmitting.value = false
  }
}

function eventLabel(type: WarehouseLogItem["eventType"]) {
  if (type === "warehouse_operation") return t("game.warehouseLogEventWarehouseOperation")
  return t("game.warehouseLogEventSystemToWarehouse")
}

function auditLabel(item: WarehouseLogItem) {
  const category = item.category ? surfaceLabel(item.category) : eventLabel(item.eventType)
  const direction = item.direction ? surfaceLabel(item.direction) : ""
  return direction ? `${category} / ${direction}` : category
}

function auditCategoryLabel(category: InventoryAuditCategoryFilter) {
  if (category === "all") return t("game.warehouseLogCategoryAll")
  return surfaceLabel(category)
}

function logItemEntries(item: WarehouseLogItem): LogItemEntry[] {
  const entries = Object.entries(item.items || {})
    .filter(([, quantity]) => Number(quantity) > 0)
    .map(([itemId, quantity]) => ({ itemId, quantity: Number(quantity) }))
  const instanceId = positiveNumber(item.instanceId)
  if (entries.length > 0) {
    if (entries.length === 1 && instanceId) {
      return [{ ...entries[0], instanceId }]
    }
    return entries
  }
  return [{ itemId: item.templateItemId, quantity: item.quantity || 0, instanceId }]
}

function buildWarehouseLogRows(items: WarehouseLogItem[]): WarehouseLogDisplayRow[] {
  const order: Array<{ kind: "alchemy"; key: string } | { kind: "log"; item: WarehouseLogItem }> = []
  const drafts = new Map<string, AlchemyLogDraft>()
  for (const item of items) {
    const groupKey = alchemyLogGroupKey(item)
    if (!groupKey) {
      order.push({ kind: "log", item })
      continue
    }
    let draft = drafts.get(groupKey)
    if (!draft) {
      draft = createAlchemyLogDraft(groupKey, item)
      drafts.set(groupKey, draft)
      order.push({ kind: "alchemy", key: groupKey })
    }
    addAlchemyLogItem(draft, item)
  }
  return order.map((entry) => {
    if (entry.kind === "log") return { kind: "log", key: `log-${entry.item.id}`, item: entry.item }
    const draft = drafts.get(entry.key)!
    return { kind: "alchemy", key: entry.key, group: finalizeAlchemyLogDraft(draft) }
  })
}

function alchemyLogGroupKey(item: WarehouseLogItem): string {
  if (item.category !== "alchemy") return ""
  const actionId = detailString(item.details, "actionId")
  if (!isAlchemyLogAction(actionId)) return ""
  const queueId = detailString(item.details, "queueId") || item.subjectId || String(item.id)
  const runId = detailString(item.details, "runId")
  if (runId) return `alchemy-${actionId}-${queueId}-${runId}`
  const runNo = detailString(item.details, "runNo")
  if (runNo) return `alchemy-${actionId}-${queueId}-${runNo}`
  return `alchemy-${actionId}-${queueId}`
}

function createAlchemyLogDraft(key: string, item: WarehouseLogItem): AlchemyLogDraft {
  const actionId = detailString(item.details, "actionId")
  return {
    key,
    actionId,
    kind: alchemyLogKind(actionId),
    runNo: detailNumber(item.details, "runNo"),
    createdAt: item.createdAt,
    costMap: new Map(),
    rewardMap: new Map(),
    rewardInstances: [],
    sourceSurfaceSet: new Set(),
    targetSurfaceSet: new Set(),
    carrierInstanceId: detailNumber(item.details, "carrierInstanceId"),
    targetCarrierInstanceId: detailNumber(item.details, "targetCarrierInstanceId") || positiveNumber(item.instanceId),
    failed: detailBoolean(item.details, "failed")
  }
}

function addAlchemyLogItem(draft: AlchemyLogDraft, item: WarehouseLogItem) {
  if (new Date(item.createdAt).getTime() > new Date(draft.createdAt).getTime()) {
    draft.createdAt = item.createdAt
  }
  if (item.sourceSurface) draft.sourceSurfaceSet.add(item.sourceSurface)
  if (item.targetSurface) draft.targetSurfaceSet.add(item.targetSurface)
  const carrierInstanceId = detailNumber(item.details, "carrierInstanceId")
  if (carrierInstanceId) draft.carrierInstanceId = carrierInstanceId
  const targetCarrierInstanceId = detailNumber(item.details, "targetCarrierInstanceId") || positiveNumber(item.instanceId)
  if (targetCarrierInstanceId) draft.targetCarrierInstanceId = targetCarrierInstanceId
  const failed = detailBoolean(item.details, "failed")
  if (failed !== null) draft.failed = failed
  const costLog = isAlchemyCostLog(item)
  const target = costLog ? draft.costMap : draft.rewardMap
  for (const entry of logItemEntries(item)) {
    if (entry.itemId && entry.quantity > 0) {
      if (!costLog && draft.kind === "bench" && entry.instanceId) {
        draft.rewardInstances.push(entry)
      } else {
        target.set(entry.itemId, (target.get(entry.itemId) || 0) + entry.quantity)
      }
    }
  }
}

function finalizeAlchemyLogDraft(draft: AlchemyLogDraft): AlchemyLogDisplay {
  return {
    key: draft.key,
    actionId: draft.actionId,
    kind: draft.kind,
    runNo: draft.runNo,
    createdAt: draft.createdAt,
    costs: mapToLogEntries(draft.costMap),
    rewards: [...draft.rewardInstances, ...mapToLogEntries(draft.rewardMap)],
    sourceSurfaces: [...draft.sourceSurfaceSet],
    targetSurfaces: [...draft.targetSurfaceSet],
    carrierInstanceId: draft.carrierInstanceId,
    targetCarrierInstanceId: draft.targetCarrierInstanceId,
    failed: draft.failed
  }
}

function mapToLogEntries(source: Map<string, number>): LogItemEntry[] {
  return [...source.entries()]
    .filter(([, quantity]) => quantity > 0)
    .map(([itemId, quantity]) => ({ itemId, quantity }))
    .sort((a, b) => itemLabel(a.itemId).localeCompare(itemLabel(b.itemId)))
}

function isAlchemyLogAction(actionId: string): boolean {
  return actionId === "alchemy.convert" || actionId === "alchemy.bench" || actionId === "alchemy.transmute"
}

function alchemyLogKind(actionId: string): AlchemyLogKind {
  if (actionId === "alchemy.convert") return "convert"
  if (actionId === "alchemy.bench") return "bench"
  if (actionId === "alchemy.transmute") return "transmute"
  return "other"
}

function isAlchemyCostLog(item: WarehouseLogItem): boolean {
  if (item.direction === "consume" || item.direction === "fee" || item.direction === "lock") return true
  if (item.sourceSurface === "inventory") return true
  return item.targetSurface === "action_queue" || item.targetSurface.startsWith("alchemy_")
}

function alchemyLogTitle(group: AlchemyLogDisplay) {
  const base = alchemyActionLabel(group.kind)
  return group.runNo ? `${base} #${group.runNo}` : base
}

function alchemyActionLabel(kind: AlchemyLogKind) {
  if (kind === "convert") return surfaceLabel("alchemy_convert")
  if (kind === "bench") return surfaceLabel("alchemy_bench")
  if (kind === "transmute") return surfaceLabel("alchemy_transmute")
  return surfaceLabel("alchemy")
}

function alchemyCostLabel(group: AlchemyLogDisplay) {
  if (group.kind === "bench") return t("game.warehouseLogAlchemyBenchCost")
  return t("game.warehouseLogAlchemyCost")
}

function alchemyRewardLabel(group: AlchemyLogDisplay) {
  if (group.kind === "convert") return t("game.warehouseLogAlchemyConvertReward")
  if (group.kind === "bench") return t("game.warehouseLogAlchemyBenchReward")
  if (group.kind === "transmute") return t("game.warehouseLogAlchemyTransmuteReward")
  return surfaceLabel("gain")
}

function alchemyEmptyRewardLabel(group: AlchemyLogDisplay) {
  if (group.kind === "convert") return group.failed ? t("game.warehouseLogAlchemyConvertFailed") : t("game.warehouseLogAlchemyNoOutput")
  if (group.kind === "bench") return group.failed ? t("game.warehouseLogAlchemyBenchWeakResult") : t("game.warehouseLogAlchemyBenchResult")
  if (group.kind === "transmute") return t("game.warehouseLogAlchemyNoOutput")
  return "-"
}

function alchemyBenchInstanceSummary(group: AlchemyLogDisplay): string {
  if (group.kind !== "bench") return ""
  const target = group.targetCarrierInstanceId
  const source = group.carrierInstanceId
  if (target && source && target !== source) {
    return t("game.warehouseLogAlchemyBenchCarrierSplit", { source, target })
  }
  if (target) return t("game.warehouseLogAlchemyBenchTargetCarrier", { id: target })
  if (source) return t("game.warehouseLogAlchemyBenchCarrier", { id: source })
  return ""
}

function alchemySurfaceSummary(group: AlchemyLogDisplay) {
  return `${surfaceLabel("inventory")} → ${alchemyActionLabel(group.kind)} → ${surfaceLabel("inventory")}`
}

function detailString(details: Record<string, unknown> | undefined, key: string): string {
  const value = details?.[key]
  if (value == null) return ""
  return String(value).trim()
}

function detailNumber(details: Record<string, unknown> | undefined, key: string): number | null {
  const raw = details?.[key]
  const value = Number(raw)
  return Number.isFinite(value) && value > 0 ? value : null
}

function positiveNumber(raw: unknown): number | null {
  const value = Number(raw)
  return Number.isFinite(value) && value > 0 ? value : null
}

function detailBoolean(details: Record<string, unknown> | undefined, key: string): boolean | null {
  const value = details?.[key]
  if (typeof value === "boolean") return value
  return null
}

function surfaceLabel(surface: string) {
  const key = `game.surface.${surface}`
  if (!te(key)) return surface
  return t(key)
}

function formatTime(raw: string) {
  return new Date(raw).toLocaleString()
}
</script>

<style scoped>
.collectibles-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(240px, 1fr));
  gap: 12px;
  padding: 4px 0;
}
.collectible-card { height: 100%; }

.alchemy-log-flow {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
  margin-top: 4px;
  color: var(--app-muted);
}

.alchemy-log-part {
  display: inline-flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 6px;
  min-width: 0;
}

.alchemy-log-part__label {
  font-size: 11px;
  font-weight: 700;
  color: var(--el-text-color-primary);
}

.alchemy-log-flow__arrow {
  color: var(--el-text-color-placeholder);
}

</style>
<style scoped src="../../../components/game/item/batch-card-selection.css"></style>
