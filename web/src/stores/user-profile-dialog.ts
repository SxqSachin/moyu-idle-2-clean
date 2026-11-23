import { defineStore } from "pinia"

export const useUserProfileDialogStore = defineStore("user-profile-dialog", {
  state: () => ({
    visible: false,
    userId: 0
  }),
  actions: {
    open(userId: number) {
      const normalizedID = Math.floor(Number(userId || 0))
      if (normalizedID <= 0) return
      this.userId = normalizedID
      this.visible = true
    },
    close() {
      this.visible = false
      this.userId = 0
    }
  }
})
