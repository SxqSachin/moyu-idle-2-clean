import type { DeviceProfile } from "@/types/auth"

const localDeviceStorageKey = "risk.local_device_id"
const browserInstallStorageKey = "risk.browser_install_id"
const cookieStorageKey = "risk.cookie_id"
const sessionChainStorageKey = "risk.session_chain_id"
const indexedDbName = "risk-device-profile"
const indexedDbStore = "kv"

let cachedProfilePromise: Promise<DeviceProfile> | null = null

type NavigatorWithClientHints = Navigator & {
  deviceMemory?: number
  userAgentData?: {
    platform?: string
    brands?: Array<{ brand: string; version: string }>
  }
}

export async function collectDeviceProfile(): Promise<DeviceProfile> {
  if (!cachedProfilePromise) {
    cachedProfilePromise = buildDeviceProfile().catch(() => buildEmptyDeviceProfile())
  }
  return cachedProfilePromise
}

function buildEmptyDeviceProfile(): DeviceProfile {
  return {
    fingerprintId: "",
    fingerprintVersion: "custom-v1",
    componentsHash: "",
    deviceFamilyHash: "",
    browserFamilyHash: "",
    runtimeFamilyHash: "",
    cookieId: "",
    localDeviceId: "",
    browserInstallId: "",
    sessionChainId: "",
    timezone: "",
    language: "",
    screenBucket: "",
    canvasHash: "",
    webglHash: "",
    audioHash: "",
    platformHint: "",
    cpuHint: "",
    memoryHint: "",
    captchaScore: 0
  }
}

async function buildDeviceProfile(): Promise<DeviceProfile> {
  const timezone = Intl.DateTimeFormat().resolvedOptions().timeZone || ""
  const language = navigator.language || ""
  const screenBucket = resolveScreenBucket()
  const nav = navigator as NavigatorWithClientHints
  const platformHint = nav.userAgentData?.platform || navigator.platform || ""
  const cpuHint = typeof navigator.hardwareConcurrency === "number" ? String(navigator.hardwareConcurrency) : ""
  const memoryHint = typeof nav.deviceMemory === "number" ? String(nav.deviceMemory) : ""
  const webglHash = await hashHex(await collectWebGLFingerprint())
  const canvasHash = await hashHex(collectCanvasFingerprint())
  const audioHash = await hashHex(await collectAudioFingerprint())
  const browserFamilyRaw = [
    platformHint,
    nav.userAgentData?.brands?.map((row) => `${row.brand}:${row.version}`).join("|") || navigator.userAgent,
    language
  ].join("|")
  const runtimeFamilyRaw = [navigator.vendor || "", timezone, cpuHint, memoryHint].join("|")
  const deviceFamilyRaw = [platformHint, cpuHint, memoryHint, screenBucket, timezone, language, webglHash, canvasHash, audioHash].join("|")
  const cookieId = ensureCookieID()
  const localDeviceId = await ensurePersistentID(localDeviceStorageKey, "local-device")
  const browserInstallId = await ensurePersistentID(browserInstallStorageKey, "browser-install")
  const sessionChainId = ensureSessionID()
  const browserFamilyHash = await hashHex(browserFamilyRaw)
  const runtimeFamilyHash = await hashHex(runtimeFamilyRaw)
  const deviceFamilyHash = await hashHex(deviceFamilyRaw)
  const componentsHash = await hashHex([
    browserFamilyHash,
    runtimeFamilyHash,
    deviceFamilyHash,
    cookieId,
    localDeviceId,
    browserInstallId,
    sessionChainId
  ].join("|"))
  return {
    fingerprintId: componentsHash,
    fingerprintVersion: "custom-v1",
    componentsHash,
    deviceFamilyHash,
    browserFamilyHash,
    runtimeFamilyHash,
    cookieId,
    localDeviceId,
    browserInstallId,
    sessionChainId,
    timezone,
    language,
    screenBucket,
    canvasHash,
    webglHash,
    audioHash,
    platformHint,
    cpuHint,
    memoryHint,
    captchaScore: 0
  }
}

function resolveScreenBucket(): string {
  const width = typeof window.screen?.width === "number" ? window.screen.width : 0
  const height = typeof window.screen?.height === "number" ? window.screen.height : 0
  const ratio = typeof window.devicePixelRatio === "number" ? window.devicePixelRatio : 1
  return `${width}x${height}@${ratio}`
}

function collectCanvasFingerprint(): string {
  const canvas = document.createElement("canvas")
  canvas.width = 240
  canvas.height = 60
  const ctx = canvas.getContext("2d")
  if (!ctx) return ""
  ctx.textBaseline = "top"
  ctx.font = "16px serif"
  ctx.fillStyle = "#0f172a"
  ctx.fillRect(0, 0, 240, 60)
  ctx.fillStyle = "#f8fafc"
  ctx.fillText("MySecondWebGame|risk", 12, 18)
  return canvas.toDataURL()
}

async function collectWebGLFingerprint(): Promise<string> {
  const canvas = document.createElement("canvas")
  const gl = canvas.getContext("webgl")
  if (!gl) return ""
  const webgl = gl as WebGLRenderingContext
  const debug = webgl.getExtension("WEBGL_debug_renderer_info")
  const vendor = debug ? String(webgl.getParameter(debug.UNMASKED_VENDOR_WEBGL) || "") : ""
  const renderer = debug ? String(webgl.getParameter(debug.UNMASKED_RENDERER_WEBGL) || "") : ""
  return `${vendor}|${renderer}|${webgl.getParameter(webgl.VERSION) || ""}`
}

async function collectAudioFingerprint(): Promise<string> {
  try {
    const AudioCtx = window.OfflineAudioContext || (window as unknown as { webkitOfflineAudioContext?: typeof OfflineAudioContext }).webkitOfflineAudioContext
    if (!AudioCtx) return ""
    const ctx = new AudioCtx(1, 44100, 44100)
    const oscillator = ctx.createOscillator()
    oscillator.type = "triangle"
    oscillator.frequency.value = 1000
    const compressor = ctx.createDynamicsCompressor()
    oscillator.connect(compressor)
    compressor.connect(ctx.destination)
    oscillator.start(0)
    const buffer = await ctx.startRendering()
    const data = buffer.getChannelData(0)
    let sum = 0
    for (let i = 0; i < Math.min(64, data.length); i += 1) {
      sum += Math.abs(data[i] || 0)
    }
    return String(sum)
  } catch {
    return ""
  }
}

function ensureCookieID(): string {
  const existing = readCookie(cookieStorageKey)
  if (existing) return existing
  const value = crypto.randomUUID()
  document.cookie = `${cookieStorageKey}=${encodeURIComponent(value)}; path=/; max-age=${60 * 60 * 24 * 365 * 2}; samesite=lax`
  return value
}

function readCookie(name: string): string {
  const prefix = `${name}=`
  const parts = document.cookie.split(";")
  for (const part of parts) {
    const trimmed = part.trim()
    if (trimmed.startsWith(prefix)) {
      return decodeURIComponent(trimmed.slice(prefix.length))
    }
  }
  return ""
}

function ensureSessionID(): string {
  const existing = sessionStorage.getItem(sessionChainStorageKey)
  if (existing) return existing
  const value = crypto.randomUUID()
  sessionStorage.setItem(sessionChainStorageKey, value)
  return value
}

async function ensurePersistentID(storageKey: string, prefix: string): Promise<string> {
  const existing = localStorage.getItem(storageKey)
  if (existing) {
    await writeIndexedDB(storageKey, existing)
    return existing
  }
  const fromDb = await readIndexedDB(storageKey)
  if (fromDb) {
    localStorage.setItem(storageKey, fromDb)
    return fromDb
  }
  const value = `${prefix}-${crypto.randomUUID()}`
  localStorage.setItem(storageKey, value)
  await writeIndexedDB(storageKey, value)
  return value
}

function openIndexedDB(): Promise<IDBDatabase | null> {
  return new Promise((resolve) => {
    if (!("indexedDB" in window)) {
      resolve(null)
      return
    }
    const request = window.indexedDB.open(indexedDbName, 1)
    request.onupgradeneeded = () => {
      const db = request.result
      if (!db.objectStoreNames.contains(indexedDbStore)) {
        db.createObjectStore(indexedDbStore)
      }
    }
    request.onsuccess = () => resolve(request.result)
    request.onerror = () => resolve(null)
  })
}

async function readIndexedDB(key: string): Promise<string> {
  const db = await openIndexedDB()
  if (!db) return ""
  return new Promise((resolve) => {
    const tx = db.transaction(indexedDbStore, "readonly")
    const store = tx.objectStore(indexedDbStore)
    const request = store.get(key)
    request.onsuccess = () => resolve(typeof request.result === "string" ? request.result : "")
    request.onerror = () => resolve("")
  })
}

async function writeIndexedDB(key: string, value: string): Promise<void> {
  const db = await openIndexedDB()
  if (!db) return
  await new Promise<void>((resolve) => {
    const tx = db.transaction(indexedDbStore, "readwrite")
    const store = tx.objectStore(indexedDbStore)
    store.put(value, key)
    tx.oncomplete = () => resolve()
    tx.onerror = () => resolve()
    tx.onabort = () => resolve()
  })
}

async function hashHex(value: string): Promise<string> {
  if (!value) return ""
  const bytes = new TextEncoder().encode(value)
  const digest = await crypto.subtle.digest("SHA-256", bytes)
  return Array.from(new Uint8Array(digest)).map((b) => b.toString(16).padStart(2, "0")).join("")
}
