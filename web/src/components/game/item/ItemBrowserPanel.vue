<template>
  <div class="space-y-3" data-tutorial="item-browser" :data-tutorial-scope="tutorialScope || undefined">
    <div v-if="showToolbar" class="flex flex-col gap-2 md:flex-row md:items-center" data-tutorial="item-browser-toolbar">
      <el-input
        v-if="showSearch"
        v-model="searchQuery"
        clearable
        :placeholder="searchPlaceholder"
        :class="['w-full md:shrink-0', searchWidthClass]"
        data-tutorial="item-browser-search"
      />
      <div class="flex items-center gap-2 overflow-x-auto md:min-w-0 md:flex-1">
        <el-tabs
          v-model="category"
          :class="['min-w-64 flex-1 [&_.el-tabs__content]:hidden [&_.el-tabs__header]:!m-0', tabClass]"
          data-tutorial="item-browser-categories"
        >
          <el-tab-pane :label="allLabel || t('game.itemCategoryAll')" name="all" />
          <el-tab-pane
            v-for="itemType in categoryOptions"
            :key="`browser-category-${itemType}`"
            :label="localizeItemType(itemType)"
            :name="itemType"
          />
        </el-tabs>
        <el-dropdown ref="dropdownRef" trigger="click" data-tutorial="item-browser-tools" @command="handleSortCommand">
          <el-button circle :title="t('game.itemSortDefault')" data-tutorial="item-browser-tools-trigger">
            <slot name="sort-icon">
              <el-icon><Icon icon="ep:sort" /></el-icon>
            </slot>
          </el-button>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item command="none">{{ t("game.itemSortDefault") }}</el-dropdown-item>
              <el-dropdown-item command="rarity_desc">{{ t("game.itemSortRarityDesc") }}</el-dropdown-item>
              <el-dropdown-item command="rarity_asc">{{ t("game.itemSortRarityAsc") }}</el-dropdown-item>
              <template v-if="shouldRenderSecondaryCategoryToggle">
                <el-divider class="my-2!" />
                <div class="px-3 py-2" @click.stop>
                  <div class="flex items-center justify-between gap-3">
                    <div class="text-xs app-muted whitespace-nowrap">{{ t("game.itemBrowserSecondaryCategorySwitch") }}</div>
                    <el-switch
                      v-model="secondaryCategoryEnabled"
                      inline-prompt
                      :active-text="t('game.switchOn')"
                      :inactive-text="t('game.switchOff')"
                    />
                  </div>
                </div>
              </template>
              <slot name="toolbar-extra" :close="() => dropdownRef?.handleClose()" />
            </el-dropdown-menu>
          </template>
        </el-dropdown>
      </div>
    </div>

    <div v-if="!hasVisibleItems" class="text-sm app-muted py-4">
      {{ emptyText }}
    </div>
    <template v-else>
      <div v-if="allowClear" class="w-fit ml-3">
        <slot name="clear" :onClear="handleClear">
          <el-button @click="handleClear">{{ clearLabel || t("game.inventorySelectorClearSelection") }}</el-button>
        </slot>
      </div>

      <div
        v-if="shouldUseVirtualGrid"
        ref="virtualScrollRef"
        class="item-browser-virtual-grid"
        :style="virtualScrollStyle"
        data-tutorial="item-browser-grid"
      >
        <div class="relative w-full" :style="{ height: `${rowVirtualizer.getTotalSize()}px` }">
          <div
            v-for="entry in virtualRowsInView"
            :key="String(entry.virtual.key)"
            class="item-browser-virtual-grid__row"
            :style="{ transform: `translateY(${entry.virtual.start}px)` }"
          >
            <div
              v-if="entry.row.kind === 'header'"
              class="item-browser-virtual-grid__header"
              :class="sectionTitleClass(entry.row.depth)"
              :style="sectionDepthStyle(entry.row.depth)"
            >
              {{ entry.row.label }}
            </div>
            <div
              v-else
              :class="['grid', gridGapClass, 'item-browser-virtual-grid__items']"
              :style="[virtualGridRowStyle, sectionDepthStyle(entry.row.depth)]"
            >
              <div v-for="item in entry.row.items" :key="item.key">
                <slot name="item" :item="item" :onClick="() => emitItemClick(item)">
                  <el-button text @click="emitItemClick(item)">{{ itemLabel(item.itemId) }}</el-button>
                </slot>
              </div>
            </div>
          </div>
        </div>
      </div>

      <div v-else-if="shouldRenderSections" class="space-y-4" data-tutorial="item-browser-grid">
        <section v-for="section in groupedSections" :key="`browser-section-${section.id}`" class="item-browser-section">
          <div :class="sectionTitleClass(0)">{{ section.label }}</div>
          <div v-if="section.items.length > 0" class="py-2" :class="['grid', gridGapClass]" :style="gridStyle">
            <div v-for="item in section.items" :key="item.key">
              <slot name="item" :item="item" :onClick="() => emitItemClick(item)">
                <el-button text @click="emitItemClick(item)">{{ itemLabel(item.itemId) }}</el-button>
              </slot>
            </div>
          </div>
          <div v-if="section.subSections?.length" class="item-browser-section-children">
            <section v-for="subSection in section.subSections" :key="`browser-section-${section.id}-${subSection.id}`" class="item-browser-section">
              <div :class="sectionTitleClass(1)">{{ subSection.label }}</div>
              <div v-if="subSection.items.length > 0" class="py-2" :class="['grid', gridGapClass]" :style="gridStyle">
                <div v-for="item in subSection.items" :key="item.key">
                  <slot name="item" :item="item" :onClick="() => emitItemClick(item)">
                    <el-button text @click="emitItemClick(item)">{{ itemLabel(item.itemId) }}</el-button>
                  </slot>
                </div>
              </div>
              <div
                v-if="subSection.subSections?.length"
                class="item-browser-section-children item-browser-section-children--deep"
              >
                <section
                  v-for="thirdSection in subSection.subSections"
                  :key="`browser-section-${section.id}-${subSection.id}-${thirdSection.id}`"
                  class="item-browser-section"
                >
                  <div :class="sectionTitleClass(2)">{{ thirdSection.label }}</div>
                  <div class="py-2" :class="['grid', gridGapClass]" :style="gridStyle">
                    <div v-for="item in thirdSection.items" :key="item.key">
                      <slot name="item" :item="item" :onClick="() => emitItemClick(item)">
                        <el-button text @click="emitItemClick(item)">{{ itemLabel(item.itemId) }}</el-button>
                      </slot>
                    </div>
                  </div>
                </section>
              </div>
            </section>
          </div>
        </section>
      </div>

      <div v-else :class="['grid', gridGapClass]" :style="gridStyle" data-tutorial="item-browser-grid">
        <div v-for="item in visibleItems" :key="item.key">
          <slot name="item" :item="item" :onClick="() => emitItemClick(item)">
            <el-button text @click="emitItemClick(item)">{{ itemLabel(item.itemId) }}</el-button>
          </slot>
        </div>
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch, type CSSProperties } from "vue"
import { useVirtualizer, type VirtualItem } from "@tanstack/vue-virtual"
import type { DropdownInstance } from "element-plus"
import { Icon } from "@iconify/vue"
import { useI18n } from "vue-i18n"
import { compareItemIDsByRarity, getItemDefinition, orderItemTypes } from "@services/item-definition"
import { buildEquipmentSlotSecondarySections } from "@services/equipment-grouping"
import { buildFishingTackleSecondarySections } from "@services/fishing-tackle-grouping"
import {
  getItemBrowserSecondaryCategoryEnabled,
  setItemBrowserSecondaryCategoryEnabled
} from "@services/item-browser-secondary-category"
import { getLocalizedItemName } from "@services/item-label"
import { getSkillDefinition, getSkillTypeDefinition, listSkillTypeDefinitions } from "@services/skill-definition"

export type ItemBrowserItem = {
  key: string | number
  itemId: string
  itemTypes: string[]
  instanceId?: number
  quantity?: number
  searchText?: string
  payload?: any
}

type SortMode = "none" | "rarity_desc" | "rarity_asc"
type BrowserSection = { id: string; label: string; items: ItemBrowserItem[]; subSections?: BrowserSection[] }
type VirtualizedMode = boolean | "auto"
type VirtualGridRow =
  | { kind: "header"; key: string; label: string; depth: number }
  | { kind: "items"; key: string; items: ItemBrowserItem[]; depth: number }

const SKILLBOOK_TYPE = "skillbook"
const FISHING_GEAR_TYPE = "fishing_gear"
const DEFAULT_VIRTUAL_GRID_HEIGHT = "min(68vh, 760px)"
const DEFAULT_VIRTUALIZE_THRESHOLD = 120

const props = withDefaults(
  defineProps<{
    items: ItemBrowserItem[]
    emptyText: string
    searchPlaceholder?: string
    showToolbar?: boolean
    showSearch?: boolean
    searchWidthClass?: string
    tabClass?: string
    allLabel?: string
    allowClear?: boolean
    clearLabel?: string
    enableCategoryToggle?: boolean
    categoryToggleDefault?: boolean
    categoryToggleLabel?: string
    groupAllByType?: boolean
    gridMinWidth?: number
    gridGapClass?: string
    maxHeight?: string
    active?: boolean
    resetOnActivate?: boolean
    equipmentCategoryWeaponClassSections?: boolean
    fishingGearSecondarySections?: boolean
    secondaryCategoryToggleVisible?: boolean
    secondaryCategoryOverrideEnabled?: boolean
    virtualized?: VirtualizedMode
    virtualizeThreshold?: number
    virtualGridHeight?: string
    virtualItemHeight?: number
    virtualHeaderHeight?: number
    virtualOverscan?: number
    tutorialScope?: string
  }>(),
  {
    searchPlaceholder: "",
    showToolbar: true,
    showSearch: true,
    searchWidthClass: "md:w-56",
    tabClass: "",
    allLabel: "",
    allowClear: false,
    clearLabel: "",
    enableCategoryToggle: false,
    categoryToggleDefault: false,
    categoryToggleLabel: "",
    groupAllByType: true,
    gridMinWidth: 64,
    gridGapClass: "gap-3",
    maxHeight: "",
    active: true,
    resetOnActivate: false,
    equipmentCategoryWeaponClassSections: true,
    fishingGearSecondarySections: true,
    secondaryCategoryToggleVisible: true,
    secondaryCategoryOverrideEnabled: undefined,
    virtualized: "auto",
    virtualizeThreshold: DEFAULT_VIRTUALIZE_THRESHOLD,
    virtualGridHeight: DEFAULT_VIRTUAL_GRID_HEIGHT,
    virtualItemHeight: 94,
    virtualHeaderHeight: 28,
    virtualOverscan: 4,
    tutorialScope: ""
  }
)

const emit = defineEmits<{
  (e: "item-click", item: ItemBrowserItem | null): void
}>()

const { t } = useI18n()

const dropdownRef = ref<DropdownInstance>()
const searchQuery = ref("")
const category = ref("all")
const sortMode = ref<SortMode>("rarity_asc")
const categoryFilterEnabled = ref(!!props.categoryToggleDefault)
const virtualScrollRef = ref<HTMLElement | null>(null)
const virtualGridWidth = ref(0)
let virtualGridResizeObserver: ResizeObserver | null = null

const categoryOptions = computed(() => {
  const types = new Set<string>()
  for (const item of props.items || []) {
    for (const itemType of item.itemTypes || []) {
      if (itemType && itemType !== "unknown") types.add(itemType)
    }
  }
  return orderItemTypes(types)
})

const effectiveCategory = computed(() => {
  if (!props.enableCategoryToggle) return category.value
  if (!categoryFilterEnabled.value) return "all"
  return category.value
})

const secondaryCategoryKey = computed(() => (effectiveCategory.value === "all" ? "equipment" : effectiveCategory.value))
const secondaryCategoryEnabled = computed({
  get: () =>
    typeof props.secondaryCategoryOverrideEnabled === "boolean"
      ? props.secondaryCategoryOverrideEnabled
      : getItemBrowserSecondaryCategoryEnabled(secondaryCategoryKey.value),
  set: (enabled: boolean) => {
    if (typeof props.secondaryCategoryOverrideEnabled === "boolean") return
    setItemBrowserSecondaryCategoryEnabled(secondaryCategoryKey.value, enabled)
  }
})
const shouldRenderSecondaryCategoryToggle = computed(
  () =>
    props.secondaryCategoryToggleVisible &&
    props.groupAllByType &&
    ((props.equipmentCategoryWeaponClassSections &&
      (effectiveCategory.value === "all" || effectiveCategory.value === "equipment")) ||
      (props.fishingGearSecondarySections && effectiveCategory.value === FISHING_GEAR_TYPE))
)
const shouldUseEquipmentSecondarySections = computed(
  () =>
    props.equipmentCategoryWeaponClassSections &&
    (typeof props.secondaryCategoryOverrideEnabled === "boolean"
      ? props.secondaryCategoryOverrideEnabled
      : getItemBrowserSecondaryCategoryEnabled("equipment"))
)
const shouldUseFishingGearSecondarySections = computed(
  () =>
    props.fishingGearSecondarySections &&
    (typeof props.secondaryCategoryOverrideEnabled === "boolean"
      ? props.secondaryCategoryOverrideEnabled
      : getItemBrowserSecondaryCategoryEnabled(FISHING_GEAR_TYPE))
)

const visibleItems = computed(() => {
  const keyword = searchQuery.value.trim().toLowerCase()
  const filtered = (props.items || []).filter((item) => {
    if (effectiveCategory.value !== "all" && !(item.itemTypes || []).includes(effectiveCategory.value)) return false
    if (!keyword) return true
    const label = itemLabel(item.itemId).toLowerCase()
    const extra = String(item.searchText || "").toLowerCase()
    return label.includes(keyword) || item.itemId.toLowerCase().includes(keyword) || extra.includes(keyword)
  })
  if (sortMode.value === "none") return filtered
  const direction = sortMode.value === "rarity_asc" ? "asc" : "desc"
  return filtered.slice().sort((a, b) => {
    const rarityCompare = compareItemIDsByRarity(a.itemId, b.itemId, direction)
    if (rarityCompare !== 0) return rarityCompare
    return itemLabel(a.itemId).localeCompare(itemLabel(b.itemId))
  })
})

const groupedByTypeSections = computed<BrowserSection[]>(() => {
  if (!props.groupAllByType) return []
  if (shouldUseEquipmentSecondarySections.value && effectiveCategory.value === "equipment") {
    return buildEquipmentSlotSections(visibleItems.value)
  }
  if (shouldUseFishingGearSecondarySections.value && effectiveCategory.value === FISHING_GEAR_TYPE) {
    return buildFishingGearSections(visibleItems.value)
  }
  if (effectiveCategory.value !== "all") return []
  const groups = new Map<string, ItemBrowserItem[]>()
  const unknownItems: ItemBrowserItem[] = []
  for (const item of visibleItems.value) {
    const primaryType = (item.itemTypes || []).find((entry) => String(entry || "").trim() && entry !== "unknown") || "unknown"
    if (primaryType === "unknown") {
      unknownItems.push(item)
      continue
    }
    if (!groups.has(primaryType)) groups.set(primaryType, [])
    groups.get(primaryType)?.push(item)
  }
  const sections = orderItemTypes(groups.keys()).map((itemType) => ({
    id: itemType,
    label: localizeItemType(itemType),
    items: groups.get(itemType) || []
  }))
  if (unknownItems.length > 0) {
    sections.push({ id: "unknown", label: localizeItemType("unknown"), items: unknownItems })
  }
  return sections
    .map<BrowserSection>((section) => {
      if (section.id === "equipment" && shouldUseEquipmentSecondarySections.value) {
        const subSections = buildEquipmentSlotSections(section.items)
        return {
          ...section,
          items: subSections.length > 0 ? [] : section.items,
          subSections: subSections.length > 0 ? subSections : undefined
        }
      }
      if (section.id === FISHING_GEAR_TYPE && shouldUseFishingGearSecondarySections.value) {
        const subSections = buildFishingGearSections(section.items)
        return {
          ...section,
          items: subSections.length > 0 ? [] : section.items,
          subSections: subSections.length > 0 ? subSections : undefined
        }
      }
      if (section.id === SKILLBOOK_TYPE) {
        const subSections = buildSkillbookSections(section.items)
        if (subSections.length === 0) return section
        return {
          ...section,
          items: [],
          subSections
        }
      }
      return section
    })
    .filter((section) => section.items.length > 0 || (section.subSections?.length || 0) > 0)
})

const shouldGroupBySkillbookCategory = computed(() => effectiveCategory.value === SKILLBOOK_TYPE)

const groupedBySkillbookSections = computed<BrowserSection[]>(() => {
  if (!shouldGroupBySkillbookCategory.value) return []
  return buildSkillbookSections(visibleItems.value)
})

const groupedSections = computed<BrowserSection[]>(() => {
  if (shouldGroupBySkillbookCategory.value) return groupedBySkillbookSections.value
  return groupedByTypeSections.value
})
const shouldRenderSections = computed(() => groupedSections.value.length > 0)
const hasVisibleItems = computed(() => visibleItems.value.length > 0 || props.allowClear)

const gridStyle = computed<CSSProperties>(() => ({
  gridTemplateColumns: `repeat(auto-fill, minmax(${Math.max(48, props.gridMinWidth)}px, 1fr))`,
  maxHeight: props.maxHeight || undefined,
  overflowY: props.maxHeight ? "auto" : undefined
}))
const virtualGapPx = computed(() => resolveGapPx(props.gridGapClass))
const virtualColumnCount = computed(() => {
  const width = Math.max(0, virtualGridWidth.value)
  const minWidth = Math.max(48, props.gridMinWidth)
  const gap = virtualGapPx.value
  if (width <= 0) return 1
  return Math.max(1, Math.floor((width + gap) / (minWidth + gap)))
})
const virtualGridRowStyle = computed<CSSProperties>(() => ({
  gridTemplateColumns: `repeat(${virtualColumnCount.value}, minmax(${Math.max(48, props.gridMinWidth)}px, 1fr))`
}))
const virtualScrollStyle = computed<CSSProperties>(() => ({
  height: props.maxHeight || props.virtualGridHeight || DEFAULT_VIRTUAL_GRID_HEIGHT,
  overflowY: "auto"
}))
const shouldUseVirtualGrid = computed(() => {
  if (visibleItems.value.length <= 0) return false
  if (props.virtualized === true) return true
  if (props.virtualized === false) return false
  return visibleItems.value.length >= Math.max(1, props.virtualizeThreshold)
})
const virtualRows = computed<VirtualGridRow[]>(() => {
  if (!shouldUseVirtualGrid.value) return []
  const rows: VirtualGridRow[] = []
  if (shouldRenderSections.value) {
    groupedSections.value.forEach((section, index) => {
      appendVirtualSectionRows(rows, section, 0, `root-${index}-${section.id}`)
    })
  } else {
    appendVirtualItemRows(rows, "all", visibleItems.value, 0)
  }
  return rows
})
const rowVirtualizer = useVirtualizer<HTMLElement, HTMLDivElement>(
  computed(() => ({
    count: virtualRows.value.length,
    getScrollElement: () => virtualScrollRef.value,
    estimateSize: (index: number) => estimateVirtualRowSize(virtualRows.value[index]),
    getItemKey: (index: number) => virtualRows.value[index]?.key || index,
    overscan: Math.max(1, props.virtualOverscan)
  }))
)
const virtualRowsInView = computed(() =>
  rowVirtualizer.value.getVirtualItems().map((virtual: VirtualItem) => ({
    virtual,
    row: virtualRows.value[virtual.index]
  })).filter((entry): entry is { virtual: VirtualItem; row: VirtualGridRow } => !!entry.row)
)

watch(
  () => categoryOptions.value.join(","),
  () => {
    if (category.value === "all") return
    if (categoryOptions.value.includes(category.value)) return
    category.value = "all"
  }
)

watch(
  () => props.active,
  (active, previous) => {
    if (!props.resetOnActivate || !active || previous) return
    searchQuery.value = ""
    category.value = "all"
    sortMode.value = "none"
    categoryFilterEnabled.value = !!props.categoryToggleDefault
  },
  { immediate: true }
)

function handleTutorialOpenTools(event: Event) {
  const detail = (event as CustomEvent<{ scope?: string }>).detail
  if (detail?.scope && detail.scope !== props.tutorialScope) return
  void nextTick(() => {
    dropdownRef.value?.handleOpen?.()
  })
}

function handleTutorialCloseTools(event: Event) {
  const detail = (event as CustomEvent<{ scope?: string }>).detail
  if (detail?.scope && detail.scope !== props.tutorialScope) return
  dropdownRef.value?.handleClose?.()
}

onMounted(() => {
  window.addEventListener("item-browser:tutorial-open-tools", handleTutorialOpenTools)
  window.addEventListener("item-browser:tutorial-close-tools", handleTutorialCloseTools)
  attachVirtualGridResizeObserver()
})

onBeforeUnmount(() => {
  window.removeEventListener("item-browser:tutorial-open-tools", handleTutorialOpenTools)
  window.removeEventListener("item-browser:tutorial-close-tools", handleTutorialCloseTools)
  virtualGridResizeObserver?.disconnect()
  virtualGridResizeObserver = null
})

watch(
  () => virtualScrollRef.value,
  () => attachVirtualGridResizeObserver(),
  { flush: "post" }
)

watch(
  () => [searchQuery.value, effectiveCategory.value, sortMode.value, secondaryCategoryEnabled.value],
  () => {
    if (!shouldUseVirtualGrid.value) return
    void nextTick(() => {
      rowVirtualizer.value.scrollToOffset(0)
      rowVirtualizer.value.measure()
    })
  }
)

watch(
  () => [virtualColumnCount.value, visibleItems.value.length],
  () => {
    if (!shouldUseVirtualGrid.value) return
    void nextTick(() => {
      rowVirtualizer.value.measure()
    })
  }
)

function emitItemClick(item: ItemBrowserItem) {
  emit("item-click", item)
}

function handleClear() {
  emit("item-click", null)
}

function attachVirtualGridResizeObserver() {
  virtualGridResizeObserver?.disconnect()
  virtualGridResizeObserver = null
  const el = virtualScrollRef.value
  if (!el || typeof ResizeObserver === "undefined") {
    virtualGridWidth.value = el?.clientWidth || 0
    return
  }
  virtualGridWidth.value = el.clientWidth
  virtualGridResizeObserver = new ResizeObserver((entries) => {
    const entry = entries[0]
    virtualGridWidth.value = Math.floor(entry?.contentRect.width || el.clientWidth || 0)
  })
  virtualGridResizeObserver.observe(el)
}

function appendVirtualSectionRows(rows: VirtualGridRow[], section: BrowserSection, depth: number, pathKey: string) {
  rows.push({
    kind: "header",
    key: `header-${pathKey}`,
    label: section.label,
    depth
  })
  appendVirtualItemRows(rows, `section-${pathKey}`, section.items, depth)
  const subSections = section.subSections || []
  subSections.forEach((subSection, index) => {
    appendVirtualSectionRows(rows, subSection, depth + 1, `${pathKey}-${index}-${subSection.id}`)
  })
}

function appendVirtualItemRows(rows: VirtualGridRow[], keyPrefix: string, items: ItemBrowserItem[], depth: number) {
  const columns = virtualColumnCount.value
  if (columns <= 0) return
  for (let index = 0; index < items.length; index += columns) {
    rows.push({
      kind: "items",
      key: `${keyPrefix}-items-${index}`,
      items: items.slice(index, index + columns),
      depth
    })
  }
}

function estimateVirtualRowSize(row: VirtualGridRow | undefined) {
  if (row?.kind === "header") return Math.max(20, props.virtualHeaderHeight)
  return Math.max(48, props.virtualItemHeight) + virtualGapPx.value
}

function resolveGapPx(className: string) {
  const match = String(className || "").match(/(?:^|\s)gap-(\d+)(?:\s|$)/)
  if (!match) return 12
  return Number(match[1]) * 4
}

function sectionTitleClass(depth: number) {
  return [
    "item-browser-section-title",
    `item-browser-section-title--level-${Math.min(2, Math.max(0, depth))}`
  ]
}

function sectionDepthStyle(_depth: number): CSSProperties {
  return {}
}

function itemLabel(itemID: string) {
  return getLocalizedItemName(t, itemID, { fallbackText: itemID })
}

function localizeItemType(itemType: string) {
  const key = `item_type.${itemType}.name`
  const localized = t(key)
  return localized === key ? itemType : localized
}

function buildEquipmentSlotSections(items: ItemBrowserItem[]): BrowserSection[] {
  if (!props.equipmentCategoryWeaponClassSections || items.length === 0) return []
  return buildEquipmentSlotSecondarySections({
    items,
    getItemId: (item) => item.itemId,
    t
  })
    .map<BrowserSection>((section) => ({
      id: section.id,
      label: section.label,
      items: section.items,
      subSections: section.subSections?.map((subSection) => ({
        id: subSection.id,
        label: subSection.label,
        items: subSection.items
      }))
    }))
    .filter((section) => section.items.length > 0 || (section.subSections?.length || 0) > 0)
}

function buildFishingGearSections(items: ItemBrowserItem[]): BrowserSection[] {
  if (!props.fishingGearSecondarySections || items.length === 0) return []
  return buildFishingTackleSecondarySections({
    items,
    getItemId: (item) => item.itemId,
    t
  })
}

function resolveSkillbookSectionId(itemId: string): string {
  const skillId = resolveSkillbookSkillId(itemId)
  if (!skillId) return "other"
  const category = String(getSkillDefinition(skillId)?.category || "").trim()
  return category || "other"
}

function buildSkillbookSections(items: ItemBrowserItem[]): BrowserSection[] {
  if (!items.length) return []
  const groups = new Map<string, ItemBrowserItem[]>()
  const otherItems: ItemBrowserItem[] = []
  for (const item of items) {
    const sectionId = resolveSkillbookSectionId(item.itemId)
    if (sectionId === "other") {
      otherItems.push(item)
      continue
    }
    if (!groups.has(sectionId)) groups.set(sectionId, [])
    groups.get(sectionId)?.push(item)
  }
  const orderedSections: BrowserSection[] = []
  const sectionOrder = listSkillTypeDefinitions()
    .map((row) => String(row.id || "").trim())
    .filter((id) => id !== "")
  const orderedIDs = sectionOrder.filter((id) => groups.has(id))
  for (const id of groups.keys()) {
    if (!orderedIDs.includes(id)) orderedIDs.push(id)
  }
  for (const sectionId of orderedIDs) {
    const sectionItems = groups.get(sectionId)
    if (!sectionItems?.length) continue
    orderedSections.push({
      id: sectionId,
      label: localizeSkillbookSection(sectionId),
      items: sectionItems
    })
  }
  if (otherItems.length > 0) {
    orderedSections.push({ id: "other", label: t("game.itemSectionOther"), items: otherItems })
  }
  return orderedSections
}

function resolveSkillbookSkillId(itemId: string): string {
  const item = getItemDefinition(itemId)
  const fromItem = String(item?.useGrantSkillId || "").trim()
  if (fromItem) return fromItem
  if (itemId.startsWith("skillbook_")) return itemId.slice("skillbook_".length)
  return ""
}

function localizeSkillbookSection(sectionId: string): string {
  const nameKey = getSkillTypeDefinition(sectionId)?.nameKey
  if (!nameKey) return sectionId
  const localized = t(nameKey)
  return localized === nameKey ? sectionId : localized
}

function handleSortCommand(command: string | number | object) {
  const next = String(command || "")
  if (next === "none" || next === "rarity_desc" || next === "rarity_asc") {
    sortMode.value = next
  }
}
</script>

<style scoped>
.item-browser-section {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.item-browser-section-children {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.item-browser-section-children--deep {
  gap: 12px;
}

.item-browser-section-title {
  letter-spacing: 0;
  line-height: 1.25;
}

.item-browser-section-title--level-0 {
  color: var(--app-text, var(--el-text-color-primary));
  font-size: 15px;
  font-weight: 700;
}

.item-browser-section-title--level-1 {
  color: var(--el-text-color-regular);
  font-size: 13px;
  font-weight: 650;
}

.item-browser-section-title--level-2 {
  color: var(--app-muted, var(--el-text-color-secondary));
  font-size: 12px;
  font-weight: 600;
}

.item-browser-virtual-grid {
  contain: layout paint;
  min-height: 160px;
  overscroll-behavior: contain;
  padding-right: 2px;
}

.item-browser-virtual-grid__row {
  box-sizing: border-box;
  left: 0;
  position: absolute;
  top: 0;
  width: 100%;
}

.item-browser-virtual-grid__header {
  align-items: center;
  box-sizing: border-box;
  display: flex;
  min-height: 28px;
}

.item-browser-virtual-grid__items {
  box-sizing: border-box;
}
</style>
