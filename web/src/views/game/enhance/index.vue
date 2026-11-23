<template>
  <div class="page-shell space-y-4">
    <el-card class="app-card app-card--no-border game-page-frame" shadow="never" data-tutorial="enhance-page">
      <template #header>
        <GamePageHeader
          title-key="game.enhanceDialogTitle"
          tutorial-id="enhance"
          :visual-src="enhanceVisual.src"
          :visual-tone="enhanceVisual.tone"
          visual-key="enhance"
        >
          <template #actions>
            <el-button size="small" @click="refreshAll">{{ t("game.refresh") }}</el-button>
          </template>
        </GamePageHeader>
      </template>

      <el-tabs v-model="activeTab" class="enhance-tabs">
        <el-tab-pane :label="t('game.enhanceTabEnhance')" name="enhance">
          <div class="space-y-4" v-loading="loading">
        <OperationEffectNotice :context="{ domain: 'enhancement', actionId: 'equipment_enhance', actorKind: selectedActor.actorType, actorId: selectedActor.actorId }" />
        <div class="space-y-2" data-tutorial="enhance-target">
          <div class="text-sm">{{ t("game.affixTargetEquipment") }}</div>
          <div class="flex flex-wrap items-center gap-2">
            <Card
              v-if="selectedEntry"
              :item-id="selectedEntry.templateItemId"
              :instance-id="selectedEntry.instanceId"
              :quantity="selectedEntry.quantity"
              @click="selectorVisible = true"
            />
            <EmptyCard v-else :label="t('game.inventorySelectorChoose')" @click="selectorVisible = true" />
          </div>
        </div>

        <template v-if="selectedInstanceId">
          <div class="grid gap-3 md:grid-cols-2" data-tutorial="enhance-settings">
            <el-form-item :label="t('game.enhanceTargetLevel')">
              <el-input-number v-model="targetEnhanceLevel" :min="1" :max="maxLevel" :step="1" :controls="false" />
            </el-form-item>
            <el-form-item :label="t('game.enhanceMaxAttempts')">
              <el-input-number v-model="maxAttempts" :min="1" :max="2000" :step="1" :controls="false" />
            </el-form-item>
            <el-form-item :label="t('game.enhanceUseProtect')">
              <el-switch v-model="useProtect" />
            </el-form-item>
            <el-form-item v-if="useProtect" :label="t('game.enhanceProtectStartLevel')">
              <el-input-number v-model="protectStartLevel" :min="1" :max="maxLevel" :step="1" :controls="false" />
            </el-form-item>
            <el-form-item v-if="useProtect" :label="t('game.enhanceProtectItem')">
              <el-select
                v-model="selectedProtectItemId"
                class="w-full"
                clearable
                :placeholder="t('game.enhanceProtectItemPlaceholder')"
              >
                <el-option
                  v-for="itemId in protectItemOptions"
                  :key="`enhance-protect-${itemId}`"
                  :value="itemId"
                  :label="itemLabel(itemId)"
                />
              </el-select>
              <div v-if="selectedProtectItemId" class="mt-1 text-xs app-muted">
                {{ t("game.enhanceProtectItemOwned", { count: protectItemOwnedCount }) }}
              </div>
            </el-form-item>
          </div>

          <div class="rounded-lg border border-[var(--app-border)] p-3 text-sm space-y-4" data-tutorial="enhance-preview">
            <div>{{ t("game.enhanceLevel") }}: +{{ currentLevel }}</div>
            <div v-if="currentBonusEntries.length" class="space-y-1">
              <div class="text-xs app-muted">{{ t("game.itemDetailEnhanceBonus") }}</div>
              <div class="flex flex-wrap gap-2">
                <StatusTag v-for="row in currentBonusEntries" :key="row.attrKey" :attr-key="row.attrKey" :value="row.value" />
              </div>
            </div>
            <div v-if="nextPreview" class="space-y-1">
              <div class="text-xs app-muted">{{ t("game.itemDetailEnhanceNext") }}</div>
              <div v-if="nextPreviewRateBreakdown" class="text-gray-600">
                {{ formatSuccessRateBreakdown(nextPreviewRateBreakdown) }}
              </div>
            </div>

            <div v-if="nextPreview" class="space-y-1">
              <div class="text-xs app-muted">{{ t("game.itemDetailEnhanceRequireResourceInfo") }}</div>
              <div class="flex flex-wrap gap-2">
                <template v-for="row in nextCostEntries" :key="row.itemId">
                  <QuickRefWithRequire :item-id="row.itemId" :required-count="row.qty" />
                </template>
              </div>
            </div>

            <div v-if="nextDeltaEntries.length" class="space-y-1">
              <div class="text-xs app-muted">{{ t("game.enhanceNextDeltaAttrs") }}</div>
              <div class="flex flex-wrap gap-2">
                <StatusTag v-for="row in nextDeltaEntries" :key="row.attrKey" :attr-key="row.attrKey" :value="row.value" />
              </div>
            </div>
          </div>

          <div class="flex flex-wrap gap-2" data-tutorial="enhance-actions">
            <el-select v-model="selectedActorKey" style="min-width: 240px">
              <el-option
                v-for="option in actorOptions"
                :key="option.key"
                :label="option.label"
                :value="option.key"
              >
                <div class="flex items-center gap-1">
                  <span>{{ option.label }}</span>
                  <el-tooltip v-if="option.actorType === 'kitty'" :content="kittyExecutorHint" placement="top">
                    <span class="cursor-help text-[var(--app-muted)]">ⓘ</span>
                  </el-tooltip>
                </div>
              </el-option>
            </el-select>
            <el-button class="ml-0!" type="primary" @click="submit('run')">{{ t("game.actionRunNow") }}</el-button>
            <el-button class="ml-0!" @click="submit('queue')">{{ t("game.actionEnqueue") }}</el-button>
            <el-button class="ml-0!" @click="allLevelDialogVisible = true">{{ t("game.enhanceAllLevelPreview") }}</el-button>
          </div>
        </template>

        <div v-if="filteredLogs.length" class="space-y-2">
          <div class="text-sm font-semibold">{{ t("game.enhanceResult") }}</div>
          <div class="space-y-2">
            <div
              v-for="row in filteredLogs"
              :key="`${row.queueId}-${row.attemptNo}`"
              class="rounded-md border border-[var(--app-border)] p-2 text-xs"
            >
              <div>#{{ row.attemptNo }} · +{{ row.beforeLevel }} → +{{ row.afterLevel }} · {{ row.success ? t("game.enhanceSuccess") : t("game.enhanceFail") }}</div>
              <div>{{ formatSuccessRateBreakdown(resolveResultRateBreakdown(row)) }}</div>
              <div v-if="row.stopReason">{{ t("game.enhanceStopReason") }}: {{ localizeStopReason(row.stopReason) }}</div>
            </div>
          </div>
        </div>
          </div>
        </el-tab-pane>

        <el-tab-pane :label="t('game.enhanceTabQueue')" name="queue">
          <div class="space-y-3">
            <div class="flex flex-wrap items-center justify-between gap-2">
              <div class="text-sm font-semibold">{{ t("game.enhanceQueueTitle") }}</div>
              <el-button size="small" @click="requestActionQueue">{{ t("game.refresh") }}</el-button>
            </div>

            <el-empty v-if="enhanceQueueItems.length === 0" :description="t('game.enhanceQueueEmpty')" />
            <div v-else class="space-y-2">
              <div
                v-for="row in enhanceQueueItems"
                :key="row.id"
                class="rounded-md border border-[var(--app-border)] p-3"
              >
                <div class="flex flex-wrap items-start justify-between gap-3">
                  <div class="flex min-w-0 flex-1 items-start gap-3">
                    <Card
                      v-if="row.display?.enhance?.templateItemId"
                      :item-id="row.display.enhance.templateItemId"
                      :instance-id="row.display.enhance.equipmentInstanceId"
                      :quantity="1"
                      @click="selectQueueEquipment(row)"
                    />
                    <div class="min-w-0 space-y-1">
                      <div class="flex flex-wrap items-center gap-2">
                        <div class="font-medium">{{ queueItemTitle(row) }}</div>
                        <el-tag size="small" effect="plain" :type="queueStatusTagType(row.status)">
                          {{ queueStatusLabel(row.status) }}
                        </el-tag>
                      </div>
                      <div class="text-xs app-muted">
                        {{ t("game.enhanceQueueActor", { actor: queueActorLabel(row) }) }}
                      </div>
                      <div class="text-xs app-muted">
                        {{ t("game.enhanceQueuePosition", { position: queueLanePosition(row) }) }}
                      </div>
                      <div class="text-xs app-muted">
                        {{ t("game.enhanceQueueProgress", { done: row.doneCount, total: row.totalCount }) }}
                      </div>
                      <div v-if="row.display?.enhance" class="flex flex-wrap gap-2 text-xs">
                        <el-tag size="small" effect="plain" type="success">
                          {{ t("game.enhanceQueueTarget", { level: row.display.enhance.targetEnhanceLevel }) }}
                        </el-tag>
                        <el-tag size="small" effect="plain" :type="row.display.enhance.useProtect ? 'warning' : 'info'">
                          {{ queueProtectLabel(row) }}
                        </el-tag>
                      </div>
                    </div>
                  </div>
                  <el-button
                    class="ml-0!"
                    type="primary"
                    size="small"
                    :disabled="row.status !== 'pending'"
                    @click="runEnhanceQueueNow(row.id)"
                  >
                    {{ t("game.queueRunNow") }}
                  </el-button>
                </div>
              </div>
            </div>
          </div>
        </el-tab-pane>
      </el-tabs>
    </el-card>

    <InventoryInstanceSelectorDialog
      v-model="selectorVisible"
      :title="t('game.affixTargetEquipment')"
      :instance-ids="equipmentInstanceIds"
      :selected-instance-id="selectedInstanceId"
      :allow-clear="true"
      :template-item-ids="equipmentTemplateItemIds"
      :template-items-title="t('game.inventorySelectorAvailableItemsTitle', { title: t('game.affixTargetEquipment') })"
      :empty-text="t('game.inventorySelectorNoItems')"
      @confirm="handleSelectEquipment"
    />

    <GameDialog
      v-model="allLevelDialogVisible"
      :title="t('game.enhanceAllLevelPreviewTitle')"
      width="720px"
      destroy-on-close
    >
      <div class="max-h-[60vh] space-y-2 overflow-auto">
        <div
          v-for="row in allLevelPreviews"
          :key="row.level"
          class="space-y-1 rounded-md border border-[var(--app-border)] p-2"
          :class="{ 'border-[var(--el-color-primary)] bg-[color-mix(in_srgb,var(--el-color-primary)_10%,transparent)]': row.level === currentLevel }"
        >
          <div class="flex items-center gap-2 font-medium">
            <span>+{{ row.level }}</span>
            <el-tag v-if="row.isMilestone" size="small" type="warning" effect="plain">{{ t("game.enhanceMilestoneMarker") }}</el-tag>
          </div>
          <div class="text-xs">{{ formatSuccessRateBreakdown(row) }}</div>
          <div class="flex flex-wrap gap-2">
            <QuickRef v-for="cost in row.costs" :key="`${row.level}-${cost.itemId}`" :item-id="cost.itemId" :count="cost.qty" />
          </div>
          <div v-if="row.deltaAttrs.length" class="flex flex-wrap gap-2">
            <StatusTag v-for="delta in row.deltaAttrs" :key="`${row.level}-${delta.attrKey}`" :attr-key="delta.attrKey" :value="delta.value" />
          </div>
          <div v-if="row.milestoneAttrs.length" class="space-y-1">
            <div class="text-xs app-muted">{{ t("game.enhanceMilestoneAttrs") }}</div>
            <div class="flex flex-wrap gap-2">
              <StatusTag v-for="delta in row.milestoneAttrs" :key="`milestone-attr-${row.level}-${delta.attrKey}`" :attr-key="delta.attrKey" :value="delta.value" />
            </div>
          </div>
          <div v-if="row.milestoneAffixes.length" class="space-y-1">
            <div class="text-xs app-muted">{{ t("game.enhanceMilestoneAffixes") }}</div>
            <div class="flex flex-wrap items-center gap-2">
              <div v-for="entry in row.milestoneAffixes" :key="entry.key" class="inline-flex items-center gap-1">
                <AffixQuickRef :affix-id="entry.affixId" :slot="entry.slot" />
                <span class="text-xs app-muted">{{ t(getRarityNameKey(entry.rarityId)) }}</span>
                <span v-if="entry.rollValue !== null" class="text-xs app-muted">
                  {{ t("game.enhanceMilestoneRollValue", { value: formatRollValue(entry.rollValue) }) }}
                </span>
              </div>
            </div>
          </div>
        </div>
      </div>
    </GameDialog>
  </div>
</template>

<script setup lang="ts">
import OperationEffectNotice from "@components/game/operation-effect/OperationEffectNotice.vue"
import { useOperationEffects } from "@composables/game/useOperationEffects"
import { computed, onMounted, ref, watch } from "vue"
import { ElMessage } from "element-plus"
import { useI18n } from "vue-i18n"
import { useRoute, useRouter } from "vue-router"
import { resolveErrorMessage } from "@services/error-messages"
import { getEquipmentTemplateDefinition, getRarityNameKey } from "@services/item-definition"
import { getLocalizedItemName } from "@services/item-label"
import { send } from "@services/ws"
import { buildEnhancePreview } from "@services/equipment-enhance"
import { useActionSubmit } from "@composables/game/action/useActionSubmit"
import { useGameStore } from "@stores/game"
import { useInventoryStore } from "@stores/inventory"
import { useEquipmentStore } from "@stores/equipment"
import { useRealtimeStore } from "@stores/realtime"
import { usePlayerStore } from "@stores/player"
import { useItemInstanceDetailStore } from "@stores/item-instance-detail"
import { KITTY_STATUS_EXP_MULTIPLIER } from "@/config/shared-global-config"
import Card from "@components/game/item/Card.vue"
import EmptyCard from "@components/game/item/EmptyCard.vue"
import QuickRef from "@components/game/item/QuickRef.vue"
import AffixQuickRef from "@components/game/affix/AffixQuickRef.vue"
import InventoryInstanceSelectorDialog from "@components/game/inventory/InventoryInstanceSelectorDialog.vue"
import StatusTag from "@components/game/attribute/StatusTag.vue"
import type { ItemDetailResponse } from "@/types/equipment"
import QuickRefWithRequire from "@/components/game/item/QuickRefWithRequire.vue"
import type { RealtimeQueueItem } from "@stores/realtime"
import GameDialog from "@components/game/common/GameDialog.vue"
import GamePageHeader from "@components/game/common/GamePageHeader.vue"
import { gamePageVisuals } from "@services/game-page-visuals"

const { t } = useI18n()
const enhanceVisual = gamePageVisuals.enhance
const route = useRoute()
const router = useRouter()
const gameStore = useGameStore()
const inventoryStore = useInventoryStore()
const equipmentStore = useEquipmentStore()
const realtimeStore = useRealtimeStore()
const playerStore = usePlayerStore()
const instanceDetailStore = useItemInstanceDetailStore()
const { submit: submitAction } = useActionSubmit()

const loading = ref(false)
const selectorVisible = ref(false)
const selectedInstanceId = ref<number | null>(null)
const detail = ref<ItemDetailResponse | null>(null)
const targetEnhanceLevel = ref(1)
const maxAttempts = ref(10)
const useProtect = ref(false)
const protectStartLevel = ref(1)
const selectedProtectItemId = ref("")
const allLevelDialogVisible = ref(false)
const selectedActorKey = ref("player:0")
const activeTab = ref<"enhance" | "queue">("enhance")

const equipmentInstanceIds = computed(() =>
  inventoryStore.instances
    .filter((row) => row.state === "inventory" && row.quantity > 0 )
    .filter(row => {
      const eq = getEquipmentTemplateDefinition(row.templateItemId);

      return (eq?.enhanceMaxLevel ?? 0) > 0;
    })
    .map((row) => row.instanceId)
)

const equipmentTemplateItemIds = computed(() => {
  const seen = new Set<string>()
  const out: string[] = []
  for (const row of inventoryStore.instances) {
    if (row.state !== "inventory" || row.quantity <= 0) continue
    if ((getEquipmentTemplateDefinition(row.templateItemId)?.enhanceMaxLevel ?? 0) <= 0) continue
    if (seen.has(row.templateItemId)) continue
    seen.add(row.templateItemId)
    out.push(row.templateItemId)
  }
  return out
})

const selectedEntry = computed(() => inventoryStore.instances.find((row) => row.instanceId === selectedInstanceId.value) || null)
const protectItemOptions = computed(() => {
  const template = selectedEntry.value ? getEquipmentTemplateDefinition(selectedEntry.value.templateItemId) : undefined
  if (!template) return []
  const seen = new Set<string>()
  const out: string[] = []
  for (const itemId of template.enhanceProtectItemId || []) {
    const normalized = String(itemId || "").trim()
    if (!normalized || seen.has(normalized)) continue
    seen.add(normalized)
    out.push(normalized)
  }
  return out
})
const protectItemOwnedCount = computed(() => {
  if (!selectedProtectItemId.value) return 0
  return inventoryStore.instances
    .filter((row) => row.state === "inventory" && row.templateItemId === selectedProtectItemId.value)
    .reduce((sum, row) => sum + Number(row.quantity || 0), 0)
})
const currentLevel = computed(() => Number(detail.value?.instance?.enhanceLevel || selectedEntry.value?.enhanceLevel || 0))
const maxLevel = computed(() => Math.max(1, Number(detail.value?.enhance?.maxLevel || currentLevel.value + 1)))
const nextPreview = computed(() => detail.value?.enhance?.nextPreview || null)
const { enhancementSuccessBonusFor } = useOperationEffects()
const enhancementSuccessBonus = computed(() => enhancementSuccessBonusFor({ actorKind: selectedActor.value.actorType, actorId: selectedActor.value.actorId }))
const enhancementSuccessRateBonus = computed(() => {
  const raw = Number(realtimeStore.attributes.enhancement_success_rate || 0)
  if (!Number.isFinite(raw) || raw <= 0) return 0
  return raw
})
const nextPreviewRateBreakdown = computed(() => {
  if (!nextPreview.value) return null
  const hasSplitFields = nextPreview.value.baseSuccessRate !== undefined || nextPreview.value.successRateBonus !== undefined
  return buildRateBreakdown(
    Number(nextPreview.value.baseSuccessRate ?? nextPreview.value.successRate ?? 0),
    Number(hasSplitFields ? (nextPreview.value.successRateBonus ?? 0) : 0) + enhancementSuccessBonus.value
  )
})
const currentBonusEntries = computed(() =>
  Object.entries(detail.value?.enhance?.cumulativeAttrs || {})
    .filter(([, value]) => Number.isFinite(value) && Number(value) !== 0)
    .map(([attrKey, value]) => ({ attrKey, value: Number(value) }))
)
const nextCostEntries = computed(() =>
  Object.entries(nextPreview.value?.costs || {})
    .filter(([, qty]) => Number(qty) > 0)
    .map(([itemId, qty]) => ({ itemId, qty: Number(qty) }))
)
const nextDeltaEntries = computed(() =>
  Object.entries(nextPreview.value?.deltaAttrs || {})
    .filter(([, value]) => Number.isFinite(value) && Number(value) !== 0)
    .map(([attrKey, value]) => ({ attrKey, value: Number(value) }))
)
const allLevelPreviews = computed(() => {
  const template = selectedEntry.value ? getEquipmentTemplateDefinition(selectedEntry.value.templateItemId) : undefined
  if (!template || template.enhanceMaxLevel <= 0) return []
  const out: Array<{
    level: number
    isMilestone: boolean
    baseSuccessRate: number
    successRateBonus: number
    successRate: number
    costs: Array<{ itemId: string; qty: number }>
    deltaAttrs: Array<{ attrKey: string; value: number }>
    milestoneAttrs: Array<{ attrKey: string; value: number }>
    milestoneAffixes: Array<{
      key: string
      affixId: string
      rarityId: string
      rollValue: number | null
      slot: {
        slotIndex: number
        affixId: string
        rarityId: string
        rollValue?: number
      }
    }>
  }> = []
  for (let level = 1; level <= template.enhanceMaxLevel; level += 1) {
    const preview = buildEnhancePreview(template, level - 1, detail.value?.item?.rarity)
    const rateBreakdown = buildRateBreakdown(
      Number(preview?.successRate || 0),
      enhancementSuccessRateBonus.value
    )
    const milestoneAttrs = Object.entries(template.enhanceAttrCurve?.overrides?.[String(level)] || {})
      .filter(([, value]) => Number.isFinite(value) && Number(value) !== 0)
      .map(([attrKey, value]) => ({ attrKey, value: Number(value) }))
    const milestoneAffixes = (template.enhanceAffixMilestonePools?.[String(level)] || [])
      .map((entry, index) => {
        const affixId = String(entry.affixId || "").trim()
        const rarityId = String(entry.rarityId || "").trim()
        if (!affixId || !rarityId) return null
        const rollValue = Number.isFinite(entry.rollValue) ? Number(entry.rollValue) : null
        return {
          key: `${level}-${index}-${affixId}-${rarityId}`,
          affixId,
          rarityId,
          rollValue,
          slot: {
            slotIndex: index,
            affixId,
            rarityId,
            ...(rollValue === null ? {} : { rollValue })
          }
        }
      })
      .filter((entry): entry is NonNullable<typeof entry> => entry !== null)
    const regularDeltaAttrs = Object.entries(preview?.deltaAttrs || {})
      .filter(([, value]) => Number.isFinite(value) && Number(value) !== 0)
      .map(([attrKey, value]) => ({ attrKey, value: Number(value) }))
    const deltaAttrs = milestoneAttrs.length > 0 ? [] : regularDeltaAttrs
    out.push({
      level,
      isMilestone: milestoneAttrs.length > 0 || milestoneAffixes.length > 0,
      baseSuccessRate: rateBreakdown.base,
      successRateBonus: rateBreakdown.bonus,
      successRate: rateBreakdown.total,
      costs: Object.entries(preview?.costs || {})
        .filter(([, qty]) => Number(qty) > 0)
        .map(([itemId, qty]) => ({ itemId, qty: Number(qty) })),
      deltaAttrs,
      milestoneAttrs,
      milestoneAffixes
    })
  }
  return out
})
const filteredLogs = computed(() =>
  realtimeStore.enhanceResults
    .filter((row) => row.equipmentInstanceId === selectedInstanceId.value)
    .slice()
    .sort((a, b) => b.attemptNo - a.attemptNo)
)
const enhanceQueueItems = computed(() =>
  realtimeStore.actionQueue.filter((row) => row.actionId === "equipment_enhance")
)
const queueLanePositions = computed(() => {
  const counts = new Map<string, number>()
  const positions = new Map<number, number>()
  for (const row of realtimeStore.actionQueue) {
    const laneKey = `${row.actorType}:${row.actorId}`
    const nextPosition = (counts.get(laneKey) || 0) + 1
    counts.set(laneKey, nextPosition)
    positions.set(row.id, nextPosition)
  }
  return positions
})
const actorOptions = computed<Array<{ key: string; label: string; actorType: "player" | "kitty"; actorId: number }>>(() => {
  const options: Array<{ key: string; label: string; actorType: "player" | "kitty"; actorId: number }> = [
    { key: "player:0", label: t("game.executorPlayer"), actorType: "player", actorId: 0 }
  ]
  for (const kitty of playerStore.kitties || []) {
    options.push({
      key: `kitty:${kitty.id}`,
      label: `${t("game.executorKitty")}: ${kitty.name}`,
      actorType: "kitty",
      actorId: kitty.id
    })
  }
  return options
})
const selectedActor = computed(() => actorOptions.value.find((row) => row.key === selectedActorKey.value) || actorOptions.value[0])
const kittyExecutorHint = computed(() => {
  if (KITTY_STATUS_EXP_MULTIPLIER <= 0) {
    return t("game.executorKittyTooltipNoExp")
  }
  return t("game.executorKittyTooltip", {
    expMultiplier: KITTY_STATUS_EXP_MULTIPLIER.toFixed(2)
  })
})

watch(
  () => route.query.instanceId,
  () => {
    const routeInstanceId = parseInstanceIdQuery(route.query.instanceId)
    if (!routeInstanceId) {
      return
    }
    if (!equipmentInstanceIds.value.includes(routeInstanceId)) {
      return
    }
    if (selectedInstanceId.value !== routeInstanceId) {
      selectedInstanceId.value = routeInstanceId
    }
  }
)

watch(
  () => selectedInstanceId.value,
  async (instanceId) => {
    if (!instanceId) {
      detail.value = null
      syncRouteQuery()
      return
    }
    syncRouteQuery()
    syncOperationTarget(instanceId)
    await loadDetail(instanceId)
    targetEnhanceLevel.value = Math.max(currentLevel.value + 1, Math.min(maxLevel.value, targetEnhanceLevel.value))
    protectStartLevel.value = Math.max(1, Math.min(maxLevel.value, protectStartLevel.value))
    selectedProtectItemId.value = ""
  }
)

watch(
  () => useProtect.value,
  (enabled) => {
    if (!enabled) {
      selectedProtectItemId.value = ""
    }
  }
)

watch(actorOptions, (options) => {
  if (!options.some((row) => row.key === selectedActorKey.value)) {
    selectedActorKey.value = "player:0"
  }
})

onMounted(async () => {
  gameStore.restoreToken()
  if (!gameStore.token) return
  requestActionQueue()
  await inventoryStore.initialize(gameStore.token)
  selectedInstanceId.value = resolveInitialSelectedInstanceId()
})

async function loadDetail(instanceId: number) {
  if (!gameStore.token || instanceId <= 0) return
  loading.value = true
  try {
    detail.value = await instanceDetailStore.fetchByInstance(gameStore.token, instanceId, { force: true })
  } catch (err) {
    ElMessage.error(resolveErrorMessage(t, err, "game.itemDetailLoadFailed"))
  } finally {
    loading.value = false
  }
}

async function refreshAll() {
  if (!gameStore.token) return
  requestActionQueue()
  await inventoryStore.initialize(gameStore.token, true)
  const current = selectedInstanceId.value
  if (current && equipmentInstanceIds.value.includes(current)) {
    await loadDetail(selectedInstanceId.value)
    return
  }
  selectedInstanceId.value = resolveInitialSelectedInstanceId()
  if (!selectedInstanceId.value) {
    detail.value = null
    syncRouteQuery()
  }
}

function submit(mode: "run" | "queue") {
  if (!selectedInstanceId.value) return
  const safeTarget = Math.max(currentLevel.value + 1, Math.min(maxLevel.value, Math.round(targetEnhanceLevel.value)))
  const safeAttempts = Math.max(1, Math.round(maxAttempts.value))
  const safeProtectStart = Math.max(1, Math.min(maxLevel.value, Math.round(protectStartLevel.value)))
  const protectItemID = selectedProtectItemId.value.trim()
  if (useProtect.value) {
    if (!protectItemID || !protectItemOptions.value.includes(protectItemID)) {
      ElMessage.warning(t("game.enhanceProtectItemRequired"))
      return
    }
  }
  submitAction("equipment_enhance", mode, 1, {
    equipmentInstanceId: selectedInstanceId.value,
    maxAttempts: safeAttempts,
    targetEnhanceLevel: safeTarget,
    useProtect: !!useProtect.value,
    protectStartLevel: useProtect.value ? safeProtectStart : undefined,
    protectItemId: useProtect.value ? protectItemID : undefined
  }, {
    actorType: selectedActor.value.actorType,
    actorId: selectedActor.value.actorId
  })
}

function handleSelectEquipment(instanceId: number | null) {
  selectedInstanceId.value = instanceId
}

function requestActionQueue() {
  send("action.queue.get", {})
}

function runEnhanceQueueNow(queueID: number) {
  if (!Number.isInteger(queueID) || queueID <= 0) return
  send("action.queue.run_now", { queueId: queueID })
}

function queueItemTitle(row: RealtimeQueueItem): string {
  const enhance = row.display?.enhance
  if (!enhance?.templateItemId) {
    return t("game.queueAction_equipment_enhance")
  }
  return itemLabel(enhance.templateItemId)
}

function queueStatusLabel(status: RealtimeQueueItem["status"]): string {
  const key = `game.queueStatus.${status}`
  const localized = t(key)
  return localized === key ? status : localized
}

function queueStatusTagType(status: RealtimeQueueItem["status"]): "success" | "warning" | "info" | "danger" {
  switch (status) {
    case "running":
      return "warning"
    case "done":
      return "success"
    case "failed":
      return "danger"
    default:
      return "info"
  }
}

function queueActorLabel(row: RealtimeQueueItem): string {
  if (row.actorType !== "kitty") {
    return t("game.executorPlayer")
  }
  const kitty = playerStore.kitties.find((entry) => entry.id === row.actorId)
  return kitty?.name ? t("game.itemOperationTargetKitty", { name: kitty.name }) : t("game.executorKitty")
}

function queueLanePosition(row: RealtimeQueueItem): number {
  return queueLanePositions.value.get(row.id) || 0
}

function queueProtectLabel(row: RealtimeQueueItem): string {
  const enhance = row.display?.enhance
  if (!enhance?.useProtect) {
    return t("game.enhanceQueueProtectDisabled")
  }
  const itemID = String(enhance.protectItemId || "").trim()
  return t("game.enhanceQueueProtectEnabled", {
    level: enhance.protectStartLevel || 1,
    item: itemID ? itemLabel(itemID) : t("game.queueEnhanceProtect")
  })
}

function selectQueueEquipment(row: RealtimeQueueItem) {
  const instanceID = row.display?.enhance?.equipmentInstanceId
  if (!Number.isInteger(instanceID) || !equipmentInstanceIds.value.includes(instanceID)) return
  selectedInstanceId.value = instanceID
  activeTab.value = "enhance"
}

function resolveInitialSelectedInstanceId(): number | null {
  const fromQuery = parseInstanceIdQuery(route.query.instanceId)
  if (fromQuery && equipmentInstanceIds.value.includes(fromQuery)) {
    return fromQuery
  }
  const fromStore = equipmentStore.operationTarget?.instanceId
  if (typeof fromStore === "number" && equipmentInstanceIds.value.includes(fromStore)) {
    return fromStore
  }
  return equipmentInstanceIds.value[0] || null
}

function parseInstanceIdQuery(raw: unknown): number | null {
  const first = Array.isArray(raw) ? raw[0] : raw
  const value = Number(first)
  if (!Number.isInteger(value) || value <= 0) return null
  return value
}

function syncRouteQuery() {
  const nextInstanceID = selectedInstanceId.value
  const currentInstanceID = parseInstanceIdQuery(route.query.instanceId)
  if ((nextInstanceID || null) === (currentInstanceID || null)) return
  const query: Record<string, string> = {}
  for (const [key, value] of Object.entries(route.query)) {
    if (key === "instanceId") continue
    if (typeof value === "string") {
      query[key] = value
      continue
    }
    if (Array.isArray(value) && typeof value[0] === "string") {
      query[key] = value[0]
    }
  }
  if (nextInstanceID) {
    query.instanceId = String(nextInstanceID)
  }
  void router.replace({ path: "/game/enhance", query })
}

function syncOperationTarget(instanceId: number) {
  const selected = inventoryStore.instances.find((row) => row.instanceId === instanceId)
  if (!selected) return
  equipmentStore.setOperationTarget({
    instanceId: selected.instanceId,
    templateItemId: selected.templateItemId
  })
}

function localizeStopReason(reason: string): string {
  const key = `game.enhanceStopReason_${reason}`
  const localized = t(key)
  return localized === key ? reason : localized
}

function itemLabel(itemID: string): string {
  return getLocalizedItemName(t, itemID, { fallbackText: itemID })
}

function clampRate(value: number): number {
  if (!Number.isFinite(value) || value <= 0) return 0
  if (value >= 1) return 1
  return value
}

function formatRatePercent(value: number): number {
  if (!Number.isFinite(value) || value <= 0) return 0
  return +(value * 100).toFixed(2)
}

function formatRollValue(value: number): string {
  if (!Number.isFinite(value)) return "0.00"
  return value.toFixed(2)
}

function buildRateBreakdown(baseRaw: number, bonusRaw: number, totalRaw?: number) {
  const base = clampRate(baseRaw)
  const bonus = Number.isFinite(bonusRaw) && bonusRaw > 0 ? bonusRaw : 0
  const fallbackTotal = clampRate(base + bonus)
  const total = Number.isFinite(totalRaw ?? Number.NaN) ? clampRate(Number(totalRaw)) : fallbackTotal
  return { base, bonus, total }
}

function resolveResultRateBreakdown(row: { baseSuccessRate?: number; successRateBonus?: number; successRate?: number }) {
  const hasSplitFields = row.baseSuccessRate !== undefined || row.successRateBonus !== undefined
  return buildRateBreakdown(
    Number(row.baseSuccessRate ?? row.successRate ?? 0),
    Number(hasSplitFields ? (row.successRateBonus ?? 0) : 0),
    Number(row.successRate ?? Number.NaN)
  )
}

function formatSuccessRateBreakdown(rate: { baseSuccessRate?: number; successRateBonus?: number; successRate?: number; base?: number; bonus?: number; total?: number }) {
  const base = Number(rate.base ?? rate.baseSuccessRate ?? 0)
  const bonus = Number(rate.bonus ?? rate.successRateBonus ?? 0)
  const total = Number(rate.total ?? rate.successRate ?? 0)
  return t("game.enhanceSuccessRateBreakdown", {
    base: formatRatePercent(base),
    bonus: formatRatePercent(bonus),
    total: formatRatePercent(total)
  })
}
</script>
