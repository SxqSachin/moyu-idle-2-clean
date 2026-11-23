import { reactive } from "vue"

const STORAGE_KEY = "item-browser:secondary-category"
const DEFAULT_SECONDARY_CATEGORY_SETTINGS: Readonly<Record<string, boolean>> = {
  fishing_gear: true
}

const secondaryCategorySettings = reactive<Record<string, boolean>>({})

let hydrated = false

function normalizeCategoryKey(raw: string): string {
  const key = String(raw || "").trim()
  return key || "all"
}

function hydrateSecondaryCategorySettings() {
  if (hydrated) return
  hydrated = true
  if (typeof localStorage === "undefined") return

  const raw = localStorage.getItem(STORAGE_KEY)
  if (!raw) return

  try {
    const parsed = JSON.parse(raw) as Record<string, unknown>
    if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) return
    for (const [key, value] of Object.entries(parsed)) {
      if (typeof value === "boolean") {
        secondaryCategorySettings[normalizeCategoryKey(key)] = value
      }
    }
  } catch {
    // Ignore malformed cache and fall back to category defaults.
  }
}

function persistSecondaryCategorySettings() {
  if (typeof localStorage === "undefined") return
  localStorage.setItem(STORAGE_KEY, JSON.stringify(secondaryCategorySettings))
}

export function getItemBrowserSecondaryCategoryEnabled(category: string): boolean {
  hydrateSecondaryCategorySettings()
  const key = normalizeCategoryKey(category)
  if (Object.prototype.hasOwnProperty.call(secondaryCategorySettings, key)) {
    return Boolean(secondaryCategorySettings[key])
  }
  return Boolean(DEFAULT_SECONDARY_CATEGORY_SETTINGS[key])
}

export function setItemBrowserSecondaryCategoryEnabled(category: string, enabled: boolean) {
  hydrateSecondaryCategorySettings()
  secondaryCategorySettings[normalizeCategoryKey(category)] = Boolean(enabled)
  persistSecondaryCategorySettings()
}

export function toggleItemBrowserSecondaryCategoryEnabled(category: string) {
  setItemBrowserSecondaryCategoryEnabled(category, !getItemBrowserSecondaryCategoryEnabled(category))
}

export function resetItemBrowserSecondaryCategorySettingsForTest() {
  hydrated = false
  for (const key of Object.keys(secondaryCategorySettings)) {
    delete secondaryCategorySettings[key]
  }
}
