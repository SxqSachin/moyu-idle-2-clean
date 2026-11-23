import { fishMutationIds, type FishMutationSource } from "@services/fish-mutations"
import { getRarityDefinition, listRarityDefinitions } from "@services/item-definition"
import type { FishingSpecies } from "@services/fishing"

export type FishFilters = {
  search: string
  species: string
  rarity: string
  mutation: string
  health: "all" | "healthy" | "attention"
  sort: "recent" | "rarity" | "weight" | "length" | "health" | "name"
  direction: "asc" | "desc"
}

export function defaultFishFilters(sort: FishFilters["sort"] = "rarity"): FishFilters {
  return { search: "", species: "", rarity: "", mutation: "", health: "all", sort, direction: "desc" }
}

export function fishFilterRarities(): string[] {
  return listRarityDefinitions().slice().sort((a, b) => a.order - b.order).map((row) => row.id)
}

type FilterFish = FishMutationSource & { speciesId: string; health?: number; weightKg: number; lengthCm: number }
export function fishHealthPercent(fish: Pick<FilterFish, "health">): number {
  return Math.max(0, Math.min(100, Math.round((fish.health || 0) * 100)))
}

export function filterAndSortFish<T>(rows: readonly T[], filters: FishFilters, context: {
  fish: (row: T) => FilterFish
  name: (row: T) => string
  speciesName: (id: string) => string
  speciesById: ReadonlyMap<string, FishingSpecies>
  instanceId: (row: T) => number
  locale: string
}): T[] {
  const search = filters.search.trim().toLocaleLowerCase(context.locale)
  const rarity = (fish: FilterFish) => context.speciesById.get(fish.speciesId)?.rarity || "common"
  const filtered = rows.filter((row) => {
    const fish = context.fish(row)
    if (filters.species && fish.speciesId !== filters.species) return false
    if (filters.rarity && rarity(fish) !== filters.rarity) return false
    if (filters.health === "healthy" && fishHealthPercent(fish) < 80) return false
    if (filters.health === "attention" && fishHealthPercent(fish) >= 80) return false
    if (filters.mutation) {
      const ids = fishMutationIds(fish)
      if (filters.mutation === "any" ? ids.length === 0 : filters.mutation === "none" ? ids.length > 0 : !ids.includes(filters.mutation)) return false
    }
    return !search || `${context.name(row)} ${context.speciesName(fish.speciesId)}`.toLocaleLowerCase(context.locale).includes(search)
  })
  // The complete inventory list already carries the server's newest-first order.
  if (filters.sort === "recent") return filters.direction === "desc" ? filtered : filtered.reverse()
  return filtered.sort((left, right) => {
    const a = context.fish(left), b = context.fish(right)
    let compared = 0
    if (filters.sort === "rarity") compared = (getRarityDefinition(rarity(a))?.order || 0) - (getRarityDefinition(rarity(b))?.order || 0)
    if (filters.sort === "weight") compared = a.weightKg - b.weightKg
    if (filters.sort === "length") compared = a.lengthCm - b.lengthCm
    if (filters.sort === "health") compared = fishHealthPercent(a) - fishHealthPercent(b)
    if (filters.sort === "name") compared = context.name(left).localeCompare(context.name(right), context.locale)
    if (!compared) compared = context.instanceId(left) - context.instanceId(right)
    return compared * (filters.direction === "desc" ? -1 : 1)
  })
}
