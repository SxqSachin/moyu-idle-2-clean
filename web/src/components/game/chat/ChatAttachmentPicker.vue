<template>
  <div class="chat-attachment-picker" data-testid="chat-attachment-picker">
    <el-tabs v-model="activeTab" stretch class="chat-attachment-picker__tabs">
      <el-tab-pane :label="t('game.chatInsertItemCatalog')" name="items">
        <ItemBrowserPanel
          :items="catalogItems"
          :empty-text="t('game.chatInsertItemEmpty')"
          :search-placeholder="t('game.inventorySearchPlaceholder')"
          search-width-class="md:w-32!"
          :all-label="t('game.itemCategoryAll')"
          :grid-min-width="gridMinWidth"
          :grid-gap-class="gridGapClass"
          :max-height="contentHeight"
          :active="active && activeTab === 'items'"
          :reset-on-activate="true"
          :secondary-category-toggle-visible="false"
          :secondary-category-override-enabled="true"
          @item-click="selectCatalogItem"
        >
          <template #item="{ item, onClick }">
            <Card
              :item-id="item.itemId"
              :width="cardSize"
              :height="cardSize"
              :icon-size="iconSize"
              :preview-enabled="false"
              @click="onClick"
            />
          </template>
        </ItemBrowserPanel>
      </el-tab-pane>

      <el-tab-pane :label="t('game.chatInsertInventory')" name="inventory" lazy>
        <ItemBrowserPanel
          :items="inventoryItems"
          :empty-text="t('game.chatInsertInventoryEmpty')"
          :search-placeholder="t('game.inventorySearchPlaceholder')"
          search-width-class="md:w-32!"
          :all-label="t('game.itemCategoryAll')"
          :grid-min-width="gridMinWidth"
          :grid-gap-class="gridGapClass"
          :max-height="contentHeight"
          :active="active && activeTab === 'inventory'"
          :reset-on-activate="true"
          :secondary-category-toggle-visible="false"
          :secondary-category-override-enabled="true"
          @item-click="selectInventoryItem"
        >
          <template #item="{ item, onClick }">
            <Card
              :item-id="item.itemId"
              :instance-id="item.instanceId"
              :instance-preview="item.payload || null"
              :quantity="item.quantity || 0"
              :width="cardSize"
              :height="cardSize"
              :icon-size="iconSize"
              :preview-enabled="false"
              @click="onClick"
            />
          </template>
        </ItemBrowserPanel>
      </el-tab-pane>

      <el-tab-pane :label="t('game.chatInsertAction')" name="actions" lazy>
        <div class="chat-action-browser">
          <el-input
            v-model="actionQuery"
            clearable
            :placeholder="t('game.chatInsertActionSearch')"
          >
            <template #prefix><Icon icon="lucide:search" /></template>
          </el-input>
          <el-tabs v-model="actionMainType" class="chat-action-browser__categories">
            <el-tab-pane :label="t('game.itemCategoryAll')" name="all" />
            <el-tab-pane
              v-for="mainType in actionMainTypes"
              :key="mainType.id"
              :label="actionMainTypeLabel(mainType)"
              :name="mainType.id"
            />
          </el-tabs>
          <div v-if="visibleActions.length" class="chat-action-browser__grid" :style="actionGridStyle">
            <ActionCard
              v-for="action in visibleActions"
              :key="action.id"
              :action-id="action.id"
              :icon-id="action.iconId"
              :label="actionLabel(action)"
              :description="actionDescription(action)"
              :duration-s="action.durationS"
              :requirement-count="Object.keys(action.costs || {}).length"
              :output-count="Object.keys(action.itemReward || {}).length"
              :frame-width="cardSize"
              :frame-height="cardSize"
              :icon-size="iconSize"
              label-class="text-xs"
              @select="selectAction(action.id)"
            />
          </div>
          <div v-else class="py-4 text-sm app-muted">{{ t("game.chatInsertActionEmpty") }}</div>
        </div>
      </el-tab-pane>
    </el-tabs>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch, type CSSProperties } from "vue"
import { useI18n } from "vue-i18n"
import { Icon } from "@iconify/vue"
import ActionCard from "@components/game/action/ActionCard.vue"
import Card from "@components/game/item/Card.vue"
import ItemBrowserPanel, { type ItemBrowserItem } from "@components/game/item/ItemBrowserPanel.vue"
import { useDevice } from "@composables/game/useDevice"
import {
  listActionDefinitions,
  listActionMainTypeDefinitions
} from "@services/action-definition"
import {
  createItemInstanceChatAttachment,
  type ChatAttachment
} from "@services/chat-attachments"
import { getFishSpeciesDisplay } from "@services/fish-instance-display"
import { itemCustomName } from "@services/item-customization"
import { getItemDefinition, listItemDefinitions } from "@services/item-definition"
import { useInventoryStore } from "@stores/inventory"
import type { ActionDefinition, ActionMainTypeDefinition } from "@game-data/game-data"

const props = withDefaults(defineProps<{
  active?: boolean
}>(), {
  active: true
})

const emit = defineEmits<{
  (e: "select", attachment: ChatAttachment): void
}>()

const { t } = useI18n()
const { isMobile } = useDevice()
const inventoryStore = useInventoryStore()
const activeTab = ref<"items" | "inventory" | "actions">("items")
const actionQuery = ref("")
const actionMainType = ref("all")

const gridMinWidth = computed(() => isMobile.value ? 68 : 76)
const gridGapClass = computed(() => isMobile.value ? "gap-2" : "gap-[10px]")
const cardSize = computed(() => isMobile.value ? 52 : 60)
const iconSize = computed(() => isMobile.value ? 28 : 32)
const contentHeight = computed(() => isMobile.value ? "calc(70dvh - 190px)" : "278px")

const catalogItems = computed<ItemBrowserItem[]>(() => listItemDefinitions().map((item) => ({
  key: item.id,
  itemId: item.id,
  itemTypes: Array.isArray(item.type) ? item.type : []
})))

const inventoryItems = computed<ItemBrowserItem[]>(() => inventoryStore.instances
  .filter((item) => item.state === "inventory" && item.quantity > 0 && !item.archivedAt)
  .map((item) => ({
    key: item.instanceId,
    itemId: item.templateItemId,
    itemTypes: item.fishData ? ["fish"] : getItemDefinition(item.templateItemId)?.type || [],
    instanceId: item.instanceId,
    quantity: item.quantity,
    searchText: inventoryItemSearchText(item),
    payload: item
  })))

const actions = computed(() => listActionDefinitions())
const actionMainTypes = computed(() => listActionMainTypeDefinitions())
const visibleActions = computed(() => {
  const keyword = actionQuery.value.trim().toLowerCase()
  return actions.value.filter((action) => {
    const mainType = String(action.mainType || action.menu || "").trim()
    if (actionMainType.value !== "all" && mainType !== actionMainType.value) return false
    if (!keyword) return true
    return action.id.toLowerCase().includes(keyword) || actionLabel(action).toLowerCase().includes(keyword)
  })
})
const actionGridStyle = computed<CSSProperties>(() => ({
  gridTemplateColumns: `repeat(auto-fill, minmax(${gridMinWidth.value}px, 1fr))`,
  maxHeight: contentHeight.value
}))

watch(
  () => props.active,
  (active, previous) => {
    if (!active || previous) return
    actionQuery.value = ""
    actionMainType.value = "all"
  }
)

watch(actionMainTypes, (mainTypes) => {
  if (actionMainType.value === "all") return
  if (!mainTypes.some((entry) => entry.id === actionMainType.value)) actionMainType.value = "all"
})

function selectCatalogItem(item: ItemBrowserItem | null) {
  if (!item?.itemId) return
  emit("select", { type: "item", itemId: item.itemId })
}

function selectInventoryItem(item: ItemBrowserItem | null) {
  const instance = item?.payload
  if (!instance || typeof instance !== "object") return
  emit("select", createItemInstanceChatAttachment(instance))
}

function selectAction(actionId: string) {
  const normalized = String(actionId || "").trim()
  if (!normalized) return
  emit("select", { type: "action", actionId: normalized })
}

function inventoryItemSearchText(item: { customData?: { name?: string }; fishData?: { speciesId?: string } }): string {
  const customName = itemCustomName(item.customData)
  const species = getFishSpeciesDisplay(item.fishData?.speciesId || "")
  const speciesNames = species?.names ? Object.values(species.names).join(" ") : ""
  return [customName, item.fishData?.speciesId, speciesNames].filter(Boolean).join(" ")
}

function actionLabel(action: ActionDefinition): string {
  if (action.nameKey) {
    const localized = t(action.nameKey)
    if (localized !== action.nameKey && localized.trim()) return localized
  }
  const key = `action.${action.id}.name`
  const localized = t(key)
  return localized !== key && localized.trim() ? localized : action.id
}

function actionDescription(action: ActionDefinition): string {
  if (!action.descKey) return ""
  const localized = t(action.descKey)
  return localized !== action.descKey ? localized : ""
}

function actionMainTypeLabel(mainType: ActionMainTypeDefinition): string {
  const localized = t(mainType.nameKey)
  if (localized !== mainType.nameKey && localized.trim()) return localized
  const fallbackKey = `action_main_type.${mainType.id}.name`
  const fallback = t(fallbackKey)
  return fallback !== fallbackKey && fallback.trim() ? fallback : mainType.id
}
</script>

<style scoped>
.chat-attachment-picker {
  min-width: 0;
}

.chat-attachment-picker__tabs :deep(.el-tabs__header) {
  margin: 0 0 12px;
}

.chat-attachment-picker__tabs :deep(.el-tabs__item) {
  min-height: 40px;
  padding: 0 10px;
}

.chat-action-browser {
  min-width: 0;
  display: grid;
  gap: 10px;
}

.chat-action-browser__categories {
  min-width: 0;
}

.chat-action-browser__categories :deep(.el-tabs__header) {
  margin: 0;
}

.chat-action-browser__categories :deep(.el-tabs__content) {
  display: none;
}

.chat-action-browser__grid {
  display: grid;
  gap: 10px;
  overflow-y: auto;
  overscroll-behavior: contain;
  padding: 2px 2px 8px;
}

@media (max-width: 767px) {
  .chat-attachment-picker__tabs :deep(.el-tabs__item) {
    min-height: 44px;
    padding: 0 6px;
    font-size: 13px;
  }

  .chat-action-browser__categories :deep(.el-tabs__nav-wrap) {
    overflow-x: auto;
  }
}
</style>
