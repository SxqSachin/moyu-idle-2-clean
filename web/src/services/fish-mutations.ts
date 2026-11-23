import type { FishingMutationDefinition } from "@services/fishing"

export type FishMutationSource = {
  genes?: { mutations?: string[] } | null
  mutations?: string[] | null
}

export type ResolvedFishMutation = {
  id: string
  nameKey: string
  descriptionKey: string
  visualEffect: string
  badgeIcon: string
  definition?: FishingMutationDefinition
}

function normalizeToken(value: unknown): string {
  return String(value || "")
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9_-]+/g, "-")
    .replace(/^-+|-+$/g, "")
}

export function fishMutationIds(fish: FishMutationSource | null | undefined): string[] {
  const raw = fish?.genes?.mutations?.length ? fish.genes.mutations : fish?.mutations
  const seen = new Set<string>()
  const ids: string[] = []
  for (const value of raw || []) {
    const id = normalizeToken(value)
    if (!id || seen.has(id)) continue
    seen.add(id)
    ids.push(id)
  }
  return ids
}

export function resolveFishMutations(
  fish: FishMutationSource | null | undefined,
  definitions: readonly FishingMutationDefinition[] | null | undefined
): ResolvedFishMutation[] {
  const byId = new Map((definitions || []).map((definition) => [normalizeToken(definition.id), definition]))
  return fishMutationIds(fish).map((id) => {
    const definition = byId.get(id)
    return {
      id,
      nameKey: String(definition?.nameKey || `game.fishing.mutation_${id}`),
      descriptionKey: String(definition?.descriptionKey || `game.fishing.mutation_${id}Desc`),
      visualEffect: normalizeToken(definition?.visualEffect || id) || "unknown",
      badgeIcon: String(definition?.badgeIcon || "lucide:sparkles").trim() || "lucide:sparkles",
      definition
    }
  })
}

export function fishMutationClassNames(
  fish: FishMutationSource | null | undefined,
  definitions?: readonly FishingMutationDefinition[] | null
): string[] {
  const mutations = resolveFishMutations(fish, definitions)
  if (!mutations.length) return []
  return [
    "has-fish-mutation",
    ...(mutations.length > 1 ? ["has-stacked-fish-mutations"] : []),
    ...mutations.map((mutation) => `fish-mutation--${mutation.visualEffect}`)
  ]
}
