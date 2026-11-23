import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"
import { createPinia, setActivePinia } from "pinia"
import { useGameStore } from "../src/stores/game"
import { useSpecializationStore } from "../src/stores/specialization"
import type { SpecializationMutationResult, SpecializationRulesetResetNotice } from "../src/services/specialization"

function summary(revision: number, rulesetReset?: SpecializationRulesetResetNotice) {
  return {
    tree: {
      id: "player_combat_specialization",
      schemaVersion: 2,
      rulesetVersion: "specialization-v6",
      balanceRevision: 1,
      ownerType: "player",
      nameKey: "specialization.tree.player_combat_specialization.name",
      descKey: "specialization.tree.player_combat_specialization.desc",
      initialPoints: 2,
      maxPoints: 65,
      maxActiveRootNodes: 3,
      unlock: { battleLevel: 10 },
      pointBands: [],
      reset: { itemId: "gold_coin", count: 5000 },
      schools: [],
      nodes: [],
      ui: { layout: "five_spoke", schoolOrder: [] }
    },
    status: {
      treeId: "player_combat_specialization",
      rulesetVersion: "specialization-v6",
      balanceRevision: 1,
      revision,
      battleLevel: 90,
      formulaPoints: 18,
      bonusPoints: 0,
      totalPoints: 18,
      spentPoints: 0,
      availablePoints: 18,
      pointBreakdown: {
        unlockGrantPoints: 2,
        levelMilestonePoints: 16,
        rewardPoints: 0,
        countedRewardPoints: 0,
        cappedPoints: 0,
        maxPoints: 65,
        atCap: false,
        nextPointBattleLevel: 100,
        nextPointPoints: 1,
        milestoneBands: [],
        sources: []
      },
      allocations: [],
      nodeStates: [],
      resetCost: { itemId: "gold_coin", count: 5000 }
    },
    characterAttributes: {},
    effectiveSlots: [],
    slotCapacities: {},
    rulesetReset
  }
}

function mutationResult(revision: number, committed = false) {
  const current = summary(revision)
  return {
    committed,
    requiresConfirmation: !committed,
    status: current.status,
    effectiveSlots: current.effectiveSlots,
    slotCapacities: current.slotCapacities,
    characterAttributes: current.characterAttributes,
    impact: {
      digest: `impact-${revision}`,
      disabledSlots: [],
      enabledSlots: [],
      autoUnequipped: [],
      disabledSkills: [{ skillId: "skill-a", reason: "requirements_unmet" }]
    }
  }
}

function jsonResponse(data: unknown) {
  return new Response(JSON.stringify({ ok: true, data }), {
    status: 200,
    headers: { "Content-Type": "application/json" }
  })
}

function deferredResponse() {
  let resolve!: (response: Response) => void
  const promise = new Promise<Response>((done) => { resolve = done })
  return { promise, resolve }
}

describe("specialization store principal isolation", () => {
  beforeEach(() => setActivePinia(createPinia()))
  afterEach(() => vi.unstubAllGlobals())

  it.each(["allocate", "reset"] as const)("waits for the active refresh before %s uses its revision", async (operation) => {
    const refresh = deferredResponse()
    let gets = 0
    const posts: unknown[] = []
    vi.stubGlobal("fetch", vi.fn((_input: RequestInfo | URL, init?: RequestInit) => {
      if (init?.method === "POST") {
        posts.push(JSON.parse(String(init.body)))
        return Promise.resolve(jsonResponse(mutationResult(22)))
      }
      return ++gets === 1 ? Promise.resolve(jsonResponse(summary(11))) : refresh.promise
    }))
    const game = useGameStore()
    game.userId = 1
    game.token = "token-a"
    const specialization = useSpecializationStore()
    await specialization.load()
    const loading = specialization.load(true)
    const mutation = operation === "allocate" ? specialization.allocate("strength_root_break_stance") : specialization.reset()
    expect(posts).toHaveLength(0)
    refresh.resolve(jsonResponse(summary(22)))
    await loading
    await mutation
    expect(posts).toEqual([expect.objectContaining({ expectedRevision: 22 })])
  })

  it.each(["allocate", "reset"] as const)("does not let a pre-commit GET overwrite a committed %s", async (operation) => {
    const post = deferredResponse()
    const oldGet = deferredResponse()
    let gets = 0
    vi.stubGlobal("fetch", vi.fn((_input: RequestInfo | URL, init?: RequestInit) => {
      if (init?.method === "POST") return post.promise
      gets += 1
      if (gets === 2) return oldGet.promise
      return Promise.resolve(jsonResponse(summary(gets === 1 ? 11 : 12)))
    }))
    const game = useGameStore()
    game.userId = 1
    game.token = "token-a"
    const specialization = useSpecializationStore()
    await specialization.load()
    const mutation = operation === "allocate" ? specialization.allocate("strength_root_break_stance") : specialization.reset()
    const outcome = expect(mutation).resolves.toMatchObject({ committed: true })
    await vi.waitFor(() => expect(specialization.mutating).toBe(true))
    const staleLoad = specialization.load(true)
    post.resolve(jsonResponse(mutationResult(12, true)))
    await outcome
    expect(gets).toBe(3)
    oldGet.resolve(jsonResponse(summary(11)))
    await expect(staleLoad).resolves.toBeNull()
    expect(specialization.summary?.status.revision).toBe(12)
    expect(specialization.mutating).toBe(false)
  })

  it.each(["allocate", "reset"] as const)("preserves a committed %s when its follow-up GET fails", async (operation) => {
    let gets = 0
    let posts = 0
    const initial = summary(11)
    initial.characterAttributes = { strength: 80 }
    const committed: SpecializationMutationResult = mutationResult(12, true)
    delete committed.characterAttributes
    committed.effectiveSlots = [{
      slot: "off_hand_2", family: "hand", ordinal: 4,
      enabled: operation === "allocate", accepts: ["weapon"]
    }]
    committed.slotCapacities = { hand: operation === "allocate" ? 4 : 2 }
    if (operation === "allocate") {
      committed.status.allocations = [{ nodeId: "strength_root_break_stance", rank: 2, paidCost: 3 }]
      committed.status.spentPoints = 3
      committed.status.availablePoints = 15
    }
    vi.stubGlobal("fetch", vi.fn((_input: RequestInfo | URL, init?: RequestInit) => {
      if (init?.method === "POST") {
        posts += 1
        return Promise.resolve(jsonResponse(committed))
      }
      return ++gets === 1
        ? Promise.resolve(jsonResponse(initial))
        : Promise.reject(new Error("refresh unavailable"))
    }))
    const game = useGameStore()
    game.userId = 1
    game.token = "token-a"
    const specialization = useSpecializationStore()
    await specialization.load()
    const mutation = operation === "allocate" ? specialization.allocate("strength_root_break_stance") : specialization.reset()
    await expect(mutation).resolves.toMatchObject({ committed: true })
    expect(specialization.summary?.status.revision).toBe(12)
    expect(specialization.summary?.status).toEqual(committed.status)
    expect(specialization.summary?.effectiveSlots).toEqual(committed.effectiveSlots)
    expect(specialization.summary?.slotCapacities).toEqual(committed.slotCapacities)
    expect(specialization.enabledBattleSlotIds).toEqual(operation === "allocate" ? ["off_hand_2"] : [])
    expect(specialization.summary?.characterAttributes).toEqual(initial.characterAttributes)
    expect(specialization.loadError).toBe("refresh unavailable")
    expect(posts).toBe(1)
    expect(specialization.mutating).toBe(false)
  })

  it("rejects overlapping writes without replacing the in-flight mutation", async () => {
    const post = deferredResponse()
    let posts = 0
    vi.stubGlobal("fetch", vi.fn((_input: RequestInfo | URL, init?: RequestInit) => {
      if (init?.method === "POST") { posts += 1; return post.promise }
      return Promise.resolve(jsonResponse(summary(11)))
    }))
    const game = useGameStore()
    game.userId = 1
    game.token = "token-a"
    const specialization = useSpecializationStore()
    await specialization.load()
    const first = specialization.allocate("strength_root_break_stance")
    await expect(specialization.reset()).rejects.toMatchObject({ code: "REQUEST_DUPLICATE" })
    expect(posts).toBe(1)
    expect(specialization.mutating).toBe(true)
    post.resolve(jsonResponse(mutationResult(11)))
    await first
    expect(specialization.mutating).toBe(false)
  })

  it("starts a new authoritative GET after conflict instead of joining a pre-conflict GET", async () => {
    const post = deferredResponse()
    const oldGet = deferredResponse()
    let gets = 0
    vi.stubGlobal("fetch", vi.fn((_input: RequestInfo | URL, init?: RequestInit) => {
      if (init?.method === "POST") return post.promise
      gets += 1
      if (gets === 2) return oldGet.promise
      return Promise.resolve(jsonResponse(summary(gets === 1 ? 11 : 22)))
    }))
    const game = useGameStore()
    game.userId = 1
    game.token = "token-a"
    const specialization = useSpecializationStore()
    await specialization.load()
    const mutation = specialization.allocate("strength_root_break_stance")
    const outcome = expect(mutation).rejects.toMatchObject({ code: "SPECIALIZATION_REVISION_CONFLICT" })
    await vi.waitFor(() => expect(specialization.mutating).toBe(true))
    const staleLoad = specialization.load(true)
    post.resolve(new Response(JSON.stringify({
      ok: false,
      error: { code: "SPECIALIZATION_REVISION_CONFLICT", message: "conflict" }
    }), { status: 409, headers: { "Content-Type": "application/json" } }))
    await outcome
    expect(gets).toBe(3)
    oldGet.resolve(jsonResponse(summary(11)))
    await expect(staleLoad).resolves.toBeNull()
    expect(specialization.summary?.status.revision).toBe(22)
  })

  it.each(["allocate", "reset"] as const)("isolates the committed %s refresh when the account changes", async (operation) => {
    const oldGet = deferredResponse()
    let gets = 0
    vi.stubGlobal("fetch", vi.fn((_input: RequestInfo | URL, init?: RequestInit) => {
      if (init?.method === "POST") return Promise.resolve(jsonResponse(mutationResult(12, true)))
      gets += 1
      if (gets === 2) return oldGet.promise
      return Promise.resolve(jsonResponse(summary(gets === 1 ? 11 : 22)))
    }))
    const game = useGameStore()
    game.userId = 1
    game.token = "token-a"
    const specialization = useSpecializationStore()
    await specialization.load()
    const mutation = operation === "allocate" ? specialization.allocate("strength_root_break_stance") : specialization.reset()
    const outcome = expect(mutation).rejects.toMatchObject({ code: "SPECIALIZATION_PRINCIPAL_CHANGED" })
    await vi.waitFor(() => expect(gets).toBe(2))
    expect(specialization.summary?.status.revision).toBe(12)
    game.userId = 2
    game.token = "token-b"
    await specialization.load()
    oldGet.resolve(jsonResponse(summary(12)))
    await outcome
    expect(specialization.summary?.status.revision).toBe(22)
    expect(specialization.mutating).toBe(false)
  })

  it("does not dispatch a write if a pending prerequisite GET was invalidated by clear", async () => {
    const get = deferredResponse()
    let posts = 0
    vi.stubGlobal("fetch", vi.fn((_input: RequestInfo | URL, init?: RequestInit) => {
      if (init?.method === "POST") posts += 1
      return get.promise
    }))
    const game = useGameStore()
    game.userId = 1
    game.token = "token-a"
    const specialization = useSpecializationStore()
    const mutation = specialization.allocate("strength_root_break_stance")
    const outcome = expect(mutation).rejects.toMatchObject({ code: "SPECIALIZATION_PRINCIPAL_CHANGED" })
    specialization.clear()
    get.resolve(jsonResponse(summary(11)))
    await outcome
    expect(posts).toBe(0)
    expect(specialization.summary).toBeNull()
  })

  it("retains the forced ruleset reset receipt and returned equipment", async () => {
    const rulesetReset: SpecializationRulesetResetNotice = {
      reason: "ruleset_changed",
      previousRulesetVersion: "specialization-v5",
      currentRulesetVersion: "specialization-v6",
      previousRevision: 8,
      currentRevision: 9,
      removedAllocations: [{ nodeId: "three_armed", rank: 1, paidCost: 4 }],
      impact: {
        digest: "ruleset-reset-9",
        disabledSlots: ["right_hand_2"],
        enabledSlots: [],
        autoUnequipped: [{ instanceId: 202, templateItemId: "old_blade", fromSlots: ["right_hand_2"], reason: "slot_disabled" }],
        disabledSkills: []
      }
    }
    vi.stubGlobal("fetch", vi.fn(() => Promise.resolve(jsonResponse(summary(9, rulesetReset)))))

    const game = useGameStore()
    const specialization = useSpecializationStore()
    game.userId = 1
    game.token = "token-a"
    await specialization.load()

    expect(specialization.summary?.rulesetReset).toEqual(rulesetReset)
    expect(specialization.summary?.rulesetReset?.impact.autoUnequipped[0]).toMatchObject({
      instanceId: 202,
      fromSlots: ["right_hand_2"]
    })
  })

  it("does not reuse or apply an in-flight response after the account changes", async () => {
    const pending: Array<(response: Response) => void> = []
    vi.stubGlobal("fetch", vi.fn(() => new Promise<Response>((resolve) => pending.push(resolve))))

    const game = useGameStore()
    const specialization = useSpecializationStore()
    game.userId = 1
    game.token = "token-a"
    const oldLoad = specialization.load()

    game.userId = 2
    game.token = "token-b"
    const newLoad = specialization.load()
    expect(pending).toHaveLength(2)

    pending[1](new Response(JSON.stringify({ ok: true, data: summary(22) }), {
      status: 200,
      headers: { "Content-Type": "application/json" }
    }))
    await expect(newLoad).resolves.toMatchObject({ status: { revision: 22 } })

    pending[0](new Response(JSON.stringify({ ok: true, data: summary(11) }), {
      status: 200,
      headers: { "Content-Type": "application/json" }
    }))
    await expect(oldLoad).resolves.toBeNull()
    expect(specialization.summary?.status.revision).toBe(22)
  })

  it("invalidates an in-flight response when logout clears the store", async () => {
    let resolveRequest: ((response: Response) => void) | undefined
    vi.stubGlobal("fetch", vi.fn(() => new Promise<Response>((resolve) => { resolveRequest = resolve })))

    const game = useGameStore()
    const specialization = useSpecializationStore()
    game.userId = 1
    game.token = "token-a"
    const load = specialization.load()
    specialization.clear()
    game.clearAuth()

    resolveRequest?.(new Response(JSON.stringify({ ok: true, data: summary(11) }), {
      status: 200,
      headers: { "Content-Type": "application/json" }
    }))
    await expect(load).resolves.toBeNull()
    expect(specialization.summary).toBeNull()
  })

  it.each(["allocate", "reset"] as const)("fails a stale %s closed and clears mutation state on principal switch", async (operation) => {
    let resolveMutation: ((response: Response) => void) | undefined
    let getCount = 0
    vi.stubGlobal("fetch", vi.fn((_input: RequestInfo | URL, init?: RequestInit) => {
      if (init?.method === "POST") return new Promise<Response>((resolve) => { resolveMutation = resolve })
      getCount += 1
      return Promise.resolve(jsonResponse(summary(getCount === 1 ? 11 : 22)))
    }))

    const game = useGameStore()
    const specialization = useSpecializationStore()
    game.userId = 1
    game.token = "token-a"
    await specialization.load()

    const mutation = operation === "allocate"
      ? specialization.allocate("strength_root_break_stance")
      : specialization.reset()
    await vi.waitFor(() => expect(specialization.mutating).toBe(true))

    game.userId = 2
    game.token = "token-b"
    await specialization.load()
    expect(specialization.mutating).toBe(false)
    expect(specialization.summary?.status.revision).toBe(22)

    resolveMutation?.(jsonResponse(mutationResult(11)))
    await expect(mutation).rejects.toMatchObject({ code: "SPECIALIZATION_PRINCIPAL_CHANGED" })
    expect(specialization.summary?.status.revision).toBe(22)
    expect(specialization.mutating).toBe(false)
  })

  it("clears mutating after a committed allocation refresh advances the load generation", async () => {
    let getCount = 0
    vi.stubGlobal("fetch", vi.fn((_input: RequestInfo | URL, init?: RequestInit) => {
      if (init?.method === "POST") return Promise.resolve(jsonResponse(mutationResult(12, true)))
      getCount += 1
      return Promise.resolve(jsonResponse(summary(getCount === 1 ? 11 : 12)))
    }))

    const game = useGameStore()
    const specialization = useSpecializationStore()
    game.userId = 1
    game.token = "token-a"
    await specialization.load()
    await expect(specialization.allocate("strength_root_break_stance")).resolves.toMatchObject({ committed: true })
    expect(specialization.summary?.status.revision).toBe(12)
    expect(specialization.mutating).toBe(false)
  })

  it("reuses the operation idempotency key when an impact preview is confirmed", async () => {
    const postKeys: string[] = []
    let postCount = 0
    let getCount = 0
    vi.stubGlobal("fetch", vi.fn((_input: RequestInfo | URL, init?: RequestInit) => {
      if (init?.method === "POST") {
        postCount += 1
        postKeys.push(new Headers(init.headers).get("X-Idempotency-Key") || "")
        return Promise.resolve(jsonResponse(mutationResult(postCount === 1 ? 11 : 12, postCount > 1)))
      }
      getCount += 1
      return Promise.resolve(jsonResponse(summary(getCount === 1 ? 11 : 12)))
    }))

    const game = useGameStore()
    const specialization = useSpecializationStore()
    game.userId = 1
    game.token = "token-a"
    await specialization.load()

    const idempotencyKey = "specialization-allocate-fixed"
    await expect(specialization.allocate("strength_root_break_stance", "", idempotencyKey))
      .resolves.toMatchObject({ committed: false, requiresConfirmation: true })
    await expect(specialization.allocate("strength_root_break_stance", "impact-11", idempotencyKey))
      .resolves.toMatchObject({ committed: true })

    expect(postKeys).toEqual([idempotencyKey, idempotencyKey])
    expect(specialization.summary?.status.revision).toBe(12)
  })

  it.each([
    ["allocate", "SPECIALIZATION_REVISION_CONFLICT"],
    ["allocate", "SPECIALIZATION_RULESET_CONFLICT"],
    ["allocate", "SPECIALIZATION_IMPACT_CONFLICT"],
    ["reset", "SPECIALIZATION_REVISION_CONFLICT"],
    ["reset", "SPECIALIZATION_RULESET_CONFLICT"],
    ["reset", "SPECIALIZATION_IMPACT_CONFLICT"]
  ] as const)("refreshes after a stale %s %s without replaying the mutation", async (operation, errorCode) => {
    const postKeys: string[] = []
    const postBodies: Array<Record<string, unknown>> = []
    let postCount = 0
    let getCount = 0
    vi.stubGlobal("fetch", vi.fn((_input: RequestInfo | URL, init?: RequestInit) => {
      if (init?.method === "POST") {
        postCount += 1
        postKeys.push(new Headers(init.headers).get("X-Idempotency-Key") || "")
        postBodies.push(JSON.parse(String(init.body || "{}")) as Record<string, unknown>)
        return Promise.resolve(new Response(JSON.stringify({
          ok: false,
          error: {
            code: errorCode,
            message: "specialization state conflict"
          }
        }), {
          status: 409,
          headers: { "Content-Type": "application/json" }
        }))
      }
      getCount += 1
      return Promise.resolve(jsonResponse(summary(getCount === 1 ? 11 : 22)))
    }))

    const game = useGameStore()
    const specialization = useSpecializationStore()
    game.userId = 1
    game.token = "token-a"
    await specialization.load()

    const idempotencyKey = `specialization-${operation}-${errorCode.toLowerCase()}`
    const mutation = operation === "allocate"
      ? specialization.allocate("strength_root_break_stance", "", idempotencyKey)
      : specialization.reset("", idempotencyKey)

    await expect(mutation).rejects.toMatchObject({
      code: errorCode,
      status: 409
    })
    expect(postKeys).toEqual([idempotencyKey])
    expect(postBodies).toEqual([expect.objectContaining({
      expectedRevision: 11,
      expectedRulesetVersion: "specialization-v6"
    })])
    expect(getCount).toBe(2)
    expect(postCount).toBe(1)
    expect(specialization.summary?.status.revision).toBe(22)
    expect(specialization.mutating).toBe(false)
  })

  it.each([
    ["allocate", "SPECIALIZATION_POINTS_INSUFFICIENT"],
    ["reset", "SPECIALIZATION_RESET_COST_INSUFFICIENT"]
  ] as const)("does not force-refresh after an ordinary %s rejection", async (operation, errorCode) => {
    let getCount = 0
    let postCount = 0
    vi.stubGlobal("fetch", vi.fn((_input: RequestInfo | URL, init?: RequestInit) => {
      if (init?.method === "POST") {
        postCount += 1
        return Promise.resolve(new Response(JSON.stringify({
          ok: false,
          error: {
            code: errorCode,
            message: "specialization operation rejected"
          }
        }), {
          status: 409,
          headers: { "Content-Type": "application/json" }
        }))
      }
      getCount += 1
      return Promise.resolve(jsonResponse(summary(11)))
    }))

    const game = useGameStore()
    const specialization = useSpecializationStore()
    game.userId = 1
    game.token = "token-a"
    await specialization.load()

    const mutation = operation === "allocate"
      ? specialization.allocate("strength_root_break_stance")
      : specialization.reset()
    await expect(mutation).rejects.toMatchObject({ code: errorCode })
    expect(getCount).toBe(1)
    expect(postCount).toBe(1)
    expect(specialization.summary?.status.revision).toBe(11)
    expect(specialization.mutating).toBe(false)
  })

  it("preserves the mutation conflict when the authoritative refresh fails", async () => {
    let getCount = 0
    let postCount = 0
    vi.stubGlobal("fetch", vi.fn((_input: RequestInfo | URL, init?: RequestInit) => {
      if (init?.method === "POST") {
        postCount += 1
        return Promise.resolve(new Response(JSON.stringify({
          ok: false,
          error: {
            code: "SPECIALIZATION_REVISION_CONFLICT",
            message: "specialization revision conflict"
          }
        }), {
          status: 409,
          headers: { "Content-Type": "application/json" }
        }))
      }
      getCount += 1
      if (getCount === 1) return Promise.resolve(jsonResponse(summary(11)))
      return Promise.resolve(new Response(JSON.stringify({
        ok: false,
        error: { code: "TEMPORARY_UNAVAILABLE", message: "refresh unavailable" }
      }), {
        status: 503,
        headers: { "Content-Type": "application/json" }
      }))
    }))

    const game = useGameStore()
    const specialization = useSpecializationStore()
    game.userId = 1
    game.token = "token-a"
    await specialization.load()

    await expect(specialization.allocate("strength_root_break_stance")).rejects.toMatchObject({
      code: "SPECIALIZATION_REVISION_CONFLICT",
      status: 409
    })
    expect(getCount).toBe(2)
    expect(postCount).toBe(1)
    expect(specialization.summary?.status.revision).toBe(11)
    expect(specialization.loadError).toBe("refresh unavailable")
    expect(specialization.mutating).toBe(false)
  })

  it("does not refresh the old principal when its conflict arrives after an account switch", async () => {
    let resolveMutation: ((response: Response) => void) | undefined
    let getCount = 0
    vi.stubGlobal("fetch", vi.fn((_input: RequestInfo | URL, init?: RequestInit) => {
      if (init?.method === "POST") {
        return new Promise<Response>((resolve) => { resolveMutation = resolve })
      }
      getCount += 1
      return Promise.resolve(jsonResponse(summary(getCount === 1 ? 11 : 22)))
    }))

    const game = useGameStore()
    const specialization = useSpecializationStore()
    game.userId = 1
    game.token = "token-a"
    await specialization.load()
    const mutation = specialization.allocate("strength_root_break_stance")
    await vi.waitFor(() => expect(specialization.mutating).toBe(true))

    game.userId = 2
    game.token = "token-b"
    await specialization.load()
    resolveMutation?.(new Response(JSON.stringify({
      ok: false,
      error: {
        code: "SPECIALIZATION_REVISION_CONFLICT",
        message: "specialization revision conflict"
      }
    }), {
      status: 409,
      headers: { "Content-Type": "application/json" }
    }))

    await expect(mutation).rejects.toMatchObject({ code: "SPECIALIZATION_REVISION_CONFLICT" })
    expect(getCount).toBe(2)
    expect(specialization.summary?.status.revision).toBe(22)
    expect(specialization.mutating).toBe(false)
  })
})
