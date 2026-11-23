import { apiGet, apiPost } from "@services/api"

export type AchievementCategoryView = {
  id: string
  nameKey: string
  iconId?: string
  order: number
  hidden?: boolean
  disabled?: boolean
}

export type AchievementDefinition = {
  id: string
  categoryId: string
  nameKey?: string
  descKey?: string
  rarity: string
  iconId?: string
  order: number
  hidden?: boolean
  disabled?: boolean
  prereqAchievementIds: string[]
  condition: Record<string, unknown>
  rewards: Array<Record<string, unknown>>
  stageProgressMode?: "sequential" | "cumulative" | string
  stages?: AchievementStageDefinition[]
  baseAchievementId?: string
  stageIndex?: number
  stageCount?: number
}

export type AchievementStageDefinition = {
  nameKey?: string
  descKey?: string
  rarity?: string
  iconId?: string
  order?: number
  hidden?: boolean
  disabled?: boolean
  condition?: Record<string, unknown>
  rewards?: Array<Record<string, unknown>>
}

export type PlayerAchievementStateView = {
  id: string
  completed: boolean
  claimed: boolean
  progressValue: number
  unlockedAt?: string
  completedAt?: string
  claimedAt?: string
}

export type PlayerAchievementView = AchievementDefinition & {
  locked: boolean
  completed: boolean
  claimed: boolean
  progressValue: number
  targetValue: number
  unlockedAt?: string
  completedAt?: string
  claimedAt?: string
}

export type PlayerAchievementList = {
  achievements: PlayerAchievementStateView[]
}

export type AchievementClaimResponse = {
  ok: boolean
  result?: {
    inventoryChanged?: boolean
    wardrobeChanged?: boolean
  }
}

export async function fetchAchievements(token: string): Promise<PlayerAchievementList> {
  return apiGet<PlayerAchievementList>("/api/achievements", token)
}

export async function claimAchievement(token: string, achievementId: string): Promise<AchievementClaimResponse> {
  return apiPost<AchievementClaimResponse>(`/api/achievements/${encodeURIComponent(achievementId)}/claim`, token, {})
}
