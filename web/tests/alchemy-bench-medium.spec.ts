import { afterAll, afterEach, expect, it, vi } from "vitest"
import { createApp, nextTick, type App } from "vue"
import { createPinia, setActivePinia } from "pinia"
import { createI18n } from "vue-i18n"
import ElementPlus from "element-plus"
import Bench from "@components/game/alchemy/AlchemyBenchPanel.vue"
import { useInventoryStore } from "@stores/inventory"
import { usePlayerStore } from "@stores/player"
import { useOperationEffectsStore } from "@stores/operation-effects"
import { listAlchemyMediums, type AlchemyComboRecipe, type AlchemyTraitEffect } from "@services/alchemy-data"
import type { InventoryInstance } from "@/types/equipment"
import type { OperationEffectView } from "@services/operation-effects"
import zh from "@/locales/game/zh-CN"
import data from "@/locales/game-data/zh-CN"

vi.hoisted(() => {
  vi.stubGlobal("matchMedia", vi.fn(() => ({ matches: false, addEventListener: vi.fn(), removeEventListener: vi.fn() })))
  vi.stubGlobal("ResizeObserver", class { observe() {} unobserve() {} disconnect() {} })
})
afterAll(() => vi.unstubAllGlobals())

type Range = { min: number; max: number }
type Preview = Range & { attr: string }
type Line = { label: string; value: string }
type BenchState = {
  handleSelectMedium(id: number | null): void
  handleAddMaterial(id: number): void
  materials: Record<string, number>
  aggregatedAttrEntries: Preview[]
  formatAttrRange(attr: string, min: number, max: number): string
  effectRangeTooltipRows(effect: AlchemyTraitEffect, qty: number, family: string): Line[]
  formatEffectTotalPlain(effect: AlchemyTraitEffect, qty: number, family: string): string
  comboAttrPreviewEntries(combo: AlchemyComboRecipe, multiplier: number): Preview[]
  comboPowerRange(combo: AlchemyComboRecipe, multiplier: number): Range
}

let app: App | undefined
let host: HTMLElement | undefined
afterEach(() => { app?.unmount(); host?.remove(); vi.useRealTimers() })

function mountBench(precision = 0, stability = 0) {
  const pinia = createPinia()
  setActivePinia(pinia)
  const inventory = useInventoryStore()
  const mediums = listAlchemyMediums()
  inventory.instances = ["base4_cursed_wing_membrane", ...mediums.map(medium => medium.itemId)]
    .map((templateItemId, index) => ({ instanceId: index + 1, templateItemId, quantity: 10000, state: "inventory", customData: {} } as InventoryInstance))
  const player = usePlayerStore()
  const attribute = (value: number): NonNullable<typeof player.attributes>[string] => ({ value, unit: "number", category: "work", growthType: "static", order: 0 })
  player.attributes = { alchemy_level: attribute(80), alchemy_precision: attribute(precision), alchemy_craft_success: attribute(stability) }
  useOperationEffectsStore().applySnapshot({ revision: 1, serverNow: new Date().toISOString(), items: [] })
  host = document.createElement("div")
  document.body.append(host)
  app = createApp(Bench)
  app.use(pinia).use(ElementPlus).use(createI18n({ legacy: false, locale: "zh-CN", messages: { "zh-CN": { ...data, ...zh } } }))
  const state = (app.mount(host) as unknown as { $: { setupState: BenchState } }).$.setupState
  const select = (id: string | null) => state.handleSelectMedium(id === null ? null : inventory.instances.find(row => row.templateItemId === id)!.instanceId)
  return { state, select, mediums }
}

it("shows the same medium bonus on both bounds in material text and its explanation", () => {
  const { state, select } = mountBench()
  const effect = { attr: "attack", valuePerUnit: { min: 10, max: 100 } }
  expect(state.formatEffectTotalPlain(effect, 1, "alchemy")).toBe("10 ~ 100")
  select("moon_resin")
  expect(state.formatEffectTotalPlain(effect, 1, "alchemy")).toBe("12.2 ~ 122")
  const rows = state.effectRangeTooltipRows(effect, 1, "alchemy")
  expect(rows.find(row => row.label === "属性调整后")?.value).toBe("10 ~ 100")
  expect(rows.find(row => row.label === "介质修正")?.value).toBe("+22.00%")
  expect(rows.find(row => row.label === "最终预估")?.value).toBe("12.2 ~ 122")
  select(null)
  expect(state.formatEffectTotalPlain(effect, 1, "alchemy")).toBe("10 ~ 100")
})

it("preserves precision and stability before applying medium affinity", () => {
  const { state, select } = mountBench(1, 1)
  const effect = { attr: "attack", valuePerUnit: { min: 10, max: 100 } }
  expect(state.formatEffectTotalPlain(effect, 1, "alchemy")).toBe("50 ~ 163")
  select("moon_resin")
  expect(state.formatEffectTotalPlain(effect, 1, "alchemy")).toBe("61 ~ 198")
})

it("refreshes preview ranges when the authoritative attribute snapshot changes", async () => {
  const { state, select } = mountBench()
  const player = usePlayerStore()
  const effect = { attr: "attack", valuePerUnit: { min: 10, max: 100 } }
  select("moon_resin")
  const before = state.formatEffectTotalPlain(effect, 1, "alchemy")
  player.patchAttributesEvent({ meta: { version: 1 }, upsert: { alchemy_precision: { value: 1 }, alchemy_craft_success: { value: 1 } } })
  await nextTick()
  expect(state.formatEffectTotalPlain(effect, 1, "alchemy")).toBe("61 ~ 198")
  player.patchAttributesEvent({ meta: { version: 2 }, upsert: { alchemy_precision: { value: 0 }, alchemy_craft_success: { value: 0 } } })
  await nextTick()
  expect(state.formatEffectTotalPlain(effect, 1, "alchemy")).toBe(before)
})

it("combines active range buffs with medium bonuses and removes only the expired or exhausted effect", async () => {
  vi.useFakeTimers()
  vi.setSystemTime(new Date("2026-09-06T00:00:00.000Z"))
  const { state, select } = mountBench(1, 1)
  state.handleAddMaterial(1)
  state.materials.base4_cursed_wing_membrane = 48
  select("moon_resin")
  await nextTick()
  const effect = { attr: "attack", valuePerUnit: { min: 10, max: 100 } }
  const baseline = state.aggregatedAttrEntries.map(entry => ({ ...entry }))
  const cache = useOperationEffectsStore()
  const common = { rulesetVersion: "test", domain: "alchemy", targetKind: "player", validFrom: new Date().toISOString(), descKey: "", selector: { actionIds: ["alchemy.bench"] } } satisfies Partial<OperationEffectView>
  const floor: OperationEffectView = {
    ...common, grantId: 1, definitionId: "floor", kind: "alchemy.infusion_capacity_floor_tradeoff",
    nameKey: "game.fishCuisine.names.mossfire_dual_fish_grill", lifetimeKind: "charges", initialCharges: 1, remainingCharges: 1,
    params: { extraMaterialQuantity: 10, positiveFloorReductionRate: .2 }
  }
  const ceiling: OperationEffectView = {
    ...common, grantId: 2, definitionId: "ceiling", kind: "alchemy.infusion_precision_ceiling_bonus",
    nameKey: "game.fishCuisine.names.starlight_silverfish_soup", lifetimeKind: "duration", validUntil: "2026-09-06T00:00:03.000Z",
    params: { positiveCeilingBonusRate: .1 }
  }
  cache.applySnapshot({ revision: 2, serverNow: new Date().toISOString(), items: [floor, ceiling] })
  await nextTick()
  expect(state.formatEffectTotalPlain(effect, 1, "alchemy")).toBe("48.8 ~ 218")
  const rows = state.effectRangeTooltipRows(effect, 1, "alchemy")
  expect(rows.find(row => row.label === "最终预估")?.value).toBe("48.8 ~ 218")
  for (const entry of state.aggregatedAttrEntries) {
    const original = baseline.find(row => row.attr === entry.attr)!
    expect(entry.min).toBeCloseTo(original.min * (original.min > 0 ? .8 : 1), 3)
    expect(entry.max).toBeCloseTo(original.max * (original.max > 0 ? 1.1 : 1), 3)
  }
  const combo: AlchemyComboRecipe = {
    id: "preview", nameKey: "preview", descKey: "preview", kind: "combat", minAlchemyLevel: 1,
    inputs: {}, extraGoldCost: 0, extraDurationSec: 0, failBaseChance: 0, failChancePerLevelAbove: 0,
    priority: 0, active: true, outputAffix: { params: { attr: "attack" }, valueRange: { min: 10, max: 100 } }
  }
  expect(state.comboAttrPreviewEntries(combo, 1)[0]).toMatchObject({ min: 40, max: 178.75 })
  expect(state.comboPowerRange(combo, 1).min).toBe(40)
  expect(state.comboPowerRange(combo, 1).max).toBeCloseTo(178.75, 6)
  await vi.advanceTimersByTimeAsync(3100)
  await nextTick()
  expect(state.formatEffectTotalPlain(effect, 1, "alchemy")).toBe("48.8 ~ 198")
  expect(cache.items[0].remainingCharges).toBe(1)
  cache.applySnapshot({ revision: 3, serverNow: new Date().toISOString(), items: [{ ...floor, remainingCharges: 0 }, ceiling] })
  await nextTick()
  expect(state.formatEffectTotalPlain(effect, 1, "alchemy")).toBe("61 ~ 198")
  expect(state.aggregatedAttrEntries).toEqual(baseline)
})

it("updates real material and aggregate previews together when a medium is selected or removed", async () => {
  const { state, select } = mountBench(0.6, 0.6)
  state.handleAddMaterial(1)
  state.materials.base4_cursed_wing_membrane = 48
  await nextTick()
  const before = state.aggregatedAttrEntries.map(entry => ({ ...entry }))
  expect(before.length).toBeGreaterThan(0)
  select("crystal")
  await nextTick()
  for (const entry of state.aggregatedAttrEntries) {
    const original = before.find(row => row.attr === entry.attr)!
    for (const bound of ["min", "max"] as const) {
      expect(entry[bound]).toBeCloseTo(original[bound] * (original[bound] >= 0 ? 1.33 : 1.4), 3)
    }
    const expected = state.formatAttrRange(entry.attr, entry.min, entry.max)
    expect(host!.querySelector("[data-tutorial='alchemy-bench-preview']")!.textContent).toContain(expected)
    const materialValues = [...host!.querySelectorAll("[data-tutorial='alchemy-bench-materials'] .alchemy-effect-row__value")]
      .map(element => element.textContent?.trim())
    expect(materialValues).toContain(expected)
  }
  select(null)
  await nextTick()
  expect(state.aggregatedAttrEntries).toEqual(before)
})

it("uses the correct value sign for every medium in combination previews", () => {
  const { state, select, mediums } = mountBench()
  for (const medium of mediums) {
    select(medium.itemId)
    for (const range of [{ min: 10, max: 100 }, { min: -100, max: -10 }, { min: -10, max: 100 }]) {
      const combo: AlchemyComboRecipe = {
        id: "preview", nameKey: "preview", descKey: "preview", kind: "combat", minAlchemyLevel: 1,
        inputs: {}, extraGoldCost: 0, extraDurationSec: 0, failBaseChance: 0, failChancePerLevelAbove: 0,
        priority: 0, active: true, outputAffix: { params: { attr: "attack" }, valueRange: range }
      }
      const entries = state.comboAttrPreviewEntries(combo, 2)
      const power = state.comboPowerRange(combo, 2)
      for (const bound of ["min", "max"] as const) {
        const multiplier = range[bound] >= 0 ? medium.modifiers.valueMultiplier : medium.modifiers.negativeMultiplier
        expect(entries[0][bound]).toBeCloseTo(range[bound] * 2 * multiplier, 3)
        expect(power[bound]).toBeCloseTo(entries[0][bound], 3)
      }
    }
  }
})
