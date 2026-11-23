import type { SpecializationNode } from "@services/specialization"

export function specializationNodePrerequisiteIds(node: SpecializationNode): string[] {
  const ids = [
    ...(node.requirements?.all || []),
    ...(node.requirements?.anyOf || []).flat()
  ]
    .filter((requirement) => requirement.kind === "node_rank")
    .map((requirement) => String(requirement.nodeId || "").trim())
    .filter(Boolean)
  return Array.from(new Set(ids))
}

/**
 * Returns the graph's visible cover edges rather than every redundant guard.
 * Runtime validation may repeat a root prerequisite on a terminal node even
 * when an intermediate prerequisite already implies that root. Drawing both
 * edges makes one route look like two separate paths.
 */
export function minimalSpecializationPrerequisiteIds(
  nodes: readonly SpecializationNode[],
  node: SpecializationNode
): string[] {
  const byId = new Map(nodes.map((candidate) => [candidate.id, candidate]))
  const direct = specializationNodePrerequisiteIds(node).filter((id) => byId.has(id))
  if (direct.length < 2) return direct
  const mandatoryIds = (node.requirements?.all || [])
    .filter((requirement) => requirement.kind === "node_rank")
    .map((requirement) => String(requirement.nodeId || "").trim())
    .filter(Boolean)
  const alternativeGroups = (node.requirements?.anyOf || []).map((group) => group
    .filter((requirement) => requirement.kind === "node_rank")
    .map((requirement) => String(requirement.nodeId || "").trim())
    .filter(Boolean))
  const alternativeIds = new Set(alternativeGroups.flat())

  const ancestorCache = new Map<string, Set<string>>()
  const ancestorsOf = (nodeId: string, visiting = new Set<string>()): Set<string> => {
    const cached = ancestorCache.get(nodeId)
    if (cached) return cached
    if (visiting.has(nodeId)) return new Set()
    const candidate = byId.get(nodeId)
    if (!candidate) return new Set()
    const nextVisiting = new Set(visiting).add(nodeId)
    const ancestors = new Set<string>()
    for (const prerequisiteId of specializationNodePrerequisiteIds(candidate)) {
      if (!byId.has(prerequisiteId)) continue
      ancestors.add(prerequisiteId)
      for (const ancestorId of ancestorsOf(prerequisiteId, nextVisiting)) ancestors.add(ancestorId)
    }
    ancestorCache.set(nodeId, ancestors)
    return ancestors
  }

  return direct.filter((prerequisiteId) => {
    // An alternative is a real player choice, so never erase its visible edge.
    if (alternativeIds.has(prerequisiteId)) return true
    if (!mandatoryIds.includes(prerequisiteId)) return true
    const impliedByMandatory = mandatoryIds.some((otherId) =>
      otherId !== prerequisiteId && ancestorsOf(otherId).has(prerequisiteId)
    )
    const impliedByEveryAlternativeInAGroup = alternativeGroups.some((group) =>
      group.length > 0 && group.every((otherId) => ancestorsOf(otherId).has(prerequisiteId))
    )
    return !impliedByMandatory && !impliedByEveryAlternativeInAGroup
  })
}

export function collectSpecializationAncestorIds(
  nodes: readonly SpecializationNode[],
  selectedNodeId: string
): Set<string> {
  const byId = new Map(nodes.map((node) => [node.id, node]))
  const ancestors = new Set<string>()
  const visiting = new Set<string>()

  const visit = (nodeId: string) => {
    if (visiting.has(nodeId)) return
    const node = byId.get(nodeId)
    if (!node) return
    visiting.add(nodeId)
    for (const prerequisiteId of specializationNodePrerequisiteIds(node)) {
      if (!byId.has(prerequisiteId)) continue
      ancestors.add(prerequisiteId)
      visit(prerequisiteId)
    }
    visiting.delete(nodeId)
  }

  visit(selectedNodeId)
  ancestors.delete(selectedNodeId)
  return ancestors
}

export function buildSpecializationDepthMap(
  nodes: readonly SpecializationNode[]
): Map<string, number> {
  const byId = new Map(nodes.map((node) => [node.id, node]))
  const depths = new Map<string, number>()
  const visiting = new Set<string>()

  const depthOf = (nodeId: string): number => {
    const cached = depths.get(nodeId)
    if (cached !== undefined) return cached
    if (visiting.has(nodeId)) return 0
    const node = byId.get(nodeId)
    if (!node) return 0
    visiting.add(nodeId)
    const prerequisiteDepths = specializationNodePrerequisiteIds(node)
      .filter((prerequisiteId) => byId.has(prerequisiteId))
      .map((prerequisiteId) => depthOf(prerequisiteId) + 1)
    visiting.delete(nodeId)
    const depth = prerequisiteDepths.length > 0 ? Math.max(...prerequisiteDepths) : 0
    depths.set(nodeId, depth)
    return depth
  }

  for (const node of nodes) depthOf(node.id)
  return depths
}
