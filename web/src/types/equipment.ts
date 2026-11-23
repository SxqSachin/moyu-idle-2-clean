import type { ItemDefinition } from "@/types/game-data"

export type EquipmentRuntimeSnapshot = Record<string, unknown>
export type EquipmentAffixMap = Record<string, unknown>
export type EquipmentAffixSlot = {
  slotIndex: number
  affixId: string
  rarityId: string
  sourceKind?: "template" | "craft" | string
  alchemyRulesetVersion?: string
  rollValue?: number
  valuePrecision?: number
  attrMods?: Record<string, number>
  effectType?: string
  effectParams?: Record<string, unknown>
  displayNameKey?: string
  displayNameArgs?: Record<string, unknown>
}
export type EquipmentItemType = "weapon" | "armor" | "cloak" | "helmet" | "pants" | "boots" | "accessory"
export type EquipmentSlotType = "work" | "battle"
export type EquipmentPanelTab = EquipmentSlotType | "fishing"
export type RuntimeItemDefinition = ItemDefinition & { isEquipment: boolean }
export type EquipmentItemDefinition = RuntimeItemDefinition & { isEquipment: true; type: string[] } & EquipmentTemplateDefinition
export type TargetDirection = "vanguard" | "midguard" | "rearguard"
export type TargetSeekStrategy = "rear_to_front" | "front_to_rear" | "random_alive" | "none"
export type TargetScope = "ally" | "enemy" | "all" | "self"
export type SelectRule =
  | "hp_max"
  | "hp_min"
  | "hp_percent_max"
  | "hp_percent_min"
  | "first"
  | "attr_max"
  | "attr_min"
  | "random"
  | "boss_first"
  | "self"
export type BasicAttackFormulaTermDefinition = {
  attrKey: string
  coefficient: number
}
export type CombatFormulaTermDefinition = {
  attrKey: string
  coefficient: number
  perSkillLevel?: number
}
export type BasicAttackFormulaDefinition = {
  damageType: "physical" | "magical"
  formula: BasicAttackFormulaTermDefinition[]
  damageSpec?: BasicAttackDamageSpecDefinition
}
export type BasicAttackTargetPredicateDefinition = {
  kind: "compare_attr" | "exclude_enemy_type" | "require_target_has_status" | "exclude_target_has_status" | "target_hp_not_full"
  actorAttr?: string
  targetAttr?: string
  operator?: string
  enemyType?: string
  statusId?: string
}
export type BasicAttackTargetingDefinition = {
  targetPositions: TargetDirection[]
  secondarySeek: TargetSeekStrategy
  targetScope: TargetScope
  selectRule: SelectRule
  selectAttrKey: string
  fallbackSelfIfEmpty: boolean
  targetCount: {
    base: number
    formula: CombatFormulaTermDefinition[]
    max?: number
  }
  targetPredicates?: BasicAttackTargetPredicateDefinition[]
}
export type BasicAttackAmountFormulaDefinition = {
  baseAdd: number
  terms: CombatFormulaTermDefinition[]
}
export type EquipmentHookFormulaDefinition = {
  baseAdd?: number
  terms?: Array<CombatFormulaTermDefinition & { perEnhanceLevel?: number }>
}
export type BasicAttackDamageSpecDefinition = {
  damageType?: "physical" | "magical"
  amountFormula?: BasicAttackAmountFormulaDefinition
  hitCount?: {
    base: number
    formula: CombatFormulaTermDefinition[]
  }
  hitDamageScaling?: {
    kind: "powered_exponential_decay"
    secondHitMultiplier: number
    curvePower: number
  }
  flatDefensePenetrationFormula?: BasicAttackAmountFormulaDefinition
  percentDefensePenetrationFormula?: BasicAttackAmountFormulaDefinition
  physicalFlatDefensePenetrationFormula?: BasicAttackAmountFormulaDefinition
  physicalPercentDefensePenetrationFormula?: BasicAttackAmountFormulaDefinition
  magicalFlatDefensePenetrationFormula?: BasicAttackAmountFormulaDefinition
  magicalPercentDefensePenetrationFormula?: BasicAttackAmountFormulaDefinition
  extraDamageAmplificationFormula?: BasicAttackAmountFormulaDefinition
  bonusAgainstTargetTypes?: Record<string, number>
}
export type BasicAttackStatusPayloadDefinition = {
  statusIds: string[]
  durationBaseTurns: number
  durationPerSkillLevelTurns: number
  durationFormula?: BasicAttackAmountFormulaDefinition
  applyChanceBase?: number
  applyChancePerSkillLevel?: number
  applyChanceFormula?: BasicAttackAmountFormulaDefinition
  applyChanceMin?: number
  applyChanceMax?: number
  payload?: Record<string, unknown>
  payloadFormula?: Record<string, BasicAttackAmountFormulaDefinition>
}
export type BasicAttackSummonPayloadDefinition = {
  unitId: string
  side?: "source" | "player" | "enemy"
  countFormula: BasicAttackAmountFormulaDefinition
  durationFormula?: BasicAttackAmountFormulaDefinition
  durationTurns?: number
  maxCountPerUnit?: number
  maxCountFormula?: BasicAttackAmountFormulaDefinition
  persistAcrossBattles?: boolean
  onCasterDeath?: "keep_until_duration_end" | "despawn_immediately"
  onCapOverflow?: "replace_oldest" | "fail_new_summon"
  onDispel?: "despawn_immediately" | "ignore_dispel"
  onDurationEnd?: "despawn_immediately"
  statsFormula?: Record<string, BasicAttackAmountFormulaDefinition>
}
export type BattleWeightedStatusChoiceDefinition = {
  id: string
  weight: number
  statusId: string
  durationTurns: number
  payload?: Record<string, unknown>
}
export type BasicAttackEffectDefinition = {
  kind: "damage" | "heal" | "resource" | "status" | "summon" | "adjust_skill_cooldown" | "weighted_random_per_target"
  resource?: string
  damageType?: "physical" | "magical"
  targeting: BasicAttackTargetingDefinition
  onlyWhenNoTargetFromEffects?: number[]
  reuseTargetsFromEffect?: number
  ignoreUntargetable?: boolean
  requiredCasterStatuses?: string[]
  requiredTargetStatuses?: string[]
  forbiddenCasterStatuses?: string[]
  forbiddenTargetStatuses?: string[]
  consumeStatusesOnHit?: string[]
  consumeCasterStatusesOnHit?: string[]
  amountFormula?: BasicAttackAmountFormulaDefinition
  damageSpec?: BasicAttackDamageSpecDefinition
  statusPayload?: BasicAttackStatusPayloadDefinition
  summonPayload?: BasicAttackSummonPayloadDefinition
  choices?: BattleWeightedStatusChoiceDefinition[]
  applyChanceBase?: number
  applyChancePerSkillLevel?: number
  applyChanceFormula?: BasicAttackAmountFormulaDefinition
  applyChanceMin?: number
  applyChanceMax?: number
  amount?: number
  includeBasic?: boolean
  skillIds?: string[]
  excludeSkillIds?: string[]
  mpCost?: BasicAttackAmountFormulaDefinition
}
export type BattleUsableItemDefinition = {
  id: string
  itemId: string
  name?: string
  icon?: string
  cooldownTurns: number
  targeting: BasicAttackTargetingDefinition
  effects: BasicAttackEffectDefinition[]
  isTrigger?: boolean
  dungeonIds?: string[]
  spawnEnemyIds?: string[]
  minWave?: number
  perWaveChance?: number
  intervalWaves?: number
  partyTriggerGroup?: string
}
export type EquipmentBattleHookConditionDefinition = Record<string, unknown>
export type EquipmentBattleHookEffectDefinition = {
  kind: string
  buffId?: string
  durationTurns?: number
  targetMode?: "self" | "source" | "targeting"
  targeting?: BasicAttackTargetingDefinition
  value?: number
  amount?: number
  valueFormula?: EquipmentHookFormulaDefinition
  amountFormula?: EquipmentHookFormulaDefinition
  probabilityKinds?: Array<"equipment_proc" | "status_apply">
  sourceIds?: string[]
  excludeSourceIds?: string[]
  procChanceBase?: number
  procChanceMin?: number
  procChanceMax?: number
  procChanceFormula?: EquipmentHookFormulaDefinition
  probabilityGroup?: string
  maxSuccessesPerTurn?: number
  ignoreProbabilityHooks?: boolean
  [key: string]: unknown
}
export type EquipmentBattleHookDefinition = {
  id?: string
  trigger: string
  priority: number
  activationCost?: {
    maxHpRatio?: number
    maxMpRatio?: number
  }
  cooldownCompletedTurns?: number
  subjectFilter?: {
    requireAlive?: boolean
    includeUnitTypes?: string[]
    excludeUnitTypes?: string[]
    includeSummonTemplateIds?: string[]
    excludeSummonTemplateIds?: string[]
  }
  eventFilter?: {
    buffIds?: string[]
    excludeBuffIds?: string[]
    buffKinds?: Array<"buff" | "debuff">
    firstBuffInstanceOnly?: boolean
    applicationKinds?: Array<"attach" | "refresh">
    excludeSelf?: boolean
    turnKinds?: Array<"ordered" | "extra">
    includePreparation?: boolean
    includeSuppressedTurns?: boolean
    probabilityKinds?: Array<"equipment_proc" | "status_apply">
    sourceIds?: string[]
    excludeSourceIds?: string[]
  }
  conditions: EquipmentBattleHookConditionDefinition[]
  effects: EquipmentBattleHookEffectDefinition[]
  maxTriggersPerTurn?: number
  maxTriggersPerWave?: number
  maxTriggersPerBattle?: number
  sharedTriggerGroup?: string
  hideDescription?: boolean
}
export type EquipmentTemplateDefinition = {
	grantedSkill?: { skillId: string; baseCapacity: number; capacityMilestones: number[] }
  id: string
  equipType: EquipmentSlotType
  equipTargets?: string[]
  slot: string
  occupySlots: string[]
  weaponClass?: string[]
  equipmentSecondaryClass?: string
  statusRequirement: Record<string, number>
  accessoryGroup: string
  affixSlotCount: number
  affixAllowedIds: string[]
  baseAttrs: Record<string, number>
  maxDurability: number
  decayRate: number
  enhanceMaxLevel: number
  enhanceDurationS: number
  enhanceExp: number
  enhanceProtectItemId: string[]
  enhanceCostCurve: {
    base: Record<string, number>
    step: Record<string, number>
    overrides: Record<string, Record<string, number>>
  }
  enhanceAttrCurve: {
    base: Record<string, number>
    step: Record<string, number>
    overrides: Record<string, Record<string, number>>
  }
  enhanceAffixMilestonePools?: Record<string, {
    affixId: string
    rarityId: string
    rollValue?: number
  }[]>
  basicAttackTargetDirection?: TargetDirection
  targetSeekStrategy?: TargetSeekStrategy
  basicAttackFormula?: BasicAttackFormulaDefinition
  basicAttackEffects?: BasicAttackEffectDefinition[]
  battleHooks?: EquipmentBattleHookDefinition[]
  attrConversions?: Array<{
    fromAttr: string
    toAttr: string
    threshold?: number
    ratio?: number
    ratioFormula?: { baseAdd: number; terms?: Array<{ attrKey: string; coefficient: number; perSkillLevel?: number }> }
  }>
  defaultAffixSlots?: EquipmentAffixSlot[]
}

export type RarityDefinition = {
  id: string
  nameKey: string
  order: number
  color: string
}

export type ItemTypeDefinition = {
  id: string
  nameKey: string
  descKey: string
  order: number
}

export type AffixDefinition = {
  id: string
  nameKey: string
  descKey: string
  effectType: string
  effectParams: Record<string, unknown>
  type: EquipmentSlotType
  valuePrecision: number
  maxEffectivePerEquipment: number
  maxEffectivePerBody: number
  allowedRarities: string[]
  active: boolean
  smeltOverride?: Record<string, AffixSmeltRewardEntry[]>
}

export type AffixSmeltRewardEntry = {
  itemId: string
  minCount: number
  maxCount: number
}

export type AffixSmeltRule = {
  id: string
  rarityRewards: Record<string, AffixSmeltRewardEntry[]>
  active: boolean
}

export type EquipmentOperationInfo = {
  instanceId: number
  templateItemId: string
  enhanceLevel: number
  affixSlotCount?: number
  affixSlots?: EquipmentAffixSlot[]
  detachRules?: {
    toolItemId: string
    destroyChanceBase: number
    luckImpactFactor: number
    affixDropBeforeDestroyCheck: boolean
  }[]
  enchantRules?: {
    toolItemId: string
    consumeCount: number
    costs?: Record<string, number>
    failChance: number
    rarityWeights: Record<string, number>
    allowedAffixIds?: string[]
  }[]
  carrierRules?: {
    carrierItemId: string
    allowedRarityIds?: string[]
    maxCustomAffixes?: number
    allowedTraitKind?: string
  }[]
}

export type FishInstanceData = {
  speciesId: string
  ageDays: number
  sex: "male" | "female"
  lengthCm: number
  weightKg: number
  condition: number
  health: number
  waterId: string
  spotId: string
  caughtAt: string
  rulesetVersion: string
  recordFlags?: string[]
  origin?: "wild" | "cultured"
  pondZone?: "main" | "nursery"
  lifeState?: "alive" | "dead"
  generation?: number
  genes?: {
    sizePotential?: number
    growthRate?: number
    feedEfficiency?: number
    vitality?: number
    lifespan?: number
    fertility?: number
    mutations?: string[]
  }
}

export type ItemInstance = {
  instanceId: number
  templateItemId: string
  customData?: ItemCustomization
  quantity: number
  state: string
  fishData?: FishInstanceData
  archivedAt?: string | null
  forgedSkillBook?: {
    id: string
    instanceId: number
    customSkillId: string
    revisionId: string
    sourceSkillId: string
    status: string
    rulesetVersion: number
    usable: boolean
    targetKittyId?: number
    nameParams?: Record<string, unknown>
    compiledSkill?: Record<string, unknown>
    budget?: Record<string, unknown>
  }
  equipType?: EquipmentSlotType
  equippedSlot?: string
}

export type ItemCustomization = {
  name?: string
  labelColor?: string
}

export type EquipmentInstanceExtension = {
  enhanceLevel: number
  durability: number
  maxDurability: number
  baseSnapshot?: EquipmentRuntimeSnapshot
  affixes?: EquipmentAffixSlot[]
}

export type EquipmentItemInstance = ItemInstance & EquipmentInstanceExtension
export type InventoryInstance = EquipmentItemInstance

export type ItemDetailItemMeta = Pick<RuntimeItemDefinition, "id" | "type" | "rarity" | "nameKey" | "descKey" | "effectDescKey" | "economyBaseValue" | "useGrantSkillId" | "useGrantCollectibleId" | "useOperationEffectId" | "useSkillExpGain"> & {
  isEquipment?: boolean
  affixSlotCount?: number
}

export type ItemDetailResponse = {
  item: ItemDetailItemMeta
  operationEffect?: import("@services/operation-effects").OperationEffectDefinitionView
  recycleGoldCoefficient?: number
  recycleAffixMismatchExtraCoefficient?: number
  instance?: EquipmentItemInstance
  operations?: EquipmentOperationInfo
  enhance?: {
    maxLevel: number
    currentLevel: number
    cumulativeAttrs: Record<string, number>
    nextPreview?: {
      nextLevel: number
      baseSuccessRate?: number
      successRateBonus?: number
      successRate: number
      costs: Record<string, number>
      deltaAttrs: Record<string, number>
    }
  }
}
