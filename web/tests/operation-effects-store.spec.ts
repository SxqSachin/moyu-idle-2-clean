import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"
import { createPinia, setActivePinia } from "pinia"
import { useOperationEffectsStore } from "../src/stores/operation-effects"
import type { OperationEffectProjection, OperationEffectView } from "../src/services/operation-effects"

function effect(overrides: Partial<OperationEffectView> = {}): OperationEffectView {
  return {
    grantId: 1,
    definitionId: "test.life.refund",
    rulesetVersion: "test-v1",
    domain: "life_action",
    kind: "life_action.material_refund_chance",
    nameKey: "operation_effect.test.name",
    descKey: "operation_effect.test.desc",
    targetKind: "player",
    lifetimeKind: "duration",
    validFrom: "2026-09-04T00:00:00.000Z",
    validUntil: "2026-09-04T00:10:00.000Z",
    sourceItemId: "test_food",
    params: {},
    ...overrides
  }
}

function projection(revision: number, serverNow: string, items: OperationEffectView[]): OperationEffectProjection {
  return { revision, serverNow, items }
}

describe("operation effects store", () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.useFakeTimers()
    vi.setSystemTime(new Date("2026-09-04T00:01:00.000Z"))
  })

  afterEach(() => {
    vi.restoreAllMocks()
    vi.useRealTimers()
  })

  it("rejects older revisions and older snapshots at the same revision", () => {
    const store = useOperationEffectsStore()
    const current = effect()

    expect(store.applySnapshot(projection(2, "2026-09-04T00:01:00.000Z", [current]))).toBe(true)
    expect(store.applySnapshot(projection(1, "2026-09-04T00:02:00.000Z", []))).toBe(false)
    expect(store.applySnapshot(projection(2, "2026-09-04T00:00:59.000Z", []))).toBe(false)
    expect(store.items).toEqual([current])
  })

  it("uses the server clock baseline and expires duration effects at the exact boundary", () => {
    const store = useOperationEffectsStore()
    const duration = effect({ validUntil: "2026-09-04T00:01:05.000Z" })
    vi.spyOn(performance, "now").mockReturnValue(1_000)
    store.applySnapshot(projection(1, "2026-09-04T00:01:00.000Z", [duration]))

    expect(store.activeItemsAt(store.estimatedServerNow(5_999))).toHaveLength(1)

    expect(store.activeItemsAt(store.estimatedServerNow(6_000))).toHaveLength(0)
  })

  it("uses wall-clock elapsed time when the monotonic clock pauses during sleep", () => {
    const store = useOperationEffectsStore()
    const duration = effect({ validUntil: "2026-09-04T00:01:05.000Z" })
    vi.spyOn(performance, "now").mockReturnValue(1_000)
    store.applySnapshot(projection(1, "2026-09-04T00:01:00.000Z", [duration]))

    vi.setSystemTime(new Date("2026-09-04T00:01:06.000Z"))
    expect(store.activeItemsAt(store.estimatedServerNow(1_000))).toHaveLength(0)
  })

  it("keeps positive charge effects independent of wall-clock expiry", () => {
    const store = useOperationEffectsStore()
    const charged = effect({
      lifetimeKind: "charges",
      validUntil: undefined,
      initialCharges: 3,
    remainingCharges: 2
    })
    store.applySnapshot(projection(4, "2026-09-04T00:01:00.000Z", [charged]))

    vi.advanceTimersByTime(86_400_000)
    expect(store.activeItemsAt(store.estimatedServerNow())).toEqual([charged])
  })

  it("does not present a grant before its server-side validFrom", () => {
    const store = useOperationEffectsStore()
    const future = effect({
      validFrom: "2026-09-04T00:02:00.000Z",
      validUntil: "2026-09-04T00:12:00.000Z"
    })
    store.applySnapshot(projection(1, "2026-09-04T00:01:00.000Z", [future]))

    expect(store.activeItemsAt(store.estimatedServerNow())).toHaveLength(0)
  })

  it("accepts a newer server snapshot at the same revision", () => {
    const store = useOperationEffectsStore()
    store.applySnapshot(projection(7, "2026-09-04T00:01:00.000Z", [effect()]))

    expect(store.applySnapshot(projection(7, "2026-09-04T00:02:00.000Z", []))).toBe(true)
    expect(store.items).toEqual([])
  })
})
