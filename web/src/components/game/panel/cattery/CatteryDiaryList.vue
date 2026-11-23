<template>
  <div v-if="loading" class="app-muted">{{ t("game.catteryJournalLoading") }}</div>
  <div v-else-if="!entries.length" class="app-muted">{{ t("game.catteryDiaryEmpty") }}</div>
  <div v-else class="space-y-2">
    <div
      v-for="entry in entries"
      :key="entry.id"
      class="rounded border border-[var(--app-border)] bg-[var(--el-bg-color)] px-3 py-2"
    >
      <div class="font-medium">{{ resolveTitle(entry.titleKey) }}</div>
      <div class="mt-1">
        <CatteryDiaryContent
          :content-key="entry.contentKey"
          :content-params="entry.contentParams"
          :action-id="actionIdFromParams(entry)"
        />
      </div>
      <div class="mt-1 text-xs app-muted">{{ entry.createdAt ? formatDate(entry.createdAt) : "-" }}</div>
      <div v-if="rewardSegments(entry.reward).length" class="mt-1 text-xs app-muted">
        <template v-for="(segment, index) in rewardSegments(entry.reward)" :key="segment.key">
          <span v-if="index > 0" class="mx-1">·</span>
          <ItemQuickRef
            v-if="segment.type === 'item'"
            :item-id="segment.itemId"
            :count="segment.quantity"
            :normal-number-format="true"
          />
          <span v-else>{{ segment.text }}</span>
        </template>
      </div>
      <div v-if="acquiredPersonalityId(entry)" class="mt-1">
        <span class="mr-1 text-xs app-muted">{{ t("game.catteryJournalAcquiredPersonality") }}</span>
        <KittyPersonalityQuickRef :personality-id="acquiredPersonalityId(entry)!" />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, watch } from "vue"
import { ElMessage } from "element-plus"
import { useI18n } from "vue-i18n"
import { apiGet } from "@services/api"
import { useGameStore } from "@stores/game"
import { resolveErrorMessage } from "@services/error-messages"
import CatteryDiaryContent from "./CatteryDiaryContent.vue"
import KittyPersonalityQuickRef from "@components/game/cattery/KittyPersonalityQuickRef.vue"
import ItemQuickRef from "@components/game/item/QuickRef.vue"

type CatteryJournalEntry = {
  id: number
  titleKey?: string
  contentKey: string
  contentParams?: Record<string, unknown>
  reward?: Record<string, unknown>
  extra?: Record<string, unknown>
  createdAt?: string
}

type CatteryPagedResponse<T> = {
  items?: T[]
}

type RewardSegment =
  | { type: "item"; key: string; itemId: string; quantity: number }
  | { type: "text"; key: string; text: string }

const props = defineProps<{
  selectedKittyId: number
}>()

const { t } = useI18n()
const gameStore = useGameStore()
const loading = ref(false)
const entries = ref<CatteryJournalEntry[]>([])

watch(
  () => [props.selectedKittyId, gameStore.token] as const,
  () => void load(),
  { immediate: true }
)

async function load() {
  const kittyId = props.selectedKittyId
  if (!gameStore.token || kittyId <= 0) {
    entries.value = []
    return
  }
  loading.value = true
  try {
    const resp = await apiGet<CatteryPagedResponse<CatteryJournalEntry>>(
      `/api/cattery/journals?kittyId=${encodeURIComponent(String(kittyId))}&page=1&pageSize=20`,
      gameStore.token
    )
    entries.value = Array.isArray(resp?.items) ? resp.items : []
  } catch (err) {
    entries.value = []
    ElMessage.error(resolveErrorMessage(t, err, "game.catteryJournalLoadFailed"))
  } finally {
    loading.value = false
  }
}

function acquiredPersonalityId(entry: CatteryJournalEntry): string | null {
  const v = entry.extra?.acquiredPersonality
  return typeof v === "string" && v.trim() ? v.trim() : null
}

function actionIdFromParams(entry: CatteryJournalEntry): string | undefined {
  const v = entry.contentParams?.actionId
  return typeof v === "string" && v.trim() ? v.trim() : undefined
}

function resolveTitle(key: string | undefined): string {
  const k = key?.trim()
  if (!k) return ""
  return t(k)
}

function formatDate(value: string): string {
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString()
}

function rewardSegments(reward: Record<string, unknown> | undefined): RewardSegment[] {
  if (!reward) return []
  const parts: RewardSegment[] = []
  const items = Array.isArray(reward.items) ? reward.items : []
  items.forEach((item, index) => {
    if (!item || typeof item !== "object") return
    const node = item as Record<string, unknown>
    const itemId = typeof node.itemId === "string" ? node.itemId : ""
    const quantity = Number(node.quantity || 0)
    if (!itemId || quantity <= 0) return
    parts.push({ type: "item", key: `item-${index}-${itemId}`, itemId, quantity })
  })
  const kittyExp = Number(reward.kittyExp || 0)
  const bondExp = Number(reward.bondExp || 0)
  if (kittyExp > 0) parts.push({ type: "text", key: "kitty-exp", text: `${t("game.catteryKittyExp")} +${kittyExp}` })
  if (bondExp > 0) parts.push({ type: "text", key: "bond-exp", text: `${t("game.catteryKittyBond")} +${bondExp}` })
  return parts
}
</script>
