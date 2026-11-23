import { hierarchy, tree } from "d3-hierarchy"

export type TreeLayoutNodeInput<TMeta = unknown> = {
  id: string
  children: TreeLayoutNodeInput<TMeta>[]
  meta: TMeta
}

export type TreeLayoutNodeOutput<TMeta = unknown> = {
  id: string
  x: number
  y: number
  meta: TMeta
}

export type TreeLayoutLinkOutput = {
  source: { x: number; y: number }
  target: { x: number; y: number }
}

export type TreeLayoutOptions = {
  stepX: number
  stepY: number
  topOffset: number
  lockRootCenter?: boolean
  minLeftPadding?: number
}

export function buildTreeLayout<TMeta>(
  root: TreeLayoutNodeInput<TMeta>,
  options: TreeLayoutOptions
): {
  nodes: TreeLayoutNodeOutput<TMeta>[]
  links: TreeLayoutLinkOutput[]
  span: { minX: number; maxX: number; maxY: number }
} {
  const hierarchyRoot = hierarchy<TreeLayoutNodeInput<TMeta>>(root)
  const treeLayout = tree<TreeLayoutNodeInput<TMeta>>().nodeSize([options.stepX, options.stepY])
  const laidOut = treeLayout(hierarchyRoot)
  const descendants = laidOut.descendants()
  const rawLinks = laidOut.links()

  const minX = Math.min(...descendants.map((node) => node.x))
  const maxX = Math.max(...descendants.map((node) => node.x))
  const maxY = Math.max(...descendants.map((node) => node.y))

  let nodes = descendants.map((node) => ({
    id: node.data.id,
    x: node.x - minX,
    y: node.y + options.topOffset,
    meta: node.data.meta
  }))
  let links = rawLinks.map((link) => ({
    source: { x: link.source.x - minX, y: link.source.y + options.topOffset },
    target: { x: link.target.x - minX, y: link.target.y + options.topOffset }
  }))

  if (options.lockRootCenter) {
    const baseWidth = maxX - minX
    const rootX = nodes[0]?.x || 0
    const centeredShift = baseWidth / 2 - rootX
    const leftPad = options.minLeftPadding ?? 0
    const minAfterCenter = Math.min(...nodes.map((node) => node.x + centeredShift))
    const edgeFix = minAfterCenter < leftPad ? leftPad - minAfterCenter : 0
    const shift = centeredShift + edgeFix
    nodes = nodes.map((node) => ({ ...node, x: node.x + shift }))
    links = links.map((link) => ({
      source: { x: link.source.x + shift, y: link.source.y },
      target: { x: link.target.x + shift, y: link.target.y }
    }))
  }

  const finalMinX = Math.min(...nodes.map((node) => node.x))
  const finalMaxX = Math.max(...nodes.map((node) => node.x))
  const finalMaxY = Math.max(...nodes.map((node) => node.y))

  return {
    nodes,
    links,
    span: {
      minX: finalMinX,
      maxX: finalMaxX,
      maxY: finalMaxY
    }
  }
}
