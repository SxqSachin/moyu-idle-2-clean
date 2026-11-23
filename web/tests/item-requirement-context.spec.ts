import { afterEach, describe, expect, it } from "vitest"
import { createPinia, setActivePinia } from "pinia"
import { createApp, h, nextTick, ref, type App } from "vue"
import { createI18n } from "vue-i18n"
import ElementPlus from "element-plus"
import { useEquipmentSlots } from "../src/composables/game/attributes/useEquipmentSlots"
import { useInventoryStore } from "../src/stores/inventory"
import { useItemPopoverDialogStore } from "../src/stores/item-popover-dialog"
import { useRoleStateStore } from "../src/stores/role-state"
import type { EquipmentItemDefinition, EquipmentItemInstance, EquipmentSlotType } from "../src/types/equipment"

const mountedApps: Array<{ app: App; container: HTMLElement }> = []

Object.defineProperty(window, "matchMedia", {
  writable: true,
  value: (query: string) => ({
    matches: false,
    media: query,
    onchange: null,
    addListener: () => {},
    removeListener: () => {},
    addEventListener: () => {},
    removeEventListener: () => {},
    dispatchEvent: () => false
  })
})

afterEach(() => {
  for (const view of mountedApps.splice(0)) {
    view.app.unmount()
    view.container.remove()
  }
})

function mountView(component: Parameters<typeof h>[0], props?: Record<string, unknown>) {
  const pinia = createPinia()
  setActivePinia(pinia)
  const i18n = createI18n({
    legacy: false,
    locale: "zh-CN",
    messages: {
      "zh-CN": {
        attr: {
          strength: { name: "力量", desc: "力量" },
          perception: { name: "感知", desc: "感知" }
        },
        game: {
          attrTooltipDescTitle: "说明",
          statusRequirements: "属性需求",
          itemDetailEquipmentEffect: "装备后",
          itemDetailBattleHooks: "战斗触发",
          itemDetailBattleHookNode: "效果 {index}: {trigger}",
          itemDetailBattleHookIf: "如果",
          itemDetailBattleHookThen: "则",
          itemDetailBattleHookEffectDamage: "对{target}造成{damageType}伤害：{formula}",
          itemDetailBattleHookAttrEquipmentEnhanceLevel: "强化等级",
          itemDetailBattleHookConditionIncomingHealGte: "实际治疗量不少于 {value}",
          itemDetailBattleHookTargetMode: { source: "关联目标" },
          itemDetailBattleHookTrigger: { on_after_deal_heal: "成功治疗后" },
          damageType: { magical: "魔法" },
          itemDetailAttrConversion: "属性转换"
        }
      }
    }
  })
  const container = document.createElement("div")
  document.body.appendChild(container)
  const app = createApp({ render: () => h(component, props || {}) })
  app.use(pinia)
  app.use(i18n)
  app.use(ElementPlus)
  app.mount(container)
  mountedApps.push({ app, container })
  return { container, pinia }
}

function minimalEquipmentDefinition(statusRequirement: Record<string, number>) {
  return {
    id: "test_helmet",
    equipType: "battle",
    slot: "head",
    occupySlots: ["head"],
    statusRequirement,
    accessoryGroup: "",
    affixSlotCount: 0,
    affixAllowedIds: [],
    baseAttrs: {},
    maxDurability: 0,
    decayRate: 0,
    enhanceMaxLevel: 0,
    enhanceDurationS: 0,
    enhanceExp: 0,
    enhanceProtectItemId: [],
    enhanceCostCurve: { base: {}, step: {}, overrides: {} },
    enhanceAttrCurve: { base: {}, step: {}, overrides: {} }
  } as EquipmentItemDefinition
}

describe("item equipment requirement context", () => {
  it("uses explicit requirement attributes for equipment requirement tags", async () => {
    const { default: EquipmentStaticInfo } = await import("../src/components/game/item/EquipmentStaticInfo.vue")
    const { container } = mountView(EquipmentStaticInfo, {
      equipmentDefinition: minimalEquipmentDefinition({ strength: 10 }),
      requirementAttributes: { strength: 12 }
    })
    await nextTick()

    expect(container.textContent).toContain("力量")
    expect(container.textContent).toContain(": 10 (12)")
  })

  it("falls back to player attributes when no requirement context is provided", async () => {
    const { default: EquipmentStaticInfo } = await import("../src/components/game/item/EquipmentStaticInfo.vue")
    const { container } = mountView(EquipmentStaticInfo, {
      equipmentDefinition: minimalEquipmentDefinition({ strength: 10 })
    })
    const roleStateStore = useRoleStateStore()
    roleStateStore.patchRoleAttributesPayload({
      roleType: "player",
      roleId: "self",
      upsert: {
        strength: { value: 7, category: "battle", unit: "number", growthType: "static", order: 1 }
      },
      meta: { version: 1 }
    })
    await nextTick()

    expect(container.textContent).toContain(": 10 (7)")
  })

  it("keeps long battle-hook text beside its bullet instead of creating an empty line", async () => {
    const { default: EquipmentStaticInfo } = await import("../src/components/game/item/EquipmentStaticInfo.vue")
    const equipmentDefinition = minimalEquipmentDefinition({})
    equipmentDefinition.battleHooks = [{
      id: "long_damage_hook",
      trigger: "on_after_deal_heal",
      priority: 0,
      conditions: [{ type: "incoming_heal_gte", value: 1 }],
      effects: [{
        kind: "damage",
        targetMode: "source",
        damageType: "magical",
        amountFormula: { baseAdd: 80, terms: [{ attrKey: "perception", coefficient: 0.55 }] }
      }]
    }]
    const { container } = mountView(EquipmentStaticInfo, { equipmentDefinition })
    await nextTick()

    const effectLine = container.querySelector(".battle-hook-node-effects .battle-hook-content-line")
    expect(effectLine?.querySelector(":scope > .battle-hook-bullet + .battle-hook-inline-text")).not.toBeNull()
    expect(effectLine?.querySelector(".battle-hook-inline-text")?.textContent).toContain("80")
  })

  it("rebuilds candidate requirement attributes when the active kitty changes", async () => {
    const pinia = createPinia()
    setActivePinia(pinia)
    const inventoryStore = useInventoryStore()
    inventoryStore.applySnapshot([
      {
        instanceId: 1001,
        templateItemId: "wood_sword",
        quantity: 1,
        state: "inventory",
        equipType: "battle",
        enhanceLevel: 0,
        durability: 10,
        maxDurability: 10
      } as EquipmentItemInstance
    ])

    const activeEquipType = ref<EquipmentSlotType>("battle")
    const activeKittyAttributes = ref<Record<string, number>>({ strength: 0 })
    const container = document.createElement("div")
    document.body.appendChild(container)
    let slotCards!: ReturnType<typeof useEquipmentSlots>["slotCards"]
    const app = createApp({
      setup() {
        slotCards = useEquipmentSlots(activeEquipType, {
          currentAttributes: activeKittyAttributes
        }).slotCards
        return () => null
      }
    })
    app.use(pinia)
    app.mount(container)
    mountedApps.push({ app, container })

    await nextTick()
    let rightHandCard = slotCards.value.find((card) => card.slot === "right_hand")
    expect(rightHandCard?.requirementAttributes.strength).toBe(0)
    let candidate = rightHandCard?.candidates[0]
    expect(candidate?.requirementAttributes.strength).toBe(0)
    expect(candidate?.canEquip).toBe(false)

    activeKittyAttributes.value = { strength: 12 }
    await nextTick()
    rightHandCard = slotCards.value.find((card) => card.slot === "right_hand")
    expect(rightHandCard?.requirementAttributes.strength).toBe(12)
    candidate = rightHandCard?.candidates[0]
    expect(candidate?.requirementAttributes.strength).toBe(12)
    expect(candidate?.canEquip).toBe(true)
  })

  it("clears mobile popover requirement attributes between opens", () => {
    const pinia = createPinia()
    setActivePinia(pinia)
    const dialogStore = useItemPopoverDialogStore()

    dialogStore.open({
      itemId: "wood_sword",
      instanceId: 1001,
      requirementAttributes: { strength: 12 }
    })
    expect(dialogStore.requirementAttributes).toEqual({ strength: 12 })

    dialogStore.close()
    expect(dialogStore.requirementAttributes).toBeNull()

    dialogStore.open({ itemId: "wood_sword", instanceId: 1001 })
    expect(dialogStore.requirementAttributes).toBeNull()
  })
})
