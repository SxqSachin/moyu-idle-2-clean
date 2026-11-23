import { describe, expect, it } from "vitest"
import { conversionFoodFailureChance, conversionFoodReduction } from "@services/conversion-food"
import { getFishFoodRecipeByActionId, getFishFoodEffectDefinition } from "@services/fish-food"

describe("conversion food", () => {
  it("reduces loss relatively rather than adding success, including the old loss floor", () => {
    expect(conversionFoodFailureChance(.1865,.05)).toBeCloseTo(.177175)
    expect(conversionFoodFailureChance(.1865,.1)).toBeCloseTo(.16785)
    expect(conversionFoodFailureChance(.15,.1)).toBeCloseTo(.135)
    for(const rate of [-1,NaN,.11])expect(conversionFoodFailureChance(.1865,rate)).toBe(.1865)
  })
  it("does not add overlapping strengths or accept unknown food kinds", () => {
    const food=(kind:string,rate:number)=>({kind,params:{lossReductionRate:rate}})
    expect(conversionFoodReduction([food('alchemy.conversion_loss_reduction',.05),food('alchemy.conversion_loss_reduction',.08)])).toBe(.08)
    expect(conversionFoodReduction([food('alchemy.infusion_success_bonus',.1)])).toBe(0)
  })
  it("uses common loach and aligned 220-use / 900-second servings", () => {
    for(const form of ['grill','soup']){
      const recipe=getFishFoodRecipeByActionId(`cook_gentle_current_${form}`)!,def=getFishFoodEffectDefinition(recipe)!
      expect(recipe.fishIngredients).toEqual([{speciesId:'mud_loach',quantity:1}])
      expect(def.params).toEqual({lossReductionRate:.05})
      expect(def.selector).toEqual({actionIds:['alchemy.convert']})
      expect(def.lifetime).toEqual(form==='grill'?{kind:'charges',charges:220}:{kind:'duration',durationSeconds:900})
    }
  })
})
