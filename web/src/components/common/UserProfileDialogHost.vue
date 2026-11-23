<template>
  <GameDialog
    v-model="dialogVisible"
    :title="t('game.userProfileDialogTitle')"
    width="700px"
    class="user-profile-dialog dialog-mobile-windowed"
    destroy-on-close
  >
    <div v-if="loading" class="app-muted">{{ t("game.userProfileLoading") }}</div>
    <div v-else-if="!profile" class="app-muted">{{ t("game.userProfileUnavailable") }}</div>
    <div v-else class="user-profile-dialog__content">
      <!-- Header: Name + Title + Reputation -->
      <div class="mb-4 rounded-xl border border-[var(--app-border)] bg-[var(--app-surface)] p-4">
        <div class="flex items-start justify-between gap-3">
          <div class="min-w-0">
            <div class="flex items-center gap-1">
              <div class="min-w-0 break-all text-lg font-bold tracking-tight">{{ profile.username }}</div>
              <el-popover v-if="nameHistory.length" placement="bottom-start" :width="280" trigger="click">
                <template #reference>
                  <el-button
                    text
                    circle
                    size="small"
                    class="shrink-0"
                    :aria-label="t('game.userProfileNameHistoryButton')"
                    :title="t('game.userProfileNameHistoryButton')"
                  >
                    <Icon icon="material-symbols:history-rounded" class="text-base" />
                  </el-button>
                </template>
                <div class="text-sm font-semibold">{{ t("game.userProfileNameHistory") }}</div>
                <div class="mt-2 max-h-56 divide-y divide-[var(--app-border)] overflow-y-auto">
                  <div
                    v-for="(entry, index) in nameHistory"
                    :key="`${index}:${entry.oldName}`"
                    class="py-2 first:pt-0 last:pb-0"
                  >
                    <div class="break-all text-sm font-medium">{{ entry.oldName }}</div>
                  </div>
                </div>
                <div v-if="profile.nameHistoryHasMore" class="mt-2 border-t border-[var(--app-border)] pt-2 text-xs text-[var(--app-muted)]">
                  {{ t("game.userProfileNameHistoryHasMore") }}
                </div>
              </el-popover>
            </div>
            <div v-if="displayTitle" class="mt-0.5 text-xs text-[var(--app-muted)]">{{ displayTitle }}</div>
          </div>
          <div v-if="reputation" class="shrink-0 text-right">
            <div class="text-xs text-[var(--app-muted)]">{{ t("game.userProfileReputation") }}</div>
            <div class="text-base font-bold text-[var(--el-color-primary)]">
              {{ t("game.userProfileReputationLevel", { level: reputation.level }) }}
            </div>
            <div v-if="reputation.nextLevelXp > 0" class="mt-1 w-24">
              <div class="h-1.5 overflow-hidden rounded-full bg-[var(--app-border)]">
                <div
                  class="h-full rounded-full bg-[var(--el-color-primary)] transition-all"
                  :style="{ width: `${Math.min(100, (reputation.levelReputation / reputation.nextLevelXp) * 100)}%` }"
                />
              </div>
              <div class="mt-0.5 text-[10px] text-[var(--app-muted)]">
                {{ reputation.levelReputation }} / {{ reputation.nextLevelXp }}
              </div>
            </div>
            <div v-else class="text-[10px] text-[var(--el-color-success)]">{{ t("game.userProfileReputationMaxLevel") }}</div>
          </div>
        </div>
      </div>

      <el-tabs v-model="activeTab">
        <!-- Overview Tab -->
        <el-tab-pane :label="t('game.userProfileTabOverview')" name="overview">
          <div class="space-y-4 pt-2">
            <!-- Kitties section -->
            <div class="rounded-lg border border-[var(--app-border)] p-3">
              <div class="mb-2 text-sm font-semibold">{{ t("game.userProfileKittiesTitle") }}</div>
              <div v-if="!kittiesList.length" class="text-xs app-muted">{{ t("game.userProfileNoKitties") }}</div>
              <div v-else class="flex flex-wrap gap-2">
                <div
                  v-for="kitty in displayedKitties"
                  :key="kitty.id"
                  class="flex items-center gap-2 rounded-lg bg-[var(--app-surface)] px-3 py-1.5"
                >
                  <span class="text-sm font-medium">{{ kitty.name }}</span>
                  <span class="text-xs text-[var(--app-muted)]">Lv.{{ kitty.level }}</span>
                </div>
                <div v-if="kittiesList.length > 6" class="flex items-center px-2 text-xs text-[var(--app-muted)]">
                  {{ t("game.userProfileKittiesMore", { count: kittiesList.length - 6 }) }}
                </div>
              </div>
            </div>

            <!-- Skills section -->
            <div class="rounded-lg border border-[var(--app-border)] p-3">
              <div class="mb-2 text-sm font-semibold">{{ t("game.userProfileSkillsTitle") }}</div>
              <div v-if="!enabledSkills.length" class="text-xs app-muted">{{ t("game.userProfileNoSkills") }}</div>
              <div v-else class="flex flex-wrap gap-1.5">
                <div
                  v-for="skill in displayedSkills"
                  :key="skill.skillId"
                  class="flex items-center gap-1.5 rounded-md bg-[var(--app-surface)] px-2 py-1 text-xs"
                >
				  <SkillQuickRef :skill-id="skill.skillId" :unit-skill-level="skill.level" :runtime-skill="skill.runtimeSkill" />
                  <span class="text-[var(--app-muted)]">Lv.{{ skill.level }}</span>
                </div>
                <div v-if="enabledSkills.length > 8" class="flex items-center px-1 text-xs text-[var(--app-muted)]">
                  {{ t("game.userProfileSkillsMore", { count: enabledSkills.length - 8 }) }}
                </div>
              </div>
            </div>

            <!-- Stats: Collectibles + Achievements (clickable) -->
            <div class="grid grid-cols-2 gap-3">
              <button
                type="button"
                class="flex cursor-pointer flex-col items-center rounded-lg border border-[var(--app-border)] p-4 transition-colors hover:border-[var(--el-color-primary)] hover:bg-[var(--el-color-primary-light-9)]"
                @click="openDetail('collectibles')"
              >
                <div class="text-2xl font-bold">{{ profile.collectiblesCount || 0 }}</div>
                <div class="mt-1 text-xs text-[var(--app-muted)]">{{ t("game.userProfileCollectibles") }}</div>
              </button>
              <button
                type="button"
                class="flex cursor-pointer flex-col items-center rounded-lg border border-[var(--app-border)] p-4 transition-colors hover:border-[var(--el-color-primary)] hover:bg-[var(--el-color-primary-light-9)]"
                @click="openDetail('achievements')"
              >
                <div class="text-2xl font-bold">{{ t("game.userProfileAchievementsFormat", { completed: profile.achievementsCompleted || 0, total: profile.achievementsTotal || 0 }) }}</div>
                <div class="mt-1 text-xs text-[var(--app-muted)]">{{ t("game.userProfileAchievements") }}</div>
              </button>
            </div>
          </div>
        </el-tab-pane>

        <!-- Attributes Tab -->
        <el-tab-pane :label="t('game.userProfileTabAttributes')" name="attributes" lazy>
          <div class="pt-2">
            <div v-if="attrLoading" class="py-6 text-center text-sm text-[var(--app-muted)]">
              {{ t("game.userProfileLoading") }}
            </div>
            <div v-else-if="!publicProfile" class="py-6 text-center text-sm text-[var(--app-muted)]">
              {{ t("game.userProfileUnavailable") }}
            </div>
            <AttributesStatsTab
              v-else
              :rows="attrRows"
              :basic-attack="publicProfile.basicAttack as Record<string, unknown> | undefined"
              :unit-attributes="publicProfile.derived as Record<string, number> | undefined"
              :show-allocate-button="false"
              :visible-group-keys="['battle', 'work']"
              @refresh="refreshAttributes"
              @open-allocate-dialog="() => {}"
            />
          </div>
        </el-tab-pane>

        <el-tab-pane :label="t('game.userProfileTabPond')" name="pond" lazy>
          <UserProfilePondTab :user-id="profile.userId" />
        </el-tab-pane>
      </el-tabs>
    </div>
  </GameDialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from "vue"
import type { Ref } from "vue"
import { Icon } from "@iconify/vue"
import { useI18n } from "vue-i18n"
import { useGameStore } from "@stores/game"
import { useLocalUserStore } from "@stores/local-user"
import { useUserProfileDialogStore } from "@stores/user-profile-dialog"
import { useProfileDetailDialogStore, type ProfileDetailType } from "@stores/profile-detail-dialog"
import { getLocalizedSkillName, mergeRuntimeSkillDefinitions } from "@services/skill-definition"
import { useAttributeRows } from "@/composables/game/attributes/useAttributeRows"
import { findPrefix, findSuffix } from "@composables/useTitleDisplay"
import AttributesStatsTab from "@components/game/attributes/AttributesStatsTab.vue"
import GameDialog from "@components/game/common/GameDialog.vue"
import SkillQuickRef from "@components/game/skill/SkillQuickRef.vue"
import UserProfilePondTab from "@components/common/UserProfilePondTab.vue"

const { t } = useI18n()
const gameStore = useGameStore()
const localUserStore = useLocalUserStore()
const dialogStore = useUserProfileDialogStore()
const detailDialogStore = useProfileDetailDialogStore()
const loading = ref(false)
const attrLoading = ref(false)
const activeTab = ref("overview")

const dialogVisible = computed({
  get() {
    return dialogStore.visible
  },
  set(value: boolean) {
    if (!value) dialogStore.close()
  }
})

const profile = computed(() => localUserStore.getProfile(dialogStore.userId))
const publicProfile = computed(() => localUserStore.get(dialogStore.userId))
const nameHistory = computed(() =>
  (profile.value?.nameHistory || []).filter((entry) => String(entry.oldName || "").trim() !== "")
)

watch(
  () => [dialogStore.visible, dialogStore.userId, gameStore.token] as const,
  async ([visible, userId, token]) => {
    if (!visible || userId <= 0 || !token) {
      activeTab.value = "overview"
      return
    }
    loading.value = true
    try {
		const loaded = await localUserStore.fetchProfileByUserId(token, userId)
		mergeRuntimeSkillDefinitions((loaded.customSkills || []).map((row) => row.compiledSkill))
    } finally {
      loading.value = false
    }
  },
  { immediate: true }
)

watch(
  () => [activeTab.value, dialogStore.userId, gameStore.token] as const,
  async ([tab, userId, token]) => {
    if (tab !== "attributes" || userId <= 0 || !token) return
    if (publicProfile.value?.attributes) return
    attrLoading.value = true
    try {
      await localUserStore.fetchByUserId(token, userId)
    } finally {
      attrLoading.value = false
    }
  }
)

async function refreshAttributes() {
  const token = gameStore.token
  const userId = dialogStore.userId
  if (!token || userId <= 0) return
  attrLoading.value = true
  try {
    await localUserStore.fetchByUserId(token, userId, { force: true })
  } finally {
    attrLoading.value = false
  }
}

const attrAttributes = computed(() => (publicProfile.value?.attributes as Record<string, Record<string, unknown>>) || {})
const attrProgress = computed(() => (publicProfile.value?.attributeProgress as Record<string, Record<string, unknown>>) || {})
const attrFormula = computed(() => (publicProfile.value?.formula as { derived?: Record<string, string> }) || {})

const { attrRows } = useAttributeRows(
  attrAttributes as Ref<Record<string, Record<string, unknown>>>,
  attrProgress as Ref<Record<string, Record<string, unknown>>>,
  attrFormula as Ref<{ derived?: Record<string, string> }>,
  t
)

const displayTitle = computed(() => {
  if (!profile.value) return ""
  const parts: string[] = []
  const prefixId = profile.value.titlePrefixId
  const suffixId = profile.value.titleSuffixId
  if (prefixId) {
    const entry = findPrefix(prefixId)
    if (entry?.nameKey) {
      const translated = t(entry.nameKey)
      if (translated && translated !== entry.nameKey) parts.push(translated)
    }
  }
  if (suffixId) {
    const entry = findSuffix(suffixId)
    if (entry?.nameKey) {
      const translated = t(entry.nameKey)
      if (translated && translated !== entry.nameKey) parts.push(translated)
    }
  }
  return parts.join("")
})

const reputation = computed(() => profile.value?.reputation || null)

const kittiesList = computed(() => profile.value?.kitties || [])
const displayedKitties = computed(() => kittiesList.value.slice(0, 6))

const enabledSkills = computed(() => {
  const skills = profile.value?.skills || []
	const customByID = new Map((profile.value?.customSkills || []).map((row) => [row.id,row.compiledSkill || null]))
  return skills
    .filter((s) => s.enabled)
    .sort((a, b) => a.castOrder - b.castOrder)
		.map((s) => ({ ...s, name: getLocalizedSkillName(t, s.skillId, s.skillId), runtimeSkill: customByID.get(s.skillId) || null }))
})

const displayedSkills = computed(() => enabledSkills.value.slice(0, 8))

function openDetail(type: ProfileDetailType) {
  if (!profile.value) return
  detailDialogStore.open(profile.value.userId, type)
}
</script>

<style scoped>
.user-profile-dialog__content,
.user-profile-dialog__content :deep(.el-tabs),
.user-profile-dialog__content :deep(.el-tabs__content),
.user-profile-dialog__content :deep(.el-tab-pane) {
  min-width: 0;
}

:global(.user-profile-dialog.el-dialog) {
  max-width: calc(100vw - 24px);
  display: flex;
  flex-direction: column;
}

:global(.user-profile-dialog.el-dialog .el-dialog__body) {
  min-width: 0;
  min-height: 0;
  overflow-x: hidden;
  overscroll-behavior: contain;
}

@media (min-width: 768px) {
  :global(.user-profile-dialog.el-dialog) {
    max-height: 90dvh;
    margin: 5dvh auto !important;
  }

  :global(.user-profile-dialog.el-dialog .el-dialog__body) {
    overflow-y: auto;
  }
}
</style>
