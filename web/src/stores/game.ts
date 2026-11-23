import { defineStore } from "pinia"
import type { PlayerLanguage, PlayerMenuLayout, PlayerTheme } from "@services/player-settings"
import { DefaultAppLocale, normalizeSupportedLocale } from "@/i18n"

const DEFAULT_PLAYER_LANGUAGE: PlayerLanguage = DefaultAppLocale
const DEFAULT_PLAYER_THEME: PlayerTheme = "light"
const DEFAULT_PLAYER_MENU_LAYOUT: PlayerMenuLayout = "icon-rail"
const DEFAULT_PLAYER_SIMPLE_MODE = false
const LEGACY_PLAYER_SIMPLE_MODE = true

function normalizeMenuLayout(raw: PlayerMenuLayout | string | undefined): PlayerMenuLayout {
  return raw === "drawer" || raw === "icon-rail" ? raw : DEFAULT_PLAYER_MENU_LAYOUT
}

function settingsStorageKey(userId: number): string {
  return `player-settings:${userId}`
}

type StoredPlayerSettings = {
  language?: PlayerLanguage
  theme?: PlayerTheme
  menuLayout?: PlayerMenuLayout
  simpleMode?: boolean
  emailVerified?: boolean
}

export const useGameStore = defineStore("game", {
  state: () => ({
    userId: 0,
    username: "",
    token: "",
    emailVerified: false,
    playerLanguage: DEFAULT_PLAYER_LANGUAGE as PlayerLanguage,
    playerTheme: DEFAULT_PLAYER_THEME as PlayerTheme,
    playerMenuLayout: DEFAULT_PLAYER_MENU_LAYOUT as PlayerMenuLayout,
    playerSimpleMode: DEFAULT_PLAYER_SIMPLE_MODE,
    chat: [] as Array<{ userId: number; content: string; ts: number }>,
    recycleConfig: {
      goldCoefficient: 0.5,
      affixMismatchExtraCoefficient: 0.6,
    }
  }),
  actions: {
    setAuth(userId: number, token: string, username = "") {
      this.userId = userId
      this.username = username
      this.token = token
      localStorage.setItem("userId", String(userId))
      localStorage.setItem("username", username)
      localStorage.setItem("token", token)
      this.restorePlayerSettings()
    },
    restoreToken() {
      const rawUserId = localStorage.getItem("userId") || "0"
      const parsedUserId = Number.parseInt(rawUserId, 10)
      this.userId = Number.isFinite(parsedUserId) ? parsedUserId : 0
      this.username = localStorage.getItem("username") || ""
      this.token = localStorage.getItem("token") || ""
      this.restorePlayerSettings()
    },
    clearAuth() {
      this.userId = 0
      this.username = ""
      this.token = ""
      this.emailVerified = false
      this.playerLanguage = DEFAULT_PLAYER_LANGUAGE
      this.playerTheme = DEFAULT_PLAYER_THEME
      this.playerMenuLayout = DEFAULT_PLAYER_MENU_LAYOUT
      this.playerSimpleMode = DEFAULT_PLAYER_SIMPLE_MODE
      localStorage.removeItem("userId")
      localStorage.removeItem("username")
      localStorage.removeItem("token")
    },
    appendChat(line: { userId: number; content: string; ts: number }) {
      this.chat.push(line)
      if (this.chat.length > 100) this.chat.shift()
    },
    restorePlayerSettings() {
      if (this.userId <= 0) {
        this.playerLanguage = normalizeSupportedLocale(localStorage.getItem("locale"))
        this.playerTheme = (localStorage.getItem("dark-mode") === "1" ? "dark" : "light") as PlayerTheme
        this.playerMenuLayout = DEFAULT_PLAYER_MENU_LAYOUT
        this.playerSimpleMode = DEFAULT_PLAYER_SIMPLE_MODE
        this.emailVerified = false
        return
      }
      const raw = localStorage.getItem(settingsStorageKey(this.userId))
      if (!raw) {
        this.playerLanguage = normalizeSupportedLocale(localStorage.getItem("locale"))
        this.playerTheme = (localStorage.getItem("dark-mode") === "1" ? "dark" : "light") as PlayerTheme
        this.playerMenuLayout = DEFAULT_PLAYER_MENU_LAYOUT
        this.playerSimpleMode = LEGACY_PLAYER_SIMPLE_MODE
        this.emailVerified = false
        return
      }
      try {
        const parsed = JSON.parse(raw) as StoredPlayerSettings
        this.playerLanguage = normalizeSupportedLocale(parsed.language)
        this.playerTheme = parsed.theme === "dark" ? "dark" : "light"
        this.playerMenuLayout = normalizeMenuLayout(parsed.menuLayout)
        this.playerSimpleMode = typeof parsed.simpleMode === "boolean" ? parsed.simpleMode : LEGACY_PLAYER_SIMPLE_MODE
        this.emailVerified = Boolean(parsed.emailVerified)
      } catch {
        this.playerLanguage = DEFAULT_PLAYER_LANGUAGE
        this.playerTheme = DEFAULT_PLAYER_THEME
        this.playerMenuLayout = DEFAULT_PLAYER_MENU_LAYOUT
        this.playerSimpleMode = LEGACY_PLAYER_SIMPLE_MODE
        this.emailVerified = false
      }
    },
    persistPlayerSettings() {
      if (this.userId <= 0) return
      const payload: StoredPlayerSettings = {
        language: this.playerLanguage,
        theme: this.playerTheme,
        menuLayout: this.playerMenuLayout,
        simpleMode: this.playerSimpleMode,
        emailVerified: this.emailVerified
      }
      localStorage.setItem(settingsStorageKey(this.userId), JSON.stringify(payload))
    },
    setEmailVerified(emailVerified: boolean) {
      this.emailVerified = Boolean(emailVerified)
      this.persistPlayerSettings()
    },
    setPlayerLanguage(language: PlayerLanguage) {
      const normalized = normalizeSupportedLocale(language)
      this.playerLanguage = normalized
      this.persistPlayerSettings()
      localStorage.setItem("locale", normalized)
    },
    setPlayerTheme(theme: PlayerTheme) {
      this.playerTheme = theme
      this.persistPlayerSettings()
      localStorage.setItem("dark-mode", theme === "dark" ? "1" : "0")
    },
    setPlayerMenuLayout(menuLayout: PlayerMenuLayout) {
      this.playerMenuLayout = normalizeMenuLayout(menuLayout)
      this.persistPlayerSettings()
    },
    setPlayerSimpleMode(simpleMode: boolean) {
      this.playerSimpleMode = Boolean(simpleMode)
      this.persistPlayerSettings()
    },
    applyRemoteSettings(payload: { username?: string; language?: PlayerLanguage; theme?: PlayerTheme; menuLayout?: PlayerMenuLayout; simpleMode?: boolean; emailVerified?: boolean }) {
      if (payload.username) {
        this.username = payload.username
        localStorage.setItem("username", payload.username)
      }
      if (typeof payload.emailVerified === "boolean") {
        this.setEmailVerified(payload.emailVerified)
      }
      if (payload.language) {
        this.setPlayerLanguage(payload.language)
      }
      if (payload.theme) {
        this.setPlayerTheme(payload.theme)
      }
      if (payload.menuLayout) {
        this.setPlayerMenuLayout(payload.menuLayout)
      }
      if (typeof payload.simpleMode === "boolean") {
        this.setPlayerSimpleMode(payload.simpleMode)
      }
    },
    applyRecycleConfig(cfg: { goldCoefficient?: number; affixMismatchExtraCoefficient?: number }) {
      if (typeof cfg.goldCoefficient === "number" && cfg.goldCoefficient > 0) {
        this.recycleConfig.goldCoefficient = cfg.goldCoefficient
      }
      if (typeof cfg.affixMismatchExtraCoefficient === "number" && cfg.affixMismatchExtraCoefficient > 0) {
        this.recycleConfig.affixMismatchExtraCoefficient = cfg.affixMismatchExtraCoefficient
      }
    }
  }
})
