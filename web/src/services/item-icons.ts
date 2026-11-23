import { getGameDataChunk } from "@services/game-data-runtime"
import { getItemDefinition } from "@services/item-definition"
import type { IconDefinition } from "@/types/game-data"

function isIconDefinition(value: unknown): value is IconDefinition {
  if (!value || typeof value !== "object") return false
  const icon = value as Record<string, unknown>
  return typeof icon.id === "string" && typeof icon.path === "string"
}

function getRawIcons(catalog: unknown): IconDefinition[] {
  if (!catalog || typeof catalog !== "object") return []
  const icons = (catalog as { icons?: unknown }).icons
  if (!Array.isArray(icons)) return []
  return icons.filter((icon): icon is IconDefinition => isIconDefinition(icon))
}

let iconPathMap: Map<string, string> | null = null

function getIconPathMap() {
  if (!iconPathMap) {
    iconPathMap = new Map<string, string>(
      getRawIcons(getGameDataChunk("core")).map((icon) => [icon.id, icon.path])
    )
  }
  return iconPathMap
}

export function resolveIconIdByItemId(itemId: string): string {
  const definition = getItemDefinition(itemId)
  return typeof definition?.iconId === "string" ? definition.iconId : ""
}

function normalizePath(rawPath: string): string {
  return rawPath.startsWith("/") ? rawPath : `/${rawPath}`
}

function getMappedIconPath(iconId: string): string | null {
  const map = getIconPathMap()
  if (!map.has(iconId)) return null
  const rawPath = map.get(iconId)
  if (typeof rawPath !== "string") return ""
  const normalized = rawPath.trim()
  if (!normalized) return ""
  return normalizePath(normalized)
}

export function parseIconId(iconId: string): { type: string; id: string } | null {
  const [type, ...rest] = iconId.split(".")
  const id = rest.join(".")
  if (!type || !id) return null
  return { type, id }
}

export function resolveIconPathCandidatesByTypeAndId(type: string, id: string): string[] {
  const normalizedType = type.trim()
  const normalizedId = id.trim()
  if (!normalizedType || !normalizedId) return []
  const mapped = getMappedIconPath(`${normalizedType}.${normalizedId}`)
  return mapped ? [mapped] : []
}

export function resolveIconPathByIconId(iconId: string): string {
  const mapped = getMappedIconPath(iconId)
  return mapped || ""
}

export function resolveIconPathCandidatesByIconId(iconId: string): string[] {
  const mapped = getMappedIconPath(iconId)
  return mapped ? [mapped] : []
}

export function resolveIconPathByItemId(itemId: string): string {
  return resolveIconPathByIconId(resolveIconIdByItemId(itemId))
}

export function resolveItemFallbackIcon(itemId: string): string {
  const definition = getItemDefinition(itemId)
  return typeof definition?.emojiIcon === "string" && definition.emojiIcon ? definition.emojiIcon : "❓"
}

export function resolveFallbackIconByIconId(iconId: string, fallback = "🖼️"): string {
  const parsed = parseIconId(iconId)
  if (parsed?.type === "item") return resolveItemFallbackIcon(parsed.id)
  return fallback
}
