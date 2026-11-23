import { TABLES } from "./schemaRegistry"

const ROOT_DIR_NAME = "game-data"

function sortObjectDeep(value) {
  if (Array.isArray(value)) return value.map((item) => sortObjectDeep(item))
  if (!value || typeof value !== "object") return value
  const out = {}
  for (const key of Object.keys(value).sort((a, b) => a.localeCompare(b))) {
    out[key] = sortObjectDeep(value[key])
  }
  return out
}

async function readJsonFile(fileHandle) {
  const file = await fileHandle.getFile()
  const text = await file.text()
  return JSON.parse(text)
}

export async function requestGameDataDir() {
  if (typeof window.showDirectoryPicker !== "function") {
    throw new Error("当前浏览器不支持 File System Access API，无法使用该工具。")
  }
  const handle = await window.showDirectoryPicker({ mode: "readwrite" })
  if (handle.name !== ROOT_DIR_NAME) {
    throw new Error("请选择 packages/game-data 目录。")
  }
  return handle
}

async function tryReadTableFromSourceJson(gameDataDirHandle, table) {
  try {
    const sourceDir = await gameDataDirHandle.getDirectoryHandle("source")
    const dirHandle = await sourceDir.getDirectoryHandle("json")
    const fileHandle = await dirHandle.getFileHandle(`${table}.json`)
    const parsed = await readJsonFile(fileHandle)
    if (Array.isArray(parsed)) return parsed
    if (parsed && Array.isArray(parsed[table])) return parsed[table]
    return []
  } catch {
    return []
  }
}

async function readChunkBootstrap(gameDataDirHandle, table) {
  try {
    const generated = await gameDataDirHandle.getDirectoryHandle("generated")
    const chunks = await generated.getDirectoryHandle("chunks")
    if (table === "items") {
      const coreFile = await chunks.getFileHandle("core.json")
      const coreParsed = await readJsonFile(coreFile)
      return Array.isArray(coreParsed?.items) ? coreParsed.items : []
    }
    const chunkFile = await chunks.getFileHandle(`${table}.json`)
    const parsed = await readJsonFile(chunkFile)
    return Array.isArray(parsed?.[table]) ? parsed[table] : []
  } catch {
    return []
  }
}

export async function loadCatalog(gameDataDirHandle) {
  const catalog = {}
  for (const table of TABLES) {
    const fromChunk = await readChunkBootstrap(gameDataDirHandle, table)
    catalog[table] = fromChunk.length > 0 ? fromChunk : await tryReadTableFromSourceJson(gameDataDirHandle, table)
  }
  return catalog
}

export async function saveCatalog(gameDataDirHandle, catalog) {
  const sourceDir = await gameDataDirHandle.getDirectoryHandle("source")
  const dirHandle = await sourceDir.getDirectoryHandle("json", { create: true })
  for (const table of TABLES) {
    const rows = Array.isArray(catalog[table]) ? catalog[table] : []
    const payload = {
      version: 1,
      [table]: sortObjectDeep(rows)
    }
    const fileHandle = await dirHandle.getFileHandle(`${table}.json`, { create: true })
    const writable = await fileHandle.createWritable()
    await writable.write(`${JSON.stringify(payload, null, 2)}\n`)
    await writable.close()
  }
}
