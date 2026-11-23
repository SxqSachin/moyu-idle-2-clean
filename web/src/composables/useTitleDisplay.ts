import { computed } from "vue"
import { useI18n } from "vue-i18n"
import { useWardrobeStore } from "@stores/wardrobe"
import { getGameDataTable } from "@services/game-data-runtime"

type TitleEntry = {
  id: string
  nameKey: string
  rarity: string
  category?: string
  order: number
  disabled: boolean
}

let titlePrefixes: TitleEntry[] | null = null
let titleSuffixes: TitleEntry[] | null = null

function getTitlePrefixes(): TitleEntry[] {
  if (!titlePrefixes) {
    titlePrefixes = getGameDataTable<TitleEntry>("title_prefixes")
  }
  return titlePrefixes
}

function getTitleSuffixes(): TitleEntry[] {
  if (!titleSuffixes) {
    titleSuffixes = getGameDataTable<TitleEntry>("title_suffixes")
  }
  return titleSuffixes
}

export function findPrefix(id: string): TitleEntry | null {
  return getTitlePrefixes().find((e) => e.id === id && !e.disabled) ?? null
}

export function findSuffix(id: string): TitleEntry | null {
  return getTitleSuffixes().find((e) => e.id === id && !e.disabled) ?? null
}

/**
 * Formats a player's equipped title from wardrobe store data.
 * Translates nameKeys via i18n.
 */
export function usePlayerTitleDisplay() {
  const { t } = useI18n()
  const wardrobe = useWardrobeStore()

  const titlePrefixEntry = computed(() => {
    const id = wardrobe.playerTitlePrefix
    return id ? findPrefix(id) : null
  })

  const titleSuffixEntry = computed(() => {
    const id = wardrobe.playerTitleSuffix
    return id ? findSuffix(id) : null
  })

  const displayTitle = computed(() => {
    const parts: string[] = []
    const prefix = titlePrefixEntry.value
    const suffix = titleSuffixEntry.value
    if (prefix?.nameKey) {
      const translated = t(prefix.nameKey)
      if (translated && translated !== prefix.nameKey) parts.push(translated)
    }
    if (suffix?.nameKey) {
      const translated = t(suffix.nameKey)
      if (translated && translated !== suffix.nameKey) parts.push(translated)
    }
    return parts.join("")
  })

  const hasTitle = computed(() => displayTitle.value.length > 0)

  return { displayTitle, hasTitle, titlePrefixEntry, titleSuffixEntry }
}
