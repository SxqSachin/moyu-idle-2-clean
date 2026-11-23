import { ref, watch } from "vue"
import {
  DEFAULT_SKIN,
  isThemeSkin,
  readStoredSkin,
  writeStoredSkin,
  type ThemeSkin,
} from "@/themes/registry"

const ATTR = "data-skin"

const currentSkin = ref<ThemeSkin>(DEFAULT_SKIN)
let initialized = false

function apply(skin: ThemeSkin): void {
  document.documentElement.setAttribute(ATTR, skin)
  currentSkin.value = skin
}

/**
 * Initialize skin from localStorage (or default). Safe to call once at app
 * bootstrap. Subsequent calls are no-ops.
 */
export function useSkin() {
  function init(): ThemeSkin {
    if (initialized) return currentSkin.value
    const stored = readStoredSkin()
    apply(stored)
    initialized = true
    return stored
  }

  function set(skin: unknown): ThemeSkin {
    const next = isThemeSkin(skin) ? skin : DEFAULT_SKIN
    apply(next)
    writeStoredSkin(next)
    return next
  }

  function reset(): ThemeSkin {
    return set(DEFAULT_SKIN)
  }

  /** Reactive current skin — use in templates for conditional styling */
  function useCurrentSkin() {
    return currentSkin
  }

  /** Watch skin changes — useful for analytics or side effects */
  function watchSkin(handler: (skin: ThemeSkin) => void) {
    return watch(currentSkin, handler)
  }

  return { init, set, reset, useCurrentSkin, watchSkin }
}
