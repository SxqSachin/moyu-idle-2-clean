import { apiDelete, apiGet, apiPost, apiPut } from "@services/api"

export type SkillForgePortType = "flow" | "effect" | "skill" | "condition"

export type SkillForgeParameterKind =
  | "requirements"
  | "hp_cost"
  | "mp_cost"
  | "cooldown"
  | "conditions"
  | "target_scope"
  | "target_count"
  | "target_count_max"
  | "target_direction"
  | "select_rule"
  | "target_positions"
  | "formula"
  | "hit_count"
  | "status_apply"
  | "status_duration"
  | "effect_chain"

export type SkillForgeModule = {
  id: string
  itemId: string
  nodeKind: string
  typedPorts: {
    inputs: Array<{ id: string; type: SkillForgePortType }>
    outputs: Array<{ id: string; type: SkillForgePortType }>
  }
  compileOp: Record<string, unknown> & { kind: string }
  valuePerUnit: { min: number; max: number }
  unitBasis: number
  maxBasisMultiplier: number
  minAlchemyLevel: number
  failBaseChance: number
  failChancePerLevelAbove: number
  successChanceModifier: number
  budgetCreditPerUnit: number
  nameParts: Record<string, string>
  icon: string
}

export type SkillForgeGraphNode = {
  id: string
  kind: "source" | "output" | "effect" | "parameter" | "material"
  moduleId?: string
  quantity?: number
  effectKind?: "empty" | "damage" | "heal" | "status" | "resource"
  effectIndex?: number
  parameterKind?: SkillForgeParameterKind
  dslPath?: string
  targetNodeId?: string
  position: { x: number; y: number }
}

export type SkillForgeGraphEdge = {
  id: string
  source: string
  sourceHandle: string
  target: string
  targetHandle: string
}

export type SkillForgeGraph = {
  version: 1
  source: {
    kind: "blank" | "official"
    skillId?: string
    definitionHash?: string
  }
  nodes: SkillForgeGraphNode[]
  edges: SkillForgeGraphEdge[]
}

export type SkillForgeDraft = {
  id: string
  customSkillId?: string
  sourceKind: "blank" | "official"
  sourceSkillId?: string
  sourceSkillbookItemId?: string
  sourceSkillbookCount: number
  status: "active" | "completed"
  sourceDefinitionHash?: string
  graph: SkillForgeGraph
  revisionNo: number
  pendingRevisionId?: string
  createdAt: string
  updatedAt: string
}

export type SkillForgeRange = {
  nodeId: string
  targetNodeId?: string
  kind: string
  direction?: "increase" | "reduce"
  baseMin: number
  baseMax: number
  min: number
  max: number
  successChanceModifier: number
  rolled?: number
}

export type SkillForgePreview = {
  valid: boolean
  blockingReasons: string[]
  compiledSkill?: Record<string, unknown>
  ranges: SkillForgeRange[]
  recipe: Record<string, number>
  budgetUsed: number
  budgetLimit: number
  budgetRefund: number
  budgetMultiplier: number
  failChance: number
  baseSuccessChance: number
  materialSuccessModifier: number
  baseFailChance: number
  skillbookBonus: number
  finalFailChance: number
  sourceSkillbookItemId?: string
  sourceSkillbookCount?: number
  goldCost: number
  goldCostBreakdown: { base: number; materials: number; budget: number }
  failed?: boolean
  compilerVersion: number
  rulesetVersion: number
  graphHash?: string
}

export type SkillForgeRevision = {
  id: string
  customSkillId: string
  draftId?: string
  revisionNo: number
  compiledSkill: Record<string, unknown>
  recipe: Record<string, number>
  requestedRecipe: Record<string, number>
  consumedRecipe: Record<string, number>
  refundedRecipe: Record<string, number>
  budget: Record<string, unknown>
  rolls: SkillForgeRange[]
  failed: boolean
  resultStatus: "pending" | "accepted" | "discarded" | "superseded" | "failed"
  compilerVersion: number
  rulesetVersion: number
  createdAt: string
  forgedSkillbook?: {
    id: string
    instanceId: number
    customSkillId: string
    revisionId: string
    sourceSkillId: string
    status: string
    rulesetVersion: number
    usable: boolean
    compiledSkill?: Record<string, unknown>
    budget?: Record<string, unknown>
  }
}

export type PlayerCustomSkill = {
  id: string
  sourceKind: "blank" | "official"
  sourceSkillId?: string
  currentRevisionId?: string
  archived: boolean
  rulesetVersion: number
  usable: boolean
  disabledReason?: string
  compiledSkill?: Record<string, unknown>
  budget?: Record<string, unknown>
}

export type SkillForgeState = {
  modules: SkillForgeModule[]
  limits: {
    compilerVersion: number
    rulesetVersion: number
    disabledRulesetVersions: number[]
    baseSuccessChance: number
    sourceSkillbookMinCount: number
    sourceSkillbookMaxCount: number
    sourceSkillbookCountStep: number
    sourceSkillbookBonusPerStep: number
    sourceSkillbookMaxBonus: number
    maxCustomSkills: number
    maxDrafts: number
    maxPendingResultsPerDraft: number
    recentHistoryLimit: number
    idempotencyRetentionDays: number
    maxNodes: number
    maxEdges: number
    maxEffects: number
    maxFormulaTermsPerEffect: number
    maxTargetCount: number
    maxHitCount: number
    minCooldown: number
    maxCooldown: number
    maxGraphBytes: number
    maxTradeoffRefundRatio: number
    statusBaseApplyChance: number
    statusApplyChancePerStep: number
    customizedNodeSealItemId: string
    customizedNodeSealCount: number
    forgeCatalystItemId: string
    forgeCatalystCount: number
    goldCostItemId: string
    goldCostBase: number
    goldCostPerMaterial: number
    goldCostPerBudget: number
  }
  derived: { level: number; precision: number; stability: number }
  inventory: Array<{ itemId: string; quantity: number }>
  drafts: SkillForgeDraft[]
  skills: PlayerCustomSkill[]
  pendingRevisions: SkillForgeRevision[]
  ownedSourceIds: string[]
  sourceSkillbooks: Array<{ itemId: string; skillId: string; rarity: string; iconId: string; quantity: number }>
}

export function normalizeSkillForgeMaterialQuantity(
  quantity: number,
  module: Pick<SkillForgeModule, "unitBasis" | "maxBasisMultiplier">,
  rarity = ""
): number {
  const current = Math.max(0, Math.floor(Number(quantity) || 0))
  const maximum = module.unitBasis * module.maxBasisMultiplier
  if (current >= module.unitBasis && current <= maximum && current % module.unitBasis === 0) return current

  // Preserve batch counts across the corrected legendary/mythic balance rollout.
  const legacyBasis = rarity === "mythic"
    ? module.unitBasis === 15 ? 50 : module.unitBasis === 75 ? 250 : 0
    : rarity === "legendary"
      ? module.unitBasis === 200 ? 15 : module.unitBasis === 1000 ? 75 : 0
      : 0
  if (legacyBasis > 0 && current >= legacyBasis && current % legacyBasis === 0) {
    const migrated = (current / legacyBasis) * module.unitBasis
    return Math.min(maximum, Math.max(module.unitBasis, migrated))
  }

  return Math.min(maximum, Math.max(module.unitBasis, Math.ceil(current / module.unitBasis) * module.unitBasis))
}

export function createBlankSkillForgeGraph(
  sourceKind: SkillForgeGraph["source"]["kind"] = "blank",
  skillId = "",
  sourceEffectKinds: string[] = []
): SkillForgeGraph {
  const effectKinds = sourceEffectKinds.length > 0 ? sourceEffectKinds : ["empty"]
  if (sourceKind === "blank") {
    return createSkillForgeDslGraph(
      "blank",
      "",
      { effects: effectKinds.map((kind) => ({ kind })) },
      true
    )
  }
  const effectNodes: SkillForgeGraphNode[] = effectKinds.map((kind, index) => ({
    id: `effect-${index + 1}`,
    kind: "effect",
    effectKind: kind === "heal" || kind === "status" || kind === "resource" ? kind : kind === "damage" ? "damage" : "empty",
    effectIndex: index,
    position: { x: 380, y: 80 + index * 220 }
  }))
  const effectEdges: SkillForgeGraphEdge[] = effectNodes.flatMap((node, index) => [
    { id: `edge-source-effect-${index + 1}`, source: "source", sourceHandle: "effect", target: node.id, targetHandle: "in" },
    { id: `edge-effect-output-${index + 1}`, source: node.id, sourceHandle: "out", target: "output", targetHandle: "effect" }
  ])
  return {
    version: 1,
    source: { kind: sourceKind, ...(skillId ? { skillId } : {}) },
    nodes: [
      { id: "source", kind: "source", position: { x: 40, y: 240 } },
      ...effectNodes,
      { id: "output", kind: "output", position: { x: 980, y: 240 } }
    ],
    edges: effectEdges
  }
}

type SkillDefinitionRecord = Record<string, unknown>

export function createOfficialSkillForgeGraph(skillId: string, definition: SkillDefinitionRecord): SkillForgeGraph {
  return createSkillForgeDslGraph("official", skillId, definition, false)
}

function createSkillForgeDslGraph(
  sourceKind: SkillForgeGraph["source"]["kind"],
  skillId: string,
  definition: SkillDefinitionRecord,
  genericEffectParameters: boolean
): SkillForgeGraph {
  const rawEffects = Array.isArray(definition.effects) ? definition.effects : []
  const effects = rawEffects
    .map((effect) => effect && typeof effect === "object" && !Array.isArray(effect) ? effect as SkillDefinitionRecord : {})
    .slice(0, 3)
  const nodes: SkillForgeGraphNode[] = [{ id: "source", kind: "source", position: { x: 20, y: 32 } }]
  const edges: SkillForgeGraphEdge[] = []
  const columnGap = 304
  let longestBranch = 0

  const skillParameters: Array<{ id: string; parameterKind: SkillForgeParameterKind; dslPath: string }> = [
    { id: "skill-requirements", parameterKind: "requirements", dslPath: "skill.requirements" },
    { id: "skill-hp-cost", parameterKind: "hp_cost", dslPath: "skill.hpCost" },
    { id: "skill-mp-cost", parameterKind: "mp_cost", dslPath: "skill.mpCost" },
    { id: "skill-cooldown", parameterKind: "cooldown", dslPath: "skill.cooldown" }
  ]
  let previous = "source"
  skillParameters.forEach((parameter, index) => {
    nodes.push({ ...parameter, kind: "parameter", position: { x: 300 + index * columnGap, y: 32 } })
    edges.push({
      id: `edge-skill-${index + 1}`,
      source: previous,
      sourceHandle: previous === "source" ? "skill" : "out",
      target: parameter.id,
      targetHandle: "in"
    })
    previous = parameter.id
  })
  longestBranch = Math.max(longestBranch, skillParameters.length)

  effects.forEach((effect, effectIndex) => {
    const kind = normalizeEffectKind(String(effect.kind || "damage"))
    const row = effectIndex + 1
    const parameters: Array<{ id: string; parameterKind: SkillForgeParameterKind; dslPath: string }> = [
      { id: `effect-${row}-conditions`, parameterKind: "conditions", dslPath: `effects.${effectIndex}.conditions` },
      { id: `effect-${row}-target-scope`, parameterKind: "target_scope", dslPath: `effects.${effectIndex}.targeting.targetScope` },
      { id: `effect-${row}-target-positions`, parameterKind: "target_positions", dslPath: `effects.${effectIndex}.targeting.targetPositions` },
      { id: `effect-${row}-target-direction`, parameterKind: "target_direction", dslPath: `effects.${effectIndex}.targeting.secondarySeek` },
      { id: `effect-${row}-target-count`, parameterKind: "target_count", dslPath: `effects.${effectIndex}.targeting.targetCount.base` },
      { id: `effect-${row}-target-count-max`, parameterKind: "target_count_max", dslPath: `effects.${effectIndex}.targeting.targetCount.max` },
      { id: `effect-${row}-select-rule`, parameterKind: "select_rule", dslPath: `effects.${effectIndex}.targeting.selectRule` }
    ]
    if (genericEffectParameters || kind === "damage" || kind === "heal" || kind === "resource") {
      parameters.push({ id: `effect-${row}-formula`, parameterKind: "formula", dslPath: `effects.${effectIndex}.amountFormula` })
    }
    if (genericEffectParameters || kind === "damage") {
      parameters.push({ id: `effect-${row}-hit-count`, parameterKind: "hit_count", dslPath: `effects.${effectIndex}.damageSpec.hitCount.base` })
    }
    if (genericEffectParameters || kind === "status") {
      parameters.push(
        { id: `effect-${row}-status-apply`, parameterKind: "status_apply", dslPath: `effects.${effectIndex}.statusPayload.statusIds` },
        { id: `effect-${row}-status-duration`, parameterKind: "status_duration", dslPath: `effects.${effectIndex}.statusPayload.durationBaseTurns` }
      )
    }
    if (effectIndex > 0 || Object.prototype.hasOwnProperty.call(effect, "reuseTargetsFromEffect")) {
      parameters.push({ id: `effect-${row}-chain`, parameterKind: "effect_chain", dslPath: `effects.${effectIndex}.reuseTargetsFromEffect` })
    }

    const y = 220 + effectIndex * 480
    const condition = parameters.shift()!
    nodes.push({ ...condition, kind: "parameter", ...(genericEffectParameters ? {} : { effectKind: kind }), effectIndex, position: { x: 300, y } })
    const effectNodeId = `effect-${row}`
    nodes.push({ id: effectNodeId, kind: "effect", effectKind: kind, effectIndex, position: { x: 300 + columnGap, y } })
    edges.push(
      { id: `edge-source-effect-${row}`, source: "source", sourceHandle: "effect", target: condition.id, targetHandle: "in" },
      { id: `edge-effect-${row}-condition`, source: condition.id, sourceHandle: "out", target: effectNodeId, targetHandle: "in" }
    )
    previous = effectNodeId
    parameters.forEach((parameter, index) => {
      nodes.push({ ...parameter, kind: "parameter", ...(genericEffectParameters ? {} : { effectKind: kind }), effectIndex, position: { x: 300 + (index + 2) * columnGap, y } })
      edges.push({
        id: `edge-effect-${row}-dsl-${index + 1}`,
        source: previous,
        sourceHandle: "out",
        target: parameter.id,
        targetHandle: "in"
      })
      previous = parameter.id
    })
    longestBranch = Math.max(longestBranch, parameters.length + 2)
  })

  const outputX = 300 + (longestBranch + 1) * columnGap
  nodes.push({ id: "output", kind: "output", position: { x: outputX, y: 32 } })
  edges.push({ id: "edge-skill-output", source: skillParameters[skillParameters.length - 1]!.id, sourceHandle: "out", target: "output", targetHandle: "skill" })
  effects.forEach((_, effectIndex) => {
    const row = effectIndex + 1
    const branchNodes = nodes
      .filter((node) => node.effectIndex === effectIndex && (node.kind === "effect" || node.kind === "parameter"))
      .sort((a, b) => a.position.x - b.position.x)
    const terminal = branchNodes[branchNodes.length - 1]
    if (terminal) edges.push({ id: `edge-effect-${row}-output`, source: terminal.id, sourceHandle: "out", target: "output", targetHandle: "effect" })
  })

  return {
    version: 1,
    source: { kind: sourceKind, ...(skillId ? { skillId } : {}) },
    nodes,
    edges
  }
}

function normalizeEffectKind(kind: string): "empty" | "damage" | "heal" | "status" | "resource" {
  return kind === "damage" || kind === "heal" || kind === "status" || kind === "resource" ? kind : "empty"
}

export const fetchSkillForgeState = (token: string) => apiGet<SkillForgeState>("/api/alchemy/skill-forge/state", token)

export const createSkillForgeDraft = (token: string, sourceKind: SkillForgeGraph["source"]["kind"], sourceSkillbookItemId: string, sourceSkillbookCount: number, graph: SkillForgeGraph) =>
  apiPost<SkillForgeDraft>("/api/alchemy/skill-forge/drafts", token, { sourceKind, sourceSkillbookItemId, sourceSkillbookCount, graph })

export const updateSkillForgeDraft = (token: string, draftId: string, expectedRevision: number, sourceSkillbookCount: number, graph: SkillForgeGraph) =>
  apiPut<SkillForgeDraft>(`/api/alchemy/skill-forge/drafts/${encodeURIComponent(draftId)}`, token, { expectedRevision, sourceSkillbookCount, graph })

export const deleteSkillForgeDraft = (token: string, draftId: string) =>
  apiDelete<{ ok: boolean }>(`/api/alchemy/skill-forge/drafts/${encodeURIComponent(draftId)}`, token)

export const previewSkillForgeDraft = (token: string, draftId: string) =>
  apiPost<SkillForgePreview>(`/api/alchemy/skill-forge/drafts/${encodeURIComponent(draftId)}/preview`, token)

export const rebaseSkillForgeDraft = (token: string, draftId: string) =>
  apiPost<SkillForgeDraft>(`/api/alchemy/skill-forge/drafts/${encodeURIComponent(draftId)}/rebase`, token)

export const forgeSkillDraft = (token: string, draftId: string, expectedRevision: number, expectedGraphHash: string, idempotencyKey: string) =>
  apiPost<SkillForgeRevision>(
    `/api/alchemy/skill-forge/drafts/${encodeURIComponent(draftId)}/forge`,
    token,
    { expectedRevision, expectedGraphHash },
    { "X-Idempotency-Key": idempotencyKey }
  )

export const acceptSkillForgeRevision = (token: string, revisionId: string) =>
  apiPost<SkillForgeRevision>(`/api/alchemy/skill-forge/results/${encodeURIComponent(revisionId)}/accept`, token)

export const discardSkillForgeRevision = (token: string, revisionId: string) =>
  apiPost<SkillForgeRevision>(`/api/alchemy/skill-forge/results/${encodeURIComponent(revisionId)}/discard`, token)

export const archiveCustomSkill = (token: string, skillId: string) =>
  apiPost<{ ok: boolean }>(`/api/alchemy/skill-forge/skills/${encodeURIComponent(skillId)}/archive`, token)
