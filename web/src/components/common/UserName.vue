<template>
  <button
    type="button"
    :class="[
      'cursor-pointer text-left',
      plain
        ? 'text-[var(--app-text)] hover:text-[var(--el-color-primary)] hover:underline'
        : 'text-[var(--el-color-primary)] hover:underline'
    ]"
    @click="openProfile"
  >
    {{ displayName }}
  </button>
</template>

<script setup lang="ts">
import { computed, watch } from "vue"
import { useI18n } from "vue-i18n"
import { useLocalUserStore } from "@stores/local-user"
import { useUserProfileDialogStore } from "@stores/user-profile-dialog"

const props = defineProps<{
  userId: number
  username?: string
  plain?: boolean
}>()

const { t } = useI18n()
const localUserStore = useLocalUserStore()
const dialogStore = useUserProfileDialogStore()

watch(
  () => [props.userId, props.username] as const,
  ([userId, username]) => {
    const normalizedID = Math.floor(Number(userId || 0))
    if (normalizedID <= 0) return
    const normalizedName = String(username || "").trim()
    if (!normalizedName) return
    localUserStore.setPreview(normalizedID, normalizedName)
  },
  { immediate: true }
)

const displayName = computed(() => {
  const userID = Math.floor(Number(props.userId || 0))
  const rawName = String(props.username || "").trim()
  if (rawName) return rawName
  const cached = localUserStore.get(userID)
  if (cached?.username) return cached.username
  return `${t("game.playerLabel")} ${userID}`
})

function openProfile() {
  const userID = Math.floor(Number(props.userId || 0))
  if (userID <= 0) return
  dialogStore.open(userID)
}
</script>
