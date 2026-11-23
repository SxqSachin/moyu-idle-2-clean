import rawSharedGlobalConfig from "@shared-config/shared-global-config.json"
import { getGameDataChunk, isGameDataLoaded } from "@services/game-data-runtime"

export type MarketPriceStep = {
  min: number
  max: number
  step: number
}

type RawMarketPriceStep = {
  min?: unknown
  max?: unknown
  step?: unknown
}

type RawKittySettings = {
  actionDurationMultiplier?: unknown
  statusExpMultiplier?: unknown
  battleAttrExpMultiplier?: unknown
  battleSkillExpMultiplier?: unknown
  battleLevelCapDefault?: unknown
  battleLevelCapPerKittyLevel?: unknown
  maxPersonalities?: unknown
  battleEquipSlots?: unknown
  removePersonalityItemId?: unknown
  removePersonalityCount?: unknown
  defaultPersonalityId?: unknown
}

export type KittyPersonalityEffect = {
  kind: string
  attrs: Record<string, number>
  position: string
  battleHooks?: Record<string, unknown>[]
  effectMutations?: Record<string, unknown>[]
}

export type KittyPersonalityProfile = {
  id: string
  nameKey: string
  descKey: string
  weight: number
  effects: KittyPersonalityEffect[]
}

type RawAuctionSettings = {
  feeRateBps?: unknown
  minBidIncrement?: unknown
  antiSnipeTriggerSeconds?: unknown
  antiSnipeExtendSeconds?: unknown
  minRepLevel?: unknown
}

type RawItemRenameSettings = {
  cost?: unknown
  maxNameRunes?: unknown
  labelColors?: unknown
}

type RawPlayerUsernameRenameSettings = {
  costItemId?: unknown
  costItemCount?: unknown
}

type RawPresenceSettings = {
  visibilityOfflineDelaySeconds?: unknown
}

type RawActionSettings = {
  enhanceInheritanceRatio?: unknown
  queueLaneLimit?: unknown
  kittyQueueLaneLimit?: unknown
}

type RawAlchemySettings = {
  infusionBaseGoldCost?: unknown
  infusionBaseDurationSec?: unknown
  infusionBaseExp?: unknown
  durationsSec?: unknown
  transmuteDurationByRarityIndex?: unknown
  levelDeficitFailChancePerLevel?: unknown
  levelDeficitFailChanceCap?: unknown
  convertGroupEconomyByRarity?: unknown
  inherit?: unknown
}

export type AlchemyDurationsConfig = {
  default: number
  convert: number
  bench: number
  transmute: number
  smelt: number
}

export type AlchemyConvertGroupEconomy = {
  gold: number
  duration: number
  exp: number
  upgrade: number
}

export type AlchemyEconomyConfig = {
  infusionBaseGoldCost: number
  infusionBaseDurationSec: number
  infusionBaseExp: number
  durationsSec: AlchemyDurationsConfig
  transmuteDurationByRarityIndex: number[]
  levelDeficitFailChancePerLevel: number
  levelDeficitFailChanceCap: number
  convertGroupEconomyByRarity: Record<string, AlchemyConvertGroupEconomy>
  inherit: AlchemyInheritGlobalConfig
}

export type AlchemyInheritMultiplierRangeConfig = {
  min: number
  max: number
}

export type AlchemyInheritGlobalConfig = {
  baseInheritSlots: number
  baseSuppressSlots: number
  inheritSlotPrecisionScalePerPoint: number
  suppressSlotStabilityBonusPerPoint: number
  positiveIgnoreKeepChanceBase: number
  negativeIgnoreKeepChanceBase: number
  hybridMergeDivisor: number
  inbreedMutationChance: number
  inbreedGoodChance: number
  inbreedGoodMultiplierRange: AlchemyInheritMultiplierRangeConfig
  inbreedBadMultiplierRange: AlchemyInheritMultiplierRangeConfig
  mutationChance: number
  numericValueCapMultiplier: number
}

type RawAttributeLayoutSceneConfig = {
  battlePrimary?: unknown
  battle?: unknown
  work?: unknown
  previewLimit?: unknown
}

type RawAttributeLayoutConfig = {
  scenes?: {
    character_stats?: RawAttributeLayoutSceneConfig
    battle_unit_card?: RawAttributeLayoutSceneConfig
    enemy_quick_ref?: RawAttributeLayoutSceneConfig
    battle_unit_card_popover?: RawAttributeLayoutSceneConfig
  }
}

export type AttributeLayoutDomain = "battlePrimary" | "battle" | "work"
export type AttributeLayoutSceneId = "character_stats" | "battle_unit_card" | "enemy_quick_ref" | "battle_unit_card_popover"

export type AttributeLayoutSceneConfig = {
  battlePrimary: string[][]
  battle: string[][]
  work: string[][]
  previewLimit: number | null
}

export type AttributeLayoutConfig = {
  scenes: Record<AttributeLayoutSceneId, AttributeLayoutSceneConfig>
}

export type CombatScoringConfig = {
  constants: {
    goldPerBudget: number
    levelBudgetBase: number
    levelBudgetGrowth: number
    levelBudgetPower: number
    affixSlotBaseBudget: number
    extraSkillTargetValue: number
    extraBuffTargetValue: number
    extraTurnValue: number
    cooldownPenaltyPerTurn: number
    mpCostPenalty: number
    targetPositionValue: number
    rearTargetValue: number
    seekStrategyValue: number
    selectRuleValue: number
    fallbackSelfValue: number
    battleHookValue: number
    effectMutationValue: number
    enhanceAttrScoreMultiplier: number
    affixInstanceScoreMultiplier: number
    enemySkillScoreMultiplier: number
    enemyGoldRewardRate: number
    enemyExpRewardRate: number
  }
  attributeUnitValues: Record<string, number>
  rarityMultipliers: Record<string, number>
  slotMultipliers: Record<string, number>
  enemyTierMultipliers: Record<string, number>
  enemyRewardMultipliers: Record<string, number>
}

const FALLBACK_MARKET_PRICE_STEP_CONFIG: MarketPriceStep[] = [
  { min: 0, max: 100, step: 1 },
  { min: 100, max: 1000, step: 10 },
  { min: 1000, max: 10000, step: 50 },
  { min: 10000, max: 100000, step: 100 },
  { min: 100000, max: 1000000, step: 1000 },
  { min: 1000000, max: 10000000, step: 10000 },
  { min: 10000000, max: Number.POSITIVE_INFINITY, step: 100000 }
]

const FALLBACK_KITTY_STATUS_EXP_MULTIPLIER = 0
const FALLBACK_KITTY_BATTLE_ATTR_EXP_MULTIPLIER = 1
const FALLBACK_KITTY_BATTLE_SKILL_EXP_MULTIPLIER = 1
const FALLBACK_KITTY_BATTLE_LEVEL_CAP_DEFAULT = 100
const FALLBACK_KITTY_BATTLE_LEVEL_CAP_PER_KITTY_LEVEL = 25
const FALLBACK_KITTY_MAX_PERSONALITIES = 4
const FALLBACK_KITTY_BATTLE_EQUIP_SLOTS = [
  "cloak",
  "head",
  "left_hand",
  "body",
  "right_hand",
  "arm",
  "legs",
  "feet",
  "accessory_1",
  "accessory_2",
  "accessory_3",
  "accessory_4"
]
const FALLBACK_KITTY_DEFAULT_PERSONALITY_ID = "balanced"
const FALLBACK_KITTY_REMOVE_PERSONALITY_ITEM_ID = "gold_coin"
const FALLBACK_KITTY_REMOVE_PERSONALITY_COUNT = 1
const FALLBACK_KITTY_PERSONALITIES: KittyPersonalityProfile[] = [
  {
    id: "balanced",
    nameKey: "kitty.personality.balanced.name",
    descKey: "kitty.personality.balanced.desc",
    weight: 1,
    effects: []
  }
]
const FALLBACK_VISIBILITY_OFFLINE_DELAY_SECONDS = 120
const FALLBACK_ENHANCE_INHERITANCE_RATIO = 0.5
const FALLBACK_ACTION_QUEUE_LANE_LIMIT = 30
const FALLBACK_KITTY_ACTION_QUEUE_LANE_LIMIT = 20
const FALLBACK_AUCTION_FEE_RATE_BPS = 500
const FALLBACK_AUCTION_ANTI_SNIPE_TRIGGER_SECONDS = 30 * 60
const FALLBACK_AUCTION_ANTI_SNIPE_EXTEND_SECONDS = 10 * 60
const FALLBACK_AUCTION_MIN_REP_LEVEL = 10
const FALLBACK_ITEM_RENAME_COST: Record<string, number> = { harmonic_embryo: 1 }
const FALLBACK_ITEM_RENAME_MAX_NAME_RUNES = 16
const FALLBACK_ITEM_RENAME_LABEL_COLORS = ["gold", "rose", "violet", "amber", "sky", "emerald", "fuchsia"]
const FALLBACK_PLAYER_USERNAME_RENAME_COST_ITEM_ID = "harmonic_embryo"
const FALLBACK_PLAYER_USERNAME_RENAME_COST_ITEM_COUNT = 1
const FALLBACK_BATTLE_ITEM_BAG_MAX_KINDS = 4
const FALLBACK_BATTLE_ITEM_BAG_MAX_TOTAL_QUANTITY = 1000
const FALLBACK_INTERVAL_BETWEEN_ITEM_AND_ACTION = 100
const FALLBACK_INTERVAL_BETWEEN_MULTI_HIT_DAMAGE = 80
const FALLBACK_BATTLE_CHAIN_FLOAT_JITTER_SCALE = 1
const FALLBACK_ENEMY_HP_REGEN_RATIO = 0.34
const FALLBACK_PERCENT_PENETRATION_CAP = 0.8
const FALLBACK_ALCHEMY_DURATIONS_SEC: AlchemyDurationsConfig = {
  default: 16,
  convert: 14,
  bench: 16,
  transmute: 8,
  smelt: 24
}
const FALLBACK_ALCHEMY_TRANSMUTE_DURATION_BY_RARITY_INDEX = [8, 9, 10, 11, 12, 12]
const FALLBACK_ALCHEMY_LEVEL_DEFICIT_FAIL_CHANCE_PER_LEVEL = 0.06
const FALLBACK_ALCHEMY_LEVEL_DEFICIT_FAIL_CHANCE_CAP = 0.85
const FALLBACK_ALCHEMY_CONVERT_GROUP_ECONOMY_BY_RARITY: Record<string, AlchemyConvertGroupEconomy> = {
  common: { gold: 45, duration: 14, exp: 2, upgrade: 0.02 },
  uncommon: { gold: 105, duration: 27, exp: 3.5, upgrade: 0.018 },
  rare: { gold: 245, duration: 39, exp: 5, upgrade: 0.016 },
  epic: { gold: 550, duration: 52, exp: 6.5, upgrade: 0.014 },
  legendary: { gold: 1200, duration: 64, exp: 8, upgrade: 0 }
}
const FALLBACK_ALCHEMY_INHERIT_CONFIG: AlchemyInheritGlobalConfig = {
  baseInheritSlots: 3,
  baseSuppressSlots: 0,
  inheritSlotPrecisionScalePerPoint: 1,
  suppressSlotStabilityBonusPerPoint: 1,
  positiveIgnoreKeepChanceBase: 0.4,
  negativeIgnoreKeepChanceBase: 1,
  hybridMergeDivisor: 1.9,
  inbreedMutationChance: 0.8,
  inbreedGoodChance: 0.05,
  inbreedGoodMultiplierRange: { min: 1.1, max: 1.35 },
  inbreedBadMultiplierRange: { min: 0.35, max: 0.8 },
  mutationChance: 0.05,
  numericValueCapMultiplier: 3
}
const FALLBACK_ATTRIBUTE_LAYOUT_CONFIG: AttributeLayoutConfig = {
  scenes: {
    character_stats: {
      battlePrimary: [["strength", "dexterity", "intelligence", "perception"]],
      battle: [[
        "stamina",
        "luck",
        "hp",
        "hp_regen",
        "mp",
        "mp_regen",
        "speed",
        "physics_damage",
        "magical_damage",
        "physics_defence",
        "magical_defence",
        "physics_damage_amplification",
        "magical_damage_amplification",
        "physics_damage_resist",
        "magical_damage_resist",
        "physics_flat_defense_penetration",
        "magicial_flat_defense_penetration",
        "physics_percent_defense_penetration",
        "magicial_percent_defense_penetration",
        "crit_chance",
        "magic_crit_chance",
        "crit_damage",
        "magic_crit_damage",
        "crit_damage_resist"
      ]],
      work: [[
        "mining",
        "collecting",
        "farming_animal",
        "cooking",
        "cooking_speed",
        "crafting",
        "sewing",
        "forging",
        "enhance",
        "enhancement_success_rate",
        "collecting_speed",
        "mining_speed",
        "farming_animal_speed",
        "crafting_speed",
        "sewing_speed",
        "forging_speed"
      ]],
      previewLimit: null
    },
    battle_unit_card: {
      battlePrimary: [["strength", "dexterity", "intelligence", "perception"]],
      battle: [[
        "stamina",
        "hp",
        "mp",
        "physics_damage",
        "magical_damage",
        "physics_defence",
        "magical_defence",
        "speed",
        "crit_chance",
        "magic_crit_chance",
        "crit_damage",
        "magic_crit_damage",
        "crit_damage_resist"
      ]],
      work: [[]],
      previewLimit: 8
    },
    enemy_quick_ref: {
      battlePrimary: [["strength", "dexterity", "intelligence", "perception"]],
      battle: [[
        "hp",
        "mp",
        "stamina",
        "physics_damage",
        "magical_damage",
        "physics_defence",
        "magical_defence",
        "speed",
        "crit_chance",
        "magic_crit_chance",
        "crit_damage",
        "magic_crit_damage",
        "crit_damage_resist"
      ]],
      work: [[]],
      previewLimit: 14
    },
    battle_unit_card_popover: {
      battlePrimary: [],
      "battle": [
          [
            "hp",
            "mp",
            "speed",
            "physics_damage",
            "physics_defence",
            "physics_damage_amplification",
            "physics_damage_resist",
            "physics_flat_defense_penetration",
            "physics_percent_defense_penetration",
            "crit_chance",
            "crit_damage"
          ],
          [
            "hp_regen",
            "mp_regen",
            "",
            "magical_damage",
            "magical_defence",
            "magical_damage_amplification",
            "magical_damage_resist",
            "magicial_flat_defense_penetration",
            "magicial_percent_defense_penetration",
            "magic_crit_chance",
            "magic_crit_damage",
            "crit_damage_resist"
          ]
        ],
        "work": [],
        previewLimit: 10
    }
  }
}
const FALLBACK_COMBAT_SCORING_CONFIG: CombatScoringConfig = {
  constants: {
    goldPerBudget: 12,
    levelBudgetBase: 8,
    levelBudgetGrowth: 2.8,
    levelBudgetPower: 1.18,
    affixSlotBaseBudget: 18,
    extraSkillTargetValue: 0.62,
    extraBuffTargetValue: 0.55,
    extraTurnValue: 0.45,
    cooldownPenaltyPerTurn: 0.08,
    mpCostPenalty: 0.55,
    targetPositionValue: 4,
    rearTargetValue: 12,
    seekStrategyValue: 6,
    selectRuleValue: 5,
    fallbackSelfValue: 3,
    battleHookValue: 18,
    effectMutationValue: 18,
    enhanceAttrScoreMultiplier: 0.72,
    affixInstanceScoreMultiplier: 1,
    enemySkillScoreMultiplier: 0.45,
    enemyGoldRewardRate: 0.18,
    enemyExpRewardRate: 0.018
  },
  attributeUnitValues: {},
  rarityMultipliers: {},
  slotMultipliers: {},
  enemyTierMultipliers: { normal: 1, elite: 2.3, boss: 6.5 },
  enemyRewardMultipliers: { normal: 1, elite: 1.65, boss: 3.5 }
}

function parseNumber(raw: unknown, fallback: number): number {
  if (typeof raw === "number" && Number.isFinite(raw)) return raw
  if (typeof raw === "string" && raw.trim()) {
    const parsed = Number(raw)
    if (Number.isFinite(parsed)) return parsed
  }
  return fallback
}

function normalizeStep(raw: RawMarketPriceStep): MarketPriceStep | null {
  const min = Math.max(0, Math.floor(parseNumber(raw.min, Number.NaN)))
  const step = Math.max(1, Math.floor(parseNumber(raw.step, Number.NaN)))
  const rawMax = raw.max
  const max =
    rawMax === null || rawMax === undefined
      ? Number.POSITIVE_INFINITY
      : Math.floor(parseNumber(rawMax, Number.NaN))
  if (!Number.isFinite(min) || !Number.isFinite(step)) return null
  if (!(Number.isFinite(max) || max === Number.POSITIVE_INFINITY)) return null
  if (max !== Number.POSITIVE_INFINITY && max <= min) return null
  return { min, max, step }
}

function normalizeMarketPriceSteps(raw: unknown): MarketPriceStep[] {
  if (!Array.isArray(raw)) return FALLBACK_MARKET_PRICE_STEP_CONFIG
  const parsed = raw
    .map((row) => {
      if (!row || typeof row !== "object") return null
      return normalizeStep(row as RawMarketPriceStep)
    })
    .filter((row): row is MarketPriceStep => !!row)
    .sort((a, b) => a.min - b.min)
  if (parsed.length === 0) return FALLBACK_MARKET_PRICE_STEP_CONFIG
  return parsed
}

export const MARKET_PRICE_STEP_CONFIG = normalizeMarketPriceSteps(
  (rawSharedGlobalConfig as { market?: { priceSteps?: unknown } })?.market?.priceSteps
)

function normalizeKittyStatusExpMultiplier(raw: unknown): number {
  const value = parseNumber(raw, FALLBACK_KITTY_STATUS_EXP_MULTIPLIER)
  if (!Number.isFinite(value) || value < 0) return FALLBACK_KITTY_STATUS_EXP_MULTIPLIER
  return value
}

function normalizeRatio(raw: unknown, fallback: number): number {
  const value = parseNumber(raw, fallback)
  if (!Number.isFinite(value) || value < 0 || value > 1) return fallback
  return value
}

function normalizeStringList(raw: unknown): string[] {
  if (!Array.isArray(raw)) return []
  const out: string[] = []
  const seen = new Set<string>()
  for (const entry of raw) {
    const value = String(entry || "").trim().toLowerCase()
    if (!value || seen.has(value)) continue
    seen.add(value)
    out.push(value)
  }
  return out
}

function normalizeKittyBattleEquipSlots(raw: unknown): string[] {
  const parsed = normalizeStringList(raw)
  return parsed.length > 0 ? parsed : [...FALLBACK_KITTY_BATTLE_EQUIP_SLOTS]
}

function normalizeKittyPersonalityEffects(raw: unknown): KittyPersonalityEffect[] {
  if (!Array.isArray(raw)) return []
  const out: KittyPersonalityEffect[] = []
  for (const row of raw) {
    if (!row || typeof row !== "object" || Array.isArray(row)) continue
    const source = row as Record<string, unknown>
    const attrsRaw = source.attrs && typeof source.attrs === "object" && !Array.isArray(source.attrs)
      ? source.attrs as Record<string, unknown>
      : {}
    const attrs: Record<string, number> = {}
    for (const [rawAttrKey, rawValue] of Object.entries(attrsRaw)) {
      const attrKey = String(rawAttrKey || "").trim().toLowerCase()
      const value = parseNumber(rawValue, 0)
      if (!attrKey || !Number.isFinite(value) || value === 0) continue
      attrs[attrKey] = value
    }
    const kind = String(source.kind || "").trim().toLowerCase()
    const normalized: KittyPersonalityEffect = {
      kind,
      attrs,
      position: String(source.position || "").trim().toLowerCase()
    }
    if ((kind === "battle_hook" || kind === "effect_mutation") && Array.isArray(source.battleHooks)) {
      normalized.battleHooks = source.battleHooks.filter((row): row is Record<string, unknown> => !!row && typeof row === "object" && !Array.isArray(row))
    }
    if ((kind === "battle_hook" || kind === "effect_mutation") && Array.isArray(source.effectMutations)) {
      normalized.effectMutations = source.effectMutations.filter((row): row is Record<string, unknown> => !!row && typeof row === "object" && !Array.isArray(row))
    }
    out.push(normalized)
  }
  return out
}

function normalizeKittyPersonalities(raw: unknown): KittyPersonalityProfile[] {
  if (!Array.isArray(raw)) return [...FALLBACK_KITTY_PERSONALITIES]
  const out: KittyPersonalityProfile[] = []
  const seen = new Set<string>()
  for (const row of raw) {
    if (!row || typeof row !== "object" || Array.isArray(row)) continue
    const source = row as Record<string, unknown>
    const id = String(source.id || "").trim().toLowerCase()
    if (!id || seen.has(id)) continue
    seen.add(id)
    out.push({
      id,
      nameKey: String(source.nameKey || "").trim(),
      descKey: String(source.descKey || "").trim(),
      weight: Math.max(0, parseNumber(source.weight, 1)),
      effects: normalizeKittyPersonalityEffects(source.effects)
    })
  }
  return out.length > 0 ? out : [...FALLBACK_KITTY_PERSONALITIES]
}

const rawKittySettings = (rawSharedGlobalConfig as { kitty?: RawKittySettings })?.kitty
const rawAuctionSettings = (rawSharedGlobalConfig as { auction?: RawAuctionSettings })?.auction
const rawItemRenameSettings = (rawSharedGlobalConfig as { itemRename?: RawItemRenameSettings })?.itemRename
const rawPlayerUsernameRenameSettings = (rawSharedGlobalConfig as { playerUsernameRename?: RawPlayerUsernameRenameSettings })?.playerUsernameRename
const rawPresenceSettings = (rawSharedGlobalConfig as { presence?: RawPresenceSettings })?.presence
const rawActionSettings = (rawSharedGlobalConfig as { action?: RawActionSettings })?.action
const rawAlchemySettings = (rawSharedGlobalConfig as { alchemy?: RawAlchemySettings })?.alchemy
const rawCombatScoring = (rawSharedGlobalConfig as { combatScoring?: unknown })?.combatScoring
const rawBattleItemBagMaxKinds = (rawSharedGlobalConfig as { battleItemBagMaxKinds?: unknown })?.battleItemBagMaxKinds
const rawBattleItemBagMaxTotalQuantity = (rawSharedGlobalConfig as { battleItemBagMaxTotalQuantity?: unknown })?.battleItemBagMaxTotalQuantity
const rawIntervalBetweenItemAndAction = (rawSharedGlobalConfig as { intervalBetweenItemAndAction?: unknown })?.intervalBetweenItemAndAction
const rawIntervalBetweenMultiHitDamage = (rawSharedGlobalConfig as { intervalBetweenMultiHitDamage?: unknown })?.intervalBetweenMultiHitDamage
const rawBattleChainFloatJitterScale = (rawSharedGlobalConfig as { battleChainFloatJitterScale?: unknown })?.battleChainFloatJitterScale
const rawCombatBalanceModel = (rawSharedGlobalConfig as { combatBalanceModel?: { enemyHpRegenRatio?: unknown; percentPenetrationCap?: unknown; defenseK?: unknown } })?.combatBalanceModel
const rawAttributeLayout = (rawSharedGlobalConfig as { attributeLayout?: RawAttributeLayoutConfig })?.attributeLayout
let kittyPersonalitiesCache: KittyPersonalityProfile[] | null = null

function getKittyPersonalities(): KittyPersonalityProfile[] {
  if (!isGameDataLoaded.value) return FALLBACK_KITTY_PERSONALITIES
  if (!kittyPersonalitiesCache) {
    const rawKittyPersonalityTable = getGameDataChunk("kitty_personalities")
    const rawKittyPersonalities = (rawKittyPersonalityTable as { kitty_personalities?: unknown })?.kitty_personalities
    kittyPersonalitiesCache = normalizeKittyPersonalities(rawKittyPersonalities)
  }
  return kittyPersonalitiesCache
}

export const KITTY_STATUS_EXP_MULTIPLIER = normalizeKittyStatusExpMultiplier(rawKittySettings?.statusExpMultiplier)
export const KITTY_BATTLE_ATTR_EXP_MULTIPLIER = normalizeRatio(rawKittySettings?.battleAttrExpMultiplier, FALLBACK_KITTY_BATTLE_ATTR_EXP_MULTIPLIER)
export const KITTY_BATTLE_SKILL_EXP_MULTIPLIER = normalizeRatio(rawKittySettings?.battleSkillExpMultiplier, FALLBACK_KITTY_BATTLE_SKILL_EXP_MULTIPLIER)
export const KITTY_BATTLE_LEVEL_CAP_DEFAULT = Math.max(1, Math.floor(parseNumber(rawKittySettings?.battleLevelCapDefault, FALLBACK_KITTY_BATTLE_LEVEL_CAP_DEFAULT)))
export const KITTY_BATTLE_LEVEL_CAP_PER_KITTY_LEVEL = Math.max(0, Math.floor(parseNumber(rawKittySettings?.battleLevelCapPerKittyLevel, FALLBACK_KITTY_BATTLE_LEVEL_CAP_PER_KITTY_LEVEL)))
export const KITTY_MAX_PERSONALITIES = Math.max(1, Math.floor(parseNumber(rawKittySettings?.maxPersonalities, FALLBACK_KITTY_MAX_PERSONALITIES)))
export const KITTY_BATTLE_EQUIP_SLOTS = normalizeKittyBattleEquipSlots(rawKittySettings?.battleEquipSlots)
export const KITTY_PERSONALITIES = new Proxy(FALLBACK_KITTY_PERSONALITIES as KittyPersonalityProfile[], {
  get(_target, prop, receiver) {
    return Reflect.get(getKittyPersonalities(), prop, receiver)
  },
  has(_target, prop) {
    return prop in getKittyPersonalities()
  },
  ownKeys() {
    return Reflect.ownKeys(getKittyPersonalities())
  },
  getOwnPropertyDescriptor(_target, prop) {
    return Reflect.getOwnPropertyDescriptor(getKittyPersonalities(), prop)
  }
})
const configuredKittyDefaultPersonalityID = String(rawKittySettings?.defaultPersonalityId || FALLBACK_KITTY_DEFAULT_PERSONALITY_ID).trim().toLowerCase()
export const KITTY_DEFAULT_PERSONALITY_ID = FALLBACK_KITTY_PERSONALITIES.some((row) => row.id === configuredKittyDefaultPersonalityID)
  ? configuredKittyDefaultPersonalityID
  : (FALLBACK_KITTY_PERSONALITIES[0]?.id || FALLBACK_KITTY_DEFAULT_PERSONALITY_ID)
export const KITTY_REMOVE_PERSONALITY_ITEM_ID = String(rawKittySettings?.removePersonalityItemId || FALLBACK_KITTY_REMOVE_PERSONALITY_ITEM_ID).trim() || FALLBACK_KITTY_REMOVE_PERSONALITY_ITEM_ID
export const ALCHEMY_CONFIG: AlchemyEconomyConfig = {
  infusionBaseGoldCost: Math.max(0, Math.floor(parseNumber(rawAlchemySettings?.infusionBaseGoldCost, 32))),
  infusionBaseDurationSec: Math.max(1, parseNumber(rawAlchemySettings?.infusionBaseDurationSec, 16)),
  infusionBaseExp: Math.max(0, parseNumber(rawAlchemySettings?.infusionBaseExp, 2)),
  durationsSec: normalizeAlchemyDurations(rawAlchemySettings?.durationsSec),
  transmuteDurationByRarityIndex: normalizeAlchemyDurationList(rawAlchemySettings?.transmuteDurationByRarityIndex),
  levelDeficitFailChancePerLevel: normalizeRatio(rawAlchemySettings?.levelDeficitFailChancePerLevel, FALLBACK_ALCHEMY_LEVEL_DEFICIT_FAIL_CHANCE_PER_LEVEL),
  levelDeficitFailChanceCap: normalizeRatio(rawAlchemySettings?.levelDeficitFailChanceCap, FALLBACK_ALCHEMY_LEVEL_DEFICIT_FAIL_CHANCE_CAP),
  convertGroupEconomyByRarity: normalizeAlchemyConvertGroupEconomy(rawAlchemySettings?.convertGroupEconomyByRarity),
  inherit: normalizeAlchemyInheritGlobalConfig(rawAlchemySettings?.inherit)
}
export const KITTY_REMOVE_PERSONALITY_COUNT = Math.max(1, Math.floor(parseNumber(rawKittySettings?.removePersonalityCount, FALLBACK_KITTY_REMOVE_PERSONALITY_COUNT)))

export const AUCTION_FEE_RATE_BPS = Math.max(
  0,
  Math.min(10_000, Math.floor(parseNumber(rawAuctionSettings?.feeRateBps, FALLBACK_AUCTION_FEE_RATE_BPS)))
)
export const AUCTION_MIN_BID_INCREMENT = Math.max(
  1,
  Math.floor(parseNumber(rawAuctionSettings?.minBidIncrement, 1))
)
export const AUCTION_ANTI_SNIPE_TRIGGER_SECONDS = Math.max(
  0,
  Math.floor(parseNumber(rawAuctionSettings?.antiSnipeTriggerSeconds, FALLBACK_AUCTION_ANTI_SNIPE_TRIGGER_SECONDS))
)
export const AUCTION_ANTI_SNIPE_EXTEND_SECONDS = Math.max(
  0,
  Math.floor(parseNumber(rawAuctionSettings?.antiSnipeExtendSeconds, FALLBACK_AUCTION_ANTI_SNIPE_EXTEND_SECONDS))
)
export const AUCTION_MIN_REP_LEVEL = Math.max(
  0,
  Math.floor(parseNumber(rawAuctionSettings?.minRepLevel, FALLBACK_AUCTION_MIN_REP_LEVEL))
)
function normalizePositiveCostMap(raw: unknown, fallback: Record<string, number>): Record<string, number> {
  if (!raw || typeof raw !== "object" || Array.isArray(raw)) return { ...fallback }
  const out: Record<string, number> = {}
  for (const [key, value] of Object.entries(raw as Record<string, unknown>)) {
    const itemId = String(key || "").trim()
    const count = Math.floor(parseNumber(value, 0))
    if (!itemId || count <= 0) continue
    out[itemId] = (out[itemId] || 0) + count
  }
  return Object.keys(out).length > 0 ? out : { ...fallback }
}
export const ITEM_RENAME_COST = normalizePositiveCostMap(rawItemRenameSettings?.cost, FALLBACK_ITEM_RENAME_COST)
export const ITEM_RENAME_MAX_NAME_RUNES = Math.max(
  1,
  Math.floor(parseNumber(rawItemRenameSettings?.maxNameRunes, FALLBACK_ITEM_RENAME_MAX_NAME_RUNES))
)
export const PLAYER_USERNAME_RENAME_COST_ITEM_ID =
  String(rawPlayerUsernameRenameSettings?.costItemId || "").trim() || FALLBACK_PLAYER_USERNAME_RENAME_COST_ITEM_ID
export const PLAYER_USERNAME_RENAME_COST_ITEM_COUNT = Math.max(
  1,
  Math.floor(parseNumber(rawPlayerUsernameRenameSettings?.costItemCount, FALLBACK_PLAYER_USERNAME_RENAME_COST_ITEM_COUNT))
)
function normalizeStringListWithFallback(raw: unknown, fallback: string[]): string[] {
  if (!Array.isArray(raw)) return [...fallback]
  const seen = new Set<string>()
  const out: string[] = []
  for (const value of raw) {
    const normalized = String(value || "").trim().toLowerCase()
    if (!normalized || seen.has(normalized) || !isSupportedItemRenameLabelColor(normalized)) continue
    seen.add(normalized)
    out.push(normalized)
  }
  return out.length > 0 ? out : [...fallback]
}
function isSupportedItemRenameLabelColor(value: string): boolean {
  return FALLBACK_ITEM_RENAME_LABEL_COLORS.includes(value)
}
export const ITEM_RENAME_LABEL_COLORS = normalizeStringListWithFallback(rawItemRenameSettings?.labelColors, FALLBACK_ITEM_RENAME_LABEL_COLORS)
export const VISIBILITY_OFFLINE_DELAY_SECONDS = Math.max(
  0,
  Math.floor(parseNumber(rawPresenceSettings?.visibilityOfflineDelaySeconds, FALLBACK_VISIBILITY_OFFLINE_DELAY_SECONDS))
)
export const VISIBILITY_OFFLINE_DELAY_MS = VISIBILITY_OFFLINE_DELAY_SECONDS * 1000
export const ENHANCE_INHERITANCE_RATIO = Math.max(
  0,
  Math.min(1, parseNumber(rawActionSettings?.enhanceInheritanceRatio, FALLBACK_ENHANCE_INHERITANCE_RATIO))
)
export const ACTION_QUEUE_LANE_LIMIT = Math.max(
  1,
  Math.floor(parseNumber(rawActionSettings?.queueLaneLimit, FALLBACK_ACTION_QUEUE_LANE_LIMIT))
)
export const KITTY_ACTION_QUEUE_LANE_LIMIT = Math.max(
  1,
  Math.floor(parseNumber(rawActionSettings?.kittyQueueLaneLimit, FALLBACK_KITTY_ACTION_QUEUE_LANE_LIMIT))
)
export const BATTLE_ITEM_BAG_MAX_KINDS = Math.max(
  1,
  Math.floor(parseNumber(rawBattleItemBagMaxKinds, FALLBACK_BATTLE_ITEM_BAG_MAX_KINDS))
)
export const BATTLE_ITEM_BAG_MAX_TOTAL_QUANTITY = Math.max(
  1,
  Math.floor(parseNumber(rawBattleItemBagMaxTotalQuantity, FALLBACK_BATTLE_ITEM_BAG_MAX_TOTAL_QUANTITY))
)
export const INTERVAL_BETWEEN_ITEM_AND_ACTION = Math.max(
  0,
  Math.floor(parseNumber(rawIntervalBetweenItemAndAction, FALLBACK_INTERVAL_BETWEEN_ITEM_AND_ACTION))
)
export const INTERVAL_BETWEEN_MULTI_HIT_DAMAGE = Math.max(
  0,
  Math.floor(parseNumber(rawIntervalBetweenMultiHitDamage, FALLBACK_INTERVAL_BETWEEN_MULTI_HIT_DAMAGE))
)
export const BATTLE_CHAIN_FLOAT_JITTER_SCALE = Math.max(
  0,
  Math.min(3, parseNumber(rawBattleChainFloatJitterScale, FALLBACK_BATTLE_CHAIN_FLOAT_JITTER_SCALE))
)
export const ENEMY_HP_REGEN_RATIO = Math.max(
  0,
  Math.min(1, parseNumber(rawCombatBalanceModel?.enemyHpRegenRatio, FALLBACK_ENEMY_HP_REGEN_RATIO))
)
export const PERCENT_PENETRATION_CAP = Math.max(
  0,
  Math.min(1, parseNumber(rawCombatBalanceModel?.percentPenetrationCap, FALLBACK_PERCENT_PENETRATION_CAP))
)
export const DEFENSE_K = Math.max(1, parseNumber(rawCombatBalanceModel?.defenseK, 100))

function normalizeLayoutColumn(raw: unknown): string[] {
  if (!Array.isArray(raw)) return []
  const out: string[] = []
  for (const row of raw) {
    if (typeof row !== "string") {
      if (typeof row === "number") {
        out.push(String(row))
      }
      continue
    }
    const key = row.trim()
    out.push(key)
  }
  return out
}

function normalizeLayoutColumns(raw: unknown, fallback: string[][]): string[][] {
  if (!Array.isArray(raw)) return fallback.map((column) => [...column])
  const hasNested = raw.some((row) => Array.isArray(row))
  const columns = hasNested
    ? raw
        .filter((row) => Array.isArray(row))
        .map((row) => normalizeLayoutColumn(row))
    : [normalizeLayoutColumn(raw)]
  const nonEmptyColumns = columns.filter((column) => column.length > 0)
  if (nonEmptyColumns.length > 0) return nonEmptyColumns
  return fallback.map((column) => [...column])
}

function normalizeAttributeLayoutSceneConfig(raw: RawAttributeLayoutSceneConfig | undefined, fallback: AttributeLayoutSceneConfig): AttributeLayoutSceneConfig {
  const source = raw && typeof raw === "object" ? raw : {}
  const battlePrimary = normalizeLayoutColumns(source.battlePrimary, fallback.battlePrimary)
  const battle = normalizeLayoutColumns(source.battle, fallback.battle)
  const work = normalizeLayoutColumns(source.work, fallback.work)
  const rawPreviewLimit = parseNumber(source.previewLimit, Number.NaN)
  const previewLimit = Number.isFinite(rawPreviewLimit) ? Math.max(1, Math.floor(rawPreviewLimit)) : fallback.previewLimit
  return {
    battlePrimary,
    battle,
    work,
    previewLimit
  }
}

function normalizeAttributeLayout(raw: RawAttributeLayoutConfig | undefined): AttributeLayoutConfig {
  const scenes = raw?.scenes
  return {
    scenes: {
      character_stats: normalizeAttributeLayoutSceneConfig(scenes?.character_stats, FALLBACK_ATTRIBUTE_LAYOUT_CONFIG.scenes.character_stats),
      battle_unit_card: normalizeAttributeLayoutSceneConfig(scenes?.battle_unit_card, FALLBACK_ATTRIBUTE_LAYOUT_CONFIG.scenes.battle_unit_card),
      enemy_quick_ref: normalizeAttributeLayoutSceneConfig(scenes?.enemy_quick_ref, FALLBACK_ATTRIBUTE_LAYOUT_CONFIG.scenes.enemy_quick_ref),
      battle_unit_card_popover: normalizeAttributeLayoutSceneConfig(scenes?.battle_unit_card_popover, FALLBACK_ATTRIBUTE_LAYOUT_CONFIG.scenes.battle_unit_card_popover)
    }
  }
}

function normalizeNumberMap(raw: unknown, fallback: Record<string, number>): Record<string, number> {
  if (!raw || typeof raw !== "object" || Array.isArray(raw)) return { ...fallback }
  const out: Record<string, number> = {}
  for (const [key, value] of Object.entries(raw as Record<string, unknown>)) {
    const normalizedKey = String(key || "").trim()
    const normalizedValue = parseNumber(value, Number.NaN)
    if (!normalizedKey || !Number.isFinite(normalizedValue)) continue
    out[normalizedKey] = normalizedValue
  }
  return Object.keys(out).length > 0 ? out : { ...fallback }
}

function normalizeAlchemyDurations(raw: unknown): AlchemyDurationsConfig {
  const source = raw && typeof raw === "object" && !Array.isArray(raw) ? raw as Record<string, unknown> : {}
  return {
    default: Math.max(1, Math.floor(parseNumber(source.default, FALLBACK_ALCHEMY_DURATIONS_SEC.default))),
    convert: Math.max(1, Math.floor(parseNumber(source.convert, FALLBACK_ALCHEMY_DURATIONS_SEC.convert))),
    bench: Math.max(1, Math.floor(parseNumber(source.bench, FALLBACK_ALCHEMY_DURATIONS_SEC.bench))),
    transmute: Math.max(1, Math.floor(parseNumber(source.transmute, FALLBACK_ALCHEMY_DURATIONS_SEC.transmute))),
    smelt: Math.max(1, Math.floor(parseNumber(source.smelt, FALLBACK_ALCHEMY_DURATIONS_SEC.smelt)))
  }
}

function normalizeAlchemyDurationList(raw: unknown): number[] {
  if (!Array.isArray(raw)) return [...FALLBACK_ALCHEMY_TRANSMUTE_DURATION_BY_RARITY_INDEX]
  const parsed = raw
    .map((value) => Math.floor(parseNumber(value, Number.NaN)))
    .filter((value) => Number.isFinite(value) && value > 0)
  return parsed.length > 0 ? parsed : [...FALLBACK_ALCHEMY_TRANSMUTE_DURATION_BY_RARITY_INDEX]
}

function normalizeAlchemyConvertGroupEconomy(raw: unknown): Record<string, AlchemyConvertGroupEconomy> {
  const source = raw && typeof raw === "object" && !Array.isArray(raw) ? raw as Record<string, unknown> : {}
  const out: Record<string, AlchemyConvertGroupEconomy> = {}
  for (const [rarity, fallback] of Object.entries(FALLBACK_ALCHEMY_CONVERT_GROUP_ECONOMY_BY_RARITY)) {
    const row = source[rarity]
    const data = row && typeof row === "object" && !Array.isArray(row) ? row as Record<string, unknown> : {}
    out[rarity] = {
      gold: Math.max(0, Math.floor(parseNumber(data.gold, fallback.gold))),
      duration: Math.max(1, parseNumber(data.duration, fallback.duration)),
      exp: Math.max(0, parseNumber(data.exp, fallback.exp)),
      upgrade: Math.max(0, Math.min(1, parseNumber(data.upgrade, fallback.upgrade)))
    }
  }
  return out
}

function normalizeAlchemyInheritGlobalConfig(raw: unknown): AlchemyInheritGlobalConfig {
  const source = raw && typeof raw === "object" && !Array.isArray(raw) ? raw as Record<string, unknown> : {}
  return {
    baseInheritSlots: Math.max(0, Math.floor(parseNumber(source.baseInheritSlots, FALLBACK_ALCHEMY_INHERIT_CONFIG.baseInheritSlots))),
    baseSuppressSlots: Math.max(0, Math.floor(parseNumber(source.baseSuppressSlots, FALLBACK_ALCHEMY_INHERIT_CONFIG.baseSuppressSlots))),
    inheritSlotPrecisionScalePerPoint: Math.max(0, parseNumber(source.inheritSlotPrecisionScalePerPoint, FALLBACK_ALCHEMY_INHERIT_CONFIG.inheritSlotPrecisionScalePerPoint)),
    suppressSlotStabilityBonusPerPoint: Math.max(0, parseNumber(source.suppressSlotStabilityBonusPerPoint, FALLBACK_ALCHEMY_INHERIT_CONFIG.suppressSlotStabilityBonusPerPoint)),
    positiveIgnoreKeepChanceBase: normalizeRatio(source.positiveIgnoreKeepChanceBase, FALLBACK_ALCHEMY_INHERIT_CONFIG.positiveIgnoreKeepChanceBase),
    negativeIgnoreKeepChanceBase: normalizeRatio(source.negativeIgnoreKeepChanceBase, FALLBACK_ALCHEMY_INHERIT_CONFIG.negativeIgnoreKeepChanceBase),
    hybridMergeDivisor: Math.max(1, parseNumber(source.hybridMergeDivisor, FALLBACK_ALCHEMY_INHERIT_CONFIG.hybridMergeDivisor)),
    inbreedMutationChance: normalizeRatio(source.inbreedMutationChance, FALLBACK_ALCHEMY_INHERIT_CONFIG.inbreedMutationChance),
    inbreedGoodChance: normalizeRatio(source.inbreedGoodChance, FALLBACK_ALCHEMY_INHERIT_CONFIG.inbreedGoodChance),
    inbreedGoodMultiplierRange: normalizeAlchemyInheritMultiplierRange(source.inbreedGoodMultiplierRange, FALLBACK_ALCHEMY_INHERIT_CONFIG.inbreedGoodMultiplierRange),
    inbreedBadMultiplierRange: normalizeAlchemyInheritMultiplierRange(source.inbreedBadMultiplierRange, FALLBACK_ALCHEMY_INHERIT_CONFIG.inbreedBadMultiplierRange),
    mutationChance: normalizeRatio(source.mutationChance, FALLBACK_ALCHEMY_INHERIT_CONFIG.mutationChance),
    numericValueCapMultiplier: Math.max(1, parseNumber(source.numericValueCapMultiplier, FALLBACK_ALCHEMY_INHERIT_CONFIG.numericValueCapMultiplier))
  }
}

function normalizeAlchemyInheritMultiplierRange(raw: unknown, fallback: AlchemyInheritMultiplierRangeConfig): AlchemyInheritMultiplierRangeConfig {
  const source = raw && typeof raw === "object" && !Array.isArray(raw) ? raw as Record<string, unknown> : {}
  const min = Math.max(0, parseNumber(source.min, fallback.min))
  const max = Math.max(min, parseNumber(source.max, fallback.max))
  return { min, max }
}

function normalizeCombatScoring(raw: unknown): CombatScoringConfig {
  const source = raw && typeof raw === "object" && !Array.isArray(raw) ? raw as Record<string, unknown> : {}
  const rawConstants = source.constants && typeof source.constants === "object" && !Array.isArray(source.constants)
    ? source.constants as Record<string, unknown>
    : {}
  const constants = { ...FALLBACK_COMBAT_SCORING_CONFIG.constants }
  for (const key of Object.keys(constants) as Array<keyof CombatScoringConfig["constants"]>) {
    constants[key] = parseNumber(rawConstants[key], constants[key])
  }
  return {
    constants,
    attributeUnitValues: normalizeNumberMap(source.attributeUnitValues, FALLBACK_COMBAT_SCORING_CONFIG.attributeUnitValues),
    rarityMultipliers: normalizeNumberMap(source.rarityMultipliers, FALLBACK_COMBAT_SCORING_CONFIG.rarityMultipliers),
    slotMultipliers: normalizeNumberMap(source.slotMultipliers, FALLBACK_COMBAT_SCORING_CONFIG.slotMultipliers),
    enemyTierMultipliers: normalizeNumberMap(source.enemyTierMultipliers, FALLBACK_COMBAT_SCORING_CONFIG.enemyTierMultipliers),
    enemyRewardMultipliers: normalizeNumberMap(source.enemyRewardMultipliers, FALLBACK_COMBAT_SCORING_CONFIG.enemyRewardMultipliers)
  }
}

export const COMBAT_SCORING_CONFIG = normalizeCombatScoring(rawCombatScoring)
export const ATTRIBUTE_LAYOUT_CONFIG = normalizeAttributeLayout(rawAttributeLayout)
