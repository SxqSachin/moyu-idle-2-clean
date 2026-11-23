import type { ActionDefinition, ActionItemRewardRule } from "@game-data/game-data"
import { resolveActionDurationSeconds } from "@/services/action-duration"
import { isGenericActionVisible } from "@/services/action-definition"

export type ProductionRecipeOption = {
  actionId: string
  nameKey: string
  outputPerRun: number
  durationS: number
}

export type ProductionProbabilityHint = {
  itemId: string
  percent: number
  countHint: number
}

export type ProductionMaterialNode = {
  kind: "material"
  nodeId: string
  itemId: string
  required: number
  consumedFromInventory: number
  deficit: number
  options: ProductionRecipeOption[]
  selectedActionId?: string
  unresolved: boolean
  cycleDetected: boolean
  childAction?: ProductionActionNode
}

export type ProductionActionNode = {
  kind: "action"
  nodeId: string
  actionId: string
  nameKey: string
  count: number
  durationS: number
  totalDurationS: number
  costs: ProductionMaterialNode[]
  probabilityHints: ProductionProbabilityHint[]
}

export type ProductionPlanActionStep = {
  actionId: string
  nameKey: string
  count: number
  durationS: number
  totalDurationS: number
}

export type ProductionPlan = {
  root: ProductionActionNode
  actionSteps: ProductionPlanActionStep[]
  unresolvedMaterials: Array<{ itemId: string; deficit: number }>
  totalDurationS: number
  hasCycle: boolean
}

export type ProductionPlanInput = {
  rootAction: ActionDefinition
  rootCount: number
  actions: ActionDefinition[]
  inventoryByItemId: Record<string, number>
  selectedRecipeByNode?: Record<string, string>
  durationContext?: {
    actorType?: "player" | "kitty"
    attributes?: Record<string, number>
    additionalEfficiency?: (mainType: string, subType: string, actionID: string) => number
  }
}

export type ProductionPlanRow =
  | {
      kind: "action"
      nodeId: string
      depth: number
      actionId: string
      nameKey: string
      count: number
      durationS: number
      totalDurationS: number
      probabilityHints: ProductionProbabilityHint[]
    }
  | {
      kind: "material"
      nodeId: string
      depth: number
      itemId: string
      required: number
      consumedFromInventory: number
      deficit: number
      options: ProductionRecipeOption[]
      selectedActionId?: string
      unresolved: boolean
      cycleDetected: boolean
    }

function normalizePositiveInt(value: number): number {
  if (!Number.isFinite(value)) return 0
  return Math.max(0, Math.floor(value))
}

function resolveGuaranteedReward(raw: number | ActionItemRewardRule | undefined): number {
  if (typeof raw === "number") {
    return normalizePositiveInt(raw)
  }
  if (!raw || typeof raw !== "object") return 0
  const percent = typeof raw.percent === "number" ? raw.percent : 1
  if (!Number.isFinite(percent) || percent < 1) return 0
  if (raw.range && Number.isFinite(raw.range.min) && Number.isFinite(raw.range.max)) {
    return normalizePositiveInt(raw.range.min)
  }
  if (typeof raw.count === "number") {
    return normalizePositiveInt(raw.count)
  }
  return 0
}

function resolveProbabilityHints(action: ActionDefinition): ProductionProbabilityHint[] {
  const out: ProductionProbabilityHint[] = []
  for (const [itemId, raw] of Object.entries(action.itemReward || {})) {
    if (typeof raw === "number") continue
    if (!raw || typeof raw !== "object" || typeof raw.percent !== "number") continue
    if (!Number.isFinite(raw.percent) || raw.percent >= 1 || raw.percent <= 0) continue
    const countHint = typeof raw.count === "number"
      ? normalizePositiveInt(raw.count)
      : raw.range && Number.isFinite(raw.range.min)
        ? normalizePositiveInt(raw.range.min)
        : 1
    out.push({
      itemId,
      percent: Math.max(0, Math.min(1, raw.percent)),
      countHint: Math.max(1, countHint)
    })
  }
  return out
}

function buildProducerMap(
  actions: ActionDefinition[],
  durationContext: NonNullable<ProductionPlanInput["durationContext"]>
): Map<string, ProductionRecipeOption[]> {
  const out = new Map<string, ProductionRecipeOption[]>()
  for (const action of actions) {
    if (!isGenericActionVisible(action)) continue
    for (const [itemId, reward] of Object.entries(action.itemReward || {})) {
      const outputPerRun = resolveGuaranteedReward(reward)
      if (outputPerRun <= 0) continue
      const durationS = resolveActionDurationSeconds(
        Number(action.durationS || 1),
        action.mainType,
        action.subType,
        action.id,
        durationContext.actorType || "player",
        durationContext.attributes || {},
        durationContext.additionalEfficiency?.(action.mainType || '', action.subType || '', action.id) || 0
      )
      const list = out.get(itemId) || []
      list.push({
        actionId: action.id,
        nameKey: action.nameKey || `action.${action.id}.name`,
        outputPerRun,
        durationS
      })
      out.set(itemId, list)
    }
  }
  for (const [itemId, options] of out.entries()) {
    options.sort((a, b) => {
      const leftCost = a.durationS / Math.max(1, a.outputPerRun)
      const rightCost = b.durationS / Math.max(1, b.outputPerRun)
      if (leftCost !== rightCost) return leftCost - rightCost
      if (a.durationS !== b.durationS) return a.durationS - b.durationS
      return a.actionId.localeCompare(b.actionId)
    })
    out.set(itemId, options)
  }
  return out
}

export function buildProductionPlan(input: ProductionPlanInput): ProductionPlan {
  const rootCount = Math.max(1, normalizePositiveInt(input.rootCount || 1))
  const durationContext = input.durationContext || {}
  const inventory = new Map<string, number>()
  for (const [itemId, value] of Object.entries(input.inventoryByItemId || {})) {
    const count = normalizePositiveInt(Number(value || 0))
    if (count > 0) inventory.set(itemId, count)
  }
  const actionById = new Map<string, ActionDefinition>(input.actions.map((row) => [row.id, row]))
  const producerMap = buildProducerMap(input.actions, durationContext)
  const selectedRecipeByNode = input.selectedRecipeByNode || {}
  let hasCycle = false

  function buildActionNode(action: ActionDefinition, count: number, path: string, actionPath: Set<string>): ProductionActionNode {
    const nodeId = `${path}/action:${action.id}`
    const nextActionPath = new Set(actionPath)
    nextActionPath.add(action.id)
    const durationS = resolveActionDurationSeconds(
      Number(action.durationS || 1),
      action.mainType,
      action.subType,
      action.id,
      durationContext.actorType || "player",
      durationContext.attributes || {},
      durationContext.additionalEfficiency?.(action.mainType || '', action.subType || '', action.id) || 0
    )
    const costs: ProductionMaterialNode[] = []

    for (const [itemId, rawRequired] of Object.entries(action.costs || {})) {
      const required = normalizePositiveInt(Number(rawRequired || 0)) * count
      const materialNodeId = `${nodeId}/material:${itemId}`
      if (required <= 0) {
        costs.push({
          kind: "material",
          nodeId: materialNodeId,
          itemId,
          required,
          consumedFromInventory: 0,
          deficit: 0,
          options: producerMap.get(itemId) || [],
          unresolved: false,
          cycleDetected: false
        })
        continue
      }
      const owned = inventory.get(itemId) || 0
      const consumedFromInventory = Math.min(owned, required)
      if (consumedFromInventory > 0) {
        inventory.set(itemId, owned - consumedFromInventory)
      }
      const deficit = Math.max(0, required - consumedFromInventory)
      const options = producerMap.get(itemId) || []
      const selectedActionId = selectedRecipeByNode[materialNodeId] || options[0]?.actionId
      const selectedOption = options.find((row) => row.actionId === selectedActionId) || options[0]

      let childAction: ProductionActionNode | undefined
      let unresolved = false
      let cycleDetected = false
      if (deficit > 0) {
        if (!selectedOption) {
          unresolved = true
        } else if (nextActionPath.has(selectedOption.actionId)) {
          unresolved = true
          cycleDetected = true
          hasCycle = true
        } else {
          const producerAction = actionById.get(selectedOption.actionId)
          if (!producerAction) {
            unresolved = true
          } else {
            const runs = Math.max(1, Math.ceil(deficit / Math.max(1, selectedOption.outputPerRun)))
            childAction = buildActionNode(producerAction, runs, materialNodeId, nextActionPath)
          }
        }
      }

      costs.push({
        kind: "material",
        nodeId: materialNodeId,
        itemId,
        required,
        consumedFromInventory,
        deficit,
        options,
        selectedActionId: selectedOption?.actionId,
        unresolved,
        cycleDetected,
        childAction
      })
    }

    return {
      kind: "action",
      nodeId,
      actionId: action.id,
      nameKey: action.nameKey || `action.${action.id}.name`,
      count,
      durationS,
      totalDurationS: durationS * count,
      costs,
      probabilityHints: resolveProbabilityHints(action)
    }
  }

  const root = buildActionNode(input.rootAction, rootCount, "root", new Set())
  const totals = new Map<string, ProductionPlanActionStep>()
  const order: string[] = []
  const unresolvedMap = new Map<string, { itemId: string; deficit: number }>()

  function collect(node: ProductionActionNode) {
    for (const material of node.costs) {
      if (material.unresolved && material.deficit > 0) {
        const current = unresolvedMap.get(material.itemId)
        if (!current) {
          unresolvedMap.set(material.itemId, { itemId: material.itemId, deficit: material.deficit })
        } else {
          current.deficit += material.deficit
        }
      }
      if (material.childAction) {
        collect(material.childAction)
      }
    }
    const existing = totals.get(node.actionId)
    if (!existing) {
      totals.set(node.actionId, {
        actionId: node.actionId,
        nameKey: node.nameKey,
        count: node.count,
        durationS: node.durationS,
        totalDurationS: node.totalDurationS
      })
      order.push(node.actionId)
      return
    }
    existing.count += node.count
    existing.totalDurationS += node.totalDurationS
  }

  collect(root)
  const actionSteps = order.map((actionId) => totals.get(actionId)!).filter((row) => row.count > 0)
  const totalDurationS = actionSteps.reduce((sum, row) => sum + row.totalDurationS, 0)

  return {
    root,
    actionSteps,
    unresolvedMaterials: Array.from(unresolvedMap.values()),
    totalDurationS,
    hasCycle
  }
}

export function flattenProductionPlan(root: ProductionActionNode): ProductionPlanRow[] {
  const rows: ProductionPlanRow[] = []
  function walkAction(action: ProductionActionNode, depth: number) {
    rows.push({
      kind: "action",
      nodeId: action.nodeId,
      depth,
      actionId: action.actionId,
      nameKey: action.nameKey,
      count: action.count,
      durationS: action.durationS,
      totalDurationS: action.totalDurationS,
      probabilityHints: action.probabilityHints
    })
    for (const material of action.costs) {
      rows.push({
        kind: "material",
        nodeId: material.nodeId,
        depth: depth + 1,
        itemId: material.itemId,
        required: material.required,
        consumedFromInventory: material.consumedFromInventory,
        deficit: material.deficit,
        options: material.options,
        selectedActionId: material.selectedActionId,
        unresolved: material.unresolved,
        cycleDetected: material.cycleDetected
      })
      if (material.childAction) {
        walkAction(material.childAction, depth + 2)
      }
    }
  }
  walkAction(root, 0)
  return rows
}
