import { describe, expect, it } from "vitest"
import { infusionMaterialQuantityBonus, infusionQuantityLimits, applyInfusionOperationRange, applyInfusionFoodFailureChance } from "@services/alchemy-operation-effects"

it('combines pure success bonuses with food costs before clamping', () => {
  for (const [fail,penalty,bonus,floor,want] of [[.08,0,.03,0,.05],[.08,.06,.03,0,.11],[.01,0,.03,0,0],[0,.04,.03,0,.01],[.82,.06,.06,.16,.82],[.2,0,.2,0,.10]]) {
    expect(applyInfusionFoodFailureChance(fail,penalty,floor,bonus)).toBeCloseTo(want)
  }
})

it('bases proportional capacity on current capacity and preserves food risk and medium floors', () => {
  const food = {extraMaterialQuantity:0,bulkMaterialQuantity:50,preciousMaterialQuantity:25,bulkQuantityLimitBonusRate:.1,preciousQuantityLimitBonusRate:.1}
  expect(infusionQuantityLimits('epic',100,10000,1,food).capacity).toBe(150)
  expect(infusionQuantityLimits('mythic',100,10000,5,food).capacity).toBe(125)
  expect(applyInfusionFoodFailureChance(0,.06)).toBe(.06)
  expect(applyInfusionFoodFailureChance(.3,.06)).toBeCloseTo(.36)
  expect(applyInfusionFoodFailureChance(.82,.06,.16)).toBe(.84)
  expect(applyInfusionFoodFailureChance(.97,.06)).toBe(1)
  expect(applyInfusionFoodFailureChance(.123,0)).toBe(.123)
})

describe("bulk infusion material limits", () => {
  it('breaks the unbuffed endgame ceiling without adding the fixed and percentage bonuses twice', () => {
    const food = {extraMaterialQuantity:0,bulkMaterialQuantity:363,preciousMaterialQuantity:91,bulkQuantityLimitBonusRate:.17,preciousQuantityLimitBonusRate:.17}
    expect(infusionQuantityLimits('mythic',1600,10000,5,food)).toMatchObject({capacity:1870,effectiveBonus:270})
    expect(infusionQuantityLimits('legendary',6400,10000,4,food)).toMatchObject({capacity:7488,effectiveBonus:1088})
    expect(infusionQuantityLimits('epic',6400,10000,4,food).capacity).toBe(7488)
    expect(infusionMaterialQuantityBonus('epic',200,food)).toBe(363)
    expect(infusionMaterialQuantityBonus('mythic',50,food)).toBe(91)
    expect(infusionMaterialQuantityBonus('unknown',6400,food)).toBe(0)
    expect(infusionQuantityLimits('mythic',1600,10000,5,{...food,preciousMaterialQuantity:25,preciousQuantityLimitBonusRate:.1}).capacity).toBe(1760)
  })
  it('keeps precious and bulk capacities independent in mixed infusions', () => {
    const food = {extraMaterialQuantity:0,bulkMaterialQuantity:363,preciousMaterialQuantity:91}
    expect(infusionMaterialQuantityBonus('epic',100,food)).toBe(200)
    expect(infusionMaterialQuantityBonus('legendary',50,food)).toBe(91)
    expect(infusionMaterialQuantityBonus('mythic',10,food)).toBe(20)
    expect(infusionMaterialQuantityBonus('unknown',100,food)).toBe(0)
    expect(infusionQuantityLimits('legendary',48,200,8,food)).toMatchObject({capacity:136,effectiveBonus:88})
  })
  it.each([["common",200,363],["epic",50,100],["epic",120,240],["legendary",50,0],["mythic",50,0],["unknown",200,0]] as const)("%s cap %s grants %s", (rarity, base, expected) => {
    expect(infusionMaterialQuantityBonus(rarity,base,{extraMaterialQuantity:0,bulkMaterialQuantity:363})).toBe(expected)
  })

  it('separates food capacity from stock and batch alignment', () => {
    const food = {extraMaterialQuantity:0, bulkMaterialQuantity:50}
    const limits = infusionQuantityLimits('rare', 80, 50, 8, food)
    expect(limits).toEqual({baseCapacity:80, capacity:128, bonus:50, effectiveBonus:48, usableStock:48, selectable:48, stockLimited:true, basis:8})
    expect(infusionQuantityLimits('rare',80,200,8,food).selectable).toBe(128)
    expect(infusionQuantityLimits('rare',80,200,8,{...food,bulkMaterialQuantity:0}).selectable).toBe(80)
    expect(infusionQuantityLimits('mythic',80,200,8,food).effectiveBonus).toBe(0)
    expect(infusionQuantityLimits('rare',80,7,8,food).selectable).toBe(0)
  })

  it('uses the same food range calculation for previews and explanations', () => {
    const none = {positiveFloorReductionRate:0,positiveCeilingBonusRate:0}
    expect(applyInfusionOperationRange(1,4,none)).toEqual([1,4])
    expect(applyInfusionOperationRange(1,4,{...none,positiveFloorReductionRate:.12})).toEqual([.88,4])
    expect(applyInfusionOperationRange(1,4,{...none,positiveCeilingBonusRate:.1})).toEqual([1,4.4])
    expect(applyInfusionOperationRange(-4,-1,{positiveFloorReductionRate:.12,positiveCeilingBonusRate:.1})).toEqual([-4,-1])
    expect(applyInfusionOperationRange(4,4,{positiveFloorReductionRate:.12,positiveCeilingBonusRate:.1})).toEqual([3.52,4.4])
    expect(applyInfusionOperationRange(-2,4,{positiveFloorReductionRate:.12,positiveCeilingBonusRate:.1})).toEqual([-2,4.4])
    expect(applyInfusionOperationRange(4,1,none)).toEqual([1,4])
  })
})
