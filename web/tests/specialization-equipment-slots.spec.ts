import { afterEach, describe, expect, it, vi } from "vitest"
import { createPinia, setActivePinia } from "pinia"
import { computed, ref } from "vue"
import { useEquipmentSlots } from "../src/composables/game/attributes/useEquipmentSlots"
import { resolveAllowedEquipSlots } from "../src/services/item-definition"
import {
  allocatePlayerSpecialization,
  filterPlayerBattleEquipSlots,
  normalizeDisabledSkillImpacts
} from "../src/services/specialization"
import type { EquipmentItemInstance, EquipmentSlotType, EquipmentTemplateDefinition } from "../src/types/equipment"

function template(slot: string): EquipmentTemplateDefinition {
  return { slot } as EquipmentTemplateDefinition
}

describe("specialization equipment topology", () => {
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it("projects the authoritative enabled slot list into equipment cards", () => {
    setActivePinia(createPinia())
    const equipType = ref<EquipmentSlotType>("battle")
    const enabledSlots = ref([
      "body",
      "left_hand",
      "right_hand",
      "right_hand_2",
      "accessory_1",
      "accessory_2",
      "accessory_3",
      "accessory_4"
    ])
    const { slotCards } = useEquipmentSlots(equipType, {
      visibleSlots: computed(() => enabledSlots.value)
    })

    expect(slotCards.value.map((card) => card.slot)).toEqual([
      "left_hand",
      "body",
      "right_hand",
      "right_hand_2",
      "accessory_1",
      "accessory_2",
      "accessory_3",
      "accessory_4"
    ])
    expect(slotCards.value.some((card) => card.slot === "head")).toBe(false)
  })

  it("removes the second right-hand card immediately when a reset restores base topology", () => {
    setActivePinia(createPinia())
    const equipType = ref<EquipmentSlotType>("battle")
    const enabledSlots = ref(["right_hand", "right_hand_2", "accessory_1"])
    const equippedItems = ref([{
      instanceId: 91,
      templateItemId: "cat_sword",
      enhanceLevel: 0,
      durability: 100,
      maxDurability: 100,
      state: "equipped" as EquipmentItemInstance["state"],
      equippedSlot: "right_hand_2",
      equipType: "battle" as const
    }])
    const { slotCards } = useEquipmentSlots(equipType, {
      visibleSlots: computed(() => enabledSlots.value),
      equippedItems
    })

    expect(slotCards.value.find((card) => card.slot === "right_hand_2")?.instance?.instanceId).toBe(91)

    enabledSlots.value = ["right_hand", "accessory_1"]
    equippedItems.value = [{
      ...equippedItems.value[0]!,
      state: "inventory",
      equippedSlot: ""
    }]

    expect(slotCards.value.some((card) => card.slot === "right_hand_2")).toBe(false)
    expect(slotCards.value.some((card) => card.instance?.instanceId === 91)).toBe(false)
  })

  it("keeps the legacy base topology while specialization state is unavailable", () => {
    setActivePinia(createPinia())
    const equipType = ref<EquipmentSlotType>("battle")
    const { slotCards } = useEquipmentSlots(equipType, { visibleSlots: [] })

    expect(slotCards.value.some((card) => card.slot === "right_hand")).toBe(true)
    expect(slotCards.value.some((card) => card.slot === "right_hand_2")).toBe(false)
    expect(slotCards.value.some((card) => card.slot === "accessory_5")).toBe(false)
  })

  it("allows eligible templates to target specialization-created concrete slots", () => {
    expect(resolveAllowedEquipSlots(template("right_hand"))).toContain("right_hand_2")
    expect(resolveAllowedEquipSlots(template("head"))).toEqual(["head", "head_2"])
    expect(resolveAllowedEquipSlots(template("body"))).toEqual(["body", "body_2"])
    expect(resolveAllowedEquipSlots(template("arm"))).toEqual(["arm", "arm_2"])
    expect(resolveAllowedEquipSlots(template("feet"))).toEqual(["feet", "feet_2"])
    expect(resolveAllowedEquipSlots(template("accessory"))).toEqual([
      "accessory_1",
      "accessory_2",
      "accessory_3",
      "accessory_4",
      "accessory_5",
      "accessory_6",
      "accessory_7",
      "accessory_8"
    ])
  })

  it("fails closed to base player slots until specialization topology is available", () => {
    expect(filterPlayerBattleEquipSlots(resolveAllowedEquipSlots(template("right_hand")), undefined)).toEqual([
      "right_hand"
    ])
    expect(filterPlayerBattleEquipSlots(resolveAllowedEquipSlots(template("accessory")), undefined)).toEqual([
      "accessory_1",
      "accessory_2",
      "accessory_3",
      "accessory_4"
    ])
    expect(filterPlayerBattleEquipSlots(resolveAllowedEquipSlots(template("accessory")), [
      "accessory_1",
      "accessory_2",
      "accessory_3",
      "accessory_4",
      "accessory_5",
      "accessory_6",
      "accessory_7",
      "accessory_8"
    ])).toHaveLength(8)
  })

  it("turns the 409 impact challenge into a typed confirmation preview", async () => {
    const preview = {
      committed: false,
      requiresConfirmation: true,
      status: {
        treeId: "player_combat_specialization",
        rulesetVersion: "specialization-v5",
        revision: 2,
        totalPoints: 18,
        spentPoints: 10,
        availablePoints: 8,
        allocations: [],
        resetCost: { itemId: "gold_coin", count: 5000 }
      },
      effectiveSlots: [],
      slotCapacities: {},
      impact: {
        digest: "impact-digest",
        disabledSlots: ["head"],
        enabledSlots: ["accessory_5", "accessory_6"],
        autoUnequipped: [{ instanceId: 12, templateItemId: "test_hat", fromSlots: ["head"], reason: "slot_disabled" }],
        disabledSkills: [
          { skillId: "guard_stance", reason: "requirements_unmet" },
          { skillId: "guard_stance", reason: "slot_capacity_exceeded" }
        ]
      }
    }
    vi.stubGlobal("fetch", vi.fn(async () => new Response(JSON.stringify({
      ok: false,
      error: { code: "SPECIALIZATION_IMPACT_CONFIRMATION_REQUIRED", message: "confirmation required" },
      data: preview,
      ts: Date.now()
    }), { status: 409, headers: { "Content-Type": "application/json" } })))

    const result = await allocatePlayerSpecialization(
      "token",
      "player_combat_specialization",
      "perception_terminal_crownless_astrolabe",
      { expectedRevision: 2, expectedRulesetVersion: "specialization-v5" }
    )
    expect(result).toEqual(preview)
    expect(normalizeDisabledSkillImpacts(result.impact)).toEqual([{
      skillId: "guard_stance",
      reasons: ["requirements_unmet", "slot_capacity_exceeded"]
    }])
  })
})
