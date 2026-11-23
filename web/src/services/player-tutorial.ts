import { apiGet, apiPost } from "@services/api"
import type { TutorialProgress } from "@/tutorial"

export type TutorialProgressMap = Record<string, TutorialProgress>

export async function fetchTutorialProgress(token: string) {
  return apiGet<{ tutorialProgress: TutorialProgressMap }>("/api/player/tutorial-progress", token)
}

export async function saveTutorialProgress(
  token: string,
  tutorialId: string,
  completed: boolean,
  version: number
) {
  return apiPost<{ tutorialId: string; completed: boolean; completedAt: number; version: number }>(
    "/api/player/tutorial-progress",
    token,
    { tutorialId, completed, version }
  )
}

export async function resetTutorialProgress(token: string, tutorialId: string) {
  return apiPost<{ tutorialId: string }>("/api/player/tutorial-progress/reset", token, { tutorialId })
}
