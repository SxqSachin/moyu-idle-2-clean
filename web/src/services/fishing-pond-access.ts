export const FISHING_POND_CAPABILITY = {
  inspectFish: "fish.inspect",
  inspectVitals: "fish.inspect_vitals",
  trackFish: "fish.track",
  throwStone: "water.throw_stone",
  skipStone: "water.skip_stone",
  feed: "pond.feed"
} as const

export type FishingPondCapability = typeof FISHING_POND_CAPABILITY[keyof typeof FISHING_POND_CAPABILITY]

export const OWNER_FISHING_POND_CAPABILITIES: readonly FishingPondCapability[] = Object.freeze([
  FISHING_POND_CAPABILITY.inspectFish,
  FISHING_POND_CAPABILITY.inspectVitals,
  FISHING_POND_CAPABILITY.trackFish,
  FISHING_POND_CAPABILITY.throwStone,
  FISHING_POND_CAPABILITY.skipStone,
  FISHING_POND_CAPABILITY.feed
])

const READ_ONLY_FISHING_POND_CAPABILITIES = new Set<FishingPondCapability>([
  FISHING_POND_CAPABILITY.inspectFish,
  FISHING_POND_CAPABILITY.trackFish,
  FISHING_POND_CAPABILITY.throwStone,
  FISHING_POND_CAPABILITY.skipStone
])

// Public responses are treated as requests, not authority. New capabilities
// remain unavailable until the visitor client explicitly classifies them safe.
export function restrictFishingPondCapabilitiesToReadOnly(capabilities: readonly string[]): FishingPondCapability[] {
  const allowed = new Set<FishingPondCapability>()
  for (const capability of capabilities) {
    if (READ_ONLY_FISHING_POND_CAPABILITIES.has(capability as FishingPondCapability)) {
      allowed.add(capability as FishingPondCapability)
    }
  }
  return OWNER_FISHING_POND_CAPABILITIES.filter((capability) => allowed.has(capability))
}
