import { defineStore } from "pinia"
import { useGameStore } from "@/stores/game"
import { loadProgress, saveProgress } from "./persistence"
import { loadTutorialConfig, getAllTutorialIds } from "./configs"
import { startTutorialDriver } from "./driver-factory"
import { compareTutorialGuideEntries } from "./guide"
import { saveTutorialProgress, resetTutorialProgress, type TutorialProgressMap } from "@services/player-tutorial"
import type { TutorialConfig, TutorialProgress } from "./types"

type StartTutorialOptions = {
  ignorePrerequisites?: boolean
}

/** Merge two progress maps, keeping the entry with the higher version per id. */
function mergeProgress(
  a: Record<string, TutorialProgress>,
  b: Record<string, TutorialProgress>,
): Record<string, TutorialProgress> {
  const merged: Record<string, TutorialProgress> = { ...a }
  for (const [id, entry] of Object.entries(b)) {
    const existing = merged[id]
    if (!existing || (entry.version ?? 1) >= (existing.version ?? 1)) {
      merged[id] = entry
    }
  }
  return merged
}

export const useTutorialStore = defineStore("tutorial", {
  state: () => ({
    progress: {} as Record<string, TutorialProgress>,
    activeTutorialId: null as string | null,
    _initialized: false,
    _hydrated: false,
  }),

  getters: {
    isCompleted(): (tutorialId: string, version: number) => boolean {
      return (tutorialId, version) => {
        const p = this.progress[tutorialId]
        if (!p?.completed) return false
        return (p.version ?? 1) >= version
      }
    },

    isActive(): boolean {
      return this.activeTutorialId !== null
    },
  },

  actions: {
    _ensureInitialized() {
      if (this._initialized) return
      // Backend is the source of truth (hydrated from bootstrap). Until then,
      // fall back to the local cache so the guide panel isn't empty on first paint.
      const gameStore = useGameStore()
      this.progress = loadProgress(gameStore.userId)
      this._initialized = true
    },

    /**
     * Seed progress from the backend (bootstrap payload). Merges with any local
     * cache so a just-completed tutorial isn't lost to a stale snapshot.
     */
    hydrate(remote: TutorialProgressMap | undefined | null) {
      const gameStore = useGameStore()
      const local = loadProgress(gameStore.userId)
      this.progress = mergeProgress(local, remote ?? {})
      this._initialized = true
      this._hydrated = true
      // Persist the merged view back to the local cache.
      saveProgress(gameStore.userId, this.progress)
    },

    _persist() {
      const gameStore = useGameStore()
      saveProgress(gameStore.userId, this.progress)
    },

    /** Check if all prerequisites for a tutorial are completed. */
    isUnlocked(config: TutorialConfig): boolean {
      this._ensureInitialized()
      if (!config.prerequisites?.length) return true
      return config.prerequisites.every((prereqId) => {
        const p = this.progress[prereqId]
        return p?.completed === true
      })
    },

    async start(tutorialId: string, options: StartTutorialOptions = {}) {
      if (this.activeTutorialId) return
      const config = await loadTutorialConfig(tutorialId)
      if (!config) return
      this._ensureInitialized()
      if (!options.ignorePrerequisites && !this.isUnlocked(config)) return

      this.activeTutorialId = tutorialId

      try {
        await startTutorialDriver(
          config,
          () => this._markCompleted(tutorialId, config.version ?? 1),
          () => this._dismiss(),
        )
      } catch (err) {
        console.warn(`[tutorial] Failed to start tutorial: ${tutorialId}`, err)
        this._dismiss()
      }
    },

    _markCompleted(tutorialId: string, version: number) {
      this.progress[tutorialId] = {
        completed: true,
        completedAt: Date.now(),
        version,
      }
      this.activeTutorialId = null
      this._persist()
      // Fire-and-forget backend sync; local cache already reflects the change.
      const gameStore = useGameStore()
      if (gameStore.token) {
        void saveTutorialProgress(gameStore.token, tutorialId, true, version).catch((err) => {
          console.warn(`[tutorial] Failed to save progress to backend: ${tutorialId}`, err)
        })
      }
    },

    _dismiss() {
      this.activeTutorialId = null
    },

    reset(tutorialId: string) {
      this._ensureInitialized()
      delete this.progress[tutorialId]
      this._persist()
      const gameStore = useGameStore()
      if (gameStore.token) {
        void resetTutorialProgress(gameStore.token, tutorialId).catch((err) => {
          console.warn(`[tutorial] Failed to reset progress on backend: ${tutorialId}`, err)
        })
      }
    },

    /** Load all tutorial configs for the guide dialog. */
    async loadAll(): Promise<TutorialConfig[]> {
      this._ensureInitialized()
      const ids = getAllTutorialIds()
      const entries = (await Promise.all(
        ids.map(async (id) => {
          try {
            return await loadTutorialConfig(id)
          } catch (err) {
            console.warn(`[tutorial] Failed to load tutorial config: ${id}`, err)
            return null
          }
        }),
      )).filter((config): config is TutorialConfig => Boolean(config))
      return entries.sort(compareTutorialGuideEntries)
    },
  },
})
