import type { TutorialConfig } from "./types"

export type TutorialGuideGroupId =
  | "recommended"
  | "actions"
  | "warehouse"
  | "character"
  | "village"
  | "trade"
  | "combat"
  | "growth"
  | "other"

export type TutorialGuideGroup = {
  id: TutorialGuideGroupId
  titleKey: string
  order: number
}

export type TutorialGuidePlacement = {
  groupId: TutorialGuideGroupId
  order: number
}

export const tutorialGuideGroups: TutorialGuideGroup[] = [
  { id: "recommended", titleKey: "tutorial.guideGroup.recommended", order: 0 },
  { id: "actions", titleKey: "tutorial.guideGroup.actions", order: 1 },
  { id: "warehouse", titleKey: "tutorial.guideGroup.warehouse", order: 2 },
  { id: "character", titleKey: "tutorial.guideGroup.character", order: 3 },
  { id: "village", titleKey: "tutorial.guideGroup.village", order: 4 },
  { id: "trade", titleKey: "tutorial.guideGroup.trade", order: 5 },
  { id: "combat", titleKey: "tutorial.guideGroup.combat", order: 6 },
  { id: "growth", titleKey: "tutorial.guideGroup.growth", order: 7 },
  { id: "other", titleKey: "tutorial.guideGroup.other", order: 99 },
]

export const recommendedTutorialIds = ["onboarding", "interactions", "actions", "home", "warehouse", "character", "tasks", "combat"]

const placementByTutorialId: Record<string, TutorialGuidePlacement> = {
  onboarding: { groupId: "recommended", order: 1 },
  interactions: { groupId: "recommended", order: 2 },

  actions: { groupId: "actions", order: 1 },
  home: { groupId: "actions", order: 2 },
  fishing: { groupId: "actions", order: 3 },
  pond: { groupId: "actions", order: 4 },
  crafting: { groupId: "actions", order: 5 },

  warehouse: { groupId: "warehouse", order: 1 },

  character: { groupId: "character", order: 1 },
  "character-attributes": { groupId: "character", order: 2 },
  "character-equipment": { groupId: "character", order: 3 },
  "character-skills": { groupId: "character", order: 4 },
  "character-battle-items": { groupId: "character", order: 5 },
  "character-loadouts": { groupId: "character", order: 6 },

  tasks: { groupId: "village", order: 1 },
  achievements: { groupId: "village", order: 2 },
  building: { groupId: "village", order: 3 },
  cattery: { groupId: "village", order: 4 },

  shop: { groupId: "trade", order: 1 },
  market: { groupId: "trade", order: 2 },
  auction: { groupId: "trade", order: 3 },

  combat: { groupId: "combat", order: 1 },
  arena: { groupId: "combat", order: 2 },
  "combat-trial": { groupId: "combat", order: 3 },

  enhance: { groupId: "growth", order: 1 },
  alchemy: { groupId: "growth", order: 2 },
  "alchemy-convert": { groupId: "growth", order: 3 },
  "alchemy-bench": { groupId: "growth", order: 4 },
  "alchemy-inherit": { groupId: "growth", order: 5 },
  "alchemy-transmute": { groupId: "growth", order: 6 },
  "alchemy-smelt": { groupId: "growth", order: 7 },
  "alchemy-skill-forge": { groupId: "growth", order: 8 },
  affix: { groupId: "growth", order: 9 },

  settings: { groupId: "other", order: 1 },
}

const groupOrderById = Object.fromEntries(tutorialGuideGroups.map((group) => [group.id, group.order])) as Record<TutorialGuideGroupId, number>

export function getTutorialGuidePlacement(configOrId: TutorialConfig | string): TutorialGuidePlacement {
  const id = typeof configOrId === "string" ? configOrId : configOrId.id
  return placementByTutorialId[id] ?? { groupId: "other", order: 99 }
}

export function compareTutorialGuideEntries(a: TutorialConfig, b: TutorialConfig): number {
  const placementA = getTutorialGuidePlacement(a)
  const placementB = getTutorialGuidePlacement(b)
  const groupOrderDelta = groupOrderById[placementA.groupId] - groupOrderById[placementB.groupId]
  if (groupOrderDelta !== 0) return groupOrderDelta
  const placementDelta = placementA.order - placementB.order
  if (placementDelta !== 0) return placementDelta
  return (a.order ?? 99) - (b.order ?? 99)
}
