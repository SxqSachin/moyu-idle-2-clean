import type { TutorialConfig } from "../types"

type TutorialLoader = () => Promise<TutorialConfig>

const loaders: Record<string, TutorialLoader> = {
  // Core
  onboarding: () => import("./onboarding-tutorial").then((m) => m.onboardingTutorial),
  interactions: () => import("./interactions-tutorial").then((m) => m.interactionsTutorial),
  character: () => import("./character-tutorial").then((m) => m.characterTutorial),
  "character-attributes": () => import("./character-attributes-tutorial").then((m) => m.characterAttributesTutorial),
  "character-equipment": () => import("./character-equipment-tutorial").then((m) => m.characterEquipmentTutorial),
  "character-skills": () => import("./character-skills-tutorial").then((m) => m.characterSkillsTutorial),
  "character-battle-items": () => import("./character-battle-items-tutorial").then((m) => m.characterBattleItemsTutorial),
  "character-loadouts": () => import("./character-loadouts-tutorial").then((m) => m.characterLoadoutsTutorial),
  actions: () => import("./actions-tutorial").then((m) => m.actionsTutorial),
  home: () => import("./home-tutorial").then((m) => m.homeTutorial),

  // Economy
  shop: () => import("./shop-tutorial").then((m) => m.shopTutorial),
  warehouse: () => import("./warehouse-tutorial").then((m) => m.warehouseTutorial),
  market: () => import("./market-tutorial").then((m) => m.marketTutorial),
  auction: () => import("./auction-tutorial").then((m) => m.auctionTutorial),

  // Progression
  enhance: () => import("./enhance-tutorial").then((m) => m.enhanceTutorial),
  affix: () => import("./affix-tutorial").then((m) => m.affixTutorial),
  alchemy: () => import("./alchemy-tutorial").then((m) => m.alchemyTutorial),
  "alchemy-convert": () => import("./alchemy-tutorial").then((m) => m.alchemyConvertTutorial),
  "alchemy-bench": () => import("./alchemy-tutorial").then((m) => m.alchemyBenchTutorial),
  "alchemy-inherit": () => import("./alchemy-tutorial").then((m) => m.alchemyInheritTutorial),
  "alchemy-transmute": () => import("./alchemy-tutorial").then((m) => m.alchemyTransmuteTutorial),
  "alchemy-smelt": () => import("./alchemy-tutorial").then((m) => m.alchemySmeltTutorial),
  "alchemy-skill-forge": () => import("./alchemy-tutorial").then((m) => m.alchemySkillForgeTutorial),
  combat: () => import("./combat-tutorial").then((m) => m.combatTutorial),
  arena: () => import("./arena-tutorial").then((m) => m.arenaTutorial),
  "combat-trial": () => import("./combat-trial-tutorial").then((m) => m.combatTrialTutorial),
  fishing: () => import("./fishing-tutorial").then((m) => m.fishingTutorial),
  pond: () => import("./pond-tutorial").then((m) => m.pondTutorial),
  tasks: () => import("./tasks-tutorial").then((m) => m.tasksTutorial),
  building: () => import("./building-tutorial").then((m) => m.buildingTutorial),
  crafting: () => import("./crafting-tutorial").then((m) => m.craftingTutorial),
  achievements: () => import("./achievements-tutorial").then((m) => m.achievementsTutorial),

  // Social
  cattery: () => import("./cattery-tutorial").then((m) => m.catteryTutorial),
  settings: () => import("./settings-tutorial").then((m) => m.settingsTutorial),
}

const configCache = new Map<string, TutorialConfig>()

export async function loadTutorialConfig(id: string): Promise<TutorialConfig | null> {
  const cached = configCache.get(id)
  if (cached) return cached

  const loader = loaders[id]
  if (!loader) return null

  const config = await loader()
  configCache.set(id, config)
  return config
}

export function getAllTutorialIds(): string[] {
  return Object.keys(loaders)
}
