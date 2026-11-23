import { defineStore } from "pinia"
import type { WorldDifficultySettings } from "@services/battle-difficulty"

export type EnemyQuickRefDifficultyContext = {
  worldId: string
  worldLabel: string
  worldDifficultySettings: WorldDifficultySettings | null
}

type EnemyQuickRefPayload = {
  enemyId: string
  difficultyContext?: EnemyQuickRefDifficultyContext | null
}

type EnemyQuickRefDialogState = {
  visible: boolean
  enemyId: string
  difficultyContext: EnemyQuickRefDifficultyContext | null
}

export const useEnemyUnitQuickRefDialogStore = defineStore("enemyUnitQuickRefDialog", {
  state: (): EnemyQuickRefDialogState => ({
    visible: false,
    enemyId: "",
    difficultyContext: null
  }),
  actions: {
    open(payload: EnemyQuickRefPayload) {
      this.enemyId = String(payload.enemyId || "").trim()
      const context = payload.difficultyContext
      const worldId = String(context?.worldId || "").trim()
      this.difficultyContext = worldId
        ? {
            worldId,
            worldLabel: String(context?.worldLabel || "").trim(),
            worldDifficultySettings: context?.worldDifficultySettings || null
          }
        : null
      this.visible = this.enemyId !== ""
    },
    close() {
      this.visible = false
    }
  }
})
