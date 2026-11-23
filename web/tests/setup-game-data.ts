import { installGameDataForTest } from "@services/game-data-runtime"

const modules = import.meta.glob("@game-data/chunks/*.json", { eager: true }) as Record<string, { default: unknown }>
const chunks: Record<string, unknown> = {}

for (const [path, module] of Object.entries(modules)) {
  const match = path.match(/chunks\/([^/]+)\.json$/)
  if (!match) continue
  chunks[match[1]] = module.default
}

installGameDataForTest(chunks)
