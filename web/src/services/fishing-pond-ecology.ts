import type { FishingPondFish } from "@services/fishing"

export type FishingPondPredationRisk = {
  predator: FishingPondFish
  prey: FishingPondFish
  active: boolean
}

export function canFishingPondFishPredate(predator: FishingPondFish, prey: FishingPondFish): boolean {
  if (predator.instanceId === prey.instanceId || predator.fish.lifeState === "dead" || prey.fish.lifeState === "dead") return false
  const config = predator.predation
  if (!config || config.maxPreyLengthRatio <= 0 || predator.fish.lengthCm <= 0 || prey.fish.lengthCm <= 0) return false
  return prey.fish.lengthCm <= predator.fish.lengthCm * config.maxPreyLengthRatio + 0.0001
}

export function fishingPondPredationRisks(fish: readonly FishingPondFish[]): FishingPondPredationRisk[] {
  const unique = new Map<number, FishingPondFish>()
  for (const row of fish) unique.set(row.instanceId, row)
  const rows = Array.from(unique.values())
  const risks: FishingPondPredationRisk[] = []
  for (const predator of rows) {
    if (!predator.predation) continue
    for (const prey of rows) {
      if (!canFishingPondFishPredate(predator, prey)) continue
      risks.push({
        predator,
        prey,
        active: Number(predator.fish.satiety ?? 100) < predator.predation.huntBelowSatiety
      })
    }
  }
  return risks.sort((left, right) => (
    left.predator.instanceId - right.predator.instanceId
    || left.prey.fish.lengthCm - right.prey.fish.lengthCm
    || left.prey.instanceId - right.prey.instanceId
  ))
}

export function mergeFishingPondEcosystem(...groups: ReadonlyArray<readonly FishingPondFish[]>): FishingPondFish[] {
  const fish = new Map<number, FishingPondFish>()
  for (const group of groups) for (const row of group) fish.set(row.instanceId, row)
  return Array.from(fish.values())
}
