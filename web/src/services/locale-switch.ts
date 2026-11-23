import "element-plus/theme-chalk/el-message.css"
import "element-plus/theme-chalk/el-message-box.css"
import { ElMessage, ElMessageBox } from "element-plus"
import i18n, {
  areLocaleNamespacesLoaded,
  ensureLocaleNamespaces,
  normalizeLocaleNamespaces,
  normalizeSupportedLocale,
  syncDocumentTitle,
  type AppLocale,
  type LocaleNamespace
} from "@/i18n"

type SwitchAppLocaleOptions = {
  namespaces: readonly LocaleNamespace[]
  persistLocal?: boolean
  showSuccess?: boolean
  syncRemote?: (locale: AppLocale) => Promise<string | null | undefined | void>
}

let switchVersion = 0

export async function switchAppLocale(raw: string, options: SwitchAppLocaleOptions): Promise<AppLocale> {
  const version = ++switchVersion
  const namespaces = normalizeLocaleNamespaces(options.namespaces)
  const previousLocale = normalizeSupportedLocale(i18n.global.locale.value)
  const nextLocale = normalizeSupportedLocale(raw)
  const previousStoredLocale = localStorage.getItem("locale")

  if (nextLocale === previousLocale && areLocaleNamespacesLoaded(nextLocale, namespaces)) {
    return nextLocale
  }

  if (options.persistLocal !== false) {
    localStorage.setItem("locale", nextLocale)
  }

  const messageBoxOpened = !areLocaleNamespacesLoaded(nextLocale, namespaces)
  if (messageBoxOpened) {
    void ElMessageBox.alert(
      i18n.global.t("localeSwitch.message"),
      i18n.global.t("localeSwitch.title"),
      {
        confirmButtonText: i18n.global.t("localeSwitch.background"),
        showClose: true,
        closeOnClickModal: false,
        closeOnPressEscape: true,
        type: "info"
      }
    ).catch(() => undefined)
  }

  try {
    await ensureLocaleNamespaces(nextLocale, namespaces)
    if (version !== switchVersion) return nextLocale
    i18n.global.locale.value = nextLocale
    syncDocumentTitle()
    const remoteLocale = await options.syncRemote?.(nextLocale)
    const syncedLocale = remoteLocale ? normalizeSupportedLocale(remoteLocale) : nextLocale
    if (syncedLocale !== nextLocale) {
      await ensureLocaleNamespaces(syncedLocale, namespaces)
      if (version !== switchVersion) return syncedLocale
      i18n.global.locale.value = syncedLocale
      if (options.persistLocal !== false) localStorage.setItem("locale", syncedLocale)
      syncDocumentTitle()
    }
    if (messageBoxOpened) ElMessageBox.close()
    if (options.showSuccess !== false) {
      ElMessage.success(i18n.global.t("localeSwitch.success"))
    }
    return syncedLocale
  } catch (error) {
    if (version === switchVersion) {
      if (options.persistLocal !== false) {
        if (previousStoredLocale) localStorage.setItem("locale", previousStoredLocale)
        else localStorage.removeItem("locale")
      }
      i18n.global.locale.value = previousLocale
      syncDocumentTitle()
      if (messageBoxOpened) ElMessageBox.close()
      ElMessage.error(i18n.global.t("localeSwitch.failed"))
    }
    throw error
  }
}
