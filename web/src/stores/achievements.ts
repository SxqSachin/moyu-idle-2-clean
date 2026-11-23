import { defineStore } from "pinia"
import { computed, ref } from "vue"
import {
  claimAchievement,
  fetchAchievements,
  type AchievementCategoryView,
  type AchievementDefinition,
  type PlayerAchievementList,
  type PlayerAchievementStateView,
  type PlayerAchievementView
} from "@services/achievements"
import { getGameDataChunk } from "@services/game-data-runtime"

export type { AchievementCategoryView, AchievementDefinition, PlayerAchievementList, PlayerAchievementStateView, PlayerAchievementView }

type AchievementCategoriesChunk = { achievement_categories?: AchievementCategoryView[] }
type AchievementsChunk = { achievements?: AchievementDefinition[] }
type RarityDefinition = { id: string; order?: number }
type RarityDefinitionsChunk = { rarity_definitions?: RarityDefinition[] }

let localData: {
  categories: AchievementCategoryView[]
  definitions: AchievementDefinition[]
  rarityOrderByID: Map<string, number>
  categoryOrderByID: Map<string, number>
} | null = null

function getLocalAchievementData() {
  if (!localData) {
    const categories = ((getGameDataChunk("achievement_categories") as AchievementCategoriesChunk).achievement_categories ?? [])
      .filter((category) => !category.disabled)
      .sort((a, b) => a.order - b.order || a.id.localeCompare(b.id))
    const definitions = ((getGameDataChunk("achievements") as AchievementsChunk).achievements ?? [])
      .filter((achievement) => !achievement.disabled)
      .flatMap(expandAchievementDefinition)
    const rarityOrderByID = new Map(
      ((getGameDataChunk("rarity_definitions") as RarityDefinitionsChunk).rarity_definitions ?? [])
        .map((rarity) => [rarity.id, Number(rarity.order ?? 9999)] as const)
    )
    const categoryOrderByID = new Map(categories.map((category) => [category.id, category.order] as const))
    localData = { categories, definitions, rarityOrderByID, categoryOrderByID }
  }
  return localData
}

export const useAchievementsStore = defineStore("achievements", () => {
  const categories = ref<AchievementCategoryView[]>([])
  const stateById = ref<Record<string, PlayerAchievementStateView>>({})
  const loaded = ref(false)
  const loading = ref(false)
  const claimingIds = ref<Record<string, boolean>>({})

  let cachedAchievements: PlayerAchievementView[] = []

  const achievements = computed<PlayerAchievementView[]>(() => {
    const localDefinitions = getLocalAchievementData().definitions
    const rows: PlayerAchievementView[] = []
    for (const definition of localDefinitions) {
      const state = stateById.value[definition.id]
      const completed = !!state?.completed
      if (definition.hidden && !completed) continue
      const targetValue = achievementTargetValue(definition)
      const progressValue = completed
        ? Math.max(targetValue, Number(state?.progressValue || 0))
        : Math.max(0, Number(state?.progressValue || 0))
      rows.push({
        ...definition,
        nameKey: definition.nameKey || "",
        descKey: definition.descKey || "",
        prereqAchievementIds: Array.isArray(definition.prereqAchievementIds) ? definition.prereqAchievementIds : [],
        rewards: Array.isArray(definition.rewards) ? definition.rewards : [],
        locked: !state,
        completed,
        claimed: !!state?.claimed,
        progressValue,
        targetValue,
        unlockedAt: state?.unlockedAt,
        completedAt: state?.completedAt,
        claimedAt: state?.claimedAt
      })
    }
    rows.sort(sortAchievementView)
    if (achievementRowsEqual(cachedAchievements, rows)) return cachedAchievements
    cachedAchievements = rows
    return rows
  })

  const claimableCount = computed(() => achievements.value.filter((item) => item.completed && !item.claimed).length)

  function applyPatch(state: PlayerAchievementList | null | undefined) {
    const next: Record<string, PlayerAchievementStateView> = {}
    for (const row of Array.isArray(state?.achievements) ? state.achievements : []) {
      if (!row?.id) continue
      next[row.id] = row
    }
    stateById.value = next
    categories.value = [...getLocalAchievementData().categories]
    loaded.value = true
  }

  async function load(token: string) {
    loading.value = true
    try {
      applyPatch(await fetchAchievements(token))
    } finally {
      loading.value = false
    }
  }

  async function claim(token: string, achievementId: string) {
    if (!achievementId || claimingIds.value[achievementId]) return
    claimingIds.value = { ...claimingIds.value, [achievementId]: true }
    try {
      await claimAchievement(token, achievementId)
      await load(token)
    } finally {
      const next = { ...claimingIds.value }
      delete next[achievementId]
      claimingIds.value = next
    }
  }

  function reset() {
    stateById.value = {}
    loaded.value = false
    loading.value = false
    claimingIds.value = {}
  }

  return {
    categories,
    achievements,
    loaded,
    loading,
    claimingIds,
    claimableCount,
    applyPatch,
    load,
    claim,
    reset
  }
})

function sortAchievementView(a: PlayerAchievementView, b: PlayerAchievementView) {
  const { categoryOrderByID, rarityOrderByID } = getLocalAchievementData()
  const categoryDelta = (categoryOrderByID.get(a.categoryId) ?? 9999) - (categoryOrderByID.get(b.categoryId) ?? 9999)
  if (categoryDelta !== 0) return categoryDelta
  const rarityDelta = (rarityOrderByID.get(a.rarity) ?? 9999) - (rarityOrderByID.get(b.rarity) ?? 9999)
  if (rarityDelta !== 0) return rarityDelta
  return a.order - b.order || a.id.localeCompare(b.id)
}

function achievementRowsEqual(prev: PlayerAchievementView[], next: PlayerAchievementView[]): boolean {
  if (prev.length !== next.length) return false
  for (let i = 0; i < prev.length; i++) {
    const a = prev[i]
    const b = next[i]
    if (a.id !== b.id) return false
    if (a.progressValue !== b.progressValue) return false
    if (a.completed !== b.completed) return false
    if (a.claimed !== b.claimed) return false
    if (a.locked !== b.locked) return false
    if (a.completedAt !== b.completedAt) return false
    if (a.claimedAt !== b.claimedAt) return false
    if (a.unlockedAt !== b.unlockedAt) return false
  }
  return true
}

function achievementTargetValue(definition: AchievementDefinition) {
  const condition = definition.condition || {}
  const kind = String(condition.kind || "").trim()
  if (kind === "player_attr_reached") {
    return positiveInt(condition.targetValue ?? condition.value ?? condition.count, 1)
  }
  if (kind === "own_item" || kind === "obtain_item") {
    return positiveInt(condition.quantity, 1)
  }
  return positiveInt(condition.count, 1)
}

function expandAchievementDefinition(definition: AchievementDefinition): AchievementDefinition[] {
  const stages = Array.isArray(definition.stages) ? definition.stages : []
  if (stages.length === 0) return [definition]
  return stages
    .map((stage, index) => {
      const stageId = `${definition.id}__stage_${index + 1}`
      const prereqs = [...(Array.isArray(definition.prereqAchievementIds) ? definition.prereqAchievementIds : [])]
      if (index > 0) prereqs.push(`${definition.id}__stage_${index}`)
      return {
        ...definition,
        id: stageId,
        nameKey: stage.nameKey ?? definition.nameKey ?? "",
        descKey: stage.descKey ?? definition.descKey ?? "",
        rarity: stage.rarity ?? definition.rarity,
        iconId: stage.iconId ?? definition.iconId,
        order: Number(stage.order ?? definition.order + index),
        hidden: stage.hidden ?? definition.hidden,
        disabled: stage.disabled ?? definition.disabled,
        condition: stage.condition ?? definition.condition,
        rewards: stage.rewards ?? definition.rewards,
        prereqAchievementIds: prereqs,
        baseAchievementId: definition.id,
        stageIndex: index + 1,
        stageCount: stages.length,
        stages: undefined
      }
    })
    .filter((achievement) => !achievement.disabled)
}

function positiveInt(value: unknown, fallback: number) {
  const parsed = Number(value)
  return Number.isFinite(parsed) && parsed > 0 ? Math.floor(parsed) : fallback
}
