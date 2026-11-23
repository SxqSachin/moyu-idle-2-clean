import { defineStore } from "pinia"
import { computed, ref } from "vue"
import type { OperationEffectProjection, OperationEffectView } from "@services/operation-effects"

function timestamp(value: unknown): number {
  const parsed = Date.parse(String(value || ""))
  return Number.isFinite(parsed) ? parsed : 0
}

function monotonicNow(): number {
  return typeof performance !== "undefined" && typeof performance.now === "function"
    ? performance.now()
    : Date.now()
}

function normalizeProjection(payload: OperationEffectProjection) {
  const revision = Math.max(0, Math.trunc(Number(payload?.revision || 0)))
  const serverNowMs = timestamp(payload?.serverNow)
  const items = Array.isArray(payload?.items)
    ? payload.items.filter((item): item is OperationEffectView => {
        if (!item || typeof item !== "object") return false
        if (!Number.isFinite(Number(item.grantId)) || Number(item.grantId) <= 0) return false
        if (!String(item.definitionId || "").trim()) return false
        if (item.lifetimeKind === "duration") return timestamp(item.validUntil) > 0
        return item.lifetimeKind === "charges" && Number(item.remainingCharges || 0) > 0
      })
    : []
  return { revision, serverNowMs, items }
}

export const useOperationEffectsStore = defineStore("operation-effects", () => {
  const initialized = ref(false)
  const revision = ref(0)
  const items = ref<OperationEffectView[]>([])
  const serverNowMs = ref(0)
  const receivedAtMonotonicMs = ref(0)
  const receivedAtWallClockMs = ref(0)
  const stale = ref(false)

  const count = computed(() => items.value.length)

  function applySnapshot(payload: OperationEffectProjection | null | undefined): boolean {
    if (!payload || typeof payload !== "object") return false
    const next = normalizeProjection(payload)
    if (!next.serverNowMs) return false
    if (initialized.value && next.revision < revision.value) return false

    const currentSnapshotTime = serverNowMs.value
    if (initialized.value && next.revision === revision.value && next.serverNowMs < currentSnapshotTime) {
      return false
    }

    revision.value = next.revision
    items.value = next.items
    serverNowMs.value = next.serverNowMs
    receivedAtMonotonicMs.value = monotonicNow()
    receivedAtWallClockMs.value = Date.now()
    initialized.value = true
    stale.value = false
    return true
  }

  function estimatedServerNow(clientMonotonicMs = monotonicNow(), clientWallClockMs = Date.now()): number {
    if (!initialized.value || !serverNowMs.value) return Date.now()
    const monotonicElapsed = clientMonotonicMs - receivedAtMonotonicMs.value
    const wallClockElapsed = clientWallClockMs - receivedAtWallClockMs.value
    return serverNowMs.value + Math.max(0, monotonicElapsed, wallClockElapsed)
  }

  function activeItemsAt(serverTimestampMs: number): OperationEffectView[] {
    return items.value.filter((item) => {
      if (timestamp(item.validFrom) > serverTimestampMs) return false
      if (item.lifetimeKind === "charges") return Number(item.remainingCharges || 0) > 0
      const validUntil = timestamp(item.validUntil)
      return validUntil > serverTimestampMs
    })
  }

  function reset() {
    initialized.value = false
    revision.value = 0
    items.value = []
    serverNowMs.value = 0
    receivedAtMonotonicMs.value = 0
    receivedAtWallClockMs.value = 0
    stale.value = false
  }

  function markStale() {
    if (initialized.value) stale.value = true
  }

  return {
    initialized,
    revision,
    items,
    count,
    stale,
    applySnapshot,
    estimatedServerNow,
    activeItemsAt,
    markStale,
    reset
  }
})
