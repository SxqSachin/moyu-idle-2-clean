import { defineStore } from "pinia"

type SkillQuickRefDialogPayload = {
  skillId: string
  remainingCooldown?: number | null
  totalCooldown?: number | null
  unitAttributes?: Record<string, number> | null
  unitSkillLevel?: number | null
  unitSkillExp?: number | null
  runtimeSkill?: Record<string, unknown> | null
}

export const useSkillQuickRefDialogStore = defineStore("skillQuickRefDialog", {
  state: () => ({
    visible: false,
    skillId: "",
    remainingCooldown: null as number | null,
    totalCooldown: null as number | null,
    unitAttributes: null as Record<string, number> | null,
    unitSkillLevel: null as number | null,
    unitSkillExp: null as number | null,
    runtimeSkill: null as Record<string, unknown> | null
  }),
  actions: {
    open(payload: SkillQuickRefDialogPayload) {
      this.skillId = String(payload.skillId || "").trim()
      this.remainingCooldown = payload.remainingCooldown ?? null
      this.totalCooldown = payload.totalCooldown ?? null
      this.unitAttributes = payload.unitAttributes ?? null
      this.unitSkillLevel = payload.unitSkillLevel ?? null
      this.unitSkillExp = payload.unitSkillExp ?? null
      this.runtimeSkill = payload.runtimeSkill ?? null
      this.visible = this.skillId !== ""
    },
    close() {
      this.visible = false
      this.unitAttributes = null
      this.unitSkillLevel = null
      this.unitSkillExp = null
      this.runtimeSkill = null
    }
  }
})
