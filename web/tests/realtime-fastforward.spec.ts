import { beforeEach, describe, expect, it } from "vitest"
import { createPinia, setActivePinia } from "pinia"
import { useRealtimeStore } from "../src/stores/realtime"

describe("realtime offline fast-forward projection", () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it("keeps reward summaries separate from the enhance event stream", () => {
    const store = useRealtimeStore()
    const result = {
      queueId: 7,
      actionId: "equipment.enhance",
      attemptNo: 1,
      equipmentInstanceId: 9,
      beforeLevel: 3,
      afterLevel: 4,
      success: true,
      usedProtect: false,
      successRate: 0.5,
      costs: {},
      deltaAttrs: {}
    }

    store.appendEnhanceResult(result)
    store.patchFastForwardRewards({ enhanceResults: [result] })

    expect(store.fastForwardRewards?.enhanceResults).toEqual([result])
    expect(store.enhanceResults).toEqual([result])

    store.$reset()
    store.patchFastForwardRewards({ enhanceResults: [result] })
    store.appendEnhanceResult(result)

    expect(store.enhanceResults).toEqual([result])
  })

  it("keeps the newest attempts when a terminal batch overlaps realtime events", () => {
    const store = useRealtimeStore()
    const results = Array.from({ length: 140 }, (_, index) => ({
      queueId: 7,
      actionId: "equipment.enhance",
      attemptNo: index + 1,
      equipmentInstanceId: 9,
      beforeLevel: 0,
      afterLevel: 0,
      success: false,
      usedProtect: false,
      successRate: 0.5,
      costs: {},
      deltaAttrs: {}
    }))
    for (const result of results.slice(40)) {
      store.appendEnhanceResult(result)
    }

    store.patchFastForwardRewards({ enhanceResults: results })

    expect(store.enhanceResults).toHaveLength(100)
    expect(store.enhanceResults.map((result) => result.attemptNo)).toEqual(
      Array.from({ length: 100 }, (_, index) => index + 41)
    )
  })
})
