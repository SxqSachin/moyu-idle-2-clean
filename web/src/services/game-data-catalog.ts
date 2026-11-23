import type { Catalog } from "@game-data/game-data"
import { getGameDataCatalogSnapshot } from "@services/game-data-runtime"

let catalogPromise: Promise<Catalog> | null = null

export async function getGameDataCatalog(): Promise<Catalog> {
  if (!catalogPromise) {
    catalogPromise = Promise.resolve(getGameDataCatalogSnapshot<Catalog>())
  }
  return catalogPromise
}
