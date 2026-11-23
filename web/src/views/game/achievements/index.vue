<template>
  <div class="page-shell" data-tutorial="achievements-page">
    <GamePageFrame
      title-key="achievements.title"
      tutorial-id="achievements"
      :visual-src="achievementsVisual.src"
      :visual-tone="achievementsVisual.tone"
      visual-key="achievements"
    >
      <template #actions>
        <el-button size="small" :disabled="refreshCooldownRemaining > 0" :loading="store.loading" @click="refresh">
          {{ refreshButtonText }}
        </el-button>
      </template>

      <div class="space-y-4">
        <section class="flex flex-wrap items-center gap-2" data-tutorial="achievements-stats">
          <span class="min-w-[72px] rounded-lg border border-[var(--app-border)] bg-[var(--app-bg)] px-3 py-2 text-center text-xs text-[var(--app-muted)]">
            <strong class="block text-lg leading-none text-[var(--app-text)]">{{ store.achievements.length }}</strong>
            {{ t("achievements.total") }}
          </span>
          <span class="min-w-[72px] rounded-lg border border-[var(--app-border)] bg-[var(--app-bg)] px-3 py-2 text-center text-xs text-[var(--app-muted)]">
            <strong class="block text-lg leading-none text-[var(--app-text)]">{{ completedAchievements.length }}</strong>
            {{ t("achievements.completed") }}
          </span>
          <span
            class="min-w-[72px] rounded-lg border px-3 py-2 text-center text-xs"
            :class="store.claimableCount > 0 ? 'border-green-500/30 bg-green-500/10 text-green-700' : 'border-[var(--app-border)] bg-[var(--app-bg)] text-[var(--app-muted)]'"
          >
            <strong class="block text-lg leading-none text-[var(--app-text)]">{{ store.claimableCount }}</strong>
            {{ t("achievements.claimable") }}
          </span>
        </section>

        <section v-if="store.loading && !store.loaded" class="rounded-lg border border-[var(--app-border)] bg-[var(--app-surface)] p-6 text-center text-sm text-[var(--app-muted)]">
          {{ t("achievements.loading") }}
        </section>

        <section v-else-if="store.achievements.length === 0" class="rounded-lg border border-[var(--app-border)] bg-[var(--app-surface)] p-6 text-center text-sm text-[var(--app-muted)]">
          {{ t("achievements.emptyCategory") }}
        </section>

        <template v-else>
          <el-tabs v-model="activeTab" class="achievement-tabs" data-tutorial="achievements-tabs">
            <el-tab-pane :label="tabLabel('all', store.achievements.length)" name="all" />
            <el-tab-pane v-if="store.claimableCount > 0" :label="tabLabel('claimable', claimableAchievements.length)" name="claimable" />
            <el-tab-pane :label="tabLabel('active', activeAchievements.length)" name="active" />
            <el-tab-pane :label="tabLabel('completed', completedAchievements.length)" name="completed" />
          </el-tabs>

          <el-collapse v-if="displayGroups.length > 0" v-model="openCategoryIds" class="achievement-collapse" data-tutorial="achievements-categories">
            <el-collapse-item
              v-for="group in displayGroups"
              :key="group.category.id"
              :name="group.category.id"
              v-memo="[group.category.id, group.items.length, activeTab, isCategoryOpen(group.category.id)]"
            >
              <template #title>
                <div class="flex min-w-0 flex-1 items-center justify-between gap-3 pr-3">
                  <span class="truncate text-base font-extrabold text-[var(--app-text)]">{{ t(group.category.nameKey) }}</span>
                  <el-tag round effect="plain" size="small">{{ t("achievements.groupCount", { count: group.items.length }) }}</el-tag>
                </div>
              </template>

              <div v-if="isCategoryOpen(group.category.id)" class="space-y-3">
                <template v-if="group.subgroups.length > 1">
                  <el-collapse v-model="openSubgroupIds" class="achievement-subcollapse">
                    <el-collapse-item
                      v-for="subgroup in group.subgroups"
                      :key="subgroup.id"
                      :name="`${group.category.id}:${subgroup.id}`"
                    >
                      <template #title>
                        <div class="flex min-w-0 flex-1 items-center justify-between gap-3 pr-3">
                          <span class="truncate text-sm font-extrabold text-[var(--app-text)]">{{ subgroup.title }}</span>
                          <span class="text-xs font-semibold tabular-nums text-[var(--app-muted)]">{{ t("achievements.groupCount", { count: subgroup.count }) }}</span>
                        </div>
                      </template>
                      <div v-if="isSubgroupOpen(group.category.id, subgroup.id)" class="grid grid-cols-1 gap-2 sm:grid-cols-2 md:grid-cols-4 xl:grid-cols-5">
                        <template v-for="entry in sectionEntries(subgroup.items, activeTab === 'claimable')" :key="entry.id">
                          <AchievementStageGroup
                            v-if="entry.kind === 'stageGroup'"
                            :visible-items="entry.items"
                            :all-items="entry.allItems"
                            :claiming-ids="store.claimingIds"
                            @claim="claim"
                          />
                          <AchievementCardContent
                            v-else
                            :achievement="entry.item"
                            :claiming="!!store.claimingIds[entry.item.id]"
                            v-memo="[entry.item.id, entry.item.progressValue, entry.item.completed, entry.item.claimed, !!store.claimingIds[entry.item.id]]"
                            @claim="claim"
                          />
                        </template>
                      </div>
                    </el-collapse-item>
                  </el-collapse>
                </template>
                <div v-else class="grid grid-cols-1 gap-2 sm:grid-cols-2 md:grid-cols-4 xl:grid-cols-5 2xl:grid-cols-6">
                  <template v-for="entry in sectionEntries(group.subgroups[0]?.items ?? [], activeTab === 'claimable')" :key="entry.id">
                    <AchievementStageGroup
                      v-if="entry.kind === 'stageGroup'"
                      :visible-items="entry.items"
                      :all-items="entry.allItems"
                      :claiming-ids="store.claimingIds"
                      @claim="claim"
                    />
                    <AchievementCardContent
                      v-else
                      :achievement="entry.item"
                      :claiming="!!store.claimingIds[entry.item.id]"
                      v-memo="[entry.item.id, entry.item.progressValue, entry.item.completed, entry.item.claimed, !!store.claimingIds[entry.item.id]]"
                      @claim="claim"
                    />
                  </template>
                </div>
              </div>
            </el-collapse-item>
          </el-collapse>

          <section v-else class="rounded-lg border border-[var(--app-border)] bg-[var(--app-surface)] p-6 text-center text-sm text-[var(--app-muted)]">
            {{ t("achievements.emptyCategory") }}
          </section>
        </template>
      </div>
    </GamePageFrame>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue"
import { useI18n } from "vue-i18n"
import { ElMessage } from "element-plus"
import { useAchievementsStore, type AchievementCategoryView, type PlayerAchievementView } from "@stores/achievements"
import { useGameStore } from "@stores/game"
import { getRarityDefinition } from "@services/item-definition"
import AchievementCardContent from "./AchievementCardContent.vue"
import AchievementStageGroup from "./AchievementStageGroup.vue"
import { getGameDataChunk } from "@services/game-data-runtime"
import GamePageFrame from "@components/game/common/GamePageFrame.vue"
import { gamePageVisuals } from "@services/game-page-visuals"

const { t, locale } = useI18n()
const achievementsVisual = gamePageVisuals.achievements
const gameStore = useGameStore()
const store = useAchievementsStore()

type AchievementGroup = { category: AchievementCategoryView; items: PlayerAchievementView[] }
type AchievementDisplayGroup = AchievementGroup & { subgroups: AchievementSubgroup[] }
type AchievementSubgroup = { id: string; title: string; count: number; items: PlayerAchievementView[] }
type AchievementTab = "active" | "all" | "completed" | "claimable"
type SectionEntry =
  | { kind: "single"; id: string; item: PlayerAchievementView }
  | { kind: "stageGroup"; id: string; items: PlayerAchievementView[]; allItems: PlayerAchievementView[] }

type ActionRow = { id: string; mainType?: string; menu?: string }
type EnemyRow = { id: string; battle?: Record<string, unknown>; drops?: Record<string, unknown> }
type WorldRow = { id: string; nameKey?: string; enemyIds?: string | string[] }
type FishingSpeciesRow = { id: string; waterId?: string; names?: Record<string, string> }
type FishingWaterRow = { id: string; names?: Record<string, string> }

let achievementMeta:
  | {
    actionMetaByID: Map<string, ActionRow>
    worldRows: WorldRow[]
    worldByID: Map<string, WorldRow>
    enemyWorldByID: Map<string, string>
    fishingWaterRows: FishingWaterRow[]
    fishingWaterByID: Map<string, FishingWaterRow>
    fishingWaterBySpeciesID: Map<string, string>
  }
  | null = null

function getAchievementMeta() {
  if (!achievementMeta) {
    const worldRows = ((getGameDataChunk("world") as { world?: WorldRow[] }).world ?? [])
    const fishingWaterRows = ((getGameDataChunk("fishing_waters") as { fishing_waters?: FishingWaterRow[] }).fishing_waters ?? [])
    const fishingSpeciesRows = ((getGameDataChunk("fishing_species") as { fishing_species?: FishingSpeciesRow[] }).fishing_species ?? [])
    achievementMeta = {
      actionMetaByID: new Map(((getGameDataChunk("actions") as { actions?: ActionRow[] }).actions ?? []).map((action) => [action.id, action] as const)),
      worldRows,
      worldByID: new Map(worldRows.map((world) => [world.id, world] as const)),
      enemyWorldByID: buildEnemyWorldMap((getGameDataChunk("enemies") as { enemies?: EnemyRow[] }).enemies ?? [], worldRows),
      fishingWaterRows,
      fishingWaterByID: new Map(fishingWaterRows.map((water) => [water.id, water] as const)),
      fishingWaterBySpeciesID: new Map(fishingSpeciesRows.map((species) => [species.id, String(species.waterId || "other")] as const))
    }
  }
  return achievementMeta
}

const activeTab = ref<AchievementTab>("all")
const openCategoryIds = ref<string[]>([])
const openSubgroupIds = ref<string[]>([])
const categoryCollapseInitialized = ref(false)
const subgroupCollapseInitialized = ref(false)
const previousCategoryIds = ref<string[]>([])
const previousSubgroupIds = ref<string[]>([])
const lastRefreshAt = ref(0)
const nowMs = ref(Date.now())
let cooldownClock: number | null = null

function startCooldownTick() {
  if (cooldownClock !== null) return
  nowMs.value = Date.now()
  cooldownClock = window.setInterval(() => {
    if (refreshCooldownRemaining.value <= 0) {
      stopCooldownTick()
      return
    }
    nowMs.value = Date.now()
  }, 1000)
}

function stopCooldownTick() {
  if (cooldownClock === null) return
  window.clearInterval(cooldownClock)
  cooldownClock = null
}
const completedAchievements = computed(() => store.achievements.filter((item) => item.completed))
const activeAchievements = computed(() => store.achievements.filter((item) => !item.completed && !item.locked && item.progressValue > 0))
const claimableAchievements = computed(() => store.achievements.filter((item) => item.completed && !item.claimed))
const tabAchievements = computed(() => {
  switch (activeTab.value) {
    case "completed":
      return completedAchievements.value
    case "claimable":
      return claimableAchievements.value
    case "all":
      return store.achievements
    default:
      return activeAchievements.value
  }
})
const stageBucketsByBaseID = computed(() => {
  const map = new Map<string, PlayerAchievementView[]>()
  for (const item of store.achievements) {
    const baseID = item.baseAchievementId
    if (!baseID || !item.stageCount || item.stageCount <= 1) continue
    if (!map.has(baseID)) map.set(baseID, [])
    map.get(baseID)?.push(item)
  }
  return map
})
const visibleGroups = computed<AchievementGroup[]>(() => groupAchievements(tabAchievements.value))
const visibleSubgroupsByCategory = computed(() => new Map(visibleGroups.value.map((group) => [group.category.id, subgroupAchievements(group)] as const)))
const displayGroups = computed<AchievementDisplayGroup[]>(() => visibleGroups.value.map((group) => ({
  ...group,
  subgroups: visibleSubgroupsByCategory.value.get(group.category.id) ?? []
})))
const refreshCooldownRemaining = computed(() => Math.max(0, Math.ceil((lastRefreshAt.value + 5000 - nowMs.value) / 1000)))
const refreshButtonText = computed(() => refreshCooldownRemaining.value > 0 ? `${t("game.refresh")} ${refreshCooldownRemaining.value}s` : t("game.refresh"))

function groupAchievements(items: PlayerAchievementView[]) {
  const byID = new Map(store.categories.map((category) => [category.id, category]))
  const fallbackCategories = new Map<string, AchievementCategoryView>()
  const groups = new Map<string, PlayerAchievementView[]>()
  for (const achievement of items) {
    if (!groups.has(achievement.categoryId)) groups.set(achievement.categoryId, [])
    groups.get(achievement.categoryId)?.push(achievement)
    if (!byID.has(achievement.categoryId) && !fallbackCategories.has(achievement.categoryId)) {
      fallbackCategories.set(achievement.categoryId, {
        id: achievement.categoryId,
        nameKey: achievement.categoryId,
        order: 9999
      })
    }
  }
  return [...groups.entries()]
    .map(([categoryId, items]) => ({
      category: byID.get(categoryId) ?? fallbackCategories.get(categoryId) ?? { id: categoryId, nameKey: categoryId, order: 9999 },
      items: [...items].sort(sortAchievements)
    }))
    .sort((a, b) => a.category.order - b.category.order || a.category.id.localeCompare(b.category.id))
}

watch(
  () => gameStore.token,
  (token) => {
    if (token && !store.loaded && !store.loading) {
      void refresh()
    }
  },
  { immediate: true }
)

watch(visibleGroups, (groups) => {
  const categoryIds = groups.map((group) => group.category.id)
  openCategoryIds.value = mergeOpenKeys(openCategoryIds.value, previousCategoryIds.value, categoryIds, categoryCollapseInitialized.value)
  previousCategoryIds.value = categoryIds
  categoryCollapseInitialized.value = true
  const subgroupKeys = groups.flatMap((group) => (visibleSubgroupsByCategory.value.get(group.category.id) ?? []).map((subgroup) => `${group.category.id}:${subgroup.id}`))
  openSubgroupIds.value = mergeOpenKeys(openSubgroupIds.value, previousSubgroupIds.value, subgroupKeys, subgroupCollapseInitialized.value)
  previousSubgroupIds.value = subgroupKeys
  subgroupCollapseInitialized.value = true
}, { immediate: true })

onMounted(() => {
  void refresh()
})

onBeforeUnmount(() => {
  stopCooldownTick()
})

function sortAchievements(a: PlayerAchievementView, b: PlayerAchievementView) {
  const rarityDelta = (getRarityDefinition(a.rarity)?.order ?? 9999) - (getRarityDefinition(b.rarity)?.order ?? 9999)
  if (rarityDelta !== 0) return rarityDelta
  return a.order - b.order || a.id.localeCompare(b.id)
}

function sectionEntries(items: PlayerAchievementView[], flat = false): SectionEntry[] {
  const entries: SectionEntry[] = []
  const emittedStageBases = new Set<string>()
  const stageMap = stageBucketsByBaseID.value
  for (const item of items) {
    if (!flat) {
      const baseID = item.baseAchievementId
      if (baseID && item.stageCount && item.stageCount > 1) {
        if (emittedStageBases.has(baseID)) continue
        emittedStageBases.add(baseID)
        const visible = items.filter((candidate) => candidate.baseAchievementId === baseID).sort(sortAchievements)
        const allItems = (stageMap.get(baseID) ?? visible).sort(sortAchievements)
        entries.push({ kind: "stageGroup", id: baseID, items: visible, allItems })
        continue
      }
    }
    entries.push({ kind: "single", id: item.id, item })
  }
  return entries
}

function subgroupAchievements(group: AchievementGroup): AchievementSubgroup[] {
  const buckets = new Map<string, PlayerAchievementView[]>()
  for (const item of group.items) {
    const key = achievementSubgroupKey(item)
    if (!buckets.has(key)) buckets.set(key, [])
    buckets.get(key)?.push(item)
  }
  const entries = [...buckets.entries()]
    .map(([id, items]) => ({
      id,
      title: achievementSubgroupTitle(group.category.id, id),
      count: items.length,
      items: [...items].sort(sortAchievements)
    }))
    .sort((a, b) => achievementSubgroupOrder(group.category.id, a.id) - achievementSubgroupOrder(group.category.id, b.id) || a.title.localeCompare(b.title))
  return entries
}

function achievementSubgroupKey(item: PlayerAchievementView) {
  const condition = item.condition || {}
  if (item.categoryId === "combat" && String(condition.kind || "") === "kill_enemy") {
    return getAchievementMeta().enemyWorldByID.get(String(condition.enemyId || "")) || "other"
  }
  if (item.categoryId === "action" && String(condition.kind || "") === "complete_action") {
    const action = getAchievementMeta().actionMetaByID.get(String(condition.actionId || ""))
    return action?.mainType || action?.menu || "other"
  }
  if (item.categoryId === "fishing" && String(condition.kind || "") === "catch_fish") {
    return getAchievementMeta().fishingWaterBySpeciesID.get(String(condition.speciesId || "")) || "other"
  }
  return "default"
}

function achievementSubgroupTitle(categoryId: string, id: string) {
  if (id === "default") return t(`achievement_category.${categoryId}.name`)
  if (categoryId === "combat") {
    const world = getAchievementMeta().worldByID.get(id)
    return world?.nameKey ? t(world.nameKey) : t("achievements.subcategory.otherCombat")
  }
  if (categoryId === "action") {
    if (id === "other") return t("achievements.subcategory.otherAction")
    return t(`action_main_type.${id}.name`)
  }
  if (categoryId === "fishing") {
    const water = getAchievementMeta().fishingWaterByID.get(id)
    return water ? localizedSourceName(water.names, id) : t("achievements.subcategory.otherFishing")
  }
  return id
}

function achievementSubgroupOrder(categoryId: string, id: string) {
  if (id === "default") return 0
  if (categoryId === "combat") {
    const worldIndex = getAchievementMeta().worldRows.findIndex((world) => world.id === id)
    return worldIndex >= 0 ? worldIndex : 9999
  }
  if (categoryId === "action") {
    return id === "other" ? 9999 : 100
  }
  if (categoryId === "fishing") {
    const waterIndex = getAchievementMeta().fishingWaterRows.findIndex((water) => water.id === id)
    return waterIndex >= 0 ? waterIndex : 9999
  }
  return 0
}

function localizedSourceName(names: Record<string, string> | undefined, fallback: string) {
  const localeTag = String(locale.value || "en-US")
  return names?.[localeTag] || names?.[localeTag.split("-")[0]] || names?.["en-US"] || fallback
}

function buildEnemyWorldMap(enemies: EnemyRow[], worlds: WorldRow[]) {
  const out = new Map<string, string>()
  for (const world of worlds) {
    for (const enemyID of normalizeEnemyIDs(world.enemyIds)) {
      if (enemyID) out.set(enemyID, world.id)
    }
  }
  for (const enemy of enemies) {
    if (!enemy.id || out.has(enemy.id)) continue
    const battleKeys = Object.keys(enemy.battle || {})
    const mapKey = battleKeys.find((key) => key.startsWith("map_base_"))
    if (mapKey) {
      out.set(enemy.id, mapKey.replace(/^map_/, ""))
      continue
    }
    const dropKeys = Object.keys(enemy.drops || {})
    const dropMapKey = dropKeys.find((key) => key.startsWith("map_base_"))
    if (dropMapKey) {
      out.set(enemy.id, dropMapKey.replace(/^map_/, ""))
      continue
    }
    if (dropKeys.some((key) => key.startsWith("base4_"))) {
      out.set(enemy.id, "base_4")
      continue
    }
    if (dropKeys.some((key) => key.startsWith("base3_") || key.startsWith("rime") || key.startsWith("snow") || key.startsWith("ice_"))) {
      out.set(enemy.id, "base_3")
      continue
    }
    if (dropKeys.some((key) => key.startsWith("base2_"))) {
      out.set(enemy.id, "base_2")
      continue
    }
    if (dropKeys.some((key) => key.startsWith("holy_cat_"))) {
      out.set(enemy.id, "holy_cat_temple_001")
    }
  }
  return out
}

function normalizeEnemyIDs(value: unknown) {
  if (Array.isArray(value)) return value.map((entry) => String(entry).trim()).filter(Boolean)
  return String(value || "").split(",").map((entry) => entry.trim()).filter(Boolean)
}

function tabLabel(name: AchievementTab, count: number) {
  return `${t(`achievements.tabs.${name}`)} ${count}`
}

function isCategoryOpen(categoryId: string) {
  return openCategoryIds.value.includes(categoryId)
}

function isSubgroupOpen(categoryId: string, subgroupId: string) {
  return openSubgroupIds.value.includes(`${categoryId}:${subgroupId}`)
}

async function refresh() {
  if (!gameStore.token) return
  const remaining = refreshCooldownRemaining.value
  if (remaining > 0) return
  lastRefreshAt.value = Date.now()
  nowMs.value = lastRefreshAt.value
  startCooldownTick()
  try {
    await store.load(gameStore.token)
  } catch {
    ElMessage.error(t("achievements.loadFailed"))
  }
}

async function claim(achievementId: string) {
  if (!gameStore.token) return
  try {
    await store.claim(gameStore.token, achievementId)
    ElMessage.success(t("achievements.claimSuccess"))
  } catch {
    ElMessage.error(t("achievements.claimFailed"))
  }
}

function mergeOpenKeys(current: string[], previous: string[], available: string[], initialized: boolean) {
  const availableSet = new Set(available)
  const kept = current.filter((key) => availableSet.has(key))
  if (!initialized) return available.slice(0, 2)
  const previousSet = new Set(previous)
  const added = available.filter((key) => !previousSet.has(key))
  return [...kept, ...added]
}
</script>
