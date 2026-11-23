<template>
  <div class="title-ref-content">
    <RarityText class="title-ref-content__rarity text-xs" :rarity-id="rarity" />
    <div class="title-ref-content__name">{{ label }}</div>
    <div class="title-ref-content__meta">
      <span class="title-ref-content__position">{{ positionLabel }}</span>
      <!-- <span class="title-ref-content__state">{{ stateLabel }}</span> -->
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from "vue"
import { useI18n } from "vue-i18n"
import RarityText from "@components/game/common/RarityText.vue"
import { useWardrobeStore } from "@stores/wardrobe"
import { getGameDataTable } from "@services/game-data-runtime"

type TitleCategory = "title_prefix" | "title_suffix"
type TitleEntry = {
  id: string
  nameKey: string
  rarity: string
  category?: string
  order: number
  disabled?: boolean
}

const props = defineProps<{
  category: TitleCategory | string
  itemId: string
}>()

const { t } = useI18n()
const wardrobe = useWardrobeStore()

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

const category = computed<TitleCategory>(() => props.category === "title_prefix" ? "title_prefix" : "title_suffix")
const itemId = computed(() => String(props.itemId || "").trim())

const entry = computed(() => {
  return getTitleRows(category.value).find((e) => e.id === itemId.value) ?? null
})

const label = computed(() => {
  const e = entry.value
  return e?.nameKey ? t(e.nameKey) : itemId.value || t("wardrobe.unknownTitle")
})

const positionLabel = computed(() => category.value === "title_prefix" ? t("wardrobe.prefixes") : t("wardrobe.suffixes"))

const rarity = computed(() => entry.value?.rarity || "common")

const stateLabel = computed(() => {
  const cat = category.value
  const id = itemId.value
  const isEquipped = cat === "title_prefix"
    ? wardrobe.playerTitlePrefix === id
    : wardrobe.playerTitleSuffix === id
  if (isEquipped) return t("wardrobe.equipped")
  const isOwned = wardrobe.owned.some((o) => o.category === cat && o.itemId === id)
  return isOwned ? t("wardrobe.owned") : t("wardrobe.locked")
})
</script>

<style scoped>
.title-ref-content {
  position: relative;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.title-ref-content__rarity {
  position: absolute;
  top: 0;
  right: 0;
}

.title-ref-content__name {
  font-weight: 700;
  font-size: 12px;
  padding-right: 60px;
}

.title-ref-content__meta {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-size: 12px;
  color: var(--app-muted);
}

.title-ref-content__position {
  padding: 1px 6px;
  border-radius: 3px;
  background: var(--app-bg-secondary, #f0f0f0);
}
</style>
