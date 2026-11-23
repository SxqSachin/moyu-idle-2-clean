import { defineStore } from "pinia"

type ActionQuickRefDialogPayload = {
  actionId: string
  count?: number
}

export type ActionQuickRefDialogEntry = {
  id: number
  actionId: string
  count: number
  visible: boolean
}

export const useActionQuickRefDialogStore = defineStore("actionQuickRefDialog", {
  state: () => ({
    nextId: 1,
    entries: [] as ActionQuickRefDialogEntry[]
  }),
  actions: {
    open(payload: ActionQuickRefDialogPayload) {
      const actionId = String(payload.actionId || "").trim()
      if (!actionId) return 0
      const entry = {
        id: this.nextId++,
        actionId,
        count: Math.max(1, Math.floor(Number(payload.count || 1))),
        visible: true
      }
      this.entries.push(entry)
      return entry.id
    },
    close(id: number) {
      if (!this.entries.length) return
      const index = this.entries.findIndex((entry) => entry.id === id)
      if (index < 0) return
      this.entries.splice(index)
    },
    updateCount(id: number, count: number) {
      const entry = this.entries.find((row) => row.id === id)
      if (!entry) return
      entry.count = Math.max(1, Math.floor(Number(count || 1)))
    }
  }
})
