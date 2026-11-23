import { describe, expect, it } from "vitest"
import { enchantFoodProbabilities, transmuteFoodBonus, transmuteFoodDuration, transmuteFoodModifiers } from "@services/cuisine-operation-modifiers"
import { operationEffectEligibility } from "@services/operation-effect-context"
import { getFishFoodRecipeByActionId, getFishFoodEffectDefinition, applyFishFoodLifetimeScaling, resolveFishFoodEffectParams, type FishFoodQualityPreview } from "@services/fish-food"
import { operationEffectModifierLines } from "@services/operation-effect-display"
import type { OperationEffectView } from "@services/operation-effects"

const effect = (kind: string, params: Record<string, unknown>, remainingCharges=1): OperationEffectView => ({grantId:1,definitionId:"test",rulesetVersion:"test",domain:kind.startsWith("enchantment")?"enchantment":"alchemy",kind,nameKey:"",descKey:"",targetKind:"player",lifetimeKind:"charges",validFrom:new Date().toISOString(),remainingCharges,params})

describe("point gold and enchantment cuisine", () => {
  it("rounds bonus per input before aggregation and cannot multiply lucky gold", () => {
    expect(transmuteFoodBonus(99,.04)).toBe(3)
    expect(transmuteFoodBonus(99,.04)*2).toBe(6)
    expect(transmuteFoodBonus(1,.10)).toBe(0)
    expect(transmuteFoodBonus(100,.11)).toBe(0)
    expect(transmuteFoodBonus(100,NaN)).toBe(0)
    expect(transmuteFoodModifiers([effect("alchemy.transmute_gold_bonus",{bonusRate:.09}),effect("alchemy.transmute_gold_bonus",{bonusRate:.09})]).bonusRate).toBe(.10)
  })
  it("does not advertise a whole queue as one-second when the food covers only a prefix", () => {
    expect(transmuteFoodDuration(30,10,20)).toBe(120)
    expect(transmuteFoodDuration(10,10,20)).toBe(10)
    expect(transmuteFoodDuration(30,10,0)).toBe(300)
  })
  it("matches frozen quality ranges on both languages and all three mutations", () => {
    for (const [id,plain,mutated,param,maxPlain,maxMutated] of [
      ["honeyflame_quick_grill",40,80,"durationSeconds",1,1],
      ["sunhoney_gold_grill",24,50,"bonusRate",.06,.10],
      ["moonglass_dual_fish_grill",1,1,"weightBonusRate",.15,.25]
    ] as const) {
      const recipe=getFishFoodRecipeByActionId(`cook_${id}`)!,def=getFishFoodEffectDefinition(recipe)!
      for(const special of [1,1.815]) {
        const q:FishFoodQualityPreview={multiplier:special,specialMultiplier:special,lengthScore:1,weightScore:1,traitAverage:0,mutationCount:3,ingredients:[],extraCharges:special>1?(recipe.mutationLifetimeBonuses?.prismatic?.charges||0):0}
        applyFishFoodLifetimeScaling(recipe,def,q)
        expect((def.lifetime.charges||0)+(q.extraCharges||0)).toBe(special>1?mutated:plain)
        expect(resolveFishFoodEffectParams(def,q,recipe)?.[param]).toBe(special>1?maxMutated:maxPlain)
      }
      expect(operationEffectModifierLines(def.kind,def.params,(key)=>key).length).toBeGreaterThan(0)
    }
  })
  it("normalizes only the actual compatible candidate pool without inventing rarities", () => {
    const buff=effect("enchantment.high_rarity_weight",{weightBonusRate:.25})
    const weights={uncommon:60,rare:28,epic:8,legendary:4}
    const rows=enchantFoodProbabilities(weights,[buff])
    expect(rows.find(r=>r.rarityId==="legendary")!.after).toBeCloseTo(5/103,10)
    expect(rows.reduce((n,r)=>n+r.after,0)).toBeCloseTo(1)
    expect(weights).toEqual({uncommon:60,rare:28,epic:8,legendary:4})
    for(const pool of [{epic:8,legendary:4},{rare:100},{}]) expect(operationEffectEligibility(buff,{domain:"enchantment",enchantWeights:pool}).applicable).toBe(false)
    expect(operationEffectEligibility(buff,{domain:"enchantment",enchantWeights:weights}).applicable).toBe(true)
    expect(operationEffectEligibility(effect("alchemy.transmute_gold_bonus",{bonusRate:.04}),{domain:"alchemy"}).applicable).toBe(false)
  })
})
