<template>
  <div class="tutorial-guide-body space-y-5">
    <div v-if="loading" class="space-y-3">
      <div class="flex items-center gap-2 text-sm app-muted">
        <Icon icon="material-symbols:progress-activity" class="animate-spin text-lg text-[var(--el-color-primary)]" />
        <span>{{ t("tutorial.loadingTutorials") }}</span>
      </div>
      <el-skeleton :rows="4" animated />
    </div>
    <div v-else-if="loadFailed" class="tutorial-guide-state rounded-lg border border-[var(--app-border)] p-4 text-sm app-muted">
      <div class="mb-3">{{ t("tutorial.loadFailed") }}</div>
      <el-button size="small" @click="loadTutorials">{{ t("tutorial.retry") }}</el-button>
    </div>
    <template v-else>
      <div v-if="recommendedTutorials.length" class="tutorial-guide-recommended">
        <div class="mb-1 text-xs font-semibold uppercase tracking-wide app-muted">
          {{ t("tutorial.guide.recommendedTitle") }}
        </div>
        <div class="mb-2 text-xs app-muted">
          {{ t("tutorial.guide.recommendedDesc") }}
        </div>
        <div class="space-y-2">
          <div
            v-for="entry in recommendedTutorials"
            :key="`recommended-${entry.id}`"
            class="tutorial-guide-entry flex items-center justify-between gap-3 rounded-lg border p-3 transition"
            :class="isUnlocked(entry)
              ? 'tutorial-guide-entry--unlocked border-[var(--app-border)]'
              : 'tutorial-guide-entry--locked border-[var(--app-border)] opacity-60'"
          >
            <div class="flex min-w-0 items-center gap-3">
              <div class="tutorial-guide-icon">
                <Icon v-if="entry.icon" :icon="entry.icon" class="shrink-0 text-xl" />
              </div>
              <div class="min-w-0">
                <div class="flex items-center gap-2">
                  <span class="truncate text-sm font-semibold">{{ t(entry.nameKey) }}</span>
                  <Icon v-if="!isUnlocked(entry)" icon="material-symbols:lock-outline" class="shrink-0 text-sm app-muted" />
                </div>
                <div class="text-xs leading-relaxed app-muted">
                  {{ t(entry.descriptionKey) }}
                  <template v-if="!isUnlocked(entry) && entry.prerequisites?.length">
                    — {{ t("tutorial.requires") }}:
                    <span v-for="(pid, i) in entry.prerequisites" :key="pid" class="inline">
                      {{ i > 0 ? ", " : "" }}{{ tutorialName(pid) }}
                    </span>
                  </template>
                </div>
              </div>
            </div>
            <div class="flex shrink-0 items-center gap-2">
              <el-tag v-if="isCompleted(entry.id, entry.version ?? 1)" type="success" size="small">
                {{ t("tutorial.completed") }}
              </el-tag>
              <el-button
                size="small"
                type="primary"
                :disabled="!isUnlocked(entry)"
                @click="handleStart(entry.id)"
              >
                {{ isCompleted(entry.id, entry.version ?? 1) ? t("tutorial.replay") : t("tutorial.start") }}
              </el-button>
            </div>
          </div>
        </div>
      </div>
      <template v-for="group in groupedTutorials" :key="group.groupId">
        <div>
          <div class="mb-2 text-xs font-semibold uppercase tracking-wide app-muted">
            {{ t(group.titleKey) }}
          </div>
          <div class="space-y-2">
            <div
              v-for="entry in group.items"
              :key="entry.id"
              class="tutorial-guide-entry flex items-center justify-between gap-3 rounded-lg border p-3 transition"
              :class="isUnlocked(entry)
                ? 'tutorial-guide-entry--unlocked border-[var(--app-border)]'
                : 'tutorial-guide-entry--locked border-[var(--app-border)] opacity-60'"
            >
              <div class="flex min-w-0 items-center gap-3">
                <div class="tutorial-guide-icon">
                  <Icon v-if="entry.icon" :icon="entry.icon" class="shrink-0 text-xl" />
                </div>
                <div class="min-w-0">
                  <div class="flex items-center gap-2">
                    <span class="truncate text-sm font-semibold">{{ t(entry.nameKey) }}</span>
                    <Icon v-if="!isUnlocked(entry)" icon="material-symbols:lock-outline" class="shrink-0 text-sm app-muted" />
                  </div>
                  <div class="text-xs leading-relaxed app-muted">
                    {{ t(entry.descriptionKey) }}
                    <template v-if="!isUnlocked(entry) && entry.prerequisites?.length">
                      — {{ t("tutorial.requires") }}:
                      <span v-for="(pid, i) in entry.prerequisites" :key="pid" class="inline">
                        {{ i > 0 ? ", " : "" }}{{ tutorialName(pid) }}
                      </span>
                    </template>
                  </div>
                </div>
              </div>
              <div class="flex shrink-0 items-center gap-2">
                <el-tag v-if="isCompleted(entry.id, entry.version ?? 1)" type="success" size="small">
                  {{ t("tutorial.completed") }}
                </el-tag>
                <el-button
                  size="small"
                  type="primary"
                  :disabled="!isUnlocked(entry)"
                  @click="handleStart(entry.id)"
                >
                  {{ isCompleted(entry.id, entry.version ?? 1) ? t("tutorial.replay") : t("tutorial.start") }}
                </el-button>
              </div>
            </div>
          </div>
        </div>
      </template>
      <div v-if="groupedTutorials.length === 0 && recommendedTutorials.length === 0" class="py-8 text-center text-sm app-muted">
        {{ t("tutorial.noTutorials") }}
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from "vue"
import { useI18n } from "vue-i18n"
import { Icon } from "@iconify/vue"
import { useTutorialStore } from "@/tutorial"
import { compareTutorialGuideEntries, getTutorialGuidePlacement, recommendedTutorialIds, tutorialGuideGroups, type TutorialGuideGroupId } from "@/tutorial/guide"
import type { TutorialConfig } from "@/tutorial"

const { t } = useI18n()
const tutorialStore = useTutorialStore()
const allTutorials = ref<TutorialConfig[]>([])
const loading = ref(true)
const loadFailed = ref(false)

type GroupedEntry = { groupId: TutorialGuideGroupId; titleKey: string; items: TutorialConfig[] }

onMounted(async () => {
  await loadTutorials()
})

const groupedTutorials = computed<GroupedEntry[]>(() => {
  const groups = new Map<TutorialGuideGroupId, GroupedEntry>()
  for (const group of tutorialGuideGroups) {
    groups.set(group.id, { groupId: group.id, titleKey: group.titleKey, items: [] })
  }
  for (const tut of allTutorials.value) {
    const placement = getTutorialGuidePlacement(tut)
    groups.get(placement.groupId)?.items.push(tut)
  }
  return tutorialGuideGroups
    .map((group) => groups.get(group.id))
    .filter((group): group is GroupedEntry => Boolean(group) && group.groupId !== "recommended" && group.items.length > 0)
    .map((group) => ({ ...group, items: [...group.items].sort(compareTutorialGuideEntries) }))
})

const recommendedTutorials = computed<TutorialConfig[]>(() => {
  const byId = new Map(allTutorials.value.map((tut) => [tut.id, tut]))
  return recommendedTutorialIds
    .map((id) => byId.get(id))
    .filter((config): config is TutorialConfig => Boolean(config))
})

function isCompleted(tutorialId: string, version: number): boolean {
  return tutorialStore.isCompleted(tutorialId, version)
}

function isUnlocked(config: TutorialConfig): boolean {
  return tutorialStore.isUnlocked(config)
}

function tutorialName(tutorialId: string): string {
  const found = allTutorials.value.find((tut) => tut.id === tutorialId)
  return found ? t(found.nameKey) : tutorialId
}

function handleStart(tutorialId: string) {
  void tutorialStore.start(tutorialId)
}

async function loadTutorials() {
  loading.value = true
  loadFailed.value = false
  try {
    allTutorials.value = await tutorialStore.loadAll()
  } catch (err) {
    console.warn("[tutorial] Failed to load tutorial guide entries", err)
    allTutorials.value = []
    loadFailed.value = true
  } finally {
    loading.value = false
  }
}
</script>
