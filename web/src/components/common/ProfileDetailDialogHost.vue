<template>
  <GameDialog
    v-model="dialogVisible"
    :title="dialogTitle"
    width="560px"
    destroy-on-close
  >
    <div v-if="loading" class="app-muted">{{ t("game.userProfileDetailLoading") }}</div>
    <div v-else-if="isEmpty" class="app-muted">{{ emptyText }}</div>
    <div v-else-if="detailType === 'collectibles'" class="grid grid-cols-1 gap-2 md:grid-cols-2">
      <CollectibleItem
        v-for="item in collectibles"
        :key="item.collectibleId"
        :item-id="item.itemId"
        :name-key="item.nameKey"
        :desc-key="item.descKey"
        :fallback-name="item.collectibleId"
        :obtained-at="item.obtainedAt"
        :icon-size="64"
      />
    </div>
    <div v-else class="space-y-4">
      <div v-for="catGroup in achievementCategoryGroups" :key="catGroup.categoryId">
        <div class="mb-2 text-sm font-bold text-[var(--app-text)]">{{ categoryDisplayName(catGroup.categoryId) }}</div>
        <div class="grid grid-cols-2 gap-2 md:grid-cols-2">
          <template v-for="group in catGroup.groups" :key="group.baseId">
            <!-- Single-stage achievement -->
            <div
              v-if="group.stages.length === 1"
              class="relative overflow-hidden rounded-lg border border-[var(--app-border)] p-3"
            >
              <span
                class="absolute bottom-0 left-0 top-0 w-1"
                :style="{ backgroundColor: rarityColor(group.primary.rarity) }"
              />
              <div class="pl-2">
                <div class="flex items-start justify-between gap-2">
                  <div class="min-w-0 flex-1 truncate text-sm font-semibold leading-5">{{ achievementTitle(group.primary) }}</div>
                </div>
                <div v-if="achievementDesc(group.primary)" class="mt-1 text-xs leading-4 text-[var(--app-muted)]">{{ achievementDesc(group.primary) }}</div>
                <div v-if="group.primary.completedAt" class="mt-1.5 text-[10px] text-[var(--app-muted)]">
                  {{ t("game.userProfileAchievementCompletedAt") }}: {{ formatDate(group.primary.completedAt) }}
                </div>
              </div>
            </div>

            <!-- Multi-stage achievement -->
            <div
              v-else
              class="relative overflow-hidden rounded-lg border border-[var(--app-border)] p-3"
            >
              <span
                class="absolute bottom-0 left-0 top-0 w-1"
                :style="{ backgroundColor: rarityColor(group.primary.rarity) }"
              />
              <div class="pl-2">
                <div class="flex items-start justify-between gap-2">
                  <div class="min-w-0 flex-1 truncate text-sm font-semibold leading-5">{{ achievementTitle(group.primary) }}</div>
                  <span
                    class="shrink-0 rounded px-1.5 py-0.5 text-[10px] font-medium"
                    :style="{ backgroundColor: rarityBg(group.primary.rarity), color: rarityColor(group.primary.rarity) }"
                  >
                    {{ rarityLabel(group.primary.rarity) }}
                  </span>
                </div>
                <div v-if="achievementDesc(group.primary)" class="mt-1 text-xs leading-4 text-[var(--app-muted)]">{{ achievementDesc(group.primary) }}</div>
                <div v-if="group.primary.completedAt" class="mt-1.5 text-[10px] text-[var(--app-muted)]">
                  {{ t("game.userProfileAchievementCompletedAt") }}: {{ formatDate(group.primary.completedAt) }}
                </div>
              </div>
              <el-popover trigger="hover" placement="top-end" :width="320">
                <template #reference>
                  <button
                    type="button"
                    class="absolute right-2 top-2 z-10 rounded-full border border-[var(--app-border)] bg-[var(--app-surface)]/95 px-2 py-0.5 text-xs font-bold text-[var(--app-text)] shadow-sm transition hover:border-blue-400 hover:text-blue-600"
                  >
                    {{ t("achievements.stageCompact", { current: group.primaryStageIndex, total: group.stages.length }) }}
                  </button>
                </template>
                <div class="max-h-[300px] space-y-2 overflow-auto p-1">
                  <div
                    v-for="stage in group.stages"
                    :key="stage.id"
                    class="rounded-md border border-[var(--app-border)] bg-[var(--app-bg)] p-2"
                  >
                    <div class="flex items-center justify-between gap-2">
                      <div class="min-w-0 text-xs font-bold">
                        {{ t("achievements.stageLabel", { current: parseStageIndex(stage.id), total: group.stages.length }) }}
                      </div>
                    </div>
                    <div v-if="achievementDesc(stage)" class="mt-1 text-xs text-[var(--app-muted)]">{{ achievementDesc(stage) }}</div>
                    <div v-if="stage.completedAt" class="mt-1 text-[10px] text-[var(--app-muted)]">
                      {{ t("game.userProfileAchievementCompletedAt") }}: {{ formatDate(stage.completedAt) }}
                    </div>
                  </div>
                </div>
              </el-popover>
            </div>
          </template>
        </div>
      </div>
    </div>
  </GameDialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from "vue"
import { useI18n } from "vue-i18n"
import { useGameStore } from "@stores/game"
import { useLocalUserStore, type PlayerCollectibleDetail, type PlayerProfileAchievement } from "@stores/local-user"
import { useProfileDetailDialogStore } from "@stores/profile-detail-dialog"
import { getRarityDefinition } from "@services/item-definition"
import { resolveAchievementTitle, resolveAchievementDescription } from "@services/achievement-text"
import { getGameDataChunk } from "@services/game-data-runtime"
import CollectibleItem from "./CollectibleItem.vue"
import GameDialog from "@components/game/common/GameDialog.vue"

const { t, locale } = useI18n()
const gameStore = useGameStore()
const localUserStore = useLocalUserStore()
const dialogStore = useProfileDetailDialogStore()
const loading = ref(false)
const collectibles = ref<PlayerCollectibleDetail[]>([])
const achievements = ref<PlayerProfileAchievement[]>([])

const dialogVisible = computed({
  get() {
    return dialogStore.visible
  },
  set(value: boolean) {
    if (!value) dialogStore.close()
  }
})

const detailType = computed(() => dialogStore.detailType)

const dialogTitle = computed(() => {
  return detailType.value === "collectibles"
    ? t("game.userProfileCollectiblesTitle")
    : t("game.userProfileAchievementsTitle")
})

const isEmpty = computed(() => {
  if (detailType.value === "collectibles") return collectibles.value.length === 0
  return achievements.value.length === 0
})

const emptyText = computed(() => {
  return detailType.value === "collectibles"
    ? t("game.userProfileCollectiblesEmpty")
    : t("game.userProfileAchievementsEmpty")
})

watch(
  () => [dialogStore.visible, dialogStore.userId, dialogStore.detailType, gameStore.token] as const,
  async ([visible, userId, type, token]) => {
    if (!visible || userId <= 0 || !token) return
    loading.value = true
    try {
      if (type === "collectibles") {
        collectibles.value = await localUserStore.fetchProfileCollectibles(token, userId)
      } else {
        achievements.value = await localUserStore.fetchProfileAchievements(token, userId)
      }
    } catch {
      collectibles.value = []
      achievements.value = []
    } finally {
      loading.value = false
    }
  },
  { immediate: true }
)

const STAGE_PATTERN = /__stage_(\d+)$/

function parseBaseId(id: string): string {
  return id.replace(STAGE_PATTERN, "")
}

function parseStageIndex(id: string): number {
  const match = id.match(STAGE_PATTERN)
  return match ? Number(match[1]) : 0
}

type AchievementGroup = {
  baseId: string
  stages: PlayerProfileAchievement[]
  primary: PlayerProfileAchievement
  primaryStageIndex: number
}

const achievementGroups = computed<AchievementGroup[]>(() => {
  const map = new Map<string, PlayerProfileAchievement[]>()
  for (const item of achievements.value) {
    const baseId = parseBaseId(item.id)
    const list = map.get(baseId) || []
    list.push(item)
    map.set(baseId, list)
  }
  const groups: AchievementGroup[] = []
  for (const [baseId, stages] of map) {
    const sorted = [...stages].sort((a, b) => parseStageIndex(a.id) - parseStageIndex(b.id))
    const primary = sorted[sorted.length - 1]
    groups.push({
      baseId,
      stages: sorted,
      primary,
      primaryStageIndex: parseStageIndex(primary.id) || 1
    })
  }
  return groups.sort((a, b) => {
    const aTime = a.primary.completedAt ? new Date(a.primary.completedAt).getTime() : 0
    const bTime = b.primary.completedAt ? new Date(b.primary.completedAt).getTime() : 0
    return bTime - aTime
  })
})

type CategoryView = { id: string; nameKey: string; order: number; disabled?: boolean }
type AchievementCategoryGroup = { categoryId: string; groups: AchievementGroup[] }

let categoryMap: Map<string, CategoryView> | null = null

function getCategoryMap() {
  if (!categoryMap) {
    categoryMap = new Map<string, CategoryView>()
    const achievementCategoriesChunk = getGameDataChunk("achievement_categories")
    for (const cat of ((achievementCategoriesChunk as { achievement_categories?: CategoryView[] }).achievement_categories ?? [])) {
      if (!cat.disabled) categoryMap.set(cat.id, cat)
    }
  }
  return categoryMap
}

const achievementCategoryGroups = computed<AchievementCategoryGroup[]>(() => {
  const byCategory = new Map<string, AchievementGroup[]>()
  for (const group of achievementGroups.value) {
    const categoryId = group.primary.categoryId || ""
    const list = byCategory.get(categoryId) || []
    list.push(group)
    byCategory.set(categoryId, list)
  }
  return [...byCategory.entries()]
    .filter(([, groups]) => groups.length > 0)
    .map(([categoryId, groups]) => ({ categoryId, groups }))
    .sort((a, b) => {
      const categories = getCategoryMap()
      const aOrder = categories.get(a.categoryId)?.order ?? 9999
      const bOrder = categories.get(b.categoryId)?.order ?? 9999
      return aOrder - bOrder || a.categoryId.localeCompare(b.categoryId)
    })
})

function categoryDisplayName(categoryId: string): string {
  const cat = getCategoryMap().get(categoryId)
  if (cat?.nameKey) {
    const name = localizedOptional(cat.nameKey)
    if (name) return name
  }
  return categoryId
}

function localizedOptional(key: unknown): string {
  const normalized = String(key || "").trim()
  if (!normalized) return ""
  const localized = t(normalized)
  return localized === normalized ? "" : localized
}

function achievementTitle(item: PlayerProfileAchievement): string {
  return resolveAchievementTitle(t, item.nameKey, item.condition || {}, item.id, locale.value)
}

function achievementDesc(item: PlayerProfileAchievement): string {
  return resolveAchievementDescription(t, item.descKey, item.condition || {}, locale.value)
}

function formatDate(raw: string): string {
  if (!raw) return ""
  const date = new Date(raw)
  if (Number.isNaN(date.getTime())) return ""
  return date.toLocaleDateString()
}

function rarityColor(rarity: string): string {
  return getRarityDefinition(rarity)?.color || "var(--el-color-primary)"
}

function rarityBg(rarity: string): string {
  const color = rarityColor(rarity)
  return `${color}18`
}

function rarityLabel(rarity: string): string {
  const def = getRarityDefinition(rarity)
  if (def?.nameKey) {
    const name = localizedOptional(def.nameKey)
    if (name) return name
  }
  return rarity
}
</script>
