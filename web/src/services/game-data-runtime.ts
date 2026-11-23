import { computed, reactive } from "vue"

export type GameDataManifestFile = {
  url: string
  hash: string
  bytes: number
  label?: string
  contains?: string[]
}

export type GameDataManifest = {
  schemaVersion: number
  gameDataVersion: number
  buildId: string
  generatedAt: string
  loadOrder: string[]
  files: Record<string, GameDataManifestFile>
}

export type GameDataLoadPhase = "idle" | "manifest" | "download" | "parse" | "ready" | "error"

export type GameDataLoadState = {
  phase: GameDataLoadPhase
  loaded: boolean
  currentFileId: string
  currentFileLabel: string
  completedFiles: number
  totalFiles: number
  loadedBytes: number
  totalBytes: number
  fileLoadedBytes: number
  fileTotalBytes: number
  error: string
  version: number
  buildId: string
}

const MANIFEST_URL = "/game-data/manifest.json"
const DEV_BOOT_DELAY_QUERY = "gameDataBootDelayMs"
const DEV_BOOT_DELAY_STORAGE_KEY = "game-data:boot-delay-ms"
const MAX_CONCURRENT_GAME_DATA_REQUESTS = 5

const tableData = new Map<string, unknown>()
let manifestPromise: Promise<GameDataManifest> | null = null
let loadAllPromise: Promise<void> | null = null

export const gameDataLoadState = reactive<GameDataLoadState>({
  phase: "idle",
  loaded: false,
  currentFileId: "",
  currentFileLabel: "",
  completedFiles: 0,
  totalFiles: 0,
  loadedBytes: 0,
  totalBytes: 0,
  fileLoadedBytes: 0,
  fileTotalBytes: 0,
  error: "",
  version: 0,
  buildId: ""
})

export const isGameDataLoaded = computed(() => gameDataLoadState.loaded)

export async function loadGameDataManifest(): Promise<GameDataManifest> {
  if (!manifestPromise) {
    manifestPromise = fetch(MANIFEST_URL, { cache: "no-cache" })
      .then(async (response) => {
        if (!response.ok) {
          throw new Error(`manifest ${response.status}`)
        }
        return await response.json() as GameDataManifest
      })
      .catch((error) => {
        manifestPromise = null
        throw error
      })
  }
  return manifestPromise
}

export async function loadAllGameData(): Promise<void> {
  if (gameDataLoadState.loaded) return
  if (loadAllPromise) return loadAllPromise
  loadAllPromise = runLoadAllGameData()
  return loadAllPromise
}

export function resetGameDataRuntime() {
  manifestPromise = null
  loadAllPromise = null
  tableData.clear()
  Object.assign(gameDataLoadState, {
    phase: "idle",
    loaded: false,
    currentFileId: "",
    currentFileLabel: "",
    completedFiles: 0,
    totalFiles: 0,
    loadedBytes: 0,
    totalBytes: 0,
    fileLoadedBytes: 0,
    fileTotalBytes: 0,
    error: "",
    version: 0,
    buildId: ""
  })
}

export function installGameDataForTest(chunks: Record<string, unknown>) {
  manifestPromise = null
  loadAllPromise = null
  tableData.clear()
  for (const [fileId, chunk] of Object.entries(chunks)) {
    tableData.set(fileId, chunk)
  }
  Object.assign(gameDataLoadState, {
    phase: "ready",
    loaded: true,
    currentFileId: "",
    currentFileLabel: "",
    completedFiles: Object.keys(chunks).length,
    totalFiles: Object.keys(chunks).length,
    loadedBytes: 0,
    totalBytes: 0,
    fileLoadedBytes: 0,
    fileTotalBytes: 0,
    error: "",
    version: Number((chunks.core as { version?: unknown } | undefined)?.version || 0),
    buildId: "test"
  })
}

export function getGameDataChunk<T = unknown>(fileId: string): T {
  if (!gameDataLoadState.loaded) {
    throw new Error(`game data is not loaded before reading ${fileId}`)
  }
  if (!tableData.has(fileId)) {
    throw new Error(`game data file is missing: ${fileId}`)
  }
  return tableData.get(fileId) as T
}

export function getGameDataTable<T = unknown>(tableName: string): T[] {
  const directChunk = tableData.get(tableName)
  const directRows = directChunk && typeof directChunk === "object" && !Array.isArray(directChunk)
    ? (directChunk as Record<string, unknown>)[tableName]
    : undefined
  if (Array.isArray(directRows)) return directRows as T[]

  for (const chunk of tableData.values()) {
    if (!chunk || typeof chunk !== "object" || Array.isArray(chunk)) continue
    const rows = (chunk as Record<string, unknown>)[tableName]
    if (Array.isArray(rows)) return rows as T[]
  }

  if (!gameDataLoadState.loaded) {
    throw new Error(`game data is not loaded before reading ${tableName}`)
  }
  return []
}

export function getGameDataCatalogSnapshot<T = Record<string, unknown>>(): T {
  if (!gameDataLoadState.loaded) {
    throw new Error("game data is not loaded before reading catalog")
  }
  const catalog: Record<string, unknown> = {}
  for (const chunk of tableData.values()) {
    if (!chunk || typeof chunk !== "object" || Array.isArray(chunk)) continue
    Object.assign(catalog, chunk)
  }
  return catalog as T
}

async function runLoadAllGameData() {
  const startedAt = Date.now()
  try {
    gameDataLoadState.phase = "manifest"
    gameDataLoadState.error = ""
    const manifest = await loadGameDataManifest()
    const loadOrder = manifest.loadOrder.filter((fileId) => !!manifest.files[fileId])
    gameDataLoadState.version = Number(manifest.gameDataVersion || 0)
    gameDataLoadState.buildId = String(manifest.buildId || "")
    gameDataLoadState.totalFiles = loadOrder.length
    gameDataLoadState.completedFiles = 0
    gameDataLoadState.totalBytes = loadOrder.reduce((sum, fileId) => sum + Math.max(0, Number(manifest.files[fileId]?.bytes || 0)), 0)
    gameDataLoadState.loadedBytes = 0

    await loadGameDataFilesConcurrently(manifest, loadOrder)
    await waitForConfiguredBootDelay(startedAt)

    gameDataLoadState.phase = "ready"
    gameDataLoadState.loaded = true
    gameDataLoadState.currentFileId = ""
    gameDataLoadState.currentFileLabel = ""
  } catch (error) {
    gameDataLoadState.phase = "error"
    gameDataLoadState.loaded = false
    gameDataLoadState.error = error instanceof Error ? error.message : String(error)
    loadAllPromise = null
    throw error
  }
}

type FileProgress = {
  loaded: number
  total: number
}

async function loadGameDataFilesConcurrently(manifest: GameDataManifest, loadOrder: string[]): Promise<void> {
  const pending = [...loadOrder]
  const progressByFile = new Map<string, FileProgress>()
  for (const fileId of loadOrder) {
    const file = manifest.files[fileId]
    progressByFile.set(fileId, { loaded: 0, total: Math.max(0, Number(file.bytes || 0)) })
  }

  const workerCount = Math.max(1, Math.min(MAX_CONCURRENT_GAME_DATA_REQUESTS, pending.length))
  gameDataLoadState.phase = "download"
  gameDataLoadState.currentFileId = ""
  gameDataLoadState.currentFileLabel = `并发下载 ${workerCount} 个资源`

  async function loadNext(): Promise<void> {
    while (pending.length > 0) {
      const fileId = pending.shift()
      if (!fileId) return
      const file = manifest.files[fileId]
      gameDataLoadState.phase = "download"
      gameDataLoadState.currentFileId = fileId
      gameDataLoadState.currentFileLabel = file.label || fileId
      gameDataLoadState.fileTotalBytes = Math.max(0, Number(file.bytes || 0))
      gameDataLoadState.fileLoadedBytes = 0
      const text = await fetchTextWithProgress(file.url, file.bytes, (loaded, total) => {
        progressByFile.set(fileId, { loaded, total })
        updateAggregateProgress(progressByFile)
      })
      gameDataLoadState.phase = "parse"
      gameDataLoadState.currentFileId = fileId
      gameDataLoadState.currentFileLabel = file.label || fileId
      installDownloadedGameDataFile(fileId, JSON.parse(text), file.contains)
      const parsedBytes = Math.max(0, Number(file.bytes || text.length))
      progressByFile.set(fileId, { loaded: parsedBytes, total: parsedBytes })
      gameDataLoadState.completedFiles += 1
      updateAggregateProgress(progressByFile)
    }
  }

  await Promise.all(Array.from({ length: workerCount }, () => loadNext()))
}

function installDownloadedGameDataFile(fileId: string, payload: unknown, contains: string[] | undefined) {
  const normalizedContains = Array.isArray(contains)
    ? contains.map((entry) => String(entry || "").trim()).filter(Boolean)
    : []

  if (normalizedContains.length <= 0) {
    tableData.set(fileId, payload)
    return
  }

  if (!payload || typeof payload !== "object" || Array.isArray(payload)) {
    tableData.set(fileId, payload)
    return
  }

  const objectPayload = payload as Record<string, unknown>
  for (const chunkId of normalizedContains) {
    if (chunkId === fileId || chunkId === "core") {
      tableData.set(chunkId, objectPayload)
      continue
    }
    tableData.set(chunkId, {
      version: objectPayload.version,
      [chunkId]: objectPayload[chunkId]
    })
  }
}

function updateAggregateProgress(progressByFile: Map<string, FileProgress>) {
  let loadedBytes = 0
  for (const progress of progressByFile.values()) {
    loadedBytes += Math.max(0, Math.min(progress.loaded, progress.total || progress.loaded))
  }
  gameDataLoadState.loadedBytes = gameDataLoadState.totalBytes > 0
    ? Math.min(gameDataLoadState.totalBytes, loadedBytes)
    : loadedBytes
}

async function waitForConfiguredBootDelay(startedAt: number): Promise<void> {
  const delayMs = readConfiguredBootDelayMs()
  if (delayMs <= 0) return
  const remaining = delayMs - (Date.now() - startedAt)
  if (remaining <= 0) return
  await new Promise((resolve) => window.setTimeout(resolve, remaining))
}

function readConfiguredBootDelayMs(): number {
  if (typeof window === "undefined") return 0
  const params = new URLSearchParams(window.location.search)
  const queryValue = params.get(DEV_BOOT_DELAY_QUERY)
  if (queryValue !== null) {
    const parsed = clampBootDelayMs(Number(queryValue))
    if (parsed > 0) {
      window.localStorage.setItem(DEV_BOOT_DELAY_STORAGE_KEY, String(parsed))
    } else {
      window.localStorage.removeItem(DEV_BOOT_DELAY_STORAGE_KEY)
    }
    return parsed
  }
  return clampBootDelayMs(Number(window.localStorage.getItem(DEV_BOOT_DELAY_STORAGE_KEY) || 0))
}

function clampBootDelayMs(value: number): number {
  if (!Number.isFinite(value) || value <= 0) return 0
  return Math.min(30_000, Math.floor(value))
}

async function fetchTextWithProgress(
  url: string,
  expectedBytes: number,
  onProgress?: (loadedBytes: number, totalBytes: number) => void
): Promise<string> {
  const normalizedExpectedBytes = Math.max(0, Number(expectedBytes || 0))
  const response = await fetch(url, { cache: "force-cache" })
  if (!response.ok) {
    throw new Error(`${url} ${response.status}`)
  }
  if (!response.body) {
    const text = await response.text()
    const loadedBytes = Math.max(normalizedExpectedBytes, text.length)
    gameDataLoadState.fileLoadedBytes = loadedBytes
    onProgress?.(loadedBytes, loadedBytes)
    return text
  }

  const reader = response.body.getReader()
  const decoder = new TextDecoder()
  let received = 0
  let text = ""
  while (true) {
    const { done, value } = await reader.read()
    if (done) break
    if (!value) continue
    received += value.byteLength
    gameDataLoadState.fileLoadedBytes = received
    onProgress?.(received, normalizedExpectedBytes)
    text += decoder.decode(value, { stream: true })
  }
  text += decoder.decode()
  const loadedBytes = Math.max(received, normalizedExpectedBytes)
  gameDataLoadState.fileLoadedBytes = loadedBytes
  onProgress?.(loadedBytes, loadedBytes)
  return text
}
