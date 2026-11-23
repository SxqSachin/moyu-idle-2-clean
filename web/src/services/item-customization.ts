import type { ItemCustomization } from "@/types/equipment"

const LABEL_COLOR_CLASS_PREFIX = "item-custom-label--"
const SUPPORTED_LABEL_COLORS = new Set(["gold", "rose", "violet", "amber", "sky", "emerald", "fuchsia"])

export function normalizeItemCustomization(raw: ItemCustomization | null | undefined): ItemCustomization {
  return {
    name: String(raw?.name || "").trim(),
    labelColor: String(raw?.labelColor || "").trim().toLowerCase()
  }
}

export function itemCustomName(raw: ItemCustomization | null | undefined): string {
  return normalizeItemCustomization(raw).name || ""
}

export function itemCustomLabelColor(raw: ItemCustomization | null | undefined): string {
  const color = normalizeItemCustomization(raw).labelColor
  return SUPPORTED_LABEL_COLORS.has(color) ? color : "gold"
}

export function hasItemCustomization(raw: ItemCustomization | null | undefined): boolean {
  return itemCustomName(raw) !== ""
}

export function itemCustomLabelClass(raw: ItemCustomization | null | undefined): string {
  if (!hasItemCustomization(raw)) return ""
  return `item-custom-label ${LABEL_COLOR_CLASS_PREFIX}${itemCustomLabelColor(raw)}`
}

export function itemCustomLabelClassForColor(color: string): string {
  const normalized = String(color || "").trim().toLowerCase()
  return `item-custom-label ${LABEL_COLOR_CLASS_PREFIX}${SUPPORTED_LABEL_COLORS.has(normalized) ? normalized : "gold"}`
}

export function isValidItemCustomName(name: string): boolean {
  let seenVisible = false
  let previousEmoji = false
  let previousJoiner = false
  for (const char of Array.from(name)) {
    if (isItemCustomNameTextChar(char)) {
      if (previousJoiner) return false
      seenVisible = true
      previousEmoji = false
      previousJoiner = false
      continue
    }
    if (isEmojiModifierChar(char)) {
      if (!previousEmoji) return false
      previousJoiner = false
      continue
    }
    if (char === "\ufe0f") {
      if (!previousEmoji) return false
      previousJoiner = false
      continue
    }
    if (char === "\u200d") {
      if (!previousEmoji || previousJoiner) return false
      previousEmoji = false
      previousJoiner = true
      continue
    }
    if (isEmojiBaseChar(char)) {
      seenVisible = true
      previousEmoji = true
      previousJoiner = false
      continue
    }
    return false
  }
  return seenVisible && !previousJoiner
}

function isItemCustomNameTextChar(char: string): boolean {
  return /^[A-Za-z0-9_\u3400-\u9FFF\u3040-\u309F\u30A0-\u30FF\uAC00-\uD7AF]$/u.test(char)
}

function isEmojiBaseChar(char: string): boolean {
  return /^[\u{1F000}-\u{1FAFF}\u{2600}-\u{27BF}]$/u.test(char)
}

function isEmojiModifierChar(char: string): boolean {
  return /^[\u{1F3FB}-\u{1F3FF}]$/u.test(char)
}
