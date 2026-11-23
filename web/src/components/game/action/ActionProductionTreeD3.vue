<template>
  <div class="w-full overflow-auto rounded-lg border border-[var(--app-border)] bg-[var(--app-surface)] p-2">
    <div class="relative" :style="{ width: `${svgWidth}px`, height: `${svgHeight}px` }">
      <svg :width="svgWidth" :height="svgHeight" role="img" aria-label="production-plan-tree">
        <g :transform="`translate(${paddingX}, ${paddingY})`">
          <path
            v-for="(link, index) in links"
            :key="`link-${index}`"
            :d="linkPath(link)"
            fill="none"
            stroke="color-mix(in srgb, var(--app-border) 80%, transparent)"
            stroke-width="1.4"
          />
        </g>
      </svg>

      <div
        v-for="node in nodes"
        :key="`card-${node.id}`"
        class="absolute pointer-events-auto"
        :data-root-node="node.isRoot ? 'true' : undefined"
        :style="{
          left: `${node.x + paddingX}px`,
          top: `${node.y + paddingY}px`,
          transform: 'translate(-50%, -50%)'
        }"
      >
        <div class="inline-flex flex-col rounded border border-[var(--app-border)] bg-[var(--app-surface)] px-2 py-1 shadow-sm min-w-[138px] max-w-[260px]">
          <div class="flex items-center gap-2 flex-wrap">
            <ActionQuickRef v-if="node.type === 'action'" :action-id="node.actionId || ''" />
            <QuickRefWithRequire
              v-else
              :item-id="node.itemId || ''"
              :required-count="node.requiredCount || 0"
            />
            <span v-if="node.count && node.type === 'action'" class="text-xs font-semibold">x{{ node.count }}</span>
          </div>
          <div class="text-[11px] app-muted mt-1 whitespace-nowrap">{{ node.subtitle }}</div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from "vue"
import { useI18n } from "vue-i18n"
import { formatDurationI18n } from "@/utils/duration-format"
import ActionQuickRef from "@components/game/action/ActionQuickRef.vue"
import QuickRefWithRequire from "@components/game/item/QuickRefWithRequire.vue"
import type { ProductionActionNode } from "@/services/action-production-plan"
import { buildTreeLayout, type TreeLayoutLinkOutput, type TreeLayoutNodeInput } from "@/services/tree-layout"

type NodeMeta = {
  type: "action" | "material"
  subtitle: string
  actionId?: string
  itemId?: string
  count?: number
  requiredCount?: number
}

type PositionedNode = {
  id: string
  isRoot: boolean
  x: number
  y: number
  type: "action" | "material"
  subtitle: string
  actionId?: string
  itemId?: string
  count?: number
  requiredCount?: number
}

const props = defineProps<{
  root: ProductionActionNode | null
}>()

const { t } = useI18n()
const spacingX = 122
const spacingY = 78
const approxHalfNodeWidth = 120
const topOffset = 18
const paddingX = 18
const paddingY = 16

function toLayoutNode(node: ProductionActionNode): TreeLayoutNodeInput<NodeMeta> {
  const children: Array<TreeLayoutNodeInput<NodeMeta>> = []
  for (const material of node.costs) {
    const materialNode: TreeLayoutNodeInput<NodeMeta> = {
      id: material.nodeId,
      meta: {
        type: "material",
        itemId: material.itemId,
        requiredCount: material.required,
        subtitle: t("game.actionQuickArrangeMaterialNodeSubtitle", {
          required: material.required,
          deficit: material.deficit
        })
      },
      children: []
    }
    if (material.childAction) {
      materialNode.children.push(toLayoutNode(material.childAction))
    }
    children.push(materialNode)
  }
  return {
    id: node.nodeId,
    meta: {
      type: "action",
      actionId: node.actionId,
      count: node.count,
      subtitle: t("game.actionQuickArrangeActionNodeSubtitle", {
        single: formatDurationI18n(node.durationS, t),
        total: formatDurationI18n(node.totalDurationS, t)
      })
    },
    children
  }
}

const layoutResult = computed(() => {
  if (!props.root) {
    return {
      nodes: [] as PositionedNode[],
      links: [] as TreeLayoutLinkOutput[],
      width: 720,
      height: 320
    }
  }
  const laidOut = buildTreeLayout<NodeMeta>(toLayoutNode(props.root), {
    stepX: spacingX,
    stepY: spacingY,
    topOffset,
    lockRootCenter: true,
    minLeftPadding: approxHalfNodeWidth
  })
  const nodes: PositionedNode[] = laidOut.nodes.map((node) => ({
    id: node.id,
    isRoot: node.id === props.root?.nodeId,
    x: node.x,
    y: node.y,
    type: node.meta.type,
    subtitle: node.meta.subtitle,
    actionId: node.meta.actionId,
    itemId: node.meta.itemId,
    count: node.meta.count,
    requiredCount: node.meta.requiredCount
  }))
  const width = Math.max(720, laidOut.span.maxX + approxHalfNodeWidth + 24)
  const height = Math.max(300, laidOut.span.maxY + 68)
  return {
    nodes,
    links: laidOut.links,
    width,
    height
  }
})

const nodes = computed(() => layoutResult.value.nodes)
const links = computed(() => layoutResult.value.links)
const svgWidth = computed(() => layoutResult.value.width + paddingX * 2)
const svgHeight = computed(() => layoutResult.value.height + paddingY * 2)

function linkPath(link: TreeLayoutLinkOutput): string {
  const midY = (link.source.y + link.target.y) / 2
  return `M ${link.source.x} ${link.source.y} C ${link.source.x} ${midY}, ${link.target.x} ${midY}, ${link.target.x} ${link.target.y}`
}
</script>
