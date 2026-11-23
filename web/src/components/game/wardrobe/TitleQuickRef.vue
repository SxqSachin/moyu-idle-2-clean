<template>
  <span class="title-quick-ref">
    <el-popover
      v-model:visible="popoverActive"
      placement="top"
      :width="240"
      :trigger="supportsHover ? 'hover' : 'manual'"
      :persistent="false"
      :show-after="supportsHover ? 160 : 0"
      @show="onPopoverShow"
      @hide="onPopoverHide"
    >
      <template #reference>
        <span
          class="title-quick-ref-tag"
          :style="tagStyle"
          @click.stop="onTagClick"
          @mouseenter="primePopover"
          @focusin="primePopover"
        >
          <span v-if="prefixLabel">{{ prefixLabel }}</span>
          <span class="title-quick-ref-tag__name">{{ titleLabel }}</span>
        </span>
      </template>
      <TitleQuickRefContent v-if="popoverLoaded" :category="normalizedCategory" :item-id="normalizedItemId" />
    </el-popover>

    <GameDialog v-model="mobilePopoverVisible" class="dialog-mobile-windowed title-quick-ref-dialog" width="min(92vw, 280px)">
      <TitleQuickRefContent :category="normalizedCategory" :item-id="normalizedItemId" />
    </GameDialog>
  </span>
</template>

<script setup lang="ts">
import { computed, ref } from "vue"
import { useI18n } from "vue-i18n"
import { useDevice } from "@composables/game/useDevice"
import { getRarityDefinition } from "@services/item-definition"
import TitleQuickRefContent from "./TitleQuickRefContent.vue"
import { getGameDataTable } from "@services/game-data-runtime"
import GameDialog from "@components/game/common/GameDialog.vue"

type TitleCategory = "title_prefix" | "title_suffix"
type TitleEntry = {
  id: string
  nameKey: string
  rarity: string
  category?: string
  order: number
  disabled?: boolean
}

let titlePrefixes: TitleEntry[] | null = null
let titleSuffixes: TitleEntry[] | null = null

function getTitleRows(category: TitleCategory): TitleEntry[] {
  if (category === "title_prefix") {
    if (!titlePrefixes) {
      titlePrefixes = getGameDataTable<TitleEntry>("title_prefixes").filter((e) => !e.disabled)
    }
    return titlePrefixes
  }
  if (!titleSuffixes) {
    titleSuffixes = getGameDataTable<TitleEntry>("title_suffixes").filter((e) => !e.disabled)
  }
  return titleSuffixes
}

function findTitleEntry(category: TitleCategory, itemId: string): TitleEntry | null {
  return getTitleRows(category).find((e) => e.id === itemId) ?? null
}

const props = withDefaults(
  defineProps<{
    category: TitleCategory | string
    itemId: string
    prefixLabel?: string
  }>(),
  {
    prefixLabel: ""
  }
)

const { t } = useI18n()
const { supportsHover, isTouchLike } = useDevice()
const popoverActive = ref(false)
const popoverLoaded = ref(false)
const mobilePopoverVisible = ref(false)

const normalizedCategory = computed<TitleCategory>(() => props.category === "title_prefix" ? "title_prefix" : "title_suffix")
const normalizedItemId = computed(() => String(props.itemId || "").trim())
const titleEntry = computed(() => findTitleEntry(normalizedCategory.value, normalizedItemId.value))
const titleLabel = computed(() => titleEntry.value?.nameKey ? t(titleEntry.value.nameKey) : normalizedItemId.value || t("wardrobe.unknownTitle"))
const prefixLabel = computed(() => props.prefixLabel || "")
const rarityId = computed(() => titleEntry.value?.rarity || "common")
const rarityColor = computed(() => getRarityDefinition(rarityId.value)?.color || "")
const tagStyle = computed(() => rarityColor.value ? { "--title-rarity-color": rarityColor.value } : {})

function onPopoverShow() {
  if (!supportsHover.value) return
  popoverLoaded.value = true
  popoverActive.value = true
}

function onPopoverHide() {
  popoverActive.value = false
}

function primePopover() {
  if (!supportsHover.value) return
  popoverLoaded.value = true
}

function onTagClick() {
  if (!isTouchLike.value) return
  popoverLoaded.value = true
  mobilePopoverVisible.value = true
}
</script>

<style scoped>
.title-quick-ref {
  display: inline-flex;
}

.title-quick-ref-tag {
  display: inline-flex;
  align-items: center;
  gap: 2px;
  padding: 2px 8px;
  border-radius: 4px;
  font-size: 12px;
  cursor: pointer;
  border: 1px solid var(--title-rarity-color, var(--app-border, #ccc));
  color: var(--title-rarity-color, var(--app-text, inherit));
  background: color-mix(in srgb, var(--title-rarity-color, #888) 8%, transparent);
  line-height: 1.4;
}

.title-quick-ref-tag__name {
  font-weight: 500;
}
</style>
