import { COMBAT_SCORING_CONFIG, type CombatScoringConfig } from "@/config/shared-global-config"
import { getGameDataChunk } from "@services/game-data-runtime"
import { hitDamageEquivalentSum, normalizeHitDamageScaling } from "@services/hit-damage-scaling"

export type CombatScoreResult = {
  score: number
  goldValue: number
  detail: Record<string, number>
  sections?: CombatScoreSection[]
}

export type CombatScoreSection = {
  id: string
  label: string
  score: number
  items?: CombatScoreItem[]
}

export type CombatScoreItem = {
  label: string
  score: number
  meta?: string
}

type Row = Record<string, unknown>

type CombatScoringData = {
  equipmentByID: Map<string, Row>
  affixByID: Map<string, Row>
  skillByID: Map<string, Row>
  enemyByID: Map<string, Row>
  attributeIDs: string[]
  percentAttributeIDs: Set<string>
}

let combatScoringData: CombatScoringData | null = null

function getCombatScoringData(): CombatScoringData {
  if (!combatScoringData) {
    const attributes = ((getGameDataChunk("attributes") as { attributes?: Row[] }).attributes || []) as Row[]
    const equipmentTemplates = ((getGameDataChunk("equipment_templates") as { equipment_templates?: Row[] }).equipment_templates || []) as Row[]
    const affixDefinitions = ((getGameDataChunk("affix_definitions") as { affix_definitions?: Row[] }).affix_definitions || []) as Row[]
    const skills = ((getGameDataChunk("skills") as { skills?: Row[] }).skills || []) as Row[]
    const enemies = ((getGameDataChunk("enemies") as { enemies?: Row[] }).enemies || []) as Row[]
    combatScoringData = {
      equipmentByID: new Map(equipmentTemplates.map((row) => [String(row.id || ""), row])),
      affixByID: new Map(affixDefinitions.map((row) => [String(row.id || ""), row])),
      skillByID: new Map(skills.map((row) => [String(row.id || ""), row])),
      enemyByID: new Map(enemies.map((row) => [String(row.id || ""), row])),
      attributeIDs: attributes.map((row) => String(row.id || "")).filter(Boolean),
      percentAttributeIDs: new Set(attributes.filter((row) => String(row.unit || "") === "percent").map((row) => String(row.id || "")).filter(Boolean))
    }
  }
  return combatScoringData
}

function asNumber(raw: unknown, fallback = 0): number {
  const parsed = Number(raw)
  return Number.isFinite(parsed) ? parsed : fallback
}

function isRecord(raw: unknown): raw is Row {
  return Boolean(raw) && typeof raw === "object" && !Array.isArray(raw)
}

function asArray(raw: unknown): unknown[] {
  return Array.isArray(raw) ? raw : []
}

function round(value: number, digits = 2): number {
  const p = 10 ** digits
  return Math.round((Number.isFinite(value) ? value : 0) * p) / p
}

function withGold(score: number, detail: Record<string, number>, sections: CombatScoreSection[] = []): CombatScoreResult {
  const safe = Math.max(0, score)
  return {
    score: round(safe),
    goldValue: round(safe * COMBAT_SCORING_CONFIG.constants.goldPerBudget),
    detail,
    sections
  }
}

export function scoreAttribute(attrKey: string, value: unknown, config: CombatScoringConfig = COMBAT_SCORING_CONFIG): number {
  return round(asNumber(value, 0) * (getCombatScoringData().percentAttributeIDs.has(attrKey) ? 100 : 1) * (config.attributeUnitValues[attrKey] ?? 0))
}

export function scoreAttributeMap(raw: unknown, config: CombatScoringConfig = COMBAT_SCORING_CONFIG): number {
  if (!isRecord(raw)) return 0
  let total = 0
  for (const [key, value] of Object.entries(raw)) {
    total += scoreAttribute(key, value, config)
  }
  return total
}

function levelBudget(level: unknown, config = COMBAT_SCORING_CONFIG): number {
  const safeLevel = Math.max(1, asNumber(level, 1))
  return config.constants.levelBudgetBase + config.constants.levelBudgetGrowth * safeLevel ** config.constants.levelBudgetPower
}

function formulaBreakdown(raw: unknown, level = 1, config = COMBAT_SCORING_CONFIG): { score: number; items: CombatScoreItem[] } {
  if (!isRecord(raw)) return { score: 0, items: [] }
  const items: CombatScoreItem[] = []
  let total = asNumber(raw.baseAdd, 0)
  if (total !== 0) items.push({ label: "基础值", score: round(total), meta: `baseAdd ${total}` })
  for (const term of asArray(raw.terms)) {
    if (!isRecord(term)) continue
    const attrKey = String(term.attrKey || "")
    const coeff = asNumber(term.coefficient, 0) + asNumber(term.perSkillLevel, 0) * Math.max(0, level - 1)
    const attrValue = config.attributeUnitValues[attrKey] ?? 1
    const contribution = coeff * attrValue
    total += contribution
    items.push({ label: attrKey, score: round(contribution), meta: `${round(coeff, 4)} * ${round(attrValue, 4)}` })
  }
  return { score: total, items }
}

function formulaScore(raw: unknown, level = 1, config = COMBAT_SCORING_CONFIG): number {
  return formulaBreakdown(raw, level, config).score
}

function targetCountBreakdown(targeting: unknown, level = 1, config = COMBAT_SCORING_CONFIG): { count: number; items: CombatScoreItem[] } {
  const targetCount = isRecord(targeting) && isRecord(targeting.targetCount) ? targeting.targetCount : {}
  const base = Math.max(1, asNumber(targetCount.base, 1))
  const formula = formulaBreakdown({ baseAdd: 0, terms: asArray(targetCount.formula) }, level, config)
  const maxRaw = Math.trunc(asNumber(targetCount.max, Number.NaN))
  const uncapped = base + Math.max(0, formula.score)
  const count = Number.isFinite(maxRaw) && maxRaw > 0 ? Math.min(maxRaw, uncapped) : uncapped
  const items: CombatScoreItem[] = [{ label: "基础目标数", score: round(base) }]
  if (formula.score !== 0) items.push({ label: "目标数公式", score: round(formula.score), meta: formula.items.map((item) => `${item.label}: ${item.score}`).join(", ") })
  if (Number.isFinite(maxRaw) && maxRaw > 0) items.push({ label: "最大目标数", score: round(maxRaw) })
  return { count: Math.max(1, count), items }
}

function targetFactor(targeting: unknown, level = 1): number {
  return targetCountBreakdown(targeting, level).count
}

function targetingUtilityScore(targeting: unknown, config = COMBAT_SCORING_CONFIG): number {
  if (!isRecord(targeting)) return 0
  const positions = asArray(targeting.targetPositions).map((value) => String(value || ""))
  const positionBudget = positions.reduce((sum, position) => {
    if (position === "rearguard") return sum + config.constants.rearTargetValue
    if (position === "midguard") return sum + config.constants.targetPositionValue
    return sum
  }, 0)
  const seek = String(targeting.secondarySeek || "")
  const seekBudget = seek === "rear_to_front"
    ? config.constants.seekStrategyValue
    : seek === "random_alive"
      ? config.constants.seekStrategyValue * 0.45
      : 0
  const rule = String(targeting.selectRule || "")
  const ruleBudget = rule && rule !== "random"
    ? (rule === "boss_first" ? config.constants.selectRuleValue * 1.4 : config.constants.selectRuleValue)
    : 0
  return positionBudget + seekBudget + ruleBudget + (targeting.fallbackSelfIfEmpty ? config.constants.fallbackSelfValue : 0)
}

function damageSpecBreakdown(raw: unknown, level = 1): { score: number; items: CombatScoreItem[] } {
  if (!isRecord(raw)) return { score: 0, items: [] }
  const amount = Math.abs(formulaScore(raw.amountFormula, level))
  const hitCount = isRecord(raw.hitCount) ? Math.max(1, asNumber(raw.hitCount.base, 1)) : 1
  const hitDamageScaling = normalizeHitDamageScaling(raw.hitDamageScaling)
  const damageHitMultiplier = hitDamageScaling ? hitDamageEquivalentSum(hitDamageScaling, hitCount) : hitCount
  const penetration =
    Math.abs(formulaScore(raw.flatDefensePenetrationFormula, level)) * 0.35 +
    Math.abs(formulaScore(raw.physicalFlatDefensePenetrationFormula, level)) * 0.35 +
    Math.abs(formulaScore(raw.magicalFlatDefensePenetrationFormula, level)) * 0.35 +
    Math.abs(formulaScore(raw.percentDefensePenetrationFormula, level)) * 35 +
    Math.abs(formulaScore(raw.physicalPercentDefensePenetrationFormula, level)) * 35 +
    Math.abs(formulaScore(raw.magicalPercentDefensePenetrationFormula, level)) * 35
  const amp = Math.abs(formulaScore(raw.extraDamageAmplificationFormula, level)) * 45
  const score = amount * damageHitMultiplier + penetration + amp
  const items: CombatScoreItem[] = []
  if (amount) items.push({
    label: "DamageSpec数值",
    score: round(amount * damageHitMultiplier),
    meta: hitDamageScaling
      ? `hitCount ${round(hitCount)}, equivalentHits ${round(damageHitMultiplier, 4)}`
      : `hitCount ${round(hitCount)}`
  })
  if (penetration) items.push({ label: "穿透", score: round(penetration) })
  if (amp) items.push({ label: "增伤", score: round(amp) })
  return { score, items }
}

function damageSpecScore(raw: unknown, level = 1): number {
  return damageSpecBreakdown(raw, level).score
}

function effectBreakdown(effect: unknown, level = 1, label = "效果"): CombatScoreSection {
  if (!isRecord(effect)) return { id: label, label, score: 0, items: [] }
  const kind = String(effect.kind || "")
  const utility = targetingUtilityScore(effect.targeting)
  const target = targetCountBreakdown(effect.targeting, level)
  const items: CombatScoreItem[] = []
  let base = 0
  if (kind === "damage") {
    const spec = damageSpecBreakdown(effect.damageSpec, level)
    const amount = formulaBreakdown(effect.amountFormula, level)
    base = Math.max(spec.score, Math.abs(amount.score))
    items.push({ label: "数值公式", score: round(Math.abs(amount.score)), meta: amount.items.map((item) => `${item.label}: ${item.score}`).join(", ") })
    for (const item of amount.items) items.push({ label: `公式/${item.label}`, score: round(Math.abs(item.score)), meta: item.meta })
    items.push(...spec.items)
  } else if (kind === "heal") {
    const amount = formulaBreakdown(effect.amountFormula, level)
    base = Math.abs(amount.score) * 0.9
    items.push({ label: "治疗公式", score: round(base), meta: amount.items.map((item) => `${item.label}: ${item.score}`).join(", ") })
    for (const item of amount.items) items.push({ label: `公式/${item.label}`, score: round(Math.abs(item.score) * 0.9), meta: item.meta })
  } else if (kind === "status") {
    const payload = isRecord(effect.statusPayload) ? effect.statusPayload : {}
    const duration = asNumber(payload.durationBaseTurns, 1) + asNumber(payload.durationPerSkillLevelTurns, 0) * Math.max(0, level - 1)
    base = (asArray(payload.statusIds).length || 1) * Math.max(1, duration) * 12
    items.push({ label: "状态基础值", score: round(base), meta: `${asArray(payload.statusIds).length || 1}个状态 * ${round(duration)}回合` })
  }
  items.push({ label: "目标数乘区", score: round(base * target.count), meta: `x${round(target.count, 2)}` })
  for (const item of target.items) items.push({ ...item, label: `目标/${item.label}` })
  if (utility) items.push({ label: "目标策略", score: round(utility) })
  return { id: label, label, score: round(base * target.count + utility), items }
}

function effectScore(effect: unknown, level = 1): number {
  return effectBreakdown(effect, level).score
}

function effectsListScore(raw: unknown, level = 1): number {
  return asArray(raw).reduce<number>((sum, effect) => sum + effectScore(effect, level), 0)
}

function effectsBreakdown(raw: unknown, level = 1, labelPrefix = "效果"): CombatScoreSection[] {
  return asArray(raw).map((effect, index) => effectBreakdown(effect, level, `${labelPrefix} #${index + 1}`))
}

function curveScore(raw: unknown, maxLevel = 0): number {
  if (!isRecord(raw)) return 0
  const base = Math.abs(scoreAttributeMap(raw.base))
  const step = Math.abs(scoreAttributeMap(raw.step)) * Math.max(0, maxLevel)
  const overrides = isRecord(raw.overrides)
    ? Object.values(raw.overrides).reduce<number>((sum, value) => sum + Math.abs(scoreAttributeMap(value)), 0)
    : 0
  return base + step + overrides
}

function effectsTargetingScore(raw: unknown): number {
  return asArray(raw).reduce<number>((sum, effect) => isRecord(effect) ? sum + targetingUtilityScore(effect.targeting) : sum, 0)
}

export function scoreSkillDefinition(skillOrID: string | Row | null | undefined): CombatScoreResult | null {
  const row = typeof skillOrID === "string" ? getCombatScoringData().skillByID.get(skillOrID) : skillOrID
  if (!row) return null
  const level = Math.max(1, asNumber(row.level, 1))
  const baseBudget = levelBudget(level)
  const sections = effectsBreakdown(row.effects, level, "技能效果")
  const effectBudget = sections.reduce((sum, section) => sum + section.score, 0)
  const targetingBudget = baseBudget * (targetFactor(row.targeting, level) - 1) * 0.35
  const mpPenalty = asNumber(row.mpCost, 0) * (COMBAT_SCORING_CONFIG.attributeUnitValues.mp ?? 0.05) * COMBAT_SCORING_CONFIG.constants.mpCostPenalty
  const cooldownPenalty = baseBudget * asNumber(row.cooldown, 0) * COMBAT_SCORING_CONFIG.constants.cooldownPenaltyPerTurn
  const requirementBudget = scoreAttributeMap(row.statusRequirement) * 0.08
  return withGold(effectBudget + targetingBudget + requirementBudget - mpPenalty - cooldownPenalty, {
    effectBudget: round(effectBudget),
    targetingBudget: round(targetingBudget),
    requirementBudget: round(requirementBudget),
    mpPenalty: round(mpPenalty),
    cooldownPenalty: round(cooldownPenalty)
  }, sections)
}

export function scoreEquipmentDefinition(itemOrID: string | Row | null | undefined): CombatScoreResult | null {
  const row = typeof itemOrID === "string" ? getCombatScoringData().equipmentByID.get(itemOrID) : itemOrID
  if (!row) return null
  const maxLevel = Math.max(0, asNumber(row.enhanceMaxLevel, 0))
  const baseAttrs = Math.abs(scoreAttributeMap(row.baseAttrs))
  const enhanceAttrs = curveScore(row.enhanceAttrCurve, maxLevel)
  const sections = effectsBreakdown(row.basicAttackEffects, 1, "普攻效果")
  const basicAttack = sections.reduce((sum, section) => sum + section.score, 0)
  const targetingUtility = effectsTargetingScore(row.basicAttackEffects)
  const battleHooks = asArray(row.battleHooks).length * COMBAT_SCORING_CONFIG.constants.battleHookValue
  const affixSlots = asNumber(row.affixSlotCount, 0) * COMBAT_SCORING_CONFIG.constants.affixSlotBaseBudget
  const slotFactor = COMBAT_SCORING_CONFIG.slotMultipliers[String(row.slot || "")] ?? 1
  const score = (baseAttrs + basicAttack + battleHooks + affixSlots) * slotFactor
  return withGold(score, {
    baseAttrs: round(baseAttrs),
    enhanceAttrs: round(enhanceAttrs),
    basicAttack: round(basicAttack),
    targetingUtility: round(targetingUtility),
    battleHooks: round(battleHooks),
    affixSlots: round(affixSlots),
    slotFactor: round(slotFactor)
  }, sections)
}

export function scoreAffixDefinition(affixOrID: string | Row | null | undefined): CombatScoreResult | null {
  const row = typeof affixOrID === "string" ? getCombatScoringData().affixByID.get(affixOrID) : affixOrID
  if (!row) return null
  const manualScore = Math.max(0, asNumber(row.manualScore, 0))
  const effectParams = isRecord(row.effectParams) ? row.effectParams : {}
  const attrMods = isRecord(effectParams.attrMods) ? effectParams.attrMods : {}
  let attrBonus = 0
  for (const payload of Object.values(attrMods)) {
    if (!isRecord(payload)) continue
    const maxAttrs: Record<string, number> = {}
    for (const [attr, range] of Object.entries(payload)) {
      maxAttrs[attr] = isRecord(range) ? asNumber(range.max, asNumber(range.min, 0)) : asNumber(range, 0)
    }
    attrBonus = Math.max(attrBonus, Math.abs(scoreAttributeMap(maxAttrs)))
  }
  const hooks = asArray(effectParams.battleHooks).length * COMBAT_SCORING_CONFIG.constants.battleHookValue
  const mutation = asArray(effectParams.effectMutations).length * COMBAT_SCORING_CONFIG.constants.effectMutationValue
  const base = String(row.effectType || "").toLowerCase().includes("mutation")
    ? mutation
    : String(row.effectType || "").toLowerCase().includes("battle")
      ? hooks
      : attrBonus
  return withGold(base + manualScore, { manualScore: round(manualScore), attrBonus: round(attrBonus), mutation: round(mutation), battleHooks: round(hooks) })
}

export function scoreAffixInstance(slot: Row | null | undefined): CombatScoreResult | null {
  if (!slot) return null
  const definition = getCombatScoringData().affixByID.get(String(slot.affixId || ""))
  const templateScore = definition ? scoreAffixDefinition(definition) : null
  const attrMods = isRecord(slot.attrMods) ? slot.attrMods : {}
  const attrBonus = Math.abs(scoreAttributeMap(attrMods))
  const hookBudget = isRecord(slot.effectParams) ? asArray(slot.effectParams.battleHooks).length * COMBAT_SCORING_CONFIG.constants.battleHookValue : 0
  const manualScore = templateScore?.detail.manualScore ?? 0
  const specialScore = Math.max(0, (templateScore?.score ?? 0) - (templateScore?.detail.attrBonus ?? 0))
  return withGold((attrBonus + hookBudget + manualScore + specialScore) * COMBAT_SCORING_CONFIG.constants.affixInstanceScoreMultiplier, {
    attrBonus: round(attrBonus),
    manualScore: round(manualScore),
    specialScore: round(specialScore),
    battleHooks: round(hookBudget)
  })
}

export function scoreEquipmentInstance(
  definition: Row | string | null | undefined,
  options: {
    snapshotAttrs?: Record<string, unknown> | null
    enhanceAttrs?: Record<string, unknown> | null
    affixSlots?: Array<Row> | null
  } = {}
): CombatScoreResult | null {
  const row = typeof definition === "string" ? getCombatScoringData().equipmentByID.get(definition) : definition
  if (!row) return null
  const templateScore = scoreEquipmentDefinition(row)
  const snapshotAttrs = Math.abs(scoreAttributeMap(options.snapshotAttrs || {}))
  const enhanceAttrs = Math.abs(scoreAttributeMap(options.enhanceAttrs || {}))
  const affixInstances = (options.affixSlots || []).reduce((sum, slot) => sum + (scoreAffixInstance(slot)?.score || 0), 0)
  const attrScore = snapshotAttrs > 0 ? snapshotAttrs : (templateScore?.detail.baseAttrs || 0) + enhanceAttrs
  const structureScore =
    (templateScore?.detail.basicAttack || 0) +
    (templateScore?.detail.battleHooks || 0) +
    (templateScore?.detail.affixSlots || 0)
  return withGold(attrScore + structureScore + affixInstances, {
    templateScore: round(templateScore?.score || 0),
    snapshotAttrs: round(snapshotAttrs),
    enhanceAttrs: round(enhanceAttrs),
    affixInstances: round(affixInstances),
    basicAttack: round(templateScore?.detail.basicAttack || 0),
    targetingUtility: round(templateScore?.detail.targetingUtility || 0),
    battleHooks: round(templateScore?.detail.battleHooks || 0),
    affixCapacity: round(templateScore?.detail.affixSlots || 0)
  })
}

export function scoreEnemyDefinition(enemyOrID: string | Row | null | undefined): CombatScoreResult | null {
  const data = getCombatScoringData()
  const row = typeof enemyOrID === "string" ? data.enemyByID.get(enemyOrID) : enemyOrID
  if (!row) return null
  const attrs: Record<string, number> = {}
  for (const attrID of data.attributeIDs) {
    const value = asNumber(row[attrID], Number.NaN)
    if (Number.isFinite(value)) attrs[attrID] = value
  }
  const attrBudget = Math.abs(scoreAttributeMap(attrs))
  const levelFloor = levelBudget(row.level) * 0.18
  const skillBudget = asArray(row.skillIds).reduce<number>((sum, skillID) => sum + (scoreSkillDefinition(String(skillID || ""))?.score || 12) * COMBAT_SCORING_CONFIG.constants.enemySkillScoreMultiplier, 0)
  const tierFactor = COMBAT_SCORING_CONFIG.enemyTierMultipliers[String(row.type || "normal")] ?? 1
  const rewardFactor = COMBAT_SCORING_CONFIG.enemyRewardMultipliers[String(row.type || "normal")] ?? 1
  const score = (attrBudget + levelFloor + skillBudget) * tierFactor
  return withGold(score, {
    attrBudget: round(attrBudget),
    levelFloor: round(levelFloor),
    skillBudget: round(skillBudget),
    tierFactor: round(tierFactor),
    rewardGold: round(score * COMBAT_SCORING_CONFIG.constants.enemyGoldRewardRate * rewardFactor),
    rewardExp: round(score * COMBAT_SCORING_CONFIG.constants.enemyExpRewardRate * rewardFactor)
  })
}
