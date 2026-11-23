import { createI18n } from "vue-i18n"

export const SupportedAppLocales = ["zh-CN", "zh-TW", "en-US", "ja", "ko", "ru"] as const
export type AppLocale = typeof SupportedAppLocales[number]

export const SupportedLocaleNamespaces = ["core", "home", "auth", "game", "gameData", "tutorial"] as const
export type LocaleNamespace = typeof SupportedLocaleNamespaces[number]

export const FallbackAppLocale: AppLocale = "en-US"
export const DefaultAppLocale: AppLocale = FallbackAppLocale

export const PublicHomeLocaleNamespaces: LocaleNamespace[] = ["core", "home"]
export const PublicAuthLocaleNamespaces: LocaleNamespace[] = ["core", "auth"]
export const GameLocaleNamespaces: LocaleNamespace[] = ["core", "game", "gameData"]
export const GameBackgroundLocaleNamespaces: LocaleNamespace[] = ["tutorial"]
export const GameFullLocaleNamespaces: LocaleNamespace[] = ["core", "game", "gameData", "tutorial"]

const localeLabels: Record<AppLocale, string> = {
  "zh-CN": "简体中文",
  "zh-TW": "繁體中文",
  "en-US": "English",
  ja: "日本語",
  ko: "한국어",
  ru: "Русский"
}

const browserLocaleAliases: Array<[RegExp, AppLocale]> = [
  [/^zh-(tw|hk|mo)\b/i, "zh-TW"],
  [/^zh\b/i, "zh-CN"],
  [/^en\b/i, "en-US"],
  [/^ja\b/i, "ja"],
  [/^ko\b/i, "ko"],
  [/^ru\b/i, "ru"]
]

const initialLocaleMessages: Record<AppLocale, Record<string, unknown>> = {
  "zh-CN": {},
  "zh-TW": {},
  "en-US": {},
  ja: {},
  ko: {},
  ru: {}
}

type LocaleModule = Promise<{ default: Record<string, unknown> }>
type NamespaceLoaders = Record<LocaleNamespace, () => LocaleModule>

const lazyLocaleLoaders: Record<AppLocale, NamespaceLoaders> = {
  "zh-CN": {
    core: () => import("@/locales/core/zh-CN"),
    home: () => import("@/locales/home/zh-CN"),
    auth: () => import("@/locales/auth/zh-CN"),
    game: () => import("@/locales/game/zh-CN"),
    gameData: () => import("@/locales/game-data/zh-CN"),
    tutorial: () => import("@/locales/tutorial/zh-CN")
  },
  "zh-TW": {
    core: () => import("@/locales/core/zh-TW"),
    home: () => import("@/locales/home/zh-TW"),
    auth: () => import("@/locales/auth/zh-TW"),
    game: () => import("@/locales/game/zh-TW"),
    gameData: () => import("@/locales/game-data/zh-TW"),
    tutorial: () => import("@/locales/tutorial/zh-TW")
  },
  "en-US": {
    core: () => import("@/locales/core/en-US"),
    home: () => import("@/locales/home/en-US"),
    auth: () => import("@/locales/auth/en-US"),
    game: () => import("@/locales/game/en-US"),
    gameData: () => import("@/locales/game-data/en-US"),
    tutorial: () => import("@/locales/tutorial/en-US")
  },
  ja: {
    core: () => import("@/locales/core/ja"),
    home: () => import("@/locales/home/ja"),
    auth: () => import("@/locales/auth/ja"),
    game: () => import("@/locales/game/ja"),
    gameData: () => import("@/locales/game-data/ja"),
    tutorial: () => import("@/locales/tutorial/ja")
  },
  ko: {
    core: () => import("@/locales/core/ko"),
    home: () => import("@/locales/home/ko"),
    auth: () => import("@/locales/auth/ko"),
    game: () => import("@/locales/game/ko"),
    gameData: () => import("@/locales/game-data/ko"),
    tutorial: () => import("@/locales/tutorial/ko")
  },
  ru: {
    core: () => import("@/locales/core/ru"),
    home: () => import("@/locales/home/ru"),
    auth: () => import("@/locales/auth/ru"),
    game: () => import("@/locales/game/ru"),
    gameData: () => import("@/locales/game-data/ru"),
    tutorial: () => import("@/locales/tutorial/ru")
  }
}

const loadedLocaleNamespaces = new Map<AppLocale, Set<LocaleNamespace>>()
const loadingLocaleNamespaces = new Map<string, Promise<void>>()

function hasLocale(value: string): value is AppLocale {
  return (SupportedAppLocales as readonly string[]).includes(value)
}

function hasNamespace(value: string): value is LocaleNamespace {
  return (SupportedLocaleNamespaces as readonly string[]).includes(value)
}

function mapBrowserLocale(raw: string): AppLocale | null {
  const normalized = raw.trim()
  if (!normalized) return null
  if (hasLocale(normalized)) return normalized
  for (const [pattern, locale] of browserLocaleAliases) {
    if (pattern.test(normalized)) return locale
  }
  return null
}

export function normalizeSupportedLocale(raw: string | null | undefined): AppLocale {
  if (!raw) return DefaultAppLocale
  return mapBrowserLocale(raw) ?? DefaultAppLocale
}

export function normalizeLocaleNamespaces(namespaces: readonly string[] | null | undefined): LocaleNamespace[] {
  const normalized: LocaleNamespace[] = []
  for (const namespace of namespaces || []) {
    if (hasNamespace(namespace) && !normalized.includes(namespace)) {
      normalized.push(namespace)
    }
  }
  return normalized.length > 0 ? normalized : ["core"]
}

function resolveBrowserPreferredLocale(): AppLocale {
  if (typeof navigator === "undefined") return DefaultAppLocale
  const languages = Array.isArray(navigator.languages) && navigator.languages.length > 0
    ? navigator.languages
    : [navigator.language]
  for (const candidate of languages) {
    const resolved = mapBrowserLocale(candidate || "")
    if (resolved) return resolved
  }
  return DefaultAppLocale
}

function resolveInitialLocale(): AppLocale {
  const stored = localStorage.getItem("locale")
  if (stored) {
    const normalized = normalizeSupportedLocale(stored)
    if (normalized !== stored) localStorage.setItem("locale", normalized)
    return normalized
  }
  const detected = resolveBrowserPreferredLocale()
  localStorage.setItem("locale", detected)
  return detected
}

export function getLocaleOptions() {
  return SupportedAppLocales.map((value) => ({
    value,
    label: localeLabels[value]
  }))
}

export function persistLocaleSelection(raw: string): AppLocale {
  const normalized = normalizeSupportedLocale(raw)
  localStorage.setItem("locale", normalized)
  return normalized
}

const locale = resolveInitialLocale()

const i18n = createI18n({
  legacy: false,
  locale,
  fallbackLocale: FallbackAppLocale,
  messages: initialLocaleMessages,
  missing(locale, key) {
    if (locale !== FallbackAppLocale && !isLocaleMessagesLoaded(FallbackAppLocale)) {
      void ensureLocaleMessages(FallbackAppLocale)
    }
  }
})

function loadingKey(locale: AppLocale, namespace: LocaleNamespace) {
  return `${locale}:${namespace}`
}

function loadedSet(locale: AppLocale) {
  let set = loadedLocaleNamespaces.get(locale)
  if (!set) {
    set = new Set<LocaleNamespace>()
    loadedLocaleNamespaces.set(locale, set)
  }
  return set
}

export function isLocaleNamespaceLoaded(locale: AppLocale, namespace: LocaleNamespace): boolean {
  return loadedLocaleNamespaces.get(locale)?.has(namespace) === true
}

export function areLocaleNamespacesLoaded(locale: AppLocale, namespaces: readonly LocaleNamespace[]): boolean {
  return normalizeLocaleNamespaces(namespaces).every((namespace) => isLocaleNamespaceLoaded(locale, namespace))
}

export function isLocaleMessagesLoaded(locale: AppLocale): boolean {
  return areLocaleNamespacesLoaded(locale, SupportedLocaleNamespaces)
}

export function getLocaleNamespacesForPath(path: string): LocaleNamespace[] {
  if (path.startsWith("/game")) return GameLocaleNamespaces
  if (path.startsWith("/auth") || path.startsWith("/instructions")) return PublicAuthLocaleNamespaces
  return PublicHomeLocaleNamespaces
}

export function getRouteLocaleNamespaces(path: string, rawNamespaces: readonly string[] | readonly LocaleNamespace[] | null | undefined): LocaleNamespace[] {
  const metaNamespaces = rawNamespaces ? normalizeLocaleNamespaces(rawNamespaces) : []
  return normalizeLocaleNamespaces([...metaNamespaces, ...getLocaleNamespacesForPath(path)])
}

export async function ensureLocaleNamespaces(rawLocale: AppLocale | string, rawNamespaces: readonly LocaleNamespace[] | readonly string[]): Promise<void> {
  const locale = normalizeSupportedLocale(rawLocale)
  const namespaces = normalizeLocaleNamespaces(rawNamespaces)
  await Promise.all(namespaces.map((namespace) => ensureLocaleNamespace(locale, namespace)))
}

async function ensureLocaleNamespace(locale: AppLocale, namespace: LocaleNamespace): Promise<void> {
  if (isLocaleNamespaceLoaded(locale, namespace)) return
  const key = loadingKey(locale, namespace)
  const pending = loadingLocaleNamespaces.get(key)
  if (pending) return pending
  const loader = lazyLocaleLoaders[locale]?.[namespace]
  if (!loader) return
  const promise = loader()
    .then((module) => {
      i18n.global.mergeLocaleMessage(locale, module.default)
      loadedSet(locale).add(namespace)
    })
    .finally(() => {
      loadingLocaleNamespaces.delete(key)
    })
  loadingLocaleNamespaces.set(key, promise)
  return promise
}

export async function ensureLocaleMessages(locale: AppLocale): Promise<void> {
  await ensureLocaleNamespaces(locale, SupportedLocaleNamespaces)
}

export async function setAppLocale(raw: string | null | undefined, namespaces: readonly LocaleNamespace[] = SupportedLocaleNamespaces): Promise<AppLocale> {
  const normalized = normalizeSupportedLocale(raw)
  await ensureLocaleNamespaces(normalized, namespaces)
  i18n.global.locale.value = normalized
  syncDocumentTitle()
  return normalized
}

export function syncDocumentTitle(titleKey = "app.title") {
  if (typeof document === "undefined") return
  document.documentElement.lang = normalizeSupportedLocale(i18n.global.locale.value)
  const translated = i18n.global.t(titleKey)
  document.title = translated && translated !== titleKey ? translated : localeLabels[normalizeSupportedLocale(i18n.global.locale.value)]
}

export default i18n
