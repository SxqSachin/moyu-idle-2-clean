import { describe, expect, it } from "vitest"
import type { InventoryInstance } from "@/types/equipment"
import { getFishSpeciesDisplay } from "@services/fish-instance-display"
import zhCN from "@/locales/game/zh-CN"
import enUS from "@/locales/game/en-US"
import { getActionDefinition } from "@services/action-definition"
import { getGameDataTable } from "@services/game-data-runtime"
import type { ActionDefinition } from "@game-data/game-data"
import {
  buildFishFoodSpecificArgs,
  getFishFoodEffectDefinition,
  getFishFoodRecipeByActionId,
  resolveFishFoodEffectParams,
  resolveFishFoodQuality,
  resolveFishFoodLifetime,
  listFishFoodRecipes,
  resolveFishFoodMutationLifetime,
  projectReplacedFishFoodSnapshot,
  type FishFoodRecipeDefinition
} from "@services/fish-food"

describe("fish food contracts", () => {
  it('supports one-fish work meals and the same capped quality on both serving forms', () => {
    for (const [series,parameter,base,plain,maximum] of [['field_honey','chance',.01,.015,.0272],['patient_dual','chance',.01,.015,.0272],['clear_mind','experienceBonusRate',.05,.07,.10],['gentle_current','lossReductionRate',.05,.07,.10]] as const) {
      for (const form of ['grill','soup']) {
        const recipe=requiredRecipe(`cook_${series}_${form}`), def=getFishFoodEffectDefinition(recipe)!
        const selected=recipe.fishIngredients.map((row,i)=>fishInstance(i+1,row.speciesId,'mid'))
        const q=resolveFishFoodQuality(recipe,selected)!
        expect(def.params[parameter]).toBe(base)
        for(const [special,want] of [[1,plain],[1.815,maximum]]) {
          const params=resolveFishFoodEffectParams(def,{...q,lengthScore:1,weightScore:1,traitAverage:0,specialMultiplier:special},recipe)!
          expect(params[parameter]).toBe(want)
          expect(params.durationPenaltyRate || 0).toBe(0)
        }
        expect(buildFishFoodSpecificArgs(recipe,Object.fromEntries(selected.map(row=>[row.fishData!.speciesId,[row.instanceId]])))).toBeTruthy()
      }
    }
  })
  it('keeps creamfin quality identical between grill and soup with independent lifetime bonuses', () => {
    for(const form of ['grill','soup']) {
      const recipe=requiredRecipe(`cook_creamfin_dual_fish_${form}`)
      const def=getFishFoodEffectDefinition(recipe)!
      const quality=resolveFishFoodQuality(recipe,recipe.fishIngredients.map((row,i)=>fishInstance(i+1,row.speciesId,'high')))!
      expect(resolveFishFoodLifetime(def,quality)).toEqual(form==='grill'?{kind:'charges',charges:10}:{kind:'duration',durationSeconds:1500})
      for(const [length,weight,trait,multiplier,want] of [[.5,.5,0,1,.05],[0,0,-1,1,.0275],[1,1,1,1,.0725],[1,.5,0,1,.06],[.5,1,0,1,.0525],[1,1,1,1.815,.10]]) {
        expect(resolveFishFoodEffectParams(def,{...quality,lengthScore:length,weightScore:weight,traitAverage:trait,specialMultiplier:multiplier},recipe)).toEqual({bonusRate:want})
      }
    }
  })
  it("keeps tier scopes fixed while quality raises seven-percent capacity to bounded limits", () => {
    for (const [series,tier,before,maximum] of [["mossfire",1,.12,.20],["lotusflame",2,.10,.15],["frostmoon",0,.09,.12]] as const) {
      for (const form of ["grill","soup"]) {
        const recipe = requiredRecipe(`cook_${series}_dual_fish_${form}`)
        const def = getFishFoodEffectDefinition(recipe)!
        expect(def.params.quantityLimitBonusRate).toBe(.07)
        const q = resolveFishFoodQuality(recipe,recipe.fishIngredients.map((row,i)=>fishInstance(i+1,row.speciesId,"mid")))!
        for(const [score,multiplier,want] of [[0,1,.07],[1,1,before],[1,1.815,maximum]]) {
          const params=resolveFishFoodEffectParams(def,{...q,lengthScore:score,weightScore:score,traitAverage:0,specialMultiplier:multiplier},recipe)!
          expect(params.quantityLimitBonusRate).toBe(want)
          expect(params.materialTier || 0).toBe(tier)
        }
      }
    }
  })
  it("offers individual and shared audiences, keeping enchantment player-only", () => {
    for (const recipe of listFishFoodRecipes()) { const def=getFishFoodEffectDefinition(recipe)!; expect(def.allowedTargets).toEqual(def.domain === "enchantment" ? ["player"] : def.lifetime.kind === "duration" ? ["player", "kitty"] : ["player", "kitty", "player_and_kitties"]) }
  })
  it("caps size growth before multiplying special mutations", () => {
    for (const [id, base, cap, mutated] of [["mossfire_dual_fish_grill",50,200,363],["mossfire_dual_fish_soup",45,180,327],["frostmoon_dual_fish_grill",25,50,91],["frostmoon_dual_fish_soup",25,50,91]] as const) {
      const recipe = requiredRecipe(`cook_${id}`)
      const def = getFishFoodEffectDefinition(recipe)!
      const quality = resolveFishFoodQuality(recipe, recipe.fishIngredients.map((row,i)=>fishInstance(i+1,row.speciesId,"mid")))!
      for (const [score,trait,special,want] of [[0,0,1,base],[1,0,1,cap],[1,1,1.815,mutated]]) {
        expect(resolveFishFoodEffectParams(def,{...quality,lengthScore:score,weightScore:score,traitAverage:trait,specialMultiplier:special},recipe)?.extraMaterialQuantity).toBe(want)
      }
    }
  })
  it("configures mutation bonuses per recipe without changing potency or counting duplicate fish twice", () => {
    for (const id of ["frugal_river_grill", "harvest_river_grill", "reedbank_grill"]) {
      const recipe = requiredRecipe(`cook_${id}`)
      const definition = getFishFoodEffectDefinition(recipe)!
      expect(definition.domain).toBe("fishing_cast")
      expect(definition.lifetime.charges).toBe(12)
      const selected = recipe.fishIngredients.map((row, i) => fishInstance(i + 1, row.speciesId, "high"))
      expect(resolveFishFoodLifetime(definition, resolveFishFoodQuality(recipe, selected))).toEqual({ kind: "charges", charges: 20 })
      const custom = { ...recipe, mutationLifetimeBonuses: { prismatic: { charges: 8 }, shimmering: { charges: 3 } } }
      expect(resolveFishFoodMutationLifetime(custom, new Set(["prismatic", "prismatic", "shimmering"]))).toEqual({ extraCharges: 11, extraDurationSeconds: 0 })
      expect(resolveFishFoodMutationLifetime({ ...recipe, mutationLifetimeBonuses: {} }, new Set(["prismatic"])).extraCharges).toBe(0)
    }
    for (const recipe of listFishFoodRecipes()) {
      const definition = getFishFoodEffectDefinition(recipe)!
      if (definition.domain === "life_action") expect(definition.selector.mainTypes).toEqual(["collecting", "mining", "farming_animal"])
      expect(["life_action.material_refund_chance", "life_action.extra_main_output_chance"]).not.toContain(definition.kind)
    }
  })

  it("projects retired food effects from saved ingredients without modifying the snapshot", () => {
    const recipe = requiredRecipe("cook_frugal_river_grill")
    const original = {
      fishFood: { schemaVersion: 1, recipeId: recipe.id, qualityRulesetVersion: "fish-food-quality-v2", quality: { multiplier: 1.1, specialMultiplier: 1.1, lengthScore: 0.5, weightScore: 0.5, traitAverage: 0, mutationCount: 1 }, ingredients: [{ mutations: ["prismatic"] }, { mutations: ["prismatic"] }] },
      operationEffect: { definitionId: "food_frugal_river_grill_v2", params: { chance: 0.5, refundRate: 1 } }
    }
    const projected = projectReplacedFishFoodSnapshot(original)
    expect(projected.operationEffect).toMatchObject({ definitionId: "food_frugal_river_grill_v3", lifetime: { charges: 20 }, params: { rate: 0.088 } })
    expect(original.operationEffect.params).toEqual({ chance: 0.5, refundRate: 1 })
  })

  it("requires cooking fuel or soup water in addition to each recipe's ingredients", () => {
    for (const recipe of listFishFoodRecipes()) {
      const costs = getGameDataTable<ActionDefinition>('actions').find(action => action.id === recipe.actionId)!.costs!
      if (recipe.kind === "grilled_fish") {
        expect(costs.coal).toBeGreaterThanOrEqual(2)
        expect(costs.morning_dew).toBeUndefined()
      } else {
        expect(costs.morning_dew).toBeGreaterThanOrEqual(3)
        expect(costs.coal).toBeUndefined()
      }
      expect(Object.keys(costs).length).toBeGreaterThanOrEqual(getFishFoodEffectDefinition(recipe)!.domain === "life_action" ? 2 : 3)
    }
  })

  it("gives shimmering a visible benefit even on large fish with both other special mutations", () => {
    for (const recipe of listFishFoodRecipes()) {
      const definition = getFishFoodEffectDefinition(recipe)!
      const all = recipe.fishIngredients.map((row, i) => fishInstance(i + 1, row.speciesId, "high"))
      for (const row of all) row.fishData!.weightKg *= 1.25
      const withoutShimmer = structuredClone(all)
      for (const row of withoutShimmer) row.fishData!.genes.mutations = ["prismatic", "rainbow"]
      const baseQuality = resolveFishFoodQuality(recipe, withoutShimmer)!
      const shinyQuality = resolveFishFoodQuality(recipe, all)!
      const base = resolveFishFoodEffectParams(definition, baseQuality, recipe)!
      const shiny = resolveFishFoodEffectParams(definition, shinyQuality, recipe)!
      expect(shinyQuality.specialMultiplier).toBe(1.815)
      if (recipe.lifetimeScaling?.charges) expect(shinyQuality.extraCharges).toBeGreaterThan(baseQuality.extraCharges || 0)
      else if (recipe.lifetimeScaling?.durationSeconds) expect(shinyQuality.extraDurationSeconds).toBeGreaterThan(baseQuality.extraDurationSeconds || 0)
      else expect(resolveFishFoodLifetime(definition, shinyQuality)).toEqual(resolveFishFoodLifetime(definition, baseQuality))
      for (const [name, scale] of Object.entries(recipe.parameterScaling!)) {
        if (definition.domain === "life_action" && (scale.min === scale.max || base[name] === scale.max)) {
          expect(shiny[name]).toBe(scale.max)
          continue
        }
        if (scale.inverse) expect(shiny[name]).toBeLessThan(base[name])
        else expect(shiny[name]).toBeGreaterThan(base[name])
        expect(shiny[name]).toBeGreaterThanOrEqual(scale.min)
        expect(shiny[name]).toBeLessThanOrEqual(scale.max)
      }
    }
  })

  it("provides cuisine names at the game locale namespace used by catalog keys", () => {
    for (const messages of [zhCN, enUS]) {
      for (const recipe of listFishFoodRecipes()) {
        const names: Record<string, string> = messages.game.fishCuisine.names
        const descriptions: Record<string, string> = messages.game.fishCuisine.descriptions
        expect(names[recipe.outputItemId]).toBeTruthy()
        expect(descriptions[recipe.outputItemId]).toBeTruthy()
      }
    }
  })

  it("keeps grilled fish charge-based and fish soup duration-based", () => {
    const grill = requiredRecipe("cook_mossfire_dual_fish_grill")
    const soup = requiredRecipe("cook_starlight_silverfish_soup")

    expect(grill.kind).toBe("grilled_fish")
    expect(new Set(grill.fishIngredients.map((row) => row.speciesId)).size).toBeGreaterThanOrEqual(2)
    expect(getFishFoodEffectDefinition(grill)?.lifetime).toMatchObject({ kind: "charges", charges: 1 })

    expect(soup.kind).toBe("fish_soup")
    expect(new Set(soup.fishIngredients.map((row) => row.speciesId)).size).toBeGreaterThanOrEqual(2)
    expect(getFishFoodEffectDefinition(soup)?.lifetime).toMatchObject({ kind: "duration", durationSeconds: 600 })
  })

  it("builds exact instance arguments only after every species is satisfied", () => {
    const recipe = requiredRecipe("cook_mossfire_dual_fish_grill")
    expect(buildFishFoodSpecificArgs(recipe, { crucian_carp: [11] })).toBeUndefined()
    expect(buildFishFoodSpecificArgs(recipe, { crucian_carp: [11], bluegill: [11] })).toBeUndefined()
    expect(buildFishFoodSpecificArgs(recipe, { crucian_carp: [22], bluegill: [11] })).toEqual({
      ingredientMode: "specific",
      fishIngredientInstances: {
        crucian_carp: [22],
        bluegill: [11]
      }
    })
  })

  it("matches the server quality baseline and scales benefits without worsening the grill tradeoff", () => {
    const recipe = requiredRecipe("cook_mossfire_dual_fish_grill")
    const midpointFish = recipe.fishIngredients.map((ingredient, index) => fishInstance(index + 1, ingredient.speciesId, "mid"))
    const baseline = resolveFishFoodQuality(recipe, midpointFish)
    expect(baseline?.multiplier).toBe(1)

    const highFish = recipe.fishIngredients.map((ingredient, index) => fishInstance(index + 10, ingredient.speciesId, "high"))
    const high = resolveFishFoodQuality(recipe, highFish)
    expect(high?.specialMultiplier).toBe(1.815)
    expect(high?.extraCharges).toBe(1)

    const definition = getFishFoodEffectDefinition(recipe)
    expect(definition).not.toBeNull()
    const standardParams = resolveFishFoodEffectParams(definition!, 1)
    const highParams = resolveFishFoodEffectParams(definition!, high!, recipe)
    expect(standardParams).toEqual({ materialTier: 1, extraMaterialQuantity: 50, positiveFloorReductionRate: 0.12, quantityLimitBonusRate: .07, successRatePenalty: .02 })
    expect(highParams?.extraMaterialQuantity).toBeGreaterThan(standardParams!.extraMaterialQuantity)
    expect(highParams?.positiveFloorReductionRate).toBeLessThan(standardParams!.positiveFloorReductionRate)
  })

  it("scales soup precision from the same frozen quality multiplier", () => {
    const recipe = requiredRecipe("cook_starlight_silverfish_soup")
    const definition = getFishFoodEffectDefinition(recipe)
    expect(resolveFishFoodEffectParams(definition!, 1.25)).toEqual({ positiveCeilingBonusRate: 0.0625 })
  })

  it("keeps soup potency close to grilled food across size, traits and mutations", () => {
    for (const grill of listFishFoodRecipes().filter(recipe => recipe.kind === 'grilled_fish')) {
      if (['alchemy.transmute_duration','enchantment.high_rarity_weight'].includes(getFishFoodEffectDefinition(grill)!.kind)) continue
      const soup = requiredRecipe(grill.actionId.replace(/_grill$/, '_soup'))
      const grillDef = getFishFoodEffectDefinition(grill)!
      const soupDef = getFishFoodEffectDefinition(soup)!
      const quality = resolveFishFoodQuality(grill, grill.fishIngredients.map((row, i) => fishInstance(i + 1, row.speciesId, 'mid')))!
      expect(soupDef.lifetime.kind).toBe('duration')
      if (!soup.lifetimeScaling) expect(soupDef.lifetime.durationSeconds).toBe(600)
      for (const lengthScore of [0,.25,.5,.75,1]) for (const weightScore of [0,.25,.5,.75,1]) {
        for (const traitAverage of [-1,0,1]) for (const specialMultiplier of [1,1.815]) {
          const q = {...quality,lengthScore,weightScore,traitAverage,specialMultiplier}
          const g = resolveFishFoodEffectParams(grillDef,q,grill)!
          const s = resolveFishFoodEffectParams(soupDef,q,soup)!
          for (const name of Object.keys(g)) {
            expect(s[name]).toBeLessThanOrEqual(g[name] + .0001)
            if (!grill.parameterScaling?.[name]?.inverse) expect(s[name]).toBeGreaterThanOrEqual(g[name] * .79)
          }
        }
      }
    }
  })

  it("pairs every effect with identical species and separates lifespan from potency", () => {
    const recipes = listFishFoodRecipes()
    expect(recipes).toHaveLength(30)
    for (const grill of recipes.filter((row) => row.kind === "grilled_fish")) {
      const effect = getFishFoodEffectDefinition(grill)!
      if (["alchemy.transmute_duration","enchantment.high_rarity_weight"].includes(effect.kind)) { expect(effect.lifetime.kind).toBe("charges"); continue }
      const soup = recipes.find((row) => row.id === grill.id.replace(/_grill$/, "_soup"))!
      expect(getFishFoodEffectDefinition(soup)?.kind).toBe(effect.kind)
      expect(soup.fishIngredients).toEqual(grill.fishIngredients)
      const selected = soup.fishIngredients.map((row, i) => fishInstance(i + 1, row.speciesId, "high"))
      const quality = resolveFishFoodQuality(soup, selected)!
      expect(quality.extraCharges).toBe(0)
      expect(quality.specialMultiplier).toBe(1.815)
      const soupDef = getFishFoodEffectDefinition(soup)!
      expect(resolveFishFoodLifetime(soupDef, quality)).toEqual({ kind: "duration", durationSeconds: Number(soupDef.lifetime.durationSeconds) + Number(quality.extraDurationSeconds || 0) })
    }
  })

  it("does not double count length as weight and assigns parameter-specific weights", () => {
    const recipe = requiredRecipe("cook_mossfire_dual_fish_grill")
    const normal = recipe.fishIngredients.map((row, i) => fishInstance(i + 1, row.speciesId, "mid"))
    const long = structuredClone(normal)
    for (const row of long) { const fish = row.fishData!; fish.lengthCm *= 1.1; fish.weightKg *= 1.1 ** 3 }
    const base = resolveFishFoodQuality(recipe, normal)!
    const length = resolveFishFoodQuality(recipe, long)!
    expect(length.weightScore).toBe(base.weightScore)
    const heavy = structuredClone(normal)
    for (const row of heavy) row.fishData!.weightKg *= 1.1
    const weight = resolveFishFoodQuality(recipe, heavy)!
    expect(weight.lengthScore).toBe(base.lengthScore)
    expect(weight.parameterMultipliers!.extraMaterialQuantity).toBeGreaterThan(weight.parameterMultipliers!.positiveFloorReductionRate)
    expect(length.parameterMultipliers!.positiveFloorReductionRate).toBeGreaterThan(weight.parameterMultipliers!.positiveFloorReductionRate)
    expect(weight.parameterMultipliers!.extraMaterialQuantity).toBeGreaterThan(length.parameterMultipliers!.extraMaterialQuantity)
  })
})

function requiredRecipe(actionID: string): FishFoodRecipeDefinition {
  const recipe = getFishFoodRecipeByActionId(actionID)
  if (!recipe) throw new Error(`missing fish food recipe ${actionID}`)
  return recipe
}

function fishInstance(instanceID: number, speciesID: string, quality: "mid" | "high"): InventoryInstance {
  const species = getFishSpeciesDisplay(speciesID)
  if (!species) throw new Error(`missing fish species ${speciesID}`)
  const lengthCm = quality === "high"
    ? species.maxLengthCm
    : (species.minLengthCm + species.maxLengthCm) / 2
  const weightKg = species.shapeFactor * lengthCm ** 3 / 100_000
  const trait = quality === "high" ? 1 : 0
  return {
    instanceId: instanceID,
    templateItemId: "fish_catch",
    quantity: 1,
    state: "inventory",
    enhanceLevel: 0,
    durability: 100,
    maxDurability: 100,
    baseSnapshot: {},
    affixes: [],
    fishData: {
      speciesId: speciesID,
      ageDays: 100,
      sex: "female",
      lengthCm,
      weightKg,
      condition: 1,
      health: 1,
      waterId: "test",
      spotId: "test",
      caughtAt: "2026-09-05T00:00:00Z",
      rulesetVersion: "test",
      genes: {
        sizePotential: trait,
        growthRate: trait,
        feedEfficiency: trait,
        vitality: trait,
        lifespan: trait,
        fertility: trait,
        mutations: quality === "high" ? ["prismatic", "rainbow", "shimmering"] : []
      }
    }
  }
}
