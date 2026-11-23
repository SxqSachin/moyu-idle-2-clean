import { defineStore } from "pinia"

export type ProfileDetailType = "collectibles" | "achievements"

export const useProfileDetailDialogStore = defineStore("profile-detail-dialog", {
  state: () => ({
    visible: false,
    userId: 0,
    detailType: "collectibles" as ProfileDetailType
  }),
  actions: {
    open(userId: number, detailType: ProfileDetailType) {
      const normalizedID = Math.floor(Number(userId || 0))
      if (normalizedID <= 0) return
      this.userId = normalizedID
      this.detailType = detailType
      this.visible = true
    },
    close() {
      this.visible = false
      this.userId = 0
    }
  }
})
