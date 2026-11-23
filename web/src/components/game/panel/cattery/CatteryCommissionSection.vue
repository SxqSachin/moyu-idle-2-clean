<template>
  <div class="space-y-3">
    <div class="rounded-xl border border-[var(--app-border)] bg-[var(--app-surface-elevated)] p-3">
      <div class="flex flex-wrap items-center justify-between gap-2">
        <div>
          <div class="font-semibold">{{ t("game.catteryCommissionTitle") }}</div>
          <div class="mt-1 text-xs app-muted">
            {{ t("game.catteryCommissionLimit", { used: usedToday, limit: dailyLimit }) }}
          </div>
        </div>
        <el-select v-model="selectedKittyId" class="w-44" size="small" :disabled="!kitties.length">
          <el-option
            v-for="kitty in kitties"
            :key="kitty.id"
            :label="kitty.name || `#${kitty.id}`"
            :value="Number(kitty.id || 0)"
          />
        </el-select>
      </div>
    </div>

    <div v-if="!kitties.length" class="rounded-xl border border-dashed border-[var(--app-border)] p-4 app-muted">
      {{ t("game.catteryNoKitty") }}
    </div>

    <div v-else class="grid grid-cols-1 gap-3 lg:grid-cols-2">
      <div class="space-y-2">
        <div class="font-semibold">{{ t("game.catteryCommissionAvailable") }}</div>
        <div v-if="loadingCatalog" class="app-muted">{{ t("game.catteryCommissionLoading") }}</div>
        <div v-else-if="!catalog.length" class="rounded-xl border border-dashed border-[var(--app-border)] p-4 app-muted">
          {{ t("game.catteryCommissionEmpty") }}
        </div>
        <div
          v-for="item in catalog"
          :key="item.id"
          class="rounded-xl border border-[var(--app-border)] bg-[var(--app-surface-elevated)] p-3"
          :style="commissionCardStyle(item.rarity)"
        >
          <div class="flex items-start justify-between gap-2">
            <div>
              <div class="flex flex-wrap items-center gap-2">
                <div class="font-medium">{{ t(item.nameKey) }}</div>
                <RarityText :rarity-id="item.rarity || 'common'" />
              </div>
              <div class="mt-1 text-xs app-muted">{{ t(item.descKey) }}</div>
            </div>
            <el-button size="small" type="primary" :loading="startingId === item.id" @click="startCommission(item.id)">
              {{ t("game.catteryCommissionStart") }}
            </el-button>
          </div>
          <div class="mt-2 flex flex-wrap items-center gap-2 text-xs app-muted">
            <span>Lv.{{ item.minKittyLevel }}</span>
            <span>{{ formatDuration(item.durationSeconds) }}</span>
            <span>{{ t("game.catteryCommissionCost") }}:</span>
            <template v-if="costEntries(item.costs).length">
              <QuickRefWithRequire
                v-for="entry in costEntries(item.costs)"
                :key="`${item.id}-cost-${entry.itemId}`"
                :item-id="entry.itemId"
                :required-count="entry.quantity"
              />
            </template>
            <span v-else>-</span>
          </div>
        </div>
      </div>

      <div class="space-y-2">
        <div class="font-semibold">{{ t("game.catteryCommissionRuns") }}</div>
        <div v-if="loadingRuns" class="app-muted">{{ t("game.catteryCommissionLoading") }}</div>
        <div v-else-if="!runs.length" class="rounded-xl border border-dashed border-[var(--app-border)] p-4 app-muted">
          {{ t("game.catteryCommissionNoRuns") }}
        </div>
        <div
          v-for="run in runs"
          :key="run.id"
          class="rounded-xl border border-[var(--app-border)] bg-[var(--app-surface-elevated)] p-3"
        >
          <div class="flex items-start justify-between gap-2">
            <div>
              <div class="font-medium">{{ commissionName(run.commissionId) }}</div>
              <div class="mt-1 text-xs app-muted">{{ statusLabel(run) }}</div>
            </div>
            <el-button
              v-if="run.status === 'running'"
              size="small"
              :disabled="!isReady(run)"
              :loading="claimingId === run.id"
              @click="claimCommission(run.id)"
            >
              {{ t("game.catteryCommissionClaim") }}
            </el-button>
          </div>
          <div v-if="rewardText(run.reward)" class="mt-2 text-xs app-muted">
            {{ rewardText(run.reward) }}
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from "vue"
import { ElMessage } from "element-plus"
import { useI18n } from "vue-i18n"
import { apiGet, apiPost } from "@services/api"
import { useGameStore } from "@stores/game"
import { usePlayerStore } from "@stores/player"
import { resolveErrorMessage } from "@services/error-messages"
import QuickRefWithRequire from "@components/game/item/QuickRefWithRequire.vue"
import RarityText from "@components/game/common/RarityText.vue"
import { getRarityDefinition } from "@services/item-definition"

type CommissionItem = {
  id: string
  rarity: string
  nameKey: string
  descKey: string
  resultKey: string
  minKittyLevel: number
  durationSeconds: number
  costs?: Record<string, number>
}

type CommissionRun = {
  id: number
  commissionId: string
  status: string
  reward?: Record<string, unknown>
  startedAt?: string
  readyAt?: string
  claimedAt?: string
}

const { t } = useI18n()
const gameStore = useGameStore()
const playerStore = usePlayerStore()
const selectedKittyId = ref(0)
const catalog = ref<CommissionItem[]>([])
const runs = ref<CommissionRun[]>([])
const loadingCatalog = ref(false)
const loadingRuns = ref(false)
const startingId = ref("")
const claimingId = ref(0)
const dailyLimit = ref(0)
const usedToday = ref(0)

const kitties = computed(() => playerStore.kitties || [])

watch(
  () => kitties.value.map((kitty) => Number(kitty?.id || 0)).filter((id) => id > 0),
  (ids) => {
    if (!ids.length) {
      selectedKittyId.value = 0
      return
    }
    if (!ids.includes(selectedKittyId.value)) selectedKittyId.value = ids[0]
  },
  { immediate: true }
)

watch(
  () => [selectedKittyId.value, gameStore.token] as const,
  () => {
    void loadCatalog()
    void loadRuns()
  },
  { immediate: true }
)

async function loadCatalog() {
  if (!gameStore.token || selectedKittyId.value <= 0) {
    catalog.value = []
    return
  }
  loadingCatalog.value = true
  try {
    const resp = await apiGet<{ items?: CommissionItem[]; dailyLimit?: number; usedToday?: number }>(
      `/api/cattery/commissions/catalog?kittyId=${encodeURIComponent(String(selectedKittyId.value))}`,
      gameStore.token
    )
    catalog.value = Array.isArray(resp.items) ? resp.items : []
    dailyLimit.value = Number(resp.dailyLimit || 0)
    usedToday.value = Number(resp.usedToday || 0)
  } catch (err) {
    catalog.value = []
    ElMessage.error(resolveErrorMessage(t, err, "game.catteryCommissionLoadFailed"))
  } finally {
    loadingCatalog.value = false
  }
}

async function loadRuns() {
  if (!gameStore.token || selectedKittyId.value <= 0) {
    runs.value = []
    return
  }
  loadingRuns.value = true
  try {
    const resp = await apiGet<{ items?: CommissionRun[] }>(
      `/api/cattery/commissions/runs?kittyId=${encodeURIComponent(String(selectedKittyId.value))}`,
      gameStore.token
    )
    runs.value = Array.isArray(resp.items) ? resp.items : []
  } catch (err) {
    runs.value = []
    ElMessage.error(resolveErrorMessage(t, err, "game.catteryCommissionLoadFailed"))
  } finally {
    loadingRuns.value = false
  }
}

async function startCommission(id: string) {
  if (!gameStore.token || selectedKittyId.value <= 0) return
  startingId.value = id
  try {
    const resp = await apiPost<{ kitties?: unknown[] }>("/api/cattery/commissions/start", gameStore.token, {
      kittyId: selectedKittyId.value,
      commissionId: id
    })
    if (Array.isArray(resp.kitties)) playerStore.patchFromRequest({ kitties: resp.kitties })
    ElMessage.success(t("game.catteryCommissionStartSuccess"))
    await Promise.all([loadCatalog(), loadRuns()])
  } catch (err) {
    ElMessage.error(resolveErrorMessage(t, err, "game.catteryCommissionStartFailed"))
  } finally {
    startingId.value = ""
  }
}

async function claimCommission(id: number) {
  if (!gameStore.token) return
  claimingId.value = id
  try {
    const resp = await apiPost<{ kitties?: unknown[]; reward?: Record<string, unknown> }>("/api/cattery/commissions/claim", gameStore.token, { runId: id })
    if (Array.isArray(resp.kitties)) playerStore.patchFromRequest({ kitties: resp.kitties })
    ElMessage.success(rewardText(resp.reward) || t("game.catteryCommissionClaimSuccess"))
    await Promise.all([loadCatalog(), loadRuns()])
  } catch (err) {
    ElMessage.error(resolveErrorMessage(t, err, "game.catteryCommissionClaimFailed"))
  } finally {
    claimingId.value = 0
  }
}

function commissionName(id: string) {
  const item = catalog.value.find((row) => row.id === id)
  return item ? t(item.nameKey) : id
}

function costEntries(costs: Record<string, number> | undefined) {
  return Object.entries(costs || {})
    .map(([itemId, quantity]) => ({ itemId, quantity: Math.max(0, Math.floor(Number(quantity || 0))) }))
    .filter((entry) => entry.itemId && entry.quantity > 0)
}

function formatDuration(seconds: number) {
  const value = Math.max(0, Math.floor(Number(seconds || 0)))
  if (value < 3600) return `${Math.ceil(value / 60)}m`
  return `${Math.floor(value / 3600)}h ${Math.ceil((value % 3600) / 60)}m`
}

function isReady(run: CommissionRun) {
  const readyAt = new Date(run.readyAt || "")
  return !Number.isNaN(readyAt.getTime()) && readyAt.getTime() <= Date.now()
}

function statusLabel(run: CommissionRun) {
  if (run.status === "claimed") return t("game.catteryCommissionStatusClaimed")
  if (isReady(run)) return t("game.catteryCommissionStatusReady")
  return t("game.catteryCommissionStatusRunning", { time: run.readyAt ? new Date(run.readyAt).toLocaleString() : "-" })
}

function rewardText(reward: Record<string, unknown> | undefined) {
  if (!reward) return ""
  const parts: string[] = []
  const items = Array.isArray(reward.items) ? reward.items : []
  const itemText = items
    .map((item) => {
      if (!item || typeof item !== "object") return ""
      const node = item as Record<string, unknown>
      const itemId = typeof node.itemId === "string" ? node.itemId : ""
      const quantity = Number(node.quantity || 0)
      return itemId && quantity > 0 ? `${itemId} x${quantity}` : ""
    })
    .filter(Boolean)
    .join(", ")
  if (itemText) parts.push(itemText)
  const kittyExp = Number(reward.kittyExp || 0)
  const bondExp = Number(reward.bondExp || 0)
  if (kittyExp > 0) parts.push(`${t("game.catteryKittyExp")} +${kittyExp}`)
  if (bondExp > 0) parts.push(`${t("game.catteryKittyBond")} +${bondExp}`)
  return parts.join(" · ")
}

function commissionCardStyle(rarity: string | undefined) {
  const color = getRarityDefinition(String(rarity || "common").trim())?.color || ""
  return color
    ? {
        borderColor: color,
        boxShadow: `inset 3px 0 0 ${color}`
      }
    : {}
}
</script>
