import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"
import { effectScope, nextTick, reactive, ref, type EffectScope } from "vue"
import { useTransmuteFood } from "@/composables/game/useTransmuteFood"

const mocks = vi.hoisted(() => ({ apiGet: vi.fn(), game: {} as { token: string }, revision: {} as { value: number } }))
vi.mock("@services/api", () => ({ apiGet: mocks.apiGet }))
vi.mock("@stores/game", () => ({ useGameStore: () => mocks.game }))
vi.mock("@/composables/game/useOperationEffects", () => ({ useOperationEffects: () => ({ revision: mocks.revision }) }))
vi.mock("@/composables/game/useOperationEffectContext", async () => {
  const { ref } = await import("vue")
  return { useOperationEffectContext: () => ({ initialized: ref(true), stale: ref(false), applicable: ref([]) }) }
})

let scope: EffectScope
beforeEach(() => {
  mocks.apiGet.mockReset()
  mocks.game = reactive({ token: "account-a" })
  mocks.revision = ref(1)
  scope = effectScope()
})
afterEach(() => scope.stop())

describe("point-gold food eligibility", () => {
  it("requests authoritative eligibility for ordinary inputs without a trophy whitelist", async () => {
    mocks.apiGet.mockResolvedValue({ eligible: true })
    const food = scope.run(() => useTransmuteFood(() => "wood"))!
    await nextTick()
    expect(mocks.apiGet).toHaveBeenCalledWith("/api/operation-effects/transmute-eligibility/wood", "account-a")
    expect(food.context.value.transmuteGoldEligible).toBe(true)
  })

  it("does not reuse an old item's response or eligibility after logout", async () => {
    let resolveOld!: (value: { eligible: boolean }) => void
    mocks.apiGet.mockReturnValueOnce(new Promise(resolve => { resolveOld = resolve }))
    mocks.apiGet.mockResolvedValueOnce({ eligible: false })
    const id = ref("wood")
    const food = scope.run(() => useTransmuteFood(() => id.value))!
    id.value = "gold_coin"
    await nextTick()
    resolveOld({ eligible: true })
    await nextTick()
    expect(food.context.value.transmuteGoldEligible).toBe(false)
    mocks.game.token = ""
    await nextTick()
    expect(food.context.value.transmuteGoldEligible).toBe(false)
    expect(mocks.apiGet).toHaveBeenCalledTimes(2)
  })

  it("fails closed on read errors and retries when the effect revision changes", async () => {
    mocks.apiGet.mockRejectedValueOnce(new Error("offline"))
    const food = scope.run(() => useTransmuteFood(() => "bottled_starlight"))!
    await nextTick()
    expect(food.context.value.transmuteGoldEligible).toBe(false)
    mocks.apiGet.mockResolvedValueOnce({ eligible: true })
    mocks.revision.value++
    await nextTick()
    await nextTick()
    expect(food.context.value.transmuteGoldEligible).toBe(true)
  })
})
