<template>
  <el-popover placement="top" :width="320" trigger="hover" @show="handleHover">
    <template #reference>
      <span class="cursor-help text-[var(--el-color-primary)] hover:underline">{{ displayName }}</span>
    </template>
    <div class="space-y-1 text-xs">
      <div class="font-semibold">{{ profile?.kittyName || displayName }}</div>
      <!-- <div class="app-muted">#{{ kittyId }}</div> -->
      <div class="app-muted">
        {{ t("game.catteryOwnerLabel") }}:
        <UserName :user-id="userId" :username="profile?.username" />
      </div>
      <div class="app-muted" v-if="profile?.kittyCreatedAt">
        {{ t("game.catteryKittyCreatedAt") }}: {{ formatDate(profile.kittyCreatedAt) }}
      </div>
      <div class="app-muted" v-if="profile?.runningActionId">
        {{ t("game.catteryKittyRunningAction") }}: {{ profile.runningActionId }}
      </div>
      <div v-if="loading" class="app-muted">{{ t("game.catteryJournalLoading") }}</div>
    </div>
  </el-popover>
</template>

<script setup lang="ts">
import { computed, ref } from "vue"
import { useI18n } from "vue-i18n"
import { useGameStore } from "@stores/game"
import { useLocalKittyStore } from "@stores/local-kitty"
import UserName from "@components/common/UserName.vue"

const props = defineProps<{
  userId: number
  kittyId: number
  name?: string
}>()

const { t } = useI18n()
const gameStore = useGameStore()
const localKittyStore = useLocalKittyStore()
const loading = ref(false)

const profile = computed(() => localKittyStore.get(props.userId, props.kittyId))
const displayName = computed(() => {
  const direct = String(props.name || "").trim()
  if (direct) return direct
  return profile.value?.kittyName || `${t("game.catteryKittyLabel")} #${props.kittyId}`
})

async function handleHover() {
  if (!gameStore.token || props.userId <= 0 || props.kittyId <= 0) return
  loading.value = true
  try {
    await localKittyStore.fetchByKitty(gameStore.token, props.userId, props.kittyId)
  } finally {
    loading.value = false
  }
}

function formatDate(value: string) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString()
}
</script>
