import { afterEach, describe, expect, it } from "vitest"
import { getGameDataTable, installGameDataForTest } from "@services/game-data-runtime"

const generatedModules = import.meta.glob("@game-data/chunks/*.json", { eager: true }) as Record<string, { default: unknown }>

function installGeneratedChunks() {
  const chunks: Record<string, unknown> = {}
  for (const [path, module] of Object.entries(generatedModules)) {
    const match = path.match(/chunks\/([^/]+)\.json$/)
    if (!match) continue
    chunks[match[1]] = module.default
  }
  installGameDataForTest(chunks)
}

describe("game data runtime", () => {
  afterEach(() => {
    installGeneratedChunks()
  })

  it("reads tables from merged chunks by table name", () => {
    installGameDataForTest({
      misc: {
        version: 1,
        title_prefixes: [{ id: "angry", nameKey: "title_prefix.angry.name", disabled: false }],
        title_suffixes: [{ id: "cat_lord", nameKey: "title_suffix.cat_lord.name", disabled: false }]
      }
    })

    expect(getGameDataTable<{ id: string }>("title_prefixes").map((row) => row.id)).toEqual(["angry"])
    expect(getGameDataTable<{ id: string }>("title_suffixes").map((row) => row.id)).toEqual(["cat_lord"])
  })
})
