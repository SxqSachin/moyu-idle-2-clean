import { getItemDefinition } from "@services/item-definition"
import type { ItemLabelOptions, Translate } from "@/types/item-label"

export function getLocalizedItemName(t: Translate, itemId: string, options: ItemLabelOptions = {}): string {
  const preferredNameKey = options.nameKey || getItemDefinition(itemId)?.nameKey
  const fullKeys = typeof preferredNameKey === "string" && preferredNameKey ? [preferredNameKey] : [`item.${itemId}.name`]
  for (const fullKey of fullKeys) {
    const fullLocalized = t(fullKey)
    if (fullLocalized !== fullKey && fullLocalized.trim() !== "") return fullLocalized
  }

  if (options.fallbackText && options.fallbackText.trim() !== "") return options.fallbackText
  return itemId
}
