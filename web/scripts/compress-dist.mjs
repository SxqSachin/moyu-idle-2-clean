import { brotliCompress, gzip } from "node:zlib"
import { promisify } from "node:util"
import { readdir, readFile, writeFile } from "node:fs/promises"
import { join } from "node:path"
import { constants } from "node:zlib"

const brotliCompressAsync = promisify(brotliCompress)
const gzipAsync = promisify(gzip)
const distDir = new URL("../dist", import.meta.url)
const compressibleExts = new Set([
  ".css",
  ".html",
  ".js",
  ".json",
  ".map",
  ".svg",
  ".txt",
  ".webmanifest",
  ".xml"
])

let brotliCount = 0
let brotliBytes = 0
let gzipCount = 0
let gzipBytes = 0

await compressDirectory(distDir.pathname)

console.log(
  `[compress-dist] wrote ${brotliCount} brotli files (${formatBytes(brotliBytes)}) and ${gzipCount} gzip files (${formatBytes(gzipBytes)})`
)

async function compressDirectory(dir) {
  const entries = await readdir(dir, { withFileTypes: true })
  await Promise.all(
    entries.map(async (entry) => {
      const path = join(dir, entry.name)
      if (entry.isDirectory()) {
        await compressDirectory(path)
        return
      }
      if (!entry.isFile() || entry.name.endsWith(".br")) return
      if (!compressibleExts.has(getExt(entry.name))) return
      await compressFile(path)
    })
  )
}

async function compressFile(path) {
  const source = await readFile(path)
  const [brotliBuffer, gzipBuffer] = await Promise.all([
    brotliCompressAsync(source, {
      params: {
        [constants.BROTLI_PARAM_QUALITY]: 11
      }
    }),
    gzipAsync(source, { level: 9 })
  ])

  await Promise.all([
    writeFile(`${path}.br`, brotliBuffer),
    writeFile(`${path}.gz`, gzipBuffer)
  ])
  brotliCount += 1
  brotliBytes += brotliBuffer.length
  gzipCount += 1
  gzipBytes += gzipBuffer.length
}

function getExt(filename) {
  const basename = filename.toLowerCase()
  const index = basename.lastIndexOf(".")
  return index >= 0 ? basename.slice(index) : ""
}

function formatBytes(bytes) {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KiB`
  return `${(bytes / 1024 / 1024).toFixed(2)} MiB`
}
