<template>
  <div class="page-shell">
    <section class="noticeboard-hero">
      <div class="min-w-0">
        <div class="noticeboard-kicker">{{ t("game.publicRisk.kicker") }}</div>
        <h1 class="noticeboard-title">{{ t("game.publicRisk.title") }}</h1>
        <p class="noticeboard-subtitle">{{ t("game.publicRisk.subtitle") }}</p>
      </div>
      <div class="noticeboard-actions">
        <span v-if="activeSnapshot?.generatedAt" class="noticeboard-updated">
          {{ t("game.publicRisk.generatedAt") }}: {{ formatTime(activeSnapshot.generatedAt) }}
        </span>
        <el-button :loading="loading" :disabled="refreshCooldown > 0" @click="refreshActiveTab">
          {{ refreshCooldown > 0 ? `${refreshCooldown}s` : t("game.publicRisk.refresh") }}
        </el-button>
      </div>
      <RichModeOnly>
        <img class="noticeboard-hero__visual" :src="noticeboardVisual.src" alt="" loading="lazy" decoding="async" />
      </RichModeOnly>
    </section>

    <div class="noticeboard-stats">
      <button
        v-for="tab in tabs"
        :key="tab.category"
        type="button"
        class="noticeboard-stat"
        :class="{ 'noticeboard-stat--active': activeTab === tab.category }"
        @click="handleTabChange(tab.category)"
      >
        <span class="noticeboard-stat-label">{{ tab.label }}</span>
        <span class="noticeboard-stat-value">{{ itemCount(tab.category) }}</span>
      </button>
    </div>

    <el-card class="app-card app-card--no-border noticeboard-card" shadow="never">
      <template #header>
        <div class="flex flex-wrap items-center justify-between gap-3">
          <div>
            <div class="font-bold">{{ activeTabTitle }}</div>
            <div class="mt-1 text-xs app-muted">{{ activeTabHint }}</div>
          </div>
          <el-tag size="small" effect="plain">{{ t("game.publicRisk.window") }} {{ activeSnapshot ? `${formatDate(activeSnapshot.windowStartedAt)} - ${formatDate(activeSnapshot.windowEndedAt)}` : "-" }}</el-tag>
        </div>
      </template>

      <el-alert
        v-if="reputationLevel < minReputationLevel"
        class="mb-4"
        type="warning"
        :closable="false"
        :title="t('game.publicRisk.lockedTitle')"
        :description="t('game.publicRisk.lockedDesc', { level: minReputationLevel })"
      />

      <el-alert
        v-else-if="activeSnapshot?.stale"
        class="mb-4"
        type="warning"
        :closable="false"
        :title="t('game.publicRisk.staleTitle')"
        :description="t('game.publicRisk.staleDesc')"
      />

      <el-tabs v-model="activeTab" @tab-change="handleTabChange">
        <el-tab-pane :label="t('game.publicRisk.tabs.abnormalTrade')" name="abnormal_trade" />
        <el-tab-pane :label="t('game.publicRisk.tabs.linkedAccounts')" name="linked_accounts" />
        <el-tab-pane :label="t('game.publicRisk.tabs.banned')" name="banned" />
      </el-tabs>

      <div v-if="loading" class="py-10 text-center app-muted">{{ t("game.publicRisk.loading") }}</div>
      <el-empty
        v-else-if="!activeItems.length"
        :description="emptyDescription"
      />
      <div v-else class="noticeboard-list">
        <div v-for="(item, index) in activeItems" :key="`${activeTab}-${index}-${item.users.map((user) => user.userId).join('-')}`" class="noticeboard-row">
          <div class="noticeboard-rank">
            <span>{{ index + 1 }}</span>
          </div>
          <div class="min-w-0 flex-1">
            <div class="noticeboard-row-head">
              <div class="noticeboard-row-title">
                {{ entryTitle(item) }}
                <span v-if="item.users.length > 1" class="noticeboard-member-count">{{ item.users.length }} {{ t("game.publicRisk.memberCount") }}</span>
              </div>
              <div class="noticeboard-tags">
                <el-tag size="small" :type="statusTagType(item.status)">{{ statusLabel(item.status) }}</el-tag>
                <el-tag size="small" :type="item.confidenceBand === 'confirmed' ? 'danger' : 'warning'">
                  {{ confidenceLabel(item.confidenceBand) }}
                </el-tag>
              </div>
            </div>
            <div class="noticeboard-users" :class="{ 'noticeboard-users--group': item.category === 'linked_accounts' }">
              <UserName
                v-for="user in item.users"
                :key="user.userId"
                :user-id="user.userId"
                :username="user.username"
                class="public-user-button"
              />
            </div>
            <div class="mt-2 text-sm leading-relaxed text-[var(--app-text)]">{{ item.summaryText }}</div>
            <div class="mt-2 flex flex-wrap gap-x-4 gap-y-1 text-xs app-muted">
              <span>{{ t("game.publicRisk.expiresAt") }}: {{ formatTime(item.expiresAt) }}</span>
            </div>
          </div>
        </div>
      </div>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from "vue"
import { ElMessage } from "element-plus"
import { useI18n } from "vue-i18n"
import { APIClientError, apiGet } from "@services/api"
import { useGameStore } from "@stores/game"
import { usePlayerStore } from "@stores/player"
import UserName from "@components/common/UserName.vue"
import RichModeOnly from "@components/game/common/RichModeOnly.vue"
import { gamePageVisuals } from "@services/game-page-visuals"

type PublicRiskCategory = "abnormal_trade" | "linked_accounts" | "banned"
type PublicRiskSnapshotCategory = PublicRiskCategory | "all"

type PublicRiskUser = {
  userId: number
  username: string
}

type PublicRiskEntry = {
  category: PublicRiskCategory
  users: PublicRiskUser[]
  status: string
  confidenceBand: "high" | "confirmed" | string
  summaryText: string
  windowStartedAt: string
  windowEndedAt: string
  generatedAt: string
  expiresAt: string
  stale?: boolean
}

type PublicRiskSnapshot = {
  category: PublicRiskSnapshotCategory
  items: PublicRiskEntry[]
  windowStartedAt: string
  windowEndedAt: string
  generatedAt: string
  expiresAt: string
  stale?: boolean
}

const minReputationLevel = 20
const { t } = useI18n()
const noticeboardVisual = gamePageVisuals.cheaterList
const gameStore = useGameStore()
const playerStore = usePlayerStore()
const activeTab = ref<PublicRiskCategory>("abnormal_trade")
const loading = ref(false)
const snapshots = ref<Partial<Record<PublicRiskCategory, PublicRiskSnapshot>>>({})
const refreshCooldown = ref(0)
let cooldownTimer: ReturnType<typeof setInterval> | null = null

const tabs = computed(() => [
  { category: "abnormal_trade" as const, label: t("game.publicRisk.tabs.abnormalTrade"), hint: t("game.publicRisk.hints.abnormalTrade") },
  { category: "linked_accounts" as const, label: t("game.publicRisk.tabs.linkedAccounts"), hint: t("game.publicRisk.hints.linkedAccounts") },
  { category: "banned" as const, label: t("game.publicRisk.tabs.banned"), hint: t("game.publicRisk.hints.banned") }
])
const reputationLevel = computed(() => Number(playerStore.catVillageReputation.level || 0))
const activeSnapshot = computed(() => snapshots.value[activeTab.value] || null)
const activeItems = computed(() => activeSnapshot.value?.items || [])
const emptyDescription = computed(() => reputationLevel.value < minReputationLevel ? t("game.publicRisk.lockedEmpty") : t("game.publicRisk.empty"))
const activeTabTitle = computed(() => tabs.value.find((tab) => tab.category === activeTab.value)?.label || "")
const activeTabHint = computed(() => tabs.value.find((tab) => tab.category === activeTab.value)?.hint || "")

onMounted(async () => {
  gameStore.restoreToken()
  if (gameStore.token && !playerStore.initialized) {
    try {
      await playerStore.initialize(gameStore.token)
    } catch {
      // The server remains the source of truth for access control.
    }
  }
  if (gameStore.token && reputationLevel.value >= minReputationLevel) {
    await loadAll()
  }
})

async function handleTabChange(name: string | number) {
  const category = normalizeCategory(String(name))
  activeTab.value = category
  if (!snapshots.value[category] && reputationLevel.value >= minReputationLevel && refreshCooldown.value <= 0) {
    await loadAll()
  }
}

async function refreshActiveTab() {
  if (refreshCooldown.value > 0) return
  await loadAll(true)
}

async function loadAll(forceMessage = false) {
  if (!gameStore.token) return
  loading.value = true
  try {
    const payload = await apiGet<PublicRiskSnapshot>("/api/public-risk/noticeboard?category=all", gameStore.token)
    snapshots.value = splitSnapshotByCategory(payload)
    startCooldown(30)
  } catch (err) {
    if (err instanceof APIClientError) {
      if (err.code === "PUBLIC_RISK_REPUTATION_TOO_LOW") {
        ElMessage.warning(t("game.publicRisk.lockedTitle"))
        return
      }
      if (err.code === "PUBLIC_RISK_RATE_LIMITED") {
        const retryAfter = retryAfterFromDebug(err.debug)
        startCooldown(retryAfter > 0 ? retryAfter : 30)
        ElMessage.warning(t("game.publicRisk.rateLimited"))
        return
      }
    }
    if (forceMessage) {
      ElMessage.error(t("game.publicRisk.loadFailed"))
    }
  } finally {
    loading.value = false
  }
}

function splitSnapshotByCategory(payload: PublicRiskSnapshot): Partial<Record<PublicRiskCategory, PublicRiskSnapshot>> {
  const next: Partial<Record<PublicRiskCategory, PublicRiskSnapshot>> = {}
  for (const category of ["abnormal_trade", "linked_accounts", "banned"] as const) {
    next[category] = {
      ...payload,
      category,
      windowStartedAt: categoryWindowStartedAt(payload, category),
      items: (payload.items || []).filter((item) => item.category === category)
    }
  }
  return next
}

function categoryWindowStartedAt(payload: PublicRiskSnapshot, category: PublicRiskCategory): string {
  if (category === "banned") return payload.windowStartedAt
  const endedAt = new Date(payload.windowEndedAt)
  if (!Number.isFinite(endedAt.getTime())) return payload.windowStartedAt
  endedAt.setDate(endedAt.getDate() - 30)
  return endedAt.toISOString()
}

function startCooldown(seconds: number) {
  refreshCooldown.value = Math.max(0, Math.ceil(seconds))
  if (cooldownTimer) clearInterval(cooldownTimer)
  if (refreshCooldown.value <= 0) return
  cooldownTimer = setInterval(() => {
    refreshCooldown.value = Math.max(0, refreshCooldown.value - 1)
    if (refreshCooldown.value <= 0 && cooldownTimer) {
      clearInterval(cooldownTimer)
      cooldownTimer = null
    }
  }, 1000)
}

function retryAfterFromDebug(debug: string | undefined): number {
  if (!debug) return 0
  const match = debug.match(/retryAfterSeconds=(\d+)/)
  return match ? Number(match[1] || 0) : 0
}

function normalizeCategory(value: string): PublicRiskCategory {
  if (value === "linked_accounts" || value === "banned") return value
  return "abnormal_trade"
}

function statusLabel(status: string): string {
  if (status === "handled") return t("game.publicRisk.status.handled")
  if (status === "restricted") return t("game.publicRisk.status.restricted")
  return t("game.publicRisk.status.underReview")
}

function statusTagType(status: string) {
  if (status === "handled") return "danger"
  if (status === "restricted") return "warning"
  return "info"
}

function confidenceLabel(confidence: string): string {
  if (confidence === "confirmed") return t("game.publicRisk.confidence.confirmed")
  return t("game.publicRisk.confidence.high")
}

function itemCount(category: PublicRiskCategory): number {
  return snapshots.value[category]?.items?.length || 0
}

function entryTitle(item: PublicRiskEntry): string {
  if (item.category === "linked_accounts") return t("game.publicRisk.entryTitle.linkedAccounts")
  if (item.category === "banned") return t("game.publicRisk.entryTitle.banned")
  return t("game.publicRisk.entryTitle.abnormalTrade")
}

function formatDate(value: string): string {
  if (!value) return "-"
  return new Date(value).toLocaleDateString()
}

function formatTime(value: string): string {
  if (!value) return "-"
  return new Date(value).toLocaleString()
}

</script>

<style scoped>
.noticeboard-hero {
  position: relative;
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 16px;
  overflow: hidden;
  padding: 18px 20px;
  border-bottom: 1px solid var(--app-border);
  background:
    linear-gradient(135deg, rgba(220, 38, 38, 0.10), transparent 42%),
    linear-gradient(135deg, var(--app-surface), var(--app-surface));
}

.noticeboard-hero > :not(.noticeboard-hero__visual) {
  position: relative;
  z-index: 2;
}

.noticeboard-hero__visual {
  position: absolute;
  left: clamp(180px, 24vw, 360px);
  bottom: -28px;
  z-index: 1;
  width: clamp(132px, 12vw, 184px);
  max-height: 132px;
  object-fit: contain;
  opacity: 0.36;
  pointer-events: none;
  transform: scaleX(-1);
  filter: drop-shadow(0 12px 14px rgba(15, 23, 42, 0.14));
}

.noticeboard-kicker {
  margin-bottom: 4px;
  color: var(--app-muted);
  font-size: 12px;
  font-weight: 700;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.noticeboard-title {
  margin: 0;
  color: var(--app-text);
  font-size: 24px;
  font-weight: 800;
  line-height: 1.2;
}

.noticeboard-subtitle {
  max-width: 720px;
  margin: 6px 0 0;
  color: var(--app-muted);
  font-size: 13px;
  line-height: 1.6;
}

.noticeboard-actions {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: flex-end;
  gap: 10px;
}

.noticeboard-updated {
  color: var(--app-muted);
  font-size: 12px;
}

.noticeboard-stats {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 10px;
  padding: 14px 20px;
  background: var(--app-bg);
}

.noticeboard-stat {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  min-width: 0;
  padding: 12px 14px;
  border: 1px solid var(--app-border);
  border-radius: 8px;
  background: var(--app-surface);
  color: var(--app-text);
  cursor: pointer;
  transition: border-color 0.16s ease, transform 0.16s ease;
}

.noticeboard-stat:hover,
.noticeboard-stat--active {
  border-color: var(--app-primary);
}

.noticeboard-stat:hover {
  transform: translateY(-1px);
}

.noticeboard-stat-label {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--app-muted);
  font-size: 13px;
}

.noticeboard-stat-value {
  font-size: 22px;
  font-weight: 800;
  line-height: 1;
}

.noticeboard-card {
  border-radius: 0;
}

.noticeboard-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.noticeboard-row {
  display: flex;
  gap: 14px;
  padding: 14px;
  border: 1px solid var(--app-border);
  border-radius: 8px;
  background: var(--app-surface);
}

.noticeboard-rank {
  flex: 0 0 auto;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  border-radius: 6px;
  background: rgba(220, 38, 38, 0.10);
  color: #b91c1c;
  font-size: 12px;
  font-weight: 800;
}

.noticeboard-row-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
}

.noticeboard-row-title {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
  min-width: 0;
  font-size: 15px;
  font-weight: 800;
  color: var(--app-text);
}

.noticeboard-member-count {
  color: var(--app-muted);
  font-size: 12px;
  font-weight: 600;
}

.noticeboard-tags {
  display: flex;
  flex: 0 0 auto;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 6px;
}

.noticeboard-users {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-top: 10px;
}

.noticeboard-users--group {
  padding: 10px;
  border: 1px dashed var(--app-border);
  border-radius: 8px;
  background: rgba(0, 0, 0, 0.02);
}

.public-user-button {
  max-width: 220px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  padding: 4px 9px;
  border: 1px solid color-mix(in srgb, var(--app-primary) 35%, transparent);
  border-radius: 999px;
  background: color-mix(in srgb, var(--app-primary) 8%, transparent);
  color: var(--app-primary);
  font-weight: 600;
  cursor: pointer;
}

.public-user-button:hover {
  text-decoration: underline;
}

@media (max-width: 640px) {
  .noticeboard-hero {
    align-items: flex-start;
    flex-direction: column;
    padding: 16px;
  }

  .noticeboard-stats {
    grid-template-columns: 1fr;
    padding: 12px 16px;
  }

  .noticeboard-row {
    gap: 8px;
    padding: 10px;
  }

  .noticeboard-row-head {
    flex-direction: column;
  }

  .public-user-button {
    max-width: 150px;
  }
}
</style>
