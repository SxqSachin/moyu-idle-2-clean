import { describe, expect, it } from "vitest"
import type { ActionDefinition } from "@game-data/game-data"
import { buildProductionPlan, flattenProductionPlan } from "@/services/action-production-plan"

function action(def: Partial<ActionDefinition> & Pick<ActionDefinition, "id">): ActionDefinition {
  return {
    id: def.id,
    mainType: def.mainType || "crafting",
    subType: def.subType || "common",
    menu: def.menu || "crafting",
    disabled: def.disabled ?? false,
    rarity: def.rarity || "common",
    nameKey: def.nameKey || `action.${def.id}.name`,
    queueable: def.queueable ?? true,
    durationS: def.durationS ?? 1,
    costs: def.costs || {},
    itemReward: def.itemReward || {},
    statusExpReward: def.statusExpReward || {},
    statusRequirement: def.statusRequirement,
    buildingRequirement: def.buildingRequirement,
    descKey: def.descKey,
    iconId: def.iconId,
    economyIndustryTier: def.economyIndustryTier,
    economyDifficultyMultiplier: def.economyDifficultyMultiplier
  }
}

describe("action-production-plan", () => {
  it.each([
    "fishing.trip", "equipment_enhance", "building_upgrade",
    "alchemy.convert", "alchemy.bench", "alchemy.inherit", "alchemy.transmute", "alchemy.smelt"
  ])("never treats %s as an automatic material producer", (id) => {
    const special = action({ id, itemReward: { ingredient: 1 } })
    const root = action({ id: "craft_result", costs: { ingredient: 1 }, itemReward: { result: 1 } })
    const plan = buildProductionPlan({
      rootAction: root, rootCount: 1, actions: [special, root], inventoryByItemId: {},
      selectedRecipeByNode: { "root/action:craft_result/material:ingredient": id }
    })
    expect(plan.actionSteps.map((step) => step.actionId)).toEqual([root.id])
    expect(plan.root.costs[0].options).toEqual([])
    expect(plan.unresolvedMaterials).toEqual([{ itemId: "ingredient", deficit: 1 }])
  })

  it("expands multi-level chain with inventory deduction", () => {
    const actions: ActionDefinition[] = [
      action({ id: "mine_ore", durationS: 10, itemReward: { iron_ore: 1 } }),
      action({ id: "collect_coal", durationS: 8, itemReward: { coal: 1 } }),
      action({ id: "logging", durationS: 6, itemReward: { wood: 1 } }),
      action({ id: "smelt_iron", durationS: 20, costs: { iron_ore: 2, coal: 1 }, itemReward: { iron_ingot: 1 } }),
      action({ id: "forge_sword", durationS: 30, costs: { iron_ingot: 2, wood: 1 }, itemReward: { iron_sword: 1 } })
    ]

    const plan = buildProductionPlan({
      rootAction: actions[4],
      rootCount: 2,
      actions,
      inventoryByItemId: {
        iron_ore: 3,
        wood: 1
      }
    })

    const byAction = new Map(plan.actionSteps.map((row) => [row.actionId, row.count]))
    expect(byAction.get("forge_sword")).toBe(2)
    expect(byAction.get("smelt_iron")).toBe(4)
    expect(byAction.get("mine_ore")).toBe(5)
    expect(byAction.get("collect_coal")).toBe(4)
    expect(byAction.get("logging")).toBe(1)
    expect(plan.unresolvedMaterials).toHaveLength(0)
  })

  it("supports recipe switching on multi-producer node", () => {
    const actions: ActionDefinition[] = [
      action({ id: "saw_fast", durationS: 10, costs: { wood: 2 }, itemReward: { plank: 2 } }),
      action({ id: "saw_slow", durationS: 8, costs: { wood: 1 }, itemReward: { plank: 1 } }),
      action({ id: "logging", durationS: 5, itemReward: { wood: 1 } }),
      action({ id: "make_chair", durationS: 20, costs: { plank: 2 }, itemReward: { chair: 1 } })
    ]

    const firstPlan = buildProductionPlan({
      rootAction: actions[3],
      rootCount: 1,
      actions,
      inventoryByItemId: {}
    })
    const materialRow = flattenProductionPlan(firstPlan.root).find((row) => row.kind === "material" && row.itemId === "plank")
    expect(materialRow && materialRow.kind === "material").toBe(true)
    if (!materialRow || materialRow.kind !== "material") return
    const switched = buildProductionPlan({
      rootAction: actions[3],
      rootCount: 1,
      actions,
      inventoryByItemId: {},
      selectedRecipeByNode: {
        [materialRow.nodeId]: "saw_slow"
      }
    })
    const selectedAfterSwitch = flattenProductionPlan(switched.root).find((row) => row.kind === "material" && row.itemId === "plank")
    expect(selectedAfterSwitch && selectedAfterSwitch.kind === "material" ? selectedAfterSwitch.selectedActionId : "").toBe("saw_slow")
  })

  it("marks unresolved deficit when no producer exists", () => {
    const actions: ActionDefinition[] = [
      action({ id: "craft_ring", costs: { gem: 2 }, itemReward: { ring: 1 } })
    ]
    const plan = buildProductionPlan({
      rootAction: actions[0],
      rootCount: 1,
      actions,
      inventoryByItemId: {}
    })
    expect(plan.unresolvedMaterials).toEqual([{ itemId: "gem", deficit: 2 }])
  })

  it("ignores probabilistic rewards in guaranteed count calculation", () => {
    const actions: ActionDefinition[] = [
      action({
        id: "hunt",
        itemReward: {
          hide: 1,
          rare_gem: {
            count: 1,
            percent: 0.3
          }
        }
      }),
      action({ id: "craft_amulet", costs: { rare_gem: 1 }, itemReward: { amulet: 1 } })
    ]
    const plan = buildProductionPlan({
      rootAction: actions[1],
      rootCount: 1,
      actions,
      inventoryByItemId: {}
    })
    expect(plan.unresolvedMaterials).toEqual([{ itemId: "rare_gem", deficit: 1 }])
  })

  it("applies actor efficiency to every planned action duration", () => {
    const actions: ActionDefinition[] = [
      action({ id: "mine_ore", mainType: "mining", durationS: 10, itemReward: { iron_ore: 1 } }),
      action({ id: "forge_sword", mainType: "crafting", durationS: 30, costs: { iron_ore: 2 }, itemReward: { iron_sword: 1 } })
    ]

    const plan = buildProductionPlan({
      rootAction: actions[1],
      rootCount: 1,
      actions,
      inventoryByItemId: {},
      durationContext: {
        actorType: "player",
        attributes: {
          mining_speed: 2,
          crafting_speed: 1
        }
      }
    })

    const byAction = new Map(plan.actionSteps.map((row) => [row.actionId, row]))
    expect(byAction.get("forge_sword")?.durationS).toBe(15)
    expect(byAction.get("forge_sword")?.totalDurationS).toBe(15)
    expect(byAction.get("mine_ore")?.durationS).toBe(3.333)
    expect(byAction.get("mine_ore")?.totalDurationS).toBe(6.666)
    expect(plan.totalDurationS).toBeCloseTo(21.666, 3)
  })
})
