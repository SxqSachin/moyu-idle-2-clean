/**
 * Skin registry — single source of truth for available visual skins.
 *
 * Skins are client-side only (no backend enum). To add a skin:
 *   1. Add an `html[data-skin="<id>"]` block to `themes/skins.css`
 *   2. Add the ID to `SKINS` below
 *   3. Add i18n key `skin.<id>` to all locale files
 *   4. Add a swatch preview in `components/theme/SkinSelector.vue`
 */

export const SKINS = ["default", "mystic", "ember"] as const
export type ThemeSkin = (typeof SKINS)[number]

export const DEFAULT_SKIN: ThemeSkin = "default"

const STORAGE_KEY = "player-skin"

/** i18n key for each skin's display name */
export const SKIN_I18N_KEY: Record<ThemeSkin, string> = {
  default: "skin.default",
  mystic: "skin.mystic",
  ember: "skin.ember",
}

/** Swatch colors shown in the selector UI (primary + accent pair) */
export const SKIN_SWATCHES: Record<ThemeSkin, { primary: string; accent: string }> = {
  default: { primary: "#0066cc", accent: "#2997ff" },
  mystic: { primary: "#7c3aed", accent: "#c084fc" },
  ember: { primary: "#ea580c", accent: "#f59e0b" },
}

export function isThemeSkin(value: unknown): value is ThemeSkin {
  return typeof value === "string" && (SKINS as readonly string[]).includes(value)
}

export function readStoredSkin(): ThemeSkin {
  const raw = localStorage.getItem(STORAGE_KEY)
  return isThemeSkin(raw) ? raw : DEFAULT_SKIN
}

export function writeStoredSkin(skin: ThemeSkin): void {
  localStorage.setItem(STORAGE_KEY, skin)
}

export { STORAGE_KEY as SKIN_STORAGE_KEY }
