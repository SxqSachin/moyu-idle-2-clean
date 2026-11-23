<template>
  <GameDialog
    :model-value="modelValue"
    class="inventory-instance-selector-dialog"
    :title="title"
    :width="width"
    @close="handleClose"
  >
    <div class="space-y-3">
      <el-tabs
        v-if="archiveTabsVisible"
        v-model="archiveTab"
        class="[&_.el-tabs__content]:hidden [&_.el-tabs__header]:!m-0"
      >
        <el-tab-pane :label="t('game.inventoryTab')" name="active" />
        <el-tab-pane :label="t('game.inventoryArchivedTab')" name="archived" />
      </el-tabs>

      <template v-if="currentSelectorBrowserItems.length > 0 || allowClear">
        <ItemBrowserPanel
          :items="currentSelectorBrowserItems"
          :empty-text="emptyText"
          :search-placeholder="t('game.inventorySearchPlaceholder')"
          search-width-class="md:w-32!"
          :enable-category-toggle="true"
          :category-toggle-default="false"
          :allow-clear="allowClear"
          :clear-label="t('game.inventorySelectorClearSelection')"
          :grid-min-width="76"
          grid-gap-class="gap-[10px]"
          max-height="min(56vh, 460px)"
          :active="modelValue"
          :reset-on-activate="true"
          :equipment-category-weapon-class-sections="enableEquipmentSections"
          @item-click="handleBrowserItemClick"
        >
          <template #clear="{ onClear }">
            <EmptyCard :label="t('game.inventorySelectorClearSelection')" @click="onClear" />
          </template>
          <template #item="{ item, onClick }">
            <Card
              :item-id="item.itemId"
              :instance-id="item.instanceId"
              :quantity="item.quantity || 0"
              @click="onClick"
            />
          </template>
        </ItemBrowserPanel>
      </template>
      <div v-else-if="ineligibleBrowserItems.length === 0" class="text-sm app-muted py-2">
        {{ emptyText || t("game.equipmentNoCandidate") }}
      </div>

      <div v-if="ineligibleBrowserItems.length > 0" class="space-y-2">
        <el-divider></el-divider>
        <div class="text-sm app-muted">{{ t("game.equipmentCandidateIneligible") }}</div>
        <div class="grid gap-[10px] grid-cols-[repeat(auto-fill,minmax(76px,1fr))] opacity-50">
          <Card
            v-for="item in ineligibleBrowserItems"
            :key="`ineligible-${item.instanceId}`"
            :item-id="item.itemId"
            :instance-id="item.instanceId"
            :quantity="item.quantity || 0"
            @click="handleBrowserItemClick(item)"
          />
        </div>
      </div>

      <div v-if="templateItemRows.length > 0" class="space-y-2">
        <el-divider></el-divider>
        <div v-if="templateItemsTitleText" class="text-sm app-muted">{{ templateItemsTitleText }}</div>
        <div class="grid gap-[10px] grid-cols-[repeat(auto-fill,minmax(76px,1fr))]">
          <div v-for="itemId in templateItemRows" :key="`selector-reference-${itemId}`">
            <Card :item-id="itemId" :quantity="templateItemQuantity(itemId)" />
          </div>
        </div>
      </div>
    </div>
  </GameDialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from "vue"
import { useI18n } from "vue-i18n"
import { getItemDefinition } from "@services/item-definition"
import { useInventoryStore } from "@stores/inventory"
import Card from "@components/game/item/Card.vue"
import EmptyCard from "@components/game/item/EmptyCard.vue"
import ItemBrowserPanel, { type ItemBrowserItem } from "@components/game/item/ItemBrowserPanel.vue"
import GameDialog from "@components/game/common/GameDialog.vue"

const props = withDefaults(
  defineProps<{
    modelValue: boolean
    title: string
    instanceIds: number[]
    ineligibleInstanceIds?: number[]
    selectedInstanceId: number | null
    allowClear?: boolean
    templateItemIds?: string[]
    templateItemsTitle?: string
    emptyText?: string
    width?: string
  }>(),
  {
    allowClear: false,
    ineligibleInstanceIds: () => [],
    templateItemIds: () => [],
    templateItemsTitle: "",
    emptyText: "",
    width: "720px"
  }
)

const emit = defineEmits<{
  (e: "update:modelValue", value: boolean): void
  (e: "confirm", instanceId: number | null): void
}>()

const { t } = useI18n()
const inventoryStore = useInventoryStore()

const draftSelectedInstanceId = ref<number | null>(null)
const archiveTab = ref<"active" | "archived">("active")

const instanceIdSet = computed(() => new Set((props.instanceIds || []).filter((id) => id > 0)))
const entries = computed(() =>
  inventoryStore.instances.filter((item) => instanceIdSet.value.has(item.instanceId) && item.state === "inventory" && item.quantity > 0)
)
const activeEntries = computed(() => entries.value.filter((entry) => !entry.archivedAt))
const archivedEntries = computed(() => entries.value.filter((entry) => !!entry.archivedAt))
const archiveTabsVisible = computed(() => archivedEntries.value.length > 0)
const selectorBrowserItems = computed<ItemBrowserItem[]>(() => activeEntries.value.map(toBrowserItem))
const archivedSelectorBrowserItems = computed<ItemBrowserItem[]>(() => archivedEntries.value.map(toBrowserItem))
const currentSelectorBrowserItems = computed<ItemBrowserItem[]>(() =>
  archiveTab.value === "archived" ? archivedSelectorBrowserItems.value : selectorBrowserItems.value
)
function toBrowserItem(entry: { instanceId: number; templateItemId: string; quantity: number }): ItemBrowserItem {
  return {
    key: entry.instanceId,
    itemId: entry.templateItemId,
    itemTypes: getItemDefinition(entry.templateItemId)?.type || [],
    instanceId: entry.instanceId,
    quantity: entry.quantity
  }
}
const enableEquipmentSections = computed(() =>
  currentSelectorBrowserItems.value.some((item) => Array.isArray(item.itemTypes) && item.itemTypes.includes("equipment"))
)
const baseTemplateItemRows = computed(() => {
  const ids: string[] = []
  const seen = new Set<string>()
  for (const rawID of props.templateItemIds || []) {
    const id = String(rawID || "").trim()
    if (!id || seen.has(id)) continue
    seen.add(id)
    ids.push(id)
  }
  return ids
})
const templateItemRows = computed(() => baseTemplateItemRows.value)
const templateItemsTitleText = computed(() => String(props.templateItemsTitle || "").trim())
const templateItemQuantities = computed(() => {
  const quantities = new Map<string, number>()
  for (const entry of inventoryStore.instances) {
    if (entry.state !== "inventory" || entry.quantity <= 0) continue
    quantities.set(entry.templateItemId, (quantities.get(entry.templateItemId) || 0) + entry.quantity)
  }
  return quantities
})

const ineligibleIdSet = computed(() => new Set((props.ineligibleInstanceIds || []).filter((id) => id > 0)))
const ineligibleEntries = computed(() =>
  inventoryStore.instances.filter((item) => ineligibleIdSet.value.has(item.instanceId) && item.state === "inventory" && item.quantity > 0)
)
const currentIneligibleEntries = computed(() =>
  ineligibleEntries.value.filter((entry) => (archiveTab.value === "archived" ? !!entry.archivedAt : !entry.archivedAt))
)
const ineligibleBrowserItems = computed<ItemBrowserItem[]>(() =>
  currentIneligibleEntries.value.map((entry) => ({
    key: entry.instanceId,
    itemId: entry.templateItemId,
    itemTypes: getItemDefinition(entry.templateItemId)?.type || [],
    instanceId: entry.instanceId,
    quantity: entry.quantity
  }))
)

watch(
  () => [props.modelValue, props.selectedInstanceId, props.instanceIds, props.allowClear] as const,
  ([visible]) => {
    if (!visible) return
    const selected = props.selectedInstanceId
    if (selected && archivedSelectorBrowserItems.value.some((entry) => entry.instanceId === selected)) {
      archiveTab.value = "archived"
      draftSelectedInstanceId.value = selected
      return
    }
    archiveTab.value = "active"
    if (selected && selectorBrowserItems.value.some((entry) => entry.instanceId === selected)) {
      draftSelectedInstanceId.value = selected
      return
    }
    if (selected === null && props.allowClear) {
      draftSelectedInstanceId.value = null
      return
    }
    draftSelectedInstanceId.value = currentSelectorBrowserItems.value[0]?.instanceId ?? null
  },
  { immediate: true }
)

function handleClose() {
  emit("update:modelValue", false)
}

function handlePick(instanceId: number | null) {
  draftSelectedInstanceId.value = instanceId
  emit("confirm", instanceId)
  emit("update:modelValue", false)
}

function handleBrowserItemClick(item: ItemBrowserItem | null) {
  handlePick(item?.instanceId ?? null)
}

function templateItemQuantity(itemId: string): number {
  return templateItemQuantities.value.get(itemId) || 0
}
</script>
