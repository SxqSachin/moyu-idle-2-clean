import { beforeEach, describe, expect, it, vi } from "vitest"

const elementPlusMock = vi.hoisted(() => ({
  alert: vi.fn(() => Promise.resolve()),
  close: vi.fn(),
  success: vi.fn(),
  error: vi.fn()
}))

vi.mock("element-plus", () => ({
  ElMessage: {
    success: elementPlusMock.success,
    error: elementPlusMock.error
  },
  ElMessageBox: {
    alert: elementPlusMock.alert,
    close: elementPlusMock.close
  }
}))

type MockI18n = {
  currentLocale: { value: string }
  loaded: Set<string>
  ensureLocaleNamespaces: ReturnType<typeof vi.fn>
  syncDocumentTitle: ReturnType<typeof vi.fn>
}

function namespaceKey(locale: string, namespace: string) {
  return `${locale}:${namespace}`
}

async function loadSwitchAppLocale(mock: MockI18n) {
  vi.doMock("@/i18n", () => ({
    default: {
      global: {
        locale: mock.currentLocale,
        t: (key: string) => key
      }
    },
    areLocaleNamespacesLoaded: (locale: string, namespaces: readonly string[]) =>
      namespaces.every((namespace) => mock.loaded.has(namespaceKey(locale, namespace))),
    ensureLocaleNamespaces: mock.ensureLocaleNamespaces,
    normalizeLocaleNamespaces: (namespaces: readonly string[]) => [...namespaces],
    normalizeSupportedLocale: (locale: string | null | undefined) => locale || "en-US",
    syncDocumentTitle: mock.syncDocumentTitle
  }))

  return import("../src/services/locale-switch")
}

describe("switchAppLocale", () => {
  beforeEach(() => {
    vi.resetModules()
    vi.restoreAllMocks()
    localStorage.clear()
    elementPlusMock.alert.mockClear()
    elementPlusMock.close.mockClear()
    elementPlusMock.success.mockClear()
    elementPlusMock.error.mockClear()
  })

  it("shows a background-download message box and switches after namespaces load", async () => {
    localStorage.setItem("locale", "zh-CN")
    const mock: MockI18n = {
      currentLocale: { value: "zh-CN" },
      loaded: new Set([namespaceKey("zh-CN", "core")]),
      ensureLocaleNamespaces: vi.fn(async (locale: string, namespaces: readonly string[]) => {
        namespaces.forEach((namespace) => mock.loaded.add(namespaceKey(locale, namespace)))
      }),
      syncDocumentTitle: vi.fn()
    }
    const { switchAppLocale } = await loadSwitchAppLocale(mock)

    await expect(switchAppLocale("en-US", { namespaces: ["core", "home"] })).resolves.toBe("en-US")

    expect(elementPlusMock.alert).toHaveBeenCalledWith("localeSwitch.message", "localeSwitch.title", expect.objectContaining({
      confirmButtonText: "localeSwitch.background"
    }))
    expect(mock.currentLocale.value).toBe("en-US")
    expect(localStorage.getItem("locale")).toBe("en-US")
    expect(elementPlusMock.close).toHaveBeenCalledTimes(1)
    expect(elementPlusMock.success).toHaveBeenCalledWith("localeSwitch.success")
    expect(mock.syncDocumentTitle).toHaveBeenCalledTimes(1)
  })

  it("rolls back the stored locale when the latest download fails", async () => {
    localStorage.setItem("locale", "zh-CN")
    const mock: MockI18n = {
      currentLocale: { value: "zh-CN" },
      loaded: new Set([namespaceKey("zh-CN", "core")]),
      ensureLocaleNamespaces: vi.fn(async () => {
        throw new Error("download failed")
      }),
      syncDocumentTitle: vi.fn()
    }
    const { switchAppLocale } = await loadSwitchAppLocale(mock)

    await expect(switchAppLocale("ja", { namespaces: ["core", "home"] })).rejects.toThrow("download failed")

    expect(mock.currentLocale.value).toBe("zh-CN")
    expect(localStorage.getItem("locale")).toBe("zh-CN")
    expect(elementPlusMock.close).toHaveBeenCalledTimes(1)
    expect(elementPlusMock.error).toHaveBeenCalledWith("localeSwitch.failed")
  })

  it("rolls back local state when remote locale sync fails", async () => {
    localStorage.setItem("locale", "zh-CN")
    const mock: MockI18n = {
      currentLocale: { value: "zh-CN" },
      loaded: new Set([namespaceKey("zh-CN", "core")]),
      ensureLocaleNamespaces: vi.fn(async (locale: string, namespaces: readonly string[]) => {
        namespaces.forEach((namespace) => mock.loaded.add(namespaceKey(locale, namespace)))
      }),
      syncDocumentTitle: vi.fn()
    }
    const { switchAppLocale } = await loadSwitchAppLocale(mock)

    await expect(switchAppLocale("en-US", {
      namespaces: ["core", "home"],
      syncRemote: async () => {
        throw new Error("remote failed")
      }
    })).rejects.toThrow("remote failed")

    expect(mock.currentLocale.value).toBe("zh-CN")
    expect(localStorage.getItem("locale")).toBe("zh-CN")
    expect(elementPlusMock.error).toHaveBeenCalledWith("localeSwitch.failed")
  })

  it("keeps the last selection when an earlier pending download fails later", async () => {
    localStorage.setItem("locale", "zh-CN")
    let rejectFirstDownload: ((error: Error) => void) | undefined
    const mock: MockI18n = {
      currentLocale: { value: "zh-CN" },
      loaded: new Set([namespaceKey("zh-CN", "core")]),
      ensureLocaleNamespaces: vi.fn((locale: string, namespaces: readonly string[]) => {
        if (locale === "ja") {
          return new Promise<void>((_, reject) => {
            rejectFirstDownload = reject
          })
        }
        namespaces.forEach((namespace) => mock.loaded.add(namespaceKey(locale, namespace)))
        return Promise.resolve()
      }),
      syncDocumentTitle: vi.fn()
    }
    const { switchAppLocale } = await loadSwitchAppLocale(mock)

    const firstSwitch = switchAppLocale("ja", { namespaces: ["core", "home"] }).catch((error) => error)
    await expect(switchAppLocale("en-US", { namespaces: ["core", "home"] })).resolves.toBe("en-US")
    rejectFirstDownload?.(new Error("old download failed"))
    await expect(firstSwitch).resolves.toBeInstanceOf(Error)

    expect(mock.currentLocale.value).toBe("en-US")
    expect(localStorage.getItem("locale")).toBe("en-US")
    expect(elementPlusMock.error).not.toHaveBeenCalled()
  })
})
