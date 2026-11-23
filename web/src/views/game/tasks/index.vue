<template>
  <div class="page-shell">
    <el-card class="app-card app-card--no-border game-page-frame" shadow="never" data-tutorial="tasks-page">
      <template #header>
        <GamePageHeader
          title-key="catVillage.title"
          tutorial-id="tasks"
          :visual-src="tasksVisual.src"
          :visual-tone="tasksVisual.tone"
          visual-key="quest"
        >
          <template #actions>
            <el-button size="small" :loading="refreshing" :disabled="refreshCooldown > 0" @click="handleRefresh">
              {{ refreshCooldown > 0 ? `${refreshCooldown}s` : t("catVillage.refresh") }}
            </el-button>
          </template>
        </GamePageHeader>
      </template>

      <!-- Reputation -->
      <div v-if="reputation" class="mb-4" data-tutorial="tasks-reputation">
        <div class="flex items-center justify-between text-sm mb-1">
          <span>{{ t("catVillage.reputation") }} <el-tag size="small">Lv.{{ reputation.level }}</el-tag></span>
          <span class="app-muted text-xs">
            {{ reputation.levelReputation }} / {{ reputation.nextLevelXp > 0 ? reputation.nextLevelXp : t("catVillage.maxLevel") }}
          </span>
        </div>
        <el-progress
          v-if="reputation.nextLevelXp > 0"
          :percentage="Math.min(100, Math.round((reputation.levelReputation / reputation.nextLevelXp) * 100))"
          :show-text="false"
          size="small"
        />
        <div class="text-xs app-muted mt-1">{{ t("catVillage.totalReputation") }}: {{ reputation.totalReputation }}</div>
      </div>


      <div class="flex items-center justify-between mb-2">
        <span class="text-[12px] app-muted"> {{ t("game.tasksRefreshAt") }}: {{ refreshText }} </span>
      </div>

      <el-tabs v-model="activeTab" data-tutorial="tasks-list">
        <el-tab-pane :label="`${t('catVillage.tabLife')} (${lifeTasks.length})`" name="life">
          <div class="space-y-3 m-0.5">
            <el-card
              v-for="task in sortedLifeTasks"
              :key="task.id"
              class="app-card"
              :class="{ 'commission-card--rumor': task.isRumor }"
            >
              <!-- Title + desc -->
              <div class="mb-2">
                <div class="flex items-center gap-2">
                  <span class="font-medium text-sm">{{ t(`catVillage.commission.${task.commissionId}.name`, task.commissionId) }}</span>
                  <el-tag v-if="task.isRumor" size="small" type="warning" effect="dark">{{ t("catVillage.rumor.badge") }}</el-tag>
                </div>
                <div class="text-xs app-muted mt-0.5 leading-relaxed">{{ t(`catVillage.commission.${task.commissionId}.desc`, '') }}</div>
              </div>

              <div class="commission-card-body">
                <div class="commission-info">
                  <div class="commission-row">
                    <span class="commission-row-label app-muted">{{ t("game.tasksNeed") }}</span>
                    <div class="commission-row-items">
                      <QuickRefWithRequire
                        v-for="cost in task.costs"
                        :key="`${task.id}-cost-${cost.itemId}`"
                        :item-id="cost.itemId"
                        :required-count="cost.quantity"
                      />
                    </div>
                  </div>
                  <div class="commission-row">
                    <span class="commission-row-label app-muted">{{ t("game.tasksReward") }}</span>
                    <div class="commission-row-items">
                      <QuickRef
                        v-for="reward in task.rewardItems"
                        :key="`${task.id}-reward-${reward.itemId}`"
                        :item-id="reward.itemId"
                        :count="reward.count"
                      />
                      <el-tag v-if="task.reputationGrant > 0" size="small" type="warning" effect="plain">
                        +{{ task.reputationGrant }} {{ t("catVillage.reputation") }}
                      </el-tag>
                    </div>
                  </div>
                </div>
                <div class="commission-actions">
                  <el-button type="primary" size="small" :disabled="!canSubmitTask(task)" @click="submitTask(task.id)">
                    {{ t("game.tasksSubmit") }}
                  </el-button>
                  <el-button
                    size="small"
                    :disabled="isRefreshCommissionDisabled(task)"
                    :loading="isTaskRefreshBusy(task.id)"
                    :title="refreshCostHint"
                    @click="refreshTask(task.id)"
                  >
                    {{ t("catVillage.refreshCommission") }}
                  </el-button>
                  <el-popconfirm
                    v-if="!task.notDeletable"
                    :title="t('catVillage.confirmDelete')"
                    :confirm-button-text="t('catVillage.deleteCommission')"
                    @confirm="deleteTask(task.id)"
                  >
                    <template #reference>
                      <el-button size="small" type="danger" plain>{{ t("catVillage.deleteCommission") }}</el-button>
                    </template>
                  </el-popconfirm>
                </div>
              </div>
            </el-card>
          </div>
          <div v-if="!lifeTasks.length" class="text-sm app-muted mt-2">{{ t("game.tasksEmpty") }}</div>
        </el-tab-pane>

        <el-tab-pane :label="`${t('catVillage.tabBattle')} (${battleTasks.length})`" name="battle">
          <div class="space-y-3 m-0.5">
            <el-card
              v-for="task in sortedBattleTasks"
              :key="task.id"
              class="app-card"
            >
              <!-- Title + desc -->
              <div class="mb-2">
                <div class="flex items-center gap-2">
                  <span class="font-medium text-sm">{{ t(`catVillage.commission.${task.commissionId}.name`, task.commissionId) }}</span>
                </div>
                <div class="text-xs app-muted mt-0.5 leading-relaxed">{{ t(`catVillage.commission.${task.commissionId}.desc`, '') }}</div>
              </div>

              <div class="commission-card-body">
                <div class="commission-info">
                  <div class="commission-row">
                    <span class="commission-row-label app-muted">{{ t("game.tasksNeed") }}</span>
                    <div class="commission-row-items">
                      <QuickRefWithRequire
                        v-for="cost in task.costs"
                        :key="`${task.id}-cost-${cost.itemId}`"
                        :item-id="cost.itemId"
                        :required-count="cost.quantity"
                      />
                    </div>
                  </div>
                  <div class="commission-row">
                    <span class="commission-row-label app-muted">{{ t("game.tasksReward") }}</span>
                    <div class="commission-row-items">
                      <QuickRef
                        v-for="reward in task.rewardItems"
                        :key="`${task.id}-reward-${reward.itemId}`"
                        :item-id="reward.itemId"
                        :count="reward.count"
                      />
                      <el-tag v-if="task.reputationGrant > 0" size="small" type="warning" effect="plain">
                        +{{ task.reputationGrant }} {{ t("catVillage.reputation") }}
                      </el-tag>
                    </div>
                  </div>
                </div>
                <div class="commission-actions">
                  <el-button type="primary" size="small" :disabled="!canSubmitTask(task)" @click="submitTask(task.id)">
                    {{ t("game.tasksSubmit") }}
                  </el-button>
                  <el-button
                    size="small"
                    :disabled="isRefreshCommissionDisabled(task)"
                    :loading="isTaskRefreshBusy(task.id)"
                    :title="refreshCostHint"
                    @click="refreshTask(task.id)"
                  >
                    {{ t("catVillage.refreshCommission") }}
                  </el-button>
                  <el-popconfirm
                    v-if="!task.notDeletable"
                    :title="t('catVillage.confirmDelete')"
                    :confirm-button-text="t('catVillage.deleteCommission')"
                    @confirm="deleteTask(task.id)"
                  >
                    <template #reference>
                      <el-button size="small" type="danger" plain>{{ t("catVillage.deleteCommission") }}</el-button>
                    </template>
                  </el-popconfirm>
                </div>
              </div>
            </el-card>
          </div>
          <div v-if="!battleTasks.length" class="text-sm app-muted mt-2">{{ t("game.tasksEmpty") }}</div>
        </el-tab-pane>
      </el-tabs>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from "vue"
import { ElMessage } from "element-plus"
import { useI18n } from "vue-i18n"
import { useGameStore } from "@stores/game"
import { useInventoryStore } from "@stores/inventory"
import { apiGet, apiPost, apiDelete } from "@services/api"
import { resolveErrorMessage } from "@services/error-messages"
import { getLocalizedItemName } from "@services/item-label"
import { useInventorySummary } from "@/composables/game/useInventorySummary"
import QuickRef from "@components/game/item/QuickRef.vue"
import QuickRefWithRequire from "@components/game/item/QuickRefWithRequire.vue"
import GamePageHeader from "@components/game/common/GamePageHeader.vue"
import { gamePageVisuals } from "@services/game-page-visuals"

type CatVillageTask = {
  id: number
  commissionId: string
  costs: Array<{ itemId: string; quantity: number }>
  rewardItems: Array<{ itemId: string; count: number }>
  reputationGrant: number
  refreshed: boolean
  isRumor?: boolean
  isBattle?: boolean
  rumorBadge?: string
  notRefreshable?: boolean
  notDeletable?: boolean
  pityThreshold?: number
  pityCounter?: number
}

type CatVillageReputationState = {
  totalReputation: number
  level: number
  levelReputation: number
  nextLevelXp: number
}

type CatVillageCommissionsResponse = {
  tasks: CatVillageTask[]
  nextRefreshAt: string
  reputation: CatVillageReputationState
  refreshCost: Record<string, number>
}

const { t } = useI18n()
const tasksVisual = gamePageVisuals.tasks
const store = useGameStore()
const inventoryStore = useInventoryStore()
const { getInventoryQuantity } = useInventorySummary()
const tasks = ref<CatVillageTask[]>([])
const activeTab = ref<'life' | 'battle'>('life')

const lifeTasks = computed(() => tasks.value.filter((t) => !t.isBattle))
const battleTasks = computed(() => tasks.value.filter((t) => t.isBattle))

// Sort: rumors first, then claimable, then rest; within each group newest-id first
const sortedLifeTasks = computed(() => {
  const rank = (task: CatVillageTask): number => {
    if (task.isRumor) return 0
    if (canSubmitTask(task)) return 1
    return 2
  }
  return [...lifeTasks.value].sort((a, b) => {
    const diff = rank(a) - rank(b)
    if (diff !== 0) return diff
    return b.id - a.id
  })
})

const sortedBattleTasks = computed(() => {
  return [...battleTasks.value].sort((a, b) => {
    const claimA = canSubmitTask(a) ? 0 : 1
    const claimB = canSubmitTask(b) ? 0 : 1
    const diff = claimA - claimB
    if (diff !== 0) return diff
    return b.id - a.id
  })
})
const nextRefreshAt = ref("")
const reputation = ref<CatVillageReputationState | null>(null)
const refreshCost = ref<Record<string, number>>({})
const refreshing = ref(false)
const refreshCooldown = ref(0)
const refreshingTaskIDs = ref<Record<number, boolean>>({})
let cooldownTimer: ReturnType<typeof setInterval> | null = null

const refreshText = computed(() => {
  if (!nextRefreshAt.value) return "-"
  return new Date(nextRefreshAt.value).toLocaleString()
})

const refreshCostHint = computed(() => {
  const parts = Object.entries(refreshCost.value)
    .map(([itemId, qty]) => `${getLocalizedItemName(t, itemId, { fallbackText: itemId })} ×${qty}`)
    .join("、")
  return t("catVillage.refreshCostHint", { cost: parts || "—" })
})

onMounted(() => {
  store.restoreToken()
  if (store.token) void inventoryStore.initialize(store.token)
  void loadTasks()
})

async function loadTasks() {
  store.restoreToken()
  if (!store.token) return
  const data = await apiGet<CatVillageCommissionsResponse>("/api/cat-village/commissions", store.token)
  tasks.value = data.tasks
  nextRefreshAt.value = data.nextRefreshAt
  reputation.value = data.reputation ?? null
  refreshCost.value = data.refreshCost ?? {}
}

async function handleRefresh() {
  if (refreshing.value || refreshCooldown.value > 0) return
  refreshing.value = true
  try { await loadTasks() } catch (err) {
    ElMessage.error(resolveErrorMessage(t, err, "game.tasksSubmitFailed"))
  } finally {
    refreshing.value = false
    startCooldown(5)
  }
}

function startCooldown(seconds: number) {
  refreshCooldown.value = seconds
  if (cooldownTimer) clearInterval(cooldownTimer)
  cooldownTimer = setInterval(() => {
    refreshCooldown.value -= 1
    if (refreshCooldown.value <= 0) {
      refreshCooldown.value = 0
      if (cooldownTimer) clearInterval(cooldownTimer)
    }
  }, 1000)
}

async function submitTask(taskID: number) {
  if (!store.token) return
  try {
    await apiPost<{ ok: boolean }>(`/api/cat-village/commissions/${taskID}/submit`, store.token)
    ElMessage.success(t("game.tasksSubmitSuccess"))
    await loadTasks()
  } catch (err) {
    ElMessage.error(resolveErrorMessage(t, err, "game.tasksSubmitFailed"))
  }
}

async function deleteTask(taskID: number) {
  if (!store.token) return
  try {
    await apiDelete(`/api/cat-village/commissions/${taskID}`, store.token)
    tasks.value = tasks.value.filter((t) => t.id !== taskID)
  } catch (err) {
    ElMessage.error(resolveErrorMessage(t, err, "game.tasksSubmitFailed"))
  }
}

async function refreshTask(taskID: number) {
  if (!store.token || refreshingTaskIDs.value[taskID]) return
  refreshingTaskIDs.value = { ...refreshingTaskIDs.value, [taskID]: true }
  try {
    const res = await apiPost<{ task: CatVillageTask }>(`/api/cat-village/commissions/${taskID}/refresh`, store.token)
    const idx = tasks.value.findIndex((t) => t.id === taskID)
    if (idx >= 0 && res.task) tasks.value.splice(idx, 1, res.task)
    void inventoryStore.initialize(store.token)
  } catch (err) {
    ElMessage.error(resolveErrorMessage(t, err, "game.tasksSubmitFailed"))
  } finally {
    const next = { ...refreshingTaskIDs.value }
    delete next[taskID]
    refreshingTaskIDs.value = next
  }
}

function canSubmitTask(task: CatVillageTask): boolean {
  return task.costs.every((cost) => getInventoryQuantity(cost.itemId) >= cost.quantity)
}

function isTaskRefreshBusy(taskID: number): boolean {
  return Boolean(refreshingTaskIDs.value[taskID])
}

function isRefreshCommissionDisabled(task: CatVillageTask): boolean {
  return task.refreshed || Boolean(task.notRefreshable) || isTaskRefreshBusy(task.id)
}
</script>

<style scoped>
.commission-card-body {
  display: flex;
  align-items: flex-start;
  gap: 12px;
  flex-wrap: wrap;
}
.commission-info {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.commission-row {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}
.commission-row-label {
  font-size: 12px;
  flex-shrink: 0;
}
.commission-row-items {
  display: flex;
  align-items: center;
  gap: 4px;
  flex-wrap: wrap;
}
.commission-actions {
  display: flex;
  align-items: center;
  gap: 4px;
  flex-shrink: 0;
  flex-wrap: wrap;
}
@media (max-width: 480px) {
  .commission-card-body {
    flex-direction: column;
  }
  .commission-actions {
    width: 100%;
    justify-content: flex-end;
  }
}
:deep(.commission-card--rumor) {
  border-color: var(--el-color-warning);
  box-shadow: 0 0 0 1px var(--el-color-warning-light-5);
}
</style>
