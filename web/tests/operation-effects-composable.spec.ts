import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"
import { createPinia, setActivePinia } from "pinia"
import { useOperationEffects } from "../src/composables/game/useOperationEffects"
import type { OperationEffectProjection, OperationEffectView } from "../src/services/operation-effects"
import { infusionMaterialQuantityBonus } from "../src/services/alchemy-operation-effects"

const serviceMocks = vi.hoisted(() => ({
  fetchOperationEffects: vi.fn(),
  useOperationEffectItem: vi.fn()
}))

vi.mock("@services/operation-effects", () => serviceMocks)

function effect(overrides: Partial<OperationEffectView>): OperationEffectView {
  return {
    grantId: 1,
    definitionId: "test.alchemy",
    rulesetVersion: "test-v1",
    domain: "alchemy",
    kind: "alchemy.infusion_capacity_floor_tradeoff",
    nameKey: "effect.test.name",
    descKey: "effect.test.desc",
    targetKind: "player",
    lifetimeKind: "charges",
    validFrom: "2026-09-04T00:00:00.000Z",
    initialCharges: 3,
    remainingCharges: 3,
    selector: { actionIds: ["alchemy.bench"] },
    params: { extraMaterialQuantity: 8, positiveFloorReductionRate: 0.1 },
    ...overrides
  }
}

describe("useOperationEffects", () => {
  it("previews disjoint tiers and charges one penalty for three precious slots", () => {
    const state = useOperationEffects()
    state.hydrate({ revision: 1, serverNow: "2026-09-04T00:01:00.000Z", items: [
      effect({grantId:1,kind:"alchemy.infusion_bulk_capacity_floor_tradeoff",params:{materialTier:1,extraMaterialQuantity:50,quantityLimitBonusRate:.20,positiveFloorReductionRate:.12,successRatePenalty:.04}}),
      effect({grantId:2,kind:"alchemy.infusion_bulk_capacity_floor_tradeoff",params:{materialTier:2,extraMaterialQuantity:35,quantityLimitBonusRate:.15,positiveFloorReductionRate:.08,successRatePenalty:.05}}),
      effect({grantId:3,kind:"alchemy.infusion_precious_capacity_bonus",params:{extraMaterialQuantity:25,quantityLimitBonusRate:.12,successRatePenalty:.06}})
    ] })
    const all = state.alchemyInfusionModifiers.value
    for (const [rarity, bonus] of [["common",2000],["uncommon",2000],["rare",1500],["epic",1500],["legendary",1200],["mythic",1200]] as const) expect(infusionMaterialQuantityBonus(rarity,10000,all)).toBe(bonus)
    expect(state.alchemyInfusionModifiersForMaterials(["rare"]).tierCapacities?.[1]).toBeUndefined()
    expect(state.alchemyInfusionModifiersForMaterials(["common"]).tierCapacities?.[2]).toBeUndefined()
    expect(state.alchemyInfusionModifiersForMaterials(["legendary","legendary","legendary"]).successRatePenalty).toBe(.06)
    expect(state.alchemyInfusionModifiersForMaterials(["common","rare","legendary"]).successRatePenalty).toBeCloseTo(.15)
    expect(infusionMaterialQuantityBonus("common",100,all)).toBe(50)
  })
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.useFakeTimers()
    vi.setSystemTime(new Date("2026-09-04T00:01:00.000Z"))
    useOperationEffects().reset()
    serviceMocks.fetchOperationEffects.mockReset()
    serviceMocks.useOperationEffectItem.mockReset()
  })

  afterEach(() => vi.useRealTimers())

  it("submits a batch once and applies its authoritative effect projection", async () => {
    const state = useOperationEffects()
    const payload: OperationEffectProjection = { revision: 9, serverNow: "2026-09-04T00:01:00.000Z", items: [effect({ remainingCharges: 22000 })] }
    serviceMocks.useOperationEffectItem.mockResolvedValue(payload)
    await state.activate("token-a", 5, { targetKind: 'kitty', targetActorId: 7 }, "batch-request", 100)
    expect(serviceMocks.useOperationEffectItem).toHaveBeenCalledTimes(1)
    expect(serviceMocks.useOperationEffectItem).toHaveBeenCalledWith("token-a", 5, { targetKind: 'kitty', targetActorId: 7 }, "batch-request", 100)
    expect(state.revision.value).toBe(9)
    expect(state.activeEffects.value[0].remainingCharges).toBe(22000)
  })

  it("exposes independent bounded fishing modifiers from the shared cache", () => {
    const state = useOperationEffects()
    const castEffect = (grantId: number, kind: string, rate: number) => effect({ grantId, domain: "fishing_cast", kind, selector: { actionIds: ["fishing.trip"] }, params: { rate } })
    state.hydrate({ revision: 1, serverNow: "2026-09-04T00:01:00.000Z", items: [
      castEffect(1, "fishing_cast.line_load_reduction", 0.08),
      castEffect(2, "fishing_cast.landing_bonus", 0.1),
      castEffect(3, "fishing_cast.landing_bonus", 0.1),
      { ...castEffect(4, "fishing_cast.line_load_reduction", 0.1), targetKind: "kitty", targetActorId: 1 }
    ] })
    expect(state.fishingFightModifiers.value).toEqual({ lineLoadReductionRate: 0.08, landingBonusRate: 0.15 })
    expect(state.fishingBiteBonus.value).toBe(0)
  })

  it("exposes one authoritative cache and folds only matching player infusion effects", () => {
    const first = useOperationEffects()
    const second = useOperationEffects()
    const payload: OperationEffectProjection = {
      revision: 4,
      serverNow: "2026-09-04T00:01:00.000Z",
      items: [
        effect({}),
        effect({ grantId: 2, params: { extraMaterialQuantity: 4, positiveFloorReductionRate: 0.2 } }),
        effect({
          grantId: 3,
          kind: "alchemy.infusion_precision_ceiling_bonus",
          params: { positiveCeilingBonusRate: 0.07 }
        }),
        effect({ grantId: 4, selector: { actionIds: ["alchemy.convert"] } }),
        effect({ grantId: 5, targetKind: "kitty", targetActorId: 9 })
      ]
    }

    expect(first.hydrate(payload)).toBe(true)
    expect(second.revision.value).toBe(4)
    expect(second.alchemyInfusionModifiers.value).toEqual({
      extraMaterialQuantity: 12,
      bulkMaterialQuantity: 0,
      preciousMaterialQuantity: 0,
      bulkQuantityLimitBonusRate: 0,
      preciousQuantityLimitBonusRate: 0,
      positiveFloorReductionRate: 0.30000000000000004,
      positiveCeilingBonusRate: 0.07,
      successRatePenalty: 0,
      successRateBonus: 0
    })
    expect(second.effectsFor({ domain: "alchemy", actionId: "alchemy.bench", actorKind: "player" })).toHaveLength(3)
  })

  it("does not restore a logged-out cache when an older request finishes late", async () => {
    let resolveOldRequest: ((payload: OperationEffectProjection) => void) | undefined
    serviceMocks.fetchOperationEffects.mockReturnValueOnce(new Promise<OperationEffectProjection>((resolve) => {
      resolveOldRequest = resolve
    }))
    const operationEffects = useOperationEffects()
    const pending = operationEffects.refresh("token-a")

    operationEffects.reset()
    operationEffects.hydrate({
      revision: 2,
      serverNow: "2026-09-04T00:02:00.000Z",
      items: [effect({ grantId: 2 })]
    })
    resolveOldRequest?.({
      revision: 99,
      serverNow: "2026-09-04T00:03:00.000Z",
      items: [effect({ grantId: 99 })]
    })
    await pending

    expect(operationEffects.revision.value).toBe(2)
    expect(operationEffects.activeEffects.value.map((item) => item.grantId)).toEqual([2])
  })

  it("excludes the whole bulk tradeoff when no eligible material is selected", () => {
    const state = useOperationEffects()
    state.hydrate({ revision: 1, serverNow: "2026-09-04T00:01:00.000Z", items: [effect({kind: "alchemy.infusion_bulk_capacity_floor_tradeoff",params:{extraMaterialQuantity:363,positiveFloorReductionRate:0.12}})] })
    expect(state.alchemyInfusionModifiersForMaterials(["mythic", "legendary"]).bulkMaterialQuantity).toBe(0)
    expect(state.alchemyInfusionModifiersForMaterials(["mythic", "legendary"]).positiveFloorReductionRate).toBe(0)
    expect(state.alchemyInfusionModifiersForMaterials(["mythic", "epic"]).bulkMaterialQuantity).toBe(363)
    expect(state.alchemyInfusionModifiersForMaterials(["mythic", "epic"]).positiveFloorReductionRate).toBe(0.12)
  })

  it("applies precious food only to high-rarity selections without adding a floor penalty", () => {
    const state = useOperationEffects()
    state.hydrate({revision:1,serverNow:"2026-09-04T00:01:00.000Z",items:[effect({kind:"alchemy.infusion_precious_capacity_bonus",targetKind:"player_and_kitties",params:{extraMaterialQuantity:25}})]})
    for (const materials of [[], ["epic"], ["unknown"]]) expect(state.alchemyInfusionModifiersForMaterials(materials).preciousMaterialQuantity).toBe(0)
    for (const materials of [["legendary"], ["mythic"], ["mythic","epic"]]) {
      expect(state.alchemyInfusionModifiersForMaterials(materials)).toMatchObject({preciousMaterialQuantity:25,bulkMaterialQuantity:0,positiveFloorReductionRate:0})
    }
  })

  it("folds success bonuses once per infusion without adding capacity", () => {
    const state=useOperationEffects()
    state.hydrate({revision:1,serverNow:"2026-09-04T00:01:00.000Z",items:[effect({kind:'alchemy.infusion_success_bonus',params:{bonusRate:.03}})]})
    expect(state.alchemyInfusionModifiersForMaterials([]).successRateBonus).toBe(0)
    const mods=state.alchemyInfusionModifiersForMaterials(['legendary','legendary','legendary'])
    expect(mods.successRateBonus).toBe(.03)
    expect(infusionMaterialQuantityBonus('legendary',100,mods)).toBe(0)
    expect(mods.positiveFloorReductionRate).toBe(0)
    expect(mods.positiveCeilingBonusRate).toBe(0)
  })

  it("adds only eligible food penalties once for a mixed infusion", () => {
    const state = useOperationEffects()
    state.hydrate({revision:1,serverNow:"2026-09-04T00:01:00.000Z",items:[
      effect({grantId:1,kind:"alchemy.infusion_bulk_capacity_floor_tradeoff",params:{extraMaterialQuantity:50,positiveFloorReductionRate:.12,successRatePenalty:.04}}),
      effect({grantId:2,kind:"alchemy.infusion_precious_capacity_bonus",params:{extraMaterialQuantity:25,successRatePenalty:.06}})
    ]})
    expect(state.alchemyInfusionModifiersForMaterials([]).successRatePenalty).toBe(0)
    expect(state.alchemyInfusionModifiersForMaterials(["epic","epic"]).successRatePenalty).toBe(.04)
    expect(state.alchemyInfusionModifiersForMaterials(["mythic"]).successRatePenalty).toBe(.06)
    expect(state.alchemyInfusionModifiersForMaterials(["mythic","epic"]).successRatePenalty).toBe(.1)
  })

  it("rejects websocket snapshots after the scoped session logs out", () => {
    let token = "token-a"
    const operationEffects = useOperationEffects({ token: () => token })
    expect(operationEffects.hydrate({
      revision: 1,
      serverNow: "2026-09-04T00:01:00.000Z",
      items: [effect({})]
    })).toBe(true)

    operationEffects.reset()
    token = ""
    expect(operationEffects.hydrate({
      revision: 2,
      serverNow: "2026-09-04T00:02:00.000Z",
      items: [effect({ grantId: 2 })]
    })).toBe(false)
    expect(operationEffects.activeEffects.value).toEqual([])
  })

  it("does not mark the new account stale when an old account request fails late", async () => {
    let fail!: (error: Error) => void
    serviceMocks.fetchOperationEffects.mockReturnValueOnce(new Promise((_, reject) => { fail = reject }))
    const state = useOperationEffects()
    const pending = state.refresh("old-account").catch(() => undefined)
    state.reset()
    state.hydrate({ revision: 1, serverNow: "2026-09-04T00:01:00.000Z", items: [] })
    fail(new Error("old request failed"))
    await pending
    expect(state.stale.value).toBe(false)
  })

  it("exposes the same shared grant to every actor and preserves player-only grants", () => {
    const state = useOperationEffects()
    state.hydrate({revision:1,serverNow:"2026-09-04T00:01:00.000Z",items:[effect({domain:"enhancement",kind:"enhancement.success_bonus",targetKind:"player_and_kitties",selector:{actionIds:["equipment_enhance"]},params:{bonusRate:.025}})]})
    for (const actor of [{actorKind:"player" as const},{actorKind:"kitty" as const,actorId:7},{actorKind:"kitty" as const,actorId:99}]) {
      expect(state.enhancementSuccessBonusFor(actor)).toBe(.025)
      expect(state.effectsFor({domain:"enhancement",actionId:"equipment_enhance",...actor})[0].grantId).toBe(1)
    }
    expect(state.activeEffects.value).toHaveLength(1)
    expect(state.activeEffects.value[0].remainingCharges).toBe(3)
  })
})
