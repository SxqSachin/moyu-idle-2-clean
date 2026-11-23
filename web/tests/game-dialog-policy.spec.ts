import { describe, expect, it } from "vitest"
import { readdirSync, readFileSync, statSync } from "node:fs"
import { dirname, join, relative } from "node:path"
import { fileURLToPath } from "node:url"

const webRoot = dirname(dirname(fileURLToPath(import.meta.url)))
const srcRoot = join(webRoot, "src")
const gameDialogPath = join(srcRoot, "components/game/common/GameDialog.vue")

function listVueFiles(dir: string): string[] {
  const files: string[] = []
  for (const entry of readdirSync(dir)) {
    const fullPath = join(dir, entry)
    const stat = statSync(fullPath)
    if (stat.isDirectory()) {
      files.push(...listVueFiles(fullPath))
      continue
    }
    if (entry.endsWith(".vue")) files.push(fullPath)
  }
  return files
}

describe("game dialog policy", () => {
  it("keeps direct el-dialog usage inside GameDialog only", () => {
    const offenders = listVueFiles(srcRoot)
      .filter((file) => file !== gameDialogPath)
      .filter((file) => /<\/?el-dialog\b/.test(readFileSync(file, "utf8")))
      .map((file) => relative(webRoot, file))

    expect(offenders).toEqual([])
  })

  it("forces game dialogs to teleport into the shared overlay layer", () => {
    const source = readFileSync(gameDialogPath, "utf8")

    expect(source).toContain("<el-dialog")
    expect(source).toContain("append-to-body")
    expect(source).toContain("game-dialog-overlay")
    expect(source).toContain(":modal-class=\"mergedModalClass\"")
  })
})
