import { defineStore } from "pinia"
import { computed, ref } from "vue"
import type { FishingTrip } from "@services/fishing"

export const useFishingStore = defineStore("fishing", () => {
  const trip = ref<FishingTrip | null>(null)

  const running = computed(() => trip.value?.status === "running")

  function applyTripPayload(payload: unknown) {
    if (!payload || typeof payload !== "object") return
    const next = (payload as { trip?: FishingTrip | null }).trip
    trip.value = next || null
  }

  function setTrip(next: FishingTrip | null) {
    trip.value = next
  }

  return { trip, running, applyTripPayload, setTrip }
})
