import type { TutorialProgress } from "./types"

const STORAGE_KEY_PREFIX = "tutorial:progress:"

export function loadProgress(userId: number): Record<string, TutorialProgress> {
  if (userId <= 0) return {}
  try {
    const raw = localStorage.getItem(`${STORAGE_KEY_PREFIX}${userId}`)
    if (!raw) return {}
    const parsed = JSON.parse(raw)
    if (parsed && typeof parsed === "object") return parsed as Record<string, TutorialProgress>
    return {}
  } catch {
    return {}
  }
}

export function saveProgress(userId: number, progress: Record<string, TutorialProgress>): void {
  if (userId <= 0) return
  try {
    localStorage.setItem(`${STORAGE_KEY_PREFIX}${userId}`, JSON.stringify(progress))
  } catch {
    // storage full — silently ignore
  }
}
