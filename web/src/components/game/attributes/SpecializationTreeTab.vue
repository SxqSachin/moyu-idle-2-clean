<template>
  <section class="specialization-shell" data-tutorial="specialization-panel">
    <div class="specialization-toolbar">
      <div class="specialization-ledger" aria-live="polite">
        <div class="specialization-ledger__balance">
          <strong>{{ status?.availablePoints ?? 0 }}</strong>
          <span>{{ t("game.specialization.points") }}</span>
          <el-popover
            v-if="status"
            :placement="isMobile ? 'bottom' : 'bottom-start'"
            :width="isMobile ? 340 : 600"
            :trigger="isMobile ? 'click' : ['hover', 'focus']"
            :show-after="100"
            :hide-after="80"
            :persistent="false"
            popper-class="specialization-point-popper"
          >
            <template #reference>
              <el-button
                class="specialization-ledger__help"
                text
                circle
                :icon="QuestionFilled"
                :aria-label="pointSourceHeadline"
                data-testid="specialization-point-guide-trigger"
              />
            </template>
            <div class="specialization-point-guide" data-testid="specialization-point-guide">
              <div class="specialization-point-guide__level">
                <span>{{ t("game.specialization.battleLevel") }}</span>
                <strong>{{ status.battleLevel }}</strong>
              </div>
              <div class="specialization-point-guide__source">
                <strong>{{ pointSourceHeadline }}</strong>
                <ul>
                  <li v-for="(line, index) in pointSourceDetailLines" :key="`${index}-${line}`">
                    {{ line }}
                  </li>
                </ul>
              </div>
              <div class="specialization-point-guide__next">
                <span>{{ t("game.specialization.nextPoint") }}</span>
                <strong>{{ nextPointLabel }}</strong>
              </div>
            </div>
          </el-popover>
        </div>
        <span class="specialization-ledger__spent">
          {{ t("game.specialization.pointsUsed", { spent: status?.spentPoints ?? 0, total: status?.totalPoints ?? 0 }) }}
        </span>
        <span v-if="maxActiveRootNodes > 0" class="specialization-ledger__routes">
          {{ t("game.specialization.activeRootRoutes", { active: activeRootNodeCount, max: maxActiveRootNodes }) }}
        </span>
      </div>
      <div class="specialization-toolbar__actions">
        <el-tooltip :content="t('game.specialization.reset')" placement="top">
          <el-button
            circle
            :icon="RefreshLeft"
            :aria-label="t('game.specialization.reset')"
            :disabled="!status || status.spentPoints <= 0 || specializationBusy || resetting"
            :loading="resetting"
            @click="requestReset"
          />
        </el-tooltip>
        <el-tooltip :content="t('game.refresh')" placement="top">
          <el-button
            circle
            :icon="Refresh"
            :aria-label="t('game.refresh')"
            :disabled="specializationStore.mutating"
            :loading="specializationStore.loading"
            @click="refresh"
          />
        </el-tooltip>
      </div>
    </div>

    <el-alert
      v-if="specializationStore.loadError"
      class="specialization-alert"
      type="error"
      :closable="false"
      :title="t('game.specialization.loadFailed')"
      :description="t('game.specialization.loadFailedHint')"
      show-icon
    />

    <el-alert
      v-else-if="summary?.rulesetReset"
      class="specialization-alert"
      type="warning"
      :closable="false"
      :title="t('game.specialization.rulesetResetTitle')"
      :description="rulesetResetDescription"
      show-icon
    />

    <div v-loading="specializationStore.loading && !summary" class="specialization-workspace">
      <div v-if="summary" class="specialization-canvas-wrap">
        <div class="specialization-school-key" aria-hidden="true">
          <span v-for="school in summary.tree.schools" :key="school.id">
            <i :style="{ background: schoolAccent([school.id]) }" />
            {{ schoolName(school.id) }}
          </span>
          <span class="specialization-school-key__alternative">
            <b />
            {{ t("game.specialization.alternativeRouteLegend") }}
          </span>
        </div>
        <VueFlow
          id="player-specialization-flow"
          :nodes="flowNodes"
          :edges="flowEdges"
          :nodes-draggable="false"
          :nodes-connectable="false"
          :elements-selectable="false"
          :select-nodes-on-drag="false"
          :selection-key-code="null"
          :multi-selection-key-code="null"
          :pan-on-drag="true"
          :min-zoom="0.22"
          :max-zoom="1.6"
          :fit-view-on-init="false"
          :default-edge-options="defaultEdgeOptions"
          class="specialization-flow"
          @nodes-initialized="focusInitialBoard"
          @viewport-change-end="updateTooltipPlacement"
          @node-click="selectNode"
          @pane-click="clearNodeSelection"
        >
          <Background :gap="24" :size="1" pattern-color="var(--app-border)" />
          <MiniMap
            v-if="!isMobile"
            position="bottom-center"
            pannable
            zoomable
            :node-color="miniMapNodeColor"
            mask-color="color-mix(in srgb, var(--app-bg) 78%, transparent)"
          />
          <Controls
            position="bottom-left"
            :fit-view-params="{ padding: 0.12, minZoom: 0.22, maxZoom: 0.58, duration: 220 }"
          />

          <template #node-specialization="{ data, selected }">
            <article
              :ref="(element) => captureNodeElement(data.node.id, element)"
              tabindex="0"
              role="button"
              class="specialization-node"
              :class="[
                `specialization-node--${data.node.kind}`,
                `is-${data.visualState}`,
                `is-route-${data.routeRelation}`,
                { 'is-selected': selected }
              ]"
              :style="{ '--specialization-accent': schoolAccent(data.node.schoolIds) }"
              :data-testid="`specialization-node-${data.node.id}`"
              :aria-label="t('game.specialization.openDetails', { node: localized(data.node.nameKey, data.node.id) })"
              :aria-expanded="tooltipNodeId === data.node.id"
              @mouseenter="showNodeTooltip(data.node.id)"
              @mouseleave="scheduleNodeTooltipHide(data.node.id)"
              @focus="showNodeTooltip(data.node.id)"
              @blur="scheduleNodeTooltipHide(data.node.id)"
              @keydown.enter.stop.prevent="toggleNodeSelection(data.node.id)"
              @keydown.space.stop.prevent="toggleNodeSelection(data.node.id)"
              @keydown.esc.stop.prevent="clearNodeSelection"
            >
              <Handle id="target-top" type="target" :position="Position.Top" class="specialization-handle" />
              <Handle id="target-right" type="target" :position="Position.Right" class="specialization-handle" />
              <Handle id="target-bottom" type="target" :position="Position.Bottom" class="specialization-handle" />
              <Handle id="target-left" type="target" :position="Position.Left" class="specialization-handle" />
              <div class="specialization-node__head">
                <span>
                  {{ data.node.schoolIds.length > 1
                    ? data.node.schoolIds.map(schoolName).join(" × ")
                    : `${schoolName(data.node.schoolIds[0])} · ${nodeKindLabel(data.node.kind)}` }}
                </span>
                <strong>{{ data.state.rank }}/{{ data.state.maxRank }}</strong>
              </div>
              <h3>{{ localized(data.node.nameKey, data.node.id) }}</h3>
              <div class="specialization-node__foot">
                <span>
                  {{ data.state.rank >= data.state.maxRank
                    ? t("game.specialization.maxRankShort")
                    : t("game.specialization.cost", { cost: data.state.nextCost }) }}
                </span>
                <span class="specialization-node__state">{{ visualStateLabel(data.visualState) }}</span>
              </div>
              <Handle id="source-top" type="source" :position="Position.Top" class="specialization-handle" />
              <Handle id="source-right" type="source" :position="Position.Right" class="specialization-handle" />
              <Handle id="source-bottom" type="source" :position="Position.Bottom" class="specialization-handle" />
              <Handle id="source-left" type="source" :position="Position.Left" class="specialization-handle" />
            </article>
          </template>
        </VueFlow>
      </div>
    </div>

    <el-popover
      v-if="tooltipNode && tooltipReference"
      :key="tooltipNode.id"
      :visible="true"
      :virtual-ref="tooltipReference"
      virtual-triggering
      :width="isMobile ? 286 : 410"
      :placement="tooltipPlacement"
      :fallback-placements="['right-start', 'right-end', 'left-start', 'left-end', 'bottom-start', 'bottom-end', 'top-start', 'top-end']"
      :boundaries-padding="isMobile ? 10 : 14"
      effect="light"
      teleported
      :persistent="false"
      popper-class="specialization-node-popper"
    >
      <div
        data-testid="specialization-tooltip-surface"
        @mouseenter="cancelNodeTooltipHide"
        @mouseleave="scheduleNodeTooltipHide(tooltipNode.id)"
        @focusin="cancelNodeTooltipHide"
      >
        <SpecializationNodeInspector
          :node-state="tooltipNodeState"
          :display-name="localized(tooltipNode.nameKey, tooltipNode.id)"
          :school-names="tooltipSchoolNames"
          :keywords="tooltipKeywords"
          :current-effect-summary-lines="tooltipCurrentEffectSummaryLines"
          :next-effect-summary-lines="tooltipNextEffectSummaryLines"
          :technical-effect-lines="tooltipEffectLines"
          :requirement-lines="tooltipRequirementLines"
          :busy="specializationBusy"
          @allocate="allocateTooltipNode"
          @close="clearNodeSelection"
        />
      </div>
    </el-popover>

    <GameDialog
      v-model="impactDialogVisible"
      :title="t('game.specialization.impactTitle')"
      width="560px"
    >
      <div v-if="pendingImpact" class="specialization-impact">
        <p>{{ t("game.specialization.impactHint") }}</p>
        <div v-if="pendingImpact.disabledSlots.length" class="specialization-impact__section">
          <strong>{{ t("game.specialization.disabledSlots") }}</strong>
          <div class="specialization-impact__tags">
            <el-tag v-for="slot in pendingImpact.disabledSlots" :key="`disabled-${slot}`" type="danger" effect="plain">
              {{ slotLabel(slot) }}
            </el-tag>
          </div>
        </div>
        <div v-if="pendingImpact.enabledSlots.length" class="specialization-impact__section">
          <strong>{{ t("game.specialization.enabledSlots") }}</strong>
          <div class="specialization-impact__tags">
            <el-tag v-for="slot in pendingImpact.enabledSlots" :key="`enabled-${slot}`" type="success" effect="plain">
              {{ slotLabel(slot) }}
            </el-tag>
          </div>
        </div>
        <div v-if="pendingImpact.autoUnequipped.length" class="specialization-impact__section">
          <strong>{{ t("game.specialization.autoUnequipped") }}</strong>
          <ul>
            <li v-for="item in pendingImpact.autoUnequipped" :key="item.instanceId">
              {{ itemName(item.templateItemId) }} · {{ item.fromSlots.map(slotLabel).join(", ") }}
            </li>
          </ul>
        </div>
        <div v-if="pendingDisabledSkills.length" class="specialization-impact__section">
          <strong>{{ t("game.specialization.disabledSkills") }}</strong>
          <ul>
            <li v-for="skill in pendingDisabledSkills" :key="skill.skillId">
              {{ skillName(skill.skillId) }}
              <span v-for="reason in skill.reasons" :key="reason" class="specialization-impact__reason">{{ disabledSkillReason(reason) }}</span>
            </li>
          </ul>
        </div>
      </div>
      <template #footer>
        <el-button @click="cancelImpact">{{ t("game.cancel") }}</el-button>
        <el-button type="primary" :loading="specializationBusy" @click="confirmImpact">
          {{ t("game.specialization.confirmImpact") }}
        </el-button>
      </template>
    </GameDialog>
  </section>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, shallowRef, watch } from "vue"
import { ElMessage, ElMessageBox } from "element-plus"
import { QuestionFilled, Refresh, RefreshLeft } from "@element-plus/icons-vue"
import { useI18n } from "vue-i18n"
import { Background } from "@vue-flow/background"
import { Controls } from "@vue-flow/controls"
import { MiniMap } from "@vue-flow/minimap"
import { Handle, Position, VueFlow, type Edge, type Node, type NodeMouseEvent, useVueFlow } from "@vue-flow/core"
import "@vue-flow/core/dist/style.css"
import "@vue-flow/core/dist/theme-default.css"
import "@vue-flow/controls/dist/style.css"
import "@vue-flow/minimap/dist/style.css"
import { useDevice } from "@composables/game/useDevice"
import { useSpecializationStore } from "@stores/specialization"
import { createIdempotencyKey } from "@services/api"
import { resolveErrorMessage } from "@services/error-messages"
import { getAttributeDefinition } from "@services/attribute-display"
import { getLocalizedItemName } from "@services/item-label"
import { getLocalizedSkillName } from "@services/skill-definition"
import { buildSpecializationGrantDisplayLines } from "@services/specialization-grant-display"
import {
  normalizeSpecializationSummaryLine,
  resolveSpecializationKeywords
} from "@services/specialization-keywords"
import {
  collectSpecializationAncestorIds,
  minimalSpecializationPrerequisiteIds
} from "@services/specialization-route"
import type {
  SpecializationGrant,
  SpecializationImpact,
  SpecializationNode,
  SpecializationNodeState,
  SpecializationRequirement,
  SpecializationSchoolId
} from "@services/specialization"
import { isSpecializationStateConflictError, normalizeDisabledSkillImpacts } from "@services/specialization"
import GameDialog from "@components/game/common/GameDialog.vue"
import SpecializationNodeInspector from "@components/game/attributes/SpecializationNodeInspector.vue"

type NodeVisualState = "learned" | "available" | "locked" | "exclusive"
type RouteRelation = "none" | "selected" | "ancestor" | "muted"
type NodeTooltipPlacement = "right-start" | "right-end" | "left-start" | "left-end" | "bottom-start" | "bottom-end" | "top-start" | "top-end"
type SpecializationCanvasData = {
  node: SpecializationNode
  state: SpecializationNodeState
  visualState: NodeVisualState
  routeRelation: RouteRelation
}

type BoardPoint = { x: number; y: number }
const BOARD_CENTER: BoardPoint = { x: 1600, y: 1600 }
const BOARD_NODE_SIZE = { width: 176, height: 92 }
const BOARD_GRID_STEP = { x: 224, y: 144 }
const BOARD_TOP_BRANCH_COLUMNS = [-2, 2]
const BOARD_UPPER_BRANCH_ROWS = [-5, -2]
const BOARD_LOWER_BRANCH_ROWS = [2, 5]

const emit = defineEmits<{
  (e: "changed"): void
}>()
const props = defineProps<{
  active: boolean
}>()

const { t, locale } = useI18n()
const { isMobile } = useDevice()
const { setCenter } = useVueFlow("player-specialization-flow")
const specializationStore = useSpecializationStore()
const specializationBusy = computed(() => specializationStore.loading || specializationStore.mutating)
const selectedNodeId = ref("")
const hoveredNodeId = ref("")
const tooltipReference = shallowRef<HTMLElement | null>(null)
const tooltipPlacement = ref<NodeTooltipPlacement>("right-start")
const initialFitDone = ref(false)
const impactDialogVisible = ref(false)
const pendingImpact = ref<SpecializationImpact | null>(null)
const pendingDisabledSkills = computed(() => normalizeDisabledSkillImpacts(pendingImpact.value))
type PendingOperation =
  | { kind: "allocate"; nodeId: string; idempotencyKey: string }
  | { kind: "reset"; idempotencyKey: string }

const pendingOperation = ref<PendingOperation | null>(null)
const resetting = ref(false)
const reconciledRulesetResetRevision = ref<number | null>(null)
const nodeElements = new Map<string, HTMLElement>()
let tooltipHideTimer: ReturnType<typeof setTimeout> | null = null

function disabledSkillReason(reason: string) {
  if (reason === "requirements_unmet") return t("game.specialization.disabledSkillReason.requirementsUnmet")
  if (reason === "slot_capacity_exceeded") return t("game.specialization.disabledSkillReason.slotCapacityExceeded")
  return t("game.specialization.disabledSkillReason.other", { reason })
}

const summary = computed(() => specializationStore.summary)
const status = computed(() => summary.value?.status || null)
const rulesetResetDescription = computed(() => {
  const reset = summary.value?.rulesetReset
  if (!reset) return ""
  const equipment = reset.impact.autoUnequipped || []
  if (equipment.length === 0) return t("game.specialization.rulesetResetDescription")
  const names = new Intl.ListFormat(String(locale.value || "zh-CN"), { style: "long", type: "conjunction" })
    .format(equipment.map((item) => itemName(item.templateItemId)))
  return t("game.specialization.rulesetResetDescriptionWithEquipment", {
    count: equipment.length,
    names
  })
})
const nodeMap = computed(() => new Map((summary.value?.tree.nodes || []).map((node) => [node.id, node])))
const allocationMap = computed(() => new Map((status.value?.allocations || []).map((row) => [row.nodeId, row])))
const maxActiveRootNodes = computed(() => summary.value?.tree.maxActiveRootNodes || 0)
const activeRootNodeCount = computed(() => Array.from(allocationMap.value.values()).reduce((count, allocation) => {
  const node = nodeMap.value.get(allocation.nodeId)
  return count + (allocation.rank > 0 && node?.kind === "root" ? 1 : 0)
}, 0))
const backendNodeStateMap = computed(() => new Map((status.value?.nodeStates || []).map((row) => [row.nodeId, row])))
const schoolMap = computed(() => new Map((summary.value?.tree.schools || []).map((school) => [school.id, school])))
const selectedNode = computed(() => nodeMap.value.get(selectedNodeId.value) || null)
const tooltipNodeId = computed(() => selectedNodeId.value || hoveredNodeId.value)
const tooltipNode = computed(() => nodeMap.value.get(tooltipNodeId.value) || null)
const tooltipNodeState = computed(() => tooltipNode.value ? nodeState(tooltipNode.value) : emptyNodeState(""))
const tooltipSchoolNames = computed(() => (tooltipNode.value?.schoolIds || []).map(schoolName))
const tooltipEffectLines = computed(() => tooltipNode.value
  ? effectLines(tooltipNode.value, technicalLevelIndex(tooltipNode.value, tooltipNodeState.value.rank))
  : [])
const tooltipCurrentEffectSummaryLines = computed(() => tooltipNode.value && tooltipNodeState.value.rank > 0
  ? effectSummaryLines(tooltipNode.value, tooltipNodeState.value.rank - 1)
  : [])
const tooltipNextEffectSummaryLines = computed(() => tooltipNode.value && tooltipNodeState.value.rank < tooltipNodeState.value.maxRank
  ? effectSummaryLines(tooltipNode.value, tooltipNodeState.value.rank)
  : [])
const tooltipRequirementLines = computed(() => tooltipNode.value ? requirementLines(tooltipNode.value) : [])
const tooltipKeywords = computed(() => {
  const tree = summary.value?.tree
  const node = tooltipNode.value
  if (!tree || !node) return []
  const source = specializationSourceLocale()
  return resolveSpecializationKeywords(tree, node, {
    locale: String(locale.value || "zh-CN"),
    localizedKeywords: source?.keywords,
    t: (key, params) => t(key, params),
    nodeName: (nodeId) => {
      const candidate = nodeMap.value.get(nodeId)
      return candidate ? localized(candidate.nameKey, candidate.id) : nodeId
    }
  })
})
const selectedAncestorIds = computed(() => selectedNodeId.value
  ? collectSpecializationAncestorIds(summary.value?.tree.nodes || [], selectedNodeId.value)
  : new Set<string>())
const highlightedNodeIds = computed(() => {
  const ids = new Set(selectedAncestorIds.value)
  if (selectedNodeId.value) ids.add(selectedNodeId.value)
  return ids
})

const pointSourceHeadline = computed(() => t("game.specialization.pointOrigin", {
  total: status.value?.totalPoints ?? 0
}))

const pointSourceDetailLines = computed(() => {
  const breakdown = status.value?.pointBreakdown
  if (!breakdown) return [t("game.specialization.pointSourceFallback")]
  const parts: string[] = []
  const unlockSource = breakdown.sources.find((source) => source.kind === "unlock_grant" && source.earned)
  if (breakdown.unlockGrantPoints > 0) {
    parts.push(unlockSource?.requiredBattleLevel !== null && unlockSource?.requiredBattleLevel !== undefined
      ? t("game.specialization.pointSourceUnlockAtLevel", {
          level: unlockSource.requiredBattleLevel,
          points: breakdown.unlockGrantPoints
        })
      : t("game.specialization.pointSourcePart", {
          source: t("game.specialization.pointSource.unlockGrant"),
          points: breakdown.unlockGrantPoints
        }))
  }
  for (const band of breakdown.milestoneBands || []) {
    if (band.countedPoints <= 0 || band.firstEarnedBattleLevel === null || band.lastEarnedBattleLevel === null) continue
    if (band.earnedGrantCount > 1) {
      parts.push(t("game.specialization.pointSourceMilestoneRange", {
        from: band.firstEarnedBattleLevel,
        to: band.lastEarnedBattleLevel,
        every: band.everyLevels,
        pointsEach: band.pointsPerGrant,
        points: band.countedPoints
      }))
    } else {
      parts.push(t("game.specialization.pointSourceMilestoneSingle", {
        level: band.firstEarnedBattleLevel,
        points: band.countedPoints
      }))
    }
  }
  if (breakdown.countedRewardPoints > 0) {
    parts.push(t("game.specialization.pointSourcePart", {
      source: t("game.specialization.pointSource.reward"),
      points: breakdown.countedRewardPoints
    }))
  }
  return parts.length > 0 ? parts : [t("game.specialization.pointSourceFallback")]
})

const nextPointLabel = computed(() => {
  const breakdown = status.value?.pointBreakdown
  if (!breakdown) return t("game.specialization.nextPointUnknown")
  if (breakdown.atCap) return t("game.specialization.allPointsEarned")
  if (breakdown.nextPointBattleLevel === null) return t("game.specialization.nextPointUnknown")
  return t("game.specialization.nextPointAtLevel", {
    level: breakdown.nextPointBattleLevel,
    points: breakdown.nextPointPoints
  })
})

const visibleNodeList = computed(() => summary.value?.tree.nodes || [])
const visibleNodeIds = computed(() => new Set(visibleNodeList.value.map((node) => node.id)))
const boardPositions = computed(() => buildAlignedFiveSpokeBoardPositions(visibleNodeList.value))

const defaultEdgeOptions = {
  type: "step",
  animated: false,
  style: {
    stroke: "var(--app-border)",
    strokeWidth: 1.5
  }
}

const flowNodes = computed<Node<SpecializationCanvasData>[]>(() =>
  visibleNodeList.value.map((node) => ({
    id: node.id,
    type: "specialization",
    position: boardPositions.value.get(node.id) || BOARD_CENTER,
    data: {
      node,
      state: nodeState(node),
      visualState: nodeVisualState(node),
      routeRelation: routeRelation(node.id)
    },
    selected: node.id === selectedNodeId.value
  }))
)

const flowEdges = computed<Edge[]>(() => {
  const seen = new Set<string>()
  const edges: Edge[] = []
  for (const node of visibleNodeList.value) {
    if (node.kind === "confluence" && node.id !== selectedNodeId.value) continue
    const alternativePrerequisiteIds = new Set((node.requirements?.anyOf || [])
      .flat()
      .filter((requirement) => requirement.kind === "node_rank")
      .map((requirement) => String(requirement.nodeId || "").trim())
      .filter(Boolean))
    for (const prerequisiteId of minimalSpecializationPrerequisiteIds(visibleNodeList.value, node)) {
      if (!visibleNodeIds.value.has(prerequisiteId)) continue
      const prerequisiteNode = nodeMap.value.get(prerequisiteId)
      if (!prerequisiteNode) continue
      const id = `${prerequisiteId}->${node.id}`
      if (seen.has(id)) continue
      seen.add(id)
      const routeEdge = !!selectedNodeId.value
        && highlightedNodeIds.value.has(prerequisiteId)
        && highlightedNodeIds.value.has(node.id)
      const muted = selectedAncestorIds.value.size > 0 && !routeEdge
      const preferredAxis = preferredEdgeAxis(prerequisiteNode, node)
      const normalStroke = prerequisiteNode.schoolIds.length === 1 && node.schoolIds.length === 1
        ? `color-mix(in srgb, ${schoolAccent(prerequisiteNode.schoolIds)} 44%, var(--app-border))`
        : "var(--app-border)"
      edges.push({
        id,
        source: prerequisiteId,
        target: node.id,
        sourceHandle: edgeHandleId("source", boardPositions.value.get(prerequisiteId), boardPositions.value.get(node.id), preferredAxis),
        targetHandle: edgeHandleId("target", boardPositions.value.get(node.id), boardPositions.value.get(prerequisiteId), preferredAxis),
        type: "step",
        animated: false,
        class: routeEdge ? "specialization-edge--route" : muted ? "specialization-edge--muted" : "specialization-edge--normal",
        style: {
          stroke: routeEdge ? schoolAccent(selectedNode.value?.schoolIds || node.schoolIds) : normalStroke,
          strokeWidth: routeEdge ? 3 : 1.5,
          opacity: routeEdge ? 1 : muted ? 0.14 : 0.58,
          strokeDasharray: alternativePrerequisiteIds.has(prerequisiteId) ? "7 5" : undefined
        }
      })
    }
  }
  return edges
})

onMounted(async () => {
  window.addEventListener("keydown", handleGlobalKeydown)
  if (!props.active) return
  try {
    await specializationStore.load(!specializationStore.mutating)
    reconcileRulesetResetSideEffects()
    await focusInitialBoard()
  } catch (error: unknown) {
    ElMessage.error(resolveErrorMessage(t, error, "game.specialization.operationFailed"))
  }
})

onBeforeUnmount(() => {
  window.removeEventListener("keydown", handleGlobalKeydown)
  cancelNodeTooltipHide()
  nodeElements.clear()
})

watch(tooltipNodeId, async (nodeId) => {
  tooltipReference.value = null
  if (!nodeId) return
  await nextTick()
  tooltipReference.value = nodeElements.get(nodeId) || null
  updateTooltipPlacement()
})

watch(
  () => props.active,
  (active, previous) => {
    if (active && previous === false && !specializationStore.mutating) void refresh()
  }
)

watch(() => summary.value?.rulesetReset?.currentRevision, reconcileRulesetResetSideEffects)

watch(
  () => isMobile.value,
  () => {
    initialFitDone.value = false
    void focusInitialBoard()
  }
)

async function focusInitialBoard() {
  if (initialFitDone.value || visibleNodeList.value.length === 0) return
  initialFitDone.value = true
  await nextTick()
  const center = isMobile.value
    ? {
        x: BOARD_CENTER.x + (BOARD_TOP_BRANCH_COLUMNS[0] - 0.55) * BOARD_GRID_STEP.x,
        y: BOARD_CENTER.y - 4.8 * BOARD_GRID_STEP.y,
        zoom: 0.8
      }
    : { x: BOARD_CENTER.x, y: BOARD_CENTER.y - 4.5 * BOARD_GRID_STEP.y, zoom: 0.76 }
  await setCenter(center.x, center.y, { zoom: center.zoom, duration: 220 })
}

function emptyNodeState(nodeId: string): SpecializationNodeState {
  return { nodeId, rank: 0, maxRank: 1, nextCost: 0, canAllocate: false, unmetReasonCodes: [] }
}

function nodeState(node: SpecializationNode): SpecializationNodeState {
  const backend = backendNodeStateMap.value.get(node.id)
  if (backend) return backend
  const rank = allocationMap.value.get(node.id)?.rank || 0
  const maxRank = node.levels.length
  const nextCost = node.levels[rank]?.cost || 0
  const requirementsMet = nodeRequirementsMet(node)
  const exclusive = exclusiveConflict(node)
  const rootLimitReached = node.kind === "root"
    && rank === 0
    && maxActiveRootNodes.value > 0
    && activeRootNodeCount.value >= maxActiveRootNodes.value
  return {
    nodeId: node.id,
    rank,
    maxRank,
    nextCost,
    canAllocate: rank < maxRank && !exclusive && !rootLimitReached && requirementsMet && (status.value?.availablePoints || 0) >= nextCost,
    unmetReasonCodes: rootLimitReached ? ["ACTIVE_ROOT_LIMIT"] : []
  }
}

function nodeVisualState(node: SpecializationNode): NodeVisualState {
  const state = nodeState(node)
  if (state.rank > 0) return "learned"
  if (exclusiveConflict(node)) return "exclusive"
  return state.canAllocate ? "available" : "locked"
}

function nodeRequirements(node: SpecializationNode): SpecializationRequirement[] {
  return [
    ...(node.requirements?.all || []),
    ...(node.requirements?.anyOf || []).flat()
  ]
}

function requirementMet(requirement: SpecializationRequirement): boolean {
  if (requirement.kind === "node_rank") {
    return (allocationMap.value.get(requirement.nodeId || "")?.rank || 0) >= (requirement.minRank || 1)
  }
  if (requirement.kind === "pure_school_points_gte" || requirement.kind === "school_points_gte") {
    return schoolInvestedPoints(requirement.schoolId || "strength", requirement.kind === "pure_school_points_gte")
      >= (requirement.points || 0)
  }
  return false
}

function schoolInvestedPoints(schoolId: SpecializationSchoolId, pureOnly: boolean): number {
  return Array.from(allocationMap.value.values()).reduce((total, allocation) => {
    const allocatedNode = nodeMap.value.get(allocation.nodeId)
    if (!allocatedNode?.schoolIds.includes(schoolId)) return total
    if (pureOnly && allocatedNode.schoolIds.length !== 1) return total
    return total + allocation.paidCost
  }, 0)
}

function nodeRequirementsMet(node: SpecializationNode): boolean {
  const allMet = (node.requirements?.all || []).every(requirementMet)
  const anyGroupsMet = (node.requirements?.anyOf || []).every((group) => group.some(requirementMet))
  const hasAttributeRequirement = nodeRequirements(node).some((requirement) => requirement.kind === "attribute_gte")
  return allMet && anyGroupsMet && !hasAttributeRequirement
}

function exclusiveConflict(node: SpecializationNode): boolean {
  if (!node.exclusiveGroupId) return false
  return (summary.value?.tree.nodes || []).some((candidate) =>
    candidate.id !== node.id
    && candidate.exclusiveGroupId === node.exclusiveGroupId
    && (allocationMap.value.get(candidate.id)?.rank || 0) > 0
  )
}

/**
 * Keep every card on one shared grid: five school trees surround a compact
 * confluence matrix, while prerequisite topology decides branch membership.
 */
function buildAlignedFiveSpokeBoardPositions(nodes: readonly SpecializationNode[]): Map<string, BoardPoint> {
  const schoolOrder = summary.value?.tree.ui?.schoolOrder
    || summary.value?.tree.schools.map((school) => school.id)
    || []
  const result = new Map<string, BoardPoint>()
  const occupiedCells = new Set<string>()
  const branchByNodeId = new Map<string, number>()

  schoolOrder.forEach((schoolId, schoolIndex) => {
    const schoolNodes = nodes.filter((node) => node.schoolIds.length === 1 && node.schoolIds[0] === schoolId)
    const roots = schoolNodes.filter((node) => node.kind === "root").sort(compareLayoutNodes)
    const techniques = schoolNodes.filter((node) => node.kind === "technique").sort(compareLayoutNodes)
    const terminals = schoolNodes.filter((node) => node.kind === "terminal").sort(compareLayoutNodes)

    roots.forEach((root, branchIndex) => {
      const anchor = schoolBranchAnchor(schoolIndex, branchIndex)
      placeBoardNode(result, occupiedCells, root.id, anchor)
      branchByNodeId.set(root.id, branchIndex)

      const branchTechniques = techniques.filter((node) => nodeRankRequirementIds(node).includes(root.id))
      placeSchoolBranchTier(result, occupiedCells, branchTechniques, schoolIndex, anchor, 1)
      const techniqueIds = new Set(branchTechniques.map((node) => node.id))
      const branchTerminals = terminals.filter((node) =>
        nodeRankRequirementIds(node).some((nodeId) => techniqueIds.has(nodeId))
      )
      placeSchoolBranchTier(result, occupiedCells, branchTerminals, schoolIndex, anchor, 2)

      for (const node of [...branchTechniques, ...branchTerminals]) {
        branchByNodeId.set(node.id, branchIndex)
      }
    })
  })

  const schoolIndex = new Map(schoolOrder.map((schoolId, index) => [schoolId, index]))
  const pairOrder: string[] = []
  for (let left = 0; left < schoolOrder.length; left += 1) {
    for (let right = left + 1; right < schoolOrder.length; right += 1) {
      pairOrder.push(`${schoolOrder[left]}+${schoolOrder[right]}`)
    }
  }
  const confluenceGroups = new Map<string, SpecializationNode[]>()
  for (const node of nodes.filter((candidate) => candidate.kind === "confluence")) {
    const key = [...node.schoolIds]
      .sort((left, right) => (schoolIndex.get(left) ?? 99) - (schoolIndex.get(right) ?? 99))
      .join("+")
    const group = confluenceGroups.get(key) || []
    group.push(node)
    confluenceGroups.set(key, group)
  }
  for (const [pairKey, group] of confluenceGroups) {
    const pairIndex = pairOrder.indexOf(pairKey)
    if (pairIndex < 0) continue
    group.sort((left, right) =>
      confluenceBranchIndex(left, branchByNodeId) - confluenceBranchIndex(right, branchByNodeId)
      || compareLayoutNodes(left, right)
    )
    const column = pairIndex % 5 - 2
    const pairRow = Math.floor(pairIndex / 5) * 2 - 2
    group.forEach((node, index) => {
      placeBoardNode(result, occupiedCells, node.id, { column, row: pairRow + index })
    })
  }

  const unplacedNodes = nodes.filter((node) => !result.has(node.id)).sort(compareLayoutNodes)
  unplacedNodes.forEach((node, index) => {
    const rawColumn = Number(node.ui?.column)
    const rawRow = Number(node.ui?.row)
    placeBoardNode(result, occupiedCells, node.id, {
      column: Number.isFinite(rawColumn) ? rawColumn - 4 : index % 5 - 2,
      row: Number.isFinite(rawRow) ? rawRow + 2 : Math.floor(index / 5) + 2
    })
  })

  return result
}

function schoolBranchAnchor(schoolIndex: number, branchIndex: number): BoardCell {
  if (schoolIndex === 0) {
    return {
      column: BOARD_TOP_BRANCH_COLUMNS[branchIndex] ?? -2 + branchIndex * 4,
      row: -4
    }
  }
  const upperHalf = schoolIndex === 1 || schoolIndex === 4
  const rows = upperHalf ? BOARD_UPPER_BRANCH_ROWS : BOARD_LOWER_BRANCH_ROWS
  return {
    column: schoolIndex === 1 || schoolIndex === 2 ? 4 : -4,
    row: rows[branchIndex] ?? rows[0] + branchIndex * 3
  }
}

function placeSchoolBranchTier(
  positions: Map<string, BoardPoint>,
  occupiedCells: Set<string>,
  nodes: readonly SpecializationNode[],
  schoolIndex: number,
  anchor: BoardCell,
  tier: 1 | 2
) {
  const offsets = centeredGridOffsets(nodes.length)
  nodes.forEach((node, index) => {
    const offset = offsets[index] || 0
    const cell = schoolIndex === 0
      ? { column: anchor.column + offset, row: anchor.row - tier }
      : {
          column: anchor.column + (schoolIndex === 1 || schoolIndex === 2 ? tier : -tier),
          row: anchor.row + offset
        }
    placeBoardNode(positions, occupiedCells, node.id, cell)
  })
}

type BoardCell = { column: number; row: number }

function placeBoardNode(
  positions: Map<string, BoardPoint>,
  occupiedCells: Set<string>,
  nodeId: string,
  requestedCell: BoardCell
) {
  const cell = nearestOpenBoardCell(requestedCell, occupiedCells)
  occupiedCells.add(boardCellKey(cell))
  positions.set(nodeId, {
    x: BOARD_CENTER.x + cell.column * BOARD_GRID_STEP.x - BOARD_NODE_SIZE.width / 2,
    y: BOARD_CENTER.y + cell.row * BOARD_GRID_STEP.y - BOARD_NODE_SIZE.height / 2
  })
}

function nearestOpenBoardCell(requestedCell: BoardCell, occupiedCells: Set<string>): BoardCell {
  for (let radius = 0; radius <= 16; radius += 1) {
    for (let rowOffset = -radius; rowOffset <= radius; rowOffset += 1) {
      for (let columnOffset = -radius; columnOffset <= radius; columnOffset += 1) {
        if (Math.abs(columnOffset) + Math.abs(rowOffset) !== radius) continue
        const candidate = {
          column: requestedCell.column + columnOffset,
          row: requestedCell.row + rowOffset
        }
        if (!occupiedCells.has(boardCellKey(candidate))) return candidate
      }
    }
  }
  return requestedCell
}

function boardCellKey(cell: BoardCell): string {
  return `${cell.column}:${cell.row}`
}

function nodeRankRequirementIds(node: SpecializationNode): string[] {
  return [
    ...(node.requirements?.all || []),
    ...(node.requirements?.anyOf || []).flat()
  ]
    .filter((requirement) => requirement.kind === "node_rank")
    .map((requirement) => String(requirement.nodeId || "").trim())
    .filter(Boolean)
}

function confluenceBranchIndex(node: SpecializationNode, branchByNodeId: ReadonlyMap<string, number>): number {
  const prerequisiteBranches = nodeRankRequirementIds(node)
    .map((nodeId) => branchByNodeId.get(nodeId))
    .filter((branch): branch is number => branch !== undefined)
  if (prerequisiteBranches.length === 0) return 0
  return Math.round(prerequisiteBranches.reduce((sum, branch) => sum + branch, 0) / prerequisiteBranches.length)
}

function compareLayoutNodes(left: SpecializationNode, right: SpecializationNode) {
  return Number(left.ui?.column || 0) - Number(right.ui?.column || 0)
    || Number(left.ui?.row || 0) - Number(right.ui?.row || 0)
    || left.id.localeCompare(right.id)
}

function centeredGridOffsets(count: number): number[] {
  if (count <= 1) return [0]
  return Array.from({ length: count }, (_, index) => index * 2 - (count - 1))
}

function preferredEdgeAxis(fromNode: SpecializationNode, toNode: SpecializationNode): "horizontal" | "vertical" | undefined {
  if (fromNode.schoolIds.length !== 1 || toNode.schoolIds.length !== 1) return undefined
  if (fromNode.schoolIds[0] !== toNode.schoolIds[0]) return undefined
  const schoolOrder = summary.value?.tree.ui?.schoolOrder || summary.value?.tree.schools.map((school) => school.id) || []
  return schoolOrder.indexOf(fromNode.schoolIds[0]) === 0 ? "vertical" : "horizontal"
}

function edgeHandleId(
  kind: "source" | "target",
  from?: BoardPoint,
  to?: BoardPoint,
  preferredAxis?: "horizontal" | "vertical"
): string {
  if (!from || !to) return `${kind}-bottom`
  const dx = to.x - from.x
  const dy = to.y - from.y
  if (preferredAxis === "horizontal") return `${kind}-${dx >= 0 ? "right" : "left"}`
  if (preferredAxis === "vertical") return `${kind}-${dy >= 0 ? "bottom" : "top"}`
  if (Math.abs(dx) > Math.abs(dy)) return `${kind}-${dx >= 0 ? "right" : "left"}`
  return `${kind}-${dy >= 0 ? "bottom" : "top"}`
}

function routeRelation(nodeId: string): RouteRelation {
  if (!selectedNodeId.value) return "none"
  if (nodeId === selectedNodeId.value) return "selected"
  if (selectedAncestorIds.value.has(nodeId)) return "ancestor"
  return selectedAncestorIds.value.size > 0 ? "muted" : "none"
}

function captureNodeElement(nodeId: string, element: unknown) {
  if (element instanceof HTMLElement) {
    nodeElements.set(nodeId, element)
    if (tooltipNodeId.value === nodeId) {
      tooltipReference.value = element
      updateTooltipPlacement()
    }
    return
  }
  nodeElements.delete(nodeId)
}

function updateTooltipPlacement() {
  const reference = tooltipReference.value
  if (!reference) return
  const rect = reference.getBoundingClientRect()
  const horizontalEnd = rect.left + rect.width / 2 >= window.innerWidth / 2 ? "end" : "start"
  if (isMobile.value) {
    tooltipPlacement.value = rect.top + rect.height / 2 >= window.innerHeight / 2
      ? `top-${horizontalEnd}`
      : `bottom-${horizontalEnd}`
    return
  }
  const horizontalSide = window.innerWidth - rect.right >= rect.left ? "right" : "left"
  const verticalEdge = rect.top + rect.height / 2 >= window.innerHeight / 2 ? "end" : "start"
  tooltipPlacement.value = `${horizontalSide}-${verticalEdge}`
}

function selectNode(event: NodeMouseEvent) {
  toggleNodeSelection(event.node.id)
}

function toggleNodeSelection(nodeId: string) {
  const nextNodeId = selectedNodeId.value === nodeId ? "" : nodeId
  selectedNodeId.value = nextNodeId
  hoveredNodeId.value = nextNodeId
}

function clearNodeSelection() {
  cancelNodeTooltipHide()
  selectedNodeId.value = ""
  hoveredNodeId.value = ""
  tooltipReference.value = null
}

function showNodeTooltip(nodeId: string) {
  cancelNodeTooltipHide()
  hoveredNodeId.value = nodeId
}

function scheduleNodeTooltipHide(nodeId: string) {
  cancelNodeTooltipHide()
  tooltipHideTimer = setTimeout(() => {
    if (hoveredNodeId.value === nodeId) hoveredNodeId.value = ""
    tooltipHideTimer = null
  }, 180)
}

function cancelNodeTooltipHide() {
  if (tooltipHideTimer) clearTimeout(tooltipHideTimer)
  tooltipHideTimer = null
}

function handleGlobalKeydown(event: KeyboardEvent) {
  if (event.key !== "Escape" || !selectedNodeId.value) return
  clearNodeSelection()
}

function localized(key: string, fallback: string) {
  const source = specializationSourceLocale()
  if (source) {
    if (key === summary.value?.tree.nameKey) return source.name
    if (key === summary.value?.tree.descKey) return source.desc
    const school = summary.value?.tree.schools.find((entry) => entry.nameKey === key || entry.descKey === key)
    if (school) {
      const entry = source.schools?.[school.id]
      if (entry) return key === school.nameKey ? entry.name : entry.desc
    }
    const node = summary.value?.tree.nodes.find((entry) => entry.nameKey === key || entry.descKey === key)
    if (node) {
      const entry = source.nodes?.[node.id]
      if (entry) return key === node.nameKey ? entry.name : entry.desc
    }
  }
  const value = t(key)
  if (value !== key) return value
  return fallback
}

function specializationSourceLocale() {
  const locales = summary.value?.tree.i18n
  if (!locales) return null
  const requested = String(locale.value || "zh-CN")
  return locales[requested]
    || locales[requested.toLowerCase().startsWith("zh") ? "zh-CN" : "en-US"]
    || locales["zh-CN"]
    || locales["en-US"]
    || null
}

function schoolName(schoolId: SpecializationSchoolId) {
  const school = schoolMap.value.get(schoolId)
  return school ? localized(school.nameKey, schoolId) : schoolId
}

function schoolAccent(schoolIds: SpecializationSchoolId[]) {
  if (schoolIds.length > 1) return "var(--game-gold)"
  const configured = schoolMap.value.get(schoolIds[0])?.ui?.color
  if (configured) return configured
  switch (schoolIds[0]) {
    case "strength": return "var(--game-hp)"
    case "dexterity": return "var(--game-success)"
    case "intelligence": return "var(--app-primary)"
    case "perception": return "var(--game-gold)"
    case "stamina": return "var(--game-xp)"
    default: return "var(--app-primary)"
  }
}

function miniMapNodeColor(node: Node) {
  const data = node.data as SpecializationCanvasData | undefined
  return data ? schoolAccent(data.node.schoolIds) : "var(--app-primary)"
}

function nodeKindLabel(kind: SpecializationNode["kind"]) {
  return t(`game.specialization.kind.${kind}`)
}

function visualStateLabel(state: NodeVisualState) {
  return t(`game.specialization.state.${state}`)
}

function technicalLevelIndex(node: SpecializationNode, rank: number): number {
  if (node.levels.length === 0) return 0
  return Math.min(Math.max(0, rank), node.levels.length - 1)
}

function effectLines(node: SpecializationNode, levelIndex: number): string[] {
  return (node.levels[levelIndex]?.grants || []).flatMap((grant) => summarizeGrant(grant, true))
}

function effectSummaryLines(node: SpecializationNode, levelIndex: number): string[] {
  const localizedNode = specializationSourceLocale()?.nodes?.[node.id]
  const rankedSummary = localizedNode?.levelEffectSummary?.[levelIndex]
  if (Array.isArray(rankedSummary) && rankedSummary.length > 0) {
    return rankedSummary.map(normalizeSpecializationSummaryLine).filter(Boolean)
  }
  if (levelIndex === node.levels.length - 1 && Array.isArray(localizedNode?.effectSummary) && localizedNode.effectSummary.length > 0) {
    return localizedNode.effectSummary.map(normalizeSpecializationSummaryLine).filter(Boolean)
  }
  return (node.levels[levelIndex]?.grants || [])
    .flatMap((grant) => summarizeGrant(grant, true))
}

function summarizeGrant(grant: SpecializationGrant, includeHookMetadata: boolean): string[] {
  return buildSpecializationGrantDisplayLines(grant, {
    t: (key, params) => t(key, params),
    localized,
    skillName,
    slotFamilyLabel,
    characterAttributes: summary.value?.characterAttributes || {},
    includeHookMetadata
  })
}

function requirementLines(node: SpecializationNode): string[] {
  const allRequirements = node.requirements?.all || []
  const allNodeRequirements = allRequirements.filter((requirement) => requirement.kind === "node_rank")
  const alternativeGroups = (node.requirements?.anyOf || [])
    .map((group) => group.filter((requirement) => requirement.kind === "node_rank"))
    .filter((group) => group.length > 0)
  const routeSchoolIds = Array.from(new Set([
    ...allNodeRequirements,
    ...alternativeGroups.flat()
  ].flatMap((requirement) => nodeMap.value.get(requirement.nodeId || "")?.schoolIds || [])))
  const lines: string[] = []

  for (const schoolId of routeSchoolIds) {
    const mandatory = allNodeRequirements
      .filter((requirement) => nodeMap.value.get(requirement.nodeId || "")?.schoolIds.includes(schoolId))
      .sort((left, right) => compareRouteNodes(
        nodeMap.value.get(left.nodeId || "")!,
        nodeMap.value.get(right.nodeId || "")!
      ))
      .map(routeRequirementLabel)
    const choices = alternativeGroups
      .map((group) => group
        .filter((requirement) => nodeMap.value.get(requirement.nodeId || "")?.schoolIds.includes(schoolId))
        .sort((left, right) => compareRouteNodes(
          nodeMap.value.get(left.nodeId || "")!,
          nodeMap.value.get(right.nodeId || "")!
        ))
        .map(routeRequirementLabel))
      .filter((group) => group.length > 0)
      .map((group) => t("game.specialization.requirementChoice", { choices: group.join(t("game.specialization.requirementOr")) }))
    const route = [...mandatory, ...choices, localized(node.nameKey, node.id)].join(" → ")
    lines.push(t("game.specialization.requirementRoute", { school: schoolName(schoolId), route }))
  }

  for (const requirement of allRequirements.filter((entry) => entry.kind === "attribute_gte")) {
    lines.push(requirementLabel(requirement))
  }
  for (const requirement of allRequirements.filter((entry) => entry.kind === "pure_school_points_gte" || entry.kind === "school_points_gte")) {
    lines.push(requirementLabel(requirement))
  }
  for (const group of node.requirements?.anyOf || []) {
    const nonRouteRequirements = group.filter((requirement) => requirement.kind !== "node_rank")
    if (nonRouteRequirements.length > 0) {
      lines.push(t("game.specialization.requirementAny", { requirements: nonRouteRequirements.map(requirementLabel).join(" / ") }))
    }
  }
  if (lines.length === 0) lines.push(t("game.specialization.requirementStart", { school: schoolName(node.schoolIds[0]) }))
  if (node.exclusiveGroupId) lines.push(t("game.specialization.requirementExclusive"))
  return lines
}

function routeRequirementLabel(requirement: SpecializationRequirement) {
  const requiredNode = nodeMap.value.get(requirement.nodeId || "")
  return t("game.specialization.requirementNode", {
    node: requiredNode ? localized(requiredNode.nameKey, requiredNode.id) : requirement.nodeId || "-",
    rank: requirement.minRank || 1
  })
}

function compareRouteNodes(left: SpecializationNode, right: SpecializationNode) {
  const kindOrder: Record<SpecializationNode["kind"], number> = { root: 0, technique: 1, terminal: 2, confluence: 3 }
  return kindOrder[left.kind] - kindOrder[right.kind]
    || Number(left.ui?.row || 0) - Number(right.ui?.row || 0)
    || Number(left.ui?.column || 0) - Number(right.ui?.column || 0)
    || left.id.localeCompare(right.id)
}

function requirementLabel(requirement: SpecializationRequirement) {
  if (requirement.kind === "node_rank") {
    const requiredNode = nodeMap.value.get(requirement.nodeId || "")
    return t("game.specialization.requirementNode", {
      node: requiredNode ? localized(requiredNode.nameKey, requiredNode.id) : requirement.nodeId || "-",
      rank: requirement.minRank || 1
    })
  }
  if (requirement.kind === "pure_school_points_gte" || requirement.kind === "school_points_gte") {
    const schoolId = requirement.schoolId || "strength"
    return t("game.specialization.requirementSchoolPoints", {
      school: schoolName(schoolId),
      current: schoolInvestedPoints(schoolId, requirement.kind === "pure_school_points_gte"),
      required: requirement.points || 0
    })
  }
  const definition = getAttributeDefinition(requirement.attributeKey || "")
  const attr = definition?.nameKey ? localized(definition.nameKey, requirement.attributeKey || "") : requirement.attributeKey || "-"
  return t("game.specialization.requirementAttribute", { attr, value: requirement.value || 0 })
}

function slotFamilyLabel(family: string) {
  const key = `game.equipSlotFamily.${family}`
  return localized(key, family)
}

function slotLabel(slot: string) {
  const key = `game.equipSlot.${slot}`
  return localized(key, slot)
}

function itemName(itemId: string) {
  return getLocalizedItemName(t, itemId, { fallbackText: itemId })
}

function skillName(skillId: string) {
  return getLocalizedSkillName(t, skillId, skillId)
}

function reconcileRulesetResetSideEffects() {
  const revision = summary.value?.rulesetReset?.currentRevision
  if (revision === undefined || revision === reconciledRulesetResetRevision.value) return
  reconciledRulesetResetRevision.value = revision
  emit("changed")
}

async function refresh() {
  if (specializationStore.mutating) return
  try {
    await specializationStore.load(true)
    reconcileRulesetResetSideEffects()
  } catch (error: unknown) {
    ElMessage.error(resolveErrorMessage(t, error, "game.specialization.operationFailed"))
  }
}

async function allocateTooltipNode() {
  if (specializationBusy.value || !tooltipNode.value || !tooltipNodeState.value.canAllocate) return
  const nodeId = tooltipNode.value.id
  const idempotencyKey = createIdempotencyKey("specialization-allocate")
  try {
    const result = await specializationStore.allocate(nodeId, "", idempotencyKey)
    await handleMutationResult(result, { kind: "allocate", nodeId, idempotencyKey })
  } catch (error: unknown) {
    ElMessage.error(resolveErrorMessage(t, error, "game.specialization.operationFailed"))
  }
}

async function requestReset() {
  if (specializationBusy.value || resetting.value || !status.value || status.value.spentPoints <= 0) return
  try {
    await ElMessageBox.confirm(
      t("game.specialization.resetConfirm", {
        count: status.value.resetCost.count,
        item: itemName(status.value.resetCost.itemId)
      }),
      t("game.specialization.reset"),
      {
        confirmButtonText: t("game.confirm"),
        cancelButtonText: t("game.cancel"),
        type: "warning"
      }
    )
  } catch {
    return
  }
  resetting.value = true
  const idempotencyKey = createIdempotencyKey("specialization-reset")
  try {
    const result = await specializationStore.reset("", idempotencyKey)
    await handleMutationResult(result, { kind: "reset", idempotencyKey })
  } catch (error: unknown) {
    ElMessage.error(resolveErrorMessage(t, error, "game.specialization.operationFailed"))
  } finally {
    resetting.value = false
  }
}

async function handleMutationResult(
  result: Awaited<ReturnType<typeof specializationStore.allocate>>,
  operation: PendingOperation
) {
  if (result.requiresConfirmation && !result.committed) {
    pendingImpact.value = result.impact
    pendingOperation.value = operation
    impactDialogVisible.value = true
    return
  }
  if (!result.committed) return
  ElMessage.success(operation.kind === "reset" ? t("game.specialization.resetSuccess") : t("game.specialization.allocateSuccess"))
  emit("changed")
}

function cancelImpact() {
  impactDialogVisible.value = false
  pendingImpact.value = null
  pendingOperation.value = null
}

async function confirmImpact() {
  if (specializationBusy.value) return
  const impact = pendingImpact.value
  const operation = pendingOperation.value
  if (!impact || !operation) return
  try {
    const result = operation.kind === "allocate"
      ? await specializationStore.allocate(operation.nodeId, impact.digest, operation.idempotencyKey)
      : await specializationStore.reset(impact.digest, operation.idempotencyKey)
    if (result.committed) {
      cancelImpact()
      ElMessage.success(operation.kind === "reset" ? t("game.specialization.resetSuccess") : t("game.specialization.allocateSuccess"))
      emit("changed")
    }
  } catch (error: unknown) {
    if (isSpecializationStateConflictError(error)) cancelImpact()
    ElMessage.error(resolveErrorMessage(t, error, "game.specialization.operationFailed"))
  }
}
</script>

<style scoped>
.specialization-shell {
  --specialization-node-width: 176px;
  display: flex;
  height: 100%;
  min-height: 0;
  min-width: 0;
  flex-direction: column;
  overflow: hidden;
}

.specialization-toolbar {
  display: flex;
  flex: 0 0 auto;
  min-height: 44px;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 0 2px 6px;
}

.specialization-ledger,
.specialization-toolbar__actions,
.specialization-impact__tags {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.specialization-ledger {
  align-items: baseline;
  gap: 14px;
}

.specialization-ledger__balance {
  display: flex;
  align-items: baseline;
  gap: 7px;
}

.specialization-ledger__balance strong {
  color: var(--game-gold);
  font-size: 25px;
  line-height: 1;
}

.specialization-ledger__balance span,
.specialization-ledger__spent,
.specialization-ledger__routes {
  color: var(--app-text-muted);
  font-size: 12px;
  line-height: 1.25;
}

.specialization-ledger__help.el-button {
  width: 20px;
  height: 20px;
  align-self: center;
  margin-left: -3px;
  padding: 0;
  color: var(--app-text-muted);
  font-size: 14px;
}

.specialization-ledger__help.el-button:hover,
.specialization-ledger__help.el-button:focus-visible {
  background: color-mix(in srgb, var(--app-primary) 10%, transparent);
  color: var(--app-primary);
}

.specialization-point-guide {
  display: grid;
  grid-template-columns: 74px minmax(0, 1fr) minmax(116px, auto);
  align-items: start;
  gap: 14px;
  min-width: 0;
  padding: 2px 0;
}

.specialization-point-guide__level,
.specialization-point-guide__source,
.specialization-point-guide__next {
  display: grid;
  gap: 3px;
  min-width: 0;
}

.specialization-point-guide__level {
  border-right: 1px solid var(--app-border);
}

.specialization-point-guide__level span,
.specialization-point-guide__next span,
.specialization-point-guide__source li {
  color: var(--app-text-muted);
  font-size: 11px;
  line-height: 1.45;
}

.specialization-point-guide__source ul {
  display: grid;
  gap: 3px;
  margin: 0;
  padding: 0;
  list-style: none;
}

.specialization-point-guide__source li {
  position: relative;
  padding-left: 9px;
  overflow-wrap: anywhere;
}

.specialization-point-guide__source li::before {
  position: absolute;
  top: 0.65em;
  left: 0;
  width: 3px;
  height: 3px;
  border-radius: 50%;
  background: var(--app-primary);
  content: "";
}

.specialization-point-guide__level strong {
  font-size: 17px;
}

.specialization-point-guide__source strong,
.specialization-point-guide__next strong {
  overflow-wrap: anywhere;
  font-size: 12px;
  line-height: 1.45;
}

.specialization-point-guide__next {
  border-left: 1px solid var(--app-border);
  padding-left: 14px;
}

:global(.specialization-point-popper.el-popover) {
  max-width: calc(100vw - 20px);
  border-color: color-mix(in srgb, var(--app-primary) 20%, var(--app-border));
  border-radius: 6px;
  box-shadow: 0 10px 28px color-mix(in srgb, var(--app-bg) 72%, transparent);
  padding: 12px 14px;
}

.specialization-alert {
  flex: 0 0 auto;
  margin-top: 8px;
}

.specialization-workspace {
  display: block;
  flex: 1 1 auto;
  min-height: 0;
  overflow: hidden;
  border-top: 1px solid var(--app-border);
}

.specialization-canvas-wrap {
  position: relative;
  height: 100%;
  min-width: 0;
  min-height: 0;
  overflow: hidden;
}

.specialization-school-key {
  position: absolute;
  z-index: 5;
  top: 10px;
  left: 12px;
  display: flex;
  max-width: calc(100% - 24px);
  gap: 10px;
  border: 1px solid color-mix(in srgb, var(--app-border) 78%, transparent);
  border-radius: 6px;
  background: color-mix(in srgb, var(--app-surface) 88%, transparent);
  padding: 6px 8px;
  color: var(--app-text-muted);
  font-size: 10px;
  pointer-events: none;
}

.specialization-school-key span {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  white-space: nowrap;
}

.specialization-school-key i {
  width: 7px;
  height: 7px;
  border-radius: 50%;
}

.specialization-school-key__alternative b {
  width: 14px;
  border-top: 1px dashed currentColor;
}

.specialization-flow {
  width: 100%;
  height: 100%;
  min-height: 0;
  background: color-mix(in srgb, var(--app-bg) 64%, var(--app-surface));
}

.specialization-node {
  width: var(--specialization-node-width);
  min-height: 92px;
  border: 1px solid color-mix(in srgb, var(--specialization-accent) 38%, var(--app-border));
  border-inline-start-width: 3px;
  border-inline-start-color: color-mix(in srgb, var(--specialization-accent) 78%, var(--app-border));
  border-radius: 6px;
  background: color-mix(in srgb, var(--specialization-accent) 5%, var(--app-surface-elevated));
  box-shadow: 0 3px 10px color-mix(in srgb, var(--app-bg) 84%, transparent);
  padding: 9px 10px;
  color: var(--app-text);
  cursor: pointer;
  transition: border-color 160ms ease, box-shadow 160ms ease, opacity 160ms ease, filter 160ms ease;
}

.specialization-node:focus-visible {
  outline: 3px solid color-mix(in srgb, var(--specialization-accent) 36%, transparent);
  outline-offset: 3px;
}

.specialization-node.is-route-selected,
.specialization-node.is-selected {
  border-color: var(--specialization-accent);
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--specialization-accent) 22%, transparent), 0 6px 16px color-mix(in srgb, var(--app-bg) 76%, transparent);
}

.specialization-node.is-route-ancestor {
  border-color: color-mix(in srgb, var(--specialization-accent) 82%, var(--app-border));
  background: color-mix(in srgb, var(--specialization-accent) 13%, var(--app-surface-elevated));
  box-shadow: 0 0 0 2px color-mix(in srgb, var(--specialization-accent) 15%, transparent);
  opacity: 1;
  filter: none;
}

.specialization-node.is-route-muted {
  opacity: 0.2;
  filter: grayscale(0.7) saturate(0.35);
}

.specialization-node.is-learned {
  background: color-mix(in srgb, var(--specialization-accent) 12%, var(--app-surface-elevated));
}

.specialization-node.is-available {
  border-style: solid;
}

.specialization-node.is-locked,
.specialization-node.is-exclusive {
  opacity: 0.62;
  filter: saturate(0.62);
}

.specialization-node.is-locked.is-route-ancestor,
.specialization-node.is-exclusive.is-route-ancestor,
.specialization-node.is-locked.is-route-selected,
.specialization-node.is-exclusive.is-route-selected {
  opacity: 1;
  filter: none;
}

.specialization-node.is-route-muted,
.specialization-node.is-locked.is-route-muted,
.specialization-node.is-exclusive.is-route-muted {
  opacity: 0.2;
  filter: grayscale(0.7) saturate(0.35);
}

.specialization-node--confluence {
  border-width: 2px;
}

.specialization-node__head,
.specialization-node__foot {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  font-size: 10px;
  color: var(--app-text-muted);
}

.specialization-node h3 {
  margin: 9px 0 7px;
  overflow-wrap: anywhere;
  font-size: 14px;
  line-height: 1.25;
  letter-spacing: 0;
}

.specialization-node__foot {
  margin-top: 7px;
  border-top: 1px solid color-mix(in srgb, var(--specialization-accent) 18%, var(--app-border));
  padding-top: 6px;
}

.specialization-node__state {
  color: var(--specialization-accent);
}

.specialization-handle {
  opacity: 0;
  pointer-events: none;
}

:deep(.vue-flow__edge-path) {
  stroke-linecap: square;
  stroke-linejoin: miter;
  transition: stroke 160ms ease, stroke-width 160ms ease, opacity 160ms ease;
}

:global(.specialization-node-popper.el-popover) {
  max-width: calc(100vw - 20px);
  border-color: color-mix(in srgb, var(--app-primary) 20%, var(--app-border));
  border-radius: 6px;
  box-shadow: 0 12px 34px color-mix(in srgb, var(--app-bg) 68%, transparent);
  padding: 12px;
}

.specialization-impact {
  display: grid;
  gap: 16px;
}

.specialization-impact p {
  margin: 0;
  color: var(--app-text-muted);
}

.specialization-impact__section {
  display: grid;
  gap: 8px;
}

.specialization-impact ul {
  margin: 0;
  padding-left: 20px;
  color: var(--app-text-muted);
}

.specialization-impact__reason {
  display: block;
  margin-top: 2px;
  color: var(--app-text-secondary);
  font-size: 12px;
}

@media (max-width: 900px) {
  .specialization-shell {
    --specialization-node-width: 148px;
  }

  .specialization-toolbar {
    align-items: center;
  }

  .specialization-workspace {
    display: block;
    min-height: 0;
  }

  .specialization-canvas-wrap {
    min-height: 0;
    border-right: 0;
  }

  .specialization-flow {
    height: 100%;
    min-height: 0;
  }
}

@media (max-width: 460px) {
  .specialization-ledger {
    gap: 8px;
  }

  .specialization-ledger__balance strong {
    font-size: 24px;
  }

  .specialization-toolbar__actions {
    flex-wrap: nowrap;
  }

  .specialization-point-guide {
    grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
    gap: 10px;
  }

  .specialization-point-guide__level {
    padding-right: 10px;
  }

  .specialization-point-guide__source {
    grid-row: 2;
    grid-column: 1 / -1;
    border-top: 1px solid var(--app-border);
    padding-top: 8px;
  }

  .specialization-point-guide__next {
    grid-row: 1;
    grid-column: 2;
    border-left: 0;
    padding-left: 10px;
  }

  .specialization-flow {
    height: 100%;
    min-height: 0;
  }
}
</style>
