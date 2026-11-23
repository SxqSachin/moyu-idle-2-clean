<template>
  <div v-if="loading" class="app-muted">{{ t("game.catteryJournalLoading") }}</div>
  <div v-else-if="!entries.length" class="app-muted">{{ t("game.catteryEncounterEmpty") }}</div>
  <div v-else class="space-y-2">
    <div
      v-for="entry in entries"
      :key="entry.id"
      class="rounded border border-[var(--app-border)] bg-[var(--el-bg-color)] px-3 py-2"
    >
      <div class="font-medium">{{ t("game.catteryJournal.title.encounter") }}</div>
      <div class="mt-1">{{ resolveText(entry.contentKey, entry.contentParams) }}</div>
      <div v-if="resolveTarget(entry)" class="mt-1 text-xs app-muted">
        {{ t("game.catteryEncounterOtherKitty") }}:
        <KittyName
          :user-id="resolveTarget(entry)!.userId"
          :kitty-id="resolveTarget(entry)!.kittyId"
          :name="resolveTarget(entry)!.kittyName"
        />
      </div>
      <div class="mt-1 text-xs app-muted">{{ entry.createdAt ? formatDate(entry.createdAt) : "-" }}</div>
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
import KittyName from "@components/common/KittyName.vue"

type CatteryEncounterEntry = {
  id: number
  fromUserId: number
  fromKittyId: number
  toUserId: number
  toKittyId: number
  contentKey: string
  contentParams?: Record<string, unknown>
  createdAt?: string
}

type CatteryPagedResponse<T> = {
  items?: T[]
}

const props = defineProps<{
  selectedKittyId: number
}>()

const { t } = useI18n()
const gameStore = useGameStore()
const loading = ref(false)
const entries = ref<CatteryEncounterEntry[]>([])

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
    const resp = await apiGet<CatteryPagedResponse<CatteryEncounterEntry>>(
      `/api/cattery/encounters?kittyId=${encodeURIComponent(String(kittyId))}&page=1&pageSize=20`,
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

function resolveText(key: string | undefined, params: Record<string, unknown> | undefined): string {
  const k = key?.trim()
  if (!k) return "-"
  return t(k, params ?? {})
}

function resolveTarget(entry: CatteryEncounterEntry): { userId: number; kittyId: number; kittyName?: string } | null {
  if (!entry || props.selectedKittyId <= 0) return null
  const currentUserId = Number(gameStore.userId || 0)
  const fromIsCurrent = entry.fromUserId === currentUserId && entry.fromKittyId === props.selectedKittyId
  if (fromIsCurrent) {
    return {
      userId: entry.toUserId,
      kittyId: entry.toKittyId,
      kittyName: String(entry.contentParams?.targetKittyName || "")
    }
  }
  return {
    userId: entry.fromUserId,
    kittyId: entry.fromKittyId,
    kittyName: String(entry.contentParams?.kittyName || "")
  }
}

function formatDate(value: string): string {
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString()
}
</script>
