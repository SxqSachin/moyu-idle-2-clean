import { expect, test, type Locator, type Page } from "@playwright/test"

const now = "2026-08-31T06:30:00Z"

const waters = [
  {
    id: "sunmere_shallows", order: 1, requiredFishingLevel: 1,
    names: { "zh-CN": "晴湾浅滩", "en-US": "Sunmere Shallows" },
    descriptions: { "zh-CN": "芦苇、睡莲和缓坡浅水组成的暖水湾。", "en-US": "A warm bay of reeds and lilies." },
    baseTemperatureC: 22.5, waterKind: "warm_stillwater"
  },
  {
    id: "willowrun_valley", order: 2, requiredFishingLevel: 15,
    names: { "zh-CN": "柳溪河谷", "en-US": "Willowrun Valley" },
    descriptions: { "zh-CN": "清流穿过砾石滩、深潭和倒木岸。", "en-US": "A clear moving river." },
    baseTemperatureC: 15.5, waterKind: "temperate_river"
  },
  {
    id: "frostglass_lake", order: 3, requiredFishingLevel: 35,
    names: { "zh-CN": "霜镜深湖", "en-US": "Frostglass Lake" },
    descriptions: { "zh-CN": "雪线下的寒冷深湖，陡岸与暗色水层藏着大鱼。", "en-US": "A cold deep lake." },
    baseTemperatureC: 7.5, waterKind: "cold_deep_lake"
  }
]

const spotSpeciesWeights: Record<string, Record<string, number>> = {
  lily_edge: { crucian_carp: 3, lotus_koi: 1 },
  old_willow: { crucian_carp: 2, lotus_koi: 1 },
  gravel_run: { brown_trout: 1 },
  deep_bend: { brown_trout: 1 },
  granite_shelf: { lake_trout: 1 },
  blue_drop: { lake_trout: 1 }
}

const spots = [
  ["lily_edge", "sunmere_shallows", "睡莲边", ["float", "lure"]],
  ["old_willow", "sunmere_shallows", "老柳树下", ["float", "bottom"]],
  ["gravel_run", "willowrun_valley", "砾石长濑", ["float", "lure"]],
  ["deep_bend", "willowrun_valley", "深水河弯", ["bottom", "lure"]],
  ["granite_shelf", "frostglass_lake", "花岗岩台", ["float", "lure"]],
  ["blue_drop", "frostglass_lake", "蓝色陡坎", ["bottom", "lure"]]
].map(([id, waterId, name, methods], index) => ({
  id, waterId, order: index + 1, names: { "zh-CN": name, "en-US": name }, habitat: "shore",
  depthM: 1.5 + index, oxygen: 0.88, baseBiteRate: 0.42, methods, speciesWeights: spotSpeciesWeights[String(id)] || {}
}))

const speciesMeasurements: Record<string, [number, number, number]> = {
  crucian_carp: [10, 50, 1.55],
  lotus_koi: [24, 95, 1.55],
  emberfin: [18, 70, 1.15],
  brown_trout: [18, 100, 1.05],
  lake_trout: [30, 150, .97],
  northern_pike: [30, 150, .84]
}

const species = [
  ["crucian_carp", "sunmere_shallows", "鲫鱼", "crucian-carp.webp", false, "common"],
  ["lotus_koi", "sunmere_shallows", "莲纹锦鲤", "lotus-koi.webp", true, "epic"],
  ["emberfin", "sunmere_shallows", "烬鳍鱼", "emberfin.webp", true, "epic"],
  ["brown_trout", "willowrun_valley", "褐鳟", "brown-trout.webp", false, "uncommon"],
  ["lake_trout", "frostglass_lake", "湖鳟", "lake-trout.webp", false, "rare"],
  ["northern_pike", "frostglass_lake", "白斑狗鱼", "northern-pike.webp", false, "rare"]
].map(([id, waterId, name, file, fantasy, rarity]) => {
  const [minLengthCm, maxLengthCm, shapeFactor] = speciesMeasurements[String(id)] || [10, 100, 1]
  return {
    id, waterId, rarity, fantasy, names: { "zh-CN": name, "en-US": name }, minLengthCm, maxLengthCm,
    shapeFactor, baseValue: 400, fightPower: 1, preferredMethods: ["float", "bottom"],
    preferredLayers: id === "brown_trout" ? ["surface", "midwater"] : ["midwater", "near_bottom"], depth: "mid",
    activeHours: id === "emberfin" ? [12, 13, 14] : id === "lotus_koi" ? [5, 6, 18, 19] : [5, 6, 17, 18],
    windowWeather: fantasy ? id === "emberfin" ? "sunny" : "mist" : undefined,
    iconPath: `/assets/fishing/fish/${file}`
  }
})

const pondSpecies = [
  { id: "pond_crucian_carp", speciesId: "crucian_carp", dietClass: "herbal", secondaryFeedClasses: ["insect"], maturityAgeDays: 720, lifespanDays: 7300 },
  { id: "pond_lotus_koi", speciesId: "lotus_koi", dietClass: "herbal", secondaryFeedClasses: ["insect", "arcane"], maturityAgeDays: 1500, lifespanDays: 9125 },
  { id: "pond_emberfin", speciesId: "emberfin", dietClass: "arcane", secondaryFeedClasses: ["protein"], maturityAgeDays: 1440, lifespanDays: 4380 },
  { id: "pond_brown_trout", speciesId: "brown_trout", dietClass: "protein", secondaryFeedClasses: ["insect"], maturityAgeDays: 900, lifespanDays: 2190, predation: { maxPreyLengthRatio: 0.28, huntBelowSatiety: 55, dailyHuntChance: 0.32 } },
  { id: "pond_lake_trout", speciesId: "lake_trout", dietClass: "protein", secondaryFeedClasses: ["insect"], maturityAgeDays: 1800, lifespanDays: 7300, predation: { maxPreyLengthRatio: 0.32, huntBelowSatiety: 55, dailyHuntChance: 0.38 } },
  { id: "pond_northern_pike", speciesId: "northern_pike", dietClass: "protein", secondaryFeedClasses: ["insect"], maturityAgeDays: 1500, lifespanDays: 5475, predation: { maxPreyLengthRatio: 0.48, huntBelowSatiety: 70, dailyHuntChance: 0.75 } }
]

const fishingMutations = [
  {
    id: "prismatic", nameKey: "game.fishing.mutation_prismatic", descriptionKey: "game.fishing.mutation_prismaticDesc", visualEffect: "prismatic",
    badgeIcon: "lucide:gem", appearanceChance: 0.0025, inheritanceChance: 0.06, lifespanMultiplier: 1.8, saleValueMultiplier: 2,
    alchemy: { primaryValueMultiplier: 1.5, bonusAttributes: [{ attrKey: "alchemy_precision", min: 0.018, max: 0.032 }] }
  },
  {
    id: "rainbow", nameKey: "game.fishing.mutation_rainbow", descriptionKey: "game.fishing.mutation_rainbowDesc", visualEffect: "rainbow",
    badgeIcon: "lucide:rainbow", appearanceChance: 0.0015, inheritanceChance: 0.04, lifespanMultiplier: 1.6, saleValueMultiplier: 2.5,
    alchemy: { primaryValueMultiplier: 1.35, bonusAttributes: [{ attrKey: "alchemy_lucky_chance", min: 0.012, max: 0.025 }] }
  },
  {
    id: "shimmering", nameKey: "game.fishing.mutation_shimmering", descriptionKey: "game.fishing.mutation_shimmeringDesc", visualEffect: "shimmering",
    badgeIcon: "lucide:sparkles", appearanceChance: 0.005, inheritanceChance: 0.1, lifespanMultiplier: 1.35, saleValueMultiplier: 1.5,
    alchemy: { primaryValueMultiplier: 1.25, bonusAttributes: [{ attrKey: "battle_drop_rate_bonus", min: 0.008, max: 0.016 }] }
  }
]

const tackle = [
  { itemId: "basic_fishing_rod", slot: "rod", methods: ["float", "bottom", "lure"] },
  { itemId: "silverleaf_float_rod", slot: "rod", methods: ["float", "bottom"] },
  { itemId: "basic_spinning_reel", slot: "reel", methods: ["float", "bottom", "lure"] },
  { itemId: "braided_main_line", slot: "main_line", methods: ["float", "bottom", "lure"] },
  { itemId: "stretch_mono_line", slot: "main_line", methods: ["float", "bottom", "lure"] },
  { itemId: "clear_leader", slot: "leader", methods: ["float", "bottom", "lure"] },
  { itemId: "fine_fishing_hook", slot: "hook", methods: ["float", "bottom"] },
  { itemId: "reed_float", slot: "float_or_sinker", methods: ["float"] },
  { itemId: "slip_float", slot: "float_or_sinker", methods: ["float"] },
  { itemId: "river_sinker", slot: "float_or_sinker", methods: ["bottom"] },
  { itemId: "silver_shad_lure", slot: "lure", methods: ["lure"] },
  { itemId: "earthworm_bait", slot: "bait", methods: ["float", "bottom"] }
]

const ownedTackle = [
  { itemId: "basic_fishing_rod", slot: "rod", quantity: 1, instanceId: 101, durability: 92, maxDurability: 100 },
  { itemId: "basic_spinning_reel", slot: "reel", quantity: 1, instanceId: 102, durability: 88, maxDurability: 100 },
  { itemId: "braided_main_line", slot: "main_line", quantity: 45 },
  { itemId: "clear_leader", slot: "leader", quantity: 24 },
  { itemId: "fine_fishing_hook", slot: "hook", quantity: 32 },
  { itemId: "reed_float", slot: "float_or_sinker", quantity: 16 },
  { itemId: "river_sinker", slot: "float_or_sinker", quantity: 18 },
  { itemId: "silver_shad_lure", slot: "lure", quantity: 9 },
  { itemId: "earthworm_bait", slot: "bait", quantity: 64 }
]

const braidedMainLineInventory = {
  instanceId: 301,
  userId: 42,
  templateItemId: "braided_main_line",
  quantity: 45,
  enhanceLevel: 0,
  durability: 100,
  maxDurability: 100,
  baseSnapshot: {},
  affixes: [],
  customData: {},
  state: "inventory"
}

const basicFishingRodInventory = {
  instanceId: 302,
  userId: 42,
  templateItemId: "basic_fishing_rod",
  quantity: 1,
  enhanceLevel: 0,
  durability: 160,
  maxDurability: 160,
  baseSnapshot: {},
  affixes: [],
  customData: {},
  state: "inventory"
}

const sweetDoughBaitInventory = {
  instanceId: 303,
  userId: 42,
  templateItemId: "sweet_dough_bait",
  quantity: 24,
  enhanceLevel: 0,
  durability: 0,
  maxDurability: 0,
  baseSnapshot: {},
  affixes: [],
  customData: {},
  state: "inventory"
}

const starrippleBaitInventory = {
  instanceId: 304,
  userId: 42,
  templateItemId: "starripple_bait",
  quantity: 10,
  enhanceLevel: 0,
  durability: 0,
  maxDurability: 0,
  baseSnapshot: {},
  affixes: [],
  customData: {},
  state: "inventory"
}

const playerFishingEquipment = [
  { instanceId: 101, templateItemId: "basic_fishing_rod", equipType: "work", enhanceLevel: 0, durability: 160, maxDurability: 160, state: "equipped", equippedSlot: "fishing_rod", updatedAt: now },
  { instanceId: 102, templateItemId: "basic_spinning_reel", equipType: "work", enhanceLevel: 0, durability: 100, maxDurability: 100, state: "equipped", equippedSlot: "fishing_reel", updatedAt: now }
]

const kittyFishingEquipment = [
  { instanceId: 201, templateItemId: "basic_fishing_rod", equipType: "work", enhanceLevel: 0, durability: 160, maxDurability: 160, state: "equipped", equippedSlot: "fishing_rod", updatedAt: now },
  { instanceId: 202, templateItemId: "basic_spinning_reel", equipType: "work", enhanceLevel: 0, durability: 100, maxDurability: 100, state: "equipped", equippedSlot: "fishing_reel", updatedAt: now }
]

const kittyFishingAttributes = [
  { attrKey: "collecting_speed", finalValue: 0.1, baseDefault: 0, unit: "percent", category: "work", growthType: "static", order: 39, nameKey: "attr.collecting_speed.name", descKey: "attr.collecting_speed.desc" },
  { attrKey: "mining_speed", finalValue: 0.05, baseDefault: 0, unit: "percent", category: "work", growthType: "static", order: 40, nameKey: "attr.mining_speed.name", descKey: "attr.mining_speed.desc" },
  { attrKey: "farming_animal_speed", finalValue: 0, baseDefault: 0, unit: "percent", category: "work", growthType: "static", order: 41, nameKey: "attr.farming_animal_speed.name", descKey: "attr.farming_animal_speed.desc" },
  { attrKey: "crafting_speed", finalValue: 0, baseDefault: 0, unit: "percent", category: "work", growthType: "static", order: 42, nameKey: "attr.crafting_speed.name", descKey: "attr.crafting_speed.desc" },
  { attrKey: "sewing_speed", finalValue: 0, baseDefault: 0, unit: "percent", category: "work", growthType: "static", order: 43, nameKey: "attr.sewing_speed.name", descKey: "attr.sewing_speed.desc" },
  { attrKey: "forging_speed", finalValue: 0, baseDefault: 0, unit: "percent", category: "work", growthType: "static", order: 44, nameKey: "attr.forging_speed.name", descKey: "attr.forging_speed.desc" },
  { attrKey: "alchemy_efficiency", finalValue: 0, baseDefault: 0, unit: "percent", category: "work", growthType: "static", order: 51, nameKey: "attr.alchemy_efficiency.name", descKey: "attr.alchemy_efficiency.desc" },
  { attrKey: "cooking_speed", finalValue: 0, baseDefault: 0, unit: "percent", category: "work", growthType: "static", order: 52, nameKey: "attr.cooking_speed.name", descKey: "attr.cooking_speed.desc" },
  { attrKey: "fishing", finalValue: 1, baseDefault: 1, baseLevel: 1, skillPointLevel: 0, progressLevel: 1, progressExp: 0, progressNextExp: 100, unit: "number", category: "work", growthType: "levelable", order: 53, nameKey: "attr.fishing.name", descKey: "attr.fishing.desc" },
  { attrKey: "fishing_control", finalValue: 1, baseDefault: 0, unit: "number", category: "work", growthType: "static", order: 54, nameKey: "attr.fishing_control.name", descKey: "attr.fishing_control.desc" },
  { attrKey: "fishing_sensitivity", finalValue: 0.8, baseDefault: 0, unit: "number", category: "work", growthType: "static", order: 55, nameKey: "attr.fishing_sensitivity.name", descKey: "attr.fishing_sensitivity.desc" },
  { attrKey: "fishing_efficiency", finalValue: 0, baseDefault: 0, unit: "percent", category: "work", growthType: "static", order: 56, nameKey: "attr.fishing_efficiency.name", descKey: "attr.fishing_efficiency.desc" },
  { attrKey: "fishing_power", finalValue: 0, baseDefault: 0, unit: "percent", category: "work", growthType: "static", order: 57, nameKey: "attr.fishing_power.name", descKey: "attr.fishing_power.desc" },
  { attrKey: "fishing_casting", finalValue: 0.35, baseDefault: 0, unit: "number", category: "work", growthType: "static", order: 58, nameKey: "attr.fishing_casting.name", descKey: "attr.fishing_casting.desc" },
  { attrKey: "fishing_hooking", finalValue: 0.3, baseDefault: 0, unit: "number", category: "work", growthType: "static", order: 59, nameKey: "attr.fishing_hooking.name", descKey: "attr.fishing_hooking.desc" }
]

const loadout = {
  loadoutId: "loadout-1", name: "晴湾轻钓组", method: "float", strategy: "balanced", safetyLoadPercent: 82,
  components: {
    rodInstanceId: 101, reelInstanceId: 102, mainLineItemId: "braided_main_line", leaderItemId: "clear_leader",
    hookItemId: "fine_fishing_hook", floatOrSinkerItemId: "reed_float", baitItemId: "earthworm_bait"
  }
}

const warehouseFish = {
  instanceId: 503,
  userId: 42,
  templateItemId: "fish_catch",
  quantity: 1,
  enhanceLevel: 0,
  durability: 0,
  maxDurability: 0,
  baseSnapshot: {},
  affixes: [],
  state: "inventory",
  fishData: {
    speciesId: "lotus_koi",
    ageDays: 830,
    sex: "female",
    lengthCm: 48.3,
    weightKg: 2.61,
    condition: 1.08,
    health: 0.94,
    waterId: "sunmere_shallows",
    spotId: "sunmere_lily_cove",
    caughtAt: now,
    rulesetVersion: "fishing-v1"
  }
}

function forecast() {
  return spots.map((spot, index) => ({
    spotId: spot.id, pressure: index === 0 ? 0.12 : 0.38,
    now: { gameHour: 6.5, season: "summer", weather: index % 3 === 0 ? "sunny" : "cloud", temperatureC: 21.8, oxygen: 0.9, activity: 1.18, activityBand: index % 2 === 0 ? "busy" : "steady", trend: "rising" },
    next: Array.from({ length: 6 }, (_, step) => ({
      gameHour: 7 + step, season: "summer", weather: step < 2 ? "sunny" : "cloud", temperatureC: 21.5,
      oxygen: 0.9, activity: step < 2 ? 1.18 : 0.96, activityBand: step < 2 ? "busy" : "steady", trend: "steady"
    }))
  }))
}

function currentTrip(actor: { actorType: "player" | "kitty"; actorId: number } = { actorType: "player", actorId: 0 }) {
  const phaseNow = Date.now()
  return {
    tripId: "trip-live", queueId: 801, waterId: "sunmere_shallows", spotId: "lily_edge", loadoutId: "loadout-1",
    actorType: actor.actorType, actorId: actor.actorType === "kitty" ? actor.actorId : 0,
    strategy: "balanced", safetyLoadPercent: 82, status: "running", phase: "fighting",
    phaseStartedAt: new Date(phaseNow - 30_000).toISOString(), phaseEndsAt: new Date(phaseNow + 30_000).toISOString(), plannedEndAt: new Date(phaseNow + 90 * 60_000).toISOString(),
    currentFish: { sizeClass: "large", stamina: 100, lineLoadKg: 4.8, lineState: "tight" },
    summary: { casts: 7, bites: 4, landed: 3, kept: 2, released: 1, escaped: 2, lineBreaks: 1, baitUsed: 7, totalWeightKg: 4.82, fishingExp: 18.6,
      lastCatch: {
        catchId: 33, tripId: "trip-live", speciesId: "lotus_koi", kept: true, inventoryInstanceId: 503, caughtAt: now,
        fish: { speciesId: "lotus_koi", ageDays: 830, sex: "female", lengthCm: 48.3, weightKg: 2.61, condition: 1.08, health: 0.94, waterId: "sunmere_shallows", spotId: "lily_edge", caughtAt: now, rulesetVersion: "fishing-v1" },
        record: { speciesId: "lotus_koi", length: true, weight: false }
      },
      recentEvents: [
        { sequence: 1, kind: "cast", at: new Date(phaseNow - 210_000).toISOString(), castNumber: 4 },
        { sequence: 2, kind: "bite", at: new Date(phaseNow - 180_000).toISOString(), castNumber: 4, sizeClass: "large" },
        { sequence: 3, kind: "hooked", at: new Date(phaseNow - 177_000).toISOString(), castNumber: 4, sizeClass: "large" },
        { sequence: 4, kind: "line_break", reason: "line_overload", at: new Date(phaseNow - 145_000).toISOString(), castNumber: 4, sizeClass: "large" },
        { sequence: 5, kind: "cast", at: new Date(phaseNow - 141_000).toISOString(), castNumber: 5 },
        { sequence: 6, kind: "bite", at: new Date(phaseNow - 120_000).toISOString(), castNumber: 5, sizeClass: "medium" },
        { sequence: 7, kind: "hooked", at: new Date(phaseNow - 117_000).toISOString(), castNumber: 5, sizeClass: "medium" },
        { sequence: 8, kind: "escaped", reason: "hook_slipped", at: new Date(phaseNow - 92_000).toISOString(), castNumber: 5, sizeClass: "medium" },
        { sequence: 9, kind: "cast", at: new Date(phaseNow - 88_000).toISOString(), castNumber: 6 },
        { sequence: 10, kind: "landed", at: new Date(phaseNow - 65_000).toISOString(), castNumber: 6, sizeClass: "large", speciesId: "lotus_koi", lengthCm: 48.3, weightKg: 2.61, disposition: "kept" },
        { sequence: 11, kind: "cast", at: new Date(phaseNow - 61_000).toISOString(), castNumber: 7 },
        { sequence: 12, kind: "bite", at: new Date(phaseNow - 33_000).toISOString(), castNumber: 7, sizeClass: "large" },
        { sequence: 13, kind: "hooked", at: new Date(phaseNow - 30_000).toISOString(), castNumber: 7, sizeClass: "large" }
      ]
    },
    supplies: {
      initial: { mainLine: 8, leader: 8, hook: 12, floatOrSinker: 4, bait: 64 },
      remaining: { mainLine: 8, leader: 8, hook: 12, floatOrSinker: 4, bait: 57 },
      consumed: { mainLine: 0, leader: 0, hook: 0, floatOrSinker: 0, bait: 7 }
    },
    startedAt: now
  }
}

function fishingTripHistory() {
  return [
    {
      tripId: "trip-history-new", queueId: 0, actorType: "player", actorId: 0,
      waterId: "sunmere_shallows", spotId: "lily_edge", loadoutId: "loadout-1",
      strategy: "balanced", safetyLoadPercent: 82, status: "completed", phase: "finished",
      phaseStartedAt: "2026-08-31T06:14:00Z", phaseEndsAt: "2026-08-31T06:15:00Z",
      plannedEndAt: "2026-08-31T06:15:00Z", startedAt: "2026-08-31T05:00:00Z", endedAt: "2026-08-31T06:15:00Z",
      summary: {
        casts: 25, bites: 25, landed: 12, kept: 6, released: 6, escaped: 13, lineBreaks: 0,
        baitUsed: 25, scaleglowFound: 0, totalWeightKg: 9.84, fishingExp: 61.2,
        recentEvents: [
          { sequence: 97, kind: "cast", at: "2026-08-31T05:48:00Z", castNumber: 25 },
          { sequence: 98, kind: "bite", at: "2026-08-31T05:48:28Z", castNumber: 25, sizeClass: "small" },
          { sequence: 99, kind: "hooked", at: "2026-08-31T05:48:31Z", castNumber: 25, sizeClass: "small" },
          { sequence: 100, kind: "escaped", reason: "hook_slipped", at: "2026-08-31T05:48:39Z", castNumber: 25, sizeClass: "small" },
          { sequence: 101, kind: "finished", reason: "time_reached", at: "2026-08-31T06:15:00Z" }
        ]
      },
      supplies: {
        initial: { mainLine: 8, leader: 8, hook: 12, floatOrSinker: 4, bait: 64 },
        remaining: { mainLine: 8, leader: 8, hook: 11, floatOrSinker: 4, bait: 39 },
        consumed: { mainLine: 0, leader: 0, hook: 1, floatOrSinker: 0, bait: 25 }, returned: true
      }
    },
    {
      tripId: "trip-history-old", queueId: 0, actorType: "kitty", actorId: 77,
      waterId: "sunmere_shallows", spotId: "old_willow", loadoutId: "loadout-1",
      strategy: "conservative", safetyLoadPercent: 94, status: "stopped", phase: "finished",
      phaseStartedAt: "2026-08-30T04:40:00Z", phaseEndsAt: "2026-08-30T04:42:00Z",
      plannedEndAt: "2026-08-30T06:30:00Z", startedAt: "2026-08-30T04:30:00Z", endedAt: "2026-08-30T04:42:00Z",
      stopReason: "manual_stop",
      summary: {
        casts: 2, bites: 1, landed: 1, kept: 0, released: 1, escaped: 0, lineBreaks: 0,
        baitUsed: 2, scaleglowFound: 0, totalWeightKg: 0.82, fishingExp: 4.6, stopReason: "manual_stop",
        recentEvents: [
          { sequence: 5, kind: "landed", at: "2026-08-30T04:38:00Z", castNumber: 2, sizeClass: "medium", speciesId: "crucian_carp", lengthCm: 31.7, weightKg: 0.82, disposition: "released" },
          { sequence: 6, kind: "finished", reason: "manual_stop", at: "2026-08-30T04:42:00Z" }
        ]
      },
      supplies: {
        initial: { mainLine: 8, leader: 8, hook: 12, floatOrSinker: 4, bait: 20 },
        remaining: { mainLine: 8, leader: 8, hook: 12, floatOrSinker: 4, bait: 18 },
        consumed: { mainLine: 0, leader: 0, hook: 0, floatOrSinker: 0, bait: 2 }, returned: true
      }
    }
  ]
}

function fishingTripEventsByTripId() {
  const complete = Array.from({ length: 25 }, (_, index) => {
    const castNumber = index + 1
    const baseSequence = index * 4 + 1
    const at = new Date(Date.parse("2026-08-31T05:00:00Z") + index * 120_000).toISOString()
    const landed = castNumber % 2 === 0
    return [
      { sequence: baseSequence, kind: "cast", at, castNumber },
      { sequence: baseSequence + 1, kind: "bite", at, castNumber, sizeClass: landed ? "medium" : "small" },
      { sequence: baseSequence + 2, kind: "hooked", at, castNumber, sizeClass: landed ? "medium" : "small" },
      landed
        ? { sequence: baseSequence + 3, kind: "landed", at, castNumber, sizeClass: "medium", speciesId: "crucian_carp", lengthCm: 31.7, weightKg: 0.82, disposition: castNumber % 4 === 0 ? "released" : "kept" }
        : { sequence: baseSequence + 3, kind: "escaped", reason: "hook_slipped", at, castNumber, sizeClass: "small" }
    ]
  }).flat()
  complete.push({ sequence: 101, kind: "finished", reason: "time_reached", at: "2026-08-31T06:15:00Z" })
  return {
    "trip-history-new": { items: complete, historyComplete: true },
    "trip-history-old": {
      items: fishingTripHistory()[1].summary.recentEvents,
      historyComplete: false
    }
  }
}

function currentFishingQueueItem(actor: { actorType: "player" | "kitty"; actorId: number } = { actorType: "player", actorId: 0 }) {
  const trip = currentTrip(actor)
  const startedAtMs = Date.now() - 30 * 60_000
  const plannedEndAtMs = Date.now() + 90 * 60_000
  const phaseEndsAtMs = Date.now() + 30_000
  return {
    id: 801,
    actorType: trip.actorType,
    actorId: trip.actorId,
    actionId: "fishing.trip",
    status: "running",
    totalCount: 1,
    doneCount: 0,
    runStartAt: new Date(Date.now() - 30_000).toISOString(),
    runEndsAt: new Date(phaseEndsAtMs).toISOString(),
    display: {
      kind: "fishing.trip",
      effectiveDurationMs: 60_000,
      fishing: {
        tripId: trip.tripId,
        waterId: trip.waterId,
        spotId: "sunmere_lily_cove",
        loadoutName: loadout.name,
        method: loadout.method,
        strategy: trip.strategy,
        phase: trip.phase,
        lineState: trip.currentFish.lineState,
        plannedDurationSeconds: 7_200,
        startedAtMs,
        plannedEndAtMs,
        phaseEndsAtMs,
        landed: trip.summary.landed,
        released: trip.summary.released,
        totalWeightKg: trip.summary.totalWeightKg,
        initialSupplies: trip.supplies.initial,
        remainingSupplies: trip.supplies.remaining
      }
    }
  }
}

function regularPendingQueueItems() {
  return [
    { id: 802, actorType: "player", actorId: 0, actionId: "farm_sheep", status: "pending", totalCount: 780, doneCount: 0 },
    { id: 803, actorType: "player", actorId: 0, actionId: "mine_coal", status: "pending", totalCount: 1001, doneCount: 0 },
    { id: 804, actorType: "player", actorId: 0, actionId: "mine_stone", status: "pending", totalCount: 1001, doneCount: 0 }
  ]
}

function catches() {
  return [
    { catchId: 33, tripId: "trip-1", speciesId: "lotus_koi", kept: true, inventoryInstanceId: 503, caughtAt: now,
      fish: { speciesId: "lotus_koi", ageDays: 830, sex: "female", lengthCm: 48.3, weightKg: 2.61, condition: 1.08, health: 0.94, waterId: "sunmere_shallows", spotId: "lily_edge", caughtAt: now, rulesetVersion: "fishing-v1" },
      record: { speciesId: "lotus_koi", length: true, weight: false } },
    { catchId: 32, tripId: "trip-1", speciesId: "crucian_carp", kept: true, caughtAt: now,
      fish: { speciesId: "crucian_carp", ageDays: 480, sex: "male", lengthCm: 31.7, weightKg: 0.82, condition: 0.96, health: 0.98, waterId: "sunmere_shallows", spotId: "old_willow", caughtAt: now, rulesetVersion: "fishing-v1" },
      record: { speciesId: "crucian_carp", length: false, weight: true } }
  ]
}

function repeatedCatches(count: number) {
  const source = catches()[0]
  return Array.from({ length: count }, (_, index) => ({
    ...source,
    catchId: 10_000 - index,
    inventoryInstanceId: 20_000 + index,
    saleEligible: true,
    saleIneligibleReason: undefined as "cultured_immature" | "dead" | "invalid_data" | undefined,
    saleCurrentAgeDays: undefined as number | undefined,
    saleRequiredAgeDays: undefined as number | undefined,
    fish: {
      ...source.fish,
      lengthCm: source.fish.lengthCm + (index % 7) * 0.1,
      weightKg: source.fish.weightKg + (index % 5) * 0.01
    },
    record: { ...source.record, length: index === 0, weight: false }
  }))
}

async function itemGridMetrics(item: Locator) {
  await expect(item).toBeVisible()
  return item.evaluate((element) => {
    const itemBox = element.getBoundingClientRect()
    const cardBox = element.querySelector(".item-card")?.getBoundingClientRect()
    const grid = element.closest(".item-browser-virtual-grid__items")
    const gridStyle = grid ? getComputedStyle(grid) : null
    return {
      itemWidth: Math.round(itemBox.width),
      itemHeight: Math.round(itemBox.height),
      cardWidth: Math.round(cardBox?.width || 0),
      cardHeight: Math.round(cardBox?.height || 0),
      columnGap: Math.round(Number.parseFloat(gridStyle?.columnGap || "0")),
      rowGap: Math.round(Number.parseFloat(gridStyle?.rowGap || "0"))
    }
  })
}

function fishingSaleQuote(lineGold = 4093, itemCount = 1, mutations: string[] = [], instanceIds: number[] = []) {
  const count = Math.max(1, Math.trunc(itemCount))
  const mutationMultiplier = mutations.reduce((total, mutationID) => {
    const definition = fishingMutations.find((mutation) => mutation.id === mutationID)
    return total + Math.max(1, Number(definition?.saleValueMultiplier || 1)) - 1
  }, 1)
  const finalGold = Math.round(lineGold * mutationMultiplier)
  return {
    quoteAt: now, marketDate: "2026-08-31", totalGold: finalGold * count,
    items: Array.from({ length: count }, (_, index) => ({
      instanceId: instanceIds[index] || 503 + index, speciesId: "lotus_koi", ageDays: 830 + index, lengthCm: 48.3 + index * 0.1, weightKg: 2.61 + index * 0.01,
      mutations,
      baseValue: 3160, ageFactor: 0.993, lengthFactor: 1.058, weightFactor: 1.162,
      qualityFactor: 1.008, specimenMultiplier: 1.231, mutationMultiplier, dailyDemand: 1.012,
      supplyFactor: 1.04, cultureFactor: 1, marketMultiplier: 1.052, finalGold
    }))
  }
}

function pondOverview(includeDslFeeds = false) {
  const residentFish = {
    ...catches()[0].fish, speciesId: "lotus_koi", lifeState: "alive", origin: "cultured", satiety: 92, generation: 1,
    pondZone: "main",
    genomeRevealed: true,
    genes: { sizePotential: 0.5, growthRate: 0.2, feedEfficiency: 0.8, vitality: 0.4, lifespan: 0.6, fertility: 0.1, mutations: ["prismatic"] }
  }
  const residentMale = { ...residentFish, sex: "male", lengthCm: 44.1, weightKg: 2.12, genomeRevealed: false, genes: { mutations: ["rainbow"] } }
  const residentJuvenile = { ...residentFish, ageDays: 480, lengthCm: 18.2, weightKg: 0.16, genomeRevealed: false, genes: { mutations: ["shimmering"] } }
  const residentPredator = { ...residentFish, speciesId: "northern_pike", sex: "female", ageDays: 1900, lengthCm: 105.0, weightKg: 8.7, satiety: 28, genomeRevealed: false, genes: { mutations: ["prismatic", "rainbow", "shimmering"] } }
  const nurseryFry = { ...residentFish, pondZone: "nursery", ageDays: 30, lengthCm: 6.8, weightKg: 0.018, genomeRevealed: false, genes: { mutations: [] } }
  const availableFish = catches()[0].fish
  return {
    built: true,
    level: 1,
    rulesetVersion: "fishing-pond-v5",
    bioDaysPerRealDay: 365,
    maxOfflineHours: 720,
    limits: { fishCapacity: 12, nurseryCapacity: 12, biomassKg: 60, breedingPairs: 1, feederNutrition: 600 },
    fish: [
      { instanceId: 600, customData: { name: "小金" }, fish: residentFish, dietClass: "herbal", maturityAgeDays: 720, expectedLifespanDays: 7300, mature: true, maturityProgress: 1, maturityRealHoursRemaining: 0, phenotypes: ["efficient_feeder", "large_frame"], phenotypeEffects: [{ key: "efficient_feeder", value: 0.08 }, { key: "large_frame", value: 0.05 }] },
      { instanceId: 601, customData: { name: "赤尾" }, fish: residentMale, dietClass: "herbal", maturityAgeDays: 720, expectedLifespanDays: 7300, mature: true, phenotypes: [] },
      { instanceId: 602, customData: { name: "幼鲤" }, fish: residentJuvenile, dietClass: "herbal", maturityAgeDays: 720, expectedLifespanDays: 7300, mature: false, maturityProgress: 0.6667, maturityRealHoursRemaining: 15.8, phenotypes: [] },
      { instanceId: 603, customData: { name: "深湾" }, fish: residentPredator, dietClass: "protein", secondaryFeedClasses: ["insect"], maturityAgeDays: 1500, expectedLifespanDays: 5475, mature: true, phenotypes: [], predation: { maxPreyLengthRatio: 0.48, huntBelowSatiety: 70, dailyHuntChance: 0.75 } },
      { instanceId: 604, customData: { name: "小苗" }, fish: nurseryFry, dietClass: "herbal", maturityAgeDays: 720, expectedLifespanDays: 7300, mature: false, maturityProgress: 0.0417, maturityRealHoursRemaining: 45.4, phenotypes: [] }
    ],
    availableFish: [{ instanceId: 503, customData: {}, fish: availableFish, dietClass: "herbal", maturityAgeDays: 720, expectedLifespanDays: 7300, mature: true, phenotypes: [] }],
    pairs: [],
    feeds: [
      { itemId: "pond_feed_herbal", feedClass: "herbal", nutrition: 10, inventoryCount: 3, storedNutrition: 0 },
      ...(includeDslFeeds ? [
        { itemId: "pond_feed_selective_herbal", feedClass: "herbal", nutrition: 14, rulesetVersion: "pond-feed-v2", consumePriority: 50, conditions: [{ field: "dietClass", op: "eq", value: "herbal" }], effects: [], inventoryCount: 2, storedNutrition: 0 },
        { itemId: "pond_feed_length_growth", feedClass: "protein", nutrition: 16, rulesetVersion: "pond-feed-v2", consumePriority: 60, conditions: [], effects: [{ kind: "stat_delta", stat: "lengthCm", mode: "bonus_band", valuePerPortion: 0.04, maxTargetMultiplier: 1.025 }], inventoryCount: 2, storedNutrition: 0 },
        { itemId: "pond_feed_weight_growth", feedClass: "protein", nutrition: 20, rulesetVersion: "pond-feed-v2", consumePriority: 60, conditions: [], effects: [{ kind: "stat_delta", stat: "weightKg", mode: "bonus_band", valuePerPortion: 0.04, maxTargetMultiplier: 1.04 }], inventoryCount: 2, storedNutrition: 0 },
        { itemId: "pond_feed_health_recovery", feedClass: "herbal", nutrition: 12, rulesetVersion: "pond-feed-v2", consumePriority: 90, conditions: [{ field: "health", op: "lt", value: 0.98 }], effects: [{ kind: "stat_delta", stat: "health", mode: "flat", valuePerPortion: 0.0025, ceiling: 1 }], inventoryCount: 2, storedNutrition: 0 },
        { itemId: "pond_feed_satiety_risk", feedClass: "herbal", nutrition: 10, rulesetVersion: "pond-feed-v2", consumePriority: 70, conditions: [{ field: "satiety", op: "lt", value: 80 }, { field: "health", op: "gte", value: 0.1 }], effects: [{ kind: "stat_delta", stat: "satiety", mode: "flat", valuePerPortion: 3, ceiling: 100 }, { kind: "stat_delta", stat: "health", mode: "flat", valuePerPortion: -0.0035, floor: 0.1 }], inventoryCount: 2, storedNutrition: 0 },
        { itemId: "pond_feed_forced_growth", feedClass: "protein", nutrition: 18, rulesetVersion: "pond-feed-v2", consumePriority: 80, conditions: [{ field: "health", op: "gte", value: 0.35 }], effects: [{ kind: "stat_delta", stat: "lengthCm", mode: "bonus_band", valuePerPortion: 0.05, maxTargetMultiplier: 1.04 }, { kind: "stat_delta", stat: "weightKg", mode: "bonus_band", valuePerPortion: 0.05, maxTargetMultiplier: 1.07 }, { kind: "stat_delta", stat: "health", mode: "flat", valuePerPortion: -0.008, floor: 0.05 }], inventoryCount: 2, storedNutrition: 0 }
      ] : [])
    ],
    feedTotalNutrition: 0,
    biomassKg: 13.59,
    events: [{ id: 1, kind: "predated", fishInstanceId: 603, relatedInstanceId: 599, details: { predatorSpeciesId: "northern_pike", preySpeciesId: "crucian_carp", predatorName: "深湾", preyName: "小银" }, createdAt: now }],
    lineageReagentCount: 0,
    broodstockMashCount: 0,
    lastSettledAt: now,
    mutations: fishingMutations
  }
}

function publicPondOverview(extraFishCount = 0) {
  const pond = pondOverview()
  const fish = pond.fish.map((row) => ({
    instanceId: row.instanceId,
    customData: row.customData,
    fish: {
      speciesId: row.fish.speciesId,
      ageDays: row.fish.ageDays,
      sex: row.fish.sex,
      lengthCm: row.fish.lengthCm,
      weightKg: row.fish.weightKg,
      pondZone: row.fish.pondZone,
      lifeState: row.fish.lifeState,
      generation: row.fish.generation,
      mutations: row.fish.genes.mutations
    },
    maturityAgeDays: row.maturityAgeDays,
    mature: row.mature,
    maturityProgress: row.maturityProgress,
    phenotypes: row.phenotypes,
    phenotypeEffects: row.phenotypeEffects,
    predation: row.predation
  }))
  const ordinaryTemplate = fish[2]
  for (let index = 0; index < extraFishCount; index++) {
    fish.push({
      ...ordinaryTemplate,
      instanceId: 700 + index,
      customData: { name: `访客鱼${index + 1}` },
      fish: {
        ...ordinaryTemplate.fish,
        ageDays: ordinaryTemplate.fish.ageDays + index * 37,
        mutations: []
      },
      phenotypes: index % 2 === 0 ? ["large_frame"] : []
    })
  }
  return {
    ownerUserId: 99,
    built: true,
    level: pond.level,
    rulesetVersion: pond.rulesetVersion,
    limits: { fishCapacity: pond.limits.fishCapacity, nurseryCapacity: pond.limits.nurseryCapacity, biomassKg: pond.limits.biomassKg },
    fish,
    biomassKg: pond.biomassKg,
    capabilities: ["fish.inspect", "fish.inspect_vitals", "fish.track", "water.throw_stone", "water.skip_stone", "pond.feed"],
    mutations: fishingMutations
  }
}

type FishingTripFailure = {
  code: string
  instanceState?: string
}

type FishingMockEquipmentItem = Record<string, unknown> & {
  instanceId: number
  templateItemId: string
  equipType: "work" | "battle"
  state: string
  equippedSlot: string
}

type FishingMockOptions = {
  playerAttributes?: Record<string, unknown>
  kittyAttributes?: typeof kittyFishingAttributes
  ownedTackle?: typeof ownedTackle
  loadouts?: typeof loadout[]
  scene?: { gameHour: number; weather: string; waterId?: string }
  catchItems?: ReturnType<typeof repeatedCatches>
  offlineRewards?: Record<string, unknown>
  catchPageSize?: number
  onFishingCatchRequest?: (cursor: number) => void
  playerEquipment?: FishingMockEquipmentItem[]
  kittyEquipment?: FishingMockEquipmentItem[]
  onKittyEquipRequest?: (body: unknown) => void
  onPondFeedRequest?: (body: unknown) => void
  onPondRenameRequest?: (body: unknown) => void
  onFishingSaleRequest?: (body: unknown) => void
  fishingSaleQuoteChangesOnce?: boolean
  fishingSaleQuoteItemCount?: number
  fishingSaleQuoteMutations?: string[]
  onFishingSocketReady?: (pushTrip: (trip: Record<string, unknown>) => void) => void
  fishingAnglers?: Array<{ actorType: "player" | "kitty"; actorId: number; name?: string; status: string; fishingLevel: number }>
  liveActor?: { actorType: "player" | "kitty"; actorId: number }
  historyItems?: ReturnType<typeof fishingTripHistory>
  onFishingHistoryRequest?: () => void
  onFishingHistoryEventRequest?: (tripId: string, cursor: number) => void
  publicPondExtraFishCount?: number
  pondAvailableFish?: ReturnType<typeof pondOverview>["availableFish"]
  pondDslFeeds?: boolean
  legacyTournament?: boolean
  tournamentStandardItems?: Array<{ slot: string; itemId: string }>
}

async function mockFishing(
  page: Page,
  live = false,
  onTripRequest?: (body: unknown) => void,
  inventoryItems: unknown[] = [],
  onStopRequest?: () => void,
  tripFailure?: FishingTripFailure,
  extraQueueItems: unknown[] = [],
  options: FishingMockOptions = {}
) {
  let tripRunning = live
  let fishingSaleAttempts = 0
  let catchItems = options.catchItems || catches()
  let activePlayerEquipment = (options.playerEquipment || playerFishingEquipment).map((item) => ({ ...item }))
  let activeKittyEquipment = (options.kittyEquipment || kittyFishingEquipment).map((item) => ({ ...item }))
  const liveActor = options.liveActor || { actorType: "player" as const, actorId: 0 }
  const fishingAnglers = options.fishingAnglers || [
    { actorType: "player" as const, actorId: 0, status: "idle", fishingLevel: 42 },
    { actorType: "kitty" as const, actorId: 77, name: "小满", status: "idle", fishingLevel: 1 }
  ]
  const playerFishingLevel = fishingAnglers.find((angler) => angler.actorType === "player")?.fishingLevel || 1
  const ownerPondOverview = () => pondOverview(Boolean(options.pondDslFeeds))
  await page.addInitScript(() => {
    localStorage.setItem("locale", "zh-CN")
    localStorage.setItem("token", "e2e-token")
    localStorage.setItem("userId", "42")
    localStorage.setItem("username", "钓鱼测试员")
    localStorage.setItem("player-settings:42", JSON.stringify({ language: "zh-CN", theme: "light", menuLayout: "icon-rail", simpleMode: true, emailVerified: true }))
  })
  await page.routeWebSocket(/\/ws(?:\?|$)/, (socket) => {
    options.onFishingSocketReady?.((trip) => socket.send(JSON.stringify({
      type: "game.fishing.trip",
      ts: Date.now(),
      data: { trip }
    })))
    socket.onMessage((message) => {
      if (typeof message !== "string") return
      const event = JSON.parse(message) as { type?: string }
      if (event.type === "auth.bind") socket.send(JSON.stringify({ type: "auth.bound", ts: Date.now(), data: {} }))
      if (event.type === "action.offline.rewards.get") socket.send(JSON.stringify({
        type: "action.offline.rewards.result",
        ts: Date.now(),
        data: { hasMoreDue: false, rewards: [], ...(options.offlineRewards || {}) }
      }))
      if (event.type === "action.queue.get") socket.send(JSON.stringify({
        type: "action.queue.updated",
        ts: Date.now(),
        data: { items: live ? [currentFishingQueueItem(liveActor), ...extraQueueItems] : extraQueueItems }
      }))
    })
  })
  await page.route("**/api/**", async (route) => {
    const request = route.request()
    const requestURL = new URL(request.url())
    const path = decodeURIComponent(requestURL.pathname)
    const fulfill = (data: unknown) => route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ ok: true, ts: Date.now(), data }) })
    if (path === "/api/bootstrap/init") return fulfill({
      playerStats: {
        stats: {},
        attributes: options.playerAttributes || {},
        buildings: { cattery: 2 },
        kitties: [{ id: 77, name: "小满", status: "idle", level: 9, exp: 0, nextExp: 100, totalExp: 0, bondExp: 0, mood: 0.8, createdAt: now }]
      },
      inventory: { items: inventoryItems }, chatConfig: { channels: [], defaultChannel: "world" }, chatHistory: { items: [] },
      playerSettings: { username: "钓鱼测试员", language: "zh-CN", theme: "light", menuLayout: "icon-rail", simpleMode: true, emailVerified: true },
      tutorialProgress: {}, playerPresence: { status: "online" }, systemVersion: {}, recycleConfig: { goldCoefficient: 0.5, affixMismatchExtraCoefficient: 0.6 }
    })
    if (path === "/api/player/stats") return fulfill({
      stats: {},
      attributes: options.playerAttributes || {},
      buildings: { cattery: 2 },
      kitties: [{ id: 77, name: "小满", status: "idle", level: 9, exp: 0, nextExp: 100, totalExp: 0, bondExp: 0, mood: 0.8, createdAt: now }]
    })
    if (path === "/api/player/profile/99") return fulfill({
      userId: 99, username: "湖畔旅人", nameHistory: [], kitties: [], skills: [], customSkills: [],
      collectiblesCount: 0, achievementsCompleted: 0, achievementsTotal: 0
    })
    if (path === "/api/player/profile/99/pond") return fulfill(publicPondOverview(options.publicPondExtraFishCount))
    if (path === "/api/equipment/instances") return fulfill({ items: activePlayerEquipment })
    if (path === "/api/inventory/instances") return fulfill({ items: inventoryItems })
    if (path === "/api/cattery/kitties/77/combat-profile") return fulfill({
      kitty: { id: 77, name: "小满", status: "idle", level: 9, exp: 0, nextExp: 100, totalExp: 0, bondExp: 0, mood: 0.8, createdAt: now },
      attributes: options.kittyAttributes || kittyFishingAttributes,
      stats: {},
      skills: [],
      equipment: activeKittyEquipment
    })
    if (path === "/api/cattery/kitties/77/battle-points") return fulfill({ totalPoints: 0, spentPoints: 0, available: 0 })
    if (path === "/api/cattery/kitties/77/equip" && request.method() === "POST") {
      const body = request.postDataJSON() as { instanceId?: number; slot?: string; equipType?: "work" | "battle" }
      options.onKittyEquipRequest?.(body)
      const instanceID = Number(body.instanceId || 0)
      const slot = String(body.slot || "")
      const source = activePlayerEquipment.find((item) => item.instanceId === instanceID)
      if (source && slot) {
        activePlayerEquipment = activePlayerEquipment.filter((item) => item.instanceId !== instanceID)
        activeKittyEquipment = activeKittyEquipment
          .filter((item) => item.instanceId !== instanceID && item.equippedSlot !== slot)
          .concat({ ...source, equipType: body.equipType || "work", state: "equipped", equippedSlot: slot })
      }
      return fulfill({
        roleEquipmentPatch: {
          roleType: "kitty",
          roleId: 77,
          items: activeKittyEquipment,
          meta: { version: activeKittyEquipment.length }
        }
      })
    }
    if (path === "/api/fishing/starter/claim") return fulfill({ granted: false, grantKey: "fishing_starter_v1", loadoutId: "starter-float-v1", items: {} })
    if (path === "/api/fishing/overview") return fulfill({
      level: playerFishingLevel, anglers: fishingAnglers, waters, spots, species, pondSpecies, mutations: fishingMutations, tackle, ownedTackle: options.ownedTackle || ownedTackle,
      forecast: forecast().map(row => options.scene ? { ...row, now: { ...row.now, gameHour: options.scene.gameHour, weather: options.scene.weather } } : row),
      currentTrip: tripRunning ? { ...currentTrip(liveActor), ...(options.scene?.waterId ? { waterId: options.scene.waterId, spotId: spots.find(spot => spot.waterId === options.scene?.waterId)?.id } : {}) } : null, latestTrip: null,
      spotCounts: { lily_edge: 12, old_willow: 4, gravel_run: 7, deep_bend: 3, granite_shelf: 2, blue_drop: 1 },
      rulesetVersion: "fishing-v1"
    })
    if (path === "/api/fishing/trips/history") {
      options.onFishingHistoryRequest?.()
      return fulfill({ items: options.historyItems ?? fishingTripHistory(), limit: 20 })
    }
    const historyEventsMatch = path.match(/^\/api\/fishing\/trips\/([^/]+)\/events$/)
    if (historyEventsMatch) {
      const tripId = historyEventsMatch[1]
      const cursor = Math.max(0, Number(requestURL.searchParams.get("cursor") || 0))
      options.onFishingHistoryEventRequest?.(tripId, cursor)
      const source = fishingTripEventsByTripId()[tripId as keyof ReturnType<typeof fishingTripEventsByTripId>]
      if (!source) return route.fulfill({ status: 404, contentType: "application/json", body: JSON.stringify({ ok: false }) })
      const candidates = [...source.items]
        .filter((event) => cursor <= 0 || event.sequence < cursor)
        .sort((left, right) => right.sequence - left.sequence)
      const pageItems = candidates.slice(0, 100)
      const nextCursor = candidates.length > 100 ? pageItems[pageItems.length - 1].sequence : 0
      return fulfill({ items: pageItems, total: source.items.length, nextCursor, historyComplete: source.historyComplete })
    }
    if (path === "/api/fishing/loadouts") return fulfill({ items: options.loadouts || [loadout] })
    if (path === "/api/fishing/catches") {
      const cursor = Math.max(0, Number(requestURL.searchParams.get("cursor") || 0))
      const requestedLimit = Math.max(1, Number(requestURL.searchParams.get("limit") || 24))
      const pageSize = Math.min(requestedLimit, Math.max(1, options.catchPageSize || requestedLimit))
      const speciesId = requestURL.searchParams.get("speciesId") || ""
      options.onFishingCatchRequest?.(cursor)
      const items = (requestURL.searchParams.get("kept") === "true"
        ? catchItems.filter((item) => Boolean(item.inventoryInstanceId))
        : catchItems)
        .filter((item) => !speciesId || item.speciesId === speciesId)
        .filter((item) => cursor <= 0 || item.catchId < cursor)
        .sort((left, right) => right.catchId - left.catchId)
      const pageItems = items.slice(0, pageSize)
      const nextCursor = items.length > pageSize ? items[pageSize].catchId + 1 : 0
      return fulfill({ items: pageItems, nextCursor })
    }
    if (path === "/api/fishing/journal") return fulfill({ items: species.map((fish) => ({ speciesId: fish.id, encounters: 4, catches: 2, releases: 1, bestLengthCm: 48.3, bestWeightKg: 2.61 })) })
    if (path === "/api/fishing/leaderboard") return fulfill({ items: [{ rank: 1, userId: 42, playerName: "钓鱼测试员", speciesId: "crucian_carp", value: 2.61 }] })
    if (path === "/api/fishing/tournaments") {
      const standard = options.legacyTournament
        ? { standardRules: { fishingLevel: 20, strategy: "balanced", control: 2.4, sensitivity: 1.8, hooking: 0, maxLoadKg: 9.5 } }
        : {
            standardLoadout: {
              method: "float",
              items: options.tournamentStandardItems || [
                { slot: "rod", itemId: "basic_fishing_rod" },
                { slot: "reel", itemId: "basic_spinning_reel" },
                { slot: "main_line", itemId: "braided_main_line" },
                { slot: "leader", itemId: "clear_leader" },
                { slot: "hook", itemId: "fine_fishing_hook" },
                { slot: "float_or_sinker", itemId: "reed_float" },
                { slot: "bait", itemId: "earthworm_bait" }
              ]
            }
          }
      return fulfill({
        current: {
          id: "weekly-1",
          weekKey: options.legacyTournament ? "2026-08-31" : "2026-09-07",
          targetSpeciesId: "brown_trout",
          rulesetVersion: "fishing-v2",
          startsAt: options.legacyTournament ? now : "2026-09-07T00:00:00Z",
          endsAt: options.legacyTournament ? "2026-09-07T00:00:00Z" : "2026-09-14T00:00:00Z",
          status: "open"
        },
        ...standard,
        openLeaderboard: [], standardLeaderboard: [],
        myEntries: [{ rank: 36, division: "standard", userId: 42, playerName: "钓鱼测试员", catchId: 71, lengthCm: 42.6, weightKg: 1.84 }],
        rewards: [], tokens: 200, catchUpAvailable: true, catchUpCost: 200
      })
    }
    if (path === "/api/fishing/pond" && request.method() === "GET") {
      const pond = ownerPondOverview()
      if (options.pondAvailableFish) {
        pond.availableFish = options.pondAvailableFish.slice(0, 200)
        return fulfill({ ...pond, availableFishNextCursor: options.pondAvailableFish.length > 200 ? pond.availableFish.at(-1)!.instanceId : 0 })
      }
      return fulfill(pond)
    }
    if (path === "/api/fishing/pond/available-fish") {
      const cursor = Number(requestURL.searchParams.get("cursor"))
      const remaining = (options.pondAvailableFish || []).filter((row) => row.instanceId < cursor)
      const items = remaining.slice(0, 200)
      return fulfill({ items, nextCursor: remaining.length > 200 ? items.at(-1)!.instanceId : 0 })
    }
    if (path === "/api/fishing/pond/feed" && request.method() === "POST") {
      const body = request.postDataJSON() as { itemId?: string; quantity?: number }
      options.onPondFeedRequest?.(body)
      const overview = ownerPondOverview()
      const quantity = Math.max(0, Number(body.quantity || 0))
      const nutrition = quantity * Number(overview.feeds.find((feed) => feed.itemId === body.itemId)?.nutrition || 0)
      return fulfill({
        ...overview,
        feeds: overview.feeds.map((feed) => feed.itemId === body.itemId
          ? { ...feed, inventoryCount: Math.max(0, feed.inventoryCount - quantity), storedNutrition: feed.storedNutrition + nutrition }
          : feed),
        feedTotalNutrition: overview.feedTotalNutrition + nutrition,
        events: [{ id: 2, kind: "fed", details: { itemId: body.itemId, quantity, nutrition }, createdAt: now }, ...overview.events]
      })
    }
    if (path === "/api/fishing/pond/fish/batch" && request.method() === "POST") {
      const body = request.postDataJSON() as { instanceIds?: number[] }
      const stockedIDs = new Set(body.instanceIds || [])
      catchItems = catchItems.filter((item) => !item.inventoryInstanceId || !stockedIDs.has(item.inventoryInstanceId))
      return fulfill({
        ...ownerPondOverview(),
        fish: [...ownerPondOverview().fish, ownerPondOverview().availableFish[0]],
        availableFish: []
      })
    }
    if (/^\/api\/fishing\/pond\/fish\/\d+\/move-to-main$/.test(path) && request.method() === "POST") {
      const instanceID = Number(path.split("/")[5])
      const overview = ownerPondOverview()
      const moved = overview.fish.find((row) => row.instanceId === instanceID)
      return fulfill({
        ...overview,
        fish: overview.fish.map((row) => row.instanceId === instanceID ? { ...row, fish: { ...row.fish, pondZone: "main" } } : row),
        biomassKg: overview.biomassKg + Number(moved?.fish.weightKg || 0),
        events: [{ id: 2, kind: "moved_to_main", fishInstanceId: instanceID, details: { speciesId: moved?.fish.speciesId }, createdAt: now }, ...overview.events]
      })
    }
    if (path === "/api/items/rename" && request.method() === "POST") {
      const body = request.postDataJSON() as { instanceId?: number; name?: string; labelColor?: string }
      options.onPondRenameRequest?.(body)
      return fulfill({
        ok: true,
        instance: {
          instanceId: Number(body.instanceId || 0),
          templateItemId: "fish_catch",
          quantity: 1,
          state: "fishing_pond",
          customData: { name: String(body.name || ""), labelColor: String(body.labelColor || "") }
        },
        cost: { harmonic_embryo: 1 },
        labelColors: ["gold", "rose", "violet", "amber", "sky", "emerald", "fuchsia"]
      })
    }
    if (path === "/api/fishing/quote") {
      const body = request.postDataJSON() as { instanceIds?: number[] }
      return fulfill(fishingSaleQuote(
        fishingSaleAttempts > 0 ? 4101 : 4093,
        options.fishingSaleQuoteItemCount || body.instanceIds?.length,
        options.fishingSaleQuoteMutations,
        body.instanceIds
      ))
    }
    if (path === "/api/fishing/sell" && request.method() === "POST") {
      const body = request.postDataJSON() as { instanceIds?: number[] }
      options.onFishingSaleRequest?.(body)
      if (options.fishingSaleQuoteChangesOnce && fishingSaleAttempts === 0) {
        fishingSaleAttempts++
        return route.fulfill({
          status: 409,
          contentType: "application/json",
          body: JSON.stringify({ ok: false, ts: Date.now(), error: { code: "FISHING_SALE_QUOTE_CHANGED", message: "sale quote changed" } })
        })
      }
      fishingSaleAttempts++
      const soldIDs = new Set(body.instanceIds || [])
      catchItems = catchItems.filter((item) => !item.inventoryInstanceId || !soldIDs.has(item.inventoryInstanceId))
      return fulfill(fishingSaleQuote(
        fishingSaleAttempts > 1 ? 4101 : 4093,
        options.fishingSaleQuoteItemCount || body.instanceIds?.length,
        options.fishingSaleQuoteMutations,
        body.instanceIds
      ))
    }
    if (path === "/api/actions/queue/batch") {
      onTripRequest?.(request.postDataJSON())
      if (tripFailure) {
        return route.fulfill({
          status: 409,
          contentType: "application/json",
          body: JSON.stringify({
            ok: false,
            ts: Date.now(),
            error: {
              code: tripFailure.code,
              message: "fishing trip could not start",
              instanceState: tripFailure.instanceState
            }
          })
        })
      }
      return fulfill({})
    }
    if (path === "/api/fishing/trips/stop") {
      tripRunning = false
      onStopRequest?.()
      return fulfill({ stopped: true })
    }
    return fulfill({})
  })
}

async function expectNoHorizontalOverflow(page: Page) {
  await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth <= document.documentElement.clientWidth)).toBe(true)
}

async function dismissBuildNotice(page: Page) {
  const notice = page.locator(".el-notification").filter({ hasText: "测试版本提示" })
  await notice.waitFor({ state: "visible", timeout: 5_000 }).catch(() => undefined)
  if (await notice.isVisible()) {
    await notice.locator(".el-notification__closeBtn").click()
    await notice.waitFor({ state: "hidden", timeout: 3_000 })
  }
}

test("fishing is a standalone menu group and its tutorial reaches every workspace", async ({ page }) => {
  await page.setViewportSize({ width: 1440, height: 1000 })
  await mockFishing(page)
  await page.goto("/game/fishing")
  await dismissBuildNotice(page)

  const sidebar = page.locator(".game-shell-view__sidebar:not(.game-shell-mobile-icon-rail)")
  const fishingMenuItem = sidebar.locator(".el-menu > .el-menu-item").filter({ hasText: "钓鱼" })
  const pondMenuItem = sidebar.locator(".el-menu > .el-menu-item").filter({ hasText: "池塘" })
  const alchemyMenuItem = sidebar.locator(".el-menu > .el-menu-item").filter({ hasText: "炼金" })
  const collectingMenuItem = sidebar.locator(".el-menu > .el-menu-item").filter({ hasText: "采集" })
  await expect(fishingMenuItem).toHaveCount(1)
  await expect(pondMenuItem).toHaveCount(1)
  await expect(alchemyMenuItem).toHaveCount(1)
  await expect(collectingMenuItem).toHaveCount(1)
  const [alchemyTop, fishingTop, pondTop, collectingTop] = await Promise.all([
    alchemyMenuItem.evaluate((element) => element.getBoundingClientRect().top),
    fishingMenuItem.evaluate((element) => element.getBoundingClientRect().top),
    pondMenuItem.evaluate((element) => element.getBoundingClientRect().top),
    collectingMenuItem.evaluate((element) => element.getBoundingClientRect().top),
  ])
  expect(alchemyTop).toBeLessThan(fishingTop)
  expect(fishingTop).toBeLessThan(pondTop)
  expect(pondTop).toBeLessThan(collectingTop)
  await expect(fishingMenuItem.locator("xpath=preceding-sibling::*[1]")).toHaveClass(/el-divider/)
  await expect(fishingMenuItem.locator("xpath=following-sibling::*[1]")).toContainText("池塘")
  await expect(pondMenuItem.locator("xpath=following-sibling::*[1]")).toHaveClass(/el-divider/)

  await page.getByRole("button", { name: "教程", exact: true }).click()
  const popover = page.locator(".driver-popover")
  const steps = [
    ["钓鱼总览", "[data-tutorial='fishing-page']"],
    ["钓鱼的五个入口", "[data-tutorial='fishing-tabs']"],
    ["先读水情，再选钓点", "[data-tutorial='fishing-waters']"],
    ["安排一次钓行", "[data-tutorial='fishing-trip-start']"],
    ["先配好完整钓组", "[data-tutorial='fishing-loadouts']"],
    ["渔获、图鉴与纪录", "[data-tutorial='fishing-catches']"],
    ["回看近期钓行", "[data-tutorial='fishing-history']"],
    ["每周目标鱼赛", "[data-tutorial='fishing-tournaments']"],
  ] as const
  for (let index = 0; index < steps.length; index += 1) {
    const [title, selector] = steps[index]
    await expect(popover).toContainText(title)
    await expect(page.locator(selector)).toBeVisible()
    await popover.getByRole("button", { name: index === steps.length - 1 ? "完成" : "下一步", exact: true }).click()
  }
  await expect(popover).toBeHidden()
})

test("trip history loads lazily, stays newest first, and expands on desktop and mobile", async ({ page }, testInfo) => {
  await page.setViewportSize({ width: 1280, height: 900 })
  let historyRequests = 0
  const historyEventRequests: Array<{ tripId: string; cursor: number }> = []
  await mockFishing(page, false, undefined, [], undefined, undefined, [], {
    onFishingHistoryRequest: () => { historyRequests += 1 },
    onFishingHistoryEventRequest: (tripId, cursor) => { historyEventRequests.push({ tripId, cursor }) }
  })
  await page.goto("/game/fishing")
  await dismissBuildNotice(page)

  expect(historyRequests).toBe(0)
  expect(historyEventRequests).toHaveLength(0)
  await page.getByRole("tab", { name: "记录", exact: true }).click()
  await expect.poll(() => historyRequests).toBe(1)
  expect(historyEventRequests).toHaveLength(0)
  await expect(page.getByRole("heading", { name: "最近 20 次钓行" })).toBeVisible()

  const rows = page.locator(".fishing-history-trip")
  await expect(rows).toHaveCount(2)
  await expect(rows.nth(0)).toContainText("睡莲边")
  await expect(rows.nth(0)).toContainText("正常收竿")
  await expect(rows.nth(0)).toContainText("12 尾上岸")
  await expect(rows.nth(0)).toContainText("9.84 kg")
  await expect(rows.nth(0).locator(".fishing-history-trip__status svg")).toHaveCount(1)
  await expect(rows.nth(1)).toContainText("老柳树下")
  await expect(rows.nth(1)).toContainText("小满")
  await expect(rows.nth(1)).toContainText("提前收竿")

  await rows.nth(0).locator("summary").click()
  await expect(rows.nth(0).locator(".fishing-history-trip__body")).toBeVisible()
  await expect.poll(() => historyEventRequests).toEqual([{ tripId: "trip-history-new", cursor: 0 }])
  await expect(rows.nth(0).locator(".fishing-history-stats dt")).toHaveText(["抛竿", "鱼口", "上岸", "已保留", "放流", "跑鱼"])
  await expect(rows.nth(0).locator(".fishing-history-stats dd")).toHaveText(["25", "25", "12", "6", "6", "13"])
  await expect(rows.nth(0).locator(".fishing-history-events li").first()).toContainText("本次钓行结束")
  await expect(rows.nth(0).locator(".fishing-history-events > header")).toContainText("已加载 100 / 101 条")
  await rows.nth(0).getByRole("button", { name: "查看更早过程" }).click()
  await expect.poll(() => historyEventRequests).toEqual([
    { tripId: "trip-history-new", cursor: 0 },
    { tripId: "trip-history-new", cursor: 2 }
  ])
  await expect(rows.nth(0).locator(".fishing-history-events li")).toHaveCount(101)
  await expect(rows.nth(0).locator(".fishing-history-events > header")).toContainText("完整记录 · 101 条")
  await expect(rows.nth(0).locator(".fishing-history-events img").first()).toBeVisible()

  await rows.nth(1).locator("summary").click()
  await expect.poll(() => historyEventRequests).toContainEqual({ tripId: "trip-history-old", cursor: 0 })
  await expect(rows.nth(1).locator(".fishing-history-events > header")).toContainText("部分记录 · 保留最近 2 条")
  await expectNoHorizontalOverflow(page)
  await page.screenshot({ path: testInfo.outputPath("fishing-history-desktop.png"), fullPage: true })

  await page.setViewportSize({ width: 390, height: 844 })
  await expect(rows.nth(0).locator(".fishing-history-trip__result")).toBeVisible()
  await expect(rows.nth(0).locator(".fishing-history-stats")).toBeVisible()
  await expectNoHorizontalOverflow(page)
  await page.screenshot({ path: testInfo.outputPath("fishing-history-mobile.png"), fullPage: true })
})

test("pond tutorial explains accelerated time and each management area", async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 900 })
  await mockFishing(page)
  await page.goto("/game/pond")
  await dismissBuildNotice(page)

  const pondTutorialButton = page.getByRole("button", { name: "教程", exact: true })
  await expect(pondTutorialButton).toHaveClass(/el-button--success/)
  await pondTutorialButton.click()
  const pondTutorial = page.locator(".driver-popover")
  const pondTutorialSteps = [
    ["池塘怎么运转", "[data-tutorial='pond-page']"],
    ["现实时间与生物年龄", "[data-tutorial='pond-status']"],
    ["从池面观察鱼群", "[data-tutorial='pond-water']"],
    ["逐尾查看状态", "[data-tutorial='pond-residents']"],
    ["按食性准备饲料", "[data-tutorial='pond-feed']"],
    ["挑选亲鱼进行繁育", "[data-tutorial='pond-breeding']"],
    ["用记录确认变化", "[data-tutorial='pond-events']"],
  ] as const
  for (let index = 0; index < pondTutorialSteps.length; index += 1) {
    const [title, selector] = pondTutorialSteps[index]
    await expect(pondTutorial).toContainText(title)
    await expect(page.locator(selector)).toBeVisible()
    if (index === 1) await expect(pondTutorial).toContainText("现实 1 天 = 生物成长 365 天")
    await pondTutorial.getByRole("button", { name: index === pondTutorialSteps.length - 1 ? "完成" : "下一步", exact: true }).click()
  }
  await expect(pondTutorial).toBeHidden()
})

test("pond garden stays compact and readable across desktop and mobile", async ({ page }, testInfo) => {
  await mockFishing(page)
  const overview = pondOverview()
  const mainFish = overview.fish.filter(row => row.fish.pondZone === "main")
  overview.fish = Array.from({ length: 9 }, (_, index) => ({
    ...mainFish[index % mainFish.length],
    instanceId: 9000 + index,
    customData: { name: index === 0 ? "池塘里的长名字测试鱼" : `池鱼${index + 1}` },
    fish: { ...mainFish[index % mainFish.length].fish, genomeRevealed: index === 0 }
  }))
  overview.events = Array.from({ length: 12 }, (_, index) => ({ ...overview.events[0], id: `garden-event-${index}` }))
  await page.route("**/api/fishing/pond", route => route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ ok: true, data: overview }) }))
  await page.setViewportSize({ width: 1440, height: 1000 })
  await page.goto("/game/pond")
  await dismissBuildNotice(page)
  const garden = page.locator(".pond-water__garden")
  await expect(garden).toBeVisible()
  expect(await garden.evaluate(element => {
    const image = element as HTMLImageElement
    return image.complete && image.naturalWidth === 1536
  })).toBe(true)
  const renderBudget = await page.locator(".pond-water canvas").evaluate(async element => {
    const canvas = element as HTMLCanvasElement
    const gl = canvas.getContext("webgl")!
    const original = gl.drawArrays
    let draws = 0
    let hasWaterPixels = false
    let attributeWrites = 0
    const scene = canvas.closest(".pond-water")!
    const observer = new MutationObserver(records => { attributeWrites += records.length })
    observer.observe(scene, { attributes: true, subtree: true, attributeFilter: ["style", "class"] })
    gl.drawArrays = function(...args: Parameters<typeof original>) {
      original.apply(this, args)
      if (++draws === 1) {
        const pixels = new Uint8Array(canvas.width * canvas.height * 4)
        gl.readPixels(0, 0, canvas.width, canvas.height, gl.RGBA, gl.UNSIGNED_BYTE, pixels)
        hasWaterPixels = pixels.some((value, index) => index % 4 === 3 && value > 0)
      }
    }
    await new Promise(resolve => setTimeout(resolve, 1100))
    observer.disconnect()
    gl.drawArrays = original
    const listAnimations = document.querySelector(".pond-fish-list")!.getAnimations({ subtree: true }).filter(animation => animation.playState === "running").length
    return { draws, hasWaterPixels, pixels: canvas.width * canvas.height, attributeWrites, listAnimations }
  })
  expect(renderBudget.draws).toBeGreaterThan(0)
  expect(renderBudget.draws).toBeLessThanOrEqual(14)
  expect(renderBudget.hasWaterPixels).toBe(true)
  expect(renderBudget.pixels).toBeLessThanOrEqual(960 * 540)
  expect(renderBudget.listAnimations).toBe(0)
  await testInfo.attach("pond-render-budget", { body: JSON.stringify(renderBudget), contentType: "application/json" })
  console.info("Pond render budget:", JSON.stringify(renderBudget))
  const cards = page.locator(".pond-fish-list > article")
  await expect(cards).toHaveCount(9)
  expect((await cards.nth(1).boundingBox())!.height).toBeLessThan(205)
  const genes = cards.first().locator(".pond-genome-details")
  await expect(genes).not.toHaveAttribute("open")
  await genes.locator("summary").click()
  await expect(genes.locator(".pond-genes")).toBeVisible()
  await genes.locator("summary").press("Enter")
  await expect(genes).not.toHaveAttribute("open")
  const journal = page.locator(".pond-events ol")
  expect(await journal.evaluate(element => element.scrollHeight > element.clientHeight)).toBe(true)
  await journal.focus()
  await journal.press("End")
  await expect.poll(() => journal.evaluate(element => element.scrollTop)).toBeGreaterThan(0)
  const fish = page.locator(".pond-fish-sprite").first()
  await page.setViewportSize({ width: 390, height: 844 })
  await cards.last().scrollIntoViewIfNeeded()
  await expect(page.locator(".pond-water")).toHaveClass(/is-paused/)
  const pausedPosition = await fish.getAttribute("style")
  await page.waitForTimeout(250)
  expect(await fish.getAttribute("style")).toBe(pausedPosition)
  await page.setViewportSize({ width: 1440, height: 1000 })
  await page.locator(".pond-water").scrollIntoViewIfNeeded()
  await expect(page.locator(".pond-water")).not.toHaveClass(/is-paused/)
  const start = await fish.evaluate(element => (element as HTMLElement).style.transform)
  await expect.poll(() => fish.evaluate(element => (element as HTMLElement).style.transform)).not.toBe(start)
  expect(await fish.evaluate(element => (element as HTMLElement).style.getPropertyValue("--fish-tilt"))).toMatch(/deg$/)
  await page.mouse.move(0, 0)
  await page.locator(".pond-water").scrollIntoViewIfNeeded()
  await expectNoHorizontalOverflow(page)
  await page.screenshot({ path: testInfo.outputPath("pond-garden-desktop.png"), fullPage: true })
  await page.evaluate(() => document.documentElement.classList.add("dark"))
  await page.waitForTimeout(350)
  await page.screenshot({ path: testInfo.outputPath("pond-garden-dark.png"), fullPage: true })
  await page.evaluate(() => document.documentElement.classList.remove("dark"))
  await page.waitForTimeout(350)
  for (const width of [390, 360, 1920]) {
    await page.setViewportSize({ width, height: width < 720 ? 844 : 1080 })
    await page.locator(".pond-water").scrollIntoViewIfNeeded()
    await expectNoHorizontalOverflow(page)
    expect(await cards.evaluateAll(elements => elements.every(element => element.scrollWidth <= element.clientWidth + 1))).toBe(true)
    await page.screenshot({ path: testInfo.outputPath(`pond-garden-${width}.png`), fullPage: true })
    if (width < 720) {
      await cards.first().scrollIntoViewIfNeeded()
      await genes.locator("summary").click()
      await expect(genes.locator(".pond-genes")).toBeVisible()
      await expectNoHorizontalOverflow(page)
      await page.screenshot({ path: testInfo.outputPath(`pond-garden-residents-${width}.png`) })
      await genes.locator("summary").click()
    }
  }
})

test("pond garden survives unavailable WebGL and respects reduced motion", async ({ page }, testInfo) => {
  await page.emulateMedia({ reducedMotion: "reduce" })
  await page.addInitScript(() => {
    const original = HTMLCanvasElement.prototype.getContext
    HTMLCanvasElement.prototype.getContext = function(type: string, ...args: unknown[]) {
      if (type === "webgl") return null
      return original.apply(this, [type, ...args] as Parameters<typeof original>)
    } as typeof original
  })
  await mockFishing(page)
  await page.goto("/game/pond")
  await dismissBuildNotice(page)
  await expect(page.locator(".pond-water")).toHaveClass(/is-fallback/)
  await expect(page.locator(".pond-water__garden")).toBeVisible()
  const sprite = page.locator(".pond-fish-sprite").first()
  const before = await sprite.getAttribute("style")
  await page.waitForTimeout(250)
  expect(await sprite.getAttribute("style")).toBe(before)
  expect(await sprite.locator(".pond-fish-sprite__art").evaluate(element => getComputedStyle(element).animationName)).toBe("none")
  await page.locator(".pond-water").screenshot({ path: testInfo.outputPath("pond-garden-fallback.png") })
})

test("pond is a standalone page with named fish tooltip and camera tracking", async ({ page }, testInfo) => {
  await page.setViewportSize({ width: 1280, height: 900 })
  await mockFishing(page)
  await page.goto("/game/fishing")
  await dismissBuildNotice(page)
  await expect(page.getByRole("tab")).toHaveCount(5)
  await expect(page.getByRole("tab", { name: "鱼池" })).toHaveCount(0)

  await page.goto("/game/pond")
  await dismissBuildNotice(page)
  await expect(page.locator("main").getByText("池塘", { exact: true }).first()).toBeVisible()
  await expect(page.locator(".pond-water canvas")).toBeVisible()
  await expect(page.locator(".pond-rhythm")).toHaveCount(0)
  const pondTutorialButton = page.getByRole("button", { name: "教程", exact: true })
  await expect(pondTutorialButton).toHaveClass(/el-button--success/)
  await expect(page.locator(".pond-feed-list .game-icon-atlas")).toBeVisible()
  await dismissBuildNotice(page)
  await page.screenshot({ path: "../output/playwright/pond-overview-desktop.png", fullPage: true })
  const firstFishSprite = page.locator(".pond-fish-sprite").first()
  const firstFishArt = firstFishSprite.locator(".pond-fish-sprite__art")
  const firstFishBody = firstFishSprite.locator(".pond-fish-sprite__body")
  const firstFishTail = firstFishSprite.locator(".pond-fish-sprite__tail")
  await expect(firstFishArt).toBeVisible()
  await expect(firstFishBody).toBeVisible()
  await expect(firstFishTail).toBeVisible()
  const tailMotion = await firstFishTail.evaluate((element) => {
    const style = getComputedStyle(element)
    return { animationName: style.animationName, animationDuration: style.animationDuration, clipPath: style.clipPath, rotate: style.rotate }
  })
  expect(tailMotion.animationName).toContain("pond-tail-swim")
  expect(tailMotion.animationDuration).not.toBe("0s")
  expect(tailMotion.clipPath).not.toBe("none")
  await expect.poll(() => firstFishTail.evaluate((element) => getComputedStyle(element).rotate)).not.toBe(tailMotion.rotate)
  expect(await firstFishArt.evaluate((element) => getComputedStyle(element).transitionDuration)).not.toBe("0s")
  const readSwimMotion = () => firstFishSprite.evaluate((element) => element.classList.contains("is-kicking") ? "kicking" : "gliding")
  const initialSwimMotion = await readSwimMotion()
  await expect.poll(readSwimMotion, { timeout: 5_000 }).not.toBe(initialSwimMotion)
  const interactionAction = page.getByRole("button", { name: "互动", exact: true })
  const interactionActionBox = await interactionAction.boundingBox()
  expect(interactionActionBox?.width || 0).toBeLessThanOrEqual(44)
  await expect(interactionAction.locator("svg")).toHaveCount(1)
  const lightInteractionBackground = await interactionAction.evaluate((element) => getComputedStyle(element).backgroundColor)
  await page.evaluate(() => document.documentElement.classList.add("dark"))
  const darkInteractionBackground = await interactionAction.evaluate((element) => getComputedStyle(element).backgroundColor)
  expect(darkInteractionBackground).not.toBe(lightInteractionBackground)
  await page.evaluate(() => document.documentElement.classList.remove("dark"))
  await expect(page.locator(".pond-ecology-alert")).toHaveCount(0)
  await expect(page.locator(".pond-fish-sprite__ecology")).toHaveCount(1)
  await expect(page.getByRole("button", { name: "跟踪深湾" }).locator(".pond-fish-sprite__ecology")).toBeVisible()
  const residentCards = page.locator(".pond-fish-list > article")
  await expect(page.locator('.pond-fish-sprite[data-fish-mutations~="prismatic"]')).toHaveCount(2)
  await expect(page.locator('.pond-fish-sprite[data-fish-mutations~="rainbow"]')).toHaveCount(2)
  await expect(page.locator('.pond-fish-sprite[data-fish-mutations~="shimmering"]')).toHaveCount(2)
  await expect(page.locator(".pond-fish-sprite.has-stacked-fish-mutations")).toHaveCount(1)
  const pondPrism = page.locator(".pond-fish-sprite.fish-mutation--prismatic .pond-fish-sprite__prism").first()
  const pondRainbow = page.locator(".pond-fish-sprite.fish-mutation--rainbow .pond-fish-sprite__rainbow").first()
  const pondGlints = page.locator(".pond-fish-sprite.fish-mutation--shimmering .pond-fish-sprite__glints").first()
  await expect(pondPrism).toBeVisible()
  await expect(pondRainbow).toBeVisible()
  await expect(pondGlints).toBeVisible()
  const [pondPrismMotion, pondRainbowMotion] = await Promise.all([pondPrism, pondRainbow].map((locator) => locator.evaluate((element) => {
    const style = getComputedStyle(element)
    return { name: style.animationName, timing: style.animationTimingFunction, iterations: style.animationIterationCount }
  })))
  expect(pondPrismMotion).toMatchObject({ timing: "linear", iterations: "infinite" })
  expect(pondPrismMotion.name).toContain("fish-prismatic-flow")
  expect(pondRainbowMotion).toMatchObject({ timing: "linear", iterations: "infinite" })
  expect(pondRainbowMotion.name).toContain("fish-rainbow-flow")
  await expect(pondGlints.locator("i")).toHaveCount(5)
  expect(await pondGlints.locator("i").evaluateAll((elements) => elements.every((element) => Number(getComputedStyle(element).opacity) >= .75))).toBe(true)
  await expect(residentCards.filter({ hasText: "小金" }).locator(".pond-mutation-tag").getByText("棱彩", { exact: true })).toBeVisible()
  await expect(residentCards.filter({ hasText: "小金" }).locator('[data-fish-size-metric="length"]')).toHaveAttribute("data-arrow-count", "1")
  await expect(residentCards.filter({ hasText: "小金" }).locator('[data-fish-size-metric="weight"]')).toHaveAttribute("data-arrow-count", "3")
  await expect(residentCards.filter({ hasText: "幼鲤" }).locator(".fish-size-indicator")).toHaveCount(0)
  await expect(residentCards.filter({ hasText: "小金" }).locator(".pond-genes__mutations")).toContainText("异变基因")
  await expect(residentCards.filter({ hasText: "小金" }).locator(".pond-genes__mutations")).toContainText("棱彩")
  await expect(residentCards.filter({ hasText: "赤尾" }).getByText("彩虹", { exact: true })).toBeVisible()
  await expect(residentCards.filter({ hasText: "幼鲤" }).getByText("闪光", { exact: true })).toBeVisible()
  await expect(residentCards.filter({ hasText: "深湾" })).toHaveClass(/has-stacked-fish-mutations/)
  await page.screenshot({ path: "../output/playwright/pond-mutations-desktop.png", fullPage: true })
  await expect(residentCards.filter({ hasText: "小金" }).getByText("已成熟", { exact: true })).toBeVisible()
  await expect(residentCards.filter({ hasText: "幼鲤" }).getByText("未成熟", { exact: true })).toBeVisible()
  await residentCards.filter({ hasText: "幼鲤" }).getByText("未成熟", { exact: true }).hover()
  await expect(page.locator(".el-popper").filter({ hasText: "成熟进度 67%" })).toBeVisible()
  await residentCards.filter({ hasText: "小金" }).getByText("省饲料", { exact: true }).hover()
  await expect(page.locator(".el-popper").filter({ hasText: "有效营养 +8%" })).toBeVisible()
  await expect(residentCards.filter({ hasText: "深湾" }).getByText("捕食性", { exact: true })).toBeVisible()
  await expect(residentCards.getByText("可能成为猎物", { exact: true })).toHaveCount(0)
  await expect(page.locator(".pond-events")).toContainText("深湾 捕食了 小银")
  await page.locator(".pond-fish-list").screenshot({ path: testInfo.outputPath("pond-maturity-statuses.png") })
  const namedFish = page.getByRole("button", { name: "跟踪小金" })
  await expect(namedFish).toBeVisible()
  await namedFish.hover({ force: true })
  const namedFishTooltip = page.locator(".pond-water__tooltip")
  await expect(namedFishTooltip).toContainText("小金")
  await expect(namedFishTooltip).toContainText("莲纹锦鲤")
  await expect(namedFishTooltip.locator("strong")).toHaveClass(/item-custom-label--gold/)
  const predatorFish = page.getByRole("button", { name: "跟踪深湾" })
  await predatorFish.hover({ force: true })
  await expect(page.locator(".pond-water__tooltip")).toContainText("饱食度低于 70")
  await namedFish.click({ force: true })
  const trackingStatus = page.locator(".pond-water__tracking")
  await expect(trackingStatus).toContainText("小金")
  await expect(trackingStatus.locator("strong")).toHaveClass(/item-custom-label--gold/)
  await page.screenshot({ path: testInfo.outputPath("pond-desktop-tracking.png"), fullPage: true })
  await page.getByRole("button", { name: "退出跟踪" }).click()
  await expect(page.locator(".pond-water__tracking")).toHaveCount(0)
  await expect(page.getByRole("button", { name: "显现基因" }).first()).toBeDisabled()
  await expectNoHorizontalOverflow(page)
  await page.setViewportSize({ width: 390, height: 844 })
  await expect(residentCards.filter({ hasText: "深湾" }).getByText("棱彩", { exact: true })).toBeVisible()
  await expect(residentCards.filter({ hasText: "深湾" }).getByText("彩虹", { exact: true })).toBeVisible()
  await expect(residentCards.filter({ hasText: "深湾" }).getByText("闪光", { exact: true })).toBeVisible()
  await expectNoHorizontalOverflow(page)
  await page.screenshot({ path: "../output/playwright/pond-mutations-mobile.png", fullPage: true })
})

test("pond nursery switches cleanly and moves fry into the main pond", async ({ page }, testInfo) => {
  await page.setViewportSize({ width: 1280, height: 900 })
  await mockFishing(page)
  await page.goto("/game/pond")
  await dismissBuildNotice(page)

  const switcher = page.getByTestId("pond-zone-switch")
  await expect(switcher).toContainText("主池 4/12")
  await expect(switcher).toContainText("鱼苗池 1/12")
  await expect(page.locator(".pond-fish-list > article")).toHaveCount(4)
  await expect(page.getByText("小苗", { exact: true })).toHaveCount(0)

  await switcher.getByText("鱼苗池 1/12", { exact: true }).click()
  await expect(page.locator(".pond-fish-list > article")).toHaveCount(1)
  await expect(page.getByText("小苗", { exact: true })).toBeVisible()
  await expect(page.getByText("无捕食，鱼只成熟后停止长大", { exact: true })).toBeVisible()
  await expect(page.getByText("主池与鱼苗池共用同一料仓", { exact: true })).toBeVisible()
  const moveButton = page.getByRole("button", { name: "转入主池" })
  await expect(moveButton).toBeVisible()
  await expect(moveButton.locator("svg")).toHaveCount(1)
  await expect(page.locator(".pond-fish-sprite")).toHaveCount(1)
  await page.screenshot({ path: testInfo.outputPath("pond-nursery-desktop.png"), fullPage: true })

  await page.setViewportSize({ width: 390, height: 844 })
  await expectNoHorizontalOverflow(page)
  await page.screenshot({ path: testInfo.outputPath("pond-nursery-mobile.png"), fullPage: true })
  await moveButton.click()
  await expect(page.getByText("鱼只已转入主池", { exact: true })).toBeVisible()
  await expect(page.getByText("鱼苗池里还没有小鱼", { exact: true })).toBeVisible()
  await switcher.getByText("主池 5/12", { exact: true }).click()
  await expect(page.getByText("小苗", { exact: true })).toBeVisible()
  await expectNoHorizontalOverflow(page)
})

test("public player pond preserves mutation effects without owner actions", async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 900 })
  await mockFishing(page, false, undefined, [], undefined, undefined, [], { publicPondExtraFishCount: 8 })
  await page.goto("/game/fishing")
  await dismissBuildNotice(page)
  await page.evaluate(async () => {
    const { useUserProfileDialogStore } = await import("/src/stores/user-profile-dialog.ts")
    useUserProfileDialogStore().open(99)
  })

  const profileDialog = page.getByRole("dialog", { name: "玩家信息" })
  await expect(profileDialog).toContainText("湖畔旅人")
  await profileDialog.getByRole("tab", { name: "池塘" }).click()
  const publicPond = profileDialog.locator(".profile-pond")
  await expect(publicPond.locator('[data-testid="public-pond-water"]')).toBeVisible()
  await expect(publicPond.locator('.pond-fish-sprite[data-fish-mutations~="prismatic"]')).toHaveCount(2)
  await expect(publicPond.locator(".pond-fish-sprite.has-stacked-fish-mutations")).toHaveCount(1)
  await expect(publicPond.getByText("棱彩", { exact: true }).first()).toBeVisible()
  await expect(publicPond.getByText("彩虹", { exact: true }).first()).toBeVisible()
  await expect(publicPond.getByText("闪光", { exact: true }).first()).toBeVisible()
  const namedCard = publicPond.locator(".profile-pond__fish-list article").filter({ hasText: "小金" })
  await expect(namedCard.getByText("小金", { exact: true })).toBeVisible()
  await expect(namedCard.getByText("莲纹锦鲤", { exact: true })).toBeVisible()
  const publicLengthIndicator = namedCard.locator('[data-fish-size-metric="length"]')
  const publicWeightIndicator = namedCard.locator('[data-fish-size-metric="weight"]')
  await expect(publicLengthIndicator).toHaveAttribute("data-arrow-count", "1")
  await expect(publicLengthIndicator.locator("svg")).toHaveCount(1)
  await expect(publicWeightIndicator).toHaveAttribute("data-arrow-count", "3")
  await expect(publicWeightIndicator.locator("svg")).toHaveCount(3)
  const lengthIndicatorColor = await publicLengthIndicator.evaluate((element) => getComputedStyle(element).color)
  const weightIndicatorColor = await publicWeightIndicator.evaluate((element) => getComputedStyle(element).color)
  expect(lengthIndicatorColor).toBe("rgb(40, 118, 200)")
  expect(weightIndicatorColor).toBe(lengthIndicatorColor)
  await publicLengthIndicator.hover()
  const sizeTooltip = page.locator(".el-popper").filter({ hasText: "体长高于该鱼种标准 7% · 标准 45.3 cm" })
  await expect(sizeTooltip).toBeVisible()
  await page.mouse.move(5, 5)
  await expect(sizeTooltip).toBeHidden()
  await expect(publicPond.locator(".profile-pond__fish-list article").filter({ hasText: "幼鲤" }).locator(".fish-size-indicator")).toHaveCount(0)
  await expect(publicPond.locator(".profile-pond__bars")).toHaveCount(0)
  await expect(publicPond.getByRole("button", { name: "投喂" })).toHaveCount(0)
  await expect(publicPond.getByRole("button", { name: "放入鱼只" })).toHaveCount(0)
  await publicPond.getByRole("button", { name: "跟踪小金" }).hover({ force: true })
  await expect(publicPond.locator(".pond-water__tooltip")).toContainText("小金")
  await expect(publicPond.locator(".pond-water__tooltip")).not.toContainText("健康")
  const desktopColumns = await publicPond.locator(".profile-pond__fish-list").evaluate((element) => getComputedStyle(element).gridTemplateColumns.trim().split(/\s+/).length)
  expect(desktopColumns).toBe(2)
  const desktopLayout = await profileDialog.evaluate((element) => {
    const body = element.querySelector<HTMLElement>(".el-dialog__body")
    const list = element.querySelector<HTMLElement>(".profile-pond__fish-list")
    const cards = Array.from(list?.querySelectorAll<HTMLElement>("article") || [])
    const listRect = list?.getBoundingClientRect()
    return {
      bodyClientWidth: body?.clientWidth || 0,
      bodyScrollWidth: body?.scrollWidth || 0,
      cardsInsideList: Boolean(listRect) && cards.every((card) => {
        const rect = card.getBoundingClientRect()
        return rect.left >= listRect.left - 1 && rect.right <= listRect.right + 1
      })
    }
  })
  expect(desktopLayout.bodyScrollWidth).toBeLessThanOrEqual(desktopLayout.bodyClientWidth)
  expect(desktopLayout.cardsInsideList).toBe(true)
  await page.screenshot({ path: "../output/playwright/public-pond-mutations-desktop.png", fullPage: true })
  await page.setViewportSize({ width: 1084, height: 658 })
  const stackedMutationCard = publicPond.locator(".profile-pond__fish-list article.has-stacked-fish-mutations")
  await stackedMutationCard.scrollIntoViewIfNeeded()
  await stackedMutationCard.getByText("彩虹", { exact: true }).hover()
  const mutationTooltip = page.locator(".profile-pond-mutation-popper")
  await expect(mutationTooltip).toBeVisible()
  const tooltipLayout = await mutationTooltip.evaluate((element) => {
    const rect = element.getBoundingClientRect()
    return { left: rect.left, right: rect.right, width: rect.width, viewportWidth: window.innerWidth }
  })
  expect(tooltipLayout.width).toBeLessThanOrEqual(321)
  expect(tooltipLayout.left).toBeGreaterThanOrEqual(0)
  expect(tooltipLayout.right).toBeLessThanOrEqual(tooltipLayout.viewportWidth)
  await expect(mutationTooltip).toHaveCSS("opacity", "1")
  await page.waitForTimeout(150)
  await page.screenshot({ path: "../output/playwright/public-pond-layout-fixed.png" })
  await page.mouse.move(5, 5)
  await expect(mutationTooltip).toBeHidden()
  await expectNoHorizontalOverflow(page)
  await page.setViewportSize({ width: 390, height: 844 })
  await stackedMutationCard.scrollIntoViewIfNeeded()
  await expect(stackedMutationCard).toBeVisible()
  const mobileColumns = await publicPond.locator(".profile-pond__fish-list").evaluate((element) => getComputedStyle(element).gridTemplateColumns.trim().split(/\s+/).length)
  expect(mobileColumns).toBe(1)
  await expectNoHorizontalOverflow(page)
  await page.screenshot({ path: "../output/playwright/public-pond-mutations-mobile.png", fullPage: true })
})

test("pond residents reuse the standard item rename flow", async ({ page }, testInfo) => {
  await page.setViewportSize({ width: 1280, height: 900 })
  let renameRequest: unknown
  await mockFishing(page, false, undefined, [{
    instanceId: 700,
    templateItemId: "harmonic_embryo",
    quantity: 1,
    enhanceLevel: 0,
    durability: 0,
    maxDurability: 0,
    baseSnapshot: {},
    affixes: [],
    customData: {},
    state: "inventory"
  }], undefined, undefined, [], {
    onPondRenameRequest: (body) => { renameRequest = body }
  })
  await page.goto("/game/pond")
  await dismissBuildNotice(page)

  const fishCard = page.locator(".pond-fish-list > article").first()
  await expect(fishCard).toContainText("小金")
  await fishCard.getByRole("button", { name: "改名", exact: true }).click()
  const renameDialog = page.getByRole("dialog", { name: "改名" })
  await expect(renameDialog.getByText("改名消耗", { exact: true })).toBeVisible()
  await renameDialog.locator("input").first().fill("金鳞")
  await page.waitForTimeout(300)
  await renameDialog.screenshot({ path: testInfo.outputPath("pond-fish-rename.png") })
  await page.setViewportSize({ width: 390, height: 844 })
  const mobileDialogBox = await renameDialog.locator(".el-dialog").boundingBox()
  expect(mobileDialogBox?.width || 0).toBeLessThanOrEqual(374)
  await expectNoHorizontalOverflow(page)
  await renameDialog.screenshot({ path: testInfo.outputPath("pond-fish-rename-mobile.png") })
  await renameDialog.getByRole("button", { name: "确认", exact: true }).click()

  await expect.poll(() => renameRequest).toEqual({ instanceId: 600, name: "金鳞", labelColor: "gold" })
  await expect(fishCard.getByRole("heading", { name: "金鳞", exact: true })).toBeVisible()
  await expectNoHorizontalOverflow(page)
})

test("manual pond feeding uses the existing feed action and gathers hungry fish", async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 900 })
  let feedRequest: unknown
  await mockFishing(page, false, undefined, [], undefined, undefined, [], {
    onPondFeedRequest: (body) => { feedRequest = body }
  })
  await page.goto("/game/pond")
  await dismissBuildNotice(page)

  await page.getByRole("button", { name: "互动", exact: true }).click()
  await page.getByRole("menu", { name: "鱼池互动" }).getByRole("menuitem", { name: "撒饲料", exact: true }).click()
  const feedDialog = page.getByRole("dialog", { name: "撒饲料" })
  await expect(feedDialog.getByText("青蔬鱼粮").first()).toBeVisible()
  await expect(feedDialog.getByText("撒下份数", { exact: true })).toBeVisible()
  await feedDialog.getByRole("button", { name: "选好，去撒料", exact: true }).click()
  await expect(feedDialog).toBeHidden()

  const scatterSurface = page.getByRole("button", { name: /在这里撒下 1 份青蔬鱼粮/ })
  await expect(scatterSurface).toBeVisible()
  await expect(scatterSurface).toBeFocused()
  await scatterSurface.click({ position: { x: 520, y: 185 } })

  await expect.poll(() => feedRequest).toEqual({ itemId: "pond_feed_herbal", quantity: 1 })
  await expect(page.getByText("饲料已撒进鱼池", { exact: true })).toBeVisible()
  await expect(page.locator(".pond-feed-burst")).toBeVisible()
  await expect(page.locator(".pond-fish-sprite.is-heading-to-feed")).toHaveCount(4)
  await expect(page.locator(".pond-feed-list")).toContainText("料仓中 1 份")
  const feedingFish = page.locator(".pond-fish-sprite:not(.is-dead)").first()
  await feedingFish.hover({ force: true })
  await expect(page.locator(".pond-water__tooltip")).toBeVisible()
  await feedingFish.click({ force: true })
  await expect(page.locator(".pond-water__tracking")).toBeVisible()
  await page.getByRole("button", { name: "退出跟踪" }).click()
  await page.mouse.move(0, 0)
  await expect.poll(() => page.locator(".pond-fish-sprite.is-nibbling").count(), { timeout: 5_000 }).toBeGreaterThan(0)
  await page.locator(".pond-water").screenshot({ path: "../output/playwright/pond-manual-feeding-desktop.png" })
  await expectNoHorizontalOverflow(page)
})

test("pond feeder management preserves fractions and supports pause withdrawal and discard", async ({ page }, testInfo) => {
  await mockFishing(page, false, undefined, [], undefined, undefined, [], { pondDslFeeds: true })
  const overview = pondOverview(true)
  const feed = Object.assign(overview.feeds[0]!, { storedNutrition: 25, storedPortions: 2.5, paused: false, role: 'staple' })
  overview.feedTotalNutrition = 25
  const operations: string[] = []
  const respond = (route: import('@playwright/test').Route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ ok: true, data: overview }) })
  await page.route('**/api/fishing/pond', respond)
  await page.route('**/api/fishing/pond/feed/manage', async route => {
    const body = route.request().postDataJSON()
    operations.push(body.operation)
    expect(body.itemId).toBe(feed.itemId)
    if (body.operation === 'pause' || body.operation === 'resume') feed.paused = body.operation === 'pause'
    if (body.operation === 'withdraw') { expect(body.quantity).toBe(2); feed.storedPortions -= body.quantity }
    if (body.operation === 'discard_remainder') feed.storedPortions = 0
    feed.storedNutrition = feed.storedPortions * feed.nutrition
    overview.feedTotalNutrition = feed.storedNutrition
    return respond(route)
  })
  await page.setViewportSize({ width: 390, height: 844 })
  await page.goto('/game/pond')
  await dismissBuildNotice(page)
  const row = page.locator('.pond-feed-list > article').filter({ hasText: '青蔬鱼粮' })
  await expect(row).toContainText('料仓中 2.5 份')
  await row.getByRole('button', { name: '暂停使用', exact: true }).click()
  await expect(row).toContainText('已暂停')
  await row.getByRole('button', { name: '恢复使用', exact: true }).click()
  await row.getByRole('button', { name: '取出鱼粮', exact: true }).click()
  const prompt = page.getByRole('dialog', { name: '青蔬鱼粮' })
  await prompt.getByRole('button', { name: '取出鱼粮', exact: true }).click()
  await expect(row).toContainText('料仓中 0.5 份')
  await expect(prompt).not.toBeVisible()
  await page.mouse.move(380, 0)
  await expect(row.getByRole('button', { name: '取出鱼粮', exact: true })).toBeDisabled()
  await row.scrollIntoViewIfNeeded()
  await expectNoHorizontalOverflow(page)
  await row.screenshot({ path: testInfo.outputPath('pond-feed-management-mobile.png') })
  await row.getByRole('button', { name: '丢弃余料', exact: true }).click()
  const discard = page.getByRole('dialog', { name: '丢弃余料' })
  await discard.getByRole('button', { name: '丢弃余料', exact: true }).click()
  await expect(row).toContainText('料仓中 0 份')
  expect(operations).toEqual(['pause', 'resume', 'withdraw', 'discard_remainder'])
})

test("targeted longevity feed protects only the chosen fish and stays out of shared stock", async ({ page }, testInfo) => {
  await mockFishing(page, false, undefined, [], undefined, undefined, [], { pondDslFeeds: true })
  const overview = pondOverview(true)
  const fish = Object.assign(overview.fish[0]!, { careRevision: 0, naturalProtectionHours: 0 })
  const feed = { itemId: 'pond_feed_sacred_longevity', feedClass: 'arcane', nutrition: 0, delivery: 'targeted', role: 'functional', rulesetVersion: 'pond-feed-v2', inventoryCount: 2, storedNutrition: 0, conditions: [], effects: [
    { kind: 'stat_delta', stat: 'longevityDays', mode: 'flat', valuePerPortion: 1095 },
    { kind: 'stat_delta', stat: 'naturalProtectionDays', mode: 'flat', valuePerPortion: 1095 },
  ] }
  overview.feeds.push(feed)
  const respond = (route: import('@playwright/test').Route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ ok: true, data: overview }) })
  await page.route('**/api/fishing/pond', respond)
  let requests = 0
  await page.route(`**/api/fishing/pond/fish/${fish.instanceId}/feed`, async route => {
    expect(route.request().postDataJSON()).toEqual({ itemId: feed.itemId, expectedRevision: requests })
    requests++; fish.careRevision++; fish.naturalProtectionHours += 72; feed.inventoryCount--
    return respond(route)
  })
  await page.setViewportSize({ width: 390, height: 844 })
  await page.goto('/game/pond'); await dismissBuildNotice(page)
  await page.getByRole('button', { name: '定向投喂', exact: true }).first().click()
  const dialog = page.getByRole('dialog', { name: '定向投喂', exact: true })
  await expect(dialog).toContainText('圣晶续生粮')
  await expect(dialog).toContainText('自然死亡保护 +约 3 个现实日')
  await expect(dialog).not.toContainText('每鱼每小时')
  await expectNoHorizontalOverflow(page)
  await dialog.screenshot({ path: testInfo.outputPath('pond-longevity-mobile.png'), animations: 'disabled' })
  await page.setViewportSize({ width: 1280, height: 900 })
  await expectNoHorizontalOverflow(page)
  await dialog.screenshot({ path: testInfo.outputPath('pond-longevity-desktop.png'), animations: 'disabled' })
  await page.setViewportSize({ width: 390, height: 844 })
  await dialog.getByRole('button', { name: '消耗 1 份投喂', exact: true }).click()
  await expect(dialog).not.toBeVisible()
  await expect(page.getByText('护生剩余 约 3 个现实日', { exact: true })).toHaveCount(1)
  await page.getByRole('button', { name: '定向投喂', exact: true }).first().click()
  await dialog.getByRole('button', { name: '消耗 1 份投喂', exact: true }).click()
  await expect(page.getByText('护生剩余 约 6 个现实日', { exact: true })).toHaveCount(1)
  expect(requests).toBe(2)
  await page.getByRole('button', { name: '加入料仓', exact: true }).click()
  await expect(page.getByRole('dialog', { name: '加入料仓', exact: true })).not.toContainText('圣晶续生粮')
})

test("pond feed picker explains DSL targeting, growth, and health tradeoffs", async ({ page }, testInfo) => {
  await page.setViewportSize({ width: 1280, height: 900 })
  await mockFishing(page, false, undefined, [], undefined, undefined, [], { pondDslFeeds: true })
  await page.goto("/game/pond")
  await dismissBuildNotice(page)

  await page.getByRole("button", { name: "加入料仓", exact: true }).click()
  const feedDialog = page.getByRole("dialog", { name: "加入料仓" })
  const picker = feedDialog.locator(".pond-feed-picker")
  const selective = picker.getByRole("button").filter({ hasText: "净叶专食粮" })
  const lengthFeed = picker.getByRole("button").filter({ hasText: "韧膜伸体粮" })
  const weightFeed = picker.getByRole("button").filter({ hasText: "沉芯丰体粮" })
  const healingFeed = picker.getByRole("button").filter({ hasText: "星露护鳞粮" })
  const riskyFeed = picker.getByRole("button").filter({ hasText: "蜜爆速饱粮" })
  const forcedFeed = picker.getByRole("button").filter({ hasText: "烈性催育粮" })

  await expect(selective).toContainText("仅草食鱼取食")
  await expect(selective).toContainText("可取食鱼 4 / 5")
  await expect(lengthFeed).toContainText("长度目标 +2.5%")
  await expect(weightFeed).toContainText("体重目标 +4%")
  await expect(healingFeed).toContainText("健康 < 98%")
  await expect(healingFeed).toContainText("恢复健康")
  await expect(riskyFeed).toContainText("饱食度 < 80")
  await expect(riskyFeed).toContainText("健康 >= 10%")
  await expect(riskyFeed).toContainText("额外恢复饱食")
  await expect(riskyFeed).toContainText("消耗健康")
  await expect(forcedFeed).toContainText("长度目标 +4%")
  await expect(forcedFeed).toContainText("体重目标 +7%")
  await expect(forcedFeed).toContainText("健康 >= 35%")
  await expect(forcedFeed).toContainText("消耗健康")
  await expect(picker.locator(".item-card")).toHaveCount(7)
  for (const iconID of [
    "pond_feed_selective_herbal",
    "pond_feed_length_growth",
    "pond_feed_weight_growth",
    "pond_feed_health_recovery",
    "pond_feed_satiety_risk",
    "pond_feed_forced_growth"
  ]) {
    const iconResponse = await page.request.get(`/assets/icons/item/${iconID}.png`)
    expect(iconResponse.ok()).toBe(true)
    expect(iconResponse.headers()["content-type"]).toContain("image/png")
  }
  await page.waitForTimeout(350)
  await feedDialog.screenshot({ path: testInfo.outputPath("pond-feed-dsl-picker-desktop.png") })

  await forcedFeed.click()
  await feedDialog.getByRole("button", { name: "加入料仓", exact: true }).click()
  const warning = page.getByRole("dialog", { name: "这种饲料会损耗健康" })
  await expect(warning).toContainText("烈性催育粮被取食后可能降低鱼的健康")
  await warning.getByRole("button", { name: "取消", exact: true }).click()
  await expect(warning).toBeHidden()

  await feedDialog.getByRole("button", { name: "取消", exact: true }).click()
  await expect(feedDialog).toBeHidden()
  await page.setViewportSize({ width: 390, height: 844 })
  await page.locator("#app").getByRole("button", { name: "加入料仓", exact: true }).click()
  await expect(feedDialog).toBeVisible()
  await expectNoHorizontalOverflow(page)
  await page.waitForTimeout(350)
  await page.screenshot({ path: testInfo.outputPath("pond-feed-dsl-picker-mobile.png") })
})

test("pond stone interactions stay local and make nearby fish react", async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 900 })
  let feedRequest: unknown
  await mockFishing(page, false, undefined, [], undefined, undefined, [], {
    onPondFeedRequest: (body) => { feedRequest = body }
  })
  await page.goto("/game/pond")
  await dismissBuildNotice(page)

  const interactionAction = page.getByRole("button", { name: "互动", exact: true })
  await interactionAction.click()
  const interactionMenu = page.getByRole("menu", { name: "鱼池互动" })
  await expect(interactionMenu.getByRole("menuitem")).toHaveCount(3)
  await page.waitForTimeout(250)
  await page.locator(".pond-water").screenshot({ path: "../output/playwright/pond-interaction-menu-desktop.png" })
  const lightMenuColors = await interactionMenu.evaluate((element) => {
    const style = getComputedStyle(element)
    return { background: style.backgroundColor, color: style.color, border: style.borderColor }
  })
  await page.locator("html").evaluate((element) => element.classList.add("dark"))
  await expect.poll(() => interactionMenu.evaluate((element) => getComputedStyle(element).backgroundColor)).not.toBe(lightMenuColors.background)
  await page.locator(".pond-water").screenshot({ path: "../output/playwright/pond-interaction-menu-dark.png" })
  const darkMenuColors = await interactionMenu.evaluate((element) => {
    const style = getComputedStyle(element)
    return { background: style.backgroundColor, color: style.color, border: style.borderColor }
  })
  expect(darkMenuColors.color).not.toBe(lightMenuColors.color)
  expect(darkMenuColors.border).not.toBe(lightMenuColors.border)
  await page.locator("html").evaluate((element) => element.classList.remove("dark"))
  await expect.poll(() => interactionMenu.evaluate((element) => getComputedStyle(element).backgroundColor)).toBe(lightMenuColors.background)
  await interactionMenu.getByRole("menuitem", { name: "扔石头", exact: true }).click()

  const throwSurface = page.getByRole("button", { name: "在这里扔下石头" })
  const pondBox = await page.locator(".pond-water").boundingBox()
  const targetFishBox = await page.locator(".pond-fish-sprite:not(.is-dead)").first().boundingBox()
  expect(pondBox).not.toBeNull()
  expect(targetFishBox).not.toBeNull()
  await throwSurface.click({
    position: {
      x: targetFishBox!.x + targetFishBox!.width / 2 - pondBox!.x,
      y: targetFishBox!.y + targetFishBox!.height / 2 - pondBox!.y
    }
  })
  await expect(page.locator(".pond-stone-impact.is-throw")).toBeVisible()
  await expect.poll(() => page.locator(".pond-fish-sprite.is-startled").count()).toBeGreaterThan(0)
  await page.locator(".pond-water").screenshot({ path: "../output/playwright/pond-interaction-throw.png" })
  await expect(interactionAction).toBeEnabled({ timeout: 3_000 })

  await interactionAction.click()
  await page.getByRole("menu", { name: "鱼池互动" }).getByRole("menuitem", { name: "打水漂", exact: true }).click()
  const skipSurface = page.getByRole("button", { name: "在水面拖动，决定打水漂的方向和力道" })
  const skipBox = await skipSurface.boundingBox()
  const nearbyFishBox = await page.locator(".pond-fish-sprite:not(.is-dead)").first().boundingBox()
  expect(skipBox).not.toBeNull()
  expect(nearbyFishBox).not.toBeNull()
  const centerX = nearbyFishBox!.x + nearbyFishBox!.width / 2
  const centerY = nearbyFishBox!.y + nearbyFishBox!.height / 2
  const startX = Math.max(skipBox!.x + 36, centerX - 150)
  const endX = Math.min(skipBox!.x + skipBox!.width - 36, centerX + 190)
  await page.mouse.move(startX, centerY + 36)
  await page.mouse.down()
  await page.mouse.move(endX, centerY - 28, { steps: 8 })
  await page.mouse.up()

  await expect.poll(() => page.locator(".pond-stone-impact.is-skip").count(), { timeout: 2_500 }).toBeGreaterThanOrEqual(3)
  await expect.poll(() => page.locator(".pond-fish-sprite.is-startled").count()).toBeGreaterThan(0)
  expect(feedRequest).toBeUndefined()
  await page.locator(".pond-water").screenshot({ path: "../output/playwright/pond-interaction-skip.png" })
  await expectNoHorizontalOverflow(page)
})

test("mobile pond uses searchable fish selectors for stocking and pairing", async ({ page }, testInfo) => {
  await page.setViewportSize({ width: 390, height: 844 })
  await mockFishing(page)
  await page.goto("/game/pond")
  await dismissBuildNotice(page)
  await expect(page.locator(".pond-water canvas")).toBeVisible()
  const touchFish = page.getByRole("button", { name: "跟踪小金" })
  const touchFishBox = await touchFish.boundingBox()
  expect(touchFishBox?.width || 0).toBeGreaterThanOrEqual(46)
  expect(touchFishBox?.height || 0).toBeGreaterThanOrEqual(46)
  await touchFish.click({ force: true })
  await expect(page.locator(".pond-water__tracking")).toContainText("小金")
  await page.getByRole("button", { name: "退出跟踪" }).click()

  await page.getByRole("button", { name: "放入鱼只" }).click()
  const stockDialog = page.getByRole("dialog", { name: "放入鱼只" })
  await expect(stockDialog.getByPlaceholder("搜索鱼名或鱼种")).toBeVisible()
  await expect(stockDialog.getByRole("combobox", { name: "稀有度", exact: true })).toBeVisible()
  await stockDialog.locator(".fish-selector-card").first().click()
  await expect(stockDialog.getByText("可能成为猎物", { exact: true })).toBeVisible()
  await expect(stockDialog.getByRole("button", { name: "放入选中鱼只" })).toBeEnabled()
  await stockDialog.screenshot({ path: testInfo.outputPath("pond-stock-selector-mobile.png") })
  await page.keyboard.press("Escape")

  await page.getByRole("button", { name: "加入料仓" }).click()
  const feedDialog = page.getByRole("dialog", { name: "加入料仓" })
  await expect(feedDialog.getByText("青蔬鱼粮").first()).toBeVisible()
  await expect(feedDialog.getByText("放入份数")).toBeVisible()
  await page.keyboard.press("Escape")

  const interactionAction = page.getByRole("button", { name: "互动", exact: true })
  const interactionActionBox = await interactionAction.boundingBox()
  expect(interactionActionBox?.height || 0).toBeGreaterThanOrEqual(42)
  await interactionAction.click()
  await page.waitForTimeout(250)
  await page.locator(".pond-water").screenshot({ path: "../output/playwright/pond-interaction-menu-mobile.png" })
  await page.getByRole("menu", { name: "鱼池互动" }).getByRole("menuitem", { name: "撒饲料", exact: true }).click()
  const scatterDialog = page.getByRole("dialog", { name: "撒饲料" })
  await scatterDialog.getByRole("button", { name: "选好，去撒料", exact: true }).click()
  await expect(scatterDialog).toBeHidden()
  await page.getByRole("button", { name: /在这里撒下 1 份青蔬鱼粮/ }).click({ position: { x: 190, y: 150 } })
  await expect(page.locator(".pond-feed-burst")).toBeVisible()
  await expect.poll(() => page.locator(".pond-fish-sprite.is-nibbling").count(), { timeout: 5_000 }).toBeGreaterThan(0)
  await page.locator(".pond-water").screenshot({ path: "../output/playwright/pond-manual-feeding-mobile.png" })

  await page.getByRole("button", { name: "指定配对" }).click()
  const pairDialog = page.getByRole("dialog", { name: "指定配对" })
  await pairDialog.getByRole("button", { name: /^选择雌鱼/ }).click()
  const femaleSelector = page.getByRole("dialog", { name: "选择雌鱼" })
  await femaleSelector.locator(".fish-selector-card").first().click()
  await femaleSelector.getByRole("button", { name: "选择这尾鱼" }).click()
  await pairDialog.getByRole("button", { name: /^选择雄鱼/ }).click()
  const maleSelector = page.getByRole("dialog", { name: "选择雄鱼" })
  await maleSelector.locator(".fish-selector-card").first().click()
  await maleSelector.getByRole("button", { name: "选择这尾鱼" }).click()
  await expect(maleSelector).toBeHidden()
  await expect(pairDialog.getByRole("button", { name: "确认配对" })).toBeEnabled()
  await pairDialog.screenshot({ path: testInfo.outputPath("pond-pair-selector-mobile.png") })
  await expectNoHorizontalOverflow(page)
})

test("fish journal explains pond diet and predator behavior", async ({ page }, testInfo) => {
  await page.setViewportSize({ width: 1280, height: 900 })
  await mockFishing(page)
  await page.goto("/game/fishing")
  await dismissBuildNotice(page)
  await page.getByRole("tab", { name: "渔获" }).click()
  await page.locator(".fishing-subnav").getByText("图鉴", { exact: true }).click()

  const predatorEntry = page.locator(".fishing-journal-grid article").filter({ hasText: "白斑狗鱼" })
  await expect(predatorEntry.getByText("偏好高蛋白", { exact: true })).toBeVisible()
  await expect(predatorEntry).toContainText("猎物不超过自身 48%")
  await expect(predatorEntry).toContainText("每日最高 75%")
  await expect(page.locator(".fishing-journal-grid article").filter({ hasText: "鲫鱼" }).getByText("偏好草食", { exact: true })).toBeVisible()
  await expectNoHorizontalOverflow(page)
  await page.locator(".fishing-journal-grid").screenshot({ path: testInfo.outputPath("fish-journal-ecology.png") })
})

test("ecology dialog presents the selected tide layout across desktop and compact widths", async ({ page }) => {
  await page.setViewportSize({ width: 1440, height: 900 })
  await mockFishing(page)
  await page.goto("/game/fishing")
  await dismissBuildNotice(page)

  await page.getByRole("button", { name: "鱼情依据" }).click()
  const ecologyDialog = page.getByRole("dialog", { name: "鱼情依据" })
  const ecologyPanel = page.locator(".fishing-ecology-dialog")
  const tideScroll = ecologyDialog.locator(".fishing-tide-scroll")
  await expect(ecologyDialog).toBeVisible()
  await expect(ecologyDialog.locator(".fishing-ecology-conditions > div")).toHaveCount(4)
  await expect(ecologyDialog.locator(".fishing-tide-chart > li")).toHaveCount(6)
  await expect(ecologyDialog.locator(".fishing-strange-list > article")).toHaveCount(2)
  await expect.poll(() => ecologyPanel.evaluate((element) => getComputedStyle(element).getPropertyValue("--ecology-accent").trim())).toBe("#17747b")
  await expect.poll(() => tideScroll.evaluate((element) => element.scrollWidth <= element.clientWidth + 1)).toBe(true)
  await page.waitForTimeout(500)

  const ecologyPanelBox = await ecologyPanel.boundingBox()
  const desktopViewport = page.viewportSize()
  expect(ecologyPanelBox).not.toBeNull()
  expect(desktopViewport).not.toBeNull()
  expect(ecologyPanelBox!.width).toBeGreaterThanOrEqual(1038)
  expect(Math.abs(ecologyPanelBox!.y - (desktopViewport!.height - ecologyPanelBox!.y - ecologyPanelBox!.height))).toBeLessThanOrEqual(2)
  await page.screenshot({ path: "../output/playwright/fishing-desktop-ecology.png", fullPage: true })

  await page.setViewportSize({ width: 768, height: 900 })
  await expect.poll(() => tideScroll.evaluate((element) => element.scrollWidth > element.clientWidth)).toBe(true)
  await tideScroll.focus()
  await expect(tideScroll).toBeFocused()
  await page.keyboard.press("ArrowRight")
  await expect.poll(() => tideScroll.evaluate((element) => element.scrollLeft)).toBeGreaterThan(0)

  await page.setViewportSize({ width: 840, height: 900 })
  await expect.poll(() => tideScroll.evaluate((element) => element.scrollWidth > element.clientWidth)).toBe(true)
  await page.setViewportSize({ width: 880, height: 900 })
  await expect.poll(() => tideScroll.evaluate((element) => element.scrollWidth <= element.clientWidth + 1)).toBe(true)
})

test("selected angler drives water access and the trip planner", async ({ page }) => {
  await page.setViewportSize({ width: 1440, height: 900 })
  await mockFishing(page, false, undefined, [], undefined, undefined, [], {
    fishingAnglers: [
      { actorType: "player", actorId: 0, status: "idle", fishingLevel: 1 },
      { actorType: "kitty", actorId: 77, name: "小满", status: "idle", fishingLevel: 35 }
    ]
  })
  await page.goto("/game/fishing")
  await dismissBuildNotice(page)

  const anglerControl = page.locator(".fishing-angler-control")
  const waterButtons = page.locator(".fishing-water-switch button")
  await expect(anglerControl).toHaveAccessibleName(/当前钓手钓鱼测试员 - 我，钓鱼等级1/)
  await expect(waterButtons.nth(1)).toBeDisabled()
  await expect(waterButtons.nth(2)).toBeDisabled()
  await expect(waterButtons.nth(1)).toContainText("钓鱼测试员 - 我 · 钓鱼 1 / 15 级")

  await anglerControl.click()
  await expect(page.getByRole("menuitem").filter({ hasText: "钓鱼测试员 - 我" })).toBeVisible()
  await page.screenshot({ path: "../output/playwright/fishing-angler-player-name.png", fullPage: true })
  await page.getByRole("menuitem").filter({ hasText: "小满" }).click()
  await expect(anglerControl).toHaveAccessibleName(/当前钓手小满，钓鱼等级35/)
  await expect(waterButtons.nth(1)).toBeEnabled()
  await expect(waterButtons.nth(2)).toBeEnabled()

  await waterButtons.nth(2).click()
  await page.getByRole("button", { name: /准备出发/ }).click()
  await page.getByRole("button", { name: /下一步/ }).click()
  await expect(page.getByRole("heading", { name: "小满 · 带哪套钓组？" })).toBeVisible()
  await expect(page.locator(".fishing-angler-list")).toHaveCount(0)
  await expect(page.getByText("这次由谁垂钓？")).toHaveCount(0)
  await expectNoHorizontalOverflow(page)
  await page.screenshot({ path: "../output/playwright/fishing-angler-water-access.png", fullPage: true })

  await page.locator(".el-dialog__headerbtn:visible").click()
  await expect(page.locator(".fishing-trip-planner")).toBeHidden()

  await anglerControl.click()
  await page.getByRole("menuitem").filter({ hasText: "钓鱼测试员 - 我" }).click()
  await expect(waterButtons.nth(0)).toHaveClass(/is-selected/)
  await expect(waterButtons.nth(1)).toBeDisabled()
  await expect(waterButtons.nth(2)).toBeDisabled()
  await expect(page.getByText("已切换到钓鱼测试员 - 我可以前往的晴湾浅滩。")).toBeVisible()
  await expect(page.getByText("已切换到钓鱼测试员 - 我可以前往的晴湾浅滩。")).toBeHidden({ timeout: 5000 })
  await page.setViewportSize({ width: 390, height: 844 })
  await expect(anglerControl).toHaveAccessibleName(/当前钓手钓鱼测试员 - 我，钓鱼等级1/)
  await expectNoHorizontalOverflow(page)
  await dismissBuildNotice(page)
  await page.screenshot({ path: "../output/playwright/fishing-angler-player-name-mobile.png", fullPage: true })

  await anglerControl.click()
  await page.getByRole("menuitem").filter({ hasText: "小满" }).click()
  await waterButtons.nth(2).click()
  await expect(waterButtons.nth(2)).toHaveClass(/is-selected/)
  await expect(anglerControl).toHaveAccessibleName(/当前钓手小满，钓鱼等级35/)
  await expect(waterButtons.nth(1)).toContainText("柳溪河谷")
  await expect(waterButtons.nth(2)).toContainText("霜镜深湖")
  await expectNoHorizontalOverflow(page)
  await page.screenshot({ path: "../output/playwright/fishing-angler-water-access-mobile.png", fullPage: true })
})

test("an existing kitty fishing queue remains bound to its angler", async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 800 })
  await mockFishing(page, true, undefined, [], undefined, undefined, [], {
    fishingAnglers: [
      { actorType: "player", actorId: 0, status: "idle", fishingLevel: 1 },
      { actorType: "kitty", actorId: 77, name: "小满", status: "in_life_queue", fishingLevel: 35 }
    ],
    liveActor: { actorType: "kitty", actorId: 77 }
  })
  await page.goto("/game/fishing")
  await dismissBuildNotice(page)

  const anglerControl = page.locator(".fishing-angler-control")
  await expect(anglerControl).toBeDisabled()
  await expect(anglerControl).toHaveAccessibleName(/当前钓手小满，钓鱼等级35/)
  await expect(page.locator(".fishing-live-panel > header")).toContainText("小满")
  await expect(page.locator(".fishing-live-water")).toBeVisible()
})

test("desktop water selection stays simple and reaches departure in three steps", async ({ page }) => {
  await page.setViewportSize({ width: 1440, height: 900 })
  let submittedTrip: any
  await mockFishing(page, false, (body) => { submittedTrip = body }, [braidedMainLineInventory])
  await page.goto("/game/fishing")
  await dismissBuildNotice(page)
  await expect(page.locator(".game-page-header__title")).toHaveText("钓鱼")
  await expect(page.locator(".fishing-tabs .el-tabs__item")).toHaveCount(5)
  await expect(page.locator(".fishing-water-switch img")).toHaveCount(0)
  await expect(page.getByText("鱼口正旺").first()).toBeVisible()
  await expect(page.getByText("12 人正在垂钓").first()).toBeVisible()
  const selectedWaterCard = page.locator(".fishing-water-switch button.is-selected")
  const selectedSpotCard = page.locator(".fishing-spot-list > button.is-selected")
  const lightSelectionBackgrounds = await Promise.all([
    selectedWaterCard.evaluate((element) => getComputedStyle(element).backgroundColor),
    selectedSpotCard.evaluate((element) => getComputedStyle(element).backgroundColor)
  ])
  await page.evaluate(() => document.documentElement.classList.add("dark"))
  const darkSelectionBackgrounds = await Promise.all([
    selectedWaterCard.evaluate((element) => getComputedStyle(element).backgroundColor),
    selectedSpotCard.evaluate((element) => getComputedStyle(element).backgroundColor)
  ])
  expect(darkSelectionBackgrounds[0]).not.toBe(lightSelectionBackgrounds[0])
  expect(darkSelectionBackgrounds[1]).not.toBe(lightSelectionBackgrounds[1])
  await page.evaluate(() => document.documentElement.classList.remove("dark"))
  const spotSpecies = page.locator(".fishing-spot-species")
  await expect(spotSpecies.getByRole("heading", { name: "钓点鱼群" })).toBeVisible()
  await expect(spotSpecies.locator(".fishing-spot-species__list > div")).toHaveCount(2)
  await expect(spotSpecies.locator(".fishing-spot-species__list > div").filter({ hasText: "鲫鱼" })).toContainText("基础 75%")
  await expect(spotSpecies.locator(".fishing-spot-species__list > div").filter({ hasText: "莲纹锦鲤" })).toContainText("基础 25%")
  await page.screenshot({ path: "../output/playwright/fishing-desktop-spot-fish.png", fullPage: true })
  await expect(page.getByText("为不同水域准备")).toHaveCount(0)
  await expect(page.getByText("每一尾都有来处")).toHaveCount(0)
  const desktopMenuItems = await page.locator(".game-shell-view__sidebar:not(.game-shell-mobile-icon-rail) .el-menu-item").allTextContents()
  const alchemyMenuIndex = desktopMenuItems.findIndex((label) => label.trim() === "炼金")
  const fishingMenuIndex = desktopMenuItems.findIndex((label) => label.trim() === "钓鱼")
  const pondMenuIndex = desktopMenuItems.findIndex((label) => label.trim() === "池塘")
  const collectingMenuIndex = desktopMenuItems.findIndex((label) => label.trim() === "采集")
  expect(alchemyMenuIndex).toBeGreaterThanOrEqual(0)
  expect(fishingMenuIndex).toBe(alchemyMenuIndex + 1)
  expect(pondMenuIndex).toBe(fishingMenuIndex + 1)
  expect(collectingMenuIndex).toBe(pondMenuIndex + 1)
  await expectNoHorizontalOverflow(page)

  await page.getByRole("button", { name: "鱼情依据" }).click()
  const ecologyDialog = page.getByRole("dialog", { name: "鱼情依据" })
  await expect(ecologyDialog).toBeVisible()
  await expect(ecologyDialog.getByText("接下来一小时")).toBeVisible()
  await expect(ecologyDialog.getByText("奇鱼窗口")).toBeVisible()
  await expect(ecologyDialog.locator(".fishing-ecology-conditions > div")).toHaveCount(4)
  await expect(ecologyDialog.locator(".fishing-tide-chart > li")).toHaveCount(6)
  await expect(ecologyDialog.locator(".fishing-strange-list > article")).toHaveCount(2)
  await expect.poll(() => page.locator(".fishing-ecology-dialog").evaluate((element) => getComputedStyle(element).getPropertyValue("--ecology-accent").trim())).toBe("#17747b")
  await expect.poll(() => ecologyDialog.locator(".fishing-tide-scroll").evaluate((element) => element.scrollWidth <= element.clientWidth + 1)).toBe(true)
  await page.waitForTimeout(500)
  const ecologyPanelBox = await page.locator(".fishing-ecology-dialog").boundingBox()
  expect(ecologyPanelBox).not.toBeNull()
  expect(ecologyPanelBox!.width).toBeGreaterThanOrEqual(1038)
  expect(Math.abs(ecologyPanelBox!.y - (900 - ecologyPanelBox!.y - ecologyPanelBox!.height))).toBeLessThanOrEqual(2)
  await page.screenshot({ path: "../output/playwright/fishing-desktop-ecology.png", fullPage: true })
  await page.keyboard.press("Escape")
  await page.waitForTimeout(350)

  const anglerControl = page.locator(".fishing-angler-control")
  await anglerControl.click()
  await page.getByRole("menuitem").filter({ hasText: "小满" }).click()
  await expect(anglerControl).toHaveAccessibleName(/当前钓手小满，钓鱼等级1/)
  await page.getByRole("button", { name: /准备出发/ }).click()
  await expect(page.getByRole("heading", { name: "今天去哪里？" })).toBeVisible()
  await page.locator(".fishing-planner-step button").filter({ hasText: "睡莲边" }).click()
  await page.getByRole("button", { name: /下一步/ }).click()
  await expect(page.getByRole("heading", { name: "小满 · 带哪套钓组？" })).toBeVisible()
  await expect(page.locator(".fishing-angler-list")).toHaveCount(0)
  await page.getByRole("button", { name: /晴湾轻钓组/ }).click()
  await page.getByRole("button", { name: /下一步/ }).click()
  await expect(page.getByRole("heading", { name: "准备钓多久？" })).toBeVisible()
  await expect(page.getByText("携带耗材")).toBeVisible()
  const supplyRefs = page.locator(".fishing-supply-ref")
  await expect(supplyRefs).toHaveCount(5)
  const mainLineRef = supplyRefs.filter({ hasText: "编织主线" })
  await expect(mainLineRef).toContainText("45")
  await mainLineRef.locator(".el-tag").hover()
  await expect(page.locator(".item-popover-content")).toBeVisible()
  await expect(page.locator(".item-popover-title")).toHaveText("编织主线")
  const tackleTooltip = page.locator(".item-popover-content .fishing-tackle-info")
  await expect(tackleTooltip).toContainText("垂钓性能")
  await expect(tackleTooltip).toContainText("主线")
  await expect(tackleTooltip).toContainText("浮漂钓 / 底钓 / 拟饵钓")
  await expect(tackleTooltip).toContainText(/最大承重\s*9\.5 kg/)
  await expect(tackleTooltip).toContainText(/每次搏鱼基础磨损\s*8%/)
  await tackleTooltip.locator(".fishing-tackle-info__help").hover()
  await expect(page.locator(".el-popper").filter({ hasText: "每个部件显示各自的承重" })).toBeVisible()
  await page.screenshot({ path: "../output/playwright/fishing-desktop-supply-tooltip.png", fullPage: true })
  await page.getByRole("heading", { name: "准备钓多久？" }).hover()
  await expect(page.locator(".item-popover-content")).toBeHidden()
  const advancedSettings = page.locator(".fishing-more-settings")
  await advancedSettings.locator("summary").click()
  await expect(advancedSettings.getByText("目标水层", { exact: true })).toBeVisible()
  await expect(advancedSettings.getByText("作钓节奏", { exact: true })).toBeVisible()
  await expect(advancedSettings.getByText("控鱼策略", { exact: true })).toBeVisible()
  await expect(advancedSettings.getByText("安全负荷", { exact: true })).toHaveCount(0)
  await expect(advancedSettings.getByText("渔获处理", { exact: true })).toBeVisible()
  await advancedSettings.getByText("中层", { exact: true }).click()
  await advancedSettings.getByText("积极探钓", { exact: true }).click()
  await advancedSettings.getByText("渔获处理", { exact: true }).scrollIntoViewIfNeeded()
  await expect(page.getByRole("button", { name: /开始钓鱼/ })).toBeVisible()
  await expectNoHorizontalOverflow(page)
  await page.screenshot({ path: "../output/playwright/fishing-desktop-planner-advanced.png", fullPage: true })
  await page.getByRole("button", { name: /开始钓鱼/ }).click()
  await expect.poll(() => submittedTrip).toBeTruthy()
  expect(submittedTrip.steps).toHaveLength(1)
  expect(submittedTrip).toMatchObject({ actorType: "kitty", actorId: 77 })
  expect(submittedTrip.steps[0]).toMatchObject({
    actionId: "fishing.trip",
    count: 1,
    args: {
      spotId: "lily_edge",
      loadoutId: "loadout-1",
      targetLayer: "midwater",
      pace: "search",
      supplies: { mainLine: 8, leader: 8, hook: 12, floatOrSinker: 4, lure: 0, bait: 64 }
    }
  })
})

test("390px ecology dialog keeps the tide forecast scrollable without page overflow", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 })
  await mockFishing(page)
  await page.goto("/game/fishing")
  await dismissBuildNotice(page)

  await page.getByRole("button", { name: "鱼情依据" }).click()
  const ecologyDialog = page.getByRole("dialog", { name: "鱼情依据" })
  const tideScroll = ecologyDialog.locator(".fishing-tide-scroll")
  await expect(ecologyDialog).toBeVisible()
  await expect(ecologyDialog.locator(".fishing-ecology-conditions > div")).toHaveCount(4)
  await expect(ecologyDialog.locator(".fishing-tide-chart > li")).toHaveCount(6)
  await expect.poll(() => tideScroll.evaluate((element) => element.scrollWidth > element.clientWidth)).toBe(true)
  const dialogBody = page.locator(".fishing-ecology-dialog .el-dialog__body")
  await expect.poll(() => dialogBody.evaluate((element) => element.scrollHeight > element.clientHeight)).toBe(true)
  await expectNoHorizontalOverflow(page)
  await page.waitForTimeout(350)
  await page.screenshot({ path: "../output/playwright/fishing-mobile-ecology.png" })
  await tideScroll.evaluate((element) => element.scrollTo({ left: element.scrollWidth }))
  await expect(ecologyDialog.locator(".fishing-tide-chart > li").last()).toBeInViewport()
  await dialogBody.evaluate((element) => element.scrollTo({ top: element.scrollHeight }))
  await expect(ecologyDialog.locator(".fishing-strange-list > article").last()).toBeInViewport()
  await expect.poll(() => page.evaluate(() => window.scrollY)).toBe(0)
})

test("trip failure names the exact missing fishing supply", async ({ page }) => {
  await page.setViewportSize({ width: 1440, height: 900 })
  await mockFishing(page, false, undefined, [], undefined, {
    code: "WS_FISHING_SUPPLY_INSUFFICIENT",
    instanceState: "reed_float"
  })
  await page.goto("/game/fishing")
  await dismissBuildNotice(page)
  await page.getByRole("button", { name: /准备出发/ }).click()
  await page.locator(".fishing-planner-step button").filter({ hasText: "睡莲边" }).click()
  await page.getByRole("button", { name: /下一步/ }).click()
  await page.locator(".fishing-planner-step button").filter({ hasText: "晴湾轻钓组" }).click()
  await page.getByRole("button", { name: /下一步/ }).click()
  await page.getByRole("button", { name: /开始钓鱼/ }).click()
  await expect(page.locator(".el-message")).toContainText("仓库里的芦苇浮漂不足，请减少携带数量后再出发。")
})

test("player and kitty both see permanent fishing gear and kitty efficiency starts at zero", async ({ page }) => {
  await page.setViewportSize({ width: 1440, height: 900 })
  await mockFishing(page)
  await page.goto("/game/attributes")
  await dismissBuildNotice(page)

  const characterTabs = page.locator('[data-tutorial="character-tabs"] > .el-tabs__header')
  await characterTabs.getByRole("tab", { name: "装备", exact: true }).click()
  const equipmentPanel = page.locator('[data-tutorial="equipment-panel"]')
  await expect(equipmentPanel.locator(".el-tabs__header").getByRole("tab")).toHaveText(["生活", "渔具", "战斗"])
  await equipmentPanel.getByRole("tab", { name: "渔具", exact: true }).click()
  await expect(equipmentPanel.locator(".equipment-slot-label:visible")).toHaveText(["鱼竿", "鱼轮"])
  await expect(equipmentPanel.locator(".game-icon-atlas:visible")).toHaveCount(2)
  await page.screenshot({ path: "../output/playwright/fishing-player-equipment.png", fullPage: true })

  await page.locator(".attributes-desktop-rolebar button").filter({ hasText: "小满" }).click()
  await expect(equipmentPanel.locator(".el-tabs__header").getByRole("tab")).toHaveText(["战斗", "渔具"])
  await equipmentPanel.getByRole("tab", { name: "渔具", exact: true }).click()
  await expect(equipmentPanel.locator(".equipment-slot-label:visible")).toHaveText(["鱼竿", "鱼轮"])
  await expect(equipmentPanel.locator(".game-icon-atlas:visible")).toHaveCount(2)
  await page.screenshot({ path: "../output/playwright/fishing-kitty-equipment.png", fullPage: true })

  await characterTabs.getByRole("tab", { name: "属性", exact: true }).click()
  const actionSection = page.locator(".attribute-derived-section").filter({ hasText: "动作" })
  await expect(actionSection).toBeVisible()
  await expect(actionSection.locator(".attribute-stat-card__name")).toHaveText([
    "烹饪效率",
    "采集效率",
    "采矿效率",
    "畜牧效率",
    "制作效率",
    "缝纫效率",
    "锻造效率"
  ])
  const alchemySection = page.locator(".attribute-derived-section").filter({ hasText: "炼金" })
  await expect(alchemySection.locator(".attribute-stat-card__name")).toHaveText(["炼金效率"])
  const fishingSection = page.locator(".attribute-derived-section").filter({ hasText: "垂钓" })
  await expect(fishingSection).toBeVisible()
  const efficiency = fishingSection.locator(".attribute-stat-card").filter({ hasText: "垂钓效率" })
  await expect(efficiency.locator(".attribute-stat-card__value")).toHaveText("0%")
  await page.screenshot({ path: "../output/playwright/fishing-kitty-attributes.png", fullPage: true })
  await expectNoHorizontalOverflow(page)
})

test("kitty can take the fishing rod and reel currently equipped by the player", async ({ page }) => {
  await page.setViewportSize({ width: 1440, height: 900 })
  const equipRequests: Array<{ instanceId?: number; slot?: string; equipType?: string }> = []
  await mockFishing(page, false, undefined, [], undefined, undefined, [], {
    kittyEquipment: [],
    onKittyEquipRequest: (body) => equipRequests.push(body as { instanceId?: number; slot?: string; equipType?: string })
  })
  await page.goto("/game/attributes")
  await dismissBuildNotice(page)

  const characterTabs = page.locator('[data-tutorial="character-tabs"] > .el-tabs__header')
  await characterTabs.getByRole("tab", { name: "装备", exact: true }).click()
  await page.locator(".attributes-desktop-rolebar button").filter({ hasText: "小满" }).click()
  const equipmentPanel = page.locator('[data-tutorial="equipment-panel"]')
  await equipmentPanel.getByRole("tab", { name: "渔具", exact: true }).click()
  await expect(equipmentPanel.locator(".game-icon-atlas:visible")).toHaveCount(0)

  await equipmentPanel.locator(".equipment-slot-cell:visible").filter({ hasText: "鱼竿" }).click()
  let picker = page.locator(".el-dialog").filter({ hasText: "装备到 鱼竿" })
  await expect(picker.getByText("基础鱼竿", { exact: true })).toBeVisible()
  await picker.locator(".candidate-item").filter({ hasText: "基础鱼竿" }).click()
  await expect.poll(() => equipRequests[0]).toEqual({ instanceId: 101, slot: "fishing_rod", equipType: "work" })
  await expect(equipmentPanel.locator(".equipment-slot-cell:visible").filter({ hasText: "鱼竿" }).locator(".game-icon-atlas")).toHaveCount(1)

  await equipmentPanel.locator(".equipment-slot-cell:visible").filter({ hasText: "鱼轮" }).click()
  picker = page.locator(".el-dialog").filter({ hasText: "装备到 鱼轮" })
  await expect(picker.getByText("基础纺车轮", { exact: true })).toBeVisible()
  await picker.locator(".candidate-item").filter({ hasText: "基础纺车轮" }).click()
  await expect.poll(() => equipRequests[1]).toEqual({ instanceId: 102, slot: "fishing_reel", equipType: "work" })
  await expect(equipmentPanel.locator(".game-icon-atlas:visible")).toHaveCount(2)
  await expectNoHorizontalOverflow(page)
})

const advancedTackleOptions = {
  ownedTackle: [
    ...ownedTackle,
    { itemId: "silverleaf_float_rod", slot: "rod", quantity: 1, instanceId: 201 },
    { itemId: "light_copper_reel", slot: "reel", quantity: 1, instanceId: 202 }
  ],
  loadouts: [
    { ...loadout, loadoutId: "advanced", name: "进阶浮漂钓组", components: { ...loadout.components, rodInstanceId: 201, reelInstanceId: 202 } },
    loadout
  ]
}

for (const width of [1440, 360]) {
  test(`${width}px equipment requirements follow the chosen angler and check both rod and reel`, async ({ page }) => {
    await page.setViewportSize({ width, height: 900 })
    await mockFishing(page, false, undefined, [], undefined, undefined, [], {
      ...advancedTackleOptions,
      playerAttributes: { fishing: { value: 8, unit: "number", category: "work", growthType: "levelable" } }
    })
    await page.goto("/game/fishing")
    await dismissBuildNotice(page)
    const next = page.getByRole("button", { name: /下一步/ })
    const openLoadouts = async () => {
      await page.getByRole("button", { name: /准备出发/ }).click()
      await next.click()
    }
    await openLoadouts()
    await expect(page.locator(".fishing-method-notice")).toContainText("进阶浮漂钓组")
    await expect(next).toBeEnabled()
    await page.locator(".el-dialog__headerbtn:visible").click()
    await page.locator(".fishing-angler-control").click()
    await page.getByRole("menuitem").filter({ hasText: "小满" }).click()
    await openLoadouts()
    const advanced = page.locator(".fishing-planner-step").getByRole("button", { name: /进阶浮漂钓组/ })
    await expect(advanced.locator(".fishing-loadout-method-issue")).toHaveCount(2)
    await expect(advanced).toContainText("需要钓鱼达到 8，小满当前为 1。")
    await expect(page.locator(".fishing-method-notice")).toContainText("小满尚未满足所选钓组的装备要求")
    await expect(next).toBeDisabled()
    await expectNoHorizontalOverflow(page)
    expect(await advanced.evaluate((el) => el.scrollWidth <= el.clientWidth)).toBe(true)
    await page.screenshot({ path: `../output/playwright/fishing-equipment-requirements-${width}.png`, fullPage: true })
    await page.locator(".fishing-planner-step").getByRole("button", { name: /晴湾轻钓组/ }).click()
    await expect(next).toBeEnabled()
    await next.click()
    await expect(page.getByRole("button", { name: /开始钓鱼/ })).toBeEnabled()
  })
}

test("equipment requirements use final attributes instead of fishing map-access level", async ({ page }) => {
  await mockFishing(page, false, undefined, [], undefined, undefined, [], {
    ...advancedTackleOptions,
    playerAttributes: { fishing: { value: 1, unit: "number", category: "work", growthType: "levelable" } },
    kittyAttributes: kittyFishingAttributes.map((row) => row.attrKey === "fishing" ? { ...row, finalValue: 8 } : row)
  })
  await page.goto("/game/fishing")
  await dismissBuildNotice(page)
  await page.getByRole("button", { name: /准备出发/ }).click()
  const next = page.getByRole("button", { name: /下一步/ })
  await next.click()
  await expect(next).toBeDisabled()
  await expect(page.locator(".fishing-planner-step")).toContainText("需要钓鱼达到 8，钓鱼测试员 - 我当前为 1。")
  await page.locator(".el-dialog__headerbtn:visible").click()
  await page.locator(".fishing-angler-control").click()
  await page.getByRole("menuitem").filter({ hasText: "小满" }).click()
  await page.getByRole("button", { name: /准备出发/ }).click()
  await next.click()
  await expect(next).toBeEnabled()
  await expect(page.locator(".fishing-planner-step .fishing-loadout-method-issue")).toHaveCount(0)
})

test("late equipment attribute failures do not leak into a different angler", async ({ page }) => {
  await mockFishing(page, false, undefined, [], undefined, undefined, [], {
    ...advancedTackleOptions,
    playerAttributes: { fishing: { value: 8, unit: "number", category: "work", growthType: "levelable" } }
  })
  let release: () => void = () => undefined
  const pending = new Promise<void>((resolve) => { release = resolve })
  await page.route("**/api/cattery/kitties/77/combat-profile", async (route) => {
    await pending
    await route.fulfill({ status: 503, json: { ok: false } })
  })
  await page.goto("/game/fishing")
  await dismissBuildNotice(page)
  await page.locator(".fishing-angler-control").click()
  await page.getByRole("menuitem").filter({ hasText: "小满" }).click()
  await page.getByRole("button", { name: /准备出发/ }).click()
  const next = page.getByRole("button", { name: /下一步/ })
  await next.click()
  await expect(page.getByText("正在检查小满的装备要求…")).toBeVisible()
  await expect(next).toBeDisabled()
  await page.locator(".el-dialog__headerbtn:visible").click()
  await page.locator(".fishing-angler-control").click()
  await page.getByRole("menuitem").filter({ hasText: "钓鱼测试员 - 我" }).click()
  await page.getByRole("button", { name: /准备出发/ }).click()
  await next.click()
  await expect(next).toBeEnabled()
  const response = page.waitForResponse("**/api/cattery/kitties/77/combat-profile")
  release()
  await response
  await expect(page.locator(".fishing-method-notice")).toContainText("当前选择：进阶浮漂钓组")
  await expect(page.locator(".fishing-planner-step")).not.toContainText("未能读取")
  await expect(next).toBeEnabled()
})

test("equipment attribute loading failure blocks restricted tackle and can be retried", async ({ page }) => {
  await mockFishing(page, false, undefined, [], undefined, undefined, [], advancedTackleOptions)
  let attempts = 0
  await page.route("**/api/cattery/kitties/77/combat-profile", async (route) => {
    attempts++
    if (attempts === 1) return route.fulfill({ status: 503, json: { ok: false } })
    return route.fallback()
  })
  await page.goto("/game/fishing")
  await dismissBuildNotice(page)
  await page.locator(".fishing-angler-control").click()
  await page.getByRole("menuitem").filter({ hasText: "小满" }).click()
  await page.getByRole("button", { name: /准备出发/ }).click()
  const next = page.getByRole("button", { name: /下一步/ })
  await next.click()
  await expect(page.getByText("未能读取小满的属性，暂时无法确认装备要求，请重试。")).toBeVisible()
  await expect(next).toBeDisabled()
  await expect(page.locator(".fishing-planner-step .fishing-loadout-method-issue")).toHaveCount(0)
  await page.locator(".fishing-planner-step").getByRole("button", { name: "刷新", exact: true }).click()
  await expect(page.locator(".fishing-planner-step")).toContainText("需要钓鱼达到 8，小满当前为 1。")
  await expect(page.getByText("未能读取小满的属性，暂时无法确认装备要求，请重试。")).toHaveCount(0)
  expect(attempts).toBe(2)
})

for (const width of [1440, 360]) {
  test(`${width}px trip planner explains incompatible methods before the duration step`, async ({ page }) => {
    await page.setViewportSize({ width, height: 900 })
    let submittedTrips = 0
    await mockFishing(page, false, () => { submittedTrips++ })
    const bottomLoadout = {
      ...loadout, loadoutId: "bottom-loadout", name: "深水底钓组", method: "bottom",
      components: { ...loadout.components, floatOrSinkerItemId: "river_sinker" }
    }
    await page.route("**/api/fishing/loadouts", (route) => route.fulfill({ json: { items: [bottomLoadout, loadout] } }))
    await page.goto("/game/fishing")
    await dismissBuildNotice(page)
    await page.getByRole("button", { name: /准备出发/ }).click()
    const planner = page.locator(".fishing-planner-step")
    const next = page.getByRole("button", { name: /下一步/ })
    const back = page.getByRole("button", { name: /上一步/ })
    await planner.getByRole("button", { name: /睡莲边/ }).click()
    await next.click()
    const notice = planner.getByRole("status")
    await expect(notice).toContainText("睡莲边仅支持：浮漂钓 / 拟饵钓。")
    await expect(notice).toContainText("当前选择：深水底钓组（底钓）。")
    await expect(notice).toContainText("请更换钓组，或返回上一步更换区域")
    await expect(planner.getByRole("button", { name: /深水底钓组/ })).toContainText("此区域不支持该钓法")
    await expect(next).toBeDisabled()
    await expectNoHorizontalOverflow(page)
    expect(await notice.evaluate((el) => el.scrollWidth <= el.clientWidth)).toBe(true)
    await page.screenshot({ path: `../output/playwright/fishing-method-mismatch-${width}.png`, fullPage: true })

    await planner.getByRole("button", { name: /晴湾轻钓组/ }).click()
    await expect(notice).toContainText("当前选择：晴湾轻钓组（浮漂钓）。")
    await expect(notice).not.toContainText("请更换钓组")
    await expect(next).toBeEnabled()
    await next.click()
    await expect(page.getByRole("heading", { name: "准备钓多久？" })).toBeVisible()
    await expect(page.getByRole("button", { name: /开始钓鱼/ })).toBeEnabled()
    await back.click()
    await planner.getByRole("button", { name: /深水底钓组/ }).click()
    await expect(next).toBeDisabled()
    await back.click()
    await planner.getByRole("button", { name: /老柳树下/ }).click()
    await next.click()
    await expect(notice).toContainText("老柳树下仅支持：浮漂钓 / 底钓。")
    await expect(notice).toContainText("当前选择：深水底钓组（底钓）。")
    await expect(notice).not.toContainText("请更换钓组")
    await expect(next).toBeEnabled()
    expect(submittedTrips).toBe(0)
  })
}

test("trip planner offers recovery when no tackle set supports the spot", async ({ page }) => {
  await mockFishing(page)
  await page.goto("/game/fishing")
  await dismissBuildNotice(page)
  await page.locator(".fishing-water-switch button").filter({ hasText: "柳溪河谷" }).click()
  await page.getByRole("button", { name: /准备出发/ }).click()
  await page.locator(".fishing-planner-step").getByRole("button", { name: /深水河弯/ }).click()
  await page.getByRole("button", { name: /下一步/ }).click()
  await expect(page.locator(".fishing-method-notice")).toContainText("深水河弯仅支持：底钓 / 拟饵钓。")
  await expect(page.getByRole("button", { name: /下一步/ })).toBeDisabled()
  const recovery = page.locator(".fishing-loadout-recovery")
  await expect(recovery).toContainText("暂无适用于此区域的钓组。")
  await recovery.getByRole("button").click()
  await expect(page.locator(".fishing-trip-planner")).toBeHidden()
  await expect(page.getByRole("tab", { name: "钓组", exact: true })).toHaveAttribute("aria-selected", "true")
})

test("360px trip planner keeps every step touchable without sideways scrolling", async ({ page }) => {
  await page.setViewportSize({ width: 360, height: 640 })
  await mockFishing(page)
  await page.goto("/game/fishing")
  await dismissBuildNotice(page)
  const departureAction = page.getByRole("button", { name: /准备出发/ })
  await expect(departureAction).toBeInViewport()
  const departureDock = page.locator(".fishing-departure-dock")
  expect(await departureDock.evaluate((element) => getComputedStyle(element.parentElement!).position)).toBe("fixed")
  const naturalAffixBottom = await page.locator(".fishing-departure-affix").evaluate((element) => element.getBoundingClientRect().bottom)
  expect(naturalAffixBottom).toBeGreaterThan(640)
  const departureBox = await departureAction.boundingBox()
  expect(departureBox).not.toBeNull()
  expect(departureBox!.y + departureBox!.height).toBeLessThanOrEqual(632)
  await page.setViewportSize({ width: 360, height: 800 })
  await page.screenshot({ path: "../output/playwright/fishing-mobile-spot-fish.png", fullPage: true })
  await departureAction.click()
  await page.locator(".fishing-planner-step button").filter({ hasText: "睡莲边" }).click()
  await page.getByRole("button", { name: /下一步/ }).click()
  await page.locator(".fishing-planner-step button").filter({ hasText: "晴湾轻钓组" }).click()
  await page.getByRole("button", { name: /下一步/ }).click()
  await expect(page.getByRole("heading", { name: "准备钓多久？" })).toBeVisible()
  await expect(page.getByRole("button", { name: /开始钓鱼/ })).toBeVisible()
  const advancedSettings = page.locator(".fishing-more-settings")
  await advancedSettings.locator("summary").click()
  await expect(advancedSettings.getByText("目标水层", { exact: true })).toBeVisible()
  await expect(advancedSettings.getByText("作钓节奏", { exact: true })).toBeVisible()
  await expect(advancedSettings.getByText("控鱼策略", { exact: true })).toBeVisible()
  await expect(advancedSettings.getByText("安全负荷", { exact: true })).toHaveCount(0)
  await expect(advancedSettings.getByText("渔获处理", { exact: true })).toBeVisible()
  await advancedSettings.getByText("渔获处理", { exact: true }).scrollIntoViewIfNeeded()
  await expectNoHorizontalOverflow(page)
  const dialogBox = await page.locator(".game-dialog-overlay .el-dialog").boundingBox()
  expect(dialogBox).not.toBeNull()
  expect(dialogBox!.x).toBeLessThanOrEqual(1)
  expect(dialogBox!.width).toBeGreaterThanOrEqual(359)
  expect(dialogBox!.height).toBeGreaterThanOrEqual(790)
  const shortButtons = await page.locator(".fishing-trip-planner button:visible, .fishing-dialog-footer button:visible").evaluateAll((buttons) => buttons
    .filter((button) => button.getBoundingClientRect().height < 44)
    .map((button) => ({ label: button.getAttribute("aria-label") || button.textContent?.trim(), height: button.getBoundingClientRect().height })))
  expect(shortButtons).toEqual([])
  await page.screenshot({ path: "../output/playwright/fishing-mobile-planner-advanced.png", fullPage: true })
})

test("live fishing scenery follows authoritative time weather and water", async ({ page }, testInfo) => {
  test.setTimeout(60_000)
  const options: FishingMockOptions = { scene: { gameHour: 12, weather: "sunny", waterId: "sunmere_shallows" } }
  await page.setViewportSize({ width: 1440, height: 980 })
  await mockFishing(page, true, undefined, [], undefined, undefined, [], options)
  await page.goto("/game/fishing")
  await dismissBuildNotice(page)
  const scene = page.locator(".fishing-backdrop")
  const water = page.locator(".fishing-live-water")
  const scenarios = [
    { hour: 12, weather: "sunny", period: "day", waterId: "sunmere_shallows", asset: "sunmere", label: "白天" },
    { hour: 23, weather: "clear", period: "night", waterId: "sunmere_shallows", asset: "sunmere", label: "夜晚" },
    { hour: 6, weather: "sunny", period: "dawn", waterId: "sunmere_shallows", asset: "sunmere", label: "清晨" },
    { hour: 18, weather: "sunny", period: "dusk", waterId: "willowrun_valley", asset: "willowrun", label: "黄昏" },
    { hour: 12, weather: "cloud", period: "day", waterId: "willowrun_valley", asset: "willowrun", label: "白天" },
    { hour: 12, weather: "mist", period: "day", waterId: "willowrun_valley", asset: "willowrun", label: "白天" },
    { hour: 12, weather: "rain", period: "day", waterId: "sunmere_shallows", asset: "sunmere", label: "白天" },
    { hour: 23, weather: "storm", period: "night", waterId: "sunmere_shallows", asset: "sunmere", label: "夜晚" },
    { hour: 12, weather: "snow", period: "day", waterId: "frostglass_lake", asset: "frostglass", label: "白天" }
  ]
  for (const scenario of scenarios) {
    options.scene = { gameHour: scenario.hour, weather: scenario.weather, waterId: scenario.waterId }
    await page.getByRole("button", { name: "刷新", exact: true }).click()
    await expect(scene).toHaveAttribute("data-period", scenario.period)
    await expect(scene).toHaveAttribute("data-weather", scenario.weather === "sunny" ? "clear" : scenario.weather)
    await expect(water.locator(".fishing-live-water__time")).toContainText(scenario.label)
    await expect(scene.locator("img")).toHaveAttribute("src", `/assets/scenes/fishing-${scenario.asset}-v1.webp`)
    await expect.poll(() => scene.locator("img").evaluate(element => (element as HTMLImageElement).naturalWidth)).toBe(1536)
    const precipitation = scenario.weather === "snow" ? "snow" : ["rain", "storm"].includes(scenario.weather) ? "rain" : null
    await expect(scene.locator(".fishing-backdrop__precipitation")).toHaveCount(precipitation ? 1 : 0)
    if (precipitation) await expect(scene.locator(".fishing-backdrop__precipitation")).toHaveClass(new RegExp(`is-${precipitation}`))
    await expect(scene.locator(".fishing-backdrop__celestial")).toHaveCount(["sunny", "clear"].includes(scenario.weather) ? 1 : 0)
    await expect(water.locator(".fishing-live-bobber")).toBeVisible()
    await expect(page.getByRole("button", { name: "立即收竿" })).toBeEnabled()
    await expectNoHorizontalOverflow(page)
    await page.waitForTimeout(1500)
    await water.screenshot({ path: testInfo.outputPath(`fishing-${scenario.period}-${scenario.weather}.png`) })
  }
  await page.setViewportSize({ width: 360, height: 800 })
  await expectNoHorizontalOverflow(page)
  const head = water.locator(".fishing-live-water__head")
  expect(await head.evaluate(element => element.scrollWidth <= element.clientWidth + 1)).toBe(true)
  await page.screenshot({ path: testInfo.outputPath("fishing-snow-mobile.png"), fullPage: true })
})

test("live fishing weather remains legible with reduced motion", async ({ page }, testInfo) => {
  await page.emulateMedia({ reducedMotion: "reduce" })
  await mockFishing(page, true, undefined, [], undefined, undefined, [], { scene: { gameHour: 23, weather: "rain" } })
  await page.goto("/game/fishing")
  await dismissBuildNotice(page)
  const rain = page.locator(".fishing-backdrop__precipitation i")
  await expect(rain).toHaveCount(26)
  expect(await rain.first().evaluate(element => getComputedStyle(element).animationName)).toBe("none")
  expect(await page.locator(".fishing-live-bobber").evaluate(element => getComputedStyle(element).animationName)).toBe("none")
  await expect(page.locator(".fishing-live-water__time")).toContainText("夜晚")
  await page.locator(".fishing-live-water").screenshot({ path: testInfo.outputPath("fishing-night-rain-reduced.png") })
})

test("live supplies use tooltip-enabled item cards and remain readable at 360px", async ({ page }) => {
  await page.setViewportSize({ width: 1440, height: 900 })
  let stopRequested = false
  await mockFishing(page, true, undefined, [], () => { stopRequested = true })
  await page.goto("/game/fishing")
  await dismissBuildNotice(page)
  await expect(page.locator(".fishing-live-water")).toBeVisible()
  await expect(page.locator(".fishing-live-water .fishing-live-bobber")).toBeVisible()
  await expect(page.locator(".fishing-backdrop__landscape")).toBeVisible()
  await expect(page.locator(".fishing-water-traces path")).toHaveCount(4)
  await expect(page.locator(".fishing-live-water")).toContainText("上钩了")

  const liveLog = page.locator(".fishing-live-log")
  await expect(liveLog.getByRole("heading", { name: "钓行实况" })).toBeVisible()
  await expect(liveLog).toContainText("离线也会继续")
  await expect(liveLog).not.toContainText("在线记录中")
  await expect(liveLog).toContainText("钓到了莲纹锦鲤")
  await expect(liveLog).toContainText("2.61 kg")
  await expect(liveLog).toContainText("脱钩跑鱼")
  await expect(liveLog).toContainText("刺鱼不够牢")
  await expect(liveLog).toContainText("断线跑鱼")
  await expect(liveLog).toContainText("拉力超过安全负荷")
  await expect(liveLog.locator("li").first()).toContainText("鱼儿上钩")
  await expect(liveLog.locator("li").last()).toContainText("第 4 竿入水")
  await page.screenshot({ path: "../output/playwright/fishing-desktop-live.png", fullPage: true })

  const supplyCards = page.locator(".fishing-live-supply-grid .labeled-frame-card")
  await expect(supplyCards).toHaveCount(5)
  const mainLineSupply = supplyCards.filter({ hasText: "编织主线" })
  await expect(mainLineSupply).toContainText("8")
  await mainLineSupply.locator(".item-card").hover()
  const supplyTooltip = page.locator(".item-popover-content").filter({ hasText: "编织主线" }).last()
  await expect(supplyTooltip).toBeVisible()
  await expect(supplyTooltip).toContainText("垂钓性能")
  await expect(supplyTooltip).toContainText(/每次搏鱼基础磨损\s*8%/)
  await page.screenshot({ path: "../output/playwright/fishing-desktop-live-supply-tooltip.png", fullPage: true })
  await page.mouse.move(180, 180)
  await expect(supplyTooltip).toBeHidden()

  await page.setViewportSize({ width: 360, height: 800 })
  await page.locator(".fishing-live-water").scrollIntoViewIfNeeded()
  await page.screenshot({ path: "../output/playwright/fishing-mobile-live-scene.png" })
  await expect(page.getByText("当前阶段")).toBeVisible()
  await expect(page.getByText("鱼线状态")).toBeVisible()
  await expect(page.getByText("鱼体力")).toBeVisible()
  await expect(page.getByText("剩余耗材")).toBeVisible()
  await expect(page.locator(".fishing-live-supply-grid .item-card")).toHaveCount(5)
  await expect(page.getByText("鱼线吃紧").first()).toBeVisible()
  await expect(page.locator(".fishing-live-signals .el-progress-bar__inner")).toBeVisible()
  await page.locator(".fishing-live-supplies").screenshot({ path: "../output/playwright/fishing-mobile-live-supplies.png" })
  const shortButtons = await page.locator(".fishing-page button:visible").evaluateAll((buttons) => buttons
    .filter((button) => {
      const box = button.getBoundingClientRect()
      const inViewport = box.bottom > 0 && box.top < window.innerHeight && box.right > 0 && box.left < window.innerWidth
      return inViewport && (box.width < 44 || box.height < 44)
    })
    .map((button) => ({ label: button.getAttribute("aria-label") || button.textContent?.trim(), box: button.getBoundingClientRect().toJSON() })))
  expect(shortButtons).toEqual([])
  await expectNoHorizontalOverflow(page)
  await page.screenshot({ path: "../output/playwright/fishing-mobile-live.png", fullPage: true })

  await page.getByRole("button", { name: "立即收竿" }).click()
  await page.getByRole("button", { name: "立即收竿" }).last().click()
  await expect.poll(() => stopRequested).toBe(true)
  await expect(page.locator(".fishing-live-water")).toHaveCount(0)
})

test("live scene celebrates a landed fish and explains a lost hook", async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 900 })
  let pushTrip: ((trip: Record<string, unknown>) => void) | undefined
  await mockFishing(page, true, undefined, [], undefined, undefined, [], {
    onFishingSocketReady: (push) => { pushTrip = push }
  })

  await page.goto("/game/fishing")
  await dismissBuildNotice(page)
  await expect.poll(() => Boolean(pushTrip)).toBe(true)
  const water = page.locator(".fishing-live-water")
  const landedTrip = currentTrip()
  pushTrip?.({ ...landedTrip, phase: "landed", currentFish: undefined })
  await expect(water).toContainText("钓到了莲纹锦鲤")
  await expect(water).toContainText("2.61 kg")
  await expect(water.locator(".fishing-live-catch-thumb")).toHaveCount(0)
  await expect(page.locator(".fishing-live-supply-grid .game-icon-image")).toHaveCount(5)
  await page.screenshot({ path: "../output/playwright/fishing-desktop-live-landed.png" })

  const escapedTrip = currentTrip()
  pushTrip?.({
    ...escapedTrip,
    phase: "resting",
    currentFish: undefined,
    summary: {
      ...escapedTrip.summary,
      recentEvents: [
        ...escapedTrip.summary.recentEvents,
        { sequence: 14, kind: "escaped", reason: "hook_slipped", at: new Date().toISOString(), castNumber: 7, sizeClass: "large" }
      ]
    }
  })
  await expect(water).toContainText("脱钩跑鱼")
  await expect(water).toContainText("刺鱼不够牢")
  await expect(page.locator(".fishing-live-supply-grid .game-icon-image")).toHaveCount(5)
  await page.screenshot({ path: "../output/playwright/fishing-desktop-live-escaped.png" })
})

test("home queue summarizes the whole fishing trip on desktop and mobile", async ({ page }) => {
  await page.setViewportSize({ width: 1440, height: 900 })
  await mockFishing(page, true, undefined, [], undefined, undefined, regularPendingQueueItems())
  await page.goto("/game/home")
  await dismissBuildNotice(page)

  const fishingCard = page.locator(".queue-card--fishing")
  await expect(fishingCard).toBeVisible()
  await expect(page.locator(".queue-shell--fishing")).toContainText("预计收竿")
  await expect(fishingCard).toContainText("正在睡莲湾垂钓")
  await expect(fishingCard).toContainText("正在控鱼")
  await expect(fishingCard).toContainText("晴湾浅滩")
  await expect(fishingCard).toContainText("晴湾轻钓组")
  await expect(fishingCard).toContainText("浮漂钓")
  await expect(fishingCard).toContainText("3 尾")
  await expect(fishingCard).toContainText("4.82 kg")
  await expect(fishingCard).toContainText("鱼线吃紧")
  await expect(fishingCard.locator(".fishing-queue-metric").filter({ hasText: "鱼饵" })).toContainText("57")
  await expect(fishingCard.getByRole("button", { name: "立即收竿" })).toBeVisible()
  await expect(page.getByText("fishing.trip")).toHaveCount(0)
  const regularCards = page.locator(".queue-card:not(.queue-card--fishing)")
  await expect(regularCards).toHaveCount(3)
  const regularCardLayout = await regularCards.evaluateAll((cards) => cards.map((card) => {
    const cardBox = card.getBoundingClientRect()
    const identityBox = card.querySelector(".queue-card__identity")?.getBoundingClientRect()
    const actionsBox = card.querySelector(".queue-card__pending-actions")?.getBoundingClientRect()
    const metaBox = card.querySelector(".queue-card__meta-line")?.getBoundingClientRect()
    return {
      cardWidth: cardBox.width,
      identityWidth: identityBox?.width || 0,
      actionsRight: actionsBox?.right || 0,
      cardRight: cardBox.right,
      metaHeight: metaBox?.height || 0
    }
  }))
  expect(regularCardLayout.every((item) => item.identityWidth >= 280)).toBe(true)
  expect(regularCardLayout.every((item) => item.actionsRight <= item.cardRight + 1)).toBe(true)
  expect(regularCardLayout.every((item) => item.metaHeight <= 24)).toBe(true)
  await expectNoHorizontalOverflow(page)
  await page.screenshot({ path: "../output/playwright/fishing-queue-desktop.png", fullPage: true })

  await page.setViewportSize({ width: 360, height: 800 })
  await expect(fishingCard).toBeVisible()
  await expectNoHorizontalOverflow(page)
  const shortControls = await fishingCard.locator("button:visible").evaluateAll((buttons) => buttons
    .filter((button) => {
      const box = button.getBoundingClientRect()
      return box.width < 44 || box.height < 44
    })
    .map((button) => ({ label: button.textContent?.trim(), width: button.getBoundingClientRect().width, height: button.getBoundingClientRect().height })))
  expect(shortControls).toEqual([])
  await page.screenshot({ path: "../output/playwright/fishing-queue-mobile.png", fullPage: true })
})

test("loadout editor uses item slots and a reusable tackle picker", async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 800 })
  await mockFishing(page)
  await page.goto("/game/fishing")
  await dismissBuildNotice(page)
  await page.getByRole("tab", { name: "钓组" }).click()
  await page.getByRole("button", { name: "新建钓组" }).click()

  await expect(page.locator(".fishing-component-field")).toHaveCount(7)
  await expect(page.getByRole("button", { name: "选择鱼竿" })).toBeVisible()
  await expect(page.getByRole("button", { name: "保存钓组" })).toBeDisabled()

  await page.getByRole("button", { name: "选择鱼竿" }).click()
  const rodDialog = page.getByRole("dialog", { name: "选择鱼竿" })
  await expect(page.getByRole("heading", { name: "选择鱼竿" }).last()).toBeVisible()
  await page.waitForTimeout(350)
  await page.screenshot({ path: "../output/playwright/fishing-desktop-loadout-picker.png", fullPage: true })
  const rodReference = rodDialog.getByRole("region", { name: "可选择以下类型的物品" })
  await expect(rodReference.getByText("基础鱼竿", { exact: true })).toBeVisible()
  await expect(rodReference.getByText("银叶轻漂竿", { exact: true })).toBeVisible()
  const rodChoice = rodDialog.locator('[data-tutorial="item-browser-grid"] .labeled-frame-card').filter({ hasText: "基础鱼竿" }).locator(".item-card")
  await expect(rodChoice).toHaveCount(1)
  await rodChoice.click()
  await expect(page.getByRole("heading", { name: "选择鱼竿" })).toHaveCount(0)
  await expect(page.locator(".fishing-component-field").first().getByText("基础鱼竿", { exact: true })).toBeVisible()

  await page.getByRole("button", { name: "选择主线" }).click()
  const lineChoice = page.getByRole("dialog", { name: "选择主线" }).locator('[data-tutorial="item-browser-grid"] .labeled-frame-card').filter({ hasText: "编织主线" }).locator(".item-card")
  await expect(lineChoice).toHaveCount(1)
  await lineChoice.click()
  await expect(page.locator(".fishing-component-field").nth(2).getByText("编织主线", { exact: true })).toBeVisible()
  await expectNoHorizontalOverflow(page)

  await page.getByRole("button", { name: "选择浮漂" }).click()
  const floatDialog = page.getByRole("dialog", { name: "选择浮漂" })
  const floatReference = floatDialog.getByRole("region", { name: "可选择以下类型的物品" })
  await expect(floatReference.getByText("芦苇浮漂", { exact: true })).toBeVisible()
  await expect(floatReference.getByText("滑动深水漂", { exact: true })).toBeVisible()
  await expect(floatReference.getByText("河流铅坠", { exact: true })).toHaveCount(0)
  const floatInventory = floatDialog.locator('[data-tutorial="item-browser-grid"]')
  await expect(floatInventory.getByText("河流铅坠", { exact: true })).toHaveCount(0)
  await page.waitForTimeout(350)
  await page.screenshot({ path: "../output/playwright/fishing-float-picker.png", fullPage: true })
  await floatInventory.locator(".labeled-frame-card").filter({ hasText: "芦苇浮漂" }).locator(".item-card").click()

  await page.setViewportSize({ width: 360, height: 800 })
  await page.getByRole("button", { name: "选择卷线器" }).click()
  const pickerDialog = page.getByRole("heading", { name: "选择卷线器" }).last().locator("xpath=ancestor::*[contains(concat(' ', normalize-space(@class), ' '), ' el-dialog ')][1]")
  await expect(pickerDialog).toBeVisible()
  await page.waitForTimeout(350)
  const pickerBox = await pickerDialog.boundingBox()
  expect(pickerBox).not.toBeNull()
  expect(pickerBox!.x).toBeLessThanOrEqual(1)
  expect(pickerBox!.width).toBeGreaterThanOrEqual(359)
  expect(pickerBox!.height).toBeGreaterThanOrEqual(790)
  await expectNoHorizontalOverflow(page)
  await page.screenshot({ path: "../output/playwright/fishing-mobile-loadout-picker.png", fullPage: true })
})

test("catch toolbar exposes sale, filter, and refresh as direct actions", async ({ page }, testInfo) => {
  await page.setViewportSize({ width: 820, height: 1000 })
  await mockFishing(page)
  await page.goto("/game/fishing")
  await dismissBuildNotice(page)
  await page.getByRole("tab", { name: "渔获" }).click()

  const catchActions = page.locator(".fishing-heading-actions")
  const catchActionButtons = catchActions.getByRole("button")
  await expect(catchActionButtons).toHaveCount(4)
  await expect(catchActionButtons.nth(0)).toHaveAccessibleName("批量出售")
  await expect(catchActionButtons.nth(1)).toHaveAccessibleName("放入池塘")
  await expect(catchActionButtons.nth(2)).toHaveAccessibleName("筛选")
  await expect(catchActionButtons.nth(3)).toHaveAccessibleName("刷新")
  await expect(catchActionButtons.locator("svg")).toHaveCount(4)
  await expect(catchActionButtons.nth(0).locator("svg > *").first()).toBeAttached()

  await catchActions.getByRole("button", { name: "筛选" }).click()
  await expect(page.getByRole("dialog", { name: "筛选" }).locator(".fish-filter-controls")).toBeVisible()
  await page.keyboard.press("Escape")

  await catchActions.getByRole("button", { name: "批量出售" }).click()
  const exitBatchButton = catchActions.getByRole("button", { name: "退出批量" })
  await expect(exitBatchButton).toBeVisible()
  await expect(exitBatchButton.locator("svg")).toHaveCount(1)
  await expect(exitBatchButton.locator("svg > *").first()).toBeAttached()
  await expect(page.locator(".fishing-catch-item.is-batch")).toHaveCount(1)

  await exitBatchButton.click()
  await catchActions.getByRole("button", { name: "放入池塘" }).click()
  await expect(catchActions.getByRole("button", { name: "退出放入池塘" })).toBeVisible()
  await page.locator(".fishing-catch-item").first().click()
  await expect(page.getByRole("button", { name: "放入选中鱼只" })).toBeEnabled()

  await page.setViewportSize({ width: 360, height: 800 })
  await expect(catchActionButtons).toHaveCount(4)
  await expectNoHorizontalOverflow(page)
  await page.locator(".fishing-catch-toolbar").screenshot({ path: testInfo.outputPath("catch-toolbar.png") })
})

test("catch collection auto-loads every page and selects the same species beyond the virtual window", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 })
  const allCatches = repeatedCatches(96)
  allCatches[1] = { ...allCatches[1], fish: { ...allCatches[1].fish, genes: { mutations: ["prismatic"] } } }
  const requestedCursors: number[] = []
  await mockFishing(page, false, undefined, [], undefined, undefined, [], {
    catchItems: allCatches,
    catchPageSize: 17,
    onFishingCatchRequest: (cursor) => requestedCursors.push(cursor)
  })
  await page.goto("/game/fishing")
  await dismissBuildNotice(page)
  await page.getByRole("tab", { name: "渔获" }).click()

  const virtualGrid = page.locator(".item-browser-virtual-grid")
  await expect(virtualGrid).toBeVisible()
  await expect.poll(() => requestedCursors.length).toBe(6)
  await expect(page.getByRole("button", { name: "查看更多" })).toHaveCount(0)
  await expect(page.getByText("已保留", { exact: true })).toHaveCount(0)

  const renderedCatches = page.locator(".fishing-catch-item")
  await expect.poll(() => renderedCatches.count()).toBeGreaterThan(0)
  expect(await renderedCatches.count()).toBeLessThan(allCatches.length)
  const specialCatch = page.locator('.fishing-catch-item:has([data-fish-mutations~="prismatic"])').first()
  await expect(specialCatch.locator('.fishing-catch-mutations [aria-label="棱彩"]')).toBeVisible()

  await page.locator(".fishing-heading-actions").getByRole("button", { name: "批量出售" }).click()
  const recordCatch = page.locator(".fishing-catch-item:has(.fishing-catch-record)").first()
  await page.locator(".fishing-catch-item:not(:has(.fishing-catch-record)):not(:has(.fishing-catch-mutations))").first().click()
  const actionBar = page.locator(".batch-action-bar")
  await expect(actionBar).toBeVisible()
  await actionBar.getByRole("button", { name: "选择所有 莲纹锦鲤" }).click()
  await expect(actionBar).toContainText("已选 94 尾")
  await expect(actionBar.getByRole("button", { name: "获取估价 (94)" })).toBeEnabled()
  await expect(recordCatch).not.toHaveClass(/is-selected/)
  await expect(specialCatch).not.toHaveClass(/is-selected/)
  await virtualGrid.evaluate((element) => element.scrollTo({ top: element.scrollHeight }))
  await expect(page.locator(".fishing-catch-item:not(.is-selected)")).toHaveCount(0)
  await expectNoHorizontalOverflow(page)
})

test("pond selector filters every inventory page and bounds rendered cards", async ({ page }, testInfo) => {
  await page.setViewportSize({ width: 1440, height: 1000 })
  const base = pondOverview().availableFish[0]
  const candidates = Array.from({ length: 401 }, (_, index) => ({
    ...base, instanceId: 3000 - index, customData: { name: `候选鱼${index}` },
    fish: { ...base.fish, genes: { mutations: index === 400 ? ["prismatic", "shimmering"] : [] } }
  }))
  await mockFishing(page, false, undefined, [], undefined, undefined, [], { pondAvailableFish: candidates })
  await page.goto("/game/pond")
  await dismissBuildNotice(page)
  await page.getByRole("button", { name: "放入鱼只" }).click()
  const dialog = page.getByRole("dialog", { name: "放入鱼只" })
  await expect(dialog).toContainText("共 401 尾")
  await expect(dialog.locator(".fish-selector-card")).toHaveCount(36)
  await dialog.locator(".fish-selector-card").first().click()
  await dialog.locator(".btn-next").click()
  await expect(dialog.locator(".fish-selector-card").first()).toContainText("候选鱼36")
  await dialog.locator(".btn-prev").click()
  await expect(dialog.locator(".fish-selector-card").first()).toHaveAttribute("aria-pressed", "true")
  await dialog.locator(".el-select").filter({ has: page.getByRole("combobox", { name: "稀有度", exact: true }) }).click()
  await expect(page.getByRole("option")).toHaveCount(7)
  await page.keyboard.press("Escape")
  await dialog.locator(".el-select").filter({ has: page.getByRole("combobox", { name: "变异", exact: true }) }).click()
  await page.getByRole("option", { name: "闪光", exact: true }).click()
  await expect(dialog.locator(".fish-selector-card")).toHaveCount(1)
  await expect(dialog.locator(".fish-selector-card")).toContainText("候选鱼400")
  await dialog.locator(".fish-selector-card").click()
  await expect(dialog).toContainText("已选 2 尾")
  await dialog.screenshot({ path: testInfo.outputPath("pond-filters-desktop.png") })
  await page.setViewportSize({ width: 360, height: 800 })
  await expectNoHorizontalOverflow(page)
  await dialog.screenshot({ path: testInfo.outputPath("pond-filters-mobile.png") })
})

test("catch filters include rarity health and stacked mutations beyond the first page", async ({ page }, testInfo) => {
  const rows = repeatedCatches(351)
  rows[350] = { ...rows[350], fish: { ...rows[350].fish, health: .5, genes: { mutations: ["prismatic", "shimmering"] } } }
  await mockFishing(page, false, undefined, [], undefined, undefined, [], { catchItems: rows, catchPageSize: 100 })
  await page.goto("/game/fishing")
  await dismissBuildNotice(page)
  await page.getByRole("tab", { name: "渔获" }).click()
  const actions = page.locator(".fishing-heading-actions")
  await expect(page.locator(".fishing-catch-item").first()).toBeVisible()
  await actions.getByRole("button", { name: "筛选" }).click()
  const dialog = page.getByRole("dialog", { name: "筛选" })
  await dialog.locator(".el-select").filter({ has: page.getByRole("combobox", { name: "变异", exact: true }) }).click()
  await page.getByRole("option", { name: "棱彩", exact: true }).click()
  await dialog.locator(".el-select").filter({ has: page.getByRole("combobox", { name: "健康", exact: true }) }).click()
  await page.getByRole("option", { name: "需要照料", exact: true }).click()
  await dialog.locator(".el-select").filter({ has: page.getByRole("combobox", { name: "稀有度", exact: true }) }).click()
  await page.getByRole("option", { name: "史诗", exact: true }).click()
  await expect(page.locator(".el-select-dropdown:visible")).toHaveCount(0)
  await dialog.screenshot({ path: testInfo.outputPath("catch-filters-desktop.png") })
  await page.setViewportSize({ width: 360, height: 800 })
  await expectNoHorizontalOverflow(page)
  await dialog.screenshot({ path: testInfo.outputPath("catch-filters-mobile.png") })
  await dialog.getByRole("button", { name: "查看结果" }).click()
  await expect(page.locator(".fishing-catch-item")).toHaveCount(1)
  await expect(page.locator('.fishing-catch-item [data-fish-mutations~="shimmering"]').first()).toBeVisible()
  await actions.getByRole("button", { name: "筛选" }).click()
  await dialog.locator(".el-select").filter({ has: page.getByRole("combobox", { name: "变异", exact: true }) }).click()
  await page.getByRole("option", { name: "无变异", exact: true }).click()
  await dialog.getByRole("button", { name: "查看结果" }).click()
  await expect(page.locator(".fishing-catch-item")).toHaveCount(0)
  await actions.getByRole("button", { name: "筛选" }).click()
  await dialog.getByRole("button", { name: "重置筛选" }).click()
  await dialog.getByRole("button", { name: "查看结果" }).click()
  await expect(page.locator(".fishing-catch-item").first()).toBeVisible()
})

test("moving a catch into the pond removes it from the catch inventory", async ({ page }) => {
  await page.setViewportSize({ width: 820, height: 1000 })
  await mockFishing(page)
  await page.goto("/game/fishing")
  await dismissBuildNotice(page)
  await page.getByRole("tab", { name: "渔获" }).click()

  const catchItems = page.locator(".fishing-catch-item")
  const pondCandidate = catchItems.filter({ hasText: "莲纹锦鲤" })
  await expect(catchItems).toHaveCount(1)
  await expect(pondCandidate).toHaveCount(1)

  await page.locator(".fishing-heading-actions").getByRole("button", { name: "放入池塘" }).click()
  await pondCandidate.locator(".item-card").click()
  await page.getByRole("button", { name: "放入选中鱼只" }).click()

  const predationDialog = page.getByRole("dialog", { name: "池塘存在捕食关系" })
  await expect(predationDialog).toBeVisible()
  await predationDialog.getByRole("button", { name: "放入选中鱼只" }).click()

  await expect(page.getByText("选中的鱼只已放入鱼池")).toBeVisible()
  await expect(catchItems).toHaveCount(0)
  await expect(pondCandidate).toHaveCount(0)
  await expect(page.getByText("已出售", { exact: true })).toHaveCount(0)
})

test("tablet catch cards remain scannable with batch actions", async ({ page }) => {
  await page.setViewportSize({ width: 820, height: 1000 })
  let saleRequest: unknown
  await mockFishing(page, false, undefined, [], undefined, undefined, [], {
    onFishingSaleRequest: (body) => { saleRequest = body }
  })
  await page.goto("/game/fishing")
  await dismissBuildNotice(page)
  await page.getByRole("tab", { name: "渔获" }).click()
  const catches = page.locator(".fishing-catch-item")
  await expect(catches).toHaveCount(1)
  const catchCardBoxes = await catches.evaluateAll((cards) => cards.map((card) => {
    const box = card.getBoundingClientRect()
    return { width: box.width, height: box.height }
  }))
  expect(catchCardBoxes.every((box) => box.width <= 80 && box.height <= 112)).toBe(true)
  await expect(page.locator(".fishing-catch-item .item-card")).toHaveCount(1)
  await expect(page.locator(".fishing-catch-select, .fishing-catch-rarity")).toHaveCount(0)
  await expect(page.getByText("已出售", { exact: true })).toHaveCount(0)

  const firstFishCard = catches.first().locator(".item-card")
  await firstFishCard.hover()
  const fishTooltip = page.locator(".el-popper").filter({ has: page.locator(".item-popover-fish-summary") }).last()
  await expect(fishTooltip).toBeVisible()
  await expect(fishTooltip.getByText("48.3 cm")).toBeVisible()
  await expect(fishTooltip.getByText("2.61 kg")).toBeVisible()
  await expect(fishTooltip.locator('[data-fish-size-metric="length"]')).toHaveAttribute("data-arrow-count", "1")
  await expect(fishTooltip.locator('[data-fish-size-metric="weight"]')).toHaveAttribute("data-arrow-count", "3")
  await page.mouse.move(10, 10)

  await firstFishCard.click()
  await expect(page.getByRole("heading", { name: "莲纹锦鲤" }).last()).toBeVisible()
  const catchDetail = page.getByLabel("这尾鱼", { exact: true })
  await expect(catchDetail.getByText("2 年 3 个月")).toBeVisible()
  await expect(catchDetail.getByText("丰润", { exact: true })).toBeVisible()
  await expect(catchDetail.getByText("中层 / 近底层", { exact: true })).toBeVisible()
  await expect(catchDetail.locator(".fishing-layer-profile > .is-active")).toHaveCount(2)
  await expect(catchDetail.locator('[data-fish-size-metric="length"]')).toHaveAttribute("data-arrow-count", "1")
  await expect(catchDetail.locator('[data-fish-size-metric="weight"]')).toHaveAttribute("data-arrow-count", "3")
  await page.waitForTimeout(500)
  await page.screenshot({ path: "../output/playwright/fishing-tablet-catch-detail.png", fullPage: true })
  await page.keyboard.press("Escape")
  await page.waitForTimeout(350)

  await page.locator(".fishing-heading-actions").getByRole("button", { name: "批量出售" }).click()
  await expect(page.locator(".fishing-catch-check")).toHaveCount(0)
  await catches.first().locator(".item-card").click()
  await expect(page.locator(".fishing-catch-check")).toHaveCount(1)
  const saleButton = page.getByRole("button", { name: "获取估价" })
  await expect(saleButton).toBeVisible()
  await expect(saleButton).toBeEnabled()
  await expect(page.getByText("先看清估价，再决定出售")).toHaveCount(0)
  await saleButton.click()
  const saleDialog = page.getByRole("dialog", { name: "渔获估价" })
  await expect(saleDialog).toBeVisible()
  await expect(saleDialog.getByRole("heading", { name: "报价依据" })).toBeVisible()
  await expect(saleDialog.getByText("鱼种基价", { exact: true })).toHaveCount(2)
  await expect(saleDialog.getByText("个体品相", { exact: true })).toBeVisible()
  await expect(saleDialog.getByText("每日开市时按鱼种独立轮换，当天固定，次日更新，不受玩家出售影响。", { exact: true })).toBeVisible()
  await expect(saleDialog.getByText("鱼商按当天收货情况温和调整；收得较少的鱼种会有更好报价。", { exact: true })).toBeVisible()
  await expect(saleDialog.getByText("今日行情 = 今日偏好 × 收货余量，整体只在 90%–110% 之间温和浮动。", { exact: true })).toBeVisible()
  await expect(saleDialog.locator(".fishing-sale-guide__icon svg > *").first()).toBeVisible()
  await expect(saleDialog.locator(".fishing-sale-line__picture img")).toBeVisible()
  await expect(saleDialog.getByText("4,093", { exact: true })).toHaveCount(1)
  await expect(saleDialog.locator(".fishing-sale-total")).toContainText("4,093")
  await page.waitForTimeout(300)
  await page.screenshot({ path: "../output/playwright/fishing-tablet-sale-quote.png", fullPage: true })
  await saleDialog.getByRole("button", { name: "确认出售" }).click()
  await expect.poll(() => saleRequest).toMatchObject({ instanceIds: [503], expectedTotalGold: 4093 })
  await expect(page.getByText("渔获已出售，获得 4,093 金币")).toBeVisible()
  await expect(catches).toHaveCount(0)
  await expect(page.getByText("仓库中没有渔获")).toBeVisible()
  await page.locator(".fishing-subnav").getByText("纪录", { exact: true }).click()
  await expect(page.getByText("个人纪录", { exact: true })).toBeVisible()
  await expect(page.locator(".fishing-personal-record").getByText("48.3 cm")).toBeVisible()
  await expect(page.locator(".fishing-personal-record").getByText("2.61 kg")).toBeVisible()
  await expectNoHorizontalOverflow(page)
  await page.screenshot({ path: "../output/playwright/fishing-tablet-catches.png", fullPage: true })
})

test("mutation effects follow one fish through card detail and sale quote", async ({ page }) => {
  await page.setViewportSize({ width: 820, height: 1000 })
  const mutations = ["prismatic", "rainbow", "shimmering"]
  const catchItems = repeatedCatches(12).map((row) => ({
    ...row,
    record: { ...row.record, length: false, weight: false }
  }))
  const inventoryOnlyInstanceId = 30_000
  const inventoryItems = [{
    instanceId: inventoryOnlyInstanceId,
    templateItemId: "fish_catch",
    quantity: 1,
    state: "inventory",
    fishData: { ...catchItems[0].fish, genes: { mutations } }
  }]
  await mockFishing(page, false, undefined, inventoryItems, undefined, undefined, [], {
    catchItems,
    fishingSaleQuoteMutations: mutations
  })
  await page.goto("/game/fishing")
  await dismissBuildNotice(page)
  await page.getByRole("tab", { name: "渔获" }).click()

  expect(catchItems.some((row) => row.inventoryInstanceId === inventoryOnlyInstanceId)).toBe(false)
  await expect(page.locator(".fishing-catch-item")).toHaveCount(catchItems.length + 1)
  const specialCatch = page.locator('.fishing-catch-item:has([data-fish-mutations~="prismatic"])')
  await expect(specialCatch).toHaveCount(1)
  const fishCard = specialCatch.locator(".item-card")
  const cardArt = fishCard.locator(".fish-mutation-art.has-stacked-fish-mutations")
  await expect(cardArt).toHaveAttribute("data-fish-mutations", "prismatic rainbow shimmering")
  await expect(cardArt.locator(".fish-mutation-art__prism")).toBeVisible()
  await expect(cardArt.locator(".fish-mutation-art__rainbow")).toBeVisible()
  await expect(cardArt.locator(".fish-mutation-art__glints")).toBeVisible()
  const [cardPrismMotion, cardRainbowMotion] = await Promise.all([
    cardArt.locator(".fish-mutation-art__prism"),
    cardArt.locator(".fish-mutation-art__rainbow")
  ].map((locator) => locator.evaluate((element) => {
    const style = getComputedStyle(element)
    return { name: style.animationName, timing: style.animationTimingFunction, iterations: style.animationIterationCount }
  })))
  expect(cardPrismMotion).toMatchObject({ timing: "linear", iterations: "infinite" })
  expect(cardPrismMotion.name).toContain("fish-prismatic-flow")
  expect(cardRainbowMotion).toMatchObject({ timing: "linear", iterations: "infinite" })
  expect(cardRainbowMotion.name).toContain("fish-rainbow-flow")
  const cardGlints = cardArt.locator(".fish-mutation-art__glints i")
  await expect(cardGlints).toHaveCount(5)
  expect(await cardGlints.evaluateAll((elements) => elements.every((element) => Number(getComputedStyle(element).opacity) >= .75))).toBe(true)
  const mutationBadges = fishCard.locator("xpath=ancestor::*[contains(concat(' ', normalize-space(@class), ' '), ' fishing-catch-item ')][1]").locator(".fishing-catch-mutations > span")
  await expect(mutationBadges).toHaveCount(3)
  await expect(mutationBadges.nth(0)).toHaveAttribute("aria-label", "棱彩")
  await expect(mutationBadges.nth(1)).toHaveAttribute("aria-label", "彩虹")
  await expect(mutationBadges.nth(2)).toHaveAttribute("aria-label", "闪光")
  await page.screenshot({ path: "../output/playwright/fishing-special-catch-grid.png", fullPage: true })

  await fishCard.click()
  const catchDetail = page.getByLabel("这尾鱼", { exact: true })
  const detailArt = catchDetail.locator(".fishing-catch-portrait .fish-mutation-art.has-stacked-fish-mutations")
  await expect(detailArt).toHaveAttribute("data-fish-mutations", "prismatic rainbow shimmering")
  await expectNoHorizontalOverflow(page)
  await page.waitForTimeout(350)
  await page.screenshot({ path: "../output/playwright/fishing-special-catch-detail.png", fullPage: true })

  await page.keyboard.press("Escape")
  await expect(catchDetail).toBeHidden()
  await page.locator(".fishing-heading-actions").getByRole("button", { name: "批量出售" }).click()
  await fishCard.click()
  await page.getByRole("button", { name: "获取估价" }).click()
  const saleDialog = page.getByRole("dialog", { name: "渔获估价" })
  const saleArt = saleDialog.locator(".fishing-sale-line__picture .fish-mutation-art.has-stacked-fish-mutations")
  await expect(saleArt).toHaveAttribute("data-fish-mutations", "prismatic rainbow shimmering")
  await expect(saleDialog.getByText("特殊异变", { exact: true })).toBeVisible()
  await expect(saleDialog.getByText("×4.000", { exact: true })).toBeVisible()
  await page.waitForTimeout(350)
  await saleDialog.screenshot({ path: "../output/playwright/fishing-special-sale-quote.png" })
})

test("welcome back highlights special catches with their mutation effects", async ({ page }) => {
  await page.setViewportSize({ width: 820, height: 1000 })
  await mockFishing(page, false, undefined, [], undefined, undefined, [], {
    offlineRewards: {
      completedSteps: 18,
      offlineDurationSeconds: 7_200,
      fishingMutations,
      fishingCatches: {
        lotus_koi: {
          landed: 4,
          kept: 3,
          released: 1,
          totalWeightKg: 8.72,
          mutatedLanded: 2,
          mutatedKept: 1,
          mutatedReleased: 1,
          mutationGroups: [
            { mutations: ["shimmering", "prismatic", "rainbow"], landed: 1, kept: 1, released: 0 },
            { mutations: ["shimmering"], landed: 1, kept: 0, released: 1 }
          ]
        }
      }
    }
  })
  await page.goto("/game/fishing")
  await dismissBuildNotice(page)

  const welcomeDialog = page.getByRole("dialog").filter({ hasText: "欢迎回来！" })
  await expect(welcomeDialog).toBeVisible()
  const specialSection = welcomeDialog.locator(".fast-forward-special-catches")
  await expect(specialSection.getByText("离线特殊渔获", { exact: true })).toBeVisible()
  await expect(specialSection.locator(".fast-forward-special-catch")).toHaveCount(2)
  const stackedArt = specialSection.locator(".fish-mutation-art.has-stacked-fish-mutations")
  await expect(stackedArt).toHaveAttribute("data-fish-mutations", "prismatic rainbow shimmering")
  await expect(stackedArt.locator(".fish-mutation-art__prism")).toBeVisible()
  await expect(stackedArt.locator(".fish-mutation-art__rainbow")).toBeVisible()
  await expect(stackedArt.locator(".fish-mutation-art__glints")).toBeVisible()
  await expect(specialSection.getByText("棱彩", { exact: true })).toBeVisible()
  await expect(specialSection.getByText("彩虹", { exact: true })).toBeVisible()
  await expect(specialSection.getByText("闪光", { exact: true })).toHaveCount(2)
  await expectNoHorizontalOverflow(page)
  await page.waitForTimeout(500)
  await welcomeDialog.screenshot({ path: "../output/playwright/fishing-special-offline-welcome.png" })

  await page.setViewportSize({ width: 390, height: 844 })
  await expectNoHorizontalOverflow(page)
  const specialCards = specialSection.locator(".fast-forward-special-catch")
  const cardPositions = await specialCards.evaluateAll((elements) => elements.map((element) => {
    const rect = element.getBoundingClientRect()
    return { top: rect.top, bottom: rect.bottom }
  }))
  expect(cardPositions[1].top).toBeGreaterThanOrEqual(cardPositions[0].bottom)
  await welcomeDialog.screenshot({ path: "../output/playwright/fishing-special-offline-welcome-mobile.png" })
})

test("changed sale total refreshes the quote before any sale", async ({ page }) => {
  await page.setViewportSize({ width: 820, height: 1000 })
  const saleRequests: unknown[] = []
  await mockFishing(page, false, undefined, [], undefined, undefined, [], {
    fishingSaleQuoteChangesOnce: true,
    onFishingSaleRequest: (body) => { saleRequests.push(body) }
  })
  await page.goto("/game/fishing")
  await dismissBuildNotice(page)
  await page.getByRole("tab", { name: "渔获" }).click()
  await page.locator(".fishing-heading-actions").getByRole("button", { name: "批量出售" }).click()
  await page.locator(".fishing-catch-item").first().locator(".item-card").click()
  await page.getByRole("button", { name: "获取估价" }).click()

  const saleDialog = page.getByRole("dialog", { name: "渔获估价" })
  await expect(saleDialog.getByText("4,093", { exact: true })).toHaveCount(1)
  await expect(saleDialog.locator(".fishing-sale-total")).toContainText("4,093")
  await saleDialog.getByRole("button", { name: "确认出售" }).click()

  await expect(page.getByText("结算前行情发生变化，已刷新报价，请重新确认。")).toBeVisible()
  await expect(saleDialog.getByText("4,101", { exact: true })).toHaveCount(1)
  await expect(saleDialog.locator(".fishing-sale-total")).toContainText("4,101")
  expect(saleRequests).toHaveLength(1)
  expect(saleRequests[0]).toMatchObject({ instanceIds: [503], expectedTotalGold: 4093 })
})

test("batch sale explains and skips immature cultured fish", async ({ page }, testInfo) => {
  await page.setViewportSize({ width: 820, height: 1000 })
  const rows = repeatedCatches(7)
  rows[0] = { ...rows[0], saleEligible: true }
  rows[6] = {
    ...rows[6],
    saleEligible: false,
    saleIneligibleReason: "cultured_immature",
    saleCurrentAgeDays: 255,
    saleRequiredAgeDays: 600,
    fish: { ...rows[6].fish, ageDays: 255, origin: "cultured", lifeState: "alive" }
  }
  await mockFishing(page, false, undefined, [], undefined, undefined, [], { catchItems: rows })
  await page.goto("/game/fishing")
  await dismissBuildNotice(page)
  await page.getByRole("tab", { name: "渔获" }).click()

  const catchCards = page.locator(".fishing-catch-item")
  const immature = catchCards.filter({ has: page.locator(".fishing-catch-immature") })
  await expect(immature).toHaveCount(1)
  await expect(immature.locator(".fishing-catch-immature")).toHaveAttribute("title", "这尾养殖鱼尚未成熟（255/600 天），成熟后才能卖给鱼商。")
  await immature.locator(".item-card").hover()
  const fishTooltip = page.locator(".el-popper").filter({ hasText: "这尾养殖鱼尚未成熟（255/600 天），成熟后才能卖给鱼商。" }).last()
  await expect(fishTooltip).toBeVisible()
  await expect(fishTooltip.locator(".item-popover-fish-sale-status")).toHaveText("这尾养殖鱼尚未成熟（255/600 天），成熟后才能卖给鱼商。")
  await page.screenshot({ path: testInfo.outputPath("fishing-sale-immature-tooltip.png"), fullPage: true })
  await page.mouse.move(10, 10)

  await page.locator(".fishing-heading-actions").getByRole("button", { name: "批量出售" }).click()
  await expect(page.locator(".batch-action-bar")).toBeVisible()
  await expect(page.locator(".batch-action-bar__hint")).toContainText("另有 1 尾未成熟养殖鱼暂不可出售")
  await expect(immature).toHaveAttribute("aria-label", "这尾养殖鱼尚未成熟（255/600 天），成熟后才能卖给鱼商。")
  await expect(immature).toHaveAttribute("aria-disabled", "true")
  await expect(page.locator(".fishing-catch-check")).toHaveCount(0)

  await catchCards.first().click()
  await page.getByRole("button", { name: "选择所有 莲纹锦鲤" }).click()
  await expect(page.locator(".fishing-catch-check")).toHaveCount(6)
  await expect(immature.locator(".fishing-catch-check")).toHaveCount(0)
  await page.waitForTimeout(250)
  await page.screenshot({ path: testInfo.outputPath("fishing-sale-ineligible.png"), fullPage: true })

  const quoteRequest = page.waitForRequest((request) => new URL(request.url()).pathname === "/api/fishing/quote")
  await page.getByRole("button", { name: "获取估价" }).click()
  expect((await quoteRequest).postDataJSON()).toMatchObject({ instanceIds: [20_000, 20_001, 20_002, 20_003, 20_004, 20_005] })
})

test("large sale quotes keep every fish row readable in the fullscreen tablet dialog", async ({ page }) => {
  await page.setViewportSize({ width: 718, height: 1024 })
  await mockFishing(page, false, undefined, [], undefined, undefined, [], {
    fishingSaleQuoteItemCount: 24
  })
  await page.goto("/game/fishing")
  await dismissBuildNotice(page)
  await page.getByRole("tab", { name: "渔获" }).click()
  await page.locator(".fishing-heading-actions").getByRole("button", { name: "批量出售" }).click()
  await page.locator(".fishing-catch-item").first().locator(".item-card").click()
  await page.getByRole("button", { name: "获取估价" }).click()

  const saleDialog = page.getByRole("dialog", { name: "渔获估价" })
  const saleLines = saleDialog.locator(".fishing-sale-line")
  await expect(saleLines).toHaveCount(24)
  await expect(saleDialog.getByText("每日开市时按鱼种独立轮换，当天固定，次日更新，不受玩家出售影响。", { exact: true })).toBeVisible()
  await expect(saleDialog.getByText("鱼商按当天收货情况温和调整；收得较少的鱼种会有更好报价。", { exact: true })).toBeVisible()
  const rowHeights = await saleLines.evaluateAll((rows) => rows.map((row) => row.getBoundingClientRect().height))
  expect(Math.min(...rowHeights)).toBeGreaterThan(130)
  await expect(saleDialog.locator(".fishing-sale-dialog__footer")).toBeVisible()
  await expectNoHorizontalOverflow(page)
  await page.waitForTimeout(350)
  await page.screenshot({ path: "../output/playwright/fishing-tablet-sale-quote-many.png", fullPage: true })
  await saleLines.last().scrollIntoViewIfNeeded()
  await expect(saleLines.last().getByText("莲纹锦鲤", { exact: true })).toBeVisible()
})

test("360px catch list stays compact with batch actions", async ({ page }) => {
  await page.setViewportSize({ width: 360, height: 800 })
  await mockFishing(page)
  await page.goto("/game/fishing")
  await dismissBuildNotice(page)
  await page.getByRole("tab", { name: "渔获" }).click()

  const cards = page.locator(".fishing-catch-item")
  await expect(cards).toHaveCount(1)
  const boxes = await cards.evaluateAll((rows) => rows.map((row) => {
    const box = row.getBoundingClientRect()
    return { width: box.width, height: box.height }
  }))
  expect(boxes.every((box) => box.width <= 76 && box.height <= 112)).toBe(true)
  await expect(page.locator(".fishing-catch-select, .fishing-catch-rarity")).toHaveCount(0)
  await cards.first().locator(".item-card").click()
  const mobileDetail = page.getByLabel("这尾鱼", { exact: true })
  await expect(mobileDetail.getByRole("heading", { name: "莲纹锦鲤" })).toBeVisible()
  await expect(mobileDetail.locator(".fishing-layer-profile > .is-active")).toHaveCount(2)
  await expectNoHorizontalOverflow(page)
  await page.waitForTimeout(350)
  await page.screenshot({ path: "../output/playwright/fishing-mobile-catch-detail.png", fullPage: true })
  await mobileDetail.locator(".el-dialog__body").evaluate((element) => element.scrollTo({ top: element.scrollHeight }))
  await expect(mobileDetail.locator(".fishing-catch-profile > footer")).toBeInViewport()
  await page.keyboard.press("Escape")
  await expect(mobileDetail).toBeHidden()
  await page.locator(".fishing-heading-actions").getByRole("button", { name: "批量出售" }).click()
  await cards.first().locator(".item-card").click()
  await expect(page.locator(".fishing-catch-check")).toHaveCount(1)
  const estimateButton = page.getByRole("button", { name: "获取估价" })
  await expect(estimateButton).toBeEnabled()
  await page.waitForTimeout(300)
  await expectNoHorizontalOverflow(page)
  await page.screenshot({ path: "../output/playwright/fishing-mobile-catches.png", fullPage: true })
  await estimateButton.click()
  const saleDialog = page.getByRole("dialog", { name: "渔获估价" })
  await expect(saleDialog).toBeVisible()
  await expect(saleDialog.getByText("个体品相", { exact: true })).toBeVisible()
  await expect(saleDialog.getByText("每日开市时按鱼种独立轮换，当天固定，次日更新，不受玩家出售影响。", { exact: true })).toBeVisible()
  await expect(saleDialog.getByText("鱼商按当天收货情况温和调整；收得较少的鱼种会有更好报价。", { exact: true })).toBeVisible()
  await expect(saleDialog.locator(".fishing-sale-guide__icon svg > *").first()).toBeVisible()
  await page.waitForTimeout(350)
  await expectNoHorizontalOverflow(page)
  await page.screenshot({ path: "../output/playwright/fishing-mobile-sale-quote.png", fullPage: true })
})

test("catch cells match warehouse sizing and spacing on desktop and mobile", async ({ page }) => {
  await mockFishing(page, false, undefined, [warehouseFish])

  for (const viewport of [
    { width: 1280, height: 800, cardSize: 56, gap: 16 },
    { width: 360, height: 800, cardSize: 48, gap: 8 }
  ]) {
    await page.setViewportSize(viewport)
    await page.goto("/game/fishing")
    await dismissBuildNotice(page)
    await page.getByRole("tab", { name: "渔获" }).click()

    const catchItem = page.locator(".fishing-catch-item").first()
    const catchMetrics = await itemGridMetrics(catchItem)
    await expect(catchItem.locator(".fishing-catch-record")).toBeVisible()

    await page.goto("/game/warehouse")
    await dismissBuildNotice(page)
    const warehouseMetrics = await itemGridMetrics(page.locator(".batch-card-wrapper").first())

    expect(catchMetrics).toEqual(warehouseMetrics)
    expect(catchMetrics.cardWidth).toBe(viewport.cardSize)
    expect(catchMetrics.cardHeight).toBe(viewport.cardSize)
    expect(catchMetrics.columnGap).toBe(viewport.gap)
    expect(catchMetrics.rowGap).toBe(viewport.gap)
  }
})

test("warehouse lists individual fish and reuses the item tooltip", async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 800 })
  await mockFishing(page, false, undefined, [warehouseFish])
  await page.goto("/game/warehouse")
  await dismissBuildNotice(page)

  await expect(page.getByRole("tab", { name: "渔获" })).toBeVisible()
  const fishCard = page.locator(".item-card").filter({ has: page.locator('img[alt="莲纹锦鲤"]') }).first()
  await expect(fishCard).toBeVisible()
  await fishCard.hover()
  const tooltip = page.locator(".el-popper").filter({ has: page.locator(".item-popover-fish-summary") }).last()
  await expect(tooltip).toBeVisible()
  await expect(tooltip.getByText("48.3 cm")).toBeVisible()
  await expect(tooltip.getByText("2.61 kg")).toBeVisible()
  await expect(tooltip.locator('[data-fish-size-metric="length"]')).toHaveAttribute("data-arrow-count", "1")
  await expect(tooltip.locator('[data-fish-size-metric="weight"]')).toHaveAttribute("data-arrow-count", "3")
  await expect(tooltip.locator('[data-fish-size-metric="length"] svg')).toHaveCount(1)
  await expect(tooltip.locator('[data-fish-size-metric="length"] svg')).toBeVisible()
  await expect(tooltip.locator('[data-fish-size-metric="weight"] svg')).toHaveCount(3)
  await expect(tooltip.locator('[data-fish-size-metric="weight"] svg').first()).toBeVisible()
  await expect(tooltip.getByText("晴湾浅滩 · 睡莲湾")).toBeVisible()
  await expectNoHorizontalOverflow(page)
  await page.screenshot({ path: "../output/playwright/warehouse-fish-tooltip.png", fullPage: true })
})

test("warehouse fishing gear category includes tackle, values, and preferences", async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 800 })
  await mockFishing(page, false, undefined, [basicFishingRodInventory, braidedMainLineInventory, sweetDoughBaitInventory, starrippleBaitInventory, warehouseFish])
  await page.goto("/game/warehouse")
  await dismissBuildNotice(page)

  const fishingGearTab = page.getByRole("tab", { name: "渔具", exact: true })
  await expect(fishingGearTab).toBeVisible()
  await fishingGearTab.click()

  const toolsTrigger = page.locator('[data-tutorial="warehouse-items"] [data-tutorial="item-browser-tools-trigger"]')
  await toolsTrigger.click()
  const toolsMenu = page.locator(".el-dropdown__popper:visible").filter({ hasText: "二级分类" })
  await expect(toolsMenu.getByText("二级分类", { exact: true })).toBeVisible()
  const secondaryCategorySwitch = toolsMenu.getByRole("switch")
  await expect(secondaryCategorySwitch).toHaveAttribute("aria-checked", "true")
  await toolsTrigger.click()
  await expect(toolsMenu).toBeHidden()

  const sectionTitles = page.locator('[data-tutorial="warehouse-items"] .item-browser-section-title')
  await expect(sectionTitles.getByText("鱼竿", { exact: true })).toBeVisible()
  await expect(sectionTitles.getByText("主线", { exact: true })).toBeVisible()
  await expect(sectionTitles.getByText("鱼饵", { exact: true })).toBeVisible()
  await page.screenshot({ path: "../output/playwright/warehouse-fishing-gear-secondary.png", fullPage: true })
  await expect(page.getByText("基础鱼竿", { exact: true }).first()).toBeVisible()
  await expect(page.getByText("编织主线", { exact: true }).first()).toBeVisible()
  const baitCard = page.locator(".labeled-frame-card").filter({ hasText: "甜谷面饵" }).locator(".item-card").first()
  await expect(baitCard).toBeVisible()
  await baitCard.hover()
  const baitTooltip = page.locator(".item-popover-content").filter({ hasText: "甜谷面饵" }).last()
  await expect(baitTooltip).toContainText(/鱼口活跃度\s*\+4%/)
  await expect(baitTooltip).toContainText("遇见鲤鱼权重 +45%")
  await expect(baitTooltip).toContainText("遇见乌鳢权重 -30%")
  await page.mouse.move(10, 10)
  const specialBaitCard = page.locator(".labeled-frame-card").filter({ hasText: "星涟秘饵" }).locator(".item-card").first()
  await expect(specialBaitCard).toBeVisible()
  await specialBaitCard.hover()
  const specialBaitTooltip = page.locator(".item-popover-content").filter({ hasText: "星涟秘饵" }).last()
  await expect(specialBaitTooltip).toContainText("显著提高遇见特殊鱼类的概率")
  await expect(specialBaitTooltip).toContainText(/特殊鱼类机缘\s*显著提高/)
  await expect(specialBaitTooltip).not.toContainText(/棱彩|彩虹|闪光/)
  await expect(page.locator('.item-card img[alt="莲纹锦鲤"]')).toHaveCount(0)
  await expectNoHorizontalOverflow(page)
  await page.screenshot({ path: "../output/playwright/warehouse-starripple-bait.png", fullPage: true })
})

test("market and dictionary group fishing gear by tackle type", async ({ page }) => {
  await page.setViewportSize({ width: 1440, height: 900 })
  await mockFishing(page)

  await page.goto("/game/market")
  await dismissBuildNotice(page)
  const marketBrowser = page.locator('[data-tutorial="market-catalog"] [data-tutorial="item-browser"]')
  await expect(marketBrowser).toBeVisible()
  await marketBrowser.getByRole("tab", { name: "渔具", exact: true }).click()
  const marketSections = marketBrowser.locator(".item-browser-section-title")
  for (const label of ["鱼竿", "卷线器", "主线", "前导线", "鱼钩", "浮漂", "铅坠", "拟饵", "鱼饵"]) {
    await expect(marketSections.getByText(label, { exact: true })).toBeVisible()
  }
  await expect(marketBrowser.getByText("蚯蚓鱼饵", { exact: true }).first()).toBeVisible()
  await expectNoHorizontalOverflow(page)
  await page.screenshot({ path: "../output/playwright/market-fishing-gear-secondary.png", fullPage: true })

  await page.goto("/game/panel/dictionary")
  await dismissBuildNotice(page)
  const dictionaryBrowser = page.locator('[data-tutorial="item-browser"]').filter({ visible: true })
  await expect(dictionaryBrowser).toBeVisible()
  await dictionaryBrowser.getByRole("tab", { name: "渔具", exact: true }).click()
  const dictionarySections = dictionaryBrowser.locator(".item-browser-section-title")
  for (const label of ["鱼竿", "卷线器", "主线", "前导线", "鱼钩", "浮漂", "铅坠", "拟饵", "鱼饵"]) {
    await expect(dictionarySections.getByText(label, { exact: true })).toBeVisible()
  }
  await expectNoHorizontalOverflow(page)
  await page.screenshot({ path: "../output/playwright/dictionary-fishing-gear-secondary.png", fullPage: true })
})

test("standard event shows the exact designated tackle and readiness", async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 800 })
  await mockFishing(page)
  await page.goto("/game/fishing")
  await dismissBuildNotice(page)
  await page.getByRole("tab", { name: "赛事" }).click()
  await expect(page.getByRole("heading", { name: "褐鳟" })).toBeVisible()
  await page.getByText("标准组", { exact: true }).click()
  const standardRules = page.locator(".fishing-standard-rules")
  await expect(standardRules.getByText("本周指定钓组", { exact: true })).toBeVisible()
  await expect(standardRules.getByText("指定物品 · 实际装备", { exact: true })).toBeVisible()
  await expect(standardRules.getByText("指定钓法：浮漂钓", { exact: true })).toBeVisible()
  for (const label of ["鱼竿", "卷线器", "主线", "前导线", "鱼钩", "浮漂", "鱼饵"]) {
    await expect(standardRules.getByText(label, { exact: true })).toBeVisible()
  }
  for (const item of ["基础鱼竿", "基础纺车轮", "编织主线", "透明前导线", "细锋鱼钩", "芦苇浮漂", "蚯蚓鱼饵"]) {
    await expect(standardRules.getByText(item, { exact: true })).toBeVisible()
  }
  await expect(standardRules.locator(".fishing-standard-tackle > div")).toHaveCount(7)
  await expect(standardRules.getByText("已拥有", { exact: true })).toHaveCount(7)
  await expect(standardRules.getByText("已备妥：晴湾轻钓组", { exact: true })).toBeVisible()
  await expect(standardRules.getByText("出发前选择标准组，并实际使用以上全部指定物品。本次钓行只进入所选组别。")).toBeVisible()
  await expect(page.getByText("我的参赛鱼", { exact: true })).toBeVisible()
  await expect(page.locator(".fishing-my-entries").getByText(/第 36 名/)).toBeVisible()
  const tournamentHero = await page.locator(".fishing-tournament-hero").boundingBox()
  expect(tournamentHero).not.toBeNull()
  expect(tournamentHero!.height).toBeLessThanOrEqual(125)
  const tournamentEmpty = await page.locator(".fishing-tournament-board .fishing-empty").boundingBox()
  expect(tournamentEmpty).not.toBeNull()
  expect(tournamentEmpty!.height).toBeLessThanOrEqual(210)
  await page.waitForTimeout(350)
  await expectNoHorizontalOverflow(page)
  await page.screenshot({ path: "../output/playwright/fishing-desktop-tournament-standard.png", fullPage: true })

  await page.setViewportSize({ width: 390, height: 844 })
  await page.waitForTimeout(200)
  await expect(standardRules).toBeVisible()
  await expectNoHorizontalOverflow(page)
  const mobileRules = await standardRules.boundingBox()
  expect(mobileRules).not.toBeNull()
  expect(mobileRules!.width).toBeLessThanOrEqual(390)
  await page.screenshot({ path: "../output/playwright/fishing-mobile-tournament-standard.png", fullPage: true })
})

test("existing fishing-v2 event keeps the legacy shared-standard presentation", async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 800 })
  await mockFishing(page, false, undefined, [], undefined, undefined, [], { legacyTournament: true })
  await page.goto("/game/fishing")
  await dismissBuildNotice(page)
  await page.getByRole("tab", { name: "赛事" }).click()
  await page.getByText("标准组", { exact: true }).click()

  const standardRules = page.locator(".fishing-standard-rules")
  await expect(standardRules.getByText("本周统一标准", { exact: true })).toBeVisible()
  await expect(standardRules.getByText("自动判定 · 全员相同", { exact: true })).toBeVisible()
  await expect(standardRules.getByText("赛事钓手等级", { exact: true })).toBeVisible()
  await expect(standardRules.getByText("无需选择。目标鱼会自动更新公开组成绩；系统再抛开实际钓具按以上标准重判，成功后也会更新标准组，体长和重量不变。")).toBeVisible()
  await expect(page.getByText("钓到目标鱼后，最好的一尾会自动参赛。", { exact: true })).toBeVisible()
  await expect(standardRules.locator(".fishing-standard-tackle")).toHaveCount(0)

  await page.getByRole("tab", { name: "水域" }).click()
  await page.getByRole("button", { name: /准备出发/ }).click()
  await page.locator(".fishing-planner-step button").filter({ hasText: "睡莲边" }).click()
  await page.getByRole("button", { name: /下一步/ }).click()
  await expect(page.locator(".fishing-tournament-choice")).toHaveCount(0)
})

test("matching tackle can select the standard division for one trip", async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 800 })
  let submittedTrip: any
  await mockFishing(page, false, (body) => { submittedTrip = body })
  await page.goto("/game/fishing")
  await dismissBuildNotice(page)

  await page.getByRole("button", { name: /准备出发/ }).click()
  await page.locator(".fishing-planner-step button").filter({ hasText: "睡莲边" }).click()
  await page.getByRole("button", { name: /下一步/ }).click()
  const matchingLoadout = page.locator(".fishing-planner-step > button").filter({ hasText: "晴湾轻钓组" })
  await expect(matchingLoadout.getByText("符合标准组", { exact: true })).toBeVisible()
  await matchingLoadout.click()

  const divisionChoice = page.getByRole("radiogroup", { name: "赛事参赛组" })
  await expect(divisionChoice.getByRole("button", { name: /标准组/ })).toBeEnabled()
  await divisionChoice.getByRole("button", { name: /标准组/ }).click()
  await expect(page.getByText("当前钓组符合标准，可以参加标准组。", { exact: true })).toBeVisible()
  await expectNoHorizontalOverflow(page)
  await page.screenshot({ path: "../output/playwright/fishing-desktop-standard-division-choice.png", fullPage: true })
  await page.getByRole("button", { name: /下一步/ }).click()
  await page.getByRole("button", { name: /开始钓鱼/ }).click()

  await expect.poll(() => submittedTrip).toBeTruthy()
  expect(submittedTrip.steps[0].args).toMatchObject({
    loadoutId: "loadout-1",
    tournamentDivision: "standard"
  })
})

test("standard division stays unavailable when the selected tackle differs", async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 800 })
  await mockFishing(page, false, undefined, [], undefined, undefined, [], {
    tournamentStandardItems: [
      { slot: "rod", itemId: "silverleaf_float_rod" },
      { slot: "reel", itemId: "basic_spinning_reel" },
      { slot: "main_line", itemId: "braided_main_line" },
      { slot: "leader", itemId: "clear_leader" },
      { slot: "hook", itemId: "fine_fishing_hook" },
      { slot: "float_or_sinker", itemId: "reed_float" },
      { slot: "bait", itemId: "earthworm_bait" }
    ]
  })
  await page.goto("/game/fishing")
  await dismissBuildNotice(page)

  await page.getByRole("button", { name: /准备出发/ }).click()
  await page.locator(".fishing-planner-step button").filter({ hasText: "睡莲边" }).click()
  await page.getByRole("button", { name: /下一步/ }).click()
  await page.locator(".fishing-planner-step > button").filter({ hasText: "晴湾轻钓组" }).click()

  const divisionChoice = page.getByRole("radiogroup", { name: "赛事参赛组" })
  await expect(divisionChoice.getByRole("button", { name: /标准组/ })).toBeDisabled()
  await expect(page.getByText("当前钓组与指定物品不一致，只能参加公开组。", { exact: true })).toBeVisible()
  await expect(divisionChoice.getByRole("button", { name: /公开组/ })).toHaveAttribute("aria-pressed", "true")
})
