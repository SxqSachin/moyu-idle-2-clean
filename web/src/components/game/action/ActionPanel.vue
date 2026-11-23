<template>
  <GamePageFrame
    :title-key="titleKey"
    :tutorial-id="pageTutorialId"
    :visual-src="pageVisual?.src || ''"
    :visual-tone="pageVisual?.tone || 'green'"
    :visual-key="props.menu"
    data-tutorial="action-panel"
  >
    <div>
      <el-tabs v-if="subTypeGroups.length" v-model="activeSubType" class="action-panel-tabs" data-tutorial="action-tabs">
        <el-tab-pane
          v-for="group in subTypeGroups"
          :key="group.id"
          :name="group.id"
          :label="groupLabel(group)"
        >
          <div v-if="group.hasSections" class="space-y-4 pt-2">
            <section v-for="section in group.sectionedActions" :key="`action-slot-section-${group.id}-${section.id}`" class="space-y-2">
              <div class="text-sm font-bold">{{ section.label }}</div>
              <div v-if="section.actions.length" class="grid gap-3 grid-cols-[repeat(auto-fill,minmax(84px,1fr))]" data-tutorial="action-cards">
                <div
                  v-for="item in section.actions"
                  :key="item.id"
                  :class="{ 'opacity-30': statusRequirementUnmetByActionId[item.id] || false }"
                >
                  <ActionCard
                    :action-id="item.id"
                    :icon-id="item.iconId"
                    :label="actionLabel(item)"
                    :description="actionDescription(item)"
                    :rarity-color="actionRarityColor(item)"
                    :duration-s="estimatedDurationByActionId[item.id] || item.durationS"
                    :requirement-count="itemEntries(item.costs).length"
                    :output-count="itemEntries(item.itemReward).length"
                    @select="openAction(item)"
                  />
                </div>
              </div>
              <div
                v-if="section.subSections?.length"
                class="space-y-3 pl-4 border-l border-[var(--el-border-color-lighter)]"
              >
                <section
                  v-for="subSection in section.subSections"
                  :key="`action-slot-sub-section-${group.id}-${section.id}-${subSection.id}`"
                  class="space-y-2"
                >
                  <div class="text-xs font-semibold app-muted">{{ subSection.label }}</div>
                  <div class="grid gap-3 grid-cols-[repeat(auto-fill,minmax(84px,1fr))]" data-tutorial="action-cards">
                    <div
                      v-for="item in subSection.actions"
                      :key="item.id"
                      :class="{ 'opacity-30': statusRequirementUnmetByActionId[item.id] || false }"
                    >
                      <ActionCard
                        :action-id="item.id"
                        :icon-id="item.iconId"
                        :label="actionLabel(item)"
                        :description="actionDescription(item)"
                        :rarity-color="actionRarityColor(item)"
                        :duration-s="estimatedDurationByActionId[item.id] || item.durationS"
                        :requirement-count="itemEntries(item.costs).length"
                        :output-count="itemEntries(item.itemReward).length"
                        @select="openAction(item)"
                      />
                    </div>
                  </div>
                </section>
              </div>
            </section>
          </div>
          <div v-else-if="group.actions.length" class="grid gap-3 grid-cols-[repeat(auto-fill,minmax(84px,1fr))] pt-2" data-tutorial="action-cards">
            <div
              v-for="item in group.actions"
              :key="item.id"
              :class="{ 'opacity-30': statusRequirementUnmetByActionId[item.id] || false }"
            >
              <ActionCard
                :action-id="item.id"
                :icon-id="item.iconId"
                :label="actionLabel(item)"
                :description="actionDescription(item)"
                :rarity-color="actionRarityColor(item)"
                :duration-s="estimatedDurationByActionId[item.id] || item.durationS"
                :requirement-count="itemEntries(item.costs).length"
                :output-count="itemEntries(item.itemReward).length"
                @select="openAction(item)"
              />
            </div>
          </div>
          <div v-else class="app-muted text-sm">{{ t("game.actionEmpty") }}</div>
        </el-tab-pane>
      </el-tabs>
      <div v-else class="app-muted text-sm">{{ t("game.actionEmpty") }}</div>
    </div>
  </GamePageFrame>

    <ActionDetailDialog
      v-model="detailVisible"
      :action="selectedAction"
      :count="selectedAction ? (countById[selectedAction.id] || 1) : 1"
      :current-attributes="currentAttributes"
      @update:count="updateSelectedCount"
      @run-now="runNow"
      @enqueue="enqueue"
      @enqueue-plan="enqueuePlan"
    />
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from "vue"
import { useI18n } from "vue-i18n"
import { useRealtimeStore } from "@stores/realtime"
import { usePlayerStore } from "@stores/player"
import { getGameDataChunk } from "@services/game-data-runtime"
import type { ActionDefinition, ActionSubTypeDefinition } from "@game-data/game-data"
import ActionCard from "@components/game/action/ActionCard.vue"
import ActionDetailDialog from "@components/game/action/ActionDetailDialog.vue"
import GamePageFrame from "@components/game/common/GamePageFrame.vue"
import { useActionSubmit } from "@/composables/game/action/useActionSubmit"
import { getEquipmentTemplateDefinition, getRarityDefinition } from "@services/item-definition"
import { listActionDefinitions } from "@services/action-definition"
import { evaluateStatusRequire } from "@services/status-require"
import { resolveActionDurationSeconds } from "@/services/action-duration"
import { buildEquipmentSlotSecondarySections } from "@services/equipment-grouping"
import { buildActionPanelTabGroups } from "@services/action-panel-grouping"
import { resolveGamePageVisual } from "@services/game-page-visuals"

const props = defineProps<{
  menu: string
  titleKey: string
}>()

const { t } = useI18n()
const realtimeStore = useRealtimeStore()
const playerStore = usePlayerStore()
const allActions = ref<ActionDefinition[]>([])
const allSubTypes = ref<ActionSubTypeDefinition[]>([])
const countById = reactive<Record<string, number>>({})
const detailVisible = ref(false)
const selectedAction = ref<ActionDefinition | null>(null)
const activeSubType = ref("common")
type ActionSection = {
  id: string
  label: string
  actions: ActionDefinition[]
  subSections?: ActionSection[]
}

type ActionSubTypeGroup = {
  id: string
  nameKey?: string
  actions: ActionDefinition[]
  hasSections: boolean
  sectionedActions: ActionSection[]
}

const {
  runNowWithArgs: submitRunNowWithArgs,
  enqueueWithArgs: submitEnqueueWithArgs,
  enqueuePlanBatch
} = useActionSubmit()

const actions = computed(() => allActions.value.filter((item) => (item.mainType || item.menu) === props.menu))
const pageTutorialId = computed(() => props.menu === "crafting" ? "crafting" : "actions")
const pageVisual = computed(() => resolveGamePageVisual(props.menu))
const subTypeGroups = computed(() => {
  return buildActionPanelTabGroups(actions.value, allSubTypes.value).map<ActionSubTypeGroup>((group) => {
    const sectioned = group.sections?.length ? {
      hasSections: true,
      sectionedActions: group.sections.map((section) => ({
        id: section.id,
        label: section.nameKey ? t(section.nameKey) : section.id,
        actions: section.actions
      }))
    } : resolveGroupSections(group.actions)
    return {
      id: group.id,
      nameKey: group.nameKey,
      actions: group.actions,
      hasSections: sectioned.hasSections,
      sectionedActions: sectioned.sectionedActions
    }
  })
})
const currentAttributes = computed<Record<string, number>>(() => {
  const out: Record<string, number> = {}
  for (const [key, value] of Object.entries(playerStore.attributes || {})) {
    const node = value as { value?: unknown }
    if (typeof node?.value === "number" && Number.isFinite(node.value)) {
      out[key] = node.value
    }
  }
  for (const [key, value] of Object.entries(realtimeStore.attributes || {})) {
    if (typeof value === "number" && Number.isFinite(value)) {
      out[key] = value
    }
  }
  return out
})
const estimatedDurationByActionId = computed<Record<string, number>>(() => {
  const out: Record<string, number> = {}
  for (const action of actions.value) {
    out[action.id] = resolveActionDurationSeconds(
      Number(action.durationS || 1),
      action.mainType,
      action.subType,
      action.id,
      "player",
      currentAttributes.value
    )
  }
  return out
})
const statusRequirementUnmetByActionId = computed<Record<string, boolean>>(() => {
  const result: Record<string, boolean> = {}
  for (const action of actions.value) {
    result[action.id] = evaluateStatusRequire(action.statusRequirement, currentAttributes.value).length > 0
  }
  return result
})

onMounted(() => {
  loadCatalog()
})

watch(
  subTypeGroups,
  (groups) => {
    if (!groups.length) {
      activeSubType.value = "common"
      return
    }
    if (!groups.some((group) => group.id === activeSubType.value)) {
      activeSubType.value = groups[0].id
    }
  },
  { immediate: true }
)

function loadCatalog() {
  allActions.value = listActionDefinitions()
  allSubTypes.value = getCatalogActionSubTypes(getGameDataChunk("action_sub_types"))
  for (const item of allActions.value) {
    countById[item.id] = countById[item.id] || 1
  }
}

function getCatalogActionSubTypes(catalog: unknown): ActionSubTypeDefinition[] {
  if (!catalog || typeof catalog !== "object") return []
  const subTypes = (catalog as { action_sub_types?: unknown }).action_sub_types
  return Array.isArray(subTypes) ? (subTypes as ActionSubTypeDefinition[]) : []
}

function runNow(payload: { actionId: string; actorType: "player" | "kitty"; actorId: number; args?: Record<string, unknown> }) {
  submitRunNowWithArgs(payload.actionId, countById[payload.actionId] || 1, payload.args, {
    actorType: payload.actorType,
    actorId: payload.actorId
  })
  detailVisible.value = false
}

function enqueue(payload: { actionId: string; actorType: "player" | "kitty"; actorId: number; args?: Record<string, unknown> }) {
  submitEnqueueWithArgs(payload.actionId, countById[payload.actionId] || 1, payload.args, {
    actorType: payload.actorType,
    actorId: payload.actorId
  })
  detailVisible.value = false
}

async function enqueuePlan(payload: {
  steps: Array<{ actionId: string; count: number }>
  actorType: "player" | "kitty"
  actorId: number
  insertMode: "append" | "run_now_like_prepend"
}) {
  const result = await enqueuePlanBatch(payload.steps, {
    actorType: payload.actorType,
    actorId: payload.actorId
  }, {
    insertMode: payload.insertMode
  })
  if (result.enqueuedCount > 0) {
    detailVisible.value = false
  }
}

function actionLabel(item: ActionDefinition): string {
  if (item.nameKey) {
    const localized = t(item.nameKey)
    if (localized !== item.nameKey && localized.trim() !== "") return localized
  }
  return item.id
}

function actionDescription(item: ActionDefinition): string {
  if (!item.descKey) return ""
  const localized = t(item.descKey)
  return localized === item.descKey || localized.trim() === "" ? "" : localized
}

function actionRarityColor(item: ActionDefinition): string {
  const rarityId = String(item.rarity || "").trim() || "common"
  return getRarityDefinition(rarityId)?.color || "#9CA3AF"
}

function openAction(action: ActionDefinition) {
  selectedAction.value = action
  countById[action.id] = countById[action.id] || 1
  detailVisible.value = true
}

function updateSelectedCount(count: number) {
  if (!selectedAction.value) return
  countById[selectedAction.value.id] = count
}

function itemEntries(map?: ActionDefinition["itemReward"] | ActionDefinition["costs"]) {
  return Object.entries(map || {}).map(([itemId, quantity]) => ({ itemId, quantity }))
}

function groupLabel(group: { id: string; nameKey?: string }) {
  if (group.nameKey) {
    const localized = t(group.nameKey)
    if (localized !== group.nameKey && localized.trim() !== "") return localized
  }
  return group.id
}

function resolveGroupSections(groupActions: ActionDefinition[]): {
  hasSections: boolean
  sectionedActions: ActionSection[]
} {
  const equipmentOutputActions: ActionDefinition[] = []
  const otherActions: ActionDefinition[] = []
  for (const action of groupActions) {
    if (!hasEquipmentMainOutput(action)) {
      otherActions.push(action)
      continue
    }
    equipmentOutputActions.push(action)
  }
  if (!equipmentOutputActions.length) {
    return { hasSections: false, sectionedActions: [] }
  }
  const sectionedActions = buildEquipmentSlotSecondarySections<ActionDefinition>({
    items: equipmentOutputActions,
    getItemId: (action) => String(action.mainOutput || "").trim(),
    t
  }).map<ActionSection>((section) => ({
    id: section.id,
    label: section.label,
    actions: section.items,
    subSections: section.subSections?.map((subSection) => ({
      id: subSection.id,
      label: subSection.label,
      actions: subSection.items
    }))
  }))
  const otherSection = sectionedActions.find((section) => section.id === "other")
  if (otherActions.length > 0) {
    if (otherSection) {
      otherSection.actions = [...otherSection.actions, ...otherActions]
    } else {
      sectionedActions.push({
        id: "other",
        label: t("game.itemSectionOther"),
        actions: otherActions
      })
    }
  }
  return { hasSections: true, sectionedActions }
}

function hasEquipmentMainOutput(action: ActionDefinition): boolean {
  const itemId = String(action.mainOutput || "").trim()
  if (!itemId) return false
  return !!getEquipmentTemplateDefinition(itemId)
}
</script>
