import { readonly, shallowRef } from "vue"

export type IconAtlasImageStatus = "idle" | "loading" | "loaded" | "failed"

export type IconAtlasEntry = {
  atlas?: string
  x: number
  y: number
  w: number
  h: number
}

export type IconAtlasSheet = {
  id: string
  image: string
  width: number
  height: number
  imageBytes: number
}

export type IconAtlasManifest = {
  schemaVersion: number
  type: string
  image: string
  format: string
  cellSize: number
  width: number
  height: number
  imageBytes: number
  atlases?: ReadonlyArray<IconAtlasSheet>
  entries: Record<string, IconAtlasEntry>
}

const ITEM_ATLAS_MANIFEST_URL = "/game-data/icon-atlas/item-atlas.json"
const itemAtlasManifest = shallowRef<IconAtlasManifest | null>(null)
const itemAtlasFailed = shallowRef(false)
const itemAtlasImageStatus = shallowRef<Record<string, IconAtlasImageStatus>>({})
let itemAtlasPromise: Promise<IconAtlasManifest | null> | null = null
const itemAtlasImagePromises = new Map<string, Promise<IconAtlasImageStatus>>()

export function useItemIconAtlas() {
  return {
    manifest: readonly(itemAtlasManifest),
    failed: readonly(itemAtlasFailed),
    imageStatus: readonly(itemAtlasImageStatus)
  }
}

export function loadItemIconAtlas(): Promise<IconAtlasManifest | null> {
  if (itemAtlasManifest.value) return Promise.resolve(itemAtlasManifest.value)
  if (itemAtlasPromise) return itemAtlasPromise
  itemAtlasFailed.value = false
  itemAtlasPromise = fetch(ITEM_ATLAS_MANIFEST_URL, { cache: "no-cache" })
    .then(async (response) => {
      if (!response.ok) throw new Error(`icon atlas manifest ${response.status}`)
      const manifest = await response.json() as IconAtlasManifest
      if (!isValidItemAtlasManifest(manifest)) throw new Error("invalid icon atlas manifest")
      itemAtlasManifest.value = manifest
      return manifest
    })
    .catch(() => {
      itemAtlasFailed.value = true
      return null
    })
    .finally(() => {
      itemAtlasPromise = null
    })
  return itemAtlasPromise
}

export function loadItemIconAtlasImage(url: string): Promise<IconAtlasImageStatus> {
  const normalizedUrl = url.trim()
  if (!normalizedUrl) return Promise.resolve("failed")
  const currentStatus = itemAtlasImageStatus.value[normalizedUrl]
  if (currentStatus === "loaded" || currentStatus === "failed") return Promise.resolve(currentStatus)
  const existingPromise = itemAtlasImagePromises.get(normalizedUrl)
  if (existingPromise) return existingPromise

  setItemAtlasImageStatus(normalizedUrl, "loading")
  const promise = new Promise<IconAtlasImageStatus>((resolve) => {
    const image = new Image()
    image.decoding = "async"
    image.onload = () => {
      setItemAtlasImageStatus(normalizedUrl, "loaded")
      itemAtlasImagePromises.delete(normalizedUrl)
      resolve("loaded")
    }
    image.onerror = () => {
      setItemAtlasImageStatus(normalizedUrl, "failed")
      itemAtlasImagePromises.delete(normalizedUrl)
      resolve("failed")
    }
    image.src = normalizedUrl
  })
  itemAtlasImagePromises.set(normalizedUrl, promise)
  return promise
}

export function isItemIconAtlasCandidate(key: string): boolean {
  const normalized = key.trim()
  return normalized.startsWith("item.") || normalized.startsWith("/assets/icons/item/") || normalized.startsWith("/assets/icons/items/")
}

export function findItemIconAtlasEntry(manifest: IconAtlasManifest | null, keys: string[]): IconAtlasEntry | null {
  if (!manifest) return null
  for (const key of keys) {
    const normalized = normalizeAtlasKey(key)
    if (!normalized) continue
    const entry = manifest.entries[normalized]
    if (entry) return entry
  }
  return null
}

export function resolveItemIconAtlasSheet(manifest: IconAtlasManifest | null, entry: IconAtlasEntry | null): IconAtlasSheet | null {
  if (!manifest || !entry) return null
  const atlases = Array.isArray(manifest.atlases) ? manifest.atlases : []
  if (entry.atlas) {
    const sheet = atlases.find((atlas) => atlas.id === entry.atlas)
    if (sheet) return sheet
  }
  if (manifest.image) {
    return {
      id: entry.atlas || "item",
      image: manifest.image,
      width: manifest.width,
      height: manifest.height,
      imageBytes: manifest.imageBytes
    }
  }
  return null
}

function normalizeAtlasKey(key: string): string {
  const trimmed = key.trim()
  if (!trimmed) return ""
  return trimmed.startsWith("/") || trimmed.startsWith("item.") ? trimmed : `/${trimmed}`
}

function setItemAtlasImageStatus(url: string, status: IconAtlasImageStatus) {
  itemAtlasImageStatus.value = {
    ...itemAtlasImageStatus.value,
    [url]: status
  }
}

function isValidItemAtlasManifest(value: unknown): value is IconAtlasManifest {
  if (!value || typeof value !== "object") return false
  const manifest = value as Partial<IconAtlasManifest>
  return (
    manifest.type === "item" &&
    typeof manifest.image === "string" &&
    typeof manifest.width === "number" &&
    typeof manifest.height === "number" &&
    !!manifest.entries &&
    typeof manifest.entries === "object"
  )
}
